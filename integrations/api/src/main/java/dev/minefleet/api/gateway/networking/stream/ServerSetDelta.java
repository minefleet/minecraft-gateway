package dev.minefleet.api.gateway.networking.stream;

import dev.minefleet.api.gateway.networking.ManagedServer;

import java.util.ArrayList;
import java.util.List;
import java.util.Map;

/**
 * The change between the server set a proxy has registered and the one the
 * controller just pushed. The controller always sends the full set, so the
 * proxy derives what to register and what to drop.
 */
public record ServerSetDelta(List<ManagedServer> addedOrUpdated, List<ManagedServer> removed) {

    public static ServerSetDelta between(Map<String, ManagedServer> previous, List<ManagedServer> current) {
        List<ManagedServer> addedOrUpdated = new ArrayList<>();
        for (ManagedServer server : current) {
            ManagedServer existing = previous.get(server.uniqueId());
            if (existing == null || !existing.toProto().equals(server.toProto())) {
                addedOrUpdated.add(server);
            }
        }

        List<ManagedServer> removed = new ArrayList<>();
        for (Map.Entry<String, ManagedServer> entry : previous.entrySet()) {
            if (current.stream().noneMatch(s -> s.uniqueId().equals(entry.getKey()))) {
                removed.add(entry.getValue());
            }
        }

        return new ServerSetDelta(List.copyOf(addedOrUpdated), List.copyOf(removed));
    }

    public boolean isEmpty() {
        return addedOrUpdated.isEmpty() && removed.isEmpty();
    }
}
