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
