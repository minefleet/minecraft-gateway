package dev.minefleet.api.gateway.networking.rules;

import dev.minefleet.api.gateway.networking.ManagedService;
import dev.minefleet.api.gateway.networking.player.NetworkPlayer;

public record RuleContext(NetworkPlayer<?> player, ManagedService service){

}
