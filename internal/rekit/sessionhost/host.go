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
	"github.com/shuiyu486/re-context-kits/internal/rekit/instance"
	"github.com/shuiyu486/re-context-kits/internal/rekit/memberexecution"
	"github.com/shuiyu486/re-context-kits/internal/rekit/mission"
	"github.com/shuiyu486/re-context-kits/internal/rekit/subagents"
	"github.com/shuiyu486/re-context-kits/internal/rekit/workstream"
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
	SelectedLane                      string
	Actor                             string
	ClaudePath                        string
	ExpectedClaudeExecutableSHA256    string
	ExpectedClaudeExecutablePublisher string
	Model                             string
	Timeout                           time.Duration
	MaxAttempts                       int
	StopAfterMemberIntake             bool
	requireDailyClaudeTrust           bool
	reviewerBinding                   *reviewerBinding
}

type reviewerBinding struct {
	PacketID             string
	PacketPath           string
	PacketSHA256         string
	Lane                 string
	ShardID              string
	DispatchPromptPath   string
	DispatchPromptSHA256 string
}

type Result struct {
	SchemaVersion      int               `json:"schemaVersion"`
	Command            string            `json:"command"`
	CaseRoot           string            `json:"caseRoot"`
	Pack               string            `json:"pack,omitempty"`
	Actor              string            `json:"actor"`
	ClaudePath         string            `json:"claudePath"`
	SessionLaunches    int               `json:"sessionLaunches"`
	SessionCompletions int               `json:"sessionCompletions"`
	Replacements       int               `json:"replacements"`
	AppliedSteps       int               `json:"appliedSteps"`
	FinalMode          string            `json:"finalMode"`
	Sessions           []Session         `json:"sessions,omitempty"`
	Failure            *FailureDiagnosis `json:"failure,omitempty"`
	Boundary           []string          `json:"boundary"`
}

type Session struct {
	Started           bool              `json:"started"`
	Recovered         bool              `json:"recovered,omitempty"`
	AttemptGeneration int               `json:"attemptGeneration,omitempty"`
	RunLaunchOrdinal  int               `json:"runLaunchOrdinal,omitempty"`
	ReservationID     string            `json:"reservationId"`
	SessionID         string            `json:"sessionId,omitempty"`
	SessionKind       string            `json:"sessionKind"`
	Outcome           string            `json:"outcome"`
	ExitCode          int               `json:"exitCode,omitempty"`
	TimedOut          bool              `json:"timedOut,omitempty"`
	ResultSubtype     string            `json:"resultSubtype,omitempty"`
	ResultIsError     bool              `json:"resultIsError,omitempty"`
	DurationMillis    int64             `json:"durationMillis,omitempty"`
	PermissionDenials []any             `json:"permissionDenials,omitempty"`
	Failure           *FailureDiagnosis `json:"failure,omitempty"`
	Diagnostics       []string          `json:"diagnostics,omitempty"`
}

type currentStepPlan struct {
	Pack                          string                                `json:"pack"`
	ExpectedCurrentStepPlanSHA256 string                                `json:"expectedCurrentStepPlanSha256,omitempty"`
	CurrentDriverRequest          mission.MissionCommanderDriverRequest `json:"currentDriverRequest"`
	MemberExecution               *memberexecution.Plan                 `json:"memberExecution,omitempty"`
	ReviewerStep                  *reviewerStep                         `json:"reviewerStep,omitempty"`
	ExternalSessionStep           *externalSessionStep                  `json:"externalSessionStep,omitempty"`
}

func currentStepIsEvidenceReviewStop(plan currentStepPlan) bool {
	return plan.MemberExecution == nil &&
		plan.ReviewerStep == nil &&
		plan.ExternalSessionStep == nil &&
		strings.HasPrefix(strings.TrimSpace(plan.CurrentDriverRequest.Source), "executionEvidenceReview")
}

type memberExecutionStatus struct {
	State               string `json:"state,omitempty"`
	Lane                string `json:"lane,omitempty"`
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

type boundReviewerStepPlan struct {
	PacketID                       string                          `json:"packetId"`
	PacketPath                     string                          `json:"packetPath"`
	TargetLane                     string                          `json:"targetLane"`
	ShardID                        string                          `json:"shardId"`
	ExpectedReviewerStepPlanSHA256 string                          `json:"expectedReviewerStepPlanSha256,omitempty"`
	ReviewerResultSnapshot         *reviewerResultSnapshotIdentity `json:"reviewerResultSnapshot,omitempty"`
	ExternalHandoff                *reviewerExternalHandoff        `json:"externalHandoff,omitempty"`
}

type reviewerResultSnapshotIdentity struct {
	Path   string `json:"path"`
	SHA256 string `json:"sha256"`
	Bytes  int64  `json:"bytes"`
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

func Run(parent context.Context, opt Options) (result Result, retErr error) {
	caseRoot, err := canonicalCaseRoot(opt.Target)
	if err != nil {
		return Result{}, err
	}
	opt.Target = caseRoot
	opt.Pack = strings.TrimSpace(opt.Pack)
	if opt.Pack == "" && instance.LooksLikeCase(caseRoot) {
		inst, err := instance.Read(caseRoot)
		if err != nil {
			return Result{}, err
		}
		opt.Pack = strings.TrimSpace(inst.TemplatePack)
	}
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
	result = Result{
		SchemaVersion: 1,
		Command:       "rekit-host",
		CaseRoot:      caseRoot,
		Pack:          opt.Pack,
		Actor:         opt.Actor,
		ClaudePath:    strings.TrimSpace(opt.ClaudePath),
		Boundary: []string{
			"all LLM result bytes come from a real Claude Code process; the host generates only lifecycle metadata and submission bindings",
			"the host consumes the canonical current-step route or an explicitly bound run-reviewer-step route and does not replace deterministic runtime currentness, authorization, or strict intake",
			"submission is published only after every required real result artifact",
		},
	}
	defer func() {
		if retErr != nil && result.Failure == nil {
			result.Failure = diagnosisForError(retErr, len(result.Sessions), opt.MaxAttempts, result.AppliedSteps)
		}
		if result.Failure == nil && len(result.Sessions) > 0 {
			result.Failure = result.Sessions[len(result.Sessions)-1].Failure
		}
	}()
	if opt.MaxAttempts > 256 {
		return result, hostError("claude-attempt-limit-invalid", "attempt-control", "none", "Set -max-attempts to a value from 1 through 256, then rerun the same host command.", false, fmt.Errorf("max attempts cannot exceed 256"))
	}
	if !opt.StopAfterMemberIntake {
		rejected, err := currentReviewerRejectionAwaitingCorrection(opt.Target, opt.Pack, opt.SelectedLane)
		if err != nil {
			return result, err
		}
		if rejected {
			result.FinalMode = "reviewer-rejected-awaiting-correction"
			return result, nil
		}
	}
	var preview currentStepPlan
	if opt.requireDailyClaudeTrust || instance.LooksLikeCase(caseRoot) {
		preview, err = runCurrentStep(opt, nil, false)
		if err != nil {
			return result, err
		}
		if currentStepIsEvidenceReviewStop(preview) {
			result.FinalMode = DailyActionReadyForEvidenceReview
			return result, nil
		}
	}
	if opt.requireDailyClaudeTrust {
		dailyOpt := DailyOptions{
			ClaudePath:                        opt.ClaudePath,
			ExpectedClaudeExecutableSHA256:    opt.ExpectedClaudeExecutableSHA256,
			ExpectedClaudeExecutablePublisher: opt.ExpectedClaudeExecutablePublisher,
		}
		if err := bindDailyTrustedClaude(&dailyOpt); err != nil {
			return result, err
		}
		opt.ClaudePath = dailyOpt.ClaudePath
		opt.ExpectedClaudeExecutableSHA256 = dailyOpt.ExpectedClaudeExecutableSHA256
		opt.ExpectedClaudeExecutablePublisher = dailyOpt.ExpectedClaudeExecutablePublisher
	}
	claudePath, err := resolveClaudePath(opt.ClaudePath)
	if err != nil {
		return result, hostError("claude-executable-unavailable", "executable-resolution", "none", "Install or repair the Claude Code executable, then rerun the same host command.", false, err)
	}
	opt.ClaudePath = claudePath
	result.ClaudePath = claudePath
	var control *supervisionOwnerLease
	if trustedRecoveryProvenance(opt) {
		control, err = acquireSupervisionControl(parent, opt.Target)
		if err != nil {
			return result, err
		}
		defer control.Close()
	}

	reservationID := ""
	launchAttempts := 0
	for range 64 {
		if !opt.StopAfterMemberIntake {
			rejected, err := currentReviewerRejectionAwaitingCorrection(opt.Target, opt.Pack, opt.SelectedLane)
			if err != nil {
				return result, err
			}
			if rejected {
				result.FinalMode = "reviewer-rejected-awaiting-correction"
				return result, nil
			}
		}
		preview, err = runCurrentStep(opt, nil, false)
		if err != nil {
			return result, err
		}
		if selected := strings.TrimSpace(opt.SelectedLane); selected != "" && preview.MemberExecution != nil && strings.TrimSpace(preview.MemberExecution.Owner.Lane) != selected {
			return result, fmt.Errorf("selected lane %q resolved member execution for lane %q", selected, preview.MemberExecution.Owner.Lane)
		}
		if currentStepIsEvidenceReviewStop(preview) {
			result.FinalMode = DailyActionReadyForEvidenceReview
			return result, nil
		}
		if opt.StopAfterMemberIntake && preview.ReviewerStep != nil {
			result.FinalMode = "reviewer-ready"
			return result, nil
		}
		if preview.ReviewerStep == nil && preview.ExternalSessionStep == nil {
			status, err := runStatus(opt)
			if err != nil {
				return result, err
			}
			if status.MemberExecution != nil && status.MemberExecution.State == "reviewer-rejected-awaiting-correction" {
				result.FinalMode = "reviewer-rejected-awaiting-correction"
				return result, nil
			}
			planned, err := applyMemberReviewerPlanFromStatus(opt, status)
			if err != nil {
				return result, err
			}
			if planned {
				result.AppliedSteps++
				continue
			}
			if opt.StopAfterMemberIntake && memberIntakeComplete(opt.Target, opt.SelectedLane, preview) {
				result.FinalMode = "member-intake-ready"
				return result, nil
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
					launched := false
					run, launched, err = supervisedClaudeRun(parent, opt, pkg, receipt.ReviewerSession, nil)
					if err != nil {
						return result, err
					}
					if launched {
						launchAttempts++
						launchOrdinal = launchAttempts
					}
					if run.started {
						result.SessionLaunches++
					}
					if run.success() {
						if err := validateClaudeStructuredResult(pkg, run); err != nil {
							run.failureDetail = err.Error()
						} else if err := persistClaudeRecoveryForCase(opt.Target, opt, pkg, run); err != nil {
							return result, fmt.Errorf("persist Claude reviewer structured output recovery: %w", err)
						}
					}
				}
				data, failure, resultErr := reviewerResultBytes(pkg, run)
				if resultErr != nil {
					run.failureDetail = resultErr.Error()
					failure = resultErr.Error()
				}
				if len(data) == 0 {
					outcome := "failed"
					if launchAttempts < opt.MaxAttempts {
						outcome = "replacement-requested"
						if run.failureDetail != "" {
							outcome = "invalid-result-replacement"
						}
						result.Replacements++
					}
					session := sessionResult(run, attemptGeneration, launchOrdinal, receipt.ReviewerSession, "reviewer", outcome, opt.MaxAttempts)
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
				snapshotPath := filepath.Join(opt.Target, ".rekit", "session-host", "reviewer-results", receipt.DispatchID+".json")
				snapshot := &subagents.ReviewerResultInputSnapshot{
					Path: snapshotPath, SHA256: bytesSHA256(data), Bytes: int64(len(data)), Data: append([]byte{}, data...),
				}
				args := []string{"-ReviewerResultInputSourcePath", snapshotPath, "-Actor", opt.Actor}
				plan, err := runCurrentStepWithReviewerSnapshot(opt, args, false, snapshot)
				if err != nil {
					return result, err
				}
				if strings.TrimSpace(plan.ExpectedCurrentStepPlanSHA256) == "" {
					return result, fmt.Errorf("reviewer result save preview omitted the hash-bound plan")
				}
				if err := applyCurrentStepWithReviewerSnapshot(opt, plan, args, snapshot); err != nil {
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
				result.Sessions = append(result.Sessions, sessionResult(run, attemptGeneration, launchOrdinal, receipt.ReviewerSession, "reviewer", outcome, opt.MaxAttempts))
				reservationID = ""
				if err := observeSupervisionCut("submission"); err != nil {
					return result, err
				}
				continue
			}
			if reviewerActorStep(preview) {
				reviewerStepID := preview.ReviewerStep.ExternalHandoff.RunLoopStepID
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
				if reviewerStepID == "intake-results" {
					if err := observeSupervisionCut("reviewer-intake"); err != nil {
						return result, err
					}
					if opt.reviewerBinding != nil {
						result.FinalMode = "reviewer-intake-complete"
						return result, nil
					}
				}
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
			if step.Mode == "result-turn" {
				if err := observeSupervisionCut("intake"); err != nil {
					return result, err
				}
			}
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
			accepted := false
			run, launched, supervisionErr := supervisedClaudeRun(parent, opt, *step.HarnessPackage, reservationID, func() error {
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
			if supervisionErr != nil {
				var fencedErr *supervisionFencedError
				if errors.As(supervisionErr, &fencedErr) {
					if launched {
						launchAttempts++
					}
					reason := truncate(oneLine(fencedErr.Error()), 1024)
					args := []string{
						"-ExternalSessionLaunchOutcome", "failed",
						"-ExternalSessionActor", opt.Actor,
						"-ExternalSessionObservedAt", nowRFC3339Nano(),
						"-ExternalSessionLaunchReason", reason,
					}
					plan, planErr := runCurrentStep(opt, args, false)
					if planErr != nil {
						return result, errors.Join(supervisionErr, planErr)
					}
					if applyErr := applyCurrentStep(opt, plan, args); applyErr != nil {
						return result, errors.Join(supervisionErr, applyErr)
					}
					result.AppliedSteps++
					result.Sessions = append(result.Sessions, sessionResult(claudeRun{failureDetail: reason}, launch.Attempt.Generation, launchAttempts, reservationID, step.HarnessPackage.SessionKind, "launch-fenced", opt.MaxAttempts))
					reservationID = ""
					continue
				}
				return result, supervisionErr
			}
			if launched {
				launchAttempts++
			}
			if run.started {
				result.SessionLaunches++
			}
			if run.startCallbackErr != nil {
				result.Sessions = append(result.Sessions, sessionResult(run, launch.Attempt.Generation, launchAttempts, reservationID, step.HarnessPackage.SessionKind, "host-failed", opt.MaxAttempts))
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
				result.Sessions = append(result.Sessions, sessionResult(run, launch.Attempt.Generation, launchAttempts, reservationID, step.HarnessPackage.SessionKind, "launch-failed", opt.MaxAttempts))
				reservationID = ""
				continue
			}
			if !accepted {
				return result, fmt.Errorf("Claude process started without an accepted launch receipt")
			}
			if run.success() {
				if err := validateClaudeStructuredResult(*step.HarnessPackage, run); err != nil {
					run.failureDetail = err.Error()
				} else if err := persistClaudeRecoveryForCase(opt.Target, opt, *step.HarnessPackage, run); err != nil {
					return result, fmt.Errorf("persist Claude structured output recovery: %w", err)
				}
			}
			if err := observeSupervisionCut("result-first"); err != nil {
				return result, err
			}
			fresh, err := runCurrentStep(opt, nil, false)
			if err != nil {
				return result, err
			}
			if err := requireRunningHandoffForPackage(*step.HarnessPackage, fresh); err != nil {
				return result, err
			}
			if !run.success() && launchAttempts < opt.MaxAttempts {
				outcome := "replacement-requested"
				if run.failureDetail != "" {
					outcome = "invalid-result-replacement"
				}
				result.Sessions = append(result.Sessions, sessionResult(run, launch.Attempt.Generation, launchAttempts, reservationID, step.HarnessPackage.SessionKind, outcome, opt.MaxAttempts))
				reservationID, err = applyReplacementAttempt(opt, fresh, opt.Actor)
				if err != nil {
					return result, err
				}
				result.AppliedSteps++
				result.Replacements++
				continue
			}
			outcome, publishErr := publishClaudeResult(opt, fresh, run)
			result.Sessions = append(result.Sessions, sessionResult(run, launch.Attempt.Generation, launchAttempts, reservationID, step.HarnessPackage.SessionKind, outcome, opt.MaxAttempts))
			if publishErr == nil {
				publishErr = observeSupervisionCut("submission")
			}
			if publishErr != nil {
				return result, hostError("claude-submission-failed", "submission-publication", "result-artifact-publication-may-have-committed", "Refresh status, preserve any already-published result artifacts, and rerun the host to recover the exact current submission step.", true, publishErr)
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
				run, _, err = supervisedClaudeRun(parent, opt, *step.HarnessPackage, step.HarnessPackage.Launch.Attempt.Session, nil)
				if err != nil {
					var fencedErr *supervisionFencedError
					if errors.As(err, &fencedErr) {
						reservationID, err = applyReplacementAttempt(opt, preview, opt.Actor)
						if err != nil {
							return result, errors.Join(fencedErr, err)
						}
						result.AppliedSteps++
						result.Replacements++
						continue
					}
					return result, err
				}
				if run.success() {
					if err := validateClaudeStructuredResult(*step.HarnessPackage, run); err != nil {
						return result, err
					}
				}
			}
			if err := observeSupervisionCut("result-first"); err != nil {
				return result, err
			}
			fresh, err := runCurrentStep(opt, nil, false)
			if err != nil {
				return result, err
			}
			if err := requireSameRunningHandoff(preview, fresh); err != nil {
				return result, err
			}
			outcome, err := publishClaudeResult(opt, fresh, run)
			if err != nil {
				return result, err
			}
			if err := observeSupervisionCut("submission"); err != nil {
				return result, err
			}
			if err := removeClaudeRecoveryForCase(opt.Target, opt, *step.HarnessPackage); err != nil {
				return result, err
			}
			result.Sessions = append(result.Sessions, sessionResult(run, step.HarnessPackage.Launch.Attempt.Generation, 0, step.HarnessPackage.Launch.Attempt.Session, step.HarnessPackage.SessionKind, outcome+"-recovered", opt.MaxAttempts))
			result.SessionCompletions++
			reservationID = ""
		default:
			return result, fmt.Errorf("unsupported external session host mode %q", step.Mode)
		}
	}
	return result, fmt.Errorf("external session host exceeded transition limit")
}

func requireRunningHandoffForPackage(pkg mission.CurrentLoopExternalSessionHarnessPackage, fresh currentStepPlan) error {
	before := currentStepPlan{ExternalSessionStep: &externalSessionStep{Mode: "running-handoff", HarnessPackage: &pkg}}
	return requireSameRunningHandoff(before, fresh)
}

func requireSameRunningHandoff(before, fresh currentStepPlan) error {
	if before.ExternalSessionStep == nil || fresh.ExternalSessionStep == nil ||
		before.ExternalSessionStep.Mode != "running-handoff" || fresh.ExternalSessionStep.Mode != "running-handoff" ||
		before.ExternalSessionStep.HarnessPackage == nil || fresh.ExternalSessionStep.HarnessPackage == nil ||
		before.ExternalSessionStep.HarnessPackage.Launch == nil || fresh.ExternalSessionStep.HarnessPackage.Launch == nil {
		return fmt.Errorf("external session changed before exact supervised result publication")
	}
	left := before.ExternalSessionStep.HarnessPackage.Launch.Attempt
	right := fresh.ExternalSessionStep.HarnessPackage.Launch.Attempt
	if left.AttemptID != right.AttemptID || left.AttemptSHA256 != right.AttemptSHA256 || left.Generation != right.Generation || left.Session != right.Session ||
		before.ExternalSessionStep.HarnessPackage.JobSHA256 != fresh.ExternalSessionStep.HarnessPackage.JobSHA256 ||
		before.ExternalSessionStep.HarnessPackage.CheckpointSHA256 != fresh.ExternalSessionStep.HarnessPackage.CheckpointSHA256 {
		return fmt.Errorf("external session attempt, session, job, or checkpoint changed before exact supervised result publication")
	}
	return nil
}

func applyMemberReviewerPlanFromStatus(opt Options, status statusPlan) (bool, error) {
	if status.MissionControlRunbook == nil || status.MissionControlRunbook.Scope != "case" || status.MemberExecution == nil || status.MemberExecution.State != "intake-ready" || strings.TrimSpace(status.MemberExecution.ReviewerPlanCommand) == "" {
		return false, nil
	}
	selected := strings.TrimSpace(opt.SelectedLane)
	if selected != "" && strings.TrimSpace(status.MemberExecution.Lane) != selected {
		return false, fmt.Errorf("selected lane %q cannot apply member reviewer plan for lane %q", selected, status.MemberExecution.Lane)
	}
	args, err := cli.SplitPublicCommand(status.MemberExecution.ReviewerPlanCommand)
	if err != nil {
		return false, fmt.Errorf("decode member reviewer plan command: %w", err)
	}
	if selected != "" && !publicArgsSelectExactLane(args, selected) {
		return false, fmt.Errorf("selected lane %q member reviewer plan command changed lane", selected)
	}
	var out bytes.Buffer
	if err := cli.Run(args, &out); err != nil {
		return false, fmt.Errorf("apply member reviewer plan command: %w", err)
	}
	return true, nil
}

func publicArgsSelectExactLane(args []string, selected string) bool {
	selected = strings.TrimSpace(selected)
	for idx, arg := range args {
		if !strings.EqualFold(arg, "-Lane") && !strings.EqualFold(arg, "--lane") {
			continue
		}
		return idx+1 < len(args) && strings.TrimSpace(args[idx+1]) == selected
	}
	return false
}

func appendSelectedLaneArg(args []string, selected string) []string {
	if selected = strings.TrimSpace(selected); selected != "" {
		args = append(args, "-Lane", selected)
	}
	return args
}

func currentReviewerRejectionAwaitingCorrection(caseRoot, pack string, selected ...string) (bool, error) {
	board, err := mission.ReadBoard(caseRoot)
	if os.IsNotExist(err) {
		return false, nil
	}
	if err != nil {
		return false, err
	}
	lanes := mission.OpenBoardLanes(board.Lanes)
	if len(selected) > 0 && strings.TrimSpace(selected[0]) != "" {
		lane, ok := mission.LookupBoardLane(lanes, strings.TrimSpace(selected[0]), false)
		if !ok {
			return false, fmt.Errorf("selected lane %q is not an open current lane", selected[0])
		}
		lanes = []mission.BoardLane{lane}
	}
	for _, lane := range lanes {
		if lane.Authority || strings.TrimSpace(lane.CurrentExecutor) == "" || lane.ExecutorGeneration < 1 {
			continue
		}
		latest, found, err := memberexecution.Latest(caseRoot, lane.ID)
		if err != nil {
			return false, err
		}
		if !found || latest.State != "intake-ready" || latest.Manifest == nil || latest.Owner.Executor != lane.CurrentExecutor || latest.Owner.ExecutorGeneration != lane.ExecutorGeneration {
			continue
		}
		current, err := memberexecution.CurrentOwnerMatches(caseRoot, pack, latest.Owner)
		if err != nil {
			return false, err
		}
		if !current {
			continue
		}
		manifestRef, err := filepath.Rel(caseRoot, latest.ManifestPath)
		if err != nil {
			return false, err
		}
		_, rejected, err := workstream.CurrentMemberManifestReviewerRejection(caseRoot, lane.ID, filepath.ToSlash(manifestRef))
		if err != nil {
			return false, err
		}
		if rejected {
			return true, nil
		}
	}
	return false, nil
}

func runStatus(opt Options) (statusPlan, error) {
	args := []string{"-Command", "status", "-Target", opt.Target}
	if strings.TrimSpace(opt.Pack) != "" {
		args = append(args, "-Pack", opt.Pack)
	}
	args = appendSelectedLaneArg(args, opt.SelectedLane)
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

func memberIntakeComplete(caseRoot, selected string, plan currentStepPlan) bool {
	if plan.ExternalSessionStep != nil || plan.ReviewerStep != nil {
		return false
	}
	board, err := mission.ReadBoard(caseRoot)
	if err != nil {
		return false
	}
	lanes := mission.OpenBoardLanes(board.Lanes)
	if selected = strings.TrimSpace(selected); selected != "" {
		lane, ok := mission.LookupBoardLane(lanes, selected, false)
		if !ok {
			return false
		}
		lanes = []mission.BoardLane{lane}
	}
	for _, lane := range lanes {
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

func runCurrentStep(opt Options, extra []string, apply bool) (currentStepPlan, error) {
	return runCurrentStepWithReviewerSnapshot(opt, extra, apply, nil)
}

func runCurrentStepWithReviewerSnapshot(opt Options, extra []string, apply bool, snapshot *subagents.ReviewerResultInputSnapshot) (currentStepPlan, error) {
	if opt.reviewerBinding != nil {
		return runBoundReviewerStep(opt, extra, apply, snapshot)
	}
	args := []string{"-Command", "run-current-step", "-Target", opt.Target}
	if strings.TrimSpace(opt.Pack) != "" {
		args = append(args, "-Pack", opt.Pack)
	}
	args = appendSelectedLaneArg(args, opt.SelectedLane)
	args = append(args, extra...)
	if apply {
		args = append(args, "-Apply")
	} else {
		args = append(args, "-WhatIf")
	}
	args = append(args, "-Format", "json")
	var out bytes.Buffer
	if err := cli.RunWithReviewerResultSnapshot(args, &out, snapshot); err != nil {
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

func runBoundReviewerStep(opt Options, extra []string, apply bool, snapshot *subagents.ReviewerResultInputSnapshot) (currentStepPlan, error) {
	if err := validateReviewerBinding(opt); err != nil {
		return currentStepPlan{}, err
	}
	args := []string{"-Command", "run-reviewer-step", "-Target", opt.Target}
	if strings.TrimSpace(opt.Pack) != "" {
		args = append(args, "-Pack", opt.Pack)
	}
	args = appendSelectedLaneArg(args, opt.SelectedLane)
	args = append(args, extra...)
	if apply {
		args = append(args, "-Apply")
	} else {
		args = append(args, "-WhatIf")
	}
	args = append(args, "-Format", "json")
	var out bytes.Buffer
	if err := cli.RunWithReviewerResultSnapshot(args, &out, snapshot); err != nil {
		return currentStepPlan{}, err
	}
	var plan boundReviewerStepPlan
	dec := json.NewDecoder(bytes.NewReader(out.Bytes()))
	if err := dec.Decode(&plan); err != nil {
		return currentStepPlan{}, fmt.Errorf("decode run-reviewer-step result: %w", err)
	}
	var trailing any
	if err := dec.Decode(&trailing); err != io.EOF {
		return currentStepPlan{}, fmt.Errorf("run-reviewer-step returned trailing JSON")
	}
	if err := requireReviewerBinding(opt, plan); err != nil {
		return currentStepPlan{}, err
	}
	if err := requireReviewerResultSnapshot(snapshot, plan.ReviewerResultSnapshot); err != nil {
		return currentStepPlan{}, err
	}
	return currentStepPlan{
		Pack:                          opt.Pack,
		ExpectedCurrentStepPlanSHA256: plan.ExpectedReviewerStepPlanSHA256,
		ReviewerStep:                  &reviewerStep{ExternalHandoff: plan.ExternalHandoff},
	}, nil
}

func requireReviewerResultSnapshot(snapshot *subagents.ReviewerResultInputSnapshot, identity *reviewerResultSnapshotIdentity) error {
	if snapshot == nil {
		if identity != nil {
			return fmt.Errorf("reviewer step returned an unexpected result snapshot binding")
		}
		return nil
	}
	if identity == nil || !rekitfs.SamePath(identity.Path, snapshot.Path) ||
		!strings.EqualFold(identity.SHA256, snapshot.SHA256) || identity.Bytes != snapshot.Bytes ||
		snapshot.Bytes != int64(len(snapshot.Data)) || !strings.EqualFold(snapshot.SHA256, bytesSHA256(snapshot.Data)) {
		return fmt.Errorf("reviewer step changed the exact result snapshot binding")
	}
	return nil
}

func validateReviewerBinding(opt Options) error {
	binding := opt.reviewerBinding
	if binding == nil {
		return nil
	}
	for label, value := range map[string]string{
		"packet id":              binding.PacketID,
		"packet path":            binding.PacketPath,
		"packet sha256":          binding.PacketSHA256,
		"lane":                   binding.Lane,
		"shard id":               binding.ShardID,
		"dispatch prompt path":   binding.DispatchPromptPath,
		"dispatch prompt sha256": binding.DispatchPromptSHA256,
	} {
		if strings.TrimSpace(value) == "" {
			return fmt.Errorf("reviewer binding requires %s", label)
		}
	}
	packet, err := rekitfs.ReadStableRegularFileAnchored(opt.Target, binding.PacketPath, "bound reviewer packet", 1<<20)
	if err != nil {
		return err
	}
	if !strings.EqualFold(bytesSHA256(packet), strings.TrimSpace(binding.PacketSHA256)) {
		return fmt.Errorf("bound reviewer packet sha256 changed")
	}
	prompt, err := rekitfs.ReadStableRegularFileAnchored(opt.Target, binding.DispatchPromptPath, "bound reviewer dispatch prompt", 1<<20)
	if err != nil {
		return err
	}
	if !strings.EqualFold(bytesSHA256(prompt), strings.TrimSpace(binding.DispatchPromptSHA256)) {
		return fmt.Errorf("bound reviewer dispatch prompt sha256 changed")
	}
	return nil
}

func requireReviewerBinding(opt Options, plan boundReviewerStepPlan) error {
	binding := opt.reviewerBinding
	if binding == nil {
		return fmt.Errorf("bound reviewer step requires reviewer binding")
	}
	if plan.PacketID != strings.TrimSpace(binding.PacketID) ||
		!rekitfs.SamePath(plan.PacketPath, binding.PacketPath) ||
		plan.TargetLane != strings.TrimSpace(binding.Lane) ||
		plan.ShardID != strings.TrimSpace(binding.ShardID) {
		return fmt.Errorf("reviewer operator package changed from the exact packet, lane, or shard binding")
	}
	if plan.ExternalHandoff != nil && (!rekitfs.SamePath(plan.ExternalHandoff.DispatchPromptPath, binding.DispatchPromptPath) ||
		!strings.EqualFold(plan.ExternalHandoff.DispatchPromptSHA256, strings.TrimSpace(binding.DispatchPromptSHA256))) {
		return fmt.Errorf("reviewer operator package changed from the exact dispatch prompt binding")
	}
	return validateReviewerBinding(opt)
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
	args = appendSelectedLaneArg(args, opt.SelectedLane)
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
	return applyCurrentStepWithReviewerSnapshot(opt, plan, transitionArgs, nil)
}

func applyCurrentStepWithReviewerSnapshot(opt Options, plan currentStepPlan, transitionArgs []string, snapshot *subagents.ReviewerResultInputSnapshot) error {
	if strings.TrimSpace(plan.ExpectedCurrentStepPlanSHA256) == "" {
		return fmt.Errorf("current external session step has no deterministic apply hash")
	}
	args := append([]string{}, transitionArgs...)
	expectedFlag := "-ExpectedCurrentStepPlanSha256"
	if opt.reviewerBinding != nil {
		expectedFlag = "-ExpectedReviewerStepPlanSha256"
	}
	args = append(args, expectedFlag, plan.ExpectedCurrentStepPlanSHA256)
	_, err := runCurrentStepWithReviewerSnapshot(opt, args, true, snapshot)
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
