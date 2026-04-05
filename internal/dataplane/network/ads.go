package network

import (
	"context"
	"fmt"
	"net"
	"sync"

	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
	mcgatewayv1alpha1 "minefleet.dev/minecraft-gateway/api/network/v1alpha1"
	"sigs.k8s.io/controller-runtime/pkg/client"
	logf "sigs.k8s.io/controller-runtime/pkg/log"
)

type networkXDSServer struct {
	mcgatewayv1alpha1.UnimplementedNetworkXDSServer
	mu       sync.RWMutex
	snapshot *Snapshot
}

func newNetworkXDSServer() *networkXDSServer {
	return &networkXDSServer{}
}

func (s *networkXDSServer) updateSnapshot(snap Snapshot) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.snapshot = &snap
}

// GetSnapshot implements NetworkXDSServer.
func (s *networkXDSServer) GetSnapshot(ctx context.Context, req *mcgatewayv1alpha1.GetSnapshotRequest) (*mcgatewayv1alpha1.GetSnapshotResponse, error) {
	s.mu.RLock()
	snap := s.snapshot
	s.mu.RUnlock()

	if snap == nil {
		return nil, status.Error(codes.Unavailable, "snapshot not yet available")
	}

	ls := snap.Get(req.GatewayNamespace, req.GatewayName, req.ListenerName)
	if ls == nil {
		return nil, status.Errorf(codes.NotFound, "no snapshot for %s/%s listener %s",
			req.GatewayNamespace, req.GatewayName, req.ListenerName)
	}

	return &mcgatewayv1alpha1.GetSnapshotResponse{
		Snapshot: &mcgatewayv1alpha1.Snapshot{
			GatewayName:       ls.GatewayName,
			ListenerName:      ls.ListenerName,
			CurrentGeneration: snap.Generation,
			Services:          ls.Services,
		},
	}, nil
}

func (s *networkXDSServer) start(ctx context.Context, port int) error {
	lis, err := net.Listen("tcp", fmt.Sprintf(":%d", port))
	if err != nil {
		return fmt.Errorf("network xds listen :%d: %w", port, err)
	}
	grpcServer := grpc.NewServer()
	mcgatewayv1alpha1.RegisterNetworkXDSServer(grpcServer, s)

	go func() {
		<-ctx.Done()
		grpcServer.GracefulStop()
	}()

	return grpcServer.Serve(lis)
}

// StartADS starts the network xDS gRPC server and a goroutine that applies incoming snapshots.
func StartADS(ctx context.Context, snapshots <-chan Snapshot, cfg Config, _ client.Client) {
	log := logf.FromContext(ctx)
	srv := newNetworkXDSServer()

	go func() {
		if err := srv.start(ctx, cfg.XDSPort); err != nil && ctx.Err() == nil {
			log.Error(err, "network xDS server stopped unexpectedly")
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
				srv.updateSnapshot(snap)
				log.V(1).Info("updated network xDS snapshot", "generation", snap.Generation)
			}
		}
	}()
}
