package dev.minefleet.api.gateway.networking.snapshot;

import dev.minefleet.api.gateway.networking.ManagedServer;
import dev.minefleet.api.gateway.networking.ServerRegistrar;
import dev.minefleet.api.gateway.networking.ServerRegistrarBulkException;
import dev.minefleet.api.gateway.networking.ServerRegistrarException;
import io.grpc.Channel;

import java.util.List;
import java.util.concurrent.Executors;
import java.util.concurrent.ScheduledExecutorService;
import java.util.concurrent.TimeUnit;

public class NetworkSnapshotReconciler {

    @FunctionalInterface
    private interface RegistrarAction {
        void apply(List<ManagedServer> servers) throws ServerRegistrarBulkException;
    }

    private record RetryQueue(List<ManagedServer> servers, int retriesLeft) {
        static RetryQueue empty() { return new RetryQueue(List.of(), 0); }
        boolean hasPending() { return !servers.isEmpty() && retriesLeft > 0; }
        RetryQueue decrement(List<ManagedServer> stillFailed) {
            return stillFailed.isEmpty() ? empty() : new RetryQueue(stillFailed, retriesLeft - 1);
        }
    }

    private volatile NetworkSnapshot lastSnapshot;
    private final NetworkSnapshotContext context;
    private final int retries, intervalSeconds;
    private final NetworkSnapshotFetcher fetcher;
    private final ScheduledExecutorService scheduler = Executors.newSingleThreadScheduledExecutor();
    private final ServerRegistrar registrar;

    private RetryQueue pendingRegister = RetryQueue.empty();
    private RetryQueue pendingUnregister = RetryQueue.empty();

    public NetworkSnapshotReconciler(Channel channel, NetworkSnapshotContext context, ServerRegistrar registrar, int retries, int intervalSeconds) {
        this.context = context;
        this.retries = retries;
        this.intervalSeconds = intervalSeconds;
        this.fetcher = new NetworkSnapshotFetcher(channel, null);
        this.registrar = registrar;
    }

    // Applies the action, returning any servers that failed.
    private List<ManagedServer> tryApply(List<ManagedServer> servers, RegistrarAction action) {
        if (servers.isEmpty()) return List.of();
        try {
            action.apply(servers);
            return List.of();
        } catch (ServerRegistrarBulkException e) {
            return e.failures().stream().map(ServerRegistrarException::server).toList();
        }
    }

    private void reconcile(NetworkSnapshot newSnapshot) {
        NetworkSnapshot prev = lastSnapshot;
        lastSnapshot = newSnapshot;
        var delta = newSnapshot.serverSnapshotDelta(prev);

        // Retry pending from previous cycles before processing new delta
        if (pendingRegister.hasPending()) {
            pendingRegister = pendingRegister.decrement(tryApply(pendingRegister.servers(), registrar::registerOrUpdate));
        }
        if (pendingUnregister.hasPending()) {
            pendingUnregister = pendingUnregister.decrement(tryApply(pendingUnregister.servers(), registrar::unregister));
        }

        // Apply new delta, queueing any failures for retry
        List<ManagedServer> failedRegister = tryApply(delta.addedOrUpdatedServers(), registrar::registerOrUpdate);
        if (!failedRegister.isEmpty()) {
            pendingRegister = new RetryQueue(failedRegister, retries);
        }

        List<ManagedServer> failedUnregister = tryApply(delta.removedServers(), registrar::unregister);
        if (!failedUnregister.isEmpty()) {
            pendingUnregister = new RetryQueue(failedUnregister, retries);
        }
    }

    public void start() {
        scheduler.scheduleAtFixedRate(
                () -> fetcher.fetchSnapshot(context, this::reconcile),
                0, intervalSeconds, TimeUnit.SECONDS
        );
    }

    public void stop() {
        scheduler.shutdown();
    }
}