package bible

import (
	"context"
	"fmt"
	"strings"
	"unicode"
)

// QuoteMatch is one verse the full-text index ranked against spoken words.
// Score is the raw bm25 rank — lower is a closer match — kept so the detection
// layer can break ties; bm25 is a ranking, not a probability, so the confidence
// a quote shows is computed from word overlap, not from this number.
type QuoteMatch struct {
	Result SearchResult
	Score  float64
}

// quoteStopWords are the words a preacher and a verse share by grammar rather
// than by meaning. Left in, they let almost any verse "match" a long transcript
// and bury the distinctive words that actually identify a quotation. Meaningful
// nouns and verbs stay out of this list even when common ("god", "prayer"),
// because rarity is bm25's job, not the tokenizer's.
var quoteStopWords = map[string]bool{
	"the": true, "a": true, "an": true, "and": true, "or": true, "but": true,
	"of": true, "to": true, "in": true, "on": true, "for": true, "with": true,
	"by": true, "from": true, "into": true, "unto": true, "upon": true, "at": true,
	"is": true, "are": true, "was": true, "were": true, "be": true, "been": true,
	"being": true, "am": true, "it": true, "its": true, "this": true, "that": true,
	"these": true, "those": true, "he": true, "she": true, "they": true, "them": true,
	"his": true, "her": true, "their": true, "him": true, "you": true, "your": true,
	"yours": true, "ye": true, "i": true, "me": true, "my": true, "we": true, "our": true,
	"us": true, "who": true, "whom": true, "which": true, "what": true, "when": true,
	"where": true, "why": true, "how": true, "not": true, "no": true, "nor": true,
	"so": true, "as": true, "if": true, "then": true, "than": true, "there": true,
	"here": true, "shall": true, "will": true, "would": true, "should": true,
	"could": true, "may": true, "might": true, "must": true, "can": true, "do": true,
	"does": true, "did": true, "done": true, "have": true, "has": true, "had": true,
	"said": true, "say": true, "says": true, "also": true, "more": true, "most": true,
	"some": true, "any": true, "all": true, "both": true, "such": true, "only": true,
	"own": true, "same": true, "too": true, "very": true, "just": true, "now": true,
	"up": true, "out": true, "off": true, "over": true, "under": true, "again": true,
	"once": true, "because": true, "while": true, "about": true, "against": true,
	"between": true, "through": true, "during": true, "before": true, "after": true,
	"above": true, "below": true, "each": true, "other": true, "another": true,
}

// disfluencies are the filler words a speaker inserts mid-quote ("uh", "um").
// They carry no meaning and appear in no verse, so the same vocabulary drops
// them from both the index query and the overlap measure; otherwise a single
// "uh" between two verse words would break the verbatim run and a word-for-word
// quotation would stop matching. A real scripture word ("ah") is deliberately
// left out so stripping never removes meaning.
var disfluencies = map[string]bool{
	"uh": true, "um": true, "er": true, "erm": true, "hmm": true,
	"mm": true, "uhh": true, "umm": true, "uhuh": true, "ahem": true,
}

// ContentWords lowercases text and returns the words worth matching, in order,
// with grammar words, disfluencies, punctuation and bare numbers dropped. The quote path uses
// this one vocabulary both to build the index query and to measure how much of a
// verse the preacher said, so the two never disagree about what counts as a word.
// Numbers are dropped because in a transcript they are almost always a spoken
// reference's chapter and verse, which the explicit detector already handles.
func ContentWords(text string) []string {
	var normalized strings.Builder
	for _, r := range strings.ToLower(text) {
		if unicode.IsLetter(r) || unicode.IsDigit(r) {
			normalized.WriteRune(r)
		} else {
			normalized.WriteByte(' ')
		}
	}

	words := make([]string, 0, 16)
	for _, word := range strings.Fields(normalized.String()) {
		if quoteStopWords[word] || disfluencies[word] || isAllDigits(word) {
			continue
		}
		words = append(words, word)
	}
	return words
}

func isAllDigits(word string) bool {
	for _, r := range word {
		if r < '0' || r > '9' {
			return false
		}
	}
	return len(word) > 0
}

// MatchQuote ranks the verses of a translation against the distinctive words of
// spoken text, for detection's spoken-quote path: the preacher quoted a verse
// without naming it. It returns at most limit verses, closest bm25 match first.
// A transcript with nothing but grammar words yields no matches rather than an
// error — there is simply nothing to quote.
func (r *SQLiteRepository) MatchQuote(ctx context.Context, translationID, text string, limit int) ([]QuoteMatch, error) {
	words := ContentWords(text)
	if len(words) == 0 {
		return nil, nil
	}
	// Unique terms keep the OR query small; bm25 still weights each by rarity.
	seen := make(map[string]bool, len(words))
	terms := make([]string, 0, len(words))
	for _, word := range words {
		if seen[word] {
			continue
		}
		seen[word] = true
		terms = append(terms, quoteFTSTerm(word))
	}

	if limit <= 0 {
		limit = defaultSearchLimit
	}
	if limit > maxSearchLimit {
		limit = maxSearchLimit
	}

	packageDB, err := r.openPackage(ctx, translationID)
	if err != nil {
		return nil, err
	}
	defer packageDB.Close()

	// Exact terms, not prefixes: a quotation is the words themselves, and a
	// prefix would let "love" stand in for "loved" and credit the preacher with
	// a word they did not say. bm25 floats the verses carrying the most (and
	// rarest) of the spoken words to the top.
	q := `SELECT b.id, b.name, b.abbreviation, b.testament, v.chapter, v.verse, v.text,
		bm25(verses_fts) AS score
		FROM verses_fts
		JOIN verses v ON v.id = verses_fts.rowid
		JOIN books b ON b.id = v.book_id
		WHERE verses_fts MATCH ?
		ORDER BY score, b.id, v.chapter, v.verse
		LIMIT ?`

	rows, err := packageDB.QueryContext(ctx, q, strings.Join(terms, " OR "), limit)
	if err != nil {
		return nil, fmt.Errorf("match quote: %w", err)
	}
	defer rows.Close()

	var matches []QuoteMatch
	for rows.Next() {
		var match QuoteMatch
		var testament string
		if err := rows.Scan(
			&match.Result.Book.ID, &match.Result.Book.Name, &match.Result.Book.Abbreviation,
			&testament, &match.Result.Chapter, &match.Result.Verse, &match.Result.Text, &match.Score,
		); err != nil {
			return nil, fmt.Errorf("read quote match: %w", err)
		}
		match.Result.Book.Testament = Testament(testament)
		matches = append(matches, match)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("read quote matches: %w", err)
	}
	return matches, nil
}
