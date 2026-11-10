package main

import (
	"context"
	"flag"
	"fmt"
	"log"
	"os"
	"sort"
	"time"

	"zowe-worship/internal/bible"
	"zowe-worship/internal/database"
)

// The targets the tracker records. A service operator feels a slow reference
// immediately; the app is held to these on every change.
const (
	referenceTargetMS = 50
	searchTargetMS    = 150
)

func main() {
	log.SetFlags(0)

	translation := flag.String("translation", "niv", "id of the installed translation to measure")
	runs := flag.Int("runs", 40, "timed runs per workload (after warmup)")
	dataDir := flag.String("data-dir", "", "override the per-user data directory")
	flag.Parse()

	paths, err := resolvePaths(*dataDir)
	if err != nil {
		log.Fatal(err)
	}

	ctx := context.Background()
	core, err := database.Open(ctx, paths.CoreDatabase())
	if err != nil {
		log.Fatal(err)
	}
	defer core.Close()
	if err := database.Migrate(ctx, core, paths.Backups()); err != nil {
		log.Fatal(err)
	}

	repo := bible.NewSQLiteRepository(core, paths.Translations())
	if _, err := repo.Translation(ctx, *translation); err != nil {
		log.Fatalf("translation %q is not installed: %v", *translation, err)
	}

	type workload struct {
		name  string
		kind  string // "reference" or "search"
		run   func(context.Context) error
		times []time.Duration
	}
	reference := func(bookName string, chapter, start, end int) func(context.Context) error {
		book, _ := bible.LookupBook(bookName)
		return func(ctx context.Context) error {
			_, err := repo.Passage(ctx, *translation, book.ID, chapter, start, end)
			return err
		}
	}
	search := func(mode bible.SearchMode, text string) func(context.Context) error {
		return func(ctx context.Context) error {
			_, err := repo.Search(ctx, bible.SearchRequest{
				TranslationID: *translation, Text: text, Mode: mode, Limit: 20,
			})
			return err
		}
	}

	workloads := []*workload{
		{"single verse (John 3:16)", "reference", reference("John", 3, 16, 16), nil},
		{"verse range (1 Corinthians 13:4-7)", "reference", reference("1 Corinthians", 13, 4, 7), nil},
		{"whole chapter (Psalm 23)", "reference", reference("Psalm", 23, 0, 0), nil},
		{"long chapter (Psalm 119:1-40)", "reference", reference("Psalm", 119, 1, 40), nil},
		{"words: god love world", "search", search(bible.SearchWords, "god love world"), nil},
		{"words: believe saved", "search", search(bible.SearchWords, "believe saved"), nil},
		{"words: shepherd still waters", "search", search(bible.SearchWords, "shepherd still waters"), nil},
		{"phrase: kingdom of heaven", "search", search(bible.SearchPhrase, "kingdom of heaven"), nil},
		{"prefix: believ", "search", search(bible.SearchPrefix, "believ"), nil},
	}

	// Warmup: the first query pays for SQLite page cache and FTS index load,
	// which says nothing about steady-state service use.
	for _, w := range workloads {
		for i := 0; i < 3; i++ {
			if err := w.run(ctx); err != nil {
				log.Fatalf("warmup %s: %v", w.name, err)
			}
		}
	}

	for i := 0; i < *runs; i++ {
		for _, w := range workloads {
			started := time.Now()
			if err := w.run(ctx); err != nil {
				log.Fatalf("%s: %v", w.name, err)
			}
			w.times = append(w.times, time.Since(started))
		}
	}

	fmt.Printf("bible-bench: %s, %d runs per workload (after 3 warmups)\n\n", *translation, *runs)
	failed := false
	for _, w := range workloads {
		target := referenceTargetMS
		if w.kind == "search" {
			target = searchTargetMS
		}
		p50, p95, max := percentiles(w.times)
		ok := float64(p95.Milliseconds()) < float64(target)
		if !ok {
			failed = true
		}
		fmt.Printf("%-38s p50 %7.1fms  p95 %7.1fms  max %7.1fms  (target p95 < %dms %s)\n",
			w.name, ms(p50), ms(p95), ms(max), target, mark(ok))
	}
	if failed {
		fmt.Println("\nbench FAILED: a workload missed its target")
		os.Exit(1)
	}
	fmt.Println("\nbench PASSED")
}

func ms(d time.Duration) float64 { return float64(d.Microseconds()) / 1000.0 }

func mark(ok bool) string {
	if ok {
		return "ok"
	}
	return "MISSED"
}

func percentiles(times []time.Duration) (p50, p95, max time.Duration) {
	sorted := append([]time.Duration(nil), times...)
	sort.Slice(sorted, func(i, j int) bool { return sorted[i] < sorted[j] })
	pick := func(fraction float64) time.Duration {
		index := int(fraction * float64(len(sorted)-1))
		return sorted[index]
	}
	return pick(0.50), pick(0.95), sorted[len(sorted)-1]
}

func resolvePaths(override string) (database.Paths, error) {
	if override != "" {
		paths := database.Paths{Root: override}
		return paths, paths.Ensure()
	}
	return database.DefaultPaths()
}
