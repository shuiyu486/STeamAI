package vnextcontract

import (
	"bytes"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"
)

func TestRepositoryHasNoLegacyProductionControlPlane(t *testing.T) {
	repo := repoRoot(t)
	for _, rel := range []string{
		"cmd",
		"internal/rekit",
		"rekit",
		".claude/skills/rekit",
		".claude/skills/verify",
		"go.work",
		"go.sum",
		"packs/binary-re/scripts",
	} {
		if _, err := os.Lstat(filepath.Join(repo, filepath.FromSlash(rel))); err == nil {
			t.Errorf("legacy production path still exists: %s", rel)
		} else if !os.IsNotExist(err) {
			t.Fatalf("inspect %s: %v", rel, err)
		}
	}

	entries, err := os.ReadDir(filepath.Join(repo, "internal", "steamai", "vnextcontract"))
	if err != nil {
		t.Fatal(err)
	}
	for _, entry := range entries {
		if entry.IsDir() || !strings.HasSuffix(entry.Name(), ".go") {
			continue
		}
		if !strings.HasSuffix(entry.Name(), "_test.go") {
			t.Errorf("vNext contract package contains production Go source: %s", entry.Name())
		}
	}
}

func TestCanonicalCurrentSurfacesDoNotInvokeRemovedRuntime(t *testing.T) {
	repo := repoRoot(t)
	paths := []string{
		"README.md",
		"CLAUDE.md",
		"docs/context-routing.md",
		".github/workflows/release-gate.yml",
		"common/stop-hook-checklist.md",
		"vnext/README.md",
		"vnext/project-skill/SKILL.md",
	}
	forbidden := []string{
		"go run ./cmd/rekit",
		"go run ./cmd/rekit-host",
		"cmd/skillcontractgen",
		"./rekit/rekit.ps1",
		"/rekit status",
		"/rekit continue",
		"/rekit gate",
		"/rekit doctor",
		"/rekit promote",
		".rekit/facts",
		".rekit/lanes",
		"authorized-gate",
		"project-local deterministic runtime",
	}
	for _, rel := range paths {
		data, err := os.ReadFile(filepath.Join(repo, filepath.FromSlash(rel)))
		if err != nil {
			t.Fatalf("read %s: %v", rel, err)
		}
		text := string(data)
		for _, phrase := range forbidden {
			if strings.Contains(text, phrase) {
				t.Errorf("current surface %s invokes removed runtime phrase %q", rel, phrase)
			}
		}
	}
}

func TestCanonicalSkillAndDeliveryTemplateAreExactBytes(t *testing.T) {
	repo := repoRoot(t)
	canonical, err := os.ReadFile(filepath.Join(repo, ".claude", "skills", "steamai", "SKILL.md"))
	if err != nil {
		t.Fatal(err)
	}
	template, err := os.ReadFile(filepath.Join(repo, "vnext", "project-skill", "SKILL.md"))
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(canonical, template) {
		t.Fatal("canonical skill and project delivery template differ at the byte level")
	}
}

func TestAllPackManifestsUseThinDeclarativeShape(t *testing.T) {
	repo := repoRoot(t)
	packsRoot := filepath.Join(repo, "packs")
	entries, err := os.ReadDir(packsRoot)
	if err != nil {
		t.Fatal(err)
	}
	forbidden := []string{
		"workstreamDefaults:",
		"authorityFiles:",
		"subagentRoutes:",
		"laneTypes:",
		"mainAgentOwns:",
		".rekit/",
		".steamai/facts/",
		"authorized-gate",
		"ledger-writeback",
	}
	for _, entry := range entries {
		if !entry.IsDir() {
			continue
		}
		rel := filepath.ToSlash(filepath.Join("packs", entry.Name(), "manifest.yml"))
		data, err := os.ReadFile(filepath.Join(repo, filepath.FromSlash(rel)))
		if os.IsNotExist(err) {
			continue
		}
		if err != nil {
			t.Fatalf("read %s: %v", rel, err)
		}
		text := string(data)
		if !strings.HasPrefix(text, "schemaVersion: 2\n") {
			t.Errorf("thin pack manifest %s must start with schemaVersion 2", rel)
		}
		for _, required := range []string{"name:", "entrypoints:", "teamHints:", "heavyActions:", "learningTargets:"} {
			if !strings.Contains(text, required) {
				t.Errorf("thin pack manifest %s missing %q", rel, required)
			}
		}
		for _, phrase := range forbidden {
			if strings.Contains(text, phrase) {
				t.Errorf("thin pack manifest %s retains legacy field %q", rel, phrase)
			}
		}
	}
}

func TestPackPolicyOverlaysExtendRegisteredCommonPolicies(t *testing.T) {
	repo := repoRoot(t)
	commonData, err := os.ReadFile(filepath.Join(repo, "common", "policies", "manifest.yml"))
	if err != nil {
		t.Fatal(err)
	}
	registered := map[string]bool{}
	for line := range strings.SplitSeq(string(commonData), "\n") {
		line = strings.TrimSpace(line)
		if id, ok := strings.CutPrefix(line, "- id: "); ok {
			registered[strings.TrimSpace(id)] = true
		}
	}
	if len(registered) == 0 {
		t.Fatal("common policy manifest registered no policies")
	}
	err = filepath.WalkDir(filepath.Join(repo, "packs"), func(path string, entry os.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if entry.IsDir() || entry.Name() != "manifest.yml" || filepath.Base(filepath.Dir(path)) != "policies" {
			return nil
		}
		data, err := os.ReadFile(path)
		if err != nil {
			return err
		}
		for line := range strings.SplitSeq(string(data), "\n") {
			line = strings.TrimSpace(line)
			if base, ok := strings.CutPrefix(line, "extends: "); ok && !registered[strings.TrimSpace(base)] {
				rel, _ := filepath.Rel(repo, path)
				t.Errorf("pack policy manifest %s extends unregistered common policy %q", filepath.ToSlash(rel), strings.TrimSpace(base))
			}
		}
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
}

func TestPackAndCommonSourcesDoNotExposeLegacyCommands(t *testing.T) {
	repo := repoRoot(t)
	roots := []string{"common", "packs"}
	forbidden := []string{
		"/rekit ",
		"`.rekit/",
		".rekit/facts",
		".rekit/lanes",
		"/steamai doctor",
		"/steamai promote",
		"/steamai sync",
		"authorized-gate",
		"strict durable autonomy profile",
		"go run ./cmd/",
		"runtime bundle",
	}
	for _, root := range roots {
		err := filepath.WalkDir(filepath.Join(repo, root), func(path string, entry os.DirEntry, err error) error {
			if err != nil {
				return err
			}
			if entry.IsDir() {
				return nil
			}
			ext := strings.ToLower(filepath.Ext(path))
			if !slices.Contains([]string{".md", ".yml", ".yaml", ".json"}, ext) {
				return nil
			}
			data, err := os.ReadFile(path)
			if err != nil {
				return err
			}
			for _, phrase := range forbidden {
				if strings.Contains(string(data), phrase) {
					rel, _ := filepath.Rel(repo, path)
					t.Errorf("current declarative source %s retains legacy phrase %q", filepath.ToSlash(rel), phrase)
				}
			}
			return nil
		})
		if err != nil {
			t.Fatal(err)
		}
	}
}
