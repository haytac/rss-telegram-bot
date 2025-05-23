package database

import (
	"context"
	"crypto/aes"
	"crypto/cipher"
	"crypto/rand"
	"crypto/sha256"
	"database/sql"
	"encoding/base64"
	"encoding/hex"
	"errors"
	"fmt"
	"io"

	"github.com/rs/zerolog/log"
)

// IMPORTANT: This is a placeholder for a securely managed encryption key.
// In a real application, this key MUST be:
// - Generated securely (e.g., crypto/rand).
// - Stored securely (e.g., environment variable, secrets manager like Vault, KMS).
// - NEVER hardcoded in source code.
// For this example, we'll derive it from a "master password" in config, but this is still not ideal.
// A 32-byte key is required for AES-256.
var demoEncryptionKey []byte // To be initialized from config

// InitEncryptionKey initializes the demo key. CALL THIS FROM MAIN/APP SETUP.
// THIS IS STILL NOT PRODUCTION SAFE.
func InitEncryptionKey(keyString string) error {
	if len(keyString) == 0 {
		log.Warn().Msg("Encryption key is empty. Token encryption/decryption will NOT be secure. THIS IS FOR DEMO ONLY.")
		// Use a default insecure key for demo if nothing provided, to make it runnable
		demoEncryptionKey = []byte("a very insecure default key 123!") // Must be 32 bytes for AES-256
		if len(demoEncryptionKey) < 32 {
		    padding := make([]byte, 32-len(demoEncryptionKey))
		    demoEncryptionKey = append(demoEncryptionKey,padding...)
        }
        demoEncryptionKey = demoEncryptionKey[:32]
		return errors.New("encryption key not configured; using highly insecure default for demo")
	}
    // Derive a 32-byte key from the input string using SHA-256
    // This is better than directly using the string if it's not 32 bytes, but still relies on the secrecy of keyString
    hasher := sha256.New()
    hasher.Write([]byte(keyString))
    demoEncryptionKey = hasher.Sum(nil) // SHA-256 produces 32 bytes
	log.Info().Msg("Demo encryption key initialized (WARNING: For demo purposes only).")
	return nil
}

// encryptAES encrypts text using AES-GCM.
// WARNING: THIS IS A SIMPLIFIED EXAMPLE. Production use requires careful IV management and error handling.
func encryptAES(key []byte, plaintext string) (string, error) {
	if len(key) == 0 {
		log.Error().Msg("Attempted to encrypt with an empty key. THIS IS A SEVERE SECURITY RISK.")
		// Fallback to plain text for demo if key is not set, to avoid crashing, but log heavily.
		// In production, this should be a fatal error.
		return plaintext, errors.New("encryption key is empty, cannot encrypt (returning plaintext for demo)")
	}
	block, err := aes.NewCipher(key)
	if err != nil {
		return "", fmt.Errorf("encrypt NewCipher: %w", err)
	}
	gcm, err := cipher.NewGCM(block)
	if err != nil {
		return "", fmt.Errorf("encrypt NewGCM: %w", err)
	}
	nonce := make([]byte, gcm.NonceSize())
	if _, err = io.ReadFull(rand.Reader, nonce); err != nil {
		return "", fmt.Errorf("encrypt ReadFull nonce: %w", err)
	}
	ciphertext := gcm.Seal(nonce, nonce, []byte(plaintext), nil)
	return base64.StdEncoding.EncodeToString(ciphertext), nil
}

// decryptAES decrypts text using AES-GCM.
func decryptAES(key []byte, cryptoText string) (string, error) {
	if len(key) == 0 {
		log.Error().Msg("Attempted to decrypt with an empty key. THIS IS A SEVERE SECURITY RISK.")
		// Fallback to returning the "encrypted" text as is for demo.
		// In production, this should be a fatal error or return an error indicating decryption failure.
		return cryptoText, errors.New("encryption key is empty, cannot decrypt (returning cryptoText for demo)")
	}
	data, err := base64.StdEncoding.DecodeString(cryptoText)
	if err != nil {
		return "", fmt.Errorf("decrypt DecodeString: %w", err)
	}
	block, err := aes.NewCipher(key)
	if err != nil {
		return "", fmt.Errorf("decrypt NewCipher: %w", err)
	}
	gcm, err := cipher.NewGCM(block)
	if err != nil {
		return "", fmt.Errorf("decrypt NewGCM: %w", err)
	}
	nonceSize := gcm.NonceSize()
	if len(data) < nonceSize {
		return "", errors.New("ciphertext too short")
	}
	nonce, ciphertext := data[:nonceSize], data[nonceSize:]
	plaintext, err := gcm.Open(nil, nonce, ciphertext, nil)
	if err != nil {
		return "", fmt.Errorf("decrypt GCM Open: %w", err)
	}
	return string(plaintext), nil
}


// TelegramBotStore ... (struct definition remains)
type TelegramBotStore struct {
	db *DB
}

// NewTelegramBotStore ... (constructor remains)
func NewTelegramBotStore(db *DB) *TelegramBotStore {
	return &TelegramBotStore{db: db}
}

func hashToken(token string) string {
	hasher := sha256.New()
	hasher.Write([]byte(token))
	return hex.EncodeToString(hasher.Sum(nil))
}

// CreateBot adds a new Telegram bot configuration.
func (s *TelegramBotStore) CreateBot(ctx context.Context, rawToken string, description *string) (int64, error) {
	if len(demoEncryptionKey) == 0 {
		log.Error().Msg("Demo encryption key not initialized. Bot token will not be properly secured.")
		// Proceed with insecure storage for demo if key is not set, but this is bad.
		// return 0, errors.New("encryption key not initialized, cannot create bot securely")
	}

	tokenHash := hashToken(rawToken)
	encryptedToken, err := encryptAES(demoEncryptionKey, rawToken)
	if err != nil {
		// If encryption fails (e.g. due to empty key in demo), we might store it raw or fail.
		// For this demo, we'll log the error and proceed if encryptAES returned the raw token.
		log.Error().Err(err).Msg("Failed to encrypt bot token. Storing might be insecure.")
		if encryptedToken == rawToken { // This happens if encryptAES falls back due to no key
		    log.Warn().Msg("Storing raw token due to encryption fallback. THIS IS INSECURE.")
        } else { // A real encryption error occurred
            return 0, fmt.Errorf("CreateBot encryption failed: %w", err)
        }
	}

	stmt, err := s.db.PrepareContext(ctx, `
		INSERT INTO telegram_bots (token_hash, encrypted_token, description) VALUES (?, ?, ?)`)
	if err != nil {
		return 0, fmt.Errorf("CreateBot prepare: %w", err)
	}
	defer stmt.Close()

	res, err := stmt.ExecContext(ctx, tokenHash, encryptedToken, description)
	if err != nil {
		return 0, fmt.Errorf("CreateBot exec: %w", err)
	}
	return res.LastInsertId()
}

// GetBotByID retrieves bot metadata.
func (s *TelegramBotStore) GetBotByID(ctx context.Context, id int64) (*TelegramBot, error) {
	query := `SELECT id, token_hash, encrypted_token, description, created_at, updated_at FROM telegram_bots WHERE id = ?`
	row := s.db.QueryRowContext(ctx, query, id)
	bot := &TelegramBot{}
	var encryptedToken sql.NullString
	err := row.Scan(&bot.ID, &bot.TokenHash, &encryptedToken, &bot.Description, &bot.CreatedAt, &bot.UpdatedAt)
	if err != nil {
		if err == sql.ErrNoRows { return nil, nil }
		return nil, fmt.Errorf("GetBotByID scan: %w", err)
	}
	if encryptedToken.Valid {
		bot.EncryptedToken = &encryptedToken.String
	}
	return bot, nil
}

// GetTokenByBotID retrieves and "decrypts" the raw bot token.
func (s *TelegramBotStore) GetTokenByBotID(ctx context.Context, id int64) (string, error) {
	if len(demoEncryptionKey) == 0 {
		log.Error().Msg("Demo encryption key not initialized. Bot token cannot be properly decrypted.")
		// return "", errors.New("encryption key not initialized, cannot decrypt token")
	}
	var encryptedToken sql.NullString
	query := `SELECT encrypted_token FROM telegram_bots WHERE id = ?`
	err := s.db.QueryRowContext(ctx, query, id).Scan(&encryptedToken)
	if err != nil {
		if err == sql.ErrNoRows {
			return "", fmt.Errorf("bot with ID %d not found for token retrieval", id)
		}
		return "", fmt.Errorf("GetTokenByBotID query for bot %d: %w", id, err)
	}

	if !encryptedToken.Valid || encryptedToken.String == "" {
		return "", fmt.Errorf("no encrypted token found for bot ID %d", id)
	}

	decryptedToken, err := decryptAES(demoEncryptionKey, encryptedToken.String)
	if err != nil {
		// If decryption fails (e.g. key mismatch or data corruption, or demo key not set)
		log.Error().Err(err).Int64("bot_id", id).Msg("Failed to decrypt bot token.")
        if decryptedToken == encryptedToken.String { // This happens if decryptAES falls back due to no key
            log.Warn().Msg("Returning potentially raw/undecrypted token due to decryption fallback. THIS IS INSECURE.")
        } else { // A real decryption error
		    return "", fmt.Errorf("GetTokenByBotID decryption for bot %d failed: %w", id, err)
        }
	}
	return decryptedToken, nil
}

// ListBots retrieves all bot configurations (metadata only, not decrypted tokens).
func (s *TelegramBotStore) ListBots(ctx context.Context) ([]*TelegramBot, error) {
	query := `SELECT id, token_hash, encrypted_token, description, created_at, updated_at FROM telegram_bots ORDER BY id`
	rows, err := s.db.QueryContext(ctx, query)
	if err != nil {
		return nil, fmt.Errorf("ListBots query: %w", err)
	}
	defer rows.Close()

	var bots []*TelegramBot
	for rows.Next() {
		bot := &TelegramBot{}
		var encryptedToken sql.NullString
		err := rows.Scan(&bot.ID, &bot.TokenHash, &encryptedToken, &bot.Description, &bot.CreatedAt, &bot.UpdatedAt)
		if err != nil { return nil, fmt.Errorf("ListBots scan: %w", err) }
		if encryptedToken.Valid {
			bot.EncryptedToken = &encryptedToken.String
		}
		bots = append(bots, bot)
	}
	if err = rows.Err(); err != nil { return nil, fmt.Errorf("ListBots rows error: %w", err) }
	return bots, nil
}