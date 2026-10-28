package database

import (
	"context"
	"database/sql"
	"fmt"
	"net/url"
	"os"
	"path/filepath"
	"strings"

	_ "modernc.org/sqlite"
)

// Open opens the core database with the pragmas the application relies on.
// Every connection gets them because they travel in the DSN.
func Open(ctx context.Context, path string) (*sql.DB, error) {
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return nil, fmt.Errorf("create database directory: %w", err)
	}

	db, err := sql.Open("sqlite", dsn(path, url.Values{
		"_pragma": {"foreign_keys(1)", "busy_timeout(5000)", "journal_mode(WAL)", "synchronous(NORMAL)"},
	}))
	if err != nil {
		return nil, fmt.Errorf("open database: %w", err)
	}

	db.SetMaxOpenConns(1)
	db.SetMaxIdleConns(1)

	if err := db.PingContext(ctx); err != nil {
		_ = db.Close()
		return nil, fmt.Errorf("connect to database: %w", err)
	}
	return db, nil
}

// OpenReadOnly is used for translation packages, which are never written after install.
func OpenReadOnly(ctx context.Context, path string) (*sql.DB, error) {
	return openWith(ctx, path, url.Values{
		"mode":    {"ro"},
		"_pragma": {"busy_timeout(5000)"},
	})
}

// OpenForImport builds a database file from scratch. It avoids WAL so a
// finished package is a single self-contained file.
func OpenForImport(ctx context.Context, path string) (*sql.DB, error) {
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return nil, fmt.Errorf("create package directory: %w", err)
	}

	return openWith(ctx, path, url.Values{
		"_pragma": {"foreign_keys(1)", "busy_timeout(5000)", "journal_mode(DELETE)", "synchronous(FULL)"},
	})
}

func openWith(ctx context.Context, path string, params url.Values) (*sql.DB, error) {
	db, err := sql.Open("sqlite", dsn(path, params))
	if err != nil {
		return nil, fmt.Errorf("open database: %w", err)
	}

	db.SetMaxOpenConns(1)
	db.SetMaxIdleConns(1)

	if err := db.PingContext(ctx); err != nil {
		_ = db.Close()
		return nil, fmt.Errorf("connect to database: %w", err)
	}
	return db, nil
}

// dsn builds a SQLite file URI. The path is escaped because the per-user
// application directory can contain spaces on Windows and macOS.
func dsn(path string, params url.Values) string {
	slashPath := filepath.ToSlash(path)
	if !strings.HasPrefix(slashPath, "/") {
		slashPath = "/" + slashPath
	}
	return (&url.URL{
		Scheme:   "file",
		Path:     slashPath,
		RawQuery: params.Encode(),
	}).String()
}
