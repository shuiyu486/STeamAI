//go:build windows

package packmemoryconsumption

import (
	"os"
	"os/exec"
	"path/filepath"
	"testing"
)

func TestAnchoredWriteRejectsJunctionParent(t *testing.T) {
	caseRoot := t.TempDir()
	outside := t.TempDir()
	junction := filepath.Join(caseRoot, "linked-parent")
	command := exec.Command("cmd.exe", "/c", "mklink", "/J", junction, outside)
	if output, err := command.CombinedOutput(); err != nil {
		t.Skipf("junction creation unavailable: %v: %s", err, output)
	}
	root, err := openPinnedCaseRoot(caseRoot)
	if err != nil {
		t.Fatal(err)
	}
	defer root.Close()
	if err := root.replaceExact(filepath.Join("linked-parent", "target.json"), nil, false, []byte("safe\n"), "junction fixture"); err == nil {
		t.Fatal("junction parent was accepted")
	}
	if _, err := os.Stat(filepath.Join(outside, "target.json")); !os.IsNotExist(err) {
		t.Fatalf("write escaped through junction: %v", err)
	}
}
