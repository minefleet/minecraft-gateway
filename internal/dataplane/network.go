package dataplane

import (
	"context"
	"sync"

	discoveryv1 "k8s.io/api/discovery/v1"
	"k8s.io/apimachinery/pkg/types"
	"minefleet.dev/minecraft-gateway/internal/dataplane/network"
	"minefleet.dev/minecraft-gateway/internal/route"
	"sigs.k8s.io/controller-runtime/pkg/client"
	gatewayv1 "sigs.k8s.io/gateway-api/apis/v1"
)

type NetworkDataplane struct {
	ctx           context.Context
	c             client.Client
	cfg           network.Config
	updates       chan network.Snapshot
	mu            sync.Mutex
	snapshotCache network.GatewaySnapshotCache
}

func newNetworkDataplane(ctx context.Context, c client.Client, cfg network.Config) Dataplane {
	d := NetworkDataplane{
		ctx:           ctx,
		c:             c,
		cfg:           cfg,
		updates:       make(chan network.Snapshot, 1),
		snapshotCache: make(network.GatewaySnapshotCache),
	}
	d.SetupDataplane()
	return &d
}

func (d *NetworkDataplane) SetupDataplane() {
	network.StartADS(d.ctx, d.updates, d.cfg, d.c)
}

func (d *NetworkDataplane) SyncGateway(name types.NamespacedName, routes map[gatewayv1.Listener]route.Bag, backends []discoveryv1.EndpointSlice) error {
	d.mu.Lock()
	d.snapshotCache[name] = nil
	for listener, bag := range routes {
		d.snapshotCache[name][string(listener.Name)] = network.BuildListenerSnapshot(name, listener, bag, backends)
	}
	d.mu.Unlock()
	return nil
}

func (d *NetworkDataplane) DeleteGateway(name types.NamespacedName) error {
	d.mu.Lock()
	delete(d.snapshotCache, name)
	d.mu.Unlock()
	return nil
}
