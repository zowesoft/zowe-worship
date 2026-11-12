package confidence

import "testing"

func TestQuoteScoreGatesThinOverlap(t *testing.T) {
	cases := []struct {
		name     string
		evidence QuoteEvidence
	}{
		{"too few words", QuoteEvidence{Matched: MinQuoteWords - 1, Coverage: 1, Run: 8}},
		{"too little coverage", QuoteEvidence{Matched: 8, Coverage: MinQuoteCoverage - 0.01, Run: 8}},
		{"too short a run", QuoteEvidence{Matched: 8, Coverage: 1, Run: MinQuoteRun - 1}},
		{"nothing matched", QuoteEvidence{}},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if got := QuoteScore(tc.evidence); got != 0 {
				t.Errorf("QuoteScore = %v, want 0 below the gate", got)
			}
		})
	}
}

func TestQuoteScoreStaysBelowTheHighBand(t *testing.T) {
	// Even a perfect, long, word-for-word quotation must not read as a heard
	// reference: the intent gate blocks it and the band has to agree.
	cases := []QuoteEvidence{
		{Matched: 4, Coverage: 0.5, Run: MinQuoteRun},
		{Matched: 5, Coverage: 0.6, Run: MinQuoteRun},
		{Matched: 8, Coverage: 1, Run: 10},
		{Matched: 30, Coverage: 1, Run: 30},
	}
	for _, evidence := range cases {
		score := QuoteScore(evidence)
		if score <= 0 {
			t.Fatalf("QuoteScore(%+v) = %v, want a positive score", evidence, score)
		}
		if score > MaxQuoteScore {
			t.Errorf("QuoteScore(%+v) = %v, want at most %v", evidence, score, MaxQuoteScore)
		}
		if score >= HighThreshold {
			t.Errorf("QuoteScore(%+v) = %v, want below the high threshold %v", evidence, score, HighThreshold)
		}
		if band := BandOf(score); band != BandMedium {
			t.Errorf("BandOf(%v) = %q, want %q", score, band, BandMedium)
		}
	}
}

func TestQuoteScoreRewardsAVerbatimRun(t *testing.T) {
	scattered := QuoteScore(QuoteEvidence{Matched: 8, Coverage: 1, Run: MinQuoteRun})
	verbatim := QuoteScore(QuoteEvidence{Matched: 8, Coverage: 1, Run: 12})
	if verbatim <= scattered {
		t.Errorf("verbatim run scored %v, want more than scattered %v", verbatim, scattered)
	}
}
