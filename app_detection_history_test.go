package main

import (
	"testing"

	"zowe-worship/internal/asr"
	"zowe-worship/internal/confidence"
	"zowe-worship/internal/detection"
	"zowe-worship/internal/scripture"
)

// queueDetection puts one reference in the pending queue the way DetectScripture
// would, without needing an engine on disk.
func queueDetection(t *testing.T, app *App, text string) string {
	t.Helper()

	reference, err := scripture.Parse(text)
	if err != nil {
		t.Fatalf("parse %q: %v", text, err)
	}
	added, err := app.detection.Add(text, []asr.Detection{{Reference: reference, Raw: text}}, app.currentServiceID(app.context()))
	if err != nil {
		t.Fatalf("queue %q: %v", text, err)
	}
	if len(added) != 1 {
		t.Fatalf("queue %q added %d candidates, want 1", text, len(added))
	}
	return added[0].ID
}

func TestDetectionCandidatesCarryMethodAndExplanation(t *testing.T) {
	app := newTestApp(t)
	queueDetection(t, app, "John 3:16")

	candidates, err := app.ListDetections()
	if err != nil {
		t.Fatalf("list: %v", err)
	}
	if len(candidates) != 1 {
		t.Fatalf("candidates = %d, want 1", len(candidates))
	}

	got := candidates[0]
	if got.Method != detection.MethodExplicitReference {
		t.Errorf("method = %q, want %q", got.Method, detection.MethodExplicitReference)
	}
	if got.Explanation != detection.ExplanationExplicitReference {
		t.Errorf("explanation = %q, want %q", got.Explanation, detection.ExplanationExplicitReference)
	}
	// A written chapter:verse form is the strongest phrasing the detector can
	// hear, so the panel shows a high band rather than a flat rule match.
	if got.Confidence != confidence.ScoreSeparator {
		t.Errorf("confidence = %v, want %v", got.Confidence, confidence.ScoreSeparator)
	}
	if got.ConfidenceBand != confidence.BandHigh {
		t.Errorf("band = %q, want %q", got.ConfidenceBand, confidence.BandHigh)
	}
}

func TestListDetectionHistoryRecountsDecisionsNewestFirst(t *testing.T) {
	app := newTestApp(t)
	ctx := app.context()

	// The decisions belong to the service that is running, so there has to be
	// one for the history to be filed under.
	if _, err := app.CreateService("Sunday service"); err != nil {
		t.Fatalf("create service: %v", err)
	}

	approved := queueDetection(t, app, "John 3:16")
	dismissed := queueDetection(t, app, "Romans 8:28")
	queueDetection(t, app, "Psalm 23")

	if _, err := app.detection.Approve(ctx, approved, app.currentServiceID(ctx)); err != nil {
		t.Fatalf("approve: %v", err)
	}
	if _, err := app.DismissDetection(dismissed); err != nil {
		t.Fatalf("dismiss: %v", err)
	}
	if _, err := app.ClearDetections(); err != nil {
		t.Fatalf("clear: %v", err)
	}

	history, err := app.ListDetectionHistory(10)
	if err != nil {
		t.Fatalf("history: %v", err)
	}
	if len(history) != 3 {
		t.Fatalf("history rows = %d, want 3", len(history))
	}

	// The three decisions land in the same second, so the rowid tie-break is
	// what makes "newest first" deterministic.
	want := []struct{ reference, decision string }{
		{"Psalms 23", "ignored"},
		{"Romans 8:28", "dismissed"},
		{"John 3:16", "approved"},
	}
	for i, entry := range want {
		if history[i].Reference != entry.reference || history[i].Decision != entry.decision {
			t.Errorf("row %d = (%q, %q), want (%q, %q)", i,
				history[i].Reference, history[i].Decision, entry.reference, entry.decision)
		}
		if history[i].Confidence <= 0 || history[i].ConfidenceBand == "" {
			t.Errorf("row %d confidence = %v (%q), want a scored hearing", i,
				history[i].Confidence, history[i].ConfidenceBand)
		}
		if history[i].CreatedAt == "" {
			t.Errorf("row %d has no timestamp", i)
		}
	}

	limited, err := app.ListDetectionHistory(1)
	if err != nil {
		t.Fatalf("limited history: %v", err)
	}
	if len(limited) != 1 || limited[0].Reference != "Psalms 23" {
		t.Fatalf("limited history = %+v, want the newest row only", limited)
	}
}

func TestListDetectionHistoryStartsEmpty(t *testing.T) {
	app := newTestApp(t)

	history, err := app.ListDetectionHistory(5)
	if err != nil {
		t.Fatalf("history: %v", err)
	}
	if history == nil {
		t.Fatal("history is nil; the panel expects a list")
	}
	if len(history) != 0 {
		t.Fatalf("history = %d rows, want 0", len(history))
	}
}
