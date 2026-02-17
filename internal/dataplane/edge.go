package dataplane

import (
	"context"

	discoveryv1 "k8s.io/api/discovery/v1"
	"k8s.io/apimachinery/pkg/types"
	"minefleet.dev/minecraft-gateway/internal/dataplane/edge"
	"minefleet.dev/minecraft-gateway/internal/route"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

type EdgeDataplane struct {
	ctx     context.Context
	c       client.Client
	updates chan edge.DomainSnapshot
}

func newEdgeDataplane(ctx context.Context, c client.Client) Dataplane {
	d := &EdgeDataplane{
		ctx:     ctx,
		c:       c,
		updates: make(chan edge.DomainSnapshot, 1),
	}
	d.SetupDataplane()
	return d
}

func (d *EdgeDataplane) SetupDataplane() {
	edge.StartADS(d.ctx, d.updates)
	edge.EnsureEnvoy()
}

func (d *EdgeDataplane) SyncGateway(name types.NamespacedName, routes route.Bag, _ []discoveryv1.EndpointSlice) error {
	domains := edge.Domains(routes)
	snap := make(map[string]types.NamespacedName, len(domains))
	for _, domain := range domains {
		snap[domain] = name
	}

	select {
	case d.updates <- snap:
		return nil
	default:
		select {
		case <-d.updates:
		default:
		}
		select {
		case d.updates <- snap:
		case <-d.ctx.Done():
			return d.ctx.Err()
		}
		return nil
	}
}
