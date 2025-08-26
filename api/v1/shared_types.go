package v1

import (
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	gatewayv1 "sigs.k8s.io/gateway-api/apis/v1"
)

type MinecraftRoute struct {
	gatewayv1.CommonRouteSpec `json:",inline"`

	LabelSelector metav1.LabelSelector `json:"labelSelector"`
	// +optional
	Strategy MinecraftRouteStrategy `json:"strategy,omitempty"`
	// +optional
	FilterRules MinecraftFilterRules `json:"filterRules,omitempty"`
	// +optional
	Priority int `json:"priority,omitempty"`
}

type MinecraftRouteStrategy struct {
	Type MinecraftRouteStrategyType `json:"type,omitempty"`
}

type MinecraftRouteStrategyType string

const (
	MinecraftRouteStrategyRandom       = "random"
	MinecraftRouteStrategyLeastPlayers = "least-players"
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
