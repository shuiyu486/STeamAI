//go:build windows

package adapterhost

import (
	"bytes"
	"os"
	"path/filepath"
	"testing"
)

func TestLiveAcceptanceQuarantineDoesNotReplaceCompetitor(t *testing.T) {
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
	quarantine := caseRoot + ".cleanup"
	competitor := []byte("competitor\n")
	if err := os.Mkdir(quarantine, 0o700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(quarantine, "competitor.txt"), competitor, 0o600); err != nil {
		t.Fatal(err)
	}
	if err := identity.cleanup(nil); err == nil {
		t.Fatal("existing quarantine competitor was replaced")
	}
	if got, err := os.ReadFile(filepath.Join(quarantine, "competitor.txt")); err != nil || !bytes.Equal(got, competitor) {
		t.Fatalf("quarantine competitor changed: got=%q err=%v", got, err)
	}
	if got, err := os.ReadFile(filepath.Join(caseRoot, markerName)); err != nil || !bytes.Equal(got, marker) {
		t.Fatalf("owned case moved despite quarantine collision: got=%q err=%v", got, err)
	}
}
