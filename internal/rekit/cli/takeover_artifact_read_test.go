package cli

import (
	"os"
	"path/filepath"
	"testing"
	"time"
)

func TestStatusReadTakeoverArtifactRequiresStableFileSnapshot(t *testing.T) {
	path := filepath.Join(t.TempDir(), "takeover.json")
	if err := os.WriteFile(path, []byte(`{"ready":true}`), 0o600); err != nil {
		t.Fatal(err)
	}
	data, state, warnings := statusReadTakeoverArtifact(path)
	if state != "readable" || string(data) != `{"ready":true}` || len(warnings) != 0 {
		t.Fatalf("stable artifact read = state %q data %q warnings %v", state, data, warnings)
	}

	data, state, warnings = statusReadTakeoverArtifactWithHook(path, func() {
		future := time.Now().Add(2 * time.Second)
		if err := os.Chtimes(path, future, future); err != nil {
			t.Fatal(err)
		}
	})
	if data != nil || state != "stale-file-snapshot" || len(warnings) == 0 {
		t.Fatalf("changed artifact read = state %q data %q warnings %v", state, data, warnings)
	}

	data, state, warnings = statusReadTakeoverArtifactWithHooks(path, func() {
		replacement := filepath.Join(filepath.Dir(path), "replacement.json")
		if err := os.WriteFile(replacement, []byte(`{"ready":false}`), 0o600); err != nil {
			t.Fatal(err)
		}
		if err := os.Rename(replacement, path); err != nil {
			t.Fatal(err)
		}
	}, nil)
	if data != nil || state != "stale-file-snapshot" || len(warnings) == 0 {
		t.Fatalf("replaced artifact read = state %q data %q warnings %v", state, data, warnings)
	}
}
