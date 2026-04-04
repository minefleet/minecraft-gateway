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
	proxyMgr      *network.ProxyManager
}

func newNetworkDataplane(ctx context.Context, c client.Client, cfg network.Config) Dataplane {
	d := NetworkDataplane{
		ctx:           ctx,
		c:             c,
		cfg:           cfg,
		updates:       make(chan network.Snapshot, 1),
		snapshotCache: make(network.GatewaySnapshotCache),
		proxyMgr:      network.NewProxyManager(c, cfg),
	}
	d.SetupDataplane()
	return &d
}

func (d *NetworkDataplane) SetupDataplane() {
	network.StartADS(d.ctx, d.updates, d.cfg, d.c)
}

func (d *NetworkDataplane) SyncGateway(name types.NamespacedName, routes map[gatewayv1.Listener]route.Bag, backends []discoveryv1.EndpointSlice) error {
	d.mu.Lock()
	d.snapshotCache[name] = make(map[string]network.ListenerSnapshot)
	for listener, bag := range routes {
		d.snapshotCache[name][string(listener.Name)] = network.BuildListenerSnapshot(name, listener, bag, backends)
	}
	snap := network.BuildSnapshot(d.snapshotCache)
	d.mu.Unlock()

	select {
	case d.updates <- snap:
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
	}

	listeners := make([]gatewayv1.Listener, 0, len(routes))
	for l := range routes {
		listeners = append(listeners, l)
	}
	return d.proxyMgr.Sync(d.ctx, name, listeners)
}

func (d *NetworkDataplane) DeleteGateway(name types.NamespacedName) error {
	d.mu.Lock()
	delete(d.snapshotCache, name)
	snap := network.BuildSnapshot(d.snapshotCache)
	d.mu.Unlock()

	select {
	case d.updates <- snap:
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
	}

	return d.proxyMgr.Delete(d.ctx, name)
}
