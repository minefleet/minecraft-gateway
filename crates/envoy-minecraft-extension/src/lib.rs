//! A Minecraft Java Edition handshake-based routing listener filter.
//!
//! This filter demonstrates:
//! 1. Parsing Minecraft handshake (packet 0x00) to extract `server_address`.
//! 2. Domain to "target" mapping with wildcard support.
//! 3. Handling connections with and without a parseable handshake.
//!
//! Configuration format (JSON):
//! ```json
//! {
//!   "default_server_name": "default.example.com",
//!   "domain_mappings": {
//!     "play.example.com": "fc_play",
//!     "*.example.com": "fc_wildcard"
//!   },
//!   "reject_unknown": true,
//!   "max_read_bytes": 1024,
//!   "metadata_namespace": "dev.minefleet.edge",
//!   "metadata_key": "selected_filter_chain"
//! }
//! ```
//!
//! The filter writes the chosen value into dynamic metadata:
//! namespace = metadata_namespace, key = metadata_key
//! so you can use `filter_chain_matcher` to select a named filter chain.

use envoy_proxy_dynamic_modules_rust_sdk::*;
use serde::{Deserialize, Serialize};
use std::collections::HashMap;

fn default_max_read_bytes() -> usize {
    1024
}

fn default_metadata_namespace() -> String {
    "dev.minefleet.edge".to_string()
}

fn default_metadata_key() -> String {
    "selected_filter_chain".to_string()
}

/// Configuration data parsed from the filter config JSON.
#[derive(Serialize, Deserialize, Debug, Clone)]
struct McRouterConfigData {
    #[serde(default)]
    default_server_name: Option<String>,
    #[serde(default)]
    domain_mappings: HashMap<String, String>,
    #[serde(default)]
    reject_unknown: bool,
    #[serde(default = "default_max_read_bytes")]
    max_read_bytes: usize,
    #[serde(default = "default_metadata_namespace")]
    metadata_namespace: String,
    #[serde(default = "default_metadata_key")]
    metadata_key: String,
}

impl Default for McRouterConfigData {
    fn default() -> Self {
        McRouterConfigData {
            default_server_name: None,
            domain_mappings: HashMap::new(),
            reject_unknown: true,
            max_read_bytes: default_max_read_bytes(),
            metadata_namespace: default_metadata_namespace(),
            metadata_key: default_metadata_key(),
        }
    }
}

/// Result of Minecraft routing.
#[derive(Debug, Clone, PartialEq)]
pub enum McRoutingResult {
    Routed { host: String, target: String },
    NoMapping { host: String },
    NoHandshake { default: Option<String> },
    NotMinecraft,
    Rejected { reason: String },
    NeedMoreData,
    Invalid { reason: String },
}

/// The filter configuration.
pub struct McRouterFilterConfig {
    default_server_name: Option<String>,
    domain_mappings: HashMap<String, String>,
    reject_unknown: bool,
    max_read_bytes: usize,
    metadata_namespace: String,
    metadata_key: String,
    matches: EnvoyCounterId,
    misses: EnvoyCounterId,
}

/// Creates a new Minecraft router filter configuration.
pub fn new_filter_config<EC: EnvoyListenerFilterConfig, ELF: EnvoyListenerFilter>(
    envoy_filter_config: &mut EC,
    _name: &str,
    config: &[u8],
) -> Option<Box<dyn ListenerFilterConfig<ELF>>> {
    let config_data: McRouterConfigData = if config.is_empty() {
        McRouterConfigData::default()
    } else {
        match serde_json::from_slice(config) {
            Ok(cfg) => cfg,
            Err(err) => {
                eprintln!("Error parsing Minecraft router config: {err}");
                return None;
            }
        }
    };

    let matches = envoy_filter_config
        .define_counter("minecraft_router_matches_total")
        .expect("Failed to define matches counter");

    let misses = envoy_filter_config
        .define_counter("minecraft_router_misses_total")
        .expect("Failed to define misses counter");

    Some(Box::new(McRouterFilterConfig {
        default_server_name: config_data.default_server_name,
        domain_mappings: config_data.domain_mappings,
        reject_unknown: config_data.reject_unknown,
        max_read_bytes: config_data.max_read_bytes,
        metadata_namespace: config_data.metadata_namespace,
        metadata_key: config_data.metadata_key,
        matches,
        misses,
    }))
}

impl<ELF: EnvoyListenerFilter> ListenerFilterConfig<ELF> for McRouterFilterConfig {
    fn new_listener_filter(&self, _envoy: &mut ELF) -> Box<dyn ListenerFilter<ELF>> {
        Box::new(McRouterFilter {
            default_server_name: self.default_server_name.clone(),
            domain_mappings: self.domain_mappings.clone(),
            reject_unknown: self.reject_unknown,
            max_read_bytes: self.max_read_bytes,
            metadata_namespace: self.metadata_namespace.clone(),
            metadata_key: self.metadata_key.clone(),
            matches: self.matches,
            misses: self.misses,
            buf: Vec::with_capacity(self.max_read_bytes.min(2048)),
            decided: false,
        })
    }
}

/// The Minecraft router filter.
struct McRouterFilter {
    default_server_name: Option<String>,
    domain_mappings: HashMap<String, String>,
    reject_unknown: bool,
    max_read_bytes: usize,
    metadata_namespace: String,
    metadata_key: String,
    matches: EnvoyCounterId,
    misses: EnvoyCounterId,

    buf: Vec<u8>,
    decided: bool,
}

impl McRouterFilter {
    /// Look up the target for a given host. Supports exact match then "*.suffix" wildcard.
    pub fn lookup_target(&self, host: &str) -> Option<&String> {
        if let Some(t) = self.domain_mappings.get(host) {
            return Some(t);
        }

        for (domain, target) in &self.domain_mappings {
            if let Some(suffix) = domain.strip_prefix("*.") {
                // require subdomain
                if host.len() > suffix.len()
                    && host.ends_with(suffix)
                    && host.as_bytes()[host.len() - suffix.len() - 1] == b'.'
                {
                    return Some(target);
                }
            }
        }
        None
    }

    fn write_metadata(&self, envoy_filter: &mut impl EnvoyListenerFilter, value: &str) {
        envoy_filter.set_dynamic_metadata_string(&self.metadata_namespace, &self.metadata_key, value);
    }

    /// Process data and determine routing.
    pub fn process(&self, data: &[u8]) -> McRoutingResult {
        if data.len() < 2 {
            return McRoutingResult::NeedMoreData;
        }

        // Try parse handshake.
        match parse_handshake_server_address(data) {
            ParseHandshake::NeedMore => McRoutingResult::NeedMoreData,
            ParseHandshake::NotHandshake => McRoutingResult::NotMinecraft,
            ParseHandshake::Invalid(reason) => McRoutingResult::Invalid { reason: reason.to_string() },
            ParseHandshake::Ok { host } => {
                if let Some(target) = self.lookup_target(&host) {
                    McRoutingResult::Routed {
                        host,
                        target: target.clone(),
                    }
                } else if self.reject_unknown {
                    McRoutingResult::Rejected {
                        reason: format!("Unknown hostname: {host}"),
                    }
                } else {
                    McRoutingResult::NoMapping { host }
                }
            }
        }
    }
}

impl<ELF: EnvoyListenerFilter> ListenerFilter<ELF> for McRouterFilter {
    fn on_accept(
        &mut self,
        _envoy_filter: &mut ELF,
    ) -> abi::envoy_dynamic_module_type_on_listener_filter_status {
        // Need on_data to inspect bytes.
        abi::envoy_dynamic_module_type_on_listener_filter_status::Continue
    }

    fn on_data(
        &mut self,
        envoy_filter: &mut ELF,
    ) -> abi::envoy_dynamic_module_type_on_listener_filter_status {
        if self.decided {
            return abi::envoy_dynamic_module_type_on_listener_filter_status::Continue;
        }

        // Peek bytes currently available and accumulate up to max_read_bytes.
        let available_room = self.max_read_bytes.saturating_sub(self.buf.len());
        if available_room > 0 && let Some(chunk) = envoy_filter.get_buffer_chunk() {
            let slice: &[u8] = chunk.as_slice();
            let take = available_room.min(slice.len());
            self.buf.extend_from_slice(&slice[..take]);
        }

        let res = self.process(&self.buf);

        match res {
            McRoutingResult::NeedMoreData => {
                if self.buf.len() >= self.max_read_bytes {
                    // Stop waiting; apply fallback.
                    if let Some(def) = &self.default_server_name {
                        self.write_metadata(envoy_filter, def);
                    } else {
                        self.write_metadata(envoy_filter, "default");
                    }
                    let _ = envoy_filter.increment_counter(self.misses, 1);
                    self.decided = true;
                    return abi::envoy_dynamic_module_type_on_listener_filter_status::Continue;
                }
                abi::envoy_dynamic_module_type_on_listener_filter_status::StopIteration
            }

            McRoutingResult::Routed { host: _, target } => {
                self.write_metadata(envoy_filter, &target);
                let _ = envoy_filter.increment_counter(self.matches, 1);
                self.decided = true;
                abi::envoy_dynamic_module_type_on_listener_filter_status::Continue
            }

            McRoutingResult::NoMapping { host: _ } => {
                // Fallback to default_server_name or "default"
                if let Some(def) = &self.default_server_name {
                    self.write_metadata(envoy_filter, def);
                } else {
                    self.write_metadata(envoy_filter, "default");
                }
                let _ = envoy_filter.increment_counter(self.misses, 1);
                self.decided = true;
                abi::envoy_dynamic_module_type_on_listener_filter_status::Continue
            }

            McRoutingResult::NoHandshake { default: _ } | McRoutingResult::NotMinecraft | McRoutingResult::Invalid { .. } => {
                if let Some(def) = &self.default_server_name {
                    self.write_metadata(envoy_filter, def);
                } else {
                    self.write_metadata(envoy_filter, "default");
                }
                let _ = envoy_filter.increment_counter(self.misses, 1);
                self.decided = true;
                abi::envoy_dynamic_module_type_on_listener_filter_status::Continue
            }

            McRoutingResult::Rejected { reason } => {
                envoy_filter.set_downstream_transport_failure_reason(&reason);
                // Still choose a default chain so Envoy can proceed; you can also design a "reject" chain.
                self.write_metadata(envoy_filter, "reject");
                let _ = envoy_filter.increment_counter(self.misses, 1);
                self.decided = true;
                abi::envoy_dynamic_module_type_on_listener_filter_status::Continue
            }
        }
    }

    fn on_close(&mut self, _envoy_filter: &mut ELF) {}
}

// ---------------- Minecraft parsing ----------------

enum ParseHandshake {
    Ok { host: String },
    NeedMore,
    NotHandshake,
    Invalid(&'static str),
}

/// Parse the Minecraft handshake's server_address field from the start of stream.
/// Returns:
/// - NeedMore if more bytes are required
/// - NotHandshake if it doesn't look like a handshake packet
/// - Invalid if malformed
fn parse_handshake_server_address(buf: &[u8]) -> ParseHandshake {
    let mut i = 0;

    let (packet_len, packet_len_len) = match read_varint(buf, i) {
        VarIntRead::NeedMore => return ParseHandshake::NeedMore,
        VarIntRead::Invalid => return ParseHandshake::Invalid("bad packet length varint"),
        VarIntRead::Ok(v, n) => (v as usize, n),
    };
    i += packet_len_len;

    // Need entire frame bytes present.
    if buf.len() < i + packet_len {
        return ParseHandshake::NeedMore;
    }

    let frame_end = i + packet_len;

    let (packet_id, packet_id_len) = match read_varint(buf, i) {
        VarIntRead::NeedMore => return ParseHandshake::NeedMore,
        VarIntRead::Invalid => return ParseHandshake::Invalid("bad packet id varint"),
        VarIntRead::Ok(v, n) => (v, n),
    };
    i += packet_id_len;

    if packet_id != 0x00 {
        return ParseHandshake::NotHandshake;
    }

    // protocol version (ignored)
    let (_protocol_version, protocol_version_len) = match read_varint(buf, i) {
        VarIntRead::NeedMore => return ParseHandshake::NeedMore,
        VarIntRead::Invalid => return ParseHandshake::Invalid("bad proto varint"),
        VarIntRead::Ok(v, n) => (v, n),
    };
    i += protocol_version_len;

    // server_address string length
    let (server_name_len, server_name_len_len) = match read_varint(buf, i) {
        VarIntRead::NeedMore => return ParseHandshake::NeedMore,
        VarIntRead::Invalid => return ParseHandshake::Invalid("bad string length varint"),
        VarIntRead::Ok(v, n) => (v as usize, n),
    };
    i += server_name_len_len;

    if server_name_len == 0 || server_name_len > 255 {
        return ParseHandshake::Invalid("server_address length out of range");
    }
    if i + server_name_len > frame_end {
        return ParseHandshake::NeedMore;
    }

    let raw = &buf[i..(i + server_name_len)];
    let mut host = match std::str::from_utf8(raw) {
        Ok(s) => s.to_string(),
        Err(_) => return ParseHandshake::Invalid("server_address not utf8"),
    };

    // Forge / proxies sometimes append extra fields separated by '\0'
    if let Some(idx) = host.find('\0') {
        host.truncate(idx);
    }

    ParseHandshake::Ok { host }
}

enum VarIntRead {
    Ok(i32, usize),
    NeedMore,
    Invalid,
}

const SEGMENT_BITS: u8 = 0x7F;
const CONTINUE_BIT: u8 = 0x80;

fn read_varint(buf: &[u8], start: usize) -> VarIntRead {

    let mut num_read: usize = 0;
    let mut result: i32 = 0;
    let mut position: u32 = 0;

    loop {
        let idx = start + num_read;
        if idx >= buf.len() {
            return VarIntRead::NeedMore;
        }

        let current_byte = buf[idx];

        result |= ((current_byte & SEGMENT_BITS) as i32) << position;

        num_read += 1;

        if (current_byte & CONTINUE_BIT) == 0 {
            return VarIntRead::Ok(result, num_read);
        }

        position += 7;
        if position >= 32 {
            return VarIntRead::Invalid;
        }
    }
}

