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
	verse_start, verse_end, song_id, section_id, media_id, created_at`

type SQLiteRepository struct {
	db *sql.DB
}

var _ Repository = (*SQLiteRepository)(nil)

func NewSQLiteRepository(db *sql.DB) *SQLiteRepository {
	return &SQLiteRepository{db: db}
}

const settingCurrentService = "current_service_id"

const serviceColumns = `id, name, created_at, updated_at`

func (r *SQLiteRepository) Current(ctx context.Context) (Service, []Item, error) {
	service, err := r.currentService(ctx)
	if err != nil {
		return Service{}, nil, err
	}

	items, err := r.Items(ctx, service.ID)
	if err != nil {
		return Service{}, nil, err
	}
	return service, items, nil
}

// currentService follows the stored pointer and repairs it when it is missing
// or points at a service that no longer exists.
func (r *SQLiteRepository) currentService(ctx context.Context) (Service, error) {
	var stored string
	err := r.db.QueryRowContext(ctx,
		`SELECT value FROM settings WHERE key = ?`, settingCurrentService,
	).Scan(&stored)
	if err != nil && !errors.Is(err, sql.ErrNoRows) {
		return Service{}, fmt.Errorf("read current service: %w", err)
	}
	if err == nil {
		service, lookupErr := r.serviceByID(ctx, stored)
		if lookupErr == nil {
			return service, nil
		}
		if !errors.Is(lookupErr, ErrServiceNotFound) {
			return Service{}, lookupErr
		}
	}

	newest, err := scanService(r.db.QueryRowContext(ctx, `
		SELECT `+serviceColumns+`
		FROM services
		ORDER BY created_at DESC, rowid DESC
		LIMIT 1`))
	if errors.Is(err, sql.ErrNoRows) {
		return Service{}, ErrNoService
	}
	if err != nil {
		return Service{}, fmt.Errorf("read newest service: %w", err)
	}
	if err := r.setCurrent(ctx, newest.ID); err != nil {
		return Service{}, err
	}
	return newest, nil
}

func (r *SQLiteRepository) serviceByID(ctx context.Context, id string) (Service, error) {
	service, err := scanService(r.db.QueryRowContext(ctx, `
		SELECT `+serviceColumns+` FROM services WHERE id = ?`, id))
	if errors.Is(err, sql.ErrNoRows) {
		return Service{}, ErrServiceNotFound
	}
	if err != nil {
		return Service{}, fmt.Errorf("read service: %w", err)
	}
	return service, nil
}

func (r *SQLiteRepository) setCurrent(ctx context.Context, serviceID string) error {
	if _, err := r.db.ExecContext(ctx, `
		INSERT INTO settings (key, value) VALUES (?, ?)
		ON CONFLICT(key) DO UPDATE SET value = excluded.value`,
		settingCurrentService, serviceID,
	); err != nil {
		return fmt.Errorf("store current service: %w", err)
	}
	return nil
}

func (r *SQLiteRepository) List(ctx context.Context) ([]Service, error) {
	rows, err := r.db.QueryContext(ctx, `
		SELECT `+serviceColumns+`
		FROM services
		ORDER BY updated_at DESC, created_at DESC, rowid DESC`)
	if err != nil {
		return nil, fmt.Errorf("list services: %w", err)
	}
	defer rows.Close()

	services := make([]Service, 0)
	for rows.Next() {
		service, err := scanService(rows)
		if err != nil {
			return nil, err
		}
		services = append(services, service)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("list services: %w", err)
	}
	return services, nil
}

func (r *SQLiteRepository) Use(ctx context.Context, serviceID string) error {
	if _, err := r.serviceByID(ctx, serviceID); err != nil {
		return err
	}
	return r.setCurrent(ctx, serviceID)
}

func (r *SQLiteRepository) Rename(ctx context.Context, serviceID, name string) error {
	name = strings.TrimSpace(name)
	if name == "" {
		return fmt.Errorf("%w: a service name is required", ErrInvalidService)
	}

	result, err := r.db.ExecContext(ctx, `UPDATE services SET name = ? WHERE id = ?`, name, serviceID)
	if err != nil {
		return fmt.Errorf("rename service: %w", err)
	}
	updated, err := result.RowsAffected()
	if err != nil {
		return fmt.Errorf("rename service: %w", err)
	}
	if updated == 0 {
		return ErrServiceNotFound
	}
	return nil
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
	// Starting a service means working on it, so it becomes the current one.
	if err := r.setCurrent(ctx, id); err != nil {
		return Service{}, err
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

func (r *SQLiteRepository) AddMedia(ctx context.Context, serviceID string, input MediaInput) (Item, error) {
	if err := input.validate(); err != nil {
		return Item{}, err
	}
	return r.addItem(ctx, serviceID, Item{
		Kind:    KindMedia,
		Title:   strings.TrimSpace(input.Title),
		MediaID: input.MediaID,
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
			 verse_start, verse_end, song_id, section_id, media_id, created_at)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`,
		id, serviceID, position, item.Kind, item.Title, item.TranslationID,
		item.BookID, item.Chapter, item.VerseStart, item.VerseEnd, item.SongID, item.SectionID,
		item.MediaID, now.Format(time.RFC3339),
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

// Reorder stores a complete schedule order for the service that owns the given
// items. Positions are unique per service, so the rows step aside under
// negative numbers before their final positions land.
func (r *SQLiteRepository) Reorder(ctx context.Context, itemIDs []string) error {
	if len(itemIDs) == 0 {
		return fmt.Errorf("%w: the order is empty", ErrInvalidOrder)
	}

	tx, err := r.db.BeginTx(ctx, nil)
	if err != nil {
		return fmt.Errorf("begin reordering service: %w", err)
	}

	args := make([]any, 0, len(itemIDs))
	for _, id := range itemIDs {
		args = append(args, id)
	}
	placeholders := strings.TrimSuffix(strings.Repeat("?,", len(itemIDs)), ",")

	var serviceID string
	var matched int
	var distinct int
	err = tx.QueryRowContext(ctx, `
		SELECT service_id, COUNT(*), COUNT(DISTINCT id)
		FROM service_items WHERE id IN (`+placeholders+`)
		GROUP BY service_id`, args...).Scan(&serviceID, &matched, &distinct)
	if err != nil {
		_ = tx.Rollback()
		if errors.Is(err, sql.ErrNoRows) {
			return ErrItemNotFound
		}
		return fmt.Errorf("read reordered items: %w", err)
	}
	if matched != len(itemIDs) || distinct != len(itemIDs) {
		_ = tx.Rollback()
		return fmt.Errorf("%w: an item was listed twice or belongs to another service", ErrInvalidOrder)
	}

	var total int
	if err := tx.QueryRowContext(ctx,
		`SELECT COUNT(*) FROM service_items WHERE service_id = ?`, serviceID,
	).Scan(&total); err != nil {
		_ = tx.Rollback()
		return fmt.Errorf("count service items: %w", err)
	}
	if total != len(itemIDs) {
		_ = tx.Rollback()
		return fmt.Errorf("%w: %d of %d items were ordered", ErrInvalidOrder, len(itemIDs), total)
	}

	if _, err := tx.ExecContext(ctx,
		`UPDATE service_items SET position = -position - 1 WHERE service_id = ?`, serviceID,
	); err != nil {
		_ = tx.Rollback()
		return fmt.Errorf("clear service positions: %w", err)
	}
	for position, id := range itemIDs {
		if _, err := tx.ExecContext(ctx,
			`UPDATE service_items SET position = ? WHERE id = ?`, position+1, id,
		); err != nil {
			_ = tx.Rollback()
			return fmt.Errorf("set service item position: %w", err)
		}
	}

	now := time.Now().UTC().Truncate(time.Second)
	if _, err := tx.ExecContext(ctx,
		`UPDATE services SET updated_at = ? WHERE id = ?`, now.Format(time.RFC3339), serviceID,
	); err != nil {
		_ = tx.Rollback()
		return fmt.Errorf("update service: %w", err)
	}
	if err := tx.Commit(); err != nil {
		return fmt.Errorf("commit service order: %w", err)
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
		&item.MediaID, &createdAt,
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
