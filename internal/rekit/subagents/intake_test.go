package subagents

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"

	"github.com/shuiyu486/re-context-kits/internal/rekit/defaults"
	"github.com/shuiyu486/re-context-kits/internal/rekit/mission"
	"github.com/shuiyu486/re-context-kits/internal/rekit/note"
	syncreview "github.com/shuiyu486/re-context-kits/internal/rekit/sync"
	"github.com/shuiyu486/re-context-kits/internal/rekit/workstream"
)

func TestDecodeReviewerResultStrictContract(t *testing.T) {
	valid := reviewerResultJSON("packet-fixture", "route-fixture", "accept", "accepted", nil)
	var missingRouteFields map[string]any
	if err := json.Unmarshal(valid, &missingRouteFields); err != nil {
		t.Fatal(err)
	}
	delete(missingRouteFields, "routeOutput")
	missingRouteOutput, err := json.Marshal(missingRouteFields)
	if err != nil {
		t.Fatal(err)
	}
	result, err := decodeReviewerResult(valid)
	if err != nil {
		t.Fatal(err)
	}
	if result.ShardID != "shard-01" || result.Decision != "accept" || result.RecommendedVerdict != "accepted" || result.RouteOutput["item"] != "alpha" {
		t.Fatalf("unexpected reviewer result: %+v", result)
	}
	for _, tc := range []struct {
		name string
		data string
		want string
	}{
		{name: "missing field", data: `{"shardId":"shard-01"}`, want: "missing required field"},
		{name: "unknown field", data: strings.TrimSuffix(string(valid), "}") + `,"writes":["x"]}`, want: "unknown field"},
		{name: "trailing object", data: string(valid) + ` {}`, want: "exactly one JSON object"},
		{name: "missing route output", data: string(missingRouteOutput), want: "missing required field"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			_, err := decodeReviewerResult([]byte(tc.data))
			if err == nil || !strings.Contains(err.Error(), tc.want) {
				t.Fatalf("error = %v, want %q", err, tc.want)
			}
		})
	}
}

func TestReviewerIntakeBlockersFailClosed(t *testing.T) {
	repoRoot := filepath.Clean(filepath.Join("..", "..", ".."))
	caseRoot := filepath.Join(t.TempDir(), "case")
	writeReviewerIntakeCase(t, repoRoot, caseRoot)
	plan, err := WritePlan(repoRoot, caseRoot, defaults.DefaultPack, Options{TaskType: "feature-analysis", Items: "alpha"})
	if err != nil {
		t.Fatal(err)
	}
	packet := readReviewerPacket(t, plan.PacketPath)
	mapping, ok := reviewerDecisionMappingByDecision("accept")
	if !ok {
		t.Fatal("missing accept mapping")
	}
	result, err := decodeReviewerResult(reviewerResultForPacket(t, packet, "accept", "rejected", []string{"overlaps shard-02"}))
	if err != nil {
		t.Fatal(err)
	}
	result.Confidence = "low"
	result.RouteOutput["next_action"] = "run debugger"
	blocked, err := reviewerIntakeBlockers(repoRoot, caseRoot, packet, result, mapping)
	if err != nil {
		t.Fatal(err)
	}
	for _, marker := range []string{"unresolved conflicts", "blocked write", "conflicts with mapped", "low-confidence"} {
		if !slices.ContainsFunc(blocked, func(item string) bool { return strings.Contains(item, marker) }) {
			t.Fatalf("blockers missing %q: %v", marker, blocked)
		}
	}
}

func TestReviewerIntakeIDCanonicalizesEquivalentJSON(t *testing.T) {
	packet := Packet{PacketID: "packet-fixture"}
	resultA, err := decodeReviewerResult(reviewerResultJSON("packet-fixture", "route-fixture", "accept", "accepted", nil))
	if err != nil {
		t.Fatal(err)
	}
	var fields map[string]any
	if err := json.Unmarshal(reviewerResultJSON("packet-fixture", "route-fixture", "accept", "accepted", nil), &fields); err != nil {
		t.Fatal(err)
	}
	reordered, err := json.MarshalIndent(fields, "", "  ")
	if err != nil {
		t.Fatal(err)
	}
	resultB, err := decodeReviewerResult(append(reordered, '\n'))
	if err != nil {
		t.Fatal(err)
	}
	if left, right := reviewerIntakeID(packet, resultA, " feature-review "), reviewerIntakeID(packet, resultB, "feature-review"); left != right {
		t.Fatalf("equivalent reviewer JSON changed intake ID: %s != %s", left, right)
	}
}

func TestReviewerIntakeBlockedActionScanIgnoresEvidencePathAndSummaryWords(t *testing.T) {
	repoRoot := filepath.Clean(filepath.Join("..", "..", ".."))
	packet := Packet{Pack: defaults.DefaultPack, Route: Route{SubagentPermissions: "read-only"}}
	result := ReviewerResult{Summary: "confirmed evidence preserves authority metadata as read-only context", EvidenceRefs: []string{"workspace/authority/confirmed-review.md"}, RouteOutput: map[string]any{"tool_scope": "read-only", "next_action": "main-agent review"}}
	if reviewerResultRequestsBlockedAction(repoRoot, packet, result) {
		t.Fatalf("descriptive summary/evidence path caused a blocked-action false positive: %+v", result)
	}
	result.RouteOutput["next_action"] = "append ledger directly"
	if !reviewerResultRequestsBlockedAction(repoRoot, packet, result) {
		t.Fatalf("structured requested action was not blocked: %+v", result.RouteOutput)
	}
}

func TestIntakeReadyReviewerResultsPreviewsAndAppliesAllReadyShards(t *testing.T) {
	repoRoot := filepath.Clean(filepath.Join("..", "..", ".."))
	caseRoot := filepath.Join(t.TempDir(), "case")
	writeReviewerIntakeCase(t, repoRoot, caseRoot)
	plan, err := WritePlan(repoRoot, caseRoot, defaults.DefaultPack, Options{TaskType: "feature-analysis", Items: "alpha,beta", ItemsPerAgent: 1, MaxParallel: 2, Lane: "feature-intake"})
	if err != nil {
		t.Fatal(err)
	}
	packet := readReviewerPacket(t, plan.PacketPath)
	if len(packet.ShardHandoffs) != 2 {
		t.Fatalf("reviewer batch fixture shard count = %d, want 2", len(packet.ShardHandoffs))
	}
	evidencePath := filepath.Join(caseRoot, "workspace", "review-evidence.md")
	if err := os.MkdirAll(filepath.Dir(evidencePath), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(evidencePath, []byte("bounded reviewer batch evidence\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	for i, handoff := range packet.ShardHandoffs {
		value := ReviewerResult{
			PacketID: packet.PacketID, RouteID: packet.Route.ID, ShardID: handoff.ShardID, Items: append([]string{}, handoff.Items...),
			ReviewerSession: fmt.Sprintf("reviewer-batch-%d", i+1), Decision: "accept", Confidence: "high",
			Summary: "reviewed " + handoff.ShardID + " against bounded evidence", EvidenceRefs: []string{"workspace/review-evidence.md"}, Risks: []string{}, Conflicts: []string{}, RecommendedVerdict: "accepted",
			RouteOutput: map[string]any{"item": strings.Join(handoff.Items, ","), "decision": "accept", "confidence": "high", "evidence": "workspace/review-evidence.md", "risk": "low", "next_action": "main-agent-writeback", "tier_used": "light", "tool_scope": "read-only", "feature": "intake", "request_id": "n/a", "candidate_path": "n/a", "defer_reason": "n/a"},
		}
		data, err := json.Marshal(value)
		if err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(handoff.ReviewerResultPath, data, 0o644); err != nil {
			t.Fatal(err)
		}
	}

	preview, err := IntakeReadyReviewerResults(repoRoot, caseRoot, defaults.DefaultPack, ReviewerBatchIntakeOptions{PacketPath: plan.PacketPath, Lane: "feature-intake", Actor: "mission-commander", WhatIf: true})
	if err != nil {
		t.Fatal(err)
	}
	if preview.Mode != "reviewer-batch-intake" || preview.IsMutation || preview.Applied || preview.Total != 2 || preview.Ready != 2 || preview.Waiting != 0 || preview.Processed != 2 || preview.Stopped || len(preview.Results) != 2 || preview.Results[0].WritebackStatus != "previewed" || preview.Results[1].WritebackStatus != "previewed" {
		t.Fatalf("unexpected reviewer batch preview: %+v", preview)
	}
	if got := readOptionalFile(t, filepath.Join(caseRoot, ".rekit", "facts", "verifications.jsonl")); got != "" {
		t.Fatalf("reviewer batch WhatIf wrote verification ledger:\n%s", got)
	}

	applied, err := IntakeReadyReviewerResults(repoRoot, caseRoot, defaults.DefaultPack, ReviewerBatchIntakeOptions{PacketPath: plan.PacketPath, Lane: "feature-intake", Actor: "mission-commander"})
	if err != nil {
		t.Fatal(err)
	}
	if !applied.IsMutation || !applied.Applied || applied.Processed != 2 || applied.Completed != 2 || applied.AlreadyComplete != 0 || applied.Stopped || len(applied.Results) != 2 || applied.Results[0].WritebackStatus != "complete" || applied.Results[1].WritebackStatus != "complete" {
		t.Fatalf("unexpected reviewer batch apply: %+v", applied)
	}
	if got := strings.Count(readOptionalFile(t, filepath.Join(caseRoot, ".rekit", "facts", "verifications.jsonl")), `"shardId"`); got != 2 {
		t.Fatalf("verification shard writeback count = %d, want 2", got)
	}
	if got := strings.Count(readOptionalFile(t, filepath.Join(caseRoot, ".rekit", "facts", "decisions.jsonl")), `"shardId"`); got != 2 {
		t.Fatalf("decision shard writeback count = %d, want 2", got)
	}

	replay, err := IntakeReadyReviewerResults(repoRoot, caseRoot, defaults.DefaultPack, ReviewerBatchIntakeOptions{PacketPath: plan.PacketPath, Lane: "feature-intake", Actor: "mission-commander"})
	if err != nil {
		t.Fatal(err)
	}
	if replay.Processed != 2 || replay.Completed != 0 || replay.AlreadyComplete != 2 || replay.Applied || replay.Stopped {
		t.Fatalf("reviewer batch replay was not idempotent: %+v", replay)
	}
}

func TestIntakeReadyReviewerResultsBindsPathToShardAndPreservesWaiting(t *testing.T) {
	repoRoot := filepath.Clean(filepath.Join("..", "..", ".."))
	caseRoot := filepath.Join(t.TempDir(), "case")
	writeReviewerIntakeCase(t, repoRoot, caseRoot)
	plan, err := WritePlan(repoRoot, caseRoot, defaults.DefaultPack, Options{TaskType: "feature-analysis", Items: "alpha,beta", ItemsPerAgent: 1, MaxParallel: 2, Lane: "feature-intake"})
	if err != nil {
		t.Fatal(err)
	}
	packet := readReviewerPacket(t, plan.PacketPath)
	evidencePath := filepath.Join(caseRoot, "workspace", "review-evidence.md")
	if err := os.MkdirAll(filepath.Dir(evidencePath), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(evidencePath, []byte("bounded evidence\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	first, second := packet.ShardHandoffs[0], packet.ShardHandoffs[1]
	swapped := ReviewerResult{PacketID: packet.PacketID, RouteID: packet.Route.ID, ShardID: second.ShardID, Items: append([]string{}, second.Items...), ReviewerSession: "reviewer-swapped", Decision: "accept", Confidence: "high", Summary: "wrong path binding", EvidenceRefs: []string{"workspace/review-evidence.md"}, Risks: []string{}, Conflicts: []string{}, RecommendedVerdict: "accepted", RouteOutput: map[string]any{"item": strings.Join(second.Items, ","), "decision": "accept", "confidence": "high", "evidence": "workspace/review-evidence.md", "risk": "low", "next_action": "main-agent-writeback", "tier_used": "light", "tool_scope": "read-only", "feature": "intake", "request_id": "n/a", "candidate_path": "n/a", "defer_reason": "n/a"}}
	data, err := json.Marshal(swapped)
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(first.ReviewerResultPath, data, 0o644); err != nil {
		t.Fatal(err)
	}
	result, err := IntakeReadyReviewerResults(repoRoot, caseRoot, defaults.DefaultPack, ReviewerBatchIntakeOptions{PacketPath: plan.PacketPath, Lane: "feature-intake", Actor: "mission-commander"})
	if err == nil || !strings.Contains(err.Error(), "does not match expected packet handoff shard") || !result.Stopped || result.StopShardID != first.ShardID {
		t.Fatalf("swapped reviewer result did not fail closed: result=%+v err=%v", result, err)
	}
	if got := readOptionalFile(t, filepath.Join(caseRoot, ".rekit", "facts", "verifications.jsonl")); got != "" {
		t.Fatalf("swapped reviewer result wrote verification ledger:\n%s", got)
	}

	valid := swapped
	valid.ShardID = first.ShardID
	valid.Items = append([]string{}, first.Items...)
	valid.RouteOutput["item"] = strings.Join(first.Items, ",")
	data, err = json.Marshal(valid)
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(first.ReviewerResultPath, data, 0o644); err != nil {
		t.Fatal(err)
	}
	waiting, err := IntakeReadyReviewerResults(repoRoot, caseRoot, defaults.DefaultPack, ReviewerBatchIntakeOptions{PacketPath: plan.PacketPath, Lane: "feature-intake", Actor: "mission-commander"})
	if err != nil {
		t.Fatal(err)
	}
	if waiting.Ready != 1 || waiting.Waiting != 1 || waiting.Completed != 1 || waiting.Stopped || len(waiting.NextSteps) != 1 || !strings.Contains(waiting.NextSteps[0], "collect the remaining reviewer result JSON") {
		t.Fatalf("partial ready batch omitted waiting guidance: %+v", waiting)
	}
}

func TestIntakeReadyReviewerResultsStopsBeforeLaterShardOnBlocker(t *testing.T) {
	repoRoot := filepath.Clean(filepath.Join("..", "..", ".."))
	caseRoot := filepath.Join(t.TempDir(), "case")
	writeReviewerIntakeCase(t, repoRoot, caseRoot)
	plan, err := WritePlan(repoRoot, caseRoot, defaults.DefaultPack, Options{TaskType: "feature-analysis", Items: "alpha,beta", ItemsPerAgent: 1, MaxParallel: 2, Lane: "feature-intake"})
	if err != nil {
		t.Fatal(err)
	}
	packet := readReviewerPacket(t, plan.PacketPath)
	evidencePath := filepath.Join(caseRoot, "workspace", "review-evidence.md")
	if err := os.MkdirAll(filepath.Dir(evidencePath), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(evidencePath, []byte("bounded evidence\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	for i, handoff := range packet.ShardHandoffs {
		conflicts := []string{}
		if i == 1 {
			conflicts = []string{"unresolved batch conflict"}
		}
		value := ReviewerResult{PacketID: packet.PacketID, RouteID: packet.Route.ID, ShardID: handoff.ShardID, Items: append([]string{}, handoff.Items...), ReviewerSession: fmt.Sprintf("reviewer-block-%d", i+1), Decision: "accept", Confidence: "high", Summary: "batch blocker fixture", EvidenceRefs: []string{"workspace/review-evidence.md"}, Risks: []string{}, Conflicts: conflicts, RecommendedVerdict: "accepted", RouteOutput: map[string]any{"item": strings.Join(handoff.Items, ","), "decision": "accept", "confidence": "high", "evidence": "workspace/review-evidence.md", "risk": "low", "next_action": "main-agent-writeback", "tier_used": "light", "tool_scope": "read-only", "feature": "intake", "request_id": "n/a", "candidate_path": "n/a", "defer_reason": "n/a"}}
		data, err := json.Marshal(value)
		if err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(handoff.ReviewerResultPath, data, 0o644); err != nil {
			t.Fatal(err)
		}
	}
	result, err := IntakeReadyReviewerResults(repoRoot, caseRoot, defaults.DefaultPack, ReviewerBatchIntakeOptions{PacketPath: plan.PacketPath, Lane: "feature-intake", Actor: "mission-commander"})
	if err != nil {
		t.Fatal(err)
	}
	if !result.Stopped || result.StopShardID != "shard-02" || result.Processed != 2 || result.Completed != 1 || len(result.Results) != 2 || result.Results[0].WritebackStatus != "complete" || result.Results[1].WritebackStatus != "blocked" || !strings.Contains(result.StopReason, "writebackStatus=blocked") {
		t.Fatalf("reviewer batch did not stop on second-shard blocker: %+v", result)
	}
	if got := strings.Count(readOptionalFile(t, filepath.Join(caseRoot, ".rekit", "facts", "verifications.jsonl")), `"shardId"`); got != 1 {
		t.Fatalf("verification shard writeback count = %d, want only completed first shard", got)
	}
	if got := strings.Count(readOptionalFile(t, filepath.Join(caseRoot, ".rekit", "facts", "decisions.jsonl")), `"shardId"`); got != 1 {
		t.Fatalf("decision shard writeback count = %d, want only completed first shard", got)
	}
}

func TestIntakeReviewerResultRejectsWrongPacketAndRouteBindings(t *testing.T) {
	repoRoot := filepath.Clean(filepath.Join("..", "..", ".."))
	caseRoot := filepath.Join(t.TempDir(), "case")
	writeReviewerIntakeCase(t, repoRoot, caseRoot)
	plan, err := WritePlan(repoRoot, caseRoot, defaults.DefaultPack, Options{TaskType: "feature-analysis", Items: "alpha"})
	if err != nil {
		t.Fatal(err)
	}
	packet := readReviewerPacket(t, plan.PacketPath)
	for _, tc := range []struct {
		name     string
		packetID string
		routeID  string
		want     string
	}{
		{name: "wrong packet", packetID: "packet-stale", routeID: packet.Route.ID, want: "does not match packet"},
		{name: "wrong route", packetID: packet.PacketID, routeID: "other:route", want: "does not match packet route"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			resultPath := filepath.Join(t.TempDir(), "reviewer-result.json")
			if err := os.WriteFile(resultPath, reviewerResultJSON(tc.packetID, tc.routeID, "accept", "accepted", nil), 0o644); err != nil {
				t.Fatal(err)
			}
			_, err := IntakeReviewerResult(repoRoot, caseRoot, defaults.DefaultPack, ReviewerIntakeOptions{PacketPath: plan.PacketPath, ReviewerResultPath: resultPath, Lane: "devirt-main", Actor: "mission-commander", WhatIf: true})
			if err == nil || !strings.Contains(err.Error(), tc.want) {
				t.Fatalf("error = %v, want %q", err, tc.want)
			}
			if got := readOptionalFile(t, filepath.Join(caseRoot, ".rekit", "facts", "verifications.jsonl")); got != "" {
				t.Fatalf("invalid binding wrote verification ledger:\n%s", got)
			}
			if got := readOptionalFile(t, filepath.Join(caseRoot, ".rekit", "facts", "decisions.jsonl")); got != "" {
				t.Fatalf("invalid binding wrote decision ledger:\n%s", got)
			}
		})
	}
}

func TestIntakeReviewerResultBlocksInvalidEvidenceRefs(t *testing.T) {
	repoRoot := filepath.Clean(filepath.Join("..", "..", ".."))
	caseRoot := filepath.Join(t.TempDir(), "case")
	writeReviewerIntakeCase(t, repoRoot, caseRoot)
	plan, err := WritePlan(repoRoot, caseRoot, defaults.DefaultPack, Options{TaskType: "feature-analysis", Items: "alpha"})
	if err != nil {
		t.Fatal(err)
	}
	packet := readReviewerPacket(t, plan.PacketPath)
	for _, ref := range []string{"n/a", "workspace/missing-evidence.md", filepath.Join(t.TempDir(), "outside.md"), "../escape.md", "workspace"} {
		t.Run(strings.ReplaceAll(ref, string(filepath.Separator), "_"), func(t *testing.T) {
			value, err := decodeReviewerResult(reviewerResultForPacket(t, packet, "accept", "accepted", nil))
			if err != nil {
				t.Fatal(err)
			}
			value.EvidenceRefs = []string{ref}
			value.RouteOutput["evidence"] = ref
			data, err := json.Marshal(value)
			if err != nil {
				t.Fatal(err)
			}
			resultPath := filepath.Join(t.TempDir(), "reviewer-result.json")
			if err := os.WriteFile(resultPath, data, 0o644); err != nil {
				t.Fatal(err)
			}
			result, err := IntakeReviewerResult(repoRoot, caseRoot, defaults.DefaultPack, ReviewerIntakeOptions{PacketPath: plan.PacketPath, ReviewerResultPath: resultPath, Lane: "devirt-main", Actor: "mission-commander"})
			if err != nil {
				t.Fatal(err)
			}
			if result.WritebackStatus != "blocked" || result.ReadyForWriteback || result.Applied || len(result.BlockedReasons) == 0 {
				t.Fatalf("invalid evidence ref did not fail closed: %+v", result)
			}
		})
	}
}

func TestIntakeReviewerResultRejectsRouteOutputContractViolations(t *testing.T) {
	repoRoot := filepath.Clean(filepath.Join("..", "..", ".."))
	caseRoot := filepath.Join(t.TempDir(), "case")
	writeReviewerIntakeCase(t, repoRoot, caseRoot)
	plan, err := WritePlan(repoRoot, caseRoot, defaults.DefaultPack, Options{TaskType: "feature-analysis", Items: "alpha"})
	if err != nil {
		t.Fatal(err)
	}
	packet := readReviewerPacket(t, plan.PacketPath)
	for _, tc := range []struct {
		name   string
		mutate func(*ReviewerResult)
		want   string
	}{
		{name: "unknown field", mutate: func(result *ReviewerResult) { result.RouteOutput["execute"] = "patch binary" }, want: "unknown field"},
		{name: "empty field", mutate: func(result *ReviewerResult) { result.RouteOutput["item"] = "" }, want: "non-empty string"},
		{name: "wrong type", mutate: func(result *ReviewerResult) { result.RouteOutput["item"] = map[string]any{"value": "alpha"} }, want: "non-empty string"},
		{name: "decision mismatch", mutate: func(result *ReviewerResult) { result.RouteOutput["decision"] = "reject" }, want: "does not match top-level value"},
		{name: "confidence mismatch", mutate: func(result *ReviewerResult) { result.RouteOutput["confidence"] = "low" }, want: "does not match top-level value"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			value, err := decodeReviewerResult(reviewerResultForPacket(t, packet, "accept", "accepted", nil))
			if err != nil {
				t.Fatal(err)
			}
			tc.mutate(&value)
			data, err := json.Marshal(value)
			if err != nil {
				t.Fatal(err)
			}
			resultPath := filepath.Join(t.TempDir(), "reviewer-result.json")
			if err := os.WriteFile(resultPath, data, 0o644); err != nil {
				t.Fatal(err)
			}
			_, err = IntakeReviewerResult(repoRoot, caseRoot, defaults.DefaultPack, ReviewerIntakeOptions{PacketPath: plan.PacketPath, ReviewerResultPath: resultPath, Lane: "devirt-main", Actor: "mission-commander", WhatIf: true})
			if err == nil || !strings.Contains(err.Error(), tc.want) {
				t.Fatalf("error = %v, want %q", err, tc.want)
			}
		})
	}
}

func TestIntakeReviewerResultRejectsStaleOwnerBinding(t *testing.T) {
	repoRoot := filepath.Clean(filepath.Join("..", "..", ".."))
	caseRoot := filepath.Join(t.TempDir(), "case")
	writeReviewerIntakeCase(t, repoRoot, caseRoot)
	plan, err := WritePlan(repoRoot, caseRoot, defaults.DefaultPack, Options{TaskType: "feature-analysis", Items: "alpha", Lane: "feature-intake"})
	if err != nil {
		t.Fatal(err)
	}
	packet := readReviewerPacket(t, plan.PacketPath)
	if !packet.OwnerBinding.RequiredForIntake || packet.OwnerBinding.CurrentExecutor != "session-main" {
		t.Fatalf("plan did not bind owner executor: %+v", packet.OwnerBinding)
	}
	root, err := filepath.Abs(repoRoot)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := workstream.StartApply(root, caseRoot, defaults.DefaultPack, workstream.StartOptions{Name: "intake", Executor: "session-replacement", Actor: "mission-commander", TakeoverReason: "replace reviewer owner"}); err != nil {
		t.Fatal(err)
	}
	resultPath := filepath.Join(t.TempDir(), "reviewer-result.json")
	if err := os.WriteFile(resultPath, reviewerResultForPlan(t, plan.PacketPath, "accept", "accepted", nil), 0o644); err != nil {
		t.Fatal(err)
	}
	_, err = IntakeReviewerResult(repoRoot, caseRoot, defaults.DefaultPack, ReviewerIntakeOptions{PacketPath: plan.PacketPath, ReviewerResultPath: resultPath, Lane: "feature-intake", Actor: "mission-commander", WhatIf: true})
	if err == nil || !strings.Contains(err.Error(), "ownerBinding is stale") {
		t.Fatalf("error = %v, want stale owner binding rejection", err)
	}
	if got := readOptionalFile(t, filepath.Join(caseRoot, ".rekit", "facts", "verifications.jsonl")); got != "" {
		t.Fatalf("stale owner binding wrote verification ledger:\n%s", got)
	}
}

func TestIntakeReviewerResultRejectsTamperedPacket(t *testing.T) {
	repoRoot := filepath.Clean(filepath.Join("..", "..", ".."))
	caseRoot := filepath.Join(t.TempDir(), "case")
	writeReviewerIntakeCase(t, repoRoot, caseRoot)
	plan, err := WritePlan(repoRoot, caseRoot, defaults.DefaultPack, Options{TaskType: "feature-analysis", Items: "alpha"})
	if err != nil {
		t.Fatal(err)
	}
	packetBytes, err := os.ReadFile(plan.PacketPath)
	if err != nil {
		t.Fatal(err)
	}
	var packet map[string]any
	if err := json.Unmarshal(packetBytes, &packet); err != nil {
		t.Fatal(err)
	}
	packet["outputContract"] = "item"
	tampered, err := json.Marshal(packet)
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(plan.PacketPath, tampered, 0o644); err != nil {
		t.Fatal(err)
	}
	resultPath := filepath.Join(t.TempDir(), "reviewer-result.json")
	if err := os.WriteFile(resultPath, reviewerResultForPlan(t, plan.PacketPath, "accept", "accepted", nil), 0o644); err != nil {
		t.Fatal(err)
	}
	_, err = IntakeReviewerResult(repoRoot, caseRoot, defaults.DefaultPack, ReviewerIntakeOptions{PacketPath: plan.PacketPath, ReviewerResultPath: resultPath, Lane: "devirt-main", Actor: "mission-commander", WhatIf: true})
	if err == nil || !strings.Contains(err.Error(), "not a supported non-mutating") {
		t.Fatalf("error = %v, want packet identity rejection", err)
	}
}

func TestIntakeReviewerResultWhatIfAndApply(t *testing.T) {
	repoRoot := filepath.Clean(filepath.Join("..", "..", ".."))
	caseRoot := filepath.Join(t.TempDir(), "case")
	writeReviewerIntakeCase(t, repoRoot, caseRoot)
	plan, err := WritePlan(repoRoot, caseRoot, defaults.DefaultPack, Options{TaskType: "feature-analysis", Items: "alpha"})
	if err != nil {
		t.Fatal(err)
	}
	resultPath := filepath.Join(t.TempDir(), "reviewer-result.json")
	if err := os.WriteFile(resultPath, reviewerResultForPlan(t, plan.PacketPath, "accept", "accepted", nil), 0o644); err != nil {
		t.Fatal(err)
	}
	beforeVerifications := readOptionalFile(t, filepath.Join(caseRoot, ".rekit", "facts", "verifications.jsonl"))
	beforeDecisions := readOptionalFile(t, filepath.Join(caseRoot, ".rekit", "facts", "decisions.jsonl"))

	preview, err := IntakeReviewerResult(repoRoot, caseRoot, defaults.DefaultPack, ReviewerIntakeOptions{PacketPath: plan.PacketPath, ReviewerResultPath: resultPath, Lane: "devirt-main", Actor: "mission-commander", WhatIf: true})
	if err != nil {
		t.Fatal(err)
	}
	if preview.Command != commandName || preview.Mode != "reviewer-intake" || preview.IsMutation || preview.Applied || preview.WritebackStatus != "previewed" || !preview.ReadyForWriteback || !preview.ReviewRequired || preview.Verification == nil || preview.Decision == nil || preview.PostValidation == nil || !preview.PostValidation.Valid {
		t.Fatalf("unexpected intake preview: %+v", preview)
	}
	if preview.Verification.Applied || preview.Decision.Applied || preview.Verification.Reason != "what-if" || preview.Decision.Reason != "what-if" || preview.Verification.Event["verdict"] != "accepted" || preview.Decision.Event["decision"] != "accept" || preview.Verification.Event["reviewerSession"] != "reviewer-session-1" || preview.Verification.Event["ownerBindingMode"] != "unassigned-lane" || preview.Verification.Event["ownerBindingTarget"] != "devirt-main" {
		t.Fatalf("unexpected writeback previews: verification=%+v decision=%+v", preview.Verification, preview.Decision)
	}
	if preview.MissionCommanderAction.State != "ready-for-reviewer-intake-apply" || !strings.Contains(preview.MissionCommanderAction.PrimaryCommand, "-Apply -Format json") || !hasReviewerIntakeCommanderNextAction(preview.MissionCommanderNextActions, "reviewerIntake.previewed", preview.MissionCommanderAction.PrimaryCommand, false, true) || !hasReviewerIntakeCommanderNextAction(preview.MissionCommanderNextActions, "reviewerIntake.previewed.followUp", "/rekit handoff devirt-main", false, true) {
		t.Fatalf("preview omitted reviewer intake Mission Commander apply guidance: action=%+v next=%+v", preview.MissionCommanderAction, preview.MissionCommanderNextActions)
	}
	assertReviewerIntakeActionQueue(t, preview.MissionCommanderActionQueue, 2, 2, 0, 2, 1, preview.MissionCommanderAction.PrimaryCommand)
	if preview.OrchestrationSnapshot.Mode != "manual-main-agent-intake" || preview.OrchestrationSnapshot.DispatchIndex != 1 || preview.OrchestrationSnapshot.DispatchTotal != 1 || preview.OrchestrationSnapshot.ShardStatusAfter != "previewed" || !preview.OrchestrationSnapshot.PreviewRequiredFirst || !slices.Contains(preview.OrchestrationSnapshot.RuntimeBoundary, "runtime does not spawn subagents") {
		t.Fatalf("unexpected orchestration snapshot: %+v", preview.OrchestrationSnapshot)
	}
	if got := readOptionalFile(t, filepath.Join(caseRoot, ".rekit", "facts", "verifications.jsonl")); got != beforeVerifications {
		t.Fatalf("preview changed verifications ledger:\n%s", got)
	}
	if got := readOptionalFile(t, filepath.Join(caseRoot, ".rekit", "facts", "decisions.jsonl")); got != beforeDecisions {
		t.Fatalf("preview changed decisions ledger:\n%s", got)
	}

	applied, err := IntakeReviewerResult(repoRoot, caseRoot, defaults.DefaultPack, ReviewerIntakeOptions{PacketPath: plan.PacketPath, ReviewerResultPath: resultPath, Lane: "devirt-main", Actor: "mission-commander"})
	if err != nil {
		t.Fatal(err)
	}
	if !applied.IsMutation || !applied.Applied || applied.WritebackStatus != "complete" || !applied.ReadyForWriteback || applied.Verification == nil || applied.Decision == nil || !applied.Verification.Applied || !applied.Decision.Applied || applied.PostValidation == nil || !applied.PostValidation.Valid {
		t.Fatalf("unexpected intake apply: %+v", applied)
	}
	if applied.MissionCommanderAction.State != "reviewer-intake-writeback-complete" || len(applied.MissionCommanderNextActions) == 0 || !strings.HasPrefix(applied.MissionCommanderNextActions[0].Source, "reviewerIntake.postValidation.") {
		t.Fatalf("apply omitted post-validation Mission Commander guidance: action=%+v next=%+v", applied.MissionCommanderAction, applied.MissionCommanderNextActions)
	}
	if applied.MissionCommanderActionQueue.Counts.Total != len(applied.MissionCommanderNextActions) || applied.MissionCommanderActionQueue.CurrentAction == nil || !strings.HasPrefix(applied.MissionCommanderActionQueue.CurrentAction.Source, "reviewerIntake.postValidation.") {
		t.Fatalf("apply omitted post-validation Mission Commander action queue: queue=%+v next=%+v", applied.MissionCommanderActionQueue, applied.MissionCommanderNextActions)
	}
	verificationLedger := readOptionalFile(t, filepath.Join(caseRoot, ".rekit", "facts", "verifications.jsonl"))
	decisionLedger := readOptionalFile(t, filepath.Join(caseRoot, ".rekit", "facts", "decisions.jsonl"))
	if strings.Count(verificationLedger, applied.Verification.EventID) != 1 || strings.Count(decisionLedger, applied.Decision.EventID) != 1 || !strings.Contains(decisionLedger, applied.Verification.EventID) || !strings.Contains(verificationLedger, "reviewer-session-1") || !strings.Contains(verificationLedger, "ownerBindingMode") {
		t.Fatalf("writeback order/evidence/provenance linkage missing:\nverification=%s\ndecision=%s", verificationLedger, decisionLedger)
	}

	duplicate, err := IntakeReviewerResult(repoRoot, caseRoot, defaults.DefaultPack, ReviewerIntakeOptions{PacketPath: plan.PacketPath, ReviewerResultPath: resultPath, Lane: "devirt-main", Actor: "mission-commander"})
	if err != nil {
		t.Fatal(err)
	}
	if duplicate.Applied || duplicate.WritebackStatus != "already-complete" || duplicate.Verification.Reason != "duplicate eventId" || duplicate.Decision.Reason != "duplicate eventId" {
		t.Fatalf("repeat intake should be idempotent: %+v", duplicate)
	}
	if duplicate.MissionCommanderAction.State != "reviewer-intake-already-complete" || len(duplicate.MissionCommanderNextActions) == 0 || !strings.HasPrefix(duplicate.MissionCommanderNextActions[0].Source, "reviewerIntake.postValidation.") {
		t.Fatalf("duplicate omitted already-complete Mission Commander guidance: action=%+v next=%+v", duplicate.MissionCommanderAction, duplicate.MissionCommanderNextActions)
	}
	if duplicate.MissionCommanderActionQueue.Counts.Total != len(duplicate.MissionCommanderNextActions) || duplicate.MissionCommanderActionQueue.CurrentAction == nil || !strings.HasPrefix(duplicate.MissionCommanderActionQueue.CurrentAction.Source, "reviewerIntake.postValidation.") {
		t.Fatalf("duplicate omitted already-complete Mission Commander action queue: queue=%+v next=%+v", duplicate.MissionCommanderActionQueue, duplicate.MissionCommanderNextActions)
	}
}

func TestIntakeReviewerResultBlocksConflictsWithoutWrites(t *testing.T) {
	repoRoot := filepath.Clean(filepath.Join("..", "..", ".."))
	caseRoot := filepath.Join(t.TempDir(), "case")
	writeReviewerIntakeCase(t, repoRoot, caseRoot)
	plan, err := WritePlan(repoRoot, caseRoot, defaults.DefaultPack, Options{TaskType: "feature-analysis", Items: "alpha"})
	if err != nil {
		t.Fatal(err)
	}
	resultPath := filepath.Join(t.TempDir(), "reviewer-result.json")
	if err := os.WriteFile(resultPath, reviewerResultForPlan(t, plan.PacketPath, "accept", "accepted", []string{"overlaps another shard"}), 0o644); err != nil {
		t.Fatal(err)
	}
	result, err := IntakeReviewerResult(repoRoot, caseRoot, defaults.DefaultPack, ReviewerIntakeOptions{PacketPath: plan.PacketPath, ReviewerResultPath: resultPath, Lane: "devirt-main", Actor: "mission-commander"})
	if err != nil {
		t.Fatal(err)
	}
	if result.IsMutation || result.Applied || result.WritebackStatus != "blocked" || result.ReadyForWriteback || !result.ReviewRequired || len(result.BlockedReasons) == 0 || result.Verification != nil || result.Decision != nil || result.PostValidation == nil {
		t.Fatalf("conflicted result did not fail closed: %+v", result)
	}
	if result.MissionCommanderAction.State != "reviewer-intake-blocked" || !hasReviewerIntakeCommanderNextAction(result.MissionCommanderNextActions, "reviewerIntake.blocked", result.MissionCommanderAction.PrimaryCommand, true, true) {
		t.Fatalf("blocked intake omitted blocked Mission Commander preview guidance: action=%+v next=%+v", result.MissionCommanderAction, result.MissionCommanderNextActions)
	}
	if len(result.RepairGuidance) == 0 || !slices.ContainsFunc(result.RepairGuidance, func(item ReviewerIntakeRepairGuidance) bool {
		return strings.Contains(item.Action, "resolve or split") && slices.Contains(item.Evidence, "overlaps another shard") && slices.Contains(item.Boundary, "do not apply reviewer intake until this blocker is resolved")
	}) {
		t.Fatalf("blocked intake omitted conflict repair guidance: %+v", result.RepairGuidance)
	}
	if !slices.ContainsFunc(result.MissionCommanderNextActions[0].Reasons, func(reason string) bool { return strings.Contains(reason, "repair: resolve or split") }) {
		t.Fatalf("blocked intake next action omitted repair reason: %+v", result.MissionCommanderNextActions[0].Reasons)
	}
	assertReviewerIntakeActionQueue(t, result.MissionCommanderActionQueue, 1, 0, 1, 1, 0, result.MissionCommanderAction.PrimaryCommand)
	if got := readOptionalFile(t, filepath.Join(caseRoot, ".rekit", "facts", "verifications.jsonl")); got != "" {
		t.Fatalf("blocked intake wrote verification ledger:\n%s", got)
	}
	if got := readOptionalFile(t, filepath.Join(caseRoot, ".rekit", "facts", "decisions.jsonl")); got != "" {
		t.Fatalf("blocked intake wrote decision ledger:\n%s", got)
	}
}

func TestIntakeReviewerResultRecoversPartialVerificationWrite(t *testing.T) {
	repoRoot := filepath.Clean(filepath.Join("..", "..", ".."))
	caseRoot := filepath.Join(t.TempDir(), "case")
	writeReviewerIntakeCase(t, repoRoot, caseRoot)
	plan, err := WritePlan(repoRoot, caseRoot, defaults.DefaultPack, Options{TaskType: "feature-analysis", Items: "alpha"})
	if err != nil {
		t.Fatal(err)
	}
	resultPath := filepath.Join(t.TempDir(), "reviewer-result.json")
	if err := os.WriteFile(resultPath, reviewerResultForPlan(t, plan.PacketPath, "accept", "accepted", nil), 0o644); err != nil {
		t.Fatal(err)
	}
	originalAppend := appendReviewerNote
	appendCalls := 0
	appendReviewerNote = func(repoRoot, caseRoot, pack string, opt note.Options, whatIf bool) (note.AppendResult, error) {
		if !whatIf {
			appendCalls++
			if appendCalls == 2 {
				return note.AppendResult{}, fmt.Errorf("injected decision append failure")
			}
		}
		return originalAppend(repoRoot, caseRoot, pack, opt, whatIf)
	}
	defer func() { appendReviewerNote = originalAppend }()
	partial, err := IntakeReviewerResult(repoRoot, caseRoot, defaults.DefaultPack, ReviewerIntakeOptions{PacketPath: plan.PacketPath, ReviewerResultPath: resultPath, Lane: "devirt-main", Actor: "mission-commander"})
	appendReviewerNote = originalAppend
	if err == nil || partial.WritebackStatus != "verification-recorded" || partial.Verification == nil || !partial.Verification.Applied || partial.Decision == nil || partial.Decision.Applied {
		t.Fatalf("partial writeback status was not preserved: result=%+v err=%v", partial, err)
	}
	assertReviewerIntakeActionQueue(t, partial.MissionCommanderActionQueue, 2, 2, 0, 2, 1, partial.MissionCommanderAction.PrimaryCommand)
	if got := strings.Count(readOptionalFile(t, filepath.Join(caseRoot, ".rekit", "facts", "verifications.jsonl")), partial.Verification.EventID); got != 1 {
		t.Fatalf("verification append count = %d, want 1", got)
	}

	recovered, err := IntakeReviewerResult(repoRoot, caseRoot, defaults.DefaultPack, ReviewerIntakeOptions{PacketPath: plan.PacketPath, ReviewerResultPath: resultPath, Lane: "devirt-main", Actor: "mission-commander"})
	if err != nil {
		t.Fatal(err)
	}
	if recovered.WritebackStatus != "complete" || recovered.Verification == nil || recovered.Verification.Applied || recovered.Decision == nil || !recovered.Decision.Applied {
		t.Fatalf("partial writeback retry did not recover idempotently: %+v", recovered)
	}
	if got := strings.Count(readOptionalFile(t, filepath.Join(caseRoot, ".rekit", "facts", "verifications.jsonl")), recovered.Verification.EventID); got != 1 {
		t.Fatalf("verification append count after recovery = %d, want 1", got)
	}
	if got := strings.Count(readOptionalFile(t, filepath.Join(caseRoot, ".rekit", "facts", "decisions.jsonl")), recovered.Decision.EventID); got != 1 {
		t.Fatalf("decision append count after recovery = %d, want 1", got)
	}
}

func assertReviewerIntakeActionQueue(t *testing.T, queue mission.MissionCommanderActionQueue, total, unblocked, blocked, requiresReview, followUp int, currentCommand string) {
	t.Helper()
	if queue.Counts.Total != total || queue.Counts.Unblocked != unblocked || queue.Counts.Blocked != blocked || queue.Counts.RequiresReview != requiresReview || queue.Counts.FollowUp != followUp || queue.CurrentAction == nil || queue.CurrentAction.Command != currentCommand || !strings.Contains(queue.Summary, "current="+currentCommand) {
		t.Fatalf("reviewer intake Mission Commander action queue drifted: %+v", queue)
	}
}

func hasReviewerIntakeCommanderNextAction(items []mission.MissionCommanderNextActionItem, source, command string, blocked, requiresReview bool) bool {
	return slices.ContainsFunc(items, func(item mission.MissionCommanderNextActionItem) bool {
		return item.Source == source && item.Command == command && item.Blocked == blocked && item.RequiresReview == requiresReview
	})
}

func reviewerResultForPlan(t *testing.T, packetPath, decision, verdict string, conflicts []string) []byte {
	t.Helper()
	packet := readReviewerPacket(t, packetPath)
	caseRoot := packet.PlanRoot
	evidencePath := filepath.Join(caseRoot, "workspace", "review-evidence.md")
	if err := os.MkdirAll(filepath.Dir(evidencePath), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(evidencePath, []byte("bounded reviewer evidence\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	return reviewerResultForPacket(t, packet, decision, verdict, conflicts)
}

func reviewerResultForPacket(t *testing.T, packet Packet, decision, verdict string, conflicts []string) []byte {
	t.Helper()
	return reviewerResultJSON(packet.PacketID, packet.Route.ID, decision, verdict, conflicts)
}

func readReviewerPacket(t *testing.T, packetPath string) Packet {
	t.Helper()
	data, err := os.ReadFile(packetPath)
	if err != nil {
		t.Fatal(err)
	}
	var packet Packet
	if err := json.Unmarshal(data, &packet); err != nil {
		t.Fatal(err)
	}
	return packet
}

func reviewerResultJSON(packetID, routeID, decision, verdict string, conflicts []string) []byte {
	if conflicts == nil {
		conflicts = []string{}
	}
	routeOutput := map[string]any{}
	for _, field := range splitCSV("item,decision,confidence,evidence,risk,next_action,tier_used,tool_scope,feature,request_id,candidate_path,defer_reason") {
		routeOutput[field] = "n/a"
	}
	routeOutput["item"] = "alpha"
	routeOutput["decision"] = decision
	routeOutput["confidence"] = "high"
	if decision == "accept" || decision == "reject" {
		routeOutput["evidence"] = "workspace/review-evidence.md"
	}
	value := ReviewerResult{
		PacketID:           packetID,
		RouteID:            routeID,
		ShardID:            "shard-01",
		Items:              []string{"alpha"},
		ReviewerSession:    "reviewer-session-1",
		Decision:           decision,
		Confidence:         "high",
		Summary:            "reviewed alpha against bounded evidence",
		EvidenceRefs:       []string{"workspace/review-evidence.md"},
		Risks:              []string{},
		Conflicts:          conflicts,
		RecommendedVerdict: verdict,
		RouteOutput:        routeOutput,
	}
	data, err := json.Marshal(value)
	if err != nil {
		panic(err)
	}
	return data
}

func readOptionalFile(t *testing.T, path string) string {
	t.Helper()
	data, err := os.ReadFile(path)
	if os.IsNotExist(err) {
		return ""
	}
	if err != nil {
		t.Fatal(err)
	}
	return string(data)
}

func writeReviewerIntakeCase(t *testing.T, repoRoot, caseRoot string) {
	t.Helper()
	root, err := filepath.Abs(repoRoot)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := syncreview.Apply(root, caseRoot, defaults.DefaultPack, syncreview.ApplyOptions{ProjectName: "reviewer-intake-test", CreateLocalFiles: true, Command: "init"}); err != nil {
		t.Fatal(err)
	}
	if _, err := workstream.StartApply(root, caseRoot, defaults.DefaultPack, workstream.StartOptions{Name: "intake", Executor: "session-main", Actor: "mission-commander", TakeoverReason: "reviewer intake owner binding fixture"}); err != nil {
		t.Fatal(err)
	}
}
