package mission

import "strings"

type InterventionProjection struct {
	Open     []map[string]any
	Resolved map[string]map[string]any
}

func EffectiveInterventions(items []map[string]any) InterventionProjection {
	resolved := map[string]map[string]any{}
	for _, item := range items {
		status := strings.ToLower(strings.TrimSpace(Value(item, "status")))
		if status != "resolved" && status != "superseded" && status != "accepted" && status != "confirmed" {
			continue
		}
		resolvesEventID := strings.TrimSpace(Value(item, "resolvesEventId"))
		if resolvesEventID == "" {
			continue
		}
		resolved[resolvesEventID] = item
	}
	open := []map[string]any{}
	for _, item := range OpenEvents(items) {
		id := strings.TrimSpace(Value(item, "eventId"))
		if id != "" {
			if _, ok := resolved[id]; ok {
				continue
			}
		}
		open = append(open, item)
	}
	return InterventionProjection{Open: open, Resolved: resolved}
}

func EffectiveOpenInterventions(items []map[string]any) []map[string]any {
	return EffectiveInterventions(items).Open
}

func EffectiveOpenLaneInterventions(facts Facts, laneID string) []map[string]any {
	return EffectiveOpenInterventions(FilterLane(facts.Interventions, laneID, ""))
}
