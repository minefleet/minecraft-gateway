package discovery

import (
	"context"
	mcgatewayv1 "minefleet.dev/minecraft-gateway/api/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

type EndpointDiscoverer struct {
}

func NewEndpointDiscoverer(c client.Client, ctx context.Context) EndpointDiscoverer {

}

func (e *EndpointDiscoverer) discoverBy(route mcgatewayv1.MinecraftRoute) {

}
