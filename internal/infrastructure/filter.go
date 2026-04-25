package infrastructure

import (
	"context"
	"errors"
	"fmt"

	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/utils/ptr"
	mcgatewayv1alpha1 "minefleet.dev/minecraft-gateway/api/controller/v1alpha1"
	"sigs.k8s.io/controller-runtime/pkg/client"
	gatewayv1 "sigs.k8s.io/gateway-api/apis/v1"
)

const (
	ServerDiscoveryKind = "NetworkInfrastructure"
)

func GetNetworkInfrastructureByGateway(c client.Client, ctx context.Context, gw gatewayv1.Gateway) (mcgatewayv1alpha1.NetworkInfrastructure, error) {
	if gw.Spec.Infrastructure == nil || gw.Spec.Infrastructure.ParametersRef == nil {
		return mcgatewayv1alpha1.NetworkInfrastructure{}, errors.New("no infrastructure provided")
	}
	ref := gatewayv1.ParametersReference{
		Group:     gw.Spec.Infrastructure.ParametersRef.Group,
		Kind:      gw.Spec.Infrastructure.ParametersRef.Kind,
		Name:      gw.Spec.Infrastructure.ParametersRef.Name,
		Namespace: ptr.To(gatewayv1.Namespace(gw.Namespace)),
	}
	return GetNetworkInfrastructureByRef(c, ctx, &ref)
}

// GetNetworkInfrastructuresByService returns all NetworkInfrastructure objects
// that have resolved the given service into their Status.BackendRefs.
func GetNetworkInfrastructuresByService(c client.Client, ctx context.Context, svc corev1.Service) ([]mcgatewayv1alpha1.NetworkInfrastructure, error) {
	var all mcgatewayv1alpha1.NetworkInfrastructureList
	if err := c.List(ctx, &all); err != nil {
		return nil, err
	}
	result := make([]mcgatewayv1alpha1.NetworkInfrastructure, 0)
	for _, disc := range all.Items {
		for _, ref := range disc.Status.BackendRefs {
			refNs := disc.Namespace
			if ref.Namespace != nil {
				refNs = string(*ref.Namespace)
			}
			if refNs == svc.Namespace && string(ref.Name) == svc.Name {
				result = append(result, disc)
				break
			}
		}
	}
	return result, nil
}

func GetNetworkInfrastructureByRef(c client.Client, ctx context.Context, ref *gatewayv1.ParametersReference) (mcgatewayv1alpha1.NetworkInfrastructure, error) {
	if ref == nil {
		return mcgatewayv1alpha1.NetworkInfrastructure{}, errors.New("no infrastructure provided")
	}
	if string(ref.Group) != mcgatewayv1alpha1.GroupVersion.Group || string(ref.Kind) != ServerDiscoveryKind {
		return mcgatewayv1alpha1.NetworkInfrastructure{}, fmt.Errorf("invalid infrastructure type: %s/%s", ref.Group, ref.Kind)
	}
	var discovery mcgatewayv1alpha1.NetworkInfrastructure
	err := c.Get(ctx, types.NamespacedName{
		Namespace: string(*ref.Namespace),
		Name:      ref.Name,
	}, &discovery)
	if err != nil {
		return mcgatewayv1alpha1.NetworkInfrastructure{}, err
	}
	return discovery, nil
}
