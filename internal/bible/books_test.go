package bible

import "testing"

func TestLookupBookAcceptsDatasetSpellings(t *testing.T) {
	cases := map[string]int{
		"Genesis":         1,
		"Psalm":           19,
		"psalms":          19,
		"Ps.":             19,
		"Song Of Solomon": 22,
		"Song of Songs":   22,
		"1 Sam.":          9,
		"1 samuel":        9,
		"Revelation":      66,
	}

	for name, wantID := range cases {
		book, ok := LookupBook(name)
		if !ok {
			t.Fatalf("%q was not resolved", name)
		}
		if book.ID != wantID {
			t.Fatalf("%q resolved to %d, want %d", name, book.ID, wantID)
		}
	}

	if _, ok := LookupBook("Hezekiah"); ok {
		t.Fatal("unknown book resolved")
	}
}

func TestBookByIDBounds(t *testing.T) {
	if _, ok := BookByID(0); ok {
		t.Fatal("book 0 resolved")
	}
	if _, ok := BookByID(67); ok {
		t.Fatal("book 67 resolved")
	}
	book, ok := BookByID(66)
	if !ok || book.Name != "Revelation" {
		t.Fatalf("book 66 = %+v, ok = %v", book, ok)
	}
}

func TestCanonicalBookOrderIsContiguous(t *testing.T) {
	if len(CanonicalBooks) != 66 {
		t.Fatalf("canonical books = %d, want 66", len(CanonicalBooks))
	}
	for i, book := range CanonicalBooks {
		if book.ID != i+1 {
			t.Fatalf("book at index %d has id %d", i, book.ID)
		}
		wantTestament := OldTestament
		if book.ID > 39 {
			wantTestament = NewTestament
		}
		if book.Testament != wantTestament {
			t.Fatalf("%s has testament %q", book.Name, book.Testament)
		}
	}
}
