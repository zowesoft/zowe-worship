package main

import (
	"context"
	"net/http"
	"strings"
	"time"

	"zowe-worship/internal/media"

	"github.com/wailsapp/wails/v3/pkg/application"
)

const (
	mediaFileRoute  = "/media/file/"
	mediaThumbRoute = "/media/thumb/"
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
	Missing      bool      `json:"missing"`
	CreatedAt    time.Time `json:"createdAt"`
}

type MediaImportResult struct {
	Added    []MediaView           `json:"added"`
	Problems []media.ImportProblem `json:"problems"`
}

// PickMediaFiles opens the native picker so an operator can reach any folder
// without learning where Zowe keeps its own copies.
func (a *App) PickMediaFiles() ([]string, error) {
	desktop := application.Get()
	if desktop == nil {
		return nil, errNoApplication
	}

	paths, err := desktop.Dialog.OpenFile().
		SetTitle("Choose media to add").
		AddFilter("Media", "*.jpg;*.jpeg;*.png;*.gif;*.webp;*.bmp;*.mp4;*.m4v;*.mov;*.webm;*.mp3;*.m4a;*.aac;*.wav;*.ogg;*.flac").
		AddFilter("All files", "*.*").
		PromptForMultipleSelection()
	if err != nil {
		return nil, err
	}
	return paths, nil
}

func (a *App) ListMedia() ([]MediaView, error) {
	if a.media == nil {
		return nil, errStorageUnavailable
	}
	return a.mediaViews(a.context())
}

// ImportMedia copies the chosen files into the library. Files that cannot be
// taken come back explained rather than failing the whole import, because the
// usual case is a folder dropped with a mix of usable and unusable things in it.
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
			URL:          mediaFileRoute + asset.ID,
			ThumbnailURL: mediaThumbRoute + asset.ID,
			SizeBytes:    asset.SizeBytes,
			DurationMS:   asset.DurationMS,
			Width:        asset.Width,
			Height:       asset.Height,
			HasThumbnail: asset.HasThumbnail,
			Missing:      !a.media.Exists(asset),
			CreatedAt:    asset.CreatedAt,
		})
	}
	return views
}

// serveMedia hands out the library's own copies. An id is only ever resolved
// through the catalog, so no request can name a path on disk, and ServeFile
// gives the byte ranges a video needs to seek.
func (a *App) serveMedia(w http.ResponseWriter, r *http.Request) {
	if a.media == nil {
		http.Error(w, "media is not available", http.StatusServiceUnavailable)
		return
	}

	thumbnail := strings.HasPrefix(r.URL.Path, mediaThumbRoute)
	prefix := mediaFileRoute
	if thumbnail {
		prefix = mediaThumbRoute
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
