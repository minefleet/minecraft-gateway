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
	infrastructure, err := merge(classConfig, gwConfig)
	if err != nil {
		return Infrastructure{}, err
	}
	return infrastructure, nil
}

func GetInfrastructureByGateway(c client.Client, ctx context.Context, gateway gatewayv1.Gateway) (Infrastructure, error) {
	if config, err := mfdiscovery.GetMinecraftServerDiscoveryByGateway(c, ctx, gateway); err == nil {
		return Infrastructure{
			Status: config.Status,
		}, nil
	} else {
		return Infrastructure{}, err
	}
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

func merge(first *Infrastructure, second *Infrastructure) (Infrastructure, error) {
	if first == nil && second == nil {
		return Infrastructure{}, errors.New("no infrastructure provided")
	}
	if second == nil {
		return *first, nil
	}
	if first == nil {
		return *second, nil
	}
	// TODO: code actual merging
	return Infrastructure{}, nil
}
