package bible

import (
	"errors"
	"fmt"
	"time"
)

var (
	ErrTranslationNotFound = errors.New("translation not found")
	ErrEmptyQuery          = errors.New("search text is empty")
	ErrInvalidTranslation  = errors.New("invalid translation data")
	ErrUnsupportedPackage  = errors.New("translation package was built by a newer version")
)

type TranslationMeta struct {
	ID           string
	Name         string
	Abbreviation string
	Language     string
	Version      string
}

type Translation struct {
	TranslationMeta
	VerseCount  int
	Checksum    string
	InstalledAt time.Time
}

// Verse is a single verse addressed by canonical book, chapter and number.
type Verse struct {
	BookID  int
	Chapter int
	Number  int
	Text    string
}

type TranslationData struct {
	TranslationMeta
	Verses []Verse
}

type SearchMode string

const (
	SearchWords  SearchMode = "words"
	SearchPhrase SearchMode = "phrase"
	SearchPrefix SearchMode = "prefix"
)

type SearchRequest struct {
	TranslationID string
	Text          string
	Mode          SearchMode
	Limit         int
	Offset        int
	// Optional filters — zero value means "no filter"
	Testament Testament
	BookIDs   []int
}

type SearchResult struct {
	Book    Book
	Chapter int
	Verse   int
	Text    string
	// Snippet is an HTML-safe excerpt with <mark> tags around matched terms.
	// It is populated only for full-text search results (not passage lookups).
	Snippet string
}

func (r SearchResult) Reference() string {
	return fmt.Sprintf("%s %d:%d", r.Book.Name, r.Chapter, r.Verse)
}
