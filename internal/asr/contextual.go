package asr

import (
	"strconv"
	"strings"
)

// VerseMention is a bare "verse N" the speaker used without naming the book or
// chapter - "now read verse twenty-nine". It means nothing on its own, so this
// layer only reports the mention; the resolver in internal/detection maps it
// onto the passage that is established on the wall.
type VerseMention struct {
	Verse int
	Raw   string
}

// maxContextualHints caps how many mentions one hearing produces. A monster
// list of verse calls in thirty seconds is a mishearing, not a passage walk.
const maxContextualHints = 3

// ContextualVerseHints finds bare verse mentions: the word "verse" or "verses"
// followed by a run of number words or digits that settles to exactly one
// number ("verse twenty-nine", "verses 25 to 34" -> 25, "verse 176"). A run
// that cannot settle to one value ("verse twenty-nine and thirty") is not a
// hint, and neither is a verse number past the plausible 1..999 range.
//
// The words "verse" and "chapter" are dropped everywhere else in the pipeline,
// so this scans the raw normalized transcript instead of the final token
// stream.
func ContextualVerseHints(text string) []VerseMention {
	fields := cardinalizeOrdinals(strings.Fields(normalizeSpokenText(text)))
	hints := make([]VerseMention, 0, maxContextualHints)
	seen := make(map[int]bool, len(fields))
	for i, field := range fields {
		if field != "verse" && field != "verses" {
			continue
		}
		runEnd := i + 1
		for runEnd < len(fields) && isNumberishWord(fields[runEnd]) {
			runEnd++
		}
		// A trailing "and" belongs to the next phrase, not to this number run:
		// "verse 16 and verse 17" carries two mentions, not an ambiguous one.
		for runEnd-1 > i && fields[runEnd-1] == "and" {
			runEnd--
		}
		if runEnd == i+1 {
			continue
		}
		verse, ok := composeSingleNumber(fields[i+1 : runEnd])
		if !ok || seen[verse] {
			continue
		}
		seen[verse] = true
		raw := strings.Join(append([]string{field}, fields[i+1:runEnd]...), " ")
		hints = append(hints, VerseMention{Verse: verse, Raw: raw})
		if len(hints) >= maxContextualHints {
			return hints
		}
	}
	return hints
}

// wordOfDigit maps a digit string to its number word, so ordinals folded to
// digits can be spelled back into the vocabulary the number composer needs
// ("ninth" -> "nine"). All values 0-100 are covered by numberWords.
var wordOfDigit = func() map[string]string {
	m := make(map[string]string, len(numberWords)+1)
	for word, value := range numberWords {
		m[strconv.Itoa(value)] = word
	}
	return m
}()

// cardinalizeOrdinals rewrites ordinal words to their cardinal form so the
// existing number-run composition applies to them ("twenty-ninth" -> "twenty
// nine"). Hyphenated compounds are split before folding. This is local to
// the contextual path; the shared explicit detector is left alone.
func cardinalizeOrdinals(fields []string) []string {
	out := make([]string, 0, len(fields)+4)
	for _, field := range fields {
		parts := []string{field}
		if strings.Contains(field, "-") {
			parts = strings.Split(field, "-")
		}
		changed := false
		for i, part := range parts {
			if replacement, ok := ordinalWords[part]; ok {
				parts[i] = wordOfDigit[replacement]
				changed = true
			}
		}
		if changed {
			out = append(out, parts...)
			continue
		}
		out = append(out, field)
	}
	return out
}

func isNumberishWord(field string) bool {
	if _, ok := numberWords[field]; ok {
		return true
	}
	if _, ok := ordinalWords[field]; ok {
		return true
	}
	if allDigits([]string{field}) {
		return true
	}
	switch field {
	case "and", "hundred":
		return true
	}
	parts := strings.Split(field, "-")
	return len(parts) > 1 && (allNumberVocabulary(parts) || allDigits(parts))
}

func composeSingleNumber(run []string) (int, bool) {
	tokens := make([]string, 0, len(run))
	for _, field := range run {
		tokens = append(tokens, tokenizeField(field)...)
	}
	composed := composeNumberRuns(tokens)
	if len(composed) != 1 {
		return 0, false
	}
	return tokenNumber(composed[0])
}
