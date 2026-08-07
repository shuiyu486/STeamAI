//go:build windows

package sessionhost

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

func TestWriteLiveAcceptanceReceiptRejectsAncestorJunction(t *testing.T) {
	base := t.TempDir()
	realParent := filepath.Join(base, "real-parent")
	if err := os.Mkdir(realParent, 0o700); err != nil {
		t.Fatal(err)
	}
	linkedParent := filepath.Join(base, "linked-parent")
	if output, err := exec.Command("cmd.exe", "/c", "mklink", "/J", linkedParent, realParent).CombinedOutput(); err != nil {
		t.Skipf("Windows directory junction unavailable: %v (%s)", err, strings.TrimSpace(string(output)))
	}
	path := filepath.Join(linkedParent, "nested", "receipt.json")
	err := WriteLiveAcceptanceReceipt(path, LiveAcceptanceReceipt{Passed: true})
	if err == nil || (!strings.Contains(err.Error(), "reparse point") && !strings.Contains(err.Error(), "non-symlink directory")) {
		t.Fatalf("ancestor junction receipt error=%v", err)
	}
	if _, err := os.Lstat(filepath.Join(realParent, "nested", "receipt.json")); !os.IsNotExist(err) {
		t.Fatalf("ancestor junction publication escaped into target: %v", err)
	}
}
