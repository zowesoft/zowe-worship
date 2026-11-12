package themes

import (
	"context"
	"encoding/json"
	"errors"
	"path/filepath"
	"strings"
	"testing"

	"zowe-worship/internal/database"
)

func newTestRepository(t *testing.T) (*SQLiteRepository, context.Context) {
	t.Helper()

	ctx := context.Background()
	paths := database.Paths{Root: filepath.Join(t.TempDir(), "Zowe Worship")}
	if err := paths.Ensure(); err != nil {
		t.Fatalf("prepare directories: %v", err)
	}
	db, err := database.Open(ctx, paths.CoreDatabase())
	if err != nil {
		t.Fatalf("open core database: %v", err)
	}
	if err := database.Migrate(ctx, db, paths.Backups()); err != nil {
		t.Fatalf("migrate: %v", err)
	}
	t.Cleanup(func() { _ = db.Close() })

	return NewSQLiteRepository(db), ctx
}

func TestCreateMakesTheFirstThemeTheDefault(t *testing.T) {
	repo, ctx := newTestRepository(t)

	first, err := repo.Create(ctx, DefaultTheme(KindScripture))
	if err != nil {
		t.Fatalf("create: %v", err)
	}
	if !first.IsDefault {
		t.Fatal("the only scripture theme should be the default")
	}

	second := DefaultTheme(KindScripture)
	second.Name = "Reading large"
	created, err := repo.Create(ctx, second)
	if err != nil {
		t.Fatalf("create second: %v", err)
	}
	if created.IsDefault {
		t.Fatal("a second theme should not take over the default")
	}

	// A different kind starts its own run of defaults.
	lyrics, err := repo.Create(ctx, DefaultTheme(KindLyrics))
	if err != nil {
		t.Fatalf("create lyrics: %v", err)
	}
	if !lyrics.IsDefault {
		t.Fatal("the only lyrics theme should be its kind's default")
	}

	stored, err := repo.Default(ctx, KindScripture)
	if err != nil {
		t.Fatalf("read default: %v", err)
	}
	if stored.ID != first.ID {
		t.Fatalf("default = %q, want %q", stored.ID, first.ID)
	}
}

func TestMakeDefaultMovesTheFlag(t *testing.T) {
	repo, ctx := newTestRepository(t)

	if _, err := repo.Create(ctx, DefaultTheme(KindScripture)); err != nil {
		t.Fatalf("create first: %v", err)
	}
	secondTheme := DefaultTheme(KindScripture)
	secondTheme.Name = "Evening"
	second, err := repo.Create(ctx, secondTheme)
	if err != nil {
		t.Fatalf("create second: %v", err)
	}

	if err := repo.MakeDefault(ctx, second.ID); err != nil {
		t.Fatalf("make default: %v", err)
	}

	all, err := repo.List(ctx)
	if err != nil {
		t.Fatalf("list: %v", err)
	}
	defaults := 0
	for _, theme := range all {
		if theme.Kind == KindScripture && theme.IsDefault {
			defaults++
			if theme.ID != second.ID {
				t.Fatalf("wrong theme became the default: %+v", theme)
			}
		}
	}
	if defaults != 1 {
		t.Fatalf("scripture defaults = %d, want 1", defaults)
	}

	if err := repo.MakeDefault(ctx, "missing"); !errors.Is(err, ErrThemeNotFound) {
		t.Fatalf("err = %v, want ErrThemeNotFound", err)
	}
}

func TestNamesAreUniqueWithinAKind(t *testing.T) {
	repo, ctx := newTestRepository(t)

	if _, err := repo.Create(ctx, DefaultTheme(KindScripture)); err != nil {
		t.Fatalf("create: %v", err)
	}
	dupe := DefaultTheme(KindScripture)
	dupe.Name = "reading" // the same name, different case
	if _, err := repo.Create(ctx, dupe); !errors.Is(err, ErrNameTaken) {
		t.Fatalf("err = %v, want ErrNameTaken", err)
	}

	// The same name for another kind is a different theme.
	lyrics := DefaultTheme(KindLyrics)
	lyrics.Name = "Reading"
	if _, err := repo.Create(ctx, lyrics); err != nil {
		t.Fatalf("create lyrics with the same name: %v", err)
	}
}

func TestValidateRefusesWhatItCannotDraw(t *testing.T) {
	cases := []struct {
		name   string
		mutate func(Theme) Theme
	}{
		{"no name", func(t Theme) Theme { t.Name = "   "; return t }},
		{"unknown kind", func(t Theme) Theme { t.Kind = "announcement"; return t }},
		{"free text font", func(t Theme) Theme { t.Style.FontFamily = "x; background:url(evil)"; return t }},
		{"unknown alignment", func(t Theme) Theme { t.Style.Align = "middle"; return t }},
		{"unknown reference placement", func(t Theme) Theme { t.Reference.Placement = "above"; return t }},
		{"unknown text case", func(t Theme) Theme { t.Style.Case = "Titlecase"; return t }},
		{"free text reference font", func(t Theme) Theme {
			t.Reference.FontFamily = "x; background:url(evil)"
			return t
		}},
		{"reference colour is not hex", func(t Theme) Theme { t.Reference.Color = "red"; return t }},
		{"unknown reference alignment", func(t Theme) Theme { t.Reference.Align = "middle"; return t }},
		{"unknown reference case", func(t Theme) Theme { t.Reference.Case = "caps"; return t }},
		{"text colour is not hex", func(t Theme) Theme { t.Style.Color = "red"; return t }},
		{"solid background without colour", func(t Theme) Theme { t.Background.Color = ""; return t }},
		{"gradient with one colour", func(t Theme) Theme {
			t.Background.Kind = BackgroundGradient
			t.Background.Color2 = ""
			return t
		}},
		{"image without a picture", func(t Theme) Theme {
			t.Background.Kind = BackgroundImage
			t.Background.MediaID = ""
			return t
		}},
		{"unknown background", func(t Theme) Theme { t.Background.Kind = "video"; return t }},
	}

	for _, testCase := range cases {
		t.Run(testCase.name, func(t *testing.T) {
			theme := testCase.mutate(DefaultTheme(KindScripture))
			if err := theme.Validate(); err == nil {
				t.Fatalf("%s was accepted", testCase.name)
			} else if !errors.Is(err, ErrInvalidTheme) {
				t.Fatalf("err = %v, want ErrInvalidTheme", err)
			}
		})
	}

	// The shapes the editor is allowed to build.
	gradient := DefaultTheme(KindScripture)
	gradient.Background.Kind = BackgroundGradient
	gradient.Background.Color2 = "#122A4A"
	if err := gradient.Validate(); err != nil {
		t.Fatalf("gradient refused: %v", err)
	}
	picture := DefaultTheme(KindScripture)
	picture.Background.Kind = BackgroundImage
	picture.Background.MediaID = "abc123"
	if err := picture.Validate(); err != nil {
		t.Fatalf("image background refused: %v", err)
	}

	// A reference that inherits everything but its placement is the common case.
	small := DefaultTheme(KindScripture)
	small.Reference = Reference{Placement: ReferenceTop, Case: CaseSmallCaps}
	if err := small.Validate(); err != nil {
		t.Fatalf("inheriting reference refused: %v", err)
	}

	styled := DefaultTheme(KindScripture)
	styled.Style.Case = CaseUppercase
	styled.Reference = Reference{
		Placement:  ReferenceBottom,
		FontFamily: fontFamilies[2].Value,
		FontSize:   2.4,
		Color:      "#9FB3CE",
		Align:      "right",
		Case:       CaseSmallCaps,
		Bold:       true,
	}
	if err := styled.Validate(); err != nil {
		t.Fatalf("fully styled reference refused: %v", err)
	}
}

func TestNormalisedRepairsWhatItCannotDraw(t *testing.T) {
	broken := Theme{
		Name: "",
		Kind: "scripture",
		Style: Style{
			FontFamily: "Impact",
			FontSize:   900,
			LineHeight: 0.2,
			Margin:     -12,
			Align:      "middle",
			Color:      "red",
		},
		Background: Background{
			Kind:    "video",
			Color:   "not a colour",
			Color2:  "also not",
			Angle:   -370,
			MediaID: "",
		},
		Reference: Reference{
			Placement:  "above",
			FontFamily: "Impact",
			FontSize:   -3,
			Color:      "crimson",
			Align:      "middle",
			Case:       "shout",
		},
	}

	fixed := broken.Normalised()
	if fixed.Name == "" {
		t.Fatal("an unnamed theme needs a name to show in a list")
	}
	if !knownFont(fixed.Style.FontFamily) {
		t.Fatalf("font = %q", fixed.Style.FontFamily)
	}
	if fixed.Style.Align != "center" || !hexColor.MatchString(fixed.Style.Color) {
		t.Fatalf("style = %+v", fixed.Style)
	}
	if fixed.Style.FontSize > maxFontSize || fixed.Style.LineHeight < minLineHeight || fixed.Style.Margin < minMargin {
		t.Fatalf("sizes not clamped: %+v", fixed.Style)
	}
	if fixed.Background.Kind != BackgroundSolid || !hexColor.MatchString(fixed.Background.Color) {
		t.Fatalf("background = %+v", fixed.Background)
	}
	if fixed.Background.Angle < 0 || fixed.Background.Angle >= 360 {
		t.Fatalf("angle = %d", fixed.Background.Angle)
	}
	if !knownReference(fixed.Reference.Placement) {
		t.Fatalf("reference = %+v", fixed.Reference)
	}
	// Each inherited property falls back to "same as the slide text", not to a
	// guess that would silently restyle a reference nobody asked to change.
	if fixed.Reference.FontFamily != "" || fixed.Reference.Color != "" || fixed.Reference.Align != "" {
		t.Fatalf("reference did not fall back to inheriting: %+v", fixed.Reference)
	}
	if fixed.Reference.Case != CaseAsWritten || fixed.Reference.FontSize != 0 {
		t.Fatalf("reference case or size not repaired: %+v", fixed.Reference)
	}
	if !knownCase(fixed.Style.Case) {
		t.Fatalf("text case = %q", fixed.Style.Case)
	}
	// Repairs must land inside what Validate will accept.
	if err := fixed.Validate(); err != nil {
		t.Fatalf("normalised theme still invalid: %v", err)
	}
}

func TestReferenceStyleSurvivesStorage(t *testing.T) {
	repo, ctx := newTestRepository(t)

	theme := DefaultTheme(KindScripture)
	theme.Style.Case = CaseUppercase
	theme.Reference = Reference{
		Placement:  ReferenceTop,
		FontFamily: fontFamilies[3].Value,
		FontSize:   2.4,
		Color:      "#9FB3CE",
		Align:      "right",
		Case:       CaseSmallCaps,
		Bold:       true,
		Italic:     true,
	}

	created, err := repo.Create(ctx, theme)
	if err != nil {
		t.Fatalf("create: %v", err)
	}
	if created.Reference != theme.Reference {
		t.Fatalf("reference stored as %+v, want %+v", created.Reference, theme.Reference)
	}
	if created.Style.Case != CaseUppercase {
		t.Fatalf("text case stored as %q", created.Style.Case)
	}

	// Clearing the overrides has to come back as "inherit", not as the old values.
	clear := created
	clear.Reference = Reference{Placement: ReferenceBottom, Case: CaseAsWritten}
	clear.Style.Case = CaseAsWritten
	updated, err := repo.Update(ctx, clear)
	if err != nil {
		t.Fatalf("update: %v", err)
	}
	if updated.Reference.FontFamily != "" || updated.Reference.Color != "" || updated.Reference.FontSize != 0 {
		t.Fatalf("overrides not cleared: %+v", updated.Reference)
	}
	if updated.Style.Case != CaseAsWritten || updated.Reference.Case != CaseAsWritten {
		t.Fatalf("case not cleared: %+v %+v", updated.Style.Case, updated.Reference.Case)
	}
}

func TestUpdateKeepsIdentityAndDeleteRemoves(t *testing.T) {
	repo, ctx := newTestRepository(t)

	created, err := repo.Create(ctx, DefaultTheme(KindLyrics))
	if err != nil {
		t.Fatalf("create: %v", err)
	}

	edited := created
	edited.Name = "Sing it loud"
	edited.Style.FontSize = 6.5
	edited.Background.Kind = BackgroundGradient
	edited.Background.Color2 = "#122A4A"
	updated, err := repo.Update(ctx, edited)
	if err != nil {
		t.Fatalf("update: %v", err)
	}
	if updated.ID != created.ID || !updated.CreatedAt.Equal(created.CreatedAt) {
		t.Fatalf("identity moved: %+v", updated)
	}
	if updated.Style.FontSize != 6.5 || updated.Background.Kind != BackgroundGradient {
		t.Fatalf("changes not stored: %+v", updated)
	}
	if !updated.IsDefault {
		t.Fatal("an edit must not drop the default flag")
	}

	missing := DefaultTheme(KindLyrics)
	missing.ID = "nowhere"
	missing.Name = "Ghost"
	if _, err := repo.Update(ctx, missing); !errors.Is(err, ErrThemeNotFound) {
		t.Fatalf("update err = %v, want ErrThemeNotFound", err)
	}

	if err := repo.Delete(ctx, created.ID); err != nil {
		t.Fatalf("delete: %v", err)
	}
	if err := repo.Delete(ctx, created.ID); !errors.Is(err, ErrThemeNotFound) {
		t.Fatalf("second delete err = %v, want ErrThemeNotFound", err)
	}
	if _, err := repo.Default(ctx, KindLyrics); !errors.Is(err, ErrThemeNotFound) {
		t.Fatalf("default after deleting the only one = %v", err)
	}
}

// TestMaliciousThemePayloadsAreRefusedOrRepaired pins the claim that themes are
// data, not code: every field the renderer reads is checked against a closed
// set of values, and a row that somehow carries something else is repaired to
// a readable default rather than drawn.
func TestMaliciousThemePayloadsAreRefusedOrRepaired(t *testing.T) {
	repo, ctx := newTestRepository(t)

	refused := []struct {
		name   string
		mutate func(*Theme)
	}{
		{"css injection in the body font", func(theme *Theme) {
			theme.Style.FontFamily = `Georgia; } body { background: url(http://evil.example) }`
		}},
		{"css injection in the text colour", func(theme *Theme) {
			theme.Style.Color = "red; background-image: url('javascript:alert(1)')"
		}},
		{"script scheme in the reference colour", func(theme *Theme) {
			theme.Reference.Color = "javascript:alert(1)"
		}},
		{"markup in the slide kind", func(theme *Theme) {
			theme.Kind = Kind("<script>alert(1)</script>")
		}},
		{"unknown background kind", func(theme *Theme) {
			theme.Background.Kind = "url(javascript:alert(1))"
		}},
		{"alignment outside the set", func(theme *Theme) {
			theme.Style.Align = "left; behavior:url(evil.htc)"
		}},
	}
	for _, tc := range refused {
		t.Run("refused: "+tc.name, func(t *testing.T) {
			theme := DefaultTheme(KindScripture)
			tc.mutate(&theme)
			if _, err := repo.Create(ctx, theme); !errors.Is(err, ErrInvalidTheme) {
				t.Fatalf("create err = %v, want ErrInvalidTheme", err)
			}
		})
	}

	// An update is held to the same rule: a valid theme cannot be turned into
	// a hostile one afterwards.
	valid, err := repo.Create(ctx, DefaultTheme(KindScripture))
	if err != nil {
		t.Fatalf("create: %v", err)
	}
	smuggled := valid
	smuggled.Style.FontFamily = `"Georgia"; xss: expression(alert(1))`
	if _, err := repo.Update(ctx, smuggled); !errors.Is(err, ErrInvalidTheme) {
		t.Fatalf("update err = %v, want ErrInvalidTheme", err)
	}

	// A row from an older or corrupt build is repaired to readable defaults:
	// unknown values fall back, out-of-range numbers clamp, and nothing
	// unvalidated reaches the renderer.
	corrupt := Theme{
		Name: "  Broken row  ",
		Kind: Kind("img onerror=alert(1)"),
		Style: Style{
			FontFamily: `"><script>alert(1)</script>`,
			FontSize:   9000,
			LineHeight: 99,
			Align:      "justify",
			Color:      "url(javascript:alert(1))",
			Margin:     500,
		},
		Reference: Reference{Placement: "left", Color: "javascript:alert(1)", FontSize: -4},
		Background: Background{
			Kind:  "javascript",
			Color: "red; xss",
			Angle: 400,
		},
	}
	healed := corrupt.Normalised()
	if healed.Kind != KindScripture {
		t.Fatalf("kind = %q, want the default kind", healed.Kind)
	}
	if !knownFont(healed.Style.FontFamily) {
		t.Fatalf("font = %q, want a known family", healed.Style.FontFamily)
	}
	if healed.Style.Align != "center" {
		t.Fatalf("align = %q, want center", healed.Style.Align)
	}
	if !hexColor.MatchString(healed.Style.Color) || !hexColor.MatchString(healed.Background.Color) {
		t.Fatalf("colours = %q / %q, want hex values", healed.Style.Color, healed.Background.Color)
	}
	if healed.Style.FontSize > maxFontSize || healed.Style.LineHeight > maxLineHeight || healed.Style.Margin > maxMargin {
		t.Fatalf("numbers not clamped: %+v", healed.Style)
	}
	if healed.Reference.Placement != ReferenceBottom || healed.Reference.Color != "" || healed.Reference.FontSize != 0 {
		t.Fatalf("reference not repaired: %+v", healed.Reference)
	}
	if healed.Background.Kind != BackgroundSolid || healed.Background.Angle != 40 {
		t.Fatalf("background not repaired: %+v", healed.Background)
	}
	if err := healed.Validate(); err != nil {
		t.Fatalf("repaired theme still invalid: %v", err)
	}

	// Fields the model does not declare are dropped by decoding: a payload
	// can carry script-shaped keys only as far as the struct, and a round
	// trip through storage proves nothing of them survives.
	var decoded Theme
	payload := `{
		"name": "Imported Reading",
		"kind": "scripture",
		"onclick": "alert(1)",
		"style": {"fontFamily": "Georgia, 'Times New Roman', serif", "fontSize": 4.6,
		          "lineHeight": 1.45, "align": "center", "color": "#F3F6FC",
		          "case": "none", "margin": 9,
		          "onmouseover": "alert(1)", "background": "url(javascript:alert(1))"},
		"reference": {"placement": "bottom", "case": "none"},
		"background": {"kind": "solid", "color": "#081A2E", "angle": 180}
	}`
	if err := json.Unmarshal([]byte(payload), &decoded); err != nil {
		t.Fatalf("decode payload: %v", err)
	}
	stored, err := repo.Create(ctx, decoded)
	if err != nil {
		t.Fatalf("create decoded: %v", err)
	}
	raw, err := json.Marshal(stored)
	if err != nil {
		t.Fatalf("marshal stored: %v", err)
	}
	for _, key := range []string{"onclick", "onmouseover", "javascript:"} {
		if strings.Contains(string(raw), key) {
			t.Fatalf("stored theme smuggled %q through: %s", key, raw)
		}
	}
}
