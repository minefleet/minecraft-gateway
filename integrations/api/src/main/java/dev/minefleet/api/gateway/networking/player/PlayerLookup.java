package dev.minefleet.api.gateway.networking.player;

import java.util.Optional;
import java.util.UUID;

/**
 * Finds a connected player by UUID. Move commands address players by UUID, so
 * the platform integration must be able to resolve one.
 */
@FunctionalInterface
public interface PlayerLookup {
    Optional<NetworkPlayer> byUuid(UUID uuid);
}
