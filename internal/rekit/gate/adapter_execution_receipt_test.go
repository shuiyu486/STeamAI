package gate

import (
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/shuiyu486/re-context-kits/internal/rekit/lanemutation"
)

func managedAdapterExecutionFixture(t *testing.T) (string, string, string, ApplyResult, Options) {
	t.Helper()
	return managedAdapterExecutionFixtureWithReportPath(t, "workspace/main/debug/session-1/adapter-report.json")
}

func managedAdapterExecutionFixtureWithReportPath(t *testing.T, reportPath string) (string, string, string, ApplyResult, Options) {
	t.Helper()
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
	opt := Options{
		GateEventID: authorized.EventID, ExecutionReportPath: reportPath,
		AdapterID: "dynamic-debug-or-writeback-action", Executor: "executor-a", ExpectedExecutorGeneration: 1,
		AdapterHarness: "claude-code", AdapterSession: "adapter-session-a", ExecutionExitStatus: "0", Actor: "mission-commander",
	}
	dispatchPreview, err := RecordAdapterExecutionDispatch(repoRoot, caseRoot, pack, opt)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(dispatchPreview.ApplyCommand, "-RecordAdapterExecutionDispatch") || dispatchPreview.Dispatch.ReportPath != opt.ExecutionReportPath {
		t.Fatalf("adapter dispatch preview omitted executable binding: %+v", dispatchPreview)
	}
	dispatchOpt := opt
	dispatchOpt.ExpectedAdapterExecutionDispatchBindingSHA256 = dispatchPreview.BindingSHA256
	dispatch, err := RecordAdapterExecutionDispatch(repoRoot, caseRoot, pack, dispatchOpt)
	if err != nil || !dispatch.Applied {
		t.Fatalf("record adapter execution dispatch: %+v err=%v", dispatch, err)
	}
	base := filepath.Join(caseRoot, "workspace", "main", "debug", "session-1")
	writeGateText(t, filepath.Join(base, "result.bin"), "result-v1")
	writeGateText(t, filepath.Join(base, "evidence.json"), `{"evidence":"v1"}`)
	writeGateText(t, filepath.Join(caseRoot, filepath.FromSlash(reportPath)), `{
  "schemaVersion": 1,
  "kind": "adapter-execution-report",
  "adapterId": "dynamic-debug-or-writeback-action",
  "action": "debug",
  "status": "succeeded",
  "gateEventId": "`+authorized.EventID+`",
  "dispatch": {"dispatchId": "`+dispatch.Dispatch.DispatchID+`", "path": "`+dispatch.DispatchPath+`", "sha256": "`+dispatch.DispatchSHA256+`"},
  "actualBudget": {"runtimeSeconds": 24, "diskMB": 33, "requests": 1},
  "outputRefs": ["workspace/main/debug/session-1/result.bin"],
  "evidenceRefs": ["workspace/main/debug/session-1/evidence.json"],
  "summary": "Adapter completed bounded debug run"
}`)
	return repoRoot, caseRoot, pack, authorized, opt
}

func TestAdapterExecutionDispatchRejectsPostExecutionBackfillAndConflictingSession(t *testing.T) {
	repoRoot, caseRoot, pack, authorized, opt := managedAdapterExecutionFixture(t)
	dispatchPath := filepath.Join(caseRoot, ".rekit", "lanes", "main", "adapter-executions", authorized.EventID, "dispatch.json")
	data, err := os.ReadFile(dispatchPath)
	if err != nil {
		t.Fatal(err)
	}
	if err := os.Remove(dispatchPath); err != nil {
		t.Fatal(err)
	}
	if _, err := RecordAdapterExecutionDispatch(repoRoot, caseRoot, pack, opt); err == nil || !strings.Contains(err.Error(), "before the external execution report exists") {
		t.Fatalf("post-execution dispatch backfill error = %v", err)
	}
	if err := os.WriteFile(dispatchPath, data, 0o600); err != nil {
		t.Fatal(err)
	}
	conflict := opt
	conflict.AdapterSession = "adapter-session-b"
	conflictPreview, err := RecordAdapterExecutionDispatch(repoRoot, caseRoot, pack, conflict)
	if err != nil {
		t.Fatal(err)
	}
	conflict.ExpectedAdapterExecutionDispatchBindingSHA256 = conflictPreview.BindingSHA256
	if _, err := RecordAdapterExecutionDispatch(repoRoot, caseRoot, pack, conflict); err == nil || !strings.Contains(err.Error(), "different semantic bindings") {
		t.Fatalf("same-gate conflicting session dispatch error = %v", err)
	}
}

func TestAdapterExecutionReceiptLifecycleAndEvidenceRecord(t *testing.T) {
	repoRoot, caseRoot, pack, authorized, opt := managedAdapterExecutionFixture(t)
	validation, err := ValidateAdapterExecutionReport(repoRoot, caseRoot, pack, Options{GateEventID: authorized.EventID, ExecutionReportPath: opt.ExecutionReportPath})
	if err != nil {
		t.Fatal(err)
	}
	if validation.Valid || !validation.ReceiptRequired || validation.ReceiptPresent || validation.FailureStage != "provenance" || validation.ReceiptPreviewCommand == "" || validation.ReportSummary.RecordReady || !validation.ReportSummary.RecordBlocked {
		t.Fatalf("managed report should await receipt: %+v", validation)
	}
	assertGateNotExists(t, filepath.Join(caseRoot, ".rekit", "facts", "observations.jsonl"))

	preview, err := RecordAdapterExecutionReceipt(repoRoot, caseRoot, pack, opt)
	if err != nil {
		t.Fatal(err)
	}
	if preview.IsMutation || preview.Applied || preview.BindingSHA256 == "" || preview.ApplyCommand == "" || preview.Receipt.RecordedAt != "" || len(preview.Receipt.Artifacts) != 2 {
		t.Fatalf("unexpected receipt preview: %+v", preview)
	}
	if _, err := os.Stat(filepath.Join(caseRoot, filepath.FromSlash(preview.ReceiptPath))); !os.IsNotExist(err) {
		t.Fatalf("receipt preview wrote target: %v", err)
	}
	opt.ExpectedAdapterExecutionBindingSHA256 = preview.BindingSHA256
	applied, err := RecordAdapterExecutionReceipt(repoRoot, caseRoot, pack, opt)
	if err != nil {
		t.Fatal(err)
	}
	if !applied.Applied || applied.Replay || applied.ReceiptSHA256 == "" || applied.Receipt.RecordedAt == "" {
		t.Fatalf("unexpected receipt apply: %+v", applied)
	}
	receiptBytes, err := os.ReadFile(filepath.Join(caseRoot, filepath.FromSlash(applied.ReceiptPath)))
	if err != nil {
		t.Fatal(err)
	}
	replay, err := RecordAdapterExecutionReceipt(repoRoot, caseRoot, pack, opt)
	if err != nil {
		t.Fatal(err)
	}
	if !replay.Applied || !replay.Replay || replay.ReceiptSHA256 != applied.ReceiptSHA256 {
		t.Fatalf("unexpected receipt replay: %+v", replay)
	}
	receiptBytesAfter, _ := os.ReadFile(filepath.Join(caseRoot, filepath.FromSlash(applied.ReceiptPath)))
	if string(receiptBytesAfter) != string(receiptBytes) {
		t.Fatal("receipt replay rewrote bytes")
	}

	validation, err = ValidateAdapterExecutionReport(repoRoot, caseRoot, pack, Options{GateEventID: authorized.EventID, ExecutionReportPath: opt.ExecutionReportPath})
	if err != nil {
		t.Fatal(err)
	}
	if !validation.Valid || !validation.ProvenanceValid || !validation.ReceiptPresent || validation.AdapterExecution == nil || validation.AdapterExecutionReceiptSHA256 != applied.ReceiptSHA256 || !strings.Contains(validation.MissionCommanderAction.PrimaryCommand, "-ExpectedAdapterExecutionReceiptSha256 "+applied.ReceiptSHA256) {
		t.Fatalf("receipt-backed validation not record-ready: %+v", validation)
	}
	result, err := RecordExecution(repoRoot, caseRoot, pack, Options{
		GateEventID: authorized.EventID, Actor: "mission-commander", ExecutionReportPath: opt.ExecutionReportPath,
		ExpectedExecutionReportSHA256:         validation.RecordExpectedReportSHA256,
		AdapterExecutionReceiptPath:           validation.AdapterExecutionReceiptPath,
		ExpectedAdapterExecutionReceiptSHA256: validation.AdapterExecutionReceiptSHA256,
		Executor:                              "executor-a", ExpectedExecutorGeneration: 1,
	})
	if err != nil {
		t.Fatal(err)
	}
	if !result.Applied || result.ExecutionEvidence == nil || result.ExecutionEvidence.Execution.AdapterExecution == nil || result.ExecutionEvidence.Execution.AdapterExecutionReceiptSHA256 != applied.ReceiptSHA256 {
		t.Fatalf("observation omitted receipt lineage: %+v", result)
	}
	lines := readGateLines(t, filepath.Join(caseRoot, ".rekit", "facts", "observations.jsonl"))
	if len(lines) != 1 || !strings.Contains(lines[0], applied.ReceiptSHA256) || !strings.Contains(lines[0], "adapter-session-a") {
		t.Fatalf("observation ledger omitted durable provenance: %q", lines)
	}
	assertGateNotExists(t, filepath.Join(caseRoot, ".rekit", "facts", "authority.jsonl"))
	assertGateNotExists(t, filepath.Join(caseRoot, ".rekit", "facts", "confirmed.jsonl"))
}

func TestAdapterExecutionReceiptDuplicateObservationRecordIsIdempotent(t *testing.T) {
	repoRoot, caseRoot, pack, authorized, opt := managedAdapterExecutionFixture(t)
	preview, err := RecordAdapterExecutionReceipt(repoRoot, caseRoot, pack, opt)
	if err != nil {
		t.Fatal(err)
	}
	opt.ExpectedAdapterExecutionBindingSHA256 = preview.BindingSHA256
	applied, err := RecordAdapterExecutionReceipt(repoRoot, caseRoot, pack, opt)
	if err != nil {
		t.Fatal(err)
	}
	validation, err := ValidateAdapterExecutionReport(repoRoot, caseRoot, pack, Options{GateEventID: authorized.EventID, ExecutionReportPath: opt.ExecutionReportPath})
	if err != nil {
		t.Fatal(err)
	}
	recordOpt := Options{
		GateEventID: authorized.EventID, Actor: "mission-commander", ExecutionReportPath: opt.ExecutionReportPath,
		ExpectedExecutionReportSHA256: validation.RecordExpectedReportSHA256,
		AdapterExecutionReceiptPath:   validation.AdapterExecutionReceiptPath, ExpectedAdapterExecutionReceiptSHA256: applied.ReceiptSHA256,
		Executor: "executor-a", ExpectedExecutorGeneration: 1,
	}
	first, err := RecordExecution(repoRoot, caseRoot, pack, recordOpt)
	if err != nil {
		t.Fatal(err)
	}
	second, err := RecordExecution(repoRoot, caseRoot, pack, recordOpt)
	if err != nil {
		t.Fatal(err)
	}
	if second.Applied || second.Reason != "duplicate eventId" || second.EventID != first.EventID {
		t.Fatalf("duplicate receipt-backed record should be idempotent: first=%+v second=%+v", first, second)
	}
	if lines := readGateLines(t, filepath.Join(caseRoot, ".rekit", "facts", "observations.jsonl")); len(lines) != 1 {
		t.Fatalf("duplicate receipt-backed record appended observation: %q", lines)
	}
}

func TestManagedAdapterExecutionRejectsMissingReportAndInvalidCatalog(t *testing.T) {
	t.Run("missing report", func(t *testing.T) {
		repoRoot, caseRoot, pack, authorized, _ := managedAdapterExecutionFixture(t)
		_, err := RecordExecution(repoRoot, caseRoot, pack, Options{GateEventID: authorized.EventID, Actor: "mission-commander", ExecutionStatus: "succeeded", OutputRefs: "workspace/main/debug/session-1/result.bin"})
		if err == nil || !strings.Contains(err.Error(), "managed adapter execution provenance requires -ExecutionReportPath") {
			t.Fatalf("managed execution without report error = %v", err)
		}
		assertGateNotExists(t, filepath.Join(caseRoot, ".rekit", "facts", "observations.jsonl"))
	})

	for _, test := range []struct {
		name   string
		mutate func(t *testing.T, repoRoot, pack string)
		want   string
	}{
		{
			name: "missing catalog",
			mutate: func(t *testing.T, repoRoot, pack string) {
				if err := os.Remove(filepath.Join(repoRoot, "packs", pack, "tooling", "catalog.yml")); err != nil {
					t.Fatal(err)
				}
			},
			want: "read tooling catalog",
		},
		{
			name: "malformed catalog",
			mutate: func(t *testing.T, repoRoot, pack string) {
				writeGateText(t, filepath.Join(repoRoot, "packs", pack, "tooling", "catalog.yml"), "tools: [\n")
			},
			want: "parse tooling catalog",
		},
		{
			name: "catalog row missing id",
			mutate: func(t *testing.T, repoRoot, pack string) {
				writeGateText(t, filepath.Join(repoRoot, "packs", pack, "tooling", "catalog.yml"), "tools:\n  - identifier: dynamic-debug-or-writeback-action\n    status: supported\n    entry: <caseRoot>/tools/debug\n    purpose: Execute bounded debug.\n    sideEffects: debug,filesystem-write\n")
			},
			want: "requires a non-empty id",
		},
		{
			name: "catalog row empty id",
			mutate: func(t *testing.T, repoRoot, pack string) {
				writeGateText(t, filepath.Join(repoRoot, "packs", pack, "tooling", "catalog.yml"), "tools:\n  - id:\n    status: supported\n    entry: <caseRoot>/tools/debug\n    purpose: Execute bounded debug.\n    sideEffects: debug,filesystem-write\n")
			},
			want: "requires a non-empty id",
		},
		{
			name: "catalog duplicate id",
			mutate: func(t *testing.T, repoRoot, pack string) {
				writeGateText(t, filepath.Join(repoRoot, "packs", pack, "tooling", "catalog.yml"), "tools:\n  - id: dynamic-debug-or-writeback-action\n    status: supported\n    entry: first\n    sideEffects: debug\n  - id: dynamic-debug-or-writeback-action\n    status: auxiliary\n    entry: second\n    sideEffects: debug\n")
			},
			want: "duplicate tools id",
		},
	} {
		t.Run(test.name, func(t *testing.T) {
			repoRoot, caseRoot, pack, authorized, opt := managedAdapterExecutionFixture(t)
			test.mutate(t, repoRoot, pack)
			validation, err := ValidateAdapterExecutionReport(repoRoot, caseRoot, pack, Options{GateEventID: authorized.EventID, ExecutionReportPath: opt.ExecutionReportPath})
			if err != nil {
				t.Fatal(err)
			}
			if validation.Valid || validation.FailureStage != "provenance" || validation.FailureCode != "adapter-execution-catalog-invalid" || !strings.Contains(validation.Error, test.want) || !validation.ReportSummary.RecordBlocked {
				t.Fatalf("invalid declared catalog should fail closed: %+v", validation)
			}
			_, err = RecordExecution(repoRoot, caseRoot, pack, Options{GateEventID: authorized.EventID, Actor: "mission-commander", ExecutionStatus: "succeeded", OutputRefs: "workspace/main/debug/session-1/result.bin"})
			if err == nil || !strings.Contains(err.Error(), test.want) {
				t.Fatalf("invalid declared catalog record error = %v", err)
			}
			assertGateNotExists(t, filepath.Join(caseRoot, ".rekit", "facts", "observations.jsonl"))
		})
	}
}

func TestAdapterExecutionReceiptRejectsUnknownAndDuplicateCatalogCandidates(t *testing.T) {
	t.Run("unknown adapter", func(t *testing.T) {
		repoRoot, caseRoot, pack, authorized, opt := managedAdapterExecutionFixture(t)
		reportPath := filepath.Join(caseRoot, filepath.FromSlash(opt.ExecutionReportPath))
		data, err := os.ReadFile(reportPath)
		if err != nil {
			t.Fatal(err)
		}
		writeGateText(t, reportPath, strings.Replace(string(data), `"adapterId": "dynamic-debug-or-writeback-action"`, `"adapterId": "unknown-adapter"`, 1))
		validation, err := ValidateAdapterExecutionReport(repoRoot, caseRoot, pack, Options{GateEventID: authorized.EventID, ExecutionReportPath: opt.ExecutionReportPath})
		if err != nil {
			t.Fatal(err)
		}
		if validation.Valid || !validation.ReceiptRequired || validation.FailureStage != "provenance" || !strings.Contains(validation.Error, `exactly one selected tooling catalog candidate for "unknown-adapter"; found 0`) {
			t.Fatalf("unknown managed adapter should fail provenance closed: %+v", validation)
		}
		assertGateNotExists(t, filepath.Join(caseRoot, ".rekit", "facts", "observations.jsonl"))
	})

	t.Run("duplicate adapter", func(t *testing.T) {
		repoRoot, caseRoot, pack, authorized, opt := managedAdapterExecutionFixture(t)
		manifestPath := filepath.Join(repoRoot, "packs", pack, "manifest.yml")
		manifestData, err := os.ReadFile(manifestPath)
		if err != nil {
			t.Fatal(err)
		}
		writeGateText(t, manifestPath, strings.Replace(string(manifestData), "  - tooling/catalog.yml", "  - tooling/catalog.yml\n  - tooling/duplicate-catalog.yml", 1))
		catalogPath := filepath.Join(repoRoot, "packs", pack, "tooling", "catalog.yml")
		catalogData, err := os.ReadFile(catalogPath)
		if err != nil {
			t.Fatal(err)
		}
		writeGateText(t, filepath.Join(repoRoot, "packs", pack, "tooling", "duplicate-catalog.yml"), string(catalogData))
		validation, err := ValidateAdapterExecutionReport(repoRoot, caseRoot, pack, Options{GateEventID: authorized.EventID, ExecutionReportPath: opt.ExecutionReportPath})
		if err != nil {
			t.Fatal(err)
		}
		if validation.Valid || !validation.ReceiptRequired || validation.FailureStage != "provenance" || !strings.Contains(validation.Error, "found 2") {
			t.Fatalf("duplicate managed adapter should fail provenance closed: %+v", validation)
		}
	})
}

func TestAdapterExecutionReceiptRejectsAuthorizedGateSemanticDrift(t *testing.T) {
	for _, test := range []struct {
		name   string
		mutate func(event map[string]any)
	}{
		{name: "target", mutate: func(event map[string]any) { event["target"] = "target-beta" }},
		{name: "authorization profile", mutate: func(event map[string]any) {
			gate := event["gate"].(map[string]any)
			authorization := gate["authorization"].(map[string]any)
			authorization["profileHash"] = strings.Repeat("f", 64)
		}},
		{name: "output paths", mutate: func(event map[string]any) {
			gate := event["gate"].(map[string]any)
			gate["outputPaths"] = []any{"workspace/main/debug/session-1", "workspace/main/debug/session-2"}
		}},
		{name: "stop conditions", mutate: func(event map[string]any) {
			gate := event["gate"].(map[string]any)
			gate["stopConditions"] = []any{"scope-drift"}
		}},
	} {
		t.Run(test.name, func(t *testing.T) {
			repoRoot, caseRoot, pack, authorized, opt := managedAdapterExecutionFixture(t)
			preview, err := RecordAdapterExecutionReceipt(repoRoot, caseRoot, pack, opt)
			if err != nil {
				t.Fatal(err)
			}
			opt.ExpectedAdapterExecutionBindingSHA256 = preview.BindingSHA256
			if _, err := RecordAdapterExecutionReceipt(repoRoot, caseRoot, pack, opt); err != nil {
				t.Fatal(err)
			}
			requestPath := filepath.Join(caseRoot, ".rekit", "facts", "requests.jsonl")
			lines := readGateLines(t, requestPath)
			if len(lines) != 1 {
				t.Fatalf("request lines = %d", len(lines))
			}
			var event map[string]any
			if err := json.Unmarshal([]byte(lines[0]), &event); err != nil {
				t.Fatal(err)
			}
			test.mutate(event)
			data, err := json.Marshal(event)
			if err != nil {
				t.Fatal(err)
			}
			writeGateText(t, requestPath, string(data)+"\n")
			validation, err := ValidateAdapterExecutionReport(repoRoot, caseRoot, pack, Options{GateEventID: authorized.EventID, ExecutionReportPath: opt.ExecutionReportPath})
			if err != nil {
				t.Fatal(err)
			}
			if validation.Valid || validation.FailureStage != "provenance" || !strings.Contains(validation.Error, "dispatch gate, owner, catalog, session, or report path drifted") {
				t.Fatalf("%s drift validation = %+v", test.name, validation)
			}
		})
	}
}

func TestAdapterExecutionReceiptRejectsCatalogAndReportDrift(t *testing.T) {
	for _, test := range []struct {
		name   string
		mutate func(t *testing.T, repoRoot, caseRoot, pack string, opt Options)
		want   string
	}{
		{
			name: "catalog",
			mutate: func(t *testing.T, repoRoot, _, pack string, _ Options) {
				path := filepath.Join(repoRoot, "packs", pack, "tooling", "catalog.yml")
				data, err := os.ReadFile(path)
				if err != nil {
					t.Fatal(err)
				}
				writeGateText(t, path, strings.Replace(string(data), "Execute a bounded debug", "Execute a catalog-drifted bounded debug", 1))
			},
			want: "dispatch gate, owner, catalog, session, or report path drifted",
		},
		{
			name: "report",
			mutate: func(t *testing.T, _, caseRoot, _ string, opt Options) {
				path := filepath.Join(caseRoot, filepath.FromSlash(opt.ExecutionReportPath))
				data, err := os.ReadFile(path)
				if err != nil {
					t.Fatal(err)
				}
				writeGateText(t, path, strings.Replace(string(data), "Adapter completed bounded debug run", "Adapter completed changed bounded debug run", 1))
			},
			want: "report binding drifted",
		},
	} {
		t.Run(test.name, func(t *testing.T) {
			repoRoot, caseRoot, pack, authorized, opt := managedAdapterExecutionFixture(t)
			preview, err := RecordAdapterExecutionReceipt(repoRoot, caseRoot, pack, opt)
			if err != nil {
				t.Fatal(err)
			}
			opt.ExpectedAdapterExecutionBindingSHA256 = preview.BindingSHA256
			if _, err := RecordAdapterExecutionReceipt(repoRoot, caseRoot, pack, opt); err != nil {
				t.Fatal(err)
			}
			test.mutate(t, repoRoot, caseRoot, pack, opt)
			validation, err := ValidateAdapterExecutionReport(repoRoot, caseRoot, pack, Options{GateEventID: authorized.EventID, ExecutionReportPath: opt.ExecutionReportPath})
			if err != nil {
				t.Fatal(err)
			}
			if validation.Valid || validation.FailureStage != "provenance" || !strings.Contains(validation.Error, test.want) || validation.ReceiptPreviewCommand != "" || validation.MissionCommanderAction.State != "blocked-by-adapter-execution-provenance-drift" || strings.Contains(validation.MissionCommanderAction.PrimaryCommand, "-RecordAdapterExecutionReceipt") {
				t.Fatalf("%s drift validation = %+v", test.name, validation)
			}
		})
	}
}

func TestAdapterExecutionReceiptRejectsInvalidArtifactFiles(t *testing.T) {
	for _, test := range []struct {
		name   string
		mutate func(t *testing.T, caseRoot string, opt *Options)
		want   string
	}{
		{
			name: "missing",
			mutate: func(t *testing.T, caseRoot string, _ *Options) {
				if err := os.Remove(filepath.Join(caseRoot, "workspace", "main", "debug", "session-1", "result.bin")); err != nil {
					t.Fatal(err)
				}
			},
			want: "cannot find the file",
		},
		{
			name: "directory",
			mutate: func(t *testing.T, caseRoot string, _ *Options) {
				path := filepath.Join(caseRoot, "workspace", "main", "debug", "session-1", "result.bin")
				if err := os.Remove(path); err != nil {
					t.Fatal(err)
				}
				if err := os.Mkdir(path, 0o700); err != nil {
					t.Fatal(err)
				}
			},
			want: "regular non-symlink file",
		},
		{
			name: "symlink",
			mutate: func(t *testing.T, caseRoot string, _ *Options) {
				path := filepath.Join(caseRoot, "workspace", "main", "debug", "session-1", "result.bin")
				target := filepath.Join(caseRoot, "workspace", "main", "debug", "session-1", "target.bin")
				writeGateText(t, target, "target")
				if err := os.Remove(path); err != nil {
					t.Fatal(err)
				}
				if err := os.Symlink(target, path); err != nil {
					t.Skipf("symlink unavailable: %v", err)
				}
			},
			want: "symlink",
		},
		{
			name: "self reference",
			mutate: func(t *testing.T, caseRoot string, opt *Options) {
				path := filepath.Join(caseRoot, filepath.FromSlash(opt.ExecutionReportPath))
				data, err := os.ReadFile(path)
				if err != nil {
					t.Fatal(err)
				}
				writeGateText(t, path, strings.Replace(string(data), `"workspace/main/debug/session-1/result.bin"`, `"workspace/main/debug/session-1/adapter-report.json"`, 1))
			},
			want: "cannot reference itself",
		},
	} {
		t.Run(test.name, func(t *testing.T) {
			repoRoot, caseRoot, pack, _, opt := managedAdapterExecutionFixture(t)
			test.mutate(t, caseRoot, &opt)
			if _, err := RecordAdapterExecutionReceipt(repoRoot, caseRoot, pack, opt); err == nil || !strings.Contains(strings.ToLower(err.Error()), strings.ToLower(test.want)) {
				t.Fatalf("invalid artifact %s error = %v", test.name, err)
			}
		})
	}
}

type gateTestLease struct {
	gateLaneMutationLease
	onValidate func() error
}

func (lease *gateTestLease) Validate() error {
	if lease.onValidate != nil {
		if err := lease.onValidate(); err != nil {
			return err
		}
	}
	return lease.gateLaneMutationLease.Validate()
}

func TestAdapterExecutionReceiptTempWriteFailureLeavesCanonicalPathAbsent(t *testing.T) {
	repoRoot, caseRoot, pack, _, opt := managedAdapterExecutionFixture(t)
	preview, err := RecordAdapterExecutionReceipt(repoRoot, caseRoot, pack, opt)
	if err != nil {
		t.Fatal(err)
	}
	opt.ExpectedAdapterExecutionBindingSHA256 = preview.BindingSHA256
	original := adapterExecutionReceiptWriteHook
	adapterExecutionReceiptWriteHook = func(stage, _ string) error {
		if stage == "before-temp-write" {
			return errors.New("injected temp write failure")
		}
		return nil
	}
	t.Cleanup(func() { adapterExecutionReceiptWriteHook = original })
	_, err = RecordAdapterExecutionReceipt(repoRoot, caseRoot, pack, opt)
	if err == nil || !strings.Contains(err.Error(), "injected temp write failure") {
		t.Fatalf("temp write failure error = %v", err)
	}
	assertGateNotExists(t, filepath.Join(caseRoot, filepath.FromSlash(preview.ReceiptPath)))
	adapterExecutionReceiptWriteHook = nil
	applied, err := RecordAdapterExecutionReceipt(repoRoot, caseRoot, pack, opt)
	if err != nil || !applied.Applied {
		t.Fatalf("retry after temp write failure = %+v err=%v", applied, err)
	}
}

func TestAdapterExecutionMutationLeaseClosesOwnerAndPostCreateWindows(t *testing.T) {
	t.Run("takeover before receipt lock", func(t *testing.T) {
		repoRoot, caseRoot, pack, _, opt := managedAdapterExecutionFixture(t)
		preview, err := RecordAdapterExecutionReceipt(repoRoot, caseRoot, pack, opt)
		if err != nil {
			t.Fatal(err)
		}
		opt.ExpectedAdapterExecutionBindingSHA256 = preview.BindingSHA256
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
		_, err = RecordAdapterExecutionReceipt(repoRoot, caseRoot, pack, opt)
		if err == nil || !strings.Contains(err.Error(), "owner is stale") {
			t.Fatalf("takeover before receipt lock error = %v", err)
		}
		assertGateNotExists(t, filepath.Join(caseRoot, filepath.FromSlash(preview.ReceiptPath)))
	})

	t.Run("takeover before observation lock", func(t *testing.T) {
		repoRoot, caseRoot, pack, authorized, opt := managedAdapterExecutionFixture(t)
		preview, err := RecordAdapterExecutionReceipt(repoRoot, caseRoot, pack, opt)
		if err != nil {
			t.Fatal(err)
		}
		opt.ExpectedAdapterExecutionBindingSHA256 = preview.BindingSHA256
		applied, err := RecordAdapterExecutionReceipt(repoRoot, caseRoot, pack, opt)
		if err != nil {
			t.Fatal(err)
		}
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
		_, err = RecordExecution(repoRoot, caseRoot, pack, Options{GateEventID: authorized.EventID, Actor: "mission-commander", ExecutionReportPath: opt.ExecutionReportPath, ExpectedExecutionReportSHA256: applied.Receipt.Report.SHA256, AdapterExecutionReceiptPath: applied.ReceiptPath, ExpectedAdapterExecutionReceiptSHA256: applied.ReceiptSHA256, Executor: "executor-a", ExpectedExecutorGeneration: 1})
		if err == nil || !strings.Contains(err.Error(), "owner is stale") {
			t.Fatalf("takeover before observation lock error = %v", err)
		}
		assertGateNotExists(t, filepath.Join(caseRoot, ".rekit", "facts", "observations.jsonl"))
	})

	t.Run("post create lease failure preserves receipt", func(t *testing.T) {
		repoRoot, caseRoot, pack, _, opt := managedAdapterExecutionFixture(t)
		preview, err := RecordAdapterExecutionReceipt(repoRoot, caseRoot, pack, opt)
		if err != nil {
			t.Fatal(err)
		}
		opt.ExpectedAdapterExecutionBindingSHA256 = preview.BindingSHA256
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
					return errors.New("injected post-create validation failure")
				}
				return nil
			}}, nil
		}
		t.Cleanup(func() { acquireGateLaneMutationLease = original })
		_, err = RecordAdapterExecutionReceipt(repoRoot, caseRoot, pack, opt)
		if err == nil || !strings.Contains(err.Error(), "may already be durable") {
			t.Fatalf("post-create validation error = %v", err)
		}
		if _, err := os.Stat(filepath.Join(caseRoot, filepath.FromSlash(preview.ReceiptPath))); err != nil {
			t.Fatalf("post-create failure removed receipt: %v", err)
		}
	})
}

func TestAdapterExecutionReceiptRejectsOwnerAndArtifactDrift(t *testing.T) {
	t.Run("owner before apply", func(t *testing.T) {
		repoRoot, caseRoot, pack, _, opt := managedAdapterExecutionFixture(t)
		preview, err := RecordAdapterExecutionReceipt(repoRoot, caseRoot, pack, opt)
		if err != nil {
			t.Fatal(err)
		}
		lanePath := filepath.Join(caseRoot, ".rekit", "lanes", "main", "lane.json")
		data, _ := os.ReadFile(lanePath)
		writeGateText(t, lanePath, strings.Replace(strings.Replace(string(data), `"currentExecutor": "executor-a"`, `"currentExecutor": "executor-b"`, 1), `"executorGeneration": 1`, `"executorGeneration": 2`, 1))
		opt.ExpectedAdapterExecutionBindingSHA256 = preview.BindingSHA256
		if _, err := RecordAdapterExecutionReceipt(repoRoot, caseRoot, pack, opt); err == nil || !strings.Contains(err.Error(), "owner is stale") {
			t.Fatalf("owner drift error = %v", err)
		}
		assertGateNotExists(t, filepath.Join(caseRoot, filepath.FromSlash(preview.ReceiptPath)))
	})

	t.Run("artifact after receipt", func(t *testing.T) {
		repoRoot, caseRoot, pack, authorized, opt := managedAdapterExecutionFixture(t)
		preview, err := RecordAdapterExecutionReceipt(repoRoot, caseRoot, pack, opt)
		if err != nil {
			t.Fatal(err)
		}
		opt.ExpectedAdapterExecutionBindingSHA256 = preview.BindingSHA256
		if _, err := RecordAdapterExecutionReceipt(repoRoot, caseRoot, pack, opt); err != nil {
			t.Fatal(err)
		}
		writeGateText(t, filepath.Join(caseRoot, "workspace", "main", "debug", "session-1", "result.bin"), "result-v2")
		validation, err := ValidateAdapterExecutionReport(repoRoot, caseRoot, pack, Options{GateEventID: authorized.EventID, ExecutionReportPath: opt.ExecutionReportPath})
		if err != nil {
			t.Fatal(err)
		}
		if validation.Valid || validation.FailureStage != "provenance" || !strings.Contains(validation.Error, "artifact bytes drifted") || !validation.ReportSummary.RecordBlocked || !validation.ReceiptPresent || validation.ReceiptPreviewCommand != "" || validation.MissionCommanderAction.State != "blocked-by-adapter-execution-provenance-drift" {
			t.Fatalf("artifact drift validation = %+v", validation)
		}
		assertGateNotExists(t, filepath.Join(caseRoot, ".rekit", "facts", "observations.jsonl"))
	})

	t.Run("owner after receipt", func(t *testing.T) {
		repoRoot, caseRoot, pack, authorized, opt := managedAdapterExecutionFixture(t)
		preview, err := RecordAdapterExecutionReceipt(repoRoot, caseRoot, pack, opt)
		if err != nil {
			t.Fatal(err)
		}
		opt.ExpectedAdapterExecutionBindingSHA256 = preview.BindingSHA256
		applied, err := RecordAdapterExecutionReceipt(repoRoot, caseRoot, pack, opt)
		if err != nil {
			t.Fatal(err)
		}
		lanePath := filepath.Join(caseRoot, ".rekit", "lanes", "main", "lane.json")
		data, err := os.ReadFile(lanePath)
		if err != nil {
			t.Fatal(err)
		}
		writeGateText(t, lanePath, strings.Replace(strings.Replace(string(data), `"currentExecutor": "executor-a"`, `"currentExecutor": "executor-b"`, 1), `"executorGeneration": 1`, `"executorGeneration": 2`, 1))
		_, err = RecordExecution(repoRoot, caseRoot, pack, Options{
			GateEventID: authorized.EventID, Actor: "mission-commander", ExecutionReportPath: opt.ExecutionReportPath,
			ExpectedExecutionReportSHA256: applied.Receipt.Report.SHA256,
			AdapterExecutionReceiptPath:   applied.ReceiptPath, ExpectedAdapterExecutionReceiptSHA256: applied.ReceiptSHA256,
			Executor: "executor-a", ExpectedExecutorGeneration: 1,
		})
		if err == nil || !strings.Contains(err.Error(), "owner is stale") {
			t.Fatalf("owner drift after receipt error = %v", err)
		}
		assertGateNotExists(t, filepath.Join(caseRoot, ".rekit", "facts", "observations.jsonl"))
	})
}
