package cli

import (
	"fmt"
	"io"
	"strings"

	"github.com/shuiyu486/re-context-kits/internal/rekit/executioncontrol"
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
	status, err := buildInvocationStatusInventory(ctx, opt)
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
		plan, err = externalsession.ResolveAttemptApplyPlanUnbound(job, opt.ExternalSessionHarness, opt.ExternalSessionID, opt.ExternalSessionActor, opt.ExternalSessionStartedAt, opt.ExpectedExternalSessionAttemptSHA256)
	} else {
		plan, err = externalsession.PreviewAttempt(job, opt.ExternalSessionHarness, opt.ExternalSessionID, opt.ExternalSessionActor, opt.ExternalSessionStartedAt, opt.ExpectedExternalSessionAttemptSHA256)
	}
	if err != nil {
		return err
	}
	if err := requireExternalMemberAttemptBirthControl(ctx.Target, *checkpoint, plan); err != nil {
		return err
	}
	inspection, err := externalsession.Inspect(job)
	if err != nil {
		return err
	}
	ticket, err := externalSessionDispatchTicket(job, inspection, plan, status.MissionControlRunbook.CurrentLoopOperator)
	if err != nil {
		return err
	}
	plan, err = externalsession.BindAttemptDispatch(plan, ticket)
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
		plan.ApplyCommand = selectedLaneCommand(
			externalSessionAttemptApplyCommand(plan),
			opt.SelectedCurrentLane,
		)
		return writeJSON(out, plan)
	}
	applied, err := externalsession.ApplyAttemptCurrent(plan, opt.ExpectedExternalSessionJobSHA256, opt.ExpectedExternalSessionAttemptPlanSHA256, func() (externalsession.Job, error) {
		return currentExternalSessionJob(ctx, opt)
	})
	if err != nil {
		return err
	}
	result := currentLoopExternalSessionAttemptResult{AttemptPlan: applied}
	fresh, refreshErr := buildInvocationStatusInventory(ctx, opt)
	if refreshErr == nil {
		result.RefreshedStatus = &fresh
	}
	return writeJSON(out, result)
}

func externalSessionDispatchTicket(job externalsession.Job, inspection externalsession.Inspection, plan externalsession.AttemptPlan, operator *mission.CurrentLoopOperatorPackage) (externalsession.DispatchTicket, error) {
	attempt := externalsession.AttemptInspection{
		State: "running", Current: &plan.Attempt, AttemptSHA256: plan.AttemptSHA256,
		Path: plan.AttemptPath, Generations: plan.Attempt.Generation,
	}
	typedAttempt := &mission.CurrentLoopExternalSessionAttempt{
		AttemptID: plan.Attempt.AttemptID, AttemptSHA256: plan.AttemptSHA256,
		Generation: plan.Attempt.Generation, Harness: plan.Attempt.Harness, Session: plan.Attempt.Session,
		Actor: plan.Attempt.Actor, StartedAt: plan.Attempt.StartedAt, SupersedesSHA256: plan.Attempt.SupersedesSHA256,
		Path: plan.AttemptPath, SubmissionPath: plan.Attempt.SubmissionPath,
		SubmissionOutputs: plan.Attempt.SubmissionOutputs, SubmissionResult: plan.Attempt.SubmissionResult,
		LaunchControl: executioncontrol.CloneBinding(plan.Attempt.LaunchControl),
	}
	typed := mission.CurrentLoopExternalSessionJob{
		CurrentAttempt: typedAttempt, SubmissionPath: plan.Attempt.SubmissionPath,
		SubmissionOutputs: plan.Attempt.SubmissionOutputs, SubmissionResult: plan.Attempt.SubmissionResult,
		Dispatcher: &mission.CurrentLoopExternalSessionDispatcher{
			Ticket: &mission.CurrentLoopExternalSessionDispatchTicket{},
		},
	}
	attemptInspection := inspection
	attemptInspection.State = "awaiting-submission"
	attemptInspection.SubmissionSHA256 = ""
	attemptInspection.Submission = nil
	attemptInspection.Warnings = nil
	pkg := externalSessionHarnessPackage(job, attemptInspection, attempt, operator, typed)
	if pkg == nil || pkg.Launch == nil || !pkg.Launch.Ready || pkg.Return == nil {
		return externalsession.DispatchTicket{}, fmt.Errorf("external session attempt dispatch input is not launch-ready")
	}
	ticket := externalsession.DispatchTicket{
		Launch: externalsession.DispatchLaunch{
			Ready: pkg.Launch.Ready, Tool: pkg.Launch.Tool, AgentType: pkg.Launch.AgentType,
			ReadOnly: pkg.Launch.ReadOnly, Capability: pkg.Launch.Capability,
			ExpectedOutput:      pkg.Launch.ExpectedOutput,
			InstructionIdentity: cloneInstructionIdentity(pkg.Launch.InstructionIdentity),
			Input: externalsession.DispatchInput{
				Path: pkg.Launch.Input.Path, SHA256: pkg.Launch.Input.SHA256, Role: pkg.Launch.Input.Role,
			},
			Boundary: append([]string{}, pkg.Launch.Boundary...),
		},
		Return: externalsession.DispatchReturn{
			SubmissionPath: pkg.Return.SubmissionPath, SubmissionOutputs: pkg.Return.SubmissionOutputs,
			SubmissionResult: pkg.Return.SubmissionResult, SubmissionLast: pkg.Return.SubmissionLast,
			Boundary: append([]string{}, pkg.Return.Boundary...),
		},
		RefreshStatusCommand: pkg.RefreshStatusCommand,
		Boundary:             append([]string{}, pkg.Boundary...),
	}
	for _, template := range pkg.Return.Templates {
		replacements := append([]string{}, template.RequiredReplace...)
		replacements = mission.UniqueStrings(append(replacements, "<attempt-receipt-sha256>"))
		ticket.Return.Templates = append(ticket.Return.Templates, externalsession.DispatchSubmissionTemplate{
			Outcome: template.Outcome,
			JSON: strings.ReplaceAll(
				template.JSON,
				plan.AttemptSHA256,
				"<attempt-receipt-sha256>",
			),
			RequiredWrites:       append([]string{}, template.RequiredWrites...),
			RequiredReplacements: replacements,
		})
	}
	return ticket, nil
}

func externalSessionPendingAttemptRequest(plan externalsession.AttemptPlan) (mission.MissionCommanderDriverRequest, error) {
	refresh, err := statusMissionControlRefreshCommand(plan.Job.CaseRoot)
	if err != nil {
		return mission.MissionCommanderDriverRequest{}, err
	}
	plan.ApplyCommand = externalSessionAttemptApplyCommand(plan)
	return mission.MissionCommanderDriverRequestWithTypedCommand(
		mission.MissionCommanderDriverRequest{
			Kind: "apply-command", RunLoopStepID: "external-session-attempt-recovery:" + plan.Job.JobID,
			Actor: "mission-commander", State: "attempt-publication-pending", Source: "current-loop-external-session-dispatch",
			Lane: memberLane(plan.Job), Label: plan.Job.SessionKind + " external session attempt recovery",
			Command: plan.ApplyCommand, CommandExecutable: true, RequiresReview: true,
			ExpectedReceipt: mission.MissionCommanderDriverReceiptExpectation{
				State: "attempt-committed", RefreshStatusCommand: refresh,
				Description: "completes the exact ticket-first attempt publication by writing the attempt receipt last",
				Boundary:    []string{"immutable pending ticket fixes every attempt parameter", "different bytes or stale currentness fail closed"},
			},
			Boundary: []string{"recovery does not start or manage a session", "attempt receipt is the final commit point"},
		},
	)
}

func externalSessionAttemptRequest(job externalsession.Job, jobSHA string, inspection externalsession.AttemptInspection) (mission.MissionCommanderDriverRequest, error) {
	refresh, err := statusMissionControlRefreshCommand(job.CaseRoot)
	if err != nil {
		return mission.MissionCommanderDriverRequest{}, err
	}
	supersedes := inspection.AttemptSHA256
	if inspection.Current != nil && job.SessionKind == "reviewer" && job.Reviewer != nil && job.Reviewer.DispatchID != "" {
		return mission.MissionCommanderDriverRequest{
			Kind: "model-guidance", RunLoopStepID: "external-reviewer-redispatch:" + job.JobID,
			Actor: "mission-commander", State: "new-reviewer-dispatch-required", Source: "current-loop-external-session-job",
			Lane: memberLane(job), Label: "new durable Reviewer dispatch required", ActionID: inspection.AttemptSHA256,
			Guidance:          "Do not replace this durable Reviewer job. Create a new Reviewer dispatch with a new reviewerSession binding; the canonical dispatch path will produce a new external-session job.",
			CommandExecutable: false, RequiresReview: true,
			ExpectedReceipt: mission.MissionCommanderDriverReceiptExpectation{
				State: "new-reviewer-dispatch-required", RefreshStatusCommand: refresh,
				Description: "re-enters the durable Reviewer dispatch path rather than violating the current job's fixed harness/session identity",
				Boundary:    []string{"no same-job Reviewer replacement", "new dispatch/session/job identity required", "current attempt remains fenced by its generation"},
			},
			Boundary: []string{"the current Reviewer job identity is fixed by its durable dispatch receipt", "no session launch, transport resend, authority, confirmed state, or heavy tool is performed"},
		}, nil
	}
	harness, session := "<harness>", "<session-id>"
	if job.SessionKind == "reviewer" && job.Reviewer != nil {
		harness, session = job.Reviewer.Harness, job.Reviewer.Session
	}
	args := []string{
		"/rekit", "run-current-loop", "-Target", job.CaseRoot, "-Pack", job.Pack,
		"-ResumeCurrentLoop", "-ExpectedCurrentLoopCheckpointSha256", job.CheckpointSHA256,
		"-RecordExternalSessionAttempt", "-ExpectedExternalSessionJobSha256", jobSHA,
		"-ExternalSessionHarness", harness, "-ExternalSessionId", session,
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
	return mission.MissionCommanderDriverRequestWithTypedCommand(
		mission.MissionCommanderDriverRequest{
			Kind: "preview-command-template", RunLoopStepID: "external-session-attempt:" + job.JobID,
			Actor: "mission-commander", State: state, Source: "current-loop-external-session-job",
			Lane: memberLane(job), Label: job.SessionKind + " external session attempt", Command: command,
			CommandExecutable: false, RequiresReview: true,
			ExpectedReceipt: mission.MissionCommanderDriverReceiptExpectation{
				State: "attempt-plan-ready", RefreshStatusCommand: refresh,
				Description: description,
				Boundary:    []string{"replace placeholders before execution; WhatIf writes nothing", "Apply records ownership but does not start or manage a session"},
			},
			Boundary: []string{"only the external harness starts, polls, stops, and reports the session", "local sessionhost remains the default concrete provider; Remote Control requires a prior reviewer dispatch receipt with harness claude-code-remote-control and an explicit durable transport binding id", "replacement requires exact current attempt sha256"},
		},
	)
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
