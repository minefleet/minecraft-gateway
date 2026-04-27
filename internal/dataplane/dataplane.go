package dataplane

import (
	"context"
	"errors"
	"fmt"
	"os"

	"k8s.io/apimachinery/pkg/types"
	"minefleet.dev/minecraft-gateway/internal/dataplane/edge"
	"minefleet.dev/minecraft-gateway/internal/dataplane/network"
	"minefleet.dev/minecraft-gateway/internal/topology"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

type Dataplane interface {
	SyncGateway(tree *topology.GatewayTree) error
	DeleteGateway(name types.NamespacedName) error
}

type dataplanes struct {
	items []Dataplane
}

func (d dataplanes) SyncGateway(tree *topology.GatewayTree) error {
	var conflictErr error
	for _, dp := range d.items {
		if err := dp.SyncGateway(tree); err != nil {
			var rce RouteConflictError
			if errors.As(err, &rce) {
				conflictErr = err
				continue
			}
			return err
		}
	}
	return conflictErr
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
	podIP := os.Getenv("POD_IP")
	if podIP == "" {
		return fmt.Errorf("POD_IP env var not set; add downward API fieldRef status.podIP to the manager pod")
	}
	plane := CreateDataplane(ctx, e.Client, Config{
		Edge: edge.Config{
			Namespace: string(controllerNamespace),
			XDSPort:   18000,
			PodIP:     podIP,
		},
		Network: network.Config{
			Namespace: string(controllerNamespace),
			XDSPort:   19000,
		},
	})
	*e.Dataplane = plane
	return nil
}

func (Executor) NeedLeaderElection() bool {
	return true
}
