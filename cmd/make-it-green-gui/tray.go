package main

import (
	_ "embed"
	goruntime "runtime"

	"fyne.io/systray"
	"github.com/wailsapp/wails/v2/pkg/runtime"

	"make-it-green/engine"
)

var (
	//go:embed icons/tray-on.png
	trayOnPNG []byte
	//go:embed icons/tray-paused.png
	trayPausedPNG []byte
	//go:embed icons/tray-off.png
	trayOffPNG []byte
	//go:embed icons/tray-on.ico
	trayOnICO []byte
	//go:embed icons/tray-paused.ico
	trayPausedICO []byte
	//go:embed icons/tray-off.ico
	trayOffICO []byte
)

// setTray shows a green leaf while the engine nudges, yellow when paused, faded grey when off.
func (a *App) setTray(state string) {
	on, paused, off := trayOnPNG, trayPausedPNG, trayOffPNG
	if goruntime.GOOS == "windows" {
		on, paused, off = trayOnICO, trayPausedICO, trayOffICO
	}
	icon := off
	switch state {
	case string(engine.Active):
		icon = on
	case string(engine.Paused):
		icon = paused
	}
	systray.SetIcon(icon)
	systray.SetTooltip("Make It Green: " + state)
	if item := a.toggleItem.Load(); item != nil {
		label := "Stop"
		if state == "off" {
			label = "Keep me green"
		}
		item.SetTitle(label)
	}
}

func (a *App) onTrayReady() {
	systray.SetTitle("Make It Green")
	show := systray.AddMenuItem("Open Make It Green", "Show the window")
	toggle := systray.AddMenuItem("Keep me green", "")
	a.toggleItem.Store(toggle)
	systray.AddSeparator()
	quit := systray.AddMenuItem("Quit completely", "Stop and quit")
	a.setTray(a.State())
	systray.SetOnTapped(a.showWindow)

	// onTrayReady must return: systray serves the menu only after it does.
	go func() {
		for {
			select {
			case <-show.ClickedCh:
				a.showWindow()
			case <-toggle.ClickedCh:
				if a.State() != "off" {
					a.Stop()
				} else if c, err := a.GetConfig(); err == nil {
					a.Start(c)
				}
			case <-quit.ClickedCh:
				runtime.Quit(a.ctx)
				return
			}
		}
	}()
}

func (a *App) showWindow() {
	showInDock()
	runtime.WindowShow(a.ctx)
	runtime.WindowUnminimise(a.ctx)
}
