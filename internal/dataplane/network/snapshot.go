package network

import "k8s.io/apimachinery/pkg/types"

type Config struct {
	// XDSPort is the local port the xDS gRPC server listens on (default: 18000).
	XDSPort int
}

type Snapshot struct {
}

type GatewaySnapshot struct {
}

type SnapshotCacheKey struct {
	GatewayName  types.NamespacedName
	ListenerName string
}

type GatewaySnapshotCache = map[SnapshotCacheKey]GatewaySnapshot
