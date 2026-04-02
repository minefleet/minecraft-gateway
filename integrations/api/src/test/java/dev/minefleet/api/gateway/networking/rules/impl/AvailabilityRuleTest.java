package dev.minefleet.api.gateway.networking.rules.impl;

import dev.minefleet.api.gateway.networking.ManagedServer;
import dev.minefleet.api.gateway.networking.ManagedService;
import dev.minefleet.api.gateway.networking.rules.RuleContext;
import dev.minefleet.api.gateway.networking.player.NetworkPlayer;
import dev.minefleet.api.gateway.networking.v1alpha1.Types;
import org.junit.jupiter.api.Test;
import org.junit.jupiter.api.extension.ExtendWith;
import org.mockito.Mock;
import org.mockito.junit.jupiter.MockitoExtension;

import java.util.List;

import static org.junit.jupiter.api.Assertions.assertFalse;
import static org.junit.jupiter.api.Assertions.assertTrue;
import static org.mockito.Mockito.when;

@ExtendWith(MockitoExtension.class)
class AvailabilityRuleTest {

    @Mock NetworkPlayer<?> player;
    @Mock ManagedService service;

    private final AvailabilityRule rule = new AvailabilityRule();

    @Test
    void trueWhenServersPresent() {
        when(service.availableServersAmount()).thenReturn(1);
        assertTrue(rule.evaluate(new RuleContext(player, service)));
    }

    @Test
    void falseWhenNoServers() {
        when(service.availableServersAmount()).thenReturn(0);
        assertFalse(rule.evaluate(new RuleContext(player, service)));
    }
}
