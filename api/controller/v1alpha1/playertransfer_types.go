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

package v1alpha1

import (
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// PlayerUUID is the Minecraft player UUID in canonical dashed form.
// +kubebuilder:validation:Pattern=`^[0-9a-fA-F]{8}-[0-9a-fA-F]{4}-[0-9a-fA-F]{4}-[0-9a-fA-F]{4}-[0-9a-fA-F]{12}$`
type PlayerUUID string

// PlayerTransferSpec defines the desired state of PlayerTransfer.
//
// The gateway is always looked up in the PlayerTransfer's own namespace,
// consistent with how route CRDs reference their gateway.
type PlayerTransferSpec struct {
	// gatewayName is the name of the Gateway whose proxies hold the players.
	// +required
	GatewayName string `json:"gatewayName"`

	// listenerName is the Gateway listener (section name) the players are connected to.
	// +required
	ListenerName string `json:"listenerName"`

	// players lists the player UUIDs to transfer.
	// +required
	// +kubebuilder:validation:MinItems=1
	Players []PlayerUUID `json:"players"`

	// target selects where the players are moved to.
	// +required
	Target PlayerTransferTarget `json:"target"`

	// policy controls whether the transfer waits for all players to be present.
	// +optional
	// +kubebuilder:default=AllOrNothing
	Policy PlayerTransferPolicy `json:"policy,omitempty"`

	// ttl bounds how long the transfer waits for players to become present.
	// Omit to wait indefinitely.
	// +optional
	TTL *metav1.Duration `json:"ttl,omitempty"`
}

// PlayerTransferTarget selects the destination of a transfer. Exactly one of
// routeMode, labelSelector or serverName must be set.
// +kubebuilder:validation:XValidation:rule="[has(self.routeMode), has(self.labelSelector), has(self.serverName)].filter(x, x).size() == 1",message="exactly one of routeMode, labelSelector or serverName must be set"
type PlayerTransferTarget struct {
	// routeMode evaluates the gateway's join or fallback routes individually per
	// player, so players may land on different servers.
	// +optional
	Route *PlayerTransferRouteMode `json:"routeMode,omitempty"`

	// labelSelector picks a single server matching the selector, using the
	// backend's distribution strategy. All players are moved to that one server.
	// +optional
	Selector *metav1.LabelSelector `json:"labelSelector,omitempty"`

	// serverName names an already registered server (pod name) explicitly. The
	// controller does not resolve connection details for this mode.
	// +optional
	ServerName *string `json:"serverName,omitempty"`
}

// PlayerTransferRouteMode selects which set of routes is evaluated per player.
// +kubebuilder:validation:Enum=join;fallback
type PlayerTransferRouteMode string

const (
	JoinPlayerTransferRouteMode     PlayerTransferRouteMode = "join"
	FallbackPlayerTransferRouteMode PlayerTransferRouteMode = "fallback"
)

// PlayerTransferPolicy controls how missing players are handled.
// +kubebuilder:validation:Enum=AllOrNothing;BestEffort
type PlayerTransferPolicy string

const (
	// AllOrNothingPlayerTransferPolicy waits until every player is simultaneously
	// present before moving any of them.
	AllOrNothingPlayerTransferPolicy PlayerTransferPolicy = "AllOrNothing"
	// BestEffortPlayerTransferPolicy moves whichever players are present and
	// fails the remaining ones individually.
	BestEffortPlayerTransferPolicy PlayerTransferPolicy = "BestEffort"
)

// PlayerTransferPhase is the aggregate phase of a transfer.
// +kubebuilder:validation:Enum=Pending;Completed;PartiallyCompleted;Failed;Expired
type PlayerTransferPhase string

const (
	PendingPlayerTransferPhase            PlayerTransferPhase = "Pending"
	CompletedPlayerTransferPhase          PlayerTransferPhase = "Completed"
	PartiallyCompletedPlayerTransferPhase PlayerTransferPhase = "PartiallyCompleted"
	FailedPlayerTransferPhase             PlayerTransferPhase = "Failed"
	ExpiredPlayerTransferPhase            PlayerTransferPhase = "Expired"
)

// PlayerAssignmentPhase is the phase of a single player's move.
// +kubebuilder:validation:Enum=Pending;Moving;Completed;Failed
type PlayerAssignmentPhase string

const (
	PendingPlayerAssignmentPhase   PlayerAssignmentPhase = "Pending"
	MovingPlayerAssignmentPhase    PlayerAssignmentPhase = "Moving"
	CompletedPlayerAssignmentPhase PlayerAssignmentPhase = "Completed"
	FailedPlayerAssignmentPhase    PlayerAssignmentPhase = "Failed"
)

// Reasons reported on a failed player assignment.
const (
	PlayerAssignmentReasonNotConnected     = "PlayerNotConnected"
	PlayerAssignmentReasonNoMatchingRoute  = "NoMatchingRoute"
	PlayerAssignmentReasonNoMatchingServer = "NoMatchingServer"
	PlayerAssignmentReasonMoveFailed       = "MoveFailed"
	PlayerAssignmentReasonProxyUnavailable = "ProxyUnavailable"
)

// PlayerTransferAssignment records the outcome for a single player.
type PlayerTransferAssignment struct {
	// playerUUID is the player this assignment belongs to.
	// +required
	PlayerUUID PlayerUUID `json:"playerUUID"`

	// assignedServer is the server the player was moved to, once resolved.
	// +optional
	AssignedServer string `json:"assignedServer,omitempty"`

	// phase is the state of this player's move.
	// +required
	Phase PlayerAssignmentPhase `json:"phase"`

	// reason explains a failed assignment.
	// +optional
	Reason string `json:"reason,omitempty"`
}

// PlayerTransferStatus defines the observed state of PlayerTransfer.
type PlayerTransferStatus struct {
	// phase is the aggregate state of the transfer.
	// +optional
	Phase PlayerTransferPhase `json:"phase,omitempty"`

	// assignments holds the per-player outcome.
	// +optional
	// +listType=map
	// +listMapKey=playerUUID
	Assignments []PlayerTransferAssignment `json:"assignments,omitempty"`
}

// +kubebuilder:object:root=true
// +kubebuilder:subresource:status
// +kubebuilder:printcolumn:name="Phase",type=string,JSONPath=`.status.phase`
// +kubebuilder:printcolumn:name="Age",type=date,JSONPath=`.metadata.creationTimestamp`

// PlayerTransfer is the Schema for the playertransfers API
type PlayerTransfer struct {
	metav1.TypeMeta `json:",inline"`

	// metadata is a standard object metadata
	// +optional
	metav1.ObjectMeta `json:"metadata,omitempty,omitzero"`

	// spec defines the desired state of PlayerTransfer. It is immutable: create a
	// new PlayerTransfer instead of retargeting one in flight.
	// +required
	// +kubebuilder:validation:XValidation:rule="self == oldSelf",message="spec is immutable"
	Spec PlayerTransferSpec `json:"spec"`

	// status defines the observed state of PlayerTransfer
	// +optional
	Status PlayerTransferStatus `json:"status,omitempty,omitzero"`
}

// +kubebuilder:object:root=true

// PlayerTransferList contains a list of PlayerTransfer
type PlayerTransferList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []PlayerTransfer `json:"items"`
}
