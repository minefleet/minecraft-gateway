package topology

import (
	"context"

	discoveryv1 "k8s.io/api/discovery/v1"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/utils/ptr"
	mcgatewayv1alpha1 "minefleet.dev/minecraft-gateway/api/controller/v1alpha1"
	"minefleet.dev/minecraft-gateway/internal/endpoint"
	"minefleet.dev/minecraft-gateway/internal/gateway"
	"minefleet.dev/minecraft-gateway/internal/index"
	"sigs.k8s.io/controller-runtime/pkg/client"
	gatewayv1 "sigs.k8s.io/gateway-api/apis/v1"
)

// Build constructs a complete GatewayTree for gw: resolves the GatewayClass,
// Infrastructure, EndpointSlice backends, and per-listener route bags.
// Returns an error if the GatewayClass cannot be fetched; infrastructure and
// backend failures are silently ignored (best-effort).
func Build(ctx context.Context, c client.Client, gw *gatewayv1.Gateway) (*GatewayTree, error) {
	base := gw.DeepCopy()

	gwClass, err := gateway.GetGatewayClassByGateway(c, ctx, *gw)
	if err != nil {
		return nil, err
	}

	infra, _ := gateway.GetInfrastructureForGateway(c, ctx, *gw)
	backends := collectBackends(ctx, c, infra)

	bag, err := listRoutesForGateway(ctx, c, gw)
	if err != nil {
		return nil, err
	}

	sourced := ListenersFromGateway(gw)
	listenerTrees := make([]ListenerTree, 0, len(sourced.Listeners))
	for _, l := range sourced.Listeners {
		lt := ListenerTree{Listener: l, gw: gw}
		for _, r := range bag.Join {
			if l.CheckRouteAllowed(r, c, ctx) == nil {
				lt.routes.Join = append(lt.routes.Join, r)
			}
		}
		for _, r := range bag.Fallback {
			if l.CheckRouteAllowed(r, c, ctx) == nil {
				lt.routes.Fallback = append(lt.routes.Fallback, r)
			}
		}
		lt.routes = lt.routes.Dedupe()
		listenerTrees = append(listenerTrees, lt)
	}

	return &GatewayTree{
		gw:             gw,
		base:           base,
		Class:          GatewayClass{GatewayClass: gwClass},
		Infrastructure: infra,
		Backends:       backends,
		listeners:      listenerTrees,
	}, nil
}

// BuildForRoute resolves the Gateway referenced by ref and returns its full tree.
// Returns (nil, zero, nil) when ref does not target a Gateway kind.
// Returns (nil, gwNN, nil) when the gateway does not exist.
func BuildForRoute(ctx context.Context, c client.Client, ref gatewayv1.ParentReference, routeNS string) (*GatewayTree, types.NamespacedName, error) {
	g := ""
	if ref.Group != nil {
		g = string(*ref.Group)
	}
	k := ""
	if ref.Kind != nil {
		k = string(*ref.Kind)
	}
	if (g != "" && g != gatewayv1.GroupName) || (k != "" && k != "Gateway") {
		return nil, types.NamespacedName{}, nil
	}

	ns := routeNS
	if ref.Namespace != nil && *ref.Namespace != "" {
		ns = string(*ref.Namespace)
	}
	gwNN := types.NamespacedName{Namespace: ns, Name: string(ref.Name)}

	var gw gatewayv1.Gateway
	if err := c.Get(ctx, gwNN, &gw); err != nil {
		if client.IgnoreNotFound(err) == nil {
			return nil, gwNN, nil
		}
		return nil, gwNN, err
	}

	tree, err := Build(ctx, c, &gw)
	return tree, gwNN, err
}

func collectBackends(ctx context.Context, c client.Client, infra gateway.Infrastructure) []discoveryv1.EndpointSlice {
	var out []discoveryv1.EndpointSlice
	for _, ref := range infra.Status.BackendRefs {
		slices, err := endpoint.GetEndpointSlicesByServiceName(c, ctx, string(ptr.Deref(ref.Namespace, "")), string(ref.Name))
		if err != nil {
			continue
		}
		out = append(out, slices...)
	}
	return out
}

func listRoutesForGateway(ctx context.Context, c client.Client, gw *gatewayv1.Gateway) (RouteBag, error) {
	key := gw.Namespace + "/" + gw.Name
	var joinList mcgatewayv1alpha1.MinecraftJoinRouteList
	if err := c.List(ctx, &joinList, client.MatchingFields{index.RouteByGateway: key}); err != nil {
		return RouteBag{}, err
	}
	var fallbackList mcgatewayv1alpha1.MinecraftFallbackRouteList
	if err := c.List(ctx, &fallbackList, client.MatchingFields{index.RouteByGateway: key}); err != nil {
		return RouteBag{}, err
	}
	return ForRouteBag(joinList, fallbackList), nil
}
