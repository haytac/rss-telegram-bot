package database

import (
	"context"
	"database/sql"
	"fmt"
	"time" // Added for UpdateFeedLastProcessed and AddProcessedItem timestamps
)

// FeedStore provides methods to interact with feeds in the database.
type FeedStore struct {
	db *DB // Assuming DB is your *sql.DB wrapper defined in database.go
}

// NewFeedStore creates a new FeedStore.
func NewFeedStore(db *DB) *FeedStore {
	return &FeedStore{db: db}
}

// Helper to scan a feed row and potentially its joined data
func scanFeed(scanner interface{ Scan(...interface{}) error }, feed *Feed) error {
	// Define nullable fields for joined tables
	var (
		proxyID                 sql.NullInt64
		proxyName               sql.NullString
		proxyType               sql.NullString
		proxyAddress            sql.NullString
		proxyUsername           sql.NullString
		proxyPassword           sql.NullString
		proxyIsDefaultForRSS    sql.NullBool
		proxyIsDefaultForTelegram sql.NullBool
		formatProfileID         sql.NullInt64
		formatProfileName       sql.NullString
		formatProfileConfigJSON sql.NullString
	)

	// Note: Scanning directly into feed.TelegramBotID (if it's *int64)
	// will correctly set it to nil if the DB column is NULL.
	// Similarly for feed.UserTitle, feed.LastProcessedItemGUIDHash, feed.LastFetchedAt,
	// feed.HTTPEtag, feed.HTTPLastModified if they are pointer types.
	err := scanner.Scan(
		&feed.ID, &feed.URL, &feed.UserTitle, &feed.FrequencySeconds, &feed.TelegramBotID, &feed.TelegramChatID,
		&feed.LastProcessedItemGUIDHash, &feed.LastFetchedAt, &feed.IsEnabled,
		&feed.HTTPEtag, &feed.HTTPLastModified, &feed.CreatedAt, &feed.UpdatedAt,
		// Joined proxy fields
		&proxyID, &proxyName, &proxyType, &proxyAddress, &proxyUsername, &proxyPassword, &proxyIsDefaultForRSS, &proxyIsDefaultForTelegram,
		// Joined formatting profile fields
		&formatProfileID, &formatProfileName, &formatProfileConfigJSON,
	)
	if err != nil {
		return err
	}

	// Handle feed.ProxyID (*int64)
	if proxyID.Valid {
		val := proxyID.Int64
		feed.ProxyID = &val // Store the original ProxyID from the feeds table
	} else {
		feed.ProxyID = nil
	}

	if proxyID.Valid {
		feed.Proxy = &Proxy{ // Proxy struct from models.go
			ID:                 proxyID.Int64,
			Name:               proxyName.String,
			Type:               proxyType.String,
			Address:            proxyAddress.String,
		}
		if proxyUsername.Valid {
			feed.Proxy.Username = &proxyUsername.String
		}
		if proxyPassword.Valid {
			feed.Proxy.Password = &proxyPassword.String
		}
		if proxyIsDefaultForRSS.Valid {
			feed.Proxy.IsDefaultForRSS = proxyIsDefaultForRSS.Bool
		}
		if proxyIsDefaultForTelegram.Valid {
			feed.Proxy.IsDefaultForTelegram = proxyIsDefaultForTelegram.Bool
		}
	} else {
		feed.Proxy = nil // Ensure Proxy struct is nil if no associated proxy
	}

	// Handle feed.FormattingProfileID (*int64)
	if formatProfileID.Valid {
		val := formatProfileID.Int64
		feed.FormattingProfileID = &val // Store the original FormattingProfileID
	} else {
		feed.FormattingProfileID = nil
	}

	if formatProfileID.Valid {
		feed.FormattingProfile = &FormattingProfile{ // FormattingProfile struct from models.go
			ID:         formatProfileID.Int64,
			Name:       formatProfileName.String,
			ConfigJSON: formatProfileConfigJSON.String,
		}
		if err := feed.FormattingProfile.UnmarshalConfig(); err != nil {
			return fmt.Errorf("failed to unmarshal formatting profile %d for feed %d: %w", formatProfileID.Int64, feed.ID, err)
		}
	} else {
		feed.FormattingProfile = nil // Ensure FormattingProfile struct is nil
	}

	return nil
}

// GetFeedByID retrieves a feed by its ID, including related proxy and formatting profile.
func (s *FeedStore) GetFeedByID(ctx context.Context, id int64) (*Feed, error) {
	query := `
	SELECT 
		f.id, f.url, f.user_title, f.frequency_seconds, f.telegram_bot_id, f.telegram_chat_id,
		f.last_processed_item_guid_hash, f.last_fetched_at, f.is_enabled,
		f.http_etag, f.http_last_modified, f.created_at, f.updated_at,
		
		p.id AS proxy_id_joined, p.name AS proxy_name, p.type AS proxy_type, 
		p.address AS proxy_address, p.username AS proxy_username, p.password AS proxy_password,
		p.is_default_for_rss, p.is_default_for_telegram,

		fp.id AS fp_id_joined, fp.name AS fp_name, fp.template_config AS fp_config_json
	FROM feeds f
	LEFT JOIN proxies p ON f.proxy_id = p.id
	LEFT JOIN formatting_profiles fp ON f.formatting_profile_id = fp.id
	WHERE f.id = ?`

	row := s.db.QueryRowContext(ctx, query, id)
	feed := &Feed{} // Feed struct from models.go

	err := scanFeed(row, feed)
	if err != nil {
		if err == sql.ErrNoRows {
			return nil, nil // Or a custom ErrNotFound
		}
		return nil, fmt.Errorf("GetFeedByID scan: %w", err)
	}
	return feed, nil
}

// GetEnabledFeeds retrieves all enabled feeds with their related proxy and formatting profiles.
func (s *FeedStore) GetEnabledFeeds(ctx context.Context) ([]*Feed, error) {
	query := `
	SELECT 
		f.id, f.url, f.user_title, f.frequency_seconds, f.telegram_bot_id, f.telegram_chat_id,
		f.last_processed_item_guid_hash, f.last_fetched_at, f.is_enabled,
		f.http_etag, f.http_last_modified, f.created_at, f.updated_at,

		p.id AS proxy_id_joined, p.name AS proxy_name, p.type AS proxy_type, 
		p.address AS proxy_address, p.username AS proxy_username, p.password AS proxy_password,
		p.is_default_for_rss, p.is_default_for_telegram,

		fp.id AS fp_id_joined, fp.name AS fp_name, fp.template_config AS fp_config_json
	FROM feeds f
	LEFT JOIN proxies p ON f.proxy_id = p.id
	LEFT JOIN formatting_profiles fp ON f.formatting_profile_id = fp.id
	WHERE f.is_enabled = TRUE
	ORDER BY f.id`

	rows, err := s.db.QueryContext(ctx, query)
	if err != nil {
		return nil, fmt.Errorf("GetEnabledFeeds query: %w", err)
	}
	defer rows.Close()

	var feeds []*Feed
	for rows.Next() {
		feed := &Feed{} // Feed struct from models.go
		err := scanFeed(rows, feed)
		if err != nil {
			return nil, fmt.Errorf("GetEnabledFeeds scan: %w", err)
		}
		feeds = append(feeds, feed)
	}
	if err = rows.Err(); err != nil {
		return nil, fmt.Errorf("GetEnabledFeeds rows error: %w", err)
	}
	return feeds, nil
}

// CreateFeed adds a new feed to the database.
func (s *FeedStore) CreateFeed(ctx context.Context, feed *Feed) (int64, error) {
	stmt, err := s.db.PrepareContext(ctx, `
		INSERT INTO feeds (url, user_title, frequency_seconds, telegram_bot_id, telegram_chat_id, 
		                   proxy_id, formatting_profile_id, is_enabled)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?)`)
	if err != nil {
		return 0, fmt.Errorf("CreateFeed prepare: %w", err)
	}
	defer stmt.Close()

	res, err := stmt.ExecContext(ctx, feed.URL, feed.UserTitle, feed.FrequencySeconds,
		feed.TelegramBotID, feed.TelegramChatID, feed.ProxyID, feed.FormattingProfileID, feed.IsEnabled)
	if err != nil {
		return 0, fmt.Errorf("CreateFeed exec: %w", err)
	}
	return res.LastInsertId()
}

// UpdateFeed updates an existing feed.
// Note: This is a basic update; a real one might use optional fields or a map for partial updates.
func (s *FeedStore) UpdateFeed(ctx context.Context, feed *Feed) error {
	stmt, err := s.db.PrepareContext(ctx, `
		UPDATE feeds 
		SET url = ?, user_title = ?, frequency_seconds = ?, telegram_bot_id = ?, telegram_chat_id = ?,
		    proxy_id = ?, formatting_profile_id = ?, is_enabled = ?,
		    last_processed_item_guid_hash = ?, last_fetched_at = ?, http_etag = ?, http_last_modified = ?
		WHERE id = ?`)
	if err != nil {
		return fmt.Errorf("UpdateFeed prepare: %w", err)
	}
	defer stmt.Close()

	_, err = stmt.ExecContext(ctx,
		feed.URL, feed.UserTitle, feed.FrequencySeconds, feed.TelegramBotID, feed.TelegramChatID,
		feed.ProxyID, feed.FormattingProfileID, feed.IsEnabled,
		feed.LastProcessedItemGUIDHash, feed.LastFetchedAt, feed.HTTPEtag, feed.HTTPLastModified,
		feed.ID)
	if err != nil {
		return fmt.Errorf("UpdateFeed exec for feed ID %d: %w", feed.ID, err)
	}
	return nil
}

// DeleteFeed deletes a feed by its ID.
func (s *FeedStore) DeleteFeed(ctx context.Context, id int64) error {
	stmt, err := s.db.PrepareContext(ctx, `DELETE FROM feeds WHERE id = ?`)
	if err != nil {
		return fmt.Errorf("DeleteFeed prepare: %w", err)
	}
	defer stmt.Close()

	res, err := stmt.ExecContext(ctx, id)
	if err != nil {
		return fmt.Errorf("DeleteFeed exec for ID %d: %w", id, err)
	}
	rowsAffected, err := res.RowsAffected()
	if err != nil {
		return fmt.Errorf("DeleteFeed RowsAffected for ID %d: %w", id, err)
	}
	if rowsAffected == 0 {
		return fmt.Errorf("DeleteFeed: no feed found with ID %d", id) // Or return sql.ErrNoRows equivalent
	}
	return nil
}


// UpdateFeedLastProcessed updates tracking info for a feed after a fetch attempt.
func (s *FeedStore) UpdateFeedLastProcessed(ctx context.Context, feedID int64, lastItemHash, etag, lastModified *string) error {
	now := time.Now() // Capture current time for last_fetched_at

	// Prepare arguments, handling potential nil pointers from input by converting to sql.NullString
	var sqlLastItemHash sql.NullString
	if lastItemHash != nil {
		sqlLastItemHash = sql.NullString{String: *lastItemHash, Valid: true}
	}
	var sqlEtag sql.NullString
	if etag != nil {
		sqlEtag = sql.NullString{String: *etag, Valid: true}
	}
	var sqlLastModified sql.NullString
	if lastModified != nil {
		sqlLastModified = sql.NullString{String: *lastModified, Valid: true}
	}


	stmt, err := s.db.PrepareContext(ctx, `
		UPDATE feeds 
		SET last_processed_item_guid_hash = ?, http_etag = ?, http_last_modified = ?, last_fetched_at = ?
		WHERE id = ?`)
	if err != nil {
		return fmt.Errorf("UpdateFeedLastProcessed prepare: %w", err)
	}
	defer stmt.Close()

	_, err = stmt.ExecContext(ctx, sqlLastItemHash, sqlEtag, sqlLastModified, now, feedID)
	if err != nil {
		return fmt.Errorf("UpdateFeedLastProcessed exec: %w", err)
	}
	return nil
}

// AddProcessedItem records an item as processed.
func (s *FeedStore) AddProcessedItem(ctx context.Context, feedID int64, itemGUIDHash string) error {
	// Using INSERT OR IGNORE to prevent errors if the item was already processed
	// (e.g., due to a retry or race condition, though a robust system would try to avoid this).
	// The processed_at timestamp will only be set on the initial successful insert.
	stmt, err := s.db.PrepareContext(ctx, `
		INSERT OR IGNORE INTO processed_items (feed_id, item_guid_hash, processed_at) VALUES (?, ?, ?)`)
	if err != nil {
		return fmt.Errorf("AddProcessedItem prepare: %w", err)
	}
	defer stmt.Close()

	now := time.Now()
	_, err = stmt.ExecContext(ctx, feedID, itemGUIDHash, now)
	if err != nil {
		return fmt.Errorf("AddProcessedItem exec: %w", err)
	}
	return nil
}

// IsItemProcessed checks if an item has already been processed for a feed.
func (s *FeedStore) IsItemProcessed(ctx context.Context, feedID int64, itemGUIDHash string) (bool, error) {
	var exists int
	query := `SELECT EXISTS(SELECT 1 FROM processed_items WHERE feed_id = ? AND item_guid_hash = ? LIMIT 1)`
	err := s.db.QueryRowContext(ctx, query, feedID, itemGUIDHash).Scan(&exists)
	if err != nil {
		// If QueryRowContext returns sql.ErrNoRows, Scan will also return it.
		// However, SELECT EXISTS should always return one row (with 0 or 1).
		// So, an error here is likely a real DB error, not just "not found."
		return false, fmt.Errorf("IsItemProcessed query: %w", err)
	}
	return exists == 1, nil
}