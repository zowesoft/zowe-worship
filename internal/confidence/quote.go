package confidence

// A spoken quotation is matched by its words, not by a named reference, so its
// evidence is overlap with the verse rather than phrasing. The score is capped
// below the high band on purpose: a quote is a strong suggestion the operator
// confirms, never something the room is shown unasked. The intent gate already
// blocks auto-projection for quotes; the band agrees with it.
const (
	// MinQuoteWords is the fewest distinctive words a verse must share with the
	// transcript before the match is worth showing. One or two common words are
	// coincidence, not a quotation.
	MinQuoteWords = 4
	// MinQuoteCoverage is the fraction of the verse's distinctive words that must
	// appear in the transcript. Below half, the preacher is talking about the
	// verse's subject rather than quoting it.
	MinQuoteCoverage = 0.5
	// MinQuoteRun is the fewest consecutive verse words the transcript must
	// reproduce in order. A quotation is word-for-word, so its words come out
	// together; a verse whose words merely happen to be scattered through the
	// sermon has no such run and is not a quote, however high its coverage is.
	MinQuoteRun = 4
	// MaxQuoteScore keeps every quotation under HighThreshold however verbatim it
	// looks, so a quote never reads as a heard reference.
	MaxQuoteScore = 0.90
)

// QuoteEvidence is the overlap between one verse and the transcript.
type QuoteEvidence struct {
	// Matched counts the verse's distinctive words found in the transcript.
	Matched int
	// Coverage is Matched over the verse's distinctive word count, in [0, 1].
	Coverage float64
	// Run is the longest stretch of consecutive verse words reproduced in the
	// same order in the transcript — the signature of a verbatim quotation, as
	// opposed to the same words scattered across a sermon.
	Run int
}

// QuoteScore maps quote evidence to a confidence in [0, MaxQuoteScore], or zero
// when the overlap is too thin to be a quotation. BandOf names the result the
// same way it does for an explicit reference.
func QuoteScore(evidence QuoteEvidence) float64 {
	if evidence.Matched < MinQuoteWords || evidence.Coverage < MinQuoteCoverage || evidence.Run < MinQuoteRun {
		return 0
	}
	// Coverage carries most of the score; a long verbatim run and a wide shared
	// vocabulary add the rest, so a word-for-word quotation outranks a verse
	// whose words merely happen to appear here and there.
	score := 0.5 + 0.3*evidence.Coverage
	if evidence.Run >= 5 {
		score += 0.05
	}
	if evidence.Matched >= 8 {
		score += 0.03
	}
	if score > MaxQuoteScore {
		score = MaxQuoteScore
	}
	return score
}
