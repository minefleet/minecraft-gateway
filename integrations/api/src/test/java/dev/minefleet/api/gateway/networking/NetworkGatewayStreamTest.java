package dev.minefleet.api.gateway.networking;

import dev.minefleet.api.gateway.networking.player.KickReason;
import dev.minefleet.api.gateway.networking.player.NetworkPlayer;
import dev.minefleet.api.gateway.networking.v1alpha1.Api;
import dev.minefleet.api.gateway.networking.v1alpha1.NetworkXDSGrpc;
import dev.minefleet.api.gateway.networking.v1alpha1.Types;
import io.grpc.ManagedChannel;
import io.grpc.Server;
import io.grpc.inprocess.InProcessChannelBuilder;
import io.grpc.inprocess.InProcessServerBuilder;
import io.grpc.stub.StreamObserver;
import org.junit.jupiter.api.AfterEach;
import org.junit.jupiter.api.BeforeEach;
import org.junit.jupiter.api.Test;

import java.util.List;
import java.util.Optional;
import java.util.UUID;
import java.util.concurrent.BlockingQueue;
import java.util.concurrent.ConcurrentHashMap;
import java.util.concurrent.LinkedBlockingQueue;
import java.util.concurrent.TimeUnit;
import java.util.concurrent.atomic.AtomicReference;

import static org.junit.jupiter.api.Assertions.assertEquals;
import static org.junit.jupiter.api.Assertions.assertNotNull;
import static org.junit.jupiter.api.Assertions.assertNull;
import static org.junit.jupiter.api.Assertions.assertTrue;

/**
 * Drives the gateway against a scripted controller over an in-process stream.
 */
class NetworkGatewayStreamTest {

    /** A controller that records what the proxy sends and replies on demand. */
    private static class ScriptedController extends NetworkXDSGrpc.NetworkXDSImplBase {
        final BlockingQueue<Api.ProxyMessage> received = new LinkedBlockingQueue<>();
        final AtomicReference<StreamObserver<Api.ControllerMessage>> outbound = new AtomicReference<>();

        @Override
        public StreamObserver<Api.ProxyMessage> connect(StreamObserver<Api.ControllerMessage> responseObserver) {
            outbound.set(responseObserver);
            return new StreamObserver<>() {
                @Override
                public void onNext(Api.ProxyMessage message) {
                    received.add(message);
                }

                @Override
                public void onError(Throwable t) {
                }

                @Override
                public void onCompleted() {
                    responseObserver.onCompleted();
                }
            };
        }

        void send(Api.ControllerMessage message) {
            outbound.get().onNext(message);
        }

        Api.ProxyMessage awaitMessage(Api.ProxyMessage.MessageCase kind) throws InterruptedException {
            long deadline = System.nanoTime() + TimeUnit.SECONDS.toNanos(5);
            while (System.nanoTime() < deadline) {
                Api.ProxyMessage message = received.poll(200, TimeUnit.MILLISECONDS);
                if (message != null && message.getMessageCase() == kind) {
                    return message;
                }
            }
            throw new AssertionError("timed out waiting for " + kind);
        }
    }

    /** A registrar that just remembers what it was told to register. */
    private static class RecordingRegistrar implements ServerRegistrar {
        final ConcurrentHashMap<String, ManagedServer> servers = new ConcurrentHashMap<>();

        @Override
        public void registerOrUpdate(ManagedServer server) {
            servers.put(server.name(), server);
        }

        @Override
        public void unregister(ManagedServer server) {
            servers.remove(server.name());
        }

        @Override
        public Optional<ManagedServer> findByName(String name) {
            return Optional.ofNullable(servers.get(name));
        }
    }

    /** A player that records where it was sent or why it was kicked. */
    private static class FakePlayer implements NetworkPlayer {
        final UUID uuid = UUID.randomUUID();
        volatile ManagedServer connectedTo;
        volatile KickReason kickedWith;

        @Override
        public UUID getUuid() {
            return uuid;
        }

        @Override
        public String getConnectedDomain() {
            return "play.example.com";
        }

        @Override
        public Optional<ManagedServer> getConnectedServer() {
            return Optional.empty();
        }

        @Override
        public boolean hasPermission(String permission) {
            return "vip".equals(permission);
        }

        @Override
        public void connectToServer(ManagedServer server) {
            connectedTo = server;
        }

        @Override
        public void kick(KickReason reason) {
            kickedWith = reason;
        }
    }

    private String serverName;
    private Server grpcServer;
    private ManagedChannel channel;
    private ScriptedController controller;
    private RecordingRegistrar registrar;
    private NetworkGateway gateway;

    @BeforeEach
    void setUp() throws Exception {
        serverName = InProcessServerBuilder.generateName();
        controller = new ScriptedController();
        grpcServer = InProcessServerBuilder.forName(serverName)
                .directExecutor().addService(controller).build().start();
        channel = InProcessChannelBuilder.forName(serverName).directExecutor().build();

        registrar = new RecordingRegistrar();
        gateway = NetworkGateway.builder()
                .channel(channel)
                .context(new NetworkContext("proxy-a", "default", "gw", "minecraft"))
                .registrar(registrar)
                .build();
    }

    @AfterEach
    void tearDown() {
        gateway.stop();
        channel.shutdownNow();
        grpcServer.shutdownNow();
    }

    private static Api.ControllerMessage serverSync(String... permissions) {
        return Api.ControllerMessage.newBuilder()
                .setServerSync(Api.ServerSync.newBuilder()
                        .addServers(Types.ManagedServer.newBuilder()
                                .setUniqueId("lobby-0").setName("lobby-0")
                                .setIp("10.0.0.1").setPort(25565))
                        .addAllRequiredPermissions(List.of(permissions)))
                .build();
    }

    @Test
    void identifiesItselfAndReplaysPresenceOnConnect() throws Exception {
        gateway.start();

        Api.ProxyMessage hello = controller.awaitMessage(Api.ProxyMessage.MessageCase.HELLO);
        assertEquals("proxy-a", hello.getHello().getProxyId());
        assertEquals("gw", hello.getHello().getGatewayName());
        assertEquals("minecraft", hello.getHello().getListenerName());

        // The replay follows immediately so the controller can rebuild presence.
        controller.awaitMessage(Api.ProxyMessage.MessageCase.PRESENCE_SNAPSHOT);
    }

    @Test
    void registersServersPushedByTheController() throws Exception {
        gateway.start();
        controller.awaitMessage(Api.ProxyMessage.MessageCase.HELLO);

        controller.send(serverSync());

        assertNotNull(registrar.servers.get("lobby-0"));
        assertEquals(25565, registrar.servers.get("lobby-0").port());
    }

    @Test
    void unregistersServersDroppedFromThePush() throws Exception {
        gateway.start();
        controller.awaitMessage(Api.ProxyMessage.MessageCase.HELLO);
        controller.send(serverSync());

        controller.send(Api.ControllerMessage.newBuilder()
                .setServerSync(Api.ServerSync.getDefaultInstance()).build());

        assertTrue(registrar.servers.isEmpty(), "a server absent from the push should be unregistered");
    }

    @Test
    void rejectsJoinsWhileTheStreamIsDown() {
        // The gateway was never started, so the controller is unreachable.
        FakePlayer player = new FakePlayer();

        gateway.routeJoin(player).join();

        assertEquals(KickReason.ROUTING_UNAVAILABLE, player.kickedWith,
                "joins must be rejected rather than routed against stale state");
        assertNull(player.connectedTo);
    }

    @Test
    void connectsThePlayerToTheServerTheControllerChose() throws Exception {
        gateway.start();
        controller.awaitMessage(Api.ProxyMessage.MessageCase.HELLO);
        controller.send(serverSync("vip"));

        FakePlayer player = new FakePlayer();
        var routed = gateway.routeJoin(player);

        Api.ProxyMessage request = controller.awaitMessage(Api.ProxyMessage.MessageCase.ROUTE_REQUEST);
        assertEquals(player.uuid.toString(), request.getRouteRequest().getPlayerUuid());
        assertEquals("play.example.com", request.getRouteRequest().getContext().getConnectedDomain());
        assertTrue(request.getRouteRequest().getContext().getPermissionsMap().get("vip"),
                "the proxy evaluates the permissions the controller asked for");

        controller.send(Api.ControllerMessage.newBuilder()
                .setRouteResponse(Api.RouteResponse.newBuilder()
                        .setCorrelationId(request.getRouteRequest().getCorrelationId())
                        .setResult(Api.RouteResponse.Result.RESULT_OK)
                        .setServerName("lobby-0"))
                .build());
        routed.join();

        assertNotNull(player.connectedTo);
        assertEquals("lobby-0", player.connectedTo.name());
    }

    @Test
    void kicksWhenNoRouteMatches() throws Exception {
        gateway.start();
        controller.awaitMessage(Api.ProxyMessage.MessageCase.HELLO);
        controller.send(serverSync());

        FakePlayer player = new FakePlayer();
        var routed = gateway.routeJoin(player);

        Api.ProxyMessage request = controller.awaitMessage(Api.ProxyMessage.MessageCase.ROUTE_REQUEST);
        controller.send(Api.ControllerMessage.newBuilder()
                .setRouteResponse(Api.RouteResponse.newBuilder()
                        .setCorrelationId(request.getRouteRequest().getCorrelationId())
                        .setResult(Api.RouteResponse.Result.RESULT_NO_ROUTE))
                .build());
        routed.join();

        assertEquals(KickReason.NO_JOIN, player.kickedWith);
    }

    @Test
    void executesMoveCommandsAndReportsSuccess() throws Exception {
        gateway.start();
        controller.awaitMessage(Api.ProxyMessage.MessageCase.HELLO);
        controller.send(serverSync());

        FakePlayer player = new FakePlayer();
        gateway.setPlayerLookup(uuid -> uuid.equals(player.uuid) ? Optional.of(player) : Optional.empty());

        controller.send(Api.ControllerMessage.newBuilder()
                .setMove(Api.MoveCommand.newBuilder()
                        .setCommandId("default/transfer#" + player.uuid)
                        .setPlayerUuid(player.uuid.toString())
                        .setServerName("lobby-0"))
                .build());

        Api.ProxyMessage result = controller.awaitMessage(Api.ProxyMessage.MessageCase.MOVE_RESULT);
        assertTrue(result.getMoveResult().getSuccess());
        assertEquals("lobby-0", player.connectedTo.name());
    }

    @Test
    void reportsMoveFailureWhenThePlayerIsGone() throws Exception {
        gateway.start();
        controller.awaitMessage(Api.ProxyMessage.MessageCase.HELLO);
        controller.send(serverSync());
        gateway.setPlayerLookup(uuid -> Optional.empty());

        controller.send(Api.ControllerMessage.newBuilder()
                .setMove(Api.MoveCommand.newBuilder()
                        .setCommandId("default/transfer#" + UUID.randomUUID())
                        .setPlayerUuid(UUID.randomUUID().toString())
                        .setServerName("lobby-0"))
                .build());

        Api.ProxyMessage result = controller.awaitMessage(Api.ProxyMessage.MessageCase.MOVE_RESULT);
        assertTrue(!result.getMoveResult().getSuccess());
        assertEquals("player_not_found", result.getMoveResult().getReason());
    }

    @Test
    void reportsMoveFailureWhenTheServerIsUnknown() throws Exception {
        gateway.start();
        controller.awaitMessage(Api.ProxyMessage.MessageCase.HELLO);

        FakePlayer player = new FakePlayer();
        gateway.setPlayerLookup(uuid -> Optional.of(player));

        controller.send(Api.ControllerMessage.newBuilder()
                .setMove(Api.MoveCommand.newBuilder()
                        .setCommandId("default/transfer#" + player.uuid)
                        .setPlayerUuid(player.uuid.toString())
                        .setServerName("never-registered"))
                .build());

        Api.ProxyMessage result = controller.awaitMessage(Api.ProxyMessage.MessageCase.MOVE_RESULT);
        assertEquals("server_not_registered", result.getMoveResult().getReason());
    }

    @Test
    void reportsPresenceEvents() throws Exception {
        gateway.start();
        controller.awaitMessage(Api.ProxyMessage.MessageCase.HELLO);
        controller.send(serverSync());

        FakePlayer player = new FakePlayer();
        gateway.reportConnected(player, "lobby-0");

        Api.ProxyMessage event = controller.awaitMessage(Api.ProxyMessage.MessageCase.PRESENCE_EVENT);
        assertEquals(Api.PlayerPresenceEvent.Kind.KIND_CONNECTED, event.getPresenceEvent().getKind());
        assertEquals("lobby-0", event.getPresenceEvent().getServerName());

        gateway.reportDisconnected(player.uuid);
        Api.ProxyMessage disconnect = controller.awaitMessage(Api.ProxyMessage.MessageCase.PRESENCE_EVENT);
        assertEquals(Api.PlayerPresenceEvent.Kind.KIND_DISCONNECTED, disconnect.getPresenceEvent().getKind());
    }
}
