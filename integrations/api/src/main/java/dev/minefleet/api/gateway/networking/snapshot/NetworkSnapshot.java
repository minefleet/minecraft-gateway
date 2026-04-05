package dev.minefleet.api.gateway.networking.snapshot;

import dev.minefleet.api.gateway.networking.ManagedServer;
import dev.minefleet.api.gateway.networking.ManagedService;
import dev.minefleet.api.gateway.networking.v1alpha1.Types;

import java.util.*;
import java.util.stream.Collectors;

public class NetworkSnapshot {

    private final String version;
    private final Map<ManagedService, List<ManagedServer>> services = new HashMap<>();

    public NetworkSnapshot(Types.Snapshot proto) {
        version = proto.getCurrentGeneration();
        proto.getServicesList().forEach(service -> {
            var parsedService = new ManagedService(service);
            var parsedServers = service.getServersList().stream()
                    .map((Types.ManagedServer serverProto) -> new ManagedServer(parsedService.namespacedName(), serverProto))
                    .toList();
            services.put(parsedService, parsedServers);
        });
    }

    public String version() {
        return version;
    }

    public Map<ManagedService, List<ManagedServer>> services() {
        return services;
    }

    public NetworkSnapshotServerDelta serverSnapshotDelta(NetworkSnapshot lastSnapshot) {
        if(lastSnapshot == null || lastSnapshot.version().equals(version)) {
            // No snapshot before, create servers
            return new NetworkSnapshotServerDelta(services.values().stream().flatMap(Collection::stream).toList(), Collections.emptyList());
        }
        Map<String, ManagedServer> prevServers = lastSnapshot.services().values().stream().flatMap(Collection::stream)
                .collect(Collectors.toMap(ManagedServer::uniqueId, s -> s));

        List<ManagedServer> addedOrUpdatedServers = new ArrayList<>();

        for (ManagedService service : services.keySet()) {
            for (ManagedServer server : services.get(service)) {
                ManagedServer prevServer = prevServers.get(server.uniqueId());
                if (prevServer == null || !prevServer.toProto().equals(server.toProto())) {
                    addedOrUpdatedServers.add(server);
                }
            }
        }
        Map<String, ManagedServer> currentServers = services.keySet().stream()
                .flatMap(s -> services.get(s).stream())
                .collect(Collectors.toMap(ManagedServer::uniqueId, s -> s));

        List<ManagedServer> removedServers = prevServers.values().stream()
                .filter(s -> !currentServers.containsKey(s.uniqueId()))
                .toList();

        return new NetworkSnapshotServerDelta(addedOrUpdatedServers, removedServers);
    }
}
