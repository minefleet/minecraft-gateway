package route

import (
	"context"

	"sigs.k8s.io/controller-runtime/pkg/client"
	gatewayv1 "sigs.k8s.io/gateway-api/apis/v1beta1"
)

type ReferenceVerifier struct {
	route client.Object
	c     client.Client
	ctx   context.Context
}

func NewReferenceVerifier(route client.Object, c client.Client, ctx context.Context) ReferenceVerifier {
	return ReferenceVerifier{
		route: route,
		c:     c,
		ctx:   ctx,
	}
}

func (v ReferenceVerifier) IsGranted(obj client.Object) (bool, error) {
	if v.route.GetNamespace() == obj.GetNamespace() {
		return true, nil
	}
	var grants gatewayv1.ReferenceGrantList
	if err := v.c.List(v.ctx, &grants, client.InNamespace(obj.GetNamespace())); err != nil {
		return false, err
	}
	for _, grant := range grants.Items {
		found := false
		for _, from := range grant.Spec.From {
			if string(from.Namespace) == v.route.GetNamespace() && string(from.Kind) == v.route.GetObjectKind().GroupVersionKind().Kind && string(from.Group) == v.route.GetObjectKind().GroupVersionKind().Group {
				found = true
			}
		}
		if !found {
			continue
		}

		for _, to := range grant.Spec.To {
			if to.Name != nil && string(*to.Name) == obj.GetName() && string(to.Kind) == obj.GetObjectKind().GroupVersionKind().Kind && string(to.Group) == obj.GetObjectKind().GroupVersionKind().Group {
				return true, nil
			}
		}
	}
	return false, nil
}
