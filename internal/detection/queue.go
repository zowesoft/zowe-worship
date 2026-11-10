// Package detection holds the candidate queue between spoken-reference
// detection and operator confirmation. A candidate never reaches the room on
// its own; this queue is that gate.
package detection

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"sync"
	"time"

	"zowe-worship/internal/asr"
	"zowe-worship/internal/confidence"
	"zowe-worship/internal/database"
	"zowe-worship/internal/scripture"
)

// ErrCandidateNotFound covers both an id already decided and one lost to a
// restart.
var ErrCandidateNotFound = errors.New("detection candidate not found")

// Candidate is one detected reference waiting for a decision.
type Candidate struct {
	ID         string
	Transcript string
	Reference  scripture.Reference
	// Raw is what the detector parsed, useful when an ASR near miss needs
	// explaining.
	Raw string
	// Method names how the passage was found. Only explicit references exist
	// today; contextual and quotation matches get their own values later.
	Method string
	// Confidence is the detector's score for the phrasing it matched, banded by
	// internal/confidence. It is evidence strength, not measured accuracy.
	Confidence float64
	// Explanation is the sentence the panel shows under the candidate.
	Explanation string
	// Segment is the 1-based analysis segment that produced the hearing. Cards
	// live across segments; partials do not.
	Segment int
	// Version grows with each refinement of the hearing, live or finalized.
	Version int
	// Finalized reports that the segment's final transcript settled this card.
	Finalized   bool
	FinalizedAt time.Time
	// Repeats counts identical hearings folded into this card while it stayed
	// pending, so the panel can show the preacher returned to the passage.
	Repeats     int
	LastHeardAt time.Time
	CreatedAt   time.Time
}

// RepeatWindow is how long an identical hearing folds into the pending card
// instead of stacking a second one. It covers an operator deciding on the card
// while still letting a genuine return to the passage be its own card.
const RepeatWindow = 2 * time.Minute

// The rule-based detector produces one kind of match; its score comes from
// internal/confidence, which weighs the phrasing behind the hearing.
const (
	MethodExplicitReference      = "explicit-reference"
	ExplanationExplicitReference = "Explicit reference detected"
)

// newCandidate fills the fields every detection type shares, so Add and
// MergeLive cannot drift apart.
func newCandidate(transcript string, detection asr.Detection, now time.Time) Candidate {
	return Candidate{
		Transcript:  transcript,
		Reference:   detection.Reference,
		Raw:         detection.Raw,
		Method:      MethodExplicitReference,
		Confidence:  confidence.Evaluate(transcript, detection.Reference),
		Explanation: ExplanationExplicitReference,
		CreatedAt:   now,
	}
}

// decision values must stay inside the CHECK constraint on detection_history.
const (
	decisionApproved  = "approved"
	decisionDismissed = "dismissed"
	decisionIgnored   = "ignored"
)

// Queue keeps pending candidates in memory and records decisions in
// detection_history. Pending candidates are not persisted: after a restart the
// room has moved on, and a stale candidate inviting a wrong projection is worse
// than an empty queue.
type Queue struct {
	mu      sync.Mutex
	pending []Candidate
	// segment numbers analysis calls; zero means no segment is open. Tests set
	// now to keep the repeat window deterministic.
	segment  int
	segments int
	now      func() time.Time
	db       *sql.DB
}

// A nil db drops decisions instead of recording them.
func New(db *sql.DB) *Queue {
	return &Queue{db: db}
}

func (q *Queue) clock() time.Time {
	if q.now != nil {
		return q.now().UTC().Truncate(time.Second)
	}
	return time.Now().UTC().Truncate(time.Second)
}

// BeginSegment opens a new analysis segment and returns its 1-based number.
func (q *Queue) BeginSegment() int {
	q.mu.Lock()
	defer q.mu.Unlock()

	q.segments++
	q.segment = q.segments
	return q.segment
}

// CurrentSegment reports the open segment, or zero between analyses.
func (q *Queue) CurrentSegment() int {
	q.mu.Lock()
	defer q.mu.Unlock()
	return q.segment
}

func (q *Queue) Add(transcript string, detections []asr.Detection) ([]Candidate, error) {
	q.mu.Lock()
	defer q.mu.Unlock()

	now := q.clock()
	segment := q.segment
	if segment < 1 {
		q.segments++
		segment = q.segments
	}
	added := make([]Candidate, 0, len(detections))
	for _, detection := range detections {
		candidate := newCandidate(transcript, detection, now)
		candidate.Segment = segment
		candidate.Version = 1
		candidate.Finalized = true
		candidate.FinalizedAt = now
		candidate.LastHeardAt = now
		id, err := database.NewID()
		if err != nil {
			return nil, fmt.Errorf("new candidate id: %w", err)
		}
		candidate.ID = id
		q.pending = append(q.pending, candidate)
		added = append(added, candidate)
	}
	return added, nil
}

// MergeLive replaces the pending candidate for the same passage in place, so a
// hearing that grows or narrows as the sentence continues updates the card
// instead of stacking a second one. Non-overlapping passages stay separate.
func (q *Queue) MergeLive(transcript string, detections []asr.Detection) ([]Candidate, error) {
	q.mu.Lock()
	defer q.mu.Unlock()

	now := q.clock()
	if q.segment < 1 {
		q.segments++
		q.segment = q.segments
	}
	segment := q.segment
	added := make([]Candidate, 0, len(detections))
	for _, detection := range detections {
		replaced := -1
		for i, candidate := range q.pending {
			// Partials only refine their own segment's unfinished cards.
			if candidate.Segment != segment || candidate.Finalized {
				continue
			}
			if candidate.Reference.Book.ID != detection.Reference.Book.ID ||
				candidate.Reference.Chapter != detection.Reference.Chapter {
				continue
			}
			if overlaps(candidate.Reference, detection.Reference) {
				replaced = i
				break
			}
		}
		if replaced >= 0 {
			updated := q.pending[replaced]
			updated.Transcript = transcript
			updated.Raw = detection.Raw
			updated.Reference = detection.Reference
			updated.Method = MethodExplicitReference
			updated.Confidence = confidence.Evaluate(transcript, detection.Reference)
			updated.Explanation = ExplanationExplicitReference
			updated.Version++
			updated.LastHeardAt = now
			q.pending[replaced] = updated
			added = append(added, updated)
			continue
		}
		candidate := newCandidate(transcript, detection, now)
		candidate.Segment = segment
		candidate.Version = 1
		candidate.LastHeardAt = now
		id, err := database.NewID()
		if err != nil {
			return nil, fmt.Errorf("new candidate id: %w", err)
		}
		candidate.ID = id
		q.pending = append(q.pending, candidate)
		added = append(added, candidate)
	}
	return added, nil
}

// FinalizeResult reports what one segment's final transcript settled.
type FinalizeResult struct {
	Segment   int
	Finalized int
	// Repeats counts hearings folded into an already finalized pending card
	// instead of queuing a duplicate.
	Repeats int
}

// Finalize closes the open segment on its final transcript. Live candidates
// settle in place, keeping their IDs, and an identical hearing that arrives
// while an earlier card is still pending folds into it rather than stacking a
// duplicate the operator has to dismiss twice. Folding is not an operator
// decision, so it is not written to detection_history.
func (q *Queue) Finalize(transcript string, detections []asr.Detection) (FinalizeResult, error) {
	q.mu.Lock()
	defer q.mu.Unlock()

	if q.segment < 1 {
		q.segments++
		q.segment = q.segments
	}
	segment := q.segment
	now := q.clock()
	result := FinalizeResult{Segment: segment}

	for _, detection := range detections {
		// A partial from this segment grows into the finalized card under the
		// same ID, so the operator never sees the card move.
		live := -1
		for i, candidate := range q.pending {
			if candidate.Segment != segment || candidate.Finalized {
				continue
			}
			if overlaps(candidate.Reference, detection.Reference) {
				live = i
				break
			}
		}
		if live >= 0 {
			updated := q.pending[live]
			updated.Transcript = transcript
			updated.Raw = detection.Raw
			updated.Reference = detection.Reference
			updated.Confidence = confidence.Evaluate(transcript, detection.Reference)
			updated.Version++
			updated.Finalized = true
			updated.FinalizedAt = now
			updated.LastHeardAt = now
			q.pending[live] = updated
			result.Finalized++
			continue
		}

		if repeated, folded := q.foldRepeat(detection, transcript, now); folded {
			if repeated {
				result.Repeats++
			}
			continue
		}

		candidate := newCandidate(transcript, detection, now)
		candidate.Segment = segment
		candidate.Version = 1
		candidate.Finalized = true
		candidate.FinalizedAt = now
		candidate.LastHeardAt = now
		id, err := database.NewID()
		if err != nil {
			return FinalizeResult{}, fmt.Errorf("new candidate id: %w", err)
		}
		candidate.ID = id
		q.pending = append(q.pending, candidate)
		result.Finalized++
	}

	// A partial the final transcript dropped was still a real hearing; settle it
	// rather than leave a card looking live forever.
	for i := range q.pending {
		if q.pending[i].Segment == segment && !q.pending[i].Finalized {
			q.pending[i].Finalized = true
			q.pending[i].FinalizedAt = now
			result.Finalized++
		}
	}
	q.segment = 0
	return result, nil
}

// CloseSegment ends the open segment without a final transcript, settling any
// partials so a failed analysis cannot leave a card looking live.
func (q *Queue) CloseSegment() {
	q.mu.Lock()
	defer q.mu.Unlock()

	if q.segment < 1 {
		return
	}
	now := q.clock()
	for i := range q.pending {
		if q.pending[i].Segment == q.segment && !q.pending[i].Finalized {
			q.pending[i].Finalized = true
			q.pending[i].FinalizedAt = now
		}
	}
	q.segment = 0
}

// foldRepeat updates a pending card that heard this passage within the repeat
// window. It reports whether the reference was identical (a repeat) rather than
// a refinement of the same hearing.
func (q *Queue) foldRepeat(detection asr.Detection, transcript string, now time.Time) (repeated, folded bool) {
	for i := range q.pending {
		candidate := q.pending[i]
		if !candidate.Finalized || !overlaps(candidate.Reference, detection.Reference) {
			continue
		}
		heard := candidate.FinalizedAt
		if heard.IsZero() {
			heard = candidate.LastHeardAt
		}
		if now.Sub(heard) >= RepeatWindow {
			continue
		}
		repeated = candidate.Reference.String() == detection.Reference.String()
		if repeated {
			candidate.Repeats++
		}
		candidate.Transcript = transcript
		candidate.Raw = detection.Raw
		candidate.Reference = detection.Reference
		candidate.Confidence = confidence.Evaluate(transcript, detection.Reference)
		candidate.Version++
		candidate.LastHeardAt = now
		q.pending[i] = candidate
		return repeated, true
	}
	return false, false
}

// overlaps tests whether one hearing refines another. A zero end is the whole
// chapter.
func overlaps(a, b scripture.Reference) bool {
	aEnd, bEnd := a.VerseEnd, b.VerseEnd
	if aEnd == 0 {
		aEnd = int(^uint(0) >> 1)
	}
	if bEnd == 0 {
		bEnd = int(^uint(0) >> 1)
	}
	return a.VerseStart <= bEnd && b.VerseStart <= aEnd
}

func (q *Queue) List() []Candidate {
	q.mu.Lock()
	defer q.mu.Unlock()

	pending := make([]Candidate, len(q.pending))
	copy(pending, q.pending)
	return pending
}

// Approve records the decision. The caller projects afterwards; approval alone
// never touches the display.
func (q *Queue) Approve(ctx context.Context, id string) (Candidate, error) {
	return q.decide(ctx, id, decisionApproved)
}

func (q *Queue) Dismiss(ctx context.Context, id string) (Candidate, error) {
	return q.decide(ctx, id, decisionDismissed)
}

// Clear records the dropped candidates as ignored, so history still shows what
// was heard and let go.
func (q *Queue) Clear(ctx context.Context) error {
	q.mu.Lock()
	cleared := q.pending
	q.pending = nil
	q.mu.Unlock()

	for _, candidate := range cleared {
		if err := q.record(ctx, candidate, decisionIgnored); err != nil {
			return err
		}
	}
	return nil
}

func (q *Queue) decide(ctx context.Context, id, decision string) (Candidate, error) {
	q.mu.Lock()
	index := -1
	for i, candidate := range q.pending {
		if candidate.ID == id {
			index = i
			break
		}
	}
	if index < 0 {
		q.mu.Unlock()
		return Candidate{}, ErrCandidateNotFound
	}
	candidate := q.pending[index]
	q.pending = append(q.pending[:index], q.pending[index+1:]...)
	q.mu.Unlock()

	if err := q.record(ctx, candidate, decision); err != nil {
		return Candidate{}, err
	}
	return candidate, nil
}

// A record failure is reported, but the decision has already happened.
func (q *Queue) record(ctx context.Context, candidate Candidate, decision string) error {
	if q.db == nil {
		return nil
	}
	_, err := q.db.ExecContext(ctx, `
		INSERT INTO detection_history (id, transcript, reference, confidence, decision, created_at)
		VALUES (?, ?, ?, ?, ?, ?)`,
		candidate.ID, candidate.Transcript, candidate.Reference.String(), candidate.Confidence, decision,
		candidate.CreatedAt.Format(time.RFC3339),
	)
	if err != nil {
		return fmt.Errorf("record detection decision: %w", err)
	}
	return nil
}
