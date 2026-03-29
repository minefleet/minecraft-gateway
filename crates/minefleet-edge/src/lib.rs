mod validator;
mod router;


#[unsafe(no_mangle)]
pub extern "C" fn envoy_dynamic_module_on_program_init() -> *const ::std::os::raw::c_char {
    envoy_proxy_dynamic_modules_rust_sdk::NEW_LISTENER_FILTER_CONFIG_FUNCTION
        .get_or_init(|| validator::new_filter_config);
    envoy_proxy_dynamic_modules_rust_sdk::NEW_NETWORK_FILTER_CONFIG_FUNCTION
        .get_or_init(|| router::new_network_config);
    if init() {
        envoy_proxy_dynamic_modules_rust_sdk::abi::envoy_dynamic_modules_abi_version.as_ptr()
            as *const ::std::os::raw::c_char
    } else {
        ::std::ptr::null()
    }
}

fn init() -> bool {
    true
}