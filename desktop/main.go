package main

import (
	"log"
	"os"

	"github.com/wailsapp/wails/v2"
	"github.com/wailsapp/wails/v2/pkg/options"
	"github.com/wailsapp/wails/v2/pkg/options/assetserver"
)

func main() {
	app := NewApp()
	if err := wails.Run(&options.App{
		Title:  "Genesis",
		Width:  1100,
		Height: 760,
		AssetServer: &assetserver.Options{
			Assets: os.DirFS("frontend/dist"),
		},
		OnStartup: app.startup,
		Bind: []any{
			app,
		},
	}); err != nil {
		log.Fatal(err)
	}
}
