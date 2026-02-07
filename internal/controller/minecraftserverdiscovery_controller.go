/*
Copyright 2025.

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

package controller

import (
	"context"
	"fmt"

	corev1 "k8s.io/api/core/v1"
	discoveryv1 "k8s.io/api/discovery/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/utils/ptr"
	mcgatewayv1 "minefleet.dev/minecraft-gateway/api/v1"
	mfdiscovery "minefleet.dev/minecraft-gateway/internal/discovery"
	"minefleet.dev/minecraft-gateway/internal/endpoint"
	"minefleet.dev/minecraft-gateway/internal/gateway"
	"minefleet.dev/minecraft-gateway/internal/util"
	"sigs.k8s.io/controller-runtime/pkg/handler"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"
	gatewayv1 "sigs.k8s.io/gateway-api/apis/v1"

	"k8s.io/apimachinery/pkg/runtime"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	logf "sigs.k8s.io/controller-runtime/pkg/log"
)

// MinecraftServerDiscoveryReconciler reconciles a MinecraftServerDiscovery object
type MinecraftServerDiscoveryReconciler struct {
	client.Client
	Scheme *runtime.Scheme
}

// +kubebuilder:rbac:groups=gateway.networking.minefleet.dev,resources=minecraftserverdiscoveries,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups=gateway.networking.minefleet.dev,resources=minecraftserverdiscoveries/status,verbs=get;update;patch
// +kubebuilder:rbac:groups=gateway.networking.minefleet.dev,resources=minecraftserverdiscoveries/finalizers,verbs=update
// +kubebuilder:rbac:groups=discovery.k8s.io,resources=endpointslices,verbs=get;list;watch
// +kubebuilder:rbac:groups=gateway.networking.k8s.io,resources=gateways,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups=gateway.networking.k8s.io,resources=gateways/status,verbs=get;update;patch
// +kubebuilder:rbac:groups=gateway.networking.k8s.io,resources=gateways/finalizers,verbs=update

// Reconcile is part of the main kubernetes reconciliation loop which aims to
// move the current state of the cluster closer to the desired state.
// TODO(user): Modify the Reconcile function to compare the state specified by
// the MinecraftServerDiscovery object against the actual cluster state, and then
// perform operations to make the cluster state reflect the state specified by
// the user.
//
// For more details, check Reconcile and its Result here:
// - https://pkg.go.dev/sigs.k8s.io/controller-runtime@v0.21.0/pkg/reconcile
func (r *MinecraftServerDiscoveryReconciler) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
	log := logf.FromContext(ctx)

	var discovery mcgatewayv1.MinecraftServerDiscovery
	if err := r.Get(ctx, req.NamespacedName, &discovery); err != nil {
		return ctrl.Result{}, client.IgnoreNotFound(err)
	}
	var gws gatewayv1.GatewayList
	if err := gateway.ListGatewaysByInfrastructure(r.Client, ctx, &gws, discovery); err != nil {
		return ctrl.Result{}, client.IgnoreNotFound(err)
	}
	// TODO: add gateway class validation when this becomes standard channel
	if len(gws.Items) == 0 {
		log.Info("will not reconcile because no gateways are connected", "discovery", fmt.Sprintf("%s/%s", discovery.Namespace, discovery.Name))
		return ctrl.Result{}, nil
	}

	services, err := r.getServices(ctx, discovery)
	if err != nil {
		return ctrl.Result{}, err
	}
	backends := make([]gatewayv1.BackendObjectReference, 0)
	for _, svc := range services {
		port := minecraftPort(svc)
		if port == nil {
			log.Info("will not discover service due to no matching minecraft port", "service", fmt.Sprintf("%s/%s", svc.Namespace, svc.Name))
			continue
		}
		ref := gatewayv1.BackendObjectReference{
			Group:     (*gatewayv1.Group)(ptr.To(svc.GroupVersionKind().Group)),
			Kind:      (*gatewayv1.Kind)(ptr.To(svc.GroupVersionKind().Kind)),
			Name:      gatewayv1.ObjectName(svc.Name),
			Namespace: (*gatewayv1.Namespace)(ptr.To(svc.Namespace)),
			Port:      port,
		}
		if !includesBackendRef(discovery.Status.BackendRefs, ref) {
			log.Info("Discovered new Backend Reference", "backendRef", ref)
		}
		backends = append(backends, ref)
	}
	for _, missing := range missingBackendRefs(discovery.Status.BackendRefs, backends) {
		log.Error(nil, "previously discovered BackendReference unavailable", "backendRef", missing)
	}
	before := discovery.DeepCopy()
	if discovery.Status.Conditions == nil {
		discovery.Status.Conditions = make([]metav1.Condition, 0)
	}
	discovery.Status.BackendRefs = backends
	if err := r.Status().Patch(ctx, &discovery, client.MergeFrom(before)); err != nil {
		return ctrl.Result{}, err
	}
	return ctrl.Result{}, nil
}

func includesBackendRef(arr []gatewayv1.BackendObjectReference, obj gatewayv1.BackendObjectReference) bool {
	return util.ListIncludes(arr, obj, backendRefCompareFunc)
}

func missingBackendRefs(old, new []gatewayv1.BackendObjectReference) []gatewayv1.BackendObjectReference {
	return util.ListMissing(old, new, backendRefCompareFunc)
}

func backendRefCompareFunc(first gatewayv1.BackendObjectReference, second gatewayv1.BackendObjectReference) bool {
	return string(*first.Kind) == string(*second.Kind) && string(*first.Group) == string(*second.Group) && string(*first.Namespace) == string(*second.Namespace) && first.Name == second.Name
}

func (r *MinecraftServerDiscoveryReconciler) getServices(ctx context.Context, discovery mcgatewayv1.MinecraftServerDiscovery) ([]corev1.Service, error) {
	allNs, err := util.SelectNamespace(r.Client, ctx, discovery.Namespace, discovery.Spec.NamespaceSelector)
	if err != nil {
		return nil, err
	}

	selector, err := metav1.LabelSelectorAsSelector(&discovery.Spec.LabelSelector)
	if err != nil {
		return nil, err
	}
	result := make([]corev1.Service, 0)
	for _, ns := range allNs {
		var services corev1.ServiceList
		err = r.List(ctx, &services, client.InNamespace(ns), client.MatchingLabelsSelector{
			Selector: selector,
		})
		if err != nil {
			return nil, err
		}
		result = append(result, services.Items...)
	}
	return result, nil
}

func minecraftPort(svc corev1.Service) *gatewayv1.PortNumber {
	var fallback *gatewayv1.PortNumber

	for _, port := range svc.Spec.Ports {
		if port.Protocol != corev1.ProtocolTCP {
			continue
		}
		if port.Name == "minecraft" || port.Port == 25565 {
			return ptr.To(port.Port)
		}
		if fallback == nil {
			fallback = ptr.To(port.Port)
		}
	}

	return fallback
}

func (r *MinecraftServerDiscoveryReconciler) watchGateways(ctx context.Context, obj client.Object) []reconcile.Request {
	log := logf.FromContext(ctx)
	gw := obj.(*gatewayv1.Gateway)
	discovery, err := mfdiscovery.GetMinecraftServerDiscoveryByGateway(r.Client, ctx, *gw)
	if err != nil {
		log.Error(err, "can not get MinecraftServerDiscovery from gateway", "gateway", gw)
		return nil
	}
	return []reconcile.Request{
		{
			NamespacedName: types.NamespacedName{
				Namespace: discovery.Namespace,
				Name:      discovery.Name,
			},
		},
	}
}

func (r *MinecraftServerDiscoveryReconciler) watchEndpointsForDiscovery(ctx context.Context, obj client.Object) []reconcile.Request {
	log := logf.FromContext(ctx)
	slice := obj.(*discoveryv1.EndpointSlice)
	svc, err := endpoint.GetServiceByEndpointSlice(r.Client, ctx, *slice)
	if err != nil {
		log.Error(err, "failed to get services for endpoint slice", "EndpointSlice", slice)
		return nil
	}
	var discoveries mcgatewayv1.MinecraftServerDiscoveryList
	// TODO: discoveries by label index
	if err := r.List(ctx, &discoveries); err != nil {
		return nil
	}

	reqs := make([]reconcile.Request, 0)
	for _, disc := range discoveries.Items {
		namespaces, err := util.SelectNamespace(r.Client, ctx, disc.Namespace, disc.Spec.NamespaceSelector)
		if err != nil {
			continue
		}
		for _, ns := range namespaces {
			if svc.Namespace != ns {
				continue
			}
			selector, err := metav1.LabelSelectorAsSelector(&disc.Spec.LabelSelector)
			if err != nil {
				log.Error(err, "can not create label selector", "selector", selector)
				continue
			}
			if !selector.Matches(labels.Set(svc.Labels)) {
				continue
			}
			reqs = append(reqs, reconcile.Request{
				NamespacedName: types.NamespacedName{
					Namespace: disc.Namespace,
					Name:      disc.Name,
				},
			})
		}
	}
	return reqs
}

// SetupWithManager sets up the controller with the Manager.
func (r *MinecraftServerDiscoveryReconciler) SetupWithManager(mgr ctrl.Manager) error {
	return ctrl.NewControllerManagedBy(mgr).
		For(&mcgatewayv1.MinecraftServerDiscovery{}).
		Watches(&discoveryv1.EndpointSlice{}, handler.EnqueueRequestsFromMapFunc(r.watchEndpointsForDiscovery)).
		Watches(&gatewayv1.Gateway{}, handler.EnqueueRequestsFromMapFunc(r.watchGateways)).
		// TODO: add gateway class verification when this becomes standard channel
		Named("minecraftserverdiscovery").
		Complete(r)
}
