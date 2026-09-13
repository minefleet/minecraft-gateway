package dev.minefleet.api.gateway.networking;

import java.util.List;
import java.util.Optional;
import java.util.UUID;

/**
 * The outcome of one {@link NetworkGatewayClient#movePlayers} call, one entry
 * per player in the order they were requested.
 */
public record MoveResult(List<PlayerMove> players) {

    public MoveResult {
        players = List.copyOf(players);
    }

    /** Whether every player reached their destination. */
    public boolean allSucceeded() {
        return players.stream().allMatch(PlayerMove::success);
    }

    /** The players that did not move, with the reason each one failed. */
    public List<PlayerMove> failures() {
        return players.stream().filter(move -> !move.success()).toList();
    }

    /** Looks up one player's outcome. */
    public Optional<PlayerMove> forPlayer(UUID playerUuid) {
        return players.stream().filter(move -> move.playerUuid().equals(playerUuid)).findFirst();
    }

    /**
     * One player's terminal outcome.
     *
     * @param playerUuid the player
     * @param serverName the server they were sent to, empty if none was resolved
     * @param success    whether the proxy reported the move as done
     * @param reason     why the move failed: {@code PlayerNotConnected},
     *                   {@code NoMatchingRoute}, {@code NoMatchingServer},
     *                   {@code ProxyUnavailable}, {@code MoveFailed}, or a reason
     *                   the proxy supplied itself. Empty on success.
     */
    public record PlayerMove(UUID playerUuid, Optional<String> serverName, boolean success, Optional<String> reason) {
    }
}
