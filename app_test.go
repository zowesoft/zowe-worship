package main

import (
	"context"
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"zopresent/internal/bible"

	"github.com/wailsapp/wails/v3/pkg/application"
)

func newTestApp(t *testing.T) *App {
	t.Helper()

	app := newAppAt(filepath.Join(t.TempDir(), "Zopresent"))
	if err := app.ServiceStartup(context.Background(), application.ServiceOptions{}); err != nil {
		t.Fatalf("service startup: %v", err)
	}
	t.Cleanup(func() {
		if err := app.ServiceShutdown(); err != nil {
			t.Errorf("service shutdown: %v", err)
		}
	})
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
			{BookID: 18, Chapter: 29, Number: 15, Text: "I put on righteousness, and it clothed me."},
			{BookID: 18, Chapter: 29, Number: 16, Text: "I was a father to the poor: and the cause which I knew not I searched out."},
			{BookID: 18, Chapter: 29, Number: 17, Text: "And I brake the jaws of the wicked, and plucked the spoil out of his teeth."},
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

	if err := app.ServiceStartup(ctx, application.ServiceOptions{}); err != nil {
		t.Fatalf("service startup: %v", err)
	}
	defer func() { _ = app.ServiceShutdown() }()

	if app.ctx != ctx {
		t.Fatal("startup did not retain the runtime context")
	}
}

func TestStartupInitializesPersistentStorage(t *testing.T) {
	root := filepath.Join(t.TempDir(), "Zopresent")
	app := newAppAt(root)

	ctx := context.Background()
	if err := app.ServiceStartup(ctx, application.ServiceOptions{}); err != nil {
		t.Fatalf("service startup: %v", err)
	}

	if app.core == nil || app.bible == nil {
		t.Fatal("startup did not open storage")
	}
	if _, err := os.Stat(filepath.Join(root, "zopresent.db")); err != nil {
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
	if err := app.ServiceShutdown(); err != nil {
		t.Fatalf("service shutdown: %v", err)
	}
	if err := core.PingContext(ctx); err == nil {
		t.Fatal("database was not closed on shutdown")
	}
	if app.core != nil {
		t.Fatal("shutdown did not release the database handle")
	}

	if err := app.ServiceStartup(ctx, application.ServiceOptions{}); err != nil {
		t.Fatalf("service startup after restart: %v", err)
	}
	defer func() { _ = app.ServiceShutdown() }()
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
	if info.Name != "Zopresent" || info.Version != "0.1.0" {
		t.Fatalf("unexpected application info: %+v", info)
	}

	data, err := json.Marshal(info)
	if err != nil {
		t.Fatal(err)
	}
	if string(data) != `{"name":"Zopresent","version":"0.1.0"}` {
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
	if view.ID != "kjv" || view.Abbreviation != "KJV" || view.VerseCount != 8 {
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

func TestSearchBibleReferenceQueuesFollowingVerses(t *testing.T) {
	app := newTestApp(t)
	installTestBible(t, app)

	// A single verse leads the results and the verses after it in the chapter
	// queue behind, so the operator can send the next verses without another
	// search.
	resp, err := app.SearchBible(BibleSearchRequest{TranslationID: "kjv", Query: "Job 29:15"})
	if err != nil {
		t.Fatalf("single verse search: %v", err)
	}
	if resp.Kind != "reference" || resp.VerseStart != 15 || resp.VerseEnd != 15 {
		t.Fatalf("unexpected reference response: %+v", resp)
	}
	if len(resp.Results) != 3 {
		t.Fatalf("results = %+v, want verses 15-17", resp.Results)
	}
	for i, verse := range resp.Results {
		if verse.Verse != 15+i {
			t.Fatalf("result %d is verse %d, want %d", i, verse.Verse, 15+i)
		}
	}

	// A named range is the highlighted block; the rest still queues behind.
	respRange, err := app.SearchBible(BibleSearchRequest{TranslationID: "kjv", Query: "Job 29:15-16"})
	if err != nil {
		t.Fatalf("range search: %v", err)
	}
	if respRange.VerseStart != 15 || respRange.VerseEnd != 16 {
		t.Fatalf("range span = %d-%d, want 15-16", respRange.VerseStart, respRange.VerseEnd)
	}
	if len(respRange.Results) != 3 || respRange.Results[0].Verse != 15 {
		t.Fatalf("range results = %+v", respRange.Results)
	}

	// A verse past the chapter's text still reports not found.
	if _, err := app.SearchBible(BibleSearchRequest{TranslationID: "kjv", Query: "Job 29:40"}); err == nil || !strings.Contains(err.Error(), "was not found") {
		t.Fatalf("missing verse err = %v", err)
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

func TestListBooks(t *testing.T) {
	app := newTestApp(t)
	books := app.ListBooks()
	if len(books) != 66 {
		t.Fatalf("books count = %d, want 66", len(books))
	}
	if books[0].Name != "Genesis" || books[0].Chapters != 50 || books[0].Testament != "old" {
		t.Fatalf("first book = %+v, want Genesis", books[0])
	}
	if books[65].Name != "Revelation" || books[65].Chapters != 22 || books[65].Testament != "new" {
		t.Fatalf("last book = %+v, want Revelation", books[65])
	}
}

func TestBrowsePassage(t *testing.T) {
	app := newTestApp(t)
	installTestBible(t, app)

	// Whole chapter
	resp, err := app.BrowsePassage(BrowsePassageRequest{
		TranslationID: "kjv",
		BookID:        19,
		Chapter:       23,
	})
	if err != nil {
		t.Fatalf("browse passage: %v", err)
	}
	if resp.Kind != "passage" || resp.Total != 2 || len(resp.Results) != 2 {
		t.Fatalf("browse response = %+v, want 2 verses", resp)
	}
	if resp.Results[0].Reference != "Psalms 23:1" || resp.Results[1].Reference != "Psalms 23:2" {
		t.Fatalf("browse results = %+v", resp.Results)
	}

	// Verse range
	respSingle, err := app.BrowsePassage(BrowsePassageRequest{
		TranslationID: "kjv",
		BookID:        19,
		Chapter:       23,
		VerseStart:    1,
		VerseEnd:      1,
	})
	if err != nil {
		t.Fatalf("browse single verse: %v", err)
	}
	if len(respSingle.Results) != 1 || respSingle.Results[0].Verse != 1 {
		t.Fatalf("browse single verse result = %+v", respSingle.Results)
	}

	// Validation
	if _, err := app.BrowsePassage(BrowsePassageRequest{BookID: 19, Chapter: 23}); err == nil {
		t.Fatal("browse passage without translation must fail")
	}
}

func TestSearchBibleFiltersAndSnippets(t *testing.T) {
	app := newTestApp(t)
	installTestBible(t, app)

	// Full text search with old testament filter should find Psalm 23:1
	resp, err := app.SearchBible(BibleSearchRequest{
		TranslationID: "kjv",
		Query:         "shepherd",
		Mode:          "words",
		Testament:     "old",
		BookIDs:       []int{19},
	})
	if err != nil {
		t.Fatalf("search with filters: %v", err)
	}
	if len(resp.Results) != 1 || resp.Results[0].Reference != "Psalms 23:1" {
		t.Fatalf("unexpected filtered search results: %+v", resp.Results)
	}
	if !strings.Contains(resp.Results[0].Snippet, "<mark>") {
		t.Fatalf("expected snippet to contain <mark>, got %q", resp.Results[0].Snippet)
	}

	// Filter with new testament should yield 0 results for "shepherd"
	respEmpty, err := app.SearchBible(BibleSearchRequest{
		TranslationID: "kjv",
		Query:         "shepherd",
		Mode:          "words",
		Testament:     "new",
	})
	if err != nil {
		t.Fatalf("search new testament: %v", err)
	}
	if len(respEmpty.Results) != 0 {
		t.Fatalf("expected 0 results, got %+v", respEmpty.Results)
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
	root := filepath.Join(t.TempDir(), "Zopresent")

	first := newAppAt(root)
	if err := first.ServiceStartup(context.Background(), application.ServiceOptions{}); err != nil {
		t.Fatalf("first startup: %v", err)
	}
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
	if err := first.ServiceShutdown(); err != nil {
		t.Fatalf("first shutdown: %v", err)
	}

	second := newAppAt(root)
	if err := second.ServiceStartup(context.Background(), application.ServiceOptions{}); err != nil {
		t.Fatalf("second startup: %v", err)
	}
	defer func() { _ = second.ServiceShutdown() }()

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

func TestCustomItemsAndDuplicationInApp(t *testing.T) {
	app := newTestApp(t)
	installTestBible(t, app)

	order, err := app.CreateService("Sunday Morning")
	if err != nil {
		t.Fatalf("create service: %v", err)
	}

	// Add scripture
	order, err = app.AddScriptureToService(ScriptureRequest{
		TranslationID: "kjv", BookID: 43, Chapter: 3, VerseStart: 16,
	})
	if err != nil {
		t.Fatalf("add scripture: %v", err)
	}

	// Add Text slide
	order, err = app.AddCustomItemToService(CustomItemRequest{
		Kind:     "text",
		Title:    "Call to Worship",
		Content:  "Praise the Lord, all nations!\nExtol him, all peoples!",
		Notes:    "Leader speaks from pulpit",
		Duration: 180,
	})
	if err != nil {
		t.Fatalf("add text item: %v", err)
	}

	// Add Announcement
	order, err = app.AddCustomItemToService(CustomItemRequest{
		Kind:     "announcement",
		Title:    "Fellowship Lunch",
		Content:  "Join us in the fellowship hall after service.",
		Duration: 60,
	})
	if err != nil {
		t.Fatalf("add announcement: %v", err)
	}

	// Add Blank divider
	order, err = app.AddCustomItemToService(CustomItemRequest{
		Kind:  "blank",
		Title: "Silent Reflection",
	})
	if err != nil {
		t.Fatalf("add blank divider: %v", err)
	}

	if len(order.Items) != 4 {
		t.Fatalf("expected 4 items, got %d", len(order.Items))
	}

	// Verify all items start with upcoming status
	for _, item := range order.Items {
		if item.Status != "upcoming" {
			t.Fatalf("item %s status = %q, want upcoming", item.Title, item.Status)
		}
	}

	// Preview item 1 (John 3:16)
	_, err = app.PreviewServiceItem(order.Items[0].ID)
	if err != nil {
		t.Fatalf("preview item 0: %v", err)
	}
	order, _ = app.GetServiceOrder()
	if order.Items[0].Status != "next" {
		t.Fatalf("previewed item status = %q, want next", order.Items[0].Status)
	}

	// Send item 1 Live
	_, err = app.SendPreviewLive()
	if err != nil {
		t.Fatalf("send live: %v", err)
	}

	// Preview item 2
	_, err = app.PreviewServiceItem(order.Items[1].ID)
	if err != nil {
		t.Fatalf("preview item 1: %v", err)
	}

	order, _ = app.GetServiceOrder()
	if order.Items[0].Status != "live" {
		t.Fatalf("item 0 status = %q, want live", order.Items[0].Status)
	}
	if order.Items[1].Status != "next" {
		t.Fatalf("item 1 status = %q, want next", order.Items[1].Status)
	}
	if order.Items[2].Status != "upcoming" || order.Items[3].Status != "upcoming" {
		t.Fatalf("items 2 & 3 should be upcoming")
	}

	// Send item 1 (preview) live
	_, err = app.SendPreviewLive()
	if err != nil {
		t.Fatalf("send preview live: %v", err)
	}

	order, _ = app.GetServiceOrder()
	if order.Items[0].Status != "completed" {
		t.Fatalf("item 0 status = %q, want completed", order.Items[0].Status)
	}
	if order.Items[1].Status != "live" {
		t.Fatalf("item 1 status = %q, want live", order.Items[1].Status)
	}

	// Duplicate Service
	duplicated, err := app.DuplicateService(order.Service.ID, "Sunday Evening")
	if err != nil {
		t.Fatalf("duplicate service: %v", err)
	}
	if duplicated.Service.Name != "Sunday Evening" {
		t.Fatalf("duplicated service name = %q", duplicated.Service.Name)
	}
	if len(duplicated.Items) != 4 {
		t.Fatalf("duplicated items count = %d, want 4", len(duplicated.Items))
	}
	if duplicated.Items[1].Title != "Call to Worship" || duplicated.Items[1].Content == "" {
		t.Fatalf("duplicated text item content missing: %+v", duplicated.Items[1])
	}

	// Update metadata
	updated, err := app.UpdateServiceItemMeta(duplicated.Items[1].ID, "Pianist plays prelude", 240)
	if err != nil {
		t.Fatalf("update item meta: %v", err)
	}
	if updated.Items[1].Notes != "Pianist plays prelude" || updated.Items[1].Duration != 240 {
		t.Fatalf("meta not updated: %+v", updated.Items[1])
	}
}
