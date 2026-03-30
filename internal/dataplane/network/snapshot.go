package network

import (
	discoveryv1 "k8s.io/api/discovery/v1"
	"k8s.io/apimachinery/pkg/types"
	"minefleet.dev/minecraft-gateway/internal/route"
	gatewayv1 "sigs.k8s.io/gateway-api/apis/v1"
)

type Config struct {
	// XDSPort is the local port the xDS gRPC server listens on (default: 18000).
	XDSPort int
}

type Snapshot struct {
}

type ListenerSnapshot struct {
}

type GatewaySnapshotCache = map[types.NamespacedName]map[string]ListenerSnapshot

func BuildListenerSnapshot(gateway types.NamespacedName, listener gatewayv1.Listener, routes route.Bag, backends []discoveryv1.EndpointSlice) ListenerSnapshot {
	return ListenerSnapshot{}
}

func BuildSnapshot(cache GatewaySnapshotCache) Snapshot {
	return Snapshot{}
}
