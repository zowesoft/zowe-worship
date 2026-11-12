package asr

import "testing"

func TestContextualVerseHintsFindsMentions(t *testing.T) {
	cases := []struct {
		text string
		want []int
	}{
		{"now read verse twenty-nine", []int{29}},
		{"now read verse twenty nine", []int{29}},
		{"read verse twenty-ninth", []int{29}},
		{"read verse 29", []int{29}},
		{"verses twenty six", []int{26}},
		{"let us turn to verse 16", []int{16}},
		{"please read verse one hundred and nineteen", []int{119}},
		{"chapter eight verse 28", []int{28}},
		{"now verse 16 and verse 17", []int{16, 17}},
		{"verse 25", []int{25}},
		{"how about verse 26, turn to verse 27", []int{26, 27}},
	}
	for _, tc := range cases {
		t.Run(tc.text, func(t *testing.T) {
			got := ContextualVerseHints(tc.text)
			if len(got) != len(tc.want) {
				t.Fatalf("ContextualVerseHints(%q) = %+v, want verses %v", tc.text, got, tc.want)
			}
			for i, want := range tc.want {
				if got[i].Verse != want {
					t.Fatalf("ContextualVerseHints(%q)[%d] = %d, want %d", tc.text, i, got[i].Verse, want)
				}
			}
		})
	}
}

func TestContextualVerseHintsSkipsUnusableMentions(t *testing.T) {
	cases := []string{
		"",
		"read the next chapter",
		"verse",
		"verse and read on",
		"verse 20 and 21",                 // a run settling to two numbers is ambiguous
		"verse 1500",                      // past the plausible verse range
		"in verse twenty nine and thirty", // settles to two numbers
		"verse ts01",                      // not a number
		"read the verses aloud",
	}
	for _, tc := range cases {
		t.Run(tc, func(t *testing.T) {
			if got := ContextualVerseHints(tc); len(got) != 0 {
				t.Fatalf("ContextualVerseHints(%q) = %+v, want none", tc, got)
			}
		})
	}
}

func TestContextualVerseHintsCapsAndDedupes(t *testing.T) {
	// A single repeated mention folds to one hint.
	if got := ContextualVerseHints("verse 16 verse 16 verse 16"); len(got) != 1 || got[0].Verse != 16 {
		t.Fatalf("dedupe: got %+v, want one hint for verse 16", got)
	}
	// A long run of mentions is capped rather than flooding the queue.
	got := ContextualVerseHints("verse 1, verse 2, verse 3, verse 4, verse 5")
	if len(got) != maxContextualHints {
		t.Fatalf("cap: got %d hints, want %d", len(got), maxContextualHints)
	}
}
