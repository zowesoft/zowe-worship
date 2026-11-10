package main

import (
	"errors"
	"fmt"

	"github.com/wailsapp/wails/v3/pkg/application"
)

// MediaControlEvent carries one transport command to the window that is playing.
const MediaControlEvent = "media-control"

// MediaProgressEvent reports the clip clock from the audience player back to
// the studio, so the transport bar shows time moving while the clip plays.
const MediaProgressEvent = "media-progress"

// MediaEndedEvent tells the studio a clip finished by itself, so its transport
// button can stop offering Pause.
const MediaEndedEvent = "media-ended"

// MediaCommand is one transport instruction to the audience player. Seek takes
// a fraction of the clip (0..1); volume is a percentage (0..100).
type MediaCommand struct {
	Action string  `json:"action"`
	Value  float64 `json:"value"`
}

// MediaProgressView is the clip clock reported from the player window.
type MediaProgressView struct {
	TimeMS     int64 `json:"timeMs"`
	DurationMS int64 `json:"durationMs"`
}

var errNoAudienceDisplay = errors.New("open the audience display to control playback")

func init() {
	application.RegisterEvent[MediaCommand](MediaControlEvent)
	application.RegisterEvent[MediaProgressView](MediaProgressEvent)
	application.RegisterEvent[string](MediaEndedEvent)
}

// MediaControl sends a transport command to the audience display. That window
// is the only player, so the studio's panes never drift out of step with it.
func (a *App) MediaControl(command MediaCommand) error {
	switch command.Action {
	case "play", "pause", "restart", "stop":
	case "seek":
		if command.Value < 0 || command.Value > 1 {
			return fmt.Errorf("seek takes a fraction between 0 and 1, got %g", command.Value)
		}
	case "volume":
		if command.Value < 0 || command.Value > 100 {
			return fmt.Errorf("volume takes a percentage between 0 and 100, got %g", command.Value)
		}
	default:
		return fmt.Errorf("unknown media command %q", command.Action)
	}
	if a.display == nil {
		return errNoAudienceDisplay
	}

	a.display.DispatchWailsEvent(&application.CustomEvent{Name: MediaControlEvent, Data: command})
	return nil
}

// MediaProgress is the player's report of where the clip is, twice a second
// while it runs.
func (a *App) MediaProgress(timeMS, durationMS int64) {
	dest := application.Get()
	if dest == nil {
		return
	}
	window, ok := dest.Window.GetByName(studioWindow)
	if !ok {
		return
	}
	window.DispatchWailsEvent(&application.CustomEvent{
		Name: MediaProgressEvent,
		Data: MediaProgressView{TimeMS: timeMS, DurationMS: durationMS},
	})
}

// MediaEnded is the display's own report that a clip reached its end. The
// studio is told because it is the window showing the transport button.
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
