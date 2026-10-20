package engine

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestEnableForceStateKeepsOtherKeys(t *testing.T) {
	path := filepath.Join(t.TempDir(), "config.json")
	os.WriteFile(path, []byte(`{"appTitle":"Work","idleDetection":{"forceState":false}}`), 0o644)

	changed, err := enableForceState(path, "/x/state")
	if err != nil || !changed {
		t.Fatalf("first call: changed=%v err=%v", changed, err)
	}
	b, _ := os.ReadFile(path)
	for _, want := range []string{`"appTitle": "Work"`, `"forceState": true`, `"stateFile": "/x/state"`} {
		if !strings.Contains(string(b), want) {
			t.Errorf("config missing %s:\n%s", want, b)
		}
	}
	if changed, _ := enableForceState(path, "/x/state"); changed {
		t.Error("second call should not rewrite the config")
	}
}
