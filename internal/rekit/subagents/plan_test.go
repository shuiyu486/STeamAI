package subagents

import (
	"encoding/json"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"

	"github.com/shuiyu486/re-context-kits/internal/rekit/defaults"
	"github.com/shuiyu486/re-context-kits/internal/rekit/mission"
	syncreview "github.com/shuiyu486/re-context-kits/internal/rekit/sync"
	"github.com/shuiyu486/re-context-kits/internal/rekit/workstream"
)

func TestWritePlanIncludesShardHandoffs(t *testing.T) {
	repoRoot := filepath.Clean(filepath.Join("..", "..", ".."))
	target := t.TempDir()
	reviewRoot := filepath.Join(t.TempDir(), "review")

	result, err := WritePlan(repoRoot, target, defaults.DefaultPack, Options{TaskType: "feature-analysis", Items: "alpha,beta gamma", ItemsPerAgent: 2, MaxParallel: 5, ReviewOutputDir: reviewRoot})
	if err != nil {
		t.Fatal(err)
	}
	if result.Command != commandName || result.IsMutation || !result.WritesReviewArtifacts || !result.ReviewRequired || result.ItemCount != 3 || result.ShardCount != 2 {
		t.Fatalf("unexpected result: %+v", result)
	}
	if result.TargetLane != "devirt-main" || result.OwnerBinding.TargetLane != "devirt-main" || result.OwnerBinding.BindingMode != "out-of-case-dispatch-only" || result.OwnerBinding.RequiredForIntake {
		t.Fatalf("unexpected out-of-case owner binding: %+v", result.OwnerBinding)
	}
	if len(result.ShardHandoffs) != 2 {
		t.Fatalf("ShardHandoffs = %+v, want 2", result.ShardHandoffs)
	}
	if result.ReviewerOrchestration.Mode != "dispatch-only-unattached-target" || result.ReviewerOrchestration.ReviewerCount != 2 || result.ReviewerOrchestration.MaxParallel != 5 || len(result.ReviewerOrchestration.Dispatches) != 2 || len(result.ReviewerOrchestration.Lifecycle) != 5 || result.ReviewerOrchestration.Dispatches[0].ShardID != "shard-01" || !strings.Contains(result.ReviewerOrchestration.Lifecycle[0].Action, "does not spawn") {
		t.Fatalf("unexpected reviewer orchestration: %+v", result.ReviewerOrchestration)
	}
	if result.MissionCommanderAction.State != "reviewer-dispatch-only-target-unattached" || result.ReviewerOrchestration.MissionCommanderAction == nil || result.ReviewerOrchestration.MissionCommanderAction.State != result.MissionCommanderAction.State || !hasPlanCommanderNextAction(result.MissionCommanderNextActions, "reviewerOrchestration.dispatch", "dispatch read-only reviewer for shard-01", false, true) || !hasPlanCommanderNextAction(result.MissionCommanderNextActions, "reviewerOrchestration.dispatchOnly.attachTarget", "/rekit init", false, true) {
		t.Fatalf("out-of-case plan omitted Mission Commander dispatch-only guidance: action=%+v next=%+v orchestration=%+v", result.MissionCommanderAction, result.MissionCommanderNextActions, result.ReviewerOrchestration)
	}
	if len(result.ReviewerOrchestration.MissionCommanderNextActions) != len(result.MissionCommanderNextActions) {
		t.Fatalf("orchestration did not mirror top-level Mission Commander next actions: result=%+v orchestration=%+v", result.MissionCommanderNextActions, result.ReviewerOrchestration.MissionCommanderNextActions)
	}
	if !slices.Contains(result.ReviewerOrchestration.Lifecycle[2].MustPass, "do not expect readyForWriteback or postValidation until the target is an attached rekit case") || slices.Contains(result.ReviewerOrchestration.Lifecycle[2].MustPass, "isMutation=false") || !slices.Contains(result.ReviewerOrchestration.Lifecycle[3].MustPass, "no verification or decision ledger events are expected for dispatch-only artifacts") || slices.Contains(result.ReviewerOrchestration.Lifecycle[3].MustPass, "verification event precedes linked decision event") {
		t.Fatalf("out-of-case lifecycle advertised runnable intake gates: %+v", result.ReviewerOrchestration.Lifecycle)
	}
	assertShardHandoff(t, result.ShardHandoffs[0], "shard-01", []string{"alpha", "beta"})

	packetBytes, err := os.ReadFile(result.PacketPath)
	if err != nil {
		t.Fatal(err)
	}
	var packet Packet
	if err := json.Unmarshal(packetBytes, &packet); err != nil {
		t.Fatalf("packet JSON did not decode: %v\n%s", err, string(packetBytes))
	}
	if packet.Command != commandName || packet.PacketID == "" || packet.PacketID != packetIdentity(packet) || packet.Route.ID != defaults.DefaultPack+":lane-feature-analysis" || len(packet.ShardHandoffs) != 2 {
		t.Fatalf("unexpected packet: %+v", packet)
	}
	if packet.OwnerBinding != result.OwnerBinding || packet.ShardHandoffs[0].OwnerBinding != result.OwnerBinding {
		t.Fatalf("packet did not preserve owner binding: result=%+v packet=%+v", result.OwnerBinding, packet.OwnerBinding)
	}
	if packet.ReviewerOrchestration.Mode != result.ReviewerOrchestration.Mode || packet.ReviewerOrchestration.PacketPath != result.PacketPath || packet.ReviewerOrchestration.Dispatches[0].ReviewerResultPath != packet.ShardHandoffs[0].ReviewerResultPath || packet.ReviewerOrchestration.MissionCommanderAction == nil || len(packet.ReviewerOrchestration.MissionCommanderNextActions) != len(result.MissionCommanderNextActions) {
		t.Fatalf("packet did not preserve reviewer orchestration: result=%+v packet=%+v", result.ReviewerOrchestration, packet.ReviewerOrchestration)
	}
	assertShardHandoff(t, packet.ShardHandoffs[0], "shard-01", []string{"alpha", "beta"})

	summary, err := os.ReadFile(result.SummaryPath)
	if err != nil {
		t.Fatal(err)
	}
	for _, expected := range []string{"### reviewer orchestration", "mission commander action:", "mission commander next action:", "orchestration-step:", "reviewer-dispatch:", "### shard handoff prompts", "read-only reviewer", "Do not write files", "reviewer result root:", "main-agent result path=", "expected output=`item,decision", "reviewer result contract", "evidence-rule:", "conflict-signal:", "intake-check:", "decision-map:", "conflict-handling:", "writeback-step:", "command-binding:", "writeback-blocker:", "reviewer intake preview", "n/a: reviewer intake requires an attached rekit case", "out-of-case review artifacts are dispatch-only", "preview-check:", "post-review:"} {
		if !strings.Contains(string(summary), expected) {
			t.Fatalf("summary missing %q:\n%s", expected, string(summary))
		}
	}
}

func TestPacketIdentityMatchesLegacyPacketWithoutReviewerOrchestration(t *testing.T) {
	repoRoot := filepath.Clean(filepath.Join("..", "..", ".."))
	result, err := WritePlan(repoRoot, t.TempDir(), defaults.DefaultPack, Options{TaskType: "feature-analysis", Items: "alpha", ReviewOutputDir: t.TempDir()})
	if err != nil {
		t.Fatal(err)
	}
	packet := readPlanPacket(t, result.PacketPath)
	packet.PacketID = legacyPacketIdentity(packet)
	packet.ReviewerOrchestration = ReviewerOrchestrationPlan{}
	if !packetIdentityMatches(packet) {
		t.Fatalf("legacy packet identity without reviewerOrchestration should remain intake-compatible: %+v", packet)
	}
	packet.ReviewerOrchestration.Mode = "tampered"
	if packetIdentityMatches(packet) {
		t.Fatalf("legacy packet identity should not cover non-empty reviewerOrchestration tampering: %+v", packet.ReviewerOrchestration)
	}
}

func TestWritePlanBindsAttachedCaseLaneExecutor(t *testing.T) {
	repoRoot := filepath.Clean(filepath.Join("..", "..", ".."))
	caseRoot := filepath.Join(t.TempDir(), "case")
	root, err := filepath.Abs(repoRoot)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := syncreview.Apply(root, caseRoot, defaults.DefaultPack, syncreview.ApplyOptions{ProjectName: "plan-owner-binding-test", CreateLocalFiles: true, Command: "init"}); err != nil {
		t.Fatal(err)
	}
	if _, err := workstream.StartApply(root, caseRoot, defaults.DefaultPack, workstream.StartOptions{Name: "intake", Executor: "session-plan", Actor: "mission-commander", TakeoverReason: "plan owner binding fixture"}); err != nil {
		t.Fatal(err)
	}
	result, err := WritePlan(repoRoot, caseRoot, defaults.DefaultPack, Options{TaskType: "feature-analysis", Items: "alpha", Lane: "feature-intake"})
	if err != nil {
		t.Fatal(err)
	}
	if result.TargetLane != "feature-intake" || result.OwnerBinding.CurrentExecutor != "session-plan" || result.OwnerBinding.ExecutorGeneration != 1 || !result.OwnerBinding.RequiredForIntake || result.OwnerBinding.BindingMode != "current-executor-generation" || result.OwnerBinding.LastTakeoverBy != "mission-commander" {
		t.Fatalf("unexpected attached case owner binding: %+v", result.OwnerBinding)
	}
	packet := readPlanPacket(t, result.PacketPath)
	if packet.OwnerBinding != result.OwnerBinding || !strings.Contains(packet.ShardHandoffs[0].DispatchPrompt, "currentExecutor=session-plan") || !slices.Contains(packet.ShardHandoffs[0].ReviewerResultContract.RequiredFields, "reviewerSession") {
		t.Fatalf("packet omitted attached owner binding: %+v", packet)
	}
	if result.ReviewerOrchestration.Mode != "manual-main-agent-intake" || result.ReviewerOrchestration.TargetLane != "feature-intake" || result.ReviewerOrchestration.Dispatches[0].PreviewCommand == "" || strings.Contains(result.ReviewerOrchestration.Dispatches[0].PreviewCommand, "n/a:") {
		t.Fatalf("attached case reviewer orchestration did not expose runnable intake: %+v", result.ReviewerOrchestration)
	}
	if result.MissionCommanderAction.State != "ready-for-reviewer-dispatch" || result.ReviewerOrchestration.MissionCommanderAction == nil || result.ReviewerOrchestration.MissionCommanderAction.PrimaryCommand != result.MissionCommanderAction.PrimaryCommand || !hasPlanCommanderNextAction(result.MissionCommanderNextActions, "reviewerOrchestration.dispatch", "dispatch read-only reviewer for shard-01", false, true) || !hasPlanCommanderNextAction(result.MissionCommanderNextActions, "reviewerOrchestration.intake.preview", "-WhatIf -Format json", true, true) || !hasPlanCommanderNextAction(result.MissionCommanderNextActions, "reviewerOrchestration.intake.apply", "-Apply -Format json", true, true) {
		t.Fatalf("attached case plan omitted Mission Commander reviewer dispatch/intake guidance: action=%+v next=%+v orchestration=%+v", result.MissionCommanderAction, result.MissionCommanderNextActions, result.ReviewerOrchestration)
	}
	summary, err := os.ReadFile(result.SummaryPath)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(summary), "owner binding current executor: `session-plan`") || !strings.Contains(string(summary), "owner binding required for intake: `true`") {
		t.Fatalf("summary omitted owner binding:\n%s", string(summary))
	}
}

func TestWritePlanNoItemsKeepsEmptyShardHandoffs(t *testing.T) {
	repoRoot := filepath.Clean(filepath.Join("..", "..", ".."))
	result, err := WritePlan(repoRoot, t.TempDir(), defaults.DefaultPack, Options{Route: defaults.DefaultPack + ":bounded-review", ReviewOutputDir: t.TempDir()})
	if err != nil {
		t.Fatal(err)
	}
	if result.ItemCount != 0 || result.ShardCount != 0 || len(result.ShardHandoffs) != 0 {
		t.Fatalf("unexpected empty plan: %+v", result)
	}
	if result.MissionCommanderAction.State != "reviewer-plan-empty" || len(result.MissionCommanderNextActions) != 1 || result.MissionCommanderNextActions[0].Source != "reviewerOrchestration.plan" {
		t.Fatalf("empty plan omitted Mission Commander replanning guidance: action=%+v next=%+v", result.MissionCommanderAction, result.MissionCommanderNextActions)
	}
	summary, err := os.ReadFile(result.SummaryPath)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(summary), "- no shard handoffs planned") {
		t.Fatalf("summary missing empty handoff marker:\n%s", string(summary))
	}
}

func readPlanPacket(t *testing.T, path string) Packet {
	t.Helper()
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	var packet Packet
	if err := json.Unmarshal(data, &packet); err != nil {
		t.Fatal(err)
	}
	return packet
}

func hasPlanCommanderNextAction(items []mission.MissionCommanderNextActionItem, source, commandPart string, blocked, requiresReview bool) bool {
	return slices.ContainsFunc(items, func(item mission.MissionCommanderNextActionItem) bool {
		return item.Source == source && strings.Contains(item.Command, commandPart) && item.Blocked == blocked && item.RequiresReview == requiresReview
	})
}

func assertShardHandoff(t *testing.T, handoff ShardHandoff, wantID string, wantItems []string) {
	t.Helper()
	if handoff.ShardID != wantID || handoff.Status != "planned" || strings.Join(handoff.Items, ",") != strings.Join(wantItems, ",") || filepath.Base(handoff.ReviewerResultPath) != wantID+".json" {
		t.Fatalf("unexpected shard handoff identity: %+v", handoff)
	}
	for _, expected := range []string{"read-only reviewer", "Do not write files", "plan-subagents -ReviewerResultPath"} {
		if !strings.Contains(handoff.DispatchPrompt, expected) {
			t.Fatalf("dispatch prompt missing %q: %+v", expected, handoff)
		}
	}
	if !strings.Contains(handoff.ExpectedOutput, "decision") || !strings.Contains(handoff.ReviewerWriteback, "plan-subagents -ReviewerResultPath") || !strings.Contains(handoff.MainAgentNextAction, "reviewerResultContract") || !strings.Contains(handoff.MainAgentNextAction, "reviewerIntakeCommands") {
		t.Fatalf("unexpected reviewer contract: %+v", handoff)
	}
	if handoff.ReviewerResultContract.OutputFormat == "" || !slices.Contains(handoff.ReviewerResultContract.RequiredFields, "recommendedVerdict") || !slices.Contains(handoff.ReviewerResultContract.RequiredFields, "routeOutput") || !slices.Contains(handoff.ReviewerResultContract.AllowedDecisions, "needs-more-evidence") || !slices.Contains(handoff.ReviewerResultContract.EvidenceRules, "missing, ambiguous, or inaccessible evidenceRefs require decision=needs-more-evidence or defer") || !slices.Contains(handoff.ReviewerResultContract.ConflictSignals, "reviewer requests file writes, ledger append, authority/confirmed changes, heavy tools, or external effects") {
		t.Fatalf("unexpected reviewer result contract: %+v", handoff.ReviewerResultContract)
	}
	if !slices.Contains(handoff.IntakeChecklist, "validate reviewer output against reviewerResultContract before using any writeback template") || !slices.Contains(handoff.IntakeChecklist, "defer the main decision when conflicts, missing evidence, or blocked outputs are present") {
		t.Fatalf("unexpected intake checklist: %+v", handoff.IntakeChecklist)
	}
	if len(handoff.ReviewerDecisionMappings) != 5 || handoff.ReviewerDecisionMappings[0].ReviewerDecision != "accept" || handoff.ReviewerDecisionMappings[0].VerificationVerdict != "accepted" || handoff.ReviewerDecisionMappings[0].MainDecision != "accept" || handoff.ReviewerDecisionMappings[3].ReviewerDecision != "abandon" || handoff.ReviewerDecisionMappings[3].MainDecision != "supersede" || handoff.ReviewerDecisionMappings[4].ReviewerDecision != "needs-more-evidence" || handoff.ReviewerDecisionMappings[4].VerificationVerdict != "needs-more-evidence" || handoff.ReviewerDecisionMappings[4].MainDecision != "defer" {
		t.Fatalf("unexpected reviewer decision mappings: %+v", handoff.ReviewerDecisionMappings)
	}
	if !slices.Contains(handoff.ConflictHandling, "if any conflictSignal is present, map verification verdict to inconclusive or needs-more-evidence and keep main decision deferred unless independently resolved") || !slices.Contains(handoff.ConflictHandling, "if reviewer requests writes, heavy tools, authority/confirmed changes, or external effects, discard that output for ledger purposes and escalate through the lane gate path") {
		t.Fatalf("unexpected conflict handling: %+v", handoff.ConflictHandling)
	}
	if len(handoff.WritebackSequence) != 5 || handoff.WritebackSequence[0].Step != "validate-reviewer-result" || handoff.WritebackSequence[2].Step != "preview-reviewer-intake" || handoff.WritebackSequence[3].Step != "apply-reviewer-intake" || handoff.WritebackSequence[4].Step != "post-review-validation" {
		t.Fatalf("unexpected writeback sequence: %+v", handoff.WritebackSequence)
	}
	if !slices.Contains(handoff.WritebackSequence[0].Uses, "reviewerResultContract") || !slices.Contains(handoff.WritebackSequence[2].Uses, "reviewerIntakeCommands.previewCommand") || !slices.Contains(handoff.WritebackSequence[2].BlockedBy, "wrong packet/case/pack/shard/items") || handoff.WritebackSequence[3].NextOnSuccess != "post-review-validation" {
		t.Fatalf("unexpected writeback sequence details: %+v", handoff.WritebackSequence)
	}
	if len(handoff.WritebackSequence[2].CommandBindings) != 1 || handoff.WritebackSequence[2].CommandBindings[0].Binding != "reviewer-intake-preview" || handoff.WritebackSequence[2].CommandBindings[0].Kind != "reviewer-intake" || !strings.Contains(handoff.WritebackSequence[2].CommandBindings[0].Command, "n/a: reviewer intake requires an attached rekit case") || !slices.Contains(handoff.WritebackSequence[2].CommandBindings[0].RequiredFields, "reviewerResultPath") {
		t.Fatalf("unexpected out-of-case reviewer intake preview command binding: %+v", handoff.WritebackSequence[2].CommandBindings)
	}
	if len(handoff.WritebackSequence[3].CommandBindings) != 1 || handoff.WritebackSequence[3].CommandBindings[0].Binding != "reviewer-intake-apply" || handoff.WritebackSequence[3].CommandBindings[0].Kind != "reviewer-intake" || !strings.Contains(handoff.WritebackSequence[3].CommandBindings[0].Command, "n/a: reviewer intake requires an attached rekit case") || handoff.WritebackSequence[3].NextOnFailure != "retry-same-intake-to-complete-writeback" {
		t.Fatalf("unexpected out-of-case reviewer intake apply command binding: %+v", handoff.WritebackSequence[3])
	}
	commands := handoff.ReviewerIntakeCommands
	if !strings.Contains(commands.PreviewCommand, "n/a: reviewer intake requires an attached rekit case") || !strings.Contains(commands.ApplyCommand, "n/a: reviewer intake requires an attached rekit case") || !slices.Contains(commands.RequiredFields, "packetPath") || !slices.Contains(commands.RequiredFields, "reviewerResultPath") || !slices.Contains(commands.RequiredFields, "targetLane") || !slices.Contains(commands.PreviewChecks, "confirm reviewer intake returns isMutation=false, applied=false, and readyForWriteback=true") || !slices.Contains(commands.PreviewChecks, "out-of-case review artifacts are dispatch-only; reviewer intake/writeback is unavailable until the target is an attached rekit case") || !slices.Contains(commands.BlockedOutputs, "reviewer intake must not execute heavy tools or write authority/confirmed state") || !slices.Contains(commands.BlockedOutputs, "out-of-case plan packets must not be presented as immediately runnable reviewer intake commands") || strings.Contains(commands.ApplyCommand, "note -Kind") {
		t.Fatalf("unexpected out-of-case reviewer intake commands: %+v", commands)
	}
	if !slices.Contains(handoff.PostReviewMerge, "run reviewerIntakeCommands.previewCommand and inspect verification, decision, and postValidation before applyCommand") || !slices.Contains(handoff.PostReviewMerge, "retry the identical applyCommand when an interrupted writeback needs idempotent completion") {
		t.Fatalf("unexpected post-review merge guidance: %+v", handoff.PostReviewMerge)
	}
	if !slices.Contains(handoff.ReadOnlyBoundary, "runtime does not spawn subagents") || !slices.Contains(handoff.ReadOnlyBoundary, "subagents must not write files") {
		t.Fatalf("missing read-only boundary: %+v", handoff)
	}
	if !slices.Contains(handoff.CompletionCriteria, "reviewer verdicts are recorded in the ledger before main merge decisions") || handoff.FailureHandling == "" {
		t.Fatalf("missing completion/failure guidance: %+v", handoff)
	}
}
