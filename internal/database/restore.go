package database

import (
	"context"
	"fmt"
	"io"
	"os"
)

// InspectBackup opens a backup read-only and proves it is restorable: the
// integrity check must pass and it must record a schema version this build's
// migrations can reach.
func InspectBackup(ctx context.Context, path string) (int, error) {
	db, err := OpenReadOnly(ctx, path)
	if err != nil {
		return 0, fmt.Errorf("open backup: %w", err)
	}
	defer db.Close()

	if err := CheckIntegrity(ctx, db); err != nil {
		return 0, fmt.Errorf("backup %s: %w", path, err)
	}

	var count int
	if err := db.QueryRowContext(ctx,
		`SELECT COUNT(*) FROM schema_migrations`).Scan(&count); err != nil {
		return 0, fmt.Errorf("backup %s does not record its schema: %w", path, err)
	}
	if count == 0 {
		return 0, fmt.Errorf("backup %s has no schema recorded", path)
	}

	var version int
	if err := db.QueryRowContext(ctx,
		`SELECT MAX(version) FROM schema_migrations`).Scan(&version); err != nil {
		return 0, fmt.Errorf("read backup schema version: %w", err)
	}
	return version, nil
}

// LatestMigrationVersion is where this build's schema stops, so a backup from a
// newer app is refused instead of half-restored.
func LatestMigrationVersion() (int, error) {
	migrations, err := loadMigrations()
	if err != nil {
		return 0, err
	}
	return migrations[len(migrations)-1].version, nil
}

// ReplaceDatabase swaps the database at live for the replacement, keeping the
// old files for a rollback. The caller must have closed every connection to
// live first: a clean close leaves the sidecars gone, but any crash leftovers
// move aside with the old file — they belong to it, not to the replacement.
//
// On failure RollbackDatabase undoes the swap; on success DiscardDatabase
// deletes the kept files.
func ReplaceDatabase(live, replacement, keep string) error {
	// The old files move as a group: the database and any WAL sidecars that a
	// crash left beside it all describe the state the keep name will hold.
	for _, from := range sidecars(live) {
		to := keep + from[len(live):]
		if err := os.Rename(from, to); err != nil && !os.IsNotExist(err) {
			return fmt.Errorf("set %s aside: %w", from, err)
		}
	}
	if err := os.Rename(live, keep); err != nil {
		_ = unsidecars(keep)
		return fmt.Errorf("set the live database aside: %w", err)
	}
	if err := copyFile(live, replacement); err != nil {
		return fmt.Errorf("copy the backup in: %w", err)
	}
	return nil
}

func RollbackDatabase(live, keep string) error {
	if err := os.Remove(live); err != nil && !os.IsNotExist(err) {
		return fmt.Errorf("remove the failed copy: %w", err)
	}
	if err := unsidecars(live); err != nil {
		return err
	}
	for _, from := range sidecars(keep) {
		to := live + from[len(keep):]
		if err := os.Rename(from, to); err != nil && !os.IsNotExist(err) {
			return fmt.Errorf("return %s: %w", from, err)
		}
	}
	if err := os.Rename(keep, live); err != nil {
		return fmt.Errorf("return the live database: %w", err)
	}
	return nil
}

func DiscardDatabase(keep string) {
	_ = os.Remove(keep)
	_ = unsidecars(keep)
}

// sidecars lists the WAL journal files that can sit beside a database.
func sidecars(live string) []string {
	return []string{live + "-wal", live + "-shm"}
}

// unsidecars removes stray sidecars; a database being installed never
// inherits another database's journal.
func unsidecars(path string) error {
	for _, side := range sidecars(path) {
		if err := os.Remove(side); err != nil && !os.IsNotExist(err) {
			return fmt.Errorf("remove %s: %w", side, err)
		}
	}
	return nil
}

// copyFile replaces destination with source byte for byte, flushed to disk.
func copyFile(destination, source string) error {
	in, err := os.Open(source)
	if err != nil {
		return err
	}
	defer in.Close()

	out, err := os.Create(destination)
	if err != nil {
		return err
	}
	if _, err := io.Copy(out, in); err != nil {
		_ = out.Close()
		return err
	}
	if err := out.Sync(); err != nil {
		_ = out.Close()
		return err
	}
	return out.Close()
}

// CheckRestoreCandidate refuses a backup that will not open or carries a
// schema newer than this build; older backups are fine, Migrate brings
// them forward.
func CheckRestoreCandidate(ctx context.Context, backup string) error {
	version, err := InspectBackup(ctx, backup)
	if err != nil {
		return err
	}
	latest, err := LatestMigrationVersion()
	if err != nil {
		return err
	}
	if version > latest {
		return fmt.Errorf("backup schema v%d is newer than this build (v%d); update Zowe Worship first",
			version, latest)
	}
	return nil
}
