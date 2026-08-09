//go:build windows

package adapterhost

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestLiveAcceptanceCleanupRefusesReparsePoint(t *testing.T) {
	parent := t.TempDir()
	caseRoot := filepath.Join(parent, "case")
	if err := os.Mkdir(caseRoot, 0o700); err != nil {
		t.Fatal(err)
	}
	markerName := ".marker"
	marker := []byte("owned\n")
	if err := os.WriteFile(filepath.Join(caseRoot, markerName), marker, 0o600); err != nil {
		t.Fatal(err)
	}
	identity, err := captureLiveAcceptanceCase(caseRoot, markerName, marker)
	if err != nil {
		t.Fatal(err)
	}
	defer identity.Close()
	target := t.TempDir()
	outside := filepath.Join(target, "outside.txt")
	if err := os.WriteFile(outside, []byte("outside\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	err = identity.cleanup(func(quarantine string) error {
		return os.Symlink(outside, filepath.Join(quarantine, "outside-link"))
	})
	if err != nil && strings.Contains(strings.ToLower(err.Error()), "privilege") {
		t.Skipf("symlink unavailable: %v", err)
	}
	if err == nil || !strings.Contains(err.Error(), "reparse point") {
		t.Fatalf("reparse point should fail closed: %v", err)
	}
	if got, readErr := os.ReadFile(outside); readErr != nil || string(got) != "outside\n" {
		t.Fatalf("cleanup traversed reparse target: got=%q err=%v", got, readErr)
	}
}
