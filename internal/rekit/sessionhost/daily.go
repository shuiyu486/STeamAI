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
	"github.com/shuiyu486/re-context-kits/internal/rekit/executioncontrol"
	"github.com/shuiyu486/re-context-kits/internal/rekit/instance"
	"github.com/shuiyu486/re-context-kits/internal/rekit/lanecompletion"
	"github.com/shuiyu486/re-context-kits/internal/rekit/laneid"
	"github.com/shuiyu486/re-context-kits/internal/rekit/manifest"
	"github.com/shuiyu486/re-context-kits/internal/rekit/memberexecution"
	"github.com/shuiyu486/re-context-kits/internal/rekit/mission"
	"github.com/shuiyu486/re-context-kits/internal/rekit/missionintent"
	"github.com/shuiyu486/re-context-kits/internal/rekit/missionsuccessor"
	"github.com/shuiyu486/re-context-kits/internal/rekit/onboarding"
	"github.com/shuiyu486/re-context-kits/internal/rekit/projectexecution"
	"github.com/shuiyu486/re-context-kits/internal/rekit/projectstate"
	rekitruntime "github.com/shuiyu486/re-context-kits/internal/rekit/runtime"
	syncreview "github.com/shuiyu486/re-context-kits/internal/rekit/sync"
	"github.com/shuiyu486/re-context-kits/internal/rekit/workstream"
)

const defaultDailyActor = "rekit-daily-front-door"

const (
	DailyInputArtifactAnalysis   = memberexecution.TaskBindingArtifactAnalysis
	DailyInputWorkspaceInventory = memberexecution.TaskBindingWorkspaceInventory
)

type DailyInputRequest struct {
	Mode         string `json:"mode"`
	ArtifactPath string `json:"artifactPath,omitempty"`
	Scope        string `json:"scope,omitempty"`
}

type DailyInputReadiness struct {
	State        string `json:"state"`
	Mode         string `json:"mode,omitempty"`
	ArtifactPath string `json:"artifactPath,omitempty"`
	Scope        string `json:"scope,omitempty"`
}

type DailyOptions struct {
	Target                            string
	Goal                              string
	Correction                        string
	SelectedLane                      string
	Control                           executioncontrol.Options
	ControlWhatIf                     bool
	ControlApply                      bool
	DirectoryAdoptionAction           string
	DirectoryAdoptionPack             string
	ExpectedInitPlanSHA256            string
	SuccessorWhatIf                   bool
	SuccessorApply                    bool
	SuccessorPublicationStamp         string
	ExpectedSuccessorPlanSHA256       string
	InitializationRepoRoot            string
	InitializationSourceExecutable    string
	Actor                             string
	ClaudePath                        string
	ExpectedClaudeExecutableSHA256    string
	ExpectedClaudeExecutablePublisher string
	Model                             string
	Timeout                           time.Duration
	MaxAttempts                       int
	Input                             DailyInputRequest
	onCaseReady                       func(string) error
	beforeMemberRun                   func(caseRoot, pack, lane string) error
	binaryREAdapterPath               string
	binaryREAdapterRunner             binaryREAuthorizedRunner
	webSecurityAdapterPath            string
	webSecurityAdapterRunner          binaryREAuthorizedRunner
	evidenceReviewRunner              func(context.Context, Options, mission.CurrentLoopExternalSessionHarnessPackage, string, func() error) claudeRun
	stopAfterMemberSegment            bool
	projectExecutionLease             *projectexecution.Lease
}

type DailyResult struct {
	SchemaVersion              int                                    `json:"schemaVersion"`
	Command                    string                                 `json:"command"`
	Mode                       string                                 `json:"mode"`
	CaseRoot                   string                                 `json:"caseRoot"`
	Pack                       string                                 `json:"pack,omitempty"`
	Lane                       string                                 `json:"lane,omitempty"`
	FinalState                 string                                 `json:"finalState"`
	Replay                     bool                                   `json:"replay"`
	Blocked                    bool                                   `json:"blocked,omitempty"`
	OnboardingApplied          bool                                   `json:"onboardingApplied,omitempty"`
	OnboardingReplay           bool                                   `json:"onboardingReplay,omitempty"`
	CorrectionEventID          string                                 `json:"correctionEventId,omitempty"`
	ExecutorGeneration         int                                    `json:"executorGeneration,omitempty"`
	DriverSteps                []string                               `json:"driverSteps,omitempty"`
	HostRuns                   []Result                               `json:"hostRuns,omitempty"`
	Completion                 *workstream.CompleteResult             `json:"completion,omitempty"`
	ReopenOperationID          string                                 `json:"reopenOperationId,omitempty"`
	ReopenOperationCommit      *lanecompletion.OperationCommit        `json:"reopenOperationCommit,omitempty"`
	SessionLaunches            int                                    `json:"sessionLaunches"`
	SessionCompletions         int                                    `json:"sessionCompletions"`
	Replacements               int                                    `json:"replacements"`
	Failure                    *FailureDiagnosis                      `json:"failure,omitempty"`
	Action                     *DailyUserAction                       `json:"action,omitempty"`
	CurrentDriverRequest       *mission.MissionCommanderDriverRequest `json:"currentDriverRequest,omitempty"`
	CurrentDriverRequestSHA256 string                                 `json:"currentDriverRequestSha256,omitempty"`
	CurrentSyncRecovery        *syncreview.CurrentSyncRecovery        `json:"currentSyncRecovery,omitempty"`
	DirectoryAdoption          *DailyDirectoryAdoption                `json:"directoryAdoption,omitempty"`
	ExecutionControl           *executioncontrol.Plan                 `json:"executionControl,omitempty"`
	SuccessorMission           *missionsuccessor.Result               `json:"successorMission,omitempty"`
	BinaryREAdapter            *BinaryREAdapterLifecycleResult        `json:"binaryReAdapter,omitempty"`
	WebSecurityAdapter         *WebSecurityAdapterLifecycleResult     `json:"webSecurityAdapter,omitempty"`
	InputReadiness             *DailyInputReadiness                   `json:"inputReadiness,omitempty"`
	Boundary                   []string                               `json:"boundary"`
}

func RunDaily(parent context.Context, opt DailyOptions) (DailyResult, error) {
	return runDaily(parent, opt, false)
}

// RunDailyRecovery exposes only the zero-launch current-sync recovery result.
// If the durable transaction is no longer pending, it fails instead of falling
// through to onboarding, lane mutation, Claude discovery, or process launch.
func RunDailyRecovery(parent context.Context, opt DailyOptions) (DailyResult, error) {
	return runDaily(parent, opt, true)
}

func runDaily(parent context.Context, opt DailyOptions, recoveryOnly bool) (result DailyResult, retErr error) {
	defer func() {
		if retErr != nil && result.Failure == nil {
			result.Failure = diagnosisForError(retErr, result.SessionLaunches, opt.MaxAttempts, len(result.DriverSteps))
		}
		if retErr != nil && len(result.DriverSteps) > 0 && result.Failure != nil && !result.Failure.MutationApplied {
			failure := *result.Failure
			failure.MutationApplied = true
			failure.MutationBoundary = "durable-runtime-step-may-have-committed"
			result.Failure = &failure
		}
		if result.Mode != string(DailyOperationControl) && result.CaseRoot != "" && result.Pack != "" && result.FinalState != "not-started" {
			bindDailyResultCurrentDriverRequest(&result)
		}
		if result.Failure != nil || result.Action == nil {
			result.Action = dailyUserAction(result)
		}
	}()
	request, requestErr := ResolveDailyRequest(opt)
	result = DailyResult{
		SchemaVersion: 1,
		Command:       "rekit-host-daily",
		Mode:          string(request.Operation),
		FinalState:    "not-started",
		Boundary: []string{
			"the daily front door derives lifecycle identity and consumes only public exact preview/apply requests",
			"all member output and ReviewerResult bytes come from spawned Claude Code processes, never from the front door",
			"the front door does not write authority/confirmed state or execute an unauthorized heavy action",
		},
	}
	if requestErr != nil {
		return result, requestErr
	}
	goal := request.Goal
	correction := request.Correction
	if request.ControlRequested {
		if recoveryOnly {
			return result, fmt.Errorf("project-local recovery front door does not apply lane execution control")
		}
		return runDailyControl(opt, result)
	}
	var err error
	executionLease := opt.projectExecutionLease
	executionOwned := false
	defer func() {
		if executionOwned && executionLease != nil {
			retErr = errors.Join(retErr, executionLease.Unlock())
		}
	}()
	ensureExecutionLease := func(caseRoot string) error {
		if executionLease != nil {
			if opt.SuccessorApply {
				return executionLease.ValidateExclusiveFor(caseRoot)
			}
			return executionLease.ValidateFor(caseRoot)
		}
		acquire := acquireSharedForCurrentProject
		if opt.SuccessorApply {
			acquire = acquireExclusiveForCurrentProject
		}
		acquired, acquireErr := acquire(caseRoot)
		if acquireErr != nil {
			return acquireErr
		}
		if acquired != nil {
			executionLease = acquired
			executionOwned = true
		}
		return nil
	}
	if err := ensureExecutionLease(opt.Target); err != nil {
		return result, err
	}
	opt.projectExecutionLease = executionLease
	target, err := classifyDailyTarget(opt.Target)
	if err != nil {
		return result, err
	}
	caseRoot := target.Root
	exists := target.Kind != dailyTargetMissing
	result.CaseRoot = caseRoot
	if exists {
		stateRoot, stateErr := projectstate.Resolve(caseRoot)
		if stateErr != nil {
			return result, stateErr
		}
		if stateRoot.Existing && !stateRoot.Legacy && stateRoot.Dir == projectstate.CurrentDir {
			recovery, recoveryErr := syncreview.InspectCurrentSyncRecovery(caseRoot)
			if recoveryErr != nil {
				return result, fmt.Errorf("inspect current project update recovery: %w", recoveryErr)
			}
			if recovery.Pending {
				result.Pack = recovery.Pack
				result.FinalState = "maintenance-recovery-required"
				result.Blocked = true
				result.CurrentSyncRecovery = &recovery
				result.Action = dailyCurrentSyncRecoveryAction(recovery)
				return result, nil
			}
		}
	}
	if recoveryOnly {
		return result, fmt.Errorf(
			"project-local recovery front door requires a pending durable current project update",
		)
	}
	adoptionRequested := request.AdoptionRequested
	if target.Kind != dailyTargetOrdinary && adoptionRequested {
		return result, fmt.Errorf("daily directory adoption controls require an ordinary directory target")
	}
	if target.Kind == dailyTargetOrdinary {
		if err := runDailyDirectoryAdoption(opt, &result); err != nil {
			return result, err
		}
		return result, nil
	}
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
		if strings.TrimSpace(opt.InitializationSourceExecutable) == "" {
			return result, fmt.Errorf("daily onboarding requires a verified unified runtime executable source")
		}
		inspection, err = applyDailyOnboarding(
			caseRoot,
			goal,
			opt.Actor,
			&result,
			opt.InitializationSourceExecutable,
		)
		if err != nil {
			return result, err
		}
		exists = true
		if err := ensureExecutionLease(caseRoot); err != nil {
			return result, fmt.Errorf("acquire current project execution lease after onboarding: %w", err)
		}
		opt.projectExecutionLease = executionLease
		if recovery, recoveryErr := syncreview.InspectCurrentSyncRecovery(caseRoot); recoveryErr != nil {
			return result, fmt.Errorf("inspect current project update after onboarding: %w", recoveryErr)
		} else if recovery.Pending {
			result.Pack = recovery.Pack
			result.FinalState = "maintenance-recovery-required"
			result.Blocked = true
			result.CurrentSyncRecovery = &recovery
			result.Action = dailyCurrentSyncRecoveryAction(recovery)
			return result, nil
		}
		if opt.onCaseReady != nil {
			if bindErr := opt.onCaseReady(caseRoot); bindErr != nil {
				return result, fmt.Errorf("bind fresh daily case root identity: %w", bindErr)
			}
		}
	} else if inspection.State == "pending" {
		if correction != "" {
			return result, fmt.Errorf("daily correction cannot run while onboarding publication is pending")
		}
		if goal != "" && goal != strings.TrimSpace(inspection.Identity.Goal) {
			return result, fmt.Errorf("daily goal differs from the immutable pending mission intent")
		}
		if err := recoverDailyOnboarding(caseRoot, inspection, &result); err != nil {
			return result, err
		}
		if err := ensureExecutionLease(caseRoot); err != nil {
			return result, fmt.Errorf("acquire current project execution lease after onboarding recovery: %w", err)
		}
		opt.projectExecutionLease = executionLease
		if recovery, recoveryErr := syncreview.InspectCurrentSyncRecovery(caseRoot); recoveryErr != nil {
			return result, fmt.Errorf("inspect current project update after onboarding recovery: %w", recoveryErr)
		} else if recovery.Pending {
			result.Pack = recovery.Pack
			result.FinalState = "maintenance-recovery-required"
			result.Blocked = true
			result.CurrentSyncRecovery = &recovery
			result.Action = dailyCurrentSyncRecoveryAction(recovery)
			return result, nil
		}
		inspection, err = missionintent.Inspect(caseRoot)
		if err != nil || !inspection.Committed {
			return result, fmt.Errorf("daily onboarding recovery did not commit: state=%s err=%v", inspection.State, err)
		}
		exists = true
	} else if inspection.Committed {
		if opt.SuccessorApply && goal != "" && goal == strings.TrimSpace(inspection.Identity.Goal) {
			result.Pack = inspection.Identity.Pack
			replay, replayErr := missionsuccessor.ApplyWithLease(caseRoot, missionsuccessor.Options{
				Goal: goal, Actor: opt.Actor, PublicationStamp: opt.SuccessorPublicationStamp,
				ExpectedPlanSHA256: opt.ExpectedSuccessorPlanSHA256,
			}, opt.projectExecutionLease)
			if replayErr != nil {
				return result, replayErr
			}
			result.SuccessorMission = &replay
			result.FinalState = DailyActionReadyToContinue
			result.Replay = replay.Replay
			result.Action = &DailyUserAction{Code: DailyActionReadyToContinue, Message: "新任务已按精确计划建立；本次没有启动 Claude。", Now: "新任务已成为当前任务。", Reason: "successor mission Apply 已提交新的隔离任务代并保留旧任务审计。", Next: "刷新状态并从新任务的唯一初始 lane 预览继续。"}
			return result, nil
		}
		if goal != "" && goal != strings.TrimSpace(inspection.Identity.Goal) {
			completion, completionErr := workstream.InspectMissionCompletion(caseRoot)
			if completionErr != nil || !completion.Ready || !completion.OperationallyComplete || completion.State != "mission-complete" {
				return result, fmt.Errorf("daily goal differs from the immutable committed mission intent")
			}
			if dailyInputRequested(opt.Input) {
				return result, fmt.Errorf("typed daily input cannot be combined with an implicit successor mission")
			}
			if strings.TrimSpace(opt.SelectedLane) != "" || correction != "" {
				return result, fmt.Errorf("successor mission goal does not accept a lane or correction")
			}
			result.Pack = inspection.Identity.Pack
			successorOpt := missionsuccessor.Options{
				Goal:               goal,
				Actor:              opt.Actor,
				PublicationStamp:   opt.SuccessorPublicationStamp,
				ExpectedPlanSHA256: opt.ExpectedSuccessorPlanSHA256,
			}
			if opt.SuccessorApply {
				applied, applyErr := missionsuccessor.ApplyWithLease(caseRoot, successorOpt, opt.projectExecutionLease)
				if applyErr != nil {
					return result, applyErr
				}
				result.SuccessorMission = &applied
				result.FinalState = DailyActionReadyToContinue
				result.Replay = applied.Replay
				result.Action = &DailyUserAction{
					Code:    DailyActionReadyToContinue,
					Message: "新任务已按精确计划建立；本次没有启动 Claude。",
					Now:     "新任务已成为当前任务。",
					Reason:  "successor mission Apply 已提交新的隔离任务代并保留旧任务审计。",
					Next:    "刷新状态并从新任务的唯一初始 lane 预览继续。",
				}
				return result, nil
			}
			if opt.ExpectedSuccessorPlanSHA256 != "" || opt.SuccessorPublicationStamp != "" {
				return result, fmt.Errorf("successor mission binding requires -successor-apply")
			}
			plan, previewErr := missionsuccessor.Preview(caseRoot, successorOpt)
			if previewErr != nil {
				return result, previewErr
			}
			result.SuccessorMission = &missionsuccessor.Result{Plan: plan}
			result.FinalState = DailyActionConfirmationRequired
			result.Blocked = true
			result.Action = &DailyUserAction{
				Code:    DailyActionConfirmationRequired,
				Message: "旧任务已完成；已生成新任务的零写入精确预览。",
				Now:     "新任务尚未建立。",
				Reason:  "新目标会创建独立任务代并切换当前任务，需要先确认精确写入计划。",
				Next:    "确认后原样执行 successorMission.applyArgs；本次预览没有启动 Claude。",
			}
			return result, nil
		}
		if opt.SuccessorApply || opt.SuccessorWhatIf || opt.ExpectedSuccessorPlanSHA256 != "" || opt.SuccessorPublicationStamp != "" {
			return result, fmt.Errorf("successor mission controls require a new goal")
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
	preflightOpt := Options{
		Target:                            caseRoot,
		Pack:                              pack,
		SelectedLane:                      strings.TrimSpace(opt.SelectedLane),
		Actor:                             opt.Actor,
		ClaudePath:                        opt.ClaudePath,
		ExpectedClaudeExecutableSHA256:    opt.ExpectedClaudeExecutableSHA256,
		ExpectedClaudeExecutablePublisher: opt.ExpectedClaudeExecutablePublisher,
		Model:                             opt.Model,
		Timeout:                           opt.Timeout,
		MaxAttempts:                       opt.MaxAttempts,
		requireDailyClaudeTrust:           true,
		projectExecutionLease:             opt.projectExecutionLease,
	}
	if preflightOpt.MaxAttempts <= 0 {
		preflightOpt.MaxAttempts = defaultMaxAttempts
	}
	if hostResult, lane, held, preflightErr := prepareHeldClaudeResultWithControl(parent, preflightOpt); preflightErr != nil {
		return result, preflightErr
	} else if held {
		addDailyHostRun(&result, hostResult)
		result.Lane = lane
		result.FinalState = hostResult.FinalMode
		result.Replay = true
		result.Blocked = true
		result.Action = dailyHeldClaudeResultAction(hostResult.FinalMode)
		return result, nil
	}
	if correction != "" {
		if resumed, resumeResult, resumeErr := resumeDailyTerminalCorrection(caseRoot, pack, correction, opt.Actor, strings.TrimSpace(opt.SelectedLane), result); resumed || resumeErr != nil {
			return resumeResult, resumeErr
		}
	}
	correctionRoute := dailyCorrectionRoute{}
	if inspection.Committed {
		selected := strings.TrimSpace(opt.SelectedLane)
		var action *DailyUserAction
		if correction != "" {
			correctionRoute, action, err = resolveDailyCorrectionRoute(caseRoot, pack, correction, opt.Actor, selected, inspection)
			selected = correctionRoute.Lane
		} else {
			selected, action, err = dailySelectedLane(caseRoot, pack, selected, opt.projectExecutionLease, inspection.Identity.InitialLane)
		}
		if err != nil {
			return result, err
		}
		if action != nil {
			result.Blocked = true
			result.Action = action
			return result, nil
		}
		result.Lane = selected
	} else if goal != "" {
		return result, fmt.Errorf("daily goal requires a committed mission intent")
	}

	routeLane := result.Lane
	hostOpt := Options{
		Target:                            caseRoot,
		Pack:                              pack,
		SelectedLane:                      routeLane,
		Actor:                             opt.Actor,
		ClaudePath:                        opt.ClaudePath,
		ExpectedClaudeExecutableSHA256:    opt.ExpectedClaudeExecutableSHA256,
		ExpectedClaudeExecutablePublisher: opt.ExpectedClaudeExecutablePublisher,
		Model:                             opt.Model,
		Timeout:                           opt.Timeout,
		MaxAttempts:                       opt.MaxAttempts,
		requireDailyClaudeTrust:           true,
		projectExecutionLease:             opt.projectExecutionLease,
	}

	if correction != "" {
		switch correctionRoute.Kind {
		case dailyCorrectionRouteTerminal:
			return runDailyTerminalCorrection(hostOpt, correction, result)
		case dailyCorrectionRouteActive:
			return runDailyActiveCorrection(hostOpt, inspection, correction, result, opt)
		case dailyCorrectionRouteReviewer:
			return runDailyCorrection(parent, hostOpt, inspection, correction, result, opt)
		default:
			return result, fmt.Errorf("daily correction route is not executable: %s", correctionRoute.Kind)
		}
	}
	if state, generation, terminal, err := dailyLaneTerminal(caseRoot, result.Lane); err != nil {
		return result, err
	} else if terminal {
		result.FinalState = state
		result.ExecutorGeneration = generation
		result.Replay = true
		return result, nil
	}
	rejected, err := currentReviewerRejectionAwaitingCorrection(caseRoot, pack, routeLane)
	if err != nil {
		return result, err
	}
	if rejected {
		result.FinalState = "reviewer-rejected-awaiting-correction"
		result.Blocked = true
		result.Replay = true
		return result, nil
	}
	if err := ensureDailyStartedWithLease(caseRoot, pack, &result, opt.projectExecutionLease, routeLane); err != nil {
		return result, err
	}
	if opt.beforeMemberRun != nil {
		if err := opt.beforeMemberRun(caseRoot, pack, result.Lane); err != nil {
			return result, fmt.Errorf("prepare daily member run: %w", err)
		}
	}
	adapterReady, err := prepareProductionAdapterBeforeMember(parent, opt, &result)
	if err != nil {
		return result, err
	}
	if !adapterReady {
		return result, nil
	}
	if pack == defaults.DefaultPack {
		status, err := runPublicStatusWithLease(caseRoot, pack, result.Lane, opt.projectExecutionLease)
		if err != nil {
			return result, err
		}
		if currentDailyRequestRequiresMemberInput(status, result.Lane) {
			inputReady, err := prepareDailyInputReadiness(caseRoot, pack, result.Lane, opt.Input, &result)
			if err != nil {
				return result, err
			}
			if !inputReady {
				return result, nil
			}
		} else if dailyInputRequested(opt.Input) {
			return result, fmt.Errorf("typed daily input requires the current member continuation request")
		}
	}
	if goal != "" {
		if opt.stopAfterMemberSegment {
			hostOpt.StopAfterMemberIntake = true
			if _, err := bindHostCurrentDriverRequest(&hostOpt); err != nil {
				return result, fmt.Errorf("bind daily current driver request: %w", err)
			}
			hostResult, err := Run(parent, hostOpt)
			addDailyHostRun(&result, hostResult)
			if err != nil {
				return result, err
			}
			result.FinalState = hostResult.FinalMode
		} else if err := runDailyGoalFlow(parent, hostOpt, caseRoot, pack, &result); err != nil {
			return result, err
		}
	} else {
		owner, err := newDailySessionTransitionOwner(caseRoot, pack, result.Lane, opt.projectExecutionLease)
		if err != nil {
			return result, err
		}
		if err := owner.runHostSegment(parent, hostOpt, &result); err != nil {
			return result, err
		}
		if err := owner.finish(&result); err != nil {
			return result, err
		}
	}
	if result.Lane != "" {
		if latest, ok, inspectErr := memberexecution.Latest(caseRoot, result.Lane); inspectErr == nil && ok {
			result.ExecutorGeneration = latest.Owner.ExecutorGeneration
		}
	}
	result.Replay = result.OnboardingReplay && result.SessionLaunches == 0
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

func applyDailyOnboarding(caseRoot, goal, actor string, result *DailyResult, sourceExecutable ...string) (missionintent.Inspection, error) {
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
	executable := ""
	if len(sourceExecutable) > 0 {
		executable = strings.TrimSpace(sourceExecutable[0])
	}
	var plan onboarding.Plan
	if err := runPublicCLIWithUnifiedExecutable(args, &plan, executable); err != nil {
		return missionintent.Inspection{}, fmt.Errorf("daily public onboard preview: %w", err)
	}
	if plan.IsMutation || plan.Replay || len(plan.ApplyArgs) == 0 {
		return missionintent.Inspection{}, fmt.Errorf("daily public onboard preview omitted a fresh zero-write exact Apply request")
	}
	var applied onboarding.Result
	if err := runPublicCLIWithUnifiedExecutable(plan.ApplyArgs, &applied, executable); err != nil {
		return missionintent.Inspection{}, fmt.Errorf("daily public onboard Apply: %w", err)
	}
	if !applied.Applied || applied.Replay || !applied.Inspection.Committed {
		return missionintent.Inspection{}, fmt.Errorf("daily public onboard Apply did not commit the fresh mission intent")
	}
	result.OnboardingApplied = true
	return applied.Inspection, nil
}

func recoverDailyOnboarding(caseRoot string, inspection missionintent.Inspection, result *DailyResult) error {
	identity := inspection.Identity
	args := []string{"-Command", "onboard", "-Target", caseRoot}
	if identity.SchemaVersion == 2 {
		args = append(args, "-ProjectId", identity.ProjectID)
	}
	args = append(args,
		"-Pack", identity.Pack, "-ProjectName", identity.ProjectName, "-Goal", identity.Goal, "-Actor", identity.Actor,
		"-Executor", identity.Executor, "-InitialLane", identity.InitialLane,
		"-OnboardingPublicationStamp", inspection.PublicationStamp,
		"-ExpectedOnboardingPlanSha256", inspection.OnboardingPlanSHA256,
		"-Apply", "-Format", "json",
	)
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

func dailySelectedLane(caseRoot, pack, selected string, lease *projectexecution.Lease, fallback ...string) (string, *DailyUserAction, error) {
	status, err := runPublicStatusWithLease(caseRoot, pack, "", lease)
	if err != nil {
		return "", nil, err
	}
	choices := dailyStatusLaneChoices(status)
	selected = strings.TrimSpace(selected)
	explicit := selected != ""
	if !explicit && len(choices) == 0 && len(fallback) > 0 {
		fallbackLane := strings.TrimSpace(fallback[0])
		if fallbackLane != "" {
			board, readErr := mission.ReadBoard(caseRoot)
			if readErr == nil {
				if lane, found := mission.LookupBoardLane(board.Lanes, fallbackLane, false); found {
					switch strings.ToLower(strings.TrimSpace(lane.Status)) {
					case "archived":
						return "", nil, fmt.Errorf("daily terminal replay refuses archived lane %s because no durable archive transition is supported", fallbackLane)
					case "closed":
						return fallbackLane, nil, nil
					}
				}
			} else if !os.IsNotExist(readErr) {
				return "", nil, readErr
			}
		}
	}
	if !explicit {
		if len(choices) > 1 {
			return "", dailyLaneSelectionAction(choices), nil
		}
		if len(choices) == 1 {
			selected = choices[0].ID
		} else if len(fallback) > 0 {
			selected = strings.TrimSpace(fallback[0])
		}
	}
	if selected == "" {
		return "", dailyAction(DailyActionBlocked), nil
	}
	choice, ok := dailyChoiceForLane(choices, selected)
	if !ok {
		board, readErr := mission.ReadBoard(caseRoot)
		if readErr != nil {
			if !explicit && os.IsNotExist(readErr) {
				return selected, nil, nil
			}
			return "", nil, readErr
		}
		lane, found := mission.LookupBoardLane(board.Lanes, selected, false)
		if !found {
			return "", nil, fmt.Errorf("selected daily lane %q is not current", selected)
		}
		choice = DailyChoice{ID: selected, Label: mission.BoardLaneLabel(lane)}
		state := strings.ToLower(strings.TrimSpace(lane.Status))
		if !explicit && state == "closed" {
			return selected, nil, nil
		}
		if state == "archived" {
			return "", nil, fmt.Errorf("daily terminal replay refuses archived lane %s because no durable archive transition is supported", selected)
		}
		if explicit && state == "open" {
			rejected, rejectionErr := currentReviewerRejectionAwaitingCorrection(caseRoot, pack, selected)
			if rejectionErr != nil {
				return "", nil, rejectionErr
			}
			if rejected {
				return selected, nil, nil
			}
		}
		return "", dailySelectedLaneBlockedAction(choice), nil
	}
	if _, err := runPublicStatusWithLease(caseRoot, pack, selected, lease); err != nil {
		if explicit {
			board, readErr := mission.ReadBoard(caseRoot)
			if readErr != nil {
				return "", nil, readErr
			}
			lane, found := mission.LookupBoardLane(board.Lanes, selected, false)
			if !found {
				return "", nil, fmt.Errorf("selected daily lane %q is not current", selected)
			}
			if strings.EqualFold(strings.TrimSpace(lane.Status), "open") {
				rejected, rejectionErr := currentReviewerRejectionAwaitingCorrection(caseRoot, pack, selected)
				if rejectionErr != nil {
					return "", nil, rejectionErr
				}
				if rejected {
					return selected, nil, nil
				}
			}
		}
		return "", dailySelectedLaneBlockedAction(choice), nil
	}
	return selected, nil, nil
}

func dailyStatusLaneChoices(status publicStatus) []DailyChoice {
	return publicCaseMissionLaneChoices(status.CaseMission)
}

func dailyChoiceForLane(choices []DailyChoice, selected string) (DailyChoice, bool) {
	for _, choice := range choices {
		if choice.ID == selected {
			return choice, true
		}
	}
	return DailyChoice{}, false
}

func ensureDailyStarted(caseRoot, pack string, result *DailyResult, selected ...string) error {
	return ensureDailyStartedWithLease(caseRoot, pack, result, nil, selected...)
}

func ensureDailyStartedWithLease(caseRoot, pack string, result *DailyResult, lease *projectexecution.Lease, selected ...string) error {
	lane := firstSelectedLane(selected)
	if lane == "" && result != nil {
		lane = strings.TrimSpace(result.Lane)
	}
	if lane == "" {
		inspection, err := missionintent.Inspect(caseRoot)
		if err != nil {
			return err
		}
		if inspection.Committed {
			lane = strings.TrimSpace(inspection.Identity.InitialLane)
		}
	}
	for range 4 {
		statusLane := lane
		if board, err := mission.ReadBoard(caseRoot); os.IsNotExist(err) {
			statusLane = ""
		} else if err != nil {
			return err
		} else if statusLane != "" {
			if _, found := mission.LookupBoardLane(board.Lanes, statusLane, false); !found {
				statusLane = ""
			}
		}
		status, err := runPublicStatusWithLease(caseRoot, pack, statusLane, lease)
		if err != nil {
			return err
		}
		request := currentDailyRequest(status)
		if request == nil || dailyReviewerOwnerRequest(status, lane) {
			return nil
		}
		if err := mission.ValidateMissionCommanderDriverRequest(*request); err != nil {
			return fmt.Errorf("daily current driver request omitted its valid typed invocation: %w", err)
		}
		switch request.Invocation.Command {
		case "overview":
			if !request.CommandExecutable || request.Blocked {
				return fmt.Errorf("daily overview request is not executable")
			}
			if err := runPublicDriverRequest(*request, nil); err != nil {
				return fmt.Errorf("consume daily public overview request: %w", err)
			}
			result.DriverSteps = append(result.DriverSteps, "overview")
		case "start":
			step, err := runPublicDriverStepWithLease(caseRoot, pack, lease, statusLane)
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

func currentDailyRequest(status publicStatus) *publicDriverRequest {
	if status.MissionControlRunbook == nil {
		return nil
	}
	return status.MissionControlRunbook.CurrentDriverRequest
}

func bindDailyResultCurrentDriverRequest(result *DailyResult) {
	if result == nil {
		return
	}
	for _, lane := range []string{"", strings.TrimSpace(result.Lane)} {
		owner, err := newDailySessionTransitionOwner(result.CaseRoot, result.Pack, lane)
		if err != nil {
			return
		}
		owner.bindResultCurrentDriverRequest(result)
		if result.CurrentDriverRequest != nil || lane != "" {
			return
		}
	}
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
	selected := ""
	if result != nil {
		selected = result.Lane
	}
	owner, err := newDailySessionTransitionOwner(caseRoot, pack, selected)
	if err != nil {
		return err
	}
	return owner.finish(result)
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
