package main

import (
	"errors"
	"fmt"

	"zopresent/internal/presentation"

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

// DisplayEvent reports the audience output's state so the studio's selector
// and live chip follow open, close and reconnect without polling - including
// a projector window that was closed on the far end.
const DisplayEvent = "display"

var errNoApplication = errors.New("the audience display needs the desktop app")

// Registering the event gives the binding generator a payload type, so the
// listener is typed rather than `any`.
func init() {
	application.RegisterEvent[PresentationView](AudienceEvent)
	application.RegisterEvent[DisplayView](DisplayEvent)
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
	// Wanted is the operator's choice, kept even while the window is gone:
	// it is what lets the studio offer a reconnect instead of only ever
	// reporting "No display".
	Wanted bool `json:"wanted"`
}

// displayTarget remembers the chosen output well enough to find it again. A
// screen id is an OS handle - an HMONITOR on Windows - that does not survive
// replug or sleep, while the device name and the physical geometry usually
// do, so reconnect matches on those first.
type displayTarget struct {
	option ScreenOption
	device string
	width  int
	height int
}

func screenLabel(index int, screen *application.Screen) string {
	return fmt.Sprintf("Display %d · %d×%d", index+1,
		screen.PhysicalBounds.Width, screen.PhysicalBounds.Height)
}

// screenLabelFor numbers the screen the way the picker would, so the window
// title matches the menu entry that was clicked.
func screenLabelFor(screens []*application.Screen, target *application.Screen) string {
	for index, screen := range screens {
		if screen == target {
			return screenLabel(index, screen)
		}
	}
	return screenLabel(0, target)
}

// studioScreenID is the monitor the controls live on, and never a candidate
// for the audience output.
func studioScreenID(desktop *application.App) string {
	if studio, ok := desktop.Window.GetByName(studioWindow); ok {
		if screen, err := studio.GetScreen(); err == nil && screen != nil {
			return screen.ID
		}
	}
	return ""
}

// resolveScreen picks the monitor that best matches want, and never the
// studio's own. The match order - device name, then geometry with the primary
// flag breaking ties between identical panels, then the raw id - puts the
// signals that survive a replug ahead of the one that does not; a fresh open
// from the dropdown knows only the id and still resolves.
func resolveScreen(screens []*application.Screen, skipID string, want displayTarget) *application.Screen {
	var byDevice, byGeometry, byGeometryMatch, byID *application.Screen
	for _, screen := range screens {
		if screen.ID == skipID {
			continue
		}
		if want.device != "" && screen.Name == want.device && byDevice == nil {
			byDevice = screen
		}
		if want.width != 0 && screen.PhysicalBounds.Width == want.width && screen.PhysicalBounds.Height == want.height {
			if byGeometry == nil {
				byGeometry = screen
			}
			if screen.IsPrimary == want.option.IsPrimary && byGeometryMatch == nil {
				byGeometryMatch = screen
			}
		}
		if want.option.ID != "" && screen.ID == want.option.ID && byID == nil {
			byID = screen
		}
	}
	switch {
	case byDevice != nil:
		return byDevice
	case byGeometryMatch != nil:
		return byGeometryMatch
	case byGeometry != nil:
		return byGeometry
	case byID != nil:
		return byID
	}
	return nil
}

// ListScreens leaves the studio's own screen out rather than assuming "the
// primary", because which monitor is primary depends on how the room is wired.
func (a *App) ListScreens() ([]ScreenOption, error) {
	desktop := application.Get()
	if desktop == nil {
		return nil, errNoApplication
	}

	studioID := studioScreenID(desktop)

	screens := desktop.Screen.GetAll()
	options := make([]ScreenOption, 0, len(screens))
	for index, screen := range screens {
		if screen.ID == studioID {
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
	screens := desktop.Screen.GetAll()
	target := resolveScreen(screens, studioScreenID(desktop), displayTarget{option: ScreenOption{ID: screenID}})
	if target == nil {
		return DisplayView{}, fmt.Errorf("unknown display %q", screenID)
	}
	if err := a.openAudienceOn(desktop, screens, target); err != nil {
		return DisplayView{}, err
	}
	a.wanted = true
	a.emitDisplay()
	return a.displayView(), nil
}

// openAudienceOn places the audience window on an already-resolved screen.
// The window is created hidden, taken fullscreen and only then shown: with
// StartState fullscreen, Wails sizes the window to whichever monitor the
// HWND spawns on - the studio's - and merely centres it on the target
// afterwards, so mismatched resolutions or mixed scale factors leave the room
// with a window sized for the studio monitor. Fullscreen() after creation
// derives the bounds from the monitor the window actually sits on, and
// showing last means no mis-sized frame is ever visible.
func (a *App) openAudienceOn(desktop *application.App, screens []*application.Screen, target *application.Screen) error {
	if a.display != nil {
		a.closeAudienceDisplay()
	}
	label := screenLabelFor(screens, target)
	window := desktop.Window.NewWithOptions(application.WebviewWindowOptions{
		Name:             audienceWindow,
		Title:            fmt.Sprintf("Zopresent — %s", label),
		Screen:           target,
		Frameless:        true,
		Hidden:           true,
		BackgroundColour: application.NewRGB(8, 15, 28),
		DevToolsEnabled:  false,
		URL:              "/?view=audience",
	})
	if window == nil {
		return errors.New("the audience display could not be created")
	}

	a.display = window
	a.rememberTarget(target, label)
	// The window dying - Alt+F4 on the projector, the OS reclaiming it -
	// must not forget the output: the handle goes, the choice stays.
	window.RegisterHook(events.Common.WindowClosing, func(*application.WindowEvent) {
		a.audienceWindowClosed()
	})
	window.Fullscreen()
	window.Show()

	a.publishToDisplay(a.presentation.State())
	return nil
}

// rememberTarget records enough of the resolved screen to find it again.
func (a *App) rememberTarget(target *application.Screen, label string) {
	a.target = displayTarget{
		option: ScreenOption{ID: target.ID, Name: label, IsPrimary: target.IsPrimary},
		device: target.Name,
		width:  target.PhysicalBounds.Width,
		height: target.PhysicalBounds.Height,
	}
}

func (a *App) CloseAudienceDisplay() (DisplayView, error) {
	a.closeAudienceDisplay()
	// The operator asked for this, so no reconnect is offered until a
	// display is chosen again; the screen itself stays remembered as the
	// sensible default for the next open.
	a.wanted = false
	a.emitDisplay()
	return a.displayView(), nil
}

func (a *App) closeAudienceDisplay() {
	if a.display == nil {
		return
	}
	window := a.display
	a.audienceWindowClosed()
	window.Close()
}

// audienceWindowClosed runs for every route out of the display - the
// operator's Close, a reopen, Alt+F4 on the projector, the window being
// reclaimed by the OS. Only the handle is dropped; the remembered output and
// the operator's intent survive, which is the whole point: closing the window
// used to wipe both, so the studio could never offer it again.
func (a *App) audienceWindowClosed() {
	if a.display == nil {
		return
	}
	a.display = nil
	a.emitDisplay()
}

func (a *App) displayView() DisplayView {
	return DisplayView{
		Open:       a.display != nil,
		ScreenID:   a.target.option.ID,
		ScreenName: a.target.option.Name,
		Wanted:     a.wanted,
	}
}

// emitDisplay pushes the current state to every window, so the studio's
// selector and live chip follow the display without polling - including when
// the projector window is closed on the far end.
func (a *App) emitDisplay() {
	desktop := application.Get()
	if desktop == nil {
		return
	}
	desktop.Event.Emit(DisplayEvent, a.displayView())
}

// ReconnectAudienceDisplay puts the output back. A window that survived a
// monitor blip is re-fitted to the resolved screen - it leaves fullscreen
// first, because Fullscreen() is a no-op while the window already believes
// it is fullscreen, and the bounds have to be derived again from where the
// window now sits. A dead window is reopened on the remembered screen; if
// that screen is still absent the error says so and the reconnect offer
// stays.
func (a *App) ReconnectAudienceDisplay() (DisplayView, error) {
	desktop := application.Get()
	if desktop == nil {
		return DisplayView{}, errNoApplication
	}
	if a.target.option.ID == "" && a.target.device == "" {
		return a.displayView(), errors.New("no audience display has been chosen")
	}
	screens := desktop.Screen.GetAll()
	target := resolveScreen(screens, studioScreenID(desktop), a.target)
	if target == nil {
		known := a.target.option.Name
		if known == "" {
			known = a.target.option.ID
		}
		return a.displayView(), fmt.Errorf("%s is not connected", known)
	}
	label := screenLabelFor(screens, target)

	if a.display != nil {
		a.display.UnFullscreen()
		a.display.SetScreen(target)
		a.display.Fullscreen()
		a.rememberTarget(target, label)
		a.emitDisplay()
		return a.displayView(), nil
	}
	if err := a.openAudienceOn(desktop, screens, target); err != nil {
		return DisplayView{}, err
	}
	a.wanted = true
	a.emitDisplay()
	return a.displayView(), nil
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
