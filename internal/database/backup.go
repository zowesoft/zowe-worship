package database

import (
	"context"
	"database/sql"
	"fmt"
	"os"
	"path/filepath"
	"time"
)

// backup writes a consistent snapshot of the database before a schema upgrade.
func backup(ctx context.Context, db *sql.DB, dir string, nextVersion int) error {
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return fmt.Errorf("create backup directory: %w", err)
	}

	name := fmt.Sprintf("zowe-before-v%d-%s.db", nextVersion, time.Now().UTC().Format("20060102T150405Z"))
	dest := filepath.Join(dir, name)

	if _, err := db.ExecContext(ctx, `VACUUM INTO ?`, dest); err != nil {
		return fmt.Errorf("vacuum into %s: %w", dest, err)
	}
	return nil
}

// CheckIntegrity runs SQLite's integrity check and reports any corruption.
func CheckIntegrity(ctx context.Context, db *sql.DB) error {
	var result string
	if err := db.QueryRowContext(ctx, `PRAGMA integrity_check`).Scan(&result); err != nil {
		return fmt.Errorf("run integrity check: %w", err)
	}
	if result != "ok" {
		return fmt.Errorf("integrity check failed: %s", result)
	}
	return nil
}
