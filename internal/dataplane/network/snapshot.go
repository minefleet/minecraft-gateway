package network

import (
	"fmt"
	"sort"
	"strconv"
	"sync/atomic"

	discoveryv1 "k8s.io/api/discovery/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/apimachinery/pkg/types"
	mcgatewayv1alpha1 "minefleet.dev/minecraft-gateway/api/controller/v1alpha1"
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
	// Streams carries the proxy stream state. It is shared with the
	// PlayerTransfer reconciler so transfers can be pushed to proxies.
	Streams *StreamManager
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

// All returns every listener snapshot in the aggregate.
func (s *Snapshot) All() []*ListenerSnapshot {
	var out []*ListenerSnapshot
	for _, byListener := range s.byGateway {
		for _, ls := range byListener {
			out = append(out, ls)
		}
	}
	return out
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

	serviceMap := make(map[string]*Service)

	getOrCreate := func(svcNS, svcName string, strategy mcgatewayv1alpha1.MinecraftDistributionStrategy) *Service {
		key := svcNS + "/" + svcName
		if svc, ok := serviceMap[key]; ok {
			return svc
		}
		svc := &Service{
			NamespacedName:       key,
			Namespace:            svcNS,
			Name:                 svcName,
			Labels:               mergeSliceLabels(slicesByService[key]),
			DistributionStrategy: toDistributionStrategy(strategy),
			Servers:              buildServers(slicesByService[key], podAnnotations),
		}
		serviceMap[key] = svc
		return svc
	}

	for _, joinRoute := range routes.Join {
		ruleSets := buildJoinRuleSets(joinRoute.JoinFilterRules())
		priority := uint32(joinRoute.Priority())
		for _, backendRef := range joinRoute.BackendRefs() {
			svcNS := joinRoute.GetNamespace()
			if backendRef.Namespace != nil {
				svcNS = string(*backendRef.Namespace)
			}
			svc := getOrCreate(svcNS, string(backendRef.Name), backendRef.DistributionStrategy)
			svc.JoinRoutes = append(svc.JoinRoutes, Route{Priority: priority, RuleSets: ruleSets})
		}
	}

	for _, fallbackRoute := range routes.Fallback {
		ruleSets := buildFallbackRuleSets(fallbackRoute.FallbackFilterRules(), backends)
		priority := uint32(fallbackRoute.Priority())
		for _, backendRef := range fallbackRoute.BackendRefs() {
			svcNS := fallbackRoute.GetNamespace()
			if backendRef.Namespace != nil {
				svcNS = string(*backendRef.Namespace)
			}
			svc := getOrCreate(svcNS, string(backendRef.Name), backendRef.DistributionStrategy)
			svc.FallbackRoutes = append(svc.FallbackRoutes, Route{Priority: priority, RuleSets: ruleSets})
		}
	}

	// Sorted so routing decisions and pushed config are reproducible.
	services := make([]*Service, 0, len(serviceMap))
	for _, key := range sortedKeysOf(serviceMap) {
		services = append(services, serviceMap[key])
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

func buildServers(slices []discoveryv1.EndpointSlice, podAnnotations map[string]map[string]string) []*Server {
	var servers []*Server
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
			server := &Server{
				UniqueID: uniqueID,
				Name:     name,
				IP:       ip,
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

func buildJoinRuleSets(ruleSets []mcgatewayv1alpha1.MinecraftJoinFilterRuleSet) []RuleSet {
	result := make([]RuleSet, 0, len(ruleSets))
	for _, rs := range ruleSets {
		rules := make([]Rule, 0, len(rs.Rules))
		for _, r := range rs.Rules {
			rules = append(rules, Rule{Domain: r.Domain, Permission: r.Permission})
		}
		result = append(result, RuleSet{Type: toRuleType(rs.Type), Rules: rules})
	}
	return result
}

func buildFallbackRuleSets(ruleSets []mcgatewayv1alpha1.MinecraftFallbackFilterRuleSet, backends []discoveryv1.EndpointSlice) []RuleSet {
	result := make([]RuleSet, 0, len(ruleSets))
	for _, rs := range ruleSets {
		rules := make([]Rule, 0, len(rs.Rules))
		for _, r := range rs.Rules {
			// Expand the FallbackFor label selector to concrete service names.
			fallbackRefs := expandFallbackFor(r.FallbackFor, backends)
			if len(fallbackRefs) == 0 {
				rules = append(rules, Rule{Domain: r.Domain, Permission: r.Permission})
				continue
			}
			for _, ref := range fallbackRefs {
				rules = append(rules, Rule{Domain: r.Domain, Permission: r.Permission, FallbackFor: ref})
			}
		}
		result = append(result, RuleSet{Type: toRuleType(rs.Type), Rules: rules})
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
	for _, slice := range backends {
		svcName, ok := slice.Labels[labelServiceName]
		if !ok {
			continue
		}
		if !selector.Matches(labels.Set(slice.Labels)) {
			continue
		}
		seen[slice.Namespace+"/"+svcName] = struct{}{}
	}
	return sortedKeys(seen)
}

// mergeSliceLabels collects the labels of a service's EndpointSlices, which is
// the same label source fallbackFor selectors match against.
func mergeSliceLabels(slices []discoveryv1.EndpointSlice) map[string]string {
	if len(slices) == 0 {
		return nil
	}
	merged := make(map[string]string)
	for _, slice := range slices {
		for k, v := range slice.Labels {
			merged[k] = v
		}
	}
	return merged
}

func sortedKeys(set map[string]struct{}) []string {
	if len(set) == 0 {
		return nil
	}
	out := make([]string, 0, len(set))
	for k := range set {
		out = append(out, k)
	}
	sort.Strings(out)
	return out
}

func sortedKeysOf[V any](m map[string]V) []string {
	out := make([]string, 0, len(m))
	for k := range m {
		out = append(out, k)
	}
	sort.Strings(out)
	return out
}
