package main

import (
	"errors"
	"testing"

	"zopresent/internal/presentation"

	"github.com/wailsapp/wails/v3/pkg/application"
)

// The display plumbing has to stay inert outside the desktop app: unit tests,
// and a studio whose projector was unplugged, must not panic.
func TestAudienceDisplayStaysInertWithoutADesktopApp(t *testing.T) {
	app := newTestApp(t)

	if _, err := app.ListScreens(); !errors.Is(err, errNoApplication) {
		t.Fatalf("list screens err = %v, want errNoApplication", err)
	}
	if _, err := app.OpenAudienceDisplay("any"); !errors.Is(err, errNoApplication) {
		t.Fatalf("open display err = %v, want errNoApplication", err)
	}
	if _, err := app.ReconnectAudienceDisplay(); !errors.Is(err, errNoApplication) {
		t.Fatalf("reconnect display err = %v, want errNoApplication", err)
	}

	view, err := app.CloseAudienceDisplay()
	if err != nil {
		t.Fatalf("close with no display: %v", err)
	}
	if view.Open {
		t.Fatalf("closed display reports open: %+v", view)
	}

	if got := app.GetAudienceDisplay(); got.Open {
		t.Fatalf("display reports open: %+v", got)
	}

	app.publishToDisplay(presentation.State{})
}

func TestProjectionCommandsPublishOnlyWhenTheySucceed(t *testing.T) {
	app := newTestApp(t)
	installTestBible(t, app)

	state, err := app.applied(app.presentation.ProjectScripture(app.context(), "kjv", 43, 3, 16, 16, false))
	if err != nil {
		t.Fatalf("project: %v", err)
	}
	if state.Live == nil || state.Live.Reference != "John 3:16" {
		t.Fatalf("live after project = %+v", state.Live)
	}

	// A rejected command leaves the audience showing what it was already
	// showing, so a typo cannot blank the projector mid-service.
	if _, err := app.applied(app.presentation.PreviewScripture(app.context(), "kjv", 999, 1, 1, 1, false)); err == nil {
		t.Fatal("preview of an unknown book succeeded")
	}
}

// A screen id is an OS handle that does not survive a replug, while the device
// name does. Reconnect has to prefer the signal that survives and must never
// land on the studio's own monitor, which may well be the one the remembered
// id now points at.
func TestResolveScreenPrefersTheDeviceNameOverAChurnedID(t *testing.T) {
	projector := &application.Screen{
		ID:   "churned-1",
		Name: `\\.\DISPLAY2`,
		PhysicalBounds: application.Rect{
			Width: 1920, Height: 1080,
		},
	}
	// The studio's own monitor happens to hold the old id now, so an
	// id-first resolver would throw the room's picture onto the operator's
	// desk.
	studio := &application.Screen{
		ID:   "original-1",
		Name: `\\.\DISPLAY1`,
		PhysicalBounds: application.Rect{
			Width: 1920, Height: 1080,
		},
	}
	want := displayTarget{
		option: ScreenOption{ID: "original-1", IsPrimary: true},
		device: `\\.\DISPLAY2`,
		width:  1920,
		height: 1080,
	}

	got := resolveScreen([]*application.Screen{studio, projector}, "original-1", want)
	if got != projector {
		t.Fatalf("resolve = %+v, want the projector named by device name", got)
	}
}

// Two identical panels cannot be told apart by geometry, so the primary flag
// recorded at open time breaks the tie.
func TestResolveScreenBreaksGeometryTiesWithThePrimaryFlag(t *testing.T) {
	bounds := application.Rect{Width: 2560, Height: 1440}
	primary := &application.Screen{ID: "a", Name: `\\.\DISPLAY1`, IsPrimary: true, PhysicalBounds: bounds}
	identical := &application.Screen{ID: "b", Name: `\\.\DISPLAY2`, PhysicalBounds: bounds}
	screens := []*application.Screen{identical, primary}

	got := resolveScreen(screens, "", displayTarget{
		option: ScreenOption{IsPrimary: true},
		width:  bounds.Width,
		height: bounds.Height,
	})
	if got != primary {
		t.Fatalf("resolve = %+v, want the primary of two identical panels", got)
	}

	got = resolveScreen(screens, "", displayTarget{
		width:  bounds.Width,
		height: bounds.Height,
	})
	if got != identical {
		t.Fatalf("resolve = %+v, want the non-primary panel", got)
	}
}

// When the remembered screen is genuinely gone, reconnect must say so rather
// than pick any remaining monitor and put the service on the wrong wall.
func TestResolveScreenReturnsNilWhenTheScreenIsGone(t *testing.T) {
	remaining := &application.Screen{
		ID:   "c",
		Name: `\\.\DISPLAY3`,
		PhysicalBounds: application.Rect{
			Width: 3840, Height: 2160,
		},
	}

	got := resolveScreen([]*application.Screen{remaining}, "", displayTarget{
		option: ScreenOption{ID: "gone", Name: "Display 2 · 1920×1080"},
		device: `\\.\DISPLAY2`,
		width:  1920,
		height: 1080,
	})
	if got != nil {
		t.Fatalf("resolve = %+v, want nil - the chosen screen is not connected", got)
	}
}

// The whole point of the slice: the operator closing the window used to wipe
// the remembered output, so the studio could never offer it again. Closing
// drops only the intent - no reconnect is offered until a display is chosen
// - while the screen stays as the sensible default for the next open.
func TestClosingTheDisplayKeepsTheScreenButDropsTheIntent(t *testing.T) {
	app := newTestApp(t)
	app.target = displayTarget{
		option: ScreenOption{ID: "7", Name: "Display 2 · 1920×1080", IsPrimary: false},
		device: `\\.\DISPLAY2`,
		width:  1920,
		height: 1080,
	}
	app.wanted = true

	view, err := app.CloseAudienceDisplay()
	if err != nil {
		t.Fatalf("close display: %v", err)
	}
	if view.Wanted {
		t.Fatalf("close still offers a reconnect: %+v", view)
	}
	if view.ScreenID != "7" || view.ScreenName != "Display 2 · 1920×1080" {
		t.Fatalf("close forgot the output: %+v", view)
	}
	if app.target.device != `\\.\DISPLAY2` || app.target.width != 1920 {
		t.Fatalf("remembered target = %+v, want the screen kept for the next open", app.target)
	}
}
