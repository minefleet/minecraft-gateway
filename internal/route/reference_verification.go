package route

import (
	"context"

	"sigs.k8s.io/controller-runtime/pkg/client"
	gatewayv1 "sigs.k8s.io/gateway-api/apis/v1beta1"
)

// ReferenceVerifier checks whether a ReferenceGrant permits a route to
// reference a backend. Grants are fetched once per target namespace and
// cached for the lifetime of the verifier.
type ReferenceVerifier struct {
	c      client.Client
	ctx    context.Context
	grants map[string][]gatewayv1.ReferenceGrant
}

func NewReferenceVerifier(c client.Client, ctx context.Context) *ReferenceVerifier {
	return &ReferenceVerifier{
		c:      c,
		ctx:    ctx,
		grants: make(map[string][]gatewayv1.ReferenceGrant),
	}
}

// IsGranted reports whether route is permitted to reference obj.
// Same-namespace references are always allowed. Cross-namespace references
// require a ReferenceGrant in obj's namespace. Grants for each namespace are
// fetched once and reused across calls.
func (v *ReferenceVerifier) IsGranted(route client.Object, obj client.Object) (bool, error) {
	if route.GetNamespace() == obj.GetNamespace() {
		return true, nil
	}
	ns := obj.GetNamespace()
	if _, ok := v.grants[ns]; !ok {
		var list gatewayv1.ReferenceGrantList
		if err := v.c.List(v.ctx, &list, client.InNamespace(ns)); err != nil {
			return false, err
		}
		v.grants[ns] = list.Items
	}
	for _, grant := range v.grants[ns] {
		fromMatched := false
		for _, from := range grant.Spec.From {
			if string(from.Namespace) == route.GetNamespace() &&
				string(from.Kind) == route.GetObjectKind().GroupVersionKind().Kind &&
				string(from.Group) == route.GetObjectKind().GroupVersionKind().Group {
				fromMatched = true
				break
			}
		}
		if !fromMatched {
			continue
		}
		for _, to := range grant.Spec.To {
			if to.Name != nil && string(*to.Name) == obj.GetName() &&
				string(to.Kind) == obj.GetObjectKind().GroupVersionKind().Kind &&
				string(to.Group) == obj.GetObjectKind().GroupVersionKind().Group {
				return true, nil
			}
		}
	}
	return false, nil
}
