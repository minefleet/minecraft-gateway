package dataplane

import (
	"context"

	discoveryv1 "k8s.io/api/discovery/v1"
	"k8s.io/apimachinery/pkg/types"
	"minefleet.dev/minecraft-gateway/internal/dataplane/edge"
	"minefleet.dev/minecraft-gateway/internal/route"
	"sigs.k8s.io/controller-runtime/pkg/client"
	gatewayv1 "sigs.k8s.io/gateway-api/apis/v1"
)

type EdgeDataplane struct {
	ctx     context.Context
	c       client.Client
	cfg     edge.ProxyConfig
	updates chan edge.DomainSnapshot
}

func newEdgeDataplane(ctx context.Context, c client.Client, cfg edge.ProxyConfig) Dataplane {
	d := &EdgeDataplane{
		ctx:     ctx,
		c:       c,
		cfg:     cfg,
		updates: make(chan edge.DomainSnapshot, 1),
	}
	d.SetupDataplane()
	return d
}

func (d *EdgeDataplane) SetupDataplane() {
	edge.StartADS(d.ctx, d.updates, d.cfg, d.c)
}

func (d *EdgeDataplane) SyncGateway(name types.NamespacedName, routes map[gatewayv1.Listener]route.Bag, _ []discoveryv1.EndpointSlice) error {
	snap := edge.BuildGatewaySnapshot(name, routes)
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
