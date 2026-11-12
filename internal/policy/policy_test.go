package policy

import (
	"strings"
	"testing"

	"zopresent/internal/confidence"
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
	if verdict.Reason != "the auto-projection switch is off" {
		t.Errorf("reason = %q, want the enabled gate (the PRD's order)", verdict.Reason)
	}
	if len(verdict.Failed) != 2 {
		t.Errorf("failed = %v, want both blocking gates reported", verdict.Failed)
	}
}

// The reason is what the panel shows, so it has to read as a cause. The gate
// label is the requirement ("no conflicting candidate supersedes it"), which
// sounds like a pass when it is the reason a card was held back.
func TestReasonSaysWhatWentWrongRatherThanWhatWasRequired(t *testing.T) {
	cases := []struct {
		name   string
		broken func(*Request)
		want   string
	}{
		{"live partial", func(r *Request) { r.Finalized = false }, "the transcript has not settled yet"},
		{"chapter only", func(r *Request) { r.VerseScoped = false }, "a whole chapter is too broad"},
		{"unknown book", func(r *Request) { r.BookID = 0 }, "the book could not be resolved"},
		{"unknown chapter", func(r *Request) { r.Chapter = 0 }, "the chapter could not be resolved"},
		{"missing passage", func(r *Request) { r.PassageAvailable = false }, "the passage was not found"},
		{"quotation", func(r *Request) { r.Explicit = false }, "quotation or a contextual match"},
		{"two candidates", func(r *Request) { r.SettledInSegment = 2 }, "2 references were heard"},
		{"repeated hearing", func(r *Request) { r.Repeats = 1 }, "heard again"},
		{"switched off", func(r *Request) { r.Enabled = false }, "the auto-projection switch is off"},
		{"paused", func(r *Request) { r.Paused = true }, "paused with F9"},
		{"emergency disable", func(r *Request) { r.Disabled = true }, "switched off with F12"},
		{"medium confidence", func(r *Request) { r.Band = confidence.BandMedium }, "not high enough"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			request := passing()
			tc.broken(&request)

			verdict := Evaluate(request)
			if !strings.Contains(verdict.Reason, tc.want) {
				t.Fatalf("reason = %q, want it to say %q", verdict.Reason, tc.want)
			}
			// The requirement wording stays on the gate itself, where the
			// panel's checklist shows it next to the pass/fail mark.
			if verdict.Gates[0].Label == verdict.Reason {
				t.Errorf("reason = %q, want the cause rather than the requirement", verdict.Reason)
			}
		})
	}
}

// A card held back by two references has to say how many competed, because
// the operator is choosing between them.
func TestConflictReasonCountsTheCompetingReferences(t *testing.T) {
	request := passing()
	request.SettledInSegment = 3

	verdict := Evaluate(request)
	if !strings.Contains(verdict.Reason, "3 references were heard") {
		t.Fatalf("reason = %q, want the number of competing references", verdict.Reason)
	}
	if !strings.Contains(verdict.Reason, "choose which one to send") {
		t.Fatalf("reason = %q, want it to say what to do about it", verdict.Reason)
	}
}
