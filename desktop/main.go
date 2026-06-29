package main

import (
	"log"
	"os"
	"path/filepath"

	"github.com/wailsapp/wails/v2"
	"github.com/wailsapp/wails/v2/pkg/options"
	"github.com/wailsapp/wails/v2/pkg/options/assetserver"
	wailsruntime "github.com/wailsapp/wails/v2/pkg/runtime"
)

func main() {
	app := NewApp()
	if err := wails.Run(&options.App{
		Title:              "Genesis",
		Width:              1100,
		Height:             760,
		SingleInstanceLock: singleInstanceLock(app),
		AssetServer: &assetserver.Options{
			Assets: os.DirFS(frontendAssetDirFromRuntime()),
		},
		OnStartup: app.startup,
		Bind: []any{
			app,
		},
	}); err != nil {
		log.Fatal(err)
	}
}

func singleInstanceLock(app *App) *options.SingleInstanceLock {
	return &options.SingleInstanceLock{
		UniqueId: "genesis-desktop-local-shell",
		OnSecondInstanceLaunch: func(options.SecondInstanceData) {
			app.showExistingWindow()
		},
	}
}

func (a *App) showExistingWindow() {
	if a == nil || a.ctx == nil {
		return
	}
	wailsruntime.WindowUnminimise(a.ctx)
	wailsruntime.WindowShow(a.ctx)
}

func frontendAssetDirFromRuntime() string {
	exe, _ := os.Executable()
	cwd, _ := os.Getwd()
	return frontendAssetDir(exe, cwd)
}

func frontendAssetDir(executablePath string, cwd string) string {
	candidates := []string{}
	if executablePath != "" {
		candidates = append(candidates, filepath.Clean(filepath.Join(filepath.Dir(executablePath), "..", "..", "frontend", "dist")))
	}
	if cwd != "" {
		candidates = append(candidates,
			filepath.Join(cwd, "frontend", "dist"),
			filepath.Join(cwd, "desktop", "frontend", "dist"),
		)
	}
	for _, candidate := range candidates {
		if info, err := os.Stat(candidate); err == nil && info.IsDir() {
			return candidate
		}
	}
	if len(candidates) > 0 {
		return candidates[0]
	}
	return filepath.Join("frontend", "dist")
}
