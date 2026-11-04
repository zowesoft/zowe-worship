// Package detection holds the candidate queue between spoken-reference
// detection and operator confirmation. The PRD is explicit that detection is
// a suggestion only: a candidate never reaches the room on its own, so the
// queue is the contract the operator works against.
package detection

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"sync"
	"time"

	"zowe-worship/internal/asr"
	"zowe-worship/internal/database"
	"zowe-worship/internal/scripture"
)

// ErrCandidateNotFound is returned when an id no longer sits in the queue,
// either because it was already decided or because the app restarted.
var ErrCandidateNotFound = errors.New("detection candidate not found")

// Candidate is one detected reference waiting for the operator's decision.
type Candidate struct {
	ID         string
	Transcript string
	Reference  scripture.Reference
	// Raw is the normalized candidate string the detection was parsed from,
	// useful for debugging ASR errors that produced a near miss.
	Raw       string
	CreatedAt time.Time
}

// decision values must stay inside the CHECK constraint on detection_history.
const (
	decisionApproved = "approved"
	decisionDismissed = "dismissed"
	decisionIgnored  = "ignored"
)

// Queue keeps pending candidates in memory and records every decision the
// operator makes in detection_history. Pending candidates are deliberately
// not persisted: after a restart the room has moved on, and a stale candidate
// inviting a wrong projection is worse than an empty queue.
type Queue struct {
	mu      sync.Mutex
	pending []Candidate
	db      *sql.DB
}

// New returns a Queue that records decisions in the given database. A nil db
// is allowed for callers without storage; decisions are then dropped instead
// of recorded.
func New(db *sql.DB) *Queue {
	return &Queue{db: db}
}

// Add queues every detection found in one transcript and returns the new
// candidates in detection order.
func (q *Queue) Add(transcript string, detections []asr.Detection) ([]Candidate, error) {
	q.mu.Lock()
	defer q.mu.Unlock()

	now := time.Now().UTC().Truncate(time.Second)
	added := make([]Candidate, 0, len(detections))
	for _, detection := range detections {
		candidate := Candidate{
			Transcript: transcript,
			Reference:  detection.Reference,
			Raw:        detection.Raw,
			CreatedAt:  now,
		}
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

// List returns the pending candidates, oldest first.
func (q *Queue) List() []Candidate {
	q.mu.Lock()
	defer q.mu.Unlock()

	pending := make([]Candidate, len(q.pending))
	copy(pending, q.pending)
	return pending
}

// Approve removes the candidate from the queue and records the decision. The
// caller projects the reference afterwards; approval alone never touches the
// audience display.
func (q *Queue) Approve(ctx context.Context, id string) (Candidate, error) {
	return q.decide(ctx, id, decisionApproved)
}

// Dismiss removes the candidate from the queue and records the decision.
func (q *Queue) Dismiss(ctx context.Context, id string) (Candidate, error) {
	return q.decide(ctx, id, decisionDismissed)
}

// Clear drops every pending candidate, recording them as ignored so the
// history still shows what was heard and let go.
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

// record writes one history row. A failure after the queue was updated is
// reported to the caller, but the decision itself already happened.
func (q *Queue) record(ctx context.Context, candidate Candidate, decision string) error {
	if q.db == nil {
		return nil
	}
	_, err := q.db.ExecContext(ctx, `
		INSERT INTO detection_history (id, transcript, reference, confidence, decision, created_at)
		VALUES (?, ?, ?, ?, ?, ?)`,
		candidate.ID, candidate.Transcript, candidate.Reference.String(), confidence, decision,
		candidate.CreatedAt.Format(time.RFC3339),
	)
	if err != nil {
		return fmt.Errorf("record detection decision: %w", err)
	}
	return nil
}

// confidence is the fixed score the rule-based detector earns. It is stored
// for the history schema; the operator's own judgement is what counts.
const confidence = 1.0
