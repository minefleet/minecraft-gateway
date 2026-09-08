package network

import (
	"context"
	"errors"
	"io"

	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
	apiv1alpha1 "minefleet.dev/minecraft-gateway/api/network/v1alpha1"
	logf "sigs.k8s.io/controller-runtime/pkg/log"
)

// streamServer implements the proxy-facing Connect stream and the external
// NetworkGateway query API, both backed by the same StreamManager.
type streamServer struct {
	apiv1alpha1.UnimplementedNetworkXDSServer
	apiv1alpha1.UnimplementedNetworkGatewayServer
	mgr *StreamManager
}

func newStreamServer(mgr *StreamManager) *streamServer {
	return &streamServer{mgr: mgr}
}

// Connect is the persistent bidirectional stream with one proxy. The proxy
// opens it with a hello and a full presence replay, then reports presence and
// asks for routing decisions; the controller pushes server registrations,
// routing decisions and move commands back.
func (s *streamServer) Connect(stream apiv1alpha1.NetworkXDS_ConnectServer) error {
	ctx := stream.Context()
	log := logf.FromContext(ctx)

	first, err := stream.Recv()
	if err != nil {
		return err
	}
	hello := first.GetHello()
	if hello == nil {
		return status.Error(codes.InvalidArgument, "first message must be a hello")
	}
	if hello.GetProxyId() == "" {
		return status.Error(codes.InvalidArgument, "hello must carry a proxy id")
	}

	session := newProxySession(hello)
	if replaced := s.mgr.sessions.Add(session); replaced != nil {
		// The same proxy reconnected; its old presence is about to be replayed.
		s.mgr.presence.DropProxy(replaced.ProxyID)
	}
	defer func() {
		s.mgr.sessions.Remove(session)
		session.Close()
		s.mgr.presence.DropProxy(session.ProxyID)
	}()

	log.Info("proxy connected",
		"proxy", session.ProxyID,
		"gateway", session.GatewayNamespace+"/"+session.GatewayName,
		"listener", session.ListenerName)

	// Send the current server set straight away so a proxy that connects
	// between configuration changes is not left empty.
	if cfg := s.mgr.ConfigFor(session.GatewayNamespace, session.GatewayName, session.ListenerName); cfg != nil {
		if err := session.Send(serverSyncMessage(cfg)); err != nil {
			return err
		}
	}

	writerDone := make(chan error, 1)
	go func() {
		for msg := range session.Outgoing() {
			if err := stream.Send(msg); err != nil {
				writerDone <- err
				return
			}
		}
		writerDone <- nil
	}()

	for {
		select {
		case err := <-writerDone:
			return err
		default:
		}

		msg, err := stream.Recv()
		if errors.Is(err, io.EOF) {
			log.Info("proxy disconnected", "proxy", session.ProxyID)
			return nil
		}
		if err != nil {
			return err
		}
		s.handle(ctx, session, msg)
	}
}

func (s *streamServer) handle(ctx context.Context, session *ProxySession, msg *apiv1alpha1.ProxyMessage) {
	log := logf.FromContext(ctx)

	switch m := msg.Message.(type) {
	case *apiv1alpha1.ProxyMessage_PresenceSnapshot:
		entries := make([]Presence, 0, len(m.PresenceSnapshot.GetPlayers()))
		for _, entry := range m.PresenceSnapshot.GetPlayers() {
			entries = append(entries, presenceOf(session, entry.GetPlayerUuid(), entry.GetServerName(), entry.GetContext()))
		}
		s.mgr.presence.ReplaceForProxy(session.ProxyID, entries)
		log.V(1).Info("replayed proxy presence", "proxy", session.ProxyID, "players", len(entries))

	case *apiv1alpha1.ProxyMessage_PresenceEvent:
		event := m.PresenceEvent
		if event.GetKind() == apiv1alpha1.PlayerPresenceEvent_KIND_DISCONNECTED {
			s.mgr.presence.Remove(event.GetPlayerUuid())
			return
		}
		s.mgr.presence.Set(presenceOf(session, event.GetPlayerUuid(), event.GetServerName(), event.GetContext()))
		// The player has arrived, so the capacity held for them is no longer pending.
		s.mgr.router.Release(event.GetServerName())

	case *apiv1alpha1.ProxyMessage_RouteRequest:
		s.respondToRoute(session, m.RouteRequest)

	case *apiv1alpha1.ProxyMessage_MoveResult:
		s.mgr.publishMoveResult(MoveOutcome{
			CommandID: m.MoveResult.GetCommandId(),
			Success:   m.MoveResult.GetSuccess(),
			Reason:    m.MoveResult.GetReason(),
		})

	case *apiv1alpha1.ProxyMessage_PlayerCounts:
		// Counts are derived from presence; this only surfaces drift.
		for server, reported := range m.PlayerCounts.GetCountsByServer() {
			if known := s.mgr.presence.CountForServer(server); known != int(reported) {
				log.V(1).Info("player count drift",
					"server", server, "proxyReported", reported, "controllerKnows", known)
			}
		}

	case *apiv1alpha1.ProxyMessage_Hello:
		log.V(1).Info("ignoring repeated hello", "proxy", session.ProxyID)
	}
}

func (s *streamServer) respondToRoute(session *ProxySession, req *apiv1alpha1.RouteRequest) {
	kind := RouteKindJoin
	if req.GetKind() == apiv1alpha1.RouteKind_ROUTE_KIND_FALLBACK {
		kind = RouteKindFallback
	}

	query := RouteQuery{
		PlayerUUID:        req.GetPlayerUuid(),
		Kind:              kind,
		Context:           contextOf(req.GetContext()),
		CurrentServerName: req.GetCurrentServerName(),
	}

	response := &apiv1alpha1.RouteResponse{
		CorrelationId: req.GetCorrelationId(),
		Result:        apiv1alpha1.RouteResponse_RESULT_NO_ROUTE,
	}
	if server, ok := s.mgr.ResolveRoute(session.GatewayNamespace, session.GatewayName, session.ListenerName, query); ok {
		response.Result = apiv1alpha1.RouteResponse_RESULT_OK
		response.ServerName = server.Name
	}

	if err := session.Send(&apiv1alpha1.ControllerMessage{
		Message: &apiv1alpha1.ControllerMessage_RouteResponse{RouteResponse: response},
	}); err != nil {
		session.Close()
	}
}

func presenceOf(session *ProxySession, playerUUID, serverName string, ctx *apiv1alpha1.PlayerContext) Presence {
	return Presence{
		PlayerUUID:       playerUUID,
		ProxyID:          session.ProxyID,
		ServerName:       serverName,
		GatewayNamespace: session.GatewayNamespace,
		GatewayName:      session.GatewayName,
		ListenerName:     session.ListenerName,
		Context:          contextOf(ctx),
	}
}

func contextOf(ctx *apiv1alpha1.PlayerContext) PlayerContext {
	if ctx == nil {
		return PlayerContext{}
	}
	return PlayerContext{
		ConnectedDomain: ctx.GetConnectedDomain(),
		Permissions:     ctx.GetPermissions(),
	}
}

// GetConnection reports where one player currently is.
func (s *streamServer) GetConnection(_ context.Context, req *apiv1alpha1.GetConnectionRequest) (*apiv1alpha1.GetConnectionResponse, error) {
	presence, ok := s.mgr.presence.Lookup(req.GetPlayerUuid())
	if !ok {
		return &apiv1alpha1.GetConnectionResponse{}, nil
	}
	return &apiv1alpha1.GetConnectionResponse{Connection: connectionOf(presence)}, nil
}

// GetPlayersForServer reports every player on one backend server.
func (s *streamServer) GetPlayersForServer(_ context.Context, req *apiv1alpha1.GetPlayersForServerRequest) (*apiv1alpha1.GetPlayersForServerResponse, error) {
	return &apiv1alpha1.GetPlayersForServerResponse{
		Connections: connectionsOf(s.mgr.presence.PlayersForServer(req.GetServerName())),
	}, nil
}

// GetPlayersForService reports every player across the servers of one Service.
func (s *streamServer) GetPlayersForService(_ context.Context, req *apiv1alpha1.GetPlayersForServiceRequest) (*apiv1alpha1.GetPlayersForServiceResponse, error) {
	return &apiv1alpha1.GetPlayersForServiceResponse{
		Connections: connectionsOf(s.mgr.PlayersForService(req.GetNamespace(), req.GetName())),
	}, nil
}

func connectionsOf(players []Presence) []*apiv1alpha1.PlayerConnection {
	out := make([]*apiv1alpha1.PlayerConnection, 0, len(players))
	for _, pr := range players {
		out = append(out, connectionOf(pr))
	}
	return out
}

func connectionOf(pr Presence) *apiv1alpha1.PlayerConnection {
	return &apiv1alpha1.PlayerConnection{
		PlayerUuid:       pr.PlayerUUID,
		ProxyId:          pr.ProxyID,
		ServerName:       pr.ServerName,
		GatewayNamespace: pr.GatewayNamespace,
		GatewayName:      pr.GatewayName,
		ListenerName:     pr.ListenerName,
	}
}
