package topology

import (
	"context"
	"errors"
	"fmt"

	"k8s.io/apimachinery/pkg/types"
	"minefleet.dev/minecraft-gateway/internal/util"
	"sigs.k8s.io/controller-runtime/pkg/client"
	gatewayv1 "sigs.k8s.io/gateway-api/apis/v1"
)

const (
	JavaProtocolType    gatewayv1.ProtocolType = "JAVA"
	BedrockProtocolType gatewayv1.ProtocolType = "BEDROCK"
)

// Listener abstracts over a gatewayv1.Listener regardless of whether it comes
// from a Gateway or a ListenerSet.
type Listener interface {
	GetName() gatewayv1.SectionName
	GetProtocol() (gatewayv1.ProtocolType, error)
	GetPort() uint32
	GetHostname() *gatewayv1.Hostname
	// SupportedKinds returns the route kinds this listener will admit and
	// whether any of the listener's requested kinds are unsupported by this
	// controller. When no kinds are configured, all controller-supported kinds
	// are returned.
	SupportedKinds() (supported []gatewayv1.RouteGroupKind, hasInvalid bool)
	// CheckRouteAllowed returns nil when route passes both the namespace and
	// kind filters configured on this listener, otherwise a descriptive error.
	CheckRouteAllowed(route Route, c client.Client, ctx context.Context) error
}

// ListenersSourced groups the resolved Listener values with the Kubernetes
// object they came from (a Gateway or a ListenerSet).
type ListenersSourced struct {
	Listeners []Listener
	Object    client.Object
}

type listenerImpl struct {
	types.NamespacedName // namespace/name of the parent Gateway or ListenerSet
	gatewayv1.Listener
}

func (l *listenerImpl) GetName() gatewayv1.SectionName {
	return l.Listener.Name
}

func (l *listenerImpl) GetProtocol() (gatewayv1.ProtocolType, error) {
	switch l.Protocol {
	case JavaProtocolType, BedrockProtocolType:
		return l.Protocol, nil
	default:
		return "", errors.New("unsupported protocol: " + string(l.Protocol))
	}
}

func (l *listenerImpl) GetPort() uint32 {
	return uint32(l.Port)
}

func (l *listenerImpl) GetHostname() *gatewayv1.Hostname {
	return l.Hostname
}

func (l *listenerImpl) SupportedKinds() (supported []gatewayv1.RouteGroupKind, hasInvalid bool) {
	if len(l.AllowedRoutes.Kinds) == 0 {
		return SupportedRouteKinds, false
	}
	for _, req := range l.AllowedRoutes.Kinds {
		rg := gatewayv1.Group(gatewayv1.GroupName)
		if req.Group != nil {
			rg = *req.Group
		}
		matched := false
		for _, sk := range SupportedRouteKinds {
			if rg == *sk.Group && req.Kind == sk.Kind {
				supported = append(supported, sk)
				matched = true
				break
			}
		}
		if !matched {
			hasInvalid = true
		}
	}
	return
}

func (l *listenerImpl) CheckRouteAllowed(r Route, c client.Client, ctx context.Context) error {
	namespaces, err := l.allowedNamespaces(c, ctx)
	if err != nil {
		return err
	}
	if !namespaceAllowed(r.GetNamespace(), namespaces) {
		return fmt.Errorf("namespace %q not allowed by listener %q", r.GetNamespace(), l.Listener.Name)
	}
	if len(l.AllowedRoutes.Kinds) > 0 && !kindAllowed(r, l.AllowedRoutes.Kinds) {
		return fmt.Errorf("route kind not allowed by listener %q", l.Listener.Name)
	}
	return nil
}

func (l *listenerImpl) allowedNamespaces(c client.Client, ctx context.Context) ([]string, error) {
	return util.SelectNamespace(c, ctx, l.Namespace, l.AllowedRoutes.Namespaces)
}

// namespaceAllowed checks whether ns is permitted by the resolved namespace
// list returned by util.SelectNamespace:
//   - nil  -> NamespacesFromNone: no namespace is allowed
//   - []{} -> NamespacesFromAll: every namespace is allowed
//   - [...] -> specific namespaces only
func namespaceAllowed(ns string, allowed []string) bool {
	if allowed == nil {
		return false
	}
	if len(allowed) == 0 {
		return true
	}
	for _, a := range allowed {
		if a == ns {
			return true
		}
	}
	return false
}

// kindAllowed returns true if route r matches any of the kind constraints
// declared on the listener. Only called when kinds is non-empty.
func kindAllowed(r Route, kinds []gatewayv1.RouteGroupKind) bool {
	rk := routeKindName(r)
	for _, k := range kinds {
		g := minefleetGroup
		if k.Group != nil {
			g = *k.Group
		}
		if g == minefleetGroup && k.Kind == rk {
			return true
		}
	}
	return false
}

// NewListener wraps a gatewayv1.Listener coming from source (the namespace/name
// of its parent Gateway or ListenerSet).
func NewListener(source types.NamespacedName, listener gatewayv1.Listener) Listener {
	return &listenerImpl{NamespacedName: source, Listener: listener}
}

// ListenersFromGateway extracts all listeners from a Gateway.
func ListenersFromGateway(gw *gatewayv1.Gateway) ListenersSourced {
	nn := types.NamespacedName{Namespace: gw.Namespace, Name: gw.Name}
	result := make([]Listener, 0, len(gw.Spec.Listeners))
	for _, l := range gw.Spec.Listeners {
		result = append(result, NewListener(nn, l))
	}
	return ListenersSourced{Listeners: result, Object: gw}
}

// ListenersFromListenerSet extracts all listeners from a ListenerSet.
func ListenersFromListenerSet(ls *gatewayv1.ListenerSet) ListenersSourced {
	nn := types.NamespacedName{Namespace: ls.Namespace, Name: ls.Name}
	result := make([]Listener, 0, len(ls.Spec.Listeners))
	for _, l := range ls.Spec.Listeners {
		result = append(result, NewListener(nn, gatewayv1.Listener(l)))
	}
	return ListenersSourced{Listeners: result, Object: ls}
}
