/*
Copyright 2025.

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

package controller

import (
	"context"
	"fmt"
	"strings"
	"sync"
	"time"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	mcgatewayv1alpha1 "minefleet.dev/minecraft-gateway/api/controller/v1alpha1"
	networkdp "minefleet.dev/minecraft-gateway/internal/dataplane/network"
	"minefleet.dev/minecraft-gateway/internal/index"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/event"
	"sigs.k8s.io/controller-runtime/pkg/handler"
	logf "sigs.k8s.io/controller-runtime/pkg/log"
	"sigs.k8s.io/controller-runtime/pkg/source"
)

// moveGracePeriod is how long a dispatched move may go unanswered before it is
// sent again. A proxy may have died between receiving the command and reporting
// the result; reconnecting to a server the player already reached is harmless.
const moveGracePeriod = 15 * time.Second

// PlayerStreams is the part of the proxy stream state the reconciler needs. It
// is narrow so tests can substitute a scripted implementation.
type PlayerStreams interface {
	// Lookup reports where a player currently is.
	Lookup(playerUUID string) (networkdp.Presence, bool)
	// SendMove asks the proxy holding a player to move them.
	SendMove(proxyID string, req networkdp.MoveRequest) error
	// ResolveRoute picks a server for a player from the listener's routes.
	ResolveRoute(namespace, name, listener string, q networkdp.RouteQuery) (*networkdp.Server, bool)
	// ResolveSelector picks one server from the services matching a selector.
	ResolveSelector(namespace, name, listener string, matches func(*networkdp.Service) bool) (*networkdp.Server, bool)
	// OnMoveResult registers a callback for proxy move reports.
	OnMoveResult(fn func(networkdp.MoveOutcome))
	// Presence exposes the global presence map, for pre-staging callbacks.
	Presence() *networkdp.PresenceMap
}

// PlayerTransferReconciler reconciles a PlayerTransfer object.
//
// A transfer is a one-shot command: it waits for its players to be present on
// the addressed gateway listener, moves them, and settles in a terminal phase.
// Because the spec is immutable, the reconciler never has to unwind a transfer
// that changed target mid-flight.
type PlayerTransferReconciler struct {
	client.Client
	Scheme  *runtime.Scheme
	Streams PlayerStreams

	// events wakes a transfer when one of its players connects or a move is
	// reported, so pre-staged transfers fire the instant a player appears.
	events chan event.TypedGenericEvent[client.Object]
	// moves records when each move command was dispatched, to re-send one whose
	// result never arrived.
	moves sync.Map // commandID → time.Time
}

// +kubebuilder:rbac:groups=gateway.networking.minefleet.dev,resources=playertransfers,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups=gateway.networking.minefleet.dev,resources=playertransfers/status,verbs=get;update;patch

func (r *PlayerTransferReconciler) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
	log := logf.FromContext(ctx)

	var transfer mcgatewayv1alpha1.PlayerTransfer
	if err := r.Get(ctx, req.NamespacedName, &transfer); err != nil {
		return ctrl.Result{}, client.IgnoreNotFound(err)
	}

	if isTerminal(transfer.Status.Phase) {
		return ctrl.Result{}, nil
	}
	if r.Streams == nil {
		return ctrl.Result{}, nil
	}

	before := transfer.DeepCopy()
	if transfer.Status.Phase == "" {
		transfer.Status.Phase = mcgatewayv1alpha1.PendingPlayerTransferPhase
	}
	ensureAssignments(&transfer)

	expired, requeueAfter := deadlineOf(&transfer)

	switch transfer.Spec.Policy {
	case mcgatewayv1alpha1.BestEffortPlayerTransferPolicy:
		r.reconcileBestEffort(ctx, &transfer, expired)
	default:
		r.reconcileAllOrNothing(ctx, &transfer, expired)
	}

	if err := r.patchStatus(ctx, before, &transfer); err != nil {
		return ctrl.Result{}, err
	}

	if isTerminal(transfer.Status.Phase) {
		log.Info("player transfer settled",
			"transfer", req.NamespacedName, "phase", transfer.Status.Phase)
		return ctrl.Result{}, nil
	}
	// Re-check dispatched moves whose result never came back.
	if requeueAfter <= 0 || requeueAfter > moveGracePeriod {
		if hasPhase(&transfer, mcgatewayv1alpha1.MovingPlayerAssignmentPhase) {
			requeueAfter = moveGracePeriod
		}
	}
	return ctrl.Result{RequeueAfter: requeueAfter}, nil
}

// reconcileAllOrNothing waits until every player is simultaneously present
// before moving any of them. Absent players are never a failure: they may still
// connect, and only the TTL ends the wait.
func (r *PlayerTransferReconciler) reconcileAllOrNothing(ctx context.Context, transfer *mcgatewayv1alpha1.PlayerTransfer, expired bool) {
	if dispatched := hasPhase(transfer, mcgatewayv1alpha1.MovingPlayerAssignmentPhase); dispatched {
		// Already committed: see the moves through regardless of the TTL.
		r.redispatchStaleMoves(ctx, transfer)
		finalizeIfSettled(transfer, false)
		return
	}

	presences := make(map[string]networkdp.Presence, len(transfer.Spec.Players))
	for _, player := range transfer.Spec.Players {
		presence, ok := r.presenceFor(transfer, string(player))
		if !ok {
			if expired {
				transfer.Status.Phase = mcgatewayv1alpha1.ExpiredPlayerTransferPhase
				failRemaining(transfer, mcgatewayv1alpha1.PlayerAssignmentReasonNotConnected)
			}
			return
		}
		presences[string(player)] = presence
	}

	r.dispatch(ctx, transfer, presences)
	finalizeIfSettled(transfer, false)
}

// reconcileBestEffort moves whichever players are present now and fails the
// rest individually once the TTL runs out.
func (r *PlayerTransferReconciler) reconcileBestEffort(ctx context.Context, transfer *mcgatewayv1alpha1.PlayerTransfer, expired bool) {
	presences := make(map[string]networkdp.Presence)
	for i := range transfer.Status.Assignments {
		assignment := &transfer.Status.Assignments[i]
		if assignment.Phase != mcgatewayv1alpha1.PendingPlayerAssignmentPhase {
			continue
		}
		presence, ok := r.presenceFor(transfer, string(assignment.PlayerUUID))
		if !ok {
			if expired {
				assignment.Phase = mcgatewayv1alpha1.FailedPlayerAssignmentPhase
				assignment.Reason = mcgatewayv1alpha1.PlayerAssignmentReasonNotConnected
			}
			continue
		}
		presences[string(assignment.PlayerUUID)] = presence
	}

	r.dispatch(ctx, transfer, presences)
	r.redispatchStaleMoves(ctx, transfer)
	finalizeIfSettled(transfer, true)
}

// dispatch resolves the target and sends a move command for each present player.
func (r *PlayerTransferReconciler) dispatch(ctx context.Context, transfer *mcgatewayv1alpha1.PlayerTransfer, presences map[string]networkdp.Presence) {
	if len(presences) == 0 {
		return
	}

	// A selector target resolves once: every player goes to the same server.
	var shared *networkdp.Server
	if transfer.Spec.Target.Selector != nil {
		server, ok := r.resolveSelector(transfer)
		if !ok {
			failAll(transfer, presences, mcgatewayv1alpha1.PlayerAssignmentReasonNoMatchingServer)
			return
		}
		shared = server
	}

	for i := range transfer.Status.Assignments {
		assignment := &transfer.Status.Assignments[i]
		if assignment.Phase != mcgatewayv1alpha1.PendingPlayerAssignmentPhase {
			continue
		}
		presence, ok := presences[string(assignment.PlayerUUID)]
		if !ok {
			continue
		}

		serverName, reason := r.targetFor(transfer, presence, shared)
		if serverName == "" {
			assignment.Phase = mcgatewayv1alpha1.FailedPlayerAssignmentPhase
			assignment.Reason = reason
			continue
		}

		commandID := commandIDFor(transfer, string(assignment.PlayerUUID))
		err := r.Streams.SendMove(presence.ProxyID, networkdp.MoveRequest{
			CommandID:  commandID,
			PlayerUUID: string(assignment.PlayerUUID),
			ServerName: serverName,
		})
		if err != nil {
			logf.FromContext(ctx).Error(err, "failed to dispatch player move",
				"player", assignment.PlayerUUID, "proxy", presence.ProxyID)
			assignment.Phase = mcgatewayv1alpha1.FailedPlayerAssignmentPhase
			assignment.Reason = mcgatewayv1alpha1.PlayerAssignmentReasonProxyUnavailable
			continue
		}

		assignment.Phase = mcgatewayv1alpha1.MovingPlayerAssignmentPhase
		assignment.AssignedServer = serverName
		r.moves.Store(commandID, time.Now())
	}
}

// targetFor resolves the destination server for one player.
func (r *PlayerTransferReconciler) targetFor(transfer *mcgatewayv1alpha1.PlayerTransfer, presence networkdp.Presence, shared *networkdp.Server) (string, string) {
	target := transfer.Spec.Target

	switch {
	case target.ServerName != nil:
		// The caller already picked the server; the proxy resolves it by name.
		return *target.ServerName, ""

	case shared != nil:
		return shared.Name, ""

	case target.Route != nil:
		kind := networkdp.RouteKindJoin
		if *target.Route == mcgatewayv1alpha1.FallbackPlayerTransferRouteMode {
			kind = networkdp.RouteKindFallback
		}
		server, ok := r.Streams.ResolveRoute(
			transfer.Namespace, transfer.Spec.GatewayName, transfer.Spec.ListenerName,
			networkdp.RouteQuery{
				PlayerUUID:        presence.PlayerUUID,
				Kind:              kind,
				Context:           presence.Context,
				CurrentServerName: presence.ServerName,
			})
		if !ok {
			return "", mcgatewayv1alpha1.PlayerAssignmentReasonNoMatchingRoute
		}
		return server.Name, ""
	}

	return "", mcgatewayv1alpha1.PlayerAssignmentReasonNoMatchingServer
}

func (r *PlayerTransferReconciler) resolveSelector(transfer *mcgatewayv1alpha1.PlayerTransfer) (*networkdp.Server, bool) {
	selector, err := metav1.LabelSelectorAsSelector(transfer.Spec.Target.Selector)
	if err != nil || selector.Empty() {
		return nil, false
	}
	return r.Streams.ResolveSelector(
		transfer.Namespace, transfer.Spec.GatewayName, transfer.Spec.ListenerName,
		func(svc *networkdp.Service) bool {
			return selector.Matches(labels.Set(svc.Labels))
		})
}

// presenceFor returns a player's presence only if they are on the gateway
// listener this transfer addresses.
func (r *PlayerTransferReconciler) presenceFor(transfer *mcgatewayv1alpha1.PlayerTransfer, playerUUID string) (networkdp.Presence, bool) {
	presence, ok := r.Streams.Lookup(playerUUID)
	if !ok {
		return networkdp.Presence{}, false
	}
	if presence.GatewayNamespace != transfer.Namespace ||
		presence.GatewayName != transfer.Spec.GatewayName ||
		presence.ListenerName != transfer.Spec.ListenerName {
		return networkdp.Presence{}, false
	}
	return presence, true
}

// redispatchStaleMoves re-sends a move whose result never arrived.
func (r *PlayerTransferReconciler) redispatchStaleMoves(ctx context.Context, transfer *mcgatewayv1alpha1.PlayerTransfer) {
	for i := range transfer.Status.Assignments {
		assignment := &transfer.Status.Assignments[i]
		if assignment.Phase != mcgatewayv1alpha1.MovingPlayerAssignmentPhase {
			continue
		}
		commandID := commandIDFor(transfer, string(assignment.PlayerUUID))
		// A missing record means this controller never sent the command itself:
		// leadership moved, or it restarted mid-transfer. Re-send rather than
		// leave the transfer waiting on a result that will never arrive.
		if sentAt, ok := r.moves.Load(commandID); ok && time.Since(sentAt.(time.Time)) < moveGracePeriod {
			continue
		}
		presence, present := r.presenceFor(transfer, string(assignment.PlayerUUID))
		if !present {
			continue
		}
		if err := r.Streams.SendMove(presence.ProxyID, networkdp.MoveRequest{
			CommandID:  commandID,
			PlayerUUID: string(assignment.PlayerUUID),
			ServerName: assignment.AssignedServer,
		}); err != nil {
			logf.FromContext(ctx).V(1).Info("re-dispatch failed",
				"player", assignment.PlayerUUID, "error", err)
			continue
		}
		r.moves.Store(commandID, time.Now())
	}
}

// applyMoveResult records a proxy's report against the originating transfer.
func (r *PlayerTransferReconciler) applyMoveResult(ctx context.Context, outcome networkdp.MoveOutcome) {
	namespace, name, playerUUID, ok := parseCommandID(outcome.CommandID)
	if !ok {
		return
	}
	r.moves.Delete(outcome.CommandID)

	var transfer mcgatewayv1alpha1.PlayerTransfer
	key := types.NamespacedName{Namespace: namespace, Name: name}
	if err := r.Get(ctx, key, &transfer); err != nil {
		return
	}
	if isTerminal(transfer.Status.Phase) {
		return
	}

	before := transfer.DeepCopy()
	for i := range transfer.Status.Assignments {
		assignment := &transfer.Status.Assignments[i]
		if string(assignment.PlayerUUID) != playerUUID {
			continue
		}
		if outcome.Success {
			assignment.Phase = mcgatewayv1alpha1.CompletedPlayerAssignmentPhase
			assignment.Reason = ""
		} else {
			assignment.Phase = mcgatewayv1alpha1.FailedPlayerAssignmentPhase
			assignment.Reason = outcome.Reason
			if assignment.Reason == "" {
				assignment.Reason = mcgatewayv1alpha1.PlayerAssignmentReasonMoveFailed
			}
		}
	}

	partial := transfer.Spec.Policy == mcgatewayv1alpha1.BestEffortPlayerTransferPolicy
	finalizeIfSettled(&transfer, partial)

	if err := r.patchStatus(ctx, before, &transfer); err != nil {
		logf.FromContext(ctx).Error(err, "failed to record move result", "transfer", key)
	}
}

func (r *PlayerTransferReconciler) patchStatus(ctx context.Context, before, after *mcgatewayv1alpha1.PlayerTransfer) error {
	if equalStatus(before.Status, after.Status) {
		return nil
	}
	return r.Status().Patch(ctx, after, client.MergeFrom(before))
}

// SetupWithManager sets up the controller with the Manager.
func (r *PlayerTransferReconciler) SetupWithManager(mgr ctrl.Manager) error {
	if err := mgr.GetFieldIndexer().IndexField(context.Background(),
		&mcgatewayv1alpha1.PlayerTransfer{}, index.PlayerTransferByPlayer,
		func(obj client.Object) []string {
			transfer, ok := obj.(*mcgatewayv1alpha1.PlayerTransfer)
			if !ok {
				return nil
			}
			players := make([]string, 0, len(transfer.Spec.Players))
			for _, player := range transfer.Spec.Players {
				players = append(players, string(player))
			}
			return players
		}); err != nil {
		return err
	}

	r.events = make(chan event.TypedGenericEvent[client.Object], 128)

	if r.Streams != nil {
		// A pre-staged transfer fires the moment its players appear, so a match
		// can be staged before anyone has joined.
		r.Streams.Presence().OnConnect(func(presence networkdp.Presence) {
			r.enqueueForPlayer(context.Background(), presence.PlayerUUID)
		})
		r.Streams.OnMoveResult(func(outcome networkdp.MoveOutcome) {
			r.applyMoveResult(context.Background(), outcome)
		})
	}

	return ctrl.NewControllerManagedBy(mgr).
		For(&mcgatewayv1alpha1.PlayerTransfer{}).
		WatchesRawSource(source.Channel(r.events, &handler.EnqueueRequestForObject{})).
		Named("playertransfer").
		Complete(r)
}

// enqueueForPlayer wakes every non-terminal transfer waiting on a player.
func (r *PlayerTransferReconciler) enqueueForPlayer(ctx context.Context, playerUUID string) {
	var transfers mcgatewayv1alpha1.PlayerTransferList
	if err := r.List(ctx, &transfers,
		client.MatchingFields{index.PlayerTransferByPlayer: playerUUID}); err != nil {
		return
	}
	for i := range transfers.Items {
		transfer := &transfers.Items[i]
		if isTerminal(transfer.Status.Phase) {
			continue
		}
		select {
		case r.events <- event.TypedGenericEvent[client.Object]{Object: transfer}:
		default:
			// The queue is saturated; the periodic requeue will pick this up.
		}
	}
}

func ensureAssignments(transfer *mcgatewayv1alpha1.PlayerTransfer) {
	known := make(map[string]struct{}, len(transfer.Status.Assignments))
	for _, assignment := range transfer.Status.Assignments {
		known[string(assignment.PlayerUUID)] = struct{}{}
	}
	for _, player := range transfer.Spec.Players {
		if _, ok := known[string(player)]; ok {
			continue
		}
		transfer.Status.Assignments = append(transfer.Status.Assignments,
			mcgatewayv1alpha1.PlayerTransferAssignment{
				PlayerUUID: player,
				Phase:      mcgatewayv1alpha1.PendingPlayerAssignmentPhase,
			})
	}
}

// deadlineOf reports whether the TTL has elapsed, and how long remains.
func deadlineOf(transfer *mcgatewayv1alpha1.PlayerTransfer) (expired bool, remaining time.Duration) {
	if transfer.Spec.TTL == nil {
		return false, 0
	}
	deadline := transfer.CreationTimestamp.Add(transfer.Spec.TTL.Duration)
	remaining = time.Until(deadline)
	if remaining <= 0 {
		return true, 0
	}
	return false, remaining
}

// finalizeIfSettled sets the aggregate phase once no assignment is outstanding.
func finalizeIfSettled(transfer *mcgatewayv1alpha1.PlayerTransfer, allowPartial bool) {
	var completed, failed int
	for _, assignment := range transfer.Status.Assignments {
		switch assignment.Phase {
		case mcgatewayv1alpha1.CompletedPlayerAssignmentPhase:
			completed++
		case mcgatewayv1alpha1.FailedPlayerAssignmentPhase:
			failed++
		default:
			return // still outstanding
		}
	}

	switch {
	case failed == 0:
		transfer.Status.Phase = mcgatewayv1alpha1.CompletedPlayerTransferPhase
	case completed == 0:
		transfer.Status.Phase = mcgatewayv1alpha1.FailedPlayerTransferPhase
	case allowPartial:
		transfer.Status.Phase = mcgatewayv1alpha1.PartiallyCompletedPlayerTransferPhase
	default:
		transfer.Status.Phase = mcgatewayv1alpha1.FailedPlayerTransferPhase
	}
}

func failAll(transfer *mcgatewayv1alpha1.PlayerTransfer, only map[string]networkdp.Presence, reason string) {
	for i := range transfer.Status.Assignments {
		assignment := &transfer.Status.Assignments[i]
		if assignment.Phase != mcgatewayv1alpha1.PendingPlayerAssignmentPhase {
			continue
		}
		if _, ok := only[string(assignment.PlayerUUID)]; !ok {
			continue
		}
		assignment.Phase = mcgatewayv1alpha1.FailedPlayerAssignmentPhase
		assignment.Reason = reason
	}
}

func failRemaining(transfer *mcgatewayv1alpha1.PlayerTransfer, reason string) {
	for i := range transfer.Status.Assignments {
		assignment := &transfer.Status.Assignments[i]
		if assignment.Phase == mcgatewayv1alpha1.PendingPlayerAssignmentPhase {
			assignment.Phase = mcgatewayv1alpha1.FailedPlayerAssignmentPhase
			assignment.Reason = reason
		}
	}
}

func hasPhase(transfer *mcgatewayv1alpha1.PlayerTransfer, phase mcgatewayv1alpha1.PlayerAssignmentPhase) bool {
	for _, assignment := range transfer.Status.Assignments {
		if assignment.Phase == phase {
			return true
		}
	}
	return false
}

func isTerminal(phase mcgatewayv1alpha1.PlayerTransferPhase) bool {
	switch phase {
	case mcgatewayv1alpha1.CompletedPlayerTransferPhase,
		mcgatewayv1alpha1.PartiallyCompletedPlayerTransferPhase,
		mcgatewayv1alpha1.FailedPlayerTransferPhase,
		mcgatewayv1alpha1.ExpiredPlayerTransferPhase:
		return true
	}
	return false
}

func equalStatus(a, b mcgatewayv1alpha1.PlayerTransferStatus) bool {
	if a.Phase != b.Phase || len(a.Assignments) != len(b.Assignments) {
		return false
	}
	for i := range a.Assignments {
		if a.Assignments[i] != b.Assignments[i] {
			return false
		}
	}
	return true
}

// commandIDFor encodes the transfer and player a move belongs to, so a result
// maps back without the controller keeping per-command state.
func commandIDFor(transfer *mcgatewayv1alpha1.PlayerTransfer, playerUUID string) string {
	return fmt.Sprintf("%s/%s#%s", transfer.Namespace, transfer.Name, playerUUID)
}

func parseCommandID(commandID string) (namespace, name, playerUUID string, ok bool) {
	resource, playerUUID, ok := strings.Cut(commandID, "#")
	if !ok {
		return "", "", "", false
	}
	namespace, name, ok = strings.Cut(resource, "/")
	if !ok {
		return "", "", "", false
	}
	return namespace, name, playerUUID, true
}
