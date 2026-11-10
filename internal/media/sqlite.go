package media

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"time"
)

const assetColumns = `id, kind, title, filename, source_path, checksum, size_bytes,
	duration_ms, width, height, has_thumbnail, created_at, aspect, fit, background_id`

type SQLiteRepository struct {
	db *sql.DB
}

var _ Repository = (*SQLiteRepository)(nil)

func NewSQLiteRepository(db *sql.DB) *SQLiteRepository {
	return &SQLiteRepository{db: db}
}

func (r *SQLiteRepository) List(ctx context.Context) ([]Asset, error) {
	rows, err := r.db.QueryContext(ctx,
		`SELECT `+assetColumns+` FROM media_assets ORDER BY created_at DESC, rowid DESC`)
	if err != nil {
		return nil, fmt.Errorf("list media: %w", err)
	}
	defer rows.Close()

	assets := make([]Asset, 0)
	for rows.Next() {
		asset, err := scanAsset(rows)
		if err != nil {
			return nil, err
		}
		assets = append(assets, asset)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("list media: %w", err)
	}
	return assets, nil
}

func (r *SQLiteRepository) Add(ctx context.Context, asset Asset) error {
	if _, err := r.db.ExecContext(ctx, `
		INSERT INTO media_assets
			(id, kind, title, filename, source_path, checksum, size_bytes,
			 duration_ms, width, height, has_thumbnail, created_at, aspect, fit, background_id)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`,
		asset.ID, string(asset.Kind), asset.Title, asset.Filename, asset.SourcePath,
		asset.Checksum, asset.SizeBytes, asset.DurationMS, asset.Width, asset.Height,
		boolToInt(asset.HasThumbnail), asset.CreatedAt.Format(time.RFC3339),
		asset.Aspect, asset.Fit, asset.BackgroundID,
	); err != nil {
		return fmt.Errorf("add media asset: %w", err)
	}
	return nil
}

func (r *SQLiteRepository) SavePresentation(ctx context.Context, id string, presentation Presentation) error {
	result, err := r.db.ExecContext(ctx,
		`UPDATE media_assets SET aspect = ?, fit = ? WHERE id = ?`,
		presentation.Aspect, presentation.Fit, id,
	)
	if err != nil {
		return fmt.Errorf("save media presentation: %w", err)
	}
	saved, err := result.RowsAffected()
	if err != nil {
		return fmt.Errorf("save media presentation: %w", err)
	}
	if saved == 0 {
		return ErrAssetNotFound
	}
	return nil
}

func (r *SQLiteRepository) SaveBackground(ctx context.Context, id, backgroundID string) error {
	result, err := r.db.ExecContext(ctx,
		`UPDATE media_assets SET background_id = ? WHERE id = ?`,
		backgroundID, id,
	)
	if err != nil {
		return fmt.Errorf("save media background: %w", err)
	}
	saved, err := result.RowsAffected()
	if err != nil {
		return fmt.Errorf("save media background: %w", err)
	}
	if saved == 0 {
		return ErrAssetNotFound
	}
	return nil
}

// ClearBackgroundReferences forgets every asset that used the given picture as
// its background, so deleting a picture leaves no dangling ids behind.
func (r *SQLiteRepository) ClearBackgroundReferences(ctx context.Context, id string) error {
	if _, err := r.db.ExecContext(ctx,
		`UPDATE media_assets SET background_id = '' WHERE background_id = ?`, id,
	); err != nil {
		return fmt.Errorf("clear media background references: %w", err)
	}
	return nil
}

// SetRestored records where a relinked file was found, together with the size
// and thumbnail this package regenerated when the copy came back.
func (r *SQLiteRepository) SetRestored(ctx context.Context, asset Asset) error {
	result, err := r.db.ExecContext(ctx,
		`UPDATE media_assets SET source_path = ?, width = ?, height = ?, has_thumbnail = ? WHERE id = ?`,
		asset.SourcePath, asset.Width, asset.Height, asset.HasThumbnail, asset.ID,
	)
	if err != nil {
		return fmt.Errorf("record restored media: %w", err)
	}
	changed, err := result.RowsAffected()
	if err != nil {
		return fmt.Errorf("record restored media: %w", err)
	}
	if changed == 0 {
		return ErrAssetNotFound
	}
	return nil
}

func (r *SQLiteRepository) Remove(ctx context.Context, id string) error {
	result, err := r.db.ExecContext(ctx, `DELETE FROM media_assets WHERE id = ?`, id)
	if err != nil {
		return fmt.Errorf("remove media asset: %w", err)
	}
	deleted, err := result.RowsAffected()
	if err != nil {
		return fmt.Errorf("remove media asset: %w", err)
	}
	if deleted == 0 {
		return ErrAssetNotFound
	}
	return nil
}

func (r *SQLiteRepository) ByID(ctx context.Context, id string) (Asset, error) {
	asset, err := scanAsset(r.db.QueryRowContext(ctx,
		`SELECT `+assetColumns+` FROM media_assets WHERE id = ?`, id))
	if errors.Is(err, sql.ErrNoRows) {
		return Asset{}, ErrAssetNotFound
	}
	if err != nil {
		return Asset{}, fmt.Errorf("read media asset: %w", err)
	}
	return asset, nil
}

func (r *SQLiteRepository) ByChecksum(ctx context.Context, checksum string) (Asset, bool, error) {
	asset, err := scanAsset(r.db.QueryRowContext(ctx,
		`SELECT `+assetColumns+` FROM media_assets WHERE checksum = ? LIMIT 1`, checksum))
	if errors.Is(err, sql.ErrNoRows) {
		return Asset{}, false, nil
	}
	if err != nil {
		return Asset{}, false, fmt.Errorf("find media by checksum: %w", err)
	}
	return asset, true, nil
}

func scanAsset(row interface{ Scan(...any) error }) (Asset, error) {
	var asset Asset
	var kind string
	var hasThumbnail int
	var createdAt string
	if err := row.Scan(
		&asset.ID, &kind, &asset.Title, &asset.Filename, &asset.SourcePath, &asset.Checksum,
		&asset.SizeBytes, &asset.DurationMS, &asset.Width, &asset.Height, &hasThumbnail, &createdAt,
		&asset.Aspect, &asset.Fit, &asset.BackgroundID,
	); err != nil {
		return Asset{}, err
	}

	created, err := time.Parse(time.RFC3339, createdAt)
	if err != nil {
		return Asset{}, fmt.Errorf("parse media creation time: %w", err)
	}
	asset.Kind = Kind(kind)
	asset.HasThumbnail = hasThumbnail != 0
	asset.CreatedAt = created
	return asset, nil
}

func boolToInt(value bool) int {
	if value {
		return 1
	}
	return 0
}
