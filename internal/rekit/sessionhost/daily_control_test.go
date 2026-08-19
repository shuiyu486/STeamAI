package sessionhost

import (
	"context"
	"path/filepath"
	"slices"
	"strings"
	"testing"

	"github.com/shuiyu486/re-context-kits/internal/rekit/executioncontrol"
	"github.com/shuiyu486/re-context-kits/internal/rekit/workstream"
)

func TestRunDailyControlPreviewPauseResumeStopBeforeClaude(t *testing.T) {
	caseRoot := filepath.Join(t.TempDir(), "daily-control")
	bootstrap := DailyResult{CaseRoot: caseRoot}
	missionIntent, err := applyDailyOnboarding(caseRoot, "daily control goal", "daily-control-test", &bootstrap)
	if err != nil {
		t.Fatal(err)
	}
	bootstrap.Lane = missionIntent.Identity.InitialLane
	if err := ensureDailyStarted(caseRoot, missionIntent.Identity.Pack, &bootstrap); err != nil {
		t.Fatal(err)
	}

	actor := "daily-control-test"
	claudePath := missingClaudePath(t)
	run := func(action, reason string, applyFrom *executioncontrol.Plan) DailyResult {
		t.Helper()
		opt := DailyOptions{
			Target:       caseRoot,
			SelectedLane: bootstrap.Lane,
			Actor:        actor,
			ClaudePath:   claudePath,
			Control: executioncontrol.Options{
				Action: action,
				Reason: reason,
			},
			ControlWhatIf: applyFrom == nil,
			ControlApply:  applyFrom != nil,
		}
		if applyFrom != nil {
			opt.Control.PublicationStamp = applyFrom.PublicationStamp
			opt.Control.ExpectedPlanSHA256 = applyFrom.ExpectedPlanSHA256
		}
		result, err := RunDaily(context.Background(), opt)
		if err != nil {
			t.Fatalf("daily control %s result=%+v err=%v", action, result, err)
		}
		if result.Mode != "control" || result.Lane != bootstrap.Lane || result.ExecutionControl == nil ||
			result.SessionLaunches != 0 || result.SessionCompletions != 0 || result.Replacements != 0 ||
			len(result.HostRuns) != 0 || len(result.DriverSteps) != 0 {
			t.Fatalf("daily control crossed the zero-Claude boundary: %+v", result)
		}
		return result
	}

	before := snapshotDailyCaseFiles(t, caseRoot)
	pausePreview := run(executioncontrol.ActionPause, "operator requested a bounded pause", nil)
	if pausePreview.FinalState != DailyActionConfirmationRequired || !pausePreview.Blocked ||
		pausePreview.Action == nil || pausePreview.Action.Code != DailyActionConfirmationRequired ||
		pausePreview.ExecutionControl.Applied || !strings.HasPrefix(pausePreview.ExecutionControl.ApplyCommand, "/steamai control ") {
		t.Fatalf("pause preview is incomplete: %+v", pausePreview)
	}
	assertDailyCaseFilesEqual(t, before, snapshotDailyCaseFiles(t, caseRoot))

	paused := run(executioncontrol.ActionPause, "operator requested a bounded pause", pausePreview.ExecutionControl)
	if paused.FinalState != DailyActionPaused || !paused.Blocked || paused.Action == nil || paused.Action.Code != DailyActionPaused || !paused.ExecutionControl.Applied {
		t.Fatalf("pause Apply result = %+v", paused)
	}
	assertDailyControlHead(t, caseRoot, bootstrap.Lane, executioncontrol.StatePaused, 1)

	resumePreview := run(executioncontrol.ActionResume, "operator reviewed a bounded resume", nil)
	beforeResume := snapshotDailyCaseFiles(t, caseRoot)
	resumed := run(executioncontrol.ActionResume, "operator reviewed a bounded resume", resumePreview.ExecutionControl)
	if resumed.FinalState != DailyActionResumed || resumed.Blocked || resumed.Action == nil || resumed.Action.Code != DailyActionResumed ||
		resumed.CurrentDriverRequest != nil || resumed.CurrentDriverRequestSHA256 != "" {
		t.Fatalf("resume Apply implicitly continued work: %+v", resumed)
	}
	if len(snapshotDailyCaseFiles(t, caseRoot)) != len(beforeResume)+2 {
		t.Fatalf("resume Apply wrote more than its exact intent and receipt")
	}
	assertDailyControlHead(t, caseRoot, bootstrap.Lane, executioncontrol.StateRunning, 2)

	stopPreview := run(executioncontrol.ActionStop, "operator requested a durable stop", nil)
	stopped := run(executioncontrol.ActionStop, "operator requested a durable stop", stopPreview.ExecutionControl)
	if stopped.FinalState != DailyActionStopped || !stopped.Blocked || stopped.Action == nil || stopped.Action.Code != DailyActionStopped {
		t.Fatalf("stop Apply result = %+v", stopped)
	}
	assertDailyControlHead(t, caseRoot, bootstrap.Lane, executioncontrol.StateStopped, 3)
}

func TestRunDailyControlMultipleLanesRequiresChoiceWithoutWrite(t *testing.T) {
	caseRoot := filepath.Join(t.TempDir(), "daily-control-multiple")
	bootstrap := DailyResult{CaseRoot: caseRoot}
	missionIntent, err := applyDailyOnboarding(caseRoot, "daily control multi-lane goal", "daily-control-test", &bootstrap)
	if err != nil {
		t.Fatal(err)
	}
	bootstrap.Lane = missionIntent.Identity.InitialLane
	if err := ensureDailyStarted(caseRoot, missionIntent.Identity.Pack, &bootstrap); err != nil {
		t.Fatal(err)
	}
	if _, err := workstream.StartApply(sessionhostAttachedRepoRoot(t, caseRoot, missionIntent.Identity.Pack), caseRoot, missionIntent.Identity.Pack, workstream.StartOptions{
		Name: "login", Actor: "daily-control-test", Executor: "session-login", TakeoverReason: "daily control lane choice regression",
	}); err != nil {
		t.Fatal(err)
	}

	before := snapshotDailyCaseFiles(t, caseRoot)
	result, err := RunDaily(context.Background(), DailyOptions{
		Target: caseRoot,
		Actor:  "daily-control-test",
		Control: executioncontrol.Options{
			Action: executioncontrol.ActionPause,
			Reason: "select the exact lane before pausing",
		},
		ControlWhatIf: true,
		ClaudePath:    missingClaudePath(t),
	})
	if err != nil {
		t.Fatalf("multi-lane daily control should stop at typed choice: result=%+v err=%v", result, err)
	}
	if result.Mode != "control" || !result.Blocked || result.ExecutionControl != nil || result.Action == nil ||
		!result.Action.RequiresInput || result.Action.Code != DailyActionBlocked || len(result.Action.Choices) < 2 ||
		result.SessionLaunches != 0 || len(result.HostRuns) != 0 || len(result.DriverSteps) != 0 {
		t.Fatalf("multi-lane daily control crossed its choice boundary: %+v", result)
	}
	ids := make([]string, 0, len(result.Action.Choices))
	for _, choice := range result.Action.Choices {
		ids = append(ids, choice.ID)
	}
	if !slices.Contains(ids, bootstrap.Lane) || !slices.Contains(ids, "binary-analysis-login") {
		t.Fatalf("multi-lane daily control choices = %+v", result.Action.Choices)
	}
	assertDailyCaseFilesEqual(t, before, snapshotDailyCaseFiles(t, caseRoot))
}

func TestRunDailyControlRejectsConflictingIntentWithoutWrite(t *testing.T) {
	caseRoot := filepath.Join(t.TempDir(), "daily-control-conflict")
	result, err := RunDaily(context.Background(), DailyOptions{
		Target: caseRoot,
		Goal:   "must not be combined",
		Control: executioncontrol.Options{
			Action: executioncontrol.ActionPause,
			Reason: "conflicting request",
		},
		ControlWhatIf: true,
	})
	if err == nil || !strings.Contains(err.Error(), "cannot be combined with -goal or -correction") {
		t.Fatalf("conflicting daily control result=%+v err=%v", result, err)
	}
	if result.Mode != "control" || result.SessionLaunches != 0 {
		t.Fatalf("conflicting daily control crossed zero-launch boundary: %+v", result)
	}
}

func assertDailyControlHead(t *testing.T, caseRoot, lane, state string, generation int) {
	t.Helper()
	inspection, err := executioncontrol.Inspect(caseRoot, lane)
	if err != nil {
		t.Fatal(err)
	}
	if inspection.Pending || inspection.State != state || inspection.CurrentGeneration != generation || len(inspection.CurrentReceiptSHA256) != 64 {
		t.Fatalf("daily control head = %+v", inspection)
	}
}
