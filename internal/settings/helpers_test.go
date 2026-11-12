package settings

import (
	"context"
	"database/sql"
	"path/filepath"
	"testing"

	"zopresent/internal/database"
)

func openTestDB(t *testing.T) *sql.DB {
	t.Helper()
	root := filepath.Join(t.TempDir(), "Zopresent")
	paths := database.Paths{Root: root}
	if err := paths.Ensure(); err != nil {
		t.Fatalf("prepare directories: %v", err)
	}
	db, err := database.Open(context.Background(), paths.CoreDatabase())
	if err != nil {
		t.Fatalf("open database: %v", err)
	}
	if err := database.Migrate(context.Background(), db, paths.Backups()); err != nil {
		t.Fatalf("migrate: %v", err)
	}
	t.Cleanup(func() { _ = db.Close() })
	return db
}
