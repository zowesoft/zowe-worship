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

func TestUpdateKeepsSectionIdentity(t *testing.T) {
	ctx := context.Background()
	repo, _ := newTestRepository(t)

	created, err := repo.Create(ctx, NewSong{
		Title:  "Amazing Grace",
		Author: "John Newton",
		Sections: []NewSection{
			{Label: "Verse 1", Text: "Amazing grace! how sweet the sound"},
			{Label: "Verse 2", Text: "Twas grace that taught my heart to fear"},
		},
	})
	if err != nil {
		t.Fatalf("create: %v", err)
	}
	first := created.Sections[0]
	second := created.Sections[1]

	updated, err := repo.Update(ctx, created.ID, SongUpdate{
		Title:  "Amazing Grace (New Britain)",
		Author: "John Newton",
		Sections: []SectionUpdate{
			// Same id, new lyrics: the slide keeps its identity.
			{ID: first.ID, Label: "Verse 1", Text: "Amazing grace! how sweet the sound, that saved a wretch like me"},
			// Verse 2 is dropped by simply not being sent.
			{Label: "Chorus", Text: "Grace, grace, God's grace"},
		},
	})
	if err != nil {
		t.Fatalf("update: %v", err)
	}

	if updated.Title != "Amazing Grace (New Britain)" {
		t.Fatalf("title = %q", updated.Title)
	}
	if len(updated.Sections) != 2 {
		t.Fatalf("sections = %+v", updated.Sections)
	}
	if updated.Sections[0].ID != first.ID {
		t.Fatalf("kept section changed id: %s -> %s", first.ID, updated.Sections[0].ID)
	}
	if updated.Sections[0].Text != "Amazing grace! how sweet the sound, that saved a wretch like me" {
		t.Fatalf("kept section text = %q", updated.Sections[0].Text)
	}
	if updated.Sections[1].ID == "" || updated.Sections[1].ID == second.ID {
		t.Fatalf("added section should be new: %+v", updated.Sections[1])
	}
	for _, section := range updated.Sections {
		if section.ID == second.ID {
			t.Fatal("a dropped section survived the update")
		}
	}

	// The lyric index has to follow the edit in both directions.
	if _, _, err := repo.Section(ctx, created.ID, second.ID); !errors.Is(err, ErrSectionNotFound) {
		t.Fatalf("dropped section err = %v", err)
	}
	hits, err := repo.Search(ctx, "wretch", 10)
	if err != nil {
		t.Fatalf("search: %v", err)
	}
	if len(hits) != 1 || hits[0].Section.ID != first.ID {
		t.Fatalf("search hits = %+v", hits)
	}
	removed, err := repo.Search(ctx, "taught", 10)
	if err != nil {
		t.Fatalf("search removed lyrics: %v", err)
	}
	if len(removed) != 0 {
		t.Fatalf("removed lyrics are still searchable: %+v", removed)
	}

	if _, err := repo.Update(ctx, created.ID, SongUpdate{Title: "  "}); !errors.Is(err, ErrInvalidSong) {
		t.Fatalf("empty title err = %v", err)
	}
	if _, err := repo.Update(ctx, "missing", SongUpdate{
		Title:    "Song",
		Sections: []SectionUpdate{{Label: "Verse", Text: "words"}},
	}); !errors.Is(err, ErrSongNotFound) {
		t.Fatalf("missing song err = %v", err)
	}
}

func TestSongArrangementsAndDuplication(t *testing.T) {
	ctx := context.Background()
	repo, _ := newTestRepository(t)

	song, err := repo.Create(ctx, NewSong{
		Title:      "10,000 Reasons",
		Author:     "Matt Redman",
		Copyright:  "2011 Thankyou Music",
		CCLINumber: "6016351",
		Source:     "CCLI",
		Sections: []NewSection{
			{Label: "Chorus", Text: "Bless the Lord, O my soul\nO my soul, worship His holy name"},
			{Label: "Verse 1", Text: "The sun comes up, it's a new day dawning\nIt's time to sing Your song again"},
			{Label: "Verse 2", Text: "You're rich in love, and You're slow to anger\nYour name is great, and Your heart is kind"},
		},
	})
	if err != nil {
		t.Fatalf("create song: %v", err)
	}

	if song.CCLINumber != "6016351" || song.Source != "CCLI" {
		t.Fatalf("metadata missing: CCLI=%q, Source=%q", song.CCLINumber, song.Source)
	}

	// Should have default arrangement automatically created
	if len(song.Arrangements) != 1 {
		t.Fatalf("expected 1 arrangement, got %d", len(song.Arrangements))
	}
	defaultArr := song.Arrangements[0]
	if defaultArr.Name != "Default" || !defaultArr.IsDefault || len(defaultArr.Items) != 3 {
		t.Fatalf("unexpected default arrangement: %+v", defaultArr)
	}

	// Create custom arrangement: Chorus -> Verse 1 -> Chorus -> Verse 2 -> Chorus
	chorusID := song.Sections[0].ID
	v1ID := song.Sections[1].ID
	v2ID := song.Sections[2].ID

	customArr, err := repo.CreateArrangement(ctx, NewArrangement{
		SongID:     song.ID,
		Name:       "Full Service Flow",
		IsDefault:  false,
		SectionIDs: []string{chorusID, v1ID, chorusID, v2ID, chorusID},
	})
	if err != nil {
		t.Fatalf("create custom arrangement: %v", err)
	}
	if len(customArr.Items) != 5 {
		t.Fatalf("expected 5 items in arrangement, got %d", len(customArr.Items))
	}
	if customArr.Items[0].Label != "Chorus" || customArr.Items[1].Label != "Verse 1" || customArr.Items[2].Label != "Chorus" {
		t.Fatalf("unexpected arrangement sequence: %+v", customArr.Items)
	}

	// Update custom arrangement
	updatedArr, err := repo.UpdateArrangement(ctx, customArr.ID, ArrangementUpdate{
		Name:       "Acoustic Flow",
		IsDefault:  false,
		SectionIDs: []string{v1ID, chorusID},
	})
	if err != nil {
		t.Fatalf("update arrangement: %v", err)
	}
	if updatedArr.Name != "Acoustic Flow" || len(updatedArr.Items) != 2 {
		t.Fatalf("updated arrangement = %+v", updatedArr)
	}

	// Duplicate song
	copied, err := repo.Duplicate(ctx, song.ID, "10,000 Reasons (Reprise)")
	if err != nil {
		t.Fatalf("duplicate song: %v", err)
	}
	if copied.ID == song.ID {
		t.Fatal("duplicated song must have a new ID")
	}
	if copied.Title != "10,000 Reasons (Reprise)" {
		t.Fatalf("copied title = %q", copied.Title)
	}
	if copied.CCLINumber != "6016351" || copied.Source != "CCLI" {
		t.Fatalf("copied metadata mismatch: CCLI=%q, Source=%q", copied.CCLINumber, copied.Source)
	}
	if len(copied.Sections) != 3 {
		t.Fatalf("copied sections = %d, want 3", len(copied.Sections))
	}
	if copied.Sections[0].ID == song.Sections[0].ID {
		t.Fatal("duplicated sections must have new IDs")
	}
	// Duplicated arrangements should also exist
	if len(copied.Arrangements) != 2 {
		t.Fatalf("expected 2 arrangements in duplicate, got %d", len(copied.Arrangements))
	}
}


