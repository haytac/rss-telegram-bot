ALTER TABLE telegram_bots DROP COLUMN encrypted_token;
-- DROP TRIGGER IF EXISTS update_telegram_bots_updated_at; -- Only if added in this migration