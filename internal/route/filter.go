package route

import (
	"context"
	"strings"

	mcgatewayv1 "minefleet.dev/minecraft-gateway/api/v1"
	"minefleet.dev/minecraft-gateway/internal/util"
	"sigs.k8s.io/controller-runtime/pkg/client"
	gatewayv1 "sigs.k8s.io/gateway-api/apis/v1"
	v1 "sigs.k8s.io/gateway-api/conformance/apis/v1"
)

func FilterAllowedRoutes(c client.Client, ctx context.Context, gw gatewayv1.Gateway, routes Bag) map[gatewayv1.Listener]Bag {
	result := make(map[gatewayv1.Listener]Bag)
	for _, l := range gw.Spec.Listeners {
		namespaces, err := util.SelectNamespace(c, ctx, gw.Namespace, l.AllowedRoutes.Namespaces)
		if err != nil {
			continue
		}
		localBag := Bag{
			Join:     make([]mcgatewayv1.MinecraftJoinRoute, 0),
			Fallback: make([]mcgatewayv1.MinecraftFallbackRoute, 0),
		}
		for _, login := range routes.Join {
			if IsRouteAllowed(&login, l.AllowedRoutes.Kinds, namespaces) {
				localBag.Join = append(localBag.Join, login)
			}
		}

		for _, fallback := range routes.Fallback {
			if IsRouteAllowed(&fallback, l.AllowedRoutes.Kinds, namespaces) {
				localBag.Fallback = append(localBag.Fallback, fallback)
			}
		}

		result[l] = dedupeRoutes(localBag)
	}
	return result
}

func IsRouteAllowed(obj client.Object, allowedKinds []gatewayv1.RouteGroupKind, allowedNamespaces []string) bool {
	ns := obj.GetNamespace()
	found := false
	if allowedNamespaces == nil || len(allowedNamespaces) > 0 {
		for _, allowed := range allowedNamespaces {
			if found {
				continue
			}
			if allowed != ns {
				continue
			}
			found = true
		}
		if !found {
			return false
		}
	}
	if len(allowedKinds) > 0 {
		found = false
		kind := obj.GetObjectKind().GroupVersionKind()
		for _, allowed := range allowedKinds {
			if found {
				continue
			}
			group := allowed.Group
			if group == nil {
				defaultGroup := gatewayv1.Group(v1.Group)
				group = &defaultGroup
			}
			if kind.Group != string(*group) {
				continue
			}
			if kind.Kind != string(allowed.Kind) {
				continue
			}
			found = true
		}
	}
	return found
}

func dedupeRoutes(in Bag) Bag {
	seen := map[string]struct{}{}
	out := Bag{
		Join:     make([]mcgatewayv1.MinecraftJoinRoute, 0),
		Fallback: make([]mcgatewayv1.MinecraftFallbackRoute, 0),
	}

	dedupe := func(ns, name, kind string) bool {
		k := strings.Join([]string{kind, ns, name}, "/")
		if _, ok := seen[k]; ok {
			return false
		}
		seen[k] = struct{}{}
		return true
	}

	for _, r := range in.Join {
		if dedupe(r.Namespace, r.Name, "Join") {
			out.Join = append(out.Join, r)
		}
	}
	for _, r := range in.Fallback {
		if dedupe(r.Namespace, r.Name, "Fallback") {
			out.Fallback = append(out.Fallback, r)
		}
	}

	return out
}
