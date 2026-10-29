package serviceorder

import (
	"context"
	"database/sql"
	"errors"
	"path/filepath"
	"testing"

	"zowe-worship/internal/database"
)

func newTestRepository(t *testing.T) (*SQLiteRepository, *sql.DB, string) {
	t.Helper()

	root := filepath.Join(t.TempDir(), "Zowe Worship")
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
