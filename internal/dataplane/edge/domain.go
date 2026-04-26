package edge

import (
	"minefleet.dev/minecraft-gateway/internal/topology"
	gatewayv1 "sigs.k8s.io/gateway-api/apis/v1"
)

func Domains(routes topology.RouteBag) []string {
	result := make([]string, 0)
	for _, joinRoute := range routes.Join {
		for _, h := range joinRoute.Hostnames() {
			result = append(result, string(h))
		}
	}
	return dedupeDomains(result)
}

// filterDomainsByListener narrows a set of route hostnames to those compatible
// with the listener hostname, following Gateway API intersection semantics.
// When the listener has no hostname all domains pass through unchanged.
// A route wildcard that covers an exact listener hostname is narrowed to that
// exact hostname; other combinations either match as-is or are excluded.
func filterDomainsByListener(domains []string, listenerHostname *gatewayv1.Hostname) []string {
	if listenerHostname == nil || *listenerHostname == "" {
		return domains
	}
	lh := string(*listenerHostname)
	result := make([]string, 0, len(domains))
	for _, domain := range domains {
		if effective, ok := intersectHostname(lh, domain); ok {
			result = append(result, effective)
		}
	}
	return dedupeDomains(result)
}

// intersectHostname returns the effective hostname when a listener hostname and
// a route hostname are combined. Returns ("", false) if they are incompatible.
func intersectHostname(listener, routeHost string) (string, bool) {
	listenerWild := isWildcard(listener)
	routeWild := isWildcard(routeHost)
	switch {
	case !listenerWild && !routeWild:
		if listener == routeHost {
			return routeHost, true
		}
	case !listenerWild:
		// *.example.com covers foo.example.com → narrow to listener
		if matchesSuffix(listener, routeHost[2:]) {
			return listener, true
		}
	default: // both wildcard
		if listener == routeHost {
			return routeHost, true
		}
		// *.test.example.com is more specific than *.example.com, pass through
		if matchesSuffix(routeHost[2:], listener[2:]) {
			return routeHost, true
		}
	}
	return "", false
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
	if len(domain) <= len(suffix) {
		return false
	}
	return domain[len(domain)-len(suffix):] == suffix &&
		domain[len(domain)-len(suffix)-1] == '.'
}
