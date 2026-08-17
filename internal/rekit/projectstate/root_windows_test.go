//go:build windows

package projectstate

import (
	"os/exec"
	"path/filepath"
	"testing"
)

func TestResolveRejectsStateRootJunction(t *testing.T) {
	for _, stateDir := range []string{CurrentDir, LegacyDir} {
		t.Run(stateDir, func(t *testing.T) {
			caseRoot := t.TempDir()
			target := t.TempDir()
			junction := filepath.Join(caseRoot, stateDir)
			if output, err := exec.Command("cmd.exe", "/c", "mklink", "/J", junction, target).CombinedOutput(); err != nil {
				t.Skipf("junction unavailable: %v: %s", err, output)
			}
			if _, err := Resolve(caseRoot); err == nil {
				t.Fatalf("junction %s state root passed validation", stateDir)
			}
		})
	}
}

func TestResolveRejectsCaseRootAncestorJunction(t *testing.T) {
	target := t.TempDir()
	junction := filepath.Join(t.TempDir(), "linked-parent")
	if output, err := exec.Command("cmd.exe", "/c", "mklink", "/J", junction, target).CombinedOutput(); err != nil {
		t.Skipf("junction unavailable: %v: %s", err, output)
	}
	if _, err := Resolve(filepath.Join(junction, "case")); err == nil {
		t.Fatal("case root beneath junction ancestor passed validation")
	}
}
