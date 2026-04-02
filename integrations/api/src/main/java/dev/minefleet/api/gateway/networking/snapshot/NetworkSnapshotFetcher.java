package dev.minefleet.api.gateway.networking.snapshot;

import com.google.common.util.concurrent.FutureCallback;
import com.google.common.util.concurrent.Futures;
import dev.minefleet.api.gateway.networking.v1alpha1.Api;
import dev.minefleet.api.gateway.networking.v1alpha1.NetworkXDSGrpc;
import io.grpc.CallCredentials;
import io.grpc.Channel;
import org.jspecify.annotations.NonNull;

import java.util.concurrent.ForkJoinPool;
import java.util.function.Consumer;
import java.util.function.Function;
import java.util.function.Supplier;

public class NetworkSnapshotFetcher {
    private NetworkXDSGrpc.NetworkXDSFutureStub stub;

    public NetworkSnapshotFetcher(Channel channel, CallCredentials credentials) {
        this.stub = NetworkXDSGrpc.newFutureStub(channel);
        if(credentials != null) {
            this.stub = this.stub.withCallCredentials(credentials);
        }
    }

    public void fetchSnapshot(NetworkSnapshotContext context, Consumer<NetworkSnapshot> callback) {
        Futures.addCallback(this.stub.getSnapshot(context.toProto()), new FutureCallback<>() {
            @Override
            public void onSuccess(Api.GetSnapshotResponse response) {
                callback.accept(new NetworkSnapshot(response.getSnapshot()));
            }

            @Override
            public void onFailure(@NonNull Throwable t) {
                throw new RuntimeException(t);
            }
        }, ForkJoinPool.commonPool());
    }
}