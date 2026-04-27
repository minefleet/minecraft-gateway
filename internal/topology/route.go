package topology

import (
	"strings"

	mcgatewayv1alpha1 "minefleet.dev/minecraft-gateway/api/controller/v1alpha1"
	"sigs.k8s.io/controller-runtime/pkg/client"
	gatewayv1 "sigs.k8s.io/gateway-api/apis/v1"
)

type RouteType string

const (
	RouteTypeJoin     RouteType = "join"
	RouteTypeFallback RouteType = "fallback"
)

var (
	minefleetGroup = gatewayv1.Group("gateway.networking.minefleet.dev")

	// SupportedRouteKinds lists every route kind this controller can handle.
	SupportedRouteKinds = []gatewayv1.RouteGroupKind{
		{Group: &minefleetGroup, Kind: "MinecraftJoinRoute"},
		{Group: &minefleetGroup, Kind: "MinecraftFallbackRoute"},
	}
)

// routeKindName maps a Route's type to its Kubernetes Kind name.
func routeKindName(r Route) gatewayv1.Kind {
	switch r.RouteType() {
	case RouteTypeJoin:
		return "MinecraftJoinRoute"
	case RouteTypeFallback:
		return "MinecraftFallbackRoute"
	default:
		return ""
	}
}

// Route is the unified interface for all Minecraft route types.
// Embedding client.Object provides all Kubernetes metadata methods (GetName,
// GetNamespace, GetGeneration, DeepCopyObject, etc.) without type assertions.
type Route interface {
	client.Object

	// RouteType distinguishes join from fallback routes.
	RouteType() RouteType

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

type joinRoute struct {
	*mcgatewayv1alpha1.MinecraftJoinRoute
}

// ForJoinRoute wraps a MinecraftJoinRoute as a Route.
func ForJoinRoute(r *mcgatewayv1alpha1.MinecraftJoinRoute) Route {
	return &joinRoute{r}
}

func (r *joinRoute) RouteType() RouteType                                 { return RouteTypeJoin }
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

type fallbackRoute struct {
	*mcgatewayv1alpha1.MinecraftFallbackRoute
}

// ForFallbackRoute wraps a MinecraftFallbackRoute as a Route.
func ForFallbackRoute(r *mcgatewayv1alpha1.MinecraftFallbackRoute) Route {
	return &fallbackRoute{r}
}

func (r *fallbackRoute) RouteType() RouteType                    { return RouteTypeFallback }
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

// RouteBag holds the join and fallback routes admitted to one listener.
type RouteBag struct {
	Join     []Route
	Fallback []Route
}

// ForRouteBag wraps raw list results into a RouteBag.
func ForRouteBag(joinList mcgatewayv1alpha1.MinecraftJoinRouteList, fallbackList mcgatewayv1alpha1.MinecraftFallbackRouteList) RouteBag {
	join := make([]Route, len(joinList.Items))
	for i := range joinList.Items {
		join[i] = ForJoinRoute(&joinList.Items[i])
	}
	fallback := make([]Route, len(fallbackList.Items))
	for i := range fallbackList.Items {
		fallback[i] = ForFallbackRoute(&fallbackList.Items[i])
	}
	return RouteBag{Join: join, Fallback: fallback}
}

// Dedupe removes duplicate routes (same namespace/name/kind) keeping the first occurrence.
func (b RouteBag) Dedupe() RouteBag {
	seen := map[string]struct{}{}
	out := RouteBag{
		Join:     make([]Route, 0, len(b.Join)),
		Fallback: make([]Route, 0, len(b.Fallback)),
	}
	key := func(ns, name, kind string) string {
		return strings.Join([]string{kind, ns, name}, "/")
	}
	for _, r := range b.Join {
		k := key(r.GetNamespace(), r.GetName(), "Join")
		if _, ok := seen[k]; !ok {
			seen[k] = struct{}{}
			out.Join = append(out.Join, r)
		}
	}
	for _, r := range b.Fallback {
		k := key(r.GetNamespace(), r.GetName(), "Fallback")
		if _, ok := seen[k]; !ok {
			seen[k] = struct{}{}
			out.Fallback = append(out.Fallback, r)
		}
	}
	return out
}
