package network

import (
	"context"
	"fmt"
	"net"
	"sync"

	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
	apiv1alpha1 "minefleet.dev/minecraft-gateway/api/network/v1alpha1"
	"sigs.k8s.io/controller-runtime/pkg/client"
	logf "sigs.k8s.io/controller-runtime/pkg/log"
)

type networkXDSServer struct {
	apiv1alpha1.UnimplementedNetworkXDSServer
	mu       sync.RWMutex
	snapshot *Snapshot
	stream   *streamServer
}

func newNetworkXDSServer(mgr *StreamManager) *networkXDSServer {
	return &networkXDSServer{stream: newStreamServer(mgr)}
}

func (s *networkXDSServer) updateSnapshot(snap Snapshot) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.snapshot = &snap
}

// GetSnapshot implements NetworkXDSServer.
//
// Deprecated: superseded by Connect. Proxies migrating to the stream stop
// calling this, and it is removed once no supported proxy polls.
func (s *networkXDSServer) GetSnapshot(ctx context.Context, req *apiv1alpha1.GetSnapshotRequest) (*apiv1alpha1.GetSnapshotResponse, error) {
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

	return &apiv1alpha1.GetSnapshotResponse{
		Snapshot: &apiv1alpha1.Snapshot{
			GatewayName:       ls.GatewayName,
			ListenerName:      ls.ListenerName,
			CurrentGeneration: snap.Generation,
			Services:          servicesToProto(ls.Services),
		},
	}, nil
}

// Connect delegates the proxy stream to the stream server.
func (s *networkXDSServer) Connect(stream apiv1alpha1.NetworkXDS_ConnectServer) error {
	return s.stream.Connect(stream)
}

func servicesToProto(services []*Service) []*apiv1alpha1.ManagedService {
	out := make([]*apiv1alpha1.ManagedService, 0, len(services))
	for _, svc := range services {
		servers := make([]*apiv1alpha1.ManagedServer, 0, len(svc.Servers))
		for _, s := range svc.Servers {
			servers = append(servers, &apiv1alpha1.ManagedServer{
				UniqueId:       s.UniqueID,
				Name:           s.Name,
				Ip:             s.IP,
				Port:           s.Port,
				MaxPlayers:     s.MaxPlayers,
				CurrentPlayers: s.CurrentPlayers,
			})
		}
		routes := make([]*apiv1alpha1.Route, 0, len(svc.JoinRoutes)+len(svc.FallbackRoutes))
		for _, r := range svc.JoinRoutes {
			routes = append(routes, &apiv1alpha1.Route{Priority: r.Priority, IsJoin: true, Rules: ruleSetsToProto(r.RuleSets)})
		}
		for _, r := range svc.FallbackRoutes {
			routes = append(routes, &apiv1alpha1.Route{Priority: r.Priority, IsFallback: true, Rules: ruleSetsToProto(r.RuleSets)})
		}
		out = append(out, &apiv1alpha1.ManagedService{
			NamespacedName:       svc.NamespacedName,
			Namespace:            svc.Namespace,
			Name:                 svc.Name,
			DistributionStrategy: distributionToProto(svc.DistributionStrategy),
			Servers:              servers,
			Routes:               routes,
		})
	}
	return out
}

func ruleSetsToProto(ruleSets []RuleSet) []*apiv1alpha1.OptionRuleSet {
	out := make([]*apiv1alpha1.OptionRuleSet, 0, len(ruleSets))
	for _, rs := range ruleSets {
		rules := make([]*apiv1alpha1.Rule, 0, len(rs.Rules))
		for _, r := range rs.Rules {
			rule := &apiv1alpha1.Rule{}
			if r.Domain != "" {
				rule.Domain = &r.Domain
			}
			if r.Permission != "" {
				rule.Permission = &r.Permission
			}
			if r.FallbackFor != "" {
				rule.FallbackFor = &r.FallbackFor
			}
			rules = append(rules, rule)
		}
		out = append(out, &apiv1alpha1.OptionRuleSet{Type: ruleTypeToProto(rs.Type), Rules: rules})
	}
	return out
}

func ruleTypeToProto(t RuleType) apiv1alpha1.RuleType {
	switch t {
	case RuleTypeAny:
		return apiv1alpha1.RuleType_ANY
	case RuleTypeNone:
		return apiv1alpha1.RuleType_NONE
	default:
		return apiv1alpha1.RuleType_ALL
	}
}

func distributionToProto(s DistributionStrategy) apiv1alpha1.DistributionStrategy {
	if s == DistributionLeastPlayers {
		return apiv1alpha1.DistributionStrategy_LEAST_PLAYERS
	}
	return apiv1alpha1.DistributionStrategy_RANDOM
}

func (s *networkXDSServer) start(ctx context.Context, port int) error {
	lis, err := net.Listen("tcp", fmt.Sprintf(":%d", port))
	if err != nil {
		return fmt.Errorf("network xds listen :%d: %w", port, err)
	}
	grpcServer := grpc.NewServer()
	apiv1alpha1.RegisterNetworkXDSServer(grpcServer, s)
	apiv1alpha1.RegisterNetworkGatewayServer(grpcServer, s.stream)

	go func() {
		<-ctx.Done()
		grpcServer.GracefulStop()
	}()

	return grpcServer.Serve(lis)
}

// StartADS starts the network xDS gRPC server and a goroutine that applies incoming snapshots.
func StartADS(ctx context.Context, snapshots <-chan Snapshot, cfg Config, _ client.Client) {
	log := logf.FromContext(ctx)
	mgr := cfg.Streams
	if mgr == nil {
		mgr = NewStreamManager()
	}
	srv := newNetworkXDSServer(mgr)

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
				mgr.UpdateSnapshot(snap)
				log.V(1).Info("updated network xDS snapshot", "generation", snap.Generation)
			}
		}
	}()
}
