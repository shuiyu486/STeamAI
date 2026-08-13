//go:build !windows

package adapterhost

import (
	"bytes"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestRunCleanupQuarantinesReplacementAfterIdentityCheck(t *testing.T) {
	fixture := newHostFixture(t, "fixture/input.txt", 10, 4, 1)
	artifactRel := filepath.ToSlash(filepath.Join(
		"workspace",
		"main",
		"inspect",
		"session-1",
		"inspection.json",
	))
	artifactPath := filepath.Join(fixture.caseRoot, filepath.FromSlash(artifactRel))
	movedPath := filepath.Join(filepath.Dir(artifactPath), "owned-moved.json")
	replacement := []byte("replacement after cleanup identity check\n")
	fixture.options.testHooks = &hostTestHooks{
		beforeReportWrite: func() error { return os.ErrPermission },
		afterCleanupIdentityOpen: func(rel string) error {
			if rel != artifactRel {
				return nil
			}
			if err := os.Rename(artifactPath, movedPath); err != nil {
				return err
			}
			return os.WriteFile(artifactPath, replacement, 0o600)
		},
	}
	_, err := Run(fixture.options)
	if !errors.Is(err, os.ErrPermission) ||
		!strings.Contains(err.Error(), "quarantined output identity changed") {
		t.Fatalf("cleanup replacement race error=%v", err)
	}
	if got, readErr := os.ReadFile(artifactPath); readErr != nil ||
		!bytes.Equal(got, replacement) {
		t.Fatalf("cleanup deleted or changed replacement: got=%q err=%v", got, readErr)
	}
	if got, readErr := os.ReadFile(movedPath); readErr != nil || len(got) == 0 {
		t.Fatalf("cleanup changed moved exact-owned output: bytes=%d err=%v", len(got), readErr)
	}
}

func TestRunCleanupPreservesReplacementAfterQuarantineIdentityCheck(t *testing.T) {
	fixture := newHostFixture(t, "fixture/input.txt", 10, 4, 1)
	artifactRel := filepath.ToSlash(filepath.Join(
		"workspace",
		"main",
		"inspect",
		"session-1",
		"inspection.json",
	))
	artifactPath := filepath.Join(fixture.caseRoot, filepath.FromSlash(artifactRel))
	movedQuarantine := filepath.Join(filepath.Dir(artifactPath), "owned-quarantine-moved.json")
	replacement := []byte("replacement after quarantine identity check\n")
	fixture.options.testHooks = &hostTestHooks{
		beforeReportWrite: func() error { return os.ErrPermission },
		afterCleanupQuarantineIdentityCheck: func(rel, quarantine string) error {
			if rel != artifactRel {
				return nil
			}
			quarantinePath := filepath.Join(fixture.caseRoot, filepath.FromSlash(quarantine))
			if err := os.Rename(quarantinePath, movedQuarantine); err != nil {
				return err
			}
			return os.WriteFile(quarantinePath, replacement, 0o600)
		},
	}
	_, err := Run(fixture.options)
	if !errors.Is(err, os.ErrPermission) ||
		!strings.Contains(err.Error(), "identity changed after validation") {
		t.Fatalf("cleanup quarantine replacement race error=%v", err)
	}
	if got, readErr := os.ReadFile(artifactPath); readErr != nil || !bytes.Equal(got, replacement) {
		t.Fatalf("cleanup failed to restore canonical replacement: got=%q err=%v", got, readErr)
	}
	if got, readErr := os.ReadFile(movedQuarantine); readErr != nil || len(got) == 0 {
		t.Fatalf("cleanup changed moved exact-owned quarantine: bytes=%d err=%v", len(got), readErr)
	}
	matches, globErr := filepath.Glob(filepath.Join(
		filepath.Dir(artifactPath),
		".inspection.json.rekit-owned-cleanup-*",
	))
	if globErr != nil || len(matches) != 1 {
		t.Fatalf("cleanup replacement quarantine paths=%v err=%v", matches, globErr)
	}
	if got, readErr := os.ReadFile(matches[0]); readErr != nil || !bytes.Equal(got, replacement) {
		t.Fatalf("cleanup deleted or changed quarantine replacement: got=%q err=%v", got, readErr)
	}
}
