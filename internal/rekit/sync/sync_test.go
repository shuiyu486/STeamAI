package sync

import (
	"bytes"
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestApplyPreviewDoesNotWrite(t *testing.T) {
	repoRoot, caseRoot, pack := syncFixture(t)
	before := snapshotFiles(t, caseRoot)

	result, err := ApplyPreview(repoRoot, caseRoot, pack, ApplyOptions{ProjectName: "preview-demo"})
	if err != nil {
		t.Fatal(err)
	}
	if result.Command != "sync" || result.IsMutation || result.Applied || result.BackupRoot == "" {
		t.Fatalf("unexpected sync preview result: %+v", result)
	}
	assertSyncWriteForTest(t, result.Writes, "references/template/README.md", "overwrite-with-backup", true)
	assertSyncWriteForTest(t, result.Writes, "references/template/task-handoff.md", "skip-existing-local-file", false)
	assertSyncWriteForTest(t, result.Writes, "CLAUDE.local.md", "replace-managed-block", true)
	assertSyncWriteForTest(t, result.Writes, ".rekit/state.json", "refresh", false)
	assertNotExists(t, result.BackupRoot)
	assertSnapshotEqual(t, before, snapshotFiles(t, caseRoot))
}

func TestApplyWritesManagedBlockBackupAndState(t *testing.T) {
	repoRoot, caseRoot, pack := syncFixture(t)

	result, err := Apply(repoRoot, caseRoot, pack, ApplyOptions{ProjectName: "sync-demo"})
	if err != nil {
		t.Fatal(err)
	}
	if result.Command != "sync" || !result.IsMutation || !result.Applied || result.BackupRoot == "" {
		t.Fatalf("unexpected sync apply result: %+v", result)
	}
	readmeWrite := assertSyncWriteForTest(t, result.Writes, "references/template/README.md", "overwrite-with-backup", true)
	templateWrite := assertSyncWriteForTest(t, result.Writes, "references/template/task-handoff.md", "skip-existing-local-file", false)
	blockWrite := assertSyncWriteForTest(t, result.Writes, "CLAUDE.local.md", "replace-managed-block", true)
	assertSyncWriteForTest(t, result.Writes, ".rekit/instance.yml", "refresh", false)
	assertSyncWriteForTest(t, result.Writes, ".claude/skills/rekit/SKILL.md", "refresh", false)
	assertSyncWriteForTest(t, result.Writes, ".re-template.yml", "refresh", false)
	assertSyncWriteForTest(t, result.Writes, ".rekit/state.json", "refresh", false)

	packReadme := readFile(t, filepath.Join(repoRoot, "packs", pack, filepath.FromSlash("references/template/README.md")))
	caseReadme := readFile(t, filepath.Join(caseRoot, filepath.FromSlash("references/template/README.md")))
	if !bytes.Equal(caseReadme, packReadme) {
		t.Fatalf("managed README was not overwritten with pack content:\n%s", string(caseReadme))
	}
	if got := string(readFile(t, templateWrite.TargetPath)); !strings.Contains(got, "local handoff remains") {
		t.Fatalf("existing local template should be skipped, got:\n%s", got)
	}
	claude := string(readFile(t, blockWrite.TargetPath))
	if !strings.Contains(claude, "new managed block") || strings.Contains(claude, "old managed block") || !strings.Contains(claude, "prefix") || !strings.Contains(claude, "suffix") {
		t.Fatalf("managed block was not replaced correctly:\n%s", claude)
	}
	if got := string(readFile(t, readmeWrite.BackupPath)); !strings.Contains(got, "local README drift") {
		t.Fatalf("managed README backup did not preserve local text:\n%s", got)
	}
	if got := string(readFile(t, blockWrite.BackupPath)); !strings.Contains(got, "old managed block") {
		t.Fatalf("managed block backup did not preserve local host:\n%s", got)
	}
	statePath := filepath.Join(caseRoot, ".rekit", "state.json")
	var state struct {
		TemplateRoot string `json:"templateRoot"`
		TemplatePack string `json:"templatePack"`
		Managed      map[string]struct {
			SourceHash       string `json:"sourceHash"`
			TargetHashAtSync string `json:"targetHashAtSync"`
			LastAction       string `json:"lastAction"`
		} `json:"managed"`
	}
	if err := json.Unmarshal(readFile(t, statePath), &state); err != nil {
		t.Fatalf("sync state did not decode: %v", err)
	}
	entry := state.Managed["references/template/README.md"]
	if state.TemplateRoot != repoRoot || state.TemplatePack != pack || entry.SourceHash == "" || entry.TargetHashAtSync == "" || entry.LastAction != "sync" {
		t.Fatalf("unexpected sync state: %+v", state)
	}
}

func TestApplyForceOverwritesLocalTemplateWithBackup(t *testing.T) {
	repoRoot, caseRoot, pack := syncFixture(t)

	result, err := Apply(repoRoot, caseRoot, pack, ApplyOptions{ProjectName: "forced-demo", ForceLocalTemplates: true})
	if err != nil {
		t.Fatal(err)
	}
	templateWrite := assertSyncWriteForTest(t, result.Writes, "references/template/task-handoff.md", "overwrite-local-template-file-with-force", true)
	text := string(readFile(t, templateWrite.TargetPath))
	if !strings.Contains(text, "forced-demo") || !strings.Contains(text, caseRoot) || strings.Contains(text, "local handoff remains") {
		t.Fatalf("forced template did not replace placeholders:\n%s", text)
	}
	backup := string(readFile(t, templateWrite.BackupPath))
	if !strings.Contains(backup, "local handoff remains") {
		t.Fatalf("template backup did not preserve local handoff:\n%s", backup)
	}
}

func TestApplyPropagatesMutationLeaseUnlockError(t *testing.T) {
	repoRoot, caseRoot, pack := syncFixture(t)
	oldAcquire := acquireMutationLease
	acquireMutationLease = func(string) (mutationLease, error) {
		return syncUnlockErrorLease{}, nil
	}
	t.Cleanup(func() { acquireMutationLease = oldAcquire })

	_, err := Apply(repoRoot, caseRoot, pack, ApplyOptions{ProjectName: "unlock-demo"})
	if err == nil || !strings.Contains(err.Error(), "unlock fixture") {
		t.Fatalf("mutation lease unlock error was not propagated: %v", err)
	}
}

type syncUnlockErrorLease struct{}

func (syncUnlockErrorLease) Unlock() error { return errors.New("unlock fixture") }

func TestBackupCaseFileRejectsOutsideCaseRoot(t *testing.T) {
	caseRoot := t.TempDir()
	outside := filepath.Join(t.TempDir(), "outside.md")
	backupRoot := filepath.Join(caseRoot, ".rekit", "backups", "sync", "unit")
	writeText(t, outside, "outside")

	_, err := backupCaseFile(outside, caseRoot, backupRoot)
	if err == nil || !strings.Contains(err.Error(), "outside case root") {
		t.Fatalf("backupCaseFile error = %v, want outside case root", err)
	}
}

func syncFixture(t *testing.T) (repoRoot, caseRoot, pack string) {
	t.Helper()
	repoRoot = t.TempDir()
	caseRoot = filepath.Join(t.TempDir(), "case")
	pack = "unit-pack"
	packRoot := filepath.Join(repoRoot, "packs", pack)
	manifest := `schemaVersion: 1
name: unit-pack
version: 0.0.1
description: Unit test pack
maturity: template

managedFiles:
  - references/template/README.md

templateFiles:
  - references/template/task-handoff.template.md

localNeverOverwrite:
  - references/template/task-handoff.md

promoteFiles:
  - references/template/README.md

managedBlock:
  file: CLAUDE.local.md
  blockId: unit:router
  source: CLAUDE.local.snippet.md

toolingCandidateSources:
  - references/template/README.md

workstreamDefaults:
  defaultAuthorityLane: main
  defaultStartLaneType: feature
  backupRoot: .rekit/backups/sync
  requestDefaultTargetLane: main

authorityFiles:
  - references/template/README.md

commonPolicies: []
policyOverlays: []
subagentRoutes: []
toolingFiles: []
promptFiles: []

syncPolicy:
  managedFiles: overwrite-with-backup
  templateFiles: create-if-missing
  localFiles: never-overwrite

promoteDenyPatterns:
  - "artifacts[\\/]"

budgets:
  defaultMarkdown: 16384

heavyToolGates:
  - id: debug
    title: Debug
    sideEffects: debug,filesystem-write
    defaultRisk: high
    requiresConfirmation: true
    stopConditions: timeout

laneTypes:
  - id: main
    title: Main
    authority: true
    workspaceRoot: workspace/main
    canWrite: references/template/README.md
    readOnly: .rekit/facts/**
    outputs: publication,decision,observation
  - id: feature
    title: Feature
    authority: false
    workspaceRoot: workspace/features
    canWrite: own-workspace
    readOnly: references/template/**,.rekit/facts/**
    outputs: observation,request,candidate,summary
`
	writeText(t, filepath.Join(packRoot, "manifest.yml"), manifest)
	writeText(t, filepath.Join(repoRoot, "rekit", "templates", "case-shim", "SKILL.md"), "# thin shim\n")
	writeText(t, filepath.Join(packRoot, filepath.FromSlash("references/template/README.md")), "# Pack README\n\ncanonical managed text\n")
	writeText(t, filepath.Join(packRoot, filepath.FromSlash("references/template/task-handoff.template.md")), "# Handoff\n\nProject: <PROJECT_NAME>\nRoot: <PROJECT_ROOT>\n")
	writeText(t, filepath.Join(packRoot, "CLAUDE.local.snippet.md"), "<!-- BEGIN unit:router -->\nnew managed block\n<!-- END unit:router -->\n")

	metadata := "templateRoot: " + repoRoot + "\n" +
		"templatePack: " + pack + "\n" +
		"projectName: demo\n" +
		"projectRoot: " + caseRoot + "\n"
	writeText(t, filepath.Join(caseRoot, ".rekit", "instance.yml"), metadata)
	writeText(t, filepath.Join(caseRoot, filepath.FromSlash("references/template/README.md")), "# Local README\n\nlocal README drift\n")
	writeText(t, filepath.Join(caseRoot, filepath.FromSlash("references/template/task-handoff.md")), "# Local handoff\n\nlocal handoff remains\n")
	writeText(t, filepath.Join(caseRoot, "CLAUDE.local.md"), "prefix\n\n<!-- BEGIN unit:router -->\nold managed block\n<!-- END unit:router -->\n\nsuffix\n")
	return repoRoot, caseRoot, pack
}

func assertSyncWriteForTest(t *testing.T, writes []WriteResult, path, action string, wantBackup bool) WriteResult {
	t.Helper()
	for _, write := range writes {
		if write.Path != path {
			continue
		}
		if write.Action != action {
			t.Fatalf("sync write %s action = %q, want %q; write=%+v", path, write.Action, action, write)
		}
		if wantBackup && strings.TrimSpace(write.BackupPath) == "" {
			t.Fatalf("sync write %s missing backup path", path)
		}
		if !wantBackup && strings.TrimSpace(write.BackupPath) != "" {
			t.Fatalf("sync write %s backup path = %q, want empty", path, write.BackupPath)
		}
		return write
	}
	t.Fatalf("sync write %s not found in %+v", path, writes)
	return WriteResult{}
}

func writeText(t *testing.T, path, text string) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte(text), 0o644); err != nil {
		t.Fatal(err)
	}
}

func readFile(t *testing.T, path string) []byte {
	t.Helper()
	b, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	return b
}

func assertNotExists(t *testing.T, path string) {
	t.Helper()
	if _, err := os.Stat(path); !os.IsNotExist(err) {
		t.Fatalf("expected %s to be absent, stat err=%v", path, err)
	}
}

func snapshotFiles(t *testing.T, root string) map[string]string {
	t.Helper()
	out := map[string]string{}
	if _, err := os.Stat(root); os.IsNotExist(err) {
		return out
	}
	err := filepath.WalkDir(root, func(path string, d os.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if d.IsDir() {
			return nil
		}
		rel, err := filepath.Rel(root, path)
		if err != nil {
			return err
		}
		b, err := os.ReadFile(path)
		if err != nil {
			return err
		}
		out[filepath.ToSlash(rel)] = string(b)
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
	return out
}

func assertSnapshotEqual(t *testing.T, want, got map[string]string) {
	t.Helper()
	if len(want) != len(got) {
		t.Fatalf("snapshot length = %d, want %d; got=%+v want=%+v", len(got), len(want), got, want)
	}
	for path, wantText := range want {
		if got[path] != wantText {
			t.Fatalf("snapshot %s mismatch:\ngot:  %q\nwant: %q", path, got[path], wantText)
		}
	}
}
