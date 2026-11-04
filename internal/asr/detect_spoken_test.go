package asr

import (
	"testing"
)

// The Roger benchmark clip says "Philippians chapter 4, verses 6 and 7." The
// plural "verses" and the spoken "and" must both resolve, or the detector
// queues the whole chapter instead of the passage the preacher named.
func TestDetectSpokenVerseRanges(t *testing.T) {
	cases := []struct {
		sentence string
		want     string
	}{
		{"But listen to what the Bible says in Philippians chapter 4, verses 6 and 7.", "Philippians 4:6-7"},
		{"Philippians chapter 4 verses 6 and 7", "Philippians 4:6-7"},
		{"Turn with me to Romans chapter 8 and verse 28.", "Romans 8:28"},
		{"First Corinthians thirteen", "1 Corinthians 13"},
		// A bare chapter mention is a legitimate whole-chapter request.
		{"Turn to Philippians chapter 4", "Philippians 4"},
	}
	for _, testCase := range cases {
		detections := DetectReferences(testCase.sentence)
		if len(detections) == 0 {
			t.Errorf("%q -> no detection, want %q", testCase.sentence, testCase.want)
			continue
		}
		if got := detections[0].Reference.String(); got != testCase.want {
			t.Errorf("%q -> %q, want %q", testCase.sentence, got, testCase.want)
		}
	}
}
