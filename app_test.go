package main

import (
	"context"
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"zowe-worship/internal/bible"
)

func newTestApp(t *testing.T) *App {
	t.Helper()

	app := newAppAt(filepath.Join(t.TempDir(), "Zowe Worship"))
	app.startup(context.Background())
	t.Cleanup(func() { app.shutdown(context.Background()) })
	if app.bible == nil {
		t.Fatal("startup did not initialize storage")
	}
	return app
}

func installTestBible(t *testing.T, app *App) {
	t.Helper()

	_, err := app.bible.Install(context.Background(), bible.TranslationData{
		TranslationMeta: bible.TranslationMeta{
			ID: "kjv", Name: "King James Version", Abbreviation: "KJV", Language: "en", Version: "1.0.0",
		},
		Verses: []bible.Verse{
			{BookID: 19, Chapter: 23, Number: 1, Text: "The LORD is my shepherd; I shall not want."},
			{BookID: 19, Chapter: 23, Number: 2, Text: "He maketh me to lie down in green pastures."},
			{BookID: 43, Chapter: 3, Number: 16, Text: "For God so loved the world, that he gave his only begotten Son."},
			{BookID: 43, Chapter: 4, Number: 24, Text: "God is a Spirit: and they that worship him must worship him in spirit and in truth."},
			{BookID: 58, Chapter: 11, Number: 1, Text: "Now faith is the substance of things hoped for."},
		},
	})
	if err != nil {
		t.Fatalf("install test bible: %v", err)
	}
}

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

func TestBibleBindingsRequireStorage(t *testing.T) {
	app := newAppAt(t.TempDir())

	if _, err := app.ListTranslations(); !errors.Is(err, errStorageUnavailable) {
		t.Fatalf("ListTranslations err = %v, want errStorageUnavailable", err)
	}
	if _, err := app.SearchBible(BibleSearchRequest{TranslationID: "kjv", Query: "grace"}); !errors.Is(err, errStorageUnavailable) {
		t.Fatalf("SearchBible err = %v, want errStorageUnavailable", err)
	}
}

func TestListTranslationsBinding(t *testing.T) {
	app := newTestApp(t)
	installTestBible(t, app)

	views, err := app.ListTranslations()
	if err != nil {
		t.Fatalf("list translations: %v", err)
	}
	if len(views) != 1 {
		t.Fatalf("translations = %d, want 1", len(views))
	}
	view := views[0]
	if view.ID != "kjv" || view.Abbreviation != "KJV" || view.VerseCount != 5 {
		t.Fatalf("unexpected translation view: %+v", view)
	}
}

func TestSearchBibleResolvesReferencesBeforeText(t *testing.T) {
	app := newTestApp(t)
	installTestBible(t, app)

	reference, err := app.SearchBible(BibleSearchRequest{TranslationID: "kjv", Query: "John 3:16"})
	if err != nil {
		t.Fatalf("reference search: %v", err)
	}
	if reference.Kind != "reference" || reference.Reference != "John 3:16" {
		t.Fatalf("unexpected reference response: %+v", reference)
	}
	if len(reference.Results) != 1 || !strings.Contains(reference.Results[0].Text, "God so loved") {
		t.Fatalf("reference results = %+v", reference.Results)
	}
	if reference.Results[0].Book != "John" || reference.Results[0].Verse != 16 {
		t.Fatalf("verse identity = %+v", reference.Results[0])
	}

	chapter, err := app.SearchBible(BibleSearchRequest{TranslationID: "kjv", Query: "Psalm 23"})
	if err != nil {
		t.Fatalf("chapter search: %v", err)
	}
	if chapter.Kind != "reference" || chapter.Reference != "Psalms 23" || len(chapter.Results) != 2 {
		t.Fatalf("unexpected chapter response: %+v", chapter)
	}

	text, err := app.SearchBible(BibleSearchRequest{TranslationID: "kjv", Query: "shepherd"})
	if err != nil {
		t.Fatalf("text search: %v", err)
	}
	if text.Kind != "text" || len(text.Results) != 1 || text.Results[0].Reference != "Psalms 23:1" {
		t.Fatalf("unexpected text response: %+v", text)
	}
}

func TestSearchBibleExplicitModes(t *testing.T) {
	app := newTestApp(t)
	installTestBible(t, app)

	phrase, err := app.SearchBible(BibleSearchRequest{
		TranslationID: "kjv", Query: "god is a spirit", Mode: "phrase",
	})
	if err != nil {
		t.Fatalf("phrase search: %v", err)
	}
	if len(phrase.Results) != 1 || phrase.Results[0].Reference != "John 4:24" {
		t.Fatalf("phrase results = %+v", phrase.Results)
	}

	prefix, err := app.SearchBible(BibleSearchRequest{
		TranslationID: "kjv", Query: "shep", Mode: "prefix",
	})
	if err != nil {
		t.Fatalf("prefix search: %v", err)
	}
	if len(prefix.Results) != 1 || prefix.Results[0].Reference != "Psalms 23:1" {
		t.Fatalf("prefix results = %+v", prefix.Results)
	}

	if _, err := app.SearchBible(BibleSearchRequest{
		TranslationID: "kjv", Query: "grace", Mode: "fuzzy",
	}); err == nil {
		t.Fatal("unknown mode was accepted")
	}
}

func TestSearchBibleReportsBadReferences(t *testing.T) {
	app := newTestApp(t)
	installTestBible(t, app)

	if _, err := app.SearchBible(BibleSearchRequest{
		TranslationID: "kjv", Query: "Romans 8:28-30",
	}); err == nil || !strings.Contains(err.Error(), "was not found") {
		t.Fatalf("missing reference err = %v", err)
	}

	if _, err := app.SearchBible(BibleSearchRequest{
		TranslationID: "kjv", Query: "John 0:1",
	}); err == nil || !strings.Contains(err.Error(), "invalid scripture reference") {
		t.Fatalf("invalid reference err = %v", err)
	}

	if _, err := app.SearchBible(BibleSearchRequest{
		TranslationID: "niv", Query: "John 3:16",
	}); !errors.Is(err, bible.ErrTranslationNotFound) {
		t.Fatalf("unknown translation err = %v", err)
	}
}

func TestSearchBibleValidation(t *testing.T) {
	app := newTestApp(t)
	installTestBible(t, app)

	if _, err := app.SearchBible(BibleSearchRequest{Query: "grace"}); err == nil {
		t.Fatal("empty translation was accepted")
	}
	if _, err := app.SearchBible(BibleSearchRequest{TranslationID: "kjv", Query: "  "}); err == nil {
		t.Fatal("empty query was accepted")
	}
}

func TestServiceOrderAndPresentationFlow(t *testing.T) {
	app := newTestApp(t)
	installTestBible(t, app)

	order, err := app.GetServiceOrder()
	if err != nil {
		t.Fatalf("initial service order: %v", err)
	}
	if order.Service != nil || len(order.Items) != 0 {
		t.Fatalf("expected empty order, got %+v", order)
	}

	if _, err := app.GetPresentationState(); err != nil {
		t.Fatalf("initial presentation state: %v", err)
	}

	order, err = app.AddScriptureToService(ScriptureRequest{
		TranslationID: "kjv", BookID: 43, Chapter: 3, VerseStart: 16,
	})
	if err != nil {
		t.Fatalf("add first verse: %v", err)
	}
	if order.Service == nil || order.Service.Name != "Untitled service" {
		t.Fatalf("service = %+v", order.Service)
	}
	if len(order.Items) != 1 || order.Items[0].Title != "John 3:16" || order.Items[0].Position != 1 {
		t.Fatalf("items = %+v", order.Items)
	}

	order, err = app.AddScriptureToService(ScriptureRequest{
		TranslationID: "kjv", BookID: 19, Chapter: 23, VerseStart: 1,
	})
	if err != nil {
		t.Fatalf("add second verse: %v", err)
	}
	if len(order.Items) != 2 || order.Items[1].Title != "Psalms 23:1" {
		t.Fatalf("items = %+v", order.Items)
	}

	if _, err := app.AddScriptureToService(ScriptureRequest{
		TranslationID: "kjv", BookID: 43, Chapter: 9, VerseStart: 1,
	}); err == nil {
		t.Fatal("missing verse was accepted")
	}

	state, err := app.PreviewServiceItem(order.Items[0].ID)
	if err != nil {
		t.Fatalf("preview item: %v", err)
	}
	if state.Preview == nil || state.Live != nil {
		t.Fatalf("state after preview = %+v", state)
	}
	if state.Preview.Reference != "John 3:16" || state.Preview.Translation != "KJV" {
		t.Fatalf("preview = %+v", state.Preview)
	}

	state, err = app.SendPreviewLive()
	if err != nil {
		t.Fatalf("send live: %v", err)
	}
	if state.Live == nil || state.Live.Reference != "John 3:16" {
		t.Fatalf("live = %+v", state.Live)
	}

	state, err = app.NextServiceItem()
	if err != nil {
		t.Fatalf("next item: %v", err)
	}
	if state.Preview.ItemID != order.Items[1].ID {
		t.Fatalf("preview after next = %+v", state.Preview)
	}
	if state.Live.Reference != "John 3:16" {
		t.Fatalf("next changed live output: %+v", state.Live)
	}

	state, err = app.ClearLive()
	if err != nil {
		t.Fatalf("clear live: %v", err)
	}
	if state.Live != nil {
		t.Fatalf("live after clear = %+v", state.Live)
	}

	order, err = app.RemoveServiceItem(order.Items[0].ID)
	if err != nil {
		t.Fatalf("remove item: %v", err)
	}
	if len(order.Items) != 1 || order.Items[0].Title != "Psalms 23:1" {
		t.Fatalf("items after removal = %+v", order.Items)
	}

	order, err = app.CreateService("Sunday Service")
	if err != nil {
		t.Fatalf("create service: %v", err)
	}
	if order.Service == nil || order.Service.Name != "Sunday Service" {
		t.Fatalf("service = %+v", order.Service)
	}
	if len(order.Items) != 0 {
		t.Fatalf("new service should start empty: %+v", order.Items)
	}
}

func TestSchedulePersistsAcrossRestart(t *testing.T) {
	root := filepath.Join(t.TempDir(), "Zowe Worship")

	first := newAppAt(root)
	first.startup(context.Background())
	installTestBible(t, first)
	order, err := first.AddScriptureToService(ScriptureRequest{
		TranslationID: "kjv", BookID: 43, Chapter: 3, VerseStart: 16,
	})
	if err != nil {
		t.Fatalf("add verse: %v", err)
	}
	if _, err := first.PreviewServiceItem(order.Items[0].ID); err != nil {
		t.Fatalf("preview: %v", err)
	}
	if _, err := first.SendPreviewLive(); err != nil {
		t.Fatalf("send live: %v", err)
	}
	first.shutdown(context.Background())

	second := newAppAt(root)
	second.startup(context.Background())
	defer second.shutdown(context.Background())

	restored, err := second.GetServiceOrder()
	if err != nil {
		t.Fatalf("order after restart: %v", err)
	}
	if len(restored.Items) != 1 || restored.Items[0].Title != "John 3:16" {
		t.Fatalf("restored items = %+v", restored.Items)
	}

	state, err := second.GetPresentationState()
	if err != nil {
		t.Fatalf("presentation after restart: %v", err)
	}
	if state.Preview != nil || state.Live != nil {
		t.Fatalf("presentation should start clear after restart: %+v", state)
	}
}
