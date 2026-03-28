package dataplane

import (
	"context"
	"sync"

	discoveryv1 "k8s.io/api/discovery/v1"
	"k8s.io/apimachinery/pkg/types"
	"minefleet.dev/minecraft-gateway/internal/dataplane/edge"
	"minefleet.dev/minecraft-gateway/internal/route"
	"sigs.k8s.io/controller-runtime/pkg/client"
	logf "sigs.k8s.io/controller-runtime/pkg/log"
	gatewayv1 "sigs.k8s.io/gateway-api/apis/v1"
)

type EdgeDataplane struct {
	ctx           context.Context
	c             client.Client
	cfg           edge.ProxyConfig
	updates       chan edge.Snapshot
	mu            sync.Mutex
	snapshotCache edge.GatewaySnapshotCache
}

func newEdgeDataplane(ctx context.Context, c client.Client, cfg edge.ProxyConfig) Dataplane {
	d := &EdgeDataplane{
		ctx:           ctx,
		c:             c,
		cfg:           cfg,
		updates:       make(chan edge.Snapshot, 1),
		snapshotCache: make(edge.GatewaySnapshotCache),
	}
	d.SetupDataplane()
	return d
}

func (d *EdgeDataplane) SetupDataplane() {
	edge.StartADS(d.ctx, d.updates, d.cfg, d.c)
}

func (d *EdgeDataplane) SyncGateway(name types.NamespacedName, routes map[gatewayv1.Listener]route.Bag, _ []discoveryv1.EndpointSlice) ([]types.NamespacedName, error) {
	log := logf.FromContext(d.ctx)
	d.mu.Lock()
	tmp := edge.BuildGatewaySnapshot(name, routes)
	log.Info("built", "gatewaysnapshot", tmp)
	d.snapshotCache[name] = tmp
	snap, conflicting := edge.BuildSnapshot(d.snapshotCache)
	d.mu.Unlock()
	log.Info("total", "snapshot", snap)
	if len(conflicting) != 0 {
		return conflicting, nil
	}
	select {
	case d.updates <- snap:
		return nil, nil
	default:
		select {
		case <-d.updates:
		default:
		}
		select {
		case d.updates <- snap:
		case <-d.ctx.Done():
			return nil, d.ctx.Err()
		}
		return nil, nil
	}
}

func (d *EdgeDataplane) DeleteGateway(name types.NamespacedName) ([]types.NamespacedName, error) {
	d.mu.Lock()
	delete(d.snapshotCache, name)
	snap, conflicting := edge.BuildSnapshot(d.snapshotCache)
	d.mu.Unlock()
	if len(conflicting) != 0 {
		return conflicting, nil
	}
	select {
	case d.updates <- snap:
		return nil, nil
	default:
		select {
		case <-d.updates:
		default:
		}
		select {
		case d.updates <- snap:
		case <-d.ctx.Done():
			return nil, d.ctx.Err()
		}
		return nil, nil
	}
}
