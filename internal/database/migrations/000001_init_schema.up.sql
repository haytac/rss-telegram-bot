CREATE TABLE proxies (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    name TEXT UNIQUE NOT NULL,
    type TEXT CHECK(type IN ('http', 'https', 'socks5')) NOT NULL,
    address TEXT NOT NULL, -- host:port
    username TEXT,
    password TEXT,
    is_default_for_rss BOOLEAN DEFAULT FALSE,
    is_default_for_telegram BOOLEAN DEFAULT FALSE,
    created_at DATETIME DEFAULT CURRENT_TIMESTAMP,
    updated_at DATETIME DEFAULT CURRENT_TIMESTAMP
);

CREATE TABLE telegram_bots (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    token_hash TEXT UNIQUE NOT NULL, -- Store hash of token, not raw token
    description TEXT,
    created_at DATETIME DEFAULT CURRENT_TIMESTAMP,
    updated_at DATETIME DEFAULT CURRENT_TIMESTAMP
);

CREATE TABLE formatting_profiles (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    name TEXT UNIQUE NOT NULL,
    -- Store complex settings as JSON for flexibility
    template_config TEXT, -- JSON: { "title_template": "...", "message_template": "...", "hashtags": [...], ... }
    created_at DATETIME DEFAULT CURRENT_TIMESTAMP,
    updated_at DATETIME DEFAULT CURRENT_TIMESTAMP
);

CREATE TABLE feeds (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    url TEXT UNIQUE NOT NULL,
    user_title TEXT,
    frequency_seconds INTEGER NOT NULL DEFAULT 300,
    telegram_bot_id INTEGER, -- FK
    telegram_chat_id TEXT NOT NULL,
    last_processed_item_guid_hash TEXT, -- Hash of GUID or Link of the last processed item
    last_fetched_at DATETIME,
    proxy_id INTEGER, -- FK
    formatting_profile_id INTEGER, -- FK
    is_enabled BOOLEAN DEFAULT TRUE,
    http_etag TEXT,
    http_last_modified TEXT,
    created_at DATETIME DEFAULT CURRENT_TIMESTAMP,
    updated_at DATETIME DEFAULT CURRENT_TIMESTAMP,
    FOREIGN KEY (telegram_bot_id) REFERENCES telegram_bots(id) ON DELETE SET NULL,
    FOREIGN KEY (proxy_id) REFERENCES proxies(id) ON DELETE SET NULL,
    FOREIGN KEY (formatting_profile_id) REFERENCES formatting_profiles(id) ON DELETE SET NULL
);

CREATE TABLE processed_items (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    feed_id INTEGER NOT NULL,
    item_guid_hash TEXT NOT NULL, -- Hash of GUID or Link to identify item uniquely per feed
    processed_at DATETIME DEFAULT CURRENT_TIMESTAMP,
    FOREIGN KEY (feed_id) REFERENCES feeds(id) ON DELETE CASCADE,
    UNIQUE (feed_id, item_guid_hash)
);

-- Indexes
CREATE INDEX idx_feeds_url ON feeds(url);
CREATE INDEX idx_feeds_is_enabled ON feeds(is_enabled);
CREATE INDEX idx_processed_items_feed_id_guid_hash ON processed_items(feed_id, item_guid_hash);
CREATE INDEX idx_proxies_name ON proxies(name);
CREATE INDEX idx_telegram_bots_token_hash ON telegram_bots(token_hash);
CREATE INDEX idx_formatting_profiles_name ON formatting_profiles(name);

-- Triggers for updated_at
CREATE TRIGGER update_feeds_updated_at
AFTER UPDATE ON feeds
FOR EACH ROW
BEGIN
    UPDATE feeds SET updated_at = CURRENT_TIMESTAMP WHERE id = OLD.id;
END;

-- Similar triggers for proxies, telegram_bots, formatting_profiles

-- ... other tables ...

CREATE TABLE telegram_bots (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    token_hash TEXT UNIQUE NOT NULL, -- Store hash of token for uniqueness checks
    encrypted_token TEXT,          -- Placeholder for ACTUAL ENCRYPTED token
    description TEXT,
    created_at DATETIME DEFAULT CURRENT_TIMESTAMP,
    updated_at DATETIME DEFAULT CURRENT_TIMESTAMP
);

-- ... rest of the schema and triggers ...
-- Add trigger for telegram_bots updated_at if not already present
CREATE TRIGGER update_telegram_bots_updated_at
AFTER UPDATE ON telegram_bots
FOR EACH ROW
BEGIN
    UPDATE telegram_bots SET updated_at = CURRENT_TIMESTAMP WHERE id = OLD.id;
END;