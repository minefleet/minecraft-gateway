package network

import (
	"context"

	"sigs.k8s.io/controller-runtime/pkg/client"
)

func StartADS(ctx context.Context, snapshots <-chan Snapshot, cfg Config, c client.Client) {

}
