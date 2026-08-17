package casehealth

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/shuiyu486/re-context-kits/internal/rekit/casebind"
	"github.com/shuiyu486/re-context-kits/internal/rekit/manifest"
	"github.com/shuiyu486/re-context-kits/internal/rekit/projectstate"
	"github.com/shuiyu486/re-context-kits/internal/rekit/review"
	"github.com/shuiyu486/re-context-kits/internal/rekit/runtimebundle"
)

func TestStaticUsesCurrentAndLegacyStateRoots(t *testing.T) {
	for _, tc := range []struct {
		name string
		dir  string
	}{
		{name: "current", dir: projectstate.CurrentDir},
		{name: "legacy", dir: projectstate.LegacyDir},
	} {
		t.Run(tc.name, func(t *testing.T) {
			repoRoot, caseRoot := caseHealthFixture(t, tc.dir)
			pack := "fixture"
			if tc.dir == projectstate.CurrentDir {
				pack = "_template"
			}
			rows, err := Static(repoRoot, caseRoot, pack)
			if err != nil {
				t.Fatal(err)
			}
			instancePath := filepath.Join(caseRoot, tc.dir, "instance.yml")
			if !caseHealthRowsContain(rows, instancePath) {
				t.Fatalf("rows omitted %s: %+v", instancePath, rows)
			}
			if tc.dir == projectstate.CurrentDir {
				if !caseHealthRowsContain(rows, filepath.Join(caseRoot, ".claude", "skills", "steamai", "SKILL.md")) {
					t.Fatalf("current rows omitted project-local /steamai skill: %+v", rows)
				}
				if caseHealthRowsContain(rows, filepath.Join(caseRoot, ".claude", "skills", "rekit", "SKILL.md")) {
					t.Fatalf("current rows unexpectedly require legacy /rekit shim: %+v", rows)
				}
			} else if !caseHealthRowsContain(rows, filepath.Join(caseRoot, ".claude", "skills", "rekit", "SKILL.md")) {
				t.Fatalf("legacy rows omitted /rekit shim: %+v", rows)
			}
		})
	}
}

func TestStaticRejectsDualStateRoots(t *testing.T) {
	repoRoot, caseRoot := caseHealthFixture(t, projectstate.CurrentDir)
	if err := os.Mkdir(filepath.Join(caseRoot, projectstate.LegacyDir), 0o700); err != nil {
		t.Fatal(err)
	}
	if _, err := Static(repoRoot, caseRoot, "_template"); err == nil || !strings.Contains(err.Error(), "must not coexist") {
		t.Fatalf("dual-root static error = %v", err)
	}
}

func caseHealthFixture(t *testing.T, stateDir string) (string, string) {
	t.Helper()
	if stateDir == projectstate.CurrentDir {
		return currentCaseHealthFixture(t)
	}
	return legacyCaseHealthFixture(t)
}

func currentCaseHealthFixture(t *testing.T) (string, string) {
	t.Helper()
	caseRoot := t.TempDir()
	executable := filepath.Join(t.TempDir(), runtimebundle.ExecutableName())
	if err := os.WriteFile(executable, []byte("case health test executable"), 0o700); err != nil {
		t.Fatal(err)
	}
	bundle, err := runtimebundle.PublishForTest(caseRoot, caseHealthKitRoot(t), "_template", executable)
	if err != nil {
		t.Fatal(err)
	}
	repoRoot := filepath.Join(caseRoot, projectstate.CurrentDir)
	m, err := manifest.Load(repoRoot, "_template")
	if err != nil {
		t.Fatal(err)
	}
	for _, rel := range m.ManagedFiles {
		source, err := m.SourcePath(rel)
		if err != nil {
			t.Fatal(err)
		}
		copyCaseHealthFile(t, source, filepath.Join(caseRoot, filepath.FromSlash(rel)))
	}
	for _, rel := range m.TemplateFiles {
		source, err := m.SourcePath(rel)
		if err != nil {
			t.Fatal(err)
		}
		content, err := os.ReadFile(source)
		if err != nil {
			t.Fatal(err)
		}
		text := strings.ReplaceAll(string(content), "<PROJECT_NAME>", "consumer")
		text = strings.ReplaceAll(text, "<PROJECT_ROOT>", caseRoot)
		target := strings.TrimSuffix(rel, ".template.md") + ".md"
		writeCaseHealthFile(t, filepath.Join(caseRoot, filepath.FromSlash(target)), text)
	}
	blockSource, err := m.SourcePath(m.ManagedBlock["source"])
	if err != nil {
		t.Fatal(err)
	}
	block, err := os.ReadFile(blockSource)
	if err != nil {
		t.Fatal(err)
	}
	writeCaseHealthFile(t, filepath.Join(caseRoot, filepath.FromSlash(m.ManagedBlock["file"])), review.ApplyManagedBlock("", m.ManagedBlock["blockId"], string(block)))
	writeCaseHealthFile(t, filepath.Join(caseRoot, projectstate.CurrentDir, "instance.yml"), casebind.STeamAIInstanceText(caseRoot, "_template", "consumer", runtimebundle.ManifestRel, bundle.ManifestSHA256))
	copyCaseHealthFile(t, filepath.Join(repoRoot, "rekit", "templates", "steamai-project", "SKILL.md"), filepath.Join(caseRoot, ".claude", "skills", "steamai", "SKILL.md"))
	return repoRoot, caseRoot
}

func legacyCaseHealthFixture(t *testing.T) (string, string) {
	t.Helper()
	caseRoot := t.TempDir()
	repoRoot := t.TempDir()
	packRoot := filepath.Join(repoRoot, "packs", "fixture")
	writeCaseHealthFile(t, filepath.Join(packRoot, "manifest.yml"), `schemaVersion: 1
name: fixture
version: 1.0.0
description: fixture
maturity: experimental
managedFiles:
  - memory.md
templateFiles: []
localNeverOverwrite: []
promoteFiles:
  - memory.md
commonPolicies: []
policyOverlays: []
toolingFiles: []
promptFiles: []
laneTypes:
  - id: main
    title: Main
    authority: true
    workspaceRoot: workspace/main
    canWrite: memory.md
    readOnly: .rekit/facts/**
    outputs: publication
  - id: feature
    title: Feature
    authority: false
    workspaceRoot: workspace/features
    canWrite: own-workspace
    readOnly: memory.md
    outputs: observation
authorityFiles:
  - memory.md
toolingCandidateSources:
  - memory.md
subagentRoutes: []
heavyToolGates:
  - id: debug
    title: Debug
    sideEffects: debug,filesystem-write
    defaultRisk: high
    requiresConfirmation: true
    stopConditions: timeout
promoteDenyPatterns:
  - "artifacts[\\/]"
managedBlock:
  file: CLAUDE.md
  blockId: rekit:fixture
  source: managed-block.md
syncPolicy:
  managedFiles: overwrite-with-backup
  templateFiles: create-if-missing
  localFiles: never-overwrite
workstreamDefaults:
  defaultAuthorityLane: main
  defaultStartLaneType: feature
  backupRoot: .rekit/backups/sync
  requestDefaultTargetLane: main
budgets:
  defaultMarkdown: 16384
`)
	writeCaseHealthFile(t, filepath.Join(packRoot, "memory.md"), "pack memory\n")
	writeCaseHealthFile(t, filepath.Join(packRoot, "managed-block.md"), "managed block\n")
	writeCaseHealthFile(t, filepath.Join(caseRoot, "memory.md"), "case memory\n")
	writeCaseHealthFile(t, filepath.Join(caseRoot, "CLAUDE.md"), "<!-- BEGIN rekit:fixture -->\nmanaged block\n<!-- END rekit:fixture -->\n")
	writeCaseHealthFile(t, filepath.Join(caseRoot, projectstate.LegacyDir, "instance.yml"), "schemaVersion: 1\ntemplateRoot: "+repoRoot+"\ntemplatePack: fixture\nprojectName: consumer\nprojectRoot: "+caseRoot+"\nmode: case-local-shim\n")
	writeCaseHealthFile(t, filepath.Join(caseRoot, ".re-template.yml"), "templateRoot: "+repoRoot+"\ntemplatePack: fixture\ncurrentProjectPath: "+caseRoot+"\n")
	copyCaseHealthFile(t, filepath.Join(caseHealthKitRoot(t), ".claude", "skills", "rekit", "SKILL.md"), filepath.Join(repoRoot, ".claude", "skills", "rekit", "SKILL.md"))
	copyCaseHealthFile(t, filepath.Join(caseHealthKitRoot(t), "rekit", "templates", "case-shim", "SKILL.md"), filepath.Join(repoRoot, "rekit", "templates", "case-shim", "SKILL.md"))
	copyCaseHealthFile(t, filepath.Join(caseHealthKitRoot(t), "rekit", "templates", "case-shim", "SKILL.md"), filepath.Join(caseRoot, ".claude", "skills", "rekit", "SKILL.md"))
	return repoRoot, caseRoot
}

func caseHealthKitRoot(t *testing.T) string {
	t.Helper()
	root, err := filepath.Abs(filepath.Join("..", "..", ".."))
	if err != nil {
		t.Fatal(err)
	}
	return root
}

func caseHealthRowsContain(rows []Row, path string) bool {
	for _, row := range rows {
		if filepath.Clean(row.File) == filepath.Clean(path) {
			return true
		}
	}
	return false
}

func copyCaseHealthFile(t *testing.T, source, target string) {
	t.Helper()
	data, err := os.ReadFile(source)
	if err != nil {
		t.Fatal(err)
	}
	writeCaseHealthFile(t, target, string(data))
}

func writeCaseHealthFile(t *testing.T, path, content string) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(path), 0o700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte(content), 0o600); err != nil {
		t.Fatal(err)
	}
}
