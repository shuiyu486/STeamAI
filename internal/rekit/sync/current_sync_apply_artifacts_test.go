package sync

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/shuiyu486/re-context-kits/internal/rekit/projectstate"
)

func TestReadCurrentSyncApplySnapshotSelectsDurableRoutes(t *testing.T) {
	t.Run("fresh", func(t *testing.T) {
		fixture := newCurrentSyncFixture(t, "")
		plan, err := CurrentSyncPreview(
			fixture.repoRoot,
			fixture.caseRoot,
			fixture.pack,
			CurrentSyncOptions{SourceExecutable: fixture.sourceExecutable},
		)
		if err != nil {
			t.Fatal(err)
		}
		snapshot, err := readCurrentSyncApplySnapshot(
			fixture.caseRoot,
			filepath.Join(fixture.caseRoot, projectstate.CurrentDir),
			plan.ExpectedPlanSHA256,
		)
		if err != nil || snapshot.Route != currentSyncApplyFresh {
			t.Fatalf("current sync fresh snapshot = %+v err=%v", snapshot, err)
		}
	})

	t.Run("archived-publication-window", func(t *testing.T) {
		fixture, plan, intent := newCurrentSyncApplyArtifactsFixture(t)
		writeCurrentSyncCanonicalTest(
			t,
			filepath.Join(
				fixture.caseRoot,
				projectstate.CurrentDir,
				filepath.FromSlash(currentSyncArchivedIntentPath(intent)),
			),
			intent,
		)
		snapshot, err := readCurrentSyncApplySnapshot(
			fixture.caseRoot,
			filepath.Join(fixture.caseRoot, projectstate.CurrentDir),
			plan.ExpectedPlanSHA256,
		)
		if err != nil || snapshot.Route != currentSyncApplyRestoreActive ||
			!currentSyncCanonicalEqual(snapshot.Intent, intent) {
			t.Fatalf("current sync archived snapshot = %+v err=%v", snapshot, err)
		}
	})

	t.Run("owner-publication-window", func(t *testing.T) {
		fixture, plan, intent := newCurrentSyncApplyArtifactsFixture(t)
		writeCurrentSyncCanonicalTest(
			t,
			filepath.Join(
				fixture.caseRoot,
				projectstate.CurrentDir,
				filepath.FromSlash(currentSyncOwnerRel),
			),
			currentSyncOwnerFor(intent),
		)
		snapshot, err := readCurrentSyncApplySnapshot(
			fixture.caseRoot,
			filepath.Join(fixture.caseRoot, projectstate.CurrentDir),
			plan.ExpectedPlanSHA256,
		)
		if err != nil || snapshot.Route != currentSyncApplyRestoreActive ||
			!currentSyncCanonicalEqual(snapshot.Intent, intent) {
			t.Fatalf("current sync owner snapshot = %+v err=%v", snapshot, err)
		}
	})

	t.Run("owner-and-archive-publication-window", func(t *testing.T) {
		fixture, plan, intent := newCurrentSyncApplyArtifactsFixture(t)
		if _, err := publishCurrentSyncIntent(fixture.caseRoot, intent); err != nil {
			t.Fatal(err)
		}
		if err := os.Remove(filepath.Join(
			fixture.caseRoot,
			projectstate.CurrentDir,
			filepath.FromSlash(currentSyncIntentRel),
		)); err != nil {
			t.Fatal(err)
		}
		snapshot, err := readCurrentSyncApplySnapshot(
			fixture.caseRoot,
			filepath.Join(fixture.caseRoot, projectstate.CurrentDir),
			plan.ExpectedPlanSHA256,
		)
		if err != nil || snapshot.Route != currentSyncApplyRestoreActive ||
			!currentSyncCanonicalEqual(snapshot.Intent, intent) {
			t.Fatalf("current sync owner/archive snapshot = %+v err=%v", snapshot, err)
		}
	})

	t.Run("active-before-journal-restores", func(t *testing.T) {
		fixture, plan, intent := newCurrentSyncApplyArtifactsFixture(t)
		if _, err := publishCurrentSyncIntent(fixture.caseRoot, intent); err != nil {
			t.Fatal(err)
		}
		snapshot, err := readCurrentSyncApplySnapshot(
			fixture.caseRoot,
			filepath.Join(fixture.caseRoot, projectstate.CurrentDir),
			plan.ExpectedPlanSHA256,
		)
		if err != nil || snapshot.Route != currentSyncApplyRestoreActive ||
			!currentSyncCanonicalEqual(snapshot.Intent, intent) {
			t.Fatalf("current sync pre-journal snapshot = %+v err=%v", snapshot, err)
		}
	})

	t.Run("active-resume", func(t *testing.T) {
		fixture, plan, intent := newCurrentSyncApplyArtifactsFixture(t)
		if _, err := publishCurrentSyncIntent(fixture.caseRoot, intent); err != nil {
			t.Fatal(err)
		}
		progress, err := newCurrentSyncProgress(intent)
		if err != nil {
			t.Fatal(err)
		}
		if _, err := publishCurrentSyncProgress(fixture.caseRoot, intent, progress); err != nil {
			t.Fatal(err)
		}
		snapshot, err := readCurrentSyncApplySnapshot(
			fixture.caseRoot,
			filepath.Join(fixture.caseRoot, projectstate.CurrentDir),
			plan.ExpectedPlanSHA256,
		)
		if err != nil || snapshot.Route != currentSyncApplyResume ||
			!currentSyncCanonicalEqual(snapshot.Progress, progress) {
			t.Fatalf("current sync resume snapshot = %+v err=%v", snapshot, err)
		}
	})

	t.Run("terminal-cleanup", func(t *testing.T) {
		fixture := newCurrentSyncFixture(t, "")
		transaction := publishCommittedCurrentSyncReplayFixture(t, fixture)
		snapshot, err := readCurrentSyncApplySnapshot(
			fixture.caseRoot,
			filepath.Join(fixture.caseRoot, projectstate.CurrentDir),
			transaction.plan.ExpectedPlanSHA256,
		)
		if err != nil || snapshot.Route != currentSyncApplyCleanup ||
			!currentSyncCanonicalEqual(snapshot.Receipt, transaction.receipt) ||
			!currentSyncProgressTerminal(snapshot.Progress, snapshot.Intent) {
			t.Fatalf("current sync cleanup snapshot = %+v err=%v", snapshot, err)
		}
	})

	t.Run("lost-response-replay", func(t *testing.T) {
		fixture := newCurrentSyncFixture(t, "")
		transaction := publishCommittedCurrentSyncReplayFixture(t, fixture)
		for _, rel := range []string{currentSyncIntentRel, currentSyncOwnerRel} {
			if err := os.Remove(filepath.Join(
				fixture.caseRoot,
				projectstate.CurrentDir,
				filepath.FromSlash(rel),
			)); err != nil {
				t.Fatal(err)
			}
		}
		snapshot, err := readCurrentSyncApplySnapshot(
			fixture.caseRoot,
			filepath.Join(fixture.caseRoot, projectstate.CurrentDir),
			transaction.plan.ExpectedPlanSHA256,
		)
		if err != nil || snapshot.Route != currentSyncApplyReplay {
			t.Fatalf("current sync replay snapshot = %+v err=%v", snapshot, err)
		}
	})
}

func TestReadCurrentSyncApplySnapshotRejectsConflictingArtifacts(t *testing.T) {
	t.Run("expected-plan", func(t *testing.T) {
		fixture := newCurrentSyncFixture(t, "")
		if _, err := readCurrentSyncApplySnapshot(
			fixture.caseRoot,
			filepath.Join(fixture.caseRoot, projectstate.CurrentDir),
			"not-a-sha",
		); err == nil {
			t.Fatal("current sync accepted invalid expected plan SHA-256")
		}
	})

	t.Run("active-other-plan", func(t *testing.T) {
		fixture, plan, intent := newCurrentSyncApplyArtifactsFixture(t)
		if _, err := publishCurrentSyncIntent(fixture.caseRoot, intent); err != nil {
			t.Fatal(err)
		}
		if _, err := readCurrentSyncApplySnapshot(
			fixture.caseRoot,
			filepath.Join(fixture.caseRoot, projectstate.CurrentDir),
			strings.Repeat("f", 64),
		); err == nil || !strings.Contains(err.Error(), "active intent") {
			t.Fatalf("current sync active plan conflict error = %v plan=%s", err, plan.ExpectedPlanSHA256)
		}
	})

	t.Run("orphan-progress", func(t *testing.T) {
		fixture, plan, intent := newCurrentSyncApplyArtifactsFixture(t)
		writeCurrentSyncCanonicalTest(
			t,
			filepath.Join(
				fixture.caseRoot,
				projectstate.CurrentDir,
				filepath.FromSlash(currentSyncArchivedIntentPath(intent)),
			),
			intent,
		)
		progress, err := newCurrentSyncProgress(intent)
		if err != nil {
			t.Fatal(err)
		}
		if _, err := publishCurrentSyncProgress(fixture.caseRoot, intent, progress); err != nil {
			t.Fatal(err)
		}
		if _, err := readCurrentSyncApplySnapshot(
			fixture.caseRoot,
			filepath.Join(fixture.caseRoot, projectstate.CurrentDir),
			plan.ExpectedPlanSHA256,
		); err == nil || !strings.Contains(err.Error(), "without its active intent") {
			t.Fatalf("current sync orphan progress error = %v", err)
		}
	})
}

func newCurrentSyncApplyArtifactsFixture(
	t *testing.T,
) (currentSyncFixtureState, CurrentSyncPlan, currentSyncIntent) {
	t.Helper()
	fixture := newCurrentSyncFixture(t, "")
	plan, err := CurrentSyncPreview(
		fixture.repoRoot,
		fixture.caseRoot,
		fixture.pack,
		CurrentSyncOptions{SourceExecutable: fixture.sourceExecutable},
	)
	if err != nil {
		t.Fatal(err)
	}
	intent, err := buildCurrentSyncIntent(plan)
	if err != nil {
		t.Fatal(err)
	}
	return fixture, plan, intent
}
