package media

import (
	"context"
	"errors"
)

// Routes the asset server answers and the frontend is handed. They live here so
// a slide and the handler can never disagree about where a file is served from.
const (
	FileRoute  = "/media/file/"
	ThumbRoute = "/media/thumb/"
)

var (
	ErrNotImage = errors.New("asset is not an image")
	ErrFileGone = errors.New("the library copy of this asset is missing")
)

// FileURL is the address a browser would fetch the asset's own bytes from.
func FileURL(id string) string { return FileRoute + id }

// ThumbURL is the address of the generated tile for the library grid.
func ThumbURL(id string) string { return ThumbRoute + id }

// Image resolves an asset into the two things a slide needs: a caption and an
// address. It refuses rather than handing back a broken slide, so a projector
// keeps showing the last good image when a file has gone walkabout.
func (l *Library) Image(ctx context.Context, id string) (title string, url string, err error) {
	asset, err := l.repo.ByID(ctx, id)
	if err != nil {
		return "", "", err
	}
	if asset.Kind != KindImage {
		return "", "", ErrNotImage
	}
	if !l.Exists(asset) {
		return "", "", ErrFileGone
	}
	return asset.Title, FileURL(asset.ID), nil
}
