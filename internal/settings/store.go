package settings

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
)

const (
	// KeyDefaultTranslation is the translation ID shown when no other choice is
	// active, e.g. "kjv". Empty string means "use the first installed one".
	KeyDefaultTranslation = "default_translation_id"

	// KeyVersesPerSlide decides how a multi-verse passage becomes slides. "one"
	// gives every verse its own slide, which is the default because a wall-sized
	// verse is what an operator projects most often; "all" keeps the passage
	// together on one slide.
	KeyVersesPerSlide = "scripture_verses_per_slide"

	// KeyAsrEngine is the engine spec Live Detection transcribes with, in the
	// same "name=bin args {audio}" form the benchmark harness parses. Empty
	// means no engine is configured yet and detection reports that instead of
	// guessing an executable.
	KeyAsrEngine = "asr_engine_spec"

	// KeyDetectionDisabled is the F12 emergency switch: "true" stops new
	// detections from being processed. It never touches the audience display —
	// only the operator decides what is on air.
	KeyDetectionDisabled = "detection_disabled"

	// KeyShortcuts holds the operator key map as one JSON blob: a combo per
	// action in the canonical "Ctrl+Alt+Shift+Key" form the frontend builds
	// from a KeyboardEvent. Empty means the built-in defaults are in use.
	KeyShortcuts = "operator_shortcuts"
)

var ErrNotFound = errors.New("setting not found")

// Store reads and writes key/value settings in the core database's shared
// settings table.
type Store struct {
	db *sql.DB
}

func New(db *sql.DB) *Store {
	return &Store{db: db}
}

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

func (s *Store) GetDefault(ctx context.Context, key, fallback string) string {
	v, err := s.Get(ctx, key)
	if err != nil {
		return fallback
	}
	return v
}

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

// Deleting a missing key is not an error.
func (s *Store) Delete(ctx context.Context, key string) error {
	_, err := s.db.ExecContext(ctx, `DELETE FROM settings WHERE key = ?`, key)
	if err != nil {
		return fmt.Errorf("delete setting %q: %w", key, err)
	}
	return nil
}
