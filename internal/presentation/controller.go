package presentation

import (
	"context"
	"errors"
	"fmt"
	"sync"

	"zowe-worship/internal/bible"
	"zowe-worship/internal/serviceorder"
)

var (
	ErrNoPreview    = errors.New("no preview content")
	ErrNoSchedule   = errors.New("no service schedule")
	ErrEmptyPassage = errors.New("passage has no verses")
)

type Verse struct {
	BookID  int    `json:"bookId"`
	Book    string `json:"book"`
	Chapter int    `json:"chapter"`
	Verse   int    `json:"verse"`
	Text    string `json:"text"`
}

// Content is a frozen snapshot of scripture so live output survives a
// translation being removed or updated mid-service.
type Content struct {
	ItemID        string  `json:"itemId,omitempty"`
	Kind          string  `json:"kind"`
	Title         string  `json:"title"`
	Reference     string  `json:"reference"`
	Translation   string  `json:"translation"`
	TranslationID string  `json:"translationId"`
	Verses        []Verse `json:"verses"`
}

type State struct {
	Preview *Content `json:"preview,omitempty"`
	Live    *Content `json:"live,omitempty"`
}

type ScriptureReader interface {
	Passage(ctx context.Context, translationID string, bookID, chapter, startVerse, endVerse int) ([]bible.SearchResult, error)
	Translation(ctx context.Context, id string) (bible.Translation, error)
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
	Scripture ScriptureReader
	Schedule  ScheduleReader

	preview *Content
	live    *Content
}

func New(scripture ScriptureReader, schedule ScheduleReader) *Controller {
	return &Controller{Scripture: scripture, Schedule: schedule}
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

	content, err := c.resolve(ctx, "", "", translationID, bookID, chapter, startVerse, endVerse)
	if err != nil {
		return c.stateLocked(), err
	}
	c.preview = content
	return c.stateLocked(), nil
}

func (c *Controller) PreviewItem(ctx context.Context, itemID string) (State, error) {
	c.mu.Lock()
	defer c.mu.Unlock()

	item, err := c.Schedule.Item(ctx, itemID)
	if err != nil {
		return c.stateLocked(), err
	}
	content, err := c.resolve(ctx, item.ID, item.Title, item.TranslationID,
		item.BookID, item.Chapter, item.VerseStart, item.VerseEnd)
	if err != nil {
		return c.stateLocked(), err
	}
	c.preview = content
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

	content, err := c.resolve(ctx, "", "", translationID, bookID, chapter, startVerse, endVerse)
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
		item, err := c.Schedule.Item(ctx, c.preview.ItemID)
		if err != nil {
			return c.stateLocked(), err
		}
		serviceID = item.ServiceID
	} else {
		service, _, err := c.Schedule.Current(ctx)
		if err != nil {
			if errors.Is(err, serviceorder.ErrNoService) {
				return c.stateLocked(), ErrNoSchedule
			}
			return c.stateLocked(), err
		}
		serviceID = service.ID
	}

	items, err := c.Schedule.Items(ctx, serviceID)
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

	item := items[target]
	content, err := c.resolve(ctx, item.ID, item.Title, item.TranslationID,
		item.BookID, item.Chapter, item.VerseStart, item.VerseEnd)
	if err != nil {
		return c.stateLocked(), err
	}
	c.preview = content
	return c.stateLocked(), nil
}

func (c *Controller) resolve(
	ctx context.Context, itemID, title, translationID string, bookID, chapter, startVerse, endVerse int,
) (*Content, error) {
	verses, err := c.Scripture.Passage(ctx, translationID, bookID, chapter, startVerse, endVerse)
	if err != nil {
		return nil, err
	}
	if len(verses) == 0 {
		return nil, ErrEmptyPassage
	}
	translation, err := c.Scripture.Translation(ctx, translationID)
	if err != nil {
		return nil, err
	}

	reference := title
	if reference == "" {
		reference = referenceFromVerses(verses)
	}

	rendered := make([]Verse, 0, len(verses))
	for _, verse := range verses {
		rendered = append(rendered, Verse{
			BookID:  verse.Book.ID,
			Book:    verse.Book.Name,
			Chapter: verse.Chapter,
			Verse:   verse.Verse,
			Text:    verse.Text,
		})
	}

	return &Content{
		ItemID:        itemID,
		Kind:          "scripture",
		Title:         reference,
		Reference:     reference,
		Translation:   translation.Abbreviation,
		TranslationID: translationID,
		Verses:        rendered,
	}, nil
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
	copied.Verses = append([]Verse(nil), content.Verses...)
	return &copied
}
