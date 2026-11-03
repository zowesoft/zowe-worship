package asr

import (
	"reflect"
	"strings"
	"testing"
)

func TestDetectReferencesSpokenForms(t *testing.T) {
	cases := []struct {
		name string
		text string
		want []string // expected reference strings, in detection order
	}{
		{
			name: "written-whisper-form",
			text: "Let us turn to Romans 8:28 and see what Paul says.",
			want: []string{"Romans 8:28"},
		},
		{
			name: "spoken-chapter-verse-words",
			text: "Tonight we opened at Romans chapter eight verse twenty eight.",
			want: []string{"Romans 8:28"},
		},
		{
			name: "ordinal-book-prefix",
			text: "First Corinthians thirteen is where Paul teaches.",
			want: []string{"1 Corinthians 13"},
		},
		{
			name: "book-of-filler",
			text: "turn with me to the book of Psalms, chapter twenty three",
			want: []string{"Psalms 23"},
		},
		{
			name: "bare-spoken-number",
			text: "John three sixteen says the same thing.",
			want: []string{"John 3:16"},
		},
		{
			name: "verse-range-dash",
			text: "Romans 8:28-30 slides by.",
			want: []string{"Romans 8:28-30"},
		},
		{
			name: "hundred-verse",
			text: "Psalm one hundred nineteen verse one hundred five.",
			want: []string{"Psalms 119:105"},
		},
		{
			name: "multiword-book",
			text: "remember Song of Solomon chapter three.",
			want: []string{"Song of Solomon 3"},
		},
		{
			name: "second-timothy-verse",
			text: "2 Timothy four two says preach the word.",
			want: []string{"2 Timothy 4:2"},
		},
		{
			name: "no-book-no-detection",
			text: "Read verse 12 from the handout and the 10 commandments.",
			want: nil,
		},
		{
			name: "invalid-chapter-dropped",
			text: "Romans ninety nine twelve is not real.",
			want: nil,
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			detections := DetectReferences(tc.text)
			got := make([]string, 0, len(detections))
			for _, d := range detections {
				got = append(got, d.Reference.String())
			}
			if len(got) == 0 && len(tc.want) == 0 {
				return
			}
			if !reflect.DeepEqual(got, tc.want) {
				t.Fatalf("got %v, want %v (tokens debug: %v)", got, tc.want,
					tokenizeForDetection(normalizeSpokenText(tc.text)))
			}
		})
	}
}

func TestComposeNumberRuns(t *testing.T) {
	cases := []struct {
		in   string
		want []string
	}{
		{"one hundred nineteen", []string{"119"}},
		{"twenty eight", []string{"28"}},
		{"one hundred five", []string{"105"}},
		{"two", []string{"2"}},
		{"eight twenty eight", []string{"8", "28"}},
	}
	for _, tc := range cases {
		tokens := composeNumberRuns(strings.Fields(tc.in))
		if !reflect.DeepEqual(tokens, tc.want) {
			t.Errorf("compose %q -> %v, want %v", tc.in, tokens, tc.want)
		}
	}
}
