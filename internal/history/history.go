// Package history records what actually reached the room. The controller
// already guarantees that only an explicit projection command changes live
// output, so a row here means an operator put that slide on the wall — not that
// they looked at it in the preview pane.
//
// The table was created back in migration 009 and has sat unwritten ever since.
// The PRD asks for it in two places: recording projection actions locally, and
// restoring past output. This package does the recording; restoring is a later
// slice, which is why a row carries the schedule item it came from.
//
// A row belongs to the service that was running when it was written, so one
// service's history never lists another service's projections.
package history

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"time"

	"zowe-worship/internal/database"
)

// SourceManual marks an operator projection. The detection workstream will add
// automatic ones alongside it, and the column is what tells them apart later.
const SourceManual = "manual"

// KindClear is the one kind this package names itself. Everything else is the
// projected slide's own kind, taken straight from the content that went live, so
// the vocabulary stays with the slide rather than being re-listed here.
const KindClear = "clear"

// Entry is one projection, as stored.
type Entry struct {
	ID    string
	Kind  string
	Title string
	Label string
	// ServiceID is the service the projection belongs to. Empty means it
	// belongs to no service - it was projected outside one - and it is then
	// listed under none of them.
	ServiceID string
	// Source is who projected it; see SourceManual.
	Source string
	// Payload identifies the schedule item behind the slide, empty when the
	// operator projected something ad hoc from the Bible or media tab. A later
	// restore slice reads it back to put the same item on screen again.
	Payload     string
	ProjectedAt time.Time
}

// Log writes projections to the core database and reads them back.
type Log struct {
	db *sql.DB
}

// New returns a Log backed by the given database. A nil db is allowed for
// callers without storage, the same as the detection queue: recording is then
// skipped rather than treated as a failure.
func New(db *sql.DB) *Log {
	return &Log{db: db}
}

// Record writes one projection. An error is returned to the caller, which is
// expected to log it and carry on: the slide is already on the wall, and losing
// the audit row must not undo that.
func (l *Log) Record(ctx context.Context, entry Entry) error {
	if l.db == nil {
		return nil
	}

	id, err := database.NewID()
	if err != nil {
		return fmt.Errorf("new history id: %w", err)
	}
	projectedAt := entry.ProjectedAt
	if projectedAt.IsZero() {
		projectedAt = time.Now()
	}
	projectedAt = projectedAt.UTC().Truncate(time.Second)

	_, err = l.db.ExecContext(ctx, `
		INSERT INTO presentation_history (id, kind, title, label, service_id, source, payload, projected_at)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?)`,
		id, entry.Kind, entry.Title, entry.Label, serviceOrNil(entry.ServiceID), entry.Source, entry.Payload,
		projectedAt.Format(time.RFC3339),
	)
	if err != nil {
		return fmt.Errorf("record projection: %w", err)
	}
	return nil
}

// serviceOrNil writes an empty service as SQL NULL, so a projection made
// outside a service is not listed under any of them.
func serviceOrNil(id string) any {
	if id == "" {
		return nil
	}
	return id
}

// Entry reads one history row; a restore cannot act on a row it cannot read.
func (l *Log) Entry(ctx context.Context, id string) (Entry, error) {
	if l.db == nil {
		return Entry{}, fmt.Errorf("presentation history is not available")
	}

	row := l.db.QueryRowContext(ctx, `
		SELECT kind, title, label, service_id, source, payload, projected_at
		FROM presentation_history
		WHERE id = ?`, id)

	var (
		entry       Entry
		serviceID   sql.NullString
		projectedAt string
	)
	if err := row.Scan(&entry.Kind, &entry.Title, &entry.Label, &serviceID,
		&entry.Source, &entry.Payload, &projectedAt); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return Entry{}, fmt.Errorf("that projection is no longer in the history")
		}
		return Entry{}, fmt.Errorf("read presentation history: %w", err)
	}
	parsed, err := time.Parse(time.RFC3339, projectedAt)
	if err != nil {
		return Entry{}, fmt.Errorf("parse projection time %q: %w", projectedAt, err)
	}
	entry.ID = id
	entry.ServiceID = serviceID.String
	entry.ProjectedAt = parsed
	return entry, nil
}

// Recent returns the most recent projections of one service, newest first,
// capped at limit. A service's history is its own: the projections of the
// service before it are not listed under this one, and an empty service id
// asks for the projections that belong to no service at all.
func (l *Log) Recent(ctx context.Context, serviceID string, limit int) ([]Entry, error) {
	if l.db == nil {
		return []Entry{}, nil
	}
	if limit <= 0 {
		return []Entry{}, nil
	}

	rows, err := l.db.QueryContext(ctx, `
		SELECT id, kind, title, label, service_id, source, payload, projected_at
		FROM presentation_history
		WHERE service_id IS ?
		ORDER BY projected_at DESC, rowid DESC
		LIMIT ?`, serviceOrNil(serviceID), limit)
	if err != nil {
		return nil, fmt.Errorf("read presentation history: %w", err)
	}
	defer rows.Close()

	entries := make([]Entry, 0, limit)
	for rows.Next() {
		var (
			entry       Entry
			serviceID   sql.NullString
			projectedAt string
		)
		if err := rows.Scan(&entry.ID, &entry.Kind, &entry.Title, &entry.Label,
			&serviceID, &entry.Source, &entry.Payload, &projectedAt); err != nil {
			return nil, fmt.Errorf("scan presentation history: %w", err)
		}
		entry.ServiceID = serviceID.String
		parsed, err := time.Parse(time.RFC3339, projectedAt)
		if err != nil {
			return nil, fmt.Errorf("parse projection time %q: %w", projectedAt, err)
		}
		entry.ProjectedAt = parsed
		entries = append(entries, entry)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate presentation history: %w", err)
	}
	return entries, nil
}
