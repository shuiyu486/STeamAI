package sync

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"

	rekitfs "github.com/shuiyu486/re-context-kits/internal/rekit/fs"
	"github.com/shuiyu486/re-context-kits/internal/rekit/projectstate"
)

func requireCurrentSyncApplyForTest(t *testing.T) {
	t.Helper()
	if !rekitfs.HandleBoundExactMutationSupported() {
		t.Skip("current sync Apply requires handle-bound exact filesystem mutation")
	}
}

func TestCurrentSyncApplyRejectsUnsupportedExactMutationBeforeLeaseOrWrites(t *testing.T) {
	fixture := newCurrentSyncFixture(t, "")
	options := CurrentSyncOptions{
		Command:          "sync",
		ProjectName:      "refreshed-project",
		SourceExecutable: fixture.sourceExecutable,
	}
	plan, err := CurrentSyncPreview(
		fixture.repoRoot,
		fixture.caseRoot,
		fixture.pack,
		options,
	)
	if err != nil {
		t.Fatal(err)
	}
	before := snapshotFiles(t, fixture.caseRoot)
	previousSupport := currentSyncHandleBoundExactMutationSupported
	previousAcquire := currentSyncAcquireRefreshLease
	leaseCalled := false
	currentSyncHandleBoundExactMutationSupported = func() bool { return false }
	currentSyncAcquireRefreshLease = func(string) (*currentSyncRefreshLease, error) {
		leaseCalled = true
		return nil, fmt.Errorf("lease must not be acquired")
	}
	t.Cleanup(func() {
		currentSyncHandleBoundExactMutationSupported = previousSupport
		currentSyncAcquireRefreshLease = previousAcquire
	})

	_, err = CurrentSyncApply(
		fixture.caseRoot,
		fixture.pack,
		CurrentSyncApplyOptions{
			SourceRepoRoot:     fixture.repoRoot,
			ExpectedPlanSHA256: plan.ExpectedPlanSHA256,
			CurrentSyncOptions: options,
		},
	)
	if err == nil || !strings.Contains(err.Error(), "requires handle-bound exact filesystem mutation support") {
		t.Fatalf("unsupported current sync Apply error=%v", err)
	}
	if leaseCalled {
		t.Fatal("unsupported current sync Apply acquired a mutation lease")
	}
	assertSnapshotEqual(t, before, snapshotFiles(t, fixture.caseRoot))
	assertNotExists(t, filepath.Join(
		fixture.caseRoot,
		projectstate.CurrentDir,
		filepath.FromSlash(currentSyncNamespaceRel),
	))
}

func TestCurrentSyncApplyRefreshesAndStrictlyReplays(t *testing.T) {
	requireCurrentSyncApplyForTest(t)
	fixture := newCurrentSyncFixture(t, "")
	options := CurrentSyncOptions{
		Command:          "sync",
		ProjectName:      "refreshed-project",
		SourceExecutable: fixture.sourceExecutable,
	}
	plan, err := CurrentSyncPreview(
		fixture.repoRoot,
		fixture.caseRoot,
		fixture.pack,
		options,
	)
	if err != nil {
		t.Fatal(err)
	}
	if plan.AlreadyCurrent || plan.ExpectedPlanSHA256 == "" {
		t.Fatalf("current sync preview is not refreshable: %+v", plan)
	}

	result, err := CurrentSyncApply(
		fixture.caseRoot,
		fixture.pack,
		CurrentSyncApplyOptions{
			SourceRepoRoot:     fixture.repoRoot,
			ExpectedPlanSHA256: plan.ExpectedPlanSHA256,
			CurrentSyncOptions: options,
		},
	)
	if err != nil {
		t.Fatal(err)
	}
	if result.Status != "refreshed" || !result.IsMutation ||
		!result.Applied || result.Replay || result.AlreadyCurrent ||
		result.PlanSHA256 != plan.ExpectedPlanSHA256 || result.Receipt == nil {
		t.Fatalf("unexpected current sync Apply result: %+v", result)
	}
	stateRoot := filepath.Join(fixture.caseRoot, projectstate.CurrentDir)
	for _, rel := range []string{currentSyncIntentRel, currentSyncOwnerRel} {
		if _, err := os.Lstat(filepath.Join(stateRoot, filepath.FromSlash(rel))); !os.IsNotExist(err) {
			t.Fatalf("current sync cleanup retained %s: %v", rel, err)
		}
	}
	if _, err := os.Lstat(filepath.Join(
		stateRoot,
		filepath.FromSlash(currentSyncArchivedIntentPath(currentSyncIntent{
			PlanSHA256:      result.PlanSHA256,
			TransactionPath: currentSyncTransactionPath(result.PlanSHA256),
		})),
	)); err != nil {
		t.Fatalf("current sync cleanup removed archived intent: %v", err)
	}

	replay, err := CurrentSyncApply(
		fixture.caseRoot,
		fixture.pack,
		CurrentSyncApplyOptions{
			SourceRepoRoot:     fixture.repoRoot,
			ExpectedPlanSHA256: plan.ExpectedPlanSHA256,
			CurrentSyncOptions: options,
		},
	)
	if err != nil {
		t.Fatal(err)
	}
	if replay.Status != "already-current" || replay.IsMutation ||
		replay.Applied || !replay.Replay || !replay.AlreadyCurrent ||
		replay.PlanSHA256 != plan.ExpectedPlanSHA256 || replay.Receipt == nil {
		t.Fatalf("unexpected current sync strict replay result: %+v", replay)
	}
}
