package cli

import (
	"bytes"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/shuiyu486/re-context-kits/internal/rekit/mission"
)

func TestRunReviewerStepDispatchInputAndDeterministicPipeline(t *testing.T) {
	caseRoot := fullAttachedCase(t)
	var out bytes.Buffer
	if err := Run([]string{"-Command", "start", "-Target", caseRoot, "-Pack", "_template", "-Name", "review", "-Apply"}, &out); err != nil {
		t.Fatal(err)
	}
	out.Reset()
	if err := Run([]string{"-Command", "plan-subagents", "-Target", caseRoot, "-Pack", "_template", "-TaskType", "feature-analysis", "-Items", "alpha"}, &out); err != nil {
		t.Fatal(err)
	}
	plan := decodePlanSubagentsResult(t, out.Bytes())
	packet := decodePlanSubagentsPacket(t, plan.PacketPath)
	handoff := plan.ShardHandoffs[0]
	actor := "mission-commander"
	base := []string{"-Command", "run-reviewer-step", "-Target", caseRoot, "-Pack", "_template", "-WhatIf", "-Format", "json"}

	preview := runReviewerStepPreview(t, base)
	if preview.ExternalHandoff == nil || preview.ExternalHandoff.RunLoopStepID != "spawn-reviewer" || preview.ExternalHandoff.AgentToolRequest == nil || !preview.ExternalHandoff.AgentToolRequest.ReadOnly || preview.ExpectedReviewerStepPlanSHA256 != "" {
		t.Fatalf("spawn step did not return external harness handoff: %+v", preview)
	}
	out.Reset()
	err := Run(append(append([]string{}, base...), "-ReviewerHarness", "go-cli-test-harness", "-ReviewerSession", "reviewer-session-runner", "-ReviewerOutcome", "failed", "-ReviewerExitStatus", "exit-1", "-Actor", actor), &out)
	if err == nil || !strings.Contains(err.Error(), "spawn-reviewer accepts only") {
		t.Fatalf("spawn-reviewer accepted unrelated completion observation: %v", err)
	}
	if _, err := os.Stat(handoff.ReviewerStagingCommands.SourceCaptureInput); !os.IsNotExist(err) {
		t.Fatalf("external reviewer preview wrote reviewer input: %v", err)
	}

	dispatchPreview := runReviewerStepPreview(t, append(base,
		"-ReviewerHarness", "go-cli-test-harness",
		"-ReviewerSession", "reviewer-session-runner",
		"-Actor", actor,
	))
	if dispatchPreview.ExternalHandoff != nil || dispatchPreview.ApplyDriverRequest == nil || dispatchPreview.ExpectedReviewerStepPlanSHA256 == "" || !strings.Contains(dispatchPreview.ApplyDriverRequest.Command, "-RecordReviewerDispatch") {
		t.Fatalf("dispatch receipt preview did not return hash-bound apply: %+v", dispatchPreview)
	}
	runReviewerStepApply(t, caseRoot, dispatchPreview, "-ReviewerHarness", "go-cli-test-harness", "-ReviewerSession", "reviewer-session-runner", "-Actor", actor)

	waiting := runReviewerStepPreview(t, base)
	if waiting.ExternalHandoff == nil || waiting.ExternalHandoff.RunLoopStepID != "save-result-input" {
		t.Fatalf("running reviewer did not return result-input handoff: %+v", waiting)
	}
	evidencePath := filepath.Join(caseRoot, "workspace", "review-evidence.md")
	if err := os.WriteFile(evidencePath, []byte("bounded reviewer evidence\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	resultSource := filepath.Join(caseRoot, "workspace", "reviewer-result.json")
	data, err := json.Marshal(map[string]any{
		"packetId": packet.PacketID, "routeId": packet.Route.ID, "shardId": handoff.ShardID,
		"items": []string{"alpha"}, "reviewerSession": "reviewer-session-runner",
		"decision": "accept", "confidence": "high", "summary": "reviewed alpha",
		"evidenceRefs": []string{"workspace/review-evidence.md"}, "risks": []string{}, "conflicts": []string{},
		"recommendedVerdict": "accepted",
		"routeOutput":        map[string]any{"item": "alpha", "decision": "accept", "confidence": "high", "evidence": "workspace/review-evidence.md", "risk": "low", "next_action": "main-agent-writeback", "tier_used": "light", "tool_scope": "read-only", "feature": "review", "request_id": "n/a", "candidate_path": "n/a", "defer_reason": "n/a"},
	})
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(resultSource, data, 0o644); err != nil {
		t.Fatal(err)
	}

	out.Reset()
	err = Run(append(append([]string{}, base...), "-ReviewerResultInputSourcePath", resultSource, "-ReviewerOutcome", "failed", "-ReviewerExitStatus", "exit-1", "-Actor", actor), &out)
	if err == nil || !strings.Contains(err.Error(), "either a result source or a failed outcome observation, not both") {
		t.Fatalf("save-result-input accepted mutually exclusive observations: %v", err)
	}
	inputPreview := runReviewerStepPreview(t, append(base, "-ReviewerResultInputSourcePath", resultSource, "-Actor", actor))
	if inputPreview.ApplyDriverRequest == nil || !strings.Contains(inputPreview.ApplyDriverRequest.Command, "-SaveReviewerResultInput") {
		t.Fatalf("result input preview did not return save apply: %+v", inputPreview)
	}
	runReviewerStepApply(t, caseRoot, inputPreview, "-ReviewerResultInputSourcePath", resultSource, "-Actor", actor)

	for _, expectedStep := range []string{"record-completion", "source-capture", "stage-candidate", "collect-result", "intake-results"} {
		stepPreview := runReviewerStepPreview(t, append(base, "-Actor", actor))
		if stepPreview.CurrentDriverRequest.RunLoopStepID != expectedStep || stepPreview.ExternalHandoff != nil || stepPreview.ApplyDriverRequest == nil || stepPreview.ExpectedReviewerStepPlanSHA256 == "" {
			t.Fatalf("unexpected reviewer pipeline step %s: %+v", expectedStep, stepPreview)
		}
		runReviewerStepApply(t, caseRoot, stepPreview, "-Actor", actor)
	}

	verifications, err := os.ReadFile(filepath.Join(caseRoot, ".rekit", "facts", "verifications.jsonl"))
	if err != nil {
		t.Fatal(err)
	}
	decisions, err := os.ReadFile(filepath.Join(caseRoot, ".rekit", "facts", "decisions.jsonl"))
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(verifications), packet.PacketID) || !strings.Contains(string(decisions), packet.PacketID) {
		t.Fatalf("reviewer intake did not append verification/decision writeback")
	}
	for _, forbidden := range []string{"authority.jsonl", "confirmed.jsonl"} {
		if _, err := os.Stat(filepath.Join(caseRoot, ".rekit", "facts", forbidden)); !os.IsNotExist(err) {
			t.Fatalf("reviewer runner created forbidden ledger %s: %v", forbidden, err)
		}
	}
}

func TestRunReviewerStepDefersBatchIntakeWhileAnotherShardRuns(t *testing.T) {
	caseRoot := fullAttachedCase(t)
	var out bytes.Buffer
	if err := Run([]string{"-Command", "start", "-Target", caseRoot, "-Pack", "_template", "-Name", "review", "-Apply"}, &out); err != nil {
		t.Fatal(err)
	}
	out.Reset()
	if err := Run([]string{"-Command", "plan-subagents", "-Target", caseRoot, "-Pack", "_template", "-TaskType", "feature-analysis", "-Items", "alpha,beta", "-ItemsPerAgent", "1", "-MaxParallel", "2", "-Format", "json"}, &out); err != nil {
		t.Fatal(err)
	}
	plan := decodePlanSubagentsResult(t, out.Bytes())
	packet := decodePlanSubagentsPacket(t, plan.PacketPath)
	if len(plan.ShardHandoffs) != 2 {
		t.Fatalf("two-shard reviewer plan omitted shards: %+v", plan.ShardHandoffs)
	}
	actor := "mission-commander"
	first := plan.ShardHandoffs[0]
	second := plan.ShardHandoffs[1]
	evidencePath := filepath.Join(caseRoot, "workspace", "features", "feature-login", "review-evidence.md")
	if err := os.MkdirAll(filepath.Dir(evidencePath), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(evidencePath, []byte("bounded reviewer evidence\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	firstResult := reviewerResultForCLIPlan(t, packet, first, "accept", "accepted", "reviewer-session-first")
	stageAndCollectReviewerResultForCLIPlan(t, &out, []string{"-Target", caseRoot, "-Pack", "_template"}, plan.PacketPath, first, packet.TargetLane, actor, firstResult)
	out.Reset()
	if err := Run([]string{"-Command", "plan-subagents", "-Target", caseRoot, "-Pack", "_template", "-PacketPath", plan.PacketPath, "-RecordReviewerDispatch", "-ShardId", second.ShardID, "-ReviewerHarness", "go-cli-second-harness", "-ReviewerSession", "reviewer-session-second", "-Lane", packet.TargetLane, "-Actor", actor, "-WhatIf", "-Format", "json"}, &out); err != nil {
		t.Fatal(err)
	}
	var dispatchPreview struct {
		BindingSHA256 string `json:"bindingSha256"`
	}
	if err := json.Unmarshal(out.Bytes(), &dispatchPreview); err != nil || dispatchPreview.BindingSHA256 == "" {
		t.Fatalf("second reviewer dispatch preview did not decode: %v\n%s", err, out.String())
	}
	out.Reset()
	if err := Run([]string{"-Command", "plan-subagents", "-Target", caseRoot, "-Pack", "_template", "-PacketPath", plan.PacketPath, "-RecordReviewerDispatch", "-ShardId", second.ShardID, "-ReviewerHarness", "go-cli-second-harness", "-ReviewerSession", "reviewer-session-second", "-Lane", packet.TargetLane, "-Actor", actor, "-ExpectedReviewerDispatchBindingSha256", dispatchPreview.BindingSHA256, "-Apply", "-Format", "json"}, &out); err != nil {
		t.Fatal(err)
	}

	preview := runReviewerStepPreview(t, []string{"-Command", "run-reviewer-step", "-Target", caseRoot, "-Pack", "_template", "-WhatIf", "-Format", "json"})
	if preview.CurrentDriverRequest.RunLoopStepID != "save-result-input" || preview.ExternalHandoff == nil || preview.ExternalHandoff.RunLoopStepID != "save-result-input" || preview.ApplyDriverRequest != nil {
		t.Fatalf("mixed ready/running shards selected batch intake instead of running reviewer handoff: %+v", preview)
	}
	current := runCurrentStepPreview(t, []string{"-Command", "run-current-step", "-Target", caseRoot, "-Pack", "_template", "-WhatIf", "-Format", "json"})
	if current.Route != "reviewer" || current.ReviewerStep == nil || current.ReviewerStep.CurrentDriverRequest.RunLoopStepID != "save-result-input" || current.ReviewerStep.ExternalHandoff == nil || current.ExpectedCurrentStepPlanSHA256 != "" {
		t.Fatalf("mixed ready/running shards drifted between unified route and reviewer operator package: %+v", current)
	}
}

func TestRunReviewerStepFailsClosedAcrossReplacementExecutorTakeover(t *testing.T) {
	caseRoot := fullAttachedCase(t)
	var out bytes.Buffer
	if err := Run([]string{"-Command", "start", "-Target", caseRoot, "-Pack", "_template", "-Name", "review", "-Executor", "executor-a", "-Actor", "mission-commander", "-Reason", "initial reviewer owner", "-Apply"}, &out); err != nil {
		t.Fatal(err)
	}
	out.Reset()
	if err := Run([]string{"-Command", "plan-subagents", "-Target", caseRoot, "-Pack", "_template", "-TaskType", "feature-analysis", "-Items", "alpha", "-Lane", "feature-review"}, &out); err != nil {
		t.Fatal(err)
	}
	plan := decodePlanSubagentsResult(t, out.Bytes())
	packet := decodePlanSubagentsPacket(t, plan.PacketPath)
	if packet.TargetLane != "feature-review" || packet.OwnerBinding.CurrentExecutor != "executor-a" || packet.OwnerBinding.ExecutorGeneration < 1 {
		t.Fatalf("reviewer packet did not bind the initial executor: %+v", packet.OwnerBinding)
	}
	previewArgs := []string{"-Command", "run-reviewer-step", "-Target", caseRoot, "-Pack", "_template", "-ReviewerHarness", "harness", "-ReviewerSession", "session-a", "-Actor", "mission-commander", "-WhatIf", "-Format", "json"}
	preview := runReviewerStepPreview(t, previewArgs)
	if preview.ExpectedReviewerStepPlanSHA256 == "" {
		t.Fatalf("dispatch preview omitted plan hash: %+v", preview)
	}

	out.Reset()
	if err := Run([]string{"-Command", "start", "-Target", caseRoot, "-Pack", "_template", "-Name", "review", "-Executor", "executor-b", "-Actor", "mission-commander", "-Reason", "replacement executor takeover", "-Apply"}, &out); err != nil {
		t.Fatal(err)
	}
	out.Reset()
	if err := Run([]string{"-Command", "status", "-Target", caseRoot, "-Pack", "_template", "-Format", "json"}, &out); err != nil {
		t.Fatal(err)
	}
	var status statusInventory
	if err := json.Unmarshal(out.Bytes(), &status); err != nil {
		t.Fatal(err)
	}
	if status.CaseMission == nil || len(status.CaseMission.ReviewerDispatchIntakeHandoffs) != 1 {
		t.Fatalf("replacement status omitted reviewer handoff")
	}
	if got := status.CaseMission.ReviewerDispatchIntakeHandoffs[0].State; got != "reviewer-packet-owner-adoption-required" {
		t.Fatalf("replacement executor did not invalidate packet owner: state=%s", got)
	}
	out.Reset()
	err := Run([]string{"-Command", "run-reviewer-step", "-Target", caseRoot, "-Pack", "_template", "-ReviewerHarness", "harness", "-ReviewerSession", "session-a", "-Actor", "mission-commander", "-ExpectedReviewerStepPlanSha256", preview.ExpectedReviewerStepPlanSHA256, "-Apply", "-Format", "json"}, &out)
	if err == nil {
		t.Fatal("reviewer plan survived replacement executor takeover")
	}

	out.Reset()
	if err := Run([]string{"-Command", "plan-subagents", "-Target", caseRoot, "-Pack", "_template", "-PacketPath", plan.PacketPath, "-AdoptReviewerPacket", "-Lane", "feature-review", "-Actor", "mission-commander", "-Reason", "adopt reviewer packet after takeover", "-Apply", "-Format", "json"}, &out); err != nil {
		t.Fatal(err)
	}
	fresh := runReviewerStepPreview(t, []string{"-Command", "run-reviewer-step", "-Target", caseRoot, "-Pack", "_template", "-ReviewerHarness", "harness", "-ReviewerSession", "session-b", "-Actor", "mission-commander", "-WhatIf", "-Format", "json"})
	if fresh.ExpectedReviewerStepPlanSHA256 == "" || fresh.ExpectedReviewerStepPlanSHA256 == preview.ExpectedReviewerStepPlanSHA256 || fresh.ApplyDriverRequest == nil {
		t.Fatalf("adopted replacement executor did not receive a fresh dispatch plan: old=%s fresh=%+v", preview.ExpectedReviewerStepPlanSHA256, fresh)
	}
	runReviewerStepApply(t, caseRoot, fresh, "-ReviewerHarness", "harness", "-ReviewerSession", "session-b", "-Actor", "mission-commander")
	for _, forbidden := range []string{"authority.jsonl", "confirmed.jsonl"} {
		if _, err := os.Stat(filepath.Join(caseRoot, ".rekit", "facts", forbidden)); !os.IsNotExist(err) {
			t.Fatalf("replacement reviewer runner created forbidden ledger %s: %v", forbidden, err)
		}
	}
}

func TestRunReviewerStepRejectsStalePlanAndUnsupportedOuterArgs(t *testing.T) {
	caseRoot := fullAttachedCase(t)
	var out bytes.Buffer
	if err := Run([]string{"-Command", "start", "-Target", caseRoot, "-Pack", "_template", "-Name", "review", "-Apply"}, &out); err != nil {
		t.Fatal(err)
	}
	out.Reset()
	if err := Run([]string{"-Command", "plan-subagents", "-Target", caseRoot, "-Pack", "_template", "-TaskType", "feature-analysis", "-Items", "alpha"}, &out); err != nil {
		t.Fatal(err)
	}
	preview := runReviewerStepPreview(t, []string{"-Command", "run-reviewer-step", "-Target", caseRoot, "-Pack", "_template", "-ReviewerHarness", "harness", "-ReviewerSession", "session-a", "-Actor", "mission-commander", "-WhatIf", "-Format", "json"})
	if preview.ExpectedReviewerStepPlanSHA256 == "" {
		t.Fatalf("dispatch preview omitted plan hash: %+v", preview)
	}
	out.Reset()
	err := Run([]string{"-Command", "run-reviewer-step", "-Target", caseRoot, "-Pack", "_template", "-ReviewerHarness", "harness", "-ReviewerSession", "session-a", "-Actor", "mission-commander", "-ExpectedReviewerStepPlanSha256", strings.Repeat("0", 64), "-Apply", "-Format", "json"}, &out)
	if err == nil || !strings.Contains(err.Error(), "expected plan sha256 mismatch") {
		t.Fatalf("stale reviewer plan was not rejected: %v", err)
	}
	out.Reset()
	err = Run([]string{"-Command", "run-reviewer-step", "-Target", caseRoot, "-Pack", "_template", "-Lane", "feature-review", "-WhatIf", "-Format", "json"}, &out)
	if err == nil || !strings.Contains(err.Error(), "unsupported flag") {
		t.Fatalf("unsupported reviewer outer flag was not rejected: %v", err)
	}
}

type reviewerStepTestPlan struct {
	CurrentDriverRequest           mission.MissionCommanderDriverRequest  `json:"currentDriverRequest"`
	ApplyDriverRequest             *mission.MissionCommanderDriverRequest `json:"applyDriverRequest"`
	ExpectedReviewerStepPlanSHA256 string                                 `json:"expectedReviewerStepPlanSha256"`
	ExternalHandoff                *reviewerStepExternalHandoff           `json:"externalHandoff"`
	Receipt                        *reviewerStepReceipt                   `json:"receipt"`
	Applied                        bool                                   `json:"applied"`
}

func runReviewerStepPreview(t *testing.T, args []string) reviewerStepTestPlan {
	t.Helper()
	var out bytes.Buffer
	if err := Run(args, &out); err != nil {
		t.Fatal(err)
	}
	var result reviewerStepTestPlan
	if err := json.Unmarshal(out.Bytes(), &result); err != nil {
		t.Fatalf("reviewer step JSON did not decode: %v\n%s", err, out.String())
	}
	return result
}

func runReviewerStepApply(t *testing.T, caseRoot string, preview reviewerStepTestPlan, inputs ...string) reviewerStepTestPlan {
	t.Helper()
	var out bytes.Buffer
	args := []string{"-Command", "run-reviewer-step", "-Target", caseRoot, "-Pack", "_template"}
	args = append(args, inputs...)
	args = append(args, "-ExpectedReviewerStepPlanSha256", preview.ExpectedReviewerStepPlanSHA256, "-Apply", "-Format", "json")
	if err := Run(args, &out); err != nil {
		t.Fatal(err)
	}
	var result reviewerStepTestPlan
	if err := json.Unmarshal(out.Bytes(), &result); err != nil {
		t.Fatalf("reviewer step apply JSON did not decode: %v\n%s", err, out.String())
	}
	if !result.Applied || result.Receipt == nil || result.Receipt.State != "refreshed" || !result.Receipt.RefreshStatusCommandMatched {
		t.Fatalf("reviewer step apply omitted refreshed receipt: %+v", result)
	}
	return result
}
