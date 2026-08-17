//go:build windows

package fs

import (
	"errors"
	"os"
	"path/filepath"
	"testing"
)

func TestAnchoredRootMutationGuardsRejectRebindsBeforeSideEffects(t *testing.T) {
	for _, test := range []struct {
		name string
		move func(base, rootPath string) error
	}{
		{name: "root", move: func(base, rootPath string) error { return os.Rename(rootPath, filepath.Join(base, "root-moved")) }},
		{name: "source-parent", move: func(_ string, rootPath string) error {
			return os.Rename(filepath.Join(rootPath, "source-parent"), filepath.Join(rootPath, "source-parent-moved"))
		}},
		{name: "target-parent", move: func(_ string, rootPath string) error {
			return os.Rename(filepath.Join(rootPath, "target-parent"), filepath.Join(rootPath, "target-parent-moved"))
		}},
	} {
		t.Run(test.name, func(t *testing.T) {
			base := t.TempDir()
			rootPath := filepath.Join(base, "root")
			if err := os.MkdirAll(filepath.Join(rootPath, "source-parent"), 0o700); err != nil {
				t.Fatal(err)
			}
			if err := os.MkdirAll(filepath.Join(rootPath, "target-parent"), 0o700); err != nil {
				t.Fatal(err)
			}
			if err := os.WriteFile(filepath.Join(rootPath, "source-parent", "source"), []byte("expected"), 0o600); err != nil {
				t.Fatal(err)
			}
			root := openTestAnchoredRoot(t, rootPath)
			hookCalled := false
			restore := setAnchoredRootBeforeRenameHookForTest(func() error {
				hookCalled = true
				if err := test.move(base, rootPath); err == nil {
					return errors.New("mutation guard allowed rebind")
				}
				return errors.New("hook stopped before mutation")
			})
			t.Cleanup(restore)
			err := root.RenameFileNoReplaceExact("source-parent/source", "target-parent/target", []byte("expected"), 0o600)
			if err == nil || !hookCalled {
				t.Fatalf("guarded rename err=%v hook=%t", err, hookCalled)
			}
			if got, readErr := os.ReadFile(filepath.Join(rootPath, "source-parent", "source")); readErr != nil || string(got) != "expected" {
				t.Fatalf("source changed: %q err=%v", got, readErr)
			}
			if _, statErr := os.Lstat(filepath.Join(rootPath, "target-parent", "target")); !os.IsNotExist(statErr) {
				t.Fatalf("target published: %v", statErr)
			}
			assertNoExactMutationGuard(t, rootPath, filepath.Join(rootPath, "source-parent"), filepath.Join(rootPath, "target-parent"))
		})
	}
}

func TestAnchoredRootRemoveGuardRejectsParentRebindBeforeSideEffect(t *testing.T) {
	rootPath := t.TempDir()
	if err := os.Mkdir(filepath.Join(rootPath, "parent"), 0o700); err != nil {
		t.Fatal(err)
	}
	path := filepath.Join(rootPath, "parent", "file")
	if err := os.WriteFile(path, []byte("expected"), 0o600); err != nil {
		t.Fatal(err)
	}
	root := openTestAnchoredRoot(t, rootPath)
	restore := setAnchoredRootBeforeRemoveHookForTest(func() error {
		if err := os.Rename(filepath.Join(rootPath, "parent"), filepath.Join(rootPath, "parent-moved")); err == nil {
			return errors.New("mutation guard allowed remove-parent rebind")
		}
		return errors.New("hook stopped before removal")
	})
	t.Cleanup(restore)
	if err := root.RemoveExactFile("parent/file", []byte("expected"), 0o600); err == nil {
		t.Fatal("remove accepted parent rebind attempt")
	}
	if got, err := os.ReadFile(path); err != nil || string(got) != "expected" {
		t.Fatalf("source removed or changed: %q err=%v", got, err)
	}
	assertNoExactMutationGuard(t, rootPath, filepath.Join(rootPath, "parent"))
}

func TestAnchoredRootStableReadRejectsInPlaceMutation(t *testing.T) {
	for _, test := range []struct {
		name   string
		mutate func(string) error
	}{
		{name: "append", mutate: func(path string) error {
			file, err := os.OpenFile(path, os.O_APPEND|os.O_WRONLY, 0)
			if err != nil {
				return err
			}
			defer file.Close()
			_, err = file.Write([]byte("x"))
			return err
		}},
		{name: "overwrite", mutate: func(path string) error {
			file, err := os.OpenFile(path, os.O_WRONLY, 0)
			if err != nil {
				return err
			}
			defer file.Close()
			_, err = file.WriteAt([]byte("X"), 0)
			return err
		}},
	} {
		t.Run(test.name, func(t *testing.T) {
			rootPath := t.TempDir()
			path := filepath.Join(rootPath, "file")
			if err := os.WriteFile(path, []byte("expected"), 0o600); err != nil {
				t.Fatal(err)
			}
			root := openTestAnchoredRoot(t, rootPath)
			restore := setAnchoredRootReadAfterOpenHookForTest(func() error {
				if err := test.mutate(path); err == nil {
					return errors.New("exact read allowed in-place mutation")
				}
				return errors.New("in-place mutation rejected")
			})
			t.Cleanup(restore)
			if _, _, err := root.ReadStableFile("file", 64); err == nil {
				t.Fatal("stable read accepted mutation attempt")
			}
			if got, err := os.ReadFile(path); err != nil || string(got) != "expected" {
				t.Fatalf("file changed: %q err=%v", got, err)
			}
		})
	}
}

func TestAnchoredRootReplayRejectsInPlaceMutation(t *testing.T) {
	for _, test := range []struct {
		name   string
		mutate func(string) error
	}{
		{name: "append", mutate: func(path string) error {
			file, err := os.OpenFile(path, os.O_APPEND|os.O_WRONLY, 0)
			if err != nil {
				return err
			}
			defer file.Close()
			_, err = file.Write([]byte("x"))
			return err
		}},
		{name: "overwrite", mutate: func(path string) error {
			file, err := os.OpenFile(path, os.O_WRONLY, 0)
			if err != nil {
				return err
			}
			defer file.Close()
			_, err = file.WriteAt([]byte("X"), 0)
			return err
		}},
	} {
		t.Run(test.name, func(t *testing.T) {
			rootPath := t.TempDir()
			path := filepath.Join(rootPath, "file")
			if err := os.WriteFile(path, []byte("expected"), 0o600); err != nil {
				t.Fatal(err)
			}
			root := openTestAnchoredRoot(t, rootPath)
			restore := setAnchoredRootExactFileAfterOpenHookForTest(func() error {
				if err := test.mutate(path); err == nil {
					return errors.New("exact replay allowed in-place mutation")
				}
				return errors.New("in-place replay mutation rejected")
			})
			t.Cleanup(restore)
			if replay, err := root.WriteExclusiveFileWriteThrough("file", []byte("expected"), 0o600, true); err == nil || replay {
				t.Fatalf("exact replay accepted mutation attempt: replay=%t err=%v", replay, err)
			}
			if got, err := os.ReadFile(path); err != nil || string(got) != "expected" {
				t.Fatalf("replay file changed: %q err=%v", got, err)
			}
		})
	}
}

func TestAnchoredRootNewWriteRejectsInPlaceMutation(t *testing.T) {
	for _, test := range []struct {
		name   string
		mutate func(string) error
	}{
		{name: "append", mutate: func(path string) error {
			file, err := os.OpenFile(path, os.O_APPEND|os.O_WRONLY, 0)
			if err != nil {
				return err
			}
			defer file.Close()
			_, err = file.Write([]byte("x"))
			return err
		}},
		{name: "overwrite", mutate: func(path string) error {
			file, err := os.OpenFile(path, os.O_WRONLY, 0)
			if err != nil {
				return err
			}
			defer file.Close()
			_, err = file.WriteAt([]byte("X"), 0)
			return err
		}},
	} {
		t.Run(test.name, func(t *testing.T) {
			rootPath := t.TempDir()
			path := filepath.Join(rootPath, "file")
			root := openTestAnchoredRoot(t, rootPath)
			restore := setAnchoredRootWriteBeforeFinalValidationHookForTest(func() error {
				if err := test.mutate(path); err == nil {
					return errors.New("exact write allowed in-place mutation")
				}
				return errors.New("in-place write mutation rejected")
			})
			t.Cleanup(restore)
			if _, err := root.WriteExclusiveFileWriteThrough("file", []byte("expected"), 0o600, false); err == nil {
				t.Fatal("new write accepted in-place mutation")
			}
			if _, err := os.Lstat(path); !os.IsNotExist(err) {
				t.Fatalf("failed write left a side effect: %v", err)
			}
		})
	}
}

func assertNoExactMutationGuard(t *testing.T, paths ...string) {
	t.Helper()
	for _, path := range paths {
		if _, err := os.Lstat(filepath.Join(path, exactMutationGuardName)); !os.IsNotExist(err) {
			t.Fatalf("exact mutation guard remains in %s: %v", path, err)
		}
	}
}

func TestAnchoredRootDirectoryRenameRejectsMutationAfterFinalValidation(t *testing.T) {
	for _, test := range []struct {
		name   string
		mutate func(string) error
	}{
		{name: "append", mutate: func(path string) error {
			file, err := os.OpenFile(path, os.O_APPEND|os.O_WRONLY, 0)
			if err != nil {
				return err
			}
			defer file.Close()
			_, err = file.Write([]byte("x"))
			return err
		}},
		{name: "overwrite", mutate: func(path string) error {
			file, err := os.OpenFile(path, os.O_WRONLY, 0)
			if err != nil {
				return err
			}
			defer file.Close()
			_, err = file.WriteAt([]byte("X"), 0)
			return err
		}},
	} {
		t.Run(test.name, func(t *testing.T) {
			rootPath := t.TempDir()
			source := filepath.Join(rootPath, "stage")
			if err := os.Mkdir(source, 0o700); err != nil {
				t.Fatal(err)
			}
			filePath := filepath.Join(source, "file")
			if err := os.WriteFile(filePath, []byte("expected"), 0o600); err != nil {
				t.Fatal(err)
			}
			rootInfo, err := os.Lstat(source)
			if err != nil {
				t.Fatal(err)
			}
			fileInfo, err := os.Lstat(filePath)
			if err != nil {
				t.Fatal(err)
			}
			expected := ExpectedTree{
				Directories: []ExpectedDirectory{NewExpectedDirectory(".", rootInfo.Mode())},
				Files:       []ExpectedFile{NewExpectedFile("file", []byte("expected"), fileInfo.Mode())},
			}
			root := openTestAnchoredRoot(t, rootPath)
			restore := setAnchoredRootTreeAfterValidationHookForTest(func() error {
				if err := test.mutate(filePath); err == nil {
					return errors.New("validated tree allowed mutation")
				}
				return errors.New("validated tree mutation rejected")
			})
			t.Cleanup(restore)
			if err := root.RenameDirectoryNoReplaceExact("stage", "active", expected); err == nil {
				t.Fatal("tree rename accepted mutation after final validation")
			}
			if got, err := os.ReadFile(filePath); err != nil || string(got) != "expected" {
				t.Fatalf("validated tree changed: %q err=%v", got, err)
			}
			if _, err := os.Lstat(filepath.Join(rootPath, "active")); !os.IsNotExist(err) {
				t.Fatalf("target published after mutation attempt: %v", err)
			}
			assertNoExactMutationGuard(t, rootPath)
		})
	}
}

func TestAnchoredRootMutationGuardLeavesNoResidual(t *testing.T) {
	rootPath := t.TempDir()
	if err := os.WriteFile(filepath.Join(rootPath, "source"), []byte("expected"), 0o600); err != nil {
		t.Fatal(err)
	}
	root := openTestAnchoredRoot(t, rootPath)
	if err := root.RenameFileNoReplaceExact("source", "target", []byte("expected"), 0o600); err != nil {
		t.Fatal(err)
	}
	assertNoExactMutationGuard(t, rootPath)
	if err := root.RemoveExactFile("target", []byte("expected"), 0o600); err != nil {
		t.Fatal(err)
	}
	assertNoExactMutationGuard(t, rootPath)
}

func TestAnchoredRootDirectoryRenameRejectsTreeMutation(t *testing.T) {
	for _, test := range []struct {
		name   string
		mutate func(string) error
	}{
		{name: "extra", mutate: func(root string) error { return os.WriteFile(filepath.Join(root, "extra"), []byte("x"), 0o600) }},
		{name: "missing", mutate: func(root string) error { return os.Remove(filepath.Join(root, "file")) }},
		{name: "content", mutate: func(root string) error { return os.WriteFile(filepath.Join(root, "file"), []byte("changed"), 0o600) }},
		{name: "same-size-content", mutate: func(root string) error { return os.WriteFile(filepath.Join(root, "file"), []byte("EXPECTED"), 0o600) }},
	} {
		t.Run(test.name, func(t *testing.T) {
			rootPath := t.TempDir()
			source := filepath.Join(rootPath, "stage")
			if err := os.Mkdir(source, 0o700); err != nil {
				t.Fatal(err)
			}
			if err := os.WriteFile(filepath.Join(source, "file"), []byte("expected"), 0o600); err != nil {
				t.Fatal(err)
			}
			info, err := os.Lstat(source)
			if err != nil {
				t.Fatal(err)
			}
			fileInfo, err := os.Lstat(filepath.Join(source, "file"))
			if err != nil {
				t.Fatal(err)
			}
			expected := ExpectedTree{
				Directories: []ExpectedDirectory{NewExpectedDirectory(".", info.Mode())},
				Files:       []ExpectedFile{NewExpectedFile("file", []byte("expected"), fileInfo.Mode())},
			}
			root := openTestAnchoredRoot(t, rootPath)
			restore := setAnchoredRootBeforeRenameHookForTest(func() error {
				if err := test.mutate(source); err != nil {
					return err
				}
				return nil
			})
			t.Cleanup(restore)
			if err := root.RenameDirectoryNoReplaceExact("stage", "active", expected); err == nil {
				t.Fatal("tree mutation accepted")
			}
			if _, err := os.Lstat(source); err != nil {
				t.Fatalf("source moved after failed validation: %v", err)
			}
			if _, err := os.Lstat(filepath.Join(rootPath, "active")); !os.IsNotExist(err) {
				t.Fatalf("target published after failed validation: %v", err)
			}
			assertNoExactMutationGuard(t, rootPath)
		})
	}
}
