package main

import (
	"testing"
)

func TestMediaControlValidatesCommands(t *testing.T) {
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

	// The studio itself plays when no display is connected, so a valid command
	// is accepted even though these tests have no desktop to deliver it to.
	if err := app.MediaControl(MediaCommand{Action: "pause"}); err != nil {
		t.Fatalf("pause with no display err = %v, want nil", err)
	}

	// The display's own report must not fall over when there is no desktop to
	// tell, which is the case in tests and in a window that has just closed.
	app.MediaEnded()
	app.MediaProgress(1200, 60000)
}
