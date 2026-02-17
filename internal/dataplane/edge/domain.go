package edge

import (
	mcgatewayv1 "minefleet.dev/minecraft-gateway/api/v1"
	"minefleet.dev/minecraft-gateway/internal/route"
)

func Domains(routes route.Bag) []string {
	result := make([]string, 0)
	for _, joinRoute := range routes.Join {
		result = dedupeDomains(append(result, setListDomains(joinRoute.Spec.FilterRules)...))
	}
	return result
}

func setListDomains(rules []mcgatewayv1.MinecraftJoinFilterRuleSet) []string {
	result := make([]string, 0)
	for _, set := range rules {
		result = dedupeDomains(append(result, setDomains(set)...))
	}
	return result
}

func setDomains(set mcgatewayv1.MinecraftJoinFilterRuleSet) []string {
	switch set.Type {
	case mcgatewayv1.MinecraftFilterRuleNone:
		return nil
	case mcgatewayv1.MinecraftFilterRuleAny:
		result := make([]string, 0)
		for _, rule := range set.Rules {
			if rule.Domain == "" {
				continue
			}
			result = append(result, rule.Domain)
		}
		return dedupeDomains(result)
	case mcgatewayv1.MinecraftFilterRuleAll:
		for _, rule := range set.Rules {
			if rule.Domain == "" {
				continue
			}
			return []string{rule.Domain}
		}
		return nil
	default:
		return nil
	}
}

func dedupeDomains(domains []string) []string {
	unique := uniqueDomains(domains)
	wildcards := wildcardSuffixes(unique)

	result := make([]string, 0, len(unique))
	for domain := range unique {
		if isWildcard(domain) || !coveredByWildcard(domain, wildcards) {
			result = append(result, domain)
		}
	}

	return result
}

func uniqueDomains(domains []string) map[string]struct{} {
	m := make(map[string]struct{}, len(domains))
	for _, d := range domains {
		if d != "" {
			m[d] = struct{}{}
		}
	}
	return m
}

func wildcardSuffixes(domains map[string]struct{}) map[string]struct{} {
	m := make(map[string]struct{})
	for d := range domains {
		if isWildcard(d) {
			m[d[2:]] = struct{}{}
		}
	}
	return m
}

func isWildcard(domain string) bool {
	return len(domain) > 2 && domain[:2] == "*."
}

func coveredByWildcard(domain string, wildcards map[string]struct{}) bool {
	for suffix := range wildcards {
		if matchesSuffix(domain, suffix) {
			return true
		}
	}
	return false
}

func matchesSuffix(domain, suffix string) bool {
	if domain == suffix {
		return true
	}
	if len(domain) <= len(suffix) {
		return false
	}
	return domain[len(domain)-len(suffix):] == suffix &&
		domain[len(domain)-len(suffix)-1] == '.'
}
