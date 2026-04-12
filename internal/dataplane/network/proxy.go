package network

import (
	"context"
	"crypto/sha256"
	"encoding/json"
	"fmt"

	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/apimachinery/pkg/util/intstr"
	"k8s.io/apimachinery/pkg/util/strategicpatch"
	"k8s.io/utils/ptr"
	"minefleet.dev/minecraft-gateway/internal/version"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"
	gatewayv1 "sigs.k8s.io/gateway-api/apis/v1"

	mfgateway "minefleet.dev/minecraft-gateway/internal/gateway"
)

const (
	xdsServiceName      = "minecraft-gateway-network-xds"
	labelManagedBy      = "app.kubernetes.io/managed-by"
	labelManagedByValue = "minefleet-gateway"
	labelGatewayName    = "minefleet.dev/gateway-name"
	labelListener       = "minefleet.dev/listener"
)

// +kubebuilder:rbac:groups=apps,resources=deployments,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups="",resources=services,verbs=get;list;watch;create;update;patch;delete

// ProxyManager creates and deletes Velocity proxy Deployments to match the desired
// set of gateway listeners.
type ProxyManager struct {
	client client.Client
	cfg    Config
}

func NewProxyManager(c client.Client, cfg Config) *ProxyManager {
	return &ProxyManager{client: c, cfg: cfg}
}

// Sync ensures a proxy Deployment and Service exist for each listener, removing stale ones.
// Owner references on each Deployment point to the Gateway, so they are also
// garbage-collected automatically when the Gateway is deleted.
func (m *ProxyManager) Sync(ctx context.Context, gateway types.NamespacedName, listeners []gatewayv1.Listener, infra mfgateway.Infrastructure) error {
	gw := &gatewayv1.Gateway{}
	if err := m.client.Get(ctx, gateway, gw); err != nil {
		return fmt.Errorf("get gateway: %w", err)
	}

	desired := make(map[string]struct{}, len(listeners))
	for _, l := range listeners {
		desired[string(l.Name)] = struct{}{}
	}

	var existing appsv1.DeploymentList
	if err := m.client.List(ctx, &existing,
		client.InNamespace(gateway.Namespace),
		client.MatchingLabels{
			labelGatewayName: gateway.Name,
		},
	); err != nil {
		return fmt.Errorf("list proxy deployments: %w", err)
	}

	// Delete Deployments and Services whose listener is no longer present.
	for i := range existing.Items {
		dep := &existing.Items[i]
		if _, ok := desired[dep.Labels[labelListener]]; !ok {
			if err := m.client.Delete(ctx, dep); client.IgnoreNotFound(err) != nil {
				return fmt.Errorf("delete stale proxy deployment %s: %w", dep.Name, err)
			}
			svcName := proxyServiceName(gateway.Name, dep.Labels[labelListener])
			stale := &corev1.Service{ObjectMeta: metav1.ObjectMeta{Name: svcName, Namespace: gateway.Namespace}}
			if err := m.client.Delete(ctx, stale); client.IgnoreNotFound(err) != nil {
				return fmt.Errorf("delete stale proxy service %s: %w", svcName, err)
			}
		}
	}

	// Create a Deployment and Service for each desired listener that doesn't have one yet.
	for _, listener := range listeners {
		dep, err := m.buildDeployment(gateway, listener, gw, infra)
		if err != nil {
			return fmt.Errorf("build proxy deployment: %w", err)
		}
		var found appsv1.Deployment
		err = m.client.Get(ctx, types.NamespacedName{Name: dep.Name, Namespace: dep.Namespace}, &found)
		if errors.IsNotFound(err) {
			if createErr := m.client.Create(ctx, dep); createErr != nil {
				return fmt.Errorf("create proxy deployment %s: %w", dep.Name, createErr)
			}
		} else if err != nil {
			return fmt.Errorf("get proxy deployment %s: %w", dep.Name, err)
		} else if updateErr := m.client.Update(ctx, dep); updateErr != nil {
			return fmt.Errorf("update proxy deployment %s: %w", dep.Name, updateErr)
		}

		svc, err := m.buildService(gateway, listener, gw)
		if err != nil {
			return fmt.Errorf("build proxy service: %w", err)
		}
		var foundSvc corev1.Service
		err = m.client.Get(ctx, types.NamespacedName{Name: svc.Name, Namespace: svc.Namespace}, &foundSvc)
		if errors.IsNotFound(err) {
			if createErr := m.client.Create(ctx, svc); createErr != nil {
				return fmt.Errorf("create proxy service %s: %w", svc.Name, createErr)
			}
		} else if err != nil {
			return fmt.Errorf("get proxy service %s: %w", svc.Name, err)
		} else if updateErr := m.client.Update(ctx, svc); updateErr != nil {
			return fmt.Errorf("update proxy service %s: %w", svc.Name, updateErr)
		}
	}
	return nil
}

func (m *ProxyManager) buildDeployment(gateway types.NamespacedName, listener gatewayv1.Listener, gw *gatewayv1.Gateway, infra mfgateway.Infrastructure) (*appsv1.Deployment, error) {
	listenerName := string(listener.Name)
	name := proxyDeploymentName(gateway.Name, listenerName)
	xdsHost := fmt.Sprintf("%s.%s.svc.cluster.local", xdsServiceName, m.cfg.Namespace)

	selectorLabels := map[string]string{
		labelGatewayName: gateway.Name,
		labelListener:    listenerName,
	}
	objectLabels := make(map[string]string, len(selectorLabels)+1)
	for k, v := range selectorLabels {
		objectLabels[k] = v
	}
	objectLabels[labelManagedBy] = labelManagedByValue

	// Merge user-supplied labels/annotations from the Infrastructure into the
	// Deployment ObjectMeta. Managed labels are not overrideable.
	for k, v := range infra.Labels {
		if _, managed := objectLabels[string(k)]; !managed {
			objectLabels[string(k)] = string(v)
		}
	}
	var objectAnnotations map[string]string
	if len(infra.Annotations) > 0 {
		objectAnnotations = make(map[string]string, len(infra.Annotations))
		for k, v := range infra.Annotations {
			objectAnnotations[string(k)] = string(v)
		}
	}

	// Managed env vars injected into every proxy container.
	managedEnv := []corev1.EnvVar{
		{Name: "NAMESPACE", Value: gateway.Namespace},
		{Name: "GATEWAY_NAME", Value: gateway.Name},
		{Name: "LISTENER_NAME", Value: listenerName},
		{Name: "GATEWAY_NETWORK_XDS_HOST", Value: xdsHost},
		{Name: "GATEWAY_NETWORK_XDS_PORT", Value: fmt.Sprintf("%d", m.cfg.XDSPort)},
	}

	// Build base spec, then merge any user-provided DeploymentSpec on top.
	spec := appsv1.DeploymentSpec{
		Replicas: ptr.To(int32(1)),
		Selector: &metav1.LabelSelector{MatchLabels: selectorLabels},
		Template: corev1.PodTemplateSpec{
			ObjectMeta: metav1.ObjectMeta{Labels: selectorLabels},
			Spec: corev1.PodSpec{
				Containers: []corev1.Container{{
					Name:  "network",
					Env:   managedEnv,
					Image: fmt.Sprintf("minefleet.dev/minecraft-proxy:%s-velocity", version.Version),
					Ports: []corev1.ContainerPort{
						{
							Name:          "minecraft",
							ContainerPort: 25565,
							Protocol:      corev1.ProtocolTCP,
						},
					},
				}},
			},
		},
	}
	if infra.Config.Network != nil {
		if err := mergeDeploymentSpec(&spec, infra.Config.Network, selectorLabels, managedEnv); err != nil {
			return nil, err
		}
	}
	dep := &appsv1.Deployment{
		ObjectMeta: metav1.ObjectMeta{
			Name:        name,
			Namespace:   gateway.Namespace,
			Labels:      objectLabels,
			Annotations: objectAnnotations,
		},
		Spec: spec,
	}
	if err := controllerutil.SetControllerReference(gw, dep, m.client.Scheme()); err != nil {
		return nil, err
	}
	return dep, nil
}

// mergeDeploymentSpec applies override onto spec using a strategic merge patch.
// After merging, selectorLabels and managedEnv are re-enforced so they cannot
// be removed or replaced by user-supplied values.
func mergeDeploymentSpec(spec *appsv1.DeploymentSpec, override *appsv1.DeploymentSpec, selectorLabels map[string]string, managedEnv []corev1.EnvVar) error {
	baseJSON, err := json.Marshal(spec)
	if err != nil {
		return fmt.Errorf("marshal base deployment spec: %w", err)
	}
	overrideJSON, err := json.Marshal(override)
	if err != nil {
		return fmt.Errorf("marshal override deployment spec: %w", err)
	}
	mergedJSON, err := strategicpatch.StrategicMergePatch(baseJSON, overrideJSON, appsv1.DeploymentSpec{})
	if err != nil {
		return fmt.Errorf("strategic merge deployment spec: %w", err)
	}
	if err := json.Unmarshal(mergedJSON, spec); err != nil {
		return fmt.Errorf("unmarshal merged deployment spec: %w", err)
	}

	// Re-enforce immutable managed fields that the user cannot override.
	spec.Selector = &metav1.LabelSelector{MatchLabels: selectorLabels}
	for k, v := range selectorLabels {
		spec.Template.Labels[k] = v
	}
	managedEnvNames := make(map[string]struct{}, len(managedEnv))
	for _, e := range managedEnv {
		managedEnvNames[e.Name] = struct{}{}
	}
	filtered := spec.Template.Spec.Containers[0].Env[:0]
	for _, e := range spec.Template.Spec.Containers[0].Env {
		if _, managed := managedEnvNames[e.Name]; !managed {
			filtered = append(filtered, e)
		}
	}
	spec.Template.Spec.Containers[0].Env = append(filtered, managedEnv...)

	return nil
}

func (m *ProxyManager) buildService(gateway types.NamespacedName, listener gatewayv1.Listener, gw *gatewayv1.Gateway) (*corev1.Service, error) {
	listenerName := string(listener.Name)
	port := listener.Port

	svc := &corev1.Service{
		ObjectMeta: metav1.ObjectMeta{
			Name:      proxyServiceName(gateway.Name, listenerName),
			Namespace: gateway.Namespace,
			Labels: map[string]string{
				labelManagedBy:   labelManagedByValue,
				labelGatewayName: gateway.Name,
				labelListener:    listenerName,
			},
		},
		Spec: corev1.ServiceSpec{
			Selector: map[string]string{
				labelGatewayName: gateway.Name,
				labelListener:    listenerName,
			},
			Ports: []corev1.ServicePort{{
				Name:       "minecraft",
				Port:       port,
				TargetPort: intstr.IntOrString{Type: intstr.Int, IntVal: port},
			}},
		},
	}
	if err := controllerutil.SetControllerReference(gw, svc, m.client.Scheme()); err != nil {
		return nil, err
	}
	return svc, nil
}

// proxyDeploymentName returns a valid Kubernetes name for the proxy Deployment.
// If the natural name exceeds 63 characters it is replaced with a stable hash.
func proxyDeploymentName(gatewayName, listenerName string) string {
	name := fmt.Sprintf("network-%s-%s", gatewayName, listenerName)
	if len(name) > 63 {
		sum := sha256.Sum256([]byte(name))
		name = fmt.Sprintf("network-%x", sum[:4])
	}
	return name
}

// proxyServiceName returns the Service name for a proxy, matching the DNS pattern
// {listenerName}-{gatewayName} expected by the edge dataplane.
func proxyServiceName(gatewayName, listenerName string) string {
	name := fmt.Sprintf("%s-%s", listenerName, gatewayName)
	if len(name) > 63 {
		sum := sha256.Sum256([]byte(name))
		name = fmt.Sprintf("proxy-%x", sum[:4])
	}
	return name
}
