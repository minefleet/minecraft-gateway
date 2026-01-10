package util

import (
	"context"

	"k8s.io/apimachinery/pkg/labels"
	mcgatewayv1 "minefleet.dev/minecraft-gateway/api/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

func FetchFallbackRoutes(c client.Client, ctx context.Context, namespaces []string, selector labels.Selector) ([]mcgatewayv1.MinecraftRoute, error) {
	result := make([]mcgatewayv1.MinecraftRoute, 0)
	for _, ns := range namespaces {
		var list mcgatewayv1.MinecraftFallbackRouteList
		if err := c.List(ctx, &list, client.InNamespace(ns), client.MatchingLabelsSelector{
			Selector: selector,
		}); err != nil {
			return nil, err
		}
		for _, i := range list.Items {
			result = append(result, i.Spec.MinecraftRoute)
		}
	}
	return result, nil
}

func FetchJoinRoutes(c client.Client, ctx context.Context, namespaces []string, selector labels.Selector) ([]mcgatewayv1.MinecraftRoute, error) {
	result := make([]mcgatewayv1.MinecraftRoute, 0)
	for _, ns := range namespaces {
		var list mcgatewayv1.MinecraftJoinRouteList
		if err := c.List(ctx, &list, client.InNamespace(ns), client.MatchingLabelsSelector{
			Selector: selector,
		}); err != nil {
			return nil, err
		}
		for _, i := range list.Items {
			result = append(result, i.Spec.MinecraftRoute)
		}
	}
	return result, nil
}
