-- File: 000002_add_encrypted_token_to_bots.down.sql
-- Assuming modern SQLite (3.35.0+) for simplicity here.
-- If older, use the rename/create/copy/drop method shown previously.
ALTER TABLE telegram_bots DROP COLUMN encrypted_token;

-- If using older SQLite:
/*
PRAGMA foreign_keys=off;
BEGIN TRANSACTION;
CREATE TABLE telegram_bots_new (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    token_hash TEXT UNIQUE NOT NULL,
    description TEXT,
    created_at DATETIME DEFAULT CURRENT_TIMESTAMP,
    updated_at DATETIME DEFAULT CURRENT_TIMESTAMP
);
INSERT INTO telegram_bots_new (id, token_hash, description, created_at, updated_at)
  SELECT id, token_hash, description, created_at, updated_at
  FROM telegram_bots;
DROP TABLE telegram_bots;
ALTER TABLE telegram_bots_new RENAME TO telegram_bots;
COMMIT;
PRAGMA foreign_keys=on;
*/