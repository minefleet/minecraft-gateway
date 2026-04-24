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
	"errors"
	"fmt"

	discoveryv1 "k8s.io/api/discovery/v1"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/utils/ptr"
	mcgatewayv1alpha1 "minefleet.dev/minecraft-gateway/api/controller/v1alpha1"
	"minefleet.dev/minecraft-gateway/internal/dataplane"
	mfdiscovery "minefleet.dev/minecraft-gateway/internal/discovery"
	"minefleet.dev/minecraft-gateway/internal/endpoint"
	"minefleet.dev/minecraft-gateway/internal/gateway"
	"minefleet.dev/minecraft-gateway/internal/route"
	mcstatus "minefleet.dev/minecraft-gateway/internal/status"
	"sigs.k8s.io/controller-runtime/pkg/handler"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"
	gatewayv1 "sigs.k8s.io/gateway-api/apis/v1"

	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/codes"
	"go.opentelemetry.io/otel/trace"

	"k8s.io/apimachinery/pkg/runtime"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	logf "sigs.k8s.io/controller-runtime/pkg/log"
)

// GatewayReconciler reconciles a Gateway object
type GatewayReconciler struct {
	client.Client
	Scheme    *runtime.Scheme
	Dataplane *dataplane.Dataplane
	tracer    trace.Tracer
}

// +kubebuilder:rbac:groups=gateway.networking.k8s.io,resources=gateways;gatewayclasses,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups=gateway.networking.k8s.io,resources=referencegrants,verbs=get;list;watch
// +kubebuilder:rbac:groups=gateway.networking.k8s.io,resources=gateways/status;gatewayclasses/status,verbs=get;update;patch
// +kubebuilder:rbac:groups=gateway.networking.k8s.io,resources=gateways/finalizers;gatewayclasses/finalizers,verbs=update
// +kubebuilder:rbac:groups=gateway.networking.minefleet.dev,resources=minecraftfallbackroutes;minecraftjoinroutes;networkinfrastructures,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups=gateway.networking.minefleet.dev,resources=minecraftfallbackroutes/status;minecraftjoinroutes/status;networkinfrastructures/status,verbs=get;update;patch
// +kubebuilder:rbac:groups=gateway.networking.minefleet.dev,resources=minecraftfallbackroutes/finalizers;minecraftjoinroutes/finalizers;networkinfrastructures/finalizers,verbs=update
// +kubebuilder:rbac:groups=discovery.k8s.io,resources=endpointslices,verbs=get;list;watch
// +kubebuilder:rbac:groups=core,resources=services,verbs=get;list;watch
// +kubebuilder:rbac:groups="",resources=configmaps,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups=apps,resources=daemonsets,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups="",resources=namespaces,verbs=get;list;watch

func (r *GatewayReconciler) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
	ctx, span := r.tracer.Start(ctx, "GatewayReconciler.Reconcile",
		trace.WithAttributes(
			attribute.String("gateway.namespace", req.Namespace),
			attribute.String("gateway.name", req.Name),
		),
	)
	defer span.End()

	log := logf.FromContext(ctx)

	var gw gatewayv1.Gateway
	if err := r.Get(ctx, req.NamespacedName, &gw); err != nil {
		if client.IgnoreNotFound(err) == nil {
			return ctrl.Result{}, (*r.Dataplane).DeleteGateway(req.NamespacedName)
		}
		return ctrl.Result{}, err
	}

	gwBase := gw.DeepCopy()

	verifier, err := gateway.NewClassVerifierByGateway(r.Client, ctx, gw)
	if err != nil {
		if client.IgnoreNotFound(err) == nil {
			return ctrl.Result{}, nil
		}
		return ctrl.Result{}, err
	}
	if !verifier.IsOurs() {
		return ctrl.Result{}, nil
	}
	if !verifier.IsVerified() {
		log.Info("GatewayClass not yet accepted, marking Gateway invalid",
			"gateway", fmt.Sprintf("%s/%s", gw.Namespace, gw.Name),
			"class", gw.Spec.GatewayClassName)
		mcstatus.SetGatewayNotProgrammed(&gw, gatewayv1.GatewayReasonPending,
			"GatewayClass is not yet accepted by the controller.")
		if err := r.Status().Patch(ctx, &gw, client.MergeFrom(gwBase)); err != nil {
			return ctrl.Result{}, client.IgnoreNotFound(err)
		}
		return ctrl.Result{}, nil
	}

	mcstatus.SetGatewayAccepted(&gw)

	infrastructure, err := gateway.GetInfrastructureForGateway(r.Client, ctx, gw)
	if err != nil {
		return ctrl.Result{}, nil
	}

	var bag route.Bag
	if err := route.ListAllRoutesByGateway(r.Client, ctx, gw, &bag); err != nil {
		return ctrl.Result{}, client.IgnoreNotFound(err)
	}

	backends := make([]discoveryv1.EndpointSlice, 0)
	for _, ref := range infrastructure.Status.BackendRefs {
		backend, err := endpoint.GetEndpointSlicesByServiceName(r.Client, ctx, string(ptr.Deref(ref.Namespace, "")), string(ref.Name))
		if err != nil {
			log.Error(err, "could not resolve endpoint slices for service")
			if client.IgnoreNotFound(err) != nil {
				return ctrl.Result{}, err
			}
			continue
		}
		backends = append(backends, backend...)
	}

	network := route.FilterAllowedRoutes(r.Client, ctx, gw, bag)

	var syncErr error
	var conflictError dataplane.RouteConflictError

	if r.Dataplane != nil {
		_, syncSpan := r.tracer.Start(ctx, "dataplane.SyncGateway",
			trace.WithAttributes(
				attribute.String("gateway.namespace", req.Namespace),
				attribute.String("gateway.name", req.Name),
			),
		)
		syncErr = (*r.Dataplane).SyncGateway(req.NamespacedName, infrastructure, network, backends)
		if syncErr != nil {
			syncSpan.RecordError(syncErr)
			syncSpan.SetStatus(codes.Error, syncErr.Error())
		}
		syncSpan.End()
	}

	isConflict := errors.As(syncErr, &conflictError)
	_, selfConflicting := conflictError.Conflicting[req.NamespacedName]

	programmed := syncErr == nil || (isConflict && !selfConflicting)

	if programmed {
		mcstatus.SetGatewayProgrammed(&gw)
	} else if isConflict && selfConflicting {
		mcstatus.SetGatewayNotProgrammed(&gw, "RouteConflict",
			"Gateway has routes that conflict with a higher-priority gateway.")
		span.SetStatus(codes.Error, conflictError.Error())
	} else {
		mcstatus.SetGatewayNotProgrammed(&gw, gatewayv1.GatewayReasonInvalid, syncErr.Error())
		span.RecordError(syncErr)
		span.SetStatus(codes.Error, syncErr.Error())
	}

	listenerBags := make(map[gatewayv1.SectionName]route.Bag, len(network))
	for listener, bag := range network {
		listenerBags[listener.Name] = bag
	}
	for _, listener := range gw.Spec.Listeners {
		bag := listenerBags[listener.Name]
		attached := int32(len(bag.Join) + len(bag.Fallback))
		supportedKinds, hasInvalidKinds := route.ListenerRouteKindStatus(listener)
		mcstatus.SetListenerStatus(&gw, listener.Name, attached, programmed, supportedKinds, hasInvalidKinds)
	}

	if err := r.Status().Patch(ctx, &gw, client.MergeFrom(gwBase)); err != nil {
		return ctrl.Result{}, client.IgnoreNotFound(err)
	}

	if err := r.updateRouteStatuses(ctx, req.NamespacedName, network, selfConflicting); err != nil {
		return ctrl.Result{}, err
	}

	if isConflict && !selfConflicting {
		// Re-queue the gateways that lost so they can recover their Programmed status.
		for conflicting := range conflictError.Conflicting {
			if conflicting != req.NamespacedName {
				log.Info("requeueing conflicting gateway", "gateway", conflicting)
			}
		}
	}

	if syncErr != nil && !isConflict {
		return ctrl.Result{}, syncErr
	}
	return ctrl.Result{}, nil
}

// updateRouteStatuses sets the Accepted condition on every route in the filtered
// network map. When selfConflicting is true all routes are marked with RouteConflict.
func (r *GatewayReconciler) updateRouteStatuses(
	ctx context.Context,
	gwNN types.NamespacedName,
	network map[gatewayv1.Listener]route.Bag,
	selfConflicting bool,
) error {
	seen := make(map[types.NamespacedName]struct{})

	for _, bag := range network {
		for i := range bag.Join {
			rt := &bag.Join[i]
			nn := types.NamespacedName{Namespace: rt.Namespace, Name: rt.Name}
			if _, ok := seen[nn]; ok {
				continue
			}
			seen[nn] = struct{}{}

			base := rt.DeepCopy()
			if selfConflicting {
				mcstatus.SetJoinRouteConflict(rt, gwNN)
			} else {
				mcstatus.SetJoinRouteAccepted(rt, gwNN)
			}
			if err := r.Status().Patch(ctx, rt, client.MergeFrom(base)); err != nil {
				if client.IgnoreNotFound(err) != nil {
					return err
				}
			}
		}

		for i := range bag.Fallback {
			rt := &bag.Fallback[i]
			nn := types.NamespacedName{Namespace: rt.Namespace, Name: rt.Name}
			if _, ok := seen[nn]; ok {
				continue
			}
			seen[nn] = struct{}{}

			base := rt.DeepCopy()
			if selfConflicting {
				mcstatus.SetFallbackRouteConflict(rt, gwNN)
			} else {
				mcstatus.SetFallbackRouteAccepted(rt, gwNN)
			}
			if err := r.Status().Patch(ctx, rt, client.MergeFrom(base)); err != nil {
				if client.IgnoreNotFound(err) != nil {
					return err
				}
			}
		}
	}
	return nil
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
	var used mcgatewayv1alpha1.MinecraftRoute
	var ns string

	if join, ok := obj.(*mcgatewayv1alpha1.MinecraftJoinRoute); ok {
		used = join.Spec.MinecraftRoute
		ns = join.Namespace
	} else if fallback, ok := obj.(*mcgatewayv1alpha1.MinecraftFallbackRoute); ok {
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

	seen := make(map[types.NamespacedName]struct{})
	result := make([]reconcile.Request, 0)

	enqueue := func(name types.NamespacedName) {
		if _, ok := seen[name]; !ok {
			seen[name] = struct{}{}
			result = append(result, reconcile.Request{NamespacedName: name})
		}
	}

	discoveries, err := mfdiscovery.GetNetworkInfrastructuresByService(r.Client, ctx, svc)
	if err == nil {
		for _, disc := range discoveries {
			var gws gatewayv1.GatewayList
			if err := gateway.ListGatewaysByInfrastructure(r.Client, ctx, r.Scheme, &gws, &disc); err != nil {
				continue
			}
			for _, gw := range gws.Items {
				enqueue(types.NamespacedName{Namespace: gw.Namespace, Name: gw.Name})
			}
		}
	}

	var bag route.Bag
	if err := route.ListAllRoutesByService(r.Client, ctx, svc, &bag); err != nil {
		return result
	}
	for _, r := range bag.Join {
		for _, nn := range route.NamespacedNamesByRefs(r.Namespace, r.Spec.ParentRefs) {
			if nn.Section == "" {
				enqueue(types.NamespacedName{Namespace: nn.Namespace, Name: nn.Name})
			}
		}
	}
	for _, r := range bag.Fallback {
		for _, nn := range route.NamespacedNamesByRefs(r.Namespace, r.Spec.ParentRefs) {
			if nn.Section == "" {
				enqueue(types.NamespacedName{Namespace: nn.Namespace, Name: nn.Name})
			}
		}
	}

	return result
}

func (r *GatewayReconciler) mapInfrastructure(ctx context.Context, obj client.Object) []reconcile.Request {
	log := logf.FromContext(ctx)
	infrastructure := obj.(*mcgatewayv1alpha1.NetworkInfrastructure)
	var gws gatewayv1.GatewayList
	if err := gateway.ListGatewaysByInfrastructure(r.Client, ctx, r.Scheme, &gws, infrastructure); err != nil {
		log.Error(err, "failed to list gateways by infrastructure")
		return nil
	}
	result := make([]reconcile.Request, 0, len(gws.Items))
	for _, item := range gws.Items {
		result = append(result, reconcile.Request{
			NamespacedName: types.NamespacedName{
				Namespace: item.Namespace,
				Name:      item.Name,
			},
		})
	}

	return result
}

// SetupWithManager sets up the controller with the Manager.
func (r *GatewayReconciler) SetupWithManager(mgr ctrl.Manager) error {
	r.tracer = otel.Tracer("minefleet.dev/minecraft-gateway")

	if err := gateway.IndexGatewayByClassName(mgr); err != nil {
		return err
	}
	if err := route.IndexRoutes(mgr); err != nil {
		return err
	}
	if err := gateway.IndexGatewayByInfrastructure(mgr); err != nil {
		return err
	}

	if r.Dataplane == nil {
		r.Dataplane = new(dataplane.Dataplane)
	}
	err := mgr.Add(dataplane.Executor{
		Client:    mgr.GetClient(),
		Dataplane: r.Dataplane,
	})
	if err != nil {
		return err
	}

	return ctrl.NewControllerManagedBy(mgr).
		For(&gatewayv1.Gateway{}).
		Named("gateway").
		Watches(&gatewayv1.GatewayClass{}, handler.EnqueueRequestsFromMapFunc(r.mapGatewayClass)).
		Watches(&discoveryv1.EndpointSlice{}, handler.EnqueueRequestsFromMapFunc(r.mapEndpoints)).
		Watches(&mcgatewayv1alpha1.NetworkInfrastructure{}, handler.EnqueueRequestsFromMapFunc(r.mapInfrastructure)).
		Watches(&mcgatewayv1alpha1.MinecraftJoinRoute{}, handler.EnqueueRequestsFromMapFunc(r.mapRoute)).
		Watches(&mcgatewayv1alpha1.MinecraftFallbackRoute{}, handler.EnqueueRequestsFromMapFunc(r.mapRoute)).
		Complete(r)
}
