package cli

import (
	"bytes"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/shuiyu486/re-context-kits/internal/rekit/mission"
	"github.com/shuiyu486/re-context-kits/internal/rekit/note"
	"github.com/shuiyu486/re-context-kits/internal/rekit/subagents"
)

func TestRunPlanSubagentsReviewerIntakeWhatIfApplyE2E(t *testing.T) {
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
	evidencePath := filepath.Join(caseRoot, "workspace", "review-evidence.md")
	if err := os.WriteFile(evidencePath, []byte("bounded reviewer evidence\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	resultPath := filepath.Join(t.TempDir(), "reviewer-result.json")
	reviewerResult := map[string]any{
		"packetId":           packet.PacketID,
		"routeId":            packet.Route.ID,
		"shardId":            "shard-01",
		"items":              []string{"alpha"},
		"reviewerSession":    "reviewer-session-cli",
		"decision":           "accept",
		"confidence":         "high",
		"summary":            "reviewed alpha against bounded evidence",
		"evidenceRefs":       []string{"workspace/review-evidence.md"},
		"risks":              []string{},
		"conflicts":          []string{},
		"recommendedVerdict": "accepted",
		"routeOutput": map[string]any{
			"item": "alpha", "decision": "accept", "confidence": "high", "evidence": "workspace/review-evidence.md", "risk": "low", "next_action": "main-agent-writeback", "tier_used": "light", "tool_scope": "read-only", "feature": "review", "request_id": "n/a", "candidate_path": "n/a", "defer_reason": "n/a",
		},
	}
	data, err := json.Marshal(reviewerResult)
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(resultPath, data, 0o644); err != nil {
		t.Fatal(err)
	}

	out.Reset()
	if err := Run([]string{"-Command", "plan-subagents", "-Target", caseRoot, "-Pack", "_template", "-PacketPath", plan.PacketPath, "-ReviewerResultPath", resultPath, "-Lane", packet.TargetLane, "-Actor", "mission-commander", "-WhatIf", "-Format", "json"}, &out); err != nil {
		t.Fatal(err)
	}
	preview := decodeReviewerIntakeResult(t, out.Bytes())
	if preview.Command != "plan-subagents" || preview.Mode != "reviewer-intake" || preview.IsMutation || preview.Applied || preview.WritebackStatus != "previewed" || !preview.ReadyForWriteback || preview.Verification == nil || preview.Decision == nil || preview.PostValidation == nil || !preview.PostValidation.Valid {
		t.Fatalf("unexpected reviewer intake preview: %+v", preview)
	}
	if preview.Verification.Applied || preview.Decision.Applied || preview.Verification.Event["verdict"] != "accepted" || preview.Decision.Event["decision"] != "accept" {
		t.Fatalf("unexpected reviewer intake event previews: verification=%+v decision=%+v", preview.Verification, preview.Decision)
	}
	if preview.MissionCommanderAction.State != "ready-for-reviewer-intake-apply" || !strings.Contains(preview.MissionCommanderAction.PrimaryCommand, "-Apply -Format json") || !containsMissionCommanderNextAction(preview.MissionCommanderNextActions, "reviewerIntake.previewed", preview.MissionCommanderAction.PrimaryCommand, false, true) || !containsMissionCommanderNextAction(preview.MissionCommanderNextActions, "reviewerIntake.previewed.followUp", "/rekit handoff main", false, true) {
		t.Fatalf("preview omitted reviewer intake Mission Commander guidance: action=%+v next=%+v", preview.MissionCommanderAction, preview.MissionCommanderNextActions)
	}
	assertCLIActionQueue(t, preview.MissionCommanderActionQueue, 2, 2, 0, 2, 1, preview.MissionCommanderAction.PrimaryCommand)

	out.Reset()
	if err := Run([]string{"-Command", "plan-subagents", "-Target", caseRoot, "-Pack", "_template", "-PacketPath", plan.PacketPath, "-ReviewerResultPath", resultPath, "-Lane", packet.TargetLane, "-Actor", "mission-commander", "-WhatIf", "-Format", "text"}, &out); err != nil {
		t.Fatal(err)
	}
	for _, expected := range []string{
		"plan-subagents reviewer intake：status=previewed mutation=false applied=false readyForWriteback=true lane=main shard=shard-01",
		"reviewer intake orchestration：mode=manual-main-agent-intake dispatch=1/1 shardBefore=planned shardAfter=previewed",
		"reviewer intake verification：applied=false",
		"reviewer intake decision：applied=false",
		"reviewer intake post-validation：valid=true",
		"reviewer intake commander action：state=ready-for-reviewer-intake-apply",
		"mission commander action queue：summary=total=2 unblocked=2 blocked=0 requiresReview=2 followUp=1 current=/rekit plan-subagents",
		"mission commander action queue current：state=ready-for-reviewer-intake-apply source=reviewerIntake.previewed blocked=false requiresReview=true command=`/rekit plan-subagents",
		"mission commander next action：state=ready-for-reviewer-intake-apply source=reviewerIntake.previewed blocked=false requiresReview=true command=`/rekit plan-subagents",
	} {
		if !strings.Contains(out.String(), expected) {
			t.Fatalf("reviewer intake text output missing %q:\n%s", expected, out.String())
		}
	}

	out.Reset()
	if err := Run([]string{"-Command", "plan-subagents", "-Target", caseRoot, "-Pack", "_template", "-PacketPath", plan.PacketPath, "-ReviewerResultPath", resultPath, "-Lane", packet.TargetLane, "-Actor", "mission-commander", "-Apply", "-Format", "json"}, &out); err != nil {
		t.Fatal(err)
	}
	applied := decodeReviewerIntakeResult(t, out.Bytes())
	if !applied.IsMutation || !applied.Applied || applied.WritebackStatus != "complete" || applied.Verification == nil || applied.Decision == nil || !applied.Verification.Applied || !applied.Decision.Applied || applied.PostValidation == nil || !applied.PostValidation.Valid || applied.PostValidation.Handoff.ExecutorAction == nil {
		t.Fatalf("unexpected reviewer intake apply: %+v", applied)
	}
	if applied.MissionCommanderAction.State != "reviewer-intake-writeback-complete" || len(applied.MissionCommanderNextActions) == 0 || !strings.HasPrefix(applied.MissionCommanderNextActions[0].Source, "reviewerIntake.postValidation.") {
		t.Fatalf("apply omitted reviewer intake Mission Commander guidance: action=%+v next=%+v", applied.MissionCommanderAction, applied.MissionCommanderNextActions)
	}
	if applied.MissionCommanderActionQueue.Counts.Total != len(applied.MissionCommanderNextActions) || applied.MissionCommanderActionQueue.CurrentAction == nil || !strings.HasPrefix(applied.MissionCommanderActionQueue.CurrentAction.Source, "reviewerIntake.postValidation.") {
		t.Fatalf("apply omitted reviewer intake Mission Commander action queue: queue=%+v next=%+v", applied.MissionCommanderActionQueue, applied.MissionCommanderNextActions)
	}
	if applied.PostValidation.Overview.Sections.Verifications.Total != 1 || applied.PostValidation.Overview.Sections.Decisions.Total != 1 || applied.PostValidation.Handoff.Lane == nil || applied.PostValidation.Handoff.Lane.ID != packet.TargetLane {
		t.Fatalf("post-review validation omitted ledger/handoff state: %+v", applied.PostValidation)
	}
	verificationLedger, err := os.ReadFile(filepath.Join(caseRoot, ".rekit", "facts", "verifications.jsonl"))
	if err != nil {
		t.Fatal(err)
	}
	decisionLedger, err := os.ReadFile(filepath.Join(caseRoot, ".rekit", "facts", "decisions.jsonl"))
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(verificationLedger), applied.Verification.EventID) || !strings.Contains(string(decisionLedger), applied.Verification.EventID) || !strings.Contains(string(decisionLedger), applied.Decision.EventID) {
		t.Fatalf("reviewer writeback ledger evidence linkage missing:\nverification=%s\ndecision=%s", verificationLedger, decisionLedger)
	}
}

func TestRunPlanSubagentsReviewerIntakeRequiresExplicitMode(t *testing.T) {
	caseRoot := fullAttachedCase(t)
	var out bytes.Buffer
	err := Run([]string{"-Command", "plan-subagents", "-Target", caseRoot, "-Pack", "_template", "-PacketPath", "packet.json", "-ReviewerResultPath", "result.json", "-Lane", "main", "-Actor", "mission-commander"}, &out)
	if err == nil || !strings.Contains(err.Error(), "requires -WhatIf or -Apply") {
		t.Fatalf("error = %v, want explicit reviewer intake mode guard", err)
	}
}

func TestRunPlanSubagentsReviewerIntakeEmitsPartialRecoveryJSON(t *testing.T) {
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
	evidencePath := filepath.Join(caseRoot, "workspace", "review-evidence.md")
	if err := os.WriteFile(evidencePath, []byte("bounded reviewer evidence\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	resultPath := filepath.Join(t.TempDir(), "reviewer-result.json")
	routeOutput := map[string]any{
		"item": "alpha", "decision": "accept", "confidence": "high", "evidence": "workspace/review-evidence.md", "risk": "low", "next_action": "main-agent-writeback", "tier_used": "light", "tool_scope": "read-only", "feature": "review", "request_id": "n/a", "candidate_path": "n/a", "defer_reason": "n/a",
	}
	reviewerResult := map[string]any{"packetId": packet.PacketID, "routeId": packet.Route.ID, "shardId": "shard-01", "items": []string{"alpha"}, "reviewerSession": "reviewer-session-cli", "decision": "accept", "confidence": "high", "summary": "reviewed alpha", "evidenceRefs": []string{"workspace/review-evidence.md"}, "risks": []string{}, "conflicts": []string{}, "recommendedVerdict": "accepted", "routeOutput": routeOutput}
	data, err := json.Marshal(reviewerResult)
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(resultPath, data, 0o644); err != nil {
		t.Fatal(err)
	}
	originalIntake := intakeReviewerResult
	intakeReviewerResult = func(repoRoot, caseRoot, pack string, opt subagents.ReviewerIntakeOptions) (subagents.ReviewerIntakeResult, error) {
		primary := "/rekit plan-subagents -Target \"" + caseRoot + "\" -Pack \"" + pack + "\" -PacketPath \"" + opt.PacketPath + "\" -ReviewerResultPath \"" + opt.ReviewerResultPath + "\" -Lane \"" + opt.Lane + "\" -Actor \"" + opt.Actor + "\" -Apply -Format json"
		nextActions := []mission.MissionCommanderNextActionItem{{
			Lane:           opt.Lane,
			State:          "reviewer-intake-partial-writeback",
			Command:        primary,
			Source:         "reviewerIntake.verification-recorded",
			RequiresReview: true,
		}}
		return subagents.ReviewerIntakeResult{
			SchemaVersion:   1,
			Command:         "plan-subagents",
			Mode:            "reviewer-intake",
			WritebackStatus: "verification-recorded",
			Lane:            opt.Lane,
			Verification:    &note.AppendResult{Applied: true, EventID: "evt-review-test-verification"},
			Decision:        &note.AppendResult{Applied: false, EventID: "evt-review-test-decision"},
			MissionCommanderAction: mission.MissionCommanderAction{
				State:          "reviewer-intake-partial-writeback",
				PrimaryCommand: primary,
				Boundary:       []string{"retry the identical apply command; do not hand-write the missing decision event"},
			},
			MissionCommanderNextActions: nextActions,
			MissionCommanderActionQueue: mission.MissionCommanderActionQueueFor(nextActions),
			NextSteps:                   []string{"retry the identical reviewer intake apply"},
		}, fmt.Errorf("injected decision append failure; writebackStatus=verification-recorded")
	}
	defer func() { intakeReviewerResult = originalIntake }()

	out.Reset()
	err = Run([]string{"-Command", "plan-subagents", "-Target", caseRoot, "-Pack", "_template", "-PacketPath", plan.PacketPath, "-ReviewerResultPath", resultPath, "-Lane", packet.TargetLane, "-Actor", "mission-commander", "-Apply", "-Format", "json"}, &out)
	if err == nil || !strings.Contains(err.Error(), "verification-recorded") {
		t.Fatalf("error = %v, want partial recovery diagnostic", err)
	}
	partial := decodeReviewerIntakeResult(t, out.Bytes())
	if partial.WritebackStatus != "verification-recorded" || partial.Verification == nil || !partial.Verification.Applied || partial.Decision == nil || partial.Decision.Applied {
		t.Fatalf("CLI omitted reviewer intake partial recovery JSON: %+v", partial)
	}
	if partial.MissionCommanderAction.State != "reviewer-intake-partial-writeback" || !strings.Contains(partial.MissionCommanderAction.PrimaryCommand, "-Apply -Format json") || !containsMissionCommanderNextAction(partial.MissionCommanderNextActions, "reviewerIntake.verification-recorded", partial.MissionCommanderAction.PrimaryCommand, false, true) {
		t.Fatalf("CLI omitted partial recovery Mission Commander retry guidance: action=%+v next=%+v", partial.MissionCommanderAction, partial.MissionCommanderNextActions)
	}
	assertCLIActionQueue(t, partial.MissionCommanderActionQueue, 1, 1, 0, 1, 0, partial.MissionCommanderAction.PrimaryCommand)
}

type reviewerIntakeCLIResult struct {
	Command               string `json:"command"`
	Mode                  string `json:"mode"`
	IsMutation            bool   `json:"isMutation"`
	Applied               bool   `json:"applied"`
	WritebackStatus       string `json:"writebackStatus"`
	ReadyForWriteback     bool   `json:"readyForWriteback"`
	OrchestrationSnapshot struct {
		Mode              string   `json:"mode"`
		ReviewerCount     int      `json:"reviewerCount"`
		DispatchIndex     int      `json:"dispatchIndex"`
		DispatchTotal     int      `json:"dispatchTotal"`
		ShardStatusBefore string   `json:"shardStatusBefore"`
		ShardStatusAfter  string   `json:"shardStatusAfter"`
		NextDispatches    []string `json:"nextDispatches"`
	} `json:"orchestrationSnapshot"`
	MissionCommanderAction      missionCommanderActionSnapshot      `json:"missionCommanderAction"`
	MissionCommanderNextActions []missionCommanderNextActionItem    `json:"missionCommanderNextActions"`
	MissionCommanderActionQueue missionCommanderActionQueueSnapshot `json:"missionCommanderActionQueue"`
	Verification                *struct {
		Applied bool           `json:"applied"`
		EventID string         `json:"eventId"`
		Event   map[string]any `json:"event"`
	} `json:"verification"`
	Decision *struct {
		Applied bool           `json:"applied"`
		EventID string         `json:"eventId"`
		Event   map[string]any `json:"event"`
	} `json:"decision"`
	PostValidation *struct {
		Overview struct {
			Sections struct {
				Verifications struct {
					Total int `json:"total"`
				} `json:"verifications"`
				Decisions struct {
					Total int `json:"total"`
				} `json:"decisions"`
			} `json:"sections"`
		} `json:"overview"`
		Handoff struct {
			Lane *struct {
				ID string `json:"id"`
			} `json:"lane"`
			ExecutorAction map[string]any `json:"executorAction"`
		} `json:"handoff"`
		Valid bool `json:"valid"`
	} `json:"postValidation"`
}

func decodeReviewerIntakeResult(t *testing.T, data []byte) reviewerIntakeCLIResult {
	t.Helper()
	var result reviewerIntakeCLIResult
	if err := json.Unmarshal(data, &result); err != nil {
		t.Fatalf("reviewer intake stdout is not JSON: %v\n%s", err, string(data))
	}
	return result
}
