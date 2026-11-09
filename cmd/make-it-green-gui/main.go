// Command make-it-green-gui is the desktop app. It shares the engine with the CLI.
package main

import (
	"bytes"
	"context"
	"embed"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	goruntime "runtime"
	"sync"
	"sync/atomic"
	"time"

	"fyne.io/systray"
	"github.com/wailsapp/wails/v2"
	"github.com/wailsapp/wails/v2/pkg/options"
	"github.com/wailsapp/wails/v2/pkg/options/assetserver"
	"github.com/wailsapp/wails/v2/pkg/options/linux"
	"github.com/wailsapp/wails/v2/pkg/runtime"

	"make-it-green/engine"
)

//go:embed frontend
var assets embed.FS

//go:embed build/appicon.png
var appIcon []byte

// App methods are callable from JS as window.go.main.App.<Method>.
type App struct {
	ctx   context.Context
	mu    sync.Mutex
	stop  context.CancelFunc
	last  string    // last engine state, for a UI that loads after the engine started
	since time.Time // when the status last turned green

	toggleItem atomic.Pointer[systray.MenuItem] // set once the tray is ready
	statusItem atomic.Pointer[systray.MenuItem]
}

func (a *App) GetConfig() (engine.Config, error) { return engine.LoadConfig() }
func (a *App) SaveConfig(c engine.Config) error  { return c.Save() }
func (a *App) Autostart() bool                   { return engine.AutostartEnabled() }
func (a *App) SetAutostart(on bool) error        { return engine.SetAutostart(on) }

func (a *App) State() string {
	a.mu.Lock()
	defer a.mu.Unlock()
	if a.stop == nil {
		return "off"
	}
	return a.last
}

// GreenSince is when the status turned green, in Unix ms (0 when not green).
func (a *App) GreenSince() int64 {
	a.mu.Lock()
	defer a.mu.Unlock()
	if a.stop == nil || a.since.IsZero() {
		return 0
	}
	return a.since.UnixMilli()
}

// Start saves c and runs the engine with it. It restarts a running engine
// and keeps the green time, so a settings change does not reset the counter.
func (a *App) Start(c engine.Config) error {
	c.Stopped = false
	if err := c.Save(); err != nil {
		return err
	}
	a.mu.Lock()
	defer a.mu.Unlock()
	if a.stop != nil {
		a.stop()
	}
	ctx, cancel := context.WithCancel(a.ctx)
	a.stop = cancel
	go engine.Run(ctx, c, func(s engine.State, err error) {
		msg := ""
		a.mu.Lock()
		if err != nil {
			msg = err.Error()
		} else {
			if s == engine.Active && a.last != string(engine.Active) {
				a.since = time.Now()
			}
			a.last = string(s)
		}
		since := a.since.UnixMilli()
		a.mu.Unlock()
		if err == nil {
			a.setTray(string(s))
		}
		runtime.EventsEmit(a.ctx, "state", string(s), msg, since)
	})
	return nil
}

// Pause runs the engine paused until c.ResumeAt and returns the saved config.
func (a *App) Pause(c engine.Config) (engine.Config, error) {
	c.PausedUntil = c.ResumeAt(time.Now()).UnixMilli()
	return c, a.Start(c)
}

// Resume ends a pause, or starts a stopped engine.
func (a *App) Resume(c engine.Config) (engine.Config, error) {
	c.PausedUntil = 0
	return c, a.Start(c)
}

// Stop turns the engine off and remembers it, so autostart does not start it at the next login.
func (a *App) Stop() {
	if c, err := engine.LoadConfig(); err == nil {
		c.Stopped, c.PausedUntil = true, 0
		c.Save()
	}
	a.mu.Lock()
	defer a.mu.Unlock()
	if a.stop != nil {
		a.stop()
		a.stop = nil
		a.last = "off"
		runtime.EventsEmit(a.ctx, "state", "off", "", 0)
		a.setTray("off")
	}
}

// installDesktopEntry lets Linux desktops show our icon in the taskbar and launcher.
// Wayland ignores window icons; it matches the window's app id (ProgramName) to a .desktop file.
func installDesktopEntry() error {
	exe, err := os.Executable()
	if err != nil {
		return err
	}
	data, err := os.UserHomeDir()
	if err != nil {
		return err
	}
	data = filepath.Join(data, ".local", "share")
	if d := os.Getenv("XDG_DATA_HOME"); d != "" {
		data = d
	}
	icon := filepath.Join(data, "icons", "hicolor", "1024x1024", "apps", "make-it-green.png")
	desktop := filepath.Join(data, "applications", "make-it-green.desktop")
	entry := fmt.Sprintf("[Desktop Entry]\nType=Application\nName=Make It Green\nComment=Keep your status green and the computer awake\n"+
		"Exec=%q\nIcon=%s\nStartupWMClass=make-it-green\nCategories=Utility;\nTerminal=false\n", exe, icon)
	for path, content := range map[string][]byte{icon: appIcon, desktop: []byte(entry)} {
		if old, err := os.ReadFile(path); err == nil && bytes.Equal(old, content) {
			continue
		}
		if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
			return err
		}
		if err := os.WriteFile(path, content, 0o644); err != nil {
			return err
		}
	}
	return nil
}

func main() {
	if goruntime.GOOS == "linux" {
		if err := installDesktopEntry(); err != nil {
			println("desktop entry:", err.Error())
		}
	}
	app := &App{}
	frontend, _ := fs.Sub(assets, "frontend")
	err := wails.Run(&options.App{
		Title:            "Make It Green",
		Width:            380,
		Height:           560,
		MinWidth:         340,
		MinHeight:        480,
		AssetServer:      &assetserver.Options{Assets: frontend},
		BackgroundColour: &options.RGBA{R: 15, G: 17, B: 16, A: 1},
		// Closing the window hides it; the tray icon brings it back or quits.
		HideWindowOnClose: true,
		SingleInstanceLock: &options.SingleInstanceLock{
			UniqueId:               "make-it-green",
			OnSecondInstanceLaunch: func(options.SecondInstanceData) { app.showWindow() },
		},
		Linux: &linux.Options{Icon: appIcon, ProgramName: "make-it-green"},
		OnStartup: func(ctx context.Context) {
			app.ctx = ctx
			app.startTray()
			// Autostart launches the app with no args, so it starts working right away.
			if c, err := engine.LoadConfig(); err == nil && engine.AutostartEnabled() && !c.Stopped {
				app.Start(c)
			}
		},
		Bind: []any{app},
	})
	if err != nil {
		println("error:", err.Error())
	}
}
