package bible

import (
	"errors"
	"strings"
	"testing"
)

func testMeta() TranslationMeta {
	return TranslationMeta{ID: "kjv", Name: "King James Version", Abbreviation: "KJV", Language: "en", Version: "1.0.0"}
}

func TestDecodeJSONMapsDatasetBookSpellings(t *testing.T) {
	dataset := `{
		"Psalm": {"23": {"2": "He maketh me to lie down.", "1": "The Lord is my shepherd."}},
		"Song Of Solomon": {"1": {"1": "The song of songs."}},
		"3 John": {"1": {"15": "Peace be to thee."}}
	}`

	data, err := DecodeJSON(strings.NewReader(dataset), testMeta())
	if err != nil {
		t.Fatalf("decode: %v", err)
	}

	if len(data.Verses) != 4 {
		t.Fatalf("verses = %d, want 4", len(data.Verses))
	}

	// The Psalm verses come first in canonical order, then Song of Solomon, then 3 John.
	if data.Verses[0].BookID != 19 || data.Verses[1].BookID != 19 {
		t.Fatalf("first verses = %+v, want Psalms first", data.Verses[:2])
	}
	if data.Verses[2].BookID != 22 || data.Verses[2].Chapter != 1 {
		t.Fatalf("third verse = %+v, want Song of Solomon 1:1", data.Verses[2])
	}
	if data.Verses[3].BookID != 64 || data.Verses[3].Number != 15 {
		t.Fatalf("fourth verse = %+v, want 3 John 1:15", data.Verses[3])
	}
	if data.Verses[0].Number != 1 || data.Verses[1].Number != 2 {
		t.Fatalf("verses are not sorted numerically: %+v", data.Verses[:2])
	}
}

func TestDecodeJSONPreservesVerseText(t *testing.T) {
	dataset := `{"Genesis":{"1":{"1":"In the beginning\nGod created."}}}`
	data, err := DecodeJSON(strings.NewReader(dataset), testMeta())
	if err != nil {
		t.Fatalf("decode: %v", err)
	}
	if data.Verses[0].Text != "In the beginning\nGod created." {
		t.Fatalf("text = %q", data.Verses[0].Text)
	}
}

func TestDecodeJSONRejectsBadDatasets(t *testing.T) {
	cases := map[string]string{
		"malformed":           `{"Genesis":`,
		"empty":               `{}`,
		"unknown book":        `{"Hezekiah":{"1":{"1":"text"}}}`,
		"duplicate book":      `{"Psalm":{"1":{"1":"a"}},"Psalms":{"1":{"1":"b"}}}`,
		"non numeric chapter": `{"Genesis":{"one":{"1":"text"}}}`,
		"non numeric verse":   `{"Genesis":{"1":{"first":"text"}}}`,
		"empty verse":         `{"Genesis":{"1":{"1":"   "}}}`,
	}

	for name, dataset := range cases {
		if _, err := DecodeJSON(strings.NewReader(dataset), testMeta()); !errors.Is(err, ErrInvalidTranslation) {
			t.Fatalf("%s: err = %v, want ErrInvalidTranslation", name, err)
		}
	}
}
