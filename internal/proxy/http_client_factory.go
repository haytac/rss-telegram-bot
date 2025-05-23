package proxy

import (
	// "context" // Only if GetClient takes context
	"fmt"
	"net"
	"net/http"
	"net/url"
	"time"

	"github.com/haytac/rss-telegram-bot/internal/database"
	"golang.org/x/net/proxy" // For SOCKS5
)

// DefaultHTTPClientFactory is a basic HTTP client factory.
type DefaultHTTPClientFactory struct {
	// proxyStore *database.ProxyStore // If needed to fetch default proxies
}

// NewHTTPClientFactory creates a new DefaultHTTPClientFactory.
func NewHTTPClientFactory(/*proxyStore *database.ProxyStore*/) *DefaultHTTPClientFactory {
	return &DefaultHTTPClientFactory{/*proxyStore: proxyStore*/}
}

// GetClient returns an HTTP client, configured with the given proxy if provided.
// If proxy is nil, it returns a default HTTP client.
func (f *DefaultHTTPClientFactory) GetClient(p *database.Proxy) (*http.Client, error) {
	transport := &http.Transport{
		Proxy: http.ProxyFromEnvironment, // Default behavior
		DialContext: (&net.Dialer{
			Timeout:   30 * time.Second,
			KeepAlive: 30 * time.Second,
		}).DialContext,
		ForceAttemptHTTP2:     true,
		MaxIdleConns:          100,
		IdleConnTimeout:       90 * time.Second,
		TLSHandshakeTimeout:   10 * time.Second,
		ExpectContinueTimeout: 1 * time.Second,
	}

	if p != nil && p.Address != "" {
		proxyURLStr := fmt.Sprintf("%s://%s", p.Type, p.Address)
		if p.Username != nil && *p.Username != "" && p.Password != nil {
			// Add auth to the URL for http/https proxies if user:pass@host:port format is not already in Address
			// This depends on how p.Address is stored. If it's just host:port, construct full URL here.
			// Assuming p.Address is host:port for now.
			userInfo := url.UserPassword(*p.Username, *p.Password)
			parsedProxyURL, err := url.Parse(proxyURLStr)
			if err != nil {
				return nil, fmt.Errorf("invalid base proxy URL %s: %w", proxyURLStr, err)
			}
			parsedProxyURL.User = userInfo
			proxyURLStr = parsedProxyURL.String()
		}
		
		proxyURL, err := url.Parse(proxyURLStr)
		if err != nil {
			return nil, fmt.Errorf("failed to parse proxy URL %s: %w", proxyURLStr, err)
		}

		switch p.Type {
		case "http", "https":
			transport.Proxy = http.ProxyURL(proxyURL)
		case "socks5":
			dialer, err := proxy.FromURL(proxyURL, proxy.Direct) // proxy.Direct is the forward dialer
			if err != nil {
				return nil, fmt.Errorf("failed to create SOCKS5 dialer from %s: %w", proxyURLStr, err)
			}
			// Ensure the dialer is an http.Dialer for transport.DialContext
			contextDialer, ok := dialer.(proxy.ContextDialer)
			if !ok {
				return nil, fmt.Errorf("SOCKS5 dialer does not implement proxy.ContextDialer")
			}
			transport.DialContext = contextDialer.DialContext
			transport.Proxy = nil // SOCKS5 is handled by the custom dialer
		default:
			return nil, fmt.Errorf("unsupported proxy type: %s", p.Type)
		}
	}

	return &http.Client{
		Transport: transport,
		Timeout:   60 * time.Second, // Overall request timeout
	}, nil
}