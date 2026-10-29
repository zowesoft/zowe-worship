package songs

import (
	"context"
	"errors"
	"path/filepath"
	"strings"
	"testing"

	"zowe-worship/internal/database"
)

func newTestRepository(t *testing.T) (*SQLiteRepository, string) {
	t.Helper()

	paths := database.Paths{Root: filepath.Join(t.TempDir(), "Zowe Worship")}
	if err := paths.Ensure(); err != nil {
		t.Fatalf("prepare directories: %v", err)
	}
	db, err := database.Open(context.Background(), paths.CoreDatabase())
	if err != nil {
		t.Fatalf("open core database: %v", err)
	}
	if err := database.Migrate(context.Background(), db, paths.Backups()); err != nil {
		t.Fatalf("migrate: %v", err)
	}
	t.Cleanup(func() { _ = db.Close() })
	return NewSQLiteRepository(db), paths.CoreDatabase()
}

func amazingGrace() NewSong {
	return NewSong{
		Title:     "Amazing Grace",
		Author:    "John Newton",
		Copyright: "Public domain",
		Sections: []NewSection{
			{Label: "Verse 1", Text: "Amazing grace! how sweet the sound,\nThat saved a wretch like me!\nI once was lost, but now am found,\nWas blind, but now I see."},
			{Label: "Verse 2", Text: "'Twas grace that taught my heart to fear,\nAnd grace my fears relieved;\nHow precious did that grace appear\nThe hour I first believed!"},
			{Label: "Chorus", Text: "My chains are gone, I'll be set free.\nGrace, grace, God's grace."},
		},
	}
}

func TestCreateListAndReadSections(t *testing.T) {
	ctx := context.Background()
	repo, _ := newTestRepository(t)

	created, err := repo.Create(ctx, amazingGrace())
	if err != nil {
		t.Fatalf("create song: %v", err)
	}
	if created.Title != "Amazing Grace" || len(created.Sections) != 3 {
		t.Fatalf("created = %+v", created)
	}
	if created.Sections[0].Position != 1 || created.Sections[2].Label != "Chorus" {
		t.Fatalf("sections out of order: %+v", created.Sections)
	}
	if !strings.Contains(created.Sections[0].Text, "saved a wretch") {
		t.Fatalf("section text changed: %q", created.Sections[0].Text)
	}

	list, err := repo.List(ctx)
	if err != nil {
		t.Fatalf("list: %v", err)
	}
	if len(list) != 1 || len(list[0].Sections) != 3 {
		t.Fatalf("list = %+v", list)
	}

	song, section, err := repo.Section(ctx, created.ID, created.Sections[2].ID)
	if err != nil {
		t.Fatalf("section: %v", err)
	}
	if song.Title != "Amazing Grace" || section.Label != "Chorus" {
		t.Fatalf("song = %+v section = %+v", song, section)
	}
}

func TestSearchLyrics(t *testing.T) {
	ctx := context.Background()
	repo, _ := newTestRepository(t)

	if _, err := repo.Create(ctx, amazingGrace()); err != nil {
		t.Fatalf("create: %v", err)
	}
	if _, err := repo.Create(ctx, NewSong{
		Title:    "Holy Holy Holy",
		Sections: []NewSection{{Label: "Verse 1", Text: "Holy, holy, holy! Lord God Almighty!"}},
	}); err != nil {
		t.Fatalf("create second: %v", err)
	}

	cases := []struct {
		name     string
		query    string
		wantSong string
	}{
		{"single word", "grace", "Amazing Grace"},
		{"multiple words", "sweet sound", "Amazing Grace"},
		{"case differences", "LORD GOD", "Holy Holy Holy"},
		{"punctuation ignored", "chains are gone", "Amazing Grace"},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			hits, err := repo.Search(ctx, tc.query, 10)
			if err != nil {
				t.Fatalf("search %q: %v", tc.query, err)
			}
			if len(hits) == 0 {
				t.Fatalf("search %q found nothing", tc.query)
			}
			if hits[0].Song.Title != tc.wantSong {
				t.Fatalf("search %q top hit = %q, want %q", tc.query, hits[0].Song.Title, tc.wantSong)
			}
			if hits[0].Section.Label == "" {
				t.Fatalf("hit missing section label: %+v", hits[0])
			}
		})
	}

	none, err := repo.Search(ctx, "nonexistentword", 10)
	if err != nil {
		t.Fatalf("search empty: %v", err)
	}
	if len(none) != 0 {
		t.Fatalf("unexpected hits: %+v", none)
	}

	if _, err := repo.Search(ctx, "   ", 10); !errors.Is(err, ErrEmptyQuery) {
		t.Fatalf("err = %v, want ErrEmptyQuery", err)
	}
}

func TestSongValidationAndErrors(t *testing.T) {
	ctx := context.Background()
	repo, _ := newTestRepository(t)

	bad := []NewSong{
		{Title: "  ", Sections: []NewSection{{Label: "Verse 1", Text: "text"}}},
		{Title: "No sections"},
		{Title: "Blank lyrics", Sections: []NewSection{{Label: "Verse 1", Text: "   "}}},
		{Title: "Blank label", Sections: []NewSection{{Label: "  ", Text: "text"}}},
	}
	for _, song := range bad {
		if _, err := repo.Create(ctx, song); !errors.Is(err, ErrInvalidSong) {
			t.Fatalf("%q: err = %v, want ErrInvalidSong", song.Title, err)
		}
	}

	if _, err := repo.Get(ctx, "missing"); !errors.Is(err, ErrSongNotFound) {
		t.Fatalf("get missing err = %v", err)
	}
	if _, _, err := repo.Section(ctx, "missing", "also-missing"); !errors.Is(err, ErrSongNotFound) {
		t.Fatalf("section missing song err = %v", err)
	}
	if err := repo.Delete(ctx, "missing"); !errors.Is(err, ErrSongNotFound) {
		t.Fatalf("delete missing err = %v", err)
	}

	created, err := repo.Create(ctx, amazingGrace())
	if err != nil {
		t.Fatalf("create: %v", err)
	}
	if _, _, err := repo.Section(ctx, created.ID, "wrong-section"); !errors.Is(err, ErrSectionNotFound) {
		t.Fatalf("wrong section err = %v", err)
	}
}

func TestDeleteRemovesLyricsFromSearch(t *testing.T) {
	ctx := context.Background()
	repo, _ := newTestRepository(t)

	created, err := repo.Create(ctx, amazingGrace())
	if err != nil {
		t.Fatalf("create: %v", err)
	}
	if err := repo.Delete(ctx, created.ID); err != nil {
		t.Fatalf("delete: %v", err)
	}

	hits, err := repo.Search(ctx, "grace", 10)
	if err != nil {
		t.Fatalf("search: %v", err)
	}
	if len(hits) != 0 {
		t.Fatalf("deleted song still indexed: %+v", hits)
	}
	list, err := repo.List(ctx)
	if err != nil {
		t.Fatalf("list: %v", err)
	}
	if len(list) != 0 {
		t.Fatalf("songs remain: %+v", list)
	}
}

func TestSongsSurviveReopen(t *testing.T) {
	ctx := context.Background()
	repo, path := newTestRepository(t)

	if _, err := repo.Create(ctx, amazingGrace()); err != nil {
		t.Fatalf("create: %v", err)
	}
	db := repo.db
	if err := db.Close(); err != nil {
		t.Fatalf("close: %v", err)
	}

	reopened, err := database.Open(ctx, path)
	if err != nil {
		t.Fatalf("reopen: %v", err)
	}
	t.Cleanup(func() { _ = reopened.Close() })

	list, err := NewSQLiteRepository(reopened).List(ctx)
	if err != nil {
		t.Fatalf("list after reopen: %v", err)
	}
	if len(list) != 1 || list[0].Title != "Amazing Grace" || len(list[0].Sections) != 3 {
		t.Fatalf("songs after reopen = %+v", list)
	}
}
