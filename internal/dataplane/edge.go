package dataplane

import (
	"context"
	"fmt"
	"sync"

	discoveryv1 "k8s.io/api/discovery/v1"
	"k8s.io/apimachinery/pkg/types"
	"minefleet.dev/minecraft-gateway/internal/dataplane/edge"
	"minefleet.dev/minecraft-gateway/internal/gateway"
	"minefleet.dev/minecraft-gateway/internal/route"
	"sigs.k8s.io/controller-runtime/pkg/client"
	logf "sigs.k8s.io/controller-runtime/pkg/log"
	gatewayv1 "sigs.k8s.io/gateway-api/apis/v1"
)

type EdgeDataplane struct {
	ctx           context.Context
	c             client.Client
	cfg           edge.Config
	updates       chan edge.Snapshot
	mu            sync.Mutex
	snapshotCache edge.GatewaySnapshotCache
	manager       *edge.ProxyManager
}

func newEdgeDataplane(ctx context.Context, c client.Client, cfg edge.Config) Dataplane {
	d := &EdgeDataplane{
		ctx:           ctx,
		c:             c,
		cfg:           cfg,
		updates:       make(chan edge.Snapshot, 1),
		snapshotCache: make(edge.GatewaySnapshotCache),
		manager:       edge.NewProxyManager(c, cfg),
	}
	d.SetupDataplane()
	return d
}

func (d *EdgeDataplane) SetupDataplane() {
	edge.StartADS(d.ctx, d.updates, d.cfg, d.c)
	go func() {
		if err := d.manager.SyncBootstrap(d.ctx); err != nil {
			logf.FromContext(d.ctx).Error(err, "failed to sync edge bootstrap ConfigMap")
		}
	}()
}

func (d *EdgeDataplane) SyncGateway(name types.NamespacedName, infra gateway.Infrastructure, routes map[gatewayv1.Listener]route.Bag, _ []discoveryv1.EndpointSlice) error {
	if err := d.manager.SyncDaemonSet(d.ctx, infra.Config.Edge); err != nil {
		return fmt.Errorf("sync edge daemonset: %w", err)
	}
	d.mu.Lock()
	tmp := edge.BuildGatewaySnapshot(name, routes, infra.Config.Edge)
	d.snapshotCache[name] = tmp
	snap, conflicting := edge.BuildSnapshot(d.snapshotCache)
	d.mu.Unlock()
	if len(conflicting) != 0 {
		return RouteConflictError{Conflicting: conflicting}
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

func (d *EdgeDataplane) DeleteGateway(name types.NamespacedName) error {
	d.mu.Lock()
	delete(d.snapshotCache, name)
	snap, conflicting := edge.BuildSnapshot(d.snapshotCache)
	d.mu.Unlock()
	if len(conflicting) != 0 {
		return RouteConflictError{Conflicting: conflicting}
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
