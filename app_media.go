package main

import (
	"context"
	"fmt"
	"net/http"
	"strings"
	"time"

	"zowe-worship/internal/media"
	"zowe-worship/internal/serviceorder"

	"github.com/wailsapp/wails/v3/pkg/application"
)

type MediaView struct {
	ID           string    `json:"id"`
	Kind         string    `json:"kind"`
	Title        string    `json:"title"`
	URL          string    `json:"url"`
	ThumbnailURL string    `json:"thumbnailUrl"`
	SizeBytes    int64     `json:"sizeBytes"`
	DurationMS   int64     `json:"durationMs"`
	Width        int       `json:"width"`
	Height       int       `json:"height"`
	HasThumbnail bool      `json:"hasThumbnail"`
	Aspect       string    `json:"aspect"`
	Fit          string    `json:"fit"`
	Missing      bool      `json:"missing"`
	CreatedAt    time.Time `json:"createdAt"`
}

type MediaImportResult struct {
	Added    []MediaView           `json:"added"`
	Problems []media.ImportProblem `json:"problems"`
}

// mediaFilter is shared so both pickers offer exactly what the importer enforces.
const mediaFilter = "*.jpg;*.jpeg;*.png;*.gif;*.webp;*.bmp;*.mp4;*.m4v;*.mov;*.webm;*.mp3;*.m4a;*.aac;*.wav;*.ogg;*.flac"

// PickMediaFiles opens the native picker, so an operator never needs to know
// where the library keeps its copies.
func (a *App) PickMediaFiles() ([]string, error) {
	desktop := application.Get()
	if desktop == nil {
		return nil, errNoApplication
	}

	paths, err := desktop.Dialog.OpenFile().
		SetTitle("Choose media to add").
		AddFilter("Media", mediaFilter).
		AddFilter("All files", "*.*").
		PromptForMultipleSelection()
	if err != nil {
		return nil, err
	}
	return paths, nil
}

func (a *App) PickMediaFile() (string, error) {
	desktop := application.Get()
	if desktop == nil {
		return "", errNoApplication
	}

	path, err := desktop.Dialog.OpenFile().
		SetTitle("Find the file again").
		AddFilter("Media", mediaFilter).
		AddFilter("All files", "*.*").
		PromptForSingleSelection()
	if err != nil {
		return "", err
	}
	// A cancelled dialog comes back empty, which the browser cleans to ".".
	if path == "" || path == "." {
		return "", nil
	}
	return path, nil
}

func (a *App) ListMedia() ([]MediaView, error) {
	if a.media == nil {
		return nil, errStorageUnavailable
	}
	return a.mediaViews(a.context())
}

// ImportMedia reports unusable files individually rather than failing the whole
// import, because a dropped folder usually mixes usable and unusable things.
func (a *App) ImportMedia(paths []string) (MediaImportResult, error) {
	if a.media == nil {
		return MediaImportResult{}, errStorageUnavailable
	}
	ctx := a.context()

	imported, problems, err := a.media.Import(ctx, paths)
	if err != nil {
		return MediaImportResult{}, err
	}

	return MediaImportResult{
		Added:    a.assetViews(imported),
		Problems: problems,
	}, nil
}

func (a *App) DeleteMedia(id string) ([]MediaView, error) {
	if a.media == nil {
		return nil, errStorageUnavailable
	}
	ctx := a.context()

	if err := a.media.Delete(ctx, strings.TrimSpace(id)); err != nil {
		return nil, err
	}
	return a.mediaViews(ctx)
}

func (a *App) RelinkMedia(mediaID, path string) ([]MediaView, error) {
	if a.media == nil {
		return nil, errStorageUnavailable
	}
	ctx := a.context()

	if _, err := a.media.Relink(ctx, strings.TrimSpace(mediaID), path); err != nil {
		return nil, err
	}
	return a.mediaViews(ctx)
}

// AddMediaToService references the asset rather than copying it, so one file can
// serve two services.
func (a *App) AddMediaToService(mediaID string) (ServiceOrderView, error) {
	if a.media == nil || a.order == nil {
		return ServiceOrderView{}, errStorageUnavailable
	}
	ctx := a.context()
	id := strings.TrimSpace(mediaID)

	slide, err := a.media.Slide(ctx, id)
	if err != nil {
		return ServiceOrderView{}, err
	}
	service, err := a.ensureService(ctx)
	if err != nil {
		return ServiceOrderView{}, err
	}
	if _, err := a.order.AddMedia(ctx, service.ID, serviceorder.MediaInput{
		Title: slide.Title, MediaID: id,
	}); err != nil {
		return ServiceOrderView{}, err
	}
	return a.serviceOrderView(ctx)
}

// mediaEditorWindow doubles as the reuse key: reopening the editor for another
// image moves the existing window.
const mediaEditorWindow = "media-editor"

// OpenMediaEditor keeps framing in its own small window, so the studio stays a
// production view.
func (a *App) OpenMediaEditor(mediaID string) (EditorView, error) {
	if a.media == nil {
		return EditorView{}, errStorageUnavailable
	}

	id := strings.TrimSpace(mediaID)
	slide, err := a.media.Slide(a.context(), id)
	if err != nil {
		return EditorView{}, err
	}
	if slide.Kind != string(media.KindImage) {
		return EditorView{}, media.ErrNotImage
	}

	return openUtilityWindow(
		mediaEditorWindow,
		fmt.Sprintf("%s — Edit image", slide.Title),
		fmt.Sprintf("/?view=media-editor&media=%s", id),
		720, 600, 560, 480,
	)
}

// MediaAspects comes from the backend so the chips, the stored values and the
// renderer ratio are one list rather than three.
func (a *App) MediaAspects() []media.Aspect {
	return media.Aspects()
}

func (a *App) GetMediaPresentation(mediaID string) (media.Presentation, error) {
	if a.media == nil {
		return media.Presentation{}, errStorageUnavailable
	}
	return a.media.Presentation(a.context(), strings.TrimSpace(mediaID))
}

func (a *App) SaveMediaPresentation(mediaID string, presentation media.Presentation) (media.Slide, error) {
	if a.media == nil {
		return media.Slide{}, errStorageUnavailable
	}
	return a.media.SetPresentation(a.context(), strings.TrimSpace(mediaID), presentation)
}

func (a *App) PreviewMedia(mediaID string) (PresentationView, error) {
	if a.presentation == nil {
		return PresentationView{}, errStorageUnavailable
	}
	return a.applied(a.presentation.PreviewMedia(a.context(), strings.TrimSpace(mediaID)))
}

func (a *App) ProjectMedia(mediaID string) (PresentationView, error) {
	if a.presentation == nil {
		return PresentationView{}, errStorageUnavailable
	}
	return a.projected(a.presentation.ProjectMedia(a.context(), strings.TrimSpace(mediaID)))
}

func (a *App) mediaViews(ctx context.Context) ([]MediaView, error) {
	assets, err := a.media.List(ctx)
	if err != nil {
		return nil, err
	}
	return a.assetViews(assets), nil
}

func (a *App) assetViews(assets []media.Asset) []MediaView {
	views := make([]MediaView, 0, len(assets))
	for _, asset := range assets {
		views = append(views, MediaView{
			ID:           asset.ID,
			Kind:         string(asset.Kind),
			Title:        asset.Title,
			URL:          media.FileURL(asset.ID),
			ThumbnailURL: media.ThumbURL(asset.ID),
			SizeBytes:    asset.SizeBytes,
			DurationMS:   asset.DurationMS,
			Width:        asset.Width,
			Height:       asset.Height,
			HasThumbnail: asset.HasThumbnail,
			Aspect:       asset.Aspect,
			Fit:          asset.Fit,
			Missing:      !a.media.Exists(asset),
			CreatedAt:    asset.CreatedAt,
		})
	}
	return views
}

// serveMedia resolves ids through the catalog, so no request can name a path on
// disk. ServeFile gives video the byte ranges it needs to seek.
func (a *App) serveMedia(w http.ResponseWriter, r *http.Request) {
	if a.media == nil {
		http.Error(w, "media is not available", http.StatusServiceUnavailable)
		return
	}

	thumbnail := strings.HasPrefix(r.URL.Path, media.ThumbRoute)
	prefix := media.FileRoute
	if thumbnail {
		prefix = media.ThumbRoute
	}

	id := strings.TrimPrefix(r.URL.Path, prefix)
	if !isAssetID(id) {
		http.NotFound(w, r)
		return
	}

	asset, err := a.media.ByID(a.context(), id)
	if err != nil {
		http.NotFound(w, r)
		return
	}
	if thumbnail && !asset.HasThumbnail {
		http.NotFound(w, r)
		return
	}

	target := a.media.Path(asset)
	if thumbnail {
		target = a.media.ThumbnailPath(asset.ID)
	}
	// A re-import can replace these bytes, and the library is small enough that
	// re-reading a tile is cheaper than reasoning about staleness mid-service.
	w.Header().Set("Cache-Control", "no-cache")
	http.ServeFile(w, r, target)
}

func isAssetID(id string) bool {
	if len(id) != 32 {
		return false
	}
	return strings.IndexFunc(id, func(r rune) bool {
		return !((r >= '0' && r <= '9') || (r >= 'a' && r <= 'f'))
	}) < 0
}
