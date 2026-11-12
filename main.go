//go:build !displaycheck

package main

import (
	"log"
	"net/http"
	"runtime"
	"strings"

	"github.com/wailsapp/wails/v3/pkg/application"
)

func main() {
	studio := NewApp()

	app := application.New(application.Options{
		Name:        "Zowe Worship",
		Description: "Scripture-first worship presentation",
		// One studio per machine. Two processes would each hold their own
		// preview/live state while driving the same projector and the same
		// database, so an operator could send different slides without seeing
		// the other window. The lock keys on this identifier rather than the
		// executable name, so a renamed copy is still refused.
		SingleInstance: &application.SingleInstanceOptions{
			UniqueID: "com.zowesoft.zowe-worship",
			OnSecondInstanceLaunch: func(application.SecondInstanceData) {
				// Refusing the second launch has to be visible, or it reads as
				// the app failing to start.
				desktop := application.Get()
				if desktop == nil {
					return
				}
				if studio, ok := desktop.Window.GetByName(studioWindow); ok {
					studio.UnMinimise()
					studio.Focus()
				}
			},
		},
		Windows: application.WindowsOptions{
			// The audience display is never clicked, so Chromium's autoplay rule
			// would refuse to start a projected clip with sound.
			AdditionalBrowserArgs: []string{"--autoplay-policy=no-user-gesture-required", "--remote-debugging-port=9333"},
		},
		Services: []application.Service{
			application.NewService(studio),
		},
		Assets: application.AssetOptions{
			Handler: application.AssetFileServerFS(assets),
			Middleware: func(next http.Handler) http.Handler {
				return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
					if strings.HasPrefix(r.URL.Path, "/media/") {
						studio.serveMedia(w, r)
						return
					}
					next.ServeHTTP(w, r)
				})
			},
		},
	})

	createMenu(app, studio)

	app.Window.NewWithOptions(application.WebviewWindowOptions{
		Name:               studioWindow,
		Title:              "Zowe Worship",
		Width:              1200,
		Height:             660,
		MinWidth:           960,
		MinHeight:          600,
		StartState:         application.WindowStateMaximised,
		UseApplicationMenu: true,
		BackgroundColour:   application.NewRGB(8, 15, 28),
		URL:                "/",
	})

	if err := app.Run(); err != nil {
		log.Fatal(err)
	}
}

// createMenu puts a native menu bar on the studio window. EasyWorship's
// ribbon holds the actions an operator reaches for constantly; a menu bar is
// the platform-native home for them on Windows, macOS and Linux alike.
func createMenu(app *application.App, studio *App) {
	menu := app.NewMenu()
	if runtime.GOOS == "darwin" {
		menu.AddRole(application.AppMenu)
	}

	file := menu.AddSubmenu("File")
	file.Add("New Service").SetAccelerator("CmdOrCtrl+N").OnClick(func(*application.Context) {
		if _, err := studio.CreateService("Untitled service"); err != nil {
			log.Printf("zowe: new service from menu: %v", err)
		}
	})
	file.Add("Backup Now").OnClick(func(*application.Context) {
		if _, err := studio.BackupNow(); err != nil {
			log.Printf("zowe: backup from menu: %v", err)
		}
	})
	file.AddSeparator()
	file.Add("Exit").OnClick(func(*application.Context) { app.Quit() })

	menu.AddRole(application.EditMenu)

	live := menu.AddSubmenu("Live")
	live.Add("Project live").OnClick(func(*application.Context) {
		if _, err := studio.SendPreviewLive(); err != nil {
			log.Printf("zowe: project live from menu: %v", err)
		}
	})
	live.Add("Clear display").OnClick(func(*application.Context) {
		if _, err := studio.ClearLive(); err != nil {
			log.Printf("zowe: clear display from menu: %v", err)
		}
	})
	live.AddSeparator()
	live.Add("Emergency disable detection").OnClick(func(*application.Context) {
		if err := studio.SetDetectionDisabled(true); err != nil {
			log.Printf("zowe: disable detection from menu: %v", err)
		}
	})
	live.Add("Enable detection").OnClick(func(*application.Context) {
		if err := studio.SetDetectionDisabled(false); err != nil {
			log.Printf("zowe: enable detection from menu: %v", err)
		}
	})

	menu.AddRole(application.WindowMenu)

	view := menu.AddSubmenu("View")
	view.Add("Toggle fullscreen").SetAccelerator("F11").OnClick(func(*application.Context) {
		if window, ok := app.Window.GetByName(studioWindow); ok {
			if window.IsFullscreen() {
				window.UnFullscreen()
			} else {
				window.Fullscreen()
			}
		}
	})

	menu.AddRole(application.HelpMenu)
	app.Menu.Set(menu)
}
