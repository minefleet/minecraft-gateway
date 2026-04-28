package dev.minefleet.api.gateway.networking;

import dev.minefleet.api.gateway.networking.v1alpha1.Types;
import org.junit.jupiter.api.Test;

import static org.junit.jupiter.api.Assertions.assertFalse;
import static org.junit.jupiter.api.Assertions.assertTrue;

class ManagedServerTest {

    private ManagedServer server(int current, int max) {
        return new ManagedServer("ns/svc", Types.ManagedServer.newBuilder()
                .setUniqueId("s1").setName("server-1")
                .setCurrentPlayers(current)
                .setMaxPlayers(max)
                .build());
    }

    private ManagedServer serverWithoutMax(int current) {
        return new ManagedServer("ns/svc", Types.ManagedServer.newBuilder()
                .setUniqueId("s1").setName("server-1")
                .setCurrentPlayers(current)
                .build());
    }

    private ManagedServer serverWithoutAny() {
        return new ManagedServer("ns/svc", Types.ManagedServer.newBuilder()
                .setUniqueId("s1").setName("server-1")
                .build());
    }

    @Test
    void isFull_noAnnotations_returnsFalse() {
        assertFalse(serverWithoutAny().isFull());
    }

    @Test
    void isFull_onlyCurrentPlayers_returnsFalse() {
        assertFalse(serverWithoutMax(10).isFull());
    }

    @Test
    void isFull_onlyMaxPlayers_returnsFalse() {
        var s = new ManagedServer("ns/svc", Types.ManagedServer.newBuilder()
                .setUniqueId("s1").setName("server-1").setMaxPlayers(10).build());
        assertFalse(s.isFull());
    }

    @Test
    void isFull_currentBelowMax_returnsFalse() {
        assertFalse(server(5, 10).isFull());
    }

    @Test
    void isFull_currentEqualsMax_returnsTrue() {
        assertTrue(server(10, 10).isFull());
    }

    @Test
    void isFull_currentAboveMax_returnsTrue() {
        assertTrue(server(11, 10).isFull());
    }
}