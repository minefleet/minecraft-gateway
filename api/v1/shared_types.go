package v1

import (
	gatewayv1 "sigs.k8s.io/gateway-api/apis/v1"
)

type MinecraftRoute struct {
	gatewayv1.CommonRouteSpec `json:",inline"`
	// +optional
	BackendRefs []MinecraftBackendRef `json:"backendRefs,omitempty"`
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
	MinecraftDistributionStrategyRandom       MinecraftDistributionStrategyType = "random"
	MinecraftDistributionStrategyLeastPlayers MinecraftDistributionStrategyType = "least-players"
)

type MinecraftFilterRuleSet struct {
	// For MinecraftFilterRule.Domain only MinecraftFilterRuleAny is applicable
	// +optional
	Type MinecraftFilterRuleType `json:"type,omitempty"`
}

type MinecraftFilterRuleType string

const (
	MinecraftFilterRuleAll MinecraftFilterRuleType = "all"
	MinecraftFilterRuleAny MinecraftFilterRuleType = "any"
	// MinecraftFilterRuleNone is not a sufficient MinecraftFilterRuleType for edge routing based on MinecraftFilterRule.Domain
	MinecraftFilterRuleNone MinecraftFilterRuleType = "none"
)

type MinecraftFilterRule struct {
	// +optional
	Domain string `json:"domain,omitempty"`
	// +optional
	Permission string `json:"permission,omitempty"`
}

type MinecraftService struct {
	// +required
	Name string `json:"name"`
	// +optional
	Servers map[string]MinecraftServer `json:",omitempty"`
	// +optional
	DistributionStrategy MinecraftDistributionStrategy `json:"distributionStrategy,omitempty"`
	// +optional
	JoinRules []MinecraftJoinFilterRuleSet `json:"joinRules,omitempty"`
	// +optional
	FallbackRules []MinecraftFallbackFilterRuleSet `json:"fallbackRules,omitempty"`
}

type MinecraftServer struct {
	// +optional
	Endpoint string `json:"endpoint,omitempty"`
	// +optional
	MaxPlayers int `json:"maxPlayers,omitempty"`
	// +optional
	CurrentPlayers int `json:"currentPlayers,omitempty"`
}
