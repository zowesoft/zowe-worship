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
	name := fmt.Sprintf("zowe-before-v%d-%s.db", nextVersion, time.Now().UTC().Format("20060102T150405Z"))
	return snapshot(ctx, db, dir, name)
}

// ManualBackup writes the snapshot the operator asks for from Settings. It is
// named apart from the pre-upgrade copies so a folder of backups still says
// which ones were taken on purpose.
func ManualBackup(ctx context.Context, db *sql.DB, dir string) (string, error) {
	name := fmt.Sprintf("zowe-manual-%s.db", time.Now().UTC().Format("20060102T150405Z"))
	if err := snapshot(ctx, db, dir, name); err != nil {
		return "", err
	}
	return filepath.Join(dir, name), nil
}

func snapshot(ctx context.Context, db *sql.DB, dir, name string) error {
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return fmt.Errorf("create backup directory: %w", err)
	}
	if _, err := db.ExecContext(ctx, `VACUUM INTO ?`, filepath.Join(dir, name)); err != nil {
		return fmt.Errorf("vacuum into %s: %w", name, err)
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
