package dev.minefleet.api.gateway.networking;

import dev.minefleet.api.gateway.networking.v1alpha1.Api;

import java.util.Map;

/**
 * Where a {@link NetworkGatewayClient#movePlayers} call sends its players.
 *
 * <p>The three modes mirror a PlayerTransfer's target exactly:
 * {@link #server} names a registered server, {@link #matching} picks one server
 * from the services carrying the given labels, and {@link #route} evaluates the
 * gateway's own routes per player so they may land on different servers.
 */
public sealed interface MoveTarget {

    /** Sends every player to one named server. */
    static MoveTarget server(String serverName) {
        return new Server(serverName);
    }

    /**
     * Picks a single server from the services matching these labels, using their
     * distribution strategy. Every player of the call goes to that one server.
     */
    static MoveTarget matching(Map<String, String> labels) {
        return new Matching(Map.copyOf(labels));
    }

    /**
     * Evaluates the gateway's join or fallback routes for each player, so players
     * may land on different servers.
     */
    static MoveTarget route(RouteKind kind) {
        return new Route(kind);
    }

    /** Which set of routes {@link #route} evaluates. */
    enum RouteKind {JOIN, FALLBACK}

    /** Applies this target to a request being built. */
    void applyTo(Api.MovePlayersRequest.Builder request);

    record Server(String serverName) implements MoveTarget {
        @Override
        public void applyTo(Api.MovePlayersRequest.Builder request) {
            request.setServerName(serverName);
        }
    }

    record Matching(Map<String, String> labels) implements MoveTarget {
        @Override
        public void applyTo(Api.MovePlayersRequest.Builder request) {
            request.setLabelSelector(Api.LabelSelector.newBuilder().putAllMatchLabels(labels));
        }
    }

    record Route(RouteKind kind) implements MoveTarget {
        @Override
        public void applyTo(Api.MovePlayersRequest.Builder request) {
            request.setRouteMode(kind == RouteKind.FALLBACK
                    ? Api.RouteKind.ROUTE_KIND_FALLBACK
                    : Api.RouteKind.ROUTE_KIND_JOIN);
        }
    }
}
