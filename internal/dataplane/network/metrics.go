package network

import (
	"sync"

	"github.com/prometheus/client_golang/prometheus"
	"sigs.k8s.io/controller-runtime/pkg/metrics"
)

// listenerLabels identifies one gateway listener in the player count metrics.
type listenerLabels struct {
	gateway  string
	listener string
}

var (
	playersOnline = prometheus.NewGaugeVec(prometheus.GaugeOpts{
		Name: "minefleet_gateway_players_online",
		Help: "Players connected to the network proxies of a gateway listener, summed across proxy replicas.",
	}, []string{"gateway", "listener"})

	playersMax = prometheus.NewGaugeVec(prometheus.GaugeOpts{
		Name: "minefleet_gateway_players_max",
		Help: "Player capacity of the network proxies of a gateway listener, summed across proxy replicas.",
	}, []string{"gateway", "listener"})

	playerMoves = prometheus.NewCounterVec(prometheus.CounterOpts{
		Name: "minefleet_gateway_player_moves_total",
		Help: "Player moves dispatched by the controller, by what asked for them and how they ended.",
	}, []string{"source", "result", "reason"})
)

// Where a move came from, so the declarative and imperative paths are
// distinguishable on one dashboard.
const (
	moveSourceAPI            = "api"
	moveSourcePlayerTransfer = "playertransfer"
)

// recordPlayerMove counts one player's terminal move outcome.
func recordPlayerMove(source string, result PlayerMoveResult) {
	if result.Success {
		playerMoves.WithLabelValues(source, "ok", "").Inc()
		return
	}
	playerMoves.WithLabelValues(source, "failed", result.Reason).Inc()
}

// RecordPlayerTransferMove counts a move made on behalf of a PlayerTransfer, so
// both paths land in the same counter.
func RecordPlayerTransferMove(success bool, reason string) {
	recordPlayerMove(moveSourcePlayerTransfer, PlayerMoveResult{Success: success, Reason: reason})
}

// publishedListeners remembers which label sets are currently exported, so a
// listener that goes away is deleted rather than left at its last value.
var (
	publishedMu       sync.Mutex
	publishedListener = make(map[listenerLabels]struct{})
)

func init() {
	metrics.Registry.MustRegister(playersOnline, playersMax, playerMoves)
}

// setPlayerCountMetrics replaces the exported gauges with the given totals.
func setPlayerCountMetrics(totals map[listenerLabels]PlayerCounts) {
	publishedMu.Lock()
	defer publishedMu.Unlock()

	for labels := range publishedListener {
		if _, ok := totals[labels]; ok {
			continue
		}
		playersOnline.DeleteLabelValues(labels.gateway, labels.listener)
		playersMax.DeleteLabelValues(labels.gateway, labels.listener)
		delete(publishedListener, labels)
	}

	for labels, counts := range totals {
		playersOnline.WithLabelValues(labels.gateway, labels.listener).Set(float64(counts.OnlinePlayers))
		playersMax.WithLabelValues(labels.gateway, labels.listener).Set(float64(counts.MaxPlayers))
		publishedListener[labels] = struct{}{}
	}
}
