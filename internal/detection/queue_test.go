package detection

import (
	"context"
	"database/sql"
	"errors"
	"path/filepath"
	"testing"

	"zowe-worship/internal/asr"
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

func TestApproveRemovesCandidateAndRecordsDecision(t *testing.T) {
	ctx := context.Background()
	queue := newTestQueue(t)

	added, err := queue.Add("John 3:16", []asr.Detection{detectionOf(t, "John 3:16")})
	if err != nil {
		t.Fatalf("add: %v", err)
	}

	candidate, err := queue.Approve(ctx, added[0].ID)
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
	if _, err := queue.Dismiss(ctx, added[0].ID); err != nil {
		t.Fatalf("dismiss: %v", err)
	}
	if len(queue.List()) != 0 {
		t.Fatalf("queue still holds the dismissed candidate")
	}

	assertHistory(t, queue.db, added[0].ID, "Psalms 23", decisionDismissed)
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

	if err := queue.Clear(ctx); err != nil {
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

	if _, err := queue.Approve(ctx, "missing"); !errors.Is(err, ErrCandidateNotFound) {
		t.Fatalf("err = %v, want ErrCandidateNotFound", err)
	}
	if _, err := queue.Dismiss(ctx, "missing"); !errors.Is(err, ErrCandidateNotFound) {
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
	if _, err := queue.Approve(ctx, added[0].ID); err != nil {
		t.Fatalf("approve without storage: %v", err)
	}
	if err := queue.Clear(ctx); err != nil {
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
