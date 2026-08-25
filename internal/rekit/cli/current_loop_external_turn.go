package cli

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"path/filepath"
	"strings"

	"github.com/shuiyu486/re-context-kits/internal/rekit/currentloop"
	"github.com/shuiyu486/re-context-kits/internal/rekit/executioncontrol"
	"github.com/shuiyu486/re-context-kits/internal/rekit/externalsession"
	"github.com/shuiyu486/re-context-kits/internal/rekit/lanemutation"
	"github.com/shuiyu486/re-context-kits/internal/rekit/mission"
	"github.com/shuiyu486/re-context-kits/internal/rekit/runtime"
	"github.com/shuiyu486/re-context-kits/internal/rekit/subagents"
)

var (
	currentLoopExternalTurnBeforeClaimHook func() error
	currentLoopExternalTurnAfterClaimHook  func() error
)

func currentLoopExternalTurnAllowsControlRecovery(opt Options) bool {
	if opt.currentLoopExecutionControlRecovery {
		return true
	}
	return opt.Apply && !opt.WhatIf && opt.AdvanceExternalSessionResult &&
		strings.TrimSpace(opt.ExpectedCurrentLoopCheckpointSHA256) != "" &&
		strings.TrimSpace(opt.ExpectedExternalSessionJobSHA256) != "" &&
		strings.TrimSpace(opt.ExpectedExternalSessionSubmissionSHA256) != "" &&
		strings.TrimSpace(opt.ExpectedExternalSessionRelayPlanSHA256) != "" &&
		strings.TrimSpace(opt.ExpectedExternalSessionTurnPlanSHA256) != ""
}

type currentLoopExternalSessionTurnPlan struct {
	SchemaVersion        int                  `json:"schemaVersion"`
	Mode                 string               `json:"mode"`
	CaseRoot             string               `json:"caseRoot"`
	Pack                 string               `json:"pack"`
	Relay                externalsession.Plan `json:"relay"`
	Resume               currentLoopPlan      `json:"resume"`
	ExpectedPlanSHA256   string               `json:"expectedPlanSha256"`
	ApplyCommand         string               `json:"applyCommand,omitempty"`
	ReviewRequired       bool                 `json:"reviewRequired"`
	RequiresConfirmation bool                 `json:"requiresConfirmation"`
	Applied              bool                 `json:"applied"`
	AlreadyApplied       bool                 `json:"alreadyApplied"`
	RefreshedStatus      *statusInventory     `json:"refreshedStatus,omitempty"`
	FailureStage         string               `json:"failureStage,omitempty"`
	Boundary             []string             `json:"boundary"`
}

type currentLoopExternalSessionTurnIdentity struct {
	SchemaVersion  int                                    `json:"schemaVersion"`
	CaseRoot       string                                 `json:"caseRoot"`
	Pack           string                                 `json:"pack"`
	JobSHA256      string                                 `json:"jobSha256"`
	SubmissionSHA  string                                 `json:"submissionSha256"`
	RelayPlanSHA   string                                 `json:"relayPlanSha256"`
	Observation    externalsession.ObservationBinding     `json:"observation"`
	CheckpointSHA  string                                 `json:"checkpointSha256"`
	ResumePlanSHA  string                                 `json:"resumePlanSha256"`
	InitialRequest *mission.MissionCommanderDriverRequest `json:"initialRequest,omitempty"`
	RemainingSteps int                                    `json:"remainingSteps"`
}

func runCurrentLoopExternalSessionTurn(ctx runtime.Context, opt Options, out io.Writer) error {
	if opt.WhatIf == opt.Apply {
		return fmt.Errorf("run-current-loop external session turn requires exactly one of -WhatIf or -Apply")
	}
	if !opt.ResumeCurrentLoop || strings.TrimSpace(opt.ExpectedCurrentLoopCheckpointSHA256) == "" {
		return fmt.Errorf("run-current-loop external session turn requires -ResumeCurrentLoop and -ExpectedCurrentLoopCheckpointSha256")
	}
	if opt.RelayExternalSessionSubmission || opt.MaxSteps != 0 || strings.TrimSpace(opt.ExpectedCurrentLoopPlanSHA256) != "" || strings.TrimSpace(opt.CurrentLoopObservationPath) != "" || currentStepHasMemberObservation(opt) || currentStepHasReviewerObservation(opt) || strings.TrimSpace(opt.Start.Actor) != "" {
		return fmt.Errorf("run-current-loop external session turn cannot combine relay-only, resume execution, or legacy observation flags")
	}
	if strings.ToLower(strings.TrimSpace(opt.Format)) != "json" {
		return fmt.Errorf("run-current-loop external session turn supports only -Format json")
	}
	if err := validateCurrentLoopOuterArgs(opt); err != nil {
		return err
	}
	plan, resumeOpt, resumeStatus, err := buildCurrentLoopExternalSessionTurnPlan(ctx, opt)
	if err != nil {
		return err
	}
	if opt.WhatIf {
		if strings.TrimSpace(opt.ExpectedExternalSessionSubmissionSHA256) != "" || strings.TrimSpace(opt.ExpectedExternalSessionRelayPlanSHA256) != "" || strings.TrimSpace(opt.ExpectedExternalSessionTurnPlanSHA256) != "" {
			return fmt.Errorf("external session turn -WhatIf does not accept submission, relay, or turn plan hashes")
		}
		plan.ApplyCommand = selectedLaneCommand(
			externalSessionTurnApplyCommand(plan),
			opt.SelectedCurrentLane,
		)
		return writeJSON(out, plan)
	}
	if expected := strings.TrimSpace(opt.ExpectedExternalSessionTurnPlanSHA256); expected == "" || !strings.EqualFold(expected, plan.ExpectedPlanSHA256) {
		return fmt.Errorf("external session turn plan sha256 mismatch: got %s want %s", expected, plan.ExpectedPlanSHA256)
	}
	plan, err = applyCurrentLoopExternalSessionTurn(ctx, opt, plan, resumeOpt, resumeStatus)
	if err != nil {
		if plan.Applied {
			if writeErr := writeJSON(out, plan); writeErr != nil {
				return errors.Join(err, writeErr)
			}
		}
		return err
	}
	return writeJSON(out, plan)
}

func applyCurrentLoopExternalSessionTurn(ctx runtime.Context, opt Options, plan currentLoopExternalSessionTurnPlan, resumeOpt Options, resumeStatus statusInventory) (currentLoopExternalSessionTurnPlan, error) {
	partial := func(stage string, err error) (currentLoopExternalSessionTurnPlan, error) {
		plan.Applied = plan.Relay.Applied || plan.Resume.Applied
		plan.ReviewRequired = false
		plan.RequiresConfirmation = false
		plan.FailureStage = stage
		return plan, err
	}
	appliedRelay, err := externalsession.ApplyCurrent(plan.Relay, opt.ExpectedExternalSessionJobSHA256, opt.ExpectedExternalSessionSubmissionSHA256, opt.ExpectedExternalSessionRelayPlanSHA256, func() (externalsession.Job, error) {
		return currentExternalSessionJob(ctx, opt)
	})
	if err != nil {
		return plan, err
	}
	plan.Relay = appliedRelay
	plan.AlreadyApplied = appliedRelay.AlreadyApplied
	plan.Applied = appliedRelay.Applied
	if appliedRelay.ResultPublication != nil && appliedRelay.ResultPublication.Held {
		plan.ReviewRequired = false
		plan.RequiresConfirmation = false
		plan.FailureStage = "relay-held"
		plan.Boundary = append(plan.Boundary,
			"the exact raw submission was held by durable execution control; no observation intake, checkpoint claim, nested resume, or live progression occurred",
		)
		return plan, nil
	}

	freshResume, freshStatus, err := buildCurrentLoopPlan(ctx, resumeOpt)
	if err != nil {
		return partial("resume-reconstruction", fmt.Errorf("external session turn relay committed but resume reconstruction failed: %w", err))
	}
	if !strings.EqualFold(freshResume.ExpectedCurrentLoopPlanSHA256, plan.Resume.ExpectedCurrentLoopPlanSHA256) {
		return partial("resume-plan-drift", fmt.Errorf("external session turn relay committed but nested resume plan changed; refresh status"))
	}
	plan.Resume = freshResume
	resumeStatus = freshStatus
	resumeOpt.ExpectedCurrentLoopPlanSHA256 = freshResume.ExpectedCurrentLoopPlanSHA256
	if freshResume.InitialCurrentStep != nil && freshResume.InitialCurrentStep.MemberExecution != nil {
		resumeOpt.ExpectedMemberExecutionPlanSHA256 = freshResume.InitialCurrentStep.MemberExecution.ExpectedPlanSHA256
	}
	if err := applyCurrentLoopObservationEnvelope(ctx, &resumeOpt, *freshResume.ResumeSource); err != nil {
		return partial("observation-intake", fmt.Errorf("external session turn relay committed but observation intake failed: %w", err))
	}
	requestSHA256, err := currentloop.RequestSHA256(*freshResume.InitialCurrentDriverRequest)
	if err != nil {
		return partial("checkpoint-identity", fmt.Errorf("external session turn relay committed but checkpoint identity failed: %w", err))
	}
	if currentLoopExternalTurnBeforeClaimHook != nil {
		if err := currentLoopExternalTurnBeforeClaimHook(); err != nil {
			return partial("before-checkpoint-claim", fmt.Errorf("external session turn relay committed before checkpoint claim hook failed: %w", err))
		}
	}
	binding := executioncontrol.CloneBinding(resumeOpt.currentLoopExecutionControlBinding)
	if err := currentloop.ClaimResumeValidatedWithProjectLease(ctx.RepoRoot, ctx.Target, ctx.Pack, currentloop.Claim{
		SourceArtifactSHA256:          freshResume.ExpectedResumeCheckpointSHA256,
		ExpectedCurrentLoopPlanSHA256: freshResume.ExpectedCurrentLoopPlanSHA256,
		CurrentDriverRequestSHA256:    requestSHA256,
		Actor:                         freshResume.Actor,
	}, func(lease *lanemutation.Lease) error {
		if binding != nil {
			if err := executioncontrol.RequireCurrentBindingWithProjectLease(ctx.Target, lease, *binding); err != nil {
				return err
			}
		}
		facts, err := mission.ReadStrictLedgerFacts(ctx.Target)
		if err != nil {
			return err
		}
		if currentLoopHasUnreviewedOpenIntervention(
			facts.Facts,
			freshResume.InitialLane,
			freshResume.ReviewedOpenInterventionIDs,
		) {
			return fmt.Errorf("external session turn refuses checkpoint claim after Human-in-the-Lane intervention")
		}
		return nil
	}); err != nil {
		return partial("checkpoint-claim", fmt.Errorf("external session turn relay committed but checkpoint claim failed: %w", err))
	}
	if currentLoopExternalTurnAfterClaimHook != nil {
		if err := currentLoopExternalTurnAfterClaimHook(); err != nil {
			return partial("after-checkpoint-claim", fmt.Errorf("external session turn checkpoint claim committed but after-claim hook failed: %w", err))
		}
	}
	plan.Resume, err = applyBuiltCurrentLoop(ctx, resumeOpt, freshResume, resumeStatus)
	if err != nil {
		return partial("nested-resume", err)
	}
	plan.Applied = plan.Resume.Applied || plan.Relay.Applied
	plan.ReviewRequired = false
	plan.RequiresConfirmation = false
	fresh, refreshErr := buildInvocationStatusInventory(ctx, opt)
	if refreshErr == nil {
		plan.RefreshedStatus = &fresh
	}
	return plan, nil
}

func buildCurrentLoopExternalSessionTurnPlan(ctx runtime.Context, opt Options) (currentLoopExternalSessionTurnPlan, Options, statusInventory, error) {
	status, statusErr := buildInvocationStatusInventory(ctx, opt)
	recovered := false
	statusReady := func(candidate statusInventory) bool {
		return candidate.MissionControlRunbook != nil &&
			candidate.MissionControlRunbook.CurrentLoopOperator != nil &&
			candidate.MissionControlRunbook.CurrentLoopOperator.ExternalSessionJob != nil &&
			candidate.MissionControlRunbook.CurrentLoopSegment != nil
	}
	if (statusErr != nil || !statusReady(status)) && currentLoopExternalTurnAllowsControlRecovery(opt) {
		recoveryStatus, recoveryErr := buildControlBoundResultRecoveryStatusInventory(ctx, opt)
		if recoveryErr == nil && statusReady(recoveryStatus) {
			status = recoveryStatus
			statusErr = nil
			recovered = true
		} else if statusErr != nil {
			if recoveryErr != nil {
				return currentLoopExternalSessionTurnPlan{}, Options{}, statusInventory{}, errors.Join(statusErr, fmt.Errorf("control-bound result recovery status: %w", recoveryErr))
			}
			return currentLoopExternalSessionTurnPlan{}, Options{}, statusInventory{}, statusErr
		}
	}
	if statusErr != nil {
		return currentLoopExternalSessionTurnPlan{}, Options{}, statusInventory{}, statusErr
	}
	if !statusReady(status) {
		return currentLoopExternalSessionTurnPlan{}, Options{}, statusInventory{}, fmt.Errorf("run-current-loop external session turn requires a current externalSessionJob")
	}
	inspection := status.MissionControlRunbook.CurrentLoopSegment
	if !inspection.Ready || inspection.State != "ready" || !strings.EqualFold(inspection.ArtifactSHA256, opt.ExpectedCurrentLoopCheckpointSHA256) {
		return currentLoopExternalSessionTurnPlan{}, Options{}, statusInventory{}, fmt.Errorf("run-current-loop external session turn checkpoint is not the latest ready checkpoint")
	}
	job, err := externalSessionJobForControlRecovery(
		ctx.Target,
		status.MissionControlRunbook.CurrentLoopOperator,
		*inspection,
		recovered,
	)
	if err != nil {
		return currentLoopExternalSessionTurnPlan{}, Options{}, statusInventory{}, err
	}
	relay, err := externalsession.Preview(job)
	if err != nil {
		return currentLoopExternalSessionTurnPlan{}, Options{}, statusInventory{}, err
	}
	if expected := strings.TrimSpace(opt.ExpectedExternalSessionJobSHA256); expected == "" || !strings.EqualFold(expected, relay.JobSHA256) {
		return currentLoopExternalSessionTurnPlan{}, Options{}, statusInventory{}, fmt.Errorf("external session turn expected job sha256 mismatch: got %s want %s", expected, relay.JobSHA256)
	}
	binding := relay.Submission.LaunchControl
	if recovered && binding == nil {
		return currentLoopExternalSessionTurnPlan{}, Options{}, statusInventory{}, fmt.Errorf("control-bound result recovery requires submission execution control lineage")
	}
	if binding != nil {
		if err := executioncontrol.ValidateBinding(*binding); err != nil {
			return currentLoopExternalSessionTurnPlan{}, Options{}, statusInventory{}, fmt.Errorf("external session turn execution control binding is invalid: %w", err)
		}
		if binding.Lane != strings.TrimSpace(inspection.ExpectedLane) {
			return currentLoopExternalSessionTurnPlan{}, Options{}, statusInventory{}, fmt.Errorf("external session turn execution control lane does not match checkpoint continuation")
		}
	}
	observationPath := filepath.Join(ctx.Target, filepath.FromSlash(relay.Observation.Path))
	snapshot, err := decodeCurrentLoopObservationSnapshot(observationPath, relay.Observation.Data())
	if err != nil {
		return currentLoopExternalSessionTurnPlan{}, Options{}, statusInventory{}, err
	}
	if !executioncontrol.SameBinding(binding, snapshot.Envelope.LaunchControl) {
		return currentLoopExternalSessionTurnPlan{}, Options{}, statusInventory{}, fmt.Errorf("external session turn observation execution control binding does not match submission lineage")
	}
	resumeOpt := opt
	resumeOpt.AdvanceExternalSessionResult = false
	resumeOpt.ExpectedExternalSessionJobSHA256 = ""
	resumeOpt.ExpectedExternalSessionSubmissionSHA256 = ""
	resumeOpt.ExpectedExternalSessionRelayPlanSHA256 = ""
	resumeOpt.ExpectedExternalSessionTurnPlanSHA256 = ""
	resumeOpt.CurrentLoopObservationPath = observationPath
	resumeOpt.ExpectedCurrentLoopObservationSHA256 = snapshot.SHA256
	resumeOpt.currentLoopObservationSnapshot = &snapshot
	resumeOpt.currentLoopMemberResultSnapshot = relay.MemberResultSnapshot()
	resumeOpt.currentLoopExternalTurnResume = true
	resumeOpt.currentLoopExecutionControlBinding = executioncontrol.CloneBinding(binding)
	resumeOpt.currentLoopExecutionControlRecovery = recovered && binding != nil
	if relay.ReviewerResult != nil {
		resumeOpt.currentLoopReviewerResultSnapshot = &subagents.ReviewerResultInputSnapshot{
			Path:   filepath.Join(ctx.Target, filepath.FromSlash(relay.ReviewerResult.Path)),
			SHA256: relay.ReviewerResult.SHA256,
			Bytes:  relay.ReviewerResult.Bytes,
			Data:   relay.ReviewerResult.Data(),
		}
	}
	resumeOpt.WhatIf = true
	resumeOpt.Apply = false
	resumeOpt.rawArgs = nil
	resume, resumeStatus, err := buildCurrentLoopPlan(ctx, resumeOpt)
	if err != nil {
		return currentLoopExternalSessionTurnPlan{}, Options{}, statusInventory{}, err
	}
	if resume.ResumeSource == nil || resume.InitialCurrentDriverRequest == nil || resume.InitialCurrentStep == nil || strings.TrimSpace(resume.ExpectedCurrentLoopPlanSHA256) == "" {
		detail := strings.TrimSpace(resume.StopReason.Message)
		if detail == "" {
			detail = "nested resume omitted a current request, step, checkpoint, or plan hash"
		}
		return currentLoopExternalSessionTurnPlan{}, Options{}, statusInventory{}, fmt.Errorf("external session turn nested resume is not executable: %s; refresh status or use relay-only recovery", detail)
	}
	identity := currentLoopExternalSessionTurnIdentity{
		SchemaVersion: 1, CaseRoot: ctx.Target, Pack: ctx.Pack, JobSHA256: relay.JobSHA256,
		SubmissionSHA: relay.SubmissionSHA256, RelayPlanSHA: relay.ExpectedPlanSHA256, Observation: relay.Observation,
		CheckpointSHA: inspection.ArtifactSHA256, ResumePlanSHA: resume.ExpectedCurrentLoopPlanSHA256,
		InitialRequest: resume.InitialCurrentDriverRequest, RemainingSteps: inspection.RemainingMaxSteps,
	}
	encoded, err := json.Marshal(identity)
	if err != nil {
		return currentLoopExternalSessionTurnPlan{}, Options{}, statusInventory{}, err
	}
	sum := sha256.Sum256(encoded)
	plan := currentLoopExternalSessionTurnPlan{
		SchemaVersion: 1, Mode: "external-result-turn", CaseRoot: ctx.Target, Pack: ctx.Pack, Relay: relay, Resume: resume,
		ExpectedPlanSHA256: hex.EncodeToString(sum[:]), ReviewRequired: true, RequiresConfirmation: true,
		Boundary: []string{
			"one reviewed turn binds the exact checkpoint, job, submission, relay artifacts, observation bytes, nested resume plan, and remaining budget",
			"Apply revalidates every phase and stops at the next external session, Human-in-the-Lane, blocker, route/lane transition, budget, or terminal boundary",
			"the turn is non-transactional; a committed relay remains truthful and recoverable if a later phase rejects drift",
			"the Go runtime does not manage external sessions, invoke shell or Agent tools, execute heavy tools, or write authority/confirmed state",
		},
	}
	if opt.Apply {
		resumeOpt.WhatIf = false
		resumeOpt.Apply = true
		resumeOpt.ExpectedCurrentLoopObservationSHA256 = snapshot.SHA256
		resumeOpt.currentLoopObservationSnapshot = nil
		resumeOpt.currentLoopMemberResultSnapshot = nil
		resumeOpt.currentLoopReviewerResultSnapshot = nil
	}
	return plan, resumeOpt, resumeStatus, nil
}

func externalSessionTurnApplyCommand(plan currentLoopExternalSessionTurnPlan) string {
	return joinDriverCommand([]string{
		"/rekit", "run-current-loop", "-Target", plan.CaseRoot, "-Pack", plan.Pack,
		"-ResumeCurrentLoop", "-ExpectedCurrentLoopCheckpointSha256", plan.Relay.Job.CheckpointSHA256,
		"-AdvanceExternalSessionResult", "-ExpectedExternalSessionJobSha256", plan.Relay.JobSHA256,
		"-ExpectedExternalSessionSubmissionSha256", plan.Relay.SubmissionSHA256,
		"-ExpectedExternalSessionRelayPlanSha256", plan.Relay.ExpectedPlanSHA256,
		"-ExpectedExternalSessionTurnPlanSha256", plan.ExpectedPlanSHA256,
		"-Apply", "-Format", "json",
	})
}

func externalSessionTurnRequest(job externalsession.Job, jobSHA string) (mission.MissionCommanderDriverRequest, error) {
	refresh, err := statusMissionControlRefreshCommand(job.CaseRoot)
	if err != nil {
		return mission.MissionCommanderDriverRequest{}, err
	}
	command := joinDriverCommand([]string{
		"/rekit", "run-current-loop", "-Target", job.CaseRoot, "-Pack", job.Pack,
		"-ResumeCurrentLoop", "-ExpectedCurrentLoopCheckpointSha256", job.CheckpointSHA256,
		"-AdvanceExternalSessionResult", "-ExpectedExternalSessionJobSha256", jobSHA,
		"-WhatIf", "-Format", "json",
	})
	return mission.MissionCommanderDriverRequestWithTypedCommand(
		mission.MissionCommanderDriverRequest{
			Kind: "preview-command", RunLoopStepID: "external-session-turn:" + job.JobID, Actor: "mission-commander",
			State: "review-required", Source: "current-loop-external-session-turn", Lane: memberLane(job), Label: job.SessionKind + " session result turn",
			Command: command, CommandExecutable: true, RequiresReview: true,
			ExpectedReceipt: mission.MissionCommanderDriverReceiptExpectation{
				State: "preview-ready", RefreshStatusCommand: refresh,
				Description: "returns one exact external-result turn hash binding relay and checkpoint resume",
				Boundary:    []string{"preview publishes nothing; Apply advances only to the next typed campaign boundary"},
			},
			Boundary: []string{"execute only while the exact checkpoint, job, submission, and owner/reviewer attempt remain current"},
		},
	)
}
