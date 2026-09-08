package network

import (
	"fmt"
	"sync"

	apiv1alpha1 "minefleet.dev/minecraft-gateway/api/network/v1alpha1"
)

// MoveRequest asks a proxy to move one player to a server.
type MoveRequest struct {
	CommandID  string
	PlayerUUID string
	ServerName string
}

// StreamManager owns the controller's live view of the proxies: which are
// connected, where every player is, and what the current routing configuration
// is. It is shared between the dataplane (which pushes configuration) and the
// PlayerTransfer reconciler (which reads presence and sends move commands).
type StreamManager struct {
	sessions *SessionRegistry
	presence *PresenceMap
	router   *Router

	mu sync.RWMutex
	// configs holds the latest routing configuration per gateway listener.
	configs map[string]*ListenerSnapshot

	moveResults []func(MoveOutcome)
}

// MoveOutcome is a proxy's report on a move command.
type MoveOutcome struct {
	CommandID string
	Success   bool
	Reason    string
}

func NewStreamManager() *StreamManager {
	presence := NewPresenceMap()
	return &StreamManager{
		sessions: NewSessionRegistry(),
		presence: presence,
		router:   NewRouter(presence),
		configs:  make(map[string]*ListenerSnapshot),
	}
}

// Presence exposes the global player presence map.
func (m *StreamManager) Presence() *PresenceMap { return m.presence }

// Lookup reports where a player currently is.
func (m *StreamManager) Lookup(playerUUID string) (Presence, bool) {
	return m.presence.Lookup(playerUUID)
}

// Router exposes the routing engine.
func (m *StreamManager) Router() *Router { return m.router }

// Sessions exposes the connected proxies.
func (m *StreamManager) Sessions() *SessionRegistry { return m.sessions }

// OnMoveResult registers a callback for proxy move reports. The PlayerTransfer
// reconciler uses it to advance a transfer as results arrive.
func (m *StreamManager) OnMoveResult(fn func(MoveOutcome)) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.moveResults = append(m.moveResults, fn)
}

func (m *StreamManager) publishMoveResult(outcome MoveOutcome) {
	m.mu.RLock()
	subscribers := append([]func(MoveOutcome){}, m.moveResults...)
	m.mu.RUnlock()
	for _, fn := range subscribers {
		fn(outcome)
	}
}

// UpdateSnapshot replaces the routing configuration for every listener and
// pushes the resulting server sets to the proxies serving them.
func (m *StreamManager) UpdateSnapshot(snap Snapshot) {
	configs := make(map[string]*ListenerSnapshot)
	for _, ls := range snap.All() {
		configs[listenerKey(ls.GatewayNamespace, ls.GatewayName, ls.ListenerName)] = ls
	}

	m.mu.Lock()
	m.configs = configs
	m.mu.Unlock()

	for _, ls := range snap.All() {
		m.pushServerSync(ls)
	}
}

// ConfigFor returns the routing configuration of a gateway listener.
func (m *StreamManager) ConfigFor(namespace, name, listener string) *ListenerSnapshot {
	m.mu.RLock()
	defer m.mu.RUnlock()
	return m.configs[listenerKey(namespace, name, listener)]
}

// pushServerSync sends the full server set and required permissions to every
// proxy serving the listener. The full set is sent every time; proxies diff it
// against what they already have registered.
func (m *StreamManager) pushServerSync(ls *ListenerSnapshot) {
	sessions := m.sessions.ForListener(ls.GatewayNamespace, ls.GatewayName, ls.ListenerName)
	if len(sessions) == 0 {
		return
	}
	msg := serverSyncMessage(ls)
	for _, s := range sessions {
		if err := s.Send(msg); err != nil {
			// The proxy is too far behind to keep in sync; drop it and let it
			// reconnect with a clean replay.
			s.Close()
		}
	}
}

func serverSyncMessage(ls *ListenerSnapshot) *apiv1alpha1.ControllerMessage {
	servers := ls.Servers()
	protoServers := make([]*apiv1alpha1.ManagedServer, 0, len(servers))
	for _, s := range servers {
		protoServers = append(protoServers, &apiv1alpha1.ManagedServer{
			UniqueId:       s.UniqueID,
			Name:           s.Name,
			Ip:             s.IP,
			Port:           s.Port,
			MaxPlayers:     s.MaxPlayers,
			CurrentPlayers: s.CurrentPlayers,
		})
	}
	return &apiv1alpha1.ControllerMessage{
		Message: &apiv1alpha1.ControllerMessage_ServerSync{
			ServerSync: &apiv1alpha1.ServerSync{
				Servers:             protoServers,
				RequiredPermissions: ls.RequiredPermissions(),
			},
		},
	}
}

// SendMove asks the proxy holding a player to move them. It reports an error if
// the proxy is no longer connected.
func (m *StreamManager) SendMove(proxyID string, req MoveRequest) error {
	session, ok := m.sessions.Get(proxyID)
	if !ok {
		return fmt.Errorf("proxy %s is not connected", proxyID)
	}
	err := session.Send(&apiv1alpha1.ControllerMessage{
		Message: &apiv1alpha1.ControllerMessage_Move{
			Move: &apiv1alpha1.MoveCommand{
				CommandId:  req.CommandID,
				PlayerUuid: req.PlayerUUID,
				ServerName: req.ServerName,
			},
		},
	})
	if err != nil {
		session.Close()
		return fmt.Errorf("send move to proxy %s: %w", proxyID, err)
	}
	// A move occupies capacity on the target just like a routing decision does.
	m.router.Reserve(req.ServerName)
	return nil
}

// ResolveRoute picks a server for a player using the listener's routes.
func (m *StreamManager) ResolveRoute(namespace, name, listener string, q RouteQuery) (*Server, bool) {
	return m.router.Route(m.ConfigFor(namespace, name, listener), q)
}

// ResolveSelector picks a single server from the services matching a label
// selector, honouring the service's distribution strategy. Every player of a
// selector-targeted transfer goes to that one server.
func (m *StreamManager) ResolveSelector(namespace, name, listener string, matches func(*Service) bool) (*Server, bool) {
	cfg := m.ConfigFor(namespace, name, listener)
	if cfg == nil {
		return nil, false
	}
	for _, svc := range cfg.Services {
		if !matches(svc) {
			continue
		}
		if server := m.router.pick(svc); server != nil {
			m.router.Reserve(server.Name)
			return server, true
		}
	}
	return nil, false
}

// PlayersForService returns the presence of every player on any server of a
// service, across all listeners.
func (m *StreamManager) PlayersForService(namespace, name string) []Presence {
	key := namespace + "/" + name
	m.mu.RLock()
	configs := make([]*ListenerSnapshot, 0, len(m.configs))
	for _, cfg := range m.configs {
		configs = append(configs, cfg)
	}
	m.mu.RUnlock()

	seen := make(map[string]struct{})
	var out []Presence
	for _, cfg := range configs {
		for _, svc := range cfg.Services {
			if svc.NamespacedName != key {
				continue
			}
			for _, server := range svc.Servers {
				for _, pr := range m.presence.PlayersForServer(server.Name) {
					if _, ok := seen[pr.PlayerUUID]; ok {
						continue
					}
					seen[pr.PlayerUUID] = struct{}{}
					out = append(out, pr)
				}
			}
		}
	}
	return out
}

func listenerKey(namespace, name, listener string) string {
	return namespace + "/" + name + "#" + listener
}
