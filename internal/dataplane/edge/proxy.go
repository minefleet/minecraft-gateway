package edge

import (
	"context"
	"encoding/json"
	"fmt"

	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/intstr"
	"k8s.io/apimachinery/pkg/util/strategicpatch"
	"k8s.io/utils/ptr"
	v1alpha1 "minefleet.dev/minecraft-gateway/api/controller/v1alpha1"
	"minefleet.dev/minecraft-gateway/internal/version"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"
)

const (
	bootstrapConfigMapName = "edge-bootstrap"
	edgeDaemonSetName      = "minecraft-gateway-edge"

	edgeLabelName         = "app.kubernetes.io/name"
	edgeLabelNameValue    = "minecraft-edge"
	edgeLabelControlPlane = "control-plane"
	edgeControlPlaneValue = "controller-manager"

	bootstrapVolumeName  = "edge-bootstrap-volume"
	bootstrapMountPath   = "/etc/envoy/bootstrap.yaml"
	bootstrapMountSubKey = "bootstrap.yaml"
)

// +kubebuilder:rbac:groups=apps,resources=daemonsets,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups="",resources=configmaps,verbs=get;list;watch;create;update;patch;delete

// ProxyManager reconciles the edge DaemonSet and bootstrap ConfigMap.
type ProxyManager struct {
	client client.Client
	cfg    Config
}

func NewProxyManager(c client.Client, cfg Config) *ProxyManager {
	return &ProxyManager{client: c, cfg: cfg}
}

// SyncBootstrap creates or updates the Envoy bootstrap ConfigMap with the
// controller's own pod IP as the xDS endpoint. Called once at startup.
func (m *ProxyManager) SyncBootstrap(ctx context.Context) error {
	cm := &corev1.ConfigMap{
		ObjectMeta: metav1.ObjectMeta{
			Name:      bootstrapConfigMapName,
			Namespace: m.cfg.Namespace,
		},
	}
	_, err := controllerutil.CreateOrUpdate(ctx, m.client, cm, func() error {
		cm.Data = map[string]string{
			bootstrapMountSubKey: m.generateBootstrap(),
		}
		return nil
	})
	return err
}

// SyncDaemonSet creates or updates the edge DaemonSet, merging the user-supplied
// EdgeSpec.DaemonSet template on top of the controller-managed default.
func (m *ProxyManager) SyncDaemonSet(ctx context.Context, edge *v1alpha1.EdgeSpec) error {
	spec, err := m.buildDaemonSetSpec(edge)
	if err != nil {
		return err
	}

	selectorLabels := edgeSelectorLabels()
	ds := &appsv1.DaemonSet{
		ObjectMeta: metav1.ObjectMeta{
			Name:      edgeDaemonSetName,
			Namespace: m.cfg.Namespace,
		},
	}
	_, err = controllerutil.CreateOrUpdate(ctx, m.client, ds, func() error {
		ds.Spec = spec
		if ds.Labels == nil {
			ds.Labels = make(map[string]string)
		}
		for k, v := range selectorLabels {
			ds.Labels[k] = v
		}
		return nil
	})
	return err
}

func (m *ProxyManager) buildDaemonSetSpec(edge *v1alpha1.EdgeSpec) (appsv1.DaemonSetSpec, error) {
	spec := m.defaultDaemonSetSpec()
	if edge == nil || edge.DaemonSet == nil {
		return spec, nil
	}

	baseJSON, err := json.Marshal(spec)
	if err != nil {
		return appsv1.DaemonSetSpec{}, fmt.Errorf("marshal base daemonset spec: %w", err)
	}
	overrideJSON, err := json.Marshal(&edge.DaemonSet)
	if err != nil {
		return appsv1.DaemonSetSpec{}, fmt.Errorf("marshal override daemonset spec: %w", err)
	}
	mergedJSON, err := strategicpatch.StrategicMergePatch(baseJSON, overrideJSON, appsv1.DaemonSetSpec{})
	if err != nil {
		return appsv1.DaemonSetSpec{}, fmt.Errorf("strategic merge daemonset spec: %w", err)
	}
	if err := json.Unmarshal(mergedJSON, &spec); err != nil {
		return appsv1.DaemonSetSpec{}, fmt.Errorf("unmarshal merged daemonset spec: %w", err)
	}

	// Re-enforce immutable managed fields.
	selectorLabels := edgeSelectorLabels()
	spec.Selector = &metav1.LabelSelector{MatchLabels: selectorLabels}
	for k, v := range selectorLabels {
		spec.Template.Labels[k] = v
	}
	m.enforceBootstrapMount(&spec)

	return spec, nil
}

// enforceBootstrapMount ensures the bootstrap volume and its mount into the
// "edge" container are always present, regardless of user overrides.
func (m *ProxyManager) enforceBootstrapMount(spec *appsv1.DaemonSetSpec) {
	hasVol := false
	for _, v := range spec.Template.Spec.Volumes {
		if v.Name == bootstrapVolumeName {
			hasVol = true
			break
		}
	}
	if !hasVol {
		spec.Template.Spec.Volumes = append(spec.Template.Spec.Volumes, bootstrapVolume())
	}

	for i, c := range spec.Template.Spec.Containers {
		if c.Name != "edge" {
			continue
		}
		hasMount := false
		for _, vm := range c.VolumeMounts {
			if vm.Name == bootstrapVolumeName {
				hasMount = true
				break
			}
		}
		if !hasMount {
			spec.Template.Spec.Containers[i].VolumeMounts = append(
				spec.Template.Spec.Containers[i].VolumeMounts,
				bootstrapVolumeMount(),
			)
		}
		return
	}
}

func (m *ProxyManager) defaultDaemonSetSpec() appsv1.DaemonSetSpec {
	selectorLabels := edgeSelectorLabels()
	return appsv1.DaemonSetSpec{
		Selector: &metav1.LabelSelector{MatchLabels: selectorLabels},
		Template: corev1.PodTemplateSpec{
			ObjectMeta: metav1.ObjectMeta{Labels: selectorLabels},
			Spec: corev1.PodSpec{
				SecurityContext: &corev1.PodSecurityContext{
					RunAsNonRoot: ptr.To(true),
					SeccompProfile: &corev1.SeccompProfile{
						Type: corev1.SeccompProfileTypeRuntimeDefault,
					},
				},
				Containers: []corev1.Container{{
					Name:    "edge",
					Image:   fmt.Sprintf("ghcr.io/minefleet/minecraft-edge:%s", version.Version),
					Command: []string{"envoy"},
					Args:    []string{"-c", bootstrapMountPath, "--log-level", "info"},
					SecurityContext: &corev1.SecurityContext{
						AllowPrivilegeEscalation: ptr.To(false),
						Capabilities:             &corev1.Capabilities{Drop: []corev1.Capability{"ALL"}},
					},
					Ports: []corev1.ContainerPort{
						{Name: "minecraft", ContainerPort: 25565, HostPort: 25565, Protocol: corev1.ProtocolTCP},
						{Name: "admin", ContainerPort: 9901, Protocol: corev1.ProtocolTCP},
					},
					LivenessProbe: &corev1.Probe{
						ProbeHandler:        corev1.ProbeHandler{HTTPGet: &corev1.HTTPGetAction{Path: "/ready", Port: intstr.FromInt32(9901)}},
						InitialDelaySeconds: 10,
						PeriodSeconds:       20,
					},
					ReadinessProbe: &corev1.Probe{
						ProbeHandler:        corev1.ProbeHandler{HTTPGet: &corev1.HTTPGetAction{Path: "/ready", Port: intstr.FromInt32(9901)}},
						InitialDelaySeconds: 5,
						PeriodSeconds:       10,
					},
					VolumeMounts: []corev1.VolumeMount{bootstrapVolumeMount()},
				}},
				Volumes: []corev1.Volume{bootstrapVolume()},
			},
		},
	}
}

func (m *ProxyManager) generateBootstrap() string {
	return fmt.Sprintf(`admin:
  address:
    socket_address:
      address: 0.0.0.0
      port_value: 9901

node:
  id: %s
  cluster: minefleet-gateway

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
      type: STATIC
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
`, NodeID, m.cfg.PodIP, m.cfg.XDSPort)
}

func edgeSelectorLabels() map[string]string {
	return map[string]string{
		edgeLabelControlPlane: edgeControlPlaneValue,
		edgeLabelName:         edgeLabelNameValue,
	}
}

func bootstrapVolume() corev1.Volume {
	return corev1.Volume{
		Name: bootstrapVolumeName,
		VolumeSource: corev1.VolumeSource{
			ConfigMap: &corev1.ConfigMapVolumeSource{
				LocalObjectReference: corev1.LocalObjectReference{Name: bootstrapConfigMapName},
				Items:                []corev1.KeyToPath{{Key: bootstrapMountSubKey, Path: bootstrapMountSubKey}},
			},
		},
	}
}

func bootstrapVolumeMount() corev1.VolumeMount {
	return corev1.VolumeMount{
		Name:      bootstrapVolumeName,
		MountPath: bootstrapMountPath,
		SubPath:   bootstrapMountSubKey,
	}
}
