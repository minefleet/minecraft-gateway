package discovery

import (
	"context"
	"errors"
	"fmt"

	"k8s.io/apimachinery/pkg/types"
	"k8s.io/utils/ptr"
	mcgatewayv1 "minefleet.dev/minecraft-gateway/api/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"
	gatewayv1 "sigs.k8s.io/gateway-api/apis/v1"
)

const (
	ServerDiscoveryKind = "MinecraftServerDiscovery"
)

func GetMinecraftServerDiscoveryByGateway(c client.Client, ctx context.Context, gw gatewayv1.Gateway) (mcgatewayv1.MinecraftServerDiscovery, error) {
	if gw.Spec.Infrastructure == nil || gw.Spec.Infrastructure.ParametersRef == nil {
		return mcgatewayv1.MinecraftServerDiscovery{}, errors.New("no infrastructure provided")
	}
	ref := gatewayv1.ParametersReference{
		Group:     gw.Spec.Infrastructure.ParametersRef.Group,
		Kind:      gw.Spec.Infrastructure.ParametersRef.Kind,
		Name:      gw.Spec.Infrastructure.ParametersRef.Name,
		Namespace: ptr.To(gatewayv1.Namespace(gw.Namespace)),
	}
	return GetMinecraftServerDiscoveryByRef(c, ctx, &ref)
}

func GetMinecraftServerDiscoveryByRef(c client.Client, ctx context.Context, ref *gatewayv1.ParametersReference) (mcgatewayv1.MinecraftServerDiscovery, error) {
	if ref == nil {
		return mcgatewayv1.MinecraftServerDiscovery{}, errors.New("no infrastructure provided")
	}
	if string(ref.Group) != mcgatewayv1.GroupVersion.Group || string(ref.Kind) != ServerDiscoveryKind {
		return mcgatewayv1.MinecraftServerDiscovery{}, fmt.Errorf("invalid infrastructure type: %s/%s", ref.Group, ref.Kind)
	}
	var discovery mcgatewayv1.MinecraftServerDiscovery
	err := c.Get(ctx, types.NamespacedName{
		Namespace: string(*ref.Namespace),
		Name:      ref.Name,
	}, &discovery)
	if err != nil {
		return mcgatewayv1.MinecraftServerDiscovery{}, err
	}
	return discovery, nil
}
