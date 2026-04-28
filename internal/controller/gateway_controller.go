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

	"github.com/go-logr/logr"
	corev1 "k8s.io/api/core/v1"
	discoveryv1 "k8s.io/api/discovery/v1"
	"k8s.io/apimachinery/pkg/types"
	mcgatewayv1alpha1 "minefleet.dev/minecraft-gateway/api/controller/v1alpha1"
	"minefleet.dev/minecraft-gateway/internal/dataplane"
	"minefleet.dev/minecraft-gateway/internal/dataplane/edge"
	networkdp "minefleet.dev/minecraft-gateway/internal/dataplane/network"
	"minefleet.dev/minecraft-gateway/internal/gateway"
	mfdiscovery "minefleet.dev/minecraft-gateway/internal/infrastructure"
	"minefleet.dev/minecraft-gateway/internal/route"
	"minefleet.dev/minecraft-gateway/internal/topology"
	"sigs.k8s.io/controller-runtime/pkg/builder"
	"sigs.k8s.io/controller-runtime/pkg/event"
	"sigs.k8s.io/controller-runtime/pkg/handler"
	"sigs.k8s.io/controller-runtime/pkg/predicate"
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
// +kubebuilder:rbac:groups="",resources=pods,verbs=get;list;watch

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

	tree, err := topology.Build(ctx, r.Client, &gw)
	if err != nil {
		if client.IgnoreNotFound(err) == nil {
			return ctrl.Result{}, nil // GatewayClass not found
		}
		return ctrl.Result{}, err
	}

	if !tree.Class.IsOurs() {
		return ctrl.Result{}, nil
	}

	tree.StatusWriter().SetAccepted()

	if !tree.Class.IsAccepted() {
		log.Info("GatewayClass not yet accepted, marking Gateway not programmed",
			"gateway", req.NamespacedName,
			"class", gw.Spec.GatewayClassName)
		tree.StatusWriter().SetNotProgrammed(gatewayv1.GatewayReasonPending,
			"GatewayClass is not yet accepted by the controller.")
		return ctrl.Result{}, tree.PatchGatewayStatus(ctx, r.Client)
	}

	if invalid := r.hasInvalidProtocol(tree); invalid {
		return ctrl.Result{}, r.reconcileInvalidProtocols(ctx, log, tree, req.NamespacedName)
	}

	isConflict, selfConflicting, syncErr := r.dataplaneSync(ctx, tree, req.NamespacedName, log)
	programmed := syncErr == nil || (isConflict && !selfConflicting)

	r.writeGatewayAndListenerStatus(ctx, span, tree, isConflict, selfConflicting, programmed, syncErr)

	tree.StatusWriter().SetAddresses(r.listEdgeAddresses(ctx))

	if err := tree.PatchGatewayStatus(ctx, r.Client); err != nil {
		return ctrl.Result{}, err
	}
	if err := tree.WriteRouteStatuses(ctx, r.Client, selfConflicting); err != nil {
		return ctrl.Result{}, err
	}

	if isConflict && !selfConflicting {
		for conflicting := range conflictError(syncErr).Conflicting {
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

func (r *GatewayReconciler) hasInvalidProtocol(tree *topology.GatewayTree) bool {
	for _, lt := range tree.Listeners() {
		if _, err := lt.Listener.GetProtocol(); err != nil {
			return true
		}
	}
	return false
}

func (r *GatewayReconciler) reconcileInvalidProtocols(ctx context.Context, log logr.Logger, tree *topology.GatewayTree, nn types.NamespacedName) error {
	tree.StatusWriter().
		SetAcceptedListenersNotValid().
		SetNotProgrammed(gatewayv1.GatewayReasonInvalid, "One or more listeners have an unsupported protocol.")
	for _, lt := range tree.Listeners() {
		kinds, _ := lt.Listener.SupportedKinds()
		lsw := lt.StatusWriter()
		if _, protoErr := lt.Listener.GetProtocol(); protoErr != nil {
			lsw.SetNotAccepted(gatewayv1.ListenerReasonUnsupportedProtocol, protoErr.Error()).
				SetNotProgrammed(gatewayv1.ListenerReasonInvalid, "Listener has an unsupported protocol.").
				SetAttachedRoutes(0).SetSupportedKinds(kinds).SetNoConflicts()
		} else {
			lsw.SetAccepted().SetNoConflicts().SetAttachedRoutes(lt.AttachedRoutes()).SetSupportedKinds(kinds).
				SetNotProgrammed(gatewayv1.ListenerReasonInvalid, "Gateway not programmed.")
		}
	}
	if r.Dataplane != nil {
		if err := (*r.Dataplane).DeleteGateway(nn); err != nil {
			log.Error(err, "failed to delete gateway from dataplane after protocol validation failure")
		}
	}
	return tree.PatchGatewayStatus(ctx, r.Client)
}

func (r *GatewayReconciler) dataplaneSync(ctx context.Context, tree *topology.GatewayTree, nn types.NamespacedName, log logr.Logger) (isConflict, selfConflicting bool, syncErr error) {
	if r.Dataplane == nil {
		return false, false, nil
	}
	_, syncSpan := r.tracer.Start(ctx, "dataplane.SyncGateway",
		trace.WithAttributes(
			attribute.String("gateway.namespace", nn.Namespace),
			attribute.String("gateway.name", nn.Name),
		),
	)
	syncErr = (*r.Dataplane).SyncGateway(tree)
	if syncErr != nil {
		syncSpan.RecordError(syncErr)
		syncSpan.SetStatus(codes.Error, syncErr.Error())
	}
	syncSpan.End()

	var ce dataplane.RouteConflictError
	isConflict = errors.As(syncErr, &ce)
	_, selfConflicting = ce.Conflicting[nn]
	if selfConflicting {
		if err := (*r.Dataplane).DeleteGateway(nn); err != nil {
			log.Error(err, "failed to delete conflicting gateway from dataplane")
		}
	}
	return isConflict, selfConflicting, syncErr
}

func conflictError(syncErr error) dataplane.RouteConflictError {
	var ce dataplane.RouteConflictError
	errors.As(syncErr, &ce)
	return ce
}

func (r *GatewayReconciler) writeGatewayAndListenerStatus(ctx context.Context, span trace.Span, tree *topology.GatewayTree, isConflict, selfConflicting, programmed bool, syncErr error) {
	_ = ctx
	sw := tree.StatusWriter()
	if programmed {
		sw.SetProgrammed()
	} else if isConflict && selfConflicting {
		sw.SetNotProgrammed("RouteConflict", "Gateway has routes that conflict with a higher-priority gateway.")
		span.SetStatus(codes.Error, syncErr.Error())
	} else {
		sw.SetNotProgrammed(gatewayv1.GatewayReasonInvalid, syncErr.Error())
		span.RecordError(syncErr)
		span.SetStatus(codes.Error, syncErr.Error())
	}
	for _, lt := range tree.Listeners() {
		r.writeListenerStatus(lt, selfConflicting, programmed)
	}
}

func (r *GatewayReconciler) writeListenerStatus(lt topology.ListenerTree, selfConflicting, programmed bool) {
	kinds, hasInvalid := lt.Listener.SupportedKinds()
	lsw := lt.StatusWriter()
	if selfConflicting {
		lsw.SetConflicted(gatewayv1.ListenerReasonHostnameConflict,
			"Listener hostname conflicts with a listener on a higher-priority gateway.")
	} else {
		lsw.SetNoConflicts()
	}
	lsw.SetAccepted().SetAttachedRoutes(lt.AttachedRoutes()).SetSupportedKinds(kinds)
	if programmed {
		lsw.SetProgrammed()
	} else {
		lsw.SetNotProgrammed(gatewayv1.ListenerReasonInvalid, "Gateway not programmed.")
	}
	if hasInvalid {
		lsw.SetResolvedRefsInvalid(gatewayv1.ListenerReasonInvalidRouteKinds,
			"One or more requested route kinds are not supported by this controller.")
	} else {
		lsw.SetResolvedRefs()
	}
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
			NamespacedName: types.NamespacedName{Namespace: gws.Items[i].Namespace, Name: gws.Items[i].Name},
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
	for _, gw := range route.NamespacedNamesByRefs(ns, used.ParentRefs) {
		if gw.Section != "" {
			continue
		}
		result = append(result, reconcile.Request{
			NamespacedName: types.NamespacedName{Namespace: gw.Namespace, Name: gw.Name},
		})
	}
	return result
}

func (r *GatewayReconciler) mapEndpoints(ctx context.Context, obj client.Object) []reconcile.Request {
	slice := obj.(*discoveryv1.EndpointSlice)
	svc, err := r.getServiceByEndpointSlice(ctx, *slice)
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

	var bag topology.RouteBag
	if err := route.ListAllRoutesByService(r.Client, ctx, svc, &bag); err != nil {
		return result
	}
	for _, r := range bag.Join {
		for _, nn := range route.NamespacedNamesByRefs(r.GetNamespace(), r.ParentRefs()) {
			if nn.Section == "" {
				enqueue(types.NamespacedName{Namespace: nn.Namespace, Name: nn.Name})
			}
		}
	}
	for _, r := range bag.Fallback {
		for _, nn := range route.NamespacedNamesByRefs(r.GetNamespace(), r.ParentRefs()) {
			if nn.Section == "" {
				enqueue(types.NamespacedName{Namespace: nn.Namespace, Name: nn.Name})
			}
		}
	}
	return result
}

func (r *GatewayReconciler) getServiceByEndpointSlice(ctx context.Context, slice discoveryv1.EndpointSlice) (corev1.Service, error) {
	svcName, ok := slice.Labels[discoveryv1.LabelServiceName]
	if !ok {
		return corev1.Service{}, errors.New("no service name label on EndpointSlice")
	}
	var svc corev1.Service
	err := r.Get(ctx, types.NamespacedName{Namespace: slice.Namespace, Name: svcName}, &svc)
	return svc, err
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
			NamespacedName: types.NamespacedName{Namespace: item.Namespace, Name: item.Name},
		})
	}
	return result
}

// listEdgeAddresses returns one IPAddress entry per running edge pod.
func (r *GatewayReconciler) listEdgeAddresses(ctx context.Context) []gatewayv1.GatewayStatusAddress {
	var pods corev1.PodList
	if err := r.List(ctx, &pods, client.MatchingLabels(edge.SelectorLabels())); err != nil {
		return nil
	}
	seen := make(map[string]struct{})
	addrs := make([]gatewayv1.GatewayStatusAddress, 0, len(pods.Items))
	t := gatewayv1.IPAddressType
	for _, pod := range pods.Items {
		if pod.Status.Phase != corev1.PodRunning || pod.Status.HostIP == "" {
			continue
		}
		if _, ok := seen[pod.Status.HostIP]; ok {
			continue
		}
		seen[pod.Status.HostIP] = struct{}{}
		addrs = append(addrs, gatewayv1.GatewayStatusAddress{Type: &t, Value: pod.Status.HostIP})
	}
	return addrs
}

// mapEdgePod requeues all gateways whenever an edge pod changes.
func (r *GatewayReconciler) mapEdgePod(ctx context.Context, _ client.Object) []reconcile.Request {
	var gws gatewayv1.GatewayList
	if err := r.List(ctx, &gws); err != nil {
		return nil
	}
	reqs := make([]reconcile.Request, 0, len(gws.Items))
	for _, gw := range gws.Items {
		reqs = append(reqs, reconcile.Request{
			NamespacedName: types.NamespacedName{Namespace: gw.Namespace, Name: gw.Name},
		})
	}
	return reqs
}

// mapBackendPod maps a backend pod to the gateways that route traffic to it,
// by finding EndpointSlices in the pod's namespace that reference it and
// delegating to mapEndpoints for each.
func (r *GatewayReconciler) mapBackendPod(ctx context.Context, obj client.Object) []reconcile.Request {
	pod := obj.(*corev1.Pod)

	var slices discoveryv1.EndpointSliceList
	if err := r.List(ctx, &slices, client.InNamespace(pod.Namespace)); err != nil {
		return nil
	}

	seen := make(map[types.NamespacedName]struct{})
	var result []reconcile.Request

	for i := range slices.Items {
		slice := &slices.Items[i]
		for _, ep := range slice.Endpoints {
			if ep.TargetRef == nil || ep.TargetRef.Kind != "Pod" {
				continue
			}
			if ep.TargetRef.Name != pod.Name || ep.TargetRef.Namespace != pod.Namespace {
				continue
			}
			for _, req := range r.mapEndpoints(ctx, slice) {
				if _, ok := seen[req.NamespacedName]; !ok {
					seen[req.NamespacedName] = struct{}{}
					result = append(result, req)
				}
			}
			break
		}
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

	edgePodPredicate := predicate.NewPredicateFuncs(func(obj client.Object) bool {
		for k, v := range edge.SelectorLabels() {
			if obj.GetLabels()[k] != v {
				return false
			}
		}
		return true
	})

	backendPodPredicate := predicate.Funcs{
		UpdateFunc: func(e event.UpdateEvent) bool {
			oldAnn := e.ObjectOld.GetAnnotations()
			newAnn := e.ObjectNew.GetAnnotations()
			return oldAnn[networkdp.AnnotationCurrentPlayers] != newAnn[networkdp.AnnotationCurrentPlayers] ||
				oldAnn[networkdp.AnnotationMaxPlayers] != newAnn[networkdp.AnnotationMaxPlayers]
		},
		CreateFunc:  func(event.CreateEvent) bool { return false },
		DeleteFunc:  func(event.DeleteEvent) bool { return false },
		GenericFunc: func(event.GenericEvent) bool { return false },
	}

	return ctrl.NewControllerManagedBy(mgr).
		For(&gatewayv1.Gateway{}).
		Named("gateway").
		Watches(&gatewayv1.GatewayClass{}, handler.EnqueueRequestsFromMapFunc(r.mapGatewayClass)).
		Watches(&discoveryv1.EndpointSlice{}, handler.EnqueueRequestsFromMapFunc(r.mapEndpoints)).
		Watches(&mcgatewayv1alpha1.NetworkInfrastructure{}, handler.EnqueueRequestsFromMapFunc(r.mapInfrastructure)).
		Watches(&mcgatewayv1alpha1.MinecraftJoinRoute{}, handler.EnqueueRequestsFromMapFunc(r.mapRoute)).
		Watches(&mcgatewayv1alpha1.MinecraftFallbackRoute{}, handler.EnqueueRequestsFromMapFunc(r.mapRoute)).
		Watches(&corev1.Pod{}, handler.EnqueueRequestsFromMapFunc(r.mapEdgePod), builder.WithPredicates(edgePodPredicate)).
		Watches(&corev1.Pod{}, handler.EnqueueRequestsFromMapFunc(r.mapBackendPod), builder.WithPredicates(backendPodPredicate)).
		Complete(r)
}
