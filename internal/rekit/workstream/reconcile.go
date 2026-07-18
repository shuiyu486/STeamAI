package workstream

import (
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
	EventID string `json:"eventId"`
	Lane    string `json:"lane"`
	Subject string `json:"subject,omitempty"`
	Summary string `json:"summary,omitempty"`
	Action  string `json:"action,omitempty"`
	Status  string `json:"status,omitempty"`
	Target  string `json:"target,omitempty"`
}

type ReconcileResult struct {
	SchemaVersion        int                 `json:"schemaVersion"`
	Command              string              `json:"command"`
	CaseRoot             string              `json:"caseRoot"`
	RepoRoot             string              `json:"repoRoot"`
	Pack                 string              `json:"pack"`
	IsMutation           bool                `json:"isMutation"`
	Applied              bool                `json:"applied"`
	RequiresConfirmation bool                `json:"requiresConfirmation"`
	Selector             string              `json:"selector"`
	Lane                 Lane                `json:"lane"`
	Intervention         InterventionSummary `json:"intervention"`
	ResolutionEventID    string              `json:"resolutionEventId,omitempty"`
	Actor                string              `json:"actor"`
	Executor             string              `json:"executor"`
	PreviousExecutor     string              `json:"previousExecutor,omitempty"`
	ExecutorGeneration   int                 `json:"executorGeneration"`
	MissionBrief         mission.Brief       `json:"missionBrief"`
	ExecutorAction       laneExecutorAction  `json:"executorAction"`
	WouldWrites          []StartWrite        `json:"wouldWrites,omitempty"`
	Writes               []StartWrite        `json:"writes,omitempty"`
	BlockedActions       []string            `json:"blockedActions"`
	NextSteps            []string            `json:"nextSteps"`
}

type reconcileContext struct {
	inst         instance.Instance
	manifest     *manifest.Manifest
	board        board
	selector     string
	lane         Lane
	facts        mission.LedgerFacts
	intervention map[string]any
	actor        string
	executor     string
	reason       string
	summary      string
}

func ReconcilePreview(repoRoot, caseRoot, pack string, opt ReconcileOptions) (ReconcileResult, error) {
	ctx, err := newReconcileContext(repoRoot, caseRoot, pack, opt)
	if err != nil {
		return ReconcileResult{}, err
	}
	return ctx.result(false, false, true, ctx.plannedWrites()), nil
}

func ReconcileApply(repoRoot, caseRoot, pack string, opt ReconcileOptions) (ReconcileResult, error) {
	ctx, err := newReconcileContext(repoRoot, caseRoot, pack, opt)
	if err != nil {
		return ReconcileResult{}, err
	}
	now := isoNow()
	writes := []StartWrite{}
	laneRoot, err := laneRootPath(ctx.inst.CaseRoot, ctx.lane)
	if err != nil {
		return ReconcileResult{}, err
	}
	sourceID := mission.Value(ctx.intervention, "eventId")
	resolutionID := eventID(ctx.lane.ID, "intervention-resolved-"+sourceID, now)
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
	rel, path, err := mission.AppendFact(ctx.inst.CaseRoot, "intervention", resolution)
	if err != nil {
		return ReconcileResult{}, err
	}
	writes = append(writes, StartWrite{Path: rel, Kind: "fact-jsonl", Action: "append", TargetPath: path})

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
	if err := mission.AppendJSONLine(laneEventPath, laneEvent); err != nil {
		return ReconcileResult{}, err
	}
	writes = append(writes, StartWrite{Path: relativePath(ctx.inst.CaseRoot, laneEventPath), Kind: "lane-event", Action: "append-intervention-reconciled", TargetPath: laneEventPath})

	previousExecutor := strings.TrimSpace(ctx.lane.CurrentExecutor)
	generation := max(ctx.lane.ExecutorGeneration, 0)
	if ctx.executor != "" && !strings.EqualFold(previousExecutor, ctx.executor) {
		generation++
		ctx.lane.CurrentExecutor = ctx.executor
		ctx.lane.ExecutorGeneration = generation
		ctx.lane.LastTakeoverAt = now
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
		if err := mission.AppendJSONLine(laneEventPath, takeoverEvent); err != nil {
			return ReconcileResult{}, err
		}
		writes = append(writes, StartWrite{Path: relativePath(ctx.inst.CaseRoot, laneEventPath), Kind: "lane-event", Action: "append-executor-takeover", TargetPath: laneEventPath})
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
	resumePath, checkpointPath, err := writeLaneResume(ctx.inst.CaseRoot, ctx.manifest, ctx.lane)
	if err != nil {
		return ReconcileResult{}, err
	}
	writes = append(writes,
		StartWrite{Path: relativePath(ctx.inst.CaseRoot, resumePath), Kind: "lane-resume", Action: "refresh", TargetPath: resumePath},
		StartWrite{Path: relativePath(ctx.inst.CaseRoot, checkpointPath), Kind: "lane-checkpoint", Action: "refresh", TargetPath: checkpointPath},
	)
	boardPath, err := saveBoard(ctx.inst.CaseRoot, ctx.manifest)
	if err != nil {
		return ReconcileResult{}, err
	}
	writes = append(writes, StartWrite{Path: ".rekit/board.json", Kind: "board", Action: "refresh", TargetPath: boardPath})
	ctx.facts, err = readHandoffFacts(ctx.inst.CaseRoot)
	if err != nil {
		return ReconcileResult{}, err
	}
	result := ctx.result(true, true, false, writes)
	result.ResolutionEventID = resolutionID
	result.PreviousExecutor = previousExecutor
	result.ExecutorGeneration = generation
	result.Lane = ctx.lane
	result.NextSteps = []string{
		"run doctor after apply",
		"use /rekit continue " + workstreamLabel(ctx.lane) + " to resume the reconciled lane",
	}
	return result, nil
}

func newReconcileContext(repoRoot, caseRoot, pack string, opt ReconcileOptions) (reconcileContext, error) {
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
	return reconcileContext{inst: inst, manifest: m, board: b, selector: selector, lane: lane, facts: facts, intervention: intervention, actor: actor, executor: executor, reason: reason, summary: summary}, nil
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

func (ctx reconcileContext) result(mutating, applied, confirm bool, writes []StartWrite) ReconcileResult {
	brief := projectMissionBrief(ctx.board.Lanes, ctx.facts)
	laneBrief := laneMissionBrief(ctx.lane, ctx.facts)
	laneFacts := mission.LaneFacts(ctx.facts.Facts, ctx.lane.ID)
	result := ReconcileResult{
		SchemaVersion:        1,
		Command:              "reconcile",
		CaseRoot:             ctx.inst.CaseRoot,
		RepoRoot:             ctx.manifest.RepoRoot,
		Pack:                 ctx.manifest.Pack,
		IsMutation:           mutating,
		Applied:              applied,
		RequiresConfirmation: confirm,
		Selector:             ctx.selector,
		Lane:                 ctx.lane,
		Intervention:         summarizeIntervention(ctx.intervention),
		Actor:                ctx.actor,
		Executor:             ctx.executor,
		PreviousExecutor:     ctx.lane.CurrentExecutor,
		ExecutorGeneration:   ctx.lane.ExecutorGeneration,
		MissionBrief:         brief,
		ExecutorAction:       laneExecutorActionFor(ctx.lane, laneFacts, laneBrief),
		BlockedActions:       []string{"authority/confirmed writes", "heavy-tool execution", "external side effects"},
		NextSteps: []string{
			"review this reconcile plan, then re-run reconcile with -Apply to write case-local ledger and lane state",
			"reconcile never executes heavy-tool and never writes authority/confirmed state",
		},
	}
	if applied {
		result.Writes = writes
	} else {
		result.WouldWrites = writes
	}
	return result
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
		EventID: mission.Value(item, "eventId"),
		Lane:    mission.Value(item, "lane"),
		Subject: mission.Value(item, "subject"),
		Summary: mission.Value(item, "summary"),
		Action:  mission.Value(item, "action"),
		Status:  firstText(mission.Value(item, "status"), "open"),
		Target:  mission.Value(item, "target"),
	}
}

func interventionLabel(item map[string]any) string {
	return firstText(mission.Value(item, "subject"), mission.Value(item, "summary"), mission.Value(item, "eventId"), "intervention")
}
