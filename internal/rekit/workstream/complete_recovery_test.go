package workstream

import (
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/shuiyu486/re-context-kits/internal/rekit/defaults"
)

func TestCompleteApplyPendingIntentRevalidatesMainLastBlockers(t *testing.T) {
	repoRoot, caseRoot := setupContinueCase(t, "")
	featureLane, err := readLaneByID(caseRoot, "feature-bootstrap")
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
