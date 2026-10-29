package serviceorder

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"strings"
	"time"

	"zowe-worship/internal/database"
)

const itemColumns = `id, service_id, position, kind, title, translation_id, book_id, chapter,
	verse_start, verse_end, song_id, section_id, created_at`

type SQLiteRepository struct {
	db *sql.DB
}

var _ Repository = (*SQLiteRepository)(nil)

func NewSQLiteRepository(db *sql.DB) *SQLiteRepository {
	return &SQLiteRepository{db: db}
}

func (r *SQLiteRepository) Current(ctx context.Context) (Service, []Item, error) {
	service, err := scanService(r.db.QueryRowContext(ctx, `
		SELECT id, name, created_at, updated_at
		FROM services
		ORDER BY created_at DESC, rowid DESC
		LIMIT 1`))
	if errors.Is(err, sql.ErrNoRows) {
		return Service{}, nil, ErrNoService
	}
	if err != nil {
		return Service{}, nil, fmt.Errorf("read current service: %w", err)
	}

	items, err := r.Items(ctx, service.ID)
	if err != nil {
		return Service{}, nil, err
	}
	return service, items, nil
}

func (r *SQLiteRepository) Create(ctx context.Context, name string) (Service, error) {
	name = strings.TrimSpace(name)
	if name == "" {
		return Service{}, fmt.Errorf("%w: a service name is required", ErrInvalidService)
	}

	id, err := database.NewID()
	if err != nil {
		return Service{}, err
	}
	now := time.Now().UTC().Truncate(time.Second)

	if _, err := r.db.ExecContext(ctx, `
		INSERT INTO services (id, name, created_at, updated_at) VALUES (?, ?, ?, ?)`,
		id, name, now.Format(time.RFC3339), now.Format(time.RFC3339),
	); err != nil {
		return Service{}, fmt.Errorf("create service: %w", err)
	}

	return Service{ID: id, Name: name, CreatedAt: now, UpdatedAt: now}, nil
}

func (r *SQLiteRepository) AddScripture(ctx context.Context, serviceID string, input ScriptureInput) (Item, error) {
	if err := input.validate(); err != nil {
		return Item{}, err
	}
	return r.addItem(ctx, serviceID, Item{
		Kind:          KindScripture,
		Title:         strings.TrimSpace(input.Title),
		TranslationID: input.TranslationID,
		BookID:        input.BookID,
		Chapter:       input.Chapter,
		VerseStart:    input.VerseStart,
		VerseEnd:      input.VerseEnd,
	})
}

func (r *SQLiteRepository) AddSong(ctx context.Context, serviceID string, input SongInput) (Item, error) {
	if err := input.validate(); err != nil {
		return Item{}, err
	}
	return r.addItem(ctx, serviceID, Item{
		Kind:      KindSong,
		Title:     strings.TrimSpace(input.Title),
		SongID:    input.SongID,
		SectionID: input.SectionID,
	})
}

func (r *SQLiteRepository) addItem(ctx context.Context, serviceID string, item Item) (Item, error) {
	tx, err := r.db.BeginTx(ctx, nil)
	if err != nil {
		return Item{}, fmt.Errorf("begin adding service item: %w", err)
	}

	var exists int
	if err := tx.QueryRowContext(ctx, `SELECT 1 FROM services WHERE id = ?`, serviceID).Scan(&exists); err != nil {
		_ = tx.Rollback()
		if errors.Is(err, sql.ErrNoRows) {
			return Item{}, ErrServiceNotFound
		}
		return Item{}, fmt.Errorf("read service: %w", err)
	}

	var position int
	if err := tx.QueryRowContext(ctx,
		`SELECT COALESCE(MAX(position), 0) + 1 FROM service_items WHERE service_id = ?`, serviceID,
	).Scan(&position); err != nil {
		_ = tx.Rollback()
		return Item{}, fmt.Errorf("read service position: %w", err)
	}

	id, err := database.NewID()
	if err != nil {
		_ = tx.Rollback()
		return Item{}, err
	}
	now := time.Now().UTC().Truncate(time.Second)

	if _, err := tx.ExecContext(ctx, `
		INSERT INTO service_items
			(id, service_id, position, kind, title, translation_id, book_id, chapter,
			 verse_start, verse_end, song_id, section_id, created_at)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`,
		id, serviceID, position, item.Kind, item.Title, item.TranslationID,
		item.BookID, item.Chapter, item.VerseStart, item.VerseEnd, item.SongID, item.SectionID,
		now.Format(time.RFC3339),
	); err != nil {
		_ = tx.Rollback()
		return Item{}, fmt.Errorf("add service item: %w", err)
	}
	if _, err := tx.ExecContext(ctx,
		`UPDATE services SET updated_at = ? WHERE id = ?`, now.Format(time.RFC3339), serviceID,
	); err != nil {
		_ = tx.Rollback()
		return Item{}, fmt.Errorf("update service: %w", err)
	}
	if err := tx.Commit(); err != nil {
		return Item{}, fmt.Errorf("commit service item: %w", err)
	}

	item.ID = id
	item.ServiceID = serviceID
	item.Position = position
	item.CreatedAt = now
	return item, nil
}

func (r *SQLiteRepository) RemoveItem(ctx context.Context, itemID string) error {
	tx, err := r.db.BeginTx(ctx, nil)
	if err != nil {
		return fmt.Errorf("begin removing service item: %w", err)
	}

	var serviceID string
	if err := tx.QueryRowContext(ctx,
		`SELECT service_id FROM service_items WHERE id = ?`, itemID,
	).Scan(&serviceID); err != nil {
		_ = tx.Rollback()
		if errors.Is(err, sql.ErrNoRows) {
			return ErrItemNotFound
		}
		return fmt.Errorf("read service item: %w", err)
	}

	if _, err := tx.ExecContext(ctx, `DELETE FROM service_items WHERE id = ?`, itemID); err != nil {
		_ = tx.Rollback()
		return fmt.Errorf("remove service item: %w", err)
	}
	if _, err := tx.ExecContext(ctx,
		`UPDATE services SET updated_at = ? WHERE id = ?`,
		time.Now().UTC().Truncate(time.Second).Format(time.RFC3339), serviceID,
	); err != nil {
		_ = tx.Rollback()
		return fmt.Errorf("update service: %w", err)
	}
	if err := tx.Commit(); err != nil {
		return fmt.Errorf("commit service item removal: %w", err)
	}
	return nil
}

func (r *SQLiteRepository) Item(ctx context.Context, itemID string) (Item, error) {
	item, err := scanItem(r.db.QueryRowContext(ctx,
		`SELECT `+itemColumns+` FROM service_items WHERE id = ?`, itemID))
	if errors.Is(err, sql.ErrNoRows) {
		return Item{}, ErrItemNotFound
	}
	if err != nil {
		return Item{}, fmt.Errorf("read service item: %w", err)
	}
	return item, nil
}

func (r *SQLiteRepository) Items(ctx context.Context, serviceID string) ([]Item, error) {
	rows, err := r.db.QueryContext(ctx,
		`SELECT `+itemColumns+` FROM service_items WHERE service_id = ? ORDER BY position`, serviceID)
	if err != nil {
		return nil, fmt.Errorf("list service items: %w", err)
	}
	defer rows.Close()

	items := make([]Item, 0)
	for rows.Next() {
		item, err := scanItem(rows)
		if err != nil {
			return nil, err
		}
		items = append(items, item)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("list service items: %w", err)
	}
	return items, nil
}

func scanService(row interface{ Scan(...any) error }) (Service, error) {
	var service Service
	var createdAt, updatedAt string
	if err := row.Scan(&service.ID, &service.Name, &createdAt, &updatedAt); err != nil {
		return Service{}, err
	}

	created, err := time.Parse(time.RFC3339, createdAt)
	if err != nil {
		return Service{}, fmt.Errorf("parse service creation time: %w", err)
	}
	updated, err := time.Parse(time.RFC3339, updatedAt)
	if err != nil {
		return Service{}, fmt.Errorf("parse service update time: %w", err)
	}
	service.CreatedAt = created
	service.UpdatedAt = updated
	return service, nil
}

func scanItem(row interface{ Scan(...any) error }) (Item, error) {
	var item Item
	var kind, createdAt string
	if err := row.Scan(
		&item.ID, &item.ServiceID, &item.Position, &kind, &item.Title, &item.TranslationID,
		&item.BookID, &item.Chapter, &item.VerseStart, &item.VerseEnd, &item.SongID, &item.SectionID,
		&createdAt,
	); err != nil {
		return Item{}, err
	}

	created, err := time.Parse(time.RFC3339, createdAt)
	if err != nil {
		return Item{}, fmt.Errorf("parse service item time: %w", err)
	}
	item.Kind = ItemKind(kind)
	item.CreatedAt = created
	return item, nil
}
