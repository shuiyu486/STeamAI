package workstream

import (
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/shuiyu486/re-context-kits/internal/rekit/defaults"
	"github.com/shuiyu486/re-context-kits/internal/rekit/plancontract"
)

func TestCompleteApplyRejectsMissingAndStalePlanWithoutMutation(t *testing.T) {
	repoRoot, caseRoot := setupContinueCase(t, "")
	lane, err := readLaneByID(caseRoot, "binary-analysis-bootstrap")
	if err != nil {
		t.Fatal(err)
	}
	evidencePath := filepath.Join(caseRoot, filepath.FromSlash(lane.Workspace), "typed-plan-evidence.md")
	if err := os.WriteFile(evidencePath, []byte("reviewed completion evidence\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	opt := CompleteOptions{Selector: "bootstrap", Actor: "main-agent", Reason: "typed plan contract", EvidenceRefs: relativePath(caseRoot, evidencePath)}
	before := snapshotWorkstreamTree(t, caseRoot)
	_, err = CompleteApply(repoRoot, caseRoot, defaults.DefaultPack, opt)
	failure, typed := plancontract.FromError(err)
	if err == nil || !typed || failure.Code != plancontract.CodePlanMissing || failure.MutationApplied || !IsZeroProgress(err) {
		t.Fatalf("missing plan failure=%+v typed=%t err=%v", failure, typed, err)
	}
	if after := snapshotWorkstreamTree(t, caseRoot); after != before {
		t.Fatalf("missing plan mutated case\nbefore:\n%s\nafter:\n%s", before, after)
	}

	opt.ExpectedPreviewSHA256 = strings.Repeat("0", 64)
	_, err = CompleteApply(repoRoot, caseRoot, defaults.DefaultPack, opt)
	failure, typed = plancontract.FromError(err)
	if err == nil || !typed || failure.Code != plancontract.CodePlanMismatch || failure.MutationApplied || !IsZeroProgress(err) {
		t.Fatalf("stale plan failure=%+v typed=%t err=%v", failure, typed, err)
	}
	if after := snapshotWorkstreamTree(t, caseRoot); after != before {
		t.Fatalf("stale plan mutated case\nbefore:\n%s\nafter:\n%s", before, after)
	}
}

func TestCompleteApplyPendingIntentRevalidatesMainLastBlockers(t *testing.T) {
	repoRoot, caseRoot := setupContinueCase(t, "")
	featureLane, err := readLaneByID(caseRoot, "binary-analysis-bootstrap")
	if err != nil {
		t.Fatal(err)
	}
	featureEvidencePath := filepath.Join(caseRoot, filepath.FromSlash(featureLane.Workspace), "completion-evidence.md")
	if err := os.WriteFile(featureEvidencePath, []byte("reviewed feature completion evidence\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	featureOpt := CompleteOptions{Selector: "bootstrap", Actor: "main-agent", Reason: "reviewed feature completion", EvidenceRefs: relativePath(caseRoot, featureEvidencePath)}
	featurePreview, err := CompletePreview(repoRoot, caseRoot, defaults.DefaultPack, featureOpt)
	if err != nil {
		t.Fatal(err)
	}
	featureOpt.ExpectedPreviewSHA256 = featurePreview.CompletionPlanSHA256
	if _, err := CompleteApply(repoRoot, caseRoot, defaults.DefaultPack, featureOpt); err != nil {
		t.Fatal(err)
	}
	evidencePath := filepath.Join(caseRoot, ".rekit", "lanes", "main", "workspace", "completion-evidence.md")
	if err := os.MkdirAll(filepath.Dir(evidencePath), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(evidencePath, []byte("reviewed completion evidence\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	opt := CompleteOptions{
		Selector:     "main",
		Actor:        "main-agent",
		Reason:       "reviewed durable evidence and operational lane completion",
		EvidenceRefs: ".rekit/lanes/main/workspace/completion-evidence.md",
	}
	preview, err := CompletePreview(repoRoot, caseRoot, defaults.DefaultPack, opt)
	if err != nil {
		t.Fatal(err)
	}
	if preview.Blocked || len(preview.CompletionPlanSHA256) != 64 {
		t.Fatalf("completion preview is not ready: blocked=%t blockers=%+v hash=%q", preview.Blocked, preview.Blockers, preview.CompletionPlanSHA256)
	}
	opt.ExpectedPreviewSHA256 = preview.CompletionPlanSHA256
	completionAfterIntentHook = func() error { return errors.New("stop after completion intent") }
	_, err = CompleteApply(repoRoot, caseRoot, defaults.DefaultPack, opt)
	completionAfterIntentHook = nil
	if err == nil || !strings.Contains(err.Error(), "stop after completion intent") {
		t.Fatalf("completion intent interruption was not exercised: %v", err)
	}

	boardPath := filepath.Join(caseRoot, ".rekit", "board.json")
	data, err := os.ReadFile(boardPath)
	if err != nil {
		t.Fatal(err)
	}
	var board map[string]any
	if err := json.Unmarshal(data, &board); err != nil {
		t.Fatal(err)
	}
	board["lanes"] = append(board["lanes"].([]any), map[string]any{
		"id": "feature-late", "type": "feature", "name": "late", "status": "open", "authority": false,
	})
	data, err = json.MarshalIndent(board, "", "  ")
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(boardPath, append(data, '\n'), 0o644); err != nil {
		t.Fatal(err)
	}
	_, err = CompleteApply(repoRoot, caseRoot, defaults.DefaultPack, opt)
	if err == nil || !strings.Contains(err.Error(), "complete recovery is blocked") || !strings.Contains(err.Error(), "open-feature-lane") {
		t.Fatalf("pending intent recovery must revalidate main-last blockers: %v", err)
	}
	if _, err := os.Stat(filepath.Join(caseRoot, ".rekit", "lanes", "main", completionCommitFile)); !os.IsNotExist(err) {
		t.Fatalf("blocked recovery unexpectedly published completion commit: %v", err)
	}
}
