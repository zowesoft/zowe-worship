package confidence

import (
	"sort"
	"testing"
)

func TestSweepCountsKeptDetectionsPerThreshold(t *testing.T) {
	observations := []Observation{
		{Score: ScoreSeparator, Matched: true},
		{Score: ScoreConnectors, Matched: true},
		{Score: ScoreBareVerse, Matched: false},
		{Score: ScoreChapterOnly, Matched: true},
	}
	// Five references were spoken; one was never detected at all.
	const expected = 5

	points := Sweep(observations, expected, []float64{0.99, ScoreSeparator, HighThreshold, ScoreBareVerse, 0.70})
	if len(points) != 5 {
		t.Fatalf("points = %d, want one per threshold", len(points))
	}

	assertPoint := func(index int, kept, correct, falsePositives int, precision, recall float64) {
		t.Helper()
		point := points[index]
		if point.Kept != kept || point.Correct != correct || point.False != falsePositives {
			t.Fatalf("point %d = kept %d correct %d false %d, want %d/%d/%d",
				index, point.Kept, point.Correct, point.False, kept, correct, falsePositives)
		}
		if !closeEnough(point.Precision, precision) || !closeEnough(point.Recall, recall) {
			t.Fatalf("point %d precision/recall = %.3f/%.3f, want %.3f/%.3f",
				index, point.Precision, point.Recall, precision, recall)
		}
	}

	assertPoint(0, 0, 0, 0, 0, 0)         // 0.99 keeps nothing
	assertPoint(1, 1, 1, 0, 1, 0.2)       // only the written form
	assertPoint(2, 2, 2, 0, 1, 0.4)       // the PRD's high edge
	assertPoint(3, 3, 2, 1, 2.0/3.0, 0.4) // bare numbers add a false positive
	assertPoint(4, 4, 3, 1, 0.75, 0.6)    // everything, including the low form
}

func TestSweepRecallNeverRisesWithTheThreshold(t *testing.T) {
	observations := []Observation{
		{Score: ScoreSeparator, Matched: true},
		{Score: ScoreBareVerse, Matched: true},
		{Score: ScoreChapterOnly, Matched: false},
	}
	points := Sweep(observations, 4, DefaultThresholds())
	for i := 1; i < len(points); i++ {
		if points[i].Recall > points[i-1].Recall {
			t.Fatalf("recall rose from %.3f to %.3f at threshold %.2f",
				points[i-1].Recall, points[i].Recall, points[i].Threshold)
		}
		if points[i].Kept > points[i-1].Kept {
			t.Fatalf("kept detections rose at threshold %.2f", points[i].Threshold)
		}
	}
}

func TestSweepWithoutObservationsKeepsZeroes(t *testing.T) {
	points := Sweep(nil, 3, []float64{HighThreshold})
	if len(points) != 1 {
		t.Fatalf("points = %d, want 1", len(points))
	}
	if points[0].Kept != 0 || points[0].Precision != 0 || points[0].Recall != 0 {
		t.Fatalf("point = %+v, want zeroes", points[0])
	}
}

func TestRecommendPicksLowestThresholdMeetingTheProjectionBar(t *testing.T) {
	points := Sweep([]Observation{
		{Score: ScoreSeparator, Matched: true},
		{Score: ScoreConnectors, Matched: true},
		{Score: ScoreBareVerse, Matched: false},
		{Score: ScoreChapterOnly, Matched: true},
	}, 4, []float64{0.99, ScoreSeparator, ScoreConnectors, ScoreBareVerse, ScoreChapterOnly})

	best, ok := Recommend(points)
	if !ok {
		t.Fatalf("recommendation = %+v, want a threshold meeting the bar", best)
	}
	// 0.86 admits a false positive and misses 99% precision, so 0.93 is the
	// lowest edge that still keeps every detection it keeps correct.
	if !closeEnough(best.Threshold, ScoreConnectors) {
		t.Fatalf("recommended threshold = %.2f, want %.2f", best.Threshold, ScoreConnectors)
	}
	if best.Kept != 2 || !closeEnough(best.Precision, 1) || !closeEnough(best.Recall, 0.5) {
		t.Fatalf("recommended point = %+v, want kept 2 at precision 1 recall 0.5", best)
	}
}

func TestRecommendFallsBackToBestPrecisionWhenTheBarIsMissed(t *testing.T) {
	points := Sweep([]Observation{
		{Score: ScoreSeparator, Matched: false},
		{Score: ScoreBareVerse, Matched: false},
	}, 2, []float64{ScoreSeparator, ScoreBareVerse})

	best, ok := Recommend(points)
	if ok {
		t.Fatal("recommendation claimed the precision bar was met")
	}
	// Both keep one false detection; the lower threshold is the same answer and
	// leaves more room to move.
	if !closeEnough(best.Threshold, ScoreBareVerse) || best.Kept != 2 {
		t.Fatalf("fallback = %+v, want threshold %.2f keeping both", best, ScoreBareVerse)
	}
}

func TestRecommendReportsNothingWhenNoDetectionWasKept(t *testing.T) {
	if best, ok := Recommend(Sweep(nil, 3, DefaultThresholds())); ok || best.Kept != 0 {
		t.Fatalf("recommendation = %+v (ok=%v), want zero point and false", best, ok)
	}
}

func TestDefaultThresholdsIncludeTheBandEdgesInOrder(t *testing.T) {
	thresholds := DefaultThresholds()
	if !sort.Float64sAreSorted(thresholds) {
		t.Fatal("thresholds are not sorted ascending")
	}
	want := map[float64]bool{MediumThreshold: false, HighThreshold: false, ScoreBareVerse: false, ScoreChapterOnly: false}
	for _, threshold := range thresholds {
		if _, ok := want[threshold]; ok {
			want[threshold] = true
		}
	}
	for threshold, found := range want {
		if !found {
			t.Errorf("threshold %.2f missing from the sweep", threshold)
		}
	}
}

func closeEnough(got, want float64) bool {
	const tolerance = 1e-9
	delta := got - want
	return delta < tolerance && delta > -tolerance
}
