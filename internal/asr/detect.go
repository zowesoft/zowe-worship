// Package asr holds the offline speech-to-text adapter layer and the
// spoken-reference detector behind the Live Detection workspace.
package asr

import (
	"strconv"
	"strings"

	"zopresent/internal/bible"
	"zopresent/internal/scripture"
)

type Detection struct {
	Reference scripture.Reference
	// Raw is the normalized candidate string, useful for debugging ASR near misses.
	Raw string
	// Method names how the passage was found. Empty means an explicit spoken
	// reference, the rule-based detector's only output; the quote path sets its
	// own value so the queue can label and score it differently.
	Method string
	// Confidence carries a score the detector already computed. Zero means "score
	// the phrasing", which is what an explicit reference needs; a quote match
	// supplies its own overlap score here instead.
	Confidence float64
}

// DetectReferences scans transcript text for spoken scripture references
// ("Romans chapter eight verse twenty-eight", "John 3:16", "first Corinthians
// thirteen"). Results are deduplicated, and candidates with an unusable chapter
// or verse range are dropped. The detector is deliberately conservative:
// ambiguity means no match.
func DetectReferences(text string) []Detection {
	tokens := tokenizeForDetection(normalizeSpokenText(text))
	consumed := make([]bool, len(tokens))

	var detections []Detection
	seen := make(map[string]bool)

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

		// Spoken ordinal prefix ("first Corinthians thirteen"): its own pattern
		// because the leading digit terminates the alphabetic book scan.
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

		// A book name may carry spoken filler ("song of solomon"), so scan up to
		// six words back for it.
		bookEnd := i - 1
		for bookEnd >= 0 && consumed[bookEnd] {
			bookEnd--
		}
		// A speaker may hedge between the book and its chapter ("Isaiah, I
		// think it's chapter 41"); step back over that doubt so the window
		// still ends on the book name rather than on "its".
		for bookEnd >= 0 && !consumed[bookEnd] && hedgeFiller[tokens[bookEnd]] {
			bookEnd--
		}
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

// parseVerseTail reads the verse numbers after a chapter token (": 28 - 30",
// bare "28 30", "28 and 30", or a hedged "from 25 to 34") and returns them with
// the last token consumed.
func parseVerseTail(tokens []string, consumed []bool, index int) (verseStart, verseEnd, last int) {
	// Speakers hedge the verse ("from verse 25", "around verses 35",
	// "especially the first verse", "probably verse 28"); once the connector
	// words drop, that filler carries no number, so step over it to reach the
	// verse itself.
	for index < len(tokens) && !consumed[index] && (verseFiller[tokens[index]] || hedgeFiller[tokens[index]]) {
		index++
	}
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

	// A range closes with a dash or a spoken joiner: "28 - 30", "6 and 7",
	// "25 to 34", "35 through 41" all mean the same span.
	if verseStart > 0 && index < len(tokens) && !consumed[index] && rangeJoiner[tokens[index]] &&
		index+1 < len(tokens) && !consumed[index+1] {
		if v, ok := tokenNumber(tokens[index+1]); ok {
			verseEnd = v
			last = index + 1
		}
	}
	return verseStart, verseEnd, last
}

// verseFiller is the hedging a speaker puts between a chapter and its verse
// once the connector words ("verse", "verses") are already dropped.
var verseFiller = map[string]bool{
	"from": true, "around": true, "about": true, "especially": true,
	"the": true, "approximately": true, "roughly": true,
}

// rangeJoiner closes a verse range: the written dash and the spoken words that
// carry the same meaning.
var rangeJoiner = map[string]bool{
	"-": true, "and": true, "to": true, "through": true, "until": true,
}

// hedgeFiller is the doubt a speaker inserts between a book name and its
// chapter ("Isaiah, I think it's chapter 41"). None of these words appear in a
// canonical book name, so trimming them from the book window's edge cannot eat
// into a real multi-word book like "song of solomon".
var hedgeFiller = map[string]bool{
	"i": true, "im": true, "think": true, "its": true, "it": true,
	"believe": true, "maybe": true, "probably": true, "is": true,
	"was": true, "that": true, "this": true, "there": true,
}

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

// tokenNumber accepts only 1..999, the plausible chapter/verse range.
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
