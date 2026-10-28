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
	"zowe-worship/internal/presentation"
	"zowe-worship/internal/scripture"
	"zowe-worship/internal/serviceorder"
)

var errStorageUnavailable = errors.New("storage is not available")

type App struct {
	ctx          context.Context
	dataRoot     string
	core         *sql.DB
	bible        *bible.SQLiteRepository
	order        *serviceorder.SQLiteRepository
	presentation *presentation.Controller
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
	a.order = serviceorder.NewSQLiteRepository(core)
	a.presentation = presentation.New(a.bible, a.order)
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

type ServiceView struct {
	ID   string `json:"id"`
	Name string `json:"name"`
}

type ServiceItemView struct {
	ID            string `json:"id"`
	Position      int    `json:"position"`
	Kind          string `json:"kind"`
	Title         string `json:"title"`
	TranslationID string `json:"translationId"`
	BookID        int    `json:"bookId"`
	Chapter       int    `json:"chapter"`
	VerseStart    int    `json:"verseStart"`
	VerseEnd      int    `json:"verseEnd"`
}

type ServiceOrderView struct {
	Service *ServiceView      `json:"service"`
	Items   []ServiceItemView `json:"items"`
}

// ScriptureRequest addresses verses already resolved by Bible search, so the
// schedule stores a reference rather than a copy of scripture text.
type ScriptureRequest struct {
	TranslationID string `json:"translationId"`
	BookID        int    `json:"bookId"`
	Chapter       int    `json:"chapter"`
	VerseStart    int    `json:"verseStart"`
	VerseEnd      int    `json:"verseEnd"`
}

type PresentationView struct {
	Preview *presentation.Content `json:"preview,omitempty"`
	Live    *presentation.Content `json:"live,omitempty"`
}

func (a *App) GetServiceOrder() (ServiceOrderView, error) {
	if a.order == nil {
		return ServiceOrderView{}, errStorageUnavailable
	}
	return a.serviceOrderView(a.context())
}

func (a *App) CreateService(name string) (ServiceOrderView, error) {
	if a.order == nil {
		return ServiceOrderView{}, errStorageUnavailable
	}
	if _, err := a.order.Create(a.context(), name); err != nil {
		return ServiceOrderView{}, err
	}
	return a.serviceOrderView(a.context())
}

func (a *App) AddScriptureToService(request ScriptureRequest) (ServiceOrderView, error) {
	if a.bible == nil || a.order == nil {
		return ServiceOrderView{}, errStorageUnavailable
	}

	ctx := a.context()
	normalized, title, err := normalizeScripture(request)
	if err != nil {
		return ServiceOrderView{}, err
	}

	verses, err := a.bible.Passage(ctx, normalized.TranslationID, normalized.BookID,
		normalized.Chapter, normalized.VerseStart, normalized.VerseEnd)
	if err != nil {
		return ServiceOrderView{}, err
	}
	if len(verses) == 0 {
		return ServiceOrderView{}, fmt.Errorf("%s was not found in %s", title, normalized.TranslationID)
	}

	service, err := a.ensureService(ctx)
	if err != nil {
		return ServiceOrderView{}, err
	}
	if _, err := a.order.AddScripture(ctx, service.ID, serviceorder.ScriptureInput{
		Title:         title,
		TranslationID: normalized.TranslationID,
		BookID:        normalized.BookID,
		Chapter:       normalized.Chapter,
		VerseStart:    normalized.VerseStart,
		VerseEnd:      normalized.VerseEnd,
	}); err != nil {
		return ServiceOrderView{}, err
	}
	return a.serviceOrderView(ctx)
}

func (a *App) RemoveServiceItem(itemID string) (ServiceOrderView, error) {
	if a.order == nil {
		return ServiceOrderView{}, errStorageUnavailable
	}
	if err := a.order.RemoveItem(a.context(), strings.TrimSpace(itemID)); err != nil {
		return ServiceOrderView{}, err
	}
	return a.serviceOrderView(a.context())
}

func (a *App) GetPresentationState() (PresentationView, error) {
	if a.presentation == nil {
		return PresentationView{}, errStorageUnavailable
	}
	return presentationView(a.presentation.State()), nil
}

func (a *App) PreviewScripture(request ScriptureRequest) (PresentationView, error) {
	if a.presentation == nil {
		return PresentationView{}, errStorageUnavailable
	}
	normalized, _, err := normalizeScripture(request)
	if err != nil {
		return PresentationView{}, err
	}
	state, err := a.presentation.PreviewScripture(a.context(), normalized.TranslationID,
		normalized.BookID, normalized.Chapter, normalized.VerseStart, normalized.VerseEnd)
	return presentationView(state), err
}

func (a *App) ProjectScripture(request ScriptureRequest) (PresentationView, error) {
	if a.presentation == nil {
		return PresentationView{}, errStorageUnavailable
	}
	normalized, _, err := normalizeScripture(request)
	if err != nil {
		return PresentationView{}, err
	}
	state, err := a.presentation.ProjectScripture(a.context(), normalized.TranslationID,
		normalized.BookID, normalized.Chapter, normalized.VerseStart, normalized.VerseEnd)
	return presentationView(state), err
}

func (a *App) PreviewServiceItem(itemID string) (PresentationView, error) {
	if a.presentation == nil {
		return PresentationView{}, errStorageUnavailable
	}
	state, err := a.presentation.PreviewItem(a.context(), strings.TrimSpace(itemID))
	return presentationView(state), err
}

func (a *App) SendPreviewLive() (PresentationView, error) {
	if a.presentation == nil {
		return PresentationView{}, errStorageUnavailable
	}
	state, err := a.presentation.ProjectPreview()
	return presentationView(state), err
}

func (a *App) ClearLive() (PresentationView, error) {
	if a.presentation == nil {
		return PresentationView{}, errStorageUnavailable
	}
	return presentationView(a.presentation.ClearLive()), nil
}

func (a *App) NextServiceItem() (PresentationView, error) {
	if a.presentation == nil {
		return PresentationView{}, errStorageUnavailable
	}
	state, err := a.presentation.Next(a.context())
	return presentationView(state), err
}

func (a *App) PreviousServiceItem() (PresentationView, error) {
	if a.presentation == nil {
		return PresentationView{}, errStorageUnavailable
	}
	state, err := a.presentation.Previous(a.context())
	return presentationView(state), err
}

func (a *App) serviceOrderView(ctx context.Context) (ServiceOrderView, error) {
	service, items, err := a.order.Current(ctx)
	if errors.Is(err, serviceorder.ErrNoService) {
		return ServiceOrderView{Items: []ServiceItemView{}}, nil
	}
	if err != nil {
		return ServiceOrderView{}, err
	}

	views := make([]ServiceItemView, 0, len(items))
	for _, item := range items {
		views = append(views, ServiceItemView{
			ID:            item.ID,
			Position:      item.Position,
			Kind:          string(item.Kind),
			Title:         item.Title,
			TranslationID: item.TranslationID,
			BookID:        item.BookID,
			Chapter:       item.Chapter,
			VerseStart:    item.VerseStart,
			VerseEnd:      item.VerseEnd,
		})
	}
	return ServiceOrderView{
		Service: &ServiceView{ID: service.ID, Name: service.Name},
		Items:   views,
	}, nil
}

func (a *App) ensureService(ctx context.Context) (serviceorder.Service, error) {
	service, _, err := a.order.Current(ctx)
	if errors.Is(err, serviceorder.ErrNoService) {
		return a.order.Create(ctx, "Untitled service")
	}
	return service, err
}

func normalizeScripture(request ScriptureRequest) (ScriptureRequest, string, error) {
	translationID := strings.TrimSpace(request.TranslationID)
	if translationID == "" {
		return ScriptureRequest{}, "", errors.New("translation is required")
	}
	if request.BookID <= 0 || request.Chapter <= 0 || request.VerseStart <= 0 {
		return ScriptureRequest{}, "", errors.New("book, chapter and verse are required")
	}

	verseEnd := request.VerseEnd
	if verseEnd == 0 {
		verseEnd = request.VerseStart
	}
	if verseEnd < request.VerseStart {
		return ScriptureRequest{}, "", errors.New("verse range ends before it starts")
	}
	book, ok := bible.BookByID(request.BookID)
	if !ok {
		return ScriptureRequest{}, "", fmt.Errorf("unknown book id %d", request.BookID)
	}

	title := fmt.Sprintf("%s %d:%d", book.Name, request.Chapter, request.VerseStart)
	if verseEnd > request.VerseStart {
		title = fmt.Sprintf("%s %d:%d-%d", book.Name, request.Chapter, request.VerseStart, verseEnd)
	}

	request.TranslationID = translationID
	request.VerseEnd = verseEnd
	return request, title, nil
}

func presentationView(state presentation.State) PresentationView {
	return PresentationView{Preview: state.Preview, Live: state.Live}
}
