package statemigration

import (
	"os"
	"testing"
)

func TestOpenRootIdentityIsStable(t *testing.T) {
	rootPath := t.TempDir()
	firstRoot, first, err := OpenRootIdentity(rootPath)
	if err != nil {
		t.Fatal(err)
	}
	defer firstRoot.Close()
	secondRoot, second, err := OpenRootIdentity(rootPath)
	if err != nil {
		t.Fatal(err)
	}
	defer secondRoot.Close()
	if first != second {
		t.Fatalf("identity changed for the same directory: first=%+v second=%+v", first, second)
	}
}

func TestIdentityForRootRejectsNil(t *testing.T) {
	if _, err := IdentityForRoot(nil); err == nil {
		t.Fatal("expected nil root to be rejected")
	}
}

func TestOpenRootIdentityRejectsEmptyPath(t *testing.T) {
	if _, _, err := OpenRootIdentity(" \t"); err == nil {
		t.Fatal("expected empty path to be rejected")
	}
}

func TestIdentityValidateRejectsIncompleteValues(t *testing.T) {
	values := []Identity{
		{},
		{Scheme: "unknown"},
		{Scheme: "unix-dev-inode-v1"},
		{Scheme: "unix-dev-inode-v1", Inode: 1},
		{Scheme: "unix-dev-inode-v1", Device: 1, Inode: 1, FileIndex: 1},
		{Scheme: "windows-volume-file-index-v1"},
		{Scheme: "windows-volume-file-index-v1", VolumeSerial: 1, FileIndex: 1, Inode: 1},
	}
	for _, value := range values {
		if err := value.Validate(); err == nil {
			t.Fatalf("expected identity to be rejected: %+v", value)
		}
	}
}

func TestIdentityForFileIsStableAcrossHardlinks(t *testing.T) {
	root := t.TempDir()
	path := root + string(os.PathSeparator) + "source.bin"
	alias := root + string(os.PathSeparator) + "alias.bin"
	if err := os.WriteFile(path, []byte("physical identity\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := os.Link(path, alias); err != nil {
		t.Fatal(err)
	}
	firstFile, err := os.Open(path)
	if err != nil {
		t.Fatal(err)
	}
	defer firstFile.Close()
	secondFile, err := os.Open(alias)
	if err != nil {
		t.Fatal(err)
	}
	defer secondFile.Close()
	first, err := IdentityForFile(firstFile)
	if err != nil {
		t.Fatal(err)
	}
	second, err := IdentityForFile(secondFile)
	if err != nil {
		t.Fatal(err)
	}
	if first != second {
		t.Fatalf("hardlinks produced different file identities: first=%+v second=%+v", first, second)
	}
}

func TestIdentityForFileRejectsNilAndDirectory(t *testing.T) {
	if _, err := IdentityForFile(nil); err == nil {
		t.Fatal("expected nil file to be rejected")
	}
	file, err := os.Open(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	defer file.Close()
	if _, err := IdentityForFile(file); err == nil {
		t.Fatal("expected directory file identity to be rejected")
	}
}

func TestIdentityForRootMatchesOpenDirectory(t *testing.T) {
	root, err := os.OpenRoot(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	defer root.Close()
	identity, err := IdentityForRoot(root)
	if err != nil {
		t.Fatal(err)
	}
	if err := identity.Validate(); err != nil {
		t.Fatalf("captured identity is invalid: %v", err)
	}
}
