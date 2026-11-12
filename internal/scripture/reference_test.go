package scripture

import (
	"errors"
	"testing"

	"zopresent/internal/bible"
)

func TestParseReferences(t *testing.T) {
	cases := []struct {
		query string
		want  Reference
	}{
		{"John 3:16", Reference{Book: bookNamed(t, "John"), Chapter: 3, VerseStart: 16, VerseEnd: 16}},
		{"Romans 8:28-30", Reference{Book: bookNamed(t, "Romans"), Chapter: 8, VerseStart: 28, VerseEnd: 30}},
		{"Psalm 23", Reference{Book: bookNamed(t, "Psalms"), Chapter: 23}},
		{"1 John 5:7", Reference{Book: bookNamed(t, "1 John"), Chapter: 5, VerseStart: 7, VerseEnd: 7}},
		{"Song Of Solomon 1:1", Reference{Book: bookNamed(t, "Song of Solomon"), Chapter: 1, VerseStart: 1, VerseEnd: 1}},
		{"psalm 23:1", Reference{Book: bookNamed(t, "Psalms"), Chapter: 23, VerseStart: 1, VerseEnd: 1}},
		{"John 3:16 – 17", Reference{Book: bookNamed(t, "John"), Chapter: 3, VerseStart: 16, VerseEnd: 17}},
	}

	for _, tc := range cases {
		got, err := Parse(tc.query)
		if err != nil {
			t.Fatalf("%q: %v", tc.query, err)
		}
		if got != tc.want {
			t.Fatalf("%q parsed to %+v, want %+v", tc.query, got, tc.want)
		}
	}
}

func TestParseRejectsInvalidReferences(t *testing.T) {
	for _, query := range []string{
		"John 0:1",
		"John 3:0",
		"Romans 8:30-28",
		"John 3:16-0",
	} {
		if _, err := Parse(query); !errors.Is(err, ErrInvalidReference) {
			t.Fatalf("%q: err = %v, want ErrInvalidReference", query, err)
		}
	}
}

func TestParseReportsOrdinaryText(t *testing.T) {
	for _, query := range []string{
		"",
		"grace",
		"john",
		"3:16",
		"John three",
		"Hezekiah 3:1",
	} {
		if _, err := Parse(query); !errors.Is(err, ErrNotReference) {
			t.Fatalf("%q: err = %v, want ErrNotReference", query, err)
		}
	}
}

func TestReferenceString(t *testing.T) {
	cases := map[string]Reference{
		"John 3:16":      {Book: bookNamed(t, "John"), Chapter: 3, VerseStart: 16, VerseEnd: 16},
		"Romans 8:28-30": {Book: bookNamed(t, "Romans"), Chapter: 8, VerseStart: 28, VerseEnd: 30},
		"Psalms 23":      {Book: bookNamed(t, "Psalms"), Chapter: 23},
	}
	for want, reference := range cases {
		if got := reference.String(); got != want {
			t.Fatalf("String() = %q, want %q", got, want)
		}
	}
}

func bookNamed(t *testing.T, name string) bible.Book {
	t.Helper()
	book, ok := bible.LookupBook(name)
	if !ok {
		t.Fatalf("book %q not found", name)
	}
	return book
}
