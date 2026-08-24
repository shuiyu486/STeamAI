package cli

import (
	"io/fs"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/shuiyu486/re-context-kits/internal/rekit/testfixture"
)

type cliFixtureOptions struct {
	caseMode string
	cwd      string
}

type cliFixture struct {
	repoRoot string
	caseRoot string
}

func newCLIFixture(t *testing.T, options cliFixtureOptions) *cliFixture {
	t.Helper()
	sourceRoot := repoRoot(t)
	fixtureRoot := filepath.Join(t.TempDir(), "kit")
	copyCLIFixtureRepo(t, sourceRoot, fixtureRoot)

	fixture := &cliFixture{repoRoot: fixtureRoot}
	switch options.caseMode {
	case "":
	case "attached":
		fixture.caseRoot = fixture.attachedCase(t)
	case "full":
		fixture.caseRoot = fixture.fullAttachedCase(t)
	default:
		t.Fatalf("unsupported CLI fixture case mode %q", options.caseMode)
	}

	cwd := fixtureRoot
	if options.cwd != "" {
		if fixture.caseRoot == "" {
			t.Fatal("CLI fixture cwd requires an attached case")
		}
		cwd = filepath.Join(fixture.caseRoot, filepath.FromSlash(options.cwd))
		if err := os.MkdirAll(cwd, 0o755); err != nil {
			t.Fatal(err)
		}
	}
	fixture.chdir(t, cwd)
	return fixture
}

func (f *cliFixture) chdir(t *testing.T, cwd string) {
	t.Helper()
	oldwd, err := os.Getwd()
	if err != nil {
		t.Fatal(err)
	}
	if err := os.Chdir(cwd); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		if err := os.Chdir(oldwd); err != nil {
			t.Fatal(err)
		}
	})
}

func (f *cliFixture) attachedCase(t *testing.T) string {
	t.Helper()
	return f.attachedCaseWithPack(t, "_template", false)
}

func (f *cliFixture) fullAttachedCase(t *testing.T) string {
	t.Helper()
	return f.attachedCaseWithPack(t, "_template", true)
}

func (f *cliFixture) attachedCaseWithPack(t *testing.T, pack string, full bool) string {
	t.Helper()
	project := testfixture.NewProject(t, testfixture.ProjectOptions{
		Layout:      testfixture.LegacyCase,
		SourceRepo:  f.repoRoot,
		CaseRoot:    filepath.Join(t.TempDir(), "kit"),
		Pack:        pack,
		ProjectName: "demo",
	})
	caseRoot := project.CaseRoot
	if full {
		copyRepoFile(t, f.repoRoot, "rekit/templates/case-shim/SKILL.md", caseRoot, ".claude/skills/rekit/SKILL.md")
		for _, rel := range []string{
			"packs/_template/references/template/README.md",
			"packs/_template/references/template/agent-team.md",
			"packs/_template/references/template/workflow-template.md",
			"packs/_template/references/template/toolchain-router.md",
			"packs/_template/CLAUDE.local.snippet.md",
		} {
			target := strings.TrimPrefix(rel, "packs/_template/")
			if target == "CLAUDE.local.snippet.md" {
				target = "CLAUDE.local.md"
			}
			copyRepoFile(t, f.repoRoot, rel, caseRoot, target)
		}
		template, err := os.ReadFile(filepath.Join(f.repoRoot, "packs", "_template", "references", "template", "task-handoff.template.md"))
		if err != nil {
			t.Fatal(err)
		}
		text := strings.ReplaceAll(string(template), "<PROJECT_NAME>", "demo")
		text = strings.ReplaceAll(text, "<PROJECT_ROOT>", caseRoot)
		writeCaseFile(t, caseRoot, "references/template/task-handoff.md", text)
	}
	return caseRoot
}

func copyCLIFixtureRepo(t *testing.T, sourceRoot, targetRoot string) {
	t.Helper()
	within, err := cliFixtureTargetWithinSource(sourceRoot, targetRoot)
	if err != nil {
		t.Fatalf("resolve CLI fixture containment: %v", err)
	}
	if within {
		t.Fatalf("CLI fixture target must be outside source repo: source=%s target=%s", sourceRoot, targetRoot)
	}
	if err := filepath.WalkDir(sourceRoot, func(path string, entry fs.DirEntry, walkErr error) error {
		if walkErr != nil {
			return walkErr
		}
		rel, err := filepath.Rel(sourceRoot, path)
		if err != nil {
			return err
		}
		if rel == "." {
			return os.MkdirAll(targetRoot, 0o755)
		}
		if isCLIFixtureSkippedPath(rel) {
			if entry.IsDir() {
				return filepath.SkipDir
			}
			return nil
		}
		if entry.IsDir() {
			return os.MkdirAll(filepath.Join(targetRoot, rel), 0o755)
		}
		if err := os.MkdirAll(filepath.Dir(filepath.Join(targetRoot, rel)), 0o755); err != nil {
			return err
		}
		data, err := os.ReadFile(path)
		if err != nil {
			return err
		}
		mode := entry.Type().Perm()
		if mode == 0 {
			mode = 0o644
		}
		return os.WriteFile(filepath.Join(targetRoot, rel), data, mode)
	}); err != nil {
		t.Fatal(err)
	}
}

func cliFixtureTargetWithinSource(sourceRoot, targetRoot string) (bool, error) {
	source, err := filepath.Abs(sourceRoot)
	if err != nil {
		return false, err
	}
	target, err := filepath.Abs(targetRoot)
	if err != nil {
		return false, err
	}
	if sourceVolume, targetVolume := filepath.VolumeName(source), filepath.VolumeName(target); sourceVolume != "" && targetVolume != "" && !strings.EqualFold(sourceVolume, targetVolume) {
		return false, nil
	}
	rel, err := filepath.Rel(source, target)
	if err != nil {
		return false, err
	}
	return rel == "." || (rel != ".." && !strings.HasPrefix(rel, ".."+string(filepath.Separator))), nil
}

func TestCLIFixtureTargetWithinSource(t *testing.T) {
	root := t.TempDir()
	for name, target := range map[string]struct {
		target string
		want   bool
	}{
		"nested":  {filepath.Join(root, "tmp", "fixture"), true},
		"same":    {root, true},
		"sibling": {filepath.Join(filepath.Dir(root), "sibling-fixture"), false},
	} {
		t.Run(name, func(t *testing.T) {
			got, err := cliFixtureTargetWithinSource(root, target.target)
			if err != nil {
				t.Fatal(err)
			}
			if got != target.want {
				t.Fatalf("within = %t, want %t", got, target.want)
			}
		})
	}
}

func isCLIFixtureSkippedPath(rel string) bool {
	rel = filepath.ToSlash(rel)
	if rel == ".git" || rel == ".codegraph" || rel == ".claude/worktrees" || strings.HasPrefix(rel, ".git/") || strings.HasPrefix(rel, ".codegraph/") || strings.HasPrefix(rel, ".claude/worktrees/") {
		return true
	}
	parts := strings.Split(rel, "/")
	if len(parts) >= 3 && parts[0] == "packs" && parts[2] == "promote-candidates" {
		return true
	}
	return len(parts) >= 4 && parts[0] == "packs" && parts[2] == "tooling" && parts[3] == "candidates"
}
