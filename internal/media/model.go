package media

import (
	"errors"
	"path/filepath"
	"strings"
	"time"
)

var (
	ErrEmptyImport    = errors.New("no files were chosen")
	ErrUnsupported    = errors.New("unsupported media format")
	ErrTooLarge       = errors.New("media file is too large")
	ErrAssetNotFound  = errors.New("media asset not found")
	ErrNotTheSameFile = errors.New("that file is not the one the library holds a record of")

	ErrPlaylistNotFound = errors.New("media playlist not found")
	ErrPlaylistName     = errors.New("a playlist needs a name")
	ErrPlaylistImage    = errors.New("a playlist holds video and audio clips - an image never ends, so nothing would advance past it")
	ErrPlaylistEmpty    = errors.New("this playlist has no clips")
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

// declaredFormats is the one declaration of what the app will accept, in the
// order the UI lists it. The list is what WebView2 decodes on Windows, so
// anything missing here is genuinely unplayable rather than merely untested:
// notably .mkv and .avi are left out because Chromium refuses them regardless
// of the codec inside. The import dialog's filter, KindFor and the format list
// shown to the operator are all derived from it, so they cannot drift apart.
var declaredFormats = []struct {
	ext  string
	kind Kind
}{
	{".jpg", KindImage},
	{".jpeg", KindImage},
	{".png", KindImage},
	{".gif", KindImage},
	{".webp", KindImage},
	{".bmp", KindImage},
	{".mp4", KindVideo},
	{".m4v", KindVideo},
	{".mov", KindVideo},
	{".webm", KindVideo},
	{".mp3", KindAudio},
	{".m4a", KindAudio},
	{".aac", KindAudio},
	{".wav", KindAudio},
	{".ogg", KindAudio},
	{".flac", KindAudio},
}

// extensions indexes declaredFormats for KindFor.
var extensions = func() map[string]Kind {
	byExt := make(map[string]Kind, len(declaredFormats))
	for _, format := range declaredFormats {
		byExt[format.ext] = format.kind
	}
	return byExt
}()

// codecNotes names, per kind, the codecs that actually decode inside the
// declared containers on Windows: H.264 rides the platform media stack while
// the open codecs ship with Chromium itself. It is a declaration, not a probe -
// a file outside it may still play, but the app does not promise it.
var codecNotes = map[Kind]string{
	KindVideo: "H.264, VP8, VP9",
	KindAudio: "MP3, AAC, Opus, Vorbis, FLAC, PCM",
}

// FormatGroup is one kind's declared extensions and codecs, as the import UI
// shows them.
type FormatGroup struct {
	Kind       Kind     `json:"kind"`
	Extensions []string `json:"extensions"`
	Codecs     string   `json:"codecs"`
}

// SupportedFormats reports the declared formats grouped by kind, in
// declaration order, so the UI can state up front what will be accepted
// instead of leaving the operator to discover it one rejected file at a time.
func SupportedFormats() []FormatGroup {
	var groups []FormatGroup
	byKind := map[Kind]int{}
	for _, format := range declaredFormats {
		index, ok := byKind[format.kind]
		if !ok {
			index = len(groups)
			byKind[format.kind] = index
			groups = append(groups, FormatGroup{Kind: format.kind, Codecs: codecNotes[format.kind]})
		}
		groups[index].Extensions = append(groups[index].Extensions, format.ext)
	}
	return groups
}

// PickerFilter is the native file-dialog filter derived from the same
// declaration the importer enforces, so the picker can never offer less than
// import accepts - or more.
func PickerFilter() string {
	parts := make([]string, 0, len(declaredFormats))
	for _, format := range declaredFormats {
		parts = append(parts, "*"+format.ext)
	}
	return strings.Join(parts, ";")
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
	// BackgroundID names the library picture an audio or video asset shows
	// behind itself; empty means the plain frame.
	BackgroundID string
	CreatedAt    time.Time
	Presentation
}

// ImportProblem explains one file the library did not take. Duplicates are
// reported here too: they are a normal outcome of re-dropping a folder, not an
// error worth an alarm.
type ImportProblem struct {
	Path      string `json:"path"`
	Reason    string `json:"reason"`
	Duplicate bool   `json:"duplicate"`
}

// Playlist is an ordered run of clips the operator plays back to back. Items
// are library ids in play order; only video and audio belong here, because a
// run advances when a clip ends and an image never ends.
type Playlist struct {
	ID        string
	Name      string
	Items     []string
	CreatedAt time.Time
}
