package cli

import (
	"bytes"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/shuiyu486/re-context-kits/internal/rekit/subagents"
)

func TestRunPlanSubagentsReviewerResultRecoveryCaseLocalE2E(t *testing.T) {
	caseRoot := fullAttachedCase(t)
	var out bytes.Buffer
	if err := Run([]string{"-Command", "start", "-Target", caseRoot, "-Pack", "_template", "-Name", "review", "-Apply"}, &out); err != nil {
		t.Fatal(err)
	}
	writeCaseFile(t, caseRoot, "workspace/recovery-evidence.md", "bounded recovery evidence\n")
	nested := filepath.Join(caseRoot, "workspace")
	oldwd, err := os.Getwd()
	if err != nil {
		t.Fatal(err)
	}
	if err := os.Chdir(nested); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = os.Chdir(oldwd) })

	out.Reset()
	if err := Run([]string{"-Command", "plan-subagents", "-TaskType", "feature-analysis", "-Items", "alpha", "-Lane", "feature-review", "-Format", "json"}, &out); err != nil {
		t.Fatal(err)
	}
	plan := decodePlanSubagentsResult(t, out.Bytes())
	packet := decodePlanSubagentsPacket(t, plan.PacketPath)
	handoff := plan.ShardHandoffs[0]
	candidate, err := json.Marshal(map[string]any{
		"packetId": packet.PacketID, "routeId": packet.Route.ID, "shardId": handoff.ShardID, "items": handoff.Items,
		"reviewerSession": "reviewer-recovery-e2e", "decision": "accept", "confidence": "high", "summary": "reviewed alpha",
		"evidenceRefs": []string{"workspace/recovery-evidence.md"}, "risks": []string{}, "conflicts": []string{}, "recommendedVerdict": "accepted",
		"routeOutput": map[string]any{"item": "alpha", "decision": "accept", "confidence": "high", "evidence": "workspace/recovery-evidence.md", "risk": "low", "next_action": "main-agent-writeback", "tier_used": "light", "tool_scope": "read-only", "feature": "review", "request_id": "n/a", "candidate_path": "n/a", "defer_reason": "n/a"},
	})
	if err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(filepath.Dir(handoff.ReviewerResultCandidatePath), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(handoff.ReviewerResultCandidatePath, candidate, 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(handoff.ReviewerResultPath, []byte(`{"conflict":true}`), 0o644); err != nil {
		t.Fatal(err)
	}
	args := []string{"-Command", "plan-subagents", "-PacketPath", plan.PacketPath, "-RecoverReviewerResult", "-ShardId", handoff.ShardID, "-Lane", "feature-review", "-Actor", "mission-commander", "-Reason", "quarantine conflicting canonical reviewer result"}
	out.Reset()
	if err := Run(append(append([]string{}, args...), "-WhatIf", "-Format", "json"), &out); err != nil {
		t.Fatal(err)
	}
	var preview subagents.ReviewerResultRecoveryResult
	if err := json.Unmarshal(out.Bytes(), &preview); err != nil {
		t.Fatal(err)
	}
	if preview.Applied || preview.CandidateSHA256 == "" || preview.ReviewerResultSHA256 == "" {
		t.Fatalf("unexpected recovery preview: %+v", preview)
	}
	out.Reset()
	if err := Run(append(append([]string{}, args...), "-WhatIf", "-Format", "text"), &out); err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(out.String(), "plan-subagents reviewer result recovery：") {
		t.Fatalf("unexpected recovery text: %s", out.String())
	}
	out.Reset()
	applyArgs := append(append([]string{}, args...), "-ExpectedCandidateSha256", preview.CandidateSHA256, "-ExpectedReviewerResultSha256", preview.ReviewerResultSHA256, "-Apply", "-Format", "json")
	if err := Run(applyArgs, &out); err != nil {
		t.Fatal(err)
	}
	var applied subagents.ReviewerResultRecoveryResult
	if err := json.Unmarshal(out.Bytes(), &applied); err != nil {
		t.Fatal(err)
	}
	if !applied.Applied || applied.MissionCommanderAction.State != "reviewer-result-recovered-ready-for-collection-preview" {
		t.Fatalf("unexpected recovery apply: %+v", applied)
	}
	out.Reset()
	if err := Run([]string{"-Command", "plan-subagents", "-PacketPath", plan.PacketPath, "-CollectReviewerResult", "-ShardId", handoff.ShardID, "-Lane", "feature-review", "-Actor", "mission-commander", "-WhatIf", "-Format", "json"}, &out); err != nil {
		t.Fatal(err)
	}
	out.Reset()
	if err := Run([]string{"-Command", "plan-subagents", "-PacketPath", plan.PacketPath, "-CollectReviewerResult", "-ShardId", handoff.ShardID, "-Lane", "feature-review", "-Actor", "mission-commander", "-Apply", "-Format", "json"}, &out); err != nil {
		t.Fatal(err)
	}
	out.Reset()
	if err := Run([]string{"-Command", "plan-subagents", "-PacketPath", plan.PacketPath, "-ReadyReviewerResults", "-Lane", "feature-review", "-Actor", "mission-commander", "-WhatIf", "-Format", "json"}, &out); err != nil {
		t.Fatal(err)
	}
}

func TestRunPlanSubagentsReviewerPacketRetirementWhatIfApplyE2E(t *testing.T) {
	caseRoot := fullAttachedCase(t)
	var out bytes.Buffer
	if err := Run([]string{"-Command", "start", "-Target", caseRoot, "-Pack", "_template", "-Name", "review", "-Apply"}, &out); err != nil {
		t.Fatal(err)
	}
	out.Reset()
	if err := Run([]string{"-Command", "plan-subagents", "-Target", caseRoot, "-Pack", "_template", "-TaskType", "feature-analysis", "-Items", "alpha", "-Lane", "feature-review", "-Format", "json"}, &out); err != nil {
		t.Fatal(err)
	}
	plan := decodePlanSubagentsResult(t, out.Bytes())
	if err := os.WriteFile(plan.PacketPath, []byte("{truncated"), 0o644); err != nil {
		t.Fatal(err)
	}

	args := []string{"-Command", "plan-subagents", "-Target", caseRoot, "-Pack", "_template", "-PacketPath", plan.PacketPath, "-RetireInvalidReviewerPacket", "-Lane", "feature-review", "-Actor", "mission-commander", "-Reason", "retire exact invalid packet"}
	out.Reset()
	if err := Run(append(append([]string{}, args...), "-WhatIf", "-Format", "json"), &out); err != nil {
		t.Fatal(err)
	}
	var preview subagents.ReviewerPacketRetirementResult
	if err := json.Unmarshal(out.Bytes(), &preview); err != nil {
		t.Fatal(err)
	}
	if preview.Applied || preview.MissionCommanderAction.State != "needs-reviewer-packet-retirement-apply" {
		t.Fatalf("unexpected retirement preview: %+v", preview)
	}

	out.Reset()
	if err := Run(append(append([]string{}, args...), "-WhatIf", "-Format", "text"), &out); err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(out.String(), "plan-subagents reviewer packet retirement：") || strings.HasPrefix(strings.TrimSpace(out.String()), "{") {
		t.Fatalf("unexpected retirement text output: %s", out.String())
	}

	applyArgs := append(append([]string{}, args...), "-ExpectedPacketSha256", preview.PacketSHA256, "-ExpectedIntegritySha256", preview.IntegritySHA256)
	out.Reset()
	if err := Run(append(applyArgs, "-Apply", "-Format", "json"), &out); err != nil {
		t.Fatal(err)
	}
	var applied subagents.ReviewerPacketRetirementResult
	if err := json.Unmarshal(out.Bytes(), &applied); err != nil {
		t.Fatal(err)
	}
	if !applied.Applied || applied.MissionCommanderAction.State != "reviewer-packet-retired" {
		t.Fatalf("unexpected retirement apply: %+v", applied)
	}
}
