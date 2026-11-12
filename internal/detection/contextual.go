package detection

import (
	"fmt"

	"zopresent/internal/asr"
	"zopresent/internal/scripture"
)

// contextualConfidence is deliberate, not calibrated: a contextual match is
// the weakest evidence the queue holds, so it always bands "low" and can
// never pass the confidence gate. The intent gate blocks it first anyway.
const contextualConfidence = 0.5

// maxContextualPerSegment is the same hearing-window cap the quote path
// uses. Three verse calls in thirty seconds is a sermon walk, not a
// mishearing.
const maxContextualPerSegment = 3

// DetectContextualReferences resolves the bare "verse N" mentions in a
// transcript against the passage that is established on the wall: the same
// book and chapter, the spoken verse. A verse that does not exist in that
// chapter is dropped, and a mention already covered by an explicit reference
// from the same transcript is skipped — the passage the preacher named
// outright is stronger evidence than an implied one. verseExists reports
// whether the chapter holds the verse; the caller owns Bible access, so this
// layer stays pure and testable.
//
// Contextual references are queued for confirmation like everything else, but
// they can never auto-project: their Method is not the explicit-reference
// method, and the policy intent gate requires that.
func DetectContextualReferences(transcript string, established scripture.Reference, explicit []asr.Detection, verseExists func(verse int) bool) []asr.Detection {
	if verseExists == nil || established.Chapter <= 0 || established.Book.ID <= 0 {
		return nil
	}
	hints := asr.ContextualVerseHints(transcript)
	if len(hints) == 0 {
		return nil
	}
	detections := make([]asr.Detection, 0, len(hints))
	seen := make(map[string]bool, len(hints))
	for _, hint := range hints {
		if explicitCovers(explicit, established, hint.Verse) {
			continue
		}
		ref := scripture.Reference{
			Book:       established.Book,
			Chapter:    established.Chapter,
			VerseStart: hint.Verse,
			VerseEnd:   hint.Verse,
		}
		key := ref.String()
		if seen[key] || !verseExists(hint.Verse) {
			continue
		}
		seen[key] = true
		detections = append(detections, asr.Detection{
			Reference:  ref,
			Raw:        fmt.Sprintf("%s (same passage as the wall)", hint.Raw),
			Method:     MethodContextualReference,
			Confidence: contextualConfidence,
		})
		if len(detections) >= maxContextualPerSegment {
			break
		}
	}
	return detections
}

// explicitCovers reports whether an explicit reference from the same
// transcript already resolves the bare mention's verse, using whole-chapter
// semantics for a book-and-chapter-only reference.
func explicitCovers(explicit []asr.Detection, established scripture.Reference, verse int) bool {
	for _, detection := range explicit {
		ref := detection.Reference
		if ref.Book.ID != established.Book.ID || ref.Chapter != established.Chapter {
			continue
		}
		start, end := ref.VerseStart, ref.VerseEnd
		if start <= 0 {
			return true
		}
		if end <= 0 {
			end = start
		}
		if verse >= start && verse <= end {
			return true
		}
	}
	return false
}
