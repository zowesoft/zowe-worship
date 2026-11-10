package confidence

import "sort"

// Observation is one detection scored against labeled audio: Score is what
// Evaluate produced, and Matched reports whether the reference was actually
// spoken in the clip.
type Observation struct {
	Score   float64
	Matched bool
}

// Point is what a single threshold would have produced: the detections it
// keeps, how many of those were real, and the resulting precision and recall.
type Point struct {
	Threshold float64
	Kept      int
	Correct   int
	False     int
	Precision float64
	Recall    float64
}

// Sweep reports what each threshold would keep. expected is the number of
// references actually spoken, including ones the engine never detected, so
// recall reflects the misses a threshold cannot recover.
func Sweep(observations []Observation, expected int, thresholds []float64) []Point {
	points := make([]Point, 0, len(thresholds))
	for _, threshold := range thresholds {
		point := Point{Threshold: threshold}
		for _, observation := range observations {
			if observation.Score < threshold {
				continue
			}
			point.Kept++
			if observation.Matched {
				point.Correct++
			} else {
				point.False++
			}
		}
		if point.Kept > 0 {
			point.Precision = float64(point.Correct) / float64(point.Kept)
		}
		if expected > 0 {
			point.Recall = float64(point.Correct) / float64(expected)
		}
		points = append(points, point)
	}
	return points
}

// DefaultThresholds is the sweep the benchmark prints: a low-to-high grid around
// the PRD's band edges, plus the exact edges and the scores the scorer can
// produce, so the effect of moving a band is visible on labeled audio.
func DefaultThresholds() []float64 {
	set := make(map[float64]bool)
	for hundredths := 60; hundredths <= 99; hundredths += 2 {
		set[float64(hundredths)/100] = true
	}
	for _, edge := range []float64{MediumThreshold, HighThreshold, ScoreChapterOnly, ScoreBareVerse} {
		set[edge] = true
	}
	thresholds := make([]float64, 0, len(set))
	for threshold := range set {
		thresholds = append(thresholds, threshold)
	}
	sort.Float64s(thresholds)
	return thresholds
}
