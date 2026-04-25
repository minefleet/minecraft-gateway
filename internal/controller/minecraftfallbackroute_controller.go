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

	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/codes"
	"go.opentelemetry.io/otel/trace"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	mcgatewayv1alpha1 "minefleet.dev/minecraft-gateway/api/controller/v1alpha1"
	"minefleet.dev/minecraft-gateway/internal/gateway"
	"minefleet.dev/minecraft-gateway/internal/route"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/handler"
	logf "sigs.k8s.io/controller-runtime/pkg/log"
	gatewayv1 "sigs.k8s.io/gateway-api/apis/v1"
)

// MinecraftFallbackRouteReconciler reconciles a MinecraftFallbackRoute object
type MinecraftFallbackRouteReconciler struct {
	client.Client
	Scheme *runtime.Scheme
	tracer trace.Tracer
}

// +kubebuilder:rbac:groups=gateway.networking.minefleet.dev,resources=minecraftfallbackroutes,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups=gateway.networking.minefleet.dev,resources=minecraftfallbackroutes/status,verbs=get;update;patch
// +kubebuilder:rbac:groups=gateway.networking.minefleet.dev,resources=minecraftfallbackroutes/finalizers,verbs=update

func (r *MinecraftFallbackRouteReconciler) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
	ctx, span := r.tracer.Start(ctx, "MinecraftFallbackRouteReconciler.Reconcile",
		trace.WithAttributes(
			attribute.String("route.namespace", req.Namespace),
			attribute.String("route.name", req.Name),
		),
	)
	defer span.End()

	log := logf.FromContext(ctx)

	var rt mcgatewayv1alpha1.MinecraftFallbackRoute
	if err := r.Get(ctx, req.NamespacedName, &rt); err != nil {
		return ctrl.Result{}, client.IgnoreNotFound(err)
	}

	rt.TypeMeta = metav1.TypeMeta{
		APIVersion: mcgatewayv1alpha1.GroupVersion.String(),
		Kind:       "MinecraftFallbackRoute",
	}
	base := rt.DeepCopy()

	for _, ref := range rt.Spec.ParentRefs {
		gwNN, parent, err := route.ParentFromRef(ctx, r.Client, ref, rt.Namespace, "MinecraftFallbackRoute")
		if err != nil {
			span.RecordError(err)
			span.SetStatus(codes.Error, err.Error())
			return ctrl.Result{}, err
		}
		if gwNN.Name == "" {
			continue
		}
		if parent == nil {
			log.Info("parent gateway not found", "gateway", gwNN)
			route.StatusFor(route.ForFallback(&rt), route.ParentNotFound()).Apply(gwNN)
			continue
		}

		verifier, err := gateway.NewClassVerifierByGateway(r.Client, ctx, parent.Gateway)
		if err != nil || !verifier.IsVerified() {
			continue
		}

		ok, reason, msg := route.CheckBackendRefs(ctx, r.Client, rt.Namespace, &rt, rt.Spec.BackendRefs)
		route.StatusFor(route.ForFallback(&rt), route.ParentResolved(parent.Listeners, ok, reason, msg)).Apply(gwNN)
	}

	if err := r.Status().Patch(ctx, &rt, client.MergeFrom(base)); err != nil {
		return ctrl.Result{}, client.IgnoreNotFound(err)
	}
	return ctrl.Result{}, nil
}

// mapGatewayToFallbackRoutes re-queues all MinecraftFallbackRoutes that
// reference a changed Gateway.
func (r *MinecraftFallbackRouteReconciler) mapGatewayToFallbackRoutes(ctx context.Context, obj client.Object) []ctrl.Request {
	gw := obj.(*gatewayv1.Gateway)
	var bag route.Bag
	if err := route.ListAllRoutesByGateway(r.Client, ctx, *gw, &bag); err != nil {
		return nil
	}
	reqs := make([]ctrl.Request, 0, len(bag.Fallback))
	for _, rt := range bag.Fallback {
		reqs = append(reqs, ctrl.Request{
			NamespacedName: types.NamespacedName{Namespace: rt.GetNamespace(), Name: rt.GetName()},
		})
	}
	return reqs
}

// SetupWithManager sets up the controller with the Manager.
func (r *MinecraftFallbackRouteReconciler) SetupWithManager(mgr ctrl.Manager) error {
	r.tracer = otel.Tracer("minefleet.dev/minecraft-gateway")

	return ctrl.NewControllerManagedBy(mgr).
		For(&mcgatewayv1alpha1.MinecraftFallbackRoute{}).
		Named("minecraftfallbackroute").
		Watches(&gatewayv1.Gateway{}, handler.EnqueueRequestsFromMapFunc(r.mapGatewayToFallbackRoutes)).
		Complete(r)
}
