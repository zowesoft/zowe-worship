package bible

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"zopresent/internal/database"
)

func newTestRepository(t *testing.T) (*SQLiteRepository, string) {
	t.Helper()

	paths := database.Paths{Root: filepath.Join(t.TempDir(), "Zopresent")}
	if err := paths.Ensure(); err != nil {
		t.Fatalf("prepare data directories: %v", err)
	}
	db, err := database.Open(context.Background(), paths.CoreDatabase())
	if err != nil {
		t.Fatalf("open core database: %v", err)
	}
	t.Cleanup(func() { _ = db.Close() })
	if err := database.Migrate(context.Background(), db, paths.Backups()); err != nil {
		t.Fatalf("migrate core database: %v", err)
	}
	return NewSQLiteRepository(db, paths.Translations()), paths.Translations()
}

func fixture(id, version, shepherdVerse string) TranslationData {
	return TranslationData{
		TranslationMeta: TranslationMeta{
			ID:           id,
			Name:         strings.ToUpper(id) + " Test Bible",
			Abbreviation: strings.ToUpper(id),
			Language:     "en",
			Version:      version,
		},
		Verses: []Verse{
			{BookID: 19, Chapter: 23, Number: 1, Text: shepherdVerse},
			{BookID: 19, Chapter: 23, Number: 2, Text: "He maketh me to lie down in green pastures."},
			{BookID: 43, Chapter: 3, Number: 16, Text: "For God so loved the world, that he gave his only begotten Son."},
			{BookID: 43, Chapter: 4, Number: 24, Text: "God is a Spirit: and they that worship him must worship him in spirit and in truth."},
			{BookID: 58, Chapter: 11, Number: 1, Text: "Now faith is the substance of things hoped for, the evidence of things not seen."},
		},
	}
}

func TestInstallAndSearch(t *testing.T) {
	ctx := context.Background()
	repo, dir := newTestRepository(t)

	installed, err := repo.Install(ctx, fixture("kjv", "1.0.0", "The LORD is my shepherd; I shall not want."))
	if err != nil {
		t.Fatalf("install: %v", err)
	}
	if installed.VerseCount != 5 {
		t.Fatalf("verse count = %d, want 5", installed.VerseCount)
	}
	if len(installed.Checksum) != 64 {
		t.Fatalf("checksum = %q, want sha256 hex", installed.Checksum)
	}
	if _, err := os.Stat(filepath.Join(dir, "kjv-1.0.0.db")); err != nil {
		t.Fatalf("package file missing: %v", err)
	}

	cases := []struct {
		name    string
		request SearchRequest
		want    []string
	}{
		{
			name:    "single word",
			request: SearchRequest{TranslationID: "kjv", Text: "shepherd", Mode: SearchWords},
			want:    []string{"Psalms 23:1"},
		},
		{
			name:    "multiple words require all terms",
			request: SearchRequest{TranslationID: "kjv", Text: "God loved", Mode: SearchWords},
			want:    []string{"John 3:16"},
		},
		{
			name:    "a word reaches its inflections",
			request: SearchRequest{TranslationID: "kjv", Text: "God love world", Mode: SearchWords},
			want:    []string{"John 3:16"},
		},
		{
			name:    "exact phrase ignores punctuation and case",
			request: SearchRequest{TranslationID: "kjv", Text: "god is a spirit", Mode: SearchPhrase},
			want:    []string{"John 4:24"},
		},
		{
			name:    "phrase does not match separated words",
			request: SearchRequest{TranslationID: "kjv", Text: "spirit worship", Mode: SearchPhrase},
			want:    nil,
		},
		{
			name:    "word prefix",
			request: SearchRequest{TranslationID: "kjv", Text: "shep", Mode: SearchPrefix},
			want:    []string{"Psalms 23:1"},
		},
		{
			name:    "prefix on final term",
			request: SearchRequest{TranslationID: "kjv", Text: "green past", Mode: SearchPrefix},
			want:    []string{"Psalms 23:2"},
		},
		{
			name:    "no matches",
			request: SearchRequest{TranslationID: "kjv", Text: "tabernacle", Mode: SearchWords},
			want:    nil,
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			results, err := repo.Search(ctx, tc.request)
			if err != nil {
				t.Fatalf("search: %v", err)
			}
			var references []string
			for _, result := range results {
				references = append(references, result.Reference())
			}
			if strings.Join(references, ",") != strings.Join(tc.want, ",") {
				t.Fatalf("references = %v, want %v", references, tc.want)
			}
		})
	}
}

func TestSearchWidensWhenNothingHasEveryWord(t *testing.T) {
	ctx := context.Background()
	repo, _ := newTestRepository(t)
	if _, err := repo.Install(ctx, fixture("kjv", "1.0.0", "The LORD is my shepherd; I shall not want.")); err != nil {
		t.Fatalf("install: %v", err)
	}

	// No single verse carries all three words, so the search widens to any of
	// them. John 3:16 matches two of the three ("God", "loved"), so bm25 floats
	// it above the verses that only carry one.
	results, err := repo.Search(ctx, SearchRequest{TranslationID: "kjv", Text: "God loved shepherd", Mode: SearchWords})
	if err != nil {
		t.Fatalf("search: %v", err)
	}
	if len(results) == 0 {
		t.Fatal("widened search returned nothing")
	}
	if results[0].Reference() != "John 3:16" {
		t.Fatalf("top result = %q, want the verse matching the most words (John 3:16)", results[0].Reference())
	}

	seen := map[string]bool{}
	for _, result := range results {
		seen[result.Reference()] = true
	}
	for _, want := range []string{"John 3:16", "Psalms 23:1", "John 4:24"} {
		if !seen[want] {
			t.Fatalf("widened search missing %q; got %v", want, results)
		}
	}
}

func TestSearchRejectsEmptyQueries(t *testing.T) {
	ctx := context.Background()
	repo, _ := newTestRepository(t)
	if _, err := repo.Install(ctx, fixture("kjv", "1.0.0", "The LORD is my shepherd.")); err != nil {
		t.Fatalf("install: %v", err)
	}

	for _, text := range []string{"", "   ", "?!", "\"\""} {
		if _, err := repo.Search(ctx, SearchRequest{TranslationID: "kjv", Text: text}); !errors.Is(err, ErrEmptyQuery) {
			t.Fatalf("text %q: err = %v, want ErrEmptyQuery", text, err)
		}
	}

	if _, err := repo.Search(ctx, SearchRequest{TranslationID: "niv", Text: "grace"}); !errors.Is(err, ErrTranslationNotFound) {
		t.Fatalf("unknown translation err = %v, want ErrTranslationNotFound", err)
	}
}

func TestInstallUpdateAndRemoveStayIsolated(t *testing.T) {
	ctx := context.Background()
	repo, dir := newTestRepository(t)

	if _, err := repo.Install(ctx, fixture("kjv", "1.0.0", "The LORD is my shepherd; I shall not want.")); err != nil {
		t.Fatalf("install kjv: %v", err)
	}
	if _, err := repo.Install(ctx, fixture("amp", "1.0.0", "The Lord is my Shepherd [to feed, guide and shield me].")); err != nil {
		t.Fatalf("install amp: %v", err)
	}

	updated, err := repo.Install(ctx, fixture("kjv", "1.1.0", "The Lord is my shepherd; I lack nothing."))
	if err != nil {
		t.Fatalf("update kjv: %v", err)
	}
	if updated.Version != "1.1.0" {
		t.Fatalf("version = %q, want 1.1.0", updated.Version)
	}

	translations, err := repo.ListTranslations(ctx)
	if err != nil {
		t.Fatalf("list translations: %v", err)
	}
	if len(translations) != 2 {
		t.Fatalf("translations = %d, want 2", len(translations))
	}

	oldResults, err := repo.Search(ctx, SearchRequest{TranslationID: "kjv", Text: "want", Mode: SearchWords})
	if err != nil {
		t.Fatalf("search old kjv text: %v", err)
	}
	if len(oldResults) != 0 {
		t.Fatalf("old KJV text still indexed: %v", oldResults)
	}

	ampResults, err := repo.Search(ctx, SearchRequest{TranslationID: "amp", Text: "shield", Mode: SearchWords})
	if err != nil {
		t.Fatalf("search amp after kjv update: %v", err)
	}
	if len(ampResults) != 1 || ampResults[0].Reference() != "Psalms 23:1" {
		t.Fatalf("amp results after kjv update = %v", ampResults)
	}

	entries, err := os.ReadDir(dir)
	if err != nil {
		t.Fatalf("read translations dir: %v", err)
	}
	var names []string
	for _, entry := range entries {
		names = append(names, entry.Name())
	}
	if strings.Join(names, ",") != "amp-1.0.0.db,kjv-1.1.0.db" {
		t.Fatalf("package files = %v", names)
	}

	if err := repo.Remove(ctx, "amp"); err != nil {
		t.Fatalf("remove amp: %v", err)
	}
	if _, err := repo.Translation(ctx, "amp"); !errors.Is(err, ErrTranslationNotFound) {
		t.Fatalf("removed translation err = %v, want ErrTranslationNotFound", err)
	}
	if _, err := os.Stat(filepath.Join(dir, "amp-1.0.0.db")); !errors.Is(err, os.ErrNotExist) {
		t.Fatalf("amp package still exists: %v", err)
	}

	kjvResults, err := repo.Search(ctx, SearchRequest{TranslationID: "kjv", Text: "shepherd", Mode: SearchWords})
	if err != nil {
		t.Fatalf("search kjv after removing amp: %v", err)
	}
	if len(kjvResults) != 1 {
		t.Fatalf("kjv results after removing amp = %v", kjvResults)
	}
	if err := repo.Remove(ctx, "amp"); !errors.Is(err, ErrTranslationNotFound) {
		t.Fatalf("second remove err = %v, want ErrTranslationNotFound", err)
	}
}

func TestReinstallSameVersionReplacesContent(t *testing.T) {
	ctx := context.Background()
	repo, dir := newTestRepository(t)

	if _, err := repo.Install(ctx, fixture("kjv", "1.0.0", "The LORD is my shepherd.")); err != nil {
		t.Fatalf("install: %v", err)
	}
	if _, err := repo.Install(ctx, fixture("kjv", "1.0.0", "The Lord is my shepherd; I lack nothing.")); err != nil {
		t.Fatalf("reinstall: %v", err)
	}

	results, err := repo.Search(ctx, SearchRequest{TranslationID: "kjv", Text: "lack", Mode: SearchWords})
	if err != nil {
		t.Fatalf("search: %v", err)
	}
	if len(results) != 1 {
		t.Fatalf("reinstalled text not searchable: %v", results)
	}

	entries, err := os.ReadDir(dir)
	if err != nil {
		t.Fatalf("read translations dir: %v", err)
	}
	if len(entries) != 1 || entries[0].Name() != "kjv-1.0.0.db" {
		t.Fatalf("package files = %v", entries)
	}
}

func TestInstallRejectsInvalidDataWithoutPublishing(t *testing.T) {
	ctx := context.Background()
	repo, dir := newTestRepository(t)

	cases := map[string]TranslationData{
		"uppercase id": func() TranslationData {
			data := fixture("kjv", "1.0.0", "The LORD is my shepherd.")
			data.ID = "KJV"
			return data
		}(),
		"unknown book": func() TranslationData {
			data := fixture("kjv", "1.0.0", "The LORD is my shepherd.")
			data.Verses[0].BookID = 99
			return data
		}(),
		"empty text": func() TranslationData {
			data := fixture("kjv", "1.0.0", "The LORD is my shepherd.")
			data.Verses[0].Text = "   "
			return data
		}(),
		"no verses": {TranslationMeta: TranslationMeta{
			ID: "kjv", Name: "King James Version", Abbreviation: "KJV", Language: "en", Version: "1.0.0",
		}},
	}

	for name, data := range cases {
		if _, err := repo.Install(ctx, data); !errors.Is(err, ErrInvalidTranslation) {
			t.Fatalf("%s: err = %v, want ErrInvalidTranslation", name, err)
		}
	}

	translations, err := repo.ListTranslations(ctx)
	if err != nil {
		t.Fatalf("list translations: %v", err)
	}
	if len(translations) != 0 {
		t.Fatalf("translations after failed installs = %d, want 0", len(translations))
	}
	entries, err := os.ReadDir(dir)
	if err != nil {
		t.Fatalf("read translations dir: %v", err)
	}
	if len(entries) != 0 {
		t.Fatalf("stray package files after failed installs = %v", entries)
	}
}

func TestPassageReturnsWholeChapterInOrder(t *testing.T) {
	ctx := context.Background()
	repo, _ := newTestRepository(t)
	if _, err := repo.Install(ctx, fixture("kjv", "1.0.0", "The LORD is my shepherd; I shall not want.")); err != nil {
		t.Fatalf("install: %v", err)
	}

	results, err := repo.Passage(ctx, "kjv", 19, 23, 0, 0)
	if err != nil {
		t.Fatalf("passage: %v", err)
	}
	if len(results) != 2 {
		t.Fatalf("verses = %d, want 2", len(results))
	}
	if results[0].Reference() != "Psalms 23:1" || results[1].Reference() != "Psalms 23:2" {
		t.Fatalf("verse order = %v", results)
	}

	single, err := repo.Passage(ctx, "kjv", 19, 23, 2, 2)
	if err != nil {
		t.Fatalf("single verse: %v", err)
	}
	if len(single) != 1 || single[0].Reference() != "Psalms 23:2" {
		t.Fatalf("single verse = %v", single)
	}

	ranged, err := repo.Passage(ctx, "kjv", 19, 23, 1, 2)
	if err != nil {
		t.Fatalf("range: %v", err)
	}
	if len(ranged) != 2 {
		t.Fatalf("range verses = %d, want 2", len(ranged))
	}

	missing, err := repo.Passage(ctx, "kjv", 19, 24, 0, 0)
	if err != nil {
		t.Fatalf("missing chapter: %v", err)
	}
	if len(missing) != 0 {
		t.Fatalf("missing chapter returned %v", missing)
	}

	missingVerse, err := repo.Passage(ctx, "kjv", 43, 3, 17, 0)
	if err != nil {
		t.Fatalf("missing verse: %v", err)
	}
	if len(missingVerse) != 0 {
		t.Fatalf("missing verse returned %v", missingVerse)
	}
}

func TestPassageRejectsInvalidArguments(t *testing.T) {
	ctx := context.Background()
	repo, _ := newTestRepository(t)
	if _, err := repo.Install(ctx, fixture("kjv", "1.0.0", "The LORD is my shepherd.")); err != nil {
		t.Fatalf("install: %v", err)
	}

	cases := []struct {
		name                        string
		bookID, chapter, start, end int
	}{
		{"unknown book", 99, 1, 0, 0},
		{"zero chapter", 19, 0, 0, 0},
		{"negative verse", 19, 23, -1, 0},
		{"range without start", 19, 23, 0, 2},
		{"reversed range", 19, 23, 2, 1},
	}

	for _, tc := range cases {
		if _, err := repo.Passage(ctx, "kjv", tc.bookID, tc.chapter, tc.start, tc.end); err == nil {
			t.Fatalf("%s: expected an error", tc.name)
		}
	}

	if _, err := repo.Passage(ctx, "niv", 19, 23, 0, 0); !errors.Is(err, ErrTranslationNotFound) {
		t.Fatalf("unknown translation err = %v, want ErrTranslationNotFound", err)
	}
}

func TestSearchSurvivesRepositoryReopen(t *testing.T) {
	ctx := context.Background()
	paths := database.Paths{Root: filepath.Join(t.TempDir(), "Zopresent")}
	if err := paths.Ensure(); err != nil {
		t.Fatalf("prepare directories: %v", err)
	}

	core, err := database.Open(ctx, paths.CoreDatabase())
	if err != nil {
		t.Fatalf("open core: %v", err)
	}
	if err := database.Migrate(ctx, core, paths.Backups()); err != nil {
		t.Fatalf("migrate: %v", err)
	}
	repo := NewSQLiteRepository(core, paths.Translations())
	if _, err := repo.Install(ctx, fixture("kjv", "1.0.0", "The LORD is my shepherd.")); err != nil {
		t.Fatalf("install: %v", err)
	}
	if err := core.Close(); err != nil {
		t.Fatalf("close core: %v", err)
	}

	reopened, err := database.Open(ctx, paths.CoreDatabase())
	if err != nil {
		t.Fatalf("reopen core: %v", err)
	}
	t.Cleanup(func() { _ = reopened.Close() })
	repo = NewSQLiteRepository(reopened, paths.Translations())

	results, err := repo.Search(ctx, SearchRequest{TranslationID: "kjv", Text: "shepherd", Mode: SearchWords})
	if err != nil {
		t.Fatalf("search after reopen: %v", err)
	}
	if len(results) != 1 {
		t.Fatalf("results after reopen = %v", results)
	}
}

func TestMatchExpressionQuotesUserInput(t *testing.T) {
	cases := []struct {
		text string
		mode SearchMode
		want string
	}{
		{"the lord is my shepherd", SearchPhrase, `"the lord is my shepherd"`},
		{"shep", SearchPrefix, `"shep"*`},
		{"green past", SearchPrefix, `"green" AND "past"*`},
		{`"NEAR(foo bar)"`, SearchPhrase, `"""NEAR(foo bar)"""`},
	}

	for _, tc := range cases {
		got, err := matchExpression(tc.text, tc.mode)
		if err != nil {
			t.Fatalf("%q: %v", tc.text, err)
		}
		if got != tc.want {
			t.Fatalf("%q: got %s, want %s", tc.text, got, tc.want)
		}
	}
}

func TestRelaxedTermsPrefixesAndQuotesEveryWord(t *testing.T) {
	cases := []struct {
		text string
		want []string
	}{
		{"God love world", []string{`"God"*`, `"love"*`, `"world"*`}},
		{"grace, faith.", []string{`"grace,"*`, `"faith."*`}},
		{`"NEAR(foo bar)"`, []string{`"""NEAR(foo"*`, `"bar)"""*`}},
	}

	for _, tc := range cases {
		got, err := relaxedTerms(tc.text)
		if err != nil {
			t.Fatalf("%q: %v", tc.text, err)
		}
		if strings.Join(got, " ") != strings.Join(tc.want, " ") {
			t.Fatalf("%q: got %v, want %v", tc.text, got, tc.want)
		}
	}

	if _, err := relaxedTerms("?!"); !errors.Is(err, ErrEmptyQuery) {
		t.Fatalf("punctuation-only query err = %v, want ErrEmptyQuery", err)
	}
}
