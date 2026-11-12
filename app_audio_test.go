package main

import "testing"

func TestAudioTakeIsEmptyBeforeAnyCapture(t *testing.T) {
	app := newTestApp(t)

	if got := app.AudioTakeLast(20); got != "" {
		t.Fatalf("take last = %q, want nothing before a capture", got)
	}
	if got := app.AudioTakeRange(0, 16000); got != "" {
		t.Fatalf("take range = %q, want nothing before a capture", got)
	}
}

func TestAudioStartRefusesADeviceThatIsNotThere(t *testing.T) {
	app := newTestApp(t)

	if err := app.AudioStart("zowe-no-such-microphone"); err == nil {
		t.Fatal("starting a microphone that does not exist succeeded")
	}
	if app.audioService().Running() {
		t.Fatal("the capture service is running after a failed start")
	}
}

func TestAudioStopIsHarmlessWithoutACapture(t *testing.T) {
	app := newTestApp(t)

	if err := app.AudioStop(); err != nil {
		t.Fatalf("stop: %v", err)
	}
	if app.audioService().Running() {
		t.Fatal("the capture service is running after a stop")
	}
}
