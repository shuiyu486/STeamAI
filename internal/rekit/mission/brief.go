package mission

import (
	"fmt"
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

type BuildOptions struct {
	MaxRows            int
	OpenDecisionAction string
}

type Brief struct {
	Summary          string   `json:"summary"`
	ReadyLanes       []string `json:"readyLanes"`
	BlockedLanes     []string `json:"blockedLanes"`
	PendingGates     []string `json:"pendingGates"`
	OpenDecisions    []string `json:"openDecisions"`
	Interventions    []string `json:"interventions"`
	NextAgentActions []string `json:"nextAgentActions"`
	Escalations      []string `json:"escalations"`
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
	for _, gate := range facts.Requests {
		if Value(gate, "status") != "pending-gate" {
			continue
		}
		lane := Value(gate, "lane")
		if lane != "" {
			blocked[lane] = append(blocked[lane], "pending-gate")
		}
		pendingGateLines = append(pendingGateLines, GateLine(gate))
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
	interventionCount := len(interventionLines)
	pendingGateLines = LimitStrings(pendingGateLines, maxRows)
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
		Summary:          fmt.Sprintf("openLanes=%d ready=%d blocked=%d pendingGates=%d openDecisions=%d interventions=%d", len(open), len(readyLanes), len(blockedLanes), pendingGateCount, openDecisionCount, interventionCount),
		ReadyLanes:       readyLanes,
		BlockedLanes:     blockedLanes,
		PendingGates:     pendingGateLines,
		OpenDecisions:    openDecisions,
		Interventions:    interventionLines,
		NextAgentActions: nextActions,
		Escalations:      escalations,
	}
}

func OpenLanes(lanes []Lane) []Lane {
	open := []Lane{}
	for _, lane := range lanes {
		status := strings.ToLower(strings.TrimSpace(lane.Status))
		if status != "archived" && status != "paused" && status != "closed" {
			open = append(open, lane)
		}
	}
	return open
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

func GateLine(item map[string]any) string {
	parts := []string{Subject(item)}
	AddPart(&parts, "lane", Value(item, "lane"))
	AddPart(&parts, "risk", Value(item, "risk"))
	AddPart(&parts, "target", Value(item, "target"))
	if gate, ok := item["gate"].(map[string]any); ok {
		AddPart(&parts, "action", Value(gate, "action"))
		AddPart(&parts, "scope", Value(gate, "scope"))
	}
	return strings.Join(parts, " | ")
}

func LaneGateLine(item map[string]any) string {
	parts := []string{Subject(item)}
	if gate, ok := item["gate"].(map[string]any); ok {
		AddPart(&parts, "action", Value(gate, "action"))
		AddPart(&parts, "scope", Value(gate, "scope"))
	}
	AddPart(&parts, "risk", Value(item, "risk"))
	AddPart(&parts, "target", Value(item, "target"))
	return strings.Join(parts, " | ")
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
