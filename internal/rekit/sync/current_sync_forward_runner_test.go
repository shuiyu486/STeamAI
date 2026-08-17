package sync

import (
	"errors"
	"path/filepath"
	"strings"
	"testing"

	"github.com/shuiyu486/re-context-kits/internal/rekit/projectstate"
)

func TestRunCurrentSyncForwardPublishesAlternatingJournal(t *testing.T) {
	fixture, intent, progress := currentSyncForwardRunnerFixture(t)
	seen := []currentSyncProgressOperation{}
	terminal, err := runCurrentSyncForward(
		fixture.caseRoot,
		intent,
		progress,
		func(current currentSyncProgress, operation currentSyncProgressOperation) error {
			if current.Pending == nil ||
				!currentSyncCanonicalEqual(*current.Pending, operation) {
				t.Fatalf("current sync effect received no exact pending operation: progress=%+v operation=%+v", current, operation)
			}
			seen = append(seen, operation)
			return nil
		},
	)
	if err != nil {
		t.Fatal(err)
	}
	expected := currentSyncExpectedProgressOperations(intent)
	if !currentSyncCanonicalEqual(seen, expected) ||
		!currentSyncCanonicalEqual(terminal.Completed, expected) ||
		terminal.Pending != nil || terminal.Generation != 1+len(expected)*2 ||
		!currentSyncProgressTerminal(terminal, intent) {
		t.Fatalf("current sync terminal forward progress is invalid: %+v", terminal)
	}
	history, err := readCurrentSyncProgressHistory(
		filepath.Join(fixture.caseRoot, projectstate.CurrentDir),
		intent,
	)
	if err != nil {
		t.Fatal(err)
	}
	if len(history) != terminal.Generation ||
		!currentSyncCanonicalEqual(history[len(history)-1], terminal) {
		t.Fatalf("current sync durable forward history is incomplete: generations=%d terminal=%+v", len(history), terminal)
	}
	for index, item := range history {
		if item.Generation != index+1 {
			t.Fatalf("current sync forward generation %d = %d", index+1, item.Generation)
		}
		if index > 0 {
			if err := validateCurrentSyncProgressTransition(
				history[index-1],
				item,
				intent,
			); err != nil {
				t.Fatal(err)
			}
		}
	}
}

func TestRunCurrentSyncForwardLeavesFailedEffectPendingAndResumes(t *testing.T) {
	fixture, intent, progress := currentSyncForwardRunnerFixture(t)
	failed := errors.New("injected effect failure")
	calls := 0
	_, err := runCurrentSyncForward(
		fixture.caseRoot,
		intent,
		progress,
		func(currentSyncProgress, currentSyncProgressOperation) error {
			calls++
			return failed
		},
	)
	if !errors.Is(err, failed) || calls != 1 {
		t.Fatalf("current sync forward effect failure calls=%d err=%v", calls, err)
	}
	pending, exists, err := readCurrentSyncProgress(
		filepath.Join(fixture.caseRoot, projectstate.CurrentDir),
		intent,
	)
	if err != nil || !exists || pending.Pending == nil ||
		pending.Pending.Kind != "stage-validated" || len(pending.Completed) != 0 {
		t.Fatalf("current sync failed effect was not left pending: exists=%t progress=%+v err=%v", exists, pending, err)
	}
	resumedFirst := currentSyncProgressOperation{}
	terminal, err := runCurrentSyncForward(
		fixture.caseRoot,
		intent,
		pending,
		func(current currentSyncProgress, operation currentSyncProgressOperation) error {
			if resumedFirst.Kind == "" {
				resumedFirst = operation
			}
			return nil
		},
	)
	if err != nil {
		t.Fatal(err)
	}
	if resumedFirst.Kind != "stage-validated" ||
		!currentSyncProgressTerminal(terminal, intent) {
		t.Fatalf("current sync forward resume skipped pending operation: first=%+v terminal=%+v", resumedFirst, terminal)
	}
}

func TestRunCurrentSyncForwardRejectsMissingEffect(t *testing.T) {
	fixture, intent, progress := currentSyncForwardRunnerFixture(t)
	if _, err := runCurrentSyncForward(
		fixture.caseRoot,
		intent,
		progress,
		nil,
	); err == nil || !strings.Contains(err.Error(), "effect is missing") {
		t.Fatalf("current sync forward accepted missing effect: %v", err)
	}
}

func currentSyncForwardRunnerFixture(
	t *testing.T,
) (currentSyncFixtureState, currentSyncIntent, currentSyncProgress) {
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
	if _, err := publishCurrentSyncIntent(fixture.caseRoot, intent); err != nil {
		t.Fatal(err)
	}
	progress, err := newCurrentSyncProgress(intent)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := publishCurrentSyncProgress(
		fixture.caseRoot,
		intent,
		progress,
	); err != nil {
		t.Fatal(err)
	}
	return fixture, intent, progress
}
