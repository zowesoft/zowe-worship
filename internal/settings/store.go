package settings

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
)

// Known setting keys. Using typed constants rather than raw strings keeps
// every caller consistent and makes refactoring safe.
const (
	// KeyDefaultTranslation is the translation ID shown when no other choice is
	// active, e.g. "kjv". Empty string means "use the first installed one".
	KeyDefaultTranslation = "default_translation_id"

	// KeyVersesPerSlide decides how a multi-verse passage becomes slides. "one"
	// gives every verse its own slide, which is the default because a wall-sized
	// verse is what an operator projects most often; "all" keeps the passage
	// together on one slide.
	KeyVersesPerSlide = "scripture_verses_per_slide"
)

// ErrNotFound is returned when a key has never been written.
var ErrNotFound = errors.New("setting not found")

// Store reads and writes key/value application settings from the shared
// settings table that already exists in the core database.
type Store struct {
	db *sql.DB
}

// New returns a Store backed by the given database connection.
func New(db *sql.DB) *Store {
	return &Store{db: db}
}

// Get returns the value for a key, or ErrNotFound if it has never been set.
func (s *Store) Get(ctx context.Context, key string) (string, error) {
	var value string
	err := s.db.QueryRowContext(ctx,
		`SELECT value FROM settings WHERE key = ?`, key,
	).Scan(&value)
	if errors.Is(err, sql.ErrNoRows) {
		return "", ErrNotFound
	}
	if err != nil {
		return "", fmt.Errorf("read setting %q: %w", key, err)
	}
	return value, nil
}

// GetDefault returns the stored value for key, or fallback when not set.
func (s *Store) GetDefault(ctx context.Context, key, fallback string) string {
	v, err := s.Get(ctx, key)
	if err != nil {
		return fallback
	}
	return v
}

// Set writes a setting, creating or replacing any previous value.
func (s *Store) Set(ctx context.Context, key, value string) error {
	_, err := s.db.ExecContext(ctx, `
		INSERT INTO settings (key, value) VALUES (?, ?)
		ON CONFLICT(key) DO UPDATE SET value = excluded.value`,
		key, value,
	)
	if err != nil {
		return fmt.Errorf("write setting %q: %w", key, err)
	}
	return nil
}

// Delete removes a setting. Deleting a key that does not exist is not an error.
func (s *Store) Delete(ctx context.Context, key string) error {
	_, err := s.db.ExecContext(ctx, `DELETE FROM settings WHERE key = ?`, key)
	if err != nil {
		return fmt.Errorf("delete setting %q: %w", key, err)
	}
	return nil
}
