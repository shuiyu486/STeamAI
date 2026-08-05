package cli

import (
	"fmt"
	"io"
	"strings"

	"github.com/shuiyu486/re-context-kits/internal/rekit/externalsession"
	"github.com/shuiyu486/re-context-kits/internal/rekit/mission"
	"github.com/shuiyu486/re-context-kits/internal/rekit/runtime"
)

type currentLoopExternalSessionDispatchResult struct {
	externalsession.DispatchPlan
	RefreshedStatus *statusInventory `json:"refreshedStatus,omitempty"`
}

func runCurrentLoopExternalSessionDispatch(ctx runtime.Context, opt Options, out io.Writer) error {
	if opt.ClaimExternalSessionDispatch == opt.RecordExternalSessionLaunch {
		return fmt.Errorf("external session dispatch requires exactly one of claim or launch mode")
	}
	if opt.WhatIf == opt.Apply {
		return fmt.Errorf("external session dispatch requires exactly one of -WhatIf or -Apply")
	}
	if !opt.ResumeCurrentLoop || strings.TrimSpace(opt.ExpectedCurrentLoopCheckpointSHA256) == "" || strings.TrimSpace(opt.ExpectedExternalSessionJobSHA256) == "" || strings.TrimSpace(opt.ExpectedExternalSessionAttemptSHA256) == "" || strings.TrimSpace(opt.ExpectedExternalSessionDispatchSHA256) == "" {
		return fmt.Errorf("external session dispatch requires resume checkpoint and exact job, attempt, and dispatch sha256")
	}
	if opt.MaxSteps != 0 || strings.TrimSpace(opt.ExpectedCurrentLoopPlanSHA256) != "" || strings.TrimSpace(opt.CurrentLoopObservationPath) != "" || currentStepHasMemberObservation(opt) || currentStepHasReviewerObservation(opt) || strings.TrimSpace(opt.Start.Actor) != "" || opt.RelayExternalSessionSubmission || opt.AdvanceExternalSessionResult || opt.RecordExternalSessionAttempt {
		return fmt.Errorf("external session dispatch cannot combine other current-loop modes or observations")
	}
	if strings.ToLower(strings.TrimSpace(opt.Format)) != "json" {
		return fmt.Errorf("external session dispatch supports only -Format json")
	}
	if err := validateCurrentLoopOuterArgs(opt); err != nil {
		return err
	}
	status, err := buildStatusInventory(ctx, statusPackSource(ctx, opt))
	if err != nil {
		return err
	}
	if status.MissionControlRunbook == nil || status.MissionControlRunbook.CurrentLoopOperator == nil || status.MissionControlRunbook.CurrentLoopOperator.ExternalSessionJob == nil || status.MissionControlRunbook.CurrentLoopSegment == nil {
		return fmt.Errorf("external session dispatch requires a current externalSessionJob")
	}
	checkpoint := status.MissionControlRunbook.CurrentLoopSegment
	if !checkpoint.Ready || checkpoint.State != "ready" || !strings.EqualFold(checkpoint.ArtifactSHA256, opt.ExpectedCurrentLoopCheckpointSHA256) {
		return fmt.Errorf("external session dispatch checkpoint is not the latest ready checkpoint")
	}
	job, err := externalSessionJobFor(ctx.Target, status.MissionControlRunbook.CurrentLoopOperator, *checkpoint)
	if err != nil {
		return err
	}
	attempt, err := externalsession.InspectAttempt(job)
	if err != nil || attempt.Current == nil {
		return fmt.Errorf("external session dispatch requires the current committed attempt")
	}
	if !strings.EqualFold(attempt.AttemptSHA256, opt.ExpectedExternalSessionAttemptSHA256) {
		return fmt.Errorf("external session dispatch expected attempt sha256 mismatch")
	}
	outcome := "claimed"
	if opt.RecordExternalSessionLaunch {
		outcome = opt.ExternalSessionLaunchOutcome
	}
	plan, err := externalsession.PreviewDispatchTransition(
		job, attempt, outcome, opt.ExternalSessionActor, opt.ExternalSessionObservedAt,
		opt.ExternalSessionHarness, opt.ExternalSessionID, opt.ExternalSessionLaunchReason,
	)
	if err != nil {
		return err
	}
	if !strings.EqualFold(plan.JobSHA256, opt.ExpectedExternalSessionJobSHA256) || !strings.EqualFold(plan.DispatchSHA256, opt.ExpectedExternalSessionDispatchSHA256) || !strings.EqualFold(plan.ExpectedClaimSHA256, opt.ExpectedExternalSessionClaimSHA256) {
		return fmt.Errorf("external session dispatch durable identity mismatch")
	}
	if opt.WhatIf {
		if strings.TrimSpace(opt.ExpectedExternalSessionDispatchPlanSHA256) != "" {
			return fmt.Errorf("external session dispatch WhatIf does not accept a dispatch plan hash")
		}
		plan.ApplyCommand = externalSessionDispatchApplyCommand(plan)
		return writeJSON(out, plan)
	}
	applied, err := externalsession.ApplyDispatchTransitionCurrent(
		plan, opt.ExpectedExternalSessionJobSHA256, opt.ExpectedExternalSessionDispatchSHA256,
		opt.ExpectedExternalSessionClaimSHA256, opt.ExpectedExternalSessionDispatchPlanSHA256,
		func() (externalsession.Job, error) { return currentExternalSessionJob(ctx, opt) },
	)
	if err != nil {
		return err
	}
	result := currentLoopExternalSessionDispatchResult{DispatchPlan: applied}
	if fresh, refreshErr := buildStatusInventory(ctx, statusPackSource(ctx, opt)); refreshErr == nil {
		result.RefreshedStatus = &fresh
	}
	return writeJSON(out, result)
}

func externalSessionDispatcherPackage(job externalsession.Job, attemptSHA256 string, inspection externalsession.DispatchInspection) *mission.CurrentLoopExternalSessionDispatcher {
	pkg := &mission.CurrentLoopExternalSessionDispatcher{
		State: inspection.State, Warnings: append([]string{}, inspection.Warnings...),
		Boundary: []string{
			"attempt ownership, exclusive dispatch claim, and actual launch receipt are separate immutable facts",
			"only the exact current claim owner may launch outside the Go runtime",
			"replacement fences stale generations but does not stop an already-running external process",
		},
	}
	if inspection.Ticket != nil {
		pkg.Ticket = &mission.CurrentLoopExternalSessionDispatchTicket{
			Path: inspection.TicketPath, SHA256: inspection.TicketSHA256,
			AttemptSHA256: attemptSHA256, Generation: inspection.Ticket.Attempt.Generation,
		}
	}
	if inspection.Claim != nil {
		pkg.Claim = &mission.CurrentLoopExternalSessionDispatchClaim{
			Path: inspection.ClaimPath, SHA256: inspection.ClaimSHA256,
			Harness: inspection.Claim.Harness, Session: inspection.Claim.Session,
			Actor: inspection.Claim.Actor, ClaimedAt: inspection.Claim.ClaimedAt,
		}
	}
	if inspection.Launch != nil {
		pkg.LaunchReceipt = &mission.CurrentLoopExternalSessionLaunchReceipt{
			State: inspection.Launch.Outcome, Path: inspection.LaunchPath, SHA256: inspection.LaunchSHA256,
			ActualHarness: inspection.Launch.ActualHarness, ActualSession: inspection.Launch.ActualSession,
			Actor: inspection.Launch.Actor, ObservedAt: inspection.Launch.ObservedAt, Reason: inspection.Launch.Reason,
		}
	}
	if inspection.Ticket != nil && inspection.State == "queued" {
		request := externalSessionDispatchRequest(job, attemptSHA256, inspection, "claimed")
		pkg.ClaimRequest = &request
	}
	if inspection.Claim != nil && inspection.State == "claimed" {
		accepted := externalSessionDispatchRequest(job, attemptSHA256, inspection, "accepted")
		failed := externalSessionDispatchRequest(job, attemptSHA256, inspection, "failed")
		pkg.LaunchAcceptedRequest, pkg.LaunchFailedRequest = &accepted, &failed
	}
	return pkg
}

func externalSessionDispatcherRequestIsFocused(operator *mission.CurrentLoopOperatorPackage) bool {
	if operator == nil || operator.ExternalSessionJob == nil || operator.ExternalSessionJob.Dispatcher == nil || operator.SelectedDriverRequest == nil {
		return false
	}
	job := operator.ExternalSessionJob
	dispatcher := job.Dispatcher
	switch dispatcher.State {
	case "attempt-publication-pending":
		return job.AttemptRequest != nil && operator.SelectedDriverRequest.Command == job.AttemptRequest.Command
	case "queued":
		return job.CurrentAttempt != nil && dispatcher.ClaimRequest != nil && operator.SelectedDriverRequest.Command == dispatcher.ClaimRequest.Command
	case "claimed":
		return job.CurrentAttempt != nil && dispatcher.LaunchAcceptedRequest != nil && operator.SelectedDriverRequest.Command == dispatcher.LaunchAcceptedRequest.Command
	case "launch-failed":
		return job.CurrentAttempt != nil && job.AttemptRequest != nil && operator.SelectedDriverRequest.Command == job.AttemptRequest.Command
	default:
		return false
	}
}

func applyExternalSessionDispatchState(operator *mission.CurrentLoopOperatorPackage, job *mission.CurrentLoopExternalSessionJob, inspection externalsession.DispatchInspection) {
	if operator == nil || job == nil || job.HarnessPackage == nil || job.CurrentAttempt == nil {
		return
	}
	switch inspection.State {
	case "queued":
		operator.State = "external-session-dispatch-ready"
		operator.SelectedDriverRequest = job.Dispatcher.ClaimRequest
		operator.RunbookSteps = mission.UniqueStrings(append(operator.RunbookSteps,
			"review and claim the exact immutable dispatch ticket before any external launch",
			"attempt ownership alone is not actual launch truth",
		))
		job.HarnessPackage.State = "dispatch-ready"
		if job.HarnessPackage.Launch != nil {
			job.HarnessPackage.Launch.Ready = false
		}
	case "claimed":
		operator.State = "external-session-launch-receipt-required"
		operator.SelectedDriverRequest = job.Dispatcher.LaunchAcceptedRequest
		operator.RunbookSteps = mission.UniqueStrings(append(operator.RunbookSteps,
			"only the exact durable claim owner may launch the external session",
			"after the external launch attempt, record exactly one accepted or failed launch receipt",
		))
		job.HarnessPackage.State = "launch-ready"
	case "running":
		operator.State = "external-session-running"
		operator.SelectedDriverRequest = nil
		job.HarnessPackage.State = "running"
		if job.HarnessPackage.Launch != nil {
			job.HarnessPackage.Launch.Ready = false
		}
	case "launch-failed":
		operator.State = "external-session-launch-failed"
		operator.SelectedDriverRequest = job.AttemptRequest
		job.HarnessPackage.State = "launch-failed-replacement-review"
		if job.HarnessPackage.Launch != nil {
			job.HarnessPackage.Launch.Ready = false
		}
	}
}

func externalSessionDispatchRequest(job externalsession.Job, attemptSHA256 string, inspection externalsession.DispatchInspection, outcome string) mission.MissionCommanderDriverRequest {
	args := []string{
		"/rekit", "run-current-loop", "-Target", job.CaseRoot, "-Pack", job.Pack,
		"-ResumeCurrentLoop", "-ExpectedCurrentLoopCheckpointSha256", job.CheckpointSHA256,
		"-ExpectedExternalSessionJobSha256", inspection.Ticket.JobSHA256,
		"-ExpectedExternalSessionAttemptSha256", attemptSHA256,
		"-ExpectedExternalSessionDispatchSha256", inspection.TicketSHA256,
	}
	state := "ready-for-dispatch-claim-review"
	label := job.SessionKind + " external session dispatch claim"
	if outcome == "claimed" {
		args = append(args, "-ClaimExternalSessionDispatch", "-ExternalSessionActor", "<actor>", "-ExternalSessionObservedAt", "<rfc3339nano>")
	} else {
		state = "ready-for-launch-receipt-review"
		label = job.SessionKind + " external session launch " + outcome
		args = append(args, "-RecordExternalSessionLaunch", "-ExpectedExternalSessionClaimSha256", inspection.ClaimSHA256,
			"-ExternalSessionLaunchOutcome", outcome, "-ExternalSessionActor", "<actor>", "-ExternalSessionObservedAt", "<rfc3339nano>")
		if outcome == "accepted" {
			args = append(args, "-ExternalSessionHarness", "<actual-harness>", "-ExternalSessionId", "<actual-session>")
		} else {
			args = append(args, "-ExternalSessionLaunchReason", "<reason>")
		}
	}
	args = append(args, "-WhatIf", "-Format", "json")
	return mission.MissionCommanderDriverRequest{
		Kind: "preview-command-template", RunLoopStepID: "external-session-dispatch:" + job.JobID + ":" + outcome,
		Actor: "mission-commander", State: state, Source: "current-loop-external-session-dispatch", Lane: memberLane(job),
		Label: label, Command: joinDriverCommand(args), CommandExecutable: false, RequiresReview: true,
		ExpectedReceipt: mission.MissionCommanderDriverReceiptExpectation{
			State: "dispatch-plan-ready", RefreshStatusCommand: statusMissionControlRefreshCommand(job.CaseRoot),
			Description: "returns an exact immutable external session dispatch transition plan",
			Boundary:    []string{"replace placeholders before execution; WhatIf writes nothing", "Apply records claim or launch observation only"},
		},
		Boundary: []string{"the external harness owns actual launch", "dispatch receipts do not grant authority or execute heavy tools"},
	}
}

func externalSessionDispatchApplyCommand(plan externalsession.DispatchPlan) string {
	args := []string{
		"/rekit", "run-current-loop", "-Target", plan.Job.CaseRoot, "-Pack", plan.Job.Pack,
		"-ResumeCurrentLoop", "-ExpectedCurrentLoopCheckpointSha256", plan.Job.CheckpointSHA256,
		"-ExpectedExternalSessionJobSha256", plan.JobSHA256,
		"-ExpectedExternalSessionAttemptSha256", plan.AttemptSHA256,
		"-ExpectedExternalSessionDispatchSha256", plan.DispatchSHA256,
	}
	if plan.Outcome == "claimed" {
		args = append(args, "-ClaimExternalSessionDispatch", "-ExternalSessionActor", plan.Actor,
			"-ExternalSessionObservedAt", plan.ObservedAt)
	} else {
		args = append(args, "-RecordExternalSessionLaunch", "-ExpectedExternalSessionClaimSha256", plan.ExpectedClaimSHA256,
			"-ExternalSessionLaunchOutcome", plan.Outcome, "-ExternalSessionActor", plan.Actor,
			"-ExternalSessionObservedAt", plan.ObservedAt)
		if plan.Outcome == "accepted" {
			args = append(args, "-ExternalSessionHarness", plan.ActualHarness, "-ExternalSessionId", plan.ActualSession)
		} else {
			args = append(args, "-ExternalSessionLaunchReason", plan.Reason)
		}
	}
	args = append(args, "-ExpectedExternalSessionDispatchPlanSha256", plan.ExpectedPlanSHA256, "-Apply", "-Format", "json")
	return joinDriverCommand(args)
}
