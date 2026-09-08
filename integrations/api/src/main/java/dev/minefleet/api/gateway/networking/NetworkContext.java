package dev.minefleet.api.gateway.networking;

import dev.minefleet.api.gateway.networking.v1alpha1.Api;

import java.util.UUID;

/**
 * Identifies this proxy and the gateway listener it serves. The controller uses
 * it to decide which routing configuration to push and which players belong to
 * which listener.
 */
public record NetworkContext(String proxyId, String gatewayNamespace, String gatewayName, String listenerName) {

    public static NetworkContext fromEnv() {
        String namespace = System.getenv("NAMESPACE");
        String gatewayName = System.getenv("GATEWAY_NAME");
        String listenerName = System.getenv("LISTENER_NAME");
        if (namespace == null || gatewayName == null || listenerName == null) {
            throw new IllegalStateException(
                    "Missing required environment variables: NAMESPACE, GATEWAY_NAME, LISTENER_NAME");
        }
        // Kubernetes sets HOSTNAME to the pod name, which is unique and stable
        // for the pod's lifetime.
        String proxyId = System.getenv("HOSTNAME");
        if (proxyId == null || proxyId.isBlank()) {
            proxyId = "proxy-" + UUID.randomUUID();
        }
        return new NetworkContext(proxyId, namespace, gatewayName, listenerName);
    }

    public Api.ProxyHello toHello() {
        return Api.ProxyHello.newBuilder()
                .setProxyId(proxyId)
                .setGatewayNamespace(gatewayNamespace)
                .setGatewayName(gatewayName)
                .setListenerName(listenerName)
                .build();
    }
}
