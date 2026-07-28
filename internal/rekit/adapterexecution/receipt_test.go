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
