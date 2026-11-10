package bible

import "strings"

type Testament string

const (
	OldTestament Testament = "old"
	NewTestament Testament = "new"
)

// Book is canonical Bible book metadata. IDs follow the 66-book Protestant
// ordering and are shared by every installed translation.
type Book struct {
	ID           int
	Name         string
	Abbreviation string
	Testament    Testament
	Chapters     int
}

var CanonicalBooks = []Book{
	{1, "Genesis", "Gen", OldTestament, 50},
	{2, "Exodus", "Exod", OldTestament, 40},
	{3, "Leviticus", "Lev", OldTestament, 27},
	{4, "Numbers", "Num", OldTestament, 36},
	{5, "Deuteronomy", "Deut", OldTestament, 34},
	{6, "Joshua", "Josh", OldTestament, 24},
	{7, "Judges", "Judg", OldTestament, 21},
	{8, "Ruth", "Ruth", OldTestament, 4},
	{9, "1 Samuel", "1 Sam", OldTestament, 31},
	{10, "2 Samuel", "2 Sam", OldTestament, 24},
	{11, "1 Kings", "1 Kgs", OldTestament, 22},
	{12, "2 Kings", "2 Kgs", OldTestament, 25},
	{13, "1 Chronicles", "1 Chr", OldTestament, 29},
	{14, "2 Chronicles", "2 Chr", OldTestament, 36},
	{15, "Ezra", "Ezra", OldTestament, 10},
	{16, "Nehemiah", "Neh", OldTestament, 13},
	{17, "Esther", "Esth", OldTestament, 10},
	{18, "Job", "Job", OldTestament, 42},
	{19, "Psalms", "Ps", OldTestament, 150},
	{20, "Proverbs", "Prov", OldTestament, 31},
	{21, "Ecclesiastes", "Eccl", OldTestament, 12},
	{22, "Song of Solomon", "Song", OldTestament, 8},
	{23, "Isaiah", "Isa", OldTestament, 66},
	{24, "Jeremiah", "Jer", OldTestament, 52},
	{25, "Lamentations", "Lam", OldTestament, 5},
	{26, "Ezekiel", "Ezek", OldTestament, 48},
	{27, "Daniel", "Dan", OldTestament, 12},
	{28, "Hosea", "Hos", OldTestament, 14},
	{29, "Joel", "Joel", OldTestament, 3},
	{30, "Amos", "Amos", OldTestament, 9},
	{31, "Obadiah", "Obad", OldTestament, 1},
	{32, "Jonah", "Jonah", OldTestament, 4},
	{33, "Micah", "Mic", OldTestament, 7},
	{34, "Nahum", "Nah", OldTestament, 3},
	{35, "Habakkuk", "Hab", OldTestament, 3},
	{36, "Zephaniah", "Zeph", OldTestament, 3},
	{37, "Haggai", "Hag", OldTestament, 2},
	{38, "Zechariah", "Zech", OldTestament, 14},
	{39, "Malachi", "Mal", OldTestament, 4},
	{40, "Matthew", "Matt", NewTestament, 28},
	{41, "Mark", "Mark", NewTestament, 16},
	{42, "Luke", "Luke", NewTestament, 24},
	{43, "John", "John", NewTestament, 21},
	{44, "Acts", "Acts", NewTestament, 28},
	{45, "Romans", "Rom", NewTestament, 16},
	{46, "1 Corinthians", "1 Cor", NewTestament, 16},
	{47, "2 Corinthians", "2 Cor", NewTestament, 13},
	{48, "Galatians", "Gal", NewTestament, 6},
	{49, "Ephesians", "Eph", NewTestament, 6},
	{50, "Philippians", "Phil", NewTestament, 4},
	{51, "Colossians", "Col", NewTestament, 4},
	{52, "1 Thessalonians", "1 Thess", NewTestament, 5},
	{53, "2 Thessalonians", "2 Thess", NewTestament, 3},
	{54, "1 Timothy", "1 Tim", NewTestament, 6},
	{55, "2 Timothy", "2 Tim", NewTestament, 4},
	{56, "Titus", "Titus", NewTestament, 3},
	{57, "Philemon", "Phlm", NewTestament, 1},
	{58, "Hebrews", "Heb", NewTestament, 13},
	{59, "James", "Jas", NewTestament, 5},
	{60, "1 Peter", "1 Pet", NewTestament, 5},
	{61, "2 Peter", "2 Pet", NewTestament, 3},
	{62, "1 John", "1 John", NewTestament, 5},
	{63, "2 John", "2 John", NewTestament, 1},
	{64, "3 John", "3 John", NewTestament, 1},
	{65, "Jude", "Jude", NewTestament, 1},
	{66, "Revelation", "Rev", NewTestament, 22},
}

// bookAliases covers the spelling differences between translation datasets.
var bookAliases = map[string]string{
	"psalm":              "psalms",
	"song of songs":      "song of solomon",
	"canticles":          "song of solomon",
	"revelation of john": "revelation",
	"apocalypse":         "revelation",
}

var booksByName map[string]Book

func init() {
	booksByName = make(map[string]Book, len(CanonicalBooks)*3)
	for _, book := range CanonicalBooks {
		for _, name := range []string{book.Name, book.Abbreviation} {
			key := NormalizeBookName(name)
			if existing, ok := booksByName[key]; ok && existing.ID != book.ID {
				panic("bible: book name collision between " + existing.Name + " and " + book.Name)
			}
			booksByName[key] = book
		}
	}
	for alias, target := range bookAliases {
		book, ok := booksByName[target]
		if !ok {
			panic("bible: alias points at an unknown book: " + target)
		}
		booksByName[alias] = book
	}
}

func NormalizeBookName(name string) string {
	normalized := strings.ToLower(strings.TrimSpace(name))
	normalized = strings.ReplaceAll(normalized, ".", "")
	return strings.Join(strings.Fields(normalized), " ")
}

// MissingBooks names the canonical books a dataset does not carry, so a
// package can be checked for completeness before it is bundled.
func MissingBooks(data TranslationData) []string {
	present := make(map[int]bool, len(CanonicalBooks))
	for _, verse := range data.Verses {
		present[verse.BookID] = true
	}
	missing := make([]string, 0)
	for _, book := range CanonicalBooks {
		if !present[book.ID] {
			missing = append(missing, book.Name)
		}
	}
	return missing
}

func LookupBook(name string) (Book, bool) {
	book, ok := booksByName[NormalizeBookName(name)]
	return book, ok
}

func BookByID(id int) (Book, bool) {
	if id < 1 || id > len(CanonicalBooks) {
		return Book{}, false
	}
	return CanonicalBooks[id-1], true
}
