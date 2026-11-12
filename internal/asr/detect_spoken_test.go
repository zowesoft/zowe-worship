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

// A preacher rarely says a reference cleanly: they hedge the book ("Isaiah, I
// think it's chapter 41"), pad the verse ("from verse 25 to verse 34", "around
// verses 35 to 41"), or point at a single verse indirectly ("especially the
// first verse"). Each of these must still resolve to the passage named, not
// collapse to a bare chapter or vanish.
func TestDetectHedgedAndRangedSpeech(t *testing.T) {
	cases := []struct {
		sentence string
		want     string
	}{
		{"And Jesus gave us another powerful lesson in Matthew 6 from verse 25 to verse 34.", "Matthew 6:25-34"},
		{"I believe it's Mark chapter 4 around verses 35 to 41.", "Mark 4:35-41"},
		{"There's also that scripture in Isaiah. I think it's chapter 41, verse 10.", "Isaiah 41:10"},
		{"Psalm 23, especially the first verse, reminds us that the Lord is our shepherd.", "Psalms 23:1"},
		{"Read John 3 from verse 16 through verse 18.", "John 3:16-18"},
		{"Turn to Genesis 1 about verse 1 to verse 3.", "Genesis 1:1-3"},
		{"Maybe it's Romans chapter 8, probably verse 28.", "Romans 8:28"},
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

// The whole sermon sample, in order: every reference the preacher names must
// queue, so the panel fills with the full set rather than only the last one.
func TestDetectWholeSermonSample(t *testing.T) {
	text := "But listen to what the Bible says in Philippians 4, verses 6 and 7. " +
		"And Jesus gave us another powerful lesson in Matthew 6 from verse 25 to verse 34. " +
		"I believe it's Mark chapter 4 around verses 35 to 41. " +
		"There's also that scripture in Isaiah. I think it's chapter 41, verse 10. " +
		"And let's not forget 1 Peter 5, verse 7. " +
		"Psalm 23, especially the first verse, reminds us that the Lord is our shepherd. " +
		"And Romans 8 verse 28 says that all things work together for good."
	want := []string{
		"Philippians 4:6-7", "Matthew 6:25-34", "Mark 4:35-41", "Isaiah 41:10",
		"1 Peter 5:7", "Psalms 23:1", "Romans 8:28",
	}
	detections := DetectReferences(text)
	got := make([]string, 0, len(detections))
	for _, d := range detections {
		got = append(got, d.Reference.String())
	}
	if len(got) != len(want) {
		t.Fatalf("got %v, want %v", got, want)
	}
	for i := range want {
		if got[i] != want[i] {
			t.Fatalf("got %v, want %v", got, want)
		}
	}
}
