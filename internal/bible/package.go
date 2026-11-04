package bible

import (
	"context"
	"database/sql"
	"fmt"
	"strings"
	"unicode"

	"zowe-worship/internal/database"
)

var packageStatements = []string{
	`CREATE TABLE books (
		id           INTEGER PRIMARY KEY,
		name         TEXT NOT NULL,
		abbreviation TEXT NOT NULL,
		testament    TEXT NOT NULL CHECK (testament IN ('old', 'new')),
		position     INTEGER NOT NULL UNIQUE
	)`,
	`CREATE TABLE verses (
		id      INTEGER PRIMARY KEY,
		book_id INTEGER NOT NULL REFERENCES books(id) ON DELETE RESTRICT,
		chapter INTEGER NOT NULL CHECK (chapter > 0),
		verse   INTEGER NOT NULL CHECK (verse > 0),
		text    TEXT NOT NULL,
		UNIQUE (book_id, chapter, verse)
	)`,
	`CREATE VIRTUAL TABLE verses_fts USING fts5(
		text,
		content='verses',
		content_rowid='id',
		tokenize="unicode61 remove_diacritics 2"
	)`,
	`CREATE TRIGGER verses_after_insert AFTER INSERT ON verses BEGIN
		INSERT INTO verses_fts (rowid, text) VALUES (new.id, new.text);
	END`,
	`CREATE TRIGGER verses_after_delete AFTER DELETE ON verses BEGIN
		INSERT INTO verses_fts (verses_fts, rowid, text) VALUES ('delete', old.id, old.text);
	END`,
	`CREATE TRIGGER verses_after_update AFTER UPDATE ON verses BEGIN
		INSERT INTO verses_fts (verses_fts, rowid, text) VALUES ('delete', old.id, old.text);
		INSERT INTO verses_fts (rowid, text) VALUES (new.id, new.text);
	END`,
}

func buildPackage(ctx context.Context, path string, data TranslationData) error {
	db, err := database.OpenForImport(ctx, path)
	if err != nil {
		return err
	}
	defer db.Close()

	tx, err := db.BeginTx(ctx, nil)
	if err != nil {
		return fmt.Errorf("begin translation import: %w", err)
	}

	for _, statement := range packageStatements {
		if _, err := tx.ExecContext(ctx, statement); err != nil {
			_ = tx.Rollback()
			return fmt.Errorf("create translation schema: %w", err)
		}
	}
	if _, err := tx.ExecContext(ctx, fmt.Sprintf(`PRAGMA user_version = %d`, packageSchemaVersion)); err != nil {
		_ = tx.Rollback()
		return fmt.Errorf("stamp translation schema version: %w", err)
	}

	present := make(map[int]bool, len(data.Verses))
	for _, verse := range data.Verses {
		present[verse.BookID] = true
	}
	for _, book := range CanonicalBooks {
		if !present[book.ID] {
			continue
		}
		if _, err := tx.ExecContext(ctx, `
			INSERT INTO books (id, name, abbreviation, testament, position)
			VALUES (?, ?, ?, ?, ?)`,
			book.ID, book.Name, book.Abbreviation, book.Testament, book.ID,
		); err != nil {
			_ = tx.Rollback()
			return fmt.Errorf("insert book %s: %w", book.Name, err)
		}
	}

	insertVerse, err := tx.PrepareContext(ctx, `
		INSERT INTO verses (book_id, chapter, verse, text) VALUES (?, ?, ?, ?)`)
	if err != nil {
		_ = tx.Rollback()
		return fmt.Errorf("prepare verse import: %w", err)
	}
	for _, verse := range data.Verses {
		if _, err := insertVerse.ExecContext(ctx, verse.BookID, verse.Chapter, verse.Number, verse.Text); err != nil {
			_ = insertVerse.Close()
			_ = tx.Rollback()
			return fmt.Errorf("insert verse %d %d:%d: %w", verse.BookID, verse.Chapter, verse.Number, err)
		}
	}
	if err := insertVerse.Close(); err != nil {
		_ = tx.Rollback()
		return fmt.Errorf("finish verse import: %w", err)
	}

	if _, err := tx.ExecContext(ctx, `INSERT INTO verses_fts (verses_fts) VALUES ('integrity-check')`); err != nil {
		_ = tx.Rollback()
		return fmt.Errorf("verify full-text index: %w", err)
	}

	if err := tx.Commit(); err != nil {
		return fmt.Errorf("commit translation import: %w", err)
	}
	if err := db.Close(); err != nil {
		return fmt.Errorf("close translation package: %w", err)
	}
	return nil
}

func checkPackageVersion(ctx context.Context, db *sql.DB) error {
	var version int
	if err := db.QueryRowContext(ctx, `PRAGMA user_version`).Scan(&version); err != nil {
		return fmt.Errorf("read translation package version: %w", err)
	}
	if version != packageSchemaVersion {
		return fmt.Errorf("%w: package schema %d", ErrUnsupportedPackage, version)
	}
	return nil
}

// searchTerms splits a query into the words worth matching, dropping tokens
// that carry no letter or digit (bare punctuation, symbols).
func searchTerms(text string) ([]string, error) {
	words := make([]string, 0, 8)
	for _, field := range strings.Fields(text) {
		if hasSearchCharacter(field) {
			words = append(words, field)
		}
	}
	if len(words) == 0 {
		return nil, ErrEmptyQuery
	}
	return words, nil
}

// prefixTerms quotes each word and adds the FTS5 prefix operator. The installed
// packages tokenize with unicode61, which has no stemmer, so "love" would never
// reach "loved" on its own — the prefix is how we bridge inflections without
// rebuilding every translation database.
func prefixTerms(words []string) []string {
	terms := make([]string, len(words))
	for i, word := range words {
		terms[i] = quoteFTSTerm(word) + "*"
	}
	return terms
}

// relaxedTerms returns each query word as an FTS5 prefix term, ready to be
// joined with AND (every word required) or OR (any word).
func relaxedTerms(text string) ([]string, error) {
	words, err := searchTerms(text)
	if err != nil {
		return nil, err
	}
	return prefixTerms(words), nil
}

func matchExpression(text string, mode SearchMode) (string, error) {
	words, err := searchTerms(text)
	if err != nil {
		return "", err
	}

	switch mode {
	case SearchPhrase:
		return quoteFTSTerm(strings.Join(words, " ")), nil
	case SearchPrefix:
		quoted := make([]string, len(words))
		for i, word := range words {
			quoted[i] = quoteFTSTerm(word)
		}
		quoted[len(quoted)-1] += "*"
		return strings.Join(quoted, " AND "), nil
	default:
		return "", fmt.Errorf("unknown search mode %q", mode)
	}
}

func quoteFTSTerm(term string) string {
	return `"` + strings.ReplaceAll(term, `"`, `""`) + `"`
}

func hasSearchCharacter(token string) bool {
	for _, r := range token {
		if unicode.IsLetter(r) || unicode.IsDigit(r) {
			return true
		}
	}
	return false
}
