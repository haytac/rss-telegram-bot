DROP INDEX IF EXISTS idx_formatting_profiles_name;
DROP INDEX IF EXISTS idx_telegram_bots_token_hash;
DROP INDEX IF EXISTS idx_proxies_name;
DROP INDEX IF EXISTS idx_processed_items_feed_id_guid_hash;
DROP INDEX IF EXISTS idx_feeds_is_enabled;
DROP INDEX IF EXISTS idx_feeds_url;

DROP TRIGGER IF EXISTS update_feeds_updated_at;
-- Drop other triggers

DROP TABLE IF EXISTS processed_items;
DROP TABLE IF EXISTS feeds;
DROP TABLE IF EXISTS formatting_profiles;
DROP TABLE IF EXISTS telegram_bots;
DROP TABLE IF EXISTS proxies;

-- ...
DROP TRIGGER IF EXISTS update_telegram_bots_updated_at;
DROP TABLE IF EXISTS telegram_bots;
-- ...