package main

/*
#include <dispatch/dispatch.h>
extern void nativeStart(void); // fyne.io/systray's C start, needs the AppKit main thread
static void startTrayOnMain(void) { dispatch_async(dispatch_get_main_queue(), ^{ nativeStart(); }); }
*/
import "C"

import "fyne.io/systray"

// Wails owns the macOS main loop, so the tray joins it instead of running its own.
func (a *App) startTray() {
	systray.RunWithExternalLoop(a.onTrayReady, nil) // registers; start is called below on the main thread
	C.startTrayOnMain()
}
