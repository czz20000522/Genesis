package main

import (
	"context"
	_ "embed"
	"sync"

	"github.com/getlantern/systray"
	wailsruntime "github.com/wailsapp/wails/v2/pkg/runtime"
)

//go:embed build/windows/icon.ico
var trayIcon []byte

type desktopTray struct {
	app  *App
	once sync.Once
}

func (t *desktopTray) register() {
	if t == nil {
		return
	}
	systray.Register(func() {
		systray.SetIcon(trayIcon)
		systray.SetTooltip("Genesis")
		show := systray.AddMenuItem("显示 Genesis", "显示 Genesis 窗口")
		quit := systray.AddMenuItem("退出 Genesis", "关闭 Genesis 与其自有服务")
		go func() {
			for range show.ClickedCh {
				t.app.showExistingWindow()
			}
		}()
		go func() {
			for range quit.ClickedCh {
				t.app.quitFromTray()
			}
		}()
	}, nil)
}

func (t *desktopTray) quit() {
	if t != nil {
		t.once.Do(systray.Quit)
	}
}

func (a *App) beforeClose(context.Context) bool {
	a.closeMu.Lock()
	minimize := a.closeBehavior == closeBehaviorTray && !a.forceQuit
	a.closeMu.Unlock()
	if minimize && a.ctx != nil {
		wailsruntime.WindowHide(a.ctx)
	}
	return minimize
}

func (a *App) quitFromTray() {
	a.requestExit()
}

func (a *App) requestExit() {
	if a == nil {
		return
	}
	a.closeMu.Lock()
	a.forceQuit = true
	ctx := a.ctx
	a.closeMu.Unlock()
	if ctx != nil {
		wailsruntime.Quit(ctx)
	}
}
