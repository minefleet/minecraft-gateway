package gateway

import (
	"context"
	"fmt"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	gatewayv1 "sigs.k8s.io/gateway-api/apis/v1"
)

const gatewayByClassNameKey = "gateway.byClassName"

func IndexGatewayByClassName(mgr ctrl.Manager) error {
	if err := mgr.GetFieldIndexer().IndexField(context.Background(), &gatewayv1.Gateway{}, gatewayByClassNameKey,
		func(o client.Object) []string {
			gw := o.(*gatewayv1.Gateway)
			if gw.Spec.GatewayClassName == "" {
				return nil
			}
			return []string{string(gw.Spec.GatewayClassName)}
		},
	); err != nil {
		return fmt.Errorf("create index %s: %w", gatewayByClassNameKey, err)
	}
	return nil
}

func ListGatewaysByClass(c client.Client, ctx context.Context, gwClass gatewayv1.GatewayClass) (gatewayv1.GatewayList, error) {
	var list gatewayv1.GatewayList
	if err := c.List(ctx, &list, client.MatchingFields{gatewayByClassNameKey: gwClass.Name}); err != nil {
		return gatewayv1.GatewayList{}, err
	}
	return list, nil
}
