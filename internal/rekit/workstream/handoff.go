package workstream

import (
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
	"time"

	"github.com/shuiyu486/re-context-kits/internal/rekit/autonomy"
	"github.com/shuiyu486/re-context-kits/internal/rekit/commands"
	"github.com/shuiyu486/re-context-kits/internal/rekit/currentloop"
	refsf "github.com/shuiyu486/re-context-kits/internal/rekit/fs"
	"github.com/shuiyu486/re-context-kits/internal/rekit/instance"
	"github.com/shuiyu486/re-context-kits/internal/rekit/lanecompletion"
	"github.com/shuiyu486/re-context-kits/internal/rekit/lanemutation"
	"github.com/shuiyu486/re-context-kits/internal/rekit/manifest"
	"github.com/shuiyu486/re-context-kits/internal/rekit/mission"
	"github.com/shuiyu486/re-context-kits/internal/rekit/plancontract"
	"github.com/shuiyu486/re-context-kits/internal/rekit/projectstate"
)

const (
	maxHandoffRows                     = 5
	handoffPublicationPlanSHA256Marker = "<handoff-publication-plan-sha256>"
)

var (
	continueOwnerBindingPattern = regexp.MustCompile(`\s+-Executor\s+(?:"(?:\\.|[^"])*"|\S+)\s+-ExpectedExecutorGeneration\s+\d+`)
	handoffApplyBeforeLockHook  func()
	handoffApplyBeforeWriteHook func()
	handoffApplyAfterWriteHook  func(path string, generation bool) error
)

type HandoffOptions struct {
	Selector                           string
	ExpectedPublicationPlanSHA256      string
	PublicationStamp                   string
	ProjectMissionCommanderNextActions []mission.MissionCommanderNextActionItem
	ProjectNextBatchStarterPackage     *ProjectNextBatchStarterPackage
	ProjectPackMemoryConsumption       *PackMemoryConsumptionHandoff
	CurrentLoopOperator                *mission.CurrentLoopOperatorPackage
	CurrentDriverRequest               *mission.MissionCommanderDriverRequest
}

type PackMemoryConsumptionHandoff struct {
	Available                   int                                      `json:"available"`
	Consumed                    int                                      `json:"consumed"`
	Conflicts                   int                                      `json:"conflicts"`
	MissionCommanderNextActions []mission.MissionCommanderNextActionItem `json:"missionCommanderNextActions,omitempty"`
	MissionCommanderActionQueue mission.MissionCommanderActionQueue      `json:"missionCommanderActionQueue"`
	Boundary                    []string                                 `json:"boundary,omitempty"`
}

type ProjectNextBatchStarterPackage struct {
	Ready                   bool                                  `json:"ready"`
	LatestCompletedBatch    string                                `json:"latestCompletedBatch,omitempty"`
	SuggestedNextBatch      string                                `json:"suggestedNextBatch,omitempty"`
	CurrentBatchSection     string                                `json:"currentBatchSection"`
	ChangelogEntry          string                                `json:"changelogEntry"`
	ValidationCommands      []string                              `json:"validationCommands,omitempty"`
	ReleaseCadenceSteps     []string                              `json:"releaseCadenceSteps,omitempty"`
	RecommendedStarterSteps []string                              `json:"recommendedStarterSteps,omitempty"`
	Boundary                []string                              `json:"boundary,omitempty"`
	CurrentRunLoopStepID    string                                `json:"currentRunLoopStepId,omitempty"`
	RunLoop                 []mission.MissionCommanderRunLoopStep `json:"runLoop,omitempty"`
}

type LatestDriverReceiptHandoff struct {
	Ready                         bool                           `json:"ready"`
	State                         string                         `json:"state"`
	RunID                         string                         `json:"runId,omitempty"`
	BatchID                       string                         `json:"batchId,omitempty"`
	Lane                          string                         `json:"lane,omitempty"`
	Command                       string                         `json:"command,omitempty"`
	RunStatusPath                 string                         `json:"runStatusPath,omitempty"`
	RunDigestPath                 string                         `json:"runDigestPath,omitempty"`
	MissionCommanderDriverReceipt *MissionCommanderDriverReceipt `json:"missionCommanderDriverReceipt,omitempty"`
	TargetDocuments               []string                       `json:"targetDocuments,omitempty"`
	Boundary                      []string                       `json:"boundary,omitempty"`
}

type HandoffResult struct {
	SchemaVersion                      int                                         `json:"schemaVersion"`
	Command                            string                                      `json:"command"`
	CaseRoot                           string                                      `json:"caseRoot"`
	RepoRoot                           string                                      `json:"repoRoot"`
	Pack                               string                                      `json:"pack"`
	IsMutation                         bool                                        `json:"isMutation"`
	Applied                            bool                                        `json:"applied"`
	RequiresConfirmation               bool                                        `json:"requiresConfirmation"`
	Selector                           string                                      `json:"selector,omitempty"`
	Project                            bool                                        `json:"project"`
	Lane                               *Lane                                       `json:"lane,omitempty"`
	LaneTakeoverPackage                *LaneTakeoverPackage                        `json:"laneTakeoverPackage,omitempty"`
	LatestDriverReceiptHandoff         *LatestDriverReceiptHandoff                 `json:"latestDriverReceiptHandoff,omitempty"`
	MissionBrief                       mission.Brief                               `json:"missionBrief"`
	ExecutorAction                     *laneExecutorAction                         `json:"executorAction,omitempty"`
	LaneExecutorActions                []mission.LaneExecutorActionSnapshot        `json:"laneExecutorActions,omitempty"`
	ExecutionEvidenceReview            []ExecutionEvidenceReviewItem               `json:"executionEvidenceReview,omitempty"`
	ExecutionEvidenceReviewSummary     ExecutionEvidenceReviewSummary              `json:"executionEvidenceReviewSummary"`
	ReviewerWritebacks                 []ReviewerWritebackItem                     `json:"reviewerWritebacks,omitempty"`
	ReviewerWritebackSummary           ReviewerWritebackSummary                    `json:"reviewerWritebackSummary"`
	ReviewerDispatchIntakeHandoffs     []ReviewerDispatchIntakeHandoff             `json:"reviewerDispatchIntakeHandoffs,omitempty"`
	ReviewerDispatchIntakeSummary      ReviewerDispatchIntakeSummary               `json:"reviewerDispatchIntakeSummary"`
	ReviewerPacketRetirementHandoffs   []ReviewerPacketRetirementHandoff           `json:"reviewerPacketRetirementHandoffs,omitempty"`
	ReviewerPacketRetirementSummary    ReviewerPacketRetirementSummary             `json:"reviewerPacketRetirementSummary"`
	AuthorizedGateAdapterHandoffs      []AuthorizedGateAdapterHandoff              `json:"authorizedGateAdapterHandoffs,omitempty"`
	MissionCommanderNextActions        []mission.MissionCommanderNextActionItem    `json:"missionCommanderNextActions,omitempty"`
	MissionCommanderActionQueue        mission.MissionCommanderActionQueue         `json:"missionCommanderActionQueue"`
	DailyMissionControlRunbook         *DailyMissionControlRunbook                 `json:"dailyMissionControlRunbook,omitempty"`
	ReplacementExecutorTakeoverPackage *mission.ReplacementExecutorTakeoverPackage `json:"replacementExecutorTakeoverPackage,omitempty"`
	CurrentLoopSegment                 *currentloop.Inspection                     `json:"currentLoopSegment,omitempty"`
	CurrentLoopOperator                *mission.CurrentLoopOperatorPackage         `json:"currentLoopOperator,omitempty"`
	ProjectNextBatchStarterPackage     *ProjectNextBatchStarterPackage             `json:"projectNextBatchStarterPackage,omitempty"`
	PackMemoryConsumption              *PackMemoryConsumptionHandoff               `json:"packMemoryConsumption,omitempty"`
	PublicationPlanSHA256              string                                      `json:"publicationPlanSha256,omitempty"`
	PublicationStamp                   string                                      `json:"publicationStamp,omitempty"`
	ApplyCommand                       string                                      `json:"applyCommand,omitempty"`
	ApplyArgs                          []string                                    `json:"applyArgs,omitempty"`
	Writes                             []StartWrite                                `json:"writes"`
	BlockedActions                     []string                                    `json:"blockedActions"`
	NextSteps                          []string                                    `json:"nextSteps"`
}

func HandoffPreview(repoRoot, caseRoot, pack string, opt HandoffOptions) (HandoffResult, error) {
	ctx, err := newHandoffContext(repoRoot, caseRoot, pack, opt)
	if errors.Is(err, os.ErrNotExist) && strings.TrimSpace(opt.Selector) == "" {
		return missingBoardHandoffPreview(repoRoot, caseRoot, pack)
	}
	if err != nil {
		return HandoffResult{}, err
	}
	if _, err := plancontract.ValidatePhase(
		commands.Handoff,
		"-ExpectedHandoffPlanSha256",
		true,
		false,
		opt.ExpectedPublicationPlanSHA256,
	); err != nil {
		return HandoffResult{}, err
	}
	writes, err := ctx.plannedWrites(false)
	if err != nil {
		return HandoffResult{}, err
	}
	result, err := ctx.result(false, false, true, writes)
	if err != nil {
		return HandoffResult{}, err
	}
	takeoverWrites, err := ctx.replacementExecutorTakeoverPackageArtifactWrites(false, result.ReplacementExecutorTakeoverPackage)
	if err != nil {
		return HandoffResult{}, err
	}
	result.Writes = append(result.Writes, takeoverWrites...)
	plan, err := ctx.buildPublicationPlan()
	if err != nil {
		return HandoffResult{}, err
	}
	result = plan.Preview
	result.PublicationPlanSHA256 = plan.PublicationSHA256
	result.PublicationStamp = ctx.stamp
	result.ApplyArgs = handoffApplyArgs(ctx.inst.CaseRoot, ctx.manifest.Pack, ctx.canonicalSelector(), result.PublicationPlanSHA256, ctx.stamp)
	result.ApplyCommand = handoffCommandForArgs(result.ApplyArgs)
	bindHandoffApplyRoute(&result, result.ApplyCommand)
	return result, nil
}

func MissingBoardOnboardingAction(caseRoot string) mission.MissionCommanderNextActionItem {
	action, err := missingBoardOnboardingAction(caseRoot)
	if err == nil {
		return action
	}
	return mission.MissionCommanderNextActionItem{
		State:    "case-state-root-invalid",
		Source:   "caseMissionOnboarding",
		ActionID: "case-mission-onboarding",
		Blocked:  true,
		Reasons:  []string{err.Error()},
		Boundary: []string{
			"state-root conflicts must be resolved before publishing an executable onboarding action",
			"do not select, merge, or write either state root automatically",
		},
	}
}

func missingBoardOnboardingAction(caseRoot string) (mission.MissionCommanderNextActionItem, error) {
	_, entrypoint, err := selectedMissionCommanderSurface(caseRoot)
	if err != nil {
		return mission.MissionCommanderNextActionItem{}, err
	}
	command := entrypoint + " overview -Target " + quoteAlwaysCommandArg(caseRoot) + " -Format text"
	return mission.MissionCommanderNextActionItem{
		State:    "case-board-missing",
		Command:  command,
		Source:   "caseMissionOnboarding",
		ActionID: "case-mission-onboarding",
		Reasons: []string{
			"case-local Mission Commander board is missing",
			"initialize bounded case-local board before continue/start",
		},
		Boundary: []string{
			"status and handoff previews are read-only; they only project this onboarding action",
			"overview may bootstrap case-local Mission Commander board and does not execute heavy tools",
			"after onboarding, use -WhatIf before start/continue apply",
		},
	}, nil
}

func StartBootstrapAction(caseRoot string) mission.MissionCommanderNextActionItem {
	_, entrypoint, err := selectedMissionCommanderSurface(caseRoot)
	if err != nil {
		return mission.MissionCommanderNextActionItem{
			State:    "case-state-root-invalid",
			Source:   "caseMissionStartBootstrap",
			ActionID: "case-start-bootstrap",
			Blocked:  true,
			Reasons:  []string{err.Error()},
			Boundary: []string{"resolve the state-root conflict before publishing a start preview"},
		}
	}
	command := entrypoint + " start -Target " + quoteAlwaysCommandArg(caseRoot) + " -Name triage -WhatIf -Format json"
	return mission.MissionCommanderNextActionItem{
		Label:          "triage",
		ActionID:       "case-start-bootstrap",
		State:          "start-bootstrap-preview-required",
		Command:        command,
		Source:         "caseMissionStartBootstrap",
		RequiresReview: true,
		Reasons: []string{
			"case board exists but no lane current action is available",
			"preview the first default workstream lane before writing case-local lane/board/resume/checkpoint state",
		},
		Boundary: []string{
			"status and handoff previews are read-only; they only project this start bootstrap preview request",
			"start bootstrap uses the existing start WhatIf flow and does not execute start Apply automatically",
			"start Apply only writes case-local lane/board/resume/checkpoint state after review",
			"no authority/confirmed writes",
			"no heavy-tool execution",
		},
	}
}

func quoteAlwaysCommandArg(value string) string {
	return `"` + strings.ReplaceAll(value, `"`, `\"`) + `"`
}

func missingBoardHandoffPreview(repoRoot, caseRoot, pack string) (HandoffResult, error) {
	inst, err := instance.AssertAttached(caseRoot, repoRoot, pack)
	if err != nil {
		return HandoffResult{}, err
	}
	m, err := manifest.Load(repoRoot, pack)
	if err != nil {
		return HandoffResult{}, err
	}
	boardPath, entrypoint, err := selectedMissionCommanderSurface(inst.CaseRoot)
	if err != nil {
		return HandoffResult{}, err
	}
	action, err := missingBoardOnboardingAction(inst.CaseRoot)
	if err != nil {
		return HandoffResult{}, err
	}
	queue := mission.MissionCommanderActionQueueFor([]mission.MissionCommanderNextActionItem{action})
	runbook := DailyMissionControlRunbookFor(inst.CaseRoot, "case-onboarding", queue, handoffPreviewCommand(inst.CaseRoot, m.Pack, ""), handoffApplyCommand(inst.CaseRoot, m.Pack, ""))
	packagePath, err := stateRelPath(inst.CaseRoot, "handovers", "latest-replacement-executor-takeover.json")
	if err != nil {
		return HandoffResult{}, err
	}
	takeover, err := handoffReplacementExecutorTakeoverPackage(inst.CaseRoot, "case-onboarding", nil, queue, runbook, nil, packagePath, nil, false)
	if err != nil {
		return HandoffResult{}, err
	}
	return HandoffResult{
		SchemaVersion:                      1,
		Command:                            "handoff",
		CaseRoot:                           inst.CaseRoot,
		RepoRoot:                           m.RepoRoot,
		Pack:                               m.Pack,
		IsMutation:                         false,
		Applied:                            false,
		RequiresConfirmation:               true,
		Project:                            true,
		MissionBrief:                       mission.Brief{Summary: "case board missing; run overview to initialize Mission Commander state", NextAgentActions: []string{"follow Mission Commander current action: " + action.Command}},
		MissionCommanderNextActions:        []mission.MissionCommanderNextActionItem{action},
		MissionCommanderActionQueue:        queue,
		DailyMissionControlRunbook:         runbook,
		ReplacementExecutorTakeoverPackage: takeover,
		Writes:                             []StartWrite{},
		BlockedActions: []string{
			"handoff apply until " + boardPath + " exists",
			"board/facts/lane creation must go through " + entrypoint + " overview onboarding",
			"authority/confirmed writes",
			"heavy-tool execution without a valid current authorization decision",
		},
		NextSteps: []string{
			"consume replacementExecutorTakeoverPackage.currentDriverRequest.command to run " + entrypoint + " overview once",
			"refresh status after overview before start/continue apply",
		},
	}, nil
}

func HandoffApply(repoRoot, caseRoot, pack string, opt HandoffOptions) (result HandoffResult, err error) {
	ctx, err := newHandoffContext(repoRoot, caseRoot, pack, opt)
	if err != nil {
		return HandoffResult{}, err
	}
	expected, err := plancontract.RequireApplyBinding(
		commands.Handoff,
		"-ExpectedHandoffPlanSha256",
		opt.ExpectedPublicationPlanSHA256,
	)
	if err != nil {
		return HandoffResult{}, err
	}
	if strings.TrimSpace(opt.PublicationStamp) == "" {
		return HandoffResult{}, fmt.Errorf("handoff apply requires -HandoffPublicationStamp from the same WhatIf preview")
	}
	if handoffApplyBeforeLockHook != nil {
		handoffApplyBeforeLockHook()
	}
	var lease *workstreamMutationLease
	if ctx.project {
		lease, err = acquireProjectMutationLock(ctx.inst.CaseRoot)
	} else {
		lease, err = acquireLaneMutationLock(ctx.inst.CaseRoot, ctx.lane.ID)
	}
	if err != nil {
		return HandoffResult{}, err
	}
	defer func() {
		if unlockErr := lease.Unlock(); unlockErr != nil {
			err = errors.Join(err, unlockErr)
			result = HandoffResult{}
		}
	}()
	ctx, err = newHandoffContext(repoRoot, caseRoot, pack, opt)
	if err != nil {
		return HandoffResult{}, err
	}
	if err := lease.Validate(); err != nil {
		return HandoffResult{}, err
	}
	plan, err := ctx.buildPublicationPlan()
	if err != nil {
		return HandoffResult{}, err
	}
	if _, err := plancontract.Match(
		commands.Handoff,
		"-ExpectedHandoffPlanSha256",
		expected,
		plan.PublicationSHA256,
	); err != nil {
		return HandoffResult{}, err
	}
	if err := ctx.publishPublicationPlan(plan); err != nil {
		return HandoffResult{}, err
	}
	result = plan.Preview
	result.IsMutation = true
	result.Applied = true
	result.RequiresConfirmation = false
	result.PublicationPlanSHA256 = plan.PublicationSHA256
	result.PublicationStamp = ctx.stamp
	result.ApplyArgs = handoffApplyArgs(ctx.inst.CaseRoot, ctx.manifest.Pack, ctx.canonicalSelector(), plan.PublicationSHA256, ctx.stamp)
	result.ApplyCommand = handoffCommandForArgs(result.ApplyArgs)
	bindHandoffApplyRoute(&result, result.ApplyCommand)
	result.Writes = appliedHandoffWrites(plan.Writes)
	nextSteps, err := appliedHandoffNextSteps(result)
	if err != nil {
		return HandoffResult{}, err
	}
	result.NextSteps = mission.UniqueStrings(append([]string{"use /rekit as the Mission Commander entrypoint; JSON preview/apply is Go-owned by default"}, nextSteps...))
	return result, nil
}

type handoffContext struct {
	inst                               instance.Instance
	manifest                           *manifest.Manifest
	board                              board
	selector                           string
	project                            bool
	lane                               *Lane
	stamp                              string
	handovers                          string
	projectMissionCommanderNextActions []mission.MissionCommanderNextActionItem
	projectNextBatchStarterPackage     *ProjectNextBatchStarterPackage
	projectPackMemoryConsumption       *PackMemoryConsumptionHandoff
	currentLoopOperator                *mission.CurrentLoopOperatorPackage
	currentDriverRequest               *mission.MissionCommanderDriverRequest
	publicationPlanSHA256              string
}

func (ctx handoffContext) dailyMissionControlRunbook(scope, selector string, queue mission.MissionCommanderActionQueue) *DailyMissionControlRunbook {
	applyCommand := handoffApplyCommand(ctx.inst.CaseRoot, ctx.manifest.Pack, selector)
	applyReady := strings.TrimSpace(ctx.publicationPlanSHA256) != "" &&
		strings.EqualFold(strings.TrimSpace(selector), ctx.canonicalSelector())
	if applyReady {
		applyCommand = handoffApplyCommand(ctx.inst.CaseRoot, ctx.manifest.Pack, selector, ctx.publicationPlanSHA256, ctx.stamp)
	}
	refreshCommand := ""
	if queue.CurrentDriverRequest != nil {
		refreshCommand = strings.TrimSpace(queue.CurrentDriverRequest.ExpectedReceipt.RefreshStatusCommand)
	}
	return dailyMissionControlRunbookForWithRefresh(
		ctx.inst.CaseRoot,
		scope,
		queue,
		handoffPreviewCommand(ctx.inst.CaseRoot, ctx.manifest.Pack, selector),
		applyCommand,
		applyReady,
		refreshCommand,
	)
}

func (ctx handoffContext) canonicalSelector() string {
	if ctx.lane != nil {
		return strings.TrimSpace(ctx.lane.ID)
	}
	return strings.TrimSpace(ctx.selector)
}

func newHandoffContext(repoRoot, caseRoot, pack string, opt HandoffOptions) (handoffContext, error) {
	inst, err := instance.AssertAttached(caseRoot, repoRoot, pack)
	if err != nil {
		return handoffContext{}, err
	}
	m, err := manifest.Load(repoRoot, pack)
	if err != nil {
		return handoffContext{}, err
	}
	operations, err := lanecompletion.InspectOperations(inst.CaseRoot)
	if err != nil {
		return handoffContext{}, fmt.Errorf("handoff refuses invalid reopen operation lifecycle: %w", err)
	}
	if operations.Pending {
		return handoffContext{}, fmt.Errorf("handoff refuses pending reopen operation; recover the exact reopen Apply before publishing replacement takeover state")
	}
	b, err := readBoard(inst.CaseRoot)
	if os.IsNotExist(err) {
		boardPath, entrypoint, pathErr := selectedMissionCommanderSurface(inst.CaseRoot)
		if pathErr != nil {
			return handoffContext{}, pathErr
		}
		return handoffContext{}, fmt.Errorf("handoff requires existing %s; run %s start -Apply or %s overview once to initialize the case-local board: %w", boardPath, entrypoint, entrypoint, err)
	}
	if err != nil {
		return handoffContext{}, err
	}
	if strings.TrimSpace(b.DefaultAuthorityLane) == "" {
		b.DefaultAuthorityLane = m.WorkstreamDefaults["defaultAuthorityLane"]
	}
	selector := strings.TrimSpace(opt.Selector)
	stamp := strings.TrimSpace(opt.PublicationStamp)
	if stamp == "" {
		stamp = handoffTimestamp()
	}
	if _, err := ParseHandoffPublicationStamp(stamp); err != nil {
		return handoffContext{}, fmt.Errorf("handoff publication stamp is invalid: %s", stamp)
	}
	ctx := handoffContext{inst: inst, manifest: m, board: b, selector: selector, project: selector == "", stamp: stamp, currentLoopOperator: opt.CurrentLoopOperator}
	if ctx.project {
		ctx.projectMissionCommanderNextActions = mission.UniqueCommanderNextActions(opt.ProjectMissionCommanderNextActions)
		ctx.projectNextBatchStarterPackage = cloneProjectNextBatchStarterPackage(opt.ProjectNextBatchStarterPackage)
		ctx.projectPackMemoryConsumption = clonePackMemoryConsumptionHandoff(opt.ProjectPackMemoryConsumption)
	} else {
		lane, err := resolveHandoffLane(inst.CaseRoot, b, selector)
		if err != nil {
			return handoffContext{}, err
		}
		if err := lanemutation.AssertLaneOpen(inst.CaseRoot, lane.ID, "handoff"); err != nil {
			return handoffContext{}, err
		}
		ctx.lane = &lane
		ctx.selector = lane.ID
	}
	expectedLane := ""
	if ctx.lane != nil {
		expectedLane = ctx.lane.ID
	}
	ctx.currentDriverRequest, err = validatedHandoffCurrentDriverRequest(inst.CaseRoot, expectedLane, opt.CurrentDriverRequest)
	if err != nil {
		return handoffContext{}, err
	}
	ctx.handovers, err = projectstate.Join(inst.CaseRoot, "handovers")
	if err != nil {
		return handoffContext{}, err
	}
	return ctx, nil
}

func validatedHandoffCurrentDriverRequest(caseRoot, expectedLane string, request *mission.MissionCommanderDriverRequest) (*mission.MissionCommanderDriverRequest, error) {
	if request == nil {
		return nil, nil
	}
	clone := cloneHandoffCurrentDriverRequest(request)
	if err := mission.ValidateMissionCommanderDriverRequest(*clone); err != nil {
		return nil, fmt.Errorf("handoff current driver request is invalid: %w", err)
	}
	target, present, valid := handoffInvocationFlagValue(*clone.Invocation, "-Target", "--target")
	if !present || !valid || !refsf.SamePath(target, caseRoot) {
		return nil, fmt.Errorf("handoff current driver request -Target must bind to the attached project")
	}
	invocationLane, present, valid := handoffInvocationFlagValue(*clone.Invocation, "-Lane", "--lane")
	requestLane := strings.TrimSpace(clone.Lane)
	if !present || !valid || requestLane == "" || invocationLane != requestLane {
		return nil, fmt.Errorf("handoff current driver request must bind one exact -Lane to request.lane")
	}
	lane, err := readLaneByID(caseRoot, requestLane)
	if err != nil {
		return nil, fmt.Errorf("handoff current driver request lane is not durable: %s: %w", requestLane, err)
	}
	if lane.ID != requestLane {
		return nil, fmt.Errorf("handoff current driver request lane must use the exact durable lane ID: %s", lane.ID)
	}
	if err := lanemutation.AssertLaneOpen(caseRoot, lane.ID, "handoff current driver request"); err != nil {
		return nil, err
	}
	if expectedLane = strings.TrimSpace(expectedLane); expectedLane != "" && requestLane != expectedLane {
		return nil, fmt.Errorf("handoff current driver request lane %s differs from resolved handoff lane %s", requestLane, expectedLane)
	}
	refresh, err := validateHandoffStatusRefreshCommand(caseRoot, requestLane, clone.ExpectedReceipt.RefreshStatusCommand)
	if err != nil {
		return nil, err
	}
	clone.Kind = strings.TrimSpace(clone.Kind)
	clone.RunLoopStepID = strings.TrimSpace(clone.RunLoopStepID)
	clone.Actor = strings.TrimSpace(clone.Actor)
	clone.State = strings.TrimSpace(clone.State)
	clone.Source = strings.TrimSpace(clone.Source)
	clone.Lane = requestLane
	clone.Label = strings.TrimSpace(clone.Label)
	clone.GateEventID = strings.TrimSpace(clone.GateEventID)
	clone.ActionID = strings.TrimSpace(clone.ActionID)
	clone.Command = strings.TrimSpace(clone.Command)
	clone.Guidance = strings.TrimSpace(clone.Guidance)
	normalized := mission.MissionCommanderDriverRequestWithRefreshStatusCommand(*clone, refresh)
	clone = &normalized
	clone.ExpectedReceipt.State = strings.TrimSpace(clone.ExpectedReceipt.State)
	clone.ExpectedReceipt.Command = strings.TrimSpace(clone.ExpectedReceipt.Command)
	clone.ExpectedReceipt.RefreshStatusCommand = refresh
	clone.ExpectedReceipt.Description = strings.TrimSpace(clone.ExpectedReceipt.Description)
	clone.Boundary = mission.UniqueStrings(clone.Boundary)
	clone.ExpectedReceipt.Boundary = mission.UniqueStrings(clone.ExpectedReceipt.Boundary)
	return clone, nil
}

func cloneHandoffCurrentDriverRequest(request *mission.MissionCommanderDriverRequest) *mission.MissionCommanderDriverRequest {
	if request == nil {
		return nil
	}
	clone := *request
	if request.Invocation != nil {
		invocation := *request.Invocation
		invocation.Arguments = append([]string{}, request.Invocation.Arguments...)
		clone.Invocation = &invocation
	}
	clone.Boundary = append([]string{}, request.Boundary...)
	clone.ExpectedReceipt.Boundary = append([]string{}, request.ExpectedReceipt.Boundary...)
	return &clone
}

func handoffInvocationFlagValue(invocation commands.PublicInvocation, names ...string) (string, bool, bool) {
	value := ""
	present := false
	for index, argument := range invocation.Arguments {
		matched := false
		for _, name := range names {
			if strings.EqualFold(argument, name) {
				matched = true
				break
			}
		}
		if !matched {
			continue
		}
		if present || index+1 >= len(invocation.Arguments) || strings.TrimSpace(invocation.Arguments[index+1]) == "" || strings.HasPrefix(strings.TrimSpace(invocation.Arguments[index+1]), "-") {
			return "", true, false
		}
		present = true
		value = strings.TrimSpace(invocation.Arguments[index+1])
	}
	return value, present, true
}

func validateHandoffStatusRefreshCommand(caseRoot, expectedLane, command string) (string, error) {
	command = strings.TrimSpace(command)
	invocation, err := commands.ParsePublicInvocation(command)
	if err != nil || invocation.Command != commands.Status {
		return "", fmt.Errorf("handoff current driver request requires a typed status refresh command")
	}
	target, present, valid := handoffInvocationFlagValue(invocation, "-Target", "--target")
	if !present || !valid || !refsf.SamePath(target, caseRoot) {
		return "", fmt.Errorf("handoff status refresh command must bind -Target to the attached project")
	}
	lane, present, valid := handoffInvocationFlagValue(invocation, "-Lane", "--lane")
	if !present || !valid || lane != strings.TrimSpace(expectedLane) {
		return "", fmt.Errorf("handoff status refresh command must bind the exact current driver request -Lane")
	}
	format, present, valid := handoffInvocationFlagValue(invocation, "-Format", "--format")
	if !present || !valid || !strings.EqualFold(format, "compact-json") {
		return "", fmt.Errorf("handoff status refresh command must use -Format compact-json")
	}
	if invocation.HasFlag("-Apply") || invocation.HasFlag("--apply") || invocation.HasFlag("-WhatIf") || invocation.HasFlag("--what-if") {
		return "", fmt.Errorf("handoff status refresh command must be read-only")
	}
	root, err := projectstate.Resolve(caseRoot)
	if err != nil {
		return "", err
	}
	entrypoint := "/steamai"
	if root.Legacy {
		entrypoint = "/rekit"
	}
	if !strings.HasPrefix(command, entrypoint+" ") {
		return "", fmt.Errorf("handoff status refresh command must use the selected project entrypoint: %s", entrypoint)
	}
	return command, nil
}

func (ctx handoffContext) withCurrentDriverRequest(queue mission.MissionCommanderActionQueue) mission.MissionCommanderActionQueue {
	return handoffActionQueueWithCurrentDriverRequest(queue, ctx.currentDriverRequest)
}

func handoffActionQueueWithCurrentDriverRequest(queue mission.MissionCommanderActionQueue, currentDriverRequest *mission.MissionCommanderDriverRequest) mission.MissionCommanderActionQueue {
	if currentDriverRequest == nil {
		return queue
	}
	request := cloneHandoffCurrentDriverRequest(currentDriverRequest)
	currentInvocation := cloneHandoffCurrentDriverRequest(request).Invocation
	current := mission.MissionCommanderNextActionItem{
		Lane:           request.Lane,
		Label:          request.Label,
		GateEventID:    request.GateEventID,
		ActionID:       request.ActionID,
		State:          request.State,
		Invocation:     currentInvocation,
		Command:        request.Command,
		Source:         request.Source,
		Blocked:        request.Blocked,
		RequiresReview: request.RequiresReview,
		Boundary:       append([]string{}, request.Boundary...),
	}
	queue.CurrentAction = &current
	queue.CurrentRunLoopStepID = request.RunLoopStepID
	queue.CurrentActionRunLoop = []mission.MissionCommanderRunLoopStep{
		{
			StepID:      request.RunLoopStepID,
			Order:       1,
			Actor:       request.Actor,
			Description: firstText(request.ExpectedReceipt.Description, "execute the final handoff driver request and consume its expected receipt"),
			Command:     request.Command,
			State:       request.State,
			Source:      request.Source,
			Boundary:    append([]string{}, request.ExpectedReceipt.Boundary...),
		},
	}
	queue.CurrentDriverRequest = request
	queue.Summary = mission.MissionCommanderActionQueueSummary(queue)
	return queue
}

func clonePackMemoryConsumptionHandoff(value *PackMemoryConsumptionHandoff) *PackMemoryConsumptionHandoff {
	if value == nil {
		return nil
	}
	clone := *value
	clone.MissionCommanderNextActions = mission.UniqueCommanderNextActions(value.MissionCommanderNextActions)
	clone.MissionCommanderActionQueue = mission.MissionCommanderActionQueueFor(clone.MissionCommanderNextActions)
	clone.Boundary = append([]string{}, value.Boundary...)
	return &clone
}

func cloneProjectNextBatchStarterPackage(pkg *ProjectNextBatchStarterPackage) *ProjectNextBatchStarterPackage {
	if pkg == nil {
		return nil
	}
	clone := *pkg
	clone.ValidationCommands = append([]string{}, pkg.ValidationCommands...)
	clone.ReleaseCadenceSteps = append([]string{}, pkg.ReleaseCadenceSteps...)
	clone.RecommendedStarterSteps = append([]string{}, pkg.RecommendedStarterSteps...)
	clone.Boundary = append([]string{}, pkg.Boundary...)
	clone.RunLoop = append([]mission.MissionCommanderRunLoopStep{}, pkg.RunLoop...)
	return &clone
}

func (ctx handoffContext) result(mutating, applied, confirm bool, writes []StartWrite) (HandoffResult, error) {
	var lane *Lane
	if ctx.lane != nil {
		copyLane := *ctx.lane
		lane = &copyLane
	}
	brief, err := ctx.missionBrief()
	if err != nil {
		brief = mission.Brief{Summary: "unavailable: " + err.Error()}
	}
	if lane != nil {
		brief = bindMissionBriefContinueCommands(brief, []Lane{*lane})
	} else if ctx.project {
		brief = bindMissionBriefContinueCommands(brief, ctx.currentLanes())
	}
	var executorAction *laneExecutorAction
	laneExecutorActions := []mission.LaneExecutorActionSnapshot{}
	executionEvidenceReview := []ExecutionEvidenceReviewItem{}
	reviewerWritebacks := []ReviewerWritebackItem{}
	reviewerDispatchIntakeHandoffs := []ReviewerDispatchIntakeHandoff{}
	reviewerPacketRetirementHandoffs := []ReviewerPacketRetirementHandoff{}
	authorizedGateAdapterHandoffs := []AuthorizedGateAdapterHandoff{}
	facts, factsErr := readHandoffFacts(ctx.inst.CaseRoot)
	if factsErr == nil {
		if lane != nil {
			reviewerWritebacks = ReviewerWritebackItems(facts, lane.ID)
			reviewerDispatchIntakeHandoffs, err = ReviewerDispatchIntakeHandoffs(ctx.inst.CaseRoot, facts, lane.ID)
			if err != nil {
				return HandoffResult{}, err
			}
			reviewerPacketRetirementHandoffs, err = ReviewerPacketRetirementHandoffs(ctx.inst.CaseRoot, lane.ID)
			if err != nil {
				return HandoffResult{}, err
			}
			authorizedGateAdapterHandoffs = AuthorizedGateAdapterHandoffsWithAcknowledgements(ctx.manifest.RepoRoot, ctx.inst.CaseRoot, ctx.manifest.Pack, facts.Requests, lane.ID, ExecutionEvidenceReviewAcknowledgedIDs(facts))
		} else if ctx.project {
			reviewerWritebacks = ReviewerWritebackItems(facts, "")
			reviewerDispatchIntakeHandoffs, err = ReviewerDispatchIntakeHandoffs(ctx.inst.CaseRoot, facts, "")
			if err != nil {
				return HandoffResult{}, err
			}
			reviewerPacketRetirementHandoffs, err = ReviewerPacketRetirementHandoffs(ctx.inst.CaseRoot, "")
			if err != nil {
				return HandoffResult{}, err
			}
			authorizedGateAdapterHandoffs = AuthorizedGateAdapterHandoffsWithAcknowledgements(ctx.manifest.RepoRoot, ctx.inst.CaseRoot, ctx.manifest.Pack, facts.Requests, "", ExecutionEvidenceReviewAcknowledgedIDs(facts))
		}
	}
	if lane != nil {
		action := bindHandoffLaneExecutorAction(
			ctx.executorAction(*lane),
			lane.ID,
			workstreamLabel(*lane),
		)
		executorAction = &action
		executionEvidenceReview = ctx.executionEvidenceReview(*lane)
	} else if ctx.project {
		laneExecutorActions = ctx.laneExecutorActions()
		executionEvidenceReview = ctx.projectExecutionEvidenceReview()
	}
	evidenceNeedsMainReview := ExecutionEvidenceReviewNeedsMainReview(executionEvidenceReview)
	includeEvidenceContinue := executorAction != nil && !executorAction.Blocked
	commanderActions := laneExecutorActions
	if lane != nil && executorAction != nil {
		commanderActions = []mission.LaneExecutorActionSnapshot{laneCommanderActionSnapshot(*lane, *executorAction)}
	}
	missionCommanderNext := mission.MissionCommanderNextActions(commanderActions, executionEvidenceReview, handoffHasBlockedAction(commanderActions))
	if factsErr == nil {
		missionCommanderNext = MissionCommanderNextActionsWithAuthorizedGateAdaptersAndAcknowledgements(missionCommanderNext, authorizedGateAdapterHandoffs, ExecutionEvidenceReviewAcknowledgedIDs(facts))
	} else {
		missionCommanderNext = MissionCommanderNextActionsWithAuthorizedGateAdapters(missionCommanderNext, authorizedGateAdapterHandoffs)
	}
	missionCommanderNext = MissionCommanderNextActionsWithReviewerDispatches(missionCommanderNext, reviewerDispatchIntakeHandoffs)
	if ctx.project && len(ctx.projectMissionCommanderNextActions) > 0 {
		missionCommanderNext = mission.UniqueCommanderNextActions(append(append([]mission.MissionCommanderNextActionItem{}, missionCommanderNext...), ctx.projectMissionCommanderNextActions...))
	}
	missionCommanderActionQueue := mission.MissionCommanderActionQueueFor(missionCommanderNext)
	if lane != nil {
		missionCommanderActionQueue = bindHandoffLaneActionQueue(
			missionCommanderActionQueue,
			lane.ID,
			workstreamLabel(*lane),
		)
	}
	missionCommanderActionQueue = ctx.withCurrentDriverRequest(missionCommanderActionQueue)
	runbookScope := handoffRunbookScope(ctx.project, ctx.canonicalSelector())
	handoffSelector := ctx.canonicalSelector()
	if lane != nil {
		handoffSelector = lane.ID
	}
	dailyRunbook := ctx.dailyMissionControlRunbook(runbookScope, handoffSelector, missionCommanderActionQueue)
	if lane != nil {
		bindHandoffLaneRunbook(
			dailyRunbook,
			lane.ID,
			workstreamLabel(*lane),
		)
	}
	latestDriverReceiptHandoff, err := latestDriverReceiptHandoffFor(ctx.inst.CaseRoot, lane)
	if err != nil {
		return HandoffResult{}, err
	}
	packagePath, err := ctx.replacementExecutorTakeoverPackageLatestRel()
	if err != nil {
		return HandoffResult{}, err
	}
	currentLoopOperator := ctx.currentLoopOperator
	if lane != nil {
		currentLoopOperator = currentLoopOperatorForLane(currentLoopOperator, lane.ID)
	}
	replacementExecutorTakeoverPackage, err := handoffReplacementExecutorTakeoverPackage(ctx.inst.CaseRoot, runbookScope, lane, missionCommanderActionQueue, dailyRunbook, latestDriverReceiptHandoff, packagePath, currentLoopOperator, ctx.currentDriverRequest != nil)
	if err != nil {
		return HandoffResult{}, err
	}
	var laneTakeoverPackage *LaneTakeoverPackage
	if lane != nil && executorAction != nil {
		laneTakeoverPackage, err = laneTakeoverPackageFor(ctx.inst.CaseRoot, *lane, *executorAction, missionCommanderActionQueue, false)
		if err != nil {
			return HandoffResult{}, err
		}
	}
	next := []string{"use /rekit as the Mission Commander entrypoint; JSON preview/apply is Go-owned by default"}
	next = append(next, ExecutionEvidenceReviewNextSteps(executionEvidenceReview, includeEvidenceContinue)...)
	if applied {
		if ctx.project {
			latestPath, err := stateRelPath(ctx.inst.CaseRoot, "handovers", "latest.md")
			if err != nil {
				return HandoffResult{}, err
			}
			next = append(next, "open "+latestPath+" in the case to continue")
		} else if executorAction != nil && !evidenceNeedsMainReview {
			next = append(next, executorAction.NextAgentActions...)
		}
	} else {
		next = append(next, "review this plan, then re-run handoff with -Apply to write case-local handoff files")
	}
	next = mission.UniqueStrings(next)
	return HandoffResult{
		SchemaVersion:                      1,
		Command:                            "handoff",
		CaseRoot:                           ctx.inst.CaseRoot,
		RepoRoot:                           ctx.manifest.RepoRoot,
		Pack:                               ctx.manifest.Pack,
		IsMutation:                         mutating,
		Applied:                            applied,
		RequiresConfirmation:               confirm,
		Selector:                           ctx.selector,
		Project:                            ctx.project,
		Lane:                               lane,
		LaneTakeoverPackage:                laneTakeoverPackage,
		LatestDriverReceiptHandoff:         latestDriverReceiptHandoff,
		MissionBrief:                       brief,
		ExecutorAction:                     executorAction,
		LaneExecutorActions:                laneExecutorActions,
		ExecutionEvidenceReview:            executionEvidenceReview,
		ExecutionEvidenceReviewSummary:     ExecutionEvidenceReviewSummaryFor(executionEvidenceReview, missionCommanderActionQueue),
		ReviewerWritebacks:                 reviewerWritebacks,
		ReviewerWritebackSummary:           ReviewerWritebackSummaryFor(reviewerWritebacks),
		ReviewerDispatchIntakeHandoffs:     reviewerDispatchIntakeHandoffs,
		ReviewerDispatchIntakeSummary:      ReviewerDispatchIntakeSummaryFor(reviewerDispatchIntakeHandoffs),
		ReviewerPacketRetirementHandoffs:   reviewerPacketRetirementHandoffs,
		ReviewerPacketRetirementSummary:    ReviewerPacketRetirementSummaryFor(reviewerPacketRetirementHandoffs),
		AuthorizedGateAdapterHandoffs:      authorizedGateAdapterHandoffs,
		MissionCommanderNextActions:        missionCommanderNext,
		MissionCommanderActionQueue:        missionCommanderActionQueue,
		DailyMissionControlRunbook:         dailyRunbook,
		ReplacementExecutorTakeoverPackage: replacementExecutorTakeoverPackage,
		CurrentLoopOperator:                currentLoopOperator,
		ProjectNextBatchStarterPackage:     cloneProjectNextBatchStarterPackage(ctx.projectNextBatchStarterPackage),
		PackMemoryConsumption:              clonePackMemoryConsumptionHandoff(ctx.projectPackMemoryConsumption),
		Writes:                             writes,
		BlockedActions:                     []string{"authority/confirmed writes", "heavy-tool execution without a valid current authorization decision", "continue auto-apply", "board/facts/lane creation"},
		NextSteps:                          next,
	}, nil
}

type handoffPublicationPlanIdentity struct {
	SchemaVersion int                               `json:"schemaVersion"`
	Publications  []handoffPublicationWriteIdentity `json:"publications"`
	Inputs        []handoffPublicationInputIdentity `json:"inputs"`
}

type handoffPublicationInputIdentity struct {
	Path   string `json:"path"`
	SHA256 string `json:"sha256"`
	Bytes  int    `json:"bytes"`
}

type handoffPublicationWriteIdentity struct {
	Path   string `json:"path"`
	SHA256 string `json:"sha256"`
	Bytes  []byte `json:"bytes"`
}

type handoffPublicationWrite struct {
	Path             string
	Bytes            []byte
	Role             string
	Generation       bool
	GenerationCommit bool
}

const (
	HandoffPublicationRoleResume          = "lane-resume"
	HandoffPublicationRoleCheckpoint      = "lane-checkpoint"
	HandoffPublicationRoleHandoffStamped  = "handoff-stamped"
	HandoffPublicationRoleHandoffLatest   = "handoff-latest"
	HandoffPublicationRoleTakeoverStamped = "takeover-stamped"
	HandoffPublicationRoleTakeoverLatest  = "takeover-latest"
)

type HandoffPublicationGenerationEntry struct {
	Path                  string `json:"path"`
	Role                  string `json:"role"`
	Bytes                 int    `json:"bytes"`
	CanonicalSHA256       string `json:"canonicalSha256"`
	PlanSHA256Occurrences int    `json:"planSha256Occurrences,omitempty"`
}

type HandoffPublicationGeneration struct {
	SchemaVersion         int                                 `json:"schemaVersion"`
	Scope                 string                              `json:"scope"`
	PublicationPlanSHA256 string                              `json:"publicationPlanSha256"`
	PublicationStamp      string                              `json:"publicationStamp"`
	Entries               []HandoffPublicationGenerationEntry `json:"entries"`
}

type handoffPublicationPlan struct {
	Preview           HandoffResult
	Writes            []StartWrite
	Publications      []handoffPublicationWrite
	Inputs            []handoffPublicationInputIdentity
	PublicationSHA256 string
}

func (ctx handoffContext) buildPublicationPlan() (handoffPublicationPlan, error) {
	initialInputs, err := ctx.publicationInputIdentity()
	if err != nil {
		return handoffPublicationPlan{}, err
	}
	resumeArtifacts, err := ctx.buildResumePublications()
	if err != nil {
		return handoffPublicationPlan{}, err
	}
	publicationContext := ctx
	publicationContext.publicationPlanSHA256 = handoffPublicationPlanSHA256Marker
	markdown, writes, err := publicationContext.renderPublication(false)
	if err != nil {
		return handoffPublicationPlan{}, err
	}
	stampPath, latestPath, err := ctx.projectHandoffPaths()
	if !ctx.project && ctx.lane != nil {
		stampPath, latestPath, err = ctx.laneHandoffPaths(ctx.lane.ID)
	}
	if err != nil {
		return handoffPublicationPlan{}, err
	}
	scope := "project"
	if !ctx.project {
		scope = "lane"
	}
	writes = append(writes,
		StartWrite{Path: relativePath(ctx.inst.CaseRoot, stampPath), Kind: "handoff", Action: "would-write-" + scope + "-handoff", TargetPath: stampPath},
		StartWrite{Path: relativePath(ctx.inst.CaseRoot, latestPath), Kind: "handoff", Action: "would-write-latest-" + scope + "-handoff", TargetPath: latestPath},
	)
	preview, err := ctx.result(false, false, true, writes)
	if err != nil {
		return handoffPublicationPlan{}, err
	}
	bindHandoffApplyRoute(&preview, handoffApplyCommand(ctx.inst.CaseRoot, ctx.manifest.Pack, ctx.canonicalSelector(), handoffPublicationPlanSHA256Marker, ctx.stamp))
	takeoverWrites, err := ctx.replacementExecutorTakeoverPackageArtifactWrites(false, preview.ReplacementExecutorTakeoverPackage)
	if err != nil {
		return handoffPublicationPlan{}, err
	}
	preview.Writes = append(preview.Writes, takeoverWrites...)

	publications := make([]handoffPublicationWrite, 0, len(resumeArtifacts)*2+4)
	for _, artifact := range resumeArtifacts {
		publications = append(publications,
			handoffPublicationWrite{Path: artifact.ResumePath, Bytes: artifact.ResumeBytes, Role: HandoffPublicationRoleResume},
			handoffPublicationWrite{Path: artifact.CheckpointPath, Bytes: artifact.CheckpointBytes, Role: HandoffPublicationRoleCheckpoint},
		)
	}
	markerMarkdown := []byte(ctx.bindHandoffPublicationMarkdown(markdown, handoffPublicationPlanSHA256Marker))
	publications = append(publications,
		handoffPublicationWrite{Path: stampPath, Bytes: markerMarkdown, Role: HandoffPublicationRoleHandoffStamped},
		handoffPublicationWrite{Path: latestPath, Bytes: markerMarkdown, Role: HandoffPublicationRoleHandoffLatest},
	)
	if preview.ReplacementExecutorTakeoverPackage != nil && preview.ReplacementExecutorTakeoverPackage.Ready {
		takeoverJSON, err := json.MarshalIndent(preview.ReplacementExecutorTakeoverPackage, "", "  ")
		if err != nil {
			return handoffPublicationPlan{}, err
		}
		takeoverJSON = append(takeoverJSON, '\n')
		takeoverStamp, takeoverLatest, err := ctx.replacementExecutorTakeoverPackagePaths()
		if err != nil {
			return handoffPublicationPlan{}, err
		}
		publications = append(publications,
			handoffPublicationWrite{Path: takeoverStamp, Bytes: takeoverJSON, Role: HandoffPublicationRoleTakeoverStamped},
			handoffPublicationWrite{Path: takeoverLatest, Bytes: takeoverJSON, Role: HandoffPublicationRoleTakeoverLatest},
		)
	}
	sort.Slice(publications, func(i, j int) bool { return publications[i].Path < publications[j].Path })
	generation, err := ctx.buildPublicationGeneration(publications)
	if err != nil {
		return handoffPublicationPlan{}, err
	}
	generationBytes, err := marshalHandoffPublicationGeneration(generation)
	if err != nil {
		return handoffPublicationPlan{}, err
	}
	generationStamp, generationLatest, err := ctx.publicationGenerationPaths()
	if err != nil {
		return handoffPublicationPlan{}, err
	}
	publications = append(publications,
		handoffPublicationWrite{Path: generationStamp, Bytes: generationBytes, Generation: true},
		handoffPublicationWrite{Path: generationLatest, Bytes: generationBytes, Generation: true, GenerationCommit: true},
	)
	preview.Writes = append(preview.Writes,
		StartWrite{Path: relativePath(ctx.inst.CaseRoot, generationStamp), Kind: "handoff-publication-generation", Action: "would-write-handoff-publication-generation", TargetPath: generationStamp},
		StartWrite{Path: relativePath(ctx.inst.CaseRoot, generationLatest), Kind: "handoff-publication-generation", Action: "would-write-latest-handoff-publication-generation", TargetPath: generationLatest},
	)
	inputs, err := ctx.publicationInputIdentity()
	if err != nil {
		return handoffPublicationPlan{}, err
	}
	if !handoffPublicationInputsEqual(initialInputs, inputs) {
		return handoffPublicationPlan{}, fmt.Errorf("handoff publication inputs changed while constructing plan; rerun handoff -WhatIf")
	}
	publicationSHA256, err := handoffPublicationPlanSHA256(publications, inputs)
	if err != nil {
		return handoffPublicationPlan{}, err
	}
	for idx := range publications {
		publications[idx].Bytes = bytes.ReplaceAll(
			publications[idx].Bytes,
			[]byte(handoffPublicationPlanSHA256Marker),
			[]byte(publicationSHA256),
		)
	}
	return handoffPublicationPlan{
		Preview:           preview,
		Writes:            preview.Writes,
		Publications:      publications,
		Inputs:            inputs,
		PublicationSHA256: publicationSHA256,
	}, nil
}

func CanonicalHandoffPublicationBytes(data []byte, publicationPlanSHA256 string, expectedOccurrences int) ([]byte, bool) {
	publicationPlanSHA256 = strings.TrimSpace(publicationPlanSHA256)
	if expectedOccurrences < 0 || bytes.Contains(data, []byte(handoffPublicationPlanSHA256Marker)) {
		return nil, false
	}
	if expectedOccurrences == 0 {
		return data, true
	}
	if publicationPlanSHA256 == "" || bytes.Count(data, []byte(publicationPlanSHA256)) != expectedOccurrences {
		return nil, false
	}
	return bytes.ReplaceAll(data, []byte(publicationPlanSHA256), []byte(handoffPublicationPlanSHA256Marker)), true
}

func marshalHandoffPublicationGeneration(generation HandoffPublicationGeneration) ([]byte, error) {
	var out bytes.Buffer
	encoder := json.NewEncoder(&out)
	encoder.SetEscapeHTML(false)
	encoder.SetIndent("", "  ")
	if err := encoder.Encode(generation); err != nil {
		return nil, err
	}
	return out.Bytes(), nil
}

func (ctx handoffContext) buildPublicationGeneration(publications []handoffPublicationWrite) (HandoffPublicationGeneration, error) {
	entries := make([]HandoffPublicationGenerationEntry, 0, len(publications))
	for _, publication := range publications {
		canonicalBytes := publication.Bytes
		sum := sha256.Sum256(canonicalBytes)
		entries = append(entries, HandoffPublicationGenerationEntry{
			Path:                  relativePath(ctx.inst.CaseRoot, publication.Path),
			Role:                  publication.Role,
			Bytes:                 len(canonicalBytes),
			CanonicalSHA256:       hex.EncodeToString(sum[:]),
			PlanSHA256Occurrences: bytes.Count(canonicalBytes, []byte(handoffPublicationPlanSHA256Marker)),
		})
	}
	scope := "project"
	if !ctx.project && ctx.lane != nil {
		scope = "lane:" + ctx.lane.ID
	}
	return HandoffPublicationGeneration{
		SchemaVersion:         1,
		Scope:                 scope,
		PublicationPlanSHA256: handoffPublicationPlanSHA256Marker,
		PublicationStamp:      ctx.stamp,
		Entries:               entries,
	}, nil
}

func (ctx handoffContext) publicationGenerationPaths() (string, string, error) {
	name := "project"
	if !ctx.project && ctx.lane != nil {
		if err := validateLaneIDSegment(ctx.lane.ID); err != nil {
			return "", "", err
		}
		name = ctx.lane.ID
	}
	stampPath, err := refsf.SafeJoin(ctx.handovers, name+"-"+ctx.stamp+"-generation.json")
	if err != nil {
		return "", "", err
	}
	latestPath, err := refsf.SafeJoin(ctx.handovers, name+"-latest-generation.json")
	if err != nil {
		return "", "", err
	}
	return stampPath, latestPath, nil
}

func ParseHandoffPublicationStamp(value string) (time.Time, error) {
	value = strings.TrimSpace(value)
	if !regexp.MustCompile(`^[0-9]{8}-[0-9]{9}$`).MatchString(value) {
		return time.Time{}, fmt.Errorf("invalid handoff publication stamp")
	}
	return time.Parse("20060102-150405.000", value[:15]+"."+value[15:])
}

func (ctx handoffContext) publicationTime() string {
	value, err := ParseHandoffPublicationStamp(ctx.stamp)
	if err != nil {
		return ""
	}
	return value.UTC().Format(time.RFC3339Nano)
}

func (ctx handoffContext) buildResumePublications() ([]laneResumePublication, error) {
	lanes := []Lane{}
	if ctx.project {
		lanes = ctx.currentLanes()
	} else if ctx.lane != nil {
		lanes = []Lane{*ctx.lane}
	}
	artifacts := make([]laneResumePublication, 0, len(lanes))
	for _, lane := range lanes {
		artifact, err := buildLaneResumePublicationWithOptions(
			ctx.inst.CaseRoot,
			ctx.manifest,
			lane,
			laneResumePublicationOptions{CurrentDriverRequest: ctx.currentDriverRequest},
			ctx.publicationTime(),
		)
		if err != nil {
			return nil, err
		}
		artifacts = append(artifacts, artifact)
	}
	return artifacts, nil
}

func immutableHandoffPublication(publication handoffPublicationWrite) bool {
	return publication.Role == HandoffPublicationRoleHandoffStamped ||
		publication.Role == HandoffPublicationRoleTakeoverStamped ||
		(publication.Generation && !publication.GenerationCommit)
}

func validateImmutableHandoffPublications(
	root *refsf.AnchoredRoot,
	caseRoot string,
	publications []handoffPublicationWrite,
) error {
	for _, publication := range publications {
		if !immutableHandoffPublication(publication) {
			continue
		}
		rel := relativePath(caseRoot, publication.Path)
		info, err := root.Lstat(rel)
		if os.IsNotExist(err) {
			continue
		}
		if err != nil {
			return fmt.Errorf(
				"inspect immutable handoff publication %s: %w",
				rel,
				err,
			)
		}
		if !info.Mode().IsRegular() ||
			info.Mode()&os.ModeSymlink != 0 ||
			info.Size() != int64(len(publication.Bytes)) {
			return fmt.Errorf(
				"immutable handoff publication conflicts with existing stamped identity: %s",
				rel,
			)
		}
		current, _, err := root.ReadStableFile(
			rel,
			int64(len(publication.Bytes)),
		)
		if err != nil {
			return fmt.Errorf(
				"validate immutable handoff publication %s: %w",
				rel,
				err,
			)
		}
		if !bytes.Equal(current, publication.Bytes) {
			return fmt.Errorf(
				"immutable handoff publication conflicts with existing stamped identity: %s",
				rel,
			)
		}
	}
	return nil
}

func (ctx handoffContext) writeHandoffPublication(
	publication handoffPublicationWrite,
) error {
	if !immutableHandoffPublication(publication) {
		return writePublicationBytes(publication.Path, publication.Bytes)
	}
	rel := relativePath(ctx.inst.CaseRoot, publication.Path)
	if _, err := refsf.WriteAtomicNoReplaceRegularFileAnchoredMode(
		ctx.inst.CaseRoot,
		rel,
		"immutable stamped handoff publication",
		publication.Bytes,
		0o644,
	); err != nil {
		return fmt.Errorf("publish immutable handoff publication %s: %w", rel, err)
	}
	return nil
}

func (ctx handoffContext) publishPublicationPlan(plan handoffPublicationPlan) (resultErr error) {
	currentInputs, err := ctx.publicationInputIdentity()
	if err != nil {
		return err
	}
	if !handoffPublicationInputsEqual(plan.Inputs, currentInputs) {
		return fmt.Errorf("handoff publication inputs changed after plan construction; rerun handoff -WhatIf")
	}
	if handoffApplyBeforeWriteHook != nil {
		handoffApplyBeforeWriteHook()
		currentInputs, err = ctx.publicationInputIdentity()
		if err != nil {
			return err
		}
		if !handoffPublicationInputsEqual(plan.Inputs, currentInputs) {
			return fmt.Errorf("handoff publication inputs changed before first write; rerun handoff -WhatIf")
		}
	}
	publicationRoot, err := refsf.OpenAnchoredRoot(ctx.inst.CaseRoot)
	if err != nil {
		return err
	}
	defer func() {
		resultErr = errors.Join(resultErr, publicationRoot.Close())
	}()
	if err := validateImmutableHandoffPublications(
		publicationRoot,
		ctx.inst.CaseRoot,
		plan.Publications,
	); err != nil {
		return err
	}
	for phase := range 3 {
		for _, publication := range plan.Publications {
			publicationPhase := 0
			if publication.Generation {
				publicationPhase = 1
			}
			if publication.GenerationCommit {
				publicationPhase = 2
			}
			if publicationPhase != phase {
				continue
			}
			if err := ctx.writeHandoffPublication(publication); err != nil {
				return err
			}
			if handoffApplyAfterWriteHook != nil {
				if err := handoffApplyAfterWriteHook(publication.Path, publication.Generation); err != nil {
					return err
				}
			}
		}
	}
	return nil
}

func writePublicationBytes(path string, data []byte) (resultErr error) {
	dir := filepath.Dir(path)
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return err
	}
	temp, err := os.CreateTemp(dir, ".rekit-handoff-*.tmp")
	if err != nil {
		return err
	}
	tempPath := temp.Name()
	defer func() {
		if tempPath == "" {
			return
		}
		if closeErr := temp.Close(); closeErr != nil && resultErr == nil {
			resultErr = closeErr
		}
		_ = os.Remove(tempPath)
	}()
	if err := temp.Chmod(0o644); err != nil {
		return err
	}
	if _, err := temp.Write(data); err != nil {
		return err
	}
	if err := temp.Sync(); err != nil {
		return err
	}
	if err := temp.Close(); err != nil {
		return err
	}
	if err := os.Rename(tempPath, path); err != nil {
		return err
	}
	tempPath = ""
	return syncHandoffPublicationDirectory(dir)
}

func handoffPublicationInputsEqual(left, right []handoffPublicationInputIdentity) bool {
	leftJSON, leftErr := json.Marshal(left)
	rightJSON, rightErr := json.Marshal(right)
	return leftErr == nil && rightErr == nil && bytes.Equal(leftJSON, rightJSON)
}

func appliedHandoffWrites(writes []StartWrite) []StartWrite {
	out := append([]StartWrite{}, writes...)
	for idx := range out {
		out[idx].Action = strings.TrimPrefix(out[idx].Action, "would-")
	}
	return out
}

func appliedHandoffNextSteps(result HandoffResult) ([]string, error) {
	includeEvidenceContinue := result.ExecutorAction != nil && !result.ExecutorAction.Blocked
	next := ExecutionEvidenceReviewNextSteps(result.ExecutionEvidenceReview, includeEvidenceContinue)
	if result.Project {
		latestPath, err := stateRelPath(result.CaseRoot, "handovers", "latest.md")
		if err != nil {
			return nil, err
		}
		return append(next, "open "+latestPath+" in the case to continue"), nil
	}
	if result.ExecutorAction != nil && !ExecutionEvidenceReviewNeedsMainReview(result.ExecutionEvidenceReview) {
		return append(next, result.ExecutorAction.NextAgentActions...), nil
	}
	return next, nil
}

func handoffPublicationPlanSHA256(publications []handoffPublicationWrite, inputs []handoffPublicationInputIdentity) (string, error) {
	identityWrites := make([]handoffPublicationWriteIdentity, 0, len(publications))
	for _, publication := range publications {
		sum := sha256.Sum256(publication.Bytes)
		identityWrites = append(identityWrites, handoffPublicationWriteIdentity{
			Path:   filepath.Clean(publication.Path),
			SHA256: hex.EncodeToString(sum[:]),
			Bytes:  publication.Bytes,
		})
	}
	identity := handoffPublicationPlanIdentity{
		SchemaVersion: 2,
		Publications:  identityWrites,
		Inputs:        inputs,
	}
	data, err := json.Marshal(identity)
	if err != nil {
		return "", err
	}
	sum := sha256.Sum256(data)
	return hex.EncodeToString(sum[:]), nil
}

func (ctx handoffContext) renderPublication(apply bool) (string, []StartWrite, error) {
	if ctx.project {
		return ctx.renderProject(apply)
	}
	return ctx.renderLane(*ctx.lane, apply)
}

func (ctx handoffContext) publicationInputIdentity() ([]handoffPublicationInputIdentity, error) {
	stateRoot, err := projectstate.Resolve(ctx.inst.CaseRoot)
	if err != nil {
		return nil, err
	}
	root := stateRoot.Path
	inputs := []handoffPublicationInputIdentity{}
	err = filepath.Walk(root, func(path string, info os.FileInfo, walkErr error) error {
		if walkErr != nil {
			return walkErr
		}
		rel, err := filepath.Rel(root, path)
		if err != nil {
			return err
		}
		rel = filepath.ToSlash(rel)
		if info.IsDir() {
			if rel == "handovers" || rel == "locks" {
				return filepath.SkipDir
			}
			return nil
		}
		if strings.HasSuffix(rel, "/prompts/RESUME.md") || strings.HasSuffix(rel, "/checkpoints/latest.json") {
			return nil
		}
		if !info.Mode().IsRegular() {
			return fmt.Errorf("handoff publication input is not a regular file: %s", rel)
		}
		data, err := readStableHandoffPublicationInput(path, info)
		if err != nil {
			return err
		}
		sum := sha256.Sum256(data)
		inputs = append(inputs, handoffPublicationInputIdentity{Path: rel, SHA256: hex.EncodeToString(sum[:]), Bytes: len(data)})
		return nil
	})
	if err != nil {
		return nil, err
	}
	sort.Slice(inputs, func(i, j int) bool { return inputs[i].Path < inputs[j].Path })
	return inputs, nil
}

func readStableHandoffPublicationInput(path string, before os.FileInfo) ([]byte, error) {
	if before == nil || !before.Mode().IsRegular() {
		return nil, fmt.Errorf("handoff publication input must be a regular file: %s", path)
	}
	file, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer file.Close()
	opened, err := file.Stat()
	if err != nil {
		return nil, err
	}
	if !opened.Mode().IsRegular() || !os.SameFile(before, opened) {
		return nil, fmt.Errorf("handoff publication input changed while opening: %s", path)
	}
	data, err := io.ReadAll(file)
	if err != nil {
		return nil, err
	}
	after, err := os.Lstat(path)
	if err != nil {
		return nil, err
	}
	if !after.Mode().IsRegular() || !os.SameFile(opened, after) || after.Size() != int64(len(data)) {
		return nil, fmt.Errorf("handoff publication input changed while reading: %s", path)
	}
	return data, nil
}

func (ctx handoffContext) bindHandoffPublicationMarkdown(markdown, publicationPlanSHA256 string) string {
	generic := handoffApplyCommand(ctx.inst.CaseRoot, ctx.manifest.Pack, ctx.canonicalSelector())
	bound := handoffApplyCommand(ctx.inst.CaseRoot, ctx.manifest.Pack, ctx.canonicalSelector(), publicationPlanSHA256, ctx.stamp)
	markdown = strings.ReplaceAll(markdown, generic, bound)
	if !strings.Contains(markdown, bound) {
		markdown += "\n## Handoff publication plan\n\n"
		markdown += "- sha256: " + strings.TrimSpace(publicationPlanSHA256) + "\n"
		markdown += "- exact apply: `" + bound + "`\n"
	}
	return markdown
}

func bindHandoffApplyRoute(result *HandoffResult, command string) {
	if result == nil || strings.TrimSpace(command) == "" || result.DailyMissionControlRunbook == nil {
		return
	}
	runbook := result.DailyMissionControlRunbook
	runbook.HandoffApplyCommand = command
	for idx := range runbook.RunLoop {
		if runbook.RunLoop[idx].StepID == "write-handoff-for-takeover" {
			runbook.RunLoop[idx].Command = command
			runbook.RunLoop[idx].CommandExecutable = true
		}
	}
	runbook.HandoffApplyDriverRequest = dailyMissionControlHandoffDriverRequest(runbook, "write-handoff-for-takeover", command, true, true)
}

func handoffRunbookScope(project bool, selector string) string {
	selector = strings.TrimSpace(selector)
	if project || selector == "" {
		return "project"
	}
	return "lane:" + selector
}

func handoffPreviewCommand(caseRoot, pack, selector string) string {
	return handoffCommand(caseRoot, pack, selector, false, "", "")
}

func handoffApplyCommand(caseRoot, pack, selector string, expected ...string) string {
	sha256 := ""
	stamp := ""
	if len(expected) > 0 {
		sha256 = strings.TrimSpace(expected[0])
	}
	if len(expected) > 1 {
		stamp = strings.TrimSpace(expected[1])
	}
	return handoffCommandForArgs(handoffApplyArgs(caseRoot, pack, selector, sha256, stamp))
}

func handoffApplyArgs(caseRoot, pack, selector, expectedSHA256, publicationStamp string) []string {
	return handoffArgs(caseRoot, pack, selector, true, expectedSHA256, publicationStamp)
}

func handoffCommand(caseRoot, pack, selector string, apply bool, expectedSHA256, publicationStamp string) string {
	return handoffCommandForArgs(handoffArgs(caseRoot, pack, selector, apply, expectedSHA256, publicationStamp))
}

func handoffArgs(caseRoot, pack, selector string, apply bool, expectedSHA256, publicationStamp string) []string {
	args := []string{"-Command", "handoff", "-Target", caseRoot, "-Pack", pack}
	if apply {
		args = append(args, "-Apply")
		if expectedSHA256 != "" {
			args = append(args, "-ExpectedHandoffPlanSha256", expectedSHA256)
		}
		if publicationStamp != "" {
			args = append(args, "-HandoffPublicationStamp", publicationStamp)
		}
	} else {
		args = append(args, "-WhatIf")
	}
	selector = strings.TrimSpace(selector)
	if selector != "" {
		args = append(args, "-Lane", selector)
	}
	return append(args, "-Format", "json")
}

func handoffCommandForArgs(args []string) string {
	parts := append([]string{"/rekit", "handoff"}, args[2:]...)
	for i, part := range parts {
		parts[i] = quoteCommandArg(part)
	}
	return strings.Join(parts, " ")
}

func bindHandoffLaneExecutorAction(action laneExecutorAction, laneID, laneLabel string) laneExecutorAction {
	action.ResumeCommand = handoffLaneCommand(action.ResumeCommand, laneID, laneLabel)
	action.HandoffCommand = handoffLaneCommand(action.HandoffCommand, laneID, laneLabel)
	for index := range action.NextAgentActions {
		action.NextAgentActions[index] = handoffLaneCommand(
			action.NextAgentActions[index],
			laneID,
			laneLabel,
		)
	}
	action.MissionCommanderAction.PrimaryCommand = handoffLaneCommand(
		action.MissionCommanderAction.PrimaryCommand,
		laneID,
		laneLabel,
	)
	for index := range action.MissionCommanderAction.FollowUpCommands {
		action.MissionCommanderAction.FollowUpCommands[index] = handoffLaneCommand(
			action.MissionCommanderAction.FollowUpCommands[index],
			laneID,
			laneLabel,
		)
	}
	return action
}

func bindHandoffLaneRunbook(runbook *DailyMissionControlRunbook, laneID, laneLabel string) {
	if runbook == nil {
		return
	}
	runbook.RefreshStatusCommand = handoffLaneStatusCommand(runbook.RefreshStatusCommand, laneID)
	bindHandoffLaneDriverRequest(runbook.CurrentDriverRequest, laneID, laneLabel)
	bindHandoffLaneDriverRequest(runbook.HandoffPreviewDriverRequest, laneID, laneLabel)
	bindHandoffLaneDriverRequest(runbook.HandoffApplyDriverRequest, laneID, laneLabel)
	if runbook.CurrentDriverRequest != nil {
		runbook.CurrentCommand = runbook.CurrentDriverRequest.Command
	}
	for index := range runbook.RunLoop {
		step := &runbook.RunLoop[index]
		switch step.StepID {
		case "inspect-status", "refresh-after-driver":
			step.Command = runbook.RefreshStatusCommand
		case "consume-current-driver-request":
			if runbook.CurrentDriverRequest != nil {
				step.Command = runbook.CurrentDriverRequest.Command
			}
		}
	}
}

func bindHandoffLaneDriverRequest(request *mission.MissionCommanderDriverRequest, laneID, laneLabel string) {
	if request == nil {
		return
	}
	request.Lane = laneID
	request.Command = handoffLaneCommand(request.Command, laneID, laneLabel)
	request.ExpectedReceipt.Command = handoffLaneCommand(request.ExpectedReceipt.Command, laneID, laneLabel)
	request.ExpectedReceipt.RefreshStatusCommand = handoffLaneStatusCommand(request.ExpectedReceipt.RefreshStatusCommand, laneID)
	if request.Command != "" {
		invocation, err := commands.ParsePublicInvocation(request.Command)
		if err == nil {
			request.Invocation = &invocation
		}
	}
}

func handoffLaneStatusCommand(command, laneID string) string {
	command = strings.TrimSpace(command)
	laneID = strings.TrimSpace(laneID)
	if command == "" || laneID == "" {
		return command
	}
	invocation, err := commands.ParsePublicInvocation(command)
	if err != nil || invocation.Command != commands.Status {
		return command
	}
	if invocation.HasFlag("-Lane") || invocation.HasFlag("--lane") {
		return command
	}
	invocation.Arguments = append(invocation.Arguments, "-Lane", laneID)
	bound, err := commands.NewPublicInvocation(invocation.Command, invocation.Arguments...)
	if err != nil {
		return ""
	}
	rendered, err := bound.Render()
	if err != nil {
		return ""
	}
	return rendered
}

func bindHandoffLaneActionQueue(queue mission.MissionCommanderActionQueue, laneID, laneLabel string) mission.MissionCommanderActionQueue {
	bind := func(item mission.MissionCommanderNextActionItem) mission.MissionCommanderNextActionItem {
		item.Command = handoffLaneCommand(item.Command, laneID, laneLabel)
		if item.Command != "" {
			invocation, err := commands.ParsePublicInvocation(item.Command)
			if err == nil {
				item.Invocation = &invocation
			}
		}
		return item
	}
	items := make([]mission.MissionCommanderNextActionItem, 0, len(queue.UnblockedActions)+len(queue.BlockedActions))
	for _, item := range queue.UnblockedActions {
		items = append(items, bind(item))
	}
	for _, item := range queue.BlockedActions {
		items = append(items, bind(item))
	}
	return mission.MissionCommanderActionQueueFor(items)
}

func handoffLaneCommand(command, laneID, laneLabel string) string {
	command = strings.TrimSpace(command)
	laneID = strings.TrimSpace(laneID)
	laneLabel = strings.TrimSpace(laneLabel)
	if command == "" || laneID == "" {
		return command
	}
	invocation, err := commands.ParsePublicInvocation(command)
	if err != nil {
		return command
	}
	if invocation.Command != commands.Continue && invocation.Command != commands.Complete && invocation.Command != commands.Reconcile && invocation.Command != commands.Handoff {
		return command
	}
	arguments := append([]string{}, invocation.Arguments...)
	for index, argument := range arguments {
		if !strings.EqualFold(argument, "-Lane") && !strings.EqualFold(argument, "--lane") {
			continue
		}
		if index+1 >= len(arguments) || strings.TrimSpace(arguments[index+1]) != laneID {
			return ""
		}
		return command
	}
	selector := laneLabel
	if selector == "" {
		selector = workstreamLabel(Lane{ID: laneID})
	}
	for index, argument := range arguments {
		if strings.HasPrefix(argument, "-") {
			continue
		}
		if index == 0 && selectedLaneSelectorMatches(argument, laneID, selector) {
			arguments = append(arguments[:index], arguments[index+1:]...)
		}
		break
	}
	insertAt := len(arguments)
	for index, argument := range arguments {
		if strings.EqualFold(argument, "-WhatIf") || strings.EqualFold(argument, "--what-if") || strings.EqualFold(argument, "-Apply") || strings.EqualFold(argument, "--apply") {
			insertAt = index
			break
		}
	}
	arguments = append(arguments[:insertAt], append([]string{"-Lane", laneID}, arguments[insertAt:]...)...)
	bound, err := commands.NewPublicInvocation(invocation.Command, arguments...)
	if err != nil {
		return ""
	}
	rendered, err := bound.Render()
	if err != nil {
		return ""
	}
	return rendered
}

func selectedLaneSelectorMatches(value, laneID, label string) bool {
	value = strings.TrimSpace(value)
	return value == strings.TrimSpace(laneID) || value == strings.TrimSpace(label)
}

func handoffReplacementExecutorTakeoverPackage(caseRoot, scope string, lane *Lane, queue mission.MissionCommanderActionQueue, runbook *DailyMissionControlRunbook, latestReceipt *LatestDriverReceiptHandoff, packagePath string, operator *mission.CurrentLoopOperatorPackage, preferQueueCurrentDriverRequest bool) (*mission.ReplacementExecutorTakeoverPackage, error) {
	refresh := dailyMissionControlStatusCommand(caseRoot)
	if runbook != nil && strings.TrimSpace(runbook.RefreshStatusCommand) != "" {
		refresh = runbook.RefreshStatusCommand
	}
	if lane != nil {
		refresh = handoffLaneStatusCommand(refresh, lane.ID)
	}
	packagePath = strings.TrimSpace(packagePath)
	if packagePath == "" {
		packagePath = "replacementExecutorTakeoverPackage"
	}
	request := queue.CurrentDriverRequest
	if !preferQueueCurrentDriverRequest && operator != nil && operator.ExternalSessionJob != nil && operator.ExternalSessionJob.Dispatcher != nil && operator.SelectedDriverRequest != nil {
		switch operator.ExternalSessionJob.Dispatcher.State {
		case "attempt-publication-pending", "queued", "claimed", "launch-failed":
			request = operator.SelectedDriverRequest
		}
	}
	targetDocuments, err := handoffReplacementExecutorTakeoverTargetDocuments(caseRoot, lane, request, latestReceipt, packagePath)
	if err != nil {
		return nil, err
	}
	return mission.ReplacementExecutorTakeoverPackageFor(request, mission.ReplacementExecutorTakeoverOptions{
		Focus:                "durable-handoff-current-action",
		Scope:                scope,
		RefreshStatusCommand: refresh,
		PackagePath:          packagePath,
		TargetDocuments:      targetDocuments,
		CurrentLoopOperator:  operator,
	}), nil
}

func handoffReplacementExecutorTakeoverTargetDocuments(caseRoot string, lane *Lane, request *mission.MissionCommanderDriverRequest, latestReceipt *LatestDriverReceiptHandoff, packagePath string) ([]string, error) {
	docs := []string{strings.TrimSpace(packagePath), "replacementExecutorTakeoverPackage", "missionCommanderActionQueue.currentDriverRequest", "dailyMissionControlRunbook.currentDriverRequest"}
	handoffName := "latest.md"
	if lane != nil {
		handoffName = lane.ID + "-latest.md"
	}
	handoffPath, err := stateRelPath(caseRoot, "handovers", handoffName)
	if err != nil {
		return nil, err
	}
	if lane == nil {
		docs = append(docs, handoffPath)
	} else {
		docs = append(docs,
			handoffPath,
			relJoin(lane.LaneRoot, "prompts", "RESUME.md"),
			relJoin(lane.LaneRoot, "checkpoints", "latest.json"),
		)
	}
	if latestReceipt != nil && latestReceipt.Ready {
		docs = append(docs, latestReceipt.TargetDocuments...)
	}
	if request != nil && request.RequiresReview {
		docs = append(docs, "replacementExecutorTakeoverPackage.currentDriverRequest.expectedReceipt")
	}
	return mission.UniqueStrings(docs), nil
}

func (ctx handoffContext) missionBrief() (mission.Brief, error) {
	facts, err := readHandoffFacts(ctx.inst.CaseRoot)
	if err != nil {
		return mission.Brief{}, err
	}
	if ctx.project {
		return projectMissionBrief(ctx.board.Lanes, facts), nil
	}
	if ctx.lane == nil {
		return mission.Brief{}, nil
	}
	return laneMissionBrief(*ctx.lane, facts), nil
}

func (ctx handoffContext) executorAction(lane Lane) laneExecutorAction {
	facts, err := readHandoffFacts(ctx.inst.CaseRoot)
	if err != nil {
		brief := mission.Brief{Summary: "unavailable: " + err.Error()}
		return laneExecutorActionFor(lane, mission.Facts{}, brief)
	}
	brief := laneMissionBrief(lane, facts)
	return laneExecutorActionFor(lane, facts.Facts, brief)
}

func (ctx handoffContext) laneExecutorActions() []mission.LaneExecutorActionSnapshot {
	facts, err := readHandoffFacts(ctx.inst.CaseRoot)
	if err != nil {
		return nil
	}
	lanes := ctx.currentLanes()
	brief := projectMissionBrief(ctx.board.Lanes, facts)
	brief = bindMissionBriefContinueCommands(brief, lanes)
	items := mission.LaneExecutorActionSnapshots(ctx.board.Lanes, facts.Facts, brief)
	for idx := range items {
		lane := Lane{
			ID:                 items[idx].Lane,
			Status:             items[idx].Status,
			Workspace:          items[idx].Workspace,
			CurrentExecutor:    items[idx].CurrentExecutor,
			ExecutorGeneration: items[idx].ExecutorGeneration,
		}
		for _, current := range lanes {
			if current.ID == items[idx].Lane {
				lane = current
				break
			}
		}
		items[idx].ExecutorAction = bindLaneContinueCommands(items[idx].ExecutorAction, lane)
	}
	return items
}

func (ctx handoffContext) currentLanes() []Lane {
	lanes := make([]Lane, 0, len(ctx.board.Lanes))
	for _, row := range ctx.board.Lanes {
		lane, err := readLaneByID(ctx.inst.CaseRoot, row.ID)
		if err != nil {
			lane = Lane{
				ID:                 row.ID,
				Status:             row.Status,
				Workspace:          row.Workspace,
				CurrentExecutor:    row.CurrentExecutor,
				ExecutorGeneration: row.ExecutorGeneration,
			}
		}
		lanes = append(lanes, lane)
	}
	return lanes
}

func laneCommanderActionSnapshot(lane Lane, action laneExecutorAction) mission.LaneExecutorActionSnapshot {
	action = bindLaneContinueCommands(action, lane)
	return mission.LaneExecutorActionSnapshot{
		Lane:               lane.ID,
		Label:              workstreamLabel(lane),
		Status:             lane.Status,
		Workspace:          lane.Workspace,
		CurrentExecutor:    lane.CurrentExecutor,
		ExecutorGeneration: lane.ExecutorGeneration,
		LastTakeoverAt:     lane.LastTakeoverAt,
		LastTakeoverBy:     lane.LastTakeoverBy,
		LastTakeoverReason: lane.LastTakeoverReason,
		ExecutorAction:     action,
	}
}

func bindLaneContinueCommands(action laneExecutorAction, lane Lane) laneExecutorAction {
	action.ResumeCommand = bindContinueCommand(action.ResumeCommand, lane)
	for idx := range action.NextAgentActions {
		action.NextAgentActions[idx] = bindContinueCommand(action.NextAgentActions[idx], lane)
	}
	action.MissionCommanderAction = bindMissionCommanderActionContinueCommands(action.MissionCommanderAction, lane)
	return action
}

func bindMissionCommanderActionContinueCommands(action mission.MissionCommanderAction, lane Lane) mission.MissionCommanderAction {
	action.PrimaryCommand = bindContinueCommand(action.PrimaryCommand, lane)
	for idx := range action.FollowUpCommands {
		action.FollowUpCommands[idx] = bindContinueCommand(action.FollowUpCommands[idx], lane)
	}
	return action
}

func bindExecutionEvidenceReviewContinueCommands(items []ExecutionEvidenceReviewItem, laneFor func(string) (Lane, bool)) []ExecutionEvidenceReviewItem {
	for idx := range items {
		lane, ok := laneFor(items[idx].HandoffCommand)
		if !ok {
			lane, ok = laneFor(items[idx].MissionCommanderAction.PrimaryCommand)
		}
		if !ok {
			continue
		}
		items[idx].MissionCommanderAction = bindMissionCommanderActionContinueCommands(items[idx].MissionCommanderAction, lane)
		items[idx].FollowThrough = mission.ExecutionEvidenceReviewFollowThrough(items[idx])
		items[idx].ReviewRunbookSteps = mission.ExecutionEvidenceReviewRunbookSteps(items[idx], true)
	}
	return items
}

func bindContinueCommand(command string, lane Lane) string {
	executor := strings.TrimSpace(lane.CurrentExecutor)
	if executor == "" {
		return command
	}
	label := workstreamLabel(lane)
	prefix := "/rekit continue " + label
	if command != prefix && !strings.HasPrefix(command, prefix+" ") {
		return command
	}
	suffix := continueOwnerBindingPattern.ReplaceAllString(strings.TrimPrefix(command, prefix), "")
	return prefix + " -Executor " + quoteCommandArg(executor) + " -ExpectedExecutorGeneration " + fmt.Sprintf("%d", lane.ExecutorGeneration) + suffix
}

func CurrentLaneAuthority(caseRoot, laneID string) (mission.BoardLane, error) {
	lane, err := readLaneByID(caseRoot, laneID)
	if err == nil {
		return mission.BoardLane{
			ID:                 lane.ID,
			Status:             lane.Status,
			Authority:          lane.Authority,
			Workspace:          lane.Workspace,
			CurrentExecutor:    lane.CurrentExecutor,
			ExecutorGeneration: lane.ExecutorGeneration,
		}, nil
	}
	board, boardErr := mission.ReadBoard(caseRoot)
	if boardErr != nil {
		return mission.BoardLane{}, err
	}
	for _, lane := range board.Lanes {
		if lane.ID == laneID {
			return lane, nil
		}
	}
	return mission.BoardLane{ID: laneID, Authority: laneID == "main"}, nil
}

func BindLaneAuthorityContinueCommand(command string, lane mission.BoardLane) string {
	return bindContinueCommand(command, workstreamLanesFromBoard([]mission.BoardLane{lane})[0])
}

func BindLaneAuthorityContinueCommands(action mission.ExecutorAction, lane mission.BoardLane) mission.ExecutorAction {
	return bindLaneContinueCommands(action, workstreamLanesFromBoard([]mission.BoardLane{lane})[0])
}

func BindMissionCommanderNextActionAuthorityContinueCommands(items []mission.MissionCommanderNextActionItem, lane mission.BoardLane) ([]mission.MissionCommanderNextActionItem, error) {
	currentLane := workstreamLanesFromBoard([]mission.BoardLane{lane})[0]
	out := append([]mission.MissionCommanderNextActionItem{}, items...)
	for idx := range out {
		originalCommand := strings.TrimSpace(out[idx].Command)
		boundCommand := bindContinueCommand(originalCommand, currentLane)
		if boundCommand == originalCommand {
			continue
		}

		originalInvocation, err := commands.ParsePublicInvocation(originalCommand)
		if err != nil {
			return nil, fmt.Errorf("bind Mission Commander continue action to current lane owner: %w", err)
		}
		if out[idx].Invocation != nil {
			if err := out[idx].Invocation.Validate(); err != nil {
				return nil, fmt.Errorf("bind Mission Commander continue action to current lane owner: %w", err)
			}
			if !out[idx].Invocation.Equivalent(originalInvocation) {
				return nil, fmt.Errorf("bind Mission Commander continue action to current lane owner: typed invocation does not match the command projection")
			}
		}

		boundInvocation, err := commands.ParsePublicInvocation(boundCommand)
		if err != nil {
			return nil, fmt.Errorf("bind Mission Commander continue action to current lane owner: %w", err)
		}
		out[idx].Command = boundCommand
		out[idx].Invocation = &boundInvocation
	}
	return out, nil
}

func BindMissionBriefAuthorityContinueCommands(brief mission.Brief, lanes []mission.BoardLane) mission.Brief {
	return bindMissionBriefContinueCommands(brief, workstreamLanesFromBoard(lanes))
}

func BindExecutionEvidenceReviewAuthorityContinueCommands(items []mission.ExecutionEvidenceReviewItem, lanes []mission.BoardLane) []mission.ExecutionEvidenceReviewItem {
	workstreamLanes := workstreamLanesFromBoard(lanes)
	return bindExecutionEvidenceReviewContinueCommands(items, func(command string) (Lane, bool) {
		label, ok := strings.CutPrefix(strings.TrimSpace(command), "/rekit handoff ")
		if !ok {
			return Lane{}, false
		}
		for _, lane := range workstreamLanes {
			if workstreamLabel(lane) == strings.TrimSpace(label) {
				return lane, true
			}
		}
		return Lane{}, false
	})
}

func workstreamLanesFromBoard(lanes []mission.BoardLane) []Lane {
	workstreamLanes := make([]Lane, 0, len(lanes))
	for _, lane := range lanes {
		workstreamLanes = append(workstreamLanes, Lane{
			ID:                 lane.ID,
			Status:             lane.Status,
			Authority:          lane.Authority,
			Workspace:          lane.Workspace,
			CurrentExecutor:    lane.CurrentExecutor,
			ExecutorGeneration: lane.ExecutorGeneration,
		})
	}
	return workstreamLanes
}

func bindMissionBriefContinueCommands(brief mission.Brief, lanes []Lane) mission.Brief {
	for idx, command := range brief.NextAgentActions {
		for _, lane := range lanes {
			bound := bindContinueCommand(command, lane)
			if bound != command {
				brief.NextAgentActions[idx] = bound
				break
			}
		}
	}
	return brief
}

func handoffHasBlockedAction(actions []mission.LaneExecutorActionSnapshot) bool {
	for _, action := range actions {
		if action.ExecutorAction.Blocked {
			return true
		}
	}
	return false
}

func (ctx handoffContext) plannedWrites(apply bool) ([]StartWrite, error) {
	if ctx.project {
		return ctx.projectWrites(apply)
	}
	return ctx.laneWrites(apply, *ctx.lane)
}

func (ctx handoffContext) projectWrites(apply bool) ([]StartWrite, error) {
	_, writes, err := ctx.renderProject(apply)
	if err != nil {
		return nil, err
	}
	stampPath, latestPath, err := ctx.projectHandoffPaths()
	if err != nil {
		return nil, err
	}
	prefix := "would-"
	if apply {
		prefix = ""
	}
	writes = append(writes, StartWrite{Path: relativePath(ctx.inst.CaseRoot, stampPath), Kind: "handoff", Action: prefix + "write-project-handoff", TargetPath: stampPath})
	writes = append(writes, StartWrite{Path: relativePath(ctx.inst.CaseRoot, latestPath), Kind: "handoff", Action: prefix + "write-latest-project-handoff", TargetPath: latestPath})
	return writes, nil
}

func (ctx handoffContext) laneWrites(apply bool, lane Lane) ([]StartWrite, error) {
	_, writes, err := ctx.renderLane(lane, apply)
	if err != nil {
		return nil, err
	}
	stampPath, latestPath, err := ctx.laneHandoffPaths(lane.ID)
	if err != nil {
		return nil, err
	}
	prefix := "would-"
	if apply {
		prefix = ""
	}
	writes = append(writes, StartWrite{Path: relativePath(ctx.inst.CaseRoot, stampPath), Kind: "handoff", Action: prefix + "write-lane-handoff", TargetPath: stampPath})
	writes = append(writes, StartWrite{Path: relativePath(ctx.inst.CaseRoot, latestPath), Kind: "handoff", Action: prefix + "write-latest-lane-handoff", TargetPath: latestPath})
	return writes, nil
}

func (ctx handoffContext) renderProject(apply bool) (string, []StartWrite, error) {
	writes := []StartWrite{}
	facts, err := readHandoffFacts(ctx.inst.CaseRoot)
	if err != nil {
		return "", nil, err
	}
	handoffRel := strings.TrimSpace(ctx.manifest.WorkstreamDefaults["handoffPath"])
	taskHandoff := ""
	if handoffRel != "" {
		path, err := refsf.SafeJoin(ctx.inst.CaseRoot, handoffRel)
		if err != nil {
			return "", nil, err
		}
		if refsf.Exists(path) {
			taskHandoff = relativePath(ctx.inst.CaseRoot, path)
		}
	}
	latestDigest, err := latestRunDigest(ctx.inst.CaseRoot)
	if err != nil {
		return "", nil, err
	}
	var out bytes.Buffer
	fmt.Fprintln(&out, "# rekit 项目接手索引")
	fmt.Fprintln(&out)
	fmt.Fprintf(&out, "生成时间：%s\n", ctx.publicationTime())
	fmt.Fprintf(&out, "case：%s\n", ctx.inst.CaseRoot)
	fmt.Fprintf(&out, "pack：%s\n", ctx.manifest.Pack)
	fmt.Fprintln(&out)
	fmt.Fprintln(&out, "## 说明")
	fmt.Fprintln(&out)
	fmt.Fprintln(&out, "这是项目级索引，不代表某个会话已经选择主线或支线。新会话应先选择要接手的工作线。")
	fmt.Fprintln(&out)
	fmt.Fprintln(&out, "## 推荐读取")
	fmt.Fprintln(&out)
	if taskHandoff != "" {
		fmt.Fprintf(&out, "- `%s`：case 长期主线 handoff。\n", taskHandoff)
	}
	if latestDigest != "" {
		fmt.Fprintf(&out, "- `%s`：最近一次自动整理摘要。\n", latestDigest)
	}
	fmt.Fprintln(&out)
	writeProjectMissionBrief(&out, ctx.board.Lanes, facts, ctx.currentLanes())
	authorizedGateAdapterHandoffs := AuthorizedGateAdapterHandoffsWithAcknowledgements(ctx.manifest.RepoRoot, ctx.inst.CaseRoot, ctx.manifest.Pack, facts.Requests, "", ExecutionEvidenceReviewAcknowledgedIDs(facts))
	WriteAuthorizedGateAdapterHandoffSection(&out, "## Authorized gate adapter handoff", authorizedGateAdapterHandoffs)
	writeReviewerWritebackItems(&out, ReviewerWritebackItems(facts, ""))
	reviewerDispatchIntakeHandoffs, err := ReviewerDispatchIntakeHandoffs(ctx.inst.CaseRoot, facts, "")
	if err != nil {
		return "", nil, err
	}
	WriteReviewerDispatchIntakeHandoffSection(&out, "## Reviewer dispatch intake handoff", reviewerDispatchIntakeHandoffs)
	reviewerPacketRetirementHandoffs, err := ReviewerPacketRetirementHandoffs(ctx.inst.CaseRoot, "")
	if err != nil {
		return "", nil, err
	}
	WriteReviewerPacketRetirementHandoffSection(&out, "## Reviewer packet retirement handoff", reviewerPacketRetirementHandoffs)
	projectLaneActions := ctx.laneExecutorActions()
	projectExecutionEvidenceReview := ctx.projectExecutionEvidenceReview()
	projectMissionCommanderNext := mission.MissionCommanderNextActions(projectLaneActions, projectExecutionEvidenceReview, handoffHasBlockedAction(projectLaneActions))
	projectMissionCommanderNext = MissionCommanderNextActionsWithAuthorizedGateAdaptersAndAcknowledgements(projectMissionCommanderNext, authorizedGateAdapterHandoffs, ExecutionEvidenceReviewAcknowledgedIDs(facts))
	projectMissionCommanderNext = MissionCommanderNextActionsWithReviewerDispatches(projectMissionCommanderNext, reviewerDispatchIntakeHandoffs)
	if len(ctx.projectMissionCommanderNextActions) > 0 {
		projectMissionCommanderNext = mission.UniqueCommanderNextActions(append(append([]mission.MissionCommanderNextActionItem{}, projectMissionCommanderNext...), ctx.projectMissionCommanderNextActions...))
	}
	writeProjectMissionCommanderActionQueue(&out, projectMissionCommanderNext)
	projectActionQueue := ctx.withCurrentDriverRequest(mission.MissionCommanderActionQueueFor(projectMissionCommanderNext))
	projectRunbook := ctx.dailyMissionControlRunbook("project", "", projectActionQueue)
	writeDailyMissionControlRunbook(&out, projectRunbook)
	latestDriverReceiptHandoff, err := latestDriverReceiptHandoffFor(ctx.inst.CaseRoot, nil)
	if err != nil {
		return "", nil, err
	}
	writeLatestDriverReceiptHandoff(&out, latestDriverReceiptHandoff)
	packagePath, err := ctx.replacementExecutorTakeoverPackageLatestRel()
	if err != nil {
		return "", nil, err
	}
	takeoverPackage, err := handoffReplacementExecutorTakeoverPackage(ctx.inst.CaseRoot, "project", nil, projectActionQueue, projectRunbook, latestDriverReceiptHandoff, packagePath, ctx.currentLoopOperator, ctx.currentDriverRequest != nil)
	if err != nil {
		return "", nil, err
	}
	writeReplacementExecutorTakeoverPackage(&out, takeoverPackage)
	writeCurrentLoopOperatorPackage(&out, ctx.currentLoopOperator)
	writePackMemoryConsumptionHandoff(&out, ctx.projectPackMemoryConsumption)
	writeProjectNextBatchStarterPackage(&out, ctx.projectNextBatchStarterPackage)
	fmt.Fprintln(&out, "## 工作线")
	fmt.Fprintln(&out)
	for _, row := range ctx.board.Lanes {
		lane, err := readLaneByID(ctx.inst.CaseRoot, row.ID)
		if os.IsNotExist(err) {
			continue
		}
		if err != nil {
			return "", nil, err
		}
		resumeRel, resumeWrites, err := ctx.refreshResume(lane, apply)
		if err != nil {
			return "", nil, err
		}
		writes = append(writes, resumeWrites...)
		kind := "功能支线"
		if lane.Authority {
			kind = "主线"
		}
		autonomySummary := autonomy.ReadSummary(ctx.inst.CaseRoot, lane.ID, ctx.manifest)
		executorAction := ctx.executorAction(lane)
		executionEvidenceReview := ctx.executionEvidenceReview(lane)
		authorizedGateAdapterHandoffs := AuthorizedGateAdapterHandoffsWithAcknowledgements(ctx.manifest.RepoRoot, ctx.inst.CaseRoot, ctx.manifest.Pack, facts.Requests, lane.ID, ExecutionEvidenceReviewAcknowledgedIDs(facts))
		laneReviewerDispatches, err := ReviewerDispatchIntakeHandoffs(ctx.inst.CaseRoot, facts, lane.ID)
		if err != nil {
			return "", nil, err
		}
		executorAction = withReviewerDispatchBlocker(executorAction, laneReviewerDispatches)
		missionCommanderNextActions := mission.MissionCommanderNextActions([]mission.LaneExecutorActionSnapshot{laneCommanderActionSnapshot(lane, executorAction)}, executionEvidenceReview, executorAction.Blocked)
		missionCommanderNextActions = MissionCommanderNextActionsWithAuthorizedGateAdaptersAndAcknowledgements(missionCommanderNextActions, authorizedGateAdapterHandoffs, ExecutionEvidenceReviewAcknowledgedIDs(facts))
		missionCommanderNextActions = MissionCommanderNextActionsWithReviewerDispatches(missionCommanderNextActions, laneReviewerDispatches)
		missionCommanderActionQueue := mission.MissionCommanderActionQueueFor(missionCommanderNextActions)
		if ctx.currentDriverRequest != nil && strings.EqualFold(strings.TrimSpace(ctx.currentDriverRequest.Lane), lane.ID) {
			missionCommanderActionQueue = ctx.withCurrentDriverRequest(missionCommanderActionQueue)
		}
		executionEvidenceReviewSummary := ExecutionEvidenceReviewSummaryFor(executionEvidenceReview, missionCommanderActionQueue)
		evidenceNeedsMainReview := ExecutionEvidenceReviewNeedsMainReview(executionEvidenceReview)
		fmt.Fprintf(&out, "- %s `%s`：status=%s，workspace=`%s`，autonomy=%s ready=%t，blocked=%t\n", kind, lane.ID, lane.Status, lane.Workspace, autonomySummary.Mode, autonomySummary.Ready, executorAction.Blocked)
		fmt.Fprintf(&out, "  - executor owner：current=%s generation=%d lastTakeover=%s by=%s reason=%s\n", firstText(lane.CurrentExecutor, "unassigned"), lane.ExecutorGeneration, firstText(lane.LastTakeoverAt, "none"), firstText(lane.LastTakeoverBy, "none"), firstText(lane.LastTakeoverReason, "none"))
		fmt.Fprintf(&out, "  - executor blockers：pendingGates=%d openInterventions=%d openDecisions=%d reasons=%s\n", executorAction.PendingGates, executorAction.OpenInterventions, executorAction.OpenDecisions, firstText(strings.Join(executorAction.BlockerReasons, ","), "none"))
		fmt.Fprintf(&out, "  - requirements：reconcile=%t pendingGate=%t openDecision=%t\n", executorAction.ReconcileRequired, executorAction.PendingGateRequired, executorAction.OpenDecisionRequired)
		writeProjectLaneEvidenceNextSteps(&out, executionEvidenceReview, executorAction.Ready && !executorAction.Blocked)
		writeProjectLaneExecutionEvidenceReviewSummary(&out, executionEvidenceReviewSummary)
		writeProjectLaneMissionCommanderActionQueue(&out, missionCommanderActionQueue)
		writeDailyMissionControlRunbook(&out, ctx.dailyMissionControlRunbook("lane:"+workstreamLabel(lane), workstreamLabel(lane), missionCommanderActionQueue))
		writeCurrentLoopOperatorPackage(&out, currentLoopOperatorForLane(ctx.currentLoopOperator, lane.ID))
		writeProjectLaneMissionCommanderNextActions(&out, missionCommanderNextActions)
		if !evidenceNeedsMainReview {
			writeProjectLaneNextActions(&out, executorAction.NextAgentActions)
		}
		if executorAction.Ready {
			if evidenceNeedsMainReview {
				fmt.Fprintf(&out, "  - evidence review 后继续候选：`%s`（当前因 evidence boundary/escalation 不推荐 autonomous continue）\n", executorAction.ResumeCommand)
			} else {
				fmt.Fprintf(&out, "  - continue command：`%s`\n", executorAction.ResumeCommand)
			}
		} else {
			fmt.Fprintf(&out, "  - ready 后继续：`%s`\n", executorAction.ResumeCommand)
		}
		fmt.Fprintf(&out, "  - 指定交接：`%s`\n", executorAction.HandoffCommand)
		fmt.Fprintf(&out, "  - commander state：%s\n", executorAction.MissionCommanderAction.State)
		fmt.Fprintf(&out, "  - commander prompt：%s\n", executorAction.MissionCommanderAction.Prompt)
		fmt.Fprintf(&out, "  - commander primary：`%s`\n", executorAction.MissionCommanderAction.PrimaryCommand)
		writeProjectLaneCommanderList(&out, "commander follow-up", executorAction.MissionCommanderAction.FollowUpCommands)
		writeProjectLaneCommanderList(&out, "commander boundary", executorAction.MissionCommanderAction.Boundary)
		writeProjectLaneExecutionEvidenceReview(&out, executionEvidenceReview)
		fmt.Fprintf(&out, "  - 接续提示：`%s`\n", resumeRel)
	}
	fmt.Fprintln(&out)
	fmt.Fprintln(&out, "## 注意边界")
	fmt.Fprintln(&out)
	fmt.Fprintln(&out, "- 主线负责最终结论写入；功能支线只写自己的工作区、证据、候选和请求。")
	if handoffRel != "" {
		fmt.Fprintf(&out, "- 本索引不会覆盖 `%s`，只引用它。\n", handoffRel)
	}
	fmt.Fprintln(&out, "- 多工作线时不要使用无参数 `/rekit continue` 盲目继续，应使用 `/rekit continue main` 或 `/rekit continue <name>`。")
	return out.String(), writes, nil
}

func writeLatestDriverReceiptHandoff(out *bytes.Buffer, handoff *LatestDriverReceiptHandoff) {
	if handoff == nil || !handoff.Ready || handoff.MissionCommanderDriverReceipt == nil {
		return
	}
	fmt.Fprintln(out, "## Latest driver receipt handoff")
	fmt.Fprintln(out)
	fmt.Fprintf(out, "- ready: %t\n", handoff.Ready)
	fmt.Fprintf(out, "- state: %s\n", handoff.State)
	fmt.Fprintf(out, "- runId: %s batchId=%s lane=%s\n", handoff.RunID, handoff.BatchID, handoff.Lane)
	fmt.Fprintf(out, "- command: `%s`\n", handoff.Command)
	fmt.Fprintf(out, "- run status: `%s`\n", handoff.RunStatusPath)
	fmt.Fprintf(out, "- run digest: `%s`\n", handoff.RunDigestPath)
	for _, doc := range handoff.TargetDocuments {
		fmt.Fprintf(out, "- target document: %s\n", doc)
	}
	for _, boundary := range handoff.Boundary {
		fmt.Fprintf(out, "- boundary: %s\n", boundary)
	}
	fmt.Fprintln(out)
}

func writeReplacementExecutorTakeoverPackage(out *bytes.Buffer, pkg *mission.ReplacementExecutorTakeoverPackage) {
	if pkg == nil || !pkg.Ready {
		return
	}
	fmt.Fprintln(out, "## Replacement executor takeover package")
	fmt.Fprintln(out)
	fmt.Fprintf(out, "- ready: %t\n", pkg.Ready)
	fmt.Fprintf(out, "- focus: %s scope=%s state=%s source=%s actionId=%s\n", pkg.Focus, pkg.Scope, pkg.State, pkg.Source, pkg.ActionID)
	fmt.Fprintf(out, "- driver: kind=%s executable=%t requiresReview=%t blocked=%t command=`%s` guidance=`%s`\n", pkg.DriverKind, pkg.CommandExecutable, pkg.RequiresReview, pkg.Blocked, pkg.Command, pkg.Guidance)
	fmt.Fprintf(out, "- refresh: `%s`\n", pkg.RefreshStatusCommand)
	fmt.Fprintf(out, "- current driver request: kind=%s step=%s actor=%s executable=%t blocked=%t requiresReview=%t command=`%s` guidance=`%s`\n", pkg.CurrentDriverRequest.Kind, pkg.CurrentDriverRequest.RunLoopStepID, pkg.CurrentDriverRequest.Actor, pkg.CurrentDriverRequest.CommandExecutable, pkg.CurrentDriverRequest.Blocked, pkg.CurrentDriverRequest.RequiresReview, pkg.CurrentDriverRequest.Command, pkg.CurrentDriverRequest.Guidance)
	fmt.Fprintf(out, "- current driver request identity: sha256=%s\n", pkg.CurrentDriverRequestSHA256)
	if pkg.DurableArtifactPath != "" {
		fmt.Fprintf(out, "- durable artifact: path=%s fresh=%t state=%s sha256=%s requestSha256=%s\n", pkg.DurableArtifactPath, pkg.DurableArtifactFresh, pkg.DurableArtifactState, pkg.DurableArtifactSHA256, pkg.DurableArtifactRequestSHA256)
	}
	fmt.Fprintf(out, "- current driver request expected receipt: state=%s command=`%s` refreshStatusCommand=`%s`\n", pkg.CurrentDriverRequest.ExpectedReceipt.State, pkg.CurrentDriverRequest.ExpectedReceipt.Command, pkg.CurrentDriverRequest.ExpectedReceipt.RefreshStatusCommand)
	for _, doc := range pkg.TargetDocuments {
		fmt.Fprintf(out, "- target document: %s\n", doc)
	}
	for idx, step := range pkg.RunbookSteps {
		fmt.Fprintf(out, "- runbook step %d: %s\n", idx+1, step)
	}
	for _, boundary := range pkg.Boundary {
		fmt.Fprintf(out, "- boundary: %s\n", boundary)
	}
	fmt.Fprintln(out)
}

func writeDailyMissionControlDriverRequest(out *bytes.Buffer, label string, request *mission.MissionCommanderDriverRequest) {
	if request == nil {
		return
	}
	fmt.Fprintf(out, "- %s: kind=%s executable=%t blocked=%t requiresReview=%t command=`%s` guidance=`%s`\n", label, request.Kind, request.CommandExecutable, request.Blocked, request.RequiresReview, request.Command, request.Guidance)
	fmt.Fprintf(out, "- %s expected receipt: state=%s command=`%s` refreshStatusCommand=`%s`\n", label, request.ExpectedReceipt.State, request.ExpectedReceipt.Command, request.ExpectedReceipt.RefreshStatusCommand)
	for _, boundary := range request.Boundary {
		fmt.Fprintf(out, "  - %s boundary: %s\n", label, boundary)
	}
	for _, boundary := range request.ExpectedReceipt.Boundary {
		fmt.Fprintf(out, "  - %s receipt boundary: %s\n", label, boundary)
	}
}

func writeDailyMissionControlRunbook(out *bytes.Buffer, runbook *DailyMissionControlRunbook) {
	if runbook == nil || len(runbook.RunLoop) == 0 {
		return
	}
	fmt.Fprintln(out, "## Daily Mission Control runbook")
	fmt.Fprintln(out)
	fmt.Fprintf(out, "- ready: %t\n", runbook.Ready)
	fmt.Fprintf(out, "- scope: %s\n", firstText(runbook.Scope, "case"))
	fmt.Fprintf(out, "- current: state=%s source=%s step=%s command=`%s`\n", firstText(runbook.CurrentState, "none"), firstText(runbook.CurrentSource, "none"), firstText(runbook.CurrentRunLoopStepID, "none"), runbook.CurrentCommand)
	fmt.Fprintf(out, "- refresh: `%s`\n", runbook.RefreshStatusCommand)
	if strings.TrimSpace(runbook.HandoffPreviewCommand) != "" {
		fmt.Fprintf(out, "- handoff preview: `%s`\n", runbook.HandoffPreviewCommand)
	}
	if strings.TrimSpace(runbook.HandoffApplyCommand) != "" {
		fmt.Fprintf(out, "- handoff apply: `%s`\n", runbook.HandoffApplyCommand)
	}
	writeDailyMissionControlDriverRequest(out, "handoff preview driver request", runbook.HandoffPreviewDriverRequest)
	writeDailyMissionControlDriverRequest(out, "handoff apply driver request", runbook.HandoffApplyDriverRequest)
	if request := runbook.CurrentDriverRequest; request != nil {
		fmt.Fprintf(out, "- current driver request: kind=%s executable=%t blocked=%t requiresReview=%t command=`%s` guidance=`%s`\n", request.Kind, request.CommandExecutable, request.Blocked, request.RequiresReview, request.Command, request.Guidance)
		fmt.Fprintf(out, "- current driver request expected receipt: state=%s command=`%s` refreshStatusCommand=`%s`\n", request.ExpectedReceipt.State, request.ExpectedReceipt.Command, request.ExpectedReceipt.RefreshStatusCommand)
	}
	fmt.Fprintf(out, "- run loop: steps=%d\n", len(runbook.RunLoop))
	for _, step := range runbook.RunLoop {
		fmt.Fprintf(out, "- run loop step: order=%d step=%s actor=%s state=%s source=%s driverKind=%s executable=%t blocked=%t requiresReview=%t command=`%s` guidance=`%s`\n", step.Order, step.StepID, step.Actor, step.State, step.Source, step.DriverKind, step.CommandExecutable, step.Blocked, step.RequiresReview, step.Command, step.Guidance)
		for _, boundary := range step.Boundary {
			fmt.Fprintf(out, "  - run loop boundary: step=%s boundary=%s\n", step.StepID, boundary)
		}
	}
	for _, boundary := range runbook.Boundary {
		fmt.Fprintf(out, "- boundary: %s\n", boundary)
	}
	fmt.Fprintln(out)
}

func currentLoopOperatorForLane(pkg *mission.CurrentLoopOperatorPackage, laneID string) *mission.CurrentLoopOperatorPackage {
	if pkg == nil {
		return nil
	}
	laneID = strings.TrimSpace(laneID)
	if laneID == "" || strings.TrimSpace(pkg.Lane) != laneID {
		return nil
	}
	matches := func(request *mission.MissionCommanderDriverRequest) bool {
		return request == nil || strings.TrimSpace(request.Lane) == laneID
	}
	requests := []*mission.MissionCommanderDriverRequest{
		pkg.SourceCurrentDriverRequest,
		pkg.SelectedDriverRequest,
		pkg.StartDriverRequest,
		pkg.ResumeDriverRequest,
	}
	if pkg.ObservationInbox != nil {
		requests = append(requests, pkg.ObservationInbox.SelectedDriverRequest)
	}
	if pkg.ExternalSessionJob != nil {
		job := pkg.ExternalSessionJob
		if job.MemberOwner != nil && strings.TrimSpace(job.MemberOwner.Lane) != laneID {
			return nil
		}
		requests = append(requests, job.AttemptRequest, job.RelayPreviewRequest)
		if job.Dispatcher != nil {
			requests = append(requests,
				job.Dispatcher.ClaimRequest,
				job.Dispatcher.LaunchAcceptedRequest,
				job.Dispatcher.LaunchFailedRequest,
			)
		}
		if job.HarnessPackage != nil {
			requests = append(requests, job.HarnessPackage.AttemptReviewRequest)
			if job.HarnessPackage.Return != nil {
				requests = append(requests,
					job.HarnessPackage.Return.ReviewRequest,
					job.HarnessPackage.Return.RelayRecoveryRequest,
				)
			}
		}
		if job.Transport != nil {
			requests = append(requests,
				job.Transport.DiscoveryRequest,
				job.Transport.DeliveryRequest,
				job.Transport.LaunchRequest,
				job.Transport.ReturnRequest,
				job.Transport.ReplacementRequest,
			)
		}
	}
	if pkg.ExternalMemberHandoff != nil && strings.TrimSpace(pkg.ExternalMemberHandoff.Lane) != laneID {
		return nil
	}
	reviewerAttemptsMatch := true
	appendReviewerAttemptRequests := func(attempt *mission.CurrentLoopReviewerAttempt) {
		if attempt == nil {
			return
		}
		if strings.TrimSpace(attempt.Identity.Lane) != laneID {
			reviewerAttemptsMatch = false
		}
		requests = append(requests,
			attempt.CurrentReviewerDriverRequest,
			attempt.DurableContinuationDriverRequest,
		)
	}
	if pkg.ExternalReviewerHandoff != nil {
		handoff := pkg.ExternalReviewerHandoff
		appendReviewerAttemptRequests(handoff.Attempt)
		if handoff.Wave != nil {
			wave := handoff.Wave
			if strings.TrimSpace(wave.Lane) != laneID {
				return nil
			}
			attemptGroups := [][]*mission.CurrentLoopReviewerAttempt{
				wave.SpawnWave,
				wave.Active,
				wave.Returned,
				wave.Failed,
				wave.Blocked,
				wave.Complete,
				wave.Shards,
			}
			for _, attempts := range attemptGroups {
				for _, attempt := range attempts {
					appendReviewerAttemptRequests(attempt)
				}
			}
		}
	}
	if !reviewerAttemptsMatch {
		return nil
	}
	for _, request := range requests {
		if !matches(request) {
			return nil
		}
	}
	return pkg
}

func writeCurrentLoopOperatorPackage(out *bytes.Buffer, pkg *mission.CurrentLoopOperatorPackage) {
	if pkg == nil {
		return
	}
	fmt.Fprintln(out, "## Current-loop operator")
	fmt.Fprintln(out)
	fmt.Fprintf(out, "- state: %s\n", pkg.State)
	fmt.Fprintf(out, "- route/lane: %s / %s\n", pkg.Route, firstText(pkg.Lane, "none"))
	fmt.Fprintf(out, "- budget: default=%d remaining=%d\n", pkg.DefaultMaxSteps, pkg.RemainingMaxSteps)
	if request := pkg.SelectedDriverRequest; request != nil {
		fmt.Fprintf(out, "- selected driver request: kind=%s state=%s source=%s command=`%s`\n", request.Kind, request.State, request.Source, request.Command)
		fmt.Fprintf(out, "- expected receipt: state=%s refresh=`%s`\n", request.ExpectedReceipt.State, request.ExpectedReceipt.RefreshStatusCommand)
	}
	if receipt := pkg.ObservationReceipt; receipt != nil {
		fmt.Fprintf(out, "- observation receipt: state=%s kind=%s actor=%s source=%s successor=%s path=`%s` sha256=%s\n", receipt.State, receipt.ObservationKind, receipt.Actor, receipt.SourceCheckpointSHA256, receipt.SuccessorCheckpointSHA256, receipt.ObservationPath, receipt.ObservationSHA256)
	}
	if inbox := pkg.ObservationInbox; inbox != nil {
		fmt.Fprintf(out, "- observation inbox: state=%s path=`%s` candidates=%d matching=%d stale=%d invalid=%d\n", inbox.State, inbox.Path, inbox.CandidateCount, inbox.MatchingCount, inbox.StaleCount, inbox.InvalidCount)
		if candidate := inbox.SelectedCandidate; candidate != nil {
			fmt.Fprintf(out, "- selected inbox observation: kind=%s actor=%s path=`%s` sha256=%s\n", candidate.ObservationKind, candidate.Actor, candidate.Path, candidate.SHA256)
		}
		if request := inbox.SelectedDriverRequest; request != nil {
			fmt.Fprintf(out, "- inbox driver request: state=%s source=%s command=`%s`\n", request.State, request.Source, request.Command)
		}
		if receipt := inbox.LatestReceipt; receipt != nil && pkg.ObservationReceipt == nil {
			fmt.Fprintf(out, "- observation receipt: state=%s kind=%s actor=%s source=%s successor=%s path=`%s` sha256=%s\n", receipt.State, receipt.ObservationKind, receipt.Actor, receipt.SourceCheckpointSHA256, receipt.SuccessorCheckpointSHA256, receipt.ObservationPath, receipt.ObservationSHA256)
		}
		for _, warning := range inbox.Warnings {
			fmt.Fprintf(out, "- inbox warning: %s\n", warning)
		}
	}
	if job := pkg.ExternalSessionJob; job != nil {
		fmt.Fprintf(out, "- external session job: state=%s attemptState=%s kind=%s id=%s sha256=%s checkpoint=%s submission=`%s` outcomes=%s submissionLast=%t\n", job.State, job.AttemptState, job.SessionKind, job.JobID, job.JobSHA256, job.CheckpointSHA256, job.SubmissionPath, strings.Join(job.AllowedOutcomes, ","), job.SubmissionLast)
		if dispatcher := job.Dispatcher; dispatcher != nil {
			fmt.Fprintf(out, "- external session dispatcher: state=%s\n", dispatcher.State)
			if ticket := dispatcher.Ticket; ticket != nil {
				fmt.Fprintf(out, "- dispatch ticket: path=`%s` sha256=%s attemptSha256=%s generation=%d\n", ticket.Path, ticket.SHA256, ticket.AttemptSHA256, ticket.Generation)
			}
			if claim := dispatcher.Claim; claim != nil {
				fmt.Fprintf(out, "- dispatch claim: path=`%s` sha256=%s owner=%s/%s actor=%s claimedAt=%s\n", claim.Path, claim.SHA256, claim.Harness, claim.Session, claim.Actor, claim.ClaimedAt)
			}
			if launch := dispatcher.LaunchReceipt; launch != nil {
				fmt.Fprintf(out, "- launch receipt: state=%s path=`%s` sha256=%s actual=%s/%s actor=%s observedAt=%s reason=%s\n", launch.State, launch.Path, launch.SHA256, launch.ActualHarness, launch.ActualSession, launch.Actor, launch.ObservedAt, launch.Reason)
			}
		}
		if harness := job.HarnessPackage; harness != nil {
			fmt.Fprintf(out, "- harness package: state=%s kind=%s job=%s/%s refresh=`%s`\n", harness.State, harness.SessionKind, harness.JobID, harness.JobSHA256, harness.RefreshStatusCommand)
			if launch := harness.Launch; launch != nil {
				fmt.Fprintf(out, "- harness launch: ready=%t tool=%s agentType=%s readOnly=%t inputRole=%s input=`%s` inputSha256=%s attempt=%s generation=%d owner=%s/%s\n", launch.Ready, launch.Tool, launch.AgentType, launch.ReadOnly, launch.Input.Role, launch.Input.Path, launch.Input.SHA256, launch.Attempt.AttemptSHA256, launch.Attempt.Generation, launch.Attempt.Harness, launch.Attempt.Session)
			}
			if returned := harness.Return; returned != nil {
				fmt.Fprintf(out, "- harness return: submission=`%s` outputs=`%s` result=`%s` submissionLast=%t templates=%d\n", returned.SubmissionPath, returned.SubmissionOutputs, returned.SubmissionResult, returned.SubmissionLast, len(returned.Templates))
				for _, template := range returned.Templates {
					fmt.Fprintf(out, "- harness submission template: outcome=%s requiredWrites=%s requiredReplacements=%s\n\n", template.Outcome, strings.Join(template.RequiredWrites, "; "), strings.Join(template.RequiredReplace, "; "))
					fmt.Fprintln(out, "```json")
					fmt.Fprint(out, template.JSON)
					if !strings.HasSuffix(template.JSON, "\n") {
						fmt.Fprintln(out)
					}
					fmt.Fprintln(out, "```")
				}
				if request := returned.ReviewRequest; request != nil {
					fmt.Fprintf(out, "- harness return review: state=%s source=%s command=`%s`\n", request.State, request.Source, request.Command)
				}
			}
			for _, warning := range harness.Warnings {
				fmt.Fprintf(out, "- harness package warning: %s\n", warning)
			}
		}
		if request := job.RelayPreviewRequest; request != nil {
			fmt.Fprintf(out, "- external session relay request: state=%s source=%s command=`%s`\n", request.State, request.Source, request.Command)
		}
		for _, warning := range job.Warnings {
			fmt.Fprintf(out, "- external session job warning: %s\n", warning)
		}
	}
	if handoff := pkg.ExternalMemberHandoff; handoff != nil {
		fmt.Fprintf(out, "- external member: state=%s attempt=%s lane=%s owner=%s/%d handoff=`%s` manifest=`%s` outputs=`%s`\n", handoff.State, handoff.AttemptID, handoff.Lane, handoff.Executor, handoff.ExecutorGeneration, handoff.HandoffPath, handoff.ManifestPath, handoff.OutputsRoot)
		for _, alternative := range handoff.ObservationContract.Alternatives {
			fmt.Fprintf(out, "- member observation: kind=%s transition=%s requiredFlags=%s previewCommandTemplate=`%s` observationPathCommand=`%s` observationEnvelopeTemplate=`%s` constraints=%s\n", alternative.Kind, alternative.Transition, strings.Join(alternative.RequiredFlags, ","), alternative.PreviewCommandTemplate, alternative.ObservationPathCommand, alternative.ObservationEnvelopeTemplate, strings.Join(alternative.Constraints, "; "))
		}
	}
	if handoff := pkg.ExternalReviewerHandoff; handoff != nil {
		fmt.Fprintf(out, "- external reviewer: state=%s step=%s dropPath=`%s` dropRole=%s\n", handoff.State, handoff.RunLoopStepID, handoff.ReviewerResultDropPath, handoff.ReviewerResultDropPathRole)
		if attempt := handoff.Attempt; attempt != nil {
			fmt.Fprintf(out, "- reviewer attempt: id=%s snapshotSha256=%s state=%s step=%s action=%s packet=%s route=%s shard=%s owner=%s/%d current=%s/%d dispatch=%s session=%s lifecycle=%s\n", attempt.AttemptID, attempt.AttemptSnapshotSHA256, attempt.State, attempt.RunLoopStepID, attempt.SelectedAction.Kind, attempt.Identity.PacketID, attempt.Identity.RouteID, attempt.Identity.ShardID, attempt.Identity.OwnerExecutor, attempt.Identity.OwnerGeneration, attempt.Identity.CurrentExecutor, attempt.Identity.CurrentGeneration, attempt.Receipt.DispatchID, attempt.Receipt.Session, attempt.Receipt.SessionLifecycleState)
		}
		if wave := handoff.Wave; wave != nil {
			fmt.Fprintf(out, "- reviewer wave: packet=%s maxParallel=%d total=%d activeSlots=%d availableSlots=%d spawn=%d active=%d returned=%d failed=%d blocked=%d complete=%d\n", wave.PacketID, wave.MaxParallel, wave.TotalShards, wave.ActiveSlots, wave.AvailableSlots, len(wave.SpawnWave), len(wave.Active), len(wave.Returned), len(wave.Failed), len(wave.Blocked), len(wave.Complete))
			for _, attempt := range wave.Shards {
				fmt.Fprintf(out, "  - reviewer wave shard: shard=%s state=%s step=%s action=%s attempt=%s snapshotSha256=%s session=%s lifecycle=%s\n", attempt.Identity.ShardID, attempt.State, attempt.RunLoopStepID, attempt.SelectedAction.Kind, attempt.AttemptID, attempt.AttemptSnapshotSHA256, attempt.Receipt.Session, attempt.Receipt.SessionLifecycleState)
			}
		}
		if request := handoff.AgentToolRequest; request != nil {
			fmt.Fprintf(out, "- Agent request: tool=%s agentType=%s readOnly=%t promptPath=`%s` promptSha256=%s expectedOutput=%s\n", request.Tool, request.AgentType, request.ReadOnly, request.PromptPath, request.PromptSHA256, request.ExpectedOutput)
		}
		for _, alternative := range handoff.ObservationContract.Alternatives {
			fmt.Fprintf(out, "- observation: kind=%s transition=%s requiredFlags=%s previewCommandTemplate=`%s` observationPathCommand=`%s` observationEnvelopeTemplate=`%s` constraints=%s\n", alternative.Kind, alternative.Transition, strings.Join(alternative.RequiredFlags, ","), alternative.PreviewCommandTemplate, alternative.ObservationPathCommand, alternative.ObservationEnvelopeTemplate, strings.Join(alternative.Constraints, "; "))
		}
	}
	for _, step := range pkg.RunbookSteps {
		fmt.Fprintf(out, "- operator step: %s\n", step)
	}
	for _, criterion := range pkg.CompletionCriteria {
		fmt.Fprintf(out, "- completion criterion: %s\n", criterion)
	}
	for _, boundary := range pkg.Boundary {
		fmt.Fprintf(out, "- boundary: %s\n", boundary)
	}
	fmt.Fprintln(out)
}

func writePackMemoryConsumptionHandoff(out *bytes.Buffer, handoff *PackMemoryConsumptionHandoff) {
	if handoff == nil {
		return
	}
	fmt.Fprintln(out, "## Pack-memory consumption handoff")
	fmt.Fprintln(out)
	fmt.Fprintf(out, "- completed changes：available=%d consumed=%d conflicts=%d\n", handoff.Available, handoff.Consumed, handoff.Conflicts)
	if current := handoff.MissionCommanderActionQueue.CurrentAction; current != nil {
		fmt.Fprintf(out, "- current action：%s\n", MissionCommanderNextActionMarkdownLine(*current))
	}
	for _, item := range handoff.MissionCommanderNextActions {
		fmt.Fprintf(out, "- action：%s\n", MissionCommanderNextActionMarkdownLine(item))
	}
	for _, boundary := range handoff.Boundary {
		fmt.Fprintf(out, "- boundary：%s\n", boundary)
	}
	fmt.Fprintln(out)
}

func writeProjectNextBatchStarterPackage(out *bytes.Buffer, starter *ProjectNextBatchStarterPackage) {
	if starter == nil || !starter.Ready {
		return
	}
	fmt.Fprintln(out, "## Project next-batch starter package")
	fmt.Fprintln(out)
	fmt.Fprintf(out, "- ready: %t\n", starter.Ready)
	fmt.Fprintf(out, "- latestCompletedBatch: %s\n", firstText(starter.LatestCompletedBatch, "none"))
	fmt.Fprintf(out, "- suggestedNextBatch: %s\n", firstText(starter.SuggestedNextBatch, "none"))
	fmt.Fprintln(out, "- current batch section:")
	for line := range strings.SplitSeq(starter.CurrentBatchSection, "\n") {
		fmt.Fprintf(out, "  %s\n", line)
	}
	fmt.Fprintf(out, "- changelog entry: %s\n", starter.ChangelogEntry)
	writeProjectNextBatchStarterList(out, "validation command", starter.ValidationCommands)
	writeProjectNextBatchStarterRunLoop(out, starter)
	writeProjectNextBatchStarterList(out, "recommended step", starter.RecommendedStarterSteps)
	writeProjectNextBatchStarterList(out, "release cadence step", starter.ReleaseCadenceSteps)
	writeProjectNextBatchStarterList(out, "boundary", starter.Boundary)
	fmt.Fprintln(out)
}

func writeProjectNextBatchStarterRunLoop(out *bytes.Buffer, starter *ProjectNextBatchStarterPackage) {
	if starter == nil || len(starter.RunLoop) == 0 {
		return
	}
	fmt.Fprintf(out, "- starter run loop: currentRunLoopStep=%s steps=%d\n", starter.CurrentRunLoopStepID, len(starter.RunLoop))
	for _, step := range starter.RunLoop {
		fmt.Fprintf(out, "- starter run loop step: order=%d step=%s actor=%s state=%s source=%s command=`%s` description=%s\n", step.Order, step.StepID, step.Actor, step.State, step.Source, step.Command, step.Description)
		for _, boundary := range step.Boundary {
			fmt.Fprintf(out, "- starter run loop boundary: step=%s boundary=%s\n", step.StepID, boundary)
		}
	}
}

func writeProjectNextBatchStarterList(out *bytes.Buffer, label string, items []string) {
	if len(items) == 0 {
		fmt.Fprintf(out, "- %s: none\n", label)
		return
	}
	for _, item := range items {
		fmt.Fprintf(out, "- %s: %s\n", label, item)
	}
}

func writeProjectMissionCommanderActionQueue(out *bytes.Buffer, items []mission.MissionCommanderNextActionItem) {
	fmt.Fprintln(out, "## Project Mission Commander action queue")
	fmt.Fprintln(out)
	queue := mission.MissionCommanderActionQueueFor(items)
	fmt.Fprintf(out, "- summary: %s\n", queue.Summary)
	fmt.Fprintf(out, "- counts: total=%d unblocked=%d blocked=%d requiresReview=%d followUp=%d\n", queue.Counts.Total, queue.Counts.Unblocked, queue.Counts.Blocked, queue.Counts.RequiresReview, queue.Counts.FollowUp)
	if queue.CurrentAction == nil {
		fmt.Fprintln(out, "- current: none")
	} else {
		fmt.Fprintf(out, "- current: %s\n", MissionCommanderNextActionMarkdownLine(*queue.CurrentAction))
		for _, line := range MissionCommanderActionRunLoopMarkdownLines(queue) {
			fmt.Fprintf(out, "- %s\n", line)
		}
	}
	if len(items) == 0 {
		fmt.Fprintln(out, "- next action: none")
		fmt.Fprintln(out)
		return
	}
	for _, line := range missionCommanderNextActionLines(limitProjectMissionCommanderNextActionItems(items, maxHandoffRows)) {
		fmt.Fprintf(out, "- next action: %s\n", line)
	}
	fmt.Fprintln(out)
}

func writeProjectLaneNextActions(out *bytes.Buffer, actions []string) {
	if len(actions) == 0 {
		fmt.Fprintln(out, "  - next action：none")
		return
	}
	for _, action := range actions {
		fmt.Fprintf(out, "  - next action：%s\n", action)
	}
}

func writeProjectLaneEvidenceNextSteps(out *bytes.Buffer, items []ExecutionEvidenceReviewItem, includeContinueFollowUp bool) {
	for _, action := range ExecutionEvidenceReviewNextSteps(items, includeContinueFollowUp) {
		fmt.Fprintf(out, "  - evidence next action：%s\n", action)
	}
}

func writeProjectLaneMissionCommanderActionQueue(out *bytes.Buffer, queue mission.MissionCommanderActionQueue) {
	fmt.Fprintf(out, "  - commander action queue：%s\n", queue.Summary)
	fmt.Fprintf(out, "  - commander action queue counts：total=%d unblocked=%d blocked=%d requiresReview=%d followUp=%d\n", queue.Counts.Total, queue.Counts.Unblocked, queue.Counts.Blocked, queue.Counts.RequiresReview, queue.Counts.FollowUp)
	if queue.CurrentAction == nil {
		fmt.Fprintln(out, "  - commander action queue current：none")
		return
	}
	item := *queue.CurrentAction
	fmt.Fprintf(out, "  - commander action queue current：%s\n", MissionCommanderNextActionMarkdownLine(item))
	for _, line := range MissionCommanderActionRunLoopMarkdownLines(queue) {
		fmt.Fprintf(out, "  - commander action queue %s\n", line)
	}
}

func writeProjectLaneMissionCommanderNextActions(out *bytes.Buffer, items []mission.MissionCommanderNextActionItem) {
	if len(items) == 0 {
		fmt.Fprintln(out, "  - commander next action：none")
		return
	}
	for _, item := range missionCommanderNextActionLines(limitMissionCommanderNextActionItems(items, maxHandoffRows)) {
		fmt.Fprintf(out, "  - commander next action：%s\n", item)
	}
}

func writeLaneMissionCommanderActionQueue(out *bytes.Buffer, queue mission.MissionCommanderActionQueue) {
	fmt.Fprintln(out, "## Mission Commander action queue")
	fmt.Fprintln(out)
	fmt.Fprintf(out, "- summary: %s\n", queue.Summary)
	fmt.Fprintf(out, "- counts: total=%d unblocked=%d blocked=%d requiresReview=%d followUp=%d\n", queue.Counts.Total, queue.Counts.Unblocked, queue.Counts.Blocked, queue.Counts.RequiresReview, queue.Counts.FollowUp)
	if queue.CurrentAction == nil {
		fmt.Fprintln(out, "- current: none")
		fmt.Fprintln(out)
		return
	}
	item := *queue.CurrentAction
	fmt.Fprintf(out, "- current: %s\n", MissionCommanderNextActionMarkdownLine(item))
	for _, line := range MissionCommanderActionRunLoopMarkdownLines(queue) {
		fmt.Fprintf(out, "- %s\n", line)
	}
	fmt.Fprintln(out)
}

func writeLaneMissionCommanderNextActions(out *bytes.Buffer, items []mission.MissionCommanderNextActionItem) {
	fmt.Fprintln(out, "## Mission Commander next actions")
	fmt.Fprintln(out)
	if len(items) == 0 {
		fmt.Fprintln(out, "- none")
		fmt.Fprintln(out)
		return
	}
	for _, item := range missionCommanderNextActionLines(limitMissionCommanderNextActionItems(items, maxHandoffRows)) {
		fmt.Fprintf(out, "- %s\n", item)
	}
	fmt.Fprintln(out)
}

func missionCommanderNextActionLines(items []mission.MissionCommanderNextActionItem) []string {
	lines := make([]string, 0, len(items))
	for _, item := range items {
		lines = append(lines, MissionCommanderNextActionMarkdownLine(item))
		for _, reason := range item.Reasons {
			lines = append(lines, "reason: "+reason)
		}
		for _, boundary := range item.Boundary {
			lines = append(lines, "boundary: "+boundary)
		}
	}
	return lines
}

func MissionCommanderNextActionMarkdownLine(item mission.MissionCommanderNextActionItem) string {
	return fmt.Sprintf("state=%s source=%s blocked=%t requiresReview=%t command=`%s` lane=%s label=%s gateEventId=%s actionId=%s", item.State, item.Source, item.Blocked, item.RequiresReview, item.Command, item.Lane, item.Label, item.GateEventID, item.ActionID)
}

func MissionCommanderActionRunLoopMarkdownLines(queue mission.MissionCommanderActionQueue) []string {
	if len(queue.CurrentActionRunLoop) == 0 {
		return nil
	}
	lines := []string{fmt.Sprintf("current run loop：currentRunLoopStep=%s steps=%d", queue.CurrentRunLoopStepID, len(queue.CurrentActionRunLoop))}
	for _, step := range queue.CurrentActionRunLoop {
		lines = append(lines, fmt.Sprintf("run loop step：order=%d step=%s actor=%s state=%s source=%s command=`%s` description=%s", step.Order, step.StepID, step.Actor, step.State, step.Source, step.Command, step.Description))
		for _, boundary := range step.Boundary {
			lines = append(lines, fmt.Sprintf("run loop boundary：step=%s boundary=%s", step.StepID, boundary))
		}
	}
	if request := queue.CurrentDriverRequest; request != nil {
		lines = append(lines, fmt.Sprintf("driver request：kind=%s step=%s actor=%s executable=%t blocked=%t requiresReview=%t command=`%s` guidance=`%s` state=%s source=%s lane=%s label=%s gateEventId=%s actionId=%s", request.Kind, request.RunLoopStepID, request.Actor, request.CommandExecutable, request.Blocked, request.RequiresReview, request.Command, request.Guidance, request.State, request.Source, request.Lane, request.Label, request.GateEventID, request.ActionID))
		lines = append(lines, fmt.Sprintf("driver request expected receipt：state=%s command=`%s` refreshStatusCommand=`%s` description=%s", request.ExpectedReceipt.State, request.ExpectedReceipt.Command, request.ExpectedReceipt.RefreshStatusCommand, request.ExpectedReceipt.Description))
		for _, boundary := range request.Boundary {
			lines = append(lines, fmt.Sprintf("driver request boundary：%s", boundary))
		}
		for _, boundary := range request.ExpectedReceipt.Boundary {
			lines = append(lines, fmt.Sprintf("driver receipt boundary：%s", boundary))
		}
	}
	return lines
}

func limitMissionCommanderNextActionItems(items []mission.MissionCommanderNextActionItem, n int) []mission.MissionCommanderNextActionItem {
	if n <= 0 || len(items) <= n {
		return items
	}
	return items[len(items)-n:]
}

func limitProjectMissionCommanderNextActionItems(items []mission.MissionCommanderNextActionItem, n int) []mission.MissionCommanderNextActionItem {
	if projectNextBatchCandidateQueue(items) || projectQueueContainsNextBatchGuidance(items) {
		return items
	}
	return limitMissionCommanderNextActionItems(items, n)
}

func projectQueueContainsNextBatchGuidance(items []mission.MissionCommanderNextActionItem) bool {
	for _, item := range items {
		if strings.HasPrefix(strings.TrimSpace(item.Source), "releaseHandoffNextBatch") {
			return true
		}
	}
	return false
}

func projectNextBatchCandidateQueue(items []mission.MissionCommanderNextActionItem) bool {
	if len(items) == 0 {
		return false
	}
	for _, item := range items {
		source := strings.TrimSpace(item.Source)
		if source != "releaseHandoffNextBatch" && source != "releaseHandoffNextBatch.followUp.candidateDomain" {
			return false
		}
	}
	return true
}

func writeProjectLaneCommanderList(out *bytes.Buffer, label string, items []string) {
	if len(items) == 0 {
		fmt.Fprintf(out, "  - %s：none\n", label)
		return
	}
	for _, item := range mission.LimitStrings(items, maxHandoffRows) {
		fmt.Fprintf(out, "  - %s：%s\n", label, item)
	}
}

func writeProjectLaneExecutionEvidenceReviewSummary(out *bytes.Buffer, summary ExecutionEvidenceReviewSummary) {
	if summary.Total == 0 {
		return
	}
	fmt.Fprintf(out, "  - evidence review summary：total=%d readyForReview=%d mainEscalations=%d duplicates=%d outputRefs=%d evidenceRefs=%d boundaryHits=%d latestEventId=%s gateEventId=%s status=%s action=%s\n", summary.Total, summary.ReadyForReviewCount, summary.MainEscalationCount, summary.DuplicateCount, summary.OutputRefCount, summary.EvidenceRefCount, summary.BoundaryHitCount, summary.LatestEventID, summary.LatestGateEventID, summary.LatestStatus, firstText(summary.LatestAction, "none"))
	if strings.TrimSpace(summary.CurrentAction) != "" {
		fmt.Fprintf(out, "  - evidence review summary current action：`%s`\n", summary.CurrentAction)
	}
	if strings.TrimSpace(summary.ActionQueueSummary) != "" {
		fmt.Fprintf(out, "  - evidence review summary action queue：%s\n", summary.ActionQueueSummary)
	}
	if strings.TrimSpace(summary.LatestReviewCommand) != "" || strings.TrimSpace(summary.LatestHandoffCommand) != "" {
		fmt.Fprintf(out, "  - evidence review summary handoff：review=`%s` handoff=`%s`\n", summary.LatestReviewCommand, summary.LatestHandoffCommand)
	}
	if strings.TrimSpace(summary.LatestCommanderState) != "" || strings.TrimSpace(summary.LatestCommanderPrimary) != "" {
		fmt.Fprintf(out, "  - evidence review summary commander：state=%s primary=`%s`\n", summary.LatestCommanderState, summary.LatestCommanderPrimary)
	}
	if strings.TrimSpace(summary.LatestExecutionReportPath) != "" || strings.TrimSpace(summary.LatestExecutionReportSHA256) != "" || strings.TrimSpace(summary.LatestAdapterID) != "" || strings.TrimSpace(summary.LatestAdapterStatus) != "" {
		fmt.Fprintf(out, "  - evidence review summary report：path=%s sha256=%s adapterId=%s adapterStatus=%s\n", firstText(summary.LatestExecutionReportPath, "none"), firstText(summary.LatestExecutionReportSHA256, "none"), firstText(summary.LatestAdapterID, "none"), firstText(summary.LatestAdapterStatus, "none"))
	}
	writeProjectLaneExecutionEvidenceAdapterContext(out, summary.LatestAdapterContext)
	for _, hit := range mission.LimitStrings(summary.LatestBoundaryHits, maxHandoffRows) {
		fmt.Fprintf(out, "  - evidence review summary latest boundary hit：%s\n", hit)
	}
	if strings.TrimSpace(summary.LatestEscalation) != "" {
		fmt.Fprintf(out, "  - evidence review summary latest escalation：%s\n", summary.LatestEscalation)
	}
	if strings.TrimSpace(summary.FollowThroughState) != "" || summary.OutcomeCount > 0 {
		fmt.Fprintf(out, "  - evidence review summary follow-through：state=%s outcomes=%d\n", summary.FollowThroughState, summary.OutcomeCount)
	}
	for _, boundary := range mission.LimitStrings(summary.Boundary, maxHandoffRows) {
		fmt.Fprintf(out, "  - evidence review summary boundary：%s\n", boundary)
	}
}

func writeProjectLaneExecutionEvidenceReview(out *bytes.Buffer, items []ExecutionEvidenceReviewItem) {
	if len(items) == 0 {
		fmt.Fprintln(out, "  - execution evidence review：none")
		return
	}
	for _, item := range items {
		fmt.Fprintf(out, "  - execution evidence review：%s status=%s gateEventId=%s action=%s\n", firstText(item.Subject, item.Summary, item.EventID), item.Status, item.GateEventID, firstText(item.Action, "none"))
		writeProjectLaneExecutionEvidenceBoundaryDetail(out, item)
		writeProjectLaneExecutionEvidenceReportDetail(out, item)
		fmt.Fprintf(out, "  - evidence review command：`%s`\n", item.ReviewCommand)
		fmt.Fprintf(out, "  - evidence handoff：`%s`\n", item.HandoffCommand)
		for idx, step := range item.ReviewRunbookSteps {
			fmt.Fprintf(out, "  - evidence review runbook：step=%d text=%s\n", idx+1, step)
		}
		fmt.Fprintf(out, "  - evidence commander：state=%s primary=`%s`\n", item.MissionCommanderAction.State, item.MissionCommanderAction.PrimaryCommand)
		writeProjectLaneEvidenceFollowThrough(out, item.FollowThrough)
		for _, followUp := range mission.LimitStrings(item.MissionCommanderAction.FollowUpCommands, maxHandoffRows) {
			fmt.Fprintf(out, "  - evidence commander follow-up：%s\n", followUp)
		}
		for _, boundary := range mission.LimitStrings(item.Boundary, maxHandoffRows) {
			fmt.Fprintf(out, "  - evidence boundary：%s\n", boundary)
		}
	}
}

func writeProjectLaneExecutionEvidenceBoundaryDetail(out *bytes.Buffer, item ExecutionEvidenceReviewItem) {
	for _, hit := range mission.LimitStrings(item.BoundaryHits, maxHandoffRows) {
		fmt.Fprintf(out, "  - evidence boundary hit：%s\n", hit)
	}
	if strings.TrimSpace(item.Escalation) != "" {
		fmt.Fprintf(out, "  - evidence escalation：%s\n", item.Escalation)
	}
}

func writeProjectLaneExecutionEvidenceReportDetail(out *bytes.Buffer, item ExecutionEvidenceReviewItem) {
	if strings.TrimSpace(item.ExecutionReportPath) != "" || strings.TrimSpace(item.ExecutionReportSHA256) != "" {
		fmt.Fprintf(out, "  - evidence report：%s sha256=%s\n", firstText(item.ExecutionReportPath, "none"), firstText(item.ExecutionReportSHA256, "none"))
	}
	if item.ActualBudget != nil {
		fmt.Fprintf(out, "  - evidence budget：runtimeSeconds=%d diskMB=%d requests=%d\n", item.ActualBudget.RuntimeSeconds, item.ActualBudget.DiskMB, item.ActualBudget.Requests)
	}
	if strings.TrimSpace(item.AdapterID) != "" || strings.TrimSpace(item.AdapterStatus) != "" {
		fmt.Fprintf(out, "  - evidence adapter：adapterId=%s status=%s\n", item.AdapterID, item.AdapterStatus)
	}
	writeProjectLaneExecutionEvidenceAdapterContext(out, item.AdapterContext)
}

func writeProjectLaneExecutionEvidenceAdapterContext(out *bytes.Buffer, context *mission.ExecutionEvidenceAdapterContext) {
	if context == nil {
		return
	}
	fmt.Fprintf(out, "  - evidence adapter context：id=%s status=%s entry=%s gateActions=%s recordOnlyAfterGate=%t toolingCatalogPath=%s\n", context.ID, context.Status, context.Entry, strings.Join(context.GateActions, ","), context.RecordOnlyAfterGate, context.ToolingCatalogPath)
	if strings.TrimSpace(context.Purpose) != "" {
		fmt.Fprintf(out, "  - evidence adapter context purpose：%s\n", context.Purpose)
	}
	if len(context.SideEffects) > 0 {
		fmt.Fprintf(out, "  - evidence adapter context side effects：%s\n", strings.Join(context.SideEffects, ","))
	}
	for _, guidance := range mission.LimitStrings(context.ReportGuidance, maxHandoffRows) {
		fmt.Fprintf(out, "  - evidence adapter context report guidance：%s\n", guidance)
	}
	for _, guidance := range mission.LimitStrings(context.EvidenceGuidance, maxHandoffRows) {
		fmt.Fprintf(out, "  - evidence adapter context evidence guidance：%s\n", guidance)
	}
	if len(context.StopConditionHints) > 0 {
		fmt.Fprintf(out, "  - evidence adapter context stop conditions：%s\n", strings.Join(context.StopConditionHints, ","))
	}
}
func writeProjectLaneEvidenceFollowThrough(out *bytes.Buffer, follow mission.ExecutionEvidenceFollowThrough) {
	if strings.TrimSpace(follow.State) == "" && len(follow.Outcomes) == 0 {
		return
	}
	fmt.Fprintf(out, "  - evidence follow-through：state=%s gateEventId=%s outcomes=%d\n", follow.State, follow.GateEventID, len(follow.Outcomes))
	for _, outcome := range follow.Outcomes {
		fmt.Fprintf(out, "  - evidence follow-through outcome：name=%s state=%s command=`%s` expected=%s\n", outcome.Name, outcome.State, outcome.Command, outcome.Expected)
		if strings.TrimSpace(outcome.When) != "" {
			fmt.Fprintf(out, "  - evidence follow-through when：name=%s when=%s\n", outcome.Name, outcome.When)
		}
		for _, action := range mission.LimitStrings(outcome.Actions, maxHandoffRows) {
			fmt.Fprintf(out, "  - evidence follow-through action：name=%s action=%s\n", outcome.Name, action)
		}
		for _, command := range mission.LimitStrings(outcome.VerificationCommands, maxHandoffRows) {
			fmt.Fprintf(out, "  - evidence follow-through verification：name=%s command=%s\n", outcome.Name, command)
		}
		for _, evidence := range mission.LimitStrings(outcome.Evidence, maxHandoffRows) {
			fmt.Fprintf(out, "  - evidence follow-through evidence：name=%s evidence=%s\n", outcome.Name, evidence)
		}
	}
	if strings.TrimSpace(follow.ActionQueue.Summary) != "" {
		fmt.Fprintf(out, "  - evidence follow-through queue：summary=%s\n", follow.ActionQueue.Summary)
	}
}

func (ctx handoffContext) renderLane(lane Lane, apply bool) (string, []StartWrite, error) {
	writes := []StartWrite{}
	resumeRel, resumeWrites, err := ctx.refreshResume(lane, apply)
	if err != nil {
		return "", nil, err
	}
	writes = append(writes, resumeWrites...)
	latestDigest, err := latestRunDigest(ctx.inst.CaseRoot)
	if err != nil {
		return "", nil, err
	}
	_, latestPath, err := ctx.laneHandoffPaths(lane.ID)
	if err != nil {
		return "", nil, err
	}
	kind := "功能支线"
	if lane.Authority {
		kind = "主线"
	}
	label := workstreamLabel(lane)
	executorAction := bindHandoffLaneExecutorAction(
		ctx.executorAction(lane),
		lane.ID,
		label,
	)
	executionEvidenceReview := ctx.executionEvidenceReview(lane)
	evidenceNeedsMainReview := ExecutionEvidenceReviewNeedsMainReview(executionEvidenceReview)
	facts, err := readHandoffFacts(ctx.inst.CaseRoot)
	if err != nil {
		return "", nil, err
	}
	authorizedGateAdapterHandoffs := AuthorizedGateAdapterHandoffsWithAcknowledgements(ctx.manifest.RepoRoot, ctx.inst.CaseRoot, ctx.manifest.Pack, facts.Requests, lane.ID, ExecutionEvidenceReviewAcknowledgedIDs(facts))
	laneReviewerDispatches, err := ReviewerDispatchIntakeHandoffs(ctx.inst.CaseRoot, facts, lane.ID)
	if err != nil {
		return "", nil, err
	}
	executorAction = withReviewerDispatchBlocker(executorAction, laneReviewerDispatches)
	missionCommanderNextActions := mission.MissionCommanderNextActions([]mission.LaneExecutorActionSnapshot{laneCommanderActionSnapshot(lane, executorAction)}, executionEvidenceReview, executorAction.Blocked)
	missionCommanderNextActions = MissionCommanderNextActionsWithAuthorizedGateAdaptersAndAcknowledgements(missionCommanderNextActions, authorizedGateAdapterHandoffs, ExecutionEvidenceReviewAcknowledgedIDs(facts))
	missionCommanderNextActions = MissionCommanderNextActionsWithReviewerDispatches(missionCommanderNextActions, laneReviewerDispatches)
	missionCommanderActionQueue := mission.MissionCommanderActionQueueFor(missionCommanderNextActions)
	missionCommanderActionQueue = bindHandoffLaneActionQueue(
		missionCommanderActionQueue,
		lane.ID,
		label,
	)
	missionCommanderActionQueue = ctx.withCurrentDriverRequest(missionCommanderActionQueue)
	executionEvidenceReviewSummary := ExecutionEvidenceReviewSummaryFor(executionEvidenceReview, missionCommanderActionQueue)
	var out bytes.Buffer
	fmt.Fprintf(&out, "# rekit 工作线接手：%s\n", lane.ID)
	fmt.Fprintln(&out)
	fmt.Fprintf(&out, "生成时间：%s\n", ctx.publicationTime())
	fmt.Fprintf(&out, "case：%s\n", ctx.inst.CaseRoot)
	fmt.Fprintf(&out, "pack：%s\n", ctx.manifest.Pack)
	fmt.Fprintf(&out, "类型：%s\n", kind)
	fmt.Fprintf(&out, "状态：%s\n", lane.Status)
	fmt.Fprintf(&out, "工作区：%s\n", lane.Workspace)
	fmt.Fprintf(&out, "current executor：%s\n", firstText(lane.CurrentExecutor, "unassigned"))
	fmt.Fprintf(&out, "executor generation：%d\n", lane.ExecutorGeneration)
	fmt.Fprintf(&out, "last takeover：%s by %s reason=%s\n", firstText(lane.LastTakeoverAt, "none"), firstText(lane.LastTakeoverBy, "none"), firstText(lane.LastTakeoverReason, "none"))
	fmt.Fprintln(&out)
	fmt.Fprintln(&out, "## 新会话开场")
	fmt.Fprintln(&out)
	if executorAction.Ready {
		if evidenceNeedsMainReview {
			fmt.Fprintf(&out, "直接说：按 `%s` 接手，先 review execution evidence 并通知 main Agent；当前不要执行 `%s`。\n", relativePath(ctx.inst.CaseRoot, latestPath), executorAction.ResumeCommand)
			writeHandoffBriefList(&out, "evidence next action", ExecutionEvidenceReviewNextSteps(executionEvidenceReview, false))
		} else {
			fmt.Fprintf(&out, "直接说：按 `%s` 接手，然后执行 `%s`。\n", relativePath(ctx.inst.CaseRoot, latestPath), executorAction.ResumeCommand)
		}
	} else if executorAction.Blocked {
		fmt.Fprintf(&out, "直接说：按 `%s` 接手，先处理下列 blocker，不要执行 `/rekit continue %s`。\n", relativePath(ctx.inst.CaseRoot, latestPath), label)
		writeHandoffBriefList(&out, "next agent action", executorAction.NextAgentActions)
	} else {
		fmt.Fprintf(&out, "直接说：按 `%s` 接手，先阅读/刷新交接；当前不建议执行 `/rekit continue %s`。\n", relativePath(ctx.inst.CaseRoot, latestPath), label)
		writeHandoffBriefList(&out, "next agent action", executorAction.NextAgentActions)
	}
	fmt.Fprintln(&out)
	fmt.Fprintln(&out, "## 推荐读取")
	fmt.Fprintln(&out)
	fmt.Fprintf(&out, "- `%s`：本工作线接续提示。\n", resumeRel)
	if lane.Authority {
		if rel := strings.TrimSpace(ctx.manifest.WorkstreamDefaults["handoffPath"]); rel != "" {
			path, err := refsf.SafeJoin(ctx.inst.CaseRoot, rel)
			if err != nil {
				return "", nil, err
			}
			if refsf.Exists(path) {
				fmt.Fprintf(&out, "- `%s`：case 长期主线 handoff。\n", relativePath(ctx.inst.CaseRoot, path))
			}
		}
	}
	if latestDigest != "" {
		fmt.Fprintf(&out, "- `%s`：最近一次自动整理摘要。\n", latestDigest)
	}
	fmt.Fprintln(&out)
	if err := writeWorkspacePackets(&out, ctx.inst.CaseRoot, lane); err != nil {
		return "", nil, err
	}
	writeLaneMissionBrief(&out, lane, facts, executorAction)
	writeLaneMissionCommanderActionQueue(&out, missionCommanderActionQueue)
	dailyRunbook := ctx.dailyMissionControlRunbook("lane:"+label, lane.ID, missionCommanderActionQueue)
	bindHandoffLaneRunbook(dailyRunbook, lane.ID, label)
	writeDailyMissionControlRunbook(&out, dailyRunbook)
	laneOperator := currentLoopOperatorForLane(ctx.currentLoopOperator, lane.ID)
	writeCurrentLoopOperatorPackage(&out, laneOperator)
	latestDriverReceiptHandoff, err := latestDriverReceiptHandoffFor(ctx.inst.CaseRoot, &lane)
	if err != nil {
		return "", nil, err
	}
	writeLatestDriverReceiptHandoff(&out, latestDriverReceiptHandoff)
	packagePath, err := ctx.replacementExecutorTakeoverPackageLatestRel()
	if err != nil {
		return "", nil, err
	}
	takeoverPackage, err := handoffReplacementExecutorTakeoverPackage(ctx.inst.CaseRoot, "lane:"+label, &lane, missionCommanderActionQueue, dailyRunbook, latestDriverReceiptHandoff, packagePath, laneOperator, ctx.currentDriverRequest != nil)
	if err != nil {
		return "", nil, err
	}
	writeReplacementExecutorTakeoverPackage(&out, takeoverPackage)
	writeLaneMissionCommanderNextActions(&out, missionCommanderNextActions)
	laneTakeoverPackage, err := laneTakeoverPackageFor(ctx.inst.CaseRoot, lane, executorAction, missionCommanderActionQueue, false)
	if err != nil {
		return "", nil, err
	}
	for _, line := range appendLaneTakeoverPackage(nil, laneTakeoverPackage) {
		fmt.Fprintln(&out, line)
	}
	writeExecutorActionSection(&out, executorAction)
	writeAutonomyProfileSection(&out, ctx.inst.CaseRoot, lane, ctx.manifest)
	writeVerificationSection(&out, facts.Verifications, lane.ID)
	writeDecisionSection(&out, facts.Decisions, lane.ID)
	writeReviewerWritebackItems(&out, ReviewerWritebackItems(facts, lane.ID))
	reviewerDispatchIntakeHandoffs, err := ReviewerDispatchIntakeHandoffs(ctx.inst.CaseRoot, facts, lane.ID)
	if err != nil {
		return "", nil, err
	}
	WriteReviewerDispatchIntakeHandoffSection(&out, "## Reviewer dispatch intake handoff", reviewerDispatchIntakeHandoffs)
	reviewerPacketRetirementHandoffs, err := ReviewerPacketRetirementHandoffs(ctx.inst.CaseRoot, lane.ID)
	if err != nil {
		return "", nil, err
	}
	WriteReviewerPacketRetirementHandoffSection(&out, "## Reviewer packet retirement handoff", reviewerPacketRetirementHandoffs)
	writePendingGateSection(&out, facts.Requests, lane.ID)
	writeAuthorizedGateSection(&out, facts.Requests, lane.ID)
	WriteAuthorizedGateAdapterHandoffSection(&out, "## Authorized gate adapter handoff", authorizedGateAdapterHandoffs)
	writeExecutionEvidenceReviewSection(&out, executionEvidenceReview, executionEvidenceReviewSummary)
	writeInterventionSection(&out, facts.Interventions, lane.ID)
	writeRollbackSection(&out, facts.Rollbacks, lane.ID)
	fmt.Fprintln(&out, "## 边界")
	fmt.Fprintln(&out)
	if lane.Authority {
		fmt.Fprintln(&out, "- 这是主线；可以维护最终结论、验证和长期 handoff。")
	} else {
		fmt.Fprintln(&out, "- 这是功能支线；只写自己的 workspace、证据、候选和 request。")
	}
	fmt.Fprintln(&out, "- 不并发写 IDB 注释/rename/type；不把完整 trace、disasm、decompile、dump 内容复制进 Markdown。")
	fmt.Fprintln(&out, "- autonomy profile 只授权 bounded heavy-action；不放宽 authority/confirmed/sync/promote。")
	return out.String(), writes, nil
}

func (ctx handoffContext) refreshResume(lane Lane, apply bool) (string, []StartWrite, error) {
	resumePath, checkpointPath, err := laneResumePaths(ctx.inst.CaseRoot, lane)
	if err != nil {
		return "", nil, err
	}
	actionPrefix := "would-"
	if apply {
		resumePath, checkpointPath, err = writeLaneResume(ctx.inst.CaseRoot, ctx.manifest, lane)
		if err != nil {
			return "", nil, err
		}
		actionPrefix = ""
	}
	writes := []StartWrite{
		{Path: relativePath(ctx.inst.CaseRoot, resumePath), Kind: "lane-resume", Action: actionPrefix + "refresh", TargetPath: resumePath},
		{Path: relativePath(ctx.inst.CaseRoot, checkpointPath), Kind: "lane-checkpoint", Action: actionPrefix + "refresh", TargetPath: checkpointPath},
	}
	return relativePath(ctx.inst.CaseRoot, resumePath), writes, nil
}

func (ctx handoffContext) projectHandoffPaths() (string, string, error) {
	stampPath, err := refsf.SafeJoin(ctx.handovers, ctx.stamp+".md")
	if err != nil {
		return "", "", err
	}
	latestPath, err := refsf.SafeJoin(ctx.handovers, "latest.md")
	if err != nil {
		return "", "", err
	}
	return stampPath, latestPath, nil
}

func (ctx handoffContext) laneHandoffPaths(laneID string) (string, string, error) {
	if err := validateLaneIDSegment(laneID); err != nil {
		return "", "", err
	}
	stampPath, err := refsf.SafeJoin(ctx.handovers, laneID+"-"+ctx.stamp+".md")
	if err != nil {
		return "", "", err
	}
	latestPath, err := refsf.SafeJoin(ctx.handovers, laneID+"-latest.md")
	if err != nil {
		return "", "", err
	}
	return stampPath, latestPath, nil
}

func (ctx handoffContext) replacementExecutorTakeoverPackageLatestRel() (string, error) {
	if ctx.project || ctx.lane == nil {
		return stateRelPath(ctx.inst.CaseRoot, "handovers", "latest-replacement-executor-takeover.json")
	}
	return stateRelPath(ctx.inst.CaseRoot, "handovers", ctx.lane.ID+"-latest-replacement-executor-takeover.json")
}

func (ctx handoffContext) replacementExecutorTakeoverPackagePaths() (string, string, error) {
	if ctx.project || ctx.lane == nil {
		stampPath, err := refsf.SafeJoin(ctx.handovers, ctx.stamp+"-replacement-executor-takeover.json")
		if err != nil {
			return "", "", err
		}
		latestPath, err := refsf.SafeJoin(ctx.handovers, "latest-replacement-executor-takeover.json")
		if err != nil {
			return "", "", err
		}
		return stampPath, latestPath, nil
	}
	if err := validateLaneIDSegment(ctx.lane.ID); err != nil {
		return "", "", err
	}
	stampPath, err := refsf.SafeJoin(ctx.handovers, ctx.lane.ID+"-"+ctx.stamp+"-replacement-executor-takeover.json")
	if err != nil {
		return "", "", err
	}
	latestPath, err := refsf.SafeJoin(ctx.handovers, ctx.lane.ID+"-latest-replacement-executor-takeover.json")
	if err != nil {
		return "", "", err
	}
	return stampPath, latestPath, nil
}

func (ctx handoffContext) replacementExecutorTakeoverPackageArtifactWrites(apply bool, pkg *mission.ReplacementExecutorTakeoverPackage) ([]StartWrite, error) {
	if pkg == nil || !pkg.Ready {
		return nil, nil
	}
	stampPath, latestPath, err := ctx.replacementExecutorTakeoverPackagePaths()
	if err != nil {
		return nil, err
	}
	prefix := "would-"
	if apply {
		prefix = ""
	}
	return []StartWrite{
		{Path: relativePath(ctx.inst.CaseRoot, stampPath), Kind: "replacement-executor-takeover-package", Action: prefix + "write-replacement-executor-takeover-package", TargetPath: stampPath},
		{Path: relativePath(ctx.inst.CaseRoot, latestPath), Kind: "replacement-executor-takeover-package", Action: prefix + "write-latest-replacement-executor-takeover-package", TargetPath: latestPath},
	}, nil
}

func readBoard(caseRoot string) (board, error) {
	return mission.ReadBoard(caseRoot)
}

func ResolveHandoffLaneID(caseRoot, selector string) (string, error) {
	b, err := readBoard(caseRoot)
	if err != nil {
		return "", err
	}
	lane, err := resolveHandoffLane(caseRoot, b, selector)
	if err != nil {
		return "", err
	}
	return strings.TrimSpace(lane.ID), nil
}

func resolveHandoffLane(caseRoot string, b board, selector string) (Lane, error) {
	raw := strings.TrimSpace(selector)
	if raw == "" {
		return Lane{}, fmt.Errorf("handoff selector is empty")
	}
	normalized := strings.ToLower(raw)
	candidates := []string{}
	if normalized == "main" {
		candidates = append(candidates, b.DefaultAuthorityLane, raw)
	} else {
		candidates = append(candidates, raw)
		if !strings.HasPrefix(normalized, "feature-") {
			candidates = append(candidates, "feature-"+raw)
		}
	}
	for _, candidate := range candidates {
		for _, lane := range b.Lanes {
			if strings.EqualFold(lane.ID, candidate) {
				return readLaneByID(caseRoot, lane.ID)
			}
		}
	}
	for _, lane := range b.Lanes {
		full, err := readLaneByID(caseRoot, lane.ID)
		if os.IsNotExist(err) {
			continue
		}
		if err != nil {
			return Lane{}, err
		}
		if strings.EqualFold(full.Name, raw) {
			return full, nil
		}
	}
	return Lane{}, fmt.Errorf("unknown workstream selector: %s; available: %s", selector, availableLaneLabels(caseRoot, b))
}

func availableLaneLabels(caseRoot string, b board) string {
	labels := []string{}
	for _, lane := range b.Lanes {
		full, err := readLaneByID(caseRoot, lane.ID)
		if err != nil {
			labels = append(labels, lane.ID)
			continue
		}
		labels = append(labels, workstreamLabel(full))
	}
	sort.Strings(labels)
	return strings.Join(labels, ", ")
}

func readLaneByID(caseRoot, laneID string) (Lane, error) {
	if err := validateLaneIDSegment(laneID); err != nil {
		return Lane{}, err
	}
	path, err := projectstate.Join(caseRoot, "lanes", laneID, "lane.json")
	if err != nil {
		return Lane{}, err
	}
	lane, err := readLane(path)
	if err != nil {
		return Lane{}, err
	}
	if !strings.EqualFold(lane.ID, laneID) {
		return Lane{}, fmt.Errorf("lane id mismatch for %s: lane.json declares %s", laneID, lane.ID)
	}
	return lane, nil
}

func laneResumePaths(caseRoot string, lane Lane) (string, string, error) {
	laneRoot, err := laneRootPath(caseRoot, lane)
	if err != nil {
		return "", "", err
	}
	return filepath.Join(laneRoot, "prompts", "RESUME.md"), filepath.Join(laneRoot, "checkpoints", "latest.json"), nil
}

func latestRunDigest(caseRoot string) (string, error) {
	runs, err := projectstate.Join(caseRoot, "runs")
	if err != nil {
		return "", err
	}
	entries, err := os.ReadDir(runs)
	if os.IsNotExist(err) {
		return "", nil
	}
	if err != nil {
		return "", err
	}
	names := []string{}
	for _, entry := range entries {
		if entry.IsDir() {
			names = append(names, entry.Name())
		}
	}
	sort.Sort(sort.Reverse(sort.StringSlice(names)))
	for _, name := range names {
		digest := filepath.Join(runs, name, "digest.md")
		if refsf.Exists(digest) {
			return relativePath(caseRoot, digest), nil
		}
	}
	return "", nil
}

func latestDriverReceiptHandoffFor(caseRoot string, lane *Lane) (*LatestDriverReceiptHandoff, error) {
	runs, err := projectstate.Join(caseRoot, "runs")
	if err != nil {
		return nil, err
	}
	entries, err := os.ReadDir(runs)
	if os.IsNotExist(err) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	names := []string{}
	for _, entry := range entries {
		if entry.IsDir() {
			names = append(names, entry.Name())
		}
	}
	sort.Sort(sort.Reverse(sort.StringSlice(names)))
	for _, name := range names {
		statusPath := filepath.Join(runs, name, "status.json")
		b, err := os.ReadFile(statusPath)
		if os.IsNotExist(err) {
			continue
		}
		if err != nil {
			return nil, err
		}
		var status struct {
			RunID                         string                         `json:"runId"`
			BatchID                       string                         `json:"batchId"`
			MissionCommanderDriverReceipt *MissionCommanderDriverReceipt `json:"missionCommanderDriverReceipt"`
		}
		if err := json.Unmarshal(b, &status); err != nil {
			continue
		}
		receipt := status.MissionCommanderDriverReceipt
		if receipt == nil || strings.TrimSpace(receipt.RunID) == "" || strings.TrimSpace(receipt.RunStatusPath) == "" || strings.TrimSpace(receipt.RunDigestPath) == "" {
			continue
		}
		if lane != nil && strings.TrimSpace(receipt.Lane) != strings.TrimSpace(lane.ID) {
			continue
		}
		return &LatestDriverReceiptHandoff{
			Ready:                         true,
			State:                         "latest-driver-receipt-ready",
			RunID:                         firstText(receipt.RunID, status.RunID, name),
			BatchID:                       firstText(receipt.BatchID, status.BatchID),
			Lane:                          receipt.Lane,
			Command:                       receipt.Command,
			RunStatusPath:                 receipt.RunStatusPath,
			RunDigestPath:                 receipt.RunDigestPath,
			MissionCommanderDriverReceipt: receipt,
			TargetDocuments: mission.UniqueStrings([]string{
				"latestDriverReceiptHandoff",
				receipt.RunStatusPath,
				receipt.RunDigestPath,
			}),
			Boundary: []string{
				"latest driver receipt handoff is read-only evidence from the latest matching continue -Apply run artifact",
				"use this receipt to verify the previously consumed driver request before choosing follow-up work",
				"driver receipt does not prove the Go runtime spawned, polled, stopped, or managed an external session",
				"absence or lane mismatch means refresh status and inspect currentDriverRequest; do not infer completion from stale handoff text",
			},
		}, nil
	}
	return nil, nil
}

func writeWorkspacePackets(out *bytes.Buffer, caseRoot string, lane Lane) error {
	workspace, err := refsf.SafeJoin(caseRoot, lane.Workspace)
	if err != nil {
		return err
	}
	entries, err := os.ReadDir(workspace)
	if os.IsNotExist(err) {
		return nil
	}
	if err != nil {
		return err
	}
	packets := []string{}
	for _, entry := range entries {
		if entry.IsDir() || !strings.EqualFold(filepath.Ext(entry.Name()), ".md") {
			continue
		}
		packets = append(packets, filepath.Join(workspace, entry.Name()))
	}
	sort.Strings(packets)
	if len(packets) == 0 {
		return nil
	}
	fmt.Fprintln(out, "## workspace packet")
	fmt.Fprintln(out)
	for _, packet := range packets {
		fmt.Fprintf(out, "- `%s`：workspace 产物，含 evidence/candidate/decision packet。\n", relativePath(caseRoot, packet))
	}
	fmt.Fprintln(out)
	return nil
}

func (ctx handoffContext) projectExecutionEvidenceReview() []ExecutionEvidenceReviewItem {
	facts, err := readHandoffFacts(ctx.inst.CaseRoot)
	if err != nil {
		return nil
	}
	items := ExecutionEvidenceReviewItemsWithLedgerFacts(facts, "", ctx.laneCommandLabel)
	lanes := ctx.currentLanes()
	return bindExecutionEvidenceReviewContinueCommands(items, func(command string) (Lane, bool) {
		label, ok := strings.CutPrefix(strings.TrimSpace(command), "/rekit handoff ")
		if !ok {
			return Lane{}, false
		}
		for _, lane := range lanes {
			if workstreamLabel(lane) == strings.TrimSpace(label) {
				return lane, true
			}
		}
		return Lane{}, false
	})
}

func (ctx handoffContext) executionEvidenceReview(lane Lane) []ExecutionEvidenceReviewItem {
	facts, err := readHandoffFacts(ctx.inst.CaseRoot)
	if err != nil {
		return nil
	}
	return bindExecutionEvidenceReviewContinueCommands(laneExecutionEvidenceReview(lane, facts), func(string) (Lane, bool) {
		return lane, true
	})
}

func (ctx handoffContext) laneCommandLabel(laneID string) string {
	lane, err := readLaneByID(ctx.inst.CaseRoot, laneID)
	if err == nil {
		return workstreamLabel(lane)
	}
	return mission.BoardLaneLabel(mission.BoardLane{ID: laneID})
}

func readHandoffFacts(caseRoot string) (mission.LedgerFacts, error) {
	return mission.ReadLedgerFacts(caseRoot)
}

func projectMissionBrief(lanes []boardLane, facts mission.LedgerFacts) mission.Brief {
	return mission.BuildWithOptions(mission.BoardLanes(lanes), facts.Facts, mission.BuildOptions{
		MaxRows:            maxHandoffRows,
		OpenDecisionAction: "review open candidate/decision item(s) with evidence and authority boundary",
	})
}

func writeProjectMissionBrief(out *bytes.Buffer, lanes []boardLane, facts mission.LedgerFacts, currentLanes []Lane) {
	brief := bindMissionBriefContinueCommands(projectMissionBrief(lanes, facts), currentLanes)
	fmt.Fprintln(out, "## Mission Control brief")
	fmt.Fprintln(out)
	fmt.Fprintf(out, "- summary: %s\n", brief.Summary)
	writeHandoffBriefList(out, "ready lanes", brief.ReadyLanes)
	writeHandoffBriefList(out, "blocked lanes", brief.BlockedLanes)
	writeHandoffBriefList(out, "pending gates", brief.PendingGates)
	writeHandoffBriefList(out, "authorized gates", brief.AuthorizedGates)
	writeHandoffBriefList(out, "open decisions", brief.OpenDecisions)
	writeHandoffBriefList(out, "interventions", brief.Interventions)
	writeHandoffBriefList(out, "next agent actions", brief.NextAgentActions)
	writeHandoffBriefList(out, "escalations", brief.Escalations)
	fmt.Fprintln(out)
}

func laneMissionBrief(lane Lane, facts mission.LedgerFacts) mission.Brief {
	brief := mission.BuildWithOptions([]mission.Lane{{ID: lane.ID, Label: workstreamLabel(lane), Status: lane.Status}}, mission.LaneFacts(facts.Facts, lane.ID), mission.BuildOptions{
		MaxRows:            maxHandoffRows,
		OpenDecisionAction: "review open candidate/decision item(s) with evidence and authority boundary",
	})
	return bindMissionBriefContinueCommands(brief, []Lane{lane})
}

func writeLaneMissionBrief(out *bytes.Buffer, lane Lane, facts mission.LedgerFacts, action laneExecutorAction) {
	laneFacts := mission.LaneFacts(facts.Facts, lane.ID)
	gates := mission.FilterLane(laneFacts.Requests, lane.ID, "pending-gate")
	authorizedGates := mission.FilterLane(laneFacts.Requests, lane.ID, "authorized-gate")
	interventions := mission.EffectiveOpenInterventions(laneFacts.Interventions)
	openDecisions := mission.OpenDecisionItems(laneFacts)
	fmt.Fprintln(out, "## Mission Control brief")
	fmt.Fprintln(out)
	fmt.Fprintf(out, "- lane: %s status=%s workspace=%s\n", lane.ID, lane.Status, lane.Workspace)
	fmt.Fprintf(out, "- blocked: %t\n", action.Blocked)
	writeHandoffBriefList(out, "pending-gate", missionLines(gates, mission.LaneGateLine))
	writeHandoffBriefList(out, "authorized-gate", missionLines(authorizedGates, mission.LaneGateLine))
	writeHandoffBriefList(out, "open intervention", missionLines(interventions, mission.LaneInterventionLine))
	writeHandoffBriefList(out, "open decision", missionLines(openDecisions, mission.LaneOpenDecisionLine))
	writeHandoffBriefList(out, "next agent action", action.NextAgentActions)
	fmt.Fprintln(out)
}

func writeExecutorActionSection(out *bytes.Buffer, action laneExecutorAction) {
	fmt.Fprintln(out, "## Executor action snapshot")
	fmt.Fprintln(out)
	fmt.Fprintf(out, "- blocked: `%t`\n", action.Blocked)
	fmt.Fprintf(out, "- ready: `%t`\n", action.Ready)
	fmt.Fprintf(out, "- pending gates: `%d`\n", action.PendingGates)
	fmt.Fprintf(out, "- open interventions: `%d`\n", action.OpenInterventions)
	fmt.Fprintf(out, "- open decisions: `%d`\n", action.OpenDecisions)
	fmt.Fprintf(out, "- reconcile required: `%t`\n", action.ReconcileRequired)
	fmt.Fprintf(out, "- pending gate required: `%t`\n", action.PendingGateRequired)
	fmt.Fprintf(out, "- open decision required: `%t`\n", action.OpenDecisionRequired)
	fmt.Fprintf(out, "- resume command: `%s`\n", action.ResumeCommand)
	fmt.Fprintf(out, "- handoff command: `%s`\n", action.HandoffCommand)
	fmt.Fprintf(out, "- commander state: `%s`\n", action.MissionCommanderAction.State)
	fmt.Fprintf(out, "- commander prompt: %s\n", action.MissionCommanderAction.Prompt)
	fmt.Fprintf(out, "- commander primary command: `%s`\n", action.MissionCommanderAction.PrimaryCommand)
	writeHandoffBriefList(out, "commander follow-up commands", action.MissionCommanderAction.FollowUpCommands)
	writeHandoffBriefList(out, "commander boundary", action.MissionCommanderAction.Boundary)
	writeHandoffBriefList(out, "blocker reasons", action.BlockerReasons)
	writeHandoffBriefList(out, "executor next actions", action.NextAgentActions)
	writeHandoffBriefList(out, "executor escalations", action.Escalations)
	fmt.Fprintln(out)
}

func writeAutonomyProfileSection(out *bytes.Buffer, caseRoot string, lane Lane, m *manifest.Manifest) {
	summary := autonomy.ReadSummary(caseRoot, lane.ID, m)
	fmt.Fprintln(out, "## autonomy profile")
	fmt.Fprintln(out)
	fmt.Fprintf(out, "- mode=%s ready=%t valid=%t expired=%t profile=`%s`\n", summary.Mode, summary.Ready, summary.Valid, summary.Expired, summary.ProfilePath)
	fmt.Fprintf(out, "- allowedActions=%s deniedActions=%s outputPaths=%s\n", firstText(strings.Join(summary.AllowedActions, ","), "none"), firstText(strings.Join(summary.DeniedActions, ","), "none"), firstText(strings.Join(summary.OutputPaths, ","), "none"))
	fmt.Fprintf(out, "- recordRequired=%t notifyMainOn=%s\n", summary.RecordRequired, firstText(strings.Join(summary.NotifyMainOn, ","), "none"))
	if strings.TrimSpace(summary.Error) != "" {
		fmt.Fprintf(out, "- error=%s\n", summary.Error)
	}
	fmt.Fprintln(out)
}

func missionLines(items []map[string]any, line func(map[string]any) string) []string {
	lines := make([]string, 0, len(items))
	for _, item := range items {
		lines = append(lines, line(item))
	}
	return lines
}

func writeHandoffBriefList(out *bytes.Buffer, label string, items []string) {
	if len(items) == 0 {
		fmt.Fprintf(out, "- %s: none\n", label)
		return
	}
	fmt.Fprintf(out, "- %s:\n", label)
	for _, item := range mission.LimitStrings(items, maxHandoffRows) {
		fmt.Fprintf(out, "  - %s\n", item)
	}
}

func writeVerificationSection(out *bytes.Buffer, verifications []map[string]any, laneID string) {
	items := filterLane(verifications, laneID, "")
	if len(items) == 0 {
		return
	}
	fmt.Fprintln(out, "## verification")
	fmt.Fprintln(out)
	for _, v := range lastObjects(items, maxHandoffRows) {
		subj := firstObjectText(v, "subject", "kind")
		by := firstObjectText(v, "actor")
		byTag := ""
		if by != "" {
			byTag = " | by=" + by
		}
		reviewerTag := ""
		if reviewer := firstObjectText(v, "reviewerSession"); reviewer != "" {
			reviewerTag = " | reviewerSession=" + reviewer
		}
		fmt.Fprintf(out, "- %s | verifier=%s | verdict=%s | target=%s%s%s%s\n", subj, firstObjectText(v, "verifier"), firstObjectText(v, "verdict"), firstObjectText(v, "target"), byTag, reviewerTag, batchTag(v))
		if item, ok := reviewerWritebackItem("verification", v); ok {
			writeReviewerWritebackEventDetail(out, "  ", item)
		}
	}
	fmt.Fprintln(out)
}

func writeDecisionSection(out *bytes.Buffer, decisions []map[string]any, laneID string) {
	items := filterLane(decisions, laneID, "")
	if len(items) == 0 {
		return
	}
	fmt.Fprintln(out, "## decision")
	fmt.Fprintln(out)
	for _, d := range lastObjects(items, maxHandoffRows) {
		subj := firstObjectText(d, "subject", "kind")
		dec := firstObjectText(d, "decision", "action")
		by := firstObjectText(d, "confirmedBy", "actor")
		byTag := ""
		if by != "" {
			byTag = " | by=" + by
		}
		reviewerTag := ""
		if reviewer := firstObjectText(d, "reviewerSession"); reviewer != "" {
			reviewerTag = " | reviewerSession=" + reviewer
		}
		fmt.Fprintf(out, "- %s | decision=%s%s%s | reason=%s\n", subj, dec, byTag, reviewerTag, firstObjectText(d, "reason"))
		if item, ok := reviewerWritebackItem("decision", d); ok {
			writeReviewerWritebackEventDetail(out, "  ", item)
		}
	}
	fmt.Fprintln(out)
}

func writePendingGateSection(out *bytes.Buffer, requests []map[string]any, laneID string) {
	items := filterLane(requests, laneID, "pending-gate")
	if len(items) == 0 {
		return
	}
	fmt.Fprintln(out, "## pending-gate")
	fmt.Fprintln(out)
	for _, g := range lastObjects(items, maxHandoffRows) {
		fmt.Fprintf(out, "- %s | %s%s\n", firstObjectText(g, "subject"), firstObjectText(g, "summary"), gateRequestDetail(g, true, true))
	}
	fmt.Fprintln(out)
}

func writeAuthorizedGateSection(out *bytes.Buffer, requests []map[string]any, laneID string) {
	items := filterLane(requests, laneID, "authorized-gate")
	if len(items) == 0 {
		return
	}
	fmt.Fprintln(out, "## authorized-gate")
	fmt.Fprintln(out)
	for _, g := range lastObjects(items, maxHandoffRows) {
		fmt.Fprintf(out, "- %s | %s%s\n", firstObjectText(g, "subject"), firstObjectText(g, "summary"), gateRequestDetail(g, true, true))
	}
	fmt.Fprintln(out)
}

func writeExecutionEvidenceReviewSection(out *bytes.Buffer, items []ExecutionEvidenceReviewItem, summary ExecutionEvidenceReviewSummary) {
	if len(items) == 0 {
		return
	}
	fmt.Fprintln(out, "## execution evidence review")
	fmt.Fprintln(out)
	writeExecutionEvidenceReviewSummary(out, summary)
	for _, item := range items {
		fmt.Fprintf(out, "- %s | status=%s | gateEventId=%s | action=%s | outputRefs=%s | evidenceRefs=%s\n", firstText(item.Subject, item.Summary, item.EventID), item.Status, item.GateEventID, item.Action, firstText(strings.Join(item.OutputRefs, ","), "none"), firstText(strings.Join(item.EvidenceRefs, ","), "none"))
		writeExecutionEvidenceBoundaryDetail(out, item)
		writeExecutionEvidenceReportDetail(out, item)
		writeExecutionEvidenceAcknowledgement(out, item)
		fmt.Fprintf(out, "  - review command: `%s`\n", item.ReviewCommand)
		fmt.Fprintf(out, "  - handoff command: `%s`\n", item.HandoffCommand)
		for idx, step := range item.ReviewRunbookSteps {
			fmt.Fprintf(out, "  - review runbook: step=%d text=%s\n", idx+1, step)
		}
		fmt.Fprintf(out, "  - commander state: %s\n", item.MissionCommanderAction.State)
		fmt.Fprintf(out, "  - commander primary: `%s`\n", item.MissionCommanderAction.PrimaryCommand)
		writeExecutionEvidenceFollowThrough(out, item.FollowThrough)
		writeHandoffBriefList(out, "commander follow-up", item.MissionCommanderAction.FollowUpCommands)
		writeHandoffBriefList(out, "review boundary", item.Boundary)
	}
	fmt.Fprintln(out)
}

func writeExecutionEvidenceReviewSummary(out *bytes.Buffer, summary ExecutionEvidenceReviewSummary) {
	if summary.Total == 0 {
		return
	}
	fmt.Fprintf(out, "- summary: total=%d readyForReview=%d mainEscalations=%d duplicates=%d outputRefs=%d evidenceRefs=%d boundaryHits=%d latestEventId=%s gateEventId=%s status=%s action=%s\n", summary.Total, summary.ReadyForReviewCount, summary.MainEscalationCount, summary.DuplicateCount, summary.OutputRefCount, summary.EvidenceRefCount, summary.BoundaryHitCount, summary.LatestEventID, summary.LatestGateEventID, summary.LatestStatus, firstText(summary.LatestAction, "none"))
	if strings.TrimSpace(summary.CurrentAction) != "" {
		fmt.Fprintf(out, "  - summary current action: `%s`\n", summary.CurrentAction)
	}
	if strings.TrimSpace(summary.ActionQueueSummary) != "" {
		fmt.Fprintf(out, "  - summary action queue: %s\n", summary.ActionQueueSummary)
	}
	if strings.TrimSpace(summary.LatestReviewCommand) != "" || strings.TrimSpace(summary.LatestHandoffCommand) != "" {
		fmt.Fprintf(out, "  - summary handoff: review=`%s` handoff=`%s`\n", summary.LatestReviewCommand, summary.LatestHandoffCommand)
	}
	if strings.TrimSpace(summary.LatestCommanderState) != "" || strings.TrimSpace(summary.LatestCommanderPrimary) != "" {
		fmt.Fprintf(out, "  - summary commander: state=%s primary=`%s`\n", summary.LatestCommanderState, summary.LatestCommanderPrimary)
	}
	if strings.TrimSpace(summary.LatestExecutionReportPath) != "" || strings.TrimSpace(summary.LatestExecutionReportSHA256) != "" || strings.TrimSpace(summary.LatestAdapterID) != "" || strings.TrimSpace(summary.LatestAdapterStatus) != "" {
		fmt.Fprintf(out, "  - summary report: path=%s sha256=%s adapterId=%s adapterStatus=%s\n", firstText(summary.LatestExecutionReportPath, "none"), firstText(summary.LatestExecutionReportSHA256, "none"), firstText(summary.LatestAdapterID, "none"), firstText(summary.LatestAdapterStatus, "none"))
	}
	writeExecutionEvidenceAdapterContext(out, summary.LatestAdapterContext)
	for _, hit := range mission.LimitStrings(summary.LatestBoundaryHits, maxHandoffRows) {
		fmt.Fprintf(out, "  - summary latest boundary hit: %s\n", hit)
	}
	if strings.TrimSpace(summary.LatestEscalation) != "" {
		fmt.Fprintf(out, "  - summary latest escalation: %s\n", summary.LatestEscalation)
	}
	if strings.TrimSpace(summary.FollowThroughState) != "" || summary.OutcomeCount > 0 {
		fmt.Fprintf(out, "  - summary follow-through: state=%s outcomes=%d\n", summary.FollowThroughState, summary.OutcomeCount)
	}
	for _, boundary := range mission.LimitStrings(summary.Boundary, maxHandoffRows) {
		fmt.Fprintf(out, "  - summary boundary: %s\n", boundary)
	}
}

func writeExecutionEvidenceBoundaryDetail(out *bytes.Buffer, item ExecutionEvidenceReviewItem) {
	for _, hit := range mission.LimitStrings(item.BoundaryHits, maxHandoffRows) {
		fmt.Fprintf(out, "  - boundary hit: %s\n", hit)
	}
	if strings.TrimSpace(item.Escalation) != "" {
		fmt.Fprintf(out, "  - escalation: %s\n", item.Escalation)
	}
}

func writeExecutionEvidenceAcknowledgement(out *bytes.Buffer, item ExecutionEvidenceReviewItem) {
	ack := item.Acknowledgement
	if ack == nil {
		return
	}
	fmt.Fprintf(out, "  - acknowledgement: state=%s acceptedPreview=`%s` rejectedPreview=`%s` record=%s related=%s evidenceRefs=%s\n", ack.State, ack.AcceptedPreviewCommand, ack.RejectedPreviewCommand, ack.RecordCommand, strings.Join(ack.Related, ","), strings.Join(ack.EvidenceRefs, ","))
	for _, boundary := range mission.LimitStrings(ack.Boundary, maxHandoffRows) {
		fmt.Fprintf(out, "    - acknowledgement boundary: %s\n", boundary)
	}
}

func writeExecutionEvidenceFollowThrough(out *bytes.Buffer, follow mission.ExecutionEvidenceFollowThrough) {
	if strings.TrimSpace(follow.State) == "" && len(follow.Outcomes) == 0 {
		return
	}
	fmt.Fprintf(out, "  - follow-through: state=%s gateEventId=%s outcomes=%d\n", follow.State, follow.GateEventID, len(follow.Outcomes))
	for _, outcome := range follow.Outcomes {
		fmt.Fprintf(out, "    - outcome: name=%s state=%s command=`%s` expected=%s\n", outcome.Name, outcome.State, outcome.Command, outcome.Expected)
		if strings.TrimSpace(outcome.When) != "" {
			fmt.Fprintf(out, "      - when: %s\n", outcome.When)
		}
		for _, action := range mission.LimitStrings(outcome.Actions, maxHandoffRows) {
			fmt.Fprintf(out, "      - action: %s\n", action)
		}
		for _, command := range mission.LimitStrings(outcome.VerificationCommands, maxHandoffRows) {
			fmt.Fprintf(out, "      - verification: %s\n", command)
		}
		for _, evidence := range mission.LimitStrings(outcome.Evidence, maxHandoffRows) {
			fmt.Fprintf(out, "      - evidence: %s\n", evidence)
		}
	}
	if strings.TrimSpace(follow.ActionQueue.Summary) != "" {
		fmt.Fprintf(out, "    - queue: %s\n", follow.ActionQueue.Summary)
	}
}

func writeExecutionEvidenceReportDetail(out *bytes.Buffer, item ExecutionEvidenceReviewItem) {
	if strings.TrimSpace(item.ExecutionReportPath) != "" || strings.TrimSpace(item.ExecutionReportSHA256) != "" {
		fmt.Fprintf(out, "  - execution report: %s sha256=%s\n", firstText(item.ExecutionReportPath, "none"), firstText(item.ExecutionReportSHA256, "none"))
	}
	if item.ActualBudget != nil {
		fmt.Fprintf(out, "  - actual budget: runtimeSeconds=%d diskMB=%d requests=%d\n", item.ActualBudget.RuntimeSeconds, item.ActualBudget.DiskMB, item.ActualBudget.Requests)
	}
	if strings.TrimSpace(item.AdapterID) != "" || strings.TrimSpace(item.AdapterStatus) != "" {
		fmt.Fprintf(out, "  - adapter report: adapterId=%s status=%s\n", item.AdapterID, item.AdapterStatus)
	}
	writeExecutionEvidenceReceiptDetail(out, item)
	writeExecutionEvidenceAdapterContext(out, item.AdapterContext)
}

func writeExecutionEvidenceReceiptDetail(out *bytes.Buffer, item ExecutionEvidenceReviewItem) {
	if strings.TrimSpace(item.AdapterExecutionDispatchPath) != "" || strings.TrimSpace(item.AdapterExecutionDispatchSHA256) != "" {
		fmt.Fprintf(out, "  - dispatch: id=%s path=%s sha256=%s\n", firstText(item.AdapterExecutionDispatchID, "none"), firstText(item.AdapterExecutionDispatchPath, "none"), firstText(item.AdapterExecutionDispatchSHA256, "none"))
	}
	if strings.TrimSpace(item.AdapterExecutionReceiptPath) != "" || strings.TrimSpace(item.AdapterExecutionReceiptSHA256) != "" {
		fmt.Fprintf(out, "  - receipt: path=%s sha256=%s\n", firstText(item.AdapterExecutionReceiptPath, "none"), firstText(item.AdapterExecutionReceiptSHA256, "none"))
	}
	if strings.TrimSpace(item.CurrentExecutor) != "" || item.ExecutorGeneration > 0 || strings.TrimSpace(item.AdapterHarness) != "" || strings.TrimSpace(item.AdapterSession) != "" {
		fmt.Fprintf(out, "  - execution owner: executor=%s generation=%d harness=%s session=%s\n", item.CurrentExecutor, item.ExecutorGeneration, item.AdapterHarness, item.AdapterSession)
	}
	if strings.TrimSpace(item.ToolingCatalogSHA256) != "" || item.AdapterExecutionArtifactCount > 0 {
		fmt.Fprintf(out, "  - tooling provenance: catalogSha256=%s artifacts=%d\n", item.ToolingCatalogSHA256, item.AdapterExecutionArtifactCount)
	}
}

func writeExecutionEvidenceAdapterContext(out *bytes.Buffer, context *mission.ExecutionEvidenceAdapterContext) {
	if context == nil {
		return
	}
	fmt.Fprintf(out, "  - adapter context: id=%s status=%s entry=%s gateActions=%s recordOnlyAfterGate=%t toolingCatalogPath=%s\n", context.ID, context.Status, context.Entry, strings.Join(context.GateActions, ","), context.RecordOnlyAfterGate, context.ToolingCatalogPath)
	if strings.TrimSpace(context.Purpose) != "" {
		fmt.Fprintf(out, "  - adapter context purpose: %s\n", context.Purpose)
	}
	if len(context.SideEffects) > 0 {
		fmt.Fprintf(out, "  - adapter context side effects: %s\n", strings.Join(context.SideEffects, ","))
	}
	for _, guidance := range mission.LimitStrings(context.ReportGuidance, maxHandoffRows) {
		fmt.Fprintf(out, "  - adapter context report guidance: %s\n", guidance)
	}
	for _, guidance := range mission.LimitStrings(context.EvidenceGuidance, maxHandoffRows) {
		fmt.Fprintf(out, "  - adapter context evidence guidance: %s\n", guidance)
	}
	if len(context.StopConditionHints) > 0 {
		fmt.Fprintf(out, "  - adapter context stop conditions: %s\n", strings.Join(context.StopConditionHints, ","))
	}
}
func writeInterventionSection(out *bytes.Buffer, interventions []map[string]any, laneID string) {
	items := filterLane(interventions, laneID, "")
	if len(items) == 0 {
		return
	}
	fmt.Fprintln(out, "## intervention")
	fmt.Fprintln(out)
	for _, i := range lastObjects(items, maxHandoffRows) {
		batchTag := batchTag(i)
		fmt.Fprintf(out, "- %s | action=%s | target=%s | approvedBy=%s | scope=%s | status=%s%s\n", firstObjectText(i, "subject", "action"), firstObjectText(i, "action"), firstObjectText(i, "target"), firstObjectText(i, "approvedBy"), firstObjectText(i, "scope"), firstObjectText(i, "status"), batchTag)
	}
	fmt.Fprintln(out)
}

func writeRollbackSection(out *bytes.Buffer, rollbacks []map[string]any, laneID string) {
	items := filterLane(rollbacks, laneID, "")
	if len(items) == 0 {
		return
	}
	fmt.Fprintln(out, "## rollback")
	fmt.Fprintln(out)
	for _, r := range lastObjects(items, maxHandoffRows) {
		batchTag := batchTag(r)
		fmt.Fprintf(out, "- %s | target=%s | status=%s%s | reason=%s\n", firstObjectText(r, "subject", "kind"), firstObjectText(r, "target"), firstObjectText(r, "status"), batchTag, firstObjectText(r, "reason"))
	}
	fmt.Fprintln(out)
}

func filterLane(items []map[string]any, laneID, status string) []map[string]any {
	out := []map[string]any{}
	for _, item := range items {
		if firstObjectText(item, "lane") != laneID {
			continue
		}
		if status != "" && firstObjectText(item, "status") != status {
			continue
		}
		out = append(out, item)
	}
	return out
}

func gateRequestDetail(e map[string]any, omitStatus, omitBatch bool) string {
	parts := []string{}
	add := func(key, value string) {
		if strings.TrimSpace(value) != "" {
			parts = append(parts, key+"="+value)
		}
	}
	if !omitStatus {
		add("status", firstObjectText(e, "status"))
	}
	add("by", firstObjectText(e, "actor"))
	add("risk", firstObjectText(e, "risk"))
	add("target", firstObjectText(e, "target"))
	if !omitBatch {
		add("batch", firstObjectText(e, "batchId"))
	}
	if gate, ok := e["gate"].(map[string]any); ok {
		add("action", firstObjectText(gate, "action"))
		add("scope", firstObjectText(gate, "scope"))
		add("budget", firstObjectText(gate, "budget"))
		add("tried", firstObjectText(gate, "triedLightSteps"))
		add("stop", firstObjectText(gate, "stopConditions"))
		if auth, ok := gate["authorization"].(map[string]any); ok {
			add("auth", firstObjectText(auth, "decision"))
			add("profile", firstObjectText(auth, "profileId"))
		}
	}
	if len(parts) == 0 {
		return ""
	}
	return " | " + strings.Join(parts, " | ")
}

func batchTag(e map[string]any) string {
	batch := firstObjectText(e, "batchId")
	if batch == "" {
		return ""
	}
	return " | batch=" + batch
}

func writeText(path, text string) error {
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return err
	}
	return os.WriteFile(path, []byte(text), 0o644)
}

func handoffTimestamp() string {
	return strings.ReplaceAll(time.Now().UTC().Format("20060102-150405.000"), ".", "")
}

func isoNow() string {
	return time.Now().UTC().Format(time.RFC3339Nano)
}
