package main

import (
	"strings"

	"zowe-worship/internal/themes"
)

// themesWindow names the theme editor so opening it again moves the same window
// instead of stacking a second one.
const themesWindow = "themes"

func (a *App) ThemeFonts() []themes.FontFamily {
	return themes.FontFamilies()
}

// NewTheme hands the editor an unsaved theme with the app's readable defaults,
// so "a new theme looks like this" is stated once rather than in two languages.
func (a *App) NewTheme(kind themes.Kind) themes.Theme {
	return themes.DefaultTheme(kind)
}

// themeChange hands the editor its refreshed list and rebuilds the previewed
// slide, because an operator who changes a reference colour expects to see it in
// the pane they are aiming at rather than after hunting for the item again.
// Live output is deliberately untouched: a slide on the wall keeps the look it
// was projected with.
func (a *App) themeChange(list []themes.Theme, err error) ([]themes.Theme, error) {
	if err == nil && a.presentation != nil {
		if state, refreshErr := a.presentation.RefreshPreview(a.context()); refreshErr == nil {
			a.publishToDisplay(state)
		}
	}
	return list, err
}

func (a *App) ListThemes() ([]themes.Theme, error) {
	if a.themes == nil {
		return nil, errStorageUnavailable
	}
	return a.themes.List(a.context())
}

// CreateTheme stores one theme and returns the whole list, so the editor can
// show the new entry without a second call racing the save.
func (a *App) CreateTheme(theme themes.Theme) ([]themes.Theme, error) {
	if a.themes == nil {
		return nil, errStorageUnavailable
	}
	ctx := a.context()

	if _, err := a.themes.Create(ctx, theme); err != nil {
		return nil, err
	}
	return a.themeChange(a.themes.List(ctx))
}

func (a *App) UpdateTheme(theme themes.Theme) ([]themes.Theme, error) {
	if a.themes == nil {
		return nil, errStorageUnavailable
	}
	ctx := a.context()

	if _, err := a.themes.Update(ctx, theme); err != nil {
		return nil, err
	}
	return a.themeChange(a.themes.List(ctx))
}

func (a *App) DeleteTheme(id string) ([]themes.Theme, error) {
	if a.themes == nil {
		return nil, errStorageUnavailable
	}
	ctx := a.context()

	if err := a.themes.Delete(ctx, strings.TrimSpace(id)); err != nil {
		return nil, err
	}
	return a.themeChange(a.themes.List(ctx))
}

func (a *App) MakeThemeDefault(id string) ([]themes.Theme, error) {
	if a.themes == nil {
		return nil, errStorageUnavailable
	}
	ctx := a.context()

	if err := a.themes.MakeDefault(ctx, strings.TrimSpace(id)); err != nil {
		return nil, err
	}
	return a.themeChange(a.themes.List(ctx))
}

// DuplicateTheme copies one under a fresh name, because the usual reason to
// duplicate is to keep a working theme while trying a change on it.
func (a *App) DuplicateTheme(id string) ([]themes.Theme, error) {
	if a.themes == nil {
		return nil, errStorageUnavailable
	}
	ctx := a.context()

	source, err := a.themes.Get(ctx, strings.TrimSpace(id))
	if err != nil {
		return nil, err
	}
	existing, err := a.themes.List(ctx)
	if err != nil {
		return nil, err
	}

	copied := source
	copied.ID = ""
	copied.IsDefault = false
	copied.Name = duplicateName(source.Name, existing)
	if _, err := a.themes.Create(ctx, copied); err != nil {
		return nil, err
	}
	return a.themeChange(a.themes.List(ctx))
}

// OpenThemeWindow edits themes in their own window, which keeps the studio a
// production view.
func (a *App) OpenThemeWindow() (EditorView, error) {
	return openUtilityWindow(
		themesWindow, "Themes", "/?view=themes",
		940, 660, 760, 520,
	)
}

// duplicateName finds the first free "(copy)" suffix, so duplicating the same
// theme three times gives three usable rows instead of an error.
func duplicateName(name string, existing []themes.Theme) string {
	taken := make(map[string]bool, len(existing))
	for _, theme := range existing {
		taken[strings.ToLower(strings.TrimSpace(theme.Name))] = true
	}

	base := strings.TrimSpace(name)
	for _, suffix := range []string{" (copy)", " (copy 2)", " (copy 3)", " (copy 4)", " (copy 5)"} {
		candidate := base + suffix
		if !taken[strings.ToLower(candidate)] {
			return candidate
		}
	}
	return base + " copy"
}
