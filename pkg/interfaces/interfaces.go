package interfaces

import (
	"context"
	"net/http" // Needed for HTTPClientFactory

	// External dependencies needed by type definitions in this file
	"github.com/mmcdole/gofeed"

	// Your internal packages IF their types are directly used in interface signatures
	// AND these packages DO NOT themselves import "github.com/haytac/rss-telegram-bot/pkg/interfaces"
	"github.com/haytac/rss-telegram-bot/internal/database" // To use database.Proxy, database.Feed, etc.
)

// FetchResult holds the outcome of a feed fetch operation.
type FetchResult struct {
	Feed            *gofeed.Feed
	NewEtag         *string
	NewLastModified *string
}

// FormattedMessagePart represents a piece of a message to be sent.
type FormattedMessagePart struct {
	Text            string
	ParseMode       string
	PhotoURL        string
	DocumentURL     string
	DocumentCaption string
	DocumentName    string
}

// FeedFetcher fetches RSS feed items.
type FeedFetcher interface {
	// Uses database.Proxy from the import above
	Fetch(ctx context.Context, url string, etag, lastModified *string, proxy *database.Proxy) (*FetchResult, error)
}

// Formatter formats a feed item for notification.
type Formatter interface {
	// Uses database.Feed and database.FormattingProfile from the import above
	// Uses FormattedMessagePart defined in this package
	FormatItem(ctx context.Context, item *gofeed.Item, feed *database.Feed, profile *database.FormattingProfile) ([]FormattedMessagePart, error)
}

// Notifier sends notifications.
type Notifier interface {
	// Uses FormattedMessagePart defined in this package
	// Uses database.Proxy from the import above
	Send(ctx context.Context, botToken, chatID string, parts []FormattedMessagePart, proxy *database.Proxy) error
	Name() string
}

// Scheduler manages timed tasks for fetching feeds.
type Scheduler interface {
	// Uses database.Feed from the import above
	Add(feed *database.Feed, task func(f *database.Feed)) error
	Start(ctx context.Context)
	Stop()
}

// ProxyValidator checks if a proxy is working.
type ProxyValidator interface {
    // Uses database.Proxy from the import above
    Validate(ctx context.Context, proxy *database.Proxy, targetURL string) error
}

// HTTPClientFactory creates HTTP clients.
type HTTPClientFactory interface {
    GetClient(proxy *database.Proxy) (*http.Client, error) // Uses http.Client
}