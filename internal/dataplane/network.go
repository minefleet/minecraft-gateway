package dataplane

import (
	"context"
	discoveryv1 "k8s.io/api/discovery/v1"
	"k8s.io/apimachinery/pkg/types"
	mcgatewayv1 "minefleet.dev/minecraft-gateway/api/v1"
	"minefleet.dev/minecraft-gateway/internal/route"
	"sigs.k8s.io/controller-runtime/pkg/client"
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

func (d NetworkDataplane) SyncGateway(name types.NamespacedName, route route.Bag, backends []discoveryv1.EndpointSlice) error {
	return nil
}
