package workstream

import (
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"

	"github.com/shuiyu486/re-context-kits/internal/rekit/defaults"
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

func TestLaneExecutorActionBindsContinueCommandsToCurrentOwner(t *testing.T) {
	lane := Lane{ID: "feature-login", Name: "login", Status: "open", CurrentExecutor: "session two", ExecutorGeneration: 2}
	facts := mission.Facts{}
	brief := mission.BuildWithOptions([]mission.Lane{{ID: lane.ID, Label: "login", Status: lane.Status}}, facts, mission.BuildOptions{MaxRows: 5})
	action := laneExecutorActionFor(lane, facts, brief)
	want := `/rekit continue login -Executor "session two" -ExpectedExecutorGeneration 2`
	if action.ResumeCommand != want || action.MissionCommanderAction.PrimaryCommand != want {
		t.Fatalf("current owner was not bound into executable continue commands: %+v", action)
	}
	if !slices.Equal(action.NextAgentActions, []string{want}) {
		t.Fatalf("next actions did not use current owner: %+v", action.NextAgentActions)
	}
}

func TestBindLaneContinueCommandsTakeoverReplacesStaleDurableCommand(t *testing.T) {
	stale := laneExecutorAction{
		ResumeCommand:    "/rekit continue login -Executor executor-one -ExpectedExecutorGeneration 1",
		NextAgentActions: []string{"/rekit continue login -Executor executor-one -ExpectedExecutorGeneration 1"},
		MissionCommanderAction: mission.MissionCommanderAction{
			PrimaryCommand:   "/rekit continue login -Executor executor-one -ExpectedExecutorGeneration 1",
			FollowUpCommands: []string{"/rekit continue login -Executor executor-one -ExpectedExecutorGeneration 1 -WhatIf"},
		},
	}
	lane := Lane{ID: "feature-login", Name: "login", Status: "open", CurrentExecutor: "executor-two", ExecutorGeneration: 2}
	bound := bindLaneContinueCommands(stale, lane)
	want := "/rekit continue login -Executor executor-two -ExpectedExecutorGeneration 2"
	if bound.ResumeCommand != want || bound.NextAgentActions[0] != want || bound.MissionCommanderAction.PrimaryCommand != want || bound.MissionCommanderAction.FollowUpCommands[0] != want+" -WhatIf" {
		t.Fatalf("stale durable handoff command survived current authority rebinding: %+v", bound)
	}
}

func TestTakeoverRefreshesDurableResumeCheckpointHandoffAndDigestCommands(t *testing.T) {
	repoRoot, caseRoot := setupContinueCase(t, "executor-one")
	before, err := HandoffApply(repoRoot, caseRoot, defaults.DefaultPack, HandoffOptions{Selector: "devirt-main"})
	if err != nil {
		t.Fatal(err)
	}
	if before.ExecutorAction == nil || !strings.Contains(before.ExecutorAction.ResumeCommand, "-Executor executor-one -ExpectedExecutorGeneration 1") {
		t.Fatalf("initial handoff did not bind first owner: %+v", before.ExecutorAction)
	}
	if _, err := StartApply(repoRoot, caseRoot, defaults.DefaultPack, StartOptions{Selector: "devirt-main", Executor: "executor-two", Actor: "main-agent", TakeoverReason: "replacement"}); err != nil {
		t.Fatal(err)
	}
	preview, err := HandoffPreview(repoRoot, caseRoot, defaults.DefaultPack, HandoffOptions{Selector: "devirt-main"})
	if err != nil {
		t.Fatal(err)
	}
	want := "/rekit continue main -Executor executor-two -ExpectedExecutorGeneration 2"
	if preview.ExecutorAction == nil || preview.ExecutorAction.ResumeCommand != want {
		t.Fatalf("live handoff did not switch to current owner: %+v", preview.ExecutorAction)
	}
	applied, err := ContinueApply(repoRoot, caseRoot, defaults.DefaultPack, ContinueOptions{Selector: "devirt-main", Executor: "executor-two", ExpectedExecutorGeneration: 2})
	if err != nil {
		t.Fatal(err)
	}
	paths := []string{
		filepath.Join(caseRoot, ".rekit", "lanes", "devirt-main", "prompts", "RESUME.md"),
		filepath.Join(caseRoot, ".rekit", "lanes", "devirt-main", "checkpoints", "latest.json"),
		filepath.Join(caseRoot, ".rekit", "runs", applied.RunID, "digest.md"),
	}
	for _, path := range paths {
		data, err := os.ReadFile(path)
		if err != nil {
			t.Fatal(err)
		}
		text := string(data)
		if !strings.Contains(text, want) || strings.Contains(text, "-Executor executor-one -ExpectedExecutorGeneration 1") {
			t.Fatalf("durable artifact did not use only current authority command %s:\n%s", path, text)
		}
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
