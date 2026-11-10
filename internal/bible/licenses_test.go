package bible

import (
	"context"
	"errors"
	"testing"
)

func TestLicenseRegistryStatesFacts(t *testing.T) {
	kjv, ok := LicenseFor("kjv")
	if !ok || kjv.Status != DistributionPublicDomain || kjv.Attribution == "" {
		t.Fatalf("kjv = %+v, want public domain with an attribution line", kjv)
	}
	for id, license := range knownLicenses {
		if license.Copyright == "" || license.License == "" {
			t.Fatalf("%s carries no copyright or licence line: %+v", id, license)
		}
	}
}

func TestCheckBundlingRefusesCopyrightedText(t *testing.T) {
	if err := CheckBundling("kjv"); err != nil {
		t.Fatalf("public domain text was refused: %v", err)
	}
	if err := CheckBundling("niv"); !errors.Is(err, ErrDistributionNotPermitted) {
		t.Fatalf("err = %v, want ErrDistributionNotPermitted", err)
	}
	if err := CheckBundling("web"); !errors.Is(err, ErrLicenseUnknown) {
		t.Fatalf("unregistered translation err = %v, want ErrLicenseUnknown", err)
	}
}

func TestInstallRecordsLicenseAndAttribution(t *testing.T) {
	repo, _ := newTestRepository(t)
	ctx := context.Background()

	// A known translation picks its license up from the registry.
	installed, err := repo.Install(ctx, fixture("kjv", "1.0.0", "In the beginning God created the heaven and the earth."))
	if err != nil {
		t.Fatalf("install kjv: %v", err)
	}
	if installed.License != "Public domain" || installed.Attribution == "" {
		t.Fatalf("installed = %+v, want registry licence and attribution", installed)
	}
	read, err := repo.Translation(ctx, "kjv")
	if err != nil {
		t.Fatalf("read back: %v", err)
	}
	if read.License != installed.License || read.Attribution != installed.Attribution {
		t.Fatalf("catalog lost the licence: %+v", read)
	}

	// An unknown translation keeps whatever the importer stated.
	stated, err := repo.Install(ctx, TranslationData{
		TranslationMeta: TranslationMeta{
			ID: "web", Name: "World English Bible", Abbreviation: "WEB", Language: "en",
			Version: "1.0.0", License: "Public domain", Attribution: "World English Bible",
		},
		Verses: []Verse{{BookID: 1, Chapter: 1, Number: 1, Text: "In the beginning God created the heavens and the earth."}},
	})
	if err != nil {
		t.Fatalf("install web: %v", err)
	}
	if stated.License != "Public domain" || stated.Attribution != "World English Bible" {
		t.Fatalf("stated licence was not kept: %+v", stated)
	}
}

func TestMissingBooksFindsGaps(t *testing.T) {
	data := TranslationData{Verses: []Verse{
		{BookID: 1, Chapter: 1, Number: 1, Text: "one"},
		{BookID: 66, Chapter: 1, Number: 1, Text: "two"},
	}}
	missing := MissingBooks(data)
	if len(missing) != 64 {
		t.Fatalf("missing = %d books (%v), want 64", len(missing), missing)
	}
	if missing[0] != "Exodus" || missing[len(missing)-1] != "Jude" {
		t.Fatalf("missing endpoints = %q..%q, want Exodus..Jude", missing[0], missing[len(missing)-1])
	}
	complete := make([]Verse, 0, 66)
	for id := 1; id <= 66; id++ {
		complete = append(complete, Verse{BookID: id, Chapter: 1, Number: 1, Text: "x"})
	}
	if got := MissingBooks(TranslationData{Verses: complete}); len(got) != 0 {
		t.Fatalf("a complete dataset reported missing books: %v", got)
	}
}
