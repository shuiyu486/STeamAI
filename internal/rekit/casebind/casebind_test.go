package casebind

import (
	"bytes"
	"os"
	"path/filepath"
	"testing"
)

func TestWriteCaseShimPublishesCanonicalWindowsText(t *testing.T) {
	repoRoot := t.TempDir()
	caseRoot := t.TempDir()
	sourcePath := filepath.Join(repoRoot, "rekit", "templates", "case-shim", "SKILL.md")
	if err := os.MkdirAll(filepath.Dir(sourcePath), 0o755); err != nil {
		t.Fatal(err)
	}
	source := []byte("thin\nshim\n")
	if err := os.WriteFile(sourcePath, source, 0o644); err != nil {
		t.Fatal(err)
	}

	shimPath, err := WriteCanonicalCaseShim(caseRoot, repoRoot)
	if err != nil {
		t.Fatal(err)
	}
	got, err := os.ReadFile(shimPath)
	if err != nil {
		t.Fatal(err)
	}
	if want := []byte("thin\r\nshim\r\n"); !bytes.Equal(got, want) {
		t.Fatalf("published shim = %q, want %q", got, want)
	}
	unchanged, err := os.ReadFile(sourcePath)
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(unchanged, source) {
		t.Fatalf("source shim changed: %q", unchanged)
	}
}
