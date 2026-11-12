package main

import (
	"strings"

	"zopresent/internal/themes"
)

// themesWindow doubles as the reuse key: opening the editor again moves the
// existing window.
const themesWindow = "themes"

func (a *App) ThemeFonts() []themes.FontFamily {
	return themes.FontFamilies()
}

func (a *App) NewTheme(kind themes.Kind) themes.Theme {
	return themes.DefaultTheme(kind)
}

// themeChange refreshes the previewed slide so a colour change shows in the pane
// the editor is aimed at. Live output is deliberately untouched: a slide on the
// wall keeps the look it was projected with.
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

// DuplicateTheme exists because the usual reason to duplicate is to keep a
// working theme while trying a change on it.
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

// OpenThemeWindow keeps theme editing out of the studio, which stays a production
// view.
func (a *App) OpenThemeWindow() (EditorView, error) {
	return openUtilityWindow(
		themesWindow, "Themes", "/?view=themes",
		940, 660, 760, 520,
	)
}

// duplicateName finds the first free "(copy)" suffix, so duplicating three times
// gives three usable rows instead of an error.
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
