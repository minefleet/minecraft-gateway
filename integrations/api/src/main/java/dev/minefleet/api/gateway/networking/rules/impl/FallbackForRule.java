package dev.minefleet.api.gateway.networking.rules.impl;

import dev.minefleet.api.gateway.networking.rules.Rule;
import dev.minefleet.api.gateway.networking.rules.RuleContext;

public class FallbackForRule implements Rule {

    private final String ref;

    public FallbackForRule(String ref) {
        this.ref = ref;
    }

    @Override
    public boolean evaluate(RuleContext context) {
        var server = context.player().connectedServer().orElse(null);
        if (server == null) {
            return false;
        }
        return server.parentNamespacedName().equals(ref);
    }
}
