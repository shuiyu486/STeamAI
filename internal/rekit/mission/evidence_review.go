package mission

import (
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
		EventID:        firstObjectText(observation, "eventId"),
		GateEventID:    gateEventID,
		Subject:        firstObjectText(observation, "subject"),
		Summary:        firstObjectText(observation, "summary"),
		Status:         status,
		Action:         firstObjectText(gate, "action"),
		Target:         firstObjectText(observation, "target"),
		OutputRefs:     objectStringList(execution["outputRefs"]),
		EvidenceRefs:   objectStringList(observation["evidenceRefs"]),
		BoundaryHits:   boundaryHits,
		Escalation:     escalation,
		ReviewCommand:  "review outputRefs/evidenceRefs for gateEventId " + gateEventID,
		HandoffCommand: "/rekit handoff " + label,
		Boundary:       boundary,
	}
	item.MissionCommanderAction = ExecutionEvidenceReviewCommanderAction(item, label)
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
