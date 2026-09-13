package dev.minefleet.api.gateway.networking;

/**
 * A player count and the capacity it is measured against.
 *
 * <p>It is used in both directions. A proxy reports its own count and the
 * capacity its configuration admits; the controller sums those across the
 * proxies of a gateway or listener and pushes the total back, so that a ping
 * answered by any one proxy shows the whole network rather than the replica
 * that happened to take the connection.
 *
 * @param onlinePlayers players currently connected
 * @param maxPlayers    how many players are allowed to connect
 */
public record PlayerCount(int onlinePlayers, int maxPlayers) {
}