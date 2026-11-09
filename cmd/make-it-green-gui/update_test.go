package main

import (
	"archive/tar"
	"bytes"
	"compress/gzip"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"
)

func TestNewer(t *testing.T) {
	cases := []struct {
		tag, cur string
		want     bool
	}{
		{"v0.4.0", "v0.3.0", true},
		{"v0.10.0", "v0.9.9", true},
		{"v1.0.0", "v0.99.0", true},
		{"v0.3.0", "v0.3.0", false},
		{"v0.2.9", "v0.3.0", false},
		{"v0.4.0", "dev", false},  // dev builds never update
		{"v0.4", "v0.3.0", false}, // not a release tag
	}
	for _, c := range cases {
		if got := newer(c.tag, c.cur); got != c.want {
			t.Errorf("newer(%q, %q) = %v, want %v", c.tag, c.cur, got, c.want)
		}
	}
}

func tarGz(t *testing.T, files map[string]string) []byte {
	var buf bytes.Buffer
	gz := gzip.NewWriter(&buf)
	tw := tar.NewWriter(gz)
	for name, body := range files {
		tw.WriteHeader(&tar.Header{Name: name, Mode: 0o755, Size: int64(len(body)), Typeflag: tar.TypeReg})
		tw.Write([]byte(body))
	}
	tw.Close()
	gz.Close()
	return buf.Bytes()
}

func TestInstallSwapsOnlyInstalledFiles(t *testing.T) {
	archive := tarGz(t, map[string]string{"./make-it-green-gui": "new gui", "./make-it-green": "new cli"})
	sum := sha256.Sum256(archive)
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) { w.Write(archive) }))
	defer srv.Close()

	root := t.TempDir()
	os.WriteFile(filepath.Join(root, "make-it-green-gui"), []byte("old gui"), 0o755) // the CLI is not installed

	if err := install(context.Background(), root, srv.URL+"/x.tar.gz", "sha256:"+hex.EncodeToString(sum[:])); err != nil {
		t.Fatal(err)
	}
	if b, _ := os.ReadFile(filepath.Join(root, "make-it-green-gui")); string(b) != "new gui" {
		t.Errorf("gui = %q, want new gui", b)
	}
	if st, err := os.Stat(filepath.Join(root, "make-it-green-gui")); err != nil || st.Mode().Perm()&0o100 == 0 {
		t.Errorf("gui lost its exec bit: %v %v", st, err)
	}
	if entries, _ := os.ReadDir(root); len(entries) != 1 {
		t.Errorf("root should hold only the gui, got %v", entries) // no CLI added, no .old or .update- left
	}

	// A wrong checksum must leave the installed file alone.
	if err := install(context.Background(), root, srv.URL+"/x.tar.gz", "sha256:00"); err == nil {
		t.Error("bad checksum was accepted")
	}
}
