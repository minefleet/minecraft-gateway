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

// PlayerTransferSpec defines the desired state of PlayerTransfer
type PlayerTransferSpec struct {
	// +required
	GatewayName string `json:"gatewayName"`
	// +required
	ListenerName string `json:"listenerName"`
	// +required
	// +kubebuilder:validation:MinItems=1
	Players []PlayerUUID `json:"players"`
	// +required
	Target PlayerTransferTarget `json:"target"`
	Policy PlayerTransferPolicy `json:"policy"`
	TTL    *PlayerTransferTTL   `json:"ttl,omitempty"`
}

type PlayerTransferTarget struct {
	Route      *PlayerTransferRouteMode `json:"routeMode,omitempty"`
	Selector   *metav1.LabelSelector    `json:"labelSelector,omitempty"`
	ServerName *string                  `json:"serverName,omitempty"`
}
type PlayerUUID struct {
	// +kubebuilder:validation:Pattern=`^[0-9a-f]{8}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{12}$`
	string `json:",inline"`
}
type PlayerTransferTTL = string
type PlayerTransferRouteMode = string

const (
	JoinPlayerTransferRouteMode     PlayerTransferRouteMode = "join"
	FallbackPlayerTransferRouteMode PlayerTransferRouteMode = "fallback"
)

type PlayerTransferPolicy = string

const (
	AllOrNothingPlayerTransferPolicy PlayerTransferPolicy = "AllOrNothing"
	BestEffortPlayerTransferPolicy   PlayerTransferPolicy = "BestEffort"
)

// PlayerTransferStatus defines the observed state of PlayerTransfer.
type PlayerTransferStatus struct {
	// INSERT ADDITIONAL STATUS FIELD - define observed state of cluster
	// Important: Run "make" to regenerate code after modifying this file
}

// +kubebuilder:object:root=true
// +kubebuilder:subresource:status

// PlayerTransfer is the Schema for the playertransfers API
type PlayerTransfer struct {
	metav1.TypeMeta `json:",inline"`

	// metadata is a standard object metadata
	// +optional
	metav1.ObjectMeta `json:"metadata,omitempty,omitzero"`

	// spec defines the desired state of PlayerTransfer
	// +required
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
