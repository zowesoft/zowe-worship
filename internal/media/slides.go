package media

import (
	"context"
	"errors"
	"strings"
)

// Routes the asset server answers and the frontend is handed. They live here so
// a slide and the handler can never disagree about where a file is served from.
const (
	FileRoute  = "/media/file/"
	ThumbRoute = "/media/thumb/"
)

const (
	FitContain = "contain"
	FitCover   = "cover"
	AspectAuto = "auto"
	// AspectScreen has no shape of its own: the picture takes the whole output
	// and whatever does not fit is cropped.
	AspectScreen = "screen"
)

var (
	ErrNotImage = errors.New("asset is not an image")
	ErrFileGone = errors.New("the library copy of this asset is missing")
	// SetBackground refuses both a moving target and a still one used backwards.
	ErrBackgroundOnStill  = errors.New("a picture is its own visual and takes no background")
	ErrBackgroundNotStill = errors.New("the background must be a picture from the library")
)

// Aspect is one of the framing choices offered to an operator. The list lives
// here so the grid, the editor and the renderer cannot drift apart.
type Aspect struct {
	Value string  `json:"value"`
	Label string  `json:"label"`
	Ratio float64 `json:"ratio"`
}

var aspects = []Aspect{
	{AspectAuto, "Original", 0},
	{AspectScreen, "Fill screen", 0},
	{"16:9", "Widescreen", 16.0 / 9.0},
	{"4:3", "Classic projector", 4.0 / 3.0},
	{"1:1", "Square", 1},
	{"9:16", "Vertical", 9.0 / 16.0},
	{"3:2", "Photo", 3.0 / 2.0},
	{"21:9", "Cinematic", 21.0 / 9.0},
}

func Aspects() []Aspect {
	return append([]Aspect(nil), aspects...)
}

func knownAspect(value string) bool {
	for _, aspect := range aspects {
		if aspect.Value == value {
			return true
		}
	}
	return false
}

func ratioFor(value string) float64 {
	for _, aspect := range aspects {
		if aspect.Value == value {
			return aspect.Ratio
		}
	}
	return 0
}

// Presentation is where an image sits in the output frame.
type Presentation struct {
	Aspect string `json:"aspect"`
	Fit    string `json:"fit"`
}

// FullFrame is what an asset nobody has adjusted should do.
func FullFrame() Presentation {
	return Presentation{Aspect: AspectAuto, Fit: FitContain}
}

// Normalised falls back to the default instead of refusing the save: an unknown
// aspect can only come from an older build or a hand-edited row, and "show all
// of it" is the safe answer for both.
func (p Presentation) Normalised() Presentation {
	out := p
	if !knownAspect(out.Aspect) {
		out.Aspect = AspectAuto
	}
	if out.Fit != FitCover {
		out.Fit = FitContain
	}
	return out
}

// Slide carries the resolved ratio so the frontend never looks an aspect name up,
// plus the name itself because "fill the screen" has no ratio to derive from.
type Slide struct {
	Kind   string  `json:"kind"`
	Title  string  `json:"title"`
	URL    string  `json:"url"`
	Aspect string  `json:"aspect"`
	Ratio  float64 `json:"ratio"`
	Fit    string  `json:"fit"`
	// BackgroundURL is the picture behind the clip; empty for images and for
	// clips nobody has dressed.
	BackgroundURL string `json:"backgroundUrl,omitempty"`
}

func FileURL(id string) string { return FileRoute + id }

func ThumbURL(id string) string { return ThumbRoute + id }

// Slide resolves any playable asset into a slide. It refuses rather than handing
// back a broken one, so a projector keeps showing the last good thing when a
// file has gone walkabout.
func (l *Library) Slide(ctx context.Context, id string) (Slide, error) {
	asset, err := l.repo.ByID(ctx, id)
	if err != nil {
		return Slide{}, err
	}
	if !l.Exists(asset) {
		return Slide{}, ErrFileGone
	}
	return l.slideFor(asset), nil
}

// Presentation reports the stored framing choices for an asset, which is what
// an editor needs to highlight the right chips. Slide carries the resolved
// ratio instead, because a renderer should not have to know the names.
func (l *Library) Presentation(ctx context.Context, id string) (Presentation, error) {
	asset, err := l.repo.ByID(ctx, id)
	if err != nil {
		return Presentation{}, err
	}
	if asset.Kind != KindImage {
		return Presentation{}, ErrNotImage
	}
	return asset.Presentation, nil
}

// SetPresentation stores how an image should be framed and reports the slide as
// it will now be shown, so an editor can refresh from the same reply.
func (l *Library) SetPresentation(ctx context.Context, id string, presentation Presentation) (Slide, error) {
	asset, err := l.repo.ByID(ctx, id)
	if err != nil {
		return Slide{}, err
	}
	if asset.Kind != KindImage {
		return Slide{}, ErrNotImage
	}

	presentation = presentation.Normalised()
	if err := l.repo.SavePresentation(ctx, id, presentation); err != nil {
		return Slide{}, err
	}
	asset.Presentation = presentation
	return l.slideFor(asset), nil
}

// SetBackground dresses an audio or video clip with a library picture. An empty
// background id takes the dress off again.
func (l *Library) SetBackground(ctx context.Context, id, backgroundID string) (Slide, error) {
	asset, err := l.repo.ByID(ctx, id)
	if err != nil {
		return Slide{}, err
	}
	if asset.Kind == KindImage {
		return Slide{}, ErrBackgroundOnStill
	}

	backgroundID = strings.TrimSpace(backgroundID)
	if backgroundID != "" {
		picture, err := l.repo.ByID(ctx, backgroundID)
		if err != nil {
			return Slide{}, err
		}
		if picture.Kind != KindImage {
			return Slide{}, ErrBackgroundNotStill
		}
		if !l.Exists(picture) {
			return Slide{}, ErrFileGone
		}
	}

	if err := l.repo.SaveBackground(ctx, id, backgroundID); err != nil {
		return Slide{}, err
	}
	asset.BackgroundID = backgroundID
	return l.slideFor(asset), nil
}

func (l *Library) slideFor(asset Asset) Slide {
	slide := Slide{
		Kind:   string(asset.Kind),
		Title:  asset.Title,
		URL:    FileURL(asset.ID),
		Aspect: asset.Aspect,
		Ratio:  ratioFor(asset.Aspect),
		Fit:    asset.Fit,
	}
	if asset.BackgroundID != "" && asset.Kind != KindImage {
		slide.BackgroundURL = FileURL(asset.BackgroundID)
	}
	return slide
}
