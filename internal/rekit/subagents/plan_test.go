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
	if packet.Command != commandName || packet.Route.ID != defaults.DefaultPack+":lane-feature-analysis" || len(packet.ShardHandoffs) != 2 {
		t.Fatalf("unexpected packet: %+v", packet)
	}
	assertShardHandoff(t, packet.ShardHandoffs[0], "shard-01", []string{"alpha", "beta"})

	summary, err := os.ReadFile(result.SummaryPath)
	if err != nil {
		t.Fatal(err)
	}
	for _, expected := range []string{"### shard handoff prompts", "read-only reviewer", "Do not write files", "expected output=`item,decision", "reviewer result contract", "evidence-rule:", "conflict-signal:", "intake-check:", "decision-map:", "conflict-handling:", "writeback-step:", "command-binding:", "writeback-blocker:", "verification writeback preview", "decision writeback preview", "-WhatIf -Format json", "preview-check:", "post-review:"} {
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
	if handoff.ShardID != wantID || handoff.Status != "planned" || strings.Join(handoff.Items, ",") != strings.Join(wantItems, ",") {
		t.Fatalf("unexpected shard handoff identity: %+v", handoff)
	}
	for _, expected := range []string{"read-only reviewer", "Do not write files", "note -Kind verification"} {
		if !strings.Contains(handoff.DispatchPrompt, expected) {
			t.Fatalf("dispatch prompt missing %q: %+v", expected, handoff)
		}
	}
	if !strings.Contains(handoff.ExpectedOutput, "decision") || !strings.Contains(handoff.ReviewerWriteback, "note -Kind verification") || !strings.Contains(handoff.MainAgentNextAction, "reviewerResultContract") || !strings.Contains(handoff.MainAgentNextAction, "previewCommand") || !strings.Contains(handoff.MainAgentNextAction, "applyCommand") {
		t.Fatalf("unexpected reviewer contract: %+v", handoff)
	}
	if handoff.ReviewerResultContract.OutputFormat == "" || !slices.Contains(handoff.ReviewerResultContract.RequiredFields, "recommendedVerdict") || !slices.Contains(handoff.ReviewerResultContract.AllowedDecisions, "needs-more-evidence") || !slices.Contains(handoff.ReviewerResultContract.EvidenceRules, "missing, ambiguous, or inaccessible evidenceRefs require decision=needs-more-evidence or defer") || !slices.Contains(handoff.ReviewerResultContract.ConflictSignals, "reviewer requests file writes, ledger append, authority/confirmed changes, heavy tools, or external effects") {
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
	if len(handoff.WritebackSequence) != 7 || handoff.WritebackSequence[0].Step != "validate-reviewer-result" || handoff.WritebackSequence[2].Step != "preview-verification-note" || handoff.WritebackSequence[3].Step != "apply-verification-note" || handoff.WritebackSequence[4].Step != "preview-main-decision-note" || handoff.WritebackSequence[5].Step != "apply-main-decision-note" || handoff.WritebackSequence[6].Step != "post-review-validation" {
		t.Fatalf("unexpected writeback sequence: %+v", handoff.WritebackSequence)
	}
	if !slices.Contains(handoff.WritebackSequence[0].Uses, "reviewerResultContract") || !slices.Contains(handoff.WritebackSequence[2].Uses, "ledgerWritebackTemplates[kind=verification].previewCommand") || !slices.Contains(handoff.WritebackSequence[4].BlockedBy, "verification note missing") || handoff.WritebackSequence[5].NextOnSuccess != "post-review-validation" {
		t.Fatalf("unexpected writeback sequence details: %+v", handoff.WritebackSequence)
	}
	if len(handoff.WritebackSequence[2].CommandBindings) != 1 || handoff.WritebackSequence[2].CommandBindings[0].Binding != "verification-preview" || handoff.WritebackSequence[2].CommandBindings[0].Kind != "verification" || !strings.Contains(handoff.WritebackSequence[2].CommandBindings[0].Command, "-Kind verification") || !strings.Contains(handoff.WritebackSequence[2].CommandBindings[0].Command, "-WhatIf -Format json") || !slices.Contains(handoff.WritebackSequence[2].CommandBindings[0].RequiredFields, "verdict") {
		t.Fatalf("unexpected verification preview command binding: %+v", handoff.WritebackSequence[2].CommandBindings)
	}
	if len(handoff.WritebackSequence[5].CommandBindings) != 1 || handoff.WritebackSequence[5].CommandBindings[0].Binding != "decision-apply" || handoff.WritebackSequence[5].CommandBindings[0].Kind != "decision" || !strings.Contains(handoff.WritebackSequence[5].CommandBindings[0].Command, "-Kind decision") || !strings.Contains(handoff.WritebackSequence[5].CommandBindings[0].Command, "-Apply") || !slices.Contains(handoff.WritebackSequence[5].CommandBindings[0].RequiredFields, "decision") {
		t.Fatalf("unexpected decision apply command binding: %+v", handoff.WritebackSequence[5].CommandBindings)
	}
	if len(handoff.LedgerWritebackTemplates) != 2 || handoff.LedgerWritebackTemplates[0].Kind != "verification" || handoff.LedgerWritebackTemplates[1].Kind != "decision" {
		t.Fatalf("unexpected writeback templates: %+v", handoff.LedgerWritebackTemplates)
	}
	verification := handoff.LedgerWritebackTemplates[0]
	decision := handoff.LedgerWritebackTemplates[1]
	if !strings.Contains(verification.Command, "-Kind verification") || !strings.Contains(verification.Command, "-Apply") || !strings.Contains(verification.PreviewCommand, "-WhatIf -Format json") || !strings.Contains(verification.ApplyCommand, "-Apply") || !strings.Contains(verification.ApplyCommand, "-TargetRef \"alpha,beta\"") || !slices.Contains(verification.RequiredFields, "target") || !slices.Contains(verification.RequiredFields, "evidenceRefs") || !slices.Contains(verification.AllowedValues, "verdict=accepted|rejected|inconclusive|needs-more-evidence") || !slices.Contains(verification.PreviewChecks, "confirm note WhatIf returns isMutation=false and applied=false") || !slices.Contains(verification.BlockedOutputs, "previewCommand must not write facts, authority, confirmed, board, lane, handoff, or source files") {
		t.Fatalf("unexpected verification writeback template: %+v", verification)
	}
	if !strings.Contains(decision.Command, "-Kind decision") || !strings.Contains(decision.PreviewCommand, "-WhatIf -Format json") || !strings.Contains(decision.ApplyCommand, "-Apply") || !strings.Contains(decision.ApplyCommand, "-Decision <accept|reject|defer|supersede>") || !strings.Contains(decision.ApplyCommand, "-TargetRef \"alpha,beta\"") || !slices.Contains(decision.RequiredFields, "decision") || !slices.Contains(decision.RequiredFields, "target") || !slices.Contains(decision.AllowedValues, "decision=accept|reject|defer|supersede") || !slices.Contains(decision.PreviewChecks, "confirm note WhatIf returns isMutation=false and applied=false") {
		t.Fatalf("unexpected decision writeback template: %+v", decision)
	}
	if !slices.Contains(handoff.PostReviewMerge, "run each template previewCommand and inspect note WhatIf output before applyCommand") || !slices.Contains(handoff.PostReviewMerge, "record reviewer verdict with the verification applyCommand; do not let the reviewer append ledger events directly") || !slices.Contains(handoff.PostReviewMerge, "record the main merge decision with the decision applyCommand only after validation/conflict review") {
		t.Fatalf("unexpected post-review merge guidance: %+v", handoff.PostReviewMerge)
	}
	if !slices.Contains(handoff.ReadOnlyBoundary, "runtime does not spawn subagents") || !slices.Contains(handoff.ReadOnlyBoundary, "subagents must not write files") {
		t.Fatalf("missing read-only boundary: %+v", handoff)
	}
	if !slices.Contains(handoff.CompletionCriteria, "reviewer verdicts are recorded in the ledger before main merge decisions") || handoff.FailureHandling == "" {
		t.Fatalf("missing completion/failure guidance: %+v", handoff)
	}
}
