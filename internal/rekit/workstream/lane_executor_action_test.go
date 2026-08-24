package workstream

import (
	"encoding/json"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"

	"github.com/shuiyu486/re-context-kits/internal/rekit/commands"
	"github.com/shuiyu486/re-context-kits/internal/rekit/defaults"
	"github.com/shuiyu486/re-context-kits/internal/rekit/mission"
)

func findStartWrite(t *testing.T, writes []StartWrite, path, action string) StartWrite {
	t.Helper()
	for _, write := range writes {
		if write.Path == path && write.Action == action {
			return write
		}
	}
	t.Fatalf("missing write path=%s action=%s in %+v", path, action, writes)
	return StartWrite{}
}

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

func TestBindMissionCommanderNextActionAuthorityContinueCommandsKeepsTypedIdentity(t *testing.T) {
	invocation, err := commands.NewPublicInvocation(
		commands.Continue,
		"login",
		"-WhatIf",
		"-Format", "json",
	)
	if err != nil {
		t.Fatal(err)
	}
	items := []mission.MissionCommanderNextActionItem{
		{
			Lane:       "feature-login",
			Command:    "/rekit continue login -WhatIf -Format json",
			Invocation: &invocation,
		},
		{
			Lane:    "feature-login",
			Command: "/rekit handoff login",
		},
	}
	bound, err := BindMissionCommanderNextActionAuthorityContinueCommands(items, mission.BoardLane{
		ID:                 "feature-login",
		CurrentExecutor:    "session two",
		ExecutorGeneration: 2,
	})
	if err != nil {
		t.Fatal(err)
	}
	want := `/rekit continue login -Executor "session two" -ExpectedExecutorGeneration 2 -WhatIf -Format json`
	if bound[0].Command != want || bound[0].Invocation == nil {
		t.Fatalf("owner-bound action omitted typed identity: %+v", bound[0])
	}
	projected, err := commands.ParsePublicInvocation(bound[0].Command)
	if err != nil || !bound[0].Invocation.Equivalent(projected) {
		t.Fatalf("owner-bound command and invocation drifted: item=%+v err=%v", bound[0], err)
	}
	if items[0].Command == bound[0].Command || items[0].Invocation == nil || !items[0].Invocation.Equivalent(invocation) {
		t.Fatalf("owner binding mutated its input: before=%+v after=%+v", items[0], bound[0])
	}
	if bound[1].Command != items[1].Command || bound[1].Invocation != nil {
		t.Fatalf("non-continue action changed: before=%+v after=%+v", items[1], bound[1])
	}
}

func TestBindMissionCommanderNextActionAuthorityContinueCommandsRejectsStaleTypedIdentity(t *testing.T) {
	stale, err := commands.NewPublicInvocation(commands.Continue, "other", "-WhatIf", "-Format", "json")
	if err != nil {
		t.Fatal(err)
	}
	_, err = BindMissionCommanderNextActionAuthorityContinueCommands(
		[]mission.MissionCommanderNextActionItem{{
			Command:    "/rekit continue login -WhatIf -Format json",
			Invocation: &stale,
		}},
		mission.BoardLane{
			ID:                 "feature-login",
			CurrentExecutor:    "executor-two",
			ExecutorGeneration: 2,
		},
	)
	if err == nil || !strings.Contains(err.Error(), "typed invocation does not match") {
		t.Fatalf("stale typed identity was accepted: %v", err)
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

func TestResolveHandoffLaneIDSupportsAuthorityAndNamedLanes(t *testing.T) {
	repoRoot, caseRoot := setupContinueCase(t, "")
	started, err := StartApply(repoRoot, caseRoot, defaults.DefaultPack, StartOptions{Selector: "analysis-sample"})
	if err != nil {
		t.Fatal(err)
	}
	lanePath := filepath.Join(caseRoot, ".rekit", "lanes", started.Lane.ID, "lane.json")
	started.Lane.Name = "sample"
	data, err := json.Marshal(started.Lane)
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(lanePath, data, 0o644); err != nil {
		t.Fatal(err)
	}
	for _, fixture := range []struct {
		selector string
		want     string
	}{
		{selector: "main", want: "devirt-main"},
		{selector: "sample", want: started.Lane.ID},
		{selector: started.Lane.ID, want: started.Lane.ID},
	} {
		got, err := ResolveHandoffLaneID(caseRoot, fixture.selector)
		if err != nil {
			t.Fatalf("ResolveHandoffLaneID(%q): %v", fixture.selector, err)
		}
		if got != fixture.want {
			t.Fatalf("ResolveHandoffLaneID(%q) = %q, want %q", fixture.selector, got, fixture.want)
		}
	}
}

func TestResolveHandoffLaneIDSupportsExactMainWithoutDefaultAuthorityLane(t *testing.T) {
	_, caseRoot := setupContinueCase(t, "")
	boardPath := filepath.Join(caseRoot, ".rekit", "board.json")
	data, err := os.ReadFile(boardPath)
	if err != nil {
		t.Fatal(err)
	}
	var board map[string]any
	if err := json.Unmarshal(data, &board); err != nil {
		t.Fatal(err)
	}
	delete(board, "defaultAuthorityLane")
	board["lanes"] = []map[string]any{{"id": "main"}}
	data, err = json.Marshal(board)
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(boardPath, data, 0o644); err != nil {
		t.Fatal(err)
	}
	laneRoot := filepath.Join(caseRoot, ".rekit", "lanes", "main")
	if err := os.MkdirAll(laneRoot, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(laneRoot, "lane.json"), []byte(`{"schemaVersion":1,"id":"main","type":"main","status":"open","authority":true,"workspace":"workspace/main","laneRoot":".rekit/lanes/main"}`), 0o644); err != nil {
		t.Fatal(err)
	}
	got, err := ResolveHandoffLaneID(caseRoot, "main")
	if err != nil {
		t.Fatal(err)
	}
	if got != "main" {
		t.Fatalf("ResolveHandoffLaneID(main) = %q", got)
	}
}

func TestTakeoverRefreshesDurableResumeCheckpointHandoffAndDigestCommands(t *testing.T) {
	repoRoot, caseRoot := setupContinueCase(t, "executor-one")
	beforePreview, err := HandoffPreview(repoRoot, caseRoot, defaults.DefaultPack, HandoffOptions{Selector: "devirt-main"})
	if err != nil {
		t.Fatal(err)
	}
	before, err := HandoffApply(repoRoot, caseRoot, defaults.DefaultPack, HandoffOptions{Selector: "devirt-main", ExpectedPublicationPlanSHA256: beforePreview.PublicationPlanSHA256, PublicationStamp: beforePreview.PublicationStamp})
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
	canonicalAfter := "/rekit continue -Executor executor-two -ExpectedExecutorGeneration 2 -Lane devirt-main"
	if preview.ExecutorAction == nil || preview.ExecutorAction.ResumeCommand != canonicalAfter {
		t.Fatalf("live handoff did not switch to current owner: %+v", preview.ExecutorAction)
	}
	continuePreview, err := ContinuePreview(repoRoot, caseRoot, defaults.DefaultPack, ContinueOptions{Selector: "devirt-main", Executor: "executor-two", ExpectedExecutorGeneration: 2})
	if err != nil {
		t.Fatal(err)
	}
	applied, err := ContinueApply(repoRoot, caseRoot, defaults.DefaultPack, ContinueOptions{Selector: "devirt-main", Executor: "executor-two", ExpectedExecutorGeneration: 2, ExpectedContinuePlanSHA256: continuePreview.ContinuePlanSHA256})
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
		if !strings.Contains(text, "/rekit continue main -Executor executor-two -ExpectedExecutorGeneration 2") || strings.Contains(text, "-Executor executor-one -ExpectedExecutorGeneration 1") {
			t.Fatalf("durable artifact did not use only current authority command %s:\n%s", path, text)
		}
	}
	takeoverWrite := findStartWrite(t, before.Writes, ".rekit/handovers/devirt-main-latest-replacement-executor-takeover.json", "write-latest-replacement-executor-takeover-package")
	data, err := os.ReadFile(takeoverWrite.TargetPath)
	if err != nil {
		t.Fatal(err)
	}
	var takeover mission.ReplacementExecutorTakeoverPackage
	if err := json.Unmarshal(data, &takeover); err != nil {
		t.Fatalf("replacement executor takeover package JSON did not decode: %v\n%s", err, string(data))
	}
	canonicalBefore := "/rekit continue -Executor executor-one -ExpectedExecutorGeneration 1 -Lane devirt-main"
	previewBefore := canonicalBefore + " -WhatIf -Format json"
	if before.ExecutorAction == nil || before.ExecutorAction.ResumeCommand != canonicalBefore || before.ExecutorAction.HandoffCommand != "/rekit handoff -Lane devirt-main" || before.LaneTakeoverPackage == nil || before.LaneTakeoverPackage.ContinueCommand != canonicalBefore || before.LaneTakeoverPackage.HandoffCommand != "/rekit handoff -Lane devirt-main" || before.LaneTakeoverPackage.CurrentCommand != previewBefore {
		t.Fatalf("handoff result did not preserve executor command and typed preview action: executor=%+v takeover=%+v", before.ExecutorAction, before.LaneTakeoverPackage)
	}
	if !takeover.Ready || takeover.Focus != "durable-handoff-current-action" || takeover.Scope != "lane:devirt-main" || takeover.DriverKind != "preview-command" || !takeover.CommandExecutable || takeover.Command != previewBefore || takeover.CurrentDriverRequest.Command != takeover.Command || takeover.RefreshStatusCommand == "" || !slices.ContainsFunc(takeover.TargetDocuments, func(doc string) bool {
		return doc == ".rekit/handovers/devirt-main-latest-replacement-executor-takeover.json"
	}) || !slices.ContainsFunc(takeover.RunbookSteps, func(step string) bool {
		return strings.Contains(step, "read .rekit/handovers/devirt-main-latest-replacement-executor-takeover.json before using any prior chat context")
	}) {
		t.Fatalf("durable replacement executor takeover package drifted: %+v", takeover)
	}
}

func TestLaneTakeoverPackageUsesSelectedStateRootAndRejectsConflicts(t *testing.T) {
	snapshot := mission.LaneExecutorActionSnapshot{
		Lane:      "main",
		Label:     "main",
		Status:    "open",
		Workspace: "workspace/main",
		ExecutorAction: mission.ExecutorAction{
			Ready:          true,
			ResumeCommand:  "/rekit continue main",
			HandoffCommand: "/rekit handoff main",
		},
	}
	queue := mission.MissionCommanderActionQueue{}

	currentRoot := t.TempDir()
	if err := os.MkdirAll(filepath.Join(currentRoot, ".steamai"), 0o755); err != nil {
		t.Fatal(err)
	}
	current, err := LaneTakeoverPackageForActionSnapshotE(currentRoot, snapshot, queue, false)
	if err != nil {
		t.Fatal(err)
	}
	if current.ResumePath != ".steamai/lanes/main/prompts/RESUME.md" || current.CheckpointPath != ".steamai/lanes/main/checkpoints/latest.json" || current.HandoffPath != ".steamai/handovers/main-latest.md" {
		t.Fatalf("current takeover paths did not use selected state root: %+v", current)
	}

	conflictingRoot := t.TempDir()
	for _, root := range []string{".steamai", ".rekit"} {
		if err := os.MkdirAll(filepath.Join(conflictingRoot, root), 0o755); err != nil {
			t.Fatal(err)
		}
	}
	if pkg, err := LaneTakeoverPackageForActionSnapshotE(conflictingRoot, snapshot, queue, false); err == nil || pkg != nil {
		t.Fatalf("error-aware takeover projection accepted conflicting state roots: pkg=%+v err=%v", pkg, err)
	}
	defer func() {
		if recovered := recover(); recovered == nil {
			t.Fatal("compatibility takeover projection swallowed conflicting state roots")
		}
	}()
	_ = LaneTakeoverPackageForActionSnapshot(conflictingRoot, snapshot, queue, false)
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
	if !hasStartCommanderNextAction(items, "missionCommanderActions.followUp", "/rekit continue login -WhatIf -Format json", true, true) {
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

func TestMissionCommanderActionQueuePrioritizesReconcileBeforeOtherLaneContinue(t *testing.T) {
	items := mission.MissionCommanderNextActions([]mission.LaneExecutorActionSnapshot{
		{
			Lane:  "feature-review",
			Label: "review",
			ExecutorAction: mission.ExecutorAction{
				Blocked: true,
				MissionCommanderAction: mission.MissionCommanderAction{
					State:          "needs-reconcile",
					PrimaryCommand: "/rekit reconcile review -InterventionId intervention-1 -WhatIf",
				},
			},
		},
		{
			Lane:  "main",
			Label: "main",
			ExecutorAction: mission.ExecutorAction{
				Ready: true,
				MissionCommanderAction: mission.MissionCommanderAction{
					State:          "ready-to-continue",
					PrimaryCommand: "/rekit continue main",
				},
			},
		},
	}, nil, true)
	queue := mission.MissionCommanderActionQueueFor(items)
	if queue.CurrentAction == nil || queue.CurrentAction.State != "needs-reconcile" || queue.CurrentAction.Blocked || !queue.CurrentAction.RequiresReview {
		t.Fatalf("open intervention reconcile should preempt unrelated lane continuation: %+v", queue)
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
	if !hasStartCommanderNextAction(items, "missionCommanderActions", "/rekit continue login -WhatIf -Format json", false, true) {
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
