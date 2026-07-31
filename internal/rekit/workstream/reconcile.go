package workstream

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/shuiyu486/re-context-kits/internal/rekit/instance"
	"github.com/shuiyu486/re-context-kits/internal/rekit/manifest"
	"github.com/shuiyu486/re-context-kits/internal/rekit/mission"
)

type ReconcileOptions struct {
	Selector       string
	InterventionID string
	Actor          string
	Executor       string
	Reason         string
	Summary        string
}

type InterventionSummary struct {
	EventID    string `json:"eventId"`
	Lane       string `json:"lane"`
	Subject    string `json:"subject,omitempty"`
	Summary    string `json:"summary,omitempty"`
	Action     string `json:"action,omitempty"`
	Status     string `json:"status,omitempty"`
	Target     string `json:"target,omitempty"`
	Scope      string `json:"scope,omitempty"`
	ApprovedBy string `json:"approvedBy,omitempty"`
	BatchID    string `json:"batchId,omitempty"`
}

type ReconcileResult struct {
	SchemaVersion                      int                                         `json:"schemaVersion"`
	Command                            string                                      `json:"command"`
	CaseRoot                           string                                      `json:"caseRoot"`
	RepoRoot                           string                                      `json:"repoRoot"`
	Pack                               string                                      `json:"pack"`
	IsMutation                         bool                                        `json:"isMutation"`
	Applied                            bool                                        `json:"applied"`
	RequiresConfirmation               bool                                        `json:"requiresConfirmation"`
	Selector                           string                                      `json:"selector"`
	Lane                               Lane                                        `json:"lane"`
	Intervention                       InterventionSummary                         `json:"intervention"`
	ResolutionEventID                  string                                      `json:"resolutionEventId,omitempty"`
	Actor                              string                                      `json:"actor"`
	Executor                           string                                      `json:"executor"`
	PreviousExecutor                   string                                      `json:"previousExecutor,omitempty"`
	ExecutorGeneration                 int                                         `json:"executorGeneration"`
	MissionBrief                       mission.Brief                               `json:"missionBrief"`
	AuthorizedGateAdapterHandoffs      []AuthorizedGateAdapterHandoff              `json:"authorizedGateAdapterHandoffs,omitempty"`
	ReviewerDispatchIntakeHandoffs     []ReviewerDispatchIntakeHandoff             `json:"reviewerDispatchIntakeHandoffs,omitempty"`
	ReviewerDispatchIntakeSummary      ReviewerDispatchIntakeSummary               `json:"reviewerDispatchIntakeSummary"`
	ReviewerPacketRetirementHandoffs   []ReviewerPacketRetirementHandoff           `json:"reviewerPacketRetirementHandoffs,omitempty"`
	ReviewerPacketRetirementSummary    ReviewerPacketRetirementSummary             `json:"reviewerPacketRetirementSummary"`
	PendingGateHandoffs                []ContinuePendingGateHandoff                `json:"pendingGateHandoffs,omitempty"`
	OpenDecisionHandoffs               []ContinueOpenDecisionHandoff               `json:"openDecisionHandoffs,omitempty"`
	ExecutorAction                     laneExecutorAction                          `json:"executorAction"`
	MissionCommanderAction             mission.MissionCommanderAction              `json:"missionCommanderAction"`
	MissionCommanderNextActions        []mission.MissionCommanderNextActionItem    `json:"missionCommanderNextActions,omitempty"`
	MissionCommanderActionQueue        mission.MissionCommanderActionQueue         `json:"missionCommanderActionQueue"`
	MissionCommanderDriverReceipt      *MissionCommanderDriverReceipt              `json:"missionCommanderDriverReceipt,omitempty"`
	ReplacementExecutorTakeoverPackage *mission.ReplacementExecutorTakeoverPackage `json:"replacementExecutorTakeoverPackage,omitempty"`
	WouldWrites                        []StartWrite                                `json:"wouldWrites,omitempty"`
	Writes                             []StartWrite                                `json:"writes,omitempty"`
	BlockedActions                     []string                                    `json:"blockedActions"`
	NextSteps                          []string                                    `json:"nextSteps"`
}

type reconcileContext struct {
	inst               instance.Instance
	manifest           *manifest.Manifest
	board              board
	selector           string
	lane               Lane
	facts              mission.LedgerFacts
	intervention       map[string]any
	existingResolution map[string]any
	actor              string
	executor           string
	reason             string
	summary            string
}

func ReconcilePreview(repoRoot, caseRoot, pack string, opt ReconcileOptions) (ReconcileResult, error) {
	ctx, err := newReconcileContext(repoRoot, caseRoot, pack, opt, false)
	if err != nil {
		return ReconcileResult{}, err
	}
	return ctx.result(false, false, true, ctx.plannedWrites()), nil
}

func ReconcileApply(repoRoot, caseRoot, pack string, opt ReconcileOptions) (result ReconcileResult, err error) {
	ctx, err := newReconcileContext(repoRoot, caseRoot, pack, opt, true)
	if err != nil {
		return ReconcileResult{}, err
	}
	lease, err := acquireLaneMutationLock(ctx.inst.CaseRoot, ctx.lane.ID)
	if err != nil {
		return ReconcileResult{}, err
	}
	defer func() {
		if unlockErr := lease.Unlock(); unlockErr != nil {
			err = errors.Join(err, unlockErr)
			result = ReconcileResult{}
		}
	}()
	ctx, err = newReconcileContext(repoRoot, caseRoot, pack, opt, true)
	if err != nil {
		return ReconcileResult{}, err
	}
	if err := lease.Validate(); err != nil {
		return ReconcileResult{}, err
	}
	now := isoNow()
	writes := []StartWrite{}
	laneRoot, err := laneRootPath(ctx.inst.CaseRoot, ctx.lane)
	if err != nil {
		return ReconcileResult{}, err
	}
	sourceID := mission.Value(ctx.intervention, "eventId")
	resolutionID := ""
	if ctx.existingResolution != nil {
		now = strings.TrimSpace(mission.Value(ctx.existingResolution, "time"))
		resolutionID = mission.Value(ctx.existingResolution, "eventId")
		if resolutionID == "" || now == "" {
			return ReconcileResult{}, fmt.Errorf("existing reconcile resolution for %s is missing eventId or time; refusing replay", sourceID)
		}
		if existingExecutor := strings.TrimSpace(mission.Value(ctx.existingResolution, "executor")); existingExecutor != "" && !strings.EqualFold(existingExecutor, ctx.executor) {
			return ReconcileResult{}, fmt.Errorf("existing reconcile resolution executor differs for %s; refusing replay", sourceID)
		}
		if existingActor := strings.TrimSpace(mission.Value(ctx.existingResolution, "actor")); existingActor != "" && existingActor != ctx.actor {
			return ReconcileResult{}, fmt.Errorf("existing reconcile resolution actor differs for %s; refusing replay", sourceID)
		}
		if existingReason := strings.TrimSpace(mission.Value(ctx.existingResolution, "reason")); existingReason != "" && existingReason != ctx.reason {
			return ReconcileResult{}, fmt.Errorf("existing reconcile resolution reason differs for %s; refusing replay", sourceID)
		}
		rel, path, err := mission.FactPath(ctx.inst.CaseRoot, "intervention")
		if err != nil {
			return ReconcileResult{}, err
		}
		writes = append(writes, StartWrite{Path: rel, Kind: "fact-jsonl", Action: "already-appended", TargetPath: path})
	} else {
		resolutionID = eventID(ctx.lane.ID, "intervention-resolved-"+sourceID, now)
		resolution := ctx.reconcileResolution(sourceID, resolutionID, now)
		rel, path, err := mission.AppendFact(ctx.inst.CaseRoot, "intervention", resolution)
		if err != nil {
			return ReconcileResult{}, err
		}
		writes = append(writes, StartWrite{Path: rel, Kind: "fact-jsonl", Action: "append", TargetPath: path})
	}

	laneEventPath := LaneEventsJSONLPath(laneRoot)
	laneEvent := map[string]any{
		"eventId":           eventID(ctx.lane.ID, "intervention-reconciled-"+sourceID, now),
		"kind":              "intervention-reconciled",
		"lane":              ctx.lane.ID,
		"time":              now,
		"summary":           "reconciled intervention: " + interventionLabel(ctx.intervention),
		"sourceEventId":     sourceID,
		"resolvesEventId":   sourceID,
		"resolutionEventId": resolutionID,
		"actor":             ctx.actor,
		"executor":          ctx.executor,
		"reason":            ctx.reason,
	}
	laneEventWrite, err := appendReconcileLaneEvent(ctx.inst.CaseRoot, laneEventPath, "append-intervention-reconciled", laneEvent)
	if err != nil {
		return ReconcileResult{}, err
	}
	writes = append(writes, laneEventWrite)

	previousExecutor := strings.TrimSpace(ctx.lane.CurrentExecutor)
	generation := max(ctx.lane.ExecutorGeneration, 0)
	if ctx.existingResolution != nil {
		if strings.TrimSpace(ctx.lane.CurrentExecutor) != "" && strings.EqualFold(ctx.lane.CurrentExecutor, ctx.executor) && ctx.lane.ExecutorGeneration > 0 {
			generation = ctx.lane.ExecutorGeneration
		} else if ctx.executor != "" && generation == 0 {
			generation = 1
		}
		if ctx.executor != "" {
			ctx.lane.CurrentExecutor = ctx.executor
			ctx.lane.ExecutorGeneration = generation
			ctx.lane.LastTakeoverAt = now
			ctx.lane.LastTakeoverBy = ctx.actor
			ctx.lane.LastTakeoverReason = firstText(ctx.reason, "reconcile intervention "+sourceID)
			if !strings.EqualFold(previousExecutor, ctx.executor) {
				takeoverEvent := map[string]any{
					"eventId":              eventID(ctx.lane.ID, "executor-takeover-"+ctx.executor, now),
					"kind":                 "executor-takeover",
					"lane":                 ctx.lane.ID,
					"time":                 now,
					"summary":              "executor takeover: " + ctx.executor,
					"previousExecutor":     previousExecutor,
					"currentExecutor":      ctx.executor,
					"executorGeneration":   generation,
					"sourceInterventionId": sourceID,
					"actor":                ctx.actor,
					"reason":               ctx.reason,
				}
				takeoverWrite, err := appendReconcileLaneEvent(ctx.inst.CaseRoot, laneEventPath, "append-executor-takeover", takeoverEvent)
				if err != nil {
					return ReconcileResult{}, err
				}
				writes = append(writes, takeoverWrite)
			}
		}
	} else if ctx.executor != "" && !strings.EqualFold(previousExecutor, ctx.executor) {
		generation++
		ctx.lane.CurrentExecutor = ctx.executor
		ctx.lane.ExecutorGeneration = generation
		ctx.lane.LastTakeoverAt = now
		ctx.lane.LastTakeoverBy = ctx.actor
		ctx.lane.LastTakeoverReason = firstText(ctx.reason, "reconcile intervention "+sourceID)
		takeoverEvent := map[string]any{
			"eventId":              eventID(ctx.lane.ID, "executor-takeover-"+ctx.executor, now),
			"kind":                 "executor-takeover",
			"lane":                 ctx.lane.ID,
			"time":                 now,
			"summary":              "executor takeover: " + ctx.executor,
			"previousExecutor":     previousExecutor,
			"currentExecutor":      ctx.executor,
			"executorGeneration":   generation,
			"sourceInterventionId": sourceID,
			"actor":                ctx.actor,
			"reason":               ctx.reason,
		}
		takeoverWrite, err := appendReconcileLaneEvent(ctx.inst.CaseRoot, laneEventPath, "append-executor-takeover", takeoverEvent)
		if err != nil {
			return ReconcileResult{}, err
		}
		writes = append(writes, takeoverWrite)
	} else if ctx.executor != "" && generation == 0 {
		generation = 1
		ctx.lane.CurrentExecutor = ctx.executor
		ctx.lane.ExecutorGeneration = generation
	}
	ctx.lane.LastReconciledIntervention = sourceID
	ctx.lane.LastReconcileAt = now
	ctx.lane.UpdatedAt = now
	laneFile := filepath.Join(laneRoot, "lane.json")
	if err := writeJSON(laneFile, ctx.lane); err != nil {
		return ReconcileResult{}, err
	}
	writes = append(writes, StartWrite{Path: relativePath(ctx.inst.CaseRoot, laneFile), Kind: "lane", Action: "update-reconcile-state", TargetPath: laneFile})
	boardPath, err := saveBoard(ctx.inst.CaseRoot, ctx.manifest)
	if err != nil {
		return ReconcileResult{}, err
	}
	writes = append(writes, StartWrite{Path: ".rekit/board.json", Kind: "board", Action: "refresh", TargetPath: boardPath})
	resumePath, checkpointPath, err := writeLaneResume(ctx.inst.CaseRoot, ctx.manifest, ctx.lane)
	if err != nil {
		return ReconcileResult{}, err
	}
	writes = append(writes,
		StartWrite{Path: relativePath(ctx.inst.CaseRoot, resumePath), Kind: "lane-resume", Action: "refresh", TargetPath: resumePath},
		StartWrite{Path: relativePath(ctx.inst.CaseRoot, checkpointPath), Kind: "lane-checkpoint", Action: "refresh", TargetPath: checkpointPath},
	)
	ctx.facts, err = readHandoffFacts(ctx.inst.CaseRoot)
	if err != nil {
		return ReconcileResult{}, err
	}
	result = ctx.result(true, true, false, writes)
	result.ResolutionEventID = resolutionID
	result.PreviousExecutor = previousExecutor
	result.ExecutorGeneration = generation
	result.Lane = ctx.lane
	result.NextSteps = workstreamNextSteps(result.ExecutorAction, true)
	return result, nil
}

func newReconcileContext(repoRoot, caseRoot, pack string, opt ReconcileOptions, allowResolvedReplay bool) (reconcileContext, error) {
	inst, err := instance.AssertAttached(caseRoot, repoRoot, pack)
	if err != nil {
		return reconcileContext{}, err
	}
	m, err := manifest.Load(repoRoot, pack)
	if err != nil {
		return reconcileContext{}, err
	}
	b, err := readBoard(inst.CaseRoot)
	if os.IsNotExist(err) {
		return reconcileContext{}, fmt.Errorf("reconcile requires existing .rekit/board.json; run start -Apply or /rekit overview once to initialize the case-local board")
	}
	if err != nil {
		return reconcileContext{}, err
	}
	if strings.TrimSpace(b.DefaultAuthorityLane) == "" {
		b.DefaultAuthorityLane = m.WorkstreamDefaults["defaultAuthorityLane"]
	}
	selector := strings.TrimSpace(opt.Selector)
	if selector == "" {
		return reconcileContext{}, fmt.Errorf("reconcile requires a lane selector")
	}
	lane, err := resolveHandoffLane(inst.CaseRoot, b, selector)
	if err != nil {
		return reconcileContext{}, err
	}
	status := strings.ToLower(strings.TrimSpace(lane.Status))
	if status == "archived" || status == "paused" || status == "closed" {
		return reconcileContext{}, fmt.Errorf("target lane is not open: %s", lane.ID)
	}
	facts, err := readHandoffFacts(inst.CaseRoot)
	if err != nil {
		return reconcileContext{}, err
	}
	open := mission.EffectiveOpenLaneInterventions(facts.Facts, lane.ID)
	intervention, err := selectOpenIntervention(open, opt.InterventionID)
	var existingResolution map[string]any
	if err != nil && allowResolvedReplay && strings.TrimSpace(opt.InterventionID) != "" {
		intervention, existingResolution, err = selectResolvedReplayIntervention(facts.Facts.Interventions, lane.ID, opt.InterventionID)
	}
	if err != nil {
		return reconcileContext{}, err
	}
	if strings.TrimSpace(mission.Value(intervention, "eventId")) == "" {
		return reconcileContext{}, fmt.Errorf("selected intervention has no eventId and cannot be reconciled")
	}
	actor := strings.TrimSpace(opt.Actor)
	if actor == "" {
		actor = "main-agent"
	}
	executor := strings.TrimSpace(opt.Executor)
	if executor == "" {
		executor = actor
	}
	reason := strings.TrimSpace(opt.Reason)
	if reason == "" {
		reason = "human-in-the-lane intervention reconciled into durable lane state"
	}
	summary := strings.TrimSpace(opt.Summary)
	if summary == "" {
		summary = "reconciled intervention: " + interventionLabel(intervention)
	}
	return reconcileContext{inst: inst, manifest: m, board: b, selector: selector, lane: lane, facts: facts, intervention: intervention, existingResolution: existingResolution, actor: actor, executor: executor, reason: reason, summary: summary}, nil
}

func selectOpenIntervention(open []map[string]any, requested string) (map[string]any, error) {
	requested = strings.TrimSpace(requested)
	if requested == "" {
		if len(open) == 0 {
			return nil, fmt.Errorf("reconcile found no open intervention for the selected lane")
		}
		if len(open) > 1 {
			ids := []string{}
			for _, item := range open {
				ids = append(ids, mission.Value(item, "eventId"))
			}
			return nil, fmt.Errorf("reconcile requires -InterventionId when lane has multiple open interventions: %s", strings.Join(ids, ","))
		}
		return open[0], nil
	}
	for _, item := range open {
		if mission.Value(item, "eventId") == requested {
			return item, nil
		}
	}
	return nil, fmt.Errorf("intervention %q is not an effective open intervention for the selected lane", requested)
}

func selectResolvedReplayIntervention(items []map[string]any, laneID, requested string) (map[string]any, map[string]any, error) {
	requested = strings.TrimSpace(requested)
	var source map[string]any
	var resolution map[string]any
	for _, item := range items {
		if mission.Value(item, "lane") != laneID {
			continue
		}
		if mission.Value(item, "eventId") == requested {
			source = item
		}
		if mission.Value(item, "resolvesEventId") != requested {
			continue
		}
		status := strings.ToLower(strings.TrimSpace(mission.Value(item, "status")))
		if status != "resolved" {
			continue
		}
		if mission.Value(item, "action") != "reconcile" {
			continue
		}
		if resolution != nil {
			return nil, nil, fmt.Errorf("intervention %q has multiple reconcile resolution events; refusing replay", requested)
		}
		resolution = item
	}
	if source == nil || resolution == nil {
		return nil, nil, fmt.Errorf("intervention %q is not an effective open intervention for the selected lane", requested)
	}
	return source, resolution, nil
}

func (ctx reconcileContext) reconcileResolution(sourceID, resolutionID, now string) map[string]any {
	resolution := map[string]any{
		"schemaVersion":   1,
		"eventId":         resolutionID,
		"kind":            "intervention",
		"lane":            ctx.lane.ID,
		"subject":         firstText(ctx.summary, "reconcile: "+interventionLabel(ctx.intervention)),
		"summary":         ctx.summary,
		"action":          "reconcile",
		"status":          "resolved",
		"resolvesEventId": sourceID,
		"target":          mission.Value(ctx.intervention, "target"),
		"actor":           ctx.actor,
		"executor":        ctx.executor,
		"reason":          ctx.reason,
		"time":            now,
	}
	if strings.TrimSpace(ctx.summary) == "" {
		resolution["summary"] = "reconciled intervention: " + interventionLabel(ctx.intervention)
	}
	return resolution
}

func appendReconcileLaneEvent(caseRoot, path, action string, event map[string]any) (StartWrite, error) {
	rel := relativePath(caseRoot, path)
	eventID := mission.Value(event, "eventId")
	existing, err := mission.ReadJSONLineObjects(path)
	if err != nil {
		return StartWrite{}, err
	}
	for _, item := range existing {
		if mission.Value(item, "eventId") != eventID {
			continue
		}
		for _, key := range []string{"kind", "lane", "sourceEventId", "resolvesEventId", "resolutionEventId", "sourceInterventionId", "currentExecutor", "executorGeneration", "actor", "reason"} {
			if strings.TrimSpace(mission.Value(event, key)) != "" && mission.Value(item, key) != mission.Value(event, key) {
				return StartWrite{}, fmt.Errorf("reconcile lane event %s differs on %s; refusing replay", eventID, key)
			}
		}
		return StartWrite{Path: rel, Kind: "lane-event", Action: "already-appended", TargetPath: path}, nil
	}
	if err := mission.AppendJSONLine(path, event); err != nil {
		return StartWrite{}, err
	}
	return StartWrite{Path: rel, Kind: "lane-event", Action: action, TargetPath: path}, nil
}

func (ctx reconcileContext) result(mutating, applied, confirm bool, writes []StartWrite) ReconcileResult {
	brief := projectMissionBrief(ctx.board.Lanes, ctx.facts)
	laneBrief := laneMissionBrief(ctx.lane, ctx.facts)
	laneFacts := mission.LaneFacts(ctx.facts.Facts, ctx.lane.ID)
	executorAction := laneExecutorActionFor(ctx.lane, laneFacts, laneBrief)
	if !applied && executorAction.MissionCommanderAction.State == "needs-reconcile" {
		continuationLane := ctx.lane
		if ctx.executor != "" && !strings.EqualFold(strings.TrimSpace(continuationLane.CurrentExecutor), ctx.executor) {
			continuationLane.CurrentExecutor = ctx.executor
			continuationLane.ExecutorGeneration = max(continuationLane.ExecutorGeneration, 0) + 1
		} else if ctx.executor != "" && continuationLane.ExecutorGeneration == 0 {
			continuationLane.CurrentExecutor = ctx.executor
			continuationLane.ExecutorGeneration = 1
		}
		executorAction = bindLaneContinueCommands(executorAction, continuationLane)
		executorAction.MissionCommanderAction = ctx.reconcileApplyCommanderAction()
	}
	commanderAction := executorAction.MissionCommanderAction
	authorizedGateAdapterHandoffs := AuthorizedGateAdapterHandoffsWithAcknowledgements(ctx.manifest.RepoRoot, ctx.inst.CaseRoot, ctx.manifest.Pack, ctx.facts.Requests, ctx.lane.ID, ExecutionEvidenceReviewAcknowledgedIDs(ctx.facts))
	reviewerDispatchIntakeHandoffs, _ := ReviewerDispatchIntakeHandoffs(ctx.inst.CaseRoot, ctx.facts, ctx.lane.ID)
	reviewerPacketRetirementHandoffs, _ := ReviewerPacketRetirementHandoffs(ctx.inst.CaseRoot, ctx.lane.ID)
	pendingGateHandoffs, openDecisionHandoffs := gateDecisionHandoffs(ctx.lane, laneFacts)
	commanderNextActions := reconcileMissionCommanderNextActions(ctx.lane, executorAction, applied)
	commanderNextActions = MissionCommanderNextActionsWithAuthorizedGateAdapters(commanderNextActions, authorizedGateAdapterHandoffs)
	commanderNextActions = MissionCommanderNextActionsWithReviewerDispatches(commanderNextActions, reviewerDispatchIntakeHandoffs)
	commanderActionQueue := reconcileActionQueueWithRefresh(mission.MissionCommanderActionQueueFor(commanderNextActions), ctx.inst.CaseRoot)
	if applied {
		executorAction = withReviewerDispatchBlocker(executorAction, reviewerDispatchIntakeHandoffs)
		commanderAction = executorAction.MissionCommanderAction
	}
	result := ReconcileResult{
		SchemaVersion:                    1,
		Command:                          "reconcile",
		CaseRoot:                         ctx.inst.CaseRoot,
		RepoRoot:                         ctx.manifest.RepoRoot,
		Pack:                             ctx.manifest.Pack,
		IsMutation:                       mutating,
		Applied:                          applied,
		RequiresConfirmation:             confirm,
		Selector:                         ctx.selector,
		Lane:                             ctx.lane,
		Intervention:                     summarizeIntervention(ctx.intervention),
		Actor:                            ctx.actor,
		Executor:                         ctx.executor,
		PreviousExecutor:                 ctx.lane.CurrentExecutor,
		ExecutorGeneration:               ctx.lane.ExecutorGeneration,
		MissionBrief:                     brief,
		AuthorizedGateAdapterHandoffs:    authorizedGateAdapterHandoffs,
		ReviewerDispatchIntakeHandoffs:   reviewerDispatchIntakeHandoffs,
		ReviewerDispatchIntakeSummary:    ReviewerDispatchIntakeSummaryFor(reviewerDispatchIntakeHandoffs),
		ReviewerPacketRetirementHandoffs: reviewerPacketRetirementHandoffs,
		ReviewerPacketRetirementSummary:  ReviewerPacketRetirementSummaryFor(reviewerPacketRetirementHandoffs),
		PendingGateHandoffs:              pendingGateHandoffs,
		OpenDecisionHandoffs:             openDecisionHandoffs,
		ExecutorAction:                   executorAction,
		MissionCommanderAction:           commanderAction,
		MissionCommanderNextActions:      commanderNextActions,
		MissionCommanderActionQueue:      commanderActionQueue,
		BlockedActions:                   []string{"authority/confirmed writes", "heavy-tool execution", "external side effects"},
		NextSteps: []string{
			"review this reconcile plan, then re-run reconcile with -Apply to write case-local ledger and lane state",
			"reconcile never executes heavy-tool and never writes authority/confirmed state",
		},
	}
	result.MissionCommanderDriverReceipt = reconcileMissionCommanderDriverReceipt(result)
	result.ReplacementExecutorTakeoverPackage = reconcileReplacementExecutorTakeoverPackage(result)
	if applied {
		result.Writes = writes
	} else {
		result.WouldWrites = writes
	}
	return result
}

func reconcileActionQueueWithRefresh(queue mission.MissionCommanderActionQueue, caseRoot string) mission.MissionCommanderActionQueue {
	if queue.CurrentDriverRequest == nil {
		return queue
	}
	refreshed := mission.MissionCommanderDriverRequestWithRefreshStatusCommand(*queue.CurrentDriverRequest, dailyMissionControlStatusCommand(caseRoot))
	queue.CurrentDriverRequest = &refreshed
	return queue
}

func reconcileMissionCommanderDriverReceipt(result ReconcileResult) *MissionCommanderDriverReceipt {
	request := result.MissionCommanderActionQueue.CurrentDriverRequest
	command := result.Command
	if request != nil && strings.TrimSpace(request.Command) != "" {
		command = request.Command
	}
	receipt := &MissionCommanderDriverReceipt{
		SchemaVersion:                 1,
		State:                         "refreshed",
		Outcome:                       "explicit-command-result",
		Lane:                          result.Lane.ID,
		Command:                       command,
		RefreshedActionQueueSummary:   result.MissionCommanderActionQueue.Summary,
		RefreshedCurrentRunLoopStep:   result.MissionCommanderActionQueue.CurrentRunLoopStepID,
		RefreshedCurrentDriverRequest: request,
		Boundary: []string{
			"driver receipt records an explicit main-agent/harness reconcile result after durable state refresh",
			"driver receipt does not prove the Go runtime spawned, polled, stopped, or managed an external session",
			"reconcile does not write authority/confirmed state or execute heavy tools",
		},
	}
	if result.Applied {
		receipt.RunID = "reconcile-" + firstText(result.ResolutionEventID, result.Intervention.EventID)
		receipt.BatchID = "batch-" + receipt.RunID
	}
	return receipt
}

func reconcileReplacementExecutorTakeoverPackage(result ReconcileResult) *mission.ReplacementExecutorTakeoverPackage {
	return mission.ReplacementExecutorTakeoverPackageFor(result.MissionCommanderActionQueue.CurrentDriverRequest, mission.ReplacementExecutorTakeoverOptions{
		Focus:                "reconcile-current-action",
		Scope:                "lane:" + workstreamLabel(result.Lane),
		RefreshStatusCommand: dailyMissionControlStatusCommand(result.CaseRoot),
		PackagePath:          "replacementExecutorTakeoverPackage",
		TargetDocuments: []string{
			"missionCommanderActionQueue.currentDriverRequest",
			"missionCommanderDriverReceipt",
			relJoin(result.Lane.LaneRoot, "prompts", "RESUME.md"),
			relJoin(result.Lane.LaneRoot, "checkpoints", "latest.json"),
			relJoin(".rekit", "handovers", result.Lane.ID+"-latest.md"),
		},
	})
}

func (ctx reconcileContext) reconcileApplyCommanderAction() mission.MissionCommanderAction {
	label := workstreamLabel(ctx.lane)
	continuationLane := ctx.lane
	if ctx.executor != "" && !strings.EqualFold(strings.TrimSpace(continuationLane.CurrentExecutor), ctx.executor) {
		continuationLane.CurrentExecutor = ctx.executor
		continuationLane.ExecutorGeneration = max(continuationLane.ExecutorGeneration, 0) + 1
	} else if ctx.executor != "" && continuationLane.ExecutorGeneration == 0 {
		continuationLane.CurrentExecutor = ctx.executor
		continuationLane.ExecutorGeneration = 1
	}
	command := "/rekit reconcile " + label + " -InterventionId " + quoteCommandArg(mission.Value(ctx.intervention, "eventId")) + " -Apply"
	if strings.TrimSpace(ctx.executor) != "" {
		command += " -Executor " + quoteCommandArg(ctx.executor)
	}
	if strings.TrimSpace(ctx.actor) != "" {
		command += " -Actor " + quoteCommandArg(ctx.actor)
	}
	if strings.TrimSpace(ctx.reason) != "" {
		command += " -Reason " + quoteCommandArg(ctx.reason)
	}
	return mission.MissionCommanderAction{
		State:          "needs-reconcile",
		Prompt:         "先 review reconcile preview，再写入 selected open intervention 的 resolution 与 lane state refresh。",
		PrimaryCommand: command,
		FollowUpCommands: []string{
			bindContinueCommand("/rekit continue "+label+" -WhatIf", continuationLane),
			"/rekit handoff " + label,
		},
		Boundary: []string{
			"no authority/confirmed writes",
			"no heavy-tool execution",
			"reconcile apply only writes case-local intervention/lane/resume/checkpoint state",
		},
	}
}

func reconcileMissionCommanderNextActions(lane Lane, action laneExecutorAction, applied bool) []mission.MissionCommanderNextActionItem {
	items := mission.MissionCommanderNextActions([]mission.LaneExecutorActionSnapshot{laneCommanderActionSnapshot(lane, action)}, nil, action.Blocked)
	if applied || action.MissionCommanderAction.State != "needs-reconcile" {
		return items
	}
	for idx := range items {
		items[idx].RequiresReview = true
		items[idx].Reasons = append(items[idx].Reasons, "review reconcile preview before writing case-local ledger/lane/resume/checkpoint state")
		if items[idx].Source == "missionCommanderActions" {
			items[idx].Blocked = false
			items[idx].Reasons = append(items[idx].Reasons, "reconcile apply is the bounded action that resolves the selected open intervention")
		}
		if items[idx].Source == "missionCommanderActions.followUp" {
			items[idx].Blocked = true
			items[idx].Reasons = append(items[idx].Reasons, "run only after reconcile apply succeeds and the refreshed executor action remains ready")
		}
	}
	return items
}

func (ctx reconcileContext) plannedWrites() []StartWrite {
	laneRootRel := relJoin(".rekit", "lanes", ctx.lane.ID)
	laneEventsPath := relJoin(laneRootRel, "events.jsonl")
	writes := []StartWrite{
		{Path: mission.FactRelPath("intervention"), Kind: "fact-jsonl", Action: "would-append"},
		{Path: laneEventsPath, Kind: "lane-event", Action: "would-append-intervention-reconciled"},
	}
	if ctx.executor != "" && !strings.EqualFold(strings.TrimSpace(ctx.lane.CurrentExecutor), ctx.executor) {
		writes = append(writes, StartWrite{Path: laneEventsPath, Kind: "lane-event", Action: "would-append-executor-takeover"})
	}
	writes = append(writes,
		StartWrite{Path: relJoin(laneRootRel, "lane.json"), Kind: "lane", Action: "would-update-reconcile-state"},
		StartWrite{Path: relJoin(laneRootRel, "prompts", "RESUME.md"), Kind: "lane-resume", Action: "would-refresh"},
		StartWrite{Path: relJoin(laneRootRel, "checkpoints", "latest.json"), Kind: "lane-checkpoint", Action: "would-refresh"},
		StartWrite{Path: ".rekit/board.json", Kind: "board", Action: "would-refresh"},
	)
	return writes
}

func openLaneInterventionSummaries(caseRoot, laneID string) ([]InterventionSummary, error) {
	facts, err := readHandoffFacts(caseRoot)
	if err != nil {
		return nil, err
	}
	return interventionSummaries(mission.EffectiveOpenLaneInterventions(facts.Facts, laneID)), nil
}

func interventionSummaries(items []map[string]any) []InterventionSummary {
	out := []InterventionSummary{}
	for _, item := range items {
		out = append(out, summarizeIntervention(item))
	}
	return out
}

func summarizeIntervention(item map[string]any) InterventionSummary {
	return InterventionSummary{
		EventID:    mission.Value(item, "eventId"),
		Lane:       mission.Value(item, "lane"),
		Subject:    mission.Value(item, "subject"),
		Summary:    mission.Value(item, "summary"),
		Action:     mission.Value(item, "action"),
		Status:     firstText(mission.Value(item, "status"), "open"),
		Target:     mission.Value(item, "target"),
		Scope:      mission.Value(item, "scope"),
		ApprovedBy: mission.Value(item, "approvedBy"),
		BatchID:    mission.Value(item, "batchId"),
	}
}

func interventionLabel(item map[string]any) string {
	return firstText(mission.Value(item, "subject"), mission.Value(item, "summary"), mission.Value(item, "eventId"), "intervention")
}
