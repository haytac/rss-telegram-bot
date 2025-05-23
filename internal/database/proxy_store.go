package database

import (
	"context"
	"database/sql"
	"fmt"
)

// ProxyStore provides methods to interact with proxy configurations.
type ProxyStore struct {
	db *DB
}

// NewProxyStore creates a new ProxyStore.
func NewProxyStore(db *DB) *ProxyStore {
	return &ProxyStore{db: db}
}

// CreateProxy adds a new proxy.
func (s *ProxyStore) CreateProxy(ctx context.Context, p *Proxy) (int64, error) {
	stmt, err := s.db.PrepareContext(ctx, `
		INSERT INTO proxies (name, type, address, username, password, is_default_for_rss, is_default_for_telegram)
		VALUES (?, ?, ?, ?, ?, ?, ?)`)
	if err != nil {
		return 0, fmt.Errorf("CreateProxy prepare: %w", err)
	}
	defer stmt.Close()

	res, err := stmt.ExecContext(ctx, p.Name, p.Type, p.Address, p.Username, p.Password, p.IsDefaultForRSS, p.IsDefaultForTelegram)
	if err != nil {
		return 0, fmt.Errorf("CreateProxy exec: %w", err)
	}
	return res.LastInsertId()
}

// GetProxyByID retrieves a proxy by its ID.
func (s *ProxyStore) GetProxyByID(ctx context.Context, id int64) (*Proxy, error) {
	query := `SELECT id, name, type, address, username, password, is_default_for_rss, is_default_for_telegram, created_at, updated_at FROM proxies WHERE id = ?`
	row := s.db.QueryRowContext(ctx, query, id)
	p := &Proxy{}
	err := row.Scan(&p.ID, &p.Name, &p.Type, &p.Address, &p.Username, &p.Password, &p.IsDefaultForRSS, &p.IsDefaultForTelegram, &p.CreatedAt, &p.UpdatedAt)
	if err != nil {
		if err == sql.ErrNoRows {
			return nil, nil // Or a custom ErrNotFound
		}
		return nil, fmt.Errorf("GetProxyByID scan: %w", err)
	}
	return p, nil
}

// GetDefaultProxy retrieves the default proxy for a given type (rss or telegram).
func (s *ProxyStore) GetDefaultProxy(ctx context.Context, forType string) (*Proxy, error) {
	var query string
	switch forType {
	case "rss":
		query = `SELECT id, name, type, address, username, password, is_default_for_rss, is_default_for_telegram, created_at, updated_at FROM proxies WHERE is_default_for_rss = TRUE LIMIT 1`
	case "telegram":
		query = `SELECT id, name, type, address, username, password, is_default_for_rss, is_default_for_telegram, created_at, updated_at FROM proxies WHERE is_default_for_telegram = TRUE LIMIT 1`
	default:
		return nil, fmt.Errorf("invalid default proxy type: %s", forType)
	}
	
	row := s.db.QueryRowContext(ctx, query)
	p := &Proxy{}
	err := row.Scan(&p.ID, &p.Name, &p.Type, &p.Address, &p.Username, &p.Password, &p.IsDefaultForRSS, &p.IsDefaultForTelegram, &p.CreatedAt, &p.UpdatedAt)
	if err != nil {
		if err == sql.ErrNoRows {
			return nil, nil 
		}
		return nil, fmt.Errorf("GetDefaultProxy for %s scan: %w", forType, err)
	}
	return p, nil
}


// ListProxies retrieves all proxies.
func (s *ProxyStore) ListProxies(ctx context.Context) ([]*Proxy, error) {
	query := `SELECT id, name, type, address, username, password, is_default_for_rss, is_default_for_telegram, created_at, updated_at FROM proxies ORDER BY name`
	rows, err := s.db.QueryContext(ctx, query)
	if err != nil {
		return nil, fmt.Errorf("ListProxies query: %w", err)
	}
	defer rows.Close()

	var proxies []*Proxy
	for rows.Next() {
		p := &Proxy{}
		err := rows.Scan(&p.ID, &p.Name, &p.Type, &p.Address, &p.Username, &p.Password, &p.IsDefaultForRSS, &p.IsDefaultForTelegram, &p.CreatedAt, &p.UpdatedAt)
		if err != nil {
			return nil, fmt.Errorf("ListProxies scan: %w", err)
		}
		proxies = append(proxies, p)
	}
	if err = rows.Err(); err != nil {
		return nil, fmt.Errorf("ListProxies rows error: %w", err)
	}
	return proxies, nil
}

// UpdateProxy updates an existing proxy. (Implement as needed)
// DeleteProxy deletes a proxy. (Implement as needed)