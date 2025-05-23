package metrics

import (
	"net/http"

	"github.com/go-chi/chi/v5"
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promauto"
	"github.com/prometheus/client_golang/prometheus/promhttp"
	"github.com/rs/zerolog/log"
)

var (
	// FeedsProcessed counts the number of feeds processed.
	FeedsProcessed = promauto.NewCounterVec(
		prometheus.CounterOpts{
			Name: "rssbot_feeds_processed_total",
			Help: "Total number of RSS feeds processed.",
		},
		[]string{"feed_url", "status"}, // status: success, error, no_new_items
	)

	// NewItemsSent counts new items sent to Telegram.
	NewItemsSent = promauto.NewCounterVec(
		prometheus.CounterOpts{
			Name: "rssbot_new_items_sent_total",
			Help: "Total number of new RSS items sent to Telegram.",
		},
		[]string{"feed_url"},
	)
	
	// HTTPCacheEvents counts cache hits and misses for RSS fetching.
	HTTPCacheEvents = promauto.NewCounterVec(
		prometheus.CounterOpts{
			Name: "rssbot_http_cache_events_total",
			Help: "Total number of HTTP cache events (hit, miss, not_modified).",
		},
		[]string{"feed_url", "event_type"}, // hit, miss, not_modified (304)
	)

	// TelegramAPICalls counts calls to Telegram API.
	TelegramAPICalls = promauto.NewCounterVec(
		prometheus.CounterOpts{
			Name: "rssbot_telegram_api_calls_total",
			Help: "Total number of Telegram API calls.",
		},
		[]string{"method", "status"}, // method: sendMessage, sendPhoto; status: success, error, rate_limited
	)
    
    // ActiveGoroutines reports the number of active goroutines processing feeds.
    // This could be a Gauge.
    ActiveFeedWorkers = promauto.NewGauge(
        prometheus.GaugeOpts{
            Name: "rssbot_active_feed_workers",
            Help: "Number of currently active feed processing goroutines.",
        },
    )
)

// StartServer starts the Prometheus metrics HTTP server.
func StartServer(addr string) {
	if addr == "" {
		log.Info().Msg("Metrics server address not configured, Prometheus endpoint will not be available.")
		return
	}

	mux := chi.NewRouter()
	mux.Handle("/metrics", promhttp.Handler())

	log.Info().Str("address", addr).Msg("Starting Prometheus metrics server")
	go func() {
		if err := http.ListenAndServe(addr, mux); err != nil && err != http.ErrServerClosed {
			log.Error().Err(err).Msg("Prometheus metrics server failed")
		}
	}()
}