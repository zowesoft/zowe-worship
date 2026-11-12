//go:build displaycheck

package main

// The display check is a diagnostic build of the same program: it opens the
// audience output on a real screen, closes it the way the room does, and
// reconnects it - the one thing the unit tests cannot prove, because a
// Wails application has to own the process's main thread.
//
// It is a build tag rather than a flag so the shipped binary stays as it is:
//
//	go build -tags displaycheck -o bin/display-check.exe .
//
// The check prints what it saw and exits non-zero when the output did not
// survive, so a test (or a support engineer) can run it and trust it.
import (
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"time"

	"github.com/wailsapp/wails/v3/pkg/application"
)

func main() {
	root, err := os.MkdirTemp("", "zopresent-display-check")
	if err != nil {
		fmt.Println("display check failed:", err)
		os.Exit(1)
	}
	defer os.RemoveAll(root)

	studio := newAppAt(filepath.Join(root, "Zopresent"))
	verdict := make(chan error, 1)

	app := application.New(application.Options{
		Name: "Zopresent display check",
		Services: []application.Service{
			application.NewService(studio),
			application.NewService(&displayCheck{studio: studio, verdict: verdict}),
		},
		Assets: application.AssetOptions{Handler: application.AssetFileServerFS(assets)},
		Windows: application.WindowsOptions{
			// Its own browser profile: the app that is running on this
			// machine already holds the default one.
			WebviewUserDataPath: filepath.Join(root, "webview"),
			// The audience window is the only one here, and closing it must
			// not end the check: the studio is what keeps the real app alive.
			DisableQuitOnLastWindowClosed: true,
		},
	})

	finished := make(chan error, 1)
	go func() {
		err := <-verdict
		finished <- err
		app.Quit()
	}()

	runErr := app.Run()

	var outcome error
	select {
	case outcome = <-finished:
	case <-time.After(15 * time.Second):
		outcome = errors.New("the display check never finished")
	}
	if runErr != nil {
		fmt.Println("the display check app stopped:", runErr)
	}
	if outcome != nil {
		fmt.Println("display check failed:", outcome)
		os.Exit(1)
	}
	fmt.Println("display check: the audience window opened, closed and came back")
}

// displayCheck drives the audience output through open, close and reconnect
// once the application is up, and reports what happened.
type displayCheck struct {
	studio  *App
	verdict chan error
}

func (c *displayCheck) ServiceStartup(_ context.Context, _ application.ServiceOptions) error {
	// The application is still starting, so the check waits for it rather
	// than blocking the start.
	go func() { c.verdict <- c.run() }()
	return nil
}

func (c *displayCheck) ServiceShutdown(context.Context) error { return nil }

func (c *displayCheck) run() error {
	desktop := application.Get()
	if desktop == nil {
		return errors.New("the display check app has no desktop")
	}

	// The screens only exist once the platform is up.
	var screens []ScreenOption
	deadline := time.Now().Add(20 * time.Second)
	for time.Now().Before(deadline) {
		var err error
		screens, err = c.studio.ListScreens()
		if err == nil && len(screens) > 0 {
			break
		}
		time.Sleep(100 * time.Millisecond)
	}
	if len(screens) == 0 {
		return errors.New("this machine offers no screen for the audience display")
	}
	fmt.Printf("display check screens: %+v\n", screens)

	// The studio picks the screen the way the operator does - from the list
	// it is offered - and the output goes to a real one.
	view, err := c.studio.OpenAudienceDisplay(screens[0].ID)
	if err != nil {
		return fmt.Errorf("open the audience display: %w", err)
	}
	if !view.Open {
		return fmt.Errorf("display view = %+v, want the display open", view)
	}
	if _, ok := desktop.Window.GetByName(audienceWindow); !ok {
		return errors.New("the audience window was not created")
	}
	fmt.Printf("display check opened: %+v\n", view)

	// The room closes the window, and the operator's intent must survive it.
	c.studio.display.Close()
	deadline = time.Now().Add(20 * time.Second)
	for time.Now().Before(deadline) {
		if _, open := desktop.Window.GetByName(audienceWindow); !open {
			break
		}
		time.Sleep(50 * time.Millisecond)
	}
	if _, open := desktop.Window.GetByName(audienceWindow); open {
		return errors.New("the audience window did not go away")
	}

	after := c.studio.GetAudienceDisplay()
	if after.Open {
		return fmt.Errorf("display view = %+v, want the display closed", after)
	}
	if !after.Wanted {
		return fmt.Errorf("display view = %+v, want the reconnect still offered", after)
	}
	if after.ScreenID == "" {
		return fmt.Errorf("display view = %+v, want the screen remembered", after)
	}
	fmt.Printf("display check after the close: %+v\n", after)

	// Reconnecting brings a real window back.
	again, err := c.studio.ReconnectAudienceDisplay()
	if err != nil {
		return fmt.Errorf("reconnect the audience display: %w", err)
	}
	if !again.Open {
		return fmt.Errorf("display view = %+v, want the display open again", again)
	}
	if _, ok := desktop.Window.GetByName(audienceWindow); !ok {
		return errors.New("the audience window was not recreated")
	}
	fmt.Printf("display check reconnected: %+v\n", again)
	return nil
}
