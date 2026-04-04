package dev.minefleet.api.gateway.networking;

import dev.minefleet.api.gateway.networking.player.KickReason;
import dev.minefleet.api.gateway.networking.player.NetworkPlayer;
import dev.minefleet.api.gateway.networking.route.NetworkRouter;
import dev.minefleet.api.gateway.networking.snapshot.NetworkSnapshot;
import dev.minefleet.api.gateway.networking.snapshot.NetworkSnapshotContext;
import dev.minefleet.api.gateway.networking.snapshot.NetworkSnapshotReconciler;
import io.grpc.Channel;
import io.grpc.LoadBalancerRegistry;
import io.grpc.ManagedChannelBuilder;
import io.grpc.NameResolverRegistry;
import io.grpc.internal.DnsNameResolverProvider;
import io.grpc.internal.PickFirstLoadBalancerProvider;

public class NetworkGateway {

    private volatile NetworkSnapshot currentSnapshot;
    private final NetworkSnapshotReconciler reconciler;

    private NetworkGateway(Builder builder) {
        this.reconciler = new NetworkSnapshotReconciler(
                builder.channel,
                builder.context,
                builder.registrar,
                builder.retries,
                builder.intervalSeconds,
                snapshot -> this.currentSnapshot = snapshot
        );
    }

    public void start() {
        reconciler.start();
    }

    public void stop() {
        reconciler.stop();
    }

    public void routeJoin(NetworkPlayer<?> player) {
        if (currentSnapshot == null) { player.kick(KickReason.NO_JOIN); return; }
        new NetworkRouter(currentSnapshot).routeJoin(player);
    }

    public void routeFallback(NetworkPlayer<?> player) {
        if (currentSnapshot == null) { player.kick(KickReason.NO_FALLBACK); return; }
        new NetworkRouter(currentSnapshot).routeFallback(player);
    }

    public static Builder builder() {
        return new Builder();
    }

    public static class Builder {
        private Channel channel;
        private NetworkSnapshotContext context;
        private ServerRegistrar registrar;
        private int retries = 3;
        private int intervalSeconds = 5;

        public Builder channel(Channel channel) {
            this.channel = channel;
            return this;
        }

        public Builder context(NetworkSnapshotContext context) {
            this.context = context;
            return this;
        }

        public Builder registrar(ServerRegistrar registrar) {
            this.registrar = registrar;
            return this;
        }

        public Builder retries(int retries) {
            this.retries = retries;
            return this;
        }

        public Builder intervalSeconds(int intervalSeconds) {
            this.intervalSeconds = intervalSeconds;
            return this;
        }

        public NetworkGateway build() {
            if (channel == null) channel = channelFromEnv();
            if (context == null) context = NetworkSnapshotContext.fromEnv();
            if (registrar == null) throw new IllegalStateException("registrar is required");
            return new NetworkGateway(this);
        }

        private static Channel channelFromEnv() {
            String host = System.getenv("GATEWAY_NETWORK_XDS_HOST");
            String portStr = System.getenv("GATEWAY_NETWORK_XDS_PORT");
            if (host == null || portStr == null) {
                throw new IllegalStateException(
                        "Missing required environment variables: GATEWAY_NETWORK_XDS_HOST, GATEWAY_NETWORK_XDS_PORT");
            }
            NameResolverRegistry.getDefaultRegistry().register(new DnsNameResolverProvider());
            LoadBalancerRegistry.getDefaultRegistry().register(new PickFirstLoadBalancerProvider());
            return ManagedChannelBuilder.forTarget("dns:///" + host + ":" + portStr)
                    .usePlaintext()
                    .build();
        }
    }
}