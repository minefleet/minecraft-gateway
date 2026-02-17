package dataplane

import (
	"context"
	discoveryv1 "k8s.io/api/discovery/v1"
	"k8s.io/apimachinery/pkg/types"
	"minefleet.dev/minecraft-gateway/internal/route"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

type Dataplane interface {
	SyncGateway(name types.NamespacedName, routes route.Bag, backends []discoveryv1.EndpointSlice) error
}

type dataplanes struct {
	items []Dataplane
}

func (d dataplanes) SyncGateway(name types.NamespacedName, routes route.Bag, backends []discoveryv1.EndpointSlice) error {
	for _, dataplane := range d.items {
		if err := dataplane.SyncGateway(name, routes, backends); err != nil {
			return err
		}
	}
	return nil
}

func CreateDataplane(ctx context.Context, c client.Client) Dataplane {
	return dataplanes{
		items: []Dataplane{
			newEdgeDataplane(ctx, c),
			newNetworkDataplane(ctx, c),
		},
	}
}
