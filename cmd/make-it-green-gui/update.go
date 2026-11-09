package main

import (
	"archive/tar"
	"archive/zip"
	"compress/gzip"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	goruntime "runtime"
	"strconv"
	"strings"
	"time"

	"github.com/wailsapp/wails/v2/pkg/runtime"

	"make-it-green/engine"
)

// version is set by the release build: -ldflags "-X main.version=v1.2.3". Dev builds never update.
var version = "dev"

const releasesAPI = "https://api.github.com/repos/raticulous/make-it-green/releases/latest"

// restartEnv tells a freshly updated instance to wait for the old one to quit, and whether to start green.
const restartEnv = "MAKE_IT_GREEN_RESTART"

// releaseAsset is the release file for this OS, as made by .github/workflows/release.yml.
func releaseAsset() string {
	switch {
	case goruntime.GOOS == "darwin":
		return "make-it-green-macos-universal.zip"
	case goruntime.GOOS == "linux" && goruntime.GOARCH == "amd64":
		return "make-it-green-linux-amd64.tar.gz"
	case goruntime.GOOS == "windows" && goruntime.GOARCH == "amd64":
		return "make-it-green-windows-amd64.zip"
	}
	return ""
}

// parseVersion reads "v1.2.3" (or "1.2.3"). Anything else, like "dev", is not a release.
func parseVersion(s string) (v [3]int, ok bool) {
	parts := strings.Split(strings.TrimPrefix(s, "v"), ".")
	if len(parts) != 3 {
		return v, false
	}
	for i, p := range parts {
		n, err := strconv.Atoi(p)
		if err != nil || n < 0 {
			return v, false
		}
		v[i] = n
	}
	return v, true
}

// newer reports whether release tag is a later version than current.
func newer(tag, current string) bool {
	t, ok1 := parseVersion(tag)
	c, ok2 := parseVersion(current)
	if !ok1 || !ok2 {
		return false
	}
	for i := range t {
		if t[i] != c[i] {
			return t[i] > c[i]
		}
	}
	return false
}

func (a *App) setUpdateStatus(s string) {
	a.mu.Lock()
	a.updateStatus = s
	a.mu.Unlock()
}

// UpdateStatus is the version line shown in settings.
func (a *App) UpdateStatus() string {
	a.mu.Lock()
	defer a.mu.Unlock()
	if a.updateStatus == "" {
		return "Version " + version
	}
	return "Version " + version + " · " + a.updateStatus
}

type release struct{ Tag, URL, Digest string }

func updatable() bool {
	_, ok := parseVersion(version)
	return ok && releaseAsset() != ""
}

// updateLoop checks a minute after start, then every 6 hours. It never installs on its own:
// it shows the window and asks, once per new version.
func (a *App) updateLoop() {
	if !updatable() {
		a.setUpdateStatus("this build does not update")
		return
	}
	for wait := time.Minute; ; wait = 6 * time.Hour {
		select {
		case <-a.ctx.Done():
			return
		case <-time.After(wait):
		}
		if c, err := engine.LoadConfig(); err != nil || !c.AutoUpdate {
			continue
		}
		tag, err := a.CheckForUpdate()
		if err != nil || tag == "" {
			continue
		}
		a.mu.Lock()
		ask := a.asked != tag
		a.asked = tag
		a.mu.Unlock()
		if ask {
			a.showWindow()
			runtime.EventsEmit(a.ctx, "update-available", tag)
		}
	}
}

// CheckForUpdate returns the tag of a newer release, or "" when this version is the latest.
func (a *App) CheckForUpdate() (string, error) {
	if !updatable() {
		return "", errors.New("this build does not update (dev build or unsupported system)")
	}
	ctx, cancel := context.WithTimeout(a.ctx, time.Minute)
	defer cancel()
	var rel struct {
		Tag    string `json:"tag_name"`
		Assets []struct {
			Name   string `json:"name"`
			URL    string `json:"browser_download_url"`
			Digest string `json:"digest"` // "sha256:<hex>"
		} `json:"assets"`
	}
	if err := getJSON(ctx, releasesAPI, &rel); err != nil {
		a.setUpdateStatus("check failed")
		return "", err
	}
	if !newer(rel.Tag, version) {
		a.setUpdateStatus("up to date")
		return "", nil
	}
	for _, as := range rel.Assets {
		if as.Name == releaseAsset() {
			a.mu.Lock()
			a.pending = release{rel.Tag, as.URL, as.Digest}
			a.updateStatus = rel.Tag + " is available"
			a.mu.Unlock()
			return rel.Tag, nil
		}
	}
	return "", fmt.Errorf("release %s has no %s", rel.Tag, releaseAsset())
}

// InstallUpdate installs the release found by CheckForUpdate, then restarts the app.
func (a *App) InstallUpdate() error {
	a.mu.Lock()
	r := a.pending
	a.mu.Unlock()
	if r.Tag == "" {
		return errors.New("no update to install")
	}
	ctx, cancel := context.WithTimeout(a.ctx, 10*time.Minute)
	defer cancel()
	exe, root, err := installRoot()
	if err == nil {
		err = install(ctx, root, r.URL, r.Digest)
	}
	if err != nil {
		a.setUpdateStatus("update failed")
		return err
	}
	a.restart(exe)
	return nil
}

func getJSON(ctx context.Context, url string, v any) error {
	req, _ := http.NewRequestWithContext(ctx, "GET", url, nil)
	req.Header.Set("Accept", "application/vnd.github+json")
	req.Header.Set("User-Agent", "make-it-green-updater")
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("GitHub: %s", resp.Status)
	}
	return json.NewDecoder(resp.Body).Decode(v)
}

// installRoot is the folder that holds what the release archive holds:
// the "Make It Green.app" bundle on macOS, the executables elsewhere.
func installRoot() (exe, root string, err error) {
	if exe, err = os.Executable(); err != nil {
		return "", "", err
	}
	if exe, err = filepath.EvalSymlinks(exe); err != nil {
		return "", "", err
	}
	root = filepath.Dir(exe)
	if i := strings.Index(exe, ".app/"); goruntime.GOOS == "darwin" && i >= 0 {
		root = filepath.Dir(exe[:i+len(".app")])
	}
	return exe, root, nil
}

// install downloads the archive, checks its sha256, and swaps every top-level entry
// that already exists in root (the GUI, the CLI, the .app bundle).
func install(ctx context.Context, root, url, digest string) error {
	want, ok := strings.CutPrefix(digest, "sha256:")
	if !ok {
		return errors.New("release file has no sha256 digest")
	}
	archive, err := download(ctx, url, want)
	if err != nil {
		return err
	}
	defer os.Remove(archive)

	// Stage inside root, so the swap below is a rename on one filesystem.
	stage, err := os.MkdirTemp(root, ".update-")
	if err != nil {
		return fmt.Errorf("cannot write to %s: %w", root, err)
	}
	defer os.RemoveAll(stage)
	if strings.HasSuffix(archive, ".zip") {
		err = unzip(archive, stage)
	} else {
		err = untar(archive, stage)
	}
	if err != nil {
		return err
	}

	entries, err := os.ReadDir(stage)
	if err != nil {
		return err
	}
	swapped := false
	for _, e := range entries {
		target := filepath.Join(root, e.Name())
		if _, err := os.Lstat(target); err != nil {
			continue // not installed here: do not add files the user never had
		}
		old := target + ".old"
		os.RemoveAll(old)
		// A running exe cannot be overwritten on Windows, but it can be renamed.
		if err := os.Rename(target, old); err != nil {
			return err
		}
		if err := os.Rename(filepath.Join(stage, e.Name()), target); err != nil {
			os.Rename(old, target)
			return err
		}
		os.RemoveAll(old) // fails for the running exe on Windows; cleanOldExe removes it at next start
		swapped = true
	}
	if !swapped {
		return fmt.Errorf("the release does not match the files in %s", root)
	}
	return nil
}

// cleanOldExe removes what install left behind when Windows held the old exe open.
func cleanOldExe() {
	if exe, _, err := installRoot(); err == nil {
		os.Remove(exe + ".old")
	}
}

func download(ctx context.Context, url, sha string) (string, error) {
	req, _ := http.NewRequestWithContext(ctx, "GET", url, nil)
	req.Header.Set("User-Agent", "make-it-green-updater")
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return "", fmt.Errorf("download: %s", resp.Status)
	}
	f, err := os.CreateTemp("", "make-it-green-*"+filepath.Ext(url))
	if err != nil {
		return "", err
	}
	h := sha256.New()
	_, err = io.Copy(io.MultiWriter(f, h), resp.Body)
	if cerr := f.Close(); err == nil {
		err = cerr
	}
	if err == nil && hex.EncodeToString(h.Sum(nil)) != strings.ToLower(sha) {
		err = errors.New("download is damaged (sha256 mismatch)")
	}
	if err != nil {
		os.Remove(f.Name())
		return "", err
	}
	return f.Name(), nil
}

// writeEntry writes one archive entry under dir. It refuses paths that leave dir.
func writeEntry(dir, name string, mode os.FileMode, linkTarget string, r io.Reader) error {
	if !filepath.IsLocal(name) {
		return fmt.Errorf("bad path in archive: %q", name)
	}
	path := filepath.Join(dir, name)
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return err
	}
	switch {
	case mode.IsDir():
		return os.MkdirAll(path, 0o755)
	case mode&os.ModeSymlink != 0:
		return os.Symlink(linkTarget, path)
	case mode.IsRegular():
		f, err := os.OpenFile(path, os.O_CREATE|os.O_WRONLY|os.O_TRUNC, mode.Perm()|0o600)
		if err != nil {
			return err
		}
		_, err = io.Copy(f, r)
		if cerr := f.Close(); err == nil {
			err = cerr
		}
		return err
	}
	return nil // skip devices and other odd entries
}

func untar(archive, dir string) error {
	f, err := os.Open(archive)
	if err != nil {
		return err
	}
	defer f.Close()
	gz, err := gzip.NewReader(f)
	if err != nil {
		return err
	}
	tr := tar.NewReader(gz)
	for {
		h, err := tr.Next()
		if err == io.EOF {
			return nil
		}
		if err != nil {
			return err
		}
		if name := filepath.Clean(h.Name); name != "." {
			if err := writeEntry(dir, name, h.FileInfo().Mode(), h.Linkname, tr); err != nil {
				return err
			}
		}
	}
}

func unzip(archive, dir string) error {
	zr, err := zip.OpenReader(archive)
	if err != nil {
		return err
	}
	defer zr.Close()
	for _, zf := range zr.File {
		mode := zf.Mode()
		if goruntime.GOOS != "windows" && mode.Perm() == 0 && !mode.IsDir() {
			mode |= 0o755 // zips made on Windows store no unix mode
		}
		rc, err := zf.Open()
		if err != nil {
			return err
		}
		link := ""
		if mode&os.ModeSymlink != 0 {
			b, _ := io.ReadAll(rc)
			link = string(b)
		}
		err = writeEntry(dir, zf.Name, mode, link, rc)
		rc.Close()
		if err != nil {
			return err
		}
	}
	return nil
}

// restart starts the new exe and quits. The new instance waits for this one to release
// the single-instance lock, and turns green if this one was green.
func (a *App) restart(exe string) {
	cmd := exec.Command(exe)
	cmd.Env = append(os.Environ(), restartEnv+"="+strconv.FormatBool(a.State() != "off"))
	if err := cmd.Start(); err != nil {
		a.setUpdateStatus("updated; restart the app to finish")
		return
	}
	runtime.Quit(a.ctx)
}
