package dev.minefleet.api.gateway.networking;

import dev.minefleet.api.gateway.networking.player.KickReason;
import dev.minefleet.api.gateway.networking.player.NetworkPlayer;
import dev.minefleet.api.gateway.networking.player.PlayerLookup;
import dev.minefleet.api.gateway.networking.player.PlayerProvider;
import dev.minefleet.api.gateway.networking.stream.NetworkStreamClient;
import dev.minefleet.api.gateway.networking.stream.ServerSetDelta;
import dev.minefleet.api.gateway.networking.v1alpha1.Api;
import dev.minefleet.api.gateway.networking.v1alpha1.Types;
import io.grpc.Channel;
import io.grpc.LoadBalancerRegistry;
import io.grpc.ManagedChannelBuilder;
import io.grpc.NameResolverRegistry;
import io.grpc.internal.DnsNameResolverProvider;
import io.grpc.internal.PickFirstLoadBalancerProvider;

import java.util.ArrayList;
import java.util.Collections;
import java.util.HashMap;
import java.util.List;
import java.util.Map;
import java.util.Optional;
import java.util.UUID;
import java.util.concurrent.CompletableFuture;
import java.util.concurrent.ConcurrentHashMap;
import java.util.concurrent.TimeUnit;
import java.util.function.Supplier;
import java.util.logging.Level;
import java.util.logging.Logger;

/**
 * The proxy-side entry point to the gateway.
 *
 * <p>Routing decisions are made by the controller, which sees player counts
 * across every proxy at once. This proxy reports where its players are, asks the
 * controller where each joining player should go, and applies the move commands
 * the controller sends on behalf of PlayerTransfer resources.
 */
public class NetworkGateway {

    private static final Logger LOGGER = Logger.getLogger(NetworkGateway.class.getName());

    /** How long a join waits for a routing decision before the player is kicked. */
    private static final long ROUTE_TIMEOUT_SECONDS = 5;

    private final NetworkStreamClient stream;
    private final ServerRegistrar registrar;
    private final int retries;

    /** Servers currently registered, by unique id, so pushes can be diffed. */
    private final Map<String, ManagedServer> registered = new ConcurrentHashMap<>();
    /** Permissions the controller needs evaluated for every player. */
    private volatile List<String> requiredPermissions = List.of();
    /** In-flight routing requests, awaiting a response by correlation id. */
    private final Map<String, CompletableFuture<Api.RouteResponse>> pendingRoutes = new ConcurrentHashMap<>();

    @SuppressWarnings("rawtypes")
    private PlayerProvider playerProvider;
    private PlayerLookup playerLookup;

    private NetworkGateway(Builder builder) {
        this.registrar = builder.registrar;
        this.retries = builder.retries;
        this.stream = new NetworkStreamClient(
                builder.channel,
                builder.context,
                this::handle,
                builder.presenceSupplier);
    }

    public void start() {
        stream.start();
    }

    public void stop() {
        stream.stop();
    }

    /** Whether the controller is currently reachable. */
    public boolean isReady() {
        return stream.isReady();
    }

    /** The permissions the controller asked this proxy to evaluate. */
    public List<String> requiredPermissions() {
        return requiredPermissions;
    }

    // ---------------------------------------------------------------- routing

    /** Asks the controller where a joining player should go, then connects them. */
    public CompletableFuture<Void> routeJoin(NetworkPlayer player) {
        return route(player, Api.RouteKind.ROUTE_KIND_JOIN, KickReason.NO_JOIN);
    }

    /** Asks the controller where a kicked player should fall back to. */
    public CompletableFuture<Void> routeFallback(NetworkPlayer player) {
        return route(player, Api.RouteKind.ROUTE_KIND_FALLBACK, KickReason.NO_FALLBACK);
    }

    private CompletableFuture<Void> route(NetworkPlayer player, Api.RouteKind kind, KickReason noRouteReason) {
        if (!stream.isReady()) {
            // Routing against a stale local view is what caused servers to be
            // overloaded; refuse the join instead.
            player.kick(KickReason.ROUTING_UNAVAILABLE);
            return CompletableFuture.completedFuture(null);
        }

        String correlationId = UUID.randomUUID().toString();
        Api.RouteRequest.Builder request = Api.RouteRequest.newBuilder()
                .setCorrelationId(correlationId)
                .setPlayerUuid(player.getUuid().toString())
                .setKind(kind)
                .setContext(contextOf(player));
        player.getConnectedServer().ifPresent(server -> request.setCurrentServerName(server.name()));

        CompletableFuture<Api.RouteResponse> pending = new CompletableFuture<>();
        pendingRoutes.put(correlationId, pending);

        if (!stream.send(Api.ProxyMessage.newBuilder().setRouteRequest(request).build())) {
            pendingRoutes.remove(correlationId);
            player.kick(KickReason.ROUTING_UNAVAILABLE);
            return CompletableFuture.completedFuture(null);
        }

        return pending
                .orTimeout(ROUTE_TIMEOUT_SECONDS, TimeUnit.SECONDS)
                .handle((response, error) -> {
                    pendingRoutes.remove(correlationId);
                    if (error != null || response == null) {
                        LOGGER.log(Level.WARNING, "no routing decision for player " + player.getUuid());
                        player.kick(KickReason.ROUTING_UNAVAILABLE);
                        return null;
                    }
                    if (response.getResult() != Api.RouteResponse.Result.RESULT_OK) {
                        player.kick(noRouteReason);
                        return null;
                    }
                    Optional<ManagedServer> target = registrar.findByName(response.getServerName());
                    if (target.isEmpty()) {
                        LOGGER.warning("controller chose unregistered server " + response.getServerName());
                        player.kick(noRouteReason);
                        return null;
                    }
                    player.connectToServer(target.get());
                    return null;
                });
    }

    private Api.PlayerContext contextOf(NetworkPlayer player) {
        Api.PlayerContext.Builder context = Api.PlayerContext.newBuilder()
                .setConnectedDomain(player.getConnectedDomain());
        for (String permission : requiredPermissions) {
            context.putPermissions(permission, player.hasPermission(permission));
        }
        return context.build();
    }

    // --------------------------------------------------------------- presence

    /** Reports that a player reached a backend server. */
    public void reportConnected(NetworkPlayer player, String serverName) {
        stream.send(Api.ProxyMessage.newBuilder()
                .setPresenceEvent(Api.PlayerPresenceEvent.newBuilder()
                        .setPlayerUuid(player.getUuid().toString())
                        .setServerName(serverName)
                        .setKind(Api.PlayerPresenceEvent.Kind.KIND_CONNECTED)
                        .setContext(contextOf(player)))
                .build());
    }

    /** Reports that a player left this proxy. */
    public void reportDisconnected(UUID playerUuid) {
        stream.send(Api.ProxyMessage.newBuilder()
                .setPresenceEvent(Api.PlayerPresenceEvent.newBuilder()
                        .setPlayerUuid(playerUuid.toString())
                        .setKind(Api.PlayerPresenceEvent.Kind.KIND_DISCONNECTED))
                .build());
    }

    /** Builds a presence entry for the reconnect replay. */
    public Api.PresenceEntry presenceEntry(NetworkPlayer player, String serverName) {
        return Api.PresenceEntry.newBuilder()
                .setPlayerUuid(player.getUuid().toString())
                .setServerName(serverName)
                .setContext(contextOf(player))
                .build();
    }

    // ------------------------------------------------------- inbound handling

    private void handle(Api.ControllerMessage message) {
        switch (message.getMessageCase()) {
            case SERVER_SYNC -> applyServerSync(message.getServerSync());
            case ROUTE_RESPONSE -> {
                CompletableFuture<Api.RouteResponse> pending =
                        pendingRoutes.remove(message.getRouteResponse().getCorrelationId());
                if (pending != null) {
                    pending.complete(message.getRouteResponse());
                }
            }
            case MOVE -> applyMove(message.getMove());
            case MESSAGE_NOT_SET -> LOGGER.fine("ignoring empty controller message");
        }
    }

    /**
     * Applies the full server set pushed by the controller, registering what is
     * new and dropping what is gone.
     */
    private void applyServerSync(Api.ServerSync sync) {
        requiredPermissions = List.copyOf(sync.getRequiredPermissionsList());

        List<ManagedServer> current = new ArrayList<>(sync.getServersCount());
        for (Types.ManagedServer proto : sync.getServersList()) {
            current.add(new ManagedServer(proto));
        }

        ServerSetDelta delta = ServerSetDelta.between(Map.copyOf(registered), current);
        if (delta.isEmpty()) {
            return;
        }

        for (ManagedServer server : applyWithRetries(delta.addedOrUpdated(), registrar::registerOrUpdate)) {
            LOGGER.warning("giving up registering server " + server.name());
        }
        for (ManagedServer server : delta.addedOrUpdated()) {
            registered.put(server.uniqueId(), server);
        }

        applyWithRetries(delta.removed(), registrar::unregister);
        for (ManagedServer server : delta.removed()) {
            registered.remove(server.uniqueId());
        }
    }

    /** Applies a registrar action, retrying the failures. Returns what never succeeded. */
    private List<ManagedServer> applyWithRetries(List<ManagedServer> servers, RegistrarAction action) {
        List<ManagedServer> remaining = servers;
        for (int attempt = 0; attempt <= retries && !remaining.isEmpty(); attempt++) {
            remaining = tryApply(remaining, action);
        }
        return remaining;
    }

    private List<ManagedServer> tryApply(List<ManagedServer> servers, RegistrarAction action) {
        if (servers.isEmpty()) {
            return List.of();
        }
        try {
            action.apply(servers);
            return List.of();
        } catch (ServerRegistrarBulkException e) {
            return e.failures().stream().map(ServerRegistrarException::server).toList();
        }
    }

    @FunctionalInterface
    private interface RegistrarAction {
        void apply(List<ManagedServer> servers) throws ServerRegistrarBulkException;
    }

    /** Executes a move requested by a PlayerTransfer and reports the outcome. */
    private void applyMove(Api.MoveCommand command) {
        String reason = null;
        try {
            UUID playerUuid = UUID.fromString(command.getPlayerUuid());
            Optional<NetworkPlayer> player = playerLookup == null
                    ? Optional.empty()
                    : playerLookup.byUuid(playerUuid);
            Optional<ManagedServer> target = registrar.findByName(command.getServerName());

            if (player.isEmpty()) {
                reason = "player_not_found";
            } else if (target.isEmpty()) {
                reason = "server_not_registered";
            } else {
                player.get().connectToServer(target.get());
            }
        } catch (IllegalArgumentException e) {
            reason = "invalid_player_uuid";
        } catch (RuntimeException e) {
            LOGGER.log(Level.WARNING, "move command failed", e);
            reason = "connect_failed";
        }

        Api.MoveResult.Builder result = Api.MoveResult.newBuilder()
                .setCommandId(command.getCommandId())
                .setSuccess(reason == null);
        if (reason != null) {
            result.setReason(reason);
        }
        stream.send(Api.ProxyMessage.newBuilder().setMoveResult(result).build());
    }

    // ---------------------------------------------------------------- players

    public <P> void setPlayerProvider(Class<P> playerType, PlayerProvider<P> provider) {
        this.playerProvider = player -> {
            if (!playerType.isInstance(player)) {
                throw new IllegalArgumentException("Player is not of type " + playerType.getName());
            }
            return provider.getPlayer(playerType.cast(player));
        };
    }

    public void setPlayerLookup(PlayerLookup lookup) {
        this.playerLookup = lookup;
    }

    @SuppressWarnings("unchecked")
    public NetworkPlayer getPlayer(Object runtimePlayer) {
        if (playerProvider == null) {
            throw new IllegalStateException("No player provider registered. Call setPlayerProvider() during initialization.");
        }
        return playerProvider.getPlayer(runtimePlayer);
    }

    private static NetworkGateway INSTANCE = null;

    public static void setInstance(NetworkGateway gateway) {
        INSTANCE = gateway;
    }

    public static NetworkGateway getInstance() {
        return INSTANCE;
    }

    public static Builder builder() {
        return new Builder();
    }

    public static class Builder {
        private Channel channel;
        private NetworkContext context;
        private ServerRegistrar registrar;
        private int retries = 3;
        private Supplier<List<Api.PresenceEntry>> presenceSupplier = Collections::emptyList;

        public Builder channel(Channel channel) {
            this.channel = channel;
            return this;
        }

        public Builder context(NetworkContext context) {
            this.context = context;
            return this;
        }

        public Builder registrar(ServerRegistrar registrar) {
            this.registrar = registrar;
            return this;
        }

        public Builder retries(int retries) {
            this.retries = retries;
            return this;
        }

        /** Supplies the players to replay whenever the stream reconnects. */
        public Builder presenceSupplier(Supplier<List<Api.PresenceEntry>> presenceSupplier) {
            this.presenceSupplier = presenceSupplier;
            return this;
        }

        public NetworkGateway build() {
            if (channel == null) channel = channelFromEnv();
            if (context == null) context = NetworkContext.fromEnv();
            if (registrar == null) throw new IllegalStateException("registrar is required");
            return new NetworkGateway(this);
        }

        private static Channel channelFromEnv() {
            String host = System.getenv("GATEWAY_NETWORK_XDS_HOST");
            String portStr = System.getenv("GATEWAY_NETWORK_XDS_PORT");
            if (host == null || portStr == null) {
                throw new IllegalStateException(
                        "Missing required environment variables: GATEWAY_NETWORK_XDS_HOST, GATEWAY_NETWORK_XDS_PORT");
            }
            NameResolverRegistry.getDefaultRegistry().register(new DnsNameResolverProvider());
            LoadBalancerRegistry.getDefaultRegistry().register(new PickFirstLoadBalancerProvider());
            return ManagedChannelBuilder.forTarget("dns:///" + host + ":" + portStr)
                    .usePlaintext()
                    .build();
        }
    }
}
