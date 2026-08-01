package instance

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/shuiyu486/re-context-kits/internal/rekit/defaults"
)

func TestInstanceAcceptsExistingFilesystemAliases(t *testing.T) {
	root := t.TempDir()
	repoRoot := filepath.Join(root, "repo")
	caseRoot := filepath.Join(root, "case")
	if err := os.Mkdir(repoRoot, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.Mkdir(caseRoot, 0o755); err != nil {
		t.Fatal(err)
	}
	repoAlias := filepath.Join(root, "repo-alias")
	caseAlias := filepath.Join(root, "case-alias")
	if err := os.Symlink(repoRoot, repoAlias); err != nil {
		t.Skipf("directory symlink unavailable: %v", err)
	}
	if err := os.Symlink(caseRoot, caseAlias); err != nil {
		t.Skipf("directory symlink unavailable: %v", err)
	}
	if err := os.MkdirAll(filepath.Join(caseRoot, ".rekit"), 0o755); err != nil {
		t.Fatal(err)
	}
	content := "templateRoot: " + repoAlias + "\n" +
		"templatePack: " + defaults.DefaultPack + "\n" +
		"projectName: alias-case\n" +
		"projectRoot: " + caseAlias + "\n"
	if err := os.WriteFile(filepath.Join(caseRoot, ".rekit", "instance.yml"), []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}
	inst, err := AssertAttached(caseRoot, repoRoot, defaults.DefaultPack)
	if err != nil {
		t.Fatal(err)
	}
	if inst.Moved() {
		t.Fatal("filesystem alias must not mark the case as moved")
	}
}

func TestReadScalarFileStripsSimpleQuotes(t *testing.T) {
	caseRoot := t.TempDir()
	if err := os.MkdirAll(filepath.Join(caseRoot, ".rekit"), 0o755); err != nil {
		t.Fatal(err)
	}
	content := "templateRoot: \"C:\\kit\"\ntemplatePack: 'vmp-re'\nprojectName: demo\n"
	if err := os.WriteFile(filepath.Join(caseRoot, ".rekit", "instance.yml"), []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}
	inst, err := Read(caseRoot)
	if err != nil {
		t.Fatal(err)
	}
	if inst.TemplateRoot != `C:\kit` {
		t.Fatalf("TemplateRoot = %q, want C:\\kit", inst.TemplateRoot)
	}
	if inst.TemplatePack != defaults.DefaultPack {
		t.Fatalf("TemplatePack = %q, want %s", inst.TemplatePack, defaults.DefaultPack)
	}
}
