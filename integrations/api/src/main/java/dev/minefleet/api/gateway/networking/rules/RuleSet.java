package dev.minefleet.api.gateway.networking.rules;

import dev.minefleet.api.gateway.networking.rules.impl.AvailabilityRule;
import dev.minefleet.api.gateway.networking.rules.impl.DomainRule;
import dev.minefleet.api.gateway.networking.rules.impl.FallbackForRule;
import dev.minefleet.api.gateway.networking.rules.impl.PermissionRule;
import dev.minefleet.api.gateway.networking.v1alpha1.Types;

import java.util.ArrayList;
import java.util.List;

public class RuleSet implements Rule {

    private final List<Rule> rules;
    private final RuleType type;

    public RuleSet(List<Types.OptionRuleSet> proto) {
        this.rules = new ArrayList<>();
        this.type = RuleType.ALL;
        this.rules.addAll(proto.stream().map(RuleSet::new).toList());
        this.rules.add(new AvailabilityRule());
    }

    private RuleSet(Types.OptionRuleSet proto) {
        this.rules = new ArrayList<>();
        this.type = RuleType.of(proto.getType());
        proto.getRulesList().forEach(rule -> {
            var setRules = new ArrayList<Rule>(3);
            if(rule.hasFallbackFor()) {
                setRules.add(new FallbackForRule(rule.getFallbackFor()));
            }
            if(rule.hasDomain()) {
                setRules.add(new DomainRule(rule.getDomain()));
            }
            if(rule.hasPermission()) {
                setRules.add(new PermissionRule(rule.getPermission()));
            }
            if(setRules.size() > 1) {
                rules.add(new RuleSet(setRules, RuleType.ALL));
                return;
            }
            rules.addAll(setRules);
        });
    }

    private RuleSet(List<Rule> rules, RuleType type) {
        this.rules = rules;
        this.type = type;
    }

    @Override
    public boolean evaluate(RuleContext context) {
        return type.evaluateAll(rules, context);
    }
}
