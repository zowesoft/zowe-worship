package presentation

import (
	"context"
	"testing"

	"zopresent/internal/bible"
)

func philippianVerse(verse int, text string) bible.SearchResult {
	return bible.SearchResult{
		Book:    bible.Book{ID: 57, Name: "Philippians", Abbreviation: "Phil", Testament: bible.NewTestament},
		Chapter: 4,
		Verse:   verse,
		Text:    text,
	}
}

// Approving a detected range must honor the one-verse-per-slide setting: the
// first preview shows only the first verse, and the rest arrives by stepping.
func TestPreviewScriptureOneVerseShowsFirstVerseOnly(t *testing.T) {
	ctx := context.Background()
	scripture := &fakeScripture{
		verses: []bible.SearchResult{
			philippianVerse(6, "Do not be anxious about anything."),
			philippianVerse(7, "And the peace of God, which transcends all understanding."),
		},
		translation: bible.Translation{TranslationMeta: bible.TranslationMeta{ID: "kjv", Abbreviation: "KJV"}},
	}
	controller := New(scripture, &fakeSchedule{}, nil, nil, nil)

	state, err := controller.PreviewScripture(ctx, "kjv", 57, 4, 6, 7, true)
	if err != nil {
		t.Fatalf("preview: %v", err)
	}
	if scripture.lastStart != 6 || scripture.lastEnd != 6 {
		t.Fatalf("passage request = %d..%d, want 6..6", scripture.lastStart, scripture.lastEnd)
	}
	if state.Preview == nil {
		t.Fatalf("preview missing")
	}
	for _, line := range state.Preview.Lines {
		if line == "And the peace of God, which transcends all understanding." {
			t.Fatalf("verse 7 leaked into a one-verse preview: %+v", state.Preview.Lines)
		}
	}
}

func TestPreviewScriptureWholePassageKeepsRange(t *testing.T) {
	ctx := context.Background()
	scripture := &fakeScripture{
		verses: []bible.SearchResult{
			philippianVerse(6, "Do not be anxious about anything."),
			philippianVerse(7, "And the peace of God, which transcends all understanding."),
		},
		translation: bible.Translation{TranslationMeta: bible.TranslationMeta{ID: "kjv", Abbreviation: "KJV"}},
	}
	controller := New(scripture, &fakeSchedule{}, nil, nil, nil)

	if _, err := controller.PreviewScripture(ctx, "kjv", 57, 4, 6, 7, false); err != nil {
		t.Fatalf("preview: %v", err)
	}
	if scripture.lastStart != 6 || scripture.lastEnd != 7 {
		t.Fatalf("passage request = %d..%d, want 6..7", scripture.lastStart, scripture.lastEnd)
	}
}
