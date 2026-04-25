package route

import (
	"context"
	"fmt"

	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	mcgatewayv1alpha1 "minefleet.dev/minecraft-gateway/api/controller/v1alpha1"
	"sigs.k8s.io/controller-runtime/pkg/client"
	gatewayv1 "sigs.k8s.io/gateway-api/apis/v1"
)

// CheckBackendRefs verifies that every BackendRef in refs resolves to an
// existing Service and that cross-namespace references are covered by a
// ReferenceGrant. routeObj must have its TypeMeta set so the verifier can
// match ReferenceGrant From entries.
func CheckBackendRefs(
	ctx context.Context,
	c client.Client,
	routeNS string,
	routeObj client.Object,
	refs []mcgatewayv1alpha1.MinecraftBackendRef,
) (ok bool, reason gatewayv1.RouteConditionReason, msg string) {
	verifier := NewReferenceVerifier(c, ctx)

	for _, ref := range refs {
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
