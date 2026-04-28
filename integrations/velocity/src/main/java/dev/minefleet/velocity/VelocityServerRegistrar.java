package dev.minefleet.velocity;

import com.velocitypowered.api.proxy.ProxyServer;
import com.velocitypowered.api.proxy.server.ServerInfo;
import dev.minefleet.api.gateway.networking.ManagedServer;
import dev.minefleet.api.gateway.networking.ServerRegistrar;
import dev.minefleet.api.gateway.networking.ServerRegistrarException;

import java.net.InetSocketAddress;
import java.util.Optional;
import java.util.concurrent.ConcurrentHashMap;

class VelocityServerRegistrar implements ServerRegistrar {

    private final ProxyServer proxy;
    private final ConcurrentHashMap<String, ManagedServer> servers = new ConcurrentHashMap<>();

    VelocityServerRegistrar(ProxyServer proxy) {
        this.proxy = proxy;
    }

    @Override
    public void registerOrUpdate(ManagedServer server) throws ServerRegistrarException {
        try {
            if(!servers.containsKey(server.name())) {
                var address = InetSocketAddress.createUnresolved(server.ipAddress(), server.port());
                proxy.registerServer(new ServerInfo(server.name(), address));
            }
            servers.put(server.name(), server);
        } catch (Exception e) {
            throw new ServerRegistrarException(server, e);
        }
    }

    @Override
    public void unregister(ManagedServer server) throws ServerRegistrarException {
        try {
            proxy.getServer(server.name()).ifPresent(s -> proxy.unregisterServer(s.getServerInfo()));
            servers.remove(server.name());
        } catch (Exception e) {
            throw new ServerRegistrarException(server, e);
        }
    }

    @Override
    public Optional<ManagedServer> findByName(String name) {
        return Optional.ofNullable(servers.get(name));
    }
}