package v1

import (
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	gatewayv1 "sigs.k8s.io/gateway-api/apis/v1"
)

type MinecraftRoute struct {
	gatewayv1.CommonRouteSpec `json:",inline"`
	// +optional
	BackendRefs []MinecraftBackendRef `json:"backendRefs,omitempty"`
	// +optional
	FilterRules []MinecraftFilterRules `json:"filterRules,omitempty"`
	// +optional
	Priority int `json:"priority,omitempty"`
}

type MinecraftBackendRef struct {
	gatewayv1.BackendRef `json:",inline"`
	// +optional
	DistributionStrategy MinecraftDistributionStrategy `json:"distributionStrategy,omitempty"`
}

type MinecraftDistributionStrategy struct {
	Type MinecraftDistributionStrategyType `json:"type,omitempty"`
}

type MinecraftDistributionStrategyType string

const (
	MinecraftDistributionStrategyRandom       = "random"
	MinecraftDistributionStrategyLeastPlayers = "least-players"
)

type MinecraftFilterRules struct {
	// +optional
	Type MinecraftFilterRuleType `json:"type,omitempty"`
	// +required
	Rules []MinecraftFilterRule `json:"rules"`
}

type MinecraftFilterRuleType string

const (
	MinecraftFilterRuleAll  = "all"
	MinecraftFilterRuleAny  = "any"
	MinecraftFilterRuleNone = "none"
)

type MinecraftFilterRule struct {
	// +optional
	Domain string `json:"domain,omitempty"`
	// +optional
	Permission string `json:"permission,omitempty"`
	// +optional
	FallbackFor metav1.LabelSelector `json:"fallbackFor,omitempty"`
}

type MinecraftService struct {
	// +required
	Name string `json:"name"`
	// +optional
	Servers map[string]MinecraftServer `json:",omitempty"`
	// +optional
	DistributionStrategy MinecraftDistributionStrategy `json:"distributionStrategy,omitempty"`
	// +optional
	FilterRules []MinecraftFilterRules `json:"filterRules,omitempty"`
}

type MinecraftServer struct {
	// +optional
	Endpoint string `json:"endpoint,omitempty"`
	// +optional
	MaxPlayers int `json:"maxPlayers,omitempty"`
	// +optional
	CurrentPlayers int `json:"currentPlayers,omitempty"`
}
