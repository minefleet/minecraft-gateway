package dev.minefleet.api.gateway.networking.snapshot;

import dev.minefleet.api.gateway.networking.v1alpha1.Api;

public record NetworkSnapshotContext(String gatewayNamespace, String gatewayName, String listenerName) {
    public static NetworkSnapshotContext fromEnv() {
        String namespace = System.getenv("NAMESPACE");
        String gatewayName = System.getenv("GATEWAY_NAME");
        String listenerName = System.getenv("LISTENER_NAME");
        if (namespace == null || gatewayName == null || listenerName == null) {
            throw new IllegalStateException(
                    "Missing required environment variables: NAMESPACE, GATEWAY_NAME, LISTENER_NAME");
        }
        return new NetworkSnapshotContext(namespace, gatewayName, listenerName);
    }

    public Api.GetSnapshotRequest toProto() {
        return Api.GetSnapshotRequest.newBuilder()
                .setGatewayNamespace(gatewayNamespace)
                .setGatewayName(gatewayName)
                .setListenerName(listenerName)
                .build();
    }
}
