package confidence

import (
	"testing"

	"zowe-worship/internal/bible"
	"zowe-worship/internal/scripture"
)

func referenceOf(t *testing.T, text string) scripture.Reference {
	t.Helper()

	reference, err := scripture.Parse(text)
	if err != nil {
		t.Fatalf("parse %q: %v", text, err)
	}
	return reference
}

func chapterOf(t *testing.T, bookName string, chapter int) scripture.Reference {
	t.Helper()

	book, ok := bible.LookupBook(bookName)
	if !ok {
		t.Fatalf("lookup %q", bookName)
	}
	return scripture.Reference{Book: book, Chapter: chapter}
}

func TestScoreFollowsPhrasingEvidence(t *testing.T) {
	cases := []struct {
		name     string
		evidence Evidence
		want     float64
	}{
		{"written separator", Evidence{Explicit: true, VerseScoped: true, Separator: true}, ScoreSeparator},
		{"spoken connectors", Evidence{Explicit: true, VerseScoped: true, Connectors: true}, ScoreConnectors},
		{"bare verse numbers", Evidence{Explicit: true, VerseScoped: true}, ScoreBareVerse},
		{"chapter only", Evidence{Explicit: true}, ScoreChapterOnly},
		{"not a reference", Evidence{VerseScoped: true}, 0},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if got := Score(tc.evidence); got != tc.want {
				t.Errorf("score = %v, want %v", got, tc.want)
			}
		})
	}
}

func TestBandOfUsesPrdThresholds(t *testing.T) {
	cases := []struct {
		score float64
		want  string
	}{
		{1, BandHigh},
		{HighThreshold, BandHigh},
		{0.919, BandMedium},
		{MediumThreshold, BandMedium},
		{0.649, BandLow},
		{0, BandLow},
	}
	for _, tc := range cases {
		if got := BandOf(tc.score); got != tc.want {
			t.Errorf("bands(%v) = %q, want %q", tc.score, got, tc.want)
		}
	}
}

func TestEvaluateReadsPhrasingFromTranscript(t *testing.T) {
	john := referenceOf(t, "John 3:16")
	cases := []struct {
		name       string
		transcript string
		reference  scripture.Reference
		want       float64
	}{
		{"written reference", "Turn with me to John 3:16", john, ScoreSeparator},
		{"spaced colon", "John 3 : 16 says", john, ScoreSeparator},
		{"spoken connectors", "John chapter three verse sixteen", john, ScoreConnectors},
		{"bare numbers", "John three sixteen", john, ScoreBareVerse},
		{"chapter only", "Philippians chapter 4", chapterOf(t, "Philippians", 4), ScoreChapterOnly},
		// A different passage's separator must not lift this hearing.
		{"other reference's separator", "Romans 8:28 and John three sixteen", john, ScoreBareVerse},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if got := Evaluate(tc.transcript, tc.reference); got != tc.want {
				t.Errorf("evaluate %q = %v, want %v", tc.transcript, got, tc.want)
			}
		})
	}
}
