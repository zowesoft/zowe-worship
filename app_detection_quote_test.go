package main

import (
	"context"
	"testing"

	"zopresent/internal/bible"
	"zopresent/internal/detection"
)

// installQuoteBible installs Philippians 4:6-7 plus a distractor, so the
// spoken-quote path has a real full-text index to match an unnamed quotation
// against through the same storage the studio uses.
func installQuoteBible(t *testing.T, app *App) {
	t.Helper()

	_, err := app.bible.Install(context.Background(), bible.TranslationData{
		TranslationMeta: bible.TranslationMeta{
			ID: "kjv", Name: "King James Version", Abbreviation: "KJV", Language: "en", Version: "1.0.0",
		},
		Verses: []bible.Verse{
			{BookID: 50, Chapter: 4, Number: 6, Text: "Be careful for nothing; but in every thing by prayer and supplication with thanksgiving let your requests be made known unto God."},
			{BookID: 50, Chapter: 4, Number: 7, Text: "And the peace of God, which passeth all understanding, shall keep your hearts and minds through Christ Jesus."},
			{BookID: 43, Chapter: 3, Number: 16, Text: "For God so loved the world, that he gave his only begotten Son."},
		},
	})
	if err != nil {
		t.Fatalf("install quote bible: %v", err)
	}
}

const unnamedPhilippiansQuote = "be careful for nothing but in every thing by prayer and supplication with thanksgiving " +
	"let your requests be made known unto God and the peace of God which passeth all understanding " +
	"shall keep your hearts and minds through Christ Jesus"

// The preacher quotes Philippians 4:6-7 word for word without naming it. The
// explicit detector finds nothing; the full-text index carries the passage into
// the queue as a spoken quote, merged into the one range that was said.
func TestSpokenQuoteQueuesAnUnnamedPassage(t *testing.T) {
	app := newTestApp(t)
	installQuoteBible(t, app)

	result := runSegment(t, app, unnamedPhilippiansQuote)

	var quote *DetectionView
	for i := range result.Candidates {
		if result.Candidates[i].Method == detection.MethodSpokenQuote {
			quote = &result.Candidates[i]
		}
	}
	if quote == nil {
		t.Fatalf("no spoken-quote candidate in %+v", result.Candidates)
	}
	if quote.Reference != "Philippians 4:6-7" {
		t.Errorf("reference = %q, want the merged Philippians 4:6-7", quote.Reference)
	}
	if quote.BookID != 50 || quote.Chapter != 4 || quote.VerseStart != 6 || quote.VerseEnd != 7 {
		t.Errorf("geometry = %+v, want Philippians 4:6-7", quote)
	}
	if !quote.Finalized {
		t.Error("the quote card is not finalized after a settled transcript")
	}
	if quote.ConfidenceBand != "medium" {
		t.Errorf("band = %q, want medium: a quote never reads as a heard reference", quote.ConfidenceBand)
	}
	if quote.Explanation != detection.ExplanationSpokenQuote {
		t.Errorf("explanation = %q, want %q", quote.Explanation, detection.ExplanationSpokenQuote)
	}
	if len(result.AutoProjected) != 0 {
		t.Errorf("auto-projected %v, want none while the switch is off", result.AutoProjected)
	}
}

// Even with auto-projection armed and resumed, a quotation is held back by the
// intent gate: only a named reference may reach the room without the operator.
func TestSpokenQuoteIsHeldBackWhenAutoProjectionIsArmed(t *testing.T) {
	app := newTestApp(t)
	installQuoteBible(t, app)
	if err := app.SetAutoProjection(true); err != nil {
		t.Fatalf("enable auto-projection: %v", err)
	}
	if err := app.SetAutoProjectionPaused(false); err != nil {
		t.Fatalf("resume auto-projection: %v", err)
	}

	result := runSegment(t, app, unnamedPhilippiansQuote)
	if len(result.AutoProjected) != 0 {
		t.Fatalf("auto-projected %v, want a quotation kept for the operator", result.AutoProjected)
	}

	state, err := app.GetPresentationState()
	if err != nil {
		t.Fatalf("presentation state: %v", err)
	}
	if state.Live != nil {
		t.Fatalf("live = %+v, want the display untouched by a quote", state.Live)
	}

	var quote *DetectionView
	for i := range result.Candidates {
		if result.Candidates[i].Method == detection.MethodSpokenQuote {
			quote = &result.Candidates[i]
		}
	}
	if quote == nil {
		t.Fatalf("no spoken-quote candidate in %+v", result.Candidates)
	}
	if quote.AutoProjection == nil {
		t.Fatal("an armed, held-back quote must explain itself with a checklist")
	}
	intentFailed := false
	for _, gate := range quote.AutoProjection.Gates {
		if gate.ID == "intent" && !gate.Passed {
			intentFailed = true
		}
	}
	if !intentFailed {
		t.Errorf("checklist = %+v, want the intent gate to fail for a quote", quote.AutoProjection)
	}
}

// When the preacher names the passage and then quotes it, the explicit reference
// wins: the quote path must not queue the same passage a second time.
func TestExplicitReferenceSuppressesTheSpokenQuote(t *testing.T) {
	app := newTestApp(t)
	installQuoteBible(t, app)

	transcript := "but listen to what the Bible says in Philippians 4 verses 6 and 7 " +
		"be careful for nothing but in every thing by prayer and supplication with thanksgiving " +
		"let your requests be made known unto God"
	result := runSegment(t, app, transcript)

	philippians := 0
	for _, candidate := range result.Candidates {
		if candidate.BookID != 50 || candidate.Chapter != 4 {
			continue
		}
		philippians++
		if candidate.Method != detection.MethodExplicitReference {
			t.Errorf("Philippians card method = %q, want the explicit reference to win", candidate.Method)
		}
	}
	if philippians != 1 {
		t.Fatalf("Philippians cards = %d, want exactly one (the quote suppressed)", philippians)
	}
}
