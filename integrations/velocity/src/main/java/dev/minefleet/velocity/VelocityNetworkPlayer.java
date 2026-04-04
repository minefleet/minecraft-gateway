package dev.minefleet.velocity;

import com.velocitypowered.api.event.player.KickedFromServerEvent;
import com.velocitypowered.api.event.player.PlayerChooseInitialServerEvent;
import com.velocitypowered.api.proxy.Player;
import com.velocitypowered.api.proxy.ProxyServer;
import com.velocitypowered.api.proxy.server.RegisteredServer;
import dev.minefleet.api.gateway.networking.ManagedServer;
import dev.minefleet.api.gateway.networking.player.KickReason;
import dev.minefleet.api.gateway.networking.player.NetworkPlayer;
import net.kyori.adventure.text.Component;
import net.kyori.adventure.text.format.TextColor;

import java.net.InetSocketAddress;
import java.util.Optional;
import java.util.function.Consumer;

class VelocityNetworkPlayer implements NetworkPlayer<Player> {

    private final Player player;
    private final ProxyServer proxy;
    private final VelocityServerRegistrar registrar;
    private final Consumer<RegisteredServer> connectHandler;
    private final Consumer<Component> kickHandler;

    private VelocityNetworkPlayer(Player player, ProxyServer proxy, VelocityServerRegistrar registrar,
                                  Consumer<RegisteredServer> connectHandler, Consumer<Component> kickHandler) {
        this.player = player;
        this.proxy = proxy;
        this.registrar = registrar;
        this.connectHandler = connectHandler;
        this.kickHandler = kickHandler;
    }

    static VelocityNetworkPlayer forInitial(Player player, ProxyServer proxy, VelocityServerRegistrar registrar,
                                            PlayerChooseInitialServerEvent event) {
        return new VelocityNetworkPlayer(player, proxy, registrar,
                event::setInitialServer,
                player::disconnect);
    }

    static VelocityNetworkPlayer forKicked(Player player, ProxyServer proxy, VelocityServerRegistrar registrar,
                                           KickedFromServerEvent event) {
        return new VelocityNetworkPlayer(player, proxy, registrar,
                server -> event.setResult(KickedFromServerEvent.RedirectPlayer.create(server)),
                message -> event.setResult(KickedFromServerEvent.DisconnectPlayer.create(message)));
    }

    @Override
    public String connectedDomain() {
        return player.getVirtualHost()
                .map(InetSocketAddress::getHostString)
                .orElse("");
    }

    @Override
    public Optional<ManagedServer> connectedServer() {
        return player.getCurrentServer()
                .flatMap(conn -> registrar.findByName(conn.getServerInfo().getName()));
    }

    @Override
    public boolean hasPermission(String permission) {
        return player.hasPermission(permission);
    }

    @Override
    public void connectToServer(ManagedServer server) {
        proxy.getServer(server.name()).ifPresent(connectHandler);
    }

    @Override
    public void kick(KickReason reason) {
        Component message = switch (reason) {
            case NO_JOIN ->
                    Component.text("No server is currently available to join.").color(TextColor.color(0xc92222));
            case NO_FALLBACK ->
                    Component.text("No fallback server is currently available.").color(TextColor.color(0xc92222));
        };
        kickHandler.accept(message);
    }
}