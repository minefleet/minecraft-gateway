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
	SyncGateway(name types.NamespacedName, routes map[gatewayv1.Listener]route.Bag, backends []discoveryv1.EndpointSlice) error
	DeleteGateway(name types.NamespacedName) error
}

type dataplanes struct {
	items []Dataplane
}

func (d dataplanes) SyncGateway(name types.NamespacedName, routes map[gatewayv1.Listener]route.Bag, backends []discoveryv1.EndpointSlice) error {
	for _, dataplane := range d.items {
		if err := dataplane.SyncGateway(name, routes, backends); err != nil {
			return err
		}
	}
	return nil
}

func (d dataplanes) DeleteGateway(name types.NamespacedName) error {
	for _, dataplane := range d.items {
		if err := dataplane.DeleteGateway(name); err != nil {
			return err
		}
	}
	return nil
}

// CreateDataplane creates the composite dataplane.
// cfg configures the edge proxy: the DaemonSet namespace, container image,
// local xDS gRPC port, and the hostname DaemonSet pods use to reach the xDS server.
func CreateDataplane(ctx context.Context, c client.Client, cfg Config) Dataplane {
	return dataplanes{
		items: []Dataplane{
			newEdgeDataplane(ctx, c, cfg.Edge),
			newNetworkDataplane(ctx, c, cfg.Network),
		},
	}
}

type Executor struct {
	Client    client.Client
	Dataplane *Dataplane
}

func (e Executor) Start(ctx context.Context) error {
	controllerNamespace, err := os.ReadFile("/var/run/secrets/kubernetes.io/serviceaccount/namespace")
	if err != nil {
		return err
	}
	plane := CreateDataplane(ctx, e.Client, Config{Edge: edge.Config{
		Namespace: string(controllerNamespace),
		XDSPort:   18000,
	}})
	*e.Dataplane = plane
	return nil
}

func (Executor) NeedLeaderElection() bool {
	return true
}
