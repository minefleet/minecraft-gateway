package gateway

import (
	"context"
	"fmt"

	"k8s.io/apimachinery/pkg/types"
	"minefleet.dev/minecraft-gateway/internal/index"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	gatewayv1 "sigs.k8s.io/gateway-api/apis/v1"
)

func IndexGatewayByClassName(mgr ctrl.Manager) error {
	if err := mgr.GetFieldIndexer().IndexField(context.Background(), &gatewayv1.Gateway{}, index.GatewayByClassName,
		func(o client.Object) []string {
			gw := o.(*gatewayv1.Gateway)
			if gw.Spec.GatewayClassName == "" {
				return nil
			}
			return []string{string(gw.Spec.GatewayClassName)}
		},
	); err != nil {
		return fmt.Errorf("create index %s: %w", index.GatewayByClassName, err)
	}
	return nil
}

func GetGatewayClassByGateway(c client.Client, ctx context.Context, gw gatewayv1.Gateway) (gatewayv1.GatewayClass, error) {
	var gwClass gatewayv1.GatewayClass
	if err := c.Get(ctx, types.NamespacedName{
		Name: string(gw.Spec.GatewayClassName),
	}, &gwClass); err != nil {
		return gatewayv1.GatewayClass{}, err
	}
	return gwClass, nil
}

func ListGatewaysByClass(c client.Client, ctx context.Context, gwClass gatewayv1.GatewayClass) (gatewayv1.GatewayList, error) {
	var list gatewayv1.GatewayList
	if err := c.List(ctx, &list, client.MatchingFields{index.GatewayByClassName: gwClass.Name}); err != nil {
		return gatewayv1.GatewayList{}, err
	}
	return list, nil
}
