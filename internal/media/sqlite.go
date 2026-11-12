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

func (r *SQLiteRepository) ListPlaylists(ctx context.Context) ([]Playlist, error) {
	rows, err := r.db.QueryContext(ctx,
		`SELECT id, name, created_at FROM media_playlists ORDER BY created_at DESC, rowid DESC`)
	if err != nil {
		return nil, fmt.Errorf("list media playlists: %w", err)
	}
	defer rows.Close()

	playlists := make([]Playlist, 0)
	for rows.Next() {
		playlist, err := scanPlaylist(rows)
		if err != nil {
			return nil, err
		}
		playlists = append(playlists, playlist)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("list media playlists: %w", err)
	}
	// The membership is read per playlist: the count is the number of
	// playlists an operator has, not the number of clips in the library.
	for index := range playlists {
		items, err := r.playlistItems(ctx, playlists[index].ID)
		if err != nil {
			return nil, err
		}
		playlists[index].Items = items
	}
	return playlists, nil
}

func (r *SQLiteRepository) PlaylistByID(ctx context.Context, id string) (Playlist, error) {
	playlist, err := scanPlaylist(r.db.QueryRowContext(ctx,
		`SELECT id, name, created_at FROM media_playlists WHERE id = ?`, id))
	if errors.Is(err, sql.ErrNoRows) {
		return Playlist{}, ErrPlaylistNotFound
	}
	if err != nil {
		return Playlist{}, fmt.Errorf("read media playlist: %w", err)
	}
	items, err := r.playlistItems(ctx, id)
	if err != nil {
		return Playlist{}, err
	}
	playlist.Items = items
	return playlist, nil
}

func (r *SQLiteRepository) playlistItems(ctx context.Context, playlistID string) ([]string, error) {
	rows, err := r.db.QueryContext(ctx,
		`SELECT media_id FROM media_playlist_items WHERE playlist_id = ? ORDER BY position, rowid`, playlistID)
	if err != nil {
		return nil, fmt.Errorf("list media playlist items: %w", err)
	}
	defer rows.Close()

	items := make([]string, 0)
	for rows.Next() {
		var id string
		if err := rows.Scan(&id); err != nil {
			return nil, fmt.Errorf("read media playlist item: %w", err)
		}
		items = append(items, id)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("list media playlist items: %w", err)
	}
	return items, nil
}

func (r *SQLiteRepository) SavePlaylist(ctx context.Context, playlist Playlist) error {
	tx, err := r.db.BeginTx(ctx, nil)
	if err != nil {
		return fmt.Errorf("save media playlist: %w", err)
	}
	if _, err := tx.ExecContext(ctx, `
		INSERT INTO media_playlists (id, name, created_at) VALUES (?, ?, ?)
		ON CONFLICT(id) DO UPDATE SET name = excluded.name`,
		playlist.ID, playlist.Name, playlist.CreatedAt.Format(time.RFC3339),
	); err != nil {
		_ = tx.Rollback()
		return fmt.Errorf("save media playlist: %w", err)
	}
	// The membership is rewritten rather than patched: positions are
	// contiguous from zero, so a reorder is delete-then-insert in one
	// transaction. A clip id that no longer exists fails the foreign key here.
	if _, err := tx.ExecContext(ctx,
		`DELETE FROM media_playlist_items WHERE playlist_id = ?`, playlist.ID); err != nil {
		_ = tx.Rollback()
		return fmt.Errorf("save media playlist items: %w", err)
	}
	for position, mediaID := range playlist.Items {
		if _, err := tx.ExecContext(ctx,
			`INSERT INTO media_playlist_items (playlist_id, position, media_id) VALUES (?, ?, ?)`,
			playlist.ID, position, mediaID); err != nil {
			_ = tx.Rollback()
			return fmt.Errorf("save media playlist items: %w", err)
		}
	}
	if err := tx.Commit(); err != nil {
		return fmt.Errorf("save media playlist: %w", err)
	}
	return nil
}

func (r *SQLiteRepository) DeletePlaylist(ctx context.Context, id string) error {
	result, err := r.db.ExecContext(ctx, `DELETE FROM media_playlists WHERE id = ?`, id)
	if err != nil {
		return fmt.Errorf("delete media playlist: %w", err)
	}
	deleted, err := result.RowsAffected()
	if err != nil {
		return fmt.Errorf("delete media playlist: %w", err)
	}
	if deleted == 0 {
		return ErrPlaylistNotFound
	}
	return nil
}

func scanPlaylist(row interface{ Scan(...any) error }) (Playlist, error) {
	var playlist Playlist
	var createdAt string
	if err := row.Scan(&playlist.ID, &playlist.Name, &createdAt); err != nil {
		return Playlist{}, err
	}
	parsed, err := time.Parse(time.RFC3339, createdAt)
	if err != nil {
		return Playlist{}, fmt.Errorf("parse playlist creation time: %w", err)
	}
	playlist.CreatedAt = parsed
	return playlist, nil
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
