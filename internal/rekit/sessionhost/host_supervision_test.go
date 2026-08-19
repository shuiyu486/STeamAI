package sessionhost

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/shuiyu486/re-context-kits/internal/rekit/externalsession"
	"github.com/shuiyu486/re-context-kits/internal/rekit/memberexecution"
	"github.com/shuiyu486/re-context-kits/internal/rekit/mission"
	"github.com/shuiyu486/re-context-kits/internal/rekit/projectstate"
)

func TestAttemptArgsKeepLocalClaudeCodeProviderAsDefault(t *testing.T) {
	if defaultHarness != "claude-code-cli" {
		t.Fatalf("default external session harness constant = %q", defaultHarness)
	}
	args := attemptArgs("actor", "session", "")
	for index := 0; index+1 < len(args); index++ {
		if args[index] == "-ExternalSessionHarness" {
			if args[index+1] != defaultHarness {
				t.Fatalf("attempt external session harness = %q", args[index+1])
			}
			return
		}
	}
	t.Fatalf("attempt args omitted external session harness: %v", args)
}

func TestRequireSameRunningHandoffAcceptsExactCurrentPackage(t *testing.T) {
	plan := supervisedRunningPlanForTest()
	if err := requireSameRunningHandoff(plan, plan); err != nil {
		t.Fatal(err)
	}
}

func TestRequireSameRunningHandoffRejectsLateGenerationAndBindingDrift(t *testing.T) {
	original := supervisedRunningPlanForTest()
	tests := map[string]func(*currentStepPlan){
		"generation": func(plan *currentStepPlan) { plan.ExternalSessionStep.HarnessPackage.Launch.Attempt.Generation++ },
		"attempt": func(plan *currentStepPlan) {
			plan.ExternalSessionStep.HarnessPackage.Launch.Attempt.AttemptSHA256 = "late-attempt"
		},
		"session": func(plan *currentStepPlan) {
			plan.ExternalSessionStep.HarnessPackage.Launch.Attempt.Session = "replacement-session"
		},
		"job": func(plan *currentStepPlan) { plan.ExternalSessionStep.HarnessPackage.JobSHA256 = "replacement-job" },
		"checkpoint": func(plan *currentStepPlan) {
			plan.ExternalSessionStep.HarnessPackage.CheckpointSHA256 = "replacement-checkpoint"
		},
	}
	for name, mutate := range tests {
		t.Run(name, func(t *testing.T) {
			fresh := supervisedRunningPlanForTest()
			mutate(&fresh)
			if err := requireSameRunningHandoff(original, fresh); err == nil {
				t.Fatal("late or drifted supervised result should be fenced")
			}
		})
	}
}

func TestRequireSameRunningHandoffRejectsCompletedOrSubmissionTurn(t *testing.T) {
	original := supervisedRunningPlanForTest()
	for _, fresh := range []currentStepPlan{
		{},
		{ExternalSessionStep: &externalSessionStep{Mode: "result-turn", HarnessPackage: original.ExternalSessionStep.HarnessPackage}},
	} {
		if err := requireSameRunningHandoff(original, fresh); err == nil {
			t.Fatal("non-running current step should reject late supervised publication")
		}
	}
}

func TestClaudeRunReplacementDecisionSurvivesHostRestart(t *testing.T) {
	failed := claudeRun{recovered: true, failureDetail: "invalid recovered result"}
	outcome, replace := claudeRunReplacementOutcome(failed, 1, 0, 3)
	if !replace || outcome != "invalid-result-replacement" {
		t.Fatalf("first recovered failure outcome=%q replace=%t", outcome, replace)
	}
	first := sessionResult(failed, 1, 0, "session-id", "member", outcome, 3)
	if first.Failure == nil || !first.Failure.Replaceable || first.Failure.AttemptsUsed != 1 || first.RunLaunchOrdinal != 0 {
		t.Fatalf("first recovered failure diagnosis=%+v session=%+v", first.Failure, first)
	}

	outcome, replace = claudeRunReplacementOutcome(failed, 3, 0, 3)
	if replace || outcome != "" {
		t.Fatalf("exhausted recovered failure outcome=%q replace=%t", outcome, replace)
	}
	terminal := sessionResult(failed, 3, 0, "session-id", "member", "failed-recovered", 3)
	if terminal.Failure == nil || !terminal.Failure.Terminal || terminal.Failure.AttemptsUsed != 3 || terminal.Failure.Replaceable {
		t.Fatalf("exhausted recovered failure diagnosis=%+v", terminal.Failure)
	}
}

func TestRunFencedFirstGenerationCreatesExactReplacement(t *testing.T) {
	opt, running := runningSessionhostAttemptFixture(t, 1)
	pkg := *running.ExternalSessionStep.HarnessPackage
	bindTrustedSupervisionOptionsForTest(t, &opt, 3)
	paths := writeSessionhostSupervisionFenceForTest(t, opt, pkg)

	stopAfterReplacement := errors.New("stop after exact replacement")
	restore := setSupervisionAcceptanceObservers(nil, func(stage string) error {
		if stage == "replacement" {
			return stopAfterReplacement
		}
		return nil
	})
	defer restore()

	result, err := Run(context.Background(), opt)
	if !errors.Is(err, stopAfterReplacement) {
		t.Fatalf("first fenced generation did not stop after replacement: result=%+v err=%v", result, err)
	}
	if result.SessionLaunches != 0 || result.SessionCompletions != 0 ||
		result.Replacements != 1 || len(result.Sessions) != 1 {
		t.Fatalf("first fenced generation crossed bounded counters: %+v", result)
	}
	session := result.Sessions[0]
	if session.Started || !session.Recovered ||
		session.AttemptGeneration != 1 ||
		session.RunLaunchOrdinal != 0 ||
		session.Outcome != "replacement-requested" ||
		session.Failure == nil || session.Failure.Terminal ||
		!session.Failure.Replaceable ||
		session.Failure.Code != "claude-supervision-fenced" ||
		session.Failure.Stage != "supervision-fence" ||
		session.Failure.AttemptsUsed != 1 ||
		session.Failure.AttemptsLimit != 3 {
		t.Fatalf("first fenced generation session=%+v", session)
	}

	attemptTwo := readSessionhostAttemptForTest(t, opt.Target, pkg.JobID, 2)
	if attemptTwo.Generation != 2 ||
		attemptTwo.SupersedesSHA256 != pkg.Launch.Attempt.AttemptSHA256 ||
		attemptTwo.Session == pkg.Launch.Attempt.Session ||
		attemptTwo.Harness != defaultHarness {
		t.Fatalf("first fenced generation replacement=%+v", attemptTwo)
	}
	attemptThreePath := filepath.Join(
		opt.Target,
		projectstate.CurrentDir,
		"external-session-attempts",
		pkg.JobID,
		"000003.json",
	)
	if _, statErr := os.Lstat(attemptThreePath); !errors.Is(statErr, os.ErrNotExist) {
		t.Fatalf("first fenced generation created a third attempt: %v", statErr)
	}
	for _, path := range []string{paths.claimed, paths.started, paths.terminal} {
		if _, statErr := os.Lstat(path); !errors.Is(statErr, os.ErrNotExist) {
			t.Fatalf("first fenced generation advanced supervised process receipt %s: %v", path, statErr)
		}
	}
	if _, statErr := os.Lstat(filepath.Join(opt.Target, filepath.FromSlash(pkg.Return.SubmissionPath))); !errors.Is(statErr, os.ErrNotExist) {
		t.Fatalf("first fenced generation published a terminal submission: %v", statErr)
	}
}

func TestRunRecoveredStructuredFailurePublishesFailedWithoutCompletion(t *testing.T) {
	opt, running := runningSessionhostAttemptFixture(t, 1)
	pkg := *running.ExternalSessionStep.HarnessPackage
	bindTrustedSupervisionOptionsForTest(t, &opt, 3)
	bound, err := ensureClaudeLaunchControlBinding(opt, pkg)
	if err != nil {
		t.Fatal(err)
	}

	run := claudeRun{
		launchControlBinding: cloneClaudeLaunchControlBinding(bound.launchControlBinding),
		envelope: claudeEnvelope{
			Type:      "result",
			Subtype:   "success",
			SessionID: pkg.Launch.Attempt.Session,
		},
		sessionID:        pkg.Launch.Attempt.Session,
		structuredOutput: json.RawMessage(`{"outcome":"failed","summary":"","reason":"bounded member could not complete","outputs":[],"reviewerItemsPath":""}`),
		started:          true,
		exitCode:         0,
	}
	if !run.success() {
		t.Fatal("structured failure fixture should have a successful process envelope")
	}
	if err := persistClaudeRecoveryForCase(opt.Target, opt, pkg, run); err != nil {
		t.Fatal(err)
	}
	recoveryRoot, err := claudeRecoveryRoot(opt.Target)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		if err := os.RemoveAll(recoveryRoot); err != nil {
			t.Errorf("clean structured failure recovery fixture: %v", err)
		}
	})

	result, err := Run(context.Background(), opt)
	if err != nil {
		t.Fatalf("recovered structured failure: result=%+v err=%v", result, err)
	}
	if result.SessionLaunches != 0 || result.SessionCompletions != 0 ||
		result.Replacements != 0 || len(result.Sessions) != 1 {
		t.Fatalf("recovered structured failure crossed completion counters: %+v", result)
	}
	session := result.Sessions[0]
	if session.Started || !session.Recovered ||
		session.AttemptGeneration != 1 ||
		session.RunLaunchOrdinal != 0 ||
		session.Outcome != "failed-recovered" ||
		session.Failure == nil || !session.Failure.Terminal ||
		session.Failure.Replaceable || session.Failure.Recoverable ||
		session.Failure.Code != "claude-reported-failure" ||
		session.Failure.Stage != "structured-result" ||
		session.Failure.AttemptsUsed != 1 ||
		session.Failure.AttemptsLimit != 3 ||
		!strings.Contains(session.Failure.Detail, "bounded member could not complete") {
		t.Fatalf("recovered structured failure session=%+v", session)
	}
	if result.Failure == nil ||
		result.Failure.Code != session.Failure.Code ||
		!result.Failure.Terminal {
		t.Fatalf("recovered structured failure result diagnosis=%+v", result.Failure)
	}

	submissionData, err := os.ReadFile(
		filepath.Join(opt.Target, filepath.FromSlash(pkg.Return.SubmissionPath)),
	)
	if err != nil {
		t.Fatal(err)
	}
	var submission externalsession.Submission
	if err := json.Unmarshal(submissionData, &submission); err != nil {
		t.Fatal(err)
	}
	if submission.Outcome != "failed" ||
		submission.AttemptID != pkg.Launch.Attempt.AttemptID ||
		submission.AttemptSHA256 != pkg.Launch.Attempt.AttemptSHA256 ||
		submission.Session != pkg.Launch.Attempt.Session ||
		submission.Reason != "bounded member could not complete" {
		t.Fatalf("recovered structured failure submission=%+v", submission)
	}
	if _, statErr := os.Lstat(filepath.Join(recoveryRoot, claudeRecoveryPath(pkg, run.launchControlBinding))); !errors.Is(statErr, os.ErrNotExist) {
		t.Fatalf("recovered structured failure left consumed recovery: %v", statErr)
	}
}

func TestRunFencedThirdGenerationPublishesTerminalFailureWithoutFourthAttempt(t *testing.T) {
	opt, running := runningSessionhostAttemptFixture(t, 3)
	pkg := *running.ExternalSessionStep.HarnessPackage
	bindTrustedSupervisionOptionsForTest(t, &opt, 3)

	paths := writeSessionhostSupervisionFenceForTest(t, opt, pkg)

	result, err := Run(context.Background(), opt)
	if err != nil {
		t.Fatalf("fenced terminal run: result=%+v err=%v", result, err)
	}
	if result.SessionLaunches != 0 || result.SessionCompletions != 0 ||
		result.Replacements != 0 || len(result.Sessions) != 1 {
		t.Fatalf("fenced terminal run crossed bounded counters: %+v", result)
	}
	session := result.Sessions[0]
	if session.Started || !session.Recovered ||
		session.AttemptGeneration != 3 ||
		session.RunLaunchOrdinal != 0 ||
		session.Outcome != "failed-recovered" ||
		session.Failure == nil || !session.Failure.Terminal ||
		session.Failure.Replaceable ||
		session.Failure.Code != "claude-supervision-fenced" ||
		session.Failure.Stage != "supervision-fence" ||
		session.Failure.AttemptsUsed != 3 ||
		session.Failure.AttemptsLimit != 3 {
		t.Fatalf("fenced terminal session=%+v", session)
	}
	if result.Failure == nil ||
		result.Failure.Code != session.Failure.Code ||
		!result.Failure.Terminal {
		t.Fatalf("fenced terminal result diagnosis=%+v", result.Failure)
	}

	attemptFour := filepath.Join(
		opt.Target,
		projectstate.CurrentDir,
		"external-session-attempts",
		pkg.JobID,
		"000004.json",
	)
	if _, err := os.Lstat(attemptFour); !errors.Is(err, os.ErrNotExist) {
		t.Fatalf("fenced terminal run created generation four: %v", err)
	}
	for _, path := range []string{paths.claimed, paths.started, paths.terminal} {
		if _, err := os.Lstat(path); !errors.Is(err, os.ErrNotExist) {
			t.Fatalf("fenced terminal run advanced supervised process receipt %s: %v", path, err)
		}
	}

	submissionData, err := os.ReadFile(
		filepath.Join(opt.Target, filepath.FromSlash(pkg.Return.SubmissionPath)),
	)
	if err != nil {
		t.Fatal(err)
	}
	var submission externalsession.Submission
	if err := json.Unmarshal(submissionData, &submission); err != nil {
		t.Fatal(err)
	}
	if submission.Outcome != "failed" ||
		submission.AttemptID != pkg.Launch.Attempt.AttemptID ||
		submission.AttemptSHA256 != pkg.Launch.Attempt.AttemptSHA256 ||
		submission.Session != pkg.Launch.Attempt.Session ||
		!strings.Contains(submission.Reason, "durably fenced") {
		t.Fatalf("fenced terminal submission=%+v", submission)
	}
}

func writeSessionhostSupervisionFenceForTest(
	t *testing.T,
	opt Options,
	pkg mission.CurrentLoopExternalSessionHarnessPackage,
) supervisionPaths {
	t.Helper()
	paths, spec, specData, specSHA, err := prepareSupervision(
		opt,
		pkg,
		pkg.Launch.Attempt.Session,
	)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		if err := os.RemoveAll(paths.root); err != nil {
			t.Errorf("clean supervision fixture: %v", err)
		}
	})
	if err := os.WriteFile(paths.spec, specData, 0o600); err != nil {
		t.Fatal(err)
	}
	fenced := supervisionFenced{
		SchemaVersion: 1,
		Kind:          supervisionFencedKind,
		RunID:         spec.RunID,
		SpecSHA256:    specSHA,
		SessionID:     spec.SessionID,
		Reason:        "owner exited without terminal",
		FencedAt:      "2026-08-13T00:00:00Z",
	}
	if err := writeSupervisionJSON(
		paths.runRoot,
		"fenced.json",
		"test fence",
		fenced,
	); err != nil {
		t.Fatal(err)
	}
	return paths
}

func readSessionhostAttemptForTest(
	t *testing.T,
	caseRoot,
	jobID string,
	generation int,
) externalsession.Attempt {
	t.Helper()
	path := filepath.Join(
		caseRoot,
		projectstate.CurrentDir,
		"external-session-attempts",
		jobID,
		fmt.Sprintf("%06d.json", generation),
	)
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	var envelope struct {
		Attempt externalsession.Attempt `json:"attempt"`
	}
	if err := json.Unmarshal(data, &envelope); err != nil {
		t.Fatal(err)
	}
	return envelope.Attempt
}

func runningSessionhostAttemptFixture(t *testing.T, maxGeneration int) (Options, currentStepPlan) {
	t.Helper()
	caseRoot := filepath.Join(t.TempDir(), "running-session")
	bootstrap := DailyResult{CaseRoot: caseRoot}
	inspection, err := applyDailyOnboarding(
		caseRoot,
		"exercise bounded host supervision recovery",
		"host-supervision-test",
		&bootstrap,
	)
	if err != nil {
		t.Fatal(err)
	}
	bootstrap.Lane = inspection.Identity.InitialLane
	if err := ensureDailyStarted(caseRoot, inspection.Identity.Pack, &bootstrap); err != nil {
		t.Fatal(err)
	}

	opt := Options{
		Target:       caseRoot,
		Pack:         inspection.Identity.Pack,
		SelectedLane: inspection.Identity.InitialLane,
		Actor:        "host-supervision-test",
	}
	if err := applyMemberDispatchLoop(opt); err != nil {
		t.Fatal(err)
	}

	var running currentStepPlan
	for generation := 1; generation <= maxGeneration; generation++ {
		supersedes := ""
		if generation > 1 {
			if running.ExternalSessionStep == nil ||
				running.ExternalSessionStep.HarnessPackage == nil ||
				running.ExternalSessionStep.HarnessPackage.Launch == nil {
				t.Fatalf("generation %d fixture lost prior running handoff", generation)
			}
			supersedes = running.ExternalSessionStep.HarnessPackage.Launch.Attempt.AttemptSHA256
		}
		running = acceptSessionhostAttemptForTest(
			t,
			opt,
			generation,
			"host-session-"+string(rune('0'+generation)),
			supersedes,
		)
	}
	return opt, running
}

func bindTrustedSupervisionOptionsForTest(t *testing.T, opt *Options, maxAttempts int) {
	t.Helper()
	if opt == nil {
		t.Fatal("trusted supervision options target is nil")
	}
	executable, err := os.Executable()
	if err != nil {
		t.Fatal(err)
	}
	executable, err = resolveClaudePath(executable)
	if err != nil {
		t.Fatal(err)
	}
	opt.ClaudePath = executable
	opt.ExpectedClaudeExecutableSHA256 = strings.Repeat("a", 64)
	opt.ExpectedClaudeExecutablePublisher = liveAcceptanceClaudePublisher
	opt.Timeout = time.Minute
	opt.MaxAttempts = maxAttempts
	opt.StopAfterMemberIntake = true
}

func acceptSessionhostAttemptForTest(
	t *testing.T,
	opt Options,
	generation int,
	session,
	supersedes string,
) currentStepPlan {
	t.Helper()
	attemptInputs := attemptArgs(opt.Actor, session, supersedes)
	attempt, err := runCurrentStep(opt, attemptInputs, false)
	if err != nil {
		t.Fatal(err)
	}
	if attempt.ExternalSessionStep == nil ||
		attempt.ExternalSessionStep.Attempt == nil ||
		attempt.ExternalSessionStep.Attempt.Attempt.Generation != generation ||
		attempt.ExpectedCurrentStepPlanSHA256 == "" {
		t.Fatalf("generation %d attempt preview=%+v", generation, attempt)
	}
	if err := applyCurrentStep(opt, attempt, attemptInputs); err != nil {
		t.Fatal(err)
	}

	claimInputs := []string{
		"-ExternalSessionActor", opt.Actor,
		"-ExternalSessionObservedAt", nowRFC3339Nano(),
	}
	claim, err := runCurrentStep(opt, claimInputs, false)
	if err != nil {
		t.Fatal(err)
	}
	if claim.ExternalSessionStep == nil ||
		claim.ExternalSessionStep.Mode != "dispatch-claim" ||
		claim.ExpectedCurrentStepPlanSHA256 == "" {
		t.Fatalf("generation %d claim preview=%+v", generation, claim)
	}
	if err := applyCurrentStep(opt, claim, claimInputs); err != nil {
		t.Fatal(err)
	}

	launchInputs := []string{
		"-ExternalSessionLaunchOutcome", "accepted",
		"-ExternalSessionActor", opt.Actor,
		"-ExternalSessionObservedAt", nowRFC3339Nano(),
		"-ExternalSessionHarness", defaultHarness,
		"-ExternalSessionId", session,
	}
	launch, err := runCurrentStep(opt, launchInputs, false)
	if err != nil {
		t.Fatal(err)
	}
	if launch.ExternalSessionStep == nil ||
		launch.ExternalSessionStep.Mode != "launch-accepted" ||
		launch.ExpectedCurrentStepPlanSHA256 == "" {
		t.Fatalf("generation %d launch preview=%+v", generation, launch)
	}
	if err := applyCurrentStep(opt, launch, launchInputs); err != nil {
		t.Fatal(err)
	}

	running, err := runCurrentStep(opt, nil, false)
	if err != nil {
		t.Fatal(err)
	}
	if running.ExternalSessionStep == nil ||
		running.ExternalSessionStep.Mode != "running-handoff" ||
		running.ExternalSessionStep.HarnessPackage == nil ||
		running.ExternalSessionStep.HarnessPackage.Launch == nil ||
		running.ExternalSessionStep.HarnessPackage.Launch.Attempt.Generation != generation ||
		running.ExternalSessionStep.HarnessPackage.Launch.Attempt.Session != session {
		t.Fatalf("generation %d running handoff=%+v", generation, running)
	}
	return running
}

func TestCurrentStepIsEvidenceReviewStopRequiresExactTypedShape(t *testing.T) {
	plan := currentStepPlan{CurrentDriverRequest: mission.MissionCommanderDriverRequest{
		Source: "executionEvidenceReview",
		State:  "ready-for-evidence-review",
	}}
	if !currentStepIsEvidenceReviewStop(plan) {
		t.Fatal("typed evidence review stop was not recognized")
	}
	plan.CurrentDriverRequest.Source = "executionEvidenceReview.followUp"
	if !currentStepIsEvidenceReviewStop(plan) {
		t.Fatal("typed evidence review follow-up stop was not recognized")
	}
	plan.MemberExecution = &memberexecution.Plan{}
	if currentStepIsEvidenceReviewStop(plan) {
		t.Fatal("evidence review stop accepted a member dispatch plan")
	}
}

func supervisedRunningPlanForTest() currentStepPlan {
	return currentStepPlan{ExternalSessionStep: &externalSessionStep{
		Mode: "running-handoff",
		HarnessPackage: &mission.CurrentLoopExternalSessionHarnessPackage{
			JobSHA256: "job-sha", CheckpointSHA256: "checkpoint-sha", SessionKind: "member",
			Launch: &mission.CurrentLoopExternalSessionHarnessLaunch{Attempt: mission.CurrentLoopExternalSessionAttempt{
				AttemptID: "attempt-id", AttemptSHA256: "attempt-sha", Generation: 1, Session: "session-id",
			}},
		},
	}}
}
