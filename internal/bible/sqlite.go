package bible

import (
	"context"
	"crypto/sha256"
	"database/sql"
	"encoding/hex"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
	"time"

	"zopresent/internal/database"
)

const (
	packageSchemaVersion = 1
	defaultSearchLimit   = 50
	maxSearchLimit       = 200
)

var (
	translationIDPattern  = regexp.MustCompile(`^[a-z0-9]+$`)
	translationVerPattern = regexp.MustCompile(`^[A-Za-z0-9][A-Za-z0-9._]*$`)
)

// SQLiteRepository keeps translation catalog metadata in the core database and
// stores each translation as its own package file. Installing, updating or
// removing one translation never rewrites another translation's file.
type SQLiteRepository struct {
	core *sql.DB
	dir  string
}

var _ Repository = (*SQLiteRepository)(nil)

func NewSQLiteRepository(core *sql.DB, translationsDir string) *SQLiteRepository {
	return &SQLiteRepository{core: core, dir: translationsDir}
}

func (r *SQLiteRepository) ListTranslations(ctx context.Context) ([]Translation, error) {
	rows, err := r.core.QueryContext(ctx, `
		SELECT id, name, abbreviation, language, version, verse_count, checksum, installed_at, license, attribution
		FROM bible_translations
		ORDER BY name COLLATE NOCASE`)
	if err != nil {
		return nil, fmt.Errorf("list translations: %w", err)
	}
	defer rows.Close()

	var translations []Translation
	for rows.Next() {
		translation, err := scanTranslation(rows)
		if err != nil {
			return nil, err
		}
		translations = append(translations, translation)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("list translations: %w", err)
	}
	return translations, nil
}

func (r *SQLiteRepository) Translation(ctx context.Context, id string) (Translation, error) {
	translation, _, err := r.catalogEntry(ctx, id)
	return translation, err
}

func (r *SQLiteRepository) catalogEntry(ctx context.Context, id string) (Translation, string, error) {
	var filename string
	translation, err := scanTranslation(r.core.QueryRowContext(ctx, `
		SELECT id, name, abbreviation, language, version, verse_count, checksum, installed_at, license, attribution, filename
		FROM bible_translations WHERE id = ?`, id), &filename)
	if errors.Is(err, sql.ErrNoRows) {
		return Translation{}, "", ErrTranslationNotFound
	}
	if err != nil {
		return Translation{}, "", err
	}
	return translation, filename, nil
}

func (r *SQLiteRepository) Install(ctx context.Context, data TranslationData) (Translation, error) {
	// A translation the registry knows travels with its recorded license; an
	// unknown one has to state its own through the importer's flags.
	if data.License == "" || data.Attribution == "" {
		if license, ok := LicenseFor(data.ID); ok {
			if data.License == "" {
				data.License = license.License
			}
			if data.Attribution == "" {
				data.Attribution = license.Attribution
			}
		}
	}
	if err := validateTranslation(data); err != nil {
		return Translation{}, err
	}
	if err := os.MkdirAll(r.dir, 0o755); err != nil {
		return Translation{}, fmt.Errorf("create translations directory: %w", err)
	}

	filename := fmt.Sprintf("%s-%s.db", data.ID, data.Version)
	finalPath := filepath.Join(r.dir, filename)
	tmpPath := filepath.Join(r.dir, fmt.Sprintf("%s.tmp-%d", filename, time.Now().UnixNano()))

	if err := buildPackage(ctx, tmpPath, data); err != nil {
		_ = os.Remove(tmpPath)
		return Translation{}, err
	}
	if err := syncFile(tmpPath); err != nil {
		_ = os.Remove(tmpPath)
		return Translation{}, fmt.Errorf("flush translation package: %w", err)
	}
	checksum, err := fileChecksum(tmpPath)
	if err != nil {
		_ = os.Remove(tmpPath)
		return Translation{}, err
	}

	displaced, err := publishPackage(tmpPath, finalPath)
	if err != nil {
		_ = os.Remove(tmpPath)
		return Translation{}, err
	}

	installedAt := time.Now().UTC().Truncate(time.Second)
	translation := Translation{
		TranslationMeta: data.TranslationMeta,
		VerseCount:      len(data.Verses),
		Checksum:        checksum,
		InstalledAt:     installedAt,
	}

	if err := r.upsertCatalog(ctx, translation, filename); err != nil {
		_ = os.Remove(finalPath)
		if displaced != "" {
			_ = os.Rename(displaced, finalPath)
		}
		return Translation{}, err
	}

	if displaced != "" {
		_ = os.Remove(displaced)
	}
	r.removeOtherPackages(data.ID, filename)

	return translation, nil
}

func (r *SQLiteRepository) upsertCatalog(ctx context.Context, translation Translation, filename string) error {
	tx, err := r.core.BeginTx(ctx, nil)
	if err != nil {
		return fmt.Errorf("begin translation catalog update: %w", err)
	}

	_, err = tx.ExecContext(ctx, `
		INSERT INTO bible_translations
			(id, name, abbreviation, language, version, filename, verse_count, checksum, installed_at, license, attribution)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
		ON CONFLICT(id) DO UPDATE SET
			name = excluded.name,
			abbreviation = excluded.abbreviation,
			language = excluded.language,
			version = excluded.version,
			filename = excluded.filename,
			verse_count = excluded.verse_count,
			checksum = excluded.checksum,
			installed_at = excluded.installed_at,
			license = excluded.license,
			attribution = excluded.attribution`,
		translation.ID, translation.Name, translation.Abbreviation, translation.Language,
		translation.Version, filename, translation.VerseCount, translation.Checksum,
		translation.InstalledAt.Format(time.RFC3339), translation.License, translation.Attribution,
	)
	if err != nil {
		_ = tx.Rollback()
		return fmt.Errorf("record translation: %w", err)
	}
	if err := tx.Commit(); err != nil {
		return fmt.Errorf("commit translation catalog update: %w", err)
	}
	return nil
}

func (r *SQLiteRepository) Remove(ctx context.Context, id string) error {
	_, filename, err := r.catalogEntry(ctx, id)
	if err != nil {
		return err
	}

	tx, err := r.core.BeginTx(ctx, nil)
	if err != nil {
		return fmt.Errorf("begin translation removal: %w", err)
	}
	result, err := tx.ExecContext(ctx, `DELETE FROM bible_translations WHERE id = ?`, id)
	if err != nil {
		_ = tx.Rollback()
		return fmt.Errorf("remove translation: %w", err)
	}
	removed, err := result.RowsAffected()
	if err != nil {
		_ = tx.Rollback()
		return fmt.Errorf("remove translation: %w", err)
	}
	if removed == 0 {
		_ = tx.Rollback()
		return ErrTranslationNotFound
	}
	if err := tx.Commit(); err != nil {
		return fmt.Errorf("commit translation removal: %w", err)
	}

	r.removeOtherPackages(id, "")
	if err := os.Remove(filepath.Join(r.dir, filename)); err != nil && !errors.Is(err, os.ErrNotExist) {
		return fmt.Errorf("delete translation package: %w", err)
	}
	return nil
}

func (r *SQLiteRepository) Search(ctx context.Context, req SearchRequest) ([]SearchResult, error) {
	packageDB, err := r.openPackage(ctx, req.TranslationID)
	if err != nil {
		return nil, err
	}
	defer packageDB.Close()

	limit := req.Limit
	if limit <= 0 {
		limit = defaultSearchLimit
	}
	if limit > maxSearchLimit {
		limit = maxSearchLimit
	}
	offset := req.Offset
	if offset < 0 {
		offset = 0
	}

	filterClauses, filterArgs := searchFilters(req)

	switch req.Mode {
	case "", SearchWords:
		return r.searchRelaxed(ctx, packageDB, req.Text, filterClauses, filterArgs, limit, offset)
	case SearchPhrase, SearchPrefix:
		match, err := matchExpression(req.Text, req.Mode)
		if err != nil {
			return nil, err
		}
		return r.runSearch(ctx, packageDB, match, filterClauses, filterArgs, limit, offset)
	default:
		return nil, fmt.Errorf("unknown search mode %q", req.Mode)
	}
}

// searchFilters builds the optional testament/book WHERE clauses and their
// arguments, shared by every stage of a search.
func searchFilters(req SearchRequest) ([]string, []any) {
	var clauses []string
	var args []any
	if req.Testament != "" {
		clauses = append(clauses, "b.testament = ?")
		args = append(args, string(req.Testament))
	}
	if len(req.BookIDs) > 0 {
		placeholders := make([]string, len(req.BookIDs))
		for i, id := range req.BookIDs {
			placeholders[i] = strconv.Itoa(id)
		}
		clauses = append(clauses, "b.id IN ("+strings.Join(placeholders, ",")+")")
	}
	return clauses, args
}

// searchRelaxed answers a full-text query in two stages. Stage one keeps the
// query honest: every word must appear, matched as a prefix so an inflection
// like "loved" still answers a search for "love". Only when nothing carries
// every word does stage two widen to any word. bm25 then floats the verses
// matching the most (and rarest) of those words to the top, so the closest
// partial matches lead instead of the search coming back empty.
func (r *SQLiteRepository) searchRelaxed(
	ctx context.Context,
	db *sql.DB,
	text string,
	filterClauses []string,
	filterArgs []any,
	limit, offset int,
) ([]SearchResult, error) {
	terms, err := relaxedTerms(text)
	if err != nil {
		return nil, err
	}

	results, err := r.runSearch(ctx, db, strings.Join(terms, " AND "), filterClauses, filterArgs, limit, offset)
	if err != nil {
		return nil, err
	}
	// An empty later page means the page ran out, not that we should widen.
	if len(results) > 0 || offset > 0 {
		return results, nil
	}

	return r.runSearch(ctx, db, strings.Join(terms, " OR "), filterClauses, filterArgs, limit, offset)
}

// runSearch executes one FTS5 query and returns the ranked, snippeted verses.
func (r *SQLiteRepository) runSearch(
	ctx context.Context,
	db *sql.DB,
	match string,
	filterClauses []string,
	filterArgs []any,
	limit, offset int,
) ([]SearchResult, error) {
	where := append([]string{"verses_fts MATCH ?"}, filterClauses...)
	args := append([]any{match}, filterArgs...)
	args = append(args, limit, offset)

	q := `SELECT b.id, b.name, b.abbreviation, b.testament, v.chapter, v.verse, v.text,
		snippet(verses_fts, 0, '<mark>', '</mark>', '…', 12)
		FROM verses_fts
		JOIN verses v ON v.id = verses_fts.rowid
		JOIN books b ON b.id = v.book_id
		WHERE ` + strings.Join(where, " AND ") + `
		ORDER BY bm25(verses_fts), b.id, v.chapter, v.verse
		LIMIT ? OFFSET ?`

	rows, err := db.QueryContext(ctx, q, args...)
	if err != nil {
		return nil, fmt.Errorf("search: %w", err)
	}
	defer rows.Close()

	return scanSearchResultsWithSnippet(rows)
}

func (r *SQLiteRepository) Passage(
	ctx context.Context, translationID string, bookID, chapter, startVerse, endVerse int,
) ([]SearchResult, error) {
	if _, ok := BookByID(bookID); !ok {
		return nil, fmt.Errorf("passage: unknown book id %d", bookID)
	}
	if chapter <= 0 {
		return nil, errors.New("passage: chapter must be positive")
	}
	if startVerse < 0 || endVerse < 0 {
		return nil, errors.New("passage: verse numbers cannot be negative")
	}
	if startVerse == 0 && endVerse != 0 {
		return nil, errors.New("passage: a verse range needs a start")
	}
	if startVerse > 0 && endVerse == 0 {
		endVerse = startVerse
	}
	if endVerse < startVerse {
		return nil, errors.New("passage: range ends before it starts")
	}

	packageDB, err := r.openPackage(ctx, translationID)
	if err != nil {
		return nil, err
	}
	defer packageDB.Close()

	query := `
		SELECT b.id, b.name, b.abbreviation, b.testament, v.chapter, v.verse, v.text
		FROM verses v
		JOIN books b ON b.id = v.book_id
		WHERE v.book_id = ? AND v.chapter = ?`
	args := []any{bookID, chapter}
	if startVerse > 0 {
		query += ` AND v.verse BETWEEN ? AND ?`
		args = append(args, startVerse, endVerse)
	}
	query += ` ORDER BY v.verse`

	rows, err := packageDB.QueryContext(ctx, query, args...)
	if err != nil {
		return nil, fmt.Errorf("read passage from %s: %w", translationID, err)
	}
	defer rows.Close()

	return scanSearchResults(rows)
}

func (r *SQLiteRepository) openPackage(ctx context.Context, id string) (*sql.DB, error) {
	_, filename, err := r.catalogEntry(ctx, id)
	if err != nil {
		return nil, err
	}
	if filepath.Base(filename) != filename {
		return nil, fmt.Errorf("%w: unsafe package filename", ErrInvalidTranslation)
	}

	packageDB, err := database.OpenReadOnly(ctx, filepath.Join(r.dir, filename))
	if err != nil {
		return nil, fmt.Errorf("open translation %s: %w", id, err)
	}
	if err := checkPackageVersion(ctx, packageDB); err != nil {
		_ = packageDB.Close()
		return nil, err
	}
	return packageDB, nil
}

func scanSearchResults(rows *sql.Rows) ([]SearchResult, error) {
	var results []SearchResult
	for rows.Next() {
		var result SearchResult
		var testament string
		if err := rows.Scan(
			&result.Book.ID, &result.Book.Name, &result.Book.Abbreviation, &testament,
			&result.Chapter, &result.Verse, &result.Text,
		); err != nil {
			return nil, fmt.Errorf("read verse: %w", err)
		}
		result.Book.Testament = Testament(testament)
		results = append(results, result)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("read verses: %w", err)
	}
	return results, nil
}

func scanSearchResultsWithSnippet(rows *sql.Rows) ([]SearchResult, error) {
	var results []SearchResult
	for rows.Next() {
		var result SearchResult
		var testament string
		if err := rows.Scan(
			&result.Book.ID, &result.Book.Name, &result.Book.Abbreviation, &testament,
			&result.Chapter, &result.Verse, &result.Text, &result.Snippet,
		); err != nil {
			return nil, fmt.Errorf("read verse: %w", err)
		}
		result.Book.Testament = Testament(testament)
		results = append(results, result)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("read verses: %w", err)
	}
	return results, nil
}

func scanTranslation(row interface{ Scan(...any) error }, extra ...*string) (Translation, error) {
	var translation Translation
	var installedAt string
	dest := []any{
		&translation.ID, &translation.Name, &translation.Abbreviation, &translation.Language,
		&translation.Version, &translation.VerseCount, &translation.Checksum, &installedAt,
		&translation.License, &translation.Attribution,
	}
	for _, ptr := range extra {
		dest = append(dest, ptr)
	}

	if err := row.Scan(dest...); err != nil {
		return Translation{}, err
	}
	parsed, err := time.Parse(time.RFC3339, installedAt)
	if err != nil {
		return Translation{}, fmt.Errorf("parse install time: %w", err)
	}
	translation.InstalledAt = parsed
	// Catalog rows written before license storage existed read back empty,
	// and the registry is the record of what is known about a text, so a
	// read fills what storage left blank. Without this the bundled KJV -
	// installed long before the license columns - shows no attribution
	// anywhere in the app. Restricted translations keep their deliberately
	// empty attribution: its wording is the rights holder's to give.
	if known, ok := LicenseFor(translation.ID); ok {
		if translation.License == "" {
			translation.License = known.License
		}
		if translation.Attribution == "" {
			translation.Attribution = known.Attribution
		}
	}
	return translation, nil
}

func validateTranslation(data TranslationData) error {
	if !translationIDPattern.MatchString(data.ID) {
		return fmt.Errorf("%w: id must be lowercase letters and digits", ErrInvalidTranslation)
	}
	if !translationVerPattern.MatchString(data.Version) {
		return fmt.Errorf("%w: version %q contains unsupported characters", ErrInvalidTranslation, data.Version)
	}
	if strings.TrimSpace(data.Name) == "" || strings.TrimSpace(data.Abbreviation) == "" || strings.TrimSpace(data.Language) == "" {
		return fmt.Errorf("%w: name, abbreviation and language are required", ErrInvalidTranslation)
	}
	if len(data.Verses) == 0 {
		return fmt.Errorf("%w: no verses", ErrInvalidTranslation)
	}

	for i, verse := range data.Verses {
		if _, ok := BookByID(verse.BookID); !ok {
			return fmt.Errorf("%w: verse %d has unknown book id %d", ErrInvalidTranslation, i+1, verse.BookID)
		}
		if verse.Chapter <= 0 || verse.Number <= 0 {
			return fmt.Errorf("%w: verse %d has an invalid chapter or verse number", ErrInvalidTranslation, i+1)
		}
		if strings.TrimSpace(verse.Text) == "" {
			return fmt.Errorf("%w: verse %d has empty text", ErrInvalidTranslation, i+1)
		}
	}
	return nil
}

func syncFile(path string) error {
	file, err := os.OpenFile(path, os.O_RDWR, 0)
	if err != nil {
		return err
	}
	defer file.Close()
	return file.Sync()
}

func fileChecksum(path string) (string, error) {
	file, err := os.Open(path)
	if err != nil {
		return "", fmt.Errorf("hash translation package: %w", err)
	}
	defer file.Close()

	hash := sha256.New()
	if _, err := io.Copy(hash, file); err != nil {
		return "", fmt.Errorf("hash translation package: %w", err)
	}
	return hex.EncodeToString(hash.Sum(nil)), nil
}

// publishPackage moves a verified package into place. When reinstalling the
// same version the previous file is set aside so a failed catalog update can
// restore it.
func publishPackage(tmpPath, finalPath string) (string, error) {
	var displaced string
	if _, err := os.Stat(finalPath); err == nil {
		displaced = finalPath + ".stale-" + fmt.Sprint(time.Now().UnixNano())
		if err := os.Rename(finalPath, displaced); err != nil {
			return "", fmt.Errorf("set aside existing translation package: %w", err)
		}
	} else if !errors.Is(err, os.ErrNotExist) {
		return "", fmt.Errorf("inspect existing translation package: %w", err)
	}

	if err := os.Rename(tmpPath, finalPath); err != nil {
		if displaced != "" {
			_ = os.Rename(displaced, finalPath)
		}
		return "", fmt.Errorf("publish translation package: %w", err)
	}
	return displaced, nil
}

// removeOtherPackages deletes older versions and stale files for one
// translation only. IDs cannot contain dashes, so the prefix is unambiguous.
func (r *SQLiteRepository) removeOtherPackages(id, keep string) {
	entries, err := os.ReadDir(r.dir)
	if err != nil {
		return
	}
	prefix := id + "-"
	for _, entry := range entries {
		name := entry.Name()
		if entry.IsDir() || name == keep || !strings.HasPrefix(name, prefix) {
			continue
		}
		_ = os.Remove(filepath.Join(r.dir, name))
	}
}
