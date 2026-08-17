package attach

import (
	"os"
	"path/filepath"
	"testing"
)

func TestBuildPlanUsesSTeamAIRootForNewProject(t *testing.T) {
	repoRoot := attachFixtureRepo(t)
	caseRoot := filepath.Join(t.TempDir(), "project")
	plan, err := Preview(repoRoot, caseRoot, "_template", Options{ProjectName: "demo"})
	if err != nil {
		t.Fatal(err)
	}
	want := map[string]bool{
		filepath.Join(caseRoot, ".steamai", "instance.yml"):                 false,
		filepath.Join(caseRoot, ".claude", "skills", "steamai", "SKILL.md"): false,
		filepath.Join(caseRoot, ".steamai", "state.json"):                   false,
	}
	for _, write := range plan.Writes {
		if _, ok := want[write.Path]; ok {
			want[write.Path] = true
		}
	}
	for path, found := range want {
		if !found {
			t.Fatalf("new STeamAI attach plan missing %s: %+v", path, plan.Writes)
		}
	}
}

func TestBuildPlanKeepsLegacyRoot(t *testing.T) {
	repoRoot := attachFixtureRepo(t)
	caseRoot := t.TempDir()
	if err := os.Mkdir(filepath.Join(caseRoot, ".rekit"), 0o755); err != nil {
		t.Fatal(err)
	}
	plan, err := Preview(repoRoot, caseRoot, "_template", Options{ProjectName: "legacy"})
	if err != nil {
		t.Fatal(err)
	}
	for _, want := range []string{
		filepath.Join(caseRoot, ".rekit", "instance.yml"),
		filepath.Join(caseRoot, ".claude", "skills", "rekit", "SKILL.md"),
		filepath.Join(caseRoot, ".rekit", "state.json"),
	} {
		found := false
		for _, write := range plan.Writes {
			found = found || write.Path == want
		}
		if !found {
			t.Fatalf("legacy attach plan missing %s: %+v", want, plan.Writes)
		}
	}
}

func attachFixtureRepo(t *testing.T) string {
	t.Helper()
	root := t.TempDir()
	if err := os.MkdirAll(filepath.Join(root, "packs", "_template"), 0o755); err != nil {
		t.Fatal(err)
	}
	manifest := "schemaVersion: '1.0'\npack:\n  name: _template\n  version: 0.0.0\nmanagedFiles: []\npromoteFiles: []\ntoolingFiles: []\ntemplateFiles: []\nmanagedBlock:\n  file: CLAUDE.local.md\n  blockId: unit\n  source: CLAUDE.local.snippet.md\n"
	if err := os.WriteFile(filepath.Join(root, "packs", "_template", "manifest.yml"), []byte(manifest), 0o644); err != nil {
		t.Fatal(err)
	}
	return root
}
