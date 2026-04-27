package gateway

import (
	"context"
	"fmt"

	"k8s.io/apimachinery/pkg/runtime"
	mcgatewayv1alpha1 "minefleet.dev/minecraft-gateway/api/controller/v1alpha1"
	"minefleet.dev/minecraft-gateway/internal/index"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/manager"
	gatewayv1 "sigs.k8s.io/gateway-api/apis/v1"
)

func keyGWByInfrastructure(group, kind, name string) string {
	return fmt.Sprintf("%s/%s/%s", group, kind, name)
}

func IndexGatewayByInfrastructure(mgr manager.Manager) error {

	return mgr.GetFieldIndexer().IndexField(context.Background(), &gatewayv1.Gateway{}, index.GatewayByInfrastructure, func(object client.Object) []string {
		gw := object.(*gatewayv1.Gateway)
		if gw.Spec.Infrastructure == nil || gw.Spec.Infrastructure.ParametersRef == nil {
			return nil
		}
		ref := *gw.Spec.Infrastructure.ParametersRef
		key := keyGWByInfrastructure(string(ref.Group), string(ref.Kind), ref.Name)
		return []string{key}
	})
}

func ListGatewaysByInfrastructure(c client.Client, ctx context.Context, scheme *runtime.Scheme, list *gatewayv1.GatewayList, infra *mcgatewayv1alpha1.NetworkInfrastructure) error {
	gvks, _, err := scheme.ObjectKinds(infra)
	if err != nil {
		return err
	}
	key := keyGWByInfrastructure(gvks[0].Group, gvks[0].Kind, infra.GetName())
	return c.List(ctx, list, client.MatchingFields{
		index.GatewayByInfrastructure: key,
	})
}
