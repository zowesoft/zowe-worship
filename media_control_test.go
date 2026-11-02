package main

import (
	"errors"
	"testing"
)

func TestMediaControlNeedsAPlayer(t *testing.T) {
	app := newTestApp(t)

	if err := app.MediaControl("spin"); err == nil {
		t.Fatal("an unknown command was accepted")
	}
	if err := app.MediaControl("pause"); !errors.Is(err, errNoAudienceDisplay) {
		t.Fatalf("err = %v, want errNoAudienceDisplay", err)
	}

	// The display's own report must not fall over when there is no desktop to
	// tell, which is the case in tests and in a window that has just closed.
	app.MediaEnded()
}
