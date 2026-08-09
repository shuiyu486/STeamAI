package adapterhost

import (
	"bytes"
	"encoding/json"
	"errors"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/shuiyu486/re-context-kits/internal/rekit/gate"
	"github.com/shuiyu486/re-context-kits/internal/rekit/mission"
	"github.com/shuiyu486/re-context-kits/internal/rekit/note"
)

type hostFixture struct {
	repoRoot   string
	caseRoot   string
	authorized gate.ApplyResult
	dispatch   gate.AdapterExecutionDispatchResult
	options    Options
}

func newHostFixture(t *testing.T, target string, runtimeSeconds, diskMB, requests int) hostFixture {
	t.Helper()
	root := t.TempDir()
	repoRoot := filepath.Join(root, "repo")
	caseRoot := filepath.Join(root, "case")
	writeHostFile(t, filepath.Join(repoRoot, "packs", "_template", "manifest.yml"), `schemaVersion: 1
name: _template
version: 0.1.0
description: adapter host fixture
maturity: template
managedFiles: []
templateFiles: []
localNeverOverwrite: []
managedBlock:
  file: CLAUDE.local.md
  blockId: template-pack:router
  source: CLAUDE.local.snippet.md
syncPolicy:
  managedFiles: overwrite-with-backup
  templateFiles: create-if-missing
  localFiles: never-overwrite
workstreamDefaults:
  defaultAuthorityLane: main
  defaultStartLaneType: feature
  handoffPath: handoff.md
  backupRoot: .rekit/backups
  requestDefaultTargetLane: main
authorityFiles:
  - handoff.md
commonPolicies: []
policyOverlays: []
subagentRoutes: []
promoteFiles: []
toolingFiles:
  - tooling/catalog.yml
promptFiles: []
laneTypes:
  - id: main
    title: Main
    authority: false
    workspaceRoot: workspace/main
    canWrite: own-workspace
    readOnly: .rekit/facts/**
    outputs: observation
toolingCandidateSources: []
heavyToolGates:
  - id: inspect
    title: Bounded read-only fixture inspection
    sideEffects: inspect,filesystem-write
    defaultRisk: medium
    requiresConfirmation: true
    stopConditions: timeout,scope-drift,budget-exhausted
promoteDenyPatterns: []
budgets:
  defaultMarkdown: 16384
`)
	writeHostFile(t, filepath.Join(repoRoot, "packs", "_template", "tooling", "catalog.yml"), `schemaVersion: 1
pack: _template
purpose: adapter host fixture

tools:
  - id: rekit-readonly-inspector
    status: mainline
    entry: go-owned rekit-adapter-host
    purpose: Inspect one bounded case-local text fixture.
    sideEffects: inspect,filesystem-write
`)
	writeHostFile(t, filepath.Join(caseRoot, ".rekit", "instance.yml"), "templateRoot: \""+repoRoot+"\"\ntemplatePack: \"_template\"\nprojectName: \"adapter-host-fixture\"\nprojectRoot: \""+caseRoot+"\"\n")
	writeHostFile(t, filepath.Join(caseRoot, ".rekit", "board.json"), `{"lanes":[{"id":"main","status":"open"}]}`)
	writeHostFile(t, filepath.Join(caseRoot, ".rekit", "lanes", "main", "lane.json"), `{
  "schemaVersion": 1,
  "id": "main",
  "type": "main",
  "name": "main",
  "title": "Main",
  "status": "open",
  "authority": false,
  "workspace": "workspace/main",
  "laneRoot": ".rekit/lanes/main",
  "canWrite": ["own-workspace"],
  "readOnly": [".rekit/facts/**"],
  "outputs": ["observation"],
  "counters": {},
  "currentExecutor": "executor-a",
  "executorGeneration": 1,
  "createdAt": "2026-08-08T00:00:00Z",
  "updatedAt": "2026-08-08T00:00:00Z"
}`)
	writeHostFile(t, filepath.Join(caseRoot, ".rekit", "lanes", "main", "autonomy.json"), `{
  "schemaVersion": 1,
  "profileId": "prof-main-inspect",
  "lane": "main",
  "mode": "preauthorized",
  "allowedActions": ["inspect"],
  "deniedActions": [],
  "targetScope": [{"match":"exact","value":"`+target+`"}],
  "budget": {"runtimeSeconds": 10,"diskMB": 4,"requests": 1},
  "stopConditions": ["timeout","scope-drift","budget-exhausted"],
  "outputPaths": ["workspace/main/inspect"],
  "recordRequired": true,
  "notifyMainOn": ["boundary-hit","new-risk"],
  "grantedBy": "user",
  "grantedAt": "2026-08-08T00:00:00Z",
  "expiresAt": "2999-01-01T00:00:00Z"
}`)
	writeHostFile(t, filepath.Join(caseRoot, "fixture", "input.txt"), "alpha\nbeta\n")
	authorized, err := gate.Apply(repoRoot, caseRoot, "_template", gate.Options{
		Action: "inspect", Lane: "main", Actor: "host-test", Subject: "inspect fixture",
		TargetRef: target, RuntimeSeconds: runtimeSeconds, DiskMB: diskMB, Requests: requests,
		OutputPaths: "workspace/main/inspect/session-1", StopConditions: "timeout,scope-drift,budget-exhausted",
	})
	if err != nil || !authorized.Applied || authorized.Event == nil {
		t.Fatalf("authorize inspect: %+v err=%v", authorized, err)
	}
	dispatchOpt := gate.Options{
		GateEventID:         authorized.EventID,
		ExecutionReportPath: "workspace/main/inspect/session-1/adapter-report.json",
		AdapterID:           readonlyInspectorID,
		Executor:            "executor-a", ExpectedExecutorGeneration: 1,
		AdapterHarness: adapterHarness, AdapterSession: "adapter-session-a", Actor: "mission-commander",
	}
	preview, err := gate.RecordAdapterExecutionDispatch(repoRoot, caseRoot, "_template", dispatchOpt)
	if err != nil {
		t.Fatal(err)
	}
	dispatchOpt.ExpectedAdapterExecutionDispatchBindingSHA256 = preview.BindingSHA256
	dispatch, err := gate.RecordAdapterExecutionDispatch(repoRoot, caseRoot, "_template", dispatchOpt)
	if err != nil || !dispatch.Applied || dispatch.DispatchSHA256 == "" {
		t.Fatalf("record dispatch: %+v err=%v", dispatch, err)
	}
	return hostFixture{
		repoRoot: repoRoot, caseRoot: caseRoot, authorized: authorized, dispatch: dispatch,
		options: Options{RepoRoot: repoRoot, CaseRoot: caseRoot, Pack: "_template", GateEventID: authorized.EventID, ExpectedDispatchSHA256: dispatch.DispatchSHA256},
	}
}

func TestRunPublishesProcessGeneratedReadOnlyReport(t *testing.T) {
	fixture := newHostFixture(t, "fixture/input.txt", 10, 4, 1)
	before, err := os.ReadFile(filepath.Join(fixture.caseRoot, "fixture", "input.txt"))
	if err != nil {
		t.Fatal(err)
	}
	binary := filepath.Join(t.TempDir(), "rekit-adapter-host")
	if filepath.Separator == '\\' {
		binary += ".exe"
	}
	if err := exec.Command("go", "build", "-o", binary, "../../../cmd/rekit-adapter-host").Run(); err != nil {
		t.Fatal(err)
	}
	cmd := exec.Command(binary,
		"-repo", fixture.repoRoot,
		"-target", fixture.caseRoot,
		"-pack", "_template",
		"-gate-event-id", fixture.authorized.EventID,
		"-expected-dispatch-sha256", fixture.dispatch.DispatchSHA256,
	)
	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr
	if err := cmd.Run(); err != nil {
		t.Fatalf("adapter child failed: %v stderr=%s", err, stderr.String())
	}
	var result Result
	if err := json.Unmarshal(stdout.Bytes(), &result); err != nil {
		t.Fatalf("decode adapter child result: %v stdout=%s", err, stdout.String())
	}
	after, err := os.ReadFile(filepath.Join(fixture.caseRoot, "fixture", "input.txt"))
	if err != nil || string(after) != string(before) {
		t.Fatalf("read-only input changed: err=%v before=%q after=%q", err, before, after)
	}
	if result.ProcessID <= 0 || result.ProcessID == os.Getpid() || result.InputSHA256 == "" || result.ArtifactSHA256 == "" || result.ReportSHA256 == "" || !result.ReadOnlyInput || !result.NoNetwork || !result.NoAuthority {
		t.Fatalf("host result omitted process or boundary provenance: %+v", result)
	}
	validation, err := gate.ValidateAdapterExecutionReport(fixture.repoRoot, fixture.caseRoot, "_template", gate.Options{
		GateEventID:         fixture.authorized.EventID,
		ExecutionReportPath: result.ReportPath,
	})
	if err != nil {
		t.Fatal(err)
	}
	if validation.Valid || validation.FailureCode != "adapter-execution-receipt-invalid" || !validation.ReceiptRequired || validation.ReceiptPresent {
		t.Fatalf("report should be valid through dispatch but blocked pending receipt: %+v", validation)
	}
	receiptOpt := gate.Options{
		GateEventID:                fixture.authorized.EventID,
		ExecutionReportPath:        result.ReportPath,
		AdapterID:                  readonlyInspectorID,
		Executor:                   "executor-a",
		ExpectedExecutorGeneration: 1,
		AdapterHarness:             adapterHarness,
		AdapterSession:             result.AdapterSession,
		ExecutionExitStatus:        "completed",
		Actor:                      "mission-commander",
	}
	receiptPreview, err := gate.RecordAdapterExecutionReceipt(
		fixture.repoRoot,
		fixture.caseRoot,
		"_template",
		receiptOpt,
	)
	if err != nil || receiptPreview.BindingSHA256 == "" || receiptPreview.Applied {
		t.Fatalf("preview adapter receipt: %+v err=%v", receiptPreview, err)
	}
	receiptOpt.ExpectedAdapterExecutionBindingSHA256 = receiptPreview.BindingSHA256
	receipt, err := gate.RecordAdapterExecutionReceipt(
		fixture.repoRoot,
		fixture.caseRoot,
		"_template",
		receiptOpt,
	)
	if err != nil || !receipt.Applied || receipt.ReceiptSHA256 == "" {
		t.Fatalf("record adapter receipt: %+v err=%v", receipt, err)
	}
	validation, err = gate.ValidateAdapterExecutionReport(
		fixture.repoRoot,
		fixture.caseRoot,
		"_template",
		gate.Options{
			GateEventID:         fixture.authorized.EventID,
			ExecutionReportPath: result.ReportPath,
		},
	)
	if err != nil || !validation.Valid || !validation.ProvenanceValid || !validation.ReceiptPresent || validation.AdapterExecutionReceiptSHA256 != receipt.ReceiptSHA256 {
		t.Fatalf("receipt-backed validation should be record-ready: %+v err=%v", validation, err)
	}
	observation, err := gate.RecordExecution(
		fixture.repoRoot,
		fixture.caseRoot,
		"_template",
		gate.Options{
			GateEventID:                           fixture.authorized.EventID,
			Actor:                                 "mission-commander",
			ExecutionReportPath:                   validation.ReportPath,
			ExpectedExecutionReportSHA256:         validation.RecordExpectedReportSHA256,
			AdapterExecutionReceiptPath:           validation.AdapterExecutionReceiptPath,
			ExpectedAdapterExecutionReceiptSHA256: validation.AdapterExecutionReceiptSHA256,
			Executor:                              "executor-a", ExpectedExecutorGeneration: 1,
		},
	)
	if err != nil || !observation.Applied || observation.ExecutionEvidence == nil || observation.MissionCommanderDriverReceipt == nil || observation.MissionCommanderDriverReceipt.State != "refreshed" {
		t.Fatalf("record adapter observation: %+v err=%v", observation, err)
	}
	recordedReceipt, receiptPath, receiptSHA, present, err := gate.ReadAdapterExecutionReceipt(
		fixture.caseRoot,
		"main",
		fixture.authorized.EventID,
	)
	if err != nil || !present || recordedReceipt == nil || receiptPath != receipt.ReceiptPath || receiptSHA != receipt.ReceiptSHA256 {
		t.Fatalf("observation acknowledgement omitted receipt provenance: receipt=%+v path=%q sha=%q present=%t err=%v", recordedReceipt, receiptPath, receiptSHA, present, err)
	}
	facts, err := mission.ReadStrictLedgerFacts(fixture.caseRoot)
	if err != nil {
		t.Fatal(err)
	}
	review := mission.ExecutionEvidenceReviewItemsWithLedgerFacts(facts, "main", nil, 0)
	if len(review) != 1 || review[0].Acknowledgement == nil {
		t.Fatalf("observation should return to evidence review: %+v", review)
	}
	ackOpt := note.Options{
		Kind: "verification", Lane: "main",
		Subject:  "execution evidence review accepted",
		Summary:  "accepted recorded execution evidence for gateEventId " + fixture.authorized.EventID,
		Verifier: "manual-review", Verdict: "accepted", Status: "resolved",
		Related:      strings.Join([]string{review[0].EventID, review[0].GateEventID}, ","),
		Reason:       "reviewed outputRefs/evidenceRefs before closing execution evidence review",
		Target:       result.InputPath,
		EvidenceRefs: strings.Join([]string{result.ArtifactPath, result.ReportPath}, ","),
		CreatedAt:    "2026-08-08T00:00:00Z",
	}
	ackPreview, err := note.Append(fixture.repoRoot, fixture.caseRoot, "_template", ackOpt, true)
	if err != nil || ackPreview.EventSHA256 == "" || ackPreview.Applied {
		t.Fatalf("preview evidence acknowledgement: %+v err=%v", ackPreview, err)
	}
	ackOpt.ExpectedEventSHA256 = ackPreview.EventSHA256
	acknowledged, err := note.Append(fixture.repoRoot, fixture.caseRoot, "_template", ackOpt, false)
	if err != nil || !acknowledged.Applied {
		t.Fatalf("record evidence acknowledgement: %+v err=%v", acknowledged, err)
	}
	facts, err = mission.ReadStrictLedgerFacts(fixture.caseRoot)
	if err != nil {
		t.Fatal(err)
	}
	if pending := mission.ExecutionEvidenceReviewItemsWithLedgerFacts(facts, "main", nil, 0); len(pending) != 0 {
		t.Fatalf("acknowledged execution evidence should resume Mission Commander routing: %+v", pending)
	}
	assertHostFileMissing(t, filepath.Join(fixture.caseRoot, ".rekit", "facts", "authority.jsonl"))
	assertHostFileMissing(t, filepath.Join(fixture.caseRoot, ".rekit", "facts", "confirmed.jsonl"))
}

func TestRunRejectsStaleOwnerBeforeOutput(t *testing.T) {
	fixture := newHostFixture(t, "fixture/input.txt", 10, 4, 1)
	lanePath := filepath.Join(fixture.caseRoot, ".rekit", "lanes", "main", "lane.json")
	data, err := os.ReadFile(lanePath)
	if err != nil {
		t.Fatal(err)
	}
	data = []byte(strings.Replace(string(data), `"executorGeneration": 1`, `"executorGeneration": 2`, 1))
	if err := os.WriteFile(lanePath, data, 0o600); err != nil {
		t.Fatal(err)
	}
	if _, err := Run(fixture.options); err == nil || !strings.Contains(err.Error(), "owner is stale") {
		t.Fatalf("stale owner should fail closed, err=%v", err)
	}
	assertHostFileMissing(t, filepath.Join(fixture.caseRoot, "workspace", "main", "inspect", "session-1", "adapter-report.json"))
}

func TestRunRejectsWrongDispatchAndLateReplay(t *testing.T) {
	fixture := newHostFixture(t, "fixture/input.txt", 10, 4, 1)
	wrong := fixture.options
	wrong.ExpectedDispatchSHA256 = strings.Repeat("a", 64)
	if _, err := Run(wrong); err == nil || !strings.Contains(err.Error(), "dispatch sha256 changed") {
		t.Fatalf("wrong dispatch hash should fail closed, err=%v", err)
	}
	if _, err := Run(fixture.options); err != nil {
		t.Fatal(err)
	}
	if _, err := Run(fixture.options); err == nil || !strings.Contains(err.Error(), "existing execution report") {
		t.Fatalf("late replay should refuse existing process output, err=%v", err)
	}
}

func TestRunRejectsInputOutputAlias(t *testing.T) {
	fixture := newHostFixture(t, "workspace/main/inspect/session-1/adapter-report.json", 10, 4, 1)
	if _, err := Run(fixture.options); err == nil || !strings.Contains(err.Error(), "invalid input or output path") {
		t.Fatalf("input/output alias should fail closed, err=%v", err)
	}
}

func TestRunRejectsCatalogDriftBeforeOutput(t *testing.T) {
	fixture := newHostFixture(t, "fixture/input.txt", 10, 4, 1)
	catalogPath := filepath.Join(fixture.repoRoot, "packs", "_template", "tooling", "catalog.yml")
	data, err := os.ReadFile(catalogPath)
	if err != nil {
		t.Fatal(err)
	}
	data = append(data, []byte("\n# drift after dispatch\n")...)
	if err := os.WriteFile(catalogPath, data, 0o600); err != nil {
		t.Fatal(err)
	}
	if _, err := Run(fixture.options); err == nil || !strings.Contains(err.Error(), "catalog") {
		t.Fatalf("catalog drift should fail closed, err=%v", err)
	}
	assertHostFileMissing(t, filepath.Join(fixture.caseRoot, "workspace", "main", "inspect", "session-1", "adapter-report.json"))
}

func TestRunRejectsSymlinkInput(t *testing.T) {
	fixture := newHostFixture(t, "fixture/input.txt", 10, 4, 1)
	inputPath := filepath.Join(fixture.caseRoot, "fixture", "input.txt")
	linkTarget := filepath.Join(fixture.caseRoot, "fixture", "link-target.txt")
	if err := os.Rename(inputPath, linkTarget); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink(linkTarget, inputPath); err != nil {
		if os.IsPermission(err) {
			t.Skipf("symlink creation is unavailable: %v", err)
		}
		t.Fatal(err)
	}
	if _, err := Run(fixture.options); err == nil || !strings.Contains(err.Error(), "non-symlink") {
		t.Fatalf("symlink input should fail closed, err=%v", err)
	}
	assertHostFileMissing(t, filepath.Join(fixture.caseRoot, "workspace", "main", "inspect", "session-1", "adapter-report.json"))
}

func TestRunCleansPartialPublication(t *testing.T) {
	fixture := newHostFixture(t, "fixture/input.txt", 10, 4, 1)
	fixture.options.testHooks = &hostTestHooks{beforeReportWrite: func() error {
		return os.ErrPermission
	}}
	if _, err := Run(fixture.options); !errors.Is(err, os.ErrPermission) {
		t.Fatalf("injected report write failure should surface, err=%v", err)
	}
	outputRoot := filepath.Join(fixture.caseRoot, "workspace", "main", "inspect", "session-1")
	assertHostFileMissing(t, filepath.Join(outputRoot, "inspection.json"))
	assertHostFileMissing(t, filepath.Join(outputRoot, "adapter-report.json"))
}

func TestRunRejectsInputChangeAfterArtifactPublication(t *testing.T) {
	fixture := newHostFixture(t, "fixture/input.txt", 10, 4, 1)
	fixture.options.testHooks = &hostTestHooks{beforeReportWrite: func() error {
		return os.WriteFile(filepath.Join(fixture.caseRoot, "fixture", "input.txt"), []byte("changed\n"), 0o600)
	}}
	if _, err := Run(fixture.options); err == nil || !strings.Contains(err.Error(), "input changed") {
		t.Fatalf("post-artifact input mutation should fail closed, err=%v", err)
	}
	outputRoot := filepath.Join(fixture.caseRoot, "workspace", "main", "inspect", "session-1")
	assertHostFileMissing(t, filepath.Join(outputRoot, "inspection.json"))
	assertHostFileMissing(t, filepath.Join(outputRoot, "adapter-report.json"))
}

func TestRunDoesNotDeleteCompetingReport(t *testing.T) {
	fixture := newHostFixture(t, "fixture/input.txt", 10, 4, 1)
	reportPath := filepath.Join(fixture.caseRoot, "workspace", "main", "inspect", "session-1", "adapter-report.json")
	competing := []byte("competing report\n")
	fixture.options.testHooks = &hostTestHooks{beforeReportWrite: func() error {
		return os.WriteFile(reportPath, competing, 0o600)
	}}
	if _, err := Run(fixture.options); err == nil || !strings.Contains(strings.ToLower(err.Error()), "exist") {
		t.Fatalf("competing report should make exclusive publication fail, err=%v", err)
	}
	if got, err := os.ReadFile(reportPath); err != nil || !bytes.Equal(got, competing) {
		t.Fatalf("host removed or changed a competing report: got=%q err=%v", got, err)
	}
	assertHostFileMissing(t, filepath.Join(filepath.Dir(reportPath), "inspection.json"))
}

func TestRunRejectsRuntimeBudgetOverrun(t *testing.T) {
	fixture := newHostFixture(t, "fixture/input.txt", 1, 4, 1)
	fixture.options.testHooks = &hostTestHooks{beforeReportWrite: func() error {
		time.Sleep(1100 * time.Millisecond)
		return nil
	}}
	if _, err := Run(fixture.options); err == nil || !strings.Contains(err.Error(), "runtime exceeded") {
		t.Fatalf("runtime overrun should fail closed, err=%v", err)
	}
	outputRoot := filepath.Join(fixture.caseRoot, "workspace", "main", "inspect", "session-1")
	assertHostFileMissing(t, filepath.Join(outputRoot, "inspection.json"))
	assertHostFileMissing(t, filepath.Join(outputRoot, "adapter-report.json"))
}

func writeHostFile(t *testing.T, path, text string) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte(text), 0o600); err != nil {
		t.Fatal(err)
	}
}

func assertHostFileMissing(t *testing.T, path string) {
	t.Helper()
	if _, err := os.Lstat(path); !os.IsNotExist(err) {
		t.Fatalf("path should be missing: %s err=%v", path, err)
	}
}
