package themes

import (
	"errors"
	"fmt"
	"regexp"
	"strings"
	"time"
)

var (
	ErrThemeNotFound = errors.New("theme not found")
	ErrInvalidTheme  = errors.New("invalid theme")
	ErrNameTaken     = errors.New("a theme with that name already exists for this kind")
)

type Kind string

const (
	KindScripture Kind = "scripture"
	KindLyrics    Kind = "lyrics"
	KindText      Kind = "text"
)

type BackgroundKind string

const (
	BackgroundSolid    BackgroundKind = "solid"
	BackgroundGradient BackgroundKind = "gradient"
	BackgroundImage    BackgroundKind = "image"
)

// FontFamily is one choice from the list the app will draw. The value is a whole
// CSS stack taken from FontFamilies rather than free text, which is what keeps a
// theme definition from carrying anything but a family name.
type FontFamily struct {
	Value string `json:"value"`
	Label string `json:"label"`
}

var fontFamilies = []FontFamily{
	{"Georgia, 'Times New Roman', serif", "Georgia"},
	{"'Palatino Linotype', Palatino, 'Book Antiqua', serif", "Palatino"},
	{"Segoe UI, system-ui, sans-serif", "Segoe UI"},
	{"Verdana, Geneva, sans-serif", "Verdana"},
}

func FontFamilies() []FontFamily {
	return append([]FontFamily(nil), fontFamilies...)
}

func knownFont(value string) bool {
	for _, family := range fontFamilies {
		if family.Value == value {
			return true
		}
	}
	return false
}

var hexColor = regexp.MustCompile(`^#(?:[0-9a-fA-F]{3}|[0-9a-fA-F]{6})$`)

// Style is how the words on a slide look. Sizes are percentages of the output
// height so one theme reads the same size in a preview pane and on a projector.
type Style struct {
	FontFamily string  `json:"fontFamily"`
	FontSize   float64 `json:"fontSize"`
	LineHeight float64 `json:"lineHeight"`
	Bold       bool    `json:"bold"`
	Italic     bool    `json:"italic"`
	Align      string  `json:"align"`
	Color      string  `json:"color"`
	Shadow     bool    `json:"shadow"`
	Margin     float64 `json:"margin"`
}

// Background is what sits behind the words. An image is a media library asset,
// referenced by id, so a theme never stores a path that could point anywhere.
type Background struct {
	Kind    BackgroundKind `json:"kind"`
	Color   string         `json:"color"`
	Color2  string         `json:"color2"`
	Angle   int            `json:"angle"`
	MediaID string         `json:"mediaId"`
}

type Theme struct {
	ID         string     `json:"id"`
	Name       string     `json:"name"`
	Kind       Kind       `json:"kind"`
	IsDefault  bool       `json:"isDefault"`
	Style      Style      `json:"style"`
	Background Background `json:"background"`
	// Reference places the passage or song label, which an operator may want off
	// a full-bleed picture.
	Reference string `json:"reference"`

	CreatedAt time.Time `json:"createdAt"`
	UpdatedAt time.Time `json:"updatedAt"`
}

const (
	ReferenceNone   = "none"
	ReferenceTop    = "top"
	ReferenceBottom = "bottom"
)

// Limits are the range the editor's controls can ask for. A value outside them
// is clamped rather than refused, because it can only come from a slider that
// was dragged too far or a row written by an older build.
const (
	minFontSize   = 2.0
	maxFontSize   = 12.0
	minLineHeight = 1.0
	maxLineHeight = 2.5
	minMargin     = 0.0
	maxMargin     = 25.0
)

// DefaultStyle is the reading look a new theme starts from: large, centred and
// high contrast, which is what a room needs before anyone has tuned anything.
func DefaultStyle() Style {
	return Style{
		FontFamily: fontFamilies[0].Value,
		FontSize:   4.6,
		LineHeight: 1.45,
		Align:      "center",
		Color:      "#F3F6FC",
		Shadow:     true,
		Margin:     9,
	}
}

func DefaultBackground() Background {
	return Background{Kind: BackgroundSolid, Color: "#081A2E", Angle: 180}
}

// DefaultTheme builds one theme for the editor's "new theme" button. It is not
// stored until the operator saves it.
func DefaultTheme(kind Kind) Theme {
	name := "Untitled"
	switch kind {
	case KindScripture:
		name = "Reading"
	case KindLyrics:
		name = "Sing"
	case KindText:
		name = "Notice"
	}
	return Theme{
		Name:       name,
		Kind:       kind,
		Style:      DefaultStyle(),
		Background: DefaultBackground(),
		Reference:  ReferenceBottom,
	}
}

func knownKind(kind Kind) bool {
	switch kind {
	case KindScripture, KindLyrics, KindText:
		return true
	}
	return false
}

func knownReference(value string) bool {
	switch value {
	case ReferenceNone, ReferenceTop, ReferenceBottom:
		return true
	}
	return false
}

// Validate checks what came from the editor. Anything it cannot draw is refused
// with the reason, so a half-written theme is never stored.
func (t Theme) Validate() error {
	if strings.TrimSpace(t.Name) == "" {
		return fmt.Errorf("%w: a theme needs a name", ErrInvalidTheme)
	}
	if !knownKind(t.Kind) {
		return fmt.Errorf("%w: unknown slide kind %q", ErrInvalidTheme, t.Kind)
	}
	if !knownFont(t.Style.FontFamily) {
		return fmt.Errorf("%w: unknown font %q", ErrInvalidTheme, t.Style.FontFamily)
	}
	switch t.Style.Align {
	case "left", "center", "right":
	default:
		return fmt.Errorf("%w: unknown alignment %q", ErrInvalidTheme, t.Style.Align)
	}
	if !knownReference(t.Reference) {
		return fmt.Errorf("%w: unknown reference placement %q", ErrInvalidTheme, t.Reference)
	}
	if !hexColor.MatchString(t.Style.Color) {
		return fmt.Errorf("%w: text colour must be a hex value", ErrInvalidTheme)
	}

	switch t.Background.Kind {
	case BackgroundSolid:
		if !hexColor.MatchString(t.Background.Color) {
			return fmt.Errorf("%w: background colour must be a hex value", ErrInvalidTheme)
		}
	case BackgroundGradient:
		if !hexColor.MatchString(t.Background.Color) || !hexColor.MatchString(t.Background.Color2) {
			return fmt.Errorf("%w: a gradient needs two hex colours", ErrInvalidTheme)
		}
	case BackgroundImage:
		if strings.TrimSpace(t.Background.MediaID) == "" {
			return fmt.Errorf("%w: an image background needs a picture from the library", ErrInvalidTheme)
		}
	default:
		return fmt.Errorf("%w: unknown background %q", ErrInvalidTheme, t.Background.Kind)
	}
	return nil
}

// Normalised repairs what came out of the database. A row written by an older
// build, or with a background picture that has since been deleted, falls back to
// something readable instead of failing the whole list.
func (t Theme) Normalised() Theme {
	out := t
	out.Name = strings.TrimSpace(out.Name)
	if out.Name == "" {
		out.Name = "Untitled theme"
	}
	if !knownKind(out.Kind) {
		out.Kind = KindScripture
	}
	if !knownFont(out.Style.FontFamily) {
		out.Style.FontFamily = fontFamilies[0].Value
	}
	if out.Style.Align != "left" && out.Style.Align != "right" {
		out.Style.Align = "center"
	}
	if !knownReference(out.Reference) {
		out.Reference = ReferenceBottom
	}
	if !hexColor.MatchString(out.Style.Color) {
		out.Style.Color = DefaultStyle().Color
	}

	out.Style.FontSize = clamp(out.Style.FontSize, minFontSize, maxFontSize)
	out.Style.LineHeight = clamp(out.Style.LineHeight, minLineHeight, maxLineHeight)
	out.Style.Margin = clamp(out.Style.Margin, minMargin, maxMargin)
	out.Background.Angle = ((out.Background.Angle % 360) + 360) % 360

	switch out.Background.Kind {
	case BackgroundSolid, BackgroundGradient, BackgroundImage:
	default:
		out.Background.Kind = BackgroundSolid
	}
	if !hexColor.MatchString(out.Background.Color) {
		out.Background.Color = DefaultBackground().Color
	}
	if out.Background.Kind == BackgroundGradient && !hexColor.MatchString(out.Background.Color2) {
		out.Background.Color2 = out.Background.Color
	}
	return out
}

func clamp(value, low, high float64) float64 {
	switch {
	case value < low:
		return low
	case value > high:
		return high
	default:
		return value
	}
}
