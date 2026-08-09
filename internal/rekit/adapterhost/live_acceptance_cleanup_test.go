package adapterhost

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestLiveAcceptanceCleanupRefusesCaseReplacementAfterQuarantine(t *testing.T) {
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
	err = identity.cleanup(func(string) error {
		if err := os.Mkdir(caseRoot, 0o700); err != nil {
			return err
		}
		return os.WriteFile(filepath.Join(caseRoot, "replacement.txt"), replacement, 0o600)
	})
	if err == nil || !strings.Contains(err.Error(), "replacement created at the case root") {
		t.Fatalf("case replacement should fail closed: %v", err)
	}
	got, readErr := os.ReadFile(filepath.Join(caseRoot, "replacement.txt"))
	if readErr != nil || !bytes.Equal(got, replacement) {
		t.Fatalf("cleanup changed replacement bytes: got=%q err=%v", got, readErr)
	}
	quarantine := caseRoot + ".cleanup"
	if _, statErr := os.Lstat(quarantine); statErr != nil {
		t.Fatalf("failed cleanup should retain quarantine: %v", statErr)
	}
}

func TestLiveAcceptanceCleanupRemovesCapturedCase(t *testing.T) {
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
	if err := identity.cleanup(nil); err != nil {
		t.Fatal(err)
	}
	for _, path := range []string{caseRoot, caseRoot + ".cleanup"} {
		if _, err := os.Lstat(path); !os.IsNotExist(err) {
			t.Fatalf("cleanup retained %s: %v", path, err)
		}
	}
}
