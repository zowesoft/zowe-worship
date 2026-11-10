package main

import (
	"errors"
	"testing"
)

func TestMediaControlNeedsAPlayer(t *testing.T) {
	app := newTestApp(t)

	if err := app.MediaControl(MediaCommand{Action: "spin"}); err == nil {
		t.Fatal("an unknown command was accepted")
	}
	if err := app.MediaControl(MediaCommand{Action: "seek", Value: 1.5}); err == nil {
		t.Fatal("a seek past the end of the clip was accepted")
	}
	if err := app.MediaControl(MediaCommand{Action: "volume", Value: 140}); err == nil {
		t.Fatal("a volume over 100 was accepted")
	}
	if err := app.MediaControl(MediaCommand{Action: "pause"}); !errors.Is(err, errNoAudienceDisplay) {
		t.Fatalf("err = %v, want errNoAudienceDisplay", err)
	}
	if err := app.MediaControl(MediaCommand{Action: "stop"}); !errors.Is(err, errNoAudienceDisplay) {
		t.Fatalf("stop to no player err = %v, want errNoAudienceDisplay", err)
	}

	// The display's own report must not fall over when there is no desktop to
	// tell, which is the case in tests and in a window that has just closed.
	app.MediaEnded()
	app.MediaProgress(1200, 60000)
}
