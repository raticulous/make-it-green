package engine

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"syscall"
)

// ponytail: X11/XWayland only via xdotool (what the old script used).
// Add ydotool/uinput when Teams or Slack runs as a native Wayland client.
func nudge() error {
	if err := exec.Command("xdotool", "mousemove_relative", "--", "1", "0").Run(); err != nil {
		return fmt.Errorf("xdotool: %w (install xdotool)", err)
	}
	return exec.Command("xdotool", "mousemove_relative", "--", "-1", "0").Run()
}

func inhibitSleep(display bool) (func(), error) {
	what := "sleep"
	if display {
		what = "sleep:idle"
	}
	cmd := exec.Command("systemd-inhibit", "--what="+what, "--who=make-it-green",
		"--why=Keeping your status green", "--mode=block", "sleep", "infinity")
	cmd.SysProcAttr = &syscall.SysProcAttr{Pdeathsig: syscall.SIGTERM} // no orphan lock if we crash
	if err := cmd.Start(); err != nil {
		return nil, fmt.Errorf("systemd-inhibit: %w", err)
	}
	return func() { cmd.Process.Kill(); cmd.Wait() }, nil
}

func displayOff() error {
	tries := [][]string{
		{"kscreen-doctor", "--dpms", "off"}, // KDE Wayland
		{"busctl", "--user", "set-property", "org.gnome.Mutter.DisplayConfig", "/org/gnome/Mutter/DisplayConfig",
			"org.gnome.Mutter.DisplayConfig", "PowerSaveMode", "i", "1"}, // GNOME Wayland
		{"xset", "dpms", "force", "off"}, // X11
	}
	if os.Getenv("WAYLAND_DISPLAY") == "" {
		tries = tries[2:]
	}
	var errs []string
	for _, t := range tries {
		err := exec.Command(t[0], t[1:]...).Run()
		if err == nil {
			return nil
		}
		errs = append(errs, t[0]+": "+err.Error())
	}
	return fmt.Errorf("cannot turn display off: %s", strings.Join(errs, "; "))
}

func autostartPath() string {
	dir, _ := os.UserConfigDir()
	return filepath.Join(dir, "autostart", "make-it-green.desktop")
}

func setAutostart(enable bool, exe string, args []string) error {
	if !enable {
		err := os.Remove(autostartPath())
		if os.IsNotExist(err) {
			return nil
		}
		return err
	}
	if err := os.MkdirAll(filepath.Dir(autostartPath()), 0o755); err != nil {
		return err
	}
	desktop := fmt.Sprintf("[Desktop Entry]\nType=Application\nName=Make It Green\nExec=%q %s\nX-GNOME-Autostart-enabled=true\n",
		exe, strings.Join(args, " "))
	return os.WriteFile(autostartPath(), []byte(desktop), 0o644)
}

func AutostartEnabled() bool {
	_, err := os.Stat(autostartPath())
	return err == nil
}

// teams-for-linux reads its idle state from a file when idleDetection.forceState is on.
// Writing "active" there keeps Teams green with no input, so the screen can sleep and lock.
func teamsForLinuxDirs() []string {
	home, _ := os.UserHomeDir()
	cfg, _ := os.UserConfigDir()
	var dirs []string
	for _, d := range []string{
		filepath.Join(cfg, "teams-for-linux"),
		filepath.Join(home, "snap", "teams-for-linux", "current", ".config", "teams-for-linux"),
		filepath.Join(home, ".var", "app", "com.github.IsmaelMartinez.teams_for_linux", "config", "teams-for-linux"),
	} {
		if st, err := os.Stat(d); err == nil && st.IsDir() {
			dirs = append(dirs, d)
		}
	}
	return dirs
}

const idleStateFile = "make-it-green-idle-state"

func keepTeamsActiveWithoutInput() (bool, error) {
	dirs := teamsForLinuxDirs()
	var errs []error
	for _, d := range dirs {
		state := filepath.Join(d, idleStateFile)
		if changed, err := enableForceState(filepath.Join(d, "config.json"), state); err != nil {
			errs = append(errs, err)
		} else if changed {
			errs = append(errs, fmt.Errorf("updated %s: restart the chat app once", filepath.Join(d, "config.json")))
		}
		// Rewrite every tick: teams-for-linux deletes the file when it exits.
		if err := os.WriteFile(state, []byte("active"), 0o644); err != nil {
			errs = append(errs, err)
		}
	}
	// Slack has no idle override, so it still needs the nudge.
	return len(dirs) > 0 && !slackRunning(), errors.Join(errs...)
}

func slackRunning() bool {
	comms, _ := filepath.Glob("/proc/[0-9]*/comm")
	for _, c := range comms {
		if b, err := os.ReadFile(c); err == nil && strings.TrimSpace(string(b)) == "slack" {
			return true
		}
	}
	return false
}

func releaseTeamsActive() {
	for _, d := range teamsForLinuxDirs() {
		os.Remove(filepath.Join(d, idleStateFile)) // no file = teams-for-linux uses normal idle detection
	}
}

// enableForceState sets idleDetection in config.json and keeps every other key.
func enableForceState(path, stateFile string) (bool, error) {
	conf := map[string]any{}
	if b, err := os.ReadFile(path); err == nil {
		if err := json.Unmarshal(b, &conf); err != nil {
			return false, fmt.Errorf("%s: %w", path, err)
		}
	} else if !os.IsNotExist(err) {
		return false, err
	}
	if idle, ok := conf["idleDetection"].(map[string]any); ok && idle["forceState"] == true && idle["stateFile"] == stateFile {
		return false, nil
	}
	conf["idleDetection"] = map[string]any{"forceState": true, "stateFile": stateFile}
	b, err := json.MarshalIndent(conf, "", "  ")
	if err != nil {
		return false, err
	}
	return true, os.WriteFile(path, b, 0o644)
}
