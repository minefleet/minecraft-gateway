package network

import (
	"k8s.io/apimachinery/pkg/labels"
	mcgatewayv1alpha1 "minefleet.dev/minecraft-gateway/api/controller/v1alpha1"
)

// This file holds the targeting rules shared by the two ways a player can be
// moved: the PlayerTransfer resource and the MovePlayers RPC. Both must pick
// destinations identically, so the rules live here rather than in either caller.

// MoveTarget selects where players are moved to. Exactly one field is set,
// mirroring PlayerTransferTarget.
type MoveTarget struct {
	// Route evaluates the listener's join or fallback routes per player, so
	// players may land on different servers.
	Route *RouteKind
	// Selector picks one server from the services it matches. Every player of
	// the batch goes to that one server.
	Selector labels.Selector
	// ServerName names an already registered server explicitly.
	ServerName string
}

// IsShared reports whether the target resolves once for the whole batch rather
// than per player.
func (t MoveTarget) IsShared() bool { return t.Selector != nil }

// TargetResolver is the routing surface needed to resolve a move target. Both
// StreamManager and the reconciler's narrowed stream interface satisfy it.
type TargetResolver interface {
	ResolveRoute(namespace, name, listener string, q RouteQuery) (*Server, bool)
	ResolveSelector(namespace, name, listener string, matches func(*Service) bool) (*Server, bool)
}

// ResolveSharedServer resolves a selector target to the single server every
// player of the batch is moved to.
func ResolveSharedServer(r TargetResolver, namespace, name, listener string, selector labels.Selector) (*Server, bool) {
	if selector == nil || selector.Empty() {
		return nil, false
	}
	return r.ResolveSelector(namespace, name, listener, func(svc *Service) bool {
		return selector.Matches(labels.Set(svc.Labels))
	})
}

// ResolveMoveTarget picks the destination server for one player, returning the
// server name or the reason no destination could be found. shared carries the
// already-resolved server for selector targets and is nil otherwise.
func ResolveMoveTarget(r TargetResolver, namespace, name, listener string, target MoveTarget, presence Presence, shared *Server) (serverName, reason string) {
	switch {
	case target.ServerName != "":
		// The caller already picked the server; the proxy resolves it by name.
		return target.ServerName, ""

	case shared != nil:
		return shared.Name, ""

	case target.Route != nil:
		server, ok := r.ResolveRoute(namespace, name, listener, RouteQuery{
			PlayerUUID:        presence.PlayerUUID,
			Kind:              *target.Route,
			Context:           presence.Context,
			CurrentServerName: presence.ServerName,
		})
		if !ok {
			return "", mcgatewayv1alpha1.PlayerAssignmentReasonNoMatchingRoute
		}
		return server.Name, ""
	}

	return "", mcgatewayv1alpha1.PlayerAssignmentReasonNoMatchingServer
}

// PresenceOnListener returns a player's presence only if they are connected to
// the given gateway listener. A player who is online elsewhere counts as absent:
// moving them would take them off a listener the caller never addressed.
func PresenceOnListener(p *PresenceMap, namespace, name, listener, playerUUID string) (Presence, bool) {
	presence, ok := p.Lookup(playerUUID)
	if !ok {
		return Presence{}, false
	}
	if presence.GatewayNamespace != namespace ||
		presence.GatewayName != name ||
		presence.ListenerName != listener {
		return Presence{}, false
	}
	return presence, true
}
