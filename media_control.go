package main

import (
	"errors"
	"fmt"

	"github.com/wailsapp/wails/v3/pkg/application"
)

// MediaControlEvent carries one transport command to the window that is playing.
const MediaControlEvent = "media-control"

// MediaEndedEvent tells the studio a clip finished by itself, so its transport
// button can stop offering Pause.
const MediaEndedEvent = "media-ended"

var errNoAudienceDisplay = errors.New("open the audience display to control playback")

// Registering the events gives the binding generator a payload type, so the
// listeners in both windows are typed rather than `any`.
func init() {
	application.RegisterEvent[string](MediaControlEvent)
	application.RegisterEvent[string](MediaEndedEvent)
}

// MediaControl sends play, pause or restart to the audience display. That window
// is the only player, so the studio's panes never drift out of step with it.
func (a *App) MediaControl(action string) error {
	switch action {
	case "play", "pause", "restart":
	default:
		return fmt.Errorf("unknown media command %q", action)
	}
	if a.display == nil {
		return errNoAudienceDisplay
	}

	a.display.DispatchWailsEvent(&application.CustomEvent{Name: MediaControlEvent, Data: action})
	return nil
}

// MediaEnded is the display's own report that a clip reached its end. The studio
// is told because it is the window showing the transport button.
func (a *App) MediaEnded() {
	desktop := application.Get()
	if desktop == nil {
		return
	}
	window, ok := desktop.Window.GetByName(studioWindow)
	if !ok {
		return
	}
	window.DispatchWailsEvent(&application.CustomEvent{Name: MediaEndedEvent, Data: ""})
}
