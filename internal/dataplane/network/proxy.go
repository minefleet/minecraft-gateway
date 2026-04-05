package network

import (
	"context"
	"crypto/sha256"
	"fmt"

	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/apimachinery/pkg/util/intstr"
	"k8s.io/utils/ptr"
	"sigs.k8s.io/controller-runtime/pkg/client"
	gatewayv1 "sigs.k8s.io/gateway-api/apis/v1"
)

const (
	xdsServiceName      = "minecraft-gateway-network-xds"
	labelManagedBy      = "app.kubernetes.io/managed-by"
	labelManagedByValue = "minefleet-gateway"
	labelGatewayNS      = "minefleet.dev/gateway-namespace"
	labelGatewayName    = "minefleet.dev/gateway-name"
	labelListener       = "minefleet.dev/listener"
	gatewayKind         = "Gateway"
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
func (m *ProxyManager) Sync(ctx context.Context, gateway types.NamespacedName, listeners []gatewayv1.Listener) error {
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
			labelGatewayNS:   gateway.Namespace,
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
		dep := m.buildDeployment(gateway, listener, gw)
		var found appsv1.Deployment
		err := m.client.Get(ctx, types.NamespacedName{Name: dep.Name, Namespace: dep.Namespace}, &found)
		if errors.IsNotFound(err) {
			if createErr := m.client.Create(ctx, dep); createErr != nil {
				return fmt.Errorf("create proxy deployment %s: %w", dep.Name, createErr)
			}
		} else if err != nil {
			return fmt.Errorf("get proxy deployment %s: %w", dep.Name, err)
		}

		svc := m.buildService(gateway, listener, gw)
		var foundSvc corev1.Service
		err = m.client.Get(ctx, types.NamespacedName{Name: svc.Name, Namespace: svc.Namespace}, &foundSvc)
		if errors.IsNotFound(err) {
			if createErr := m.client.Create(ctx, svc); createErr != nil {
				return fmt.Errorf("create proxy service %s: %w", svc.Name, createErr)
			}
		} else if err != nil {
			return fmt.Errorf("get proxy service %s: %w", svc.Name, err)
		}
	}
	return nil
}

// Delete removes all proxy Deployments and Services for the given gateway.
func (m *ProxyManager) Delete(ctx context.Context, gateway types.NamespacedName) error {
	var existing appsv1.DeploymentList
	if err := m.client.List(ctx, &existing,
		client.InNamespace(gateway.Namespace),
		client.MatchingLabels{
			labelGatewayNS:   gateway.Namespace,
			labelGatewayName: gateway.Name,
		},
	); err != nil {
		return fmt.Errorf("list proxy deployments for deletion: %w", err)
	}
	for i := range existing.Items {
		dep := &existing.Items[i]
		if err := m.client.Delete(ctx, dep); client.IgnoreNotFound(err) != nil {
			return fmt.Errorf("delete proxy deployment %s: %w", dep.Name, err)
		}
		svcName := proxyServiceName(gateway.Name, dep.Labels[labelListener])
		stale := &corev1.Service{ObjectMeta: metav1.ObjectMeta{Name: svcName, Namespace: gateway.Namespace}}
		if err := m.client.Delete(ctx, stale); client.IgnoreNotFound(err) != nil {
			return fmt.Errorf("delete proxy service %s: %w", svcName, err)
		}
	}
	return nil
}

func (m *ProxyManager) buildDeployment(gateway types.NamespacedName, listener gatewayv1.Listener, gw *gatewayv1.Gateway) *appsv1.Deployment {
	listenerName := string(listener.Name)
	name := proxyDeploymentName(gateway.Name, listenerName)
	xdsHost := fmt.Sprintf("%s.%s.svc.cluster.local", xdsServiceName, m.cfg.Namespace)

	selectorLabels := map[string]string{
		labelGatewayNS:   gateway.Namespace,
		labelGatewayName: gateway.Name,
		labelListener:    listenerName,
	}
	objectLabels := make(map[string]string, len(selectorLabels)+1)
	for k, v := range selectorLabels {
		objectLabels[k] = v
	}
	objectLabels[labelManagedBy] = labelManagedByValue

	return &appsv1.Deployment{
		ObjectMeta: metav1.ObjectMeta{
			Name:      name,
			Namespace: gateway.Namespace,
			Labels:    objectLabels,
			OwnerReferences: []metav1.OwnerReference{{
				APIVersion:         gatewayv1.GroupVersion.Group + "/" + gatewayv1.GroupVersion.Version,
				Kind:               gatewayKind,
				Name:               gw.Name,
				UID:                gw.UID,
				BlockOwnerDeletion: ptr.To(true),
				Controller:         ptr.To(true),
			}},
		},
		Spec: appsv1.DeploymentSpec{
			Replicas: ptr.To(int32(1)),
			Selector: &metav1.LabelSelector{MatchLabels: selectorLabels},
			Template: corev1.PodTemplateSpec{
				ObjectMeta: metav1.ObjectMeta{Labels: selectorLabels},
				Spec: corev1.PodSpec{
					Containers: []corev1.Container{{
						Name:  "proxy",
						Image: m.cfg.ProxyImage,
						Env: []corev1.EnvVar{
							{Name: "NAMESPACE", Value: gateway.Namespace},
							{Name: "GATEWAY_NAME", Value: gateway.Name},
							{Name: "LISTENER_NAME", Value: listenerName},
							{Name: "GATEWAY_NETWORK_XDS_HOST", Value: xdsHost},
							{Name: "GATEWAY_NETWORK_XDS_PORT", Value: fmt.Sprintf("%d", m.cfg.XDSPort)},
						},
					}},
				},
			},
		},
	}
}

func (m *ProxyManager) buildService(gateway types.NamespacedName, listener gatewayv1.Listener, gw *gatewayv1.Gateway) *corev1.Service {
	listenerName := string(listener.Name)
	port := listener.Port

	selectorLabels := map[string]string{
		labelGatewayNS:   gateway.Namespace,
		labelGatewayName: gateway.Name,
		labelListener:    listenerName,
	}

	return &corev1.Service{
		ObjectMeta: metav1.ObjectMeta{
			Name:      proxyServiceName(gateway.Name, listenerName),
			Namespace: gateway.Namespace,
			Labels: map[string]string{
				labelManagedBy:   labelManagedByValue,
				labelGatewayNS:   gateway.Namespace,
				labelGatewayName: gateway.Name,
				labelListener:    listenerName,
			},
			OwnerReferences: []metav1.OwnerReference{{
				APIVersion:         gatewayv1.GroupVersion.Group + "/" + gatewayv1.GroupVersion.Version,
				Kind:               gatewayKind,
				Name:               gw.Name,
				UID:                gw.UID,
				BlockOwnerDeletion: ptr.To(true),
				Controller:         ptr.To(true),
			}},
		},
		Spec: corev1.ServiceSpec{
			Selector: selectorLabels,
			Ports: []corev1.ServicePort{{
				Name:       "minecraft",
				Port:       port,
				TargetPort: intstr.IntOrString{Type: intstr.Int, IntVal: port},
			}},
		},
	}
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
