package dev.minefleet.api.gateway.networking.player;

import dev.minefleet.api.gateway.networking.ManagedServer;

import java.util.Optional;
import java.util.UUID;

public interface NetworkPlayer {
    UUID getUuid();

    String getConnectedDomain();

    Optional<ManagedServer> getConnectedServer();

    boolean hasPermission(String permission);

    void connectToServer(ManagedServer server);

    void kick(KickReason reason);
}
