package route

import (
	"context"

	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/client"
	gatewayv1 "sigs.k8s.io/gateway-api/apis/v1"
)

// ResolvedParent holds the Gateway and relevant Listeners for a parentRef.
// Only returned when the gateway resource exists.
type ResolvedParent struct {
	Gateway gatewayv1.Gateway
	// Listeners matched by this parentRef:
	// - SectionName set → the single named listener (empty if the name is absent).
	// - SectionName absent → all listeners whose AllowedRoutes.Kinds include routeKind.
	Listeners []gatewayv1.Listener
}

// ParentFromRef resolves a ParentReference to its Gateway and the relevant Listeners.
// Returns a zero NamespacedName and nil result when ref does not target a Gateway kind.
// Returns a non-zero NamespacedName and nil result when the gateway does not exist.
//
// TODO: add ListenerSet support (gwv1alpha2).
func ParentFromRef(
	ctx context.Context,
	c client.Client,
	ref gatewayv1.ParentReference,
	routeNS string,
	routeKind gatewayv1.Kind,
) (types.NamespacedName, *ResolvedParent, error) {
	g := ""
	if ref.Group != nil {
		g = string(*ref.Group)
	}
	k := ""
	if ref.Kind != nil {
		k = string(*ref.Kind)
	}
	if (g != "" && g != gatewayv1.GroupName) || (k != "" && k != "Gateway") {
		return types.NamespacedName{}, nil, nil
	}
	ns := routeNS
	if ref.Namespace != nil && *ref.Namespace != "" {
		ns = string(*ref.Namespace)
	}
	nn := types.NamespacedName{Namespace: ns, Name: string(ref.Name)}

	var gw gatewayv1.Gateway
	if err := c.Get(ctx, nn, &gw); err != nil {
		if client.IgnoreNotFound(err) == nil {
			return nn, nil, nil
		}
		return nn, nil, err
	}

	var listeners []gatewayv1.Listener
	if ref.SectionName != nil && *ref.SectionName != "" {
		for _, l := range gw.Spec.Listeners {
			if l.Name == *ref.SectionName {
				listeners = []gatewayv1.Listener{l}
				break
			}
		}
	} else {
		for _, l := range gw.Spec.Listeners {
			supported, _ := ListenerRouteKindStatus(l)
			for _, sk := range supported {
				if sk.Kind == routeKind {
					listeners = append(listeners, l)
					break
				}
			}
		}
	}

	return nn, &ResolvedParent{Gateway: gw, Listeners: listeners}, nil
}
