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

	if brief.Summary != "openLanes=2 ready=1 blocked=1 pendingGates=1 authorizedGates=0 openDecisions=2 interventions=1" {
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

func TestLaneExecutorActionUsesSharedTypedBlockerProjection(t *testing.T) {
	facts := Facts{
		Requests: []map[string]any{
			{"kind": "request", "lane": "feature-login", "subject": "debug gate", "status": "pending-gate", "risk": "high"},
			{"kind": "request", "lane": "feature-login", "subject": "authorized debug", "status": "authorized-gate", "risk": "high"},
		},
		Interventions: []map[string]any{
			{"eventId": "evt-open", "kind": "intervention", "lane": "feature-login", "subject": "manual stop", "status": "open"},
			{"eventId": "evt-resolved", "kind": "intervention", "lane": "feature-login", "status": "resolved", "resolvesEventId": "evt-ignored"},
		},
		Candidates: []map[string]any{
			{"kind": "candidate", "lane": "feature-login", "subject": "review candidate", "status": "open"},
			{"kind": "candidate", "lane": "feature-login", "subject": "accepted candidate", "status": "accepted"},
		},
	}
	brief := BuildWithOptions([]Lane{{ID: "feature-login", Label: "login", Status: "active"}}, facts, BuildOptions{MaxRows: 10})
	action := LaneExecutorAction(Lane{ID: "feature-login", Label: "login", Status: "active"}, facts, brief)
	if !action.Blocked || action.Ready || action.PendingGates != 1 || action.OpenInterventions != 1 || action.OpenDecisions != 1 {
		t.Fatalf("unexpected executor blockers: %+v", action)
	}
	for _, reason := range []string{"pending-gate", "intervention", "open-decision"} {
		if !slices.Contains(action.BlockerReasons, reason) {
			t.Fatalf("missing blocker reason %q: %+v", reason, action)
		}
	}
	if !action.PendingGateRequired || !action.ReconcileRequired || !action.OpenDecisionRequired || action.ResumeCommand != "/rekit continue login" || action.HandoffCommand != "/rekit handoff login" {
		t.Fatalf("unexpected executor requirements: %+v", action)
	}
	commander := action.MissionCommanderAction
	if commander.State != "needs-reconcile" || commander.PrimaryCommand != "/rekit reconcile login -InterventionId <eventId> -Apply" || !slices.Contains(commander.FollowUpCommands, "/rekit continue login -WhatIf") || !slices.Contains(commander.Boundary, "do not run continue for blocked lanes") {
		t.Fatalf("blocked executor should expose Mission Commander reconcile handoff: %+v", commander)
	}
}

func TestLaneExecutorActionPendingGateUsesCanonicalLaneForGateCommand(t *testing.T) {
	facts := Facts{
		Requests: []map[string]any{{"kind": "request", "lane": "feature-login", "subject": "debug gate", "status": "pending-gate", "risk": "high"}},
	}
	brief := BuildWithOptions([]Lane{{ID: "feature-login", Label: "login", Status: "active"}}, facts, BuildOptions{MaxRows: 10})
	action := LaneExecutorAction(Lane{ID: "feature-login", Label: "login", Status: "active"}, facts, brief)
	commander := action.MissionCommanderAction
	if commander.State != "needs-gate-decision" || commander.PrimaryCommand != "/rekit handoff login" {
		t.Fatalf("pending gate should expose gate decision handoff: %+v", commander)
	}
	if !slices.Contains(commander.FollowUpCommands, "/rekit gate <action> -Lane feature-login -Apply -Actor <actor>") || !slices.Contains(commander.FollowUpCommands, "/rekit continue login -WhatIf") {
		t.Fatalf("pending gate follow-up should use canonical gate lane and display continue selector: %+v", commander)
	}
}

func TestLaneExecutorActionReadyUsesSharedMissionBrief(t *testing.T) {
	facts := Facts{}
	brief := BuildWithOptions([]Lane{{ID: "feature-login", Label: "login", Status: "active"}}, facts, BuildOptions{MaxRows: 10})
	action := LaneExecutorAction(Lane{ID: "feature-login", Label: "login", Status: "active"}, facts, brief)
	if action.Blocked || !action.Ready || len(action.BlockerReasons) != 0 || action.ResumeCommand != "/rekit continue login" || action.HandoffCommand != "/rekit handoff login" {
		t.Fatalf("unexpected ready executor action: %+v", action)
	}
	commander := action.MissionCommanderAction
	if commander.State != "ready-to-continue" || commander.PrimaryCommand != "/rekit continue login" || !strings.Contains(commander.Prompt, "继续该 lane") || !slices.Contains(commander.FollowUpCommands, "/rekit handoff login") {
		t.Fatalf("ready executor should expose Mission Commander continue handoff: %+v", commander)
	}
}

func TestFactsWithEventCopiesAndRoutesBlockerKinds(t *testing.T) {
	base := Facts{Candidates: []map[string]any{{"kind": "candidate", "lane": "main", "subject": "existing"}}}
	candidate := map[string]any{"kind": "candidate", "lane": "main", "subject": "new"}
	withCandidate := FactsWithEvent(base, "candidate", candidate)
	if len(base.Candidates) != 1 || len(withCandidate.Candidates) != 2 || len(withCandidate.Requests) != 0 {
		t.Fatalf("candidate projection mutated base or routed incorrectly: base=%+v projected=%+v", base, withCandidate)
	}
	for kind, count := range map[string]int{"request": 1, "decision": 1, "intervention": 1, "observation": 0} {
		projected := FactsWithEvent(Facts{}, kind, map[string]any{"kind": kind, "lane": "main"})
		got := len(projected.Requests) + len(projected.Decisions) + len(projected.Interventions) + len(projected.Candidates)
		if got != count {
			t.Fatalf("kind %s routed count=%d, want %d: %+v", kind, got, count, projected)
		}
	}
}

func TestLaneExecutorActionSnapshotsKeepNextActionsLaneLocal(t *testing.T) {
	lanes := []BoardLane{{ID: "main", Status: "open", Authority: true}, {ID: "feature-login", Status: "open"}}
	facts := Facts{Candidates: []map[string]any{{"kind": "candidate", "lane": "feature-login", "subject": "login review", "status": "open"}}}
	brief := Build(BoardLanes(lanes), facts, 10)
	items := LaneExecutorActionSnapshots(lanes, facts, brief)
	if len(items) != 2 {
		t.Fatalf("lane action rows = %+v", items)
	}
	mainAction := items[0].ExecutorAction
	loginAction := items[1].ExecutorAction
	if !mainAction.Ready || mainAction.Blocked || !slices.Equal(mainAction.NextAgentActions, []string{"/rekit continue main"}) || len(mainAction.Escalations) != 0 {
		t.Fatalf("ready main lane action leaked project blockers: %+v", mainAction)
	}
	if !loginAction.Blocked || loginAction.Ready || !slices.Equal(loginAction.NextAgentActions, []string{"review open candidate/decision item(s) with evidence and authority boundary"}) || slices.Contains(loginAction.NextAgentActions, "/rekit continue main") || !slices.Equal(loginAction.Escalations, []string{"authority/confirmed outcome remains deferred until explicitly approved"}) {
		t.Fatalf("blocked login lane action leaked another lane recommendation: %+v", loginAction)
	}
	if mainAction.MissionCommanderAction.State != "ready-to-continue" || mainAction.MissionCommanderAction.PrimaryCommand != "/rekit continue main" {
		t.Fatalf("ready main lane action missing Mission Commander handoff: %+v", mainAction.MissionCommanderAction)
	}
	if loginAction.MissionCommanderAction.State != "needs-open-decision-review" || loginAction.MissionCommanderAction.PrimaryCommand != "/rekit handoff login" || !strings.Contains(loginAction.MissionCommanderAction.Prompt, "candidate/decision") {
		t.Fatalf("blocked login lane action missing Mission Commander decision review handoff: %+v", loginAction.MissionCommanderAction)
	}
}

func TestLaneExecutorActionSnapshotsProjectOpenLanes(t *testing.T) {
	lanes := []BoardLane{
		{ID: "main", Status: "open", Authority: true, Workspace: "workspace/main/main"},
		{ID: "feature-login", Status: "open", Workspace: "workspace/features/feature-login", CurrentExecutor: "session-login", ExecutorGeneration: 2, LastTakeoverAt: "2026-01-01T00:00:00Z", LastTakeoverBy: "main-agent", LastTakeoverReason: "replace stuck session"},
		{ID: "feature-paused", Status: "paused", Workspace: "workspace/features/feature-paused"},
	}
	facts := Facts{
		Requests: []map[string]any{
			{"kind": "request", "lane": "feature-login", "subject": "debug gate", "status": "pending-gate"},
			{"kind": "request", "lane": "feature-login", "subject": "authorized debug", "status": "authorized-gate"},
		},
		Interventions: []map[string]any{{"kind": "intervention", "lane": "feature-login", "subject": "manual stop", "status": "open"}},
		Candidates:    []map[string]any{{"kind": "candidate", "lane": "feature-login", "subject": "review candidate", "status": "open"}},
	}
	brief := Build(BoardLanes(lanes), facts, 10)
	items := LaneExecutorActionSnapshots(lanes, facts, brief)
	if len(items) != 3 || items[0].Lane != "main" || items[0].Label != "main" || !items[0].ExecutorAction.Ready || items[0].ExecutorAction.Blocked {
		t.Fatalf("unexpected main snapshot: %+v", items)
	}
	login := items[1]
	if login.Lane != "feature-login" || login.Label != "login" || login.Workspace != "workspace/features/feature-login" || login.CurrentExecutor != "session-login" || login.ExecutorGeneration != 2 || login.LastTakeoverAt != "2026-01-01T00:00:00Z" || login.LastTakeoverBy != "main-agent" || login.LastTakeoverReason != "replace stuck session" || !login.ExecutorAction.Blocked || login.ExecutorAction.Ready {
		t.Fatalf("unexpected login snapshot: %+v", login)
	}
	if login.ExecutorAction.PendingGates != 1 || login.ExecutorAction.OpenInterventions != 1 || login.ExecutorAction.OpenDecisions != 1 || !login.ExecutorAction.PendingGateRequired || !login.ExecutorAction.ReconcileRequired || !login.ExecutorAction.OpenDecisionRequired {
		t.Fatalf("unexpected login blockers: %+v", login.ExecutorAction)
	}
	if !slices.Equal(login.ExecutorAction.BlockerReasons, []string{"pending-gate", "intervention", "open-decision"}) {
		t.Fatalf("unexpected blocker order: %+v", login.ExecutorAction.BlockerReasons)
	}
	if items[2].Lane != "feature-paused" || items[2].ExecutorAction.Blocked || items[2].ExecutorAction.Ready {
		t.Fatalf("paused lane should remain visible but not ready: %+v", items[2])
	}
}

func TestBuildDoesNotBlockOnAuthorizedGate(t *testing.T) {
	brief := Build(
		[]Lane{{ID: "main", Label: "main", Status: "active"}},
		Facts{Requests: []map[string]any{
			{
				"eventId": "evt-authorized-main",
				"kind":    "request",
				"lane":    "main",
				"subject": "authorized debug",
				"status":  "authorized-gate",
				"risk":    "high",
				"target":  "sample.bin",
				"gate": map[string]any{
					"action":          "debug",
					"scope":           "single target",
					"requestedBudget": map[string]any{"runtimeSeconds": 120, "diskMB": 64, "requests": 1},
					"outputPaths":     []any{"workspace/main/debug/session-1"},
					"stopConditions":  []any{"timeout", "budget-exhausted"},
					"authorization":   map[string]any{"decision": "preauthorized", "profileId": "profile-main"},
				},
			},
		}},
		10,
	)
	if brief.Summary != "openLanes=1 ready=1 blocked=0 pendingGates=0 authorizedGates=1 openDecisions=0 interventions=0" {
		t.Fatalf("summary = %q", brief.Summary)
	}
	if len(brief.PendingGates) != 0 || !slices.Contains(brief.ReadyLanes, "main") {
		t.Fatalf("authorized gate should not block as pending gate: %+v", brief)
	}
	for _, want := range []string{
		"authorized debug",
		"action=debug",
		"scope=single target",
		"requestedBudget=runtimeSeconds=120,diskMB=64,requests=1",
		"outputPaths=workspace/main/debug/session-1",
		"stopConditions=timeout,budget-exhausted",
		"eventId=evt-authorized-main",
		"reportContract=/rekit gate -ExecutionReportContract -GateEventId evt-authorized-main -Format json",
		"auth=preauthorized",
		"profile=profile-main",
	} {
		if len(brief.AuthorizedGates) != 1 || !containsSubstring(brief.AuthorizedGates, want) {
			t.Fatalf("authorized gate should include %q boundary detail: %+v", want, brief)
		}
	}
}

func TestLaneGateLineIncludesAuthorizedExecutionBoundaries(t *testing.T) {
	line := LaneGateLine(map[string]any{
		"eventId": "evt-authorized-login",
		"kind":    "request",
		"lane":    "feature-login",
		"subject": "authorized debug",
		"status":  "authorized-gate",
		"target":  "sample.bin",
		"risk":    "high",
		"gate": map[string]any{
			"action":          "debug",
			"scope":           "single target",
			"requestedBudget": map[string]any{"runtimeSeconds": 45, "diskMB": 16, "requests": 2},
			"outputPaths":     []any{"workspace/features/feature-login/debug/session-1"},
			"stopConditions":  []any{"timeout", "new-risk"},
			"authorization":   map[string]any{"decision": "preauthorized", "profileId": "profile-login"},
		},
	})
	for _, want := range []string{
		"authorized debug",
		"action=debug",
		"scope=single target",
		"requestedBudget=runtimeSeconds=45,diskMB=16,requests=2",
		"outputPaths=workspace/features/feature-login/debug/session-1",
		"stopConditions=timeout,new-risk",
		"eventId=evt-authorized-login",
		"reportContract=/rekit gate -ExecutionReportContract -GateEventId evt-authorized-login -Format json",
		"auth=preauthorized",
		"profile=profile-login",
		"risk=high",
		"target=sample.bin",
	} {
		if !strings.Contains(line, want) {
			t.Fatalf("lane gate line missing %q: %q", want, line)
		}
	}
	if strings.Contains(line, "lane=feature-login") {
		t.Fatalf("lane-specific gate line should omit lane field: %q", line)
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
