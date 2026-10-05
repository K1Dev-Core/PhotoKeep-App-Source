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
	app, err := NewApp("")
	if err != nil {
		log.Fatal(err)
	}
	defer app.close()
	err = wails.Run(&options.App{
		Title: "PhotoKeep — สำรองรูปภาพ",
		Width: 1180, Height: 800, MinWidth: 360, MinHeight: 520,
		AssetServer:      &assetserver.Options{Assets: assets},
		BackgroundColour: &options.RGBA{R: 247, G: 249, B: 246, A: 1},
		OnStartup:        app.Startup,
		Bind:             []interface{}{app},
	})
	if err != nil {
		log.Fatal(err)
	}
}
