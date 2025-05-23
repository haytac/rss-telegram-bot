ALTER TABLE telegram_bots ADD COLUMN encrypted_token TEXT;
-- Optionally, add the trigger here if it wasn't in the init schema
CREATE TRIGGER IF NOT EXISTS update_telegram_bots_updated_at
AFTER UPDATE ON telegram_bots
FOR EACH ROW
BEGIN
    UPDATE telegram_bots SET updated_at = CURRENT_TIMESTAMP WHERE id = OLD.id;
END;