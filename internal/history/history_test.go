package history

import (
	"context"
	"path/filepath"
	"testing"
	"time"

	"zowe-worship/internal/database"
)

func newTestLog(t *testing.T) *Log {
	t.Helper()

	paths := database.Paths{Root: filepath.Join(t.TempDir(), "Zowe Worship")}
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
	return New(db)
}

func TestRecordThenRecentReturnsNewestFirst(t *testing.T) {
	ctx := context.Background()
	log := newTestLog(t)

	base := time.Date(2026, 11, 4, 10, 0, 0, 0, time.UTC)
	entries := []Entry{
		{Kind: "scripture", Title: "John 3:16", Label: "John 3:16", Source: SourceManual, Payload: "item-1", ProjectedAt: base},
		{Kind: "song", Title: "Amazing Grace", Label: "Amazing Grace · Verse 1", Source: SourceManual, ProjectedAt: base.Add(time.Minute)},
		{Kind: KindClear, Title: "Screen cleared", Source: SourceManual, ProjectedAt: base.Add(2 * time.Minute)},
	}
	for _, entry := range entries {
		if err := log.Record(ctx, entry); err != nil {
			t.Fatalf("record %s: %v", entry.Kind, err)
		}
	}

	got, err := log.Recent(ctx, "", 10)
	if err != nil {
		t.Fatalf("recent: %v", err)
	}
	if len(got) != 3 {
		t.Fatalf("got %d entries, want 3", len(got))
	}
	if got[0].Kind != KindClear || got[2].Kind != "scripture" {
		t.Fatalf("order = %s, %s, %s; want clear first, scripture last", got[0].Kind, got[1].Kind, got[2].Kind)
	}
	if got[2].Payload != "item-1" {
		t.Fatalf("payload = %q, want item-1", got[2].Payload)
	}
	if !got[0].ProjectedAt.Equal(base.Add(2 * time.Minute)) {
		t.Fatalf("projected_at = %s, want %s", got[0].ProjectedAt, base.Add(2*time.Minute))
	}
	if got[0].ID == "" {
		t.Fatal("recorded entry has no id")
	}
}

// aService creates a real service row: the history table references services,
// so a projection can only be filed under a service that exists.
func aService(t *testing.T, log *Log, name string) string {
	t.Helper()

	id := "service-" + name
	stamp := time.Now().UTC().Format(time.RFC3339)
	if _, err := log.db.Exec(
		`INSERT OR IGNORE INTO services (id, name, created_at, updated_at) VALUES (?, ?, ?, ?)`,
		id, name, stamp, stamp); err != nil {
		t.Fatalf("create service %s: %v", name, err)
	}
	return id
}

// A service keeps its own history: what one service put on the wall is never
// listed under another, and a projection made outside any service is listed
// under none of them.
func TestRecentIsScopedToOneService(t *testing.T) {
	ctx := context.Background()
	log := newTestLog(t)

	serviceA := aService(t, log, "a")
	serviceB := aService(t, log, "b")

	base := time.Date(2026, 11, 8, 9, 0, 0, 0, time.UTC)
	rows := []struct {
		service string
		title   string
	}{
		{serviceA, "John 3:16"},
		{serviceB, "Psalm 23:1"},
		{serviceA, "Isaiah 41:10"},
		{"", "No service at all"},
	}
	for _, row := range rows {
		entry := Entry{
			Kind: "scripture", Title: row.title, Label: row.title,
			ServiceID: row.service, Source: SourceManual, ProjectedAt: base,
		}
		if err := log.Record(ctx, entry); err != nil {
			t.Fatalf("record %s: %v", row.title, err)
		}
		base = base.Add(time.Minute)
	}

	got, err := log.Recent(ctx, "service-a", 10)
	if err != nil {
		t.Fatalf("recent: %v", err)
	}
	if len(got) != 2 {
		t.Fatalf("got %d entries, want the two of service-a", len(got))
	}
	for _, entry := range got {
		if entry.ServiceID != "service-a" {
			t.Fatalf("entry %q belongs to %q, want service-a", entry.Title, entry.ServiceID)
		}
	}

	other, err := log.Recent(ctx, "service-b", 10)
	if err != nil {
		t.Fatalf("recent for service-b: %v", err)
	}
	if len(other) != 1 || other[0].Title != "Psalm 23:1" {
		t.Fatalf("service-b history = %+v, want only its own entry", other)
	}

	none, err := log.Recent(ctx, "", 10)
	if err != nil {
		t.Fatalf("recent without a service: %v", err)
	}
	if len(none) != 1 || none[0].Title != "No service at all" {
		t.Fatalf("history without a service = %+v, want only the unattributed entry", none)
	}
}

// The kind CHECK in migration 009 only knew about scripture/song/media/clear;
// the schedule has since grown text, announcement and blank, so recording one
// of those has to survive the schema rather than be rejected by it.
func TestRecordAcceptsEveryKindTheControllerCanProject(t *testing.T) {
	ctx := context.Background()
	log := newTestLog(t)

	for _, kind := range []string{"scripture", "song", "media", "image", "video", "audio", "text", "announcement", "blank", KindClear} {
		if err := log.Record(ctx, Entry{Kind: kind, Title: "x", Source: SourceManual}); err != nil {
			t.Fatalf("record kind %q: %v", kind, err)
		}
	}

	got, err := log.Recent(ctx, "", 20)
	if err != nil {
		t.Fatalf("recent: %v", err)
	}
	if len(got) != 10 {
		t.Fatalf("got %d rows, want 10", len(got))
	}
}

func TestRecentHonoursTheLimit(t *testing.T) {
	ctx := context.Background()
	log := newTestLog(t)

	for i := 0; i < 5; i++ {
		if err := log.Record(ctx, Entry{Kind: "scripture", Title: "x", Source: SourceManual}); err != nil {
			t.Fatalf("record: %v", err)
		}
	}

	got, err := log.Recent(ctx, "", 2)
	if err != nil {
		t.Fatalf("recent: %v", err)
	}
	if len(got) != 2 {
		t.Fatalf("got %d entries, want 2", len(got))
	}
}

// Rows written inside the same second are the common case during a service, and
// the stamp is truncated to the second, so insertion order has to break the tie.
func TestRecentKeepsInsertionOrderWithinASecond(t *testing.T) {
	ctx := context.Background()
	log := newTestLog(t)

	stamp := time.Date(2026, 11, 4, 10, 0, 0, 0, time.UTC)
	for _, title := range []string{"first", "second", "third"} {
		if err := log.Record(ctx, Entry{Kind: "scripture", Title: title, Source: SourceManual, ProjectedAt: stamp}); err != nil {
			t.Fatalf("record %s: %v", title, err)
		}
	}

	got, err := log.Recent(ctx, "", 10)
	if err != nil {
		t.Fatalf("recent: %v", err)
	}
	if len(got) != 3 || got[0].Title != "third" || got[2].Title != "first" {
		t.Fatalf("same-second order = %+v, want third first", got)
	}
}

func TestNilDatabaseRecordsNothingAndReportsNoHistory(t *testing.T) {
	log := New(nil)
	if err := log.Record(context.Background(), Entry{Kind: "scripture", Title: "x"}); err != nil {
		t.Fatalf("record with no database: %v", err)
	}
	got, err := log.Recent(context.Background(), "", 10)
	if err != nil {
		t.Fatalf("recent with no database: %v", err)
	}
	if len(got) != 0 {
		t.Fatalf("got %d entries from no database", len(got))
	}
}

func TestRecentWithNonPositiveLimitReturnsNothing(t *testing.T) {
	ctx := context.Background()
	log := newTestLog(t)
	if err := log.Record(ctx, Entry{Kind: "scripture", Title: "x", Source: SourceManual}); err != nil {
		t.Fatalf("record: %v", err)
	}

	got, err := log.Recent(ctx, "", 0)
	if err != nil {
		t.Fatalf("recent: %v", err)
	}
	if len(got) != 0 {
		t.Fatalf("got %d entries for a zero limit", len(got))
	}
}
