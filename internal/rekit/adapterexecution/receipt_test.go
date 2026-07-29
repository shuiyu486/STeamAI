package adapterexecution

import (
	"encoding/json"
	"strings"
	"testing"

	"github.com/shuiyu486/re-context-kits/internal/rekit/autonomy"
)

func testReceipt(t *testing.T) Receipt {
	t.Helper()
	candidate := Candidate{ID: "adapter-a", Status: "supported", ToolingCatalogPath: "tooling/catalog.yml", RecordOnlyAfterGate: true}
	candidateSHA, err := CandidateSHA256(candidate)
	if err != nil {
		t.Fatal(err)
	}
	gate := GateBinding{GateEventID: "evt-gate", Lane: "main", Action: "debug", Authorization: autonomy.Decision{Decision: autonomy.DecisionPreauthorized, Mode: autonomy.ModePreauthorized, ProfileID: "profile-a", ProfileHash: strings.Repeat("d", 64), Source: "durable-profile", RecordRequired: true}, AuthorizedBudget: autonomy.Budget{RuntimeSeconds: 30}, OutputPaths: []string{"workspace/main/debug"}}
	gateSHA, err := GateSHA256(gate)
	if err != nil {
		t.Fatal(err)
	}
	gate.SnapshotSHA256 = gateSHA
	receipt := Receipt{
		SchemaVersion: 1,
		Kind:          "adapter-execution-receipt",
		Gate:          gate,
		Adapter: AdapterBinding{
			Pack: "unit", AdapterID: candidate.ID, ToolingCatalogPath: candidate.ToolingCatalogPath,
			ToolingCatalogSHA256: strings.Repeat("a", 64), ToolingCatalogBytes: 12,
			Candidate: candidate, CandidateSnapshotSHA256: candidateSHA,
		},
		Dispatch:  DispatchBinding{DispatchID: strings.Repeat("e", 64), Path: ".rekit/lanes/main/adapter-executions/evt-gate/dispatch.json", SHA256: strings.Repeat("f", 64), Bytes: 128},
		Owner:     OwnerBinding{Lane: "main", CurrentExecutor: "executor-a", ExecutorGeneration: 1, AdapterHarness: "harness-a", AdapterSession: "session-a", BindingMode: "durable-lane-owner"},
		Execution: ExecutionBinding{Outcome: "succeeded", ExitStatus: "0", AuthorizedBudget: gate.AuthorizedBudget, ActualBudget: autonomy.Budget{RuntimeSeconds: 10}},
		Report:    FileBinding{Path: "workspace/main/debug/adapter-report.json", SHA256: strings.Repeat("b", 64), Bytes: 100},
		Artifacts: []ArtifactBinding{{Path: "workspace/main/debug/result.bin", Roles: []string{"output"}, SHA256: strings.Repeat("c", 64), Bytes: 3}},
		Actor:     "recorder", RecordedAt: "2026-07-29T00:00:00Z", NoExecute: true, NoAuthority: true,
	}
	receipt.ReceiptID, err = BindingSHA256(receipt)
	if err != nil {
		t.Fatal(err)
	}
	return receipt
}

func testDispatchReceipt(t *testing.T) DispatchReceipt {
	t.Helper()
	completion := testReceipt(t)
	dispatch := DispatchReceipt{
		SchemaVersion: 1,
		Kind:          "adapter-execution-dispatch-receipt",
		Gate:          completion.Gate,
		Adapter:       completion.Adapter,
		Owner:         completion.Owner,
		ReportPath:    completion.Report.Path,
		Actor:         completion.Actor,
		RecordedAt:    "2026-07-29T00:00:00Z",
		NoExecute:     true,
		NoObservation: true,
		NoAuthority:   true,
	}
	var err error
	dispatch.DispatchID, err = DispatchBindingSHA256(dispatch)
	if err != nil {
		t.Fatal(err)
	}
	return dispatch
}

func TestDispatchReceiptDecodeStrictAndCompletionLineage(t *testing.T) {
	dispatch := testDispatchReceipt(t)
	data, err := DispatchReceiptBytes(dispatch)
	if err != nil {
		t.Fatal(err)
	}
	decoded, err := DecodeDispatch(data)
	if err != nil || !DispatchSemanticEqual(decoded, dispatch) {
		t.Fatalf("decoded dispatch differs: %+v err=%v", decoded, err)
	}
	var object map[string]any
	if err := json.Unmarshal(data, &object); err != nil {
		t.Fatal(err)
	}
	object["unknown"] = true
	unknown, _ := json.Marshal(object)
	if _, err := DecodeDispatch(unknown); err == nil || !strings.Contains(err.Error(), "unknown field") {
		t.Fatalf("DecodeDispatch unknown field error = %v", err)
	}
	if _, err := DecodeDispatch(append(data, []byte("{}")...)); err == nil || !strings.Contains(err.Error(), "trailing data") {
		t.Fatalf("DecodeDispatch trailing data error = %v", err)
	}

	completion := testReceipt(t)
	completion.Dispatch = DispatchBinding{DispatchID: dispatch.DispatchID, Path: ".rekit/lanes/main/adapter-executions/evt-gate/dispatch.json", SHA256: SHA256(data), Bytes: int64(len(data))}
	completion.ReceiptID, err = BindingSHA256(completion)
	if err != nil {
		t.Fatal(err)
	}
	if err := ValidateCompletionDispatchLineage(completion, dispatch, completion.Dispatch.Path, completion.Dispatch.SHA256, completion.Dispatch.Bytes); err != nil {
		t.Fatal(err)
	}
	changed := completion
	changed.Owner.AdapterSession = "session-b"
	changed.ReceiptID, _ = BindingSHA256(changed)
	if err := ValidateCompletionDispatchLineage(changed, dispatch, completion.Dispatch.Path, completion.Dispatch.SHA256, completion.Dispatch.Bytes); err == nil || !strings.Contains(err.Error(), "gate/adapter/owner") {
		t.Fatalf("completion session drift error = %v", err)
	}
}

func TestReceiptDecodeStrictAndBindingSensitive(t *testing.T) {
	receipt := testReceipt(t)
	data, err := ReceiptBytes(receipt)
	if err != nil {
		t.Fatal(err)
	}
	decoded, err := Decode(data)
	if err != nil {
		t.Fatal(err)
	}
	if !SemanticEqual(decoded, receipt) {
		t.Fatalf("decoded receipt differs: %+v", decoded)
	}

	var object map[string]any
	if err := json.Unmarshal(data, &object); err != nil {
		t.Fatal(err)
	}
	object["unknown"] = true
	unknown, _ := json.Marshal(object)
	if _, err := Decode(unknown); err == nil || !strings.Contains(err.Error(), "unknown field") {
		t.Fatalf("Decode unknown field error = %v", err)
	}
	if _, err := Decode(append(data, []byte("{}")...)); err == nil || !strings.Contains(err.Error(), "trailing data") {
		t.Fatalf("Decode trailing data error = %v", err)
	}

	changed := receipt
	changed.Owner.ExecutorGeneration++
	changedSHA, err := BindingSHA256(changed)
	if err != nil {
		t.Fatal(err)
	}
	if changedSHA == receipt.ReceiptID {
		t.Fatal("owner generation drift did not change binding hash")
	}
	changed = receipt
	changed.Artifacts = append([]ArtifactBinding{}, receipt.Artifacts...)
	changed.Artifacts[0].SHA256 = strings.Repeat("d", 64)
	changedSHA, err = BindingSHA256(changed)
	if err != nil {
		t.Fatal(err)
	}
	if changedSHA == receipt.ReceiptID {
		t.Fatal("artifact drift did not change binding hash")
	}
}
