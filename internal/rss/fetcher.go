package rss

import (
	"context"
	"crypto/sha256"
	"errors"
	"fmt"
	"io"
	"net/http"
	"sort"
	"time"

	"github.com/mmcdole/gofeed"
	"github.com/rs/zerolog/log"
	"github.com/haytac/rss-telegram-bot/internal/database"
	"github.com/haytac/rss-telegram-bot/pkg/interfaces"
)

// Define these constants here
const (
	maxFetchRetries    = 3
	initialRetryDelay  = 2 * time.Second
	maxRetryDelay      = 30 * time.Second
)

// GoFeedFetcher implements FeedFetcher using gofeed.
type GoFeedFetcher struct {
	clientFactory interfaces.HTTPClientFactory
}

// NewGoFeedFetcher creates a new GoFeedFetcher.
func NewGoFeedFetcher(clientFactory interfaces.HTTPClientFactory) *GoFeedFetcher {
	return &GoFeedFetcher{clientFactory: clientFactory}
}

// Fetch retrieves an RSS feed with retries.
func (f *GoFeedFetcher) Fetch(ctx context.Context, url string, etag, lastModified *string, proxy *database.Proxy) (*interfaces.FetchResult, error) {
	var lastErr error
	currentDelay := initialRetryDelay // Now defined

	for attempt := 0; attempt <= maxFetchRetries; attempt++ { // Now defined
		if attempt > 0 {
			log.Warn().Str("feed_url", url).Int("attempt", attempt).Dur("delay", currentDelay).Msg("Retrying fetch after error")
			select {
			case <-time.After(currentDelay):
				currentDelay *= 2
				if currentDelay > maxRetryDelay { // Now defined
					currentDelay = maxRetryDelay // Now defined
				}
			case <-ctx.Done():
				return nil, fmt.Errorf("fetch context cancelled during retry backoff for %s: %w", url, ctx.Err())
			}
		}

		httpClient, errClient := f.clientFactory.GetClient(proxy)
		if errClient != nil {
			return nil, fmt.Errorf("failed to get HTTP client for %s: %w", url, errClient)
		}

		req, errReq := http.NewRequestWithContext(ctx, "GET", url, nil)
		if errReq != nil {
			return nil, fmt.Errorf("failed to create request for %s: %w", url, errReq)
		}
		if etag != nil && *etag != "" {
			req.Header.Set("If-None-Match", *etag)
		}
		if lastModified != nil && *lastModified != "" {
			req.Header.Set("If-Modified-Since", *lastModified)
		}
		req.Header.Set("User-Agent", "RSSBot/1.0 (+https://your.bot.contact.info)")

		resp, errDo := httpClient.Do(req)
		if errDo != nil {
			lastErr = fmt.Errorf("attempt %d: failed to fetch feed %s: %w", attempt, url, errDo)
			if errors.Is(errDo, context.Canceled) || errors.Is(errDo, context.DeadlineExceeded) {
				return nil, lastErr
			}
			continue
		}

		if resp.StatusCode == http.StatusNotModified {
			log.Debug().Str("feed_url", url).Msg("Feed not modified (304)")
			resp.Body.Close()
			return &interfaces.FetchResult{Feed: nil, NewEtag: etag, NewLastModified: lastModified}, nil
		}

		if resp.StatusCode != http.StatusOK {
			bodyBytes, _ := io.ReadAll(io.LimitReader(resp.Body, 1024))
			resp.Body.Close()
			lastErr = fmt.Errorf("attempt %d: failed to fetch feed %s: status %d, body: %s", attempt, url, resp.StatusCode, string(bodyBytes))
			if resp.StatusCode >= 400 && resp.StatusCode < 500 {
				return nil, lastErr
			}
			continue
		}

		fp := gofeed.NewParser()
		feed, errParse := fp.Parse(resp.Body)
		resp.Body.Close()
		if errParse != nil {
			lastErr = fmt.Errorf("attempt %d: failed to parse feed %s: %w", attempt, url, errParse)
			continue
		}

		newEtagHeader := resp.Header.Get("ETag")
		newLastModifiedHeader := resp.Header.Get("Last-Modified")
		return &interfaces.FetchResult{
			Feed:            feed,
			NewEtag:         &newEtagHeader,
			NewLastModified: &newLastModifiedHeader,
		}, nil
	}
	return nil, fmt.Errorf("all %d fetch attempts failed for %s: last error: %w", maxFetchRetries+1, url, lastErr) // Now defined
}

// GetNewItems function (ensure this is correct from previous steps)
func GetNewItems(feedData *gofeed.Feed, isItemProcessedFunc func(itemGUIDHash string) (bool, error)) ([]*gofeed.Item, string, error) {
    var newItems []*gofeed.Item
    var latestItemHash string // This will be the hash of the newest item in the current fetch data

    if feedData == nil || len(feedData.Items) == 0 {
        return newItems, "", nil
    }

    // Sort items by date, most recent first.
    sort.SliceStable(feedData.Items, func(i, j int) bool {
        dateI := feedData.Items[i].PublishedParsed
        if dateI == nil { dateI = feedData.Items[i].UpdatedParsed }
        dateJ := feedData.Items[j].PublishedParsed
        if dateJ == nil { dateJ = feedData.Items[j].UpdatedParsed }

        if dateI == nil && dateJ == nil { return i < j } // Maintain original order if no dates
        if dateI == nil { return false } // Items with dates come before those without
        if dateJ == nil { return true }
        return dateI.After(*dateJ)
    })

    // The newest item in the feed (after sorting) is feedData.Items[0]
    // We'll use its hash as the potential new "high water mark" for the feed's LastProcessedItemGUIDHash
    // if no *new* items are actually sent.
    if len(feedData.Items) > 0 {
        newestItemInFeed := feedData.Items[0]
        identifier := newestItemInFeed.GUID
        if identifier == "" {
            identifier = newestItemInFeed.Link
        }
        if identifier != "" {
            latestItemHash = fmt.Sprintf("%x", sha256.Sum256([]byte(identifier)))
        }
    }


    for _, item := range feedData.Items {
        itemIdentifier := item.GUID
        if itemIdentifier == "" {
            itemIdentifier = item.Link
        }
        if itemIdentifier == "" {
            log.Warn().Str("item_title", item.Title).Msg("Item has no GUID or Link, cannot process.")
            continue
        }

        hash := fmt.Sprintf("%x", sha256.Sum256([]byte(itemIdentifier)))

        processed, err := isItemProcessedFunc(hash)
        if err != nil {
            return nil, "", fmt.Errorf("checking if item processed (hash %s): %w", hash, err)
        }

        if !processed {
            newItems = append(newItems, item)
            // No need to update latestItemHash here based on *new* items,
            // latestItemHash already reflects the newest item from the entire fetched feed.
        }
    }

    // Reverse newItems to process them in chronological order (oldest new to newest new)
    for i, j := 0, len(newItems)-1; i < j; i, j = i+1, j-1 {
        newItems[i], newItems[j] = newItems[j], newItems[i]
    }

    return newItems, latestItemHash, nil
}