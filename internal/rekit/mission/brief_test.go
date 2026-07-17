package mission

import (
	"slices"
	"strings"
	"testing"
)

func TestBuildBlocksLanesAndUsesSharedDecisionAction(t *testing.T) {
	brief := BuildWithOptions(
		[]Lane{
			{ID: "main", Label: "main", Status: "active"},
			{ID: "feature-login", Label: "login", Status: "active"},
			{ID: "feature-closed", Label: "closed", Status: "closed"},
		},
		Facts{
			Requests: []map[string]any{
				{"kind": "request", "lane": "feature-login", "subject": "debug gate", "status": "pending-gate", "risk": "high", "gate": map[string]any{"action": "debug", "scope": "unit-test"}},
			},
			Interventions: []map[string]any{
				{"kind": "intervention", "lane": "feature-login", "subject": "manual override", "action": "pause", "status": "deferred"},
			},
			Candidates: []map[string]any{
				{"kind": "candidate", "lane": "feature-login", "subject": "candidate blocker", "status": "open", "summary": "awaiting review"},
			},
			Decisions: []map[string]any{
				{"kind": "decision", "lane": "feature-login", "subject": "merge deferred", "decision": "defer", "reason": "needs approval"},
				{"kind": "decision", "lane": "feature-login", "subject": "already deferred", "status": "deferred", "decision": "defer"},
			},
		},
		BuildOptions{MaxRows: 10, OpenDecisionAction: "review open candidate/decision item(s) with evidence and authority boundary"},
	)

	if brief.Summary != "openLanes=2 ready=1 blocked=1 pendingGates=1 openDecisions=2 interventions=1" {
		t.Fatalf("summary = %q", brief.Summary)
	}
	if !slices.Contains(brief.ReadyLanes, "main") || !slices.Contains(brief.BlockedLanes, "login (pending-gate,intervention,open-decision)") {
		t.Fatalf("unexpected lanes: ready=%v blocked=%v", brief.ReadyLanes, brief.BlockedLanes)
	}
	for _, checks := range []struct {
		name  string
		items []string
		want  string
	}{
		{name: "gate", items: brief.PendingGates, want: "action=debug"},
		{name: "intervention", items: brief.Interventions, want: "status=deferred"},
		{name: "candidate", items: brief.OpenDecisions, want: "candidate: candidate blocker"},
		{name: "decision", items: brief.OpenDecisions, want: "decision=defer"},
		{name: "action", items: brief.NextAgentActions, want: "review open candidate/decision item(s)"},
		{name: "escalation", items: brief.Escalations, want: "authority/confirmed outcome remains deferred"},
	} {
		if !containsSubstring(checks.items, checks.want) {
			t.Fatalf("%s missing %q in %v", checks.name, checks.want, checks.items)
		}
	}
	if containsSubstring(brief.OpenDecisions, "already deferred") {
		t.Fatalf("terminal deferred decision should not remain open: %v", brief.OpenDecisions)
	}
}

func TestBuildDoesNotBlockOnAuthorizedGate(t *testing.T) {
	brief := Build(
		[]Lane{{ID: "main", Label: "main", Status: "active"}},
		Facts{Requests: []map[string]any{
			{"kind": "request", "lane": "main", "subject": "authorized debug", "status": "authorized-gate", "risk": "high", "gate": map[string]any{"action": "debug"}},
		}},
		10,
	)
	if brief.Summary != "openLanes=1 ready=1 blocked=0 pendingGates=0 openDecisions=0 interventions=0" {
		t.Fatalf("summary = %q", brief.Summary)
	}
	if len(brief.PendingGates) != 0 || !slices.Contains(brief.ReadyLanes, "main") {
		t.Fatalf("authorized gate should not block as pending gate: %+v", brief)
	}
}

func TestEffectiveOpenInterventionsHonorsAppendOnlyResolution(t *testing.T) {
	items := []map[string]any{
		{"eventId": "evt-open", "kind": "intervention", "lane": "feature-login", "subject": "manual pause", "status": "open"},
		{"eventId": "evt-deferred", "kind": "intervention", "lane": "feature-login", "subject": "manual override", "status": "deferred"},
		{"eventId": "evt-resolution", "kind": "intervention", "lane": "feature-login", "subject": "resolved pause", "status": "resolved", "resolvesEventId": "evt-open"},
		{"eventId": "evt-target-only", "kind": "intervention", "lane": "feature-login", "subject": "target is not lifecycle", "status": "resolved", "target": "evt-deferred"},
	}

	open := EffectiveOpenInterventions(items)
	if len(open) != 1 || Value(open[0], "eventId") != "evt-deferred" {
		t.Fatalf("effective open interventions = %+v", open)
	}
	projection := EffectiveInterventions(items)
	if projection.Resolved["evt-open"] == nil || projection.Resolved["evt-deferred"] != nil {
		t.Fatalf("unexpected resolution projection: %+v", projection.Resolved)
	}
}

func TestLaneOpenDecisionLineOmitsLaneField(t *testing.T) {
	facts := LaneFacts(Facts{
		Candidates: []map[string]any{{"kind": "candidate", "lane": "feature-login", "subject": "candidate blocker", "status": "open"}},
		Decisions:  []map[string]any{{"kind": "decision", "lane": "feature-login", "subject": "merge deferred", "decision": "defer"}},
	}, "feature-login")
	items := OpenDecisionItems(facts)
	if len(items) != 2 {
		t.Fatalf("items = %v", items)
	}
	for _, item := range items {
		line := LaneOpenDecisionLine(item)
		if strings.Contains(line, "lane=feature-login") {
			t.Fatalf("lane-specific line should omit lane field: %q", line)
		}
	}
}

func containsSubstring(items []string, want string) bool {
	for _, item := range items {
		if strings.Contains(item, want) {
			return true
		}
	}
	return false
}
