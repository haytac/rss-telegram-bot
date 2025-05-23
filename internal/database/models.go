package database

import (
	"encoding/json"
	"time"
)

// Proxy represents a proxy configuration.
type Proxy struct {
	ID                 int64     `db:"id"`
	Name               string    `db:"name"`
	Type               string    `db:"type"` // http, https, socks5
	Address            string    `db:"address"`
	Username           *string   `db:"username"`
	Password           *string   `db:"password"`
	IsDefaultForRSS    bool      `db:"is_default_for_rss"`
	IsDefaultForTelegram bool    `db:"is_default_for_telegram"`
	CreatedAt          time.Time `db:"created_at"`
	UpdatedAt          time.Time `db:"updated_at"`
}

// TelegramBot represents a Telegram bot configuration.
type TelegramBot struct {
	ID             int64     `db:"id"`
	TokenHash      string    `db:"token_hash"` // Store hash, not raw token
	EncryptedToken *string   `db:"encrypted_token"` // Store "encrypted" token
	Description    *string   `db:"description"`
	CreatedAt      time.Time `db:"created_at"`
	UpdatedAt      time.Time `db:"updated_at"`
}

// FormattingProfileConfig holds detailed formatting settings.
type FormattingProfileConfig struct {
	TitleTemplate             string   `json:"title_template,omitempty"`              // Go template for item title
	MessageTemplate           string   `json:"message_template,omitempty"`            // Go template for item body
	Hashtags                  []string `json:"hashtags,omitempty"`                    // Static or dynamic hashtags
	IncludeAuthor             bool     `json:"include_author,omitempty"`
	OmitGenericTitleRegex     string   `json:"omit_generic_title_regex,omitempty"`
	UseTelegraphThresholdChars int      `json:"use_telegraph_threshold_chars,omitempty"` // 0 means disabled
	ReplaceEmojiImagesWithAlt bool     `json:"replace_emoji_images_with_alt,omitempty"`
	MediaFilterRegex          string   `json:"media_filter_regex,omitempty"`
	MediaFilterCSSSelector    string   `json:"media_filter_css_selector,omitempty"`
	// Add more specific media handling preferences here
}

// FormattingProfile represents a formatting profile.
type FormattingProfile struct {
	ID            int64     `db:"id"`
	Name          string    `db:"name"`
	ConfigJSON    string    `db:"template_config"` // Raw JSON string from DB
	ParsedConfig  FormattingProfileConfig // Parsed version
	CreatedAt     time.Time `db:"created_at"`
	UpdatedAt     time.Time `db:"updated_at"`
}

// UnmarshalConfig parses ConfigJSON into ParsedConfig.
func (fp *FormattingProfile) UnmarshalConfig() error {
	if fp.ConfigJSON == "" {
		fp.ParsedConfig = FormattingProfileConfig{} // Default empty config
		return nil
	}
	return json.Unmarshal([]byte(fp.ConfigJSON), &fp.ParsedConfig)
}

// MarshalConfig serializes ParsedConfig into ConfigJSON.
func (fp *FormattingProfile) MarshalConfig() error {
	data, err := json.Marshal(fp.ParsedConfig)
	if err != nil {
		return err
	}
	fp.ConfigJSON = string(data)
	return nil
}


// Feed represents an RSS feed configuration.
type Feed struct {
	ID                          int64      `db:"id"`
	URL                         string     `db:"url"`
	UserTitle                   *string    `db:"user_title"`
	FrequencySeconds            int        `db:"frequency_seconds"`
	TelegramBotID               *int64     `db:"telegram_bot_id"`
	TelegramChatID              string     `db:"telegram_chat_id"`
	LastProcessedItemGUIDHash *string    `db:"last_processed_item_guid_hash"`
	LastFetchedAt               *time.Time `db:"last_fetched_at"`
	ProxyID                     *int64     `db:"proxy_id"`
	FormattingProfileID         *int64     `db:"formatting_profile_id"`
	IsEnabled                   bool       `db:"is_enabled"`
	HTTPEtag                    *string    `db:"http_etag"`
	HTTPLastModified            *string    `db:"http_last_modified"`
	CreatedAt                   time.Time  `db:"created_at"`
	UpdatedAt                   time.Time  `db:"updated_at"`

	// Joined data (populated by specific queries)
	BotToken            *string // Actual bot token, fetched separately for security
	Proxy               *Proxy
	FormattingProfile   *FormattingProfile
}

// ProcessedItem tracks items that have been sent to Telegram.
type ProcessedItem struct {
	ID           int64     `db:"id"`
	FeedID       int64     `db:"feed_id"`
	ItemGUIDHash string    `db:"item_guid_hash"`
	ProcessedAt  time.Time `db:"processed_at"`
}

