package cli

import (
	"bytes"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestRunCurrentStepRoutesLaneThenReviewerLifecycle(t *testing.T) {
	caseRoot := fullAttachedCase(t)
	var out bytes.Buffer
	if err := Run([]string{"-Command", "overview", "-Target", caseRoot, "-Pack", "_template", "-Format", "json"}, &out); err != nil {
		t.Fatal(err)
	}
	base := []string{"-Command", "run-current-step", "-Target", caseRoot, "-Pack", "_template", "-WhatIf", "-Format", "json"}

	out.Reset()
	err := Run([]string{"-Command", "run-current-step", "-Target", caseRoot, "-Pack", "_template", "-ReviewerHarness", "unexpected", "-WhatIf", "-Format", "json"}, &out)
	if err == nil || !strings.Contains(err.Error(), "case route does not accept reviewer observation inputs") {
		t.Fatalf("case route accepted reviewer inputs: %v", err)
	}
	out.Reset()
	err = Run([]string{"-Command", "run-current-step", "-Target", caseRoot, "-Pack", "_template", "-ExpectedCurrentStepPlanSha256", strings.Repeat("0", 64), "-Apply", "-Format", "json"}, &out)
	if err == nil || !strings.Contains(err.Error(), "expected plan sha256 mismatch") {
		t.Fatalf("stale current-step plan was not rejected: %v", err)
	}
	out.Reset()
	err = Run([]string{"-Command", "run-current-step", "-Target", caseRoot, "-Pack", "_template", "-Lane", "feature-review", "-WhatIf", "-Format", "json"}, &out)
	if err == nil || !strings.Contains(err.Error(), "unsupported flag") {
		t.Fatalf("unsupported current-step outer flag was not rejected: %v", err)
	}

	lanePreview := runCurrentStepPreview(t, base)
	if lanePreview.Route != "case" || lanePreview.DriverStep == nil || lanePreview.ReviewerStep != nil || lanePreview.ExpectedCurrentStepPlanSHA256 == "" {
		t.Fatalf("current-step did not route initial lane start: %+v", lanePreview)
	}
	laneApply := runCurrentStepApply(t, caseRoot, lanePreview)
	if !laneApply.Applied || laneApply.Receipt == nil || laneApply.Receipt.Route != "case" || laneApply.Receipt.NestedCommand != "run-driver-step" {
		t.Fatalf("current-step lane apply omitted nested runner receipt: %+v", laneApply)
	}

	out.Reset()
	if err := Run([]string{"-Command", "plan-subagents", "-Target", caseRoot, "-Pack", "_template", "-TaskType", "feature-analysis", "-Items", "alpha"}, &out); err != nil {
		t.Fatal(err)
	}
	plan := decodePlanSubagentsResult(t, out.Bytes())
	packet := decodePlanSubagentsPacket(t, plan.PacketPath)
	handoff := plan.ShardHandoffs[0]
	actor := "mission-commander"

	spawn := runCurrentStepPreview(t, base)
	if spawn.Route != "reviewer" || spawn.ReviewerStep == nil || spawn.ReviewerStep.ExternalHandoff == nil || spawn.ReviewerStep.ExternalHandoff.RunLoopStepID != "spawn-reviewer" || spawn.ExpectedCurrentStepPlanSHA256 != "" {
		t.Fatalf("current-step did not route reviewer spawn handoff: %+v", spawn)
	}
	dispatchArgs := append(append([]string{}, base...), "-ReviewerHarness", "current-step-harness", "-ReviewerSession", "current-step-reviewer", "-Actor", actor)
	dispatch := runCurrentStepPreview(t, dispatchArgs)
	if dispatch.Route != "reviewer" || dispatch.ReviewerStep == nil || dispatch.ReviewerStep.ApplyDriverRequest == nil || dispatch.ExpectedCurrentStepPlanSHA256 == "" {
		t.Fatalf("current-step reviewer dispatch did not return hash-bound apply: %+v", dispatch)
	}
	runCurrentStepApply(t, caseRoot, dispatch, "-ReviewerHarness", "current-step-harness", "-ReviewerSession", "current-step-reviewer", "-Actor", actor)

	waiting := runCurrentStepPreview(t, base)
	if waiting.Route != "reviewer" || waiting.ReviewerStep == nil || waiting.ReviewerStep.ExternalHandoff == nil || waiting.ReviewerStep.ExternalHandoff.RunLoopStepID != "save-result-input" {
		t.Fatalf("current-step did not retain reviewer route while waiting for result: %+v", waiting)
	}
	evidencePath := filepath.Join(caseRoot, "workspace", "review-evidence.md")
	if err := os.WriteFile(evidencePath, []byte("bounded reviewer evidence\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	resultSource := filepath.Join(caseRoot, "workspace", "current-step-reviewer-result.json")
	data, err := json.Marshal(map[string]any{
		"packetId": packet.PacketID, "routeId": packet.Route.ID, "shardId": handoff.ShardID,
		"items": []string{"alpha"}, "reviewerSession": "current-step-reviewer",
		"decision": "accept", "confidence": "high", "summary": "reviewed through current-step",
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
	inputArgs := append(append([]string{}, base...), "-ReviewerResultInputSourcePath", resultSource, "-Actor", actor)
	input := runCurrentStepPreview(t, inputArgs)
	runCurrentStepApply(t, caseRoot, input, "-ReviewerResultInputSourcePath", resultSource, "-Actor", actor)

	for _, stepID := range []string{"record-completion", "source-capture", "stage-candidate", "collect-result", "intake-results"} {
		stepArgs := append(append([]string{}, base...), "-Actor", actor)
		step := runCurrentStepPreview(t, stepArgs)
		if step.Route != "reviewer" || step.ReviewerStep == nil || step.ReviewerStep.CurrentDriverRequest.RunLoopStepID != stepID || step.ExpectedCurrentStepPlanSHA256 == "" {
			t.Fatalf("current-step reviewer pipeline step %s mismatch: %+v", stepID, step)
		}
		runCurrentStepApply(t, caseRoot, step, "-Actor", actor)
	}

	resumed := runCurrentStepPreview(t, base)
	if resumed.Route != "case" || resumed.DriverStep == nil || resumed.ReviewerStep != nil {
		t.Fatalf("current-step did not return to lane route after reviewer intake: %+v", resumed)
	}
	for _, forbidden := range []string{"authority.jsonl", "confirmed.jsonl"} {
		if _, err := os.Stat(filepath.Join(caseRoot, ".rekit", "facts", forbidden)); !os.IsNotExist(err) {
			t.Fatalf("current-step created forbidden ledger %s: %v", forbidden, err)
		}
	}

	driftPreview := runCurrentStepPreview(t, base)
	if driftPreview.Route != "case" || driftPreview.ExpectedCurrentStepPlanSHA256 == "" {
		t.Fatalf("post-review valid plan missing: %+v", driftPreview)
	}
	out.Reset()
	if err := Run([]string{"-Command", "start", "-Target", caseRoot, "-Pack", "_template", "-Name", "drifted", "-Apply", "-Format", "json"}, &out); err != nil {
		t.Fatal(err)
	}
	out.Reset()
	err = Run([]string{"-Command", "run-current-step", "-Target", caseRoot, "-Pack", "_template", "-ExpectedCurrentStepPlanSha256", driftPreview.ExpectedCurrentStepPlanSHA256, "-Apply", "-Format", "json"}, &out)
	if err == nil || !strings.Contains(err.Error(), "expected plan sha256 mismatch") {
		t.Fatalf("valid current-step plan survived durable state drift: %v", err)
	}
}

type currentStepTestPlan struct {
	Route                         string              `json:"route"`
	Applied                       bool                `json:"applied"`
	DriverStep                    *driverStepPlan     `json:"driverStep"`
	ReviewerStep                  *reviewerStepPlan   `json:"reviewerStep"`
	ExpectedCurrentStepPlanSHA256 string              `json:"expectedCurrentStepPlanSha256"`
	Receipt                       *currentStepReceipt `json:"receipt"`
}

func runCurrentStepPreview(t *testing.T, args []string) currentStepTestPlan {
	t.Helper()
	var out bytes.Buffer
	if err := Run(args, &out); err != nil {
		t.Fatal(err)
	}
	var result currentStepTestPlan
	if err := json.Unmarshal(out.Bytes(), &result); err != nil {
		t.Fatalf("current-step JSON did not decode: %v\n%s", err, out.String())
	}
	return result
}

func runCurrentStepApply(t *testing.T, caseRoot string, preview currentStepTestPlan, inputs ...string) currentStepTestPlan {
	t.Helper()
	var out bytes.Buffer
	args := []string{"-Command", "run-current-step", "-Target", caseRoot, "-Pack", "_template"}
	args = append(args, inputs...)
	args = append(args, "-ExpectedCurrentStepPlanSha256", preview.ExpectedCurrentStepPlanSHA256, "-Apply", "-Format", "json")
	if err := Run(args, &out); err != nil {
		t.Fatal(err)
	}
	var result currentStepTestPlan
	if err := json.Unmarshal(out.Bytes(), &result); err != nil {
		t.Fatalf("current-step apply JSON did not decode: %v\n%s", err, out.String())
	}
	if !result.Applied || result.Receipt == nil || result.Receipt.State != "refreshed" {
		t.Fatalf("current-step apply omitted refreshed receipt: %+v", result)
	}
	return result
}
