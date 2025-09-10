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
	corev1 "k8s.io/api/core/v1"
	discoveryv1 "k8s.io/api/discovery/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	"minefleet.dev/minecraft-gateway/internal/util"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/handler"
	logf "sigs.k8s.io/controller-runtime/pkg/log"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"

	mcgatewayv1 "minefleet.dev/minecraft-gateway/api/v1"
)

const serviceNameLabel = "kubernetes.io/service-name"

// MinecraftServerDiscoveryReconciler reconciles a MinecraftServerDiscovery object
type MinecraftServerDiscoveryReconciler struct {
	client.Client
	Scheme *runtime.Scheme
}

// +kubebuilder:rbac:groups=gateway.networking.minefleet.dev,resources=minecraftserverdiscoveries,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups=gateway.networking.minefleet.dev,resources=minecraftserverdiscoveries/status,verbs=get;update;patch
// +kubebuilder:rbac:groups=gateway.networking.minefleet.dev,resources=minecraftserverdiscoveries/finalizers,verbs=update

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
	_ = logf.FromContext(ctx)

	var discovery mcgatewayv1.MinecraftServerDiscovery
	if err := r.Get(ctx, req.NamespacedName, &discovery); err != nil {
		return ctrl.Result{}, client.IgnoreNotFound(err)
	}

	services, err := r.getServices(ctx, discovery)
	if err != nil {
		return ctrl.Result{}, err
	}
	result := make([]mcgatewayv1.Minecraft, 0)
	for _, svc := range services {
		mcSvc := mcgatewayv1.MinecraftServ{
			controllerName: svc.Name,
			Pods:           make([]corev1.Pod, 0),
		}
		labelSelector := metav1.LabelSelector{
			MatchLabels: svc.Spec.Selector,
		}
		selector, err := metav1.LabelSelectorAsSelector(&labelSelector)
		if err != nil {
			//TODO: check what to do later
			continue
		}
		var pods corev1.PodList
		err = r.List(ctx, &pods, client.InNamespace(svc.Namespace), selector.(client.MatchingLabelsSelector))
		if err != nil {
			//TODO: check what to do later
			continue
		}
		for _, po := range pods.Items {
			if po.Status.Phase == corev1.PodRunning {
				mcSvc.Pods = append(mcSvc.Pods, po)
			}
		}
		result = append(result, mcSvc)
	}
	//TODO: save into status
	_ = result
	return ctrl.Result{}, nil
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
		err = r.List(ctx, &services, client.InNamespace(ns), selector.(client.MatchingLabelsSelector))
		if err != nil {
			return nil, err
		}
		for _, svc := range services.Items {
			result = append(result, svc)
		}
	}
	return result, nil
}

func (r *MinecraftServerDiscoveryReconciler) watchEndpointsForDiscovery(ctx context.Context, obj client.Object) []reconcile.Request {
	slice := obj.(*discoveryv1.EndpointSlice)

	var discoveries mcgatewayv1.MinecraftServerDiscoveryList
	if err := r.List(ctx, &discoveries); err != nil {
		return nil
	}
	svcName := slice.Labels[serviceNameLabel]
	if svcName == "" {
		return nil
	}
	var svc corev1.Service
	if err := r.Get(ctx, types.NamespacedName{
		Namespace: slice.Namespace,
		Name:      svc.Name,
	}, &svc); err != nil {
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
			break
		}
	}
	return reqs
}

// SetupWithManager sets up the controller with the Manager.
func (r *MinecraftServerDiscoveryReconciler) SetupWithManager(mgr ctrl.Manager) error {
	return ctrl.NewControllerManagedBy(mgr).
		For(&mcgatewayv1.MinecraftServerDiscovery{}).
		Watches(&discoveryv1.EndpointSlice{}, handler.EnqueueRequestsFromMapFunc(r.watchEndpointsForDiscovery)).
		Named("minecraftserverdiscovery").
		Complete(r)
}
