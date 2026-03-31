package dev.minefleet.api.gateway.networking.snapshot;

import dev.minefleet.api.gateway.networking.ManagedServer;
import dev.minefleet.api.gateway.networking.ManagedService;

import java.util.ArrayList;
import java.util.List;
import java.util.Map;
import java.util.stream.Collectors;

public record NetworkSnapshot(long version, List<ManagedService> services) {
    public NetworkSnapshotDelta delta(NetworkSnapshot lastSnapshot) {
        Map<String, ManagedService> prevServices = lastSnapshot.services().stream()
                .collect(Collectors.toMap(ManagedService::namespacedName, s -> s));
        Map<String, ManagedServer> prevServers = lastSnapshot.services().stream()
                .flatMap(s -> s.servers().stream())
                .collect(Collectors.toMap(ManagedServer::uniqueId, s -> s));

        List<ManagedService> addedOrUpdatedServices = new ArrayList<>();
        List<ManagedServer> addedOrUpdatedServers = new ArrayList<>();

        for (ManagedService service : services) {
            ManagedService prev = prevServices.get(service.namespacedName());
            if (prev == null || !prev.toProto().equals(service.toProto())) {
                addedOrUpdatedServices.add(service);
            }
            for (ManagedServer server : service.servers()) {
                ManagedServer prevServer = prevServers.get(server.uniqueId());
                if (prevServer == null || !prevServer.toProto().equals(server.toProto())) {
                    addedOrUpdatedServers.add(server);
                }
            }
        }

        Map<String, ManagedService> currentServices = services.stream()
                .collect(Collectors.toMap(ManagedService::namespacedName, s -> s));
        Map<String, ManagedServer> currentServers = services.stream()
                .flatMap(s -> s.servers().stream())
                .collect(Collectors.toMap(ManagedServer::uniqueId, s -> s));

        List<ManagedService> removedServices = lastSnapshot.services().stream()
                .filter(s -> !currentServices.containsKey(s.namespacedName()))
                .toList();
        List<ManagedServer> removedServers = prevServers.values().stream()
                .filter(s -> !currentServers.containsKey(s.uniqueId()))
                .toList();

        return new NetworkSnapshotDelta(addedOrUpdatedServices, addedOrUpdatedServers, removedServices, removedServers);
    }
}
