package discovery

import (
	discoveryv1 "k8s.io/api/discovery/v1"
	"minefleet.dev/minecraft-gateway/internal/route"
)

func GetRoutesByEndpointSlice(slice discoveryv1.EndpointSlice) route.Bag {
	svc := slice.Labels[discoveryv1.LabelServiceName]
	svcKey := slice.Namespace + "/" + svc
	_ = svc
	_ = svcKey
	return route.Bag{}
}
