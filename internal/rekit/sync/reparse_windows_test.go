//go:build windows

package sync

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestRejectExclusiveInitReparsePathRejectsWindowsReparsePoint(t *testing.T) {
	root := t.TempDir()
	real := filepath.Join(root, "real")
	link := filepath.Join(root, "link")
	if err := os.Mkdir(real, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink(real, link); err != nil {
		t.Skipf("Windows symlink creation unavailable: %v", err)
	}
	if err := rejectExclusiveInitReparsePath(real); err != nil {
		t.Fatalf("regular directory rejected: %v", err)
	}
	if err := rejectExclusiveInitReparsePath(link); err == nil || !strings.Contains(err.Error(), "reparse point") {
		t.Fatalf("Windows reparse point error = %v", err)
	}
}
