package workstream

import (
	"slices"
	"testing"

	"github.com/shuiyu486/re-context-kits/internal/rekit/mission"
)

func TestLaneExecutorActionUsesTypedLaneFactsForBlockers(t *testing.T) {
	lane := Lane{ID: "main", Authority: true, Status: "open"}
	facts := mission.Facts{
		Requests: []map[string]any{
			{"kind": "request", "lane": "main", "status": "pending-gate", "subject": "debug gate"},
			{"kind": "request", "lane": "main", "status": "authorized-gate", "subject": "authorized debug"},
		},
		Interventions: []map[string]any{
			{"eventId": "evt-open", "kind": "intervention", "lane": "main", "status": "open", "subject": "manual stop"},
			{"eventId": "evt-resolved", "kind": "intervention", "lane": "main", "status": "resolved", "resolvesEventId": "evt-ignored", "subject": "resolved other"},
		},
		Candidates: []map[string]any{
			{"kind": "candidate", "lane": "main", "status": "open", "subject": "review candidate"},
			{"kind": "candidate", "lane": "main", "status": "accepted", "subject": "accepted candidate"},
		},
	}
	brief := mission.BuildWithOptions([]mission.Lane{{ID: "main", Label: "renamed-main", Status: "open"}}, facts, mission.BuildOptions{MaxRows: 5})
	action := laneExecutorActionFor(lane, facts, brief)
	if !action.Blocked || action.Ready {
		t.Fatalf("expected typed blockers to mark lane blocked: %+v", action)
	}
	for _, reason := range []string{"pending-gate", "intervention", "open-decision"} {
		if !slices.Contains(action.BlockerReasons, reason) {
			t.Fatalf("missing blocker reason %q: %+v", reason, action)
		}
	}
	if action.PendingGates != 1 || action.OpenInterventions != 1 || action.OpenDecisions != 1 {
		t.Fatalf("unexpected typed blocker counts: %+v", action)
	}
	if !action.PendingGateRequired || !action.ReconcileRequired || !action.OpenDecisionRequired {
		t.Fatalf("unexpected executor requirements: %+v", action)
	}
	if action.ResumeCommand != "/rekit continue main" || action.HandoffCommand != "/rekit handoff main" {
		t.Fatalf("unexpected executor commands: %+v", action)
	}
}

func TestLaneExecutorActionReadyUsesMissionBriefReadyLane(t *testing.T) {
	lane := Lane{ID: "feature-login", Name: "login", Status: "open"}
	facts := mission.Facts{}
	brief := mission.BuildWithOptions([]mission.Lane{{ID: "feature-login", Label: "login", Status: "open"}}, facts, mission.BuildOptions{MaxRows: 5})
	action := laneExecutorActionFor(lane, facts, brief)
	if action.Blocked || !action.Ready || len(action.BlockerReasons) != 0 {
		t.Fatalf("expected ready lane executor action: %+v", action)
	}
	if action.ResumeCommand != "/rekit continue login" || action.HandoffCommand != "/rekit handoff login" {
		t.Fatalf("unexpected ready lane commands: %+v", action)
	}
}
