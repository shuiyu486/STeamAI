package reviewersession

import (
	"path/filepath"
	"strings"
	"testing"
)

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
