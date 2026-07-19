package overview

import (
	"bytes"
	"encoding/json"
	"fmt"
	"maps"
	"os"
	"sort"
	"strconv"
	"strings"

	"github.com/shuiyu486/re-context-kits/internal/rekit/autonomy"
	"github.com/shuiyu486/re-context-kits/internal/rekit/instance"
	"github.com/shuiyu486/re-context-kits/internal/rekit/manifest"
	"github.com/shuiyu486/re-context-kits/internal/rekit/mission"
	"github.com/shuiyu486/re-context-kits/internal/rekit/workstream"
)

const maxRows = 10

type event = map[string]any

type Inventory struct {
	SchemaVersion           int                                      `json:"schemaVersion"`
	Command                 string                                   `json:"command"`
	CaseRoot                string                                   `json:"caseRoot"`
	RepoRoot                string                                   `json:"repoRoot"`
	Pack                    string                                   `json:"pack"`
	IsMutation              bool                                     `json:"isMutation"`
	AutomationMode          string                                   `json:"automationMode"`
	Lanes                   []LaneSummary                            `json:"lanes"`
	Counts                  FactCounts                               `json:"counts"`
	MissionBrief            MissionBrief                             `json:"missionBrief"`
	LaneExecutorActions     []mission.LaneExecutorActionSnapshot     `json:"laneExecutorActions"`
	MissionCommanderActions []MissionCommanderActionIndexItem        `json:"missionCommanderActions"`
	ExecutionEvidenceReview []workstream.ExecutionEvidenceReviewItem `json:"executionEvidenceReview"`
	Sections                OverviewSections                         `json:"sections"`
	NextSteps               []string                                 `json:"nextSteps"`
}

type LaneSummary struct {
	ID                 string `json:"id"`
	Label              string `json:"label"`
	Kind               string `json:"kind"`
	Status             string `json:"status"`
	Workspace          string `json:"workspace"`
	Authority          bool   `json:"authority"`
	CurrentExecutor    string `json:"currentExecutor,omitempty"`
	ExecutorGeneration int    `json:"executorGeneration,omitempty"`
	LastTakeoverAt     string `json:"lastTakeoverAt,omitempty"`
	LastTakeoverBy     string `json:"lastTakeoverBy,omitempty"`
	LastTakeoverReason string `json:"lastTakeoverReason,omitempty"`
	AutonomyMode       string `json:"autonomyMode"`
	AutonomyReady      bool   `json:"autonomyReady"`
	AutonomyProfile    string `json:"autonomyProfile"`
}

type MissionCommanderActionIndexItem struct {
	Lane             string                         `json:"lane"`
	Label            string                         `json:"label"`
	Status           string                         `json:"status"`
	Blocked          bool                           `json:"blocked"`
	Ready            bool                           `json:"ready"`
	BlockerReasons   []string                       `json:"blockerReasons,omitempty"`
	PrimaryCommand   string                         `json:"primaryCommand,omitempty"`
	FollowUpCommands []string                       `json:"followUpCommands,omitempty"`
	Boundary         []string                       `json:"boundary,omitempty"`
	Action           mission.MissionCommanderAction `json:"action"`
}

type MissionBrief = mission.Brief

type FactCounts struct {
	Observations     int `json:"observations"`
	Requests         int `json:"requests"`
	Candidates       int `json:"candidates"`
	Publications     int `json:"publications"`
	PendingDecisions int `json:"pendingDecisions"`
}

type OverviewSections struct {
	OpenCandidates    EventSection `json:"openCandidates"`
	PendingGates      EventSection `json:"pendingGates"`
	AuthorizedGates   EventSection `json:"authorizedGates"`
	Verifications     EventSection `json:"verifications"`
	Decisions         EventSection `json:"decisions"`
	Batches           BatchSection `json:"batches"`
	OpenInterventions EventSection `json:"openInterventions"`
	Interventions     EventSection `json:"interventions"`
	Rollbacks         EventSection `json:"rollbacks"`
}

type EventSection struct {
	Total  int              `json:"total"`
	Shown  int              `json:"shown"`
	Events []map[string]any `json:"events"`
}

type BatchSection struct {
	Total   int            `json:"total"`
	Shown   int            `json:"shown"`
	Batches []BatchSummary `json:"batches"`
}

type BatchSummary struct {
	ID     string         `json:"id"`
	Events int            `json:"events"`
	Kinds  map[string]int `json:"kinds"`
	Last   string         `json:"last"`
}

type overviewData struct {
	inst        instance.Instance
	manifest    *manifest.Manifest
	board       event
	facts       factSet
	lanes       []event
	pending     int
	sections    OverviewSections
	initialized bool
}

type batchSummary struct {
	id       string
	events   []event
	lastTime string
	lastIdx  int
}

type factSet = mission.LedgerFacts

func Render(repoRoot, caseRoot, pack string) (string, error) {
	data, err := loadOverviewData(repoRoot, caseRoot, pack)
	if err != nil {
		return "", err
	}
	facts := data.facts

	var out bytes.Buffer
	fmt.Fprintf(&out, "项目概览：%s\n", data.inst.CaseRoot)
	fmt.Fprintf(&out, "自动化模式：%s\n", stringValue(data.board, "automationMode"))
	fmt.Fprintln(&out, "当前是项目总览，还没有为本会话选择具体工作线。")
	fmt.Fprintln(&out)
	fmt.Fprintln(&out, "工作线：")
	for _, lane := range data.lanes {
		kind := "功能/工具支线"
		if boolValue(lane, "authority") {
			kind = "主线"
		}
		autonomySummary := autonomy.ReadSummary(data.inst.CaseRoot, stringValue(lane, "id"), data.manifest)
		fmt.Fprintf(&out, "- %s：%s，选择名=%s，状态=%s，工作区=%s，executor=%s generation=%d，autonomy=%s ready=%t\n", kind, stringValue(lane, "id"), workstreamLabel(lane), stringValue(lane, "status"), stringValue(lane, "workspace"), firstText(stringValue(lane, "currentExecutor"), "unassigned"), intValue(lane, "executorGeneration"), autonomySummary.Mode, autonomySummary.Ready)
	}
	fmt.Fprintln(&out)
	fmt.Fprintln(&out, "共享事实：")
	fmt.Fprintf(&out, "- observation: %d\n", len(facts.Observations))
	fmt.Fprintf(&out, "- request: %d\n", len(facts.Requests))
	fmt.Fprintf(&out, "- candidate: %d\n", len(facts.Candidates))
	fmt.Fprintf(&out, "- publication: %d\n", len(facts.Publications))
	fmt.Fprintf(&out, "- 需要确认: %d\n", data.pending)
	fmt.Fprintln(&out)

	brief := buildMissionBrief(data.lanes, facts)
	actions := buildLaneExecutorActions(data.lanes, facts, brief)
	evidenceReview := overviewExecutionEvidenceReview(data.lanes, facts)
	writeMissionBrief(&out, brief)
	writeLaneExecutorActions(&out, actions)
	writeMissionCommanderActionIndex(&out, missionCommanderActionIndex(actions))
	writeExecutionEvidenceReview(&out, evidenceReview)
	writeOpenCandidates(&out, facts.Candidates)
	writePendingGates(&out, facts.Requests)
	writeAuthorizedGates(&out, facts.Requests)
	writeVerifications(&out, facts.Verifications)
	writeDecisions(&out, facts.Decisions)
	writeBatches(&out, facts.AllBatchEvents)
	writeInterventions(&out, facts.Interventions)
	writeRollbacks(&out, facts.Rollbacks)
	writeNextSteps(&out, overviewNextSteps(brief, evidenceReview))
	return out.String(), nil
}

func BuildInventory(repoRoot, caseRoot, pack string) (Inventory, error) {
	data, err := loadOverviewData(repoRoot, caseRoot, pack)
	if err != nil {
		return Inventory{}, err
	}
	lanes := make([]LaneSummary, 0, len(data.lanes))
	for _, lane := range data.lanes {
		kind := "feature"
		if boolValue(lane, "authority") {
			kind = "main"
		}
		laneID := stringValue(lane, "id")
		autonomySummary := autonomy.ReadSummary(data.inst.CaseRoot, laneID, data.manifest)
		lanes = append(lanes, LaneSummary{
			ID:                 laneID,
			Label:              workstreamLabel(lane),
			Kind:               kind,
			Status:             stringValue(lane, "status"),
			Workspace:          stringValue(lane, "workspace"),
			Authority:          boolValue(lane, "authority"),
			CurrentExecutor:    stringValue(lane, "currentExecutor"),
			ExecutorGeneration: intValue(lane, "executorGeneration"),
			LastTakeoverAt:     stringValue(lane, "lastTakeoverAt"),
			LastTakeoverBy:     stringValue(lane, "lastTakeoverBy"),
			LastTakeoverReason: stringValue(lane, "lastTakeoverReason"),
			AutonomyMode:       autonomySummary.Mode,
			AutonomyReady:      autonomySummary.Ready,
			AutonomyProfile:    autonomySummary.ProfilePath,
		})
	}
	facts := data.facts
	brief := buildMissionBrief(data.lanes, facts)
	actions := buildLaneExecutorActions(data.lanes, facts, brief)
	evidenceReview := overviewExecutionEvidenceReview(data.lanes, facts)
	return Inventory{
		SchemaVersion:  1,
		Command:        "overview",
		CaseRoot:       data.inst.CaseRoot,
		RepoRoot:       repoRoot,
		Pack:           pack,
		IsMutation:     data.initialized,
		AutomationMode: stringValue(data.board, "automationMode"),
		Lanes:          lanes,
		Counts: FactCounts{
			Observations:     len(facts.Observations),
			Requests:         len(facts.Requests),
			Candidates:       len(facts.Candidates),
			Publications:     len(facts.Publications),
			PendingDecisions: data.pending,
		},
		MissionBrief:            brief,
		LaneExecutorActions:     actions,
		MissionCommanderActions: missionCommanderActionIndex(actions),
		ExecutionEvidenceReview: evidenceReview,
		Sections:                data.sections,
		NextSteps:               overviewNextSteps(brief, evidenceReview),
	}, nil
}

func loadOverviewData(repoRoot, caseRoot, pack string) (overviewData, error) {
	inst, err := instance.AssertAttached(caseRoot, repoRoot, pack)
	if err != nil {
		return overviewData{}, err
	}
	m, err := manifest.Load(repoRoot, pack)
	if err != nil {
		return overviewData{}, err
	}
	board, err := mission.ReadBoard(inst.CaseRoot)
	initialized := false
	if os.IsNotExist(err) {
		if err := workstream.EnsureBoard(repoRoot, inst.CaseRoot, pack); err != nil {
			return overviewData{}, err
		}
		initialized = true
		board, err = mission.ReadBoard(inst.CaseRoot)
	}
	if err != nil {
		return overviewData{}, err
	}
	facts, err := readFacts(inst.CaseRoot)
	if err != nil {
		return overviewData{}, err
	}
	return overviewData{
		inst:        inst,
		manifest:    m,
		board:       boardEvent(board),
		facts:       facts,
		lanes:       boardLaneEvents(board.Lanes),
		pending:     facts.PendingDecision,
		sections:    buildOverviewSections(facts),
		initialized: initialized,
	}, nil
}

func buildOverviewSections(facts factSet) OverviewSections {
	openCandidates := openStatusEvents(facts.Candidates)
	pendingGates := []event{}
	authorizedGates := []event{}
	for _, request := range facts.Requests {
		if mission.IsPendingGateRequest(request) {
			pendingGates = append(pendingGates, request)
			continue
		}
		if mission.IsAuthorizedGateRequest(request) {
			authorizedGates = append(authorizedGates, request)
		}
	}
	openInterventions := mission.EffectiveOpenInterventions(facts.Interventions)
	return OverviewSections{
		OpenCandidates:    newEventSection(openCandidates),
		PendingGates:      newEventSection(pendingGates),
		AuthorizedGates:   newEventSection(authorizedGates),
		Verifications:     newEventSection(facts.Verifications),
		Decisions:         newEventSection(facts.Decisions),
		Batches:           newBatchSection(facts.AllBatchEvents),
		OpenInterventions: newEventSection(openInterventions),
		Interventions:     newEventSection(facts.Interventions),
		Rollbacks:         newEventSection(facts.Rollbacks),
	}
}

func buildMissionBrief(lanes []event, facts factSet) MissionBrief {
	return mission.Build(missionLanes(lanes), facts.Facts, maxRows)
}

func buildLaneExecutorActions(lanes []event, facts factSet, brief MissionBrief) []mission.LaneExecutorActionSnapshot {
	return mission.LaneExecutorActionSnapshots(boardLanes(lanes), facts.Facts, brief)
}

func overviewExecutionEvidenceReview(lanes []event, facts factSet) []workstream.ExecutionEvidenceReviewItem {
	labels := map[string]string{}
	for _, lane := range lanes {
		laneID := stringValue(lane, "id")
		if laneID != "" {
			labels[laneID] = workstreamLabel(lane)
		}
	}
	return workstream.ExecutionEvidenceReviewItems(facts.Observations, "", func(laneID string) string {
		if label := labels[laneID]; label != "" {
			return label
		}
		return mission.BoardLaneLabel(mission.BoardLane{ID: laneID})
	})
}

func boardLanes(lanes []event) []mission.BoardLane {
	items := make([]mission.BoardLane, 0, len(lanes))
	for _, lane := range lanes {
		items = append(items, mission.BoardLane{
			ID:                 stringValue(lane, "id"),
			Status:             stringValue(lane, "status"),
			Authority:          boolValue(lane, "authority"),
			Workspace:          stringValue(lane, "workspace"),
			CurrentExecutor:    stringValue(lane, "currentExecutor"),
			ExecutorGeneration: intValue(lane, "executorGeneration"),
			LastTakeoverAt:     stringValue(lane, "lastTakeoverAt"),
			LastTakeoverBy:     stringValue(lane, "lastTakeoverBy"),
			LastTakeoverReason: stringValue(lane, "lastTakeoverReason"),
		})
	}
	return items
}

func missionLanes(lanes []event) []mission.Lane {
	return mission.BoardLanes(boardLanes(lanes))
}

func writeMissionBrief(out *bytes.Buffer, brief MissionBrief) {
	fmt.Fprintln(out, "Mission Control brief：")
	fmt.Fprintf(out, "- summary: %s\n", brief.Summary)
	writeBriefList(out, "ready lanes", brief.ReadyLanes)
	writeBriefList(out, "blocked lanes", brief.BlockedLanes)
	writeBriefList(out, "pending gates", brief.PendingGates)
	writeBriefList(out, "authorized gates", brief.AuthorizedGates)
	writeBriefList(out, "open decisions", brief.OpenDecisions)
	writeBriefList(out, "interventions", brief.Interventions)
	writeBriefList(out, "next agent actions", brief.NextAgentActions)
	writeBriefList(out, "escalations", brief.Escalations)
	fmt.Fprintln(out)
}

func writeLaneExecutorActions(out *bytes.Buffer, actions []mission.LaneExecutorActionSnapshot) {
	fmt.Fprintln(out, "Lane executor actions：")
	for _, item := range actions {
		action := item.ExecutorAction
		fmt.Fprintf(out, "- %s：blocked=%t ready=%t pendingGates=%d openInterventions=%d openDecisions=%d\n", item.Label, action.Blocked, action.Ready, action.PendingGates, action.OpenInterventions, action.OpenDecisions)
		fmt.Fprintf(out, "  - executor: current=%s generation=%d lastTakeover=%s by=%s reason=%s\n", firstText(item.CurrentExecutor, "unassigned"), item.ExecutorGeneration, firstText(item.LastTakeoverAt, "none"), firstText(item.LastTakeoverBy, "none"), firstText(item.LastTakeoverReason, "none"))
		fmt.Fprintf(out, "  - requirements: reconcile=%t pendingGate=%t openDecision=%t\n", action.ReconcileRequired, action.PendingGateRequired, action.OpenDecisionRequired)
		if len(action.BlockerReasons) > 0 {
			fmt.Fprintf(out, "  - blocker reasons: %s\n", strings.Join(action.BlockerReasons, ","))
		}
		fmt.Fprintf(out, "  - continue: %s\n", action.ResumeCommand)
		fmt.Fprintf(out, "  - handoff: %s\n", action.HandoffCommand)
		fmt.Fprintf(out, "  - commander: state=%s prompt=%s\n", action.MissionCommanderAction.State, action.MissionCommanderAction.Prompt)
	}
	fmt.Fprintln(out)
}

func missionCommanderActionIndex(actions []mission.LaneExecutorActionSnapshot) []MissionCommanderActionIndexItem {
	items := make([]MissionCommanderActionIndexItem, 0, len(actions))
	for _, item := range actions {
		action := item.ExecutorAction.MissionCommanderAction
		items = append(items, MissionCommanderActionIndexItem{
			Lane:             item.Lane,
			Label:            item.Label,
			Status:           item.Status,
			Blocked:          item.ExecutorAction.Blocked,
			Ready:            item.ExecutorAction.Ready,
			BlockerReasons:   append([]string{}, item.ExecutorAction.BlockerReasons...),
			PrimaryCommand:   action.PrimaryCommand,
			FollowUpCommands: append([]string{}, action.FollowUpCommands...),
			Boundary:         append([]string{}, action.Boundary...),
			Action:           action,
		})
	}
	return items
}

func writeMissionCommanderActionIndex(out *bytes.Buffer, items []MissionCommanderActionIndexItem) {
	fmt.Fprintln(out, "Mission Commander action index：")
	if len(items) == 0 {
		fmt.Fprintln(out, "- none")
		fmt.Fprintln(out)
		return
	}
	for _, item := range items {
		fmt.Fprintf(out, "- %s：state=%s blocked=%t ready=%t primary=`%s`\n", item.Label, item.Action.State, item.Blocked, item.Ready, item.PrimaryCommand)
		fmt.Fprintf(out, "  - prompt: %s\n", item.Action.Prompt)
		writeActionIndexList(out, "follow-up", item.FollowUpCommands)
		writeActionIndexList(out, "boundary", item.Boundary)
		if len(item.BlockerReasons) > 0 {
			fmt.Fprintf(out, "  - blocker reasons: %s\n", strings.Join(item.BlockerReasons, ","))
		}
	}
	fmt.Fprintln(out)
}

func writeExecutionEvidenceReview(out *bytes.Buffer, items []workstream.ExecutionEvidenceReviewItem) {
	fmt.Fprintln(out, "Execution evidence review：")
	if len(items) == 0 {
		fmt.Fprintln(out, "- none")
		fmt.Fprintln(out)
		return
	}
	for _, item := range items {
		fmt.Fprintf(out, "- %s：status=%s gateEventId=%s action=%s laneHandoff=`%s`\n", firstText(item.Subject, item.Summary, item.EventID), item.Status, item.GateEventID, firstText(item.Action, "none"), item.HandoffCommand)
		if refs := strings.Join(item.OutputRefs, ","); refs != "" {
			fmt.Fprintf(out, "  - outputRefs: %s\n", refs)
		}
		if refs := strings.Join(item.EvidenceRefs, ","); refs != "" {
			fmt.Fprintf(out, "  - evidenceRefs: %s\n", refs)
		}
		fmt.Fprintf(out, "  - review command: `%s`\n", item.ReviewCommand)
		fmt.Fprintf(out, "  - commander: state=%s primary=`%s`\n", item.MissionCommanderAction.State, item.MissionCommanderAction.PrimaryCommand)
		writeActionIndexList(out, "commander follow-up", item.MissionCommanderAction.FollowUpCommands)
		writeActionIndexList(out, "boundary", item.Boundary)
	}
	fmt.Fprintln(out)
}

func writeActionIndexList(out *bytes.Buffer, label string, items []string) {
	if len(items) == 0 {
		fmt.Fprintf(out, "  - %s: none\n", label)
		return
	}
	fmt.Fprintf(out, "  - %s:\n", label)
	for _, item := range items {
		fmt.Fprintf(out, "    - %s\n", item)
	}
}

func writeBriefList(out *bytes.Buffer, label string, items []string) {
	if len(items) == 0 {
		fmt.Fprintf(out, "- %s: none\n", label)
		return
	}
	fmt.Fprintf(out, "- %s:\n", label)
	for _, item := range items {
		fmt.Fprintf(out, "  - %s\n", item)
	}
}

func newEventSection(items []event) EventSection {
	shown := lastEvents(items, maxRows)
	return EventSection{Total: len(items), Shown: len(shown), Events: cloneEvents(shown)}
}

func openStatusEvents(items []event) []event {
	terminal := map[string]bool{"confirmed": true, "accepted": true, "rejected": true, "superseded": true, "resolved": true}
	open := []event{}
	for _, item := range items {
		status := strings.ToLower(strings.TrimSpace(stringValue(item, "status")))
		if status == "" || !terminal[status] {
			open = append(open, item)
		}
	}
	return open
}

func newBatchSection(events []event) BatchSection {
	batches := batchSummaries(events)
	shown := batches
	if len(shown) > maxRows {
		shown = shown[len(shown)-maxRows:]
	}
	out := make([]BatchSummary, 0, len(shown))
	for _, batch := range shown {
		out = append(out, BatchSummary{ID: batch.id, Events: len(batch.events), Kinds: kindCounts(batch.events), Last: batch.lastTime})
	}
	return BatchSection{Total: len(batches), Shown: len(shown), Batches: out}
}

func batchSummaries(events []event) []batchSummary {
	byID := map[string]*batchSummary{}
	order := []*batchSummary{}
	for idx, e := range events {
		id := stringValue(e, "batchId")
		if id == "" {
			continue
		}
		b := byID[id]
		if b == nil {
			b = &batchSummary{id: id}
			byID[id] = b
			order = append(order, b)
		}
		b.events = append(b.events, e)
		b.lastIdx = idx
		b.lastTime = stringValue(e, "time")
		if b.lastTime == "" {
			b.lastTime = stringValue(e, "createdAt")
		}
	}
	sort.SliceStable(order, func(i, j int) bool {
		if order[i].lastTime == order[j].lastTime {
			return order[i].lastIdx < order[j].lastIdx
		}
		return order[i].lastTime < order[j].lastTime
	})
	out := make([]batchSummary, 0, len(order))
	for _, item := range order {
		out = append(out, *item)
	}
	return out
}

func kindCounts(events []event) map[string]int {
	counts := map[string]int{}
	for _, e := range events {
		kind := stringValue(e, "kind")
		if kind == "" {
			kind = "unknown"
		}
		counts[kind]++
	}
	return counts
}

func cloneEvents(items []event) []map[string]any {
	out := make([]map[string]any, 0, len(items))
	for _, item := range items {
		out = append(out, maps.Clone(map[string]any(item)))
	}
	return out
}

func overviewNextSteps(brief MissionBrief, evidenceReview []workstream.ExecutionEvidenceReviewItem) []string {
	blocked := len(brief.BlockedLanes) > 0 || len(brief.PendingGates) > 0 || len(brief.OpenDecisions) > 0 || len(brief.Interventions) > 0
	steps := append([]string{}, workstream.ExecutionEvidenceReviewNextSteps(evidenceReview, !blocked)...)
	if !workstream.ExecutionEvidenceReviewNeedsMainReview(evidenceReview) {
		steps = append(steps, brief.NextAgentActions...)
	}
	steps = append(steps, "/rekit start <name>", "/rekit handoff", "/rekit handoff main 或 /rekit handoff <name>")
	return uniqueStrings(steps)
}

func uniqueStrings(items []string) []string {
	seen := map[string]bool{}
	out := []string{}
	for _, item := range items {
		item = strings.TrimSpace(item)
		if item == "" || seen[item] {
			continue
		}
		seen[item] = true
		out = append(out, item)
	}
	return out
}

func readFacts(caseRoot string) (factSet, error) {
	return mission.ReadStrictLedgerFacts(caseRoot)
}

func writeOpenCandidates(out *bytes.Buffer, candidates []event) {
	terminal := map[string]bool{"confirmed": true, "accepted": true, "rejected": true, "superseded": true, "resolved": true}
	open := []event{}
	counts := map[string]int{}
	for _, c := range candidates {
		status := strings.ToLower(strings.TrimSpace(stringValue(c, "status")))
		if status == "" || !terminal[status] {
			open = append(open, c)
			counts[stringValue(c, "subject")]++
		}
	}
	if len(open) == 0 {
		return
	}
	fmt.Fprintln(out, "未决 candidate：")
	shown := lastEvents(open, maxRows)
	for _, c := range shown {
		mark := ""
		if counts[stringValue(c, "subject")] > 1 {
			mark = " [冲突]"
		}
		fmt.Fprintf(out, "- %s | %s | confidence=%s%s\n", stringValue(c, "subject"), stringValue(c, "summary"), stringValue(c, "confidence"), mark)
	}
	if rest := len(open) - len(shown); rest > 0 {
		fmt.Fprintf(out, "- 另有 %d 条未决 candidate\n", rest)
	}
	fmt.Fprintln(out)
}

func writePendingGates(out *bytes.Buffer, requests []event) {
	pending := []event{}
	for _, r := range requests {
		if mission.IsPendingGateRequest(r) {
			pending = append(pending, r)
		}
	}
	if len(pending) == 0 {
		return
	}
	fmt.Fprintln(out, "pending-gate（heavy-tool 待确认）：")
	shown := lastEvents(pending, maxRows)
	for _, g := range shown {
		fmt.Fprintf(out, "- %s | %s%s\n", stringValue(g, "subject"), stringValue(g, "summary"), gateDetail(g, true, false))
	}
	if rest := len(pending) - len(shown); rest > 0 {
		fmt.Fprintf(out, "- 另有 %d 条 pending-gate\n", rest)
	}
	fmt.Fprintln(out)
}

func writeAuthorizedGates(out *bytes.Buffer, requests []event) {
	authorized := []event{}
	for _, r := range requests {
		if mission.IsAuthorizedGateRequest(r) {
			authorized = append(authorized, r)
		}
	}
	if len(authorized) == 0 {
		return
	}
	fmt.Fprintln(out, "authorized-gate（durable autonomy 已授权，非阻塞）：")
	shown := lastEvents(authorized, maxRows)
	for _, g := range shown {
		fmt.Fprintf(out, "- %s | %s%s\n", stringValue(g, "subject"), stringValue(g, "summary"), gateDetail(g, true, false))
	}
	if rest := len(authorized) - len(shown); rest > 0 {
		fmt.Fprintf(out, "- 另有 %d 条 authorized-gate\n", rest)
	}
	fmt.Fprintln(out)
}

func writeVerifications(out *bytes.Buffer, verifications []event) {
	if len(verifications) == 0 {
		return
	}
	fmt.Fprintln(out, "最近 verification：")
	shown := lastEvents(verifications, maxRows)
	for _, v := range shown {
		subject := stringValue(v, "subject")
		if strings.TrimSpace(subject) == "" {
			subject = stringValue(v, "kind")
		}
		by := ""
		if actor := stringValue(v, "actor"); strings.TrimSpace(actor) != "" {
			by = " | by=" + actor
		}
		fmt.Fprintf(out, "- %s | lane=%s | verifier=%s | verdict=%s | target=%s%s%s\n", subject, stringValue(v, "lane"), stringValue(v, "verifier"), stringValue(v, "verdict"), stringValue(v, "target"), by, batchTag(v))
	}
	if rest := len(verifications) - len(shown); rest > 0 {
		fmt.Fprintf(out, "- 另有 %d 条 verification\n", rest)
	}
	fmt.Fprintln(out)
}

func writeDecisions(out *bytes.Buffer, decisions []event) {
	if len(decisions) == 0 {
		return
	}
	fmt.Fprintln(out, "最近 decision：")
	shown := lastEvents(decisions, maxRows)
	for _, d := range shown {
		subject := stringValue(d, "subject")
		if strings.TrimSpace(subject) == "" {
			subject = stringValue(d, "kind")
		}
		decision := stringValue(d, "decision")
		if strings.TrimSpace(decision) == "" {
			decision = stringValue(d, "action")
		}
		actor := stringValue(d, "actor")
		if strings.TrimSpace(actor) == "" {
			actor = stringValue(d, "confirmedBy")
		}
		extra := ""
		if strings.TrimSpace(actor) != "" {
			extra = " | by=" + actor
		}
		fmt.Fprintf(out, "- %s | lane=%s | decision=%s%s | reason=%s\n", subject, stringValue(d, "lane"), decision, extra, stringValue(d, "reason"))
	}
	if rest := len(decisions) - len(shown); rest > 0 {
		fmt.Fprintf(out, "- 另有 %d 条 decision\n", rest)
	}
	fmt.Fprintln(out)
}

func writeBatches(out *bytes.Buffer, events []event) {
	if len(events) == 0 {
		return
	}
	type batch struct {
		id       string
		events   []event
		lastTime string
		lastIdx  int
	}
	byID := map[string]*batch{}
	order := []*batch{}
	for idx, e := range events {
		id := stringValue(e, "batchId")
		if id == "" {
			continue
		}
		b := byID[id]
		if b == nil {
			b = &batch{id: id}
			byID[id] = b
			order = append(order, b)
		}
		b.events = append(b.events, e)
		b.lastIdx = idx
		b.lastTime = stringValue(e, "time")
		if b.lastTime == "" {
			b.lastTime = stringValue(e, "createdAt")
		}
	}
	sort.SliceStable(order, func(i, j int) bool {
		if order[i].lastTime == order[j].lastTime {
			return order[i].lastIdx < order[j].lastIdx
		}
		return order[i].lastTime < order[j].lastTime
	})
	shown := order
	if len(shown) > maxRows {
		shown = shown[len(shown)-maxRows:]
	}
	fmt.Fprintln(out, "最近 batch：")
	for _, b := range shown {
		fmt.Fprintf(out, "- %s | events=%d | kinds=%s | last=%s\n", b.id, len(b.events), kindSummary(b.events), b.lastTime)
	}
	if rest := len(order) - len(shown); rest > 0 {
		fmt.Fprintf(out, "- 另有 %d 个 batch\n", rest)
	}
	fmt.Fprintln(out)
}

func writeInterventions(out *bytes.Buffer, interventions []event) {
	terminal := map[string]bool{"confirmed": true, "accepted": true, "rejected": true, "superseded": true, "resolved": true}
	open := []event{}
	for _, i := range interventions {
		status := strings.ToLower(strings.TrimSpace(stringValue(i, "status")))
		if status == "" || !terminal[status] {
			open = append(open, i)
		}
	}
	if len(open) > 0 {
		fmt.Fprintln(out, "未解决 intervention：")
		shown := lastEvents(open, maxRows)
		for _, i := range shown {
			subject := stringValue(i, "subject")
			if strings.TrimSpace(subject) == "" {
				subject = stringValue(i, "action")
			}
			status := stringValue(i, "status")
			if strings.TrimSpace(status) == "" {
				status = "open"
			}
			batch := batchTag(i)
			fmt.Fprintf(out, "- %s | action=%s | target=%s | status=%s%s | summary=%s\n", subject, stringValue(i, "action"), stringValue(i, "target"), status, batch, stringValue(i, "summary"))
		}
		if rest := len(open) - len(shown); rest > 0 {
			fmt.Fprintf(out, "- 另有 %d 条未解决 intervention\n", rest)
		}
		fmt.Fprintln(out)
	}
	if len(interventions) == 0 {
		return
	}
	fmt.Fprintln(out, "最近 intervention：")
	shown := lastEvents(interventions, maxRows)
	for _, i := range shown {
		subject := stringValue(i, "subject")
		if strings.TrimSpace(subject) == "" {
			subject = stringValue(i, "action")
		}
		fmt.Fprintf(out, "- %s | action=%s | target=%s | approvedBy=%s | scope=%s%s\n", subject, stringValue(i, "action"), stringValue(i, "target"), stringValue(i, "approvedBy"), stringValue(i, "scope"), batchTag(i))
	}
	if rest := len(interventions) - len(shown); rest > 0 {
		fmt.Fprintf(out, "- 另有 %d 条 intervention\n", rest)
	}
	fmt.Fprintln(out)
}

func writeRollbacks(out *bytes.Buffer, rollbacks []event) {
	if len(rollbacks) == 0 {
		return
	}
	fmt.Fprintln(out, "最近 rollback：")
	shown := lastEvents(rollbacks, maxRows)
	for _, r := range shown {
		subject := stringValue(r, "subject")
		if strings.TrimSpace(subject) == "" {
			subject = stringValue(r, "kind")
		}
		fmt.Fprintf(out, "- %s | target=%s | status=%s%s | reason=%s\n", subject, stringValue(r, "target"), stringValue(r, "status"), batchTag(r), stringValue(r, "reason"))
	}
	if rest := len(rollbacks) - len(shown); rest > 0 {
		fmt.Fprintf(out, "- 另有 %d 条 rollback\n", rest)
	}
	fmt.Fprintln(out)
}

func writeNextSteps(out *bytes.Buffer, steps []string) {
	fmt.Fprintln(out, "建议下一步：")
	for _, step := range steps {
		fmt.Fprintf(out, "- %s\n", step)
	}
}

func boardEvent(board mission.Board) event {
	return event{
		"schemaVersion":        board.SchemaVersion,
		"caseRoot":             board.CaseRoot,
		"repoRoot":             board.RepoRoot,
		"pack":                 board.Pack,
		"automationMode":       board.AutomationMode,
		"defaultAuthorityLane": board.DefaultAuthorityLane,
		"factsRoot":            board.FactsRoot,
		"updatedAt":            board.UpdatedAt,
	}
}

func boardLaneEvents(lanes []mission.BoardLane) []event {
	out := make([]event, 0, len(lanes))
	for _, lane := range lanes {
		out = append(out, event{
			"id":                 lane.ID,
			"type":               lane.Type,
			"title":              lane.Title,
			"status":             lane.Status,
			"authority":          lane.Authority,
			"workspace":          lane.Workspace,
			"currentExecutor":    lane.CurrentExecutor,
			"executorGeneration": lane.ExecutorGeneration,
			"lastTakeoverAt":     lane.LastTakeoverAt,
			"lastTakeoverBy":     lane.LastTakeoverBy,
			"lastTakeoverReason": lane.LastTakeoverReason,
			"updatedAt":          lane.UpdatedAt,
		})
	}
	return out
}

func lastEvents(items []event, n int) []event {
	if len(items) <= n {
		return items
	}
	return items[len(items)-n:]
}

func kindSummary(events []event) string {
	counts := map[string]int{}
	keys := []string{}
	for _, e := range events {
		kind := stringValue(e, "kind")
		if kind == "" {
			kind = "unknown"
		}
		if _, ok := counts[kind]; !ok {
			keys = append(keys, kind)
		}
		counts[kind]++
	}
	sort.Strings(keys)
	parts := []string{}
	for _, key := range keys {
		parts = append(parts, fmt.Sprintf("%s=%d", key, counts[key]))
	}
	return strings.Join(parts, ",")
}

func gateDetail(e event, omitStatus, omitBatch bool) string {
	parts := []string{}
	add := func(key, value string) {
		if strings.TrimSpace(value) != "" {
			parts = append(parts, key+"="+value)
		}
	}
	if !omitStatus {
		add("status", stringValue(e, "status"))
	}
	add("by", stringValue(e, "actor"))
	add("risk", stringValue(e, "risk"))
	add("target", stringValue(e, "target"))
	if !omitBatch {
		add("batch", stringValue(e, "batchId"))
	}
	if gate, ok := e["gate"].(map[string]any); ok {
		add("action", stringValue(gate, "action"))
		add("scope", stringValue(gate, "scope"))
		add("budget", stringValue(gate, "budget"))
		add("tried", stringValue(gate, "triedLightSteps"))
		add("stop", stringValue(gate, "stopConditions"))
		if auth, ok := gate["authorization"].(map[string]any); ok {
			add("auth", stringValue(auth, "decision"))
			add("profile", stringValue(auth, "profileId"))
		}
	}
	if len(parts) == 0 {
		return ""
	}
	return " | " + strings.Join(parts, " | ")
}

func batchTag(e event) string {
	batch := stringValue(e, "batchId")
	if strings.TrimSpace(batch) == "" {
		return ""
	}
	return " | batch=" + batch
}

func workstreamLabel(lane event) string {
	id := stringValue(lane, "id")
	return mission.BoardLaneLabel(mission.BoardLane{ID: id, Authority: boolValue(lane, "authority")})
}

func stringValue(m map[string]any, key string) string {
	v, ok := m[key]
	if !ok || v == nil {
		return ""
	}
	switch t := v.(type) {
	case string:
		return t
	case []any:
		parts := []string{}
		for _, item := range t {
			text := strings.TrimSpace(fmt.Sprint(item))
			if text != "" {
				parts = append(parts, text)
			}
		}
		return strings.Join(parts, ",")
	case bool:
		if t {
			return "true"
		}
		return "false"
	default:
		return strings.TrimSpace(fmt.Sprint(t))
	}
}

func boolValue(m map[string]any, key string) bool {
	v, ok := m[key]
	if !ok || v == nil {
		return false
	}
	switch t := v.(type) {
	case bool:
		return t
	case string:
		return strings.EqualFold(strings.TrimSpace(t), "true")
	default:
		return false
	}
}

func intValue(m map[string]any, key string) int {
	v, ok := m[key]
	if !ok || v == nil {
		return 0
	}
	switch t := v.(type) {
	case int:
		return t
	case int64:
		return int(t)
	case float64:
		return int(t)
	case json.Number:
		value, err := t.Int64()
		if err == nil {
			return int(value)
		}
	case string:
		value, err := strconv.Atoi(strings.TrimSpace(t))
		if err == nil {
			return value
		}
	}
	return 0
}

func firstText(values ...string) string {
	for _, value := range values {
		value = strings.TrimSpace(value)
		if value != "" {
			return value
		}
	}
	return ""
}
