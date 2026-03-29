use envoy_proxy_dynamic_modules_rust_sdk::abi::*;
use envoy_proxy_dynamic_modules_rust_sdk::*;

pub fn new_network_config<EC: EnvoyNetworkFilterConfig, ENF: EnvoyNetworkFilter>(
    _envoy_filter_config: &mut EC,
    filter_name: &str,
    _config: &[u8],
) -> Option<Box<dyn NetworkFilterConfig<ENF>>> {
    if filter_name != "router" {
        panic!("Unknown network filter name: {filter_name}");
    }
    Some(Box::new(McNetworkFilterConfig))
}

struct McNetworkFilterConfig;

impl<ENF: EnvoyNetworkFilter> NetworkFilterConfig<ENF> for McNetworkFilterConfig {
    fn new_network_filter(&self, _envoy: &mut ENF) -> Box<dyn NetworkFilter<ENF>> {
        Box::new(McNetworkFilter { decided: false })
    }
}

struct McNetworkFilter {
    decided: bool,
}

impl<ENF: EnvoyNetworkFilter> NetworkFilter<ENF> for McNetworkFilter {
    fn on_new_connection(
        &mut self,
        envoy: &mut ENF,
    ) -> envoy_dynamic_module_type_on_network_filter_data_status {
        if !self.decided {
            if let Some(cluster) = envoy.get_dynamic_metadata_string("dev.minefleet.edge", "cluster") {
                eprintln!("[minefleet-edge/router] forwarding connection to cluster {cluster:?}");
                envoy.set_filter_state_typed(b"envoy.tcp_proxy.cluster", cluster.as_bytes());
            } else {
                eprintln!("[minefleet-edge/router] no cluster metadata found, tcp_proxy will use its default cluster");
            }
            self.decided = true;
        }
        envoy_dynamic_module_type_on_network_filter_data_status::Continue
    }
}
