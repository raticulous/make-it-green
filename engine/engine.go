// Package engine keeps Teams and Slack green and the computer awake. CLI and GUI share it.
package engine

import (
	"context"
	"encoding/json"
	"errors"
	"io/fs"
	"os"
	"path/filepath"
	"time"
)

type Config struct {
	IntervalSec  int  `json:"interval_sec"`
	Schedule     bool `json:"schedule"` // only run inside work hours
	StartHour    int  `json:"start_hour"`
	EndHour      int  `json:"end_hour"`
	WeekdaysOnly bool `json:"weekdays_only"`
	ScreenOff    bool `json:"screen_off"` // experimental: let the display sleep, keep Teams and Slack green
}

func DefaultConfig() Config {
	return Config{IntervalSec: 50, StartHour: 9, EndHour: 18, WeekdaysOnly: true}
}

func ConfigPath() string {
	dir, err := os.UserConfigDir()
	if err != nil {
		dir = os.TempDir()
	}
	return filepath.Join(dir, "make-it-green", "config.json")
}

// LoadConfig returns defaults when no config file exists yet.
func LoadConfig() (Config, error) {
	c := DefaultConfig()
	b, err := os.ReadFile(ConfigPath())
	if errors.Is(err, fs.ErrNotExist) {
		return c, nil
	}
	if err != nil {
		return c, err
	}
	return c, json.Unmarshal(b, &c)
}

func (c Config) Save() error {
	if err := os.MkdirAll(filepath.Dir(ConfigPath()), 0o755); err != nil {
		return err
	}
	b, _ := json.MarshalIndent(c, "", "  ")
	return os.WriteFile(ConfigPath(), b, 0o644)
}

// InHours reports whether t is inside [StartHour, EndHour) on an allowed day.
func (c Config) InHours(t time.Time) bool {
	if c.WeekdaysOnly && (t.Weekday() == time.Saturday || t.Weekday() == time.Sunday) {
		return false
	}
	return t.Hour() >= c.StartHour && t.Hour() < c.EndHour
}

type State string

const (
	Active State = "active"
	Paused State = "paused (outside work hours)"
)

// Run nudges the mouse every interval until ctx ends.
// onState gets every state change and every nudge error; it can be nil.
func Run(ctx context.Context, c Config, onState func(State, error)) error {
	if onState == nil {
		onState = func(State, error) {}
	}
	if c.IntervalSec < 5 {
		c.IntervalSec = 5
	}
	var release func()
	defer func() {
		if release != nil {
			release()
		}
		releaseTeamsActive()
	}()

	tick := time.NewTicker(time.Duration(c.IntervalSec) * time.Second)
	defer tick.Stop()
	var last State
	for {
		state := Active
		if c.Schedule && !c.InHours(time.Now()) {
			state = Paused
		}
		if state != last {
			if state == Active {
				r, err := inhibitSleep(!c.ScreenOff)
				if err != nil {
					onState(state, err) // not fatal: nudges still keep most systems awake
				}
				release = r
			} else {
				if release != nil {
					release()
					release = nil
				}
				releaseTeamsActive()
			}
			last = state
			onState(state, nil)
		}
		if state == Active {
			quiet := false
			if c.ScreenOff {
				// Best case: tell Teams directly that we are active. No input, so the
				// OS turns the screen off and locks it on its own schedule.
				var err error
				if quiet, err = keepTeamsActiveWithoutInput(); err != nil {
					onState(state, err)
				}
			}
			if !quiet {
				if err := nudge(); err != nil {
					onState(state, err)
				}
			}
			if c.ScreenOff && !quiet {
				// ponytail: the nudge wakes the screen, so blank it again. This re-blanks
				// even when the user is back; add user-return detection if it annoys.
				if err := displayOff(); err != nil {
					onState(state, err)
				}
			}
		}
		select {
		case <-ctx.Done():
			return nil
		case <-tick.C:
		}
	}
}

// SetAutostart registers (or removes) this executable with args to run at login.
func SetAutostart(enable bool, args ...string) error {
	exe, err := os.Executable()
	if err != nil {
		return err
	}
	return setAutostart(enable, exe, args)
}
