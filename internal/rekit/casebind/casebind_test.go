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
	sourcePath := filepath.Join(repoRoot, "rekit", "templates", "steamai-project", "SKILL.md")
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
	if want := filepath.Join(caseRoot, ".claude", "skills", "steamai", "SKILL.md"); shimPath != want {
		t.Fatalf("shim path = %s, want %s", shimPath, want)
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

func TestWriteCaseShimKeepsLegacyRekitEntrypoint(t *testing.T) {
	repoRoot := t.TempDir()
	caseRoot := t.TempDir()
	if err := os.Mkdir(filepath.Join(caseRoot, ".rekit"), 0o755); err != nil {
		t.Fatal(err)
	}
	sourcePath := filepath.Join(repoRoot, "rekit", "templates", "case-shim", "SKILL.md")
	if err := os.MkdirAll(filepath.Dir(sourcePath), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(sourcePath, []byte("legacy shim\n"), 0o644); err != nil {
		t.Fatal(err)
	}

	shimPath, err := WriteCaseShim(caseRoot, repoRoot)
	if err != nil {
		t.Fatal(err)
	}
	if want := filepath.Join(caseRoot, ".claude", "skills", "rekit", "SKILL.md"); shimPath != want {
		t.Fatalf("shim path = %s, want %s", shimPath, want)
	}
	if _, err := os.Stat(filepath.Join(caseRoot, ".claude", "skills", "steamai", "SKILL.md")); !os.IsNotExist(err) {
		t.Fatalf("legacy project unexpectedly received STeamAI skill: %v", err)
	}
}
