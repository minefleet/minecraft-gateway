package route

import (
	"context"
	"fmt"
	"log"

	corev1 "k8s.io/api/core/v1"
	apimeta "k8s.io/apimachinery/pkg/api/meta"
	"k8s.io/utils/ptr"
	mcgatewayv1alpha1 "minefleet.dev/minecraft-gateway/api/controller/v1alpha1"
	"minefleet.dev/minecraft-gateway/internal/index"
	"minefleet.dev/minecraft-gateway/internal/topology"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	gatewayv1 "sigs.k8s.io/gateway-api/apis/v1"
)

func keySvc(ns, name string) string            { return ns + "/" + name }
func keyGW(ns, name string) string             { return ns + "/" + name }
func keyGWListener(ns, name, ln string) string { return ns + "/" + name + "#" + ln }

func IndexRoutes(mgr ctrl.Manager) error {
	if err := indexRouteParents(mgr, &mcgatewayv1alpha1.MinecraftJoinRoute{}); err != nil {
		return err
	}
	if err := indexRouteParents(mgr, &mcgatewayv1alpha1.MinecraftFallbackRoute{}); err != nil {
		return err
	}
	return nil
}

func indexRouteParents[T client.Object](mgr ctrl.Manager, zero T) error {
	ctx := context.Background()
	// by gateway (ns/name)
	if err := mgr.GetFieldIndexer().IndexField(ctx, zero, index.RouteByGateway,
		func(o client.Object) []string {
			return extractGatewayParentKeys(o, false)
		},
	); err != nil {
		return err
	}
	// by gateway+listener (ns/name#listener)
	if err := mgr.GetFieldIndexer().IndexField(ctx, zero, index.RouteByGatewayListener,
		func(o client.Object) []string {
			return extractGatewayParentKeys(o, true)
		},
	); err != nil {
		return err
	}
	// by svc (ns/name)
	if err := mgr.GetFieldIndexer().IndexField(ctx, zero, index.RouteByService,
		func(o client.Object) []string {
			return extractServiceKeys(o)
		},
	); err != nil {
		return err
	}
	return nil
}

func extractServiceKeys(o client.Object) []string {
	r, err := extractRouteFromObject(o)
	if err != nil {
		return nil
	}
	ns := r.GetNamespace()
	result := make([]string, 0)
	for _, ref := range r.BackendRefs() {
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
	r, err := extractRouteFromObject(o)
	if err != nil {
		log.Print(err)
		return nil
	}
	return parentKeysFromRefs(r.GetNamespace(), r.ParentRefs(), withListener)
}

func extractRouteFromObject(o client.Object) (topology.Route, error) {
	switch r := o.(type) {
	case *mcgatewayv1alpha1.MinecraftJoinRoute:
		return topology.ForJoinRoute(r), nil
	case *mcgatewayv1alpha1.MinecraftFallbackRoute:
		return topology.ForFallbackRoute(r), nil
	default:
		return nil, fmt.Errorf("unsupported route type: %T", o)
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
	if err := c.List(ctx, zero, client.MatchingFields{index.RouteByGateway: keyGW(gw.Namespace, gw.Name)}); err != nil {
		return err
	}
	return nil
}

func ListAllRoutesByGateway(c client.Client, ctx context.Context, gw gatewayv1.Gateway, into *topology.RouteBag) error {
	var join mcgatewayv1alpha1.MinecraftJoinRouteList
	var fallback mcgatewayv1alpha1.MinecraftFallbackRouteList
	if err := ListRoutesByGateway(c, ctx, gw, &join); err != nil {
		return err
	}
	if err := ListRoutesByGateway(c, ctx, gw, &fallback); err != nil {
		return err
	}
	*into = topology.ForRouteBag(join, fallback)
	return nil
}

func ListRoutesByService[T client.ObjectList](c client.Client, ctx context.Context, svc corev1.Service, zero T) error {
	if err := c.List(ctx, zero, client.MatchingFields{index.RouteByService: keySvc(svc.Namespace, svc.Name)}); err != nil {
		return err
	}
	return nil
}

func ListAllRoutesByService(c client.Client, ctx context.Context, svc corev1.Service, into *topology.RouteBag) error {
	var join mcgatewayv1alpha1.MinecraftJoinRouteList
	var fallback mcgatewayv1alpha1.MinecraftFallbackRouteList
	if err := ListRoutesByService(c, ctx, svc, &join); err != nil {
		return err
	}
	if err := ListRoutesByService(c, ctx, svc, &fallback); err != nil {
		return err
	}

	verifier := topology.NewReferenceVerifier(c, ctx)

	filteredJoin, err := filterGranted(verifier, &join, &svc)
	if err != nil {
		return err
	}
	filteredFallback, err := filterGranted(verifier, &fallback, &svc)
	if err != nil {
		return err
	}
	*into = topology.ForRouteBag(*filteredJoin, *filteredFallback)
	return nil
}

func filterGranted[T client.ObjectList](v *topology.ReferenceVerifier, list T, obj client.Object) (T, error) {
	items, err := apimeta.ExtractList(list)
	if err != nil {
		return list, err
	}
	out := items[:0]
	for _, item := range items {
		o := item.(client.Object)
		granted, err := v.IsGranted(o, obj)
		if err != nil {
			return list, err
		}
		if granted {
			out = append(out, item)
		}
	}
	if err := apimeta.SetList(list, out); err != nil {
		return list, err
	}
	return list, nil
}
