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

import java.util.Optional;

import static org.junit.jupiter.api.Assertions.assertFalse;
import static org.junit.jupiter.api.Assertions.assertTrue;
import static org.mockito.Mockito.when;

@ExtendWith(MockitoExtension.class)
class FallbackForRuleTest {

    @Mock NetworkPlayer player;
    @Mock ManagedService service;

    private static final String REF = "default/lobby";
    private final FallbackForRule rule = new FallbackForRule(REF);

    private ManagedServer serverWithParent(String parentRef) {
        return new ManagedServer(parentRef,
                Types.ManagedServer.newBuilder().setUniqueId("s1").build());
    }

    @Test
    void trueWhenConnectedServerMatchesRef() {
        when(player.getConnectedServer()).thenReturn(Optional.of(serverWithParent(REF)));
        assertTrue(rule.evaluate(new RuleContext(player, service)));
    }

    @Test
    void falseWhenConnectedServerHasDifferentRef() {
        when(player.getConnectedServer()).thenReturn(Optional.of(serverWithParent("default/survival")));
        assertFalse(rule.evaluate(new RuleContext(player, service)));
    }

    @Test
    void falseWhenNoConnectedServer() {
        when(player.getConnectedServer()).thenReturn(Optional.empty());
        assertFalse(rule.evaluate(new RuleContext(player, service)));
    }
}
