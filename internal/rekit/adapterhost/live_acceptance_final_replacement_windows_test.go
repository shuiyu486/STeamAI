//go:build windows

package adapterhost

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestLiveAcceptanceCleanupPreservesFinalQuarantineReplacement(t *testing.T) {
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

	replacement := []byte("replacement\n")
	var replacementInfo os.FileInfo
	err = identity.cleanupWithHooks(nil, func(quarantinePath string) error {
		original := quarantinePath + ".original"
		if err := os.Rename(quarantinePath, original); err != nil {
			return err
		}
		if err := os.Mkdir(quarantinePath, 0o700); err != nil {
			return err
		}
		if err := os.WriteFile(filepath.Join(quarantinePath, "replacement.txt"), replacement, 0o600); err != nil {
			return err
		}
		replacementInfo, err = os.Lstat(quarantinePath)
		return err
	})
	if err == nil || !strings.Contains(err.Error(), "identity changed") {
		t.Fatalf("final quarantine replacement should fail closed: %v", err)
	}
	quarantinePath := caseRoot + ".cleanup"
	current, statErr := os.Lstat(quarantinePath)
	if statErr != nil || replacementInfo == nil || !os.SameFile(replacementInfo, current) {
		t.Fatalf("final quarantine replacement was removed: replacement=%v current=%v err=%v", replacementInfo, current, statErr)
	}
	got, readErr := os.ReadFile(filepath.Join(quarantinePath, "replacement.txt"))
	if readErr != nil || !bytes.Equal(got, replacement) {
		t.Fatalf("final quarantine replacement bytes changed: got=%q err=%v", got, readErr)
	}
}
