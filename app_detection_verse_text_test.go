package main

import (
	"context"
	"strings"
	"testing"

	"zopresent/internal/bible"
	"zopresent/internal/settings"
)

// The same verse in two translations, so a card's text proves which one it was
// read from rather than merely that some text was returned.
const (
	kjvPhilippians46 = "Be careful for nothing; but in every thing by prayer and supplication with thanksgiving let your requests be made known unto God."
	nivPhilippians46 = "Do not be anxious about anything, but in every situation, by prayer and petition, with thanksgiving, present your requests to God."
)

func installVerseTextBibles(t *testing.T, app *App) {
	t.Helper()

	install := func(id, name, abbreviation, text string) {
		t.Helper()
		_, err := app.bible.Install(context.Background(), bible.TranslationData{
			TranslationMeta: bible.TranslationMeta{
				ID: id, Name: name, Abbreviation: abbreviation, Language: "en", Version: "1.0.0",
			},
			Verses: []bible.Verse{{BookID: 50, Chapter: 4, Number: 6, Text: text}},
		})
		if err != nil {
			t.Fatalf("install %s: %v", id, err)
		}
	}
	install("kjv", "King James Version", "KJV", kjvPhilippians46)
	install("niv", "New International Version", "NIV", nivPhilippians46)
}

func setDefaultTranslation(t *testing.T, app *App, id string) {
	t.Helper()
	if err := app.settings.Set(context.Background(), settings.KeyDefaultTranslation, id); err != nil {
		t.Fatalf("set default translation %q: %v", id, err)
	}
}

// The PRD's detection panel requires the exact retrieved verse text and the
// translation beside every candidate. A card is read from the operator's
// selected translation, so switching the selection changes the text the card
// shows without changing the card itself.
func TestDetectionCardsCarryTheSelectedTranslationText(t *testing.T) {
	app := newTestApp(t)
	installVerseTextBibles(t, app)
	setDefaultTranslation(t, app, "niv")

	result := runSegment(t, app, "Turn with me to Philippians chapter four verse six")
	if len(result.Candidates) != 1 {
		t.Fatalf("candidates = %d, want the one reference that was spoken", len(result.Candidates))
	}
	card := result.Candidates[0]
	if card.Reference != "Philippians 4:6" {
		t.Fatalf("reference = %q, want Philippians 4:6", card.Reference)
	}
	if card.Translation != "NIV" {
		t.Errorf("translation = %q, want the selected NIV", card.Translation)
	}
	if card.VerseText != nivPhilippians46 {
		t.Errorf("verse text = %q, want the NIV wording %q", card.VerseText, nivPhilippians46)
	}
	if strings.Contains(card.VerseText, "Be careful") {
		t.Error("the card shows the KJV wording while NIV is selected")
	}

	// The same pending card, read again after the operator switches translation.
	setDefaultTranslation(t, app, "kjv")
	views, err := app.ListDetections()
	if err != nil {
		t.Fatalf("list detections: %v", err)
	}
	if len(views) != 1 {
		t.Fatalf("views = %d, want the single pending card", len(views))
	}
	if views[0].Translation != "KJV" {
		t.Errorf("translation = %q, want KJV after the switch", views[0].Translation)
	}
	if views[0].VerseText != kjvPhilippians46 {
		t.Errorf("verse text = %q, want the KJV wording after the switch", views[0].VerseText)
	}
}

// A passage the selected translation does not hold must leave the card without
// text: the panel may show nothing, but it must never invent or substitute
// another translation's verse.
func TestDetectionCardWithoutPassageCarriesNoText(t *testing.T) {
	app := newTestApp(t)
	installVerseTextBibles(t, app)
	setDefaultTranslation(t, app, "niv")

	result := runSegment(t, app, "Turn with me to Philippians chapter four verse ninety nine")
	if len(result.Candidates) != 1 {
		t.Fatalf("candidates = %d, want the unreadable passage queued for the operator", len(result.Candidates))
	}
	card := result.Candidates[0]
	if card.VerseText != "" {
		t.Errorf("verse text = %q, want empty for a passage the translation does not hold", card.VerseText)
	}
}
