package asr

import (
	"strconv"
	"strings"
)

// numberWords maps spoken number tokens to their value. Tens and hundred are
// represented so a run of tokens such as "one hundred nineteen" can be
// composed into 119.
var numberWords = map[string]int{
	"zero": 0, "one": 1, "two": 2, "three": 3, "four": 4, "five": 5, "six": 6,
	"seven": 7, "eight": 8, "nine": 9, "ten": 10, "eleven": 11, "twelve": 12,
	"thirteen": 13, "fourteen": 14, "fifteen": 15, "sixteen": 16,
	"seventeen": 17, "eighteen": 18, "nineteen": 19, "twenty": 20,
	"thirty": 30, "forty": 40, "fifty": 50, "sixty": 60, "seventy": 70,
	"eighty": 80, "ninety": 90, "hundred": 100,
}

// ordinalWords maps spoken ordinal tokens (used before book names, e.g.
// "first Corinthians") to their digit form. Ranges beyond third are rarely
// book ordinals but still normalized so "twentieth" behaves like a number.
var ordinalWords = map[string]string{
	"first": "1", "second": "2", "third": "3", "fourth": "4", "fifth": "5",
	"sixth": "6", "seventh": "7", "eighth": "8", "ninth": "9", "tenth": "10",
	"eleventh": "11", "twelfth": "12",
	"thirteenth": "13", "fourteenth": "14", "fifteenth": "15",
	"sixteenth": "16", "seventeenth": "17", "eighteenth": "18",
	"nineteenth": "19", "twentieth": "20",
	"thirtieth": "30", "fortieth": "40", "fiftieth": "50",
	"sixtieth": "60", "seventieth": "70", "eightieth": "80",
	"ninetieth": "90", "hundredth": "100",
}

// normalizeSpokenText strips punctuation ASR engines insert, lowercases, and
// keeps only characters the detector token pipeline needs (letters, digits,
// apostrophes, colon, hyphen). A colon always becomes a standalone token so
// "Revelation:" does not swallow the separator ("revelation" : 22 : 21);
// hyphens stay attached because spoken compounds ("twenty-eight") rejoin.
// Unicode punctuation from realistic ASR output ("’", "–", full-width
// quotes) is folded to ASCII equivalents.
func normalizeSpokenText(text string) string {
	var b strings.Builder
	for _, r := range strings.ToLower(text) {
		switch {
		case r >= 'a' && r <= 'z', r >= '0' && r <= '9':
			b.WriteRune(r)
		case r == '\'':
			// "let's" -> "lets": drop the apostrophe entirely.
		case r == '’':
			b.WriteRune(' ')
		case r == ':':
			b.WriteString(" : ")
		case r == '-':
			b.WriteRune('-')
		case r == '–' || r == '—':
			b.WriteRune('-')
		default:
			// Commas, periods, quotes, brackets and any other noise become
			// separator spaces.
			b.WriteRune(' ')
		}
	}
	return b.String()
}

// tokenizeForDetection turns normalized text into detector tokens:
// punctuation runs become separate ":" tokens, hyphenated spoken compounds
// ("twenty-eight") become natural numbers, number words are folded to digits
// ("one hundred nineteen" -> "119"), the ordinals representing book numbers
// become "1"/"2"/"3", and the connector words "chapter"/"verse" are dropped
// because the surrounding numbers carry their meaning.
func tokenizeForDetection(text string) []string {
	fields := strings.Fields(text)
	tokens := make([]string, 0, len(fields)+8)

	for _, field := range fields {
		tokens = append(tokens, tokenizeField(field)...)
	}

	// Number-word composition runs on the full token stream so connector
	// words (chapter, verse, and, of) break tokens apart correctly; the
	// connectors are only removed afterwards.
	composed := composeNumberRuns(tokens)

	filtered := make([]string, 0, len(composed))
	for _, token := range composed {
		switch token {
		case "chapter", "verse", "verses", "chapters":
			continue
		}
		filtered = append(filtered, token)
	}
	return filtered
}

// tokenizeField expands one whitespace-separated field into detector tokens.
func tokenizeField(field string) []string {
	// Ordinals ("first", "21st") fold to plain digits first so later digit
	// handling sees uniform tokens.
	if replacement, ok := ordinalWords[field]; ok {
		field = replacement
	} else if digit, suffixed := stripOrdinalSuffix(field); suffixed {
		field = digit
	}

	// Hyphenated spoken compounds: "twenty-eight" composes to 28.
	parts := strings.Split(field, "-")
	if len(parts) > 1 && allNumberVocabulary(parts) {
		return composeNumberList(parts)
	}

	// Colon-separated verse spans keep their separators.
	if strings.Contains(field, ":") {
		segments := strings.Split(field, ":")
		out := make([]string, 0, 2*len(segments))
		for _, segment := range segments {
			if segment != "" {
				out = append(out, tokenizeField(segment)...)
			}
			out = append(out, ":")
		}
		return out[:len(out)-1]
	}

	// ASR verse ranges written with a dash: "28-30" -> 28, -, 30.
	if len(parts) > 1 && allDigits(parts) {
		out := make([]string, 0, 2*len(parts)-1)
		for _, part := range parts {
			out = append(out, part, "-")
		}
		return out[:len(out)-1]
	}

	return []string{field}
}

func allDigits(parts []string) bool {
	for _, part := range parts {
		if part == "" {
			return false
		}
		for _, r := range part {
			if r < '0' || r > '9' {
				return false
			}
		}
	}
	return true
}

func allNumberVocabulary(parts []string) bool {
	for _, part := range parts {
		if part == "" {
			continue
		}
		if _, ok := numberWords[part]; !ok {
			return false
		}
	}
	return true
}

// composeNumberList resolves a run of number-word tokens into natural
// numbers, following English composition rules: tens take units ("twenty +
// eight" -> 28), "hundred" scales the accumulator ("one hundred" -> 100,
// "one hundred nineteen" -> 119), and completed numbers start anew when a
// following value cannot extend them ("eight twenty eight" -> 8, 28).
func composeNumberList(values []string) []string {
	var out []string
	acc := 0
	hasHundred := false

	flushAcc := func() {
		if acc > 0 {
			out = append(out, strconv.Itoa(acc))
			acc = 0
			hasHundred = false
		}
	}

	for _, value := range values {
		v, ok := numberWords[value]
		if !ok {
			// "and" only ever acts as glue inside a number ("a hundred and
			// five"); anything else never belongs in a numeric run.
			continue
		}
		switch {
		case v == 100:
			if acc > 9 && acc%10 != 0 {
				flushAcc()
			}
			if acc == 0 {
				acc = 1
			}
			acc *= 100
			hasHundred = true
		case v >= 10:
			switch {
			case acc > 9 && acc%10 != 0:
				// A completed number cannot absorb a tens/teens value.
				flushAcc()
				acc = v
			case acc > 0 && !hasHundred:
				// units | tens ("eight twenty...") never merge.
				flushAcc()
				acc = v
			case hasHundred:
				// "one hundred nineteen", "one hundred twenty eight".
				acc += v
			default:
				flushAcc()
				acc = v
			}
		default: // units 0..9
			switch {
			case acc > 9 && acc%10 != 0:
				flushAcc()
				acc = v
			case acc%10 == 0 && acc > 0:
				// Tens ("twenty eight") or hundreds ("one hundred five").
				acc += v
			case acc > 0:
				// unit | unit never merges ("two, five").
				flushAcc()
				acc = v
			default:
				acc = v
			}
		}
	}
	flushAcc()
	return out
}

// composeNumberRuns merges consecutive number-word tokens into natural
// numbers ("one", "hundred", "nineteen" -> "119"). The word "and" inside a
// numeric run acts as glue and disappears.
func composeNumberRuns(tokens []string) []string {
	out := make([]string, 0, len(tokens))
	run := make([]string, 0, 4)

	flush := func() {
		if len(run) == 0 {
			return
		}
		var values []string
		numeric := false
		for _, token := range run {
			if token == "and" {
				continue
			}
			if _, ok := numberWords[token]; ok {
				values = append(values, token)
				numeric = true
			}
		}
		if numeric {
			out = append(out, composeNumberList(values)...)
		} else {
			out = append(out, run...)
		}
		run = run[:0]
	}

	for _, token := range tokens {
		_, isNumberWord := numberWords[token]
		if isNumberWord || token == "and" {
			run = append(run, token)
			continue
		}
		flush()
		out = append(out, token)
	}
	flush()

	return out
}

// stripOrdinalSuffix converts digit tokens with ordinal suffix letters
// ("21st", "3RD") to their numeral form. The boolean reports whether the
// conversion applied.
func stripOrdinalSuffix(token string) (string, bool) {
	digits := 0
	for digits < len(token) && token[digits] >= '0' && token[digits] <= '9' {
		digits++
	}
	if digits == 0 || digits == len(token) {
		return token, false
	}
	suffix := strings.ToLower(token[digits:])
	switch suffix {
	case "st", "nd", "rd", "th":
		return token[:digits], true
	}
	return "", false
}
