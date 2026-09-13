package dev.minefleet.api.gateway.networking;

import dev.minefleet.api.gateway.networking.v1alpha1.Api;
import dev.minefleet.api.gateway.networking.v1alpha1.NetworkGatewayGrpc;
import io.grpc.Channel;
import io.grpc.ManagedChannel;
import io.grpc.ManagedChannelBuilder;

import java.time.Duration;
import java.util.Collection;
import java.util.List;
import java.util.Optional;
import java.util.UUID;
import java.util.concurrent.TimeUnit;

/**
 * A client for the controller's {@code NetworkGateway} service, for components
 * that live outside the proxies — queue systems, matchmakers, gameserver
 * orchestrators — and need to find players or move them.
 *
 * <p>This is not the proxy-side API. A proxy uses {@link NetworkGateway}, which
 * holds an open stream to the controller; this client makes ordinary calls and
 * keeps no state of its own.
 *
 * <p>The controller serves this only on its elected leader, so a channel built
 * against the Kubernetes Service reconnects by itself across a failover.
 */
public final class NetworkGatewayClient implements AutoCloseable {

    /** How long a call waits before giving up, when no explicit wait is asked for. */
    private static final Duration DEFAULT_DEADLINE = Duration.ofSeconds(30);

    /** Headroom over the requested wait, to let the last moves report back. */
    private static final Duration DEADLINE_HEADROOM = Duration.ofSeconds(10);

    private final NetworkGatewayGrpc.NetworkGatewayBlockingStub stub;
    private final ManagedChannel owned;

    private NetworkGatewayClient(Channel channel, ManagedChannel owned) {
        this.stub = NetworkGatewayGrpc.newBlockingStub(channel);
        this.owned = owned;
    }

    /**
     * Connects to the controller at {@code host:port}, typically
     * {@code minecraft-gateway-network-xds.<namespace>:19000}. The returned
     * client owns the channel and shuts it down when closed.
     */
    public static NetworkGatewayClient forTarget(String target) {
        ManagedChannel channel = ManagedChannelBuilder.forTarget(target).usePlaintext().build();
        return new NetworkGatewayClient(channel, channel);
    }

    /** Uses a channel the caller manages; closing this client leaves it open. */
    public static NetworkGatewayClient using(Channel channel) {
        return new NetworkGatewayClient(channel, null);
    }

    // ---------------------------------------------------------------- queries

    /** Where a player currently is, if they are online. */
    public Optional<PlayerConnection> connectionOf(UUID playerUuid) {
        Api.GetConnectionResponse response = stub.getConnection(Api.GetConnectionRequest.newBuilder()
                .setPlayerUuid(playerUuid.toString())
                .build());
        return response.hasConnection()
                ? Optional.of(PlayerConnection.of(response.getConnection()))
                : Optional.empty();
    }

    /** Every player on one backend server. */
    public List<PlayerConnection> playersOnServer(String serverName) {
        return stub.getPlayersForServer(Api.GetPlayersForServerRequest.newBuilder()
                        .setServerName(serverName).build())
                .getConnectionsList().stream().map(PlayerConnection::of).toList();
    }

    /** Every player across the servers of one Service. */
    public List<PlayerConnection> playersOnService(String namespace, String name) {
        return stub.getPlayersForService(Api.GetPlayersForServiceRequest.newBuilder()
                        .setNamespace(namespace).setName(name).build())
                .getConnectionsList().stream().map(PlayerConnection::of).toList();
    }

    // ------------------------------------------------------------------ moves

    /**
     * Connects players to a server, blocking until the proxies report the
     * outcome. Players who are not online fail immediately as
     * {@code PlayerNotConnected}; use
     * {@link #movePlayers(GatewayListener, Collection, MoveTarget, MovePolicy, Duration)}
     * to wait for them instead.
     */
    public MoveResult movePlayers(GatewayListener listener, Collection<UUID> players, MoveTarget target) {
        return movePlayers(listener, players, target, MovePolicy.ALL_OR_NOTHING, Duration.ZERO);
    }

    /**
     * Connects players to a server, waiting up to {@code wait} for players who
     * are not online yet — the same pre-staging a PlayerTransfer's {@code ttl}
     * allows.
     *
     * <p>The call blocks until every player has an outcome. If its deadline
     * fires first the call throws, and any move already handed to a proxy is
     * still carried out: query {@link #connectionOf} to see where players ended
     * up.
     */
    public MoveResult movePlayers(GatewayListener listener,
                                  Collection<UUID> players,
                                  MoveTarget target,
                                  MovePolicy policy,
                                  Duration wait) {
        if (players.isEmpty()) {
            throw new IllegalArgumentException("players must not be empty");
        }

        Api.MovePlayersRequest.Builder request = Api.MovePlayersRequest.newBuilder()
                .setGatewayNamespace(listener.namespace())
                .setGatewayName(listener.gatewayName())
                .setListenerName(listener.listenerName())
                .setPolicy(policy == MovePolicy.BEST_EFFORT
                        ? Api.MovePolicy.MOVE_POLICY_BEST_EFFORT
                        : Api.MovePolicy.MOVE_POLICY_ALL_OR_NOTHING)
                .setWaitSeconds((int) wait.toSeconds());
        players.forEach(player -> request.addPlayers(player.toString()));
        target.applyTo(request);

        // Outlive the wait the controller was asked for, or the deadline would
        // cut the call off before it could report anything.
        Duration deadline = wait.isZero() ? DEFAULT_DEADLINE : wait.plus(DEADLINE_HEADROOM);
        Api.MovePlayersResponse response = stub
                .withDeadlineAfter(deadline.toMillis(), TimeUnit.MILLISECONDS)
                .movePlayers(request.build());

        return new MoveResult(response.getResultsList().stream()
                .map(result -> new MoveResult.PlayerMove(
                        UUID.fromString(result.getPlayerUuid()),
                        result.getServerName().isEmpty() ? Optional.empty() : Optional.of(result.getServerName()),
                        result.getSuccess(),
                        result.getReason().isEmpty() ? Optional.empty() : Optional.of(result.getReason())))
                .toList());
    }

    /** How players who are not connected are handled. */
    public enum MovePolicy {
        /** Move nobody until every player is present. */
        ALL_OR_NOTHING,
        /** Move each player as they become present; fail the rest individually. */
        BEST_EFFORT
    }

    /** The gateway listener a call addresses. */
    public record GatewayListener(String namespace, String gatewayName, String listenerName) {
    }

    /** Where one player is, as the controller sees it. */
    public record PlayerConnection(UUID playerUuid, String proxyId, String serverName,
                                   GatewayListener listener) {
        static PlayerConnection of(Api.PlayerConnection proto) {
            return new PlayerConnection(
                    UUID.fromString(proto.getPlayerUuid()),
                    proto.getProxyId(),
                    proto.getServerName(),
                    new GatewayListener(proto.getGatewayNamespace(), proto.getGatewayName(),
                            proto.getListenerName()));
        }
    }

    @Override
    public void close() {
        if (owned != null) {
            owned.shutdownNow();
        }
    }
}
