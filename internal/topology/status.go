package topology

import (
	"context"
	"strings"

	apimeta "k8s.io/apimachinery/pkg/api/meta"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	mcgatewayv1alpha1 "minefleet.dev/minecraft-gateway/api/controller/v1alpha1"
	"sigs.k8s.io/controller-runtime/pkg/client"
	gatewayv1 "sigs.k8s.io/gateway-api/apis/v1"
)

// GatewayStatusWriter writes conditions to a GatewayTree's gateway object.
// Mutations accumulate in-memory; call tree.PatchGatewayStatus to commit.
type GatewayStatusWriter struct {
	tree *GatewayTree
}

func (w *GatewayStatusWriter) SetAccepted() *GatewayStatusWriter {
	gw := w.tree.gw
	apimeta.SetStatusCondition(&gw.Status.Conditions, metav1.Condition{
		Type:               string(gatewayv1.GatewayConditionAccepted),
		Status:             metav1.ConditionTrue,
		Reason:             string(gatewayv1.GatewayReasonAccepted),
		Message:            "Gateway accepted by the controller.",
		ObservedGeneration: gw.Generation,
	})
	return w
}

func (w *GatewayStatusWriter) SetAcceptedListenersNotValid() *GatewayStatusWriter {
	gw := w.tree.gw
	apimeta.SetStatusCondition(&gw.Status.Conditions, metav1.Condition{
		Type:               string(gatewayv1.GatewayConditionAccepted),
		Status:             metav1.ConditionTrue,
		Reason:             string(gatewayv1.GatewayReasonListenersNotValid),
		Message:            "Gateway accepted but one or more listeners have an invalid or unsupported configuration.",
		ObservedGeneration: gw.Generation,
	})
	return w
}

func (w *GatewayStatusWriter) SetNotAccepted(reason gatewayv1.GatewayConditionReason, msg string) *GatewayStatusWriter {
	gw := w.tree.gw
	apimeta.SetStatusCondition(&gw.Status.Conditions, metav1.Condition{
		Type:               string(gatewayv1.GatewayConditionAccepted),
		Status:             metav1.ConditionFalse,
		Reason:             string(reason),
		Message:            msg,
		ObservedGeneration: gw.Generation,
	})
	return w
}

func (w *GatewayStatusWriter) SetProgrammed() *GatewayStatusWriter {
	gw := w.tree.gw
	apimeta.SetStatusCondition(&gw.Status.Conditions, metav1.Condition{
		Type:               string(gatewayv1.GatewayConditionProgrammed),
		Status:             metav1.ConditionTrue,
		Reason:             string(gatewayv1.GatewayReasonProgrammed),
		Message:            "Gateway successfully programmed.",
		ObservedGeneration: gw.Generation,
	})
	return w
}

func (w *GatewayStatusWriter) SetNotProgrammed(reason gatewayv1.GatewayConditionReason, msg string) *GatewayStatusWriter {
	gw := w.tree.gw
	apimeta.SetStatusCondition(&gw.Status.Conditions, metav1.Condition{
		Type:               string(gatewayv1.GatewayConditionProgrammed),
		Status:             metav1.ConditionFalse,
		Reason:             string(reason),
		Message:            msg,
		ObservedGeneration: gw.Generation,
	})
	return w
}

func (w *GatewayStatusWriter) SetAddresses(addrs []gatewayv1.GatewayStatusAddress) *GatewayStatusWriter {
	w.tree.gw.Status.Addresses = addrs
	return w
}

// ListenerStatusWriter writes conditions to one listener entry in the gateway
// status. Mutations are committed by tree.PatchGatewayStatus.
type ListenerStatusWriter struct {
	gw   *gatewayv1.Gateway
	name gatewayv1.SectionName
}

func (w *ListenerStatusWriter) entry() *gatewayv1.ListenerStatus {
	for i := range w.gw.Status.Listeners {
		if w.gw.Status.Listeners[i].Name == w.name {
			return &w.gw.Status.Listeners[i]
		}
	}
	w.gw.Status.Listeners = append(w.gw.Status.Listeners, gatewayv1.ListenerStatus{Name: w.name})
	return &w.gw.Status.Listeners[len(w.gw.Status.Listeners)-1]
}

func (w *ListenerStatusWriter) SetAccepted() *ListenerStatusWriter {
	apimeta.SetStatusCondition(&w.entry().Conditions, metav1.Condition{
		Type:               string(gatewayv1.ListenerConditionAccepted),
		Status:             metav1.ConditionTrue,
		Reason:             string(gatewayv1.ListenerReasonAccepted),
		Message:            "Listener accepted.",
		ObservedGeneration: w.gw.Generation,
	})
	return w
}

func (w *ListenerStatusWriter) SetNotAccepted(reason gatewayv1.ListenerConditionReason, msg string) *ListenerStatusWriter {
	apimeta.SetStatusCondition(&w.entry().Conditions, metav1.Condition{
		Type:               string(gatewayv1.ListenerConditionAccepted),
		Status:             metav1.ConditionFalse,
		Reason:             string(reason),
		Message:            msg,
		ObservedGeneration: w.gw.Generation,
	})
	return w
}

func (w *ListenerStatusWriter) SetProgrammed() *ListenerStatusWriter {
	apimeta.SetStatusCondition(&w.entry().Conditions, metav1.Condition{
		Type:               string(gatewayv1.ListenerConditionProgrammed),
		Status:             metav1.ConditionTrue,
		Reason:             string(gatewayv1.ListenerReasonProgrammed),
		Message:            "Listener programmed.",
		ObservedGeneration: w.gw.Generation,
	})
	return w
}

func (w *ListenerStatusWriter) SetNotProgrammed(reason gatewayv1.ListenerConditionReason, msg string) *ListenerStatusWriter {
	apimeta.SetStatusCondition(&w.entry().Conditions, metav1.Condition{
		Type:               string(gatewayv1.ListenerConditionProgrammed),
		Status:             metav1.ConditionFalse,
		Reason:             string(reason),
		Message:            msg,
		ObservedGeneration: w.gw.Generation,
	})
	return w
}

func (w *ListenerStatusWriter) SetResolvedRefs() *ListenerStatusWriter {
	apimeta.SetStatusCondition(&w.entry().Conditions, metav1.Condition{
		Type:               string(gatewayv1.ListenerConditionResolvedRefs),
		Status:             metav1.ConditionTrue,
		Reason:             string(gatewayv1.ListenerReasonResolvedRefs),
		Message:            "Listener references resolved.",
		ObservedGeneration: w.gw.Generation,
	})
	return w
}

func (w *ListenerStatusWriter) SetResolvedRefsInvalid(reason gatewayv1.ListenerConditionReason, msg string) *ListenerStatusWriter {
	apimeta.SetStatusCondition(&w.entry().Conditions, metav1.Condition{
		Type:               string(gatewayv1.ListenerConditionResolvedRefs),
		Status:             metav1.ConditionFalse,
		Reason:             string(reason),
		Message:            msg,
		ObservedGeneration: w.gw.Generation,
	})
	return w
}

func (w *ListenerStatusWriter) SetAttachedRoutes(n int32) *ListenerStatusWriter {
	w.entry().AttachedRoutes = n
	return w
}

func (w *ListenerStatusWriter) SetSupportedKinds(kinds []gatewayv1.RouteGroupKind) *ListenerStatusWriter {
	w.entry().SupportedKinds = kinds
	return w
}

func (w *ListenerStatusWriter) SetNoConflicts() *ListenerStatusWriter {
	apimeta.SetStatusCondition(&w.entry().Conditions, metav1.Condition{
		Type:               string(gatewayv1.ListenerConditionConflicted),
		Status:             metav1.ConditionFalse,
		Reason:             string(gatewayv1.ListenerReasonNoConflicts),
		Message:            "No listener conflicts detected.",
		ObservedGeneration: w.gw.Generation,
	})
	return w
}

func (w *ListenerStatusWriter) SetConflicted(reason gatewayv1.ListenerConditionReason, msg string) *ListenerStatusWriter {
	apimeta.SetStatusCondition(&w.entry().Conditions, metav1.Condition{
		Type:               string(gatewayv1.ListenerConditionConflicted),
		Status:             metav1.ConditionTrue,
		Reason:             string(reason),
		Message:            msg,
		ObservedGeneration: w.gw.Generation,
	})
	return w
}

// RouteStatusWriter accumulates Gateway API conditions for a route across
// multiple parent gateways and patches them in one call to Patch.
// Obtain one via NewRouteStatusWriter.
type RouteStatusWriter struct {
	route Route
	base  client.Object // deep copy taken at construction
}

// NewRouteStatusWriter returns a writer for r. It takes a deep copy immediately
// so the base for patching is stable regardless of subsequent mutations.
func NewRouteStatusWriter(r Route) *RouteStatusWriter {
	obj := r.Object()
	return &RouteStatusWriter{
		route: r,
		base:  obj.DeepCopyObject().(client.Object),
	}
}

// SetNoMatchingParent writes Accepted=False/NoMatchingParent for gwNN.
func (w *RouteStatusWriter) SetNoMatchingParent(gwNN types.NamespacedName) *RouteStatusWriter {
	w.upsert(gwNN, acceptedCond(false, gatewayv1.RouteReasonNoMatchingParent, "Parent gateway not found."))
	return w
}

// SetAccepted writes Accepted for the gateway controller path: one listener + conflict flag.
func (w *RouteStatusWriter) SetAccepted(gwNN types.NamespacedName, listener Listener, conflict bool) *RouteStatusWriter {
	w.upsert(gwNN, w.evalAcceptedFromListener(listener, conflict))
	return w
}

// SetAcceptedFromListeners writes Accepted for the route reconciler path: all admitted listeners.
// Accepted=True if any listener's hostname is compatible with the route's hostnames.
func (w *RouteStatusWriter) SetAcceptedFromListeners(gwNN types.NamespacedName, listeners []ListenerTree) *RouteStatusWriter {
	w.upsert(gwNN, w.evalAcceptedFromListeners(listeners))
	return w
}

// SetResolvedRefs writes the ResolvedRefs condition for gwNN.
func (w *RouteStatusWriter) SetResolvedRefs(gwNN types.NamespacedName, ok bool, reason gatewayv1.RouteConditionReason, msg string) *RouteStatusWriter {
	w.upsert(gwNN, resolvedRefsCond(ok, reason, msg))
	return w
}

// Patch commits all accumulated conditions to the API server in a single call.
func (w *RouteStatusWriter) Patch(ctx context.Context, c client.Client) error {
	return client.IgnoreNotFound(c.Status().Patch(ctx, w.route.Object(), client.MergeFrom(w.base)))
}

func (w *RouteStatusWriter) upsert(gwNN types.NamespacedName, conds ...metav1.Condition) {
	status := w.route.RouteStatus()
	gen := w.route.GetGeneration()
	for _, ref := range w.route.ParentRefs() {
		if !refMatchesGateway(ref, w.route.GetNamespace(), gwNN) {
			continue
		}
		status.Parents = upsertRouteParentEntry(status.Parents, ref, w.route.GetNamespace(), gen, conds...)
	}
}

func (w *RouteStatusWriter) evalAcceptedFromListener(listener Listener, conflict bool) metav1.Condition {
	if conflict {
		return acceptedCond(false, mcgatewayv1alpha1.RouteReasonRouteConflict,
			"Route conflicts with another route attached to the same gateway.")
	}
	lh := listener.GetHostname()
	if lh != nil && *lh != "" {
		hostnames := w.route.Hostnames()
		if hostnames != nil && !anyHostnameCompatible(hostnames, *lh) {
			return acceptedCond(false, gatewayv1.RouteReasonNoMatchingListenerHostname,
				"No route hostname is compatible with the listener hostname.")
		}
	}
	return acceptedCond(true, gatewayv1.RouteReasonAccepted, "Route accepted.")
}

func (w *RouteStatusWriter) evalAcceptedFromListeners(listeners []ListenerTree) metav1.Condition {
	if len(listeners) == 0 {
		return acceptedCond(false, gatewayv1.RouteReasonNotAllowedByListeners,
			"Route is not allowed by any listener namespace or kind filter.")
	}
	hostnames := w.route.Hostnames()
	for _, lt := range listeners {
		lh := lt.Listener.GetHostname()
		if lh == nil || *lh == "" || hostnames == nil || anyHostnameCompatible(hostnames, *lh) {
			return acceptedCond(true, gatewayv1.RouteReasonAccepted, "Route accepted.")
		}
	}
	return acceptedCond(false, gatewayv1.RouteReasonNoMatchingListenerHostname,
		"No route hostname is compatible with any listener hostname.")
}

func anyHostnameCompatible(routeHostnames []gatewayv1.Hostname, listenerHostname gatewayv1.Hostname) bool {
	for _, rh := range routeHostnames {
		if hostnamesCompatible(string(listenerHostname), string(rh)) {
			return true
		}
	}
	return false
}

func hostnamesCompatible(listener, routeHost string) bool {
	lWild := strings.HasPrefix(listener, "*.")
	rWild := strings.HasPrefix(routeHost, "*.")
	switch {
	case !lWild && !rWild:
		return listener == routeHost
	case !lWild:
		return dotSuffix(listener, routeHost[2:])
	case !rWild:
		return dotSuffix(routeHost, listener[2:])
	default:
		return listener == routeHost || dotSuffix(routeHost[2:], listener[2:])
	}
}

func dotSuffix(s, suffix string) bool {
	return len(s) > len(suffix) &&
		s[len(s)-len(suffix):] == suffix &&
		s[len(s)-len(suffix)-1] == '.'
}

// Condition helpers

func acceptedCond(ok bool, reason gatewayv1.RouteConditionReason, msg string) metav1.Condition {
	s := metav1.ConditionTrue
	if !ok {
		s = metav1.ConditionFalse
	}
	return metav1.Condition{
		Type:    string(gatewayv1.RouteConditionAccepted),
		Status:  s,
		Reason:  string(reason),
		Message: msg,
	}
}

func resolvedRefsCond(ok bool, reason gatewayv1.RouteConditionReason, msg string) metav1.Condition {
	s := metav1.ConditionTrue
	if !ok {
		s = metav1.ConditionFalse
	}
	return metav1.Condition{
		Type:    string(gatewayv1.RouteConditionResolvedRefs),
		Status:  s,
		Reason:  string(reason),
		Message: msg,
	}
}

// Parent status helpers

func upsertRouteParentEntry(
	parents []gatewayv1.RouteParentStatus,
	specRef gatewayv1.ParentReference,
	routeNS string,
	generation int64,
	conds ...metav1.Condition,
) []gatewayv1.RouteParentStatus {
	setAll := func(conditions *[]metav1.Condition) {
		for _, c := range conds {
			c.ObservedGeneration = generation
			apimeta.SetStatusCondition(conditions, c)
		}
	}

	controller := gatewayv1.GatewayController(ControllerName)
	canonicalNN := types.NamespacedName{
		Namespace: resolveNS(specRef, routeNS),
		Name:      string(specRef.Name),
	}
	for i, p := range parents {
		if p.ControllerName != controller {
			continue
		}
		if !refMatchesGateway(p.ParentRef, routeNS, canonicalNN) {
			continue
		}
		setAll(&parents[i].Conditions)
		return parents
	}

	ns := gatewayv1.Namespace(resolveNS(specRef, routeNS))
	entry := gatewayv1.RouteParentStatus{
		ParentRef:      gatewayv1.ParentReference{Namespace: &ns, Name: specRef.Name},
		ControllerName: controller,
	}
	setAll(&entry.Conditions)
	return append(parents, entry)
}

func refMatchesGateway(ref gatewayv1.ParentReference, routeNS string, gwNN types.NamespacedName) bool {
	g := ""
	if ref.Group != nil {
		g = string(*ref.Group)
	}
	k := ""
	if ref.Kind != nil {
		k = string(*ref.Kind)
	}
	if (g != "" && g != gatewayv1.GroupName) || (k != "" && k != "Gateway") {
		return false
	}
	if string(ref.Name) != gwNN.Name {
		return false
	}
	return resolveNS(ref, routeNS) == gwNN.Namespace
}

func resolveNS(ref gatewayv1.ParentReference, routeNS string) string {
	if ref.Namespace != nil && *ref.Namespace != "" {
		return string(*ref.Namespace)
	}
	return routeNS
}
