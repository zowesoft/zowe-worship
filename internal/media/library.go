package media

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"image"
	"image/jpeg"
	"io"
	"os"
	"path/filepath"
	"strings"
	"time"

	"golang.org/x/image/bmp"
	"golang.org/x/image/draw"
	xwebp "golang.org/x/image/webp"

	_ "image/gif"
	_ "image/jpeg"
	_ "image/png"

	"zopresent/internal/database"
)

// Tile sizes keep the library grid predictable without shipping a different
// thumbnail per source resolution.
const (
	thumbMaxWidth  = 480
	thumbMaxHeight = 270
)

// Library owns the copies of imported files and their thumbnails. The database
// only ever stores the name inside dir, so the whole folder can be moved by
// changing Paths.
type Library struct {
	dir       string
	thumbsDir string
	repo      Repository
}

func NewLibrary(repo Repository, dir, thumbsDir string) *Library {
	return &Library{dir: dir, thumbsDir: thumbsDir, repo: repo}
}

// Import copies each chosen file into the library. It never fails wholesale:
// the assets that landed come back alongside one problem per rejected file, so a
// volunteer who drops a folder of mixed things still gets the usable parts.
func (l *Library) Import(ctx context.Context, paths []string) ([]Asset, []ImportProblem, error) {
	chosen := make([]string, 0, len(paths))
	for _, path := range paths {
		if trimmed := strings.TrimSpace(path); trimmed != "" {
			chosen = append(chosen, trimmed)
		}
	}
	if len(chosen) == 0 {
		return nil, nil, ErrEmptyImport
	}

	imported := make([]Asset, 0, len(chosen))
	problems := make([]ImportProblem, 0)
	for _, path := range chosen {
		asset, err := l.importOne(ctx, path)
		switch {
		case errors.Is(err, errDuplicate):
			problems = append(problems, ImportProblem{
				Path: path, Reason: "already in the library", Duplicate: true,
			})
		case errors.Is(err, ErrUnsupported):
			problems = append(problems, ImportProblem{Path: path, Reason: "unsupported format"})
		case errors.Is(err, ErrTooLarge):
			problems = append(problems, ImportProblem{Path: path, Reason: "larger than 2 GB"})
		case err != nil:
			problems = append(problems, ImportProblem{Path: path, Reason: err.Error()})
		default:
			imported = append(imported, asset)
		}
	}
	return imported, problems, nil
}

var errDuplicate = errors.New("duplicate")

func (l *Library) importOne(ctx context.Context, path string) (Asset, error) {
	kind := KindFor(path)
	if kind == "" {
		return Asset{}, ErrUnsupported
	}

	info, err := os.Stat(path)
	if err != nil {
		return Asset{}, fmt.Errorf("read file: %w", err)
	}
	if info.IsDir() || info.Size() == 0 {
		return Asset{}, ErrUnsupported
	}
	if info.Size() > maxAssetBytes {
		return Asset{}, ErrTooLarge
	}

	checksum, err := checksumFile(path)
	if err != nil {
		return Asset{}, err
	}
	if _, found, err := l.repo.ByChecksum(ctx, checksum); err != nil {
		return Asset{}, err
	} else if found {
		return Asset{}, errDuplicate
	}

	id, err := database.NewID()
	if err != nil {
		return Asset{}, err
	}
	stored := id + strings.ToLower(filepath.Ext(path))
	destination := filepath.Join(l.dir, stored)

	if err := copyFile(path, destination); err != nil {
		return Asset{}, err
	}

	asset := Asset{
		ID:           id,
		Kind:         kind,
		Title:        strings.TrimSuffix(filepath.Base(path), filepath.Ext(path)),
		Filename:     stored,
		SourcePath:   path,
		Checksum:     checksum,
		SizeBytes:    info.Size(),
		CreatedAt:    time.Now().UTC().Truncate(time.Second),
		Presentation: FullFrame(),
	}

	switch kind {
	case KindImage:
		width, height, thumbnailErr := l.writeThumbnail(asset)
		if thumbnailErr == nil {
			asset.Width, asset.Height, asset.HasThumbnail = width, height, true
		}
	case KindVideo, KindAudio:
		asset.DurationMS = durationMS(destination, kind)
	}

	if err := l.repo.Add(ctx, asset); err != nil {
		_ = l.removeFiles(asset)
		return Asset{}, err
	}
	return asset, nil
}

// ByID looks one asset up by the id this package generated.
func (l *Library) ByID(ctx context.Context, id string) (Asset, error) {
	return l.repo.ByID(ctx, id)
}

// List returns the catalog newest first, which is the order an operator most
// often wants: what was just imported is what is about to be used.
func (l *Library) List(ctx context.Context) ([]Asset, error) {
	return l.repo.List(ctx)
}

func (l *Library) Exists(asset Asset) bool {
	_, err := os.Stat(l.Path(asset))
	return err == nil
}

// Delete drops the record and the files it owns. A missing file is not an error:
// the point of deleting is to end up with nothing left. Anything that used the
// asset as its background is undressed first, so no slide keeps a dangling id.
func (l *Library) Delete(ctx context.Context, id string) error {
	asset, err := l.repo.ByID(ctx, id)
	if err != nil {
		return err
	}
	if err := l.repo.ClearBackgroundReferences(ctx, id); err != nil {
		return err
	}
	if err := l.repo.Remove(ctx, id); err != nil {
		return err
	}
	return l.removeFiles(asset)
}

func (l *Library) removeFiles(asset Asset) error {
	thumbErr := os.Remove(l.ThumbnailPath(asset.ID))
	fileErr := os.Remove(l.Path(asset))
	if fileErr != nil && !errors.Is(fileErr, os.ErrNotExist) {
		return fileErr
	}
	if thumbErr != nil && !errors.Is(thumbErr, os.ErrNotExist) {
		return thumbErr
	}
	return nil
}

// Path resolves an asset to the file the library owns. The stored name is one
// this package generated, so it can never escape the media directory.
func (l *Library) Path(asset Asset) string {
	return filepath.Join(l.dir, filepath.Base(asset.Filename))
}

func (l *Library) ThumbnailPath(id string) string {
	return filepath.Join(l.thumbsDir, filepath.Base(id)+".jpg")
}

// Missing reports the assets whose stored files have gone. Deleting them from
// the library is the operator's call, because the file may be on a drive that is
// simply unplugged.
func (l *Library) Missing(ctx context.Context) ([]Asset, error) {
	assets, err := l.repo.List(ctx)
	if err != nil {
		return nil, err
	}

	missing := make([]Asset, 0)
	for _, asset := range assets {
		if _, err := os.Stat(l.Path(asset)); err != nil {
			missing = append(missing, asset)
		}
	}
	return missing, nil
}

// Relink adopts a new copy of the file behind an asset whose library copy has
// gone. The candidate is hashed and compared with the checksum taken at import,
// so a similarly named but different file cannot quietly take the place a
// service was built around.
func (l *Library) Relink(ctx context.Context, id, sourcePath string) (Asset, error) {
	asset, err := l.repo.ByID(ctx, id)
	if err != nil {
		return Asset{}, err
	}

	source := strings.TrimSpace(sourcePath)
	if source == "" {
		return Asset{}, ErrEmptyImport
	}
	info, err := os.Stat(source)
	if err != nil {
		return Asset{}, fmt.Errorf("read file: %w", err)
	}
	if info.IsDir() || info.Size() == 0 {
		return Asset{}, ErrUnsupported
	}
	if info.Size() > maxAssetBytes {
		return Asset{}, ErrTooLarge
	}

	checksum, err := checksumFile(source)
	if err != nil {
		return Asset{}, err
	}
	if checksum != asset.Checksum {
		return Asset{}, ErrNotTheSameFile
	}

	if !l.Exists(asset) {
		if err := copyFile(source, l.Path(asset)); err != nil {
			return Asset{}, err
		}
		// The thumbnail went with the file, so draw it again rather than leave a
		// tile blank for a picture that is back on the disk.
		if asset.Kind == KindImage {
			if width, height, thumbnailErr := l.writeThumbnail(asset); thumbnailErr == nil {
				asset.Width, asset.Height, asset.HasThumbnail = width, height, true
			}
		}
	}

	asset.SourcePath = source
	if err := l.repo.SetRestored(ctx, asset); err != nil {
		return Asset{}, err
	}
	return asset, nil
}

func (l *Library) writeThumbnail(asset Asset) (int, int, error) {
	source, err := decodeImage(l.Path(asset))
	if err != nil {
		return 0, 0, err
	}
	bounds := source.Bounds()

	scale := float64(bounds.Dx()) / thumbMaxWidth
	if float64(bounds.Dy())/thumbMaxHeight > scale {
		scale = float64(bounds.Dy()) / thumbMaxHeight
	}
	if scale < 1 {
		scale = 1
	}
	target := image.Rect(0, 0, int(float64(bounds.Dx())/scale), int(float64(bounds.Dy())/scale))

	canvas := image.NewRGBA(target)
	draw.CatmullRom.Scale(canvas, target, source, bounds, draw.Over, nil)

	file, err := os.Create(l.ThumbnailPath(asset.ID))
	if err != nil {
		return 0, 0, err
	}
	defer file.Close()
	if err := jpeg.Encode(file, canvas, &jpeg.Options{Quality: 72}); err != nil {
		return 0, 0, err
	}
	return bounds.Dx(), bounds.Dy(), nil
}

func (l *Library) ListPlaylists(ctx context.Context) ([]Playlist, error) {
	return l.repo.ListPlaylists(ctx)
}

func (l *Library) PlaylistByID(ctx context.Context, id string) (Playlist, error) {
	return l.repo.PlaylistByID(ctx, id)
}

// SavePlaylist stores a run only if it could actually play: it needs a name,
// and every item has to be a clip. Images are refused because the run
// advances when a clip ends - an image never ends, so the room would sit on
// it forever. A file that is merely missing is allowed through so a playlist
// can still be reordered while its clip is away being relinked; playing
// refuses it at the point it matters.
func (l *Library) SavePlaylist(ctx context.Context, playlist Playlist) error {
	playlist.Name = strings.TrimSpace(playlist.Name)
	if playlist.Name == "" {
		return ErrPlaylistName
	}
	for _, id := range playlist.Items {
		asset, err := l.repo.ByID(ctx, id)
		if err != nil {
			return err
		}
		if asset.Kind == KindImage {
			return fmt.Errorf("%w: %s", ErrPlaylistImage, asset.Title)
		}
	}
	return l.repo.SavePlaylist(ctx, playlist)
}

func (l *Library) DeletePlaylist(ctx context.Context, id string) error {
	return l.repo.DeletePlaylist(ctx, id)
}

func decodeImage(path string) (image.Image, error) {
	file, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer file.Close()

	switch strings.ToLower(filepath.Ext(path)) {
	case ".webp":
		return xwebp.Decode(file)
	case ".bmp":
		return bmp.Decode(file)
	default:
		source, _, err := image.Decode(file)
		return source, err
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
		return "", fmt.Errorf("hash media file: %w", err)
	}
	return hex.EncodeToString(hash.Sum(nil)), nil
}

func copyFile(source, destination string) error {
	from, err := os.Open(source)
	if err != nil {
		return err
	}
	defer from.Close()

	to, err := os.OpenFile(destination, os.O_WRONLY|os.O_CREATE|os.O_EXCL, 0o600)
	if err != nil {
		return fmt.Errorf("create library file: %w", err)
	}

	_, copyErr := io.Copy(to, from)
	closeErr := to.Close()
	if copyErr != nil {
		_ = os.Remove(destination)
		return fmt.Errorf("copy media file: %w", copyErr)
	}
	if closeErr != nil {
		_ = os.Remove(destination)
		return fmt.Errorf("copy media file: %w", closeErr)
	}
	return nil
}
