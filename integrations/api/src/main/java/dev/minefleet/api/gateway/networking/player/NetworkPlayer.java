package dev.minefleet.api.gateway.networking.player;

import dev.minefleet.api.gateway.networking.ManagedServer;

import java.util.Optional;

public interface NetworkPlayer<P> {
    String connectedDomain();

    Optional<ManagedServer> connectedServer();

    boolean hasPermission(String permission);

    void connectToServer(ManagedServer server);

    void kick(KickReason reason);
}
