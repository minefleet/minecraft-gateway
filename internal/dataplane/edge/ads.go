package edge

import (
	"context"

	"sigs.k8s.io/controller-runtime/pkg/client"
	logf "sigs.k8s.io/controller-runtime/pkg/log"
)

var log = logf.Log.WithName("edge")

// StartADS starts three goroutines:
//   - the xDS gRPC server (ADS) on cfg.XDSPort
//   - a one-shot health check verifying the edge DaemonSet and xDS Service are present
//   - a loop that applies incoming DomainSnapshots to the xDS cache
func StartADS(ctx context.Context, snapshots <-chan DomainSnapshot, cfg ProxyConfig, c client.Client) {
	xds := newXDSServer(ctx)

	// Serve xDS over gRPC.
	go func() {
		if err := xds.Start(ctx, cfg.XDSPort); err != nil && ctx.Err() == nil {
			log.Error(err, "xDS server stopped unexpectedly")
		}
	}()

	// Verify edge infrastructure is present and healthy.
	pm := newProxyManager(c, cfg)
	go func() {
		if err := pm.CheckHealth(ctx); err != nil {
			log.Error(err, "edge proxy health check failed")
		}
	}()

	// Feed snapshots into the xDS cache.
	go func() {
		for {
			select {
			case <-ctx.Done():
				return
			case snap, ok := <-snapshots:
				if !ok {
					return
				}
				if err := xds.UpdateSnapshot(ctx, snap); err != nil {
					log.Error(err, "failed to update xDS snapshot")
				}
			}
		}
	}()
}
