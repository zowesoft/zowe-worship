package main

import (
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"testing"

	"zowe-worship/internal/bible"
)

func TestStartupStoresRuntimeContext(t *testing.T) {
	app := newAppAt(t.TempDir())
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	app.startup(ctx)
	defer app.shutdown(context.Background())

	if app.ctx != ctx {
		t.Fatal("startup did not retain the runtime context")
	}
}

func TestStartupInitializesPersistentStorage(t *testing.T) {
	root := filepath.Join(t.TempDir(), "Zowe Worship")
	app := newAppAt(root)

	ctx := context.Background()
	app.startup(ctx)

	if app.core == nil || app.bible == nil {
		t.Fatal("startup did not open storage")
	}
	if _, err := os.Stat(filepath.Join(root, "zowe.db")); err != nil {
		t.Fatalf("core database missing: %v", err)
	}
	if _, err := os.Stat(filepath.Join(root, "translations")); err != nil {
		t.Fatalf("translations directory missing: %v", err)
	}

	installed, err := app.bible.Install(ctx, bible.TranslationData{
		TranslationMeta: bible.TranslationMeta{
			ID: "web", Name: "World English Bible", Abbreviation: "WEB", Language: "en", Version: "1.0.0",
		},
		Verses: []bible.Verse{
			{BookID: 19, Chapter: 23, Number: 1, Text: "The LORD is my shepherd; I shall not want."},
		},
	})
	if err != nil {
		t.Fatalf("install through app storage: %v", err)
	}
	if installed.VerseCount != 1 {
		t.Fatalf("verse count = %d, want 1", installed.VerseCount)
	}

	core := app.core
	app.shutdown(ctx)
	if err := core.PingContext(ctx); err == nil {
		t.Fatal("database was not closed on shutdown")
	}
	if app.core != nil {
		t.Fatal("shutdown did not release the database handle")
	}

	app.startup(ctx)
	defer app.shutdown(context.Background())
	translations, err := app.bible.ListTranslations(ctx)
	if err != nil {
		t.Fatalf("list translations after restart: %v", err)
	}
	if len(translations) != 1 || translations[0].ID != "web" {
		t.Fatalf("translations after restart = %+v", translations)
	}
}

func TestGetAppInfo(t *testing.T) {
	info := NewApp().GetAppInfo()
	if info.Name != "Zowe Worship" || info.Version != "0.1.0" {
		t.Fatalf("unexpected application info: %+v", info)
	}

	data, err := json.Marshal(info)
	if err != nil {
		t.Fatal(err)
	}
	if string(data) != `{"name":"Zowe Worship","version":"0.1.0"}` {
		t.Fatalf("unexpected bridge payload: %s", data)
	}
}
