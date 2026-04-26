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
	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	gatewayv1 "sigs.k8s.io/gateway-api/apis/v1"
)

// EDIT THIS FILE!  THIS IS SCAFFOLDING FOR YOU TO OWN!
// NOTE: json tags are required.  Any new fields you add must have json tags for the fields to be serialized.

// NetworkDeploymentTemplate customizes the Velocity proxy Deployment created per
// gateway listener. Fields set here are merged into the controller-managed default
// via strategic merge patch. The selector and required proxy container env vars are
// always controller-managed and cannot be set here.
type NetworkDeploymentTemplate struct {
	// +optional
	Replicas *int32 `json:"replicas,omitempty"`
	// +optional
	Template *corev1.PodTemplateSpec `json:"template,omitempty"`
	// +optional
	Strategy *appsv1.DeploymentStrategy `json:"strategy,omitempty"`
}

// EdgeDaemonSetTemplate customizes the edge proxy DaemonSet. Fields set here are
// merged into the controller-managed default via strategic merge patch. The selector
// and bootstrap volume/mount are always controller-managed and cannot be set here.
type EdgeDaemonSetTemplate struct {
	// +optional
	Template *corev1.PodTemplateSpec `json:"template,omitempty"`
	// +optional
	UpdateStrategy *appsv1.DaemonSetUpdateStrategy `json:"updateStrategy,omitempty"`
	// +optional
	MinReadySeconds int32 `json:"minReadySeconds,omitempty"`
}

// NetworkInfrastructureSpec defines the desired state of NetworkInfrastructure
type NetworkInfrastructureSpec struct {
	Discovery Discovery `json:"discovery"`

	// networkTemplate optionally customizes the Deployment created for each
	// gateway listener. Fields set here are merged into the controller-managed
	// default. The selector and required proxy container env vars are always
	// enforced and cannot be overridden.
	// +optional
	Network *NetworkDeploymentTemplate `json:"networkTemplate,omitempty"`

	// edgeTemplate optionally configures the edge proxy DaemonSet and xDS
	// behavior for gateways using this infrastructure.
	// +optional
	Edge *EdgeSpec `json:"edgeTemplate,omitempty"`
}

// EdgeSpec configures the edge proxy DaemonSet and the xDS resources the
// controller pushes to it.
type EdgeSpec struct {
	// daemonSet optionally customizes the edge DaemonSet. Fields set here are
	// merged into the controller-managed default via strategic merge patch.
	// The selector and bootstrap volume/mount are always enforced.
	// +optional
	DaemonSet *EdgeDaemonSetTemplate `json:"daemonSet,omitempty"`

	// proxyProtocol enables the PROXY protocol v2 transport socket on upstream
	// clusters, so backend servers receive the real client IP.
	// +optional
	ProxyProtocol bool `json:"proxyProtocol,omitempty"`

	// rejectUnknown drops connections whose hostname is not matched by any
	// route. Defaults to false (connections fall through to the default cluster).
	// +optional
	RejectUnknown bool `json:"rejectUnknown,omitempty"`
}

type Discovery struct {
	NamespaceSelector *gatewayv1.RouteNamespaces `json:"namespaceSelector"`
	LabelSelector     metav1.LabelSelector       `json:"labelSelector"`
}

// NetworkInfrastructureStatus defines the observed state of NetworkInfrastructure.
type NetworkInfrastructureStatus struct {
	BackendRefs []gatewayv1.BackendObjectReference `json:"backendRefs"`
	Conditions  []metav1.Condition                 `json:"conditions"`
}

// +kubebuilder:object:root=true
// +kubebuilder:subresource:status

// NetworkInfrastructure is the Schema for the networkinfrastructures API
type NetworkInfrastructure struct {
	metav1.TypeMeta `json:",inline"`

	// metadata is a standard object metadata
	// +optional
	metav1.ObjectMeta `json:"metadata,omitempty,omitzero"`

	// spec defines the desired state of NetworkInfrastructure
	// +required
	Spec NetworkInfrastructureSpec `json:"spec"`

	// status defines the observed state of NetworkInfrastructure
	// +optional
	Status NetworkInfrastructureStatus `json:"status,omitempty,omitzero"`
}

// +kubebuilder:object:root=true

// NetworkInfrastructureList contains a list of NetworkInfrastructure
type NetworkInfrastructureList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []NetworkInfrastructure `json:"items"`
}

func init() {
	SchemeBuilder.Register(&NetworkInfrastructure{}, &NetworkInfrastructureList{})
}
