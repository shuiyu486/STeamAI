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

func TestOrdinaryInitPreviewRejectsNestedWindowsReparsePoint(t *testing.T) {
	repoRoot, pack := exclusiveInitFixture(t)
	caseRoot := t.TempDir()
	outside := t.TempDir()
	if err := os.Symlink(outside, filepath.Join(caseRoot, "linked")); err != nil {
		t.Skipf("Windows symlink creation unavailable: %v", err)
	}
	_, err := InitPreview(repoRoot, caseRoot, pack, ApplyOptions{ProjectName: "reparse", CreateLocalFiles: true, Command: "init"})
	if err == nil || !strings.Contains(strings.ToLower(err.Error()), "reparse") {
		t.Fatalf("ordinary reparse preview error = %v", err)
	}
	if _, statErr := os.Lstat(filepath.Join(caseRoot, ".rekit")); !os.IsNotExist(statErr) {
		t.Fatalf("reparse preview wrote .rekit: %v", statErr)
	}
}
