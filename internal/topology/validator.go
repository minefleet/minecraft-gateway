package topology

import (
	"context"
	"fmt"

	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/client"
	gatewayv1 "sigs.k8s.io/gateway-api/apis/v1"
	gatewayv1beta1 "sigs.k8s.io/gateway-api/apis/v1beta1"
)

// ReferenceVerifier checks whether ReferenceGrants permit cross-namespace backend
// references. Grants are fetched once per target namespace and cached.
type ReferenceVerifier struct {
	c      client.Client
	ctx    context.Context
	grants map[string][]gatewayv1beta1.ReferenceGrant
}

// NewReferenceVerifier returns a verifier backed by c. It is not safe for
// concurrent use; create one per reconcile loop.
func NewReferenceVerifier(c client.Client, ctx context.Context) *ReferenceVerifier {
	return &ReferenceVerifier{c: c, ctx: ctx, grants: make(map[string][]gatewayv1beta1.ReferenceGrant)}
}

// IsGranted reports whether route is permitted to reference obj.
// Same-namespace references are always allowed. Cross-namespace references
// require a ReferenceGrant in obj's namespace.
func (v *ReferenceVerifier) IsGranted(route client.Object, obj client.Object) (bool, error) {
	if route.GetNamespace() == obj.GetNamespace() {
		return true, nil
	}
	ns := obj.GetNamespace()
	if _, ok := v.grants[ns]; !ok {
		var list gatewayv1beta1.ReferenceGrantList
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

// CheckBackendRefs verifies every BackendRef on r resolves to an existing Service
// and that cross-namespace references are covered by a ReferenceGrant.
func CheckBackendRefs(ctx context.Context, c client.Client, r Route) (bool, gatewayv1.RouteConditionReason, string) {
	verifier := NewReferenceVerifier(c, ctx)
	routeNS := r.GetNamespace()
	routeObj := r.Object()

	for _, ref := range r.BackendRefs() {
		if ref.Kind != nil && *ref.Kind != "Service" {
			return false, gatewayv1.RouteReasonInvalidKind,
				fmt.Sprintf("unsupported backend kind %q; only Service is supported", *ref.Kind)
		}

		ns := routeNS
		if ref.Namespace != nil && *ref.Namespace != "" {
			ns = string(*ref.Namespace)
		}

		var svc corev1.Service
		if err := c.Get(ctx, types.NamespacedName{Namespace: ns, Name: string(ref.Name)}, &svc); err != nil {
			if client.IgnoreNotFound(err) == nil {
				return false, gatewayv1.RouteReasonBackendNotFound,
					fmt.Sprintf("service %s/%s not found", ns, ref.Name)
			}
			return false, gatewayv1.RouteReasonBackendNotFound, err.Error()
		}

		if ns != routeNS {
			svc.TypeMeta = metav1.TypeMeta{APIVersion: "v1", Kind: "Service"}
			granted, err := verifier.IsGranted(routeObj, &svc)
			if err != nil {
				return false, gatewayv1.RouteReasonRefNotPermitted, err.Error()
			}
			if !granted {
				return false, gatewayv1.RouteReasonRefNotPermitted,
					fmt.Sprintf("no ReferenceGrant permits reference to service %s/%s", ns, ref.Name)
			}
		}
	}
	return true, gatewayv1.RouteReasonResolvedRefs, "All backend references resolved."
}
