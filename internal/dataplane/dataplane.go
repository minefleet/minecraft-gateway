package dataplane

import (
	corev1 "k8s.io/api/core/v1"
	mcgatewayv1 "minefleet.dev/minecraft-gateway/api/v1"
)

type Dataplane struct {
	Data []mcgatewayv1.MinecraftService
}

func (d Dataplane) SyncRoute(route mcgatewayv1.MinecraftRoute) {

}

func (d Dataplane) SyncPlayers(service corev1.Service, server mcgatewayv1.MinecraftServer) {

}

func (d Dataplane) SyncService(service corev1.Service) {

}
