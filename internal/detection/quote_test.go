package detection

import (
	"testing"

	"zopresent/internal/asr"
	"zopresent/internal/bible"
	"zopresent/internal/confidence"
)

const philippiansQuote = "be careful for nothing but in every thing by prayer and supplication with thanksgiving " +
	"let your requests be made known unto God and the peace of God which passeth all understanding " +
	"shall keep your hearts and minds through Christ Jesus"

func quoteMatchOf(bookID, chapter, verse int, text string) bible.QuoteMatch {
	book, _ := bible.BookByID(bookID)
	return bible.QuoteMatch{Result: bible.SearchResult{Book: book, Chapter: chapter, Verse: verse, Text: text}}
}

func philippiansMatches() []bible.QuoteMatch {
	return []bible.QuoteMatch{
		quoteMatchOf(50, 4, 6, "Be careful for nothing; but in every thing by prayer and supplication with thanksgiving let your requests be made known unto God."),
		quoteMatchOf(50, 4, 7, "And the peace of God, which passeth all understanding, shall keep your hearts and minds through Christ Jesus."),
		quoteMatchOf(43, 3, 16, "For God so loved the world, that he gave his only begotten Son, that whosoever believeth in him should not perish, but have everlasting life."),
	}
}

func TestDetectQuotesMergesNeighboursAndTagsTheMethod(t *testing.T) {
	detections := DetectQuotes(philippiansQuote, philippiansMatches(), nil, nil)
	if len(detections) != 1 {
		t.Fatalf("detections = %d, want the two quoted verses merged into one passage", len(detections))
	}
	got := detections[0]
	if got.Reference.String() != "Philippians 4:6-7" {
		t.Errorf("reference = %q, want Philippians 4:6-7", got.Reference.String())
	}
	if got.Method != MethodSpokenQuote {
		t.Errorf("method = %q, want %q", got.Method, MethodSpokenQuote)
	}
	if got.Confidence <= 0 || got.Confidence > confidence.MaxQuoteScore {
		t.Errorf("confidence = %v, want a quote score in (0, %v]", got.Confidence, confidence.MaxQuoteScore)
	}
	if band := confidence.BandOf(got.Confidence); band != confidence.BandMedium {
		t.Errorf("band = %q, want %q", band, confidence.BandMedium)
	}
}

func TestDetectQuotesSkipsPassagesAnExplicitReferenceCovers(t *testing.T) {
	explicit := []asr.Detection{detectionOf(t, "Philippians 4:6")}
	if detections := DetectQuotes(philippiansQuote, philippiansMatches(), explicit, nil); len(detections) != 0 {
		t.Fatalf("detections = %v, want none: the explicit reference already covers the quote", detections)
	}
}

func TestDetectQuotesSkipsPassagesAlreadyPending(t *testing.T) {
	pending := []Candidate{{Reference: detectionOf(t, "Philippians 4:6-7").Reference, Finalized: true}}
	if detections := DetectQuotes(philippiansQuote, philippiansMatches(), nil, pending); len(detections) != 0 {
		t.Fatalf("detections = %v, want none: the passage is already queued", detections)
	}
}

func TestDetectQuotesCapsPassagesPerSegment(t *testing.T) {
	// Five fully-quoted verses in five books: each clears the gate on its own and
	// none merge, so the cap is what keeps the queue to three.
	words := []string{
		"alpha beta gamma delta epsilon",
		"zeta eta theta iota kappa",
		"lambda mu nu xi omicron",
		"pi rho sigma tau upsilon",
		"phi chi psi omega aleph",
	}
	transcript := ""
	var matches []bible.QuoteMatch
	for i, text := range words {
		transcript += text + " "
		matches = append(matches, quoteMatchOf(i+1, 1, 1, text))
	}
	detections := DetectQuotes(transcript, matches, nil, nil)
	if len(detections) != maxQuotesPerSegment {
		t.Fatalf("detections = %d, want the cap of %d", len(detections), maxQuotesPerSegment)
	}
}

func TestQueueKeepsAQuoteScoreAndUpgradesOnAnExplicitRehearing(t *testing.T) {
	queue := newTestQueue(t)
	reference := detectionOf(t, "Philippians 4:6-7").Reference
	quote := asr.Detection{Reference: reference, Raw: reference.String(), Method: MethodSpokenQuote, Confidence: 0.88}

	queue.BeginSegment()
	settled, err := queue.Finalize(philippiansQuote, []asr.Detection{quote}, "service-1")
	if err != nil {
		t.Fatalf("finalize quote: %v", err)
	}
	if len(settled.Settled) != 1 {
		t.Fatalf("settled = %d, want 1", len(settled.Settled))
	}
	card := settled.Settled[0]
	if card.Method != MethodSpokenQuote || card.Explanation != ExplanationSpokenQuote {
		t.Errorf("method/explanation = %q/%q, want the spoken-quote pair", card.Method, card.Explanation)
	}
	// The quote's own score survives; the explicit phrasing scale would have
	// returned a different number for this unnamed reference.
	if card.Confidence != 0.88 {
		t.Errorf("confidence = %v, want the quote score 0.88", card.Confidence)
	}

	// The preacher later names the same passage: the stronger explicit hearing
	// upgrades the card instead of leaving the weaker quote label or zeroing it.
	queue.BeginSegment()
	if _, err := queue.Finalize("turn to Philippians 4:6-7", []asr.Detection{detectionOf(t, "Philippians 4:6-7")}, "service-1"); err != nil {
		t.Fatalf("finalize explicit: %v", err)
	}
	pending := queue.List("service-1")
	if len(pending) != 1 {
		t.Fatalf("pending = %d, want the one upgraded card", len(pending))
	}
	upgraded := pending[0]
	if upgraded.Method != MethodExplicitReference {
		t.Errorf("method = %q, want it upgraded to %q", upgraded.Method, MethodExplicitReference)
	}
	if upgraded.Confidence != confidence.ScoreSeparator {
		t.Errorf("confidence = %v, want the explicit separator score %v", upgraded.Confidence, confidence.ScoreSeparator)
	}
	if upgraded.Repeats != 1 {
		t.Errorf("repeats = %d, want the re-hearing folded as a repeat", upgraded.Repeats)
	}
}

func TestLongestRunFindsTheVerbatimStretch(t *testing.T) {
	verse := []string{"peace", "god", "passeth", "understanding"}
	spoken := []string{"keep", "peace", "god", "passeth", "understanding", "hearts"}
	if got := longestRun(verse, spoken); got != 4 {
		t.Fatalf("longestRun = %d, want 4", got)
	}
	scattered := []string{"understanding", "keep", "god", "hearts", "peace"}
	if got := longestRun(verse, scattered); got != 1 {
		t.Fatalf("longestRun = %d, want 1 for scattered words", got)
	}
}
