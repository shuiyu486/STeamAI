package cli

import (
	"fmt"
	"io"
	"strings"

	"github.com/shuiyu486/re-context-kits/internal/rekit/externalsession"
	"github.com/shuiyu486/re-context-kits/internal/rekit/mission"
	"github.com/shuiyu486/re-context-kits/internal/rekit/runtime"
)

type currentLoopExternalSessionAttemptResult struct {
	externalsession.AttemptPlan
	RefreshedStatus *statusInventory `json:"refreshedStatus,omitempty"`
}

func runCurrentLoopExternalSessionAttempt(ctx runtime.Context, opt Options, out io.Writer) error {
	if opt.WhatIf == opt.Apply {
		return fmt.Errorf("run-current-loop external session attempt requires exactly one of -WhatIf or -Apply")
	}
	if !opt.ResumeCurrentLoop || strings.TrimSpace(opt.ExpectedCurrentLoopCheckpointSHA256) == "" || strings.TrimSpace(opt.ExpectedExternalSessionJobSHA256) == "" {
		return fmt.Errorf("external session attempt requires resume checkpoint and expected job sha256")
	}
	if opt.RelayExternalSessionSubmission || opt.AdvanceExternalSessionResult || opt.MaxSteps != 0 || strings.TrimSpace(opt.ExpectedCurrentLoopPlanSHA256) != "" || strings.TrimSpace(opt.CurrentLoopObservationPath) != "" || currentStepHasMemberObservation(opt) || currentStepHasReviewerObservation(opt) || strings.TrimSpace(opt.Start.Actor) != "" {
		return fmt.Errorf("external session attempt cannot combine result, relay, resume execution, or observation flags")
	}
	if strings.ToLower(strings.TrimSpace(opt.Format)) != "json" {
		return fmt.Errorf("external session attempt supports only -Format json")
	}
	if err := validateCurrentLoopOuterArgs(opt); err != nil {
		return err
	}
	status, err := buildStatusInventory(ctx, statusPackSource(ctx, opt))
	if err != nil {
		return err
	}
	if status.MissionControlRunbook == nil || status.MissionControlRunbook.CurrentLoopOperator == nil || status.MissionControlRunbook.CurrentLoopOperator.ExternalSessionJob == nil || status.MissionControlRunbook.CurrentLoopSegment == nil {
		return fmt.Errorf("external session attempt requires a current externalSessionJob")
	}
	checkpoint := status.MissionControlRunbook.CurrentLoopSegment
	if !checkpoint.Ready || checkpoint.State != "ready" || !strings.EqualFold(checkpoint.ArtifactSHA256, opt.ExpectedCurrentLoopCheckpointSHA256) {
		return fmt.Errorf("external session attempt checkpoint is not the latest ready checkpoint")
	}
	job, err := externalSessionJobFor(ctx.Target, status.MissionControlRunbook.CurrentLoopOperator, *checkpoint)
	if err != nil {
		return err
	}
	var plan externalsession.AttemptPlan
	if opt.Apply {
		plan, err = externalsession.ResolveAttemptApplyPlan(job, opt.ExternalSessionHarness, opt.ExternalSessionID, opt.ExternalSessionActor, opt.ExternalSessionStartedAt, opt.ExpectedExternalSessionAttemptSHA256, opt.ExpectedExternalSessionAttemptPlanSHA256)
	} else {
		plan, err = externalsession.PreviewAttempt(job, opt.ExternalSessionHarness, opt.ExternalSessionID, opt.ExternalSessionActor, opt.ExternalSessionStartedAt, opt.ExpectedExternalSessionAttemptSHA256)
	}
	if err != nil {
		return err
	}
	if !strings.EqualFold(strings.TrimSpace(opt.ExpectedExternalSessionJobSHA256), plan.JobSHA256) {
		return fmt.Errorf("external session attempt expected job sha256 mismatch")
	}
	if opt.WhatIf {
		if strings.TrimSpace(opt.ExpectedExternalSessionAttemptPlanSHA256) != "" {
			return fmt.Errorf("external session attempt WhatIf does not accept an attempt plan hash")
		}
		plan.ApplyCommand = externalSessionAttemptApplyCommand(plan)
		return writeJSON(out, plan)
	}
	applied, err := externalsession.ApplyAttemptCurrent(plan, opt.ExpectedExternalSessionJobSHA256, opt.ExpectedExternalSessionAttemptPlanSHA256, func() (externalsession.Job, error) {
		return currentExternalSessionJob(ctx, opt)
	})
	if err != nil {
		return err
	}
	result := currentLoopExternalSessionAttemptResult{AttemptPlan: applied}
	fresh, refreshErr := buildStatusInventory(ctx, statusPackSource(ctx, opt))
	if refreshErr == nil {
		result.RefreshedStatus = &fresh
	}
	return writeJSON(out, result)
}

func externalSessionAttemptRequest(job externalsession.Job, jobSHA string, inspection externalsession.AttemptInspection) mission.MissionCommanderDriverRequest {
	supersedes := inspection.AttemptSHA256
	args := []string{
		"/rekit", "run-current-loop", "-Target", job.CaseRoot, "-Pack", job.Pack,
		"-ResumeCurrentLoop", "-ExpectedCurrentLoopCheckpointSha256", job.CheckpointSHA256,
		"-RecordExternalSessionAttempt", "-ExpectedExternalSessionJobSha256", jobSHA,
		"-ExternalSessionHarness", "<harness>", "-ExternalSessionId", "<session-id>",
		"-ExternalSessionActor", "<actor>", "-ExternalSessionStartedAt", "<rfc3339nano>",
	}
	if supersedes != "" {
		args = append(args, "-ExpectedExternalSessionAttemptSha256", supersedes)
	}
	args = append(args, "-WhatIf", "-Format", "json")
	command := joinDriverCommand(args)
	state := "ready-for-attempt-review"
	description := "returns an exact first external harness attempt plan"
	if inspection.Current != nil {
		state = "running-or-replacement-review"
		description = "current attempt is running; the command previews an explicit replacement bound to its exact sha256"
	}
	return mission.MissionCommanderDriverRequest{
		Kind: "preview-command-template", RunLoopStepID: "external-session-attempt:" + job.JobID,
		Actor: "mission-commander", State: state, Source: "current-loop-external-session-job",
		Lane: memberLane(job), Label: job.SessionKind + " external session attempt", Command: command,
		CommandExecutable: false, RequiresReview: true,
		ExpectedReceipt: mission.MissionCommanderDriverReceiptExpectation{
			State: "attempt-plan-ready", RefreshStatusCommand: statusMissionControlRefreshCommand(job.CaseRoot),
			Description: description,
			Boundary:    []string{"replace placeholders before execution; WhatIf writes nothing", "Apply records ownership but does not start or manage a session"},
		},
		Boundary: []string{"only the external harness starts, polls, stops, and reports the session", "replacement requires exact current attempt sha256"},
	}
}

func externalSessionAttemptApplyCommand(plan externalsession.AttemptPlan) string {
	args := []string{
		"/rekit", "run-current-loop", "-Target", plan.Job.CaseRoot, "-Pack", plan.Job.Pack,
		"-ResumeCurrentLoop", "-ExpectedCurrentLoopCheckpointSha256", plan.Job.CheckpointSHA256,
		"-RecordExternalSessionAttempt", "-ExpectedExternalSessionJobSha256", plan.JobSHA256,
		"-ExternalSessionHarness", plan.Attempt.Harness, "-ExternalSessionId", plan.Attempt.Session,
		"-ExternalSessionActor", plan.Attempt.Actor, "-ExternalSessionStartedAt", plan.Attempt.StartedAt,
	}
	if plan.Attempt.SupersedesSHA256 != "" {
		args = append(args, "-ExpectedExternalSessionAttemptSha256", plan.Attempt.SupersedesSHA256)
	}
	args = append(args, "-ExpectedExternalSessionAttemptPlanSha256", plan.ExpectedPlanSHA256, "-Apply", "-Format", "json")
	return joinDriverCommand(args)
}
