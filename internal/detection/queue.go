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

	"zopresent/internal/asr"
	"zopresent/internal/confidence"
	"zopresent/internal/database"
	"zopresent/internal/scripture"
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
	// Method names how the passage was found: an explicit spoken reference or a
	// quotation matched against the translation's full-text index.
	Method string
	// Confidence is the detector's score for the phrasing it matched, banded by
	// internal/confidence. It is evidence strength, not measured accuracy.
	Confidence float64
	// Explanation is the sentence the panel shows under the candidate.
	Explanation string
	// Segment is the 1-based analysis segment that produced the hearing. Cards
	// live across segments; partials do not.
	Segment int
	// ServiceID is the service that was running when the hearing came in. A
	// card belongs to that service: switching to another one does not show it,
	// and it cannot be sent live into a service it was not heard in.
	ServiceID string
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

// The detector produces two kinds of match. An explicit reference is scored from
// its phrasing by internal/confidence; a spoken quotation carries the overlap
// score its full-text match already earned.
const (
	MethodExplicitReference      = "explicit-reference"
	ExplanationExplicitReference = "Explicit reference detected"
	MethodSpokenQuote            = "spoken-quote"
	ExplanationSpokenQuote       = "Spoken quote matched in the selected translation"
)

// scoreDetection resolves the method, explanation and confidence for one
// hearing. A quotation keeps the score its match earned; anything else is an
// explicit reference, scored from the phrasing behind it. Sharing this keeps Add,
// MergeLive, Finalize and foldRepeat from drifting apart, and lets an explicit
// re-hearing of a quote card upgrade it rather than leave the weaker label.
func scoreDetection(transcript string, detection asr.Detection) (method, explanation string, score float64) {
	if detection.Method == MethodSpokenQuote {
		return MethodSpokenQuote, ExplanationSpokenQuote, detection.Confidence
	}
	if detection.Method == MethodContextualReference {
		return MethodContextualReference, ExplanationContextualReference, detection.Confidence
	}
	return MethodExplicitReference, ExplanationExplicitReference, confidence.Evaluate(transcript, detection.Reference)
}

// newCandidate fills the fields every detection type shares, so Add and
// MergeLive cannot drift apart.
func newCandidate(transcript string, detection asr.Detection, now time.Time, serviceID string) Candidate {
	method, explanation, score := scoreDetection(transcript, detection)
	return Candidate{
		Transcript:  transcript,
		Reference:   detection.Reference,
		Raw:         detection.Raw,
		Method:      method,
		Confidence:  score,
		Explanation: explanation,
		ServiceID:   serviceID,
		CreatedAt:   now,
	}
}

// decision values must stay inside the CHECK constraint on detection_history.
const (
	decisionApproved  = "approved"
	decisionDismissed = "dismissed"
	decisionIgnored   = "ignored"
	// decisionAutoProjected is written by the policy layer when every safety
	// gate passed and the operator was not asked.
	decisionAutoProjected = "auto_projected"
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
	// activeService is the service the queue is currently hearing for. A
	// change of service closes the open segment, so the partials of the service
	// before it do not stay looking live.
	activeService string
	now           func() time.Time
	db            *sql.DB
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

// Add queues the cards of one hearing. The service is the one that was running
// when the words were heard, and the card belongs to it.
func (q *Queue) Add(transcript string, detections []asr.Detection, serviceID string) ([]Candidate, error) {
	q.mu.Lock()
	defer q.mu.Unlock()

	q.switchService(serviceID)
	now := q.clock()
	segment := q.segment
	if segment < 1 {
		q.segments++
		segment = q.segments
	}
	added := make([]Candidate, 0, len(detections))
	for _, detection := range detections {
		candidate := newCandidate(transcript, detection, now, serviceID)
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
// Only the cards of the running service are refined.
func (q *Queue) MergeLive(transcript string, detections []asr.Detection, serviceID string) ([]Candidate, error) {
	q.mu.Lock()
	defer q.mu.Unlock()

	q.switchService(serviceID)
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
			// Partials only refine their own segment's unfinished cards, and
			// only the ones of the service they were heard in.
			if candidate.Segment != segment || candidate.Finalized || candidate.ServiceID != serviceID {
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
			updated.Method, updated.Explanation, updated.Confidence = scoreDetection(transcript, detection)
			updated.Version++
			updated.LastHeardAt = now
			q.pending[replaced] = updated
			added = append(added, updated)
			continue
		}
		candidate := newCandidate(transcript, detection, now, serviceID)
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
	// Settled lists the cards this final transcript settled. They keep their
	// IDs; the policy layer weighs each one for auto-projection.
	Settled []Candidate
	// Repeats counts hearings folded into an already finalized pending card
	// instead of queuing a duplicate.
	Repeats int
}

// Finalize closes the open segment on its final transcript. Live candidates
// settle in place, keeping their IDs, and an identical hearing that arrives
// while an earlier card is still pending folds into it rather than stacking a
// duplicate the operator has to dismiss twice. Folding is not an operator
// decision, so it is not written to detection_history. Only the cards of the
// running service are settled by it.
func (q *Queue) Finalize(transcript string, detections []asr.Detection, serviceID string) (FinalizeResult, error) {
	q.mu.Lock()
	defer q.mu.Unlock()

	q.switchService(serviceID)
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
			if candidate.Segment != segment || candidate.Finalized || candidate.ServiceID != serviceID {
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
			updated.Method, updated.Explanation, updated.Confidence = scoreDetection(transcript, detection)
			updated.Version++
			updated.Finalized = true
			updated.FinalizedAt = now
			updated.LastHeardAt = now
			q.pending[live] = updated
			result.Finalized++
			result.Settled = append(result.Settled, updated)
			continue
		}

		if repeated, folded := q.foldRepeat(detection, transcript, now, serviceID); folded {
			if repeated {
				result.Repeats++
			}
			continue
		}

		candidate := newCandidate(transcript, detection, now, serviceID)
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
		result.Settled = append(result.Settled, candidate)
	}

	// A partial the final transcript dropped was still a real hearing; settle it
	// rather than leave a card looking live forever.
	for i := range q.pending {
		if q.pending[i].Segment == segment && !q.pending[i].Finalized && q.pending[i].ServiceID == serviceID {
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
// a refinement of the same hearing. Only a card of the same service can be
// folded into: the same words heard in another service are that service's
// hearing.
func (q *Queue) foldRepeat(detection asr.Detection, transcript string, now time.Time, serviceID string) (repeated, folded bool) {
	for i := range q.pending {
		candidate := q.pending[i]
		if !candidate.Finalized || candidate.ServiceID != serviceID || !overlaps(candidate.Reference, detection.Reference) {
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
		candidate.Method, candidate.Explanation, candidate.Confidence = scoreDetection(transcript, detection)
		candidate.Version++
		candidate.LastHeardAt = now
		q.pending[i] = candidate
		return repeated, true
	}
	return false, false
}

// overlaps tests whether one hearing refines another. A zero end is the whole
// chapter. The book and chapter must match first: verse numbers alone are not
// an address, or "Philippians 4:6-7" would fold into "1 Peter 5:7" and a whole
// sermon would collapse into a single pending card.
func overlaps(a, b scripture.Reference) bool {
	if a.Book.ID != b.Book.ID || a.Chapter != b.Chapter {
		return false
	}
	aEnd, bEnd := a.VerseEnd, b.VerseEnd
	if aEnd == 0 {
		aEnd = int(^uint(0) >> 1)
	}
	if bEnd == 0 {
		bEnd = int(^uint(0) >> 1)
	}
	return a.VerseStart <= bEnd && b.VerseStart <= aEnd
}

// List returns the pending cards of one service. A card heard in another
// service is not listed here and cannot be sent live from this one.
func (q *Queue) List(serviceID string) []Candidate {
	q.mu.Lock()
	defer q.mu.Unlock()

	pending := make([]Candidate, 0, len(q.pending))
	for _, candidate := range q.pending {
		if candidate.ServiceID == serviceID {
			pending = append(pending, candidate)
		}
	}
	return pending
}

// switchService settles an open segment when the operator moves to another
// service, so the partials of the service before it do not stay looking live.
// The first service to be heard simply becomes the active one.
// The caller holds the lock.
func (q *Queue) switchService(serviceID string) {
	if q.activeService == serviceID {
		return
	}
	if q.activeService != "" && q.segment >= 1 {
		now := q.clock()
		for i := range q.pending {
			if q.pending[i].Segment == q.segment && !q.pending[i].Finalized {
				q.pending[i].Finalized = true
				q.pending[i].FinalizedAt = now
			}
		}
		q.segment = 0
	}
	q.activeService = serviceID
}

// Approve records the decision. The caller projects afterwards; approval alone
// never touches the display.
func (q *Queue) Approve(ctx context.Context, id, serviceID string) (Candidate, error) {
	return q.decide(ctx, id, decisionApproved, serviceID)
}

func (q *Queue) Dismiss(ctx context.Context, id, serviceID string) (Candidate, error) {
	return q.decide(ctx, id, decisionDismissed, serviceID)
}

// AutoProject records the policy layer's decision: the card was projected
// without the operator and is listed as such in the panel's history.
func (q *Queue) AutoProject(ctx context.Context, id, serviceID string) (Candidate, error) {
	return q.decide(ctx, id, decisionAutoProjected, serviceID)
}

// Clear records the dropped candidates of one service as ignored, so history
// still shows what was heard and let go. The cards of another service are
// left alone: they belong to it.
func (q *Queue) Clear(ctx context.Context, serviceID string) error {
	q.mu.Lock()
	kept := make([]Candidate, 0, len(q.pending))
	cleared := make([]Candidate, 0, len(q.pending))
	for _, candidate := range q.pending {
		if candidate.ServiceID == serviceID {
			cleared = append(cleared, candidate)
			continue
		}
		kept = append(kept, candidate)
	}
	q.pending = kept
	q.mu.Unlock()

	for _, candidate := range cleared {
		if err := q.record(ctx, candidate, decisionIgnored, serviceID); err != nil {
			return err
		}
	}
	return nil
}

func (q *Queue) decide(ctx context.Context, id, decision, serviceID string) (Candidate, error) {
	q.mu.Lock()
	index := -1
	for i, candidate := range q.pending {
		// A card belongs to the service it was heard in: a stale id from
		// another service must not be decided here.
		if candidate.ID == id && candidate.ServiceID == serviceID {
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

	if err := q.record(ctx, candidate, decision, serviceID); err != nil {
		return Candidate{}, err
	}
	return candidate, nil
}

// A record failure is reported, but the decision has already happened.
func (q *Queue) record(ctx context.Context, candidate Candidate, decision, serviceID string) error {
	if q.db == nil {
		return nil
	}
	_, err := q.db.ExecContext(ctx, `
		INSERT INTO detection_history (id, transcript, reference, confidence, decision, service_id, created_at)
		VALUES (?, ?, ?, ?, ?, ?, ?)`,
		candidate.ID, candidate.Transcript, candidate.Reference.String(), candidate.Confidence, decision,
		serviceOrNil(serviceID), candidate.CreatedAt.Format(time.RFC3339),
	)
	if err != nil {
		return fmt.Errorf("record detection decision: %w", err)
	}
	return nil
}

// serviceOrNil writes an empty service as SQL NULL, so a decision taken
// outside a service is not listed under any of them.
func serviceOrNil(id string) any {
	if id == "" {
		return nil
	}
	return id
}

// HistoryEntry is one recorded decision about a spoken reference.
type HistoryEntry struct {
	ID         string
	Transcript string
	Reference  string
	Confidence float64
	Decision   string
	CreatedAt  time.Time
}

// History returns the decisions of one service, newest first, capped at
// limit. A service's decisions are its own: what another service heard and
// what the operator decided about it never shows up here.
func (q *Queue) History(ctx context.Context, serviceID string, limit int) ([]HistoryEntry, error) {
	if q.db == nil {
		return []HistoryEntry{}, nil
	}
	if limit <= 0 {
		return []HistoryEntry{}, nil
	}

	rows, err := q.db.QueryContext(ctx, `
		SELECT id, transcript, reference, confidence, decision, created_at
		FROM detection_history
		WHERE service_id IS ?
		ORDER BY created_at DESC, rowid DESC
		LIMIT ?`, serviceOrNil(serviceID), limit)
	if err != nil {
		return nil, fmt.Errorf("read detection history: %w", err)
	}
	defer rows.Close()

	entries := make([]HistoryEntry, 0, limit)
	for rows.Next() {
		var (
			entry     HistoryEntry
			createdAt string
		)
		if err := rows.Scan(&entry.ID, &entry.Transcript, &entry.Reference,
			&entry.Confidence, &entry.Decision, &createdAt); err != nil {
			return nil, fmt.Errorf("read detection history row: %w", err)
		}
		parsed, err := time.Parse(time.RFC3339, createdAt)
		if err != nil {
			return nil, fmt.Errorf("parse detection decision time %q: %w", createdAt, err)
		}
		entry.CreatedAt = parsed
		entries = append(entries, entry)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("read detection history: %w", err)
	}
	return entries, nil
}
