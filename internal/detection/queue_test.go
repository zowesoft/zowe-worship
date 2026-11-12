package detection

import (
	"context"
	"database/sql"
	"errors"
	"path/filepath"
	"testing"
	"time"

	"zowe-worship/internal/asr"
	"zowe-worship/internal/bible"
	"zowe-worship/internal/confidence"
	"zowe-worship/internal/database"
	"zowe-worship/internal/scripture"
)

func newTestQueue(t *testing.T) *Queue {
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

func detectionOf(t *testing.T, text string) asr.Detection {
	t.Helper()
	reference, err := scripture.Parse(text)
	if err != nil {
		t.Fatalf("parse %q: %v", text, err)
	}
	return asr.Detection{Reference: reference, Raw: text}
}

// aService creates a real service row: the history table references services,
// so a decision can only be filed under a service that exists.
func aService(t *testing.T, queue *Queue, name string) string {
	t.Helper()

	id := "service-" + name
	stamp := time.Now().UTC().Format(time.RFC3339)
	if _, err := queue.db.Exec(
		`INSERT OR IGNORE INTO services (id, name, created_at, updated_at) VALUES (?, ?, ?, ?)`,
		id, name, stamp, stamp); err != nil {
		t.Fatalf("create service %s: %v", name, err)
	}
	return id
}

func TestAddQueuesOneCandidatePerDetection(t *testing.T) {
	queue := newTestQueue(t)

	added, err := queue.Add("let us turn to John 3:16 and Romans 8", []asr.Detection{
		detectionOf(t, "John 3:16"),
		detectionOf(t, "Romans 8"),
	})
	if err != nil {
		t.Fatalf("add: %v", err)
	}
	if len(added) != 2 {
		t.Fatalf("added = %d candidates, want 2", len(added))
	}
	if added[0].Reference.String() != "John 3:16" || added[1].Reference.String() != "Romans 8" {
		t.Fatalf("references = %q, %q", added[0].Reference.String(), added[1].Reference.String())
	}
	if added[0].Transcript != "let us turn to John 3:16 and Romans 8" {
		t.Fatalf("transcript = %q", added[0].Transcript)
	}

	pending := queue.List()
	if len(pending) != 2 {
		t.Fatalf("pending = %d candidates, want 2", len(pending))
	}
	if pending[0].ID != added[0].ID {
		t.Fatalf("queue order changed: %q then %q", pending[0].ID, added[0].ID)
	}
}

func TestCandidateConfidenceTracksPhrasing(t *testing.T) {
	queue := newTestQueue(t)

	cases := []struct {
		name       string
		transcript string
		reference  string
		want       string
	}{
		{"written reference", "Turn with me to John 3:16", "John 3:16", confidence.BandHigh},
		{"spoken connectors", "John chapter three verse sixteen", "John 3:16", confidence.BandHigh},
		{"bare verse numbers", "John three sixteen", "John 3:16", confidence.BandMedium},
		{"chapter only", "Philippians chapter 4", "Philippians 4", confidence.BandMedium},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			added, err := queue.Add(tc.transcript, []asr.Detection{detectionOf(t, tc.reference)})
			if err != nil {
				t.Fatalf("add: %v", err)
			}
			if got := confidence.BandOf(added[0].Confidence); got != tc.want {
				t.Errorf("band = %q (score %v), want %q", got, added[0].Confidence, tc.want)
			}
		})
	}
}

func TestApproveRemovesCandidateAndRecordsDecision(t *testing.T) {
	ctx := context.Background()
	queue := newTestQueue(t)

	added, err := queue.Add("John 3:16", []asr.Detection{detectionOf(t, "John 3:16")})
	if err != nil {
		t.Fatalf("add: %v", err)
	}

	candidate, err := queue.Approve(ctx, added[0].ID, aService(t, queue, "one"))
	if err != nil {
		t.Fatalf("approve: %v", err)
	}
	if candidate.Reference.String() != "John 3:16" {
		t.Fatalf("approved reference = %q", candidate.Reference.String())
	}
	if len(queue.List()) != 0 {
		t.Fatalf("queue still holds %d candidates after approval", len(queue.List()))
	}

	assertHistory(t, queue.db, added[0].ID, "John 3:16", decisionApproved)
}

func TestDismissRecordsDecision(t *testing.T) {
	ctx := context.Background()
	queue := newTestQueue(t)

	added, err := queue.Add("Psalm 23", []asr.Detection{detectionOf(t, "Psalm 23")})
	if err != nil {
		t.Fatalf("add: %v", err)
	}
	if _, err := queue.Dismiss(ctx, added[0].ID, aService(t, queue, "one")); err != nil {
		t.Fatalf("dismiss: %v", err)
	}
	if len(queue.List()) != 0 {
		t.Fatalf("queue still holds the dismissed candidate")
	}

	assertHistory(t, queue.db, added[0].ID, "Psalms 23", decisionDismissed)
}

// A decision belongs to the service it was taken in: what one service heard
// and what the operator decided about it never shows up under another.
func TestHistoryIsScopedToOneService(t *testing.T) {
	ctx := context.Background()
	queue := newTestQueue(t)

	first, err := queue.Add("John 3:16", []asr.Detection{detectionOf(t, "John 3:16")})
	if err != nil {
		t.Fatalf("add: %v", err)
	}
	if _, err := queue.Approve(ctx, first[0].ID, aService(t, queue, "a")); err != nil {
		t.Fatalf("approve: %v", err)
	}

	second, err := queue.Add("Psalm 23", []asr.Detection{detectionOf(t, "Psalm 23")})
	if err != nil {
		t.Fatalf("add: %v", err)
	}
	if _, err := queue.Dismiss(ctx, second[0].ID, aService(t, queue, "b")); err != nil {
		t.Fatalf("dismiss: %v", err)
	}

	mine, err := queue.History(ctx, "service-a", 10)
	if err != nil {
		t.Fatalf("history: %v", err)
	}
	if len(mine) != 1 || mine[0].Reference != "John 3:16" {
		t.Fatalf("service-a history = %+v, want only its own decision", mine)
	}
	if mine[0].Decision != decisionApproved {
		t.Fatalf("decision = %q, want %q", mine[0].Decision, decisionApproved)
	}

	theirs, err := queue.History(ctx, "service-b", 10)
	if err != nil {
		t.Fatalf("history for service-b: %v", err)
	}
	if len(theirs) != 1 || theirs[0].Reference != "Psalms 23" {
		t.Fatalf("service-b history = %+v, want only its own decision", theirs)
	}

	none, err := queue.History(ctx, "", 10)
	if err != nil {
		t.Fatalf("history without a service: %v", err)
	}
	if len(none) != 0 {
		t.Fatalf("history without a service = %+v, want nothing", none)
	}
}

func TestClearRecordsIgnoredAndEmptiesQueue(t *testing.T) {
	ctx := context.Background()
	queue := newTestQueue(t)

	first, err := queue.Add("John 3:16", []asr.Detection{detectionOf(t, "John 3:16")})
	if err != nil {
		t.Fatalf("add first: %v", err)
	}
	second, err := queue.Add("Romans 8:28", []asr.Detection{detectionOf(t, "Romans 8:28")})
	if err != nil {
		t.Fatalf("add second: %v", err)
	}

	if err := queue.Clear(ctx, aService(t, queue, "one")); err != nil {
		t.Fatalf("clear: %v", err)
	}
	if len(queue.List()) != 0 {
		t.Fatalf("queue still holds candidates after clear")
	}

	assertHistory(t, queue.db, first[0].ID, "John 3:16", decisionIgnored)
	assertHistory(t, queue.db, second[0].ID, "Romans 8:28", decisionIgnored)
}

func TestDecideUnknownCandidateFails(t *testing.T) {
	ctx := context.Background()
	queue := newTestQueue(t)

	if _, err := queue.Approve(ctx, "missing", aService(t, queue, "one")); !errors.Is(err, ErrCandidateNotFound) {
		t.Fatalf("err = %v, want ErrCandidateNotFound", err)
	}
	if _, err := queue.Dismiss(ctx, "missing", aService(t, queue, "one")); !errors.Is(err, ErrCandidateNotFound) {
		t.Fatalf("err = %v, want ErrCandidateNotFound", err)
	}
}

func TestQueueWithoutDatabaseStillWorks(t *testing.T) {
	ctx := context.Background()
	queue := New(nil)

	added, err := queue.Add("John 3:16", []asr.Detection{detectionOf(t, "John 3:16")})
	if err != nil {
		t.Fatalf("add: %v", err)
	}
	if _, err := queue.Approve(ctx, added[0].ID, "service-1"); err != nil {
		t.Fatalf("approve without storage: %v", err)
	}
	if err := queue.Clear(ctx, "service-1"); err != nil {
		t.Fatalf("clear without storage: %v", err)
	}
}

func assertHistory(t *testing.T, db *sql.DB, id, reference, decision string) {
	t.Helper()

	var gotReference, gotDecision string
	err := db.QueryRow(`SELECT reference, decision FROM detection_history WHERE id = ?`, id).
		Scan(&gotReference, &gotDecision)
	if err != nil {
		t.Fatalf("read history for %s: %v", id, err)
	}
	if gotReference != reference || gotDecision != decision {
		t.Fatalf("history = (%q, %q), want (%q, %q)", gotReference, gotDecision, reference, decision)
	}
}

func TestMergeLiveRefinesWideHearingIntoNarrow(t *testing.T) {
	queue := newTestQueue(t)

	// A segment boundary can land mid-sentence: the chapter is heard before
	// the verses finish, so the first candidate is the whole chapter. A bare
	// chapter hearing is chapter-wide (zero verse range), which scripture.Parse
	// does not produce, so it is built directly.
	book, ok := bible.LookupBook("Philippians")
	if !ok {
		t.Fatalf("lookup Philippians")
	}
	chapterWide := asr.Detection{Reference: scripture.Reference{Book: book, Chapter: 4}, Raw: "Philippians 4"}
	first, err := queue.MergeLive("Philippians chapter 4", []asr.Detection{chapterWide})
	if err != nil {
		t.Fatalf("first merge: %v", err)
	}
	if len(first) != 1 {
		t.Fatalf("first merge added %d candidates", len(first))
	}

	narrow := detectionOf(t, "Philippians 4:6-7")
	second, err := queue.MergeLive("Philippians chapter 4 verses 6 and 7", []asr.Detection{narrow})
	if err != nil {
		t.Fatalf("second merge: %v", err)
	}
	pending := queue.List()
	if len(pending) != 1 {
		t.Fatalf("queue holds %d candidates, want 1 replaced card", len(pending))
	}
	if pending[0].ID != first[0].ID {
		t.Error("replacement changed the candidate id; the panel would stack a second card")
	}
	if pending[0].Reference.VerseStart != 6 || pending[0].Reference.VerseEnd != 7 {
		t.Errorf("pending range = %d-%d, want 6-7", pending[0].Reference.VerseStart, pending[0].Reference.VerseEnd)
	}
	if len(second) != 1 || second[0].ID != first[0].ID {
		t.Errorf("merge result = %+v, want the refined candidate", second)
	}
}

func TestMergeLiveGrowsNarrowHearingIntoWide(t *testing.T) {
	queue := newTestQueue(t)

	first, err := queue.MergeLive("Philippians 4:6", []asr.Detection{detectionOf(t, "Philippians 4:6")})
	if err != nil {
		t.Fatalf("first merge: %v", err)
	}

	wide := detectionOf(t, "Philippians 4:6-7")
	second, err := queue.MergeLive("Philippians 4:6-7", []asr.Detection{wide})
	if err != nil {
		t.Fatalf("second merge: %v", err)
	}
	pending := queue.List()
	if len(pending) != 1 {
		t.Fatalf("queue holds %d candidates, want 1 replaced card", len(pending))
	}
	if pending[0].ID != first[0].ID {
		t.Error("replacement changed the candidate id; the panel would stack a second card")
	}
	if pending[0].Reference.VerseEnd != 7 {
		t.Errorf("pending range = %d-%d, want 6-7", pending[0].Reference.VerseStart, pending[0].Reference.VerseEnd)
	}
	if len(second) != 1 || second[0].ID != first[0].ID {
		t.Errorf("merge result = %+v, want the upgraded candidate", second)
	}
}

func TestMergeLiveRehearingSamePassageUpdatesInPlace(t *testing.T) {
	queue := newTestQueue(t)

	if _, err := queue.MergeLive("Philippians 4:6-7", []asr.Detection{detectionOf(t, "Philippians 4:6-7")}); err != nil {
		t.Fatalf("first merge: %v", err)
	}
	if _, err := queue.MergeLive("Philippians 4:6-7 again", []asr.Detection{detectionOf(t, "Philippians 4:6-7")}); err != nil {
		t.Fatalf("second merge: %v", err)
	}
	if pending := queue.List(); len(pending) != 1 {
		t.Fatalf("queue holds %d candidates, want 1", len(pending))
	}
}

func TestMergeLiveKeepsDistinctPassages(t *testing.T) {
	queue := newTestQueue(t)

	if _, err := queue.MergeLive("Philippians 4:6", []asr.Detection{detectionOf(t, "Philippians 4:6")}); err != nil {
		t.Fatalf("first merge: %v", err)
	}
	if _, err := queue.MergeLive("Romans 8:28", []asr.Detection{detectionOf(t, "Romans 8:28")}); err != nil {
		t.Fatalf("second merge: %v", err)
	}
	if pending := queue.List(); len(pending) != 2 {
		t.Fatalf("queue holds %d candidates, want 2 distinct passages", len(pending))
	}
}
