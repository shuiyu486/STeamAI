package reviewersession

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/shuiyu486/re-context-kits/internal/rekit/capabilitycontract"
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

func TestCanonicalDispatchPromptRequiresExactPacketBindings(t *testing.T) {
	caseRoot := t.TempDir()
	packetRel := ".rekit/reviews/packet/packet.json"
	promptRel := ".rekit/reviews/packet/prompts/shard-01.prompt.md"
	packetPath := filepath.Join(caseRoot, filepath.FromSlash(packetRel))
	promptPath := filepath.Join(caseRoot, filepath.FromSlash(promptRel))
	if err := os.MkdirAll(filepath.Dir(promptPath), 0o700); err != nil {
		t.Fatal(err)
	}
	prompt := "canonical packet-owned reviewer prompt\n"
	promptSHA := sha256Hex([]byte(prompt))
	binding := map[string]any{
		"shardId": "shard-01", "items": []string{"item-a"},
		"dispatchPrompt": strings.TrimSuffix(prompt, "\n"), "dispatchPromptPath": promptRel, "dispatchPromptSha256": promptSHA,
		"agentToolRequest": map[string]any{
			"tool": "Claude Code Agent", "agentType": "read-only-reviewer", "readOnly": true,
			"prompt": strings.TrimSuffix(prompt, "\n"), "promptPath": promptRel, "promptSha256": promptSHA,
		},
	}
	packet := map[string]any{
		"packetId": "packet-a", "command": "plan-subagents",
		"route":                 map[string]any{"id": "route-a", "outputContract": "item,decision"},
		"shards":                []any{map[string]any{"id": "shard-01", "items": []string{"item-a"}}},
		"shardHandoffs":         []any{binding},
		"reviewerOrchestration": map[string]any{"dispatches": []any{binding}},
		"outputContract":        "item,decision",
	}
	data, err := json.Marshal(packet)
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(packetPath, data, 0o600); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(promptPath, []byte(prompt), 0o600); err != nil {
		t.Fatal(err)
	}
	receipt := DispatchReceipt{
		PacketID: "packet-a", PacketPath: packetRel, PacketSHA256: sha256Hex(data), RouteID: "route-a",
		ShardID: "shard-01", Items: []string{"item-a"}, PromptPath: promptRel, PromptSHA256: promptSHA,
	}
	got, ok, err := CanonicalDispatchPrompt(caseRoot, receipt)
	if err != nil || !ok || got != prompt {
		t.Fatalf("canonical prompt=%q ok=%t err=%v", got, ok, err)
	}

	for _, drift := range []struct {
		name   string
		mutate func(map[string]any)
	}{
		{name: "custom packet", mutate: func(value map[string]any) { value["command"] = "custom" }},
		{name: "handoff prompt", mutate: func(value map[string]any) {
			value["shardHandoffs"].([]any)[0].(map[string]any)["dispatchPrompt"] = "drift"
		}},
		{name: "dispatch agent", mutate: func(value map[string]any) {
			value["reviewerOrchestration"].(map[string]any)["dispatches"].([]any)[0].(map[string]any)["agentToolRequest"].(map[string]any)["agentType"] = "custom"
		}},
	} {
		t.Run(drift.name, func(t *testing.T) {
			var changed map[string]any
			if err := json.Unmarshal(data, &changed); err != nil {
				t.Fatal(err)
			}
			drift.mutate(changed)
			changedData, err := json.Marshal(changed)
			if err != nil {
				t.Fatal(err)
			}
			if err := os.WriteFile(packetPath, changedData, 0o600); err != nil {
				t.Fatal(err)
			}
			changedReceipt := receipt
			changedReceipt.PacketSHA256 = sha256Hex(changedData)
			if got, ok, err := CanonicalDispatchPrompt(caseRoot, changedReceipt); err != nil || ok || got != "" {
				t.Fatalf("drifted prompt=%q ok=%t err=%v", got, ok, err)
			}
			if err := os.WriteFile(packetPath, data, 0o600); err != nil {
				t.Fatal(err)
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

func TestDecodeDispatchAndCompletionRejectCapabilityGrant(t *testing.T) {
	dispatch := DispatchReceipt{
		SchemaVersion: 1, Kind: "reviewer-session-dispatch", DispatchID: "dispatch-a", PacketID: "packet-a", RouteID: "route-a", ShardID: "shard-a",
		PromptSHA256: strings.Repeat("a", 64), ReviewerHarness: "harness", ReviewerSession: "session", ReadOnly: true,
		RecordedAt: "2026-08-18T00:00:00Z", Capability: ReadOnlyCapability(), NoSpawn: true, NoHeavyTool: true, NoAuthority: true,
	}
	dispatch.Capability.Contract.NoAuthority = false
	data, err := json.Marshal(dispatch)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := DecodeDispatch(data); err == nil || !strings.Contains(err.Error(), "authority") {
		t.Fatalf("Reviewer dispatch capability grant accepted: %v", err)
	}

	completion := CompletionReceipt{
		SchemaVersion: 1, Kind: "reviewer-session-completion", DispatchID: "dispatch-a", DispatchReceiptPath: "dispatch.json", DispatchReceiptSHA256: strings.Repeat("b", 64),
		PacketID: "packet-a", RouteID: "route-a", ShardID: "shard-a", ReviewerHarness: "harness", ReviewerSession: "session", Outcome: "failed", ExitStatus: "1",
		RecordedAt: "2026-08-18T00:01:00Z", Capability: ReadOnlyCapability(), NoCollection: true, NoIntake: true, NoFacts: true, NoHeavyTool: true, NoAuthority: true,
	}
	completion.Capability.Contract.AuthorizedGate = true
	data, err = json.Marshal(completion)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := DecodeCompletion(data); err == nil || !strings.Contains(err.Error(), "authorized gate") {
		t.Fatalf("Reviewer completion capability grant accepted: %v", err)
	}
}

func TestValidateCompletionDispatchLineageRejectsDrift(t *testing.T) {
	dispatchPath := filepath.Join(t.TempDir(), "dispatch.json")
	dispatchSHA256 := strings.Repeat("a", 64)
	owner := Owner{CurrentExecutor: "session-a", ExecutorGeneration: 2, BindingMode: "durable-lane-owner"}
	capability, err := capabilitycontract.Bind(capabilitycontract.ReadOnly())
	if err != nil {
		t.Fatal(err)
	}
	dispatch := DispatchReceipt{
		DispatchID:          "dispatch-a",
		Capability:          capability,
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
		Capability:            capability,
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
		{name: "capability hash", mutate: func(value *CompletionReceipt) { value.Capability.SHA256 = strings.Repeat("e", 64) }},
		{name: "capability policy", mutate: func(value *CompletionReceipt) {
			value.Capability, _ = capabilitycontract.Bind(capabilitycontract.Transport())
		}},
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
