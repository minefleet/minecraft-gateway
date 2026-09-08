package dev.minefleet.velocity;

import com.google.inject.Inject;
import com.velocitypowered.api.event.EventTask;
import com.velocitypowered.api.event.Subscribe;
import com.velocitypowered.api.event.connection.DisconnectEvent;
import com.velocitypowered.api.event.player.KickedFromServerEvent;
import com.velocitypowered.api.event.player.PlayerChooseInitialServerEvent;
import com.velocitypowered.api.event.player.ServerPostConnectEvent;
import com.velocitypowered.api.event.proxy.ProxyInitializeEvent;
import com.velocitypowered.api.event.proxy.ProxyShutdownEvent;
import com.velocitypowered.api.plugin.Plugin;
import com.velocitypowered.api.proxy.Player;
import com.velocitypowered.api.proxy.ProxyServer;
import dev.minefleet.api.BuildInfo;
import dev.minefleet.api.gateway.networking.NetworkGateway;
import dev.minefleet.api.gateway.networking.v1alpha1.Api;
import org.slf4j.Logger;

import java.util.ArrayList;
import java.util.List;
import java.util.Optional;

@Plugin(
        id = "minefleet-gateway",
        name = "Minefleet Gateway",
        version = BuildInfo.VERSION,
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
                .presenceSupplier(this::currentPresence)
                .build();
        gateway.setPlayerProvider(Player.class, p -> VelocityNetworkPlayer.of(p, proxy, registrar));
        gateway.setPlayerLookup(uuid -> proxy.getPlayer(uuid)
                .map(p -> VelocityNetworkPlayer.of(p, proxy, registrar)));
        gateway.start();
        NetworkGateway.setInstance(gateway);
        proxy.getAllServers().forEach(server -> proxy.unregisterServer(server.getServerInfo()));
        logger.info("Minefleet gateway started.");
    }

    @Subscribe
    public void onProxyShutdown(ProxyShutdownEvent ignoredEvent) {
        if (gateway != null) {
            gateway.stop();
        }
    }

    /**
     * The controller keeps presence in memory only, so every reconnect replays
     * the players this proxy currently holds.
     */
    private List<Api.PresenceEntry> currentPresence() {
        List<Api.PresenceEntry> entries = new ArrayList<>();
        for (Player player : proxy.getAllPlayers()) {
            player.getCurrentServer().ifPresent(connection -> entries.add(gateway.presenceEntry(
                    VelocityNetworkPlayer.of(player, proxy, registrar),
                    connection.getServerInfo().getName())));
        }
        return entries;
    }

    // Routing now round-trips to the controller, so the join is completed
    // asynchronously rather than blocking Velocity's event thread.
    @Subscribe
    public EventTask onPlayerChooseInitialServer(PlayerChooseInitialServerEvent event) {
        return EventTask.resumeWhenComplete(gateway.routeJoin(
                VelocityNetworkPlayer.forInitialEvent(event.getPlayer(), proxy, registrar, event)));
    }

    @Subscribe
    public EventTask onKickedFromServer(KickedFromServerEvent event) {
        return EventTask.resumeWhenComplete(gateway.routeFallback(
                VelocityNetworkPlayer.forKickedEvent(event.getPlayer(), proxy, registrar, event)));
    }

    @Subscribe
    public void onServerPostConnect(ServerPostConnectEvent event) {
        Optional<String> serverName = event.getPlayer().getCurrentServer()
                .map(connection -> connection.getServerInfo().getName());
        serverName.ifPresent(name -> gateway.reportConnected(
                VelocityNetworkPlayer.of(event.getPlayer(), proxy, registrar), name));
    }

    @Subscribe
    public void onDisconnect(DisconnectEvent event) {
        gateway.reportDisconnected(event.getPlayer().getUniqueId());
    }
}
