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

	"minefleet.dev/minecraft-gateway/internal/endpoint"

	discoveryv1 "k8s.io/api/discovery/v1"
	"k8s.io/apimachinery/pkg/types"
	mcgatewayv1 "minefleet.dev/minecraft-gateway/api/v1"
	"minefleet.dev/minecraft-gateway/internal/gateway"
	"minefleet.dev/minecraft-gateway/internal/route"
	"sigs.k8s.io/controller-runtime/pkg/handler"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"
	gatewayv1 "sigs.k8s.io/gateway-api/apis/v1"

	"k8s.io/apimachinery/pkg/runtime"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	logf "sigs.k8s.io/controller-runtime/pkg/log"
)

// GatewayReconciler reconciles a Gateway object
type GatewayReconciler struct {
	client.Client
	Scheme *runtime.Scheme
}

// +kubebuilder:rbac:groups=gateway.networking.sigs.k8s.io,resources=gateways,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups=gateway.networking.sigs.k8s.io,resources=gateways/status,verbs=get;update;patch
// +kubebuilder:rbac:groups=gateway.networking.sigs.k8s.io,resources=gateways/finalizers,verbs=update

// Reconcile is part of the main kubernetes reconciliation loop which aims to
// move the current state of the cluster closer to the desired state.
// TODO(user): Modify the Reconcile function to compare the state specified by
// the Gateway object against the actual cluster state, and then
// perform operations to make the cluster state reflect the state specified by
// the user.
//
// For more details, check Reconcile and its Result here:
// - https://pkg.go.dev/sigs.k8s.io/controller-runtime@v0.21.0/pkg/reconcile
func (r *GatewayReconciler) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
	log := logf.FromContext(ctx)

	var gw gatewayv1.Gateway
	if err := r.Get(ctx, req.NamespacedName, &gw); err != nil {
		return ctrl.Result{}, client.IgnoreNotFound(err)
	}
	verifier, err := gateway.NewClassVerifierByGateway(r.Client, ctx, gw)
	if err != nil {
		return ctrl.Result{}, client.IgnoreNotFound(err)
	}
	if !verifier.IsVerified() {
		log.Info("Gateway Class was not verified or rejected", "gateway", fmt.Sprintf("%s/%s", gw.Namespace, gw.Name), "class", gw.Spec.GatewayClassName)
		return ctrl.Result{}, nil
	}

	infrastructure, err := gateway.GetInfrastructureByGateway(r.Client, ctx, gw)
	if err != nil {
		return ctrl.Result{}, nil
	}
	log.Info("infrastructure test log", "infrastructure", infrastructure)

	// TODO: actually reconcile the gateway as it is "locked and loaded"
	// [] Make sure there are daemon sets, proxy services for each listener (and gate lite instances)
	//    Generally speaking: One daemon set and gate lite service for each port generally common across all gateways,
	//    Then proxy services for each listener
	// [X] List routes for each consecutive listener
	// [] Regenerate config map per listener (proxy level)
	// [] Regenerate config map for listeners (gate lite level)
	// TODO: copy infrastructure stuff from the gwclass via mutating admission webhook (maybe)

	var bag route.Bag
	if err := route.ListAllRoutesByGateway(r.Client, ctx, gw, &bag); err != nil {
		return ctrl.Result{}, client.IgnoreNotFound(err)
	}
	network := route.FilterAllowedRoutes(r.Client, ctx, gw, bag)
	for listener, routes := range network {
		// TODO: create proxy
		// TODO: edit the gate daemonset to provide listener
		for i, joinRoute := range routes.Join {
			_ = i
			_ = listener
			_ = joinRoute
		}
	}

	// TODO: regenerate config map

	return ctrl.Result{}, nil
}

func (r *GatewayReconciler) mapGatewayClass(ctx context.Context, obj client.Object) []reconcile.Request {
	gc := obj.(*gatewayv1.GatewayClass)

	gws, err := gateway.ListGatewaysByClass(r.Client, ctx, *gc)
	if err != nil {
		return nil
	}
	reqs := make([]reconcile.Request, 0, len(gws.Items))
	for i := range gws.Items {
		reqs = append(reqs, reconcile.Request{
			NamespacedName: types.NamespacedName{
				Namespace: gws.Items[i].Namespace, Name: gws.Items[i].Name,
			},
		})
	}
	return reqs
}

func (r *GatewayReconciler) mapRoute(_ context.Context, obj client.Object) []reconcile.Request {
	var used mcgatewayv1.MinecraftRoute
	var ns string

	if join, ok := obj.(*mcgatewayv1.MinecraftJoinRoute); ok {
		used = join.Spec.MinecraftRoute
		ns = join.Namespace
	} else if fallback, ok := obj.(*mcgatewayv1.MinecraftFallbackRoute); ok {
		used = fallback.Spec.MinecraftRoute
		ns = fallback.Namespace
	} else {
		return nil
	}
	result := make([]reconcile.Request, 0)
	targetsNamespaced := route.NamespacedNamesByRefs(ns, used.ParentRefs)
	for _, gw := range targetsNamespaced {
		// Do not reconcile if listeners are targeted
		// TODO: maybe handle listeners later, for now they are unsupported
		if gw.Section != "" {
			continue
		}
		result = append(result, reconcile.Request{
			NamespacedName: types.NamespacedName{
				Namespace: gw.Namespace,
				Name:      gw.Name,
			},
		})
	}
	return result
}

func (r *GatewayReconciler) mapEndpoints(ctx context.Context, obj client.Object) []reconcile.Request {
	slice := obj.(*discoveryv1.EndpointSlice)
	svc, err := endpoint.GetServiceByEndpointSlice(r.Client, ctx, *slice)
	if err != nil {
		return nil
	}
	// TODO: requeue if the slices service is being targeted by infrastructure config

	// requeue if the slices service is being targeted by either Join or Fallback routes
	var bag route.Bag
	if err := route.ListAllRoutesByService(r.Client, ctx, svc, &bag); err != nil {
		return nil
	}

	// TODO: actually requeue

	return nil
}

// SetupWithManager sets up the controller with the Manager.
func (r *GatewayReconciler) SetupWithManager(mgr ctrl.Manager) error {
	if err := gateway.IndexGatewayByClassName(mgr); err != nil {
		return err
	}
	if err := route.IndexRoutes(mgr); err != nil {
		return err
	}
	if err := gateway.IndexGatewayByInfrastructure(mgr); err != nil {
		return err
	}

	return ctrl.NewControllerManagedBy(mgr).
		For(&gatewayv1.Gateway{}).
		Named("gateway").
		Watches(&gatewayv1.GatewayClass{}, handler.EnqueueRequestsFromMapFunc(r.mapGatewayClass)).
		Watches(&discoveryv1.EndpointSlice{}, handler.EnqueueRequestsFromMapFunc(r.mapEndpoints)).
		Watches(&mcgatewayv1.MinecraftJoinRoute{}, handler.EnqueueRequestsFromMapFunc(r.mapRoute)).
		Watches(&mcgatewayv1.MinecraftFallbackRoute{}, handler.EnqueueRequestsFromMapFunc(r.mapRoute)).
		Complete(r)
}
