package dev.minefleet.api.gateway.networking;

import dev.minefleet.api.gateway.networking.v1alpha1.Types;

import java.util.OptionalInt;

public final class ManagedServer {

    private final Types.ManagedServer proto;
    private final String parentNamespacedName;

    public ManagedServer(String parentNamespacedName, Types.ManagedServer proto) {
        this.proto = proto;
        this.parentNamespacedName = parentNamespacedName;
    }

    public String parentNamespacedName() {
        return parentNamespacedName;
    }

    public String uniqueId() {
        return proto.getUniqueId();
    }

    public String name() {
        return proto.getName();
    }

    public OptionalInt numericalId() {
        return proto.hasNumericalId() ? OptionalInt.of(proto.getNumericalId()) : OptionalInt.empty();
    }

    public OptionalInt maxPlayers() {
        return proto.hasMaxPlayers() ? OptionalInt.of(proto.getMaxPlayers()) : OptionalInt.empty();
    }

    public OptionalInt currentPlayers() {
        return proto.hasCurrentPlayers() ? OptionalInt.of(proto.getCurrentPlayers()) : OptionalInt.empty();
    }

    public Types.ManagedServer toProto() {
        return proto;
    }
}
