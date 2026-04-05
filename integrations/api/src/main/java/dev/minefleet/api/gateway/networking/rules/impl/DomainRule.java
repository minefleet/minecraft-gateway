package dev.minefleet.api.gateway.networking.rules.impl;

import dev.minefleet.api.gateway.networking.rules.Rule;
import dev.minefleet.api.gateway.networking.rules.RuleContext;

import java.util.regex.Pattern;

public class DomainRule implements Rule {

    private final Pattern pattern;

    public DomainRule(String domain) {
        // Convert glob-style domain pattern to regex.
        // '*' matches exactly one DNS label (no dots), e.g. staging.*.example.com
        StringBuilder regex = new StringBuilder("^");
        for (String part : domain.split("\\*", -1)) {
            regex.append(Pattern.quote(part));
            regex.append("[^.]+");
        }
        // Remove the trailing [^.]+ added after the last split segment
        regex.delete(regex.length() - "[^.]+".length(), regex.length());
        regex.append("$");
        this.pattern = Pattern.compile(regex.toString());
    }

    @Override
    public boolean evaluate(RuleContext context) {
        return pattern.matcher(context.player().connectedDomain()).matches();
    }
}
