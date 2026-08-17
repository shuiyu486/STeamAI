package reviewersession

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestOutputContractFieldsBindsExactPacketRouteShardAndItems(t *testing.T) {
	caseRoot := t.TempDir()
	packetRel := ".rekit/reviews/packet/packet.json"
	packetPath := filepath.Join(caseRoot, filepath.FromSlash(packetRel))
	if err := os.MkdirAll(filepath.Dir(packetPath), 0o700); err != nil {
		t.Fatal(err)
	}
	packet := map[string]any{
		"packetId":             "packet-a",
		"route":                map[string]any{"id": "route-a", "outputContract": "item,decision,candidate_path"},
		"shards":               []any{map[string]any{"id": "shard-01", "items": []string{"item-a"}}},
		"outputContract":       "item,decision,candidate_path",
		"unrelatedPacketField": true,
	}
	data, err := json.Marshal(packet)
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(packetPath, data, 0o600); err != nil {
		t.Fatal(err)
	}
	receipt := DispatchReceipt{PacketID: "packet-a", PacketPath: packetRel, PacketSHA256: sha256Hex(data), RouteID: "route-a", ShardID: "shard-01", Items: []string{"item-a"}}
	fields, err := OutputContractFields(caseRoot, receipt)
	if err != nil || strings.Join(fields, ",") != "item,decision,candidate_path" {
		t.Fatalf("fields=%v err=%v", fields, err)
	}
	if err := ValidateRouteOutput(fields, map[string]any{"item": "item-a", "decision": "accept", "candidate_path": "candidate"}); err != nil {
		t.Fatal(err)
	}
	for _, drift := range []struct {
		name   string
		mutate func(*DispatchReceipt)
	}{
		{name: "packet hash", mutate: func(value *DispatchReceipt) { value.PacketSHA256 = strings.Repeat("a", 64) }},
		{name: "route", mutate: func(value *DispatchReceipt) { value.RouteID = "route-b" }},
		{name: "items", mutate: func(value *DispatchReceipt) { value.Items = []string{"item-b"} }},
	} {
		t.Run(drift.name, func(t *testing.T) {
			changed := receipt
			drift.mutate(&changed)
			if _, err := OutputContractFields(caseRoot, changed); err == nil {
				t.Fatal("drifted packet contract was accepted")
			}
		})
	}
}

func TestOutputContractFieldsUsesSTeamAIReviewNamespace(t *testing.T) {
	caseRoot := t.TempDir()
	packetRel := ".steamai/reviews/packet/packet.json"
	packetPath := filepath.Join(caseRoot, filepath.FromSlash(packetRel))
	if err := os.MkdirAll(filepath.Dir(packetPath), 0o700); err != nil {
		t.Fatal(err)
	}
	packet := map[string]any{
		"packetId":       "packet-a",
		"route":          map[string]any{"id": "route-a", "outputContract": "item,decision"},
		"shards":         []any{map[string]any{"id": "shard-01", "items": []string{"item-a"}}},
		"outputContract": "item,decision",
	}
	data, err := json.Marshal(packet)
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(packetPath, data, 0o600); err != nil {
		t.Fatal(err)
	}
	receipt := DispatchReceipt{PacketID: "packet-a", PacketPath: packetRel, PacketSHA256: sha256Hex(data), RouteID: "route-a", ShardID: "shard-01", Items: []string{"item-a"}}
	fields, err := OutputContractFields(caseRoot, receipt)
	if err != nil || strings.Join(fields, ",") != "item,decision" {
		t.Fatalf("fields=%v err=%v", fields, err)
	}
}

func TestOutputContractFieldsRejectsDualStateRoots(t *testing.T) {
	caseRoot := t.TempDir()
	for _, root := range []string{".steamai", ".rekit"} {
		if err := os.MkdirAll(filepath.Join(caseRoot, root), 0o700); err != nil {
			t.Fatal(err)
		}
	}
	receipt := DispatchReceipt{PacketPath: ".steamai/reviews/packet/packet.json"}
	if _, err := OutputContractFields(caseRoot, receipt); err == nil {
		t.Fatal("dual mutable roots must fail closed")
	}
}

func TestValidateCompletionDispatchLineageRejectsDrift(t *testing.T) {
	dispatchPath := filepath.Join(t.TempDir(), "dispatch.json")
	dispatchSHA256 := strings.Repeat("a", 64)
	owner := Owner{CurrentExecutor: "session-a", ExecutorGeneration: 2, BindingMode: "durable-lane-owner"}
	dispatch := DispatchReceipt{
		DispatchID:          "dispatch-a",
		PacketID:            "packet-a",
		RouteID:             "route-a",
		ShardID:             "shard-01",
		ReviewerHarness:     "harness-a",
		ReviewerSession:     "reviewer-session-a",
		EffectiveOwner:      owner,
		OwnerAdoptionPath:   filepath.Join(filepath.Dir(dispatchPath), "adoption.json"),
		OwnerAdoptionSHA256: strings.Repeat("b", 64),
	}
	completion := CompletionReceipt{
		DispatchID:            dispatch.DispatchID,
		DispatchReceiptPath:   dispatchPath,
		DispatchReceiptSHA256: dispatchSHA256,
		PacketID:              dispatch.PacketID,
		RouteID:               dispatch.RouteID,
		ShardID:               dispatch.ShardID,
		ReviewerHarness:       dispatch.ReviewerHarness,
		ReviewerSession:       dispatch.ReviewerSession,
		CompletionOwner:       owner,
		OwnerAdoptionPath:     dispatch.OwnerAdoptionPath,
		OwnerAdoptionSHA256:   dispatch.OwnerAdoptionSHA256,
	}
	if err := ValidateCompletionDispatchLineage(completion, dispatch, dispatchPath, dispatchSHA256); err != nil {
		t.Fatal(err)
	}

	tests := []struct {
		name   string
		mutate func(*CompletionReceipt)
	}{
		{name: "dispatch id", mutate: func(value *CompletionReceipt) { value.DispatchID = "dispatch-b" }},
		{name: "dispatch path", mutate: func(value *CompletionReceipt) {
			value.DispatchReceiptPath = filepath.Join(filepath.Dir(dispatchPath), "other.json")
		}},
		{name: "dispatch hash", mutate: func(value *CompletionReceipt) { value.DispatchReceiptSHA256 = strings.Repeat("c", 64) }},
		{name: "packet id", mutate: func(value *CompletionReceipt) { value.PacketID = "packet-b" }},
		{name: "route id", mutate: func(value *CompletionReceipt) { value.RouteID = "route-b" }},
		{name: "shard id", mutate: func(value *CompletionReceipt) { value.ShardID = "shard-02" }},
		{name: "reviewer harness", mutate: func(value *CompletionReceipt) { value.ReviewerHarness = "harness-b" }},
		{name: "reviewer session", mutate: func(value *CompletionReceipt) { value.ReviewerSession = "reviewer-session-b" }},
		{name: "completion owner", mutate: func(value *CompletionReceipt) { value.CompletionOwner.ExecutorGeneration++ }},
		{name: "adoption path", mutate: func(value *CompletionReceipt) {
			value.OwnerAdoptionPath = filepath.Join(filepath.Dir(dispatchPath), "other-adoption.json")
		}},
		{name: "adoption hash", mutate: func(value *CompletionReceipt) { value.OwnerAdoptionSHA256 = strings.Repeat("d", 64) }},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			drifted := completion
			tt.mutate(&drifted)
			if err := ValidateCompletionDispatchLineage(drifted, dispatch, dispatchPath, dispatchSHA256); err == nil {
				t.Fatal("drifted completion lineage was accepted")
			}
		})
	}
}
