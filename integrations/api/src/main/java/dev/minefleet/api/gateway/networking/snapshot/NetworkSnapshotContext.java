package dev.minefleet.api.gateway.networking.snapshot;

import dev.minefleet.api.gateway.networking.v1alpha1.Api;

public record NetworkSnapshotContext(String gatewayNamespace, String gatewayName, String listenerName) {
    public Api.GetSnapshotRequest toProto() {
        return Api.GetSnapshotRequest.newBuilder()
                .setGatewayNamespace(gatewayNamespace)
                .setGatewayName(gatewayName)
                .setListenerName(listenerName)
                .build();
    }
}
