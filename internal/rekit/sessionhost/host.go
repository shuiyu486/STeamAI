package sessionhost

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/hex"
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
	Target                             string
	Pack                               string
	SelectedLane                       string
	ExpectedCurrentDriverRequestSHA256 string
	Actor                              string
	ClaudePath                         string
	ExpectedClaudeExecutableSHA256     string
	ExpectedClaudeExecutablePublisher  string
	Model                              string
	Timeout                            time.Duration
	MaxAttempts                        int
	StopAfterMemberIntake              bool
	requireCurrentDriverRequest        bool
	requireDailyClaudeTrust            bool
	reviewerBinding                    *reviewerBinding
}

func (opt *Options) RequireCurrentDriverRequest() {
	if opt != nil {
		opt.requireCurrentDriverRequest = true
	}
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

type memberExecutionStatus struct {
	State               string `json:"state,omitempty"`
	Lane                string `json:"lane,omitempty"`
	ReviewerPlanCommand string `json:"reviewerPlanCommand,omitempty"`
}

type statusPlan struct {
	MemberExecution       *memberExecutionStatus       `json:"memberExecution,omitempty"`
	MissionControlRunbook *publicMissionControlRunbook `json:"missionControlRunbook,omitempty"`
	CaseMission           *publicCaseMission           `json:"caseMission,omitempty"`
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
	if opt.requireCurrentDriverRequest || strings.TrimSpace(opt.ExpectedCurrentDriverRequestSHA256) != "" {
		if err := validateHostCurrentDriverRequest(opt); err != nil {
			return result, err
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

func validateHostCurrentDriverRequest(opt Options) error {
	if opt.reviewerBinding != nil {
		if strings.TrimSpace(opt.ExpectedCurrentDriverRequestSHA256) != "" {
			return hostError("current-driver-request-binding-invalid", "driver-request-currentness", "none", "Remove the case driver request SHA from the explicitly bound Reviewer host route.", false, fmt.Errorf("reviewer-bound host must not consume a case current driver request SHA-256"))
		}
		return nil
	}
	expected := strings.ToLower(strings.TrimSpace(opt.ExpectedCurrentDriverRequestSHA256))
	if decoded, err := hex.DecodeString(expected); err != nil || len(decoded) != sha256.Size {
		return hostError("current-driver-request-required", "driver-request-currentness", "none", "Refresh status and rerun the host with the exact missionControlRunbook.currentDriverRequestSha256.", false, fmt.Errorf("host requires an exact current driver request SHA-256 before starting Claude"))
	}
	status, err := runStatus(opt)
	if err != nil {
		return hostError("current-driver-request-unavailable", "driver-request-currentness", "none", "Refresh status and retry only after one focused executable driver request is available.", false, fmt.Errorf("refresh host current driver request: %w", err))
	}
	selected := strings.TrimSpace(opt.SelectedLane)
	if selected == "" && len(publicCaseMissionLaneChoices(status.CaseMission)) > 1 {
		return hostError("current-driver-request-lane-required", "driver-request-currentness", "none", "Refresh status, select one typed lane choice, and rerun the host with that lane and its exact current driver request SHA-256.", false, fmt.Errorf("host current driver request is ambiguous across multiple executable lanes"))
	}
	if status.MissionControlRunbook == nil || status.MissionControlRunbook.CurrentDriverRequest == nil {
		return hostError("current-driver-request-unavailable", "driver-request-currentness", "none", "Refresh status and retry only after one focused executable driver request is available.", false, fmt.Errorf("status omitted the current driver request"))
	}
	request := *status.MissionControlRunbook.CurrentDriverRequest
	if err := mission.ValidateMissionCommanderDriverRequest(request); err != nil {
		return hostError("current-driver-request-invalid", "driver-request-currentness", "none", "Refresh status and resolve the blocked or malformed typed invocation before starting the host.", false, err)
	}
	if selected != "" && strings.TrimSpace(request.Lane) != selected {
		return hostError("current-driver-request-stale", "driver-request-currentness", "none", "Refresh the selected lane status and use its new exact current driver request SHA-256.", false, fmt.Errorf("selected lane %q resolved current driver request for lane %q", selected, request.Lane))
	}
	actual, err := mission.MissionCommanderDriverRequestSHA256(request)
	if err != nil {
		return hostError("current-driver-request-invalid", "driver-request-currentness", "none", "Refresh status and resolve the malformed typed invocation before starting the host.", false, err)
	}
	published := strings.ToLower(strings.TrimSpace(status.MissionControlRunbook.CurrentDriverRequestSHA256))
	if published == "" || published != actual {
		return hostError("current-driver-request-invalid", "driver-request-currentness", "none", "Refresh status because its current driver request identity projection is incomplete or inconsistent.", false, fmt.Errorf("status current driver request SHA-256 does not match the typed request"))
	}
	if expected != actual {
		return hostError("current-driver-request-stale", "driver-request-currentness", "none", "Refresh status and use the new exact missionControlRunbook.currentDriverRequestSha256; do not reuse the stale request.", false, fmt.Errorf("expected current driver request SHA-256 is stale"))
	}
	return nil
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
