package fs

import (
	"errors"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"
)

func TestValidateNoReparseComponentsAcceptsRegularAndMissingSuffix(t *testing.T) {
	root := t.TempDir()
	regular := filepath.Join(root, "regular")
	if err := os.Mkdir(regular, 0o700); err != nil {
		t.Fatal(err)
	}
	regularFile := filepath.Join(regular, "file")
	if err := os.WriteFile(regularFile, []byte("regular\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	for _, path := range []string{
		regular,
		filepath.Join(regular, "missing", "suffix"),
		filepath.Join(regularFile, "missing"),
	} {
		if err := ValidateNoReparseComponents(path); err != nil {
			t.Fatalf("ValidateNoReparseComponents(%q): %v", path, err)
		}
	}
}

func TestWriteNewExclusiveRegularFileAnchoredRejectsExistingFile(t *testing.T) {
	root := t.TempDir()
	if err := WriteNewExclusiveRegularFileAnchored(root, "nested/receipt.json", "receipt", []byte("first\n")); err != nil {
		t.Fatal(err)
	}
	if err := WriteNewExclusiveRegularFileAnchored(root, "nested/receipt.json", "receipt", []byte("second\n")); err == nil {
		t.Fatal("new-only anchored publication replaced an existing file")
	}
	data, err := os.ReadFile(filepath.Join(root, "nested", "receipt.json"))
	if err != nil || string(data) != "first\n" {
		t.Fatalf("existing anchored file changed: %q err=%v", data, err)
	}
}

func TestWriteNewExclusiveRegularFileAnchoredRejectsAncestorReplacement(t *testing.T) {
	base := t.TempDir()
	parent := filepath.Join(base, "parent")
	if err := os.Mkdir(parent, 0o700); err != nil {
		t.Fatal(err)
	}
	moved := filepath.Join(base, "parent-original")
	path := filepath.Join(parent, "receipt.json")
	restore := SetWriteExclusiveRegularFileAfterPublishHookForTest(func() error {
		if err := os.Rename(parent, moved); err != nil {
			return err
		}
		return os.Mkdir(parent, 0o700)
	})
	t.Cleanup(restore)
	if err := WriteNewExclusiveRegularFileAnchored(base, "parent/receipt.json", "receipt", []byte("receipt\n")); err == nil {
		t.Fatal("anchored publication accepted ancestor replacement")
	}
	if _, err := os.Lstat(path); !os.IsNotExist(err) {
		t.Fatalf("replacement namespace received receipt: %v", err)
	}
	if _, err := os.Lstat(filepath.Join(moved, "receipt.json")); !os.IsNotExist(err) {
		t.Fatalf("failed publication left receipt in original namespace: %v", err)
	}
}

func TestSameExistingPathResolvesDirectoryAliasWithoutChangingLexicalIdentity(t *testing.T) {
	root := t.TempDir()
	target := filepath.Join(root, "target")
	if err := os.Mkdir(target, 0o755); err != nil {
		t.Fatal(err)
	}
	alias := filepath.Join(root, "alias")
	if err := os.Symlink(target, alias); err != nil {
		t.Skipf("directory symlink unavailable: %v", err)
	}
	same, err := SameExistingPath(target, alias)
	if err != nil {
		t.Fatal(err)
	}
	if !same {
		t.Fatalf("existing directory aliases should have the same identity: target=%s alias=%s", target, alias)
	}
	if SamePath(target, alias) {
		t.Fatalf("lexical path identity must not accept an alias: target=%s alias=%s", target, alias)
	}
}

func TestSameExistingPathRejectsDifferentDirectories(t *testing.T) {
	root := t.TempDir()
	left := filepath.Join(root, "left")
	right := filepath.Join(root, "right")
	if err := os.Mkdir(left, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.Mkdir(right, 0o755); err != nil {
		t.Fatal(err)
	}
	same, err := SameExistingPath(left, right)
	if err != nil {
		t.Fatal(err)
	}
	if same {
		t.Fatalf("different existing directories must remain distinct: left=%s right=%s", left, right)
	}
}

func TestSamePathKeepsMissingPathsDistinct(t *testing.T) {
	root := t.TempDir()
	left := filepath.Join(root, "left", "missing")
	right := filepath.Join(root, "right", "missing")
	if SamePath(left, right) {
		t.Fatalf("missing paths must not collapse: left=%s right=%s", left, right)
	}
}

func TestListRegularFilesAnchoredRejectsInvalidNamespaceEntries(t *testing.T) {
	caseRoot := t.TempDir()
	inbox := filepath.Join(caseRoot, ".rekit", "external-session-observations", "inbox")
	if files, err := ListRegularFilesAnchored(caseRoot, ".rekit/external-session-observations/inbox", "observation inbox", 16); err != nil || len(files) != 0 {
		t.Fatalf("missing inbox files=%v err=%v", files, err)
	}
	if err := os.MkdirAll(inbox, 0o700); err != nil {
		t.Fatal(err)
	}
	for _, name := range []string{"b.json", "a.json"} {
		if err := os.WriteFile(filepath.Join(inbox, name), []byte("{}\n"), 0o600); err != nil {
			t.Fatal(err)
		}
	}
	files, err := ListRegularFilesAnchored(caseRoot, ".rekit/external-session-observations/inbox", "observation inbox", 16)
	if err != nil || len(files) != 2 || filepath.Base(files[0]) != "a.json" || filepath.Base(files[1]) != "b.json" {
		t.Fatalf("regular inbox files=%v err=%v", files, err)
	}
	if err := os.Mkdir(filepath.Join(inbox, "nested"), 0o700); err != nil {
		t.Fatal(err)
	}
	if _, err := ListRegularFilesAnchored(caseRoot, ".rekit/external-session-observations/inbox", "observation inbox", 16); err == nil {
		t.Fatal("anchored listing accepted a nested directory")
	}
}

func TestWriteAtomicNoReplaceRegularFileAnchoredPublishesOnlyCompleteFinalBytes(t *testing.T) {
	caseRoot := t.TempDir()
	rel := ".steamai/lanes/main/adapter-executions/gate-a/.binary-inventory-output-commit.json"
	data := []byte("{\"committed\":true}\n")
	finalPath := filepath.Join(caseRoot, filepath.FromSlash(rel))
	var tempPath string
	restore := SetWriteAtomicNoReplaceAfterTempSyncHookForTest(func(path string) error {
		tempPath = path
		if _, err := os.Lstat(finalPath); !os.IsNotExist(err) {
			t.Fatalf("final marker appeared before atomic install: %v", err)
		}
		got, err := os.ReadFile(path)
		if err != nil || string(got) != string(data) {
			t.Fatalf("owned temp bytes=%q err=%v", got, err)
		}
		return errors.New("synthetic interruption after temp sync")
	})
	defer restore()
	if _, err := WriteAtomicNoReplaceRegularFileAnchored(caseRoot, rel, "output commit", data); err == nil || !strings.Contains(err.Error(), "synthetic interruption") {
		t.Fatalf("interrupted atomic publication error=%v", err)
	}
	if _, err := os.Lstat(finalPath); !os.IsNotExist(err) {
		t.Fatalf("interrupted atomic publication exposed final marker: %v", err)
	}
	if tempPath == "" {
		t.Fatal("atomic publication did not reach the synced temp hook")
	}
	if _, err := os.Lstat(tempPath); !os.IsNotExist(err) {
		t.Fatalf("failed atomic publication left owned temp: %v", err)
	}
	restore()

	replayed, err := WriteAtomicNoReplaceRegularFileAnchored(caseRoot, rel, "output commit", data)
	if err != nil || replayed {
		t.Fatalf("first atomic publication replayed=%t err=%v", replayed, err)
	}
	replayed, err = WriteAtomicNoReplaceRegularFileAnchored(caseRoot, rel, "output commit", data)
	if err != nil || !replayed {
		t.Fatalf("exact atomic replay replayed=%t err=%v", replayed, err)
	}
	if _, err := WriteAtomicNoReplaceRegularFileAnchored(caseRoot, rel, "output commit", []byte("different\n")); err == nil {
		t.Fatal("atomic publication accepted different existing bytes")
	}
}

func TestWriteAtomicNoReplaceRegularFileAnchoredModePreservesMode(t *testing.T) {
	caseRoot := t.TempDir()
	rel := ".steamai/handovers/main-stamped.md"
	data := []byte("# handoff\n")
	replayed, err := WriteAtomicNoReplaceRegularFileAnchoredMode(
		caseRoot,
		rel,
		"stamped handoff",
		data,
		0o644,
	)
	if err != nil || replayed {
		t.Fatalf("mode-aware publication replayed=%t err=%v", replayed, err)
	}
	info, err := os.Stat(filepath.Join(caseRoot, filepath.FromSlash(rel)))
	if err != nil || !anchoredModeMatches(0o644, info.Mode()) {
		t.Fatalf("mode-aware publication mode=%v err=%v", info, err)
	}
	replayed, err = WriteAtomicNoReplaceRegularFileAnchoredMode(
		caseRoot,
		rel,
		"stamped handoff",
		data,
		0o644,
	)
	if err != nil || !replayed {
		t.Fatalf("mode-aware replay replayed=%t err=%v", replayed, err)
	}
}

func TestWriteAtomicNoReplaceRegularFileAnchoredIgnoresStaleOwnedTemp(t *testing.T) {
	caseRoot := t.TempDir()
	rel := ".steamai/lanes/main/adapter-executions/gate-a/.binary-inventory-output-commit.json"
	path := filepath.Join(caseRoot, filepath.FromSlash(rel))
	if err := os.MkdirAll(filepath.Dir(path), 0o700); err != nil {
		t.Fatal(err)
	}
	stale := filepath.Join(filepath.Dir(path), "."+filepath.Base(path)+".owned-00000000000000000000000000000000.tmp")
	if err := os.WriteFile(stale, []byte("partial temp"), 0o600); err != nil {
		t.Fatal(err)
	}
	data := []byte("{\"committed\":true}\n")
	replayed, err := WriteAtomicNoReplaceRegularFileAnchored(caseRoot, rel, "output commit", data)
	if err != nil || replayed {
		t.Fatalf("publication around stale temp replayed=%t err=%v", replayed, err)
	}
	got, err := os.ReadFile(path)
	if err != nil || string(got) != string(data) {
		t.Fatalf("published final bytes=%q err=%v", got, err)
	}
	if got, err := os.ReadFile(stale); err != nil || string(got) != "partial temp" {
		t.Fatalf("unowned stale temp was changed: %q err=%v", got, err)
	}
}

func TestWriteAtomicNoReplaceRegularFileAnchoredRejectsPartialFinalMarker(t *testing.T) {
	caseRoot := t.TempDir()
	rel := ".steamai/lanes/main/adapter-executions/gate-a/.binary-inventory-success-seal.json"
	path := filepath.Join(caseRoot, filepath.FromSlash(rel))
	if err := os.MkdirAll(filepath.Dir(path), 0o700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte("{\"schema"), 0o600); err != nil {
		t.Fatal(err)
	}
	if _, err := WriteAtomicNoReplaceRegularFileAnchored(caseRoot, rel, "success seal", []byte("{\"schemaVersion\":1}\n")); err == nil {
		t.Fatal("atomic publication accepted or replaced a partial final marker")
	}
	got, err := os.ReadFile(path)
	if err != nil || string(got) != "{\"schema" {
		t.Fatalf("partial obstruction changed: %q err=%v", got, err)
	}
}

func TestWriteExclusiveRegularFileAnchoredWriteThroughPublishesOrReplaysExactBytes(t *testing.T) {
	caseRoot := t.TempDir()
	rel := ".rekit/lanes/main/adapter-executions/gate-a/child-launch.json"
	data := []byte("{\"launched\":true}\n")
	replayed, err := WriteExclusiveRegularFileAnchoredWriteThrough(caseRoot, rel, "child launch proof", data)
	if err != nil || replayed {
		t.Fatalf("first write-through publication replayed=%t err=%v", replayed, err)
	}
	if got, err := os.ReadFile(filepath.Join(caseRoot, filepath.FromSlash(rel))); err != nil || string(got) != string(data) {
		t.Fatalf("write-through publication bytes=%q err=%v", got, err)
	}
	replayed, err = WriteExclusiveRegularFileAnchoredWriteThrough(caseRoot, rel, "child launch proof", data)
	if err != nil || !replayed {
		t.Fatalf("write-through replay replayed=%t err=%v", replayed, err)
	}
	if _, err := WriteExclusiveRegularFileAnchoredWriteThrough(caseRoot, rel, "child launch proof", []byte("different\n")); err == nil {
		t.Fatal("write-through publication accepted different existing bytes")
	}
}

func TestWriteExclusiveRegularFileAnchoredPublishesOrReplaysExactBytes(t *testing.T) {
	caseRoot := t.TempDir()
	rel := ".rekit/external-session-relays/job-a/publication.json"
	data := []byte("{\"ready\":true}\n")
	replayed, err := WriteExclusiveRegularFileAnchored(caseRoot, rel, "relay receipt", data)
	if err != nil || replayed {
		t.Fatalf("first publication replayed=%t err=%v", replayed, err)
	}
	path := filepath.Join(caseRoot, filepath.FromSlash(rel))
	if got, err := os.ReadFile(path); err != nil || string(got) != string(data) {
		t.Fatalf("published bytes=%q err=%v", got, err)
	}
	replayed, err = WriteExclusiveRegularFileAnchored(caseRoot, rel, "relay receipt", data)
	if err != nil || !replayed {
		t.Fatalf("exact replay replayed=%t err=%v", replayed, err)
	}
	if _, err := WriteExclusiveRegularFileAnchored(caseRoot, rel, "relay receipt", []byte("different\n")); err == nil {
		t.Fatal("exclusive publication accepted different existing bytes")
	}
}

func TestWriteExclusiveRegularFileAnchoredConcurrentExactReplay(t *testing.T) {
	caseRoot := t.TempDir()
	rel := ".rekit/external-session-relays/job-a/publication.json"
	data := []byte("{\"ready\":true}\n")
	const writers = 8
	var wg sync.WaitGroup
	type outcome struct {
		replayed bool
		err      error
	}
	results := make(chan outcome, writers)
	for range writers {
		wg.Go(func() {
			replayed, err := WriteExclusiveRegularFileAnchored(caseRoot, rel, "relay receipt", data)
			results <- outcome{replayed: replayed, err: err}
		})
	}
	wg.Wait()
	close(results)
	created := 0
	replayed := 0
	for result := range results {
		if result.err != nil {
			t.Fatal(result.err)
		}
		if result.replayed {
			replayed++
		} else {
			created++
		}
	}
	if created != 1 || replayed != writers-1 {
		t.Fatalf("created=%d replayed=%d", created, replayed)
	}
}

func TestWalkRegularFilesAnchoredRejectsEmptyDirectoriesAndLimit(t *testing.T) {
	caseRoot := t.TempDir()
	root := filepath.Join(caseRoot, ".rekit", "external-session-jobs", "job-a", "outputs")
	if err := os.MkdirAll(filepath.Join(root, "empty"), 0o700); err != nil {
		t.Fatal(err)
	}
	if _, err := WalkRegularFilesAnchored(caseRoot, ".rekit/external-session-jobs/job-a/outputs", "outputs", 4); err == nil {
		t.Fatal("walk accepted empty nested directory")
	}
	if err := os.Remove(filepath.Join(root, "empty")); err != nil {
		t.Fatal(err)
	}
	for _, name := range []string{"a.txt", "b.txt"} {
		if err := os.WriteFile(filepath.Join(root, name), []byte(name), 0o600); err != nil {
			t.Fatal(err)
		}
	}
	if _, err := WalkRegularFilesAnchored(caseRoot, ".rekit/external-session-jobs/job-a/outputs", "outputs", 1); err == nil {
		t.Fatal("walk accepted more files than limit")
	}
}

func TestReadStableRegularFileAllowEmptyAnchored(t *testing.T) {
	caseRoot := t.TempDir()
	path := filepath.Join(caseRoot, "empty.jsonl")
	if err := os.WriteFile(path, nil, 0o600); err != nil {
		t.Fatal(err)
	}
	data, err := ReadStableRegularFileAllowEmptyAnchored(caseRoot, path, "empty ledger", 1024)
	if err != nil {
		t.Fatal(err)
	}
	if len(data) != 0 {
		t.Fatalf("empty ledger bytes = %q", data)
	}
	if _, err := ReadStableRegularFileAnchored(caseRoot, path, "non-empty ledger", 1024); err == nil || !strings.Contains(err.Error(), "non-empty") {
		t.Fatalf("non-empty reader accepted empty ledger: %v", err)
	}
}

func TestReadStableRegularFileAnchoredRejectsSymlinkComponents(t *testing.T) {
	caseRoot := t.TempDir()
	real := filepath.Join(caseRoot, "real")
	if err := os.Mkdir(real, 0o755); err != nil {
		t.Fatal(err)
	}
	file := filepath.Join(real, "evidence.txt")
	if err := os.WriteFile(file, []byte("evidence\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	if data, err := ReadStableRegularFileAnchored(caseRoot, file, "evidence", 1024); err != nil || string(data) != "evidence\n" {
		t.Fatalf("regular anchored read data=%q err=%v", data, err)
	}
	alias := filepath.Join(caseRoot, "alias")
	if err := os.Symlink(real, alias); err != nil {
		t.Skipf("directory symlink unavailable: %v", err)
	}
	if _, err := ReadStableRegularFileAnchored(caseRoot, filepath.Join(alias, "evidence.txt"), "evidence", 1024); err == nil {
		t.Fatal("anchored reader accepted a symlink directory component")
	}
	leaf := filepath.Join(caseRoot, "evidence-link.txt")
	if err := os.Symlink(file, leaf); err != nil {
		t.Skipf("file symlink unavailable: %v", err)
	}
	if _, err := ReadStableRegularFileAnchored(caseRoot, leaf, "evidence", 1024); err == nil {
		t.Fatal("anchored reader accepted a symlink leaf")
	}
}
