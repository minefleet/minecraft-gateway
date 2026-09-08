package network

import (
	mcgatewayv1alpha1 "minefleet.dev/minecraft-gateway/api/controller/v1alpha1"
)

// This file defines the controller's internal routing model. Routes and rules
// are evaluated in the controller and never reach a proxy, so these types are
// deliberately independent of the wire format.

// RuleType determines how the rules of a RuleSet are combined.
type RuleType int

const (
	RuleTypeAll RuleType = iota
	RuleTypeAny
	RuleTypeNone
)

// Rule is a single routing predicate. An empty field means the corresponding
// predicate is not part of this rule; a rule with several fields set requires
// all of them to hold.
type Rule struct {
	Domain      string
	Permission  string
	FallbackFor string
}

// RuleSet combines rules according to its type.
type RuleSet struct {
	Type  RuleType
	Rules []Rule
}

// Route is one route attached to a service, at a given priority. Lower
// priorities win.
type Route struct {
	Priority uint32
	RuleSets []RuleSet
}

// Server is one backend pod behind a service.
type Server struct {
	UniqueID string
	Name     string
	IP       string
	Port     uint32
	// MaxPlayers is the capacity reported by the pod's annotations, if any.
	// Without it a server is never considered full.
	MaxPlayers *uint32
	// CurrentPlayers is the pod-annotated count. It seeds the count before any
	// proxy has reported presence; live routing prefers the presence map.
	CurrentPlayers *uint32
}

// DistributionStrategy selects how players are spread across a service's servers.
type DistributionStrategy int

const (
	DistributionRandom DistributionStrategy = iota
	DistributionLeastPlayers
)

// Service is a backend Service with the routes that direct players to it.
type Service struct {
	NamespacedName string
	Namespace      string
	Name           string
	// Labels are the EndpointSlice labels of the service's endpoints, which is
	// what fallbackFor and PlayerTransfer label selectors match against.
	Labels               map[string]string
	DistributionStrategy DistributionStrategy
	Servers              []*Server
	JoinRoutes           []Route
	FallbackRoutes       []Route
}

// ListenerSnapshot is the routing configuration for one gateway listener.
type ListenerSnapshot struct {
	GatewayNamespace string
	GatewayName      string
	ListenerName     string
	Services         []*Service
}

// RequiredPermissions returns the deduplicated set of permissions appearing in
// any rule of the listener, sorted for a stable wire representation. Proxies
// evaluate exactly these for every player and report the results back, so the
// controller can apply permission rules without asking mid-decision.
func (l ListenerSnapshot) RequiredPermissions() []string {
	seen := make(map[string]struct{})
	for _, svc := range l.Services {
		for _, route := range append(append([]Route{}, svc.JoinRoutes...), svc.FallbackRoutes...) {
			for _, rs := range route.RuleSets {
				for _, r := range rs.Rules {
					if r.Permission != "" {
						seen[r.Permission] = struct{}{}
					}
				}
			}
		}
	}
	return sortedKeys(seen)
}

// Servers returns every server of the listener, across all services.
func (l ListenerSnapshot) Servers() []*Server {
	var servers []*Server
	for _, svc := range l.Services {
		servers = append(servers, svc.Servers...)
	}
	return servers
}

// ServiceOfServer returns the namespaced name of the service owning the named
// server, which is what fallbackFor rules match against.
func (l ListenerSnapshot) ServiceOfServer(serverName string) (string, bool) {
	for _, svc := range l.Services {
		for _, s := range svc.Servers {
			if s.Name == serverName {
				return svc.NamespacedName, true
			}
		}
	}
	return "", false
}

func toRuleType(t mcgatewayv1alpha1.MinecraftFilterRuleType) RuleType {
	switch t {
	case mcgatewayv1alpha1.MinecraftFilterRuleAny:
		return RuleTypeAny
	case mcgatewayv1alpha1.MinecraftFilterRuleNone:
		return RuleTypeNone
	default:
		return RuleTypeAll
	}
}

func toDistributionStrategy(s mcgatewayv1alpha1.MinecraftDistributionStrategy) DistributionStrategy {
	if s.Type == mcgatewayv1alpha1.MinecraftDistributionStrategyLeastPlayers {
		return DistributionLeastPlayers
	}
	return DistributionRandom
}
