//go:build windows

package main

import (
	"os"
	"os/exec"
	"strings"
	"testing"
)

// A real window is the only way to show that the audience output survives the
// room: the unit tests cover the decisions, this one drives an actual window
// through open, close and reconnect.
//
// A Wails application has to own the process's main thread, so the check runs
// as the diagnostic build of the program rather than inside the test binary:
//
//	ZOPRESENT_LIVE_WINDOWS=1 go test -run TestLiveAudienceWindow .
//
// The machine has to allow it: the check opens a real window on a real screen.
const (
	liveWindowEnv = "ZOPRESENT_LIVE_WINDOWS"
	liveWindowOK  = "the audience window opened, closed and came back"
)

func TestLiveAudienceWindowSurvivesCloseAndReconnect(t *testing.T) {
	if os.Getenv(liveWindowEnv) == "" {
		t.Skipf("set %s=1 to drive a real audience window", liveWindowEnv)
	}

	command := exec.Command("go", "run", "-tags", "displaycheck", ".")
	out, err := command.CombinedOutput()
	t.Logf("display check:\n%s", out)
	if err != nil {
		t.Fatalf("the live window run failed: %v", err)
	}
	if !strings.Contains(string(out), liveWindowOK) {
		t.Fatalf("the display check did not report a surviving window:\n%s", out)
	}
}
