package dev.minefleet.api.gateway.networking;

import java.util.ArrayList;
import java.util.List;

public interface ServerRegistrar {

    default void registerOrUpdate(ManagedService service) throws ServerRegistrarBulkException {
        List<ServerRegistrarException> failures = new ArrayList<>();
        for (ManagedServer server : service.servers()) {
            try {
                registerOrUpdate(server);
            } catch (ServerRegistrarException e) {
                failures.add(e);
            }
        }
        if (!failures.isEmpty()) throw new ServerRegistrarBulkException(failures);
    }

    void registerOrUpdate(ManagedServer server) throws ServerRegistrarException;

    default void unregister(ManagedService service) throws ServerRegistrarBulkException {
        List<ServerRegistrarException> failures = new ArrayList<>();
        for (ManagedServer server : service.servers()) {
            try {
                unregister(server);
            } catch (ServerRegistrarException e) {
                failures.add(e);
            }
        }
        if (!failures.isEmpty()) throw new ServerRegistrarBulkException(failures);
    }

    void unregister(ManagedServer server) throws ServerRegistrarException;

}
