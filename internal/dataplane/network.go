package dataplane

import (
	"context"
	"sync"

	corev1 "k8s.io/api/core/v1"
	discoveryv1 "k8s.io/api/discovery/v1"
	"k8s.io/apimachinery/pkg/types"
	"minefleet.dev/minecraft-gateway/internal/dataplane/network"
	"minefleet.dev/minecraft-gateway/internal/topology"
	"sigs.k8s.io/controller-runtime/pkg/client"
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

func (d *NetworkDataplane) SyncGateway(tree *topology.GatewayTree) error {
	name := tree.NamespacedName()
	infra := tree.Infrastructure
	backends := tree.Backends

	podAnnotations := collectPodAnnotations(d.ctx, d.c, backends)

	d.mu.Lock()
	d.snapshotCache[name] = make(map[string]network.ListenerSnapshot)
	for _, lt := range tree.Listeners() {
		listenerName := string(lt.Listener.GetName())
		d.snapshotCache[name][listenerName] = network.BuildListenerSnapshot(name, lt, backends, podAnnotations)
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

	listeners := make([]topology.Listener, 0, len(tree.Listeners()))
	for _, lt := range tree.Listeners() {
		listeners = append(listeners, lt.Listener)
	}
	return d.proxyMgr.Sync(d.ctx, name, listeners, infra)
}

func collectPodAnnotations(ctx context.Context, c client.Client, backends []discoveryv1.EndpointSlice) map[string]map[string]string {
	result := make(map[string]map[string]string)
	for _, slice := range backends {
		for _, ep := range slice.Endpoints {
			if ep.TargetRef == nil || ep.TargetRef.Kind != "Pod" || ep.TargetRef.Name == "" {
				continue
			}
			key := ep.TargetRef.Namespace + "/" + ep.TargetRef.Name
			if _, ok := result[key]; ok {
				continue
			}
			var pod corev1.Pod
			if err := c.Get(ctx, types.NamespacedName{
				Namespace: ep.TargetRef.Namespace,
				Name:      ep.TargetRef.Name,
			}, &pod); err != nil {
				continue
			}
			result[key] = pod.Annotations
		}
	}
	return result
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

	return nil
}
