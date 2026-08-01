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
