package dataplane

import (
	"context"
	"os"

	discoveryv1 "k8s.io/api/discovery/v1"
	"k8s.io/apimachinery/pkg/types"
	"minefleet.dev/minecraft-gateway/internal/dataplane/edge"
	"minefleet.dev/minecraft-gateway/internal/route"
	"sigs.k8s.io/controller-runtime/pkg/client"
	gatewayv1 "sigs.k8s.io/gateway-api/apis/v1"
)

type Dataplane interface {
	SyncGateway(name types.NamespacedName, routes map[gatewayv1.Listener]route.Bag, backends []discoveryv1.EndpointSlice) ([]types.NamespacedName, error)
	DeleteGateway(name types.NamespacedName) ([]types.NamespacedName, error)
}

type dataplanes struct {
	items []Dataplane
}

func (d dataplanes) SyncGateway(name types.NamespacedName, routes map[gatewayv1.Listener]route.Bag, backends []discoveryv1.EndpointSlice) ([]types.NamespacedName, error) {
	for _, dataplane := range d.items {
		if conflicting, err := dataplane.SyncGateway(name, routes, backends); conflicting != nil || err != nil {
			return conflicting, err
		}
	}
	return nil, nil
}

func (d dataplanes) DeleteGateway(name types.NamespacedName) ([]types.NamespacedName, error) {
	for _, dataplane := range d.items {
		if conflicting, err := dataplane.DeleteGateway(name); conflicting != nil || err != nil {
			return conflicting, err
		}
	}

	return nil, nil
}

// CreateDataplane creates the composite dataplane.
// cfg configures the edge proxy: the DaemonSet namespace, container image,
// local xDS gRPC port, and the hostname DaemonSet pods use to reach the xDS server.
func CreateDataplane(ctx context.Context, c client.Client, cfg edge.ProxyConfig) Dataplane {
	return dataplanes{
		items: []Dataplane{
			newEdgeDataplane(ctx, c, cfg),
			newNetworkDataplane(ctx, c),
		},
	}
}

type Executor struct {
	Client    client.Client
	Dataplane *Dataplane
}

func (e Executor) Start(ctx context.Context) error {
	namespace, err := os.ReadFile("/var/run/secrets/kubernetes.io/serviceaccount/namespace")
	if err != nil {
		return err
	}
	plane := CreateDataplane(ctx, e.Client, edge.ProxyConfig{
		Namespace: string(namespace),
		XDSPort:   18000,
	})
	*e.Dataplane = plane
	return nil
}

func (Executor) NeedLeaderElection() bool {
	return true
}
