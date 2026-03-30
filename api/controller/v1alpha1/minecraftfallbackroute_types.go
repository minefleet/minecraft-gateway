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
	gatewayv1 "sigs.k8s.io/gateway-api/apis/v1"
)

// EDIT THIS FILE!  THIS IS SCAFFOLDING FOR YOU TO OWN!
// NOTE: json tags are required.  Any new fields you add must have json tags for the fields to be serialized.

// MinecraftFallbackRouteSpec defines the desired state of MinecraftFallbackRoute
type MinecraftFallbackRouteSpec struct {
	MinecraftRoute `json:",inline"`
	// +optional
	FilterRules []MinecraftFallbackFilterRuleSet `json:"filterRules,omitempty"`
}

// MinecraftFallbackRouteStatus defines the observed state of MinecraftFallbackRoute.
type MinecraftFallbackRouteStatus struct {
	gatewayv1.RouteStatus `json:",inline"`
}

type MinecraftFallbackFilterRuleSet struct {
	MinecraftFilterRuleSet `json:",inline"`
	// +required
	Rules []MinecraftFallbackFilterRule `json:"rules"`
}

type MinecraftFallbackFilterRule struct {
	MinecraftFilterRule `json:",inline"`
	// +optional
	FallbackFor metav1.LabelSelector `json:"fallbackFor,omitempty"`
}

// +kubebuilder:object:root=true
// +kubebuilder:subresource:status

// MinecraftFallbackRoute is the Schema for the minecraftfallbackroutes API
type MinecraftFallbackRoute struct {
	metav1.TypeMeta `json:",inline"`

	// metadata is a standard object metadata
	// +optional
	metav1.ObjectMeta `json:"metadata,omitempty,omitzero"`

	// spec defines the desired state of MinecraftFallbackRoute
	// +required
	Spec MinecraftFallbackRouteSpec `json:"spec"`

	// status defines the observed state of MinecraftFallbackRoute
	// +optional
	Status MinecraftFallbackRouteStatus `json:"status,omitempty,omitzero"`
}

// +kubebuilder:object:root=true

// MinecraftFallbackRouteList contains a list of MinecraftFallbackRoute
type MinecraftFallbackRouteList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []MinecraftFallbackRoute `json:"items"`
}

func init() {
	SchemeBuilder.Register(&MinecraftFallbackRoute{}, &MinecraftFallbackRouteList{})
}
