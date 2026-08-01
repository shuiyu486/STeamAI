package testenv

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestConfigureCanonicalTempRootDrivesTempDir(t *testing.T) {
	root, err := ConfigureCanonicalTempRoot()
	if err != nil {
		t.Fatal(err)
	}
	if got := filepath.Clean(os.TempDir()); got != filepath.Clean(root) {
		t.Fatalf("os.TempDir() = %s, want canonical root %s", got, root)
	}
	created := t.TempDir()
	rel, err := filepath.Rel(root, created)
	if err != nil {
		t.Fatal(err)
	}
	if rel == "." || rel == ".." || filepath.IsAbs(rel) || strings.HasPrefix(rel, ".."+string(filepath.Separator)) {
		t.Fatalf("t.TempDir() = %s, want descendant of %s", created, root)
	}
}
