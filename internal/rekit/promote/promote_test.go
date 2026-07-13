package promote

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestCreateCandidatesWhatIfDoesNotWrite(t *testing.T) {
	repoRoot, caseRoot, pack := promoteFixture(t)

	result, err := CreateCandidates(repoRoot, caseRoot, pack, CandidateOptions{WhatIf: true})
	if err != nil {
		t.Fatal(err)
	}
	if result.Command != "promote" || result.IsMutation || result.Applied || result.Created != 2 || result.Blocked < 1 || result.RequiresCleanup {
		t.Fatalf("unexpected what-if candidate result: %+v", result)
	}
	if result.IndexPath == "" {
		t.Fatal("what-if result missing index path")
	}
	assertCandidateWriteForTest(t, result.Writes, "references/template/README.md", "would-create-candidate")
	assertCandidateWriteForTest(t, result.Writes, "references/template/workflow-template.md", "blocked-deny-pattern")
	assertCandidateWriteForTest(t, result.Writes, "references/template/toolchain-router.md", "would-create-candidate")
	for _, write := range result.Writes {
		if write.Action == "would-create-candidate" {
			assertNotExists(t, write.TargetPath)
		}
	}
	assertNotExists(t, result.IndexPath)
	assertTreeEmptyOrMissing(t, result.CandidateRoot)
	assertTreeEmptyOrMissing(t, result.ToolingRoot)
}

func TestCreateCandidatesWritesIndexAndSanitizedTooling(t *testing.T) {
	repoRoot, caseRoot, pack := promoteFixture(t)

	result, err := CreateCandidates(repoRoot, caseRoot, pack, CandidateOptions{})
	if err != nil {
		t.Fatal(err)
	}
	if result.Command != "promote" || !result.IsMutation || !result.Applied || result.Created != 2 || result.Blocked < 1 || !result.RequiresCleanup {
		t.Fatalf("unexpected candidate result: %+v", result)
	}
	readmeWrite := assertCandidateWriteForTest(t, result.Writes, "references/template/README.md", "create-candidate")
	workflowWrite := assertCandidateWriteForTest(t, result.Writes, "references/template/workflow-template.md", "blocked-deny-pattern")
	toolingWrite := assertCandidateWriteForTest(t, result.Writes, "references/template/toolchain-router.md", "create-candidate")
	if err := assertInsideRoot(result.CandidateRoot, readmeWrite.TargetPath); err != nil {
		t.Fatalf("managed candidate escaped candidate root: %v", err)
	}
	if err := assertInsideRoot(result.ToolingRoot, toolingWrite.TargetPath); err != nil {
		t.Fatalf("tooling candidate escaped tooling root: %v", err)
	}
	if workflowWrite.TargetPath != filepath.Join(repoRoot, "packs", pack, filepath.FromSlash("references/template/workflow-template.md")) {
		t.Fatalf("blocked write target = %q, want pack source", workflowWrite.TargetPath)
	}
	assertExists(t, readmeWrite.TargetPath)
	assertExists(t, toolingWrite.TargetPath)
	assertExists(t, result.IndexPath)

	var index []candidateIndexEntry
	b, err := os.ReadFile(result.IndexPath)
	if err != nil {
		t.Fatal(err)
	}
	if err := json.Unmarshal(b, &index); err != nil {
		t.Fatalf("candidate index did not decode: %v\n%s", err, string(b))
	}
	if len(index) != 1 || index[0].Path != "references/template/README.md" || index[0].Candidate != readmeWrite.TargetPath {
		t.Fatalf("unexpected candidate index: %+v", index)
	}

	readme, err := os.ReadFile(readmeWrite.TargetPath)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(readme), "Reusable package-test candidate") {
		t.Fatalf("managed candidate missing case content:\n%s", string(readme))
	}
	tooling, err := os.ReadFile(toolingWrite.TargetPath)
	if err != nil {
		t.Fatal(err)
	}
	toolingText := string(tooling)
	for _, expected := range []string{"<caseRoot>", "<absolutePath>", "<artifactsPath>", "<capturesPath>", "<address>", "<ctxNNN>", "<roundN>", "Task #<n>"} {
		if !strings.Contains(toolingText, expected) {
			t.Fatalf("tooling candidate missing %q:\n%s", expected, toolingText)
		}
	}
	for _, unexpected := range []string{caseRoot, "C:\\cases", "demo-trace.csv", "demo-dump.bin", "0x401000", "ctx123", "round7", "Task #99"} {
		if strings.Contains(toolingText, unexpected) {
			t.Fatalf("tooling candidate still contains %q:\n%s", unexpected, toolingText)
		}
	}
}

func TestSanitizeToolingCandidateRedactsCaseSpecificValues(t *testing.T) {
	caseRoot := filepath.Join(t.TempDir(), "targetalpha")
	input := "Case root: " + caseRoot + "\n" +
		"Absolute: C:\\cases\\targetalpha\\sample.exe\n" +
		"Artifacts: artifacts/run/demo-trace.csv\n" +
		"Captures: captures/run/demo-dump.bin\n" +
		"Address: 0x401000 ctx123 round7 Task #99\n"

	out, counts := sanitizeToolingCandidate(input, caseRoot)
	for _, expected := range []string{"<caseRoot>", "<absolutePath>", "<artifactsPath>", "<capturesPath>", "<address>", "<ctxNNN>", "<roundN>", "Task #<n>"} {
		if !strings.Contains(out, expected) {
			t.Fatalf("sanitized output missing %q:\n%s", expected, out)
		}
	}
	for _, key := range []string{"caseRoot", "absolutePath", "artifactsPath", "capturesPath", "address", "ctx", "round", "task"} {
		if counts[key] == 0 {
			t.Fatalf("replacement count for %s = 0; counts=%+v output=%s", key, counts, out)
		}
	}
	for _, unexpected := range []string{caseRoot, "targetalpha", "C:\\cases", "demo-trace.csv", "demo-dump.bin", "0x401000", "ctx123", "round7", "Task #99"} {
		if strings.Contains(out, unexpected) {
			t.Fatalf("sanitized output still contains %q:\n%s", unexpected, out)
		}
	}
}

func TestUniqueCandidatePathAvoidsExistingFiles(t *testing.T) {
	root := t.TempDir()
	writeText(t, filepath.Join(root, "candidate.md"), "existing")
	path, err := uniqueCandidatePath(root, "candidate.md")
	if err != nil {
		t.Fatal(err)
	}
	if path != filepath.Join(root, "candidate-1.md") {
		t.Fatalf("uniqueCandidatePath = %q, want candidate-1.md", path)
	}
}

func TestRestorePromoteBackupsRestoresPromotedTargets(t *testing.T) {
	root := t.TempDir()
	target := filepath.Join(root, "pack", "README.md")
	backup := filepath.Join(root, "backup", "README.md")
	ignoredTarget := filepath.Join(root, "pack", "ignored.md")
	ignoredBackup := filepath.Join(root, "backup", "ignored.md")
	writeText(t, target, "new text")
	writeText(t, backup, "old text")
	writeText(t, ignoredTarget, "keep new")
	writeText(t, ignoredBackup, "keep old")

	err := restorePromoteBackups([]ApplyWrite{
		{Action: "promote", TargetPath: target, BackupPath: backup},
		{Action: "would-promote", TargetPath: ignoredTarget, BackupPath: ignoredBackup},
	})
	if err != nil {
		t.Fatal(err)
	}
	if got := readText(t, target); got != "old text" {
		t.Fatalf("restored target = %q, want old text", got)
	}
	if got := readText(t, ignoredTarget); got != "keep new" {
		t.Fatalf("ignored target = %q, want keep new", got)
	}
}

func TestApplyWhatIfDoesNotWritePack(t *testing.T) {
	repoRoot, caseRoot, pack := promoteApplyFixture(t, "16384", "# README\n\nReusable guidance.\n")
	readmePath := filepath.Join(repoRoot, "packs", pack, filepath.FromSlash("references/template/README.md"))
	before := readText(t, readmePath)

	result, err := Apply(repoRoot, caseRoot, pack, ApplyOptions{WhatIf: true})
	if err != nil {
		t.Fatal(err)
	}
	if result.Command != "promote" || result.IsMutation || result.Applied || result.Changed != 1 || result.Blocked != 1 || result.BackupRoot != "" || result.RequiresCleanup {
		t.Fatalf("unexpected promote apply what-if result: %+v", result)
	}
	write := assertApplyWriteForTest(t, result.Writes, "references/template/README.md", "would-promote")
	if write.BackupPath == "" {
		t.Fatal("what-if promote write missing preview backup path")
	}
	assertNotExists(t, write.BackupPath)
	assertApplyWriteForTest(t, result.Writes, "references/template/workflow-template.md", "blocked-deny-pattern")
	if got := readText(t, readmePath); got != before {
		t.Fatalf("what-if changed pack README:\ngot:  %q\nwant: %q", got, before)
	}
}

func TestApplyWritesPackBackupAndValidationRows(t *testing.T) {
	repoRoot, caseRoot, pack := promoteApplyFixture(t, "16384", "# README\n\nReusable guidance.\n")
	readmePath := filepath.Join(repoRoot, "packs", pack, filepath.FromSlash("references/template/README.md"))
	workflowPath := filepath.Join(repoRoot, "packs", pack, filepath.FromSlash("references/template/workflow-template.md"))
	oldReadme := readText(t, readmePath)
	oldWorkflow := readText(t, workflowPath)

	result, err := Apply(repoRoot, caseRoot, pack, ApplyOptions{})
	if err != nil {
		t.Fatal(err)
	}
	if result.Command != "promote" || !result.IsMutation || !result.Applied || result.Changed != 1 || result.Blocked != 1 || !result.RequiresCleanup || len(result.ValidationRows) == 0 {
		t.Fatalf("unexpected promote apply result: %+v", result)
	}
	write := assertApplyWriteForTest(t, result.Writes, "references/template/README.md", "promote")
	assertApplyWriteForTest(t, result.Writes, "references/template/workflow-template.md", "blocked-deny-pattern")
	if got := readText(t, write.BackupPath); got != oldReadme {
		t.Fatalf("backup README = %q, want %q", got, oldReadme)
	}
	if got := readText(t, readmePath); !strings.Contains(got, "Reusable guidance") {
		t.Fatalf("pack README was not promoted: %q", got)
	}
	if got := readText(t, workflowPath); got != oldWorkflow {
		t.Fatalf("blocked workflow changed:\ngot:  %q\nwant: %q", got, oldWorkflow)
	}
}

func TestApplyValidationFailureRestoresPackSource(t *testing.T) {
	repoRoot, caseRoot, pack := promoteApplyFixture(t, "16", "# README\n\nThis reusable guidance is intentionally too long for the tiny budget.\n")
	readmePath := filepath.Join(repoRoot, "packs", pack, filepath.FromSlash("references/template/README.md"))
	oldReadme := readText(t, readmePath)

	result, err := Apply(repoRoot, caseRoot, pack, ApplyOptions{})
	if err == nil {
		t.Fatal("Apply returned nil error for validation failure")
	}
	if !strings.Contains(err.Error(), "pack files restored from backup") {
		t.Fatalf("error = %v, want restore message", err)
	}
	if result.Command != "promote" || !result.IsMutation || !result.Applied || result.Changed != 1 {
		t.Fatalf("unexpected failed apply result: %+v", result)
	}
	if got := readText(t, readmePath); got != oldReadme {
		t.Fatalf("pack README was not restored:\ngot:  %q\nwant: %q", got, oldReadme)
	}
}

func promoteFixture(t *testing.T) (repoRoot, caseRoot, pack string) {
	t.Helper()
	repoRoot = t.TempDir()
	caseRoot = filepath.Join(t.TempDir(), "targetalpha")
	pack = "unit-pack"
	packRoot := filepath.Join(repoRoot, "packs", pack)
	manifest := `schemaVersion: 1
name: unit-pack
version: 0.0.1
description: Unit test pack
maturity: template

managedFiles:
  - references/template/README.md
  - references/template/workflow-template.md
  - references/template/toolchain-router.md
templateFiles: []
localNeverOverwrite: []

promoteFiles:
  - references/template/README.md
  - references/template/workflow-template.md
  - references/template/toolchain-router.md

commonPolicies: []
policyOverlays: []
subagentRoutes: []
toolingFiles: []
promptFiles: []

toolingCandidateSources:
  - references/template/toolchain-router.md

heavyToolGates:
  - id: debug
    title: Dynamic debug or attach
    sideEffects: debug,filesystem-write
    defaultRisk: high
    requiresConfirmation: true
    stopConditions: timeout,unexpected side effect,scope drift

promoteDenyPatterns:
  - "C:\\"
  - "artifacts[\\/]"
  - "captures[\\/]"
  - "[A-Za-z0-9_.-]*trace[A-Za-z0-9_.-]*\\.(csv|jsonl|log|txt|bin)"
  - "[A-Za-z0-9_.-]*dump[A-Za-z0-9_.-]*\\.(dmp|bin|raw|exe|dll)"
  - "\\.dmp\\b"
  - "0x[0-9A-Fa-f]{6,}"
  - "ctx[0-9]+"
  - "round[0-9]+"
  - "Task #[0-9]+"
`
	writeText(t, filepath.Join(packRoot, "manifest.yml"), manifest)
	writeText(t, filepath.Join(packRoot, filepath.FromSlash("references/template/README.md")), "# README\n\nOld pack text.\n")
	writeText(t, filepath.Join(packRoot, filepath.FromSlash("references/template/workflow-template.md")), "# Workflow\n\nOld pack text.\n")
	writeText(t, filepath.Join(packRoot, filepath.FromSlash("references/template/toolchain-router.md")), "# Tooling\n\nOld pack text.\n")

	metadata := "templateRoot: " + repoRoot + "\n" +
		"templatePack: " + pack + "\n" +
		"projectName: demo\n" +
		"projectRoot: " + caseRoot + "\n"
	writeText(t, filepath.Join(caseRoot, ".rekit", "instance.yml"), metadata)
	writeText(t, filepath.Join(caseRoot, filepath.FromSlash("references/template/README.md")), "# README\n\nReusable package-test candidate.\n")
	writeText(t, filepath.Join(caseRoot, filepath.FromSlash("references/template/workflow-template.md")), "# Workflow\n\nDo not promote C:\\case\\artifact\\sample-trace.csv from this case.\n")
	writeText(t, filepath.Join(caseRoot, filepath.FromSlash("references/template/toolchain-router.md")), "# Tooling\n\nCase root: "+caseRoot+"\nAbsolute: C:\\cases\\targetalpha\\sample.exe\nArtifacts: artifacts/run/demo-trace.csv\nCaptures: captures/run/demo-dump.bin\nAddress: 0x401000\nContext: ctx123 round7 Task #99\n")
	return repoRoot, caseRoot, pack
}

func promoteApplyFixture(t *testing.T, defaultBudget, caseReadme string) (repoRoot, caseRoot, pack string) {
	t.Helper()
	repoRoot = t.TempDir()
	caseRoot = filepath.Join(t.TempDir(), "applycase")
	pack = "unit-pack"
	packRoot := filepath.Join(repoRoot, "packs", pack)
	manifest := `schemaVersion: 1
name: unit-pack
version: 0.0.1
description: Unit test pack
maturity: template

managedFiles:
  - references/template/README.md
  - references/template/workflow-template.md
templateFiles: []
localNeverOverwrite: []

promoteFiles:
  - references/template/README.md
  - references/template/workflow-template.md

managedBlock:
  file: CLAUDE.local.md
  blockId: unit:router
  source: CLAUDE.local.snippet.md

syncPolicy:
  managedFiles: overwrite-with-backup
  templateFiles: create-if-missing
  localFiles: never-overwrite

workstreamDefaults:
  defaultAuthorityLane: main
  defaultStartLaneType: feature
  backupRoot: .rekit/backups/sync
  requestDefaultTargetLane: main

authorityFiles:
  - references/template/task-handoff.md

laneTypes:
  - id: main
    title: Main
    authority: true
    workspaceRoot: workspace/main
    canWrite: references/template/task-handoff.md
    readOnly: .rekit/facts/**
    outputs: publication,decision,observation
  - id: feature
    title: Feature
    authority: false
    workspaceRoot: workspace/features
    canWrite: own-workspace
    readOnly: references/template/**,.rekit/facts/**
    outputs: observation,request,candidate,summary

commonPolicies:
  - agent-team
policyOverlays: []
subagentRoutes: []
toolingFiles: []
promptFiles: []

toolingCandidateSources:
  - references/template/README.md

heavyToolGates:
  - id: debug
    title: Dynamic debug or attach
    sideEffects: debug,filesystem-write
    defaultRisk: high
    requiresConfirmation: true
    stopConditions: timeout,unexpected side effect,scope drift

promoteDenyPatterns:
  - "C:\\"
  - "artifacts[\\/]"
  - "captures[\\/]"
  - "[A-Za-z0-9_.-]*trace[A-Za-z0-9_.-]*\\.(csv|jsonl|log|txt|bin)"
  - "[A-Za-z0-9_.-]*dump[A-Za-z0-9_.-]*\\.(dmp|bin|raw|exe|dll)"
  - "\\.dmp\\b"
  - "0x[0-9A-Fa-f]{6,}"
  - "ctx[0-9]+"
  - "round[0-9]+"
  - "Task #[0-9]+"

budgets:
  defaultMarkdown: ` + defaultBudget + `
`
	writeText(t, filepath.Join(packRoot, "manifest.yml"), manifest)
	writeText(t, filepath.Join(repoRoot, ".claude", "skills", "rekit", "SKILL.md"), "# skill\n")
	writeText(t, filepath.Join(repoRoot, "rekit", "templates", "case-shim", "SKILL.md"), "# shim\n")
	writeText(t, filepath.Join(repoRoot, "common", "policies", "manifest.yml"), "policies:\n  - id: agent-team\n    path: agent-team.md\n")
	writeText(t, filepath.Join(repoRoot, "common", "policies", "README.md"), "# policies\n")
	writeText(t, filepath.Join(repoRoot, "common", "policies", "agent-team.md"), "# agent team\n")
	writeText(t, filepath.Join(packRoot, "policies", "manifest.yml"), "overlays: []\n")
	writeText(t, filepath.Join(packRoot, "policies", "README.md"), "# overlays\n")
	writeText(t, filepath.Join(packRoot, "CLAUDE.local.snippet.md"), "<!-- BEGIN unit:router -->\nunit router\n<!-- END unit:router -->\n")
	writeText(t, filepath.Join(packRoot, filepath.FromSlash("references/template/README.md")), "# README\n\nOld pack text.\n")
	writeText(t, filepath.Join(packRoot, filepath.FromSlash("references/template/workflow-template.md")), "# Workflow\n\nOld pack text.\n")
	writeText(t, filepath.Join(packRoot, filepath.FromSlash("references/template/task-handoff.md")), "# Handoff\n")

	metadata := "templateRoot: " + repoRoot + "\n" +
		"templatePack: " + pack + "\n" +
		"projectName: demo\n" +
		"projectRoot: " + caseRoot + "\n"
	writeText(t, filepath.Join(caseRoot, ".rekit", "instance.yml"), metadata)
	writeText(t, filepath.Join(caseRoot, filepath.FromSlash("references/template/README.md")), caseReadme)
	writeText(t, filepath.Join(caseRoot, filepath.FromSlash("references/template/workflow-template.md")), "# Workflow\n\nDo not promote C:\\case\\artifact\\sample-trace.csv.\n")
	return repoRoot, caseRoot, pack
}

func assertApplyWriteForTest(t *testing.T, writes []ApplyWrite, path, action string) ApplyWrite {
	t.Helper()
	for _, write := range writes {
		if write.Path == path && write.Action == action {
			return write
		}
	}
	t.Fatalf("promote apply write %s/%s not found in %+v", path, action, writes)
	return ApplyWrite{}
}

func assertCandidateWriteForTest(t *testing.T, writes []CandidateWrite, path, action string) CandidateWrite {
	t.Helper()
	for _, write := range writes {
		if write.Path == path && write.Action == action {
			if write.TargetPath == "" && !strings.HasPrefix(action, "blocked") {
				t.Fatalf("candidate write %s/%s missing target path", path, action)
			}
			return write
		}
	}
	t.Fatalf("candidate write %s/%s not found in %+v", path, action, writes)
	return CandidateWrite{}
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

func readText(t *testing.T, path string) string {
	t.Helper()
	b, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	return string(b)
}

func assertExists(t *testing.T, path string) {
	t.Helper()
	if _, err := os.Stat(path); err != nil {
		t.Fatalf("expected %s to exist: %v", path, err)
	}
}

func assertNotExists(t *testing.T, path string) {
	t.Helper()
	if _, err := os.Stat(path); !os.IsNotExist(err) {
		t.Fatalf("expected %s to be absent, stat err=%v", path, err)
	}
}

func assertTreeEmptyOrMissing(t *testing.T, root string) {
	t.Helper()
	entries, err := os.ReadDir(root)
	if os.IsNotExist(err) {
		return
	}
	if err != nil {
		t.Fatal(err)
	}
	if len(entries) != 0 {
		t.Fatalf("expected %s to be empty, got %d entries", root, len(entries))
	}
}
