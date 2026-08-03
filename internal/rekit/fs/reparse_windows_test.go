//go:build windows

package fs

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestReadStableRegularFileAnchoredRejectsWindowsReparseDirectory(t *testing.T) {
	caseRoot := t.TempDir()
	real := filepath.Join(caseRoot, "real")
	if err := os.Mkdir(real, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(real, "evidence.txt"), []byte("evidence\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	link := filepath.Join(caseRoot, "link")
	if err := os.Symlink(real, link); err != nil {
		t.Skipf("Windows reparse creation unavailable: %v", err)
	}
	if _, err := ReadStableRegularFileAnchored(caseRoot, filepath.Join(link, "evidence.txt"), "evidence", 1024); err == nil || (!strings.Contains(err.Error(), "reparse") && !strings.Contains(err.Error(), "symlink")) {
		t.Fatalf("Windows reparse component error = %v", err)
	}
}
