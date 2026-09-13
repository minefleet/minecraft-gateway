package network

import (
	"context"
	"time"

	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	apiv1alpha1 "minefleet.dev/minecraft-gateway/api/network/v1alpha1"
	logf "sigs.k8s.io/controller-runtime/pkg/log"
)

// MovePlayers connects players to a server, blocking until the proxies report
// the outcome. It is the imperative counterpart of a PlayerTransfer: same
// targets, same policies, but nothing is persisted and the result is returned
// to the caller.
func (s *streamServer) MovePlayers(ctx context.Context, req *apiv1alpha1.MovePlayersRequest) (*apiv1alpha1.MovePlayersResponse, error) {
	order, err := moveOrderOf(req)
	if err != nil {
		return nil, err
	}
	if s.moves == nil {
		return nil, status.Error(codes.Unavailable, "the controller is not serving moves")
	}

	log := logf.FromContext(ctx)
	results, execErr := s.moves.Execute(ctx, order)
	if execErr != nil && len(results) == 0 {
		return nil, status.Error(codes.FailedPrecondition, execErr.Error())
	}

	gateway := order.GatewayNamespace + "/" + order.GatewayName
	for _, result := range results {
		recordPlayerMove(moveSourceAPI, result)
		if result.Success {
			log.Info("moved player",
				"player", result.PlayerUUID, "server", result.ServerName,
				"gateway", gateway, "listener", order.ListenerName, "source", moveSourceAPI)
			continue
		}
		log.Info("player move failed",
			"player", result.PlayerUUID, "server", result.ServerName, "reason", result.Reason,
			"gateway", gateway, "listener", order.ListenerName, "source", moveSourceAPI)
		s.recordMoveFailure(order.GatewayNamespace, order.GatewayName, result)
	}

	if execErr != nil {
		// Players already handed to a proxy are still being moved; report what
		// is known rather than discarding it behind a bare error.
		return nil, status.Errorf(codes.DeadlineExceeded,
			"move cut short before every player reported: %v", execErr)
	}
	return &apiv1alpha1.MovePlayersResponse{Results: moveResultsOf(results)}, nil
}

// moveOrderOf validates a request and converts it to the internal order.
func moveOrderOf(req *apiv1alpha1.MovePlayersRequest) (MoveOrder, error) {
	if req.GetGatewayNamespace() == "" || req.GetGatewayName() == "" || req.GetListenerName() == "" {
		return MoveOrder{}, status.Error(codes.InvalidArgument,
			"gateway_namespace, gateway_name and listener_name are required")
	}
	if len(req.GetPlayers()) == 0 {
		return MoveOrder{}, status.Error(codes.InvalidArgument, "players must not be empty")
	}

	target, err := moveTargetOf(req)
	if err != nil {
		return MoveOrder{}, err
	}

	policy := MoveAllOrNothing
	if req.GetPolicy() == apiv1alpha1.MovePolicy_MOVE_POLICY_BEST_EFFORT {
		policy = MoveBestEffort
	}

	return MoveOrder{
		GatewayNamespace: req.GetGatewayNamespace(),
		GatewayName:      req.GetGatewayName(),
		ListenerName:     req.GetListenerName(),
		Players:          req.GetPlayers(),
		Target:           target,
		Policy:           policy,
		Wait:             time.Duration(req.GetWaitSeconds()) * time.Second,
	}, nil
}

func moveTargetOf(req *apiv1alpha1.MovePlayersRequest) (MoveTarget, error) {
	switch target := req.GetTarget().(type) {
	case *apiv1alpha1.MovePlayersRequest_ServerName:
		if target.ServerName == "" {
			return MoveTarget{}, status.Error(codes.InvalidArgument, "server_name must not be empty")
		}
		return MoveTarget{ServerName: target.ServerName}, nil

	case *apiv1alpha1.MovePlayersRequest_RouteMode:
		kind := RouteKindJoin
		if target.RouteMode == apiv1alpha1.RouteKind_ROUTE_KIND_FALLBACK {
			kind = RouteKindFallback
		}
		return MoveTarget{Route: &kind}, nil

	case *apiv1alpha1.MovePlayersRequest_LabelSelector:
		selector, err := metav1.LabelSelectorAsSelector(labelSelectorOf(target.LabelSelector))
		if err != nil {
			return MoveTarget{}, status.Errorf(codes.InvalidArgument, "invalid label_selector: %v", err)
		}
		if selector.Empty() {
			return MoveTarget{}, status.Error(codes.InvalidArgument, "label_selector must not be empty")
		}
		return MoveTarget{Selector: selector}, nil
	}

	return MoveTarget{}, status.Error(codes.InvalidArgument,
		"exactly one of route_mode, label_selector or server_name must be set")
}

func labelSelectorOf(proto *apiv1alpha1.LabelSelector) *metav1.LabelSelector {
	if proto == nil {
		return &metav1.LabelSelector{}
	}
	selector := &metav1.LabelSelector{MatchLabels: proto.GetMatchLabels()}
	for _, requirement := range proto.GetMatchExpressions() {
		selector.MatchExpressions = append(selector.MatchExpressions, metav1.LabelSelectorRequirement{
			Key:      requirement.GetKey(),
			Operator: metav1.LabelSelectorOperator(requirement.GetOperator()),
			Values:   requirement.GetValues(),
		})
	}
	return selector
}

func moveResultsOf(results []PlayerMoveResult) []*apiv1alpha1.PlayerMoveResult {
	out := make([]*apiv1alpha1.PlayerMoveResult, 0, len(results))
	for _, result := range results {
		out = append(out, &apiv1alpha1.PlayerMoveResult{
			PlayerUuid: result.PlayerUUID,
			ServerName: result.ServerName,
			Success:    result.Success,
			Reason:     result.Reason,
		})
	}
	return out
}
