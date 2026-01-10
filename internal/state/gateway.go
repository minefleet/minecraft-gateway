package state

import (
	"context"

	"k8s.io/apimachinery/pkg/types"
	mcgatewayv1 "minefleet.dev/minecraft-gateway/api/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

type GatewayState struct {
	Checksum string
	Version  uint64
	State    []mcgatewayv1.MinecraftService
}

func NewGatewayState(c client.Client, ctx context.Context, gw types.NamespacedName, lastVersion uint64) GatewayState {
	return GatewayState{}
}
