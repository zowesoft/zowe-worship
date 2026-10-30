package main

import (
	"errors"
	"testing"

	"zowe-worship/internal/presentation"
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

	state, err := app.applied(app.presentation.ProjectScripture(app.context(), "kjv", 43, 3, 16, 16))
	if err != nil {
		t.Fatalf("project: %v", err)
	}
	if state.Live == nil || state.Live.Reference != "John 3:16" {
		t.Fatalf("live after project = %+v", state.Live)
	}

	// A rejected command leaves the audience showing what it was already
	// showing, so a typo cannot blank the projector mid-service.
	if _, err := app.applied(app.presentation.PreviewScripture(app.context(), "kjv", 999, 1, 1, 1)); err == nil {
		t.Fatal("preview of an unknown book succeeded")
	}
}
