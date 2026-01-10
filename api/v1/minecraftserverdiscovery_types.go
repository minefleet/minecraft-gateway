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

package v1

import (
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	gatewayv1 "sigs.k8s.io/gateway-api/apis/v1"
)

// EDIT THIS FILE!  THIS IS SCAFFOLDING FOR YOU TO OWN!
// NOTE: json tags are required.  Any new fields you add must have json tags for the fields to be serialized.

// MinecraftServerDiscoverySpec defines the desired state of MinecraftServerDiscovery
type MinecraftServerDiscoverySpec struct {
	NamespaceSelector *gatewayv1.RouteNamespaces `json:"namespaceSelector"`
	LabelSelector     metav1.LabelSelector       `json:"labelSelector"`
}

// MinecraftServerDiscoveryStatus defines the observed state of MinecraftServerDiscovery.
type MinecraftServerDiscoveryStatus struct {
	BackendRefs []gatewayv1.BackendObjectReference `json:"backendRefs"`
	Conditions  []metav1.Condition                 `json:"conditions"`
}

// +kubebuilder:object:root=true
// +kubebuilder:subresource:status

// MinecraftServerDiscovery is the Schema for the minecraftserverdiscoveries API
type MinecraftServerDiscovery struct {
	metav1.TypeMeta `json:",inline"`

	// metadata is a standard object metadata
	// +optional
	metav1.ObjectMeta `json:"metadata,omitempty,omitzero"`

	// spec defines the desired state of MinecraftServerDiscovery
	// +required
	Spec MinecraftServerDiscoverySpec `json:"spec"`

	// status defines the observed state of MinecraftServerDiscovery
	// +optional
	Status MinecraftServerDiscoveryStatus `json:"status,omitempty,omitzero"`
}

// +kubebuilder:object:root=true

// MinecraftServerDiscoveryList contains a list of MinecraftServerDiscovery
type MinecraftServerDiscoveryList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []MinecraftServerDiscovery `json:"items"`
}

func init() {
	SchemeBuilder.Register(&MinecraftServerDiscovery{}, &MinecraftServerDiscoveryList{})
}
