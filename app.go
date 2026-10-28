package main

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"log"
	"strings"

	"zowe-worship/internal/bible"
	"zowe-worship/internal/database"
	"zowe-worship/internal/scripture"
)

var errStorageUnavailable = errors.New("storage is not available")

type App struct {
	ctx      context.Context
	dataRoot string
	core     *sql.DB
	bible    *bible.SQLiteRepository
}

type AppInfo struct {
	Name    string `json:"name"`
	Version string `json:"version"`
}

type TranslationView struct {
	ID           string `json:"id"`
	Name         string `json:"name"`
	Abbreviation string `json:"abbreviation"`
	Language     string `json:"language"`
	Version      string `json:"version"`
	VerseCount   int    `json:"verseCount"`
}

// BibleSearchRequest is one operator query. Mode "auto" resolves a reference
// first and falls back to word search; "words", "phrase" and "prefix" force
// full-text search.
type BibleSearchRequest struct {
	TranslationID string `json:"translationId"`
	Query         string `json:"query"`
	Mode          string `json:"mode"`
	Limit         int    `json:"limit"`
}

type BibleSearchResponse struct {
	Kind      string       `json:"kind"`
	Reference string       `json:"reference,omitempty"`
	Results   []BibleVerse `json:"results"`
}

type BibleVerse struct {
	BookID    int    `json:"bookId"`
	Book      string `json:"book"`
	Chapter   int    `json:"chapter"`
	Verse     int    `json:"verse"`
	Reference string `json:"reference"`
	Text      string `json:"text"`
}

func NewApp() *App {
	return &App{}
}

// newAppAt lets tests point storage at a temporary directory.
func newAppAt(dataRoot string) *App {
	return &App{dataRoot: dataRoot}
}

func (a *App) startup(ctx context.Context) {
	a.ctx = ctx

	paths, err := a.paths()
	if err != nil {
		log.Printf("zowe: storage unavailable: %v", err)
		return
	}
	if err := paths.Ensure(); err != nil {
		log.Printf("zowe: storage unavailable: %v", err)
		return
	}

	core, err := database.Open(ctx, paths.CoreDatabase())
	if err != nil {
		log.Printf("zowe: open database: %v", err)
		return
	}
	if err := database.Migrate(ctx, core, paths.Backups()); err != nil {
		_ = core.Close()
		log.Printf("zowe: migrate database: %v", err)
		return
	}

	a.core = core
	a.bible = bible.NewSQLiteRepository(core, paths.Translations())
}

func (a *App) shutdown(_ context.Context) {
	if a.core == nil {
		return
	}
	if err := a.core.Close(); err != nil {
		log.Printf("zowe: close database: %v", err)
	}
	a.core = nil
}

func (a *App) paths() (database.Paths, error) {
	if a.dataRoot != "" {
		return database.Paths{Root: a.dataRoot}, nil
	}
	return database.DefaultPaths()
}

func (a *App) GetAppInfo() AppInfo {
	return AppInfo{
		Name:    "Zowe Worship",
		Version: "0.1.0",
	}
}

func (a *App) ListTranslations() ([]TranslationView, error) {
	if a.bible == nil {
		return nil, errStorageUnavailable
	}

	translations, err := a.bible.ListTranslations(a.context())
	if err != nil {
		return nil, err
	}

	views := make([]TranslationView, 0, len(translations))
	for _, translation := range translations {
		views = append(views, TranslationView{
			ID:           translation.ID,
			Name:         translation.Name,
			Abbreviation: translation.Abbreviation,
			Language:     translation.Language,
			Version:      translation.Version,
			VerseCount:   translation.VerseCount,
		})
	}
	return views, nil
}

func (a *App) SearchBible(request BibleSearchRequest) (BibleSearchResponse, error) {
	if a.bible == nil {
		return BibleSearchResponse{}, errStorageUnavailable
	}

	translationID := strings.TrimSpace(request.TranslationID)
	query := strings.TrimSpace(request.Query)
	if translationID == "" {
		return BibleSearchResponse{}, errors.New("translation is required")
	}
	if query == "" {
		return BibleSearchResponse{}, errors.New("search text is required")
	}

	mode := strings.ToLower(strings.TrimSpace(request.Mode))
	if mode == "" {
		mode = "auto"
	}

	ctx := a.context()

	if mode == "auto" {
		reference, err := scripture.Parse(query)
		switch {
		case err == nil:
			verses, err := a.bible.Passage(ctx, translationID,
				reference.Book.ID, reference.Chapter, reference.VerseStart, reference.VerseEnd)
			if err != nil {
				return BibleSearchResponse{}, err
			}
			if len(verses) == 0 {
				return BibleSearchResponse{}, fmt.Errorf("%s was not found in %s", reference.String(), translationID)
			}
			return BibleSearchResponse{
				Kind:      "reference",
				Reference: reference.String(),
				Results:   toBibleVerses(verses),
			}, nil
		case errors.Is(err, scripture.ErrNotReference):
			mode = "words"
		default:
			return BibleSearchResponse{}, err
		}
	}

	var searchMode bible.SearchMode
	switch mode {
	case "words":
		searchMode = bible.SearchWords
	case "phrase":
		searchMode = bible.SearchPhrase
	case "prefix":
		searchMode = bible.SearchPrefix
	default:
		return BibleSearchResponse{}, fmt.Errorf("unknown search mode %q", mode)
	}

	results, err := a.bible.Search(ctx, bible.SearchRequest{
		TranslationID: translationID,
		Text:          query,
		Mode:          searchMode,
		Limit:         request.Limit,
	})
	if err != nil {
		return BibleSearchResponse{}, err
	}
	return BibleSearchResponse{Kind: "text", Results: toBibleVerses(results)}, nil
}

func (a *App) context() context.Context {
	if a.ctx != nil {
		return a.ctx
	}
	return context.Background()
}

func toBibleVerses(results []bible.SearchResult) []BibleVerse {
	verses := make([]BibleVerse, 0, len(results))
	for _, result := range results {
		verses = append(verses, BibleVerse{
			BookID:    result.Book.ID,
			Book:      result.Book.Name,
			Chapter:   result.Chapter,
			Verse:     result.Verse,
			Reference: result.Reference(),
			Text:      result.Text,
		})
	}
	return verses
}
