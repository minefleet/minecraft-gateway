package dev.minefleet.api.gateway.networking.snapshot;

import dev.minefleet.api.gateway.networking.ManagedServer;

import java.util.List;

public record NetworkSnapshotServerDelta(List<ManagedServer> addedOrUpdatedServers,
                                         List<ManagedServer> removedServers) {
}
