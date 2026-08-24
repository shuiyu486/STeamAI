package cli

import (
	"bytes"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/shuiyu486/re-context-kits/internal/rekit/gate"
)

func cliAdapterExecutionFixture(t *testing.T) (string, string, string) {
	t.Helper()
	root := t.TempDir()
	repoRoot := filepath.Join(root, "repo")
	caseRoot := filepath.Join(root, "case")
	pack := "unit"
	writeCaseFile(t, repoRoot, "go.mod", "module github.com/shuiyu486/re-context-kits\n")
	writeCaseFile(t, repoRoot, "rekit/rekit.ps1", "# fixture\n")
	writeCaseFile(t, repoRoot, "packs/unit/manifest.yml", `name: unit
managedFiles:
  - references/unit/README.md
toolingFiles:
  - tooling/catalog.yml
heavyToolGates:
  - id: debug
    title: Debug
    sideEffects: debug,filesystem-write
    defaultRisk: high
    requiresConfirmation: true
    stopConditions: timeout
`)
	writeCaseFile(t, repoRoot, "packs/unit/tooling/catalog.yml", `schemaVersion: 1
pack: unit
purpose: test

tools:
  - id: dynamic-debug-or-writeback-action
    status: supported
    entry: <caseRoot>/tools/debug
    purpose: Execute bounded debug.
    sideEffects: debug,filesystem-write
`)
	writeCaseFile(t, caseRoot, ".rekit/instance.yml", "templateRoot: \""+repoRoot+"\"\ntemplatePack: \"unit\"\nprojectName: \"adapter-provenance\"\nprojectRoot: \""+caseRoot+"\"\n")
	writeCaseFile(t, caseRoot, ".rekit/board.json", `{"lanes":[{"id":"main","currentExecutor":"executor-a","executorGeneration":1}]}`)
	writeCaseFile(t, caseRoot, ".rekit/lanes/main/autonomy.json", `{
  "schemaVersion": 1,
  "profileId": "prof-main-debug",
  "lane": "main",
  "mode": "preauthorized",
  "allowedActions": ["debug"],
  "deniedActions": [],
  "targetScope": [{"match":"exact","value":"target-alpha"}],
  "budget": {"runtimeSeconds": 60, "diskMB": 128, "requests": 2},
  "stopConditions": ["timeout"],
  "outputPaths": ["workspace/main/debug"],
  "recordRequired": true,
  "notifyMainOn": ["boundary-hit"],
  "grantedBy": "user",
  "grantedAt": "2026-01-01T00:00:00Z",
  "expiresAt": "2999-01-01T00:00:00Z"
}`)
	oldwd, err := os.Getwd()
	if err != nil {
		t.Fatal(err)
	}
	if err := os.Chdir(repoRoot); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		if err := os.Chdir(oldwd); err != nil {
			t.Fatal(err)
		}
	})
	return repoRoot, caseRoot, pack
}

func TestRunGateAdapterExecutionReceiptRejectsModeConflicts(t *testing.T) {
	_, caseRoot, pack := cliAdapterExecutionFixture(t)
	base := []string{"-Command", "gate", "-Target", caseRoot, "-Pack", pack, "-GateEventId", "evt-test", "-RecordAdapterExecutionReceipt", "-ExecutionReportPath", "workspace/main/debug/adapter-report.json", "-Executor", "executor-a", "-ExpectedExecutorGeneration", "1", "-AdapterHarness", "harness", "-AdapterSession", "session", "-ExecutionExitStatus", "0", "-Actor", "recorder"}
	for _, test := range []struct {
		name string
		args []string
		want string
	}{
		{name: "execution status", args: []string{"-ExecutionStatus", "succeeded"}, want: "cannot be combined"},
		{name: "actual budget", args: []string{"-ActualRuntimeSeconds", "1"}, want: "cannot be combined"},
		{name: "output refs", args: []string{"-OutputRefs", "workspace/main/debug/result.bin"}, want: "cannot be combined"},
		{name: "evidence refs", args: []string{"-ExecutionEvidenceRefs", "workspace/main/debug/evidence.json"}, want: "cannot be combined"},
		{name: "boundary hits", args: []string{"-BoundaryHits", "timeout"}, want: "cannot be combined"},
		{name: "escalation", args: []string{"-Escalation", "stop"}, want: "cannot be combined"},
		{name: "report hash", args: []string{"-ExpectedExecutionReportSha256", strings.Repeat("a", 64)}, want: "cannot be combined"},
		{name: "receipt path", args: []string{"-AdapterExecutionReceiptPath", ".rekit/receipt.json"}, want: "cannot be combined"},
		{name: "receipt hash", args: []string{"-ExpectedAdapterExecutionReceiptSha256", strings.Repeat("b", 64)}, want: "cannot be combined"},
		{name: "summary", args: []string{"-Summary", "different execution"}, want: "cannot be combined"},
		{name: "subject", args: []string{"-Subject", "different subject"}, want: "cannot be combined"},
		{name: "risk", args: []string{"-Risk", "high"}, want: "cannot be combined"},
		{name: "target ref", args: []string{"-TargetRef", "target-beta"}, want: "cannot be combined"},
		{name: "batch id", args: []string{"-BatchId", "batch-other"}, want: "cannot be combined"},
		{name: "scope", args: []string{"-Scope", "different scope"}, want: "cannot be combined"},
		{name: "authorized budget", args: []string{"-RuntimeSeconds", "5"}, want: "cannot be combined"},
		{name: "output paths", args: []string{"-OutputPaths", "workspace/main/other"}, want: "cannot be combined"},
		{name: "stop conditions", args: []string{"-StopConditions", "other"}, want: "cannot be combined"},
		{name: "actor consumes force", args: []string{"-Actor", "-Force"}, want: "cannot be combined"},
		{name: "exit status consumes whatif", args: []string{"-ExecutionExitStatus", "-WhatIf"}, want: "cannot be combined"},
	} {
		t.Run(test.name, func(t *testing.T) {
			var out bytes.Buffer
			err := Run(append(append([]string{}, base...), test.args...), &out)
			if err == nil || !strings.Contains(err.Error(), test.want) {
				t.Fatalf("receipt mode conflict error = %v", err)
			}
		})
	}

	for _, args := range [][]string{
		{"-Command", "gate", "-Target", caseRoot, "-Pack", pack, "-GateEventId", "evt-test", "-AdapterHarness", "harness", "-Apply"},
		{"-Command", "gate", "-Target", caseRoot, "-Pack", pack, "-GateEventId", "evt-test", "-AdapterSession", "session", "-Apply"},
		{"-Command", "gate", "-Target", caseRoot, "-Pack", pack, "-GateEventId", "evt-test", "-ExecutionExitStatus", "0", "-Apply"},
		{"-Command", "gate", "-Target", caseRoot, "-Pack", pack, "-GateEventId", "evt-test", "-ExpectedAdapterExecutionBindingSha256", strings.Repeat("c", 64), "-Apply"},
		{"-Command", "gate", "-Target", caseRoot, "-Pack", pack, "-GateEventId", "evt-test", "-ExpectedAdapterExecutionDispatchBindingSha256", strings.Repeat("d", 64), "-Apply"},
	} {
		var out bytes.Buffer
		if err := Run(args, &out); err == nil || !strings.Contains(err.Error(), "supported only with gate adapter execution dispatch or completion receipt mode") {
			t.Fatalf("adapter execution mode-only flag error = %v", err)
		}
	}
}

func TestRunGateAdapterExecutionReceiptProductPath(t *testing.T) {
	_, caseRoot, pack := cliAdapterExecutionFixture(t)
	writeCaseFile(t, caseRoot, ".rekit/lanes/main/lane.json", `{
  "schemaVersion": 1,
  "id": "main",
  "type": "main",
  "name": "main",
  "title": "Main",
  "status": "active",
  "authority": true,
  "workspace": "workspace/main",
  "laneRoot": ".rekit/lanes/main",
  "canWrite": [],
  "readOnly": [],
  "outputs": [],
  "counters": {},
  "currentExecutor": "executor-a",
  "executorGeneration": 1,
  "createdAt": "2026-07-29T00:00:00Z",
  "updatedAt": "2026-07-29T00:00:00Z"
}`)
	var out bytes.Buffer
	if err := Run([]string{"-Command", "gate", "-Target", caseRoot, "-Pack", pack, "-Action", "debug", "-Lane", "main", "-TargetRef", "target-alpha", "-RuntimeSeconds", "30", "-DiskMB", "64", "-Requests", "1", "-OutputPaths", "workspace/main/debug/session-1", "-StopConditions", "timeout", "-Actor", "gate-test", "-Apply", "-Format", "json"}, &out); err != nil {
		t.Fatal(err)
	}
	var authorized gate.ApplyResult
	if err := json.Unmarshal(out.Bytes(), &authorized); err != nil {
		t.Fatal(err)
	}
	if authorized.Event == nil || authorized.Event.Status != "authorized-gate" {
		t.Fatalf("unexpected gate apply: %+v", authorized)
	}
	workspace := filepath.Join(caseRoot, "workspace", "main", "debug", "session-1")
	if err := os.MkdirAll(workspace, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.Chdir(workspace); err != nil {
		t.Fatal(err)
	}
	dispatchArgs := []string{"-Command", "gate", "-Target", caseRoot, "-Pack", pack, "-GateEventId", authorized.EventID, "-RecordAdapterExecutionDispatch", "-ExecutionReportPath", "adapter-report.json", "-AdapterId", "dynamic-debug-or-writeback-action", "-Executor", "executor-a", "-ExpectedExecutorGeneration", "1", "-AdapterHarness", "claude-code", "-AdapterSession", "cli-session-a", "-Actor", "mission-commander", "-Format", "json"}
	out.Reset()
	if err := Run(dispatchArgs, &out); err != nil {
		t.Fatal(err)
	}
	var dispatchPreview gate.AdapterExecutionDispatchResult
	if err := json.Unmarshal(out.Bytes(), &dispatchPreview); err != nil {
		t.Fatal(err)
	}
	if dispatchPreview.Applied || dispatchPreview.BindingSHA256 == "" || dispatchPreview.ApplyCommand == "" {
		t.Fatalf("unexpected CLI dispatch preview: %+v", dispatchPreview)
	}
	if _, err := os.Stat(filepath.Join(caseRoot, filepath.FromSlash(dispatchPreview.DispatchPath))); !os.IsNotExist(err) {
		t.Fatalf("CLI preview wrote dispatch: %v", err)
	}
	out.Reset()
	dispatchApplyArgs := append(append([]string{}, dispatchArgs[:len(dispatchArgs)-2]...), "-ExpectedAdapterExecutionDispatchBindingSha256", dispatchPreview.BindingSHA256, "-Apply", "-Format", "json")
	if err := Run(dispatchApplyArgs, &out); err != nil {
		t.Fatal(err)
	}
	var dispatch gate.AdapterExecutionDispatchResult
	if err := json.Unmarshal(out.Bytes(), &dispatch); err != nil {
		t.Fatal(err)
	}
	if !dispatch.Applied || dispatch.Replay || dispatch.DispatchSHA256 == "" || dispatch.Dispatch.DispatchID == "" || dispatch.Dispatch.ReportPath != "workspace/main/debug/session-1/adapter-report.json" {
		t.Fatalf("unexpected nested-cwd CLI dispatch apply: %+v", dispatch)
	}

	writeCaseFile(t, caseRoot, "workspace/main/debug/session-1/result.bin", "cli-result")
	writeCaseFile(t, caseRoot, "workspace/main/debug/session-1/adapter-report.json", `{
  "schemaVersion": 1,
  "kind": "adapter-execution-report",
  "adapterId": "dynamic-debug-or-writeback-action",
  "action": "debug",
  "status": "succeeded",
  "gateEventId": "`+authorized.EventID+`",
  "dispatch": {"dispatchId": "`+dispatch.Dispatch.DispatchID+`", "path": "`+dispatch.DispatchPath+`", "sha256": "`+dispatch.DispatchSHA256+`"},
  "actualBudget": {"runtimeSeconds": 20, "diskMB": 24, "requests": 1},
  "outputRefs": ["workspace/main/debug/session-1/result.bin"],
  "summary": "CLI adapter execution completed"
}`)

	receiptArgs := []string{"-Command", "gate", "-Target", caseRoot, "-Pack", pack, "-GateEventId", authorized.EventID, "-RecordAdapterExecutionReceipt", "-ExecutionReportPath", "workspace/main/debug/session-1/adapter-report.json", "-AdapterId", "dynamic-debug-or-writeback-action", "-Executor", "executor-a", "-ExpectedExecutorGeneration", "1", "-AdapterHarness", "claude-code", "-AdapterSession", "cli-session-a", "-ExecutionExitStatus", "-1", "-Actor", "mission-commander", "-Format", "json"}
	out.Reset()
	if err := Run(receiptArgs, &out); err != nil {
		t.Fatal(err)
	}
	var preview gate.AdapterExecutionReceiptResult
	if err := json.Unmarshal(out.Bytes(), &preview); err != nil {
		t.Fatal(err)
	}
	if preview.Applied || preview.BindingSHA256 == "" || preview.ApplyCommand == "" {
		t.Fatalf("unexpected CLI receipt preview: %+v", preview)
	}
	if _, err := os.Stat(filepath.Join(caseRoot, filepath.FromSlash(preview.ReceiptPath))); !os.IsNotExist(err) {
		t.Fatalf("CLI preview wrote receipt: %v", err)
	}

	applyArgs := append(append([]string{}, receiptArgs[:len(receiptArgs)-2]...), "-ExpectedAdapterExecutionBindingSha256", preview.BindingSHA256, "-Apply", "-Format", "json")
	out.Reset()
	if err := Run(applyArgs, &out); err != nil {
		t.Fatal(err)
	}
	var receipt gate.AdapterExecutionReceiptResult
	if err := json.Unmarshal(out.Bytes(), &receipt); err != nil {
		t.Fatal(err)
	}
	if !receipt.Applied || receipt.Replay || receipt.ReceiptSHA256 == "" || receipt.Receipt.Execution.ExitStatus != "-1" {
		t.Fatalf("unexpected CLI receipt apply with dash-prefixed exit status: %+v", receipt)
	}
	if receipt.Receipt.Dispatch.DispatchID != dispatch.Dispatch.DispatchID || receipt.Receipt.Dispatch.Path != dispatch.DispatchPath || receipt.Receipt.Dispatch.SHA256 != dispatch.DispatchSHA256 {
		t.Fatalf("CLI completion receipt omitted immutable dispatch lineage: dispatch=%+v receipt=%+v", dispatch, receipt)
	}

	out.Reset()
	if err := Run([]string{"-Command", "gate", "-Target", caseRoot, "-Pack", pack, "-GateEventId", authorized.EventID, "-ValidateExecutionReport", "-ExecutionReportPath", "workspace/main/debug/session-1/adapter-report.json", "-Format", "json"}, &out); err != nil {
		t.Fatal(err)
	}
	var validation gate.AdapterExecutionReportValidation
	if err := json.Unmarshal(out.Bytes(), &validation); err != nil {
		t.Fatal(err)
	}
	if !validation.Valid || !validation.ProvenanceValid || validation.AdapterExecutionDispatch == nil || validation.AdapterExecutionDispatchPath != dispatch.DispatchPath || validation.AdapterExecutionDispatchSHA256 != dispatch.DispatchSHA256 || !strings.Contains(validation.MissionCommanderAction.PrimaryCommand, "-ExpectedAdapterExecutionReceiptSha256") {
		t.Fatalf("CLI validation omitted dispatch/completion-backed record command: %+v", validation)
	}
	validationReceipt := validation.MissionCommanderDriverReceipt
	validationDriver := validation.MissionCommanderActionQueue.CurrentDriverRequest
	if validationReceipt == nil || validationReceipt.SchemaVersion != 1 || validationReceipt.State != "refreshed" || validationReceipt.Outcome != "adapter-report-validation-valid-result" || validationReceipt.Lane != "main" || !strings.Contains(validationReceipt.Command, "-ValidateExecutionReport") || validationReceipt.RefreshedActionQueueSummary != validation.MissionCommanderActionQueue.Summary || validationReceipt.RefreshedCurrentRunLoopStep != validation.MissionCommanderActionQueue.CurrentRunLoopStepID || validationReceipt.RefreshedCurrentDriverRequest == nil || validationDriver == nil || validationReceipt.RefreshedCurrentDriverRequest.Command != validationDriver.Command || !strings.Contains(validationReceipt.RefreshedCurrentDriverRequest.Command, "-ExpectedExecutionReportSha256") || validationReceipt.RefreshedCurrentDriverRequest.ExpectedReceipt.RefreshStatusCommand == "" || !strings.Contains(validationReceipt.RefreshedCurrentDriverRequest.ExpectedReceipt.RefreshStatusCommand, caseRoot) || !containsSubstring(validationReceipt.Boundary, "validation is read-only") || !containsSubstring(validationReceipt.Boundary, "does not write authority/confirmed") {
		t.Fatalf("CLI validation omitted adapter validation run-loop receipt: receipt=%+v queue=%+v", validationReceipt, validation.MissionCommanderActionQueue)
	}
	out.Reset()
	if err := Run([]string{"-Command", "gate", "-Target", caseRoot, "-Pack", pack, "-GateEventId", authorized.EventID, "-ValidateExecutionReport", "-ExecutionReportPath", "workspace/main/debug/session-1/adapter-report.json", "-Format", "text"}, &out); err != nil {
		t.Fatal(err)
	}
	for _, expected := range []string{"gate adapter report validation driver receipt：state=refreshed outcome=adapter-report-validation-valid-result", "gate adapter report validation driver receipt refreshed driver request：kind=preview-command", "refreshStatusCommand=`/rekit status -Target", "gate adapter report validation driver receipt boundary：driver receipt does not prove the Mission Control runtime executed an adapter"} {
		if !strings.Contains(out.String(), expected) {
			t.Fatalf("adapter validation text omitted receipt %q:\n%s", expected, out.String())
		}
	}

	recordArgs := []string{"-Command", "gate", "-Target", caseRoot, "-Pack", pack, "-Apply", "-GateEventId", authorized.EventID, "-ExecutionReportPath", validation.ReportPath, "-ExpectedExecutionReportSha256", validation.RecordExpectedReportSHA256, "-AdapterExecutionReceiptPath", validation.AdapterExecutionReceiptPath, "-ExpectedAdapterExecutionReceiptSha256", validation.AdapterExecutionReceiptSHA256, "-Executor", "executor-a", "-ExpectedExecutorGeneration", "1", "-Actor", "mission-commander", "-Format", "json"}
	out.Reset()
	if err := Run(recordArgs, &out); err != nil {
		t.Fatal(err)
	}
	var result gate.ApplyResult
	if err := json.Unmarshal(out.Bytes(), &result); err != nil {
		t.Fatal(err)
	}
	if !result.Applied || result.ExecutionEvidence == nil || result.ExecutionEvidence.Execution.AdapterExecution == nil || result.ExecutionEvidence.Execution.AdapterExecutionDispatch == nil || result.ExecutionEvidence.Execution.AdapterExecutionDispatchPath != dispatch.DispatchPath || result.ExecutionEvidence.Execution.AdapterExecutionDispatchSHA256 != dispatch.DispatchSHA256 {
		t.Fatalf("CLI record omitted adapter execution dispatch/completion lineage: %+v", result)
	}
	recordReceipt := result.MissionCommanderDriverReceipt
	recordDriver := result.MissionCommanderActionQueue.CurrentDriverRequest
	if recordReceipt == nil || recordReceipt.SchemaVersion != 1 || recordReceipt.State != "refreshed" || recordReceipt.Outcome != "adapter-execution-record-apply-result" || recordReceipt.Lane != "main" || !strings.Contains(recordReceipt.Command, "-ExpectedExecutionReportSha256") || recordReceipt.RefreshedActionQueueSummary != result.MissionCommanderActionQueue.Summary || recordReceipt.RefreshedCurrentRunLoopStep != result.MissionCommanderActionQueue.CurrentRunLoopStepID || recordReceipt.RefreshedCurrentDriverRequest == nil || recordDriver == nil || recordReceipt.RefreshedCurrentDriverRequest.Command != recordDriver.Command || recordReceipt.RefreshedCurrentDriverRequest.ExpectedReceipt.RefreshStatusCommand == "" || !strings.Contains(recordReceipt.RefreshedCurrentDriverRequest.ExpectedReceipt.RefreshStatusCommand, caseRoot) || !containsSubstring(recordReceipt.Boundary, "does not prove the Mission Control runtime executed an adapter") || !containsSubstring(recordReceipt.Boundary, "does not write authority/confirmed") {
		t.Fatalf("CLI record omitted adapter record run-loop receipt: receipt=%+v queue=%+v", recordReceipt, result.MissionCommanderActionQueue)
	}
	textRecordArgs := append(append([]string{}, recordArgs[:len(recordArgs)-2]...), "-Format", "text")
	out.Reset()
	if err := Run(textRecordArgs, &out); err != nil {
		t.Fatal(err)
	}
	for _, expected := range []string{"gate execution evidence driver receipt：state=refreshed outcome=adapter-execution-record-duplicate-result", "gate execution evidence driver receipt refreshed driver request：kind=preview-command", "refreshStatusCommand=`/rekit status -Target", "gate execution evidence driver receipt boundary：driver receipt does not prove the Mission Control runtime executed an adapter"} {
		if !strings.Contains(out.String(), expected) {
			t.Fatalf("adapter record text omitted receipt %q:\n%s", expected, out.String())
		}
	}
	facts, err := os.ReadFile(filepath.Join(caseRoot, ".rekit", "facts", "observations.jsonl"))
	if err != nil || !strings.Contains(string(facts), "cli-session-a") || !strings.Contains(string(facts), receipt.ReceiptSHA256) {
		t.Fatalf("CLI observation omitted receipt lineage: err=%v facts=%s", err, facts)
	}
	assertFileNotExists(t, filepath.Join(caseRoot, ".rekit", "facts", "authority.jsonl"))
	assertFileNotExists(t, filepath.Join(caseRoot, ".rekit", "facts", "confirmed.jsonl"))
}
