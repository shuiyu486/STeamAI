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

func TestReadUsesSTeamAIStateRootForNewProjects(t *testing.T) {
	caseRoot := t.TempDir()
	if err := os.MkdirAll(filepath.Join(caseRoot, ".steamai"), 0o755); err != nil {
		t.Fatal(err)
	}
	content := "templateRoot: " + caseRoot + "\n" +
		"templatePack: " + defaults.DefaultPack + "\n" +
		"projectName: steamai-case\n" +
		"projectRoot: " + caseRoot + "\n"
	instancePath := filepath.Join(caseRoot, ".steamai", "instance.yml")
	if err := os.WriteFile(instancePath, []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}

	if !LooksLikeCase(caseRoot) {
		t.Fatal(".steamai/instance.yml must identify an STeamAI project")
	}
	inst, err := Read(caseRoot)
	if err != nil {
		t.Fatal(err)
	}
	if inst.Source != "steamai" || inst.InstancePath != instancePath || inst.StateDir != ".steamai" {
		t.Fatalf("instance = %#v, want STeamAI state root", inst)
	}
}

func TestReadKeepsLegacyRekitStateRoot(t *testing.T) {
	caseRoot := t.TempDir()
	if err := os.MkdirAll(filepath.Join(caseRoot, ".rekit"), 0o755); err != nil {
		t.Fatal(err)
	}
	content := "templateRoot: " + caseRoot + "\n" +
		"templatePack: " + defaults.DefaultPack + "\n" +
		"projectName: legacy-case\n" +
		"projectRoot: " + caseRoot + "\n"
	instancePath := filepath.Join(caseRoot, ".rekit", "instance.yml")
	if err := os.WriteFile(instancePath, []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}

	inst, err := Read(caseRoot)
	if err != nil {
		t.Fatal(err)
	}
	if inst.Source != "instance" || inst.InstancePath != instancePath || inst.StateDir != ".rekit" {
		t.Fatalf("instance = %#v, want legacy ReKit state root", inst)
	}
}

func TestReadRejectsConflictingMutableStateRoots(t *testing.T) {
	caseRoot := t.TempDir()
	for _, stateDir := range []string{".steamai", ".rekit"} {
		if err := os.MkdirAll(filepath.Join(caseRoot, stateDir), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(filepath.Join(caseRoot, stateDir, "instance.yml"), []byte("templatePack: "+defaults.DefaultPack+"\n"), 0o644); err != nil {
			t.Fatal(err)
		}
	}

	if LooksLikeCase(caseRoot) {
		t.Fatal("conflicting mutable state roots must not be accepted as a case")
	}
	if _, err := Read(caseRoot); err == nil {
		t.Fatal("Read must fail closed when .steamai and .rekit both contain instance metadata")
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
