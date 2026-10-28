package database

import (
	"context"
	"database/sql"
	"os"
	"path/filepath"
	"runtime"
	"testing"
)

func openTestDatabase(t *testing.T) (*sql.DB, string) {
	t.Helper()

	dbPath := filepath.Join(t.TempDir(), "Zowe Worship", "zowe.db")
	db, err := Open(context.Background(), dbPath)
	if err != nil {
		t.Fatalf("open database: %v", err)
	}
	t.Cleanup(func() { _ = db.Close() })
	return db, dbPath
}

func TestOpenHandlesApplicationPathWithSpaces(t *testing.T) {
	db, dbPath := openTestDatabase(t)

	if _, err := os.Stat(dbPath); err != nil {
		t.Fatalf("database file not created at %s: %v", dbPath, err)
	}

	var journalMode string
	if err := db.QueryRow(`PRAGMA journal_mode`).Scan(&journalMode); err != nil {
		t.Fatalf("read journal mode: %v", err)
	}
	if journalMode != "wal" {
		t.Fatalf("journal mode = %q, want wal", journalMode)
	}

	var foreignKeys int
	if err := db.QueryRow(`PRAGMA foreign_keys`).Scan(&foreignKeys); err != nil {
		t.Fatalf("read foreign_keys: %v", err)
	}
	if foreignKeys != 1 {
		t.Fatalf("foreign_keys = %d, want 1", foreignKeys)
	}
}

func TestMigrateIsIdempotentAndSurvivesReopen(t *testing.T) {
	ctx := context.Background()
	db, dbPath := openTestDatabase(t)

	if err := Migrate(ctx, db, ""); err != nil {
		t.Fatalf("first migrate: %v", err)
	}
	if err := Migrate(ctx, db, ""); err != nil {
		t.Fatalf("second migrate: %v", err)
	}

	var applied int
	if err := db.QueryRow(`SELECT COUNT(*) FROM schema_migrations`).Scan(&applied); err != nil {
		t.Fatalf("count migrations: %v", err)
	}
	if applied != 2 {
		t.Fatalf("applied migrations = %d, want 2", applied)
	}

	if _, err := db.Exec(`
		INSERT INTO bible_translations
			(id, name, abbreviation, language, version, filename, verse_count, checksum, installed_at)
		VALUES ('kjv', 'King James Version', 'KJV', 'en', '1.0.0', 'kjv-1.0.0.db', 31102, 'abc', '2026-10-28T00:00:00Z')`); err != nil {
		t.Fatalf("insert translation row: %v", err)
	}
	if err := db.Close(); err != nil {
		t.Fatalf("close database: %v", err)
	}

	reopened, err := Open(ctx, dbPath)
	if err != nil {
		t.Fatalf("reopen database: %v", err)
	}
	t.Cleanup(func() { _ = reopened.Close() })

	if err := Migrate(ctx, reopened, ""); err != nil {
		t.Fatalf("migrate after reopen: %v", err)
	}

	var name string
	if err := reopened.QueryRow(`SELECT name FROM bible_translations WHERE id = 'kjv'`).Scan(&name); err != nil {
		t.Fatalf("read persisted translation: %v", err)
	}
	if name != "King James Version" {
		t.Fatalf("name = %q, want King James Version", name)
	}
}

func TestBackupProducesReadableSnapshot(t *testing.T) {
	ctx := context.Background()
	db, _ := openTestDatabase(t)

	if err := Migrate(ctx, db, ""); err != nil {
		t.Fatalf("migrate: %v", err)
	}
	if _, err := db.Exec(`INSERT INTO bible_translations
		(id, name, abbreviation, language, version, filename, verse_count, checksum, installed_at)
		VALUES ('kjv', 'King James Version', 'KJV', 'en', '1.0.0', 'kjv-1.0.0.db', 31102, 'abc', '2026-10-28T00:00:00Z')`); err != nil {
		t.Fatalf("insert translation row: %v", err)
	}

	dir := filepath.Join(t.TempDir(), "backups")
	if err := backup(ctx, db, dir, 2); err != nil {
		t.Fatalf("backup: %v", err)
	}

	files, err := filepath.Glob(filepath.Join(dir, "zowe-before-v2-*.db"))
	if err != nil || len(files) != 1 {
		t.Fatalf("backup files = %v, err = %v", files, err)
	}

	snapshot, err := Open(ctx, files[0])
	if err != nil {
		t.Fatalf("open snapshot: %v", err)
	}
	t.Cleanup(func() { _ = snapshot.Close() })

	if err := CheckIntegrity(ctx, snapshot); err != nil {
		t.Fatalf("snapshot integrity: %v", err)
	}
	var name string
	if err := snapshot.QueryRow(`SELECT name FROM bible_translations WHERE id = 'kjv'`).Scan(&name); err != nil {
		t.Fatalf("read snapshot row: %v", err)
	}
	if name != "King James Version" {
		t.Fatalf("snapshot name = %q", name)
	}
}

func TestDefaultPathsStayLocal(t *testing.T) {
	if runtime.GOOS != "windows" {
		t.Skip("Windows-specific local AppData check")
	}

	base := t.TempDir()
	t.Setenv("LOCALAPPDATA", base)

	paths, err := DefaultPaths()
	if err != nil {
		t.Fatalf("default paths: %v", err)
	}

	wantRoot := filepath.Join(base, "Zowesoft", "Zowe Worship")
	if paths.Root != wantRoot {
		t.Fatalf("root = %q, want %q", paths.Root, wantRoot)
	}
	if filepath.Base(filepath.Dir(paths.Root)) == "Roaming" {
		t.Fatal("bible data must not be stored under roaming AppData")
	}
}
