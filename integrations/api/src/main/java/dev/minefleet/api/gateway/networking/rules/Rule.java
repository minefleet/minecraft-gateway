package dev.minefleet.api.gateway.networking.rules;

public interface Rule {
    boolean evaluate(RuleContext context);
}
