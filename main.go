package main

import (
	"embed"
	"log"

	"github.com/wailsapp/wails/v2"
	"github.com/wailsapp/wails/v2/pkg/options"
	"github.com/wailsapp/wails/v2/pkg/options/assetserver"
)

//go:embed all:frontend/dist
var assets embed.FS

func main() {
	app := NewApp()

	if err := wails.Run(&options.App{
		Title:     "Zowe Worship",
		Width:     1440,
		Height:    900,
		MinWidth:  960,
		MinHeight: 640,
		AssetServer: &assetserver.Options{
			Assets: assets,
		},
		BackgroundColour: &options.RGBA{R: 8, G: 15, B: 28, A: 255},
		OnStartup:        app.startup,
		Bind:             []interface{}{app},
	}); err != nil {
		log.Fatal(err)
	}
}
