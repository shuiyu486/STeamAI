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
		"reviewer intake post-validation summary current action：state=",
		"reviewer intake post-validation summary next action：state=",
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
	if err := os.WriteFile(resultPath, data, 0o644); err != nil {
		t.Fatal(err)
	}

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
	if err := Run([]string{"-Command", "plan-subagents", "-TaskType", "feature-analysis", "-Items", "alpha"}, &out); err != nil {
		t.Fatal(err)
	}
	plan := decodePlanSubagentsResult(t, out.Bytes())
	packet := decodePlanSubagentsPacket(t, plan.PacketPath)
	resultPath := plan.ShardHandoffs[0].ReviewerResultPath
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
