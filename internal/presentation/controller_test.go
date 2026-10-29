package presentation

import (
	"context"
	"errors"
	"testing"

	"zowe-worship/internal/bible"
	"zowe-worship/internal/serviceorder"
	"zowe-worship/internal/songs"
)

type fakeScripture struct {
	verses      []bible.SearchResult
	translation bible.Translation
	err         error
}

func (f *fakeScripture) Passage(context.Context, string, int, int, int, int) ([]bible.SearchResult, error) {
	return f.verses, f.err
}

func (f *fakeScripture) Translation(context.Context, string) (bible.Translation, error) {
	return f.translation, nil
}

type fakeSongs struct {
	song    songs.Song
	section songs.Section
	err     error
}

func (f *fakeSongs) Section(context.Context, string, string) (songs.Song, songs.Section, error) {
	return f.song, f.section, f.err
}

type fakeSchedule struct {
	service    serviceorder.Service
	items      []serviceorder.Item
	currentErr error
}

func (f *fakeSchedule) Current(context.Context) (serviceorder.Service, []serviceorder.Item, error) {
	if f.currentErr != nil {
		return serviceorder.Service{}, nil, f.currentErr
	}
	return f.service, f.items, nil
}

func (f *fakeSchedule) Item(_ context.Context, itemID string) (serviceorder.Item, error) {
	for _, item := range f.items {
		if item.ID == itemID {
			return item, nil
		}
	}
	return serviceorder.Item{}, serviceorder.ErrItemNotFound
}

func (f *fakeSchedule) Items(context.Context, string) ([]serviceorder.Item, error) {
	return f.items, nil
}

func johnVerse() bible.SearchResult {
	return bible.SearchResult{
		Book:    bible.Book{ID: 43, Name: "John", Abbreviation: "John", Testament: bible.NewTestament},
		Chapter: 3,
		Verse:   16,
		Text:    "For God so loved the world.",
	}
}

func newController() (*Controller, *fakeScripture) {
	scripture := &fakeScripture{
		verses:      []bible.SearchResult{johnVerse()},
		translation: bible.Translation{TranslationMeta: bible.TranslationMeta{ID: "kjv", Abbreviation: "KJV"}},
	}
	return New(scripture, &fakeSchedule{}, nil), scripture
}

func TestPreviewThenSendLiveKeepsLiveFrozen(t *testing.T) {
	ctx := context.Background()
	controller, scripture := newController()

	state, err := controller.PreviewScripture(ctx, "kjv", 43, 3, 16, 16)
	if err != nil {
		t.Fatalf("preview: %v", err)
	}
	if state.Preview == nil || state.Live != nil {
		t.Fatalf("state after preview = %+v", state)
	}
	if state.Preview.Label != "John 3:16" || state.Preview.Translation != "KJV" {
		t.Fatalf("preview = %+v", state.Preview)
	}

	live, err := controller.ProjectPreview()
	if err != nil {
		t.Fatalf("project: %v", err)
	}
	if live.Live == nil || live.Live.Lines[0] != "For God so loved the world." {
		t.Fatalf("live = %+v", live.Live)
	}

	scripture.verses = []bible.SearchResult{{
		Book: johnVerse().Book, Chapter: 3, Verse: 16, Text: "A different rendering.",
	}}
	updated, err := controller.PreviewScripture(ctx, "kjv", 43, 3, 16, 16)
	if err != nil {
		t.Fatalf("second preview: %v", err)
	}
	if updated.Preview.Lines[0] != "A different rendering." {
		t.Fatalf("preview not updated: %+v", updated.Preview)
	}
	if updated.Live.Lines[0] != "For God so loved the world." {
		t.Fatalf("live changed with preview: %+v", updated.Live)
	}

	cleared := controller.ClearLive()
	if cleared.Live != nil || cleared.Preview == nil {
		t.Fatalf("state after clear = %+v", cleared)
	}
}

func TestProjectWithoutPreviewFails(t *testing.T) {
	controller, _ := newController()
	if _, err := controller.ProjectPreview(); !errors.Is(err, ErrNoPreview) {
		t.Fatalf("err = %v, want ErrNoPreview", err)
	}
}

func TestEmptyPassageIsRejected(t *testing.T) {
	ctx := context.Background()
	controller, scripture := newController()
	scripture.verses = nil

	if _, err := controller.PreviewScripture(ctx, "kjv", 43, 3, 16, 16); !errors.Is(err, ErrEmptyPassage) {
		t.Fatalf("err = %v, want ErrEmptyPassage", err)
	}
	if controller.State().Preview != nil {
		t.Fatal("failed preview changed state")
	}
}

func TestSongSectionBecomesLyricSlide(t *testing.T) {
	ctx := context.Background()
	reader := &fakeSongs{
		song:    songs.Song{ID: "song-1", Title: "Amazing Grace"},
		section: songs.Section{ID: "section-1", SongID: "song-1", Label: "Verse 1", Text: "Amazing grace! how sweet the sound,\nThat saved a wretch like me!"},
	}
	controller := New(&fakeScripture{}, &fakeSchedule{}, reader)

	state, err := controller.PreviewSongSection(ctx, "song-1", "section-1")
	if err != nil {
		t.Fatalf("preview song: %v", err)
	}
	if state.Preview.Kind != "song" || state.Preview.Label != "Amazing Grace · Verse 1" {
		t.Fatalf("preview = %+v", state.Preview)
	}
	if len(state.Preview.Lines) != 2 || state.Preview.Reference != "" {
		t.Fatalf("lyric lines = %+v", state.Preview)
	}

	live, err := controller.ProjectSongSection(ctx, "song-1", "section-1")
	if err != nil {
		t.Fatalf("project song: %v", err)
	}
	if live.Live.Label != "Amazing Grace · Verse 1" {
		t.Fatalf("live = %+v", live.Live)
	}
}

func TestScheduleNavigationMixesScriptureAndSongs(t *testing.T) {
	ctx := context.Background()
	schedule := &fakeSchedule{
		service: serviceorder.Service{ID: "svc", Name: "Sunday Service"},
		items: []serviceorder.Item{
			{ID: "one", ServiceID: "svc", Position: 1, Kind: serviceorder.KindScripture, Title: "John 3:16", TranslationID: "kjv", BookID: 43, Chapter: 3, VerseStart: 16, VerseEnd: 16},
			{ID: "two", ServiceID: "svc", Position: 2, Kind: serviceorder.KindSong, Title: "Amazing Grace · Verse 1", SongID: "song-1", SectionID: "section-1"},
		},
	}
	controller := New(&fakeScripture{
		verses:      []bible.SearchResult{johnVerse()},
		translation: bible.Translation{TranslationMeta: bible.TranslationMeta{ID: "kjv", Abbreviation: "KJV"}},
	}, schedule, &fakeSongs{
		song:    songs.Song{ID: "song-1", Title: "Amazing Grace"},
		section: songs.Section{ID: "section-1", SongID: "song-1", Label: "Verse 1", Text: "Amazing grace"},
	})

	state, err := controller.PreviewItem(ctx, "one")
	if err != nil {
		t.Fatalf("preview scripture item: %v", err)
	}
	if state.Preview.Kind != "scripture" || state.Preview.ItemID != "one" {
		t.Fatalf("preview = %+v", state.Preview)
	}

	next, err := controller.Next(ctx)
	if err != nil {
		t.Fatalf("next: %v", err)
	}
	if next.Preview.Kind != "song" || next.Preview.ItemID != "two" {
		t.Fatalf("after next = %+v", next.Preview)
	}
	if next.Live != nil {
		t.Fatalf("next changed live output: %+v", next.Live)
	}

	previous, err := controller.Previous(ctx)
	if err != nil {
		t.Fatalf("previous: %v", err)
	}
	if previous.Preview.ItemID != "one" {
		t.Fatalf("after previous = %+v", previous.Preview)
	}
}

func TestNextWithoutScheduleReports(t *testing.T) {
	ctx := context.Background()
	controller := New(&fakeScripture{
		verses:      []bible.SearchResult{johnVerse()},
		translation: bible.Translation{TranslationMeta: bible.TranslationMeta{ID: "kjv", Abbreviation: "KJV"}},
	}, &fakeSchedule{currentErr: serviceorder.ErrNoService}, nil)

	if _, err := controller.Next(ctx); !errors.Is(err, ErrNoSchedule) {
		t.Fatalf("err = %v, want ErrNoSchedule", err)
	}
}
