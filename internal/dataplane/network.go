package dataplane

import (
	"context"

	discoveryv1 "k8s.io/api/discovery/v1"
	"k8s.io/apimachinery/pkg/types"
	mcgatewayv1 "minefleet.dev/minecraft-gateway/api/v1"
	"minefleet.dev/minecraft-gateway/internal/route"
	"sigs.k8s.io/controller-runtime/pkg/client"
	gatewayv1 "sigs.k8s.io/gateway-api/apis/v1"
)

type NetworkDataplane struct {
	Data []mcgatewayv1.MinecraftService
	ctx  context.Context
	c    client.Client
}

func newNetworkDataplane(ctx context.Context, c client.Client) Dataplane {
	return NetworkDataplane{
		Data: make([]mcgatewayv1.MinecraftService, 0),
		ctx:  ctx,
		c:    c,
	}
}

func (d NetworkDataplane) SetupDataplane() {

}

func (d NetworkDataplane) SyncGateway(name types.NamespacedName, routes map[gatewayv1.Listener]route.Bag, backends []discoveryv1.EndpointSlice) ([]types.NamespacedName, error) {
	return nil, nil
}

func (d NetworkDataplane) DeleteGateway(name types.NamespacedName) ([]types.NamespacedName, error) {
	return nil, nil
}
