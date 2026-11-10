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
