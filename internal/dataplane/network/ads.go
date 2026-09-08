package network

import (
	"context"
	"fmt"
	"net"

	"google.golang.org/grpc"
	apiv1alpha1 "minefleet.dev/minecraft-gateway/api/network/v1alpha1"
	"sigs.k8s.io/controller-runtime/pkg/client"
	logf "sigs.k8s.io/controller-runtime/pkg/log"
)

// serve runs the gRPC server carrying both the proxy stream and the external
// player query API.
func serve(ctx context.Context, srv *streamServer, port int) error {
	lis, err := net.Listen("tcp", fmt.Sprintf(":%d", port))
	if err != nil {
		return fmt.Errorf("network xds listen :%d: %w", port, err)
	}
	grpcServer := grpc.NewServer()
	apiv1alpha1.RegisterNetworkXDSServer(grpcServer, srv)
	apiv1alpha1.RegisterNetworkGatewayServer(grpcServer, srv)

	go func() {
		<-ctx.Done()
		grpcServer.GracefulStop()
	}()

	return grpcServer.Serve(lis)
}

// StartADS starts the network gRPC server and a goroutine that applies incoming
// routing configuration, pushing it to the connected proxies.
func StartADS(ctx context.Context, snapshots <-chan Snapshot, cfg Config, _ client.Client) {
	log := logf.FromContext(ctx)
	mgr := cfg.Streams
	if mgr == nil {
		mgr = NewStreamManager()
	}
	srv := newStreamServer(mgr)

	go func() {
		if err := serve(ctx, srv, cfg.XDSPort); err != nil && ctx.Err() == nil {
			log.Error(err, "network gRPC server stopped unexpectedly")
		}
	}()

	go func() {
		for {
			select {
			case <-ctx.Done():
				return
			case snap, ok := <-snapshots:
				if !ok {
					return
				}
				mgr.UpdateSnapshot(snap)
				log.V(1).Info("updated network routing configuration", "generation", snap.Generation)
			}
		}
	}()
}
