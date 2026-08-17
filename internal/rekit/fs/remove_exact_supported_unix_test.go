//go:build linux || darwin

package fs

import (
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestAnchoredRootUnixRemoveGuardRejectsParentRebind(t *testing.T) {
	base := t.TempDir()
	rootPath := filepath.Join(base, "root")
	parentPath := filepath.Join(rootPath, "parent")
	if err := os.MkdirAll(parentPath, 0o700); err != nil {
		t.Fatal(err)
	}
	filePath := filepath.Join(parentPath, "intent.json")
	if err := os.WriteFile(filePath, []byte("expected"), 0o600); err != nil {
		t.Fatal(err)
	}
	root := openTestAnchoredRoot(t, rootPath)
	restore := setAnchoredRootBeforeRemoveHookForTest(func() error {
		if err := os.Rename(parentPath, filepath.Join(rootPath, "parent-moved")); err != nil {
			return err
		}
		if err := os.Mkdir(parentPath, 0o700); err != nil {
			return err
		}
		return os.WriteFile(filePath, []byte("replacement"), 0o600)
	})
	t.Cleanup(restore)

	err := root.RemoveExactFile("parent/intent.json", []byte("expected"), 0o600)
	if err == nil || !strings.Contains(err.Error(), "guard path changed") {
		t.Fatalf("parent rebind error=%v", err)
	}
	got, readErr := os.ReadFile(filePath)
	if readErr != nil || string(got) != "replacement" {
		t.Fatalf("replacement parent was mutated: %q err=%v", got, readErr)
	}
	got, readErr = os.ReadFile(filepath.Join(rootPath, "parent-moved", "intent.json"))
	if readErr != nil || string(got) != "expected" {
		t.Fatalf("original parent was mutated: %q err=%v", got, readErr)
	}
}

func TestAnchoredRootUnixRemoveExactObjectsFailClosed(t *testing.T) {
	rootPath := t.TempDir()
	emptyPath := filepath.Join(rootPath, "empty")
	if err := os.Mkdir(emptyPath, 0o700); err != nil {
		t.Fatal(err)
	}
	filePath := filepath.Join(rootPath, "intent.json")
	if err := os.WriteFile(filePath, []byte("expected"), 0o600); err != nil {
		t.Fatal(err)
	}
	root := openTestAnchoredRoot(t, rootPath)
	if err := root.RemoveExactFile("intent.json", []byte("expected"), 0o600); !errors.Is(err, errAnchoredExactMutationUnsupported) {
		t.Fatalf("exact file removal error=%v", err)
	}
	if err := root.RemoveEmptyDirectory("empty"); !errors.Is(err, errAnchoredExactMutationUnsupported) {
		t.Fatalf("exact directory removal error=%v", err)
	}
	got, err := os.ReadFile(filePath)
	if err != nil || string(got) != "expected" {
		t.Fatalf("exact file was mutated: %q err=%v", got, err)
	}
	entries, err := os.ReadDir(emptyPath)
	if err != nil || len(entries) != 0 {
		t.Fatalf("exact directory was mutated: entries=%v err=%v", entries, err)
	}
	if err := root.RemoveExactFile("missing.json", nil, 0o600); err != nil {
		t.Fatalf("missing exact file replay: %v", err)
	}
	if err := root.RemoveEmptyDirectory("missing"); err != nil {
		t.Fatalf("missing exact directory replay: %v", err)
	}
}
