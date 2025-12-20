package endpoint

import (
	"context"
	"fmt"
	corev1 "k8s.io/api/core/v1"
	discoveryv1 "k8s.io/api/discovery/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/manager"
)

const (
	IndexEndpointsByService = "endpoints.byService"
)

func IndexServiceByName(mgr manager.Manager) error {
	ctx := context.Background()
	if err := mgr.GetFieldIndexer().IndexField(ctx, &corev1.Service{}, IndexEndpointsByService,
		func(o client.Object) []string {
			svc, ok := o.(*corev1.Service)
			if !ok {
				return nil
			}
			return []string{svc.Name}
		},
	); err != nil {
		return err
	}
	return nil
}

func GetServiceByEndpointSlice(c client.Client, ctx context.Context, slice discoveryv1.EndpointSlice) (corev1.Service, error) {
	var svcList corev1.ServiceList
	svcName, ok := slice.Labels[discoveryv1.LabelServiceName]
	if !ok {
		return corev1.Service{}, nil
	}
	err := c.List(
		ctx,
		&svcList,
		client.InNamespace(slice.Namespace),
		client.MatchingFields{
			IndexEndpointsByService: svcName,
		},
	)
	if err != nil {
		return corev1.Service{}, err
	}

	if len(svcList.Items) == 0 {
		return corev1.Service{}, fmt.Errorf("no Service found for EndpointSlice %s", slice.Name)
	}

	return svcList.Items[0], nil
}
