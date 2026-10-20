package engine

/*
#cgo LDFLAGS: -framework ApplicationServices
#include <ApplicationServices/ApplicationServices.h>

static int nudge(void) {
	CGEventRef cur = CGEventCreate(NULL);
	if (!cur) return -1;
	CGPoint p = CGEventGetLocation(cur);
	CFRelease(cur);
	for (int dx = 1; dx >= 0; dx--) {
		CGEventRef e = CGEventCreateMouseEvent(NULL, kCGEventMouseMoved, CGPointMake(p.x + dx, p.y), kCGMouseButtonLeft);
		if (!e) return -1;
		CGEventPost(kCGHIDEventTap, e);
		CFRelease(e);
	}
	return 0;
}
*/
import "C"

import (
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
)

// Needs Accessibility permission (System Settings > Privacy & Security > Accessibility).
func nudge() error {
	if C.nudge() != 0 {
		return errors.New("cannot post mouse event (grant Accessibility permission)")
	}
	return nil
}

func inhibitSleep(display bool) (func(), error) {
	flags := "-i"
	if display {
		flags = "-di"
	}
	// -w exits caffeinate when we exit, so a crash never leaves the Mac awake forever.
	cmd := exec.Command("caffeinate", flags, "-w", strconv.Itoa(os.Getpid()))
	if err := cmd.Start(); err != nil {
		return nil, fmt.Errorf("caffeinate: %w", err)
	}
	return func() { cmd.Process.Kill(); cmd.Wait() }, nil
}

func displayOff() error {
	return exec.Command("pmset", "displaysleepnow").Run()
}

func autostartPath() string {
	home, _ := os.UserHomeDir()
	return filepath.Join(home, "Library", "LaunchAgents", "com.make-it-green.plist")
}

func setAutostart(enable bool, exe string, args []string) error {
	if !enable {
		err := os.Remove(autostartPath())
		if os.IsNotExist(err) {
			return nil
		}
		return err
	}
	progArgs := "<string>" + exe + "</string>"
	for _, a := range args {
		progArgs += "<string>" + a + "</string>"
	}
	plist := `<?xml version="1.0" encoding="UTF-8"?>
<!DOCTYPE plist PUBLIC "-//Apple//DTD PLIST 1.0//EN" "http://www.apple.com/DTDs/PropertyList-1.0.dtd">
<plist version="1.0"><dict>
<key>Label</key><string>com.make-it-green</string>
<key>ProgramArguments</key><array>` + progArgs + `</array>
<key>RunAtLoad</key><true/>
</dict></plist>
`
	if err := os.MkdirAll(filepath.Dir(autostartPath()), 0o755); err != nil {
		return err
	}
	return os.WriteFile(autostartPath(), []byte(plist), 0o644)
}

func AutostartEnabled() bool {
	_, err := os.Stat(autostartPath())
	return err == nil
}

// Teams on this OS has no idle override, so screen-off mode falls back to nudge + blank.
func keepTeamsActiveWithoutInput() (bool, error) { return false, nil }
func releaseTeamsActive()                        {}
