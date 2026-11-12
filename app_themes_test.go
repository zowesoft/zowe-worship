package main

import (
	"errors"
	"strings"
	"testing"

	"zopresent/internal/themes"
)

func TestThemeCommandsRoundTrip(t *testing.T) {
	app := newTestApp(t)

	empty, err := app.ListThemes()
	if err != nil {
		t.Fatalf("list before any theme: %v", err)
	}
	if len(empty) != 0 {
		t.Fatalf("themes started non-empty: %+v", empty)
	}

	if len(app.ThemeFonts()) == 0 {
		t.Fatal("the font menu needs at least one family")
	}

	reading := themes.DefaultTheme(themes.KindScripture)
	reading.Name = "Reading"
	list, err := app.CreateTheme(reading)
	if err != nil {
		t.Fatalf("create: %v", err)
	}
	if len(list) != 1 || !list[0].IsDefault {
		t.Fatalf("after create = %+v", list)
	}
	firstID := list[0].ID

	// A second theme of the same kind waits its turn.
	evening := reading
	evening.ID = ""
	evening.Name = "Evening"
	list, err = app.CreateTheme(evening)
	if err != nil {
		t.Fatalf("create second: %v", err)
	}
	if len(list) != 2 {
		t.Fatalf("themes = %+v", list)
	}

	// Duplicate takes a free name rather than failing on a clash.
	duplicated, err := app.DuplicateTheme(firstID)
	if err != nil {
		t.Fatalf("duplicate: %v", err)
	}
	if len(duplicated) != 3 {
		t.Fatalf("after duplicate = %+v", duplicated)
	}
	var copy themes.Theme
	for _, theme := range duplicated {
		if theme.ID != firstID && !theme.IsDefault {
			copy = theme
		}
	}
	if copy.ID == "" {
		t.Fatalf("no copy in %+v", duplicated)
	}
	if !strings.Contains(copy.Name, "copy") {
		t.Fatalf("the duplicate should say so in its name: %q", copy.Name)
	}
	if copy.IsDefault {
		t.Fatal("a copy should not steal the default")
	}

	again, err := app.DuplicateTheme(firstID)
	if err != nil {
		t.Fatalf("duplicate the same theme twice: %v", err)
	}
	if len(again) != 4 {
		t.Fatalf("after second duplicate = %+v", again)
	}

	moved, err := app.MakeThemeDefault(copy.ID)
	if err != nil {
		t.Fatalf("make default: %v", err)
	}
	defaults := 0
	for _, theme := range moved {
		if theme.IsDefault {
			defaults++
			if theme.ID != copy.ID {
				t.Fatalf("wrong theme became the default: %+v", theme)
			}
		}
	}
	if defaults != 1 {
		t.Fatalf("defaults = %d, want 1", defaults)
	}

	// A name already used for that kind is refused, not silently duplicated.
	renamed := copy
	renamed.Name = "Reading"
	if _, err := app.UpdateTheme(renamed); !errors.Is(err, themes.ErrNameTaken) {
		t.Fatalf("rename clash err = %v, want ErrNameTaken", err)
	}

	remaining, err := app.DeleteTheme(copy.ID)
	if err != nil {
		t.Fatalf("delete: %v", err)
	}
	if len(remaining) != 3 {
		t.Fatalf("after delete = %+v", remaining)
	}
	if _, err := app.DeleteTheme(copy.ID); err == nil {
		t.Fatal("deleting the same theme twice succeeded")
	}
}
