package main

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"zopresent/internal/database"
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
	if !strings.HasPrefix(after.Backups[0].Name, "zopresent-manual-") {
		t.Fatalf("backup name = %q", after.Backups[0].Name)
	}
	if after.Backups[0].SizeBytes <= 0 {
		t.Fatalf("backup size = %d", after.Backups[0].SizeBytes)
	}
}

func TestBackupNeedsStorage(t *testing.T) {
	app := newAppAt(filepath.Join(t.TempDir(), "Zopresent"))
	if _, err := app.BackupNow(); !errors.Is(err, errStorageUnavailable) {
		t.Fatalf("backup without storage err = %v", err)
	}
	if _, err := app.RestoreBackup("zopresent-manual-x.db"); !errors.Is(err, errStorageUnavailable) {
		t.Fatalf("restore without storage err = %v", err)
	}
}

func TestRestoreBackupRollsDataBack(t *testing.T) {
	app := newTestApp(t)
	installTestBible(t, app)

	view, err := app.BackupNow()
	if err != nil {
		t.Fatalf("backup: %v", err)
	}
	name := view.Backups[0].Name

	// A song created after the backup disappears when the backup returns.
	if _, err := app.CreateSong(NewSongRequest{Title: "Amazing Grace", Sections: []NewSongSectionRequest{
		{Label: "Verse 1", Text: "Amazing grace, how sweet the sound"},
	}}); err != nil {
		t.Fatalf("create song: %v", err)
	}

	restored, err := app.RestoreBackup(name)
	if err != nil {
		t.Fatalf("restore: %v", err)
	}

	songs, err := app.ListSongs("")
	if err != nil {
		t.Fatalf("list songs after restore: %v", err)
	}
	if len(songs.Songs) != 0 {
		t.Fatalf("the song made after the backup survived the restore: %+v", songs.Songs)
	}

	// Translation packages live outside the core database and survive.
	translations := restored.Translations
	if len(translations) != 1 || translations[0].ID != "kjv" {
		t.Fatalf("translations after restore = %+v", translations)
	}

	// The safety copy taken ahead of the swap is in the list.
	if restored.BackupCount < 2 {
		t.Fatalf("backups after restore = %d, want the safety copy listed", restored.BackupCount)
	}
	if _, err := app.ListTranslations(); err != nil {
		t.Fatalf("storage did not reopen after restore: %v", err)
	}
	if app.core == nil || app.songs == nil {
		t.Fatal("stores were not rewired after restore")
	}
}

func TestRestoreBackupRefusesForeignFiles(t *testing.T) {
	app := newTestApp(t)
	installTestBible(t, app)

	if _, err := app.CreateSong(NewSongRequest{Title: "Kept", Sections: []NewSongSectionRequest{
		{Label: "Verse 1", Text: "Kept for verification"},
	}}); err != nil {
		t.Fatalf("create song: %v", err)
	}

	paths, err := app.paths()
	if err != nil {
		t.Fatalf("paths: %v", err)
	}

	// A file SQLite cannot open.
	junk := filepath.Join(paths.Backups(), "junk.db")
	if err := os.WriteFile(junk, []byte("this is not a database"), 0o644); err != nil {
		t.Fatalf("write junk backup: %v", err)
	}
	if _, err := app.RestoreBackup("junk.db"); err == nil || (!strings.Contains(err.Error(), "integrity") && !strings.Contains(err.Error(), "not a database")) {
		t.Fatalf("junk backup err = %v, want a refusal", err)
	}

	// A schema this build cannot read: newer than any migration it ships.
	future := filepath.Join(paths.Backups(), "future.db")
	db, err := database.Open(context.Background(), future)
	if err != nil {
		t.Fatalf("make future backup: %v", err)
	}
	if _, err := db.Exec(`CREATE TABLE schema_migrations (version INTEGER PRIMARY KEY, name TEXT NOT NULL, checksum TEXT NOT NULL, applied_at TEXT NOT NULL);
		INSERT INTO schema_migrations VALUES (99999, 'future', 'x', 'now')`); err != nil {
		t.Fatalf("seed future schema: %v", err)
	}
	if err := db.Close(); err != nil {
		t.Fatalf("close future backup: %v", err)
	}
	if _, err := app.RestoreBackup("future.db"); err == nil || !strings.Contains(err.Error(), "update Zopresent") {
		t.Fatalf("future backup err = %v, want a too-new refusal", err)
	}

	// Traversal never reaches outside the backups folder.
	if _, err := app.RestoreBackup("../zopresent.db"); err == nil {
		t.Fatal("a path outside the backups folder was accepted")
	}

	// The live database is untouched by every refusal.
	songs, err := app.ListSongs("")
	if err != nil {
		t.Fatalf("list songs: %v", err)
	}
	if len(songs.Songs) != 1 || songs.Songs[0].Title != "Kept" {
		t.Fatalf("refused restores still touched the live data: %+v", songs.Songs)
	}
}


func TestSettingsGetSetAndDefaultTranslation(t *testing.T) {
	app := newTestApp(t)

	// Fresh app reports fallback default translation "kjv"
	storage, err := app.GetStorage()
	if err != nil {
		t.Fatalf("GetStorage: %v", err)
	}
	if storage.DefaultTranslation != "kjv" {
		t.Fatalf("DefaultTranslation = %q, want 'kjv'", storage.DefaultTranslation)
	}

	// Get unset setting returns empty string without error
	val, err := app.GetSetting("custom_key")
	if err != nil {
		t.Fatalf("GetSetting: %v", err)
	}
	if val != "" {
		t.Fatalf("GetSetting = %q, want empty", val)
	}

	// Set and Get custom setting
	if err := app.SetSetting("custom_key", "custom_val"); err != nil {
		t.Fatalf("SetSetting: %v", err)
	}
	val, err = app.GetSetting("custom_key")
	if err != nil || val != "custom_val" {
		t.Fatalf("GetSetting after set = %q (err %v), want 'custom_val'", val, err)
	}

	// Update default translation
	updatedStorage, err := app.SetDefaultTranslation("esv")
	if err != nil {
		t.Fatalf("SetDefaultTranslation: %v", err)
	}
	if updatedStorage.DefaultTranslation != "esv" {
		t.Fatalf("updatedStorage.DefaultTranslation = %q, want 'esv'", updatedStorage.DefaultTranslation)
	}
}

