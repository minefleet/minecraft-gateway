package network

import (
	"context"
	"sync"
	"time"
)

// playerCountFlushInterval bounds how often aggregated counts are pushed to the
// proxies. Every join and quit changes the total, so pushing on each one would
// send a message to every proxy of a gateway for every player movement. A ping
// showing a count a second old is indistinguishable from a live one.
const playerCountFlushInterval = time.Second

// ProxyStatus is what one proxy reports about itself: the players it currently
// holds and the capacity its own configuration admits (Velocity's
// show-max-players). The controller never derives these — a proxy's config is
// the only thing that knows how many connections it will take.
type ProxyStatus struct {
	OnlinePlayers uint32
	MaxPlayers    uint32
}

// PlayerCounts is a total summed over some set of proxies.
type PlayerCounts struct {
	OnlinePlayers uint32
	MaxPlayers    uint32
}

// sentCounts is the last thing a proxy was told, including the case of being
// told to stop using an aggregate.
type sentCounts struct {
	aggregated bool
	counts     PlayerCounts
}

// PlayerCountAggregator holds the latest self-report of every connected proxy
// and sums them on demand. Reports are keyed by proxy id, so a proxy that
// reconnects replaces its previous figures rather than double-counting, and a
// proxy that disconnects stops advertising capacity nobody can reach.
type PlayerCountAggregator struct {
	mu       sync.RWMutex
	byProxy  map[string]ProxyStatus
	lastSent map[string]sentCounts
	dirty    bool
}

func NewPlayerCountAggregator() *PlayerCountAggregator {
	return &PlayerCountAggregator{
		byProxy:  make(map[string]ProxyStatus),
		lastSent: make(map[string]sentCounts),
	}
}

// Set records a proxy's latest self-report.
func (a *PlayerCountAggregator) Set(proxyID string, status ProxyStatus) {
	a.mu.Lock()
	defer a.mu.Unlock()
	if prev, ok := a.byProxy[proxyID]; ok && prev == status {
		return
	}
	a.byProxy[proxyID] = status
	a.dirty = true
}

// Drop forgets a proxy, e.g. when its stream closes.
func (a *PlayerCountAggregator) Drop(proxyID string) {
	a.mu.Lock()
	defer a.mu.Unlock()
	if _, ok := a.byProxy[proxyID]; !ok {
		delete(a.lastSent, proxyID)
		return
	}
	delete(a.byProxy, proxyID)
	delete(a.lastSent, proxyID)
	a.dirty = true
}

// MarkDirty forces the next flush to recompute, for changes that are not a
// proxy report — a listener's configuration changing, say.
func (a *PlayerCountAggregator) MarkDirty() {
	a.mu.Lock()
	defer a.mu.Unlock()
	a.dirty = true
}

// takeDirty reports whether anything changed since the last flush, clearing the
// flag.
func (a *PlayerCountAggregator) takeDirty() bool {
	a.mu.Lock()
	defer a.mu.Unlock()
	was := a.dirty
	a.dirty = false
	return was
}

// Total sums the reports of the named proxies. It reports false when none of
// them has ever reported, so a proxy running an integration that does not send
// its status is never told the network is empty.
func (a *PlayerCountAggregator) Total(proxyIDs []string) (PlayerCounts, bool) {
	a.mu.RLock()
	defer a.mu.RUnlock()

	var total PlayerCounts
	known := false
	for _, id := range proxyIDs {
		status, ok := a.byProxy[id]
		if !ok {
			continue
		}
		known = true
		total.OnlinePlayers += status.OnlinePlayers
		total.MaxPlayers += status.MaxPlayers
	}
	return total, known
}

// shouldSend reports whether a proxy still needs to be told these counts,
// recording them as sent when it does.
func (a *PlayerCountAggregator) shouldSend(proxyID string, counts PlayerCounts) bool {
	a.mu.Lock()
	defer a.mu.Unlock()
	next := sentCounts{aggregated: true, counts: counts}
	if prev, ok := a.lastSent[proxyID]; ok && prev == next {
		return false
	}
	a.lastSent[proxyID] = next
	return true
}

// shouldDisable reports whether a proxy is still holding an aggregate it should
// be told to drop, because its listener has stopped aggregating. A proxy that
// was never given one has nothing to undo.
func (a *PlayerCountAggregator) shouldDisable(proxyID string) bool {
	a.mu.Lock()
	defer a.mu.Unlock()
	prev, ok := a.lastSent[proxyID]
	if !ok || !prev.aggregated {
		return false
	}
	a.lastSent[proxyID] = sentCounts{}
	return true
}

// RunPlayerCounts pushes aggregated ping player counts to the proxies whose
// totals changed, until ctx is done. Updates are coalesced onto interval.
func (m *StreamManager) RunPlayerCounts(ctx context.Context, interval time.Duration) {
	ticker := time.NewTicker(interval)
	defer ticker.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			m.flushPlayerCounts()
		}
	}
}

// flushPlayerCounts recomputes every listener's aggregate and pushes it to the
// proxies whose figures changed. Per-listener totals are always published as
// metrics, whether or not the gateway asked for aggregated ping responses.
func (m *StreamManager) flushPlayerCounts() {
	if !m.playerCounts.takeDirty() {
		return
	}

	for _, session := range m.sessions.All() {
		cfg := m.ConfigFor(session.GatewayNamespace, session.GatewayName, session.ListenerName)
		if cfg == nil || cfg.PlayerCount == nil {
			// The listener does not aggregate. A proxy still holding a total
			// from before the setting was removed is told to drop it and go
			// back to its own numbers.
			if m.playerCounts.shouldDisable(session.ProxyID) {
				if err := session.Send(playerCountDisabledMessage()); err != nil {
					session.Close()
				}
			}
			continue
		}

		peers := m.sessions.ForListener(session.GatewayNamespace, session.GatewayName, session.ListenerName)
		if cfg.PlayerCount.Scope == PlayerCountScopeGateway {
			peers = m.sessions.ForGateway(session.GatewayNamespace, session.GatewayName)
		}

		counts, known := m.playerCounts.Total(proxyIDsOf(peers))
		if !known || !m.playerCounts.shouldSend(session.ProxyID, counts) {
			continue
		}
		if err := session.Send(playerCountMessage(counts)); err != nil {
			session.Close()
		}
	}

	m.publishPlayerCountMetrics()
}

// publishPlayerCountMetrics exports the per-listener totals, which are the raw
// sums of what each proxy reports and so are independent of the aggregation
// scope a gateway configured for its ping responses.
func (m *StreamManager) publishPlayerCountMetrics() {
	byListener := make(map[listenerLabels]PlayerCounts)
	for _, session := range m.sessions.All() {
		labels := listenerLabels{
			gateway:  session.gatewayKey(),
			listener: session.ListenerName,
		}
		counts, known := m.playerCounts.Total([]string{session.ProxyID})
		if !known {
			// Keep the listener in the map so a listener whose proxies are all
			// silent still reports zero rather than vanishing from the metric.
			if _, ok := byListener[labels]; !ok {
				byListener[labels] = PlayerCounts{}
			}
			continue
		}
		total := byListener[labels]
		total.OnlinePlayers += counts.OnlinePlayers
		total.MaxPlayers += counts.MaxPlayers
		byListener[labels] = total
	}
	setPlayerCountMetrics(byListener)
}

func proxyIDsOf(sessions []*ProxySession) []string {
	ids := make([]string, 0, len(sessions))
	for _, s := range sessions {
		ids = append(ids, s.ProxyID)
	}
	return ids
}
