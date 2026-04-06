package edge

import (
	"context"
	"fmt"
	"net"
	"sync/atomic"

	discoverygrpc "github.com/envoyproxy/go-control-plane/envoy/service/discovery/v3"
	cachetypes "github.com/envoyproxy/go-control-plane/pkg/cache/types"
	cachev3 "github.com/envoyproxy/go-control-plane/pkg/cache/v3"
	resourcev3 "github.com/envoyproxy/go-control-plane/pkg/resource/v3"
	serverv3 "github.com/envoyproxy/go-control-plane/pkg/server/v3"
	"google.golang.org/grpc"
	logf "sigs.k8s.io/controller-runtime/pkg/log"
)

type xdsServer struct {
	cache  cachev3.SnapshotCache
	server serverv3.Server
	ver    atomic.Int64
}

func newXDSServer(ctx context.Context) *xdsServer {
	c := cachev3.NewSnapshotCache(true, cachev3.IDHash{}, nil)
	return &xdsServer{
		cache:  c,
		server: serverv3.NewServer(ctx, c, nil),
	}
}

// Start runs the xDS gRPC server on port, blocking until ctx is cancelled.
func (x *xdsServer) Start(ctx context.Context, port int) error {
	lis, err := net.Listen("tcp", fmt.Sprintf(":%d", port))
	if err != nil {
		return fmt.Errorf("xds listen :%d: %w", port, err)
	}
	grpcServer := grpc.NewServer()
	discoverygrpc.RegisterAggregatedDiscoveryServiceServer(grpcServer, x.server)

	go func() {
		<-ctx.Done()
		grpcServer.GracefulStop()
	}()

	return grpcServer.Serve(lis)
}

// UpdateSnapshot builds and pushes a new LDS+CDS snapshot for all edge nodes.
func (x *xdsServer) UpdateSnapshot(ctx context.Context, snap Snapshot) error {
	log := logf.FromContext(ctx)
	v := x.ver.Add(1)

	listener, err := buildListenerResources(snap)
	if err != nil {
		return fmt.Errorf("build listener: %w", err)
	}

	clusters, err := buildClusterResources(snap)
	if err != nil {
		return fmt.Errorf("build clusters: %w", err)
	}
	clusterResources := make([]cachetypes.Resource, len(clusters))
	for i, c := range clusters {
		clusterResources[i] = c
	}

	snapshot, err := cachev3.NewSnapshot(
		fmt.Sprintf("%d", v),
		map[resourcev3.Type][]cachetypes.Resource{
			resourcev3.ListenerType: {listener},
			resourcev3.ClusterType:  clusterResources,
		},
	)
	if err != nil {
		return fmt.Errorf("new xds snapshot: %w", err)
	}
	log.Info("updated edge xDS snapshot", "clusters", len(snapshot.GetResources(resourcev3.ClusterType)), "mapping", snap.DomainMapping)
	return x.cache.SetSnapshot(ctx, NodeID, snapshot)
}
