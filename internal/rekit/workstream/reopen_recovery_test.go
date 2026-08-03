package workstream

import (
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/shuiyu486/re-context-kits/internal/rekit/defaults"
	"github.com/shuiyu486/re-context-kits/internal/rekit/lanecompletion"
)

func TestReopenApplyRecoversExactPendingOperationIntent(t *testing.T) {
	repoRoot, caseRoot := setupContinueCase(t, "")
	completeLaneForReopenTest(t, repoRoot, caseRoot, "bootstrap")
	completeLaneForReopenTest(t, repoRoot, caseRoot, "main")

	evidencePath := filepath.Join(caseRoot, ".rekit", "reopen-evidence.md")
	if err := os.WriteFile(evidencePath, []byte("review found additional feature work\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	opt := ReopenOptions{
		Selector:     "bootstrap",
		Actor:        "main-agent",
		Reason:       "post-completion review requires additional feature evidence",
		EvidenceRefs: ".rekit/reopen-evidence.md",
	}
	preview, err := ReopenPreview(repoRoot, caseRoot, defaults.DefaultPack, opt)
	if err != nil {
		t.Fatal(err)
	}
	if len(preview.EffectiveTargets) != 2 || len(preview.ReopenPlanSHA256) != 64 {
		t.Fatalf("unexpected compound reopen preview: %+v", preview)
	}
	targetIDs := map[string]bool{}
	for _, target := range preview.EffectiveTargets {
		targetIDs[target.Lane.ID] = true
	}
	if !targetIDs["feature-bootstrap"] {
		t.Fatalf("compound reopen omitted requested feature target: %+v", preview.EffectiveTargets)
	}
	opt.ExpectedPreviewSHA256 = preview.ReopenPlanSHA256
	opt.PublicationStamp = preview.PublicationStamp
	reopenAfterOperationIntentHook = func() error { return errors.New("stop after reopen operation intent") }
	_, err = ReopenApply(repoRoot, caseRoot, defaults.DefaultPack, opt)
	reopenAfterOperationIntentHook = nil
	if err == nil || !strings.Contains(err.Error(), "stop after reopen operation intent") {
		t.Fatalf("reopen operation interruption was not exercised: %v", err)
	}

	operations, err := lanecompletion.InspectOperations(caseRoot)
	if err != nil {
		t.Fatal(err)
	}
	if !operations.Pending || operations.PendingSequence != 1 || operations.PendingIntentPath == "" {
		t.Fatalf("operation intent was not exposed as pending recovery: %+v", operations)
	}
	if _, err := ReopenPreview(repoRoot, caseRoot, defaults.DefaultPack, opt); err == nil || !strings.Contains(err.Error(), "recover the exact original reopen Apply") {
		t.Fatalf("new preview was accepted during pending publication: %v", err)
	}
	if _, err := CompletePreview(repoRoot, caseRoot, defaults.DefaultPack, CompleteOptions{Selector: "main", Actor: "main-agent", Reason: "must remain blocked", EvidenceRefs: ".rekit/lanes/main/workspace/completion-evidence.md"}); err == nil || !strings.Contains(err.Error(), "reopen operation publication is incomplete") {
		t.Fatalf("ordinary completion was accepted during pending reopen: %v", err)
	}
	wrong := opt
	wrong.ExpectedPreviewSHA256 = strings.Repeat("0", 64)
	if _, err := ReopenApply(repoRoot, caseRoot, defaults.DefaultPack, wrong); err == nil || !strings.Contains(err.Error(), "does not match") {
		t.Fatalf("wrong recovery hash was accepted: %v", err)
	}

	applied, err := ReopenApply(repoRoot, caseRoot, defaults.DefaultPack, opt)
	if err != nil {
		t.Fatal(err)
	}
	if !applied.Applied || applied.OperationCommit == nil || applied.OperationCommit.State != "committed" {
		t.Fatalf("exact recovery did not finish compound reopen: %+v", applied)
	}
	operations, err = lanecompletion.InspectOperations(caseRoot)
	if err != nil {
		t.Fatal(err)
	}
	if operations.Pending || operations.LatestSequence != 1 || operations.LatestCommitSHA == "" {
		t.Fatalf("recovered operation did not become effective: %+v", operations)
	}
	for laneID := range targetIDs {
		lifecycle, err := lanecompletion.Inspect(caseRoot, laneID)
		if err != nil {
			t.Fatal(err)
		}
		if lifecycle.State != lanecompletion.StateReopened || lifecycle.HeadSequence != 2 {
			t.Fatalf("lane %s was not reopened by recovered operation: %+v", laneID, lifecycle)
		}
	}
}

func TestReopenApplyRecoversEveryPartialPublicationStage(t *testing.T) {
	stages := []string{"intent", "event", "lane", "board", "resume", "receipt", "before-commit"}
	for _, stage := range stages {
		t.Run(stage, func(t *testing.T) {
			repoRoot, caseRoot := setupContinueCase(t, "")
			completeLaneForReopenTest(t, repoRoot, caseRoot, "bootstrap")
			completeLaneForReopenTest(t, repoRoot, caseRoot, "main")
			evidencePath := filepath.Join(caseRoot, ".rekit", "reopen-evidence.md")
			if err := os.WriteFile(evidencePath, []byte("review found additional feature work\n"), 0o644); err != nil {
				t.Fatal(err)
			}
			opt := ReopenOptions{Selector: "bootstrap", Actor: "main-agent", Reason: "post-completion review requires additional feature evidence", EvidenceRefs: ".rekit/reopen-evidence.md"}
			preview, err := ReopenPreview(repoRoot, caseRoot, defaults.DefaultPack, opt)
			if err != nil {
				t.Fatal(err)
			}
			opt.ExpectedPreviewSHA256 = preview.ReopenPlanSHA256
			opt.PublicationStamp = preview.PublicationStamp
			stopped := false
			reopenPublicationHook = func(current, lane string) error {
				if !stopped && current == stage {
					stopped = true
					return errors.New("stop after " + stage + " publication")
				}
				return nil
			}
			t.Cleanup(func() { reopenPublicationHook = nil })
			if _, err := ReopenApply(repoRoot, caseRoot, defaults.DefaultPack, opt); err == nil || !strings.Contains(err.Error(), "stop after "+stage) {
				t.Fatalf("stage interruption was not exercised: %v", err)
			}
			reopenPublicationHook = nil
			operations, err := lanecompletion.InspectOperations(caseRoot)
			if err != nil {
				t.Fatal(err)
			}
			if !operations.Pending {
				t.Fatalf("stage %s did not leave a recoverable pending operation: %+v", stage, operations)
			}
			applied, err := ReopenApply(repoRoot, caseRoot, defaults.DefaultPack, opt)
			if err != nil {
				t.Fatal(err)
			}
			if !applied.Applied || applied.OperationCommit == nil || applied.OperationCommit.State != "committed" {
				t.Fatalf("stage %s exact replay did not commit: %+v", stage, applied)
			}
			for _, target := range preview.EffectiveTargets {
				lifecycle, err := lanecompletion.Inspect(caseRoot, target.Lane.ID)
				if err != nil {
					t.Fatal(err)
				}
				if lifecycle.State != lanecompletion.StateReopened || lifecycle.HeadSequence != 2 {
					t.Fatalf("stage %s target %s did not finish reopening: %+v", stage, target.Lane.ID, lifecycle)
				}
			}
		})
	}
}

func TestReopenApplyReturnsCommittedReplayAfterResponseLoss(t *testing.T) {
	repoRoot, caseRoot := setupContinueCase(t, "")
	completeLaneForReopenTest(t, repoRoot, caseRoot, "bootstrap")
	completeLaneForReopenTest(t, repoRoot, caseRoot, "main")
	evidencePath := filepath.Join(caseRoot, ".rekit", "reopen-evidence.md")
	if err := os.WriteFile(evidencePath, []byte("review found additional feature work\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	opt := ReopenOptions{Selector: "bootstrap", Actor: "main-agent", Reason: "post-completion review requires additional feature evidence", EvidenceRefs: ".rekit/reopen-evidence.md"}
	preview, err := ReopenPreview(repoRoot, caseRoot, defaults.DefaultPack, opt)
	if err != nil {
		t.Fatal(err)
	}
	opt.ExpectedPreviewSHA256, opt.PublicationStamp = preview.ReopenPlanSHA256, preview.PublicationStamp
	reopenPublicationHook = func(stage, lane string) error {
		if stage == "commit" {
			return errors.New("response lost after commit")
		}
		return nil
	}
	if _, err := ReopenApply(repoRoot, caseRoot, defaults.DefaultPack, opt); err == nil || !strings.Contains(err.Error(), "response lost after commit") {
		t.Fatalf("post-commit response loss was not exercised: %v", err)
	}
	reopenPublicationHook = nil
	before := snapshotWorkstreamTree(t, caseRoot)
	replayed, err := ReopenApply(repoRoot, caseRoot, defaults.DefaultPack, opt)
	if err != nil {
		t.Fatal(err)
	}
	if !replayed.Applied || !replayed.Replay || replayed.OperationCommit == nil || replayed.OperationCommit.PreviewSHA256 != preview.ReopenPlanSHA256 {
		t.Fatalf("exact Apply did not return committed replay: %+v", replayed)
	}
	after := snapshotWorkstreamTree(t, caseRoot)
	if after != before {
		t.Fatal("committed replay changed case state")
	}

	wrong := opt
	wrong.Reason = "different reopen reason"
	if _, err := ReopenApply(repoRoot, caseRoot, defaults.DefaultPack, wrong); err == nil {
		t.Fatal("mismatched committed replay was accepted")
	}
}

func TestReopenApplyRejectsTamperedPendingPublicationWithOriginalHash(t *testing.T) {
	repoRoot, caseRoot := setupContinueCase(t, "")
	completeLaneForReopenTest(t, repoRoot, caseRoot, "bootstrap")
	completeLaneForReopenTest(t, repoRoot, caseRoot, "main")
	evidencePath := filepath.Join(caseRoot, ".rekit", "reopen-evidence.md")
	if err := os.WriteFile(evidencePath, []byte("review found additional feature work\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	opt := ReopenOptions{Selector: "bootstrap", Actor: "main-agent", Reason: "post-completion review requires additional feature evidence", EvidenceRefs: ".rekit/reopen-evidence.md"}
	preview, err := ReopenPreview(repoRoot, caseRoot, defaults.DefaultPack, opt)
	if err != nil {
		t.Fatal(err)
	}
	opt.ExpectedPreviewSHA256, opt.PublicationStamp = preview.ReopenPlanSHA256, preview.PublicationStamp
	reopenAfterOperationIntentHook = func() error { return errors.New("stop after immutable operation intent") }
	if _, err := ReopenApply(repoRoot, caseRoot, defaults.DefaultPack, opt); err == nil {
		t.Fatal("operation intent interruption was not exercised")
	}
	reopenAfterOperationIntentHook = nil
	intentPath := lanecompletion.OperationIntentPath(caseRoot, 1)
	data, err := os.ReadFile(intentPath)
	if err != nil {
		t.Fatal(err)
	}
	var intent lanecompletion.OperationIntent
	if err := json.Unmarshal(data, &intent); err != nil {
		t.Fatal(err)
	}
	intent.Publications[0].Bytes = []byte("{\"tampered\":true}\n")
	intent.Publications[0].AfterSHA256 = lanecompletion.SHA256Bytes(intent.Publications[0].Bytes)
	tamperedSHA, err := lanecompletion.ExactPublicationSHA256(intent)
	if err != nil {
		t.Fatal(err)
	}
	intent.ExactPublicationSHA256 = tamperedSHA
	tampered, err := json.MarshalIndent(intent, "", "  ")
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(intentPath, append(tampered, '\n'), 0o600); err != nil {
		t.Fatal(err)
	}
	before := snapshotWorkstreamTree(t, caseRoot)
	if _, err := ReopenApply(repoRoot, caseRoot, defaults.DefaultPack, opt); err == nil || !strings.Contains(err.Error(), "reviewed publication hash mismatch") {
		t.Fatalf("tampered pending publication was not rejected: %v", err)
	}
	if after := snapshotWorkstreamTree(t, caseRoot); after != before {
		t.Fatal("tampered pending recovery wrote case state")
	}
}

func TestReopenApplyRecoversFromIntentWithoutLiveEvidence(t *testing.T) {
	repoRoot, caseRoot := setupContinueCase(t, "")
	completeLaneForReopenTest(t, repoRoot, caseRoot, "bootstrap")
	completeLaneForReopenTest(t, repoRoot, caseRoot, "main")
	evidencePath := filepath.Join(caseRoot, ".rekit", "reopen-evidence.md")
	if err := os.WriteFile(evidencePath, []byte("review found additional feature work\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	opt := ReopenOptions{Selector: "bootstrap", Actor: "main-agent", Reason: "post-completion review requires additional feature evidence", EvidenceRefs: ".rekit/reopen-evidence.md"}
	preview, err := ReopenPreview(repoRoot, caseRoot, defaults.DefaultPack, opt)
	if err != nil {
		t.Fatal(err)
	}
	opt.ExpectedPreviewSHA256, opt.PublicationStamp = preview.ReopenPlanSHA256, preview.PublicationStamp
	reopenAfterOperationIntentHook = func() error { return errors.New("stop after immutable operation intent") }
	if _, err := ReopenApply(repoRoot, caseRoot, defaults.DefaultPack, opt); err == nil {
		t.Fatal("operation intent interruption was not exercised")
	}
	reopenAfterOperationIntentHook = nil
	if err := os.Remove(evidencePath); err != nil {
		t.Fatal(err)
	}
	applied, err := ReopenApply(repoRoot, caseRoot, defaults.DefaultPack, opt)
	if err != nil {
		t.Fatal(err)
	}
	if !applied.Applied || applied.OperationCommit == nil {
		t.Fatalf("immutable intent recovery did not complete: %+v", applied)
	}
}

func TestProjectHandoffRefusesPendingReopenWithoutWrites(t *testing.T) {
	repoRoot, caseRoot := setupContinueCase(t, "")
	completeLaneForReopenTest(t, repoRoot, caseRoot, "bootstrap")
	completeLaneForReopenTest(t, repoRoot, caseRoot, "main")
	evidencePath := filepath.Join(caseRoot, ".rekit", "reopen-evidence.md")
	if err := os.WriteFile(evidencePath, []byte("review found additional feature work\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	opt := ReopenOptions{Selector: "bootstrap", Actor: "main-agent", Reason: "post-completion review requires additional feature evidence", EvidenceRefs: ".rekit/reopen-evidence.md"}
	preview, err := ReopenPreview(repoRoot, caseRoot, defaults.DefaultPack, opt)
	if err != nil {
		t.Fatal(err)
	}
	opt.ExpectedPreviewSHA256 = preview.ReopenPlanSHA256
	opt.PublicationStamp = preview.PublicationStamp
	reopenPublicationHook = func(stage, lane string) error {
		if stage == "lane" {
			return errors.New("stop with partially reopened lane")
		}
		return nil
	}
	if _, err := ReopenApply(repoRoot, caseRoot, defaults.DefaultPack, opt); err == nil {
		t.Fatal("reopen interruption was not exercised")
	}
	reopenPublicationHook = nil
	before := snapshotWorkstreamTree(t, caseRoot)
	if _, err := HandoffPreview(repoRoot, caseRoot, defaults.DefaultPack, HandoffOptions{}); err == nil || !strings.Contains(err.Error(), "pending reopen operation") {
		t.Fatalf("project handoff preview accepted pending reopen: %v", err)
	}
	if got := snapshotWorkstreamTree(t, caseRoot); got != before {
		t.Fatal("project handoff preview wrote during pending reopen")
	}
	if _, err := HandoffApply(repoRoot, caseRoot, defaults.DefaultPack, HandoffOptions{ExpectedPublicationPlanSHA256: strings.Repeat("0", 64), PublicationStamp: "20260803-010203000"}); err == nil || !strings.Contains(err.Error(), "pending reopen operation") {
		t.Fatalf("project handoff apply accepted pending reopen: %v", err)
	}
	if got := snapshotWorkstreamTree(t, caseRoot); got != before {
		t.Fatal("project handoff apply wrote during pending reopen")
	}
}

func completeLaneForReopenTest(t *testing.T, repoRoot, caseRoot, selector string) {
	t.Helper()
	laneID := "feature-" + selector
	if selector == "main" {
		laneID = "main"
	}
	evidencePath := filepath.Join(caseRoot, ".rekit", "lanes", laneID, "workspace", "completion-evidence.md")
	if err := os.MkdirAll(filepath.Dir(evidencePath), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(evidencePath, []byte("reviewed completion evidence\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	opt := CompleteOptions{Selector: selector, Actor: "main-agent", Reason: "reviewed operational lane completion", EvidenceRefs: relativePath(caseRoot, evidencePath)}
	preview, err := CompletePreview(repoRoot, caseRoot, defaults.DefaultPack, opt)
	if err != nil {
		t.Fatal(err)
	}
	if preview.Blocked || preview.CompletionPlanSHA256 == "" {
		t.Fatalf("completion preview is blocked: %+v", preview)
	}
	opt.ExpectedPreviewSHA256 = preview.CompletionPlanSHA256
	if _, err := CompleteApply(repoRoot, caseRoot, defaults.DefaultPack, opt); err != nil {
		t.Fatal(err)
	}
}
