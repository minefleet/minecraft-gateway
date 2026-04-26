package topology

import (
	apimeta "k8s.io/apimachinery/pkg/api/meta"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	gatewayv1 "sigs.k8s.io/gateway-api/apis/v1"
)

// ControllerName is the controller name registered with GatewayClass objects.
const ControllerName = "minefleet.dev/gateway-controller"

// GatewayClass wraps gatewayv1.GatewayClass with status reader helpers.
type GatewayClass struct {
	gatewayv1.GatewayClass
}

// IsOurs reports whether this class is managed by this controller,
// regardless of whether it has been accepted yet.
func (gc GatewayClass) IsOurs() bool {
	return string(gc.Spec.ControllerName) == ControllerName
}

// IsAccepted reports whether this class has the Accepted condition set to True.
func (gc GatewayClass) IsAccepted() bool {
	c := apimeta.FindStatusCondition(gc.Status.Conditions, string(gatewayv1.GatewayClassConditionStatusAccepted))
	return c != nil && c.Status == metav1.ConditionTrue
}
