package network

import (
	"errors"
	"sync"

	apiv1alpha1 "minefleet.dev/minecraft-gateway/api/network/v1alpha1"
)

// sendBuffer bounds how far a proxy may fall behind before its stream is torn
// down. Routing responses and move commands must not be dropped, so a proxy
// that cannot keep up is disconnected and replays its state on reconnect.
const sendBuffer = 64

// ErrSessionClosed is returned when sending to a proxy whose stream is gone.
var ErrSessionClosed = errors.New("proxy session closed")

// ProxySession is one connected proxy's half of the Connect stream.
type ProxySession struct {
	ProxyID          string
	GatewayNamespace string
	GatewayName      string
	ListenerName     string

	send chan *apiv1alpha1.ControllerMessage

	mu     sync.Mutex
	closed bool
}

func newProxySession(hello *apiv1alpha1.ProxyHello) *ProxySession {
	return &ProxySession{
		ProxyID:          hello.GetProxyId(),
		GatewayNamespace: hello.GetGatewayNamespace(),
		GatewayName:      hello.GetGatewayName(),
		ListenerName:     hello.GetListenerName(),
		send:             make(chan *apiv1alpha1.ControllerMessage, sendBuffer),
	}
}

// listenerKey identifies the gateway listener a session serves.
func (s *ProxySession) listenerKey() string {
	return s.GatewayNamespace + "/" + s.GatewayName + "#" + s.ListenerName
}

// Send queues a message to the proxy. It reports an error rather than blocking
// when the proxy has fallen too far behind; the caller closes the session.
func (s *ProxySession) Send(msg *apiv1alpha1.ControllerMessage) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.closed {
		return ErrSessionClosed
	}
	select {
	case s.send <- msg:
		return nil
	default:
		return ErrSessionClosed
	}
}

// Outgoing is the stream of messages to write to the proxy.
func (s *ProxySession) Outgoing() <-chan *apiv1alpha1.ControllerMessage {
	return s.send
}

// Close stops the session. It is safe to call more than once.
func (s *ProxySession) Close() {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.closed {
		return
	}
	s.closed = true
	close(s.send)
}

// SessionRegistry tracks the connected proxies, by proxy and by the listener
// they serve.
type SessionRegistry struct {
	mu         sync.RWMutex
	byProxy    map[string]*ProxySession
	byListener map[string]map[string]*ProxySession
}

func NewSessionRegistry() *SessionRegistry {
	return &SessionRegistry{
		byProxy:    make(map[string]*ProxySession),
		byListener: make(map[string]map[string]*ProxySession),
	}
}

// Add registers a session. A reconnecting proxy replaces and closes its
// previous session, which is returned so the caller can drop its presence.
func (r *SessionRegistry) Add(s *ProxySession) (replaced *ProxySession) {
	r.mu.Lock()
	defer r.mu.Unlock()

	if prev, ok := r.byProxy[s.ProxyID]; ok {
		r.removeLocked(prev)
		prev.Close()
		replaced = prev
	}
	r.byProxy[s.ProxyID] = s
	key := s.listenerKey()
	if _, ok := r.byListener[key]; !ok {
		r.byListener[key] = make(map[string]*ProxySession)
	}
	r.byListener[key][s.ProxyID] = s
	return replaced
}

// Remove deregisters a session, unless it has already been replaced.
func (r *SessionRegistry) Remove(s *ProxySession) {
	r.mu.Lock()
	defer r.mu.Unlock()
	if current, ok := r.byProxy[s.ProxyID]; !ok || current != s {
		return
	}
	r.removeLocked(s)
}

func (r *SessionRegistry) removeLocked(s *ProxySession) {
	delete(r.byProxy, s.ProxyID)
	key := s.listenerKey()
	if sessions, ok := r.byListener[key]; ok {
		delete(sessions, s.ProxyID)
		if len(sessions) == 0 {
			delete(r.byListener, key)
		}
	}
}

// Get returns a session by proxy id.
func (r *SessionRegistry) Get(proxyID string) (*ProxySession, bool) {
	r.mu.RLock()
	defer r.mu.RUnlock()
	s, ok := r.byProxy[proxyID]
	return s, ok
}

// ForListener returns every proxy serving a gateway listener.
func (r *SessionRegistry) ForListener(namespace, name, listener string) []*ProxySession {
	r.mu.RLock()
	defer r.mu.RUnlock()
	sessions := r.byListener[namespace+"/"+name+"#"+listener]
	out := make([]*ProxySession, 0, len(sessions))
	for _, s := range sessions {
		out = append(out, s)
	}
	return out
}

// Len returns the number of connected proxies.
func (r *SessionRegistry) Len() int {
	r.mu.RLock()
	defer r.mu.RUnlock()
	return len(r.byProxy)
}
