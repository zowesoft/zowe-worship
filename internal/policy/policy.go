// Package policy is the only place allowed to turn a detection into an
// automatic projection. It encodes the PRD's hybrid decision policy and its
// safety checklist as one named gate per requirement, so a blocked candidate
// stays in the operator's queue and the live slide is never touched.
package policy

import (
	"fmt"

	"zopresent/internal/confidence"
)

// Gate IDs are stable: tests name them, and the panel can explain which one
// blocked a candidate.
const (
	GateFinalized   = "finalized"
	GateUnambiguous = "unambiguous"
	GatePassage     = "passage"
	GateIntent      = "intent"
	GateConflict    = "conflict"
	GateRepeat      = "repeat"
	GateEnabled     = "enabled"
	GateConfidence  = "confidence"
)

// Gate is one checklist line: what it requires, what is wrong with the
// candidate when it does not hold, and whether the candidate met it.
type Gate struct {
	ID    string
	Label string
	// Cause is what went wrong, in the operator's words. The label is the
	// requirement ("no conflicting candidate supersedes it"), which reads
	// backwards when it is the reason a card was held back, so the panel
	// shows this instead.
	Cause  string
	Passed bool
}

// Request is everything the checklist needs about one finalized candidate. The
// caller has already resolved the passage, because that lookup is its data, not
// policy.
type Request struct {
	// Enabled is the operator's standing choice, Paused is the F9 pause, and
	// Disabled is the F12 emergency switch, which stops detections entirely.
	Enabled  bool
	Paused   bool
	Disabled bool

	// Finalized reports that the segment's final transcript settled this card;
	// a live partial is never eligible.
	Finalized bool
	// Explicit reports a spoken explicit reference rather than a contextual or
	// quotation match.
	Explicit bool
	// BookID and Chapter must resolve and VerseScoped must hold: a whole
	// chapter is too broad to put on the wall unasked.
	BookID      int
	Chapter     int
	VerseScoped bool
	// PassageAvailable reports that the selected translation returned verses.
	PassageAvailable bool
	// SettledInSegment is how many cards the segment settled. More than one
	// means two passages competed for the same moment.
	SettledInSegment int
	// Repeats counts folded repeat hearings: repeating a reference is exactly
	// what the duplicate gate exists to stop.
	Repeats int
	// Band is the candidate's confidence band.
	Band string
}

// Verdict is the checklist outcome. Reason names the cause of the first
// failing gate, and Gates keeps all of them so the panel can explain a
// refusal.
type Verdict struct {
	Project bool
	Gates   []Gate
	Reason  string
	Failed  []string
}

// Evaluate runs the PRD's safety gates in order.
func Evaluate(request Request) Verdict {
	gates := []Gate{
		{GateFinalized, "The final transcript settled the segment", "the transcript has not settled yet, so this card is still being written", request.Finalized},
		{GateUnambiguous, "The reference is unambiguous", unambiguousCause(request), request.BookID > 0 && request.Chapter > 0 && request.VerseScoped},
		{GatePassage, "The passage exists in the selected translation", "the passage was not found in the selected translation", request.PassageAvailable},
		{GateIntent, "An explicit scripture reference was spoken", "this is a quotation or a contextual match, not a spoken reference", request.Explicit},
		{GateConflict, "No conflicting candidate supersedes it", conflictCause(request), request.SettledInSegment == 1},
		{GateRepeat, "The hearing is not a repeat", "the same reference was heard again, so the hearings were folded into one card", request.Repeats == 0},
		{GateEnabled, "Auto-projection is on and not paused", enabledCause(request), request.Enabled && !request.Paused && !request.Disabled},
		{GateConfidence, "The confidence band is high", "the confidence is not high enough to send it unasked", request.Band == confidence.BandHigh},
	}

	verdict := Verdict{Gates: gates, Project: true}
	for _, gate := range gates {
		if gate.Passed {
			continue
		}
		verdict.Project = false
		if verdict.Reason == "" {
			verdict.Reason = gate.Cause
		}
		verdict.Failed = append(verdict.Failed, gate.ID)
	}
	return verdict
}

// unambiguousCause says which part of the reference was missing, so the
// operator is not left guessing what "unambiguous" wanted.
func unambiguousCause(request Request) string {
	switch {
	case request.BookID <= 0:
		return "the book could not be resolved"
	case request.Chapter <= 0:
		return "the chapter could not be resolved"
	default:
		return "a whole chapter is too broad to send on its own"
	}
}

// conflictCause names the other reference that competed for the same moment:
// the operator is the only one who knows which of them the preacher meant.
func conflictCause(request Request) string {
	if request.SettledInSegment > 1 {
		return fmt.Sprintf("%d references were heard in the same stretch of speech, so choose which one to send", request.SettledInSegment)
	}
	return "another reference was heard in the same stretch of speech, so choose which one to send"
}

// enabledCause says which switch stopped it: off, paused with F9 or disabled
// with F12.
func enabledCause(request Request) string {
	switch {
	case request.Disabled:
		return "detection is switched off with F12"
	case request.Paused:
		return "auto-projection is paused with F9"
	default:
		return "the auto-projection switch is off"
	}
}
