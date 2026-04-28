package dev.minefleet.api.gateway.networking.player;

@FunctionalInterface
public interface PlayerProvider<P> {
    NetworkPlayer getPlayer(P player);
}