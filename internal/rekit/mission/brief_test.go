package mission

import (
	"slices"
	"strings"
	"testing"

	"github.com/shuiyu486/re-context-kits/internal/rekit/commands"
)

func TestMissionCommanderActionQueueProjectsTypedInvocation(t *testing.T) {
	queue := MissionCommanderActionQueueFor([]MissionCommanderNextActionItem{{
		State:   "ready-to-continue",
		Source:  "missionCommanderActions",
		Lane:    "feature-mission",
		Command: `/rekit continue "feature-mission" -WhatIf -Format json`,
	}})
	if queue.CurrentAction == nil || queue.CurrentAction.Invocation == nil || queue.CurrentDriverRequest == nil || queue.CurrentDriverRequest.Invocation == nil {
		t.Fatalf("typed current action/request missing: %+v", queue)
	}
	if queue.CurrentAction.Invocation.Command != "continue" || !queue.CurrentAction.Invocation.HasFlag("-WhatIf") {
		t.Fatalf("unexpected typed action: %+v", queue.CurrentAction)
	}
	request := queue.CurrentDriverRequest
	if request.Kind != "preview-command" || !request.CommandExecutable || request.Command != `/rekit continue "feature-mission" -WhatIf -Format json` {
		t.Fatalf("unexpected typed request: %+v", request)
	}
	first, err := MissionCommanderDriverRequestSHA256(*request)
	if err != nil {
		t.Fatal(err)
	}
	second, err := MissionCommanderDriverRequestSHA256(*request)
	if err != nil || first != second || len(first) != 64 {
		t.Fatalf("driver request hash first=%q second=%q err=%v", first, second, err)
	}
}

func TestMissionCommanderDriverRequestRejectsExpectedReceiptCommandDrift(t *testing.T) {
	queue := MissionCommanderActionQueueFor([]MissionCommanderNextActionItem{{
		State:   "ready-to-continue",
		Source:  "missionCommanderActions",
		Lane:    "main",
		Command: "/rekit continue main -WhatIf -Format json",
	}})
	if queue.CurrentDriverRequest == nil {
		t.Fatalf("typed current driver request missing: %+v", queue)
	}
	drifted := *queue.CurrentDriverRequest
	drifted.ExpectedReceipt.Command = "/rekit status -Format json"
	if _, err := MissionCommanderDriverRequestSHA256(drifted); err == nil || !strings.Contains(err.Error(), "expected receipt command differs") {
		t.Fatalf("expected receipt command drift should fail closed: %v", err)
	}
}

func TestMissionCommanderActionQueueDoesNotSelectAmbiguousCurrentLane(t *testing.T) {
	for _, items := range [][]MissionCommanderNextActionItem{
		{
			{State: "ready-to-continue", Source: "missionCommanderActions", Lane: "feature-alpha", Command: "/rekit continue alpha -WhatIf -Format json"},
			{State: "ready-to-continue", Source: "missionCommanderActions", Lane: "feature-beta", Command: "/rekit continue beta -WhatIf -Format json"},
		},
		{
			{State: "ready-to-continue", Source: "missionCommanderActions", Lane: "feature-beta", Command: "/rekit continue beta -WhatIf -Format json"},
			{State: "ready-to-continue", Source: "missionCommanderActions", Lane: "feature-alpha", Command: "/rekit continue alpha -WhatIf -Format json"},
		},
	} {
		queue := MissionCommanderActionQueueFor(items)
		if queue.CurrentAction != nil || queue.CurrentDriverRequest != nil || queue.CurrentRunLoopStepID != "" || len(queue.CurrentActionRunLoop) != 0 {
			t.Fatalf("ambiguous lane queue selected an executable current action: %+v", queue)
		}
		if len(queue.UnblockedActions) != 2 || queue.Counts.Unblocked != 2 || queue.Summary != "total=2 unblocked=2 blocked=0 requiresReview=0 followUp=0 current=none" {
			t.Fatalf("ambiguous lane queue lost typed candidates: %+v", queue)
		}
		choices := MissionCommanderActionQueueLaneChoices(queue)
		if !MissionCommanderActionQueueRequiresLaneChoice(queue) || len(choices) != 2 {
			t.Fatalf("ambiguous lane queue did not publish typed choices: %+v", choices)
		}
	}
}

func TestMissionCommanderActionQueueKeepsSameLaneCurrentAction(t *testing.T) {
	queue := MissionCommanderActionQueueFor([]MissionCommanderNextActionItem{
		{State: "ready-to-continue", Source: "missionCommanderActions", Lane: "feature-alpha", Command: "/rekit continue alpha -WhatIf -Format json"},
		{State: "ready-to-continue", Source: "missionCommanderActions.followUp", Lane: "feature-alpha", Command: "/rekit handoff alpha -WhatIf -Format json"},
	})
	if queue.CurrentAction == nil || queue.CurrentDriverRequest == nil || queue.CurrentAction.Lane != "feature-alpha" || !queue.CurrentDriverRequest.CommandExecutable {
		t.Fatalf("single-lane queue lost current action: %+v", queue)
	}
	if MissionCommanderActionQueueRequiresLaneChoice(queue) || len(MissionCommanderActionQueueLaneChoices(queue)) != 0 {
		t.Fatalf("single-lane queue published a false lane choice: %+v", queue)
	}
}

func TestMissionCommanderActionQueueBlocksInvalidReKitText(t *testing.T) {
	for _, command := range []string{"rekit-host -daily", "rekitfoo status", "/rekit unknown", "/rekit continue -Command status"} {
		t.Run(command, func(t *testing.T) {
			queue := MissionCommanderActionQueueFor([]MissionCommanderNextActionItem{{State: "ready", Source: "test", Command: command}})
			if queue.CurrentAction == nil || !queue.CurrentAction.Blocked || !queue.CurrentAction.RequiresReview || queue.CurrentAction.Invocation != nil {
				t.Fatalf("invalid ReKit text was not blocked: %+v", queue.CurrentAction)
			}
			if queue.CurrentDriverRequest == nil || queue.CurrentDriverRequest.CommandExecutable || queue.CurrentDriverRequest.Invocation != nil || queue.CurrentDriverRequest.Kind != "blocked-review" {
				t.Fatalf("invalid ReKit text produced executable request: %+v", queue.CurrentDriverRequest)
			}
		})
	}
}

func TestValidateMissionCommanderDriverRequestRejectsInvocationTextDrift(t *testing.T) {
	invocation, err := commands.NewPublicInvocation(commands.Continue, "feature-mission")
	if err != nil {
		t.Fatal(err)
	}
	request := MissionCommanderDriverRequest{
		Kind:              "execute-command",
		RunLoopStepID:     "apply-or-run-current",
		Invocation:        &invocation,
		Command:           "/rekit status",
		CommandExecutable: true,
	}
	if err := ValidateMissionCommanderDriverRequest(request); err == nil || !strings.Contains(err.Error(), "differs") {
		t.Fatalf("ValidateMissionCommanderDriverRequest error=%v", err)
	}
}

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
	if commander.State != "needs-reconcile" || commander.PrimaryCommand != "/rekit reconcile login -InterventionId evt-open -WhatIf" || !slices.Contains(commander.FollowUpCommands, "/rekit reconcile login -InterventionId evt-open -Apply") || !slices.Contains(commander.FollowUpCommands, "/rekit continue login -WhatIf") || !slices.Contains(commander.Boundary, "review reconcile -WhatIf output before running the bounded -Apply follow-up") {
		t.Fatalf("blocked executor should expose concrete Mission Commander reconcile handoff: %+v", commander)
	}
}

func TestLaneExecutorActionListsConcreteMultiInterventionPreviewOptions(t *testing.T) {
	facts := Facts{
		Interventions: []map[string]any{
			{"eventId": "evt-open-a", "kind": "intervention", "lane": "feature-login", "subject": "manual stop", "status": "open"},
			{"eventId": "evt-open-b", "kind": "intervention", "lane": "feature-login", "subject": "manual override", "status": "open"},
		},
	}
	brief := BuildWithOptions([]Lane{{ID: "feature-login", Label: "login", Status: "active"}}, facts, BuildOptions{MaxRows: 10})
	action := LaneExecutorAction(Lane{ID: "feature-login", Label: "login", Status: "active"}, facts, brief)
	commander := action.MissionCommanderAction
	if commander.State != "needs-reconcile" || commander.PrimaryCommand != "/rekit handoff login" {
		t.Fatalf("multiple interventions should keep handoff as primary instead of choosing one: %+v", commander)
	}
	for _, command := range []string{"/rekit reconcile login -InterventionId evt-open-a -WhatIf", "/rekit reconcile login -InterventionId evt-open-b -WhatIf", "/rekit continue login -WhatIf"} {
		if !slices.Contains(commander.FollowUpCommands, command) {
			t.Fatalf("multiple interventions should expose concrete preview option %q: %+v", command, commander)
		}
	}
	if slices.Contains(commander.FollowUpCommands, "/rekit reconcile login -InterventionId <eventId> -WhatIf") {
		t.Fatalf("all-concrete multiple interventions should not require placeholder event selection: %+v", commander)
	}
	if !slices.Contains(commander.Boundary, "multiple or unidentified open interventions require handoff review before selecting a concrete eventId") {
		t.Fatalf("multiple interventions should retain selection boundary: %+v", commander)
	}
}

func TestLaneExecutorActionKeepsPlaceholderForUnidentifiedMultiIntervention(t *testing.T) {
	facts := Facts{
		Interventions: []map[string]any{
			{"eventId": "evt-open-a", "kind": "intervention", "lane": "feature-login", "subject": "manual stop", "status": "open"},
			{"kind": "intervention", "lane": "feature-login", "subject": "manual override", "status": "open"},
		},
	}
	brief := BuildWithOptions([]Lane{{ID: "feature-login", Label: "login", Status: "active"}}, facts, BuildOptions{MaxRows: 10})
	action := LaneExecutorAction(Lane{ID: "feature-login", Label: "login", Status: "active"}, facts, brief)
	commander := action.MissionCommanderAction
	if commander.PrimaryCommand != "/rekit handoff login" || !slices.Contains(commander.FollowUpCommands, "/rekit reconcile login -InterventionId evt-open-a -WhatIf") || !slices.Contains(commander.FollowUpCommands, "/rekit reconcile login -InterventionId <eventId> -WhatIf") {
		t.Fatalf("unidentified multi intervention should expose known options plus placeholder: %+v", commander)
	}
}

func TestLaneExecutorActionPendingGateUsesConcreteWhatIfBeforeApply(t *testing.T) {
	facts := Facts{
		Requests: []map[string]any{{"kind": "request", "lane": "feature-login", "subject": "debug gate", "status": "pending-gate", "risk": "high", "gate": map[string]any{"action": "debug", "scope": "unit-test"}}},
	}
	brief := BuildWithOptions([]Lane{{ID: "feature-login", Label: "login", Status: "active"}}, facts, BuildOptions{MaxRows: 10})
	action := LaneExecutorAction(Lane{ID: "feature-login", Label: "login", Status: "active"}, facts, brief)
	commander := action.MissionCommanderAction
	if commander.State != "needs-gate-decision" || commander.PrimaryCommand != "/rekit gate -Action debug -Lane feature-login -WhatIf" {
		t.Fatalf("pending gate should expose concrete gate decision preview: %+v", commander)
	}
	for _, command := range []string{"/rekit gate -Action debug -Lane feature-login -Apply -Actor <actor>", "/rekit continue login -WhatIf", "/rekit handoff login"} {
		if !slices.Contains(commander.FollowUpCommands, command) {
			t.Fatalf("pending gate follow-up should expose %q: %+v", command, commander)
		}
	}
	if containsSubstring(commander.FollowUpCommands, "<action>") {
		t.Fatalf("concrete pending gate should not require action placeholder: %+v", commander)
	}
	if !slices.Contains(commander.Boundary, "review gate -WhatIf output before running the bounded -Apply follow-up") {
		t.Fatalf("pending gate should require WhatIf review boundary: %+v", commander)
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

func TestMissionCommanderNextActionsIncludeLaneFollowUps(t *testing.T) {
	items := MissionCommanderNextActions([]LaneExecutorActionSnapshot{
		{
			Lane:  "main",
			Label: "main",
			ExecutorAction: ExecutorAction{
				Ready: true,
				MissionCommanderAction: MissionCommanderAction{
					State:            "ready-to-continue",
					PrimaryCommand:   "/rekit continue main",
					FollowUpCommands: []string{"/rekit handoff main"},
					Boundary:         []string{"no authority/confirmed writes"},
				},
			},
		},
		{
			Lane:  "feature-login",
			Label: "login",
			ExecutorAction: ExecutorAction{
				Blocked:        true,
				BlockerReasons: []string{"intervention"},
				MissionCommanderAction: MissionCommanderAction{
					State:            "needs-reconcile",
					PrimaryCommand:   "/rekit reconcile login -InterventionId evt-open -WhatIf",
					FollowUpCommands: []string{"/rekit reconcile login -InterventionId evt-open -Apply", "/rekit continue login -WhatIf", "/rekit handoff login"},
					Boundary:         []string{"do not run continue for blocked lanes", "review reconcile -WhatIf output before running the bounded -Apply follow-up"},
				},
			},
		},
	}, nil, false)

	if len(items) != 6 || items[0].Source != "missionCommanderActions" || items[0].Command != "/rekit continue main" || items[1].Source != "missionCommanderActions" || items[1].Command != "/rekit reconcile login -InterventionId evt-open -WhatIf" || items[1].Blocked || !items[1].RequiresReview {
		t.Fatalf("unexpected primary action ordering: %+v", items)
	}
	if !slices.ContainsFunc(items, func(item MissionCommanderNextActionItem) bool {
		return item.Source == "missionCommanderActions.followUp" && item.Command == "/rekit handoff main" && !item.Blocked && !item.RequiresReview && containsSubstring(item.Reasons, "follow Mission Commander handoff after primary action")
	}) {
		t.Fatalf("ready lane follow-up handoff missing: %+v", items)
	}
	if !slices.ContainsFunc(items, func(item MissionCommanderNextActionItem) bool {
		return item.Source == "missionCommanderActions.followUp" && item.Command == "/rekit reconcile login -InterventionId evt-open -Apply" && item.Blocked && item.RequiresReview && containsSubstring(item.Reasons, "follow-up is available only after resolving current lane blockers") && containsSubstring(item.Boundary, "bounded -Apply follow-up")
	}) {
		t.Fatalf("blocked lane reconcile apply follow-up should remain blocked until preview is reviewed: %+v", items)
	}
	if !slices.ContainsFunc(items, func(item MissionCommanderNextActionItem) bool {
		return item.Source == "missionCommanderActions.followUp" && item.Command == "/rekit continue login -WhatIf" && item.Blocked && item.RequiresReview && containsSubstring(item.Reasons, "run as -WhatIf first") && containsSubstring(item.Boundary, "do not run continue")
	}) {
		t.Fatalf("blocked lane continue follow-up should remain blocked with reason/boundary: %+v", items)
	}
	queue := MissionCommanderActionQueueFor(items)
	if queue.Summary != "total=6 unblocked=3 blocked=3 requiresReview=4 followUp=4 current=/rekit reconcile login -InterventionId evt-open -WhatIf" || queue.Counts.Total != 6 || queue.Counts.Unblocked != 3 || queue.Counts.Blocked != 3 || queue.Counts.RequiresReview != 4 || queue.Counts.FollowUp != 4 || queue.CurrentAction == nil || queue.CurrentAction.Command != "/rekit reconcile login -InterventionId evt-open -WhatIf" || len(queue.UnblockedActions) != 3 || len(queue.BlockedActions) != 3 || len(queue.ReviewRequiredActions) != 4 || len(queue.FollowUpActions) != 4 {
		t.Fatalf("Mission Commander action queue drifted: %+v", queue)
	}
}

func TestMissionCommanderActionQueuePromotesReviewBlockerOverFollowUp(t *testing.T) {
	items := []MissionCommanderNextActionItem{
		{State: "ready-to-continue", Command: "/rekit handoff login", Source: "missionCommanderActions.followUp"},
		{State: "waiting-for-reviewer-result", Command: "dispatch read-only reviewer for shard-02", Source: "reviewerDispatchIntakeHandoffs", Blocked: true, RequiresReview: true},
	}

	queue := MissionCommanderActionQueueFor(items)
	if queue.CurrentAction == nil || queue.CurrentAction.Command != "dispatch read-only reviewer for shard-02" || queue.CurrentAction.Source != "reviewerDispatchIntakeHandoffs" || queue.Summary != "total=2 unblocked=1 blocked=1 requiresReview=1 followUp=1 current=dispatch read-only reviewer for shard-02" {
		t.Fatalf("Mission Commander action queue did not promote reviewer blocker over ordinary follow-up: %+v", queue)
	}
}

func TestMissionCommanderActionQueuePromotesActiveProjectWorkOverLaneContinue(t *testing.T) {
	items := []MissionCommanderNextActionItem{
		{State: "ready-to-continue", Command: "/rekit continue main", Source: "missionCommanderActions"},
		{ActionID: "pack-memory-verification-provision-required", State: "pack-memory-verification-required", Command: "/rekit promote -ProvisionCandidateVerificationCases -WhatIf -Format json", Source: "packMemoryCandidates._template", RequiresReview: true},
	}

	queue := MissionCommanderActionQueueFor(items)
	if queue.CurrentAction == nil || queue.CurrentAction.ActionID != "pack-memory-verification-provision-required" || queue.CurrentAction.Source != "packMemoryCandidates._template" || queue.Summary != "total=2 unblocked=2 blocked=0 requiresReview=1 followUp=0 current=/rekit promote -ProvisionCandidateVerificationCases -WhatIf -Format json" {
		t.Fatalf("Mission Commander action queue should promote active project work over lane continue: %+v", queue)
	}
}

func TestMissionCommanderActionQueueRequiresChoiceBetweenOwnedAndUnassignedLanes(t *testing.T) {
	actions := []LaneExecutorActionSnapshot{
		{
			Lane:  "devirt-main",
			Label: "main",
			ExecutorAction: ExecutorAction{Ready: true, MissionCommanderAction: MissionCommanderAction{
				State:          "ready-to-continue",
				PrimaryCommand: "/rekit continue main",
			}},
		},
		{
			Lane:               "feature-analysis-live-acceptance",
			Label:              "analysis-live-acceptance",
			CurrentExecutor:    "live-member-generation-1",
			ExecutorGeneration: 1,
			ExecutorAction: ExecutorAction{Ready: true, MissionCommanderAction: MissionCommanderAction{
				State:          "ready-to-continue",
				PrimaryCommand: `/rekit continue analysis-live-acceptance -Executor "live-member-generation-1" -ExpectedExecutorGeneration 1`,
			}},
		},
	}

	queue := MissionCommanderActionQueueFor(MissionCommanderNextActions(actions, nil, false))
	if queue.CurrentAction != nil || queue.CurrentDriverRequest != nil || queue.CurrentRunLoopStepID != "" {
		t.Fatalf("Mission Commander action queue silently selected one of multiple current lanes: %+v", queue)
	}
	if len(queue.UnblockedActions) != 2 || queue.UnblockedActions[0].Lane != "feature-analysis-live-acceptance" || queue.UnblockedActions[1].Lane != "devirt-main" {
		t.Fatalf("Mission Commander action queue lost the typed lane choices: %+v", queue)
	}
}

func TestMissionCommanderActionQueueDefersIdleNextBatchGuidance(t *testing.T) {
	items := []MissionCommanderNextActionItem{
		{State: "ready-for-next-batch-selection", Command: "select the next Windows-verifiable product-path closure", Source: "releaseHandoffNextBatch"},
		{State: "next-batch-candidate-domain", Command: "select a replacement executor takeover slice", Source: "releaseHandoffNextBatch.followUp.candidateDomain"},
		{State: "ready-for-evidence-review", Command: "/rekit handoff main", Source: "executionEvidenceReview", RequiresReview: true},
	}

	queue := MissionCommanderActionQueueFor(items)
	if queue.CurrentAction == nil || queue.CurrentAction.Command != "/rekit handoff main" || queue.CurrentAction.Source != "executionEvidenceReview" || queue.Summary != "total=3 unblocked=3 blocked=0 requiresReview=1 followUp=1 current=/rekit handoff main" {
		t.Fatalf("Mission Commander action queue should defer idle next-batch guidance behind active evidence review: %+v", queue)
	}
}

func TestMissionCommanderActionQueueAllowsIdleNextBatchGuidanceWhenAlone(t *testing.T) {
	items := []MissionCommanderNextActionItem{
		{State: "ready-for-next-batch-selection", Command: "select the next Windows-verifiable product-path closure", Source: "releaseHandoffNextBatch"},
		{State: "next-batch-candidate-domain", Command: "select a replacement executor takeover slice", Source: "releaseHandoffNextBatch.followUp.candidateDomain"},
	}

	queue := MissionCommanderActionQueueFor(items)
	if queue.CurrentAction == nil || queue.CurrentAction.Command != "select the next Windows-verifiable product-path closure" || queue.CurrentAction.Source != "releaseHandoffNextBatch" || queue.Summary != "total=2 unblocked=2 blocked=0 requiresReview=0 followUp=1 current=select the next Windows-verifiable product-path closure" {
		t.Fatalf("Mission Commander action queue should allow idle next-batch guidance when no active work remains: %+v", queue)
	}
}

func TestMissionCommanderActionQueuePromotesPendingGateWhatIfOverBlockedHandoff(t *testing.T) {
	items := []MissionCommanderNextActionItem{
		{State: "needs-gate-decision", Command: "/rekit gate -Action debug -Lane feature-login -WhatIf", Source: "missionCommanderActions", RequiresReview: true, Reasons: []string{"pending-gate"}},
		{State: "needs-gate-decision", Command: "/rekit gate -Action debug -Lane feature-login -Apply -Actor <actor>", Source: "missionCommanderActions.followUp", Blocked: true, RequiresReview: true, Reasons: []string{"pending-gate"}},
		{State: "needs-gate-decision", Command: "/rekit continue login -WhatIf", Source: "missionCommanderActions.followUp", Blocked: true, RequiresReview: true, Reasons: []string{"pending-gate"}},
	}

	queue := MissionCommanderActionQueueFor(items)
	if queue.CurrentAction == nil || queue.CurrentAction.Command != "/rekit gate -Action debug -Lane feature-login -WhatIf" || queue.CurrentAction.Source != "missionCommanderActions" || queue.CurrentAction.Blocked || !queue.CurrentAction.RequiresReview || queue.Summary != "total=3 unblocked=1 blocked=2 requiresReview=3 followUp=2 current=/rekit gate -Action debug -Lane feature-login -WhatIf" {
		t.Fatalf("Mission Commander action queue did not promote pending-gate WhatIf over blocked follow-ups: %+v", queue)
	}
}

func TestMissionCommanderActionQueueAddsCurrentActionRunLoop(t *testing.T) {
	items := []MissionCommanderNextActionItem{
		{Lane: "main", Label: "main", State: "ready-to-continue", Command: "/rekit continue main", Source: "missionCommanderActions"},
		{Lane: "main", Label: "main", State: "ready-to-continue", Command: "/rekit handoff main", Source: "missionCommanderActions.followUp"},
		{Lane: "feature-login", Label: "login", State: "ready-to-continue", Command: "/rekit handoff login", Source: "missionCommanderActions.followUp"},
	}

	queue := MissionCommanderActionQueueFor(items)

	if queue.CurrentAction == nil || queue.CurrentAction.Command != "/rekit continue main" || queue.CurrentRunLoopStepID != "apply-or-run-current" {
		t.Fatalf("ready current action run loop drifted: %+v", queue)
	}
	assertMissionCommanderRunLoopStepIDs(t, queue.CurrentActionRunLoop, []string{"inspect-current", "apply-or-run-current", "refresh-state", "follow-up-after-refresh"})
	if queue.CurrentActionRunLoop[1].Command != "/rekit continue main" || queue.CurrentActionRunLoop[3].Command != "/rekit handoff main" {
		t.Fatalf("run loop did not bind current command and matching follow-up: %+v", queue.CurrentActionRunLoop)
	}
	if !containsSubstring(queue.CurrentActionRunLoop[1].Boundary, "Go runtime does not auto-run queued actions") || !containsSubstring(queue.CurrentActionRunLoop[3].Boundary, "remain candidates until refreshed state") {
		t.Fatalf("run loop boundaries should preserve read-only/no-auto-run semantics: %+v", queue.CurrentActionRunLoop)
	}
}

func TestMissionCommanderActionQueueRunLoopKeepsGuidanceOutOfExecutableCommandSteps(t *testing.T) {
	items := []MissionCommanderNextActionItem{
		{State: "ready-for-next-batch-selection", Command: "select the next Windows-verifiable product-path closure", Source: "releaseHandoffNextBatch"},
	}

	queue := MissionCommanderActionQueueFor(items)

	if queue.CurrentAction == nil || queue.CurrentAction.Command != "select the next Windows-verifiable product-path closure" || queue.CurrentRunLoopStepID != "inspect-current" {
		t.Fatalf("guidance current action should remain visible but not executable: %+v", queue)
	}
	assertMissionCommanderRunLoopStepIDs(t, queue.CurrentActionRunLoop, []string{"inspect-current", "refresh-state"})
	for _, step := range queue.CurrentActionRunLoop {
		if step.Command == "select the next Windows-verifiable product-path closure" {
			t.Fatalf("guidance text must not be exposed as an executable run-loop command: %+v", queue.CurrentActionRunLoop)
		}
	}
	if !containsSubstring(queue.CurrentActionRunLoop[0].Boundary, "not every current action command is shell-executable") {
		t.Fatalf("guidance run loop should explain executable command boundary: %+v", queue.CurrentActionRunLoop)
	}
}

func TestMissionCommanderActionQueueBuildsReadOnlyDriverRequestForExecutableCommand(t *testing.T) {
	items := []MissionCommanderNextActionItem{
		{Lane: "main", Label: "main", State: "ready-to-continue", Command: "/rekit continue main", Source: "missionCommanderActions"},
	}

	queue := MissionCommanderActionQueueFor(items)

	if queue.CurrentDriverRequest == nil || queue.CurrentDriverRequest.Kind != "execute-command" || queue.CurrentDriverRequest.Command != "/rekit continue main" || queue.CurrentDriverRequest.Guidance != "" || !queue.CurrentDriverRequest.CommandExecutable {
		t.Fatalf("executable command driver request drifted: %+v", queue.CurrentDriverRequest)
	}
	if queue.CurrentDriverRequest.ExpectedReceipt.State != "refresh-required" || queue.CurrentDriverRequest.ExpectedReceipt.Command != "/rekit continue main" {
		t.Fatalf("driver request should require refresh receipt after executable command: %+v", queue.CurrentDriverRequest.ExpectedReceipt)
	}
	if !containsSubstring(queue.CurrentDriverRequest.Boundary, "does not spawn, poll, stop, or run external sessions") || !containsSubstring(queue.CurrentDriverRequest.ExpectedReceipt.Boundary, "do not write authority/confirmed") {
		t.Fatalf("driver request lost no-spawn/no-authority boundaries: %+v", queue.CurrentDriverRequest)
	}
}

func TestMissionCommanderActionQueueBuildsReadOnlyDriverRequestForGuidance(t *testing.T) {
	items := []MissionCommanderNextActionItem{
		{State: "ready-for-next-batch-selection", Command: "select the next Windows-verifiable product-path closure", Source: "releaseHandoffNextBatch"},
	}

	queue := MissionCommanderActionQueueFor(items)

	if queue.CurrentDriverRequest == nil || queue.CurrentDriverRequest.Kind != "review-guidance" || queue.CurrentDriverRequest.Command != "" || queue.CurrentDriverRequest.Guidance != "select the next Windows-verifiable product-path closure" || queue.CurrentDriverRequest.CommandExecutable {
		t.Fatalf("guidance driver request should not expose executable command: %+v", queue.CurrentDriverRequest)
	}
	if queue.CurrentDriverRequest.RunLoopStepID != "inspect-current" || queue.CurrentDriverRequest.ExpectedReceipt.Command != "" {
		t.Fatalf("guidance driver request should stop at inspect-current and require refresh: %+v", queue.CurrentDriverRequest)
	}
	if !containsSubstring(queue.CurrentDriverRequest.Boundary, "guidance text must be reviewed") {
		t.Fatalf("guidance driver request lost no-run boundary: %+v", queue.CurrentDriverRequest.Boundary)
	}
}

func TestMissionCommanderActionQueueBuildsReadOnlyDriverRequestForBlockedReview(t *testing.T) {
	items := []MissionCommanderNextActionItem{
		{Lane: "feature-login", Label: "login", State: "waiting-for-reviewer-result", Command: "dispatch read-only reviewer for shard-02", Source: "reviewerDispatchIntakeHandoffs", Blocked: true, RequiresReview: true},
	}

	queue := MissionCommanderActionQueueFor(items)

	if queue.CurrentDriverRequest == nil || queue.CurrentDriverRequest.Kind != "blocked-review" || queue.CurrentDriverRequest.Command != "" || queue.CurrentDriverRequest.Guidance != "dispatch read-only reviewer for shard-02" {
		t.Fatalf("blocked guidance driver request drifted: %+v", queue.CurrentDriverRequest)
	}
	if !queue.CurrentDriverRequest.Blocked || !queue.CurrentDriverRequest.RequiresReview || !containsSubstring(queue.CurrentDriverRequest.Boundary, "blocked current actions require blocker review") || !containsSubstring(queue.CurrentDriverRequest.Boundary, "review-required current actions") {
		t.Fatalf("blocked driver request lost blocker/review boundaries: %+v", queue.CurrentDriverRequest)
	}
}

func TestMissionCommanderActionQueueRunLoopUsesPreviewForReviewRequiredWhatIf(t *testing.T) {
	items := []MissionCommanderNextActionItem{
		{Lane: "feature-login", Label: "login", State: "needs-gate-decision", Command: "/rekit gate -Action debug -Lane feature-login -WhatIf", Source: "missionCommanderActions", RequiresReview: true},
		{Lane: "feature-login", Label: "login", State: "needs-gate-decision", Command: "/rekit gate -Action debug -Lane feature-login -Apply -Actor <actor>", Source: "missionCommanderActions.followUp", Blocked: true, RequiresReview: true},
	}

	queue := MissionCommanderActionQueueFor(items)

	if queue.CurrentAction == nil || queue.CurrentRunLoopStepID != "preview-current" {
		t.Fatalf("review-required WhatIf current action should stop at preview-current: %+v", queue)
	}
	assertMissionCommanderRunLoopStepIDs(t, queue.CurrentActionRunLoop, []string{"inspect-current", "preview-current", "refresh-state", "follow-up-after-refresh"})
	if queue.CurrentActionRunLoop[1].Command != "/rekit gate -Action debug -Lane feature-login -WhatIf" || !containsSubstring(queue.CurrentActionRunLoop[1].Boundary, "review preview output") {
		t.Fatalf("preview run loop step lost WhatIf review boundary: %+v", queue.CurrentActionRunLoop[1])
	}
}

func TestMissionCommanderActionQueueRunLoopStopsBlockedCurrentAtInspect(t *testing.T) {
	items := []MissionCommanderNextActionItem{
		{Lane: "feature-login", Label: "login", State: "waiting-for-reviewer-result", Command: "dispatch read-only reviewer for shard-02", Source: "reviewerDispatchIntakeHandoffs", Blocked: true, RequiresReview: true},
	}

	queue := MissionCommanderActionQueueFor(items)

	if queue.CurrentAction == nil || queue.CurrentRunLoopStepID != "inspect-current" {
		t.Fatalf("blocked current action should stop at inspect-current: %+v", queue)
	}
	assertMissionCommanderRunLoopStepIDs(t, queue.CurrentActionRunLoop, []string{"inspect-current", "refresh-state"})
	for _, step := range queue.CurrentActionRunLoop {
		if step.Command == "dispatch read-only reviewer for shard-02" {
			t.Fatalf("blocked guidance text must not be exposed as executable run-loop command: %+v", queue.CurrentActionRunLoop)
		}
	}
	if !containsSubstring(queue.CurrentActionRunLoop[0].Boundary, "blocked current actions must not be treated as autonomous continue/run permission") || !containsSubstring(queue.CurrentActionRunLoop[0].Boundary, "not every current action command is shell-executable") {
		t.Fatalf("blocked current run loop should retain autonomous and executable-boundary guidance: %+v", queue.CurrentActionRunLoop[0])
	}
}

func assertMissionCommanderRunLoopStepIDs(t *testing.T, steps []MissionCommanderRunLoopStep, want []string) {
	t.Helper()
	if len(steps) != len(want) {
		t.Fatalf("run loop length=%d, want %d: %+v", len(steps), len(want), steps)
	}
	for idx, step := range steps {
		if step.Order != idx+1 || step.StepID != want[idx] {
			t.Fatalf("run loop step %d = order=%d id=%s, want order=%d id=%s: %+v", idx, step.Order, step.StepID, idx+1, want[idx], steps)
		}
	}
}

func TestMissionCommanderNextActionsKeepGateSpecificEvidenceActions(t *testing.T) {
	review := func(gateEventID, status string) ExecutionEvidenceReviewItem {
		return ExecutionEvidenceReviewItem{
			GateEventID: gateEventID,
			Status:      status,
			MissionCommanderAction: MissionCommanderAction{
				State:            "ready-for-evidence-review",
				PrimaryCommand:   "/rekit handoff main",
				FollowUpCommands: []string{"/rekit overview"},
			},
		}
	}
	items := MissionCommanderNextActions(nil, []ExecutionEvidenceReviewItem{
		review("gate-a", "succeeded"),
		review("gate-b", "escalated"),
	}, false)
	if len(items) != 4 {
		t.Fatalf("gate-specific evidence actions were deduplicated: %+v", items)
	}
	for _, gateEventID := range []string{"gate-a", "gate-b"} {
		if !slices.ContainsFunc(items, func(item MissionCommanderNextActionItem) bool {
			return item.GateEventID == gateEventID && item.Source == "executionEvidenceReview" && item.Command == "/rekit handoff main"
		}) {
			t.Fatalf("missing evidence action for %s: %+v", gateEventID, items)
		}
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
	loginCommander := loginAction.MissionCommanderAction
	if loginCommander.State != "needs-open-decision-review" || !strings.HasPrefix(loginCommander.PrimaryCommand, "/rekit note -Kind decision -Lane feature-login") || !strings.Contains(loginCommander.PrimaryCommand, "-Subject \"decision for candidate: login review\"") || !strings.Contains(loginCommander.PrimaryCommand, "-Decision <accept|reject|defer|supersede> -Reason \"reviewed open candidate/decision item\" -WhatIf") || !strings.Contains(loginCommander.Prompt, "candidate/decision") {
		t.Fatalf("blocked login lane action missing executable Mission Commander decision review preview: %+v", loginCommander)
	}
	if !slices.Equal(loginCommander.FollowUpCommands, []string{"/rekit continue login -WhatIf", "/rekit handoff login"}) || !containsSubstring(loginCommander.Boundary, "hash-bound recordCommand") {
		t.Fatalf("blocked login lane action missing hash-bound follow-up boundary: %+v", loginCommander)
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

func TestEffectiveOpenCandidatesHonorsRelatedDecisionNotes(t *testing.T) {
	facts := Facts{
		Candidates: []map[string]any{
			{"eventId": "cand-accepted", "kind": "candidate", "lane": "feature-login", "subject": "accepted candidate", "status": "open"},
			{"eventId": "cand-rejected", "kind": "candidate", "lane": "feature-login", "subject": "rejected candidate", "status": "open"},
			{"eventId": "cand-deferred", "kind": "candidate", "lane": "feature-login", "subject": "deferred candidate", "status": "open"},
			{"eventId": "cand-status-open", "kind": "candidate", "lane": "feature-login", "subject": "status-open decision", "status": "open"},
			{"kind": "candidate", "lane": "feature-login", "subject": "missing event id", "status": "open"},
		},
		Decisions: []map[string]any{
			{"kind": "decision", "lane": "feature-login", "subject": "accept accepted", "decision": "accept", "related": []any{"cand-accepted"}},
			{"kind": "decision", "lane": "feature-login", "subject": "reject rejected", "decision": "reject", "related": "cand-rejected"},
			{"kind": "decision", "lane": "feature-login", "subject": "defer still open", "decision": "defer", "related": []any{"cand-deferred"}},
			{"kind": "decision", "lane": "feature-login", "subject": "status open does not close", "decision": "accept", "status": "open", "related": []any{"cand-status-open"}},
			{"kind": "decision", "lane": "feature-login", "subject": "target is not lifecycle", "decision": "accept", "target": "cand-deferred"},
		},
	}

	open := EffectiveOpenCandidates(facts)
	if len(open) != 3 || Value(open[0], "eventId") != "cand-deferred" || Value(open[1], "eventId") != "cand-status-open" || Value(open[2], "subject") != "missing event id" {
		t.Fatalf("effective open candidates = %+v", open)
	}
	items := OpenDecisionItems(facts)
	if len(items) != 5 || containsOpenDecisionItem(items, "cand-accepted") || containsOpenDecisionItem(items, "cand-rejected") {
		t.Fatalf("open decision items should exclude candidates closed by related accept/reject notes: %+v", items)
	}
	brief := BuildWithOptions([]Lane{{ID: "feature-login", Label: "login", Status: "active"}}, facts, BuildOptions{MaxRows: 10})
	if brief.Summary != "openLanes=1 ready=0 blocked=1 pendingGates=0 authorizedGates=0 openDecisions=5 interventions=0" || !slices.Contains(brief.BlockedLanes, "login (open-decision)") || containsSubstring(brief.OpenDecisions, "accepted candidate") || containsSubstring(brief.OpenDecisions, "rejected candidate") {
		t.Fatalf("mission brief did not honor related decision closure: %+v", brief)
	}
}

func TestExecutionEvidenceReviewItemsWithLedgerFactsHonorsRelatedReviewNotes(t *testing.T) {
	observations := []map[string]any{
		executionEvidenceObservation("obs-accepted", "gate-accepted"),
		executionEvidenceObservation("obs-gate-related", "gate-related"),
		executionEvidenceObservation("obs-needs-more", "gate-needs-more"),
		executionEvidenceObservation("obs-inconclusive", "gate-inconclusive"),
		executionEvidenceObservation("obs-status-open", "gate-status-open"),
		executionEvidenceObservation("obs-deferred", "gate-deferred"),
		executionEvidenceObservation("obs-target-only", "gate-target-only"),
		executionEvidenceObservation("obs-unreviewed", "gate-unreviewed"),
	}
	facts := LedgerFacts{
		Facts: Facts{
			Decisions: []map[string]any{
				{"kind": "decision", "lane": "main", "status": "resolved", "decision": "accept", "related": []any{"obs-target-only"}},
			},
		},
		Observations: observations,
		Verifications: []map[string]any{
			{"kind": "verification", "lane": "main", "status": "resolved", "verdict": "accepted", "related": []any{"obs-accepted"}},
			{"kind": "verification", "lane": "main", "status": "accepted", "verdict": "accepted", "related": []any{"gate-related"}},
			{"kind": "verification", "lane": "main", "status": "needs_more_evidence", "verdict": "needs-more-evidence", "related": []any{"obs-needs-more"}},
			{"kind": "verification", "lane": "main", "status": "resolved", "verdict": "inconclusive", "related": []any{"obs-inconclusive"}},
			{"kind": "verification", "lane": "main", "status": "open", "verdict": "accepted", "related": []any{"obs-status-open"}},
			{"kind": "verification", "lane": "main", "status": "deferred", "verdict": "accepted", "related": []any{"obs-deferred"}},
			{"kind": "verification", "lane": "main", "status": "resolved", "verdict": "accepted", "target": "obs-target-only"},
		},
	}

	items := ExecutionEvidenceReviewItemsWithLedgerFacts(facts, "main", func(string) string { return "main" }, 0)
	if len(items) != 5 || items[0].EventID != "obs-needs-more" || items[1].EventID != "obs-inconclusive" || items[2].EventID != "obs-status-open" || items[3].EventID != "obs-deferred" || items[4].EventID != "obs-unreviewed" {
		t.Fatalf("execution evidence review items = %+v", items)
	}
}

func TestExecutionEvidenceReviewRejectedNotesRemainOpen(t *testing.T) {
	for name, facts := range map[string]LedgerFacts{
		"verification": {
			Observations: []map[string]any{executionEvidenceObservation("obs-rejected", "gate-rejected")},
			Verifications: []map[string]any{
				{"kind": "verification", "lane": "main", "status": "resolved", "verdict": "rejected", "related": []any{"obs-rejected", "gate-rejected"}},
			},
		},
		"decision": {
			Observations: []map[string]any{executionEvidenceObservation("obs-rejected", "gate-rejected")},
			Facts: Facts{Decisions: []map[string]any{
				{"kind": "decision", "lane": "main", "status": "resolved", "decision": "reject", "related": []any{"obs-rejected", "gate-rejected"}},
			}},
		},
	} {
		t.Run(name, func(t *testing.T) {
			items := ExecutionEvidenceReviewItemsWithLedgerFacts(
				facts,
				"main",
				func(string) string { return "main" },
				0,
			)
			if len(items) != 1 || items[0].EventID != "obs-rejected" {
				t.Fatalf("rejected evidence review incorrectly closed the blocker: %+v", items)
			}
		})
	}
}

func TestExecutionEvidenceReviewItemProjectsAdapterExecutionReceiptLineage(t *testing.T) {
	observation := executionEvidenceObservation("obs-receipt", "gate-receipt")
	execution := observation["execution"].(map[string]any)
	execution["executionReportPath"] = "workspace/main/debug/adapter-report.json"
	execution["executionReportSha256"] = strings.Repeat("a", 64)
	execution["adapterExecutionReceiptPath"] = ".rekit/lanes/main/adapter-executions/gate-receipt/receipt.json"
	execution["adapterExecutionReceiptSha256"] = strings.Repeat("b", 64)
	execution["adapterExecution"] = map[string]any{
		"owner": map[string]any{
			"currentExecutor": "executor-a", "executorGeneration": float64(3),
			"adapterHarness": "claude-code", "adapterSession": "session-a",
		},
		"adapter": map[string]any{"toolingCatalogSha256": strings.Repeat("c", 64)},
		"artifacts": []any{
			map[string]any{"path": "workspace/main/debug/result.bin"},
			map[string]any{"path": "workspace/main/debug/evidence.json"},
		},
	}
	item, ok := ExecutionEvidenceReviewItemFromObservation(observation, "main", nil)
	if !ok {
		t.Fatal("receipt-backed observation was not projected")
	}
	if item.AdapterExecutionReceiptPath == "" || item.AdapterExecutionReceiptSHA256 != strings.Repeat("b", 64) || item.CurrentExecutor != "executor-a" || item.ExecutorGeneration != 3 || item.AdapterHarness != "claude-code" || item.AdapterSession != "session-a" || item.ToolingCatalogSHA256 != strings.Repeat("c", 64) || item.AdapterExecutionArtifactCount != 2 {
		t.Fatalf("execution evidence review omitted compact receipt lineage: %+v", item)
	}
}

func executionEvidenceObservation(eventID, gateEventID string) map[string]any {
	return map[string]any{
		"eventId": eventID,
		"kind":    "observation",
		"lane":    "main",
		"status":  "succeeded",
		"execution": map[string]any{
			"gateEventId":   gateEventID,
			"authorization": "preauthorized",
		},
	}
}

func containsOpenDecisionItem(items []map[string]any, eventID string) bool {
	for _, item := range items {
		if Value(item, "eventId") == eventID {
			return true
		}
	}
	return false
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
