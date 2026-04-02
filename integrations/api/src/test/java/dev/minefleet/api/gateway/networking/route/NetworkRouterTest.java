package dev.minefleet.api.gateway.networking.route;

import dev.minefleet.api.gateway.networking.player.KickReason;
import dev.minefleet.api.gateway.networking.player.NetworkPlayer;
import dev.minefleet.api.gateway.networking.snapshot.NetworkSnapshot;
import dev.minefleet.api.gateway.networking.v1alpha1.Types;
import org.junit.jupiter.api.Test;
import org.junit.jupiter.api.extension.ExtendWith;
import org.mockito.ArgumentCaptor;
import org.mockito.Mock;
import org.mockito.junit.jupiter.MockitoExtension;

import java.util.List;
import java.util.Random;

import static org.junit.jupiter.api.Assertions.assertEquals;
import static org.mockito.Mockito.*;

@ExtendWith(MockitoExtension.class)
class NetworkRouterTest {

    @Mock NetworkPlayer<?> player;

    // --- Builders ---

    private Types.ManagedServer server(String uniqueId) {
        return Types.ManagedServer.newBuilder().setUniqueId(uniqueId).setName(uniqueId).build();
    }

    private Types.ManagedServer serverWithPlayers(String uniqueId, int current) {
        return Types.ManagedServer.newBuilder().setUniqueId(uniqueId).setName(uniqueId).setCurrentPlayers(current).build();
    }

    private Types.Route joinRoute(int priority) {
        return Types.Route.newBuilder().setIsJoin(true).setPriority(priority).build();
    }

    private Types.Route fallbackRoute(int priority) {
        return Types.Route.newBuilder().setIsFallback(true).setPriority(priority).build();
    }

    private Types.Route joinRouteWithDomain(int priority, String domain) {
        return Types.Route.newBuilder()
                .setIsJoin(true).setPriority(priority)
                .addRules(Types.OptionRuleSet.newBuilder()
                        .setType(Types.RuleType.ALL)
                        .addRules(Types.Rule.newBuilder().setDomain(domain)))
                .build();
    }

    private Types.ManagedService service(String name, Types.DistributionStrategy strategy,
                                         List<Types.Route> routes, List<Types.ManagedServer> servers) {
        return Types.ManagedService.newBuilder()
                .setNamespacedName("ns/" + name).setNamespace("ns").setName(name)
                .setDistributionStrategy(strategy)
                .addAllRoutes(routes)
                .addAllServers(servers)
                .build();
    }

    private NetworkRouter routerWith(Types.ManagedService... services) {
        var snapshot = new NetworkSnapshot(Types.Snapshot.newBuilder()
                .setCurrentGeneration("v1")
                .addAllServices(List.of(services))
                .build());
        return new NetworkRouter(snapshot);
    }

    // --- routeJoin ---

    @Test
    void routeJoin_noServices_kicksWithNoJoin() {
        routerWith().routeJoin(player);
        verify(player).kick(KickReason.NO_JOIN);
        verify(player, never()).connectToServer(any());
    }

    @Test
    void routeJoin_noJoinServices_kicksWithNoJoin() {
        var svc = service("svc", Types.DistributionStrategy.RANDOM,
                List.of(fallbackRoute(1)), List.of(server("s1")));
        routerWith(svc).routeJoin(player);
        verify(player).kick(KickReason.NO_JOIN);
    }

    @Test
    void routeJoin_matchingService_connectsToServer() {
        var svc = service("svc", Types.DistributionStrategy.RANDOM,
                List.of(joinRoute(1)), List.of(server("s1")));
        routerWith(svc).routeJoin(player);
        verify(player).connectToServer(any());
        verify(player, never()).kick(any());
    }

    @Test
    void routeJoin_routeFailsAvailability_kicksWithNoJoin() {
        // no servers → AvailabilityRule fails → route doesn't match
        var svc = service("svc", Types.DistributionStrategy.RANDOM,
                List.of(joinRoute(1)), List.of());
        routerWith(svc).routeJoin(player);
        verify(player).kick(KickReason.NO_JOIN);
    }

    @Test
    void routeJoin_domainRuleMismatch_kicksWithNoJoin() {
        when(player.connectedDomain()).thenReturn("other.example.com");
        var svc = service("svc", Types.DistributionStrategy.RANDOM,
                List.of(joinRouteWithDomain(1, "play.example.com")), List.of(server("s1")));
        routerWith(svc).routeJoin(player);
        verify(player).kick(KickReason.NO_JOIN);
    }

    @Test
    void routeJoin_domainRuleMatches_connectsToServer() {
        when(player.connectedDomain()).thenReturn("play.example.com");
        var svc = service("svc", Types.DistributionStrategy.RANDOM,
                List.of(joinRouteWithDomain(1, "play.example.com")), List.of(server("s1")));
        routerWith(svc).routeJoin(player);
        verify(player).connectToServer(any());
    }

    @Test
    void routeJoin_multipleServices_lowestPriorityWins() {
        var low = service("low-priority", Types.DistributionStrategy.RANDOM,
                List.of(joinRoute(10)), List.of(server("s-low")));
        var high = service("high-priority", Types.DistributionStrategy.RANDOM,
                List.of(joinRoute(1)), List.of(server("s-high")));

        var captor = ArgumentCaptor.forClass(dev.minefleet.api.gateway.networking.ManagedServer.class);
        routerWith(low, high).routeJoin(player);
        verify(player).connectToServer(captor.capture());
        assertEquals("s-high", captor.getValue().uniqueId());
    }

    @Test
    void routeJoin_leastPlayers_picksServerWithFewestPlayers() {
        var svc = service("svc", Types.DistributionStrategy.LEAST_PLAYERS,
                List.of(joinRoute(1)),
                List.of(serverWithPlayers("busy", 10), serverWithPlayers("free", 2)));

        var captor = ArgumentCaptor.forClass(dev.minefleet.api.gateway.networking.ManagedServer.class);
        routerWith(svc).routeJoin(player);
        verify(player).connectToServer(captor.capture());
        assertEquals("free", captor.getValue().uniqueId());
    }

    @Test
    void routeJoin_leastPlayers_noPlayerData_fallsBackToFirst() {
        // servers without currentPlayers set → filter removes them → falls back to getFirst()
        var svc = service("svc", Types.DistributionStrategy.LEAST_PLAYERS,
                List.of(joinRoute(1)),
                List.of(server("s1"), server("s2")));

        var captor = ArgumentCaptor.forClass(dev.minefleet.api.gateway.networking.ManagedServer.class);
        routerWith(svc).routeJoin(player);
        verify(player).connectToServer(captor.capture());
        assertEquals("s1", captor.getValue().uniqueId());
    }

    @Test
    void routeJoin_random_picksServerAtMockedIndex() {
        var svc = service("svc", Types.DistributionStrategy.RANDOM,
                List.of(joinRoute(1)),
                List.of(server("s1"), server("s2"), server("s3")));

        try (var ignored = mockConstruction(Random.class, (mock, ctx) -> when(mock.nextInt(3)).thenReturn(2))) {
            var captor = ArgumentCaptor.forClass(dev.minefleet.api.gateway.networking.ManagedServer.class);
            routerWith(svc).routeJoin(player);
            verify(player).connectToServer(captor.capture());
            assertEquals("s3", captor.getValue().uniqueId());
        }
    }

    // --- routeFallback ---

    @Test
    void routeFallback_noFallbackServices_kicksWithNoFallback() {
        var svc = service("svc", Types.DistributionStrategy.RANDOM,
                List.of(joinRoute(1)), List.of(server("s1")));
        routerWith(svc).routeFallback(player);
        verify(player).kick(KickReason.NO_FALLBACK);
    }

    @Test
    void routeFallback_matchingService_connectsToServer() {
        var svc = service("svc", Types.DistributionStrategy.RANDOM,
                List.of(fallbackRoute(1)), List.of(server("s1")));
        routerWith(svc).routeFallback(player);
        verify(player).connectToServer(any());
        verify(player, never()).kick(any());
    }

    @Test
    void routeFallback_routeFailsAvailability_kicksWithNoFallback() {
        var svc = service("svc", Types.DistributionStrategy.RANDOM,
                List.of(fallbackRoute(1)), List.of());
        routerWith(svc).routeFallback(player);
        verify(player).kick(KickReason.NO_FALLBACK);
    }
}