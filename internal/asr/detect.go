// Package asr holds the offline speech-to-text adapter layer and the
// spoken-reference detector used by the future Live Detection workspace.
package asr

import (
	"strconv"
	"strings"

	"zowe-worship/internal/bible"
	"zowe-worship/internal/scripture"
)

// Detection is a scripture reference found in spoken (or transcribed) text.
type Detection struct {
	Reference scripture.Reference
	// Raw is the normalized candidate string the detection was parsed from,
	// useful for debugging ASR errors that produced a near miss.
	Raw string
}

// DetectReferences scans free-form transcript text for spoken scripture
// references such as "Romans chapter eight verse twenty-eight",
// "let's turn to John 3:16", "first Corinthians thirteen" or "Psalm twenty
// three". Results are deduplicated; candidates that name a real book but
// carry an unusable chapter or verse range are dropped by scripture.Parse's
// rules and therefore not returned.
//
// The detector is deliberately conservative: ambiguity means no match, so an
// operator always confirms before projection (PRD section 9).
func DetectReferences(text string) []Detection {
	tokens := tokenizeForDetection(normalizeSpokenText(text))
	consumed := make([]bool, len(tokens))

	var detections []Detection
	seen := make(map[string]bool)

	// commit records a validated detection spanning tokens [from..to].
	// Returns false when the surrounding numbers were not a usable reference.
	commit := func(from, to int, book bible.Book, chapter, verseStart, verseEnd int) bool {
		ref, ok := buildReference(book, chapter, verseStart, verseEnd)
		if !ok {
			return false
		}
		key := ref.String()
		if !seen[key] {
			seen[key] = true
			detections = append(detections, Detection{Reference: ref, Raw: key})
		}
		for k := from; k <= to && k < len(consumed); k++ {
			consumed[k] = true
		}
		return true
	}

	for i := 0; i < len(tokens); i++ {
		if consumed[i] {
			continue
		}
		chapter, ok := tokenNumber(tokens[i])
		if !ok {
			continue
		}

		// Spoken ordinal prefix, e.g. "1 Corinthians 13" or "2 Timothy 4 2"
		// spoken as "first Corinthians thirteen", "second Timothy four two".
		// Handled as its own pattern because the leading digit terminates the
		// alphabetic book scan.
		if chapter <= 3 && i+2 < len(tokens) && !consumed[i+1] && !consumed[i+2] {
			book, ok := bible.LookupBook(tokens[i+1])
			if !ok {
				// Standalone "corinthians" is not canonical; "2 timothy" is.
				book, ok = bible.LookupBook(strconv.Itoa(chapter) + " " + tokens[i+1])
			}
			if ok {
				if chapterNumber, ok := tokenNumber(tokens[i+2]); ok {
					verseStart, verseEnd, last := parseVerseTail(tokens, consumed, i+3)
					if commit(i, last, book, chapterNumber, verseStart, verseEnd) {
						i = last
						continue
					}
				}
			}
		}

		// Book name may be preceded by spoken filler; scan at most six words
		// back ("song of solomon", "to the book of first peter" after
		// normalization maps first->1 ... handled by the pattern above).
		bookEnd := i - 1
		for bookEnd >= 0 && consumed[bookEnd] {
			bookEnd--
		}
		// Candidate book names are the tokens between bookEnd and the chapter
		// number: bookEnd..i-1, so the widest window is min(i, 6) tokens.
		window := i
		if window > 6 {
			window = 6
		}
		verseStart, verseEnd, tailLast := parseVerseTail(tokens, consumed, i+1)
		for width := 1; width <= window; width++ {
			start := bookEnd - width + 1
			if start < 0 {
				continue
			}
			// Standalone ":" tokens (from "Revelation: 22:21") never belong
			// inside a book name; trim them from both window edges.
			nameStart := start
			for nameStart <= bookEnd && tokens[nameStart] == ":" {
				nameStart++
			}
			nameEnd := bookEnd
			for nameEnd >= nameStart && tokens[nameEnd] == ":" {
				nameEnd--
			}
			if nameStart > nameEnd {
				continue
			}
			book, ok := bible.LookupBook(strings.Join(tokens[nameStart:nameEnd+1], " "))
			if !ok {
				continue
			}
			if commit(nameStart, tailLast, book, chapter, verseStart, verseEnd) {
				i = tailLast
				break
			}
		}
	}

	return detections
}

// parseVerseTail reads ": verse - verse" or bare trailing verse numbers after
// a chapter token, e.g. "8 : 28 - 30", "8 28 30" (spoken with chapter and
// verse words already removed) or "8 28". Returns the verse values and the
// index of the last token consumed.
func parseVerseTail(tokens []string, consumed []bool, index int) (verseStart, verseEnd, last int) {
	last = index - 1
	if index >= len(tokens) || consumed[index] {
		return
	}

	if tokens[index] == ":" {
		if index+1 < len(tokens) {
			if v, ok := tokenNumber(tokens[index+1]); ok {
				verseStart = v
				last = index + 1
				index += 2
			}
		}
	} else if n, ok := tokenNumber(tokens[index]); ok {
		// Bare second number: "John 3 16" after connector words were dropped.
		// Written text rarely has this form, but spoken transcripts produce it
		// constantly.
		verseStart = n
		last = index
		index++
	} else if tokens[index] == "and" && index+1 < len(tokens) && !consumed[index+1] {
		// "chapter 8 and verse 28" leaves "8 and 28" once the connector words
		// drop: here the conjunction introduces the verse, not a second
		// chapter.
		if v, ok := tokenNumber(tokens[index+1]); ok {
			verseStart = v
			last = index + 1
			index += 2
		}
	}

	if verseStart > 0 && index < len(tokens) && tokens[index] == "-" &&
		index+1 < len(tokens) && !consumed[index+1] {
		if v, ok := tokenNumber(tokens[index+1]); ok {
			verseEnd = v
			last = index + 1
		}
	}

	// Spoken ranges join with "and": "verses 6 and 7" leaves the tokens
	// "6 and 7" once the connector words are dropped, so the conjunction
	// closes the range exactly like a dash would.
	if verseEnd == 0 && verseStart > 0 && index < len(tokens) && tokens[index] == "and" &&
		index+1 < len(tokens) && !consumed[index+1] {
		if v, ok := tokenNumber(tokens[index+1]); ok {
			verseEnd = v
			last = index + 1
		}
	}
	return verseStart, verseEnd, last
}

// buildReference validates numbers against the book's shape and renders the
// raw candidate string.
func buildReference(book bible.Book, chapter, verseStart, verseEnd int) (scripture.Reference, bool) {
	if chapter <= 0 || chapter > book.Chapters {
		return scripture.Reference{}, false
	}
	ref := scripture.Reference{Book: book, Chapter: chapter}
	if verseStart > 0 {
		if verseEnd == 0 {
			verseEnd = verseStart
		}
		if verseEnd < verseStart {
			return scripture.Reference{}, false
		}
		ref.VerseStart = verseStart
		ref.VerseEnd = verseEnd
	}
	return ref, true
}

// tokenNumber parses an all-digit token as a plausible chapter/verse number.
func tokenNumber(token string) (int, bool) {
	value := 0
	for _, r := range token {
		if r < '0' || r > '9' {
			return 0, false
		}
		value = value*10 + int(r-'0')
	}
	if value <= 0 || value > 999 {
		return 0, false
	}
	return value, true
}
