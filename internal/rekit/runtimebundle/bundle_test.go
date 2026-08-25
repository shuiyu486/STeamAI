package runtimebundle

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestBuildAndValidateStrictUnifiedLayout(t *testing.T) {
	repoRoot, err := filepath.Abs(filepath.Join("..", "..", ".."))
	if err != nil {
		t.Fatal(err)
	}
	caseRoot := t.TempDir()
	executable := filepath.Join(t.TempDir(), ExecutableName())
	if err := os.WriteFile(executable, []byte("test runtime executable\n"), 0o755); err != nil {
		t.Fatal(err)
	}
	plan, err := PublishForTest(caseRoot, repoRoot, "_template", executable)
	if err != nil {
		t.Fatal(err)
	}
	assetRoot := filepath.Join(caseRoot, ".steamai")
	manifest, err := Validate(assetRoot, ManifestRel, plan.ManifestSHA256, "_template")
	if err != nil {
		t.Fatal(err)
	}
	if manifest.PacksRoot != "packs" || manifest.Executable.Path != filepath.ToSlash(filepath.Join("runtime", "bin", ExecutableName())) {
		t.Fatalf("unexpected manifest layout: %+v", manifest)
	}
	if _, err := os.Lstat(filepath.Join(assetRoot, "runtime", "packs")); !os.IsNotExist(err) {
		t.Fatalf("packs must not be duplicated beneath runtime: %v", err)
	}
	for _, artifact := range manifest.Files {
		if strings.HasSuffix(strings.ToLower(artifact.Path), ".ps1") {
			t.Fatalf("bundle included PowerShell asset: %s", artifact.Path)
		}
	}
}

func TestBuildRejectsProjectSkillProvenanceDrift(t *testing.T) {
	repoRoot, err := filepath.Abs(filepath.Join("..", "..", ".."))
	if err != nil {
		t.Fatal(err)
	}
	fixtureRoot := t.TempDir()
	for _, rel := range []string{
		".claude/skills/steamai/SKILL.md",
		"rekit/templates/steamai-project/SKILL.md",
		"packs/_template/manifest.yml",
		"common/policies/manifest.yml",
		"common/policies/README.md",
		"rekit/schemas/instance.schema.yml",
		"rekit/schemas/pack-manifest.schema.yml",
		"rekit/tests/catalog.json",
	} {
		source := filepath.Join(repoRoot, filepath.FromSlash(rel))
		data, err := os.ReadFile(source)
		if err != nil {
			t.Fatal(err)
		}
		target := filepath.Join(fixtureRoot, filepath.FromSlash(rel))
		if err := os.MkdirAll(filepath.Dir(target), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(target, data, 0o644); err != nil {
			t.Fatal(err)
		}
	}
	templatePath := filepath.Join(
		fixtureRoot,
		filepath.FromSlash("rekit/templates/steamai-project/SKILL.md"),
	)
	if err := os.WriteFile(templatePath, []byte("drift\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	executable := filepath.Join(t.TempDir(), ExecutableName())
	if err := os.WriteFile(executable, []byte("test runtime executable\n"), 0o755); err != nil {
		t.Fatal(err)
	}
	if _, err := BuildWithExecutable(fixtureRoot, "_template", executable); err == nil ||
		!strings.Contains(err.Error(), "skill provenance") {
		t.Fatalf("drifted project skill bundle error=%v", err)
	}
}

func TestBuildExcludesGeneratedPackState(t *testing.T) {
	fixtureRoot := t.TempDir()
	fixturePackRoot := filepath.Join(fixtureRoot, "packs", "_template")
	for _, rel := range []string{
		"packs/_template/manifest.yml",
		"common/policies/manifest.yml",
		"common/policies/README.md",
		"rekit/schemas/instance.schema.yml",
		"rekit/schemas/pack-manifest.schema.yml",
		"rekit/tests/catalog.json",
	} {
		path := filepath.Join(fixtureRoot, filepath.FromSlash(rel))
		if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(path, []byte("fixture\n"), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	repoRoot, err := filepath.Abs(filepath.Join("..", "..", ".."))
	if err != nil {
		t.Fatal(err)
	}
	for _, rel := range []string{
		".claude/skills/steamai/SKILL.md",
		"rekit/templates/steamai-project/SKILL.md",
	} {
		data, err := os.ReadFile(filepath.Join(repoRoot, filepath.FromSlash(rel)))
		if err != nil {
			t.Fatal(err)
		}
		path := filepath.Join(fixtureRoot, filepath.FromSlash(rel))
		if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(path, data, 0o644); err != nil {
			t.Fatal(err)
		}
	}
	for rel, content := range map[string]string{
		"promote-candidates/.decision.lock":         "mutable lock\n",
		"promote-candidates/index.json":             "[]\n",
		"tooling/candidates/generated.candidate.md": "generated candidate\n",
	} {
		path := filepath.Join(fixturePackRoot, filepath.FromSlash(rel))
		if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	executable := filepath.Join(t.TempDir(), ExecutableName())
	if err := os.WriteFile(executable, []byte("test runtime executable\n"), 0o755); err != nil {
		t.Fatal(err)
	}
	plan, err := BuildWithExecutable(fixtureRoot, "_template", executable)
	if err != nil {
		t.Fatal(err)
	}
	for _, artifact := range plan.Manifest.Files {
		lower := strings.ToLower(filepath.ToSlash(artifact.Path))
		if strings.Contains(lower, "/promote-candidates/") || strings.HasSuffix(lower, "/promote-candidates") || strings.Contains(lower, "/tooling/candidates/") || strings.HasSuffix(lower, "/tooling/candidates") {
			t.Fatalf("bundle included generated pack state: %s", artifact.Path)
		}
	}
	for _, publication := range plan.Publications {
		lower := strings.ToLower(filepath.ToSlash(publication.Path))
		if strings.Contains(lower, "/promote-candidates/") || strings.HasSuffix(lower, "/promote-candidates") || strings.Contains(lower, "/tooling/candidates/") || strings.HasSuffix(lower, "/tooling/candidates") {
			t.Fatalf("bundle planned generated pack state publication: %s", publication.Path)
		}
	}
}

func TestValidateExecutableIdentityBindsCanonicalProjectImage(t *testing.T) {
	repoRoot, err := filepath.Abs(filepath.Join("..", "..", ".."))
	if err != nil {
		t.Fatal(err)
	}
	caseRoot := t.TempDir()
	source := filepath.Join(t.TempDir(), ExecutableName())
	if err := os.WriteFile(source, []byte("test runtime executable\n"), 0o755); err != nil {
		t.Fatal(err)
	}
	plan, err := PublishForTest(caseRoot, repoRoot, "_template", source)
	if err != nil {
		t.Fatal(err)
	}
	assetRoot := filepath.Join(caseRoot, ".steamai")
	executable := ExecutablePath(assetRoot, plan.Manifest)
	gotRoot, projectLocal, err := AssetRootForExecutable(executable)
	if err != nil || !projectLocal || !strings.EqualFold(gotRoot, assetRoot) {
		t.Fatalf("project-local executable root=%q local=%t err=%v", gotRoot, projectLocal, err)
	}
	manifest, err := Validate(assetRoot, ManifestRel, plan.ManifestSHA256, "_template")
	if err != nil {
		t.Fatal(err)
	}
	if err := ValidateExecutableIdentity(assetRoot, executable, manifest); err != nil {
		t.Fatal(err)
	}
	outside := filepath.Join(t.TempDir(), ExecutableName())
	if err := os.WriteFile(outside, []byte("test runtime executable\n"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := ValidateExecutableIdentity(assetRoot, outside, manifest); err == nil {
		t.Fatal("same-byte executable outside the canonical bundle passed process identity validation")
	}
	for _, wrongLayout := range []string{
		filepath.Join(assetRoot, "runtime", "other", ExecutableName()),
		filepath.Join(assetRoot, "runtime", "bin", "renamed-"+ExecutableName()),
	} {
		if err := os.MkdirAll(filepath.Dir(wrongLayout), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(wrongLayout, []byte("test runtime executable\n"), 0o755); err != nil {
			t.Fatal(err)
		}
		if _, _, err := AssetRootForExecutable(wrongLayout); err == nil {
			t.Fatalf("project-local executable in the wrong location passed layout detection: %s", wrongLayout)
		}
	}
}

func TestValidateRejectsTamperAndExtraControlledFile(t *testing.T) {
	repoRoot, err := filepath.Abs(filepath.Join("..", "..", ".."))
	if err != nil {
		t.Fatal(err)
	}
	executable := filepath.Join(t.TempDir(), ExecutableName())
	if err := os.WriteFile(executable, []byte("test runtime executable\n"), 0o755); err != nil {
		t.Fatal(err)
	}
	for name, mutate := range map[string]func(string, Plan) error{
		"artifact tamper": func(assetRoot string, plan Plan) error {
			return os.WriteFile(filepath.Join(assetRoot, filepath.FromSlash(plan.Manifest.Files[0].Path)), []byte("tampered\n"), 0o644)
		},
		"extra controlled file": func(assetRoot string, _ Plan) error {
			return os.WriteFile(filepath.Join(assetRoot, "runtime", "extra.bin"), []byte("extra\n"), 0o644)
		},
	} {
		t.Run(name, func(t *testing.T) {
			caseRoot := t.TempDir()
			plan, err := PublishForTest(caseRoot, repoRoot, "_template", executable)
			if err != nil {
				t.Fatal(err)
			}
			assetRoot := filepath.Join(caseRoot, ".steamai")
			if err := mutate(assetRoot, plan); err != nil {
				t.Fatal(err)
			}
			if _, err := Validate(assetRoot, ManifestRel, plan.ManifestSHA256, "_template"); err == nil {
				t.Fatal("tampered bundle passed strict validation")
			}
		})
	}
}
