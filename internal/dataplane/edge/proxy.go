package edge

import (
	"context"
	"fmt"

	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

const (
	daemonSetName  = "minecraft-gateway-edge"
	xdsServiceName = "minecraft-gateway-edge-xds"
)

type proxyManager struct {
	client client.Client
	cfg    Config
}

func newProxyManager(c client.Client, cfg Config) *proxyManager {
	return &proxyManager{client: c, cfg: cfg}
}

// CheckHealth verifies the edge DaemonSet and xDS Service are present and healthy.
// Returns an error describing what is wrong so the caller can surface it on the Gateway status.
func (m *proxyManager) CheckHealth(ctx context.Context) error {
	if err := m.checkDaemonSet(ctx); err != nil {
		return err
	}
	return m.checkXDSService(ctx)
}

// +kubebuilder:rbac:groups=apps,resources=daemonsets,verbs=get;list;watch

func (m *proxyManager) checkDaemonSet(ctx context.Context) error {
	ds := &appsv1.DaemonSet{}
	if err := m.client.Get(ctx, types.NamespacedName{Name: daemonSetName, Namespace: m.cfg.Namespace}, ds); err != nil {
		return fmt.Errorf("edge DaemonSet %q not found in namespace %q — were the edge manifests applied?: %w",
			daemonSetName, m.cfg.Namespace, err)
	}

	desired := ds.Status.DesiredNumberScheduled
	if desired == 0 {
		return fmt.Errorf("edge DaemonSet %q has no scheduled pods — no nodes may be available", daemonSetName)
	}
	if ds.Status.NumberReady != desired {
		return fmt.Errorf("edge DaemonSet %q not fully ready: %d/%d pods ready",
			daemonSetName, ds.Status.NumberReady, desired)
	}
	return nil
}

func (m *proxyManager) checkXDSService(ctx context.Context) error {
	svc := &corev1.Service{}
	if err := m.client.Get(ctx, types.NamespacedName{Name: xdsServiceName, Namespace: m.cfg.Namespace}, svc); err != nil {
		return fmt.Errorf("xDS Service %q not found in namespace %q — were the manager manifests applied?: %w",
			xdsServiceName, m.cfg.Namespace, err)
	}
	return nil
}
