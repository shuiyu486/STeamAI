package workstream

import (
	"crypto/sha256"
	"encoding/csv"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"maps"
	"os"
	"path/filepath"
	"slices"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/shuiyu486/re-context-kits/internal/rekit/autonomy"
	refsf "github.com/shuiyu486/re-context-kits/internal/rekit/fs"
	"github.com/shuiyu486/re-context-kits/internal/rekit/instance"
	"github.com/shuiyu486/re-context-kits/internal/rekit/manifest"
	"github.com/shuiyu486/re-context-kits/internal/rekit/mission"
)

const continuePreviewRunID = "run-preview"

type ContinueOptions struct {
	Selector                   string
	Executor                   string
	ExpectedExecutorGeneration int
}

type ContinueResult struct {
	SchemaVersion                  int                                      `json:"schemaVersion"`
	Command                        string                                   `json:"command"`
	CaseRoot                       string                                   `json:"caseRoot"`
	RepoRoot                       string                                   `json:"repoRoot"`
	Pack                           string                                   `json:"pack"`
	IsMutation                     bool                                     `json:"isMutation"`
	Applied                        bool                                     `json:"applied"`
	RequiresConfirmation           bool                                     `json:"requiresConfirmation"`
	Selector                       string                                   `json:"selector"`
	Lane                           Lane                                     `json:"lane"`
	AutonomyProfile                autonomy.Summary                         `json:"autonomyProfile"`
	RunID                          string                                   `json:"runId"`
	BatchID                        string                                   `json:"batchId"`
	Summary                        ContinueSummary                          `json:"summary"`
	MissionBrief                   mission.Brief                            `json:"missionBrief"`
	ExecutorAction                 laneExecutorAction                       `json:"executorAction"`
	ExecutionEvidenceReview        []ExecutionEvidenceReviewItem            `json:"executionEvidenceReview,omitempty"`
	ExecutionEvidenceReviewSummary ExecutionEvidenceReviewSummary           `json:"executionEvidenceReviewSummary"`
	ReviewerWritebacks             []ReviewerWritebackItem                  `json:"reviewerWritebacks,omitempty"`
	ReviewerWritebackSummary       ReviewerWritebackSummary                 `json:"reviewerWritebackSummary"`
	ReviewerDispatchIntakeHandoffs []ReviewerDispatchIntakeHandoff          `json:"reviewerDispatchIntakeHandoffs,omitempty"`
	ReviewerDispatchIntakeSummary  ReviewerDispatchIntakeSummary            `json:"reviewerDispatchIntakeSummary"`
	AuthorizedGateAdapterHandoffs  []AuthorizedGateAdapterHandoff           `json:"authorizedGateAdapterHandoffs,omitempty"`
	MissionCommanderNextActions    []mission.MissionCommanderNextActionItem `json:"missionCommanderNextActions,omitempty"`
	MissionCommanderActionQueue    mission.MissionCommanderActionQueue      `json:"missionCommanderActionQueue"`
	Inputs                         []string                                 `json:"inputs"`
	PacketRefs                     []string                                 `json:"packetRefs"`
	Events                         []ContinueEventPreview                   `json:"events"`
	OpenRisks                      []string                                 `json:"openRisks"`
	Blocked                        bool                                     `json:"blocked"`
	ReconcileRequired              bool                                     `json:"reconcileRequired"`
	PendingGateRequired            bool                                     `json:"pendingGateRequired"`
	OpenDecisionRequired           bool                                     `json:"openDecisionRequired"`
	OpenInterventions              []InterventionSummary                    `json:"openInterventions,omitempty"`
	ReconcileHandoffs              []ContinueReconcileHandoff               `json:"reconcileHandoffs,omitempty"`
	PendingGateHandoffs            []ContinuePendingGateHandoff             `json:"pendingGateHandoffs,omitempty"`
	OpenDecisionHandoffs           []ContinueOpenDecisionHandoff            `json:"openDecisionHandoffs,omitempty"`
	WouldWrites                    []StartWrite                             `json:"wouldWrites"`
	Writes                         []StartWrite                             `json:"writes,omitempty"`
	BlockedActions                 []string                                 `json:"blockedActions"`
	NextSteps                      []string                                 `json:"nextSteps"`
}

type ContinueSummary struct {
	Collected            int `json:"collected"`
	Observations         int `json:"observations"`
	Requests             int `json:"requests"`
	Routed               int `json:"routed"`
	Candidates           int `json:"candidates"`
	AcceptedCandidates   int `json:"acceptedCandidates"`
	Publications         int `json:"publications"`
	AuthorityApplied     int `json:"authorityApplied"`
	AuthorityWouldAppend int `json:"authorityWouldAppend"`
	PendingUser          int `json:"pendingUser"`
	Skipped              int `json:"skipped"`
}

type ContinueReconcileHandoff struct {
	EventID          string   `json:"eventId,omitempty"`
	Lane             string   `json:"lane,omitempty"`
	Subject          string   `json:"subject,omitempty"`
	Summary          string   `json:"summary,omitempty"`
	Action           string   `json:"action,omitempty"`
	Target           string   `json:"target,omitempty"`
	Status           string   `json:"status,omitempty"`
	ReviewCommand    string   `json:"reviewCommand,omitempty"`
	WhatIfCommand    string   `json:"whatIfCommand,omitempty"`
	ApplyCommand     string   `json:"applyCommand,omitempty"`
	DecisionBoundary string   `json:"decisionBoundary,omitempty"`
	ContinueBoundary string   `json:"continueBoundary,omitempty"`
	Evidence         []string `json:"evidence,omitempty"`
}

type ContinuePendingGateHandoff struct {
	EventID          string   `json:"eventId,omitempty"`
	Lane             string   `json:"lane,omitempty"`
	Subject          string   `json:"subject,omitempty"`
	Action           string   `json:"action,omitempty"`
	Target           string   `json:"target,omitempty"`
	Status           string   `json:"status,omitempty"`
	Risk             string   `json:"risk,omitempty"`
	Authorization    string   `json:"authorization,omitempty"`
	Profile          string   `json:"profile,omitempty"`
	ReviewCommand    string   `json:"reviewCommand,omitempty"`
	WhatIfCommand    string   `json:"whatIfCommand,omitempty"`
	ApplyCommand     string   `json:"applyCommand,omitempty"`
	DecisionBoundary string   `json:"decisionBoundary,omitempty"`
	ContinueBoundary string   `json:"continueBoundary,omitempty"`
	Evidence         []string `json:"evidence,omitempty"`
}

type ContinueOpenDecisionHandoff struct {
	EventID          string   `json:"eventId,omitempty"`
	Kind             string   `json:"kind,omitempty"`
	Lane             string   `json:"lane,omitempty"`
	Subject          string   `json:"subject,omitempty"`
	Summary          string   `json:"summary,omitempty"`
	Decision         string   `json:"decision,omitempty"`
	Reason           string   `json:"reason,omitempty"`
	Status           string   `json:"status,omitempty"`
	Target           string   `json:"target,omitempty"`
	Confidence       string   `json:"confidence,omitempty"`
	SourceKind       string   `json:"sourceKind,omitempty"`
	SourcePath       string   `json:"sourcePath,omitempty"`
	SourceCommand    string   `json:"sourceCommand,omitempty"`
	RecordPath       string   `json:"recordPath,omitempty"`
	ReviewCommand    string   `json:"reviewCommand,omitempty"`
	WhatIfCommand    string   `json:"whatIfCommand,omitempty"`
	RecordCommand    string   `json:"recordCommand,omitempty"`
	DecisionBoundary string   `json:"decisionBoundary,omitempty"`
	ContinueBoundary string   `json:"continueBoundary,omitempty"`
	Evidence         []string `json:"evidence,omitempty"`
}

type ContinueEventPreview struct {
	EventID       string         `json:"eventId"`
	Kind          string         `json:"kind"`
	Lane          string         `json:"lane"`
	Subject       string         `json:"subject,omitempty"`
	Summary       string         `json:"summary,omitempty"`
	Decision      string         `json:"decision"`
	Reason        string         `json:"reason"`
	TargetLane    string         `json:"targetLane,omitempty"`
	AuthorityFile string         `json:"authorityFile,omitempty"`
	Rows          int            `json:"rows,omitempty"`
	Verification  map[string]any `json:"verification,omitempty"`
	WouldWrites   []StartWrite   `json:"wouldWrites,omitempty"`
}

type continueContext struct {
	inst     instance.Instance
	manifest *manifest.Manifest
	board    board
	policy   continuePolicy
	selector string
	lane     Lane
}

type continuePolicy struct {
	AutoVerify                  bool
	AutoRouteRequests           bool
	AutoPublishSharedFacts      bool
	AutoAcceptLowRiskCandidates bool
	AuthorityAutoAppend         string
	RequireEvidence             bool
	RequireVerifier             bool
	MinConfidence               float64
	RequireNoConflict           bool
	RequireSchemaValid          bool
	RequireBackup               bool
	RequireDiff                 bool
	MaxAuthorityRowsPerRun      int
}

func ContinuePreview(repoRoot, caseRoot, pack string, opt ContinueOptions) (ContinueResult, error) {
	ctx, err := newContinueContext(repoRoot, caseRoot, pack, opt)
	if err != nil {
		return ContinueResult{}, err
	}
	if blocked, err := ctx.blockedByOpenInterventions(false); err != nil || blocked.Blocked {
		return blocked, err
	}
	if blocked := ctx.blockedByReviewerDispatches(false); blocked.Blocked {
		return blocked, nil
	}
	if blocked, err := ctx.blockedByPendingGateOrOpenDecision(false); err != nil || blocked.Blocked {
		return blocked, err
	}
	known, err := mission.ReadLedgerEventIDs(ctx.inst.CaseRoot)
	if err != nil {
		return ContinueResult{}, err
	}
	inputs, err := continueInputRefs(ctx.inst.CaseRoot, ctx.lane)
	if err != nil {
		return ContinueResult{}, err
	}
	packets, err := continuePacketRefs(ctx.inst.CaseRoot, ctx.lane)
	if err != nil {
		return ContinueResult{}, err
	}
	rawEvents, err := laneOutputEvents(ctx.inst.CaseRoot, ctx.lane, ctx.manifest)
	if err != nil {
		return ContinueResult{}, err
	}
	executorAction := ctx.executorAction()
	executionEvidenceReview := ctx.executionEvidenceReview()
	reviewerWritebacks := ctx.reviewerWritebacks()
	reviewerDispatchIntakeHandoffs := ctx.reviewerDispatchIntakeHandoffs()
	authorizedGateAdapterHandoffs := ctx.authorizedGateAdapterHandoffs()
	commanderNextActions := ctx.missionCommanderNextActions(executorAction, executionEvidenceReview, authorizedGateAdapterHandoffs, reviewerDispatchIntakeHandoffs)
	commanderActionQueue := mission.MissionCommanderActionQueueFor(commanderNextActions)
	result := ContinueResult{
		SchemaVersion:                  1,
		Command:                        "continue",
		CaseRoot:                       ctx.inst.CaseRoot,
		RepoRoot:                       ctx.manifest.RepoRoot,
		Pack:                           ctx.manifest.Pack,
		IsMutation:                     false,
		Applied:                        false,
		RequiresConfirmation:           true,
		Selector:                       ctx.selector,
		Lane:                           ctx.lane,
		AutonomyProfile:                autonomy.ReadSummary(ctx.inst.CaseRoot, ctx.lane.ID, ctx.manifest),
		RunID:                          continuePreviewRunID,
		BatchID:                        "batch-" + continuePreviewRunID,
		MissionBrief:                   ctx.missionBrief(),
		ExecutorAction:                 executorAction,
		ExecutionEvidenceReview:        executionEvidenceReview,
		ExecutionEvidenceReviewSummary: ExecutionEvidenceReviewSummaryFor(executionEvidenceReview, commanderActionQueue),
		ReviewerWritebacks:             reviewerWritebacks,
		ReviewerWritebackSummary:       ReviewerWritebackSummaryFor(reviewerWritebacks),
		ReviewerDispatchIntakeHandoffs: reviewerDispatchIntakeHandoffs,
		ReviewerDispatchIntakeSummary:  ReviewerDispatchIntakeSummaryFor(reviewerDispatchIntakeHandoffs),
		AuthorizedGateAdapterHandoffs:  authorizedGateAdapterHandoffs,
		MissionCommanderNextActions:    commanderNextActions,
		MissionCommanderActionQueue:    commanderActionQueue,
		Inputs:                         uniqueStrings(inputs),
		PacketRefs:                     uniqueStrings(packets),
		BlockedActions:                 []string{"run directory creation", "facts JSONL writes", "lane resume/checkpoint refresh", "board refresh", "authority/confirmed writes", "heavy-tool execution without a valid current authorization decision"},
		NextSteps: []string{
			"review this preview, then re-run continue with -Apply when the case-local facts/route/digest writes are acceptable",
			"use /rekit as the Mission Commander entrypoint; JSON preview and explicit apply are Go-owned by default",
		},
	}
	for _, raw := range rawEvents {
		event := copyEvent(raw)
		event["lane"] = ctx.lane.ID
		id := strings.TrimSpace(stringFrom(event, "eventId"))
		if id == "" {
			id = generatedEventID(ctx.lane.ID, event)
			event["eventId"] = id
		}
		if known[id] {
			result.Summary.Skipped++
			continue
		}
		preview := ctx.previewEvent(event)
		result.Summary.Collected++
		result.Events = append(result.Events, preview)
		result.WouldWrites = append(result.WouldWrites, preview.WouldWrites...)
		if preview.Decision == "defer" || preview.Decision == "pending-user" {
			result.OpenRisks = append(result.OpenRisks, riskLine(preview))
		}
		switch preview.Kind {
		case "observation":
			if preview.Decision == "accept" {
				result.Summary.Observations++
			}
		case "request":
			result.Summary.Requests++
			if preview.TargetLane != "" && preview.Decision == "accept" {
				result.Summary.Routed++
			}
		case "candidate":
			result.Summary.Candidates++
			if preview.Decision == "accept" {
				if preview.AuthorityFile != "" {
					result.Summary.AuthorityWouldAppend += preview.Rows
				} else {
					result.Summary.AcceptedCandidates++
				}
			} else {
				result.Summary.PendingUser++
			}
		case "publication":
			if preview.Decision == "accept" {
				result.Summary.Publications++
			}
		}
	}
	return result, nil
}

func ContinueApply(repoRoot, caseRoot, pack string, opt ContinueOptions) (result ContinueResult, err error) {
	ctx, err := newContinueContext(repoRoot, caseRoot, pack, opt)
	if err != nil {
		return ContinueResult{}, err
	}
	lease, err := acquireLaneMutationLock(ctx.inst.CaseRoot, ctx.lane.ID)
	if err != nil {
		return ContinueResult{}, err
	}
	defer func() {
		if unlockErr := lease.Unlock(); unlockErr != nil {
			err = errors.Join(err, unlockErr)
			result = ContinueResult{}
		}
	}()
	ctx, err = newContinueContext(repoRoot, caseRoot, pack, opt)
	if err != nil {
		return ContinueResult{}, err
	}
	if err := lease.Validate(); err != nil {
		return ContinueResult{}, err
	}
	if blocked, err := ctx.blockedByOpenInterventions(true); err != nil || blocked.Blocked {
		return blocked, err
	}
	if blocked := ctx.blockedByReviewerDispatches(true); blocked.Blocked {
		return blocked, nil
	}
	if blocked, err := ctx.blockedByPendingGateOrOpenDecision(true); err != nil || blocked.Blocked {
		return blocked, err
	}
	known, err := mission.ReadLedgerEventIDs(ctx.inst.CaseRoot)
	if err != nil {
		return ContinueResult{}, err
	}
	inputs, err := continueInputRefs(ctx.inst.CaseRoot, ctx.lane)
	if err != nil {
		return ContinueResult{}, err
	}
	packets, err := continuePacketRefs(ctx.inst.CaseRoot, ctx.lane)
	if err != nil {
		return ContinueResult{}, err
	}
	rawEvents, err := laneOutputEvents(ctx.inst.CaseRoot, ctx.lane, ctx.manifest)
	if err != nil {
		return ContinueResult{}, err
	}
	stamp := time.Now().UTC().Format("20060102-150405000")
	runID := "run-" + stamp
	batchID := "batch-" + runID
	runRoot, err := refsf.SafeJoin(ctx.inst.CaseRoot, relJoin(".rekit", "runs", runID))
	if err != nil {
		return ContinueResult{}, err
	}
	if err := os.MkdirAll(runRoot, 0o755); err != nil {
		return ContinueResult{}, err
	}
	result = ContinueResult{
		SchemaVersion:        1,
		Command:              "continue",
		CaseRoot:             ctx.inst.CaseRoot,
		RepoRoot:             ctx.manifest.RepoRoot,
		Pack:                 ctx.manifest.Pack,
		IsMutation:           true,
		Applied:              true,
		RequiresConfirmation: false,
		Selector:             ctx.selector,
		Lane:                 ctx.lane,
		AutonomyProfile:      autonomy.ReadSummary(ctx.inst.CaseRoot, ctx.lane.ID, ctx.manifest),
		RunID:                runID,
		BatchID:              batchID,
		Inputs:               uniqueStrings(inputs),
		PacketRefs:           uniqueStrings(packets),
		Events:               []ContinueEventPreview{},
		OpenRisks:            []string{},
		WouldWrites:          []StartWrite{},
		Writes:               []StartWrite{},
		BlockedActions:       []string{"authority/confirmed writes", "heavy-tool execution without a valid current authorization decision"},
		NextSteps: []string{
			"run doctor after apply",
			"use /rekit handoff " + workstreamLabel(ctx.lane) + " to refresh case-local handoff when needed",
		},
	}
	for _, raw := range rawEvents {
		event := copyEvent(raw)
		event["lane"] = ctx.lane.ID
		id := strings.TrimSpace(stringFrom(event, "eventId"))
		if id == "" {
			id = generatedEventID(ctx.lane.ID, event)
			event["eventId"] = id
		}
		if known[id] {
			result.Summary.Skipped++
			continue
		}
		if strings.TrimSpace(stringFrom(event, "time")) == "" {
			event["time"] = isoNow()
		}
		if strings.TrimSpace(stringFrom(event, "batchId")) == "" {
			event["batchId"] = batchID
		}
		preview := ctx.previewEvent(event)
		if preview.AuthorityFile != "" && preview.Decision == "accept" {
			preview.Decision = "defer"
			preview.Reason = "authority append requires explicit user confirmation; Go continue -Apply does not write authority/confirmed"
			preview.WouldWrites = wouldFactKinds("candidate", "decision")
		}
		writes, err := ctx.applyContinueEvent(event, preview, runID, batchID)
		if err != nil {
			return ContinueResult{}, err
		}
		result.Summary.Collected++
		result.Events = append(result.Events, preview)
		result.Writes = append(result.Writes, writes...)
		if preview.Decision == "defer" || preview.Decision == "pending-user" {
			result.OpenRisks = append(result.OpenRisks, riskLine(preview))
		}
		result.updateApplySummary(preview)
		known[id] = true
	}
	resumePath, checkpointPath, err := writeLaneResume(ctx.inst.CaseRoot, ctx.manifest, ctx.lane)
	if err != nil {
		return ContinueResult{}, err
	}
	result.Writes = append(result.Writes,
		StartWrite{Path: relativePath(ctx.inst.CaseRoot, resumePath), Kind: "lane-resume", Action: "refresh", TargetPath: resumePath},
		StartWrite{Path: relativePath(ctx.inst.CaseRoot, checkpointPath), Kind: "lane-checkpoint", Action: "refresh", TargetPath: checkpointPath},
	)
	boardPath, err := saveBoard(ctx.inst.CaseRoot, ctx.manifest)
	if err != nil {
		return ContinueResult{}, err
	}
	result.Writes = append(result.Writes, StartWrite{Path: ".rekit/board.json", Kind: "board", Action: "refresh", TargetPath: boardPath})
	result.MissionBrief = ctx.missionBrief()
	result.ExecutorAction = ctx.executorAction()
	result.ExecutionEvidenceReview = ctx.executionEvidenceReview()
	result.ReviewerWritebacks = ctx.reviewerWritebacks()
	result.ReviewerWritebackSummary = ReviewerWritebackSummaryFor(result.ReviewerWritebacks)
	result.ReviewerDispatchIntakeHandoffs = ctx.reviewerDispatchIntakeHandoffs()
	result.ReviewerDispatchIntakeSummary = ReviewerDispatchIntakeSummaryFor(result.ReviewerDispatchIntakeHandoffs)
	result.AuthorizedGateAdapterHandoffs = ctx.authorizedGateAdapterHandoffs()
	result.MissionCommanderNextActions = ctx.missionCommanderNextActions(result.ExecutorAction, result.ExecutionEvidenceReview, result.AuthorizedGateAdapterHandoffs, result.ReviewerDispatchIntakeHandoffs)
	result.MissionCommanderActionQueue = mission.MissionCommanderActionQueueFor(result.MissionCommanderNextActions)
	result.ExecutionEvidenceReviewSummary = ExecutionEvidenceReviewSummaryFor(result.ExecutionEvidenceReview, result.MissionCommanderActionQueue)
	result.NextSteps = workstreamNextSteps(result.ExecutorAction, true)
	statusPath, digestPath, err := writeContinueRunArtifacts(runRoot, result)
	if err != nil {
		return ContinueResult{}, err
	}
	result.Writes = append(result.Writes,
		StartWrite{Path: relativePath(ctx.inst.CaseRoot, statusPath), Kind: "run-status", Action: "write", TargetPath: statusPath},
		StartWrite{Path: relativePath(ctx.inst.CaseRoot, digestPath), Kind: "run-digest", Action: "write", TargetPath: digestPath},
	)
	return result, nil
}

func newContinueContext(repoRoot, caseRoot, pack string, opt ContinueOptions) (continueContext, error) {
	inst, err := instance.AssertAttached(caseRoot, repoRoot, pack)
	if err != nil {
		return continueContext{}, err
	}
	m, err := manifest.Load(repoRoot, pack)
	if err != nil {
		return continueContext{}, err
	}
	b, err := readBoard(inst.CaseRoot)
	if os.IsNotExist(err) {
		return continueContext{}, fmt.Errorf("continue requires existing .rekit/board.json; run /rekit overview once to initialize the case-local board")
	}
	if err != nil {
		return continueContext{}, err
	}
	if strings.TrimSpace(b.DefaultAuthorityLane) == "" {
		b.DefaultAuthorityLane = m.WorkstreamDefaults["defaultAuthorityLane"]
	}
	selector := strings.TrimSpace(opt.Selector)
	if selector == "" {
		open := mission.OpenBoardLanes(b.Lanes)
		if len(open) != 1 {
			return continueContext{}, fmt.Errorf("continue requires a lane selector when multiple open lanes exist; use main or a workstream name")
		}
		lane, err := readLaneByID(inst.CaseRoot, open[0].ID)
		if err != nil {
			return continueContext{}, err
		}
		selector = workstreamLabel(lane)
	}
	lane, err := resolveHandoffLane(inst.CaseRoot, b, selector)
	if err != nil {
		return continueContext{}, err
	}
	status := strings.ToLower(strings.TrimSpace(lane.Status))
	if status == "archived" || status == "paused" || status == "closed" {
		return continueContext{}, fmt.Errorf("target lane is not open: %s", lane.ID)
	}
	if err := validateContinueOwnerBinding(lane, opt); err != nil {
		return continueContext{}, err
	}
	policy, err := readContinuePolicy(inst.CaseRoot)
	if err != nil {
		return continueContext{}, err
	}
	return continueContext{inst: inst, manifest: m, board: b, policy: policy, selector: selector, lane: lane}, nil
}

func validateContinueOwnerBinding(lane Lane, opt ContinueOptions) error {
	currentExecutor := strings.TrimSpace(lane.CurrentExecutor)
	if currentExecutor == "" {
		if lane.ExecutorGeneration == 0 && strings.TrimSpace(opt.Executor) == "" && opt.ExpectedExecutorGeneration == 0 {
			return nil
		}
		return fmt.Errorf("continue owner guard mismatch for legacy unassigned lane %s: expected executor=%s generation=%d current executor=unassigned generation=%d", lane.ID, textOrUnassigned(opt.Executor), opt.ExpectedExecutorGeneration, lane.ExecutorGeneration)
	}
	expectedExecutor := strings.TrimSpace(opt.Executor)
	if expectedExecutor == "" {
		return fmt.Errorf("continue requires explicit Executor and ExpectedExecutorGeneration for owned lane %s: current executor=%s generation=%d", lane.ID, currentExecutor, lane.ExecutorGeneration)
	}
	if opt.ExpectedExecutorGeneration <= 0 {
		return fmt.Errorf("continue requires positive ExpectedExecutorGeneration for owned lane %s: current executor=%s generation=%d", lane.ID, currentExecutor, lane.ExecutorGeneration)
	}
	if expectedExecutor != currentExecutor || opt.ExpectedExecutorGeneration != lane.ExecutorGeneration {
		return fmt.Errorf("continue owner guard is not current for lane %s: expected executor=%s generation=%d current executor=%s generation=%d", lane.ID, textOrUnassigned(expectedExecutor), opt.ExpectedExecutorGeneration, textOrUnassigned(currentExecutor), lane.ExecutorGeneration)
	}
	return nil
}

func textOrUnassigned(value string) string {
	if value = strings.TrimSpace(value); value != "" {
		return value
	}
	return "unassigned"
}

func (ctx continueContext) missionBrief() mission.Brief {
	facts, err := readHandoffFacts(ctx.inst.CaseRoot)
	if err != nil {
		return mission.Brief{Summary: "unavailable: " + err.Error()}
	}
	return projectMissionBrief(ctx.board.Lanes, facts)
}

func (ctx continueContext) executorAction() laneExecutorAction {
	facts, err := readHandoffFacts(ctx.inst.CaseRoot)
	if err != nil {
		brief := mission.Brief{Summary: "unavailable: " + err.Error()}
		return laneExecutorActionFor(ctx.lane, mission.Facts{}, brief)
	}
	brief := laneMissionBrief(ctx.lane, facts)
	return laneExecutorActionFor(ctx.lane, facts.Facts, brief)
}

func (ctx continueContext) executionEvidenceReview() []ExecutionEvidenceReviewItem {
	facts, err := readHandoffFacts(ctx.inst.CaseRoot)
	if err != nil {
		return nil
	}
	return bindExecutionEvidenceReviewContinueCommands(laneExecutionEvidenceReview(ctx.lane, facts.Observations), func(string) (Lane, bool) {
		return ctx.lane, true
	})
}

func (ctx continueContext) reviewerWritebacks() []ReviewerWritebackItem {
	facts, err := readHandoffFacts(ctx.inst.CaseRoot)
	if err != nil {
		return nil
	}
	return ReviewerWritebackItems(facts, ctx.lane.ID)
}

func (ctx continueContext) reviewerDispatchIntakeHandoffs() []ReviewerDispatchIntakeHandoff {
	facts, err := readHandoffFacts(ctx.inst.CaseRoot)
	if err != nil {
		return nil
	}
	items, err := ReviewerDispatchIntakeHandoffs(ctx.inst.CaseRoot, facts, ctx.lane.ID)
	if err != nil {
		return nil
	}
	return items
}

func (ctx continueContext) authorizedGateAdapterHandoffs() []AuthorizedGateAdapterHandoff {
	facts, err := readHandoffFacts(ctx.inst.CaseRoot)
	if err != nil {
		return nil
	}
	return AuthorizedGateAdapterHandoffs(ctx.manifest.RepoRoot, ctx.inst.CaseRoot, ctx.manifest.Pack, facts.Requests, ctx.lane.ID)
}

func (ctx continueContext) missionCommanderNextActions(action laneExecutorAction, evidenceReview []ExecutionEvidenceReviewItem, adapterHandoffs []AuthorizedGateAdapterHandoff, reviewerHandoffs []ReviewerDispatchIntakeHandoff) []mission.MissionCommanderNextActionItem {
	items := mission.MissionCommanderNextActions([]mission.LaneExecutorActionSnapshot{laneCommanderActionSnapshot(ctx.lane, action)}, evidenceReview, action.Blocked)
	items = MissionCommanderNextActionsWithAuthorizedGateAdapters(items, adapterHandoffs)
	return MissionCommanderNextActionsWithReviewerDispatches(items, reviewerHandoffs)
}

func (ctx continueContext) blockedByReviewerDispatches(apply bool) ContinueResult {
	handoffs := ctx.reviewerDispatchIntakeHandoffs()
	if len(handoffs) == 0 {
		return ContinueResult{}
	}
	executorAction := ctx.executorAction()
	executionEvidenceReview := ctx.executionEvidenceReview()
	reviewerWritebacks := ctx.reviewerWritebacks()
	adapterHandoffs := ctx.authorizedGateAdapterHandoffs()
	nextActions := ctx.missionCommanderNextActions(executorAction, executionEvidenceReview, adapterHandoffs, handoffs)
	queue := mission.MissionCommanderActionQueueFor(nextActions)
	return ContinueResult{
		SchemaVersion:                  1,
		Command:                        "continue",
		CaseRoot:                       ctx.inst.CaseRoot,
		RepoRoot:                       ctx.manifest.RepoRoot,
		Pack:                           ctx.manifest.Pack,
		IsMutation:                     apply,
		Applied:                        false,
		RequiresConfirmation:           false,
		Selector:                       ctx.selector,
		Lane:                           ctx.lane,
		AutonomyProfile:                autonomy.ReadSummary(ctx.inst.CaseRoot, ctx.lane.ID, ctx.manifest),
		RunID:                          continuePreviewRunID,
		BatchID:                        "batch-" + continuePreviewRunID,
		MissionBrief:                   ctx.missionBrief(),
		ExecutorAction:                 executorAction,
		ExecutionEvidenceReview:        executionEvidenceReview,
		ExecutionEvidenceReviewSummary: ExecutionEvidenceReviewSummaryFor(executionEvidenceReview, queue),
		ReviewerWritebacks:             reviewerWritebacks,
		ReviewerWritebackSummary:       ReviewerWritebackSummaryFor(reviewerWritebacks),
		ReviewerDispatchIntakeHandoffs: handoffs,
		ReviewerDispatchIntakeSummary:  ReviewerDispatchIntakeSummaryFor(handoffs),
		AuthorizedGateAdapterHandoffs:  adapterHandoffs,
		MissionCommanderNextActions:    nextActions,
		MissionCommanderActionQueue:    queue,
		Blocked:                        true,
		BlockedActions:                 []string{"run directory creation", "facts JSONL writes", "lane resume/checkpoint refresh", "board refresh", "lane continuation while reviewer dispatch/intake remains open"},
		NextSteps:                      []string{ReviewerDispatchIntakeSummaryFor(handoffs).NextAction},
	}
}

func (ctx continueContext) blockedByOpenInterventions(apply bool) (ContinueResult, error) {
	open, err := openLaneInterventionSummaries(ctx.inst.CaseRoot, ctx.lane.ID)
	if err != nil {
		return ContinueResult{}, err
	}
	if len(open) == 0 {
		return ContinueResult{}, nil
	}
	executorAction := ctx.executorAction()
	executionEvidenceReview := ctx.executionEvidenceReview()
	reviewerWritebacks := ctx.reviewerWritebacks()
	reviewerDispatchIntakeHandoffs := ctx.reviewerDispatchIntakeHandoffs()
	authorizedGateAdapterHandoffs := ctx.authorizedGateAdapterHandoffs()
	commanderNextActions := ctx.missionCommanderNextActions(executorAction, executionEvidenceReview, authorizedGateAdapterHandoffs, reviewerDispatchIntakeHandoffs)
	commanderActionQueue := mission.MissionCommanderActionQueueFor(commanderNextActions)
	return ContinueResult{
		SchemaVersion:                  1,
		Command:                        "continue",
		CaseRoot:                       ctx.inst.CaseRoot,
		RepoRoot:                       ctx.manifest.RepoRoot,
		Pack:                           ctx.manifest.Pack,
		IsMutation:                     apply,
		Applied:                        false,
		RequiresConfirmation:           false,
		Selector:                       ctx.selector,
		Lane:                           ctx.lane,
		AutonomyProfile:                autonomy.ReadSummary(ctx.inst.CaseRoot, ctx.lane.ID, ctx.manifest),
		RunID:                          continuePreviewRunID,
		BatchID:                        "batch-" + continuePreviewRunID,
		MissionBrief:                   ctx.missionBrief(),
		ExecutorAction:                 executorAction,
		ExecutionEvidenceReview:        executionEvidenceReview,
		ExecutionEvidenceReviewSummary: ExecutionEvidenceReviewSummaryFor(executionEvidenceReview, commanderActionQueue),
		ReviewerWritebacks:             reviewerWritebacks,
		ReviewerWritebackSummary:       ReviewerWritebackSummaryFor(reviewerWritebacks),
		ReviewerDispatchIntakeHandoffs: reviewerDispatchIntakeHandoffs,
		ReviewerDispatchIntakeSummary:  ReviewerDispatchIntakeSummaryFor(reviewerDispatchIntakeHandoffs),
		AuthorizedGateAdapterHandoffs:  authorizedGateAdapterHandoffs,
		MissionCommanderNextActions:    commanderNextActions,
		MissionCommanderActionQueue:    commanderActionQueue,
		OpenRisks:                      interventionRiskLines(open),
		Blocked:                        true,
		ReconcileRequired:              true,
		OpenInterventions:              open,
		ReconcileHandoffs:              continueReconcileHandoffs(ctx.lane, open),
		WouldWrites:                    []StartWrite{},
		Writes:                         []StartWrite{},
		BlockedActions:                 []string{"run directory creation", "facts JSONL writes", "lane resume/checkpoint refresh", "board refresh", "authority/confirmed writes", "heavy-tool execution without a valid current authorization decision"},
		NextSteps:                      executorAction.NextAgentActions,
	}, nil
}

func (ctx continueContext) blockedByPendingGateOrOpenDecision(apply bool) (ContinueResult, error) {
	facts, err := readHandoffFacts(ctx.inst.CaseRoot)
	if err != nil {
		return ContinueResult{}, err
	}
	laneFacts := mission.LaneFacts(facts.Facts, ctx.lane.ID)
	pendingGates := []map[string]any{}
	for _, item := range laneFacts.Requests {
		if mission.IsPendingGateRequest(item) {
			pendingGates = append(pendingGates, item)
		}
	}
	openDecisions := continueOpenDecisionItems(laneFacts)
	if len(pendingGates) == 0 && len(openDecisions) == 0 {
		return ContinueResult{}, nil
	}
	executorAction := ctx.executorAction()
	executionEvidenceReview := ctx.executionEvidenceReview()
	reviewerWritebacks := ctx.reviewerWritebacks()
	reviewerDispatchIntakeHandoffs := ctx.reviewerDispatchIntakeHandoffs()
	authorizedGateAdapterHandoffs := ctx.authorizedGateAdapterHandoffs()
	commanderNextActions := ctx.missionCommanderNextActions(executorAction, executionEvidenceReview, authorizedGateAdapterHandoffs, reviewerDispatchIntakeHandoffs)
	commanderActionQueue := mission.MissionCommanderActionQueueFor(commanderNextActions)
	return ContinueResult{
		SchemaVersion:                  1,
		Command:                        "continue",
		CaseRoot:                       ctx.inst.CaseRoot,
		RepoRoot:                       ctx.manifest.RepoRoot,
		Pack:                           ctx.manifest.Pack,
		IsMutation:                     apply,
		Applied:                        false,
		RequiresConfirmation:           false,
		Selector:                       ctx.selector,
		Lane:                           ctx.lane,
		AutonomyProfile:                autonomy.ReadSummary(ctx.inst.CaseRoot, ctx.lane.ID, ctx.manifest),
		RunID:                          continuePreviewRunID,
		BatchID:                        "batch-" + continuePreviewRunID,
		MissionBrief:                   ctx.missionBrief(),
		ExecutorAction:                 executorAction,
		ExecutionEvidenceReview:        executionEvidenceReview,
		ExecutionEvidenceReviewSummary: ExecutionEvidenceReviewSummaryFor(executionEvidenceReview, commanderActionQueue),
		ReviewerWritebacks:             reviewerWritebacks,
		ReviewerWritebackSummary:       ReviewerWritebackSummaryFor(reviewerWritebacks),
		ReviewerDispatchIntakeHandoffs: reviewerDispatchIntakeHandoffs,
		ReviewerDispatchIntakeSummary:  ReviewerDispatchIntakeSummaryFor(reviewerDispatchIntakeHandoffs),
		AuthorizedGateAdapterHandoffs:  authorizedGateAdapterHandoffs,
		MissionCommanderNextActions:    commanderNextActions,
		MissionCommanderActionQueue:    commanderActionQueue,
		OpenRisks:                      append(continuePendingGateRiskLines(pendingGates), continueOpenDecisionRiskLines(openDecisions)...),
		Blocked:                        true,
		PendingGateRequired:            len(pendingGates) > 0,
		OpenDecisionRequired:           len(openDecisions) > 0,
		PendingGateHandoffs:            continuePendingGateHandoffs(ctx.lane, pendingGates),
		OpenDecisionHandoffs:           continueOpenDecisionHandoffs(ctx.lane, openDecisions),
		WouldWrites:                    []StartWrite{},
		Writes:                         []StartWrite{},
		BlockedActions:                 []string{"run directory creation", "facts JSONL writes", "lane resume/checkpoint refresh", "board refresh", "authority/confirmed writes", "heavy-tool execution without a valid current authorization decision", "lane continuation while pending gate or open decision remains unresolved"},
		NextSteps:                      executorAction.NextAgentActions,
	}, nil
}

func continuePendingGateRiskLines(items []map[string]any) []string {
	lines := []string{}
	for _, item := range items {
		lines = append(lines, fmt.Sprintf("pending-gate: %s | lane=%s | status=%s", firstText(mission.Value(item, "subject"), mission.Value(item, "summary"), mission.Value(item, "eventId")), mission.Value(item, "lane"), firstText(mission.Value(item, "status"), "pending-gate")))
	}
	return lines
}

func gateDecisionHandoffsForLane(caseRoot string, lane Lane) ([]ContinuePendingGateHandoff, []ContinueOpenDecisionHandoff) {
	facts, err := readHandoffFacts(caseRoot)
	if err != nil {
		return nil, nil
	}
	return gateDecisionHandoffs(lane, mission.LaneFacts(facts.Facts, lane.ID))
}

func gateDecisionHandoffs(lane Lane, facts mission.Facts) ([]ContinuePendingGateHandoff, []ContinueOpenDecisionHandoff) {
	pendingGates := []map[string]any{}
	for _, item := range facts.Requests {
		if mission.IsPendingGateRequest(item) {
			pendingGates = append(pendingGates, item)
		}
	}
	return continuePendingGateHandoffs(lane, pendingGates), continueOpenDecisionHandoffs(lane, continueOpenDecisionItems(facts))
}

type continueOpenDecisionItem struct {
	SourceKind string
	Event      map[string]any
}

func continueOpenDecisionItems(facts mission.Facts) []continueOpenDecisionItem {
	items := []continueOpenDecisionItem{}
	for _, event := range mission.EffectiveOpenCandidates(facts) {
		items = append(items, continueOpenDecisionItem{SourceKind: "candidate", Event: event})
	}
	for _, event := range mission.OpenDecisionEvents(facts.Decisions) {
		items = append(items, continueOpenDecisionItem{SourceKind: "decision", Event: event})
	}
	return items
}

func continueOpenDecisionRiskLines(items []continueOpenDecisionItem) []string {
	lines := []string{}
	for _, item := range items {
		lines = append(lines, fmt.Sprintf("open-decision: %s | lane=%s | status=%s", firstText(mission.Value(item.Event, "subject"), mission.Value(item.Event, "summary"), mission.Value(item.Event, "eventId")), mission.Value(item.Event, "lane"), firstText(mission.Value(item.Event, "status"), "open")))
	}
	return lines
}

func interventionRiskLines(items []InterventionSummary) []string {
	lines := []string{}
	for _, item := range items {
		lines = append(lines, fmt.Sprintf("intervention: %s | lane=%s | status=%s", firstText(item.Subject, item.Summary, item.EventID), item.Lane, item.Status))
	}
	return lines
}

func continueReconcileHandoffs(lane Lane, items []InterventionSummary) []ContinueReconcileHandoff {
	label := workstreamLabel(lane)
	handoffs := make([]ContinueReconcileHandoff, 0, len(items))
	for _, item := range items {
		evidence := []string{}
		if strings.TrimSpace(item.EventID) != "" {
			evidence = append(evidence, "open intervention ledger event "+item.EventID)
		} else {
			evidence = append(evidence, "open intervention has no eventId; review lane handoff before replacing <eventId> in reconcile apply")
		}
		if strings.TrimSpace(item.ApprovedBy) != "" {
			evidence = append(evidence, "approvedBy "+item.ApprovedBy)
		}
		if strings.TrimSpace(item.Scope) != "" {
			evidence = append(evidence, "scope "+item.Scope)
		}
		if strings.TrimSpace(item.Target) != "" {
			evidence = append(evidence, "target "+item.Target)
		}
		if strings.TrimSpace(item.BatchID) != "" {
			evidence = append(evidence, "batchId "+item.BatchID)
		}
		whatIfCommand := continueReconcileCommand(label, item.EventID, false)
		applyCommand := continueReconcileCommand(label, item.EventID, true)
		if lane.CurrentExecutor != "" {
			ownerArg := " -Executor " + quoteCommandArg(lane.CurrentExecutor)
			whatIfCommand += ownerArg
			applyCommand += ownerArg
		}
		handoffs = append(handoffs, ContinueReconcileHandoff{
			EventID:          item.EventID,
			Lane:             item.Lane,
			Subject:          item.Subject,
			Summary:          item.Summary,
			Action:           item.Action,
			Target:           item.Target,
			Status:           firstText(item.Status, "open"),
			ReviewCommand:    "/rekit handoff " + label,
			WhatIfCommand:    whatIfCommand,
			ApplyCommand:     applyCommand,
			DecisionBoundary: "review the open intervention before reconcile apply; apply command only writes case-local intervention/lane/resume/checkpoint/board state and never writes authority/confirmed or executes heavy-tool",
			ContinueBoundary: "blocked continue is zero-write and only exposes reconcile handoff; do not continue autonomously while intervention remains open",
			Evidence:         evidence,
		})
	}
	return handoffs
}

func continueReconcileCommand(label, eventID string, apply bool) string {
	parts := []string{"/rekit", "reconcile", label, "-InterventionId", firstText(eventID, "<eventId>")}
	if apply {
		parts = append(parts, "-Apply")
	} else {
		parts = append(parts, "-WhatIf")
	}
	return continueCommand(parts...)
}

func continuePendingGateHandoffs(lane Lane, items []map[string]any) []ContinuePendingGateHandoff {
	label := workstreamLabel(lane)
	handoffs := []ContinuePendingGateHandoff{}
	for _, item := range lastObjects(items, maxHandoffRows) {
		gate, _ := item["gate"].(map[string]any)
		authorization, _ := gate["authorization"].(map[string]any)
		eventID := mission.Value(item, "eventId")
		evidence := []string{}
		if eventID != "" {
			evidence = append(evidence, "pending-gate ledger event "+eventID)
		} else {
			evidence = append(evidence, "pending-gate has no eventId; review lane handoff before replaying a gate decision request")
		}
		if reasons := mission.Value(authorization, "reasons"); reasons != "" {
			evidence = append(evidence, "authorization reasons "+reasons)
		}
		if budget := mission.Value(gate, "budget"); budget != "" {
			evidence = append(evidence, "requested budget "+budget)
		}
		if budget := continueRequestedBudgetEvidence(gate); budget != "" {
			evidence = append(evidence, "requestedBudget "+budget)
		}
		if outputs := mission.Value(gate, "outputPaths"); outputs != "" {
			evidence = append(evidence, "requested outputPaths "+outputs)
		}
		if stops := mission.Value(gate, "stopConditions"); stops != "" {
			evidence = append(evidence, "requested stopConditions "+stops)
		}
		if tried := mission.Value(gate, "triedLightSteps"); tried != "" {
			evidence = append(evidence, "triedLightSteps "+tried)
		}
		handoffs = append(handoffs, ContinuePendingGateHandoff{
			EventID:          eventID,
			Lane:             mission.Value(item, "lane"),
			Subject:          mission.Value(item, "subject"),
			Action:           mission.Value(gate, "action"),
			Target:           mission.Value(item, "target"),
			Status:           mission.Value(item, "status"),
			Risk:             mission.Value(item, "risk"),
			Authorization:    mission.Value(authorization, "decision"),
			Profile:          mission.Value(authorization, "profileId"),
			ReviewCommand:    "/rekit handoff " + label,
			WhatIfCommand:    continueGateRequestCommand(lane, item, false),
			ApplyCommand:     continueGateRequestCommand(lane, item, true),
			DecisionBoundary: "review with the main agent/user or update strict durable autonomy before any heavy action; apply command only replays/records the gate request decision and does not execute or approve heavy action by itself",
			ContinueBoundary: "blocked continue is zero-write and only exposes pending-gate handoff; do not continue autonomously while the pending gate remains unresolved",
			Evidence:         evidence,
		})
	}
	return handoffs
}

func continueRequestedBudgetEvidence(gate map[string]any) string {
	budget, _ := gate["requestedBudget"].(map[string]any)
	runtimeSeconds := mission.Value(budget, "runtimeSeconds")
	diskMB := mission.Value(budget, "diskMB")
	requests := mission.Value(budget, "requests")
	if continueEmptyBudgetValue(runtimeSeconds) && continueEmptyBudgetValue(diskMB) && continueEmptyBudgetValue(requests) {
		return ""
	}
	parts := []string{}
	continueAddEvidencePart(&parts, "runtimeSeconds", runtimeSeconds)
	continueAddEvidencePart(&parts, "diskMB", diskMB)
	continueAddEvidencePart(&parts, "requests", requests)
	return strings.Join(parts, ",")
}

func continueEmptyBudgetValue(value string) bool {
	value = strings.TrimSpace(value)
	return value == "" || value == "0" || value == "0.0"
}

func continueAddEvidencePart(parts *[]string, key, value string) {
	if strings.TrimSpace(value) != "" {
		*parts = append(*parts, key+"="+strings.TrimSpace(value))
	}
}

func continueGateRequestCommand(lane Lane, item map[string]any, apply bool) string {
	gate, _ := item["gate"].(map[string]any)
	action := firstText(mission.Value(gate, "action"), "<action>")
	laneID := firstText(mission.Value(item, "lane"), lane.ID)
	parts := []string{"/rekit", "gate", "-Action", action, "-Lane", laneID}
	if apply {
		parts = append(parts, "-Apply", "-Actor", firstText(mission.Value(item, "actor"), "<actor>"))
	} else {
		parts = append(parts, "-WhatIf")
	}
	parts = continueAppendCommandArg(parts, "-Subject", mission.Value(item, "subject"))
	parts = continueAppendCommandArg(parts, "-Summary", mission.Value(item, "summary"))
	parts = continueAppendCommandArg(parts, "-TargetRef", mission.Value(item, "target"))
	parts = continueAppendCommandArg(parts, "-BatchId", mission.Value(item, "batchId"))
	parts = continueAppendCommandArg(parts, "-Scope", mission.Value(gate, "scope"))
	parts = continueAppendCommandArg(parts, "-Budget", mission.Value(gate, "budget"))
	budget, _ := gate["requestedBudget"].(map[string]any)
	parts = continueAppendCommandArg(parts, "-RuntimeSeconds", mission.Value(budget, "runtimeSeconds"))
	parts = continueAppendCommandArg(parts, "-DiskMB", mission.Value(budget, "diskMB"))
	parts = continueAppendCommandArg(parts, "-Requests", mission.Value(budget, "requests"))
	parts = continueAppendCommandArg(parts, "-OutputPaths", mission.Value(gate, "outputPaths"))
	parts = continueAppendCommandArg(parts, "-TriedLightSteps", mission.Value(gate, "triedLightSteps"))
	parts = continueAppendCommandArg(parts, "-StopConditions", mission.Value(gate, "stopConditions"))
	parts = continueAppendCommandArg(parts, "-Risk", mission.Value(item, "risk"))
	return continueCommand(parts...)
}

func continueOpenDecisionHandoffs(lane Lane, items []continueOpenDecisionItem) []ContinueOpenDecisionHandoff {
	label := workstreamLabel(lane)
	handoffs := []ContinueOpenDecisionHandoff{}
	for _, item := range continueLimitOpenDecisionItems(items, maxHandoffRows) {
		event := item.Event
		eventID := mission.Value(event, "eventId")
		sourceKind := continueOpenDecisionSourceKind(item.SourceKind)
		kind := firstText(mission.Value(event, "kind"), sourceKind, "decision")
		sourcePath := mission.FactRelPath(sourceKind)
		recordPath := mission.FactRelPath("decision")
		decision := mission.Value(event, "decision")
		if decision == "" {
			decision = mission.Value(event, "action")
		}
		evidence := []string{}
		if eventID != "" {
			evidence = append(evidence, kind+" ledger event "+eventID)
		} else {
			evidence = append(evidence, kind+" has no eventId; review lane handoff before adding related refs to a decision note")
		}
		if confidence := mission.Value(event, "confidence"); confidence != "" {
			evidence = append(evidence, "confidence "+confidence)
		}
		if refs := mission.Value(event, "evidenceRefs"); refs != "" {
			evidence = append(evidence, "evidenceRefs "+refs)
		}
		evidence = append(evidence, "sourcePath "+sourcePath)
		evidence = append(evidence, "recordPath "+recordPath)
		if target := mission.Value(event, "target"); target != "" {
			evidence = append(evidence, "target "+target)
		}
		if batchID := mission.Value(event, "batchId"); batchID != "" {
			evidence = append(evidence, "batchId "+batchID)
		}
		handoffs = append(handoffs, ContinueOpenDecisionHandoff{
			EventID:          eventID,
			Kind:             kind,
			Lane:             mission.Value(event, "lane"),
			Subject:          mission.Value(event, "subject"),
			Summary:          mission.Value(event, "summary"),
			Decision:         decision,
			Reason:           mission.Value(event, "reason"),
			Status:           firstText(mission.Value(event, "status"), "open"),
			Target:           mission.Value(event, "target"),
			Confidence:       mission.Value(event, "confidence"),
			SourceKind:       sourceKind,
			SourcePath:       sourcePath,
			SourceCommand:    continueOpenDecisionSourceCommand(lane, sourceKind, mission.Value(event, "lane")),
			RecordPath:       recordPath,
			ReviewCommand:    "/rekit handoff " + label,
			WhatIfCommand:    continueDecisionNoteCommand(lane, event, true),
			RecordCommand:    "run the hash-bound recordCommand returned by note -WhatIf",
			DecisionBoundary: "review evidence and choose accept/reject/defer/supersede with note -WhatIf first; then run the returned hash-bound recordCommand, which only appends case-local decision ledger state and never writes authority/confirmed or executes heavy-tool",
			ContinueBoundary: "blocked continue is zero-write and only exposes open-decision handoff; do not continue autonomously while the open decision remains unresolved",
			Evidence:         evidence,
		})
	}
	return handoffs
}

func continueLimitOpenDecisionItems(events []continueOpenDecisionItem, n int) []continueOpenDecisionItem {
	if n <= 0 || len(events) <= n {
		return events
	}
	return events[len(events)-n:]
}

func continueOpenDecisionSourceKind(kind string) string {
	switch strings.ToLower(strings.TrimSpace(kind)) {
	case "candidate":
		return "candidate"
	case "decision":
		return "decision"
	default:
		return "decision"
	}
}

func continueOpenDecisionSourceCommand(lane Lane, kind, laneID string) string {
	parts := []string{"/rekit", "note", "-List", "-Kind", firstText(kind, "decision")}
	if laneID = firstText(laneID, lane.ID); strings.TrimSpace(laneID) != "" {
		parts = append(parts, "-Lane", laneID)
	}
	return continueCommand(parts...)
}

func continueDecisionNoteCommand(lane Lane, event map[string]any, whatIf bool) string {
	laneID := firstText(mission.Value(event, "lane"), lane.ID)
	if strings.TrimSpace(laneID) == "" {
		return ""
	}
	decision := mission.Value(event, "decision")
	if decision == "" || decision == "defer" || decision == "pending-user" {
		decision = "<accept|reject|defer|supersede>"
	}
	parts := []string{"/rekit", "note", "-Kind", "decision", "-Lane", laneID}
	parts = continueAppendCommandArg(parts, "-Subject", continueDecisionNoteSubject(event))
	parts = continueAppendCommandArg(parts, "-Summary", continueDecisionNoteSummary(event))
	parts = continueAppendCommandArg(parts, "-Decision", decision)
	parts = continueAppendCommandArg(parts, "-Reason", firstText(mission.Value(event, "reason"), "reviewed open candidate/decision item"))
	parts = continueAppendCommandArg(parts, "-TargetRef", mission.Value(event, "target"))
	if eventID := mission.Value(event, "eventId"); eventID != "" {
		parts = continueAppendCommandArg(parts, "-Related", eventID)
	}
	parts = continueAppendCommandArg(parts, "-EvidenceRefs", mission.Value(event, "evidenceRefs"))
	parts = continueAppendCommandArg(parts, "-BatchId", mission.Value(event, "batchId"))
	if whatIf {
		parts = append(parts, "-WhatIf")
	}
	return continueCommand(parts...)
}

func continueDecisionNoteSubject(event map[string]any) string {
	kind := firstText(mission.Value(event, "kind"), "decision")
	subject := mission.Value(event, "subject")
	if strings.TrimSpace(subject) == "" {
		subject = firstText(mission.Value(event, "summary"), "open item")
	}
	return "decision for " + kind + ": " + subject
}

func continueDecisionNoteSummary(event map[string]any) string {
	summary := mission.Value(event, "summary")
	if strings.TrimSpace(summary) == "" {
		summary = mission.Value(event, "subject")
	}
	return firstText(summary, "record reviewed open candidate/decision outcome")
}

func continueAppendCommandArg(parts []string, flag, value string) []string {
	value = strings.TrimSpace(value)
	if value == "" {
		return parts
	}
	return append(parts, flag, value)
}

func continueCommand(parts ...string) string {
	out := append([]string{}, parts...)
	for idx := range out {
		out[idx] = quoteCommandArg(out[idx])
	}
	return strings.Join(out, " ")
}

func (ctx continueContext) previewEvent(event map[string]any) ContinueEventPreview {
	kind := strings.ToLower(strings.TrimSpace(stringFrom(event, "kind")))
	if kind == "" {
		kind = "observation"
	}
	preview := ContinueEventPreview{
		EventID: stringFrom(event, "eventId"),
		Kind:    kind,
		Lane:    ctx.lane.ID,
		Subject: stringFrom(event, "subject"),
		Summary: stringFrom(event, "summary"),
	}
	switch kind {
	case "observation":
		if ctx.policy.AutoPublishSharedFacts {
			preview.Decision = "accept"
			preview.Reason = "shared observation"
			preview.WouldWrites = wouldFactKinds("observation", "decision")
		} else {
			preview.Decision = "defer"
			preview.Reason = "autoPublishSharedFacts disabled"
		}
	case "request":
		preview.Decision = "accept"
		preview.Reason = "would route request"
		preview.WouldWrites = wouldFactKinds("request", "decision")
		if ctx.policy.AutoRouteRequests {
			targetLane := stringFrom(event, "targetLane")
			if targetLane == "" {
				targetLane = ctx.manifest.WorkstreamDefaults["requestDefaultTargetLane"]
			}
			if err := canRouteRequest(ctx.inst.CaseRoot, targetLane); err != nil {
				preview.Decision = "defer"
				preview.Reason = err.Error()
			} else if !requestAlreadyRouted(ctx.inst.CaseRoot, targetLane, event) {
				preview.TargetLane = targetLane
				preview.WouldWrites = append(preview.WouldWrites, wouldLane(targetLane, "tasks.jsonl"), wouldLane(targetLane, "inbox.jsonl"))
			}
		} else {
			preview.Decision = "defer"
			preview.Reason = "autoRouteRequests disabled"
		}
	case "candidate":
		verification := verifyCandidate(ctx.inst.CaseRoot, ctx.policy, event)
		preview.Verification = verification
		authorityFile := candidateAuthorityFile(event)
		preview.AuthorityFile = authorityFile
		if authorityFile != "" {
			rows := candidateRows(event)
			preview.Rows = len(rows)
			if reason := ctx.authorityAppendReason(event, verification, authorityFile, rows); reason != "" {
				preview.Decision = "defer"
				preview.Reason = reason
				preview.WouldWrites = wouldFactKinds("candidate", "decision")
			} else {
				preview.Decision = "accept"
				preview.Reason = "passed authority append policy"
				preview.WouldWrites = append([]StartWrite{wouldAuthority(authorityFile), wouldRunArtifact("backups", authorityFile), wouldRunArtifact("diffs", sanitizedDiffName(authorityFile))}, wouldFactKinds("publication", "decision")...)
			}
		} else if ctx.policy.AutoAcceptLowRiskCandidates && boolFrom(verification, "hasEvidence") && verifierAccepted(ctx.policy, verification) {
			preview.Decision = "accept"
			preview.Reason = "candidate has evidence, verifier accepted, and does not touch authority"
			preview.WouldWrites = wouldFactKinds("candidate", "decision")
		} else {
			preview.Decision = "defer"
			preview.Reason = "candidate lacks evidence or policy disabled"
			preview.WouldWrites = wouldFactKinds("candidate", "decision")
		}
	case "publication":
		preview.Decision = "accept"
		preview.Reason = "publication event"
		preview.WouldWrites = wouldFactKinds("publication", "decision")
	default:
		preview.Decision = "accept"
		preview.Reason = "unknown kind treated as observation: " + kind
		preview.WouldWrites = wouldFactKinds("observation", "decision")
	}
	return preview
}

func (ctx continueContext) applyContinueEvent(event map[string]any, preview ContinueEventPreview, runID, batchID string) ([]StartWrite, error) {
	writes := []StartWrite{}
	kind := preview.Kind
	switch kind {
	case "observation":
		if preview.Decision == "accept" {
			if err := ctx.appendContinueFact(&writes, "observation", event); err != nil {
				return nil, err
			}
			if err := ctx.appendContinueFact(&writes, "decision", continueDecision(event, preview, runID, batchID)); err != nil {
				return nil, err
			}
		}
	case "request":
		if err := ctx.appendContinueFact(&writes, "request", event); err != nil {
			return nil, err
		}
		if preview.Decision == "accept" && preview.TargetLane != "" {
			routeWrites, err := routeContinueRequest(ctx.inst.CaseRoot, preview.TargetLane, event)
			if err != nil {
				return nil, err
			}
			writes = append(writes, routeWrites...)
		}
		if err := ctx.appendContinueFact(&writes, "decision", continueDecision(event, preview, runID, batchID)); err != nil {
			return nil, err
		}
	case "candidate":
		if preview.Verification != nil {
			event["verification"] = preview.Verification
			event["verifier"] = stringFrom(preview.Verification, "verifier")
			event["verifierVerdict"] = stringFrom(preview.Verification, "verdict")
			event["verifierConfidence"] = preview.Verification["confidence"]
		}
		if preview.Decision == "accept" {
			event["decision"] = "accepted-shared"
		} else {
			event["decision"] = "pending-user"
		}
		event["decisionReason"] = preview.Reason
		if err := ctx.appendContinueFact(&writes, "candidate", event); err != nil {
			return nil, err
		}
		if err := ctx.appendContinueFact(&writes, "decision", continueDecision(event, preview, runID, batchID)); err != nil {
			return nil, err
		}
	case "publication":
		if preview.Decision == "accept" {
			if err := ctx.appendContinueFact(&writes, "publication", event); err != nil {
				return nil, err
			}
			if err := ctx.appendContinueFact(&writes, "decision", continueDecision(event, preview, runID, batchID)); err != nil {
				return nil, err
			}
		}
	default:
		if err := ctx.appendContinueFact(&writes, "observation", event); err != nil {
			return nil, err
		}
		if err := ctx.appendContinueFact(&writes, "decision", continueDecision(event, preview, runID, batchID)); err != nil {
			return nil, err
		}
	}
	return writes, nil
}

func (result *ContinueResult) updateApplySummary(preview ContinueEventPreview) {
	switch preview.Kind {
	case "observation":
		if preview.Decision == "accept" {
			result.Summary.Observations++
		}
	case "request":
		result.Summary.Requests++
		if preview.TargetLane != "" && preview.Decision == "accept" {
			result.Summary.Routed++
		}
		if preview.Decision == "defer" || preview.Decision == "pending-user" {
			result.Summary.PendingUser++
		}
	case "candidate":
		result.Summary.Candidates++
		if preview.Decision == "accept" {
			if preview.AuthorityFile == "" {
				result.Summary.AcceptedCandidates++
			}
		} else {
			result.Summary.PendingUser++
		}
	case "publication":
		if preview.Decision == "accept" {
			result.Summary.Publications++
		}
	}
}

func continueDecision(event map[string]any, preview ContinueEventPreview, runID, batchID string) map[string]any {
	decision := "defer"
	if preview.Decision == "accept" {
		decision = "accept"
	}
	out := map[string]any{
		"schemaVersion": 1,
		"eventId":       stringFrom(event, "eventId"),
		"kind":          "decision",
		"lane":          stringFrom(event, "lane"),
		"subject":       firstText(stringFrom(event, "subject"), stringFrom(event, "summary"), preview.Kind),
		"summary":       stringFrom(event, "summary"),
		"decision":      decision,
		"confirmedBy":   "runtime",
		"reason":        preview.Reason,
		"runId":         runID,
		"batchId":       batchID,
		"time":          isoNow(),
	}
	if preview.AuthorityFile != "" {
		out["authorityFile"] = preview.AuthorityFile
	}
	return out
}

func routeContinueRequest(caseRoot, targetLane string, event map[string]any) ([]StartWrite, error) {
	if requestAlreadyRouted(caseRoot, targetLane, event) {
		return nil, nil
	}
	lane, err := readLaneByID(caseRoot, targetLane)
	if err != nil {
		return nil, err
	}
	laneRoot, err := laneRootPath(caseRoot, lane)
	if err != nil {
		return nil, err
	}
	now := isoNow()
	taskPath := LaneTasksJSONLPath(laneRoot)
	inboxPath := LaneInboxJSONLPath(laneRoot)
	sourceLane := stringFrom(event, "lane")
	requestID := stringFrom(event, "requestId")
	summary := firstText(stringFrom(event, "summary"), stringFrom(event, "subject"), stringFrom(event, "eventId"))
	task := map[string]any{"taskId": "task-" + strings.TrimPrefix(stringFrom(event, "eventId"), "evt-"), "eventId": stringFrom(event, "eventId"), "requestId": requestID, "kind": stringFrom(event, "kind"), "sourceLane": sourceLane, "summary": summary, "status": "open", "createdAt": now}
	inbox := map[string]any{"eventId": stringFrom(event, "eventId"), "requestId": requestID, "kind": "routed-request", "sourceLane": sourceLane, "summary": summary, "time": now}
	if err := mission.AppendJSONLine(taskPath, task); err != nil {
		return nil, err
	}
	if err := mission.AppendJSONLine(inboxPath, inbox); err != nil {
		return nil, err
	}
	return []StartWrite{
		{Path: relativePath(caseRoot, taskPath), Kind: "lane-jsonl", Action: "append", TargetPath: taskPath},
		{Path: relativePath(caseRoot, inboxPath), Kind: "lane-jsonl", Action: "append", TargetPath: inboxPath},
	}, nil
}

func writeContinueRunArtifacts(runRoot string, result ContinueResult) (string, string, error) {
	if err := os.MkdirAll(runRoot, 0o755); err != nil {
		return "", "", err
	}
	statusPath := filepath.Join(runRoot, "status.json")
	status := map[string]any{
		"schemaVersion":                  1,
		"runId":                          result.RunID,
		"batchId":                        result.BatchID,
		"summary":                        result.Summary,
		"autonomyProfile":                result.AutonomyProfile,
		"missionBrief":                   result.MissionBrief,
		"executorAction":                 result.ExecutorAction,
		"executionEvidenceReview":        result.ExecutionEvidenceReview,
		"executionEvidenceReviewSummary": result.ExecutionEvidenceReviewSummary,
		"reviewerDispatchIntakeHandoffs": result.ReviewerDispatchIntakeHandoffs,
		"reviewerDispatchIntakeSummary":  result.ReviewerDispatchIntakeSummary,
		"authorizedGateAdapterHandoffs":  result.AuthorizedGateAdapterHandoffs,
		"missionCommanderNextActions":    result.MissionCommanderNextActions,
		"missionCommanderActionQueue":    result.MissionCommanderActionQueue,
		"inputs":                         result.Inputs,
		"packetRefs":                     result.PacketRefs,
		"openRisks":                      result.OpenRisks,
		"time":                           isoNow(),
	}
	if err := writeJSON(statusPath, status); err != nil {
		return "", "", err
	}
	digestPath := filepath.Join(runRoot, "digest.md")
	if err := writeText(digestPath, continueDigestText(result)); err != nil {
		return "", "", err
	}
	return statusPath, digestPath, nil
}

func continueDigestText(result ContinueResult) string {
	lines := []string{
		"# rekit continue digest：" + result.RunID,
		"",
		"## 输入",
		"",
		"case: `" + result.CaseRoot + "`",
		"pack: `" + result.Pack + "`",
		"runId: `" + result.RunID + "`",
		"batchId: `" + result.BatchID + "`",
		"focus lane: `" + result.Lane.ID + "`",
		"autonomy mode: `" + firstText(result.AutonomyProfile.Mode, autonomy.ModeManualGate) + "`",
		"autonomy profile: `" + firstText(result.AutonomyProfile.ProfilePath, autonomy.RelPath(result.Lane.ID)) + "`",
		"autonomy ready: `" + fmt.Sprintf("%t", result.AutonomyProfile.Ready) + "`",
		"",
		"## Mission Control brief",
		"",
		"- summary: " + result.MissionBrief.Summary,
	}
	lines = appendMissionBriefDigestList(lines, "ready lanes", result.MissionBrief.ReadyLanes)
	lines = appendMissionBriefDigestList(lines, "blocked lanes", result.MissionBrief.BlockedLanes)
	lines = appendMissionBriefDigestList(lines, "pending gates", result.MissionBrief.PendingGates)
	lines = appendMissionBriefDigestList(lines, "authorized gates", result.MissionBrief.AuthorizedGates)
	lines = AppendAuthorizedGateAdapterHandoffDigest(lines, "Authorized gate adapter handoff", result.AuthorizedGateAdapterHandoffs)
	lines = appendMissionBriefDigestList(lines, "open decisions", result.MissionBrief.OpenDecisions)
	lines = appendMissionBriefDigestList(lines, "interventions", result.MissionBrief.Interventions)
	lines = appendMissionBriefDigestList(lines, "next agent actions", result.MissionBrief.NextAgentActions)
	lines = appendMissionBriefDigestList(lines, "escalations", result.MissionBrief.Escalations)
	lines = append(lines,
		"",
		"## Executor action snapshot",
		"",
		"- blocked: `"+fmt.Sprintf("%t", result.ExecutorAction.Blocked)+"`",
		"- ready: `"+fmt.Sprintf("%t", result.ExecutorAction.Ready)+"`",
		"- pending gates: `"+fmt.Sprintf("%d", result.ExecutorAction.PendingGates)+"`",
		"- open interventions: `"+fmt.Sprintf("%d", result.ExecutorAction.OpenInterventions)+"`",
		"- open decisions: `"+fmt.Sprintf("%d", result.ExecutorAction.OpenDecisions)+"`",
		"- reconcile required: `"+fmt.Sprintf("%t", result.ExecutorAction.ReconcileRequired)+"`",
		"- pending gate required: `"+fmt.Sprintf("%t", result.ExecutorAction.PendingGateRequired)+"`",
		"- open decision required: `"+fmt.Sprintf("%t", result.ExecutorAction.OpenDecisionRequired)+"`",
		"- resume command: `"+result.ExecutorAction.ResumeCommand+"`",
		"- handoff command: `"+result.ExecutorAction.HandoffCommand+"`",
		"- commander state: `"+result.ExecutorAction.MissionCommanderAction.State+"`",
		"- commander prompt: "+result.ExecutorAction.MissionCommanderAction.Prompt,
	)
	lines = appendMissionBriefDigestList(lines, "commander follow-up commands", result.ExecutorAction.MissionCommanderAction.FollowUpCommands)
	lines = appendMissionBriefDigestList(lines, "commander boundary", result.ExecutorAction.MissionCommanderAction.Boundary)
	lines = appendMissionCommanderActionQueue(lines, result.MissionCommanderActionQueue)
	lines = appendContinueMissionCommanderNextActions(lines, result.MissionCommanderNextActions)
	lines = appendContinueExecutionEvidenceReview(lines, result.ExecutionEvidenceReview, result.ExecutionEvidenceReviewSummary)
	lines = appendDigestReviewerWritebacks(lines, result.ReviewerWritebacks)
	lines = appendReviewerDispatchIntakeHandoff(lines, result.ReviewerDispatchIntakeHandoffs)
	lines = appendMissionBriefDigestList(lines, "blocker reasons", result.ExecutorAction.BlockerReasons)
	lines = appendMissionBriefDigestList(lines, "executor next actions", result.ExecutorAction.NextAgentActions)
	lines = appendMissionBriefDigestList(lines, "executor escalations", result.ExecutorAction.Escalations)
	lines = append(lines,
		"",
		"## packet refs",
		"",
	)
	if len(result.PacketRefs) == 0 {
		lines = append(lines, "- 无。")
	} else {
		for _, ref := range result.PacketRefs {
			lines = append(lines, "- `"+ref+"`")
		}
	}
	lines = append(lines, "", "## inputs", "")
	if len(result.Inputs) == 0 {
		lines = append(lines, "- 无。")
	} else {
		for _, ref := range result.Inputs {
			lines = append(lines, "- `"+ref+"`")
		}
	}
	lines = append(lines, "", "## outputs", "")
	lines = append(lines,
		fmt.Sprintf("- collected: %d", result.Summary.Collected),
		fmt.Sprintf("- observations: %d", result.Summary.Observations),
		fmt.Sprintf("- requests: %d", result.Summary.Requests),
		fmt.Sprintf("- routed: %d", result.Summary.Routed),
		fmt.Sprintf("- candidates: %d", result.Summary.Candidates),
		fmt.Sprintf("- acceptedCandidates: %d", result.Summary.AcceptedCandidates),
		fmt.Sprintf("- publications: %d", result.Summary.Publications),
		fmt.Sprintf("- authorityApplied: %d", result.Summary.AuthorityApplied),
		fmt.Sprintf("- pendingUser: %d", result.Summary.PendingUser),
		fmt.Sprintf("- skipped: %d", result.Summary.Skipped),
	)
	lines = append(lines, "", "## decisions", "")
	if len(result.Events) == 0 {
		lines = append(lines, "- 无。")
	} else {
		for _, event := range result.Events {
			lines = append(lines, fmt.Sprintf("- %s | lane=%s | decision=%s | reason=%s", firstText(event.Subject, event.Summary, event.EventID), event.Lane, event.Decision, event.Reason))
		}
	}
	lines = append(lines, "", "## open risks", "")
	if len(result.OpenRisks) == 0 {
		lines = append(lines, "- 无。")
	} else {
		lines = append(lines, result.OpenRisks...)
	}
	lines = append(lines, "")
	return strings.Join(lines, "\r\n")
}

func appendContinueMissionCommanderNextActions(lines []string, items []mission.MissionCommanderNextActionItem) []string {
	lines = append(lines, "", "## Mission Commander next actions", "")
	if len(items) == 0 {
		return append(lines, "- none")
	}
	shown := items
	if maxHandoffRows > 0 && len(shown) > maxHandoffRows {
		shown = shown[len(shown)-maxHandoffRows:]
	}
	for _, item := range shown {
		lines = append(lines, fmt.Sprintf("- state=%s source=%s blocked=%t requiresReview=%t command=`%s`", item.State, item.Source, item.Blocked, item.RequiresReview, item.Command))
		lines = appendMissionBriefDigestList(lines, "reasons", item.Reasons)
		lines = appendMissionBriefDigestList(lines, "boundary", item.Boundary)
	}
	return lines
}

func appendContinueExecutionEvidenceReview(lines []string, items []ExecutionEvidenceReviewItem, summary ExecutionEvidenceReviewSummary) []string {
	lines = append(lines, "", "## Execution evidence review", "")
	if len(items) == 0 {
		return append(lines, "- none")
	}
	lines = appendExecutionEvidenceReviewSummary(lines, summary)
	shown := items
	if maxHandoffRows > 0 && len(shown) > maxHandoffRows {
		shown = shown[len(shown)-maxHandoffRows:]
	}
	for _, item := range shown {
		lines = append(lines, fmt.Sprintf("- %s | status=%s | gateEventId=%s | action=%s", firstText(item.Subject, item.Summary, item.EventID), item.Status, item.GateEventID, firstText(item.Action, "none")))
		lines = appendMissionBriefDigestList(lines, "boundaryHits", item.BoundaryHits)
		if strings.TrimSpace(item.Escalation) != "" {
			lines = append(lines, "- escalation: "+item.Escalation)
		}
		lines = appendContinueExecutionEvidenceReportDetail(lines, item)
		lines = appendMissionBriefDigestList(lines, "outputRefs", item.OutputRefs)
		lines = appendMissionBriefDigestList(lines, "evidenceRefs", item.EvidenceRefs)
		lines = append(lines, "- review command: `"+item.ReviewCommand+"`")
		lines = append(lines, "- handoff command: `"+item.HandoffCommand+"`")
		lines = append(lines, "- commander state: "+item.MissionCommanderAction.State)
		lines = append(lines, "- commander primary: `"+item.MissionCommanderAction.PrimaryCommand+"`")
		lines = appendContinueExecutionEvidenceFollowThrough(lines, item.FollowThrough)
		lines = appendMissionBriefDigestList(lines, "commander follow-up", item.MissionCommanderAction.FollowUpCommands)
		lines = appendMissionBriefDigestList(lines, "review boundary", item.Boundary)
	}
	return lines
}

func appendExecutionEvidenceReviewSummary(lines []string, summary ExecutionEvidenceReviewSummary) []string {
	if summary.Total == 0 {
		return lines
	}
	lines = append(lines, fmt.Sprintf("- summary: total=%d readyForReview=%d mainEscalations=%d duplicates=%d outputRefs=%d evidenceRefs=%d boundaryHits=%d latestEventId=%s gateEventId=%s status=%s action=%s", summary.Total, summary.ReadyForReviewCount, summary.MainEscalationCount, summary.DuplicateCount, summary.OutputRefCount, summary.EvidenceRefCount, summary.BoundaryHitCount, summary.LatestEventID, summary.LatestGateEventID, summary.LatestStatus, firstText(summary.LatestAction, "none")))
	if strings.TrimSpace(summary.CurrentAction) != "" {
		lines = append(lines, "- summary current action: `"+summary.CurrentAction+"`")
	}
	if strings.TrimSpace(summary.ActionQueueSummary) != "" {
		lines = append(lines, "- summary action queue: "+summary.ActionQueueSummary)
	}
	if strings.TrimSpace(summary.LatestReviewCommand) != "" || strings.TrimSpace(summary.LatestHandoffCommand) != "" {
		lines = append(lines, "- summary handoff: review=`"+summary.LatestReviewCommand+"` handoff=`"+summary.LatestHandoffCommand+"`")
	}
	if strings.TrimSpace(summary.LatestCommanderState) != "" || strings.TrimSpace(summary.LatestCommanderPrimary) != "" {
		lines = append(lines, "- summary commander: state="+summary.LatestCommanderState+" primary=`"+summary.LatestCommanderPrimary+"`")
	}
	if strings.TrimSpace(summary.LatestExecutionReportPath) != "" || strings.TrimSpace(summary.LatestExecutionReportSHA256) != "" || strings.TrimSpace(summary.LatestAdapterID) != "" || strings.TrimSpace(summary.LatestAdapterStatus) != "" {
		lines = append(lines, "- summary report: path="+firstText(summary.LatestExecutionReportPath, "none")+" sha256="+firstText(summary.LatestExecutionReportSHA256, "none")+" adapterId="+firstText(summary.LatestAdapterID, "none")+" adapterStatus="+firstText(summary.LatestAdapterStatus, "none"))
	}
	lines = appendContinueExecutionEvidenceAdapterContext(lines, summary.LatestAdapterContext)
	for _, hit := range mission.LimitStrings(summary.LatestBoundaryHits, maxHandoffRows) {
		lines = append(lines, "- summary latest boundary hit: "+hit)
	}
	if strings.TrimSpace(summary.LatestEscalation) != "" {
		lines = append(lines, "- summary latest escalation: "+summary.LatestEscalation)
	}
	if strings.TrimSpace(summary.FollowThroughState) != "" || summary.OutcomeCount > 0 {
		lines = append(lines, fmt.Sprintf("- summary follow-through: state=%s outcomes=%d", summary.FollowThroughState, summary.OutcomeCount))
	}
	for _, boundary := range mission.LimitStrings(summary.Boundary, maxHandoffRows) {
		lines = append(lines, "- summary boundary: "+boundary)
	}
	return lines
}

func appendContinueExecutionEvidenceReportDetail(lines []string, item ExecutionEvidenceReviewItem) []string {
	if strings.TrimSpace(item.ExecutionReportPath) != "" || strings.TrimSpace(item.ExecutionReportSHA256) != "" {
		lines = append(lines, "- execution report: "+firstText(item.ExecutionReportPath, "none")+" sha256="+firstText(item.ExecutionReportSHA256, "none"))
	}
	if item.ActualBudget != nil {
		lines = append(lines, fmt.Sprintf("- actual budget: runtimeSeconds=%d diskMB=%d requests=%d", item.ActualBudget.RuntimeSeconds, item.ActualBudget.DiskMB, item.ActualBudget.Requests))
	}
	if strings.TrimSpace(item.AdapterID) != "" || strings.TrimSpace(item.AdapterStatus) != "" {
		lines = append(lines, fmt.Sprintf("- adapter report: adapterId=%s status=%s", item.AdapterID, item.AdapterStatus))
	}
	return appendContinueExecutionEvidenceAdapterContext(lines, item.AdapterContext)
}

func appendContinueExecutionEvidenceAdapterContext(lines []string, context *mission.ExecutionEvidenceAdapterContext) []string {
	if context == nil {
		return lines
	}
	lines = append(lines, fmt.Sprintf("- adapter context: id=%s status=%s entry=%s gateActions=%s recordOnlyAfterGate=%t toolingCatalogPath=%s", context.ID, context.Status, context.Entry, strings.Join(context.GateActions, ","), context.RecordOnlyAfterGate, context.ToolingCatalogPath))
	if strings.TrimSpace(context.Purpose) != "" {
		lines = append(lines, "- adapter context purpose: "+context.Purpose)
	}
	if len(context.SideEffects) > 0 {
		lines = append(lines, "- adapter context side effects: "+strings.Join(context.SideEffects, ","))
	}
	for _, guidance := range mission.LimitStrings(context.ReportGuidance, maxHandoffRows) {
		lines = append(lines, "- adapter context report guidance: "+guidance)
	}
	for _, guidance := range mission.LimitStrings(context.EvidenceGuidance, maxHandoffRows) {
		lines = append(lines, "- adapter context evidence guidance: "+guidance)
	}
	if len(context.StopConditionHints) > 0 {
		lines = append(lines, "- adapter context stop conditions: "+strings.Join(context.StopConditionHints, ","))
	}
	return lines
}

func appendContinueExecutionEvidenceFollowThrough(lines []string, follow mission.ExecutionEvidenceFollowThrough) []string {
	if strings.TrimSpace(follow.State) == "" && len(follow.Outcomes) == 0 {
		return lines
	}
	lines = append(lines, "- follow-through: state="+follow.State+" gateEventId="+follow.GateEventID+" outcomes="+fmt.Sprintf("%d", len(follow.Outcomes)))
	for _, outcome := range limitExecutionEvidenceOutcomes(follow.Outcomes, maxHandoffRows) {
		lines = append(lines, "  - outcome: name="+outcome.Name+" state="+outcome.State+" command=`"+outcome.Command+"` expected="+outcome.Expected)
		if strings.TrimSpace(outcome.When) != "" {
			lines = append(lines, "    - when: "+outcome.When)
		}
		for _, action := range mission.LimitStrings(outcome.Actions, maxHandoffRows) {
			lines = append(lines, "    - action: "+action)
		}
		for _, command := range mission.LimitStrings(outcome.VerificationCommands, maxHandoffRows) {
			lines = append(lines, "    - verification: "+command)
		}
		for _, evidence := range mission.LimitStrings(outcome.Evidence, maxHandoffRows) {
			lines = append(lines, "    - evidence: "+evidence)
		}
	}
	if strings.TrimSpace(follow.ActionQueue.Summary) != "" {
		lines = append(lines, "  - queue: "+follow.ActionQueue.Summary)
	}
	return lines
}

func appendMissionBriefDigestList(lines []string, label string, items []string) []string {
	if len(items) == 0 {
		return append(lines, "- "+label+": none")
	}
	lines = append(lines, "- "+label+":")
	for _, item := range items {
		lines = append(lines, "  - "+item)
	}
	return lines
}

func firstText(values ...string) string {
	for _, value := range values {
		if strings.TrimSpace(value) != "" {
			return strings.TrimSpace(value)
		}
	}
	return ""
}

func (ctx continueContext) authorityAppendReason(event map[string]any, verification map[string]any, authorityFile string, rows []any) string {
	if !containsString(ctx.manifest.AuthorityFiles, authorityFile) {
		return "authority file is not allowed: " + authorityFile
	}
	if strings.EqualFold(strings.TrimSpace(ctx.policy.AuthorityAutoAppend), "never") {
		return "authority auto append disabled"
	}
	confidence := eventConfidence(event)
	if confidence < ctx.policy.MinConfidence {
		return fmt.Sprintf("confidence below threshold: %s < %s", formatFloat(confidence), formatFloat(ctx.policy.MinConfidence))
	}
	if ctx.policy.RequireEvidence && !eventHasEvidence(ctx.inst.CaseRoot, event) {
		return "missing evidence"
	}
	if !verifierAccepted(ctx.policy, verification) {
		return "missing accepted verifier verdict"
	}
	path, err := refsf.SafeJoin(ctx.inst.CaseRoot, authorityFile)
	if err != nil {
		return err.Error()
	}
	if !refsf.Exists(path) {
		return "missing authority file: " + authorityFile
	}
	if !strings.HasSuffix(strings.ToLower(authorityFile), ".csv") {
		return "only csv authority append is automated"
	}
	if ctx.policy.RequireSchemaValid && !candidateCSVSchemaValid(path, rows) {
		return "candidate row does not match authority csv schema"
	}
	if ctx.policy.RequireNoConflict && candidateCSVConflict(path, rows) {
		return "authority key conflict"
	}
	if len(rows) == 0 {
		return "no candidate rows"
	}
	for _, row := range rows {
		if text, ok := row.(string); ok && strings.ContainsAny(text, "\r\n") {
			return "candidate row contains newline"
		}
	}
	if len(rows) > ctx.policy.MaxAuthorityRowsPerRun {
		return fmt.Sprintf("too many rows: %d > %d", len(rows), ctx.policy.MaxAuthorityRowsPerRun)
	}
	return ""
}

func laneOutputEvents(caseRoot string, lane Lane, m *manifest.Manifest) ([]map[string]any, error) {
	laneRoot, err := laneRootPath(caseRoot, lane)
	if err != nil {
		return nil, err
	}
	workspace, err := refsf.SafeJoin(caseRoot, lane.Workspace)
	if err != nil {
		return nil, err
	}
	var events []map[string]any
	for _, path := range LaneOutputJSONLPaths(laneRoot, workspace) {
		items, err := mission.ReadJSONLineObjects(path)
		if err != nil {
			return nil, err
		}
		for _, item := range items {
			events = append(events, ensurePreviewEventID(lane.ID, item))
		}
	}
	lowering := filepath.Join(workspace, "lowering_requests.csv")
	if refsf.Exists(lowering) {
		rows, err := readCSVRows(lowering)
		if err != nil {
			return nil, err
		}
		for _, row := range rows {
			status := strings.ToLower(strings.TrimSpace(row["status"]))
			if terminalStatus(status) {
				continue
			}
			summary := row["reason"]
			if strings.TrimSpace(summary) == "" {
				summary = "lowering request"
			}
			event := map[string]any{"kind": "request", "lane": lane.ID, "targetLane": m.WorkstreamDefaults["requestDefaultTargetLane"], "requestId": row["request_id"], "summary": summary, "evidence": row["evidence"], "priority": row["priority"], "status": "open", "source": relativePath(caseRoot, lowering)}
			if strings.TrimSpace(row["request_id"]) != "" {
				event["eventId"] = "evt-" + hashText(lane.ID + "|request|" + row["request_id"])[:16]
			}
			events = append(events, ensurePreviewEventID(lane.ID, event))
		}
	}
	candidatesDir := filepath.Join(workspace, "candidates")
	entries, err := os.ReadDir(candidatesDir)
	if err != nil && !os.IsNotExist(err) {
		return nil, err
	}
	if err == nil {
		for _, entry := range entries {
			if entry.IsDir() || !strings.EqualFold(filepath.Ext(entry.Name()), ".csv") {
				continue
			}
			path := filepath.Join(candidatesDir, entry.Name())
			rows, err := readCSVRows(path)
			if err != nil {
				return nil, err
			}
			for _, row := range rows {
				status := strings.ToLower(strings.TrimSpace(row["status"]))
				if terminalStatus(status) {
					continue
				}
				event := map[string]any{"kind": "candidate", "lane": lane.ID, "target": strings.TrimSuffix(entry.Name(), filepath.Ext(entry.Name())), "summary": "candidate from " + entry.Name(), "evidence": row["evidence"], "confidence": row["confidence"], "status": "open", "source": relativePath(caseRoot, path), "row": row}
				events = append(events, ensurePreviewEventID(lane.ID, event))
			}
		}
	}
	return events, nil
}

func continueInputRefs(caseRoot string, lane Lane) ([]string, error) {
	laneRoot, err := laneRootPath(caseRoot, lane)
	if err != nil {
		return nil, err
	}
	workspace, err := refsf.SafeJoin(caseRoot, lane.Workspace)
	if err != nil {
		return nil, err
	}
	refs := []string{}
	for _, path := range LaneOutputJSONLPaths(laneRoot, workspace) {
		if refsf.Exists(path) {
			refs = append(refs, relativePath(caseRoot, path))
		}
	}
	return refs, nil
}

func continuePacketRefs(caseRoot string, lane Lane) ([]string, error) {
	workspace, err := refsf.SafeJoin(caseRoot, lane.Workspace)
	if err != nil {
		return nil, err
	}
	entries, err := os.ReadDir(workspace)
	if os.IsNotExist(err) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	packets := []string{}
	for _, entry := range entries {
		if !entry.IsDir() && strings.EqualFold(filepath.Ext(entry.Name()), ".md") {
			packets = append(packets, relativePath(caseRoot, filepath.Join(workspace, entry.Name())))
		}
	}
	sort.Strings(packets)
	if len(packets) > 10 {
		packets = packets[len(packets)-10:]
	}
	return packets, nil
}

func readContinuePolicy(caseRoot string) (continuePolicy, error) {
	policy := defaultContinuePolicy()
	path, err := refsf.SafeJoin(caseRoot, ".rekit/policy.yml")
	if err != nil {
		return policy, err
	}
	b, err := os.ReadFile(path)
	if os.IsNotExist(err) {
		return policy, nil
	}
	if err != nil {
		return policy, err
	}
	values := map[string]string{}
	for line := range strings.SplitSeq(strings.ReplaceAll(string(b), "\r\n", "\n"), "\n") {
		trim := strings.TrimSpace(line)
		key, value, ok := strings.Cut(trim, ":")
		if trim == "" || strings.HasPrefix(trim, "#") || !ok {
			continue
		}
		values[strings.TrimSpace(key)] = strings.Trim(strings.TrimSpace(value), `"'`)
	}
	policy.AutoVerify = boolPolicy(values, "autoVerify", policy.AutoVerify)
	policy.AutoRouteRequests = boolPolicy(values, "autoRouteRequests", policy.AutoRouteRequests)
	policy.AutoPublishSharedFacts = boolPolicy(values, "autoPublishSharedFacts", policy.AutoPublishSharedFacts)
	policy.AutoAcceptLowRiskCandidates = boolPolicy(values, "autoAcceptLowRiskCandidates", policy.AutoAcceptLowRiskCandidates)
	if value := strings.TrimSpace(values["authorityAutoAppend"]); value != "" {
		policy.AuthorityAutoAppend = value
	}
	policy.RequireEvidence = boolPolicy(values, "requireEvidence", policy.RequireEvidence)
	policy.RequireVerifier = boolPolicy(values, "requireVerifier", policy.RequireVerifier)
	policy.RequireNoConflict = boolPolicy(values, "requireNoConflict", policy.RequireNoConflict)
	policy.RequireSchemaValid = boolPolicy(values, "requireSchemaValid", policy.RequireSchemaValid)
	policy.RequireBackup = boolPolicy(values, "requireBackup", policy.RequireBackup)
	policy.RequireDiff = boolPolicy(values, "requireDiff", policy.RequireDiff)
	if n, err := strconv.ParseFloat(strings.TrimSpace(values["minConfidence"]), 64); err == nil {
		policy.MinConfidence = n
	}
	if n, err := strconv.Atoi(strings.TrimSpace(values["maxAuthorityRowsPerRun"])); err == nil {
		policy.MaxAuthorityRowsPerRun = n
	}
	return policy, nil
}

func defaultContinuePolicy() continuePolicy {
	return continuePolicy{AutoVerify: true, AutoRouteRequests: true, AutoPublishSharedFacts: true, AutoAcceptLowRiskCandidates: true, AuthorityAutoAppend: "conditional", RequireEvidence: true, RequireVerifier: true, MinConfidence: 0.90, RequireNoConflict: true, RequireSchemaValid: true, RequireBackup: true, RequireDiff: true, MaxAuthorityRowsPerRun: 10}
}

func boolPolicy(values map[string]string, key string, def bool) bool {
	value := strings.ToLower(strings.TrimSpace(values[key]))
	switch value {
	case "true", "yes", "1":
		return true
	case "false", "no", "0":
		return false
	default:
		return def
	}
}

func verifyCandidate(caseRoot string, policy continuePolicy, event map[string]any) map[string]any {
	confidence := eventConfidence(event)
	hasEvidence := eventHasEvidence(caseRoot, event)
	authorityFile := candidateAuthorityFile(event)
	schemaValid := true
	conflict := false
	if authorityFile != "" {
		if path, err := refsf.SafeJoin(caseRoot, authorityFile); err == nil {
			rows := candidateRows(event)
			schemaValid = candidateCSVSchemaValid(path, rows)
			conflict = candidateCSVConflict(path, rows)
		} else {
			schemaValid = false
			conflict = true
		}
	}
	reasons := []string{}
	if policy.RequireEvidence && !hasEvidence {
		reasons = append(reasons, "missing evidence")
	}
	if confidence < policy.MinConfidence {
		reasons = append(reasons, fmt.Sprintf("confidence below threshold: %s < %s", formatFloat(confidence), formatFloat(policy.MinConfidence)))
	}
	if policy.RequireSchemaValid && !schemaValid {
		reasons = append(reasons, "schema invalid")
	}
	if conflict {
		reasons = append(reasons, "authority conflict")
	}
	verdict := "accepted"
	if len(reasons) > 0 {
		verdict = "rejected"
	}
	verifier := "rule-verifier"
	if !policy.AutoVerify {
		verifier = "policy-disabled"
		verdict = "skipped"
		reasons = []string{"autoVerify disabled"}
	}
	return map[string]any{"verifier": verifier, "verdict": verdict, "confidence": confidence, "hasEvidence": hasEvidence, "schemaValid": schemaValid, "conflict": conflict, "reasons": reasons}
}

func verifierAccepted(policy continuePolicy, verification map[string]any) bool {
	if !policy.RequireVerifier {
		return true
	}
	return strings.TrimSpace(stringFrom(verification, "verifier")) != "" && strings.EqualFold(stringFrom(verification, "verdict"), "accepted")
}

func eventConfidence(event map[string]any) float64 {
	text := strings.ToLower(strings.TrimSpace(stringFrom(event, "confidence")))
	if text == "" {
		return 0
	}
	if n, err := strconv.ParseFloat(text, 64); err == nil {
		return n
	}
	switch text {
	case "high":
		return 0.95
	case "medium_high", "medium-high":
		return 0.82
	case "medium":
		return 0.65
	case "medium_low", "medium-low":
		return 0.45
	case "low":
		return 0.25
	default:
		return 0
	}
}

func eventHasEvidence(caseRoot string, event map[string]any) bool {
	for _, item := range evidenceItems(event) {
		if strings.ContainsAny(item, `/\`) || filepath.IsAbs(item) {
			path := item
			if !filepath.IsAbs(path) {
				joined, err := refsf.SafeJoin(caseRoot, filepath.ToSlash(path))
				if err != nil {
					continue
				}
				path = joined
			}
			if refsf.Exists(path) {
				return true
			}
		} else if len(item) >= 8 {
			return true
		}
	}
	return false
}

func evidenceItems(event map[string]any) []string {
	value, ok := event["evidence"]
	if !ok || value == nil {
		return nil
	}
	if list, ok := value.([]any); ok {
		items := []string{}
		for _, item := range list {
			if text := strings.TrimSpace(fmt.Sprint(item)); text != "" {
				items = append(items, text)
			}
		}
		return items
	}
	return splitScalarList(fmt.Sprint(value))
}

func candidateAuthorityFile(event map[string]any) string {
	for _, key := range []string{"authorityFile", "authorityCsv", "targetFile", "file"} {
		if value := strings.TrimSpace(stringFrom(event, key)); value != "" {
			return filepath.ToSlash(value)
		}
	}
	return ""
}

func candidateRows(event map[string]any) []any {
	for _, key := range []string{"rows", "row", "csvRow"} {
		value, ok := event[key]
		if !ok || value == nil {
			continue
		}
		if list, ok := value.([]any); ok {
			return list
		}
		return []any{value}
	}
	return nil
}

func candidateCSVSchemaValid(csvPath string, rows []any) bool {
	if !refsf.Exists(csvPath) || len(rows) == 0 {
		return false
	}
	header, err := csvHeader(csvPath)
	if err != nil || len(header) == 0 || strings.TrimSpace(header[0]) == "" {
		return false
	}
	for _, row := range rows {
		switch t := row.(type) {
		case string:
			if strings.TrimSpace(t) == "" || strings.ContainsAny(t, "\r\n") {
				return false
			}
			records, err := csv.NewReader(strings.NewReader(strings.Join(header, ",") + "\n" + t + "\n")).ReadAll()
			if err != nil || len(records) != 2 {
				return false
			}
		case map[string]any:
			for _, column := range header {
				if _, ok := t[strings.Trim(column, `"`)]; !ok {
					return false
				}
			}
		default:
			m, ok := structToMap(t)
			if !ok {
				return false
			}
			for _, column := range header {
				if _, ok := m[strings.Trim(column, `"`)]; !ok {
					return false
				}
			}
		}
	}
	return true
}

func candidateCSVConflict(csvPath string, rows []any) bool {
	if !refsf.Exists(csvPath) || len(rows) == 0 {
		return false
	}
	header, err := csvHeader(csvPath)
	if err != nil || len(header) == 0 {
		return false
	}
	key := candidateRowKey(rows[0], strings.Trim(header[0], `"`))
	if key == "" {
		return false
	}
	file, err := os.Open(csvPath)
	if err != nil {
		return false
	}
	defer file.Close()
	reader := csv.NewReader(file)
	records, err := reader.ReadAll()
	if err != nil || len(records) <= 1 {
		return false
	}
	for _, record := range records[1:] {
		if len(record) > 0 && record[0] == key {
			return true
		}
	}
	return false
}

func csvHeader(path string) ([]string, error) {
	b, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	first, _, _ := strings.Cut(strings.ReplaceAll(string(b), "\r\n", "\n"), "\n")
	parts := strings.Split(first, ",")
	for i := range parts {
		parts[i] = strings.TrimSpace(strings.Trim(parts[i], `"`))
	}
	return parts, nil
}

func candidateRowKey(row any, firstColumn string) string {
	switch t := row.(type) {
	case string:
		key, _, _ := strings.Cut(t, ",")
		return strings.Trim(key, `"`)
	case map[string]any:
		return strings.TrimSpace(fmt.Sprint(t[firstColumn]))
	default:
		m, ok := structToMap(t)
		if !ok {
			return ""
		}
		return strings.TrimSpace(fmt.Sprint(m[firstColumn]))
	}
}

func canRouteRequest(caseRoot, laneID string) error {
	lane, err := readLaneByID(caseRoot, laneID)
	if err != nil {
		return fmt.Errorf("target lane does not exist: %s", laneID)
	}
	status := strings.ToLower(strings.TrimSpace(lane.Status))
	if status == "archived" || status == "paused" {
		return fmt.Errorf("target lane is not open: %s", laneID)
	}
	return nil
}

func requestAlreadyRouted(caseRoot, laneID string, event map[string]any) bool {
	lane, err := readLaneByID(caseRoot, laneID)
	if err != nil {
		return false
	}
	laneRoot, err := laneRootPath(caseRoot, lane)
	if err != nil {
		return false
	}
	tasks, err := mission.ReadJSONLineObjects(LaneTasksJSONLPath(laneRoot))
	if err != nil {
		return false
	}
	sourceLane := stringFrom(event, "lane")
	requestID := stringFrom(event, "requestId")
	eventID := stringFrom(event, "eventId")
	for _, task := range tasks {
		if requestID != "" {
			if stringFrom(task, "requestId") == requestID && stringFrom(task, "sourceLane") == sourceLane {
				return true
			}
		} else if eventID != "" && stringFrom(task, "eventId") == eventID {
			return true
		}
	}
	return false
}

func readCSVRows(path string) ([]map[string]string, error) {
	file, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer file.Close()
	reader := csv.NewReader(file)
	records, err := reader.ReadAll()
	if err != nil {
		return nil, err
	}
	if len(records) == 0 {
		return nil, nil
	}
	header := records[0]
	rows := []map[string]string{}
	for _, record := range records[1:] {
		row := map[string]string{}
		for i, key := range header {
			if i < len(record) {
				row[key] = record[i]
			} else {
				row[key] = ""
			}
		}
		rows = append(rows, row)
	}
	return rows, nil
}

func terminalStatus(status string) bool {
	switch strings.ToLower(strings.TrimSpace(status)) {
	case "resolved", "done", "closed", "accepted", "rejected":
		return true
	default:
		return false
	}
}

func ensurePreviewEventID(laneID string, event map[string]any) map[string]any {
	out := copyEvent(event)
	if strings.TrimSpace(stringFrom(out, "eventId")) == "" {
		out["eventId"] = generatedEventID(laneID, out)
	}
	return out
}

func generatedEventID(laneID string, event map[string]any) string {
	b, _ := json.Marshal(event)
	return "evt-" + hashText(laneID + "|" + string(b))[:16]
}

func hashText(text string) string {
	sum := sha256.Sum256([]byte(text))
	return hex.EncodeToString(sum[:])
}

func copyEvent(event map[string]any) map[string]any {
	out := map[string]any{}
	maps.Copy(out, event)
	return out
}

func uniqueStrings(values []string) []string {
	seen := map[string]bool{}
	out := []string{}
	for _, value := range values {
		if strings.TrimSpace(value) == "" || seen[value] {
			continue
		}
		seen[value] = true
		out = append(out, value)
	}
	return out
}

func splitScalarList(value string) []string {
	if strings.TrimSpace(value) == "" {
		return nil
	}
	parts := strings.FieldsFunc(value, func(r rune) bool { return r == ',' || r == ';' })
	out := []string{}
	for _, part := range parts {
		part = strings.TrimSpace(part)
		if part != "" {
			out = append(out, part)
		}
	}
	return out
}

func stringFrom(m map[string]any, key string) string {
	return objectText(m[key])
}

func boolFrom(m map[string]any, key string) bool {
	v, ok := m[key].(bool)
	return ok && v
}

func structToMap(value any) (map[string]any, bool) {
	b, err := json.Marshal(value)
	if err != nil {
		return nil, false
	}
	var out map[string]any
	if err := json.Unmarshal(b, &out); err != nil {
		return nil, false
	}
	return out, true
}

func containsString(values []string, target string) bool {
	return slices.Contains(values, target)
}

func formatFloat(value float64) string {
	return strconv.FormatFloat(value, 'f', -1, 64)
}

func sanitizedDiffName(rel string) string {
	replacer := strings.NewReplacer("/", "_", "\\", "_", ":", "_", "*", "_", "?", "_", "\"", "_", "<", "_", ">", "_", "|", "_")
	return replacer.Replace(rel) + ".diff"
}

func wouldLane(laneID, file string) StartWrite {
	return StartWrite{Path: relJoin(".rekit", "lanes", laneID, file), Kind: "lane-jsonl", Action: "would-append"}
}

func wouldAuthority(rel string) StartWrite {
	return StartWrite{Path: rel, Kind: "authority-csv", Action: "would-append"}
}

func wouldRunArtifact(kind, rel string) StartWrite {
	return StartWrite{Path: relJoin(".rekit", "runs", continuePreviewRunID, kind, rel), Kind: "run-artifact", Action: "would-create"}
}

func riskLine(preview ContinueEventPreview) string {
	subject := preview.Subject
	if subject == "" {
		subject = preview.Summary
	}
	if subject == "" {
		subject = preview.EventID
	}
	return fmt.Sprintf("%s: %s | lane=%s | reason=%s", preview.Kind, subject, preview.Lane, preview.Reason)
}
