package sessionhost

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"time"

	"github.com/shuiyu486/re-context-kits/internal/rekit/cli"
	rekitfs "github.com/shuiyu486/re-context-kits/internal/rekit/fs"
	"github.com/shuiyu486/re-context-kits/internal/rekit/memberexecution"
	"github.com/shuiyu486/re-context-kits/internal/rekit/mission"
)

const (
	defaultActor       = "rekit-claude-host"
	defaultHarness     = "claude-code-cli"
	defaultTimeout     = 30 * time.Minute
	defaultMaxAttempts = 3
)

type Options struct {
	Target                            string
	Pack                              string
	Actor                             string
	ClaudePath                        string
	ExpectedClaudeExecutableSHA256    string
	ExpectedClaudeExecutablePublisher string
	Model                             string
	Timeout                           time.Duration
	MaxAttempts                       int
	StopAfterMemberIntake             bool
}

type Result struct {
	SchemaVersion      int       `json:"schemaVersion"`
	Command            string    `json:"command"`
	CaseRoot           string    `json:"caseRoot"`
	Pack               string    `json:"pack,omitempty"`
	Actor              string    `json:"actor"`
	ClaudePath         string    `json:"claudePath"`
	SessionLaunches    int       `json:"sessionLaunches"`
	SessionCompletions int       `json:"sessionCompletions"`
	Replacements       int       `json:"replacements"`
	AppliedSteps       int       `json:"appliedSteps"`
	FinalMode          string    `json:"finalMode"`
	Sessions           []Session `json:"sessions,omitempty"`
	Boundary           []string  `json:"boundary"`
}

type Session struct {
	Started           bool     `json:"started"`
	Recovered         bool     `json:"recovered,omitempty"`
	AttemptGeneration int      `json:"attemptGeneration,omitempty"`
	RunLaunchOrdinal  int      `json:"runLaunchOrdinal,omitempty"`
	ReservationID     string   `json:"reservationId"`
	SessionID         string   `json:"sessionId,omitempty"`
	SessionKind       string   `json:"sessionKind"`
	Outcome           string   `json:"outcome"`
	ExitCode          int      `json:"exitCode,omitempty"`
	TimedOut          bool     `json:"timedOut,omitempty"`
	ResultSubtype     string   `json:"resultSubtype,omitempty"`
	ResultIsError     bool     `json:"resultIsError,omitempty"`
	DurationMillis    int64    `json:"durationMillis,omitempty"`
	PermissionDenials []any    `json:"permissionDenials,omitempty"`
	Diagnostics       []string `json:"diagnostics,omitempty"`
}

type currentStepPlan struct {
	Pack                          string                `json:"pack"`
	ExpectedCurrentStepPlanSHA256 string                `json:"expectedCurrentStepPlanSha256,omitempty"`
	MemberExecution               *memberexecution.Plan `json:"memberExecution,omitempty"`
	ReviewerStep                  *reviewerStep         `json:"reviewerStep,omitempty"`
	ExternalSessionStep           *externalSessionStep  `json:"externalSessionStep,omitempty"`
}

type memberExecutionStatus struct {
	State               string `json:"state,omitempty"`
	ReviewerPlanCommand string `json:"reviewerPlanCommand,omitempty"`
}

type statusPlan struct {
	MemberExecution       *memberExecutionStatus `json:"memberExecution,omitempty"`
	MissionControlRunbook *struct {
		Scope string `json:"scope,omitempty"`
	} `json:"missionControlRunbook,omitempty"`
}

type currentLoopPlan struct {
	ExpectedCurrentLoopPlanSHA256 string                  `json:"expectedCurrentLoopPlanSha256,omitempty"`
	InitialCurrentStep            *currentStepPlan        `json:"initialCurrentStep,omitempty"`
	Applied                       bool                    `json:"applied,omitempty"`
	AppliedSteps                  int                     `json:"appliedSteps,omitempty"`
	StopReason                    currentLoopStopReason   `json:"stopReason"`
	SegmentCheckpoint             *currentLoopCheckpoint  `json:"segmentCheckpoint,omitempty"`
	FinalStatus                   *currentLoopFinalStatus `json:"finalStatus,omitempty"`
}

type currentLoopStopReason struct {
	Code    string `json:"code,omitempty"`
	Phase   string `json:"phase,omitempty"`
	Message string `json:"message,omitempty"`
}

type currentLoopCheckpoint struct {
	State    string `json:"state,omitempty"`
	StopCode string `json:"stopCode,omitempty"`
	Ready    bool   `json:"ready,omitempty"`
}

type currentLoopFinalStatus struct {
	CurrentMode string `json:"currentMode,omitempty"`
}

type reviewerStep struct {
	ExternalHandoff *reviewerExternalHandoff `json:"externalHandoff,omitempty"`
}

type reviewerExternalHandoff struct {
	State                         string `json:"state"`
	RunLoopStepID                 string `json:"runLoopStepId"`
	DispatchPromptPath            string `json:"dispatchPromptPath,omitempty"`
	DispatchPromptSHA256          string `json:"dispatchPromptSha256,omitempty"`
	ReviewerResultInputPath       string `json:"reviewerResultInputPath,omitempty"`
	ReviewerResultSourcePath      string `json:"reviewerResultSourcePath,omitempty"`
	ReviewerDispatchID            string `json:"reviewerDispatchId,omitempty"`
	ReviewerDispatchReceiptPath   string `json:"reviewerDispatchReceiptPath,omitempty"`
	ReviewerDispatchReceiptSHA256 string `json:"reviewerDispatchReceiptSha256,omitempty"`
	ReviewerHarness               string `json:"reviewerHarness,omitempty"`
	ReviewerSession               string `json:"reviewerSession,omitempty"`
}

type externalSessionStep struct {
	Mode           string                                            `json:"mode"`
	Attempt        *attemptPlan                                      `json:"attempt,omitempty"`
	Dispatch       *dispatchPlan                                     `json:"dispatch,omitempty"`
	HarnessPackage *mission.CurrentLoopExternalSessionHarnessPackage `json:"harnessPackage,omitempty"`
}

type attemptPlan struct {
	AttemptSHA256 string `json:"attemptSha256"`
	Attempt       struct {
		Generation int `json:"generation"`
	} `json:"attempt"`
}

type dispatchPlan struct {
	AttemptSHA256 string `json:"attemptSha256"`
}

func Run(parent context.Context, opt Options) (Result, error) {
	caseRoot, err := canonicalCaseRoot(opt.Target)
	if err != nil {
		return Result{}, err
	}
	opt.Target = caseRoot
	opt.Actor = strings.TrimSpace(opt.Actor)
	if opt.Actor == "" {
		opt.Actor = defaultActor
	}
	if strings.ContainsAny(opt.Actor, "\r\n") {
		return Result{}, fmt.Errorf("host actor must be a single line")
	}
	if opt.Timeout <= 0 {
		opt.Timeout = defaultTimeout
	}
	if opt.MaxAttempts <= 0 {
		opt.MaxAttempts = defaultMaxAttempts
	}
	if opt.MaxAttempts > 256 {
		return Result{}, fmt.Errorf("max attempts cannot exceed 256")
	}
	claudePath, err := resolveClaudePath(opt.ClaudePath)
	if err != nil {
		return Result{}, err
	}
	opt.ClaudePath = claudePath
	result := Result{
		SchemaVersion: 1,
		Command:       "rekit-host",
		CaseRoot:      caseRoot,
		Pack:          strings.TrimSpace(opt.Pack),
		Actor:         opt.Actor,
		ClaudePath:    claudePath,
		Boundary: []string{
			"all LLM result bytes come from a real Claude Code process; the host generates only lifecycle metadata and submission bindings",
			"the host consumes run-current-step and does not replace deterministic runtime currentness, authorization, or strict intake",
			"submission is published only after every required real result artifact",
		},
	}

	reservationID := ""
	launchAttempts := 0
	for range 64 {
		preview, err := runCurrentStep(opt, nil, false)
		if err != nil {
			return result, err
		}
		if opt.StopAfterMemberIntake && memberIntakeComplete(opt.Target, preview) {
			result.FinalMode = "member-intake-ready"
			return result, nil
		}
		if !opt.StopAfterMemberIntake && preview.ReviewerStep == nil && preview.ExternalSessionStep == nil {
			planned, err := applyMemberReviewerPlan(opt)
			if err != nil {
				return result, err
			}
			if planned {
				result.AppliedSteps++
				continue
			}
		}
		if preview.ExternalSessionStep == nil {
			if preview.MemberExecution != nil {
				if err := applyMemberDispatchLoop(opt); err != nil {
					return result, err
				}
				result.AppliedSteps++
				continue
			}
			result.FinalMode = "reviewer-step"
			if reviewerDispatchReady(preview) {
				if launchAttempts >= opt.MaxAttempts {
					return result, fmt.Errorf("external session attempt limit reached after %d attempts", launchAttempts)
				}
				reservationID, err = newUUID()
				if err != nil {
					return result, err
				}
				args := []string{
					"-ReviewerHarness", defaultHarness,
					"-ReviewerSession", reservationID,
					"-Actor", opt.Actor,
				}
				plan, err := runCurrentStep(opt, args, false)
				if err != nil {
					return result, err
				}
				if reviewerDispatchReady(plan) || strings.TrimSpace(plan.ExpectedCurrentStepPlanSHA256) == "" {
					return result, fmt.Errorf("reviewer dispatch preview omitted the hash-bound acceptance plan")
				}
				if err := applyCurrentStep(opt, plan, args); err != nil {
					return result, err
				}
				result.AppliedSteps++
				continue
			}
			if reviewerSessionPending(preview) {
				handoff := *preview.ReviewerStep.ExternalHandoff
				pkg, receipt, err := reviewerClaudePackage(opt.Target, handoff)
				if err != nil {
					return result, err
				}
				run, recovered, err := recoverClaudeRunForCase(opt.Target, opt, pkg)
				if err != nil {
					return result, err
				}
				attemptGeneration := pkg.Launch.Attempt.Generation
				launchOrdinal := 0
				if !recovered {
					if launchAttempts >= opt.MaxAttempts {
						return result, fmt.Errorf("external session attempt limit reached after %d attempts", launchAttempts)
					}
					launchAttempts++
					launchOrdinal = launchAttempts
					run = runClaude(parent, opt, pkg, receipt.ReviewerSession, nil)
					if run.started {
						result.SessionLaunches++
					}
					if run.success() {
						if err := persistClaudeRecoveryForCase(opt.Target, opt, pkg, run); err != nil {
							return result, fmt.Errorf("persist Claude reviewer structured output recovery: %w", err)
						}
					}
				}
				data, failure, resultErr := reviewerResultBytes(run)
				if resultErr != nil {
					failure = resultErr.Error()
				}
				if len(data) == 0 {
					outcome := "failed"
					if launchAttempts < opt.MaxAttempts {
						outcome = "replacement-requested"
						result.Replacements++
					}
					session := sessionResult(run, attemptGeneration, launchOrdinal, receipt.ReviewerSession, "reviewer", outcome)
					if resultErr != nil {
						session.Diagnostics = append(session.Diagnostics, truncate(oneLine(resultErr.Error()), 1024))
					}
					result.Sessions = append(result.Sessions, session)
					if err := applyReviewerFailure(opt, failure); err != nil {
						return result, err
					}
					result.AppliedSteps++
					reservationID = ""
					continue
				}
				sourcePath, err := publishReviewerSource(opt.Target, receipt.DispatchID, data)
				if err != nil {
					return result, err
				}
				args := []string{"-ReviewerResultInputSourcePath", sourcePath, "-Actor", opt.Actor}
				plan, err := runCurrentStep(opt, args, false)
				if err != nil {
					return result, err
				}
				if strings.TrimSpace(plan.ExpectedCurrentStepPlanSHA256) == "" {
					return result, fmt.Errorf("reviewer result save preview omitted the hash-bound plan")
				}
				if err := applyCurrentStep(opt, plan, args); err != nil {
					return result, err
				}
				result.AppliedSteps++
				result.SessionCompletions++
				if err := removeClaudeRecoveryForCase(opt.Target, opt, pkg); err != nil {
					return result, err
				}
				outcome := "returned"
				if recovered {
					outcome = "returned-recovered"
				}
				result.Sessions = append(result.Sessions, sessionResult(run, attemptGeneration, launchOrdinal, receipt.ReviewerSession, "reviewer", outcome))
				reservationID = ""
				continue
			}
			if reviewerActorStep(preview) {
				args := []string{"-Actor", opt.Actor}
				plan, err := runCurrentStep(opt, args, false)
				if err != nil {
					return result, err
				}
				if strings.TrimSpace(plan.ExpectedCurrentStepPlanSHA256) == "" {
					return result, fmt.Errorf("reviewer %s preview omitted the hash-bound plan", preview.ReviewerStep.ExternalHandoff.RunLoopStepID)
				}
				if err := applyCurrentStep(opt, plan, args); err != nil {
					return result, err
				}
				result.AppliedSteps++
				continue
			}
			result.FinalMode = "no-external-session"
			return result, nil
		}
		step := preview.ExternalSessionStep
		result.FinalMode = step.Mode
		switch step.Mode {
		case "attempt-input":
			if launchAttempts >= opt.MaxAttempts {
				return result, fmt.Errorf("external session attempt limit reached after %d attempts", launchAttempts)
			}
			if reservationID == "" {
				reservationID, err = newUUID()
				if err != nil {
					return result, err
				}
			}
			supersedes := ""
			if step.HarnessPackage != nil && step.HarnessPackage.Launch != nil {
				supersedes = step.HarnessPackage.Launch.Attempt.AttemptSHA256
			}
			args := attemptArgs(opt.Actor, reservationID, supersedes)
			plan, err := runCurrentStep(opt, args, false)
			if err != nil {
				return result, err
			}
			if plan.ExternalSessionStep == nil || plan.ExternalSessionStep.Attempt == nil {
				return result, fmt.Errorf("attempt preview omitted the exact attempt plan")
			}
			if err := applyCurrentStep(opt, plan, args); err != nil {
				return result, err
			}
			result.AppliedSteps++
			if plan.ExternalSessionStep.Attempt.Attempt.Generation > 1 {
				result.Replacements++
			}
		case "attempt-publication-recovery", "dispatch-claim", "launch-failed", "launch-accepted", "result-turn":
			if err := applyCurrentStep(opt, preview, nil); err != nil {
				return result, err
			}
			result.AppliedSteps++
		case "dispatch-claim-input":
			args := []string{
				"-ExternalSessionActor", opt.Actor,
				"-ExternalSessionObservedAt", nowRFC3339Nano(),
			}
			plan, err := runCurrentStep(opt, args, false)
			if err != nil {
				return result, err
			}
			if err := applyCurrentStep(opt, plan, args); err != nil {
				return result, err
			}
			result.AppliedSteps++
		case "launch-receipt-input":
			if launchAttempts >= opt.MaxAttempts {
				return result, fmt.Errorf("external session attempt limit reached after %d attempts", launchAttempts)
			}
			if step.HarnessPackage == nil || step.HarnessPackage.Launch == nil || !step.HarnessPackage.Launch.Ready {
				return result, fmt.Errorf("external session launch package is not ready")
			}
			launch := step.HarnessPackage.Launch
			if reservationID == "" {
				reservationID = launch.Attempt.Session
			}
			launchAttempts++
			accepted := false
			run := runClaude(parent, opt, *step.HarnessPackage, reservationID, func() error {
				args := []string{
					"-ExternalSessionLaunchOutcome", "accepted",
					"-ExternalSessionActor", opt.Actor,
					"-ExternalSessionObservedAt", nowRFC3339Nano(),
					"-ExternalSessionHarness", defaultHarness,
					"-ExternalSessionId", reservationID,
				}
				plan, err := runCurrentStep(opt, args, false)
				if err != nil {
					return err
				}
				if err := applyCurrentStep(opt, plan, args); err != nil {
					return err
				}
				result.AppliedSteps++
				accepted = true
				return nil
			})
			if run.started {
				result.SessionLaunches++
			}
			if run.startCallbackErr != nil {
				result.Sessions = append(result.Sessions, sessionResult(run, launch.Attempt.Generation, launchAttempts, reservationID, step.HarnessPackage.SessionKind, "host-failed"))
				return result, fmt.Errorf("record accepted Claude launch: %w", run.startCallbackErr)
			}
			if run.spawnErr != nil {
				reason := run.failureReason()
				args := []string{
					"-ExternalSessionLaunchOutcome", "failed",
					"-ExternalSessionActor", opt.Actor,
					"-ExternalSessionObservedAt", nowRFC3339Nano(),
					"-ExternalSessionLaunchReason", reason,
				}
				plan, planErr := runCurrentStep(opt, args, false)
				if planErr != nil {
					return result, errors.Join(run.spawnErr, planErr)
				}
				if applyErr := applyCurrentStep(opt, plan, args); applyErr != nil {
					return result, errors.Join(run.spawnErr, applyErr)
				}
				result.AppliedSteps++
				result.Sessions = append(result.Sessions, sessionResult(run, launch.Attempt.Generation, launchAttempts, reservationID, step.HarnessPackage.SessionKind, "launch-failed"))
				reservationID = ""
				continue
			}
			if !accepted {
				return result, fmt.Errorf("Claude process started without an accepted launch receipt")
			}
			if run.success() {
				if err := persistClaudeRecoveryForCase(opt.Target, opt, *step.HarnessPackage, run); err != nil {
					return result, fmt.Errorf("persist Claude structured output recovery: %w", err)
				}
			}
			fresh, err := runCurrentStep(opt, nil, false)
			if err != nil {
				return result, err
			}
			if !run.success() && launchAttempts < opt.MaxAttempts {
				result.Sessions = append(result.Sessions, sessionResult(run, launch.Attempt.Generation, launchAttempts, reservationID, step.HarnessPackage.SessionKind, "replacement-requested"))
				reservationID, err = applyReplacementAttempt(opt, fresh, opt.Actor)
				if err != nil {
					return result, err
				}
				result.AppliedSteps++
				result.Replacements++
				continue
			}
			outcome, publishErr := publishClaudeResult(opt, fresh, run)
			if publishErr != nil && launchAttempts < opt.MaxAttempts {
				result.Sessions = append(result.Sessions, sessionResult(run, launch.Attempt.Generation, launchAttempts, reservationID, step.HarnessPackage.SessionKind, "invalid-result-replacement"))
				reservationID, err = applyReplacementAttempt(opt, fresh, opt.Actor)
				if err != nil {
					return result, errors.Join(publishErr, err)
				}
				result.AppliedSteps++
				result.Replacements++
				continue
			}
			result.Sessions = append(result.Sessions, sessionResult(run, launch.Attempt.Generation, launchAttempts, reservationID, step.HarnessPackage.SessionKind, outcome))
			if publishErr != nil {
				return result, publishErr
			}
			if err := removeClaudeRecoveryForCase(opt.Target, opt, *step.HarnessPackage); err != nil {
				return result, err
			}
			if run.success() {
				result.SessionCompletions++
			}
			reservationID = ""
		case "running-handoff":
			if step.HarnessPackage == nil || step.HarnessPackage.Launch == nil {
				return result, fmt.Errorf("external session running handoff omitted recovery bindings")
			}
			run, recovered, err := recoverClaudeRunForCase(opt.Target, opt, *step.HarnessPackage)
			if err != nil {
				return result, err
			}
			if !recovered {
				return result, fmt.Errorf("external session is already running and has no exact host-owned structured output recovery")
			}
			outcome, err := publishClaudeResult(opt, preview, run)
			if err != nil {
				return result, err
			}
			if err := removeClaudeRecoveryForCase(opt.Target, opt, *step.HarnessPackage); err != nil {
				return result, err
			}
			result.Sessions = append(result.Sessions, sessionResult(run, step.HarnessPackage.Launch.Attempt.Generation, 0, step.HarnessPackage.Launch.Attempt.Session, step.HarnessPackage.SessionKind, outcome+"-recovered"))
			result.SessionCompletions++
			reservationID = ""
		default:
			return result, fmt.Errorf("unsupported external session host mode %q", step.Mode)
		}
	}
	return result, fmt.Errorf("external session host exceeded transition limit")
}

func applyMemberReviewerPlan(opt Options) (bool, error) {
	status, err := runStatus(opt)
	if err != nil {
		return false, err
	}
	if status.MissionControlRunbook == nil || status.MissionControlRunbook.Scope != "case" || status.MemberExecution == nil || status.MemberExecution.State != "intake-ready" || strings.TrimSpace(status.MemberExecution.ReviewerPlanCommand) == "" {
		return false, nil
	}
	args, err := cli.SplitPublicCommand(status.MemberExecution.ReviewerPlanCommand)
	if err != nil {
		return false, fmt.Errorf("decode member reviewer plan command: %w", err)
	}
	var out bytes.Buffer
	if err := cli.Run(args, &out); err != nil {
		return false, fmt.Errorf("apply member reviewer plan command: %w", err)
	}
	return true, nil
}

func runStatus(opt Options) (statusPlan, error) {
	args := []string{"-Command", "status", "-Target", opt.Target}
	if strings.TrimSpace(opt.Pack) != "" {
		args = append(args, "-Pack", opt.Pack)
	}
	args = append(args, "-Format", "json")
	var out bytes.Buffer
	if err := cli.Run(args, &out); err != nil {
		return statusPlan{}, err
	}
	var status statusPlan
	dec := json.NewDecoder(bytes.NewReader(out.Bytes()))
	if err := dec.Decode(&status); err != nil {
		return statusPlan{}, fmt.Errorf("decode status result: %w", err)
	}
	var trailing any
	if err := dec.Decode(&trailing); err != io.EOF {
		return statusPlan{}, fmt.Errorf("status returned trailing JSON")
	}
	return status, nil
}

func memberIntakeComplete(caseRoot string, plan currentStepPlan) bool {
	if plan.ExternalSessionStep != nil || plan.ReviewerStep != nil {
		return false
	}
	board, err := mission.ReadBoard(caseRoot)
	if err != nil {
		return false
	}
	for _, lane := range mission.OpenBoardLanes(board.Lanes) {
		if strings.TrimSpace(lane.CurrentExecutor) == "" || lane.ExecutorGeneration < 1 {
			continue
		}
		latest, ok, err := memberexecution.Latest(caseRoot, lane.ID)
		if err == nil && ok && latest.State == "intake-ready" && latest.Owner.Executor == lane.CurrentExecutor && latest.Owner.ExecutorGeneration == lane.ExecutorGeneration {
			return true
		}
	}
	return false
}

func reviewerDispatchReady(plan currentStepPlan) bool {
	return plan.ReviewerStep != nil && plan.ReviewerStep.ExternalHandoff != nil &&
		plan.ReviewerStep.ExternalHandoff.State == "ready-for-reviewer-dispatch" &&
		plan.ReviewerStep.ExternalHandoff.RunLoopStepID == "spawn-reviewer"
}

func reviewerSessionPending(plan currentStepPlan) bool {
	return plan.ReviewerStep != nil && plan.ReviewerStep.ExternalHandoff != nil &&
		plan.ReviewerStep.ExternalHandoff.State == "reviewer-session-running-unknown" &&
		plan.ReviewerStep.ExternalHandoff.RunLoopStepID == "save-result-input" &&
		strings.TrimSpace(plan.ReviewerStep.ExternalHandoff.ReviewerSession) != ""
}

func reviewerActorStep(plan currentStepPlan) bool {
	if plan.ReviewerStep == nil || plan.ReviewerStep.ExternalHandoff == nil {
		return false
	}
	switch plan.ReviewerStep.ExternalHandoff.RunLoopStepID {
	case "verify-prompt", "record-completion", "source-capture", "stage-candidate", "collect-result", "intake-results":
		return true
	default:
		return false
	}
}

func applyReviewerFailure(opt Options, reason string) error {
	reason = truncate(oneLine(strings.TrimSpace(reason)), 1024)
	if reason == "" {
		reason = "Claude reviewer session failed"
	}
	args := []string{"-ReviewerOutcome", "failed", "-ReviewerExitStatus", reason, "-Actor", opt.Actor}
	plan, err := runCurrentStep(opt, args, false)
	if err != nil {
		return err
	}
	if strings.TrimSpace(plan.ExpectedCurrentStepPlanSHA256) == "" {
		return fmt.Errorf("reviewer failure preview omitted the hash-bound plan")
	}
	return applyCurrentStep(opt, plan, args)
}

func publishReviewerSource(caseRoot, dispatchID string, data []byte) (string, error) {
	dispatchID = strings.TrimSpace(dispatchID)
	if dispatchID == "" {
		return "", fmt.Errorf("reviewer source requires a durable dispatch id")
	}
	rel := filepath.ToSlash(filepath.Join(".rekit", "session-host", "reviewer-results", dispatchID+".json"))
	if _, err := rekitfs.WriteExclusiveRegularFileAnchored(caseRoot, rel, "Claude reviewer source", data); err != nil {
		return "", err
	}
	return filepath.Join(caseRoot, filepath.FromSlash(rel)), nil
}

func runCurrentStep(opt Options, extra []string, apply bool) (currentStepPlan, error) {
	args := []string{"-Command", "run-current-step", "-Target", opt.Target}
	if strings.TrimSpace(opt.Pack) != "" {
		args = append(args, "-Pack", opt.Pack)
	}
	args = append(args, extra...)
	if apply {
		args = append(args, "-Apply")
	} else {
		args = append(args, "-WhatIf")
	}
	args = append(args, "-Format", "json")
	var out bytes.Buffer
	if err := cli.Run(args, &out); err != nil {
		return currentStepPlan{}, err
	}
	var plan currentStepPlan
	dec := json.NewDecoder(bytes.NewReader(out.Bytes()))
	if err := dec.Decode(&plan); err != nil {
		return currentStepPlan{}, fmt.Errorf("decode run-current-step result: %w", err)
	}
	var trailing any
	if err := dec.Decode(&trailing); err != io.EOF {
		return currentStepPlan{}, fmt.Errorf("run-current-step returned trailing JSON")
	}
	return plan, nil
}

func applyMemberDispatchLoop(opt Options) error {
	preview, err := runCurrentLoop(opt, false, "", "")
	if err != nil {
		return err
	}
	if strings.TrimSpace(preview.ExpectedCurrentLoopPlanSHA256) == "" {
		return fmt.Errorf("member dispatch current-loop preview omitted the hash-bound plan")
	}
	memberPlanSHA256 := ""
	if preview.InitialCurrentStep != nil && preview.InitialCurrentStep.MemberExecution != nil {
		memberPlanSHA256 = preview.InitialCurrentStep.MemberExecution.ExpectedPlanSHA256
	}
	if strings.TrimSpace(memberPlanSHA256) == "" {
		return fmt.Errorf("member dispatch current-loop preview omitted the nested member execution plan")
	}
	applied, err := runCurrentLoop(opt, true, preview.ExpectedCurrentLoopPlanSHA256, memberPlanSHA256)
	if err != nil {
		return err
	}
	if !applied.Applied {
		return fmt.Errorf(
			"member dispatch current-loop Apply did not record a durable step: stop=%s phase=%s message=%s appliedSteps=%d checkpoint=%s finalMode=%s",
			strings.TrimSpace(applied.StopReason.Code),
			strings.TrimSpace(applied.StopReason.Phase),
			strings.TrimSpace(applied.StopReason.Message),
			applied.AppliedSteps,
			currentLoopCheckpointSummary(applied.SegmentCheckpoint),
			currentLoopFinalMode(applied.FinalStatus),
		)
	}
	return nil
}

func currentLoopCheckpointSummary(checkpoint *currentLoopCheckpoint) string {
	if checkpoint == nil {
		return "<none>"
	}
	return fmt.Sprintf("%s/%s/ready=%t", strings.TrimSpace(checkpoint.State), strings.TrimSpace(checkpoint.StopCode), checkpoint.Ready)
}

func currentLoopFinalMode(status *currentLoopFinalStatus) string {
	if status == nil || strings.TrimSpace(status.CurrentMode) == "" {
		return "<none>"
	}
	return strings.TrimSpace(status.CurrentMode)
}

func runCurrentLoop(opt Options, apply bool, expected, memberPlanSHA256 string) (currentLoopPlan, error) {
	args := []string{"-Command", "run-current-loop", "-Target", opt.Target}
	if strings.TrimSpace(opt.Pack) != "" {
		args = append(args, "-Pack", opt.Pack)
	}
	args = append(args, "-MaxSteps", "2")
	if apply {
		args = append(args, "-ExpectedMemberExecutionPlanSha256", memberPlanSHA256, "-ExpectedCurrentLoopPlanSha256", expected, "-Apply")
	} else {
		args = append(args, "-WhatIf")
	}
	args = append(args, "-Format", "json")
	var out bytes.Buffer
	if err := cli.Run(args, &out); err != nil {
		return currentLoopPlan{}, err
	}
	var plan currentLoopPlan
	dec := json.NewDecoder(bytes.NewReader(out.Bytes()))
	if err := dec.Decode(&plan); err != nil {
		return currentLoopPlan{}, fmt.Errorf("decode run-current-loop result: %w", err)
	}
	var trailing any
	if err := dec.Decode(&trailing); err != io.EOF {
		return currentLoopPlan{}, fmt.Errorf("run-current-loop returned trailing JSON")
	}
	return plan, nil
}

func applyCurrentStep(opt Options, plan currentStepPlan, transitionArgs []string) error {
	if strings.TrimSpace(plan.ExpectedCurrentStepPlanSHA256) == "" {
		return fmt.Errorf("current external session step has no deterministic apply hash")
	}
	args := append([]string{}, transitionArgs...)
	args = append(args, "-ExpectedCurrentStepPlanSha256", plan.ExpectedCurrentStepPlanSHA256)
	_, err := runCurrentStep(opt, args, true)
	return err
}

func applyReplacementAttempt(opt Options, running currentStepPlan, actor string) (string, error) {
	step := running.ExternalSessionStep
	if step == nil || step.Mode != "running-handoff" || step.HarnessPackage == nil || step.HarnessPackage.Launch == nil {
		return "", fmt.Errorf("replacement requires the current accepted running handoff")
	}
	currentAttemptSHA := step.HarnessPackage.Launch.Attempt.AttemptSHA256
	if currentAttemptSHA == "" {
		return "", fmt.Errorf("replacement handoff omitted current attempt sha256")
	}
	session, err := newUUID()
	if err != nil {
		return "", err
	}
	args := attemptArgs(actor, session, currentAttemptSHA)
	plan, err := runCurrentStep(opt, args, false)
	if err != nil {
		return "", err
	}
	if plan.ExternalSessionStep == nil || plan.ExternalSessionStep.Mode != "replacement-attempt" || plan.ExternalSessionStep.Attempt == nil {
		return "", fmt.Errorf("replacement preview omitted the exact next attempt")
	}
	if err := applyCurrentStep(opt, plan, args); err != nil {
		return "", err
	}
	return session, nil
}

func attemptArgs(actor, session, supersedes string) []string {
	args := []string{
		"-ExternalSessionHarness", defaultHarness,
		"-ExternalSessionId", session,
		"-ExternalSessionActor", actor,
		"-ExternalSessionStartedAt", nowRFC3339Nano(),
	}
	if supersedes != "" {
		args = append(args, "-ExpectedExternalSessionAttemptSha256", supersedes)
	}
	return args
}

func canonicalCaseRoot(value string) (string, error) {
	value = strings.TrimSpace(value)
	if value == "" {
		return "", fmt.Errorf("host requires -Target <attached case>")
	}
	full, err := filepath.Abs(value)
	if err != nil {
		return "", err
	}
	info, err := os.Lstat(full)
	if err != nil || !info.IsDir() || info.Mode()&os.ModeSymlink != 0 {
		return "", fmt.Errorf("host target must be an existing non-symlink directory: %s", full)
	}
	return filepath.Clean(full), nil
}

func resolveClaudePath(value string) (string, error) {
	value = strings.TrimSpace(value)
	if value == "" {
		value = "claude"
	}
	var resolved string
	var err error
	if runtime.GOOS == "windows" && filepath.Ext(value) == "" {
		resolved, err = exec.LookPath(value + ".exe")
	}
	if resolved == "" {
		resolved, err = exec.LookPath(value)
	}
	if err != nil {
		return "", fmt.Errorf("resolve Claude Code executable: %w", err)
	}
	full, err := filepath.Abs(resolved)
	if err != nil {
		return "", err
	}
	full = filepath.Clean(full)
	if runtime.GOOS == "windows" {
		switch strings.ToLower(filepath.Ext(full)) {
		case ".cmd", ".bat":
			return "", fmt.Errorf("Claude Code command scripts are not directly executable; provide the native claude.exe path: %s", full)
		}
	}
	return full, nil
}

func nowRFC3339Nano() string {
	return time.Now().UTC().Format(time.RFC3339Nano)
}
