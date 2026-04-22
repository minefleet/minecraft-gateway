package status

import (
	apimeta "k8s.io/apimachinery/pkg/api/meta"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	mcgatewayv1alpha1 "minefleet.dev/minecraft-gateway/api/controller/v1alpha1"
	"minefleet.dev/minecraft-gateway/internal/gateway"
	gatewayv1 "sigs.k8s.io/gateway-api/apis/v1"
)

// Gateway reconciler — sets Accepted condition.

func SetJoinRouteAccepted(route *mcgatewayv1alpha1.MinecraftJoinRoute, gwNN types.NamespacedName) {
	route.Status.Parents = upsertRouteParentStatuses(
		route.Status.Parents, route.Spec.ParentRefs, route.Namespace, gwNN, route.Generation,
		acceptedCond(true, gatewayv1.RouteReasonAccepted, "Route accepted."),
	)
}

func SetJoinRouteConflict(route *mcgatewayv1alpha1.MinecraftJoinRoute, gwNN types.NamespacedName) {
	route.Status.Parents = upsertRouteParentStatuses(
		route.Status.Parents, route.Spec.ParentRefs, route.Namespace, gwNN, route.Generation,
		acceptedCond(false, mcgatewayv1alpha1.RouteReasonRouteConflict,
			"Route conflicts with another route attached to the same gateway."),
	)
}

func SetFallbackRouteAccepted(route *mcgatewayv1alpha1.MinecraftFallbackRoute, gwNN types.NamespacedName) {
	route.Status.Parents = upsertRouteParentStatuses(
		route.Status.Parents, route.Spec.ParentRefs, route.Namespace, gwNN, route.Generation,
		acceptedCond(true, gatewayv1.RouteReasonAccepted, "Route accepted."),
	)
}

func SetFallbackRouteConflict(route *mcgatewayv1alpha1.MinecraftFallbackRoute, gwNN types.NamespacedName) {
	route.Status.Parents = upsertRouteParentStatuses(
		route.Status.Parents, route.Spec.ParentRefs, route.Namespace, gwNN, route.Generation,
		acceptedCond(false, mcgatewayv1alpha1.RouteReasonRouteConflict,
			"Route conflicts with another route attached to the same gateway."),
	)
}

// Route reconcilers — set Accepted=False/NoMatchingParent and ResolvedRefs.

func SetJoinRouteNoMatchingParent(route *mcgatewayv1alpha1.MinecraftJoinRoute, gwNN types.NamespacedName) {
	route.Status.Parents = upsertRouteParentStatuses(
		route.Status.Parents, route.Spec.ParentRefs, route.Namespace, gwNN, route.Generation,
		acceptedCond(false, gatewayv1.RouteReasonNoMatchingParent, "Parent gateway not found."),
	)
}

func SetJoinRouteResolvedRefs(route *mcgatewayv1alpha1.MinecraftJoinRoute, gwNN types.NamespacedName, ok bool, reason gatewayv1.RouteConditionReason, msg string) {
	route.Status.Parents = upsertRouteParentStatuses(
		route.Status.Parents, route.Spec.ParentRefs, route.Namespace, gwNN, route.Generation,
		resolvedRefsCond(ok, reason, msg),
	)
}

func SetFallbackRouteNoMatchingParent(route *mcgatewayv1alpha1.MinecraftFallbackRoute, gwNN types.NamespacedName) {
	route.Status.Parents = upsertRouteParentStatuses(
		route.Status.Parents, route.Spec.ParentRefs, route.Namespace, gwNN, route.Generation,
		acceptedCond(false, gatewayv1.RouteReasonNoMatchingParent, "Parent gateway not found."),
	)
}

func SetFallbackRouteResolvedRefs(route *mcgatewayv1alpha1.MinecraftFallbackRoute, gwNN types.NamespacedName, ok bool, reason gatewayv1.RouteConditionReason, msg string) {
	route.Status.Parents = upsertRouteParentStatuses(
		route.Status.Parents, route.Spec.ParentRefs, route.Namespace, gwNN, route.Generation,
		resolvedRefsCond(ok, reason, msg),
	)
}

// --- helpers ----------------------------------------------------------------

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

// upsertRouteParentStatuses iterates the route's spec parentRefs, finds refs
// that point at gwNN, and upserts the corresponding RouteParentStatus entry
// with the given conditions.
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
