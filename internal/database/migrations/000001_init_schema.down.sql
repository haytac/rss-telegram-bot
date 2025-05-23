-- File: 000002_add_encrypted_token_to_bots.up.sql
ALTER TABLE telegram_bots ADD COLUMN encrypted_token TEXT;