package edge

import (
	"fmt"
	"strings"

	"k8s.io/apimachinery/pkg/types"
	"minefleet.dev/minecraft-gateway/internal/route"
	gatewayv1 "sigs.k8s.io/gateway-api/apis/v1"
)

// Config holds the configuration for the edge dataplane.
type Config struct {
	// Namespace where the edge DaemonSet and ConfigMap will be created.
	Namespace string
	// XDSPort is the local port the xDS gRPC server listens on (default: 18000).
	XDSPort int
}

// GatewaySnapshot is the full routing config for one gateway sync cycle.
type GatewaySnapshot struct {
	// DomainMapping maps each hostname to a cluster name.
	DomainMapping map[string]string
	// Clusters defines the upstream clusters with their endpoints.
	Clusters []ClusterConfig
}

// ClusterConfig holds the Envoy cluster definition.
type ClusterConfig struct {
	Name      string
	Endpoints []EndpointConfig
}

// EndpointConfig is a single upstream address.
type EndpointConfig struct {
	Address string
	Port    uint32
}

type Snapshot struct {
	GatewaySnapshot
	// RejectUnknown controls whether connections to unmapped domains are rejected.
	RejectUnknown bool
}

type GatewaySnapshotCache = map[types.NamespacedName]GatewaySnapshot

// BuildGatewaySnapshot constructs a GatewaySnapshot for one Gateway.
func BuildGatewaySnapshot(name types.NamespacedName, routes map[gatewayv1.Listener]route.Bag) GatewaySnapshot {
	domainMapping := make(map[string]string)
	clusters := make([]ClusterConfig, 0, len(routes))
	for listener, bag := range routes {
		cluster := ClusterConfig{
			Name: toClusterName(name, listener.Name),
			Endpoints: []EndpointConfig{
				{
					Address: fmt.Sprintf("%s-%s.%s.svc.cluster.local", listener.Name, name.Name, name.Namespace),
					Port:    uint32(listener.Port),
				},
			},
		}
		clusters = append(clusters, cluster)
		for _, domain := range Domains(bag) {
			domainMapping[domain] = cluster.Name
		}
	}
	return GatewaySnapshot{
		DomainMapping: domainMapping,
		Clusters:      clusters,
	}
}

func toClusterName(gatewayName types.NamespacedName, listenerName gatewayv1.SectionName) string {
	return fmt.Sprintf("%s_%s_%s", gatewayName.Name, gatewayName.Namespace, listenerName)
}

func fromClusterName(clusterName string) types.NamespacedName {
	args := strings.Split(clusterName, "_")
	return types.NamespacedName{
		Name:      args[0],
		Namespace: args[1],
	}
}

// BuildSnapshot constructs a Snapshot for all gateways.
// It returns the snapshot and a list of Gateways that cant be applied due to a conflicting Domain setup.
func BuildSnapshot(cache GatewaySnapshotCache) (Snapshot, map[types.NamespacedName]types.NamespacedName) {
	mapping := make(map[string]string)
	clusters := make([]ClusterConfig, 0)
	conflicting := make(map[types.NamespacedName]types.NamespacedName)
	for gateway, snap := range cache {
		isFullyConflicting := true
		for domain, key := range snap.DomainMapping {
			// Check if there is a conflict for the current domain (same domain dropped twice)
			if mapping[domain] != "" && mapping[domain] != key {
				// TODO: choose the actual route, nothing else, have to restructure everything for that though
				conflict := fromClusterName(mapping[domain])
				conflicting[gateway] = conflict
				conflicting[conflict] = gateway
				continue
			}
			isFullyConflicting = false
			mapping[domain] = key
		}
		if isFullyConflicting {
			continue
		}
		clusters = append(clusters, snap.Clusters...)
	}

	return Snapshot{
		GatewaySnapshot: GatewaySnapshot{
			DomainMapping: mapping,
			Clusters:      clusters,
		},
		RejectUnknown: false,
	}, conflicting
}
