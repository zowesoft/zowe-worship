package main

import (
	"testing"
)

// ProjectDetection is the operator's explicit send from a queued card, so it
// must not depend on the auto-projection switch or its pause.
func TestProjectDetectionSendsTheCandidateLive(t *testing.T) {
	app := newTestApp(t)
	installTestBible(t, app)
	candidate := autoCandidate(t, app, "Turn with me to John 3:16", "John 3:16")

	view, err := app.ProjectDetection(candidate.ID)
	if err != nil {
		t.Fatalf("project detection: %v", err)
	}
	if view.Live == nil || view.Live.Reference != "John 3:16" {
		t.Fatalf("live = %+v, want John 3:16 on air", view.Live)
	}
	if pending := app.detection.List(); len(pending) != 0 {
		t.Fatalf("queue holds %d cards, want the projected one removed", len(pending))
	}
	if decision := autoHistoryDecision(t, app, candidate.ID); decision != "approved" {
		t.Errorf("history decision = %q, want approved", decision)
	}

	state, err := app.GetPresentationState()
	if err != nil {
		t.Fatalf("presentation state: %v", err)
	}
	if state.Live == nil || state.Live.Reference != "John 3:16" {
		t.Fatalf("state live = %+v, want the passage still on air", state.Live)
	}
}

// A verse that is not in the installed translation must fail before the room
// changes, or the operator would have a dark or empty slide and no card left.
func TestProjectDetectionLeavesTheCardWhenThePassageIsMissing(t *testing.T) {
	app := newTestApp(t)
	installTestBible(t, app)
	candidate := autoCandidate(t, app, "Open to John nine", "John 9:99")

	if _, err := app.ProjectDetection(candidate.ID); err == nil {
		t.Fatal("project detection succeeded for a passage the translation does not hold")
	}
	if pending := app.detection.List(); len(pending) != 1 || pending[0].ID != candidate.ID {
		t.Fatalf("queue = %+v, want the card left for a retry", pending)
	}
	state, err := app.GetPresentationState()
	if err != nil {
		t.Fatalf("presentation state: %v", err)
	}
	if state.Live != nil {
		t.Fatalf("live = %+v, want the audience unchanged", state.Live)
	}
}

func TestProjectDetectionRejectsAnIDNoLongerQueued(t *testing.T) {
	app := newTestApp(t)
	installTestBible(t, app)

	if _, err := app.ProjectDetection("missing"); err == nil {
		t.Fatal("project detection accepted an id that is not pending")
	}
	state, err := app.GetPresentationState()
	if err != nil {
		t.Fatalf("presentation state: %v", err)
	}
	if state.Live != nil {
		t.Fatalf("live = %+v, want the audience unchanged", state.Live)
	}
}
