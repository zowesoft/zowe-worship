package main

import (
	"errors"
	"fmt"

	"zowe-worship/internal/presentation"

	"github.com/wailsapp/wails/v3/pkg/application"
	"github.com/wailsapp/wails/v3/pkg/events"
)

// audienceWindow doubles as the reuse key: a reopen replaces the window instead
// of stacking a second one.
const audienceWindow = "audience"

// studioWindow's monitor is never offered as an audience display.
const studioWindow = "studio"

// AudienceEvent pushes presentation state, so the display never polls.
const AudienceEvent = "presentation"

var errNoApplication = errors.New("the audience display needs the desktop app")

// Registering the event gives the binding generator a payload type, so the
// listener is typed rather than `any`.
func init() {
	application.RegisterEvent[PresentationView](AudienceEvent)
}

type ScreenOption struct {
	ID        string `json:"id"`
	Name      string `json:"name"`
	IsPrimary bool   `json:"isPrimary"`
}

type DisplayView struct {
	Open       bool   `json:"open"`
	ScreenID   string `json:"screenId"`
	ScreenName string `json:"screenName"`
}

func screenLabel(index int, screen *application.Screen) string {
	return fmt.Sprintf("Display %d · %d×%d", index+1,
		screen.PhysicalBounds.Width, screen.PhysicalBounds.Height)
}

// ListScreens leaves the studio's own screen out rather than assuming "the
// primary", because which monitor is primary depends on how the room is wired.
func (a *App) ListScreens() ([]ScreenOption, error) {
	desktop := application.Get()
	if desktop == nil {
		return nil, errNoApplication
	}

	var studioScreenID string
	if studio, ok := desktop.Window.GetByName(studioWindow); ok {
		if screen, err := studio.GetScreen(); err == nil && screen != nil {
			studioScreenID = screen.ID
		}
	}

	screens := desktop.Screen.GetAll()
	options := make([]ScreenOption, 0, len(screens))
	for index, screen := range screens {
		if screen.ID == studioScreenID {
			continue
		}
		options = append(options, ScreenOption{
			ID:        screen.ID,
			Name:      screenLabel(index, screen),
			IsPrimary: screen.IsPrimary,
		})
	}
	return options, nil
}

func (a *App) GetAudienceDisplay() DisplayView {
	return a.displayView()
}

// OpenAudienceDisplay shows whatever is already live rather than starting blank,
// so an operator can open it mid-service.
func (a *App) OpenAudienceDisplay(screenID string) (DisplayView, error) {
	desktop := application.Get()
	if desktop == nil {
		return DisplayView{}, errNoApplication
	}
	if a.display != nil {
		a.closeAudienceDisplay()
	}

	var target *application.Screen
	var label string
	for index, screen := range desktop.Screen.GetAll() {
		if screen.ID == screenID {
			target = screen
			label = screenLabel(index, screen)
			break
		}
	}
	if target == nil {
		return DisplayView{}, fmt.Errorf("unknown display %q", screenID)
	}

	window := desktop.Window.NewWithOptions(application.WebviewWindowOptions{
		Name:             audienceWindow,
		Title:            fmt.Sprintf("Zowe Worship — %s", label),
		Screen:           target,
		Frameless:        true,
		StartState:       application.WindowStateFullscreen,
		BackgroundColour: application.NewRGB(8, 15, 28),
		DevToolsEnabled:  false,
		URL:              "/?view=audience",
	})
	if window == nil {
		return DisplayView{}, errors.New("the audience display could not be created")
	}

	a.display = window
	a.displayScreen = ScreenOption{ID: target.ID, Name: label, IsPrimary: target.IsPrimary}
	window.RegisterHook(events.Common.WindowClosing, func(*application.WindowEvent) {
		a.clearAudienceDisplay()
	})

	a.publishToDisplay(a.presentation.State())
	return a.displayView(), nil
}

func (a *App) CloseAudienceDisplay() (DisplayView, error) {
	a.closeAudienceDisplay()
	return a.displayView(), nil
}

func (a *App) closeAudienceDisplay() {
	if a.display == nil {
		return
	}
	window := a.display
	a.clearAudienceDisplay()
	window.Close()
}

func (a *App) clearAudienceDisplay() {
	a.display = nil
	a.displayScreen = ScreenOption{}
}

func (a *App) displayView() DisplayView {
	if a.display == nil {
		return DisplayView{Open: false}
	}
	return DisplayView{
		Open:       true,
		ScreenID:   a.displayScreen.ID,
		ScreenName: a.displayScreen.Name,
	}
}

// publishToDisplay swallows failures on purpose: the studio must keep working if
// a projector is unplugged mid-service.
func (a *App) publishToDisplay(state presentation.State) {
	if a.presentation == nil {
		return
	}
	payload := PresentationView{Preview: state.Preview, Live: state.Live}

	// The studio follows the same event as the projector, so a change that did not
	// come from one of its own buttons — a theme edit in its own window, say —
	// still appears in the preview pane.
	if desktop := application.Get(); desktop != nil {
		if studio, ok := desktop.Window.GetByName(studioWindow); ok {
			studio.DispatchWailsEvent(&application.CustomEvent{Name: AudienceEvent, Data: payload})
		}
	}

	if a.display == nil {
		return
	}
	a.display.DispatchWailsEvent(&application.CustomEvent{Name: AudienceEvent, Data: payload})
}
