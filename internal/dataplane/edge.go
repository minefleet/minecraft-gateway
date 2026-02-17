package dataplane

import (
	"context"
	discoveryv1 "k8s.io/api/discovery/v1"
	"k8s.io/apimachinery/pkg/types"
	"minefleet.dev/minecraft-gateway/internal/route"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

type EdgeDataplane struct {
	Data map[string]types.NamespacedName
	ctx  context.Context
	c    client.Client
}

func newEdgeDataplane(ctx context.Context, c client.Client) Dataplane {
	return EdgeDataplane{
		ctx: ctx,
		c:   c,
	}
}

func (d EdgeDataplane) SetupDataplane() {

}

func (d EdgeDataplane) SyncGateway(name types.NamespacedName, route route.Bag, backends []discoveryv1.EndpointSlice) error {
	return nil
}
