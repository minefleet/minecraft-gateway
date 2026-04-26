package topology

import (
	"context"
	"errors"

	discoveryv1 "k8s.io/api/discovery/v1"
	apimeta "k8s.io/apimachinery/pkg/api/meta"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"minefleet.dev/minecraft-gateway/internal/gateway"
	"sigs.k8s.io/controller-runtime/pkg/client"
	gatewayv1 "sigs.k8s.io/gateway-api/apis/v1"
)

// GatewayTree is the fully-resolved state of one Gateway at reconcile time.
// It is the single value both the gateway reconciler and route reconcilers work
// with. Build constructs one; all status mutations write through the writer
// methods and are committed in a single patch via PatchGatewayStatus.
type GatewayTree struct {
	gw             *gatewayv1.Gateway // mutable — status written here
	base           *gatewayv1.Gateway // immutable deep copy for status patch
	Class          GatewayClass
	Infrastructure gateway.Infrastructure
	Backends       []discoveryv1.EndpointSlice
	listeners      []ListenerTree // in spec order
}

// NamespacedName returns the gateway's namespace/name key.
func (t *GatewayTree) NamespacedName() types.NamespacedName {
	return types.NamespacedName{Namespace: t.gw.Namespace, Name: t.gw.Name}
}

// Gateway returns the mutable underlying gateway object.
func (t *GatewayTree) Gateway() *gatewayv1.Gateway { return t.gw }

// GetListener returns the ListenerTree for the given section name, if present.
func (t *GatewayTree) GetListener(name gatewayv1.SectionName) (ListenerTree, bool) {
	for _, lt := range t.listeners {
		if lt.Listener.GetName() == name {
			return lt, true
		}
	}
	return ListenerTree{}, false
}

// Listeners returns all listener trees in spec order.
func (t *GatewayTree) Listeners() []ListenerTree { return t.listeners }

// EachRoute calls fn for every route across all listeners.
// A route appearing in multiple listeners is called once per listener.
func (t *GatewayTree) EachRoute(fn func(ListenerTree, Route)) {
	for _, lt := range t.listeners {
		lt.EachRoute(func(r Route) { fn(lt, r) })
	}
}

// AdmittedListeners returns the listeners that have admitted route r.
// Used by the route reconciler to evaluate the Accepted condition.
func (t *GatewayTree) AdmittedListeners(r Route) []ListenerTree {
	nn := types.NamespacedName{Namespace: r.GetNamespace(), Name: r.GetName()}
	var out []ListenerTree
	for _, lt := range t.listeners {
		if lt.hasRoute(nn) {
			out = append(out, lt)
		}
	}
	return out
}

// ─── Status Readers ──────────────────────────────────────────────────────────

// IsAccepted reports whether the gateway's Accepted condition is True.
func (t *GatewayTree) IsAccepted() bool {
	c := apimeta.FindStatusCondition(t.gw.Status.Conditions, string(gatewayv1.GatewayConditionAccepted))
	return c != nil && c.Status == metav1.ConditionTrue
}

// IsProgrammed reports whether the gateway's Programmed condition is True.
func (t *GatewayTree) IsProgrammed() bool {
	c := apimeta.FindStatusCondition(t.gw.Status.Conditions, string(gatewayv1.GatewayConditionProgrammed))
	return c != nil && c.Status == metav1.ConditionTrue
}

// ─── Status Writers ──────────────────────────────────────────────────────────

// StatusWriter returns a fluent writer for the gateway-level conditions.
func (t *GatewayTree) StatusWriter() *GatewayStatusWriter {
	return &GatewayStatusWriter{tree: t}
}

// PatchGatewayStatus commits all accumulated gateway and listener status
// mutations to the API server in one patch call.
func (t *GatewayTree) PatchGatewayStatus(ctx context.Context, c client.Client) error {
	return client.IgnoreNotFound(c.Status().Patch(ctx, t.gw, client.MergeFrom(t.base)))
}

// WriteRouteStatuses patches the Accepted condition on every route admitted to
// any listener. Each route is patched at most once even if it appears in
// multiple listeners.
func (t *GatewayTree) WriteRouteStatuses(ctx context.Context, c client.Client, selfConflicting bool) error {
	gwNN := t.NamespacedName()
	seen := make(map[types.NamespacedName]struct{})
	errs := make([]error, 0)
	t.EachRoute(func(lt ListenerTree, route Route) {
		if err := t.patchOneRouteStatus(ctx, c, seen, gwNN, route, lt.Listener, selfConflicting); err != nil {
			errs = append(errs, err)
		}
	})
	return errors.Join(errs...)
}

func (t *GatewayTree) patchOneRouteStatus(ctx context.Context, c client.Client, seen map[types.NamespacedName]struct{}, gwNN types.NamespacedName, r Route, listener Listener, conflict bool) error {
	nn := types.NamespacedName{Namespace: r.GetNamespace(), Name: r.GetName()}
	if _, ok := seen[nn]; ok {
		return nil
	}
	seen[nn] = struct{}{}
	w := NewRouteStatusWriter(r)
	w.SetAccepted(gwNN, listener, conflict)
	return w.Patch(ctx, c)
}

// ─── ListenerTree ────────────────────────────────────────────────────────────

// ListenerTree pairs a Listener with the routes that have been admitted to it.
// Obtain it via GatewayTree.GetListener or GatewayTree.Listeners.
type ListenerTree struct {
	Listener Listener
	routes   RouteBag
	gw       *gatewayv1.Gateway // back-reference for status writes
}

// Routes returns the bag of join and fallback routes admitted to this listener.
func (lt ListenerTree) Routes() RouteBag { return lt.routes }

// AttachedRoutes returns the total number of routes admitted to this listener.
func (lt ListenerTree) AttachedRoutes() int32 {
	return int32(len(lt.routes.Join) + len(lt.routes.Fallback))
}

// EachRoute calls fn for every route (join then fallback) admitted to this listener.
func (lt ListenerTree) EachRoute(fn func(Route)) {
	for _, r := range lt.routes.Join {
		fn(r)
	}
	for _, r := range lt.routes.Fallback {
		fn(r)
	}
}

func (lt ListenerTree) hasRoute(nn types.NamespacedName) bool {
	check := func(routes []Route) bool {
		for _, r := range routes {
			if (types.NamespacedName{Namespace: r.GetNamespace(), Name: r.GetName()}) == nn {
				return true
			}
		}
		return false
	}
	return check(lt.routes.Join) || check(lt.routes.Fallback)
}

// Listener Status Readers

// IsAccepted reports whether this listener's Accepted condition is True.
func (lt ListenerTree) IsAccepted() bool {
	e := lt.statusEntry()
	if e == nil {
		return false
	}
	c := apimeta.FindStatusCondition(e.Conditions, string(gatewayv1.ListenerConditionAccepted))
	return c != nil && c.Status == metav1.ConditionTrue
}

// IsProgrammed reports whether this listener's Programmed condition is True.
func (lt ListenerTree) IsProgrammed() bool {
	e := lt.statusEntry()
	if e == nil {
		return false
	}
	c := apimeta.FindStatusCondition(e.Conditions, string(gatewayv1.ListenerConditionProgrammed))
	return c != nil && c.Status == metav1.ConditionTrue
}

func (lt ListenerTree) statusEntry() *gatewayv1.ListenerStatus {
	if lt.gw == nil {
		return nil
	}
	for i := range lt.gw.Status.Listeners {
		if lt.gw.Status.Listeners[i].Name == lt.Listener.GetName() {
			return &lt.gw.Status.Listeners[i]
		}
	}
	return nil
}

// StatusWriter returns a fluent writer for this listener's conditions.
// Mutations are committed when tree.PatchGatewayStatus is called.
func (lt ListenerTree) StatusWriter() *ListenerStatusWriter {
	return &ListenerStatusWriter{gw: lt.gw, name: lt.Listener.GetName()}
}
