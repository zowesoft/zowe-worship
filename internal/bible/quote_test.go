package bible

import (
	"context"
	"testing"
)

func quoteFixture() TranslationData {
	return TranslationData{
		TranslationMeta: TranslationMeta{
			ID: "kjv", Name: "King James Version", Abbreviation: "KJV", Language: "en", Version: "1.0.0",
		},
		Verses: []Verse{
			{BookID: 50, Chapter: 4, Number: 6, Text: "Be careful for nothing; but in every thing by prayer and supplication with thanksgiving let your requests be made known unto God."},
			{BookID: 50, Chapter: 4, Number: 7, Text: "And the peace of God, which passeth all understanding, shall keep your hearts and minds through Christ Jesus."},
			{BookID: 43, Chapter: 3, Number: 16, Text: "For God so loved the world, that he gave his only begotten Son, that whosoever believeth in him should not perish, but have everlasting life."},
			{BookID: 19, Chapter: 23, Number: 1, Text: "The LORD is my shepherd; I shall not want."},
			{BookID: 40, Chapter: 6, Number: 25, Text: "Therefore I say unto you, Take no thought for your life, what ye shall eat, or what ye shall drink; nor yet for your body what ye shall put on."},
		},
	}
}

func TestContentWordsDropsGrammarAndNumbers(t *testing.T) {
	got := ContentWords("The LORD is my shepherd; I shall not want. John 3:16!")
	want := []string{"lord", "shepherd", "want", "john"}
	if len(got) != len(want) {
		t.Fatalf("ContentWords = %v, want %v", got, want)
	}
	for i := range want {
		if got[i] != want[i] {
			t.Fatalf("ContentWords = %v, want %v", got, want)
		}
	}
}

func TestMatchQuoteRanksTheQuotedVersesFirst(t *testing.T) {
	ctx := context.Background()
	repo, _ := newTestRepository(t)
	if _, err := repo.Install(ctx, quoteFixture()); err != nil {
		t.Fatalf("install: %v", err)
	}

	// Philippians 4:6-7 quoted verbatim, never named.
	transcript := "be careful for nothing but in every thing by prayer and supplication with thanksgiving " +
		"let your requests be made known unto God and the peace of God which passeth all understanding " +
		"shall keep your hearts and minds through Christ Jesus"

	matches, err := repo.MatchQuote(ctx, "kjv", transcript, 10)
	if err != nil {
		t.Fatalf("match quote: %v", err)
	}
	if len(matches) < 2 {
		t.Fatalf("matches = %d, want at least the two quoted verses", len(matches))
	}

	// The two quoted verses lead, in bm25 order, ahead of every distractor.
	top := map[int]bool{}
	for _, match := range matches[:2] {
		if match.Result.Book.ID != 50 || match.Result.Chapter != 4 {
			t.Fatalf("top match = %s, want Philippians 4", match.Result.Reference())
		}
		top[match.Result.Verse] = true
		if match.Score >= 0 {
			t.Errorf("bm25 score = %v, want a negative rank", match.Score)
		}
	}
	if !top[6] || !top[7] {
		t.Fatalf("top verses = %v, want 6 and 7", top)
	}
	if matches[0].Score > matches[1].Score {
		t.Errorf("matches not ranked: %v then %v", matches[0].Score, matches[1].Score)
	}
}

func TestMatchQuoteEmptyTranscriptYieldsNothing(t *testing.T) {
	ctx := context.Background()
	repo, _ := newTestRepository(t)
	if _, err := repo.Install(ctx, quoteFixture()); err != nil {
		t.Fatalf("install: %v", err)
	}
	matches, err := repo.MatchQuote(ctx, "kjv", "the and of but to in is are", 10)
	if err != nil {
		t.Fatalf("match quote: %v", err)
	}
	if len(matches) != 0 {
		t.Fatalf("matches = %v, want none for an all-grammar transcript", matches)
	}
}

func TestMatchQuoteUnknownTranslation(t *testing.T) {
	ctx := context.Background()
	repo, _ := newTestRepository(t)
	if _, err := repo.MatchQuote(ctx, "nope", "peace of god", 10); err == nil {
		t.Fatal("want an error for a translation that is not installed")
	}
}
