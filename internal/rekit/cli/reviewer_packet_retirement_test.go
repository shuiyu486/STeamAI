package cli

import (
	"bytes"
	"encoding/json"
	"os"
	"strings"
	"testing"

	"github.com/shuiyu486/re-context-kits/internal/rekit/subagents"
)

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
