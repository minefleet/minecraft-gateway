package network

import (
	"math/rand"
	"regexp"
	"strings"
	"sync"
	"time"
)

// reservationTTL bounds how long a routing decision counts against a server's
// capacity before the player is expected to have shown up in the presence map.
// Too short lets simultaneous decisions converge on the same server again; too
// long strands capacity after a failed connect.
const reservationTTL = 30 * time.Second

// RouteKind selects which set of routes is evaluated.
type RouteKind int

const (
	RouteKindJoin RouteKind = iota
	RouteKindFallback
)

// RouteQuery is a routing decision request for one player.
type RouteQuery struct {
	PlayerUUID string
	Kind       RouteKind
	Context    PlayerContext
	// CurrentServerName is the server the player is being kicked from, for
	// fallback routing. Empty on join.
	CurrentServerName string
}

// counter reports how many players currently occupy a server.
type counter interface {
	CountForServer(serverName string) int
}

// Router turns a routing request into a target server. Unlike the per-proxy
// routing it replaces, it decides against the controller's global view of
// player counts, so simultaneous decisions across proxies cannot all converge
// on the same "least loaded" server.
type Router struct {
	presence counter

	mu           sync.Mutex
	reservations map[string][]time.Time
	// now and intn are injectable so tests can control expiry and selection.
	now  func() time.Time
	intn func(n int) int
}

func NewRouter(presence counter) *Router {
	return &Router{
		presence:     presence,
		reservations: make(map[string][]time.Time),
		now:          time.Now,
		intn:         rand.Intn,
	}
}

// Route picks a server for the player, or reports that no route matched. A
// successful decision reserves capacity on the chosen server until the player
// is observed there.
func (r *Router) Route(cfg *ListenerSnapshot, q RouteQuery) (*Server, bool) {
	if cfg == nil {
		return nil, false
	}

	var best *Service
	var bestPriority uint32
	for _, svc := range cfg.Services {
		routes := svc.JoinRoutes
		if q.Kind == RouteKindFallback {
			routes = svc.FallbackRoutes
		}
		if len(routes) == 0 {
			continue
		}
		priority, matched := r.matchedPriority(cfg, svc, routes, q)
		if !matched {
			continue
		}
		if best == nil || priority < bestPriority {
			best, bestPriority = svc, priority
		}
	}
	if best == nil {
		return nil, false
	}

	server := r.pick(best)
	if server == nil {
		return nil, false
	}
	r.Reserve(server.Name)
	return server, true
}

// matchedPriority returns the lowest priority among the service's routes whose
// rules all hold for this player.
func (r *Router) matchedPriority(cfg *ListenerSnapshot, svc *Service, routes []Route, q RouteQuery) (uint32, bool) {
	var best uint32
	found := false
	for _, route := range routes {
		if !r.evaluateRoute(cfg, svc, route, q) {
			continue
		}
		if !found || route.Priority < best {
			best, found = route.Priority, true
		}
	}
	return best, found
}

// evaluateRoute requires every rule set of the route to hold and the service to
// have at least one server with room.
func (r *Router) evaluateRoute(cfg *ListenerSnapshot, svc *Service, route Route, q RouteQuery) bool {
	for _, rs := range route.RuleSets {
		if !r.evaluateRuleSet(cfg, rs, q) {
			return false
		}
	}
	return len(r.availableServers(svc)) > 0
}

func (r *Router) evaluateRuleSet(cfg *ListenerSnapshot, rs RuleSet, q RouteQuery) bool {
	switch rs.Type {
	case RuleTypeAny:
		for _, rule := range rs.Rules {
			if r.evaluateRule(cfg, rule, q) {
				return true
			}
		}
		return false
	case RuleTypeNone:
		for _, rule := range rs.Rules {
			if r.evaluateRule(cfg, rule, q) {
				return false
			}
		}
		return true
	default: // RuleTypeAll
		for _, rule := range rs.Rules {
			if !r.evaluateRule(cfg, rule, q) {
				return false
			}
		}
		return true
	}
}

// evaluateRule requires every predicate set on the rule to hold. A rule with no
// predicate set contributes nothing and holds trivially.
func (r *Router) evaluateRule(cfg *ListenerSnapshot, rule Rule, q RouteQuery) bool {
	if rule.FallbackFor != "" {
		if q.CurrentServerName == "" {
			return false
		}
		owner, ok := cfg.ServiceOfServer(q.CurrentServerName)
		if !ok || owner != rule.FallbackFor {
			return false
		}
	}
	if rule.Domain != "" && !matchDomain(rule.Domain, q.Context.ConnectedDomain) {
		return false
	}
	if rule.Permission != "" && !q.Context.HasPermission(rule.Permission) {
		return false
	}
	return true
}

// availableServers returns the servers of a service that still have room.
func (r *Router) availableServers(svc *Service) []*Server {
	available := make([]*Server, 0, len(svc.Servers))
	for _, s := range svc.Servers {
		if !r.isFull(s) {
			available = append(available, s)
		}
	}
	return available
}

func (r *Router) isFull(s *Server) bool {
	if s.MaxPlayers == nil {
		return false
	}
	return uint32(r.load(s)) >= *s.MaxPlayers
}

// load is the live occupancy of a server: players actually present plus
// decisions already handed out but not yet observed.
func (r *Router) load(s *Server) int {
	count := r.presence.CountForServer(s.Name)
	if count == 0 && s.CurrentPlayers != nil {
		// No proxy has reported presence for this server yet; fall back to the
		// pod-annotated count so the first decisions are not made blind.
		count = int(*s.CurrentPlayers)
	}
	return count + r.reservationCount(s.Name)
}

func (r *Router) pick(svc *Service) *Server {
	available := r.availableServers(svc)
	if len(available) == 0 {
		return nil
	}
	if svc.DistributionStrategy == DistributionLeastPlayers {
		best := available[0]
		bestLoad := r.load(best)
		for _, s := range available[1:] {
			if l := r.load(s); l < bestLoad {
				best, bestLoad = s, l
			}
		}
		return best
	}
	return available[r.intn(len(available))]
}

// Reserve counts one pending arrival against a server.
func (r *Router) Reserve(serverName string) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.reservations[serverName] = append(r.reservations[serverName], r.now().Add(reservationTTL))
}

// Release drops one reservation for a server, once the player is observed there.
func (r *Router) Release(serverName string) {
	r.mu.Lock()
	defer r.mu.Unlock()
	pending := r.reservations[serverName]
	if len(pending) == 0 {
		return
	}
	r.reservations[serverName] = pending[1:]
	if len(r.reservations[serverName]) == 0 {
		delete(r.reservations, serverName)
	}
}

func (r *Router) reservationCount(serverName string) int {
	r.mu.Lock()
	defer r.mu.Unlock()
	pending := r.reservations[serverName]
	if len(pending) == 0 {
		return 0
	}
	now := r.now()
	live := pending[:0]
	for _, expiry := range pending {
		if expiry.After(now) {
			live = append(live, expiry)
		}
	}
	if len(live) == 0 {
		delete(r.reservations, serverName)
		return 0
	}
	r.reservations[serverName] = live
	return len(live)
}

var domainPatterns sync.Map // string → *regexp.Regexp

// matchDomain matches a domain pattern where "*" stands for exactly one label.
func matchDomain(pattern, domain string) bool {
	re, ok := domainPatterns.Load(pattern)
	if !ok {
		parts := strings.Split(pattern, "*")
		quoted := make([]string, 0, len(parts))
		for _, part := range parts {
			quoted = append(quoted, regexp.QuoteMeta(part))
		}
		compiled, err := regexp.Compile("^" + strings.Join(quoted, "[^.]+") + "$")
		if err != nil {
			return false
		}
		re = compiled
		domainPatterns.Store(pattern, compiled)
	}
	return re.(*regexp.Regexp).MatchString(domain)
}
