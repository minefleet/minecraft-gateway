package edge

import (
	"fmt"

	"k8s.io/apimachinery/pkg/types"
	"minefleet.dev/minecraft-gateway/internal/route"
	gatewayv1 "sigs.k8s.io/gateway-api/apis/v1"
)

// ProxyConfig holds the configuration for the edge dataplane.
type ProxyConfig struct {
	// Namespace where the edge DaemonSet and ConfigMap will be created.
	Namespace string
	// Image is the edge container image (e.g., minefleet.dev/minecraft-edge:v0.0.1).
	Image string
	// XDSPort is the local port the xDS gRPC server listens on (default: 18000).
	XDSPort int
	// XDSHost is the hostname that DaemonSet pods use to reach the xDS server.
	// Typically a Kubernetes Service DNS name,
	// e.g., "minecraft-gateway-xds.namespace.svc.cluster.local".
	XDSHost string
}

// DomainSnapshot is the full routing config for one gateway sync cycle.
type DomainSnapshot struct {
	// DomainMappings maps each hostname to a cluster name.
	DomainMappings map[string]string
	// Clusters defines the upstream clusters with their endpoints.
	Clusters []ClusterConfig
	// RejectUnknown controls whether connections to unmapped domains are rejected.
	RejectUnknown bool
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

// BuildSnapshot constructs a Do

// BuildGatewaySnapshot constructs a DomainSnapshot for one Gateway.
func BuildGatewaySnapshot(name types.NamespacedName, routes map[gatewayv1.Listener]route.Bag) DomainSnapshot {
	domainMappings := make(map[string]string)
	clusters := make([]ClusterConfig, 0, len(routes))
	for listener, bag := range routes {
		cluster := ClusterConfig{
			Name: fmt.Sprintf("%s-%s", listener.Name, name.Name),
			Endpoints: []EndpointConfig{
				{
					Address: fmt.Sprintf("%s-%s.%s.cluster.svc.local:%v", listener.Name, name.Name, name.Namespace, listener.Port),
					Port:    uint32(listener.Port),
				},
			},
		}
		clusters = append(clusters, cluster)
		for _, domain := range Domains(bag) {
			domainMappings[domain] = cluster.Endpoints[0].Address
		}
	}
	return DomainSnapshot{
		DomainMappings: domainMappings,
		Clusters:       clusters,
	}
}
