package dev.minefleet.api.gateway.networking;

import dev.minefleet.api.gateway.networking.v1alpha1.Api;
import dev.minefleet.api.gateway.networking.v1alpha1.NetworkGatewayGrpc;
import io.grpc.ManagedChannel;
import io.grpc.Server;
import io.grpc.inprocess.InProcessChannelBuilder;
import io.grpc.inprocess.InProcessServerBuilder;
import io.grpc.stub.StreamObserver;
import org.junit.jupiter.api.AfterEach;
import org.junit.jupiter.api.BeforeEach;
import org.junit.jupiter.api.Test;

import java.time.Duration;
import java.util.List;
import java.util.Map;
import java.util.UUID;
import java.util.concurrent.atomic.AtomicReference;

import static org.junit.jupiter.api.Assertions.assertEquals;
import static org.junit.jupiter.api.Assertions.assertFalse;
import static org.junit.jupiter.api.Assertions.assertThrows;
import static org.junit.jupiter.api.Assertions.assertTrue;

/**
 * Drives the external client against a scripted controller over an in-process
 * channel, checking what it puts on the wire and what it makes of the answer.
 */
class NetworkGatewayClientTest {

    /** A controller that records the request and replies with a canned result. */
    private static class ScriptedGateway extends NetworkGatewayGrpc.NetworkGatewayImplBase {
        final AtomicReference<Api.MovePlayersRequest> received = new AtomicReference<>();
        volatile Api.MovePlayersResponse response = Api.MovePlayersResponse.getDefaultInstance();

        @Override
        public void movePlayers(Api.MovePlayersRequest request,
                                StreamObserver<Api.MovePlayersResponse> observer) {
            received.set(request);
            observer.onNext(response);
            observer.onCompleted();
        }

        @Override
        public void getConnection(Api.GetConnectionRequest request,
                                  StreamObserver<Api.GetConnectionResponse> observer) {
            observer.onNext(Api.GetConnectionResponse.newBuilder()
                    .setConnection(Api.PlayerConnection.newBuilder()
                            .setPlayerUuid(request.getPlayerUuid())
                            .setProxyId("proxy-a")
                            .setServerName("lobby-0")
                            .setGatewayNamespace("default")
                            .setGatewayName("gw")
                            .setListenerName("minecraft"))
                    .build());
            observer.onCompleted();
        }
    }

    private static final UUID ALICE = UUID.fromString("11111111-1111-1111-1111-111111111111");
    private static final UUID BOB = UUID.fromString("22222222-2222-2222-2222-222222222222");
    private static final NetworkGatewayClient.GatewayListener LISTENER =
            new NetworkGatewayClient.GatewayListener("default", "gw", "minecraft");

    private Server grpcServer;
    private ManagedChannel channel;
    private ScriptedGateway controller;
    private NetworkGatewayClient client;

    @BeforeEach
    void setUp() throws Exception {
        String name = InProcessServerBuilder.generateName();
        controller = new ScriptedGateway();
        grpcServer = InProcessServerBuilder.forName(name)
                .directExecutor().addService(controller).build().start();
        channel = InProcessChannelBuilder.forName(name).directExecutor().build();
        client = NetworkGatewayClient.using(channel);
    }

    @AfterEach
    void tearDown() {
        client.close();
        channel.shutdownNow();
        grpcServer.shutdownNow();
    }

    private static Api.PlayerMoveResult moved(UUID player, String server) {
        return Api.PlayerMoveResult.newBuilder()
                .setPlayerUuid(player.toString()).setServerName(server).setSuccess(true).build();
    }

    private static Api.PlayerMoveResult failed(UUID player, String reason) {
        return Api.PlayerMoveResult.newBuilder()
                .setPlayerUuid(player.toString()).setSuccess(false).setReason(reason).build();
    }

    @Test
    void sendsANamedServerTarget() {
        controller.response = Api.MovePlayersResponse.newBuilder()
                .addResults(moved(ALICE, "arena-3")).build();

        MoveResult result = client.movePlayers(LISTENER, List.of(ALICE), MoveTarget.server("arena-3"));

        Api.MovePlayersRequest request = controller.received.get();
        assertEquals("arena-3", request.getServerName());
        assertEquals(List.of(ALICE.toString()), request.getPlayersList());
        assertEquals(Api.MovePolicy.MOVE_POLICY_ALL_OR_NOTHING, request.getPolicy());
        assertEquals(0, request.getWaitSeconds());

        assertTrue(result.allSucceeded());
        assertEquals("arena-3", result.forPlayer(ALICE).orElseThrow().serverName().orElseThrow());
    }

    @Test
    void sendsALabelSelectorTarget() {
        controller.response = Api.MovePlayersResponse.newBuilder()
                .addResults(moved(ALICE, "arena-0")).build();

        client.movePlayers(LISTENER, List.of(ALICE), MoveTarget.matching(Map.of("role", "arena")));

        assertEquals(Map.of("role", "arena"), controller.received.get().getLabelSelector().getMatchLabelsMap());
    }

    @Test
    void sendsARouteModeTarget() {
        controller.response = Api.MovePlayersResponse.newBuilder()
                .addResults(moved(ALICE, "lobby-1")).build();

        client.movePlayers(LISTENER, List.of(ALICE),
                MoveTarget.route(MoveTarget.RouteKind.FALLBACK));

        assertEquals(Api.RouteKind.ROUTE_KIND_FALLBACK, controller.received.get().getRouteMode());
    }

    @Test
    void passesThePolicyAndWaitThrough() {
        controller.response = Api.MovePlayersResponse.newBuilder()
                .addResults(moved(ALICE, "arena-3")).build();

        client.movePlayers(LISTENER, List.of(ALICE), MoveTarget.server("arena-3"),
                NetworkGatewayClient.MovePolicy.BEST_EFFORT, Duration.ofSeconds(30));

        Api.MovePlayersRequest request = controller.received.get();
        assertEquals(Api.MovePolicy.MOVE_POLICY_BEST_EFFORT, request.getPolicy());
        assertEquals(30, request.getWaitSeconds());
    }

    @Test
    void reportsPerPlayerFailures() {
        controller.response = Api.MovePlayersResponse.newBuilder()
                .addResults(moved(ALICE, "arena-3"))
                .addResults(failed(BOB, "PlayerNotConnected"))
                .build();

        MoveResult result = client.movePlayers(LISTENER, List.of(ALICE, BOB), MoveTarget.server("arena-3"));

        assertFalse(result.allSucceeded());
        assertEquals(1, result.failures().size());
        MoveResult.PlayerMove failure = result.failures().getFirst();
        assertEquals(BOB, failure.playerUuid());
        assertEquals("PlayerNotConnected", failure.reason().orElseThrow());
        assertTrue(failure.serverName().isEmpty(), "a player who never moved has no server");
    }

    @Test
    void rejectsAnEmptyPlayerList() {
        assertThrows(IllegalArgumentException.class,
                () -> client.movePlayers(LISTENER, List.of(), MoveTarget.server("arena-3")));
    }

    @Test
    void readsWhereAPlayerIs() {
        var connection = client.connectionOf(ALICE).orElseThrow();
        assertEquals("lobby-0", connection.serverName());
        assertEquals(LISTENER, connection.listener());
    }
}
