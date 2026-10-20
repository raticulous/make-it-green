// Command make-it-green is the CLI. It shares the engine with the desktop app.
package main

import (
	"context"
	"flag"
	"fmt"
	"os"
	"os/signal"
	"syscall"
	"time"

	"make-it-green/engine"
)

const usage = `Usage: make-it-green [COMMAND] [FLAGS]

Commands:
  run (default)        Keep your status green now, ignore work hours (Ctrl+C to stop)
  schedule             Keep your status green only inside work hours
  autostart on|off     Run "make-it-green schedule" at login
  status               Show config and autostart state

Flags (run, schedule):
  -interval N          Seconds between nudges
  -screen-off          Experimental: let the display sleep, keep your status green

Settings live in %s (the desktop app edits the same file).
`

func main() {
	cmd := "run"
	args := os.Args[1:]
	if len(args) > 0 && args[0][0] != '-' {
		cmd, args = args[0], args[1:]
	}
	if err := runCmd(cmd, args); err != nil {
		fmt.Fprintln(os.Stderr, "error:", err)
		os.Exit(1)
	}
}

func runCmd(cmd string, args []string) error {
	cfg, err := engine.LoadConfig()
	if err != nil {
		return fmt.Errorf("read %s: %w", engine.ConfigPath(), err)
	}
	switch cmd {
	case "run", "schedule":
		fs := flag.NewFlagSet(cmd, flag.ExitOnError)
		fs.IntVar(&cfg.IntervalSec, "interval", cfg.IntervalSec, "seconds between nudges")
		fs.BoolVar(&cfg.ScreenOff, "screen-off", cfg.ScreenOff, "let the display sleep")
		fs.Parse(args)
		cfg.Schedule = cmd == "schedule"

		ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
		defer stop()
		fmt.Printf("🟢 make-it-green: nudge every %ds", cfg.IntervalSec)
		if cfg.Schedule {
			fmt.Printf(", %02d:00-%02d:00", cfg.StartHour, cfg.EndHour)
			if cfg.WeekdaysOnly {
				fmt.Print(" Mon-Fri")
			}
		}
		fmt.Println(". Ctrl+C to stop.")
		engine.Run(ctx, cfg, func(s engine.State, err error) {
			ts := time.Now().Format("15:04")
			if err != nil {
				fmt.Printf("%s ⚠️  %v\n", ts, err)
				return
			}
			fmt.Printf("%s %s\n", ts, s)
		})
		fmt.Println("\n🛑 stopped.")
		return nil
	case "autostart":
		if len(args) != 1 || (args[0] != "on" && args[0] != "off") {
			return fmt.Errorf("usage: make-it-green autostart on|off")
		}
		if err := engine.SetAutostart(args[0] == "on", "schedule"); err != nil {
			return err
		}
		fmt.Println("autostart", args[0])
		return nil
	case "status":
		fmt.Printf("config:    %s\n%+v\nautostart: %v\n", engine.ConfigPath(), cfg, engine.AutostartEnabled())
		return nil
	case "help", "-h", "--help":
		fmt.Printf(usage, engine.ConfigPath())
		return nil
	}
	fmt.Printf(usage, engine.ConfigPath())
	return fmt.Errorf("unknown command %q", cmd)
}
