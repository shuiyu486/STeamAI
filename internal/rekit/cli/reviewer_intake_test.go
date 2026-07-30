package cli

import (
	"bytes"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"slices"
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
	resultPath := plan.ShardHandoffs[0].ReviewerResultPath
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
	stageAndCollectReviewerResultForCLIPlan(t, &out, []string{"-Target", caseRoot, "-Pack", "_template"}, plan.PacketPath, plan.ShardHandoffs[0], packet.TargetLane, "mission-commander", data)

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
	if preview.Summary.Status != "previewed" || !preview.Summary.ReadyForWriteback || preview.Summary.Applied || preview.Summary.Lane != packet.TargetLane || preview.Summary.ShardID != "shard-01" || preview.Summary.DispatchIndex != 1 || preview.Summary.DispatchTotal != 1 || preview.Summary.BlockedCount != 0 || !preview.Summary.PostValidationPresent || !preview.Summary.PostValidationValid || preview.Summary.CurrentAction == nil || preview.Summary.ActionTotal != 2 || !containsStringWith(preview.Summary.Boundary, "intake summary is read-only") {
		t.Fatalf("preview compact summary omitted intake state: %+v", preview.Summary)
	}

	out.Reset()
	if err := Run([]string{"-Command", "plan-subagents", "-Target", caseRoot, "-Pack", "_template", "-PacketPath", plan.PacketPath, "-ReviewerResultPath", resultPath, "-Lane", packet.TargetLane, "-Actor", "mission-commander", "-WhatIf", "-Format", "text"}, &out); err != nil {
		t.Fatal(err)
	}
	for _, expected := range []string{
		"plan-subagents reviewer intake：status=previewed mutation=false applied=false readyForWriteback=true lane=main shard=shard-01",
		"reviewer intake summary：status=previewed readyForWriteback=true applied=false lane=main shard=shard-01",
		"reviewerSession=reviewer-session-cli verification=accepted decision=accept dispatch=1/1 shardBefore=planned shardAfter=previewed blocked=0 repairs=0 postValidation=true valid=true",
		"reviewer intake summary current action：state=ready-for-reviewer-intake-apply source=reviewerIntake.previewed blocked=false requiresReview=true command=`/rekit plan-subagents",
		"lane=main label=main gateEventId= actionId=",
		"reviewer intake summary current action reason：WhatIf preview passed; apply only after main-agent evidence review",
		"reviewer intake summary current action boundary：do not execute heavy tools from reviewer intake",
		"reviewer intake summary next action：state=ready-for-reviewer-intake-apply source=reviewerIntake.previewed blocked=false requiresReview=true command=`/rekit plan-subagents",
		"reviewer intake summary boundary：intake summary is read-only; full reviewer result, note writebacks, orchestration snapshot, postValidation, and action queue remain available",
		"reviewer intake summary boundary：consume postValidation summary before continuing or handing off the lane",
		"reviewer intake result：packetId=" + packet.PacketID + " routeId=" + packet.Route.ID + " shard=shard-01 decision=accept confidence=high reviewerSession=reviewer-session-cli recommendedVerdict=accepted",
		"reviewer intake result summary：reviewed alpha against bounded evidence",
		"reviewer intake result evidenceRefs：workspace/review-evidence.md",
		"reviewer intake route output：tool_scope=read-only",
		"reviewer intake orchestration：mode=manual-main-agent-intake dispatch=1/1 shardBefore=planned shardAfter=previewed",
		"reviewer intake verification：applied=false",
		"reviewer intake verification event：kind=verification eventId=evt-",
		"reviewer intake verification event field：verdict=accepted",
		"reviewer intake verification event field：reviewerSession=reviewer-session-cli",
		"reviewer intake verification note handoff：mutation=false applied=false reason=what-if eventId=evt-",
		"note append：mutation=false applied=false reason=what-if eventId=evt-",
		"path=.rekit/facts/verifications.jsonl kind=verification lane=main",
		"note event：eventId=evt-",
		"verdict=accepted",
		"note event field：eventId=evt-",
		"key=reviewerSession value=reviewer-session-cli",
		"note mission brief：summary=",
		"note executor action：blocked=false ready=true",
		"note would executor action：blocked=false ready=true",
		"note would mission commander next action：state=ready-to-continue",
		"reviewer intake decision：applied=false",
		"reviewer intake decision event：kind=decision eventId=evt-",
		"reviewer intake decision event field：decision=accept",
		"reviewer intake decision event field：reason=validated bounded reviewer intake",
		"reviewer intake decision note handoff：mutation=false applied=false reason=what-if eventId=evt-",
		"path=.rekit/facts/decisions.jsonl kind=decision lane=main",
		"decision=accept",
		"note would executor action：blocked=",
		"note would executor action commander action：state=",
		"note would mission commander next action：state=",
		"reviewer intake post-validation：valid=true overviewVerifications=0 overviewDecisions=0 doctorRows=",
		"reviewer intake post-validation summary：valid=true",
		"lane=main project=false executorAction=true ready=",
		"reviewer intake post-validation summary current action：state=",
		"reviewer intake post-validation summary current action reason：active reviewer packet must be resolved before ordinary lane continuation",
		"reviewer intake post-validation summary current action boundary：reviewer dispatch intake handoff is read-only; full packet.json and reviewerOrchestration remain source of truth",
		"reviewer intake post-validation summary next action：state=",
		"reviewer intake post-validation summary boundary：postValidation summary is read-only; full overview/handoff/doctor snapshots remain available",
		"reviewer intake post-validation handoff：lane=main project=false executorAction=true",
		"reviewer intake post-validation handoff queue：summary=total=",
		"reviewer intake post-validation handoff queue current：state=",
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
	if applied.Summary.Status != "complete" || !applied.Summary.Applied || applied.Summary.PostValidationOverviewVerifications != 1 || applied.Summary.PostValidationOverviewDecisions != 1 || applied.Summary.ReviewerWritebacks != len(applied.PostValidation.Handoff.ReviewerWritebacks) || applied.Summary.ReviewerWritebackSummary == nil || applied.Summary.ReviewerWritebackSummary.Total != 2 || applied.Summary.ReviewerWritebackSummary.LatestReviewerResult != resultPath || applied.Summary.CurrentAction == nil || !strings.HasPrefix(applied.Summary.CurrentAction.Source, "reviewerIntake.postValidation.") || !containsStringWith(applied.Summary.Boundary, "consume postValidation summary") {
		t.Fatalf("apply compact summary omitted post-validation progress: %+v", applied.Summary)
	}
	if !applied.PostValidation.Summary.Valid || applied.PostValidation.Summary.OverviewVerifications != 1 || applied.PostValidation.Summary.OverviewDecisions != 1 || applied.PostValidation.Summary.Lane != packet.TargetLane || !applied.PostValidation.Summary.ExecutorActionPresent || applied.PostValidation.Summary.ReviewerWritebackSummary == nil || applied.PostValidation.Summary.ReviewerWritebackSummary.Total != 2 || applied.PostValidation.Summary.ReviewerWritebackSummary.LatestReviewerResult != resultPath || applied.PostValidation.Summary.CurrentAction == nil || len(applied.PostValidation.Summary.NextActions) == 0 || !containsStringWith(applied.PostValidation.Summary.Boundary, "postValidation summary is read-only") || !strings.HasPrefix(applied.PostValidation.Summary.CurrentAction.Source, "missionCommanderActions") {
		t.Fatalf("post-review validation compact summary omitted takeover state: %+v", applied.PostValidation.Summary)
	}

	out.Reset()
	if err := Run([]string{"-Command", "plan-subagents", "-Target", caseRoot, "-Pack", "_template", "-PacketPath", plan.PacketPath, "-ReviewerResultPath", resultPath, "-Lane", packet.TargetLane, "-Actor", "mission-commander", "-WhatIf", "-Format", "text"}, &out); err != nil {
		t.Fatal(err)
	}
	for _, expected := range []string{
		"plan-subagents reviewer intake：status=already-complete mutation=false applied=false readyForWriteback=true lane=main shard=shard-01",
		"reviewer intake summary：status=already-complete readyForWriteback=true applied=false lane=main shard=shard-01",
		"postValidation=true valid=true postValidationVerifications=1 postValidationDecisions=1 reviewerWritebacks=2",
		"reviewer intake summary post-validation reviewer writeback summary：total=2 verifications=1 decisions=1 lanes=1 latestKind=decision",
		"reviewer intake summary current action：state=ready-to-continue source=reviewerIntake.postValidation.missionCommanderActions blocked=false requiresReview=false command=`/rekit continue main`",
		"reviewer intake summary next action：state=ready-to-continue source=reviewerIntake.postValidation.missionCommanderActions blocked=false requiresReview=false command=`/rekit continue main`",
		"reviewer intake summary boundary：intake summary is read-only; full reviewer result, note writebacks, orchestration snapshot, postValidation, and action queue remain available",
		"reviewer intake result：packetId=" + packet.PacketID + " routeId=" + packet.Route.ID + " shard=shard-01 decision=accept confidence=high reviewerSession=reviewer-session-cli recommendedVerdict=accepted",
		"reviewer intake result items：alpha",
		"reviewer intake route output：next_action=main-agent-writeback",
		"reviewer intake post-validation：valid=true overviewVerifications=1 overviewDecisions=1 doctorRows=",
		"reviewer intake post-validation summary：valid=true",
		"lane=main project=false executorAction=true ready=",
		"reviewer intake post-validation summary reviewer writeback summary：total=2 verifications=1 decisions=1 lanes=1 latestKind=decision",
		"reviewer intake post-validation summary current action：state=ready-to-continue source=missionCommanderActions blocked=false requiresReview=false command=`/rekit continue main` lane=main label=main",
		"reviewer intake post-validation summary current action reason：ready lane primary action",
		"reviewer intake post-validation summary current action boundary：no heavy-tool execution",
		"reviewer intake post-validation summary next action：state=ready-to-continue source=missionCommanderActions",
		"reviewer intake post-validation summary boundary：postValidation summary is read-only; full overview/handoff/doctor snapshots remain available",
		"reviewer intake post-validation handoff：lane=main project=false executorAction=true",
		"reviewer intake post-validation reviewer writeback：kind=verification eventId=evt-",
		"reviewer intake post-validation reviewer result：eventId=evt-",
		"reviewer intake post-validation reviewer owner：eventId=evt-",
		"reviewer intake post-validation reviewer decision detail：eventId=evt-",
		"reviewer intake post-validation reviewer route output：eventId=evt-",
		"reviewer intake post-validation reviewer evidence ref：eventId=evt-",
		"reviewer intake post-validation handoff queue：summary=total=",
		"reviewer intake post-validation handoff queue current：state=",
		"reviewer intake verification note handoff：mutation=false applied=false reason=duplicate eventId eventId=evt-",
		"note append：mutation=false applied=false reason=duplicate eventId eventId=evt-",
		"reviewer intake decision note handoff：mutation=false applied=false reason=duplicate eventId eventId=evt-",
		"reviewer intake post-validation next action：state=ready-to-continue source=missionCommanderActions",
		"reviewer intake commander action：state=reviewer-intake-already-complete",
		"mission commander action queue current：state=ready-to-continue source=reviewerIntake.postValidation.missionCommanderActions",
	} {
		if !strings.Contains(out.String(), expected) {
			t.Fatalf("reviewer intake already-complete text output missing %q:\n%s", expected, out.String())
		}
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

	factsBeforeContinue := snapshotFiles(t, filepath.Join(caseRoot, ".rekit", "facts"))
	out.Reset()
	if err := Run([]string{"-Command", "continue", "-Target", caseRoot, "-Pack", "_template", "main", "-Apply", "-Format", "json"}, &out); err != nil {
		t.Fatal(err)
	}
	var continueApply struct {
		RunID                       string                              `json:"runId"`
		Applied                     bool                                `json:"applied"`
		Blocked                     bool                                `json:"blocked"`
		ReviewerWritebacks          []reviewerWritebackCLIItem          `json:"reviewerWritebacks"`
		ReviewerWritebackSummary    reviewerWritebackSummaryCLIItem     `json:"reviewerWritebackSummary"`
		MissionCommanderActionQueue missionCommanderActionQueueSnapshot `json:"missionCommanderActionQueue"`
		Writes                      []startWrite                        `json:"writes"`
	}
	if err := json.Unmarshal(out.Bytes(), &continueApply); err != nil {
		t.Fatalf("continue after reviewer writeback stdout is not JSON: %v\n%s", err, out.String())
	}
	if !continueApply.Applied || continueApply.Blocked || continueApply.RunID == "" || continueApply.RunID == "run-preview" || len(continueApply.ReviewerWritebacks) != 2 || continueApply.ReviewerWritebackSummary.Total != 2 || continueApply.ReviewerWritebackSummary.VerificationCount != 1 || continueApply.ReviewerWritebackSummary.DecisionCount != 1 || continueApply.ReviewerWritebackSummary.LatestReviewerResult != resultPath || continueApply.ReviewerWritebackSummary.LatestReviewerSession != "reviewer-session-cli" || !continueApply.ReviewerWritebackSummary.HasReviewerResult || !continueApply.ReviewerWritebackSummary.HasOwnerBinding || !continueApply.ReviewerWritebackSummary.HasRouteOutput {
		t.Fatalf("continue after reviewer writeback omitted writeback summary: %+v", continueApply)
	}
	if continueApply.MissionCommanderActionQueue.CurrentAction == nil || continueApply.MissionCommanderActionQueue.CurrentAction.Command != "/rekit continue main" {
		t.Fatalf("continue after reviewer writeback should remain ready for replacement executor continuation: %+v", continueApply.MissionCommanderActionQueue)
	}
	resumePath := assertStartWrite(t, continueApply.Writes, ".rekit/lanes/main/prompts/RESUME.md", "refresh").TargetPath
	checkpointPath := assertStartWrite(t, continueApply.Writes, ".rekit/lanes/main/checkpoints/latest.json", "refresh").TargetPath
	statusPath := assertStartWrite(t, continueApply.Writes, ".rekit/runs/"+continueApply.RunID+"/status.json", "write").TargetPath
	digestPath := assertStartWrite(t, continueApply.Writes, ".rekit/runs/"+continueApply.RunID+"/digest.md", "write").TargetPath
	assertSnapshotEqual(t, factsBeforeContinue, snapshotFiles(t, filepath.Join(caseRoot, ".rekit", "facts")))

	statusBytes, err := os.ReadFile(statusPath)
	if err != nil {
		t.Fatal(err)
	}
	var runStatus struct {
		ReviewerWritebacks       []reviewerWritebackCLIItem      `json:"reviewerWritebacks"`
		ReviewerWritebackSummary reviewerWritebackSummaryCLIItem `json:"reviewerWritebackSummary"`
	}
	if err := json.Unmarshal(statusBytes, &runStatus); err != nil {
		t.Fatalf("continue run status after reviewer writeback did not decode: %v\n%s", err, string(statusBytes))
	}
	if len(runStatus.ReviewerWritebacks) != 2 || runStatus.ReviewerWritebackSummary.Total != 2 || runStatus.ReviewerWritebackSummary.LatestReviewerResult != resultPath {
		t.Fatalf("continue run status omitted reviewer writeback provenance: %+v", runStatus)
	}

	resume, err := os.ReadFile(resumePath)
	if err != nil {
		t.Fatal(err)
	}
	digest, err := os.ReadFile(digestPath)
	if err != nil {
		t.Fatal(err)
	}
	for label, text := range map[string]string{"lane RESUME": string(resume), "continue digest": string(digest)} {
		for _, expected := range []string{"## Reviewer writeback", "summary: total=`2` verifications=`1` decisions=`1`", "latestShard=`shard-01`", "reviewer-session-cli", "reviewer result: `" + resultPath + "`", "reviewer decision detail: reviewerDecision=accept recommendedVerdict=accepted", "reviewer writeback downstream handoff must not execute heavy tools or spawn reviewer sessions"} {
			if !strings.Contains(text, expected) {
				t.Fatalf("%s after reviewer writeback omitted durable handoff %q:\n%s", label, expected, text)
			}
		}
	}
	checkpointBytes, err := os.ReadFile(checkpointPath)
	if err != nil {
		t.Fatal(err)
	}
	var checkpoint struct {
		ReviewerWritebackSummary    reviewerWritebackSummaryCLIItem     `json:"reviewerWritebackSummary"`
		MissionCommanderActionQueue missionCommanderActionQueueSnapshot `json:"missionCommanderActionQueue"`
	}
	if err := json.Unmarshal(checkpointBytes, &checkpoint); err != nil {
		t.Fatalf("checkpoint after reviewer writeback did not decode: %v\n%s", err, string(checkpointBytes))
	}
	if checkpoint.ReviewerWritebackSummary.Total != 2 || checkpoint.ReviewerWritebackSummary.LatestReviewerResult != resultPath || checkpoint.MissionCommanderActionQueue.CurrentAction == nil || checkpoint.MissionCommanderActionQueue.CurrentAction.Command != "/rekit continue main" {
		t.Fatalf("checkpoint after reviewer writeback omitted durable continuation handoff: %+v", checkpoint)
	}
}

func TestRunPlanSubagentsReviewerPacketAdoptionCaseLocalProductPath(t *testing.T) {
	caseRoot := fullAttachedCase(t)
	var out bytes.Buffer
	if err := Run([]string{"-Command", "start", "-Target", caseRoot, "-Pack", "_template", "-Name", "review", "-Executor", "session-a", "-Actor", "mission-commander", "-Reason", "initial reviewer owner", "-Apply", "-Format", "json"}, &out); err != nil {
		t.Fatal(err)
	}
	out.Reset()
	if err := Run([]string{"-Command", "plan-subagents", "-Target", caseRoot, "-Pack", "_template", "-TaskType", "feature-analysis", "-Items", "alpha", "-Lane", "feature-review", "-Format", "json"}, &out); err != nil {
		t.Fatal(err)
	}
	plan := decodePlanSubagentsResult(t, out.Bytes())
	packet := decodePlanSubagentsPacket(t, plan.PacketPath)
	packetBefore, err := os.ReadFile(plan.PacketPath)
	if err != nil {
		t.Fatal(err)
	}
	writeCaseFile(t, caseRoot, "workspace/features/feature-login/review-evidence.md", "bounded reviewer adoption evidence\n")
	reviewerResult := reviewerResultForCLIPlan(t, packet, plan.ShardHandoffs[0], "accept", "accepted", "reviewer-adoption-e2e")
	stageAndCollectReviewerResultForCLIPlan(t, &out, []string{"-Target", caseRoot, "-Pack", "_template"}, plan.PacketPath, plan.ShardHandoffs[0], "feature-review", "mission-commander", reviewerResult)

	out.Reset()
	if err := Run([]string{"-Command", "start", "-Target", caseRoot, "-Pack", "_template", "-Name", "review", "-Executor", "session-b", "-Actor", "mission-commander", "-Reason", "replacement reviewer owner", "-Apply", "-Format", "json"}, &out); err != nil {
		t.Fatal(err)
	}
	out.Reset()
	if err := Run([]string{"-Command", "status", "-Target", caseRoot, "-Pack", "_template", "-Format", "json"}, &out); err != nil {
		t.Fatal(err)
	}
	var status struct {
		CaseMission struct {
			MissionCommanderActionQueue missionCommanderActionQueueSnapshot `json:"missionCommanderActionQueue"`
		} `json:"caseMission"`
	}
	if err := json.Unmarshal(out.Bytes(), &status); err != nil {
		t.Fatal(err)
	}
	if status.CaseMission.MissionCommanderActionQueue.CurrentAction == nil || status.CaseMission.MissionCommanderActionQueue.CurrentAction.State != "reviewer-packet-owner-adoption-required" {
		t.Fatalf("status did not promote reviewer packet adoption: %+v", status.CaseMission.MissionCommanderActionQueue)
	}
	out.Reset()
	if err := Run([]string{"-Command", "plan-subagents", "-Target", caseRoot, "-Pack", "_template", "-PacketPath", plan.PacketPath, "-ReadyReviewerResults", "-Lane", "feature-review", "-Actor", "mission-commander", "-WhatIf", "-Format", "json"}, &out); err == nil || !strings.Contains(err.Error(), "run reviewer packet adoption WhatIf then Apply") {
		t.Fatalf("ready reviewer results before adoption error = %v\n%s", err, out.String())
	}
	if got := readOptionalCaseFile(t, caseRoot, ".rekit/facts/verifications.jsonl"); got != "" {
		t.Fatalf("blocked pre-adoption batch intake wrote verification facts:\n%s", got)
	}
	adoptionArgs := []string{"-Command", "plan-subagents", "-Target", caseRoot, "-Pack", "_template", "-PacketPath", plan.PacketPath, "-AdoptReviewerPacket", "-Lane", "feature-review", "-Actor", "mission-commander", "-Reason", "adopt existing reviewer work", "-WhatIf", "-Format", "json"}
	out.Reset()
	if err := Run(adoptionArgs, &out); err != nil {
		t.Fatal(err)
	}
	var preview struct {
		Applied      bool   `json:"applied"`
		IsMutation   bool   `json:"isMutation"`
		AdoptionPath string `json:"adoptionPath"`
	}
	if err := json.Unmarshal(out.Bytes(), &preview); err != nil {
		t.Fatal(err)
	}
	if preview.Applied || preview.IsMutation {
		t.Fatalf("adoption WhatIf mutated: %+v", preview)
	}
	if _, err := os.Stat(preview.AdoptionPath); !os.IsNotExist(err) {
		t.Fatalf("adoption WhatIf wrote receipt: %v", err)
	}
	adoptionArgs[15] = "-Apply"
	out.Reset()
	if err := Run(adoptionArgs, &out); err != nil {
		t.Fatal(err)
	}
	var applied struct {
		Applied      bool   `json:"applied"`
		AdoptionPath string `json:"adoptionPath"`
		AdoptedOwner struct {
			CurrentExecutor    string `json:"currentExecutor"`
			ExecutorGeneration int    `json:"executorGeneration"`
		} `json:"adoptedOwner"`
	}
	if err := json.Unmarshal(out.Bytes(), &applied); err != nil {
		t.Fatal(err)
	}
	if !applied.Applied || applied.AdoptedOwner.CurrentExecutor != "session-b" || applied.AdoptedOwner.ExecutorGeneration != 2 {
		t.Fatalf("unexpected adoption apply: %+v", applied)
	}
	packetAfter, err := os.ReadFile(plan.PacketPath)
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(packetBefore, packetAfter) {
		t.Fatal("CLI adoption modified immutable reviewer packet")
	}
	recordReviewerSessionReceiptsForCLIPlan(t, &out, []string{"-Target", caseRoot, "-Pack", "_template"}, plan.PacketPath, plan.ShardHandoffs[0], "feature-review", "mission-commander", "replacement-cli-harness", reviewerResult)

	out.Reset()
	if err := Run([]string{"-Command", "status", "-Target", caseRoot, "-Pack", "_template", "-Format", "json"}, &out); err != nil {
		t.Fatal(err)
	}
	var adoptedStatus struct {
		CaseMission struct {
			ReviewerDispatchIntakeHandoffs []reviewerDispatchIntakeCLIItem      `json:"reviewerDispatchIntakeHandoffs"`
			ReviewerDispatchIntakeSummary  reviewerDispatchIntakeSummaryCLIItem `json:"reviewerDispatchIntakeSummary"`
			MissionCommanderActionQueue    missionCommanderActionQueueSnapshot  `json:"missionCommanderActionQueue"`
		} `json:"caseMission"`
	}
	if err := json.Unmarshal(out.Bytes(), &adoptedStatus); err != nil {
		t.Fatalf("status after reviewer packet adoption stdout is not JSON: %v\n%s", err, out.String())
	}
	adoptedDispatch, ok := reviewerDispatchIntakeByShard(adoptedStatus.CaseMission.ReviewerDispatchIntakeHandoffs, "shard-01")
	statusCurrent := adoptedStatus.CaseMission.MissionCommanderActionQueue.CurrentAction
	if !ok || adoptedDispatch.State != "ready-for-reviewer-intake-preview" || adoptedDispatch.OwnerAdoptionRequired || !adoptedDispatch.OwnerAdoptionCurrent || adoptedDispatch.OwnerAdoptionPath != applied.AdoptionPath || adoptedDispatch.OwnerAdoptionActor != "mission-commander" || adoptedDispatch.OwnerAdoptionReason != "adopt existing reviewer work" || adoptedDispatch.CurrentExecutor != "session-b" || adoptedDispatch.CurrentGeneration != 2 || !containsSubstring(adoptedDispatch.Evidence, "ownerAdoption current") || !containsSubstring(adoptedDispatch.RunbookSteps, "owner adoption receipt is current") || !containsSubstring(adoptedDispatch.Boundary, "current reviewer packet adoption receipt") || adoptedStatus.CaseMission.ReviewerDispatchIntakeSummary.NextActionState != "ready-for-reviewer-intake-preview" || statusCurrent == nil || statusCurrent.State != "ready-for-reviewer-intake-preview" || statusCurrent.Blocked || !strings.Contains(statusCurrent.Command, "-ReadyReviewerResults") {
		t.Fatalf("status after adoption should expose adopted ready-intake continuation: dispatch=%+v summary=%+v current=%+v", adoptedDispatch, adoptedStatus.CaseMission.ReviewerDispatchIntakeSummary, statusCurrent)
	}

	out.Reset()
	if err := Run([]string{"-Command", "handoff", "-Target", caseRoot, "-Pack", "_template", "-WhatIf", "review", "-Format", "json"}, &out); err != nil {
		t.Fatal(err)
	}
	handoffPreview := decodeHandoffResult(t, out.Bytes())
	handoffDispatch, ok := reviewerDispatchIntakeByShard(handoffPreview.ReviewerDispatchIntakeHandoffs, "shard-01")
	handoffCurrent := handoffPreview.MissionCommanderActionQueue.CurrentAction
	if !ok || !handoffDispatch.OwnerAdoptionCurrent || handoffDispatch.OwnerAdoptionRequired || handoffDispatch.OwnerAdoptionPath != applied.AdoptionPath || handoffDispatch.State != "ready-for-reviewer-intake-preview" || handoffCurrent == nil || handoffCurrent.State != "ready-for-reviewer-intake-preview" || handoffCurrent.Blocked || !strings.Contains(handoffCurrent.Command, "-ReadyReviewerResults") {
		t.Fatalf("handoff after adoption should preserve adopted ready-intake continuation: dispatch=%+v current=%+v", handoffDispatch, handoffCurrent)
	}

	beforeBlockedContinue := snapshotFiles(t, filepath.Join(caseRoot, ".rekit"))
	out.Reset()
	if err := Run([]string{"-Command", "continue", "-Target", caseRoot, "-Pack", "_template", "review", "-Executor", "session-b", "-ExpectedExecutorGeneration", "2", "-Apply", "-Format", "json"}, &out); err != nil {
		t.Fatal(err)
	}
	var blockedContinue struct {
		Applied                        bool                                 `json:"applied"`
		Blocked                        bool                                 `json:"blocked"`
		RunID                          string                               `json:"runId"`
		Writes                         []startWrite                         `json:"writes"`
		ReviewerDispatchIntakeHandoffs []reviewerDispatchIntakeCLIItem      `json:"reviewerDispatchIntakeHandoffs"`
		ReviewerDispatchIntakeSummary  reviewerDispatchIntakeSummaryCLIItem `json:"reviewerDispatchIntakeSummary"`
		MissionCommanderActionQueue    missionCommanderActionQueueSnapshot  `json:"missionCommanderActionQueue"`
	}
	if err := json.Unmarshal(out.Bytes(), &blockedContinue); err != nil {
		t.Fatalf("blocked continue after reviewer packet adoption stdout is not JSON: %v\n%s", err, out.String())
	}
	continueDispatch, ok := reviewerDispatchIntakeByShard(blockedContinue.ReviewerDispatchIntakeHandoffs, "shard-01")
	continueCurrent := blockedContinue.MissionCommanderActionQueue.CurrentAction
	if blockedContinue.Applied || !blockedContinue.Blocked || blockedContinue.RunID != "run-preview" || len(blockedContinue.Writes) != 0 || !ok || !continueDispatch.OwnerAdoptionCurrent || continueDispatch.OwnerAdoptionRequired || continueDispatch.State != "ready-for-reviewer-intake-preview" || blockedContinue.ReviewerDispatchIntakeSummary.NextActionState != "ready-for-reviewer-intake-preview" || continueCurrent == nil || continueCurrent.State != "ready-for-reviewer-intake-preview" || continueCurrent.Blocked || !strings.Contains(continueCurrent.Command, "-ReadyReviewerResults") {
		t.Fatalf("blocked continue after adoption should be zero-write ready-intake handoff: continue=%+v dispatch=%+v current=%+v", blockedContinue, continueDispatch, continueCurrent)
	}
	assertSnapshotEqual(t, beforeBlockedContinue, snapshotFiles(t, filepath.Join(caseRoot, ".rekit")))

	out.Reset()
	if err := Run([]string{"-Command", "plan-subagents", "-Target", caseRoot, "-Pack", "_template", "-PacketPath", plan.PacketPath, "-ReadyReviewerResults", "-Lane", "feature-review", "-Actor", "mission-commander", "-WhatIf", "-Format", "json"}, &out); err != nil {
		t.Fatal(err)
	}
	ready := decodeReviewerBatchIntakeResult(t, out.Bytes())
	if ready.Total != 1 || ready.Ready != 1 || ready.Waiting != 0 || ready.Processed != 1 || ready.Stopped || len(ready.Results) != 1 || ready.Results[0].WritebackStatus != "previewed" || !ready.Results[0].ReadyForWriteback || ready.Results[0].Verification == nil || ready.Results[0].Decision == nil || mission.Value(ready.Results[0].Verification.Event, "ownerExecutor") != "session-b" || mission.Value(ready.Results[0].Decision.Event, "ownerExecutor") != "session-b" {
		t.Fatalf("ready reviewer results did not use adopted owner: %+v", ready)
	}
	if got := readOptionalCaseFile(t, caseRoot, ".rekit/facts/verifications.jsonl"); got != "" {
		t.Fatalf("post-adoption batch intake WhatIf wrote verification facts:\n%s", got)
	}

	out.Reset()
	if err := Run([]string{"-Command", "start", "-Target", caseRoot, "-Pack", "_template", "-Name", "review", "-Executor", "session-c", "-Actor", "mission-commander", "-Reason", "second replacement reviewer owner", "-Apply", "-Format", "json"}, &out); err != nil {
		t.Fatal(err)
	}
	out.Reset()
	if err := Run([]string{"-Command", "status", "-Target", caseRoot, "-Pack", "_template", "-Format", "json"}, &out); err != nil {
		t.Fatal(err)
	}
	if err := json.Unmarshal(out.Bytes(), &status); err != nil {
		t.Fatal(err)
	}
	if status.CaseMission.MissionCommanderActionQueue.CurrentAction == nil || status.CaseMission.MissionCommanderActionQueue.CurrentAction.State != "reviewer-packet-owner-adoption-required" {
		t.Fatalf("status did not promote stale reviewer packet adoption after second takeover: %+v", status.CaseMission.MissionCommanderActionQueue)
	}
	out.Reset()
	if err := Run([]string{"-Command", "plan-subagents", "-Target", caseRoot, "-Pack", "_template", "-PacketPath", plan.PacketPath, "-ReadyReviewerResults", "-Lane", "feature-review", "-Actor", "mission-commander", "-WhatIf", "-Format", "json"}, &out); err == nil || !strings.Contains(err.Error(), "adoption is stale") {
		t.Fatalf("ready reviewer results with stale adoption error = %v\n%s", err, out.String())
	}
	if got := readOptionalCaseFile(t, caseRoot, ".rekit/facts/verifications.jsonl"); got != "" {
		t.Fatalf("blocked stale-adoption batch intake wrote verification facts:\n%s", got)
	}

	readoptionArgs := []string{"-Command", "plan-subagents", "-Target", caseRoot, "-Pack", "_template", "-PacketPath", plan.PacketPath, "-AdoptReviewerPacket", "-Lane", "feature-review", "-Actor", "mission-commander", "-Reason", "re-adopt reviewer work after second takeover", "-WhatIf", "-Format", "json"}
	out.Reset()
	if err := Run(readoptionArgs, &out); err != nil {
		t.Fatal(err)
	}
	var secondPreview struct {
		Applied      bool   `json:"applied"`
		IsMutation   bool   `json:"isMutation"`
		AdoptionPath string `json:"adoptionPath"`
	}
	if err := json.Unmarshal(out.Bytes(), &secondPreview); err != nil {
		t.Fatal(err)
	}
	if secondPreview.Applied || secondPreview.IsMutation || secondPreview.AdoptionPath != applied.AdoptionPath {
		t.Fatalf("second adoption WhatIf mutated or changed receipt path: %+v", secondPreview)
	}
	readoptionArgs[15] = "-Apply"
	out.Reset()
	if err := Run(readoptionArgs, &out); err != nil {
		t.Fatal(err)
	}
	var secondApplied struct {
		Applied      bool   `json:"applied"`
		AdoptionPath string `json:"adoptionPath"`
		AdoptedOwner struct {
			CurrentExecutor    string `json:"currentExecutor"`
			ExecutorGeneration int    `json:"executorGeneration"`
		} `json:"adoptedOwner"`
	}
	if err := json.Unmarshal(out.Bytes(), &secondApplied); err != nil {
		t.Fatal(err)
	}
	if !secondApplied.Applied || secondApplied.AdoptionPath != applied.AdoptionPath || secondApplied.AdoptedOwner.CurrentExecutor != "session-c" || secondApplied.AdoptedOwner.ExecutorGeneration != 3 {
		t.Fatalf("unexpected second adoption apply: %+v", secondApplied)
	}
	firstAdoptionHistory := filepath.Join(filepath.Dir(secondApplied.AdoptionPath), "history", packet.PacketID+"-generation-2.json")
	if _, err := os.Stat(firstAdoptionHistory); err != nil {
		t.Fatalf("first adoption receipt was not archived during re-adoption: %v", err)
	}
	packetAfterSecondAdoption, err := os.ReadFile(plan.PacketPath)
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(packetBefore, packetAfterSecondAdoption) {
		t.Fatal("CLI re-adoption modified immutable reviewer packet")
	}
	recordReviewerSessionReceiptsForCLIPlan(t, &out, []string{"-Target", caseRoot, "-Pack", "_template"}, plan.PacketPath, plan.ShardHandoffs[0], "feature-review", "mission-commander", "replacement-cli-harness-session-c", reviewerResult)

	out.Reset()
	if err := Run([]string{"-Command", "plan-subagents", "-Target", caseRoot, "-Pack", "_template", "-PacketPath", plan.PacketPath, "-ReadyReviewerResults", "-Lane", "feature-review", "-Actor", "mission-commander", "-WhatIf", "-Format", "json"}, &out); err != nil {
		t.Fatal(err)
	}
	readoptedReady := decodeReviewerBatchIntakeResult(t, out.Bytes())
	if readoptedReady.Total != 1 || readoptedReady.Ready != 1 || readoptedReady.Waiting != 0 || readoptedReady.Processed != 1 || readoptedReady.Stopped || len(readoptedReady.Results) != 1 || readoptedReady.Results[0].WritebackStatus != "previewed" || !readoptedReady.Results[0].ReadyForWriteback || readoptedReady.Results[0].Verification == nil || readoptedReady.Results[0].Decision == nil || mission.Value(readoptedReady.Results[0].Verification.Event, "ownerExecutor") != "session-c" || mission.Value(readoptedReady.Results[0].Decision.Event, "ownerExecutor") != "session-c" {
		t.Fatalf("ready reviewer results did not use re-adopted owner: %+v", readoptedReady)
	}
	if got := readOptionalCaseFile(t, caseRoot, ".rekit/facts/verifications.jsonl"); got != "" {
		t.Fatalf("post-readoption batch intake WhatIf wrote verification facts:\n%s", got)
	}

	out.Reset()
	if err := Run([]string{"-Command", "plan-subagents", "-Target", caseRoot, "-Pack", "_template", "-PacketPath", plan.PacketPath, "-ReadyReviewerResults", "-Lane", "feature-review", "-Actor", "mission-commander", "-Apply", "-Format", "json"}, &out); err != nil {
		t.Fatal(err)
	}
	readoptedApplied := decodeReviewerBatchIntakeResult(t, out.Bytes())
	if !readoptedApplied.IsMutation || !readoptedApplied.Applied || readoptedApplied.Total != 1 || readoptedApplied.Ready != 1 || readoptedApplied.Waiting != 0 || readoptedApplied.Processed != 1 || readoptedApplied.Completed != 1 || readoptedApplied.AlreadyComplete != 0 || readoptedApplied.Stopped || len(readoptedApplied.Results) != 1 {
		t.Fatalf("unexpected re-adopted ready reviewer results apply: %+v", readoptedApplied)
	}
	appliedResult := readoptedApplied.Results[0]
	if appliedResult.WritebackStatus != "complete" || !appliedResult.Applied || !appliedResult.ReadyForWriteback || appliedResult.Verification == nil || appliedResult.Decision == nil || !appliedResult.Verification.Applied || !appliedResult.Decision.Applied || appliedResult.PostValidation == nil || !appliedResult.PostValidation.Valid || appliedResult.PostValidation.Overview.Sections.Verifications.Total != 1 || appliedResult.PostValidation.Overview.Sections.Decisions.Total != 1 || appliedResult.PostValidation.Summary.ReviewerWritebackSummary == nil || appliedResult.PostValidation.Summary.ReviewerWritebackSummary.Total != 2 || mission.Value(appliedResult.Verification.Event, "ownerExecutor") != "session-c" || mission.Value(appliedResult.Decision.Event, "ownerExecutor") != "session-c" {
		t.Fatalf("re-adopted batch apply did not complete writeback with latest owner: %+v", appliedResult)
	}
	verificationFacts, err := mission.ReadStrictFact(caseRoot, "verification")
	if err != nil {
		t.Fatal(err)
	}
	decisionFacts, err := mission.ReadStrictFact(caseRoot, "decision")
	if err != nil {
		t.Fatal(err)
	}
	if len(verificationFacts) != 1 || len(decisionFacts) != 1 || mission.Value(verificationFacts[0], "ownerExecutor") != "session-c" || mission.Value(decisionFacts[0], "ownerExecutor") != "session-c" {
		t.Fatalf("re-adopted batch apply wrote unexpected owner facts: verifications=%+v decisions=%+v", verificationFacts, decisionFacts)
	}

	out.Reset()
	if err := Run([]string{"-Command", "plan-subagents", "-Target", caseRoot, "-Pack", "_template", "-PacketPath", plan.PacketPath, "-ReadyReviewerResults", "-Lane", "feature-review", "-Actor", "mission-commander", "-WhatIf", "-Format", "json"}, &out); err != nil {
		t.Fatal(err)
	}
	alreadyComplete := decodeReviewerBatchIntakeResult(t, out.Bytes())
	if alreadyComplete.IsMutation || alreadyComplete.Applied || alreadyComplete.Total != 1 || alreadyComplete.Ready != 1 || alreadyComplete.Waiting != 0 || alreadyComplete.Processed != 1 || alreadyComplete.Completed != 0 || alreadyComplete.AlreadyComplete != 1 || alreadyComplete.Stopped || len(alreadyComplete.Results) != 1 || alreadyComplete.Results[0].WritebackStatus != "already-complete" {
		t.Fatalf("re-adopted batch did not become already-complete without new writes: %+v", alreadyComplete)
	}
	verificationFactsAfter, err := mission.ReadStrictFact(caseRoot, "verification")
	if err != nil {
		t.Fatal(err)
	}
	decisionFactsAfter, err := mission.ReadStrictFact(caseRoot, "decision")
	if err != nil {
		t.Fatal(err)
	}
	if len(verificationFactsAfter) != 1 || len(decisionFactsAfter) != 1 {
		t.Fatalf("already-complete preview duplicated reviewer facts: verifications=%+v decisions=%+v", verificationFactsAfter, decisionFactsAfter)
	}
}

func TestRunPlanSubagentsReviewerIntakeCaseLocalProductPathUsesMetadataRuntime(t *testing.T) {
	root := repoRoot(t)
	caseRoot := fullAttachedCase(t)
	var out bytes.Buffer
	if err := Run([]string{"-Command", "start", "-Target", caseRoot, "-Pack", "_template", "-Name", "review", "-Apply"}, &out); err != nil {
		t.Fatal(err)
	}
	start := decodeStartResult(t, out.Bytes())
	if start.Lane.ID != "feature-review" || start.Lane.Workspace != "workspace/features/feature-review" {
		t.Fatalf("unexpected reviewer product-path lane: %+v", start.Lane)
	}
	evidenceRel := start.Lane.Workspace + "/reviewer-evidence.md"
	writeCaseFile(t, caseRoot, evidenceRel, "bounded nested reviewer evidence\n")

	oldwd, err := os.Getwd()
	if err != nil {
		t.Fatal(err)
	}
	nested := filepath.Join(caseRoot, filepath.FromSlash(start.Lane.Workspace))
	if err := os.Chdir(nested); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		if err := os.Chdir(oldwd); err != nil {
			t.Fatal(err)
		}
	})

	out.Reset()
	if err := Run([]string{"-Command", "plan-subagents", "-TaskType", "feature-analysis", "-Items", "alpha"}, &out); err != nil {
		t.Fatal(err)
	}
	plan := decodePlanSubagentsResult(t, out.Bytes())
	if plan.Command != "plan-subagents" || plan.PlanRoot != caseRoot || plan.Pack != "_template" || plan.TargetLane != "main" || plan.ShardCount != 1 || len(plan.ShardHandoffs) != 1 {
		t.Fatalf("unexpected nested case-local plan-subagents result: %+v", plan)
	}
	packet := decodePlanSubagentsPacket(t, plan.PacketPath)
	if packet.TargetLane != "main" || packet.OwnerBinding.TargetLane != "main" || packet.Route.ID == "" {
		t.Fatalf("unexpected nested case-local reviewer packet: %+v", packet)
	}
	resultPath := plan.ShardHandoffs[0].ReviewerResultPath
	if resultPath == "" || !strings.HasPrefix(resultPath, filepath.Join(caseRoot, ".rekit")) {
		t.Fatalf("unexpected nested reviewer result path: %q", resultPath)
	}
	reviewerResult := map[string]any{
		"packetId":           packet.PacketID,
		"routeId":            packet.Route.ID,
		"shardId":            "shard-01",
		"items":              []string{"alpha"},
		"reviewerSession":    "reviewer-session-product-path",
		"decision":           "accept",
		"confidence":         "high",
		"summary":            "reviewed alpha from nested case-local workspace",
		"evidenceRefs":       []string{evidenceRel},
		"risks":              []string{"bounded residual risk"},
		"conflicts":          []string{},
		"recommendedVerdict": "accepted",
		"routeOutput": map[string]any{
			"item": "alpha", "decision": "accept", "confidence": "high", "evidence": evidenceRel, "risk": "low", "next_action": "main-agent-writeback", "tier_used": "light", "tool_scope": "read-only", "feature": "review", "request_id": "n/a", "candidate_path": "n/a", "defer_reason": "n/a",
		},
	}
	data, err := json.Marshal(reviewerResult)
	if err != nil {
		t.Fatal(err)
	}
	stageAndCollectReviewerResultForCLIPlan(t, &out, nil, plan.PacketPath, plan.ShardHandoffs[0], packet.TargetLane, "mission-commander", data)

	out.Reset()
	if err := Run([]string{"-Command", "plan-subagents", "-PacketPath", plan.PacketPath, "-ReviewerResultPath", resultPath, "-Lane", packet.TargetLane, "-Actor", "mission-commander", "-WhatIf", "-Format", "json"}, &out); err != nil {
		t.Fatal(err)
	}
	preview := decodeReviewerIntakeResult(t, out.Bytes())
	if preview.Command != "plan-subagents" || preview.Mode != "reviewer-intake" || preview.CaseRoot != caseRoot || preview.Pack != "_template" || preview.Lane != "main" || preview.IsMutation || preview.Applied || preview.WritebackStatus != "previewed" || !preview.ReadyForWriteback || preview.Verification == nil || preview.Decision == nil || preview.PostValidation == nil || !preview.PostValidation.Valid {
		t.Fatalf("unexpected nested reviewer intake preview: %+v", preview)
	}
	if preview.MissionCommanderAction.State != "ready-for-reviewer-intake-apply" || !strings.Contains(preview.MissionCommanderAction.PrimaryCommand, `-Target "`+caseRoot+`"`) || !strings.Contains(preview.MissionCommanderAction.PrimaryCommand, `-Pack "_template"`) || !containsMissionCommanderNextAction(preview.MissionCommanderNextActions, "reviewerIntake.previewed", preview.MissionCommanderAction.PrimaryCommand, false, true) {
		t.Fatalf("nested preview omitted recoverable Mission Commander guidance: action=%+v next=%+v", preview.MissionCommanderAction, preview.MissionCommanderNextActions)
	}
	if preview.Verification.Event["reviewerDecision"] != "accept" || preview.Verification.Event["recommendedVerdict"] != "accepted" || !containsSubstring(eventStringList(preview.Verification.Event["reviewerRisks"]), "bounded residual risk") || preview.Verification.Event["routeOutput"] == nil || preview.Decision.Event["reviewerDecision"] != "accept" {
		t.Fatalf("nested preview omitted reviewer result provenance in note events: verification=%+v decision=%+v", preview.Verification.Event, preview.Decision.Event)
	}
	assertCLIActionQueue(t, preview.MissionCommanderActionQueue, 2, 2, 0, 2, 1, preview.MissionCommanderAction.PrimaryCommand)

	out.Reset()
	if err := Run([]string{"-Command", "plan-subagents", "-PacketPath", plan.PacketPath, "-ReviewerResultPath", resultPath, "-Lane", packet.TargetLane, "-Actor", "mission-commander", "-WhatIf", "-Format", "text"}, &out); err != nil {
		t.Fatal(err)
	}
	for _, expected := range []string{
		"plan-subagents reviewer intake：status=previewed mutation=false applied=false readyForWriteback=true lane=main shard=shard-01",
		"reviewer intake result summary：reviewed alpha from nested case-local workspace",
		"reviewer intake route output：tool_scope=read-only",
		"reviewer intake post-validation：valid=true overviewVerifications=0 overviewDecisions=0 doctorRows=",
		"reviewer intake commander action：state=ready-for-reviewer-intake-apply",
		"mission commander action queue current：state=ready-for-reviewer-intake-apply source=reviewerIntake.previewed blocked=false requiresReview=true command=`/rekit plan-subagents",
	} {
		if !strings.Contains(out.String(), expected) {
			t.Fatalf("nested reviewer intake preview text missing %q:\n%s", expected, out.String())
		}
	}
	if strings.Contains(out.String(), "{\n  ") {
		t.Fatalf("nested reviewer intake preview text should not emit JSON object:\n%s", out.String())
	}

	out.Reset()
	if err := Run([]string{"-Command", "plan-subagents", "-PacketPath", plan.PacketPath, "-ReviewerResultPath", resultPath, "-Lane", packet.TargetLane, "-Actor", "mission-commander", "-Apply", "-Format", "json"}, &out); err != nil {
		t.Fatal(err)
	}
	applied := decodeReviewerIntakeResult(t, out.Bytes())
	if !applied.IsMutation || !applied.Applied || applied.WritebackStatus != "complete" || applied.Verification == nil || applied.Decision == nil || !applied.Verification.Applied || !applied.Decision.Applied || applied.PostValidation == nil || !applied.PostValidation.Valid || applied.PostValidation.Handoff.Lane == nil || applied.PostValidation.Handoff.Lane.ID != "main" || applied.PostValidation.Overview.Sections.Verifications.Total != 1 || applied.PostValidation.Overview.Sections.Decisions.Total != 1 {
		t.Fatalf("unexpected nested reviewer intake apply: %+v", applied)
	}
	if applied.MissionCommanderAction.State != "reviewer-intake-writeback-complete" || applied.MissionCommanderActionQueue.CurrentAction == nil || !strings.HasPrefix(applied.MissionCommanderActionQueue.CurrentAction.Source, "reviewerIntake.postValidation.") {
		t.Fatalf("nested apply omitted post-validation Mission Commander guidance: action=%+v queue=%+v", applied.MissionCommanderAction, applied.MissionCommanderActionQueue)
	}
	if len(applied.PostValidation.Handoff.ReviewerWritebacks) != 2 || applied.PostValidation.Handoff.ReviewerWritebacks[0].ReviewerSession != "reviewer-session-product-path" || applied.PostValidation.Handoff.ReviewerWritebacks[0].ShardID != "shard-01" || applied.PostValidation.Handoff.ReviewerWritebacks[0].ReviewerDecision != "accept" || applied.PostValidation.Handoff.ReviewerWritebacks[0].RecommendedVerdict != "accepted" || !containsSubstring(applied.PostValidation.Handoff.ReviewerWritebacks[0].ReviewerRisks, "bounded residual risk") || applied.PostValidation.Handoff.ReviewerWritebacks[0].RouteOutput["tool_scope"] != "read-only" || applied.PostValidation.Handoff.ReviewerWritebacks[1].Kind != "decision" || applied.PostValidation.Handoff.ReviewerWritebacks[1].ReviewerDecision != "accept" || !containsSubstring(applied.PostValidation.Handoff.ReviewerWritebacks[1].EvidenceRefs, applied.Verification.EventID) {
		t.Fatalf("nested apply omitted reviewer writeback handoff identity: %+v", applied.PostValidation.Handoff.ReviewerWritebacks)
	}

	out.Reset()
	if err := Run([]string{"-Command", "plan-subagents", "-PacketPath", plan.PacketPath, "-ReviewerResultPath", resultPath, "-Lane", packet.TargetLane, "-Actor", "mission-commander", "-WhatIf", "-Format", "text"}, &out); err != nil {
		t.Fatal(err)
	}
	for _, expected := range []string{
		"plan-subagents reviewer intake：status=already-complete mutation=false applied=false readyForWriteback=true lane=main shard=shard-01",
		"reviewer intake post-validation reviewer writeback：kind=verification eventId=evt-",
		"reviewer intake post-validation reviewer result：eventId=evt-",
		"reviewer intake post-validation reviewer owner：eventId=evt-",
		"reviewer intake post-validation reviewer decision detail：eventId=evt-",
		"reviewer intake post-validation reviewer risk：eventId=evt-",
		"reviewer intake post-validation reviewer route output：eventId=evt-",
		"reviewer intake post-validation reviewer evidence ref：eventId=evt-",
	} {
		if !strings.Contains(out.String(), expected) {
			t.Fatalf("nested already-complete reviewer intake post-validation text missing %q:\n%s", expected, out.String())
		}
	}

	out.Reset()
	if err := Run([]string{"-Command", "status", "-Format", "json"}, &out); err != nil {
		t.Fatal(err)
	}
	var status statusInventory
	if err := json.Unmarshal(out.Bytes(), &status); err != nil {
		t.Fatalf("nested status stdout is not JSON: %v\n%s", err, out.String())
	}
	if status.Command != "status" || status.Mode != "case" || status.Pack != "_template" || status.PackSource != "case-metadata" || status.TargetProvided || status.Target != caseRoot || status.TemplateRoot != root || status.Case == nil || status.Case.TemplatePack != "_template" || status.CaseMission == nil || status.CaseMission.Sections == nil || status.CaseMission.Sections.Verifications.Total != 1 || status.CaseMission.Sections.Decisions.Total != 1 || status.CaseMission.MissionCommanderActionQueue.CurrentAction == nil || len(status.CaseMission.ReviewerWritebacks) != 2 {
		t.Fatalf("nested status omitted reviewer intake post-validation handoff state: %+v", status)
	}
	if status.CaseMission.ReviewerWritebacks[0].PacketID != packet.PacketID || status.CaseMission.ReviewerWritebacks[0].RouteID != packet.Route.ID || status.CaseMission.ReviewerWritebacks[0].ReviewerResultPath != resultPath || status.CaseMission.ReviewerWritebacks[0].OwnerBindingTarget != "main" || status.CaseMission.ReviewerWritebacks[1].Kind != "decision" || !containsSubstring(status.CaseMission.ReviewerWritebacks[1].EvidenceRefs, applied.Verification.EventID) {
		t.Fatalf("nested status reviewer writeback identity missing: %+v", status.CaseMission.ReviewerWritebacks)
	}
	if status.CaseMission.ReviewerWritebackSummary.Total != 2 || status.CaseMission.ReviewerWritebackSummary.VerificationCount != 1 || status.CaseMission.ReviewerWritebackSummary.DecisionCount != 1 || status.CaseMission.ReviewerWritebackSummary.LaneCount != 1 || status.CaseMission.ReviewerWritebackSummary.LatestKind != "decision" || status.CaseMission.ReviewerWritebackSummary.LatestReviewerSession != "reviewer-session-product-path" || status.CaseMission.ReviewerWritebackSummary.LatestShardID != "shard-01" || status.CaseMission.ReviewerWritebackSummary.LatestPacketID != packet.PacketID || !status.CaseMission.ReviewerWritebackSummary.HasOwnerBinding || !status.CaseMission.ReviewerWritebackSummary.HasRisks || !status.CaseMission.ReviewerWritebackSummary.HasRouteOutput || !containsStringWith(status.CaseMission.ReviewerWritebackSummary.Boundary, "reviewer writeback summary is read-only") {
		t.Fatalf("nested status reviewer writeback summary missing: %+v", status.CaseMission.ReviewerWritebackSummary)
	}

	out.Reset()
	if err := Run([]string{}, &out); err != nil {
		t.Fatal(err)
	}
	for _, expected := range []string{
		"rekit go backend:",
		"template root: " + root,
		"pack: _template",
		"pack source: case-metadata",
		"case: " + caseRoot,
		"status case mission section：name=verifications total=1 shown=1",
		"status case mission section event：section=verifications index=1 eventId=evt-",
		"status case mission section reviewer detail：eventId=evt-",
		"status case mission section reviewer decision detail：eventId=evt-",
		"status case mission section reviewer risk：eventId=evt-",
		"status case mission section reviewer route output：eventId=evt-",
		"status case mission reviewer writeback summary：total=2 verifications=1 decisions=1 lanes=1 latestKind=decision",
		"status case mission reviewer writeback summary boundary：reviewer writeback summary is read-only; full reviewerWritebacks remain available",
		"status case mission reviewer writeback：kind=verification eventId=evt-",
		"status case mission reviewer result：eventId=evt-",
		"status case mission reviewer owner：eventId=evt-",
		"status case mission reviewer decision detail：eventId=evt-",
		"status case mission reviewer risk：eventId=evt-",
		"status case mission reviewer route output：eventId=evt-",
		"status case mission reviewer evidence ref：eventId=evt-",
		"status case mission section：name=decisions total=1 shown=1",
		"status case mission section event：section=decisions index=1 eventId=evt-",
		"status case mission reviewer writeback：kind=decision eventId=evt-",
		"status case mission queue action：bucket=current",
		"continueBoundary=status is read-only; run continue with -WhatIf first",
	} {
		if !strings.Contains(out.String(), expected) {
			t.Fatalf("nested default status after reviewer intake missing %q:\n%s", expected, out.String())
		}
	}
	if strings.Contains(out.String(), "{\n  ") {
		t.Fatalf("nested default status should not emit JSON object:\n%s", out.String())
	}

	out.Reset()
	if err := Run([]string{"-Command", "overview", "-Format", "text"}, &out); err != nil {
		t.Fatal(err)
	}
	for _, expected := range []string{
		"overview section reviewer detail：eventId=evt-",
		"overview section reviewer owner：eventId=evt-",
		"overview section reviewer decision detail：eventId=evt-",
		"overview section reviewer risk：eventId=evt-",
		"overview section reviewer route output：eventId=evt-",
		"overview section reviewer evidence ref：eventId=evt-",
	} {
		if !strings.Contains(out.String(), expected) {
			t.Fatalf("nested overview reviewer writeback text missing %q:\n%s", expected, out.String())
		}
	}
	if strings.Contains(out.String(), "{\n  ") {
		t.Fatalf("nested overview text should not emit JSON object:\n%s", out.String())
	}

	out.Reset()
	if err := Run([]string{"-Command", "handoff", "main", "-WhatIf", "-Format", "text"}, &out); err != nil {
		t.Fatal(err)
	}
	for _, expected := range []string{
		"handoff reviewer writeback summary：total=2 verifications=1 decisions=1 lanes=1 latestKind=decision",
		"handoff reviewer writeback summary boundary：reviewer writeback summary is read-only; full reviewerWritebacks remain available",
		"handoff reviewer writeback：kind=verification eventId=evt-",
		"handoff reviewer result：eventId=evt-",
		"handoff reviewer owner：eventId=evt-",
		"handoff reviewer decision detail：eventId=evt-",
		"handoff reviewer risk：eventId=evt-",
		"handoff reviewer route output：eventId=evt-",
		"handoff reviewer evidence ref：eventId=evt-",
	} {
		if !strings.Contains(out.String(), expected) {
			t.Fatalf("nested handoff reviewer writeback text missing %q:\n%s", expected, out.String())
		}
	}
	if strings.Contains(out.String(), "{\n  ") {
		t.Fatalf("nested handoff text should not emit JSON object:\n%s", out.String())
	}

	out.Reset()
	if err := Run([]string{"-Command", "handoff", "main", "-Apply", "-Format", "json"}, &out); err != nil {
		t.Fatal(err)
	}
	handoffApply := decodeHandoffResult(t, out.Bytes())
	if len(handoffApply.ReviewerWritebacks) != 2 || handoffApply.ReviewerWritebacks[0].ReviewerSession != "reviewer-session-product-path" || handoffApply.ReviewerWritebacks[1].Kind != "decision" {
		t.Fatalf("nested handoff apply JSON omitted reviewer writebacks: %+v", handoffApply.ReviewerWritebacks)
	}
	if handoffApply.ReviewerWritebackSummary.Total != 2 || handoffApply.ReviewerWritebackSummary.LatestKind != "decision" || handoffApply.ReviewerWritebackSummary.LatestReviewerSession != "reviewer-session-product-path" || !handoffApply.ReviewerWritebackSummary.HasRouteOutput {
		t.Fatalf("nested handoff apply JSON omitted reviewer writeback summary: %+v", handoffApply.ReviewerWritebackSummary)
	}
	latestPath := assertStartWrite(t, handoffApply.Writes, ".rekit/handovers/main-latest.md", "write-latest-lane-handoff").TargetPath
	latestText, err := os.ReadFile(latestPath)
	if err != nil {
		t.Fatal(err)
	}
	for _, expected := range []string{"## reviewer writeback", "reviewer writeback summary: total=`2`", "reviewer writeback summary boundary: reviewer writeback summary is read-only", "reviewerSession=reviewer-session-product-path", "reviewer result: `" + resultPath + "`", "owner binding: target=main mode=", "reviewer decision detail: reviewerDecision=accept recommendedVerdict=accepted", "reviewer risk: bounded residual risk", "reviewer route output:"} {
		if !strings.Contains(string(latestText), expected) {
			t.Fatalf("written handoff omitted reviewer writeback %q:\n%s", expected, string(latestText))
		}
	}
	resumePath := filepath.Join(caseRoot, ".rekit", "lanes", "main", "prompts", "RESUME.md")
	resumeText, err := os.ReadFile(resumePath)
	if err != nil {
		t.Fatal(err)
	}
	for _, expected := range []string{"## Reviewer writeback", "summary: total=`2`", "summary boundary: reviewer writeback summary is read-only", "reviewerSession=reviewer-session-product-path", "reviewer result: `" + resultPath + "`", "reviewer decision detail: reviewerDecision=accept recommendedVerdict=accepted", "reviewer risk: bounded residual risk", "reviewer route output:"} {
		if !strings.Contains(string(resumeText), expected) {
			t.Fatalf("lane RESUME omitted reviewer writeback %q:\n%s", expected, string(resumeText))
		}
	}
	checkpointBytes, err := os.ReadFile(filepath.Join(caseRoot, ".rekit", "lanes", "main", "checkpoints", "latest.json"))
	if err != nil {
		t.Fatal(err)
	}
	var checkpoint struct {
		ReviewerWritebacks       []reviewerWritebackCLIItem      `json:"reviewerWritebacks"`
		ReviewerWritebackSummary reviewerWritebackSummaryCLIItem `json:"reviewerWritebackSummary"`
	}
	if err := json.Unmarshal(checkpointBytes, &checkpoint); err != nil {
		t.Fatalf("checkpoint is not JSON: %v\n%s", err, string(checkpointBytes))
	}
	if len(checkpoint.ReviewerWritebacks) != 2 || checkpoint.ReviewerWritebacks[0].PacketID != packet.PacketID || checkpoint.ReviewerWritebacks[1].Kind != "decision" {
		t.Fatalf("checkpoint omitted reviewer writebacks: %+v", checkpoint.ReviewerWritebacks)
	}
	if checkpoint.ReviewerWritebackSummary.Total != 2 || checkpoint.ReviewerWritebackSummary.LatestKind != "decision" || !checkpoint.ReviewerWritebackSummary.HasOwnerBinding {
		t.Fatalf("checkpoint omitted reviewer writeback summary: %+v", checkpoint.ReviewerWritebackSummary)
	}

	out.Reset()
	if err := Run([]string{"-Command", "continue", "main", "-WhatIf", "-Format", "text"}, &out); err != nil {
		t.Fatal(err)
	}
	for _, expected := range []string{
		"continue reviewer writeback summary：total=2 verifications=1 decisions=1 lanes=1 latestKind=decision",
		"continue reviewer writeback summary boundary：reviewer writeback summary is read-only; full reviewerWritebacks remain available",
		"continue reviewer writeback：kind=verification eventId=evt-",
		"continue reviewer result：eventId=evt-",
		"continue reviewer owner：eventId=evt-",
		"continue reviewer decision detail：eventId=evt-",
		"continue reviewer risk：eventId=evt-",
		"continue reviewer route output：eventId=evt-",
		"continue reviewer evidence ref：eventId=evt-",
	} {
		if !strings.Contains(out.String(), expected) {
			t.Fatalf("nested continue reviewer writeback text missing %q:\n%s", expected, out.String())
		}
	}

	out.Reset()
	if err := Run([]string{"-Command", "continue", "main", "-Apply", "-Format", "json"}, &out); err != nil {
		t.Fatal(err)
	}
	var continueApply struct {
		RunID                    string                          `json:"runId"`
		ReviewerWritebacks       []reviewerWritebackCLIItem      `json:"reviewerWritebacks"`
		ReviewerWritebackSummary reviewerWritebackSummaryCLIItem `json:"reviewerWritebackSummary"`
	}
	if err := json.Unmarshal(out.Bytes(), &continueApply); err != nil {
		t.Fatalf("continue apply stdout is not JSON: %v\n%s", err, out.String())
	}
	if len(continueApply.ReviewerWritebacks) != 2 || continueApply.ReviewerWritebacks[0].ReviewerSession != "reviewer-session-product-path" || continueApply.ReviewerWritebacks[1].Kind != "decision" {
		t.Fatalf("continue apply omitted reviewer writebacks: %+v", continueApply.ReviewerWritebacks)
	}
	if continueApply.ReviewerWritebackSummary.Total != 2 || continueApply.ReviewerWritebackSummary.LatestKind != "decision" || continueApply.ReviewerWritebackSummary.LatestReviewerSession != "reviewer-session-product-path" || !continueApply.ReviewerWritebackSummary.HasRouteOutput {
		t.Fatalf("continue apply omitted reviewer writeback summary: %+v", continueApply.ReviewerWritebackSummary)
	}
	digest, err := os.ReadFile(filepath.Join(caseRoot, ".rekit", "runs", continueApply.RunID, "digest.md"))
	if err != nil {
		t.Fatal(err)
	}
	for _, expected := range []string{"## Reviewer writeback", "summary: total=`2`", "summary boundary: reviewer writeback summary is read-only", "reviewerSession=reviewer-session-product-path", "reviewer result: `" + resultPath + "`", "reviewer decision detail: reviewerDecision=accept recommendedVerdict=accepted", "reviewer risk: bounded residual risk", "reviewer route output:"} {
		if !strings.Contains(string(digest), expected) {
			t.Fatalf("continue digest omitted reviewer writeback %q:\n%s", expected, string(digest))
		}
	}
}

func TestRunPlanSubagentsReviewerSessionReceiptProductPath(t *testing.T) {
	caseRoot := fullAttachedCase(t)
	var out bytes.Buffer
	if err := Run([]string{"-Command", "start", "-Target", caseRoot, "-Pack", "_template", "-Name", "review", "-Apply"}, &out); err != nil {
		t.Fatal(err)
	}
	writeCaseFile(t, caseRoot, "workspace/review-evidence.md", "bounded reviewer evidence\n")
	out.Reset()
	if err := Run([]string{"-Command", "plan-subagents", "-Target", caseRoot, "-Pack", "_template", "-TaskType", "feature-analysis", "-Items", "alpha", "-Lane", "feature-review", "-Format", "json"}, &out); err != nil {
		t.Fatal(err)
	}
	plan := decodePlanSubagentsResult(t, out.Bytes())
	packet := decodePlanSubagentsPacket(t, plan.PacketPath)
	handoff := plan.ShardHandoffs[0]

	out.Reset()
	if err := Run([]string{"-Command", "plan-subagents", "-Target", caseRoot, "-Pack", "_template", "-PacketPath", plan.PacketPath, "-RecordReviewerDispatch", "-ShardId", handoff.ShardID, "-ReviewerHarness", "claude-code-agent", "-ReviewerSession", "reviewer-session-cli-receipt", "-Lane", packet.TargetLane, "-Actor", "mission-commander", "-WhatIf", "-Format", "json"}, &out); err != nil {
		t.Fatal(err)
	}
	var dispatchPreview subagents.ReviewerSessionReceiptResult
	if err := json.Unmarshal(out.Bytes(), &dispatchPreview); err != nil {
		t.Fatal(err)
	}
	if dispatchPreview.DispatchID == "" || dispatchPreview.BindingSHA256 == "" || !strings.Contains(dispatchPreview.ApplyCommand, "-ExpectedReviewerDispatchBindingSha256") {
		t.Fatalf("unexpected dispatch preview: %+v", dispatchPreview)
	}
	out.Reset()
	if err := Run([]string{"-Command", "plan-subagents", "-Target", caseRoot, "-Pack", "_template", "-PacketPath", plan.PacketPath, "-RecordReviewerDispatch", "-ShardId", handoff.ShardID, "-ReviewerHarness", "claude-code-agent", "-ReviewerSession", "reviewer-session-cli-receipt", "-Lane", packet.TargetLane, "-Actor", "mission-commander", "-ExpectedReviewerDispatchBindingSha256", dispatchPreview.BindingSHA256, "-Apply", "-Format", "json"}, &out); err != nil {
		t.Fatal(err)
	}
	var dispatch subagents.ReviewerSessionReceiptResult
	if err := json.Unmarshal(out.Bytes(), &dispatch); err != nil {
		t.Fatal(err)
	}
	if !dispatch.Applied || dispatch.ReceiptSHA256 == "" {
		t.Fatalf("unexpected dispatch apply: %+v", dispatch)
	}

	inputPath := handoff.ReviewerStagingCommands.SourceCaptureInput
	result := map[string]any{
		"packetId": packet.PacketID, "routeId": packet.Route.ID, "shardId": handoff.ShardID, "items": handoff.Items,
		"reviewerSession": "reviewer-session-cli-receipt", "decision": "accept", "confidence": "high", "summary": "reviewed alpha",
		"evidenceRefs": []string{"workspace/review-evidence.md"}, "risks": []string{}, "conflicts": []string{}, "recommendedVerdict": "accepted",
		"routeOutput": map[string]any{"item": "alpha", "decision": "accept", "confidence": "high", "evidence": "workspace/review-evidence.md", "risk": "low", "next_action": "main-agent-writeback", "tier_used": "light", "tool_scope": "read-only", "feature": "review", "request_id": "n/a", "candidate_path": "n/a", "defer_reason": "n/a"},
	}
	data, err := json.Marshal(result)
	if err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(filepath.Dir(inputPath), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(inputPath, data, 0o644); err != nil {
		t.Fatal(err)
	}

	out.Reset()
	if err := Run([]string{"-Command", "plan-subagents", "-Target", caseRoot, "-Pack", "_template", "-PacketPath", plan.PacketPath, "-RecordReviewerCompletion", "-ReviewerDispatchId", dispatch.DispatchID, "-ReviewerOutcome", "succeeded", "-ReviewerExitStatus", "completed", "-ReviewerResultInputPath", inputPath, "-Lane", packet.TargetLane, "-Actor", "mission-commander", "-WhatIf", "-Format", "json"}, &out); err != nil {
		t.Fatal(err)
	}
	var completionPreview subagents.ReviewerSessionReceiptResult
	if err := json.Unmarshal(out.Bytes(), &completionPreview); err != nil {
		t.Fatal(err)
	}
	if completionPreview.DispatchReceiptSHA256 != dispatch.ReceiptSHA256 || completionPreview.ReviewerResultInputSHA256 == "" || !strings.Contains(completionPreview.ApplyCommand, "-ExpectedReviewerResultInputSha256") {
		t.Fatalf("unexpected completion preview: %+v", completionPreview)
	}
	out.Reset()
	if err := Run([]string{"-Command", "plan-subagents", "-Target", caseRoot, "-Pack", "_template", "-PacketPath", plan.PacketPath, "-RecordReviewerCompletion", "-ReviewerDispatchId", dispatch.DispatchID, "-ReviewerOutcome", "succeeded", "-ReviewerExitStatus", "completed", "-ReviewerResultInputPath", inputPath, "-Lane", packet.TargetLane, "-Actor", "mission-commander", "-ExpectedReviewerDispatchReceiptSha256", completionPreview.DispatchReceiptSHA256, "-ExpectedReviewerResultInputSha256", completionPreview.ReviewerResultInputSHA256, "-Apply", "-Format", "json"}, &out); err != nil {
		t.Fatal(err)
	}
	var completion subagents.ReviewerSessionReceiptResult
	if err := json.Unmarshal(out.Bytes(), &completion); err != nil {
		t.Fatal(err)
	}
	if !completion.Applied || completion.Outcome != "succeeded" {
		t.Fatalf("unexpected completion apply: %+v", completion)
	}
	out.Reset()
	if err := Run([]string{"-Command", "plan-subagents", "-Target", caseRoot, "-Pack", "_template", "-PacketPath", plan.PacketPath, "-CaptureReviewerResultSource", "-ShardId", handoff.ShardID, "-ReviewerResultInputPath", inputPath, "-Lane", packet.TargetLane, "-Actor", "mission-commander", "-WhatIf", "-Format", "json"}, &out); err != nil {
		t.Fatal(err)
	}
}

func TestRunPlanSubagentsReviewerOperatorPackageExecutableRunLoopProductPath(t *testing.T) {
	caseRoot := fullAttachedCase(t)
	var out bytes.Buffer
	if err := Run([]string{"-Command", "start", "-Target", caseRoot, "-Pack", "_template", "-Name", "login", "-Apply", "-Executor", "session-login", "-Actor", "mission-commander", "-Reason", "operator package executable run loop"}, &out); err != nil {
		t.Fatal(err)
	}
	writeCaseFile(t, caseRoot, "workspace/features/feature-login/review-evidence.md", "bounded operator package reviewer evidence\n")

	out.Reset()
	if err := Run([]string{"-Command", "plan-subagents", "-Target", caseRoot, "-Pack", "_template", "-TaskType", "feature-analysis", "-Items", "alpha", "-ItemsPerAgent", "1", "-MaxParallel", "1", "-Lane", "feature-login", "-Format", "json"}, &out); err != nil {
		t.Fatal(err)
	}
	plan := decodePlanSubagentsResult(t, out.Bytes())
	packet := decodePlanSubagentsPacket(t, plan.PacketPath)
	handoff := plan.ShardHandoffs[0]
	baseArgs := []string{"-Target", caseRoot, "-Pack", "_template"}
	reviewerSession := "reviewer-session-operator-package"
	runtimeCommand := func(command string) string {
		return strings.NewReplacer("<harness>", "go-cli-operator-harness", "<session-id>", reviewerSession, "<main-agent>", "mission-commander").Replace(command)
	}
	type reviewerOperatorStatus struct {
		Summary          reviewerDispatchIntakeSummaryCLIItem
		ReviewerQueue    missionCommanderActionQueueSnapshot
		FirstScreenQueue missionCommanderActionQueueSnapshot
	}
	statusSnapshot := func(label string) reviewerOperatorStatus {
		out.Reset()
		if err := Run([]string{"-Command", "status", "-Target", caseRoot, "-Pack", "_template", "-Format", "json"}, &out); err != nil {
			t.Fatal(err)
		}
		var status struct {
			CaseMission struct {
				ReviewerDispatchIntakeHandoffs    []reviewerDispatchIntakeCLIItem      `json:"reviewerDispatchIntakeHandoffs"`
				ReviewerDispatchIntakeSummary     reviewerDispatchIntakeSummaryCLIItem `json:"reviewerDispatchIntakeSummary"`
				ReviewerDispatchIntakeActionQueue missionCommanderActionQueueSnapshot  `json:"reviewerDispatchIntakeActionQueue"`
				MissionCommanderActionQueue       missionCommanderActionQueueSnapshot  `json:"missionCommanderActionQueue"`
			} `json:"caseMission"`
		}
		if err := json.Unmarshal(out.Bytes(), &status); err != nil {
			t.Fatalf("%s status JSON did not decode: %v\n%s", label, err, out.String())
		}
		firstScreenCurrent := status.CaseMission.MissionCommanderActionQueue.CurrentAction
		reviewerCurrent := status.CaseMission.ReviewerDispatchIntakeActionQueue.CurrentAction
		firstScreenRequest := status.CaseMission.MissionCommanderActionQueue.CurrentDriverRequest
		reviewerRequest := status.CaseMission.ReviewerDispatchIntakeActionQueue.CurrentDriverRequest
		if firstScreenCurrent == nil || firstScreenCurrent.Source != "reviewerDispatchIntakeHandoffs" || reviewerCurrent == nil || reviewerCurrent.Label != firstScreenCurrent.Label || firstScreenRequest == nil || reviewerRequest == nil || firstScreenRequest.Kind != reviewerRequest.Kind || firstScreenRequest.Command != reviewerRequest.Command || firstScreenRequest.Guidance != reviewerRequest.Guidance {
			t.Fatalf("%s status omitted reviewer current driver request: firstScreen=%+v reviewer=%+v", label, status.CaseMission.MissionCommanderActionQueue, status.CaseMission.ReviewerDispatchIntakeActionQueue)
		}
		return reviewerOperatorStatus{Summary: status.CaseMission.ReviewerDispatchIntakeSummary, ReviewerQueue: status.CaseMission.ReviewerDispatchIntakeActionQueue, FirstScreenQueue: status.CaseMission.MissionCommanderActionQueue}
	}

	driverRequestCommandArgs := func(label string, request *missionCommanderDriverRequestSnapshot) []string {
		t.Helper()
		if request == nil {
			t.Fatalf("%s missing driver request", label)
		}
		requestCopy := *request
		requestCopy.Command = runtimeCommand(requestCopy.Command)
		args, ok := missionCommanderDriverRequestCommandCLIArgs(t, &requestCopy)
		if !ok {
			t.Fatalf("%s driver request did not expose executable command: %+v", label, request)
		}
		withBase := append([]string{}, args[:2]...)
		withBase = append(withBase, baseArgs...)
		return append(withBase, args[2:]...)
	}

	status := statusSnapshot("initial operator package")
	summary := status.Summary
	assertReviewerDispatchIntakeSummary(t, "initial operator package", summary, 1, 1, 0, "shard-01", "ready-for-reviewer-dispatch")
	assertReviewerDispatchOperatorPackage(t, "initial operator package", summary, "shard-01", handoff.DispatchPromptSHA256)
	pkg := summary.OperatorPackage
	if pkg.CurrentRunLoopStepID != "spawn-reviewer" || pkg.Current.ReviewerDispatchRecordCommand == "" || !strings.Contains(pkg.Current.DispatchCommand, "dispatch read-only reviewer for shard-01") {
		t.Fatalf("initial operator package did not point at reviewer dispatch: %+v", pkg)
	}

	dispatchRequest := requireMissionCommanderDriverRequest(t, status.FirstScreenQueue, "blocked-review", "inspect-current", pkg.Current.ReviewerDispatchRecordCommand, true, true, true)
	if status.ReviewerQueue.CurrentDriverRequest == nil || status.ReviewerQueue.CurrentDriverRequest.Command != dispatchRequest.Command {
		t.Fatalf("initial operator reviewer queue driver request drifted from first screen: firstScreen=%+v reviewer=%+v", dispatchRequest, status.ReviewerQueue.CurrentDriverRequest)
	}
	out.Reset()
	if err := Run(driverRequestCommandArgs("initial operator dispatch", dispatchRequest), &out); err != nil {
		t.Fatal(err)
	}
	var dispatchPreview struct {
		IsMutation    bool   `json:"isMutation"`
		Applied       bool   `json:"applied"`
		BindingSHA256 string `json:"bindingSha256"`
		ApplyCommand  string `json:"applyCommand"`
	}
	if err := json.Unmarshal(out.Bytes(), &dispatchPreview); err != nil {
		t.Fatalf("operator dispatch preview JSON did not decode: %v\n%s", err, out.String())
	}
	if dispatchPreview.IsMutation || dispatchPreview.Applied || dispatchPreview.BindingSHA256 == "" || !strings.Contains(dispatchPreview.ApplyCommand, "-ExpectedReviewerDispatchBindingSha256") {
		t.Fatalf("unexpected operator dispatch preview: %+v", dispatchPreview)
	}
	out.Reset()
	if err := Run(reviewerPrimaryCommandCLIArgs(t, baseArgs, runtimeCommand(dispatchPreview.ApplyCommand)), &out); err != nil {
		t.Fatal(err)
	}
	var dispatchApply struct {
		Applied       bool   `json:"applied"`
		DispatchID    string `json:"dispatchId"`
		ReceiptSHA256 string `json:"receiptSha256"`
	}
	if err := json.Unmarshal(out.Bytes(), &dispatchApply); err != nil {
		t.Fatalf("operator dispatch apply JSON did not decode: %v\n%s", err, out.String())
	}
	if !dispatchApply.Applied || dispatchApply.DispatchID == "" || dispatchApply.ReceiptSHA256 == "" {
		t.Fatalf("unexpected operator dispatch apply: %+v", dispatchApply)
	}

	status = statusSnapshot("after operator dispatch")
	summary = status.Summary
	assertReviewerDispatchOperatorPackage(t, "after operator dispatch", summary, "shard-01", handoff.DispatchPromptSHA256)
	if summary.OperatorPackage.CurrentRunLoopStepID != "save-result-input" || summary.OperatorPackage.Current.State != "reviewer-session-running-unknown" || summary.OperatorPackage.Current.ReviewerDispatchID != dispatchApply.DispatchID || summary.OperatorPackage.Current.ReviewerSession != reviewerSession {
		t.Fatalf("operator package did not advance to saved-input handoff: %+v", summary.OperatorPackage)
	}
	runningRequest := requireMissionCommanderDriverRequest(t, status.ReviewerQueue, "blocked-review", "inspect-current", "", false, true, true)
	if !strings.Contains(runningRequest.Guidance, "inspect harness reviewer session") {
		t.Fatalf("operator running driver request omitted manual harness guidance: %+v", runningRequest)
	}
	if args, ok := missionCommanderDriverRequestCommandCLIArgs(t, runningRequest); ok || args != nil {
		t.Fatalf("operator running guidance must not produce executable CLI args: args=%+v request=%+v", args, runningRequest)
	}
	resultData := reviewerResultForCLIPlan(t, packet, handoff, "accept", "accepted", reviewerSession)
	if err := os.MkdirAll(filepath.Dir(summary.OperatorPackage.Current.ReviewerResultInputPath), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(summary.OperatorPackage.Current.ReviewerResultInputPath, resultData, 0o644); err != nil {
		t.Fatal(err)
	}

	status = statusSnapshot("after operator input save")
	summary = status.Summary
	assertReviewerDispatchOperatorPackage(t, "after operator input save", summary, "shard-01", handoff.DispatchPromptSHA256)
	if summary.OperatorPackage.CurrentRunLoopStepID != "record-completion" || summary.OperatorPackage.Current.State != "ready-for-reviewer-completion-receipt-preview" || summary.OperatorPackage.Current.ReviewerCompletionRecordCommand == "" {
		t.Fatalf("operator package did not advance to completion receipt: %+v", summary.OperatorPackage)
	}
	completionRequest := requireMissionCommanderDriverRequest(t, status.ReviewerQueue, "blocked-review", "inspect-current", summary.OperatorPackage.Current.ReviewerCompletionRecordCommand, true, true, true)
	out.Reset()
	if err := Run(driverRequestCommandArgs("operator completion receipt", completionRequest), &out); err != nil {
		t.Fatal(err)
	}
	var completionPreview struct {
		Applied                   bool   `json:"applied"`
		DispatchReceiptSHA256     string `json:"dispatchReceiptSha256"`
		ReviewerResultInputSHA256 string `json:"reviewerResultInputSha256"`
		ApplyCommand              string `json:"applyCommand"`
	}
	if err := json.Unmarshal(out.Bytes(), &completionPreview); err != nil {
		t.Fatalf("operator completion preview JSON did not decode: %v\n%s", err, out.String())
	}
	if completionPreview.Applied || completionPreview.DispatchReceiptSHA256 != dispatchApply.ReceiptSHA256 || completionPreview.ReviewerResultInputSHA256 == "" || !strings.Contains(completionPreview.ApplyCommand, "-ExpectedReviewerResultInputSha256") {
		t.Fatalf("unexpected operator completion preview: %+v", completionPreview)
	}
	out.Reset()
	if err := Run(reviewerPrimaryCommandCLIArgs(t, baseArgs, runtimeCommand(completionPreview.ApplyCommand)), &out); err != nil {
		t.Fatal(err)
	}
	var completionApply struct {
		Applied bool   `json:"applied"`
		Outcome string `json:"outcome"`
	}
	if err := json.Unmarshal(out.Bytes(), &completionApply); err != nil {
		t.Fatalf("operator completion apply JSON did not decode: %v\n%s", err, out.String())
	}
	if !completionApply.Applied || completionApply.Outcome != "succeeded" {
		t.Fatalf("unexpected operator completion apply: %+v", completionApply)
	}

	status = statusSnapshot("after operator completion")
	summary = status.Summary
	assertReviewerDispatchOperatorPackage(t, "after operator completion", summary, "shard-01", handoff.DispatchPromptSHA256)
	if summary.OperatorPackage.CurrentRunLoopStepID != "source-capture" || summary.OperatorPackage.Current.State != "ready-for-reviewer-result-source-capture-preview" {
		t.Fatalf("operator package did not advance to source capture: %+v", summary.OperatorPackage)
	}
	sourceCaptureRequest := requireMissionCommanderDriverRequest(t, status.ReviewerQueue, "preview-command", "preview-current", summary.OperatorPackage.Current.ReviewerResultSourceCapturePreviewCommand, true, false, true)
	out.Reset()
	if err := Run(driverRequestCommandArgs("operator source capture", sourceCaptureRequest), &out); err != nil {
		t.Fatal(err)
	}
	sourcePreview := decodeReviewerResultSourceCaptureCLIResult(t, out.Bytes())
	if sourcePreview.Status != "previewed" || sourcePreview.MissionCommanderAction.State != "ready-for-reviewer-result-source-capture-apply" || !strings.Contains(sourcePreview.MissionCommanderAction.PrimaryCommand, "-ExpectedReviewerResultInputSha256") {
		t.Fatalf("unexpected operator source capture preview: %+v", sourcePreview)
	}
	out.Reset()
	if err := Run(reviewerPrimaryCommandCLIArgs(t, baseArgs, runtimeCommand(sourcePreview.MissionCommanderAction.PrimaryCommand)), &out); err != nil {
		t.Fatal(err)
	}
	sourceApply := decodeReviewerResultSourceCaptureCLIResult(t, out.Bytes())
	if sourceApply.Status != "captured" || !sourceApply.Applied || sourceApply.MissionCommanderAction.State != "reviewer-result-source-ready-for-staging-preview" {
		t.Fatalf("unexpected operator source capture apply: %+v", sourceApply)
	}

	status = statusSnapshot("after operator source capture")
	summary = status.Summary
	assertReviewerDispatchOperatorPackage(t, "after operator source capture", summary, "shard-01", handoff.DispatchPromptSHA256)
	if summary.OperatorPackage.CurrentRunLoopStepID != "stage-candidate" || summary.OperatorPackage.Current.State != "ready-for-reviewer-result-staging-preview" {
		t.Fatalf("operator package did not advance to staging: %+v", summary.OperatorPackage)
	}
	stagingRequest := requireMissionCommanderDriverRequest(t, status.ReviewerQueue, "preview-command", "preview-current", summary.OperatorPackage.Current.ReviewerResultStagingPreviewCommand, true, false, true)
	out.Reset()
	if err := Run(driverRequestCommandArgs("operator staging", stagingRequest), &out); err != nil {
		t.Fatal(err)
	}
	stagingPreview := decodeReviewerResultStagingCLIResult(t, out.Bytes())
	if stagingPreview.Status != "previewed" || stagingPreview.MissionCommanderAction.State != "ready-for-reviewer-result-staging-apply" || !strings.Contains(stagingPreview.MissionCommanderAction.PrimaryCommand, "-ExpectedSourceSha256") {
		t.Fatalf("unexpected operator staging preview: %+v", stagingPreview)
	}
	out.Reset()
	if err := Run(reviewerPrimaryCommandCLIArgs(t, baseArgs, runtimeCommand(stagingPreview.MissionCommanderAction.PrimaryCommand)), &out); err != nil {
		t.Fatal(err)
	}
	stagingApply := decodeReviewerResultStagingCLIResult(t, out.Bytes())
	if stagingApply.Status != "staged" || !stagingApply.Applied || stagingApply.MissionCommanderAction.State != "reviewer-result-staged-ready-for-collection-preview" {
		t.Fatalf("unexpected operator staging apply: %+v", stagingApply)
	}

	status = statusSnapshot("after operator staging")
	summary = status.Summary
	assertReviewerDispatchOperatorPackage(t, "after operator staging", summary, "shard-01", handoff.DispatchPromptSHA256)
	if summary.OperatorPackage.CurrentRunLoopStepID != "collect-result" || summary.OperatorPackage.Current.State != "ready-for-reviewer-result-collection-preview" {
		t.Fatalf("operator package did not advance to collection: %+v", summary.OperatorPackage)
	}
	collectionRequest := requireMissionCommanderDriverRequest(t, status.ReviewerQueue, "preview-command", "preview-current", summary.OperatorPackage.Current.ReviewerResultCollectionPreviewCommand, true, false, true)
	out.Reset()
	if err := Run(driverRequestCommandArgs("operator collection", collectionRequest), &out); err != nil {
		t.Fatal(err)
	}
	collectionPreview := decodeReviewerResultCollectionCLIResult(t, out.Bytes())
	if collectionPreview.Status != "previewed" || collectionPreview.MissionCommanderAction.State != "ready-for-reviewer-result-collection-apply" || !strings.Contains(collectionPreview.MissionCommanderAction.PrimaryCommand, "-Apply") {
		t.Fatalf("unexpected operator collection preview: %+v", collectionPreview)
	}
	out.Reset()
	if err := Run(reviewerPrimaryCommandCLIArgs(t, baseArgs, runtimeCommand(collectionPreview.MissionCommanderAction.PrimaryCommand)), &out); err != nil {
		t.Fatal(err)
	}
	collectionApply := decodeReviewerResultCollectionCLIResult(t, out.Bytes())
	if collectionApply.Status != "collected" || !collectionApply.Applied || collectionApply.MissionCommanderAction.State != "reviewer-result-collected-ready-for-batch-intake-preview" {
		t.Fatalf("unexpected operator collection apply: %+v", collectionApply)
	}

	status = statusSnapshot("after operator collection")
	summary = status.Summary
	assertReviewerDispatchOperatorPackage(t, "after operator collection", summary, "shard-01", handoff.DispatchPromptSHA256)
	if summary.OperatorPackage.CurrentRunLoopStepID != "intake-results" || summary.OperatorPackage.Current.State != "ready-for-reviewer-intake-preview" || summary.OperatorPackage.Current.ReviewerResultBatchIntakePreviewCommand == "" {
		t.Fatalf("operator package did not advance to intake: %+v", summary.OperatorPackage)
	}
	intakeRequest := requireMissionCommanderDriverRequest(t, status.ReviewerQueue, "preview-command", "preview-current", summary.OperatorPackage.Current.ReviewerResultBatchIntakePreviewCommand, true, false, true)
	out.Reset()
	if err := Run(driverRequestCommandArgs("operator batch intake", intakeRequest), &out); err != nil {
		t.Fatal(err)
	}
	batchPreview := decodeReviewerBatchIntakeResult(t, out.Bytes())
	if batchPreview.IsMutation || batchPreview.Applied || batchPreview.Total != 1 || batchPreview.Ready != 1 || batchPreview.Processed != 1 || batchPreview.Completed != 0 || batchPreview.Waiting != 0 || batchPreview.MissionCommanderAction.State != "ready-for-reviewer-batch-intake-apply-after-preview" || !strings.Contains(batchPreview.MissionCommanderAction.PrimaryCommand, "-Apply") {
		t.Fatalf("unexpected operator batch intake preview: %+v", batchPreview)
	}
	out.Reset()
	if err := Run(reviewerPrimaryCommandCLIArgs(t, baseArgs, runtimeCommand(batchPreview.MissionCommanderAction.PrimaryCommand)), &out); err != nil {
		t.Fatal(err)
	}
	batchApply := decodeReviewerBatchIntakeResult(t, out.Bytes())
	if !batchApply.IsMutation || !batchApply.Applied || batchApply.Total != 1 || batchApply.Completed != 1 || batchApply.AlreadyComplete != 0 || batchApply.Partial || batchApply.MissionCommanderAction.State != "reviewer-batch-intake-writeback-complete" {
		t.Fatalf("unexpected operator batch intake apply: %+v", batchApply)
	}

	out.Reset()
	if err := Run([]string{"-Command", "status", "-Target", caseRoot, "-Pack", "_template", "-Format", "json"}, &out); err != nil {
		t.Fatal(err)
	}
	var finalStatus struct {
		CaseMission struct {
			ReviewerDispatchIntakeHandoffs []reviewerDispatchIntakeCLIItem      `json:"reviewerDispatchIntakeHandoffs"`
			ReviewerDispatchIntakeSummary  reviewerDispatchIntakeSummaryCLIItem `json:"reviewerDispatchIntakeSummary"`
			ReviewerWritebackSummary       reviewerWritebackSummaryCLIItem      `json:"reviewerWritebackSummary"`
		} `json:"caseMission"`
	}
	if err := json.Unmarshal(out.Bytes(), &finalStatus); err != nil {
		t.Fatalf("final operator status JSON did not decode: %v\n%s", err, out.String())
	}
	if len(finalStatus.CaseMission.ReviewerDispatchIntakeHandoffs) != 0 || finalStatus.CaseMission.ReviewerDispatchIntakeSummary.Total != 0 || finalStatus.CaseMission.ReviewerDispatchIntakeSummary.OperatorPackage != nil || finalStatus.CaseMission.ReviewerWritebackSummary.Total != 2 || finalStatus.CaseMission.ReviewerWritebackSummary.LatestReviewerSession != reviewerSession {
		t.Fatalf("operator package run loop did not close reviewer handoff: %+v", finalStatus.CaseMission)
	}
	verifications := readOptionalCaseFile(t, caseRoot, ".rekit/facts/verifications.jsonl")
	decisions := readOptionalCaseFile(t, caseRoot, ".rekit/facts/decisions.jsonl")
	for _, expected := range []string{reviewerSession, "ownerBindingMode", "current-executor-generation", "feature-login"} {
		if !strings.Contains(verifications, expected) || !strings.Contains(decisions, expected) {
			t.Fatalf("operator reviewer provenance %q missing:\nverifications=%s\ndecisions=%s", expected, verifications, decisions)
		}
	}
}

func TestRunPlanSubagentsRejectsShardIDOutsideReviewerShardModes(t *testing.T) {
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
	out.Reset()
	err := Run([]string{"-Command", "plan-subagents", "-Target", caseRoot, "-Pack", "_template", "-PacketPath", plan.PacketPath, "-ReadyReviewerResults", "-ShardId", "shard-01", "-Lane", "feature-review", "-Actor", "mission-commander", "-Apply", "-Format", "json"}, &out)
	if err == nil || !strings.Contains(err.Error(), "supported only with -StageReviewerResult") {
		t.Fatalf("-ShardId outside collection error = %v", err)
	}
	if got := readOptionalCaseFile(t, caseRoot, ".rekit/facts/verifications.jsonl"); got != "" {
		t.Fatalf("invalid shard-scoped batch intake wrote verification facts:\n%s", got)
	}
}

func TestRunPlanSubagentsReadyReviewerResultsCaseLocalProductPath(t *testing.T) {
	caseRoot := fullAttachedCase(t)
	var out bytes.Buffer
	if err := Run([]string{"-Command", "start", "-Target", caseRoot, "-Pack", "_template", "-Name", "review", "-Apply"}, &out); err != nil {
		t.Fatal(err)
	}
	writeCaseFile(t, caseRoot, "workspace/reviewer-batch-evidence.md", "bounded reviewer batch evidence\n")

	oldwd, err := os.Getwd()
	if err != nil {
		t.Fatal(err)
	}
	nested := filepath.Join(caseRoot, "workspace")
	if err := os.Chdir(nested); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		if err := os.Chdir(oldwd); err != nil {
			t.Fatal(err)
		}
	})

	out.Reset()
	if err := Run([]string{"-Command", "plan-subagents", "-TaskType", "feature-analysis", "-Items", "alpha,beta", "-ItemsPerAgent", "1", "-MaxParallel", "2", "-Lane", "feature-review", "-Format", "json"}, &out); err != nil {
		t.Fatal(err)
	}
	plan := decodePlanSubagentsResult(t, out.Bytes())
	packet := decodePlanSubagentsPacket(t, plan.PacketPath)
	if len(plan.ShardHandoffs) != 2 {
		t.Fatalf("reviewer batch product fixture shard count = %d, want 2", len(plan.ShardHandoffs))
	}
	var batchPreviewCommand string
	for i, handoff := range plan.ShardHandoffs {
		result := map[string]any{
			"packetId": packet.PacketID, "routeId": packet.Route.ID, "shardId": handoff.ShardID, "items": append([]string{}, handoff.Items...),
			"reviewerSession": fmt.Sprintf("reviewer-batch-product-%d", i+1), "decision": "accept", "confidence": "high", "summary": "reviewed " + handoff.ShardID + " in batch product path",
			"evidenceRefs": []string{"workspace/reviewer-batch-evidence.md"}, "risks": []string{}, "conflicts": []string{}, "recommendedVerdict": "accepted",
			"routeOutput": map[string]any{"item": strings.Join(handoff.Items, ","), "decision": "accept", "confidence": "high", "evidence": "workspace/reviewer-batch-evidence.md", "risk": "low", "next_action": "main-agent-writeback", "tier_used": "light", "tool_scope": "read-only", "feature": "review", "request_id": "n/a", "candidate_path": "n/a", "defer_reason": "n/a"},
		}
		data, err := json.Marshal(result)
		if err != nil {
			t.Fatal(err)
		}
		if handoff.ReviewerResultCandidatePath == "" || handoff.ReviewerResultCandidatePath == handoff.ReviewerResultPath {
			t.Fatalf("reviewer collection candidate path is not distinct: %+v", handoff)
		}
		if handoff.ReviewerStagingCommands == nil || handoff.ReviewerStagingCommands.SourcePath == "" || handoff.ReviewerStagingCommands.SourcePathArgument != handoff.ReviewerStagingCommands.SourcePath {
			t.Fatalf("reviewer staging source path was not packet-derived: %+v", handoff.ReviewerStagingCommands)
		}
		sourcePath := handoff.ReviewerStagingCommands.SourcePath
		inputPath := handoff.ReviewerStagingCommands.SourceCaptureInput
		if inputPath == "" || inputPath != filepath.Join(filepath.Dir(sourcePath), "..", "inputs", handoff.ShardID+".reviewer-input.json") || !strings.Contains(handoff.ReviewerStagingCommands.SourceCaptureCommand, "-ReviewerResultInputPath") || !strings.Contains(handoff.ReviewerStagingCommands.SourceCaptureCommand, inputPath) || !strings.Contains(handoff.ReviewerStagingCommands.SourceCaptureApply, "-ExpectedReviewerResultInputSha256") {
			t.Fatalf("reviewer input drop path was not packet-derived: %+v", handoff.ReviewerStagingCommands)
		}
		recordReviewerSessionReceiptsForCLIPlan(t, &out, nil, plan.PacketPath, handoff, "feature-review", "mission-commander", fmt.Sprintf("go-cli-batch-harness-%d", i+1), data)
		out.Reset()
		if err := Run([]string{"-Command", "plan-subagents", "-PacketPath", plan.PacketPath, "-CaptureReviewerResultSource", "-ShardId", handoff.ShardID, "-ReviewerResultInputPath", inputPath, "-Lane", "feature-review", "-Actor", "mission-commander", "-WhatIf", "-Format", "json"}, &out); err != nil {
			t.Fatal(err)
		}
		var sourcePreview struct {
			Mode                   string   `json:"mode"`
			Status                 string   `json:"status"`
			IsMutation             bool     `json:"isMutation"`
			Applied                bool     `json:"applied"`
			InputPath              string   `json:"inputPath"`
			InputSHA256            string   `json:"inputSha256"`
			SourcePath             string   `json:"sourcePath"`
			SourceSHA256           string   `json:"sourceSha256"`
			RunbookSteps           []string `json:"runbookSteps"`
			Boundary               []string `json:"boundary"`
			MissionCommanderAction struct {
				State          string `json:"state"`
				PrimaryCommand string `json:"primaryCommand"`
			} `json:"missionCommanderAction"`
		}
		if err := json.Unmarshal(out.Bytes(), &sourcePreview); err != nil {
			t.Fatalf("reviewer source capture preview JSON did not decode: %v\n%s", err, out.String())
		}
		if sourcePreview.Mode != "reviewer-result-source-capture" || sourcePreview.Status != "previewed" || sourcePreview.IsMutation || sourcePreview.Applied || sourcePreview.InputPath != inputPath || sourcePreview.SourcePath != sourcePath || sourcePreview.InputSHA256 == "" || sourcePreview.SourceSHA256 != sourcePreview.InputSHA256 || sourcePreview.MissionCommanderAction.State != "ready-for-reviewer-result-source-capture-apply" || !strings.Contains(sourcePreview.MissionCommanderAction.PrimaryCommand, "-ExpectedReviewerResultInputSha256") || !containsSubstring(sourcePreview.RunbookSteps, "source capture") || !containsSubstring(sourcePreview.Boundary, "does not append facts") {
			t.Fatalf("unexpected reviewer source capture preview: %+v", sourcePreview)
		}
		if _, err := os.Stat(sourcePath); !os.IsNotExist(err) {
			t.Fatalf("reviewer source capture WhatIf wrote source: %v", err)
		}
		out.Reset()
		if err := Run([]string{"-Command", "plan-subagents", "-PacketPath", plan.PacketPath, "-CaptureReviewerResultSource", "-ShardId", handoff.ShardID, "-ReviewerResultInputPath", inputPath, "-Lane", "feature-review", "-Actor", "mission-commander", "-ExpectedReviewerResultInputSha256", sourcePreview.InputSHA256, "-Apply", "-Format", "json"}, &out); err != nil {
			t.Fatal(err)
		}
		var sourceApply struct {
			Mode                   string `json:"mode"`
			Status                 string `json:"status"`
			IsMutation             bool   `json:"isMutation"`
			Applied                bool   `json:"applied"`
			AlreadyCaptured        bool   `json:"alreadyCaptured"`
			InputPath              string `json:"inputPath"`
			InputSHA256            string `json:"inputSha256"`
			SourcePath             string `json:"sourcePath"`
			SourceSHA256           string `json:"sourceSha256"`
			MissionCommanderAction struct {
				State          string `json:"state"`
				PrimaryCommand string `json:"primaryCommand"`
			} `json:"missionCommanderAction"`
		}
		if err := json.Unmarshal(out.Bytes(), &sourceApply); err != nil {
			t.Fatalf("reviewer source capture apply JSON did not decode: %v\n%s", err, out.String())
		}
		if sourceApply.Mode != "reviewer-result-source-capture" || sourceApply.Status != "captured" || !sourceApply.IsMutation || !sourceApply.Applied || sourceApply.AlreadyCaptured || sourceApply.InputPath != inputPath || sourceApply.SourcePath != sourcePath || sourceApply.InputSHA256 != sourcePreview.InputSHA256 || sourceApply.SourceSHA256 != sourcePreview.SourceSHA256 || sourceApply.MissionCommanderAction.State != "reviewer-result-source-ready-for-staging-preview" || !strings.Contains(sourceApply.MissionCommanderAction.PrimaryCommand, "-StageReviewerResult") {
			t.Fatalf("unexpected reviewer source capture apply: %+v", sourceApply)
		}
		captured, err := os.ReadFile(sourcePath)
		if err != nil {
			t.Fatal(err)
		}
		if !bytes.Equal(captured, data) {
			t.Fatalf("reviewer source capture wrote different bytes: %s", string(captured))
		}
		out.Reset()
		if err := Run([]string{"-Command", "plan-subagents", "-PacketPath", plan.PacketPath, "-CaptureReviewerResultSource", "-ShardId", handoff.ShardID, "-ReviewerResultInputPath", inputPath, "-Lane", "feature-review", "-Actor", "mission-commander", "-WhatIf", "-Format", "json"}, &out); err != nil {
			t.Fatal(err)
		}
		var sourceReplayPreview struct {
			Mode                   string `json:"mode"`
			Status                 string `json:"status"`
			IsMutation             bool   `json:"isMutation"`
			Applied                bool   `json:"applied"`
			AlreadyCaptured        bool   `json:"alreadyCaptured"`
			InputPath              string `json:"inputPath"`
			InputSHA256            string `json:"inputSha256"`
			SourcePath             string `json:"sourcePath"`
			SourceSHA256           string `json:"sourceSha256"`
			MissionCommanderAction struct {
				State          string `json:"state"`
				PrimaryCommand string `json:"primaryCommand"`
			} `json:"missionCommanderAction"`
		}
		if err := json.Unmarshal(out.Bytes(), &sourceReplayPreview); err != nil {
			t.Fatalf("reviewer source capture replay preview JSON did not decode: %v\n%s", err, out.String())
		}
		if sourceReplayPreview.Mode != "reviewer-result-source-capture" || sourceReplayPreview.Status != "already-captured" || sourceReplayPreview.IsMutation || sourceReplayPreview.Applied || !sourceReplayPreview.AlreadyCaptured || sourceReplayPreview.InputPath != inputPath || sourceReplayPreview.SourcePath != sourcePath || sourceReplayPreview.InputSHA256 != sourcePreview.InputSHA256 || sourceReplayPreview.SourceSHA256 != sourcePreview.SourceSHA256 || sourceReplayPreview.MissionCommanderAction.State != "reviewer-result-source-ready-for-staging-preview" || !strings.Contains(sourceReplayPreview.MissionCommanderAction.PrimaryCommand, "-StageReviewerResult") || strings.Contains(sourceReplayPreview.MissionCommanderAction.PrimaryCommand, "-ExpectedReviewerResultInputSha256") {
			t.Fatalf("unexpected reviewer source capture replay preview: %+v", sourceReplayPreview)
		}
		if _, err := os.Stat(handoff.ReviewerResultCandidatePath); !os.IsNotExist(err) {
			t.Fatalf("reviewer source capture replay WhatIf wrote candidate: %v", err)
		}
		out.Reset()
		if err := Run([]string{"-Command", "plan-subagents", "-PacketPath", plan.PacketPath, "-StageReviewerResult", "-ShardId", handoff.ShardID, "-ReviewerResultSourcePath", sourcePath, "-Lane", "feature-review", "-Actor", "mission-commander", "-WhatIf", "-Format", "json"}, &out); err != nil {
			t.Fatal(err)
		}
		stagingPreview := decodeReviewerResultStagingCLIResult(t, out.Bytes())
		if stagingPreview.Mode != "reviewer-result-staging" || stagingPreview.Status != "previewed" || stagingPreview.IsMutation || stagingPreview.Applied || stagingPreview.SourceSHA256 == "" || stagingPreview.CandidatePath != handoff.ReviewerResultCandidatePath {
			t.Fatalf("unexpected reviewer result staging preview: %+v", stagingPreview)
		}
		if _, err := os.Stat(handoff.ReviewerResultCandidatePath); !os.IsNotExist(err) {
			t.Fatalf("reviewer result staging WhatIf wrote candidate: %v", err)
		}
		out.Reset()
		if err := Run([]string{"-Command", "plan-subagents", "-PacketPath", plan.PacketPath, "-StageReviewerResult", "-ShardId", handoff.ShardID, "-ReviewerResultSourcePath", sourcePath, "-Lane", "feature-review", "-Actor", "mission-commander", "-ExpectedSourceSha256", stagingPreview.SourceSHA256, "-Apply", "-Format", "json"}, &out); err != nil {
			t.Fatal(err)
		}
		stagingApply := decodeReviewerResultStagingCLIResult(t, out.Bytes())
		if stagingApply.Status != "staged" || !stagingApply.IsMutation || !stagingApply.Applied || stagingApply.AlreadyStaged {
			t.Fatalf("unexpected reviewer result staging apply: %+v", stagingApply)
		}

		out.Reset()
		if err := Run([]string{"-Command", "plan-subagents", "-PacketPath", plan.PacketPath, "-CollectReviewerResult", "-ShardId", handoff.ShardID, "-Lane", "feature-review", "-Actor", "mission-commander", "-WhatIf", "-Format", "json"}, &out); err != nil {
			t.Fatal(err)
		}
		collectionPreview := decodeReviewerResultCollectionCLIResult(t, out.Bytes())
		if collectionPreview.Mode != "reviewer-result-collection" || collectionPreview.Status != "previewed" || collectionPreview.IsMutation || collectionPreview.Applied || collectionPreview.CandidatePath != handoff.ReviewerResultCandidatePath || collectionPreview.ReviewerResultPath != handoff.ReviewerResultPath {
			t.Fatalf("unexpected reviewer result collection preview: %+v", collectionPreview)
		}
		if _, err := os.Stat(handoff.ReviewerResultPath); !os.IsNotExist(err) {
			t.Fatalf("reviewer result collection WhatIf wrote canonical result: %v", err)
		}

		out.Reset()
		if err := Run([]string{"-Command", "plan-subagents", "-PacketPath", plan.PacketPath, "-CollectReviewerResult", "-ShardId", handoff.ShardID, "-Lane", "feature-review", "-Actor", "mission-commander", "-Apply", "-Format", "json"}, &out); err != nil {
			t.Fatal(err)
		}
		collectionApply := decodeReviewerResultCollectionCLIResult(t, out.Bytes())
		if collectionApply.Status != "collected" || !collectionApply.IsMutation || !collectionApply.Applied || collectionApply.AlreadyCollected || collectionApply.CandidateSHA256 == "" || collectionApply.CandidateSHA256 != collectionApply.ReviewerResultSHA256 || collectionApply.MissionCommanderAction.State != "reviewer-result-collected-ready-for-batch-intake-preview" || !strings.Contains(collectionApply.MissionCommanderAction.PrimaryCommand, "-ReadyReviewerResults") {
			t.Fatalf("unexpected reviewer result collection apply: %+v", collectionApply)
		}
		batchPreviewCommand = strings.Replace(collectionApply.MissionCommanderAction.PrimaryCommand, "-Actor <main-agent>", "-Actor mission-commander", 1)
	}

	firstSingleApplyCommand := plan.ShardHandoffs[0].ReviewerIntakeCommands.ApplyCommand
	if firstSingleApplyCommand == "" || !strings.Contains(firstSingleApplyCommand, "-ReviewerResultPath") || !strings.Contains(firstSingleApplyCommand, "-Apply") || !strings.Contains(firstSingleApplyCommand, "-Actor <main-agent>") {
		t.Fatalf("first shard did not expose single intake apply command: %+v", plan.ShardHandoffs[0].ReviewerIntakeCommands)
	}
	firstSingleApplyCommand = strings.Replace(firstSingleApplyCommand, "-Actor <main-agent>", "-Actor mission-commander", 1)
	out.Reset()
	if err := Run(reviewerPrimaryCommandCLIArgs(t, nil, firstSingleApplyCommand), &out); err != nil {
		t.Fatal(err)
	}
	firstSingleApply := decodeReviewerIntakeResult(t, out.Bytes())
	if firstSingleApply.WritebackStatus != "complete" || !firstSingleApply.IsMutation || !firstSingleApply.Applied || firstSingleApply.Summary.OrchestrationProgress == nil || firstSingleApply.Summary.OrchestrationProgress.Completed != 1 || firstSingleApply.Summary.OrchestrationProgress.Open != 1 || firstSingleApply.Summary.OrchestrationProgress.NextOpenShardID != "shard-02" || !slices.Contains(firstSingleApply.Summary.OrchestrationProgress.RemainingShardIDs, "shard-02") || firstSingleApply.MissionCommanderActionQueue.CurrentAction == nil || !strings.Contains(firstSingleApply.MissionCommanderActionQueue.CurrentAction.Command, "-ReadyReviewerResults") {
		t.Fatalf("single-shard intake did not expose partial reviewer batch continuation: result=%+v queue=%+v", firstSingleApply, firstSingleApply.MissionCommanderActionQueue)
	}
	if got := strings.Count(readOptionalCaseFile(t, caseRoot, ".rekit/facts/verifications.jsonl"), `"shardId"`); got != 1 {
		t.Fatalf("partial reviewer intake verification count = %d, want 1", got)
	}
	out.Reset()
	if err := Run([]string{"-Command", "status", "-Format", "json"}, &out); err != nil {
		t.Fatal(err)
	}
	var partialStatus struct {
		CaseMission struct {
			ReviewerWritebackSummary       reviewerWritebackSummaryCLIItem      `json:"reviewerWritebackSummary"`
			ReviewerDispatchIntakeHandoffs []reviewerDispatchIntakeCLIItem      `json:"reviewerDispatchIntakeHandoffs"`
			ReviewerDispatchIntakeSummary  reviewerDispatchIntakeSummaryCLIItem `json:"reviewerDispatchIntakeSummary"`
			MissionCommanderActionQueue    missionCommanderActionQueueSnapshot  `json:"missionCommanderActionQueue"`
		} `json:"caseMission"`
	}
	if err := json.Unmarshal(out.Bytes(), &partialStatus); err != nil {
		t.Fatalf("partial status stdout is not JSON: %v\n%s", err, out.String())
	}
	if partialStatus.CaseMission.ReviewerWritebackSummary.Total != 2 || len(partialStatus.CaseMission.ReviewerDispatchIntakeHandoffs) != 1 || partialStatus.CaseMission.ReviewerDispatchIntakeSummary.Total != 1 || partialStatus.CaseMission.ReviewerDispatchIntakeSummary.LatestPacketDispatchCompleted != 1 || partialStatus.CaseMission.ReviewerDispatchIntakeSummary.LatestPacketDispatchOpen != 1 || partialStatus.CaseMission.ReviewerDispatchIntakeSummary.NextActionShardID != "shard-02" || partialStatus.CaseMission.ReviewerDispatchIntakeSummary.NextActionState != "ready-for-reviewer-intake-preview" || !strings.Contains(partialStatus.CaseMission.ReviewerDispatchIntakeSummary.NextActionBatchPreviewCommand, "-ReadyReviewerResults") || partialStatus.CaseMission.MissionCommanderActionQueue.CurrentAction == nil || partialStatus.CaseMission.MissionCommanderActionQueue.CurrentAction.Source != "reviewerDispatchIntakeHandoffs" || partialStatus.CaseMission.MissionCommanderActionQueue.CurrentAction.State != "ready-for-reviewer-intake-preview" || !strings.Contains(partialStatus.CaseMission.MissionCommanderActionQueue.CurrentAction.Command, "-ReadyReviewerResults") {
		t.Fatalf("partial status omitted reviewer batch recovery handoff: %+v", partialStatus.CaseMission)
	}
	out.Reset()
	if err := Run([]string{"-Command", "continue", "review", "-WhatIf", "-Format", "json"}, &out); err != nil {
		t.Fatal(err)
	}
	var partialContinue struct {
		Blocked                        bool                                 `json:"blocked"`
		ReviewerDispatchIntakeHandoffs []reviewerDispatchIntakeCLIItem      `json:"reviewerDispatchIntakeHandoffs"`
		ReviewerDispatchIntakeSummary  reviewerDispatchIntakeSummaryCLIItem `json:"reviewerDispatchIntakeSummary"`
		MissionCommanderActionQueue    missionCommanderActionQueueSnapshot  `json:"missionCommanderActionQueue"`
	}
	if err := json.Unmarshal(out.Bytes(), &partialContinue); err != nil {
		t.Fatalf("partial continue stdout is not JSON: %v\n%s", err, out.String())
	}
	if !partialContinue.Blocked || len(partialContinue.ReviewerDispatchIntakeHandoffs) != 1 || partialContinue.ReviewerDispatchIntakeSummary.NextActionShardID != "shard-02" || partialContinue.MissionCommanderActionQueue.CurrentAction == nil || partialContinue.MissionCommanderActionQueue.CurrentAction.Source != "reviewerDispatchIntakeHandoffs" || !strings.Contains(partialContinue.MissionCommanderActionQueue.CurrentAction.Command, "-ReadyReviewerResults") {
		t.Fatalf("partial continue did not block on reviewer batch recovery handoff: %+v", partialContinue)
	}

	out.Reset()
	if err := Run(reviewerPrimaryCommandCLIArgs(t, nil, batchPreviewCommand), &out); err != nil {
		t.Fatal(err)
	}
	preview := decodeReviewerBatchIntakeResult(t, out.Bytes())
	if preview.Command != "plan-subagents" || preview.Mode != "reviewer-batch-intake" || preview.CaseRoot != caseRoot || preview.Pack != "_template" || preview.IsMutation || preview.Applied || preview.Total != 2 || preview.Ready != 2 || preview.Waiting != 0 || preview.Processed != 2 || preview.Completed != 0 || preview.AlreadyComplete != 1 || preview.Stopped || !preview.Partial || len(preview.Results) != 2 || preview.Results[0].WritebackStatus != "already-complete" || preview.Results[1].WritebackStatus != "previewed" || preview.NextOpenShardID != "shard-02" || !slices.Contains(preview.RemainingShardIDs, "shard-02") || !strings.Contains(preview.RerunCommand, "-ReadyReviewerResults") {
		t.Fatalf("unexpected case-local reviewer batch partial preview: %+v", preview)
	}
	if preview.MissionCommanderAction.State != "ready-for-reviewer-batch-intake-apply-after-preview" || !strings.Contains(preview.MissionCommanderAction.PrimaryCommand, "-Apply -Format json") || !containsMissionCommanderNextAction(preview.MissionCommanderNextActions, "reviewerBatchIntake", preview.MissionCommanderAction.PrimaryCommand, false, true) {
		t.Fatalf("case-local reviewer batch preview omitted apply handoff: action=%+v next=%+v", preview.MissionCommanderAction, preview.MissionCommanderNextActions)
	}
	assertCLIActionQueue(t, preview.MissionCommanderActionQueue, 1, 1, 0, 1, 0, preview.MissionCommanderAction.PrimaryCommand)
	if got := strings.Count(readOptionalCaseFile(t, caseRoot, ".rekit/facts/verifications.jsonl"), `"shardId"`); got != 1 {
		t.Fatalf("reviewer batch WhatIf changed verification count = %d, want existing single-shard count 1", got)
	}
	if got := strings.Count(readOptionalCaseFile(t, caseRoot, ".rekit/facts/decisions.jsonl"), `"shardId"`); got != 1 {
		t.Fatalf("reviewer batch WhatIf changed decision count = %d, want existing single-shard count 1", got)
	}

	out.Reset()
	if err := Run([]string{"-Command", "plan-subagents", "-PacketPath", plan.PacketPath, "-ReadyReviewerResults", "-Lane", "feature-review", "-Actor", "mission-commander", "-WhatIf", "-Format", "text"}, &out); err != nil {
		t.Fatal(err)
	}
	for _, expected := range []string{
		"plan-subagents reviewer batch intake：mutation=false applied=false lane=feature-review total=2 ready=2 waiting=0 processed=2 completed=0 alreadyComplete=1 stopped=false partial=true",
		"nextOpen=shard-02 remaining=shard-02",
		"reviewer batch intake shard：shard=shard-01 status=already-complete",
		"reviewer batch intake shard：shard=shard-02 status=previewed",
		"reviewer batch intake boundary：each shard preserves strict reviewer result validation and verification-before-decision writeback",
		"reviewer batch intake commander action：state=ready-for-reviewer-batch-intake-apply-after-preview",
		"mission commander action queue current：state=ready-for-reviewer-batch-intake-apply-after-preview source=reviewerBatchIntake blocked=false requiresReview=true command=`/rekit plan-subagents",
		"mission commander next action：state=ready-for-reviewer-batch-intake-apply-after-preview source=reviewerBatchIntake blocked=false requiresReview=true command=`/rekit plan-subagents",
	} {
		if !strings.Contains(out.String(), expected) {
			t.Fatalf("reviewer batch preview text missing %q:\n%s", expected, out.String())
		}
	}
	if strings.Contains(out.String(), "{\n  ") {
		t.Fatalf("reviewer batch text should not emit JSON object:\n%s", out.String())
	}

	out.Reset()
	if err := Run([]string{"-Command", "plan-subagents", "-PacketPath", plan.PacketPath, "-ReadyReviewerResults", "-Lane", "feature-review", "-Actor", "mission-commander", "-Apply", "-Format", "json"}, &out); err != nil {
		t.Fatal(err)
	}
	applied := decodeReviewerBatchIntakeResult(t, out.Bytes())
	if !applied.IsMutation || !applied.Applied || applied.Processed != 2 || applied.Completed != 1 || applied.AlreadyComplete != 1 || applied.Stopped || applied.Partial || applied.NextOpenShardID != "" || len(applied.RemainingShardIDs) != 0 || len(applied.Results) != 2 || applied.Results[0].WritebackStatus != "already-complete" || applied.Results[1].WritebackStatus != "complete" {
		t.Fatalf("unexpected case-local reviewer batch recovery apply: %+v", applied)
	}
	if applied.MissionCommanderAction.State != "reviewer-batch-intake-writeback-complete" || len(applied.MissionCommanderNextActions) == 0 || !strings.HasPrefix(applied.MissionCommanderNextActions[0].Source, "reviewerBatchIntake.reviewerIntake.postValidation.") || applied.MissionCommanderActionQueue.CurrentAction == nil {
		t.Fatalf("case-local reviewer batch apply omitted post-validation handoff: action=%+v next=%+v queue=%+v", applied.MissionCommanderAction, applied.MissionCommanderNextActions, applied.MissionCommanderActionQueue)
	}
	assertCLIActionQueue(t, applied.MissionCommanderActionQueue, 2, 2, 0, 0, 1, applied.MissionCommanderActionQueue.CurrentAction.Command)
	if got := strings.Count(readOptionalCaseFile(t, caseRoot, ".rekit/facts/verifications.jsonl"), `"shardId"`); got != 2 {
		t.Fatalf("reviewer batch verification count = %d, want 2", got)
	}
	if got := strings.Count(readOptionalCaseFile(t, caseRoot, ".rekit/facts/decisions.jsonl"), `"shardId"`); got != 2 {
		t.Fatalf("reviewer batch decision count = %d, want 2", got)
	}

	out.Reset()
	if err := Run([]string{"-Command", "status", "-Format", "json"}, &out); err != nil {
		t.Fatal(err)
	}
	var statusAfterBatch struct {
		CaseMission struct {
			ReviewerWritebacks             []reviewerWritebackCLIItem           `json:"reviewerWritebacks"`
			ReviewerWritebackSummary       reviewerWritebackSummaryCLIItem      `json:"reviewerWritebackSummary"`
			ReviewerDispatchIntakeHandoffs []reviewerDispatchIntakeCLIItem      `json:"reviewerDispatchIntakeHandoffs"`
			ReviewerDispatchIntakeSummary  reviewerDispatchIntakeSummaryCLIItem `json:"reviewerDispatchIntakeSummary"`
			MissionCommanderActionQueue    missionCommanderActionQueueSnapshot  `json:"missionCommanderActionQueue"`
		} `json:"caseMission"`
	}
	if err := json.Unmarshal(out.Bytes(), &statusAfterBatch); err != nil {
		t.Fatalf("status after reviewer batch intake stdout is not JSON: %v\n%s", err, out.String())
	}
	if len(statusAfterBatch.CaseMission.ReviewerDispatchIntakeHandoffs) != 0 || statusAfterBatch.CaseMission.ReviewerDispatchIntakeSummary.Total != 0 || statusAfterBatch.CaseMission.ReviewerDispatchIntakeSummary.NextAction != "" {
		t.Fatalf("status after reviewer batch intake should close reviewer dispatch/intake handoffs: %+v", statusAfterBatch.CaseMission)
	}
	if len(statusAfterBatch.CaseMission.ReviewerWritebacks) != 4 || statusAfterBatch.CaseMission.ReviewerWritebackSummary.Total != 4 || statusAfterBatch.CaseMission.ReviewerWritebackSummary.VerificationCount != 2 || statusAfterBatch.CaseMission.ReviewerWritebackSummary.DecisionCount != 2 || statusAfterBatch.CaseMission.ReviewerWritebackSummary.LatestLane != "feature-review" || statusAfterBatch.CaseMission.ReviewerWritebackSummary.LatestReviewerSession != "reviewer-batch-product-2" {
		t.Fatalf("status after reviewer batch intake omitted writeback closure: %+v", statusAfterBatch.CaseMission.ReviewerWritebackSummary)
	}
	if statusAfterBatch.CaseMission.MissionCommanderActionQueue.CurrentAction == nil || statusAfterBatch.CaseMission.MissionCommanderActionQueue.CurrentAction.Command != "/rekit continue review" {
		t.Fatalf("status after reviewer batch intake should advance to feature-review continuation: %+v", statusAfterBatch.CaseMission.MissionCommanderActionQueue)
	}

	out.Reset()
	if err := Run([]string{"-Command", "continue", "review", "-WhatIf", "-Format", "json"}, &out); err != nil {
		t.Fatal(err)
	}
	var continuePreview struct {
		Applied                        bool                                 `json:"applied"`
		Blocked                        bool                                 `json:"blocked"`
		RequiresConfirmation           bool                                 `json:"requiresConfirmation"`
		ReviewerWritebacks             []reviewerWritebackCLIItem           `json:"reviewerWritebacks"`
		ReviewerWritebackSummary       reviewerWritebackSummaryCLIItem      `json:"reviewerWritebackSummary"`
		ReviewerDispatchIntakeHandoffs []reviewerDispatchIntakeCLIItem      `json:"reviewerDispatchIntakeHandoffs"`
		ReviewerDispatchIntakeSummary  reviewerDispatchIntakeSummaryCLIItem `json:"reviewerDispatchIntakeSummary"`
		MissionCommanderActionQueue    missionCommanderActionQueueSnapshot  `json:"missionCommanderActionQueue"`
	}
	if err := json.Unmarshal(out.Bytes(), &continuePreview); err != nil {
		t.Fatalf("continue preview after reviewer batch intake stdout is not JSON: %v\n%s", err, out.String())
	}
	if continuePreview.Applied || continuePreview.Blocked || !continuePreview.RequiresConfirmation || len(continuePreview.ReviewerDispatchIntakeHandoffs) != 0 || continuePreview.ReviewerDispatchIntakeSummary.Total != 0 {
		t.Fatalf("continue preview after reviewer batch intake should be unblocked and dispatch-clean: %+v", continuePreview)
	}
	if len(continuePreview.ReviewerWritebacks) != 4 || continuePreview.ReviewerWritebackSummary.Total != 4 || continuePreview.ReviewerWritebackSummary.LatestShardID != "shard-02" || continuePreview.MissionCommanderActionQueue.CurrentAction == nil || continuePreview.MissionCommanderActionQueue.CurrentAction.Command != "/rekit continue review" {
		t.Fatalf("continue preview after reviewer batch intake omitted reviewer writeback continuation handoff: %+v", continuePreview)
	}

	factsBeforeContinue := snapshotFiles(t, filepath.Join(caseRoot, ".rekit", "facts"))
	out.Reset()
	if err := Run([]string{"-Command", "continue", "review", "-Apply", "-Format", "json"}, &out); err != nil {
		t.Fatal(err)
	}
	var continueApply struct {
		RunID                          string                              `json:"runId"`
		Applied                        bool                                `json:"applied"`
		Blocked                        bool                                `json:"blocked"`
		ReviewerWritebacks             []reviewerWritebackCLIItem          `json:"reviewerWritebacks"`
		ReviewerWritebackSummary       reviewerWritebackSummaryCLIItem     `json:"reviewerWritebackSummary"`
		ReviewerDispatchIntakeHandoffs []reviewerDispatchIntakeCLIItem     `json:"reviewerDispatchIntakeHandoffs"`
		MissionCommanderActionQueue    missionCommanderActionQueueSnapshot `json:"missionCommanderActionQueue"`
		Writes                         []startWrite                        `json:"writes"`
	}
	if err := json.Unmarshal(out.Bytes(), &continueApply); err != nil {
		t.Fatalf("continue apply after reviewer batch intake stdout is not JSON: %v\n%s", err, out.String())
	}
	if !continueApply.Applied || continueApply.Blocked || len(continueApply.ReviewerDispatchIntakeHandoffs) != 0 || len(continueApply.ReviewerWritebacks) != 4 || continueApply.ReviewerWritebackSummary.Total != 4 || continueApply.ReviewerWritebackSummary.LatestReviewerSession != "reviewer-batch-product-2" || continueApply.MissionCommanderActionQueue.CurrentAction == nil || continueApply.MissionCommanderActionQueue.CurrentAction.Command != "/rekit continue review" {
		t.Fatalf("continue apply after reviewer batch intake omitted durable unblocked handoff: %+v", continueApply)
	}
	assertSnapshotEqual(t, factsBeforeContinue, snapshotFiles(t, filepath.Join(caseRoot, ".rekit", "facts")))
	statusPath := assertStartWrite(t, continueApply.Writes, ".rekit/runs/"+continueApply.RunID+"/status.json", "write").TargetPath
	digestPath := assertStartWrite(t, continueApply.Writes, ".rekit/runs/"+continueApply.RunID+"/digest.md", "write").TargetPath
	resumePath := assertStartWrite(t, continueApply.Writes, ".rekit/lanes/feature-review/prompts/RESUME.md", "refresh").TargetPath
	checkpointPath := assertStartWrite(t, continueApply.Writes, ".rekit/lanes/feature-review/checkpoints/latest.json", "refresh").TargetPath
	statusBytes, err := os.ReadFile(statusPath)
	if err != nil {
		t.Fatal(err)
	}
	var runStatus struct {
		ReviewerWritebacks             []reviewerWritebackCLIItem      `json:"reviewerWritebacks"`
		ReviewerWritebackSummary       reviewerWritebackSummaryCLIItem `json:"reviewerWritebackSummary"`
		ReviewerDispatchIntakeHandoffs []reviewerDispatchIntakeCLIItem `json:"reviewerDispatchIntakeHandoffs"`
	}
	if err := json.Unmarshal(statusBytes, &runStatus); err != nil {
		t.Fatalf("continue run status after reviewer batch intake did not decode: %v\n%s", err, string(statusBytes))
	}
	if len(runStatus.ReviewerDispatchIntakeHandoffs) != 0 || len(runStatus.ReviewerWritebacks) != 4 || runStatus.ReviewerWritebackSummary.Total != 4 || runStatus.ReviewerWritebackSummary.LatestReviewerResult != plan.ShardHandoffs[1].ReviewerResultPath {
		t.Fatalf("continue run status after reviewer batch intake omitted closure: %+v", runStatus)
	}
	for label, path := range map[string]string{"continue digest": digestPath, "lane RESUME": resumePath} {
		text, err := os.ReadFile(path)
		if err != nil {
			t.Fatal(err)
		}
		for _, expected := range []string{"## Reviewer writeback", "summary: total=`4` verifications=`2` decisions=`2`", "latestShard=`shard-02`", "reviewer-batch-product-2", "reviewer writeback downstream handoff must not execute heavy tools or spawn reviewer sessions"} {
			if !strings.Contains(string(text), expected) {
				t.Fatalf("%s after reviewer batch intake omitted durable handoff %q:\n%s", label, expected, string(text))
			}
		}
	}
	checkpointBytes, err := os.ReadFile(checkpointPath)
	if err != nil {
		t.Fatal(err)
	}
	var checkpoint struct {
		ReviewerWritebackSummary       reviewerWritebackSummaryCLIItem     `json:"reviewerWritebackSummary"`
		ReviewerDispatchIntakeHandoffs []reviewerDispatchIntakeCLIItem     `json:"reviewerDispatchIntakeHandoffs"`
		MissionCommanderActionQueue    missionCommanderActionQueueSnapshot `json:"missionCommanderActionQueue"`
	}
	if err := json.Unmarshal(checkpointBytes, &checkpoint); err != nil {
		t.Fatalf("checkpoint after reviewer batch intake did not decode: %v\n%s", err, string(checkpointBytes))
	}
	if len(checkpoint.ReviewerDispatchIntakeHandoffs) != 0 || checkpoint.ReviewerWritebackSummary.Total != 4 || checkpoint.MissionCommanderActionQueue.CurrentAction == nil || checkpoint.MissionCommanderActionQueue.CurrentAction.Command != "/rekit continue review" {
		t.Fatalf("checkpoint after reviewer batch intake omitted durable continuation handoff: %+v", checkpoint)
	}
}

func TestRunPlanSubagentsReviewerIntakeBlockedRepairGuidanceCaseLocalProductPath(t *testing.T) {
	caseRoot := fullAttachedCase(t)
	var out bytes.Buffer
	if err := Run([]string{"-Command", "start", "-Target", caseRoot, "-Pack", "_template", "-Name", "review", "-Apply"}, &out); err != nil {
		t.Fatal(err)
	}
	start := decodeStartResult(t, out.Bytes())
	evidenceRel := start.Lane.Workspace + "/blocked-reviewer-evidence.md"
	writeCaseFile(t, caseRoot, evidenceRel, "bounded blocked reviewer evidence\n")

	oldwd, err := os.Getwd()
	if err != nil {
		t.Fatal(err)
	}
	nested := filepath.Join(caseRoot, filepath.FromSlash(start.Lane.Workspace))
	if err := os.Chdir(nested); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		if err := os.Chdir(oldwd); err != nil {
			t.Fatal(err)
		}
	})

	out.Reset()
	legacyReviewRoot := filepath.Join(caseRoot, "artifacts", "legacy-blocked-reviewer-intake")
	if err := Run([]string{"-Command", "plan-subagents", "-TaskType", "feature-analysis", "-Items", "alpha", "-ReviewOutputDir", legacyReviewRoot}, &out); err != nil {
		t.Fatal(err)
	}
	plan := decodePlanSubagentsResult(t, out.Bytes())
	packet := decodePlanSubagentsPacket(t, plan.PacketPath)
	resultPath := filepath.Join(t.TempDir(), "reviewer-result.json")
	reviewerResult := map[string]any{
		"packetId":           packet.PacketID,
		"routeId":            packet.Route.ID,
		"shardId":            "shard-01",
		"items":              []string{"alpha"},
		"reviewerSession":    "reviewer-session-blocked-product-path",
		"decision":           "accept",
		"confidence":         "high",
		"summary":            "reviewed alpha but found conflicting shard ownership",
		"evidenceRefs":       []string{evidenceRel},
		"risks":              []string{},
		"conflicts":          []string{"overlaps another shard"},
		"recommendedVerdict": "accepted",
		"routeOutput": map[string]any{
			"item": "alpha", "decision": "accept", "confidence": "high", "evidence": evidenceRel, "risk": "blocked", "next_action": "main-agent-resolve-conflict", "tier_used": "light", "tool_scope": "read-only", "feature": "review", "request_id": "n/a", "candidate_path": "n/a", "defer_reason": "conflict",
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
	if err := Run([]string{"-Command", "plan-subagents", "-PacketPath", plan.PacketPath, "-ReviewerResultPath", resultPath, "-Lane", packet.TargetLane, "-Actor", "mission-commander", "-Apply", "-Format", "json"}, &out); err != nil {
		t.Fatal(err)
	}
	blocked := decodeReviewerIntakeResult(t, out.Bytes())
	if blocked.CaseRoot != caseRoot || blocked.Pack != "_template" || blocked.WritebackStatus != "blocked" || blocked.IsMutation || blocked.Applied || blocked.ReadyForWriteback || blocked.Verification != nil || blocked.Decision != nil || blocked.PostValidation == nil || !blocked.PostValidation.Valid {
		t.Fatalf("unexpected nested blocked reviewer intake: %+v", blocked)
	}
	if !containsStringWith(blocked.BlockedReasons, "unresolved conflicts: overlaps another shard") {
		t.Fatalf("blocked reviewer intake omitted conflict reason: %+v", blocked.BlockedReasons)
	}
	if !containsRepairGuidance(blocked.RepairGuidance, "resolve or split", "overlaps another shard", "do not apply reviewer intake until this blocker is resolved") {
		t.Fatalf("blocked reviewer intake omitted repair guidance: %+v", blocked.RepairGuidance)
	}
	if blocked.Summary.Status != "blocked" || blocked.Summary.ReadyForWriteback || blocked.Summary.BlockedCount == 0 || blocked.Summary.RepairGuidanceCount == 0 || blocked.Summary.RepairGuidanceSummary == nil || blocked.Summary.RepairGuidanceSummary.Total != 1 || !strings.Contains(blocked.Summary.RepairGuidanceSummary.PrimaryAction, "resolve or split") || !containsStringWith(blocked.Summary.RepairGuidanceSummary.Evidence, "overlaps another shard") || !containsStringWith(blocked.Summary.RepairGuidanceSummary.Boundary, "do not apply reviewer intake until this blocker is resolved") || !strings.Contains(blocked.Summary.RepairGuidanceSummary.NextSafeCommand, "-WhatIf") || blocked.Summary.CurrentAction == nil || !blocked.Summary.CurrentAction.Blocked || !containsStringWith(blocked.Summary.Boundary, "do not apply reviewer intake while blockedReasons") {
		t.Fatalf("blocked reviewer intake compact summary omitted repair state: %+v", blocked.Summary)
	}
	if blocked.MissionCommanderAction.State != "reviewer-intake-blocked" || !strings.Contains(blocked.MissionCommanderAction.PrimaryCommand, `-Target "`+caseRoot+`"`) || !strings.Contains(blocked.MissionCommanderAction.PrimaryCommand, `-Pack "_template"`) || !containsMissionCommanderNextAction(blocked.MissionCommanderNextActions, "reviewerIntake.blocked", blocked.MissionCommanderAction.PrimaryCommand, true, true) || !nextActionReasonContains(blocked.MissionCommanderNextActions, "repair: resolve or split") {
		t.Fatalf("blocked reviewer intake omitted Mission Commander repair guidance: action=%+v next=%+v", blocked.MissionCommanderAction, blocked.MissionCommanderNextActions)
	}
	assertCLIActionQueue(t, blocked.MissionCommanderActionQueue, 1, 0, 1, 1, 0, blocked.MissionCommanderAction.PrimaryCommand)
	if got := readCLIOptionalFile(t, filepath.Join(caseRoot, ".rekit", "facts", "verifications.jsonl")); got != "" {
		t.Fatalf("blocked reviewer intake wrote verification ledger:\n%s", got)
	}
	if got := readCLIOptionalFile(t, filepath.Join(caseRoot, ".rekit", "facts", "decisions.jsonl")); got != "" {
		t.Fatalf("blocked reviewer intake wrote decision ledger:\n%s", got)
	}

	out.Reset()
	if err := Run([]string{"-Command", "plan-subagents", "-PacketPath", plan.PacketPath, "-ReviewerResultPath", resultPath, "-Lane", packet.TargetLane, "-Actor", "mission-commander", "-WhatIf", "-Format", "text"}, &out); err != nil {
		t.Fatal(err)
	}
	for _, expected := range []string{
		"plan-subagents reviewer intake：status=blocked mutation=false applied=false readyForWriteback=false lane=main shard=shard-01",
		"reviewer intake summary：status=blocked readyForWriteback=false applied=false lane=main shard=shard-01",
		"blocked=1 repairs=1 postValidation=true valid=true",
		"reviewer intake summary repair guidance：total=1 primaryReason=reviewer result reports unresolved conflicts: overlaps another shard primaryAction=resolve or split",
		"reviewer intake summary repair evidence：overlaps another shard",
		"reviewer intake summary repair boundary：do not apply reviewer intake until this blocker is resolved",
		"reviewer intake summary current action：state=reviewer-intake-blocked source=reviewerIntake.blocked blocked=true requiresReview=true command=`/rekit plan-subagents",
		"reviewer intake summary boundary：do not apply reviewer intake while blockedReasons or repairGuidance remain unresolved",
		"reviewer intake blocked reason：reviewer result reports unresolved conflicts: overlaps another shard",
		"reviewer intake repair guidance：reason=reviewer result reports unresolved conflicts: overlaps another shard action=resolve or split",
		"reviewer intake repair evidence：reason=reviewer result reports unresolved conflicts: overlaps another shard evidence=overlaps another shard",
		"reviewer intake repair boundary：reason=reviewer result reports unresolved conflicts: overlaps another shard boundary=do not apply reviewer intake until this blocker is resolved",
		"reviewer intake commander action：state=reviewer-intake-blocked",
		"mission commander action queue：summary=total=1 unblocked=0 blocked=1 requiresReview=1 followUp=0",
		"mission commander action queue current：state=reviewer-intake-blocked source=reviewerIntake.blocked blocked=true requiresReview=true command=`/rekit plan-subagents",
		"mission commander next action：state=reviewer-intake-blocked source=reviewerIntake.blocked blocked=true requiresReview=true command=`/rekit plan-subagents",
		"mission commander next action reason：repair: resolve or split",
	} {
		if !strings.Contains(out.String(), expected) {
			t.Fatalf("nested blocked reviewer intake text missing %q:\n%s", expected, out.String())
		}
	}
	if strings.Contains(out.String(), "{\n  ") {
		t.Fatalf("nested blocked reviewer intake text should not emit JSON object:\n%s", out.String())
	}
	if got := readCLIOptionalFile(t, filepath.Join(caseRoot, ".rekit", "facts", "verifications.jsonl")); got != "" {
		t.Fatalf("blocked reviewer intake text wrote verification ledger:\n%s", got)
	}
	if got := readCLIOptionalFile(t, filepath.Join(caseRoot, ".rekit", "facts", "decisions.jsonl")); got != "" {
		t.Fatalf("blocked reviewer intake text wrote decision ledger:\n%s", got)
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

func TestRunPlanSubagentsReadyReviewerResultsRejectsPlanningScopeFlags(t *testing.T) {
	caseRoot := fullAttachedCase(t)
	var out bytes.Buffer
	err := Run([]string{"-Command", "plan-subagents", "-Target", caseRoot, "-Pack", "_template", "-PacketPath", "packet.json", "-ReadyReviewerResults", "-Lane", "main", "-Actor", "mission-commander", "-Items", "alpha", "-Apply"}, &out)
	if err == nil || !strings.Contains(err.Error(), "intake scope is fixed by -PacketPath") {
		t.Fatalf("error = %v, want reviewer batch planning scope guard", err)
	}
}

func TestRunPlanSubagentsReadyReviewerResultsEmitsStrictErrorEnvelope(t *testing.T) {
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
	handoff := plan.ShardHandoffs[0]
	malformed := []byte("{malformed")
	if err := os.WriteFile(handoff.ReviewerResultCandidatePath, malformed, 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(handoff.ReviewerResultPath, malformed, 0o644); err != nil {
		t.Fatal(err)
	}
	out.Reset()
	err := Run([]string{"-Command", "plan-subagents", "-Target", caseRoot, "-Pack", "_template", "-PacketPath", plan.PacketPath, "-ReadyReviewerResults", "-Lane", "feature-review", "-Actor", "mission-commander", "-Apply", "-Format", "json"}, &out)
	if err == nil || !strings.Contains(err.Error(), "reviewer result must contain exactly one JSON object") {
		t.Fatalf("error = %v, want strict reviewer result failure", err)
	}
	result := decodeReviewerBatchIntakeResult(t, out.Bytes())
	if !result.Stopped || !result.Partial || result.StopShardID != "shard-01" || result.StopReason == "" || result.RecoveryAction == nil || result.RecoveryAction.Source != "reviewerBatchIntake.recovery" || result.RecoveryAction.ActionID != "reviewer-batch-intake-stop-shard" || !result.RecoveryAction.Blocked || !result.RecoveryAction.RequiresReview || !containsMissionCommanderNextAction(result.MissionCommanderNextActions, "reviewerBatchIntake.recovery", result.RecoveryAction.Command, true, true) {
		t.Fatalf("strict reviewer batch error omitted recovery envelope: %+v", result)
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

	out.Reset()
	err = Run([]string{"-Command", "plan-subagents", "-Target", caseRoot, "-Pack", "_template", "-PacketPath", plan.PacketPath, "-ReviewerResultPath", resultPath, "-Lane", packet.TargetLane, "-Actor", "mission-commander", "-Apply", "-Format", "text"}, &out)
	if err == nil || !strings.Contains(err.Error(), "verification-recorded") {
		t.Fatalf("error = %v, want partial recovery text diagnostic", err)
	}
	for _, expected := range []string{
		"plan-subagents reviewer intake：status=verification-recorded",
		"reviewer intake writeback checkpoint：status=verification-recorded verificationApplied=true verificationEventId=evt-review-test-verification decisionApplied=false decisionEventId=evt-review-test-decision",
		"reviewer intake verification：applied=true eventId=evt-review-test-verification",
		"reviewer intake decision：applied=false eventId=evt-review-test-decision",
		"reviewer intake commander action：state=reviewer-intake-partial-writeback",
		"mission commander next action：state=reviewer-intake-partial-writeback source=reviewerIntake.verification-recorded blocked=false requiresReview=true command=`/rekit plan-subagents",
	} {
		if !strings.Contains(out.String(), expected) {
			t.Fatalf("reviewer intake partial recovery text missing %q:\n%s", expected, out.String())
		}
	}
}

type reviewerResultSourceCaptureCLIResult struct {
	Mode                   string                         `json:"mode"`
	IsMutation             bool                           `json:"isMutation"`
	Applied                bool                           `json:"applied"`
	Status                 string                         `json:"status"`
	InputPath              string                         `json:"inputPath"`
	InputSHA256            string                         `json:"inputSha256"`
	InputBytes             int                            `json:"inputBytes"`
	SourcePath             string                         `json:"sourcePath"`
	SourceSHA256           string                         `json:"sourceSha256"`
	SourceBytes            int                            `json:"sourceBytes"`
	AlreadyCaptured        bool                           `json:"alreadyCaptured"`
	MissionCommanderAction missionCommanderActionSnapshot `json:"missionCommanderAction"`
	RunbookSteps           []string                       `json:"runbookSteps"`
}

func decodeReviewerResultSourceCaptureCLIResult(t *testing.T, data []byte) reviewerResultSourceCaptureCLIResult {
	t.Helper()
	var result reviewerResultSourceCaptureCLIResult
	if err := json.Unmarshal(data, &result); err != nil {
		t.Fatalf("reviewer result source capture stdout is not JSON: %v\n%s", err, string(data))
	}
	return result
}

type reviewerResultStagingCLIResult struct {
	Mode                   string                         `json:"mode"`
	IsMutation             bool                           `json:"isMutation"`
	Applied                bool                           `json:"applied"`
	Status                 string                         `json:"status"`
	SourcePath             string                         `json:"sourcePath"`
	SourceSHA256           string                         `json:"sourceSha256"`
	SourceBytes            int                            `json:"sourceBytes"`
	CandidatePath          string                         `json:"candidatePath"`
	AlreadyStaged          bool                           `json:"alreadyStaged"`
	MissionCommanderAction missionCommanderActionSnapshot `json:"missionCommanderAction"`
	RunbookSteps           []string                       `json:"runbookSteps"`
}

func decodeReviewerResultStagingCLIResult(t *testing.T, data []byte) reviewerResultStagingCLIResult {
	t.Helper()
	var result reviewerResultStagingCLIResult
	if err := json.Unmarshal(data, &result); err != nil {
		t.Fatalf("reviewer result staging stdout is not JSON: %v\n%s", err, string(data))
	}
	return result
}

type reviewerResultCollectionCLIResult struct {
	Mode                   string                         `json:"mode"`
	IsMutation             bool                           `json:"isMutation"`
	Applied                bool                           `json:"applied"`
	Status                 string                         `json:"status"`
	CandidatePath          string                         `json:"candidatePath"`
	CandidateSHA256        string                         `json:"candidateSha256"`
	ReviewerResultPath     string                         `json:"reviewerResultPath"`
	ReviewerResultSHA256   string                         `json:"reviewerResultSha256"`
	AlreadyCollected       bool                           `json:"alreadyCollected"`
	MissionCommanderAction missionCommanderActionSnapshot `json:"missionCommanderAction"`
	RunbookSteps           []string                       `json:"runbookSteps"`
}

func decodeReviewerResultCollectionCLIResult(t *testing.T, data []byte) reviewerResultCollectionCLIResult {
	t.Helper()
	var result reviewerResultCollectionCLIResult
	if err := json.Unmarshal(data, &result); err != nil {
		t.Fatalf("reviewer result collection stdout is not JSON: %v\n%s", err, string(data))
	}
	return result
}

func recordReviewerSessionReceiptsForCLIPlan(t *testing.T, out *bytes.Buffer, baseArgs []string, packetPath string, handoff planSubagentsHandoff, lane, actor, harness string, data []byte) {
	t.Helper()
	inputPath := handoff.ReviewerStagingCommands.SourceCaptureInput
	if err := os.MkdirAll(filepath.Dir(inputPath), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(inputPath, data, 0o644); err != nil {
		t.Fatal(err)
	}
	var reviewerResult struct {
		ReviewerSession string `json:"reviewerSession"`
	}
	if err := json.Unmarshal(data, &reviewerResult); err != nil || strings.TrimSpace(reviewerResult.ReviewerSession) == "" {
		t.Fatalf("reviewer result lacks session identity: %v", err)
	}
	out.Reset()
	if err := Run(planSubagentsCLIArgs(baseArgs, "-PacketPath", packetPath, "-RecordReviewerDispatch", "-ShardId", handoff.ShardID, "-ReviewerHarness", harness, "-ReviewerSession", reviewerResult.ReviewerSession, "-Lane", lane, "-Actor", actor, "-WhatIf", "-Format", "json"), out); err != nil {
		t.Fatal(err)
	}
	var dispatchPreview subagents.ReviewerSessionReceiptResult
	if err := json.Unmarshal(out.Bytes(), &dispatchPreview); err != nil {
		t.Fatal(err)
	}
	out.Reset()
	if err := Run(planSubagentsCLIArgs(baseArgs, "-PacketPath", packetPath, "-RecordReviewerDispatch", "-ShardId", handoff.ShardID, "-ReviewerHarness", harness, "-ReviewerSession", reviewerResult.ReviewerSession, "-Lane", lane, "-Actor", actor, "-ExpectedReviewerDispatchBindingSha256", dispatchPreview.BindingSHA256, "-Apply", "-Format", "json"), out); err != nil {
		t.Fatal(err)
	}
	var dispatch subagents.ReviewerSessionReceiptResult
	if err := json.Unmarshal(out.Bytes(), &dispatch); err != nil {
		t.Fatal(err)
	}
	out.Reset()
	if err := Run(planSubagentsCLIArgs(baseArgs, "-PacketPath", packetPath, "-RecordReviewerCompletion", "-ReviewerDispatchId", dispatch.DispatchID, "-ReviewerOutcome", "succeeded", "-ReviewerExitStatus", "completed", "-ReviewerResultInputPath", inputPath, "-Lane", lane, "-Actor", actor, "-WhatIf", "-Format", "json"), out); err != nil {
		t.Fatal(err)
	}
	var completionPreview subagents.ReviewerSessionReceiptResult
	if err := json.Unmarshal(out.Bytes(), &completionPreview); err != nil {
		t.Fatal(err)
	}
	out.Reset()
	if err := Run(planSubagentsCLIArgs(baseArgs, "-PacketPath", packetPath, "-RecordReviewerCompletion", "-ReviewerDispatchId", dispatch.DispatchID, "-ReviewerOutcome", "succeeded", "-ReviewerExitStatus", "completed", "-ReviewerResultInputPath", inputPath, "-Lane", lane, "-Actor", actor, "-ExpectedReviewerDispatchReceiptSha256", completionPreview.DispatchReceiptSHA256, "-ExpectedReviewerResultInputSha256", completionPreview.ReviewerResultInputSHA256, "-Apply", "-Format", "json"), out); err != nil {
		t.Fatal(err)
	}
}

func captureReviewerResultSourceForCLIPlan(t *testing.T, out *bytes.Buffer, baseArgs []string, packetPath string, handoff planSubagentsHandoff, lane, actor string, data []byte) reviewerResultSourceCaptureCLIResult {
	t.Helper()
	if handoff.ReviewerStagingCommands == nil || handoff.ReviewerStagingCommands.SourcePath == "" || handoff.ReviewerStagingCommands.SourcePathArgument != handoff.ReviewerStagingCommands.SourcePath {
		t.Fatalf("reviewer staging source path was not packet-derived: %+v", handoff.ReviewerStagingCommands)
	}
	sourcePath := handoff.ReviewerStagingCommands.SourcePath
	inputPath := handoff.ReviewerStagingCommands.SourceCaptureInput
	if inputPath == "" || inputPath != filepath.Join(filepath.Dir(sourcePath), "..", "inputs", handoff.ShardID+".reviewer-input.json") || !strings.Contains(handoff.ReviewerStagingCommands.SourceCaptureCommand, "-ReviewerResultInputPath") || !strings.Contains(handoff.ReviewerStagingCommands.SourceCaptureCommand, inputPath) || !strings.Contains(handoff.ReviewerStagingCommands.SourceCaptureApply, "-ExpectedReviewerResultInputSha256") {
		t.Fatalf("reviewer input drop path was not packet-derived: %+v", handoff.ReviewerStagingCommands)
	}
	recordReviewerSessionReceiptsForCLIPlan(t, out, baseArgs, packetPath, handoff, lane, actor, "go-cli-test-harness", data)

	out.Reset()
	if err := Run(planSubagentsCLIArgs(baseArgs, "-PacketPath", packetPath, "-CaptureReviewerResultSource", "-ShardId", handoff.ShardID, "-ReviewerResultInputPath", inputPath, "-Lane", lane, "-Actor", actor, "-WhatIf", "-Format", "json"), out); err != nil {
		t.Fatal(err)
	}
	capturePreview := decodeReviewerResultSourceCaptureCLIResult(t, out.Bytes())
	if capturePreview.Mode != "reviewer-result-source-capture" || capturePreview.IsMutation || capturePreview.Applied || capturePreview.InputPath != inputPath || capturePreview.InputSHA256 == "" || capturePreview.SourcePath != sourcePath || capturePreview.SourceSHA256 != capturePreview.InputSHA256 {
		t.Fatalf("unexpected reviewer result source capture preview: %+v", capturePreview)
	}
	switch capturePreview.Status {
	case "previewed":
		if capturePreview.MissionCommanderAction.State != "ready-for-reviewer-result-source-capture-apply" || !strings.Contains(capturePreview.MissionCommanderAction.PrimaryCommand, "-ExpectedReviewerResultInputSha256") {
			t.Fatalf("previewed reviewer source capture omitted apply handoff: %+v", capturePreview)
		}
	case "already-captured":
		if capturePreview.MissionCommanderAction.State != "reviewer-result-source-ready-for-staging-preview" || !strings.Contains(capturePreview.MissionCommanderAction.PrimaryCommand, "-StageReviewerResult") || strings.Contains(capturePreview.MissionCommanderAction.PrimaryCommand, "-ExpectedReviewerResultInputSha256") {
			t.Fatalf("already-captured reviewer source capture omitted staging handoff: %+v", capturePreview)
		}
	default:
		t.Fatalf("unexpected reviewer result source capture preview status: %+v", capturePreview)
	}
	assertStringContains(t, capturePreview.RunbookSteps, "run current Mission Commander command")
	assertStringContains(t, capturePreview.RunbookSteps, "hash-bound Apply command")
	out.Reset()
	if err := Run(planSubagentsCLIArgs(baseArgs, "-PacketPath", packetPath, "-CaptureReviewerResultSource", "-ShardId", handoff.ShardID, "-ReviewerResultInputPath", inputPath, "-Lane", lane, "-Actor", actor, "-WhatIf", "-Format", "text"), out); err != nil {
		t.Fatal(err)
	}
	expectedCaptureRunbook := []string{
		"reviewer result source capture runbook：confirm reviewer result source capture status=" + capturePreview.Status,
		"reviewer result source capture runbook：run current Mission Commander command:",
		"reviewer result source capture runbook：after each Apply, rerun the next WhatIf and use only the returned hash-bound Apply command",
	}
	if capturePreview.Status == "already-captured" {
		expectedCaptureRunbook = append(expectedCaptureRunbook, "-StageReviewerResult")
	} else {
		expectedCaptureRunbook = append(expectedCaptureRunbook, "-ExpectedReviewerResultInputSha256")
	}
	for _, expected := range expectedCaptureRunbook {
		if !strings.Contains(out.String(), expected) {
			t.Fatalf("reviewer result source capture text omitted runbook %q:\n%s", expected, out.String())
		}
	}
	if capturePreview.Status == "already-captured" {
		captured, err := os.ReadFile(sourcePath)
		if err != nil {
			t.Fatal(err)
		}
		if !bytes.Equal(captured, data) {
			t.Fatalf("already captured reviewer result source bytes differ from input")
		}
		return capturePreview
	}
	if capturePreview.Status != "previewed" {
		t.Fatalf("unexpected reviewer result source capture preview status: %+v", capturePreview)
	}
	if _, err := os.Stat(sourcePath); !os.IsNotExist(err) {
		t.Fatalf("reviewer result source capture preview wrote source path: err=%v", err)
	}

	out.Reset()
	if err := Run(reviewerPrimaryCommandCLIArgs(t, baseArgs, capturePreview.MissionCommanderAction.PrimaryCommand), out); err != nil {
		t.Fatal(err)
	}
	captureApply := decodeReviewerResultSourceCaptureCLIResult(t, out.Bytes())
	if captureApply.Status != "captured" || !captureApply.IsMutation || !captureApply.Applied || captureApply.AlreadyCaptured || captureApply.InputPath != inputPath || captureApply.SourcePath != sourcePath || captureApply.SourceSHA256 != capturePreview.InputSHA256 || captureApply.MissionCommanderAction.State != "reviewer-result-source-ready-for-staging-preview" || !strings.Contains(captureApply.MissionCommanderAction.PrimaryCommand, "-StageReviewerResult") || strings.Contains(captureApply.MissionCommanderAction.PrimaryCommand, "-ExpectedReviewerResultInputSha256") {
		t.Fatalf("unexpected reviewer result source capture apply: %+v", captureApply)
	}
	captured, err := os.ReadFile(sourcePath)
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(captured, data) {
		t.Fatalf("captured reviewer result source bytes changed")
	}
	return captureApply
}

func stageAndCollectReviewerResultForCLIPlan(t *testing.T, out *bytes.Buffer, baseArgs []string, packetPath string, handoff planSubagentsHandoff, lane, actor string, data []byte) reviewerResultCollectionCLIResult {
	t.Helper()
	if handoff.ReviewerResultCandidatePath == "" || handoff.ReviewerResultCandidatePath == handoff.ReviewerResultPath {
		t.Fatalf("reviewer collection candidate path is not distinct: %+v", handoff)
	}
	sourcePath := handoff.ReviewerStagingCommands.SourcePath
	captureApply := captureReviewerResultSourceForCLIPlan(t, out, baseArgs, packetPath, handoff, lane, actor, data)

	out.Reset()
	if err := Run(reviewerPrimaryCommandCLIArgs(t, baseArgs, captureApply.MissionCommanderAction.PrimaryCommand), out); err != nil {
		t.Fatal(err)
	}
	stagingPreview := decodeReviewerResultStagingCLIResult(t, out.Bytes())
	if stagingPreview.Mode != "reviewer-result-staging" || stagingPreview.Status != "previewed" || stagingPreview.IsMutation || stagingPreview.Applied || stagingPreview.SourcePath != sourcePath || stagingPreview.SourceSHA256 == "" || stagingPreview.SourceSHA256 != captureApply.SourceSHA256 || stagingPreview.CandidatePath != handoff.ReviewerResultCandidatePath || stagingPreview.MissionCommanderAction.State != "ready-for-reviewer-result-staging-apply" || !strings.Contains(stagingPreview.MissionCommanderAction.PrimaryCommand, "-ExpectedSourceSha256") {
		t.Fatalf("unexpected reviewer result staging preview: %+v", stagingPreview)
	}
	assertStringContains(t, stagingPreview.RunbookSteps, "run current Mission Commander command")
	assertStringContains(t, stagingPreview.RunbookSteps, "hash-bound Apply command")
	out.Reset()
	if err := Run(planSubagentsCLIArgs(baseArgs, "-PacketPath", packetPath, "-StageReviewerResult", "-ShardId", handoff.ShardID, "-ReviewerResultSourcePath", sourcePath, "-Lane", lane, "-Actor", actor, "-WhatIf", "-Format", "text"), out); err != nil {
		t.Fatal(err)
	}
	for _, expected := range []string{
		"reviewer result staging runbook：confirm reviewer result staging status=previewed",
		"reviewer result staging runbook：run current Mission Commander command:",
		"-ExpectedSourceSha256",
		"reviewer result staging runbook：after each Apply, rerun the next WhatIf and use only the returned hash-bound Apply command",
	} {
		if !strings.Contains(out.String(), expected) {
			t.Fatalf("reviewer result staging text omitted runbook %q:\n%s", expected, out.String())
		}
	}

	out.Reset()
	if err := Run(reviewerPrimaryCommandCLIArgs(t, baseArgs, stagingPreview.MissionCommanderAction.PrimaryCommand), out); err != nil {
		t.Fatal(err)
	}
	stagingApply := decodeReviewerResultStagingCLIResult(t, out.Bytes())
	if stagingApply.Status != "staged" || !stagingApply.IsMutation || !stagingApply.Applied || stagingApply.AlreadyStaged || stagingApply.CandidatePath != handoff.ReviewerResultCandidatePath || stagingApply.MissionCommanderAction.State != "reviewer-result-staged-ready-for-collection-preview" || !strings.Contains(stagingApply.MissionCommanderAction.PrimaryCommand, "-CollectReviewerResult") || strings.Contains(stagingApply.MissionCommanderAction.PrimaryCommand, "-ExpectedSourceSha256") {
		t.Fatalf("unexpected reviewer result staging apply: %+v", stagingApply)
	}
	assertStringContains(t, stagingApply.RunbookSteps, "-CollectReviewerResult")
	assertStringContains(t, stagingApply.RunbookSteps, "separate bounded operations")

	out.Reset()
	if err := Run(reviewerPrimaryCommandCLIArgs(t, baseArgs, stagingApply.MissionCommanderAction.PrimaryCommand), out); err != nil {
		t.Fatal(err)
	}
	collectionPreview := decodeReviewerResultCollectionCLIResult(t, out.Bytes())
	if collectionPreview.Mode != "reviewer-result-collection" || collectionPreview.Status != "previewed" || collectionPreview.IsMutation || collectionPreview.Applied || collectionPreview.CandidatePath != handoff.ReviewerResultCandidatePath || collectionPreview.ReviewerResultPath != handoff.ReviewerResultPath || collectionPreview.MissionCommanderAction.State != "ready-for-reviewer-result-collection-apply" || !strings.Contains(collectionPreview.MissionCommanderAction.PrimaryCommand, "-Apply") {
		t.Fatalf("unexpected reviewer result collection preview: %+v", collectionPreview)
	}
	assertStringContains(t, collectionPreview.RunbookSteps, "run current Mission Commander command")
	assertStringContains(t, collectionPreview.RunbookSteps, "hash-bound Apply command")
	out.Reset()
	if err := Run(planSubagentsCLIArgs(baseArgs, "-PacketPath", packetPath, "-CollectReviewerResult", "-ShardId", handoff.ShardID, "-Lane", lane, "-Actor", actor, "-WhatIf", "-Format", "text"), out); err != nil {
		t.Fatal(err)
	}
	for _, expected := range []string{
		"reviewer result collection runbook：confirm reviewer result collection status=previewed",
		"reviewer result collection runbook：run current Mission Commander command:",
		"-CollectReviewerResult",
		"reviewer result collection runbook：after each Apply, rerun the next WhatIf and use only the returned hash-bound Apply command",
	} {
		if !strings.Contains(out.String(), expected) {
			t.Fatalf("reviewer result collection text omitted runbook %q:\n%s", expected, out.String())
		}
	}

	out.Reset()
	if err := Run(reviewerPrimaryCommandCLIArgs(t, baseArgs, collectionPreview.MissionCommanderAction.PrimaryCommand), out); err != nil {
		t.Fatal(err)
	}
	collectionApply := decodeReviewerResultCollectionCLIResult(t, out.Bytes())
	if collectionApply.Status != "collected" || !collectionApply.IsMutation || !collectionApply.Applied || collectionApply.AlreadyCollected || collectionApply.CandidatePath != handoff.ReviewerResultCandidatePath || collectionApply.ReviewerResultPath != handoff.ReviewerResultPath || collectionApply.CandidateSHA256 == "" || collectionApply.CandidateSHA256 != collectionApply.ReviewerResultSHA256 || collectionApply.MissionCommanderAction.State != "reviewer-result-collected-ready-for-batch-intake-preview" || !strings.Contains(collectionApply.MissionCommanderAction.PrimaryCommand, "-ReadyReviewerResults") {
		t.Fatalf("unexpected reviewer result collection apply: %+v", collectionApply)
	}
	assertStringContains(t, collectionApply.RunbookSteps, "ready reviewer results intake")
	if _, err := os.Stat(handoff.ReviewerResultPath); err != nil {
		t.Fatal(err)
	}
	return collectionApply
}

func reviewerPrimaryCommandCLIArgs(t *testing.T, baseArgs []string, command string) []string {
	t.Helper()
	fields := splitReviewerPrimaryCommand(t, command)
	if len(fields) < 2 || fields[0] != "/rekit" || fields[1] != "plan-subagents" {
		t.Fatalf("unexpected reviewer primary command: %q", command)
	}
	args := append([]string{"-Command", fields[1]}, baseArgs...)
	return append(args, fields[2:]...)
}

func splitReviewerPrimaryCommand(t *testing.T, command string) []string {
	t.Helper()
	command = strings.TrimSpace(command)
	if command == "" {
		t.Fatal("empty reviewer primary command")
	}
	fields := []string{}
	var current strings.Builder
	inQuote := false
	inField := false
	for i := 0; i < len(command); i++ {
		ch := command[i]
		if inQuote {
			inField = true
			if ch == '\\' && i+1 < len(command) && command[i+1] == '"' {
				current.WriteByte('"')
				i++
				continue
			}
			if ch == '"' {
				inQuote = false
				continue
			}
			current.WriteByte(ch)
			continue
		}
		switch ch {
		case ' ', '\t', '\n', '\r':
			if inField {
				fields = append(fields, current.String())
				current.Reset()
				inField = false
			}
		case '"':
			inQuote = true
			inField = true
		default:
			inField = true
			current.WriteByte(ch)
		}
	}
	if inQuote {
		t.Fatalf("unterminated quote in reviewer primary command: %q", command)
	}
	if inField {
		fields = append(fields, current.String())
	}
	return fields
}

func planSubagentsCLIArgs(baseArgs []string, extra ...string) []string {
	args := append([]string{"-Command", "plan-subagents"}, baseArgs...)
	return append(args, extra...)
}

type reviewerBatchIntakeCLIResult struct {
	Command                     string                              `json:"command"`
	Mode                        string                              `json:"mode"`
	CaseRoot                    string                              `json:"caseRoot"`
	Pack                        string                              `json:"pack"`
	IsMutation                  bool                                `json:"isMutation"`
	Applied                     bool                                `json:"applied"`
	Total                       int                                 `json:"total"`
	Ready                       int                                 `json:"ready"`
	Waiting                     int                                 `json:"waiting"`
	Processed                   int                                 `json:"processed"`
	Completed                   int                                 `json:"completed"`
	AlreadyComplete             int                                 `json:"alreadyComplete"`
	Stopped                     bool                                `json:"stopped"`
	StopShardID                 string                              `json:"stopShardId"`
	StopReason                  string                              `json:"stopReason"`
	Partial                     bool                                `json:"partial"`
	NextOpenShardID             string                              `json:"nextOpenShardId"`
	RemainingShardIDs           []string                            `json:"remainingShardIds"`
	RerunCommand                string                              `json:"rerunCommand"`
	RecoveryAction              *missionCommanderNextActionItem     `json:"recoveryAction"`
	Results                     []reviewerIntakeCLIResult           `json:"results"`
	MissionCommanderAction      missionCommanderActionSnapshot      `json:"missionCommanderAction"`
	MissionCommanderNextActions []missionCommanderNextActionItem    `json:"missionCommanderNextActions"`
	MissionCommanderActionQueue missionCommanderActionQueueSnapshot `json:"missionCommanderActionQueue"`
}

func decodeReviewerBatchIntakeResult(t *testing.T, data []byte) reviewerBatchIntakeCLIResult {
	t.Helper()
	var result reviewerBatchIntakeCLIResult
	if err := json.Unmarshal(data, &result); err != nil {
		t.Fatalf("reviewer batch intake stdout is not JSON: %v\n%s", err, string(data))
	}
	return result
}

type reviewerIntakeCLIResult struct {
	Command               string                                `json:"command"`
	Mode                  string                                `json:"mode"`
	CaseRoot              string                                `json:"caseRoot"`
	Pack                  string                                `json:"pack"`
	Lane                  string                                `json:"lane"`
	IsMutation            bool                                  `json:"isMutation"`
	Applied               bool                                  `json:"applied"`
	WritebackStatus       string                                `json:"writebackStatus"`
	ReadyForWriteback     bool                                  `json:"readyForWriteback"`
	BlockedReasons        []string                              `json:"blockedReasons"`
	RepairGuidance        []reviewerIntakeRepairGuidanceCLIItem `json:"repairGuidance"`
	OrchestrationSnapshot struct {
		Mode              string   `json:"mode"`
		ReviewerCount     int      `json:"reviewerCount"`
		DispatchIndex     int      `json:"dispatchIndex"`
		DispatchTotal     int      `json:"dispatchTotal"`
		ShardStatusBefore string   `json:"shardStatusBefore"`
		ShardStatusAfter  string   `json:"shardStatusAfter"`
		NextDispatches    []string `json:"nextDispatches"`
	} `json:"orchestrationSnapshot"`
	Summary struct {
		Status                              string                                   `json:"status"`
		ReadyForWriteback                   bool                                     `json:"readyForWriteback"`
		Applied                             bool                                     `json:"applied"`
		Lane                                string                                   `json:"lane"`
		ShardID                             string                                   `json:"shardId"`
		IntakeID                            string                                   `json:"intakeId"`
		ReviewerSession                     string                                   `json:"reviewerSession"`
		VerificationVerdict                 string                                   `json:"verificationVerdict"`
		MainDecision                        string                                   `json:"mainDecision"`
		DispatchIndex                       int                                      `json:"dispatchIndex"`
		DispatchTotal                       int                                      `json:"dispatchTotal"`
		ShardStatusBefore                   string                                   `json:"shardStatusBefore"`
		ShardStatusAfter                    string                                   `json:"shardStatusAfter"`
		OrchestrationProgress               *reviewerIntakeOrchestrationProgressItem `json:"orchestrationProgress"`
		BlockedCount                        int                                      `json:"blockedCount"`
		RepairGuidanceCount                 int                                      `json:"repairGuidanceCount"`
		RepairGuidanceSummary               *reviewerIntakeRepairGuidanceSummaryItem `json:"repairGuidanceSummary"`
		PostValidationPresent               bool                                     `json:"postValidationPresent"`
		PostValidationValid                 bool                                     `json:"postValidationValid"`
		PostValidationOverviewVerifications int                                      `json:"postValidationOverviewVerifications"`
		PostValidationOverviewDecisions     int                                      `json:"postValidationOverviewDecisions"`
		ReviewerWritebacks                  int                                      `json:"reviewerWritebacks"`
		ReviewerWritebackSummary            *reviewerWritebackSummaryCLIItem         `json:"reviewerWritebackSummary"`
		ActionTotal                         int                                      `json:"actionTotal"`
		ActionUnblocked                     int                                      `json:"actionUnblocked"`
		ActionBlocked                       int                                      `json:"actionBlocked"`
		ActionRequiresReview                int                                      `json:"actionRequiresReview"`
		ActionFollowUp                      int                                      `json:"actionFollowUp"`
		QueueSummary                        string                                   `json:"queueSummary"`
		CurrentAction                       *struct {
			State          string `json:"state"`
			Source         string `json:"source"`
			Command        string `json:"command"`
			Blocked        bool   `json:"blocked"`
			RequiresReview bool   `json:"requiresReview"`
		} `json:"currentAction"`
		NextActions []struct {
			State          string `json:"state"`
			Source         string `json:"source"`
			Command        string `json:"command"`
			Blocked        bool   `json:"blocked"`
			RequiresReview bool   `json:"requiresReview"`
		} `json:"nextActions"`
		Boundary []string `json:"boundary"`
	} `json:"summary"`
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
		Summary struct {
			Valid                    bool                             `json:"valid"`
			OverviewVerifications    int                              `json:"overviewVerifications"`
			OverviewDecisions        int                              `json:"overviewDecisions"`
			DoctorRows               int                              `json:"doctorRows"`
			Lane                     string                           `json:"lane"`
			ExecutorActionPresent    bool                             `json:"executorActionPresent"`
			ExecutorActionReady      bool                             `json:"executorActionReady"`
			ExecutorActionBlocked    bool                             `json:"executorActionBlocked"`
			ExecutorActionState      string                           `json:"executorActionState"`
			ReviewerWritebacks       int                              `json:"reviewerWritebacks"`
			ReviewerWritebackSummary *reviewerWritebackSummaryCLIItem `json:"reviewerWritebackSummary"`
			QueueSummary             string                           `json:"queueSummary"`
			CurrentAction            *struct {
				State          string `json:"state"`
				Source         string `json:"source"`
				Command        string `json:"command"`
				Blocked        bool   `json:"blocked"`
				RequiresReview bool   `json:"requiresReview"`
			} `json:"currentAction"`
			NextActions []struct {
				State          string `json:"state"`
				Source         string `json:"source"`
				Command        string `json:"command"`
				Blocked        bool   `json:"blocked"`
				RequiresReview bool   `json:"requiresReview"`
			} `json:"nextActions"`
			Boundary []string `json:"boundary"`
		} `json:"summary"`
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
			ExecutorAction     map[string]any             `json:"executorAction"`
			ReviewerWritebacks []reviewerWritebackCLIItem `json:"reviewerWritebacks"`
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

type reviewerIntakeRepairGuidanceCLIItem struct {
	Reason   string   `json:"reason"`
	Action   string   `json:"action"`
	Evidence []string `json:"evidence"`
	Boundary []string `json:"boundary"`
}

type reviewerIntakeOrchestrationProgressItem struct {
	DispatchIndex      int      `json:"dispatchIndex"`
	DispatchTotal      int      `json:"dispatchTotal"`
	Completed          int      `json:"completed"`
	Open               int      `json:"open"`
	CurrentShardID     string   `json:"currentShardId"`
	CurrentShardStatus string   `json:"currentShardStatus"`
	NextOpenShardID    string   `json:"nextOpenShardId"`
	RemainingShardIDs  []string `json:"remainingShardIds"`
	Boundary           []string `json:"boundary"`
}

type reviewerIntakeRepairGuidanceSummaryItem struct {
	Total           int      `json:"total"`
	PrimaryReason   string   `json:"primaryReason"`
	PrimaryAction   string   `json:"primaryAction"`
	Evidence        []string `json:"evidence"`
	Boundary        []string `json:"boundary"`
	NextSafeCommand string   `json:"nextSafeCommand"`
}

func containsStringWith(values []string, want string) bool {
	for _, value := range values {
		if strings.Contains(value, want) {
			return true
		}
	}
	return false
}

func containsRepairGuidance(items []reviewerIntakeRepairGuidanceCLIItem, action, evidence, boundary string) bool {
	for _, item := range items {
		if !strings.Contains(item.Action, action) {
			continue
		}
		if !containsStringWith(item.Evidence, evidence) {
			continue
		}
		if !containsStringWith(item.Boundary, boundary) {
			continue
		}
		return true
	}
	return false
}

func nextActionReasonContains(items []missionCommanderNextActionItem, want string) bool {
	for _, item := range items {
		if containsStringWith(item.Reasons, want) {
			return true
		}
	}
	return false
}

func readCLIOptionalFile(t *testing.T, path string) string {
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
