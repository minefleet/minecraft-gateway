package network

import (
	"fmt"
	"strconv"
	"sync/atomic"

	discoveryv1 "k8s.io/api/discovery/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/apimachinery/pkg/types"
	mcgatewayv1alpha1 "minefleet.dev/minecraft-gateway/api/controller/v1alpha1"
	apiv1alpha1 "minefleet.dev/minecraft-gateway/api/network/v1alpha1"
	"minefleet.dev/minecraft-gateway/internal/topology"
)

const (
	AnnotationCurrentPlayers = "gateway.networking.minefleet.dev/current-players"
	AnnotationMaxPlayers     = "gateway.networking.minefleet.dev/max-players"
)

const labelServiceName = "kubernetes.io/service-name"

var generation atomic.Int64

// Config holds the configuration for the network dataplane.
type Config struct {
	// XDSPort is the local port the xDS gRPC server listens on (default: 18001).
	XDSPort int
	// Namespace is the namespace where the controller runs.
	Namespace string
}

// ListenerSnapshot is the routing snapshot for one gateway listener.
type ListenerSnapshot struct {
	GatewayNamespace string
	GatewayName      string
	ListenerName     string
	Services         []*apiv1alpha1.ManagedService
}

// Snapshot is the aggregate of all listener snapshots.
// Keyed for O(1) lookup: gateway "namespace/name" → listener name → ListenerSnapshot.
type Snapshot struct {
	byGateway  map[string]map[string]*ListenerSnapshot
	Generation string
}

// Get returns the ListenerSnapshot for the given gateway and listener, or nil if not found.
func (s *Snapshot) Get(namespace, name, listener string) *ListenerSnapshot {
	if s.byGateway == nil {
		return nil
	}
	byListener := s.byGateway[namespace+"/"+name]
	if byListener == nil {
		return nil
	}
	return byListener[listener]
}

// GatewaySnapshotCache holds per-listener snapshots indexed by gateway then listener name.
type GatewaySnapshotCache = map[types.NamespacedName]map[string]ListenerSnapshot

// BuildListenerSnapshot constructs a ListenerSnapshot for one gateway listener.
func BuildListenerSnapshot(gateway types.NamespacedName, lt topology.ListenerTree, backends []discoveryv1.EndpointSlice, podAnnotations map[string]map[string]string) ListenerSnapshot {
	listener := lt.Listener
	routes := lt.Routes()
	// Index EndpointSlices by service key (namespace/name).
	slicesByService := make(map[string][]discoveryv1.EndpointSlice)
	for _, slice := range backends {
		svcName, ok := slice.Labels[labelServiceName]
		if !ok {
			continue
		}
		key := slice.Namespace + "/" + svcName
		slicesByService[key] = append(slicesByService[key], slice)
	}

	serviceMap := make(map[string]*apiv1alpha1.ManagedService)

	getOrCreate := func(svcNS, svcName string, strategy mcgatewayv1alpha1.MinecraftDistributionStrategy) *apiv1alpha1.ManagedService {
		key := svcNS + "/" + svcName
		if svc, ok := serviceMap[key]; ok {
			return svc
		}
		svc := &apiv1alpha1.ManagedService{
			NamespacedName:       key,
			Namespace:            svcNS,
			Name:                 svcName,
			DistributionStrategy: toProtoDistStrategy(strategy),
			Servers:              buildServers(slicesByService[key], podAnnotations),
		}
		serviceMap[key] = svc
		return svc
	}

	for _, joinRoute := range routes.Join {
		rules := buildJoinRules(joinRoute.JoinFilterRules())
		priority := uint32(joinRoute.Priority())
		for _, backendRef := range joinRoute.BackendRefs() {
			svcNS := joinRoute.GetNamespace()
			if backendRef.Namespace != nil {
				svcNS = string(*backendRef.Namespace)
			}
			svc := getOrCreate(svcNS, string(backendRef.Name), backendRef.DistributionStrategy)
			svc.Routes = append(svc.Routes, &apiv1alpha1.Route{
				Priority: priority,
				IsJoin:   true,
				Rules:    rules,
			})
		}
	}

	for _, fallbackRoute := range routes.Fallback {
		rules := buildFallbackRules(fallbackRoute.FallbackFilterRules(), backends)
		priority := uint32(fallbackRoute.Priority())
		for _, backendRef := range fallbackRoute.BackendRefs() {
			svcNS := fallbackRoute.GetNamespace()
			if backendRef.Namespace != nil {
				svcNS = string(*backendRef.Namespace)
			}
			svc := getOrCreate(svcNS, string(backendRef.Name), backendRef.DistributionStrategy)
			svc.Routes = append(svc.Routes, &apiv1alpha1.Route{
				Priority:   priority,
				IsFallback: true,
				Rules:      rules,
			})
		}
	}

	services := make([]*apiv1alpha1.ManagedService, 0, len(serviceMap))
	for _, svc := range serviceMap {
		services = append(services, svc)
	}

	return ListenerSnapshot{
		GatewayNamespace: gateway.Namespace,
		GatewayName:      gateway.Name,
		ListenerName:     string(listener.GetName()),
		Services:         services,
	}
}

// BuildSnapshot constructs a Snapshot from the full per-gateway, per-listener cache.
func BuildSnapshot(cache GatewaySnapshotCache) Snapshot {
	gen := generation.Add(1)
	s := Snapshot{
		byGateway:  make(map[string]map[string]*ListenerSnapshot),
		Generation: fmt.Sprintf("%d", gen),
	}
	for gwName, listeners := range cache {
		key := gwName.String()
		s.byGateway[key] = make(map[string]*ListenerSnapshot)
		for listenerName, ls := range listeners {
			lsCopy := ls
			s.byGateway[key][listenerName] = &lsCopy
		}
	}
	return s
}

func buildServers(slices []discoveryv1.EndpointSlice, podAnnotations map[string]map[string]string) []*apiv1alpha1.ManagedServer {
	var servers []*apiv1alpha1.ManagedServer
	for _, slice := range slices {
		var port uint32
		if len(slice.Ports) > 0 && slice.Ports[0].Port != nil {
			port = uint32(*slice.Ports[0].Port)
		}
		for _, ep := range slice.Endpoints {
			if len(ep.Addresses) == 0 {
				continue
			}
			ip := ep.Addresses[0]
			name := ip
			uniqueID := ip
			var podKey string
			if ep.TargetRef != nil && ep.TargetRef.Name != "" {
				name = ep.TargetRef.Name
				uniqueID = ep.TargetRef.Namespace + "-" + ep.TargetRef.Name
				podKey = ep.TargetRef.Namespace + "/" + ep.TargetRef.Name
			}
			server := &apiv1alpha1.ManagedServer{
				UniqueId: uniqueID,
				Name:     name,
				Ip:       ip,
				Port:     port,
			}
			if podKey != "" {
				if ann, ok := podAnnotations[podKey]; ok {
					currentStr, hasCurrent := ann[AnnotationCurrentPlayers]
					maxStr, hasMax := ann[AnnotationMaxPlayers]
					if hasCurrent && hasMax {
						current, err1 := strconv.ParseUint(currentStr, 10, 32)
						max, err2 := strconv.ParseUint(maxStr, 10, 32)
						if err1 == nil && err2 == nil {
							c, m := uint32(current), uint32(max)
							server.CurrentPlayers = &c
							server.MaxPlayers = &m
						}
					}
				}
			}
			servers = append(servers, server)
		}
	}
	return servers
}

func buildJoinRules(ruleSets []mcgatewayv1alpha1.MinecraftJoinFilterRuleSet) []*apiv1alpha1.OptionRuleSet {
	result := make([]*apiv1alpha1.OptionRuleSet, 0, len(ruleSets))
	for _, rs := range ruleSets {
		protoRules := make([]*apiv1alpha1.Rule, 0, len(rs.Rules))
		for _, r := range rs.Rules {
			protoRules = append(protoRules, buildRule(r.Domain, r.Permission, ""))
		}
		result = append(result, &apiv1alpha1.OptionRuleSet{
			Type:  toProtoRuleType(rs.Type),
			Rules: protoRules,
		})
	}
	return result
}

func buildFallbackRules(ruleSets []mcgatewayv1alpha1.MinecraftFallbackFilterRuleSet, backends []discoveryv1.EndpointSlice) []*apiv1alpha1.OptionRuleSet {
	result := make([]*apiv1alpha1.OptionRuleSet, 0, len(ruleSets))
	for _, rs := range ruleSets {
		protoRules := make([]*apiv1alpha1.Rule, 0, len(rs.Rules))
		for _, r := range rs.Rules {
			// Expand FallbackFor label selector to concrete service namespaced names.
			fallbackRefs := expandFallbackFor(r.FallbackFor, backends)
			if len(fallbackRefs) > 0 {
				for _, ref := range fallbackRefs {
					protoRules = append(protoRules, buildRule(r.Domain, r.Permission, ref))
				}
			} else {
				protoRules = append(protoRules, buildRule(r.Domain, r.Permission, ""))
			}
		}
		result = append(result, &apiv1alpha1.OptionRuleSet{
			Type:  toProtoRuleType(rs.Type),
			Rules: protoRules,
		})
	}
	return result
}

// expandFallbackFor resolves a LabelSelector to service namespaced names by matching
// against EndpointSlice labels (which carry the kubernetes.io/service-name label).
func expandFallbackFor(sel metav1.LabelSelector, backends []discoveryv1.EndpointSlice) []string {
	selector, err := metav1.LabelSelectorAsSelector(&sel)
	if err != nil || selector.Empty() {
		return nil
	}
	seen := make(map[string]struct{})
	var refs []string
	for _, slice := range backends {
		svcName, ok := slice.Labels[labelServiceName]
		if !ok {
			continue
		}
		if !selector.Matches(labels.Set(slice.Labels)) {
			continue
		}
		ref := slice.Namespace + "/" + svcName
		if _, ok := seen[ref]; ok {
			continue
		}
		seen[ref] = struct{}{}
		refs = append(refs, ref)
	}
	return refs
}

func buildRule(domain, permission, fallbackFor string) *apiv1alpha1.Rule {
	r := &apiv1alpha1.Rule{}
	if domain != "" {
		r.Domain = &domain
	}
	if permission != "" {
		r.Permission = &permission
	}
	if fallbackFor != "" {
		r.FallbackFor = &fallbackFor
	}
	return r
}

func toProtoRuleType(t mcgatewayv1alpha1.MinecraftFilterRuleType) apiv1alpha1.RuleType {
	switch t {
	case mcgatewayv1alpha1.MinecraftFilterRuleAll:
		return apiv1alpha1.RuleType_ALL
	case mcgatewayv1alpha1.MinecraftFilterRuleAny:
		return apiv1alpha1.RuleType_ANY
	case mcgatewayv1alpha1.MinecraftFilterRuleNone:
		return apiv1alpha1.RuleType_NONE
	default:
		return apiv1alpha1.RuleType_ALL
	}
}

func toProtoDistStrategy(s mcgatewayv1alpha1.MinecraftDistributionStrategy) apiv1alpha1.DistributionStrategy {
	switch s.Type {
	case mcgatewayv1alpha1.MinecraftDistributionStrategyLeastPlayers:
		return apiv1alpha1.DistributionStrategy_LEAST_PLAYERS
	default:
		return apiv1alpha1.DistributionStrategy_RANDOM
	}
}
