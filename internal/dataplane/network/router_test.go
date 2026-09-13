package network

import (
	"testing"
	"time"

	"k8s.io/utils/ptr"
)

// fakeCounter reports fixed player counts per server.
type fakeCounter map[string]int

func (f fakeCounter) CountForServer(server string) int { return f[server] }

func serverWithCap(name string, max uint32) *Server {
	return &Server{UniqueID: name, Name: name, MaxPlayers: ptr.To(max)}
}

func TestMatchDomain(t *testing.T) {
	tests := []struct {
		pattern string
		domain  string
		want    bool
	}{
		{"play.example.com", "play.example.com", true},
		{"play.example.com", "other.example.com", false},
		{"*.example.com", "play.example.com", true},
		{"*.example.com", "deep.play.example.com", false}, // "*" is exactly one label
		{"*.example.com", "example.com", false},
		{"play.*.com", "play.example.com", true},
		{"play.*.com", "play.a.b.com", false},
		{"a*b", "a-mid-b", true},
		{"a*b", "ab", false},
	}
	for _, tt := range tests {
		if got := matchDomain(tt.pattern, tt.domain); got != tt.want {
			t.Errorf("matchDomain(%q, %q) = %v, want %v", tt.pattern, tt.domain, got, tt.want)
		}
	}
}

func TestRuleEvaluation(t *testing.T) {
	cfg := &ListenerSnapshot{Services: []*Service{{
		NamespacedName: "default/lobby",
		Servers:        []*Server{{Name: "lobby-0"}},
	}}}
	r := NewRouter(fakeCounter{})

	query := RouteQuery{
		Context:           PlayerContext{ConnectedDomain: "play.example.com", Permissions: map[string]bool{"vip": true}},
		CurrentServerName: "lobby-0",
	}

	tests := []struct {
		name string
		set  RuleSet
		want bool
	}{
		{"all rules hold", RuleSet{Type: RuleTypeAll, Rules: []Rule{{Domain: "play.example.com"}, {Permission: "vip"}}}, true},
		{"all with one failing", RuleSet{Type: RuleTypeAll, Rules: []Rule{{Domain: "play.example.com"}, {Permission: "staff"}}}, false},
		{"any with one holding", RuleSet{Type: RuleTypeAny, Rules: []Rule{{Permission: "staff"}, {Permission: "vip"}}}, true},
		{"any with none holding", RuleSet{Type: RuleTypeAny, Rules: []Rule{{Permission: "staff"}}}, false},
		{"none with none holding", RuleSet{Type: RuleTypeNone, Rules: []Rule{{Permission: "staff"}}}, true},
		{"none with one holding", RuleSet{Type: RuleTypeNone, Rules: []Rule{{Permission: "vip"}}}, false},
		{"empty all holds", RuleSet{Type: RuleTypeAll}, true},
		{"empty any fails", RuleSet{Type: RuleTypeAny}, false},
		{"rule needs every predicate", RuleSet{Type: RuleTypeAll, Rules: []Rule{{Domain: "play.example.com", Permission: "staff"}}}, false},
		{"fallbackFor matches owning service", RuleSet{Type: RuleTypeAll, Rules: []Rule{{FallbackFor: "default/lobby"}}}, true},
		{"fallbackFor other service", RuleSet{Type: RuleTypeAll, Rules: []Rule{{FallbackFor: "default/games"}}}, false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := r.evaluateRuleSet(cfg, tt.set, query); got != tt.want {
				t.Errorf("evaluateRuleSet() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestRouteSelectsLowestPriorityService(t *testing.T) {
	cfg := &ListenerSnapshot{Services: []*Service{
		{
			NamespacedName: "default/high",
			Servers:        []*Server{{Name: "high-0"}},
			JoinRoutes:     []Route{{Priority: 10}},
		},
		{
			NamespacedName: "default/low",
			Servers:        []*Server{{Name: "low-0"}},
			JoinRoutes:     []Route{{Priority: 1}},
		},
	}}

	r := NewRouter(fakeCounter{})
	server, ok := r.Route(cfg, RouteQuery{Kind: RouteKindJoin})
	if !ok {
		t.Fatal("expected a route to match")
	}
	if server.Name != "low-0" {
		t.Errorf("got %q, want the lowest-priority service's server low-0", server.Name)
	}
}

func TestRouteSkipsFullServers(t *testing.T) {
	cfg := &ListenerSnapshot{Services: []*Service{{
		NamespacedName: "default/lobby",
		Servers:        []*Server{serverWithCap("full", 2), serverWithCap("open", 2)},
		JoinRoutes:     []Route{{Priority: 1}},
	}}}

	r := NewRouter(fakeCounter{"full": 2, "open": 0})
	for i := range 3 {
		server, ok := r.Route(cfg, RouteQuery{Kind: RouteKindJoin})
		if i < 2 {
			if !ok || server.Name != "open" {
				t.Fatalf("decision %d: got %v, want the server with room", i, server)
			}
			continue
		}
		// Two reservations have filled the only open server.
		if ok {
			t.Fatalf("decision %d: expected no route once every server is full, got %q", i, server.Name)
		}
	}
}

func TestRouteWithoutMaxPlayersIsNeverFull(t *testing.T) {
	cfg := &ListenerSnapshot{Services: []*Service{{
		NamespacedName: "default/lobby",
		Servers:        []*Server{{Name: "lobby-0"}},
		JoinRoutes:     []Route{{Priority: 1}},
	}}}

	r := NewRouter(fakeCounter{"lobby-0": 5000})
	if _, ok := r.Route(cfg, RouteQuery{Kind: RouteKindJoin}); !ok {
		t.Error("a server without a max-players annotation should always accept players")
	}
}

// A mass fallback must spread across servers rather than converging on whichever
// one looked least loaded when the burst began. This is the herding regression.
func TestLeastPlayersSpreadsSimultaneousDecisions(t *testing.T) {
	const servers, players = 4, 100

	svc := &Service{
		NamespacedName:       "default/lobby",
		DistributionStrategy: DistributionLeastPlayers,
		FallbackRoutes:       []Route{{Priority: 1}},
	}
	for i := range servers {
		svc.Servers = append(svc.Servers, serverWithCap(string(rune('a'+i)), 1000))
	}
	cfg := &ListenerSnapshot{Services: []*Service{svc}}

	// Presence never updates: every decision in the burst sees the same counts,
	// exactly as proxies did when they each polled a stale snapshot.
	r := NewRouter(fakeCounter{})
	assigned := map[string]int{}
	for range players {
		server, ok := r.Route(cfg, RouteQuery{Kind: RouteKindFallback})
		if !ok {
			t.Fatal("expected every player to be routed")
		}
		assigned[server.Name]++
	}

	if len(assigned) != servers {
		t.Fatalf("players landed on %d of %d servers: %v", len(assigned), servers, assigned)
	}
	for name, count := range assigned {
		if want := players / servers; count != want {
			t.Errorf("server %s took %d players, want an even %d", name, count, want)
		}
	}
}

func TestReservationsExpire(t *testing.T) {
	now := time.Now()
	r := NewRouter(fakeCounter{})
	r.now = func() time.Time { return now }

	r.Reserve("lobby-0")
	if got := r.reservationCount("lobby-0"); got != 1 {
		t.Fatalf("reservationCount = %d, want 1", got)
	}

	now = now.Add(reservationTTL + time.Second)
	if got := r.reservationCount("lobby-0"); got != 0 {
		t.Errorf("reservationCount after expiry = %d, want 0", got)
	}
}

func TestReleaseDropsReservation(t *testing.T) {
	r := NewRouter(fakeCounter{})
	r.Reserve("lobby-0")
	r.Reserve("lobby-0")
	r.Release("lobby-0")

	if got := r.reservationCount("lobby-0"); got != 1 {
		t.Errorf("reservationCount after release = %d, want 1", got)
	}
}

func TestRouteIgnoresOtherKind(t *testing.T) {
	cfg := &ListenerSnapshot{Services: []*Service{{
		NamespacedName: "default/lobby",
		Servers:        []*Server{{Name: "lobby-0"}},
		JoinRoutes:     []Route{{Priority: 1}},
	}}}

	r := NewRouter(fakeCounter{})
	if _, ok := r.Route(cfg, RouteQuery{Kind: RouteKindFallback}); ok {
		t.Error("a join-only service must not answer a fallback request")
	}
}
