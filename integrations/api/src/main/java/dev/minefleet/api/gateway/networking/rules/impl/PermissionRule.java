package dev.minefleet.api.gateway.networking.rules.impl;

import dev.minefleet.api.gateway.networking.rules.Rule;
import dev.minefleet.api.gateway.networking.rules.RuleContext;

public class PermissionRule implements Rule {

    private final String permission;

    public PermissionRule(String permission) {
        this.permission = permission;
    }

    @Override
    public boolean evaluate(RuleContext context) {
        return context.player().hasPermission(permission);
    }
}
