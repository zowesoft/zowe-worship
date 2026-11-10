package main

import "testing"

// runSegment finishes what DetectScripture does after transcription, so a test
// can drive consecutive analyses without an engine on disk.
func runSegment(t *testing.T, app *App, transcript string) DetectionResult {
	t.Helper()

	app.detection.BeginSegment()
	result, err := app.settleSegment(transcript)
	if err != nil {
		t.Fatalf("settle %q: %v", transcript, err)
	}
	return result
}

func TestTwoConsecutiveAnalysesFoldTheRepeat(t *testing.T) {
	app := newTestApp(t)
	installTestBible(t, app)

	first := runSegment(t, app, "Turn with me to John chapter three verse sixteen")
	if first.Segment != 1 {
		t.Fatalf("first segment = %d, want 1", first.Segment)
	}
	if first.Repeats != 0 {
		t.Errorf("first repeats = %d, want 0", first.Repeats)
	}
	if len(first.Candidates) != 1 {
		t.Fatalf("first candidates = %d, want 1", len(first.Candidates))
	}
	opening := first.Candidates[0]
	if !opening.Finalized {
		t.Error("the first card is not finalized")
	}
	if opening.Segment != 1 {
		t.Errorf("first card segment = %d, want 1", opening.Segment)
	}
	if opening.ConfidenceBand != "high" {
		t.Errorf("first card band = %q, want high for spoken connectors", opening.ConfidenceBand)
	}
	if len(first.AutoProjected) != 0 {
		t.Errorf("auto-projected %v while the switch is off", first.AutoProjected)
	}

	second := runSegment(t, app, "John 3:16 again")
	if second.Segment != 2 {
		t.Fatalf("second segment = %d, want 2", second.Segment)
	}
	if second.Repeats != 1 {
		t.Errorf("second repeats = %d, want 1 folded hearing", second.Repeats)
	}
	if len(second.Candidates) != 1 {
		t.Fatalf("candidates = %d, want the repeat folded into the existing card", len(second.Candidates))
	}

	folded := second.Candidates[0]
	if folded.ID != opening.ID {
		t.Error("the repeat changed the card ID; the operator would see two cards")
	}
	if folded.Repeats != 1 {
		t.Errorf("card repeats = %d, want 1", folded.Repeats)
	}
	if folded.Version < 2 {
		t.Errorf("card version = %d, want the repeat counted", folded.Version)
	}
	if !folded.Finalized {
		t.Error("the folded card lost its finalized state")
	}
	if folded.Segment != 1 {
		t.Errorf("card segment = %d, want the original segment", folded.Segment)
	}
	if folded.Transcript == opening.Transcript {
		t.Error("the folded card kept the old transcript")
	}

	// Folding is not an operator decision, so nothing is written to history.
	var rows int
	if err := app.core.QueryRow(`SELECT COUNT(*) FROM detection_history`).Scan(&rows); err != nil {
		t.Fatalf("count history: %v", err)
	}
	if rows != 0 {
		t.Errorf("detection_history rows = %d, want 0 for an untouched queue", rows)
	}
}
