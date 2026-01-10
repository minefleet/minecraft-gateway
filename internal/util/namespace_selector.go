package util

import (
	"context"
	"errors"

	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"
	gatewayv1 "sigs.k8s.io/gateway-api/apis/v1"
)

func SelectNamespace(c client.Client, ctx context.Context, currentNs string, namespaces *gatewayv1.RouteNamespaces) ([]string, error) {
	if namespaces == nil {
		return []string{currentNs}, nil
	}

	from := gatewayv1.NamespacesFromSame
	if namespaces.From != nil {
		from = *namespaces.From
	}

	switch from {
	case gatewayv1.NamespacesFromNone:
		return nil, nil
	case gatewayv1.NamespacesFromSame:
		return []string{currentNs}, nil
	case gatewayv1.NamespacesFromAll:
		return []string{}, nil
	case gatewayv1.NamespacesFromSelector:
		if namespaces.Selector == nil {
			return nil, errors.New("invalid RouteNamespaces definition: no selector was specified")
		}
		selector, err := metav1.LabelSelectorAsSelector(namespaces.Selector)
		if err != nil {
			return nil, err
		}
		var list corev1.NamespaceList
		err = c.List(ctx, &list, client.MatchingLabelsSelector{
			Selector: selector,
		})
		if err != nil {
			return nil, err
		}
		result := make([]string, 0)
		for _, ns := range list.Items {
			result = append(result, ns.Name)
		}
		return result, nil
	}

	return nil, errors.New("invalid RouteNamespaces definition")
}
