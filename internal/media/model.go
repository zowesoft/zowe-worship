package media

import (
	"errors"
	"path/filepath"
	"strings"
	"time"
)

var (
	ErrEmptyImport   = errors.New("no files were chosen")
	ErrUnsupported   = errors.New("unsupported media format")
	ErrTooLarge      = errors.New("media file is too large")
	ErrAssetNotFound = errors.New("media asset not found")
)

type Kind string

const (
	KindImage Kind = "image"
	KindVideo Kind = "video"
	KindAudio Kind = "audio"
)

// maxAssetBytes keeps a mistaken drop of a feature-length raw clip from filling
// the user's disk; 2 GiB is well past anything a service slide needs.
const maxAssetBytes = 2 << 30

// extensions maps what the app will accept to its kind. The list is what
// WebView2 decodes on Windows, so anything missing here is genuinely unplayable
// rather than merely untested: notably .mkv and .avi are left out because
// Chromium refuses them regardless of the codec inside.
var extensions = map[string]Kind{
	".jpg":  KindImage,
	".jpeg": KindImage,
	".png":  KindImage,
	".gif":  KindImage,
	".webp": KindImage,
	".bmp":  KindImage,
	".mp4":  KindVideo,
	".m4v":  KindVideo,
	".mov":  KindVideo,
	".webm": KindVideo,
	".mp3":  KindAudio,
	".m4a":  KindAudio,
	".aac":  KindAudio,
	".wav":  KindAudio,
	".ogg":  KindAudio,
	".flac": KindAudio,
}

// KindFor reports the media kind of a filename, or an empty kind when the
// extension is not one the app can play.
func KindFor(path string) Kind {
	return extensions[strings.ToLower(filepath.Ext(path))]
}

type Asset struct {
	ID           string
	Kind         Kind
	Title        string
	Filename     string
	SourcePath   string
	Checksum     string
	SizeBytes    int64
	DurationMS   int64
	Width        int
	Height       int
	HasThumbnail bool
	CreatedAt    time.Time
}

// ImportProblem explains one file the library did not take. Duplicates are
// reported here too: they are a normal outcome of re-dropping a folder, not an
// error worth an alarm.
type ImportProblem struct {
	Path      string `json:"path"`
	Reason    string `json:"reason"`
	Duplicate bool   `json:"duplicate"`
}
