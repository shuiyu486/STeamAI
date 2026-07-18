package subagents

import (
	"encoding/json"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"

	"github.com/shuiyu486/re-context-kits/internal/rekit/defaults"
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
	if len(result.ShardHandoffs) != 2 {
		t.Fatalf("ShardHandoffs = %+v, want 2", result.ShardHandoffs)
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
	assertShardHandoff(t, packet.ShardHandoffs[0], "shard-01", []string{"alpha", "beta"})

	summary, err := os.ReadFile(result.SummaryPath)
	if err != nil {
		t.Fatal(err)
	}
	for _, expected := range []string{"### shard handoff prompts", "read-only reviewer", "Do not write files", "reviewer result root:", "main-agent result path=", "expected output=`item,decision", "reviewer result contract", "evidence-rule:", "conflict-signal:", "intake-check:", "decision-map:", "conflict-handling:", "writeback-step:", "command-binding:", "writeback-blocker:", "reviewer intake preview", "n/a: reviewer intake requires an attached rekit case", "out-of-case review artifacts are dispatch-only", "preview-check:", "post-review:"} {
		if !strings.Contains(string(summary), expected) {
			t.Fatalf("summary missing %q:\n%s", expected, string(summary))
		}
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
	summary, err := os.ReadFile(result.SummaryPath)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(summary), "- no shard handoffs planned") {
		t.Fatalf("summary missing empty handoff marker:\n%s", string(summary))
	}
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
