package endpoint

import (
	"context"
	"errors"
	corev1 "k8s.io/api/core/v1"
	discoveryv1 "k8s.io/api/discovery/v1"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

func GetEndpointSlicesByService(c client.Client, ctx context.Context, svc corev1.Service) ([]discoveryv1.EndpointSlice, error) {
	var slice discoveryv1.EndpointSliceList
	if err := c.List(ctx, &slice,
		client.InNamespace(svc.Namespace),
		client.MatchingLabels{"kubernetes.io/service-name": svc.Name},
	); err != nil {
		return nil, err
	}
	return slice.Items, nil
}

func GetServiceByEndpointSlice(c client.Client, ctx context.Context, slice discoveryv1.EndpointSlice) (corev1.Service, error) {
	var svc corev1.Service
	svcName, ok := slice.Labels[discoveryv1.LabelServiceName]
	if !ok {
		return corev1.Service{}, errors.New("no service name label found")
	}
	err := c.Get(
		ctx,
		types.NamespacedName{
			Namespace: slice.Namespace,
			Name:      svcName,
		},
		&svc,
	)
	if err != nil {
		return corev1.Service{}, err
	}

	return svc, nil
}
