package edge

import (
	"fmt"
	"strings"

	"k8s.io/apimachinery/pkg/types"
	v1alpha1 "minefleet.dev/minecraft-gateway/api/controller/v1alpha1"
	"minefleet.dev/minecraft-gateway/internal/topology"
	gatewayv1 "sigs.k8s.io/gateway-api/apis/v1"
)

// Config holds the configuration for the edge dataplane.
type Config struct {
	// Namespace where the edge DaemonSet and ConfigMap will be created.
	Namespace string
	// XDSPort is the local port the xDS gRPC server listens on (default: 18000).
	XDSPort int
	// PodIP is the controller pod's IP, injected via the downward API.
	// Edge proxies connect to this address for xDS.
	PodIP string
}

// GatewaySnapshot is the full routing config for one gateway sync cycle.
type GatewaySnapshot struct {
	// DomainMapping maps each hostname to a cluster name.
	DomainMapping map[string]string
	// Clusters defines the upstream clusters with their endpoints.
	Clusters []ClusterConfig
	// RejectUnknown drops connections to unmapped domains.
	RejectUnknown bool
}

// ClusterConfig holds the Envoy cluster definition.
type ClusterConfig struct {
	Name      string
	Endpoints []EndpointConfig
	// ProxyProtocol enables PROXY protocol v2 on this upstream cluster.
	ProxyProtocol bool
}

// EndpointConfig is a single upstream address.
type EndpointConfig struct {
	Address string
	Port    uint32
}

type Snapshot struct {
	GatewaySnapshot
}

type GatewaySnapshotCache = map[types.NamespacedName]GatewaySnapshot

// BuildGatewaySnapshot constructs a GatewaySnapshot for one Gateway.
// edge may be nil, in which case defaults are used.
func BuildGatewaySnapshot(name types.NamespacedName, listeners []topology.ListenerTree, edge *v1alpha1.EdgeSpec) GatewaySnapshot {
	var proxyProtocol, rejectUnknown bool
	if edge != nil {
		proxyProtocol = edge.ProxyProtocol
		rejectUnknown = edge.RejectUnknown
	}

	domainMapping := make(map[string]string)
	clusters := make([]ClusterConfig, 0, len(listeners))
	for _, lt := range listeners {
		listenerName := lt.Listener.GetName()
		cluster := ClusterConfig{
			Name: toClusterName(name, listenerName),
			Endpoints: []EndpointConfig{
				{
					Address: fmt.Sprintf("%s-%s.%s.svc.cluster.local", listenerName, name.Name, name.Namespace),
					Port:    lt.Listener.GetPort(),
				},
			},
			ProxyProtocol: proxyProtocol,
		}
		clusters = append(clusters, cluster)
		for _, domain := range filterDomainsByListener(Domains(lt.Routes()), lt.Listener.GetHostname()) {
			domainMapping[domain] = cluster.Name
		}
	}
	return GatewaySnapshot{
		DomainMapping: domainMapping,
		Clusters:      clusters,
		RejectUnknown: rejectUnknown,
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

	rejectUnknown := false
	for _, snap := range cache {
		if snap.RejectUnknown {
			rejectUnknown = true
			break
		}
	}

	return Snapshot{
		GatewaySnapshot: GatewaySnapshot{
			DomainMapping: mapping,
			Clusters:      clusters,
			RejectUnknown: rejectUnknown,
		},
	}, conflicting
}
