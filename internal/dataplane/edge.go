package dataplane

import (
	"context"
	"fmt"
	"sync"

	"k8s.io/apimachinery/pkg/types"
	"minefleet.dev/minecraft-gateway/internal/dataplane/edge"
	"minefleet.dev/minecraft-gateway/internal/topology"
	"sigs.k8s.io/controller-runtime/pkg/client"
	logf "sigs.k8s.io/controller-runtime/pkg/log"
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

func (d *EdgeDataplane) SyncGateway(tree *topology.GatewayTree) error {
	infra := tree.Infrastructure
	name := tree.NamespacedName()

	if err := d.manager.SyncDaemonSet(d.ctx, infra.Config.Edge); err != nil {
		return fmt.Errorf("sync edge daemonset: %w", err)
	}
	d.mu.Lock()
	previous, hadPrevious := d.snapshotCache[name]
	d.snapshotCache[name] = edge.BuildGatewaySnapshot(name, tree.Listeners(), infra.Config.Edge)
	snap, conflicting := edge.BuildSnapshot(d.snapshotCache)
	if _, selfConflicting := conflicting[name]; selfConflicting {
		if hadPrevious {
			d.snapshotCache[name] = previous
		} else {
			delete(d.snapshotCache, name)
		}
		snap, _ = edge.BuildSnapshot(d.snapshotCache)
	}
	d.mu.Unlock()

	if err := d.sendSnapshot(snap); err != nil {
		return err
	}
	if len(conflicting) != 0 {
		return RouteConflictError{Conflicting: conflicting}
	}
	return nil
}

func (d *EdgeDataplane) sendSnapshot(snap edge.Snapshot) error {
	select {
	case d.updates <- snap:
		return nil
	default:
	}
	select {
	case <-d.updates:
	default:
	}
	select {
	case d.updates <- snap:
		return nil
	case <-d.ctx.Done():
		return d.ctx.Err()
	}
}

func (d *EdgeDataplane) DeleteGateway(name types.NamespacedName) error {
	d.mu.Lock()
	delete(d.snapshotCache, name)
	snap, conflicting := edge.BuildSnapshot(d.snapshotCache)
	d.mu.Unlock()
	if err := d.sendSnapshot(snap); err != nil {
		return err
	}
	if len(conflicting) != 0 {
		return RouteConflictError{Conflicting: conflicting}
	}
	return nil
}
