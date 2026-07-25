package mission

import (
	"encoding/json"
	"fmt"
	"strings"
)

func ExecutionEvidenceReviewItems(observations []map[string]any, laneID string, labelFor func(string) string, maxRows int) []ExecutionEvidenceReviewItem {
	items := []ExecutionEvidenceReviewItem{}
	for _, observation := range observations {
		item, ok := ExecutionEvidenceReviewItemFromObservation(observation, laneID, labelFor)
		if ok {
			items = append(items, item)
		}
	}
	if maxRows > 0 && len(items) > maxRows {
		return items[len(items)-maxRows:]
	}
	return items
}

func ExecutionEvidenceReviewItemFromObservation(observation map[string]any, laneID string, labelFor func(string) string) (ExecutionEvidenceReviewItem, bool) {
	if !strings.EqualFold(firstObjectText(observation, "kind"), "observation") {
		return ExecutionEvidenceReviewItem{}, false
	}
	lane := firstObjectText(observation, "lane")
	if laneID != "" && lane != laneID {
		return ExecutionEvidenceReviewItem{}, false
	}
	execution := objectMap(observation["execution"])
	gateEventID := firstObjectText(execution, "gateEventId")
	if gateEventID == "" {
		return ExecutionEvidenceReviewItem{}, false
	}
	authorization := firstObjectText(execution, "authorization")
	if authorization == "" {
		gate := objectMap(observation["gate"])
		authorization = firstObjectText(objectMap(gate["authorization"]), "decision")
	}
	if !strings.EqualFold(authorization, "preauthorized") {
		return ExecutionEvidenceReviewItem{}, false
	}
	label := lane
	if labelFor != nil {
		label = labelFor(lane)
	}
	if label == "" {
		label = BoardLaneLabel(BoardLane{ID: lane})
	}
	if label == "" {
		label = "main"
	}
	status := firstObjectText(observation, "status")
	if status == "" {
		status = firstObjectText(execution, "status")
	}
	gate := objectMap(observation["gate"])
	boundary := []string{
		"observation evidence is already recorded; do not replay heavy tool",
		"review outputRefs/evidenceRefs before any authority/confirmed outcome",
		"no authority/confirmed writes",
	}
	boundaryHits := objectStringList(execution["boundaryHits"])
	escalation := firstObjectText(execution, "escalation")
	if status == "boundary-hit" || status == "escalated" || len(boundaryHits) > 0 || escalation != "" {
		boundary = append(boundary, "boundary/escalation requires main review before autonomous continuation")
	}
	item := ExecutionEvidenceReviewItem{
		EventID:               firstObjectText(observation, "eventId"),
		GateEventID:           gateEventID,
		Subject:               firstObjectText(observation, "subject"),
		Summary:               firstObjectText(observation, "summary"),
		Status:                status,
		Action:                firstObjectText(gate, "action"),
		Target:                firstObjectText(observation, "target"),
		OutputRefs:            objectStringList(execution["outputRefs"]),
		EvidenceRefs:          objectStringList(observation["evidenceRefs"]),
		ExecutionReportPath:   firstObjectText(execution, "executionReportPath"),
		ExecutionReportSHA256: firstObjectText(execution, "executionReportSha256"),
		ActualBudget:          executionEvidenceBudget(execution),
		AdapterID:             firstObjectText(objectMap(execution["adapter"]), "adapterId"),
		AdapterStatus:         firstObjectText(objectMap(execution["adapter"]), "status"),
		AdapterContext:        executionEvidenceAdapterContext(execution),
		BoundaryHits:          boundaryHits,
		Escalation:            escalation,
		ReviewCommand:         "review outputRefs/evidenceRefs for gateEventId " + gateEventID,
		HandoffCommand:        "/rekit handoff " + label,
		Boundary:              boundary,
	}
	item.MissionCommanderAction = ExecutionEvidenceReviewCommanderAction(item, label)
	item.FollowThrough = ExecutionEvidenceReviewFollowThrough(item)
	return item, true
}

func ExecutionEvidenceReviewCommanderAction(item ExecutionEvidenceReviewItem, label string) MissionCommanderAction {
	state := "ready-for-evidence-review"
	prompt := "authorized gate `" + item.GateEventID + "` 的 observation evidence 已记录；先 review output/evidence refs，再考虑任何 authority/confirmed outcome。"
	if ExecutionEvidenceReviewItemNeedsMainReview(item) {
		state = "needs-main-escalation"
		prompt = "authorized gate `" + item.GateEventID + "` 的 observation evidence 记录了 boundary/escalation；停止该 action 的自主推进并通知 main Agent。"
	}
	boundary := append([]string{}, item.Boundary...)
	if len(boundary) == 0 {
		boundary = []string{
			"observation evidence is already recorded; do not replay heavy tool",
			"review outputRefs/evidenceRefs before any authority/confirmed outcome",
			"no authority/confirmed writes",
		}
	}
	followUp := []string{"/rekit overview"}
	if label == "" {
		label = "main"
	}
	if !ExecutionEvidenceReviewItemNeedsMainReview(item) {
		followUp = append(followUp, "/rekit continue "+label+" -WhatIf")
	}
	return MissionCommanderAction{
		State:            state,
		Prompt:           prompt,
		PrimaryCommand:   item.HandoffCommand,
		FollowUpCommands: followUp,
		Boundary:         boundary,
	}
}

func ExecutionEvidenceReviewFollowThrough(item ExecutionEvidenceReviewItem) ExecutionEvidenceFollowThrough {
	state := strings.TrimSpace(item.MissionCommanderAction.State)
	if state == "" {
		state = "ready-for-evidence-review"
	}
	boundary := append([]string{}, item.MissionCommanderAction.Boundary...)
	if len(boundary) == 0 {
		boundary = append([]string{}, item.Boundary...)
	}
	if len(boundary) == 0 {
		boundary = []string{
			"observation evidence is already recorded; do not replay heavy tool",
			"review outputRefs/evidenceRefs before any authority/confirmed outcome",
			"no authority/confirmed writes",
		}
	}
	follow := ExecutionEvidenceFollowThrough{
		State:       state,
		GateEventID: item.GateEventID,
		Boundary:    boundary,
	}
	follow.Outcomes = []ExecutionEvidenceOutcome{executionEvidenceOutcomeFor(item, state, boundary)}
	follow.ActionQueue = MissionCommanderActionQueueFor(executionEvidenceFollowThroughActions(item, state, boundary))
	return follow
}

func executionEvidenceOutcomeFor(item ExecutionEvidenceReviewItem, state string, boundary []string) ExecutionEvidenceOutcome {
	name := "recorded-evidence-review"
	when := "bounded observation evidence was recorded for an authorized gate"
	expected := "reviewed outputRefs/evidenceRefs before any authority/confirmed outcome"
	actions := []string{
		"review outputRefs/evidenceRefs for gateEventId " + item.GateEventID,
		"run the evidence handoff command before continuing the lane",
	}
	verification := []string{item.HandoffCommand, "/rekit overview"}
	switch state {
	case "needs-main-escalation":
		name = "boundary-or-escalation-review"
		when = "recorded evidence reports boundaryHits, escalation, boundary-hit, or escalated status"
		expected = "main Agent reviews boundary/escalation before any autonomous continuation"
		actions = []string{
			"stop autonomous continuation for this action",
			"notify main Agent and review outputRefs/evidenceRefs for gateEventId " + item.GateEventID,
		}
		verification = []string{item.HandoffCommand, "/rekit overview"}
	case "evidence-already-recorded":
		name = "duplicate-record-review"
		when = "record command was replayed for an already recorded execution evidence eventId"
		expected = "duplicate replay does not append observation evidence; review the existing evidence only"
		actions = []string{
			"do not replay the heavy tool or append duplicate observation evidence",
			"review the already recorded outputRefs/evidenceRefs for gateEventId " + item.GateEventID,
		}
		verification = []string{item.HandoffCommand, "/rekit overview"}
	}
	return ExecutionEvidenceOutcome{
		Name:                 name,
		State:                state,
		When:                 when,
		Command:              item.HandoffCommand,
		Actions:              actions,
		VerificationCommands: compactStrings(verification),
		Expected:             expected,
		Evidence:             compactStrings(append(append([]string{}, item.OutputRefs...), item.EvidenceRefs...)),
		Boundary:             boundary,
	}
}

func executionEvidenceFollowThroughActions(item ExecutionEvidenceReviewItem, state string, boundary []string) []MissionCommanderNextActionItem {
	items := []MissionCommanderNextActionItem{}
	lane := evidenceReviewLane(item)
	label := evidenceReviewLabel(item)
	needsMainReview := state == "needs-main-escalation"
	if command := strings.TrimSpace(item.MissionCommanderAction.PrimaryCommand); command != "" {
		items = append(items, MissionCommanderNextActionItem{
			Lane:           lane,
			Label:          label,
			State:          state,
			Command:        command,
			Source:         "executionEvidenceReview.followThrough",
			Blocked:        needsMainReview,
			RequiresReview: true,
			Reasons:        executionEvidenceFollowThroughReasons(item, needsMainReview),
			Boundary:       append([]string{}, boundary...),
		})
	}
	for _, followUp := range item.MissionCommanderAction.FollowUpCommands {
		if strings.Contains(followUp, "/rekit continue") {
			continue
		}
		items = append(items, MissionCommanderNextActionItem{
			Lane:           lane,
			Label:          label,
			State:          state,
			Command:        followUp,
			Source:         "executionEvidenceReview.followThrough.followUp",
			Blocked:        needsMainReview,
			RequiresReview: true,
			Reasons:        executionEvidenceFollowThroughReasons(item, needsMainReview),
			Boundary:       append([]string{}, boundary...),
		})
	}
	return UniqueCommanderNextActions(items)
}

func executionEvidenceFollowThroughReasons(item ExecutionEvidenceReviewItem, needsMainReview bool) []string {
	reasons := []string{}
	if item.GateEventID != "" {
		reasons = append(reasons, "review execution evidence for gateEventId "+item.GateEventID)
	}
	if item.ReviewCommand != "" {
		reasons = append(reasons, item.ReviewCommand)
	}
	if needsMainReview {
		reasons = append(reasons, "boundary hit or escalation in execution evidence; stop autonomous continuation and notify main Agent")
	}
	return reasons
}

func compactStrings(items []string) []string {
	out := []string{}
	for _, item := range items {
		if text := strings.TrimSpace(item); text != "" {
			out = append(out, text)
		}
	}
	return UniqueStrings(out)
}

func executionEvidenceBudget(execution map[string]any) *ExecutionEvidenceBudget {
	budget := objectMap(execution["actualBudget"])
	if len(budget) == 0 {
		return nil
	}
	return &ExecutionEvidenceBudget{
		RuntimeSeconds: objectInt(budget["runtimeSeconds"]),
		DiskMB:         objectInt(budget["diskMB"]),
		Requests:       objectInt(budget["requests"]),
	}
}

func executionEvidenceAdapterContext(execution map[string]any) *ExecutionEvidenceAdapterContext {
	context := objectMap(execution["adapterContext"])
	if len(context) == 0 {
		return nil
	}
	candidate := ExecutionEvidenceAdapterContext{
		ID:                  firstObjectText(context, "id"),
		Status:              firstObjectText(context, "status"),
		Entry:               firstObjectText(context, "entry"),
		Purpose:             firstObjectText(context, "purpose"),
		SideEffects:         objectStringList(context["sideEffects"]),
		GateActions:         objectStringList(context["gateActions"]),
		ToolingCatalogPath:  firstObjectText(context, "toolingCatalogPath"),
		ReportGuidance:      objectStringList(context["reportGuidance"]),
		EvidenceGuidance:    objectStringList(context["evidenceGuidance"]),
		StopConditionHints:  objectStringList(context["stopConditionHints"]),
		RecordOnlyAfterGate: objectBool(context["recordOnlyAfterGate"]),
	}
	if candidate.ID == "" && candidate.Status == "" && candidate.Entry == "" && candidate.Purpose == "" && len(candidate.SideEffects) == 0 && len(candidate.GateActions) == 0 && candidate.ToolingCatalogPath == "" && len(candidate.ReportGuidance) == 0 && len(candidate.EvidenceGuidance) == 0 && len(candidate.StopConditionHints) == 0 && !candidate.RecordOnlyAfterGate {
		return nil
	}
	return &candidate
}

func objectMap(value any) map[string]any {
	if item, ok := value.(map[string]any); ok {
		return item
	}
	return nil
}

func objectStringList(value any) []string {
	switch t := value.(type) {
	case []string:
		return append([]string{}, t...)
	case []any:
		items := []string{}
		for _, item := range t {
			if text := objectText(item); text != "" {
				items = append(items, text)
			}
		}
		return items
	case string:
		text := strings.TrimSpace(t)
		if text == "" {
			return nil
		}
		items := []string{}
		for part := range strings.SplitSeq(text, ",") {
			if trimmed := strings.TrimSpace(part); trimmed != "" {
				items = append(items, trimmed)
			}
		}
		return items
	default:
		if text := objectText(value); text != "" {
			return []string{text}
		}
		return nil
	}
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

func objectInt(value any) int {
	switch t := value.(type) {
	case int:
		return t
	case int64:
		return int(t)
	case float64:
		return int(t)
	case json.Number:
		n, _ := t.Int64()
		return int(n)
	case string:
		var n int
		_, _ = fmt.Sscanf(strings.TrimSpace(t), "%d", &n)
		return n
	default:
		return 0
	}
}

func objectBool(value any) bool {
	switch t := value.(type) {
	case bool:
		return t
	case string:
		return strings.EqualFold(strings.TrimSpace(t), "true")
	default:
		return false
	}
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
