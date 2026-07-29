package gate

import (
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/shuiyu486/re-context-kits/internal/rekit/lanemutation"
)

func managedAdapterDispatchOnlyFixture(t *testing.T) (string, string, string, ApplyResult, Options) {
	t.Helper()
	repoRoot, caseRoot, pack, authorized, opt := managedAdapterExecutionFixture(t)
	if err := os.Remove(filepath.Join(caseRoot, filepath.FromSlash(opt.ExecutionReportPath))); err != nil {
		t.Fatal(err)
	}
	return repoRoot, caseRoot, pack, authorized, opt
}

func TestAdapterReportContractRoutesCurrentDispatchToTerminalReport(t *testing.T) {
	repoRoot, caseRoot, pack, authorized, opt := managedAdapterDispatchOnlyFixture(t)
	contract, err := AdapterReportContract(repoRoot, caseRoot, pack, Options{GateEventID: authorized.EventID})
	if err != nil {
		t.Fatal(err)
	}
	live := contract.LiveValidation
	if !live.DispatchRequired || !live.DispatchPresent || !live.DispatchCurrent || live.AdapterExecutionDispatchID == "" || live.SidecarTemplate.Dispatch == nil {
		t.Fatalf("current dispatch was not projected into report contract: %+v", live)
	}
	if contract.MissionCommanderAction.State != "adapter-execution-dispatched-awaiting-report" || strings.Contains(contract.MissionCommanderAction.PrimaryCommand, "-RecordAdapterExecutionDispatch") || !strings.Contains(contract.MissionCommanderAction.PrimaryCommand, "-DraftExecutionReport") || !strings.Contains(contract.MissionCommanderAction.PrimaryCommand, "-ExecutionStatus failed|aborted") {
		t.Fatalf("current dispatch did not route to terminal report preview: %+v", contract.MissionCommanderAction)
	}
	if contract.MissionCommanderActionQueue.CurrentAction == nil || contract.MissionCommanderActionQueue.CurrentAction.ActionID != authorized.EventID+":adapter-execution-terminal-report-preview" {
		t.Fatalf("terminal report preview was not current: %+v", contract.MissionCommanderActionQueue)
	}

	draftOpt := Options{
		GateEventID: authorized.EventID, ExecutionReportPath: opt.ExecutionReportPath,
		AdapterID: opt.AdapterID, ExecutionStatus: "failed", Summary: "External harness reported failure before writing its sidecar",
	}
	preview, err := DraftAdapterExecutionReport(repoRoot, caseRoot, pack, draftOpt)
	if err != nil {
		t.Fatal(err)
	}
	if preview.Applied || preview.Report.Dispatch == nil || preview.Report.Dispatch.DispatchID != live.AdapterExecutionDispatchID || preview.Report.Status != "failed" || preview.ApplyCommand == "" {
		t.Fatalf("terminal report preview omitted dispatch-bound failure: %+v", preview)
	}
	draftOpt.ExpectedExecutionReportSHA256 = preview.ReportSHA256
	applied, err := DraftAdapterExecutionReport(repoRoot, caseRoot, pack, draftOpt)
	if err != nil {
		t.Fatal(err)
	}
	if !applied.Applied || applied.Report.Status != "failed" {
		t.Fatalf("terminal report Apply failed: %+v", applied)
	}

	receiptOpt := opt
	receiptOpt.ExecutionExitStatus = "external-failure"
	receiptPreview, err := RecordAdapterExecutionReceipt(repoRoot, caseRoot, pack, receiptOpt)
	if err != nil {
		t.Fatal(err)
	}
	receiptOpt.ExpectedAdapterExecutionBindingSHA256 = receiptPreview.BindingSHA256
	receipt, err := RecordAdapterExecutionReceipt(repoRoot, caseRoot, pack, receiptOpt)
	if err != nil {
		t.Fatal(err)
	}
	validation, err := ValidateAdapterExecutionReport(repoRoot, caseRoot, pack, Options{GateEventID: authorized.EventID, ExecutionReportPath: opt.ExecutionReportPath})
	if err != nil {
		t.Fatal(err)
	}
	if !validation.Valid || !validation.ProvenanceValid || validation.Report == nil || validation.Report.Status != "failed" || validation.AdapterExecutionReceiptSHA256 != receipt.ReceiptSHA256 {
		t.Fatalf("terminal report did not close completion provenance: %+v", validation)
	}
	result, err := RecordExecution(repoRoot, caseRoot, pack, Options{
		GateEventID: authorized.EventID, Actor: "mission-commander", ExecutionReportPath: opt.ExecutionReportPath,
		ExpectedExecutionReportSHA256: validation.RecordExpectedReportSHA256,
		AdapterExecutionReceiptPath:   validation.AdapterExecutionReceiptPath, ExpectedAdapterExecutionReceiptSHA256: validation.AdapterExecutionReceiptSHA256,
		Executor: "executor-a", ExpectedExecutorGeneration: 1,
	})
	if err != nil {
		t.Fatal(err)
	}
	if !result.Applied || result.ExecutionEvidence == nil || result.ExecutionEvidence.Execution.Status != "failed" {
		t.Fatalf("terminal observation was not recorded: %+v", result)
	}
	assertGateNotExists(t, filepath.Join(caseRoot, ".rekit", "facts", "authority.jsonl"))
	assertGateNotExists(t, filepath.Join(caseRoot, ".rekit", "facts", "confirmed.jsonl"))
}

func TestAdapterReportContractRoutesStaleDispatchToDistinctGate(t *testing.T) {
	repoRoot, caseRoot, pack, authorized, _ := managedAdapterDispatchOnlyFixture(t)
	lanePath := filepath.Join(caseRoot, ".rekit", "lanes", "main", "lane.json")
	data, err := os.ReadFile(lanePath)
	if err != nil {
		t.Fatal(err)
	}
	writeGateText(t, lanePath, strings.Replace(strings.Replace(string(data), `"currentExecutor": "executor-a"`, `"currentExecutor": "executor-b"`, 1), `"executorGeneration": 1`, `"executorGeneration": 2`, 1))

	contract, err := AdapterReportContract(repoRoot, caseRoot, pack, Options{GateEventID: authorized.EventID})
	if err != nil {
		t.Fatal(err)
	}
	live := contract.LiveValidation
	if !live.DispatchPresent || live.DispatchCurrent || live.DispatchError == "" || live.AdapterExecutionDispatchID == "" {
		t.Fatalf("stale dispatch state was not projected: %+v", live)
	}
	action := contract.MissionCommanderAction
	if action.State != "blocked-by-adapter-execution-dispatch-drift" || !strings.Contains(action.PrimaryCommand, "-WhatIf") || strings.Contains(action.PrimaryCommand, "-RecordAdapterExecutionDispatch") || !strings.Contains(strings.Join(action.Boundary, " "), "distinct gate") {
		t.Fatalf("stale dispatch did not route to distinct reauthorization: %+v", action)
	}
	if contract.MissionCommanderActionQueue.CurrentAction == nil || !contract.MissionCommanderActionQueue.CurrentAction.Blocked || contract.MissionCommanderActionQueue.CurrentAction.ActionID != authorized.EventID+":adapter-execution-dispatch-drift-retry" {
		t.Fatalf("stale dispatch retry action was not current/blocked: %+v", contract.MissionCommanderActionQueue)
	}
	assertGateNotExists(t, filepath.Join(caseRoot, ".rekit", "facts", "observations.jsonl"))
}

func TestAdapterReportTerminalRecoveryUsesDispatchReportPath(t *testing.T) {
	const reportPath = "workspace/main/debug/session-1/custom-terminal.json"
	repoRoot, caseRoot, pack, authorized, opt := managedAdapterExecutionFixtureWithReportPath(t, reportPath)
	if err := os.Remove(filepath.Join(caseRoot, filepath.FromSlash(reportPath))); err != nil {
		t.Fatal(err)
	}

	contract, err := AdapterReportContract(repoRoot, caseRoot, pack, Options{GateEventID: authorized.EventID})
	if err != nil {
		t.Fatal(err)
	}
	if contract.LiveValidation.CaseRelativeReportPath != reportPath || !strings.Contains(contract.MissionCommanderAction.PrimaryCommand, "-ExecutionReportPath "+reportPath) {
		t.Fatalf("terminal recovery did not retain dispatch report path: %+v", contract.LiveValidation)
	}
	wrongPath := "workspace/main/debug/session-1/adapter-report.json"
	if _, err := DraftAdapterExecutionReport(repoRoot, caseRoot, pack, Options{GateEventID: authorized.EventID, ExecutionReportPath: wrongPath, AdapterID: opt.AdapterID, ExecutionStatus: "failed", Summary: "known failure"}); err == nil || !strings.Contains(err.Error(), "must match immutable dispatch report path") {
		t.Fatalf("wrong dispatch report path error = %v", err)
	}
	draftOpt := Options{GateEventID: authorized.EventID, ExecutionReportPath: reportPath, AdapterID: opt.AdapterID, ExecutionStatus: "aborted", Summary: "external harness aborted before sidecar write"}
	preview, err := DraftAdapterExecutionReport(repoRoot, caseRoot, pack, draftOpt)
	if err != nil || preview.ReportPath != reportPath || preview.Report.Dispatch == nil {
		t.Fatalf("custom terminal report preview = %+v err=%v", preview, err)
	}
	draftOpt.ExpectedExecutionReportSHA256 = preview.ReportSHA256
	applied, err := DraftAdapterExecutionReport(repoRoot, caseRoot, pack, draftOpt)
	if err != nil || !applied.Applied {
		t.Fatalf("custom terminal report Apply = %+v err=%v", applied, err)
	}
	receiptOpt := opt
	receiptOpt.ExecutionExitStatus = "external-abort"
	receiptPreview, err := RecordAdapterExecutionReceipt(repoRoot, caseRoot, pack, receiptOpt)
	if err != nil {
		t.Fatal(err)
	}
	receiptOpt.ExpectedAdapterExecutionBindingSHA256 = receiptPreview.BindingSHA256
	receipt, err := RecordAdapterExecutionReceipt(repoRoot, caseRoot, pack, receiptOpt)
	if err != nil || !receipt.Applied {
		t.Fatalf("custom terminal receipt = %+v err=%v", receipt, err)
	}
	validation, err := ValidateAdapterExecutionReport(repoRoot, caseRoot, pack, Options{GateEventID: authorized.EventID, ExecutionReportPath: reportPath})
	if err != nil || !validation.Valid || validation.ReportPath != reportPath || validation.AdapterExecutionReceiptSHA256 != receipt.ReceiptSHA256 {
		t.Fatalf("custom terminal validation = %+v err=%v", validation, err)
	}
	result, err := RecordExecution(repoRoot, caseRoot, pack, Options{
		GateEventID: authorized.EventID, Actor: "mission-commander", ExecutionReportPath: reportPath,
		ExpectedExecutionReportSHA256: validation.RecordExpectedReportSHA256,
		AdapterExecutionReceiptPath:   validation.AdapterExecutionReceiptPath, ExpectedAdapterExecutionReceiptSHA256: validation.AdapterExecutionReceiptSHA256,
		Executor: "executor-a", ExpectedExecutorGeneration: 1,
	})
	if err != nil || !result.Applied || result.ExecutionEvidence == nil || result.ExecutionEvidence.Execution.ExecutionReportPath != reportPath {
		t.Fatalf("custom terminal observation = %+v err=%v", result, err)
	}
}

func TestAdapterReportTerminalRecoveryRejectsSucceededAndMalformedCatalog(t *testing.T) {
	t.Run("succeeded requires external report", func(t *testing.T) {
		repoRoot, caseRoot, pack, authorized, opt := managedAdapterDispatchOnlyFixture(t)
		_, err := DraftAdapterExecutionReport(repoRoot, caseRoot, pack, Options{GateEventID: authorized.EventID, ExecutionReportPath: opt.ExecutionReportPath, AdapterID: opt.AdapterID, ExecutionStatus: "succeeded"})
		if err == nil || !strings.Contains(err.Error(), "requires -ExecutionStatus failed|aborted") {
			t.Fatalf("succeeded terminal recovery error = %v", err)
		}
	})

	t.Run("malformed catalog blocks dispatch", func(t *testing.T) {
		repoRoot, caseRoot, pack := gateToolingFixture(t)
		writePreauthorizedProfile(t, caseRoot)
		writeGateText(t, filepath.Join(caseRoot, ".rekit", "lanes", "main", "lane.json"), `{
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
		authorized, err := Apply(repoRoot, caseRoot, pack, Options{Action: "debug", Lane: "main", Actor: "gate-test", Subject: "authorized debug", TargetRef: "target-alpha", RuntimeSeconds: 30, DiskMB: 64, Requests: 1, OutputPaths: "workspace/main/debug/session-1", StopConditions: "timeout"})
		if err != nil {
			t.Fatal(err)
		}
		writeGateText(t, filepath.Join(repoRoot, "packs", pack, "tooling", "catalog.yml"), "tools: [\n")
		contract, err := AdapterReportContract(repoRoot, caseRoot, pack, Options{GateEventID: authorized.EventID})
		if err != nil {
			t.Fatal(err)
		}
		if contract.MissionCommanderAction.State != "blocked-by-adapter-execution-catalog-invalid" || strings.Contains(contract.MissionCommanderAction.PrimaryCommand, "-RecordAdapterExecutionDispatch") || strings.Contains(contract.MissionCommanderAction.PrimaryCommand, "-DraftExecutionReport") || contract.LiveValidation.DispatchRequirementError == "" {
			t.Fatalf("malformed catalog did not fail closed: %+v", contract)
		}
	})
}

func TestAdapterReportTerminalRecoveryMutationLease(t *testing.T) {
	t.Run("takeover before draft lock", func(t *testing.T) {
		repoRoot, caseRoot, pack, authorized, opt := managedAdapterDispatchOnlyFixture(t)
		draftOpt := Options{GateEventID: authorized.EventID, ExecutionReportPath: opt.ExecutionReportPath, AdapterID: opt.AdapterID, ExecutionStatus: "failed", Summary: "known harness failure"}
		preview, err := DraftAdapterExecutionReport(repoRoot, caseRoot, pack, draftOpt)
		if err != nil {
			t.Fatal(err)
		}
		draftOpt.ExpectedExecutionReportSHA256 = preview.ReportSHA256
		original := acquireGateLaneMutationLease
		acquireGateLaneMutationLease = func(root, lane string) (gateLaneMutationLease, error) {
			lanePath := filepath.Join(root, ".rekit", "lanes", lane, "lane.json")
			data, err := os.ReadFile(lanePath)
			if err != nil {
				return nil, err
			}
			writeGateText(t, lanePath, strings.Replace(strings.Replace(string(data), `"currentExecutor": "executor-a"`, `"currentExecutor": "executor-b"`, 1), `"executorGeneration": 1`, `"executorGeneration": 2`, 1))
			return lanemutation.AcquireLane(root, lane)
		}
		t.Cleanup(func() { acquireGateLaneMutationLease = original })
		_, err = DraftAdapterExecutionReport(repoRoot, caseRoot, pack, draftOpt)
		if err == nil || !strings.Contains(err.Error(), "immutable dispatch") {
			t.Fatalf("takeover before draft lock error = %v", err)
		}
		assertGateNotExists(t, filepath.Join(caseRoot, filepath.FromSlash(opt.ExecutionReportPath)))
	})

	t.Run("post write lease failure preserves report", func(t *testing.T) {
		repoRoot, caseRoot, pack, authorized, opt := managedAdapterDispatchOnlyFixture(t)
		draftOpt := Options{GateEventID: authorized.EventID, ExecutionReportPath: opt.ExecutionReportPath, AdapterID: opt.AdapterID, ExecutionStatus: "aborted", Summary: "known harness abort"}
		preview, err := DraftAdapterExecutionReport(repoRoot, caseRoot, pack, draftOpt)
		if err != nil {
			t.Fatal(err)
		}
		draftOpt.ExpectedExecutionReportSHA256 = preview.ReportSHA256
		original := acquireGateLaneMutationLease
		acquireGateLaneMutationLease = func(root, lane string) (gateLaneMutationLease, error) {
			base, err := lanemutation.AcquireLane(root, lane)
			if err != nil {
				return nil, err
			}
			calls := 0
			return &gateTestLease{gateLaneMutationLease: base, onValidate: func() error {
				calls++
				if calls == 2 {
					return errors.New("injected post-write validation failure")
				}
				return nil
			}}, nil
		}
		t.Cleanup(func() { acquireGateLaneMutationLease = original })
		_, err = DraftAdapterExecutionReport(repoRoot, caseRoot, pack, draftOpt)
		if err == nil || !strings.Contains(err.Error(), "may already be durable") {
			t.Fatalf("post-write validation error = %v", err)
		}
		if _, err := os.Stat(filepath.Join(caseRoot, filepath.FromSlash(opt.ExecutionReportPath))); err != nil {
			t.Fatalf("post-write failure removed terminal report: %v", err)
		}
	})
}
