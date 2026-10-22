//go:build !darwin

package main

import (
	"runtime"

	"fyne.io/systray"
)

// The tray runs its own loop on its own OS thread (Windows message loop, Linux D-Bus).
func (a *App) startTray() {
	go func() {
		runtime.LockOSThread()
		systray.Run(a.onTrayReady, nil)
	}()
}

func showInDock() {}

func showMenuImages() {}
