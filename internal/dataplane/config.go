package dataplane

import (
	"minefleet.dev/minecraft-gateway/internal/dataplane/edge"
	"minefleet.dev/minecraft-gateway/internal/dataplane/network"
)

type Config struct {
	Edge    edge.Config
	Network network.Config
}
