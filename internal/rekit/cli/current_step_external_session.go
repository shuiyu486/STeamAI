package cli

import (
	"fmt"
	"strings"

	"github.com/shuiyu486/re-context-kits/internal/rekit/executioncontrol"
	"github.com/shuiyu486/re-context-kits/internal/rekit/externalsession"
	"github.com/shuiyu486/re-context-kits/internal/rekit/lanemutation"
	"github.com/shuiyu486/re-context-kits/internal/rekit/mission"
	"github.com/shuiyu486/re-context-kits/internal/rekit/runtime"
)

func currentStepExternalIdentity(step *currentStepExternalSessionPlan) currentStepExternalApplyIdentity {
	identity := currentStepExternalApplyIdentity{Mode: step.Mode}
	switch {
	case step.Attempt != nil:
		identity.NestedPlanSHA256 = step.Attempt.ExpectedPlanSHA256
		identity.JobSHA256 = step.Attempt.JobSHA256
		identity.AttemptSHA256 = step.Attempt.AttemptSHA256
		identity.DispatchSHA256 = step.Attempt.DispatchSHA256
		identity.CheckpointSHA256 = step.Attempt.Job.CheckpointSHA256
	case step.Dispatch != nil:
		identity.NestedPlanSHA256 = step.Dispatch.ExpectedPlanSHA256
		identity.JobSHA256 = step.Dispatch.JobSHA256
		identity.AttemptSHA256 = step.Dispatch.AttemptSHA256
		identity.DispatchSHA256 = step.Dispatch.DispatchSHA256
		identity.ClaimSHA256 = step.Dispatch.ExpectedClaimSHA256
		identity.CheckpointSHA256 = step.Dispatch.Job.CheckpointSHA256
	case step.Transport != nil:
		identity.NestedPlanSHA256 = step.Transport.ExpectedPlanSHA256
		identity.JobSHA256 = step.Transport.JobSHA256
		identity.AttemptSHA256 = step.Transport.AttemptSHA256
		identity.DispatchSHA256 = step.Transport.DispatchSHA256
		identity.ClaimSHA256 = step.Transport.ClaimSHA256
		identity.CheckpointSHA256 = step.Transport.Job.CheckpointSHA256
	case step.TransportReturn != nil:
		identity.NestedPlanSHA256 = step.TransportReturn.ExpectedPlanSHA256
		identity.JobSHA256 = step.TransportReturn.JobSHA256
		identity.AttemptSHA256 = step.TransportReturn.AttemptSHA256
		identity.DispatchSHA256 = step.TransportReturn.DispatchSHA256
		identity.ClaimSHA256 = step.TransportReturn.ClaimSHA256
		identity.CheckpointSHA256 = step.TransportReturn.Job.CheckpointSHA256
	case step.Turn != nil:
		identity.NestedPlanSHA256 = step.Turn.ExpectedPlanSHA256
		identity.JobSHA256 = step.Turn.Relay.JobSHA256
		identity.SubmissionSHA256 = step.Turn.Relay.SubmissionSHA256
		identity.CheckpointSHA256 = step.Turn.Relay.Job.CheckpointSHA256
	}
	return identity
}

func currentStepHasExternalSessionInputs(opt Options) bool {
	return strings.TrimSpace(opt.ExternalSessionHarness) != "" || strings.TrimSpace(opt.ExternalSessionID) != "" ||
		strings.TrimSpace(opt.ExternalSessionActor) != "" || strings.TrimSpace(opt.ExternalSessionStartedAt) != "" ||
		strings.TrimSpace(opt.ExternalSessionLaunchOutcome) != "" || strings.TrimSpace(opt.ExternalSessionObservedAt) != "" ||
		strings.TrimSpace(opt.ExternalSessionLaunchReason) != "" || strings.TrimSpace(opt.ExternalSessionTransportEndpoint) != "" ||
		strings.TrimSpace(opt.ExternalSessionDeliveryOutcome) != "" || strings.TrimSpace(opt.ExternalSessionProviderAckFingerprint) != "" ||
		strings.TrimSpace(opt.ExternalSessionDeliveryReason) != "" || strings.TrimSpace(opt.ExternalSessionReviewerResultSourcePath) != "" ||
		strings.TrimSpace(opt.ExpectedExternalSessionAttemptSHA256) != ""
}

func currentStepHasExternalSessionTransportInputs(opt Options) bool {
	return strings.TrimSpace(opt.ExternalSessionTransportEndpoint) != "" || strings.TrimSpace(opt.ExternalSessionDeliveryOutcome) != "" ||
		strings.TrimSpace(opt.ExternalSessionProviderAckFingerprint) != "" || strings.TrimSpace(opt.ExternalSessionDeliveryReason) != "" ||
		strings.TrimSpace(opt.ExternalSessionReviewerResultSourcePath) != ""
}

func validateCurrentStepExternalAttemptInputs(opt Options) error {
	if strings.TrimSpace(opt.ExternalSessionLaunchOutcome) != "" || strings.TrimSpace(opt.ExternalSessionObservedAt) != "" || strings.TrimSpace(opt.ExternalSessionLaunchReason) != "" || currentStepHasExternalSessionTransportInputs(opt) {
		return fmt.Errorf("run-current-step attempt transition accepts only attempt identity inputs")
	}
	return nil
}

func validateCurrentStepExternalClaimInputs(opt Options) error {
	if strings.TrimSpace(opt.ExternalSessionHarness) != "" || strings.TrimSpace(opt.ExternalSessionID) != "" || strings.TrimSpace(opt.ExternalSessionStartedAt) != "" || strings.TrimSpace(opt.ExternalSessionLaunchOutcome) != "" || strings.TrimSpace(opt.ExternalSessionLaunchReason) != "" || currentStepHasExternalSessionTransportInputs(opt) || strings.TrimSpace(opt.ExpectedExternalSessionAttemptSHA256) != "" {
		return fmt.Errorf("run-current-step dispatch claim accepts only externalSessionActor and externalSessionObservedAt")
	}
	return nil
}

func validateCurrentStepExternalOuterInputs(opt Options) error {
	if strings.TrimSpace(opt.Start.Actor) != "" {
		return fmt.Errorf("run-current-step external session route does not accept -Actor; use -ExternalSessionActor for lifecycle transitions")
	}
	if strings.TrimSpace(opt.ExpectedMemberExecutionPlanSHA256) != "" {
		return fmt.Errorf("run-current-step external session route does not accept -ExpectedMemberExecutionPlanSha256")
	}
	return nil
}

func validateCurrentStepExternalLaunchInputs(opt Options, outcome string) error {
	if strings.TrimSpace(opt.ExternalSessionStartedAt) != "" || currentStepHasExternalSessionTransportInputs(opt) || strings.TrimSpace(opt.ExpectedExternalSessionAttemptSHA256) != "" {
		return fmt.Errorf("run-current-step launch receipt does not accept attempt or transport transition inputs")
	}
	switch outcome {
	case "accepted":
		if strings.TrimSpace(opt.ExternalSessionLaunchReason) != "" {
			return fmt.Errorf("run-current-step accepted launch receipt does not accept externalSessionLaunchReason")
		}
	case "failed":
		if strings.TrimSpace(opt.ExternalSessionHarness) != "" || strings.TrimSpace(opt.ExternalSessionID) != "" {
			return fmt.Errorf("run-current-step failed launch receipt does not accept externalSessionHarness or externalSessionId")
		}
	case "":
		return nil
	default:
		return fmt.Errorf("run-current-step external launch outcome must be accepted or failed")
	}
	return nil
}

func buildCurrentStepExternalSessionPlan(ctx runtime.Context, opt Options, status statusInventory, request mission.MissionCommanderDriverRequest) (*currentStepExternalSessionPlan, string, bool, error) {
	if opt.currentLoopExternalTurnResume || currentStepHasMemberObservation(opt) || currentStepHasReviewerObservation(opt) {
		return nil, "", false, nil
	}
	runbook := status.MissionControlRunbook
	if runbook == nil || runbook.CurrentLoopOperator == nil || runbook.CurrentLoopOperator.ExternalSessionJob == nil || runbook.CurrentLoopSegment == nil {
		return nil, "", false, nil
	}
	operator := runbook.CurrentLoopOperator
	if operator.ObservationInbox != nil && operator.ObservationInbox.State != "empty" {
		return nil, "", false, nil
	}
	typed := operator.ExternalSessionJob
	if request.Source != "current-step-external-session" && request.Source != "current-loop-external-session-job" && request.Source != "current-loop-external-session-dispatch" && request.Source != "current-loop-external-session-transport" && request.Source != "current-loop-external-session-turn" {
		return nil, "", false, nil
	}
	if err := validateCurrentStepExternalOuterInputs(opt); err != nil {
		return nil, "", true, err
	}
	job, err := externalSessionJobFor(ctx.Target, operator, *runbook.CurrentLoopSegment)
	if err != nil {
		return nil, "", true, err
	}
	refresh, err := statusMissionControlRefreshCommand(ctx.Target)
	if err != nil {
		return nil, "", true, err
	}
	plan := &currentStepExternalSessionPlan{
		SchemaVersion:        1,
		State:                operator.State,
		HarnessPackage:       typed.HarnessPackage,
		RefreshStatusCommand: refresh,
		Boundary: []string{
			"the unified step binds the exact current external job, checkpoint, attempt generation, and nested plan",
			"the Go runtime records deterministic artifacts only; the external harness owns session launch, reconnect, polling, stop, and result production",
			"Remote Control endpoint and delivery observations are transport facts only; accepted delivery is not completion or authority",
			"no heavy tool, authority, or confirmed state is produced by this step",
		},
	}
	attempt, err := externalsession.InspectAttempt(job)
	if err != nil {
		return nil, "", true, err
	}
	dispatch, err := externalsession.InspectDispatch(job, attempt)
	if err != nil {
		return nil, "", true, err
	}
	transport, err := externalsession.InspectTransport(job, attempt, dispatch)
	if err != nil {
		return nil, "", true, err
	}

	if opt.Apply {
		switch {
		case strings.TrimSpace(opt.ExternalSessionTransportEndpoint) != "":
			if strings.TrimSpace(opt.ExternalSessionDeliveryOutcome) != "" || strings.TrimSpace(opt.ExternalSessionProviderAckFingerprint) != "" || strings.TrimSpace(opt.ExternalSessionDeliveryReason) != "" || strings.TrimSpace(opt.ExternalSessionReviewerResultSourcePath) != "" || strings.TrimSpace(opt.ExternalSessionLaunchOutcome) != "" || strings.TrimSpace(opt.ExternalSessionStartedAt) != "" || strings.TrimSpace(opt.ExternalSessionHarness) != "" || strings.TrimSpace(opt.ExternalSessionID) != "" {
				return nil, "", true, fmt.Errorf("run-current-step Remote Control endpoint transition accepts only endpoint, actor, and observedAt inputs")
			}
			transportPlan, err := externalsession.PreviewTransportEndpoint(job, attempt, dispatch, opt.ExternalSessionTransportEndpoint, opt.ExternalSessionActor, opt.ExternalSessionObservedAt)
			if err != nil {
				return nil, "", true, err
			}
			plan.Mode, plan.Transport = "remote-control-endpoint", &transportPlan
			return plan, transportPlan.ExpectedPlanSHA256, true, nil
		case strings.TrimSpace(opt.ExternalSessionDeliveryOutcome) != "":
			if strings.TrimSpace(opt.ExternalSessionTransportEndpoint) != "" || strings.TrimSpace(opt.ExternalSessionReviewerResultSourcePath) != "" || strings.TrimSpace(opt.ExternalSessionLaunchOutcome) != "" || strings.TrimSpace(opt.ExternalSessionStartedAt) != "" || strings.TrimSpace(opt.ExternalSessionHarness) != "" || strings.TrimSpace(opt.ExternalSessionID) != "" {
				return nil, "", true, fmt.Errorf("run-current-step Remote Control delivery transition accepts only outcome, provider ack fingerprint, actor, observedAt, and reason inputs")
			}
			transportPlan, err := externalsession.PreviewTransportDelivery(job, attempt, dispatch, opt.ExternalSessionDeliveryOutcome, opt.ExternalSessionProviderAckFingerprint, opt.ExternalSessionActor, opt.ExternalSessionObservedAt, opt.ExternalSessionDeliveryReason)
			if err != nil {
				return nil, "", true, err
			}
			plan.Mode, plan.Transport = "remote-control-delivery", &transportPlan
			return plan, transportPlan.ExpectedPlanSHA256, true, nil
		case strings.TrimSpace(opt.ExternalSessionReviewerResultSourcePath) != "":
			if strings.TrimSpace(opt.ExternalSessionTransportEndpoint) != "" || strings.TrimSpace(opt.ExternalSessionDeliveryOutcome) != "" || strings.TrimSpace(opt.ExternalSessionProviderAckFingerprint) != "" || strings.TrimSpace(opt.ExternalSessionDeliveryReason) != "" || strings.TrimSpace(opt.ExternalSessionLaunchOutcome) != "" || strings.TrimSpace(opt.ExternalSessionStartedAt) != "" || strings.TrimSpace(opt.ExternalSessionHarness) != "" || strings.TrimSpace(opt.ExternalSessionID) != "" {
				return nil, "", true, fmt.Errorf("run-current-step Remote Control return accepts only source path, actor, and observedAt inputs")
			}
			returnPlan, err := externalsession.PreviewTransportReturn(job, opt.ExternalSessionReviewerResultSourcePath, opt.ExternalSessionActor, opt.ExternalSessionObservedAt)
			if err != nil {
				return nil, "", true, err
			}
			plan.Mode, plan.TransportReturn = "remote-control-return", &returnPlan
			return plan, returnPlan.ExpectedPlanSHA256, true, nil
		case strings.TrimSpace(opt.ExternalSessionStartedAt) != "":
			if err := validateCurrentStepExternalAttemptInputs(opt); err != nil {
				return nil, "", true, err
			}
			var attemptPlan externalsession.AttemptPlan
			if opt.currentLoopReplacementMutationLease != nil {
				attemptPlan, err = externalsession.ResolveAttemptApplyPlanUnboundWithProjectLease(
					job,
					opt.ExternalSessionHarness,
					opt.ExternalSessionID,
					opt.ExternalSessionActor,
					opt.ExternalSessionStartedAt,
					opt.ExpectedExternalSessionAttemptSHA256,
					opt.currentLoopReplacementMutationLease,
				)
			} else {
				attemptPlan, err = externalsession.ResolveAttemptApplyPlanUnbound(job, opt.ExternalSessionHarness, opt.ExternalSessionID, opt.ExternalSessionActor, opt.ExternalSessionStartedAt, opt.ExpectedExternalSessionAttemptSHA256)
			}
			if err != nil {
				return nil, "", true, err
			}
			inspection, err := externalsession.Inspect(job)
			if err != nil {
				return nil, "", true, err
			}
			ticket, err := externalSessionDispatchTicket(job, inspection, attemptPlan, operator)
			if err != nil {
				return nil, "", true, err
			}
			attemptPlan, err = externalsession.BindAttemptDispatch(attemptPlan, ticket)
			if err != nil {
				return nil, "", true, err
			}
			plan.Mode = "attempt"
			if attemptPlan.Attempt.SupersedesSHA256 != "" {
				plan.Mode = "replacement-attempt"
			}
			plan.Attempt = &attemptPlan
			return plan, attemptPlan.ExpectedPlanSHA256, true, nil
		case strings.TrimSpace(opt.ExternalSessionLaunchOutcome) != "":
			outcome := strings.ToLower(strings.TrimSpace(opt.ExternalSessionLaunchOutcome))
			if err := validateCurrentStepExternalLaunchInputs(opt, outcome); err != nil {
				return nil, "", true, err
			}
			dispatchPlan, err := externalsession.PreviewDispatchTransition(job, attempt, outcome, opt.ExternalSessionActor, opt.ExternalSessionObservedAt, opt.ExternalSessionHarness, opt.ExternalSessionID, opt.ExternalSessionLaunchReason)
			if err != nil {
				return nil, "", true, err
			}
			plan.Mode = "launch-" + outcome
			plan.Dispatch = &dispatchPlan
			return plan, dispatchPlan.ExpectedPlanSHA256, true, nil
		case strings.TrimSpace(opt.ExternalSessionObservedAt) != "":
			if err := validateCurrentStepExternalClaimInputs(opt); err != nil {
				return nil, "", true, err
			}
			dispatchPlan, err := externalsession.PreviewDispatchTransition(job, attempt, "claimed", opt.ExternalSessionActor, opt.ExternalSessionObservedAt, "", "", "")
			if err != nil {
				return nil, "", true, err
			}
			plan.Mode = "dispatch-claim"
			plan.Dispatch = &dispatchPlan
			return plan, dispatchPlan.ExpectedPlanSHA256, true, nil
		case dispatch.State == "queued" && attempt.Current != nil && strings.TrimSpace(opt.ExpectedCurrentStepPlanSHA256) != "":
			var attemptPlan externalsession.AttemptPlan
			if opt.currentLoopReplacementMutationLease != nil {
				attemptPlan, err = externalsession.ResolveAttemptApplyPlanUnboundWithProjectLease(
					job,
					attempt.Current.Harness,
					attempt.Current.Session,
					attempt.Current.Actor,
					attempt.Current.StartedAt,
					attempt.Current.SupersedesSHA256,
					opt.currentLoopReplacementMutationLease,
				)
			} else {
				attemptPlan, err = externalsession.ResolveAttemptApplyPlanUnbound(job, attempt.Current.Harness, attempt.Current.Session, attempt.Current.Actor, attempt.Current.StartedAt, attempt.Current.SupersedesSHA256)
			}
			if err != nil {
				return nil, "", true, err
			}
			inspection, err := externalsession.Inspect(job)
			if err != nil {
				return nil, "", true, err
			}
			ticket, err := externalSessionDispatchTicket(job, inspection, attemptPlan, operator)
			if err != nil {
				return nil, "", true, err
			}
			attemptPlan, err = externalsession.BindAttemptDispatch(attemptPlan, ticket)
			if err != nil {
				return nil, "", true, err
			}
			plan.Mode = "attempt-publication-recovery"
			plan.Attempt = &attemptPlan
			return plan, attemptPlan.ExpectedPlanSHA256, true, nil
		}
	}

	switch {
	case typed.State == "submission-ready":
		if currentStepHasExternalSessionInputs(opt) {
			return nil, "", true, fmt.Errorf("run-current-step external result turn does not accept transition inputs")
		}
		inner := externalSessionTurnOptions(ctx, opt, job, typed.JobSHA256)
		turn, _, _, err := buildCurrentLoopExternalSessionTurnPlan(ctx, inner)
		if err != nil {
			return nil, "", true, err
		}
		turn.ApplyCommand = selectedLaneCommand(
			externalSessionTurnApplyCommand(turn),
			opt.SelectedCurrentLane,
		)
		plan.Mode = "result-turn"
		plan.Turn = &turn
		return plan, turn.ExpectedPlanSHA256, true, nil
	case dispatch.State == "attempt-publication-pending":
		if currentStepHasExternalSessionInputs(opt) {
			return nil, "", true, fmt.Errorf("run-current-step attempt publication recovery does not accept external session transition inputs")
		}
		pending, err := externalsession.PendingAttemptPlan(job, dispatch)
		if err != nil {
			return nil, "", true, err
		}
		plan.Mode = "attempt-publication-recovery"
		plan.Attempt = &pending
		return plan, pending.ExpectedPlanSHA256, true, nil
	case typed.State == "invalid":
		if attempt.Current != nil && job.SessionKind == "reviewer" && job.Reviewer != nil && job.Reviewer.DispatchID != "" {
			if currentStepHasExternalSessionInputs(opt) {
				return nil, "", true, fmt.Errorf("invalid durable Reviewer job blocks same-job replacement inputs; create a new Reviewer dispatch, session binding, and external-session job")
			}
			plan.Mode = "new-reviewer-dispatch-required"
			plan.ReplacementRequest = typed.AttemptRequest
			return plan, "", true, nil
		}
		if err := validateCurrentStepExternalAttemptInputs(opt); err != nil {
			return nil, "", true, err
		}
		if strings.TrimSpace(opt.ExternalSessionHarness) == "" || strings.TrimSpace(opt.ExternalSessionID) == "" || strings.TrimSpace(opt.ExternalSessionActor) == "" || strings.TrimSpace(opt.ExternalSessionStartedAt) == "" {
			plan.Mode = "attempt-input"
			plan.InputRequired = []string{"externalSessionHarness", "externalSessionId", "externalSessionActor", "externalSessionStartedAt"}
			plan.ReplacementRequest = typed.AttemptRequest
			return plan, "", true, nil
		}
		attemptPlan, err := externalsession.PreviewAttempt(job, opt.ExternalSessionHarness, opt.ExternalSessionID, opt.ExternalSessionActor, opt.ExternalSessionStartedAt, attempt.AttemptSHA256)
		if err != nil {
			return nil, "", true, err
		}
		inspection, err := externalsession.Inspect(job)
		if err != nil {
			return nil, "", true, err
		}
		ticket, err := externalSessionDispatchTicket(job, inspection, attemptPlan, operator)
		if err != nil {
			return nil, "", true, err
		}
		attemptPlan, err = externalsession.BindAttemptDispatch(attemptPlan, ticket)
		if err != nil {
			return nil, "", true, err
		}
		plan.Mode, plan.Attempt = "replacement-attempt", &attemptPlan
		return plan, attemptPlan.ExpectedPlanSHA256, true, nil
	case dispatch.State == "queued":
		if err := validateCurrentStepExternalClaimInputs(opt); err != nil {
			return nil, "", true, err
		}
		if strings.TrimSpace(opt.ExternalSessionActor) == "" || strings.TrimSpace(opt.ExternalSessionObservedAt) == "" {
			plan.Mode = "dispatch-claim-input"
			plan.InputRequired = []string{"externalSessionActor", "externalSessionObservedAt"}
			return plan, "", true, nil
		}
		dispatchPlan, err := externalsession.PreviewDispatchTransition(job, attempt, "claimed", opt.ExternalSessionActor, opt.ExternalSessionObservedAt, "", "", "")
		if err != nil {
			return nil, "", true, err
		}
		plan.Mode = "dispatch-claim"
		plan.Dispatch = &dispatchPlan
		return plan, dispatchPlan.ExpectedPlanSHA256, true, nil
	case dispatch.State == "claimed" && externalsession.IsRemoteControlAttempt(attempt.Current):
		switch transport.State {
		case "endpoint-required":
			if strings.TrimSpace(opt.ExternalSessionTransportEndpoint) == "" || strings.TrimSpace(opt.ExternalSessionActor) == "" || strings.TrimSpace(opt.ExternalSessionObservedAt) == "" {
				plan.Mode = "remote-control-discovery-input"
				plan.InputRequired = []string{"run ListAgents", "externalSessionTransportEndpoint=name [ref]", "externalSessionActor", "externalSessionObservedAt"}
				return plan, "", true, nil
			}
			transportPlan, err := externalsession.PreviewTransportEndpoint(job, attempt, dispatch, opt.ExternalSessionTransportEndpoint, opt.ExternalSessionActor, opt.ExternalSessionObservedAt)
			if err != nil {
				return nil, "", true, err
			}
			plan.Mode, plan.Transport = "remote-control-endpoint", &transportPlan
			return plan, transportPlan.ExpectedPlanSHA256, true, nil
		case "delivery-required":
			if strings.TrimSpace(opt.ExternalSessionDeliveryOutcome) == "" || strings.TrimSpace(opt.ExternalSessionActor) == "" || strings.TrimSpace(opt.ExternalSessionObservedAt) == "" {
				plan.Mode = "remote-control-delivery-input"
				plan.InputRequired = []string{"send transport.message.message with SendMessage to transport.message.recipient", "externalSessionDeliveryOutcome=accepted|rejected|uncertain", "externalSessionActor", "externalSessionObservedAt", "rejected|uncertain: externalSessionDeliveryReason"}
				return plan, "", true, nil
			}
			transportPlan, err := externalsession.PreviewTransportDelivery(job, attempt, dispatch, opt.ExternalSessionDeliveryOutcome, opt.ExternalSessionProviderAckFingerprint, opt.ExternalSessionActor, opt.ExternalSessionObservedAt, opt.ExternalSessionDeliveryReason)
			if err != nil {
				return nil, "", true, err
			}
			plan.Mode, plan.Transport = "remote-control-delivery", &transportPlan
			return plan, transportPlan.ExpectedPlanSHA256, true, nil
		case "delivery-accepted", "delivery-rejected":
			outcome, actor, observedAt, actualHarness, actualSession, reason, err := externalsession.TransportLaunchTransition(transport)
			if err != nil {
				return nil, "", true, err
			}
			dispatchPlan, err := externalsession.PreviewDispatchTransition(job, attempt, outcome, actor, observedAt, actualHarness, actualSession, reason)
			if err != nil {
				return nil, "", true, err
			}
			plan.Mode = "launch-" + outcome
			plan.Dispatch = &dispatchPlan
			return plan, dispatchPlan.ExpectedPlanSHA256, true, nil
		case "delivery-uncertain":
			if currentStepHasExternalSessionInputs(opt) {
				return nil, "", true, fmt.Errorf("uncertain Remote Control delivery blocks automatic resend, launch receipt, and same-job replacement inputs")
			}
			plan.Mode = "remote-control-delivery-uncertain"
			if typed.Transport != nil {
				plan.ReplacementRequest = typed.Transport.ReplacementRequest
			}
			return plan, "", true, nil
		default:
			return nil, "", true, fmt.Errorf("run-current-step Remote Control transport state %q is unsupported", transport.State)
		}
	case dispatch.State == "claimed":
		outcome := strings.ToLower(strings.TrimSpace(opt.ExternalSessionLaunchOutcome))
		if err := validateCurrentStepExternalLaunchInputs(opt, outcome); err != nil {
			return nil, "", true, err
		}
		if outcome == "" || strings.TrimSpace(opt.ExternalSessionActor) == "" || strings.TrimSpace(opt.ExternalSessionObservedAt) == "" || (outcome == "accepted" && (strings.TrimSpace(opt.ExternalSessionHarness) == "" || strings.TrimSpace(opt.ExternalSessionID) == "")) || (outcome == "failed" && strings.TrimSpace(opt.ExternalSessionLaunchReason) == "") {
			plan.Mode = "launch-receipt-input"
			plan.InputRequired = []string{"externalSessionLaunchOutcome=accepted|failed", "externalSessionActor", "externalSessionObservedAt", "accepted: externalSessionHarness + externalSessionId", "failed: externalSessionLaunchReason"}
			return plan, "", true, nil
		}
		if outcome != "accepted" && outcome != "failed" {
			return nil, "", true, fmt.Errorf("run-current-step external launch outcome must be accepted or failed")
		}
		dispatchPlan, err := externalsession.PreviewDispatchTransition(job, attempt, outcome, opt.ExternalSessionActor, opt.ExternalSessionObservedAt, opt.ExternalSessionHarness, opt.ExternalSessionID, opt.ExternalSessionLaunchReason)
		if err != nil {
			return nil, "", true, err
		}
		plan.Mode = "launch-" + outcome
		plan.Dispatch = &dispatchPlan
		return plan, dispatchPlan.ExpectedPlanSHA256, true, nil
	case dispatch.State == "running" && externalsession.IsRemoteControlAttempt(attempt.Current):
		if strings.TrimSpace(opt.ExternalSessionReviewerResultSourcePath) == "" {
			if currentStepHasExternalSessionInputs(opt) {
				return nil, "", true, fmt.Errorf("running Remote Control return accepts only ReviewerResult source path, actor, and observedAt")
			}
			plan.Mode = "remote-control-return-input"
			plan.InputRequired = []string{"write one case-local ReviewerResult source file", "externalSessionReviewerResultSourcePath", "externalSessionActor", "externalSessionObservedAt"}
			if typed.Transport != nil {
				plan.ReturnRequest = typed.Transport.ReturnRequest
			}
			return plan, "", true, nil
		}
		if strings.TrimSpace(opt.ExternalSessionActor) == "" || strings.TrimSpace(opt.ExternalSessionObservedAt) == "" || strings.TrimSpace(opt.ExternalSessionTransportEndpoint) != "" || strings.TrimSpace(opt.ExternalSessionDeliveryOutcome) != "" || strings.TrimSpace(opt.ExternalSessionProviderAckFingerprint) != "" || strings.TrimSpace(opt.ExternalSessionDeliveryReason) != "" || strings.TrimSpace(opt.ExternalSessionLaunchOutcome) != "" || strings.TrimSpace(opt.ExternalSessionStartedAt) != "" || strings.TrimSpace(opt.ExternalSessionHarness) != "" || strings.TrimSpace(opt.ExternalSessionID) != "" || strings.TrimSpace(opt.ExpectedExternalSessionAttemptSHA256) != "" {
			return nil, "", true, fmt.Errorf("running Remote Control return requires only source path, actor, and observedAt")
		}
		returnPlan, err := externalsession.PreviewTransportReturn(job, opt.ExternalSessionReviewerResultSourcePath, opt.ExternalSessionActor, opt.ExternalSessionObservedAt)
		if err != nil {
			return nil, "", true, err
		}
		plan.Mode, plan.TransportReturn = "remote-control-return", &returnPlan
		return plan, returnPlan.ExpectedPlanSHA256, true, nil
	case dispatch.State == "running":
		if job.SessionKind == "reviewer" && job.Reviewer != nil && job.Reviewer.DispatchID != "" {
			if currentStepHasExternalSessionInputs(opt) {
				return nil, "", true, fmt.Errorf("running durable Reviewer job blocks same-job replacement inputs; create a new Reviewer dispatch, session binding, and external-session job")
			}
			plan.Mode = "new-reviewer-dispatch-required"
			plan.ReplacementRequest = typed.AttemptRequest
			return plan, "", true, nil
		}
		if !currentStepHasExternalSessionInputs(opt) {
			plan.Mode = "running-handoff"
			plan.ReplacementRequest = typed.AttemptRequest
			return plan, "", true, nil
		}
		if err := validateCurrentStepExternalAttemptInputs(opt); err != nil {
			return nil, "", true, err
		}
		if strings.TrimSpace(opt.ExternalSessionHarness) == "" || strings.TrimSpace(opt.ExternalSessionID) == "" || strings.TrimSpace(opt.ExternalSessionActor) == "" || strings.TrimSpace(opt.ExternalSessionStartedAt) == "" || strings.TrimSpace(opt.ExpectedExternalSessionAttemptSHA256) == "" {
			return nil, "", true, fmt.Errorf("run-current-step running replacement requires attempt identity inputs and the exact current attempt sha256")
		}
		attemptPlan, err := externalsession.PreviewAttempt(job, opt.ExternalSessionHarness, opt.ExternalSessionID, opt.ExternalSessionActor, opt.ExternalSessionStartedAt, opt.ExpectedExternalSessionAttemptSHA256)
		if err != nil {
			return nil, "", true, err
		}
		inspection, err := externalsession.Inspect(job)
		if err != nil {
			return nil, "", true, err
		}
		ticket, err := externalSessionDispatchTicket(job, inspection, attemptPlan, operator)
		if err != nil {
			return nil, "", true, err
		}
		attemptPlan, err = externalsession.BindAttemptDispatch(attemptPlan, ticket)
		if err != nil {
			return nil, "", true, err
		}
		plan.Mode, plan.Attempt = "replacement-attempt", &attemptPlan
		return plan, attemptPlan.ExpectedPlanSHA256, true, nil
	case dispatch.State == "launch-failed" || attempt.Current == nil || typed.State == "invalid":
		if attempt.Current != nil && job.SessionKind == "reviewer" && job.Reviewer != nil && job.Reviewer.DispatchID != "" {
			if currentStepHasExternalSessionInputs(opt) {
				return nil, "", true, fmt.Errorf("failed durable Reviewer job blocks same-job replacement inputs; create a new Reviewer dispatch, session binding, and external-session job")
			}
			plan.Mode = "new-reviewer-dispatch-required"
			plan.ReplacementRequest = typed.AttemptRequest
			return plan, "", true, nil
		}
		if err := validateCurrentStepExternalAttemptInputs(opt); err != nil {
			return nil, "", true, err
		}
		if strings.TrimSpace(opt.ExternalSessionHarness) == "" || strings.TrimSpace(opt.ExternalSessionID) == "" || strings.TrimSpace(opt.ExternalSessionActor) == "" || strings.TrimSpace(opt.ExternalSessionStartedAt) == "" {
			plan.Mode = "attempt-input"
			plan.InputRequired = []string{"externalSessionHarness", "externalSessionId", "externalSessionActor", "externalSessionStartedAt"}
			plan.ReplacementRequest = typed.AttemptRequest
			return plan, "", true, nil
		}
		attemptPlan, err := externalsession.PreviewAttempt(job, opt.ExternalSessionHarness, opt.ExternalSessionID, opt.ExternalSessionActor, opt.ExternalSessionStartedAt, attempt.AttemptSHA256)
		if err != nil {
			return nil, "", true, err
		}
		inspection, err := externalsession.Inspect(job)
		if err != nil {
			return nil, "", true, err
		}
		ticket, err := externalSessionDispatchTicket(job, inspection, attemptPlan, operator)
		if err != nil {
			return nil, "", true, err
		}
		attemptPlan, err = externalsession.BindAttemptDispatch(attemptPlan, ticket)
		if err != nil {
			return nil, "", true, err
		}
		plan.Mode = "attempt"
		if attempt.Current != nil {
			plan.Mode = "replacement-attempt"
		}
		plan.Attempt = &attemptPlan
		return plan, attemptPlan.ExpectedPlanSHA256, true, nil
	default:
		return nil, "", true, fmt.Errorf("run-current-step external session state %q is unsupported", dispatch.State)
	}
}

func externalSessionTurnOptions(ctx runtime.Context, opt Options, job externalsession.Job, jobSHA256 string) Options {
	inner := opt
	inner.Command, inner.Target, inner.Pack, inner.PackProvided = "run-current-loop", ctx.Target, ctx.Pack, true
	inner.WhatIf, inner.Apply, inner.ResumeCurrentLoop, inner.AdvanceExternalSessionResult = true, false, true, true
	inner.ExpectedCurrentLoopCheckpointSHA256 = job.CheckpointSHA256
	inner.ExpectedExternalSessionJobSHA256 = jobSHA256
	inner.ExpectedCurrentStepPlanSHA256 = ""
	inner.rawArgs = nil
	return inner
}

func applyCurrentStepExternalSession(ctx runtime.Context, opt Options, outer currentStepPlan) (currentStepPlan, error) {
	step := outer.ExternalSessionStep
	if step == nil {
		return currentStepPlan{}, currentStepZeroProgressError{cause: fmt.Errorf("run-current-step external session plan is missing")}
	}
	if step.Attempt != nil {
		current := func() (externalsession.Job, error) {
			return currentExternalSessionJob(ctx, opt)
		}
		var applied externalsession.AttemptPlan
		var err error
		if publication := opt.currentLoopReplacementResultPublication; publication != nil {
			if step.Mode != "replacement-attempt" || opt.currentLoopReplacementMutationLease == nil ||
				opt.currentLoopReplacementResultChecked == nil {
				return currentStepPlan{}, currentStepZeroProgressError{cause: fmt.Errorf("replacement result publication requires the runner-owned project mutation lease")}
			}
			applied, err = externalsession.ApplyAttemptCurrentWithLease(
				*step.Attempt,
				step.Attempt.JobSHA256,
				step.Attempt.ExpectedPlanSHA256,
				opt.currentLoopReplacementMutationLease,
				current,
				func(lease *lanemutation.Lease) error {
					prepared, err := executioncontrol.PrepareResultWithProjectLease(ctx.Target, lease, *publication)
					if err != nil {
						return err
					}
					if prepared.Held {
						return &executioncontrol.ResultHeldError{Publication: prepared}
					}
					if prepared.Disposition != executioncontrol.ResultDispositionCurrent {
						return fmt.Errorf("replacement result preparation returned unexpected disposition %q", prepared.Disposition)
					}
					*opt.currentLoopReplacementResultChecked = true
					return nil
				},
			)
			if err == nil && applied.AlreadyApplied {
				*opt.currentLoopReplacementResultChecked = true
			}
		} else {
			if opt.currentLoopReplacementMutationLease != nil || opt.currentLoopReplacementResultChecked != nil {
				return currentStepPlan{}, currentStepZeroProgressError{cause: fmt.Errorf("replacement mutation ownership omitted its exact result provenance")}
			}
			applied, err = externalsession.ApplyAttemptCurrent(
				*step.Attempt,
				step.Attempt.JobSHA256,
				step.Attempt.ExpectedPlanSHA256,
				current,
			)
		}
		if err != nil {
			return currentStepPlan{}, currentStepZeroProgressError{cause: err}
		}
		step.Attempt = &applied
		outer.Applied, outer.IsMutation = applied.Applied, applied.Applied
	} else if step.Dispatch != nil {
		applied, err := externalsession.ApplyDispatchTransitionCurrent(*step.Dispatch, step.Dispatch.JobSHA256, step.Dispatch.DispatchSHA256, step.Dispatch.ExpectedClaimSHA256, step.Dispatch.ExpectedPlanSHA256, func() (externalsession.Job, error) {
			return currentExternalSessionJob(ctx, opt)
		})
		if err != nil {
			return currentStepPlan{}, currentStepZeroProgressError{cause: err}
		}
		step.Dispatch = &applied
		outer.Applied, outer.IsMutation = applied.Applied, applied.Applied
	} else if step.Transport != nil {
		applied, err := externalsession.ApplyTransportCurrent(*step.Transport, step.Transport.ExpectedPlanSHA256, func() (externalsession.Job, error) {
			return currentExternalSessionJob(ctx, opt)
		})
		if err != nil {
			return currentStepPlan{}, currentStepZeroProgressError{cause: err}
		}
		step.Transport = &applied
		outer.Applied, outer.IsMutation = applied.Applied, applied.Applied
	} else if step.TransportReturn != nil {
		applied, err := externalsession.ApplyTransportReturnCurrent(*step.TransportReturn, step.TransportReturn.ExpectedPlanSHA256, func() (externalsession.Job, error) {
			return currentExternalSessionJob(ctx, opt)
		})
		if err != nil {
			return currentStepPlan{}, currentStepZeroProgressError{cause: err}
		}
		step.TransportReturn = &applied
		outer.Applied, outer.IsMutation = applied.Applied, applied.Applied
	} else if step.Turn != nil {
		inner := externalSessionTurnOptions(ctx, opt, step.Turn.Relay.Job, step.Turn.Relay.JobSHA256)
		inner.WhatIf, inner.Apply = false, true
		inner.ExpectedExternalSessionSubmissionSHA256 = step.Turn.Relay.SubmissionSHA256
		inner.ExpectedExternalSessionRelayPlanSHA256 = step.Turn.Relay.ExpectedPlanSHA256
		inner.ExpectedExternalSessionTurnPlanSHA256 = step.Turn.ExpectedPlanSHA256
		inner.rawArgs = nil
		_, resumeOpt, resumeStatus, err := buildCurrentLoopExternalSessionTurnPlan(ctx, inner)
		if err != nil {
			return currentStepPlan{}, currentStepZeroProgressError{cause: err}
		}
		applied, err := applyCurrentLoopExternalSessionTurn(ctx, inner, *step.Turn, resumeOpt, resumeStatus)
		step.Turn = &applied
		outer.ExternalSessionStep = step
		outer.Applied, outer.IsMutation = applied.Applied, applied.Applied
		if applied.Relay.ResultPublication != nil && applied.Relay.ResultPublication.Held {
			outer.ReviewRequired = false
			outer.RequiresConfirmation = false
			return outer, nil
		}
		if err != nil {
			if !applied.Applied {
				return currentStepPlan{}, currentStepZeroProgressError{cause: err}
			}
			return currentStepPartialResult(outer, "run-current-loop"), err
		}
	} else {
		return currentStepPlan{}, currentStepZeroProgressError{cause: fmt.Errorf("run-current-step external session route requires a deterministic nested plan")}
	}
	outer.ReviewRequired, outer.RequiresConfirmation = false, false
	outer.ExternalSessionStep = step
	fresh, err := buildInvocationStatusInventory(ctx, opt)
	if err != nil {
		return currentStepPartialResult(outer, "run-current-loop"), fmt.Errorf("refresh status after external session step: %w", err)
	}
	outer.RefreshedStatus = &fresh
	outer.Receipt = &currentStepReceipt{
		State: "refreshed", Outcome: "current-step-applied", Route: outer.Route, NestedCommand: "run-current-loop",
		Boundary: []string{"receipt records the selected deterministic external transition; it does not prove external process liveness or execution", "consume refreshedCurrentDriverRequest before follow-up work", "no authority/confirmed state or heavy-tool execution is produced"},
	}
	if fresh.MissionControlRunbook != nil {
		outer.Receipt.RefreshedCurrentDriverRequest = fresh.MissionControlRunbook.CurrentDriverRequest
	}
	return outer, nil
}
