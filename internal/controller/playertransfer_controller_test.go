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
	"sync"
	"time"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/utils/ptr"
	mcgatewayv1alpha1 "minefleet.dev/minecraft-gateway/api/controller/v1alpha1"
	networkdp "minefleet.dev/minecraft-gateway/internal/dataplane/network"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"
)

const (
	alice = "6bf1e7f4-2c15-4c2f-9a5e-1a0b3c4d5e6f"
	bob   = "9d2c8a10-77b3-4a1e-8f0c-2b3d4e5f6a7b"
)

// fakeStreams scripts player presence and captures the moves the reconciler
// dispatches, standing in for connected proxies.
type fakeStreams struct {
	mu       sync.Mutex
	presence map[string]networkdp.Presence
	moves    []networkdp.MoveRequest
	sendErr  error
	// route and selector answer the corresponding target modes.
	route    *networkdp.Server
	selector *networkdp.Server
}

func newFakeStreams() *fakeStreams {
	return &fakeStreams{presence: map[string]networkdp.Presence{}}
}

func (f *fakeStreams) connect(playerUUID, server string) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.presence[playerUUID] = networkdp.Presence{
		PlayerUUID:       playerUUID,
		ProxyID:          "proxy-a",
		ServerName:       server,
		GatewayNamespace: "default",
		GatewayName:      "test-gateway",
		ListenerName:     "minecraft",
	}
}

func (f *fakeStreams) Lookup(playerUUID string) (networkdp.Presence, bool) {
	f.mu.Lock()
	defer f.mu.Unlock()
	presence, ok := f.presence[playerUUID]
	return presence, ok
}

func (f *fakeStreams) SendMove(_ string, req networkdp.MoveRequest) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	if f.sendErr != nil {
		return f.sendErr
	}
	f.moves = append(f.moves, req)
	return nil
}

func (f *fakeStreams) sentMoves() []networkdp.MoveRequest {
	f.mu.Lock()
	defer f.mu.Unlock()
	return append([]networkdp.MoveRequest{}, f.moves...)
}

func (f *fakeStreams) ResolveRoute(_, _, _ string, _ networkdp.RouteQuery) (*networkdp.Server, bool) {
	return f.route, f.route != nil
}

func (f *fakeStreams) ResolveSelector(_, _, _ string, _ func(*networkdp.Service) bool) (*networkdp.Server, bool) {
	return f.selector, f.selector != nil
}

func (f *fakeStreams) OnMoveResult(func(networkdp.MoveOutcome)) {}

func (f *fakeStreams) Presence() *networkdp.PresenceMap { return networkdp.NewPresenceMap() }

var _ = Describe("PlayerTransfer Controller", func() {
	var (
		ctx      context.Context
		streams  *fakeStreams
		r        *PlayerTransferReconciler
		created  []types.NamespacedName
		nextName func() string
	)

	BeforeEach(func() {
		ctx = context.Background()
		streams = newFakeStreams()
		r = &PlayerTransferReconciler{Client: k8sClient, Scheme: k8sClient.Scheme(), Streams: streams}
		created = nil

		counter := 0
		nextName = func() string {
			counter++
			return fmt.Sprintf("transfer-%d-%d", time.Now().UnixNano(), counter)
		}
	})

	AfterEach(func() {
		for _, name := range created {
			resource := &mcgatewayv1alpha1.PlayerTransfer{}
			if err := k8sClient.Get(ctx, name, resource); err == nil {
				Expect(k8sClient.Delete(ctx, resource)).To(Succeed())
			}
		}
	})

	// create writes a transfer and returns its key.
	create := func(spec mcgatewayv1alpha1.PlayerTransferSpec) types.NamespacedName {
		name := types.NamespacedName{Name: nextName(), Namespace: "default"}
		Expect(k8sClient.Create(ctx, &mcgatewayv1alpha1.PlayerTransfer{
			ObjectMeta: metav1.ObjectMeta{Name: name.Name, Namespace: name.Namespace},
			Spec:       spec,
		})).To(Succeed())
		created = append(created, name)
		return name
	}

	get := func(name types.NamespacedName) *mcgatewayv1alpha1.PlayerTransfer {
		resource := &mcgatewayv1alpha1.PlayerTransfer{}
		Expect(k8sClient.Get(ctx, name, resource)).To(Succeed())
		return resource
	}

	reconcileOnce := func(name types.NamespacedName) reconcile.Result {
		result, err := r.Reconcile(ctx, reconcile.Request{NamespacedName: name})
		Expect(err).NotTo(HaveOccurred())
		return result
	}

	baseSpec := func(players ...string) mcgatewayv1alpha1.PlayerTransferSpec {
		uuids := make([]mcgatewayv1alpha1.PlayerUUID, 0, len(players))
		for _, p := range players {
			uuids = append(uuids, mcgatewayv1alpha1.PlayerUUID(p))
		}
		return mcgatewayv1alpha1.PlayerTransferSpec{
			GatewayName:  "test-gateway",
			ListenerName: "minecraft",
			Players:      uuids,
			Target:       mcgatewayv1alpha1.PlayerTransferTarget{ServerName: ptr.To("game-0")},
		}
	}

	Context("with the AllOrNothing policy", func() {
		It("waits while any player is absent", func() {
			name := create(baseSpec(alice, bob))
			streams.connect(alice, "lobby-0")

			reconcileOnce(name)

			Expect(streams.sentMoves()).To(BeEmpty(),
				"no player may be moved until all of them are present")
			transfer := get(name)
			Expect(transfer.Status.Phase).To(Equal(mcgatewayv1alpha1.PendingPlayerTransferPhase))
			Expect(transfer.Status.Assignments).To(HaveLen(2))
		})

		It("moves every player once they are all present", func() {
			name := create(baseSpec(alice, bob))
			streams.connect(alice, "lobby-0")
			streams.connect(bob, "lobby-1")

			reconcileOnce(name)

			moves := streams.sentMoves()
			Expect(moves).To(HaveLen(2))
			for _, move := range moves {
				Expect(move.ServerName).To(Equal("game-0"))
			}
			transfer := get(name)
			Expect(transfer.Status.Phase).To(Equal(mcgatewayv1alpha1.PendingPlayerTransferPhase))
			for _, assignment := range transfer.Status.Assignments {
				Expect(assignment.Phase).To(Equal(mcgatewayv1alpha1.MovingPlayerAssignmentPhase))
				Expect(assignment.AssignedServer).To(Equal("game-0"))
			}
		})

		It("completes when every move succeeds", func() {
			name := create(baseSpec(alice))
			streams.connect(alice, "lobby-0")
			reconcileOnce(name)

			r.applyMoveResult(ctx, networkdp.MoveOutcome{
				CommandID: fmt.Sprintf("default/%s#%s", name.Name, alice),
				Success:   true,
			})

			transfer := get(name)
			Expect(transfer.Status.Phase).To(Equal(mcgatewayv1alpha1.CompletedPlayerTransferPhase))
			Expect(transfer.Status.Assignments[0].Phase).
				To(Equal(mcgatewayv1alpha1.CompletedPlayerAssignmentPhase))
		})

		It("fails when a move fails", func() {
			name := create(baseSpec(alice))
			streams.connect(alice, "lobby-0")
			reconcileOnce(name)

			r.applyMoveResult(ctx, networkdp.MoveOutcome{
				CommandID: fmt.Sprintf("default/%s#%s", name.Name, alice),
				Success:   false,
				Reason:    "server_not_registered",
			})

			transfer := get(name)
			Expect(transfer.Status.Phase).To(Equal(mcgatewayv1alpha1.FailedPlayerTransferPhase))
			Expect(transfer.Status.Assignments[0].Reason).To(Equal("server_not_registered"))
		})

		It("expires when the TTL elapses before everyone is present", func() {
			spec := baseSpec(alice, bob)
			spec.TTL = &metav1.Duration{Duration: time.Nanosecond}
			name := create(spec)
			streams.connect(alice, "lobby-0")

			time.Sleep(10 * time.Millisecond)
			reconcileOnce(name)

			transfer := get(name)
			Expect(transfer.Status.Phase).To(Equal(mcgatewayv1alpha1.ExpiredPlayerTransferPhase))
			Expect(streams.sentMoves()).To(BeEmpty())
		})

		It("ignores a player connected to a different gateway", func() {
			name := create(baseSpec(alice))
			streams.mu.Lock()
			streams.presence[alice] = networkdp.Presence{
				PlayerUUID: alice, ProxyID: "proxy-z", ServerName: "lobby-0",
				GatewayNamespace: "default", GatewayName: "other-gateway", ListenerName: "minecraft",
			}
			streams.mu.Unlock()

			reconcileOnce(name)

			Expect(streams.sentMoves()).To(BeEmpty(),
				"a player on another gateway is not addressable by this transfer")
		})
	})

	Context("with the BestEffort policy", func() {
		It("moves the players who are present and leaves the rest pending", func() {
			spec := baseSpec(alice, bob)
			spec.Policy = mcgatewayv1alpha1.BestEffortPlayerTransferPolicy
			name := create(spec)
			streams.connect(alice, "lobby-0")

			reconcileOnce(name)

			Expect(streams.sentMoves()).To(HaveLen(1))
			Expect(streams.sentMoves()[0].PlayerUUID).To(Equal(alice))
			Expect(get(name).Status.Phase).To(Equal(mcgatewayv1alpha1.PendingPlayerTransferPhase))
		})

		It("settles as PartiallyCompleted when some players never arrive", func() {
			spec := baseSpec(alice, bob)
			spec.Policy = mcgatewayv1alpha1.BestEffortPlayerTransferPolicy
			spec.TTL = &metav1.Duration{Duration: time.Nanosecond}
			name := create(spec)
			streams.connect(alice, "lobby-0")

			reconcileOnce(name)
			r.applyMoveResult(ctx, networkdp.MoveOutcome{
				CommandID: fmt.Sprintf("default/%s#%s", name.Name, alice),
				Success:   true,
			})
			time.Sleep(10 * time.Millisecond)
			reconcileOnce(name)

			transfer := get(name)
			Expect(transfer.Status.Phase).
				To(Equal(mcgatewayv1alpha1.PartiallyCompletedPlayerTransferPhase))
			for _, assignment := range transfer.Status.Assignments {
				if string(assignment.PlayerUUID) == bob {
					Expect(assignment.Phase).To(Equal(mcgatewayv1alpha1.FailedPlayerAssignmentPhase))
					Expect(assignment.Reason).
						To(Equal(mcgatewayv1alpha1.PlayerAssignmentReasonNotConnected))
				}
			}
		})

		It("fails when no player ever arrives", func() {
			spec := baseSpec(alice)
			spec.Policy = mcgatewayv1alpha1.BestEffortPlayerTransferPolicy
			spec.TTL = &metav1.Duration{Duration: time.Nanosecond}
			name := create(spec)

			time.Sleep(10 * time.Millisecond)
			reconcileOnce(name)

			Expect(get(name).Status.Phase).To(Equal(mcgatewayv1alpha1.FailedPlayerTransferPhase))
		})
	})

	Context("target modes", func() {
		It("routes each player individually in routeMode", func() {
			spec := baseSpec(alice)
			spec.Target = mcgatewayv1alpha1.PlayerTransferTarget{
				Route: ptr.To(mcgatewayv1alpha1.JoinPlayerTransferRouteMode),
			}
			streams.route = &networkdp.Server{Name: "routed-0"}
			name := create(spec)
			streams.connect(alice, "lobby-0")

			reconcileOnce(name)

			Expect(streams.sentMoves()).To(HaveLen(1))
			Expect(streams.sentMoves()[0].ServerName).To(Equal("routed-0"))
		})

		It("fails a player with no matching route", func() {
			spec := baseSpec(alice)
			spec.Target = mcgatewayv1alpha1.PlayerTransferTarget{
				Route: ptr.To(mcgatewayv1alpha1.JoinPlayerTransferRouteMode),
			}
			name := create(spec)
			streams.connect(alice, "lobby-0")

			reconcileOnce(name)

			transfer := get(name)
			Expect(transfer.Status.Phase).To(Equal(mcgatewayv1alpha1.FailedPlayerTransferPhase))
			Expect(transfer.Status.Assignments[0].Reason).
				To(Equal(mcgatewayv1alpha1.PlayerAssignmentReasonNoMatchingRoute))
		})

		It("sends every player to one server in labelSelector mode", func() {
			spec := baseSpec(alice, bob)
			spec.Target = mcgatewayv1alpha1.PlayerTransferTarget{
				Selector: &metav1.LabelSelector{MatchLabels: map[string]string{"fleet": "lobby"}},
			}
			streams.selector = &networkdp.Server{Name: "selected-0"}
			name := create(spec)
			streams.connect(alice, "lobby-0")
			streams.connect(bob, "lobby-1")

			reconcileOnce(name)

			moves := streams.sentMoves()
			Expect(moves).To(HaveLen(2))
			for _, move := range moves {
				Expect(move.ServerName).To(Equal("selected-0"),
					"all players of a selector transfer share one server")
			}
		})
	})

	It("re-sends a move whose result was lost with the previous leader", func() {
		name := create(baseSpec(alice))
		streams.connect(alice, "lobby-0")
		reconcileOnce(name)
		Expect(streams.sentMoves()).To(HaveLen(1))

		// A new leader takes over: the transfer is still Moving, but nothing
		// in memory records that the command was ever sent.
		fresh := &PlayerTransferReconciler{Client: k8sClient, Scheme: k8sClient.Scheme(), Streams: streams}
		_, err := fresh.Reconcile(ctx, reconcile.Request{NamespacedName: name})
		Expect(err).NotTo(HaveOccurred())

		Expect(streams.sentMoves()).To(HaveLen(2),
			"the move must be re-sent rather than awaited forever")
	})

	It("does not reconcile a transfer that already settled", func() {
		name := create(baseSpec(alice))
		streams.connect(alice, "lobby-0")
		reconcileOnce(name)
		r.applyMoveResult(ctx, networkdp.MoveOutcome{
			CommandID: fmt.Sprintf("default/%s#%s", name.Name, alice),
			Success:   true,
		})

		before := len(streams.sentMoves())
		reconcileOnce(name)

		Expect(streams.sentMoves()).To(HaveLen(before),
			"a completed transfer must not dispatch further moves")
	})

	It("rejects a spec change", func() {
		name := create(baseSpec(alice))
		transfer := get(name)
		transfer.Spec.ListenerName = "changed"

		Expect(k8sClient.Update(ctx, transfer)).NotTo(Succeed(),
			"the spec is immutable so a transfer cannot be retargeted in flight")
	})
})
