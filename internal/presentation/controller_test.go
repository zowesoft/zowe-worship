package presentation

import (
	"context"
	"errors"
	"testing"

	"zowe-worship/internal/bible"
	"zowe-worship/internal/media"
	"zowe-worship/internal/serviceorder"
	"zowe-worship/internal/songs"
	"zowe-worship/internal/themes"
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

type fakeMedia struct {
	kind  string
	title string
	url   string
	err   error
}

func (f *fakeMedia) Slide(context.Context, string) (media.Slide, error) {
	if f.err != nil {
		return media.Slide{}, f.err
	}
	return media.Slide{Kind: f.kind, Title: f.title, URL: f.url, Fit: media.FitContain}, nil
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

type fakeThemes struct {
	defaults map[themes.Kind]themes.Theme
	byID     map[string]themes.Theme
}

func (f *fakeThemes) Get(_ context.Context, id string) (themes.Theme, error) {
	theme, ok := f.byID[id]
	if !ok {
		return themes.Theme{}, themes.ErrThemeNotFound
	}
	return theme, nil
}

func (f *fakeThemes) Default(_ context.Context, kind themes.Kind) (themes.Theme, error) {
	theme, ok := f.defaults[kind]
	if !ok {
		return themes.Theme{}, themes.ErrThemeNotFound
	}
	return theme, nil
}

// bright is one distinct theme per kind, so a test can tell which theme a slide
// actually picked up.
func bright() *fakeThemes {
	scripture := themes.DefaultTheme(themes.KindScripture)
	scripture.ID, scripture.Name, scripture.Style.Color = "theme-scripture", "Bright reading", "#FFE100"
	lyrics := themes.DefaultTheme(themes.KindLyrics)
	lyrics.ID, lyrics.Name, lyrics.Style.Color = "theme-lyrics", "Bright lyrics", "#00FF88"
	return &fakeThemes{
		defaults: map[themes.Kind]themes.Theme{themes.KindScripture: scripture, themes.KindLyrics: lyrics},
		byID:     map[string]themes.Theme{"theme-scripture": scripture, "theme-lyrics": lyrics},
	}
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
	return New(scripture, &fakeSchedule{}, nil, nil, nil), scripture
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
	controller := New(&fakeScripture{}, &fakeSchedule{}, reader, nil, bright())

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
	}, nil, bright())

	state, err := controller.PreviewItem(ctx, "one")
	if err != nil {
		t.Fatalf("preview scripture item: %v", err)
	}
	if state.Preview.Kind != "scripture" || state.Preview.ItemID != "one" {
		t.Fatalf("preview = %+v", state.Preview)
	}
	// Each kind picks up its own default, so navigating the schedule cannot put
	// the lyric theme on a Bible slide.
	if state.Preview.Theme.ID != "theme-scripture" {
		t.Fatalf("scripture theme = %+v", state.Preview.Theme)
	}

	next, err := controller.Next(ctx)
	if err != nil {
		t.Fatalf("next: %v", err)
	}
	if next.Preview.Kind != "song" || next.Preview.ItemID != "two" {
		t.Fatalf("after next = %+v", next.Preview)
	}
	if next.Preview.Theme.ID != "theme-lyrics" {
		t.Fatalf("lyric theme = %+v", next.Preview.Theme)
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
	}, &fakeSchedule{currentErr: serviceorder.ErrNoService}, nil, nil, nil)

	if _, err := controller.Next(ctx); !errors.Is(err, ErrNoSchedule) {
		t.Fatalf("err = %v, want ErrNoSchedule", err)
	}
}

func TestMediaSlidesNeverBlankLiveOutput(t *testing.T) {
	ctx := context.Background()
	media := &fakeMedia{kind: "image", title: "Upper room", url: "/media/file/abc123"}
	controller := New(&fakeScripture{}, &fakeSchedule{}, nil, media, nil)

	state, err := controller.ProjectMedia(ctx, "abc123")
	if err != nil {
		t.Fatalf("project media: %v", err)
	}
	if state.Live == nil || state.Live.Kind != "image" || state.Live.Source != "/media/file/abc123" {
		t.Fatalf("live = %+v", state.Live)
	}
	if state.Live.Label != "Upper room" || len(state.Live.Lines) != 0 {
		t.Fatalf("live slide = %+v", state.Live)
	}

	media.err = errors.New("the library copy of this asset is missing")
	if _, err := controller.ProjectMedia(ctx, "gone"); err == nil {
		t.Fatal("projecting a missing asset succeeded")
	}

	after := controller.State()
	if after.Live == nil || after.Live.Source != "/media/file/abc123" {
		t.Fatalf("a failed projection changed live output: %+v", after.Live)
	}

	before := controller.State()
	if _, err := controller.PreviewMedia(ctx, "gone"); err == nil {
		t.Fatal("previewing a missing asset succeeded")
	}
	unchanged := controller.State()
	if unchanged.Preview.Source != before.Preview.Source || unchanged.Live.Source != before.Live.Source {
		t.Fatalf("a failed preview moved state: %+v -> %+v", before, unchanged)
	}
}

// The viewer picks a player from the kind, so a clip and a song recording have
// to arrive labelled rather than as a picture that will never load.
func TestVideoAndAudioKeepTheirKind(t *testing.T) {
	ctx := context.Background()
	controller := New(&fakeScripture{}, &fakeSchedule{}, nil,
		&fakeMedia{kind: "video", title: "Creed", url: "/media/file/clip"}, nil)

	state, err := controller.ProjectMedia(ctx, "clip")
	if err != nil {
		t.Fatalf("project video: %v", err)
	}
	if state.Live == nil || state.Live.Kind != "video" || state.Live.Source != "/media/file/clip" {
		t.Fatalf("live video = %+v", state.Live)
	}

	audio := New(&fakeScripture{}, &fakeSchedule{}, nil,
		&fakeMedia{kind: "audio", title: "Anthem", url: "/media/file/anthem"}, nil)
	state, err = audio.PreviewMedia(ctx, "anthem")
	if err != nil {
		t.Fatalf("preview audio: %v", err)
	}
	if state.Preview == nil || state.Preview.Kind != "audio" {
		t.Fatalf("preview audio = %+v", state.Preview)
	}
	if state.Live != nil {
		t.Fatalf("a preview went live: %+v", state.Live)
	}
}

func TestScheduleItemDispatchesToMedia(t *testing.T) {
	ctx := context.Background()
	schedule := &fakeSchedule{
		service: serviceorder.Service{ID: "svc"},
		items: []serviceorder.Item{
			{ID: "img", ServiceID: "svc", Position: 1, Kind: serviceorder.KindMedia, Title: "Cross", MediaID: "media-9"},
		},
	}
	controller := New(&fakeScripture{}, schedule, nil, &fakeMedia{kind: "image", title: "Cross", url: "/media/file/media-9"}, nil)

	state, err := controller.PreviewItem(ctx, "img")
	if err != nil {
		t.Fatalf("preview media item: %v", err)
	}
	if state.Preview == nil || state.Preview.Kind != "image" || state.Preview.ItemID != "img" {
		t.Fatalf("preview = %+v", state.Preview)
	}
	if state.Preview.Source != "/media/file/media-9" {
		t.Fatalf("preview source = %s", state.Preview.Source)
	}
	// A picture or clip fills the screen itself, so it carries no theme.
	if state.Preview.Theme != (themes.Theme{}) {
		t.Fatalf("media slide picked up a theme: %+v", state.Preview.Theme)
	}
}

func scheduleWith(override string) *fakeSchedule {
	return &fakeSchedule{
		service: serviceorder.Service{ID: "svc"},
		items: []serviceorder.Item{{
			ID: "one", ServiceID: "svc", Position: 1, Kind: serviceorder.KindScripture,
			Title: "John 3:16", TranslationID: "kjv", BookID: 43, Chapter: 3,
			VerseStart: 16, VerseEnd: 16, ThemeID: override,
		}},
	}
}

func themedController(reader ThemeReader, override string) *Controller {
	return New(&fakeScripture{
		verses:      []bible.SearchResult{johnVerse()},
		translation: bible.Translation{TranslationMeta: bible.TranslationMeta{ID: "kjv", Abbreviation: "KJV"}},
	}, scheduleWith(override), nil, nil, reader)
}

func TestItemThemeOverrideBeatsTheKindDefault(t *testing.T) {
	ctx := context.Background()
	reader := bright()
	quiet := themes.DefaultTheme(themes.KindScripture)
	quiet.ID, quiet.Name, quiet.Style.FontSize = "theme-quiet", "Quiet reading", 3.2
	reader.byID["theme-quiet"] = quiet

	state, err := themedController(reader, "theme-quiet").PreviewItem(ctx, "one")
	if err != nil {
		t.Fatalf("preview: %v", err)
	}
	if state.Preview.Theme.ID != "theme-quiet" {
		t.Fatalf("override ignored, got %+v", state.Preview.Theme)
	}
}

// A theme that was deleted, or one borrowed from another kind, must not decide
// how a slide looks. The operator chose it once; the room still needs to read.
func TestUnusableThemeOverridesFallBackToTheDefault(t *testing.T) {
	ctx := context.Background()
	reader := bright()

	missing, err := themedController(reader, "theme-gone").PreviewItem(ctx, "one")
	if err != nil {
		t.Fatalf("preview with a deleted theme: %v", err)
	}
	if missing.Preview.Theme.ID != "theme-scripture" {
		t.Fatalf("deleted override gave %+v", missing.Preview.Theme)
	}

	wrongKind, err := themedController(reader, "theme-lyrics").PreviewItem(ctx, "one")
	if err != nil {
		t.Fatalf("preview with a lyric theme on a bible slide: %v", err)
	}
	if wrongKind.Preview.Theme.ID != "theme-scripture" {
		t.Fatalf("lyric theme leaked onto a scripture slide: %+v", wrongKind.Preview.Theme)
	}
}

// A fresh install has no themes at all, and the first slide still has to be
// readable, so the app's own defaults are the last resort.
func TestNoThemesStillDrawsReadableSlides(t *testing.T) {
	ctx := context.Background()
	state, err := themedController(nil, "").PreviewItem(ctx, "one")
	if err != nil {
		t.Fatalf("preview: %v", err)
	}
	expected := themes.DefaultTheme(themes.KindScripture)
	if state.Preview.Theme.Style != expected.Style {
		t.Fatalf("theme = %+v, want %+v", state.Preview.Theme.Style, expected.Style)
	}
	if state.Preview.Theme.Reference != expected.Reference {
		t.Fatalf("reference = %+v, want %+v", state.Preview.Theme.Reference, expected.Reference)
	}
}

func TestLiveKeepsTheThemeItWasProjectedWith(t *testing.T) {
	ctx := context.Background()
	reader := bright()
	controller := themedController(reader, "")

	first, err := controller.ProjectScripture(ctx, "kjv", 43, 3, 16, 16)
	if err != nil {
		t.Fatalf("project: %v", err)
	}
	if first.Live.Theme.ID != "theme-scripture" {
		t.Fatalf("live theme = %+v", first.Live.Theme)
	}

	// Making another theme the default must not restyle the slide already on the
	// wall; the operator changes that by projecting the next item.
	evening := themes.DefaultTheme(themes.KindScripture)
	evening.ID, evening.Name, evening.Style.Color = "theme-evening", "Evening", "#123456"
	reader.byID["theme-evening"] = evening
	reader.defaults[themes.KindScripture] = evening

	updated, err := controller.PreviewScripture(ctx, "kjv", 43, 3, 16, 16)
	if err != nil {
		t.Fatalf("preview after the default changed: %v", err)
	}
	if updated.Preview.Theme.ID != "theme-evening" {
		t.Fatalf("preview did not take the new default: %+v", updated.Preview.Theme)
	}
	if updated.Live.Theme.ID != "theme-scripture" {
		t.Fatalf("live output restyled itself mid-service: %+v", updated.Live.Theme)
	}
}

func TestCustomSlideKinds(t *testing.T) {
	ctx := context.Background()
	schedule := &fakeSchedule{
		service: serviceorder.Service{ID: "svc"},
		items: []serviceorder.Item{
			{ID: "txt", ServiceID: "svc", Position: 1, Kind: serviceorder.KindText, Title: "Welcome", Content: "Line 1\nLine 2"},
			{ID: "ann", ServiceID: "svc", Position: 2, Kind: serviceorder.KindAnnouncement, Title: "Meeting", Content: "6 PM in the Hall"},
			{ID: "blk", ServiceID: "svc", Position: 3, Kind: serviceorder.KindBlank, Title: "Pause"},
		},
	}
	controller := New(&fakeScripture{}, schedule, nil, nil, bright())

	// Text slide
	state, err := controller.PreviewItem(ctx, "txt")
	if err != nil {
		t.Fatalf("preview text: %v", err)
	}
	if state.Preview.Kind != "text" || len(state.Preview.Lines) != 2 || state.Preview.Lines[0] != "Line 1" {
		t.Fatalf("unexpected text slide: %+v", state.Preview)
	}

	// Announcement slide
	state, err = controller.PreviewItem(ctx, "ann")
	if err != nil {
		t.Fatalf("preview announcement: %v", err)
	}
	if state.Preview.Kind != "announcement" || len(state.Preview.Lines) != 1 || state.Preview.Label != "Meeting" {
		t.Fatalf("unexpected announcement slide: %+v", state.Preview)
	}

	// Blank slide
	state, err = controller.PreviewItem(ctx, "blk")
	if err != nil {
		t.Fatalf("preview blank: %v", err)
	}
	if state.Preview.Kind != "blank" || len(state.Preview.Lines) != 0 {
		t.Fatalf("unexpected blank slide: %+v", state.Preview)
	}

	// Send live
	liveState, err := controller.ProjectPreview()
	if err != nil {
		t.Fatalf("send live blank: %v", err)
	}
	if liveState.Live == nil || liveState.Live.Kind != "blank" {
		t.Fatalf("live blank failed: %+v", liveState.Live)
	}
}
