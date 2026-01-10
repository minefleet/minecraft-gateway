package gateway

import (
	"context"
	"errors"
	"fmt"

	mcgatewayv1 "minefleet.dev/minecraft-gateway/api/v1"
	mfdiscovery "minefleet.dev/minecraft-gateway/internal/discovery"
	"sigs.k8s.io/controller-runtime/pkg/client"
	logf "sigs.k8s.io/controller-runtime/pkg/log"
	gatewayv1 "sigs.k8s.io/gateway-api/apis/v1"
)

type Infrastructure struct {
	Labels      map[gatewayv1.LabelKey]gatewayv1.LabelValue           `json:"labels,omitempty"`
	Annotations map[gatewayv1.AnnotationKey]gatewayv1.AnnotationValue `json:"annotations,omitempty"`
	Status      mcgatewayv1.MinecraftServerDiscoveryStatus            `json:"config,omitempty"`
}

func GetInfrastructureByGateway(c client.Client, ctx context.Context, gw gatewayv1.Gateway) (Infrastructure, error) {
	log := logf.FromContext(ctx)
	var gwConfig *mcgatewayv1.MinecraftServerDiscovery
	if config, err := mfdiscovery.GetMinecraftServerDiscoveryByGateway(c, ctx, gw); err == nil {
		gwConfig = &config
	} else {
		log.Info(fmt.Sprintf("%e: no (valid) infrastructure config found", err))
	}
	class, err := GetGatewayClassByGateway(c, ctx, gw)
	if err != nil {
		return Infrastructure{}, err
	}
	var classConfig *mcgatewayv1.MinecraftServerDiscovery
	if config, err := mfdiscovery.GetMinecraftServerDiscoveryByRef(c, ctx, class.Spec.ParametersRef); err == nil {
		classConfig = &config
	} else {
		log.Info(fmt.Sprintf("%e: no (valid) class infrastructure config found", err))
	}
	status, err := merge(classConfig, gwConfig)
	if err != nil {
		return Infrastructure{}, err
	}
	return Infrastructure{
		Labels:      gw.Spec.Infrastructure.Labels,
		Annotations: gw.Spec.Infrastructure.Annotations,
		Status:      status,
	}, nil
}

func GetInfrastructureByClass(c client.Client, ctx context.Context, class gatewayv1.GatewayClass) (Infrastructure, error) {
	if config, err := mfdiscovery.GetMinecraftServerDiscoveryByRef(c, ctx, class.Spec.ParametersRef); err == nil {
		return Infrastructure{
			Status: config.Status,
		}, nil
	} else {
		return Infrastructure{}, err
	}
}

func merge(first *mcgatewayv1.MinecraftServerDiscovery, second *mcgatewayv1.MinecraftServerDiscovery) (mcgatewayv1.MinecraftServerDiscoveryStatus, error) {
	if first == nil && second == nil {
		return mcgatewayv1.MinecraftServerDiscoveryStatus{}, errors.New("no infrastructure provided")
	}
	if second == nil {
		return first.Status, nil
	}
	if first == nil {
		return second.Status, nil
	}
	// TODO: code actual merging
	return mcgatewayv1.MinecraftServerDiscoveryStatus{}, nil
}
