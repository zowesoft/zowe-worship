package serviceorder

import (
	"context"
	"database/sql"
	"errors"
	"path/filepath"
	"testing"

	"zopresent/internal/database"
)

func newTestRepository(t *testing.T) (*SQLiteRepository, *sql.DB, string) {
	t.Helper()

	root := filepath.Join(t.TempDir(), "Zopresent")
	paths := database.Paths{Root: root}
	if err := paths.Ensure(); err != nil {
		t.Fatalf("prepare directories: %v", err)
	}
	db, err := database.Open(context.Background(), paths.CoreDatabase())
	if err != nil {
		t.Fatalf("open core database: %v", err)
	}
	if err := database.Migrate(context.Background(), db, paths.Backups()); err != nil {
		t.Fatalf("migrate: %v", err)
	}
	t.Cleanup(func() { _ = db.Close() })
	return NewSQLiteRepository(db), db, paths.CoreDatabase()
}

func scriptureInput(title string, bookID, chapter, verse int) ScriptureInput {
	return ScriptureInput{
		Title:         title,
		TranslationID: "kjv",
		BookID:        bookID,
		Chapter:       chapter,
		VerseStart:    verse,
		VerseEnd:      verse,
	}
}

func TestCurrentReportsNoService(t *testing.T) {
	repo, _, _ := newTestRepository(t)

	if _, _, err := repo.Current(context.Background()); !errors.Is(err, ErrNoService) {
		t.Fatalf("err = %v, want ErrNoService", err)
	}
}

func TestCreateAddAndRemoveScheduleItems(t *testing.T) {
	ctx := context.Background()
	repo, _, _ := newTestRepository(t)

	service, err := repo.Create(ctx, "  Sunday Service  ")
	if err != nil {
		t.Fatalf("create service: %v", err)
	}
	if service.Name != "Sunday Service" {
		t.Fatalf("name = %q", service.Name)
	}

	first, err := repo.AddScripture(ctx, service.ID, scriptureInput("John 3:16", 43, 3, 16))
	if err != nil {
		t.Fatalf("add first item: %v", err)
	}
	if first.Position != 1 || first.Kind != KindScripture {
		t.Fatalf("first item = %+v", first)
	}

	second, err := repo.AddScripture(ctx, service.ID, scriptureInput("Psalms 23:1", 19, 23, 1))
	if err != nil {
		t.Fatalf("add second item: %v", err)
	}
	if second.Position != 2 {
		t.Fatalf("second position = %d, want 2", second.Position)
	}

	current, items, err := repo.Current(ctx)
	if err != nil {
		t.Fatalf("current: %v", err)
	}
	if current.ID != service.ID {
		t.Fatalf("current service = %s, want %s", current.ID, service.ID)
	}
	if len(items) != 2 || items[0].ID != first.ID || items[1].ID != second.ID {
		t.Fatalf("items = %+v", items)
	}
	if items[0].Title != "John 3:16" || items[0].TranslationID != "kjv" {
		t.Fatalf("first item content = %+v", items[0])
	}

	loaded, err := repo.Item(ctx, first.ID)
	if err != nil {
		t.Fatalf("load item: %v", err)
	}
	if loaded.Title != first.Title || loaded.BookID != 43 {
		t.Fatalf("loaded item = %+v", loaded)
	}

	if err := repo.RemoveItem(ctx, first.ID); err != nil {
		t.Fatalf("remove item: %v", err)
	}
	_, items, err = repo.Current(ctx)
	if err != nil {
		t.Fatalf("current after removal: %v", err)
	}
	if len(items) != 1 || items[0].ID != second.ID {
		t.Fatalf("items after removal = %+v", items)
	}
	if err := repo.RemoveItem(ctx, first.ID); !errors.Is(err, ErrItemNotFound) {
		t.Fatalf("second removal err = %v", err)
	}
}

func TestReorderSchedule(t *testing.T) {
	ctx := context.Background()
	repo, _, _ := newTestRepository(t)

	service, err := repo.Create(ctx, "Sunday Service")
	if err != nil {
		t.Fatalf("create: %v", err)
	}

	ids := make([]string, 0, 3)
	for index, title := range []string{"John 3:16", "Psalms 23:1", "Genesis 1:1"} {
		item, err := repo.AddScripture(ctx, service.ID, scriptureInput(title, 43, 3, 16+index))
		if err != nil {
			t.Fatalf("add %s: %v", title, err)
		}
		ids = append(ids, item.ID)
	}

	if err := repo.Reorder(ctx, []string{ids[2], ids[0], ids[1]}); err != nil {
		t.Fatalf("reorder: %v", err)
	}
	assertOrder(t, repo, service.ID, []string{ids[2], ids[0], ids[1]})

	// A dropped row must not leave the schedule half-written.
	if err := repo.Reorder(ctx, []string{ids[0], ids[1]}); !errors.Is(err, ErrInvalidOrder) {
		t.Fatalf("partial reorder err = %v", err)
	}
	assertOrder(t, repo, service.ID, []string{ids[2], ids[0], ids[1]})

	for _, bad := range [][]string{
		{},
		{ids[0], ids[0], ids[1]},
		{ids[0], ids[1], ids[2], "missing"},
	} {
		if err := repo.Reorder(ctx, bad); !errors.Is(err, ErrInvalidOrder) && !errors.Is(err, ErrItemNotFound) {
			t.Fatalf("reorder %v err = %v", bad, err)
		}
	}
	assertOrder(t, repo, service.ID, []string{ids[2], ids[0], ids[1]})
}

func assertOrder(t *testing.T, repo *SQLiteRepository, serviceID string, want []string) {
	t.Helper()

	items, err := repo.Items(context.Background(), serviceID)
	if err != nil {
		t.Fatalf("list items: %v", err)
	}
	if len(items) != len(want) {
		t.Fatalf("items = %d, want %d", len(items), len(want))
	}
	for index, item := range items {
		if item.ID != want[index] {
			t.Fatalf("position %d = %s, want %s", index+1, item.ID, want[index])
		}
		if item.Position != index+1 {
			t.Fatalf("item %s position = %d, want %d", item.ID, item.Position, index+1)
		}
	}
}

func TestCreateAndInputValidation(t *testing.T) {
	ctx := context.Background()
	repo, _, _ := newTestRepository(t)

	if _, err := repo.Create(ctx, "   "); !errors.Is(err, ErrInvalidService) {
		t.Fatalf("empty name err = %v", err)
	}

	service, err := repo.Create(ctx, "Service")
	if err != nil {
		t.Fatalf("create: %v", err)
	}

	bad := []ScriptureInput{
		{Title: "", TranslationID: "kjv", BookID: 43, Chapter: 3, VerseStart: 16, VerseEnd: 16},
		{Title: "John 3:16", TranslationID: "", BookID: 43, Chapter: 3, VerseStart: 16, VerseEnd: 16},
		{Title: "John 3:16", TranslationID: "kjv", BookID: 0, Chapter: 3, VerseStart: 16, VerseEnd: 16},
		{Title: "John 3:16", TranslationID: "kjv", BookID: 43, Chapter: 3, VerseStart: 0, VerseEnd: 0},
		{Title: "John 3:17-16", TranslationID: "kjv", BookID: 43, Chapter: 3, VerseStart: 17, VerseEnd: 16},
	}
	for _, input := range bad {
		if _, err := repo.AddScripture(ctx, service.ID, input); !errors.Is(err, ErrInvalidService) {
			t.Fatalf("input %+v: err = %v", input, err)
		}
	}

	if _, err := repo.AddScripture(ctx, "missing", scriptureInput("John 3:16", 43, 3, 16)); !errors.Is(err, ErrServiceNotFound) {
		t.Fatalf("missing service err = %v", err)
	}
}

func TestServiceSwitching(t *testing.T) {
	ctx := context.Background()
	repo, db, _ := newTestRepository(t)

	first, err := repo.Create(ctx, "Christmas Eve")
	if err != nil {
		t.Fatalf("create first: %v", err)
	}
	if current, _, err := repo.Current(ctx); err != nil || current.ID != first.ID {
		t.Fatalf("current = %+v err = %v, want the only service", current, err)
	}

	second, err := repo.Create(ctx, "Easter")
	if err != nil {
		t.Fatalf("create second: %v", err)
	}
	if current, _, err := repo.Current(ctx); err != nil || current.ID != second.ID {
		t.Fatalf("current after create = %+v err = %v, want the new service", current, err)
	}

	if _, err := repo.AddScripture(ctx, second.ID, scriptureInput("John 3:16", 43, 3, 16)); err != nil {
		t.Fatalf("add item: %v", err)
	}
	if err := repo.Use(ctx, first.ID); err != nil {
		t.Fatalf("switch to first: %v", err)
	}
	current, items, err := repo.Current(ctx)
	if err != nil {
		t.Fatalf("current: %v", err)
	}
	if current.ID != first.ID {
		t.Fatalf("current = %s, want %s", current.ID, first.ID)
	}
	if len(items) != 0 {
		t.Fatalf("switching services carried %d items over", len(items))
	}

	services, err := repo.List(ctx)
	if err != nil {
		t.Fatalf("list services: %v", err)
	}
	if len(services) != 2 {
		t.Fatalf("services = %+v, want two", services)
	}

	if err := repo.Use(ctx, "missing"); !errors.Is(err, ErrServiceNotFound) {
		t.Fatalf("use missing err = %v", err)
	}
	if err := repo.Rename(ctx, first.ID, "  Baptism Service  "); err != nil {
		t.Fatalf("rename: %v", err)
	}
	if current, _, err := repo.Current(ctx); err != nil || current.Name != "Baptism Service" {
		t.Fatalf("current after rename = %+v err = %v", current, err)
	}
	if err := repo.Rename(ctx, first.ID, "   "); !errors.Is(err, ErrInvalidService) {
		t.Fatalf("empty rename err = %v", err)
	}
	if err := repo.Rename(ctx, "missing", "Name"); !errors.Is(err, ErrServiceNotFound) {
		t.Fatalf("rename missing err = %v", err)
	}

	// Deleting the stored service leaves a dangling pointer that has to heal.
	if _, err := db.ExecContext(ctx, `DELETE FROM services WHERE id = ?`, first.ID); err != nil {
		t.Fatalf("delete current service: %v", err)
	}
	if current, _, err := repo.Current(ctx); err != nil || current.ID != second.ID {
		t.Fatalf("current after deletion = %+v err = %v, want the surviving service", current, err)
	}
}

func TestScheduleSurvivesReopen(t *testing.T) {
	ctx := context.Background()
	repo, db, path := newTestRepository(t)

	service, err := repo.Create(ctx, "Sunday Service")
	if err != nil {
		t.Fatalf("create: %v", err)
	}
	if _, err := repo.AddScripture(ctx, service.ID, scriptureInput("John 3:16", 43, 3, 16)); err != nil {
		t.Fatalf("add: %v", err)
	}
	if err := db.Close(); err != nil {
		t.Fatalf("close: %v", err)
	}

	reopened, err := database.Open(ctx, path)
	if err != nil {
		t.Fatalf("reopen: %v", err)
	}
	t.Cleanup(func() { _ = reopened.Close() })
	if err := database.Migrate(ctx, reopened, ""); err != nil {
		t.Fatalf("migrate after reopen: %v", err)
	}

	current, items, err := NewSQLiteRepository(reopened).Current(ctx)
	if err != nil {
		t.Fatalf("current after reopen: %v", err)
	}
	if current.Name != "Sunday Service" || len(items) != 1 || items[0].Title != "John 3:16" {
		t.Fatalf("current = %+v items = %+v", current, items)
	}
}

func TestItemThemeOverrideRoundTrip(t *testing.T) {
	repo, _, _ := newTestRepository(t)
	ctx := context.Background()

	service, err := repo.Create(ctx, "Sunday Service")
	if err != nil {
		t.Fatalf("create service: %v", err)
	}
	item, err := repo.AddScripture(ctx, service.ID, scriptureInput("John 3:16", 43, 3, 16))
	if err != nil {
		t.Fatalf("add scripture: %v", err)
	}
	if item.ThemeID != "" {
		t.Fatalf("a new item should start on its kind default, got %q", item.ThemeID)
	}

	if err := repo.SetTheme(ctx, item.ID, "theme-42"); err != nil {
		t.Fatalf("set theme: %v", err)
	}
	stored, err := repo.Item(ctx, item.ID)
	if err != nil {
		t.Fatalf("read item: %v", err)
	}
	if stored.ThemeID != "theme-42" {
		t.Fatalf("theme = %q, want theme-42", stored.ThemeID)
	}

	// Clearing the override puts the item back on whatever its kind default is
	// at the time it is projected, rather than freezing today's winner in place.
	if err := repo.SetTheme(ctx, item.ID, ""); err != nil {
		t.Fatalf("clear theme: %v", err)
	}
	cleared, err := repo.Item(ctx, item.ID)
	if err != nil {
		t.Fatalf("read item again: %v", err)
	}
	if cleared.ThemeID != "" {
		t.Fatalf("clearing the override left %q", cleared.ThemeID)
	}

	if err := repo.SetTheme(ctx, "missing", "theme-42"); !errors.Is(err, ErrItemNotFound) {
		t.Fatalf("err = %v, want ErrItemNotFound", err)
	}
}

func TestDuplicateService(t *testing.T) {
	repo, _, _ := newTestRepository(t)
	ctx := context.Background()

	service, err := repo.Create(ctx, "Morning Service")
	if err != nil {
		t.Fatalf("create service: %v", err)
	}

	_, err = repo.AddScripture(ctx, service.ID, scriptureInput("John 3:16", 43, 3, 16))
	if err != nil {
		t.Fatalf("add scripture: %v", err)
	}
	_, err = repo.AddCustomItem(ctx, service.ID, CustomItemInput{
		Kind:     KindText,
		Title:    "Welcome",
		Content:  "Welcome to church!",
		Notes:    "Play background music",
		Duration: 120,
	})
	if err != nil {
		t.Fatalf("add text item: %v", err)
	}

	copySvc, err := repo.Duplicate(ctx, service.ID, "Morning Service - Evening")
	if err != nil {
		t.Fatalf("duplicate service: %v", err)
	}
	if copySvc.Name != "Morning Service - Evening" {
		t.Fatalf("name = %q, want Morning Service - Evening", copySvc.Name)
	}

	items, err := repo.Items(ctx, copySvc.ID)
	if err != nil {
		t.Fatalf("read items: %v", err)
	}
	if len(items) != 2 {
		t.Fatalf("items count = %d, want 2", len(items))
	}
	if items[0].Title != "John 3:16" || items[1].Title != "Welcome" {
		t.Fatalf("unexpected items: %+v", items)
	}
	if items[1].Content != "Welcome to church!" || items[1].Notes != "Play background music" || items[1].Duration != 120 {
		t.Fatalf("unexpected text item copy: %+v", items[1])
	}
}

func TestCustomItemKindsAndMetaUpdate(t *testing.T) {
	repo, _, _ := newTestRepository(t)
	ctx := context.Background()

	service, err := repo.Create(ctx, "Test Service")
	if err != nil {
		t.Fatalf("create service: %v", err)
	}

	announcement, err := repo.AddCustomItem(ctx, service.ID, CustomItemInput{
		Kind:     KindAnnouncement,
		Title:    "Youth Camp",
		Content:  "Register by Friday at the desk.",
		Duration: 60,
	})
	if err != nil {
		t.Fatalf("add announcement: %v", err)
	}
	if announcement.Kind != KindAnnouncement || announcement.Duration != 60 {
		t.Fatalf("announcement = %+v", announcement)
	}

	blank, err := repo.AddCustomItem(ctx, service.ID, CustomItemInput{
		Kind:  KindBlank,
		Title: "Silent Prayer",
	})
	if err != nil {
		t.Fatalf("add blank: %v", err)
	}
	if blank.Kind != KindBlank {
		t.Fatalf("blank kind = %s", blank.Kind)
	}

	// Update metadata
	if err := repo.UpdateItemMeta(ctx, announcement.ID, "Display slide during announcements", 90); err != nil {
		t.Fatalf("update meta: %v", err)
	}
	updated, err := repo.Item(ctx, announcement.ID)
	if err != nil {
		t.Fatalf("read updated item: %v", err)
	}
	if updated.Notes != "Display slide during announcements" || updated.Duration != 90 {
		t.Fatalf("updated item meta = %+v", updated)
	}
}
