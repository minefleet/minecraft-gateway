package dev.minefleet.api.gateway.networking;

import dev.minefleet.api.gateway.networking.rules.RuleSet;
import dev.minefleet.api.gateway.networking.v1alpha1.Types;

import java.util.List;

public final class ManagedService {

    public record RouteEntry(int priority, RuleSet rules) {
    }

    private final Types.ManagedService proto;
    private final List<RouteEntry> joinRoutes;
    private final List<RouteEntry> fallbackRoutes;
    private final int availableServersAmount;

    public ManagedService(Types.ManagedService proto) {
        this.proto = proto;
        this.availableServersAmount = (int) proto.getServersList().stream()
                .filter(s -> !(s.hasMaxPlayers() && s.hasCurrentPlayers() && s.getCurrentPlayers() >= s.getMaxPlayers()))
                .count();
        this.joinRoutes = proto.getRoutesList().stream()
                .filter(Types.Route::getIsJoin)
                .map(r -> new RouteEntry(r.getPriority(), new RuleSet(r.getRulesList())))
                .toList();
        this.fallbackRoutes = proto.getRoutesList().stream()
                .filter(Types.Route::getIsFallback)
                .map(r -> new RouteEntry(r.getPriority(), new RuleSet(r.getRulesList())))
                .toList();
    }

    public String namespacedName() {
        return proto.getNamespacedName();
    }

    public String namespace() {
        return proto.getNamespace();
    }

    public String name() {
        return proto.getName();
    }

    public int availableServersAmount() {
        return availableServersAmount;
    }

    public boolean isJoinService() {
        return !joinRoutes.isEmpty();
    }

    public List<RouteEntry> joinRoutes() {
        return joinRoutes;
    }

    public boolean isFallbackService() {
        return !fallbackRoutes.isEmpty();
    }

    public List<RouteEntry> fallbackRoutes() {
        return fallbackRoutes;
    }

    public Types.DistributionStrategy distributionStrategy() {
        return proto.getDistributionStrategy();
    }

    public Types.ManagedService toProto() {
        return proto;
    }
}