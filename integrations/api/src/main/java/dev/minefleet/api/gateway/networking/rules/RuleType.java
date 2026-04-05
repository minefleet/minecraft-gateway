package dev.minefleet.api.gateway.networking.rules;

import dev.minefleet.api.gateway.networking.v1alpha1.Types;

import java.util.List;

public enum RuleType {
    ALL,
    ANY,
    NONE;

    public static RuleType of(Types.RuleType proto) {
        switch (proto.getNumber()) {
            case Types.RuleType.ALL_VALUE -> {
                return RuleType.ALL;
            }
            case Types.RuleType.ANY_VALUE -> {
                return RuleType.ANY;
            }
            case Types.RuleType.NONE_VALUE -> {
                return RuleType.NONE;
            }
            default -> throw new IllegalArgumentException("no rule type found for " + proto.getValueDescriptor());
        }
    }

    public boolean evaluateAll(List<Rule> rules, RuleContext context) {
        return switch (this) {
            case NONE -> rules.stream().noneMatch(rule -> rule.evaluate(context));
            case ANY -> rules.stream().anyMatch(rule -> rule.evaluate(context));
            case ALL -> rules.stream().allMatch(rule -> rule.evaluate(context));
            default -> false;
        };
    }
}
