package workstream

import (
	"crypto/sha256"
	"encoding/csv"
	"encoding/hex"
	"encoding/json"
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
	Selector string
}

type ContinueResult struct {
	SchemaVersion        int                    `json:"schemaVersion"`
	Command              string                 `json:"command"`
	CaseRoot             string                 `json:"caseRoot"`
	RepoRoot             string                 `json:"repoRoot"`
	Pack                 string                 `json:"pack"`
	IsMutation           bool                   `json:"isMutation"`
	Applied              bool                   `json:"applied"`
	RequiresConfirmation bool                   `json:"requiresConfirmation"`
	Selector             string                 `json:"selector"`
	Lane                 Lane                   `json:"lane"`
	AutonomyProfile      autonomy.Summary       `json:"autonomyProfile"`
	RunID                string                 `json:"runId"`
	BatchID              string                 `json:"batchId"`
	Summary              ContinueSummary        `json:"summary"`
	MissionBrief         mission.Brief          `json:"missionBrief"`
	ExecutorAction       laneExecutorAction     `json:"executorAction"`
	Inputs               []string               `json:"inputs"`
	PacketRefs           []string               `json:"packetRefs"`
	Events               []ContinueEventPreview `json:"events"`
	OpenRisks            []string               `json:"openRisks"`
	Blocked              bool                   `json:"blocked"`
	ReconcileRequired    bool                   `json:"reconcileRequired"`
	OpenInterventions    []InterventionSummary  `json:"openInterventions,omitempty"`
	WouldWrites          []StartWrite           `json:"wouldWrites"`
	Writes               []StartWrite           `json:"writes,omitempty"`
	BlockedActions       []string               `json:"blockedActions"`
	NextSteps            []string               `json:"nextSteps"`
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
	result := ContinueResult{
		SchemaVersion:        1,
		Command:              "continue",
		CaseRoot:             ctx.inst.CaseRoot,
		RepoRoot:             ctx.manifest.RepoRoot,
		Pack:                 ctx.manifest.Pack,
		IsMutation:           false,
		Applied:              false,
		RequiresConfirmation: true,
		Selector:             ctx.selector,
		Lane:                 ctx.lane,
		AutonomyProfile:      autonomy.ReadSummary(ctx.inst.CaseRoot, ctx.lane.ID, ctx.manifest),
		RunID:                continuePreviewRunID,
		BatchID:              "batch-" + continuePreviewRunID,
		MissionBrief:         ctx.missionBrief(),
		ExecutorAction:       ctx.executorAction(),
		Inputs:               uniqueStrings(inputs),
		PacketRefs:           uniqueStrings(packets),
		BlockedActions:       []string{"run directory creation", "facts JSONL writes", "lane resume/checkpoint refresh", "board refresh", "authority/confirmed writes", "heavy-tool execution without a valid current authorization decision"},
		NextSteps: []string{
			"review this preview, then re-run continue with -Apply when the case-local facts/route/digest writes are acceptable",
			"PowerShell /rekit remains the public entrypoint; JSON preview and explicit apply are Go-owned by default",
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

func ContinueApply(repoRoot, caseRoot, pack string, opt ContinueOptions) (ContinueResult, error) {
	ctx, err := newContinueContext(repoRoot, caseRoot, pack, opt)
	if err != nil {
		return ContinueResult{}, err
	}
	if blocked, err := ctx.blockedByOpenInterventions(true); err != nil || blocked.Blocked {
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
	result := ContinueResult{
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
	policy, err := readContinuePolicy(inst.CaseRoot)
	if err != nil {
		return continueContext{}, err
	}
	return continueContext{inst: inst, manifest: m, board: b, policy: policy, selector: selector, lane: lane}, nil
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

func (ctx continueContext) blockedByOpenInterventions(apply bool) (ContinueResult, error) {
	open, err := openLaneInterventionSummaries(ctx.inst.CaseRoot, ctx.lane.ID)
	if err != nil {
		return ContinueResult{}, err
	}
	if len(open) == 0 {
		return ContinueResult{}, nil
	}
	executorAction := ctx.executorAction()
	return ContinueResult{
		SchemaVersion:        1,
		Command:              "continue",
		CaseRoot:             ctx.inst.CaseRoot,
		RepoRoot:             ctx.manifest.RepoRoot,
		Pack:                 ctx.manifest.Pack,
		IsMutation:           apply,
		Applied:              false,
		RequiresConfirmation: false,
		Selector:             ctx.selector,
		Lane:                 ctx.lane,
		AutonomyProfile:      autonomy.ReadSummary(ctx.inst.CaseRoot, ctx.lane.ID, ctx.manifest),
		RunID:                continuePreviewRunID,
		BatchID:              "batch-" + continuePreviewRunID,
		MissionBrief:         ctx.missionBrief(),
		ExecutorAction:       executorAction,
		OpenRisks:            interventionRiskLines(open),
		Blocked:              true,
		ReconcileRequired:    true,
		OpenInterventions:    open,
		WouldWrites:          []StartWrite{},
		Writes:               []StartWrite{},
		BlockedActions:       []string{"run directory creation", "facts JSONL writes", "lane resume/checkpoint refresh", "board refresh", "authority/confirmed writes", "heavy-tool execution without a valid current authorization decision"},
		NextSteps:            executorAction.NextAgentActions,
	}, nil
}

func interventionRiskLines(items []InterventionSummary) []string {
	lines := []string{}
	for _, item := range items {
		lines = append(lines, fmt.Sprintf("intervention: %s | lane=%s | status=%s", firstText(item.Subject, item.Summary, item.EventID), item.Lane, item.Status))
	}
	return lines
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
	status := map[string]any{"schemaVersion": 1, "runId": result.RunID, "batchId": result.BatchID, "summary": result.Summary, "autonomyProfile": result.AutonomyProfile, "missionBrief": result.MissionBrief, "executorAction": result.ExecutorAction, "inputs": result.Inputs, "packetRefs": result.PacketRefs, "openRisks": result.OpenRisks, "time": isoNow()}
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
	)
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
