package route

import (
	"context"
	"fmt"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/utils/ptr"
	"log"
	mcgatewayv1 "minefleet.dev/minecraft-gateway/api/v1"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	gatewayv1 "sigs.k8s.io/gateway-api/apis/v1"
)

type Bag struct {
	Join     []mcgatewayv1.MinecraftJoinRoute
	Fallback []mcgatewayv1.MinecraftFallbackRoute
}

const (
	IndexRouteByGateway         = "route.byGateway"
	IndexRouteByGatewayListener = "route.byGatewayListener"
	IndexRouteByService         = "route.byListener"
)

func keySvc(ns, name string) string            { return ns + "/" + name }
func keyGW(ns, name string) string             { return ns + "/" + name }
func keyGWListener(ns, name, ln string) string { return ns + "/" + name + "#" + ln }

func IndexRoutes(mgr ctrl.Manager) error {
	if err := indexRouteParents(mgr, &mcgatewayv1.MinecraftJoinRoute{}); err != nil {
		return err
	}
	if err := indexRouteParents(mgr, &mcgatewayv1.MinecraftFallbackRoute{}); err != nil {
		return err
	}
	return nil
}

func indexRouteParents[T client.Object](mgr ctrl.Manager, zero T) error {
	ctx := context.Background()
	// by gateway (ns/name)
	if err := mgr.GetFieldIndexer().IndexField(ctx, zero, IndexRouteByGateway,
		func(o client.Object) []string {
			return extractGatewayParentKeys(o, false)
		},
	); err != nil {
		return err
	}
	// by gateway+listener (ns/name#listener)
	if err := mgr.GetFieldIndexer().IndexField(ctx, zero, IndexRouteByGatewayListener,
		func(o client.Object) []string {
			return extractGatewayParentKeys(o, true)
		},
	); err != nil {
		return err
	}
	// by svc (ns/name)
	if err := mgr.GetFieldIndexer().IndexField(ctx, zero, IndexRouteByService,
		func(o client.Object) []string {
			return extractServiceKeys(o)
		},
	); err != nil {
		return err
	}
	return nil
}

func extractServiceKeys(o client.Object) []string {
	route, ns, err := extractRouteAndNamespace(o)
	if err != nil {
		return nil
	}
	result := make([]string, 0)
	for _, ref := range route.BackendRefs {
		// Skip if kind is not a service, only service backends are supported
		if ref.Kind != nil && *ref.Kind != "Service" {
			continue
		}
		usedNs := ref.Namespace
		if usedNs == nil {
			usedNs = ptr.To(gatewayv1.Namespace(ns))
		}
		result = append(result, keySvc(string(*usedNs), string(ref.Name)))
	}
	return result
}

func extractGatewayParentKeys(o client.Object, withListener bool) []string {
	route, ns, err := extractRouteAndNamespace(o)
	if err != nil {
		log.Print(err)
		return nil
	}
	return parentKeysFromRefs(ns, route.ParentRefs, withListener)
}

func extractRouteAndNamespace(o client.Object) (mcgatewayv1.MinecraftRoute, string, error) {
	switch r := o.(type) {
	case *mcgatewayv1.MinecraftJoinRoute:
		return r.Spec.MinecraftRoute, o.GetNamespace(), nil
	case *mcgatewayv1.MinecraftFallbackRoute:
		return r.Spec.MinecraftRoute, o.GetNamespace(), nil
	default:
		return mcgatewayv1.MinecraftRoute{}, "", fmt.Errorf("no such minecraft route: %T", r)
	}
}

func parentKeysFromRefs(routeNS string, refs []gatewayv1.ParentReference, withListener bool) []string {
	out := make([]string, 0, len(refs))
	for _, pr := range refs {
		ref := namespacedNameByRef(routeNS, pr)
		if ref == nil {
			continue
		}
		if withListener && ref.Section != "" {
			out = append(out, keyGWListener(ref.Namespace, ref.Name, ref.Section))
		} else {
			out = append(out, keyGW(ref.Namespace, ref.Name))
		}
	}
	return out
}

type namespacedName struct {
	Namespace string
	Name      string
	Section   string
}

func namespacedNameByRef(routeNS string, ref gatewayv1.ParentReference) *namespacedName {
	g := stringPtrTo(ref.Group)
	k := stringPtrToKind(ref.Kind)
	if (g != "" && g != gatewayv1.GroupName) || (k != "" && k != "Gateway") {
		return nil
	}
	if ref.Name == "" {
		return nil
	}
	parentNS := routeNS
	if ref.Namespace != nil && *ref.Namespace != "" {
		parentNS = string(*ref.Namespace)
	}
	if ref.SectionName != nil && *ref.SectionName != "" {
		return &namespacedName{
			Namespace: parentNS,
			Name:      string(ref.Name),
			Section:   string(*ref.SectionName),
		}
	}

	return &namespacedName{
		Namespace: parentNS,
		Name:      string(ref.Name),
	}
}

func NamespacedNamesByRefs(routeNS string, refs []gatewayv1.ParentReference) []namespacedName {
	result := make([]namespacedName, 0)
	for _, pr := range refs {
		ref := namespacedNameByRef(routeNS, pr)
		if ref == nil {
			continue
		}
		result = append(result, *ref)
	}
	return result
}

func stringPtrTo(p *gatewayv1.Group) string {
	if p == nil {
		return ""
	}
	return string(*p)
}

func stringPtrToKind(p *gatewayv1.Kind) string {
	if p == nil {
		return ""
	}
	return string(*p)
}

func ListRoutesByGateway[T client.ObjectList](c client.Client, ctx context.Context, gw gatewayv1.Gateway, zero T) error {
	if err := c.List(ctx, zero, client.MatchingFields{IndexRouteByGateway: keyGW(gw.Namespace, gw.Name)}); err != nil {
		return err
	}
	return nil
}

func ListAllRoutesByGateway(c client.Client, ctx context.Context, gw gatewayv1.Gateway, into *Bag) error {
	var join mcgatewayv1.MinecraftJoinRouteList
	var fallback mcgatewayv1.MinecraftFallbackRouteList
	if err := ListRoutesByGateway(c, ctx, gw, &join); err != nil {
		return err
	}
	if err := ListRoutesByGateway(c, ctx, gw, &fallback); err != nil {
		return err
	}
	*into = Bag{
		Join:     join.Items,
		Fallback: fallback.Items,
	}
	return nil
}

func ListRoutesByService[T client.ObjectList](c client.Client, ctx context.Context, svc corev1.Service, zero T) error {
	if err := c.List(ctx, zero, client.MatchingFields{IndexRouteByService: keySvc(svc.Namespace, svc.Name)}); err != nil {
		return err
	}
	return nil
}

func ListAllRoutesByService(c client.Client, ctx context.Context, svc corev1.Service, into *Bag) error {
	var join mcgatewayv1.MinecraftJoinRouteList
	var fallback mcgatewayv1.MinecraftFallbackRouteList
	if err := ListRoutesByService(c, ctx, svc, &join); err != nil {
		return err
	}
	if err := ListRoutesByService(c, ctx, svc, &fallback); err != nil {
		return err
	}

	return nil
}
