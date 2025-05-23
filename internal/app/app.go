package app

import (
	"context"
	"fmt"
	"os"
	"os/signal"
	"syscall"

	"github.com/rs/zerolog/log"
	"github.com/haytac/rss-telegram-bot/internal/config"       // Module path
	"github.com/haytac/rss-telegram-bot/internal/database"    // Module path
	"github.com/haytac/rss-telegram-bot/internal/formatter"   // Module path
	"github.com/haytac/rss-telegram-bot/internal/metrics"     // Module path
	"github.com/haytac/rss-telegram-bot/internal/proxy"       // Module path
	"github.com/haytac/rss-telegram-bot/internal/rss"         // Module path
	"github.com/haytac/rss-telegram-bot/internal/scheduler"   // Module path
	"github.com/haytac/rss-telegram-bot/internal/telegram"    // Module path
	"github.com/haytac/rss-telegram-bot/pkg/interfaces" // Module path
)

// Application holds all dependencies for the app.
type Application struct {
	Config     *config.AppConfig
	DB         *database.DB
	Scheduler  interfaces.Scheduler
	FeedWorker *FeedWorker
	
	// Stores
	FeedStore            *database.FeedStore
	ProxyStore           *database.ProxyStore
	TelegramBotStore     *database.TelegramBotStore
	FormattingProfStore  *database.FormattingProfileStore
}

// NewApplication creates and initializes a new application instance.
func NewApplication(cfg *config.AppConfig) (*Application, error) {
	// Initialize encryption key (DEMO ONLY)
	if cfg.EncryptionKey == "" {
		log.Warn().Msg("Configuration 'encryption_key' (or RSS_BOT_ENCRYPTION_KEY env var) is not set. Token storage will be INSECURE (DEMO MODE).")
	}
	// This error can be ignored for demo, but logged. In prod, might be fatal.
	if errKey := database.InitEncryptionKey(cfg.EncryptionKey); errKey != nil {
	    log.Warn().Err(errKey).Msg("Encryption key initialization issue. Tokens may not be handled securely.")
    }


	db, err := database.Connect(cfg.DatabasePath, "internal/database/migrations")
	if err != nil {
		return nil, fmt.Errorf("failed to connect to database: %w", err)
	}

	// Stores
	feedStore := database.NewFeedStore(db)
	proxyStore := database.NewProxyStore(db)
	tgBotStore := database.NewTelegramBotStore(db) // Add encryption key here if implementing
	fmtProfStore := database.NewFormattingProfileStore(db)

	httpClientFactory := proxy.NewHTTPClientFactory() // Pass proxyStore if factory needs it

	rssFetcher := rss.NewGoFeedFetcher(httpClientFactory)
	msgFormatter := formatter.NewDefaultFormatter()
	// Pass client factory for proxy support to Telegram client
	tgNotifier := telegram.NewClient(httpClientFactory) 
	
	appScheduler := scheduler.NewFeedScheduler()

	// Pass necessary stores to FeedWorker for it to retrieve fresh data
	worker := NewFeedWorker(db, feedStore, proxyStore, tgBotStore, fmtProfStore, rssFetcher, msgFormatter, tgNotifier, cfg)

	return &Application{
		Config:     cfg,
		DB:         db,
		Scheduler:  appScheduler,
		FeedWorker: worker,
		FeedStore:  feedStore,
		ProxyStore: proxyStore,
		TelegramBotStore: tgBotStore,
		FormattingProfStore: fmtProfStore,
	}, nil
}
// Run starts the application's main loop (scheduler, metrics server).
func (app *Application) Run(ctx context.Context) error {
	log.Info().Msg("Starting application...")

	// Start Prometheus metrics server
	metrics.StartServer(app.Config.MetricsPort)

	// Load feeds from DB and add to scheduler
	feeds, err := app.FeedStore.GetEnabledFeeds(ctx)
	if err != nil {
		log.Error().Err(err).Msg("Failed to load feeds from database")
		return fmt.Errorf("loading feeds: %w", err)
	}

	if len(feeds) == 0 {
		log.Info().Msg("No enabled feeds found in the database. Add feeds via CLI.")
	} else {
		for _, feed := range feeds {
			// Capture feed in closure for the task function
			f := feed 
			// TODO: Ensure feed.Proxy, feed.FormattingProfile, feed.BotToken are loaded
			// by GetEnabledFeeds or lazy-loaded in the worker.
			// This is crucial. The worker needs these details.
			// A better GetEnabledFeeds would join and populate these.
			if err := app.Scheduler.Add(f, app.FeedWorker.ProcessFeed); err != nil {
				log.Error().Err(err).Int64("feed_id", f.ID).Msg("Failed to add feed to scheduler")
			}
		}
	}
	
	app.Scheduler.Start(ctx)

	// Graceful shutdown handling
	sigCh := make(chan os.Signal, 1)
	signal.Notify(sigCh, syscall.SIGINT, syscall.SIGTERM)

	select {
	case s := <-sigCh:
		log.Info().Str("signal", s.String()).Msg("Received shutdown signal")
	case <-ctx.Done(): // If parent context is cancelled
		log.Info().Msg("Application context done, shutting down")
	}

	// Perform cleanup
	log.Info().Msg("Shutting down scheduler...")
	app.Scheduler.Stop() // This should be blocking or use a waitgroup

	// TODO: Wait for scheduler to fully stop if it has ongoing tasks.
	// For simplicity, assuming Stop is relatively quick or non-critical tasks can be interrupted.

	log.Info().Msg("Closing database connection...")
	if err := app.DB.Close(); err != nil {
		log.Error().Err(err).Msg("Error closing database")
	}

	log.Info().Msg("Application shut down gracefully.")
	return nil
}