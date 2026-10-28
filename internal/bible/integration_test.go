//go:build integration

package bible

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"io"
	"os"
	"path/filepath"
	"sort"
	"testing"
	"time"

	"zowe-worship/internal/database"
)

// TestRealDatasets installs the full KJV and AMP datasets and checks that the
// search targets hold on real content. Run with:
//
//	ZOWE_BIBLE_DATASETS=C:/Projects/Python/bible go test -tags integration ./internal/bible -run TestRealDatasets -v
func TestRealDatasets(t *testing.T) {
	datasets := os.Getenv("ZOWE_BIBLE_DATASETS")
	if datasets == "" {
		t.Skip("set ZOWE_BIBLE_DATASETS to the directory holding KJV/ and AMP/")
	}

	ctx := context.Background()
	repo, _ := newTestRepository(t)

	cases := []struct {
		id, name, abbr, dir, file string
		wantVerses                int
	}{
		{"kjv", "King James Version", "KJV", "KJV", "KJV_bible.json", 31102},
		{"amp", "Amplified Bible", "AMP", "AMP", "AMP_bible.json", 31103},
	}

	for _, tc := range cases {
		source, err := os.Open(filepath.Join(datasets, tc.dir, tc.file))
		if err != nil {
			t.Fatalf("open %s: %v", tc.id, err)
		}

		decodeStart := time.Now()
		data, err := DecodeJSON(source, TranslationMeta{
			ID: tc.id, Name: tc.name, Abbreviation: tc.abbr, Language: "en", Version: "1.0.0",
		})
		_ = source.Close()
		if err != nil {
			t.Fatalf("decode %s: %v", tc.id, err)
		}
		decodeDuration := time.Since(decodeStart)

		if len(data.Verses) != tc.wantVerses {
			t.Fatalf("%s verses = %d, want %d", tc.id, len(data.Verses), tc.wantVerses)
		}

		installStart := time.Now()
		installed, err := repo.Install(ctx, data)
		if err != nil {
			t.Fatalf("install %s: %v", tc.id, err)
		}
		installDuration := time.Since(installStart)

		t.Logf("%s: %d verses, decode %s, install %s",
			tc.id, installed.VerseCount, decodeDuration.Round(time.Millisecond),
			installDuration.Round(time.Millisecond))
	}

	searchCases := []struct {
		name string
		req  SearchRequest
		want string
	}{
		{"word", SearchRequest{TranslationID: "kjv", Text: "shepherd", Mode: SearchWords}, "Psalms 23:1"},
		{"phrase", SearchRequest{TranslationID: "kjv", Text: "the lord is my shepherd", Mode: SearchPhrase}, "Psalms 23:1"},
		{"prefix", SearchRequest{TranslationID: "kjv", Text: "shep", Mode: SearchPrefix}, "Psalms 23:1"},
		{"amp word", SearchRequest{TranslationID: "amp", Text: "shepherd", Mode: SearchWords}, "Psalms 23:1"},
		{"john", SearchRequest{TranslationID: "kjv", Text: "only begotten", Mode: SearchPhrase}, "John 3:16"},
	}
	for _, tc := range searchCases {
		results, err := repo.Search(ctx, tc.req)
		if err != nil {
			t.Fatalf("%s search: %v", tc.name, err)
		}
		found := false
		for _, result := range results {
			if result.Reference() == tc.want {
				found = true
				break
			}
		}
		if !found {
			t.Fatalf("%s search did not return %s; results: %v", tc.name, tc.want, results)
		}
	}

	var timings []time.Duration
	queries := []SearchRequest{
		{TranslationID: "kjv", Text: "faith", Mode: SearchWords},
		{TranslationID: "kjv", Text: "the lord is my shepherd", Mode: SearchPhrase},
		{TranslationID: "kjv", Text: "gra", Mode: SearchPrefix},
		{TranslationID: "kjv", Text: "love your enemies", Mode: SearchWords},
		{TranslationID: "kjv", Text: "in the beginning", Mode: SearchPhrase},
	}
	for i := 0; i < 40; i++ {
		req := queries[i%len(queries)]
		start := time.Now()
		if _, err := repo.Search(ctx, req); err != nil {
			t.Fatalf("timed search %q: %v", req.Text, err)
		}
		timings = append(timings, time.Since(start))
	}
	sort.Slice(timings, func(i, j int) bool { return timings[i] < timings[j] })
	t.Logf("search p95 over %d queries: %s (median %s)",
		len(timings), timings[len(timings)*95/100].Round(time.Microsecond),
		timings[len(timings)/2].Round(time.Microsecond))
}

// TestInstalledPackages checks packages that were installed into the real
// per-user data directory:
//
//	ZOWE_INSTALLED_DATA="$LOCALAPPDATA/Zowesoft/Zowe Worship" go test -tags integration ./internal/bible -run TestInstalledPackages -v
func TestInstalledPackages(t *testing.T) {
	root := os.Getenv("ZOWE_INSTALLED_DATA")
	if root == "" {
		t.Skip("set ZOWE_INSTALLED_DATA to the installed application data directory")
	}

	ctx := context.Background()
	paths := database.Paths{Root: root}
	core, err := database.Open(ctx, paths.CoreDatabase())
	if err != nil {
		t.Fatalf("open installed core database: %v", err)
	}
	defer core.Close()

	repo := NewSQLiteRepository(core, paths.Translations())
	translations, err := repo.ListTranslations(ctx)
	if err != nil {
		t.Fatalf("list installed translations: %v", err)
	}
	if len(translations) < 2 {
		t.Fatalf("installed translations = %d, want at least KJV and AMP", len(translations))
	}

	for _, translation := range translations {
		packagePath := filepath.Join(paths.Translations(),
			translation.ID+"-"+translation.Version+".db")
		checksum, err := checksumFile(packagePath)
		if err != nil {
			t.Fatalf("checksum %s: %v", translation.ID, err)
		}
		if checksum != translation.Checksum {
			t.Fatalf("%s checksum does not match catalog", translation.ID)
		}
		info, err := os.Stat(packagePath)
		if err != nil {
			t.Fatalf("stat %s package: %v", translation.ID, err)
		}

		results, err := repo.Search(ctx, SearchRequest{
			TranslationID: translation.ID, Text: "shepherd", Mode: SearchWords, Limit: 20,
		})
		if err != nil {
			t.Fatalf("search %s: %v", translation.ID, err)
		}
		found := false
		for _, result := range results {
			if result.Reference() == "Psalms 23:1" {
				found = true
				break
			}
		}
		if !found {
			t.Fatalf("%s shepherd search did not include Psalms 23:1: %v", translation.ID, results)
		}

		phrase, err := repo.Search(ctx, SearchRequest{
			TranslationID: translation.ID, Text: "the lord is my shepherd", Mode: SearchPhrase, Limit: 5,
		})
		if err != nil {
			t.Fatalf("phrase search %s: %v", translation.ID, err)
		}
		if len(phrase) == 0 || phrase[0].Reference() != "Psalms 23:1" {
			t.Fatalf("%s phrase search = %v", translation.ID, phrase)
		}
		t.Logf("%s %s: %d verses, %d file bytes, checksum ok",
			translation.Abbreviation, translation.Version, translation.VerseCount, info.Size())
	}
}

func checksumFile(path string) (string, error) {
	file, err := os.Open(path)
	if err != nil {
		return "", err
	}
	defer file.Close()

	hash := sha256.New()
	if _, err := io.Copy(hash, file); err != nil {
		return "", err
	}
	return hex.EncodeToString(hash.Sum(nil)), nil
}
