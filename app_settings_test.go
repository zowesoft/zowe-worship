package main

import (
	"errors"
	"path/filepath"
	"strings"
	"testing"
)

func TestSettingsReportsStorageAndTakesBackups(t *testing.T) {
	app := newTestApp(t)

	view, err := app.GetStorage()
	if err != nil {
		t.Fatalf("storage view: %v", err)
	}
	if view.Version != appVersion {
		t.Fatalf("version = %q, want %q", view.Version, appVersion)
	}
	if view.DataRoot == "" {
		t.Fatal("no data folder reported")
	}
	if view.DatabaseBytes <= 0 {
		t.Fatalf("database size = %d", view.DatabaseBytes)
	}
	if view.MediaItems != 0 || view.BackupCount != 0 {
		t.Fatalf("a fresh library = %+v", view)
	}

	source := t.TempDir()
	if _, err := app.ImportMedia([]string{writeMediaFixture(t, source, "stage.png", 40, 30)}); err != nil {
		t.Fatalf("import: %v", err)
	}

	after, err := app.BackupNow()
	if err != nil {
		t.Fatalf("backup: %v", err)
	}
	if after.MediaItems != 1 || after.MediaBytes <= 0 {
		t.Fatalf("imported media not counted: %+v", after)
	}
	if after.BackupCount != 1 || len(after.Backups) != 1 {
		t.Fatalf("backups = %+v", after.Backups)
	}
	if !strings.HasPrefix(after.Backups[0].Name, "zowe-manual-") {
		t.Fatalf("backup name = %q", after.Backups[0].Name)
	}
	if after.Backups[0].SizeBytes <= 0 {
		t.Fatalf("backup size = %d", after.Backups[0].SizeBytes)
	}
}

func TestBackupNeedsStorage(t *testing.T) {
	app := newAppAt(filepath.Join(t.TempDir(), "Zowe Worship"))
	if _, err := app.BackupNow(); !errors.Is(err, errStorageUnavailable) {
		t.Fatalf("backup without storage err = %v", err)
	}
}
