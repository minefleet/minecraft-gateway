package network

import (
	"context"
	"strings"
	"testing"
	"time"

	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/apimachinery/pkg/types"
	mcgatewayv1alpha1 "minefleet.dev/minecraft-gateway/api/controller/v1alpha1"
)

const (
	alice = "11111111-1111-1111-1111-111111111111"
	bob   = "22222222-2222-2222-2222-222222222222"
)

// moveHarness is a StreamManager with one connected proxy and one service, plus
// the engine under test. The proxy's outgoing queue stands in for a real stream.
type moveHarness struct {
	mgr     *StreamManager
	engine  *MoveEngine
	session *ProxySession
}

func newMoveHarness(t *testing.T, servers ...*Server) *moveHarness {
	t.Helper()
	gw := types.NamespacedName{Namespace: "default", Name: "gw"}
	cache := GatewaySnapshotCache{gw: map[string]ListenerSnapshot{
		"minecraft": {
			GatewayNamespace: gw.Namespace,
			GatewayName:      gw.Name,
			ListenerName:     "minecraft",
			Services: []*Service{{
				NamespacedName: "default/arena",
				Namespace:      "default",
				Name:           "arena",
				Labels:         map[string]string{"role": "arena"},
				Servers:        servers,
				JoinRoutes:     []Route{{Priority: 0}},
				FallbackRoutes: []Route{{Priority: 0}},
			}},
		},
	}}

	mgr := NewStreamManager()
	mgr.UpdateSnapshot(BuildSnapshot(cache))
	session := newProxySession(helloFor("proxy-a"))
	mgr.sessions.Add(session)

	return &moveHarness{mgr: mgr, engine: NewMoveEngine(mgr), session: session}
}

// connect marks a player as present on the harness's listener.
func (h *moveHarness) connect(playerUUID, server string) {
	h.mgr.presence.Set(Presence{
		PlayerUUID:       playerUUID,
		ProxyID:          "proxy-a",
		ServerName:       server,
		GatewayNamespace: "default",
		GatewayName:      "gw",
		ListenerName:     "minecraft",
	})
}

// replyToMoves answers every move command the proxy is sent, as a real proxy
// would, and reports which players it was asked to move.
func (h *moveHarness) replyToMoves(success bool, reason string) <-chan []string {
	moved := make(chan []string, 1)
	go func() {
		var seen []string
		for msg := range h.session.Outgoing() {
			move := msg.GetMove()
			if move == nil {
				continue
			}
			seen = append(seen, move.GetPlayerUuid())
			h.mgr.publishMoveResult(MoveOutcome{
				CommandID: move.GetCommandId(),
				Success:   success,
				Reason:    reason,
			})
			select {
			case moved <- seen:
			default:
			}
		}
	}()
	return moved
}

func serverNamed(name string) *Server {
	return &Server{UniqueID: name, Name: name, IP: "10.0.0.1", Port: 25565}
}

func resultFor(results []PlayerMoveResult, playerUUID string) PlayerMoveResult {
	for _, result := range results {
		if result.PlayerUUID == playerUUID {
			return result
		}
	}
	return PlayerMoveResult{}
}

func TestMoveToNamedServer(t *testing.T) {
	h := newMoveHarness(t, serverNamed("arena-0"))
	h.connect(alice, "lobby-0")
	h.replyToMoves(true, "")

	results, err := h.engine.Execute(context.Background(), MoveOrder{
		GatewayNamespace: "default", GatewayName: "gw", ListenerName: "minecraft",
		Players: []string{alice},
		Target:  MoveTarget{ServerName: "arena-0"},
	})
	if err != nil {
		t.Fatalf("Execute: %v", err)
	}

	result := resultFor(results, alice)
	if !result.Success || result.ServerName != "arena-0" {
		t.Errorf("got %+v, want a successful move to arena-0", result)
	}
}

func TestMoveBySelectorSendsEveryoneToOneServer(t *testing.T) {
	h := newMoveHarness(t, serverNamed("arena-0"))
	h.connect(alice, "lobby-0")
	h.connect(bob, "lobby-1")
	h.replyToMoves(true, "")

	selector, err := labels.Parse("role=arena")
	if err != nil {
		t.Fatal(err)
	}

	results, err := h.engine.Execute(context.Background(), MoveOrder{
		GatewayNamespace: "default", GatewayName: "gw", ListenerName: "minecraft",
		Players: []string{alice, bob},
		Target:  MoveTarget{Selector: selector},
	})
	if err != nil {
		t.Fatalf("Execute: %v", err)
	}

	for _, result := range results {
		if !result.Success || result.ServerName != "arena-0" {
			t.Errorf("%s got %+v, want a successful move to arena-0", result.PlayerUUID, result)
		}
	}
}

func TestMoveByRouteResolvesPerPlayer(t *testing.T) {
	h := newMoveHarness(t, serverNamed("arena-0"))
	h.connect(alice, "lobby-0")
	h.replyToMoves(true, "")

	kind := RouteKindJoin
	results, err := h.engine.Execute(context.Background(), MoveOrder{
		GatewayNamespace: "default", GatewayName: "gw", ListenerName: "minecraft",
		Players: []string{alice},
		Target:  MoveTarget{Route: &kind},
	})
	if err != nil {
		t.Fatalf("Execute: %v", err)
	}
	if result := resultFor(results, alice); !result.Success || result.ServerName != "arena-0" {
		t.Errorf("got %+v, want a successful routed move", result)
	}
}

func TestAllOrNothingMovesNobodyWhenOnePlayerIsAbsent(t *testing.T) {
	h := newMoveHarness(t, serverNamed("arena-0"))
	h.connect(alice, "lobby-0")
	moved := h.replyToMoves(true, "")

	results, err := h.engine.Execute(context.Background(), MoveOrder{
		GatewayNamespace: "default", GatewayName: "gw", ListenerName: "minecraft",
		Players: []string{alice, bob},
		Target:  MoveTarget{ServerName: "arena-0"},
		Policy:  MoveAllOrNothing,
	})
	if err != nil {
		t.Fatalf("Execute: %v", err)
	}

	for _, result := range results {
		if result.Success {
			t.Errorf("%s was moved; AllOrNothing must move nobody", result.PlayerUUID)
		}
		if result.Reason != mcgatewayv1alpha1.PlayerAssignmentReasonNotConnected {
			t.Errorf("%s reason = %q, want PlayerNotConnected", result.PlayerUUID, result.Reason)
		}
	}
	select {
	case players := <-moved:
		t.Errorf("proxy was asked to move %v", players)
	default:
	}
}

func TestBestEffortMovesThePlayersItCan(t *testing.T) {
	h := newMoveHarness(t, serverNamed("arena-0"))
	h.connect(alice, "lobby-0")
	h.replyToMoves(true, "")

	results, err := h.engine.Execute(context.Background(), MoveOrder{
		GatewayNamespace: "default", GatewayName: "gw", ListenerName: "minecraft",
		Players: []string{alice, bob},
		Target:  MoveTarget{ServerName: "arena-0"},
		Policy:  MoveBestEffort,
	})
	if err != nil {
		t.Fatalf("Execute: %v", err)
	}

	if result := resultFor(results, alice); !result.Success {
		t.Errorf("alice got %+v, want a successful move", result)
	}
	if result := resultFor(results, bob); result.Success ||
		result.Reason != mcgatewayv1alpha1.PlayerAssignmentReasonNotConnected {
		t.Errorf("bob got %+v, want PlayerNotConnected", result)
	}
}

func TestWaitLetsALatecomerJoinTheMove(t *testing.T) {
	h := newMoveHarness(t, serverNamed("arena-0"))
	h.connect(alice, "lobby-0")
	h.replyToMoves(true, "")

	// Bob arrives after the call has already started waiting.
	go func() {
		time.Sleep(50 * time.Millisecond)
		h.connect(bob, "lobby-1")
	}()

	start := time.Now()
	results, err := h.engine.Execute(context.Background(), MoveOrder{
		GatewayNamespace: "default", GatewayName: "gw", ListenerName: "minecraft",
		Players: []string{alice, bob},
		Target:  MoveTarget{ServerName: "arena-0"},
		Policy:  MoveAllOrNothing,
		Wait:    5 * time.Second,
	})
	if err != nil {
		t.Fatalf("Execute: %v", err)
	}

	for _, result := range results {
		if !result.Success {
			t.Errorf("%s got %+v, want a successful move", result.PlayerUUID, result)
		}
	}
	// The arrival must wake the call, not the timer.
	if elapsed := time.Since(start); elapsed > 2*time.Second {
		t.Errorf("took %v; a player arriving should wake the call immediately", elapsed)
	}
}

func TestPlayerOnAnotherListenerCountsAsAbsent(t *testing.T) {
	h := newMoveHarness(t, serverNamed("arena-0"))
	h.mgr.presence.Set(Presence{
		PlayerUUID:       alice,
		ProxyID:          "proxy-vip",
		ServerName:       "lobby-0",
		GatewayNamespace: "default",
		GatewayName:      "gw",
		ListenerName:     "vip",
	})
	h.replyToMoves(true, "")

	results, err := h.engine.Execute(context.Background(), MoveOrder{
		GatewayNamespace: "default", GatewayName: "gw", ListenerName: "minecraft",
		Players: []string{alice},
		Target:  MoveTarget{ServerName: "arena-0"},
		Policy:  MoveBestEffort,
	})
	if err != nil {
		t.Fatalf("Execute: %v", err)
	}
	if result := resultFor(results, alice); result.Success {
		t.Error("a player on another listener must not be moved off it")
	}
}

func TestProxyFailureIsReportedPerPlayer(t *testing.T) {
	h := newMoveHarness(t, serverNamed("arena-0"))
	h.connect(alice, "lobby-0")
	h.replyToMoves(false, "server_not_registered")

	results, err := h.engine.Execute(context.Background(), MoveOrder{
		GatewayNamespace: "default", GatewayName: "gw", ListenerName: "minecraft",
		Players: []string{alice},
		Target:  MoveTarget{ServerName: "arena-0"},
	})
	if err != nil {
		t.Fatalf("Execute: %v", err)
	}
	if result := resultFor(results, alice); result.Success || result.Reason != "server_not_registered" {
		t.Errorf("got %+v, want the proxy's own failure reason", result)
	}
}

func TestUnknownListenerIsRejected(t *testing.T) {
	h := newMoveHarness(t, serverNamed("arena-0"))

	if _, err := h.engine.Execute(context.Background(), MoveOrder{
		GatewayNamespace: "default", GatewayName: "gw", ListenerName: "nope",
		Players: []string{alice},
		Target:  MoveTarget{ServerName: "arena-0"},
	}); err == nil {
		t.Error("a move addressed to an unknown listener should be rejected")
	}
}

func TestTransferMovesAreNotDeliveredToTheEngine(t *testing.T) {
	h := newMoveHarness(t, serverNamed("arena-0"))
	h.connect(alice, "lobby-0")

	// A PlayerTransfer's result, which the engine must ignore.
	h.mgr.publishMoveResult(MoveOutcome{CommandID: "default/xfer#" + alice, Success: true})

	go func() {
		for msg := range h.session.Outgoing() {
			if move := msg.GetMove(); move != nil {
				if !strings.HasPrefix(move.GetCommandId(), apiMoveCommandPrefix) {
					t.Errorf("engine dispatched command id %q without its own prefix", move.GetCommandId())
				}
				h.mgr.publishMoveResult(MoveOutcome{CommandID: move.GetCommandId(), Success: true})
			}
		}
	}()

	results, err := h.engine.Execute(context.Background(), MoveOrder{
		GatewayNamespace: "default", GatewayName: "gw", ListenerName: "minecraft",
		Players: []string{alice},
		Target:  MoveTarget{ServerName: "arena-0"},
	})
	if err != nil {
		t.Fatalf("Execute: %v", err)
	}
	if result := resultFor(results, alice); !result.Success {
		t.Errorf("got %+v, want the engine's own result to settle the move", result)
	}
}
