package bible

import (
	"encoding/json"
	"fmt"
	"io"
	"sort"
	"strconv"
	"strings"
)

// DecodeJSON reads a translation dataset shaped as book -> chapter -> verse -> text.
// It maps dataset book spelling ("Psalm", "Song Of Solomon") onto canonical books.
func DecodeJSON(r io.Reader, meta TranslationMeta) (TranslationData, error) {
	var raw map[string]map[string]map[string]string
	if err := json.NewDecoder(r).Decode(&raw); err != nil {
		return TranslationData{}, fmt.Errorf("%w: decode dataset: %v", ErrInvalidTranslation, err)
	}
	if len(raw) == 0 {
		return TranslationData{}, fmt.Errorf("%w: dataset is empty", ErrInvalidTranslation)
	}

	byBook := make(map[int]map[string]map[string]string, len(raw))
	for datasetName, chapters := range raw {
		book, ok := LookupBook(datasetName)
		if !ok {
			return TranslationData{}, fmt.Errorf("%w: unknown book %q", ErrInvalidTranslation, datasetName)
		}
		if _, exists := byBook[book.ID]; exists {
			return TranslationData{}, fmt.Errorf("%w: book %q is present more than once", ErrInvalidTranslation, book.Name)
		}
		byBook[book.ID] = chapters
	}

	var verses []Verse
	for _, book := range CanonicalBooks {
		chapters, ok := byBook[book.ID]
		if !ok {
			continue
		}
		chapterNumbers, err := sortedNumbers(chapters)
		if err != nil {
			return TranslationData{}, fmt.Errorf("%w: %s: %v", ErrInvalidTranslation, book.Name, err)
		}
		for _, chapterNumber := range chapterNumbers {
			chapter := chapters[strconv.Itoa(chapterNumber)]
			verseNumbers, err := sortedNumbers(chapter)
			if err != nil {
				return TranslationData{}, fmt.Errorf(
					"%w: %s %d: %v", ErrInvalidTranslation, book.Name, chapterNumber, err)
			}
			for _, verseNumber := range verseNumbers {
				text := chapter[strconv.Itoa(verseNumber)]
				if strings.TrimSpace(text) == "" {
					return TranslationData{}, fmt.Errorf(
						"%w: %s %d:%d has no text", ErrInvalidTranslation, book.Name, chapterNumber, verseNumber)
				}
				verses = append(verses, Verse{
					BookID:  book.ID,
					Chapter: chapterNumber,
					Number:  verseNumber,
					Text:    text,
				})
			}
		}
	}

	if len(verses) == 0 {
		return TranslationData{}, fmt.Errorf("%w: dataset has no verses", ErrInvalidTranslation)
	}
	return TranslationData{TranslationMeta: meta, Verses: verses}, nil
}

func sortedNumbers[T any](entries map[string]T) ([]int, error) {
	numbers := make([]int, 0, len(entries))
	for key := range entries {
		number, err := strconv.Atoi(key)
		if err != nil || number <= 0 {
			return nil, fmt.Errorf("invalid verse or chapter number %q", key)
		}
		numbers = append(numbers, number)
	}
	sort.Ints(numbers)
	return numbers, nil
}
