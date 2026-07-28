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
	} {
		var out bytes.Buffer
		if err := Run(args, &out); err == nil || !strings.Contains(err.Error(), "supported only with gate -RecordAdapterExecutionReceipt") {
			t.Fatalf("receipt-only flag outside mode error = %v", err)
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
	writeCaseFile(t, caseRoot, "workspace/main/debug/session-1/result.bin", "cli-result")
	writeCaseFile(t, caseRoot, "workspace/main/debug/session-1/adapter-report.json", `{
  "schemaVersion": 1,
  "kind": "adapter-execution-report",
  "adapterId": "dynamic-debug-or-writeback-action",
  "action": "debug",
  "status": "succeeded",
  "gateEventId": "`+authorized.EventID+`",
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

	out.Reset()
	if err := Run([]string{"-Command", "gate", "-Target", caseRoot, "-Pack", pack, "-GateEventId", authorized.EventID, "-ValidateExecutionReport", "-ExecutionReportPath", "workspace/main/debug/session-1/adapter-report.json", "-Format", "json"}, &out); err != nil {
		t.Fatal(err)
	}
	var validation gate.AdapterExecutionReportValidation
	if err := json.Unmarshal(out.Bytes(), &validation); err != nil {
		t.Fatal(err)
	}
	if !validation.Valid || !validation.ProvenanceValid || !strings.Contains(validation.MissionCommanderAction.PrimaryCommand, "-ExpectedAdapterExecutionReceiptSha256") {
		t.Fatalf("CLI validation omitted receipt-backed record command: %+v", validation)
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
	if !result.Applied || result.ExecutionEvidence == nil || result.ExecutionEvidence.Execution.AdapterExecution == nil {
		t.Fatalf("CLI record omitted adapter execution receipt: %+v", result)
	}
	facts, err := os.ReadFile(filepath.Join(caseRoot, ".rekit", "facts", "observations.jsonl"))
	if err != nil || !strings.Contains(string(facts), "cli-session-a") || !strings.Contains(string(facts), receipt.ReceiptSHA256) {
		t.Fatalf("CLI observation omitted receipt lineage: err=%v facts=%s", err, facts)
	}
	assertFileNotExists(t, filepath.Join(caseRoot, ".rekit", "facts", "authority.jsonl"))
	assertFileNotExists(t, filepath.Join(caseRoot, ".rekit", "facts", "confirmed.jsonl"))
}
