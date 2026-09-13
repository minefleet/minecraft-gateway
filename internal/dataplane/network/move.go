package network

import (
	"context"
	"fmt"
	"sync"
	"time"

	"github.com/google/uuid"
	mcgatewayv1alpha1 "minefleet.dev/minecraft-gateway/api/controller/v1alpha1"
)

// apiMoveCommandPrefix marks a move as belonging to a MovePlayers call rather
// than a PlayerTransfer. The reconciler's command ids are "<ns>/<name>#<uuid>",
// which this can never collide with because it contains no slash — so each side
// ignores the other's results.
const apiMoveCommandPrefix = "api:"

// MovePolicy controls how players who are not connected are handled.
type MovePolicy int

const (
	// MoveAllOrNothing dispatches nobody until every player is present.
	MoveAllOrNothing MovePolicy = iota
	// MoveBestEffort dispatches each player as they become present and fails the
	// rest individually.
	MoveBestEffort
)

// MoveOrder is one batch of players to connect to a server.
type MoveOrder struct {
	GatewayNamespace string
	GatewayName      string
	ListenerName     string
	Players          []string
	Target           MoveTarget
	Policy           MovePolicy
	// Wait bounds how long to wait for players who are not connected yet. Zero
	// fails them immediately.
	Wait time.Duration
}

// PlayerMoveResult is the terminal outcome for one player of a batch.
type PlayerMoveResult struct {
	PlayerUUID string
	ServerName string
	Success    bool
	Reason     string
}

// MoveEngine runs MovePlayers batches against the connected proxies. It is the
// imperative sibling of the PlayerTransfer reconciler: same targets, same
// policies, but the outcome is returned to the caller instead of written to a
// resource, so nothing is persisted.
//
// One engine serves every in-flight batch. It subscribes once to proxy move
// results and to player arrivals, and fans them out to the waiting batches,
// because the underlying callbacks cannot be unregistered.
type MoveEngine struct {
	mgr *StreamManager

	mu sync.Mutex
	// results routes a proxy's report back to the batch that dispatched it.
	results map[string]chan MoveOutcome
	// arrivals wakes the batches waiting on a player who has just connected.
	arrivals map[string]map[chan struct{}]struct{}
}

// NewMoveEngine wires an engine to the proxy streams. It must be constructed
// once per StreamManager.
func NewMoveEngine(mgr *StreamManager) *MoveEngine {
	e := &MoveEngine{
		mgr:      mgr,
		results:  make(map[string]chan MoveOutcome),
		arrivals: make(map[string]map[chan struct{}]struct{}),
	}
	mgr.OnMoveResult(e.deliver)
	mgr.Presence().OnConnect(func(presence Presence) {
		e.wake(presence.PlayerUUID)
	})
	return e
}

// Execute carries out one batch, blocking until every player has a terminal
// outcome or ctx is done. Moves already handed to a proxy are carried out
// regardless of ctx, so a cancelled call may still have moved players.
func (e *MoveEngine) Execute(ctx context.Context, order MoveOrder) ([]PlayerMoveResult, error) {
	if len(order.Players) == 0 {
		return nil, fmt.Errorf("no players to move")
	}
	if e.mgr.ConfigFor(order.GatewayNamespace, order.GatewayName, order.ListenerName) == nil {
		return nil, fmt.Errorf("gateway %s/%s has no listener %q",
			order.GatewayNamespace, order.GatewayName, order.ListenerName)
	}

	batch := newMoveBatch(order)
	defer e.release(batch)
	e.register(batch)

	deadline := time.Now().Add(order.Wait)
	for {
		e.dispatchPresent(batch)
		if !batch.hasPending() {
			break
		}
		remaining := time.Until(deadline)
		if remaining <= 0 {
			batch.failPending(mcgatewayv1alpha1.PlayerAssignmentReasonNotConnected)
			break
		}
		// A player connecting wakes the batch immediately; the timer only bounds
		// the wait for players who never arrive.
		timer := time.NewTimer(remaining)
		select {
		case <-ctx.Done():
			timer.Stop()
			batch.failPending(mcgatewayv1alpha1.PlayerAssignmentReasonNotConnected)
			return batch.results(), ctx.Err()
		case <-batch.wake:
			timer.Stop()
		case <-timer.C:
		}
	}

	if err := e.awaitOutstanding(ctx, batch); err != nil {
		return batch.results(), err
	}
	return batch.results(), nil
}

// dispatchPresent sends a move for every pending player who is present, subject
// to the batch's policy.
func (e *MoveEngine) dispatchPresent(batch *moveBatch) {
	order := batch.order
	presences := make(map[string]Presence, len(order.Players))
	for _, player := range batch.pending() {
		presence, ok := PresenceOnListener(e.mgr.Presence(),
			order.GatewayNamespace, order.GatewayName, order.ListenerName, player)
		if ok {
			presences[player] = presence
		}
	}

	if order.Policy == MoveAllOrNothing && len(presences) != len(batch.pending()) {
		// Nobody moves until everybody is here.
		return
	}
	if len(presences) == 0 {
		return
	}

	// A selector target resolves once: every player goes to the same server.
	var shared *Server
	if order.Target.IsShared() {
		server, ok := ResolveSharedServer(e.mgr,
			order.GatewayNamespace, order.GatewayName, order.ListenerName, order.Target.Selector)
		if !ok {
			batch.failPending(mcgatewayv1alpha1.PlayerAssignmentReasonNoMatchingServer)
			return
		}
		shared = server
	}

	for player, presence := range presences {
		serverName, reason := ResolveMoveTarget(e.mgr,
			order.GatewayNamespace, order.GatewayName, order.ListenerName,
			order.Target, presence, shared)
		if serverName == "" {
			batch.fail(player, reason)
			continue
		}

		commandID := apiMoveCommandPrefix + uuid.NewString() + "#" + player
		results := e.await(commandID)
		err := e.mgr.SendMove(presence.ProxyID, MoveRequest{
			CommandID:  commandID,
			PlayerUUID: player,
			ServerName: serverName,
		})
		if err != nil {
			e.forget(commandID)
			batch.fail(player, mcgatewayv1alpha1.PlayerAssignmentReasonProxyUnavailable)
			continue
		}
		batch.moving(player, serverName, commandID, results)
	}
}

// awaitOutstanding blocks until every dispatched move has been reported.
func (e *MoveEngine) awaitOutstanding(ctx context.Context, batch *moveBatch) error {
	for _, move := range batch.outstanding() {
		select {
		case outcome := <-move.results:
			batch.settle(move.player, outcome)
		case <-ctx.Done():
			// The move is already with the proxy and will still happen; the
			// caller just will not learn the outcome here.
			return ctx.Err()
		}
		e.forget(move.commandID)
	}
	return nil
}

// await registers interest in a command's result before it is dispatched, so a
// result that comes back immediately is never missed.
func (e *MoveEngine) await(commandID string) chan MoveOutcome {
	results := make(chan MoveOutcome, 1)
	e.mu.Lock()
	defer e.mu.Unlock()
	e.results[commandID] = results
	return results
}

func (e *MoveEngine) forget(commandID string) {
	e.mu.Lock()
	defer e.mu.Unlock()
	delete(e.results, commandID)
}

// deliver routes a proxy's report to the batch that dispatched it. Results of
// PlayerTransfer moves are not ours and fall through.
func (e *MoveEngine) deliver(outcome MoveOutcome) {
	e.mu.Lock()
	results, ok := e.results[outcome.CommandID]
	e.mu.Unlock()
	if !ok {
		return
	}
	select {
	case results <- outcome:
	default:
	}
}

// register makes the batch wakeable by the arrival of any player it wants.
func (e *MoveEngine) register(batch *moveBatch) {
	e.mu.Lock()
	defer e.mu.Unlock()
	for _, player := range batch.order.Players {
		if _, ok := e.arrivals[player]; !ok {
			e.arrivals[player] = make(map[chan struct{}]struct{})
		}
		e.arrivals[player][batch.wake] = struct{}{}
	}
}

func (e *MoveEngine) release(batch *moveBatch) {
	e.mu.Lock()
	defer e.mu.Unlock()
	for _, player := range batch.order.Players {
		waiters, ok := e.arrivals[player]
		if !ok {
			continue
		}
		delete(waiters, batch.wake)
		if len(waiters) == 0 {
			delete(e.arrivals, player)
		}
	}
	for _, move := range batch.outstanding() {
		delete(e.results, move.commandID)
	}
}

func (e *MoveEngine) wake(playerUUID string) {
	e.mu.Lock()
	waiters := make([]chan struct{}, 0, len(e.arrivals[playerUUID]))
	for wake := range e.arrivals[playerUUID] {
		waiters = append(waiters, wake)
	}
	e.mu.Unlock()

	for _, wake := range waiters {
		select {
		case wake <- struct{}{}:
		default:
			// Already signalled; the batch re-reads presence when it runs.
		}
	}
}

// moveBatch is the mutable state of one in-flight MovePlayers call.
type moveBatch struct {
	order MoveOrder
	wake  chan struct{}

	mu      sync.Mutex
	state   map[string]*playerMove
	ordered []string
}

type playerMove struct {
	player     string
	serverName string
	commandID  string
	results    chan MoveOutcome
	done       bool
	success    bool
	reason     string
}

func newMoveBatch(order MoveOrder) *moveBatch {
	b := &moveBatch{
		order: order,
		wake:  make(chan struct{}, 1),
		state: make(map[string]*playerMove, len(order.Players)),
	}
	for _, player := range order.Players {
		if _, ok := b.state[player]; ok {
			continue // a player listed twice is moved once
		}
		b.state[player] = &playerMove{player: player}
		b.ordered = append(b.ordered, player)
	}
	return b
}

// pending returns the players who have neither been dispatched nor failed.
func (b *moveBatch) pending() []string {
	b.mu.Lock()
	defer b.mu.Unlock()
	var out []string
	for _, player := range b.ordered {
		if move := b.state[player]; !move.done && move.commandID == "" {
			out = append(out, player)
		}
	}
	return out
}

func (b *moveBatch) hasPending() bool { return len(b.pending()) > 0 }

func (b *moveBatch) fail(player, reason string) {
	b.mu.Lock()
	defer b.mu.Unlock()
	move := b.state[player]
	move.done = true
	move.reason = reason
}

func (b *moveBatch) failPending(reason string) {
	for _, player := range b.pending() {
		b.fail(player, reason)
	}
}

func (b *moveBatch) moving(player, serverName, commandID string, results chan MoveOutcome) {
	b.mu.Lock()
	defer b.mu.Unlock()
	move := b.state[player]
	move.serverName = serverName
	move.commandID = commandID
	move.results = results
}

func (b *moveBatch) settle(player string, outcome MoveOutcome) {
	b.mu.Lock()
	defer b.mu.Unlock()
	move := b.state[player]
	move.done = true
	move.success = outcome.Success
	if !outcome.Success {
		move.reason = outcome.Reason
		if move.reason == "" {
			move.reason = mcgatewayv1alpha1.PlayerAssignmentReasonMoveFailed
		}
	}
}

// outstanding returns the dispatched moves still awaiting a proxy report.
func (b *moveBatch) outstanding() []*playerMove {
	b.mu.Lock()
	defer b.mu.Unlock()
	var out []*playerMove
	for _, player := range b.ordered {
		if move := b.state[player]; move.commandID != "" && !move.done {
			out = append(out, move)
		}
	}
	return out
}

// results reports the outcome of every player, in the order they were listed.
func (b *moveBatch) results() []PlayerMoveResult {
	b.mu.Lock()
	defer b.mu.Unlock()
	out := make([]PlayerMoveResult, 0, len(b.ordered))
	for _, player := range b.ordered {
		move := b.state[player]
		result := PlayerMoveResult{
			PlayerUUID: player,
			ServerName: move.serverName,
			Success:    move.done && move.success,
			Reason:     move.reason,
		}
		if !move.done && result.Reason == "" {
			// Dispatched but never reported, because the call was cut short.
			result.Reason = mcgatewayv1alpha1.PlayerAssignmentReasonMoveFailed
		}
		out = append(out, result)
	}
	return out
}
