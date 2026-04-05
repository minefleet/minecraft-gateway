package dev.minefleet.api.gateway.networking.rules.impl;

import dev.minefleet.api.gateway.networking.ManagedService;
import dev.minefleet.api.gateway.networking.rules.RuleContext;
import dev.minefleet.api.gateway.networking.player.NetworkPlayer;
import org.junit.jupiter.api.Test;
import org.junit.jupiter.api.extension.ExtendWith;
import org.mockito.Mock;
import org.mockito.junit.jupiter.MockitoExtension;

import static org.junit.jupiter.api.Assertions.assertFalse;
import static org.junit.jupiter.api.Assertions.assertTrue;
import static org.mockito.Mockito.when;

@ExtendWith(MockitoExtension.class)
class PermissionRuleTest {

    @Mock NetworkPlayer player;
    @Mock ManagedService service;

    private static final String PERM = "minefleet.vip";
    private final PermissionRule rule = new PermissionRule(PERM);

    @Test
    void trueWhenPlayerHasPermission() {
        when(player.hasPermission(PERM)).thenReturn(true);
        assertTrue(rule.evaluate(new RuleContext(player, service)));
    }

    @Test
    void falseWhenPlayerLacksPermission() {
        when(player.hasPermission(PERM)).thenReturn(false);
        assertFalse(rule.evaluate(new RuleContext(player, service)));
    }
}
