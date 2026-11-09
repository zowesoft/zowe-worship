package main

import (
	"errors"

	"github.com/wailsapp/wails/v3/pkg/application"
)

type EditorView struct {
	Open  bool   `json:"open"`
	Title string `json:"title"`
}

// openUtilityWindow shows one of the small editing windows. If that editor is
// already open it is pointed at the new item, so two windows never stack.
func openUtilityWindow(name, title, target string, width, height, minWidth, minHeight int) (EditorView, error) {
	desktop := application.Get()
	if desktop == nil {
		return EditorView{}, errNoApplication
	}

	if existing, ok := desktop.Window.GetByName(name); ok {
		existing.SetTitle(title)
		existing.SetURL(target)
		existing.Focus()
		return EditorView{Open: true, Title: title}, nil
	}

	window := desktop.Window.NewWithOptions(application.WebviewWindowOptions{
		Name:             name,
		Title:            title,
		Width:            width,
		Height:           height,
		MinWidth:         minWidth,
		MinHeight:        minHeight,
		URL:              target,
		BackgroundColour: application.NewRGB(8, 15, 28),
		DevToolsEnabled:  false,
	})
	if window == nil {
		return EditorView{}, errors.New("the editor window could not be opened")
	}
	return EditorView{Open: true, Title: title}, nil
}
