package dev.minefleet.api.gateway.networking;

public class ServerRegistrarException extends RuntimeException {
    private final ManagedServer server;
    public ServerRegistrarException(ManagedServer server, Throwable cause) {
        super(String.format("Server %s could not be managed at this time", server.name()), cause);
        this.server = server;
    }

    public ManagedServer server() {
        return server;
    }
}
