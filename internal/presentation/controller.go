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

// Controller owns preview and live output. Recognition and UI code never write
// live state directly; only the explicit projection commands here do.
type Controller struct {
	mu        sync.Mutex
	scripture ScriptureReader
	song      SongReader
	media     MediaReader
	schedule  ScheduleReader

	preview *Content
	live    *Content
}

func New(scripture ScriptureReader, schedule ScheduleReader, song SongReader, media MediaReader) *Controller {
	return &Controller{scripture: scripture, schedule: schedule, song: song, media: media}
}

func (c *Controller) State() State {
	c.mu.Lock()
	defer c.mu.Unlock()
	return c.stateLocked()
}

func (c *Controller) stateLocked() State {
	return State{Preview: clone(c.preview), Live: clone(c.live)}
}

func (c *Controller) PreviewScripture(
	ctx context.Context, translationID string, bookID, chapter, startVerse, endVerse int,
) (State, error) {
	c.mu.Lock()
	defer c.mu.Unlock()

	content, err := c.scriptureContent(ctx, "", translationID, bookID, chapter, startVerse, endVerse)
	if err != nil {
		return c.stateLocked(), err
	}
	c.preview = content
	return c.stateLocked(), nil
}

func (c *Controller) PreviewSongSection(ctx context.Context, songID, sectionID string) (State, error) {
	c.mu.Lock()
	defer c.mu.Unlock()

	content, err := c.songContent(ctx, "", songID, sectionID)
	if err != nil {
		return c.stateLocked(), err
	}
	c.preview = content
	return c.stateLocked(), nil
}

func (c *Controller) PreviewItem(ctx context.Context, itemID string) (State, error) {
	c.mu.Lock()
	defer c.mu.Unlock()

	item, err := c.schedule.Item(ctx, itemID)
	if err != nil {
		return c.stateLocked(), err
	}
	content, err := c.itemContent(ctx, item)
	if err != nil {
		return c.stateLocked(), err
	}
	c.preview = content
	return c.stateLocked(), nil
}

func (c *Controller) PreviewMedia(ctx context.Context, mediaID string) (State, error) {
	c.mu.Lock()
	defer c.mu.Unlock()

	content, err := c.mediaContent(ctx, "", mediaID)
	if err != nil {
		return c.stateLocked(), err
	}
	c.preview = content
	return c.stateLocked(), nil
}

// ProjectMedia leaves live output untouched when the asset cannot be shown, so a
// missing file never blanks the room.
func (c *Controller) ProjectMedia(ctx context.Context, mediaID string) (State, error) {
	c.mu.Lock()
	defer c.mu.Unlock()

	content, err := c.mediaContent(ctx, "", mediaID)
	if err != nil {
		return c.stateLocked(), err
	}
	c.preview = content
	c.live = clone(content)
	return c.stateLocked(), nil
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

func (c *Controller) ProjectScripture(
	ctx context.Context, translationID string, bookID, chapter, startVerse, endVerse int,
) (State, error) {
	c.mu.Lock()
	defer c.mu.Unlock()

	content, err := c.scriptureContent(ctx, "", translationID, bookID, chapter, startVerse, endVerse)
	if err != nil {
		return c.stateLocked(), err
	}
	c.preview = content
	c.live = clone(content)
	return c.stateLocked(), nil
}

func (c *Controller) ProjectSongSection(ctx context.Context, songID, sectionID string) (State, error) {
	c.mu.Lock()
	defer c.mu.Unlock()

	content, err := c.songContent(ctx, "", songID, sectionID)
	if err != nil {
		return c.stateLocked(), err
	}
	c.preview = content
	c.live = clone(content)
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

	content, err := c.itemContent(ctx, items[target])
	if err != nil {
		return c.stateLocked(), err
	}
	c.preview = content
	return c.stateLocked(), nil
}

func (c *Controller) itemContent(ctx context.Context, item serviceorder.Item) (*Content, error) {
	switch item.Kind {
	case serviceorder.KindSong:
		return c.songContent(ctx, item.ID, item.SongID, item.SectionID)
	case serviceorder.KindMedia:
		return c.mediaContent(ctx, item.ID, item.MediaID)
	default:
		return c.scriptureContent(ctx, item.ID, item.TranslationID,
			item.BookID, item.Chapter, item.VerseStart, item.VerseEnd)
	}
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
	ctx context.Context, itemID, translationID string, bookID, chapter, startVerse, endVerse int,
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
	}, nil
}

func (c *Controller) songContent(ctx context.Context, itemID, songID, sectionID string) (*Content, error) {
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
