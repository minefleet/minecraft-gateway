package dev.minefleet.api.gateway.networking.route;

import dev.minefleet.api.gateway.networking.ManagedServer;
import dev.minefleet.api.gateway.networking.ManagedService;
import dev.minefleet.api.gateway.networking.player.KickReason;
import dev.minefleet.api.gateway.networking.player.NetworkPlayer;
import dev.minefleet.api.gateway.networking.rules.RuleContext;
import dev.minefleet.api.gateway.networking.snapshot.NetworkSnapshot;

import java.util.Comparator;
import java.util.List;
import java.util.Map;
import java.util.Random;
import java.util.function.Function;
import java.util.function.Predicate;

public class NetworkRouter {
    private final NetworkSnapshot snapshot;

    public NetworkRouter(NetworkSnapshot snapshot) {
        this.snapshot = snapshot;
    }

    public void routeJoin(NetworkPlayer networkPlayer) {
        route(networkPlayer, ManagedService::isJoinService, ManagedService::joinRoutes, KickReason.NO_JOIN);
    }

    public void routeFallback(NetworkPlayer networkPlayer) {
        route(networkPlayer, ManagedService::isFallbackService, ManagedService::fallbackRoutes, KickReason.NO_FALLBACK);
    }

    private void route(NetworkPlayer networkPlayer,
                       Predicate<ManagedService> filter,
                       Function<ManagedService, List<ManagedService.RouteEntry>> routeGetter,
                       KickReason kickReason) {
        var target = snapshot.services().keySet().stream()
                .filter(filter)
                .map(service -> Map.entry(service, matchedPriority(service, routeGetter.apply(service), networkPlayer)))
                .filter(e -> e.getValue() >= 0)
                .min(Map.Entry.comparingByValue())
                .map(e -> getRouteTarget(e.getKey()));
        if (target.isPresent()) {
            networkPlayer.connectToServer(target.get());
        } else {
            networkPlayer.kick(kickReason);
        }
    }

    private int matchedPriority(ManagedService service, List<ManagedService.RouteEntry> routes, NetworkPlayer networkPlayer) {
        return routes.stream()
                .filter(r -> r.rules().evaluate(new RuleContext(networkPlayer, service)))
                .mapToInt(ManagedService.RouteEntry::priority)
                .min()
                .orElse(-1);
    }

    private ManagedServer getRouteTarget(ManagedService service) {
        var servers = snapshot.services().get(service);
        if (servers == null || servers.isEmpty()) return null;
        var available = servers.stream().filter(s -> !s.isFull()).toList();
        if (available.isEmpty()) return null;
        return switch (service.distributionStrategy()) {
            case RANDOM -> available.get(new Random().nextInt(available.size()));
            case LEAST_PLAYERS -> available.stream()
                    .filter(s -> s.currentPlayers().isPresent())
                    .min(Comparator.comparingInt(s -> s.currentPlayers().getAsInt()))
                    .orElseGet(available::getFirst);
            default -> null;
        };
    }
}