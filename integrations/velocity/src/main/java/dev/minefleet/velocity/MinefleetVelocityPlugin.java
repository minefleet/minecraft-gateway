package dev.minefleet.velocity;

import com.google.inject.Inject;
import com.velocitypowered.api.event.Subscribe;
import com.velocitypowered.api.event.player.KickedFromServerEvent;
import com.velocitypowered.api.event.player.PlayerChooseInitialServerEvent;
import com.velocitypowered.api.event.proxy.ProxyInitializeEvent;
import com.velocitypowered.api.event.proxy.ProxyShutdownEvent;
import com.velocitypowered.api.plugin.Plugin;
import com.velocitypowered.api.plugin.Dependency;
import com.velocitypowered.api.proxy.ProxyServer;
import dev.minefleet.api.gateway.networking.NetworkGateway;
import org.slf4j.Logger;

@Plugin(
        id = "minefleet-gateway",
        name = "Minefleet Gateway",
        version = "0.0.1-SNAPSHOT",
        description = "Minefleet gateway integration for Velocity",
        authors = {"The minefleet authors"}
)
public class MinefleetVelocityPlugin {

    private final ProxyServer proxy;
    private final Logger logger;
    private VelocityServerRegistrar registrar;
    private NetworkGateway gateway;

    @Inject
    public MinefleetVelocityPlugin(ProxyServer proxy, Logger logger) {
        this.proxy = proxy;
        this.logger = logger;
    }

    @Subscribe
    public void onProxyInitialize(ProxyInitializeEvent ignoredEvent) {
        registrar = new VelocityServerRegistrar(proxy);
        gateway = NetworkGateway.builder()
                .registrar(registrar)
                .build();
        gateway.start();
        proxy.getAllServers().forEach(server -> {
            proxy.unregisterServer(server.getServerInfo());
        });
        logger.info("Minefleet gateway started.");
    }

    @Subscribe
    public void onProxyShutdown(ProxyShutdownEvent ignoredEvent) {
        if (gateway != null) {
            gateway.stop();
        }
    }

    @Subscribe
    public void onPlayerChooseInitialServer(PlayerChooseInitialServerEvent event) {
        gateway.routeJoin(VelocityNetworkPlayer.forInitial(event.getPlayer(), proxy, registrar, event));
    }

    @Subscribe
    public void onKickedFromServer(KickedFromServerEvent event) {
        gateway.routeFallback(VelocityNetworkPlayer.forKicked(event.getPlayer(), proxy, registrar, event));
    }
}