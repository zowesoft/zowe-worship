package presentation

import (
	"context"
	"errors"
	"testing"

	"zowe-worship/internal/bible"
	"zowe-worship/internal/serviceorder"
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

func johnVerse(text string) bible.SearchResult {
	return bible.SearchResult{
		Book:    bible.Book{ID: 43, Name: "John", Abbreviation: "John", Testament: bible.NewTestament},
		Chapter: 3,
		Verse:   16,
		Text:    text,
	}
}

func newController() (*Controller, *fakeScripture) {
	scripture := &fakeScripture{
		verses:      []bible.SearchResult{johnVerse("For God so loved the world.")},
		translation: bible.Translation{TranslationMeta: bible.TranslationMeta{ID: "kjv", Abbreviation: "KJV"}},
	}
	return New(scripture, &fakeSchedule{}), scripture
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
	if state.Preview.Reference != "John 3:16" || state.Preview.Translation != "KJV" {
		t.Fatalf("preview = %+v", state.Preview)
	}

	live, err := controller.ProjectPreview()
	if err != nil {
		t.Fatalf("project: %v", err)
	}
	if live.Live == nil || live.Live.Verses[0].Text != "For God so loved the world." {
		t.Fatalf("live = %+v", live.Live)
	}

	scripture.verses = []bible.SearchResult{johnVerse("A different verse now.")}
	updated, err := controller.PreviewScripture(ctx, "kjv", 43, 3, 16, 16)
	if err != nil {
		t.Fatalf("second preview: %v", err)
	}
	if updated.Preview.Verses[0].Text != "A different verse now." {
		t.Fatalf("preview not updated: %+v", updated.Preview)
	}
	if updated.Live.Verses[0].Text != "For God so loved the world." {
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

func TestScheduleNavigationMovesPreviewOnly(t *testing.T) {
	ctx := context.Background()
	schedule := &fakeSchedule{
		service: serviceorder.Service{ID: "svc", Name: "Sunday Service"},
		items: []serviceorder.Item{
			{ID: "one", ServiceID: "svc", Position: 1, Kind: serviceorder.KindScripture, Title: "John 3:16", TranslationID: "kjv", BookID: 43, Chapter: 3, VerseStart: 16, VerseEnd: 16},
			{ID: "two", ServiceID: "svc", Position: 2, Kind: serviceorder.KindScripture, Title: "John 4:24", TranslationID: "kjv", BookID: 43, Chapter: 4, VerseStart: 24, VerseEnd: 24},
		},
	}
	controller := New(&fakeScripture{
		verses:      []bible.SearchResult{johnVerse("verse text")},
		translation: bible.Translation{TranslationMeta: bible.TranslationMeta{ID: "kjv", Abbreviation: "KJV"}},
	}, schedule)

	state, err := controller.PreviewItem(ctx, "one")
	if err != nil {
		t.Fatalf("preview item: %v", err)
	}
	if state.Preview.ItemID != "one" || state.Preview.Reference != "John 3:16" {
		t.Fatalf("preview = %+v", state.Preview)
	}

	next, err := controller.Next(ctx)
	if err != nil {
		t.Fatalf("next: %v", err)
	}
	if next.Preview.ItemID != "two" || next.Live != nil {
		t.Fatalf("after next = %+v", next)
	}

	first, err := controller.Previous(ctx)
	if err != nil {
		t.Fatalf("previous: %v", err)
	}
	if first.Preview.ItemID != "one" {
		t.Fatalf("after previous = %+v", first.Preview)
	}

	clamped, err := controller.Previous(ctx)
	if err != nil {
		t.Fatalf("previous at start: %v", err)
	}
	if clamped.Preview.ItemID != "one" {
		t.Fatalf("previous did not clamp: %+v", clamped.Preview)
	}
}

func TestNextWithoutScheduleReports(t *testing.T) {
	ctx := context.Background()
	controller := New(&fakeScripture{
		verses:      []bible.SearchResult{johnVerse("verse text")},
		translation: bible.Translation{TranslationMeta: bible.TranslationMeta{ID: "kjv", Abbreviation: "KJV"}},
	}, &fakeSchedule{currentErr: serviceorder.ErrNoService})

	if _, err := controller.Next(ctx); !errors.Is(err, ErrNoSchedule) {
		t.Fatalf("err = %v, want ErrNoSchedule", err)
	}
}
