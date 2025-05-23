package app

import (
	"context"
	"crypto/sha256"
	"fmt"
	"time"
	"github.com/rs/zerolog/log"
	"github.com/haytac/rss-telegram-bot/internal/config"       // Module path
	"github.com/haytac/rss-telegram-bot/internal/database"    // Module path
	"github.com/haytac/rss-telegram-bot/internal/metrics"     // Module path
	"github.com/haytac/rss-telegram-bot/internal/rss"         // Module path
	"github.com/haytac/rss-telegram-bot/pkg/interfaces" // Module path
    "github.com/haytac/rss-telegram-bot/internal/telegram" // No alias, so use telegram.Client
)

// FeedWorker handles fetching and processing a single feed.
type FeedWorker struct {
	db                   *database.DB // For transactions or direct access if needed
	feedStore            *database.FeedStore
	proxyStore           *database.ProxyStore
	botStore             *database.TelegramBotStore
	formattingProfStore  *database.FormattingProfileStore
	fetcher              interfaces.FeedFetcher
	formatter            interfaces.Formatter
	notifier             interfaces.Notifier // This is now the telegram.Client
	appConfig            *config.AppConfig
}

// NewFeedWorker creates a new FeedWorker.
func NewFeedWorker(
	db *database.DB,
	fs *database.FeedStore,
	ps *database.ProxyStore,
	bs *database.TelegramBotStore,
	fps *database.FormattingProfileStore,
	fetcher interfaces.FeedFetcher,
	formatter interfaces.Formatter,
	notifier interfaces.Notifier, // Changed from telegram.Client to interfaces.Notifier
	appCfg *config.AppConfig,
) *FeedWorker {
	return &FeedWorker{
		db:                  db,
		feedStore:           fs,
		proxyStore:          ps,
		botStore:            bs,
		formattingProfStore: fps,
		fetcher:             fetcher,
		formatter:           formatter,
		notifier:            notifier,
		appConfig:           appCfg,
	}
}

// ProcessFeed fetches, formats, and sends updates for a given feed.
func (w *FeedWorker) ProcessFeed(feedFromScheduler *database.Feed) {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Minute)
	defer cancel()

	metrics.ActiveFeedWorkers.Inc()
	defer metrics.ActiveFeedWorkers.Dec()

	l := log.With().Int64("feed_id", feedFromScheduler.ID).Str("feed_url", feedFromScheduler.URL).Logger()
	l.Info().Msg("Starting to process feed")

	// Reload feed details to get the absolute latest config, including joined Proxy and FormattingProfile.
	// The feedFromScheduler might be slightly stale if config changed via CLI since it was scheduled.
	currentFeed, err := w.feedStore.GetFeedByID(ctx, feedFromScheduler.ID)
	if err != nil {
		l.Error().Err(err).Msg("Failed to reload feed details from DB")
		metrics.FeedsProcessed.WithLabelValues(feedFromScheduler.URL, "db_error").Inc()
		return
	}
	if currentFeed == nil || !currentFeed.IsEnabled {
		l.Info().Msg("Feed no longer exists or is disabled, skipping.")
		return
	}
	
	// currentFeed.Proxy and currentFeed.FormattingProfile are now populated by GetFeedByID if they exist.
	// If currentFeed.Proxy is nil, the fetcher/notifier should use default (no proxy or global default proxy).
	// The client factory in fetcher/notifier handles nil proxy.

	// 1. Fetch RSS feed
	// The proxy for RSS fetch can be specific to the feed, or a global default.
	// currentFeed.Proxy already holds the specific proxy if configured.
	// If not, fetcher's clientFactory should handle nil to use no proxy or its own default.
	
		// Determine proxy for RSS fetch
		rssProxy := currentFeed.Proxy
		if rssProxy == nil && !w.appConfig.DryRun { // Don't fetch default proxy in dry run if not needed for logic
			defaultRSSProxy, errP := w.proxyStore.GetDefaultProxy(ctx, "rss")
			if errP != nil {
				l.Warn().Err(errP).Msg("Failed to get default RSS proxy")
			} else if defaultRSSProxy != nil {
				l.Debug().Str("proxy_name", defaultRSSProxy.Name).Msg("Using default RSS proxy")
				rssProxy = defaultRSSProxy
			}
		}
	
		fetchResult, err := w.fetcher.Fetch(ctx, currentFeed.URL, currentFeed.HTTPEtag, currentFeed.HTTPLastModified, rssProxy)
		if err != nil {
		l.Error().Err(err).Msg("Failed to fetch RSS feed")
		metrics.FeedsProcessed.WithLabelValues(currentFeed.URL, "fetch_error").Inc()
		return
	}

	// ... (rest of the fetchResult handling, 304, etc. remains similar) ...
	if fetchResult.Feed == nil { 
		l.Info().Msg("Feed content not modified")
		metrics.HTTPCacheEvents.WithLabelValues(currentFeed.URL, "not_modified").Inc()
		if err := w.feedStore.UpdateFeedLastProcessed(ctx, currentFeed.ID, currentFeed.LastProcessedItemGUIDHash, currentFeed.HTTPEtag, currentFeed.HTTPLastModified); err != nil {
			l.Error().Err(err).Msg("Failed to update feed last fetched time after 304")
		}
		metrics.FeedsProcessed.WithLabelValues(currentFeed.URL, "not_modified").Inc()
		return
	}
	metrics.HTTPCacheEvents.WithLabelValues(currentFeed.URL, "fetched").Inc()


	isItemProcessed := func(itemGUIDHash string) (bool, error) {
		return w.feedStore.IsItemProcessed(ctx, currentFeed.ID, itemGUIDHash)
	}
	newItems, latestItemInFeedHash, err := rss.GetNewItems(fetchResult.Feed, isItemProcessed)
	if err != nil {
		l.Error().Err(err).Msg("Failed to identify new items")
		metrics.FeedsProcessed.WithLabelValues(currentFeed.URL, "filter_error").Inc()
		return
	}

	if len(newItems) == 0 {
		l.Info().Msg("No new items found in feed")
		var hashToStore *string
		if latestItemInFeedHash != "" { hashToStore = &latestItemInFeedHash } else { hashToStore = currentFeed.LastProcessedItemGUIDHash }
		if err := w.feedStore.UpdateFeedLastProcessed(ctx, currentFeed.ID, hashToStore, fetchResult.NewEtag, fetchResult.NewLastModified); err != nil {
			l.Error().Err(err).Msg("Failed to update feed metadata after no new items")
		}
		metrics.FeedsProcessed.WithLabelValues(currentFeed.URL, "no_new_items").Inc()
		return
	}
	l.Info().Int("new_items_count", len(newItems)).Msg("New items found")


	// Get Bot Token (securely, on-demand)
	var botToken string
	if currentFeed.TelegramBotID != nil {
		token, errToken := w.botStore.GetTokenByBotID(ctx, *currentFeed.TelegramBotID)
		if errToken != nil {
			l.Error().Err(errToken).Int64("bot_id", *currentFeed.TelegramBotID).Msg("Failed to retrieve Telegram bot token")
			metrics.FeedsProcessed.WithLabelValues(currentFeed.URL, "token_error").Inc()
			return // Cannot proceed without token
		}
		botToken = token
	} else {
		// This case should ideally be prevented by DB constraints or CLI validation (feed needs a bot).
		// Or there's a global default bot token in appConfig.
		l.Error().Msg("Feed is not associated with a Telegram bot ID, cannot send messages.")
		metrics.FeedsProcessed.WithLabelValues(currentFeed.URL, "config_error").Inc()
		return
	}
    
    // Determine proxy for Telegram: could be feed-specific, global default, or none
    telegramProxy := currentFeed.Proxy // Start with feed-specific proxy
	if telegramProxy == nil && !w.appConfig.DryRun { // No feed-specific proxy, try global Telegram default
		defaultTGProxy, errP := w.proxyStore.GetDefaultProxy(ctx, "telegram")
		if errP != nil {
			l.Warn().Err(errP).Msg("Failed to get default Telegram proxy")
		} else if defaultTGProxy != nil {
			l.Debug().Str("proxy_name", defaultTGProxy.Name).Msg("Using default Telegram proxy")
			telegramProxy = defaultTGProxy
		}
	}


	var lastSuccessfullyProcessedItemHash string
	for _, item := range newItems {
		itemCtx := log.With().Str("item_title", Truncate(item.Title, 50)).Str("item_link", item.Link).Logger().WithContext(ctx)
		
		// currentFeed.FormattingProfile is already populated
		formattedParts, err := w.formatter.FormatItem(itemCtx, item, currentFeed, currentFeed.FormattingProfile)
		if err != nil {
			l.Error().Err(err).Str("item_title", item.Title).Msg("Failed to format item")
			continue
		}

		if w.appConfig.DryRun {
			l.Info().Interface("formatted_parts", formattedParts).Msg("[DRY RUN] Would send formatted item")
		} else {
			// The notifier interface's Send method should ideally take the proxy.
			// Let's assume the telegram.Client's Send method (which implements interfaces.Notifier)
			// is modified to accept a proxy *database.Proxy argument.
			// We need to cast w.notifier to its concrete type or modify interface.
			// For simplicity, let's assume interfaces.Notifier.Send takes proxy.
			// If Notifier is specifically telegram.Client:
			if tgClient, ok := w.notifier.(*telegram.Client); ok {
				err = tgClient.Send(itemCtx, botToken, currentFeed.TelegramChatID, formattedParts, telegramProxy)
			} else {
				// Fallback or error if notifier is not the expected type
				// This indicates a mismatch in DI. For now, assume it's telegram.Client.
				// Or, the Notifier interface needs to be:
				// Send(ctx context.Context, recipient string, message interface{}, proxy *database.Proxy) error
				l.Error().Msg("Notifier is not of expected type *telegram.Client to pass proxy")
				err = fmt.Errorf("notifier type mismatch for proxy handling") 
			}


			if err != nil {
				l.Error().Err(err).Str("item_title", item.Title).Msg("Failed to send item to notifier")
				metrics.TelegramAPICalls.WithLabelValues(w.notifier.Name(), "send_error").Inc()
				return 
			}
			metrics.TelegramAPICalls.WithLabelValues(w.notifier.Name(), "success").Inc()
		}

		itemIdentifier := item.GUID
		if itemIdentifier == "" { itemIdentifier = item.Link }
		currentItemHash := fmt.Sprintf("%x", sha256.Sum256([]byte(itemIdentifier)))
		if err := w.feedStore.AddProcessedItem(itemCtx, currentFeed.ID, currentItemHash); err != nil {
			l.Error().Err(err).Str("item_guid_hash", currentItemHash).Msg("Failed to mark item as processed")
		}
		lastSuccessfullyProcessedItemHash = currentItemHash
		metrics.NewItemsSent.WithLabelValues(currentFeed.URL).Inc()
	}

	var finalHashToStore *string
	if lastSuccessfullyProcessedItemHash != "" {
		finalHashToStore = &lastSuccessfullyProcessedItemHash
	} else if latestItemInFeedHash != "" {
		finalHashToStore = &latestItemInFeedHash
	} else {
		finalHashToStore = currentFeed.LastProcessedItemGUIDHash
	}

	if err := w.feedStore.UpdateFeedLastProcessed(ctx, currentFeed.ID, finalHashToStore, fetchResult.NewEtag, fetchResult.NewLastModified); err != nil {
		l.Error().Err(err).Msg("Failed to update feed metadata after processing items")
	}

	l.Info().Int("new_items_processed", len(newItems)).Msg("Finished processing feed")
	metrics.FeedsProcessed.WithLabelValues(currentFeed.URL, "success").Inc()
}

// ... (Truncate function) ...

// Truncate string to max length
func Truncate(s string, maxLength int) string {
    if len(s) <= maxLength {
        return s
    }
    return s[:maxLength-3] + "..."
}