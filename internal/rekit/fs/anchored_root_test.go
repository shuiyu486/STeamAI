package fs

import (
	"bytes"
	"errors"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
)

func openTestAnchoredRoot(t *testing.T, path string) *AnchoredRoot {
	t.Helper()
	root, err := OpenAnchoredRoot(path)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		if err := root.Close(); err != nil {
			t.Error(err)
		}
	})
	return root
}

func TestAnchoredRootStagesReadsRenamesAndRemoves(t *testing.T) {
	if runtime.GOOS != "windows" {
		t.Skip("exact handle-bound rename and removal are currently Windows-only")
	}
	path := t.TempDir()
	root := openTestAnchoredRoot(t, path)
	if err := root.MkdirAllNoFollow("maintenance/current-sync-v1/stage", 0o700); err != nil {
		t.Fatal(err)
	}
	data := []byte("staged\n")
	replay, err := root.WriteExclusiveFileWriteThrough("maintenance/current-sync-v1/stage/intent.json", data, 0o600, true)
	if err != nil || replay {
		t.Fatalf("initial write replay=%t err=%v", replay, err)
	}
	replay, err = root.WriteExclusiveFileWriteThrough("maintenance/current-sync-v1/stage/intent.json", data, 0o600, true)
	if err != nil || !replay {
		t.Fatalf("exact replay replay=%t err=%v", replay, err)
	}
	got, info, err := root.ReadStableFile("maintenance/current-sync-v1/stage/intent.json", 1024)
	if err != nil || string(got) != string(data) || info.Size() != int64(len(data)) {
		t.Fatalf("stable read data=%q info=%v err=%v", got, info, err)
	}
	if err := root.RenameFileNoReplaceExact("maintenance/current-sync-v1/stage/intent.json", "maintenance/current-sync-v1/intent.json", data, 0o600); err != nil {
		t.Fatal(err)
	}
	listed, err := root.ListRegularFilesNoFollow("maintenance/current-sync-v1/stage", 4)
	if err != nil || len(listed) != 0 {
		t.Fatalf("stage listed=%v err=%v", listed, err)
	}
	if err := root.RemoveExactFile("maintenance/current-sync-v1/intent.json", data, 0o600); err != nil {
		t.Fatal(err)
	}
	if err := root.RemoveExactFile("maintenance/current-sync-v1/intent.json", data, 0o600); err != nil {
		t.Fatalf("missing exact file replay: %v", err)
	}
	if err := root.RemoveEmptyDirectory("maintenance/current-sync-v1/stage"); err != nil {
		t.Fatal(err)
	}
}

func TestAnchoredRootRejectsNonCanonicalRelativePaths(t *testing.T) {
	root := openTestAnchoredRoot(t, t.TempDir())
	for _, rel := range []string{" ../escape", "../escape", "/absolute", "child/../../escape"} {
		if _, err := root.WriteExclusiveFileWriteThrough(rel, []byte("x"), 0o600, false); err == nil {
			t.Fatalf("anchored write accepted invalid path %q", rel)
		}
	}
}

func TestAnchoredRootWriteSupportsEmptyFileReplay(t *testing.T) {
	root := openTestAnchoredRoot(t, t.TempDir())
	if err := root.MkdirAllNoFollow("stage", 0o700); err != nil {
		t.Fatal(err)
	}
	replay, err := root.WriteExclusiveFileWriteThrough("stage/empty.bin", nil, 0o600, true)
	if err != nil || replay {
		t.Fatalf("empty initial write replay=%t err=%v", replay, err)
	}
	replay, err = root.WriteExclusiveFileWriteThrough("stage/empty.bin", nil, 0o600, true)
	if err != nil || !replay {
		t.Fatalf("empty replay replay=%t err=%v", replay, err)
	}
	data, info, err := root.ReadStableFile("stage/empty.bin", 0)
	if err != nil || len(data) != 0 || info.Size() != 0 {
		t.Fatalf("empty stable read data=%v info=%v err=%v", data, info, err)
	}
}

func TestAnchoredRootReplaceFileExactPublishesOnlyReviewedPredecessor(t *testing.T) {
	if !HandleBoundExactMutationSupported() {
		t.Skip("handle-bound exact replacement is platform-specific")
	}
	rootPath := t.TempDir()
	root, err := OpenAnchoredRoot(rootPath)
	if err != nil {
		t.Fatal(err)
	}
	defer root.Close()
	previous := []byte("previous\n")
	planned := []byte("planned\n")
	if _, err := root.WriteExclusiveFileWriteThrough("active.json", previous, 0o600, false); err != nil {
		t.Fatal(err)
	}
	if _, err := root.WriteExclusiveFileWriteThrough(".active.tmp", planned, 0o600, false); err != nil {
		t.Fatal(err)
	}
	if err := root.ReplaceFileExact(".active.tmp", "active.json", planned, 0o600, previous, 0o600); err != nil {
		t.Fatal(err)
	}
	data, _, err := root.ReadStableFile("active.json", 1<<10)
	if err != nil || !bytes.Equal(data, planned) {
		t.Fatalf("replacement data=%q err=%v", data, err)
	}
}

func TestAnchoredRootReplaceFileExactRecoversAfterPredecessorRename(t *testing.T) {
	if !HandleBoundExactMutationSupported() {
		t.Skip("handle-bound exact replacement is platform-specific")
	}
	rootPath := t.TempDir()
	root, err := OpenAnchoredRoot(rootPath)
	if err != nil {
		t.Fatal(err)
	}
	defer root.Close()
	previous := []byte("previous\n")
	planned := []byte("planned\n")
	if _, err := root.WriteExclusiveFileWriteThrough("active.json", previous, 0o600, false); err != nil {
		t.Fatal(err)
	}
	if _, err := root.WriteExclusiveFileWriteThrough(".active.tmp", planned, 0o600, false); err != nil {
		t.Fatal(err)
	}
	stopErr := errors.New("stop after predecessor rename")
	restore := SetAnchoredRootAfterPredecessorRenameHookForTest(func() error { return stopErr })
	err = root.ReplaceFileExact(".active.tmp", "active.json", planned, 0o600, previous, 0o600)
	restore()
	if !errors.Is(err, stopErr) {
		t.Fatalf("replacement cut err=%v", err)
	}
	if err := root.ReplaceFileExact(".active.tmp", "active.json", planned, 0o600, previous, 0o600); err != nil {
		t.Fatal(err)
	}
	data, _, err := root.ReadStableFile("active.json", 1<<10)
	if err != nil || !bytes.Equal(data, planned) {
		t.Fatalf("recovered replacement data=%q err=%v", data, err)
	}
}

func TestAnchoredRootReplaceFileExactRejectsPredecessorRebind(t *testing.T) {
	if !HandleBoundExactMutationSupported() {
		t.Skip("handle-bound exact replacement is platform-specific")
	}
	rootPath := t.TempDir()
	root, err := OpenAnchoredRoot(rootPath)
	if err != nil {
		t.Fatal(err)
	}
	defer root.Close()
	previous := []byte("previous\n")
	planned := []byte("planned\n")
	if _, err := root.WriteExclusiveFileWriteThrough("active.json", previous, 0o600, false); err != nil {
		t.Fatal(err)
	}
	if _, err := root.WriteExclusiveFileWriteThrough(".active.tmp", planned, 0o600, false); err != nil {
		t.Fatal(err)
	}
	restore := setAnchoredRootBeforeReplaceHookForTest(func() error {
		return os.WriteFile(filepath.Join(rootPath, "active.json"), []byte("changed\n"), 0o600)
	})
	err = root.ReplaceFileExact(".active.tmp", "active.json", planned, 0o600, previous, 0o600)
	restore()
	if err == nil {
		t.Fatal("replacement accepted a changed predecessor")
	}
	data, readErr := os.ReadFile(filepath.Join(rootPath, "active.json"))
	if readErr != nil || string(data) != "changed\n" {
		t.Fatalf("replacement overwrote changed predecessor: data=%q err=%v", data, readErr)
	}
}

func TestAnchoredRootRenameRejectsExistingTarget(t *testing.T) {
	if runtime.GOOS != "windows" {
		t.Skip("exact handle-bound rename is currently Windows-only")
	}
	path := t.TempDir()
	root := openTestAnchoredRoot(t, path)
	if err := root.MkdirAllNoFollow("stage", 0o700); err != nil {
		t.Fatal(err)
	}
	if _, err := root.WriteExclusiveFileWriteThrough("stage/source", []byte("source"), 0o600, false); err != nil {
		t.Fatal(err)
	}
	if _, err := root.WriteExclusiveFileWriteThrough("stage/target", []byte("target"), 0o600, false); err != nil {
		t.Fatal(err)
	}
	if err := root.RenameFileNoReplaceExact("stage/source", "stage/target", []byte("source"), 0o600); err == nil || !strings.Contains(err.Error(), "already exists") {
		t.Fatalf("rename target exists error=%v", err)
	}
	for name, want := range map[string]string{"source": "source", "target": "target"} {
		got, err := os.ReadFile(filepath.Join(path, "stage", name))
		if err != nil || string(got) != want {
			t.Fatalf("%s=%q err=%v", name, got, err)
		}
	}
}

func TestAnchoredRootRejectsSymlinkParentAndLeaf(t *testing.T) {
	path := t.TempDir()
	real := filepath.Join(path, "real")
	if err := os.Mkdir(real, 0o700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(real, "file"), []byte("real"), 0o600); err != nil {
		t.Fatal(err)
	}
	alias := filepath.Join(path, "alias")
	if err := os.Symlink(real, alias); err != nil {
		t.Skipf("symlink unavailable: %v", err)
	}
	root := openTestAnchoredRoot(t, path)
	if _, _, err := root.ReadStableFile("alias/file", 16); err == nil {
		t.Fatal("stable read accepted symlink parent")
	}
	if _, err := root.WriteExclusiveFileWriteThrough("alias/new", []byte("new"), 0o600, false); err == nil {
		t.Fatal("exclusive write accepted symlink parent")
	}
	leaf := filepath.Join(path, "leaf")
	if err := os.Symlink(filepath.Join(real, "file"), leaf); err != nil {
		t.Skipf("file symlink unavailable: %v", err)
	}
	if _, _, err := root.ReadStableFile("leaf", 16); err == nil {
		t.Fatal("stable read accepted symlink leaf")
	}
	if err := root.RemoveExactFile("leaf", []byte("real"), 0o600); err == nil {
		t.Fatal("exact remove accepted symlink leaf")
	}
	if _, err := root.ListNoFollow(".", 16); err == nil {
		t.Fatal("no-follow listing accepted symlink entries")
	}
}

func TestOpenAnchoredRootRejectsSymlinkAncestor(t *testing.T) {
	base := t.TempDir()
	real := filepath.Join(base, "real")
	if err := os.MkdirAll(filepath.Join(real, "root"), 0o700); err != nil {
		t.Fatal(err)
	}
	alias := filepath.Join(base, "alias")
	if err := os.Symlink(real, alias); err != nil {
		t.Skipf("directory symlink unavailable: %v", err)
	}
	if root, err := OpenAnchoredRoot(filepath.Join(alias, "root")); err == nil {
		_ = root.Close()
		t.Fatal("anchored root accepted a symlink ancestor")
	}
}

func TestAnchoredRootRejectsReparseParent(t *testing.T) {
	if runtime.GOOS != "windows" {
		t.Skip("Windows reparse coverage")
	}
	path := t.TempDir()
	real := filepath.Join(path, "real-reparse-target")
	if err := os.Mkdir(real, 0o700); err != nil {
		t.Fatal(err)
	}
	link := filepath.Join(path, "reparse-parent")
	if err := os.Symlink(real, link); err != nil {
		t.Skipf("Windows reparse creation unavailable: %v", err)
	}
	root := openTestAnchoredRoot(t, path)
	if _, err := root.WriteExclusiveFileWriteThrough("reparse-parent/file", []byte("x"), 0o600, false); err == nil || (!strings.Contains(err.Error(), "reparse") && !strings.Contains(err.Error(), "symlink")) {
		t.Fatalf("reparse parent error=%v", err)
	}
}

func TestAnchoredRootRejectsRootRebind(t *testing.T) {
	base := t.TempDir()
	path := filepath.Join(base, "root")
	if err := os.Mkdir(path, 0o700); err != nil {
		t.Fatal(err)
	}
	root := openTestAnchoredRoot(t, path)
	moved := filepath.Join(base, "root-original")
	if err := os.Rename(path, moved); err != nil {
		t.Skipf("open root cannot be rebound on this platform: %v", err)
	}
	if err := os.Mkdir(path, 0o700); err != nil {
		t.Fatal(err)
	}
	if err := root.Validate(); err == nil || !strings.Contains(err.Error(), "identity changed") {
		t.Fatalf("root rebind validation error=%v", err)
	}
	if _, err := root.WriteExclusiveFileWriteThrough("must-not-exist", []byte("x"), 0o600, false); err == nil {
		t.Fatal("write accepted rebound root")
	}
	if _, err := os.Lstat(filepath.Join(path, "must-not-exist")); !os.IsNotExist(err) {
		t.Fatalf("replacement root was mutated: %v", err)
	}
}

func TestAnchoredRootStableReadRejectsNameRebind(t *testing.T) {
	path := t.TempDir()
	if err := os.WriteFile(filepath.Join(path, "file"), []byte("original"), 0o600); err != nil {
		t.Fatal(err)
	}
	root := openTestAnchoredRoot(t, path)
	restore := setAnchoredRootReadAfterOpenHookForTest(func() error {
		if err := os.Rename(filepath.Join(path, "file"), filepath.Join(path, "opened")); err != nil {
			return err
		}
		return os.WriteFile(filepath.Join(path, "file"), []byte("replacement"), 0o600)
	})
	t.Cleanup(restore)
	if _, _, err := root.ReadStableFile("file", 64); err == nil || !strings.Contains(err.Error(), "changed") {
		t.Fatalf("stable read name rebind error=%v", err)
	}
}

func TestAnchoredRootRenameExactRejectsSourceRebind(t *testing.T) {
	if runtime.GOOS != "windows" {
		t.Skip("exact handle-bound rename is currently Windows-only")
	}
	path := t.TempDir()
	if err := os.Mkdir(filepath.Join(path, "stage"), 0o700); err != nil {
		t.Fatal(err)
	}
	source := filepath.Join(path, "stage", "source")
	if err := os.WriteFile(source, []byte("expected"), 0o600); err != nil {
		t.Fatal(err)
	}
	root := openTestAnchoredRoot(t, path)
	restore := setAnchoredRootBeforeRenameHookForTest(func() error {
		if err := os.Rename(source, filepath.Join(path, "stage", "original")); err != nil {
			return err
		}
		return os.WriteFile(source, []byte("replacement"), 0o600)
	})
	t.Cleanup(restore)
	if err := root.RenameFileNoReplaceExact("stage/source", "published", []byte("expected"), 0o600); err == nil || !strings.Contains(err.Error(), "identity changed") {
		t.Fatalf("exact rename source rebind error=%v", err)
	}
	got, err := os.ReadFile(source)
	if err != nil || string(got) != "replacement" {
		t.Fatalf("replacement source was moved or changed: %q err=%v", got, err)
	}
	if _, err := os.Lstat(filepath.Join(path, "published")); !os.IsNotExist(err) {
		t.Fatalf("rename published a replacement: %v", err)
	}
}

func TestAnchoredRootRenamesExactDirectoryIdentity(t *testing.T) {
	if runtime.GOOS != "windows" {
		t.Skip("exact handle-bound rename is currently Windows-only")
	}
	path := t.TempDir()
	if err := os.MkdirAll(filepath.Join(path, "transaction", "stage"), 0o700); err != nil {
		t.Fatal(err)
	}
	root := openTestAnchoredRoot(t, path)
	identity, err := root.Lstat("transaction/stage")
	if err != nil {
		t.Fatal(err)
	}
	expected := ExpectedTree{Directories: []ExpectedDirectory{NewExpectedDirectory(".", identity.Mode())}}
	if err := root.RenameDirectoryNoReplaceExact("transaction/stage", "transaction/active", expected); err != nil {
		t.Fatal(err)
	}
	if current, err := root.Lstat("transaction/active"); err != nil || !os.SameFile(identity, current) {
		t.Fatalf("renamed directory identity=%v err=%v", current, err)
	}
}

func TestAnchoredRootRenamesExactNonEmptyTree(t *testing.T) {
	if runtime.GOOS != "windows" {
		t.Skip("exact handle-bound rename is currently Windows-only")
	}
	path := t.TempDir()
	source := filepath.Join(path, "transaction", "stage")
	if err := os.MkdirAll(filepath.Join(source, "nested"), 0o700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(source, "root.txt"), []byte("root"), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(source, "nested", "child.txt"), []byte("child"), 0o600); err != nil {
		t.Fatal(err)
	}
	rootInfo, err := os.Lstat(source)
	if err != nil {
		t.Fatal(err)
	}
	nestedInfo, err := os.Lstat(filepath.Join(source, "nested"))
	if err != nil {
		t.Fatal(err)
	}
	rootFileInfo, err := os.Lstat(filepath.Join(source, "root.txt"))
	if err != nil {
		t.Fatal(err)
	}
	childInfo, err := os.Lstat(filepath.Join(source, "nested", "child.txt"))
	if err != nil {
		t.Fatal(err)
	}
	expected := ExpectedTree{
		Directories: []ExpectedDirectory{
			NewExpectedDirectory(".", rootInfo.Mode()),
			NewExpectedDirectory("nested", nestedInfo.Mode()),
		},
		Files: []ExpectedFile{
			NewExpectedFile("root.txt", []byte("root"), rootFileInfo.Mode()),
			NewExpectedFile("nested/child.txt", []byte("child"), childInfo.Mode()),
		},
	}
	root := openTestAnchoredRoot(t, path)
	if err := root.RenameDirectoryNoReplaceExact("transaction/stage", "transaction/active", expected); err != nil {
		t.Fatal(err)
	}
	for rel, want := range map[string]string{"root.txt": "root", "nested/child.txt": "child"} {
		got, err := os.ReadFile(filepath.Join(path, "transaction", "active", filepath.FromSlash(rel)))
		if err != nil || string(got) != want {
			t.Fatalf("%s=%q err=%v", rel, got, err)
		}
	}
	if _, err := os.Lstat(source); !os.IsNotExist(err) {
		t.Fatalf("source remains after exact tree rename: %v", err)
	}
}

func TestAnchoredRootRemoveExactFileRejectsDifferentBytesAndNameRebind(t *testing.T) {
	if !exactRemoveSupportedForTest() {
		t.Skip("exact handle-bound removal is unsupported on this platform")
	}
	path := t.TempDir()
	if err := os.WriteFile(filepath.Join(path, "intent.json"), []byte("expected"), 0o600); err != nil {
		t.Fatal(err)
	}
	root := openTestAnchoredRoot(t, path)
	if err := root.RemoveExactFile("missing.json", []byte("expected"), 0o600); err != nil {
		t.Fatalf("missing exact remove replay: %v", err)
	}
	if err := root.RemoveExactFile("intent.json", []byte("different"), 0o600); err == nil {
		t.Fatal("exact remove accepted different bytes")
	}
	restore := setAnchoredRootBeforeRemoveHookForTest(func() error {
		if err := os.Rename(filepath.Join(path, "intent.json"), filepath.Join(path, "original.json")); err != nil {
			return err
		}
		return os.WriteFile(filepath.Join(path, "intent.json"), []byte("replacement"), 0o600)
	})
	t.Cleanup(restore)
	if err := root.RemoveExactFile("intent.json", []byte("expected"), 0o600); err == nil || !strings.Contains(err.Error(), "identity changed") {
		t.Fatalf("exact remove name rebind error=%v", err)
	}
	got, err := os.ReadFile(filepath.Join(path, "intent.json"))
	if err != nil || string(got) != "replacement" {
		t.Fatalf("replacement file was removed or changed: %q err=%v", got, err)
	}
}

func TestAnchoredRootExactMutationsFailClosedWhenUnsupported(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("Windows provides handle-bound exact mutation")
	}
	path := t.TempDir()
	if err := os.WriteFile(filepath.Join(path, "source"), []byte("source"), 0o600); err != nil {
		t.Fatal(err)
	}
	root := openTestAnchoredRoot(t, path)
	if err := root.RenameFileNoReplaceExact("source", "target", []byte("source"), 0o600); !errors.Is(err, errAnchoredExactMutationUnsupported) {
		t.Fatalf("unsupported rename error=%v", err)
	}
	if !exactRemoveSupportedForTest() {
		if err := root.RemoveExactFile("source", []byte("source"), 0o600); !errors.Is(err, errAnchoredExactMutationUnsupported) {
			t.Fatalf("unsupported remove error=%v", err)
		}
	}
	if _, err := root.WriteExclusiveFileWriteThrough("new", []byte("new"), 0o600, false); !errors.Is(err, errAnchoredExactMutationUnsupported) {
		t.Fatalf("unsupported write error=%v", err)
	}
	if _, err := os.Lstat(filepath.Join(path, "new")); !os.IsNotExist(err) {
		t.Fatalf("unsupported write mutated filesystem: %v", err)
	}
}

func exactRemoveSupportedForTest() bool {
	return runtime.GOOS == "windows"
}

func TestExpectedTreePathCollisionsFollowPlatformSemantics(t *testing.T) {
	tree := ExpectedTree{
		Directories: []ExpectedDirectory{NewExpectedDirectory(".", os.ModeDir|0o700)},
		Files: []ExpectedFile{
			NewExpectedFile("Name", []byte("one"), 0o600),
			NewExpectedFile("name", []byte("two"), 0o600),
		},
	}
	_, err := validateExpectedTree(tree)
	if runtime.GOOS == "windows" && err == nil {
		t.Fatal("Windows expected tree accepted case-colliding names")
	}
	if runtime.GOOS != "windows" && err != nil {
		t.Fatalf("case-sensitive platform rejected distinct names: %v", err)
	}
}

func TestAnchoredRootRemoveExactFileRejectsNonRegular(t *testing.T) {
	path := t.TempDir()
	if err := os.Mkdir(filepath.Join(path, "directory"), 0o700); err != nil {
		t.Fatal(err)
	}
	root := openTestAnchoredRoot(t, path)
	if err := root.RemoveExactFile("directory", nil, 0o600); err == nil {
		t.Fatal("exact file remove accepted directory")
	}
	if err := os.WriteFile(filepath.Join(path, "not-empty"), []byte("child"), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := root.RemoveEmptyDirectory("missing-directory"); err != nil {
		t.Fatalf("missing empty directory replay: %v", err)
	}
	if err := os.Mkdir(filepath.Join(path, "non-empty-directory"), 0o700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(path, "non-empty-directory", "child"), []byte("child"), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := root.RemoveEmptyDirectory("non-empty-directory"); err == nil {
		t.Fatal("empty directory remove accepted non-empty directory")
	}
}
