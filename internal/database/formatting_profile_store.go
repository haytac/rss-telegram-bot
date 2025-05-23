package database

import (
	"context"
	"database/sql"
	"fmt"
)

// FormattingProfileStore provides methods for formatting profiles.
type FormattingProfileStore struct {
	db *DB
}

// NewFormattingProfileStore creates a new FormattingProfileStore.
func NewFormattingProfileStore(db *DB) *FormattingProfileStore {
	return &FormattingProfileStore{db: db}
}

// CreateProfile adds a new formatting profile.
func (s *FormattingProfileStore) CreateProfile(ctx context.Context, p *FormattingProfile) (int64, error) {
	if err := p.MarshalConfig(); err != nil { // Ensure ConfigJSON is up-to-date
		return 0, fmt.Errorf("CreateProfile marshal config: %w", err)
	}
	stmt, err := s.db.PrepareContext(ctx, `
		INSERT INTO formatting_profiles (name, template_config) VALUES (?, ?)`)
	if err != nil {
		return 0, fmt.Errorf("CreateProfile prepare: %w", err)
	}
	defer stmt.Close()

	res, err := stmt.ExecContext(ctx, p.Name, p.ConfigJSON)
	if err != nil {
		return 0, fmt.Errorf("CreateProfile exec: %w", err)
	}
	return res.LastInsertId()
}

// GetProfileByID retrieves a formatting profile by ID.
func (s *FormattingProfileStore) GetProfileByID(ctx context.Context, id int64) (*FormattingProfile, error) {
	query := `SELECT id, name, template_config, created_at, updated_at FROM formatting_profiles WHERE id = ?`
	row := s.db.QueryRowContext(ctx, query, id)
	p := &FormattingProfile{}
	err := row.Scan(&p.ID, &p.Name, &p.ConfigJSON, &p.CreatedAt, &p.UpdatedAt)
	if err != nil {
		if err == sql.ErrNoRows {
			return nil, nil
		}
		return nil, fmt.Errorf("GetProfileByID scan: %w", err)
	}
	if err := p.UnmarshalConfig(); err != nil { // Parse JSON into struct
		return nil, fmt.Errorf("GetProfileByID unmarshal config for profile %d: %w", p.ID, err)
	}
	return p, nil
}

// ListProfiles retrieves all formatting profiles.
func (s *FormattingProfileStore) ListProfiles(ctx context.Context) ([]*FormattingProfile, error) {
	query := `SELECT id, name, template_config, created_at, updated_at FROM formatting_profiles ORDER BY name`
	rows, err := s.db.QueryContext(ctx, query)
	if err != nil {
		return nil, fmt.Errorf("ListProfiles query: %w", err)
	}
	defer rows.Close()

	var profiles []*FormattingProfile
	for rows.Next() {
		p := &FormattingProfile{}
		err := rows.Scan(&p.ID, &p.Name, &p.ConfigJSON, &p.CreatedAt, &p.UpdatedAt)
		if err != nil {
			return nil, fmt.Errorf("ListProfiles scan: %w", err)
		}
		if err := p.UnmarshalConfig(); err != nil {
			// Log error but continue, or return error immediately?
			// For a list, maybe log and skip problematic ones. For now, return error.
			return nil, fmt.Errorf("ListProfiles unmarshal config for profile %s (ID %d): %w", p.Name, p.ID, err)
		}
		profiles = append(profiles, p)
	}
	if err = rows.Err(); err != nil {
		return nil, fmt.Errorf("ListProfiles rows error: %w", err)
	}
	return profiles, nil
}