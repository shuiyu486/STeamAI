package cli

import (
	"fmt"

	"github.com/shuiyu486/re-context-kits/internal/rekit/externalsession"
	"github.com/shuiyu486/re-context-kits/internal/rekit/mission"
)

func externalSessionTransportPackage(job externalsession.Job, attempt externalsession.AttemptInspection, dispatch externalsession.DispatchInspection, inspection externalsession.TransportInspection) (*mission.CurrentLoopExternalSessionTransportPackage, error) {
	if !inspection.Applicable {
		return nil, nil
	}
	refresh, err := statusMissionControlRefreshCommand(job.CaseRoot)
	if err != nil {
		return nil, err
	}
	pkg := &mission.CurrentLoopExternalSessionTransportPackage{
		State: inspection.State,
		Binding: mission.CurrentLoopExternalSessionTransportBinding{
			Transport: inspection.Binding.Transport, TransportBindingID: inspection.Binding.TransportBindingID,
			JobID: inspection.Binding.JobID, JobSHA256: inspection.Binding.JobSHA256, CheckpointSHA256: inspection.Binding.CheckpointSHA256,
			AttemptID: inspection.Binding.AttemptID, AttemptSHA256: inspection.Binding.AttemptSHA256, Generation: inspection.Binding.Generation,
			DispatchSHA256: inspection.Binding.DispatchSHA256, ClaimSHA256: inspection.Binding.ClaimSHA256,
		},
		BundlePath: inspection.BundlePath, BundleSHA256: inspection.BundleSHA256, BundleBytes: inspection.BundleBytes,
		DiscoveryTool: "ListAgents",
		Warnings:      append([]string{}, inspection.Warnings...),
		Boundary: []string{
			"ListAgents name [ref] is an opaque endpoint snapshot, never a durable lane or executor identity",
			"SendMessage delivery is a transport observation, not completion, artifact publication, authority, or heavy-action authorization",
			"uncertain delivery is never retried automatically; recovery requires a new durable Reviewer dispatch, session binding, and job",
			"the receiver must return one bounded ReviewerResult to the sender; existing submission-last and strict intake remain authoritative",
		},
	}
	if inspection.Endpoint != nil {
		pkg.Endpoint = &mission.CurrentLoopExternalSessionTransportEndpoint{
			Path: inspection.EndpointPath, SHA256: inspection.EndpointSHA256, DiscoveryTool: inspection.Endpoint.DiscoveryTool,
			Endpoint: inspection.Endpoint.Endpoint, Actor: inspection.Endpoint.Actor, ObservedAt: inspection.Endpoint.ObservedAt,
		}
		pkg.Message = &mission.CurrentLoopExternalSessionTransportMessage{
			Operation: inspection.Endpoint.Envelope.Operation, Recipient: inspection.Endpoint.Envelope.Recipient,
			SourceInputRole: inspection.Endpoint.Envelope.SourceInputRole, SourceInputSHA256: inspection.Endpoint.Envelope.SourceInputSHA256,
			SourceInputBytes: inspection.Endpoint.Envelope.SourceInputBytes, BundlePath: inspection.Endpoint.Envelope.BundlePath,
			BundleSHA256: inspection.Endpoint.Envelope.BundleSHA256, BundleBytes: inspection.Endpoint.Envelope.BundleBytes,
			Message:       inspection.Endpoint.Envelope.Message,
			MessageSHA256: inspection.Endpoint.Envelope.MessageSHA256, MessageBytes: inspection.Endpoint.Envelope.MessageBytes,
			ExpectedReply: inspection.Endpoint.Envelope.ExpectedReply, NoFileTransfer: inspection.Endpoint.Envelope.NoFileTransfer,
		}
	}
	if inspection.Delivery != nil {
		pkg.Delivery = &mission.CurrentLoopExternalSessionTransportDelivery{
			Path: inspection.DeliveryPath, SHA256: inspection.DeliverySHA256, Outcome: inspection.Delivery.Outcome,
			ProviderAckFingerprint: inspection.Delivery.ProviderAckFingerprint, Actor: inspection.Delivery.Actor,
			ObservedAt: inspection.Delivery.ObservedAt, Reason: inspection.Delivery.Reason,
		}
	}
	if attempt.Current == nil || dispatch.Claim == nil {
		return pkg, nil
	}
	lane := memberLane(job)
	switch inspection.State {
	case "endpoint-required":
		pkg.DiscoveryRequest = &mission.MissionCommanderDriverRequest{
			Kind: "model-tool-handoff", RunLoopStepID: "remote-control-discovery:" + job.JobID,
			Actor: dispatch.Claim.Actor, State: inspection.State, Source: "current-loop-external-session-transport",
			Lane: lane, Label: job.SessionKind + " Remote Control endpoint discovery", ActionID: inspection.Binding.TransportBindingID,
			Guidance:          "Call the Claude Code ListAgents model tool once, select the intended Remote Control session by its exact opaque name [ref], then preview run-current-step with -ExternalSessionTransportEndpoint, -ExternalSessionActor, and -ExternalSessionObservedAt.",
			CommandExecutable: false, RequiresReview: true,
			ExpectedReceipt: mission.MissionCommanderDriverReceiptExpectation{
				State: "endpoint-snapshot-preview", RefreshStatusCommand: refresh,
				Description: "returns a hash-bound immutable endpoint snapshot and exact SendMessage envelope",
				Boundary:    []string{"do not infer endpoint stability across reconnect, resume, or rename", "do not send before endpoint snapshot Apply"},
			},
			Boundary: append([]string{}, pkg.Boundary...),
		}
	case "delivery-required":
		pkg.DeliveryRequest = &mission.MissionCommanderDriverRequest{
			Kind: "model-tool-handoff", RunLoopStepID: "remote-control-delivery:" + job.JobID,
			Actor: dispatch.Claim.Actor, State: inspection.State, Source: "current-loop-external-session-transport",
			Lane: lane, Label: job.SessionKind + " Remote Control SendMessage", ActionID: inspection.EndpointSHA256 + ":" + inspection.EnvelopeSHA256,
			Guidance:          "Call the Claude Code SendMessage model tool exactly once with transport.message.recipient and transport.message.message. Then record accepted, rejected, or uncertain delivery using a fresh run-current-step preview; never resend automatically after an ambiguous result.",
			CommandExecutable: false, RequiresReview: true,
			ExpectedReceipt: mission.MissionCommanderDriverReceiptExpectation{
				State: "delivery-observation-preview", RefreshStatusCommand: refresh,
				Description: "records one immutable delivery observation bound to the exact endpoint and message bytes",
				Boundary:    []string{"accepted delivery is not task completion", "uncertain delivery blocks automatic resend"},
			},
			Boundary: append([]string{}, pkg.Boundary...),
		}
	case "delivery-accepted", "delivery-rejected":
		if inspection.State == "delivery-accepted" && dispatch.State == "running" {
			request := mission.MissionCommanderDriverRequest{
				Kind: "model-tool-handoff", RunLoopStepID: "remote-control-return:" + job.JobID,
				Actor: dispatch.Claim.Actor, State: "remote-control-reviewer-result-required",
				Source: "current-loop-external-session-transport", Lane: lane,
				Label: "Remote Control ReviewerResult return", ActionID: dispatch.LaunchSHA256 + ":" + inspection.DeliverySHA256,
				Guidance:          "Capture exactly one received ReviewerResult JSON object into a bounded case-local source file. Then preview run-current-step with -ExternalSessionReviewerResultSourcePath, -ExternalSessionActor, and -ExternalSessionObservedAt; do not write submission.json manually.",
				CommandExecutable: false, RequiresReview: true,
				ExpectedReceipt: mission.MissionCommanderDriverReceiptExpectation{
					State: "transport-return-preview", RefreshStatusCommand: refresh,
					Description: "previews result-first, return-receipt-second, submission-last publication bound to the accepted delivery and launch lineage",
					Boundary:    []string{"one bounded case-local ReviewerResult source", "no manual submission write", "existing relay and strict intake remain authoritative"},
				},
				Boundary: append([]string{}, pkg.Boundary...),
			}
			pkg.ReturnRequest = &request
			break
		}
		outcome, actor, observedAt, harness, session, reason, err := externalsession.TransportLaunchTransition(inspection)
		if err == nil {
			transition, transitionErr := externalsession.PreviewDispatchTransition(job, attempt, outcome, actor, observedAt, harness, session, reason)
			if transitionErr == nil {
				request, requestErr := externalSessionDispatchRequest(job, attempt.AttemptSHA256, dispatch, outcome)
				if requestErr != nil {
					return nil, requestErr
				}
				request.Kind = "apply-command"
				request.State = "transport-delivery-" + inspection.Delivery.Outcome
				request.Source = "current-loop-external-session-transport"
				request.Command = externalSessionDispatchApplyCommand(transition)
				request.CommandExecutable = true
				request.Guidance = ""
				request.ActionID = inspection.DeliverySHA256
				request.ExpectedReceipt.Description = "derives the existing launch receipt exactly from the immutable Remote Control delivery observation"
				request.Boundary = mission.UniqueStrings(append(request.Boundary, pkg.Boundary...))
				request, requestErr = mission.MissionCommanderDriverRequestWithTypedCommand(request)
				if requestErr != nil {
					return nil, requestErr
				}
				pkg.LaunchRequest = &request
			}
		}
	case "delivery-uncertain":
		request := mission.MissionCommanderDriverRequest{
			Kind: "model-guidance", RunLoopStepID: "remote-control-redispatch:" + job.JobID,
			Actor: dispatch.Claim.Actor, State: "transport-delivery-uncertain-new-dispatch-required",
			Source: "current-loop-external-session-transport", Lane: lane,
			Label: "new durable Reviewer dispatch required", ActionID: inspection.DeliverySHA256,
			Guidance:          "Do not resend or replace this job. Create a new durable Reviewer dispatch with a new reviewerSession transport binding; let that dispatch produce a new external-session job.",
			CommandExecutable: false, RequiresReview: true,
			ExpectedReceipt: mission.MissionCommanderDriverReceiptExpectation{
				State: "new-reviewer-dispatch-required", RefreshStatusCommand: refresh,
				Description: "re-enters the canonical Reviewer dispatch path instead of reusing ambiguous transport identity",
				Boundary:    []string{"no automatic resend", "no same-job replacement", "new dispatch/session/job identity required"},
			},
			Boundary: append([]string{}, pkg.Boundary...),
		}
		pkg.ReplacementRequest = &request
	}
	return pkg, nil
}

func applyExternalSessionTransportState(operator *mission.CurrentLoopOperatorPackage, job *mission.CurrentLoopExternalSessionJob, transport externalsession.TransportInspection) {
	if operator == nil || job == nil || !transport.Applicable {
		return
	}
	pkg := job.Transport
	if pkg == nil {
		operator.Ready = false
		operator.State = "external-session-transport-invalid"
		operator.Boundary = mission.UniqueStrings(append(operator.Boundary, "Remote Control transport package is unavailable"))
		return
	}
	switch transport.State {
	case "endpoint-required":
		operator.State = "remote-control-discovery-required"
		operator.SelectedDriverRequest = pkg.DiscoveryRequest
		operator.RunbookSteps = mission.UniqueStrings(append(operator.RunbookSteps,
			"discover the exact intended Remote Control endpoint with ListAgents",
			"record the endpoint snapshot before sending any message",
		))
	case "delivery-required":
		operator.State = "remote-control-delivery-required"
		operator.SelectedDriverRequest = pkg.DeliveryRequest
		operator.RunbookSteps = mission.UniqueStrings(append(operator.RunbookSteps,
			"send the exact content-addressed message envelope once with SendMessage",
			"record accepted, rejected, or uncertain delivery without inferring completion",
		))
	case "delivery-accepted", "delivery-rejected":
		if transport.State == "delivery-accepted" && pkg.ReturnRequest != nil {
			operator.State = "remote-control-reviewer-result-required"
			operator.SelectedDriverRequest = pkg.ReturnRequest
			operator.RunbookSteps = mission.UniqueStrings(append(operator.RunbookSteps,
				"capture exactly one bounded ReviewerResult into a case-local source file",
				"use the deterministic return producer; do not write submission.json manually",
			))
			break
		}
		operator.State = "remote-control-launch-receipt-ready"
		operator.SelectedDriverRequest = pkg.LaunchRequest
		operator.RunbookSteps = mission.UniqueStrings(append(operator.RunbookSteps,
			"derive the existing launch receipt from the immutable delivery observation",
			"refresh status before waiting for the ReviewerResult return",
		))
	case "delivery-uncertain":
		operator.State = "remote-control-delivery-uncertain"
		operator.SelectedDriverRequest = pkg.ReplacementRequest
		operator.RunbookSteps = mission.UniqueStrings(append(operator.RunbookSteps,
			"do not resend the ambiguous SendMessage envelope or replace this job",
			"create a new durable Reviewer dispatch with a new session binding and external-session job",
		))
	default:
		operator.Ready = false
		operator.State = "external-session-transport-invalid"
		operator.Boundary = mission.UniqueStrings(append(operator.Boundary, fmt.Sprintf("unsupported Remote Control transport state %q", transport.State)))
	}
}
