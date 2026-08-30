package cli

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"strings"

	"github.com/shuiyu486/re-context-kits/internal/rekit/commands"
	"github.com/shuiyu486/re-context-kits/internal/rekit/currentloop"
	"github.com/shuiyu486/re-context-kits/internal/rekit/executioncontrol"
	"github.com/shuiyu486/re-context-kits/internal/rekit/externalsession"
	"github.com/shuiyu486/re-context-kits/internal/rekit/lanemutation"
	"github.com/shuiyu486/re-context-kits/internal/rekit/memberexecution"
	"github.com/shuiyu486/re-context-kits/internal/rekit/mission"
	"github.com/shuiyu486/re-context-kits/internal/rekit/packmemoryconsumption"
	"github.com/shuiyu486/re-context-kits/internal/rekit/runtime"
)

type currentStepZeroProgressError struct {
	cause error
}

func (e currentStepZeroProgressError) Error() string {
	return e.cause.Error()
}

func (e currentStepZeroProgressError) Unwrap() error {
	return e.cause
}

func currentStepErrorIsZeroProgress(err error) bool {
	var zeroProgress currentStepZeroProgressError
	return errors.As(err, &zeroProgress)
}

type currentStepPlan struct {
	SchemaVersion                 int                                   `json:"schemaVersion"`
	Command                       string                                `json:"command"`
	CaseRoot                      string                                `json:"caseRoot"`
	Pack                          string                                `json:"pack"`
	Route                         string                                `json:"route"`
	IsMutation                    bool                                  `json:"isMutation"`
	Applied                       bool                                  `json:"applied"`
	ReviewRequired                bool                                  `json:"reviewRequired"`
	RequiresConfirmation          bool                                  `json:"requiresConfirmation"`
	CurrentDriverRequest          mission.MissionCommanderDriverRequest `json:"currentDriverRequest"`
	DriverStep                    *driverStepPlan                       `json:"driverStep,omitempty"`
	ReviewerStep                  *reviewerStepPlan                     `json:"reviewerStep,omitempty"`
	MemberExecution               *memberexecution.Plan                 `json:"memberExecution,omitempty"`
	PackMemoryConsumerLane        string                                `json:"packMemoryConsumerLane,omitempty"`
	ExternalSessionStep           *currentStepExternalSessionPlan       `json:"externalSessionStep,omitempty"`
	ExpectedCurrentStepPlanSHA256 string                                `json:"expectedCurrentStepPlanSha256,omitempty"`
	Receipt                       *currentStepReceipt                   `json:"receipt,omitempty"`
	RefreshedStatus               *statusInventory                      `json:"refreshedStatus,omitempty"`
	Boundary                      []string                              `json:"boundary"`
	memberExecutionAlreadyApplied bool
}

type currentStepReceipt struct {
	State                         string                                 `json:"state"`
	Outcome                       string                                 `json:"outcome"`
	Route                         string                                 `json:"route"`
	NestedCommand                 string                                 `json:"nestedCommand"`
	RefreshedCurrentDriverRequest *mission.MissionCommanderDriverRequest `json:"refreshedCurrentDriverRequest,omitempty"`
	Boundary                      []string                               `json:"boundary"`
}

type currentStepExternalSessionPlan struct {
	SchemaVersion        int                                               `json:"schemaVersion"`
	Mode                 string                                            `json:"mode"`
	State                string                                            `json:"state"`
	InputRequired        []string                                          `json:"inputRequired,omitempty"`
	Attempt              *externalsession.AttemptPlan                      `json:"attempt,omitempty"`
	Dispatch             *externalsession.DispatchPlan                     `json:"dispatch,omitempty"`
	Transport            *externalsession.TransportPlan                    `json:"transport,omitempty"`
	TransportReturn      *externalsession.TransportReturnPlan              `json:"transportReturn,omitempty"`
	Turn                 *currentLoopExternalSessionTurnPlan               `json:"turn,omitempty"`
	HarnessPackage       *mission.CurrentLoopExternalSessionHarnessPackage `json:"harnessPackage,omitempty"`
	ReturnRequest        *mission.MissionCommanderDriverRequest            `json:"returnRequest,omitempty"`
	ReplacementRequest   *mission.MissionCommanderDriverRequest            `json:"replacementRequest,omitempty"`
	RefreshStatusCommand string                                            `json:"refreshStatusCommand"`
	Boundary             []string                                          `json:"boundary"`
}

type currentStepPlanIdentity struct {
	Route                        string                                `json:"route"`
	RoutedDriverRequest          mission.MissionCommanderDriverRequest `json:"routedDriverRequest"`
	NestedDriverRequest          mission.MissionCommanderDriverRequest `json:"nestedDriverRequest"`
	ExpectedNestedStepPlanSHA256 string                                `json:"expectedNestedStepPlanSha256"`
	ExpectedMemberPlanSHA256     string                                `json:"expectedMemberPlanSha256,omitempty"`
	External                     *currentStepExternalApplyIdentity     `json:"external,omitempty"`
	ReplacementResultPublication *currentStepResultPublicationIdentity `json:"replacementResultPublication,omitempty"`
}

type currentStepResultPublicationIdentity struct {
	Lane       string                        `json:"lane"`
	Birth      executioncontrol.ResultBirth  `json:"birth"`
	Source     executioncontrol.ResultSource `json:"source"`
	Actor      string                        `json:"actor"`
	ObservedAt string                        `json:"observedAt"`
}

type currentStepExternalApplyIdentity struct {
	Mode             string `json:"mode"`
	NestedPlanSHA256 string `json:"nestedPlanSha256"`
	JobSHA256        string `json:"jobSha256"`
	AttemptSHA256    string `json:"attemptSha256,omitempty"`
	DispatchSHA256   string `json:"dispatchSha256,omitempty"`
	ClaimSHA256      string `json:"claimSha256,omitempty"`
	SubmissionSHA256 string `json:"submissionSha256,omitempty"`
	CheckpointSHA256 string `json:"checkpointSha256"`
}

var (
	currentStepBeforeStatusRefreshHook         func(string) error
	currentStepValidatePackMemoryConsumerTask  = packmemoryconsumption.ValidateCurrentConsumerTask
	currentStepWithPackMemoryConsumerTaskLease = packmemoryconsumption.WithCurrentConsumerTaskLease
)

func runCurrentStep(ctx runtime.Context, opt Options, out io.Writer) (retErr error) {
	if !ctx.TargetProvided {
		return fmt.Errorf("run-current-step requires -Target for an attached case")
	}
	if opt.WhatIf && opt.Apply {
		return fmt.Errorf("run-current-step cannot combine -WhatIf and -Apply")
	}
	if !opt.WhatIf && !opt.Apply {
		return fmt.Errorf("run-current-step requires -WhatIf or -Apply")
	}
	if strings.ToLower(strings.TrimSpace(opt.Format)) != "json" {
		return fmt.Errorf("run-current-step supports only -Format json")
	}
	if err := validateCurrentStepOuterArgs(opt); err != nil {
		return err
	}
	releasePublication, err := acquireReviewerResultPublicationLease(ctx, &opt)
	if err != nil {
		return err
	}
	if releasePublication != nil {
		defer func() { retErr = releasePublication(retErr) }()
	}
	releaseReplacement, err := acquireReplacementResultPublicationLease(ctx, &opt)
	if err != nil {
		return err
	}
	if releaseReplacement != nil {
		defer func() { retErr = releaseReplacement(retErr) }()
	}
	plan, err := buildCurrentStepPlan(ctx, opt)
	if err != nil && opt.Apply && !opt.WhatIf && strings.TrimSpace(opt.ExpectedCurrentStepPlanSHA256) != "" {
		recoveryOpt := opt
		recoveryOpt.currentLoopExecutionControlRecovery = true
		recoveryStatus, recoveryErr := buildControlBoundResultRecoveryStatusInventory(ctx, recoveryOpt)
		if recoveryErr == nil {
			recoveryPlan, buildErr := buildCurrentStepPlanFromStatus(ctx, recoveryOpt, recoveryStatus)
			if buildErr == nil && recoveryPlan.ExternalSessionStep != nil &&
				recoveryPlan.ExternalSessionStep.Mode == "result-turn" &&
				recoveryPlan.ExternalSessionStep.Turn != nil &&
				recoveryPlan.ExternalSessionStep.Turn.Relay.Submission.LaunchControl != nil {
				plan = recoveryPlan
				opt = recoveryOpt
				err = nil
			} else if buildErr != nil {
				recoveryErr = buildErr
			} else {
				recoveryErr = fmt.Errorf("recovered route is not one control-bound external result turn")
			}
		}
		if err != nil && recoveryErr != nil {
			return errors.Join(err, fmt.Errorf("control-bound result recovery: %w", recoveryErr))
		}
	}
	if err != nil {
		return err
	}
	if opt.WhatIf {
		if strings.TrimSpace(opt.ExpectedCurrentStepPlanSHA256) != "" {
			return fmt.Errorf("run-current-step -WhatIf does not accept -ExpectedCurrentStepPlanSha256")
		}
		diagnostics, err := buildCurrentStepDiagnosticsDTO(plan, ctx.Target)
		if err != nil {
			return err
		}
		return writeJSON(out, diagnostics)
	}
	if plan.MemberExecution != nil {
		expectedMember := strings.TrimSpace(opt.ExpectedMemberExecutionPlanSHA256)
		if expectedMember == "" {
			return fmt.Errorf("run-current-step member execution -Apply requires -ExpectedMemberExecutionPlanSha256 from -WhatIf")
		}
		if !strings.EqualFold(expectedMember, plan.MemberExecution.ExpectedPlanSHA256) {
			return fmt.Errorf("run-current-step expected member execution plan sha256 mismatch: got %s want %s", expectedMember, plan.MemberExecution.ExpectedPlanSHA256)
		}
	}
	if plan.ExpectedCurrentStepPlanSHA256 == "" {
		return fmt.Errorf("run-current-step current route requires an external harness action before Apply")
	}
	expected := strings.TrimSpace(opt.ExpectedCurrentStepPlanSHA256)
	if expected == "" {
		return fmt.Errorf("run-current-step -Apply requires -ExpectedCurrentStepPlanSha256 from -WhatIf")
	}
	if !strings.EqualFold(expected, plan.ExpectedCurrentStepPlanSHA256) {
		return fmt.Errorf("run-current-step expected plan sha256 mismatch: got %s want %s", expected, plan.ExpectedCurrentStepPlanSHA256)
	}
	plan, err = applyCurrentStepPlan(ctx, opt, plan)
	if err != nil {
		if plan.Applied {
			diagnostics, diagnosticsErr := buildCurrentStepDiagnosticsDTO(plan, ctx.Target)
			if diagnosticsErr != nil {
				return errors.Join(err, diagnosticsErr)
			}
			if writeErr := writeJSON(out, diagnostics); writeErr != nil {
				return errors.Join(err, writeErr)
			}
		}
		return err
	}
	diagnostics, err := buildCurrentStepDiagnosticsDTO(plan, ctx.Target)
	if err != nil {
		return err
	}
	return writeJSON(out, diagnostics)
}

func acquireReplacementResultPublicationLease(
	ctx runtime.Context,
	opt *Options,
) (func(error) error, error) {
	if opt == nil || opt.currentLoopReplacementResultPublication == nil || !opt.Apply {
		return nil, nil
	}
	if opt.currentLoopReplacementMutationLease != nil || opt.currentLoopReplacementResultChecked != nil {
		return nil, fmt.Errorf("replacement result publication runner already carries mutation ownership")
	}
	publication := *opt.currentLoopReplacementResultPublication
	lease, err := lanemutation.AcquireProject(ctx.Target)
	if err != nil {
		return nil, err
	}
	fail := func(cause error) (func(error) error, error) {
		return nil, errors.Join(cause, lease.Unlock())
	}
	if err := lease.ValidateProjectFor(ctx.Target); err != nil {
		return fail(err)
	}
	prepared, err := executioncontrol.PrepareResultWithProjectLease(ctx.Target, lease, publication)
	if err != nil {
		return fail(err)
	}
	if prepared.Held {
		return fail(&executioncontrol.ResultHeldError{Publication: prepared})
	}
	if prepared.Disposition != executioncontrol.ResultDispositionCurrent {
		return fail(fmt.Errorf("replacement result preparation returned unexpected disposition %q", prepared.Disposition))
	}
	checked := false
	opt.currentLoopReplacementMutationLease = lease
	opt.currentLoopReplacementResultChecked = &checked
	return func(runErr error) error {
		if runErr == nil && !checked {
			runErr = fmt.Errorf("replacement result Apply completed without a currentness check or committed replay")
		}
		return errors.Join(runErr, lease.Unlock())
	}, nil
}

func acquireReviewerResultPublicationLease(
	ctx runtime.Context,
	opt *Options,
) (func(error) error, error) {
	if opt == nil || opt.currentLoopReviewerResultPublication == nil || !opt.Apply {
		return nil, nil
	}
	if opt.currentLoopReviewerMutationLease != nil || opt.currentLoopReviewerResultPublished != nil {
		return nil, fmt.Errorf("reviewer result publication runner already carries mutation ownership")
	}
	publication := *opt.currentLoopReviewerResultPublication
	lease, err := lanemutation.AcquireOpenLane(ctx.Target, publication.Lane, "reviewer result publication")
	if err != nil {
		return nil, err
	}
	fail := func(cause error) (func(error) error, error) {
		return nil, errors.Join(cause, lease.Unlock())
	}
	if err := lease.ValidateLaneFor(ctx.Target, publication.Lane); err != nil {
		return fail(err)
	}
	if err := ensureReviewerWaveLaneNotIntervened(ctx.Target, publication.Lane); err != nil {
		return fail(err)
	}
	prepared, err := executioncontrol.PrepareResultWithLease(ctx.Target, lease, publication)
	if err != nil {
		return fail(err)
	}
	if prepared.Held {
		return fail(&executioncontrol.ResultHeldError{Publication: prepared})
	}
	if prepared.Disposition != executioncontrol.ResultDispositionCurrent {
		return fail(fmt.Errorf("reviewer result preparation returned unexpected disposition %q", prepared.Disposition))
	}
	published := false
	opt.currentLoopReviewerMutationLease = lease
	opt.currentLoopReviewerResultPublished = &published
	return func(runErr error) error {
		if runErr == nil && !published {
			runErr = fmt.Errorf("reviewer result Apply completed without canonical publication")
		}
		return errors.Join(runErr, lease.Unlock())
	}, nil
}

func buildCurrentStepPlan(ctx runtime.Context, opt Options) (currentStepPlan, error) {
	status, err := buildInvocationStatusInventory(ctx, opt)
	if err != nil {
		return currentStepPlan{}, err
	}
	return buildCurrentStepPlanFromStatus(ctx, opt, status)
}

func currentStepUsesCheckpointSourceRequest(opt Options, status statusInventory) bool {
	if status.MissionControlRunbook == nil || status.MissionControlRunbook.CurrentLoopOperator == nil ||
		status.MissionControlRunbook.CurrentLoopOperator.SourceCurrentDriverRequest == nil {
		return false
	}
	source := *status.MissionControlRunbook.CurrentLoopOperator.SourceCurrentDriverRequest
	return currentStepRequestIsEvidenceReview(source) || opt.currentLoopExternalTurnResume ||
		currentStepHasMemberObservation(opt) || currentStepHasReviewerObservation(opt)
}

func currentStepPackMemoryConsumerRequest(repoRoot, caseRoot, pack string, status statusInventory) (*mission.MissionCommanderDriverRequest, error) {
	if status.MissionControlRunbook == nil || status.MissionControlRunbook.Scope != "pack-memory" || status.CaseMission == nil || status.CaseMission.MissionCommanderActionQueue.CurrentDriverRequest == nil {
		return nil, fmt.Errorf("current pack-memory focus has no case member request")
	}
	request := status.CaseMission.MissionCommanderActionQueue.CurrentDriverRequest
	lane := strings.TrimSpace(request.Lane)
	if err := currentStepValidatePackMemoryConsumerTask(repoRoot, caseRoot, pack, lane); err != nil {
		return nil, err
	}
	invocation := statusMissionControlInvocationDriverRequest(caseRoot, *request)
	invocation = mission.MissionCommanderDriverRequestWithRefreshStatusCommand(
		invocation,
		status.MissionControlRunbook.RefreshStatusCommand,
	)
	return cloneStatusMissionCommanderDriverRequest(&invocation), nil
}

func buildCurrentStepPlanFromStatus(ctx runtime.Context, opt Options, status statusInventory) (currentStepPlan, error) {
	if status.MissionControlRunbook != nil && status.MissionControlRunbook.Focus == "case-lane-choice" && status.MissionControlRunbook.CurrentDriverRequest == nil {
		return currentStepPlan{}, fmt.Errorf("run-current-step requires -Lane from the current typed lane choices")
	}
	if status.MissionControlRunbook == nil || status.MissionControlRunbook.CurrentDriverRequest == nil {
		return currentStepPlan{}, fmt.Errorf("run-current-step requires missionControlRunbook.currentDriverRequest")
	}
	packMemoryConsumerLane := ""
	if status.MissionControlRunbook.Scope == "case" {
		lane := strings.TrimSpace(status.MissionControlRunbook.CurrentDriverRequest.Lane)
		board, err := mission.ReadBoard(ctx.Target)
		if err != nil {
			return currentStepPlan{}, fmt.Errorf("run-current-step current task binding board: %w", err)
		}
		owner, ownerReady := mission.LookupBoardLane(board.Lanes, lane, false)
		ownerReady = ownerReady && strings.TrimSpace(owner.CurrentExecutor) != "" && owner.ExecutorGeneration > 0
		if ownerReady {
			binding, err := memberexecution.CurrentTaskBinding(ctx.Target, lane)
			if err != nil {
				return currentStepPlan{}, fmt.Errorf("run-current-step current task binding: %w", err)
			}
			if binding != nil && binding.Kind == "pack-memory-consumer" {
				if err := currentStepValidatePackMemoryConsumerTask(ctx.RepoRoot, ctx.Target, ctx.Pack, lane); err != nil {
					return currentStepPlan{}, fmt.Errorf("run-current-step pack-memory consumer route is not current: %w", err)
				}
				packMemoryConsumerLane = lane
			}
		}
	}
	if status.MissionControlRunbook.Scope != "case" && status.MissionControlRunbook.Scope != "reviewer" {
		caseRequest, err := currentStepPackMemoryConsumerRequest(ctx.RepoRoot, ctx.Target, ctx.Pack, status)
		if err != nil {
			if status.MissionControlRunbook.Scope != "pack-memory" {
				return currentStepPlan{}, fmt.Errorf("run-current-step supports only focused case or reviewer requests; got scope %q", status.MissionControlRunbook.Scope)
			}
			return currentStepPlan{}, fmt.Errorf("run-current-step pack-memory consumer route is not current: %w", err)
		}
		packMemoryConsumerLane = strings.TrimSpace(caseRequest.Lane)
		runbook := *status.MissionControlRunbook
		runbook.Scope = "case"
		runbook.CurrentDriverRequest = caseRequest
		runbook.CurrentRunLoopStepID = strings.TrimSpace(caseRequest.RunLoopStepID)
		runbook.CurrentCommand = strings.TrimSpace(caseRequest.Command)
		inspection := currentloop.InspectAttached(ctx.Target, caseRequest)
		runbook.CurrentLoopSegment = &inspection
		runbook.CurrentLoopOperator = statusCurrentLoopOperatorPackage(ctx.Target, status.CaseMission, &runbook, inspection)
		if operator := runbook.CurrentLoopOperator; operator != nil && externalSessionDispatcherRequestIsFocused(operator) {
			request, err := externalSessionCurrentStepRequest(operator)
			if err != nil {
				return currentStepPlan{}, fmt.Errorf("run-current-step external session wrapper: %w", err)
			}
			runbook.CurrentDriverRequest = &request
			runbook.CurrentRunLoopStepID = request.RunLoopStepID
			runbook.CurrentCommand = request.Command
		}
		status.MissionControlRunbook = &runbook
	}
	routedRequest := *status.MissionControlRunbook.CurrentDriverRequest
	if currentStepUsesCheckpointSourceRequest(opt, status) {
		routedRequest = *status.MissionControlRunbook.CurrentLoopOperator.SourceCurrentDriverRequest
		runbook := *status.MissionControlRunbook
		runbook.CurrentDriverRequest = &routedRequest
		runbook.CurrentRunLoopStepID = routedRequest.RunLoopStepID
		runbook.CurrentCommand = routedRequest.Command
		status.MissionControlRunbook = &runbook
	}
	plan := currentStepPlan{
		SchemaVersion:          1,
		Command:                commands.RunCurrentStep,
		CaseRoot:               ctx.Target,
		Pack:                   ctx.Pack,
		Route:                  status.MissionControlRunbook.Scope,
		ReviewRequired:         true,
		RequiresConfirmation:   true,
		CurrentDriverRequest:   routedRequest,
		PackMemoryConsumerLane: packMemoryConsumerLane,
		Boundary: []string{
			"router selects only the focused case or reviewer request from refreshed missionControlRunbook status",
			"case steps retain the run-driver-step lane mutation lease and preview hash guards",
			"reviewer steps retain the run-reviewer-step packet, artifact, receipt, candidate hash, and reviewer intake lock guards",
			"the Go runtime does not invoke a shell or Agent tool, spawn or poll sessions, fabricate reviewer output, execute heavy tools, or write authority/confirmed state",
			"status is rebuilt by the selected nested runner after Apply before follow-up work is selected",
		},
	}
	nestedSHA256 := ""
	external, externalSHA256, externalMatched, err := buildCurrentStepExternalSessionPlan(ctx, opt, status, routedRequest)
	if err != nil {
		return currentStepPlan{}, fmt.Errorf("run-current-step external session route: %w", err)
	}
	if externalMatched {
		plan.ExternalSessionStep = external
		plan.RequiresConfirmation = externalSHA256 != ""
		nestedSHA256 = externalSHA256
	} else {
		if currentStepHasExternalSessionInputs(opt) {
			return currentStepPlan{}, fmt.Errorf("run-current-step external session inputs require the focused external session route")
		}
		switch plan.Route {
		case "case":
			if currentStepHasReviewerObservation(opt) {
				return currentStepPlan{}, fmt.Errorf("run-current-step case route does not accept reviewer observation inputs")
			}
			if currentStepRequestIsEvidenceReview(routedRequest) {
				if currentStepHasMemberObservation(opt) {
					return currentStepPlan{}, fmt.Errorf("run-current-step evidence review route does not accept member observation inputs")
				}
				plan.RequiresConfirmation = false
				plan.Boundary = append(plan.Boundary,
					"execution evidence review is a typed user-review stop; it does not dispatch a member or apply the acknowledgement",
					"accepted or superseded durable review closure must become fresh current status before member execution can resume",
				)
				break
			}
			if currentStepHasMemberObservation(opt) && !currentStepRequestOwnsMemberExecution(routedRequest) {
				return currentStepPlan{}, fmt.Errorf("run-current-step member observation requires the current member-owned continuation request")
			}
			if currentStepRequestUsesMemberContinueCommand(routedRequest) && !currentStepRequestOwnsMemberExecution(routedRequest) {
				return currentStepPlan{}, fmt.Errorf("run-current-step rejects unrecognized member continuation source %q state %q", routedRequest.Source, routedRequest.State)
			}
			if currentStepRequestOwnsMemberExecution(routedRequest) {
				member, intake, err := buildMemberExecutionStep(ctx, opt, routedRequest)
				if err != nil {
					return currentStepPlan{}, fmt.Errorf("run-current-step member execution: %w", err)
				}
				if member != nil {
					plan.MemberExecution = member
					if !intake {
						nestedSHA256 = member.ExpectedPlanSHA256
						break
					}
				}
			}
			nested, err := buildDriverStepPlanFromStatus(ctx, status)
			if err != nil {
				return currentStepPlan{}, fmt.Errorf("run-current-step case route: %w", err)
			}
			plan.DriverStep = &nested
			plan.CurrentDriverRequest = nested.CurrentDriverRequest
			nestedSHA256 = nested.ExpectedDriverStepPlanSHA256
		case "reviewer":
			nested, err := buildReviewerStepPlanFromStatus(ctx, opt, status)
			if err != nil {
				return currentStepPlan{}, fmt.Errorf("run-current-step reviewer route: %w", err)
			}
			if !currentStepReviewerRequestsMatch(ctx.Target, routedRequest, nested.CurrentDriverRequest) {
				return currentStepPlan{}, fmt.Errorf("run-current-step reviewer route request drift: missionControlRunbook current request does not match reviewer operator package request")
			}
			plan.ReviewerStep = &nested
			plan.CurrentDriverRequest = nested.CurrentDriverRequest
			plan.RequiresConfirmation = nested.ExternalHandoff == nil
			nestedSHA256 = nested.ExpectedReviewerStepPlanSHA256
		}
	}
	if nestedSHA256 == "" {
		return plan, nil
	}
	identity := currentStepPlanIdentity{
		Route:                        plan.Route,
		RoutedDriverRequest:          routedRequest,
		NestedDriverRequest:          plan.CurrentDriverRequest,
		ExpectedNestedStepPlanSHA256: nestedSHA256,
	}
	if plan.MemberExecution != nil {
		identity.ExpectedMemberPlanSHA256 = plan.MemberExecution.ExpectedPlanSHA256
	}
	if plan.ExternalSessionStep != nil {
		externalIdentity := currentStepExternalIdentity(plan.ExternalSessionStep)
		identity.External = &externalIdentity
	}
	if publication := opt.currentLoopReplacementResultPublication; publication != nil {
		if plan.ExternalSessionStep == nil || plan.ExternalSessionStep.Mode != "replacement-attempt" ||
			plan.ExternalSessionStep.Attempt == nil || strings.TrimSpace(plan.ExternalSessionStep.Attempt.Attempt.SupersedesSHA256) == "" {
			return currentStepPlan{}, fmt.Errorf("replacement result publication provenance requires the exact replacement-attempt route")
		}
		identity.ReplacementResultPublication = &currentStepResultPublicationIdentity{
			Lane: publication.Lane, Birth: publication.Birth, Source: publication.Source,
			Actor: publication.Actor, ObservedAt: publication.ObservedAt,
		}
	}
	encoded, err := json.Marshal(identity)
	if err != nil {
		return currentStepPlan{}, err
	}
	sum := sha256.Sum256(encoded)
	plan.ExpectedCurrentStepPlanSHA256 = hex.EncodeToString(sum[:])
	return plan, nil
}

func validateCurrentMemberExecutionRequest(ctx runtime.Context, opt Options, expected mission.MissionCommanderDriverRequest) error {
	fresh, err := buildInvocationStatusInventory(ctx, opt)
	if err != nil {
		return fmt.Errorf("member execution current request refresh: %w", err)
	}
	if fresh.MissionControlRunbook == nil || fresh.MissionControlRunbook.CurrentDriverRequest == nil {
		return fmt.Errorf("member execution current request is unavailable")
	}
	current := *fresh.MissionControlRunbook.CurrentDriverRequest
	if !currentStepRequestOwnsMemberExecution(current) {
		return fmt.Errorf("member execution current request is no longer the member-owned continuation")
	}
	expectedSHA256, err := currentloop.RequestSHA256(expected)
	if err != nil {
		return fmt.Errorf("member execution expected current request identity: %w", err)
	}
	currentSHA256, err := currentloop.RequestSHA256(current)
	if err != nil {
		return fmt.Errorf("member execution fresh current request identity: %w", err)
	}
	if !strings.EqualFold(expectedSHA256, currentSHA256) {
		return fmt.Errorf("member execution current request changed before Apply")
	}
	return nil
}

func applyCurrentStepPlan(ctx runtime.Context, opt Options, plan currentStepPlan) (currentStepPlan, error) {
	if plan.ExternalSessionStep != nil {
		return applyCurrentStepExternalSession(ctx, opt, plan)
	}
	var refreshed *statusInventory
	nestedCommand := ""
	switch plan.Route {
	case "case":
		if plan.MemberExecution != nil {
			var memberResult memberexecution.Result
			applyMember := func() error {
				var err error
				memberResult, err = memberexecution.ApplyCurrentWithLease(
					*plan.MemberExecution,
					opt.ExpectedMemberExecutionPlanSHA256,
					func(lease *lanemutation.Lease) error {
						if binding := opt.currentLoopExecutionControlBinding; binding != nil {
							if err := executioncontrol.RequireCurrentBindingWithLease(ctx.Target, lease, *binding); err != nil {
								return err
							}
						}
						return validateCurrentMemberExecutionRequest(ctx, opt, plan.CurrentDriverRequest)
					},
				)
				return err
			}
			var err error
			if plan.PackMemoryConsumerLane != "" {
				err = currentStepWithPackMemoryConsumerTaskLease(ctx.RepoRoot, ctx.Target, ctx.Pack, plan.PackMemoryConsumerLane, applyMember)
			} else {
				err = applyMember()
			}
			if err != nil {
				return currentStepPlan{}, currentStepZeroProgressError{cause: err}
			}
			plan.MemberExecution = &memberResult.Plan
			plan.IsMutation = memberResult.Applied
			plan.Applied = memberResult.Applied
			plan.memberExecutionAlreadyApplied = memberResult.AlreadyApplied
			plan.ReviewRequired = false
			plan.RequiresConfirmation = false
			nestedCommand = commands.RunCurrentStep
			plan.Receipt = &currentStepReceipt{State: "refreshed", Outcome: "current-step-applied", Route: plan.Route, NestedCommand: nestedCommand, Boundary: append([]string{"member execution outcome: " + memberResult.Inspection.State}, boundariesForMemberReceipt()...)}
			if !plan.Applied {
				if !plan.memberExecutionAlreadyApplied || opt.Command != commands.RunCurrentLoop {
					return plan, nil
				}
				plan.Receipt.Boundary = append(plan.Receipt.Boundary,
					"the current-loop recovered an exact already-published member execution after owner, plan hash, and artifact bytes were revalidated",
				)
			}
			if currentStepBeforeStatusRefreshHook != nil {
				if err := currentStepBeforeStatusRefreshHook(nestedCommand); err != nil {
					return currentStepPartialResult(plan, nestedCommand), fmt.Errorf("refresh status after member execution: %w", err)
				}
			}
			refreshOpt, err := optionsWithEffectiveSelectedCurrentLane(opt, plan.CurrentDriverRequest.Lane)
			if err != nil {
				return currentStepPartialResult(plan, nestedCommand), fmt.Errorf("refresh status after member execution: %w", err)
			}
			fresh, err := buildInvocationStatusInventory(ctx, refreshOpt)
			if err != nil {
				return currentStepPartialResult(plan, nestedCommand), fmt.Errorf("refresh status after member execution: %w", err)
			}
			plan.RefreshedStatus = &fresh
			if fresh.MissionControlRunbook != nil {
				plan.Receipt.RefreshedCurrentDriverRequest = fresh.MissionControlRunbook.CurrentDriverRequest
			}
			return plan, nil
		}
		if plan.DriverStep == nil {
			return currentStepPlan{}, currentStepZeroProgressError{cause: fmt.Errorf("run-current-step case route omitted driver step plan")}
		}
		nested, err := applyDriverStepPlan(ctx, opt, *plan.DriverStep)
		plan.DriverStep = &nested
		plan.Applied = nested.Applied
		refreshed = nested.RefreshedStatus
		nestedCommand = commands.RunDriverStep
		if err != nil {
			if plan.Applied {
				return currentStepPartialResult(plan, nestedCommand), err
			}
			return currentStepPlan{}, err
		}
	case "reviewer":
		if plan.ReviewerStep == nil || plan.ReviewerStep.ExternalHandoff != nil {
			return currentStepPlan{}, currentStepZeroProgressError{cause: fmt.Errorf("run-current-step reviewer route requires a deterministic reviewer step plan")}
		}
		nested, err := applyReviewerStepPlan(ctx, opt, *plan.ReviewerStep)
		plan.ReviewerStep = &nested
		plan.Applied = nested.Applied
		refreshed = nested.RefreshedStatus
		nestedCommand = commands.RunReviewerStep
		if err != nil {
			if plan.Applied {
				return currentStepPartialResult(plan, nestedCommand), err
			}
			return currentStepPlan{}, err
		}
	default:
		return currentStepPlan{}, fmt.Errorf("run-current-step route %q is unsupported", plan.Route)
	}
	plan.IsMutation = true
	plan.ReviewRequired = false
	plan.RequiresConfirmation = false
	plan.RefreshedStatus = refreshed
	plan.Receipt = &currentStepReceipt{
		State:         "refreshed",
		Outcome:       "current-step-applied",
		Route:         plan.Route,
		NestedCommand: nestedCommand,
		Boundary: []string{
			"receipt identifies the selected nested runner; it does not prove external session execution",
			"consume refreshedCurrentDriverRequest only after the selected nested runner rebuilt durable status",
			"no authority/confirmed state or heavy-tool execution is produced by this router",
		},
	}
	if refreshed != nil && refreshed.MissionControlRunbook != nil {
		plan.Receipt.RefreshedCurrentDriverRequest = refreshed.MissionControlRunbook.CurrentDriverRequest
	}
	return plan, nil
}

func currentStepPartialResult(plan currentStepPlan, nestedCommand string) currentStepPlan {
	state := "refresh-failed"
	outcome := "current-step-applied-status-refresh-failed"
	boundary := "the nested mutation was applied, but refreshed durable status is unavailable"
	if plan.ExternalSessionStep != nil && plan.ExternalSessionStep.Turn != nil && strings.TrimSpace(plan.ExternalSessionStep.Turn.FailureStage) != "" {
		stage := strings.TrimSpace(plan.ExternalSessionStep.Turn.FailureStage)
		state = "nested-partial"
		outcome = "current-step-applied-external-turn-" + stage + "-failed"
		boundary = "the external session relay was committed, but the nested external-result turn stopped at " + stage
	} else if plan.RefreshedStatus != nil {
		state = "receipt-failed"
		outcome = "current-step-applied-receipt-finalization-failed"
		boundary = "the nested mutation and status refresh completed, but receipt finalization failed"
	}
	plan.IsMutation = true
	plan.ReviewRequired = false
	plan.RequiresConfirmation = false
	plan.Receipt = &currentStepReceipt{
		State:         state,
		Outcome:       outcome,
		Route:         plan.Route,
		NestedCommand: nestedCommand,
		Boundary: []string{
			boundary,
			"do not infer follow-up work from an incomplete receipt; run the returned fresh loop resume command after repairing the reported error",
			"no authority/confirmed state or heavy-tool execution is produced by this router",
		},
	}
	return plan
}

func validateCurrentStepOuterArgs(opt Options) error {
	valueFlags := map[string]bool{
		"-command": true, "--command": true,
		"-target": true, "--target": true,
		"-pack": true, "--pack": true,
		"-lane": true, "--lane": true,
		"-format": true, "--format": true,
		"-expectedcurrentstepplansha256": true, "--expected-current-step-plan-sha256": true,
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
		"-externalsessionharness": true, "--external-session-harness": true,
		"-externalsessionid": true, "--external-session-id": true,
		"-externalsessionactor": true, "--external-session-actor": true,
		"-externalsessionstartedat": true, "--external-session-started-at": true,
		"-externalsessionlaunchoutcome": true, "--external-session-launch-outcome": true,
		"-externalsessionobservedat": true, "--external-session-observed-at": true,
		"-externalsessionlaunchreason": true, "--external-session-launch-reason": true,
		"-externalsessiontransportendpoint": true, "--external-session-transport-endpoint": true,
		"-externalsessiondeliveryoutcome": true, "--external-session-delivery-outcome": true,
		"-externalsessionproviderackfingerprint": true, "--external-session-provider-ack-fingerprint": true,
		"-externalsessiondeliveryreason": true, "--external-session-delivery-reason": true,
		"-externalsessionreviewerresultsourcepath": true, "--external-session-reviewer-result-source-path": true,
		"-expectedexternalsessionattemptsha256": true, "--expected-external-session-attempt-sha256": true,
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
				return fmt.Errorf("run-current-step accepts -- only once at the start of the argument list")
			}
			separatorSeen = true
			continue
		}
		key := strings.ToLower(strings.SplitN(token, "=", 2)[0])
		if !strings.HasPrefix(key, "-") {
			return fmt.Errorf("run-current-step contains unsupported positional argument %s", token)
		}
		canonical := currentStepCanonicalOuterFlag(key)
		if key != canonical && !valueFlags[key] && !switchFlags[key] {
			return fmt.Errorf("run-current-step contains unsupported flag %s", token)
		}
		if seen[canonical] {
			return fmt.Errorf("run-current-step repeats flag %s", token)
		}
		seen[canonical] = true
		if switchFlags[key] {
			continue
		}
		if !valueFlags[key] {
			return fmt.Errorf("run-current-step contains unsupported flag %s", token)
		}
		if !strings.Contains(token, "=") {
			if i+1 >= len(opt.rawArgs) || strings.HasPrefix(opt.rawArgs[i+1], "-") {
				return fmt.Errorf("run-current-step flag %s is missing a value", token)
			}
			i++
		}
	}
	return nil
}

func currentStepCanonicalOuterFlag(key string) string {
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
	case "--expected-current-step-plan-sha256":
		return "-expectedcurrentstepplansha256"
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
	case "--external-session-harness":
		return "-externalsessionharness"
	case "--external-session-id":
		return "-externalsessionid"
	case "--external-session-actor":
		return "-externalsessionactor"
	case "--external-session-started-at":
		return "-externalsessionstartedat"
	case "--external-session-launch-outcome":
		return "-externalsessionlaunchoutcome"
	case "--external-session-observed-at":
		return "-externalsessionobservedat"
	case "--external-session-launch-reason":
		return "-externalsessionlaunchreason"
	case "--external-session-transport-endpoint":
		return "-externalsessiontransportendpoint"
	case "--external-session-delivery-outcome":
		return "-externalsessiondeliveryoutcome"
	case "--external-session-provider-ack-fingerprint":
		return "-externalsessionproviderackfingerprint"
	case "--external-session-delivery-reason":
		return "-externalsessiondeliveryreason"
	case "--external-session-reviewer-result-source-path":
		return "-externalsessionreviewerresultsourcepath"
	case "--expected-external-session-attempt-sha256":
		return "-expectedexternalsessionattemptsha256"
	default:
		return key
	}
}

func currentStepReviewerRequestsMatch(caseRoot string, routed, nested mission.MissionCommanderDriverRequest) bool {
	if strings.TrimSpace(caseRoot) == "" ||
		strings.TrimSpace(routed.Source) != "reviewerDispatchOperatorPackage" ||
		strings.TrimSpace(nested.Source) != "reviewerDispatchOperatorPackage" {
		return false
	}
	projection, err := resolveProjectPublicProjection(caseRoot)
	if err != nil || mission.ValidateMissionCommanderDriverRequest(nested) != nil {
		return false
	}
	routedRefresh := strings.TrimSpace(
		routed.ExpectedReceipt.RefreshStatusCommand,
	)
	if !driverStepRefreshCommandMatches(
		runtime.Context{Target: caseRoot},
		routedRefresh,
	) {
		return false
	}
	entrypoint := projection.entrypoint
	if !strings.HasPrefix(routedRefresh, entrypoint+" ") {
		return false
	}
	refreshInvocation, err := commands.ParsePublicInvocation(routedRefresh)
	if err != nil {
		return false
	}
	selectedLane, lanePresent, laneValid :=
		refreshInvocation.FlagValue(
			"-Lane",
			"--lane",
		)
	if !lanePresent || !laneValid || selectedLane == "" ||
		selectedLane != strings.TrimSpace(nested.Lane) {
		return false
	}

	projected := nested
	if projected.CommandExecutable {
		projected = statusMissionControlInvocationDriverRequest(
			caseRoot,
			projected,
		)
	}
	refresh, err := statusMissionControlRefreshCommand(caseRoot)
	if err != nil {
		return false
	}
	projected = mission.MissionCommanderDriverRequestWithRefreshStatusCommand(
		projected,
		refresh,
	)
	bindSelectedLaneDriverRequest(&projected, selectedLane)
	projectedSHA256, err := mission.MissionCommanderDriverRequestSHA256(
		projected,
	)
	if err != nil {
		return false
	}
	routedSHA256, err := mission.MissionCommanderDriverRequestSHA256(
		routed,
	)
	return err == nil && routedSHA256 == projectedSHA256
}

func currentStepHasReviewerObservation(opt Options) bool {
	return strings.TrimSpace(opt.ReviewerResultInputSourcePath) != "" ||
		strings.TrimSpace(opt.ReviewerHarness) != "" ||
		strings.TrimSpace(opt.ReviewerSession) != "" ||
		strings.TrimSpace(opt.ReviewerOutcome) != "" ||
		strings.TrimSpace(opt.ReviewerExitStatus) != ""
}
