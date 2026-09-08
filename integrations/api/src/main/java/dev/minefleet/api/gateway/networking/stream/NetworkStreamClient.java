package dev.minefleet.api.gateway.networking.stream;

import dev.minefleet.api.gateway.networking.NetworkContext;
import dev.minefleet.api.gateway.networking.v1alpha1.Api;
import dev.minefleet.api.gateway.networking.v1alpha1.NetworkXDSGrpc;
import io.grpc.Channel;
import io.grpc.stub.StreamObserver;

import java.util.List;
import java.util.concurrent.Executors;
import java.util.concurrent.ScheduledExecutorService;
import java.util.concurrent.ThreadLocalRandom;
import java.util.concurrent.TimeUnit;
import java.util.concurrent.atomic.AtomicBoolean;
import java.util.function.Consumer;
import java.util.function.Supplier;
import java.util.logging.Level;
import java.util.logging.Logger;

/**
 * The proxy's half of the persistent stream to the controller.
 *
 * <p>The stream replaces snapshot polling: the controller pushes server
 * registrations, routing decisions and move commands, while the proxy reports
 * player presence and asks for routing decisions. On disconnect the client
 * reconnects with exponential backoff, and callers must treat the stream as
 * unavailable in the meantime rather than routing against stale state.
 */
public class NetworkStreamClient {

    private static final Logger LOGGER = Logger.getLogger(NetworkStreamClient.class.getName());

    private static final long INITIAL_BACKOFF_MILLIS = 500;
    private static final long MAX_BACKOFF_MILLIS = 30_000;

    /** Connection state of the stream. */
    public enum State { DISCONNECTED, CONNECTING, READY }

    private final Channel channel;
    private final NetworkContext context;
    private final Consumer<Api.ControllerMessage> onMessage;
    /** Supplies the full presence set to replay whenever the stream (re)connects. */
    private final Supplier<List<Api.PresenceEntry>> presenceSupplier;

    private final ScheduledExecutorService scheduler =
            Executors.newSingleThreadScheduledExecutor(runnable -> {
                Thread thread = new Thread(runnable, "minefleet-network-stream");
                thread.setDaemon(true);
                return thread;
            });

    private final AtomicBoolean running = new AtomicBoolean();
    private volatile State state = State.DISCONNECTED;
    private volatile StreamObserver<Api.ProxyMessage> outbound;
    private long backoffMillis = INITIAL_BACKOFF_MILLIS;

    public NetworkStreamClient(Channel channel,
                               NetworkContext context,
                               Consumer<Api.ControllerMessage> onMessage,
                               Supplier<List<Api.PresenceEntry>> presenceSupplier) {
        this.channel = channel;
        this.context = context;
        this.onMessage = onMessage;
        this.presenceSupplier = presenceSupplier;
    }

    public State state() {
        return state;
    }

    /** Whether the controller can currently answer routing requests. */
    public boolean isReady() {
        return state == State.READY;
    }

    public void start() {
        if (running.compareAndSet(false, true)) {
            connect();
        }
    }

    public void stop() {
        running.set(false);
        state = State.DISCONNECTED;
        StreamObserver<Api.ProxyMessage> current = outbound;
        if (current != null) {
            try {
                current.onCompleted();
            } catch (RuntimeException ignored) {
                // The stream is already broken; nothing to clean up.
            }
        }
        scheduler.shutdownNow();
    }

    /** Sends a message to the controller, if the stream is up. */
    public boolean send(Api.ProxyMessage message) {
        StreamObserver<Api.ProxyMessage> current = outbound;
        if (current == null || state != State.READY) {
            return false;
        }
        try {
            synchronized (this) {
                current.onNext(message);
            }
            return true;
        } catch (RuntimeException e) {
            LOGGER.log(Level.FINE, "failed to send on network stream", e);
            return false;
        }
    }

    private void connect() {
        if (!running.get()) {
            return;
        }
        state = State.CONNECTING;

        StreamObserver<Api.ControllerMessage> inbound = new StreamObserver<>() {
            @Override
            public void onNext(Api.ControllerMessage message) {
                // The first message proves the stream is usable end to end.
                backoffMillis = INITIAL_BACKOFF_MILLIS;
                try {
                    onMessage.accept(message);
                } catch (RuntimeException e) {
                    LOGGER.log(Level.WARNING, "error handling controller message", e);
                }
            }

            @Override
            public void onError(Throwable error) {
                LOGGER.log(Level.WARNING, "network stream failed: " + error.getMessage());
                scheduleReconnect();
            }

            @Override
            public void onCompleted() {
                LOGGER.info("network stream closed by controller");
                scheduleReconnect();
            }
        };

        try {
            outbound = NetworkXDSGrpc.newStub(channel).connect(inbound);
            // Identify first, then replay presence so the controller can rebuild
            // its view of this proxy after either side restarted.
            outbound.onNext(Api.ProxyMessage.newBuilder().setHello(context.toHello()).build());
            outbound.onNext(Api.ProxyMessage.newBuilder()
                    .setPresenceSnapshot(Api.PresenceSnapshot.newBuilder()
                            .addAllPlayers(presenceSupplier.get()))
                    .build());
            state = State.READY;
        } catch (RuntimeException e) {
            LOGGER.log(Level.WARNING, "could not open network stream", e);
            scheduleReconnect();
        }
    }

    private synchronized void scheduleReconnect() {
        if (!running.get()) {
            return;
        }
        state = State.DISCONNECTED;
        outbound = null;

        // Jitter keeps a fleet of proxies from reconnecting in lockstep after a
        // controller restart or leader failover.
        long jitter = ThreadLocalRandom.current().nextLong(backoffMillis / 2 + 1);
        long delay = backoffMillis + jitter;
        backoffMillis = Math.min(backoffMillis * 2, MAX_BACKOFF_MILLIS);

        try {
            scheduler.schedule(this::connect, delay, TimeUnit.MILLISECONDS);
        } catch (RuntimeException ignored) {
            // The scheduler is shutting down.
        }
    }
}
