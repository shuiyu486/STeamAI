package promote

import (
	"encoding/json"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"

	"github.com/shuiyu486/re-context-kits/internal/rekit/doctor"
	"github.com/shuiyu486/re-context-kits/internal/rekit/instance"
	"github.com/shuiyu486/re-context-kits/internal/rekit/manifest"
	"github.com/shuiyu486/re-context-kits/internal/rekit/mission"
	syncpkg "github.com/shuiyu486/re-context-kits/internal/rekit/sync"
)

func TestCreateCandidatesWhatIfDoesNotWrite(t *testing.T) {
	repoRoot, caseRoot, pack := promoteFixture(t)

	result, err := CreateCandidates(repoRoot, caseRoot, pack, CandidateOptions{WhatIf: true})
	if err != nil {
		t.Fatal(err)
	}
	if result.Command != "promote" || result.IsMutation || result.Applied || result.Created != 2 || result.Blocked < 1 || result.RequiresCleanup {
		t.Fatalf("unexpected what-if candidate result: %+v", result)
	}
	if result.IndexPath == "" {
		t.Fatal("what-if result missing index path")
	}
	assertCandidateWriteForTest(t, result.Writes, "references/template/README.md", "would-create-candidate")
	assertCandidateWriteForTest(t, result.Writes, "references/template/workflow-template.md", "blocked-deny-pattern")
	assertCandidateWriteForTest(t, result.Writes, "references/template/toolchain-router.md", "would-create-candidate")
	if result.ReviewPlan.Mode != "candidate-review-preview" || result.ReviewPlan.ItemCount != len(result.Writes) || len(result.ReviewPlan.DecisionChecklist) != len(result.Writes) || len(result.ReviewPlan.DecisionFollowThrough) != len(result.Writes) || len(result.ReviewPlan.CleanupTargets) != 2 || result.ReviewPlan.Reconsume.Mode != "pack-memory-reconsume-after-merge" {
		t.Fatalf("unexpected what-if review plan: %+v", result.ReviewPlan)
	}
	if summary := result.ReviewPlan.ReviewSummary; summary.Mode != "candidate-review-preview" || summary.Total != len(result.Writes) || summary.PendingReviewCount != 2 || summary.BlockedCount != result.Blocked || summary.ManagedDocCount != 3 || summary.ToolingCandidateCount != 1 || summary.CleanupTargetCount != 2 || summary.ReviewArtifactCount != len(result.ReviewPlan.ReviewArtifacts) || summary.DecisionChecklistCount != len(result.ReviewPlan.DecisionChecklist) || summary.DecisionFollowThroughCount != len(result.ReviewPlan.DecisionFollowThrough) || summary.ExecutionStepCount != len(result.ReviewPlan.MainAgentExecutionPlan) || summary.ReconsumeCheckCount != len(result.ReviewPlan.Reconsume.VerificationChecklist) || summary.NextActionCount != len(result.ReviewPlan.MissionCommanderNextActions) || !summary.RequiresReview || summary.RequiresCleanup || !summary.HasToolingCandidate || !summary.HasBlockedItems || !summary.HasIndex || !summary.HasDecisionArtifacts || !summary.HasCleanupArtifacts || !summary.HasReconsumeArtifacts || summary.ProofSummary.Total != len(result.ReviewPlan.ReviewArtifacts) || summary.ProofSummary.ProofProgress != "0/10" || summary.ProofSummary.CurrentStage != "decision-proof-required" || summary.ProofSummary.NextMissingProofType != "candidate-decision-note" || !strings.HasSuffix(summary.ProofSummary.NextMissingProofPath, ".candidate-decision-note.md") || summary.ProofSummary.NextMissingProof == nil || summary.ProofSummary.NextMissingProof.ProofType != "candidate-decision-note" || !strings.Contains(summary.ProofSummary.NextMissingProof.Action, "selected decisionFollowThrough outcome") || !promoteContainsSubstring(summary.ProofSummary.NextMissingProof.Boundary, "WhatIf did not create") || !promoteContainsSubstring(summary.ProofSummary.Boundary, "WhatIf did not create") || !summary.WhatIf || !promoteContainsSubstring(summary.Boundary, "WhatIf did not write") || !promoteContainsSubstring(summary.Boundary, "reviewSummary is read-only") {
		t.Fatalf("what-if review summary missing compact handoff: %+v", summary)
	}
	if !candidateDecisionFollowThroughContainsForTest(result.ReviewPlan.DecisionFollowThrough, "references/template/README.md", "accept", "promote -Apply is not a candidate-scoped accept path") || !candidateDecisionFollowThroughContainsForTest(result.ReviewPlan.DecisionFollowThrough, "references/template/README.md", "reject", "update or remove indexPath") || !candidateDecisionFollowThroughContainsForTest(result.ReviewPlan.DecisionFollowThrough, "references/template/toolchain-router.md", "accept", "doctor -Target <attached-case>") || !candidateDecisionFollowThroughContainsForTest(result.ReviewPlan.DecisionFollowThrough, "references/template/workflow-template.md", "blocked", "blocked item must not be copied") {
		t.Fatalf("what-if review plan missing accepted/rejected/superseded follow-through: %+v", result.ReviewPlan.DecisionFollowThrough)
	}
	if !candidateReviewArtifactContainsForTest(result.ReviewPlan.ReviewArtifacts, "references/template/README.md", "candidate-decision-note", "selected decisionFollowThrough outcome") || !candidateReviewArtifactContainsForTest(result.ReviewPlan.ReviewArtifacts, "references/template/README.md", "candidate-cleanup-proof", "WhatIf did not create candidatePath") || !candidateReviewArtifactContainsForTest(result.ReviewPlan.ReviewArtifacts, "references/template/toolchain-router.md", "fresh-case-reconsume-proof", "temporary fresh case") || !candidateReviewArtifactContainsForTest(result.ReviewPlan.ReviewArtifacts, "references/template/workflow-template.md", "blocked-review-note", "blocked item must not be copied") {
		t.Fatalf("what-if review plan missing review artifact handoff: %+v", result.ReviewPlan.ReviewArtifacts)
	}
	if !candidateExecutionPlanContainsForTest(result.ReviewPlan.MainAgentExecutionPlan, "collect-review-artifacts", "replacement executor can resume") || !candidateExecutionPlanContainsForTest(result.ReviewPlan.MainAgentExecutionPlan, "materialize-candidates", "promote -Target <attached-case>") || !candidateExecutionPlanContainsForTest(result.ReviewPlan.MainAgentExecutionPlan, "review-decisions", "matching decisionFollowThrough outcome") || !candidateExecutionPlanContainsForTest(result.ReviewPlan.MainAgentExecutionPlan, "cleanup-rejected-or-merged-candidates", "indexPath") {
		t.Fatalf("what-if review plan missing executable main agent handoff: %+v", result.ReviewPlan.MainAgentExecutionPlan)
	}
	if !candidateNextActionContainsForTest(result.ReviewPlan.MissionCommanderNextActions, "reviewPlan.decisionChecklist", "decisionFollowThrough") || !candidateNextActionContainsForTest(result.ReviewPlan.MissionCommanderNextActions, "reviewPlan.cleanupTargets", "delete candidatePath") || !candidateNextActionContainsForTest(result.ReviewPlan.MissionCommanderNextActions, "reviewPlan.reconsume.verificationChecklist", "doctor -Pack "+pack) || !candidateNextActionBoundaryContainsForTest(result.ReviewPlan.MissionCommanderNextActions, "WhatIf did not write") || result.ReviewPlan.MissionCommanderActionQueue.CurrentAction == nil || result.ReviewPlan.MissionCommanderActionQueue.CurrentAction.Source != "reviewPlan.decisionChecklist" {
		t.Fatalf("what-if review plan missing Mission Commander next actions/queue: actions=%+v queue=%+v", result.ReviewPlan.MissionCommanderNextActions, result.ReviewPlan.MissionCommanderActionQueue)
	}
	commander := result.ReviewPlan.MissionCommanderAction
	if commander.State != "preview-pack-memory-candidates" || commander.PrimaryCommand != "review reviewPlan.reviewItems" || !strings.Contains(commander.Prompt, "WhatIf preview") || !promoteContainsSubstring(commander.FollowUpCommands, "promote -CreateCandidates") || !promoteContainsSubstring(commander.Boundary, "WhatIf did not write") || !promoteContainsSubstring(commander.Boundary, "no authority/confirmed") || !promoteContainsSubstring(commander.Boundary, "no heavy-tool") {
		t.Fatalf("what-if review plan omitted Mission Commander candidate preview handoff: %+v", commander)
	}
	assertCandidateReviewItemForTest(t, result.ReviewPlan.ReviewItems, "references/template/README.md", "pending-review")
	assertCandidateReviewItemForTest(t, result.ReviewPlan.ReviewItems, "references/template/workflow-template.md", "blocked")
	if !strings.Contains(strings.Join(result.ReviewPlan.RuntimeBoundary, "\n"), "when not WhatIf") {
		t.Fatalf("what-if review plan should explain write boundary: %+v", result.ReviewPlan.RuntimeBoundary)
	}
	for _, write := range result.Writes {
		if write.Action == "would-create-candidate" {
			assertNotExists(t, write.TargetPath)
		}
	}
	assertNotExists(t, result.IndexPath)
	assertTreeEmptyOrMissing(t, result.CandidateRoot)
	assertTreeEmptyOrMissing(t, result.ToolingRoot)
}

func TestPlanIgnoresOnlyManagedTextLineEndingRepresentation(t *testing.T) {
	repoRoot, caseRoot, pack := promoteFixture(t)
	rel := filepath.FromSlash("references/template/README.md")
	packPath := filepath.Join(repoRoot, "packs", pack, rel)
	casePath := filepath.Join(caseRoot, rel)
	if err := os.WriteFile(packPath, []byte("# README\n\nshared content\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(casePath, []byte("# README\r\n\r\nshared content\r\n"), 0o644); err != nil {
		t.Fatal(err)
	}

	plan, err := Plan(repoRoot, caseRoot, pack)
	if err != nil {
		t.Fatal(err)
	}
	item := plan.Items[0]
	if item.Action != "unchanged" || item.Changed {
		t.Fatalf("line-ending-only representation should be unchanged: %+v", item)
	}
	if item.CaseHash == "" || item.PackHash == "" || item.CaseHash == item.PackHash {
		t.Fatalf("plan should retain distinct raw-byte hashes: %+v", item)
	}

	if err := os.WriteFile(casePath, []byte("# README\r\n\r\ndifferent content\r\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	plan, err = Plan(repoRoot, caseRoot, pack)
	if err != nil {
		t.Fatal(err)
	}
	item = plan.Items[0]
	if item.Action != "candidate-after-llm-review" || !item.Changed {
		t.Fatalf("real content change should require candidate review: %+v", item)
	}
}

func TestCreateCandidatesWritesIndexAndSanitizedTooling(t *testing.T) {
	repoRoot, caseRoot, pack := promoteFixture(t)

	result, err := CreateCandidates(repoRoot, caseRoot, pack, CandidateOptions{})
	if err != nil {
		t.Fatal(err)
	}
	if result.Command != "promote" || !result.IsMutation || !result.Applied || result.Created != 2 || result.Blocked < 1 || !result.RequiresCleanup {
		t.Fatalf("unexpected candidate result: %+v", result)
	}
	readmeWrite := assertCandidateWriteForTest(t, result.Writes, "references/template/README.md", "create-candidate")
	workflowWrite := assertCandidateWriteForTest(t, result.Writes, "references/template/workflow-template.md", "blocked-deny-pattern")
	toolingWrite := assertCandidateWriteForTest(t, result.Writes, "references/template/toolchain-router.md", "create-candidate")
	if err := assertInsideRoot(result.CandidateRoot, readmeWrite.TargetPath); err != nil {
		t.Fatalf("managed candidate escaped candidate root: %v", err)
	}
	if err := assertInsideRoot(result.ToolingRoot, toolingWrite.TargetPath); err != nil {
		t.Fatalf("tooling candidate escaped tooling root: %v", err)
	}
	if workflowWrite.TargetPath != filepath.Join(repoRoot, "packs", pack, filepath.FromSlash("references/template/workflow-template.md")) {
		t.Fatalf("blocked write target = %q, want pack source", workflowWrite.TargetPath)
	}
	assertExists(t, readmeWrite.TargetPath)
	assertExists(t, toolingWrite.TargetPath)
	assertExists(t, result.IndexPath)
	if result.ReviewPlan.Mode != "candidate-review" || result.ReviewPlan.ItemCount != len(result.Writes) || len(result.ReviewPlan.DecisionChecklist) != len(result.Writes) || len(result.ReviewPlan.DecisionFollowThrough) != len(result.Writes) || len(result.ReviewPlan.CleanupTargets) != 2 {
		t.Fatalf("unexpected candidate review plan: %+v", result.ReviewPlan)
	}
	if summary := result.ReviewPlan.ReviewSummary; summary.Mode != "candidate-review" || summary.Total != len(result.Writes) || summary.PendingReviewCount != 2 || summary.BlockedCount != result.Blocked || summary.ManagedDocCount != 3 || summary.ToolingCandidateCount != 1 || summary.CleanupTargetCount != 2 || summary.ReviewArtifactCount != len(result.ReviewPlan.ReviewArtifacts) || summary.DecisionChecklistCount != len(result.ReviewPlan.DecisionChecklist) || summary.DecisionFollowThroughCount != len(result.ReviewPlan.DecisionFollowThrough) || summary.ExecutionStepCount != len(result.ReviewPlan.MainAgentExecutionPlan) || summary.ReconsumeCheckCount != len(result.ReviewPlan.Reconsume.VerificationChecklist) || summary.NextActionCount != len(result.ReviewPlan.MissionCommanderNextActions) || !summary.RequiresReview || !summary.RequiresCleanup || !summary.HasToolingCandidate || !summary.HasBlockedItems || !summary.HasIndex || !summary.HasDecisionArtifacts || !summary.HasCleanupArtifacts || !summary.HasReconsumeArtifacts || summary.ProofSummary.Total != len(result.ReviewPlan.ReviewArtifacts) || summary.ProofSummary.ProofProgress != "0/10" || summary.ProofSummary.CurrentStage != "decision-proof-required" || summary.ProofSummary.NextMissingProofType != "candidate-decision-note" || !strings.HasSuffix(summary.ProofSummary.NextMissingProofPath, ".candidate-decision-note.md") || summary.ProofSummary.NextMissingProof == nil || summary.ProofSummary.NextMissingProof.Stage != "decision-proof-required" || !strings.Contains(summary.ProofSummary.NextMissingProof.Format, "decision") || !promoteContainsSubstring(summary.ProofSummary.NextMissingProof.Evidence, "selected decisionFollowThrough outcome") || !promoteContainsSubstring(summary.ProofSummary.Boundary, "proofSummary is read-only") || summary.WhatIf || !promoteContainsSubstring(summary.Boundary, "reviewSummary is read-only") || !promoteContainsSubstring(summary.Boundary, "does not merge candidates") {
		t.Fatalf("candidate review summary missing compact handoff: %+v", summary)
	}
	readmeReview := assertCandidateReviewItemForTest(t, result.ReviewPlan.ReviewItems, "references/template/README.md", "pending-review")
	toolingReview := assertCandidateReviewItemForTest(t, result.ReviewPlan.ReviewItems, "references/template/toolchain-router.md", "pending-review")
	assertCandidateReviewItemForTest(t, result.ReviewPlan.ReviewItems, "references/template/workflow-template.md", "blocked")
	if readmeReview.CleanupPath != readmeWrite.TargetPath || !strings.Contains(readmeReview.MergeTargetHint, "pack managed doc") || strings.Contains(readmeReview.MergeTargetHint, "rerun promote -Apply") {
		t.Fatalf("managed doc review guidance drifted: %+v", readmeReview)
	}
	if !strings.Contains(strings.Join(readmeReview.MainAgentActions, "\n"), "update or remove indexPath") {
		t.Fatalf("managed doc review guidance missing index cleanup: %+v", readmeReview.MainAgentActions)
	}
	if !candidateDecisionFollowThroughContainsForTest(result.ReviewPlan.DecisionFollowThrough, "references/template/README.md", "accept", "pack source diff") || !candidateDecisionFollowThroughContainsForTest(result.ReviewPlan.DecisionFollowThrough, "references/template/README.md", "superseded", "review note naming the replacement") {
		t.Fatalf("managed doc decision follow-through missing accept/superseded outcomes: %+v", result.ReviewPlan.DecisionFollowThrough)
	}
	if !candidateReviewArtifactContainsForTest(result.ReviewPlan.ReviewArtifacts, "references/template/README.md", "candidate-decision-note", "selected decisionFollowThrough outcome") || !candidateReviewArtifactContainsForTest(result.ReviewPlan.ReviewArtifacts, "references/template/README.md", "candidate-cleanup-proof", "indexPath update/removal check") || !candidateReviewArtifactContainsForTest(result.ReviewPlan.ReviewArtifacts, "references/template/toolchain-router.md", "fresh-case-reconsume-proof", "fresh case doctor output") || !candidateReviewArtifactContainsForTest(result.ReviewPlan.ReviewArtifacts, "references/template/toolchain-router.md", "attached-case-reconsume-proof", "templateRoot/templatePack") || !candidateReviewArtifactContainsForTest(result.ReviewPlan.ReviewArtifacts, "references/template/workflow-template.md", "blocked-review-note", "blocked item must not be copied") {
		t.Fatalf("candidate review plan missing downstream review artifacts: %+v", result.ReviewPlan.ReviewArtifacts)
	}
	if toolingReview.CleanupPath != toolingWrite.TargetPath || !strings.Contains(toolingReview.MergeTargetHint, "tooling/catalog.yml") || !strings.Contains(result.ReviewPlan.Reconsume.Tooling, "tooling/recipes") || !candidateDecisionFollowThroughContainsForTest(result.ReviewPlan.DecisionFollowThrough, "references/template/toolchain-router.md", "accept", "fresh or attached case doctor output") {
		t.Fatalf("tooling review/reconsume guidance drifted: item=%+v reconsume=%+v followThrough=%+v", toolingReview, result.ReviewPlan.Reconsume, result.ReviewPlan.DecisionFollowThrough)
	}
	commander := result.ReviewPlan.MissionCommanderAction
	if commander.State != "ready-to-review-pack-memory-candidates" || commander.PrimaryCommand != "review reviewPlan.reviewItems" || !strings.Contains(commander.Prompt, "已生成") || !promoteContainsSubstring(commander.FollowUpCommands, "doctor -Pack "+pack) || !promoteContainsSubstring(commander.FollowUpCommands, "init -Target <fresh-case>") || !promoteContainsSubstring(commander.Boundary, "promote -Apply is not a candidate-scoped accept path") || !promoteContainsSubstring(commander.Boundary, "accepted tooling candidates require manual") || !promoteContainsSubstring(commander.Boundary, "verify fresh or attached case reconsume") || !promoteContainsSubstring(commander.Boundary, "no authority/confirmed") || !promoteContainsSubstring(commander.Boundary, "no heavy-tool") {
		t.Fatalf("review plan omitted Mission Commander reconsume handoff: %+v", commander)
	}
	criteria := strings.Join(result.ReviewPlan.CompletionCriteria, "\n")
	if !strings.Contains(criteria, "fresh or attached case reconsume") || !strings.Contains(criteria, "promote -Apply is not a candidate-scoped accept path") || !strings.Contains(criteria, "indexPath is updated or removed") {
		t.Fatalf("review plan missing candidate-safe completion criteria: %+v", result.ReviewPlan.CompletionCriteria)
	}
	cleanup := strings.Join(func() []string {
		items := []string{}
		for _, target := range result.ReviewPlan.CleanupTargets {
			items = append(items, target.CleanupWhen)
		}
		return items
	}(), "\n")
	if !strings.Contains(cleanup, "update or remove indexPath") {
		t.Fatalf("review plan cleanup targets missing index cleanup guidance: %+v", result.ReviewPlan.CleanupTargets)
	}
	if !candidateExecutionPlanContainsForTest(result.ReviewPlan.MainAgentExecutionPlan, "review-decisions", "matching decisionFollowThrough outcome") || !candidateExecutionPlanContainsForTest(result.ReviewPlan.MainAgentExecutionPlan, "cleanup-rejected-or-merged-candidates", "indexPath") || !candidateExecutionPlanContainsForTest(result.ReviewPlan.MainAgentExecutionPlan, "pack-doctor-after-accepted-merge", "doctor -Pack "+pack) || !candidateExecutionPlanContainsForTest(result.ReviewPlan.MainAgentExecutionPlan, "fresh-case-reconsume-after-tooling-merge", "fresh-case") || !candidateExecutionPlanContainsForTest(result.ReviewPlan.MainAgentExecutionPlan, "attached-case-reconsume-after-tooling-merge", "attached-case") {
		t.Fatalf("review plan missing main agent execution plan: %+v", result.ReviewPlan.MainAgentExecutionPlan)
	}
	if !candidateNextActionContainsForTest(result.ReviewPlan.MissionCommanderNextActions, "reviewPlan.decisionChecklist", "decisionFollowThrough") || result.ReviewPlan.MissionCommanderActionQueue.CurrentAction == nil || result.ReviewPlan.MissionCommanderActionQueue.CurrentAction.Source != "reviewPlan.decisionChecklist" || result.ReviewPlan.MissionCommanderActionQueue.Counts.Total != len(result.ReviewPlan.MissionCommanderNextActions) {
		t.Fatalf("review plan missing Mission Commander decision follow-through queue: actions=%+v queue=%+v", result.ReviewPlan.MissionCommanderNextActions, result.ReviewPlan.MissionCommanderActionQueue)
	}
	if !strings.Contains(strings.Join(result.ReviewPlan.RuntimeBoundary, "\n"), "decisionFollowThrough") || !strings.Contains(strings.Join(result.ReviewPlan.RuntimeBoundary, "\n"), "runtime does not execute merge, cleanup, init, or doctor") {
		t.Fatalf("review plan runtime boundary missing execution-plan no-run guard: %+v", result.ReviewPlan.RuntimeBoundary)
	}

	var index []candidateIndexEntry
	b, err := os.ReadFile(result.IndexPath)
	if err != nil {
		t.Fatal(err)
	}
	if err := json.Unmarshal(b, &index); err != nil {
		t.Fatalf("candidate index did not decode: %v\n%s", err, string(b))
	}
	if len(index) != 1 || index[0].Path != "references/template/README.md" || index[0].Candidate != readmeWrite.TargetPath {
		t.Fatalf("unexpected candidate index: %+v", index)
	}

	readme, err := os.ReadFile(readmeWrite.TargetPath)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(readme), "Reusable package-test candidate") {
		t.Fatalf("managed candidate missing case content:\n%s", string(readme))
	}
	tooling, err := os.ReadFile(toolingWrite.TargetPath)
	if err != nil {
		t.Fatal(err)
	}
	toolingText := string(tooling)
	for _, expected := range []string{"<caseRoot>", "<absolutePath>", "<artifactsPath>", "<capturesPath>", "<address>", "<ctxNNN>", "<roundN>", "Task #<n>"} {
		if !strings.Contains(toolingText, expected) {
			t.Fatalf("tooling candidate missing %q:\n%s", expected, toolingText)
		}
	}
	for _, unexpected := range []string{caseRoot, "C:\\cases", "demo-trace.csv", "demo-dump.bin", "0x401000", "ctx123", "round7", "Task #99"} {
		if strings.Contains(toolingText, unexpected) {
			t.Fatalf("tooling candidate still contains %q:\n%s", unexpected, toolingText)
		}
	}
}

func TestSanitizeToolingCandidateRedactsCaseSpecificValues(t *testing.T) {
	caseRoot := filepath.Join(t.TempDir(), "targetalpha")
	input := "Case root: " + caseRoot + "\n" +
		"Absolute: C:\\cases\\targetalpha\\sample.exe\n" +
		"Artifacts: artifacts/run/demo-trace.csv\n" +
		"Captures: captures/run/demo-dump.bin\n" +
		"Address: 0x401000 ctx123 round7 Task #99\n"

	out, counts := sanitizeToolingCandidate(input, caseRoot)
	for _, expected := range []string{"<caseRoot>", "<absolutePath>", "<artifactsPath>", "<capturesPath>", "<address>", "<ctxNNN>", "<roundN>", "Task #<n>"} {
		if !strings.Contains(out, expected) {
			t.Fatalf("sanitized output missing %q:\n%s", expected, out)
		}
	}
	for _, key := range []string{"caseRoot", "absolutePath", "artifactsPath", "capturesPath", "address", "ctx", "round", "task"} {
		if counts[key] == 0 {
			t.Fatalf("replacement count for %s = 0; counts=%+v output=%s", key, counts, out)
		}
	}
	for _, unexpected := range []string{caseRoot, "targetalpha", "C:\\cases", "demo-trace.csv", "demo-dump.bin", "0x401000", "ctx123", "round7", "Task #99"} {
		if strings.Contains(out, unexpected) {
			t.Fatalf("sanitized output still contains %q:\n%s", unexpected, out)
		}
	}
}

func TestPackMemoryPromoteReconsumeE2E(t *testing.T) {
	repoRoot, sourceCase, freshCase, pack := packMemoryReconsumeFixture(t)

	plan, err := Plan(repoRoot, sourceCase, pack)
	if err != nil {
		t.Fatal(err)
	}
	if len(plan.ToolingItems) != 1 {
		t.Fatalf("promote plan tooling items = %d, want 1", len(plan.ToolingItems))
	}
	if item := plan.ToolingItems[0]; item.Action != "sanitized-preview-for-llm-review" || item.SanitizedPreviewText == "" || len(item.DenyViolations) != 0 {
		t.Fatalf("unexpected tooling promote item: %+v", item)
	}

	result, err := CreateCandidates(repoRoot, sourceCase, pack, CandidateOptions{})
	if err != nil {
		t.Fatal(err)
	}
	write := assertCandidateWriteForTest(t, result.Writes, "references/template/toolchain-router.md", "create-candidate")
	reviewItem := assertCandidateReviewItemForTest(t, result.ReviewPlan.ReviewItems, "references/template/toolchain-router.md", "pending-review")
	if reviewItem.CandidatePath != write.TargetPath || reviewItem.CleanupPath != write.TargetPath || !strings.Contains(reviewItem.MergeTargetHint, "tooling/recipes") {
		t.Fatalf("pack-memory tooling candidate guidance drifted: %+v", reviewItem)
	}
	if len(result.ReviewPlan.DecisionChecklist) != len(result.Writes) || len(result.ReviewPlan.DecisionFollowThrough) != len(result.Writes) || !candidateDecisionChecklistContainsForTest(result.ReviewPlan.DecisionChecklist, "references/template/toolchain-router.md", "fresh-case-reconsume") || !candidateDecisionFollowThroughContainsForTest(result.ReviewPlan.DecisionFollowThrough, "references/template/toolchain-router.md", "accept", "doctor -Target <attached-case>") {
		t.Fatalf("pack-memory decision checklist/follow-through missing tooling reconsume handoff: checklist=%+v followThrough=%+v", result.ReviewPlan.DecisionChecklist, result.ReviewPlan.DecisionFollowThrough)
	}
	if !strings.Contains(result.ReviewPlan.Reconsume.Tooling, "fresh case") || !strings.Contains(strings.Join(result.ReviewPlan.Reconsume.Boundary, "\n"), "no tooling recipe copy") || !candidateReconsumeChecklistContainsForTest(result.ReviewPlan.Reconsume.VerificationChecklist, "fresh-case-reconsume") {
		t.Fatalf("pack-memory reconsume guidance drifted: %+v", result.ReviewPlan.Reconsume)
	}
	if result.ReviewPlan.MissionCommanderAction.State != "ready-to-review-pack-memory-candidates" || !promoteContainsSubstring(result.ReviewPlan.MissionCommanderAction.FollowUpCommands, "doctor -Target <fresh-case>") || !promoteContainsSubstring(result.ReviewPlan.MissionCommanderAction.Boundary, "verify fresh or attached case reconsume") {
		t.Fatalf("pack-memory reconsume omitted Mission Commander follow-through: %+v", result.ReviewPlan.MissionCommanderAction)
	}
	if !candidateExecutionPlanContainsForTest(result.ReviewPlan.MainAgentExecutionPlan, "fresh-case-reconsume-after-tooling-merge", "init -Target <fresh-case>") || !candidateExecutionPlanContainsForTest(result.ReviewPlan.MainAgentExecutionPlan, "attached-case-reconsume-after-tooling-merge", "doctor -Target <attached-case>") || !candidateExecutionPlanContainsForTest(result.ReviewPlan.MainAgentExecutionPlan, "fresh-case-reconsume-after-tooling-merge", "sync does not copy tooling recipes") {
		t.Fatalf("pack-memory execution plan omitted reconsume command handoff: %+v", result.ReviewPlan.MainAgentExecutionPlan)
	}
	if !candidateNextActionContainsForTest(result.ReviewPlan.MissionCommanderNextActions, "reviewPlan.decisionChecklist", "decisionFollowThrough") || !candidateNextActionContainsForTest(result.ReviewPlan.MissionCommanderNextActions, "reviewPlan.cleanupTargets", "update/remove indexPath") || !candidateNextActionContainsForTest(result.ReviewPlan.MissionCommanderNextActions, "reviewPlan.reconsume.verificationChecklist", "init -Target <fresh-case>") || !candidateNextActionContainsForTest(result.ReviewPlan.MissionCommanderNextActions, "reviewPlan.reconsume.verificationChecklist", "doctor -Target <attached-case>") || !candidateNextActionBoundaryContainsForTest(result.ReviewPlan.MissionCommanderNextActions, "runtime does not execute") || result.ReviewPlan.MissionCommanderActionQueue.CurrentAction == nil || result.ReviewPlan.MissionCommanderActionQueue.CurrentAction.Command != "review reviewPlan.decisionChecklist" {
		t.Fatalf("pack-memory review plan omitted Mission Commander next-action command UX: actions=%+v queue=%+v", result.ReviewPlan.MissionCommanderNextActions, result.ReviewPlan.MissionCommanderActionQueue)
	}
	candidateText := readText(t, write.TargetPath)
	for _, expected := range []string{"# Tooling candidate from case", "promoted-memory-tool", "<caseRoot>", "<absolutePath>", "<artifactsPath>", "<capturesPath>", "<address>", "<ctxNNN>", "Task #<n>"} {
		if !strings.Contains(candidateText, expected) {
			t.Fatalf("candidate missing %q:\n%s", expected, candidateText)
		}
	}
	for _, unexpected := range []string{sourceCase, "C:\\cases", "sourcecase", "reusable-trace.csv", "reusable-dump.bin", "0x401000", "ctx123", "Task #42"} {
		if strings.Contains(candidateText, unexpected) {
			t.Fatalf("candidate still contains case-specific %q:\n%s", unexpected, candidateText)
		}
	}

	consumedRecipeRel := "tooling/recipes/promoted-memory-tool.md"
	consumedRecipe := filepath.Join(repoRoot, "packs", pack, filepath.FromSlash(consumedRecipeRel))
	writeText(t, consumedRecipe, candidateText)
	m, err := manifest.Load(repoRoot, pack)
	if err != nil {
		t.Fatal(err)
	}
	if err := m.ValidateSchema(); err != nil {
		t.Fatalf("consumed tooling recipe is not manifest-valid: %v", err)
	}
	if _, err := doctor.Pack(repoRoot, pack); err != nil {
		t.Fatalf("pack doctor rejected consumed memory recipe: %v", err)
	}

	if _, err := syncpkg.Apply(repoRoot, freshCase, pack, syncpkg.ApplyOptions{CreateLocalFiles: true, Command: "init", ProjectName: "fresh-reconsumer"}); err != nil {
		t.Fatal(err)
	}
	inst, err := instance.Read(freshCase)
	if err != nil {
		t.Fatal(err)
	}
	if inst.TemplateRoot != repoRoot || inst.TemplatePack != pack || inst.ProjectName != "fresh-reconsumer" {
		t.Fatalf("fresh case metadata did not bind to promoted pack: %+v", inst)
	}
	if _, err := doctor.Case(repoRoot, freshCase, pack); err != nil {
		t.Fatalf("fresh case doctor rejected reconsumer case: %v", err)
	}
	freshRouter := readText(t, filepath.Join(freshCase, filepath.FromSlash("references/template/toolchain-router.md")))
	if strings.Contains(freshRouter, "promoted-memory-tool") {
		t.Fatalf("fresh managed router unexpectedly consumed tooling memory before review:\n%s", freshRouter)
	}
	freshRecipePath := filepath.Join(inst.TemplateRoot, "packs", inst.TemplatePack, filepath.FromSlash(consumedRecipeRel))
	freshRecipe := readText(t, freshRecipePath)
	for _, expected := range []string{"promoted-memory-tool", "<caseRoot>", "<absolutePath>", "<artifactsPath>", "<capturesPath>", "<address>", "<ctxNNN>", "Task #<n>"} {
		if !strings.Contains(freshRecipe, expected) {
			t.Fatalf("fresh reconsume recipe missing %q:\n%s", expected, freshRecipe)
		}
	}
	for _, unexpected := range []string{sourceCase, freshCase, "C:\\cases", "sourcecase", "freshcase", "reusable-trace.csv", "reusable-dump.bin", "0x401000", "ctx123", "Task #42"} {
		if strings.Contains(freshRecipe, unexpected) {
			t.Fatalf("fresh reconsume recipe leaked case-specific %q:\n%s", unexpected, freshRecipe)
		}
	}
}

func TestUniqueCandidatePathAvoidsExistingFiles(t *testing.T) {
	root := t.TempDir()
	writeText(t, filepath.Join(root, "candidate.md"), "existing")
	path, err := uniqueCandidatePath(root, "candidate.md")
	if err != nil {
		t.Fatal(err)
	}
	if path != filepath.Join(root, "candidate-1.md") {
		t.Fatalf("uniqueCandidatePath = %q, want candidate-1.md", path)
	}
}

func TestRestorePromoteBackupsRestoresPromotedTargets(t *testing.T) {
	root := t.TempDir()
	target := filepath.Join(root, "pack", "README.md")
	backup := filepath.Join(root, "backup", "README.md")
	ignoredTarget := filepath.Join(root, "pack", "ignored.md")
	ignoredBackup := filepath.Join(root, "backup", "ignored.md")
	writeText(t, target, "new text")
	writeText(t, backup, "old text")
	writeText(t, ignoredTarget, "keep new")
	writeText(t, ignoredBackup, "keep old")

	err := restorePromoteBackups([]ApplyWrite{
		{Action: "promote", TargetPath: target, BackupPath: backup},
		{Action: "would-promote", TargetPath: ignoredTarget, BackupPath: ignoredBackup},
	})
	if err != nil {
		t.Fatal(err)
	}
	if got := readText(t, target); got != "old text" {
		t.Fatalf("restored target = %q, want old text", got)
	}
	if got := readText(t, ignoredTarget); got != "keep new" {
		t.Fatalf("ignored target = %q, want keep new", got)
	}
}

func TestApplyWhatIfDoesNotWritePack(t *testing.T) {
	repoRoot, caseRoot, pack := promoteApplyFixture(t, "16384", "# README\n\nReusable guidance.\n")
	readmePath := filepath.Join(repoRoot, "packs", pack, filepath.FromSlash("references/template/README.md"))
	before := readText(t, readmePath)

	result, err := Apply(repoRoot, caseRoot, pack, ApplyOptions{WhatIf: true})
	if err != nil {
		t.Fatal(err)
	}
	if result.Command != "promote" || result.IsMutation || result.Applied || result.Changed != 1 || result.Blocked != 1 || result.BackupRoot != "" || result.RequiresCleanup {
		t.Fatalf("unexpected promote apply what-if result: %+v", result)
	}
	write := assertApplyWriteForTest(t, result.Writes, "references/template/README.md", "would-promote")
	if write.BackupPath == "" {
		t.Fatal("what-if promote write missing preview backup path")
	}
	assertNotExists(t, write.BackupPath)
	assertApplyWriteForTest(t, result.Writes, "references/template/workflow-template.md", "blocked-deny-pattern")
	if got := readText(t, readmePath); got != before {
		t.Fatalf("what-if changed pack README:\ngot:  %q\nwant: %q", got, before)
	}
}

func TestApplyWritesPackBackupAndValidationRows(t *testing.T) {
	repoRoot, caseRoot, pack := promoteApplyFixture(t, "16384", "# README\n\nReusable guidance.\n")
	readmePath := filepath.Join(repoRoot, "packs", pack, filepath.FromSlash("references/template/README.md"))
	workflowPath := filepath.Join(repoRoot, "packs", pack, filepath.FromSlash("references/template/workflow-template.md"))
	oldReadme := readText(t, readmePath)
	oldWorkflow := readText(t, workflowPath)

	result, err := Apply(repoRoot, caseRoot, pack, ApplyOptions{})
	if err != nil {
		t.Fatal(err)
	}
	if result.Command != "promote" || !result.IsMutation || !result.Applied || result.Changed != 1 || result.Blocked != 1 || !result.RequiresCleanup || len(result.ValidationRows) == 0 {
		t.Fatalf("unexpected promote apply result: %+v", result)
	}
	write := assertApplyWriteForTest(t, result.Writes, "references/template/README.md", "promote")
	assertApplyWriteForTest(t, result.Writes, "references/template/workflow-template.md", "blocked-deny-pattern")
	if got := readText(t, write.BackupPath); got != oldReadme {
		t.Fatalf("backup README = %q, want %q", got, oldReadme)
	}
	if got := readText(t, readmePath); !strings.Contains(got, "Reusable guidance") {
		t.Fatalf("pack README was not promoted: %q", got)
	}
	if got := readText(t, workflowPath); got != oldWorkflow {
		t.Fatalf("blocked workflow changed:\ngot:  %q\nwant: %q", got, oldWorkflow)
	}
}

func TestApplyValidationFailureRestoresPackSource(t *testing.T) {
	repoRoot, caseRoot, pack := promoteApplyFixture(t, "16", "# README\n\nThis reusable guidance is intentionally too long for the tiny budget.\n")
	readmePath := filepath.Join(repoRoot, "packs", pack, filepath.FromSlash("references/template/README.md"))
	oldReadme := readText(t, readmePath)

	result, err := Apply(repoRoot, caseRoot, pack, ApplyOptions{})
	if err == nil {
		t.Fatal("Apply returned nil error for validation failure")
	}
	if !strings.Contains(err.Error(), "pack files restored from backup") {
		t.Fatalf("error = %v, want restore message", err)
	}
	if result.Command != "promote" || !result.IsMutation || !result.Applied || result.Changed != 1 {
		t.Fatalf("unexpected failed apply result: %+v", result)
	}
	if got := readText(t, readmePath); got != oldReadme {
		t.Fatalf("pack README was not restored:\ngot:  %q\nwant: %q", got, oldReadme)
	}
}

func promoteFixture(t *testing.T) (repoRoot, caseRoot, pack string) {
	t.Helper()
	repoRoot = t.TempDir()
	caseRoot = filepath.Join(t.TempDir(), "targetalpha")
	pack = "unit-pack"
	packRoot := filepath.Join(repoRoot, "packs", pack)
	manifest := `schemaVersion: 1
name: unit-pack
version: 0.0.1
description: Unit test pack
maturity: template

managedFiles:
  - references/template/README.md
  - references/template/workflow-template.md
  - references/template/toolchain-router.md
templateFiles: []
localNeverOverwrite: []

promoteFiles:
  - references/template/README.md
  - references/template/workflow-template.md
  - references/template/toolchain-router.md

commonPolicies: []
policyOverlays: []
subagentRoutes: []
toolingFiles: []
promptFiles: []

toolingCandidateSources:
  - references/template/toolchain-router.md

heavyToolGates:
  - id: debug
    title: Dynamic debug or attach
    sideEffects: debug,filesystem-write
    defaultRisk: high
    requiresConfirmation: true
    stopConditions: timeout,unexpected-side-effect,scope-drift

promoteDenyPatterns:
  - "C:\\"
  - "artifacts[\\/]"
  - "captures[\\/]"
  - "[A-Za-z0-9_.-]*trace[A-Za-z0-9_.-]*\\.(csv|jsonl|log|txt|bin)"
  - "[A-Za-z0-9_.-]*dump[A-Za-z0-9_.-]*\\.(dmp|bin|raw|exe|dll)"
  - "\\.dmp\\b"
  - "0x[0-9A-Fa-f]{6,}"
  - "ctx[0-9]+"
  - "round[0-9]+"
  - "Task #[0-9]+"
`
	writeText(t, filepath.Join(packRoot, "manifest.yml"), manifest)
	writeText(t, filepath.Join(packRoot, filepath.FromSlash("references/template/README.md")), "# README\n\nOld pack text.\n")
	writeText(t, filepath.Join(packRoot, filepath.FromSlash("references/template/workflow-template.md")), "# Workflow\n\nOld pack text.\n")
	writeText(t, filepath.Join(packRoot, filepath.FromSlash("references/template/toolchain-router.md")), "# Tooling\n\nOld pack text.\n")

	metadata := "templateRoot: " + repoRoot + "\n" +
		"templatePack: " + pack + "\n" +
		"projectName: demo\n" +
		"projectRoot: " + caseRoot + "\n"
	writeText(t, filepath.Join(caseRoot, ".rekit", "instance.yml"), metadata)
	writeText(t, filepath.Join(caseRoot, filepath.FromSlash("references/template/README.md")), "# README\n\nReusable package-test candidate.\n")
	writeText(t, filepath.Join(caseRoot, filepath.FromSlash("references/template/workflow-template.md")), "# Workflow\n\nDo not promote C:\\case\\artifact\\sample-trace.csv from this case.\n")
	writeText(t, filepath.Join(caseRoot, filepath.FromSlash("references/template/toolchain-router.md")), "# Tooling\n\nCase root: "+caseRoot+"\nAbsolute: C:\\cases\\targetalpha\\sample.exe\nArtifacts: artifacts/run/demo-trace.csv\nCaptures: captures/run/demo-dump.bin\nAddress: 0x401000\nContext: ctx123 round7 Task #99\n")
	return repoRoot, caseRoot, pack
}

func packMemoryReconsumeFixture(t *testing.T) (repoRoot, sourceCase, freshCase, pack string) {
	t.Helper()
	repoRoot = t.TempDir()
	sourceCase = filepath.Join(t.TempDir(), "sourcecase")
	freshCase = filepath.Join(t.TempDir(), "freshcase")
	pack = "unit-pack"
	packRoot := filepath.Join(repoRoot, "packs", pack)
	manifestText := `schemaVersion: 1
name: unit-pack
version: 0.0.1
description: Unit test pack
maturity: template

managedFiles:
  - references/template/README.md
  - references/template/agent-team.md
  - references/template/workflow-template.md
  - references/template/toolchain-router.md
templateFiles:
  - references/template/task-handoff.template.md
localNeverOverwrite:
  - references/template/task-handoff.md

promoteFiles:
  - references/template/README.md
  - references/template/agent-team.md
  - references/template/workflow-template.md
  - references/template/toolchain-router.md

managedBlock:
  file: CLAUDE.local.md
  blockId: unit:router
  source: CLAUDE.local.snippet.md

syncPolicy:
  managedFiles: overwrite-with-backup
  templateFiles: create-if-missing
  localFiles: never-overwrite

workstreamDefaults:
  defaultAuthorityLane: main
  defaultStartLaneType: feature
  backupRoot: .rekit/backups/sync
  requestDefaultTargetLane: main
  handoffPath: references/template/task-handoff.md

authorityFiles:
  - references/template/task-handoff.md

laneTypes:
  - id: main
    title: Main
    authority: true
    workspaceRoot: workspace/main
    canWrite: references/template/task-handoff.md
    readOnly: .rekit/facts/**
    outputs: publication,decision,observation
  - id: feature
    title: Feature
    authority: false
    workspaceRoot: workspace/features
    canWrite: own-workspace
    readOnly: references/template/**,.rekit/facts/**
    outputs: observation,request,candidate,summary

commonPolicies:
  - agent-team
policyOverlays: []
subagentRoutes: []
toolingFiles:
  - tooling/README.md
  - tooling/catalog.yml
  - tooling/recipes/promoted-memory-tool.md
promptFiles: []

toolingCandidateSources:
  - references/template/toolchain-router.md

heavyToolGates:
  - id: debug
    title: Dynamic debug or attach
    sideEffects: debug,filesystem-write
    defaultRisk: high
    requiresConfirmation: true
    stopConditions: timeout,unexpected-side-effect,scope-drift

promoteDenyPatterns:
  - "C:\\\\"
  - "artifacts[\\\\/]"
  - "captures[\\\\/]"
  - "[A-Za-z0-9_.-]*trace[A-Za-z0-9_.-]*\\\\.(csv|jsonl|log|txt|bin)"
  - "[A-Za-z0-9_.-]*dump[A-Za-z0-9_.-]*\\\\.(dmp|bin|raw|exe|dll)"
  - "\\\\.dmp\\\\b"
  - "0x[0-9A-Fa-f]{6,}"
  - "ctx[0-9]+"
  - "round[0-9]+"
  - "Task #[0-9]+"

budgets:
  defaultMarkdown: 32768
`
	writeText(t, filepath.Join(packRoot, "manifest.yml"), manifestText)
	writeText(t, filepath.Join(repoRoot, ".claude", "skills", "rekit", "SKILL.md"), "# skill\n\n底层 Go CLI 是 canonical runtime\n`rekit.ps1` 只是 retained compatibility façade\ncase 只生成 `.claude/skills/rekit/SKILL.md` 薄 shim\n底层 runtime 只作为 `/rekit` 的内部实现\n")
	writeText(t, filepath.Join(repoRoot, "rekit", "templates", "case-shim", "SKILL.md"), "# shim\n\ncase-local 薄 shim\n不包含业务逻辑\ncanonical `/rekit`\n.rekit/instance.yml\n.re-template.yml\n<templateRoot>/.claude/skills/rekit/SKILL.md\ncanonical runtime\nsync` / `promote` 默认必须 review-first\n新会话 first screen 先使用 `/rekit`\nstatus case shim ready=true\ninstalledShimMatchesTemplate=true\ndurable artifacts 接手\nnext-batch action queues\n不要在本 shim 里维护模板规则\n不要读取或修改用户级 `~/.claude/skills`\n不要在 shim 中复制逻辑\n不展示底层脚本或 CLI 命令\nGo-native backend\n")
	writeText(t, filepath.Join(repoRoot, "common", "policies", "manifest.yml"), "policies:\n  - id: agent-team\n    path: agent-team.md\n")
	writeText(t, filepath.Join(repoRoot, "common", "policies", "README.md"), "# policies\n")
	writeText(t, filepath.Join(repoRoot, "common", "policies", "agent-team.md"), "# agent team\n")
	writeText(t, filepath.Join(packRoot, "policies", "manifest.yml"), "overlays: []\n")
	writeText(t, filepath.Join(packRoot, "policies", "README.md"), "# overlays\n")
	writeText(t, filepath.Join(packRoot, "CLAUDE.local.snippet.md"), "<!-- BEGIN unit:router -->\nunit router\n<!-- END unit:router -->\n")
	writeText(t, filepath.Join(packRoot, filepath.FromSlash("references/template/README.md")), "# README\n\nPack route.\n")
	writeText(t, filepath.Join(packRoot, filepath.FromSlash("references/template/agent-team.md")), "# Agent team\n\nPack route.\n")
	writeText(t, filepath.Join(packRoot, filepath.FromSlash("references/template/workflow-template.md")), "# Workflow\n\nPack route.\n")
	writeText(t, filepath.Join(packRoot, filepath.FromSlash("references/template/toolchain-router.md")), "# Tooling\n\nPack route.\n")
	writeText(t, filepath.Join(packRoot, filepath.FromSlash("references/template/task-handoff.template.md")), "# <PROJECT_NAME> handoff\n\nRoot: <PROJECT_ROOT>\n")
	writeText(t, filepath.Join(packRoot, filepath.FromSlash("tooling/README.md")), "# Tooling\n\nUse recipes after review.\n")
	writeText(t, filepath.Join(packRoot, filepath.FromSlash("tooling/catalog.yml")), "schemaVersion: 1\npack: unit-pack\ntools:\n  - id: promoted-memory-tool\n    status: candidate\n")
	writeText(t, filepath.Join(packRoot, filepath.FromSlash("tooling/recipes/promoted-memory-tool.md")), "# Promoted memory tool\n\nPlaceholder recipe.\n")

	sourceMetadata := "templateRoot: " + repoRoot + "\n" +
		"templatePack: " + pack + "\n" +
		"projectName: source\n" +
		"projectRoot: " + sourceCase + "\n"
	writeText(t, filepath.Join(sourceCase, ".rekit", "instance.yml"), sourceMetadata)
	writeText(t, filepath.Join(sourceCase, filepath.FromSlash("references/template/toolchain-router.md")), "# Tooling\n\nPromote candidate promoted-memory-tool after sourcecase run.\nCase root: "+sourceCase+"\nAbsolute: C:\\cases\\sourcecase\\sample.exe\nArtifacts: artifacts/run/reusable-trace.csv\nCaptures: captures/run/reusable-dump.bin\nAddress: 0x401000\nContext: ctx123 Task #42\n")

	return repoRoot, sourceCase, freshCase, pack
}

func promoteApplyFixture(t *testing.T, defaultBudget, caseReadme string) (repoRoot, caseRoot, pack string) {
	t.Helper()
	repoRoot = t.TempDir()
	caseRoot = filepath.Join(t.TempDir(), "applycase")
	pack = "unit-pack"
	packRoot := filepath.Join(repoRoot, "packs", pack)
	manifest := `schemaVersion: 1
name: unit-pack
version: 0.0.1
description: Unit test pack
maturity: template

managedFiles:
  - references/template/README.md
  - references/template/workflow-template.md
templateFiles: []
localNeverOverwrite: []

promoteFiles:
  - references/template/README.md
  - references/template/workflow-template.md

managedBlock:
  file: CLAUDE.local.md
  blockId: unit:router
  source: CLAUDE.local.snippet.md

syncPolicy:
  managedFiles: overwrite-with-backup
  templateFiles: create-if-missing
  localFiles: never-overwrite

workstreamDefaults:
  defaultAuthorityLane: main
  defaultStartLaneType: feature
  backupRoot: .rekit/backups/sync
  requestDefaultTargetLane: main

authorityFiles:
  - references/template/task-handoff.md

laneTypes:
  - id: main
    title: Main
    authority: true
    workspaceRoot: workspace/main
    canWrite: references/template/task-handoff.md
    readOnly: .rekit/facts/**
    outputs: publication,decision,observation
  - id: feature
    title: Feature
    authority: false
    workspaceRoot: workspace/features
    canWrite: own-workspace
    readOnly: references/template/**,.rekit/facts/**
    outputs: observation,request,candidate,summary

commonPolicies:
  - agent-team
policyOverlays: []
subagentRoutes: []
toolingFiles: []
promptFiles: []

toolingCandidateSources:
  - references/template/README.md

heavyToolGates:
  - id: debug
    title: Dynamic debug or attach
    sideEffects: debug,filesystem-write
    defaultRisk: high
    requiresConfirmation: true
    stopConditions: timeout,unexpected-side-effect,scope-drift

promoteDenyPatterns:
  - "C:\\"
  - "artifacts[\\/]"
  - "captures[\\/]"
  - "[A-Za-z0-9_.-]*trace[A-Za-z0-9_.-]*\\.(csv|jsonl|log|txt|bin)"
  - "[A-Za-z0-9_.-]*dump[A-Za-z0-9_.-]*\\.(dmp|bin|raw|exe|dll)"
  - "\\.dmp\\b"
  - "0x[0-9A-Fa-f]{6,}"
  - "ctx[0-9]+"
  - "round[0-9]+"
  - "Task #[0-9]+"

budgets:
  defaultMarkdown: ` + defaultBudget + `
`
	writeText(t, filepath.Join(packRoot, "manifest.yml"), manifest)
	writeText(t, filepath.Join(repoRoot, ".claude", "skills", "rekit", "SKILL.md"), "# skill\n\n底层 Go CLI 是 canonical runtime\n`rekit.ps1` 只是 retained compatibility façade\ncase 只生成 `.claude/skills/rekit/SKILL.md` 薄 shim\n底层 runtime 只作为 `/rekit` 的内部实现\n")
	writeText(t, filepath.Join(repoRoot, "rekit", "templates", "case-shim", "SKILL.md"), "# shim\n\ncase-local 薄 shim\n不包含业务逻辑\ncanonical `/rekit`\n.rekit/instance.yml\n.re-template.yml\n<templateRoot>/.claude/skills/rekit/SKILL.md\ncanonical runtime\nsync` / `promote` 默认必须 review-first\n新会话 first screen 先使用 `/rekit`\nstatus case shim ready=true\ninstalledShimMatchesTemplate=true\ndurable artifacts 接手\nnext-batch action queues\n不要在本 shim 里维护模板规则\n不要读取或修改用户级 `~/.claude/skills`\n不要在 shim 中复制逻辑\n不展示底层脚本或 CLI 命令\nGo-native backend\n")
	writeText(t, filepath.Join(repoRoot, "common", "policies", "manifest.yml"), "policies:\n  - id: agent-team\n    path: agent-team.md\n")
	writeText(t, filepath.Join(repoRoot, "common", "policies", "README.md"), "# policies\n")
	writeText(t, filepath.Join(repoRoot, "common", "policies", "agent-team.md"), "# agent team\n")
	writeText(t, filepath.Join(packRoot, "policies", "manifest.yml"), "overlays: []\n")
	writeText(t, filepath.Join(packRoot, "policies", "README.md"), "# overlays\n")
	writeText(t, filepath.Join(packRoot, "CLAUDE.local.snippet.md"), "<!-- BEGIN unit:router -->\nunit router\n<!-- END unit:router -->\n")
	writeText(t, filepath.Join(packRoot, filepath.FromSlash("references/template/README.md")), "# README\n\nOld pack text.\n")
	writeText(t, filepath.Join(packRoot, filepath.FromSlash("references/template/workflow-template.md")), "# Workflow\n\nOld pack text.\n")
	writeText(t, filepath.Join(packRoot, filepath.FromSlash("references/template/task-handoff.md")), "# Handoff\n")

	metadata := "templateRoot: " + repoRoot + "\n" +
		"templatePack: " + pack + "\n" +
		"projectName: demo\n" +
		"projectRoot: " + caseRoot + "\n"
	writeText(t, filepath.Join(caseRoot, ".rekit", "instance.yml"), metadata)
	writeText(t, filepath.Join(caseRoot, filepath.FromSlash("references/template/README.md")), caseReadme)
	writeText(t, filepath.Join(caseRoot, filepath.FromSlash("references/template/workflow-template.md")), "# Workflow\n\nDo not promote C:\\case\\artifact\\sample-trace.csv.\n")
	return repoRoot, caseRoot, pack
}

func assertApplyWriteForTest(t *testing.T, writes []ApplyWrite, path, action string) ApplyWrite {
	t.Helper()
	for _, write := range writes {
		if write.Path == path && write.Action == action {
			return write
		}
	}
	t.Fatalf("promote apply write %s/%s not found in %+v", path, action, writes)
	return ApplyWrite{}
}

func assertCandidateWriteForTest(t *testing.T, writes []CandidateWrite, path, action string) CandidateWrite {
	t.Helper()
	for _, write := range writes {
		if write.Path == path && write.Action == action {
			if write.TargetPath == "" && !strings.HasPrefix(action, "blocked") {
				t.Fatalf("candidate write %s/%s missing target path", path, action)
			}
			return write
		}
	}
	t.Fatalf("candidate write %s/%s not found in %+v", path, action, writes)
	return CandidateWrite{}
}

func assertCandidateReviewItemForTest(t *testing.T, items []CandidateReviewItem, path, decision string) CandidateReviewItem {
	t.Helper()
	for _, item := range items {
		if item.Path == path && item.ReviewDecision == decision {
			if decision == "pending-review" && (item.CandidatePath == "" || item.CleanupPath == "") {
				t.Fatalf("candidate review item %s missing candidate/cleanup path: %+v", path, item)
			}
			if len(item.MainAgentActions) == 0 {
				t.Fatalf("candidate review item %s missing main agent actions: %+v", path, item)
			}
			return item
		}
	}
	t.Fatalf("candidate review item %s/%s not found in %+v", path, decision, items)
	return CandidateReviewItem{}
}

func promoteContainsSubstring(items []string, want string) bool {
	for _, item := range items {
		if strings.Contains(item, want) {
			return true
		}
	}
	return false
}

func candidateNextActionContainsForTest(items []mission.MissionCommanderNextActionItem, source, want string) bool {
	return slices.ContainsFunc(items, func(item mission.MissionCommanderNextActionItem) bool {
		if item.Source != source {
			return false
		}
		fields := []string{item.Lane, item.Label, item.State, item.Command}
		fields = append(fields, item.Reasons...)
		fields = append(fields, item.Boundary...)
		return promoteContainsSubstring(fields, want)
	})
}

func candidateNextActionBoundaryContainsForTest(items []mission.MissionCommanderNextActionItem, want string) bool {
	return slices.ContainsFunc(items, func(item mission.MissionCommanderNextActionItem) bool {
		return promoteContainsSubstring(item.Boundary, want)
	})
}

func candidateDecisionChecklistContainsForTest(items []CandidateDecisionChecklist, path, want string) bool {
	for _, item := range items {
		if item.Path != path {
			continue
		}
		fields := []string{item.ReviewAction}
		fields = append(fields, item.AcceptActions...)
		fields = append(fields, item.RejectActions...)
		fields = append(fields, item.CleanupActions...)
		fields = append(fields, item.VerificationCommands...)
		fields = append(fields, item.Boundary...)
		if promoteContainsSubstring(fields, want) {
			return true
		}
	}
	return false
}

func candidateDecisionFollowThroughContainsForTest(items []CandidateDecisionFollowThrough, path, decision, want string) bool {
	for _, item := range items {
		if item.Path != path {
			continue
		}
		fields := []string{item.ReviewDecision, item.CandidatePath, item.PackTarget}
		fields = append(fields, item.Boundary...)
		for _, outcome := range item.Outcomes {
			if outcome.Decision != decision {
				continue
			}
			fields = append(fields, outcome.State, outcome.When, outcome.Expected)
			fields = append(fields, outcome.Actions...)
			fields = append(fields, outcome.CleanupActions...)
			fields = append(fields, outcome.VerificationCommands...)
			fields = append(fields, outcome.Evidence...)
			fields = append(fields, outcome.Boundary...)
		}
		if promoteContainsSubstring(fields, want) {
			return true
		}
	}
	return false
}

func candidateReconsumeChecklistContainsForTest(items []CandidateReconsumeVerification, name string) bool {
	for _, item := range items {
		if item.Name == name {
			return true
		}
	}
	return false
}

func candidateReviewArtifactContainsForTest(items []CandidateReviewArtifact, path, name, want string) bool {
	for _, item := range items {
		if item.Path != path || item.Name != name {
			continue
		}
		fields := []string{item.Kind, item.When, item.Action, item.CandidatePath, item.PackTarget, item.Format}
		fields = append(fields, item.Evidence...)
		fields = append(fields, item.Boundary...)
		if promoteContainsSubstring(fields, want) {
			return true
		}
	}
	return false
}

func candidateExecutionPlanContainsForTest(items []CandidateExecutionStep, name, want string) bool {
	for _, item := range items {
		if item.Name != name {
			continue
		}
		fields := []string{item.When, item.Expected}
		fields = append(fields, item.AppliesTo...)
		fields = append(fields, item.Actions...)
		fields = append(fields, item.Commands...)
		fields = append(fields, item.Evidence...)
		fields = append(fields, item.Boundary...)
		if promoteContainsSubstring(fields, want) {
			return true
		}
	}
	return false
}

func writeText(t *testing.T, path, text string) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte(text), 0o644); err != nil {
		t.Fatal(err)
	}
}

func readText(t *testing.T, path string) string {
	t.Helper()
	b, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	return string(b)
}

func assertExists(t *testing.T, path string) {
	t.Helper()
	if _, err := os.Stat(path); err != nil {
		t.Fatalf("expected %s to exist: %v", path, err)
	}
}

func assertNotExists(t *testing.T, path string) {
	t.Helper()
	if _, err := os.Stat(path); !os.IsNotExist(err) {
		t.Fatalf("expected %s to be absent, stat err=%v", path, err)
	}
}

func assertTreeEmptyOrMissing(t *testing.T, root string) {
	t.Helper()
	entries, err := os.ReadDir(root)
	if os.IsNotExist(err) {
		return
	}
	if err != nil {
		t.Fatal(err)
	}
	if len(entries) != 0 {
		t.Fatalf("expected %s to be empty, got %d entries", root, len(entries))
	}
}
