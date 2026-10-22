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
	//go:embed icons/tray-on-template.png
	trayOnTemplate []byte
	//go:embed icons/tray-paused-template.png
	trayPausedTemplate []byte
	//go:embed icons/tray-off-template.png
	trayOffTemplate []byte

	// macOS menu icons, made by assets/icons/menu-icons.swift
	//go:embed icons/status-on.png
	statusOnPNG []byte
	//go:embed icons/status-paused.png
	statusPausedPNG []byte
	//go:embed icons/status-off.png
	statusOffPNG []byte
	//go:embed icons/menu-open.png
	menuOpenPNG []byte
	//go:embed icons/menu-start.png
	menuStartPNG []byte
	//go:embed icons/menu-stop.png
	menuStopPNG []byte
	//go:embed icons/menu-quit.png
	menuQuitPNG []byte
)

// menuIcon sets an SF Symbol icon on a menu item. Only macOS: Windows wants .ico and Linux themes may hide black icons.
func menuIcon(item *systray.MenuItem, png []byte) {
	if goruntime.GOOS == "darwin" {
		item.SetTemplateIcon(png, png)
	}
}

// setTray shows a green leaf while the engine nudges, yellow when paused, faded grey when off.
// macOS gets monochrome template leaves instead (check / plain / outline) so the menu bar tints them like its other icons.
func (a *App) setTray(state string) {
	on, paused, off := trayOnPNG, trayPausedPNG, trayOffPNG
	switch goruntime.GOOS {
	case "windows":
		on, paused, off = trayOnICO, trayPausedICO, trayOffICO
	case "darwin":
		on, paused, off = trayOnTemplate, trayPausedTemplate, trayOffTemplate
	}
	icon, dot, status := off, statusOffPNG, "Make It Green is stopped"
	switch state {
	case string(engine.Active):
		icon, dot, status = on, statusOnPNG, "Make It Green is running"
	case string(engine.Paused):
		icon, dot, status = paused, statusPausedPNG, "Make It Green is paused (outside work hours)"
	}
	if goruntime.GOOS == "darwin" {
		systray.SetTemplateIcon(icon, icon)
	} else {
		systray.SetIcon(icon)
	}
	systray.SetTooltip(status)
	if item := a.statusItem.Load(); item != nil {
		item.SetTitle(status)
		if goruntime.GOOS == "darwin" {
			item.SetIcon(dot) // not a template: the dot keeps its color
		}
	}
	if item := a.toggleItem.Load(); item != nil {
		label, png := "Stop", menuStopPNG
		if state == "off" {
			label, png = "Keep me green", menuStartPNG
		}
		item.SetTitle(label)
		menuIcon(item, png)
	}
}

func (a *App) onTrayReady() {
	status := systray.AddMenuItem("", "")
	status.Disable() // a status line, like Docker Desktop's "is running"
	a.statusItem.Store(status)
	systray.AddSeparator()
	show := systray.AddMenuItem("Open", "Show the window")
	menuIcon(show, menuOpenPNG)
	toggle := systray.AddMenuItem("Keep me green", "")
	a.toggleItem.Store(toggle)
	systray.AddSeparator()
	quit := systray.AddMenuItem("Quit Make It Green", "Stop and quit")
	menuIcon(quit, menuQuitPNG)
	a.setTray(a.State())
	showMenuImages()
	// no SetOnTapped: a click on the icon then opens this menu, not the window

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
