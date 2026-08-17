package sync

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/shuiyu486/re-context-kits/internal/rekit/projectstate"
)

func TestInspectCurrentSyncPendingOwnershipClassifiesTransactionHistory(t *testing.T) {
	t.Run("archived-only-publication-window", func(t *testing.T) {
		fixture, _, intent := newCurrentSyncApplyArtifactsFixture(t)
		writeCurrentSyncCanonicalTest(
			t,
			filepath.Join(
				fixture.caseRoot,
				projectstate.CurrentDir,
				filepath.FromSlash(currentSyncArchivedIntentPath(intent)),
			),
			intent,
		)
		ownership, err := inspectCurrentSyncPendingOwnership(
			fixture.caseRoot,
			filepath.Join(fixture.caseRoot, projectstate.CurrentDir),
		)
		if err != nil || !ownership.Exists ||
			!currentSyncCanonicalEqual(ownership.Intent, intent) {
			t.Fatalf("archived-only current sync owner = %+v err=%v", ownership, err)
		}
	})

	t.Run("terminal-history-is-not-pending", func(t *testing.T) {
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
		ownership, err := inspectCurrentSyncPendingOwnership(
			fixture.caseRoot,
			filepath.Join(fixture.caseRoot, projectstate.CurrentDir),
		)
		if err != nil || ownership.Exists {
			t.Fatalf("terminal current sync history became a pending owner: %+v err=%v transaction=%s", ownership, err, transaction.plan.ExpectedPlanSHA256)
		}
	})

	t.Run("active-owner-coexists-with-terminal-history", func(t *testing.T) {
		fixture := newCurrentSyncFixture(t, "")
		publishCommittedCurrentSyncReplayFixture(t, fixture)
		for _, rel := range []string{currentSyncIntentRel, currentSyncOwnerRel} {
			if err := os.Remove(filepath.Join(
				fixture.caseRoot,
				projectstate.CurrentDir,
				filepath.FromSlash(rel),
			)); err != nil {
				t.Fatal(err)
			}
		}

		if err := os.WriteFile(
			filepath.Join(fixture.repoRoot, "common", "policies", "second-refresh.md"),
			[]byte("second refresh\n"),
			0o644,
		); err != nil {
			t.Fatal(err)
		}
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
		if _, err := publishCurrentSyncIntent(fixture.caseRoot, intent); err != nil {
			t.Fatal(err)
		}
		ownership, err := inspectCurrentSyncPendingOwnership(
			fixture.caseRoot,
			filepath.Join(fixture.caseRoot, projectstate.CurrentDir),
		)
		if err != nil || !ownership.Exists ||
			!currentSyncCanonicalEqual(ownership.Intent, intent) {
			t.Fatalf("current sync active owner with terminal history = %+v err=%v", ownership, err)
		}
	})

	t.Run("orphan-nonterminal-journal-fails-closed", func(t *testing.T) {
		fixture, _, intent := newCurrentSyncApplyArtifactsFixture(t)
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
		_, err = inspectCurrentSyncPendingOwnership(
			fixture.caseRoot,
			filepath.Join(fixture.caseRoot, projectstate.CurrentDir),
		)
		if err == nil || !strings.Contains(err.Error(), "without its active intent") {
			t.Fatalf("orphan nonterminal current sync journal error = %v", err)
		}
	})

	t.Run("owner-with-unrelated-nonterminal-journal-fails-closed", func(t *testing.T) {
		fixture := newCurrentSyncFixture(t, "")
		first := publishCommittedCurrentSyncReplayFixture(t, fixture)
		for _, rel := range []string{currentSyncIntentRel, currentSyncOwnerRel} {
			if err := os.Remove(filepath.Join(
				fixture.caseRoot,
				projectstate.CurrentDir,
				filepath.FromSlash(rel),
			)); err != nil {
				t.Fatal(err)
			}
		}
		if err := os.WriteFile(
			filepath.Join(fixture.repoRoot, "common", "policies", "second-refresh.md"),
			[]byte("second refresh\n"),
			0o644,
		); err != nil {
			t.Fatal(err)
		}
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
		if err := os.Remove(filepath.Join(
			fixture.caseRoot,
			projectstate.CurrentDir,
			filepath.FromSlash(currentSyncProgressPath(
				first.intent,
				first.committed.Generation,
			)),
		)); err != nil {
			t.Fatal(err)
		}
		if _, err := publishCurrentSyncIntent(fixture.caseRoot, intent); err != nil {
			t.Fatal(err)
		}
		_, err = inspectCurrentSyncPendingOwnership(
			fixture.caseRoot,
			filepath.Join(fixture.caseRoot, projectstate.CurrentDir),
		)
		if err == nil || !strings.Contains(err.Error(), "unrelated nonterminal journal") {
			t.Fatalf("owner with unrelated nonterminal journal error = %v", err)
		}
	})
}
