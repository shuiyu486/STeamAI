package cli

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"path/filepath"
	"strings"

	"github.com/shuiyu486/re-context-kits/internal/rekit/currentloop"
	"github.com/shuiyu486/re-context-kits/internal/rekit/externalsession"
	rekitfs "github.com/shuiyu486/re-context-kits/internal/rekit/fs"
	"github.com/shuiyu486/re-context-kits/internal/rekit/memberexecution"
	"github.com/shuiyu486/re-context-kits/internal/rekit/mission"
	"github.com/shuiyu486/re-context-kits/internal/rekit/reviewersession"
	"github.com/shuiyu486/re-context-kits/internal/rekit/runtime"
)

func runCurrentLoopExternalSessionRelay(ctx runtime.Context, opt Options, out io.Writer) error {
	if opt.WhatIf == opt.Apply {
		return fmt.Errorf("run-current-loop external session relay requires exactly one of -WhatIf or -Apply")
	}
	if !opt.ResumeCurrentLoop || strings.TrimSpace(opt.ExpectedCurrentLoopCheckpointSHA256) == "" {
		return fmt.Errorf("run-current-loop external session relay requires -ResumeCurrentLoop and -ExpectedCurrentLoopCheckpointSha256")
	}
	if opt.MaxSteps != 0 || strings.TrimSpace(opt.ExpectedCurrentLoopPlanSHA256) != "" || strings.TrimSpace(opt.CurrentLoopObservationPath) != "" || currentStepHasMemberObservation(opt) || currentStepHasReviewerObservation(opt) || strings.TrimSpace(opt.Start.Actor) != "" {
		return fmt.Errorf("run-current-loop external session relay cannot combine resume execution or legacy observation flags")
	}
	if strings.ToLower(strings.TrimSpace(opt.Format)) != "json" {
		return fmt.Errorf("run-current-loop external session relay supports only -Format json")
	}
	if err := validateCurrentLoopOuterArgs(opt); err != nil {
		return err
	}
	status, err := buildInvocationStatusInventory(ctx, opt)
	if err != nil {
		return err
	}
	if status.MissionControlRunbook == nil || status.MissionControlRunbook.CurrentLoopOperator == nil || status.MissionControlRunbook.CurrentLoopOperator.ExternalSessionJob == nil || status.MissionControlRunbook.CurrentLoopSegment == nil {
		return fmt.Errorf("run-current-loop external session relay requires a current externalSessionJob")
	}
	inspection := status.MissionControlRunbook.CurrentLoopSegment
	if !inspection.Ready || inspection.State != "ready" || !strings.EqualFold(inspection.ArtifactSHA256, opt.ExpectedCurrentLoopCheckpointSHA256) {
		return fmt.Errorf("run-current-loop external session relay checkpoint is not the latest ready checkpoint")
	}
	job, err := externalSessionJobFor(ctx.Target, status.MissionControlRunbook.CurrentLoopOperator, *inspection)
	if err != nil {
		return err
	}
	plan, err := externalsession.Preview(job)
	if err != nil {
		return err
	}
	if opt.WhatIf {
		if strings.TrimSpace(opt.ExpectedExternalSessionSubmissionSHA256) != "" || strings.TrimSpace(opt.ExpectedExternalSessionRelayPlanSHA256) != "" {
			return fmt.Errorf("external session relay -WhatIf does not accept submission or relay plan hashes")
		}
		if expected := strings.TrimSpace(opt.ExpectedExternalSessionJobSHA256); expected == "" || !strings.EqualFold(expected, plan.JobSHA256) {
			return fmt.Errorf("external session relay expected job sha256 mismatch: got %s want %s", expected, plan.JobSHA256)
		}
		plan.ApplyCommand = externalSessionRelayApplyCommand(plan)
		return writeJSON(out, plan)
	}
	applied, err := externalsession.ApplyCurrent(plan, opt.ExpectedExternalSessionJobSHA256, opt.ExpectedExternalSessionSubmissionSHA256, opt.ExpectedExternalSessionRelayPlanSHA256, func() (externalsession.Job, error) {
		return currentExternalSessionJob(ctx, opt)
	})
	if err != nil {
		return err
	}
	fresh, refreshErr := buildInvocationStatusInventory(ctx, opt)
	result := currentLoopExternalSessionRelayResult{Plan: applied}
	if refreshErr == nil && fresh.MissionControlRunbook != nil {
		applied.Boundary = append(applied.Boundary, "status refreshed after publication; consume the unique observation inbox selected request before checkpoint resume")
		result.Plan = applied
		result.RefreshedStatus = &fresh
	}
	return writeJSON(out, result)
}

type currentLoopExternalSessionRelayResult struct {
	externalsession.Plan
	RefreshedStatus *statusInventory `json:"refreshedStatus,omitempty"`
}

func currentExternalSessionJob(ctx runtime.Context, opt Options) (externalsession.Job, error) {
	status, err := buildInvocationStatusInventory(ctx, opt)
	if err != nil {
		return externalsession.Job{}, err
	}
	if status.MissionControlRunbook == nil || status.MissionControlRunbook.CurrentLoopOperator == nil || status.MissionControlRunbook.CurrentLoopOperator.ExternalSessionJob == nil || status.MissionControlRunbook.CurrentLoopSegment == nil {
		return externalsession.Job{}, fmt.Errorf("current externalSessionJob is unavailable")
	}
	checkpoint := status.MissionControlRunbook.CurrentLoopSegment
	if !checkpoint.Ready || checkpoint.State != "ready" {
		return externalsession.Job{}, fmt.Errorf("current externalSessionJob checkpoint is not ready")
	}
	return externalSessionJobFor(ctx.Target, status.MissionControlRunbook.CurrentLoopOperator, *checkpoint)
}

func bindExternalSessionJob(target string, pkg *mission.CurrentLoopOperatorPackage, inspection currentloop.Inspection) {
	if pkg == nil || !inspection.Ready || inspection.State != "ready" || inspection.Continuation == nil || inspection.Continuation.ObservationContract == nil {
		return
	}
	job, err := externalSessionJobFor(target, pkg, inspection)
	if err != nil {
		return
	}
	job.DispatchRequired = true
	inspected, err := externalsession.Inspect(job)
	if err != nil {
		return
	}
	attempt, err := externalsession.InspectAttempt(job)
	if err != nil {
		return
	}
	dispatch, err := externalsession.InspectDispatch(job, attempt)
	if err != nil {
		pkg.Ready = false
		pkg.State = "external-session-dispatch-invalid"
		pkg.Boundary = mission.UniqueStrings(append(pkg.Boundary, err.Error()))
		return
	}
	typed := missionExternalSessionJob(inspected)
	typed.Dispatcher = externalSessionDispatcherPackage(job, attempt.AttemptSHA256, dispatch)
	bindExternalSessionRequestLane(&typed, pkg.Lane)
	if dispatch.State == "attempt-publication-pending" && inspected.State != "submission-ready" && (pkg.ObservationInbox == nil || pkg.ObservationInbox.State == "empty") {
		pending, pendingErr := externalsession.PendingAttemptPlan(job, dispatch)
		if pendingErr != nil {
			pkg.Ready = false
			pkg.State = "external-session-dispatch-invalid"
			pkg.Boundary = mission.UniqueStrings(append(pkg.Boundary, pendingErr.Error()))
			return
		}
		request := externalSessionPendingAttemptRequest(pending)
		typed.AttemptState = dispatch.State
		typed.AttemptRequest = &request
		typed.Dispatcher.ClaimRequest = nil
		pkg.State = "external-session-attempt-publication-pending"
		pkg.SelectedDriverRequest = &request
		pkg.ExternalSessionJob = &typed
		pkg.RunbookSteps = mission.UniqueStrings(append(pkg.RunbookSteps,
			"recover the exact ticket-first attempt publication; do not choose new owner parameters",
			"execute only the hash-bound Apply command reconstructed from the immutable pending ticket",
		))
		return
	}
	typed.AttemptState = attempt.State
	if attempt.Current != nil {
		typed.CurrentAttempt = &mission.CurrentLoopExternalSessionAttempt{
			AttemptID: attempt.Current.AttemptID, AttemptSHA256: attempt.AttemptSHA256,
			Generation: attempt.Current.Generation, Harness: attempt.Current.Harness, Session: attempt.Current.Session,
			Actor: attempt.Current.Actor, StartedAt: attempt.Current.StartedAt, SupersedesSHA256: attempt.Current.SupersedesSHA256,
			Path: attempt.Path, SubmissionPath: attempt.Current.SubmissionPath,
			SubmissionOutputs: attempt.Current.SubmissionOutputs, SubmissionResult: attempt.Current.SubmissionResult,
		}
		typed.SubmissionPath = attempt.Current.SubmissionPath
		typed.SubmissionOutputs = attempt.Current.SubmissionOutputs
		typed.SubmissionResult = attempt.Current.SubmissionResult
	}
	defer func() {
		bindExternalSessionRequestLane(&typed, pkg.Lane)
		typed.HarnessPackage = externalSessionHarnessPackage(job, inspected, attempt, pkg, typed)
		if inspected.State == "awaiting-submission" && (pkg.ObservationInbox == nil || pkg.ObservationInbox.State == "empty") {
			applyExternalSessionDispatchState(pkg, &typed, dispatch)
		}
		pkg.ExternalSessionJob = &typed
	}()
	pkg.ExternalSessionJob = &typed
	if pkg.ObservationInbox != nil && pkg.ObservationInbox.State != "empty" {
		return
	}
	switch inspected.State {
	case "submission-ready":
		relayRequest := externalSessionRelayRequest(job, inspected.JobSHA256)
		turnRequest := externalSessionTurnRequest(job, inspected.JobSHA256)
		if strings.TrimSpace(relayRequest.Lane) == "" {
			relayRequest.Lane = pkg.Lane
		}
		if strings.TrimSpace(turnRequest.Lane) == "" {
			turnRequest.Lane = pkg.Lane
		}
		typed.RelayPreviewRequest = &relayRequest
		pkg.ExternalSessionJob = &typed
		pkg.State = "external-session-turn-review-required"
		pkg.SelectedDriverRequest = &turnRequest
		pkg.RunbookSteps = []string{
			"review externalSessionJob and execute the selected external-result turn preview exactly",
			"review the exact relay artifacts, observation, nested resume plan, and remaining budget, then execute only its hash-bound Apply",
			"refresh status and stop at the next typed external, Human-in-the-Lane, route/lane, budget, or terminal boundary",
			"use externalSessionJob.relayPreviewRequest only for relay-only recovery or diagnosis",
		}
	case "awaiting-submission":
		attemptRequest := externalSessionAttemptRequest(job, inspected.JobSHA256, attempt)
		typed.AttemptRequest = &attemptRequest
		pkg.ExternalSessionJob = &typed
		pkg.SelectedDriverRequest = &attemptRequest
		if attempt.Current == nil {
			pkg.State = "external-session-ready-for-attempt"
			pkg.RunbookSteps = mission.UniqueStrings(append(pkg.RunbookSteps,
				"review the externalSessionJob attemptRequest and substitute a concrete harness/session/actor/startedAt before WhatIf",
				"review and Apply the returned exact attempt hash; the external harness starts the session outside the Go runtime",
				"write outputs or reviewer-result first, then write submissionPath last with the current attempt identity",
			))
		} else {
			pkg.State = "external-session-running"
			pkg.RunbookSteps = mission.UniqueStrings(append(pkg.RunbookSteps,
				"the current durable attempt owns this job; wait for its submission or explicitly record a replacement attempt",
				"a replacement must use attemptRequest with the exact current attempt sha256 and a distinct harness or session",
				"write outputs or reviewer-result first, then write submissionPath last with the current attempt identity",
			))
		}
	case "invalid":
		pkg.Ready = false
		pkg.State = "external-session-submission-invalid"
		pkg.ResumeDriverRequest = nil
		if attempt.Current != nil {
			replacementRequest := externalSessionAttemptRequest(job, inspected.JobSHA256, attempt)
			typed.AttemptRequest = &replacementRequest
			pkg.ExternalSessionJob = &typed
			pkg.SelectedDriverRequest = &replacementRequest
		} else {
			pkg.SelectedDriverRequest = nil
		}
		pkg.Boundary = mission.UniqueStrings(append(pkg.Boundary, inspected.Warnings...))
	}
}

func externalSessionJobFor(target string, pkg *mission.CurrentLoopOperatorPackage, inspection currentloop.Inspection) (externalsession.Job, error) {
	allowed := currentLoopExternalSessionAllowedOutcomes(inspection)
	if inspection.StopCode == "external-member-handoff" && inspection.Continuation.ExternalMemberHandoff != nil {
		expected := inspection.Continuation.ExternalMemberHandoff
		memberInspection, err := memberexecution.Inspect(target, expected.Lane, expected.AttemptID)
		if err != nil || memberInspection.Handoff == nil || memberInspection.State != expected.State || memberInspection.Owner.Lane != expected.Lane || memberInspection.Owner.Executor != expected.Executor || memberInspection.Owner.ExecutorGeneration != expected.ExecutorGeneration {
			return externalsession.Job{}, fmt.Errorf("external session member attempt no longer matches checkpoint")
		}
		job, err := externalsession.NewMemberJob(target, pkg.Pack, inspection.ArtifactSHA256, expected.AttemptID, memberInspection.Owner, memberInspection.Handoff.ManifestPath, memberInspection.Handoff.OutputsRoot, allowed)
		job.DispatchRequired = err == nil
		return job, err
	}
	if pkg.ExternalReviewerHandoff == nil || pkg.ExternalReviewerHandoff.Attempt == nil {
		return externalsession.Job{}, fmt.Errorf("external session job requires a current member or reviewer handoff")
	}
	attempt := pkg.ExternalReviewerHandoff.Attempt
	receipt, err := reviewersession.ReadDispatch(target, attempt.Receipt.DispatchPath, attempt.Receipt.DispatchSHA256)
	if err != nil {
		return externalsession.Job{}, err
	}
	fields, err := reviewersession.OutputContractFields(target, receipt)
	if err != nil {
		return externalsession.Job{}, err
	}
	if receipt.PacketID != attempt.Identity.PacketID || receipt.RouteID != attempt.Identity.RouteID ||
		receipt.ShardID != attempt.Identity.ShardID || receipt.DispatchID != attempt.Receipt.DispatchID ||
		receipt.ReviewerHarness != attempt.Receipt.Harness || receipt.ReviewerSession != attempt.Receipt.Session {
		return externalsession.Job{}, fmt.Errorf("external session reviewer job does not match exact durable dispatch receipt")
	}
	reviewer := externalsession.ReviewerIdentity{
		AttemptSHA256: attempt.AttemptSnapshotSHA256,
		PacketID:      receipt.PacketID, RouteID: receipt.RouteID, ShardID: receipt.ShardID,
		Items: append([]string{}, receipt.Items...), OutputFields: fields,
		DispatchPath: attempt.Receipt.DispatchPath, DispatchSHA256: attempt.Receipt.DispatchSHA256,
		DispatchID: receipt.DispatchID, Harness: receipt.ReviewerHarness, Session: receipt.ReviewerSession,
	}
	job, err := externalsession.NewReviewerJob(target, pkg.Pack, inspection.ArtifactSHA256, reviewer, allowed)
	job.DispatchRequired = err == nil
	return job, err
}

func bindExternalSessionRequestLane(job *mission.CurrentLoopExternalSessionJob, lane string) {
	if job == nil || strings.TrimSpace(lane) == "" {
		return
	}
	requests := []*mission.MissionCommanderDriverRequest{job.AttemptRequest, job.RelayPreviewRequest}
	if job.Dispatcher != nil {
		requests = append(requests, job.Dispatcher.ClaimRequest, job.Dispatcher.LaunchAcceptedRequest, job.Dispatcher.LaunchFailedRequest)
	}
	for _, request := range requests {
		if request != nil && strings.TrimSpace(request.Lane) == "" {
			request.Lane = lane
		}
	}
}

func sameExternalSessionPath(left, right string) bool {
	leftPath, leftErr := filepath.Abs(left)
	rightPath, rightErr := filepath.Abs(right)
	return leftErr == nil && rightErr == nil && strings.EqualFold(
		filepath.Clean(leftPath),
		filepath.Clean(rightPath),
	)
}

func externalReviewerLaunchIdentity(
	caseRoot string,
	handoff *mission.CurrentLoopExternalReviewerHandoff,
	launch *mission.CurrentLoopExternalSessionHarnessLaunch,
) (*mission.CurrentLoopExternalSessionReviewerIdentity, error) {
	if handoff == nil || handoff.Attempt == nil || launch == nil ||
		strings.TrimSpace(handoff.Attempt.Receipt.DispatchPath) == "" ||
		strings.TrimSpace(handoff.Attempt.Receipt.DispatchSHA256) == "" {
		return nil, fmt.Errorf("current reviewer handoff omitted durable dispatch receipt identity")
	}
	dispatchPath := externalSessionHarnessInputPath(
		caseRoot,
		handoff.Attempt.Receipt.DispatchPath,
	)
	data, err := rekitfs.ReadStableRegularFileAnchored(
		caseRoot,
		dispatchPath,
		"external reviewer dispatch receipt",
		1<<20,
	)
	if err != nil || !strings.EqualFold(
		externalSessionBytesSHA(data),
		handoff.Attempt.Receipt.DispatchSHA256,
	) {
		return nil, fmt.Errorf("external reviewer dispatch receipt is missing or changed")
	}
	receipt, err := reviewersession.DecodeDispatch(data)
	if err != nil {
		return nil, err
	}
	fields, err := reviewersession.OutputContractFields(caseRoot, receipt)
	if err != nil {
		return nil, err
	}
	identity := handoff.Attempt.Identity
	if receipt.PacketID != identity.PacketID || receipt.RouteID != identity.RouteID ||
		receipt.ShardID != identity.ShardID || receipt.DispatchID != handoff.Attempt.Receipt.DispatchID ||
		receipt.ReviewerSession != handoff.Attempt.Receipt.Session ||
		!strings.EqualFold(receipt.PromptSHA256, launch.Input.SHA256) ||
		!sameExternalSessionPath(receipt.PromptPath, launch.Input.Path) ||
		!receipt.ReadOnly || !receipt.NoHeavyTool || !receipt.NoAuthority {
		return nil, fmt.Errorf("external reviewer dispatch receipt does not match current attempt and launch")
	}
	return &mission.CurrentLoopExternalSessionReviewerIdentity{
		PacketID: receipt.PacketID, RouteID: receipt.RouteID,
		ShardID: receipt.ShardID, Items: append([]string{}, receipt.Items...),
		OutputFields:   append([]string{}, fields...),
		DispatchPath:   handoff.Attempt.Receipt.DispatchPath,
		DispatchSHA256: handoff.Attempt.Receipt.DispatchSHA256,
		DispatchID:     receipt.DispatchID, ReviewerSession: receipt.ReviewerSession,
		PromptPath: receipt.PromptPath, PromptSHA256: receipt.PromptSHA256,
		NoHeavyTool: receipt.NoHeavyTool, NoAuthority: receipt.NoAuthority,
	}, nil
}

func externalSessionHarnessPackage(job externalsession.Job, inspection externalsession.Inspection, attempt externalsession.AttemptInspection, operator *mission.CurrentLoopOperatorPackage, typed mission.CurrentLoopExternalSessionJob) *mission.CurrentLoopExternalSessionHarnessPackage {
	pkg := &mission.CurrentLoopExternalSessionHarnessPackage{
		SchemaVersion: 1, State: "attempt-review-required", CaseRoot: job.CaseRoot,
		JobID: job.JobID, JobSHA256: inspection.JobSHA256, CheckpointSHA256: job.CheckpointSHA256, SessionKind: job.SessionKind,
		AttemptReviewRequest: typed.AttemptRequest, RefreshStatusCommand: statusMissionControlRefreshCommand(job.CaseRoot),
		Boundary: []string{
			"the main Agent reviews and records the attempt before the external harness starts a session",
			"the external harness owns session start, polling, stop, and result production; Go records only request, receipt, and state",
			"write result artifacts first and the exact submissionPath last; never write authority/confirmed or execute heavy tools from this package",
		},
	}
	if attempt.Current == nil {
		return pkg
	}
	input := mission.CurrentLoopExternalSessionHarnessInput{Role: "durable-member-handoff"}
	launch := &mission.CurrentLoopExternalSessionHarnessLaunch{
		Ready: true, Tool: "Claude Code session", AgentType: "durable-member-executor", ReadOnly: false,
		ExpectedOutput: "bounded member outputs and one strict external session submission JSON object",
		Attempt:        *typed.CurrentAttempt,
		Boundary:       []string{"start or reconnect idempotently by the exact harness/session identity; never create a second session for the same attempt", "execute only for the exact current attempt and owner generation", "do not reuse this launch after replacement or checkpoint drift"},
	}
	switch job.SessionKind {
	case "member":
		if operator.ExternalMemberHandoff == nil {
			launch.Ready = false
			pkg.Warnings = append(pkg.Warnings, "current durable member handoff is unavailable; refresh status and do not launch")
			break
		}
		input.Path = operator.ExternalMemberHandoff.TaskContextPath
		input.SHA256 = operator.ExternalMemberHandoff.TaskContextSHA256
		memberInspection, err := memberexecution.Inspect(job.CaseRoot, operator.ExternalMemberHandoff.Lane, operator.ExternalMemberHandoff.AttemptID)
		if err != nil || memberexecution.ValidateActionableTaskContext(job.CaseRoot, memberInspection) != nil {
			launch.Ready = false
			pkg.Warnings = append(pkg.Warnings, "current immutable member task context is stale; refresh status and do not launch")
			break
		}
		data, err := rekitfs.ReadStableRegularFileAnchored(job.CaseRoot, externalSessionHarnessInputPath(job.CaseRoot, input.Path), "external member task context", 1<<20)
		if err != nil {
			launch.Ready = false
			pkg.Warnings = append(pkg.Warnings, "current immutable member task context cannot be read as a stable case-anchored regular file; refresh status and do not launch")
			break
		}
		if input.Path == "" || input.SHA256 == "" || !strings.EqualFold(externalSessionBytesSHA(data), input.SHA256) {
			launch.Ready = false
			pkg.Warnings = append(pkg.Warnings, "current immutable member task context does not match its committed sha256; refresh status and do not launch")
		}
	case "reviewer":
		launch.Tool = "Claude Code Agent"
		launch.AgentType = "read-only-reviewer"
		launch.ReadOnly = true
		launch.ExpectedOutput = "exactly one ReviewerResult JSON object; no Markdown fence or surrounding prose"
		input.Role = "reviewer-dispatch-prompt"
		if operator.ExternalReviewerHandoff == nil {
			launch.Ready = false
			pkg.Warnings = append(pkg.Warnings, "current reviewer handoff is unavailable; refresh status and do not launch")
			break
		}
		input.Path = operator.ExternalReviewerHandoff.DispatchPromptPath
		input.SHA256 = operator.ExternalReviewerHandoff.DispatchPromptSHA256
		if request := operator.ExternalReviewerHandoff.AgentToolRequest; request != nil {
			launch.Tool, launch.AgentType, launch.ReadOnly = request.Tool, request.AgentType, request.ReadOnly
			launch.ExpectedOutput = request.ExpectedOutput
		}
		if input.Path == "" || input.SHA256 == "" {
			launch.Ready = false
			pkg.Warnings = append(pkg.Warnings, "current reviewer prompt path or sha256 is unavailable; refresh status and do not launch")
			break
		}
		data, err := rekitfs.ReadStableRegularFileAnchored(job.CaseRoot, externalSessionHarnessInputPath(job.CaseRoot, input.Path), "external reviewer harness input", 1<<20)
		if err != nil || !strings.EqualFold(externalSessionBytesSHA(data), input.SHA256) {
			launch.Ready = false
			pkg.Warnings = append(pkg.Warnings, "current reviewer prompt is unavailable or does not match its dispatch sha256; refresh status and do not launch")
			break
		}
		launch.Input = input
		identity, identityErr := externalReviewerLaunchIdentity(job.CaseRoot, operator.ExternalReviewerHandoff, launch)
		if identityErr != nil {
			launch.Ready = false
			pkg.Warnings = append(pkg.Warnings, "current reviewer dispatch receipt is unavailable or does not match the exact launch identity; refresh status and do not launch")
			break
		}
		launch.ReviewerIdentity = identity
	}
	launch.Input = input
	pkg.Launch = launch
	pkg.State = "running"
	pkg.Return = externalSessionReturnContract(job, inspection, attempt, operator, typed)
	switch inspection.State {
	case "submission-ready":
		pkg.State = "return-review-required"
		pkg.Launch.Ready = false
	case "invalid":
		pkg.State = "return-invalid-replacement-review"
		pkg.Launch.Ready = false
	}
	return pkg
}

func externalSessionReturnContract(job externalsession.Job, inspection externalsession.Inspection, attempt externalsession.AttemptInspection, operator *mission.CurrentLoopOperatorPackage, typed mission.CurrentLoopExternalSessionJob) *mission.CurrentLoopExternalSessionReturnContract {
	if attempt.Current == nil {
		return nil
	}
	contract := &mission.CurrentLoopExternalSessionReturnContract{
		SubmissionPath: attempt.Current.SubmissionPath, SubmissionOutputs: attempt.Current.SubmissionOutputs,
		SubmissionResult: attempt.Current.SubmissionResult, SubmissionLast: true,
		Boundary: []string{"select exactly one allowed outcome template", "submission must bind the exact job and attempt receipt SHA", "write all required result paths before submissionPath", "refresh status after submission; do not relay or resume without review"},
	}
	for _, outcome := range job.AllowedOutcomes {
		template, replacements := externalSessionSubmissionTemplate(job, inspection.JobSHA256, attempt, typed.Dispatcher, outcome)
		required := []string{attempt.Current.SubmissionPath + " (last)"}
		if job.SessionKind == "member" && outcome == "returned" {
			required = append([]string{attempt.Current.SubmissionOutputs + "/**"}, required...)
		}
		if job.SessionKind == "reviewer" && outcome == "returned" {
			required = append([]string{attempt.Current.SubmissionResult}, required...)
		}
		contract.Templates = append(contract.Templates, mission.CurrentLoopExternalSessionSubmissionTemplate{Outcome: outcome, JSON: template, RequiredWrites: required, RequiredReplace: replacements})
	}
	if inspection.State == "submission-ready" {
		contract.ReviewRequest = operator.SelectedDriverRequest
		contract.RelayRecoveryRequest = typed.RelayPreviewRequest
	}
	return contract
}

func externalSessionSubmissionTemplate(job externalsession.Job, jobSHA string, attempt externalsession.AttemptInspection, dispatcher *mission.CurrentLoopExternalSessionDispatcher, outcome string) (string, []string) {
	value := externalsession.Submission{
		SchemaVersion: externalsession.SchemaVersion, Kind: externalsession.KindSubmission, JobID: job.JobID, JobSHA256: jobSHA,
		Outcome: outcome, Actor: "<actor>", AttemptID: attempt.Current.AttemptID, AttemptSHA256: attempt.AttemptSHA256,
		Harness: attempt.Current.Harness, Session: attempt.Current.Session, NoAuthorityOrConfirmed: true, NoHeavyTool: true,
	}
	replacements := []string{"<actor>"}
	if dispatcher != nil && dispatcher.Ticket != nil {
		if dispatcher.Claim != nil && dispatcher.LaunchReceipt != nil && dispatcher.LaunchReceipt.State == "accepted" {
			value.DispatchClaimSHA256 = dispatcher.Claim.SHA256
			value.LaunchReceiptSHA256 = dispatcher.LaunchReceipt.SHA256
			value.Harness = dispatcher.LaunchReceipt.ActualHarness
			value.Session = dispatcher.LaunchReceipt.ActualSession
		} else {
			value.DispatchClaimSHA256 = "<dispatch-claim-sha256>"
			value.LaunchReceiptSHA256 = "<launch-receipt-sha256>"
			value.Harness = "<actual-harness>"
			value.Session = "<actual-session>"
			replacements = append(replacements,
				"<dispatch-claim-sha256>", "<launch-receipt-sha256>",
				"<actual-harness>", "<actual-session>",
			)
		}
	}
	if job.SessionKind == "member" {
		value.ObservedAt = "<rfc3339nano>"
		replacements = append(replacements, "<rfc3339nano>")
		switch outcome {
		case "returned":
			value.Summary = "<summary>"
			replacements = append(replacements, "<summary>")
		case "failed":
			value.Reason = "<reason>"
			replacements = append(replacements, "<reason>")
		}
	} else {
		switch outcome {
		case "accepted":
			value.ReviewerHarness, value.ReviewerSession = value.Harness, value.Session
		case "returned":
			value.ReviewerSession = value.Session
		case "failed":
			value.ReviewerExitStatus = "<exit-status>"
			replacements = append(replacements, "<exit-status>")
		}
	}
	data, _ := json.MarshalIndent(value, "", "  ")
	return string(append(data, '\n')), replacements
}

func externalSessionHarnessInputPath(caseRoot, path string) string {
	if filepath.IsAbs(path) {
		return path
	}
	return filepath.Join(caseRoot, filepath.FromSlash(path))
}

func externalSessionBytesSHA(data []byte) string {
	sum := sha256.Sum256(data)
	return hex.EncodeToString(sum[:])
}

func currentLoopExternalSessionAllowedOutcomes(inspection currentloop.Inspection) []string {
	out := []string{}
	if inspection.Continuation == nil || inspection.Continuation.ObservationContract == nil {
		return out
	}
	for _, alternative := range inspection.Continuation.ObservationContract.Alternatives {
		for _, prefix := range []string{"member-session-", "reviewer-session-", "reviewer-result-"} {
			if outcome, ok := strings.CutPrefix(alternative.Kind, prefix); ok {
				if outcome == "returned" || outcome == "accepted" || outcome == "failed" {
					out = append(out, outcome)
				}
			}
		}
	}
	return out
}

func missionExternalSessionJob(inspection externalsession.Inspection) mission.CurrentLoopExternalSessionJob {
	job := inspection.Job
	typed := mission.CurrentLoopExternalSessionJob{
		SchemaVersion: job.SchemaVersion, Kind: job.Kind, JobID: job.JobID, JobSHA256: inspection.JobSHA256,
		State: inspection.State, SessionKind: job.SessionKind, CheckpointSHA256: job.CheckpointSHA256,
		AllowedOutcomes: append([]string{}, job.AllowedOutcomes...), SubmissionPath: job.SubmissionPath,
		SubmissionOutputs: job.SubmissionOutputs, SubmissionResult: job.SubmissionResult, MemberAttemptID: job.MemberAttemptID,
		MemberManifestPath: job.MemberManifestPath, MemberOutputsRoot: job.MemberOutputsRoot, RelayResultPath: job.RelayResultPath,
		PublicationPath: job.PublicationPath, ObservationPath: job.ObservationPath,
		SubmissionSHA256: inspection.SubmissionSHA256, Warnings: append([]string{}, inspection.Warnings...), SubmissionLast: job.SubmissionLast,
		NoSessionManagement: job.NoSessionManagement, NoHeavyTool: job.NoHeavyTool, NoAuthority: job.NoAuthority, NoConfirmed: job.NoConfirmed,
	}
	if job.MemberOwner != nil {
		typed.MemberOwner = &mission.CurrentLoopExternalSessionJobOwner{Lane: job.MemberOwner.Lane, Executor: job.MemberOwner.Executor, ExecutorGeneration: job.MemberOwner.ExecutorGeneration}
	}
	if job.Reviewer != nil {
		typed.Reviewer = &mission.CurrentLoopExternalSessionJobReviewer{
			AttemptSHA256: job.Reviewer.AttemptSHA256, PacketID: job.Reviewer.PacketID, RouteID: job.Reviewer.RouteID, ShardID: job.Reviewer.ShardID,
			DispatchID: job.Reviewer.DispatchID, Harness: job.Reviewer.Harness, Session: job.Reviewer.Session,
		}
	}
	return typed
}

func externalSessionRelayApplyCommand(plan externalsession.Plan) string {
	return joinDriverCommand([]string{
		"/rekit", "run-current-loop", "-Target", plan.Job.CaseRoot,
		"-ResumeCurrentLoop", "-ExpectedCurrentLoopCheckpointSha256", plan.Job.CheckpointSHA256,
		"-RelayExternalSessionSubmission", "-ExpectedExternalSessionJobSha256", plan.JobSHA256,
		"-ExpectedExternalSessionSubmissionSha256", plan.SubmissionSHA256,
		"-ExpectedExternalSessionRelayPlanSha256", plan.ExpectedPlanSHA256,
		"-Apply", "-Format", "json",
	})
}

func externalSessionRelayRequest(job externalsession.Job, jobSHA string) mission.MissionCommanderDriverRequest {
	command := joinDriverCommand([]string{"/rekit", "run-current-loop", "-Target", job.CaseRoot, "-ResumeCurrentLoop", "-ExpectedCurrentLoopCheckpointSha256", job.CheckpointSHA256, "-RelayExternalSessionSubmission", "-ExpectedExternalSessionJobSha256", jobSHA, "-WhatIf", "-Format", "json"})
	return mission.MissionCommanderDriverRequest{
		Kind: "preview-command", RunLoopStepID: "external-session-relay:" + job.JobID, Actor: "mission-commander",
		State: "review-required", Source: "current-loop-external-session-job", Lane: memberLane(job), Label: job.SessionKind + " session result relay",
		Command: command, CommandExecutable: true, RequiresReview: true,
		ExpectedReceipt: mission.MissionCommanderDriverReceiptExpectation{
			State: "preview-ready", RefreshStatusCommand: statusMissionControlRefreshCommand(job.CaseRoot),
			Description: "returns exact submission and relay plan hashes for reviewed Apply",
			Boundary:    []string{"preview does not publish artifacts or consume the checkpoint"},
		},
		Boundary: []string{"execute only while externalSessionJob jobSha256 and submissionSha256 remain current"},
	}
}

func memberLane(job externalsession.Job) string {
	if job.MemberOwner != nil {
		return job.MemberOwner.Lane
	}
	return ""
}
