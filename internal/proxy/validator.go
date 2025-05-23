package proxy

import (
	"context"
	"fmt"
	"net/http"
	"time"

	"github.com/rs/zerolog/log"
	"github.com/haytac/rss-telegram-bot/internal/database" // Module path
	"github.com/haytac/rss-telegram-bot/pkg/interfaces"   // Module path
)

// DefaultProxyValidator implements ProxyValidator.
type DefaultProxyValidator struct {
	clientFactory interfaces.HTTPClientFactory
}

// NewDefaultProxyValidator creates a new validator.
func NewDefaultProxyValidator(factory interfaces.HTTPClientFactory) *DefaultProxyValidator {
	return &DefaultProxyValidator{clientFactory: factory}
}

// Validate checks if a proxy can connect to a target URL.
// A common targetURL for general proxy validation is something like "https://www.google.com/generate_204" or "http://detectportal.firefox.com/success.txt"
func (v *DefaultProxyValidator) Validate(ctx context.Context, p *database.Proxy, targetURL string) error {
	if targetURL == "" {
		targetURL = "https://www.google.com/generate_204" // Default validation target
	}

	client, err := v.clientFactory.GetClient(p)
	if err != nil {
		return fmt.Errorf("proxy %s (%s): failed to get HTTP client: %w", p.Name, p.Address, err)
	}

	reqCtx, cancel := context.WithTimeout(ctx, 15*time.Second) // Timeout for validation request
	defer cancel()

	req, err := http.NewRequestWithContext(reqCtx, http.MethodGet, targetURL, nil)
	if err != nil {
		return fmt.Errorf("proxy %s (%s): failed to create request to %s: %w", p.Name, p.Address, targetURL, err)
	}
	req.Header.Set("User-Agent", "RSSBotProxyValidator/1.0")


	log.Debug().Str("proxy_name", p.Name).Str("proxy_address", p.Address).Str("target_url", targetURL).Msg("Attempting to validate proxy")

	resp, err := client.Do(req)
	if err != nil {
		return fmt.Errorf("proxy %s (%s): connection test to %s failed: %w", p.Name, p.Address, targetURL, err)
	}
	defer resp.Body.Close()

	if resp.StatusCode >= 200 && resp.StatusCode < 300 {
		log.Info().Str("proxy_name", p.Name).Str("proxy_address", p.Address).Int("status_code", resp.StatusCode).Msg("Proxy validation successful")
		return nil
	}

	return fmt.Errorf("proxy %s (%s): connection test to %s returned status %d", p.Name, p.Address, targetURL, resp.StatusCode)
}