package dev.minefleet.api.gateway.networking.player;

public enum KickReason {
    NO_FALLBACK,
    NO_JOIN,
    /**
     * The controller is unreachable. Joins are rejected rather than routed
     * against stale state while the stream is down.
     */
    ROUTING_UNAVAILABLE,
}
