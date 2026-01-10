package gateway

import (
	"context"
	"fmt"

	"k8s.io/apimachinery/pkg/api/meta"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/client"
	gatewayv1 "sigs.k8s.io/gateway-api/apis/v1"
)

const ControllerName = "minefleet.dev/gateway-controller"

type ClassVerifier struct {
	gwClass gatewayv1.GatewayClass
}

func NewClassVerifier(gwClass gatewayv1.GatewayClass) ClassVerifier {
	return ClassVerifier{
		gwClass: gwClass,
	}
}

func NewClassVerifierByGateway(c client.Client, ctx context.Context, gw gatewayv1.Gateway) (ClassVerifier, error) {
	var gwClass gatewayv1.GatewayClass
	if err := c.Get(ctx, types.NamespacedName{
		Namespace: gw.Namespace,
		Name:      string(gw.Spec.GatewayClassName),
	}, &gwClass); err != nil {
		return ClassVerifier{}, err
	}
	return NewClassVerifier(gwClass), nil
}

func (v *ClassVerifier) IsVerified() bool {
	if v.gwClass.Spec.ControllerName != ControllerName {
		return false
	}
	return meta.IsStatusConditionTrue(v.gwClass.Status.Conditions, string(gatewayv1.GatewayClassConditionStatusAccepted))
}

func (v *ClassVerifier) Verify() *gatewayv1.GatewayClass {
	if v.gwClass.Spec.ControllerName != ControllerName {
		return nil
	}
	class := v.gwClass.DeepCopy()
	// TODO: filter supported stuff only and else dont accept this class
	// TODO: check valid parentRef (if parent ref is provided)
	if changed := meta.SetStatusCondition(&class.Status.Conditions, metav1.Condition{
		Type:    string(gatewayv1.GatewayClassConditionStatusAccepted),
		Status:  metav1.ConditionTrue,
		Reason:  string(gatewayv1.GatewayClassReasonAccepted),
		Message: fmt.Sprintf("Gateway Class accepted by %s.", ControllerName),
	}); !changed {
		return nil
	}
	return class
}
