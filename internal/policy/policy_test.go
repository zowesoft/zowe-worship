package policy

import (
	"testing"

	"zowe-worship/internal/confidence"
)

// passing is the one candidate shape every gate accepts, so each test below can
// break exactly one requirement.
func passing() Request {
	return Request{
		Enabled:          true,
		Finalized:        true,
		Explicit:         true,
		BookID:           43,
		Chapter:          3,
		VerseScoped:      true,
		PassageAvailable: true,
		SettledInSegment: 1,
		Band:             confidence.BandHigh,
	}
}

func TestEvaluatePassesWhenEveryGateHolds(t *testing.T) {
	verdict := Evaluate(passing())
	if !verdict.Project {
		t.Fatalf("blocked by %v (%s), want projection", verdict.Failed, verdict.Reason)
	}
	if len(verdict.Gates) != 8 {
		t.Fatalf("gates = %d, want the eight PRD gates", len(verdict.Gates))
	}
	if verdict.Reason != "" {
		t.Errorf("reason = %q, want empty when everything passed", verdict.Reason)
	}
}

func TestEachGateBlocksOnItsOwn(t *testing.T) {
	cases := []struct {
		name   string
		gate   string
		broken func(*Request)
	}{
		{"live partial", GateFinalized, func(r *Request) { r.Finalized = false }},
		{"chapter only", GateUnambiguous, func(r *Request) { r.VerseScoped = false }},
		{"unknown book", GateUnambiguous, func(r *Request) { r.BookID = 0 }},
		{"missing passage", GatePassage, func(r *Request) { r.PassageAvailable = false }},
		{"quotation", GateIntent, func(r *Request) { r.Explicit = false }},
		{"two candidates", GateConflict, func(r *Request) { r.SettledInSegment = 2 }},
		{"repeated hearing", GateRepeat, func(r *Request) { r.Repeats = 1 }},
		{"switched off", GateEnabled, func(r *Request) { r.Enabled = false }},
		{"paused", GateEnabled, func(r *Request) { r.Paused = true }},
		{"emergency disable", GateEnabled, func(r *Request) { r.Disabled = true }},
		{"medium confidence", GateConfidence, func(r *Request) { r.Band = confidence.BandMedium }},
		{"low confidence", GateConfidence, func(r *Request) { r.Band = confidence.BandLow }},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			request := passing()
			tc.broken(&request)
			verdict := Evaluate(request)
			if verdict.Project {
				t.Fatal("projected despite a failing gate")
			}
			if len(verdict.Failed) != 1 || verdict.Failed[0] != tc.gate {
				t.Fatalf("failed = %v, want only %q", verdict.Failed, tc.gate)
			}
			if verdict.Reason == "" {
				t.Error("a refusal must explain itself")
			}
		})
	}
}

func TestReasonNamesTheFirstFailingGate(t *testing.T) {
	request := passing()
	request.Enabled = false
	request.Band = confidence.BandLow

	verdict := Evaluate(request)
	if verdict.Reason != "Auto-projection is on and not paused" {
		t.Errorf("reason = %q, want the enabled gate (the PRD's order)", verdict.Reason)
	}
	if len(verdict.Failed) != 2 {
		t.Errorf("failed = %v, want both blocking gates reported", verdict.Failed)
	}
}
