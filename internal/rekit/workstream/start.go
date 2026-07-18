package workstream

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"slices"
	"sort"
	"strings"
	"time"

	"github.com/shuiyu486/re-context-kits/internal/rekit/autonomy"
	refsf "github.com/shuiyu486/re-context-kits/internal/rekit/fs"
	"github.com/shuiyu486/re-context-kits/internal/rekit/instance"
	"github.com/shuiyu486/re-context-kits/internal/rekit/manifest"
	"github.com/shuiyu486/re-context-kits/internal/rekit/mission"
)

const defaultPolicyText = `schemaVersion: 1
automationMode: assisted-autopilot
autoCollect: true
autoVerify: true
autoRouteRequests: true
autoSyncLanes: true
autoPublishSharedFacts: true
autoAcceptLowRiskCandidates: true
authorityAutoAppend: conditional
authorityAutoOverwrite: never
authorityAutoDelete: never
requireEvidence: true
requireVerifier: true
minConfidence: 0.90
requireNoConflict: true
requireSchemaValid: true
requireBackup: true
requireDiff: true
maxAuthorityRowsPerRun: 10
askUserWhen: conflict,overwriteAuthority,deleteAuthority,confidenceBelowThreshold,schemaChange,changesProjectBaseline,externalSideEffect,destructiveAction
`

var safeLaneIDSegment = regexp.MustCompile(`^[a-z0-9](?:[a-z0-9._-]*[a-z0-9])?$`)

type StartOptions struct {
	Name  string
	Force bool
}

type StartWrite struct {
	Path       string `json:"path"`
	Kind       string `json:"kind"`
	Action     string `json:"action"`
	TargetPath string `json:"targetPath,omitempty"`
	Reason     string `json:"reason,omitempty"`
}

type StartResult struct {
	SchemaVersion        int              `json:"schemaVersion"`
	Command              string           `json:"command"`
	CaseRoot             string           `json:"caseRoot"`
	RepoRoot             string           `json:"repoRoot"`
	Pack                 string           `json:"pack"`
	IsMutation           bool             `json:"isMutation"`
	Applied              bool             `json:"applied"`
	RequiresConfirmation bool             `json:"requiresConfirmation"`
	Lane                 Lane             `json:"lane"`
	AutonomyProfile      autonomy.Summary `json:"autonomyProfile"`
	MissionBrief         mission.Brief    `json:"missionBrief"`
	Writes               []StartWrite     `json:"writes"`
	BlockedActions       []string         `json:"blockedActions"`
	NextSteps            []string         `json:"nextSteps"`
}

type Lane struct {
	SchemaVersion              int            `json:"schemaVersion"`
	ID                         string         `json:"id"`
	Type                       string         `json:"type"`
	Name                       string         `json:"name"`
	Title                      string         `json:"title"`
	Status                     string         `json:"status"`
	Authority                  bool           `json:"authority"`
	Workspace                  string         `json:"workspace"`
	LaneRoot                   string         `json:"laneRoot"`
	CanWrite                   []string       `json:"canWrite"`
	ReadOnly                   []string       `json:"readOnly"`
	Outputs                    []string       `json:"outputs"`
	Counters                   map[string]int `json:"counters"`
	CurrentExecutor            string         `json:"currentExecutor,omitempty"`
	ExecutorGeneration         int            `json:"executorGeneration,omitempty"`
	LastTakeoverAt             string         `json:"lastTakeoverAt,omitempty"`
	LastTakeoverReason         string         `json:"lastTakeoverReason,omitempty"`
	LastReconciledIntervention string         `json:"lastReconciledIntervention,omitempty"`
	LastReconcileAt            string         `json:"lastReconcileAt,omitempty"`
	CreatedAt                  string         `json:"createdAt"`
	UpdatedAt                  string         `json:"updatedAt"`
}

type laneExecutorAction struct {
	Blocked              bool     `json:"blocked"`
	Ready                bool     `json:"ready"`
	BlockerReasons       []string `json:"blockerReasons"`
	PendingGates         int      `json:"pendingGates"`
	OpenInterventions    int      `json:"openInterventions"`
	OpenDecisions        int      `json:"openDecisions"`
	ReconcileRequired    bool     `json:"reconcileRequired"`
	PendingGateRequired  bool     `json:"pendingGateRequired"`
	OpenDecisionRequired bool     `json:"openDecisionRequired"`
	ResumeCommand        string   `json:"resumeCommand"`
	HandoffCommand       string   `json:"handoffCommand"`
	NextAgentActions     []string `json:"nextAgentActions"`
	Escalations          []string `json:"escalations"`
}

type laneCheckpoint struct {
	SchemaVersion              int                   `json:"schemaVersion"`
	Lane                       string                `json:"lane"`
	Status                     string                `json:"status"`
	Workspace                  string                `json:"workspace"`
	CurrentExecutor            string                `json:"currentExecutor"`
	ExecutorGeneration         int                   `json:"executorGeneration"`
	LastReconciledIntervention string                `json:"lastReconciledIntervention"`
	LastReconcileAt            string                `json:"lastReconcileAt"`
	AutonomyProfile            autonomy.Summary      `json:"autonomyProfile"`
	MissionBrief               mission.Brief         `json:"missionBrief"`
	ExecutorAction             laneExecutorAction    `json:"executorAction"`
	PendingGates               []string              `json:"pendingGates"`
	AuthorizedGates            []string              `json:"authorizedGates"`
	OpenInterventions          []InterventionSummary `json:"openInterventions"`
	Inbox                      int                   `json:"inbox"`
	Tasks                      int                   `json:"tasks"`
	UpdatedAt                  string                `json:"updatedAt"`
	Resume                     string                `json:"resume"`
}

type board = mission.Board

type boardLane = mission.BoardLane

func StartPreview(repoRoot, caseRoot, pack string, opt StartOptions) (StartResult, error) {
	inst, m, laneType, laneID, name, err := startContext(repoRoot, caseRoot, pack, opt)
	if err != nil {
		return StartResult{}, err
	}
	lane, err := plannedLane(inst.CaseRoot, laneType, laneID, name, "")
	if err != nil {
		return StartResult{}, err
	}
	brief := startMissionBrief(inst.CaseRoot)
	laneFile, err := refsf.SafeJoin(inst.CaseRoot, relJoin(".rekit", "lanes", laneID, "lane.json"))
	if err != nil {
		return StartResult{}, err
	}
	action := "would-create-lane"
	if refsf.Exists(laneFile) {
		action = "would-enter-existing-lane"
		if opt.Force {
			action = "would-refresh-lane-with-force"
		}
	}
	return StartResult{
		SchemaVersion:        1,
		Command:              "start",
		CaseRoot:             inst.CaseRoot,
		RepoRoot:             m.RepoRoot,
		Pack:                 m.Pack,
		IsMutation:           false,
		Applied:              false,
		RequiresConfirmation: true,
		Lane:                 lane,
		MissionBrief:         brief,
		Writes: []StartWrite{{
			Path:       relJoin(".rekit", "lanes", laneID, "lane.json"),
			Kind:       "lane",
			Action:     action,
			TargetPath: laneFile,
		}},
		AutonomyProfile: autonomy.ReadSummary(inst.CaseRoot, laneID, m),
		BlockedActions:  []string{"authority/confirmed writes", "heavy-tool execution without a valid current authorization decision", "handoff writes", "continue auto-apply"},
		NextSteps: []string{
			"review this plan, then re-run start with -Apply to create or enter the workstream",
			"PowerShell /rekit remains the public entrypoint; JSON preview/apply is Go-owned by default",
		},
	}, nil
}

func StartApply(repoRoot, caseRoot, pack string, opt StartOptions) (StartResult, error) {
	inst, m, laneType, laneID, name, err := startContext(repoRoot, caseRoot, pack, opt)
	if err != nil {
		return StartResult{}, err
	}
	writes := []StartWrite{}
	if err := ensureWorkstreamState(inst.CaseRoot, m, &writes); err != nil {
		return StartResult{}, err
	}
	lane, laneWrites, err := writeLane(inst.CaseRoot, m, laneType, laneID, name, opt.Force)
	if err != nil {
		return StartResult{}, err
	}
	writes = append(writes, laneWrites...)
	boardPath, err := saveBoard(inst.CaseRoot, m)
	if err != nil {
		return StartResult{}, err
	}
	writes = append(writes, StartWrite{Path: ".rekit/board.json", Kind: "board", Action: "refresh", TargetPath: boardPath})
	brief := startMissionBrief(inst.CaseRoot)
	return StartResult{
		SchemaVersion:        1,
		Command:              "start",
		CaseRoot:             inst.CaseRoot,
		RepoRoot:             m.RepoRoot,
		Pack:                 m.Pack,
		IsMutation:           true,
		Applied:              true,
		RequiresConfirmation: false,
		Lane:                 lane,
		AutonomyProfile:      autonomy.ReadSummary(inst.CaseRoot, lane.ID, m),
		MissionBrief:         brief,
		Writes:               writes,
		BlockedActions:       []string{"authority/confirmed writes", "heavy-tool execution without a valid current authorization decision", "handoff writes", "continue auto-apply"},
		NextSteps: []string{
			"run doctor after apply",
			"use /rekit continue " + workstreamLabel(lane) + " to enter the lane workflow",
		},
	}, nil
}

func startMissionBrief(caseRoot string) mission.Brief {
	brief, err := mission.CaseBrief(caseRoot, mission.BuildOptions{
		MaxRows:            maxHandoffRows,
		OpenDecisionAction: "review open candidate/decision item(s) with evidence and authority boundary",
	})
	if err != nil {
		if os.IsNotExist(err) {
			return mission.Build(nil, mission.Facts{}, maxHandoffRows)
		}
		return mission.Brief{Summary: "unavailable: " + err.Error()}
	}
	return brief
}

func EnsureBoard(repoRoot, caseRoot, pack string) error {
	inst, err := instance.AssertAttached(caseRoot, repoRoot, pack)
	if err != nil {
		return err
	}
	m, err := manifest.Load(repoRoot, pack)
	if err != nil {
		return err
	}
	writes := []StartWrite{}
	if err := ensureWorkstreamState(inst.CaseRoot, m, &writes); err != nil {
		return err
	}
	_, err = saveBoard(inst.CaseRoot, m)
	return err
}

func startContext(repoRoot, caseRoot, pack string, opt StartOptions) (instance.Instance, *manifest.Manifest, manifest.LaneType, string, string, error) {
	inst, err := instance.AssertAttached(caseRoot, repoRoot, pack)
	if err != nil {
		return instance.Instance{}, nil, manifest.LaneType{}, "", "", err
	}
	m, err := manifest.Load(repoRoot, pack)
	if err != nil {
		return instance.Instance{}, nil, manifest.LaneType{}, "", "", err
	}
	name := strings.TrimSpace(opt.Name)
	if name == "" {
		return instance.Instance{}, nil, manifest.LaneType{}, "", "", fmt.Errorf("start requires a feature name, e.g. /rekit start login")
	}
	laneTypeID := strings.TrimSpace(m.WorkstreamDefaults["defaultStartLaneType"])
	if laneTypeID == "" {
		laneTypeID = defaultStartLaneType(m)
	}
	laneType, err := m.LaneType(laneTypeID)
	if err != nil {
		return instance.Instance{}, nil, manifest.LaneType{}, "", "", err
	}
	laneID := laneID(laneType.ID, name)
	if laneID == "" {
		return instance.Instance{}, nil, manifest.LaneType{}, "", "", fmt.Errorf("start produced an empty lane id for %q", name)
	}
	return inst, m, laneType, laneID, name, nil
}

func ensureWorkstreamState(caseRoot string, m *manifest.Manifest, writes *[]StartWrite) error {
	for _, rel := range []string{".rekit", ".rekit/lanes", ".rekit/facts", ".rekit/runs", ".rekit/reviews", ".rekit/backups"} {
		path, err := refsf.SafeJoin(caseRoot, rel)
		if err != nil {
			return err
		}
		if err := os.MkdirAll(path, 0o755); err != nil {
			return err
		}
	}
	for _, rel := range mission.FactRelPaths() {
		path, err := refsf.SafeJoin(caseRoot, rel)
		if err != nil {
			return err
		}
		action, err := ensureEmptyFile(path)
		if err != nil {
			return err
		}
		*writes = append(*writes, StartWrite{Path: rel, Kind: "fact-jsonl", Action: action, TargetPath: path})
	}
	policyPath, err := refsf.SafeJoin(caseRoot, ".rekit/policy.yml")
	if err != nil {
		return err
	}
	policyAction := "unchanged"
	if !refsf.Exists(policyPath) {
		if err := os.WriteFile(policyPath, []byte(defaultPolicyText), 0o644); err != nil {
			return err
		}
		policyAction = "create-policy"
	}
	*writes = append(*writes, StartWrite{Path: ".rekit/policy.yml", Kind: "policy", Action: policyAction, TargetPath: policyPath})

	authorityType, err := m.LaneType(m.WorkstreamDefaults["defaultAuthorityLane"])
	if err != nil {
		return err
	}
	authorityID := laneID(authorityType.ID, "")
	authorityFile, err := refsf.SafeJoin(caseRoot, relJoin(".rekit", "lanes", authorityID, "lane.json"))
	if err != nil {
		return err
	}
	if !refsf.Exists(authorityFile) {
		_, laneWrites, err := writeLane(caseRoot, m, authorityType, authorityID, "", false)
		if err != nil {
			return err
		}
		*writes = append(*writes, laneWrites...)
	}
	return nil
}

func writeLane(caseRoot string, m *manifest.Manifest, laneType manifest.LaneType, id, name string, force bool) (Lane, []StartWrite, error) {
	laneRootRel := relJoin(".rekit", "lanes", id)
	laneRoot, err := refsf.SafeJoin(caseRoot, laneRootRel)
	if err != nil {
		return Lane{}, nil, err
	}
	laneFile := filepath.Join(laneRoot, "lane.json")
	laneExists := refsf.Exists(laneFile)
	if laneExists && !force {
		lane, err := readLane(laneFile)
		if err != nil {
			return Lane{}, nil, err
		}
		profileRel, profilePath, err := autonomy.EnsureManualProfile(caseRoot, id)
		if err != nil {
			return Lane{}, nil, err
		}
		return lane, []StartWrite{
			{Path: relJoin(laneRootRel, "lane.json"), Kind: "lane", Action: "enter-existing-lane", TargetPath: laneFile},
			{Path: profileRel, Kind: "autonomy-profile", Action: "ensure-manual-profile", TargetPath: profilePath},
		}, nil
	}
	var existingLane Lane
	if laneExists {
		existingLane, err = readLane(laneFile)
		if err != nil {
			return Lane{}, nil, err
		}
	}
	workspaceRel := relJoin(laneType.WorkspaceRoot, id)
	workspace, err := refsf.SafeJoin(caseRoot, workspaceRel)
	if err != nil {
		return Lane{}, nil, err
	}
	for _, path := range []string{laneRoot, filepath.Join(laneRoot, "checkpoints"), filepath.Join(laneRoot, "prompts"), filepath.Join(laneRoot, "reviews"), workspace} {
		if err := os.MkdirAll(path, 0o755); err != nil {
			return Lane{}, nil, err
		}
	}
	writes := []StartWrite{}
	for i, path := range LaneJSONLPaths(laneRoot) {
		file := LaneJSONLFileNames()[i]
		action, err := ensureEmptyFile(path)
		if err != nil {
			return Lane{}, nil, err
		}
		writes = append(writes, StartWrite{Path: relJoin(laneRootRel, file), Kind: "lane-jsonl", Action: action, TargetPath: path})
	}
	for i, path := range WorkspaceJSONLPaths(workspace) {
		file := WorkspaceJSONLFileNames()[i]
		action, err := ensureEmptyFile(path)
		if err != nil {
			return Lane{}, nil, err
		}
		writes = append(writes, StartWrite{Path: relJoin(workspaceRel, file), Kind: "workspace-jsonl", Action: action, TargetPath: path})
	}
	if !laneType.Authority {
		for _, file := range []string{"summary.md", "evidence.md", "notes.md"} {
			path := filepath.Join(workspace, file)
			action := "unchanged"
			if !refsf.Exists(path) {
				if err := os.WriteFile(path, []byte("# "+id+"\r\n\r\n待填写。\r\n"), 0o644); err != nil {
					return Lane{}, nil, err
				}
				action = "create-workspace-note"
			}
			writes = append(writes, StartWrite{Path: relJoin(workspaceRel, file), Kind: "workspace-note", Action: action, TargetPath: path})
		}
	}
	now := time.Now().UTC().Format(time.RFC3339Nano)
	lane, err := plannedLane(caseRoot, laneType, id, name, now)
	if err != nil {
		return Lane{}, nil, err
	}
	action := "create-lane"
	eventKind := "lane-created"
	eventAction := "append-lane-created"
	eventSummary := "lane created: " + id
	if laneExists && force {
		action = "refresh-lane-with-force"
		eventKind = "lane-refreshed"
		eventAction = "append-lane-refreshed"
		eventSummary = "lane refreshed: " + id
		if strings.TrimSpace(existingLane.Status) != "" {
			lane.Status = existingLane.Status
		}
		if existingLane.Counters != nil {
			lane.Counters = existingLane.Counters
		}
		lane.CurrentExecutor = existingLane.CurrentExecutor
		lane.ExecutorGeneration = existingLane.ExecutorGeneration
		lane.LastTakeoverAt = existingLane.LastTakeoverAt
		lane.LastTakeoverReason = existingLane.LastTakeoverReason
		lane.LastReconciledIntervention = existingLane.LastReconciledIntervention
		lane.LastReconcileAt = existingLane.LastReconcileAt
		if strings.TrimSpace(existingLane.CreatedAt) != "" {
			lane.CreatedAt = existingLane.CreatedAt
		}
	}
	if err := writeJSON(laneFile, lane); err != nil {
		return Lane{}, nil, err
	}
	writes = append(writes, StartWrite{Path: relJoin(laneRootRel, "lane.json"), Kind: "lane", Action: action, TargetPath: laneFile})
	profileRel, profilePath, err := autonomy.EnsureManualProfile(caseRoot, id)
	if err != nil {
		return Lane{}, nil, err
	}
	writes = append(writes, StartWrite{Path: profileRel, Kind: "autonomy-profile", Action: "ensure-manual-profile", TargetPath: profilePath})
	eventPath := LaneEventsJSONLPath(laneRoot)
	event := map[string]any{"eventId": eventID(id, eventKind, now), "kind": eventKind, "lane": id, "time": now, "summary": eventSummary}
	if err := mission.AppendJSONLine(eventPath, event); err != nil {
		return Lane{}, nil, err
	}
	writes = append(writes, StartWrite{Path: relJoin(laneRootRel, "events.jsonl"), Kind: "lane-event", Action: eventAction, TargetPath: eventPath})
	resumePath, checkpointPath, err := writeLaneResume(caseRoot, m, lane)
	if err != nil {
		return Lane{}, nil, err
	}
	writes = append(writes, StartWrite{Path: relJoin(laneRootRel, "prompts", "RESUME.md"), Kind: "lane-resume", Action: "refresh", TargetPath: resumePath})
	writes = append(writes, StartWrite{Path: relJoin(laneRootRel, "checkpoints", "latest.json"), Kind: "lane-checkpoint", Action: "refresh", TargetPath: checkpointPath})
	return lane, writes, nil
}

func plannedLane(caseRoot string, laneType manifest.LaneType, id, name, now string) (Lane, error) {
	workspaceRel := relJoin(laneType.WorkspaceRoot, id)
	workspace, err := refsf.SafeJoin(caseRoot, workspaceRel)
	if err != nil {
		return Lane{}, err
	}
	laneRoot, err := refsf.SafeJoin(caseRoot, relJoin(".rekit", "lanes", id))
	if err != nil {
		return Lane{}, err
	}
	title := laneType.Title
	if strings.TrimSpace(name) != "" {
		title += ": " + name
	}
	return Lane{
		SchemaVersion: 1,
		ID:            id,
		Type:          laneType.ID,
		Name:          name,
		Title:         title,
		Status:        "open",
		Authority:     laneType.Authority,
		Workspace:     relativePath(caseRoot, workspace),
		LaneRoot:      relativePath(caseRoot, laneRoot),
		CanWrite:      append([]string{}, laneType.CanWrite...),
		ReadOnly:      append([]string{}, laneType.ReadOnly...),
		Outputs:       append([]string{}, laneType.Outputs...),
		Counters:      map[string]int{"observations": 0, "requests": 0, "candidates": 0, "publications": 0, "pendingUser": 0},
		CreatedAt:     now,
		UpdatedAt:     now,
	}, nil
}

func saveBoard(caseRoot string, m *manifest.Manifest) (string, error) {
	lanesRoot, err := refsf.SafeJoin(caseRoot, ".rekit/lanes")
	if err != nil {
		return "", err
	}
	entries, err := os.ReadDir(lanesRoot)
	if err != nil {
		return "", err
	}
	lanes := []boardLane{}
	for _, entry := range entries {
		if !entry.IsDir() {
			continue
		}
		lane, err := readLane(filepath.Join(lanesRoot, entry.Name(), "lane.json"))
		if os.IsNotExist(err) {
			continue
		}
		if err != nil {
			return "", err
		}
		lanes = append(lanes, boardLane{ID: lane.ID, Type: lane.Type, Title: lane.Title, Status: lane.Status, Authority: lane.Authority, Workspace: lane.Workspace, UpdatedAt: lane.UpdatedAt})
	}
	sort.SliceStable(lanes, func(i, j int) bool { return lanes[i].ID < lanes[j].ID })
	path, err := refsf.SafeJoin(caseRoot, ".rekit/board.json")
	if err != nil {
		return "", err
	}
	b := board{SchemaVersion: 1, CaseRoot: caseRoot, RepoRoot: m.RepoRoot, Pack: m.Pack, AutomationMode: readAutomationMode(caseRoot), DefaultAuthorityLane: m.WorkstreamDefaults["defaultAuthorityLane"], Lanes: lanes, FactsRoot: ".rekit/facts", UpdatedAt: time.Now().UTC().Format(time.RFC3339Nano)}
	return path, writeJSON(path, b)
}

func writeLaneResume(caseRoot string, m *manifest.Manifest, lane Lane) (string, string, error) {
	laneRoot, err := laneRootPath(caseRoot, lane)
	if err != nil {
		return "", "", err
	}
	inbox, err := mission.ReadJSONLineObjects(LaneInboxJSONLPath(laneRoot))
	if err != nil {
		return "", "", err
	}
	tasks, err := mission.ReadJSONLineObjects(LaneTasksJSONLPath(laneRoot))
	if err != nil {
		return "", "", err
	}
	openInterventions, err := openLaneInterventionSummaries(caseRoot, lane.ID)
	if err != nil {
		return "", "", err
	}
	ledgerFacts, err := mission.ReadLedgerFacts(caseRoot)
	if err != nil {
		return "", "", err
	}
	laneFacts := mission.LaneFacts(ledgerFacts.Facts, lane.ID)
	brief := laneMissionBrief(lane, ledgerFacts)
	pendingGateLines := missionLines(mission.FilterLane(laneFacts.Requests, lane.ID, "pending-gate"), mission.LaneGateLine)
	authorizedGateLines := missionLines(mission.FilterLane(laneFacts.Requests, lane.ID, "authorized-gate"), mission.LaneGateLine)
	autonomySummary := autonomy.ReadSummary(caseRoot, lane.ID, m)
	executorAction := laneExecutorActionFor(lane, laneFacts, brief)
	lines := []string{
		"# RESUME：" + lane.ID,
		"",
		"lane type: `" + lane.Type + "`",
		"status: `" + lane.Status + "`",
		"workspace: `" + lane.Workspace + "`",
		"current executor: `" + firstText(lane.CurrentExecutor, "unassigned") + "`",
		fmt.Sprintf("executor generation: `%d`", lane.ExecutorGeneration),
		"last reconciled intervention: `" + firstText(lane.LastReconciledIntervention, "none") + "`",
		"autonomy mode: `" + firstText(autonomySummary.Mode, autonomy.ModeManualGate) + "`",
		"autonomy profile: `" + firstText(autonomySummary.ProfilePath, autonomy.RelPath(lane.ID)) + "`",
		"autonomy ready: `" + fmt.Sprintf("%t", autonomySummary.Ready) + "`",
		"",
		"## 边界",
		"",
		"- 只写本工作线 workspace，除非 lane.json 明确列入 canWrite。",
		"- authority 文件只由主线或 policy gate 写入。",
		"- 新发现写入 outbox.jsonl / workspace observations.jsonl / requests.jsonl / candidates.jsonl。",
		"- autonomy profile 只限定 heavy-action 预授权边界；不放宽 authority/confirmed/sync/promote。",
		"- heavy-tool execution 仍需有效 gate authorization decision，并且执行后必须记录 evidence。",
		"",
		"## Autonomy profile",
		"",
		"- mode: `" + firstText(autonomySummary.Mode, autonomy.ModeManualGate) + "`",
		"- profile: `" + firstText(autonomySummary.ProfilePath, autonomy.RelPath(lane.ID)) + "`",
		"- ready: `" + fmt.Sprintf("%t", autonomySummary.Ready) + "` valid=`" + fmt.Sprintf("%t", autonomySummary.Valid) + "` expired=`" + fmt.Sprintf("%t", autonomySummary.Expired) + "`",
		"- allowed actions: `" + firstText(strings.Join(autonomySummary.AllowedActions, ","), "none") + "`",
		"- denied actions: `" + firstText(strings.Join(autonomySummary.DeniedActions, ","), "none") + "`",
		"- output paths: `" + firstText(strings.Join(autonomySummary.OutputPaths, ","), "none") + "`",
		"- record required: `" + fmt.Sprintf("%t", autonomySummary.RecordRequired) + "`",
		"",
		"## Mission Control brief",
		"",
		"- summary: " + brief.Summary,
	}
	lines = appendResumeList(lines, "ready lanes", brief.ReadyLanes)
	lines = appendResumeList(lines, "blocked lanes", brief.BlockedLanes)
	lines = appendResumeList(lines, "pending gates", brief.PendingGates)
	lines = appendResumeList(lines, "authorized gates", brief.AuthorizedGates)
	lines = appendResumeList(lines, "open decisions", brief.OpenDecisions)
	lines = appendResumeList(lines, "interventions", brief.Interventions)
	lines = appendResumeList(lines, "next agent actions", brief.NextAgentActions)
	lines = appendResumeList(lines, "escalations", brief.Escalations)
	lines = append(lines,
		"",
		"## Executor action snapshot",
		"",
		"- blocked: `"+fmt.Sprintf("%t", executorAction.Blocked)+"`",
		"- ready: `"+fmt.Sprintf("%t", executorAction.Ready)+"`",
		"- pending gates: `"+fmt.Sprintf("%d", executorAction.PendingGates)+"`",
		"- open interventions: `"+fmt.Sprintf("%d", executorAction.OpenInterventions)+"`",
		"- open decisions: `"+fmt.Sprintf("%d", executorAction.OpenDecisions)+"`",
		"- reconcile required: `"+fmt.Sprintf("%t", executorAction.ReconcileRequired)+"`",
		"- pending gate required: `"+fmt.Sprintf("%t", executorAction.PendingGateRequired)+"`",
		"- open decision required: `"+fmt.Sprintf("%t", executorAction.OpenDecisionRequired)+"`",
		"- resume command: `"+executorAction.ResumeCommand+"`",
		"- handoff command: `"+executorAction.HandoffCommand+"`",
	)
	lines = appendResumeList(lines, "blocker reasons", executorAction.BlockerReasons)
	lines = appendResumeList(lines, "executor next actions", executorAction.NextAgentActions)
	lines = appendResumeList(lines, "executor escalations", executorAction.Escalations)
	lines = append(lines,
		"",
		"## Heavy-action gate decisions",
		"",
	)
	lines = appendResumeList(lines, "pending-gate", pendingGateLines)
	lines = appendResumeList(lines, "authorized-gate", authorizedGateLines)
	lines = append(lines,
		"",
		"## 最近 inbox",
		"",
	)
	for _, msg := range lastObjects(inbox, 8) {
		lines = append(lines, "- "+firstObjectText(msg, "summary", "kind", "eventId"))
	}
	if len(inbox) == 0 {
		lines = append(lines, "- 无。")
	}
	lines = append(lines, "", "## Open interventions", "")
	for _, item := range openInterventions {
		lines = append(lines, "- "+firstText(item.Subject, item.Summary, item.EventID)+" | eventId=`"+item.EventID+"` | status=`"+item.Status+"`")
	}
	if len(openInterventions) == 0 {
		lines = append(lines, "- 无。")
	}
	openTasks := []map[string]any{}
	for _, task := range tasks {
		status := strings.ToLower(strings.TrimSpace(firstObjectText(task, "status")))
		if status != "closed" && status != "resolved" {
			openTasks = append(openTasks, task)
		}
	}
	lines = append(lines, "", "## 未关闭任务", "")
	for _, task := range lastObjects(openTasks, 12) {
		lines = append(lines, "- "+firstObjectText(task, "summary", "taskId", "eventId"))
	}
	if len(openTasks) == 0 {
		lines = append(lines, "- 无。")
	}
	resume := strings.Join(append(lines, ""), "\r\n")
	resumePath := filepath.Join(laneRoot, "prompts", "RESUME.md")
	if err := os.MkdirAll(filepath.Dir(resumePath), 0o755); err != nil {
		return "", "", err
	}
	if err := os.WriteFile(resumePath, []byte(resume), 0o644); err != nil {
		return "", "", err
	}
	checkpointPath := filepath.Join(laneRoot, "checkpoints", "latest.json")
	checkpoint := laneCheckpoint{
		SchemaVersion:              1,
		Lane:                       lane.ID,
		Status:                     lane.Status,
		Workspace:                  lane.Workspace,
		CurrentExecutor:            lane.CurrentExecutor,
		ExecutorGeneration:         lane.ExecutorGeneration,
		LastReconciledIntervention: lane.LastReconciledIntervention,
		LastReconcileAt:            lane.LastReconcileAt,
		AutonomyProfile:            autonomySummary,
		MissionBrief:               brief,
		ExecutorAction:             executorAction,
		PendingGates:               pendingGateLines,
		AuthorizedGates:            authorizedGateLines,
		OpenInterventions:          openInterventions,
		Inbox:                      len(inbox),
		Tasks:                      len(tasks),
		UpdatedAt:                  time.Now().UTC().Format(time.RFC3339Nano),
		Resume:                     relativePath(caseRoot, resumePath),
	}
	if err := writeJSON(checkpointPath, checkpoint); err != nil {
		return "", "", err
	}
	return resumePath, checkpointPath, nil
}

func laneExecutorActionFor(lane Lane, laneFacts mission.Facts, brief mission.Brief) laneExecutorAction {
	label := workstreamLabel(lane)
	laneFacts = mission.LaneFacts(laneFacts, lane.ID)
	pendingGates := len(mission.FilterLane(laneFacts.Requests, lane.ID, "pending-gate"))
	openInterventions := len(mission.EffectiveOpenInterventions(laneFacts.Interventions))
	openDecisions := len(mission.OpenDecisionItems(laneFacts))
	reasons := []string{}
	if pendingGates > 0 {
		reasons = append(reasons, "pending-gate")
	}
	if openInterventions > 0 {
		reasons = append(reasons, "intervention")
	}
	if openDecisions > 0 {
		reasons = append(reasons, "open-decision")
	}
	blocked := len(reasons) > 0
	return laneExecutorAction{
		Blocked:              blocked,
		Ready:                !blocked && slices.Contains(brief.ReadyLanes, label),
		BlockerReasons:       reasons,
		PendingGates:         pendingGates,
		OpenInterventions:    openInterventions,
		OpenDecisions:        openDecisions,
		ReconcileRequired:    openInterventions > 0,
		PendingGateRequired:  pendingGates > 0,
		OpenDecisionRequired: openDecisions > 0,
		ResumeCommand:        "/rekit continue " + label,
		HandoffCommand:       "/rekit handoff " + label,
		NextAgentActions:     brief.NextAgentActions,
		Escalations:          brief.Escalations,
	}
}

func defaultStartLaneType(m *manifest.Manifest) string {
	for _, lane := range m.LaneTypes {
		if !lane.Authority {
			return lane.ID
		}
	}
	return m.WorkstreamDefaults["defaultAuthorityLane"]
}

func laneID(laneType, name string) string {
	raw := laneType
	if strings.TrimSpace(name) != "" {
		if strings.Contains(laneType, "feature") {
			raw = "feature-" + name
		} else {
			raw = laneType + "-" + name
		}
	}
	safe := regexp.MustCompile(`[^a-z0-9._-]+`).ReplaceAllString(strings.ToLower(strings.TrimSpace(raw)), "-")
	return strings.Trim(safe, "-_.")
}

func workstreamLabel(lane Lane) string {
	if lane.Authority {
		return "main"
	}
	if name, ok := strings.CutPrefix(lane.ID, "feature-"); ok {
		return name
	}
	return lane.ID
}

func validateLaneIDSegment(id string) error {
	if !safeLaneIDSegment.MatchString(strings.TrimSpace(id)) {
		return fmt.Errorf("invalid lane id path segment: %s", id)
	}
	return nil
}

func laneRootPath(caseRoot string, lane Lane) (string, error) {
	if err := validateLaneIDSegment(lane.ID); err != nil {
		return "", err
	}
	rootFromID, err := refsf.SafeJoin(caseRoot, relJoin(".rekit", "lanes", lane.ID))
	if err != nil {
		return "", err
	}
	if strings.TrimSpace(lane.LaneRoot) == "" {
		return rootFromID, nil
	}
	rootFromLane, err := refsf.SafeJoin(caseRoot, lane.LaneRoot)
	if err != nil {
		return "", err
	}
	if !strings.EqualFold(filepath.Clean(rootFromLane), filepath.Clean(rootFromID)) {
		return "", fmt.Errorf("laneRoot mismatch for %s: got %s, want %s", lane.ID, lane.LaneRoot, relativePath(caseRoot, rootFromID))
	}
	return rootFromID, nil
}

func ensureEmptyFile(path string) (string, error) {
	if refsf.Exists(path) {
		return "unchanged", nil
	}
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return "", err
	}
	if err := os.WriteFile(path, []byte{}, 0o644); err != nil {
		return "", err
	}
	return "create-empty-file", nil
}

func readLane(path string) (Lane, error) {
	b, err := os.ReadFile(path)
	if err != nil {
		return Lane{}, err
	}
	var lane Lane
	if err := json.Unmarshal(b, &lane); err != nil {
		return Lane{}, fmt.Errorf("invalid lane json %s: %w", path, err)
	}
	return lane, nil
}

func lastObjects(items []map[string]any, limit int) []map[string]any {
	if limit <= 0 || len(items) <= limit {
		return items
	}
	return items[len(items)-limit:]
}

func appendResumeList(lines []string, label string, items []string) []string {
	if len(items) == 0 {
		return append(lines, "- "+label+": none")
	}
	lines = append(lines, "- "+label+":")
	for _, item := range mission.LimitStrings(items, maxHandoffRows) {
		lines = append(lines, "  - "+item)
	}
	return lines
}

func firstObjectText(item map[string]any, keys ...string) string {
	for _, key := range keys {
		if value, ok := item[key]; ok {
			text := objectText(value)
			if text != "" {
				return text
			}
		}
	}
	return ""
}

func objectText(value any) string {
	if value == nil {
		return ""
	}
	switch t := value.(type) {
	case string:
		return strings.TrimSpace(t)
	case []any:
		parts := []string{}
		for _, item := range t {
			text := objectText(item)
			if text != "" {
				parts = append(parts, text)
			}
		}
		return strings.Join(parts, ",")
	default:
		text := strings.TrimSpace(fmt.Sprint(t))
		if text == "<nil>" {
			return ""
		}
		return text
	}
}

func readAutomationMode(caseRoot string) string {
	path, err := refsf.SafeJoin(caseRoot, ".rekit/policy.yml")
	if err != nil {
		return "assisted-autopilot"
	}
	b, err := os.ReadFile(path)
	if err != nil {
		return "assisted-autopilot"
	}
	for line := range strings.SplitSeq(strings.ReplaceAll(string(b), "\r\n", "\n"), "\n") {
		parts := strings.SplitN(strings.TrimSpace(line), ":", 2)
		if len(parts) == 2 && parts[0] == "automationMode" {
			mode := strings.TrimSpace(parts[1])
			if mode != "" {
				return mode
			}
		}
	}
	return "assisted-autopilot"
}

func writeJSON(path string, v any) error {
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return err
	}
	b, err := json.MarshalIndent(v, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(path, append(b, '\n'), 0o644)
}

func eventID(laneID, kind, createdAt string) string {
	sum := sha256.Sum256([]byte(laneID + "|" + kind + "|" + createdAt))
	return "evt-" + hex.EncodeToString(sum[:])[:16]
}

func relativePath(root, path string) string {
	rel, err := filepath.Rel(root, path)
	if err != nil {
		return filepath.ToSlash(path)
	}
	return filepath.ToSlash(rel)
}

func relJoin(parts ...string) string {
	clean := []string{}
	for _, part := range parts {
		part = strings.TrimSpace(part)
		if part != "" {
			clean = append(clean, filepath.FromSlash(part))
		}
	}
	return filepath.ToSlash(filepath.Join(clean...))
}
