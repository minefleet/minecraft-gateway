package state

import (
	"context"
	"k8s.io/apimachinery/pkg/types"
	mcgatewayv1 "minefleet.dev/minecraft-gateway/api/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"
	gatewayv1 "sigs.k8s.io/gateway-api/apis/v1"
)

type GatewayState struct {
	Checksum string
	Version  uint64
	state    []mcgatewayv1.MinecraftService
}

func NewGatewayState(c client.Client, ctx context.Context, gw types.NamespacedName, lastVersion uint64) GatewayState {

}

func (r *GatewayState) fetchJoinRoutes(gw gatewayv1.Gateway) ([]mcgatewayv1.MinecraftRoute, error) {
	namespaces, err := gw.Spec
}

func (r *GatewayState) fetchServices(disc mcgatewayv1.MinecraftServerDiscovery) []mcgatewayv1.MinecraftService {

}
