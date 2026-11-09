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
	Raw       string
	CreatedAt time.Time
}

// decision values must stay inside the CHECK constraint on detection_history.
const (
	decisionApproved = "approved"
	decisionDismissed = "dismissed"
	decisionIgnored  = "ignored"
)

// Queue keeps pending candidates in memory and records decisions in
// detection_history. Pending candidates are not persisted: after a restart the
// room has moved on, and a stale candidate inviting a wrong projection is worse
// than an empty queue.
type Queue struct {
	mu      sync.Mutex
	pending []Candidate
	db      *sql.DB
}

// A nil db drops decisions instead of recording them.
func New(db *sql.DB) *Queue {
	return &Queue{db: db}
}

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

// MergeLive replaces the pending candidate for the same passage in place, so a
// hearing that grows or narrows as the sentence continues updates the card
// instead of stacking a second one. Non-overlapping passages stay separate.
func (q *Queue) MergeLive(transcript string, detections []asr.Detection) ([]Candidate, error) {
	q.mu.Lock()
	defer q.mu.Unlock()

	now := time.Now().UTC().Truncate(time.Second)
	added := make([]Candidate, 0, len(detections))
	for _, detection := range detections {
		replaced := -1
		for i, candidate := range q.pending {
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
			q.pending[replaced] = updated
			added = append(added, updated)
			continue
		}
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
		candidate.ID, candidate.Transcript, candidate.Reference.String(), confidence, decision,
		candidate.CreatedAt.Format(time.RFC3339),
	)
	if err != nil {
		return fmt.Errorf("record detection decision: %w", err)
	}
	return nil
}

// confidence is a fixed score for the rule-based detector, stored for the
// history schema only.
const confidence = 1.0
