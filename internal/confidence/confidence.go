// Package confidence turns the evidence behind a scripture detection into a
// score and a policy band. The queue uses it so the panel can show how sure the
// detector is about a hearing instead of a flat rule match.
//
// The thresholds are the PRD's starting tuning values, not measured accuracy:
// the rule-based detector has not been scored against labeled audio yet, so a
// high band means "the phrasing was heard cleanly", not "this is verified".
package confidence

import (
	"regexp"
	"strconv"
	"strings"

	"zopresent/internal/scripture"
)

// Bands name the policy outcome a score falls into. Only the confirmation queue
// and the panel consume them today; auto-projection stays off until the detector
// is measured against labeled audio.
const (
	BandHigh   = "high"
	BandMedium = "medium"
	BandLow    = "low"
)

// Thresholds are the PRD's illustrative band edges.
const (
	HighThreshold   = 0.92
	MediumThreshold = 0.65
)

// Evidence is what the detector's match tells us about one hearing.
type Evidence struct {
	// Explicit reports a book name followed by a chapter. Only explicit
	// references reach the queue today.
	Explicit bool
	// VerseScoped reports a named verse or verse range, so the passage is
	// narrow enough to project.
	VerseScoped bool
	// Connectors reports spoken "chapter"/"verse" words, which make a
	// numbered match far less likely to be coincidence than bare numbers.
	Connectors bool
	// Separator reports a written chapter:verse form such as "3:16", the
	// phrasing speech engines transcribe most reliably.
	Separator bool
}

// Scores per phrasing. A chapter-only hearing ranks lower because the operator
// still has to narrow it before it is usable; a bare "John three sixteen" ranks
// lower than a spoken or written separator because two numbers in a row are the
// form most easily produced by coincidence.
const (
	ScoreSeparator   = 0.96
	ScoreConnectors  = 0.93
	ScoreBareVerse   = 0.86
	ScoreChapterOnly = 0.72
)

// Score maps the evidence to a confidence value in [0, 1].
func Score(evidence Evidence) float64 {
	switch {
	case !evidence.Explicit:
		return 0
	case !evidence.VerseScoped:
		return ScoreChapterOnly
	case evidence.Separator:
		return ScoreSeparator
	case evidence.Connectors:
		return ScoreConnectors
	default:
		return ScoreBareVerse
	}
}

// BandOf names the band a score falls into.
func BandOf(score float64) string {
	switch {
	case score >= HighThreshold:
		return BandHigh
	case score >= MediumThreshold:
		return BandMedium
	default:
		return BandLow
	}
}

// colonReference finds a written chapter:verse pair in the transcript.
var colonReference = regexp.MustCompile(`\b([0-9]{1,3})\s*:\s*([0-9]{1,3})\b`)

// Evaluate reads the phrasing evidence for one reference out of the transcript
// the detector matched it in. It is deliberately coarse: the segment is short
// and the operator reads the transcript beside the score, so a connector word
// anywhere in the segment counts.
func Evaluate(transcript string, reference scripture.Reference) float64 {
	evidence := Evidence{
		Explicit:    reference.Book.ID > 0 && reference.Chapter > 0,
		VerseScoped: reference.VerseStart > 0,
		Connectors:  hasWord(transcript, "chapter") || hasWord(transcript, "verse") || hasWord(transcript, "verses"),
	}
	if evidence.VerseScoped {
		for _, match := range colonReference.FindAllStringSubmatch(transcript, -1) {
			chapter, _ := strconv.Atoi(match[1])
			verse, _ := strconv.Atoi(match[2])
			if chapter == reference.Chapter && verse == reference.VerseStart {
				evidence.Separator = true
				break
			}
		}
	}
	return Score(evidence)
}

func hasWord(text, word string) bool {
	return strings.Contains(strings.ToLower(text), word)
}
