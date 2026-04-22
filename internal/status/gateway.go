package status

import (
	apimeta "k8s.io/apimachinery/pkg/api/meta"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	gatewayv1 "sigs.k8s.io/gateway-api/apis/v1"
)

func SetGatewayAccepted(gw *gatewayv1.Gateway) {
	apimeta.SetStatusCondition(&gw.Status.Conditions, metav1.Condition{
		Type:               string(gatewayv1.GatewayConditionAccepted),
		Status:             metav1.ConditionTrue,
		Reason:             string(gatewayv1.GatewayReasonAccepted),
		Message:            "Gateway accepted by the controller.",
		ObservedGeneration: gw.Generation,
	})
}

func SetGatewayProgrammed(gw *gatewayv1.Gateway) {
	apimeta.SetStatusCondition(&gw.Status.Conditions, metav1.Condition{
		Type:               string(gatewayv1.GatewayConditionProgrammed),
		Status:             metav1.ConditionTrue,
		Reason:             string(gatewayv1.GatewayReasonProgrammed),
		Message:            "Gateway successfully programmed.",
		ObservedGeneration: gw.Generation,
	})
}

func SetGatewayNotProgrammed(gw *gatewayv1.Gateway, reason gatewayv1.GatewayConditionReason, msg string) {
	apimeta.SetStatusCondition(&gw.Status.Conditions, metav1.Condition{
		Type:               string(gatewayv1.GatewayConditionProgrammed),
		Status:             metav1.ConditionFalse,
		Reason:             string(reason),
		Message:            msg,
		ObservedGeneration: gw.Generation,
	})
}

// SetListenerStatus upserts the status entry for a single listener by name.
func SetListenerStatus(gw *gatewayv1.Gateway, name gatewayv1.SectionName, attachedRoutes int32, programmed bool) {
	progStatus := metav1.ConditionTrue
	progReason := string(gatewayv1.ListenerReasonProgrammed)
	if !programmed {
		progStatus = metav1.ConditionFalse
		progReason = string(gatewayv1.ListenerReasonInvalid)
	}

	var entry *gatewayv1.ListenerStatus
	for i := range gw.Status.Listeners {
		if gw.Status.Listeners[i].Name == name {
			entry = &gw.Status.Listeners[i]
			break
		}
	}
	if entry == nil {
		gw.Status.Listeners = append(gw.Status.Listeners, gatewayv1.ListenerStatus{Name: name})
		entry = &gw.Status.Listeners[len(gw.Status.Listeners)-1]
	}

	entry.AttachedRoutes = attachedRoutes
	apimeta.SetStatusCondition(&entry.Conditions, metav1.Condition{
		Type:               string(gatewayv1.ListenerConditionAccepted),
		Status:             metav1.ConditionTrue,
		Reason:             string(gatewayv1.ListenerReasonAccepted),
		Message:            "Listener accepted.",
		ObservedGeneration: gw.Generation,
	})
	apimeta.SetStatusCondition(&entry.Conditions, metav1.Condition{
		Type:               string(gatewayv1.ListenerConditionProgrammed),
		Status:             metav1.ConditionStatus(progStatus),
		Reason:             progReason,
		ObservedGeneration: gw.Generation,
	})
}
