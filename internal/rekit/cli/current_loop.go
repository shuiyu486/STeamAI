package cli

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"path/filepath"
	"strings"

	"github.com/shuiyu486/re-context-kits/internal/rekit/commands"
	"github.com/shuiyu486/re-context-kits/internal/rekit/currentloop"
	"github.com/shuiyu486/re-context-kits/internal/rekit/memberexecution"
	"github.com/shuiyu486/re-context-kits/internal/rekit/mission"
	"github.com/shuiyu486/re-context-kits/internal/rekit/runtime"
)

const currentLoopRoutePolicy = "fixed-initial-route-and-lane"

var currentLoopBeforeApplyStepHook func(int) error

type currentLoopPlan struct {
	SchemaVersion                  int                                    `json:"schemaVersion"`
	Command                        string                                 `json:"command"`
	CaseRoot                       string                                 `json:"caseRoot"`
	Pack                           string                                 `json:"pack"`
	Actor                          string                                 `json:"actor"`
	RoutePolicy                    string                                 `json:"routePolicy"`
	MaxSteps                       int                                    `json:"maxSteps"`
	AppliedSteps                   int                                    `json:"appliedSteps"`
	IsMutation                     bool                                   `json:"isMutation"`
	Applied                        bool                                   `json:"applied"`
	ReviewRequired                 bool                                   `json:"reviewRequired"`
	RequiresConfirmation           bool                                   `json:"requiresConfirmation"`
	InitialRoute                   string                                 `json:"initialRoute"`
	InitialLane                    string                                 `json:"initialLane"`
	InitialCurrentDriverRequest    *mission.MissionCommanderDriverRequest `json:"initialCurrentDriverRequest,omitempty"`
	InitialCurrentStep             *currentStepPlan                       `json:"initialCurrentStep,omitempty"`
	ExpectedCurrentLoopPlanSHA256  string                                 `json:"expectedCurrentLoopPlanSha256,omitempty"`
	Steps                          []currentLoopStepReceipt               `json:"steps"`
	StopReason                     currentLoopStopReason                  `json:"stopReason"`
	ResumeCommand                  string                                 `json:"resumeCommand"`
	ContinuationRequest            *currentLoopContinuationRequest        `json:"continuationRequest,omitempty"`
	ResumeSource                   *currentloop.Inspection                `json:"resumeSource,omitempty"`
	ExpectedResumeCheckpointSHA256 string                                 `json:"expectedResumeCheckpointSha256,omitempty"`
	ObservationPath                string                                 `json:"observationPath,omitempty"`
	ObservationSHA256              string                                 `json:"observationSha256,omitempty"`
	ObservationKind                string                                 `json:"observationKind,omitempty"`
	ObservationActor               string                                 `json:"observationActor,omitempty"`
	ObservationReceipt             *currentLoopObservationReceipt         `json:"observationReceipt,omitempty"`
	ApplyCommand                   string                                 `json:"applyCommand,omitempty"`
	SegmentCheckpoint              *currentloop.Inspection                `json:"segmentCheckpoint,omitempty"`
	FinalStatus                    *statusInventory                       `json:"finalStatus,omitempty"`
	Boundary                       []string                               `json:"boundary"`
	zeroProgressVerified           bool                                   `json:"-"`
}

type currentLoopObservationReceipt = mission.CurrentLoopObservationReceipt

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
	Code                          string                                    `json:"code"`
	Phase                         string                                    `json:"phase"`
	Message                       string                                    `json:"message"`
	CurrentDriverRequest          *mission.MissionCommanderDriverRequest    `json:"currentDriverRequest,omitempty"`
	ExternalHandoff               *reviewerStepExternalHandoff              `json:"externalHandoff,omitempty"`
	ExternalMemberHandoff         *mission.CurrentLoopExternalMemberHandoff `json:"externalMemberHandoff,omitempty"`
	ExpectedReviewerAttemptSHA256 string                                    `json:"expectedReviewerAttemptSha256,omitempty"`
}

type currentLoopContinuationRequest struct {
	Kind                  string                                    `json:"kind"`
	State                 string                                    `json:"state"`
	StopCode              string                                    `json:"stopCode"`
	SegmentMaxSteps       int                                       `json:"segmentMaxSteps"`
	AppliedStepsInSegment int                                       `json:"appliedStepsInSegment"`
	RemainingMaxSteps     int                                       `json:"remainingMaxSteps"`
	SegmentRoute          string                                    `json:"segmentRoute"`
	SegmentLane           string                                    `json:"segmentLane"`
	ExpectedRoute         string                                    `json:"expectedRoute"`
	ExpectedLane          string                                    `json:"expectedLane"`
	WhatIfCommand         string                                    `json:"whatIfCommand"`
	ObservationContract   *mission.CurrentLoopObservationContract   `json:"observationContract,omitempty"`
	ExternalMemberHandoff *mission.CurrentLoopExternalMemberHandoff `json:"externalMemberHandoff,omitempty"`
	FreshPreviewRequired  bool                                      `json:"freshPreviewRequired"`
	CumulativeReceipts    bool                                      `json:"cumulativeReceipts"`
	Boundary              []string                                  `json:"boundary"`
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
	ResumeSourceArtifactSHA256    string                                `json:"resumeSourceArtifactSha256,omitempty"`
	ObservationPath               string                                `json:"observationPath,omitempty"`
	ObservationSHA256             string                                `json:"observationSha256,omitempty"`
}

func runCurrentLoop(ctx runtime.Context, opt Options, out io.Writer) error {
	if !ctx.TargetProvided {
		return fmt.Errorf("run-current-loop requires -Target for an attached case")
	}
	externalModes := 0
	for _, selected := range []bool{opt.RecordExternalSessionAttempt, opt.ClaimExternalSessionDispatch, opt.RecordExternalSessionLaunch, opt.AdvanceExternalSessionResult, opt.RelayExternalSessionSubmission} {
		if selected {
			externalModes++
		}
	}
	if externalModes > 1 {
		return fmt.Errorf("run-current-loop external session modes are mutually exclusive")
	}
	if opt.RecordExternalSessionAttempt {
		return runCurrentLoopExternalSessionAttempt(ctx, opt, out)
	}
	if opt.ClaimExternalSessionDispatch || opt.RecordExternalSessionLaunch {
		return runCurrentLoopExternalSessionDispatch(ctx, opt, out)
	}
	if opt.AdvanceExternalSessionResult {
		return runCurrentLoopExternalSessionTurn(ctx, opt, out)
	}
	if opt.RelayExternalSessionSubmission {
		return runCurrentLoopExternalSessionRelay(ctx, opt, out)
	}
	if strings.TrimSpace(opt.ExpectedExternalSessionJobSHA256) != "" || strings.TrimSpace(opt.ExpectedExternalSessionSubmissionSHA256) != "" || strings.TrimSpace(opt.ExpectedExternalSessionRelayPlanSHA256) != "" || strings.TrimSpace(opt.ExpectedExternalSessionTurnPlanSHA256) != "" || strings.TrimSpace(opt.ExpectedExternalSessionAttemptSHA256) != "" || strings.TrimSpace(opt.ExpectedExternalSessionAttemptPlanSHA256) != "" || strings.TrimSpace(opt.ExpectedExternalSessionDispatchSHA256) != "" || strings.TrimSpace(opt.ExpectedExternalSessionClaimSHA256) != "" || strings.TrimSpace(opt.ExpectedExternalSessionDispatchPlanSHA256) != "" || strings.TrimSpace(opt.ExternalSessionHarness) != "" || strings.TrimSpace(opt.ExternalSessionID) != "" || strings.TrimSpace(opt.ExternalSessionActor) != "" || strings.TrimSpace(opt.ExternalSessionStartedAt) != "" || strings.TrimSpace(opt.ExternalSessionLaunchOutcome) != "" || strings.TrimSpace(opt.ExternalSessionObservedAt) != "" || strings.TrimSpace(opt.ExternalSessionLaunchReason) != "" {
		return fmt.Errorf("external session fields require a result, relay, or attempt mode")
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
	if !opt.ResumeCurrentLoop && (opt.MaxSteps < 1 || opt.MaxSteps > 20) {
		return fmt.Errorf("run-current-loop requires -MaxSteps between 1 and 20")
	}
	if opt.ResumeCurrentLoop && opt.MaxSteps != 0 {
		return fmt.Errorf("run-current-loop -ResumeCurrentLoop derives remaining budget and does not accept -MaxSteps")
	}
	if opt.ResumeCurrentLoop && strings.TrimSpace(opt.ExpectedCurrentLoopCheckpointSHA256) == "" {
		return fmt.Errorf("run-current-loop -ResumeCurrentLoop requires -ExpectedCurrentLoopCheckpointSha256 from status or handoff")
	}
	if strings.TrimSpace(opt.CurrentLoopObservationPath) != "" {
		if !opt.ResumeCurrentLoop {
			return fmt.Errorf("run-current-loop -CurrentLoopObservationPath requires -ResumeCurrentLoop")
		}
		if currentStepHasMemberObservation(opt) || currentStepHasReviewerObservation(opt) || strings.TrimSpace(opt.Start.Actor) != "" {
			return fmt.Errorf("run-current-loop observation envelope cannot be combined with legacy member/reviewer observation flags or -Actor")
		}
		if opt.WhatIf && strings.TrimSpace(opt.ExpectedCurrentLoopObservationSHA256) != "" {
			return fmt.Errorf("run-current-loop observation envelope -WhatIf does not accept -ExpectedCurrentLoopObservationSha256")
		}
		if opt.Apply && strings.TrimSpace(opt.ExpectedCurrentLoopObservationSHA256) == "" {
			return fmt.Errorf("run-current-loop observation envelope -Apply requires -ExpectedCurrentLoopObservationSha256 from -WhatIf")
		}
	} else if strings.TrimSpace(opt.ExpectedCurrentLoopObservationSHA256) != "" {
		return fmt.Errorf("run-current-loop -ExpectedCurrentLoopObservationSha256 requires -CurrentLoopObservationPath")
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
	if plan.InitialCurrentStep != nil && plan.InitialCurrentStep.MemberExecution != nil {
		expectedMember := strings.TrimSpace(opt.ExpectedMemberExecutionPlanSHA256)
		if expectedMember == "" {
			return fmt.Errorf("run-current-loop member execution -Apply requires -ExpectedMemberExecutionPlanSha256 from -WhatIf")
		}
		if !strings.EqualFold(expectedMember, plan.InitialCurrentStep.MemberExecution.ExpectedPlanSHA256) {
			return fmt.Errorf("run-current-loop expected member execution plan sha256 mismatch: got %s want %s", expectedMember, plan.InitialCurrentStep.MemberExecution.ExpectedPlanSHA256)
		}
	}
	expected := strings.TrimSpace(opt.ExpectedCurrentLoopPlanSHA256)
	if expected == "" {
		return fmt.Errorf("run-current-loop -Apply requires -ExpectedCurrentLoopPlanSha256 from -WhatIf")
	}
	if !strings.EqualFold(expected, plan.ExpectedCurrentLoopPlanSHA256) {
		return fmt.Errorf("run-current-loop expected plan sha256 mismatch: got %s want %s", expected, plan.ExpectedCurrentLoopPlanSHA256)
	}
	if plan.ResumeSource != nil && strings.TrimSpace(opt.CurrentLoopObservationPath) != "" {
		if err := applyCurrentLoopObservationEnvelope(ctx, &opt, *plan.ResumeSource); err != nil {
			return err
		}
	}
	if plan.ResumeSource != nil {
		requestSHA256, hashErr := currentloop.RequestSHA256(*plan.InitialCurrentDriverRequest)
		if hashErr != nil {
			return hashErr
		}
		if claimErr := currentloop.ClaimResume(ctx.RepoRoot, ctx.Target, ctx.Pack, currentloop.Claim{
			SourceArtifactSHA256:          plan.ExpectedResumeCheckpointSHA256,
			ExpectedCurrentLoopPlanSHA256: plan.ExpectedCurrentLoopPlanSHA256,
			CurrentDriverRequestSHA256:    requestSHA256,
			Actor:                         plan.Actor,
		}); claimErr != nil {
			return claimErr
		}
	}
	plan, err = applyBuiltCurrentLoop(ctx, opt, plan, status)
	if err != nil {
		return err
	}
	return writeJSON(out, plan)
}

func applyBuiltCurrentLoop(ctx runtime.Context, opt Options, plan currentLoopPlan, status statusInventory) (currentLoopPlan, error) {
	plan, err := applyCurrentLoopPlan(ctx, opt, plan, status)
	if err != nil {
		return plan, err
	}
	if plan.ResumeSource != nil && plan.AppliedSteps == 0 && plan.zeroProgressVerified {
		fresh, refreshErr := buildInvocationStatusInventory(ctx, opt)
		freshRequest, freshRoute, freshStop := currentLoopStatusCandidate(fresh)
		initialRequestSHA256, initialHashErr := currentloop.RequestSHA256(*plan.InitialCurrentDriverRequest)
		freshRequestSHA256 := ""
		if freshRequest != nil {
			freshRequestSHA256, _ = currentloop.RequestSHA256(*freshRequest)
		}
		if refreshErr == nil && initialHashErr == nil && freshStop.Code == "" && freshRoute == plan.InitialRoute && strings.TrimSpace(freshRequest.Lane) == plan.InitialLane && strings.EqualFold(freshRequestSHA256, initialRequestSHA256) {
			plan.FinalStatus = &fresh
			plan.StopReason.Code = "zero-progress-retry"
			plan.StopReason.Message = "resume claim succeeded, but the nested step failed before mutation: " + plan.StopReason.Message
			plan.ContinuationRequest = currentLoopContinuationFor(ctx, currentLoopSelectedLane(opt, plan.InitialLane), plan.MaxSteps, 0, plan.InitialRoute, plan.InitialLane, freshRoute, plan.Actor, plan.StopReason)
			if plan.ContinuationRequest != nil {
				plan.ResumeCommand = plan.ContinuationRequest.WhatIfCommand
			}
		} else {
			plan.Boundary = append(plan.Boundary,
				"the nested step failed before mutation, but refreshed status did not preserve the claimed route, lane, and exact current request; the claim remains consumed and no retry checkpoint is published",
			)
		}
	}
	if plan.AppliedSteps > 0 || plan.StopReason.Code == "zero-progress-retry" {
		inspection, checkpointErr := writeCurrentLoopSegmentCheckpoint(ctx, opt, plan)
		plan.SegmentCheckpoint = &inspection
		if checkpointErr == nil && plan.ObservationSHA256 != "" {
			plan.ObservationReceipt = &currentLoopObservationReceipt{
				State:                     "processed",
				SourceCheckpointSHA256:    plan.ExpectedResumeCheckpointSHA256,
				SuccessorCheckpointSHA256: inspection.ArtifactSHA256,
				ObservationPath:           plan.ObservationPath,
				ObservationSHA256:         plan.ObservationSHA256,
				ObservationKind:           plan.ObservationKind,
				Actor:                     plan.ObservationActor,
				Boundary: []string{
					"receipt is proven by the strict successor checkpoint that binds the exact observation bytes and consumed source checkpoint",
					"the observation file is not deleted or mutated; replay is blocked by the one-shot source checkpoint claim",
				},
			}
		}
		if checkpointErr != nil {
			plan.Boundary = append(plan.Boundary,
				"durable current-loop segment checkpoint publication failed; this invocation result remains the only receipt and status must not recover its budget",
			)
		}
	}
	return plan, nil
}

func buildCurrentLoopPlan(ctx runtime.Context, opt Options) (currentLoopPlan, statusInventory, error) {
	status, err := buildInvocationStatusInventory(ctx, opt)
	if err != nil {
		return currentLoopPlan{}, statusInventory{}, err
	}
	if err := validateCurrentLoopReviewerAttemptObservation(opt, status); err != nil {
		return currentLoopPlan{}, statusInventory{}, err
	}
	var resumeSource *currentloop.Inspection
	if opt.ResumeCurrentLoop {
		if status.MissionControlRunbook == nil || status.MissionControlRunbook.CurrentLoopSegment == nil {
			return currentLoopPlan{}, statusInventory{}, fmt.Errorf("run-current-loop -ResumeCurrentLoop requires a durable current-loop checkpoint")
		}
		inspection := status.MissionControlRunbook.CurrentLoopSegment
		if !inspection.Ready || inspection.State != "ready" || inspection.Continuation == nil || inspection.RemainingMaxSteps < 1 || inspection.ArtifactSHA256 == "" {
			return currentLoopPlan{}, statusInventory{}, fmt.Errorf("run-current-loop -ResumeCurrentLoop requires the latest checkpoint to be state=ready")
		}
		if err := applyCurrentLoopObservationEnvelope(ctx, &opt, *inspection); err != nil {
			return currentLoopPlan{}, statusInventory{}, err
		}
		if currentStepUsesCheckpointSourceRequest(opt, status) {
			request := *status.MissionControlRunbook.CurrentLoopOperator.SourceCurrentDriverRequest
			runbook := *status.MissionControlRunbook
			runbook.CurrentDriverRequest = &request
			runbook.CurrentRunLoopStepID = request.RunLoopStepID
			runbook.CurrentCommand = request.Command
			status.MissionControlRunbook = &runbook
		}
		if err := validateCurrentLoopReviewerAttemptObservation(opt, status); err != nil {
			return currentLoopPlan{}, statusInventory{}, err
		}
		if inspection.StopCode == "external-member-handoff" {
			if !currentStepHasMemberObservation(opt) {
				return currentLoopPlan{}, statusInventory{}, fmt.Errorf("run-current-loop external member checkpoint resume requires one member observation from continuation.observationContract")
			}
			if err := validateCurrentLoopMemberCheckpointObservation(ctx, opt, *inspection); err != nil {
				return currentLoopPlan{}, statusInventory{}, err
			}
		}
		if err := validateCurrentLoopResumeStatus(status, *inspection); err != nil {
			return currentLoopPlan{}, statusInventory{}, err
		}
		if strings.TrimSpace(opt.ExpectedCurrentLoopCheckpointSHA256) != "" && !strings.EqualFold(strings.TrimSpace(opt.ExpectedCurrentLoopCheckpointSHA256), inspection.ArtifactSHA256) {
			return currentLoopPlan{}, statusInventory{}, fmt.Errorf("run-current-loop expected checkpoint sha256 mismatch: got %s want %s", strings.TrimSpace(opt.ExpectedCurrentLoopCheckpointSHA256), inspection.ArtifactSHA256)
		}
		opt.MaxSteps = inspection.RemainingMaxSteps
		copy := *inspection
		resumeSource = &copy
	}
	plan := currentLoopPlan{
		SchemaVersion:        1,
		Command:              commands.RunCurrentLoop,
		CaseRoot:             ctx.Target,
		Pack:                 ctx.Pack,
		Actor:                strings.TrimSpace(opt.Start.Actor),
		RoutePolicy:          currentLoopRoutePolicy,
		MaxSteps:             opt.MaxSteps,
		ResumeSource:         resumeSource,
		ReviewRequired:       true,
		RequiresConfirmation: true,
		Steps:                []currentLoopStepReceipt{},
		ResumeCommand:        currentLoopResumeCommand(ctx, opt.MaxSteps, opt.SelectedCurrentLane),
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
	effectiveOpt, err := optionsWithEffectiveSelectedCurrentLane(opt, plan.InitialLane)
	if err != nil {
		return currentLoopPlan{}, statusInventory{}, err
	}
	if strings.TrimSpace(opt.SelectedCurrentLane) == "" {
		status, err = buildInvocationStatusInventory(ctx, effectiveOpt)
		if err != nil {
			return currentLoopPlan{}, statusInventory{}, err
		}
		request, route, stop = currentLoopStatusCandidate(status)
		if stop.Code != "" {
			plan.StopReason = stop
			plan.FinalStatus = &status
			return plan, status, nil
		}
		if route != plan.InitialRoute || strings.TrimSpace(request.Lane) != plan.InitialLane {
			return currentLoopPlan{}, statusInventory{}, fmt.Errorf("run-current-loop selected lane projection changed the initial route")
		}
	}
	plan.ResumeCommand = currentLoopResumeCommand(ctx, opt.MaxSteps, currentLoopSelectedLane(effectiveOpt, plan.InitialLane))
	plan.InitialCurrentDriverRequest = request
	step, err := buildCurrentStepPlanFromStatus(ctx, effectiveOpt, status)
	if err != nil {
		plan.StopReason = currentLoopStopReason{Code: "requires-review", Phase: "preview", Message: err.Error(), CurrentDriverRequest: request}
		plan.FinalStatus = &status
		return plan, status, nil
	}
	plan.InitialCurrentStep = &step
	if step.ExpectedCurrentStepPlanSHA256 == "" {
		plan.StopReason = currentLoopExternalStop(step, request, status)
		plan.ContinuationRequest = currentLoopContinuationFor(ctx, currentLoopSelectedLane(opt, plan.InitialLane), opt.MaxSteps, 0, route, plan.InitialLane, route, plan.Actor, plan.StopReason)
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
	if resumeSource != nil {
		identity.ResumeSourceArtifactSHA256 = resumeSource.ArtifactSHA256
		identity.ObservationPath = opt.CurrentLoopObservationPath
		identity.ObservationSHA256 = opt.ExpectedCurrentLoopObservationSHA256
	}
	encoded, err := json.Marshal(identity)
	if err != nil {
		return currentLoopPlan{}, statusInventory{}, err
	}
	sum := sha256.Sum256(encoded)
	plan.ExpectedCurrentLoopPlanSHA256 = hex.EncodeToString(sum[:])
	if resumeSource != nil {
		plan.ExpectedResumeCheckpointSHA256 = resumeSource.ArtifactSHA256
		plan.ObservationPath = opt.CurrentLoopObservationPath
		plan.ObservationSHA256 = opt.ExpectedCurrentLoopObservationSHA256
		if plan.ObservationPath != "" {
			plan.ObservationKind = currentLoopObservationKind(opt)
			plan.ObservationActor = strings.TrimSpace(opt.Start.Actor)
		}
		plan.ApplyCommand = currentLoopResumeApplyCommand(ctx, plan, opt)
		plan.Boundary = append(plan.Boundary,
			"resume preview binds the latest ready checkpoint artifact and its remaining budget; Apply revalidates that exact checkpoint before executing the new segment",
		)
		if plan.ObservationSHA256 != "" {
			plan.Boundary = append(plan.Boundary,
				"observationPath and observationSha256 bind the exact strict case-local external session observation envelope; Apply rereads the same bytes before mutation",
			)
		}
	}
	plan.StopReason = currentLoopStopReason{Code: "ready", Phase: "preview", Message: "bounded current loop is ready for hash-bound Apply", CurrentDriverRequest: request}
	return plan, status, nil
}

func applyCurrentLoopPlan(ctx runtime.Context, opt Options, plan currentLoopPlan, status statusInventory) (currentLoopPlan, error) {
	initialRoute := plan.InitialRoute
	initialLane := plan.InitialLane
	effectiveOpt, err := optionsWithEffectiveSelectedCurrentLane(opt, initialLane)
	if err != nil {
		return currentLoopPlan{}, err
	}
	seen := map[string]bool{}
	stepOpt := effectiveOpt
	for stepNumber := 1; stepNumber <= plan.MaxSteps; stepNumber++ {
		request, route, stop := currentLoopStatusCandidate(status)
		if stop.Code != "" {
			plan.StopReason = stop
			break
		}
		if route != initialRoute || strings.TrimSpace(request.Lane) != initialLane {
			plan.StopReason = currentLoopStopReason{Code: "route-policy", Phase: "before-step", Message: "refreshed route or lane changed; review a fresh loop preview", CurrentDriverRequest: request}
			plan.ContinuationRequest = currentLoopContinuationFor(ctx, currentLoopSelectedLane(opt, plan.InitialLane), plan.MaxSteps, plan.AppliedSteps, initialRoute, initialLane, route, plan.Actor, plan.StopReason)
			if plan.ContinuationRequest != nil {
				plan.ResumeCommand = plan.ContinuationRequest.WhatIfCommand
			}
			break
		}
		if policyStop := currentLoopBeforeStepPolicyStop(stepNumber, route, request); policyStop.Code != "" {
			plan.StopReason = policyStop
			plan.ContinuationRequest = currentLoopContinuationFor(ctx, currentLoopSelectedLane(opt, plan.InitialLane), plan.MaxSteps, plan.AppliedSteps, initialRoute, initialLane, route, plan.Actor, plan.StopReason)
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
			plan.StopReason = currentLoopExternalStop(step, request, status)
			plan.ContinuationRequest = currentLoopContinuationFor(ctx, currentLoopSelectedLane(opt, plan.InitialLane), plan.MaxSteps, plan.AppliedSteps, initialRoute, initialLane, route, plan.Actor, plan.StopReason)
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
			if currentStepErrorIsZeroProgress(err) {
				plan.zeroProgressVerified = true
			}
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
		if (applied.Applied || applied.memberExecutionAlreadyApplied) && applied.Receipt != nil && applied.MemberExecution != nil {
			receipt := currentLoopStepReceipt{Step: stepNumber, Route: route, Lane: strings.TrimSpace(request.Lane), RunLoopStepID: strings.TrimSpace(request.RunLoopStepID), ExpectedCurrentStepPlanSHA256: step.ExpectedCurrentStepPlanSHA256, RequestBefore: *request, CurrentStepReceipt: applied.Receipt}
			if applied.RefreshedStatus == nil {
				receipt.CurrentStepReceipt.State = "refresh-failed"
				receipt.CurrentStepReceipt.Outcome = "current-step-applied-status-refresh-failed"
				plan.Steps = append(plan.Steps, receipt)
				plan.AppliedSteps++
				plan.Applied = true
				plan.FinalStatus = nil
				plan.StopReason = currentLoopRefreshErrorStop(stepNumber, request, fmt.Errorf("current-step member execution omitted refreshed status"))
				break
			}
			status = *applied.RefreshedStatus
			bindCurrentLoopReceiptStatus(&receipt, status)
			plan.Steps = append(plan.Steps, receipt)
			plan.AppliedSteps++
			plan.Applied = true
			plan.FinalStatus = &status
			memberState := applied.MemberExecution.Inspection.State
			if memberState == "intake-ready" {
				plan.StopReason = currentLoopStopReason{Code: "member-intake-ready", Phase: "after-step", Message: "member result passed strict intake; review a fresh segment before continuing deterministic work", CurrentDriverRequest: receipt.RequestAfter}
				plan.ContinuationRequest = currentLoopContinuationFor(ctx, currentLoopSelectedLane(opt, plan.InitialLane), plan.MaxSteps, plan.AppliedSteps, initialRoute, initialLane, route, plan.Actor, plan.StopReason)
				if plan.ContinuationRequest != nil {
					plan.ResumeCommand = plan.ContinuationRequest.WhatIfCommand
				}
				break
			}
			plan.StopReason = currentLoopStopReason{Code: "external-member-handoff", Phase: "after-step", Message: "durable member execution handoff or observation was recorded; refresh status before continuing", CurrentDriverRequest: receipt.RequestAfter}
			if memberState == "handoff-ready" || memberState == "accepted" {
				plan.StopReason.ExternalMemberHandoff = currentLoopExternalMemberHandoff(ctx, applied.MemberExecution.Inspection, nil)
			}
			plan.ContinuationRequest = currentLoopContinuationFor(ctx, currentLoopSelectedLane(opt, plan.InitialLane), plan.MaxSteps, plan.AppliedSteps, initialRoute, initialLane, route, plan.Actor, plan.StopReason)
			if plan.ContinuationRequest != nil {
				plan.ResumeCommand = plan.ContinuationRequest.WhatIfCommand
			}
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
		bindCurrentLoopReceiptStatus(&receipt, status)
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

func currentLoopSelectedLane(opt Options, reviewedLane string) string {
	if selected := strings.TrimSpace(opt.SelectedCurrentLane); selected != "" {
		return selected
	}
	return strings.TrimSpace(reviewedLane)
}

func bindCurrentLoopReceiptStatus(
	receipt *currentLoopStepReceipt,
	status statusInventory,
) {
	if receipt == nil || receipt.CurrentStepReceipt == nil {
		return
	}
	request, _, _ := currentLoopStatusCandidate(status)
	receipt.RequestAfter = request
	receipt.CurrentStepReceipt.RefreshedCurrentDriverRequest = request
}

func validateCurrentLoopReviewerAttemptObservation(opt Options, status statusInventory) error {
	if currentStepHasMemberObservation(opt) && (strings.TrimSpace(opt.ReviewerResultInputSourcePath) != "" || strings.TrimSpace(opt.ReviewerHarness) != "" || strings.TrimSpace(opt.ReviewerSession) != "" || strings.TrimSpace(opt.ReviewerOutcome) != "" || strings.TrimSpace(opt.ReviewerExitStatus) != "") {
		return fmt.Errorf("run-current-loop cannot combine member and reviewer observations")
	}
	hasObservation := strings.TrimSpace(opt.ReviewerResultInputSourcePath) != "" ||
		strings.TrimSpace(opt.ReviewerHarness) != "" ||
		strings.TrimSpace(opt.ReviewerSession) != "" ||
		strings.TrimSpace(opt.ReviewerOutcome) != "" ||
		strings.TrimSpace(opt.ReviewerExitStatus) != ""
	expected := strings.TrimSpace(opt.ExpectedCurrentLoopReviewerAttemptSHA256)
	if !hasObservation {
		if expected != "" {
			return fmt.Errorf("run-current-loop -ExpectedCurrentLoopReviewerAttemptSha256 requires a reviewer observation")
		}
		return nil
	}
	if expected == "" {
		return fmt.Errorf("run-current-loop reviewer observations require -ExpectedCurrentLoopReviewerAttemptSha256 from status or handoff")
	}
	if status.MissionControlRunbook == nil || status.MissionControlRunbook.CurrentLoopOperator == nil ||
		status.MissionControlRunbook.CurrentLoopOperator.ExternalReviewerHandoff == nil ||
		status.MissionControlRunbook.CurrentLoopOperator.ExternalReviewerHandoff.Attempt == nil {
		return fmt.Errorf("run-current-loop reviewer observation requires a fresh current-loop reviewer attempt")
	}
	attempt := status.MissionControlRunbook.CurrentLoopOperator.ExternalReviewerHandoff.Attempt
	if !strings.EqualFold(expected, strings.TrimSpace(attempt.AttemptSnapshotSHA256)) {
		return fmt.Errorf("run-current-loop expected reviewer attempt sha256 mismatch: got %s want %s", expected, attempt.AttemptSnapshotSHA256)
	}
	return nil
}

func validateCurrentLoopResumeStatus(status statusInventory, inspection currentloop.Inspection) error {
	request, route, stop := currentLoopStatusCandidate(status)
	lane := ""
	if request != nil {
		lane = strings.TrimSpace(request.Lane)
	}
	if stop.Code != "" || request == nil || route != inspection.ExpectedRoute || lane != inspection.ExpectedLane {
		return fmt.Errorf("run-current-loop ready checkpoint expected route or lane does not match refreshed status: scope=%q expectedRoute=%q lane=%q expectedLane=%q", route, inspection.ExpectedRoute, lane, inspection.ExpectedLane)
	}
	return nil
}

func currentLoopStatusCandidate(status statusInventory) (*mission.MissionCommanderDriverRequest, string, currentLoopStopReason) {
	if status.CaseMission != nil && status.CaseMission.MissionCompletion != nil && status.CaseMission.MissionCompletion.Ready && status.CaseMission.MissionCompletion.State == "mission-complete" {
		return nil, "case", currentLoopStopReason{Code: "mission-complete", Phase: "status", Message: "all durable lanes have committed completion receipts"}
	}
	if status.MissionControlRunbook == nil || status.MissionControlRunbook.CurrentDriverRequest == nil {
		return nil, "", currentLoopStopReason{Code: "no-current-request", Phase: "status", Message: "refreshed status has no current driver request"}
	}
	request := status.MissionControlRunbook.CurrentDriverRequest
	route := strings.TrimSpace(status.MissionControlRunbook.Scope)
	if route == "pack-memory" {
		if consumerRequest, err := currentStepPackMemoryConsumerRequest(status.TemplateRoot, status.Target, status.Pack, status); err == nil {
			request = consumerRequest
			route = "case"
		}
	}
	if route != "case" && route != "reviewer" {
		return request, route, currentLoopStopReason{Code: "route-policy", Phase: "status", Message: fmt.Sprintf("focused scope %q is outside the bounded case/reviewer loop", route), CurrentDriverRequest: request}
	}
	if request.Blocked {
		return request, route, currentLoopStopReason{Code: "blocked", Phase: "status", Message: "current driver request is blocked", CurrentDriverRequest: request}
	}
	if route != "reviewer" && (!request.CommandExecutable || request.Invocation == nil || request.Invocation.Validate() != nil) {
		return request, route, currentLoopStopReason{Code: "guidance-only", Phase: "status", Message: "current driver request requires main-agent guidance instead of command execution", CurrentDriverRequest: request}
	}
	return request, route, currentLoopStopReason{}
}

func currentLoopBeforeStepPolicyStop(stepNumber int, route string, request *mission.MissionCommanderDriverRequest) currentLoopStopReason {
	if stepNumber > 1 && route == "case" && request != nil && request.Invocation != nil && request.Invocation.Validate() == nil && request.Invocation.Command == commands.Reconcile {
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

func currentLoopExternalStop(step currentStepPlan, request *mission.MissionCommanderDriverRequest, status statusInventory) currentLoopStopReason {
	stop := currentLoopStopReason{Code: "requires-review", Phase: "before-step", Message: "current step has no deterministic Apply hash", CurrentDriverRequest: request}
	if step.ReviewerStep != nil && step.ReviewerStep.ExternalHandoff != nil {
		stop.Code = "external-reviewer-handoff"
		stop.Message = "reviewer lifecycle requires an external harness action before the loop can resume"
		stop.ExternalHandoff = step.ReviewerStep.ExternalHandoff
		if status.MissionControlRunbook != nil && status.MissionControlRunbook.CurrentLoopOperator != nil &&
			status.MissionControlRunbook.CurrentLoopOperator.ExternalReviewerHandoff != nil &&
			status.MissionControlRunbook.CurrentLoopOperator.ExternalReviewerHandoff.Attempt != nil {
			stop.ExpectedReviewerAttemptSHA256 = status.MissionControlRunbook.CurrentLoopOperator.ExternalReviewerHandoff.Attempt.AttemptSnapshotSHA256
		}
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

func currentLoopResumeCommand(ctx runtime.Context, maxSteps int, selected ...string) string {
	command := fmt.Sprintf("/rekit run-current-loop -Target %s -Pack %s -MaxSteps %d -WhatIf -Format json", statusQuoteCommandArg(ctx.Target), statusQuoteCommandArg(ctx.Pack), maxSteps)
	if len(selected) > 0 {
		command = selectedLaneCommand(command, selected[0])
	}
	return command
}

func currentLoopResumeApplyCommand(ctx runtime.Context, plan currentLoopPlan, opt Options) string {
	args := []string{
		"/rekit", "run-current-loop",
		"-Target", statusQuoteCommandArg(ctx.Target),
		"-Pack", statusQuoteCommandArg(ctx.Pack),
		"-ResumeCurrentLoop",
		"-ExpectedCurrentLoopCheckpointSha256", plan.ExpectedResumeCheckpointSHA256,
	}
	appendValue := func(flag, value string) {
		if strings.TrimSpace(value) != "" {
			args = append(args, flag, statusQuoteCommandArg(value))
		}
	}
	appendValue("-Lane", opt.SelectedCurrentLane)
	if strings.TrimSpace(opt.CurrentLoopObservationPath) != "" {
		appendValue("-CurrentLoopObservationPath", opt.CurrentLoopObservationPath)
		appendValue("-ExpectedCurrentLoopObservationSha256", opt.ExpectedCurrentLoopObservationSHA256)
	} else {
		appendValue("-Actor", opt.Start.Actor)
		appendValue("-MemberExecutionAttemptId", opt.MemberExecutionAttemptID)
		appendValue("-MemberExecutionOutcome", opt.MemberExecutionOutcome)
		appendValue("-MemberExecutionReason", opt.MemberExecutionReason)
		appendValue("-MemberExecutionObservedAt", opt.MemberExecutionObservedAt)
		appendValue("-ExpectedCurrentLoopReviewerAttemptSha256", opt.ExpectedCurrentLoopReviewerAttemptSHA256)
		appendValue("-ReviewerResultInputSourcePath", opt.ReviewerResultInputSourcePath)
		appendValue("-ReviewerHarness", opt.ReviewerHarness)
		appendValue("-ReviewerSession", opt.ReviewerSession)
		appendValue("-ReviewerOutcome", opt.ReviewerOutcome)
		appendValue("-ReviewerExitStatus", opt.ReviewerExitStatus)
	}
	if plan.InitialCurrentStep != nil && plan.InitialCurrentStep.MemberExecution != nil {
		appendValue("-ExpectedMemberExecutionPlanSha256", plan.InitialCurrentStep.MemberExecution.ExpectedPlanSHA256)
	}
	args = append(args, "-ExpectedCurrentLoopPlanSha256", plan.ExpectedCurrentLoopPlanSHA256, "-Apply", "-Format", "json")
	return strings.Join(args, " ")
}

func validateCurrentLoopMemberCheckpointObservation(ctx runtime.Context, opt Options, inspection currentloop.Inspection) error {
	if inspection.Continuation == nil || inspection.Continuation.ExternalMemberHandoff == nil || inspection.Continuation.ObservationContract == nil {
		return fmt.Errorf("run-current-loop external member checkpoint omitted its durable observation contract")
	}
	handoff := inspection.Continuation.ExternalMemberHandoff
	attemptID := strings.TrimSpace(opt.MemberExecutionAttemptID)
	outcome := strings.ToLower(strings.TrimSpace(opt.MemberExecutionOutcome))
	if attemptID != handoff.AttemptID {
		return fmt.Errorf("run-current-loop member observation attempt %q does not match checkpoint attempt %q", attemptID, handoff.AttemptID)
	}
	allowed := false
	for _, alternative := range inspection.Continuation.ObservationContract.Alternatives {
		if alternative.Kind == "member-session-"+outcome {
			allowed = true
			break
		}
	}
	if !allowed {
		return fmt.Errorf("run-current-loop member observation outcome %q is not allowed by checkpoint continuation", outcome)
	}
	current, err := memberexecution.Inspect(ctx.Target, handoff.Lane, handoff.AttemptID)
	if err != nil {
		return fmt.Errorf("inspect current-loop member checkpoint attempt: %w", err)
	}
	if current.Owner.Lane != handoff.Lane || current.Owner.Executor != handoff.Executor || current.Owner.ExecutorGeneration != handoff.ExecutorGeneration || current.State != handoff.State {
		return fmt.Errorf("run-current-loop member checkpoint attempt owner or state changed; refresh status instead of consuming prior budget")
	}
	return nil
}

func currentLoopExternalMemberHandoff(ctx runtime.Context, inspection memberexecution.Inspection, selected *mission.MissionCommanderDriverRequest) *mission.CurrentLoopExternalMemberHandoff {
	if inspection.Handoff == nil || inspection.AttemptID == "" || (inspection.State != "handoff-ready" && inspection.State != "accepted") {
		return nil
	}
	previewCommand := currentLoopResumeCommand(ctx, 1, inspection.Owner.Lane)
	if selected != nil && strings.TrimSpace(selected.Command) != "" {
		previewCommand = strings.TrimSpace(selected.Command)
	}
	observation := mission.CurrentLoopObservationContract{
		Boundary: []string{
			"choose exactly one member observation matching the durable attempt and current owner generation",
			"accepted records external harness acceptance; returned requires the strict manifest and every declared output; failed is terminal",
		},
	}
	for _, outcome := range []string{"accepted", "returned", "failed"} {
		if inspection.State == "accepted" && outcome == "accepted" {
			continue
		}
		alternative := mission.CurrentLoopObservationAlternative{
			Kind:                   "member-session-" + outcome,
			RequiredFlags:          []string{"-MemberExecutionAttemptId", "-MemberExecutionOutcome", "-MemberExecutionObservedAt", "-Actor"},
			PreviewCommandTemplate: currentLoopMemberObservationPreviewCommand(previewCommand, inspection.AttemptID, outcome),
			Transition:             inspection.State + "-to-" + outcome,
			Constraints: []string{
				"attemptId must equal " + inspection.AttemptID,
				"owner executor/generation must remain " + inspection.Owner.Executor + "/" + fmt.Sprint(inspection.Owner.ExecutorGeneration),
			},
		}
		if outcome == "returned" {
			alternative.Constraints = append(alternative.Constraints, "write strict manifest at "+inspection.Handoff.ManifestPath+" and bounded outputs below "+inspection.Handoff.OutputsRoot+" before preview")
		}
		observation.Alternatives = append(observation.Alternatives, alternative)
	}
	handoffPath := filepath.ToSlash(filepath.Join(".rekit", "lanes", inspection.Owner.Lane, "member-executions", inspection.AttemptID, "handoff.json"))
	return &mission.CurrentLoopExternalMemberHandoff{
		State:               inspection.State,
		AttemptID:           inspection.AttemptID,
		Lane:                inspection.Owner.Lane,
		Executor:            inspection.Owner.Executor,
		ExecutorGeneration:  inspection.Owner.ExecutorGeneration,
		HandoffPath:         handoffPath,
		HandoffSHA256:       inspection.HandoffSHA256,
		TaskContextPath:     inspection.Handoff.TaskContextPath,
		TaskContextSHA256:   inspection.Handoff.TaskContextSHA256,
		ManifestPath:        inspection.Handoff.ManifestPath,
		OutputsRoot:         inspection.Handoff.OutputsRoot,
		NextSteps:           append([]string{}, inspection.Handoff.NextSteps...),
		ObservationContract: observation,
		Boundary: mission.UniqueStrings(append([]string{
			"external harness owns the member session lifecycle; Go does not spawn, poll, or stop it",
			"handoff and observation commands do not grant authority or confirmed state and do not execute heavy tools",
		}, inspection.Handoff.Boundary...)),
	}
}

func currentLoopMemberObservationPreviewCommand(command, attemptID, outcome string) string {
	fields, err := splitDriverCommand(command)
	if err != nil || len(fields) < 2 {
		return ""
	}
	insertAt := len(fields)
	for idx, field := range fields {
		if strings.EqualFold(field, "-WhatIf") {
			insertAt = idx
			break
		}
	}
	insert := []string{"-MemberExecutionAttemptId", attemptID, "-MemberExecutionOutcome", outcome, "-MemberExecutionObservedAt", "<RFC3339Nano>", "-Actor", "<harness>"}
	fields = append(fields[:insertAt], append(insert, fields[insertAt:]...)...)
	return joinDriverCommand(fields)
}

func currentLoopContinuationFor(ctx runtime.Context, selectedLane string, segmentMaxSteps, appliedSteps int, segmentRoute, segmentLane, expectedRoute, actor string, stop currentLoopStopReason) *currentLoopContinuationRequest {
	if stop.Code != "external-reviewer-handoff" && stop.Code != "external-member-handoff" && stop.Code != "member-intake-ready" && stop.Code != "route-policy" && stop.Code != "human-intervention" && stop.Code != "zero-progress-retry" {
		return nil
	}
	if stop.Code == "external-reviewer-handoff" && stop.ExternalHandoff == nil {
		return nil
	}
	if stop.Code == "external-member-handoff" && stop.ExternalMemberHandoff == nil {
		return nil
	}
	remaining := segmentMaxSteps - appliedSteps
	if remaining < 1 || stop.CurrentDriverRequest == nil {
		return nil
	}
	whatIfCommand := currentLoopResumeCommand(ctx, remaining, selectedLane)
	if strings.TrimSpace(actor) != "" {
		whatIfCommand = strings.Replace(whatIfCommand, " -WhatIf", " -Actor "+statusQuoteCommandArg(actor)+" -WhatIf", 1)
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
		WhatIfCommand:         whatIfCommand,
		FreshPreviewRequired:  true,
		CumulativeReceipts:    false,
		Boundary: []string{
			"the continuation starts a fresh hash-bound loop segment from refreshed durable status; it does not carry the previous segment plan hash across the boundary",
			"the continuation retains only the remaining deterministic step budget; receipts stay in the previous segment result and are not accumulated across invocations",
			"the expected route and lane describe the reviewed transition target and must be revalidated by the fresh preview",
			"without this typed result, status may start a fresh loop preview but cannot claim recovery of the previous segment budget",
			"the continuation request is an orchestration handoff, not authority or a durable authorization token",
		},
	}
	if stop.Code == "external-reviewer-handoff" && stop.ExternalHandoff != nil {
		observation := mission.CurrentLoopObservationContract{Boundary: append([]string{}, stop.ExternalHandoff.ObservationContract.Boundary...)}
		previewRequest := &mission.MissionCommanderDriverRequest{Command: continuation.WhatIfCommand}
		for _, source := range stop.ExternalHandoff.ObservationContract.Alternatives {
			alternative := mission.CurrentLoopObservationAlternative{Kind: source.Kind, RequiredFlags: append([]string{}, source.RequiredFlags...), Constraints: append([]string{}, source.Constraints...)}
			if strings.TrimSpace(stop.ExpectedReviewerAttemptSHA256) != "" && alternative.Kind != "reviewer-result-direct-write" {
				alternative.RequiredFlags = append([]string{"-ExpectedCurrentLoopReviewerAttemptSha256"}, alternative.RequiredFlags...)
			}
			alternative.PreviewCommandTemplate = statusCurrentLoopObservationPreviewCommand(previewRequest, alternative.Kind, stop.ExpectedReviewerAttemptSHA256)
			alternative.Transition = statusCurrentLoopReviewerAttemptTransition(alternative.Kind)
			if alternative.Kind == "reviewer-result-direct-write" {
				alternative.Transition = "external-write-then-refresh-status"
			}
			observation.Alternatives = append(observation.Alternatives, alternative)
		}
		continuation.State = "awaiting-external-observation"
		continuation.ObservationContract = &observation
		continuation.Boundary = append([]string{
			"consume exactly one observation alternative through its previewCommandTemplate; alternatives with different attempt-guard requirements do not share observation flags",
			"reviewer-result-direct-write performs the external write first and then uses its unguarded fresh-preview template; do not carry the predecessor attempt snapshot into the successor state",
		}, continuation.Boundary...)
	}
	if stop.Code == "external-member-handoff" && stop.ExternalMemberHandoff != nil {
		observation := stop.ExternalMemberHandoff.ObservationContract
		continuation.State = "awaiting-external-observation"
		continuation.ObservationContract = &observation
		continuation.ExternalMemberHandoff = stop.ExternalMemberHandoff
		continuation.Boundary = append([]string{
			"external member execution remains owned by the durable lane executor; use exactly one accepted, returned, or failed preview template",
			"returned observation is valid only after the external member writes the strict manifest and all declared bounded outputs",
		}, continuation.Boundary...)
	}
	if stop.Code == "human-intervention" {
		continuation.Boundary = append([]string{
			"review the refreshed Human-in-the-Lane intervention in the fresh segment before applying reconcile",
		}, continuation.Boundary...)
	}
	return continuation
}

func writeCurrentLoopSegmentCheckpoint(ctx runtime.Context, opt Options, plan currentLoopPlan) (currentloop.Inspection, error) {
	payload := currentloop.Payload{
		Actor:                         plan.Actor,
		RoutePolicy:                   plan.RoutePolicy,
		InitialCurrentDriverRequest:   *plan.InitialCurrentDriverRequest,
		ExpectedCurrentLoopPlanSHA256: plan.ExpectedCurrentLoopPlanSHA256,
		ResumeSourceArtifactSHA256:    plan.ExpectedResumeCheckpointSHA256,
		ObservationPath:               plan.ObservationPath,
		ObservationSHA256:             plan.ObservationSHA256,
		ObservationKind:               plan.ObservationKind,
		ObservationActor:              plan.ObservationActor,
		ZeroProgressRecovery:          plan.StopReason.Code == "zero-progress-retry",
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
	if payload.ZeroProgressRecovery && plan.InitialCurrentStep != nil {
		payload.ExpectedInitialCurrentStepSHA256 = plan.InitialCurrentStep.ExpectedCurrentStepPlanSHA256
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
	if plan.FinalStatus != nil {
		request, _, _ := currentLoopStatusCandidate(*plan.FinalStatus)
		payload.RefreshedCurrentDriverRequest = request
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
	validate := (func() error)(nil)
	if payload.ZeroProgressRecovery {
		validate = func() error {
			fresh, err := buildInvocationStatusInventory(ctx, opt)
			if err != nil {
				return err
			}
			request, route, stop := currentLoopStatusCandidate(fresh)
			if stop.Code != "" || request == nil || route != plan.InitialRoute || strings.TrimSpace(request.Lane) != plan.InitialLane {
				return fmt.Errorf("refreshed route or lane changed after zero-progress recovery review")
			}
			requestSHA256, err := currentloop.RequestSHA256(*request)
			if err != nil || !strings.EqualFold(requestSHA256, payload.InitialCurrentDriverRequestSHA256) {
				return fmt.Errorf("refreshed current driver request changed after zero-progress recovery review")
			}
			step, err := buildCurrentStepPlanFromStatus(ctx, currentLoopFollowupOptions(opt), fresh)
			if err != nil || plan.InitialCurrentStep == nil || !strings.EqualFold(step.ExpectedCurrentStepPlanSHA256, plan.InitialCurrentStep.ExpectedCurrentStepPlanSHA256) {
				return fmt.Errorf("refreshed current step changed after zero-progress recovery review")
			}
			return nil
		}
	}
	inspection, err := currentloop.WriteValidated(ctx.RepoRoot, ctx.Target, ctx.Pack, payload, validate)
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
		ExternalMemberHandoff: source.ExternalMemberHandoff,
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
				Kind:                   alternative.Kind,
				RequiredFlags:          append([]string{}, alternative.RequiredFlags...),
				PreviewCommandTemplate: alternative.PreviewCommandTemplate,
				Transition:             alternative.Transition,
				Constraints:            append([]string{}, alternative.Constraints...),
			})
		}
		continuation.ObservationContract = observation
	}
	return continuation
}

func currentLoopFollowupOptions(opt Options) Options {
	opt.skipMemberExecutionDispatch = true
	opt.ExpectedMemberExecutionPlanSHA256 = ""
	opt.MemberExecutionAttemptID = ""
	opt.MemberExecutionOutcome = ""
	opt.MemberExecutionReason = ""
	opt.MemberExecutionObservedAt = ""
	opt.ExpectedCurrentLoopReviewerAttemptSHA256 = ""
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
		"-lane": true, "--lane": true,
		"-format": true, "--format": true,
		"-maxsteps": true, "--max-steps": true,
		"-expectedcurrentloopplansha256": true, "--expected-current-loop-plan-sha256": true,
		"-expectedcurrentloopcheckpointsha256": true, "--expected-current-loop-checkpoint-sha256": true,
		"-expectedcurrentloopreviewerattemptsha256": true, "--expected-current-loop-reviewer-attempt-sha256": true,
		"-currentloopobservationpath": true, "--current-loop-observation-path": true,
		"-expectedcurrentloopobservationsha256": true, "--expected-current-loop-observation-sha256": true,
		"-expectedexternalsessionjobsha256": true, "--expected-external-session-job-sha256": true,
		"-externalsessionharness": true, "--external-session-harness": true,
		"-externalsessionid": true, "--external-session-id": true,
		"-externalsessionactor": true, "--external-session-actor": true,
		"-externalsessionstartedat": true, "--external-session-started-at": true,
		"-expectedexternalsessionattemptsha256": true, "--expected-external-session-attempt-sha256": true,
		"-expectedexternalsessionattemptplansha256": true, "--expected-external-session-attempt-plan-sha256": true,
		"-externalsessionlaunchoutcome": true, "--external-session-launch-outcome": true,
		"-externalsessionobservedat": true, "--external-session-observed-at": true,
		"-externalsessionlaunchreason": true, "--external-session-launch-reason": true,
		"-expectedexternalsessiondispatchsha256": true, "--expected-external-session-dispatch-sha256": true,
		"-expectedexternalsessionclaimsha256": true, "--expected-external-session-claim-sha256": true,
		"-expectedexternalsessiondispatchplansha256": true, "--expected-external-session-dispatch-plan-sha256": true,
		"-expectedexternalsessionsubmissionsha256": true, "--expected-external-session-submission-sha256": true,
		"-expectedexternalsessionrelayplansha256": true, "--expected-external-session-relay-plan-sha256": true,
		"-expectedexternalsessionturnplansha256": true, "--expected-external-session-turn-plan-sha256": true,
		"-actor": true, "--actor": true,
		"-expectedmemberexecutionplansha256": true, "--expected-member-execution-plan-sha256": true,
		"-memberexecutionattemptid": true, "--member-execution-attempt-id": true,
		"-memberexecutionoutcome": true, "--member-execution-outcome": true,
		"-memberexecutionreason": true, "--member-execution-reason": true,
		"-memberexecutionobservedat": true, "--member-execution-observed-at": true,
		"-reviewerresultinputsourcepath": true, "--reviewer-result-input-source-path": true,
		"-reviewerharness": true, "--reviewer-harness": true,
		"-reviewersession": true, "--reviewer-session": true,
		"-revieweroutcome": true, "--reviewer-outcome": true,
		"-reviewerexitstatus": true, "--reviewer-exit-status": true,
	}
	switchFlags := map[string]bool{
		"-whatif": true, "--what-if": true,
		"-apply": true, "--apply": true,
		"-resumecurrentloop": true, "--resume-current-loop": true,
		"-relayexternalsessionsubmission": true, "--relay-external-session-submission": true,
		"-advanceexternalsessionresult": true, "--advance-external-session-result": true,
		"-recordexternalsessionattempt": true, "--record-external-session-attempt": true,
		"-claimexternalsessiondispatch": true, "--claim-external-session-dispatch": true,
		"-recordexternalsessionlaunch": true, "--record-external-session-launch": true,
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
	case "--lane":
		return "-lane"
	case "--format":
		return "-format"
	case "--max-steps":
		return "-maxsteps"
	case "--expected-current-loop-plan-sha256":
		return "-expectedcurrentloopplansha256"
	case "--expected-current-loop-checkpoint-sha256":
		return "-expectedcurrentloopcheckpointsha256"
	case "--current-loop-observation-path":
		return "-currentloopobservationpath"
	case "--expected-current-loop-observation-sha256":
		return "-expectedcurrentloopobservationsha256"
	case "--resume-current-loop":
		return "-resumecurrentloop"
	case "--relay-external-session-submission":
		return "-relayexternalsessionsubmission"
	case "--advance-external-session-result":
		return "-advanceexternalsessionresult"
	case "--record-external-session-attempt":
		return "-recordexternalsessionattempt"
	case "--claim-external-session-dispatch":
		return "-claimexternalsessiondispatch"
	case "--record-external-session-launch":
		return "-recordexternalsessionlaunch"
	case "--external-session-harness":
		return "-externalsessionharness"
	case "--external-session-id":
		return "-externalsessionid"
	case "--external-session-actor":
		return "-externalsessionactor"
	case "--external-session-started-at":
		return "-externalsessionstartedat"
	case "--expected-external-session-attempt-sha256":
		return "-expectedexternalsessionattemptsha256"
	case "--expected-external-session-attempt-plan-sha256":
		return "-expectedexternalsessionattemptplansha256"
	case "--external-session-launch-outcome":
		return "-externalsessionlaunchoutcome"
	case "--external-session-observed-at":
		return "-externalsessionobservedat"
	case "--external-session-launch-reason":
		return "-externalsessionlaunchreason"
	case "--expected-external-session-dispatch-sha256":
		return "-expectedexternalsessiondispatchsha256"
	case "--expected-external-session-claim-sha256":
		return "-expectedexternalsessionclaimsha256"
	case "--expected-external-session-dispatch-plan-sha256":
		return "-expectedexternalsessiondispatchplansha256"
	case "--expected-external-session-job-sha256":
		return "-expectedexternalsessionjobsha256"
	case "--expected-external-session-submission-sha256":
		return "-expectedexternalsessionsubmissionsha256"
	case "--expected-external-session-relay-plan-sha256":
		return "-expectedexternalsessionrelayplansha256"
	case "--expected-external-session-turn-plan-sha256":
		return "-expectedexternalsessionturnplansha256"
	case "--what-if":
		return "-whatif"
	case "--apply":
		return "-apply"
	case "--actor":
		return "-actor"
	case "--expected-member-execution-plan-sha256":
		return "-expectedmemberexecutionplansha256"
	case "--member-execution-attempt-id":
		return "-memberexecutionattemptid"
	case "--member-execution-outcome":
		return "-memberexecutionoutcome"
	case "--member-execution-reason":
		return "-memberexecutionreason"
	case "--member-execution-observed-at":
		return "-memberexecutionobservedat"
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
