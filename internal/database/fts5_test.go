package database_test

import (
	"database/sql"
	"path/filepath"
	"testing"

	_ "modernc.org/sqlite"
)

func openScratch(t *testing.T) *sql.DB {
	t.Helper()

	path := filepath.Join(t.TempDir(), "fts5 probe.db")
	db, err := sql.Open("sqlite", "file:"+filepath.ToSlash(path))
	if err != nil {
		t.Fatalf("open sqlite: %v", err)
	}
	t.Cleanup(func() { _ = db.Close() })
	return db
}

func TestFTS5SupportsScriptureSearch(t *testing.T) {
	db := openScratch(t)

	if _, err := db.Exec(`
		CREATE TABLE verses (
			id INTEGER PRIMARY KEY,
			reference TEXT NOT NULL,
			text TEXT NOT NULL
		);
		CREATE VIRTUAL TABLE verses_fts USING fts5(
			text,
			content='verses',
			content_rowid='id',
			tokenize="unicode61 remove_diacritics 2"
		);
	`); err != nil {
		t.Fatalf("create fts5 table: %v", err)
	}

	verses := []struct{ reference, text string }{
		{"Psalms 23:1", "The LORD is my shepherd; I shall not want."},
		{"Psalms 23:2", "He maketh me to lie down in green pastures: he leadeth me beside the still waters."},
		{"John 3:16", "For God so loved the world, that he gave his only begotten Son."},
		{"John 4:24", "God is a Spirit: and they that worship him must worship him in spirit and in truth."},
		{"Hebrews 11:1", "Now faith is the substance of things hoped for, the evidence of things not seen."},
	}
	for _, v := range verses {
		if _, err := db.Exec(
			`INSERT INTO verses (reference, text) VALUES (?, ?)`, v.reference, v.text,
		); err != nil {
			t.Fatalf("insert verse: %v", err)
		}
	}
	if _, err := db.Exec(`INSERT INTO verses_fts (verses_fts) VALUES ('rebuild')`); err != nil {
		t.Fatalf("rebuild index: %v", err)
	}

	cases := []struct {
		name  string
		match string
		want  int
	}{
		{"single word", `"shepherd"`, 1},
		{"word with case difference", `"lord"`, 1},
		{"exact phrase", `"the lord is my shepherd"`, 1},
		{"phrase without punctuation", `"god is a spirit"`, 1},
		{"prefix", `"shep"*`, 1},
		{"prefix with multiple words", `"worship"* AND "spirit"`, 1},
		{"no match", `"tabernacle"`, 0},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			rows, err := db.Query(`
				SELECT v.reference, bm25(verses_fts) AS rank
				FROM verses_fts
				JOIN verses v ON v.id = verses_fts.rowid
				WHERE verses_fts MATCH ?
				ORDER BY rank, v.id`, tc.match)
			if err != nil {
				t.Fatalf("query %q: %v", tc.match, err)
			}
			defer rows.Close()

			var found int
			for rows.Next() {
				var reference string
				var rank float64
				if err := rows.Scan(&reference, &rank); err != nil {
					t.Fatalf("scan: %v", err)
				}
				found++
			}
			if err := rows.Err(); err != nil {
				t.Fatalf("iterate: %v", err)
			}
			if found != tc.want {
				t.Fatalf("query %q returned %d rows, want %d", tc.match, found, tc.want)
			}
		})
	}
}
