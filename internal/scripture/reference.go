package scripture

import (
	"errors"
	"fmt"
	"regexp"
	"strconv"
	"strings"

	"zopresent/internal/bible"
)

var (
	// ErrNotReference means the query is ordinary text, so search should
	// continue with full-text matching.
	ErrNotReference = errors.New("not a scripture reference")
	// ErrInvalidReference means the query names a real book and looks like a
	// reference, but the chapter or verse range is unusable.
	ErrInvalidReference = errors.New("invalid scripture reference")
)

// Reference is a parsed scripture address. VerseStart and VerseEnd are zero
// when the reference covers a whole chapter.
type Reference struct {
	Book       bible.Book
	Chapter    int
	VerseStart int
	VerseEnd   int
}

func (r Reference) String() string {
	if r.VerseStart == 0 {
		return fmt.Sprintf("%s %d", r.Book.Name, r.Chapter)
	}
	if r.VerseEnd == 0 || r.VerseEnd == r.VerseStart {
		return fmt.Sprintf("%s %d:%d", r.Book.Name, r.Chapter, r.VerseStart)
	}
	return fmt.Sprintf("%s %d:%d-%d", r.Book.Name, r.Chapter, r.VerseStart, r.VerseEnd)
}

var referencePattern = regexp.MustCompile(`^(.+?)\s+(\d+)(?:\s*:\s*(\d+)(?:\s*[-–]\s*(\d+))?)?$`)

// Parse accepts forms such as "John 3:16", "Romans 8:28-30", "Psalm 23" and
// "1 John 5:7". Book names are resolved through the canonical alias table.
func Parse(query string) (Reference, error) {
	query = strings.TrimSpace(query)
	if query == "" {
		return Reference{}, ErrNotReference
	}

	match := referencePattern.FindStringSubmatch(query)
	if match == nil {
		return Reference{}, ErrNotReference
	}

	book, ok := bible.LookupBook(match[1])
	if !ok {
		return Reference{}, ErrNotReference
	}

	chapter, err := strconv.Atoi(match[2])
	if err != nil || chapter <= 0 {
		return Reference{}, fmt.Errorf("%w: chapter must be a positive number", ErrInvalidReference)
	}

	reference := Reference{Book: book, Chapter: chapter}
	if match[3] == "" {
		return reference, nil
	}

	start, err := strconv.Atoi(match[3])
	if err != nil || start <= 0 {
		return Reference{}, fmt.Errorf("%w: verse must be a positive number", ErrInvalidReference)
	}
	reference.VerseStart = start
	reference.VerseEnd = start

	if match[4] != "" {
		end, err := strconv.Atoi(match[4])
		if err != nil || end <= 0 {
			return Reference{}, fmt.Errorf("%w: verse must be a positive number", ErrInvalidReference)
		}
		if end < start {
			return Reference{}, fmt.Errorf("%w: verse range ends before it starts", ErrInvalidReference)
		}
		reference.VerseEnd = end
	}

	return reference, nil
}
