# make-it-green

Keeps Microsoft Teams and Slack status green (Available / Active) and keeps the computer awake.
The same mouse nudge works for both desktop apps, so no setup per app is needed.
One engine (`engine/`), two front ends:

- `make-it-green`: CLI
- `make-it-green-gui`: desktop app (Wails, HTML/CSS UI)

Both read and write the same settings file (`make-it-green status` prints its path).

## How it works

- Every N seconds (default 50, max 240) it moves the mouse 1 px and back. Teams and Slack see activity (Teams goes Away after about 5 minutes idle, Slack after about 10).
- While active it holds an OS sleep lock: `SetThreadExecutionState` (Windows), `caffeinate` (macOS), `systemd-inhibit` (Linux).
- Optional schedule: work hours and weekdays only.

## Install

Download a release from GitHub Actions (push a `v*` tag), or build:

```sh
# CLI (Linux/Windows: no cgo; macOS: needs Xcode tools)
go build -o make-it-green ./cmd/make-it-green

# Desktop app
go install github.com/wailsapp/wails/v2/cmd/wails@v2.17.0
cd cmd/make-it-green-gui && wails build -skipbindings   # Linux: add -tags webkit2_41
```

Requirements per OS:

| OS | Needs |
|---|---|
| Linux | X11: `xdotool`. Wayland: `ydotool`, user in the `input` group, `systemctl --user enable --now ydotool`. GUI needs `libwebkit2gtk-4.1` and `libgtk-3` |
| macOS | Accessibility permission (System Settings > Privacy & Security > Accessibility) |
| Windows | WebView2 runtime (included in Windows 10/11) |

## CLI

```
make-it-green                 keep green now, ignore work hours
make-it-green schedule        keep green only inside work hours
make-it-green -screen-off     experimental: let the display sleep
make-it-green autostart on    run "make-it-green schedule" at login
make-it-green status
```

## Can the computer sleep or the screen turn off, with Teams or Slack still green?

**Screen off, computer awake: yes, experimental (`-screen-off`).**
The app holds a "system awake" lock but lets the display sleep. It turns the display off after each nudge
(`SC_MONITORPOWER` on Windows, `pmset displaysleepnow` on macOS, `kscreen-doctor` / Mutter D-Bus / `xset` on Linux).
Limits:

- On Windows and macOS the synthetic mouse move can wake the display for a moment. The app turns it off again at once.
- If the OS locks the screen when the display turns off, Teams and Slack show Away. Turn that lock off.
- The mode re-blanks the display after every nudge. Stop it when you are back.

**Computer asleep (suspend): no, not from this computer.**
A sleeping computer runs no code and has no network, so the Teams client disconnects.
Microsoft Graph [`setUserPreferredPresence`](https://learn.microsoft.com/en-us/graph/api/presence-setuserpreferredpresence?view=graph-rest-1.0)
applies only while at least one presence session exists, otherwise Teams shows Offline.
A server job that calls Graph `setPresence` can create such a session, but it needs the
`Presence.ReadWrite.All` application permission (admin consent in your tenant).
Teams on your phone also keeps a session, which is the simplest option.
Slack is the same: the Slack phone app keeps you Active, a sleeping computer does not.
