package mission

import (
	"encoding/json"
	"fmt"
	"slices"
	"strings"
)

const DefaultMaxRows = 10

type Lane struct {
	ID     string
	Label  string
	Status string
}

type Facts struct {
	Candidates    []map[string]any
	Requests      []map[string]any
	Decisions     []map[string]any
	Interventions []map[string]any
}

func FactsWithEvent(facts Facts, kind string, event map[string]any) Facts {
	out := Facts{
		Candidates:    append([]map[string]any{}, facts.Candidates...),
		Requests:      append([]map[string]any{}, facts.Requests...),
		Decisions:     append([]map[string]any{}, facts.Decisions...),
		Interventions: append([]map[string]any{}, facts.Interventions...),
	}
	switch strings.ToLower(strings.TrimSpace(kind)) {
	case "candidate":
		out.Candidates = append(out.Candidates, event)
	case "request":
		out.Requests = append(out.Requests, event)
	case "decision":
		out.Decisions = append(out.Decisions, event)
	case "intervention":
		out.Interventions = append(out.Interventions, event)
	}
	return out
}

type BuildOptions struct {
	MaxRows            int
	OpenDecisionAction string
}

type Brief struct {
	Summary          string   `json:"summary"`
	ReadyLanes       []string `json:"readyLanes"`
	BlockedLanes     []string `json:"blockedLanes"`
	PendingGates     []string `json:"pendingGates"`
	AuthorizedGates  []string `json:"authorizedGates"`
	OpenDecisions    []string `json:"openDecisions"`
	Interventions    []string `json:"interventions"`
	NextAgentActions []string `json:"nextAgentActions"`
	Escalations      []string `json:"escalations"`
}

type ExecutorAction struct {
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

type LaneExecutorActionSnapshot struct {
	Lane               string         `json:"lane"`
	Label              string         `json:"label"`
	Status             string         `json:"status"`
	Workspace          string         `json:"workspace,omitempty"`
	CurrentExecutor    string         `json:"currentExecutor,omitempty"`
	ExecutorGeneration int            `json:"executorGeneration,omitempty"`
	LastTakeoverAt     string         `json:"lastTakeoverAt,omitempty"`
	LastTakeoverBy     string         `json:"lastTakeoverBy,omitempty"`
	LastTakeoverReason string         `json:"lastTakeoverReason,omitempty"`
	ExecutorAction     ExecutorAction `json:"executorAction"`
}

func Build(lanes []Lane, facts Facts, maxRows int) Brief {
	return BuildWithOptions(lanes, facts, BuildOptions{MaxRows: maxRows})
}

func BuildWithOptions(lanes []Lane, facts Facts, opts BuildOptions) Brief {
	maxRows := opts.MaxRows
	if maxRows <= 0 {
		maxRows = DefaultMaxRows
	}
	open := OpenLanes(lanes)
	blocked := map[string][]string{}
	pendingGateLines := []string{}
	authorizedGateLines := []string{}
	for _, gate := range facts.Requests {
		if IsPendingGateRequest(gate) {
			lane := Value(gate, "lane")
			if lane != "" {
				blocked[lane] = append(blocked[lane], "pending-gate")
			}
			pendingGateLines = append(pendingGateLines, GateLine(gate))
			continue
		}
		if IsAuthorizedGateRequest(gate) {
			authorizedGateLines = append(authorizedGateLines, GateLine(gate))
		}
	}
	interventionLines := []string{}
	for _, item := range EffectiveOpenInterventions(facts.Interventions) {
		lane := Value(item, "lane")
		if lane != "" {
			blocked[lane] = append(blocked[lane], "intervention")
		}
		interventionLines = append(interventionLines, InterventionLine(item))
	}
	openDecisionCount := len(OpenCandidates(facts.Candidates)) + len(OpenDecisionEvents(facts.Decisions))
	openDecisions := OpenDecisionLines(facts)
	for _, lane := range OpenDecisionLanes(facts) {
		blocked[lane] = append(blocked[lane], "open-decision")
	}
	pendingGateCount := len(pendingGateLines)
	authorizedGateCount := len(authorizedGateLines)
	interventionCount := len(interventionLines)
	pendingGateLines = LimitStrings(pendingGateLines, maxRows)
	authorizedGateLines = LimitStrings(authorizedGateLines, maxRows)
	interventionLines = LimitStrings(interventionLines, maxRows)
	openDecisions = LimitStrings(openDecisions, maxRows)
	readyLanes := []string{}
	blockedLanes := []string{}
	for _, lane := range open {
		label := FirstText(lane.Label, lane.ID)
		if reasons := UniqueStrings(blocked[lane.ID]); len(reasons) > 0 {
			blockedLanes = append(blockedLanes, fmt.Sprintf("%s (%s)", label, strings.Join(reasons, ",")))
		} else {
			readyLanes = append(readyLanes, label)
		}
	}
	nextActions := NextActionsWithOptions(readyLanes, pendingGateLines, interventionLines, openDecisions, opts)
	escalations := Escalations(pendingGateLines, interventionLines, openDecisions)
	return Brief{
		Summary:          fmt.Sprintf("openLanes=%d ready=%d blocked=%d pendingGates=%d authorizedGates=%d openDecisions=%d interventions=%d", len(open), len(readyLanes), len(blockedLanes), pendingGateCount, authorizedGateCount, openDecisionCount, interventionCount),
		ReadyLanes:       readyLanes,
		BlockedLanes:     blockedLanes,
		PendingGates:     pendingGateLines,
		AuthorizedGates:  authorizedGateLines,
		OpenDecisions:    openDecisions,
		Interventions:    interventionLines,
		NextAgentActions: nextActions,
		Escalations:      escalations,
	}
}

func LaneExecutorActionSnapshots(lanes []BoardLane, facts Facts, brief Brief) []LaneExecutorActionSnapshot {
	items := make([]LaneExecutorActionSnapshot, 0, len(lanes))
	for _, lane := range lanes {
		label := BoardLaneLabel(lane)
		items = append(items, LaneExecutorActionSnapshot{
			Lane:               lane.ID,
			Label:              label,
			Status:             lane.Status,
			Workspace:          lane.Workspace,
			CurrentExecutor:    lane.CurrentExecutor,
			ExecutorGeneration: lane.ExecutorGeneration,
			LastTakeoverAt:     lane.LastTakeoverAt,
			LastTakeoverBy:     lane.LastTakeoverBy,
			LastTakeoverReason: lane.LastTakeoverReason,
			ExecutorAction:     LaneExecutorAction(Lane{ID: lane.ID, Label: label, Status: lane.Status}, facts, brief),
		})
	}
	return items
}

func LaneExecutorAction(lane Lane, facts Facts, brief Brief) ExecutorAction {
	label := laneCommandLabel(lane)
	laneFacts := LaneFacts(facts, lane.ID)
	pendingGates := len(FilterLane(laneFacts.Requests, lane.ID, "pending-gate"))
	openInterventions := len(EffectiveOpenInterventions(laneFacts.Interventions))
	openDecisions := len(OpenDecisionItems(laneFacts))
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
	ready := !blocked && slices.Contains(brief.ReadyLanes, label)
	return ExecutorAction{
		Blocked:              blocked,
		Ready:                ready,
		BlockerReasons:       reasons,
		PendingGates:         pendingGates,
		OpenInterventions:    openInterventions,
		OpenDecisions:        openDecisions,
		ReconcileRequired:    openInterventions > 0,
		PendingGateRequired:  pendingGates > 0,
		OpenDecisionRequired: openDecisions > 0,
		ResumeCommand:        "/rekit continue " + label,
		HandoffCommand:       "/rekit handoff " + label,
		NextAgentActions:     LaneExecutorNextActions(label, ready, pendingGates, openInterventions, openDecisions),
		Escalations:          LaneExecutorEscalations(pendingGates, openInterventions, openDecisions),
	}
}

func LaneExecutorNextActions(label string, ready bool, pendingGates, openInterventions, openDecisions int) []string {
	actions := []string{}
	if openInterventions > 0 {
		actions = append(actions, "reconcile open intervention(s) before continuing this lane")
	}
	if pendingGates > 0 {
		actions = append(actions, "resolve or keep deferred pending-gate request(s); gate records the request and never executes heavy-tool")
	}
	if openDecisions > 0 {
		actions = append(actions, "review open candidate/decision item(s) with evidence and authority boundary")
	}
	if len(actions) == 0 && ready {
		actions = append(actions, "/rekit continue "+strings.TrimSpace(label))
	}
	if len(actions) == 0 {
		actions = append(actions, "/rekit handoff "+strings.TrimSpace(label))
	}
	return actions
}

func LaneExecutorEscalations(pendingGates, openInterventions, openDecisions int) []string {
	escalations := []string{}
	if pendingGates > 0 {
		escalations = append(escalations, "pending-gate requires main-agent/user decision before heavy action")
	}
	if openInterventions > 0 {
		escalations = append(escalations, "open intervention must be reconciled into durable lane state")
	}
	if openDecisions > 0 {
		escalations = append(escalations, "authority/confirmed outcome remains deferred until explicitly approved")
	}
	return escalations
}

func laneCommandLabel(lane Lane) string {
	label := strings.TrimSpace(lane.Label)
	if label != "" {
		return label
	}
	if lane.ID == "main" {
		return "main"
	}
	if name, ok := strings.CutPrefix(lane.ID, "feature-"); ok {
		return name
	}
	return lane.ID
}

func OpenLanes(lanes []Lane) []Lane {
	open := []Lane{}
	for _, lane := range lanes {
		if isOpenLaneStatus(lane.Status) {
			open = append(open, lane)
		}
	}
	return open
}

func isOpenLaneStatus(status string) bool {
	status = strings.ToLower(strings.TrimSpace(status))
	return status != "archived" && status != "paused" && status != "closed"
}

func OpenEvents(items []map[string]any) []map[string]any {
	open := []map[string]any{}
	for _, item := range items {
		status := strings.ToLower(strings.TrimSpace(Value(item, "status")))
		if status == "" || !IsClosedStatus(status) {
			open = append(open, item)
		}
	}
	return open
}

func OpenCandidates(items []map[string]any) []map[string]any {
	open := []map[string]any{}
	for _, item := range items {
		status := strings.ToLower(strings.TrimSpace(Value(item, "status")))
		switch status {
		case "confirmed", "accepted", "rejected", "resolved", "superseded":
			continue
		default:
			open = append(open, item)
		}
	}
	return open
}

func OpenDecisionEvents(decisions []map[string]any) []map[string]any {
	open := []map[string]any{}
	for _, decision := range decisions {
		status := strings.ToLower(strings.TrimSpace(Value(decision, "status")))
		decisionValue := strings.ToLower(strings.TrimSpace(FirstText(Value(decision, "decision"), Value(decision, "action"))))
		if (status == "" && decisionValue == "defer") || (status != "" && !IsTerminalStatus(status)) || decisionValue == "pending-user" {
			open = append(open, decision)
		}
	}
	return open
}

func OpenDecisionLanes(facts Facts) []string {
	lanes := []string{}
	for _, candidate := range OpenCandidates(facts.Candidates) {
		if lane := Value(candidate, "lane"); lane != "" {
			lanes = append(lanes, lane)
		}
	}
	for _, decision := range OpenDecisionEvents(facts.Decisions) {
		if lane := Value(decision, "lane"); lane != "" {
			lanes = append(lanes, lane)
		}
	}
	return UniqueStrings(lanes)
}

func OpenDecisionItems(facts Facts) []map[string]any {
	items := []map[string]any{}
	items = append(items, OpenCandidates(facts.Candidates)...)
	items = append(items, OpenDecisionEvents(facts.Decisions)...)
	return items
}

func OpenDecisionLines(facts Facts) []string {
	lines := []string{}
	for _, item := range OpenDecisionItems(facts) {
		lines = append(lines, OpenDecisionLine(item))
	}
	return lines
}

func LaneFacts(facts Facts, laneID string) Facts {
	return Facts{
		Candidates:    FilterLane(facts.Candidates, laneID, ""),
		Requests:      FilterLane(facts.Requests, laneID, ""),
		Decisions:     FilterLane(facts.Decisions, laneID, ""),
		Interventions: FilterLane(facts.Interventions, laneID, ""),
	}
}

func FilterLane(items []map[string]any, laneID, status string) []map[string]any {
	out := []map[string]any{}
	for _, item := range items {
		if Value(item, "lane") != laneID {
			continue
		}
		if status != "" && Value(item, "status") != status {
			continue
		}
		out = append(out, item)
	}
	return out
}

func IsPendingGateRequest(item map[string]any) bool {
	return Value(item, "status") == "pending-gate"
}

func IsAuthorizedGateRequest(item map[string]any) bool {
	return Value(item, "status") == "authorized-gate"
}

func GateLine(item map[string]any) string {
	parts := []string{Subject(item)}
	AddPart(&parts, "lane", Value(item, "lane"))
	AddPart(&parts, "risk", Value(item, "risk"))
	AddPart(&parts, "target", Value(item, "target"))
	addGateParts(&parts, item)
	return strings.Join(parts, " | ")
}

func LaneGateLine(item map[string]any) string {
	parts := []string{Subject(item)}
	addGateParts(&parts, item)
	AddPart(&parts, "risk", Value(item, "risk"))
	AddPart(&parts, "target", Value(item, "target"))
	return strings.Join(parts, " | ")
}

func addGateParts(parts *[]string, item map[string]any) {
	gate, ok := item["gate"].(map[string]any)
	if !ok {
		return
	}
	AddPart(parts, "action", Value(gate, "action"))
	AddPart(parts, "scope", Value(gate, "scope"))
	AddPart(parts, "requestedBudget", budgetLine(gate["requestedBudget"]))
	AddPart(parts, "outputPaths", Value(gate, "outputPaths"))
	AddPart(parts, "stopConditions", Value(gate, "stopConditions"))
	if eventID := Value(item, "eventId"); eventID != "" && Value(item, "status") == "authorized-gate" {
		AddPart(parts, "eventId", eventID)
		AddPart(parts, "reportContract", "/rekit gate -ExecutionReportContract -GateEventId "+eventID+" -Format json")
	}
	if auth, ok := gate["authorization"].(map[string]any); ok {
		AddPart(parts, "auth", Value(auth, "decision"))
		AddPart(parts, "profile", Value(auth, "profileId"))
	}
}

func InterventionLine(item map[string]any) string {
	parts := []string{Subject(item)}
	AddPart(&parts, "lane", Value(item, "lane"))
	AddPart(&parts, "action", Value(item, "action"))
	AddPart(&parts, "status", FirstText(Value(item, "status"), "open"))
	AddPart(&parts, "target", Value(item, "target"))
	return strings.Join(parts, " | ")
}

func LaneInterventionLine(item map[string]any) string {
	parts := []string{Subject(item)}
	AddPart(&parts, "action", Value(item, "action"))
	AddPart(&parts, "status", FirstText(Value(item, "status"), "open"))
	AddPart(&parts, "target", Value(item, "target"))
	return strings.Join(parts, " | ")
}

func CandidateLine(item map[string]any) string {
	parts := []string{"candidate: " + Subject(item)}
	AddPart(&parts, "lane", Value(item, "lane"))
	AddPart(&parts, "status", Value(item, "status"))
	AddPart(&parts, "summary", Value(item, "summary"))
	return strings.Join(parts, " | ")
}

func LaneCandidateLine(item map[string]any) string {
	parts := []string{"candidate: " + Subject(item)}
	AddPart(&parts, "status", Value(item, "status"))
	AddPart(&parts, "summary", Value(item, "summary"))
	return strings.Join(parts, " | ")
}

func DecisionLine(item map[string]any) string {
	parts := []string{Subject(item)}
	AddPart(&parts, "lane", Value(item, "lane"))
	AddPart(&parts, "decision", FirstText(Value(item, "decision"), Value(item, "action")))
	AddPart(&parts, "reason", Value(item, "reason"))
	return strings.Join(parts, " | ")
}

func LaneDecisionLine(item map[string]any) string {
	parts := []string{Subject(item)}
	AddPart(&parts, "decision", FirstText(Value(item, "decision"), Value(item, "action")))
	AddPart(&parts, "reason", Value(item, "reason"))
	return strings.Join(parts, " | ")
}

func OpenDecisionLine(item map[string]any) string {
	if Value(item, "kind") == "candidate" {
		return CandidateLine(item)
	}
	return DecisionLine(item)
}

func LaneOpenDecisionLine(item map[string]any) string {
	if Value(item, "kind") == "candidate" {
		return LaneCandidateLine(item)
	}
	return LaneDecisionLine(item)
}

func NextActions(ready, gates, interventions, decisions []string, maxRows int) []string {
	return NextActionsWithOptions(ready, gates, interventions, decisions, BuildOptions{MaxRows: maxRows})
}

func NextActionsWithOptions(ready, gates, interventions, decisions []string, opts BuildOptions) []string {
	maxRows := opts.MaxRows
	if maxRows <= 0 {
		maxRows = DefaultMaxRows
	}
	actions := []string{}
	if len(interventions) > 0 {
		actions = append(actions, "reconcile open intervention(s) before continuing the affected lane")
	}
	if len(gates) > 0 {
		actions = append(actions, "resolve or keep deferred pending-gate request(s); gate records the request and never executes heavy-tool")
	}
	if len(decisions) > 0 {
		action := FirstText(opts.OpenDecisionAction, "review open candidates/decisions and record accept/reject/defer with evidence")
		actions = append(actions, action)
	}
	for _, lane := range ready {
		actions = append(actions, "/rekit continue "+lane)
	}
	if len(actions) == 0 {
		actions = append(actions, "/rekit start <name>", "/rekit handoff")
	}
	return LimitStrings(actions, maxRows)
}

func Escalations(gates, interventions, decisions []string) []string {
	escalations := []string{}
	if len(gates) > 0 {
		escalations = append(escalations, "pending-gate requires main-agent/user decision before heavy action")
	}
	if len(interventions) > 0 {
		escalations = append(escalations, "open intervention must be reconciled into durable lane state")
	}
	if len(decisions) > 0 {
		escalations = append(escalations, "authority/confirmed outcome remains deferred until explicitly approved")
	}
	return escalations
}

func Subject(item map[string]any) string {
	return FirstText(Value(item, "subject"), Value(item, "summary"), Value(item, "kind"), "item")
}

func budgetLine(value any) string {
	budget := map[string]any{}
	switch t := value.(type) {
	case map[string]any:
		budget = t
	default:
		data, err := json.Marshal(t)
		if err != nil {
			return ""
		}
		if err := json.Unmarshal(data, &budget); err != nil {
			return ""
		}
	}
	runtimeSeconds := Value(budget, "runtimeSeconds")
	diskMB := Value(budget, "diskMB")
	requests := Value(budget, "requests")
	if emptyBudgetValue(runtimeSeconds) && emptyBudgetValue(diskMB) && emptyBudgetValue(requests) {
		return ""
	}
	parts := []string{}
	AddPart(&parts, "runtimeSeconds", runtimeSeconds)
	AddPart(&parts, "diskMB", diskMB)
	AddPart(&parts, "requests", requests)
	return strings.Join(parts, ",")
}

func emptyBudgetValue(value string) bool {
	value = strings.TrimSpace(value)
	return value == "" || value == "0" || value == "0.0"
}

func AddPart(parts *[]string, key, value string) {
	if strings.TrimSpace(value) != "" {
		*parts = append(*parts, key+"="+strings.TrimSpace(value))
	}
}

func FirstText(values ...string) string {
	for _, value := range values {
		if strings.TrimSpace(value) != "" {
			return strings.TrimSpace(value)
		}
	}
	return ""
}

func Value(m map[string]any, key string) string {
	v, ok := m[key]
	if !ok || v == nil {
		return ""
	}
	switch t := v.(type) {
	case string:
		return t
	case []string:
		parts := []string{}
		for _, item := range t {
			text := strings.TrimSpace(item)
			if text != "" {
				parts = append(parts, text)
			}
		}
		return strings.Join(parts, ",")
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

func LimitStrings(items []string, n int) []string {
	if n <= 0 || len(items) <= n {
		return items
	}
	return items[len(items)-n:]
}

func UniqueStrings(items []string) []string {
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

func IsTerminalStatus(status string) bool {
	switch strings.ToLower(strings.TrimSpace(status)) {
	case "confirmed", "accepted", "rejected", "resolved", "deferred", "superseded":
		return true
	default:
		return false
	}
}

func IsClosedStatus(status string) bool {
	switch strings.ToLower(strings.TrimSpace(status)) {
	case "confirmed", "accepted", "rejected", "resolved", "superseded":
		return true
	default:
		return false
	}
}
