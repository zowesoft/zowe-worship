package presentation

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"sync"

	"zowe-worship/internal/bible"
	"zowe-worship/internal/media"
	"zowe-worship/internal/serviceorder"
	"zowe-worship/internal/songs"
	"zowe-worship/internal/themes"
)

var (
	ErrNoPreview    = errors.New("no preview content")
	ErrNoSchedule   = errors.New("no service schedule")
	ErrEmptyPassage = errors.New("passage has no verses")
)

// Content is one slide. It is a frozen snapshot, so live output survives a
// translation update or a song edit until the operator projects something else.
type Content struct {
	ItemID      string   `json:"itemId,omitempty"`
	Kind        string   `json:"kind"`
	Title       string   `json:"title"`
	Subtitle    string   `json:"subtitle,omitempty"`
	Label       string   `json:"label"`
	Reference   string   `json:"reference,omitempty"`
	Translation string   `json:"translation,omitempty"`
	Source      string   `json:"source,omitempty"`
	Aspect      string   `json:"aspect,omitempty"`
	Fit         string   `json:"fit,omitempty"`
	Ratio       float64  `json:"ratio"`
	Lines       []string `json:"lines"`
	// Theme is the look frozen with this slide. Media slides fill the screen
	// themselves, so they carry no theme and a zero value says so.
	Theme themes.Theme `json:"theme"`
}

type State struct {
	Preview *Content `json:"preview,omitempty"`
	Live    *Content `json:"live,omitempty"`
}

type ScriptureReader interface {
	Passage(ctx context.Context, translationID string, bookID, chapter, startVerse, endVerse int) ([]bible.SearchResult, error)
	Translation(ctx context.Context, id string) (bible.Translation, error)
}

type SongReader interface {
	Section(ctx context.Context, songID, sectionID string) (songs.Song, songs.Section, error)
}

// MediaReader resolves a library asset into a slide. It reports a missing file
// as an error rather than a blank slide, so live output keeps what it was
// showing.
type MediaReader interface {
	Slide(ctx context.Context, mediaID string) (media.Slide, error)
}

type ScheduleReader interface {
	Current(ctx context.Context) (serviceorder.Service, []serviceorder.Item, error)
	Item(ctx context.Context, itemID string) (serviceorder.Item, error)
	Items(ctx context.Context, serviceID string) ([]serviceorder.Item, error)
}

// ThemeReader supplies the look a slide is drawn with: the theme an item asks
// for, or the default for its kind.
type ThemeReader interface {
	Get(ctx context.Context, id string) (themes.Theme, error)
	Default(ctx context.Context, kind themes.Kind) (themes.Theme, error)
}

// previewSource remembers what the previewed slide was built from, so a theme
// edit can rebuild the same slide rather than leave the operator looking at the
// old colours until they happen to click the item again.
type previewSource struct {
	itemID        string
	translationID string
	bookID        int
	chapter       int
	verseStart    int
	verseEnd      int
	songID        string
	sectionID     string
	mediaID       string
}

// Controller owns preview and live output. Recognition and UI code never write
// live state directly; only the explicit projection commands here do.
type Controller struct {
	mu        sync.Mutex
	scripture ScriptureReader
	song      SongReader
	media     MediaReader
	schedule  ScheduleReader
	themes    ThemeReader

	preview *Content
	live    *Content
	source  previewSource
}

func New(
	scripture ScriptureReader, schedule ScheduleReader, song SongReader, media MediaReader, themeReader ThemeReader,
) *Controller {
	return &Controller{
		scripture: scripture,
		schedule:  schedule,
		song:      song,
		media:     media,
		themes:    themeReader,
	}
}

func (c *Controller) State() State {
	c.mu.Lock()
	defer c.mu.Unlock()
	return c.stateLocked()
}

func (c *Controller) stateLocked() State {
	return State{Preview: clone(c.preview), Live: clone(c.live)}
}

func (c *Controller) previewScripture(
	ctx context.Context, translationID string, bookID, chapter, startVerse, endVerse int,
) (State, error) {
	content, err := c.scriptureContent(ctx, "", "", translationID, bookID, chapter, startVerse, endVerse)
	if err != nil {
		return c.stateLocked(), err
	}
	c.preview = content
	c.source = previewSource{
		translationID: translationID, bookID: bookID, chapter: chapter,
		verseStart: startVerse, verseEnd: endVerse,
	}
	return c.stateLocked(), nil
}

func (c *Controller) previewSongSection(ctx context.Context, songID, sectionID string) (State, error) {
	content, err := c.songContent(ctx, "", "", songID, sectionID)
	if err != nil {
		return c.stateLocked(), err
	}
	c.preview = content
	c.source = previewSource{songID: songID, sectionID: sectionID}
	return c.stateLocked(), nil
}

func (c *Controller) previewMedia(ctx context.Context, mediaID string) (State, error) {
	content, err := c.mediaContent(ctx, "", mediaID)
	if err != nil {
		return c.stateLocked(), err
	}
	c.preview = content
	c.source = previewSource{mediaID: mediaID}
	return c.stateLocked(), nil
}

func (c *Controller) previewItem(ctx context.Context, itemID string) (State, error) {
	content, err := c.itemContent(ctx, itemID)
	if err != nil {
		return c.stateLocked(), err
	}
	c.preview = content
	return c.stateLocked(), nil
}

// sendLive promotes whatever the preview helper just built. A failed preview
// leaves live output alone, so a missing file never blanks the room.
func (c *Controller) sendLive(state State, err error) (State, error) {
	if err != nil || c.preview == nil {
		return state, err
	}
	c.live = clone(c.preview)
	return c.stateLocked(), nil
}

func (c *Controller) PreviewScripture(
	ctx context.Context, translationID string, bookID, chapter, startVerse, endVerse int,
) (State, error) {
	c.mu.Lock()
	defer c.mu.Unlock()
	return c.previewScripture(ctx, translationID, bookID, chapter, startVerse, endVerse)
}

func (c *Controller) PreviewSongSection(ctx context.Context, songID, sectionID string) (State, error) {
	c.mu.Lock()
	defer c.mu.Unlock()
	return c.previewSongSection(ctx, songID, sectionID)
}

func (c *Controller) PreviewItem(ctx context.Context, itemID string) (State, error) {
	c.mu.Lock()
	defer c.mu.Unlock()
	return c.previewItem(ctx, itemID)
}

func (c *Controller) PreviewMedia(ctx context.Context, mediaID string) (State, error) {
	c.mu.Lock()
	defer c.mu.Unlock()
	return c.previewMedia(ctx, mediaID)
}

func (c *Controller) ProjectMedia(ctx context.Context, mediaID string) (State, error) {
	c.mu.Lock()
	defer c.mu.Unlock()
	return c.sendLive(c.previewMedia(ctx, mediaID))
}

func (c *Controller) ProjectScripture(
	ctx context.Context, translationID string, bookID, chapter, startVerse, endVerse int,
) (State, error) {
	c.mu.Lock()
	defer c.mu.Unlock()
	return c.sendLive(c.previewScripture(ctx, translationID, bookID, chapter, startVerse, endVerse))
}

func (c *Controller) ProjectSongSection(ctx context.Context, songID, sectionID string) (State, error) {
	c.mu.Lock()
	defer c.mu.Unlock()
	return c.sendLive(c.previewSongSection(ctx, songID, sectionID))
}

// RefreshPreview rebuilds the previewed slide from whatever it was made of, so a
// theme edit appears without the operator hunting for the item again. Live output
// is never touched: a slide already on the wall keeps the look it was projected
// with until something else is sent.
func (c *Controller) RefreshPreview(ctx context.Context) (State, error) {
	c.mu.Lock()
	defer c.mu.Unlock()

	if c.preview == nil {
		return c.stateLocked(), ErrNoPreview
	}

	source := c.source
	switch {
	case source.itemID != "":
		return c.previewItem(ctx, source.itemID)
	case source.mediaID != "":
		return c.previewMedia(ctx, source.mediaID)
	case source.songID != "":
		return c.previewSongSection(ctx, source.songID, source.sectionID)
	case source.translationID != "":
		return c.previewScripture(ctx, source.translationID, source.bookID,
			source.chapter, source.verseStart, source.verseEnd)
	}
	return c.stateLocked(), ErrNoPreview
}

func (c *Controller) ProjectPreview() (State, error) {
	c.mu.Lock()
	defer c.mu.Unlock()

	if c.preview == nil {
		return c.stateLocked(), ErrNoPreview
	}
	c.live = clone(c.preview)
	return c.stateLocked(), nil
}

func (c *Controller) ClearLive() State {
	c.mu.Lock()
	defer c.mu.Unlock()

	c.live = nil
	return c.stateLocked()
}

func (c *Controller) Next(ctx context.Context) (State, error) {
	c.mu.Lock()
	defer c.mu.Unlock()
	return c.move(ctx, 1)
}

func (c *Controller) Previous(ctx context.Context) (State, error) {
	c.mu.Lock()
	defer c.mu.Unlock()
	return c.move(ctx, -1)
}

func (c *Controller) move(ctx context.Context, direction int) (State, error) {
	var serviceID string
	currentIndex := -1

	if c.preview != nil && c.preview.ItemID != "" {
		item, err := c.schedule.Item(ctx, c.preview.ItemID)
		if err != nil {
			return c.stateLocked(), err
		}
		serviceID = item.ServiceID
	} else {
		service, _, err := c.schedule.Current(ctx)
		if err != nil {
			if errors.Is(err, serviceorder.ErrNoService) {
				return c.stateLocked(), ErrNoSchedule
			}
			return c.stateLocked(), err
		}
		serviceID = service.ID
	}

	items, err := c.schedule.Items(ctx, serviceID)
	if err != nil {
		return c.stateLocked(), err
	}
	if len(items) == 0 {
		return c.stateLocked(), ErrNoSchedule
	}

	if c.preview != nil && c.preview.ItemID != "" {
		for index, item := range items {
			if item.ID == c.preview.ItemID {
				currentIndex = index
				break
			}
		}
	}

	target := currentIndex + direction
	if currentIndex == -1 {
		target = 0
	}
	if target < 0 {
		target = 0
	}
	if target >= len(items) {
		target = len(items) - 1
	}

	content, err := c.contentForItem(ctx, items[target])
	if err != nil {
		return c.stateLocked(), err
	}
	c.preview = content
	c.source = previewSource{itemID: items[target].ID}
	return c.stateLocked(), nil
}

// itemContent builds the slide for one schedule item and remembers the item, so
// a later theme edit rebuilds this same slide rather than the last one clicked.
func (c *Controller) itemContent(ctx context.Context, itemID string) (*Content, error) {
	item, err := c.schedule.Item(ctx, itemID)
	if err != nil {
		return nil, err
	}
	content, err := c.contentForItem(ctx, item)
	if err != nil {
		return nil, err
	}
	c.source = previewSource{itemID: itemID}
	return content, nil
}

func (c *Controller) contentForItem(ctx context.Context, item serviceorder.Item) (*Content, error) {
	switch item.Kind {
	case serviceorder.KindSong:
		return c.songContent(ctx, item.ID, item.ThemeID, item.SongID, item.SectionID)
	case serviceorder.KindMedia:
		return c.mediaContent(ctx, item.ID, item.MediaID)
	case serviceorder.KindText:
		return c.textContent(ctx, item)
	case serviceorder.KindAnnouncement:
		return c.announcementContent(ctx, item)
	case serviceorder.KindBlank:
		return c.blankContent(ctx, item)
	default:
		return c.scriptureContent(ctx, item.ID, item.ThemeID, item.TranslationID,
			item.BookID, item.Chapter, item.VerseStart, item.VerseEnd)
	}
}

func (c *Controller) textContent(ctx context.Context, item serviceorder.Item) (*Content, error) {
	return &Content{
		ItemID: item.ID,
		Kind:   "text",
		Title:  item.Title,
		Label:  item.Title,
		Lines:  splitLines(item.Content),
		Theme:  c.themeFor(ctx, item.ThemeID, themes.KindText),
	}, nil
}

func (c *Controller) announcementContent(ctx context.Context, item serviceorder.Item) (*Content, error) {
	return &Content{
		ItemID: item.ID,
		Kind:   "announcement",
		Title:  item.Title,
		Label:  item.Title,
		Lines:  splitLines(item.Content),
		Theme:  c.themeFor(ctx, item.ThemeID, themes.KindText),
	}, nil
}

func (c *Controller) blankContent(ctx context.Context, item serviceorder.Item) (*Content, error) {
	return &Content{
		ItemID: item.ID,
		Kind:   "blank",
		Title:  item.Title,
		Label:  item.Title,
		Lines:  []string{},
		Theme:  c.themeFor(ctx, item.ThemeID, themes.KindScripture),
	}, nil
}

// themeFor decides which theme a slide is drawn with: the override on the item,
// then the default for its kind, then the app's own readable defaults. That last
// step matters on a fresh install, where nobody has made a theme yet but a slide
// still has to look right on the wall.
//
// An override that no longer exists, or belongs to another kind, is ignored
// rather than trusted: a theme deleted between services should not blank a slide.
func (c *Controller) themeFor(ctx context.Context, override string, kind themes.Kind) themes.Theme {
	if c.themes != nil {
		if override != "" {
			if theme, err := c.themes.Get(ctx, override); err == nil && theme.Kind == kind {
				return theme.Normalised()
			}
		}
		if theme, err := c.themes.Default(ctx, kind); err == nil {
			return theme.Normalised()
		}
	}
	return themes.DefaultTheme(kind)
}

func (c *Controller) mediaContent(ctx context.Context, itemID, mediaID string) (*Content, error) {
	if c.media == nil {
		return nil, errors.New("media is not available")
	}
	slide, err := c.media.Slide(ctx, mediaID)
	if err != nil {
		return nil, err
	}

	return &Content{
		ItemID: itemID,
		Kind:   slide.Kind,
		Title:  slide.Title,
		Label:  slide.Title,
		Source: slide.URL,
		Aspect: slide.Aspect,
		Fit:    slide.Fit,
		Ratio:  slide.Ratio,
		Lines:  []string{},
	}, nil
}

func (c *Controller) scriptureContent(
	ctx context.Context, itemID, themeID, translationID string, bookID, chapter, startVerse, endVerse int,
) (*Content, error) {
	verses, err := c.scripture.Passage(ctx, translationID, bookID, chapter, startVerse, endVerse)
	if err != nil {
		return nil, err
	}
	if len(verses) == 0 {
		return nil, ErrEmptyPassage
	}
	translation, err := c.scripture.Translation(ctx, translationID)
	if err != nil {
		return nil, err
	}

	reference := referenceFromVerses(verses)
	lines := make([]string, 0, len(verses))
	for _, verse := range verses {
		lines = append(lines, splitLines(verse.Text)...)
	}

	return &Content{
		ItemID:      itemID,
		Kind:        "scripture",
		Title:       reference,
		Label:       reference,
		Reference:   reference,
		Translation: translation.Abbreviation,
		Lines:       lines,
		Theme:       c.themeFor(ctx, themeID, themes.KindScripture),
	}, nil
}

func (c *Controller) songContent(ctx context.Context, itemID, themeID, songID, sectionID string) (*Content, error) {
	if c.song == nil {
		return nil, errors.New("songs are not available")
	}
	song, section, err := c.song.Section(ctx, songID, sectionID)
	if err != nil {
		return nil, err
	}

	label := song.Title
	if section.Label != "" {
		label = fmt.Sprintf("%s · %s", song.Title, section.Label)
	}

	return &Content{
		ItemID:   itemID,
		Kind:     "song",
		Title:    song.Title,
		Subtitle: section.Label,
		Label:    label,
		Lines:    splitLines(section.Text),
		Theme:    c.themeFor(ctx, themeID, themes.KindLyrics),
	}, nil
}

func splitLines(text string) []string {
	parts := strings.Split(text, "\n")
	lines := make([]string, 0, len(parts))
	for _, part := range parts {
		if trimmed := strings.TrimSpace(part); trimmed != "" {
			lines = append(lines, trimmed)
		}
	}
	return lines
}

func referenceFromVerses(verses []bible.SearchResult) string {
	first := verses[0]
	last := verses[len(verses)-1]
	if first.Chapter == last.Chapter && last.Verse > first.Verse {
		return fmt.Sprintf("%s %d:%d-%d", first.Book.Name, first.Chapter, first.Verse, last.Verse)
	}
	if last.Verse > first.Verse {
		return fmt.Sprintf("%s %d:%d-%d:%d", first.Book.Name, first.Chapter, first.Verse, last.Chapter, last.Verse)
	}
	return first.Reference()
}

func clone(content *Content) *Content {
	if content == nil {
		return nil
	}
	copied := *content
	copied.Lines = append([]string(nil), content.Lines...)
	return &copied
}
