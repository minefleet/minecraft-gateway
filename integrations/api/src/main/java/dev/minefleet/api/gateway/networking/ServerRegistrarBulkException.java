package dev.minefleet.api.gateway.networking;

import java.util.List;

public class ServerRegistrarBulkException extends RuntimeException {
    private final List<ServerRegistrarException> failures;

    public ServerRegistrarBulkException(List<ServerRegistrarException> failures) {
        super(failures.size() + " server(s) could not be managed");
        failures.forEach(this::addSuppressed);
        this.failures = List.copyOf(failures);
    }

    public List<ServerRegistrarException> failures() {
        return failures;
    }
}
