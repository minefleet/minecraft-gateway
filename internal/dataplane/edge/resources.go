package edge

import (
	"encoding/json"
	"fmt"
	"time"

	clusterv3 "github.com/envoyproxy/go-control-plane/envoy/config/cluster/v3"
	corev3 "github.com/envoyproxy/go-control-plane/envoy/config/core/v3"
	endpointv3 "github.com/envoyproxy/go-control-plane/envoy/config/endpoint/v3"
	listenerv3 "github.com/envoyproxy/go-control-plane/envoy/config/listener/v3"
	dynmodulesv3 "github.com/envoyproxy/go-control-plane/envoy/extensions/dynamic_modules/v3"
	dynmodlistenerv3 "github.com/envoyproxy/go-control-plane/envoy/extensions/filters/listener/dynamic_modules/v3"
	dynmodnetworkv3 "github.com/envoyproxy/go-control-plane/envoy/extensions/filters/network/dynamic_modules/v3"
	tcpproxyv3 "github.com/envoyproxy/go-control-plane/envoy/extensions/filters/network/tcp_proxy/v3"
	"google.golang.org/protobuf/types/known/anypb"
	"google.golang.org/protobuf/types/known/durationpb"
	"google.golang.org/protobuf/types/known/wrapperspb"
)

const (
	// NodeID is the Envoy node ID used in the DaemonSet bootstrap config.
	NodeID = "minefleet-edge"

	listenerName    = "minecraft_gateway"
	minecraftPort   = uint32(25565)
	maxScannedBytes = 1024
)

// buildListenerResources builds the Envoy Listener xDS resource for Minecraft routing.
func buildListenerResources(snap Snapshot) (*listenerv3.Listener, error) {
	filterCfgJSON, err := json.Marshal(map[string]any{
		"domain_mappings":   snap.DomainMapping,
		"max_scanned_bytes": maxScannedBytes,
		"reject_unknown":    snap.RejectUnknown,
	})
	if err != nil {
		return nil, fmt.Errorf("marshal filter config: %w", err)
	}

	filterCfgAny, err := anypb.New(&wrapperspb.StringValue{Value: string(filterCfgJSON)})
	if err != nil {
		return nil, err
	}
	emptyAny, err := anypb.New(&wrapperspb.StringValue{Value: ""})
	if err != nil {
		return nil, err
	}

	validatorAny, err := anypb.New(&dynmodlistenerv3.DynamicModuleListenerFilter{
		DynamicModuleConfig: &dynmodulesv3.DynamicModuleConfig{Name: "minefleet_edge"},
		FilterName:          "validator",
		FilterConfig:        filterCfgAny,
	})
	if err != nil {
		return nil, err
	}

	routerAny, err := anypb.New(&dynmodnetworkv3.DynamicModuleNetworkFilter{
		DynamicModuleConfig: &dynmodulesv3.DynamicModuleConfig{Name: "minefleet_edge"},
		FilterName:          "router",
		FilterConfig:        emptyAny,
	})
	if err != nil {
		return nil, err
	}

	tcpProxyAny, err := anypb.New(&tcpproxyv3.TcpProxy{
		StatPrefix:       "minecraft",
		ClusterSpecifier: &tcpproxyv3.TcpProxy_Cluster{Cluster: "default"},
	})
	if err != nil {
		return nil, err
	}

	return &listenerv3.Listener{
		Name: listenerName,
		Address: &corev3.Address{
			Address: &corev3.Address_SocketAddress{
				SocketAddress: &corev3.SocketAddress{
					Address:       "0.0.0.0",
					PortSpecifier: &corev3.SocketAddress_PortValue{PortValue: minecraftPort},
				},
			},
		},
		ListenerFilters: []*listenerv3.ListenerFilter{
			{
				Name:       "envoy.filters.listener.dynamic_modules",
				ConfigType: &listenerv3.ListenerFilter_TypedConfig{TypedConfig: validatorAny},
			},
		},
		FilterChains: []*listenerv3.FilterChain{
			{
				Name: "gateway",
				Filters: []*listenerv3.Filter{
					{
						Name:       "envoy.filters.network.dynamic_modules",
						ConfigType: &listenerv3.Filter_TypedConfig{TypedConfig: routerAny},
					},
					{
						Name:       "envoy.filters.network.tcp_proxy",
						ConfigType: &listenerv3.Filter_TypedConfig{TypedConfig: tcpProxyAny},
					},
				},
			},
		},
	}, nil
}

// buildClusterResources builds Envoy Cluster xDS resources from a GatewaySnapshot.
func buildClusterResources(snap Snapshot) []*clusterv3.Cluster {
	result := make([]*clusterv3.Cluster, 0, len(snap.Clusters))
	for _, c := range snap.Clusters {
		lbEndpoints := make([]*endpointv3.LbEndpoint, 0, len(c.Endpoints))
		for _, ep := range c.Endpoints {
			lbEndpoints = append(lbEndpoints, &endpointv3.LbEndpoint{
				HostIdentifier: &endpointv3.LbEndpoint_Endpoint{
					Endpoint: &endpointv3.Endpoint{
						Address: &corev3.Address{
							Address: &corev3.Address_SocketAddress{
								SocketAddress: &corev3.SocketAddress{
									Address:       ep.Address,
									PortSpecifier: &corev3.SocketAddress_PortValue{PortValue: ep.Port},
								},
							},
						},
					},
				},
			})
		}
		result = append(result, &clusterv3.Cluster{
			Name:           c.Name,
			ConnectTimeout: durationpb.New(time.Second),
			ClusterDiscoveryType: &clusterv3.Cluster_Type{
				Type: clusterv3.Cluster_STRICT_DNS,
			},
			LoadAssignment: &endpointv3.ClusterLoadAssignment{
				ClusterName: c.Name,
				Endpoints: []*endpointv3.LocalityLbEndpoints{
					{LbEndpoints: lbEndpoints},
				},
			},
		})
	}
	return result
}
