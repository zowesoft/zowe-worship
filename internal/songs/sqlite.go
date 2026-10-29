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
	songColumns        = "id, title, author, copyright, created_at, updated_at"
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
		INSERT INTO songs (id, title, author, copyright, created_at, updated_at)
		VALUES (?, ?, ?, ?, ?, ?)`,
		songID, song.Title, song.Author, song.Copyright, now, now,
	); err != nil {
		_ = tx.Rollback()
		return SongWithSections{}, fmt.Errorf("create song: %w", err)
	}

	for index, section := range song.Sections {
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
			return SongWithSections{}, fmt.Errorf("create song section: %w", err)
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

	return SongWithSections{Song: song, Sections: sections}, nil
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
		SELECT s.id, s.title, s.author, s.copyright, s.created_at, s.updated_at,
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
			&hit.Song.ID, &hit.Song.Title, &hit.Song.Author, &hit.Song.Copyright, &createdAt, &updatedAt,
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
	if err := row.Scan(&song.ID, &song.Title, &song.Author, &song.Copyright, &createdAt, &updatedAt); err != nil {
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
