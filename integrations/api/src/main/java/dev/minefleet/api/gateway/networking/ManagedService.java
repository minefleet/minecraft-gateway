package dev.minefleet.api.gateway.networking;

import dev.minefleet.api.gateway.networking.v1alpha1.Types;

import java.util.List;

public final class ManagedService {

    private final Types.ManagedService proto;
    private final List<ManagedServer> servers;

    public ManagedService(Types.ManagedService proto) {
        this.proto = proto;
        this.servers = proto.getServersList().stream()
                .map((Types.ManagedServer serverProto) -> new ManagedServer(namespacedName(), serverProto))
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

    public Types.DistributionStrategy distributionStrategy() {
        return proto.getDistributionStrategy();
    }

    public List<ManagedServer> servers() {
        return servers;
    }

    public List<Types.OptionRuleSet> ruleSets() {
        return proto.getRuleSetsList();
    }

    public Types.ManagedService toProto() {
        return proto;
    }
}
