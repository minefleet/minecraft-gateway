package dev.minefleet.api.gateway.networking.stream;

import dev.minefleet.api.gateway.networking.ManagedServer;
import dev.minefleet.api.gateway.networking.v1alpha1.Types;
import org.junit.jupiter.api.Test;

import java.util.List;
import java.util.Map;
import java.util.stream.Collectors;

import static org.junit.jupiter.api.Assertions.assertEquals;
import static org.junit.jupiter.api.Assertions.assertTrue;

class ServerSetDeltaTest {

    private static ManagedServer server(String id, int port) {
        return new ManagedServer(Types.ManagedServer.newBuilder()
                .setUniqueId(id)
                .setName(id)
                .setIp("10.0.0.1")
                .setPort(port)
                .build());
    }

    private static Map<String, ManagedServer> asMap(ManagedServer... servers) {
        return List.of(servers).stream().collect(Collectors.toMap(ManagedServer::uniqueId, s -> s));
    }

    @Test
    void everythingIsAddedWhenNothingIsRegisteredYet() {
        var delta = ServerSetDelta.between(Map.of(), List.of(server("a", 25565), server("b", 25565)));

        assertEquals(2, delta.addedOrUpdated().size());
        assertTrue(delta.removed().isEmpty());
    }

    @Test
    void unchangedServersProduceNoWork() {
        var existing = asMap(server("a", 25565));
        var delta = ServerSetDelta.between(existing, List.of(server("a", 25565)));

        assertTrue(delta.isEmpty());
    }

    @Test
    void changedServerIsReRegistered() {
        var existing = asMap(server("a", 25565));
        var delta = ServerSetDelta.between(existing, List.of(server("a", 25566)));

        assertEquals(1, delta.addedOrUpdated().size());
        assertEquals(25566, delta.addedOrUpdated().getFirst().port());
        assertTrue(delta.removed().isEmpty());
    }

    @Test
    void missingServerIsRemoved() {
        var existing = asMap(server("a", 25565), server("b", 25565));
        var delta = ServerSetDelta.between(existing, List.of(server("a", 25565)));

        assertTrue(delta.addedOrUpdated().isEmpty());
        assertEquals(1, delta.removed().size());
        assertEquals("b", delta.removed().getFirst().uniqueId());
    }

    @Test
    void emptyPushRemovesEverything() {
        var existing = asMap(server("a", 25565), server("b", 25565));
        var delta = ServerSetDelta.between(existing, List.of());

        assertEquals(2, delta.removed().size());
    }
}
