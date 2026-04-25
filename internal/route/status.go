package route

import (
	"strings"

	apimeta "k8s.io/apimachinery/pkg/api/meta"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	mcgatewayv1alpha1 "minefleet.dev/minecraft-gateway/api/controller/v1alpha1"
	"minefleet.dev/minecraft-gateway/internal/gateway"
	gatewayv1 "sigs.k8s.io/gateway-api/apis/v1"
)

// --- ParentContext -----------------------------------------------------------

// ParentContext describes the outcome of evaluating one route parentRef.
// Construct it with one of the three factory functions below.
type ParentContext struct {
	gatewayFound  bool
	listener      *gatewayv1.Listener  // set by ParentAttached (gateway controller)
	listeners     []gatewayv1.Listener // set by ParentResolved (route reconciler)
	conflict      bool
	backendOK     bool
	backendReason gatewayv1.RouteConditionReason
	backendMsg    string
}

// ParentNotFound returns a context for when the parent gateway resource does
// not exist. The evaluator will write Accepted=False/NoMatchingParent.
func ParentNotFound() ParentContext {
	return ParentContext{}
}

// ParentResolved returns a context for when the parent gateway is present and
// managed by this controller. The evaluator writes ResolvedRefs and, when
// listeners is non-empty, Accepted based on listener hostname compatibility.
// ok, reason, and msg are the direct output of CheckBackendRefs.
func ParentResolved(listeners []gatewayv1.Listener, ok bool, reason gatewayv1.RouteConditionReason, msg string) ParentContext {
	return ParentContext{
		gatewayFound:  true,
		listeners:     listeners,
		backendOK:     ok,
		backendReason: reason,
		backendMsg:    msg,
	}
}

// ParentAttached returns a context for a route that has been matched to a
// listener in the routing table. The evaluator writes the Accepted condition,
// checking listener hostname compatibility and the conflict flag.
func ParentAttached(listener gatewayv1.Listener, conflict bool) ParentContext {
	return ParentContext{
		gatewayFound: true,
		listener:     &listener,
		conflict:     conflict,
	}
}

// --- Status interface --------------------------------------------------------

// Status evaluates and applies Gateway API conditions for one route-parent
// relationship. Obtain an instance via StatusFor.
type Status interface {
	// EvaluateConditions returns the conditions this context implies,
	// without modifying the route. Useful for testing.
	EvaluateConditions() []metav1.Condition
	// Apply writes the evaluated conditions to the route's RouteStatus for
	// the given parent gateway.
	Apply(gwNN types.NamespacedName)
}

// StatusFor returns a Status evaluator for route r in the given ParentContext.
func StatusFor(r Route, ctx ParentContext) Status {
	return &routeStatus{route: r, ctx: ctx}
}

type routeStatus struct {
	route Route
	ctx   ParentContext
}

func (s *routeStatus) EvaluateConditions() []metav1.Condition {
	if !s.ctx.gatewayFound {
		return []metav1.Condition{
			acceptedCond(false, gatewayv1.RouteReasonNoMatchingParent, "Parent gateway not found."),
		}
	}
	if s.ctx.listener != nil {
		// Gateway controller path: Accepted based on one listener + conflict flag.
		return []metav1.Condition{s.acceptedCondition()}
	}
	// Route reconciler path: ResolvedRefs + Accepted via listener hostname check.
	conds := []metav1.Condition{
		resolvedRefsCond(s.ctx.backendOK, s.ctx.backendReason, s.ctx.backendMsg),
	}
	if len(s.ctx.listeners) > 0 {
		conds = append(conds, s.acceptedFromListeners())
	}
	return conds
}

func (s *routeStatus) Apply(gwNN types.NamespacedName) {
	conds := s.EvaluateConditions()
	s.route.RouteStatus().Parents = upsertRouteParentStatuses(
		s.route.RouteStatus().Parents, s.route.ParentRefs(), s.route.GetNamespace(),
		gwNN, s.route.GetGeneration(), conds...,
	)
}

// acceptedCondition evaluates Accepted for the gateway controller path,
// where a single listener and the conflict flag are known.
func (s *routeStatus) acceptedCondition() metav1.Condition {
	if s.ctx.conflict {
		return acceptedCond(false, mcgatewayv1alpha1.RouteReasonRouteConflict,
			"Route conflicts with another route attached to the same gateway.")
	}
	lh := s.ctx.listener.Hostname
	if lh != nil && *lh != "" {
		hostnames := s.route.Hostnames()
		// nil means the route matches any hostname (fallback routes).
		// non-nil empty means no hostnames declared — reject.
		if hostnames != nil && !anyHostnameCompatible(hostnames, *lh) {
			return acceptedCond(false, gatewayv1.RouteReasonNoMatchingListenerHostname,
				"No route hostname is compatible with the listener hostname.")
		}
	}
	return acceptedCond(true, gatewayv1.RouteReasonAccepted, "Route accepted.")
}

// acceptedFromListeners evaluates Accepted for the route reconciler path,
// where all candidate listeners are known but conflict state is not yet determined.
// Accepted=True if any listener's hostname is compatible with the route's hostnames.
func (s *routeStatus) acceptedFromListeners() metav1.Condition {
	hostnames := s.route.Hostnames()
	for _, l := range s.ctx.listeners {
		lh := l.Hostname
		if lh == nil || *lh == "" || hostnames == nil || anyHostnameCompatible(hostnames, *lh) {
			return acceptedCond(true, gatewayv1.RouteReasonAccepted, "Route accepted.")
		}
	}
	return acceptedCond(false, gatewayv1.RouteReasonNoMatchingListenerHostname,
		"No route hostname is compatible with any listener hostname.")
}

// --- Hostname compatibility --------------------------------------------------

func anyHostnameCompatible(routeHostnames []gatewayv1.Hostname, listenerHostname gatewayv1.Hostname) bool {
	for _, rh := range routeHostnames {
		if hostnamesCompatible(string(listenerHostname), string(rh)) {
			return true
		}
	}
	return false
}

// hostnamesCompatible checks whether a listener hostname and a route hostname
// are compatible under Gateway API intersection semantics.
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

// --- Condition constructors -------------------------------------------------

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

// --- Parent status upsert ---------------------------------------------------

func upsertRouteParentStatuses(
	parents []gatewayv1.RouteParentStatus,
	specRefs []gatewayv1.ParentReference,
	routeNS string,
	gwNN types.NamespacedName,
	generation int64,
	conds ...metav1.Condition,
) []gatewayv1.RouteParentStatus {
	for _, ref := range specRefs {
		if !refMatchesGateway(ref, routeNS, gwNN) {
			continue
		}
		parents = upsertEntry(parents, ref, routeNS, generation, conds...)
	}
	return parents
}

func upsertEntry(
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

	controller := gatewayv1.GatewayController(gateway.ControllerName)
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
