package database

import (
	"context"
	"crypto/sha256"
	"database/sql"
	"embed"
	"encoding/hex"
	"fmt"
	"io/fs"
	"path"
	"sort"
	"strconv"
	"strings"
	"time"
)

//go:embed migrations/*.sql
var migrationFiles embed.FS

type migration struct {
	version  int
	name     string
	checksum string
	sql      string
}

func loadMigrations() ([]migration, error) {
	entries, err := fs.ReadDir(migrationFiles, "migrations")
	if err != nil {
		return nil, fmt.Errorf("read embedded migrations: %w", err)
	}

	migrations := make([]migration, 0, len(entries))
	for _, entry := range entries {
		filename := entry.Name()
		if entry.IsDir() || !strings.HasSuffix(filename, ".sql") {
			continue
		}

		prefix, _, ok := strings.Cut(filename, "_")
		if !ok {
			return nil, fmt.Errorf("migration %s is not named <version>_<name>.sql", filename)
		}
		version, err := strconv.Atoi(prefix)
		if err != nil || version <= 0 {
			return nil, fmt.Errorf("migration %s has an invalid version prefix", filename)
		}

		content, err := fs.ReadFile(migrationFiles, path.Join("migrations", filename))
		if err != nil {
			return nil, fmt.Errorf("read migration %s: %w", filename, err)
		}
		// Git can hand the same migration over with CRLF endings, which would
		// otherwise look like someone edited applied history.
		normalised := normalizeSQL(content)

		migrations = append(migrations, migration{
			version:  version,
			name:     strings.TrimSuffix(filename, ".sql"),
			checksum: migrationChecksum(normalised),
			sql:      normalised,
		})
	}

	sort.Slice(migrations, func(i, j int) bool {
		return migrations[i].version < migrations[j].version
	})
	for i := 1; i < len(migrations); i++ {
		if migrations[i].version == migrations[i-1].version {
			return nil, fmt.Errorf("duplicate migration version %d", migrations[i].version)
		}
	}
	return migrations, nil
}

// normalizeSQL drops the CRLF endings Git can hand back for the same migration,
// so a checkout on Windows is not mistaken for someone editing applied history.
func normalizeSQL(content []byte) string {
	return strings.ReplaceAll(string(content), "\r\n", "\n")
}

func migrationChecksum(statements string) string {
	sum := sha256.Sum256([]byte(statements))
	return hex.EncodeToString(sum[:])
}

// SchemaAheadError says the database carries applied migrations this build does
// not know, so writing to it could damage a schema this build cannot read.
type SchemaAheadError struct {
	Highest int
	Known   int
}

func (e *SchemaAheadError) Error() string {
	if e.Highest > e.Known {
		return fmt.Sprintf("the database was last written by a newer Zopresent: schema version %d, this build knows %d", e.Highest, e.Known)
	}
	return fmt.Sprintf("the database records applied migration %d, which this build does not know", e.Highest)
}

// Migrate applies pending migrations in order. Each migration is its own
// transaction, so a failed upgrade leaves the schema at the last version that
// completed. Applied migrations are checksummed to catch edited history, and a
// database from a newer build is refused rather than opened.
func Migrate(ctx context.Context, db *sql.DB, backupDir string) error {
	migrations, err := loadMigrations()
	if err != nil {
		return err
	}

	if _, err := db.ExecContext(ctx, `
		CREATE TABLE IF NOT EXISTS schema_migrations (
			version    INTEGER PRIMARY KEY,
			name       TEXT NOT NULL,
			checksum   TEXT NOT NULL,
			applied_at TEXT NOT NULL
		)`); err != nil {
		return fmt.Errorf("create migration table: %w", err)
	}

	applied, err := appliedMigrations(ctx, db)
	if err != nil {
		return err
	}

	known := make(map[int]bool, len(migrations))
	highest := 0
	for _, m := range migrations {
		known[m.version] = true
		if m.version > highest {
			highest = m.version
		}
	}
	ahead := 0
	for version := range applied {
		if !known[version] && version > ahead {
			ahead = version
		}
	}
	if ahead > 0 {
		return &SchemaAheadError{Highest: ahead, Known: highest}
	}

	var pending []migration
	for _, m := range migrations {
		recorded, ok := applied[m.version]
		if !ok {
			pending = append(pending, m)
			continue
		}
		if recorded != m.checksum {
			return fmt.Errorf("migration %d was modified after it was applied", m.version)
		}
	}

	if len(pending) == 0 {
		return nil
	}

	if len(applied) > 0 && backupDir != "" {
		if err := backup(ctx, db, backupDir, pending[0].version); err != nil {
			return fmt.Errorf("back up database before migration: %w", err)
		}
	}

	for _, m := range pending {
		tx, err := db.BeginTx(ctx, nil)
		if err != nil {
			return fmt.Errorf("begin migration %d: %w", m.version, err)
		}

		if _, err := tx.ExecContext(ctx, m.sql); err != nil {
			_ = tx.Rollback()
			return fmt.Errorf("apply migration %d (%s): %w", m.version, m.name, err)
		}
		if _, err := tx.ExecContext(ctx,
			`INSERT INTO schema_migrations (version, name, checksum, applied_at) VALUES (?, ?, ?, ?)`,
			m.version, m.name, m.checksum, time.Now().UTC().Format(time.RFC3339),
		); err != nil {
			_ = tx.Rollback()
			return fmt.Errorf("record migration %d: %w", m.version, err)
		}
		if err := tx.Commit(); err != nil {
			return fmt.Errorf("commit migration %d: %w", m.version, err)
		}
	}
	return nil
}

func appliedMigrations(ctx context.Context, db *sql.DB) (map[int]string, error) {
	rows, err := db.QueryContext(ctx, `SELECT version, checksum FROM schema_migrations`)
	if err != nil {
		return nil, fmt.Errorf("read applied migrations: %w", err)
	}
	defer rows.Close()

	applied := make(map[int]string)
	for rows.Next() {
		var version int
		var checksum string
		if err := rows.Scan(&version, &checksum); err != nil {
			return nil, fmt.Errorf("scan applied migration: %w", err)
		}
		applied[version] = checksum
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate applied migrations: %w", err)
	}
	return applied, nil
}
