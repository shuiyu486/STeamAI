package sessionhost

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/shuiyu486/re-context-kits/internal/rekit/defaults"
	"github.com/shuiyu486/re-context-kits/internal/rekit/instance"
	"github.com/shuiyu486/re-context-kits/internal/rekit/lanecompletion"
	"github.com/shuiyu486/re-context-kits/internal/rekit/laneid"
	"github.com/shuiyu486/re-context-kits/internal/rekit/manifest"
	"github.com/shuiyu486/re-context-kits/internal/rekit/memberexecution"
	"github.com/shuiyu486/re-context-kits/internal/rekit/mission"
	"github.com/shuiyu486/re-context-kits/internal/rekit/missionintent"
	"github.com/shuiyu486/re-context-kits/internal/rekit/note"
	"github.com/shuiyu486/re-context-kits/internal/rekit/onboarding"
	rekitruntime "github.com/shuiyu486/re-context-kits/internal/rekit/runtime"
	"github.com/shuiyu486/re-context-kits/internal/rekit/workstream"
)

const defaultDailyActor = "rekit-daily-front-door"

type DailyOptions struct {
	Target                            string
	Goal                              string
	Correction                        string
	Actor                             string
	ClaudePath                        string
	ExpectedClaudeExecutableSHA256    string
	ExpectedClaudeExecutablePublisher string
	Model                             string
	Timeout                           time.Duration
	MaxAttempts                       int
	onCaseReady                       func(string) error
	beforeMemberRun                   func(caseRoot, pack, lane string) error
}

type DailyResult struct {
	SchemaVersion      int                        `json:"schemaVersion"`
	Command            string                     `json:"command"`
	Mode               string                     `json:"mode"`
	CaseRoot           string                     `json:"caseRoot"`
	Pack               string                     `json:"pack,omitempty"`
	Lane               string                     `json:"lane,omitempty"`
	FinalState         string                     `json:"finalState"`
	Replay             bool                       `json:"replay"`
	Blocked            bool                       `json:"blocked,omitempty"`
	OnboardingApplied  bool                       `json:"onboardingApplied,omitempty"`
	OnboardingReplay   bool                       `json:"onboardingReplay,omitempty"`
	CorrectionEventID  string                     `json:"correctionEventId,omitempty"`
	ExecutorGeneration int                        `json:"executorGeneration,omitempty"`
	DriverSteps        []string                   `json:"driverSteps,omitempty"`
	HostRuns           []Result                   `json:"hostRuns,omitempty"`
	Completion         *workstream.CompleteResult `json:"completion,omitempty"`
	SessionLaunches    int                        `json:"sessionLaunches"`
	SessionCompletions int                        `json:"sessionCompletions"`
	Replacements       int                        `json:"replacements"`
	Failure            *FailureDiagnosis          `json:"failure,omitempty"`
	Boundary           []string                   `json:"boundary"`
}

func RunDaily(parent context.Context, opt DailyOptions) (result DailyResult, retErr error) {
	defer func() {
		if retErr != nil && result.Failure == nil {
			result.Failure = diagnosisForError(retErr, result.SessionLaunches, opt.MaxAttempts, len(result.DriverSteps))
		}
	}()
	goal := strings.TrimSpace(opt.Goal)
	correction := strings.TrimSpace(opt.Correction)
	mode := "resume"
	if goal != "" {
		mode = "goal"
	}
	if correction != "" {
		mode = "correction"
	}
	result = DailyResult{
		SchemaVersion: 1,
		Command:       "rekit-host-daily",
		Mode:          mode,
		FinalState:    "not-started",
		Boundary: []string{
			"the daily front door derives lifecycle identity and consumes only public exact preview/apply requests",
			"all member output and ReviewerResult bytes come from spawned Claude Code processes, never from the front door",
			"the front door does not write authority/confirmed state or execute an unauthorized heavy action",
		},
	}
	if goal != "" && correction != "" {
		return result, fmt.Errorf("daily front door accepts either -goal or -correction, not both")
	}
	caseRoot, exists, err := canonicalDailyTarget(opt.Target)
	if err != nil {
		return result, err
	}
	result.CaseRoot = caseRoot
	if exists && opt.onCaseReady != nil {
		if err := opt.onCaseReady(caseRoot); err != nil {
			return result, fmt.Errorf("bind existing daily case root identity: %w", err)
		}
	}
	opt.Actor = strings.TrimSpace(opt.Actor)
	if opt.Actor == "" {
		opt.Actor = defaultDailyActor
	}
	if strings.ContainsAny(opt.Actor, "\r\n") {
		return result, fmt.Errorf("daily front door actor must be a single line")
	}
	if strings.TrimSpace(opt.ClaudePath) != "" && (strings.TrimSpace(opt.ExpectedClaudeExecutableSHA256) == "" || strings.TrimSpace(opt.ExpectedClaudeExecutablePublisher) == "") {
		return result, fmt.Errorf("daily front door refuses a custom or unbound Claude executable; omit -claude so the canonical signed installation is discovered")
	}
	inspection, err := missionintent.Inspect(caseRoot)
	if err != nil {
		return result, fmt.Errorf("inspect daily mission intent: %w", err)
	}
	if inspection.State == "absent" {
		if goal == "" {
			return result, fmt.Errorf("daily target without committed onboarding requires -goal <natural-language goal>")
		}
		inspection, err = applyDailyOnboarding(caseRoot, goal, opt.Actor, &result)
		if opt.onCaseReady != nil {
			if bindErr := opt.onCaseReady(caseRoot); bindErr != nil {
				return result, errors.Join(err, fmt.Errorf("bind fresh daily case root identity: %w", bindErr))
			}
		}
		if err != nil {
			return result, err
		}
		exists = true
	} else if inspection.State == "pending" {
		if correction != "" {
			return result, fmt.Errorf("daily correction cannot run while onboarding publication is pending")
		}
		if goal != "" && goal != strings.TrimSpace(inspection.Identity.Goal) {
			return result, fmt.Errorf("daily goal differs from the immutable pending mission intent")
		}
		if err := recoverDailyOnboarding(inspection, &result); err != nil {
			return result, err
		}
		inspection, err = missionintent.Inspect(caseRoot)
		if err != nil || !inspection.Committed {
			return result, fmt.Errorf("daily onboarding recovery did not commit: state=%s err=%v", inspection.State, err)
		}
		exists = true
	} else if inspection.Committed {
		if goal != "" && goal != strings.TrimSpace(inspection.Identity.Goal) {
			return result, fmt.Errorf("daily goal differs from the immutable committed mission intent")
		}
		if goal != "" {
			result.OnboardingReplay = true
		}
	} else if inspection.State != "absent" {
		return result, fmt.Errorf("daily mission intent is not usable: state=%s", inspection.State)
	}
	if !exists {
		return result, fmt.Errorf("daily target is unavailable after onboarding")
	}

	pack, err := dailyPack(caseRoot, inspection)
	if err != nil {
		return result, err
	}
	result.Pack = pack
	if inspection.Committed {
		result.Lane = inspection.Identity.InitialLane
	} else if goal != "" {
		return result, fmt.Errorf("daily goal requires a committed mission intent")
	}

	hostOpt := Options{
		Target:                            caseRoot,
		Pack:                              pack,
		Actor:                             opt.Actor,
		ClaudePath:                        opt.ClaudePath,
		ExpectedClaudeExecutableSHA256:    opt.ExpectedClaudeExecutableSHA256,
		ExpectedClaudeExecutablePublisher: opt.ExpectedClaudeExecutablePublisher,
		Model:                             opt.Model,
		Timeout:                           opt.Timeout,
		MaxAttempts:                       opt.MaxAttempts,
	}

	if correction != "" {
		return runDailyCorrection(parent, hostOpt, inspection, correction, result)
	}
	if goal == "" {
		rejected, err := currentReviewerRejectionAwaitingCorrection(caseRoot, pack)
		if err != nil {
			return result, err
		}
		if rejected {
			result.FinalState = "reviewer-rejected-awaiting-correction"
			result.Blocked = true
			result.Replay = true
			return result, nil
		}
	}
	if err := ensureDailyStarted(caseRoot, pack, &result); err != nil {
		return result, err
	}
	if state, generation, terminal, err := dailyLaneTerminal(caseRoot, result.Lane); err != nil {
		return result, err
	} else if terminal {
		result.FinalState = state
		result.ExecutorGeneration = generation
		result.Replay = true
		return result, nil
	}
	if goal != "" && result.OnboardingReplay {
		generation, ready, err := dailyCurrentMemberReady(caseRoot, result.Lane, goal)
		if err != nil {
			return result, err
		}
		if ready {
			result.FinalState = "member-intake-ready"
			result.ExecutorGeneration = generation
			result.Replay = true
			return result, nil
		}
	}
	if opt.beforeMemberRun != nil {
		if err := opt.beforeMemberRun(caseRoot, pack, result.Lane); err != nil {
			return result, fmt.Errorf("prepare daily member run: %w", err)
		}
	}
	if err := bindDailyTrustedClaude(&opt); err != nil {
		return result, err
	}
	hostOpt.ClaudePath = opt.ClaudePath
	hostOpt.ExpectedClaudeExecutableSHA256 = opt.ExpectedClaudeExecutableSHA256
	hostOpt.ExpectedClaudeExecutablePublisher = opt.ExpectedClaudeExecutablePublisher
	hostOpt.StopAfterMemberIntake = goal != ""
	hostResult, err := Run(parent, hostOpt)
	addDailyHostRun(&result, hostResult)
	if err != nil {
		return result, err
	}
	result.FinalState = hostResult.FinalMode
	if goal != "" {
		if result.Lane != "" {
			if latest, ok, inspectErr := memberexecution.Latest(caseRoot, result.Lane); inspectErr == nil && ok {
				result.ExecutorGeneration = latest.Owner.ExecutorGeneration
			}
		}
		result.Replay = result.OnboardingReplay && hostResult.SessionLaunches == 0
		return result, nil
	}
	if err := finishDailyCompletion(caseRoot, pack, &result); err != nil {
		return result, err
	}
	return result, nil
}

func bindDailyTrustedClaude(opt *DailyOptions) error {
	if opt == nil {
		return fmt.Errorf("daily front door options are missing")
	}
	requestedPath := strings.TrimSpace(opt.ClaudePath)
	expectedHash := strings.ToLower(strings.TrimSpace(opt.ExpectedClaudeExecutableSHA256))
	expectedPublisher := strings.TrimSpace(opt.ExpectedClaudeExecutablePublisher)
	if requestedPath == "" && expectedHash == "" && expectedPublisher == "" {
		identity, err := discoverTrustedClaudeExecutable()
		if err != nil {
			return fmt.Errorf("discover trusted Claude Code executable for daily front door: %w", err)
		}
		opt.ClaudePath = identity.Path
		opt.ExpectedClaudeExecutableSHA256 = identity.SHA256
		opt.ExpectedClaudeExecutablePublisher = identity.Publisher
		return nil
	}
	if requestedPath == "" || len(expectedHash) != 64 || !strings.EqualFold(expectedPublisher, liveAcceptanceClaudePublisher) {
		return fmt.Errorf("daily front door refuses a custom or unbound Claude executable; omit -claude so the canonical signed installation is discovered")
	}
	identity, err := inspectTrustedClaudeExecutable(requestedPath, false)
	if err != nil {
		return fmt.Errorf("validate trusted Claude Code executable for daily front door: %w", err)
	}
	if !strings.EqualFold(identity.SHA256, expectedHash) || !strings.EqualFold(identity.Publisher, expectedPublisher) {
		return fmt.Errorf("daily trusted Claude executable identity differs from the expected SHA-256 or publisher")
	}
	opt.ClaudePath = identity.Path
	opt.ExpectedClaudeExecutableSHA256 = identity.SHA256
	opt.ExpectedClaudeExecutablePublisher = identity.Publisher
	return nil
}

func canonicalDailyTarget(value string) (string, bool, error) {
	value = strings.TrimSpace(value)
	if value == "" {
		return "", false, fmt.Errorf("daily front door requires -target <case root>")
	}
	full, err := filepath.Abs(value)
	if err != nil {
		return "", false, err
	}
	full = filepath.Clean(full)
	info, err := os.Lstat(full)
	if os.IsNotExist(err) {
		return full, false, nil
	}
	if err != nil {
		return "", false, err
	}
	if !info.IsDir() || info.Mode()&os.ModeSymlink != 0 {
		return "", false, fmt.Errorf("daily target must be a regular directory or a missing fresh path: %s", full)
	}
	return full, true, nil
}

func applyDailyOnboarding(caseRoot, goal, actor string, result *DailyResult) (missionintent.Inspection, error) {
	pack := defaults.DefaultPack
	projectName := strings.TrimSpace(filepath.Base(caseRoot))
	if projectName == "" || projectName == "." || projectName == string(filepath.Separator) {
		projectName = "rekit-daily-case"
	}
	if inst, err := instance.Read(caseRoot); err != nil {
		return missionintent.Inspection{}, err
	} else if inst.Source != "missing" {
		pack = inst.TemplatePack
		projectName = inst.ProjectName
	}
	lane, err := dailyInitialLane(caseRoot, pack)
	if err != nil {
		return missionintent.Inspection{}, err
	}
	executor := dailyExecutorID(caseRoot, 1)
	args := []string{
		"-Command", "onboard", "-Target", caseRoot, "-Pack", pack,
		"-ProjectName", projectName, "-Goal", goal, "-Actor", actor,
		"-Executor", executor, "-InitialLane", lane,
		"-WhatIf", "-Format", "json",
	}
	var plan onboarding.Plan
	if err := runPublicCLI(args, &plan); err != nil {
		return missionintent.Inspection{}, fmt.Errorf("daily public onboard preview: %w", err)
	}
	if plan.IsMutation || plan.Replay || len(plan.ApplyArgs) == 0 {
		return missionintent.Inspection{}, fmt.Errorf("daily public onboard preview omitted a fresh zero-write exact Apply request")
	}
	var applied onboarding.Result
	if err := runPublicCLI(plan.ApplyArgs, &applied); err != nil {
		return missionintent.Inspection{}, fmt.Errorf("daily public onboard Apply: %w", err)
	}
	if !applied.Applied || applied.Replay || !applied.Inspection.Committed {
		return missionintent.Inspection{}, fmt.Errorf("daily public onboard Apply did not commit the fresh mission intent")
	}
	result.OnboardingApplied = true
	return applied.Inspection, nil
}

func recoverDailyOnboarding(inspection missionintent.Inspection, result *DailyResult) error {
	identity := inspection.Identity
	args := []string{
		"-Command", "onboard", "-Target", identity.Target, "-Pack", identity.Pack,
		"-ProjectName", identity.ProjectName, "-Goal", identity.Goal, "-Actor", identity.Actor,
		"-Executor", identity.Executor, "-InitialLane", identity.InitialLane,
		"-OnboardingPublicationStamp", inspection.PublicationStamp,
		"-ExpectedOnboardingPlanSha256", inspection.OnboardingPlanSHA256,
		"-Apply", "-Format", "json",
	}
	var applied onboarding.Result
	if err := runPublicCLI(args, &applied); err != nil {
		return fmt.Errorf("recover daily public onboarding: %w", err)
	}
	if !applied.Applied || applied.Replay || !applied.Inspection.Committed {
		return fmt.Errorf("daily public onboarding recovery did not publish the pending exact generation")
	}
	result.OnboardingApplied = true
	return nil
}

func dailyInitialLane(caseRoot, pack string) (string, error) {
	ctx, err := runtimeContextForDailyPack(caseRoot, pack)
	if err != nil {
		return "", err
	}
	m, err := manifest.Load(ctx, pack)
	if err != nil {
		return "", err
	}
	if err := m.ValidateSchema(); err != nil {
		return "", err
	}
	laneType := strings.TrimSpace(m.WorkstreamDefaults["defaultStartLaneType"])
	lane := laneid.Resolve(laneType, "mission")
	if lane == "" {
		return "", fmt.Errorf("daily front door could not derive an initial lane for pack %s", pack)
	}
	return lane, nil
}

func runtimeContextForDailyPack(caseRoot, pack string) (string, error) {
	ctx, err := rekitruntime.New(caseRoot, pack)
	if err != nil {
		return "", err
	}
	return ctx.RepoRoot, nil
}

func dailyPack(caseRoot string, inspection missionintent.Inspection) (string, error) {
	if inspection.Committed {
		return inspection.Identity.Pack, nil
	}
	inst, err := instance.Read(caseRoot)
	if err != nil {
		return "", err
	}
	if inst.Source == "missing" {
		return "", fmt.Errorf("existing daily target is not an attached rekit case")
	}
	if inst.Moved() {
		return "", instance.MovedRepairPreviewError(caseRoot, inst.TemplatePack)
	}
	return inst.TemplatePack, nil
}

func ensureDailyStarted(caseRoot, pack string, result *DailyResult) error {
	for range 4 {
		status, err := runPublicStatus(caseRoot, pack)
		if err != nil {
			return err
		}
		request := currentDailyRequest(status)
		if request == nil {
			return nil
		}
		command, err := publicCommandName(request.Command)
		if err != nil {
			return err
		}
		switch command {
		case "overview":
			if !request.CommandExecutable || request.Blocked {
				return fmt.Errorf("daily overview request is not executable")
			}
			if err := runPublicCommand(request.Command); err != nil {
				return fmt.Errorf("consume daily public overview request: %w", err)
			}
			result.DriverSteps = append(result.DriverSteps, "overview")
		case "start":
			step, err := runPublicDriverStep(caseRoot, pack)
			if err != nil {
				return fmt.Errorf("daily public start route: %w", err)
			}
			if step.ResultCommand != "start" {
				return fmt.Errorf("daily public start route returned %q", step.ResultCommand)
			}
			result.DriverSteps = append(result.DriverSteps, step.ResultCommand)
		default:
			return nil
		}
	}
	return fmt.Errorf("daily front door exceeded bootstrap transition limit")
}

func currentDailyRequest(status publicStatus) *struct {
	Kind              string `json:"kind,omitempty"`
	RunLoopStepID     string `json:"runLoopStepId,omitempty"`
	Command           string `json:"command"`
	CommandExecutable bool   `json:"commandExecutable"`
	Blocked           bool   `json:"blocked,omitempty"`
} {
	if status.MissionControlRunbook == nil {
		return nil
	}
	return status.MissionControlRunbook.CurrentDriverRequest
}

func runDailyCorrection(parent context.Context, hostOpt Options, inspection missionintent.Inspection, correction string, result DailyResult) (DailyResult, error) {
	lane, boardLane, existing, err := dailyCorrectionLane(result.CaseRoot, inspection, correction, hostOpt.Actor)
	if err != nil {
		return result, err
	}
	result.Lane = lane
	result.CorrectionEventID = dailyCorrectionEventID(dailyCorrectionScope(result.CaseRoot, inspection), lane, correction)
	terminalState := strings.ToLower(strings.TrimSpace(boardLane.Status))
	if terminalState == "archived" {
		return result, fmt.Errorf("daily correction refuses archived lane %s because no durable archive transition is supported", lane)
	}
	if terminalState == "closed" {
		if existing == nil {
			return result, fmt.Errorf("daily correction refuses closed lane %s", lane)
		}
		result.CorrectionEventID = mission.Value(existing, "eventId")
		if err := validateExistingDailyCorrectionRejection(result.CaseRoot, inspection, lane, correction, existing); err != nil {
			return result, err
		}
		resolution, resolved, err := inspectDailyCorrectionResolution(result.CaseRoot, lane, result.CorrectionEventID)
		if err != nil {
			return result, err
		}
		if !resolved || resolution.ExecutorGeneration != boardLane.ExecutorGeneration {
			return result, fmt.Errorf("daily terminal correction replay requires the durable current reconcile resolution")
		}
		state, generation, terminal, err := dailyLaneTerminal(result.CaseRoot, lane)
		if err != nil {
			return result, err
		}
		if !terminal || generation != resolution.ExecutorGeneration {
			return result, fmt.Errorf("daily terminal correction replay requires the committed current lane completion")
		}
		result.FinalState = state
		result.ExecutorGeneration = generation
		result.Replay = true
		return result, nil
	}

	if existing != nil && (mission.Value(existing, "reviewerVerificationEventId") == "" || mission.Value(existing, "reviewerDecisionEventId") == "") {
		existing = nil
	}
	var rejection workstream.MemberReviewerRejection
	targetRef := ""
	if existing != nil {
		result.CorrectionEventID = mission.Value(existing, "eventId")
		if err := validateExistingDailyCorrectionRejection(result.CaseRoot, inspection, lane, correction, existing); err != nil {
			return result, err
		}
		targetRef = mission.Value(existing, "target")
		var rejected bool
		rejection, rejected, err = workstream.CurrentMemberManifestReviewerRejection(result.CaseRoot, lane, targetRef)
		if err != nil {
			return result, err
		}
		if !rejected || !dailyCorrectionBindsRejection(existing, rejection) {
			return result, fmt.Errorf("existing daily correction does not match canonical reviewer rejection")
		}
	} else {
		rejection, targetRef, err = currentDailyCorrectionMemberTarget(result.CaseRoot, boardLane)
		if err != nil {
			return result, err
		}
		result.CorrectionEventID = dailyCorrectionEventID(dailyCorrectionScope(result.CaseRoot, inspection), lane, correction, rejection)
		existing, err = existingDailyCorrectionByID(result.CaseRoot, result.CorrectionEventID, lane, correction, hostOpt.Actor)
		if err != nil {
			return result, err
		}
	}
	if existing != nil {
		if err := verifyDailyCorrection(existing, lane, correction, hostOpt.Actor, result.CorrectionEventID, targetRef, rejection); err != nil {
			return result, err
		}
	}
	resolution, resolved, err := inspectDailyCorrectionResolution(result.CaseRoot, lane, result.CorrectionEventID)
	if err != nil {
		return result, err
	}
	if !resolved {
		if existing == nil {
			createdAt := nowRFC3339Nano()
			args := dailyCorrectionArgs(result.CaseRoot, result.Pack, lane, correction, hostOpt.Actor, result.CorrectionEventID, createdAt, targetRef, rejection)
			var preview note.AppendResult
			if err := runPublicCLI(args, &preview); err != nil {
				return result, fmt.Errorf("daily public correction preview: %w", err)
			}
			if preview.IsMutation || preview.Applied || preview.Reason != "what-if" || len(preview.RecordArgs) == 0 {
				return result, fmt.Errorf("daily public correction preview omitted a zero-write exact record request")
			}
			var applied note.AppendResult
			if err := runPublicCLI(preview.RecordArgs, &applied); err != nil {
				return result, fmt.Errorf("record daily public correction: %w", err)
			}
			if !applied.Applied {
				return result, fmt.Errorf("daily public correction was not applied: %s", applied.Reason)
			}
			result.DriverSteps = append(result.DriverSteps, "note-intervention")
			existing = applied.Event
		}
		if err := verifyDailyCorrection(existing, lane, correction, hostOpt.Actor, result.CorrectionEventID, targetRef, rejection); err != nil {
			return result, err
		}
		step, err := runPublicDriverStep(result.CaseRoot, result.Pack)
		if err != nil {
			return result, fmt.Errorf("daily public correction reconcile: %w", err)
		}
		if step.ResultCommand != "reconcile" {
			return result, fmt.Errorf("daily correction expected reconcile, got %q", step.ResultCommand)
		}
		result.DriverSteps = append(result.DriverSteps, step.ResultCommand)
		resolution, resolved, err = inspectDailyCorrectionResolution(result.CaseRoot, lane, result.CorrectionEventID)
		if err != nil {
			return result, err
		}
		if !resolved || step.Actor != resolution.Actor || step.Executor != resolution.Executor || step.ExecutorGeneration != resolution.ExecutorGeneration {
			return result, fmt.Errorf("daily correction durable reconcile identity differs from public Apply result")
		}
	}
	result.ExecutorGeneration = resolution.ExecutorGeneration

	if state, generation, terminal, err := dailyLaneTerminal(result.CaseRoot, lane); err != nil {
		return result, err
	} else if terminal {
		result.FinalState = state
		result.ExecutorGeneration = generation
		result.Replay = true
		return result, nil
	}
	dailyOpt := DailyOptions{
		ClaudePath:                        hostOpt.ClaudePath,
		ExpectedClaudeExecutableSHA256:    hostOpt.ExpectedClaudeExecutableSHA256,
		ExpectedClaudeExecutablePublisher: hostOpt.ExpectedClaudeExecutablePublisher,
	}
	if err := bindDailyTrustedClaude(&dailyOpt); err != nil {
		return result, err
	}
	hostOpt.ClaudePath = dailyOpt.ClaudePath
	hostOpt.ExpectedClaudeExecutableSHA256 = dailyOpt.ExpectedClaudeExecutableSHA256
	hostOpt.ExpectedClaudeExecutablePublisher = dailyOpt.ExpectedClaudeExecutablePublisher
	hostResult, err := Run(parent, hostOpt)
	addDailyHostRun(&result, hostResult)
	if err != nil {
		return result, err
	}
	result.FinalState = hostResult.FinalMode
	if err := finishDailyCompletion(result.CaseRoot, result.Pack, &result); err != nil {
		return result, err
	}
	return result, nil
}

func dailyCorrectionLane(caseRoot string, inspection missionintent.Inspection, correction, actor string) (string, mission.BoardLane, map[string]any, error) {
	board, err := mission.ReadBoard(caseRoot)
	if err != nil {
		return "", mission.BoardLane{}, nil, fmt.Errorf("daily correction requires an initialized board: %w", err)
	}
	scope := dailyCorrectionScope(caseRoot, inspection)
	if inspection.Committed {
		laneID := inspection.Identity.InitialLane
		lane, ok := mission.LookupBoardLane(board.Lanes, laneID, false)
		if !ok {
			return "", mission.BoardLane{}, nil, fmt.Errorf("daily committed initial lane is missing from board: %s", laneID)
		}
		latest, found, latestErr := memberexecution.Latest(caseRoot, laneID)
		if latestErr != nil {
			return "", mission.BoardLane{}, nil, latestErr
		}
		if found && latest.State == "intake-ready" && latest.Manifest != nil && latest.Owner.Executor == lane.CurrentExecutor && latest.Owner.ExecutorGeneration == lane.ExecutorGeneration {
			targetRef := relativeLiveAcceptancePath(caseRoot, latest.ManifestPath)
			rejection, rejected, rejectionErr := workstream.CurrentMemberManifestReviewerRejection(caseRoot, laneID, targetRef)
			if rejectionErr != nil {
				return "", mission.BoardLane{}, nil, rejectionErr
			}
			if rejected {
				eventID := dailyCorrectionEventID(scope, laneID, correction, rejection)
				existing, existingErr := existingDailyCorrectionByID(caseRoot, eventID, laneID, correction, actor)
				return laneID, lane, existing, existingErr
			}
		}
		if eventID := strings.TrimSpace(lane.LastReconciledIntervention); eventID != "" {
			existing, err := existingDailyCorrectionByID(caseRoot, eventID, laneID, correction, actor)
			if err != nil {
				return "", mission.BoardLane{}, nil, err
			}
			if existing == nil {
				return "", mission.BoardLane{}, nil, fmt.Errorf("daily correction board reconcile identity is missing from the intervention ledger: %s", eventID)
			}
			return laneID, lane, existing, nil
		}
		existing, err := existingDailyCorrectionForRequest(caseRoot, scope, laneID, correction, actor)
		return laneID, lane, existing, err
	}
	matches := []struct {
		lane  mission.BoardLane
		event map[string]any
	}{}
	for _, lane := range board.Lanes {
		if !strings.EqualFold(strings.TrimSpace(lane.Status), "closed") && !strings.EqualFold(strings.TrimSpace(lane.Status), "archived") {
			continue
		}
		event, err := existingDailyCorrectionForRequest(caseRoot, scope, lane.ID, correction, actor)
		if err != nil {
			return "", mission.BoardLane{}, nil, err
		}
		if event != nil {
			matches = append(matches, struct {
				lane  mission.BoardLane
				event map[string]any
			}{lane, event})
		}
	}
	if len(matches) == 1 {
		return matches[0].lane.ID, matches[0].lane, matches[0].event, nil
	}
	if len(matches) > 1 {
		return "", mission.BoardLane{}, nil, fmt.Errorf("daily correction identity matches multiple legacy lanes")
	}
	candidates := []mission.BoardLane{}
	for _, lane := range mission.OpenBoardLanes(board.Lanes) {
		if lane.Authority || strings.TrimSpace(lane.CurrentExecutor) == "" || lane.ExecutorGeneration < 1 {
			continue
		}
		latest, ok, inspectErr := memberexecution.Latest(caseRoot, lane.ID)
		if inspectErr == nil && ok && latest.State == "intake-ready" && latest.Owner.Executor == lane.CurrentExecutor && latest.Owner.ExecutorGeneration == lane.ExecutorGeneration {
			candidates = append(candidates, lane)
		}
	}
	if len(candidates) != 1 {
		return "", mission.BoardLane{}, nil, fmt.Errorf("daily correction requires exactly one current intake-ready feature lane; found %d", len(candidates))
	}
	return candidates[0].ID, candidates[0], nil, nil
}

func currentDailyCorrectionMemberTarget(caseRoot string, boardLane mission.BoardLane) (workstream.MemberReviewerRejection, string, error) {
	latest, ok, err := memberexecution.Latest(caseRoot, boardLane.ID)
	if err != nil {
		return workstream.MemberReviewerRejection{}, "", err
	}
	if !ok || latest.State != "intake-ready" || latest.Manifest == nil || latest.Owner.Executor != boardLane.CurrentExecutor || latest.Owner.ExecutorGeneration != boardLane.ExecutorGeneration {
		return workstream.MemberReviewerRejection{}, "", fmt.Errorf("daily correction requires the current real member result to be durably intake-ready")
	}
	targetRef := relativeLiveAcceptancePath(caseRoot, latest.ManifestPath)
	rejection, rejected, err := workstream.CurrentMemberManifestReviewerRejection(caseRoot, boardLane.ID, targetRef)
	if err != nil {
		return workstream.MemberReviewerRejection{}, "", err
	}
	if !rejected {
		return workstream.MemberReviewerRejection{}, "", fmt.Errorf("daily correction requires a canonical reviewer rejection for the current member manifest")
	}
	return rejection, targetRef, nil
}

func dailyCorrectionArgs(caseRoot, pack, lane, correction, actor, eventID, createdAt, targetRef string, rejections ...workstream.MemberReviewerRejection) []string {
	args := []string{
		"-Command", "note", "-Target", caseRoot, "-Pack", pack,
		"-Kind", "intervention", "-Lane", lane,
		"-Subject", "daily human correction", "-Summary", correction,
		"-Actor", actor, "-Action", "override", "-Status", "open",
		"-EventId", eventID, "-CreatedAt", createdAt, "-TargetRef", targetRef,
	}
	if len(rejections) > 0 {
		rejection := rejections[0]
		args = append(args,
			"-Related", rejection.VerificationEventID+","+rejection.DecisionEventID,
			"-EvidenceRefs", strings.Join(rejection.EvidenceRefs, ","),
			"-ReviewerPacketId", rejection.PacketID, "-ReviewerRouteId", rejection.RouteID, "-ReviewerShardId", rejection.ShardID,
			"-ReviewerPacketPath", rejection.PacketPath, "-ReviewerResultLineagePath", rejection.ReviewerResultPath, "-ReviewerLineageSession", rejection.ReviewerSession,
			"-ReviewerDispatchReceiptPath", rejection.ReviewerDispatchPath, "-ReviewerDispatchReceiptSha256", rejection.ReviewerDispatchSHA256,
			"-ReviewerCompletionReceiptPath", rejection.ReviewerCompletionPath, "-ReviewerCompletionReceiptSha256", rejection.ReviewerCompletionSHA256,
			"-ReviewerLineageInputPath", rejection.ReviewerResultInputPath, "-ReviewerLineageInputSha256", rejection.ReviewerResultInputSHA256,
			"-ReviewerLineageInputBytes", fmt.Sprint(rejection.ReviewerResultInputBytes), "-ReviewerLineageResultSha256", rejection.ReviewerResultSHA256,
			"-ReviewerManifestSha256", rejection.ManifestSHA256, "-ReviewerVerificationEventId", rejection.VerificationEventID, "-ReviewerDecisionEventId", rejection.DecisionEventID,
			"-ReviewerOwnerExecutor", rejection.OwnerExecutor, "-ReviewerOwnerGeneration", fmt.Sprint(rejection.OwnerGeneration),
		)
	}
	return append(args, "-WhatIf", "-Format", "json")
}

func dailyCorrectionScope(caseRoot string, inspection missionintent.Inspection) string {
	if inspection.Committed && inspection.MissionIntentSHA256 != "" {
		return strings.ToLower(inspection.MissionIntentSHA256)
	}
	return strings.ToLower(filepath.Clean(caseRoot))
}

func dailyCorrectionEventID(scope, lane, correction string, rejection ...workstream.MemberReviewerRejection) string {
	identity := []string{"daily-correction-v1", scope, lane, correction}
	if len(rejection) > 0 {
		item := rejection[0]
		identity = []string{"daily-correction-v2", scope, lane, correction, item.ManifestRef, item.ManifestSHA256, item.PacketID, item.ShardID, item.ReviewerResultInputSHA256, item.VerificationEventID, item.DecisionEventID, item.ReviewerSession, item.OwnerExecutor, fmt.Sprint(item.OwnerGeneration)}
	}
	sum := sha256.Sum256([]byte(strings.Join(identity, "\x00")))
	return "daily-correction-" + hex.EncodeToString(sum[:12])
}

func existingDailyCorrectionByID(caseRoot, eventID, lane, correction, actor string) (map[string]any, error) {
	items, err := mission.ReadStrictFact(caseRoot, "intervention")
	if err != nil {
		return nil, err
	}
	var found map[string]any
	for _, item := range items {
		if mission.Value(item, "eventId") != eventID {
			continue
		}
		if found != nil {
			return nil, fmt.Errorf("daily correction event is duplicated: %s", eventID)
		}
		if err := verifyDailyCorrection(item, lane, correction, actor, eventID); err != nil {
			return nil, err
		}
		found = item
	}
	return found, nil
}

func validateExistingDailyCorrectionRejection(caseRoot string, inspection missionintent.Inspection, lane, correction string, event map[string]any) error {
	if event == nil || mission.Value(event, "reviewerVerificationEventId") == "" || mission.Value(event, "reviewerDecisionEventId") == "" {
		return nil
	}
	targetRef := mission.Value(event, "target")
	rejection, rejected, err := workstream.CurrentMemberManifestReviewerRejection(caseRoot, lane, targetRef)
	if err != nil {
		return err
	}
	if !rejected || dailyCorrectionEventID(dailyCorrectionScope(caseRoot, inspection), lane, correction, rejection) != mission.Value(event, "eventId") || !dailyCorrectionBindsRejection(event, rejection) {
		return fmt.Errorf("existing daily correction does not match its canonical reviewer rejection")
	}
	return nil
}

func existingDailyCorrectionForRequest(caseRoot, scope, lane, correction, actor string) (map[string]any, error) {
	items, err := mission.ReadStrictFact(caseRoot, "intervention")
	if err != nil {
		return nil, err
	}
	legacyID := dailyCorrectionEventID(scope, lane, correction)
	var found map[string]any
	for _, item := range items {
		if mission.Value(item, "lane") != lane || mission.Value(item, "subject") != "daily human correction" || mission.Value(item, "summary") != correction || mission.Value(item, "actor") != actor || mission.Value(item, "action") != "override" || !strings.EqualFold(mission.Value(item, "status"), "open") {
			continue
		}
		eventID := mission.Value(item, "eventId")
		isLegacy := eventID == legacyID
		isRejectionBound := mission.Value(item, "reviewerVerificationEventId") != "" && mission.Value(item, "reviewerDecisionEventId") != ""
		if !isLegacy && !isRejectionBound {
			continue
		}
		if found != nil {
			return nil, fmt.Errorf("daily correction logical request matches multiple durable events for lane %s", lane)
		}
		if err := verifyDailyCorrection(item, lane, correction, actor, eventID); err != nil {
			return nil, err
		}
		found = item
	}
	return found, nil
}

func verifyDailyCorrection(event map[string]any, lane, correction, actor, eventID string, bindings ...any) error {
	if event == nil || mission.Value(event, "eventId") != eventID || mission.Value(event, "kind") != "intervention" || mission.Value(event, "lane") != lane || mission.Value(event, "subject") != "daily human correction" || mission.Value(event, "summary") != correction || mission.Value(event, "actor") != actor || mission.Value(event, "action") != "override" || !strings.EqualFold(mission.Value(event, "status"), "open") {
		return fmt.Errorf("existing daily correction does not match the exact logical request: %s", eventID)
	}
	if len(bindings) > 0 {
		targetRef, ok := bindings[0].(string)
		if !ok || mission.Value(event, "target") != targetRef {
			return fmt.Errorf("existing daily correction does not match the exact logical request: %s", eventID)
		}
	}
	if len(bindings) > 1 {
		rejection, ok := bindings[1].(workstream.MemberReviewerRejection)
		if !ok || !dailyCorrectionBindsRejection(event, rejection) {
			return fmt.Errorf("existing daily correction does not match canonical reviewer rejection: %s", eventID)
		}
	}
	return nil
}

func dailyCorrectionBindsRejection(event map[string]any, rejection workstream.MemberReviewerRejection) bool {
	return mission.Value(event, "packetId") == rejection.PacketID &&
		mission.Value(event, "routeId") == rejection.RouteID &&
		mission.Value(event, "shardId") == rejection.ShardID &&
		mission.Value(event, "packetPath") == rejection.PacketPath &&
		mission.Value(event, "reviewerResultPath") == rejection.ReviewerResultPath &&
		mission.Value(event, "reviewerSession") == rejection.ReviewerSession &&
		mission.Value(event, "reviewerDispatchReceiptPath") == rejection.ReviewerDispatchPath &&
		strings.EqualFold(mission.Value(event, "reviewerDispatchReceiptSha256"), rejection.ReviewerDispatchSHA256) &&
		mission.Value(event, "reviewerCompletionReceiptPath") == rejection.ReviewerCompletionPath &&
		strings.EqualFold(mission.Value(event, "reviewerCompletionReceiptSha256"), rejection.ReviewerCompletionSHA256) &&
		mission.Value(event, "reviewerResultInputPath") == rejection.ReviewerResultInputPath &&
		strings.EqualFold(mission.Value(event, "reviewerResultInputSha256"), rejection.ReviewerResultInputSHA256) &&
		mission.Value(event, "reviewerResultInputBytes") == fmt.Sprint(rejection.ReviewerResultInputBytes) &&
		strings.EqualFold(mission.Value(event, "reviewerResultSha256"), rejection.ReviewerResultSHA256) &&
		strings.EqualFold(mission.Value(event, "reviewerManifestSha256"), rejection.ManifestSHA256) &&
		mission.Value(event, "reviewerVerificationEventId") == rejection.VerificationEventID &&
		mission.Value(event, "reviewerDecisionEventId") == rejection.DecisionEventID &&
		mission.Value(event, "ownerExecutor") == rejection.OwnerExecutor &&
		mission.Value(event, "ownerGeneration") == fmt.Sprint(rejection.OwnerGeneration)
}

type dailyCorrectionResolution struct {
	EventID            string
	Time               string
	Actor              string
	Executor           string
	ExecutorGeneration int
}

func inspectDailyCorrectionResolution(caseRoot, lane, eventID string) (dailyCorrectionResolution, bool, error) {
	items, err := mission.ReadStrictFact(caseRoot, "intervention")
	if err != nil {
		return dailyCorrectionResolution{}, false, err
	}
	var resolution dailyCorrectionResolution
	found := false
	for _, item := range items {
		if mission.Value(item, "lane") != lane || mission.Value(item, "resolvesEventId") != eventID || mission.Value(item, "action") != "reconcile" || !strings.EqualFold(mission.Value(item, "status"), "resolved") {
			continue
		}
		if found {
			return dailyCorrectionResolution{}, false, fmt.Errorf("daily correction has multiple reconcile resolutions: %s", eventID)
		}
		resolution = dailyCorrectionResolution{
			EventID:  strings.TrimSpace(mission.Value(item, "eventId")),
			Time:     strings.TrimSpace(mission.Value(item, "time")),
			Actor:    strings.TrimSpace(mission.Value(item, "actor")),
			Executor: strings.TrimSpace(mission.Value(item, "executor")),
		}
		if resolution.EventID == "" || resolution.Time == "" || resolution.Actor == "" || resolution.Executor == "" {
			return dailyCorrectionResolution{}, false, fmt.Errorf("daily correction resolution omitted durable identity: %s", eventID)
		}
		found = true
	}
	if !found {
		return dailyCorrectionResolution{}, false, nil
	}
	board, err := mission.ReadBoard(caseRoot)
	if err != nil {
		return dailyCorrectionResolution{}, false, err
	}
	boardLane, ok := mission.LookupBoardLane(board.Lanes, lane, false)
	if !ok || boardLane.LastReconciledIntervention != eventID || boardLane.LastReconcileAt != resolution.Time || boardLane.CurrentExecutor != resolution.Executor || boardLane.ExecutorGeneration < 1 {
		return dailyCorrectionResolution{}, false, fmt.Errorf("daily correction durable resolution differs from current board owner: %s", eventID)
	}
	resolution.ExecutorGeneration = boardLane.ExecutorGeneration
	return resolution, true, nil
}

func dailyCurrentMemberReady(caseRoot, laneID, goal string) (int, bool, error) {
	board, err := mission.ReadBoard(caseRoot)
	if err != nil {
		return 0, false, err
	}
	lane, ok := mission.LookupBoardLane(board.Lanes, laneID, false)
	if !ok {
		return 0, false, fmt.Errorf("daily committed initial lane is missing from board: %s", laneID)
	}
	latest, found, err := memberexecution.Latest(caseRoot, laneID)
	if err != nil || !found {
		return lane.ExecutorGeneration, false, err
	}
	ready := latest.State == "intake-ready" &&
		latest.Manifest != nil &&
		latest.TaskContext != nil &&
		latest.TaskContext.Correction == nil &&
		latest.TaskContext.Goal == goal &&
		latest.TaskContext.GoalSource == "committed-mission-intent" &&
		latest.Owner.Executor == lane.CurrentExecutor &&
		latest.Owner.ExecutorGeneration == lane.ExecutorGeneration
	return lane.ExecutorGeneration, ready, nil
}

func dailyLaneTerminal(caseRoot, laneID string) (string, int, bool, error) {
	if strings.TrimSpace(laneID) == "" {
		return "", 0, false, nil
	}
	board, err := mission.ReadBoard(caseRoot)
	if err != nil {
		if os.IsNotExist(err) {
			return "", 0, false, nil
		}
		return "", 0, false, err
	}
	lane, ok := mission.LookupBoardLane(board.Lanes, laneID, false)
	if !ok {
		return "", 0, false, nil
	}
	status := strings.ToLower(strings.TrimSpace(lane.Status))
	if status == "archived" {
		return "", lane.ExecutorGeneration, false, fmt.Errorf("daily terminal replay refuses archived lane %s because no durable archive transition is supported", laneID)
	}
	if status != "closed" {
		return "", lane.ExecutorGeneration, false, nil
	}
	lifecycle, err := lanecompletion.Inspect(caseRoot, laneID)
	if err != nil {
		return "", lane.ExecutorGeneration, false, err
	}
	if lifecycle.State == lanecompletion.StatePending {
		return "", lane.ExecutorGeneration, false, fmt.Errorf("daily terminal replay refuses pending lane completion publication")
	}
	if lifecycle.State != lanecompletion.StateComplete || lifecycle.CurrentCompletion == nil {
		return "", lane.ExecutorGeneration, false, fmt.Errorf("daily terminal board state lacks a committed lane completion receipt")
	}
	verified, err := workstream.InspectLaneCompletion(caseRoot, laneID)
	if err != nil {
		return "", lane.ExecutorGeneration, false, err
	}
	if verified.Lane != laneID || verified.ExecutorGeneration != lane.ExecutorGeneration || !verified.NoAuthority || !verified.NoConfirmed || !verified.NoHeavyTool {
		return "", lane.ExecutorGeneration, false, fmt.Errorf("daily terminal lane completion does not match the current fail-closed board state")
	}
	return "lane-" + status, lane.ExecutorGeneration, true, nil
}

func finishDailyCompletion(caseRoot, pack string, result *DailyResult) error {
	if result != nil && result.FinalState == "reviewer-rejected-awaiting-correction" {
		result.Blocked = true
		return nil
	}
	status, err := runPublicStatus(caseRoot, pack)
	if err != nil {
		return err
	}
	if status.CaseMission != nil && status.CaseMission.MissionCompletion != nil && status.CaseMission.MissionCompletion.Ready && status.CaseMission.MissionCompletion.OperationallyComplete {
		result.FinalState = "mission-complete"
		result.Replay = result.SessionLaunches == 0
		return nil
	}
	request := currentDailyRequest(status)
	if request == nil || !request.CommandExecutable || request.Blocked {
		result.Blocked = true
		result.FinalState = "attention-required"
		return nil
	}
	command, err := publicCommandName(request.Command)
	if err != nil {
		return err
	}
	if command != "complete" {
		result.Blocked = true
		result.FinalState = "attention-required"
		return nil
	}
	step, err := runPublicDriverStep(caseRoot, pack)
	if err != nil {
		return fmt.Errorf("daily public completion: %w", err)
	}
	if step.ResultCommand != "complete" || step.Completion == nil || step.Completion.Blocked || !step.Completion.Applied || step.Completion.Lane.Status != "closed" {
		return fmt.Errorf("daily public completion did not close the current lane")
	}
	result.DriverSteps = append(result.DriverSteps, step.ResultCommand)
	result.Completion = step.Completion
	result.Lane = step.Completion.Lane.ID
	result.ExecutorGeneration = step.Completion.Lane.ExecutorGeneration
	result.FinalState = "lane-closed"
	return nil
}

func addDailyHostRun(result *DailyResult, host Result) {
	result.HostRuns = append(result.HostRuns, host)
	result.SessionLaunches += host.SessionLaunches
	result.SessionCompletions += host.SessionCompletions
	result.Replacements += host.Replacements
	result.Failure = host.Failure
}

func dailyExecutorID(caseRoot string, generation int) string {
	sum := sha256.Sum256([]byte(strings.ToLower(filepath.Clean(caseRoot))))
	return fmt.Sprintf("rekit-daily-member-g%d-%s", generation, hex.EncodeToString(sum[:6]))
}
