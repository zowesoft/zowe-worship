// Package policy is the only place allowed to turn a detection into an
// automatic projection. It encodes the PRD's hybrid decision policy and its
// safety checklist as one named gate per requirement, so a blocked candidate
// stays in the operator's queue and the live slide is never touched.
package policy

import "zowe-worship/internal/confidence"

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

// Gate is one checklist line: what it requires and whether the candidate met it.
type Gate struct {
	ID     string
	Label  string
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

// Verdict is the checklist outcome. Reason names the first failing gate, and
// Gates keeps all of them so the panel can explain a refusal.
type Verdict struct {
	Project bool
	Gates   []Gate
	Reason  string
	Failed  []string
}

// Evaluate runs the PRD's safety gates in order.
func Evaluate(request Request) Verdict {
	gates := []Gate{
		{GateFinalized, "The final transcript settled the segment", request.Finalized},
		{GateUnambiguous, "The reference is unambiguous", request.BookID > 0 && request.Chapter > 0 && request.VerseScoped},
		{GatePassage, "The passage exists in the selected translation", request.PassageAvailable},
		{GateIntent, "An explicit scripture reference was spoken", request.Explicit},
		{GateConflict, "No conflicting candidate supersedes it", request.SettledInSegment == 1},
		{GateRepeat, "The hearing is not a repeat", request.Repeats == 0},
		{GateEnabled, "Auto-projection is on and not paused", request.Enabled && !request.Paused && !request.Disabled},
		{GateConfidence, "The confidence band is high", request.Band == confidence.BandHigh},
	}

	verdict := Verdict{Gates: gates, Project: true}
	for _, gate := range gates {
		if gate.Passed {
			continue
		}
		verdict.Project = false
		if verdict.Reason == "" {
			verdict.Reason = gate.Label
		}
		verdict.Failed = append(verdict.Failed, gate.ID)
	}
	return verdict
}
