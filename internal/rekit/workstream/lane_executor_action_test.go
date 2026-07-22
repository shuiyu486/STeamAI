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
	if slices.Contains(action.NextAgentActions, "/rekit continue main") || len(action.NextAgentActions) != 3 {
		t.Fatalf("blocked lane should only recommend blocker resolution: %+v", action.NextAgentActions)
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
	if !slices.Equal(action.NextAgentActions, []string{"/rekit continue login"}) {
		t.Fatalf("ready lane should recommend only its own continue command: %+v", action.NextAgentActions)
	}
}

func TestStartMissionCommanderNextActionsRequireReviewBeforeApply(t *testing.T) {
	lane := Lane{ID: "feature-login", Name: "login", Status: "open"}
	action := laneExecutorAction{
		MissionCommanderAction: startApplyCommanderAction(lane, StartOptions{}, executorClaim{}),
	}
	items := startMissionCommanderNextActions(lane, action)
	if !hasStartCommanderNextAction(items, "missionCommanderActions", "/rekit start login -Apply", false, true) {
		t.Fatalf("start preview should expose review-owned apply action: %+v", items)
	}
	if !hasStartCommanderNextAction(items, "missionCommanderActions.followUp", "/rekit continue login", true, true) {
		t.Fatalf("start preview follow-up should remain blocked until apply succeeds: %+v", items)
	}
	if !hasStartCommanderNextAction(items, "missionCommanderActions.followUp", "/rekit handoff login", true, true) {
		t.Fatalf("start preview handoff follow-up should remain blocked until apply succeeds: %+v", items)
	}
}

func TestStartMissionCommanderNextActionsKeepBlockedLaneTakeoverApplyConsumable(t *testing.T) {
	lane := Lane{ID: "feature-login", Name: "login", Status: "open"}
	action := laneExecutorAction{
		Blocked:                true,
		MissionCommanderAction: startApplyCommanderAction(lane, StartOptions{}, executorClaim{}),
	}
	items := startMissionCommanderNextActions(lane, action)
	if !hasStartCommanderNextAction(items, "missionCommanderActions", "/rekit start login -Apply", false, true) {
		t.Fatalf("blocked lane start preview should keep bounded takeover apply consumable: %+v", items)
	}
	queue := mission.MissionCommanderActionQueueFor(append(items, mission.MissionCommanderNextActionItem{
		Lane:           "feature-login",
		GateEventID:    "gate-invalid",
		State:          "repair-adapter-report",
		Command:        "add-boundary-marker",
		Source:         "adapterReportValidation.repairHints",
		RequiresReview: true,
	}))
	if queue.CurrentAction == nil || queue.CurrentAction.State != "needs-start-apply" {
		t.Fatalf("start takeover apply should remain current before adapter repair: %+v", queue)
	}
}

func TestStartMissionCommanderNextActionsKeepReadyApplyConsumable(t *testing.T) {
	lane := Lane{ID: "feature-login", Name: "login", Status: "open"}
	action := laneExecutorAction{
		Ready: true,
		MissionCommanderAction: mission.MissionCommanderAction{
			State:            "ready-to-continue",
			PrimaryCommand:   "/rekit continue login",
			FollowUpCommands: []string{"/rekit handoff login"},
		},
	}
	items := startMissionCommanderNextActions(lane, action)
	if !hasStartCommanderNextAction(items, "missionCommanderActions", "/rekit continue login", false, false) {
		t.Fatalf("ready start apply should expose consumable continue action: %+v", items)
	}
	if !hasStartCommanderNextAction(items, "missionCommanderActions.followUp", "/rekit handoff login", false, false) {
		t.Fatalf("ready start apply should expose consumable handoff follow-up: %+v", items)
	}
}

func hasStartCommanderNextAction(items []mission.MissionCommanderNextActionItem, source, command string, blocked, requiresReview bool) bool {
	return slices.ContainsFunc(items, func(item mission.MissionCommanderNextActionItem) bool {
		return item.Source == source && item.Command == command && item.Blocked == blocked && item.RequiresReview == requiresReview
	})
}
