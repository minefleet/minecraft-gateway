package dev.minefleet.api.gateway.networking.rules.impl;

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
class DomainRuleTest {

    @Mock NetworkPlayer player;
    @Mock dev.minefleet.api.gateway.networking.ManagedService service;

    private RuleContext ctx(String domain) {
        when(player.getConnectedDomain()).thenReturn(domain);
        return new RuleContext(player, service);
    }

    @Test
    void exactMatch() {
        assertTrue(new DomainRule("play.example.com").evaluate(ctx("play.example.com")));
    }

    @Test
    void exactMismatch() {
        assertFalse(new DomainRule("play.example.com").evaluate(ctx("other.example.com")));
    }

    @Test
    void prefixWildcardMatches() {
        assertTrue(new DomainRule("*.example.com").evaluate(ctx("foo.example.com")));
    }

    @Test
    void prefixWildcardDoesNotMatchMultipleLabels() {
        assertFalse(new DomainRule("*.example.com").evaluate(ctx("foo.bar.example.com")));
    }

    @Test
    void prefixWildcardDoesNotMatchBareApex() {
        assertFalse(new DomainRule("*.example.com").evaluate(ctx("example.com")));
    }

    @Test
    void midWildcardMatches() {
        assertTrue(new DomainRule("staging.*.example.com").evaluate(ctx("staging.foo.example.com")));
    }

    @Test
    void midWildcardDoesNotMatchMultipleLabels() {
        assertFalse(new DomainRule("staging.*.example.com").evaluate(ctx("staging.foo.bar.example.com")));
    }

    @Test
    void midWildcardDoesNotMatchWrongPrefix() {
        assertFalse(new DomainRule("staging.*.example.com").evaluate(ctx("prod.foo.example.com")));
    }
}
