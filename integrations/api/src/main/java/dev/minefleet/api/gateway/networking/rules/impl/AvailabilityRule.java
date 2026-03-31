package dev.minefleet.api.gateway.networking.rules.impl;

import dev.minefleet.api.gateway.networking.rules.Rule;
import dev.minefleet.api.gateway.networking.rules.RuleContext;

public class AvailabilityRule implements Rule {
    @Override
    public boolean evaluate(RuleContext context) {
        return !context.service().servers().isEmpty();
    }
}
