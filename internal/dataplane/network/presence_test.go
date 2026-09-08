package network

import "testing"

func presenceOn(uuid, proxy, server string) Presence {
	return Presence{PlayerUUID: uuid, ProxyID: proxy, ServerName: server}
}

func TestPresenceTracksPlayerMoves(t *testing.T) {
	p := NewPresenceMap()
	p.Set(presenceOn("alice", "proxy-a", "lobby-0"))

	if got, ok := p.Lookup("alice"); !ok || got.ServerName != "lobby-0" {
		t.Fatalf("Lookup = %+v, %v; want alice on lobby-0", got, ok)
	}
	if got := p.CountForServer("lobby-0"); got != 1 {
		t.Errorf("CountForServer(lobby-0) = %d, want 1", got)
	}

	// Moving a player must not leave them counted on the old server.
	p.Set(presenceOn("alice", "proxy-a", "game-0"))
	if got := p.CountForServer("lobby-0"); got != 0 {
		t.Errorf("CountForServer(lobby-0) after move = %d, want 0", got)
	}
	if got := p.CountForServer("game-0"); got != 1 {
		t.Errorf("CountForServer(game-0) after move = %d, want 1", got)
	}
}

func TestPresenceRemove(t *testing.T) {
	p := NewPresenceMap()
	p.Set(presenceOn("alice", "proxy-a", "lobby-0"))
	p.Remove("alice")

	if _, ok := p.Lookup("alice"); ok {
		t.Error("alice should be gone after Remove")
	}
	if got := p.CountForServer("lobby-0"); got != 0 {
		t.Errorf("CountForServer = %d, want 0", got)
	}
}

func TestReplaceForProxyReplacesOnlyThatProxy(t *testing.T) {
	p := NewPresenceMap()
	p.Set(presenceOn("alice", "proxy-a", "lobby-0"))
	p.Set(presenceOn("bob", "proxy-b", "lobby-0"))

	// proxy-a reconnects reporting a different set of players.
	p.ReplaceForProxy("proxy-a", []Presence{presenceOn("carol", "proxy-a", "game-0")})

	if _, ok := p.Lookup("alice"); ok {
		t.Error("alice was not in proxy-a's replay and should have been dropped")
	}
	if _, ok := p.Lookup("bob"); !ok {
		t.Error("bob is behind proxy-b and should be untouched")
	}
	if _, ok := p.Lookup("carol"); !ok {
		t.Error("carol was in the replay and should be present")
	}
}

func TestDropProxyForgetsOnlyItsPlayers(t *testing.T) {
	p := NewPresenceMap()
	p.Set(presenceOn("alice", "proxy-a", "lobby-0"))
	p.Set(presenceOn("bob", "proxy-b", "lobby-0"))

	p.DropProxy("proxy-a")

	if _, ok := p.Lookup("alice"); ok {
		t.Error("alice should be gone with proxy-a")
	}
	if _, ok := p.Lookup("bob"); !ok {
		t.Error("bob should survive proxy-a going away")
	}
	if got := p.CountForServer("lobby-0"); got != 1 {
		t.Errorf("CountForServer = %d, want 1", got)
	}
}

func TestOnConnectFires(t *testing.T) {
	p := NewPresenceMap()
	var seen []string
	p.OnConnect(func(pr Presence) { seen = append(seen, pr.PlayerUUID) })

	p.Set(presenceOn("alice", "proxy-a", "lobby-0"))
	p.ReplaceForProxy("proxy-b", []Presence{presenceOn("bob", "proxy-b", "lobby-0")})

	if len(seen) != 2 || seen[0] != "alice" || seen[1] != "bob" {
		t.Errorf("subscribers saw %v, want [alice bob]", seen)
	}
}

func TestPlayersForServer(t *testing.T) {
	p := NewPresenceMap()
	p.Set(presenceOn("alice", "proxy-a", "lobby-0"))
	p.Set(presenceOn("bob", "proxy-b", "lobby-0"))
	p.Set(presenceOn("carol", "proxy-a", "game-0"))

	if got := len(p.PlayersForServer("lobby-0")); got != 2 {
		t.Errorf("PlayersForServer(lobby-0) returned %d players, want 2", got)
	}
	if got := p.Len(); got != 3 {
		t.Errorf("Len = %d, want 3", got)
	}
}
