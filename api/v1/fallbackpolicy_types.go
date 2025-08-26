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
	"sigs.k8s.io/gateway-api/apis/v1alpha2"
)

// EDIT THIS FILE!  THIS IS SCAFFOLDING FOR YOU TO OWN!
// NOTE: json tags are required.  Any new fields you add must have json tags for the fields to be serialized.

// FallbackPolicySpec defines the desired state of FallbackPolicy
type FallbackPolicySpec struct {
	// TargetRefs identifies an API object to apply the policy to.
	// Only Services have Extended support. Implementations MAY support
	// additional objects, with Implementation Specific support.
	// Note that this config applies to the entire referenced resource
	// by default, but this default may change in the future to provide
	// a more granular application of the policy.
	//
	// TargetRefs must be _distinct_. This means either that:
	//
	// * They select different targets. If this is the case, then targetRef
	//   entries are distinct. In terms of fields, this means that the
	//   multi-part key defined by `group`, `kind`, and `name` must
	//   be unique across all targetRef entries in the FallbackPolicy.
	// * They select different sectionNames in the same target.
	//
	// Support: Extended for Kubernetes Service
	//
	// Support: Implementation-specific for any other resource
	//
	// +required
	// +listType=atomic
	// +kubebuilder:validation:MinItems=1
	// +kubebuilder:validation:MaxItems=16
	// +kubebuilder:validation:XValidation:message="sectionName must be specified when targetRefs includes 2 or more references to the same target",rule="self.all(p1, self.all(p2, p1.group == p2.group && p1.kind == p2.kind && p1.name == p2.name ? ((!has(p1.sectionName) || p1.sectionName == '') == (!has(p2.sectionName) || p2.sectionName == '')) : true))"
	// +kubebuilder:validation:XValidation:message="sectionName must be unique when targetRefs includes 2 or more references to the same target",rule="self.all(p1, self.exists_one(p2, p1.group == p2.group && p1.kind == p2.kind && p1.name == p2.name && (((!has(p1.sectionName) || p1.sectionName == '') && (!has(p2.sectionName) || p2.sectionName == '')) || (has(p1.sectionName) && has(p2.sectionName) && p1.sectionName == p2.sectionName))))"
	TargetRefs []v1alpha2.LocalPolicyTargetReferenceWithSectionName `json:"targetRefs"`

	// Priority indicates the priority of this fallback policy.
	// This field determines the relative ordering of fallback policies
	// when multiple policies may apply to the same target.
	//
	// * A lower value indicates a higher precedence.
	// * If multiple fallback policies apply, the one with the lowest
	//   priority MUST be selected.
	// * If two or more fallback policies share the same priority,
	//   behavior is implementation-specific.
	// * If unspecified, This FallbackPolicy will have the lowest possible precedence.
	//
	// +optional
	Priority int `json:"priority"`

	// Permission indicates the required permission for a Minecraft player in order for this
	// FallbackPolicy to be applied to them.
	//
	// Implementations MAY integrate with permission systems to enforce
	// this requirement. If unspecified, no permission is required and the
	// FallbackPolicy applies to all players unless restricted by other
	// fields.
	//
	// +optional
	Permission string `json:"permission"`
}

// +kubebuilder:object:root=true
// +kubebuilder:subresource:status

// FallbackPolicy is the Schema for the fallbackpolicies API
type FallbackPolicy struct {
	metav1.TypeMeta `json:",inline"`

	// metadata is a standard object metadata
	// +optional
	metav1.ObjectMeta `json:"metadata,omitempty,omitzero"`

	// spec defines the desired state of FallbackPolicy
	// +required
	Spec FallbackPolicySpec `json:"spec"`

	// status defines the observed state of FallbackPolicy
	// +optional
	Status v1alpha2.PolicyStatus `json:"status,omitempty,omitzero"`
}

// +kubebuilder:object:root=true

// FallbackPolicyList contains a list of FallbackPolicy
type FallbackPolicyList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []FallbackPolicy `json:"items"`
}

func init() {
	SchemeBuilder.Register(&FallbackPolicy{}, &FallbackPolicyList{})
}
