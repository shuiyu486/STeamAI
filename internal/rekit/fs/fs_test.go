package fs

import (
	"os"
	"path/filepath"
	"testing"
)

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
