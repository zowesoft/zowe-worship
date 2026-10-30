package main

import (
	"embed"
	"log"

	"github.com/wailsapp/wails/v3/pkg/application"
)

//go:embed all:frontend/dist
var assets embed.FS

func main() {
	app := application.New(application.Options{
		Name:        "Zowe Worship",
		Description: "Scripture-first worship presentation",
		Services: []application.Service{
			application.NewService(NewApp()),
		},
		Assets: application.AssetOptions{
			Handler: application.AssetFileServerFS(assets),
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
