//go:build windows

package releasecheck

import (
	"os"
	"path/filepath"
	"syscall"
	"testing"
)

func TestReplaceLocalValidationReceiptReplacesExistingFile(t *testing.T) {
	dir := t.TempDir()
	destination := filepath.Join(dir, "receipt.json")
	temp := filepath.Join(dir, "receipt.tmp")
	if err := os.WriteFile(destination, []byte("old"), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(temp, []byte("new"), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := replaceLocalValidationReceipt(temp, destination); err != nil {
		t.Fatal(err)
	}
	data, err := os.ReadFile(destination)
	if err != nil || string(data) != "new" {
		t.Fatalf("destination=%q err=%v", data, err)
	}
}

func TestReplaceLocalValidationReceiptFailurePreservesExistingFile(t *testing.T) {
	previousReplace := replaceFileCall
	previousMove := moveFileExCall
	defer func() {
		replaceFileCall = previousReplace
		moveFileExCall = previousMove
	}()
	replaceFileCall = func(destination, replacement uintptr) (uintptr, error) {
		return 0, syscall.ERROR_ACCESS_DENIED
	}
	moveFileExCall = func(source, destination, flags uintptr) (uintptr, error) {
		t.Fatal("MoveFileEx fallback must not run when an existing destination cannot be atomically replaced")
		return 0, syscall.ERROR_ACCESS_DENIED
	}

	dir := t.TempDir()
	destination := filepath.Join(dir, "receipt.json")
	temp := filepath.Join(dir, "receipt.tmp")
	if err := os.WriteFile(destination, []byte("old"), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(temp, []byte("new"), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := replaceLocalValidationReceipt(temp, destination); err == nil {
		t.Fatal("replace unexpectedly succeeded")
	}
	data, err := os.ReadFile(destination)
	if err != nil || string(data) != "old" {
		t.Fatalf("existing receipt was not preserved: %q err=%v", data, err)
	}
}
