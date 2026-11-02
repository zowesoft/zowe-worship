package songs

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"strings"
	"time"
	"unicode"

	"zowe-worship/internal/database"
)

var ErrEmptyQuery = errors.New("search text is empty")

const (
	defaultSearchLimit = 50
	maxSearchLimit     = 200
	songColumns        = "id, title, author, copyright, ccli_number, source, created_at, updated_at"
)

type SQLiteRepository struct {
	db *sql.DB
}

var _ Repository = (*SQLiteRepository)(nil)

func NewSQLiteRepository(db *sql.DB) *SQLiteRepository {
	return &SQLiteRepository{db: db}
}

func (r *SQLiteRepository) Create(ctx context.Context, input NewSong) (SongWithSections, error) {
	song, err := input.normalised()
	if err != nil {
		return SongWithSections{}, err
	}

	songID, err := database.NewID()
	if err != nil {
		return SongWithSections{}, err
	}
	now := time.Now().UTC().Truncate(time.Second).Format(time.RFC3339)

	tx, err := r.db.BeginTx(ctx, nil)
	if err != nil {
		return SongWithSections{}, fmt.Errorf("begin song creation: %w", err)
	}

	if _, err := tx.ExecContext(ctx, `
		INSERT INTO songs (id, title, author, copyright, ccli_number, source, created_at, updated_at)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?)`,
		songID, song.Title, song.Author, song.Copyright, song.CCLINumber, song.Source, now, now,
	); err != nil {
		_ = tx.Rollback()
		return SongWithSections{}, fmt.Errorf("create song: %w", err)
	}

	sectionIDs := make([]string, 0, len(song.Sections))
	for index, section := range song.Sections {
		sectionID, err := database.NewID()
		if err != nil {
			_ = tx.Rollback()
			return SongWithSections{}, err
		}
		sectionIDs = append(sectionIDs, sectionID)
		if _, err := tx.ExecContext(ctx, `
			INSERT INTO song_sections (id, song_id, position, label, text) VALUES (?, ?, ?, ?, ?)`,
			sectionID, songID, index+1, section.Label, section.Text,
		); err != nil {
			_ = tx.Rollback()
			return SongWithSections{}, fmt.Errorf("create song section: %w", err)
		}
	}

	// Create default arrangement containing all sections in order
	arrID, err := database.NewID()
	if err != nil {
		_ = tx.Rollback()
		return SongWithSections{}, err
	}
	if _, err := tx.ExecContext(ctx, `
		INSERT INTO song_arrangements (id, song_id, name, is_default, created_at, updated_at)
		VALUES (?, ?, 'Default', 1, ?, ?)`,
		arrID, songID, now, now,
	); err != nil {
		_ = tx.Rollback()
		return SongWithSections{}, fmt.Errorf("create default arrangement: %w", err)
	}
	for index, secID := range sectionIDs {
		itemID, err := database.NewID()
		if err != nil {
			_ = tx.Rollback()
			return SongWithSections{}, err
		}
		if _, err := tx.ExecContext(ctx, `
			INSERT INTO song_arrangement_items (id, arrangement_id, section_id, position)
			VALUES (?, ?, ?, ?)`,
			itemID, arrID, secID, index+1,
		); err != nil {
			_ = tx.Rollback()
			return SongWithSections{}, fmt.Errorf("create arrangement item: %w", err)
		}
	}

	if _, err := tx.ExecContext(ctx, `INSERT INTO song_sections_fts (song_sections_fts) VALUES ('integrity-check')`); err != nil {
		_ = tx.Rollback()
		return SongWithSections{}, fmt.Errorf("verify lyric index: %w", err)
	}
	if err := tx.Commit(); err != nil {
		return SongWithSections{}, fmt.Errorf("commit song creation: %w", err)
	}

	return r.Get(ctx, songID)
}

// Update rewrites a song in place. A section that keeps its id keeps its
// identity, so a service item scheduled from it still resolves after the lyrics
// change; sections the editor dropped are removed.
func (r *SQLiteRepository) Update(ctx context.Context, songID string, input SongUpdate) (SongWithSections, error) {
	update, err := input.normalised()
	if err != nil {
		return SongWithSections{}, err
	}

	tx, err := r.db.BeginTx(ctx, nil)
	if err != nil {
		return SongWithSections{}, fmt.Errorf("begin song update: %w", err)
	}

	var exists int
	if err := tx.QueryRowContext(ctx, `SELECT 1 FROM songs WHERE id = ?`, songID).Scan(&exists); err != nil {
		_ = tx.Rollback()
		if errors.Is(err, sql.ErrNoRows) {
			return SongWithSections{}, ErrSongNotFound
		}
		return SongWithSections{}, fmt.Errorf("read song: %w", err)
	}

	now := time.Now().UTC().Truncate(time.Second).Format(time.RFC3339)
	if _, err := tx.ExecContext(ctx, `
		UPDATE songs SET title = ?, author = ?, copyright = ?, ccli_number = ?, source = ?, updated_at = ? WHERE id = ?`,
		update.Title, update.Author, update.Copyright, update.CCLINumber, update.Source, now, songID,
	); err != nil {
		_ = tx.Rollback()
		return SongWithSections{}, fmt.Errorf("update song: %w", err)
	}

	// Positions are unique per song, and an edit can both move a kept section and
	// add another, so the rows step aside first and are numbered afterwards.
	if _, err := tx.ExecContext(ctx,
		`UPDATE song_sections SET position = -position WHERE song_id = ?`, songID,
	); err != nil {
		_ = tx.Rollback()
		return SongWithSections{}, fmt.Errorf("clear section positions: %w", err)
	}

	kept := make([]string, 0, len(update.Sections))
	for index, section := range update.Sections {
		if section.ID != "" {
			result, err := tx.ExecContext(ctx, `
				UPDATE song_sections SET position = ?, label = ?, text = ?
				WHERE id = ? AND song_id = ?`,
				index+1, section.Label, section.Text, section.ID, songID,
			)
			if err != nil {
				_ = tx.Rollback()
				return SongWithSections{}, fmt.Errorf("update song section: %w", err)
			}
			changed, err := result.RowsAffected()
			if err != nil {
				_ = tx.Rollback()
				return SongWithSections{}, fmt.Errorf("update song section: %w", err)
			}
			if changed == 0 {
				_ = tx.Rollback()
				return SongWithSections{}, ErrSectionNotFound
			}
			kept = append(kept, section.ID)
			continue
		}

		sectionID, err := database.NewID()
		if err != nil {
			_ = tx.Rollback()
			return SongWithSections{}, err
		}
		if _, err := tx.ExecContext(ctx, `
			INSERT INTO song_sections (id, song_id, position, label, text) VALUES (?, ?, ?, ?, ?)`,
			sectionID, songID, index+1, section.Label, section.Text,
		); err != nil {
			_ = tx.Rollback()
			return SongWithSections{}, fmt.Errorf("add song section: %w", err)
		}
		kept = append(kept, sectionID)
	}

	args := make([]any, 0, len(kept)+1)
	args = append(args, songID)
	for _, id := range kept {
		args = append(args, id)
	}
	placeholders := strings.TrimSuffix(strings.Repeat("?,", len(kept)), ",")
	if _, err := tx.ExecContext(ctx,
		`DELETE FROM song_sections WHERE song_id = ? AND id NOT IN (`+placeholders+`)`, args...,
	); err != nil {
		_ = tx.Rollback()
		return SongWithSections{}, fmt.Errorf("remove song section: %w", err)
	}

	// Ensure at least one arrangement exists
	var arrCount int
	_ = tx.QueryRowContext(ctx, `SELECT COUNT(*) FROM song_arrangements WHERE song_id = ?`, songID).Scan(&arrCount)
	if arrCount == 0 && len(kept) > 0 {
		arrID, err := database.NewID()
		if err == nil {
			if _, err := tx.ExecContext(ctx, `
				INSERT INTO song_arrangements (id, song_id, name, is_default, created_at, updated_at)
				VALUES (?, ?, 'Default', 1, ?, ?)`,
				arrID, songID, now, now,
			); err == nil {
				for index, secID := range kept {
					itemID, err := database.NewID()
					if err == nil {
						_, _ = tx.ExecContext(ctx, `
							INSERT INTO song_arrangement_items (id, arrangement_id, section_id, position)
							VALUES (?, ?, ?, ?)`,
							itemID, arrID, secID, index+1,
						)
					}
				}
			}
		}
	}

	if _, err := tx.ExecContext(ctx, `INSERT INTO song_sections_fts (song_sections_fts) VALUES ('integrity-check')`); err != nil {
		_ = tx.Rollback()
		return SongWithSections{}, fmt.Errorf("verify lyric index: %w", err)
	}
	if err := tx.Commit(); err != nil {
		return SongWithSections{}, fmt.Errorf("commit song update: %w", err)
	}

	return r.Get(ctx, songID)
}

func (r *SQLiteRepository) List(ctx context.Context) ([]SongWithSections, error) {
	songs, err := r.querySongs(ctx, `SELECT `+songColumns+` FROM songs ORDER BY title COLLATE NOCASE`)
	if err != nil {
		return nil, err
	}

	sections, err := r.db.QueryContext(ctx, `
		SELECT id, song_id, position, label, text
		FROM song_sections
		ORDER BY song_id, position`)
	if err != nil {
		return nil, fmt.Errorf("list song sections: %w", err)
	}
	defer sections.Close()

	bySong := make(map[string][]Section)
	for sections.Next() {
		section, err := scanSection(sections)
		if err != nil {
			return nil, err
		}
		bySong[section.SongID] = append(bySong[section.SongID], section)
	}
	if err := sections.Err(); err != nil {
		return nil, fmt.Errorf("list song sections: %w", err)
	}

	for index := range songs {
		songs[index].Sections = bySong[songs[index].ID]
		arrs, _ := r.ListArrangements(ctx, songs[index].ID)
		if arrs == nil {
			arrs = []Arrangement{}
		}
		songs[index].Arrangements = arrs
	}
	return songs, nil
}

func (r *SQLiteRepository) Get(ctx context.Context, songID string) (SongWithSections, error) {
	song, err := scanSong(r.db.QueryRowContext(ctx,
		`SELECT `+songColumns+` FROM songs WHERE id = ?`, songID))
	if errors.Is(err, sql.ErrNoRows) {
		return SongWithSections{}, ErrSongNotFound
	}
	if err != nil {
		return SongWithSections{}, fmt.Errorf("read song: %w", err)
	}

	rows, err := r.db.QueryContext(ctx, `
		SELECT id, song_id, position, label, text
		FROM song_sections WHERE song_id = ? ORDER BY position`, songID)
	if err != nil {
		return SongWithSections{}, fmt.Errorf("read song sections: %w", err)
	}
	defer rows.Close()

	sections := make([]Section, 0)
	for rows.Next() {
		section, err := scanSection(rows)
		if err != nil {
			return SongWithSections{}, err
		}
		sections = append(sections, section)
	}
	if err := rows.Err(); err != nil {
		return SongWithSections{}, fmt.Errorf("read song sections: %w", err)
	}

	arrs, err := r.ListArrangements(ctx, songID)
	if err != nil {
		return SongWithSections{}, err
	}
	if arrs == nil {
		arrs = []Arrangement{}
	}

	return SongWithSections{Song: song, Sections: sections, Arrangements: arrs}, nil
}

func (r *SQLiteRepository) Delete(ctx context.Context, songID string) error {
	tx, err := r.db.BeginTx(ctx, nil)
	if err != nil {
		return fmt.Errorf("begin song removal: %w", err)
	}

	result, err := tx.ExecContext(ctx, `DELETE FROM songs WHERE id = ?`, songID)
	if err != nil {
		_ = tx.Rollback()
		return fmt.Errorf("remove song: %w", err)
	}
	removed, err := result.RowsAffected()
	if err != nil {
		_ = tx.Rollback()
		return fmt.Errorf("remove song: %w", err)
	}
	if removed == 0 {
		_ = tx.Rollback()
		return ErrSongNotFound
	}
	if err := tx.Commit(); err != nil {
		return fmt.Errorf("commit song removal: %w", err)
	}
	return nil
}

func (r *SQLiteRepository) Duplicate(ctx context.Context, songID, newTitle string) (SongWithSections, error) {
	orig, err := r.Get(ctx, songID)
	if err != nil {
		return SongWithSections{}, err
	}

	title := strings.TrimSpace(newTitle)
	if title == "" {
		title = orig.Title + " (Copy)"
	}

	newID, err := database.NewID()
	if err != nil {
		return SongWithSections{}, err
	}
	now := time.Now().UTC().Truncate(time.Second).Format(time.RFC3339)

	tx, err := r.db.BeginTx(ctx, nil)
	if err != nil {
		return SongWithSections{}, fmt.Errorf("begin song duplication: %w", err)
	}

	if _, err := tx.ExecContext(ctx, `
		INSERT INTO songs (id, title, author, copyright, ccli_number, source, created_at, updated_at)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?)`,
		newID, title, orig.Author, orig.Copyright, orig.CCLINumber, orig.Source, now, now,
	); err != nil {
		_ = tx.Rollback()
		return SongWithSections{}, fmt.Errorf("duplicate song: %w", err)
	}

	sectionMap := make(map[string]string) // oldSectionID -> newSectionID
	for index, sec := range orig.Sections {
		secID, err := database.NewID()
		if err != nil {
			_ = tx.Rollback()
			return SongWithSections{}, err
		}
		sectionMap[sec.ID] = secID
		if _, err := tx.ExecContext(ctx, `
			INSERT INTO song_sections (id, song_id, position, label, text)
			VALUES (?, ?, ?, ?, ?)`,
			secID, newID, index+1, sec.Label, sec.Text,
		); err != nil {
			_ = tx.Rollback()
			return SongWithSections{}, fmt.Errorf("duplicate song section: %w", err)
		}
	}

	for _, arr := range orig.Arrangements {
		arrID, err := database.NewID()
		if err != nil {
			_ = tx.Rollback()
			return SongWithSections{}, err
		}
		isDef := 0
		if arr.IsDefault {
			isDef = 1
		}
		if _, err := tx.ExecContext(ctx, `
			INSERT INTO song_arrangements (id, song_id, name, is_default, created_at, updated_at)
			VALUES (?, ?, ?, ?, ?, ?)`,
			arrID, newID, arr.Name, isDef, now, now,
		); err != nil {
			_ = tx.Rollback()
			return SongWithSections{}, fmt.Errorf("duplicate song arrangement: %w", err)
		}

		for pos, item := range arr.Items {
			newSecID, ok := sectionMap[item.SectionID]
			if !ok {
				continue
			}
			itemID, err := database.NewID()
			if err != nil {
				_ = tx.Rollback()
				return SongWithSections{}, err
			}
			if _, err := tx.ExecContext(ctx, `
				INSERT INTO song_arrangement_items (id, arrangement_id, section_id, position)
				VALUES (?, ?, ?, ?)`,
				itemID, arrID, newSecID, pos+1,
			); err != nil {
				_ = tx.Rollback()
				return SongWithSections{}, fmt.Errorf("duplicate arrangement item: %w", err)
			}
		}
	}

	if _, err := tx.ExecContext(ctx, `INSERT INTO song_sections_fts (song_sections_fts) VALUES ('integrity-check')`); err != nil {
		_ = tx.Rollback()
		return SongWithSections{}, fmt.Errorf("verify lyric index: %w", err)
	}

	if err := tx.Commit(); err != nil {
		return SongWithSections{}, fmt.Errorf("commit song duplication: %w", err)
	}

	return r.Get(ctx, newID)
}

func (r *SQLiteRepository) CreateArrangement(ctx context.Context, input NewArrangement) (Arrangement, error) {
	arr, err := input.normalised()
	if err != nil {
		return Arrangement{}, err
	}

	var exists int
	if err := r.db.QueryRowContext(ctx, `SELECT 1 FROM songs WHERE id = ?`, arr.SongID).Scan(&exists); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return Arrangement{}, ErrSongNotFound
		}
		return Arrangement{}, fmt.Errorf("verify song: %w", err)
	}

	arrID, err := database.NewID()
	if err != nil {
		return Arrangement{}, err
	}
	now := time.Now().UTC().Truncate(time.Second).Format(time.RFC3339)

	tx, err := r.db.BeginTx(ctx, nil)
	if err != nil {
		return Arrangement{}, fmt.Errorf("begin create arrangement: %w", err)
	}

	if arr.IsDefault {
		if _, err := tx.ExecContext(ctx, `UPDATE song_arrangements SET is_default = 0 WHERE song_id = ?`, arr.SongID); err != nil {
			_ = tx.Rollback()
			return Arrangement{}, fmt.Errorf("reset default arrangement: %w", err)
		}
	}

	isDef := 0
	if arr.IsDefault {
		isDef = 1
	}

	if _, err := tx.ExecContext(ctx, `
		INSERT INTO song_arrangements (id, song_id, name, is_default, created_at, updated_at)
		VALUES (?, ?, ?, ?, ?, ?)`,
		arrID, arr.SongID, arr.Name, isDef, now, now,
	); err != nil {
		_ = tx.Rollback()
		return Arrangement{}, fmt.Errorf("create arrangement: %w", err)
	}

	for index, secID := range arr.SectionIDs {
		itemID, err := database.NewID()
		if err != nil {
			_ = tx.Rollback()
			return Arrangement{}, err
		}
		if _, err := tx.ExecContext(ctx, `
			INSERT INTO song_arrangement_items (id, arrangement_id, section_id, position)
			VALUES (?, ?, ?, ?)`,
			itemID, arrID, secID, index+1,
		); err != nil {
			_ = tx.Rollback()
			return Arrangement{}, fmt.Errorf("create arrangement item: %w", err)
		}
	}

	if err := tx.Commit(); err != nil {
		return Arrangement{}, fmt.Errorf("commit arrangement creation: %w", err)
	}

	return r.GetArrangement(ctx, arrID)
}

func (r *SQLiteRepository) UpdateArrangement(ctx context.Context, arrangementID string, input ArrangementUpdate) (Arrangement, error) {
	update, err := input.normalised()
	if err != nil {
		return Arrangement{}, err
	}

	var songID string
	if err := r.db.QueryRowContext(ctx, `SELECT song_id FROM song_arrangements WHERE id = ?`, arrangementID).Scan(&songID); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return Arrangement{}, ErrArrangementNotFound
		}
		return Arrangement{}, fmt.Errorf("verify arrangement: %w", err)
	}

	tx, err := r.db.BeginTx(ctx, nil)
	if err != nil {
		return Arrangement{}, fmt.Errorf("begin update arrangement: %w", err)
	}

	if update.IsDefault {
		if _, err := tx.ExecContext(ctx, `UPDATE song_arrangements SET is_default = 0 WHERE song_id = ?`, songID); err != nil {
			_ = tx.Rollback()
			return Arrangement{}, fmt.Errorf("reset default arrangement: %w", err)
		}
	}

	isDef := 0
	if update.IsDefault {
		isDef = 1
	}
	now := time.Now().UTC().Truncate(time.Second).Format(time.RFC3339)

	if _, err := tx.ExecContext(ctx, `
		UPDATE song_arrangements SET name = ?, is_default = ?, updated_at = ? WHERE id = ?`,
		update.Name, isDef, now, arrangementID,
	); err != nil {
		_ = tx.Rollback()
		return Arrangement{}, fmt.Errorf("update arrangement: %w", err)
	}

	if _, err := tx.ExecContext(ctx, `DELETE FROM song_arrangement_items WHERE arrangement_id = ?`, arrangementID); err != nil {
		_ = tx.Rollback()
		return Arrangement{}, fmt.Errorf("clear arrangement items: %w", err)
	}

	for index, secID := range update.SectionIDs {
		itemID, err := database.NewID()
		if err != nil {
			_ = tx.Rollback()
			return Arrangement{}, err
		}
		if _, err := tx.ExecContext(ctx, `
			INSERT INTO song_arrangement_items (id, arrangement_id, section_id, position)
			VALUES (?, ?, ?, ?)`,
			itemID, arrangementID, secID, index+1,
		); err != nil {
			_ = tx.Rollback()
			return Arrangement{}, fmt.Errorf("create arrangement item: %w", err)
		}
	}

	if err := tx.Commit(); err != nil {
		return Arrangement{}, fmt.Errorf("commit arrangement update: %w", err)
	}

	return r.GetArrangement(ctx, arrangementID)
}

func (r *SQLiteRepository) DeleteArrangement(ctx context.Context, arrangementID string) error {
	result, err := r.db.ExecContext(ctx, `DELETE FROM song_arrangements WHERE id = ?`, arrangementID)
	if err != nil {
		return fmt.Errorf("delete arrangement: %w", err)
	}
	rows, err := result.RowsAffected()
	if err != nil {
		return fmt.Errorf("delete arrangement: %w", err)
	}
	if rows == 0 {
		return ErrArrangementNotFound
	}
	return nil
}

func (r *SQLiteRepository) GetArrangement(ctx context.Context, arrangementID string) (Arrangement, error) {
	var arr Arrangement
	var isDef int
	var createdAt, updatedAt string

	if err := r.db.QueryRowContext(ctx, `
		SELECT id, song_id, name, is_default, created_at, updated_at
		FROM song_arrangements WHERE id = ?`, arrangementID,
	).Scan(&arr.ID, &arr.SongID, &arr.Name, &isDef, &createdAt, &updatedAt); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return Arrangement{}, ErrArrangementNotFound
		}
		return Arrangement{}, fmt.Errorf("read arrangement: %w", err)
	}
	arr.IsDefault = isDef == 1
	cTime, _ := time.Parse(time.RFC3339, createdAt)
	uTime, _ := time.Parse(time.RFC3339, updatedAt)
	arr.CreatedAt = cTime
	arr.UpdatedAt = uTime

	rows, err := r.db.QueryContext(ctx, `
		SELECT ai.id, ai.arrangement_id, ai.section_id, ai.position, ss.label, ss.text
		FROM song_arrangement_items ai
		JOIN song_sections ss ON ss.id = ai.section_id
		WHERE ai.arrangement_id = ?
		ORDER BY ai.position`, arrangementID)
	if err != nil {
		return Arrangement{}, fmt.Errorf("read arrangement items: %w", err)
	}
	defer rows.Close()

	items := make([]ArrangementItem, 0)
	for rows.Next() {
		var item ArrangementItem
		if err := rows.Scan(&item.ID, &item.ArrangementID, &item.SectionID, &item.Position, &item.Label, &item.Text); err != nil {
			return Arrangement{}, err
		}
		items = append(items, item)
	}
	if err := rows.Err(); err != nil {
		return Arrangement{}, err
	}
	arr.Items = items
	return arr, nil
}

func (r *SQLiteRepository) ListArrangements(ctx context.Context, songID string) ([]Arrangement, error) {
	rows, err := r.db.QueryContext(ctx, `
		SELECT id, song_id, name, is_default, created_at, updated_at
		FROM song_arrangements WHERE song_id = ?
		ORDER BY is_default DESC, name COLLATE NOCASE`, songID)
	if err != nil {
		return nil, fmt.Errorf("list arrangements: %w", err)
	}
	defer rows.Close()

	arrs := make([]Arrangement, 0)
	for rows.Next() {
		var arr Arrangement
		var isDef int
		var createdAt, updatedAt string
		if err := rows.Scan(&arr.ID, &arr.SongID, &arr.Name, &isDef, &createdAt, &updatedAt); err != nil {
			return nil, err
		}
		arr.IsDefault = isDef == 1
		cTime, _ := time.Parse(time.RFC3339, createdAt)
		uTime, _ := time.Parse(time.RFC3339, updatedAt)
		arr.CreatedAt = cTime
		arr.UpdatedAt = uTime
		arrs = append(arrs, arr)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}

	for i := range arrs {
		full, err := r.GetArrangement(ctx, arrs[i].ID)
		if err != nil {
			return nil, err
		}
		arrs[i].Items = full.Items
	}
	return arrs, nil
}

func (r *SQLiteRepository) Search(ctx context.Context, query string, limit int) ([]SectionHit, error) {
	match, err := matchExpression(query)
	if err != nil {
		return nil, err
	}
	if limit <= 0 {
		limit = defaultSearchLimit
	}
	if limit > maxSearchLimit {
		limit = maxSearchLimit
	}

	rows, err := r.db.QueryContext(ctx, `
		SELECT s.id, s.title, s.author, s.copyright, s.ccli_number, s.source, s.created_at, s.updated_at,
		       ss.id, ss.song_id, ss.position, ss.label, ss.text
		FROM song_sections_fts
		JOIN song_sections ss ON ss.rowid = song_sections_fts.rowid
		JOIN songs s ON s.id = ss.song_id
		WHERE song_sections_fts MATCH ?
		ORDER BY bm25(song_sections_fts), s.title COLLATE NOCASE, ss.position
		LIMIT ?`, match, limit)
	if err != nil {
		return nil, fmt.Errorf("search lyrics: %w", err)
	}
	defer rows.Close()

	hits := make([]SectionHit, 0)
	for rows.Next() {
		var hit SectionHit
		var createdAt, updatedAt string
		if err := rows.Scan(
			&hit.Song.ID, &hit.Song.Title, &hit.Song.Author, &hit.Song.Copyright, &hit.Song.CCLINumber, &hit.Song.Source, &createdAt, &updatedAt,
			&hit.Section.ID, &hit.Section.SongID, &hit.Section.Position, &hit.Section.Label, &hit.Section.Text,
		); err != nil {
			return nil, fmt.Errorf("read lyric match: %w", err)
		}
		hits = append(hits, hit)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("search lyrics: %w", err)
	}
	return hits, nil
}

func (r *SQLiteRepository) Section(ctx context.Context, songID, sectionID string) (Song, Section, error) {
	song, err := scanSong(r.db.QueryRowContext(ctx,
		`SELECT `+songColumns+` FROM songs WHERE id = ?`, songID))
	if errors.Is(err, sql.ErrNoRows) {
		return Song{}, Section{}, ErrSongNotFound
	}
	if err != nil {
		return Song{}, Section{}, fmt.Errorf("read song: %w", err)
	}

	section, err := scanSection(r.db.QueryRowContext(ctx, `
		SELECT id, song_id, position, label, text
		FROM song_sections WHERE id = ? AND song_id = ?`, sectionID, songID))
	if errors.Is(err, sql.ErrNoRows) {
		return Song{}, Section{}, ErrSectionNotFound
	}
	if err != nil {
		return Song{}, Section{}, fmt.Errorf("read song section: %w", err)
	}
	return song, section, nil
}

func (r *SQLiteRepository) querySongs(ctx context.Context, query string, args ...any) ([]SongWithSections, error) {
	rows, err := r.db.QueryContext(ctx, query, args...)
	if err != nil {
		return nil, fmt.Errorf("list songs: %w", err)
	}
	defer rows.Close()

	songs := make([]SongWithSections, 0)
	for rows.Next() {
		song, err := scanSong(rows)
		if err != nil {
			return nil, err
		}
		songs = append(songs, SongWithSections{Song: song, Sections: []Section{}})
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("list songs: %w", err)
	}
	return songs, nil
}

func scanSong(row interface{ Scan(...any) error }) (Song, error) {
	var song Song
	var createdAt, updatedAt string
	if err := row.Scan(&song.ID, &song.Title, &song.Author, &song.Copyright, &song.CCLINumber, &song.Source, &createdAt, &updatedAt); err != nil {
		return Song{}, err
	}

	created, err := time.Parse(time.RFC3339, createdAt)
	if err != nil {
		return Song{}, fmt.Errorf("parse song creation time: %w", err)
	}
	updated, err := time.Parse(time.RFC3339, updatedAt)
	if err != nil {
		return Song{}, fmt.Errorf("parse song update time: %w", err)
	}
	song.CreatedAt = created
	song.UpdatedAt = updated
	return song, nil
}

func scanSection(row interface{ Scan(...any) error }) (Section, error) {
	var section Section
	if err := row.Scan(&section.ID, &section.SongID, &section.Position, &section.Label, &section.Text); err != nil {
		return Section{}, err
	}
	return section, nil
}

func matchExpression(query string) (string, error) {
	terms := make([]string, 0, 4)
	for _, field := range strings.Fields(query) {
		if !strings.ContainsFunc(field, func(r rune) bool { return unicode.IsLetter(r) || unicode.IsDigit(r) }) {
			continue
		}
		terms = append(terms, `"`+strings.ReplaceAll(field, `"`, `""`)+`"`)
	}
	if len(terms) == 0 {
		return "", ErrEmptyQuery
	}
	return strings.Join(terms, " AND "), nil
}
