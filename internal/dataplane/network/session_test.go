package network

import (
	"testing"

	apiv1alpha1 "minefleet.dev/minecraft-gateway/api/network/v1alpha1"
)

func helloFor(proxyID string) *apiv1alpha1.ProxyHello {
	return &apiv1alpha1.ProxyHello{
		ProxyId:          proxyID,
		GatewayNamespace: "default",
		GatewayName:      "gw",
		ListenerName:     "minecraft",
	}
}

func TestRegistryReplacesReconnectingProxy(t *testing.T) {
	r := NewSessionRegistry()
	first := newProxySession(helloFor("proxy-a"))
	r.Add(first)

	second := newProxySession(helloFor("proxy-a"))
	replaced := r.Add(second)

	if replaced != first {
		t.Fatal("reconnecting with the same proxy id should replace the previous session")
	}
	if err := first.Send(&apiv1alpha1.ControllerMessage{}); err == nil {
		t.Error("the replaced session should be closed")
	}

	current, ok := r.Get("proxy-a")
	if !ok || current != second {
		t.Error("the registry should hold the new session")
	}
	if got := r.Len(); got != 1 {
		t.Errorf("Len = %d, want 1", got)
	}
}

func TestRegistryForListener(t *testing.T) {
	r := NewSessionRegistry()
	r.Add(newProxySession(helloFor("proxy-a")))
	r.Add(newProxySession(helloFor("proxy-b")))

	other := helloFor("proxy-c")
	other.ListenerName = "other"
	r.Add(newProxySession(other))

	if got := len(r.ForListener("default", "gw", "minecraft")); got != 2 {
		t.Errorf("ForListener returned %d sessions, want 2", got)
	}
	if got := len(r.ForListener("default", "gw", "other")); got != 1 {
		t.Errorf("ForListener(other) returned %d sessions, want 1", got)
	}
	if got := len(r.ForListener("default", "gw", "missing")); got != 0 {
		t.Errorf("ForListener(missing) returned %d sessions, want 0", got)
	}
}

func TestRemoveIgnoresAlreadyReplacedSession(t *testing.T) {
	r := NewSessionRegistry()
	first := newProxySession(helloFor("proxy-a"))
	r.Add(first)
	second := newProxySession(helloFor("proxy-a"))
	r.Add(second)

	// The old stream's cleanup must not evict the session that replaced it.
	r.Remove(first)

	if _, ok := r.Get("proxy-a"); !ok {
		t.Error("the current session should still be registered")
	}
}

func TestSendFailsWhenProxyFallsBehind(t *testing.T) {
	s := newProxySession(helloFor("proxy-a"))
	for range sendBuffer {
		if err := s.Send(&apiv1alpha1.ControllerMessage{}); err != nil {
			t.Fatalf("unexpected error filling the buffer: %v", err)
		}
	}

	// Routing decisions must never be silently dropped; the caller tears the
	// session down instead and the proxy reconnects.
	if err := s.Send(&apiv1alpha1.ControllerMessage{}); err == nil {
		t.Error("expected an error once the send buffer is full")
	}
}

func TestCloseIsIdempotent(t *testing.T) {
	s := newProxySession(helloFor("proxy-a"))
	s.Close()
	s.Close()
}
