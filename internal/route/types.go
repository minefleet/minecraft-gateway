package route

import (
	mcgatewayv1alpha1 "minefleet.dev/minecraft-gateway/api/controller/v1alpha1"
	"sigs.k8s.io/controller-runtime/pkg/client"
	gatewayv1 "sigs.k8s.io/gateway-api/apis/v1"
)

type Type string

const (
	TypeJoin     Type = "join"
	TypeFallback Type = "fallback"
)

// Route is the unified interface for all Minecraft route types.
// Embedding client.Object provides all Kubernetes metadata methods (GetName,
// GetNamespace, GetGeneration, DeepCopyObject, etc.) without type assertions.
type Route interface {
	client.Object

	// RouteType distinguishes join from fallback routes.
	RouteType() Type

	// Hostnames returns the virtual hostnames this route handles.
	// Returns nil for fallback routes (they match any hostname).
	Hostnames() []gatewayv1.Hostname

	// ParentRefs returns the gateway parent references.
	ParentRefs() []gatewayv1.ParentReference

	// BackendRefs returns the backend service references.
	BackendRefs() []mcgatewayv1alpha1.MinecraftBackendRef

	// Priority returns the route priority for conflict resolution.
	Priority() int

	// JoinFilterRules returns the join-specific filter rules; nil for fallback routes.
	JoinFilterRules() []mcgatewayv1alpha1.MinecraftJoinFilterRuleSet

	// FallbackFilterRules returns the fallback-specific filter rules; nil for join routes.
	FallbackFilterRules() []mcgatewayv1alpha1.MinecraftFallbackFilterRuleSet

	// RouteStatus returns a pointer to the underlying RouteStatus so status
	// helpers can read and write parent conditions without type assertions.
	RouteStatus() *gatewayv1.RouteStatus

	// Object returns the underlying registered Kubernetes object. Use this
	// when passing to controller-runtime APIs (e.g. Status().Patch) that
	// look up the GVK via the scheme — the wrapper types are not registered.
	Object() client.Object
}

// joinRoute wraps MinecraftJoinRoute. Embedding the pointer satisfies client.Object
// automatically; explicit methods implement the Route-specific surface.
type joinRoute struct {
	*mcgatewayv1alpha1.MinecraftJoinRoute
}

// ForJoin wraps a MinecraftJoinRoute as a Route.
func ForJoin(r *mcgatewayv1alpha1.MinecraftJoinRoute) Route {
	return &joinRoute{r}
}

func (r *joinRoute) RouteType() Type                                      { return TypeJoin }
func (r *joinRoute) Hostnames() []gatewayv1.Hostname                      { return r.Spec.Hostnames }
func (r *joinRoute) ParentRefs() []gatewayv1.ParentReference              { return r.Spec.ParentRefs }
func (r *joinRoute) BackendRefs() []mcgatewayv1alpha1.MinecraftBackendRef { return r.Spec.BackendRefs }
func (r *joinRoute) Priority() int                                        { return r.Spec.Priority }
func (r *joinRoute) JoinFilterRules() []mcgatewayv1alpha1.MinecraftJoinFilterRuleSet {
	return r.Spec.FilterRules
}
func (r *joinRoute) FallbackFilterRules() []mcgatewayv1alpha1.MinecraftFallbackFilterRuleSet {
	return nil
}
func (r *joinRoute) RouteStatus() *gatewayv1.RouteStatus { return &r.Status.RouteStatus }
func (r *joinRoute) Object() client.Object               { return r.MinecraftJoinRoute }

// fallbackRoute wraps MinecraftFallbackRoute.
type fallbackRoute struct {
	*mcgatewayv1alpha1.MinecraftFallbackRoute
}

// ForFallback wraps a MinecraftFallbackRoute as a Route.
func ForFallback(r *mcgatewayv1alpha1.MinecraftFallbackRoute) Route {
	return &fallbackRoute{r}
}

func (r *fallbackRoute) RouteType() Type                         { return TypeFallback }
func (r *fallbackRoute) Hostnames() []gatewayv1.Hostname         { return nil }
func (r *fallbackRoute) ParentRefs() []gatewayv1.ParentReference { return r.Spec.ParentRefs }
func (r *fallbackRoute) BackendRefs() []mcgatewayv1alpha1.MinecraftBackendRef {
	return r.Spec.BackendRefs
}
func (r *fallbackRoute) Priority() int { return r.Spec.Priority }
func (r *fallbackRoute) JoinFilterRules() []mcgatewayv1alpha1.MinecraftJoinFilterRuleSet {
	return nil
}
func (r *fallbackRoute) FallbackFilterRules() []mcgatewayv1alpha1.MinecraftFallbackFilterRuleSet {
	return r.Spec.FilterRules
}
func (r *fallbackRoute) RouteStatus() *gatewayv1.RouteStatus { return &r.Status.RouteStatus }
func (r *fallbackRoute) Object() client.Object               { return r.MinecraftFallbackRoute }
