package edge

import (
	"context"
	"fmt"

	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

const (
	daemonSetName  = "minecraft-gateway-edge"
	configMapName  = "minecraft-gateway-edge-bootstrap"
	componentLabel = "app.kubernetes.io/component"
	componentValue = "minecraft-gateway-edge"
)

type proxyManager struct {
	client client.Client
	cfg    ProxyConfig
}

func newProxyManager(c client.Client, cfg ProxyConfig) *proxyManager {
	return &proxyManager{client: c, cfg: cfg}
}

// EnsureResources creates or updates the bootstrap ConfigMap and the edge DaemonSet.
func (m *proxyManager) EnsureResources(ctx context.Context) error {
	if err := m.ensureConfigMap(ctx); err != nil {
		return fmt.Errorf("ensure configmap: %w", err)
	}
	if err := m.ensureDaemonSet(ctx); err != nil {
		return fmt.Errorf("ensure daemonset: %w", err)
	}
	return nil
}

func (m *proxyManager) ensureConfigMap(ctx context.Context) error {
	desired := m.bootstrapConfigMap()
	existing := &corev1.ConfigMap{}
	err := m.client.Get(ctx, types.NamespacedName{Name: configMapName, Namespace: m.cfg.Namespace}, existing)
	if apierrors.IsNotFound(err) {
		return m.client.Create(ctx, desired)
	}
	if err != nil {
		return err
	}
	existing.Data = desired.Data
	return m.client.Update(ctx, existing)
}

func (m *proxyManager) ensureDaemonSet(ctx context.Context) error {
	desired := m.daemonSet()
	existing := &appsv1.DaemonSet{}
	err := m.client.Get(ctx, types.NamespacedName{Name: daemonSetName, Namespace: m.cfg.Namespace}, existing)
	if apierrors.IsNotFound(err) {
		return m.client.Create(ctx, desired)
	}
	if err != nil {
		return err
	}
	existing.Spec = desired.Spec
	return m.client.Update(ctx, existing)
}

func (m *proxyManager) bootstrapConfigMap() *corev1.ConfigMap {
	return &corev1.ConfigMap{
		ObjectMeta: metav1.ObjectMeta{
			Name:      configMapName,
			Namespace: m.cfg.Namespace,
			Labels:    map[string]string{componentLabel: componentValue},
		},
		Data: map[string]string{
			"bootstrap.yaml": m.bootstrapYAML(),
		},
	}
}

// bootstrapYAML renders the Envoy bootstrap config that points at the xDS gRPC server.
// The DaemonSet pods connect to XDSHost:XDSPort via the xds_cluster and subscribe to
// LDS + CDS via ADS.
func (m *proxyManager) bootstrapYAML() string {
	return fmt.Sprintf(`node:
  id: %s
  cluster: %s

dynamic_resources:
  ads_config:
    api_type: GRPC
    transport_api_version: V3
    grpc_services:
      - envoy_grpc:
          cluster_name: xds_cluster
  cds_config:
    resource_api_version: V3
    ads: {}
  lds_config:
    resource_api_version: V3
    ads: {}

static_resources:
  clusters:
    - name: xds_cluster
      type: STRICT_DNS
      connect_timeout: 5s
      typed_extension_protocol_options:
        envoy.extensions.upstreams.http.v3.HttpProtocolOptions:
          "@type": type.googleapis.com/envoy.extensions.upstreams.http.v3.HttpProtocolOptions
          explicit_http_config:
            http2_protocol_options: {}
      load_assignment:
        cluster_name: xds_cluster
        endpoints:
          - lb_endpoints:
              - endpoint:
                  address:
                    socket_address:
                      address: %s
                      port_value: %d
`, NodeID, nodeCluster, m.cfg.XDSHost, m.cfg.XDSPort)
}

func (m *proxyManager) daemonSet() *appsv1.DaemonSet {
	labels := map[string]string{componentLabel: componentValue}
	return &appsv1.DaemonSet{
		ObjectMeta: metav1.ObjectMeta{
			Name:      daemonSetName,
			Namespace: m.cfg.Namespace,
			Labels:    labels,
		},
		Spec: appsv1.DaemonSetSpec{
			Selector: &metav1.LabelSelector{MatchLabels: labels},
			Template: corev1.PodTemplateSpec{
				ObjectMeta: metav1.ObjectMeta{Labels: labels},
				Spec: corev1.PodSpec{
					Containers: []corev1.Container{
						{
							Name:    "envoy",
							Image:   m.cfg.Image,
							Command: []string{"-c", "/etc/envoy/bootstrap.yaml", "--log-level", "info"},
							Ports: []corev1.ContainerPort{
								{
									Name:          "minecraft",
									ContainerPort: 25565,
									HostPort:      25565,
									Protocol:      corev1.ProtocolTCP,
								},
							},
							Env: []corev1.EnvVar{
								{Name: "ENVOY_DYNAMIC_MODULES_SEARCH_PATH", Value: "/usr/lib"},
							},
							VolumeMounts: []corev1.VolumeMount{
								{Name: "bootstrap", MountPath: "/etc/envoy", ReadOnly: true},
							},
						},
					},
					Volumes: []corev1.Volume{
						{
							Name: "bootstrap",
							VolumeSource: corev1.VolumeSource{
								ConfigMap: &corev1.ConfigMapVolumeSource{
									LocalObjectReference: corev1.LocalObjectReference{Name: configMapName},
								},
							},
						},
					},
				},
			},
		},
	}
}
