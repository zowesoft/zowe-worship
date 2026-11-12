package main

import (
	"testing"

	"zopresent/internal/asr"
	"zopresent/internal/scripture"
)

// gate returns the checklist line with this ID, or nil.
func gate(candidate DetectionView, id string) *GateView {
	if candidate.AutoProjection == nil {
		return nil
	}
	for i := range candidate.AutoProjection.Gates {
		if candidate.AutoProjection.Gates[i].ID == id {
			return &candidate.AutoProjection.Gates[i]
		}
	}
	return nil
}

func TestHeldBackCandidateCarriesTheGateChecklist(t *testing.T) {
	app := autoReadyApp(t)
	// Bare numbers after the book name score medium, so the confidence gate
	// refuses the card while auto-projection is armed.
	autoCandidate(t, app, "John three sixteen", "John 3:16")

	candidates, err := app.ListDetections()
	if err != nil {
		t.Fatalf("list: %v", err)
	}
	if len(candidates) != 1 {
		t.Fatalf("candidates = %d, want 1", len(candidates))
	}
	card := candidates[0]
	if card.AutoProjection == nil {
		t.Fatal("a held-back card has no checklist; the operator cannot see why")
	}
	if card.AutoProjection.Reason == "" {
		t.Error("checklist has no reason")
	}
	if len(card.AutoProjection.Gates) != 8 {
		t.Fatalf("gates = %d, want the eight PRD gates", len(card.AutoProjection.Gates))
	}

	confidenceGate := gate(card, "confidence")
	if confidenceGate == nil {
		t.Fatal("checklist is missing the confidence gate")
	}
	if confidenceGate.Passed {
		t.Error("confidence gate passed for a medium-confidence card")
	}
	if confidenceGate.Label == "" {
		t.Error("confidence gate has no label to show")
	}
	if !gate(card, "finalized").Passed {
		t.Error("the settled card should pass the finalized gate")
	}
}

func TestPassingCandidateHasNoChecklist(t *testing.T) {
	app := autoReadyApp(t)
	autoCandidate(t, app, "Turn with me to John 3:16", "John 3:16")

	candidates, err := app.ListDetections()
	if err != nil {
		t.Fatalf("list: %v", err)
	}
	if len(candidates) != 1 {
		t.Fatalf("candidates = %d, want 1", len(candidates))
	}
	if candidates[0].AutoProjection != nil {
		t.Errorf("a card every gate accepts shows a refusal list: %+v", candidates[0].AutoProjection)
	}
}

func TestChecklistHiddenWhilePausedOrOff(t *testing.T) {
	app := autoReadyApp(t)
	if err := app.SetAutoProjectionPaused(true); err != nil {
		t.Fatalf("pause: %v", err)
	}
	autoCandidate(t, app, "John three sixteen", "John 3:16")

	candidates, err := app.ListDetections()
	if err != nil {
		t.Fatalf("list: %v", err)
	}
	if candidates[0].AutoProjection != nil {
		t.Fatal("a paused switch is not a gate refusal; the checklist should stay hidden")
	}
}

func TestChecklistNamesCompetingCandidates(t *testing.T) {
	app := autoReadyApp(t)
	john, err := scripture.Parse("John 3:16")
	if err != nil {
		t.Fatalf("parse John: %v", err)
	}
	romans, err := scripture.Parse("Romans 8:28")
	if err != nil {
		t.Fatalf("parse Romans: %v", err)
	}
	added, err := app.detection.Add("John 3:16 and Romans 8:28", []asr.Detection{
		{Reference: john, Raw: "John 3:16"},
		{Reference: romans, Raw: "Romans 8:28"},
	}, app.currentServiceID(app.context()))
	if err != nil {
		t.Fatalf("queue: %v", err)
	}
	if len(added) != 2 {
		t.Fatalf("added = %d, want 2", len(added))
	}

	candidates, err := app.ListDetections()
	if err != nil {
		t.Fatalf("list: %v", err)
	}
	for _, card := range candidates {
		if gate(card, "conflict") == nil {
			t.Fatalf("%s has no conflict gate", card.Reference)
		}
		if gate(card, "conflict").Passed {
			t.Errorf("%s passed the conflict gate despite a second candidate", card.Reference)
		}
	}
}
