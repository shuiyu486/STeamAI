package cli

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"strings"

	"github.com/shuiyu486/re-context-kits/internal/rekit/commands"
	"github.com/shuiyu486/re-context-kits/internal/rekit/currentloop"
	"github.com/shuiyu486/re-context-kits/internal/rekit/mission"
	"github.com/shuiyu486/re-context-kits/internal/rekit/runtime"
)

const currentLoopRoutePolicy = "fixed-initial-route-and-lane"

var currentLoopBeforeApplyStepHook func(int) error

type currentLoopPlan struct {
	SchemaVersion                 int                                    `json:"schemaVersion"`
	Command                       string                                 `json:"command"`
	CaseRoot                      string                                 `json:"caseRoot"`
	Pack                          string                                 `json:"pack"`
	Actor                         string                                 `json:"actor"`
	RoutePolicy                   string                                 `json:"routePolicy"`
	MaxSteps                      int                                    `json:"maxSteps"`
	AppliedSteps                  int                                    `json:"appliedSteps"`
	IsMutation                    bool                                   `json:"isMutation"`
	Applied                       bool                                   `json:"applied"`
	ReviewRequired                bool                                   `json:"reviewRequired"`
	RequiresConfirmation          bool                                   `json:"requiresConfirmation"`
	InitialRoute                  string                                 `json:"initialRoute"`
	InitialLane                   string                                 `json:"initialLane"`
	InitialCurrentDriverRequest   *mission.MissionCommanderDriverRequest `json:"initialCurrentDriverRequest,omitempty"`
	InitialCurrentStep            *currentStepPlan                       `json:"initialCurrentStep,omitempty"`
	ExpectedCurrentLoopPlanSHA256 string                                 `json:"expectedCurrentLoopPlanSha256,omitempty"`
	Steps                         []currentLoopStepReceipt               `json:"steps"`
	StopReason                    currentLoopStopReason                  `json:"stopReason"`
	ResumeCommand                 string                                 `json:"resumeCommand"`
	ContinuationRequest           *currentLoopContinuationRequest        `json:"continuationRequest,omitempty"`
	SegmentCheckpoint             *currentloop.Inspection                `json:"segmentCheckpoint,omitempty"`
	FinalStatus                   *statusInventory                       `json:"finalStatus,omitempty"`
	Boundary                      []string                               `json:"boundary"`
}

type currentLoopStepReceipt struct {
	Step                          int                                    `json:"step"`
	Route                         string                                 `json:"route"`
	Lane                          string                                 `json:"lane"`
	RunLoopStepID                 string                                 `json:"runLoopStepId"`
	ExpectedCurrentStepPlanSHA256 string                                 `json:"expectedCurrentStepPlanSha256"`
	RequestBefore                 mission.MissionCommanderDriverRequest  `json:"requestBefore"`
	CurrentStepReceipt            *currentStepReceipt                    `json:"currentStepReceipt,omitempty"`
	RequestAfter                  *mission.MissionCommanderDriverRequest `json:"requestAfter,omitempty"`
}

type currentLoopStopReason struct {
	Code                 string                                 `json:"code"`
	Phase                string                                 `json:"phase"`
	Message              string                                 `json:"message"`
	CurrentDriverRequest *mission.MissionCommanderDriverRequest `json:"currentDriverRequest,omitempty"`
	ExternalHandoff      *reviewerStepExternalHandoff           `json:"externalHandoff,omitempty"`
}

type currentLoopContinuationRequest struct {
	Kind                  string                           `json:"kind"`
	State                 string                           `json:"state"`
	StopCode              string                           `json:"stopCode"`
	SegmentMaxSteps       int                              `json:"segmentMaxSteps"`
	AppliedStepsInSegment int                              `json:"appliedStepsInSegment"`
	RemainingMaxSteps     int                              `json:"remainingMaxSteps"`
	SegmentRoute          string                           `json:"segmentRoute"`
	SegmentLane           string                           `json:"segmentLane"`
	ExpectedRoute         string                           `json:"expectedRoute"`
	ExpectedLane          string                           `json:"expectedLane"`
	WhatIfCommand         string                           `json:"whatIfCommand"`
	ObservationContract   *reviewerStepObservationContract `json:"observationContract,omitempty"`
	FreshPreviewRequired  bool                             `json:"freshPreviewRequired"`
	CumulativeReceipts    bool                             `json:"cumulativeReceipts"`
	Boundary              []string                         `json:"boundary"`
}

type currentLoopPlanIdentity struct {
	SchemaVersion                 int                                   `json:"schemaVersion"`
	CaseRoot                      string                                `json:"caseRoot"`
	Pack                          string                                `json:"pack"`
	RoutePolicy                   string                                `json:"routePolicy"`
	MaxSteps                      int                                   `json:"maxSteps"`
	Actor                         string                                `json:"actor"`
	InitialRoute                  string                                `json:"initialRoute"`
	InitialLane                   string                                `json:"initialLane"`
	InitialCurrentDriverRequest   mission.MissionCommanderDriverRequest `json:"initialCurrentDriverRequest"`
	ExpectedCurrentStepPlanSHA256 string                                `json:"expectedCurrentStepPlanSha256"`
}

func runCurrentLoop(ctx runtime.Context, opt Options, out io.Writer) error {
	if !ctx.TargetProvided {
		return fmt.Errorf("run-current-loop requires -Target for an attached case")
	}
	if opt.WhatIf && opt.Apply {
		return fmt.Errorf("run-current-loop cannot combine -WhatIf and -Apply")
	}
	if !opt.WhatIf && !opt.Apply {
		return fmt.Errorf("run-current-loop requires -WhatIf or -Apply")
	}
	if strings.ToLower(strings.TrimSpace(opt.Format)) != "json" {
		return fmt.Errorf("run-current-loop supports only -Format json")
	}
	if opt.MaxSteps < 1 || opt.MaxSteps > 20 {
		return fmt.Errorf("run-current-loop requires -MaxSteps between 1 and 20")
	}
	if err := validateCurrentLoopOuterArgs(opt); err != nil {
		return err
	}
	plan, status, err := buildCurrentLoopPlan(ctx, opt)
	if err != nil {
		return err
	}
	if opt.WhatIf {
		if strings.TrimSpace(opt.ExpectedCurrentLoopPlanSHA256) != "" {
			return fmt.Errorf("run-current-loop -WhatIf does not accept -ExpectedCurrentLoopPlanSha256")
		}
		return writeJSON(out, plan)
	}
	if plan.ExpectedCurrentLoopPlanSHA256 == "" {
		return fmt.Errorf("run-current-loop current route requires an external or reviewed action before Apply")
	}
	expected := strings.TrimSpace(opt.ExpectedCurrentLoopPlanSHA256)
	if expected == "" {
		return fmt.Errorf("run-current-loop -Apply requires -ExpectedCurrentLoopPlanSha256 from -WhatIf")
	}
	if !strings.EqualFold(expected, plan.ExpectedCurrentLoopPlanSHA256) {
		return fmt.Errorf("run-current-loop expected plan sha256 mismatch: got %s want %s", expected, plan.ExpectedCurrentLoopPlanSHA256)
	}
	plan, err = applyCurrentLoopPlan(ctx, opt, plan, status)
	if err != nil {
		return err
	}
	if plan.AppliedSteps > 0 {
		inspection, checkpointErr := writeCurrentLoopSegmentCheckpoint(ctx, plan)
		plan.SegmentCheckpoint = &inspection
		if checkpointErr != nil {
			plan.Boundary = append(plan.Boundary,
				"durable current-loop segment checkpoint publication failed; this invocation result remains the only receipt and status must not recover its budget",
			)
		}
	}
	return writeJSON(out, plan)
}

func buildCurrentLoopPlan(ctx runtime.Context, opt Options) (currentLoopPlan, statusInventory, error) {
	status, err := buildStatusInventory(ctx, statusPackSource(ctx, opt))
	if err != nil {
		return currentLoopPlan{}, statusInventory{}, err
	}
	plan := currentLoopPlan{
		SchemaVersion:        1,
		Command:              commands.RunCurrentLoop,
		CaseRoot:             ctx.Target,
		Pack:                 ctx.Pack,
		Actor:                strings.TrimSpace(opt.Start.Actor),
		RoutePolicy:          currentLoopRoutePolicy,
		MaxSteps:             opt.MaxSteps,
		ReviewRequired:       true,
		RequiresConfirmation: true,
		Steps:                []currentLoopStepReceipt{},
		ResumeCommand:        currentLoopResumeCommand(ctx, opt.MaxSteps),
		Boundary: []string{
			"the loop hash conditionally authorizes only the exact initial step plus at most maxSteps under a fixed initial route and lane",
			"every later step is rebuilt from refreshed durable status and retains the current-step and nested runner hash, lease, packet, artifact, and lock guards",
			"the loop stops before external session work, newly surfaced Human-in-the-Lane reconciliation, guidance-only or blocked actions, route or lane changes, repeated exact step plans, and the configured step limit",
			"the loop is not transactional; completed step receipts remain valid if a later step cannot continue",
			"route or lane drift, a refreshed Human-in-the-Lane review, and external reviewer handoff return a typed campaign continuation when deterministic budget remains; every continuation starts a fresh reviewed segment",
			"campaign receipts remain segment-local and are not accumulated across invocations",
			"the Go runtime does not invoke a shell or Agent tool, spawn, poll, or stop sessions, fabricate reviewer output, execute heavy tools, or write authority/confirmed state",
		},
	}
	request, route, stop := currentLoopStatusCandidate(status)
	if stop.Code != "" {
		plan.StopReason = stop
		plan.FinalStatus = &status
		return plan, status, nil
	}
	plan.InitialRoute = route
	plan.InitialLane = strings.TrimSpace(request.Lane)
	plan.InitialCurrentDriverRequest = request
	step, err := buildCurrentStepPlanFromStatus(ctx, opt, status)
	if err != nil {
		plan.StopReason = currentLoopStopReason{Code: "requires-review", Phase: "preview", Message: err.Error(), CurrentDriverRequest: request}
		plan.FinalStatus = &status
		return plan, status, nil
	}
	plan.InitialCurrentStep = &step
	if step.ExpectedCurrentStepPlanSHA256 == "" {
		plan.StopReason = currentLoopExternalStop(step, request)
		plan.ContinuationRequest = currentLoopContinuationFor(ctx, opt.MaxSteps, 0, route, plan.InitialLane, route, plan.StopReason)
		if plan.ContinuationRequest != nil {
			plan.ResumeCommand = plan.ContinuationRequest.WhatIfCommand
		}
		plan.FinalStatus = &status
		return plan, status, nil
	}
	identity := currentLoopPlanIdentity{
		SchemaVersion:                 1,
		CaseRoot:                      ctx.Target,
		Pack:                          ctx.Pack,
		RoutePolicy:                   currentLoopRoutePolicy,
		MaxSteps:                      opt.MaxSteps,
		Actor:                         strings.TrimSpace(opt.Start.Actor),
		InitialRoute:                  plan.InitialRoute,
		InitialLane:                   plan.InitialLane,
		InitialCurrentDriverRequest:   *request,
		ExpectedCurrentStepPlanSHA256: step.ExpectedCurrentStepPlanSHA256,
	}
	encoded, err := json.Marshal(identity)
	if err != nil {
		return currentLoopPlan{}, statusInventory{}, err
	}
	sum := sha256.Sum256(encoded)
	plan.ExpectedCurrentLoopPlanSHA256 = hex.EncodeToString(sum[:])
	plan.StopReason = currentLoopStopReason{Code: "ready", Phase: "preview", Message: "bounded current loop is ready for hash-bound Apply", CurrentDriverRequest: request}
	return plan, status, nil
}

func applyCurrentLoopPlan(ctx runtime.Context, opt Options, plan currentLoopPlan, status statusInventory) (currentLoopPlan, error) {
	initialRoute := plan.InitialRoute
	initialLane := plan.InitialLane
	seen := map[string]bool{}
	stepOpt := opt
	for stepNumber := 1; stepNumber <= plan.MaxSteps; stepNumber++ {
		request, route, stop := currentLoopStatusCandidate(status)
		if stop.Code != "" {
			plan.StopReason = stop
			break
		}
		if route != initialRoute || strings.TrimSpace(request.Lane) != initialLane {
			plan.StopReason = currentLoopStopReason{Code: "route-policy", Phase: "before-step", Message: "refreshed route or lane changed; review a fresh loop preview", CurrentDriverRequest: request}
			plan.ContinuationRequest = currentLoopContinuationFor(ctx, plan.MaxSteps, plan.AppliedSteps, initialRoute, initialLane, route, plan.StopReason)
			if plan.ContinuationRequest != nil {
				plan.ResumeCommand = plan.ContinuationRequest.WhatIfCommand
			}
			break
		}
		if policyStop := currentLoopBeforeStepPolicyStop(stepNumber, route, request); policyStop.Code != "" {
			plan.StopReason = policyStop
			plan.ContinuationRequest = currentLoopContinuationFor(ctx, plan.MaxSteps, plan.AppliedSteps, initialRoute, initialLane, route, plan.StopReason)
			if plan.ContinuationRequest != nil {
				plan.ResumeCommand = plan.ContinuationRequest.WhatIfCommand
			}
			break
		}
		step, err := buildCurrentStepPlanFromStatus(ctx, stepOpt, status)
		if err != nil {
			plan.StopReason = currentLoopStopReason{Code: "requires-review", Phase: "before-step", Message: err.Error(), CurrentDriverRequest: request}
			break
		}
		if step.ExpectedCurrentStepPlanSHA256 == "" {
			plan.StopReason = currentLoopExternalStop(step, request)
			plan.ContinuationRequest = currentLoopContinuationFor(ctx, plan.MaxSteps, plan.AppliedSteps, initialRoute, initialLane, route, plan.StopReason)
			if plan.ContinuationRequest != nil {
				plan.ResumeCommand = plan.ContinuationRequest.WhatIfCommand
			}
			break
		}
		requestKey, err := currentLoopRequestKey(route, *request, step.ExpectedCurrentStepPlanSHA256)
		if err != nil {
			return currentLoopPlan{}, err
		}
		if seen[requestKey] {
			plan.StopReason = currentLoopStopReason{Code: "no-progress", Phase: "before-step", Message: "refreshed current driver request and exact step plan repeated without progress", CurrentDriverRequest: request}
			break
		}
		seen[requestKey] = true
		if currentLoopBeforeApplyStepHook != nil {
			if err := currentLoopBeforeApplyStepHook(stepNumber); err != nil {
				plan.StopReason = currentLoopApplyErrorStop(stepNumber, request, err)
				break
			}
		}
		applied, err := applyCurrentStepPlan(ctx, stepOpt, step)
		if err != nil {
			if applied.Applied && applied.Receipt != nil {
				plan.Steps = append(plan.Steps, currentLoopStepReceipt{
					Step:                          stepNumber,
					Route:                         route,
					Lane:                          strings.TrimSpace(request.Lane),
					RunLoopStepID:                 strings.TrimSpace(request.RunLoopStepID),
					ExpectedCurrentStepPlanSHA256: step.ExpectedCurrentStepPlanSHA256,
					RequestBefore:                 *request,
					CurrentStepReceipt:            applied.Receipt,
				})
				plan.AppliedSteps++
				plan.Applied = true
				plan.FinalStatus = nil
				plan.StopReason = currentLoopRefreshErrorStop(stepNumber, request, err)
				plan.IsMutation = true
				plan.ReviewRequired = false
				plan.RequiresConfirmation = false
				return plan, nil
			}
			plan.StopReason = currentLoopApplyErrorStop(stepNumber, request, err)
			break
		}
		if !applied.Applied || applied.Receipt == nil || applied.RefreshedStatus == nil {
			plan.StopReason = currentLoopStopReason{Code: "no-progress", Phase: "after-step", Message: "current step did not produce an applied receipt and refreshed status", CurrentDriverRequest: request}
			break
		}
		receipt := currentLoopStepReceipt{
			Step:                          stepNumber,
			Route:                         route,
			Lane:                          strings.TrimSpace(request.Lane),
			RunLoopStepID:                 strings.TrimSpace(request.RunLoopStepID),
			ExpectedCurrentStepPlanSHA256: step.ExpectedCurrentStepPlanSHA256,
			RequestBefore:                 *request,
			CurrentStepReceipt:            applied.Receipt,
		}
		status = *applied.RefreshedStatus
		stepOpt = currentLoopFollowupOptions(stepOpt)
		if status.MissionControlRunbook != nil {
			receipt.RequestAfter = status.MissionControlRunbook.CurrentDriverRequest
			receipt.CurrentStepReceipt.RefreshedCurrentDriverRequest = status.MissionControlRunbook.CurrentDriverRequest
		}
		plan.Steps = append(plan.Steps, receipt)
		plan.AppliedSteps++
		plan.Applied = true
		if stepNumber == plan.MaxSteps {
			if nextRequest, nextRoute, nextStop := currentLoopStatusCandidate(status); nextStop.Code != "" {
				plan.StopReason = nextStop
			} else if nextRoute != initialRoute || strings.TrimSpace(nextRequest.Lane) != initialLane {
				plan.StopReason = currentLoopStopReason{Code: "route-policy", Phase: "after-step", Message: "refreshed route or lane changed; review a fresh loop preview", CurrentDriverRequest: nextRequest}
			} else if policyStop := currentLoopBeforeStepPolicyStop(stepNumber+1, nextRoute, nextRequest); policyStop.Code != "" {
				plan.StopReason = policyStop
			} else {
				plan.StopReason = currentLoopStopReason{Code: "limit-reached", Phase: "after-step", Message: "bounded current loop reached maxSteps", CurrentDriverRequest: nextRequest}
			}
		}
	}
	plan.IsMutation = true
	plan.ReviewRequired = false
	plan.RequiresConfirmation = false
	plan.FinalStatus = &status
	return plan, nil
}

func currentLoopStatusCandidate(status statusInventory) (*mission.MissionCommanderDriverRequest, string, currentLoopStopReason) {
	if status.MissionControlRunbook == nil || status.MissionControlRunbook.CurrentDriverRequest == nil {
		return nil, "", currentLoopStopReason{Code: "no-current-request", Phase: "status", Message: "refreshed status has no current driver request"}
	}
	request := status.MissionControlRunbook.CurrentDriverRequest
	route := strings.TrimSpace(status.MissionControlRunbook.Scope)
	if route != "case" && route != "reviewer" {
		return request, route, currentLoopStopReason{Code: "route-policy", Phase: "status", Message: fmt.Sprintf("focused scope %q is outside the bounded case/reviewer loop", route), CurrentDriverRequest: request}
	}
	if request.Blocked {
		return request, route, currentLoopStopReason{Code: "blocked", Phase: "status", Message: "current driver request is blocked", CurrentDriverRequest: request}
	}
	if route != "reviewer" && (!request.CommandExecutable || strings.TrimSpace(request.Command) == "") {
		return request, route, currentLoopStopReason{Code: "guidance-only", Phase: "status", Message: "current driver request requires main-agent guidance instead of command execution", CurrentDriverRequest: request}
	}
	return request, route, currentLoopStopReason{}
}

func currentLoopBeforeStepPolicyStop(stepNumber int, route string, request *mission.MissionCommanderDriverRequest) currentLoopStopReason {
	if stepNumber > 1 && route == "case" && request != nil && driverStepCommandName(request.Command) == commands.Reconcile {
		return currentLoopStopReason{Code: "human-intervention", Phase: "before-step", Message: "a refreshed Human-in-the-Lane intervention requires a fresh loop preview before reconcile", CurrentDriverRequest: request}
	}
	return currentLoopStopReason{}
}

func currentLoopApplyErrorStop(stepNumber int, request *mission.MissionCommanderDriverRequest, err error) currentLoopStopReason {
	return currentLoopStopReason{
		Code:                 "error",
		Phase:                "apply-step",
		Message:              fmt.Sprintf("run-current-loop step %d: %v", stepNumber, err),
		CurrentDriverRequest: request,
	}
}

func currentLoopRefreshErrorStop(stepNumber int, request *mission.MissionCommanderDriverRequest, err error) currentLoopStopReason {
	return currentLoopStopReason{
		Code:                 "error",
		Phase:                "refresh-status",
		Message:              fmt.Sprintf("run-current-loop step %d was applied but status refresh failed: %v", stepNumber, err),
		CurrentDriverRequest: request,
	}
}

func currentLoopExternalStop(step currentStepPlan, request *mission.MissionCommanderDriverRequest) currentLoopStopReason {
	stop := currentLoopStopReason{Code: "requires-review", Phase: "before-step", Message: "current step has no deterministic Apply hash", CurrentDriverRequest: request}
	if step.ReviewerStep != nil && step.ReviewerStep.ExternalHandoff != nil {
		stop.Code = "external-reviewer-handoff"
		stop.Message = "reviewer lifecycle requires an external harness action before the loop can resume"
		stop.ExternalHandoff = step.ReviewerStep.ExternalHandoff
	}
	return stop
}

func currentLoopRequestKey(route string, request mission.MissionCommanderDriverRequest, stepSHA256 string) (string, error) {
	encoded, err := json.Marshal(struct {
		Route      string                                `json:"route"`
		Request    mission.MissionCommanderDriverRequest `json:"request"`
		StepSHA256 string                                `json:"stepSha256"`
	}{Route: route, Request: request, StepSHA256: stepSHA256})
	if err != nil {
		return "", err
	}
	sum := sha256.Sum256(encoded)
	return hex.EncodeToString(sum[:]), nil
}

func currentLoopResumeCommand(ctx runtime.Context, maxSteps int) string {
	return fmt.Sprintf("/rekit run-current-loop -Target %s -Pack %s -MaxSteps %d -WhatIf -Format json", statusQuoteCommandArg(ctx.Target), statusQuoteCommandArg(ctx.Pack), maxSteps)
}

func currentLoopContinuationFor(ctx runtime.Context, segmentMaxSteps, appliedSteps int, segmentRoute, segmentLane, expectedRoute string, stop currentLoopStopReason) *currentLoopContinuationRequest {
	if stop.Code != "external-reviewer-handoff" && stop.Code != "route-policy" && stop.Code != "human-intervention" {
		return nil
	}
	if stop.Code == "external-reviewer-handoff" && stop.ExternalHandoff == nil {
		return nil
	}
	remaining := segmentMaxSteps - appliedSteps
	if remaining < 1 || stop.CurrentDriverRequest == nil {
		return nil
	}
	continuation := &currentLoopContinuationRequest{
		Kind:                  "current-loop-campaign-continuation",
		State:                 "awaiting-fresh-segment-review",
		StopCode:              stop.Code,
		SegmentMaxSteps:       segmentMaxSteps,
		AppliedStepsInSegment: appliedSteps,
		RemainingMaxSteps:     remaining,
		SegmentRoute:          segmentRoute,
		SegmentLane:           segmentLane,
		ExpectedRoute:         expectedRoute,
		ExpectedLane:          strings.TrimSpace(stop.CurrentDriverRequest.Lane),
		WhatIfCommand:         currentLoopResumeCommand(ctx, remaining),
		FreshPreviewRequired:  true,
		CumulativeReceipts:    false,
		Boundary: []string{
			"the continuation starts a fresh hash-bound loop segment from refreshed durable status; it does not carry the previous segment hash across the boundary",
			"the continuation retains only the remaining deterministic step budget; receipts stay in the previous segment result and are not accumulated across invocations",
			"the expected route and lane describe the reviewed transition target and must be revalidated by the fresh preview",
			"without this typed result, status may start a fresh loop preview but cannot claim recovery of the previous segment budget",
			"the continuation request is an orchestration handoff, not authority or a durable authorization token",
		},
	}
	if stop.Code == "external-reviewer-handoff" && stop.ExternalHandoff != nil {
		observation := stop.ExternalHandoff.ObservationContract
		continuation.State = "awaiting-external-observation"
		continuation.ObservationContract = &observation
		continuation.Boundary = append([]string{
			"consume one observation alternative by adding its exact flags to whatIfCommand; do not execute guidance text as a shell command",
		}, continuation.Boundary...)
	}
	if stop.Code == "human-intervention" {
		continuation.Boundary = append([]string{
			"review the refreshed Human-in-the-Lane intervention in the fresh segment before applying reconcile",
		}, continuation.Boundary...)
	}
	return continuation
}

func writeCurrentLoopSegmentCheckpoint(ctx runtime.Context, plan currentLoopPlan) (currentloop.Inspection, error) {
	payload := currentloop.Payload{
		Actor:                         plan.Actor,
		RoutePolicy:                   plan.RoutePolicy,
		InitialCurrentDriverRequest:   *plan.InitialCurrentDriverRequest,
		ExpectedCurrentLoopPlanSHA256: plan.ExpectedCurrentLoopPlanSHA256,
		SegmentMaxSteps:               plan.MaxSteps,
		AppliedStepsInSegment:         plan.AppliedSteps,
		RemainingMaxSteps:             plan.MaxSteps - plan.AppliedSteps,
		SegmentRoute:                  plan.InitialRoute,
		SegmentLane:                   plan.InitialLane,
		Stop: currentloop.Stop{
			Code:  plan.StopReason.Code,
			Phase: plan.StopReason.Phase,
		},
		StepReceipts:    make([]currentloop.StepReceiptBinding, 0, len(plan.Steps)),
		StatusAvailable: plan.FinalStatus != nil,
		Boundary: []string{
			"checkpoint records one completed outer current-loop segment and exact receipt hashes; it does not carry executable nested receipts into a fresh invocation",
			"only a strict fresh status match may expose the typed continuation and remaining budget",
			"checkpoint publication does not execute a continuation or write authority/confirmed state",
		},
	}
	initialRequestSHA256, err := currentloop.RequestSHA256(payload.InitialCurrentDriverRequest)
	if err != nil {
		return currentloop.FailedInspection(err.Error()), err
	}
	payload.InitialCurrentDriverRequestSHA256 = initialRequestSHA256
	for _, receipt := range plan.Steps {
		requestBeforeSHA256, err := currentloop.RequestSHA256(receipt.RequestBefore)
		if err != nil {
			return currentloop.FailedInspection(err.Error()), err
		}
		currentStepReceiptSHA256, err := currentloop.ValueSHA256(receipt.CurrentStepReceipt)
		if err != nil {
			return currentloop.FailedInspection(err.Error()), err
		}
		binding := currentloop.StepReceiptBinding{
			Step:                          receipt.Step,
			Route:                         receipt.Route,
			Lane:                          receipt.Lane,
			RunLoopStepID:                 receipt.RunLoopStepID,
			ExpectedCurrentStepPlanSHA256: receipt.ExpectedCurrentStepPlanSHA256,
			RequestBefore:                 receipt.RequestBefore,
			RequestBeforeSHA256:           requestBeforeSHA256,
			CurrentStepReceipt: currentloop.StepReceipt{
				State:                         receipt.CurrentStepReceipt.State,
				Outcome:                       receipt.CurrentStepReceipt.Outcome,
				Route:                         receipt.CurrentStepReceipt.Route,
				NestedCommand:                 receipt.CurrentStepReceipt.NestedCommand,
				RefreshedCurrentDriverRequest: receipt.CurrentStepReceipt.RefreshedCurrentDriverRequest,
				Boundary:                      append([]string(nil), receipt.CurrentStepReceipt.Boundary...),
			},
			CurrentStepReceiptSHA256: currentStepReceiptSHA256,
			RequestAfter:             receipt.RequestAfter,
		}
		if receipt.RequestAfter != nil {
			binding.RequestAfterSHA256, err = currentloop.RequestSHA256(*receipt.RequestAfter)
			if err != nil {
				return currentloop.FailedInspection(err.Error()), err
			}
		}
		payload.StepReceipts = append(payload.StepReceipts, binding)
	}
	if plan.FinalStatus != nil && plan.FinalStatus.MissionControlRunbook != nil {
		payload.RefreshedCurrentDriverRequest = plan.FinalStatus.MissionControlRunbook.CurrentDriverRequest
	}
	if payload.RefreshedCurrentDriverRequest != nil {
		requestSHA256, err := currentloop.RequestSHA256(*payload.RefreshedCurrentDriverRequest)
		if err != nil {
			return currentloop.FailedInspection(err.Error()), err
		}
		payload.RefreshedCurrentDriverRequestSHA256 = requestSHA256
	}
	if plan.ContinuationRequest != nil {
		payload.Continuation = currentLoopCheckpointContinuation(plan.ContinuationRequest)
	}
	inspection, err := currentloop.Write(ctx.RepoRoot, ctx.Target, ctx.Pack, payload)
	if err != nil {
		return currentloop.FailedInspection(err.Error()), err
	}
	return inspection, nil
}

func currentLoopCheckpointContinuation(source *currentLoopContinuationRequest) *currentloop.Continuation {
	if source == nil {
		return nil
	}
	continuation := &currentloop.Continuation{
		Kind:                  source.Kind,
		State:                 source.State,
		StopCode:              source.StopCode,
		SegmentMaxSteps:       source.SegmentMaxSteps,
		AppliedStepsInSegment: source.AppliedStepsInSegment,
		RemainingMaxSteps:     source.RemainingMaxSteps,
		SegmentRoute:          source.SegmentRoute,
		SegmentLane:           source.SegmentLane,
		ExpectedRoute:         source.ExpectedRoute,
		ExpectedLane:          source.ExpectedLane,
		WhatIfCommand:         source.WhatIfCommand,
		FreshPreviewRequired:  source.FreshPreviewRequired,
		CumulativeReceipts:    source.CumulativeReceipts,
		Boundary:              append([]string{}, source.Boundary...),
	}
	if source.ObservationContract != nil {
		observation := &currentloop.ObservationContract{
			Boundary: append([]string{}, source.ObservationContract.Boundary...),
		}
		for _, alternative := range source.ObservationContract.Alternatives {
			observation.Alternatives = append(observation.Alternatives, currentloop.ObservationAlternative{
				Kind:          alternative.Kind,
				RequiredFlags: append([]string{}, alternative.RequiredFlags...),
				Constraints:   append([]string{}, alternative.Constraints...),
			})
		}
		continuation.ObservationContract = observation
	}
	return continuation
}

func currentLoopFollowupOptions(opt Options) Options {
	opt.ReviewerResultInputSourcePath = ""
	opt.ReviewerHarness = ""
	opt.ReviewerSession = ""
	opt.ReviewerOutcome = ""
	opt.ReviewerExitStatus = ""
	return opt
}

func validateCurrentLoopOuterArgs(opt Options) error {
	valueFlags := map[string]bool{
		"-command": true, "--command": true,
		"-target": true, "--target": true,
		"-pack": true, "--pack": true,
		"-format": true, "--format": true,
		"-maxsteps": true, "--max-steps": true,
		"-expectedcurrentloopplansha256": true, "--expected-current-loop-plan-sha256": true,
		"-actor": true, "--actor": true,
		"-reviewerresultinputsourcepath": true, "--reviewer-result-input-source-path": true,
		"-reviewerharness": true, "--reviewer-harness": true,
		"-reviewersession": true, "--reviewer-session": true,
		"-revieweroutcome": true, "--reviewer-outcome": true,
		"-reviewerexitstatus": true, "--reviewer-exit-status": true,
	}
	switchFlags := map[string]bool{
		"-whatif": true, "--what-if": true,
		"-apply": true, "--apply": true,
	}
	seen := map[string]bool{}
	separatorSeen := false
	for i := 0; i < len(opt.rawArgs); i++ {
		token := opt.rawArgs[i]
		if token == "--" {
			if i != 0 || separatorSeen {
				return fmt.Errorf("run-current-loop accepts -- only once at the start of the argument list")
			}
			separatorSeen = true
			continue
		}
		key := strings.ToLower(strings.SplitN(token, "=", 2)[0])
		if !strings.HasPrefix(key, "-") {
			return fmt.Errorf("run-current-loop contains unsupported positional argument %s", token)
		}
		canonical := currentLoopCanonicalOuterFlag(key)
		if key != canonical && !valueFlags[key] && !switchFlags[key] {
			return fmt.Errorf("run-current-loop contains unsupported flag %s", token)
		}
		if seen[canonical] {
			return fmt.Errorf("run-current-loop repeats flag %s", token)
		}
		seen[canonical] = true
		if switchFlags[key] {
			continue
		}
		if !valueFlags[key] {
			return fmt.Errorf("run-current-loop contains unsupported flag %s", token)
		}
		if !strings.Contains(token, "=") {
			if i+1 >= len(opt.rawArgs) || strings.HasPrefix(opt.rawArgs[i+1], "-") {
				return fmt.Errorf("run-current-loop flag %s is missing a value", token)
			}
			i++
		}
	}
	return nil
}

func currentLoopCanonicalOuterFlag(key string) string {
	switch key {
	case "--command":
		return "-command"
	case "--target":
		return "-target"
	case "--pack":
		return "-pack"
	case "--format":
		return "-format"
	case "--max-steps":
		return "-maxsteps"
	case "--expected-current-loop-plan-sha256":
		return "-expectedcurrentloopplansha256"
	case "--what-if":
		return "-whatif"
	case "--apply":
		return "-apply"
	case "--actor":
		return "-actor"
	case "--reviewer-result-input-source-path":
		return "-reviewerresultinputsourcepath"
	case "--reviewer-harness":
		return "-reviewerharness"
	case "--reviewer-session":
		return "-reviewersession"
	case "--reviewer-outcome":
		return "-revieweroutcome"
	case "--reviewer-exit-status":
		return "-reviewerexitstatus"
	default:
		return key
	}
}
