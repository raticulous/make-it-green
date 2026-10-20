package main

/*
#cgo CFLAGS: -x objective-c
#cgo LDFLAGS: -framework Cocoa
#import <Cocoa/Cocoa.h>
extern void nativeStart(void); // fyne.io/systray's C start, needs the AppKit main thread
static void startTrayOnMain(void) { dispatch_async(dispatch_get_main_queue(), ^{ nativeStart(); }); }

// Closing the window hides the app (HideWindowOnClose). Then drop the Dock icon,
// so the app lives only in the menu bar until the tray opens it again.
static void hideDockWhenHidden(void) {
	dispatch_async(dispatch_get_main_queue(), ^{
		[[NSNotificationCenter defaultCenter] addObserverForName:NSApplicationDidHideNotification object:nil queue:nil
			usingBlock:^(NSNotification *n) { [NSApp setActivationPolicy:NSApplicationActivationPolicyAccessory]; }];
	});
}

// Runs before Wails shows the window: both go through the main queue in order.
static void showInDock(void) {
	dispatch_async(dispatch_get_main_queue(), ^{
		[NSApp setActivationPolicy:NSApplicationActivationPolicyRegular];
		[NSApp unhide:nil];
		[NSApp activateIgnoringOtherApps:YES];
	});
}
*/
import "C"

import "fyne.io/systray"

// Wails owns the macOS main loop, so the tray joins it instead of running its own.
func (a *App) startTray() {
	systray.RunWithExternalLoop(a.onTrayReady, nil) // registers; start is called below on the main thread
	C.startTrayOnMain()
	C.hideDockWhenHidden()
}

func showInDock() { C.showInDock() }
