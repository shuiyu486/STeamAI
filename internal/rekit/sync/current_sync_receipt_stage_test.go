package sync

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/shuiyu486/re-context-kits/internal/rekit/projectstate"
)

func TestStageCurrentSyncReceiptIsCanonicalAndReplayable(t *testing.T) {
	fixture := newCurrentSyncFixture(t, "")
	_, intent := stageCurrentSyncExactObjectFixture(t, fixture)
	progress := currentSyncProgressBeforeOperation(
		t,
		intent,
		"receipt-stage-to-live",
	)
	receipt, binding, err := stageCurrentSyncReceipt(
		fixture.caseRoot,
		intent,
		progress,
	)
	if err != nil {
		t.Fatal(err)
	}
	replayReceipt, replayBinding, err := stageCurrentSyncReceipt(
		fixture.caseRoot,
		intent,
		progress,
	)
	if err != nil {
		t.Fatalf("current sync staged receipt replay failed: %v", err)
	}
	if !currentSyncCanonicalEqual(receipt, replayReceipt) ||
		!currentSyncCanonicalEqual(binding, replayBinding) ||
		binding.Kind != "current-sync-receipt" || binding.Mode != 0o600 ||
		binding.Path != currentSyncStatePath(currentSyncReceiptRel) ||
		!strings.EqualFold(receipt.ProgressSHA256, progress.ProgressSHA256) {
		t.Fatalf("current sync staged receipt identity drifted: receipt=%+v binding=%+v", receipt, binding)
	}
}

func TestStageCurrentSyncReceiptRejectsExistingDrift(t *testing.T) {
	fixture := newCurrentSyncFixture(t, "")
	_, intent := stageCurrentSyncExactObjectFixture(t, fixture)
	progress := currentSyncProgressBeforeOperation(
		t,
		intent,
		"receipt-stage-to-live",
	)
	path := filepath.Join(
		fixture.caseRoot,
		projectstate.CurrentDir,
		filepath.FromSlash(currentSyncStagedReceiptPath(intent)),
	)
	if err := os.WriteFile(path, []byte("drifted receipt\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	if _, _, err := stageCurrentSyncReceipt(
		fixture.caseRoot,
		intent,
		progress,
	); err == nil || !strings.Contains(err.Error(), "differs") {
		t.Fatalf("current sync accepted drifted staged receipt: %v", err)
	}
}

func currentSyncProgressBeforeOperation(
	t *testing.T,
	intent currentSyncIntent,
	kind string,
) currentSyncProgress {
	t.Helper()
	progress, err := newCurrentSyncProgress(intent)
	if err != nil {
		t.Fatal(err)
	}
	for {
		progress, err = currentSyncBeginProgressOperation(progress, intent)
		if err != nil {
			t.Fatal(err)
		}
		if progress.Pending != nil && progress.Pending.Kind == kind {
			return progress
		}
		progress, err = currentSyncCompleteProgressOperation(progress, intent)
		if err != nil {
			t.Fatal(err)
		}
	}
}
