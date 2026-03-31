package dev.minefleet.api.gateway.networking.snapshot;

import dev.minefleet.api.gateway.networking.ManagedServer;
import dev.minefleet.api.gateway.networking.ManagedService;

import java.util.List;

public record NetworkSnapshotDelta(List<ManagedService> addedOrUpdatedServices, List<ManagedServer> addedOrUpdatedServers, List<ManagedService> removedServices, List<ManagedServer> removedServers) {
}
