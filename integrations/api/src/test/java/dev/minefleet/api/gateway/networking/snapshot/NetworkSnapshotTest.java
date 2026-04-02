package dev.minefleet.api.gateway.networking.snapshot;

import dev.minefleet.api.gateway.networking.v1alpha1.Types;
import org.junit.jupiter.api.Test;

import java.util.List;

import static org.junit.jupiter.api.Assertions.*;

class NetworkSnapshotTest {

    // --- Builders ---

    private Types.ManagedServer server(String uniqueId, String name) {
        return Types.ManagedServer.newBuilder()
                .setUniqueId(uniqueId)
                .setName(name)
                .build();
    }

    private Types.ManagedService service(String namespacedName, Types.ManagedServer... servers) {
        return Types.ManagedService.newBuilder()
                .setNamespacedName(namespacedName)
                .setNamespace(namespacedName.split("/")[0])
                .setName(namespacedName.split("/")[1])
                .addAllServers(List.of(servers))
                .build();
    }

    private NetworkSnapshot snapshot(String version, Types.ManagedService... services) {
        return new NetworkSnapshot(Types.Snapshot.newBuilder()
                .setCurrentGeneration(version)
                .addAllServices(List.of(services))
                .build());
    }

    // --- version() ---

    @Test
    void version_returnsCurrentGeneration() {
        assertEquals("gen-1", snapshot("gen-1").version());
    }

    // --- services() ---

    @Test
    void services_parsesServicesAndServers() {
        var snap = snapshot("v1",
                service("ns/svc-a", server("s1", "server-1"), server("s2", "server-2")));

        assertEquals(1, snap.services().size());
        var entry = snap.services().entrySet().iterator().next();
        assertEquals("ns/svc-a", entry.getKey().namespacedName());
        assertEquals(2, entry.getValue().size());
        assertEquals("s1", entry.getValue().get(0).uniqueId());
        assertEquals("s2", entry.getValue().get(1).uniqueId());
    }

    @Test
    void services_serversInheritParentNamespacedName() {
        var snap = snapshot("v1", service("ns/svc-a", server("s1", "server-1")));
        var servers = snap.services().values().iterator().next();
        assertEquals("ns/svc-a", servers.getFirst().parentNamespacedName());
    }

    // --- serverSnapshotDelta(null) ---

    @Test
    void delta_nullPrev_allServersAreAdded() {
        var snap = snapshot("v1",
                service("ns/svc-a", server("s1", "server-1"), server("s2", "server-2")));
        var delta = snap.serverSnapshotDelta(null);

        assertEquals(2, delta.addedOrUpdatedServers().size());
        assertTrue(delta.removedServers().isEmpty());
    }

    @Test
    void delta_nullPrev_emptySnapshot_noChanges() {
        var delta = snapshot("v1").serverSnapshotDelta(null);
        assertTrue(delta.addedOrUpdatedServers().isEmpty());
        assertTrue(delta.removedServers().isEmpty());
    }

    // --- serverSnapshotDelta(prev) ---

    @Test
    void delta_identicalSnapshots_noChanges() {
        var svc = service("ns/svc-a", server("s1", "server-1"));
        var prev = snapshot("v1", svc);
        var next = snapshot("v2", svc);

        var delta = next.serverSnapshotDelta(prev);
        assertTrue(delta.addedOrUpdatedServers().isEmpty());
        assertTrue(delta.removedServers().isEmpty());
    }

    @Test
    void delta_newServer_appearsInAdded() {
        var prev = snapshot("v1", service("ns/svc-a", server("s1", "server-1")));
        var next = snapshot("v2", service("ns/svc-a", server("s1", "server-1"), server("s2", "server-2")));

        var delta = next.serverSnapshotDelta(prev);
        assertEquals(1, delta.addedOrUpdatedServers().size());
        assertEquals("s2", delta.addedOrUpdatedServers().getFirst().uniqueId());
        assertTrue(delta.removedServers().isEmpty());
    }

    @Test
    void delta_removedServer_appearsInRemoved() {
        var prev = snapshot("v1", service("ns/svc-a", server("s1", "server-1"), server("s2", "server-2")));
        var next = snapshot("v2", service("ns/svc-a", server("s1", "server-1")));

        var delta = next.serverSnapshotDelta(prev);
        assertTrue(delta.addedOrUpdatedServers().isEmpty());
        assertEquals(1, delta.removedServers().size());
        assertEquals("s2", delta.removedServers().getFirst().uniqueId());
    }

    @Test
    void delta_updatedServer_appearsInAdded() {
        var prev = snapshot("v1", service("ns/svc-a", server("s1", "server-1")));
        // same uniqueId, different name → proto differs
        var updated = Types.ManagedServer.newBuilder().setUniqueId("s1").setName("server-1-renamed").build();
        var next = snapshot("v2", service("ns/svc-a", updated));

        var delta = next.serverSnapshotDelta(prev);
        assertEquals(1, delta.addedOrUpdatedServers().size());
        assertEquals("s1", delta.addedOrUpdatedServers().getFirst().uniqueId());
        assertTrue(delta.removedServers().isEmpty());
    }

    @Test
    void delta_mixedChanges_correctBuckets() {
        var prev = snapshot("v1",
                service("ns/svc-a",
                        server("s1", "server-1"),  // unchanged
                        server("s2", "server-2")   // will be removed
                ));
        var next = snapshot("v2",
                service("ns/svc-a",
                        server("s1", "server-1"),  // unchanged
                        server("s3", "server-3")   // new
                ));

        var delta = next.serverSnapshotDelta(prev);
        assertEquals(1, delta.addedOrUpdatedServers().size());
        assertEquals("s3", delta.addedOrUpdatedServers().getFirst().uniqueId());
        assertEquals(1, delta.removedServers().size());
        assertEquals("s2", delta.removedServers().getFirst().uniqueId());
    }

    @Test
    void delta_serverMovedBetweenServices_notRemovedOrAdded() {
        var prev = snapshot("v1", service("ns/svc-a", server("s1", "server-1")));
        var next = snapshot("v2", service("ns/svc-b", server("s1", "server-1")));

        var delta = next.serverSnapshotDelta(prev);
        // same uniqueId and proto — no change expected
        assertTrue(delta.addedOrUpdatedServers().isEmpty());
        assertTrue(delta.removedServers().isEmpty());
    }
}