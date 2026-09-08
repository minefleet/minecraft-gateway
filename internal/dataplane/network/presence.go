package network

import (
	"sync"
	"time"
)

// PlayerContext is the proxy-side knowledge needed to evaluate routing rules
// for a player. Permissions are pre-evaluated by the proxy against the
// permission set the controller last pushed, so rule evaluation never has to
// call back into a proxy mid-decision.
type PlayerContext struct {
	ConnectedDomain string
	Permissions     map[string]bool
}

// HasPermission reports whether the proxy evaluated this permission as granted.
func (c PlayerContext) HasPermission(permission string) bool {
	return c.Permissions[permission]
}

// Presence is where a player currently is.
type Presence struct {
	PlayerUUID       string
	ProxyID          string
	ServerName       string
	GatewayNamespace string
	GatewayName      string
	ListenerName     string
	Context          PlayerContext
	Since            time.Time
}

// PresenceMap is the controller's global view of which player is on which
// server, behind which proxy. It is in-memory only: proxies replay their full
// presence on every (re)connect, so the map rebuilds itself after a controller
// restart or leader failover.
type PresenceMap struct {
	mu        sync.RWMutex
	byPlayer  map[string]Presence
	byServer  map[string]map[string]struct{}
	onConnect []func(Presence)
}

func NewPresenceMap() *PresenceMap {
	return &PresenceMap{
		byPlayer: make(map[string]Presence),
		byServer: make(map[string]map[string]struct{}),
	}
}

// OnConnect registers a callback invoked after a player is recorded as
// connected. The PlayerTransfer reconciler uses it to react to pre-staged
// transfers the moment their players appear. Callbacks run without the lock
// held and must not block.
func (p *PresenceMap) OnConnect(fn func(Presence)) {
	p.mu.Lock()
	defer p.mu.Unlock()
	p.onConnect = append(p.onConnect, fn)
}

// Set records a player as present, replacing any previous location.
func (p *PresenceMap) Set(pr Presence) {
	p.mu.Lock()
	p.removeLocked(pr.PlayerUUID)
	if pr.Since.IsZero() {
		pr.Since = time.Now()
	}
	p.byPlayer[pr.PlayerUUID] = pr
	p.addToServerLocked(pr.ServerName, pr.PlayerUUID)
	subscribers := append([]func(Presence){}, p.onConnect...)
	p.mu.Unlock()

	for _, fn := range subscribers {
		fn(pr)
	}
}

// Remove drops a player, e.g. on disconnect.
func (p *PresenceMap) Remove(playerUUID string) {
	p.mu.Lock()
	defer p.mu.Unlock()
	p.removeLocked(playerUUID)
}

// ReplaceForProxy swaps in the full presence set reported by one proxy,
// dropping anything previously attributed to it. Used on stream (re)connect.
func (p *PresenceMap) ReplaceForProxy(proxyID string, entries []Presence) {
	p.mu.Lock()
	p.dropProxyLocked(proxyID)
	now := time.Now()
	for _, pr := range entries {
		if pr.Since.IsZero() {
			pr.Since = now
		}
		p.removeLocked(pr.PlayerUUID)
		p.byPlayer[pr.PlayerUUID] = pr
		p.addToServerLocked(pr.ServerName, pr.PlayerUUID)
	}
	subscribers := append([]func(Presence){}, p.onConnect...)
	replayed := append([]Presence{}, entries...)
	p.mu.Unlock()

	for _, fn := range subscribers {
		for _, pr := range replayed {
			fn(pr)
		}
	}
}

// DropProxy forgets every player behind a proxy, e.g. when its stream closes.
// The proxy is the source of truth and replays presence when it reconnects.
func (p *PresenceMap) DropProxy(proxyID string) {
	p.mu.Lock()
	defer p.mu.Unlock()
	p.dropProxyLocked(proxyID)
}

// Lookup returns where a player currently is.
func (p *PresenceMap) Lookup(playerUUID string) (Presence, bool) {
	p.mu.RLock()
	defer p.mu.RUnlock()
	pr, ok := p.byPlayer[playerUUID]
	return pr, ok
}

// CountForServer returns how many players are on a server.
func (p *PresenceMap) CountForServer(serverName string) int {
	p.mu.RLock()
	defer p.mu.RUnlock()
	return len(p.byServer[serverName])
}

// PlayersForServer returns the presence of every player on a server.
func (p *PresenceMap) PlayersForServer(serverName string) []Presence {
	p.mu.RLock()
	defer p.mu.RUnlock()
	out := make([]Presence, 0, len(p.byServer[serverName]))
	for uuid := range p.byServer[serverName] {
		if pr, ok := p.byPlayer[uuid]; ok {
			out = append(out, pr)
		}
	}
	return out
}

// Len returns the number of tracked players.
func (p *PresenceMap) Len() int {
	p.mu.RLock()
	defer p.mu.RUnlock()
	return len(p.byPlayer)
}

func (p *PresenceMap) removeLocked(playerUUID string) {
	prev, ok := p.byPlayer[playerUUID]
	if !ok {
		return
	}
	delete(p.byPlayer, playerUUID)
	if players, ok := p.byServer[prev.ServerName]; ok {
		delete(players, playerUUID)
		if len(players) == 0 {
			delete(p.byServer, prev.ServerName)
		}
	}
}

func (p *PresenceMap) dropProxyLocked(proxyID string) {
	for uuid, pr := range p.byPlayer {
		if pr.ProxyID == proxyID {
			p.removeLocked(uuid)
		}
	}
}

func (p *PresenceMap) addToServerLocked(serverName, playerUUID string) {
	if serverName == "" {
		return
	}
	if _, ok := p.byServer[serverName]; !ok {
		p.byServer[serverName] = make(map[string]struct{})
	}
	p.byServer[serverName][playerUUID] = struct{}{}
}
