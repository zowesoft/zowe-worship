package main

import (
	"strings"
	"testing"

	"zopresent/internal/bible"
	"zopresent/internal/policy"
)

// job29OnTheWall puts Job 29:15-17 in the live slide, which is what
// "established" means to the contextual detector.
func job29OnTheWall(t *testing.T, app *App) {
	t.Helper()
	book, ok := bible.LookupBook("job")
	if !ok {
		t.Fatal("no job book in canon")
	}
	if _, err := app.applied(app.presentation.ProjectScripture(app.context(), "kjv", book.ID, 29, 15, 17, false)); err != nil {
		t.Fatalf("project Job 29:15-17: %v", err)
	}
}

func contextualCandidateOf(t *testing.T, app *App) (bool, string) {
	t.Helper()
	for _, card := range app.detection.List(app.currentServiceID(app.context())) {
		if card.Method == "contextual-reference" {
			return true, card.Reference.String()
		}
	}
	return false, ""
}

func TestSettleSegmentQueuesButNeverAutoProjectsContextualVerse(t *testing.T) {
	app := newTestApp(t)
	installTestBible(t, app)
	if err := app.SetAutoProjection(true); err != nil {
		t.Fatalf("enable auto-projection: %v", err)
	}
	if err := app.SetAutoProjectionPaused(false); err != nil {
		t.Fatalf("resume auto-projection: %v", err)
	}
	job29OnTheWall(t, app)

	result, err := app.settleSegment("now read verse sixteen")
	if err != nil {
		t.Fatalf("settleSegment: %v", err)
	}
	found, ref := contextualCandidateOf(t, app)
	if !found {
		t.Fatalf("no contextual candidate queued; candidates = %+v", result.Candidates)
	}
	if !strings.HasPrefix(ref, "Job 29:16") {
		t.Fatalf("contextual reference = %q, want Job 29:16", ref)
	}
	if len(result.AutoProjected) != 0 {
		t.Fatalf("contextual reference auto-projected %v; it must never go out unasked", result.AutoProjected)
	}
}

func TestSettleSegmentKeepsContextualVerseOffAutoProjectionEvenAtHighConfidence(t *testing.T) {
	app := newTestApp(t)
	installTestBible(t, app)
	if err := app.SetAutoProjection(true); err != nil {
		t.Fatalf("enable auto-projection: %v", err)
	}
	if err := app.SetAutoProjectionPaused(false); err != nil {
		t.Fatalf("resume auto-projection: %v", err)
	}
	job29OnTheWall(t, app)
	if _, err := app.settleSegment("now read verse sixteen"); err != nil {
		t.Fatalf("settleSegment: %v", err)
	}
	found := false
	for _, card := range app.detection.List(app.currentServiceID(app.context())) {
		if card.Method != "contextual-reference" {
			continue
		}
		found = true
		verdict := app.autoProjectionVerdict(app.context(), card, 1)
		if verdict.Project {
			t.Fatalf("contextual card %s auto-projects; the intent gate must hold it", card.Reference)
		}
		foundIntent := false
		for _, id := range verdict.Failed {
			if id == policy.GateIntent {
				foundIntent = true
			}
		}
		if !foundIntent {
			t.Fatalf("verdict for %s failed %v, want the intent gate", card.Reference, verdict.Failed)
		}
	}
	if !found {
		t.Fatal("no contextual card queued to weigh")
	}
}

func TestSettleSegmentDoesNothingWithoutAnEstablishedPassage(t *testing.T) {
	app := newTestApp(t)
	installTestBible(t, app)

	result, err := app.settleSegment("now read verse sixteen")
	if err != nil {
		t.Fatalf("settleSegment: %v", err)
	}
	found, _ := contextualCandidateOf(t, app)
	if found {
		t.Fatal("contextual candidate queued without an established passage")
	}
	_ = result
}

func TestSettleSegmentDropsVerseThatDoesNotExistInTheChapter(t *testing.T) {
	app := newTestApp(t)
	installTestBible(t, app)
	job29OnTheWall(t, app)

	if _, err := app.settleSegment("now read verse one hundred and seventy six"); err != nil {
		t.Fatalf("settleSegment: %v", err)
	}
	found, _ := contextualCandidateOf(t, app)
	if found {
		t.Fatal("contextual candidate queued for a verse that is not in the chapter")
	}
}

func TestSettleSegmentExplicitReferenceSupersedesTheContextualHint(t *testing.T) {
	app := newTestApp(t)
	installTestBible(t, app)
	job29OnTheWall(t, app)

	// The preacher names the verse outright: the explicit card is the only
	// one, never a duplicate contextual card for the same verse.
	result, err := app.settleSegment("job chapter twenty nine verse sixteen")
	if err != nil {
		t.Fatalf("settleSegment: %v", err)
	}
	found, ref := contextualCandidateOf(t, app)
	if found {
		t.Fatalf("contextual card %s queued for an explicit reference", ref)
	}
	_ = result
}
