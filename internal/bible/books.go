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
}

var CanonicalBooks = []Book{
	{1, "Genesis", "Gen", OldTestament},
	{2, "Exodus", "Exod", OldTestament},
	{3, "Leviticus", "Lev", OldTestament},
	{4, "Numbers", "Num", OldTestament},
	{5, "Deuteronomy", "Deut", OldTestament},
	{6, "Joshua", "Josh", OldTestament},
	{7, "Judges", "Judg", OldTestament},
	{8, "Ruth", "Ruth", OldTestament},
	{9, "1 Samuel", "1 Sam", OldTestament},
	{10, "2 Samuel", "2 Sam", OldTestament},
	{11, "1 Kings", "1 Kgs", OldTestament},
	{12, "2 Kings", "2 Kgs", OldTestament},
	{13, "1 Chronicles", "1 Chr", OldTestament},
	{14, "2 Chronicles", "2 Chr", OldTestament},
	{15, "Ezra", "Ezra", OldTestament},
	{16, "Nehemiah", "Neh", OldTestament},
	{17, "Esther", "Esth", OldTestament},
	{18, "Job", "Job", OldTestament},
	{19, "Psalms", "Ps", OldTestament},
	{20, "Proverbs", "Prov", OldTestament},
	{21, "Ecclesiastes", "Eccl", OldTestament},
	{22, "Song of Solomon", "Song", OldTestament},
	{23, "Isaiah", "Isa", OldTestament},
	{24, "Jeremiah", "Jer", OldTestament},
	{25, "Lamentations", "Lam", OldTestament},
	{26, "Ezekiel", "Ezek", OldTestament},
	{27, "Daniel", "Dan", OldTestament},
	{28, "Hosea", "Hos", OldTestament},
	{29, "Joel", "Joel", OldTestament},
	{30, "Amos", "Amos", OldTestament},
	{31, "Obadiah", "Obad", OldTestament},
	{32, "Jonah", "Jonah", OldTestament},
	{33, "Micah", "Mic", OldTestament},
	{34, "Nahum", "Nah", OldTestament},
	{35, "Habakkuk", "Hab", OldTestament},
	{36, "Zephaniah", "Zeph", OldTestament},
	{37, "Haggai", "Hag", OldTestament},
	{38, "Zechariah", "Zech", OldTestament},
	{39, "Malachi", "Mal", OldTestament},
	{40, "Matthew", "Matt", NewTestament},
	{41, "Mark", "Mark", NewTestament},
	{42, "Luke", "Luke", NewTestament},
	{43, "John", "John", NewTestament},
	{44, "Acts", "Acts", NewTestament},
	{45, "Romans", "Rom", NewTestament},
	{46, "1 Corinthians", "1 Cor", NewTestament},
	{47, "2 Corinthians", "2 Cor", NewTestament},
	{48, "Galatians", "Gal", NewTestament},
	{49, "Ephesians", "Eph", NewTestament},
	{50, "Philippians", "Phil", NewTestament},
	{51, "Colossians", "Col", NewTestament},
	{52, "1 Thessalonians", "1 Thess", NewTestament},
	{53, "2 Thessalonians", "2 Thess", NewTestament},
	{54, "1 Timothy", "1 Tim", NewTestament},
	{55, "2 Timothy", "2 Tim", NewTestament},
	{56, "Titus", "Titus", NewTestament},
	{57, "Philemon", "Phlm", NewTestament},
	{58, "Hebrews", "Heb", NewTestament},
	{59, "James", "Jas", NewTestament},
	{60, "1 Peter", "1 Pet", NewTestament},
	{61, "2 Peter", "2 Pet", NewTestament},
	{62, "1 John", "1 John", NewTestament},
	{63, "2 John", "2 John", NewTestament},
	{64, "3 John", "3 John", NewTestament},
	{65, "Jude", "Jude", NewTestament},
	{66, "Revelation", "Rev", NewTestament},
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
