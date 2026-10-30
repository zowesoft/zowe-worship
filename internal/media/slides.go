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

const (
	FitContain = "contain"
	FitCover   = "cover"
	AspectAuto = "auto"
)

var (
	ErrNotImage = errors.New("asset is not an image")
	ErrFileGone = errors.New("the library copy of this asset is missing")
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

// FullFrame shows the whole picture at its own shape, which is what an asset
// nobody has adjusted should do.
func FullFrame() Presentation {
	return Presentation{Aspect: AspectAuto, Fit: FitContain}
}

// Normalised replaces anything unrecognised with the default rather than
// refusing the save: an unknown aspect can only come from an older build or a
// hand-edited row, and "show all of it" is the safe answer for both.
func (p Presentation) Normalised() Presentation {
	out := p
	if ratioFor(out.Aspect) == 0 && out.Aspect != AspectAuto {
		out.Aspect = AspectAuto
	}
	if out.Aspect == "" {
		out.Aspect = AspectAuto
	}
	if out.Fit != FitCover {
		out.Fit = FitContain
	}
	return out
}

// Slide is everything a viewer needs to draw an image slide. The ratio is
// already resolved so the frontend never has to know the aspect names.
type Slide struct {
	Title string  `json:"title"`
	URL   string  `json:"url"`
	Ratio float64 `json:"ratio"`
	Fit   string  `json:"fit"`
}

// FileURL is the address a browser would fetch the asset's own bytes from.
func FileURL(id string) string { return FileRoute + id }

// ThumbURL is the address of the generated tile for the library grid.
func ThumbURL(id string) string { return ThumbRoute + id }

// Image resolves an asset into a slide. It refuses rather than handing back a
// broken one, so a projector keeps showing the last good image when a file has
// gone walkabout.
func (l *Library) Image(ctx context.Context, id string) (Slide, error) {
	asset, err := l.repo.ByID(ctx, id)
	if err != nil {
		return Slide{}, err
	}
	if asset.Kind != KindImage {
		return Slide{}, ErrNotImage
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

func (l *Library) slideFor(asset Asset) Slide {
	return Slide{
		Title: asset.Title,
		URL:   FileURL(asset.ID),
		Ratio: ratioFor(asset.Aspect),
		Fit:   asset.Fit,
	}
}
