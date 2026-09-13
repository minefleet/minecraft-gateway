package network

import (
	"testing"

	"k8s.io/apimachinery/pkg/types"
	apiv1alpha1 "minefleet.dev/minecraft-gateway/api/network/v1alpha1"
)

// helloOn is helloFor with an explicit listener, for gateways serving more than
// one.
func helloOn(proxyID, listener string) *apiv1alpha1.ProxyHello {
	hello := helloFor(proxyID)
	hello.ListenerName = listener
	return hello
}

// managerWith builds a StreamManager whose listeners carry the given player
// count configuration, keyed by listener name.
func managerWith(t *testing.T, configs map[string]*PlayerCountConfig) *StreamManager {
	t.Helper()
	gw := types.NamespacedName{Namespace: "default", Name: "gw"}
	cache := GatewaySnapshotCache{gw: make(map[string]ListenerSnapshot)}
	for listener, cfg := range configs {
		cache[gw][listener] = ListenerSnapshot{
			GatewayNamespace: gw.Namespace,
			GatewayName:      gw.Name,
			ListenerName:     listener,
			PlayerCount:      cfg,
		}
	}
	mgr := NewStreamManager()
	mgr.UpdateSnapshot(BuildSnapshot(cache))
	return mgr
}

// nextPlayerCount drains a session's outgoing queue and returns the last
// aggregated count it was sent, if any. A message telling the proxy to stop
// aggregating counts as no aggregate.
func nextPlayerCount(s *ProxySession) (PlayerCounts, bool) {
	var last PlayerCounts
	found := false
	for {
		select {
		case msg := <-s.Outgoing():
			if sync := msg.GetPlayerCount(); sync != nil {
				found = sync.GetAggregated()
				last = PlayerCounts{
					OnlinePlayers: sync.GetOnlinePlayers(),
					MaxPlayers:    sync.GetMaxPlayers(),
				}
			}
		default:
			return last, found
		}
	}
}

// reconfigure replaces a manager's listener configuration, as a change to the
// gateway's infrastructure would.
func reconfigure(t *testing.T, mgr *StreamManager, configs map[string]*PlayerCountConfig) {
	t.Helper()
	gw := types.NamespacedName{Namespace: "default", Name: "gw"}
	cache := GatewaySnapshotCache{gw: make(map[string]ListenerSnapshot)}
	for listener, cfg := range configs {
		cache[gw][listener] = ListenerSnapshot{
			GatewayNamespace: gw.Namespace,
			GatewayName:      gw.Name,
			ListenerName:     listener,
			PlayerCount:      cfg,
		}
	}
	mgr.UpdateSnapshot(BuildSnapshot(cache))
}

func TestAggregatorSumsReportingProxies(t *testing.T) {
	a := NewPlayerCountAggregator()
	a.Set("proxy-a", ProxyStatus{OnlinePlayers: 12, MaxPlayers: 500})
	a.Set("proxy-b", ProxyStatus{OnlinePlayers: 30, MaxPlayers: 500})

	total, known := a.Total([]string{"proxy-a", "proxy-b"})
	if !known {
		t.Fatal("a total over reporting proxies should be known")
	}
	if total.OnlinePlayers != 42 || total.MaxPlayers != 1000 {
		t.Errorf("Total = %+v, want {42 1000}", total)
	}
}

func TestAggregatorIgnoresProxiesThatNeverReported(t *testing.T) {
	a := NewPlayerCountAggregator()
	a.Set("proxy-a", ProxyStatus{OnlinePlayers: 5, MaxPlayers: 100})

	total, known := a.Total([]string{"proxy-a", "proxy-silent"})
	if !known || total.OnlinePlayers != 5 || total.MaxPlayers != 100 {
		t.Errorf("Total = %+v (known %v), want {5 100} known", total, known)
	}

	if _, known := a.Total([]string{"proxy-silent"}); known {
		t.Error("a total over proxies that never reported should not be known")
	}
}

func TestAggregatorReplacesReportsRatherThanAccumulating(t *testing.T) {
	a := NewPlayerCountAggregator()
	a.Set("proxy-a", ProxyStatus{OnlinePlayers: 5, MaxPlayers: 100})
	a.Set("proxy-a", ProxyStatus{OnlinePlayers: 7, MaxPlayers: 100})

	if total, _ := a.Total([]string{"proxy-a"}); total.OnlinePlayers != 7 {
		t.Errorf("OnlinePlayers = %d, want 7", total.OnlinePlayers)
	}
}

func TestAggregatorDropsDisconnectedProxyCapacity(t *testing.T) {
	a := NewPlayerCountAggregator()
	a.Set("proxy-a", ProxyStatus{OnlinePlayers: 5, MaxPlayers: 100})
	a.Set("proxy-b", ProxyStatus{OnlinePlayers: 5, MaxPlayers: 100})
	a.Drop("proxy-b")

	total, _ := a.Total([]string{"proxy-a", "proxy-b"})
	if total.MaxPlayers != 100 {
		t.Errorf("MaxPlayers = %d, want 100: a disconnected proxy advertises no capacity", total.MaxPlayers)
	}
}

func TestGatewayScopeSumsEveryListener(t *testing.T) {
	mgr := managerWith(t, map[string]*PlayerCountConfig{
		"normal": {Scope: PlayerCountScopeGateway},
		"vip":    {Scope: PlayerCountScopeGateway},
	})

	normal := newProxySession(helloOn("proxy-normal", "normal"))
	vip := newProxySession(helloOn("proxy-vip", "vip"))
	mgr.sessions.Add(normal)
	mgr.sessions.Add(vip)
	mgr.playerCounts.Set("proxy-normal", ProxyStatus{OnlinePlayers: 340, MaxPlayers: 1500})
	mgr.playerCounts.Set("proxy-vip", ProxyStatus{OnlinePlayers: 12, MaxPlayers: 50})

	mgr.flushPlayerCounts()

	for _, session := range []*ProxySession{normal, vip} {
		counts, ok := nextPlayerCount(session)
		if !ok {
			t.Fatalf("%s was sent no aggregated count", session.ProxyID)
		}
		if counts.OnlinePlayers != 352 || counts.MaxPlayers != 1550 {
			t.Errorf("%s got %+v, want {352 1550}", session.ProxyID, counts)
		}
	}
}

func TestListenerScopeSumsOnlyItsOwnProxies(t *testing.T) {
	mgr := managerWith(t, map[string]*PlayerCountConfig{
		"normal": {Scope: PlayerCountScopeListener},
		"vip":    {Scope: PlayerCountScopeListener},
	})

	normalA := newProxySession(helloOn("proxy-normal-a", "normal"))
	normalB := newProxySession(helloOn("proxy-normal-b", "normal"))
	vip := newProxySession(helloOn("proxy-vip", "vip"))
	mgr.sessions.Add(normalA)
	mgr.sessions.Add(normalB)
	mgr.sessions.Add(vip)
	mgr.playerCounts.Set("proxy-normal-a", ProxyStatus{OnlinePlayers: 200, MaxPlayers: 500})
	mgr.playerCounts.Set("proxy-normal-b", ProxyStatus{OnlinePlayers: 140, MaxPlayers: 500})
	mgr.playerCounts.Set("proxy-vip", ProxyStatus{OnlinePlayers: 12, MaxPlayers: 50})

	mgr.flushPlayerCounts()

	if counts, _ := nextPlayerCount(normalA); counts.OnlinePlayers != 340 || counts.MaxPlayers != 1000 {
		t.Errorf("normal listener got %+v, want {340 1000}", counts)
	}
	if counts, _ := nextPlayerCount(vip); counts.OnlinePlayers != 12 || counts.MaxPlayers != 50 {
		t.Errorf("vip listener got %+v, want {12 50}", counts)
	}
}

func TestListenerWithoutPlayerCountIsNeverPushed(t *testing.T) {
	mgr := managerWith(t, map[string]*PlayerCountConfig{"minecraft": nil})

	session := newProxySession(helloFor("proxy-a"))
	mgr.sessions.Add(session)
	mgr.playerCounts.Set("proxy-a", ProxyStatus{OnlinePlayers: 5, MaxPlayers: 100})

	mgr.flushPlayerCounts()

	if _, ok := nextPlayerCount(session); ok {
		t.Error("a listener that does not aggregate should keep the proxy's own numbers")
	}
}

func TestSilentProxiesAreNotToldTheNetworkIsEmpty(t *testing.T) {
	mgr := managerWith(t, map[string]*PlayerCountConfig{
		"minecraft": {Scope: PlayerCountScopeGateway},
	})

	session := newProxySession(helloFor("proxy-a"))
	mgr.sessions.Add(session)
	mgr.playerCounts.MarkDirty()

	mgr.flushPlayerCounts()

	if _, ok := nextPlayerCount(session); ok {
		t.Error("no proxy has reported a count, so none should be pushed")
	}
}

func TestUnchangedCountsAreNotResent(t *testing.T) {
	mgr := managerWith(t, map[string]*PlayerCountConfig{
		"minecraft": {Scope: PlayerCountScopeGateway},
	})

	session := newProxySession(helloFor("proxy-a"))
	mgr.sessions.Add(session)
	mgr.playerCounts.Set("proxy-a", ProxyStatus{OnlinePlayers: 5, MaxPlayers: 100})

	mgr.flushPlayerCounts()
	if _, ok := nextPlayerCount(session); !ok {
		t.Fatal("the first flush should push the count")
	}

	mgr.playerCounts.MarkDirty()
	mgr.flushPlayerCounts()
	if _, ok := nextPlayerCount(session); ok {
		t.Error("an unchanged count should not be pushed again")
	}
}

func TestFlushIsSkippedWhenNothingChanged(t *testing.T) {
	mgr := managerWith(t, map[string]*PlayerCountConfig{
		"minecraft": {Scope: PlayerCountScopeGateway},
	})

	session := newProxySession(helloFor("proxy-a"))
	mgr.sessions.Add(session)
	mgr.playerCounts.Set("proxy-a", ProxyStatus{OnlinePlayers: 5, MaxPlayers: 100})
	mgr.flushPlayerCounts()
	nextPlayerCount(session)

	// A repeat of the same report leaves nothing to recompute.
	mgr.playerCounts.Set("proxy-a", ProxyStatus{OnlinePlayers: 5, MaxPlayers: 100})
	if mgr.playerCounts.takeDirty() {
		t.Error("an identical report should not mark the aggregate dirty")
	}
}

func TestProxyIsToldToStopAggregatingWhenTheSettingIsRemoved(t *testing.T) {
	mgr := managerWith(t, map[string]*PlayerCountConfig{
		"minecraft": {Scope: PlayerCountScopeGateway},
	})

	session := newProxySession(helloFor("proxy-a"))
	mgr.sessions.Add(session)
	mgr.playerCounts.Set("proxy-a", ProxyStatus{OnlinePlayers: 5, MaxPlayers: 100})
	mgr.flushPlayerCounts()
	if _, ok := nextPlayerCount(session); !ok {
		t.Fatal("the proxy should first be given an aggregate")
	}

	reconfigure(t, mgr, map[string]*PlayerCountConfig{"minecraft": nil})
	mgr.flushPlayerCounts()

	sync := nextPlayerCountMessage(session)
	if sync == nil || sync.GetAggregated() {
		t.Errorf("got %v, want a message clearing the aggregate", sync)
	}

	// Nothing left to undo, so the proxy is not told again.
	mgr.playerCounts.MarkDirty()
	mgr.flushPlayerCounts()
	if repeat := nextPlayerCountMessage(session); repeat != nil {
		t.Errorf("unexpected repeat message %v", repeat)
	}
}

// nextPlayerCountMessage returns the last PlayerCountSync queued for a session,
// ignoring the server syncs a configuration change also pushes.
func nextPlayerCountMessage(s *ProxySession) *apiv1alpha1.PlayerCountSync {
	var last *apiv1alpha1.PlayerCountSync
	for {
		select {
		case msg := <-s.Outgoing():
			if sync := msg.GetPlayerCount(); sync != nil {
				last = sync
			}
		default:
			return last
		}
	}
}
