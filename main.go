//go:build !displaycheck

package main

import (
	"log"
	"net/http"
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

	app.Window.NewWithOptions(application.WebviewWindowOptions{
		Name:             studioWindow,
		Title:            "Zowe Worship",
		Width:            1200,
		Height:           660,
		MinWidth:         960,
		MinHeight:        600,
		StartState:       application.WindowStateMaximised,
		BackgroundColour: application.NewRGB(8, 15, 28),
		URL:              "/",
	})

	if err := app.Run(); err != nil {
		log.Fatal(err)
	}
}
