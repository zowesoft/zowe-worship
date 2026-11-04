package main

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"log"
	"strings"

	"github.com/wailsapp/wails/v3/pkg/application"
	"zowe-worship/internal/bible"
	"zowe-worship/internal/database"
	"zowe-worship/internal/detection"
	"zowe-worship/internal/media"
	"zowe-worship/internal/presentation"
	"zowe-worship/internal/scripture"
	"zowe-worship/internal/serviceorder"
	"zowe-worship/internal/settings"
	"zowe-worship/internal/songs"
	"zowe-worship/internal/themes"
)

var errStorageUnavailable = errors.New("storage is not available")

// appVersion is shown in Settings, so it is named rather than repeated.
const appVersion = "0.1.0"

type App struct {
	ctx           context.Context
	dataRoot      string
	core          *sql.DB
	bible         *bible.SQLiteRepository
	order         *serviceorder.SQLiteRepository
	songs         *songs.SQLiteRepository
	media         *media.Library
	themes        *themes.SQLiteRepository
	settings      *settings.Store
	presentation  *presentation.Controller
	detection     *detection.Queue
	display       application.Window
	displayScreen ScreenOption
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
	Offset        int    `json:"offset"`
	// Optional filters: empty string / nil means no filter.
	Testament string `json:"testament"`
	BookIDs   []int  `json:"bookIds"`
}

type BibleSearchResponse struct {
	Kind      string       `json:"kind"`
	Reference string       `json:"reference,omitempty"`
	Results   []BibleVerse `json:"results"`
	Total     int          `json:"total"`
}

// BibleBookView is a lightweight description of one canonical book exposed to
// the frontend for filter dropdowns and the passage browser.
type BibleBookView struct {
	ID           int    `json:"id"`
	Name         string `json:"name"`
	Abbreviation string `json:"abbreviation"`
	Testament    string `json:"testament"`
	Chapters     int    `json:"chapters"`
}

type BibleVerse struct {
	BookID    int    `json:"bookId"`
	Book      string `json:"book"`
	Chapter   int    `json:"chapter"`
	Verse     int    `json:"verse"`
	Reference string `json:"reference"`
	Text      string `json:"text"`
	Snippet   string `json:"snippet,omitempty"`
	Testament string `json:"testament"`
}

func NewApp() *App {
	return &App{}
}

// newAppAt lets tests point storage at a temporary directory.
func newAppAt(dataRoot string) *App {
	return &App{dataRoot: dataRoot}
}

// ServiceStartup opens storage before the studio window can call a bound
// method. A failure is logged and left for the bindings to report, rather than
// aborting the run: an operator who sees "storage is not available" in the app
// can act on it, whereas a process that exits silently cannot.
func (a *App) ServiceStartup(ctx context.Context, _ application.ServiceOptions) error {
	a.ctx = ctx

	paths, err := a.paths()
	if err != nil {
		log.Printf("zowe: storage unavailable: %v", err)
		return nil
	}
	if err := paths.Ensure(); err != nil {
		log.Printf("zowe: storage unavailable: %v", err)
		return nil
	}

	core, err := database.Open(ctx, paths.CoreDatabase())
	if err != nil {
		log.Printf("zowe: open database: %v", err)
		return nil
	}
	if err := database.Migrate(ctx, core, paths.Backups()); err != nil {
		_ = core.Close()
		log.Printf("zowe: migrate database: %v", err)
		return nil
	}

	a.core = core
	a.bible = bible.NewSQLiteRepository(core, paths.Translations())
	a.order = serviceorder.NewSQLiteRepository(core)
	a.songs = songs.NewSQLiteRepository(core)
	a.media = media.NewLibrary(media.NewSQLiteRepository(core), paths.Media(), paths.MediaThumbnails())
	a.themes = themes.NewSQLiteRepository(core)
	a.settings = settings.New(core)
	a.presentation = presentation.New(a.bible, a.order, a.songs, a.media, a.themes)
	a.detection = detection.New(core)
	return nil
}

func (a *App) ServiceShutdown() error {
	if a.core == nil {
		return nil
	}
	core := a.core
	a.core = nil
	if err := core.Close(); err != nil {
		return fmt.Errorf("close database: %w", err)
	}
	return nil
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
		Version: appVersion,
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

// ListBooks returns all 66 canonical books, ordered by canonical position.
// The result is static and does not depend on any installed translation.
func (a *App) ListBooks() []BibleBookView {
	books := bible.CanonicalBooks
	views := make([]BibleBookView, 0, len(books))
	for _, b := range books {
		views = append(views, BibleBookView{
			ID:           b.ID,
			Name:         b.Name,
			Abbreviation: b.Abbreviation,
			Testament:    string(b.Testament),
			Chapters:     b.Chapters,
		})
	}
	return views
}

// BrowsePassageRequest addresses a whole chapter or a verse range inside one.
type BrowsePassageRequest struct {
	TranslationID string `json:"translationId"`
	BookID        int    `json:"bookId"`
	Chapter       int    `json:"chapter"`
	VerseStart    int    `json:"verseStart"`
	VerseEnd      int    `json:"verseEnd"`
}

// BrowsePassage fetches a chapter (or verse range) from the given translation.
// When VerseStart is 0 the whole chapter is returned.
func (a *App) BrowsePassage(req BrowsePassageRequest) (BibleSearchResponse, error) {
	if a.bible == nil {
		return BibleSearchResponse{}, errStorageUnavailable
	}
	translationID := strings.TrimSpace(req.TranslationID)
	if translationID == "" {
		return BibleSearchResponse{}, errors.New("translation is required")
	}
	verses, err := a.bible.Passage(a.context(), translationID, req.BookID, req.Chapter, req.VerseStart, req.VerseEnd)
	if err != nil {
		return BibleSearchResponse{}, err
	}
	views := toBibleVerses(verses)
	return BibleSearchResponse{
		Kind:    "passage",
		Results: views,
		Total:   len(views),
	}, nil
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
			views := toBibleVerses(verses)
			return BibleSearchResponse{
				Kind:      "reference",
				Reference: reference.String(),
				Results:   views,
				Total:     len(views),
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
		Offset:        request.Offset,
		Testament:     bible.Testament(request.Testament),
		BookIDs:       request.BookIDs,
	})
	if err != nil {
		return BibleSearchResponse{}, err
	}
	views := toBibleVerses(results)
	return BibleSearchResponse{Kind: "text", Results: views, Total: len(views)}, nil
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
			Snippet:   result.Snippet,
			Testament: string(result.Book.Testament),
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
	SongID        string `json:"songId"`
	SectionID     string `json:"sectionId"`
	MediaID       string `json:"mediaId"`
	ThemeID       string `json:"themeId"`
	Content       string `json:"content"`
	Notes         string `json:"notes"`
	Duration      int    `json:"duration"`
	Status        string `json:"status"` // "completed" | "live" | "next" | "upcoming"
}

type ServiceOrderView struct {
	Service *ServiceView      `json:"service"`
	Items   []ServiceItemView `json:"items"`
}

type CustomItemRequest struct {
	Kind     string `json:"kind"` // "text", "announcement", "blank"
	Title    string `json:"title"`
	Content  string `json:"content"`
	Notes    string `json:"notes"`
	Duration int    `json:"duration"`
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

func (a *App) ListServices() ([]ServiceView, error) {
	if a.order == nil {
		return nil, errStorageUnavailable
	}
	list, err := a.order.List(a.context())
	if err != nil {
		return nil, err
	}

	views := make([]ServiceView, 0, len(list))
	for _, service := range list {
		views = append(views, ServiceView{ID: service.ID, Name: service.Name})
	}
	return views, nil
}

// UseService makes another saved service the one the studio works on. Live
// output is untouched: the audience keeps whatever is on air.
func (a *App) UseService(serviceID string) (ServiceOrderView, error) {
	if a.order == nil {
		return ServiceOrderView{}, errStorageUnavailable
	}
	ctx := a.context()
	if err := a.order.Use(ctx, strings.TrimSpace(serviceID)); err != nil {
		return ServiceOrderView{}, err
	}
	return a.serviceOrderView(ctx)
}

func (a *App) RenameService(serviceID, name string) (ServiceOrderView, error) {
	if a.order == nil {
		return ServiceOrderView{}, errStorageUnavailable
	}
	ctx := a.context()
	if err := a.order.Rename(ctx, serviceID, name); err != nil {
		return ServiceOrderView{}, err
	}
	return a.serviceOrderView(ctx)
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

	// One verse per slide turns a passage into one row per verse, so the operator
	// sees every verse in the order, can drop the ones they do not want, and steps
	// through the rest with Next and Previous like any other item.
	if a.oneVersePerSlide(ctx) && len(verses) > 1 {
		for _, verse := range verses {
			if _, err := a.order.AddScripture(ctx, service.ID, serviceorder.ScriptureInput{
				Title:         verse.Reference(),
				TranslationID: normalized.TranslationID,
				BookID:        normalized.BookID,
				Chapter:       verse.Chapter,
				VerseStart:    verse.Verse,
				VerseEnd:      verse.Verse,
			}); err != nil {
				return ServiceOrderView{}, err
			}
		}
		return a.serviceOrderView(ctx)
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

// ReorderServiceItems stores the whole schedule order at once, so the gap an
// operator drops a row into is the position that gets written.
func (a *App) ReorderServiceItems(itemIDs []string) (ServiceOrderView, error) {
	if a.order == nil {
		return ServiceOrderView{}, errStorageUnavailable
	}
	ctx := a.context()
	if err := a.order.Reorder(ctx, itemIDs); err != nil {
		return ServiceOrderView{}, err
	}
	return a.serviceOrderView(ctx)
}

// SetItemTheme points one schedule item at a theme, or back at its kind default
// with an empty id. The theme is checked before it is stored so a pick that has
// since been deleted cannot be saved quietly, and the change only reaches the
// room when the operator projects the item again.
func (a *App) SetItemTheme(itemID, themeID string) (ServiceOrderView, error) {
	if a.order == nil || a.themes == nil {
		return ServiceOrderView{}, errStorageUnavailable
	}
	ctx := a.context()
	id := strings.TrimSpace(themeID)

	if id != "" {
		if _, err := a.themes.Get(ctx, id); err != nil {
			if errors.Is(err, themes.ErrThemeNotFound) {
				return ServiceOrderView{}, errors.New("that theme is no longer in the list")
			}
			return ServiceOrderView{}, err
		}
	}

	if err := a.order.SetTheme(ctx, strings.TrimSpace(itemID), id); err != nil {
		return ServiceOrderView{}, err
	}
	return a.serviceOrderView(ctx)
}

func (a *App) DuplicateService(serviceID, newName string) (ServiceOrderView, error) {
	if a.order == nil {
		return ServiceOrderView{}, errStorageUnavailable
	}
	ctx := a.context()
	if _, err := a.order.Duplicate(ctx, strings.TrimSpace(serviceID), strings.TrimSpace(newName)); err != nil {
		return ServiceOrderView{}, err
	}
	return a.serviceOrderView(ctx)
}

func (a *App) AddCustomItemToService(request CustomItemRequest) (ServiceOrderView, error) {
	if a.order == nil {
		return ServiceOrderView{}, errStorageUnavailable
	}
	ctx := a.context()
	service, err := a.ensureService(ctx)
	if err != nil {
		return ServiceOrderView{}, err
	}
	if _, err := a.order.AddCustomItem(ctx, service.ID, serviceorder.CustomItemInput{
		Kind:     serviceorder.ItemKind(strings.TrimSpace(request.Kind)),
		Title:    strings.TrimSpace(request.Title),
		Content:  strings.TrimSpace(request.Content),
		Notes:    strings.TrimSpace(request.Notes),
		Duration: request.Duration,
	}); err != nil {
		return ServiceOrderView{}, err
	}
	return a.serviceOrderView(ctx)
}

func (a *App) UpdateServiceItemMeta(itemID, notes string, duration int) (ServiceOrderView, error) {
	if a.order == nil {
		return ServiceOrderView{}, errStorageUnavailable
	}
	ctx := a.context()
	if err := a.order.UpdateItemMeta(ctx, strings.TrimSpace(itemID), notes, duration); err != nil {
		return ServiceOrderView{}, err
	}
	return a.serviceOrderView(ctx)
}

// applied hands a controller result to the frontend and, when it succeeded,
// pushes the same slide to the audience display. Reading state never republish,
// so a reopening display catches up on its own.
func (a *App) applied(state presentation.State, err error) (PresentationView, error) {
	if err == nil {
		a.publishToDisplay(state)
	}
	return presentationView(state), err
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
	return a.applied(a.presentation.PreviewScripture(a.context(), normalized.TranslationID,
		normalized.BookID, normalized.Chapter, normalized.VerseStart, normalized.VerseEnd,
		a.oneVersePerSlide(a.context())))
}

func (a *App) ProjectScripture(request ScriptureRequest) (PresentationView, error) {
	if a.presentation == nil {
		return PresentationView{}, errStorageUnavailable
	}
	normalized, _, err := normalizeScripture(request)
	if err != nil {
		return PresentationView{}, err
	}
	return a.applied(a.presentation.ProjectScripture(a.context(), normalized.TranslationID,
		normalized.BookID, normalized.Chapter, normalized.VerseStart, normalized.VerseEnd,
		a.oneVersePerSlide(a.context())))
}

func (a *App) PreviewServiceItem(itemID string) (PresentationView, error) {
	if a.presentation == nil {
		return PresentationView{}, errStorageUnavailable
	}
	return a.applied(a.presentation.PreviewItem(a.context(), strings.TrimSpace(itemID)))
}

func (a *App) SendPreviewLive() (PresentationView, error) {
	if a.presentation == nil {
		return PresentationView{}, errStorageUnavailable
	}
	return a.applied(a.presentation.ProjectPreview())
}

func (a *App) ClearLive() (PresentationView, error) {
	if a.presentation == nil {
		return PresentationView{}, errStorageUnavailable
	}
	return a.applied(a.presentation.ClearLive(), nil)
}

func (a *App) NextServiceItem() (PresentationView, error) {
	if a.presentation == nil {
		return PresentationView{}, errStorageUnavailable
	}
	return a.applied(a.presentation.Next(a.context()))
}

func (a *App) PreviousServiceItem() (PresentationView, error) {
	if a.presentation == nil {
		return PresentationView{}, errStorageUnavailable
	}
	return a.applied(a.presentation.Previous(a.context()))
}

func (a *App) serviceOrderView(ctx context.Context) (ServiceOrderView, error) {
	service, items, err := a.order.Current(ctx)
	if errors.Is(err, serviceorder.ErrNoService) {
		return ServiceOrderView{Items: []ServiceItemView{}}, nil
	}
	if err != nil {
		return ServiceOrderView{}, err
	}

	liveID := ""
	previewID := ""
	if a.presentation != nil {
		state := a.presentation.State()
		if state.Live != nil {
			liveID = state.Live.ItemID
		}
		if state.Preview != nil {
			previewID = state.Preview.ItemID
		}
	}

	liveIdx := -1
	for i, item := range items {
		if liveID != "" && item.ID == liveID {
			liveIdx = i
			break
		}
	}

	views := make([]ServiceItemView, 0, len(items))
	for i, item := range items {
		var status string
		if liveID != "" && item.ID == liveID {
			status = "live"
		} else if previewID != "" && item.ID == previewID {
			status = "next"
		} else if liveIdx >= 0 && i < liveIdx {
			status = "completed"
		} else {
			status = "upcoming"
		}

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
			SongID:        item.SongID,
			SectionID:     item.SectionID,
			MediaID:       item.MediaID,
			ThemeID:       item.ThemeID,
			Content:       item.Content,
			Notes:         item.Notes,
			Duration:      item.Duration,
			Status:        status,
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

type SongSectionView struct {
	ID       string `json:"id"`
	Label    string `json:"label"`
	Text     string `json:"text"`
	Position int    `json:"position"`
}

type ArrangementItemView struct {
	ID        string `json:"id"`
	SectionID string `json:"sectionId"`
	Position  int    `json:"position"`
	Label     string `json:"label"`
	Text      string `json:"text"`
}

type ArrangementView struct {
	ID        string                `json:"id"`
	SongID    string                `json:"songId"`
	Name      string                `json:"name"`
	IsDefault bool                  `json:"isDefault"`
	Items     []ArrangementItemView `json:"items"`
}

type SongView struct {
	ID           string            `json:"id"`
	Title        string            `json:"title"`
	Author       string            `json:"author"`
	Copyright    string            `json:"copyright"`
	CCLINumber   string            `json:"ccliNumber"`
	Source       string            `json:"source"`
	Sections     []SongSectionView `json:"sections"`
	Arrangements []ArrangementView `json:"arrangements"`
}

type LyricHitView struct {
	SongID     string `json:"songId"`
	SongTitle  string `json:"songTitle"`
	Author     string `json:"author"`
	Copyright  string `json:"copyright"`
	CCLINumber string `json:"ccliNumber"`
	Source     string `json:"source"`
	SectionID  string `json:"sectionId"`
	Label      string `json:"label"`
	Text       string `json:"text"`
	Position   int    `json:"position"`
}

type SongsView struct {
	Songs []SongView     `json:"songs"`
	Hits  []LyricHitView `json:"hits"`
}

type NewSongSectionRequest struct {
	Label string `json:"label"`
	Text  string `json:"text"`
}

type NewSongRequest struct {
	Title      string                  `json:"title"`
	Author     string                  `json:"author"`
	Copyright  string                  `json:"copyright"`
	CCLINumber string                  `json:"ccliNumber"`
	Source     string                  `json:"source"`
	Sections   []NewSongSectionRequest `json:"sections"`
}

type SongSectionRequest struct {
	SongID    string `json:"songId"`
	SectionID string `json:"sectionId"`
}

type SongArrangementRequest struct {
	SongID        string `json:"songId"`
	ArrangementID string `json:"arrangementId"`
}

type SaveArrangementRequest struct {
	ArrangementID string   `json:"arrangementId"`
	SongID        string   `json:"songId"`
	Name          string   `json:"name"`
	IsDefault     bool     `json:"isDefault"`
	SectionIDs    []string `json:"sectionIds"`
}

func (a *App) ListSongs(query string) (SongsView, error) {
	if a.songs == nil {
		return SongsView{}, errStorageUnavailable
	}
	ctx := a.context()

	if strings.TrimSpace(query) == "" {
		return a.listSongs(ctx)
	}

	hits, err := a.songs.Search(ctx, query, 50)
	if errors.Is(err, songs.ErrEmptyQuery) {
		return a.listSongs(ctx)
	}
	if err != nil {
		return SongsView{}, err
	}

	views := make([]LyricHitView, 0, len(hits))
	for _, hit := range hits {
		views = append(views, LyricHitView{
			SongID:     hit.Song.ID,
			SongTitle:  hit.Song.Title,
			Author:     hit.Song.Author,
			Copyright:  hit.Song.Copyright,
			CCLINumber: hit.Song.CCLINumber,
			Source:     hit.Song.Source,
			SectionID:  hit.Section.ID,
			Label:      hit.Section.Label,
			Text:       hit.Section.Text,
			Position:   hit.Section.Position,
		})
	}
	return SongsView{Hits: views}, nil
}

func (a *App) CreateSong(request NewSongRequest) (SongsView, error) {
	if a.songs == nil {
		return SongsView{}, errStorageUnavailable
	}

	sections := make([]songs.NewSection, 0, len(request.Sections))
	for _, section := range request.Sections {
		sections = append(sections, songs.NewSection{Label: section.Label, Text: section.Text})
	}

	if _, err := a.songs.Create(a.context(), songs.NewSong{
		Title:      request.Title,
		Author:     request.Author,
		Copyright:  request.Copyright,
		CCLINumber: request.CCLINumber,
		Source:     request.Source,
		Sections:   sections,
	}); err != nil {
		return SongsView{}, err
	}
	return a.listSongs(a.context())
}

func (a *App) DeleteSong(songID string) (SongsView, error) {
	if a.songs == nil {
		return SongsView{}, errStorageUnavailable
	}
	if err := a.songs.Delete(a.context(), strings.TrimSpace(songID)); err != nil {
		return SongsView{}, err
	}
	return a.listSongs(a.context())
}

func (a *App) DuplicateSong(songID string) (SongsView, error) {
	if a.songs == nil {
		return SongsView{}, errStorageUnavailable
	}
	ctx := a.context()
	if _, err := a.songs.Duplicate(ctx, strings.TrimSpace(songID), ""); err != nil {
		return SongsView{}, err
	}
	return a.listSongs(ctx)
}

type SongSectionEdit struct {
	ID    string `json:"id"`
	Label string `json:"label"`
	Text  string `json:"text"`
}

type EditSongRequest struct {
	ID         string            `json:"id"`
	Title      string            `json:"title"`
	Author     string            `json:"author"`
	Copyright  string            `json:"copyright"`
	CCLINumber string            `json:"ccliNumber"`
	Source     string            `json:"source"`
	Sections   []SongSectionEdit `json:"sections"`
}

// songEditorWindow names the song editor so reopening it for another song moves
// the same window rather than stacking a second one.
const songEditorWindow = "song-editor"

// OpenSongEditor edits one song in its own window, or starts a new one away from
// the studio.
func (a *App) OpenSongEditor(songID string) (EditorView, error) {
	if a.songs == nil {
		return EditorView{}, errStorageUnavailable
	}

	id := strings.TrimSpace(songID)
	if id == "" {
		return openUtilityWindow(songEditorWindow, "New song", "/?view=song-editor", 720, 660, 560, 520)
	}

	song, err := a.songs.Get(a.context(), id)
	if err != nil {
		return EditorView{}, err
	}
	return openUtilityWindow(songEditorWindow,
		song.Title+" — Edit song",
		"/?view=song-editor&song="+id,
		720, 660, 560, 520,
	)
}

// UpdateSong rewrites one song in the library. Sections keep their ids, so a
// slide already scheduled from one still resolves afterwards.
func (a *App) UpdateSong(request EditSongRequest) (SongsView, error) {
	if a.songs == nil {
		return SongsView{}, errStorageUnavailable
	}
	ctx := a.context()

	sections := make([]songs.SectionUpdate, 0, len(request.Sections))
	for _, section := range request.Sections {
		sections = append(sections, songs.SectionUpdate{
			ID: section.ID, Label: section.Label, Text: section.Text,
		})
	}

	if _, err := a.songs.Update(ctx, strings.TrimSpace(request.ID), songs.SongUpdate{
		Title:      request.Title,
		Author:     request.Author,
		Copyright:  request.Copyright,
		CCLINumber: request.CCLINumber,
		Source:     request.Source,
		Sections:   sections,
	}); err != nil {
		return SongsView{}, err
	}
	return a.listSongs(ctx)
}

func (a *App) PreviewSongSection(request SongSectionRequest) (PresentationView, error) {
	if a.presentation == nil {
		return PresentationView{}, errStorageUnavailable
	}
	songID, sectionID, err := requireSection(request)
	if err != nil {
		return PresentationView{}, err
	}
	state, err := a.presentation.PreviewSongSection(a.context(), songID, sectionID)
	return a.applied(state, err)
}

func (a *App) ProjectSongSection(request SongSectionRequest) (PresentationView, error) {
	if a.presentation == nil {
		return PresentationView{}, errStorageUnavailable
	}
	songID, sectionID, err := requireSection(request)
	if err != nil {
		return PresentationView{}, err
	}
	state, err := a.presentation.ProjectSongSection(a.context(), songID, sectionID)
	return a.applied(state, err)
}

func (a *App) AddSongSectionToService(request SongSectionRequest) (ServiceOrderView, error) {
	if a.songs == nil || a.order == nil {
		return ServiceOrderView{}, errStorageUnavailable
	}

	ctx := a.context()
	songID, sectionID, err := requireSection(request)
	if err != nil {
		return ServiceOrderView{}, err
	}
	song, section, err := a.songs.Section(ctx, songID, sectionID)
	if err != nil {
		return ServiceOrderView{}, err
	}

	title := song.Title
	if section.Label != "" {
		title = song.Title + " · " + section.Label
	}
	service, err := a.ensureService(ctx)
	if err != nil {
		return ServiceOrderView{}, err
	}
	if _, err := a.order.AddSong(ctx, service.ID, serviceorder.SongInput{
		Title:     title,
		SongID:    song.ID,
		SectionID: section.ID,
	}); err != nil {
		return ServiceOrderView{}, err
	}
	return a.serviceOrderView(ctx)
}

func (a *App) AddSongArrangementToService(request SongArrangementRequest) (ServiceOrderView, error) {
	if a.songs == nil || a.order == nil {
		return ServiceOrderView{}, errStorageUnavailable
	}
	ctx := a.context()
	songID := strings.TrimSpace(request.SongID)
	arrID := strings.TrimSpace(request.ArrangementID)
	if songID == "" || arrID == "" {
		return ServiceOrderView{}, errors.New("song id and arrangement id are required")
	}

	song, err := a.songs.Get(ctx, songID)
	if err != nil {
		return ServiceOrderView{}, err
	}
	arr, err := a.songs.GetArrangement(ctx, arrID)
	if err != nil {
		return ServiceOrderView{}, err
	}

	service, err := a.ensureService(ctx)
	if err != nil {
		return ServiceOrderView{}, err
	}

	for _, item := range arr.Items {
		title := song.Title
		if item.Label != "" {
			title = song.Title + " · " + item.Label
		}
		if _, err := a.order.AddSong(ctx, service.ID, serviceorder.SongInput{
			Title:     title,
			SongID:    song.ID,
			SectionID: item.SectionID,
		}); err != nil {
			return ServiceOrderView{}, err
		}
	}
	return a.serviceOrderView(ctx)
}

func (a *App) SaveSongArrangement(request SaveArrangementRequest) (ArrangementView, error) {
	if a.songs == nil {
		return ArrangementView{}, errStorageUnavailable
	}
	ctx := a.context()
	if strings.TrimSpace(request.ArrangementID) == "" {
		created, err := a.songs.CreateArrangement(ctx, songs.NewArrangement{
			SongID:     strings.TrimSpace(request.SongID),
			Name:       request.Name,
			IsDefault:  request.IsDefault,
			SectionIDs: request.SectionIDs,
		})
		if err != nil {
			return ArrangementView{}, err
		}
		return toArrangementView(created), nil
	}

	updated, err := a.songs.UpdateArrangement(ctx, strings.TrimSpace(request.ArrangementID), songs.ArrangementUpdate{
		Name:       request.Name,
		IsDefault:  request.IsDefault,
		SectionIDs: request.SectionIDs,
	})
	if err != nil {
		return ArrangementView{}, err
	}
	return toArrangementView(updated), nil
}

func (a *App) DeleteSongArrangement(arrangementID string) error {
	if a.songs == nil {
		return errStorageUnavailable
	}
	return a.songs.DeleteArrangement(a.context(), strings.TrimSpace(arrangementID))
}

func toArrangementView(arr songs.Arrangement) ArrangementView {
	items := make([]ArrangementItemView, 0, len(arr.Items))
	for _, item := range arr.Items {
		items = append(items, ArrangementItemView{
			ID:        item.ID,
			SectionID: item.SectionID,
			Position:  item.Position,
			Label:     item.Label,
			Text:      item.Text,
		})
	}
	return ArrangementView{
		ID:        arr.ID,
		SongID:    arr.SongID,
		Name:      arr.Name,
		IsDefault: arr.IsDefault,
		Items:     items,
	}
}

func (a *App) listSongs(ctx context.Context) (SongsView, error) {
	list, err := a.songs.List(ctx)
	if err != nil {
		return SongsView{}, err
	}

	songsView := make([]SongView, 0, len(list))
	for _, entry := range list {
		sections := make([]SongSectionView, 0, len(entry.Sections))
		for _, section := range entry.Sections {
			sections = append(sections, SongSectionView{
				ID:       section.ID,
				Label:    section.Label,
				Text:     section.Text,
				Position: section.Position,
			})
		}
		arrangements := make([]ArrangementView, 0, len(entry.Arrangements))
		for _, arr := range entry.Arrangements {
			arrangements = append(arrangements, toArrangementView(arr))
		}
		songsView = append(songsView, SongView{
			ID:           entry.ID,
			Title:        entry.Title,
			Author:       entry.Author,
			Copyright:    entry.Copyright,
			CCLINumber:   entry.CCLINumber,
			Source:       entry.Source,
			Sections:     sections,
			Arrangements: arrangements,
		})
	}
	return SongsView{Songs: songsView}, nil
}

func requireSection(request SongSectionRequest) (string, string, error) {
	songID := strings.TrimSpace(request.SongID)
	sectionID := strings.TrimSpace(request.SectionID)
	if songID == "" || sectionID == "" {
		return "", "", errors.New("song and section are required")
	}
	return songID, sectionID, nil
}
