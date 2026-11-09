package engine

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"syscall"
)

// On Wayland, xdotool moves only the XWayland pointer: native Wayland apps (Edge/Chrome
// Teams PWA, Electron with ozone) and the compositor idle timer do not see it.
// ydotool writes to /dev/uinput, so every app sees the move.
func nudge() error {
	if os.Getenv("WAYLAND_DISPLAY") != "" {
		if err := exec.Command("ydotool", "mousemove", "-x", "1", "-y", "0").Run(); err != nil {
			return fmt.Errorf("ydotool: %w (install ydotool, add your user to the input group, run: systemctl --user enable --now ydotool)", err)
		}
		return exec.Command("ydotool", "mousemove", "-x", "-1", "-y", "0").Run()
	}
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
