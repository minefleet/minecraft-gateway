package gateway

import (
	"context"
	"fmt"

	mcgatewayv1alpha1 "minefleet.dev/minecraft-gateway/api/controller/v1alpha1"
	mfdiscovery "minefleet.dev/minecraft-gateway/internal/discovery"
	"sigs.k8s.io/controller-runtime/pkg/client"
	logf "sigs.k8s.io/controller-runtime/pkg/log"
	gatewayv1 "sigs.k8s.io/gateway-api/apis/v1"
)

type Infrastructure struct {
	Labels      map[gatewayv1.LabelKey]gatewayv1.LabelValue           `json:"labels,omitempty"`
	Annotations map[gatewayv1.AnnotationKey]gatewayv1.AnnotationValue `json:"annotations,omitempty"`
	Config      mcgatewayv1alpha1.NetworkInfrastructureSpec           `json:"config,omitempty"`
	Status      mcgatewayv1alpha1.NetworkInfrastructureStatus         `json:"status,omitempty"`
}

func GetInfrastructureForGateway(c client.Client, ctx context.Context, gw gatewayv1.Gateway) (Infrastructure, error) {
	log := logf.FromContext(ctx)
	var gwConfig *Infrastructure
	if config, err := GetInfrastructureByGateway(c, ctx, gw); err == nil {
		gwConfig = &config
	} else {
		log.Info(fmt.Sprintf("%s: no (valid) infrastructure config found", err.Error()))
	}
	class, err := GetGatewayClassByGateway(c, ctx, gw)
	if err != nil {
		return Infrastructure{}, err
	}
	var classConfig *Infrastructure
	if config, err := GetInfrastructureByClass(c, ctx, class); err == nil {
		classConfig = &config
	} else {
		log.Info(fmt.Sprintf("%s: no (valid) class infrastructure config found", err.Error()))
	}
	return merge(classConfig, gwConfig)
}

func GetInfrastructureByGateway(c client.Client, ctx context.Context, gateway gatewayv1.Gateway) (Infrastructure, error) {
	if config, err := mfdiscovery.GetNetworkInfrastructureByGateway(c, ctx, gateway); err == nil {
		return Infrastructure{
			Status:      config.Status,
			Config:      config.Spec,
			Labels:      gateway.Spec.Infrastructure.Labels,
			Annotations: gateway.Spec.Infrastructure.Annotations,
		}, nil
	} else {
		return Infrastructure{}, err
	}
}

func GetInfrastructureByClass(c client.Client, ctx context.Context, class gatewayv1.GatewayClass) (Infrastructure, error) {
	if config, err := mfdiscovery.GetNetworkInfrastructureByRef(c, ctx, class.Spec.ParametersRef); err == nil {
		return Infrastructure{
			Status: config.Status,
			Config: config.Spec,
		}, nil
	} else {
		return Infrastructure{}, err
	}
}

// merge combines class-level (first) and gateway-level (second) infrastructure.
// Gateway-level config overrides class-level for Network and Discovery.
// Edge is always taken from the class level only — it is a cluster-wide concern
// and must not be configurable per-Gateway.
func merge(first *Infrastructure, second *Infrastructure) (Infrastructure, error) {
	if first == nil && second == nil {
		return Infrastructure{}, nil
	}
	if second == nil {
		return *first, nil
	}
	if first == nil {
		// No class config: use gateway config but strip Edge.
		result := *second
		result.Config.Edge = nil
		return result, nil
	}

	labels := make(map[gatewayv1.LabelKey]gatewayv1.LabelValue, len(first.Labels)+len(second.Labels))
	for k, v := range first.Labels {
		labels[k] = v
	}
	for k, v := range second.Labels {
		labels[k] = v
	}

	annotations := make(map[gatewayv1.AnnotationKey]gatewayv1.AnnotationValue, len(first.Annotations)+len(second.Annotations))
	for k, v := range first.Annotations {
		annotations[k] = v
	}
	for k, v := range second.Annotations {
		annotations[k] = v
	}

	// Gateway-level overrides class-level for Network; Discovery always comes
	// from whichever infrastructure defines it (gateway wins if present).
	network := first.Config.Network
	if second.Config.Network != nil {
		network = second.Config.Network
	}

	return Infrastructure{
		Labels:      labels,
		Annotations: annotations,
		Config: mcgatewayv1alpha1.NetworkInfrastructureSpec{
			Discovery: second.Config.Discovery,
			Network:   network,
			Edge:      first.Config.Edge, // always class-level only
		},
		Status: first.Status,
	}, nil
}
