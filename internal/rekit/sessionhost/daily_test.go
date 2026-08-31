package sessionhost

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"maps"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"slices"
	"strings"
	"testing"
	"time"

	"github.com/shuiyu486/re-context-kits/internal/rekit/cli"
	"github.com/shuiyu486/re-context-kits/internal/rekit/defaults"
	rekitfs "github.com/shuiyu486/re-context-kits/internal/rekit/fs"
	"github.com/shuiyu486/re-context-kits/internal/rekit/lanecompletion"
	"github.com/shuiyu486/re-context-kits/internal/rekit/laneowner"
	"github.com/shuiyu486/re-context-kits/internal/rekit/memberexecution"
	"github.com/shuiyu486/re-context-kits/internal/rekit/mission"
	"github.com/shuiyu486/re-context-kits/internal/rekit/missionintent"
	"github.com/shuiyu486/re-context-kits/internal/rekit/missionsuccessor"
	"github.com/shuiyu486/re-context-kits/internal/rekit/note"
	"github.com/shuiyu486/re-context-kits/internal/rekit/projectexecution"
	"github.com/shuiyu486/re-context-kits/internal/rekit/projectstate"
	rekitruntime "github.com/shuiyu486/re-context-kits/internal/rekit/runtime"
	syncreview "github.com/shuiyu486/re-context-kits/internal/rekit/sync"
	"github.com/shuiyu486/re-context-kits/internal/rekit/workstream"
)

func TestRunDailyRejectsGoalAndCorrectionTogether(t *testing.T) {
	result, err := RunDaily(context.Background(), DailyOptions{Target: filepath.Join(t.TempDir(), "case"), Goal: "goal", Correction: "correction"})
	if err == nil || !strings.Contains(err.Error(), "either -goal or -correction") || result.Mode != "correction" {
		t.Fatalf("RunDaily result=%+v err=%v", result, err)
	}
	if result.Action == nil || result.Action.Code != DailyActionFailed || result.Action.RequiresInput {
		t.Fatalf("RunDaily failure action = %+v", result.Action)
	}
}

func TestRunDailyCaseReadyHookCapturesFreshOnboardingFailure(t *testing.T) {
	caseRoot := filepath.Join(t.TempDir(), "fresh-hook")
	unifiedExecutable := buildSessionhostUnifiedRuntimeFixture(t, sessionhostTestRepoRoot(t))
	calls := 0
	result, err := RunDaily(context.Background(), DailyOptions{
		Target:                            caseRoot,
		Goal:                              "capture fresh partial onboarding",
		InitializationSourceExecutable:    unifiedExecutable,
		ClaudePath:                        missingClaudePath(t),
		ExpectedClaudeExecutableSHA256:    strings.Repeat("0", 64),
		ExpectedClaudeExecutablePublisher: liveAcceptanceClaudePublisher,
		Input:                             DailyInputRequest{Mode: DailyInputWorkspaceInventory, Scope: "."},
		onCaseReady: func(root string) error {
			calls++
			if root != caseRoot {
				t.Fatalf("fresh hook root=%s want=%s", root, caseRoot)
			}
			if info, statErr := os.Lstat(root); statErr != nil || !info.IsDir() {
				t.Fatalf("fresh hook observed invalid root: info=%v err=%v", info, statErr)
			}
			return nil
		},
	})
	if err == nil || !strings.Contains(err.Error(), "validate trusted Claude Code executable") {
		t.Fatalf("fresh hook result=%+v err=%v", result, err)
	}
	if calls != 1 {
		t.Fatalf("fresh hook calls=%d want=1", calls)
	}
}

func TestRunDailyCaseReadyHookCapturesExistingCaseBeforeMutation(t *testing.T) {
	caseRoot := filepath.Join(t.TempDir(), "existing-hook")
	bootstrap := DailyResult{CaseRoot: caseRoot}
	inspection, err := applyDailyOnboarding(caseRoot, "existing hook goal", "daily-test", &bootstrap)
	if err != nil {
		t.Fatal(err)
	}
	calls := 0
	result, err := RunDaily(context.Background(), DailyOptions{
		Target:                            caseRoot,
		Goal:                              inspection.Identity.Goal,
		ClaudePath:                        missingClaudePath(t),
		ExpectedClaudeExecutableSHA256:    strings.Repeat("0", 64),
		ExpectedClaudeExecutablePublisher: liveAcceptanceClaudePublisher,
		Input:                             DailyInputRequest{Mode: DailyInputWorkspaceInventory, Scope: "."},
		onCaseReady: func(root string) error {
			calls++
			if root != caseRoot {
				t.Fatalf("existing hook root=%s want=%s", root, caseRoot)
			}
			return nil
		},
	})
	if err == nil || !strings.Contains(err.Error(), "validate trusted Claude Code executable") {
		t.Fatalf("existing hook result=%+v err=%v", result, err)
	}
	if calls != 1 {
		t.Fatalf("existing hook calls=%d want=1", calls)
	}
}

func TestRunDailyRejectsUnboundCustomClaudeBeforeMutation(t *testing.T) {
	caseRoot := filepath.Join(t.TempDir(), "unbound")
	result, err := RunDaily(context.Background(), DailyOptions{Target: caseRoot, Goal: "goal", ClaudePath: missingClaudePath(t)})
	if err == nil || !strings.Contains(err.Error(), "refuses a custom or unbound Claude executable") {
		t.Fatalf("unbound custom Claude result=%+v err=%v", result, err)
	}
	if result.OnboardingApplied || result.SessionLaunches != 0 {
		t.Fatalf("unbound custom Claude mutated case: %+v", result)
	}
	if result.Failure == nil || result.Failure.Code != "claude-executable-unavailable" || result.Failure.Stage != "executable-resolution" || !result.Failure.Recoverable || result.Failure.MutationApplied || result.Failure.MutationBoundary != "none" {
		t.Fatalf("unbound custom Claude diagnosis = %+v", result.Failure)
	}
	if _, err := os.Lstat(caseRoot); !os.IsNotExist(err) {
		t.Fatalf("unbound custom Claude created case: %v", err)
	}
}

func TestRunDailyFreshGoalCommitsAndStartsBeforeClaudeUnavailable(t *testing.T) {
	caseRoot := filepath.Join(t.TempDir(), "fresh")
	unifiedExecutable := buildSessionhostUnifiedRuntimeFixture(t, sessionhostTestRepoRoot(t))
	result, err := RunDaily(context.Background(), DailyOptions{Target: caseRoot, Goal: "inspect the supplied research target", InitializationSourceExecutable: unifiedExecutable, ClaudePath: missingClaudePath(t), ExpectedClaudeExecutableSHA256: strings.Repeat("0", 64), ExpectedClaudeExecutablePublisher: liveAcceptanceClaudePublisher, Input: DailyInputRequest{Mode: DailyInputWorkspaceInventory, Scope: "."}})
	if err == nil || !strings.Contains(err.Error(), "validate trusted Claude Code executable") {
		t.Fatalf("RunDaily result=%+v err=%v", result, err)
	}
	if !result.OnboardingApplied || result.Pack != defaults.DefaultPack || result.Lane != "binary-analysis-mission" || result.SessionLaunches != 0 {
		t.Fatalf("fresh daily result = %+v", result)
	}
	if result.Failure == nil || result.Failure.Code != "claude-executable-unavailable" || result.Failure.Stage != "executable-resolution" || !result.Failure.Recoverable || !result.Failure.MutationApplied || result.Failure.MutationBoundary != "durable-runtime-step-may-have-committed" {
		t.Fatalf("fresh daily executable diagnosis = %+v", result.Failure)
	}
	inspection, inspectErr := missionintent.Inspect(caseRoot)
	if inspectErr != nil || !inspection.Committed || inspection.Identity.Goal != "inspect the supplied research target" {
		t.Fatalf("fresh daily inspection=%+v err=%v", inspection, inspectErr)
	}
	board, boardErr := mission.ReadBoard(caseRoot)
	if boardErr != nil {
		t.Fatal(boardErr)
	}
	lane, ok := mission.LookupBoardLane(board.Lanes, result.Lane, false)
	if !ok || lane.ExecutorGeneration != 1 || lane.CurrentExecutor == "" {
		t.Fatalf("fresh daily lane = %+v", lane)
	}
}

func TestRunDailyResumeBootstrapsCommittedLaneBeforeRequiringTypedInput(t *testing.T) {
	caseRoot := filepath.Join(t.TempDir(), "resume")
	bootstrap := DailyResult{CaseRoot: caseRoot}
	inspection, err := applyDailyOnboarding(caseRoot, "resume goal", "daily-test", &bootstrap)
	if err != nil {
		t.Fatal(err)
	}
	result, err := RunDaily(context.Background(), DailyOptions{Target: caseRoot, ClaudePath: missingClaudePath(t), ExpectedClaudeExecutableSHA256: strings.Repeat("0", 64), ExpectedClaudeExecutablePublisher: liveAcceptanceClaudePublisher})
	if err != nil {
		t.Fatalf("resume result=%+v err=%v", result, err)
	}
	if result.Mode != "resume" || result.Pack != inspection.Identity.Pack || result.Lane != inspection.Identity.InitialLane || result.FinalState != DailyActionInputRequired || !result.Blocked || result.SessionLaunches != 0 || !containsDailyStep(result.DriverSteps, "overview") || !containsDailyStep(result.DriverSteps, "start") {
		t.Fatalf("resume result=%+v", result)
	}
}

func TestRunDailyMultipleLanesRequiresChoiceWithoutLaunchOrWrite(t *testing.T) {
	caseRoot := filepath.Join(t.TempDir(), "multiple-lanes")
	bootstrap := DailyResult{CaseRoot: caseRoot}
	inspection, err := applyDailyOnboarding(caseRoot, "multiple lane goal", "daily-test", &bootstrap)
	if err != nil {
		t.Fatal(err)
	}
	bootstrap.Lane = inspection.Identity.InitialLane
	if err := ensureDailyStarted(caseRoot, inspection.Identity.Pack, &bootstrap); err != nil {
		t.Fatal(err)
	}
	if _, err := workstream.StartApply(sessionhostAttachedRepoRoot(t, caseRoot, inspection.Identity.Pack), caseRoot, inspection.Identity.Pack, workstream.StartOptions{
		Name: "login", Actor: "daily-test", Executor: "session-login", TakeoverReason: "daily multi-lane choice regression",
	}); err != nil {
		t.Fatal(err)
	}
	before := snapshotDailyCaseFiles(t, caseRoot)
	memberRunCalled := false
	result, err := RunDaily(context.Background(), DailyOptions{
		Target:                            caseRoot,
		ClaudePath:                        missingClaudePath(t),
		ExpectedClaudeExecutableSHA256:    strings.Repeat("0", 64),
		ExpectedClaudeExecutablePublisher: liveAcceptanceClaudePublisher,
		beforeMemberRun: func(string, string, string) error {
			memberRunCalled = true
			return nil
		},
	})
	if err != nil {
		t.Fatalf("multi-lane choice should stop without executable resolution: result=%+v err=%v", result, err)
	}
	if result.Action == nil || result.Action.Code != DailyActionBlocked || !result.Action.RequiresInput || len(result.Action.Choices) < 2 {
		t.Fatalf("multi-lane choice action=%+v result=%+v", result.Action, result)
	}
	ids := make([]string, 0, len(result.Action.Choices))
	seen := map[string]bool{}
	for _, choice := range result.Action.Choices {
		if choice.ID == "" || seen[choice.ID] {
			t.Fatalf("multi-lane choices contain duplicate or empty id: %+v", result.Action.Choices)
		}
		seen[choice.ID] = true
		ids = append(ids, choice.ID)
	}
	if !slices.Contains(ids, "binary-analysis-login") {
		t.Fatalf("multi-lane choices lost started lane: %+v", result.Action.Choices)
	}
	if memberRunCalled || result.SessionLaunches != 0 || len(result.HostRuns) != 0 {
		t.Fatalf("multi-lane choice crossed the zero-launch boundary: called=%t result=%+v", memberRunCalled, result)
	}
	assertDailyCaseFilesEqual(t, before, snapshotDailyCaseFiles(t, caseRoot))
}

func TestRunDailyCorrectionMultipleLanesRequiresChoiceWithoutWrite(t *testing.T) {
	caseRoot := filepath.Join(t.TempDir(), "multiple-lane-correction")
	bootstrap := DailyResult{CaseRoot: caseRoot}
	inspection, err := applyDailyOnboarding(caseRoot, "multiple lane correction goal", "daily-test", &bootstrap)
	if err != nil {
		t.Fatal(err)
	}
	bootstrap.Lane = inspection.Identity.InitialLane
	if err := ensureDailyStarted(caseRoot, inspection.Identity.Pack, &bootstrap); err != nil {
		t.Fatal(err)
	}
	if _, err := workstream.StartApply(sessionhostAttachedRepoRoot(t, caseRoot, inspection.Identity.Pack), caseRoot, inspection.Identity.Pack, workstream.StartOptions{
		Name: "login", Actor: "daily-test", Executor: "session-login", TakeoverReason: "daily correction multi-lane choice regression",
	}); err != nil {
		t.Fatal(err)
	}

	before := snapshotDailyCaseFiles(t, caseRoot)
	result, err := RunDaily(context.Background(), DailyOptions{
		Target:                            caseRoot,
		Correction:                        "apply this only after I select the exact lane",
		ClaudePath:                        missingClaudePath(t),
		ExpectedClaudeExecutableSHA256:    strings.Repeat("0", 64),
		ExpectedClaudeExecutablePublisher: liveAcceptanceClaudePublisher,
	})
	if err != nil {
		t.Fatalf("multi-lane correction should stop before lineage or executable resolution: result=%+v err=%v", result, err)
	}
	if result.Action == nil || result.Action.Code != DailyActionBlocked || !result.Action.RequiresInput || len(result.Action.Choices) < 2 {
		t.Fatalf("multi-lane correction choice action=%+v result=%+v", result.Action, result)
	}
	ids := make([]string, 0, len(result.Action.Choices))
	for _, choice := range result.Action.Choices {
		ids = append(ids, choice.ID)
	}
	if !slices.Contains(ids, "binary-analysis-login") || result.SessionLaunches != 0 || len(result.HostRuns) != 0 || len(result.DriverSteps) != 0 {
		t.Fatalf("multi-lane correction crossed the zero-write/launch boundary: choices=%+v result=%+v", result.Action.Choices, result)
	}
	assertDailyCaseFilesEqual(t, before, snapshotDailyCaseFiles(t, caseRoot))
}

func TestRunDailyActiveCorrectionRequiresExplicitLaneWithoutWrite(t *testing.T) {
	caseRoot := filepath.Join(t.TempDir(), "single-active-correction-choice")
	bootstrap := DailyResult{CaseRoot: caseRoot}
	inspection, err := applyDailyOnboarding(caseRoot, "single active correction goal", "daily-test", &bootstrap)
	if err != nil {
		t.Fatal(err)
	}
	bootstrap.Lane = inspection.Identity.InitialLane
	if err := ensureDailyStarted(caseRoot, inspection.Identity.Pack, &bootstrap); err != nil {
		t.Fatal(err)
	}
	before := snapshotDailyCaseFiles(t, caseRoot)
	result, err := RunDaily(context.Background(), DailyOptions{
		Target:     caseRoot,
		Correction: "change the active lane approach only after explicit selection",
		Actor:      "daily-test",
	})
	if err != nil {
		t.Fatalf("active correction choice result=%+v err=%v", result, err)
	}
	if result.Action == nil || result.Action.Code != DailyActionBlocked ||
		!result.Action.RequiresInput || len(result.Action.Choices) != 1 ||
		result.Action.Choices[0].ID != bootstrap.Lane || result.SessionLaunches != 0 ||
		len(result.HostRuns) != 0 || len(result.DriverSteps) != 0 {
		t.Fatalf("single active correction did not require exact lane selection: %+v", result)
	}
	assertDailyCaseFilesEqual(t, before, snapshotDailyCaseFiles(t, caseRoot))
}

func TestRunDailyActiveCorrectionAdvancesSelectedOwnerGenerationWithoutLaunch(t *testing.T) {
	caseRoot := filepath.Join(t.TempDir(), "selected-active-correction")
	actor := "daily-test"
	correction := "replace the current approach with the bounded corrected approach"
	bootstrap := DailyResult{CaseRoot: caseRoot}
	inspection, err := applyDailyOnboarding(caseRoot, "selected active correction goal", actor, &bootstrap)
	if err != nil {
		t.Fatal(err)
	}
	bootstrap.Lane = inspection.Identity.InitialLane
	if err := ensureDailyStarted(caseRoot, inspection.Identity.Pack, &bootstrap); err != nil {
		t.Fatal(err)
	}
	board, err := mission.ReadBoard(caseRoot)
	if err != nil {
		t.Fatal(err)
	}
	before, ok := mission.LookupBoardLane(board.Lanes, bootstrap.Lane, false)
	if !ok || before.CurrentExecutor == "" || before.ExecutorGeneration < 1 {
		t.Fatalf("active correction fixture has no current owner: %+v", before)
	}

	memberRunCalled := false
	result, err := RunDaily(context.Background(), DailyOptions{
		Target: caseRoot, Correction: correction, SelectedLane: bootstrap.Lane, Actor: actor,
		beforeMemberRun: func(_, _, _ string) error { memberRunCalled = true; return nil },
	})
	if err != nil {
		t.Fatalf("active correction result=%+v err=%v", result, err)
	}
	if memberRunCalled || result.Lane != bootstrap.Lane ||
		result.FinalState != "active-correction-recorded" || result.Action == nil ||
		result.Action.Code != DailyActionReadyToContinue || result.CorrectionEventID == "" ||
		result.ExecutorGeneration != before.ExecutorGeneration+1 || result.SessionLaunches != 0 ||
		len(result.HostRuns) != 0 || !containsDailyStep(result.DriverSteps, "note-active-correction") ||
		!containsDailyStep(result.DriverSteps, "reconcile") {
		t.Fatalf("active correction crossed its bounded generation fence: called=%t result=%+v", memberRunCalled, result)
	}
	board, err = mission.ReadBoard(caseRoot)
	if err != nil {
		t.Fatal(err)
	}
	after, ok := mission.LookupBoardLane(board.Lanes, bootstrap.Lane, false)
	if !ok || after.CurrentExecutor != before.CurrentExecutor ||
		after.ExecutorGeneration != before.ExecutorGeneration+1 ||
		after.LastReconciledIntervention != result.CorrectionEventID || after.Authority {
		t.Fatalf("active correction changed the wrong lane authority/owner state: before=%+v after=%+v", before, after)
	}
	items, err := mission.ReadStrictFact(caseRoot, "intervention")
	if err != nil {
		t.Fatal(err)
	}
	eventCount := 0
	resolutionCount := 0
	for _, item := range items {
		if mission.Value(item, "eventId") == result.CorrectionEventID {
			eventCount++
			if mission.Value(item, "subject") != dailyActiveCorrectionSubject ||
				mission.Value(item, "ownerExecutor") != before.CurrentExecutor ||
				mission.Value(item, "ownerGeneration") != fmt.Sprint(before.ExecutorGeneration) {
				t.Fatalf("active correction event lost its superseded owner binding: %+v", item)
			}
		}
		if mission.Value(item, "resolvesEventId") == result.CorrectionEventID {
			resolutionCount++
		}
	}
	if eventCount != 1 || resolutionCount != 1 {
		t.Fatalf("active correction append/reconcile counts event=%d resolution=%d", eventCount, resolutionCount)
	}
	stale, err := workstream.ContinuePreview(
		sessionhostAttachedRepoRoot(t, caseRoot, inspection.Identity.Pack),
		caseRoot,
		inspection.Identity.Pack,
		workstream.ContinueOptions{
			Selector:                   bootstrap.Lane,
			Executor:                   before.CurrentExecutor,
			ExpectedExecutorGeneration: before.ExecutorGeneration,
		},
	)
	if err != nil || !stale.Blocked || stale.ContinueOwnerGuardRecovery == nil {
		t.Fatalf("superseded generation was not held by owner guard: result=%+v err=%v", stale, err)
	}
}

func TestRunDailyActiveCorrectionReplaysCommittedReconcileAfterRefreshFailure(t *testing.T) {
	caseRoot := filepath.Join(t.TempDir(), "active-correction-refresh-recovery")
	actor := "daily-test"
	correction := "recover the exact committed active correction"
	bootstrap := DailyResult{CaseRoot: caseRoot}
	inspection, err := applyDailyOnboarding(caseRoot, "active correction refresh recovery goal", actor, &bootstrap)
	if err != nil {
		t.Fatal(err)
	}
	bootstrap.Lane = inspection.Identity.InitialLane
	if err := ensureDailyStarted(caseRoot, inspection.Identity.Pack, &bootstrap); err != nil {
		t.Fatal(err)
	}
	board, err := mission.ReadBoard(caseRoot)
	if err != nil {
		t.Fatal(err)
	}
	before, ok := mission.LookupBoardLane(board.Lanes, bootstrap.Lane, false)
	if !ok {
		t.Fatal("active correction lane missing")
	}

	restoreRefreshHook := cli.SetDriverStepStatusRefreshHookForTest(func(command string) error {
		if command == "run-driver-step" {
			return errors.New("injected status refresh failure")
		}
		return nil
	})
	t.Cleanup(restoreRefreshHook)

	first, err := RunDaily(context.Background(), DailyOptions{
		Target: caseRoot, Correction: correction, SelectedLane: bootstrap.Lane, Actor: actor,
	})
	if err != nil {
		t.Fatalf("committed correction recovery result=%+v err=%v", first, err)
	}
	if !first.Replay || first.FinalState != "active-correction-recorded" ||
		first.ExecutorGeneration != before.ExecutorGeneration+1 ||
		!containsDailyStep(first.DriverSteps, "reconcile") {
		t.Fatalf("post-refresh failure was not recovered as committed: %+v", first)
	}
	restoreRefreshHook()

	second, err := RunDaily(context.Background(), DailyOptions{
		Target: caseRoot, Correction: correction, SelectedLane: bootstrap.Lane, Actor: actor,
	})
	if err != nil {
		t.Fatalf("active correction replay result=%+v err=%v", second, err)
	}
	if !second.Replay || second.CorrectionEventID != first.CorrectionEventID ||
		second.ExecutorGeneration != before.ExecutorGeneration+1 || len(second.DriverSteps) != 0 {
		t.Fatalf("same correction replay advanced generation or republished: first=%+v second=%+v", first, second)
	}
	items, err := mission.ReadStrictFact(caseRoot, "intervention")
	if err != nil {
		t.Fatal(err)
	}
	events, resolutions := 0, 0
	for _, item := range items {
		if mission.Value(item, "eventId") == first.CorrectionEventID {
			events++
		}
		if mission.Value(item, "resolvesEventId") == first.CorrectionEventID {
			resolutions++
		}
	}
	if events != 1 || resolutions != 1 {
		t.Fatalf("active correction recovery duplicated durable events: events=%d resolutions=%d", events, resolutions)
	}
}

func TestRunDailyActiveCorrectionReconcilesExactEventWhenOtherInterventionIsOpen(t *testing.T) {
	caseRoot := filepath.Join(t.TempDir(), "active-correction-other-intervention")
	actor := "daily-test"
	correction := "reconcile only this active correction"
	bootstrap := DailyResult{CaseRoot: caseRoot}
	inspection, err := applyDailyOnboarding(caseRoot, "active correction exact intervention goal", actor, &bootstrap)
	if err != nil {
		t.Fatal(err)
	}
	bootstrap.Lane = inspection.Identity.InitialLane
	if err := ensureDailyStarted(caseRoot, inspection.Identity.Pack, &bootstrap); err != nil {
		t.Fatal(err)
	}
	other, err := note.Append(
		sessionhostAttachedRepoRoot(t, caseRoot, inspection.Identity.Pack),
		caseRoot,
		inspection.Identity.Pack,
		note.Options{
			Kind: "intervention", Lane: bootstrap.Lane, Subject: "existing intervention",
			Action: "override", Status: "open", EventID: "existing-open-intervention",
		},
		false,
	)
	if err != nil || !other.Applied {
		t.Fatalf("append existing intervention result=%+v err=%v", other, err)
	}
	result, err := RunDaily(context.Background(), DailyOptions{
		Target: caseRoot, Correction: correction, SelectedLane: bootstrap.Lane, Actor: actor,
	})
	if err != nil {
		t.Fatalf("exact active correction result=%+v err=%v", result, err)
	}
	facts, err := mission.ReadStrictLedgerFacts(caseRoot)
	if err != nil {
		t.Fatal(err)
	}
	open := mission.EffectiveOpenLaneInterventions(facts.Facts, bootstrap.Lane)
	if len(open) != 1 || mission.Value(open[0], "eventId") != other.EventID ||
		result.FinalState != "active-correction-recorded" {
		t.Fatalf("active correction reconciled the wrong intervention: result=%+v open=%+v", result, open)
	}
}

func TestRunDailyActiveCorrectionRejectsOwnerDriftAfterPreviewWithoutAppend(t *testing.T) {
	caseRoot := filepath.Join(t.TempDir(), "stale-active-correction")
	bootstrap := DailyResult{CaseRoot: caseRoot}
	inspection, err := applyDailyOnboarding(caseRoot, "stale active correction goal", "daily-test", &bootstrap)
	if err != nil {
		t.Fatal(err)
	}
	bootstrap.Lane = inspection.Identity.InitialLane
	if err := ensureDailyStarted(caseRoot, inspection.Identity.Pack, &bootstrap); err != nil {
		t.Fatal(err)
	}
	previous := dailyActiveCorrectionAfterPreviewHook
	dailyActiveCorrectionAfterPreviewHook = func() error {
		board, err := mission.ReadBoard(caseRoot)
		if err != nil {
			return err
		}
		for index := range board.Lanes {
			if board.Lanes[index].ID == bootstrap.Lane {
				board.Lanes[index].ExecutorGeneration++
			}
		}
		writeDailyTestBoard(t, caseRoot, board)
		return nil
	}
	t.Cleanup(func() { dailyActiveCorrectionAfterPreviewHook = previous })
	result, err := RunDaily(context.Background(), DailyOptions{
		Target: caseRoot, Correction: "apply only while the reviewed owner is current",
		SelectedLane: bootstrap.Lane, Actor: "daily-test",
	})
	if err == nil || !strings.Contains(err.Error(), "owner generation changed after preview") {
		t.Fatalf("stale active correction result=%+v err=%v", result, err)
	}
	if result.CorrectionEventID == "" || len(result.DriverSteps) != 0 || result.SessionLaunches != 0 {
		t.Fatalf("stale active correction reported a durable correction mutation: %+v", result)
	}
	items, err := mission.ReadStrictFact(caseRoot, "intervention")
	if err != nil {
		t.Fatal(err)
	}
	for _, item := range items {
		if mission.Value(item, "eventId") == result.CorrectionEventID {
			t.Fatalf("stale active correction appended its event: %+v", item)
		}
	}
}

func TestRunDailyResolvedLaneBindsTypedInputPreparation(t *testing.T) {
	caseRoot := filepath.Join(t.TempDir(), "resolved-lane-binding")
	bootstrap := DailyResult{CaseRoot: caseRoot}
	inspection, err := applyDailyOnboarding(caseRoot, "resolved lane binding goal", "daily-test", &bootstrap)
	if err != nil {
		t.Fatal(err)
	}
	bootstrap.Lane = inspection.Identity.InitialLane
	if err := ensureDailyStarted(caseRoot, inspection.Identity.Pack, &bootstrap); err != nil {
		t.Fatal(err)
	}
	board, err := mission.ReadBoard(caseRoot)
	if err != nil {
		t.Fatal(err)
	}
	lanes := make([]mission.BoardLane, 0, 1)
	for _, lane := range board.Lanes {
		if lane.ID == inspection.Identity.InitialLane {
			lanes = append(lanes, lane)
		}
	}
	if len(lanes) != 1 {
		t.Fatalf("initial lane missing from board: %+v", board.Lanes)
	}
	board.Lanes = lanes
	writeDailyTestBoard(t, caseRoot, board)
	if _, err := runPublicStatus(caseRoot, inspection.Identity.Pack, inspection.Identity.InitialLane); err != nil {
		global, globalErr := runPublicStatus(caseRoot, inspection.Identity.Pack, "")
		var request *publicDriverRequest
		if global.MissionControlRunbook != nil {
			request = global.MissionControlRunbook.CurrentDriverRequest
		}
		t.Fatalf("resolved lane fixture is not selectable: %v globalRequest=%+v globalErr=%v", err, request, globalErr)
	}

	preparedLane := ""
	result, err := RunDaily(context.Background(), DailyOptions{
		Target:                            caseRoot,
		ClaudePath:                        missingClaudePath(t),
		ExpectedClaudeExecutableSHA256:    strings.Repeat("0", 64),
		ExpectedClaudeExecutablePublisher: liveAcceptanceClaudePublisher,
		beforeMemberRun: func(_, _, lane string) error {
			preparedLane = lane
			return nil
		},
	})
	if err != nil {
		t.Fatalf("resolved lane binding action=%+v result=%+v err=%v", result.Action, result, err)
	}
	if result.Lane != inspection.Identity.InitialLane || preparedLane != result.Lane ||
		result.FinalState != DailyActionInputRequired || !result.Blocked || result.SessionLaunches != 0 {
		t.Fatalf("resolved lane was not bound through typed input preparation: prepared=%q result=%+v", preparedLane, result)
	}
}

func TestRunDailyBinaryRERequiresTypedInputBeforeMemberOrReviewerLaunch(t *testing.T) {
	caseRoot := filepath.Join(t.TempDir(), "typed-input-required")
	bootstrap := DailyResult{CaseRoot: caseRoot}
	inspection, err := applyDailyOnboarding(caseRoot, "analyze one specific binary behavior", "daily-test", &bootstrap)
	if err != nil {
		t.Fatal(err)
	}
	memberRunCalled := false
	result, err := RunDaily(context.Background(), DailyOptions{
		Target: caseRoot, Goal: inspection.Identity.Goal,
		Input: DailyInputRequest{Mode: DailyInputArtifactAnalysis},
		beforeMemberRun: func(_, _, _ string) error {
			memberRunCalled = true
			return nil
		},
	})
	if err != nil {
		t.Fatal(err)
	}
	if !memberRunCalled || result.Pack != defaults.DefaultPack || result.Lane != inspection.Identity.InitialLane || result.FinalState != DailyActionInputRequired || !result.Blocked || result.InputReadiness == nil || result.InputReadiness.State != DailyActionInputRequired || result.Action == nil || result.Action.Code != DailyActionInputRequired || result.SessionLaunches != 0 || result.SessionCompletions != 0 || len(result.HostRuns) != 0 {
		t.Fatalf("missing typed input did not stop before launch: memberRunCalled=%t result=%+v", memberRunCalled, result)
	}
	if current, err := memberexecution.CurrentTaskBinding(caseRoot, result.Lane); err != nil || current != nil {
		t.Fatalf("missing typed input published a binding: %+v err=%v", current, err)
	}
}

func TestRunDailyResumeRequiresTypedInputBeforeMemberOrReviewerLaunch(t *testing.T) {
	caseRoot := filepath.Join(t.TempDir(), "typed-input-resume")
	bootstrap := DailyResult{CaseRoot: caseRoot}
	inspection, err := applyDailyOnboarding(caseRoot, "resume one bounded binary task", "daily-test", &bootstrap)
	if err != nil {
		t.Fatal(err)
	}
	bootstrap.Lane = inspection.Identity.InitialLane
	if err := ensureDailyStarted(caseRoot, inspection.Identity.Pack, &bootstrap); err != nil {
		t.Fatal(err)
	}
	memberRunCalled := false
	result, err := RunDaily(context.Background(), DailyOptions{
		Target: caseRoot, SelectedLane: bootstrap.Lane,
		beforeMemberRun: func(_, _, _ string) error {
			memberRunCalled = true
			return nil
		},
	})
	if err != nil {
		t.Fatal(err)
	}
	if !memberRunCalled || result.FinalState != DailyActionInputRequired || !result.Blocked ||
		result.Action == nil || result.Action.Code != DailyActionInputRequired ||
		result.SessionLaunches != 0 || result.SessionCompletions != 0 || len(result.HostRuns) != 0 {
		t.Fatalf("resume bypassed typed input readiness: memberRunCalled=%t result=%+v", memberRunCalled, result)
	}
}

func TestRunDailyArtifactAnalysisBindsExactInputBeforeExecutableResolution(t *testing.T) {
	caseRoot := filepath.Join(t.TempDir(), "typed-artifact-input")
	bootstrap := DailyResult{CaseRoot: caseRoot}
	inspection, err := applyDailyOnboarding(caseRoot, "analyze this exact binary", "daily-test", &bootstrap)
	if err != nil {
		t.Fatal(err)
	}
	artifactRel := "inputs/sample.bin"
	artifactPath := filepath.Join(caseRoot, filepath.FromSlash(artifactRel))
	if err := os.MkdirAll(filepath.Dir(artifactPath), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(artifactPath, []byte("bounded-sample"), 0o600); err != nil {
		t.Fatal(err)
	}
	result, err := RunDaily(context.Background(), DailyOptions{
		Target:                            caseRoot,
		Goal:                              inspection.Identity.Goal,
		Input:                             DailyInputRequest{Mode: DailyInputArtifactAnalysis, ArtifactPath: artifactRel},
		ClaudePath:                        missingClaudePath(t),
		ExpectedClaudeExecutableSHA256:    strings.Repeat("0", 64),
		ExpectedClaudeExecutablePublisher: liveAcceptanceClaudePublisher,
	})
	if err == nil || !strings.Contains(err.Error(), "validate trusted Claude Code executable") {
		t.Fatalf("typed artifact result=%+v err=%v", result, err)
	}
	binding, bindErr := memberexecution.CurrentTaskBinding(caseRoot, inspection.Identity.InitialLane)
	if bindErr != nil || binding == nil || binding.Kind != memberexecution.TaskBindingArtifactAnalysis || binding.Values["artifact-path"] != artifactRel || len(binding.Values["artifact-sha256"]) != 64 || binding.Values["artifact-bytes"] != "14" || result.SessionLaunches != 0 {
		t.Fatalf("typed artifact binding=%+v result=%+v err=%v", binding, result, bindErr)
	}
}

func TestRunDailyWorkspaceInventoryAcceptsEmptyScopeBeforeExecutableResolution(t *testing.T) {
	caseRoot := filepath.Join(t.TempDir(), "typed-workspace-inventory")
	bootstrap := DailyResult{CaseRoot: caseRoot}
	inspection, err := applyDailyOnboarding(caseRoot, "inventory the bounded workspace", "daily-test", &bootstrap)
	if err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(filepath.Join(caseRoot, "inputs"), 0o755); err != nil {
		t.Fatal(err)
	}
	result, err := RunDaily(context.Background(), DailyOptions{
		Target:                            caseRoot,
		Goal:                              inspection.Identity.Goal,
		Input:                             DailyInputRequest{Mode: DailyInputWorkspaceInventory, Scope: "inputs"},
		ClaudePath:                        missingClaudePath(t),
		ExpectedClaudeExecutableSHA256:    strings.Repeat("0", 64),
		ExpectedClaudeExecutablePublisher: liveAcceptanceClaudePublisher,
	})
	if err == nil || !strings.Contains(err.Error(), "validate trusted Claude Code executable") {
		t.Fatalf("workspace inventory result=%+v err=%v", result, err)
	}
	binding, bindErr := memberexecution.CurrentTaskBinding(caseRoot, inspection.Identity.InitialLane)
	if bindErr != nil || binding == nil || binding.Kind != memberexecution.TaskBindingWorkspaceInventory || binding.Values["workspace-scope"] != "inputs" || len(binding.Values) != 1 || result.InputReadiness == nil || result.InputReadiness.State != "ready" || result.SessionLaunches != 0 {
		t.Fatalf("workspace inventory binding=%+v result=%+v err=%v", binding, result, bindErr)
	}
}

func TestRunDailyInvokesMemberPreparationBeforeExecutableResolution(t *testing.T) {
	caseRoot := filepath.Join(t.TempDir(), "member-preparation")
	bootstrap := DailyResult{CaseRoot: caseRoot}
	inspection, err := applyDailyOnboarding(caseRoot, "member preparation goal", "daily-test", &bootstrap)
	if err != nil {
		t.Fatal(err)
	}
	called := false
	result, err := RunDaily(context.Background(), DailyOptions{
		Target:                            caseRoot,
		ClaudePath:                        missingClaudePath(t),
		ExpectedClaudeExecutableSHA256:    strings.Repeat("0", 64),
		ExpectedClaudeExecutablePublisher: liveAcceptanceClaudePublisher,
		Input:                             DailyInputRequest{Mode: DailyInputWorkspaceInventory, Scope: "."},
		beforeMemberRun: func(root, pack, lane string) error {
			called = true
			if root != caseRoot || pack != inspection.Identity.Pack || lane != inspection.Identity.InitialLane {
				t.Fatalf("member preparation binding drifted: root=%s pack=%s lane=%s", root, pack, lane)
			}
			return nil
		},
	})
	if err == nil || !strings.Contains(err.Error(), "validate trusted Claude Code executable") {
		t.Fatalf("RunDaily result=%+v err=%v", result, err)
	}
	if !called || result.SessionLaunches != 0 {
		t.Fatalf("member preparation was not invoked before launch: called=%t result=%+v", called, result)
	}
}

func TestRunDailyRejectsConflictingCommittedGoalWithoutLaunchingClaude(t *testing.T) {
	caseRoot := filepath.Join(t.TempDir(), "goal-conflict")
	bootstrap := DailyResult{CaseRoot: caseRoot}
	if _, err := applyDailyOnboarding(caseRoot, "immutable goal", "daily-test", &bootstrap); err != nil {
		t.Fatal(err)
	}
	result, err := RunDaily(context.Background(), DailyOptions{Target: caseRoot, Goal: "different goal", ClaudePath: missingClaudePath(t), ExpectedClaudeExecutableSHA256: strings.Repeat("0", 64), ExpectedClaudeExecutablePublisher: liveAcceptanceClaudePublisher})
	if err == nil || !strings.Contains(err.Error(), "differs from the immutable committed mission intent") {
		t.Fatalf("conflicting goal result=%+v err=%v", result, err)
	}
	if result.SessionLaunches != 0 || len(result.HostRuns) != 0 {
		t.Fatalf("conflicting goal launched a session: %+v", result)
	}
}

func TestRunDailyAdoptsExistingAttachedCaseBeforeClaudeUnavailable(t *testing.T) {
	repo := sessionhostTestRepoRoot(t)
	unifiedExecutable := buildSessionhostUnifiedRuntimeFixture(t, repo)
	caseRoot := provisionSessionhostAttachedCase(t, repo, "_template")
	result, err := RunDaily(context.Background(), DailyOptions{Target: caseRoot, Goal: "analyze the attached case", InitializationSourceExecutable: unifiedExecutable, ClaudePath: missingClaudePath(t), ExpectedClaudeExecutableSHA256: strings.Repeat("0", 64), ExpectedClaudeExecutablePublisher: liveAcceptanceClaudePublisher})
	if err == nil || !strings.Contains(err.Error(), "validate trusted Claude Code executable") {
		t.Fatalf("RunDaily result=%+v err=%v", result, err)
	}
	if !result.OnboardingApplied || result.Pack != "_template" || result.Lane != "feature-mission" || result.SessionLaunches != 0 {
		t.Fatalf("attached daily result = %+v", result)
	}
	inspection, inspectErr := missionintent.Inspect(caseRoot)
	if inspectErr != nil || !inspection.Committed || inspection.Recovery.Mode != "attached-adoption" {
		t.Fatalf("attached daily inspection=%+v err=%v", inspection, inspectErr)
	}
	content, readErr := os.ReadFile(filepath.Join(caseRoot, "case-local.txt"))
	if readErr != nil || string(content) != "preserve me\n" {
		t.Fatalf("ordinary attached content changed: %q err=%v", content, readErr)
	}
}

func TestRunDailyRecoveryRequiresPendingTransactionWithoutMutation(t *testing.T) {
	caseRoot := t.TempDir()
	before := snapshotDailyCaseFiles(t, caseRoot)
	result, err := RunDailyRecovery(context.Background(), DailyOptions{
		Target:       caseRoot,
		ClaudePath:   missingClaudePath(t),
		Model:        "recovery-test-model",
		Timeout:      time.Second,
		MaxAttempts:  7,
		SelectedLane: "must-not-be-used",
	})
	if err == nil || !strings.Contains(
		err.Error(),
		"requires a pending durable current project update",
	) {
		t.Fatalf("RunDailyRecovery result=%+v err=%v", result, err)
	}
	if result.SessionLaunches != 0 || len(result.HostRuns) != 0 ||
		result.OnboardingApplied {
		t.Fatalf("RunDailyRecovery crossed zero-launch boundary: %+v", result)
	}
	if after := snapshotDailyCaseFiles(t, caseRoot); !maps.Equal(before, after) {
		t.Fatalf("RunDailyRecovery changed target: before=%d after=%d", len(before), len(after))
	}
}

func TestRunDailyStopsForCurrentSyncRecoveryBeforeClaudeOrWrites(t *testing.T) {
	if !rekitfs.HandleBoundExactMutationSupported() {
		t.Skip("current sync Apply requires handle-bound exact filesystem mutation")
	}
	repo := sessionhostTestRepoRoot(t)
	caseRoot := provisionSessionhostAttachedCase(t, repo, "_template")
	sourceRepo := copySessionhostCurrentSyncSourceFixture(t, repo)
	refreshPath := filepath.Join(
		sourceRepo,
		"common",
		"policies",
		"current-sync-daily-recovery-test.md",
	)
	if err := os.WriteFile(
		refreshPath,
		[]byte("current sync daily recovery test\n"),
		0o644,
	); err != nil {
		t.Fatal(err)
	}
	sourceExecutable, err := os.Executable()
	if err != nil {
		t.Fatal(err)
	}
	plan, err := syncreview.CurrentSyncPreview(
		sourceRepo,
		caseRoot,
		"_template",
		syncreview.CurrentSyncOptions{
			Command:          "sync",
			ProjectName:      "attached-demo",
			SourceExecutable: sourceExecutable,
		},
	)
	if err != nil {
		t.Fatal(err)
	}
	if plan.AlreadyCurrent || len(plan.ApplyArgs) == 0 {
		t.Fatalf("current sync daily recovery fixture produced no refresh: %+v", plan)
	}
	restore := syncreview.SetCurrentSyncApplyTransitionHookForTest(
		func(stage string, _ syncreview.CurrentSyncPlan) error {
			if stage == "after-operation-effect:activation-live-to-previous" {
				return errors.New("simulated daily activation interruption")
			}
			return nil
		},
	)
	_, err = syncreview.CurrentSyncApply(
		caseRoot,
		"_template",
		syncreview.CurrentSyncApplyOptions{
			SourceRepoRoot:     sourceRepo,
			ExpectedPlanSHA256: plan.ExpectedPlanSHA256,
			CurrentSyncOptions: syncreview.CurrentSyncOptions{
				Command:          "sync",
				ProjectName:      "attached-demo",
				SourceExecutable: sourceExecutable,
			},
		},
	)
	restore()
	if err == nil || !strings.Contains(err.Error(), "simulated daily activation interruption") {
		t.Fatalf("current sync daily fixture did not interrupt activation: %v", err)
	}
	instancePath := filepath.Join(caseRoot, projectstate.CurrentDir, "instance.yml")
	if _, err := os.Lstat(instancePath); !os.IsNotExist(err) {
		t.Fatalf("current sync daily fixture retained active instance: %v", err)
	}
	before := snapshotDailyCaseFiles(t, caseRoot)
	readyCalls := 0
	memberCalls := 0
	result, err := RunDailyRecovery(context.Background(), DailyOptions{
		Target:      caseRoot,
		Goal:        "must not start while maintenance is pending",
		ClaudePath:  missingClaudePath(t),
		Model:       "recovery-test-model",
		Timeout:     time.Second,
		MaxAttempts: 7,
		onCaseReady: func(string) error {
			readyCalls++
			return nil
		},
		beforeMemberRun: func(string, string, string) error {
			memberCalls++
			return nil
		},
	})
	if err != nil {
		t.Fatal(err)
	}
	if result.FinalState != "maintenance-recovery-required" || !result.Blocked ||
		result.CurrentSyncRecovery == nil ||
		result.CurrentSyncRecovery.State != syncreview.CurrentSyncRecoveryResume ||
		result.Action == nil || result.Action.RequiresInput ||
		result.SessionLaunches != 0 || len(result.HostRuns) != 0 ||
		readyCalls != 0 || memberCalls != 0 {
		t.Fatalf(
			"daily recovery crossed zero-launch boundary: result=%+v ready=%d member=%d",
			result,
			readyCalls,
			memberCalls,
		)
	}
	if after := snapshotDailyCaseFiles(t, caseRoot); !maps.Equal(before, after) {
		t.Fatalf("daily recovery changed project files: before=%d after=%d", len(before), len(after))
	}
}

func TestRunDailyReturnsOrdinaryDirectoryAdoptionWithoutMutation(t *testing.T) {
	caseRoot := t.TempDir()
	path := filepath.Join(caseRoot, "user.txt")
	original := []byte("keep\n")
	if err := os.WriteFile(path, original, 0o600); err != nil {
		t.Fatal(err)
	}
	readyCalls := 0
	result, err := RunDaily(context.Background(), DailyOptions{
		Target: caseRoot,
		Goal:   "goal",
		onCaseReady: func(string) error {
			readyCalls++
			return nil
		},
	})
	if err != nil {
		t.Fatal(err)
	}
	if result.FinalState != DailyActionDirectoryAdoptionRequired || !result.Blocked || result.Action == nil || result.Action.Code != DailyActionDirectoryAdoptionRequired || !result.Action.RequiresInput || len(result.Action.Choices) != 2 || result.Action.Choices[0].ID != "initialize-in-place" || result.Action.Choices[1].ID != "cancel" {
		t.Fatalf("ordinary directory action = %+v", result)
	}
	if readyCalls != 0 || result.SessionLaunches != 0 || len(result.HostRuns) != 0 {
		t.Fatalf("ordinary directory crossed the zero-write admission boundary: %+v calls=%d", result, readyCalls)
	}
	if content, readErr := os.ReadFile(path); readErr != nil || !bytes.Equal(content, original) {
		t.Fatalf("ordinary directory content changed: %q err=%v", content, readErr)
	}
	if _, err := os.Lstat(filepath.Join(caseRoot, filepath.FromSlash(missionintent.IntentRel))); !os.IsNotExist(err) {
		t.Fatalf("ordinary directory adoption preview wrote intent: %v", err)
	}
}

func TestRunDailyRejectsAdoptionSupportFieldsForAttachedTarget(t *testing.T) {
	caseRoot := filepath.Join(t.TempDir(), "attached-adoption-support")
	bootstrap := DailyResult{CaseRoot: caseRoot}
	if _, err := applyDailyOnboarding(caseRoot, "attached mission", "daily-adoption-test", &bootstrap); err != nil {
		t.Fatal(err)
	}
	result, err := RunDaily(context.Background(), DailyOptions{
		Target:                 caseRoot,
		InitializationRepoRoot: sessionhostTestRepoRoot(t),
	})
	if err == nil || !strings.Contains(err.Error(), "adoption controls require an ordinary directory") {
		t.Fatalf("attached adoption support field result=%+v err=%v", result, err)
	}
	if result.Mode != string(DailyOperationAdoption) || result.SessionLaunches != 0 || len(result.HostRuns) != 0 || len(result.DriverSteps) != 0 {
		t.Fatalf("attached adoption support field crossed zero-launch boundary: %+v", result)
	}
}

func TestRunDailyDirectoryAdoptionPreviewApplyAndFreshOnboarding(t *testing.T) {
	repoRoot := sessionhostTestRepoRoot(t)
	unifiedExecutable := buildSessionhostUnifiedRuntimeFixture(t, repoRoot)
	caseRoot := t.TempDir()
	userPath := filepath.Join(caseRoot, "user.txt")
	original := []byte("keep\n")
	if err := os.WriteFile(userPath, original, 0o600); err != nil {
		t.Fatal(err)
	}

	preview, err := RunDaily(context.Background(), DailyOptions{
		Target:                         caseRoot,
		Goal:                           "inspect this project",
		DirectoryAdoptionAction:        dailyAdoptionInitialize,
		InitializationRepoRoot:         repoRoot,
		InitializationSourceExecutable: unifiedExecutable,
	})
	if err != nil {
		t.Fatal(err)
	}
	if preview.FinalState != DailyActionConfirmationRequired || !preview.Blocked || preview.Action == nil || preview.Action.Code != DailyActionConfirmationRequired || preview.DirectoryAdoption == nil || preview.DirectoryAdoption.Plan == nil || !preview.DirectoryAdoption.Plan.AdoptionReady || len(preview.DirectoryAdoption.Plan.ExpectedPlanSHA256) != 64 || preview.SessionLaunches != 0 || len(preview.HostRuns) != 0 {
		t.Fatalf("ordinary directory adoption preview = %+v", preview)
	}
	if content, readErr := os.ReadFile(userPath); readErr != nil || !bytes.Equal(content, original) {
		t.Fatalf("adoption preview changed user file: %q err=%v", content, readErr)
	}
	if _, statErr := os.Lstat(filepath.Join(caseRoot, projectstate.CurrentDir)); !os.IsNotExist(statErr) {
		t.Fatalf("adoption preview wrote project state: %v", statErr)
	}

	stale, staleErr := RunDaily(context.Background(), DailyOptions{
		Target:                         caseRoot,
		Goal:                           "inspect this project",
		DirectoryAdoptionAction:        dailyAdoptionConfirm,
		ExpectedInitPlanSHA256:         strings.Repeat("0", 64),
		InitializationRepoRoot:         repoRoot,
		InitializationSourceExecutable: unifiedExecutable,
	})
	if staleErr == nil || !strings.Contains(staleErr.Error(), "plan changed after preview") || stale.SessionLaunches != 0 || len(stale.HostRuns) != 0 {
		t.Fatalf("stale adoption confirmation = %+v err=%v", stale, staleErr)
	}
	if _, statErr := os.Lstat(filepath.Join(caseRoot, projectstate.CurrentDir)); !os.IsNotExist(statErr) {
		t.Fatalf("stale adoption confirmation wrote project state: %v", statErr)
	}

	applied, err := RunDaily(context.Background(), DailyOptions{
		Target:                         caseRoot,
		Goal:                           "inspect this project",
		DirectoryAdoptionAction:        dailyAdoptionConfirm,
		ExpectedInitPlanSHA256:         preview.DirectoryAdoption.Plan.ExpectedPlanSHA256,
		InitializationRepoRoot:         repoRoot,
		InitializationSourceExecutable: unifiedExecutable,
	})
	if err != nil {
		t.Fatal(err)
	}
	if applied.FinalState != DailyActionReadyToContinue || applied.Blocked || applied.Action == nil || applied.Action.Code != DailyActionReadyToContinue || applied.DirectoryAdoption == nil || applied.DirectoryAdoption.Apply == nil || !applied.DirectoryAdoption.Apply.Applied || applied.SessionLaunches != 0 || len(applied.HostRuns) != 0 || applied.OnboardingApplied {
		t.Fatalf("ordinary directory adoption Apply = %+v", applied)
	}
	if content, readErr := os.ReadFile(userPath); readErr != nil || !bytes.Equal(content, original) {
		t.Fatalf("adoption Apply changed user file: %q err=%v", content, readErr)
	}
	if target, classifyErr := classifyDailyTarget(caseRoot); classifyErr != nil || target.Kind != dailyTargetAttached {
		t.Fatalf("adopted target = %+v err=%v", target, classifyErr)
	}
	if _, statErr := os.Lstat(filepath.Join(caseRoot, projectstate.LegacyDir)); !os.IsNotExist(statErr) {
		t.Fatalf("adoption wrote legacy state: %v", statErr)
	}

	fresh := DailyResult{CaseRoot: caseRoot}
	if _, err := applyDailyOnboarding(caseRoot, "inspect this project", "daily-adoption-test", &fresh); err != nil {
		t.Fatal(err)
	}
	if !fresh.OnboardingApplied {
		t.Fatalf("fresh onboarding did not own the post-adoption mission: %+v", fresh)
	}
}

func TestRunDailyDirectoryAdoptionHonorsMaturePackSelection(t *testing.T) {
	repoRoot := sessionhostTestRepoRoot(t)
	unifiedExecutable := buildSessionhostUnifiedRuntimeFixture(t, repoRoot)

	for _, test := range []struct {
		name    string
		pack    string
		wantErr string
	}{
		{name: "mature web security", pack: "web-security"},
		{name: "skeleton ctf", pack: "ctf", wantErr: "is not mature"},
	} {
		t.Run(test.name, func(t *testing.T) {
			caseRoot := t.TempDir()
			preview, err := RunDaily(context.Background(), DailyOptions{
				Target:                         caseRoot,
				Goal:                           "inspect this project",
				DirectoryAdoptionAction:        dailyAdoptionInitialize,
				DirectoryAdoptionPack:          test.pack,
				InitializationRepoRoot:         repoRoot,
				InitializationSourceExecutable: unifiedExecutable,
			})
			if test.wantErr != "" {
				if err == nil || !strings.Contains(err.Error(), test.wantErr) {
					t.Fatalf("pack %s preview err=%v result=%+v", test.pack, err, preview)
				}
				if _, statErr := os.Lstat(filepath.Join(caseRoot, projectstate.CurrentDir)); !os.IsNotExist(statErr) {
					t.Fatalf("rejected pack selection wrote project state: %v", statErr)
				}
				return
			}
			if err != nil || preview.DirectoryAdoption == nil || preview.DirectoryAdoption.Plan == nil || preview.DirectoryAdoption.Plan.Pack != test.pack || len(preview.DirectoryAdoption.Plan.ExpectedPlanSHA256) != 64 {
				t.Fatalf("mature pack preview err=%v result=%+v", err, preview)
			}
			applied, err := RunDaily(context.Background(), DailyOptions{
				Target:                         caseRoot,
				Goal:                           "inspect this project",
				DirectoryAdoptionAction:        dailyAdoptionConfirm,
				DirectoryAdoptionPack:          test.pack,
				ExpectedInitPlanSHA256:         preview.DirectoryAdoption.Plan.ExpectedPlanSHA256,
				InitializationRepoRoot:         repoRoot,
				InitializationSourceExecutable: unifiedExecutable,
			})
			if err != nil || applied.DirectoryAdoption == nil || applied.DirectoryAdoption.Apply == nil || applied.DirectoryAdoption.Apply.Pack != test.pack {
				t.Fatalf("mature pack Apply err=%v result=%+v", err, applied)
			}
		})
	}
}

func TestRunDailyDirectoryAdoptionCancelAndControlValidationAreZeroWrite(t *testing.T) {
	caseRoot := t.TempDir()
	userPath := filepath.Join(caseRoot, "user.txt")
	original := []byte("keep\n")
	if err := os.WriteFile(userPath, original, 0o600); err != nil {
		t.Fatal(err)
	}
	cancelled, err := RunDaily(context.Background(), DailyOptions{
		Target: caseRoot, Goal: "inspect this project", DirectoryAdoptionAction: dailyAdoptionCancel,
	})
	if err != nil || cancelled.FinalState != "directory-adoption-cancelled" || cancelled.Action == nil || cancelled.Action.Code != DailyActionCompleted || cancelled.SessionLaunches != 0 {
		t.Fatalf("directory adoption cancel = %+v err=%v", cancelled, err)
	}
	if content, readErr := os.ReadFile(userPath); readErr != nil || !bytes.Equal(content, original) {
		t.Fatalf("adoption cancel changed user file: %q err=%v", content, readErr)
	}
	if _, err := RunDaily(context.Background(), DailyOptions{
		Target: caseRoot, Goal: "inspect this project", DirectoryAdoptionAction: "unknown",
	}); err == nil || !strings.Contains(err.Error(), "unknown daily directory adoption action") {
		t.Fatalf("unknown adoption action error = %v", err)
	}
	if _, err := RunDaily(context.Background(), DailyOptions{
		Target: caseRoot, Goal: "inspect this project", ExpectedInitPlanSHA256: strings.Repeat("0", 64),
	}); err == nil || !strings.Contains(err.Error(), "requires an exact adoption action") {
		t.Fatalf("unowned adoption hash error = %v", err)
	}
	if content, readErr := os.ReadFile(userPath); readErr != nil || !bytes.Equal(content, original) {
		t.Fatalf("invalid adoption controls changed user file: %q err=%v", content, readErr)
	}
}

func TestClassifyDailyTargetRejectsPartialCaseState(t *testing.T) {
	for _, rel := range []string{".rekit/state.json", ".claude/skills/rekit/SKILL.md"} {
		t.Run(rel, func(t *testing.T) {
			caseRoot := t.TempDir()
			path := filepath.Join(caseRoot, filepath.FromSlash(rel))
			if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
				t.Fatal(err)
			}
			if err := os.WriteFile(path, []byte("partial\n"), 0o600); err != nil {
				t.Fatal(err)
			}
			classified, err := classifyDailyTarget(caseRoot)
			if err == nil || classified.Kind != dailyTargetInvalid {
				t.Fatalf("partial target classification = %+v err=%v", classified, err)
			}
		})
	}
}

func TestClassifyDailyTargetCoversFiveAdmissionClasses(t *testing.T) {
	missing := filepath.Join(t.TempDir(), "missing")
	ordinary := t.TempDir()
	if target, err := classifyDailyTarget(missing); err != nil || target.Kind != dailyTargetMissing {
		t.Fatalf("missing target = %+v err=%v", target, err)
	}
	if target, err := classifyDailyTarget(ordinary); err != nil || target.Kind != dailyTargetOrdinary {
		t.Fatalf("ordinary target = %+v err=%v", target, err)
	}

	repoRoot, err := currentRepoRoot()
	if err != nil {
		t.Fatal(err)
	}
	attached := filepath.Join(t.TempDir(), "attached")
	preview, err := syncreview.InitPreview(repoRoot, attached, defaults.DefaultPack, syncreview.ApplyOptions{ProjectName: "attached", CreateLocalFiles: true, Command: "init"})
	if err != nil {
		t.Fatal(err)
	}
	if _, err := syncreview.Apply(repoRoot, attached, defaults.DefaultPack, syncreview.ApplyOptions{ProjectName: "attached", CreateLocalFiles: true, Command: "init", ExpectedPlanSHA256: preview.ExpectedPlanSHA256}); err != nil {
		t.Fatal(err)
	}
	if target, err := classifyDailyTarget(attached); err != nil || target.Kind != dailyTargetAttached {
		t.Fatalf("attached target = %+v err=%v", target, err)
	}
	bootstrap := DailyResult{CaseRoot: attached}
	if _, err := applyDailyOnboarding(attached, "classification goal", "daily-test", &bootstrap); err != nil {
		t.Fatal(err)
	}
	if target, err := classifyDailyTarget(attached); err != nil || target.Kind != dailyTargetMission {
		t.Fatalf("mission target = %+v err=%v", target, err)
	}

	writeText := func(path, text string) {
		t.Helper()
		if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(path, []byte(text), 0o600); err != nil {
			t.Fatal(err)
		}
	}
	invalid := filepath.Join(t.TempDir(), "invalid")
	writeText(filepath.Join(invalid, ".rekit", "instance.yml"), "templateRoot: "+repoRoot+"\ntemplatePack: missing-pack\nprojectName: invalid\nprojectRoot: "+invalid+"\n")
	if target, err := classifyDailyTarget(invalid); err == nil || target.Kind != dailyTargetInvalid {
		t.Fatalf("invalid target = %+v err=%v", target, err)
	}
}

func TestRunDailyRejectsInvalidBindingBeforeCaseReadyHook(t *testing.T) {
	repoRoot, err := currentRepoRoot()
	if err != nil {
		t.Fatal(err)
	}
	caseRoot := filepath.Join(t.TempDir(), "wrong-binding")
	if err := os.MkdirAll(filepath.Join(caseRoot, ".rekit"), 0o755); err != nil {
		t.Fatal(err)
	}
	metadata := "templateRoot: " + repoRoot + "\ntemplatePack: missing-pack\nprojectName: invalid\nprojectRoot: " + caseRoot + "\n"
	if err := os.WriteFile(filepath.Join(caseRoot, ".rekit", "instance.yml"), []byte(metadata), 0o600); err != nil {
		t.Fatal(err)
	}
	readyCalls := 0
	result, err := RunDaily(context.Background(), DailyOptions{Target: caseRoot, Goal: "goal", onCaseReady: func(string) error {
		readyCalls++
		return nil
	}})
	if err == nil || readyCalls != 0 || result.SessionLaunches != 0 {
		t.Fatalf("invalid binding crossed admission boundary: result=%+v calls=%d err=%v", result, readyCalls, err)
	}
}

func TestRunDailyPrefersOpenLaneOverCommittedClosedInitialLane(t *testing.T) {
	caseRoot := filepath.Join(t.TempDir(), "committed-terminal")
	bootstrap := DailyResult{CaseRoot: caseRoot}
	inspection, err := applyDailyOnboarding(caseRoot, "committed terminal replay goal", "daily-test", &bootstrap)
	if err != nil {
		t.Fatal(err)
	}
	bootstrap.Lane = inspection.Identity.InitialLane
	if err := ensureDailyStarted(caseRoot, inspection.Identity.Pack, &bootstrap); err != nil {
		t.Fatal(err)
	}
	evidenceRef := sessionhostStateRel(t, caseRoot, "lanes", bootstrap.Lane, "workspace", "completion-evidence.md")
	evidencePath := filepath.Join(caseRoot, filepath.FromSlash(evidenceRef))
	if err := os.MkdirAll(filepath.Dir(evidencePath), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(evidencePath, []byte("reviewed terminal replay evidence\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	completeOpt := workstream.CompleteOptions{
		Selector:     bootstrap.Lane,
		Actor:        "daily-test",
		Reason:       "publish a valid fail-closed completion for terminal replay",
		EvidenceRefs: evidenceRef,
	}
	preview, err := workstream.CompletePreview(sessionhostAttachedRepoRoot(t, caseRoot, inspection.Identity.Pack), caseRoot, inspection.Identity.Pack, completeOpt)
	if err != nil || preview.Blocked || len(preview.CompletionPlanSHA256) != 64 {
		t.Fatalf("completion preview=%+v err=%v", preview, err)
	}
	completeOpt.ExpectedPreviewSHA256 = preview.CompletionPlanSHA256
	completed, err := workstream.CompleteApply(sessionhostAttachedRepoRoot(t, caseRoot, inspection.Identity.Pack), caseRoot, inspection.Identity.Pack, completeOpt)
	if err != nil || !completed.Applied || completed.Lane.Status != "closed" || completed.CompletionReceipt == nil {
		t.Fatalf("completion result=%+v err=%v", completed, err)
	}

	before := snapshotDailyCaseFiles(t, caseRoot)
	for _, replay := range []DailyOptions{
		{Target: caseRoot, Goal: inspection.Identity.Goal},
		{Target: caseRoot},
	} {
		memberRunCalled := false
		replay.beforeMemberRun = func(_ string, _ string, lane string) error {
			memberRunCalled = true
			if lane != "devirt-main" {
				t.Fatalf("daily selected lane = %s, want open authority lane", lane)
			}
			return nil
		}
		result, err := RunDaily(context.Background(), replay)
		if err != nil {
			t.Fatalf("open-lane continuation result=%+v err=%v", result, err)
		}
		if result.Lane != "devirt-main" || result.FinalState == "lane-closed" || result.Action == nil || result.Action.Code == DailyActionCompleted || result.CurrentDriverRequest == nil || result.CurrentDriverRequest.Lane != "devirt-main" || result.SessionLaunches != 0 || result.SessionCompletions != 0 || !memberRunCalled {
			t.Fatalf("closed initial lane displaced open authority lane: called=%t result=%+v", memberRunCalled, result)
		}
		assertDailyCaseFilesEqual(t, before, snapshotDailyCaseFiles(t, caseRoot))
	}
}

func prepareTerminalMissionForSuccessor(t *testing.T, caseRoot, predecessorGoal, evidenceName, evidence string) (missionintent.Inspection, DailyResult) {
	t.Helper()
	bootstrap := DailyResult{CaseRoot: caseRoot}
	inspection, err := applyDailyOnboarding(caseRoot, predecessorGoal, "daily-test", &bootstrap)
	if err != nil {
		t.Fatal(err)
	}
	bootstrap.Lane = inspection.Identity.InitialLane
	if err := ensureDailyStarted(caseRoot, inspection.Identity.Pack, &bootstrap); err != nil {
		t.Fatal(err)
	}
	evidenceRef := sessionhostStateRel(t, caseRoot, "lanes", bootstrap.Lane, "workspace", evidenceName)
	evidencePath := filepath.Join(caseRoot, filepath.FromSlash(evidenceRef))
	if err := os.MkdirAll(filepath.Dir(evidencePath), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(evidencePath, []byte(evidence), 0o600); err != nil {
		t.Fatal(err)
	}
	completeOpt := workstream.CompleteOptions{Selector: bootstrap.Lane, Actor: "daily-test", Reason: "close predecessor before successor", EvidenceRefs: evidenceRef}
	preview, err := workstream.CompletePreview(sessionhostAttachedRepoRoot(t, caseRoot, inspection.Identity.Pack), caseRoot, inspection.Identity.Pack, completeOpt)
	if err != nil || preview.Blocked {
		t.Fatalf("completion preview=%+v err=%v", preview, err)
	}
	completeOpt.ExpectedPreviewSHA256 = preview.CompletionPlanSHA256
	if _, err := workstream.CompleteApply(sessionhostAttachedRepoRoot(t, caseRoot, inspection.Identity.Pack), caseRoot, inspection.Identity.Pack, completeOpt); err != nil {
		t.Fatal(err)
	}
	board, err := mission.ReadBoard(caseRoot)
	if err != nil {
		t.Fatal(err)
	}
	active := make([]mission.BoardLane, 0, 1)
	for _, lane := range board.Lanes {
		if lane.ID == bootstrap.Lane {
			active = append(active, lane)
		}
	}
	board.Lanes = active
	writeDailyTestBoard(t, caseRoot, board)
	if completion, err := workstream.InspectMissionCompletion(caseRoot); err != nil || !completion.Ready {
		t.Fatalf("predecessor mission is not terminal: %+v err=%v", completion, err)
	}
	return inspection, bootstrap
}

func applySuccessorMission(t *testing.T, caseRoot, goal string) DailyResult {
	t.Helper()
	planned, err := RunDaily(context.Background(), DailyOptions{Target: caseRoot, Goal: goal, Actor: "daily-test"})
	if err != nil || planned.SuccessorMission == nil {
		t.Fatalf("successor preview=%+v err=%v", planned, err)
	}
	applied, err := RunDaily(context.Background(), DailyOptions{
		Target: caseRoot, Goal: goal, Actor: "daily-test", SuccessorApply: true,
		SuccessorPublicationStamp:   planned.SuccessorMission.PublicationStamp,
		ExpectedSuccessorPlanSHA256: planned.SuccessorMission.ExpectedPlanSHA256,
	})
	if err != nil {
		t.Fatal(err)
	}
	return applied
}

func TestRunDailyTerminalNewGoalPublishesSuccessorPreviewWithoutWrite(t *testing.T) {
	caseRoot := filepath.Join(t.TempDir(), "successor-preview")
	bootstrap := DailyResult{CaseRoot: caseRoot}
	inspection, err := applyDailyOnboarding(caseRoot, "completed predecessor goal", "daily-test", &bootstrap)
	if err != nil {
		t.Fatal(err)
	}
	bootstrap.Lane = inspection.Identity.InitialLane
	if err := ensureDailyStarted(caseRoot, inspection.Identity.Pack, &bootstrap); err != nil {
		t.Fatal(err)
	}
	evidenceRef := sessionhostStateRel(t, caseRoot, "lanes", bootstrap.Lane, "workspace", "successor-evidence.md")
	evidencePath := filepath.Join(caseRoot, filepath.FromSlash(evidenceRef))
	if err := os.MkdirAll(filepath.Dir(evidencePath), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(evidencePath, []byte("reviewed successor predecessor evidence\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	completeOpt := workstream.CompleteOptions{Selector: bootstrap.Lane, Actor: "daily-test", Reason: "close predecessor before successor", EvidenceRefs: evidenceRef}
	preview, err := workstream.CompletePreview(sessionhostAttachedRepoRoot(t, caseRoot, inspection.Identity.Pack), caseRoot, inspection.Identity.Pack, completeOpt)
	if err != nil || preview.Blocked {
		t.Fatalf("completion preview=%+v err=%v", preview, err)
	}
	completeOpt.ExpectedPreviewSHA256 = preview.CompletionPlanSHA256
	if _, err := workstream.CompleteApply(sessionhostAttachedRepoRoot(t, caseRoot, inspection.Identity.Pack), caseRoot, inspection.Identity.Pack, completeOpt); err != nil {
		t.Fatal(err)
	}
	board, err := mission.ReadBoard(caseRoot)
	if err != nil {
		t.Fatal(err)
	}
	active := make([]mission.BoardLane, 0, 1)
	for _, lane := range board.Lanes {
		if lane.ID == bootstrap.Lane {
			active = append(active, lane)
		}
	}
	board.Lanes = active
	writeDailyTestBoard(t, caseRoot, board)
	if completion, err := workstream.InspectMissionCompletion(caseRoot); err != nil || !completion.Ready {
		t.Fatalf("predecessor mission is not terminal: %+v err=%v", completion, err)
	}
	before := snapshotDailyCaseFiles(t, caseRoot)
	result, err := RunDaily(context.Background(), DailyOptions{Target: caseRoot, Goal: "independent successor goal", Actor: "daily-test"})
	if err != nil {
		t.Fatal(err)
	}
	if result.FinalState != DailyActionConfirmationRequired || !result.Blocked || result.SuccessorMission == nil || result.SuccessorMission.Applied || len(result.SuccessorMission.ExpectedPlanSHA256) != 64 || len(result.SuccessorMission.ApplyArgs) == 0 || result.SessionLaunches != 0 {
		t.Fatalf("successor preview = %+v", result)
	}
	actorBound := false
	for i := 0; i+1 < len(result.SuccessorMission.ApplyArgs); i++ {
		if result.SuccessorMission.ApplyArgs[i] == "-actor" && result.SuccessorMission.ApplyArgs[i+1] == "daily-test" {
			actorBound = true
			break
		}
	}
	if !actorBound {
		t.Fatalf("successor preview Apply args do not bind the reviewed actor: %v", result.SuccessorMission.ApplyArgs)
	}
	seenWrites := make(map[string]bool, len(result.SuccessorMission.Writes))
	for _, write := range result.SuccessorMission.Writes {
		if write.Path == "" || len(write.SHA256) != 64 || write.Size < 1 || write.Phase == "" {
			t.Fatalf("successor preview contains an unbound write: %+v", write)
		}
		if seenWrites[write.Path] {
			t.Fatalf("successor preview contains duplicate write path: %s", write.Path)
		}
		seenWrites[write.Path] = true
	}
	if len(seenWrites) != 16 {
		t.Fatalf("successor preview write count = %d, want 16: %+v", len(seenWrites), result.SuccessorMission.Writes)
	}
	assertDailyCaseFilesEqual(t, before, snapshotDailyCaseFiles(t, caseRoot))
}

func TestRunDailyTerminalSuccessorApplyArgsPreserveReviewedActorThroughHostCLI(t *testing.T) {
	caseRoot := filepath.Join(t.TempDir(), "successor-apply-args-actor")
	prepareTerminalMissionForSuccessor(t, caseRoot, "actor-bound predecessor", "actor-bound-evidence.md", "actor-bound evidence\n")
	actor := "reviewed-successor-actor"
	goal := "actor-bound successor goal"
	planned, err := RunDaily(context.Background(), DailyOptions{Target: caseRoot, Goal: goal, Actor: actor})
	if err != nil || planned.SuccessorMission == nil {
		t.Fatalf("successor preview=%+v err=%v", planned, err)
	}
	executable := buildSessionhostUnifiedRuntimeFixture(t, sessionhostTestRepoRoot(t))
	before := snapshotDailyCaseFiles(t, caseRoot)
	for _, fixture := range []struct {
		name    string
		mutate  func([]string)
		wantErr string
	}{
		{name: "uppercase host", mutate: func(args []string) { args[0] = "HOST" }, wantErr: "host mode token must be exactly"},
		{name: "uppercase sha", mutate: func(args []string) { args[len(args)-1] = strings.ToUpper(args[len(args)-1]) }, wantErr: "exact fresh successorMission.applyArgs argv"},
	} {
		t.Run(fixture.name, func(t *testing.T) {
			args := append([]string(nil), planned.SuccessorMission.ApplyArgs...)
			fixture.mutate(args)
			output, err := exec.Command(executable, args...).CombinedOutput()
			if err == nil || !strings.Contains(string(output), fixture.wantErr) {
				t.Fatalf("mutated successor Apply args were not rejected: err=%v output=%s", err, output)
			}
			assertDailyCaseFilesEqual(t, before, snapshotDailyCaseFiles(t, caseRoot))
		})
	}
	command := exec.Command(executable, planned.SuccessorMission.ApplyArgs...)
	output, err := command.CombinedOutput()
	if err != nil {
		t.Fatalf("execute exact successor Apply args: %v\n%s", err, output)
	}
	var applied DailyResult
	if err := json.Unmarshal(output, &applied); err != nil {
		t.Fatalf("decode exact successor Apply result: %v\n%s", err, output)
	}
	if applied.SuccessorMission == nil || !applied.SuccessorMission.Applied || applied.SuccessorMission.Actor != actor || applied.SessionLaunches != 0 {
		t.Fatalf("exact successor Apply did not preserve reviewed actor: %+v", applied)
	}
}

func TestRunDailyTerminalSuccessorApplyActivatesIsolatedGenerationWithoutClaude(t *testing.T) {
	caseRoot := filepath.Join(t.TempDir(), "successor-apply")
	bootstrap := DailyResult{CaseRoot: caseRoot}
	inspection, err := applyDailyOnboarding(caseRoot, "completed apply predecessor", "daily-test", &bootstrap)
	if err != nil {
		t.Fatal(err)
	}
	bootstrap.Lane = inspection.Identity.InitialLane
	if err := ensureDailyStarted(caseRoot, inspection.Identity.Pack, &bootstrap); err != nil {
		t.Fatal(err)
	}
	evidenceRef := sessionhostStateRel(t, caseRoot, "lanes", bootstrap.Lane, "workspace", "successor-apply-evidence.md")
	evidencePath := filepath.Join(caseRoot, filepath.FromSlash(evidenceRef))
	if err := os.MkdirAll(filepath.Dir(evidencePath), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(evidencePath, []byte("reviewed successor apply evidence\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	completeOpt := workstream.CompleteOptions{Selector: bootstrap.Lane, Actor: "daily-test", Reason: "close predecessor before successor apply", EvidenceRefs: evidenceRef}
	preview, err := workstream.CompletePreview(sessionhostAttachedRepoRoot(t, caseRoot, inspection.Identity.Pack), caseRoot, inspection.Identity.Pack, completeOpt)
	if err != nil || preview.Blocked {
		t.Fatalf("completion preview=%+v err=%v", preview, err)
	}
	completeOpt.ExpectedPreviewSHA256 = preview.CompletionPlanSHA256
	if _, err := workstream.CompleteApply(sessionhostAttachedRepoRoot(t, caseRoot, inspection.Identity.Pack), caseRoot, inspection.Identity.Pack, completeOpt); err != nil {
		t.Fatal(err)
	}
	board, err := mission.ReadBoard(caseRoot)
	if err != nil {
		t.Fatal(err)
	}
	active := make([]mission.BoardLane, 0, 1)
	for _, lane := range board.Lanes {
		if lane.ID == bootstrap.Lane {
			active = append(active, lane)
		}
	}
	board.Lanes = active
	writeDailyTestBoard(t, caseRoot, board)
	goal := "independent applied successor"
	planned, err := RunDaily(context.Background(), DailyOptions{Target: caseRoot, Goal: goal, Actor: "daily-test"})
	if err != nil || planned.SuccessorMission == nil {
		t.Fatalf("successor preview=%+v err=%v", planned, err)
	}
	applied, err := RunDaily(context.Background(), DailyOptions{
		Target: caseRoot, Goal: goal, Actor: "daily-test", SuccessorApply: true,
		SuccessorPublicationStamp:   planned.SuccessorMission.PublicationStamp,
		ExpectedSuccessorPlanSHA256: planned.SuccessorMission.ExpectedPlanSHA256,
	})
	if err != nil {
		t.Fatal(err)
	}
	if applied.SuccessorMission == nil || !applied.SuccessorMission.Applied || applied.SessionLaunches != 0 || applied.FinalState != DailyActionReadyToContinue {
		t.Fatalf("successor Apply = %+v", applied)
	}
	replay, err := RunDaily(context.Background(), DailyOptions{
		Target: caseRoot, Goal: goal, Actor: "daily-test", SuccessorApply: true,
		SuccessorPublicationStamp:   planned.SuccessorMission.PublicationStamp,
		ExpectedSuccessorPlanSHA256: planned.SuccessorMission.ExpectedPlanSHA256,
	})
	if err != nil || replay.SuccessorMission == nil || !replay.SuccessorMission.Replay || replay.SessionLaunches != 0 {
		t.Fatalf("successor replay=%+v err=%v", replay, err)
	}
	view, err := projectstate.ResolveMissionView(caseRoot)
	if err != nil || view.Generation != 2 || view.MissionID != applied.SuccessorMission.MissionID {
		t.Fatalf("active successor view=%+v err=%v", view, err)
	}
	activeIntent, err := missionintent.Inspect(caseRoot)
	if err != nil || activeIntent.Identity.Goal != goal || activeIntent.MissionIntentSHA256 != view.MissionIntentSHA256 {
		t.Fatalf("active successor intent=%+v err=%v", activeIntent, err)
	}
	newBoard, err := mission.ReadBoard(caseRoot)
	if err != nil || len(newBoard.Lanes) != 0 {
		t.Fatalf("fresh successor board=%+v err=%v", newBoard, err)
	}
	newBoard.Lanes = []mission.BoardLane{{
		ID: inspection.Identity.InitialLane, Type: "feature", Title: "Replay mutable board", Status: "ready",
		Workspace: "workspace/features/replay-mutable-board", UpdatedAt: planned.SuccessorMission.PublicationStamp,
	}}
	writeDailyTestBoard(t, caseRoot, newBoard)
	mutableReplay, err := RunDaily(context.Background(), DailyOptions{
		Target: caseRoot, Goal: goal, Actor: "daily-test", SuccessorApply: true,
		SuccessorPublicationStamp:   planned.SuccessorMission.PublicationStamp,
		ExpectedSuccessorPlanSHA256: planned.SuccessorMission.ExpectedPlanSHA256,
	})
	if err != nil || mutableReplay.SuccessorMission == nil || !mutableReplay.SuccessorMission.Replay {
		t.Fatalf("successor replay after mutable board advance=%+v err=%v", mutableReplay, err)
	}
	for _, write := range mutableReplay.SuccessorMission.Writes {
		if write.Kind != "board" {
			continue
		}
		mutableBytes, readErr := os.ReadFile(filepath.Join(caseRoot, filepath.FromSlash(write.Path)))
		if readErr != nil {
			t.Fatal(readErr)
		}
		mutableSHA := sha256.Sum256(mutableBytes)
		if strings.EqualFold(write.SHA256, hex.EncodeToString(mutableSHA[:])) {
			t.Fatalf("committed replay replaced reviewed board digest with mutable bytes: %+v", write)
		}
	}
	newBoard.Lanes = nil
	writeDailyTestBoard(t, caseRoot, newBoard)
	freshStatus, err := runPublicStatus(caseRoot, inspection.Identity.Pack, "")
	if err != nil || freshStatus.MissionControlRunbook == nil || freshStatus.MissionControlRunbook.CurrentDriverRequest == nil || freshStatus.MissionControlRunbook.CurrentDriverRequest.Invocation == nil || freshStatus.MissionControlRunbook.CurrentDriverRequest.Invocation.Command != "start" || freshStatus.MissionControlRunbook.CurrentDriverRequest.Blocked {
		t.Fatalf("fresh successor status omitted its unique start preview: %+v err=%v", freshStatus, err)
	}
	if _, err := os.Stat(filepath.Join(caseRoot, projectstate.CurrentDir, "lanes", bootstrap.Lane, "lane.json")); err != nil {
		t.Fatalf("predecessor audit lane was not retained: %v", err)
	}
}

func TestRunDailyTerminalSecondSuccessorReplacesActivePointer(t *testing.T) {
	caseRoot := filepath.Join(t.TempDir(), "successor-generation-three")
	inspection, _ := prepareTerminalMissionForSuccessor(t, caseRoot, "generation one predecessor", "generation-one-evidence.md", "generation one evidence\n")
	second := applySuccessorMission(t, caseRoot, "generation two mission")
	if second.SuccessorMission == nil || !second.SuccessorMission.Applied {
		t.Fatalf("generation two Apply = %+v", second)
	}
	repoRoot := sessionhostAttachedRepoRoot(t, caseRoot, inspection.Identity.Pack)
	startOpt := workstream.StartOptions{Name: inspection.Identity.InitialLane, Selector: inspection.Identity.InitialLane}
	startPreview, err := workstream.StartPreview(repoRoot, caseRoot, inspection.Identity.Pack, startOpt)
	if err != nil {
		t.Fatal(err)
	}
	startPreviewBytes, err := json.Marshal(startPreview)
	if err != nil {
		t.Fatal(err)
	}
	startPreviewSHA := sha256.Sum256(startPreviewBytes)
	startOpt.ExpectedPreviewSHA256 = hex.EncodeToString(startPreviewSHA[:])
	started, err := workstream.StartApply(repoRoot, caseRoot, inspection.Identity.Pack, startOpt)
	if err != nil {
		t.Fatal(err)
	}
	bootstrap := DailyResult{CaseRoot: caseRoot, Lane: started.Lane.ID}
	evidenceRef := sessionhostStateRel(t, caseRoot, "lanes", bootstrap.Lane, "workspace", "generation-two-evidence.md")
	evidencePath := filepath.Join(caseRoot, filepath.FromSlash(evidenceRef))
	if err := os.MkdirAll(filepath.Dir(evidencePath), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(evidencePath, []byte("generation two evidence\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	completeOpt := workstream.CompleteOptions{Selector: started.Lane.ID, Actor: "daily-test", Reason: "close generation two before successor", EvidenceRefs: evidenceRef}
	preview, err := workstream.CompletePreview(sessionhostAttachedRepoRoot(t, caseRoot, inspection.Identity.Pack), caseRoot, inspection.Identity.Pack, completeOpt)
	if err != nil || preview.Blocked {
		t.Fatalf("generation two completion preview=%+v err=%v", preview, err)
	}
	completeOpt.ExpectedPreviewSHA256 = preview.CompletionPlanSHA256
	if _, err := workstream.CompleteApply(sessionhostAttachedRepoRoot(t, caseRoot, inspection.Identity.Pack), caseRoot, inspection.Identity.Pack, completeOpt); err != nil {
		t.Fatal(err)
	}
	board, err := mission.ReadBoard(caseRoot)
	if err != nil {
		t.Fatal(err)
	}
	active := make([]mission.BoardLane, 0, 1)
	for _, lane := range board.Lanes {
		if lane.ID == bootstrap.Lane {
			active = append(active, lane)
		}
	}
	board.Lanes = active
	writeDailyTestBoard(t, caseRoot, board)
	if completion, err := workstream.InspectMissionCompletion(caseRoot); err != nil || !completion.Ready || !completion.OperationallyComplete {
		t.Fatalf("generation two mission is not terminal: %+v err=%v", completion, err)
	}
	third := applySuccessorMission(t, caseRoot, "generation three mission")
	if third.SuccessorMission == nil || !third.SuccessorMission.Applied || third.SuccessorMission.Generation != 3 {
		t.Fatalf("generation three Apply = %+v", third)
	}
	view, err := projectstate.ResolveMissionView(caseRoot)
	if err != nil || view.Generation != 3 || view.MissionID != third.SuccessorMission.MissionID {
		t.Fatalf("generation three active view=%+v err=%v", view, err)
	}
	if _, err := os.Stat(filepath.Join(caseRoot, projectstate.CurrentDir, projectstate.MissionsDir, "g000002", "mission-intent.json")); err != nil {
		t.Fatalf("generation two audit mission was not retained: %v", err)
	}
}

func TestRunDailyTerminalThirdGenerationRecoversInterruptedActiveReplacement(t *testing.T) {
	caseRoot := filepath.Join(t.TempDir(), "successor-generation-three-recovery")
	inspection, _ := prepareTerminalMissionForSuccessor(t, caseRoot, "generation one recovery predecessor", "g1-recovery.md", "g1 recovery\n")
	applySuccessorMission(t, caseRoot, "generation two recovery mission")
	repoRoot := sessionhostAttachedRepoRoot(t, caseRoot, inspection.Identity.Pack)
	startOpt := workstream.StartOptions{Name: inspection.Identity.InitialLane, Selector: inspection.Identity.InitialLane}
	startPreview, err := workstream.StartPreview(repoRoot, caseRoot, inspection.Identity.Pack, startOpt)
	if err != nil {
		t.Fatal(err)
	}
	startPreviewBytes, _ := json.Marshal(startPreview)
	startPreviewSHA := sha256.Sum256(startPreviewBytes)
	startOpt.ExpectedPreviewSHA256 = hex.EncodeToString(startPreviewSHA[:])
	started, err := workstream.StartApply(repoRoot, caseRoot, inspection.Identity.Pack, startOpt)
	if err != nil {
		t.Fatal(err)
	}
	evidenceRef := sessionhostStateRel(t, caseRoot, "lanes", started.Lane.ID, "workspace", "g2-recovery.md")
	evidencePath := filepath.Join(caseRoot, filepath.FromSlash(evidenceRef))
	if err := os.MkdirAll(filepath.Dir(evidencePath), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(evidencePath, []byte("g2 recovery evidence\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	completeOpt := workstream.CompleteOptions{Selector: started.Lane.ID, Actor: "daily-test", Reason: "close generation two for recovery", EvidenceRefs: evidenceRef}
	completionPreview, err := workstream.CompletePreview(repoRoot, caseRoot, inspection.Identity.Pack, completeOpt)
	if err != nil || completionPreview.Blocked {
		t.Fatalf("completion preview=%+v err=%v", completionPreview, err)
	}
	completeOpt.ExpectedPreviewSHA256 = completionPreview.CompletionPlanSHA256
	if _, err := workstream.CompleteApply(repoRoot, caseRoot, inspection.Identity.Pack, completeOpt); err != nil {
		t.Fatal(err)
	}
	board, err := mission.ReadBoard(caseRoot)
	if err != nil {
		t.Fatal(err)
	}
	activeLanes := make([]mission.BoardLane, 0, 1)
	for _, lane := range board.Lanes {
		if lane.ID == started.Lane.ID {
			activeLanes = append(activeLanes, lane)
		}
	}
	board.Lanes = activeLanes
	writeDailyTestBoard(t, caseRoot, board)
	goal := "generation three recovery mission"
	planned, err := RunDaily(context.Background(), DailyOptions{Target: caseRoot, Goal: goal, Actor: "daily-test"})
	if err != nil || planned.SuccessorMission == nil {
		t.Fatalf("successor preview=%+v err=%v", planned, err)
	}
	stopErr := errors.New("stop after active pointer predecessor rename")
	restore := rekitfs.SetAnchoredRootAfterPredecessorRenameHookForTest(func() error { return stopErr })
	_, err = RunDaily(context.Background(), DailyOptions{
		Target: caseRoot, Goal: goal, Actor: "daily-test", SuccessorApply: true,
		SuccessorPublicationStamp: planned.SuccessorMission.PublicationStamp, ExpectedSuccessorPlanSHA256: planned.SuccessorMission.ExpectedPlanSHA256,
	})
	restore()
	if !errors.Is(err, stopErr) {
		t.Fatalf("successor active replacement cut err=%v", err)
	}
	executable := buildSessionhostUnifiedRuntimeFixture(t, sessionhostTestRepoRoot(t))
	output, err := exec.Command(executable, planned.SuccessorMission.ApplyArgs...).CombinedOutput()
	if err != nil {
		t.Fatalf("recover active replacement through exact CLI argv: %v\n%s", err, output)
	}
	var recovered DailyResult
	if err := json.Unmarshal(output, &recovered); err != nil {
		t.Fatalf("decode recovered successor result: %v\n%s", err, output)
	}
	if recovered.SuccessorMission == nil || !recovered.SuccessorMission.Applied || recovered.SuccessorMission.Generation != 3 {
		t.Fatalf("recover active replacement=%+v", recovered)
	}
}

func TestRunDailyImplicitSuccessorRejectsTypedInput(t *testing.T) {
	caseRoot := filepath.Join(t.TempDir(), "successor-typed-input")
	prepareTerminalMissionForSuccessor(t, caseRoot, "completed predecessor", "successor-evidence.md", "reviewed evidence\n")
	result, err := RunDaily(context.Background(), DailyOptions{
		Target: caseRoot,
		Goal:   "new successor goal",
		Input:  DailyInputRequest{Mode: DailyInputWorkspaceInventory, Scope: "."},
		Actor:  "daily-test",
	})
	if err == nil || !strings.Contains(err.Error(), "implicit successor mission") || result.SuccessorMission != nil || result.SessionLaunches != 0 {
		t.Fatalf("implicit successor accepted typed input: result=%+v err=%v", result, err)
	}
}

func TestRunDailyTerminalSuccessorRejectsLegacyStateWithoutWrite(t *testing.T) {
	caseRoot := filepath.Join(t.TempDir(), "successor-legacy")
	prepareTerminalMissionForSuccessor(t, caseRoot, "legacy predecessor", "legacy-evidence.md", "legacy evidence\n")
	currentRoot := filepath.Join(caseRoot, projectstate.CurrentDir)
	legacyRoot := filepath.Join(caseRoot, projectstate.LegacyDir)
	if err := os.Rename(currentRoot, legacyRoot); err != nil {
		t.Fatal(err)
	}
	before := snapshotDailyCaseFiles(t, caseRoot)
	if _, err := missionsuccessor.Preview(caseRoot, missionsuccessor.Options{Goal: "legacy successor", Actor: "daily-test"}); err == nil || !strings.Contains(err.Error(), "existing current .steamai project") {
		t.Fatalf("legacy successor preview was not rejected: %v", err)
	}
	assertDailyCaseFilesEqual(t, before, snapshotDailyCaseFiles(t, caseRoot))
}

func TestRunDailyTerminalSuccessorRejectsDualStateRootsWithoutWrite(t *testing.T) {
	caseRoot := filepath.Join(t.TempDir(), "successor-dual-root")
	prepareTerminalMissionForSuccessor(t, caseRoot, "dual predecessor", "dual-evidence.md", "dual evidence\n")
	if err := os.Mkdir(filepath.Join(caseRoot, projectstate.LegacyDir), 0o700); err != nil {
		t.Fatal(err)
	}
	before := snapshotDailyCaseFiles(t, caseRoot)
	if _, err := missionsuccessor.Preview(caseRoot, missionsuccessor.Options{Goal: "dual successor", Actor: "daily-test"}); err == nil || !strings.Contains(err.Error(), "must not coexist") {
		t.Fatalf("dual-root successor preview was not rejected: %v", err)
	}
	assertDailyCaseFilesEqual(t, before, snapshotDailyCaseFiles(t, caseRoot))
}

func TestRunDailyTerminalSuccessorRecoversExactPartialPublication(t *testing.T) {
	for _, cut := range []int{1, 2, 8, 15} {
		t.Run(fmt.Sprintf("after-%02d-publications", cut), func(t *testing.T) {
			caseRoot := filepath.Join(t.TempDir(), "successor-partial-recovery")
			prepareTerminalMissionForSuccessor(t, caseRoot, "partial predecessor", "partial-evidence.md", "partial evidence\n")
			goal := fmt.Sprintf("recover exact partial successor %d", cut)
			planned, err := RunDaily(context.Background(), DailyOptions{Target: caseRoot, Goal: goal, Actor: "daily-test"})
			if err != nil || planned.SuccessorMission == nil {
				t.Fatalf("successor preview=%+v err=%v", planned, err)
			}
			stopErr := fmt.Errorf("stop after %d durable successor publications", cut)
			calls := 0
			restore := missionsuccessor.SetApplyAfterPublicationHookForTest(func(write missionsuccessor.Write) error {
				calls++
				if calls == cut {
					return stopErr
				}
				return nil
			})
			_, err = RunDaily(context.Background(), DailyOptions{
				Target: caseRoot, Goal: goal, Actor: "daily-test", SuccessorApply: true,
				SuccessorPublicationStamp:   planned.SuccessorMission.PublicationStamp,
				ExpectedSuccessorPlanSHA256: planned.SuccessorMission.ExpectedPlanSHA256,
			})
			restore()
			if !errors.Is(err, stopErr) || calls != cut {
				t.Fatalf("partial successor Apply calls=%d err=%v", calls, err)
			}
			view, viewErr := projectstate.ResolveMissionView(caseRoot)
			if viewErr != nil || view.Generation != 1 {
				t.Fatalf("partial successor activated a generation: view=%+v err=%v", view, viewErr)
			}
			recovered, err := RunDaily(context.Background(), DailyOptions{
				Target: caseRoot, Goal: goal, Actor: "daily-test", SuccessorApply: true,
				SuccessorPublicationStamp:   planned.SuccessorMission.PublicationStamp,
				ExpectedSuccessorPlanSHA256: planned.SuccessorMission.ExpectedPlanSHA256,
			})
			if err != nil || recovered.SuccessorMission == nil || !recovered.SuccessorMission.Applied || recovered.SuccessorMission.Generation != 2 {
				t.Fatalf("recover partial successor=%+v err=%v", recovered, err)
			}
		})
	}
}

func TestRunDailyTerminalSuccessorRejectsDifferentPlanWhilePartialIntentIsPending(t *testing.T) {
	caseRoot := filepath.Join(t.TempDir(), "successor-stale-partial-plan")
	prepareTerminalMissionForSuccessor(t, caseRoot, "pending predecessor", "pending-evidence.md", "pending evidence\n")
	first, err := missionsuccessor.Preview(caseRoot, missionsuccessor.Options{
		Goal: "first pending successor", Actor: "daily-test",
	})
	if err != nil {
		t.Fatal(err)
	}
	stopErr := errors.New("stop after pending successor intent")
	restore := missionsuccessor.SetApplyAfterPublicationHookForTest(func(write missionsuccessor.Write) error { return stopErr })
	_, err = missionsuccessor.Apply(caseRoot, missionsuccessor.Options{
		Goal: first.Goal, Actor: "daily-test", PublicationStamp: first.PublicationStamp,
		ExpectedPlanSHA256: first.ExpectedPlanSHA256,
	})
	restore()
	if !errors.Is(err, stopErr) {
		t.Fatalf("partial successor Apply err=%v", err)
	}
	second, err := missionsuccessor.Preview(caseRoot, missionsuccessor.Options{
		Goal: "different successor must not bypass pending intent", Actor: "daily-test",
	})
	if err != nil {
		t.Fatal(err)
	}
	if _, err := missionsuccessor.Apply(caseRoot, missionsuccessor.Options{
		Goal: second.Goal, Actor: "daily-test", PublicationStamp: second.PublicationStamp,
		ExpectedPlanSHA256: second.ExpectedPlanSHA256,
	}); err == nil || !strings.Contains(err.Error(), "different successor transition is pending") {
		t.Fatalf("different successor bypassed pending exact recovery: %v", err)
	}
	view, viewErr := projectstate.ResolveMissionView(caseRoot)
	if viewErr != nil || view.Generation != 1 {
		t.Fatalf("different successor activated a generation: view=%+v err=%v", view, viewErr)
	}
}

func TestRunDailyTerminalSuccessorRejectsCorruptPartialPublication(t *testing.T) {
	caseRoot := filepath.Join(t.TempDir(), "successor-partial-corrupt")
	prepareTerminalMissionForSuccessor(t, caseRoot, "corrupt predecessor", "corrupt-evidence.md", "corrupt evidence\n")
	goal := "reject corrupt partial successor"
	planned, err := RunDaily(context.Background(), DailyOptions{Target: caseRoot, Goal: goal, Actor: "daily-test"})
	if err != nil || planned.SuccessorMission == nil {
		t.Fatalf("successor preview=%+v err=%v", planned, err)
	}
	stopErr := errors.New("stop after first durable successor publication")
	restore := missionsuccessor.SetApplyAfterPublicationHookForTest(func(write missionsuccessor.Write) error { return stopErr })
	_, err = RunDaily(context.Background(), DailyOptions{
		Target: caseRoot, Goal: goal, Actor: "daily-test", SuccessorApply: true,
		SuccessorPublicationStamp:   planned.SuccessorMission.PublicationStamp,
		ExpectedSuccessorPlanSHA256: planned.SuccessorMission.ExpectedPlanSHA256,
	})
	restore()
	if !errors.Is(err, stopErr) {
		t.Fatalf("partial successor Apply err=%v", err)
	}
	firstPath := filepath.Join(caseRoot, filepath.FromSlash(planned.SuccessorMission.Writes[0].Path))
	if err := os.WriteFile(firstPath, []byte("{}\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	if _, err := RunDaily(context.Background(), DailyOptions{
		Target: caseRoot, Goal: goal, Actor: "daily-test", SuccessorApply: true,
		SuccessorPublicationStamp:   planned.SuccessorMission.PublicationStamp,
		ExpectedSuccessorPlanSHA256: planned.SuccessorMission.ExpectedPlanSHA256,
	}); err == nil || (!strings.Contains(err.Error(), "existing file differs from exact complete bytes") && !strings.Contains(err.Error(), "existing file is incomplete or not regular") && !strings.Contains(err.Error(), "successor artifact is not canonical")) {
		t.Fatalf("corrupt partial successor was not rejected: %v", err)
	}
	view, viewErr := projectstate.ResolveMissionView(caseRoot)
	if viewErr != nil || view.Generation != 1 {
		t.Fatalf("corrupt partial successor activated a generation: view=%+v err=%v", view, viewErr)
	}
}

func TestRunDailyTerminalSuccessorRejectsShortPublicationStampWithoutPanic(t *testing.T) {
	caseRoot := filepath.Join(t.TempDir(), "successor-short-stamp")
	prepareTerminalMissionForSuccessor(t, caseRoot, "short stamp predecessor", "short-stamp-evidence.md", "short stamp evidence\n")
	if _, err := missionsuccessor.Apply(caseRoot, missionsuccessor.Options{
		Goal: "short stamp successor", Actor: "daily-test", PublicationStamp: "x", ExpectedPlanSHA256: strings.Repeat("a", 64),
	}); err == nil || !strings.Contains(err.Error(), "invalid successor mission publication stamp") {
		t.Fatalf("short publication stamp error = %v", err)
	}
}

func TestRunDailyTerminalSuccessorExactReplaySurvivesPackManifestDrift(t *testing.T) {
	caseRoot := filepath.Join(t.TempDir(), "successor-replay-pack-drift")
	inspection, _ := prepareTerminalMissionForSuccessor(t, caseRoot, "replay pack predecessor", "replay-pack-evidence.md", "replay pack evidence\n")
	goal := "replay pack bound successor"
	planned, err := RunDaily(context.Background(), DailyOptions{Target: caseRoot, Goal: goal, Actor: "daily-test"})
	if err != nil || planned.SuccessorMission == nil {
		t.Fatalf("successor preview=%+v err=%v", planned, err)
	}
	applied, err := RunDaily(context.Background(), DailyOptions{
		Target: caseRoot, Goal: goal, Actor: "daily-test", SuccessorApply: true,
		SuccessorPublicationStamp: planned.SuccessorMission.PublicationStamp, ExpectedSuccessorPlanSHA256: planned.SuccessorMission.ExpectedPlanSHA256,
	})
	if err != nil || applied.SuccessorMission == nil || !applied.SuccessorMission.Applied {
		t.Fatalf("successor Apply=%+v err=%v", applied, err)
	}
	manifestPath := filepath.Join(sessionhostAttachedRepoRoot(t, caseRoot, inspection.Identity.Pack), "packs", inspection.Identity.Pack, "manifest.yml")
	manifestBytes, err := os.ReadFile(manifestPath)
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(manifestPath, append(manifestBytes, []byte("\n# drift after committed successor\n")...), 0o600); err != nil {
		t.Fatal(err)
	}
	replayed, err := missionsuccessor.Apply(caseRoot, missionsuccessor.Options{
		Goal: goal, Actor: "daily-test", PublicationStamp: planned.SuccessorMission.PublicationStamp,
		ExpectedPlanSHA256: planned.SuccessorMission.ExpectedPlanSHA256,
	})
	if err != nil || !replayed.Applied || !replayed.Replay || replayed.PackManifestSHA256 != planned.SuccessorMission.PackManifestSHA256 || replayed.AuthorityLane != planned.SuccessorMission.AuthorityLane {
		t.Fatalf("committed replay after pack drift=%+v err=%v", replayed, err)
	}
}

func TestRunDailyTerminalSuccessorRejectsProjectLocalPackManifestDrift(t *testing.T) {
	caseRoot := filepath.Join(t.TempDir(), "successor-pack-manifest-drift")
	inspection, _ := prepareTerminalMissionForSuccessor(t, caseRoot, "pack manifest predecessor", "pack-manifest-evidence.md", "pack manifest evidence\n")
	goal := "pack manifest bound successor"
	planned, err := RunDaily(context.Background(), DailyOptions{Target: caseRoot, Goal: goal, Actor: "daily-test"})
	if err != nil || planned.SuccessorMission == nil || len(planned.SuccessorMission.PackManifestSHA256) != 64 || planned.SuccessorMission.AuthorityLane == "" {
		t.Fatalf("successor preview=%+v err=%v", planned, err)
	}
	manifestPath := filepath.Join(sessionhostAttachedRepoRoot(t, caseRoot, inspection.Identity.Pack), "packs", inspection.Identity.Pack, "manifest.yml")
	manifestBytes, err := os.ReadFile(manifestPath)
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(manifestPath, append(manifestBytes, []byte("\n# drift after successor review\n")...), 0o600); err != nil {
		t.Fatal(err)
	}
	if _, err := missionsuccessor.Apply(caseRoot, missionsuccessor.Options{
		Goal: goal, Actor: "daily-test", PublicationStamp: planned.SuccessorMission.PublicationStamp,
		ExpectedPlanSHA256: planned.SuccessorMission.ExpectedPlanSHA256,
	}); err == nil || !strings.Contains(err.Error(), "plan changed after review") {
		t.Fatalf("pack manifest drift was not rejected: %v", err)
	}
}

func TestRunDailyTerminalSuccessorActiveViewRequiresGenerationCommitAndProjectIdentity(t *testing.T) {
	t.Run("generation-commit", func(t *testing.T) {
		caseRoot := filepath.Join(t.TempDir(), "successor-generation-commit")
		prepareTerminalMissionForSuccessor(t, caseRoot, "generation commit predecessor", "generation-commit-evidence.md", "generation commit evidence\n")
		applied := applySuccessorMission(t, caseRoot, "generation commit successor")
		commitPath := filepath.Join(caseRoot, filepath.FromSlash(generationPathForTest(applied.SuccessorMission.Generation, "commit.json")))
		if err := os.Remove(commitPath); err != nil {
			t.Fatal(err)
		}
		if _, err := projectstate.ResolveMissionView(caseRoot); err == nil || !strings.Contains(err.Error(), "generation commit") {
			t.Fatalf("active view accepted missing generation commit: %v", err)
		}
	})
	t.Run("project-id", func(t *testing.T) {
		caseRoot := filepath.Join(t.TempDir(), "successor-project-id")
		prepareTerminalMissionForSuccessor(t, caseRoot, "project identity predecessor", "project-identity-evidence.md", "project identity evidence\n")
		applySuccessorMission(t, caseRoot, "project identity successor")
		activePath, err := projectstate.MissionActivePath(caseRoot)
		if err != nil {
			t.Fatal(err)
		}
		activeBytes, err := os.ReadFile(activePath)
		if err != nil {
			t.Fatal(err)
		}
		var active projectstate.MissionActive
		if err := json.Unmarshal(activeBytes, &active); err != nil {
			t.Fatal(err)
		}
		active.ProjectID = "0123456789abcdef"
		activeBytes, err = json.MarshalIndent(active, "", "  ")
		if err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(activePath, append(activeBytes, '\n'), 0o600); err != nil {
			t.Fatal(err)
		}
		if _, err := projectstate.ResolveMissionView(caseRoot); err == nil || !strings.Contains(err.Error(), "manifest does not bind") {
			t.Fatalf("active view accepted mismatched project identity: %v", err)
		}
	})
}

func generationPathForTest(generation int, rel string) string {
	return filepath.ToSlash(filepath.Join(projectstate.CurrentDir, projectstate.MissionsDir, fmt.Sprintf("g%06d", generation), rel))
}

func TestRunDailyTerminalSuccessorRejectsStaleClosureWithoutWrite(t *testing.T) {
	caseRoot := filepath.Join(t.TempDir(), "successor-stale")
	bootstrap := DailyResult{CaseRoot: caseRoot}
	inspection, err := applyDailyOnboarding(caseRoot, "completed stale predecessor", "daily-test", &bootstrap)
	if err != nil {
		t.Fatal(err)
	}
	bootstrap.Lane = inspection.Identity.InitialLane
	if err := ensureDailyStarted(caseRoot, inspection.Identity.Pack, &bootstrap); err != nil {
		t.Fatal(err)
	}
	evidenceRef := sessionhostStateRel(t, caseRoot, "lanes", bootstrap.Lane, "workspace", "successor-stale-evidence.md")
	evidencePath := filepath.Join(caseRoot, filepath.FromSlash(evidenceRef))
	if err := os.MkdirAll(filepath.Dir(evidencePath), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(evidencePath, []byte("reviewed successor stale evidence\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	completeOpt := workstream.CompleteOptions{Selector: bootstrap.Lane, Actor: "daily-test", Reason: "close predecessor before stale successor", EvidenceRefs: evidenceRef}
	preview, err := workstream.CompletePreview(sessionhostAttachedRepoRoot(t, caseRoot, inspection.Identity.Pack), caseRoot, inspection.Identity.Pack, completeOpt)
	if err != nil || preview.Blocked {
		t.Fatalf("completion preview=%+v err=%v", preview, err)
	}
	completeOpt.ExpectedPreviewSHA256 = preview.CompletionPlanSHA256
	if _, err := workstream.CompleteApply(sessionhostAttachedRepoRoot(t, caseRoot, inspection.Identity.Pack), caseRoot, inspection.Identity.Pack, completeOpt); err != nil {
		t.Fatal(err)
	}
	board, err := mission.ReadBoard(caseRoot)
	if err != nil {
		t.Fatal(err)
	}
	active := make([]mission.BoardLane, 0, 1)
	for _, lane := range board.Lanes {
		if lane.ID == bootstrap.Lane {
			active = append(active, lane)
		}
	}
	board.Lanes = active
	writeDailyTestBoard(t, caseRoot, board)
	goal := "stale successor goal"
	planned, err := RunDaily(context.Background(), DailyOptions{Target: caseRoot, Goal: goal, Actor: "daily-test"})
	if err != nil || planned.SuccessorMission == nil {
		t.Fatalf("successor preview=%+v err=%v", planned, err)
	}
	if err := os.WriteFile(filepath.Join(caseRoot, projectstate.CurrentDir, "facts", "observations.jsonl"), []byte("{\"eventId\":\"late\"}\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	before := snapshotDailyCaseFiles(t, caseRoot)
	applied, err := RunDaily(context.Background(), DailyOptions{
		Target: caseRoot, Goal: goal, Actor: "daily-test", SuccessorApply: true,
		SuccessorPublicationStamp:   planned.SuccessorMission.PublicationStamp,
		ExpectedSuccessorPlanSHA256: planned.SuccessorMission.ExpectedPlanSHA256,
	})
	if err == nil || !strings.Contains(err.Error(), "plan changed after review") || applied.SessionLaunches != 0 {
		t.Fatalf("stale successor Apply=%+v err=%v", applied, err)
	}
	assertDailyCaseFilesEqual(t, before, snapshotDailyCaseFiles(t, caseRoot))
}

func TestRunDailyCorrectionReopensCommittedClosedLaneBeforeNewMember(t *testing.T) {
	caseRoot := filepath.Join(t.TempDir(), "terminal-correction")
	actor := "daily-test"
	correction := "post-completion review found one more bounded verification step"
	bootstrap := DailyResult{CaseRoot: caseRoot}
	inspection, err := applyDailyOnboarding(caseRoot, "terminal correction goal", actor, &bootstrap)
	if err != nil {
		t.Fatal(err)
	}
	bootstrap.Lane = inspection.Identity.InitialLane
	if err := ensureDailyStarted(caseRoot, inspection.Identity.Pack, &bootstrap); err != nil {
		t.Fatal(err)
	}
	evidenceRef := sessionhostStateRel(t, caseRoot, "lanes", bootstrap.Lane, "workspace", "completion-evidence.md")
	evidencePath := filepath.Join(caseRoot, filepath.FromSlash(evidenceRef))
	if err := os.MkdirAll(filepath.Dir(evidencePath), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(evidencePath, []byte("reviewed completion evidence before terminal correction\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	completeOpt := workstream.CompleteOptions{Selector: bootstrap.Lane, Actor: actor, Reason: "publish terminal correction fixture", EvidenceRefs: evidenceRef}
	preview, err := workstream.CompletePreview(sessionhostAttachedRepoRoot(t, caseRoot, inspection.Identity.Pack), caseRoot, inspection.Identity.Pack, completeOpt)
	if err != nil || preview.Blocked || len(preview.CompletionPlanSHA256) != 64 {
		t.Fatalf("completion preview=%+v err=%v", preview, err)
	}
	completeOpt.ExpectedPreviewSHA256 = preview.CompletionPlanSHA256
	completed, err := workstream.CompleteApply(sessionhostAttachedRepoRoot(t, caseRoot, inspection.Identity.Pack), caseRoot, inspection.Identity.Pack, completeOpt)
	if err != nil || !completed.Applied || completed.Lane.Status != "closed" || completed.CompletionReceipt == nil {
		t.Fatalf("completion result=%+v err=%v", completed, err)
	}

	memberRunCalled := false
	result, err := RunDaily(context.Background(), DailyOptions{
		Target: caseRoot, Correction: correction, Actor: actor,
		beforeMemberRun: func(_, _, _ string) error { memberRunCalled = true; return nil },
	})
	if err != nil {
		t.Fatalf("terminal correction result=%+v err=%v", result, err)
	}
	if memberRunCalled || result.Lane != bootstrap.Lane || result.FinalState != "terminal-correction-reopened" || result.Action == nil || result.Action.Code != DailyActionReadyToContinue || result.ReopenOperationID == "" || result.ReopenOperationCommit == nil || result.ReopenOperationCommit.State != "committed" || result.ReopenOperationCommit.Reason != correction || result.SessionLaunches != 0 || len(result.HostRuns) != 0 || !containsDailyStep(result.DriverSteps, "reopen") {
		t.Fatalf("terminal correction did not stop at committed review-first reopen: memberRunCalled=%t result=%+v", memberRunCalled, result)
	}
	if result.CurrentDriverRequest == nil || result.CurrentDriverRequestSHA256 == "" {
		t.Fatalf("terminal correction omitted the fresh typed continuation request: %+v", result)
	}
	if identity, err := mission.MissionCommanderDriverRequestSHA256(*result.CurrentDriverRequest); err != nil || identity != result.CurrentDriverRequestSHA256 {
		t.Fatalf("terminal correction current driver request identity=%s err=%v result=%+v", identity, err, result)
	}
	if !result.ReopenOperationCommit.NoAuthority || !result.ReopenOperationCommit.NoConfirmed || !result.ReopenOperationCommit.NoHeavyTool || !result.ReopenOperationCommit.NoAutoResume {
		t.Fatalf("terminal correction crossed reopen boundaries: %+v", result.ReopenOperationCommit)
	}
	lifecycle, err := lanecompletion.Inspect(caseRoot, bootstrap.Lane)
	if err != nil || lifecycle.State != lanecompletion.StateReopened || lifecycle.CurrentReopen == nil || lifecycle.CurrentReopen.OperationID != result.ReopenOperationID {
		t.Fatalf("terminal correction lifecycle=%+v err=%v", lifecycle, err)
	}
	board, err := mission.ReadBoard(caseRoot)
	if err != nil {
		t.Fatal(err)
	}
	lane, ok := mission.LookupBoardLane(board.Lanes, bootstrap.Lane, false)
	if !ok || lane.Status != "open" || lane.ExecutorGeneration != completed.Lane.ExecutorGeneration+1 || lane.CurrentExecutor != "" {
		t.Fatalf("terminal correction did not fence the completed owner before fresh execution: %+v", lane)
	}
}

func TestRunDailyCommittedTerminalCorrectionDoesNotClaimDifferentExplicitLane(t *testing.T) {
	caseRoot := filepath.Join(t.TempDir(), "terminal-correction-explicit-lane")
	actor := "daily-test"
	correction := "reuse this bounded correction on the selected lane"
	bootstrap := DailyResult{CaseRoot: caseRoot}
	inspection, err := applyDailyOnboarding(caseRoot, "explicit terminal correction goal", actor, &bootstrap)
	if err != nil {
		t.Fatal(err)
	}
	bootstrap.Lane = inspection.Identity.InitialLane
	if err := ensureDailyStarted(caseRoot, inspection.Identity.Pack, &bootstrap); err != nil {
		t.Fatal(err)
	}
	if _, err := workstream.StartApply(sessionhostAttachedRepoRoot(t, caseRoot, inspection.Identity.Pack), caseRoot, inspection.Identity.Pack, workstream.StartOptions{
		Name: "login", Actor: actor, Executor: "session-login", TakeoverReason: "explicit terminal correction regression",
	}); err != nil {
		t.Fatal(err)
	}
	closed := []string{bootstrap.Lane, "binary-analysis-login"}
	for _, lane := range closed {
		evidenceRef := sessionhostStateRel(t, caseRoot, "lanes", lane, "workspace", "completion-evidence.md")
		evidencePath := filepath.Join(caseRoot, filepath.FromSlash(evidenceRef))
		if err := os.MkdirAll(filepath.Dir(evidencePath), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(evidencePath, []byte("reviewed completion evidence for "+lane+"\n"), 0o600); err != nil {
			t.Fatal(err)
		}
		completeOpt := workstream.CompleteOptions{Selector: lane, Actor: actor, Reason: "close lane for explicit terminal correction", EvidenceRefs: evidenceRef}
		preview, err := workstream.CompletePreview(sessionhostAttachedRepoRoot(t, caseRoot, inspection.Identity.Pack), caseRoot, inspection.Identity.Pack, completeOpt)
		if err != nil {
			t.Fatal(err)
		}
		completeOpt.ExpectedPreviewSHA256 = preview.CompletionPlanSHA256
		if completed, err := workstream.CompleteApply(sessionhostAttachedRepoRoot(t, caseRoot, inspection.Identity.Pack), caseRoot, inspection.Identity.Pack, completeOpt); err != nil || !completed.Applied {
			t.Fatalf("complete %s: result=%+v err=%v", lane, completed, err)
		}
	}

	first, err := RunDaily(context.Background(), DailyOptions{Target: caseRoot, Correction: correction, SelectedLane: bootstrap.Lane, Actor: actor})
	if err != nil {
		t.Fatalf("first terminal correction: result=%+v err=%v", first, err)
	}
	if first.Lane != bootstrap.Lane || first.FinalState != "terminal-correction-reopened" || first.ReopenOperationID == "" || first.SessionLaunches != 0 || len(first.HostRuns) != 0 {
		t.Fatalf("first explicit terminal correction crossed its boundary: %+v", first)
	}

	second, err := RunDaily(context.Background(), DailyOptions{Target: caseRoot, Correction: correction, SelectedLane: "binary-analysis-login", Actor: actor})
	if err != nil {
		t.Fatalf("second terminal correction: result=%+v err=%v", second, err)
	}
	if second.Lane != "binary-analysis-login" || second.FinalState != "terminal-correction-reopened" || second.ReopenOperationID == "" || second.ReopenOperationID == first.ReopenOperationID || second.Replay || second.SessionLaunches != 0 || len(second.HostRuns) != 0 {
		t.Fatalf("different explicit lane did not create its own terminal correction: first=%+v second=%+v", first, second)
	}
	operations, err := lanecompletion.InspectOperations(caseRoot)
	if err != nil || operations.LatestSequence != 2 || len(operations.Commits) != 2 {
		t.Fatalf("different explicit lane operation history=%+v err=%v", operations, err)
	}

	beforeReplay := snapshotDailyCaseFiles(t, caseRoot)
	replayed, err := RunDaily(context.Background(), DailyOptions{Target: caseRoot, Correction: correction, SelectedLane: "binary-analysis-login", Actor: actor})
	if err != nil {
		t.Fatalf("same explicit lane replay: result=%+v err=%v", replayed, err)
	}
	if !replayed.Replay || replayed.Lane != "binary-analysis-login" || replayed.ReopenOperationID != second.ReopenOperationID || replayed.SessionLaunches != 0 || len(replayed.HostRuns) != 0 {
		t.Fatalf("same explicit lane did not replay its exact committed operation: %+v", replayed)
	}
	assertDailyCaseFilesEqual(t, beforeReplay, snapshotDailyCaseFiles(t, caseRoot))

	beforeHistoricalReplay := snapshotDailyCaseFiles(t, caseRoot)
	historical, err := RunDaily(context.Background(), DailyOptions{Target: caseRoot, Correction: correction, SelectedLane: bootstrap.Lane, Actor: actor})
	if err != nil {
		t.Fatalf("historical explicit lane replay: result=%+v err=%v", historical, err)
	}
	if !historical.Replay || historical.Lane != bootstrap.Lane || historical.ReopenOperationID != first.ReopenOperationID || historical.SessionLaunches != 0 || len(historical.HostRuns) != 0 {
		t.Fatalf("historical explicit lane did not replay operation 1: %+v", historical)
	}
	assertDailyCaseFilesEqual(t, beforeHistoricalReplay, snapshotDailyCaseFiles(t, caseRoot))
}

func TestRunDailyTerminalCorrectionMultipleClosedLanesRequiresChoiceWithoutWrite(t *testing.T) {
	caseRoot := filepath.Join(t.TempDir(), "multiple-terminal-correction")
	actor := "daily-test"
	bootstrap := DailyResult{CaseRoot: caseRoot}
	inspection, err := applyDailyOnboarding(caseRoot, "multiple terminal correction goal", actor, &bootstrap)
	if err != nil {
		t.Fatal(err)
	}
	bootstrap.Lane = inspection.Identity.InitialLane
	if err := ensureDailyStarted(caseRoot, inspection.Identity.Pack, &bootstrap); err != nil {
		t.Fatal(err)
	}
	if _, err := workstream.StartApply(sessionhostAttachedRepoRoot(t, caseRoot, inspection.Identity.Pack), caseRoot, inspection.Identity.Pack, workstream.StartOptions{
		Name: "login", Actor: actor, Executor: "session-login", TakeoverReason: "multiple terminal correction choice regression",
	}); err != nil {
		t.Fatal(err)
	}
	closed := []string{bootstrap.Lane, "binary-analysis-login"}
	for _, lane := range closed {
		evidenceRef := sessionhostStateRel(t, caseRoot, "lanes", lane, "workspace", "completion-evidence.md")
		evidencePath := filepath.Join(caseRoot, filepath.FromSlash(evidenceRef))
		if err := os.MkdirAll(filepath.Dir(evidencePath), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(evidencePath, []byte("reviewed completion evidence for "+lane+"\n"), 0o600); err != nil {
			t.Fatal(err)
		}
		completeOpt := workstream.CompleteOptions{Selector: lane, Actor: actor, Reason: "close lane for terminal correction choice", EvidenceRefs: evidenceRef}
		preview, err := workstream.CompletePreview(sessionhostAttachedRepoRoot(t, caseRoot, inspection.Identity.Pack), caseRoot, inspection.Identity.Pack, completeOpt)
		if err != nil {
			t.Fatal(err)
		}
		completeOpt.ExpectedPreviewSHA256 = preview.CompletionPlanSHA256
		if completed, err := workstream.CompleteApply(sessionhostAttachedRepoRoot(t, caseRoot, inspection.Identity.Pack), caseRoot, inspection.Identity.Pack, completeOpt); err != nil || !completed.Applied {
			t.Fatalf("complete %s: result=%+v err=%v", lane, completed, err)
		}
	}

	before := snapshotDailyCaseFiles(t, caseRoot)
	result, err := RunDaily(context.Background(), DailyOptions{Target: caseRoot, Correction: "reopen only the lane I select", Actor: actor})
	if err != nil {
		t.Fatalf("multiple terminal correction choices: result=%+v err=%v", result, err)
	}
	if result.Action == nil || result.Action.Code != DailyActionBlocked || !result.Action.RequiresInput || len(result.Action.Choices) != len(closed) || result.SessionLaunches != 0 || len(result.HostRuns) != 0 || len(result.DriverSteps) != 0 {
		t.Fatalf("multiple terminal correction did not stop at typed choice: %+v", result)
	}
	ids := make([]string, 0, len(result.Action.Choices))
	for _, choice := range result.Action.Choices {
		ids = append(ids, choice.ID)
	}
	for _, lane := range closed {
		if !slices.Contains(ids, lane) {
			t.Fatalf("multiple terminal correction omitted %s: %+v", lane, result.Action.Choices)
		}
	}
	assertDailyCaseFilesEqual(t, before, snapshotDailyCaseFiles(t, caseRoot))
}

func TestRunDailyTerminalCorrectionRecoversExactPendingReopen(t *testing.T) {
	caseRoot := filepath.Join(t.TempDir(), "terminal-correction-recovery")
	actor := "daily-test"
	correction := "resume the exact terminal correction after interruption"
	bootstrap := DailyResult{CaseRoot: caseRoot}
	inspection, err := applyDailyOnboarding(caseRoot, "terminal correction recovery goal", actor, &bootstrap)
	if err != nil {
		t.Fatal(err)
	}
	bootstrap.Lane = inspection.Identity.InitialLane
	if err := ensureDailyStarted(caseRoot, inspection.Identity.Pack, &bootstrap); err != nil {
		t.Fatal(err)
	}
	evidenceRef := sessionhostStateRel(t, caseRoot, "lanes", bootstrap.Lane, "workspace", "completion-evidence.md")
	evidencePath := filepath.Join(caseRoot, filepath.FromSlash(evidenceRef))
	if err := os.MkdirAll(filepath.Dir(evidencePath), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(evidencePath, []byte("reviewed completion evidence before recoverable reopen\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	completeOpt := workstream.CompleteOptions{Selector: bootstrap.Lane, Actor: actor, Reason: "publish terminal correction recovery fixture", EvidenceRefs: evidenceRef}
	preview, err := workstream.CompletePreview(sessionhostAttachedRepoRoot(t, caseRoot, inspection.Identity.Pack), caseRoot, inspection.Identity.Pack, completeOpt)
	if err != nil {
		t.Fatal(err)
	}
	completeOpt.ExpectedPreviewSHA256 = preview.CompletionPlanSHA256
	if completed, err := workstream.CompleteApply(sessionhostAttachedRepoRoot(t, caseRoot, inspection.Identity.Pack), caseRoot, inspection.Identity.Pack, completeOpt); err != nil || !completed.Applied {
		t.Fatalf("complete recovery fixture: result=%+v err=%v", completed, err)
	}

	restore := workstream.SetReopenAfterOperationIntentHookForTest(func() error { return errors.New("stop after daily reopen intent") })
	interrupted, err := RunDaily(context.Background(), DailyOptions{Target: caseRoot, Correction: correction, Actor: actor})
	restore()
	if err == nil || !strings.Contains(err.Error(), "stop after daily reopen intent") {
		t.Fatalf("terminal correction interruption result=%+v err=%v", interrupted, err)
	}
	if !containsDailyStep(interrupted.DriverSteps, "reopen") || interrupted.Failure == nil || !interrupted.Failure.MutationApplied || interrupted.Failure.MutationBoundary != "durable-runtime-step-may-have-committed" {
		t.Fatalf("terminal correction interruption omitted its durable mutation boundary: %+v", interrupted)
	}
	operations, err := lanecompletion.InspectOperations(caseRoot)
	if err != nil || !operations.Pending {
		t.Fatalf("interrupted correction did not leave an exact pending operation: %+v err=%v", operations, err)
	}
	beforeMismatch := snapshotDailyCaseFiles(t, caseRoot)
	mismatch, err := RunDaily(context.Background(), DailyOptions{Target: caseRoot, Correction: "a different terminal correction", Actor: actor})
	if err == nil || !strings.Contains(err.Error(), "exact original correction") {
		t.Fatalf("mismatched pending recovery result=%+v err=%v", mismatch, err)
	}
	assertDailyCaseFilesEqual(t, beforeMismatch, snapshotDailyCaseFiles(t, caseRoot))

	recovered, err := RunDaily(context.Background(), DailyOptions{Target: caseRoot, Correction: correction, Actor: actor})
	if err != nil {
		t.Fatalf("recover pending terminal correction: result=%+v err=%v", recovered, err)
	}
	if recovered.FinalState != "terminal-correction-reopened" || recovered.Action == nil || recovered.Action.Code != DailyActionReadyToContinue || recovered.ReopenOperationCommit == nil || recovered.ReopenOperationCommit.State != "committed" || recovered.SessionLaunches != 0 || len(recovered.HostRuns) != 0 || !containsDailyStep(recovered.DriverSteps, "reopen") {
		t.Fatalf("pending terminal correction recovery crossed its boundary: %+v", recovered)
	}
	operations, err = lanecompletion.InspectOperations(caseRoot)
	if err != nil || operations.Pending || operations.LatestSequence != 1 {
		t.Fatalf("pending terminal correction did not commit exactly once: %+v err=%v", operations, err)
	}
	replayed, err := RunDaily(context.Background(), DailyOptions{Target: caseRoot, Correction: correction, Actor: actor})
	if err != nil || !replayed.Replay || replayed.ReopenOperationID != recovered.ReopenOperationID || replayed.FinalState != "terminal-correction-reopened" || replayed.SessionLaunches != 0 {
		t.Fatalf("committed terminal correction did not replay exactly: result=%+v err=%v", replayed, err)
	}
	operations, err = lanecompletion.InspectOperations(caseRoot)
	if err != nil || operations.LatestSequence != 1 || len(operations.Commits) != 1 {
		t.Fatalf("committed replay changed operation history: %+v err=%v", operations, err)
	}
	board, err := mission.ReadBoard(caseRoot)
	if err != nil {
		t.Fatal(err)
	}
	for index := range board.Lanes {
		if board.Lanes[index].ID == bootstrap.Lane {
			board.Lanes[index].CurrentExecutor = "post-reopen-session"
		}
	}
	writeDailyTestBoard(t, caseRoot, board)
	stale, err := RunDaily(context.Background(), DailyOptions{Target: caseRoot, Correction: correction, Actor: actor})
	if err == nil || !strings.Contains(err.Error(), "current board lane") {
		t.Fatalf("advanced committed correction replay result=%+v err=%v", stale, err)
	}
	if len(stale.DriverSteps) != 0 || stale.Failure == nil || stale.Failure.MutationApplied {
		t.Fatalf("advanced committed correction replay was reported as a new mutation: %+v", stale)
	}
}

func TestRunDailyTerminalCorrectionRejectsStaleCompletionAfterPreview(t *testing.T) {
	caseRoot := filepath.Join(t.TempDir(), "terminal-correction-stale")
	actor := "daily-test"
	bootstrap := DailyResult{CaseRoot: caseRoot}
	inspection, err := applyDailyOnboarding(caseRoot, "terminal correction stale goal", actor, &bootstrap)
	if err != nil {
		t.Fatal(err)
	}
	bootstrap.Lane = inspection.Identity.InitialLane
	if err := ensureDailyStarted(caseRoot, inspection.Identity.Pack, &bootstrap); err != nil {
		t.Fatal(err)
	}
	evidenceRef := sessionhostStateRel(t, caseRoot, "lanes", bootstrap.Lane, "workspace", "completion-evidence.md")
	evidencePath := filepath.Join(caseRoot, filepath.FromSlash(evidenceRef))
	if err := os.MkdirAll(filepath.Dir(evidencePath), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(evidencePath, []byte("completion evidence before stale preview\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	completeOpt := workstream.CompleteOptions{Selector: bootstrap.Lane, Actor: actor, Reason: "publish stale terminal correction fixture", EvidenceRefs: evidenceRef}
	preview, err := workstream.CompletePreview(sessionhostAttachedRepoRoot(t, caseRoot, inspection.Identity.Pack), caseRoot, inspection.Identity.Pack, completeOpt)
	if err != nil {
		t.Fatal(err)
	}
	completeOpt.ExpectedPreviewSHA256 = preview.CompletionPlanSHA256
	if completed, err := workstream.CompleteApply(sessionhostAttachedRepoRoot(t, caseRoot, inspection.Identity.Pack), caseRoot, inspection.Identity.Pack, completeOpt); err != nil || !completed.Applied {
		t.Fatalf("complete stale fixture: result=%+v err=%v", completed, err)
	}

	previous := dailyTerminalCorrectionAfterPreviewHook
	dailyTerminalCorrectionAfterPreviewHook = func() error {
		board, err := mission.ReadBoard(caseRoot)
		if err != nil {
			return err
		}
		board.UpdatedAt = "2026-08-12T23:59:59Z"
		writeDailyTestBoard(t, caseRoot, board)
		return nil
	}
	t.Cleanup(func() { dailyTerminalCorrectionAfterPreviewHook = previous })
	result, err := RunDaily(context.Background(), DailyOptions{Target: caseRoot, Correction: "reopen only if the reviewed terminal state stays current", Actor: actor})
	if err == nil || (!strings.Contains(err.Error(), "changed") && !strings.Contains(err.Error(), "mismatch")) {
		t.Fatalf("stale terminal correction result=%+v err=%v", result, err)
	}
	if result.ReopenOperationID != "" || result.ReopenOperationCommit != nil || containsDailyStep(result.DriverSteps, "reopen") || result.SessionLaunches != 0 {
		t.Fatalf("stale terminal correction crossed zero-commit boundary: %+v", result)
	}
	lifecycle, err := lanecompletion.Inspect(caseRoot, bootstrap.Lane)
	if err != nil || lifecycle.State != lanecompletion.StateComplete || lifecycle.CurrentCompletion == nil {
		t.Fatalf("stale terminal correction changed lifecycle: %+v err=%v", lifecycle, err)
	}
}

func TestRunDailyRefusesPendingCompletionAsTerminalReplay(t *testing.T) {
	caseRoot := filepath.Join(t.TempDir(), "pending-completion")
	bootstrap := DailyResult{CaseRoot: caseRoot}
	inspection, err := applyDailyOnboarding(caseRoot, "pending completion goal", "daily-test", &bootstrap)
	if err != nil {
		t.Fatal(err)
	}
	bootstrap.Lane = inspection.Identity.InitialLane
	if err := ensureDailyStarted(caseRoot, inspection.Identity.Pack, &bootstrap); err != nil {
		t.Fatal(err)
	}
	boardPath := sessionhostStatePath(t, caseRoot, "board.json")
	board, err := mission.ReadBoard(caseRoot)
	if err != nil {
		t.Fatal(err)
	}
	lane, ok := mission.LookupBoardLane(board.Lanes, bootstrap.Lane, false)
	if !ok {
		t.Fatalf("started lane missing from board: %+v", board.Lanes)
	}
	for index := range board.Lanes {
		if board.Lanes[index].ID == bootstrap.Lane {
			board.Lanes[index].Status = "closed"
		}
	}
	boardBytes, err := json.MarshalIndent(board, "", "  ")
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(boardPath, append(boardBytes, '\n'), 0o600); err != nil {
		t.Fatal(err)
	}
	intent := lanecompletion.CompletionIntent{
		SchemaVersion:      1,
		Kind:               "lane-completion-intent",
		Sequence:           1,
		Lane:               bootstrap.Lane,
		Label:              "mission",
		PreviousStatus:     "active",
		Actor:              "daily-test",
		Reason:             "simulate interrupted publication without LLM output",
		EvidenceRefs:       []string{},
		Evidence:           []lanecompletion.Evidence{},
		CurrentExecutor:    lane.CurrentExecutor,
		ExecutorGeneration: lane.ExecutorGeneration,
		CreatedAt:          "2026-08-07T01:02:03Z",
		EventID:            "lane-completed-pending-test",
		PreviewSHA256:      strings.Repeat("a", 64),
		NoAuthority:        true,
		NoConfirmed:        true,
		NoHeavyTool:        true,
	}
	intentBytes, err := json.MarshalIndent(intent, "", "  ")
	if err != nil {
		t.Fatal(err)
	}
	intentPath := lanecompletion.IntentPath(caseRoot, bootstrap.Lane, 1, "complete")
	if err := os.WriteFile(intentPath, append(intentBytes, '\n'), 0o600); err != nil {
		t.Fatal(err)
	}
	result, err := RunDaily(context.Background(), DailyOptions{
		Target:                            caseRoot,
		Goal:                              inspection.Identity.Goal,
		ClaudePath:                        missingClaudePath(t),
		ExpectedClaudeExecutableSHA256:    strings.Repeat("0", 64),
		ExpectedClaudeExecutablePublisher: liveAcceptanceClaudePublisher,
	})
	if err == nil || (!strings.Contains(err.Error(), "pending lane completion publication") && !strings.Contains(err.Error(), "completion publication is incomplete")) {
		t.Fatalf("pending completion result=%+v err=%v", result, err)
	}
	if result.Replay || result.SessionLaunches != 0 || result.SessionCompletions != 0 {
		t.Fatalf("pending completion was treated as terminal replay: %+v", result)
	}
	if _, err := os.Lstat(lanecompletion.ReceiptPath(caseRoot, bootstrap.Lane, 1, "complete")); !os.IsNotExist(err) {
		t.Fatalf("daily replay wrote pending completion commit: %v", err)
	}
}

func TestRunDailyRefusesArchivedLaneWithoutDurableArchiveTransition(t *testing.T) {
	caseRoot := filepath.Join(t.TempDir(), "archived-lane")
	bootstrap := DailyResult{CaseRoot: caseRoot}
	inspection, err := applyDailyOnboarding(caseRoot, "archived lane goal", "daily-test", &bootstrap)
	if err != nil {
		t.Fatal(err)
	}
	bootstrap.Lane = inspection.Identity.InitialLane
	if err := ensureDailyStarted(caseRoot, inspection.Identity.Pack, &bootstrap); err != nil {
		t.Fatal(err)
	}
	board, err := mission.ReadBoard(caseRoot)
	if err != nil {
		t.Fatal(err)
	}
	for index := range board.Lanes {
		if board.Lanes[index].ID == bootstrap.Lane {
			board.Lanes[index].Status = "archived"
		}
	}
	boardBytes, err := json.MarshalIndent(board, "", "  ")
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(sessionhostStatePath(t, caseRoot, "board.json"), append(boardBytes, '\n'), 0o600); err != nil {
		t.Fatal(err)
	}
	result, err := RunDaily(context.Background(), DailyOptions{Target: caseRoot, Goal: inspection.Identity.Goal, SelectedLane: bootstrap.Lane})
	if err == nil || !strings.Contains(err.Error(), "no durable archive transition is supported") {
		t.Fatalf("explicit archived lane result=%+v err=%v", result, err)
	}
	if result.Replay || result.SessionLaunches != 0 {
		t.Fatalf("explicit archived lane was treated as replay: %+v", result)
	}
}

func TestRunDailyCorrectionRefusesPendingCompletionAsTerminalReplay(t *testing.T) {
	caseRoot := filepath.Join(t.TempDir(), "pending-correction-completion")
	actor := "daily-test"
	correction := "retry the durable correction without launching another member"
	bootstrap := DailyResult{CaseRoot: caseRoot}
	inspection, err := applyDailyOnboarding(caseRoot, "pending correction completion goal", actor, &bootstrap)
	if err != nil {
		t.Fatal(err)
	}
	bootstrap.Lane = inspection.Identity.InitialLane
	if err := ensureDailyStarted(caseRoot, inspection.Identity.Pack, &bootstrap); err != nil {
		t.Fatal(err)
	}
	board, err := mission.ReadBoard(caseRoot)
	if err != nil {
		t.Fatal(err)
	}
	lane, ok := mission.LookupBoardLane(board.Lanes, bootstrap.Lane, false)
	if !ok {
		t.Fatalf("started lane missing from board: %+v", board.Lanes)
	}
	eventID := dailyCorrectionEventID(dailyCorrectionScope(caseRoot, inspection), bootstrap.Lane, correction)
	correctionEvent := map[string]any{
		"schemaVersion": 1, "eventId": eventID, "kind": "intervention", "lane": bootstrap.Lane,
		"subject": "daily human correction", "summary": correction, "actor": actor,
		"action": "override", "status": "open", "target": missionintent.MissionIntentRel,
	}
	resolutionTime := "2026-08-07T01:02:03Z"
	resolutionEvent := map[string]any{
		"schemaVersion": 1, "eventId": "daily-correction-resolution-pending-test", "kind": "intervention", "lane": bootstrap.Lane,
		"subject": "reconcile pending correction", "summary": "deterministic lifecycle fixture without LLM output",
		"action": "reconcile", "status": "resolved", "resolvesEventId": eventID,
		"target": missionintent.MissionIntentRel, "actor": actor, "executor": lane.CurrentExecutor,
		"reason": "simulate interrupted completion publication", "time": resolutionTime,
	}
	_, interventionPath, err := mission.FactPath(caseRoot, "intervention")
	if err != nil {
		t.Fatal(err)
	}
	if err := mission.AppendJSONLine(interventionPath, correctionEvent); err != nil {
		t.Fatal(err)
	}
	if err := mission.AppendJSONLine(interventionPath, resolutionEvent); err != nil {
		t.Fatal(err)
	}
	for index := range board.Lanes {
		if board.Lanes[index].ID == bootstrap.Lane {
			board.Lanes[index].Status = "closed"
			board.Lanes[index].LastReconciledIntervention = eventID
			board.Lanes[index].LastReconcileAt = resolutionTime
		}
	}
	boardBytes, err := json.MarshalIndent(board, "", "  ")
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(sessionhostStatePath(t, caseRoot, "board.json"), append(boardBytes, '\n'), 0o600); err != nil {
		t.Fatal(err)
	}
	intent := lanecompletion.CompletionIntent{
		SchemaVersion: 1, Kind: "lane-completion-intent", Sequence: 1, Lane: bootstrap.Lane, Label: "mission",
		PreviousStatus: "active", Actor: actor, Reason: "simulate interrupted completion publication without LLM output",
		EvidenceRefs: []string{}, Evidence: []lanecompletion.Evidence{}, CurrentExecutor: lane.CurrentExecutor,
		ExecutorGeneration: lane.ExecutorGeneration, CreatedAt: resolutionTime, EventID: "lane-completed-pending-correction-test",
		PreviewSHA256: strings.Repeat("a", 64), NoAuthority: true, NoConfirmed: true, NoHeavyTool: true,
	}
	intentBytes, err := json.MarshalIndent(intent, "", "  ")
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(lanecompletion.IntentPath(caseRoot, bootstrap.Lane, 1, "complete"), append(intentBytes, '\n'), 0o600); err != nil {
		t.Fatal(err)
	}

	result, err := RunDaily(context.Background(), DailyOptions{
		Target: caseRoot, Correction: correction, SelectedLane: inspection.Identity.InitialLane, Actor: actor, ClaudePath: missingClaudePath(t),
		ExpectedClaudeExecutableSHA256: strings.Repeat("0", 64), ExpectedClaudeExecutablePublisher: liveAcceptanceClaudePublisher,
	})
	if err == nil || (!strings.Contains(err.Error(), "pending lane completion publication") && !strings.Contains(err.Error(), "completion publication is incomplete")) {
		t.Fatalf("pending correction completion result=%+v err=%v", result, err)
	}
	if result.Replay || result.SessionLaunches != 0 || result.SessionCompletions != 0 {
		t.Fatalf("pending correction completion was treated as terminal replay: %+v", result)
	}
}

func TestRunDailyCorrectionReplacementGenerationRequiresFreshTypedInput(t *testing.T) {
	caseRoot := filepath.Join(t.TempDir(), "correction-input-readiness")
	actor := "daily-test"
	bootstrap := DailyResult{CaseRoot: caseRoot}
	inspection, err := applyDailyOnboarding(caseRoot, "correction input readiness goal", actor, &bootstrap)
	if err != nil {
		t.Fatal(err)
	}
	lane := inspection.Identity.InitialLane
	if err := ensureDailyStarted(caseRoot, inspection.Identity.Pack, &bootstrap, lane); err != nil {
		t.Fatal(err)
	}
	artifact := filepath.Join(caseRoot, "inputs", "target.bin")
	if err := os.MkdirAll(filepath.Dir(artifact), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(artifact, []byte("bounded target"), 0o600); err != nil {
		t.Fatal(err)
	}
	binding, err := memberexecution.ArtifactAnalysisTaskBinding(caseRoot, "inputs/target.bin")
	if err != nil {
		t.Fatal(err)
	}
	board, err := mission.ReadBoard(caseRoot)
	if err != nil {
		t.Fatal(err)
	}
	owner, found := mission.LookupBoardLane(board.Lanes, lane, false)
	if !found {
		t.Fatalf("current lane %q is missing", lane)
	}
	if _, _, err := memberexecution.WriteTaskBindingForOwner(caseRoot, lane, owner.CurrentExecutor, owner.ExecutorGeneration, binding); err != nil {
		t.Fatal(err)
	}
	manifestRef := writeDailyReviewerRejectionFixture(t, caseRoot, inspection.Identity.Pack, lane, actor)
	if _, rejected, err := workstream.CurrentMemberManifestReviewerRejection(caseRoot, lane, manifestRef); err != nil || !rejected {
		t.Fatalf("reviewer rejection fixture is not canonical: rejected=%t err=%v", rejected, err)
	}

	result, err := RunDaily(context.Background(), DailyOptions{
		Target:                            caseRoot,
		SelectedLane:                      lane,
		Correction:                        "apply the bounded reviewer correction",
		Actor:                             actor,
		ClaudePath:                        missingClaudePath(t),
		ExpectedClaudeExecutableSHA256:    strings.Repeat("0", 64),
		ExpectedClaudeExecutablePublisher: liveAcceptanceClaudePublisher,
	})
	if err != nil {
		t.Fatalf("corrected generation readiness result=%+v err=%v", result, err)
	}
	if result.FinalState != "input-required" || !result.Blocked || result.Action == nil || result.Action.Code != "input-required" || result.SessionLaunches != 0 || result.SessionCompletions != 0 {
		t.Fatalf("replacement generation bypassed typed input readiness: %+v", result)
	}
	if result.ExecutorGeneration <= owner.ExecutorGeneration {
		t.Fatalf("correction did not publish a replacement owner generation: before=%d result=%+v", owner.ExecutorGeneration, result)
	}
	current, rel, bindingSHA256, err := memberexecution.ReadTaskBindingForOwner(caseRoot, lane, result.ExecutorGeneration)
	if err != nil {
		t.Fatal(err)
	}
	if current != nil || rel == "" || bindingSHA256 != "" {
		t.Fatalf("replacement generation inherited typed input binding: rel=%q sha=%q binding=%+v", rel, bindingSHA256, current)
	}
}

func TestRunDailyExplicitReviewerRejectionReplayPreservesLaneWithoutMutation(t *testing.T) {
	caseRoot := filepath.Join(t.TempDir(), "explicit-reviewer-rejection")
	actor := "daily-test"
	bootstrap := DailyResult{CaseRoot: caseRoot}
	inspection, err := applyDailyOnboarding(caseRoot, "explicit reviewer rejection replay goal", actor, &bootstrap)
	if err != nil {
		t.Fatal(err)
	}
	lane := inspection.Identity.InitialLane
	if err := ensureDailyStarted(caseRoot, inspection.Identity.Pack, &bootstrap, lane); err != nil {
		t.Fatal(err)
	}
	manifestRef := writeDailyReviewerRejectionFixture(t, caseRoot, inspection.Identity.Pack, lane, actor)
	if _, rejected, err := workstream.CurrentMemberManifestReviewerRejection(caseRoot, lane, manifestRef); err != nil || !rejected {
		t.Fatalf("reviewer rejection fixture is not canonical: rejected=%t err=%v", rejected, err)
	}

	before := snapshotDailyCaseFiles(t, caseRoot)
	result, err := RunDaily(context.Background(), DailyOptions{
		Target:                            caseRoot,
		SelectedLane:                      lane,
		ClaudePath:                        missingClaudePath(t),
		ExpectedClaudeExecutableSHA256:    strings.Repeat("0", 64),
		ExpectedClaudeExecutablePublisher: liveAcceptanceClaudePublisher,
		beforeMemberRun: func(_, _, _ string) error {
			t.Fatal("reviewer rejection replay reached member execution")
			return nil
		},
	})
	if err != nil {
		t.Fatalf("explicit reviewer rejection replay: result=%+v err=%v", result, err)
	}
	if result.Lane != lane || result.FinalState != "reviewer-rejected-awaiting-correction" || !result.Blocked || !result.Replay || result.Action == nil || result.Action.Code != DailyActionWaitingForCorrection || result.SessionLaunches != 0 || result.SessionCompletions != 0 || result.Replacements != 0 || len(result.HostRuns) != 0 || len(result.DriverSteps) != 0 {
		t.Fatalf("explicit reviewer rejection replay lost its canonical projection: %+v", result)
	}
	assertDailyCaseFilesEqual(t, before, snapshotDailyCaseFiles(t, caseRoot))
}

func writeDailyReviewerRejectionFixture(t *testing.T, caseRoot, pack, lane, actor string) string {
	t.Helper()
	if current, err := memberexecution.CurrentTaskBinding(caseRoot, lane); err != nil {
		t.Fatal(err)
	} else if current == nil && pack == defaults.DefaultPack {
		owner, err := laneowner.Read(caseRoot, lane)
		if err != nil {
			t.Fatal(err)
		}
		binding, err := memberexecution.WorkspaceInventoryTaskBinding(caseRoot, ".")
		if err != nil {
			t.Fatal(err)
		}
		if _, _, err := memberexecution.WriteTaskBindingForOwner(
			caseRoot, lane, owner.CurrentExecutor, owner.ExecutorGeneration, binding,
		); err != nil {
			t.Fatal(err)
		}
	}
	status, err := runPublicStatus(caseRoot, pack, lane)
	if err != nil || status.MissionControlRunbook == nil || status.MissionControlRunbook.CurrentDriverRequest == nil {
		t.Fatalf("read member dispatch request: status=%+v err=%v", status, err)
	}
	requestSHA256, err := mission.MissionCommanderDriverRequestSHA256(*status.MissionControlRunbook.CurrentDriverRequest)
	if err != nil {
		t.Fatal(err)
	}
	dispatch, err := memberexecution.PreviewDispatch(memberexecution.DispatchOptions{CaseRoot: caseRoot, Pack: pack, Lane: lane, RequestSHA256: requestSHA256, CreatedAt: "2026-08-12T01:00:00Z"})
	if err != nil {
		t.Fatal(err)
	}
	if _, err := memberexecution.Apply(dispatch, dispatch.ExpectedPlanSHA256); err != nil {
		t.Fatal(err)
	}
	accepted, err := memberexecution.PreviewObservation(memberexecution.ObservationOptions{CaseRoot: caseRoot, Pack: pack, Lane: lane, AttemptID: dispatch.AttemptID, Outcome: "accepted", Actor: actor, ObservedAt: "2026-08-12T01:01:00Z"})
	if err != nil {
		t.Fatal(err)
	}
	if _, err := memberexecution.Apply(accepted, accepted.ExpectedPlanSHA256); err != nil {
		t.Fatal(err)
	}

	output := []byte("bounded member result for reviewer rejection\n")
	manifest := memberexecution.ResultManifest{
		SchemaVersion: 1, Kind: memberexecution.KindManifest, AttemptID: dispatch.AttemptID, Owner: dispatch.Owner,
		Summary: "bounded member result", Outputs: []memberexecution.Output{{Path: "review-items.txt", SHA256: bytesSHA256(output), Bytes: int64(len(output))}},
		ReviewerItemsPath: "review-items.txt", NoAuthority: true, NoConfirmed: true, NoHeavyTool: true,
	}
	manifestData, err := memberexecution.MarshalResultManifest(manifest)
	if err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(dispatch.Inspection.OutputsRoot, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dispatch.Inspection.OutputsRoot, "review-items.txt"), output, 0o600); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(dispatch.Inspection.ManifestPath, manifestData, 0o600); err != nil {
		t.Fatal(err)
	}
	returned, err := memberexecution.PreviewObservation(memberexecution.ObservationOptions{
		CaseRoot: caseRoot, Pack: pack, Lane: lane, AttemptID: dispatch.AttemptID, Outcome: "returned", Actor: actor, ObservedAt: "2026-08-12T01:02:00Z",
	})
	if err != nil {
		t.Fatal(err)
	}
	if _, err := memberexecution.Apply(returned, returned.ExpectedPlanSHA256); err != nil {
		t.Fatal(err)
	}
	latest, found, err := memberexecution.Latest(caseRoot, lane)
	if err != nil || !found || latest.State != "intake-ready" || latest.Manifest == nil {
		t.Fatalf("member rejection fixture is not intake-ready: latest=%+v found=%t err=%v", latest, found, err)
	}
	manifestRef := relativeLiveAcceptancePath(caseRoot, latest.ManifestPath)

	planCommand := "/rekit plan-subagents -Target " + quoteDailyCommandArg(caseRoot) + " -Pack " + pack + " -TaskType feature-analysis -Items " + quoteDailyCommandArg(manifestRef) + " -Lane " + lane + " -Format json"
	var reviewerPlan struct {
		PacketID      string `json:"packetId"`
		PacketPath    string `json:"packetPath"`
		TargetLane    string `json:"targetLane"`
		ShardHandoffs []struct {
			ShardID string `json:"shardId"`
		} `json:"shardHandoffs"`
	}
	if err := runPublicCommandJSON(planCommand, &reviewerPlan); err != nil {
		t.Fatal(err)
	}
	if reviewerPlan.PacketPath == "" || reviewerPlan.TargetLane != lane || len(reviewerPlan.ShardHandoffs) != 1 {
		t.Fatalf("reviewer rejection plan identity is incomplete: %+v", reviewerPlan)
	}

	hostOpt := Options{Target: caseRoot, Pack: pack, SelectedLane: lane, Actor: actor}
	spawn, err := runCurrentStep(hostOpt, []string{"-ReviewerHarness", "daily-test-harness", "-ReviewerSession", "daily-test-reviewer", "-Actor", actor}, false)
	if err != nil {
		t.Fatal(err)
	}
	if err := applyCurrentStep(hostOpt, spawn, []string{"-ReviewerHarness", "daily-test-harness", "-ReviewerSession", "daily-test-reviewer", "-Actor", actor}); err != nil {
		t.Fatal(err)
	}
	status, err = runPublicStatus(caseRoot, pack, lane)
	if err != nil {
		t.Fatal(err)
	}
	operator := dailyReviewerOperatorPackage(t, caseRoot, pack, lane)
	if operator.PacketID == "" || operator.RouteID == "" || operator.TargetLane != lane || operator.Current == nil || operator.Current.ShardID != reviewerPlan.ShardHandoffs[0].ShardID {
		t.Fatalf("reviewer rejection operator identity is incomplete: %+v", operator)
	}
	resultData, err := json.Marshal(map[string]any{
		"packetId": operator.PacketID, "routeId": operator.RouteID, "shardId": reviewerPlan.ShardHandoffs[0].ShardID, "items": []string{manifestRef},
		"reviewerSession": "daily-test-reviewer", "decision": "reject", "confidence": "high", "summary": "bounded member result requires correction", "evidenceRefs": []string{manifestRef}, "risks": []string{"missing bounded acceptance detail"}, "conflicts": []string{}, "recommendedVerdict": "rejected",
		"routeOutput": map[string]any{"item": manifestRef, "decision": "reject", "confidence": "high", "evidence": manifestRef, "risk": "missing bounded acceptance detail", "next_action": "main-agent-writeback", "tier_used": "light", "tool_scope": "read-only", "feature": "mission", "request_id": "n/a", "candidate_path": "n/a", "defer_reason": "n/a"},
	})
	if err != nil {
		t.Fatal(err)
	}
	resultSource := filepath.Join(filepath.Dir(operator.PacketPath), "results", "incoming", "daily-reviewer-rejection.json")
	if err := os.MkdirAll(filepath.Dir(resultSource), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(resultSource, resultData, 0o600); err != nil {
		t.Fatal(err)
	}
	input, err := runCurrentStep(hostOpt, []string{"-ReviewerResultInputSourcePath", resultSource, "-Actor", actor}, false)
	if err != nil {
		t.Fatal(err)
	}
	if err := applyCurrentStep(hostOpt, input, []string{"-ReviewerResultInputSourcePath", resultSource, "-Actor", actor}); err != nil {
		t.Fatal(err)
	}
	for _, stepID := range []string{"record-completion", "source-capture", "stage-candidate", "collect-result", "intake-results"} {
		operator = dailyReviewerOperatorPackage(t, caseRoot, pack, lane)
		if operator.CurrentRunLoopStepID != stepID {
			t.Fatalf("reviewer rejection pipeline step=%s want=%s operator=%+v", operator.CurrentRunLoopStepID, stepID, operator)
		}
		step, err := runCurrentStep(hostOpt, []string{"-Actor", actor}, false)
		if err != nil {
			t.Fatal(err)
		}
		if err := applyCurrentStep(hostOpt, step, []string{"-Actor", actor}); err != nil {
			t.Fatal(err)
		}
	}
	return manifestRef
}

func dailyReviewerOperatorPackage(t *testing.T, caseRoot, pack, lane string) *workstream.ReviewerDispatchOperatorPackage {
	t.Helper()
	var status struct {
		CaseMission *struct {
			ReviewerDispatchIntakeSummary workstream.ReviewerDispatchIntakeSummary `json:"reviewerDispatchIntakeSummary"`
		} `json:"caseMission"`
	}
	if err := runPublicCLI([]string{"-Command", "status", "-Target", caseRoot, "-Pack", pack, "-Lane", lane, "-Format", "json"}, &status); err != nil {
		t.Fatal(err)
	}
	if status.CaseMission == nil || status.CaseMission.ReviewerDispatchIntakeSummary.OperatorPackage == nil {
		t.Fatalf("reviewer operator package is missing: %+v", status)
	}
	return status.CaseMission.ReviewerDispatchIntakeSummary.OperatorPackage
}

func runPublicCommandJSON(command string, target any) error {
	args, err := cli.SplitPublicCommand(command)
	if err != nil {
		return err
	}
	return runPublicCLI(args, target)
}

func quoteDailyCommandArg(value string) string {
	return `"` + strings.ReplaceAll(value, `"`, `\"`) + `"`
}

func TestFinishDailyCompletionPreservesReviewerRejectionState(t *testing.T) {
	result := DailyResult{FinalState: "reviewer-rejected-awaiting-correction"}
	if err := finishDailyCompletion(t.TempDir(), liveAcceptancePack, &result); err != nil {
		t.Fatal(err)
	}
	if result.FinalState != "reviewer-rejected-awaiting-correction" || !result.Blocked {
		t.Fatalf("typed reviewer rejection was not preserved: %+v", result)
	}
}

func TestExistingDailyCorrectionByIDDistinguishesRepeatedTextAcrossRejections(t *testing.T) {
	caseRoot := t.TempDir()
	lane := "feature-mission"
	actor := "daily-test"
	correction := "apply the same correction text to the current rejection"
	firstID := "daily-correction-first-rejection"
	secondID := "daily-correction-second-rejection"
	_, interventionPath, err := mission.FactPath(caseRoot, "intervention")
	if err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(filepath.Dir(interventionPath), 0o755); err != nil {
		t.Fatal(err)
	}
	for _, eventID := range []string{firstID, secondID} {
		event := map[string]any{
			"schemaVersion": 1, "eventId": eventID, "kind": "intervention", "lane": lane,
			"subject": "daily human correction", "summary": correction, "actor": actor,
			"action": "override", "status": "open",
			"reviewerVerificationEventId": eventID + "-verification",
			"reviewerDecisionEventId":     eventID + "-decision",
		}
		if err := mission.AppendJSONLine(interventionPath, event); err != nil {
			t.Fatal(err)
		}
	}
	first, err := existingDailyCorrectionByID(caseRoot, firstID, lane, correction, actor)
	if err != nil || mission.Value(first, "eventId") != firstID {
		t.Fatalf("first correction event=%v err=%v", first, err)
	}
	second, err := existingDailyCorrectionByID(caseRoot, secondID, lane, correction, actor)
	if err != nil || mission.Value(second, "eventId") != secondID {
		t.Fatalf("second correction event=%v err=%v", second, err)
	}
}

func TestDailyCorrectionLaneFindsResolvedRequestOnOpenReplacementGeneration(t *testing.T) {
	caseRoot := filepath.Join(t.TempDir(), "correction-replacement")
	actor := "daily-source-actor"
	goal := "inspect the supplied research target"
	bootstrap := DailyResult{CaseRoot: caseRoot}
	inspection, err := applyDailyOnboarding(caseRoot, goal, actor, &bootstrap)
	if err != nil {
		t.Fatal(err)
	}
	if err := ensureDailyStarted(caseRoot, inspection.Identity.Pack, &bootstrap); err != nil {
		t.Fatal(err)
	}

	correction := "apply the rejection-bound correction"
	eventID := "daily-correction-rejection-bound"
	_, interventionPath, err := mission.FactPath(caseRoot, "intervention")
	if err != nil {
		t.Fatal(err)
	}
	event := map[string]any{
		"schemaVersion": 1, "eventId": eventID, "kind": "intervention", "lane": inspection.Identity.InitialLane,
		"subject": "daily human correction", "summary": correction, "actor": actor, "action": "override", "status": "open",
		"reviewerVerificationEventId": "verification", "reviewerDecisionEventId": "decision",
	}
	if err := mission.AppendJSONLine(interventionPath, event); err != nil {
		t.Fatal(err)
	}

	lane, boardLane, existing, err := dailyCorrectionLane(caseRoot, inspection, correction, actor)
	if err != nil || lane != inspection.Identity.InitialLane || boardLane.ID != lane || mission.Value(existing, "eventId") != eventID {
		t.Fatalf("open replacement correction lookup: lane=%s board=%+v existing=%v err=%v", lane, boardLane, existing, err)
	}
}

func TestDailyCorrectionLaneUsesLatestReconciledIdentityForRepeatedText(t *testing.T) {
	caseRoot := filepath.Join(t.TempDir(), "repeated-correction-recovery")
	actor := "daily-source-actor"
	goal := "inspect the supplied research target"
	bootstrap := DailyResult{CaseRoot: caseRoot}
	inspection, err := applyDailyOnboarding(caseRoot, goal, actor, &bootstrap)
	if err != nil {
		t.Fatal(err)
	}
	if err := ensureDailyStarted(caseRoot, inspection.Identity.Pack, &bootstrap); err != nil {
		t.Fatal(err)
	}

	correction := "apply the repeated rejection-bound correction"
	firstID := "daily-correction-generation-1"
	secondID := "daily-correction-generation-2"
	firstTime := "2026-08-07T01:02:03Z"
	secondTime := "2026-08-07T02:03:04Z"
	_, interventionPath, err := mission.FactPath(caseRoot, "intervention")
	if err != nil {
		t.Fatal(err)
	}
	for generation, eventID := range []string{firstID, secondID} {
		event := map[string]any{
			"schemaVersion": 1, "eventId": eventID, "kind": "intervention", "lane": inspection.Identity.InitialLane,
			"subject": "daily human correction", "summary": correction, "actor": actor, "action": "override", "status": "open",
			"target":                      ".rekit/member-generation-" + string(rune('1'+generation)) + "/evidence/manifest.json",
			"reviewerVerificationEventId": "verification-" + eventID, "reviewerDecisionEventId": "decision-" + eventID,
		}
		if err := mission.AppendJSONLine(interventionPath, event); err != nil {
			t.Fatal(err)
		}
	}
	for _, resolution := range []map[string]any{
		{
			"schemaVersion": 1, "eventId": "resolution-generation-1", "kind": "intervention", "lane": inspection.Identity.InitialLane,
			"subject": "reconcile first correction", "summary": "first replacement generation", "actor": actor,
			"action": "reconcile", "status": "resolved", "resolvesEventId": firstID,
			"executor": "replacement-generation-2", "time": firstTime,
		},
		{
			"schemaVersion": 1, "eventId": "resolution-generation-2", "kind": "intervention", "lane": inspection.Identity.InitialLane,
			"subject": "reconcile second correction", "summary": "second replacement generation", "actor": actor,
			"action": "reconcile", "status": "resolved", "resolvesEventId": secondID,
			"executor": "replacement-generation-3", "time": secondTime,
		},
	} {
		if err := mission.AppendJSONLine(interventionPath, resolution); err != nil {
			t.Fatal(err)
		}
	}

	board, err := mission.ReadBoard(caseRoot)
	if err != nil {
		t.Fatal(err)
	}
	for index := range board.Lanes {
		if board.Lanes[index].ID == inspection.Identity.InitialLane {
			board.Lanes[index].CurrentExecutor = "replacement-generation-3"
			board.Lanes[index].ExecutorGeneration = 3
			board.Lanes[index].LastReconciledIntervention = secondID
			board.Lanes[index].LastReconcileAt = secondTime
		}
	}
	writeDailyTestBoard(t, caseRoot, board)

	lane, boardLane, existing, err := dailyCorrectionLane(caseRoot, inspection, correction, actor)
	if err != nil || lane != inspection.Identity.InitialLane || boardLane.ID != lane || mission.Value(existing, "eventId") != secondID {
		t.Fatalf("latest reconciled correction lookup: lane=%s board=%+v existing=%v err=%v", lane, boardLane, existing, err)
	}
	resolution, resolved, err := inspectDailyCorrectionResolution(caseRoot, lane, secondID)
	if err != nil || !resolved || resolution.EventID != "resolution-generation-2" || resolution.Time != secondTime || resolution.Executor != "replacement-generation-3" || resolution.ExecutorGeneration != 3 {
		t.Fatalf("latest correction resolution: resolution=%+v resolved=%t err=%v", resolution, resolved, err)
	}

	for _, mutate := range []struct {
		name string
		set  func(*mission.BoardLane)
	}{
		{name: "event-id", set: func(lane *mission.BoardLane) { lane.LastReconciledIntervention = firstID }},
		{name: "resolution-time", set: func(lane *mission.BoardLane) { lane.LastReconcileAt = firstTime }},
		{name: "executor", set: func(lane *mission.BoardLane) { lane.CurrentExecutor = "stale-generation" }},
		{name: "generation", set: func(lane *mission.BoardLane) { lane.ExecutorGeneration = 0 }},
	} {
		t.Run(mutate.name, func(t *testing.T) {
			tampered := board
			tampered.Lanes = append([]mission.BoardLane{}, board.Lanes...)
			for index := range tampered.Lanes {
				if tampered.Lanes[index].ID == lane {
					mutate.set(&tampered.Lanes[index])
				}
			}
			writeDailyTestBoard(t, caseRoot, tampered)
			if _, ok, err := inspectDailyCorrectionResolution(caseRoot, lane, secondID); err == nil || ok {
				t.Fatalf("tampered resolution accepted: ok=%t err=%v", ok, err)
			}
			writeDailyTestBoard(t, caseRoot, board)
		})
	}
}

func writeDailyTestBoard(t *testing.T, caseRoot string, board mission.Board) {
	t.Helper()
	data, err := json.MarshalIndent(board, "", "  ")
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(sessionhostStatePath(t, caseRoot, "board.json"), append(data, '\n'), 0o600); err != nil {
		t.Fatal(err)
	}
}

func TestRunDailyStopsForExecutionEvidenceReviewWithoutTrustedClaude(t *testing.T) {
	caseRoot := filepath.Join(t.TempDir(), "evidence-review-stop")
	bootstrap := DailyResult{CaseRoot: caseRoot}
	inspection, err := applyDailyOnboarding(caseRoot, "review bounded execution evidence", "daily-test", &bootstrap)
	if err != nil {
		t.Fatal(err)
	}
	bootstrap.Lane = inspection.Identity.InitialLane
	if err := ensureDailyStarted(caseRoot, inspection.Identity.Pack, &bootstrap, bootstrap.Lane); err != nil {
		t.Fatal(err)
	}
	observation := `{"kind":"observation","eventId":"obs-daily-host-stop","lane":"` + bootstrap.Lane + `","subject":"bounded adapter output","summary":"preauthorized adapter output ready for review","status":"complete","target":"target-alpha","evidenceRefs":["evidence/debug.json"],"execution":{"gateEventId":"gate-daily-host-stop","authorization":"preauthorized","status":"complete","outputRefs":["workspace/output.txt"]},"gate":{"action":"inspect","authorization":{"decision":"preauthorized"}}}` + "\n"
	writeSessionhostTestFile(t, caseRoot, sessionhostStateRel(t, caseRoot, "facts", "observations.jsonl"), []byte(observation))

	before := snapshotDailyCaseFiles(t, caseRoot)
	result, err := RunDaily(context.Background(), DailyOptions{
		Target:                            caseRoot,
		SelectedLane:                      bootstrap.Lane,
		ClaudePath:                        missingClaudePath(t),
		ExpectedClaudeExecutableSHA256:    strings.Repeat("0", 64),
		ExpectedClaudeExecutablePublisher: liveAcceptanceClaudePublisher,
		MaxAttempts:                       1,
	})
	if err != nil {
		t.Fatal(err)
	}
	if result.FinalState != DailyActionReadyForEvidenceReview || result.Action == nil || result.Action.Code != DailyActionReadyForEvidenceReview || result.SessionLaunches != 0 || result.SessionCompletions != 0 || result.Replacements != 0 || len(result.HostRuns) != 1 || result.HostRuns[0].AppliedSteps != 0 {
		t.Fatalf("daily evidence review stop launched or mutated host work: %+v", result)
	}
	assertDailyCaseFilesEqual(t, before, snapshotDailyCaseFiles(t, caseRoot))
}

func TestRunDailyPrewrittenCorrectionCannotBypassMemberReadiness(t *testing.T) {
	caseRoot := filepath.Join(t.TempDir(), "correction-retry")
	actor := "daily-source-actor"
	goal := "inspect the supplied research target"
	bootstrap := DailyResult{CaseRoot: caseRoot}
	inspection, err := applyDailyOnboarding(caseRoot, goal, actor, &bootstrap)
	if err != nil {
		t.Fatal(err)
	}
	pack := inspection.Identity.Pack
	bootstrap.Lane = inspection.Identity.InitialLane
	if err := ensureDailyStarted(caseRoot, pack, &bootstrap); err != nil {
		t.Fatal(err)
	}

	correction := "prioritize the recovered control-flow evidence"
	eventID := dailyCorrectionEventID(dailyCorrectionScope(caseRoot, inspection), inspection.Identity.InitialLane, correction)
	args := dailyCorrectionArgs(caseRoot, pack, inspection.Identity.InitialLane, correction, actor, eventID, "2026-08-07T01:02:03Z", missionintent.MissionIntentRel)
	var preview note.AppendResult
	if err := runPublicCLI(args, &preview); err != nil {
		t.Fatal(err)
	}
	var recorded note.AppendResult
	if err := runPublicCLI(preview.RecordArgs, &recorded); err != nil || !recorded.Applied {
		t.Fatalf("record correction: applied=%t err=%v", recorded.Applied, err)
	}

	opt := DailyOptions{Target: caseRoot, Correction: correction, SelectedLane: inspection.Identity.InitialLane, Actor: actor, ClaudePath: missingClaudePath(t), ExpectedClaudeExecutableSHA256: strings.Repeat("0", 64), ExpectedClaudeExecutablePublisher: liveAcceptanceClaudePublisher}
	first, firstErr := RunDaily(context.Background(), opt)
	if firstErr == nil || !strings.Contains(firstErr.Error(), "current real member result") {
		t.Fatalf("prewritten correction result=%+v err=%v", first, firstErr)
	}
	if first.ExecutorGeneration != 0 || containsDailyStep(first.DriverSteps, "reconcile") || first.SessionLaunches != 0 {
		t.Fatalf("prewritten correction bypassed member readiness: %+v", first)
	}
	if _, ok, inspectErr := inspectDailyCorrectionResolution(caseRoot, inspection.Identity.InitialLane, eventID); inspectErr != nil || ok {
		t.Fatalf("prewritten correction created resolution: ok=%t err=%v", ok, inspectErr)
	}
}

func TestRunDailyHoldsSharedExecutionLease(t *testing.T) {
	repo := sessionhostTestRepoRoot(t)
	caseRoot := provisionSessionhostAttachedCase(t, repo, "_template")
	entered := make(chan struct{})
	release := make(chan struct{})
	dailyDone := make(chan error, 1)
	go func() {
		_, err := RunDaily(context.Background(), DailyOptions{
			Target: caseRoot,
			Goal:   "hold daily execution fence",
			onCaseReady: func(string) error {
				close(entered)
				<-release
				return errors.New("stop after observing the daily execution fence")
			},
		})
		dailyDone <- err
	}()
	select {
	case <-entered:
	case <-time.After(5 * time.Second):
		t.Fatal("daily did not reach its case-ready boundary")
	}

	exclusiveDone := make(chan struct {
		lease *projectexecution.Lease
		err   error
	}, 1)
	go func() {
		lease, err := projectexecution.AcquireExclusive(caseRoot)
		exclusiveDone <- struct {
			lease *projectexecution.Lease
			err   error
		}{lease, err}
	}()
	select {
	case acquired := <-exclusiveDone:
		if acquired.lease != nil {
			_ = acquired.lease.Unlock()
		}
		close(release)
		t.Fatalf("current-sync exclusive lease crossed active daily: %v", acquired.err)
	case <-time.After(150 * time.Millisecond):
	}
	close(release)
	select {
	case err := <-dailyDone:
		if err == nil || !strings.Contains(err.Error(), "stop after observing") {
			t.Fatalf("daily stop error = %v", err)
		}
	case <-time.After(5 * time.Second):
		t.Fatal("daily did not release its execution lease")
	}
	select {
	case acquired := <-exclusiveDone:
		if acquired.err != nil {
			t.Fatal(acquired.err)
		}
		if err := acquired.lease.Unlock(); err != nil {
			t.Fatal(err)
		}
	case <-time.After(5 * time.Second):
		t.Fatal("current-sync exclusive lease did not acquire after daily returned")
	}
}

func provisionSessionhostAttachedCase(t *testing.T, repo, pack string) string {
	t.Helper()
	caseRoot := filepath.Join(t.TempDir(), "attached")
	plan, err := syncreview.PlanExclusiveInit(repo, caseRoot, pack, syncreview.ExclusiveInitOptions{ProjectName: "attached-demo", ProvisionID: "sessionhost-attached-fixture", Role: "sessionhost-attached-fixture", CreatedAt: time.Date(2026, 8, 7, 0, 0, 0, 0, time.UTC), SkipVerificationMarker: true, DefaultPublicationPhase: 1})
	if err != nil {
		t.Fatal(err)
	}
	if _, err := syncreview.ApplyExclusiveInit(plan); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(caseRoot, "case-local.txt"), []byte("preserve me\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	return caseRoot
}

func buildSessionhostUnifiedRuntimeFixture(t *testing.T, repoRoot string) string {
	t.Helper()
	name := "steamai-sessionhost-test"
	if runtime.GOOS == "windows" {
		name += ".exe"
	}
	path := filepath.Join(t.TempDir(), name)
	command := exec.Command("go", "build", "-o", path, "./cmd/rekit")
	command.Dir = repoRoot
	if output, err := command.CombinedOutput(); err != nil {
		t.Fatalf("build sessionhost unified runtime fixture: %v\n%s", err, output)
	}
	return path
}

func sessionhostTestRepoRoot(t *testing.T) string {
	t.Helper()
	ctx, err := rekitruntime.New("", "_template")
	if err != nil {
		t.Fatal(err)
	}
	return ctx.RepoRoot
}

func copySessionhostCurrentSyncSourceFixture(t *testing.T, source string) string {
	t.Helper()
	target := filepath.Join(t.TempDir(), "maintenance-source")
	for _, rel := range []string{
		"packs/_template",
		"common",
		".claude/skills/steamai/SKILL.md",
		"rekit/templates/steamai-project/SKILL.md",
		"rekit/schemas/instance.schema.yml",
		"rekit/schemas/pack-manifest.schema.yml",
		"rekit/tests/catalog.json",
	} {
		sourcePath := filepath.Join(source, filepath.FromSlash(rel))
		info, err := os.Stat(sourcePath)
		if err != nil {
			t.Fatal(err)
		}
		if info.IsDir() {
			err = filepath.WalkDir(sourcePath, func(path string, entry os.DirEntry, walkErr error) error {
				if walkErr != nil {
					return walkErr
				}
				nested, err := filepath.Rel(source, path)
				if err != nil {
					return err
				}
				destination := filepath.Join(target, nested)
				if entry.IsDir() {
					return os.MkdirAll(destination, 0o755)
				}
				data, err := os.ReadFile(path)
				if err != nil {
					return err
				}
				return os.WriteFile(destination, data, 0o644)
			})
		} else {
			data, readErr := os.ReadFile(sourcePath)
			if readErr != nil {
				t.Fatal(readErr)
			}
			destination := filepath.Join(target, filepath.FromSlash(rel))
			if err := os.MkdirAll(filepath.Dir(destination), 0o755); err != nil {
				t.Fatal(err)
			}
			err = os.WriteFile(destination, data, 0o644)
		}
		if err != nil {
			t.Fatal(err)
		}
	}
	return target
}

func sessionhostAttachedRepoRoot(t *testing.T, caseRoot, pack string) string {
	t.Helper()
	ctx, err := rekitruntime.New(caseRoot, pack)
	if err != nil {
		t.Fatal(err)
	}
	return ctx.RepoRoot
}

func sessionhostStateRel(t *testing.T, caseRoot string, parts ...string) string {
	t.Helper()
	path, err := projectstate.Rel(caseRoot, parts...)
	if err != nil {
		t.Fatal(err)
	}
	return path
}

func sessionhostStatePath(t *testing.T, caseRoot string, parts ...string) string {
	t.Helper()
	path, err := projectstate.Join(caseRoot, parts...)
	if err != nil {
		t.Fatal(err)
	}
	return path
}

func snapshotDailyCaseFiles(t *testing.T, caseRoot string) map[string]string {
	t.Helper()
	files := map[string]string{}
	err := filepath.WalkDir(caseRoot, func(path string, entry os.DirEntry, walkErr error) error {
		if walkErr != nil {
			return walkErr
		}
		if entry.IsDir() || !entry.Type().IsRegular() {
			return nil
		}
		rel, err := filepath.Rel(caseRoot, path)
		if err != nil {
			return err
		}
		data, err := os.ReadFile(path)
		if err != nil {
			return err
		}
		files[filepath.ToSlash(rel)] = string(data)
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
	return files
}

func assertDailyCaseFilesEqual(t *testing.T, want, got map[string]string) {
	t.Helper()
	if len(want) != len(got) {
		t.Fatalf("daily case file count changed: before=%d after=%d", len(want), len(got))
	}
	for path, before := range want {
		if after, ok := got[path]; !ok {
			t.Fatalf("daily case file disappeared: %s", path)
		} else if after != before {
			t.Fatalf("daily case file changed: %s", path)
		}
	}
}

func missingClaudePath(t *testing.T) string {
	t.Helper()
	return filepath.Join(t.TempDir(), "missing-claude.exe")
}

func containsDailyStep(steps []string, want string) bool {
	return slices.Contains(steps, want)
}
