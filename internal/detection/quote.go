package detection

import (
	"sort"
	"strings"

	"zopresent/internal/asr"
	"zopresent/internal/bible"
	"zopresent/internal/confidence"
	"zopresent/internal/scripture"
)

// maxQuotesPerSegment caps how many quoted passages one settled chunk can add.
// A chunk is thirty seconds of speech; even a dense sermon quotes only a handful
// of verses in it, and the cap keeps a topical near-match from flooding the queue
// the operator has to work through.
const maxQuotesPerSegment = 3

// DetectQuotes turns the index's ranked verse matches into spoken-quote
// detections. It keeps only verses the preacher actually quoted — enough
// distinctive words, reproduced closely enough — merges neighbouring verses into
// the one passage they form, and drops anything an explicit reference or a
// pending card already covers, so the stronger evidence is never duplicated or
// overwritten by the weaker quote.
//
// matches come from bible.MatchQuote for this transcript and the selected
// translation. explicit and pending are the hearings already in hand; a quote
// overlapping either is skipped. The returned detections carry MethodSpokenQuote
// and their own confidence, which the queue keeps as-is.
func DetectQuotes(transcript string, matches []bible.QuoteMatch, explicit []asr.Detection, pending []Candidate) []asr.Detection {
	spoken := bible.ContentWords(transcript)
	if len(spoken) == 0 || len(matches) == 0 {
		return nil
	}
	spokenSet := make(map[string]bool, len(spoken))
	for _, word := range spoken {
		spokenSet[word] = true
	}

	// Keep the verses whose words the transcript reproduces closely enough to be
	// a quotation rather than a shared subject.
	type quotedVerse struct {
		book    bible.Book
		chapter int
		number  int
		text    string
	}
	var verses []quotedVerse
	for _, match := range matches {
		words := bible.ContentWords(match.Result.Text)
		if score := quoteOverlap(words, spoken, spokenSet); score == 0 {
			continue
		}
		book, ok := bible.BookByID(match.Result.Book.ID)
		if !ok {
			book = match.Result.Book
		}
		verses = append(verses, quotedVerse{
			book: book, chapter: match.Result.Chapter, number: match.Result.Verse, text: match.Result.Text,
		})
	}
	if len(verses) == 0 {
		return nil
	}

	// bm25 ranks by closeness, not by address; order by address so neighbours sit
	// together, then join consecutive verses into the range that was quoted.
	sort.Slice(verses, func(i, j int) bool {
		if verses[i].book.ID != verses[j].book.ID {
			return verses[i].book.ID < verses[j].book.ID
		}
		if verses[i].chapter != verses[j].chapter {
			return verses[i].chapter < verses[j].chapter
		}
		return verses[i].number < verses[j].number
	})
	var groups [][]quotedVerse
	for _, verse := range verses {
		if n := len(groups); n > 0 {
			last := groups[n-1]
			previous := last[len(last)-1]
			if previous.book.ID == verse.book.ID && previous.chapter == verse.chapter && verse.number == previous.number+1 {
				groups[n-1] = append(last, verse)
				continue
			}
		}
		groups = append(groups, []quotedVerse{verse})
	}

	explicitRefs := make([]scripture.Reference, 0, len(explicit))
	for _, detection := range explicit {
		explicitRefs = append(explicitRefs, detection.Reference)
	}
	pendingRefs := make([]scripture.Reference, 0, len(pending))
	for _, candidate := range pending {
		pendingRefs = append(pendingRefs, candidate.Reference)
	}

	type passage struct {
		reference scripture.Reference
		score     float64
	}
	var passages []passage
	for _, group := range groups {
		texts := make([]string, 0, len(group))
		for _, verse := range group {
			texts = append(texts, verse.text)
		}
		// Score the merged range over its combined words, so a two-verse quote is
		// judged as the passage it is, not as two half-matches.
		score := quoteOverlap(bible.ContentWords(strings.Join(texts, " ")), spoken, spokenSet)
		if score == 0 {
			continue
		}
		reference := scripture.Reference{
			Book:       group[0].book,
			Chapter:    group[0].chapter,
			VerseStart: group[0].number,
			VerseEnd:   group[len(group)-1].number,
		}
		if overlapsAny(reference, explicitRefs) || overlapsAny(reference, pendingRefs) {
			continue
		}
		passages = append(passages, passage{reference: reference, score: score})
	}

	// Strongest quotations first, then the cap, so a crowded chunk still leads
	// with the passages the preacher most clearly said.
	sort.Slice(passages, func(i, j int) bool { return passages[i].score > passages[j].score })
	if len(passages) > maxQuotesPerSegment {
		passages = passages[:maxQuotesPerSegment]
	}

	detections := make([]asr.Detection, 0, len(passages))
	for _, p := range passages {
		detections = append(detections, asr.Detection{
			Reference:  p.reference,
			Raw:        p.reference.String(),
			Method:     MethodSpokenQuote,
			Confidence: p.score,
		})
	}
	return detections
}

// quoteOverlap scores how much of a verse the transcript reproduces: the share
// of its distinctive words that were said, and the longest verbatim run. It
// returns zero when the overlap is too thin to be a quotation.
func quoteOverlap(verseWords, spoken []string, spokenSet map[string]bool) float64 {
	if len(verseWords) == 0 {
		return 0
	}
	matched := 0
	for _, word := range verseWords {
		if spokenSet[word] {
			matched++
		}
	}
	return confidence.QuoteScore(confidence.QuoteEvidence{
		Matched:  matched,
		Coverage: float64(matched) / float64(len(verseWords)),
		Run:      longestRun(verseWords, spoken),
	})
}

// longestRun is the longest stretch of consecutive verse words that appears in
// the same order in the transcript — a word-level longest common substring. A
// verbatim quotation has a long run; a verse whose words are merely scattered
// through the sermon does not.
func longestRun(verse, spoken []string) int {
	best := 0
	previous := make([]int, len(spoken)+1)
	current := make([]int, len(spoken)+1)
	for i := 1; i <= len(verse); i++ {
		for j := 1; j <= len(spoken); j++ {
			if verse[i-1] == spoken[j-1] {
				current[j] = previous[j-1] + 1
				if current[j] > best {
					best = current[j]
				}
			} else {
				current[j] = 0
			}
		}
		previous, current = current, previous
	}
	return best
}

func overlapsAny(reference scripture.Reference, others []scripture.Reference) bool {
	for _, other := range others {
		if overlaps(reference, other) {
			return true
		}
	}
	return false
}
