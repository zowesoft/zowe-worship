package main

import (
	"context"
	"flag"
	"fmt"
	"log"
	"os"
	"path/filepath"
	"time"

	"zowe-worship/internal/bible"
	"zowe-worship/internal/database"
)

func main() {
	log.SetFlags(0)

	source := flag.String("source", "", "path to the <ABBR>_bible.json dataset")
	id := flag.String("id", "", "translation id, lowercase letters and digits (e.g. kjv)")
	name := flag.String("name", "", "translation display name")
	abbreviation := flag.String("abbr", "", "translation abbreviation shown to operators")
	language := flag.String("language", "en", "translation language code")
	version := flag.String("version", "1.0.0", "package version")
	dataDir := flag.String("data-dir", "", "override the per-user data directory")
	flag.Parse()

	switch {
	case *source == "":
		log.Fatal("-source is required")
	case *id == "":
		log.Fatal("-id is required")
	case *name == "":
		log.Fatal("-name is required")
	case *abbreviation == "":
		log.Fatal("-abbr is required")
	}

	paths, err := resolvePaths(*dataDir)
	if err != nil {
		log.Fatal(err)
	}
	if err := paths.Ensure(); err != nil {
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

	sourceFile, err := os.Open(*source)
	if err != nil {
		log.Fatalf("open dataset: %v", err)
	}
	defer sourceFile.Close()

	meta := bible.TranslationMeta{
		ID:           *id,
		Name:         *name,
		Abbreviation: *abbreviation,
		Language:     *language,
		Version:      *version,
	}

	started := time.Now()
	data, err := bible.DecodeJSON(sourceFile, meta)
	if err != nil {
		log.Fatal(err)
	}

	repo := bible.NewSQLiteRepository(core, paths.Translations())
	translation, err := repo.Install(ctx, data)
	if err != nil {
		log.Fatal(err)
	}

	fmt.Printf("installed %s (%s) %s\n", translation.Abbreviation, translation.Name, translation.Version)
	fmt.Printf("verses:  %d\n", translation.VerseCount)
	fmt.Printf("package: %s\n", filepath.Join(paths.Translations(), fmt.Sprintf("%s-%s.db", *id, *version)))
	fmt.Printf("elapsed: %s\n", time.Since(started).Round(time.Millisecond))
}

func resolvePaths(override string) (database.Paths, error) {
	if override != "" {
		return database.Paths{Root: override}, nil
	}
	return database.DefaultPaths()
}
