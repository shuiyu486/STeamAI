package gate

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/shuiyu486/re-context-kits/internal/rekit/mission"
)

func TestPlanDryRunDoesNotWriteRequestLedger(t *testing.T) {
	repoRoot, caseRoot, pack := gateFixture(t)

	plan, err := PlanDryRun(repoRoot, caseRoot, pack, Options{Action: "debug", Lane: "main", Subject: "preview gate"})
	if err != nil {
		t.Fatal(err)
	}
	if plan.Command != "gate" || plan.IsMutation || !plan.RequiresConfirmation || plan.EventPreview.Status != "pending-gate" || len(plan.BlockedActions) != 1 || plan.BlockedActions[0] != "debug" {
		t.Fatalf("unexpected gate dry-run plan: %+v", plan)
	}
	if plan.MissionBrief.Summary == "" || len(plan.MissionBrief.ReadyLanes) != 1 || plan.MissionBrief.ReadyLanes[0] != "main" || len(plan.MissionBrief.PendingGates) != 0 {
		t.Fatalf("gate dry-run missing pre-apply mission brief: %+v", plan.MissionBrief)
	}
	if plan.ExecutorAction.Blocked || !plan.ExecutorAction.Ready || plan.ExecutorAction.PendingGates != 0 || plan.ExecutorAction.ResumeCommand != "/rekit continue main" {
		t.Fatalf("gate dry-run current executor action drifted: %+v", plan.ExecutorAction)
	}
	if !plan.WouldExecutorAction.Blocked || plan.WouldExecutorAction.Ready || plan.WouldExecutorAction.PendingGates != 1 || !plan.WouldExecutorAction.PendingGateRequired || plan.WouldExecutorAction.ResumeCommand != "/rekit continue main" {
		t.Fatalf("gate dry-run would executor action drifted: %+v", plan.WouldExecutorAction)
	}
	if plan.MissionCommanderAction.State != "needs-gate-apply" || !strings.Contains(plan.MissionCommanderAction.PrimaryCommand, "/rekit gate -Pack vmp-re -Action debug -Lane main -Apply -Actor <actor>") || !gateNextActionContainsCommand(plan.MissionCommanderNextActions, plan.MissionCommanderAction.PrimaryCommand) || !gateNextActionContainsSource(plan.MissionCommanderNextActions, "missionCommanderActions.followUp") || !gateNextActionBoundaryContains(plan.MissionCommanderNextActions, "pending-gate still requires explicit authorization") {
		t.Fatalf("gate dry-run omitted top-level Mission Commander apply projection: action=%+v next=%+v", plan.MissionCommanderAction, plan.MissionCommanderNextActions)
	}
	assertGateNotExists(t, filepath.Join(caseRoot, ".rekit", "facts", "requests.jsonl"))
}

func TestPlanDryRunAcceptsManifestDeclaredAction(t *testing.T) {
	repoRoot, caseRoot, pack := gateFixture(t)

	plan, err := PlanDryRun(repoRoot, caseRoot, pack, Options{Action: "symex", Lane: "main", Subject: "symbolic execution gate"})
	if err != nil {
		t.Fatal(err)
	}
	if plan.EventPreview.Risk != "medium" || plan.EventPreview.Gate.Action != "symex" || len(plan.EventPreview.Gate.StopConditions) != 3 || plan.EventPreview.Gate.StopConditions[0] != "path-explosion" {
		t.Fatalf("unexpected manifest-driven symex gate plan: %+v", plan.EventPreview)
	}
}

func TestPlanDryRunRejectsUndeclaredAction(t *testing.T) {
	repoRoot, caseRoot, pack := gateFixture(t)

	_, err := PlanDryRun(repoRoot, caseRoot, pack, Options{Action: "network", Lane: "main", Subject: "undeclared gate"})
	if err == nil || !strings.Contains(err.Error(), "invalid gate action") || !strings.Contains(err.Error(), "debug,symex") {
		t.Fatalf("PlanDryRun error = %v, want manifest allowed action list", err)
	}
	assertGateNotExists(t, filepath.Join(caseRoot, ".rekit", "facts", "requests.jsonl"))
}

func TestApplyRequiresActorAndDoesNotWrite(t *testing.T) {
	repoRoot, caseRoot, pack := gateFixture(t)

	_, err := Apply(repoRoot, caseRoot, pack, Options{Action: "debug", Lane: "main", Subject: "missing actor"})
	if err == nil || !strings.Contains(err.Error(), "requires -Actor") {
		t.Fatalf("Apply error = %v, want requires -Actor", err)
	}
	assertGateNotExists(t, filepath.Join(caseRoot, ".rekit", "facts", "requests.jsonl"))
}

func TestPlanDryRunRejectsInvalidStopConditionsOverride(t *testing.T) {
	for _, tc := range []struct {
		name  string
		value string
		want  string
	}{
		{
			name:  "phrase",
			value: "timeout,unexpected side effect",
			want:  "gate stopConditions has invalid item: unexpected side effect",
		},
		{
			name:  "empty-item",
			value: "timeout,",
			want:  "gate stopConditions contains an empty item",
		},
		{
			name:  "duplicate",
			value: "timeout,timeout",
			want:  "gate stopConditions contains duplicate item: timeout",
		},
	} {
		t.Run(tc.name, func(t *testing.T) {
			repoRoot, caseRoot, pack := gateFixture(t)
			_, err := PlanDryRun(repoRoot, caseRoot, pack, Options{Action: "debug", Lane: "main", Subject: "bad stop conditions", StopConditions: tc.value})
			if err == nil || !strings.Contains(err.Error(), tc.want) {
				t.Fatalf("PlanDryRun error = %v, want %s", err, tc.want)
			}
			assertGateNotExists(t, filepath.Join(caseRoot, ".rekit", "facts", "requests.jsonl"))
		})
	}
}

func TestApplyRejectsInvalidStopConditionsOverrideDoesNotWrite(t *testing.T) {
	repoRoot, caseRoot, pack := gateFixture(t)

	_, err := Apply(repoRoot, caseRoot, pack, Options{Action: "debug", Lane: "main", Actor: "gate-test", Subject: "bad stop conditions", StopConditions: "timeout,unexpected side effect"})
	if err == nil || !strings.Contains(err.Error(), "gate stopConditions has invalid item: unexpected side effect") {
		t.Fatalf("Apply error = %v, want invalid stopConditions item error", err)
	}
	assertGateNotExists(t, filepath.Join(caseRoot, ".rekit", "facts", "requests.jsonl"))
}

func TestPlanDryRunRejectsInvalidRiskOverride(t *testing.T) {
	for _, tc := range []struct {
		name  string
		value string
	}{
		{name: "uppercase", value: "High"},
		{name: "unsupported", value: "low"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			repoRoot, caseRoot, pack := gateFixture(t)
			_, err := PlanDryRun(repoRoot, caseRoot, pack, Options{Action: "debug", Lane: "main", Subject: "bad risk", Risk: tc.value})
			want := "gate risk has unsupported value: " + tc.value
			if err == nil || !strings.Contains(err.Error(), want) {
				t.Fatalf("PlanDryRun error = %v, want %s", err, want)
			}
			assertGateNotExists(t, filepath.Join(caseRoot, ".rekit", "facts", "requests.jsonl"))
		})
	}
}

func TestApplyRejectsInvalidRiskOverrideDoesNotWrite(t *testing.T) {
	repoRoot, caseRoot, pack := gateFixture(t)

	_, err := Apply(repoRoot, caseRoot, pack, Options{Action: "debug", Lane: "main", Actor: "gate-test", Subject: "bad risk", Risk: "High"})
	if err == nil || !strings.Contains(err.Error(), "gate risk has unsupported value: High") {
		t.Fatalf("Apply error = %v, want invalid risk error", err)
	}
	assertGateNotExists(t, filepath.Join(caseRoot, ".rekit", "facts", "requests.jsonl"))
}

func TestPlanDryRunRejectsInvalidManifestDefaultRisk(t *testing.T) {
	repoRoot, caseRoot, pack := gateFixtureWithDefaultRisk(t, "High")

	_, err := PlanDryRun(repoRoot, caseRoot, pack, Options{Action: "debug", Lane: "main", Subject: "bad manifest risk"})
	if err == nil || !strings.Contains(err.Error(), "gate action \"debug\" has invalid manifest defaultRisk has unsupported value: High") {
		t.Fatalf("PlanDryRun error = %v, want invalid manifest defaultRisk error", err)
	}
	assertGateNotExists(t, filepath.Join(caseRoot, ".rekit", "facts", "requests.jsonl"))
}

func TestApplyWritesOnlyPendingGateRequest(t *testing.T) {
	repoRoot, caseRoot, pack := gateFixture(t)

	result, err := Apply(repoRoot, caseRoot, pack, Options{
		Action:          "debug",
		Lane:            "main",
		Actor:           "gate-test",
		Risk:            "high",
		Subject:         "debug gate",
		Summary:         "request bounded debug",
		TargetRef:       "batch-115-target",
		BatchID:         "batch-115",
		Scope:           "handler only",
		Budget:          "30s",
		TriedLightSteps: "overview,static review",
		StopConditions:  "timeout,unexpected-side-effect",
	})
	if err != nil {
		t.Fatal(err)
	}
	if result.Command != "gate" || !result.IsMutation || !result.Applied || result.EventID == "" || result.Path != ".rekit/facts/requests.jsonl" {
		t.Fatalf("unexpected gate apply result: %+v", result)
	}
	if !result.ExecutorAction.Blocked || result.ExecutorAction.Ready || result.ExecutorAction.PendingGates != 1 || !result.ExecutorAction.PendingGateRequired || result.ExecutorAction.ResumeCommand != "/rekit continue main" {
		t.Fatalf("gate apply executor action drifted: %+v", result.ExecutorAction)
	}
	if result.MissionCommanderAction.State != "needs-gate-decision" || result.MissionCommanderAction.PrimaryCommand != "/rekit handoff main" || !gateNextActionContainsCommand(result.MissionCommanderNextActions, "/rekit handoff main") || !gateNextActionContainsCommand(result.MissionCommanderNextActions, "/rekit continue main -WhatIf") || !gateNextActionBoundaryContains(result.MissionCommanderNextActions, "no authority/confirmed") {
		t.Fatalf("gate apply omitted top-level pending-gate Mission Commander projection: action=%+v next=%+v", result.MissionCommanderAction, result.MissionCommanderNextActions)
	}
	requestPath := filepath.Join(caseRoot, ".rekit", "facts", "requests.jsonl")
	event := readSingleGateEvent(t, requestPath)
	if event.Kind != "request" || event.Status != "pending-gate" || event.Lane != "main" || event.Actor != "gate-test" || event.Risk != "high" || event.Target != "batch-115-target" || event.BatchID != "batch-115" || event.EventID != result.EventID || event.CreatedAt == "" {
		t.Fatalf("unexpected gate event: %+v", event)
	}
	if event.Gate.Action != "debug" || event.Gate.Scope != "handler only" || event.Gate.Budget != "30s" || !event.Gate.RequiresConfirmation {
		t.Fatalf("unexpected gate details: %+v", event.Gate)
	}
	if got := strings.Join(event.Gate.TriedLightSteps, ","); got != "overview,static review" {
		t.Fatalf("triedLightSteps = %q", got)
	}
	if got := strings.Join(event.Gate.StopConditions, ","); got != "timeout,unexpected-side-effect" {
		t.Fatalf("stopConditions = %q", got)
	}
	if len(event.Gate.DeniedUntilUserConfirmation) != 1 || event.Gate.DeniedUntilUserConfirmation[0] != "debug" {
		t.Fatalf("denied actions = %+v", event.Gate.DeniedUntilUserConfirmation)
	}
	assertGateNotExists(t, filepath.Join(caseRoot, ".rekit", "facts", "authority.jsonl"))
	assertGateNotExists(t, filepath.Join(caseRoot, ".rekit", "facts", "confirmed.jsonl"))
	assertGateNotExists(t, filepath.Join(caseRoot, "captures"))
	assertGateNotExists(t, filepath.Join(caseRoot, "artifacts"))
}

func TestApplyDuplicateEventDoesNotAppend(t *testing.T) {
	repoRoot, caseRoot, pack := gateFixture(t)
	opt := Options{Action: "debug", Lane: "main", Actor: "gate-test", Subject: "duplicate gate", Summary: "same semantic request"}

	first, err := Apply(repoRoot, caseRoot, pack, opt)
	if err != nil {
		t.Fatal(err)
	}
	second, err := Apply(repoRoot, caseRoot, pack, opt)
	if err != nil {
		t.Fatal(err)
	}
	if !first.Applied || second.Applied || second.EventID != first.EventID || second.Reason != "duplicate eventId" {
		t.Fatalf("unexpected duplicate results: first=%+v second=%+v", first, second)
	}
	lines := readGateLines(t, filepath.Join(caseRoot, ".rekit", "facts", "requests.jsonl"))
	if len(lines) != 1 {
		t.Fatalf("duplicate apply wrote %d lines, want 1: %q", len(lines), lines)
	}
}

func TestApplyDuplicateEventUsesSharedJSONLReader(t *testing.T) {
	repoRoot, caseRoot, pack := gateFixture(t)
	opt := Options{Action: "debug", Lane: "main", Actor: "gate-test", Subject: "duplicate gate", Summary: "same semantic request"}

	first, err := Apply(repoRoot, caseRoot, pack, opt)
	if err != nil {
		t.Fatal(err)
	}
	requestPath := filepath.Join(caseRoot, ".rekit", "facts", "requests.jsonl")
	writeGateText(t, requestPath, "not json\n"+strings.Join(readGateLines(t, requestPath), "\n")+"\n")
	second, err := Apply(repoRoot, caseRoot, pack, opt)
	if err != nil {
		t.Fatal(err)
	}
	if second.Applied || second.EventID != first.EventID || second.Reason != "duplicate eventId" {
		t.Fatalf("duplicate lookup did not skip malformed JSONL via shared reader: first=%+v second=%+v", first, second)
	}
	lines := readGateLines(t, requestPath)
	if len(lines) != 2 {
		t.Fatalf("duplicate apply wrote new event after malformed line, lines=%q", lines)
	}
}

func gateFixture(t *testing.T) (repoRoot, caseRoot, pack string) {
	t.Helper()
	return gateFixtureWithDefaultRisk(t, "high")
}

func gateFixtureWithDefaultRisk(t *testing.T, defaultRisk string) (repoRoot, caseRoot, pack string) {
	t.Helper()
	root := t.TempDir()
	repoRoot = filepath.Join(root, "repo")
	caseRoot = filepath.Join(root, "case")
	pack = "vmp-re"
	writeGateText(t, filepath.Join(repoRoot, "packs", pack, "manifest.yml"), `name: vmp-re
managedFiles:
  - references/vmp-re/README.md
heavyToolGates:
  - id: debug
    title: Dynamic debug or attach
    sideEffects: debug,filesystem-write
    defaultRisk: `+defaultRisk+`
    requiresConfirmation: true
    stopConditions: timeout,unexpected-side-effect,scope-drift
  - id: symex
    title: Long symbolic execution
    sideEffects: symex,filesystem-write
    defaultRisk: medium
    requiresConfirmation: true
    stopConditions: path-explosion,budget-exhausted,output-exceeds-bounded-evidence-packet
`)
	writeGateText(t, filepath.Join(caseRoot, ".rekit", "instance.yml"), "templateRoot: \""+repoRoot+"\"\ntemplatePack: \""+pack+"\"\nprojectName: \"gate-fixture\"\nprojectRoot: \""+caseRoot+"\"\n")
	writeGateText(t, filepath.Join(caseRoot, ".rekit", "board.json"), `{"lanes":[{"id":"main"}]}`)
	return repoRoot, caseRoot, pack
}

func gateToolingFixture(t *testing.T) (repoRoot, caseRoot, pack string) {
	repoRoot, caseRoot, pack = gateFixture(t)
	writeGateText(t, filepath.Join(repoRoot, "packs", pack, "manifest.yml"), `name: vmp-re
managedFiles:
  - references/vmp-re/README.md
toolingFiles:
  - tooling/catalog.yml
heavyToolGates:
  - id: debug
    title: Dynamic debug or attach
    sideEffects: debug,filesystem-write
    defaultRisk: high
    requiresConfirmation: true
    stopConditions: timeout,unexpected-side-effect,scope-drift
  - id: symex
    title: Long symbolic execution
    sideEffects: symex,filesystem-write
    defaultRisk: medium
    requiresConfirmation: true
    stopConditions: path-explosion,budget-exhausted,output-exceeds-bounded-evidence-packet
`)
	writeGateText(t, filepath.Join(repoRoot, "packs", pack, "tooling", "catalog.yml"), `schemaVersion: 1
pack: vmp-re
purpose: tooling fixture

tools:
  - id: static-summary-sidecar
    status: mainline-template
    entry: saved summary sidecar
    purpose: Review saved metadata without side effects.
    sideEffects: none
  - id: dynamic-debug-or-writeback-action
    status: cautious
    entry: <caseRoot>/tools/dynamic-debug-or-writeback-action
    purpose: Execute a bounded debug, trace, dump, patch, rename/comment, bulk decompile, database writeback, or networked verification step.
    sideEffects: process execution, debugger state, trace files, database writes, binary patches, filesystem writes
`)
	return repoRoot, caseRoot, pack
}

func TestPlanDryRunUsesPreauthorizedLaneAutonomyProfile(t *testing.T) {
	repoRoot, caseRoot, pack := gateFixture(t)
	writeGateText(t, filepath.Join(caseRoot, ".rekit", "lanes", "main", "autonomy.json"), `{
  "schemaVersion": 1,
  "profileId": "prof-main-debug",
  "lane": "main",
  "mode": "preauthorized",
  "allowedActions": ["debug"],
  "deniedActions": ["symex"],
  "targetScope": [{"match":"exact","value":"target-alpha"}],
  "budget": {"runtimeSeconds": 60, "diskMB": 128, "requests": 2},
  "stopConditions": ["timeout", "scope-drift"],
  "outputPaths": ["workspace/main/debug"],
  "recordRequired": true,
  "notifyMainOn": ["boundary-hit", "new-risk"],
  "grantedBy": "user",
  "grantedAt": "2026-01-01T00:00:00Z",
  "expiresAt": "2999-01-01T00:00:00Z"
}`)
	plan, err := PlanDryRun(repoRoot, caseRoot, pack, Options{Action: "debug", Lane: "main", TargetRef: "target-alpha", RuntimeSeconds: 30, DiskMB: 64, Requests: 1, OutputPaths: "workspace/main/debug/session-1", StopConditions: "timeout"})
	if err != nil {
		t.Fatal(err)
	}
	if plan.RequiresConfirmation || plan.ReviewRequired || len(plan.BlockedActions) != 0 || plan.EventPreview.Status != "authorized-gate" {
		t.Fatalf("unexpected authorized plan: %+v", plan)
	}
	if plan.EventPreview.Gate.Authorization.Decision != "preauthorized" || plan.EventPreview.Gate.Authorization.ProfileID != "prof-main-debug" || plan.EventPreview.Gate.RequiresConfirmation {
		t.Fatalf("unexpected authorization details: %+v", plan.EventPreview.Gate.Authorization)
	}
	if plan.WouldExecutorAction.Blocked || !plan.WouldExecutorAction.Ready || plan.WouldExecutorAction.PendingGates != 0 {
		t.Fatalf("authorized gate would executor action should remain non-blocking: %+v", plan.WouldExecutorAction)
	}
	if plan.MissionCommanderAction.State != "needs-authorized-gate-apply" || !strings.Contains(plan.MissionCommanderAction.PrimaryCommand, "/rekit gate -Pack vmp-re -Action debug -Lane main -Apply -Actor <actor>") || !gateNextActionContainsSource(plan.MissionCommanderNextActions, "missionCommanderActions") || !gateNextActionContainsCommand(plan.MissionCommanderNextActions, "-ExecutionReportContract") || !gateNextActionBoundaryContains(plan.MissionCommanderNextActions, "actual heavy tool must stay within authorized target") {
		t.Fatalf("authorized gate plan omitted top-level Mission Commander apply projection: action=%+v next=%+v", plan.MissionCommanderAction, plan.MissionCommanderNextActions)
	}
	if plan.EventPreview.Gate.RequestedBudget.RuntimeSeconds != 30 || strings.Join(plan.EventPreview.Gate.OutputPaths, ",") != "workspace/main/debug/session-1" {
		t.Fatalf("missing typed request contract: %+v", plan.EventPreview.Gate)
	}
	assertGateNotExists(t, filepath.Join(caseRoot, ".rekit", "facts", "requests.jsonl"))
}

func TestPlanDryRunFallsBackToPendingGateWhenAutonomyOutOfScope(t *testing.T) {
	repoRoot, caseRoot, pack := gateFixture(t)
	writeGateText(t, filepath.Join(caseRoot, ".rekit", "lanes", "main", "autonomy.json"), `{
  "schemaVersion": 1,
  "profileId": "prof-main-debug",
  "lane": "main",
  "mode": "preauthorized",
  "allowedActions": ["debug"],
  "deniedActions": [],
  "targetScope": [{"match":"exact","value":"target-alpha"}],
  "budget": {"runtimeSeconds": 60, "diskMB": 128, "requests": 2},
  "stopConditions": ["timeout"],
  "outputPaths": ["workspace/main/debug"],
  "recordRequired": true,
  "notifyMainOn": ["boundary-hit"],
  "grantedBy": "user",
  "grantedAt": "2026-01-01T00:00:00Z",
  "expiresAt": "2999-01-01T00:00:00Z"
}`)
	plan, err := PlanDryRun(repoRoot, caseRoot, pack, Options{Action: "debug", Lane: "main", TargetRef: "target-beta", RuntimeSeconds: 30, DiskMB: 64, Requests: 1, OutputPaths: "workspace/main/debug/session-1", StopConditions: "timeout"})
	if err != nil {
		t.Fatal(err)
	}
	if !plan.RequiresConfirmation || len(plan.BlockedActions) != 1 || plan.EventPreview.Status != "pending-gate" || plan.EventPreview.Gate.Authorization.Decision != "out-of-scope" {
		t.Fatalf("unexpected out-of-scope plan: %+v", plan)
	}
}

func TestRecordExecutionWritesObservationForAuthorizedGate(t *testing.T) {
	repoRoot, caseRoot, pack := gateFixture(t)
	writePreauthorizedProfile(t, caseRoot)
	authorized, err := Apply(repoRoot, caseRoot, pack, Options{Action: "debug", Lane: "main", Actor: "gate-test", Subject: "authorized debug", TargetRef: "target-alpha", RuntimeSeconds: 30, DiskMB: 64, Requests: 1, OutputPaths: "workspace/main/debug/session-1", StopConditions: "timeout"})
	if err != nil {
		t.Fatal(err)
	}
	if authorized.Event == nil || authorized.Event.Status != "authorized-gate" {
		t.Fatalf("expected authorized gate event: %+v", authorized)
	}

	result, err := RecordExecution(repoRoot, caseRoot, pack, Options{GateEventID: authorized.EventID, Actor: "executor-1", ExecutionStatus: "succeeded", ActualRuntimeSeconds: 25, ActualDiskMB: 32, ActualRequests: 1, OutputRefs: "workspace/main/debug/session-1/result.json", EvidenceRefs: "workspace/main/debug/session-1/result.json"})
	if err != nil {
		t.Fatal(err)
	}
	if !result.Applied || result.Path != ".rekit/facts/observations.jsonl" || result.ExecutionEvidence == nil || result.Event != nil {
		t.Fatalf("unexpected execution evidence result: %+v", result)
	}
	if result.ExecutionEvidence.Kind != "observation" || result.ExecutionEvidence.Lane != "main" || result.ExecutionEvidence.Status != "succeeded" || result.ExecutionEvidence.Execution.GateEventID != authorized.EventID || result.ExecutionEvidence.Execution.Authorization != "preauthorized" || !result.ExecutionEvidence.Execution.RecordRequired {
		t.Fatalf("unexpected execution evidence event: %+v", result.ExecutionEvidence)
	}
	if strings.Join(result.ExecutionEvidence.Related, ",") != authorized.EventID || strings.Join(result.ExecutionEvidence.Execution.OutputRefs, ",") != "workspace/main/debug/session-1/result.json" || strings.Join(result.ExecutionEvidence.EvidenceRefs, ",") != "workspace/main/debug/session-1/result.json" {
		t.Fatalf("execution evidence refs drifted: %+v", result.ExecutionEvidence)
	}
	commander := result.MissionCommanderAction
	if commander.State != "ready-for-evidence-review" || commander.PrimaryCommand != "/rekit handoff main" || !strings.Contains(commander.Prompt, authorized.EventID) || !gateContainsSubstring(commander.FollowUpCommands, "/rekit continue main -WhatIf") || !gateContainsSubstring(commander.Boundary, "bounded observation evidence") || !gateContainsSubstring(commander.Boundary, "did not execute the heavy tool") || !gateContainsSubstring(commander.Boundary, "no authority/confirmed") {
		t.Fatalf("execution evidence omitted Mission Commander review handoff: %+v", commander)
	}
	if !gateContainsSubstring(result.NextSteps, "/rekit handoff main") || !gateContainsSubstring(result.NextSteps, "Review output refs") {
		t.Fatalf("execution evidence next steps omitted handoff/review guidance: %+v", result.NextSteps)
	}
	assertGateActionQueue(t, result.MissionCommanderActionQueue, 5, 5, 0, 3, 3, "/rekit handoff main")
	if result.ExecutionEvidenceReviewSummary.Total != 1 || result.ExecutionEvidenceReviewSummary.ReadyForReviewCount != 1 || result.ExecutionEvidenceReviewSummary.MainEscalationCount != 0 || result.ExecutionEvidenceReviewSummary.OutputRefCount != 1 || result.ExecutionEvidenceReviewSummary.EvidenceRefCount != 1 || result.ExecutionEvidenceReviewSummary.LatestGateEventID != authorized.EventID || result.ExecutionEvidenceReviewSummary.LatestStatus != "succeeded" || result.ExecutionEvidenceReviewSummary.CurrentAction != "/rekit handoff main" || result.ExecutionEvidenceReviewSummary.NextActionCount != 5 || result.ExecutionEvidenceReviewSummary.ReviewRequiredActionCount != 3 || !gateContainsSubstring(result.ExecutionEvidenceReviewSummary.Boundary, "summary is read-only") || !gateContainsSubstring(result.ExecutionEvidenceReviewSummary.Boundary, "no authority/confirmed") {
		t.Fatalf("execution evidence review summary omitted compact handoff: %+v", result.ExecutionEvidenceReviewSummary)
	}
	if len(result.ExecutionEvidenceReview) != 1 || result.ExecutionEvidenceReview[0].FollowThrough.State != "ready-for-evidence-review" || !executionEvidenceFollowThroughContainsForTest(result.ExecutionEvidenceReview[0].FollowThrough, "recorded-evidence-review", "reviewed outputRefs/evidenceRefs") || result.ExecutionEvidenceReview[0].FollowThrough.ActionQueue.CurrentAction == nil || result.ExecutionEvidenceReview[0].FollowThrough.ActionQueue.CurrentAction.Command != "/rekit handoff main" || strings.Contains(result.ExecutionEvidenceReview[0].FollowThrough.ActionQueue.Summary, "/rekit continue") {
		t.Fatalf("execution evidence review omitted recorded follow-through: %+v", result.ExecutionEvidenceReview)
	}
	observed := readSingleExecutionEvidence(t, filepath.Join(caseRoot, ".rekit", "facts", "observations.jsonl"))
	if observed.EventID != result.EventID || observed.Execution.GateEventID != authorized.EventID || observed.Gate.Authorization.Decision != "preauthorized" {
		t.Fatalf("observation ledger mismatch: %+v", observed)
	}
	assertGateNotExists(t, filepath.Join(caseRoot, ".rekit", "facts", "authority.jsonl"))
	assertGateNotExists(t, filepath.Join(caseRoot, ".rekit", "facts", "confirmed.jsonl"))
}

func TestRecordExecutionDuplicateDoesNotAppend(t *testing.T) {
	repoRoot, caseRoot, pack := gateFixture(t)
	writePreauthorizedProfile(t, caseRoot)
	authorized, err := Apply(repoRoot, caseRoot, pack, Options{Action: "debug", Lane: "main", Actor: "gate-test", Subject: "authorized debug", TargetRef: "target-alpha", RuntimeSeconds: 30, DiskMB: 64, Requests: 1, OutputPaths: "workspace/main/debug/session-1", StopConditions: "timeout"})
	if err != nil {
		t.Fatal(err)
	}
	opt := Options{GateEventID: authorized.EventID, Actor: "executor-1", ExecutionStatus: "succeeded", ActualRuntimeSeconds: 25, ActualDiskMB: 32, ActualRequests: 1, OutputRefs: "workspace/main/debug/session-1/result.json"}
	first, err := RecordExecution(repoRoot, caseRoot, pack, opt)
	if err != nil {
		t.Fatal(err)
	}
	second, err := RecordExecution(repoRoot, caseRoot, pack, opt)
	if err != nil {
		t.Fatal(err)
	}
	if !first.Applied || second.Applied || second.EventID != first.EventID || second.Reason != "duplicate eventId" {
		t.Fatalf("unexpected duplicate execution results: first=%+v second=%+v", first, second)
	}
	if second.MissionCommanderAction.State != "evidence-already-recorded" || !gateContainsSubstring(second.MissionCommanderAction.Boundary, "did not append observation evidence") || gateContainsSubstring(second.MissionCommanderAction.FollowUpCommands, "-Apply") || gateContainsSubstring(second.MissionCommanderAction.FollowUpCommands, "/rekit continue") {
		t.Fatalf("duplicate execution evidence omitted idempotent Mission Commander handoff: %+v", second.MissionCommanderAction)
	}
	if len(second.ExecutionEvidenceReview) != 1 || second.ExecutionEvidenceReview[0].MissionCommanderAction.State != "evidence-already-recorded" || !gateContainsSubstring(second.ExecutionEvidenceReview[0].MissionCommanderAction.Boundary, "did not append observation evidence") || second.ExecutionEvidenceReview[0].FollowThrough.State != "evidence-already-recorded" || !executionEvidenceFollowThroughContainsForTest(second.ExecutionEvidenceReview[0].FollowThrough, "duplicate-record-review", "does not append observation evidence") || strings.Contains(second.ExecutionEvidenceReview[0].FollowThrough.ActionQueue.Summary, "/rekit continue") {
		t.Fatalf("duplicate execution evidence review omitted idempotent follow-through: %+v", second.ExecutionEvidenceReview)
	}
	if len(second.MissionCommanderNextActions) != 2 || second.MissionCommanderNextActions[0].State != "evidence-already-recorded" || second.MissionCommanderNextActions[0].Source != "executionEvidenceReview" || second.MissionCommanderNextActions[0].Command != "/rekit handoff main" || second.MissionCommanderNextActions[1].Command != "/rekit overview" || gateNextActionContainsCommand(second.MissionCommanderNextActions, "/rekit continue") || gateNextActionContainsSource(second.MissionCommanderNextActions, "missionCommanderActions") || !gateNextActionBoundaryContains(second.MissionCommanderNextActions, "did not append observation evidence") {
		t.Fatalf("duplicate execution evidence next actions should be review-only and idempotent: %+v", second.MissionCommanderNextActions)
	}
	assertGateActionQueue(t, second.MissionCommanderActionQueue, 2, 2, 0, 2, 1, "/rekit handoff main")
	if second.ExecutionEvidenceReviewSummary.Total != 1 || second.ExecutionEvidenceReviewSummary.DuplicateCount != 1 || second.ExecutionEvidenceReviewSummary.ReadyForReviewCount != 1 || second.ExecutionEvidenceReviewSummary.LatestCommanderState != "evidence-already-recorded" || second.ExecutionEvidenceReviewSummary.NextActionCount != 2 || second.ExecutionEvidenceReviewSummary.ReviewRequiredActionCount != 2 || second.ExecutionEvidenceReviewSummary.CurrentAction != "/rekit handoff main" {
		t.Fatalf("duplicate execution evidence review summary omitted idempotent handoff: %+v", second.ExecutionEvidenceReviewSummary)
	}
	lines := readGateLines(t, filepath.Join(caseRoot, ".rekit", "facts", "observations.jsonl"))
	if len(lines) != 1 {
		t.Fatalf("duplicate execution evidence wrote %d lines, want 1: %q", len(lines), lines)
	}
}

func TestRecordExecutionRejectsPendingGate(t *testing.T) {
	repoRoot, caseRoot, pack := gateFixture(t)
	pending, err := Apply(repoRoot, caseRoot, pack, Options{Action: "debug", Lane: "main", Actor: "gate-test", Subject: "pending debug"})
	if err != nil {
		t.Fatal(err)
	}
	_, err = RecordExecution(repoRoot, caseRoot, pack, Options{GateEventID: pending.EventID, Actor: "executor-1", ExecutionStatus: "succeeded", OutputRefs: "workspace/main/debug/result.json"})
	if err == nil || !strings.Contains(err.Error(), "requires an authorized-gate request") {
		t.Fatalf("RecordExecution error = %v, want authorized-gate rejection", err)
	}
	assertGateNotExists(t, filepath.Join(caseRoot, ".rekit", "facts", "observations.jsonl"))
}

func TestAdapterReportContractDescribesAuthorizedGateBoundaries(t *testing.T) {
	repoRoot, caseRoot, pack := gateFixture(t)
	writePreauthorizedProfile(t, caseRoot)
	authorized, err := Apply(repoRoot, caseRoot, pack, Options{Action: "debug", Lane: "main", Actor: "gate-test", Subject: "authorized debug", TargetRef: "target-alpha", RuntimeSeconds: 30, DiskMB: 64, Requests: 1, OutputPaths: "workspace/main/debug/session-1", StopConditions: "timeout"})
	if err != nil {
		t.Fatal(err)
	}
	contract, err := AdapterReportContract(repoRoot, caseRoot, pack, Options{GateEventID: authorized.EventID})
	if err != nil {
		t.Fatal(err)
	}
	if contract.Kind != "adapter-execution-report-contract" || contract.GateEventID != authorized.EventID || contract.Action != "debug" || contract.ReportKind != "adapter-execution-report" || contract.ReportSchemaVersion != 1 {
		t.Fatalf("unexpected adapter report contract identity: %+v", contract)
	}
	if strings.Join(contract.AllowedStatuses, ",") != "succeeded,failed,boundary-hit,escalated,aborted" || strings.Join(contract.AllowedOutputPaths, ",") != "workspace/main/debug/session-1" || contract.DefaultReportPath != "workspace/main/debug/session-1/adapter-report.json" || contract.AuthorizedBudget.RuntimeSeconds != 30 {
		t.Fatalf("adapter report contract omitted authorized boundaries: %+v", contract)
	}
	if strings.Join(contract.StopConditions, ",") != "timeout" || contract.SummaryMaxBytes != 4096 || contract.EscalationMaxBytes != 4096 || !contract.RecordRequired || !strings.Contains(strings.Join(contract.DeniedActions, ","), "heavy-tool execution") || !strings.Contains(contract.ReportPathRule, "current-workspace relative") || !strings.Contains(strings.Join(contract.RefPathRequires, ","), "evidenceRefs must stay under authorized outputPaths") {
		t.Fatalf("adapter report contract omitted live validation rules: %+v", contract)
	}
	if !strings.Contains(strings.Join(contract.BoundaryStatusRequires, ","), "authorized stopConditions") || !strings.Contains(strings.Join(contract.StatusSummaryRequires, ","), "failed/boundary-hit/escalated/aborted") {
		t.Fatalf("adapter report contract omitted status enforcement rules: %+v", contract)
	}
	follow := contract.AuthorizedExecutionFollowThrough
	if follow.State != "needs-adapter-report-validation" || follow.GateEventID != authorized.EventID || follow.ReportPath != "workspace/main/debug/session-1/adapter-report.json" || len(follow.Outcomes) != 3 || follow.ActionQueue.Summary != contract.MissionCommanderActionQueue.Summary {
		t.Fatalf("adapter report contract omitted lifecycle follow-through: %+v", follow)
	}
	if !authorizedFollowThroughContainsForTest(follow, "write-and-validate-report", "run read-only validation") || !authorizedFollowThroughContainsForTest(follow, "valid-report-record", "validated sidecar becomes bounded observation evidence") || !authorizedFollowThroughContainsForTest(follow, "invalid-report-repair", "record remains blocked") || !authorizedFollowThroughContainsForTest(follow, "valid-report-record", "do not replay heavy tool") || !gateContainsSubstring(follow.Boundary, "guidance only") {
		t.Fatalf("adapter report contract follow-through omitted lifecycle outcomes: %+v", follow)
	}
	if !strings.Contains(contract.LiveValidation.InvocationCwd, "authorizedWorkspaces") || !strings.Contains(contract.LiveValidation.InvocationCwd, "caseRelativeReportPath") || strings.Join(contract.LiveValidation.AuthorizedWorkspaces, ",") != "workspace/main/debug/session-1" || contract.LiveValidation.ReportFileName != "adapter-report.json" || contract.LiveValidation.CaseRelativeReportPath != "workspace/main/debug/session-1/adapter-report.json" || contract.LiveValidation.SidecarTemplate.Action != "debug" || contract.LiveValidation.SidecarTemplate.GateEventID != authorized.EventID || contract.LiveValidation.SidecarTemplate.Kind != "adapter-execution-report" || !strings.Contains(strings.Join(contract.LiveValidation.SidecarTemplate.EvidenceRefs, ","), "authorized outputPaths") || !strings.Contains(strings.Join(contract.LiveValidation.Notes, ","), "outputRefs/evidenceRefs") || !strings.Contains(strings.Join(contract.LiveValidation.Notes, ","), "omit runnable RecordArgs/CaseRelativeRecordArgs") || !strings.Contains(contract.LiveValidation.ReplayBehavior, "hash-bound record command") || !strings.Contains(contract.LiveValidation.ReplayBehavior, "duplicate eventId") {
		t.Fatalf("adapter report contract omitted live-validation handoff: %+v", contract.LiveValidation)
	}
	if len(contract.LiveValidation.AdapterCandidates) != 0 {
		t.Fatalf("fixture without tooling catalog should not invent adapter candidates: %+v", contract.LiveValidation.AdapterCandidates)
	}
	if strings.Join(contract.LiveValidation.ScaffoldArgs, " ") != "-Command gate -Pack "+pack+" -GateEventId "+authorized.EventID+" -ScaffoldExecutionReport -ExecutionReportPath adapter-report.json -Format json" {
		t.Fatalf("adapter report contract scaffold args drifted: %+v", contract.LiveValidation.ScaffoldArgs)
	}
	if contract.LiveValidation.ScaffoldCommand != "rekit "+strings.Join(contract.LiveValidation.ScaffoldArgs, " ") {
		t.Fatalf("adapter report contract scaffold command drifted: %q", contract.LiveValidation.ScaffoldCommand)
	}
	if len(contract.LiveValidation.ScaffoldApplyArgs) != 14 || strings.Join(contract.LiveValidation.ScaffoldApplyArgs[:10], " ") != "-Command gate -Pack "+pack+" -GateEventId "+authorized.EventID+" -ScaffoldExecutionReport -ExecutionReportPath adapter-report.json -ExpectedExecutionReportSha256" || contract.LiveValidation.ScaffoldApplyArgs[11] != "-Apply" || strings.Join(contract.LiveValidation.ScaffoldApplyArgs[12:], " ") != "-Format json" {
		t.Fatalf("adapter report contract scaffold apply args drifted: %+v", contract.LiveValidation.ScaffoldApplyArgs)
	}
	if contract.LiveValidation.SidecarTemplateSHA256 == "" || contract.LiveValidation.ScaffoldApplyArgs[10] != contract.LiveValidation.SidecarTemplateSHA256 || !strings.Contains(contract.LiveValidation.ScaffoldApplyCommand, contract.LiveValidation.SidecarTemplateSHA256) {
		t.Fatalf("adapter report contract scaffold hash handoff drifted: hash=%q args=%+v command=%q", contract.LiveValidation.SidecarTemplateSHA256, contract.LiveValidation.ScaffoldApplyArgs, contract.LiveValidation.ScaffoldApplyCommand)
	}
	if strings.Join(contract.LiveValidation.ValidateArgs, " ") != "-Command gate -Pack "+pack+" -GateEventId "+authorized.EventID+" -ValidateExecutionReport -ExecutionReportPath adapter-report.json -Format json" {
		t.Fatalf("adapter report contract validate args drifted: %+v", contract.LiveValidation.ValidateArgs)
	}
	if contract.LiveValidation.ValidateCommand != "rekit "+strings.Join(contract.LiveValidation.ValidateArgs, " ") {
		t.Fatalf("adapter report contract validate command drifted: %q", contract.LiveValidation.ValidateCommand)
	}
	if len(contract.LiveValidation.RecordArgs) != 0 || contract.LiveValidation.RecordCommand != "" {
		t.Fatalf("adapter report contract should omit pre-validation record command: args=%+v command=%q", contract.LiveValidation.RecordArgs, contract.LiveValidation.RecordCommand)
	}
	if strings.Join(contract.LiveValidation.CaseRelativeScaffoldArgs, " ") != "-Command gate -Pack "+pack+" -GateEventId "+authorized.EventID+" -ScaffoldExecutionReport -ExecutionReportPath workspace/main/debug/session-1/adapter-report.json -Format json" {
		t.Fatalf("adapter report contract case-relative scaffold args drifted: %+v", contract.LiveValidation.CaseRelativeScaffoldArgs)
	}
	if contract.LiveValidation.CaseRelativeScaffoldCommand != "rekit "+strings.Join(contract.LiveValidation.CaseRelativeScaffoldArgs, " ") {
		t.Fatalf("adapter report contract case-relative scaffold command drifted: %q", contract.LiveValidation.CaseRelativeScaffoldCommand)
	}
	if len(contract.LiveValidation.CaseRelativeScaffoldApplyArgs) != 14 || strings.Join(contract.LiveValidation.CaseRelativeScaffoldApplyArgs[:10], " ") != "-Command gate -Pack "+pack+" -GateEventId "+authorized.EventID+" -ScaffoldExecutionReport -ExecutionReportPath workspace/main/debug/session-1/adapter-report.json -ExpectedExecutionReportSha256" || contract.LiveValidation.CaseRelativeScaffoldApplyArgs[10] != contract.LiveValidation.SidecarTemplateSHA256 || contract.LiveValidation.CaseRelativeScaffoldApplyArgs[11] != "-Apply" || strings.Join(contract.LiveValidation.CaseRelativeScaffoldApplyArgs[12:], " ") != "-Format json" {
		t.Fatalf("adapter report contract case-relative scaffold apply args drifted: %+v", contract.LiveValidation.CaseRelativeScaffoldApplyArgs)
	}
	if contract.LiveValidation.CaseRelativeScaffoldApplyCommand != "rekit "+strings.Join(contract.LiveValidation.CaseRelativeScaffoldApplyArgs, " ") {
		t.Fatalf("adapter report contract case-relative scaffold apply command drifted: %q", contract.LiveValidation.CaseRelativeScaffoldApplyCommand)
	}
	if strings.Join(contract.LiveValidation.CaseRelativeValidateArgs, " ") != "-Command gate -Pack "+pack+" -GateEventId "+authorized.EventID+" -ValidateExecutionReport -ExecutionReportPath workspace/main/debug/session-1/adapter-report.json -Format json" {
		t.Fatalf("adapter report contract case-relative validate args drifted: %+v", contract.LiveValidation.CaseRelativeValidateArgs)
	}
	if contract.LiveValidation.CaseRelativeValidateCommand != "rekit "+strings.Join(contract.LiveValidation.CaseRelativeValidateArgs, " ") {
		t.Fatalf("adapter report contract case-relative validate command drifted: %q", contract.LiveValidation.CaseRelativeValidateCommand)
	}
	if len(contract.LiveValidation.CaseRelativeRecordArgs) != 0 || contract.LiveValidation.CaseRelativeRecordCommand != "" {
		t.Fatalf("adapter report contract should omit pre-validation case-relative record command: args=%+v command=%q", contract.LiveValidation.CaseRelativeRecordArgs, contract.LiveValidation.CaseRelativeRecordCommand)
	}
	commander := contract.MissionCommanderAction
	wantValidate := "/rekit gate -Pack " + pack + " -GateEventId " + authorized.EventID + " -ValidateExecutionReport -ExecutionReportPath workspace/main/debug/session-1/adapter-report.json -Format json"
	if commander.State != "needs-adapter-report-validation" || commander.PrimaryCommand != wantValidate || !strings.Contains(commander.Prompt, authorized.EventID) || !strings.Contains(commander.Prompt, "valid=true") {
		t.Fatalf("adapter report contract omitted Mission Commander validation handoff: %+v", commander)
	}
	if gateContainsSubstring(commander.FollowUpCommands, "-Apply") || !gateContainsSubstring(commander.FollowUpCommands, "/rekit handoff main") || !gateContainsSubstring(commander.Boundary, "read-only") || !gateContainsSubstring(commander.Boundary, "bounded observation evidence") || !gateContainsSubstring(commander.Boundary, "never executes the heavy tool") || !gateContainsSubstring(commander.Boundary, "no authority/confirmed") || !gateContainsSubstring(commander.Boundary, "does not provide a runnable record Apply") {
		t.Fatalf("adapter report contract omitted Mission Commander follow-up boundaries: %+v", commander)
	}
	if len(contract.MissionCommanderNextActions) != 2 || contract.MissionCommanderNextActions[0].State != "needs-adapter-report-validation" || contract.MissionCommanderNextActions[0].Source != "adapterReportContract.missionCommanderAction" || contract.MissionCommanderNextActions[0].Command != wantValidate || contract.MissionCommanderNextActions[1].Command != "/rekit handoff main" || gateNextActionContainsCommand(contract.MissionCommanderNextActions, "-Apply") || !gateNextActionBoundaryContains(contract.MissionCommanderNextActions, "does not provide a runnable record Apply") {
		t.Fatalf("adapter report contract omitted Mission Commander next actions: %+v", contract.MissionCommanderNextActions)
	}
	assertGateActionQueue(t, contract.MissionCommanderActionQueue, 2, 2, 0, 2, 1, wantValidate)
	summary := contract.ReportSummary
	if summary.State != "needs-adapter-report-validation" || summary.GateEventID != authorized.EventID || summary.Action != "debug" || summary.Lane != "main" || summary.ReportPath != "workspace/main/debug/session-1/adapter-report.json" || summary.DefaultReportPath != "workspace/main/debug/session-1/adapter-report.json" || summary.ReportPresent || summary.Valid || summary.RecordReady || !summary.RecordBlocked || !summary.RequiresValidation || summary.RequiresRepair || summary.RequiresMainEscalation || summary.AllowedStatusCount != 5 || summary.AllowedOutputPathCount != 1 || summary.AuthorizedStopCount != 1 || summary.RepairHintCount != len(contract.ValidationRepairHints) || summary.RecordBlockedHintCount != len(contract.ValidationRepairHints) || summary.EscalateHintCount != 1 || summary.OutcomeCount != 3 || summary.NextActionCount != 2 || summary.ReviewRequiredActionCount != 2 || summary.CurrentAction != wantValidate || summary.ActionQueueSummary != contract.MissionCommanderActionQueue.Summary || !gateContainsSubstring(summary.Boundary, "summary is read-only") {
		t.Fatalf("adapter report contract omitted compact summary: %+v", summary)
	}
	if !gateContainsSubstring(contract.NextSteps, wantValidate) || !gateContainsSubstring(contract.NextSteps, "hash-bound record command") || !gateContainsSubstring(contract.NextSteps, "valid=true") || !gateContainsSubstring(contract.NextSteps, "never executes the heavy tool") {
		t.Fatalf("adapter report contract omitted concrete Mission Commander next steps: %+v", contract.NextSteps)
	}
	stages := map[string]bool{}
	for _, stage := range contract.ValidationFailureStages {
		stages[stage.Stage] = stage.Description != ""
	}
	for _, stage := range []string{"path", "read", "decode", "schema", "identity", "refs", "budget", "boundary", "summary"} {
		if !stages[stage] {
			t.Fatalf("adapter report contract omitted validation failure stage %q: %+v", stage, contract.ValidationFailureStages)
		}
	}
	codes := map[string]string{}
	for _, code := range contract.ValidationFailureCodes {
		if code.Description == "" {
			t.Fatalf("adapter report contract omitted failure code description: %+v", code)
		}
		codes[code.Code] = code.Stage
	}
	for code, stage := range map[string]string{"report-path-out-of-scope": "path", "report-json-invalid": "decode", "gate-event-mismatch": "identity", "output-refs-out-of-scope": "refs", "evidence-refs-out-of-scope": "refs", "budget-marker-missing": "budget", "boundary-marker-missing": "boundary", "boundary-hits-not-authorized": "boundary", "status-summary-missing": "summary"} {
		if codes[code] != stage {
			t.Fatalf("adapter report contract failure code %q stage = %q, want %q; codes=%+v", code, codes[code], stage, contract.ValidationFailureCodes)
		}
	}
	hints := map[string]AdapterReportRepairHint{}
	for _, hint := range contract.ValidationRepairHints {
		if hint.RepairAction == "" || !hint.RecordBlocked || !hint.RerunValidation || hint.Detail == "" {
			t.Fatalf("adapter report contract omitted stable repair hint fields: %+v", hint)
		}
		hints[hint.Code] = hint
	}
	for code, action := range map[string]string{"": "provide-execution-report-path", "report-path-out-of-scope": "move-report-under-authorized-output-path", "report-json-invalid": "fix-report-json", "gate-event-mismatch": "match-authorized-gate-event", "evidence-refs-out-of-scope": "move-evidence-refs-under-authorized-output-paths", "boundary-marker-missing": "add-boundary-marker", "status-summary-missing": "add-required-status-summary"} {
		hint, ok := hints[code]
		if !ok {
			t.Fatalf("adapter report contract omitted repair hint %q: %+v", code, contract.ValidationRepairHints)
		}
		if hint.RepairAction != action {
			t.Fatalf("adapter report contract repair hint %q action = %q, want %q; hints=%+v", code, hint.RepairAction, action, contract.ValidationRepairHints)
		}
	}
	if strings.Join(hints["report-path-out-of-scope"].AllowedOutputPaths, ",") != "workspace/main/debug/session-1" || strings.Join(hints["boundary-marker-missing"].AllowedStopConditions, ",") != "timeout" || hints["status-summary-missing"].MaxBytes != 4096 {
		t.Fatalf("adapter report contract repair hints omitted boundaries: %+v", contract.ValidationRepairHints)
	}
	assertGateNotExists(t, filepath.Join(caseRoot, ".rekit", "facts", "observations.jsonl"))
	assertGateNotExists(t, filepath.Join(caseRoot, ".rekit", "facts", "authority.jsonl"))
	assertGateNotExists(t, filepath.Join(caseRoot, ".rekit", "facts", "confirmed.jsonl"))
}

func TestAdapterReportContractProjectsPackToolingCandidateOperationalClosure(t *testing.T) {
	repoRoot, caseRoot, pack := gateToolingFixture(t)
	writePreauthorizedProfile(t, caseRoot)
	authorized, err := Apply(repoRoot, caseRoot, pack, Options{Action: "debug", Lane: "main", Actor: "gate-test", Subject: "authorized pack tooling debug", TargetRef: "target-alpha", RuntimeSeconds: 30, DiskMB: 64, Requests: 1, OutputPaths: "workspace/main/debug/session-1", StopConditions: "timeout"})
	if err != nil {
		t.Fatal(err)
	}

	contract, err := AdapterReportContract(repoRoot, caseRoot, pack, Options{GateEventID: authorized.EventID})
	if err != nil {
		t.Fatal(err)
	}
	if len(contract.LiveValidation.AdapterCandidates) != 1 {
		t.Fatalf("adapter report contract should project the concrete pack tooling candidate: %+v", contract.LiveValidation.AdapterCandidates)
	}
	candidate := contract.LiveValidation.AdapterCandidates[0]
	if candidate.ID != "dynamic-debug-or-writeback-action" || candidate.Status != "cautious" || candidate.ToolingCatalogPath != "tooling/catalog.yml" || strings.Join(candidate.GateActions, ",") != "debug" || !candidate.RecordOnlyAfterGate {
		t.Fatalf("unexpected adapter tooling candidate identity: %+v", candidate)
	}
	if !strings.Contains(candidate.Entry, "dynamic-debug-or-writeback-action") || !strings.Contains(candidate.Purpose, "bounded debug") || !strings.Contains(strings.Join(candidate.SideEffects, ","), "debugger state") || !strings.Contains(strings.Join(candidate.ReportGuidance, ","), "adapterId") || !strings.Contains(strings.Join(candidate.EvidenceGuidance, ","), "ValidateArgs") || strings.Join(candidate.StopConditionHints, ",") != "timeout,unexpected-side-effect,scope-drift" {
		t.Fatalf("adapter tooling candidate omitted operational handoff guidance: %+v", candidate)
	}
	if contract.LiveValidation.SidecarTemplate.AdapterID != candidate.ID || contract.LiveValidation.SelectedAdapter == nil || contract.LiveValidation.SelectedAdapter.ID != candidate.ID || contract.LiveValidation.SidecarTemplate.Action != "debug" {
		t.Fatalf("adapter sidecar template drifted: %+v", contract.LiveValidation)
	}

	reportPath := filepath.Join(caseRoot, "workspace", "main", "debug", "session-1", "adapter-report.json")
	writeGateText(t, reportPath, `{
  "schemaVersion": 1,
  "kind": "adapter-execution-report",
  "adapterId": "dynamic-debug-or-writeback-action",
  "action": "debug",
  "status": "succeeded",
  "gateEventId": "`+authorized.EventID+`",
  "actualBudget": {"runtimeSeconds": 24, "diskMB": 33, "requests": 1},
  "outputRefs": ["workspace/main/debug/session-1/result.json"],
  "evidenceRefs": ["workspace/main/debug/session-1/result.json"],
  "summary": "Adapter completed bounded debug handoff without running heavy tool through rekit"
}`)
	writeGateText(t, filepath.Join(caseRoot, "workspace", "main", "debug", "session-1", "result.json"), `{"ok":true}`)
	validation, err := ValidateAdapterExecutionReport(repoRoot, caseRoot, pack, Options{GateEventID: authorized.EventID, ExecutionReportPath: reportPath})
	if err != nil {
		t.Fatal(err)
	}
	if !validation.Valid || validation.IsMutation || validation.Applied || validation.Report == nil || validation.Report.AdapterID != candidate.ID || len(validation.Contract.LiveValidation.AdapterCandidates) != 1 || validation.AdapterContext == nil || len(validation.AdapterContext.Candidates) != 1 || validation.AdapterContext.Selected == nil || validation.AdapterContext.Selected.ID != candidate.ID {
		t.Fatalf("adapter candidate sidecar should validate read-only before record: %+v", validation)
	}
	assertGateNotExists(t, filepath.Join(caseRoot, ".rekit", "facts", "observations.jsonl"))

	recorded, err := RecordExecution(repoRoot, caseRoot, pack, Options{GateEventID: authorized.EventID, Actor: "executor-1", ExecutionReportPath: reportPath})
	if err != nil {
		t.Fatal(err)
	}
	if !recorded.Applied || recorded.ExecutionEvidence == nil || recorded.ExecutionEvidence.Execution.Adapter == nil || recorded.ExecutionEvidence.Execution.Adapter.AdapterID != candidate.ID || recorded.ExecutionEvidence.Execution.AdapterContext == nil || recorded.ExecutionEvidence.Execution.AdapterContext.ID != candidate.ID || recorded.ExecutionEvidence.Execution.ExecutionReportPath != "workspace/main/debug/session-1/adapter-report.json" {
		t.Fatalf("adapter candidate sidecar should record bounded observation evidence: %+v", recorded)
	}
	assertGateNotExists(t, filepath.Join(caseRoot, ".rekit", "facts", "authority.jsonl"))
	assertGateNotExists(t, filepath.Join(caseRoot, ".rekit", "facts", "confirmed.jsonl"))
}

func TestScaffoldAdapterExecutionReportPreviewApplyAndReplay(t *testing.T) {
	repoRoot, caseRoot, pack := gateFixture(t)
	writePreauthorizedProfile(t, caseRoot)
	authorized, err := Apply(repoRoot, caseRoot, pack, Options{Action: "debug", Lane: "main", Actor: "gate-test", Subject: "authorized debug", TargetRef: "target-alpha", RuntimeSeconds: 30, DiskMB: 64, Requests: 1, OutputPaths: "workspace/main/debug/session-1", StopConditions: "timeout"})
	if err != nil {
		t.Fatal(err)
	}
	reportRel := "workspace/main/debug/session-1/adapter-report.json"
	reportFull := filepath.Join(caseRoot, filepath.FromSlash(reportRel))

	preview, err := ScaffoldAdapterExecutionReport(repoRoot, caseRoot, pack, Options{GateEventID: authorized.EventID})
	if err != nil {
		t.Fatal(err)
	}
	if preview.Kind != "adapter-execution-report-scaffold" || preview.GateEventID != authorized.EventID || preview.ReportPath != reportRel || preview.IsMutation || preview.Applied || preview.Mode != "preview" || preview.ReportSHA256 == "" || !preview.RequiresConfirmation || preview.SidecarTemplate.GateEventID != authorized.EventID || preview.SidecarTemplate.Action != "debug" || !strings.Contains(preview.ApplyCommand, "-ExpectedExecutionReportSha256 "+preview.ReportSHA256) {
		t.Fatalf("unexpected scaffold preview: %+v", preview)
	}
	if preview.MissionCommanderAction.State != "ready-for-adapter-report-scaffold-apply" || preview.MissionCommanderAction.PrimaryCommand != preview.ApplyCommand || !gateNextActionContainsSource(preview.MissionCommanderNextActions, "adapterReportScaffold.preview") || !gateNextActionContainsCommand(preview.MissionCommanderNextActions, preview.ValidateCommand) || !gateNextActionBoundaryContains(preview.MissionCommanderNextActions, "does not execute the adapter") {
		t.Fatalf("scaffold preview omitted Mission Commander handoff: action=%+v next=%+v", preview.MissionCommanderAction, preview.MissionCommanderNextActions)
	}
	assertGateNotExists(t, reportFull)
	assertGateNotExists(t, filepath.Join(caseRoot, ".rekit", "facts", "observations.jsonl"))
	assertGateNotExists(t, filepath.Join(caseRoot, ".rekit", "facts", "authority.jsonl"))
	assertGateNotExists(t, filepath.Join(caseRoot, ".rekit", "facts", "confirmed.jsonl"))

	applied, err := ScaffoldAdapterExecutionReport(repoRoot, caseRoot, pack, Options{GateEventID: authorized.EventID, ExpectedExecutionReportSHA256: preview.ReportSHA256})
	if err != nil {
		t.Fatal(err)
	}
	if !applied.IsMutation || !applied.Applied || applied.Mode != "scaffolded" || applied.AlreadyExists || applied.RequiresConfirmation || applied.ReportSHA256 != preview.ReportSHA256 || applied.MissionCommanderAction.State != "adapter-report-scaffolded-awaiting-adapter-output" || applied.MissionCommanderAction.PrimaryCommand != applied.ValidateCommand {
		t.Fatalf("unexpected scaffold apply result: %+v", applied)
	}
	writtenBytes, err := os.ReadFile(reportFull)
	if err != nil {
		t.Fatal(err)
	}
	var written AdapterReportSidecarTemplate
	if err := json.Unmarshal(writtenBytes, &written); err != nil {
		t.Fatalf("scaffolded report did not decode: %v\n%s", err, string(writtenBytes))
	}
	if written.Kind != "adapter-execution-report" || written.GateEventID != authorized.EventID || written.Action != "debug" || written.Status != "succeeded|failed|boundary-hit|escalated|aborted" || !strings.Contains(strings.Join(written.OutputRefs, ","), "authorized outputPaths") {
		t.Fatalf("scaffolded sidecar template drifted: %+v", written)
	}
	assertGateNotExists(t, filepath.Join(caseRoot, ".rekit", "facts", "observations.jsonl"))
	assertGateNotExists(t, filepath.Join(caseRoot, ".rekit", "facts", "authority.jsonl"))
	assertGateNotExists(t, filepath.Join(caseRoot, ".rekit", "facts", "confirmed.jsonl"))

	replay, err := ScaffoldAdapterExecutionReport(repoRoot, caseRoot, pack, Options{GateEventID: authorized.EventID, ExpectedExecutionReportSHA256: preview.ReportSHA256})
	if err != nil {
		t.Fatal(err)
	}
	if !replay.IsMutation || replay.Applied || !replay.AlreadyExists || replay.Mode != "already-scaffolded" || replay.ApplyCommand != "" || replay.RequiresConfirmation {
		t.Fatalf("unexpected exact scaffold replay: %+v", replay)
	}
	if _, err := ScaffoldAdapterExecutionReport(repoRoot, caseRoot, pack, Options{GateEventID: authorized.EventID, ExpectedExecutionReportSHA256: strings.Repeat("0", 64)}); err == nil || !strings.Contains(err.Error(), "template changed after preview") {
		t.Fatalf("hash-gated scaffold replay error = %v, want template changed", err)
	}
	writeGateText(t, filepath.Join(caseRoot, "workspace", "main", "debug", "session-1", "custom-report.json"), `{"different":true}`)
	_, err = ScaffoldAdapterExecutionReport(repoRoot, caseRoot, pack, Options{GateEventID: authorized.EventID, ExecutionReportPath: "workspace/main/debug/session-1/custom-report.json"})
	if err == nil || !strings.Contains(err.Error(), "target already exists with different bytes") {
		t.Fatalf("different existing scaffold error = %v, want refusal", err)
	}
}

func TestDraftAdapterExecutionReportPreviewApplyReplayAndScaffoldReplace(t *testing.T) {
	repoRoot, caseRoot, pack := gateFixture(t)
	writePreauthorizedProfile(t, caseRoot)
	authorized, err := Apply(repoRoot, caseRoot, pack, Options{Action: "debug", Lane: "main", Actor: "gate-test", Subject: "authorized debug", TargetRef: "target-alpha", RuntimeSeconds: 30, DiskMB: 64, Requests: 1, OutputPaths: "workspace/main/debug/session-1", StopConditions: "timeout"})
	if err != nil {
		t.Fatal(err)
	}
	reportRel := "workspace/main/debug/session-1/adapter-report.json"
	reportFull := filepath.Join(caseRoot, filepath.FromSlash(reportRel))
	draftOpt := Options{GateEventID: authorized.EventID, ExecutionReportPath: reportRel, AdapterID: "unit-adapter", ExecutionStatus: "succeeded", ActualRuntimeSeconds: 24, ActualDiskMB: 33, ActualRequests: 1, OutputRefs: "workspace/main/debug/session-1/result.json", EvidenceRefs: "workspace/main/debug/session-1/evidence.json", Summary: "Adapter completed deterministic draft"}

	preview, err := DraftAdapterExecutionReport(repoRoot, caseRoot, pack, draftOpt)
	if err != nil {
		t.Fatal(err)
	}
	if preview.Kind != "adapter-execution-report-draft" || preview.GateEventID != authorized.EventID || preview.ReportPath != reportRel || preview.IsMutation || preview.Applied || preview.Mode != "preview" || preview.ReportSHA256 == "" || !preview.RequiresConfirmation || preview.AlreadyExists || preview.ReplacesScaffold || preview.Report.AdapterID != "unit-adapter" || preview.Report.Status != "succeeded" || strings.Join(preview.Report.OutputRefs, ",") != "workspace/main/debug/session-1/result.json" || strings.Join(preview.Report.EvidenceRefs, ",") != "workspace/main/debug/session-1/evidence.json" || !strings.Contains(preview.ApplyCommand, "-ExpectedExecutionReportSha256 "+preview.ReportSHA256) {
		t.Fatalf("unexpected draft preview: %+v", preview)
	}
	if preview.MissionCommanderAction.State != "ready-for-adapter-report-draft-apply" || preview.MissionCommanderAction.PrimaryCommand != preview.ApplyCommand || !gateNextActionContainsSource(preview.MissionCommanderNextActions, "adapterReportDraft.preview") || !gateNextActionContainsCommand(preview.MissionCommanderNextActions, preview.ValidateCommand) || !gateNextActionBoundaryContains(preview.MissionCommanderNextActions, "does not execute the adapter") {
		t.Fatalf("draft preview omitted Mission Commander handoff: action=%+v next=%+v", preview.MissionCommanderAction, preview.MissionCommanderNextActions)
	}
	assertGateActionQueue(t, preview.MissionCommanderActionQueue, 3, 2, 1, 3, 1, preview.ApplyCommand)
	assertGateNotExists(t, reportFull)
	assertGateNotExists(t, filepath.Join(caseRoot, ".rekit", "facts", "observations.jsonl"))
	assertGateNotExists(t, filepath.Join(caseRoot, ".rekit", "facts", "authority.jsonl"))
	assertGateNotExists(t, filepath.Join(caseRoot, ".rekit", "facts", "confirmed.jsonl"))

	if _, err := DraftAdapterExecutionReport(repoRoot, caseRoot, pack, Options{GateEventID: authorized.EventID, ExecutionReportPath: "workspace/main/debug/session-1/no-adapter.json", ExecutionStatus: "succeeded"}); err == nil || !strings.Contains(err.Error(), "requires -AdapterId") {
		t.Fatalf("draft without adapter id error = %v, want adapter id requirement", err)
	}
	if _, err := DraftAdapterExecutionReport(repoRoot, caseRoot, pack, Options{GateEventID: authorized.EventID, ExecutionReportPath: reportRel, AdapterID: "unit-adapter", ExecutionStatus: "succeeded", OutputRefs: "workspace/main/other/result.json"}); err == nil || !strings.Contains(err.Error(), "outputRefs must stay within authorized gate outputPaths") {
		t.Fatalf("draft out-of-scope output error = %v, want output boundary", err)
	}

	applyOpt := draftOpt
	applyOpt.ExpectedExecutionReportSHA256 = preview.ReportSHA256
	applied, err := DraftAdapterExecutionReport(repoRoot, caseRoot, pack, applyOpt)
	if err != nil {
		t.Fatal(err)
	}
	if !applied.IsMutation || !applied.Applied || applied.Mode != "drafted" || applied.AlreadyExists || applied.ReplacesScaffold || applied.RequiresConfirmation || applied.ReportSHA256 != preview.ReportSHA256 || applied.MissionCommanderAction.State != "adapter-report-drafted-ready-for-validation" || applied.MissionCommanderAction.PrimaryCommand != applied.ValidateCommand {
		t.Fatalf("unexpected draft apply result: %+v", applied)
	}
	writtenBytes, err := os.ReadFile(reportFull)
	if err != nil {
		t.Fatal(err)
	}
	var written AdapterReport
	if err := json.Unmarshal(writtenBytes, &written); err != nil {
		t.Fatalf("drafted report did not decode: %v\n%s", err, string(writtenBytes))
	}
	if written.Kind != "adapter-execution-report" || written.AdapterID != "unit-adapter" || written.GateEventID != authorized.EventID || written.Action != "debug" || written.Status != "succeeded" || written.ActualBudget.RuntimeSeconds != 24 || strings.Join(written.OutputRefs, ",") != "workspace/main/debug/session-1/result.json" || written.Summary != "Adapter completed deterministic draft" {
		t.Fatalf("drafted sidecar content drifted: %+v", written)
	}
	validation, err := ValidateAdapterExecutionReport(repoRoot, caseRoot, pack, Options{GateEventID: authorized.EventID, ExecutionReportPath: reportRel})
	if err != nil {
		t.Fatal(err)
	}
	if !validation.Valid || validation.Report == nil || validation.Report.AdapterID != "unit-adapter" || validation.ReportSummary.State != "ready-to-record-evidence" || !validation.ReportSummary.RecordReady {
		t.Fatalf("drafted sidecar should validate read-only: %+v", validation)
	}
	assertGateNotExists(t, filepath.Join(caseRoot, ".rekit", "facts", "observations.jsonl"))
	assertGateNotExists(t, filepath.Join(caseRoot, ".rekit", "facts", "authority.jsonl"))
	assertGateNotExists(t, filepath.Join(caseRoot, ".rekit", "facts", "confirmed.jsonl"))

	replay, err := DraftAdapterExecutionReport(repoRoot, caseRoot, pack, applyOpt)
	if err != nil {
		t.Fatal(err)
	}
	if !replay.IsMutation || replay.Applied || !replay.AlreadyExists || replay.Mode != "already-drafted" || replay.ApplyCommand != "" || replay.RequiresConfirmation {
		t.Fatalf("unexpected exact draft replay: %+v", replay)
	}
	wrongHashOpt := draftOpt
	wrongHashOpt.ExpectedExecutionReportSHA256 = strings.Repeat("0", 64)
	if _, err := DraftAdapterExecutionReport(repoRoot, caseRoot, pack, wrongHashOpt); err == nil || !strings.Contains(err.Error(), "draft changed after preview") {
		t.Fatalf("draft apply with wrong hash error = %v, want draft changed", err)
	}
	writeGateText(t, filepath.Join(caseRoot, "workspace", "main", "debug", "session-1", "custom-draft.json"), `{"different":true}`)
	_, err = DraftAdapterExecutionReport(repoRoot, caseRoot, pack, Options{GateEventID: authorized.EventID, ExecutionReportPath: "workspace/main/debug/session-1/custom-draft.json", AdapterID: "unit-adapter", ExecutionStatus: "succeeded"})
	if err == nil || !strings.Contains(err.Error(), "target already exists with different bytes") {
		t.Fatalf("different existing draft error = %v, want refusal", err)
	}

	scaffoldRel := "workspace/main/debug/session-1/scaffold-then-draft.json"
	scaffold, err := ScaffoldAdapterExecutionReport(repoRoot, caseRoot, pack, Options{GateEventID: authorized.EventID, ExecutionReportPath: scaffoldRel})
	if err != nil {
		t.Fatal(err)
	}
	if _, err := ScaffoldAdapterExecutionReport(repoRoot, caseRoot, pack, Options{GateEventID: authorized.EventID, ExecutionReportPath: scaffoldRel, ExpectedExecutionReportSHA256: scaffold.ReportSHA256}); err != nil {
		t.Fatal(err)
	}
	live, present, err := AdapterReportLiveSnapshot(repoRoot, caseRoot, pack, Options{GateEventID: authorized.EventID, ExecutionReportPath: scaffoldRel})
	if err != nil {
		t.Fatal(err)
	}
	if !present || live.Valid || live.FailureCode != "report-scaffold-placeholder" || live.ReportSummary.State != "adapter-report-scaffold-awaiting-draft" || !live.ReportSummary.ReportPresent || !live.ReportSummary.RecordBlocked || live.MissionCommanderAction.State != "adapter-report-scaffold-awaiting-draft" || !strings.Contains(live.MissionCommanderAction.PrimaryCommand, "-DraftExecutionReport") || !gateNextActionContainsSource(live.MissionCommanderNextActions, "adapterReportLiveSnapshot.scaffold.draft") {
		t.Fatalf("exact scaffold live snapshot should route to draft: %+v", live)
	}
	scaffoldDraftOpt := draftOpt
	scaffoldDraftOpt.ExecutionReportPath = scaffoldRel
	scaffoldDraftPreview, err := DraftAdapterExecutionReport(repoRoot, caseRoot, pack, scaffoldDraftOpt)
	if err != nil {
		t.Fatal(err)
	}
	if !scaffoldDraftPreview.ReplacesScaffold || scaffoldDraftPreview.AlreadyExists || scaffoldDraftPreview.Mode != "preview" {
		t.Fatalf("draft preview should allow replacing exact scaffold: %+v", scaffoldDraftPreview)
	}
	scaffoldDraftOpt.ExpectedExecutionReportSHA256 = scaffoldDraftPreview.ReportSHA256
	replaced, err := DraftAdapterExecutionReport(repoRoot, caseRoot, pack, scaffoldDraftOpt)
	if err != nil {
		t.Fatal(err)
	}
	if !replaced.Applied || replaced.ReplacesScaffold || replaced.Mode != "drafted" {
		t.Fatalf("draft apply should replace exact scaffold: %+v", replaced)
	}
	assertGateNotExists(t, filepath.Join(caseRoot, ".rekit", "facts", "observations.jsonl"))
	assertGateNotExists(t, filepath.Join(caseRoot, ".rekit", "facts", "authority.jsonl"))
	assertGateNotExists(t, filepath.Join(caseRoot, ".rekit", "facts", "confirmed.jsonl"))
}

func TestScaffoldAdapterExecutionReportAcceptsCwdRelativeAuthorizedPath(t *testing.T) {
	repoRoot, caseRoot, pack := gateFixture(t)
	writePreauthorizedProfile(t, caseRoot)
	authorized, err := Apply(repoRoot, caseRoot, pack, Options{Action: "debug", Lane: "main", Actor: "gate-test", Subject: "authorized debug", TargetRef: "target-alpha", RuntimeSeconds: 30, DiskMB: 64, Requests: 1, OutputPaths: "workspace/main/debug/session-1", StopConditions: "timeout"})
	if err != nil {
		t.Fatal(err)
	}
	workspace := filepath.Join(caseRoot, "workspace", "main", "debug", "session-1")
	preview, err := ScaffoldAdapterExecutionReport(repoRoot, caseRoot, pack, Options{GateEventID: authorized.EventID, ExecutionReportCwd: workspace, ExecutionReportPath: "adapter-report.json"})
	if err != nil {
		t.Fatal(err)
	}
	if preview.ReportPath != "workspace/main/debug/session-1/adapter-report.json" || preview.Applied || preview.IsMutation {
		t.Fatalf("cwd-relative scaffold preview drifted: %+v", preview)
	}
	applied, err := ScaffoldAdapterExecutionReport(repoRoot, caseRoot, pack, Options{GateEventID: authorized.EventID, ExecutionReportCwd: workspace, ExecutionReportPath: "adapter-report.json", ExpectedExecutionReportSHA256: preview.ReportSHA256})
	if err != nil {
		t.Fatal(err)
	}
	if !applied.Applied || applied.ReportPath != preview.ReportPath {
		t.Fatalf("cwd-relative scaffold apply drifted: %+v", applied)
	}
	if _, err := os.Stat(filepath.Join(workspace, "adapter-report.json")); err != nil {
		t.Fatalf("cwd-relative scaffold did not write sidecar: %v", err)
	}
	assertGateNotExists(t, filepath.Join(caseRoot, ".rekit", "facts", "observations.jsonl"))
	assertGateNotExists(t, filepath.Join(caseRoot, ".rekit", "facts", "authority.jsonl"))
	assertGateNotExists(t, filepath.Join(caseRoot, ".rekit", "facts", "confirmed.jsonl"))
}

func TestValidateAdapterExecutionReportReadOnlyPreflight(t *testing.T) {
	repoRoot, caseRoot, pack := gateFixture(t)
	writePreauthorizedProfile(t, caseRoot)
	authorized, err := Apply(repoRoot, caseRoot, pack, Options{Action: "debug", Lane: "main", Actor: "gate-test", Subject: "authorized debug", TargetRef: "target-alpha", RuntimeSeconds: 30, DiskMB: 64, Requests: 1, OutputPaths: "workspace/main/debug/session-1", StopConditions: "timeout"})
	if err != nil {
		t.Fatal(err)
	}
	reportPath := filepath.Join(caseRoot, "workspace", "main", "debug", "session-1", "adapter-report.json")
	writeGateText(t, reportPath, `{
  "schemaVersion": 1,
  "kind": "adapter-execution-report",
  "adapterId": "unit-adapter",
  "action": "debug",
  "status": "succeeded",
  "gateEventId": "`+authorized.EventID+`",
  "actualBudget": {"runtimeSeconds": 24, "diskMB": 33, "requests": 1},
  "outputRefs": ["workspace/main/debug/session-1/result.json"],
  "evidenceRefs": ["workspace/main/debug/session-1/evidence.json"],
  "summary": "Adapter completed bounded debug run"
}`)

	validation, err := ValidateAdapterExecutionReport(repoRoot, caseRoot, pack, Options{GateEventID: authorized.EventID, ExecutionReportPath: reportPath})
	if err != nil {
		t.Fatal(err)
	}
	if validation.Kind != "adapter-execution-report-validation" || validation.IsMutation || validation.Applied || !validation.Valid || validation.ReportPath != "workspace/main/debug/session-1/adapter-report.json" || validation.Report == nil || validation.Report.AdapterID != "unit-adapter" {
		t.Fatalf("unexpected adapter report validation result: %+v", validation)
	}
	if validation.ReportSHA256 == "" || validation.RecordExpectedReportSHA256 != validation.ReportSHA256 {
		t.Fatalf("valid adapter report validation omitted hash handoff: %+v", validation)
	}
	if validation.Contract.Kind != "adapter-execution-report-contract" || validation.Contract.GateEventID != authorized.EventID || validation.Contract.AuthorizedBudget.RuntimeSeconds != 30 {
		t.Fatalf("validation omitted adapter contract boundaries: %+v", validation.Contract)
	}
	commander := validation.MissionCommanderAction
	wantRecord := "/rekit gate -Pack " + pack + " -Apply -GateEventId " + authorized.EventID + " -ExecutionReportPath workspace/main/debug/session-1/adapter-report.json -ExpectedExecutionReportSha256 " + validation.RecordExpectedReportSHA256 + " -Actor <executor-id> -Format json"
	if commander.State != "ready-to-record-evidence" || commander.PrimaryCommand != wantRecord || !strings.Contains(commander.Prompt, "valid=true") || !gateContainsSubstring(commander.FollowUpCommands, "/rekit handoff main") {
		t.Fatalf("valid adapter report validation omitted Mission Commander record handoff: %+v", commander)
	}
	if len(validation.MissionCommanderNextActions) != 2 || validation.MissionCommanderNextActions[0].State != "ready-to-record-evidence" || validation.MissionCommanderNextActions[0].Source != "adapterReportValidation.missionCommanderAction" || validation.MissionCommanderNextActions[0].Command != wantRecord || validation.MissionCommanderNextActions[1].Command != "/rekit handoff main" || !gateNextActionBoundaryContains(validation.MissionCommanderNextActions, "replace <executor-id>") {
		t.Fatalf("valid adapter report validation omitted record next actions: %+v", validation.MissionCommanderNextActions)
	}
	assertGateActionQueue(t, validation.MissionCommanderActionQueue, 2, 2, 0, 2, 1, wantRecord)
	summary := validation.ReportSummary
	if summary.State != "ready-to-record-evidence" || summary.GateEventID != authorized.EventID || summary.Action != "debug" || summary.Lane != "main" || summary.ReportPath != "workspace/main/debug/session-1/adapter-report.json" || summary.ReportSHA256 != validation.ReportSHA256 || summary.RecordExpectedReportSHA256 != validation.RecordExpectedReportSHA256 || !summary.ReportPresent || !summary.Valid || !summary.RecordReady || summary.RecordBlocked || summary.RequiresValidation || summary.RequiresRepair || summary.RequiresMainEscalation || summary.ReportStatus != "succeeded" || summary.AdapterID != "unit-adapter" || summary.ActualRuntimeSeconds != 24 || summary.ActualDiskMB != 33 || summary.ActualRequests != 1 || summary.OutputRefCount != 1 || summary.EvidenceRefCount != 1 || summary.BoundaryHitCount != 0 || summary.HasEscalation || !summary.HasSummary || summary.OutcomeCount != 1 || summary.NextActionCount != 2 || summary.ReviewRequiredActionCount != 2 || summary.CurrentAction != wantRecord || summary.ActionQueueSummary != validation.MissionCommanderActionQueue.Summary || !gateContainsSubstring(summary.Boundary, "validation is read-only") {
		t.Fatalf("valid adapter report validation omitted compact summary: %+v", summary)
	}
	follow := validation.AuthorizedExecutionFollowThrough
	if follow.State != "ready-to-record-evidence" || follow.GateEventID != authorized.EventID || follow.ReportPath != "workspace/main/debug/session-1/adapter-report.json" || follow.ActionQueue.Summary != validation.MissionCommanderActionQueue.Summary || !authorizedFollowThroughContainsForTest(follow, "valid-report-record", "replace <executor-id>") || !authorizedFollowThroughContainsForTest(follow, "valid-report-record", "observation evidence records adapter status") {
		t.Fatalf("valid adapter report validation omitted record follow-through: %+v", follow)
	}
	if !gateContainsSubstring(commander.Boundary, "read-only") || !gateContainsSubstring(commander.Boundary, "bounded observation evidence") || !gateContainsSubstring(commander.Boundary, "never executes the heavy tool") || !gateContainsSubstring(validation.NextSteps, wantRecord) {
		t.Fatalf("valid adapter report validation omitted Mission Commander boundaries: commander=%+v next=%+v", commander, validation.NextSteps)
	}
	assertGateNotExists(t, filepath.Join(caseRoot, ".rekit", "facts", "observations.jsonl"))
	assertGateNotExists(t, filepath.Join(caseRoot, ".rekit", "facts", "authority.jsonl"))
	assertGateNotExists(t, filepath.Join(caseRoot, ".rekit", "facts", "confirmed.jsonl"))
}

func TestAdapterReportLiveSnapshotMarksMalformedSidecarPresent(t *testing.T) {
	repoRoot, caseRoot, pack := gateFixture(t)
	writePreauthorizedProfile(t, caseRoot)
	authorized, err := Apply(repoRoot, caseRoot, pack, Options{Action: "debug", Lane: "main", Actor: "gate-test", Subject: "authorized debug", TargetRef: "target-alpha", RuntimeSeconds: 30, DiskMB: 64, Requests: 1, OutputPaths: "workspace/main/debug/session-1", StopConditions: "timeout"})
	if err != nil {
		t.Fatal(err)
	}
	reportPath := filepath.Join(caseRoot, "workspace", "main", "debug", "session-1", "adapter-report.json")
	writeGateText(t, reportPath, `{not-json`)

	validation, present, err := AdapterReportLiveSnapshot(repoRoot, caseRoot, pack, Options{GateEventID: authorized.EventID, ExecutionReportPath: reportPath})
	if err != nil {
		t.Fatal(err)
	}
	if !present || validation.Valid || !validation.ReportSummary.ReportPresent || validation.ReportSummary.State != "repair-adapter-report" || validation.FailureCode != "report-json-invalid" || validation.FailureStage != "decode" {
		t.Fatalf("malformed arrived sidecar must be present and repairable: %+v", validation)
	}
}

func TestAdapterReportLiveSnapshotTracksRecordedReportIdentity(t *testing.T) {
	repoRoot, caseRoot, pack := gateFixture(t)
	writePreauthorizedProfile(t, caseRoot)
	authorized, err := Apply(repoRoot, caseRoot, pack, Options{Action: "debug", Lane: "main", Actor: "gate-test", Subject: "authorized debug", TargetRef: "target-alpha", RuntimeSeconds: 30, DiskMB: 64, Requests: 1, OutputPaths: "workspace/main/debug/session-1", StopConditions: "timeout"})
	if err != nil {
		t.Fatal(err)
	}
	reportPath := filepath.Join(caseRoot, "workspace", "main", "debug", "session-1", "adapter-report.json")
	writeReport := func(summary string, requests int) {
		writeGateText(t, reportPath, `{
  "schemaVersion": 1,
  "kind": "adapter-execution-report",
  "adapterId": "unit-adapter",
  "action": "debug",
  "status": "succeeded",
  "gateEventId": "`+authorized.EventID+`",
  "actualBudget": {"runtimeSeconds": 24, "diskMB": 33, "requests": `+fmt.Sprint(requests)+`},
  "outputRefs": ["workspace/main/debug/session-1/result.json"],
  "summary": "`+summary+`"
}`)
	}
	writeReport("first bounded execution", 1)

	observationsPath := filepath.Join(caseRoot, ".rekit", "facts", "observations.jsonl")
	live, present, err := AdapterReportLiveSnapshot(repoRoot, caseRoot, pack, Options{GateEventID: authorized.EventID, ExecutionReportPath: reportPath})
	if err != nil {
		t.Fatal(err)
	}
	if !present || !live.Valid || live.ReportSummary.State != "ready-to-record-evidence" || !live.ReportSummary.RecordReady {
		t.Fatalf("unrecorded live report should be record-ready: %+v", live)
	}
	assertGateNotExists(t, observationsPath)

	recorded, err := RecordExecution(repoRoot, caseRoot, pack, Options{GateEventID: authorized.EventID, Actor: "executor-1", ExecutionReportPath: reportPath})
	if err != nil {
		t.Fatal(err)
	}
	if !recorded.Applied {
		t.Fatalf("expected report evidence record: %+v", recorded)
	}
	live, present, err = AdapterReportLiveSnapshot(repoRoot, caseRoot, pack, Options{GateEventID: authorized.EventID, ExecutionReportPath: reportPath})
	if err != nil {
		t.Fatal(err)
	}
	if !present || live.ReportSummary.State != "evidence-already-recorded" || live.ReportSummary.RecordReady || !live.ReportSummary.RecordBlocked || live.ReportSummary.RequiresValidation || live.ReportSummary.RequiresMainEscalation || len(live.NextSteps) != 2 || !strings.Contains(live.NextSteps[0], "do not record or replay") || live.MissionCommanderAction.State != "evidence-already-recorded" || strings.Contains(live.MissionCommanderAction.PrimaryCommand, "-Apply") || len(live.MissionCommanderNextActions) != 2 || strings.Contains(live.MissionCommanderActionQueue.Summary, "-Apply") || live.AuthorizedExecutionFollowThrough.State != "evidence-already-recorded" || len(live.AuthorizedExecutionFollowThrough.Outcomes) != 1 || live.AuthorizedExecutionFollowThrough.Outcomes[0].Name != "duplicate-record-review" {
		t.Fatalf("recorded live report should route to evidence review without record queue: %+v", live)
	}

	writeReport("changed bounded execution", 0)
	live, present, err = AdapterReportLiveSnapshot(repoRoot, caseRoot, pack, Options{GateEventID: authorized.EventID, ExecutionReportPath: reportPath})
	if err != nil {
		t.Fatal(err)
	}
	if !present || live.ReportSummary.State != "ready-to-record-evidence" || !live.ReportSummary.RecordReady {
		t.Fatalf("changed same-path report must not match old evidence: %+v", live)
	}
}

func TestAdapterReportLiveSnapshotPreservesRecordedBoundaryEscalation(t *testing.T) {
	repoRoot, caseRoot, pack := gateFixture(t)
	writePreauthorizedProfile(t, caseRoot)
	authorized, err := Apply(repoRoot, caseRoot, pack, Options{Action: "debug", Lane: "main", Actor: "gate-test", Subject: "authorized debug", TargetRef: "target-alpha", RuntimeSeconds: 30, DiskMB: 64, Requests: 1, OutputPaths: "workspace/main/debug/session-1", StopConditions: "timeout"})
	if err != nil {
		t.Fatal(err)
	}
	reportPath := filepath.Join(caseRoot, "workspace", "main", "debug", "session-1", "adapter-report.json")
	writeGateText(t, reportPath, `{
  "schemaVersion": 1,
  "kind": "adapter-execution-report",
  "adapterId": "unit-adapter",
  "action": "debug",
  "status": "boundary-hit",
  "gateEventId": "`+authorized.EventID+`",
  "actualBudget": {"runtimeSeconds": 24, "diskMB": 33, "requests": 1},
  "boundaryHits": ["timeout"],
  "summary": "bounded execution stopped at timeout"
}`)
	if _, err := RecordExecution(repoRoot, caseRoot, pack, Options{GateEventID: authorized.EventID, Actor: "executor-1", ExecutionReportPath: reportPath}); err != nil {
		t.Fatal(err)
	}
	live, present, err := AdapterReportLiveSnapshot(repoRoot, caseRoot, pack, Options{GateEventID: authorized.EventID, ExecutionReportPath: reportPath})
	if err != nil {
		t.Fatal(err)
	}
	if !present || live.ReportSummary.State != "needs-main-escalation" || !live.ReportSummary.RequiresMainEscalation || live.ReportSummary.RecordReady || live.MissionCommanderAction.State != "needs-main-escalation" || !strings.Contains(live.MissionCommanderAction.Prompt, "停止") || len(live.AuthorizedExecutionFollowThrough.Outcomes) != 1 || live.AuthorizedExecutionFollowThrough.Outcomes[0].Name != "boundary-or-escalation-review" || strings.Contains(live.MissionCommanderActionQueue.Summary, "-Apply") {
		t.Fatalf("recorded boundary report must preserve main escalation: %+v", live)
	}
}

func TestValidateAdapterExecutionReportRejectsSymlinkPathComponents(t *testing.T) {
	repoRoot, caseRoot, pack := gateFixture(t)
	writePreauthorizedProfile(t, caseRoot)
	authorized, err := Apply(repoRoot, caseRoot, pack, Options{Action: "debug", Lane: "main", Actor: "gate-test", Subject: "authorized debug", TargetRef: "target-alpha", RuntimeSeconds: 30, DiskMB: 64, Requests: 1, OutputPaths: "workspace/main/debug/session-1", StopConditions: "timeout"})
	if err != nil {
		t.Fatal(err)
	}
	report := `{
  "schemaVersion": 1,
  "kind": "adapter-execution-report",
  "adapterId": "unit-adapter",
  "action": "debug",
  "status": "succeeded",
  "gateEventId": "` + authorized.EventID + `",
  "actualBudget": {"runtimeSeconds": 24, "diskMB": 33, "requests": 1},
  "summary": "bounded execution"
}`
	realPath := filepath.Join(caseRoot, "workspace", "main", "debug", "session-1", "real-report.json")
	writeGateText(t, realPath, report)
	leafLink := filepath.Join(caseRoot, "workspace", "main", "debug", "session-1", "adapter-report.json")
	if err := os.Symlink(realPath, leafLink); err != nil {
		t.Skipf("symlinks unavailable: %v", err)
	}
	validation, err := ValidateAdapterExecutionReport(repoRoot, caseRoot, pack, Options{GateEventID: authorized.EventID, ExecutionReportPath: leafLink})
	if err != nil {
		t.Fatal(err)
	}
	if validation.Valid || validation.FailureCode != "report-not-regular" || validation.FailureStage != "read" {
		t.Fatalf("leaf symlink should fail closed: %+v", validation)
	}

	realDir := filepath.Join(caseRoot, "workspace", "real-session")
	writeGateText(t, filepath.Join(realDir, "adapter-report.json"), report)
	linkedDir := filepath.Join(caseRoot, "workspace", "main", "debug", "session-1", "linked-dir")
	if err := os.Symlink(realDir, linkedDir); err != nil {
		t.Skipf("directory symlinks unavailable: %v", err)
	}
	validation, err = ValidateAdapterExecutionReport(repoRoot, caseRoot, pack, Options{GateEventID: authorized.EventID, ExecutionReportPath: filepath.Join(linkedDir, "adapter-report.json")})
	if err != nil {
		t.Fatal(err)
	}
	if validation.Valid || validation.FailureCode != "report-not-regular" || validation.FailureStage != "read" {
		t.Fatalf("intermediate symlink should fail closed: %+v", validation)
	}
}

func TestValidateAdapterExecutionReportAcceptsCwdRelativeAuthorizedPath(t *testing.T) {
	repoRoot, caseRoot, pack := gateFixture(t)
	writePreauthorizedProfile(t, caseRoot)
	authorized, err := Apply(repoRoot, caseRoot, pack, Options{Action: "debug", Lane: "main", Actor: "gate-test", Subject: "authorized debug", TargetRef: "target-alpha", RuntimeSeconds: 30, DiskMB: 64, Requests: 1, OutputPaths: "workspace/main/debug/session-1", StopConditions: "timeout"})
	if err != nil {
		t.Fatal(err)
	}
	workspace := filepath.Join(caseRoot, "workspace", "main", "debug", "session-1")
	reportPath := filepath.Join(workspace, "adapter-report.json")
	writeGateText(t, reportPath, `{
  "schemaVersion": 1,
  "kind": "adapter-execution-report",
  "adapterId": "unit-adapter",
  "action": "debug",
  "status": "succeeded",
  "gateEventId": "`+authorized.EventID+`",
  "actualBudget": {"runtimeSeconds": 24, "diskMB": 33, "requests": 1},
  "outputRefs": ["workspace/main/debug/session-1/result.json"],
  "summary": "Adapter completed bounded debug run"
}`)

	validation, err := ValidateAdapterExecutionReport(repoRoot, caseRoot, pack, Options{GateEventID: authorized.EventID, ExecutionReportCwd: workspace, ExecutionReportPath: "adapter-report.json"})
	if err != nil {
		t.Fatal(err)
	}
	if validation.Kind != "adapter-execution-report-validation" || validation.IsMutation || validation.Applied || !validation.Valid || validation.ReportPath != "workspace/main/debug/session-1/adapter-report.json" || validation.Report == nil || validation.Report.AdapterID != "unit-adapter" {
		t.Fatalf("unexpected cwd-relative adapter report validation result: %+v", validation)
	}
	assertGateNotExists(t, filepath.Join(caseRoot, ".rekit", "facts", "observations.jsonl"))
}

func TestValidateAdapterExecutionReportMissingPathExposesMissionCommanderRepair(t *testing.T) {
	repoRoot, caseRoot, pack := gateFixture(t)
	writePreauthorizedProfile(t, caseRoot)
	authorized, err := Apply(repoRoot, caseRoot, pack, Options{Action: "debug", Lane: "main", Actor: "gate-test", Subject: "authorized debug", TargetRef: "target-alpha", RuntimeSeconds: 30, DiskMB: 64, Requests: 1, OutputPaths: "workspace/main/debug/session-1", StopConditions: "timeout"})
	if err != nil {
		t.Fatal(err)
	}

	validation, err := ValidateAdapterExecutionReport(repoRoot, caseRoot, pack, Options{GateEventID: authorized.EventID})
	if err != nil {
		t.Fatal(err)
	}
	if validation.Kind != "adapter-execution-report-validation" || validation.IsMutation || validation.Applied || validation.Valid || validation.Error != "gate execution report validation requires -ExecutionReportPath" || validation.ReportPath != "" || validation.Report != nil || validation.Contract.GateEventID != authorized.EventID {
		t.Fatalf("unexpected missing-path adapter report validation envelope: %+v", validation)
	}
	if len(validation.RepairHints) != 1 || validation.RepairHints[0].RepairAction != "provide-execution-report-path" || !validation.RepairHints[0].RecordBlocked || !validation.RepairHints[0].RerunValidation {
		t.Fatalf("missing-path validation omitted repair hint: %+v", validation.RepairHints)
	}
	commander := validation.MissionCommanderAction
	wantValidate := "/rekit gate -Pack " + pack + " -GateEventId " + authorized.EventID + " -ValidateExecutionReport -ExecutionReportPath workspace/main/debug/session-1/adapter-report.json -Format json"
	if commander.State != "needs-execution-report-path" || commander.PrimaryCommand != wantValidate || !strings.Contains(commander.Prompt, "valid=true") || gateContainsSubstring(commander.FollowUpCommands, "-Apply") || !gateContainsSubstring(commander.Boundary, "do not record evidence until validation returns valid=true") {
		t.Fatalf("missing-path validation omitted Mission Commander repair handoff: %+v", commander)
	}
	if len(validation.MissionCommanderNextActions) != 2 || validation.MissionCommanderNextActions[0].Source != "adapterReportValidation.repairHints" || validation.MissionCommanderNextActions[0].Command != "provide-execution-report-path" || validation.MissionCommanderNextActions[1].Command != wantValidate || !gateNextActionBoundaryContains(validation.MissionCommanderNextActions, "do not record evidence until validation returns valid=true") {
		t.Fatalf("missing-path validation omitted repair next actions: %+v", validation.MissionCommanderNextActions)
	}
	assertGateActionQueue(t, validation.MissionCommanderActionQueue, 2, 2, 0, 2, 0, "provide-execution-report-path")
	follow := validation.AuthorizedExecutionFollowThrough
	if follow.State != "needs-execution-report-path" || follow.ReportPath != "workspace/main/debug/session-1/adapter-report.json" || follow.ActionQueue.Summary != validation.MissionCommanderActionQueue.Summary || !authorizedFollowThroughContainsForTest(follow, "invalid-report-repair", "provide-execution-report-path") || !authorizedFollowThroughContainsForTest(follow, "invalid-report-repair", "recordBlocked=true") {
		t.Fatalf("missing-path validation omitted repair follow-through: %+v", follow)
	}
	assertGateNotExists(t, filepath.Join(caseRoot, ".rekit", "facts", "observations.jsonl"))
}

func TestValidateAdapterExecutionReportReturnsInvalidEnvelopeReadOnly(t *testing.T) {
	repoRoot, caseRoot, pack := gateFixture(t)
	writePreauthorizedProfile(t, caseRoot)
	authorized, err := Apply(repoRoot, caseRoot, pack, Options{Action: "debug", Lane: "main", Actor: "gate-test", Subject: "authorized debug", TargetRef: "target-alpha", RuntimeSeconds: 30, DiskMB: 64, Requests: 1, OutputPaths: "workspace/main/debug/session-1", StopConditions: "timeout"})
	if err != nil {
		t.Fatal(err)
	}
	reportPath := filepath.Join(caseRoot, "workspace", "main", "debug", "session-1", "bad-report.json")
	writeGateText(t, reportPath, `{
  "schemaVersion": 1,
  "kind": "adapter-execution-report",
  "adapterId": "unit-adapter",
  "action": "debug",
  "status": "boundary-hit",
  "gateEventId": "`+authorized.EventID+`",
  "actualBudget": {"runtimeSeconds": 24, "diskMB": 33, "requests": 1},
  "outputRefs": ["workspace/main/debug/session-1/result.json"]
}`)

	validation, err := ValidateAdapterExecutionReport(repoRoot, caseRoot, pack, Options{GateEventID: authorized.EventID, ExecutionReportPath: reportPath})
	if err != nil {
		t.Fatal(err)
	}
	if validation.Kind != "adapter-execution-report-validation" || validation.IsMutation || validation.Applied || validation.Valid || validation.ReportPath != "workspace/main/debug/session-1/bad-report.json" || validation.Report == nil || validation.Report.Status != "boundary-hit" {
		t.Fatalf("unexpected invalid adapter report validation envelope: %+v", validation)
	}
	if validation.Error == "" || !strings.Contains(validation.Error, "requires boundaryHits or escalation") || len(validation.Errors) != 1 || validation.Contract.GateEventID != authorized.EventID {
		t.Fatalf("invalid validation envelope omitted error or contract: %+v", validation)
	}
	if len(validation.RepairHints) != 1 || validation.RepairHints[0].RepairAction != "add-boundary-marker" || validation.RepairHints[0].Code != "boundary-marker-missing" || validation.RepairHints[0].Stage != "boundary" || !validation.RepairHints[0].RecordBlocked || !validation.RepairHints[0].RerunValidation || strings.Join(validation.RepairHints[0].AllowedStopConditions, ",") != "timeout" || !strings.Contains(strings.Join(validation.NextSteps, ","), "repairAction: add-boundary-marker") {
		t.Fatalf("invalid validation envelope omitted repair hints: %+v", validation)
	}
	commander := validation.MissionCommanderAction
	wantValidate := "/rekit gate -Pack " + pack + " -GateEventId " + authorized.EventID + " -ValidateExecutionReport -ExecutionReportPath workspace/main/debug/session-1/bad-report.json -Format json"
	if commander.State != "repair-adapter-report" || commander.PrimaryCommand != wantValidate || !strings.Contains(commander.Prompt, "valid=true") || gateContainsSubstring(commander.FollowUpCommands, "-Apply") || !gateContainsSubstring(commander.Boundary, "do not record evidence until validation returns valid=true") {
		t.Fatalf("invalid adapter report validation omitted Mission Commander repair handoff: %+v", commander)
	}
	if len(validation.MissionCommanderNextActions) != 2 || validation.MissionCommanderNextActions[0].Source != "adapterReportValidation.repairHints" || validation.MissionCommanderNextActions[0].Command != "add-boundary-marker" || validation.MissionCommanderNextActions[1].Command != wantValidate || !gateNextActionBoundaryContains(validation.MissionCommanderNextActions, "do not record evidence until validation returns valid=true") {
		t.Fatalf("invalid adapter report validation omitted repair next actions: %+v", validation.MissionCommanderNextActions)
	}
	assertGateActionQueue(t, validation.MissionCommanderActionQueue, 2, 2, 0, 2, 0, "add-boundary-marker")
	summary := validation.ReportSummary
	if summary.State != "repair-adapter-report" || summary.GateEventID != authorized.EventID || summary.ReportPath != "workspace/main/debug/session-1/bad-report.json" || !summary.ReportPresent || summary.Valid || summary.RecordReady || !summary.RecordBlocked || summary.RequiresValidation || !summary.RequiresRepair || summary.RequiresMainEscalation || summary.ReportStatus != "boundary-hit" || summary.AdapterID != "unit-adapter" || summary.ActualRuntimeSeconds != 24 || summary.OutputRefCount != 1 || summary.BoundaryHitCount != 0 || summary.HasSummary || summary.ValidationFailureCode != "boundary-marker-missing" || summary.ValidationFailureStage != "boundary" || summary.RepairHintCount != 1 || summary.RecordBlockedHintCount != 1 || summary.OutcomeCount != 1 || summary.NextActionCount != 2 || summary.ReviewRequiredActionCount != 2 || summary.CurrentAction != "add-boundary-marker" || summary.ActionQueueSummary != validation.MissionCommanderActionQueue.Summary || !gateContainsSubstring(summary.Boundary, "valid=true") {
		t.Fatalf("invalid adapter report validation omitted compact summary: %+v", summary)
	}
	follow := validation.AuthorizedExecutionFollowThrough
	if follow.State != "repair-adapter-report" || follow.ReportPath != "workspace/main/debug/session-1/bad-report.json" || follow.ActionQueue.Summary != validation.MissionCommanderActionQueue.Summary || !authorizedFollowThroughContainsForTest(follow, "invalid-report-repair", "add-boundary-marker") || !authorizedFollowThroughContainsForTest(follow, "invalid-report-repair", "failureCode/failureStage") {
		t.Fatalf("invalid adapter report validation omitted repair follow-through: %+v", follow)
	}
	assertGateNotExists(t, filepath.Join(caseRoot, ".rekit", "facts", "observations.jsonl"))
}

func TestValidateAdapterExecutionReportInvalidEnvelopeFailureCodes(t *testing.T) {
	repoRoot, caseRoot, pack := gateFixture(t)
	writePreauthorizedProfile(t, caseRoot)
	authorized, err := Apply(repoRoot, caseRoot, pack, Options{Action: "debug", Lane: "main", Actor: "gate-test", Subject: "authorized debug", TargetRef: "target-alpha", RuntimeSeconds: 30, DiskMB: 64, Requests: 1, OutputPaths: "workspace/main/debug/session-1", StopConditions: "timeout"})
	if err != nil {
		t.Fatal(err)
	}
	cases := []struct {
		name       string
		path       string
		body       string
		wantCode   string
		wantStage  string
		wantError  string
		wantReport bool
	}{
		{
			name:      "report path outside authorized outputs",
			path:      "workspace/main/other/out-of-scope-report.json",
			wantCode:  "report-path-out-of-scope",
			wantStage: "path",
			wantError: "must stay within authorized gate outputPaths",
		},
		{
			name:      "malformed json",
			path:      "workspace/main/debug/session-1/malformed-report.json",
			body:      `{"schemaVersion": 1,`,
			wantCode:  "report-json-invalid",
			wantStage: "decode",
			wantError: "invalid adapter execution report",
		},
		{
			name: "trailing json data",
			path: "workspace/main/debug/session-1/trailing-report.json",
			body: `{
  "schemaVersion": 1,
  "kind": "adapter-execution-report",
  "adapterId": "unit-adapter",
  "action": "debug",
  "status": "succeeded",
  "gateEventId": "` + authorized.EventID + `",
  "actualBudget": {"runtimeSeconds": 24, "diskMB": 33, "requests": 1},
  "outputRefs": ["workspace/main/debug/session-1/result.json"]
}
{}`,
			wantCode:   "report-trailing-data",
			wantStage:  "decode",
			wantError:  "trailing data",
			wantReport: true,
		},
		{
			name: "gate event mismatch",
			path: "workspace/main/debug/session-1/wrong-gate-report.json",
			body: `{
  "schemaVersion": 1,
  "kind": "adapter-execution-report",
  "adapterId": "unit-adapter",
  "action": "debug",
  "status": "succeeded",
  "gateEventId": "evt-other",
  "actualBudget": {"runtimeSeconds": 24, "diskMB": 33, "requests": 1},
  "outputRefs": ["workspace/main/debug/session-1/result.json"]
}`,
			wantCode:   "gate-event-mismatch",
			wantStage:  "identity",
			wantError:  "does not match authorized gate eventId",
			wantReport: true,
		},
		{
			name: "output refs out of scope",
			path: "workspace/main/debug/session-1/out-of-scope-refs-report.json",
			body: `{
  "schemaVersion": 1,
  "kind": "adapter-execution-report",
  "adapterId": "unit-adapter",
  "action": "debug",
  "status": "succeeded",
  "gateEventId": "` + authorized.EventID + `",
  "actualBudget": {"runtimeSeconds": 24, "diskMB": 33, "requests": 1},
  "outputRefs": ["workspace/main/other/result.json"]
}`,
			wantCode:   "output-refs-out-of-scope",
			wantStage:  "refs",
			wantError:  "outputRefs must stay within authorized gate outputPaths",
			wantReport: true,
		},
		{
			name: "evidence refs out of scope",
			path: "workspace/main/debug/session-1/out-of-scope-evidence-refs-report.json",
			body: `{
  "schemaVersion": 1,
  "kind": "adapter-execution-report",
  "adapterId": "unit-adapter",
  "action": "debug",
  "status": "succeeded",
  "gateEventId": "` + authorized.EventID + `",
  "actualBudget": {"runtimeSeconds": 24, "diskMB": 33, "requests": 1},
  "outputRefs": ["workspace/main/debug/session-1/result.json"],
  "evidenceRefs": ["workspace/main/other/evidence.json"]
}`,
			wantCode:   "evidence-refs-out-of-scope",
			wantStage:  "refs",
			wantError:  "evidenceRefs must stay within authorized gate outputPaths",
			wantReport: true,
		},
		{
			name: "budget marker missing",
			path: "workspace/main/debug/session-1/budget-marker-missing-report.json",
			body: `{
  "schemaVersion": 1,
  "kind": "adapter-execution-report",
  "adapterId": "unit-adapter",
  "action": "debug",
  "status": "succeeded",
  "gateEventId": "` + authorized.EventID + `",
  "actualBudget": {"runtimeSeconds": 31, "diskMB": 33, "requests": 1},
  "outputRefs": ["workspace/main/debug/session-1/result.json"]
}`,
			wantCode:   "budget-marker-missing",
			wantStage:  "budget",
			wantError:  "actualBudget exceeds authorized request",
			wantReport: true,
		},
		{
			name: "boundary hit outside authorized stop conditions",
			path: "workspace/main/debug/session-1/boundary-not-authorized-report.json",
			body: `{
  "schemaVersion": 1,
  "kind": "adapter-execution-report",
  "adapterId": "unit-adapter",
  "action": "debug",
  "status": "boundary-hit",
  "gateEventId": "` + authorized.EventID + `",
  "actualBudget": {"runtimeSeconds": 24, "diskMB": 33, "requests": 1},
  "outputRefs": ["workspace/main/debug/session-1/result.json"],
  "boundaryHits": ["scope-drift"],
  "summary": "Adapter hit an out-of-contract boundary"
}`,
			wantCode:   "boundary-hits-not-authorized",
			wantStage:  "boundary",
			wantError:  "boundaryHits must be covered by authorized gate stopConditions",
			wantReport: true,
		},
		{
			name: "failed status summary missing",
			path: "workspace/main/debug/session-1/failed-summary-missing-report.json",
			body: `{
  "schemaVersion": 1,
  "kind": "adapter-execution-report",
  "adapterId": "unit-adapter",
  "action": "debug",
  "status": "failed",
  "gateEventId": "` + authorized.EventID + `",
  "actualBudget": {"runtimeSeconds": 24, "diskMB": 33, "requests": 1},
  "outputRefs": ["workspace/main/debug/session-1/result.json"]
}`,
			wantCode:   "status-summary-missing",
			wantStage:  "summary",
			wantError:  "requires a bounded summary",
			wantReport: true,
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if tc.body != "" {
				writeGateText(t, filepath.Join(caseRoot, filepath.FromSlash(tc.path)), tc.body)
			}
			validation, err := ValidateAdapterExecutionReport(repoRoot, caseRoot, pack, Options{GateEventID: authorized.EventID, ExecutionReportPath: tc.path})
			if err != nil {
				t.Fatal(err)
			}
			if validation.Kind != "adapter-execution-report-validation" || validation.IsMutation || validation.Applied || validation.Valid || validation.ReportPath != tc.path || validation.FailureCode != tc.wantCode || validation.FailureStage != tc.wantStage || validation.Error == "" || !strings.Contains(validation.Error, tc.wantError) || len(validation.Errors) != 1 || validation.Contract.GateEventID != authorized.EventID {
				t.Fatalf("unexpected invalid adapter report validation envelope: %+v", validation)
			}
			if len(validation.RepairHints) != 1 || validation.RepairHints[0].Code != tc.wantCode || validation.RepairHints[0].Stage != tc.wantStage || validation.RepairHints[0].RepairAction == "" || !validation.RepairHints[0].RecordBlocked || !validation.RepairHints[0].RerunValidation || !strings.Contains(strings.Join(validation.NextSteps, ","), "repairAction: "+validation.RepairHints[0].RepairAction) {
				t.Fatalf("invalid adapter report validation omitted repair hints: %+v", validation)
			}
			wantState := "repair-adapter-report"
			if validation.RepairHints[0].EscalateToMain {
				wantState = "needs-main-escalation"
			}
			commander := validation.MissionCommanderAction
			wantValidate := "/rekit gate -Pack " + pack + " -GateEventId " + authorized.EventID + " -ValidateExecutionReport -ExecutionReportPath " + tc.path + " -Format json"
			if commander.State != wantState || commander.PrimaryCommand != wantValidate || !strings.Contains(commander.Prompt, "valid=true") || gateContainsSubstring(commander.FollowUpCommands, "-Apply") || !gateContainsSubstring(commander.Boundary, "do not record evidence until validation returns valid=true") {
				t.Fatalf("invalid adapter report validation omitted Mission Commander repair handoff: %+v", commander)
			}
			if tc.wantCode == "report-path-out-of-scope" && strings.Join(validation.RepairHints[0].AllowedOutputPaths, ",") != "workspace/main/debug/session-1" {
				t.Fatalf("path repair hint omitted allowed output paths: %+v", validation.RepairHints[0])
			}
			if tc.wantCode == "boundary-hits-not-authorized" && (strings.Join(validation.RepairHints[0].AllowedStopConditions, ",") != "timeout" || !validation.RepairHints[0].EscalateToMain || commander.State != "needs-main-escalation") {
				t.Fatalf("boundary repair hint omitted stop conditions, escalation marker, or commander state: hint=%+v commander=%+v", validation.RepairHints[0], commander)
			}
			if tc.wantCode == "status-summary-missing" && validation.RepairHints[0].MaxBytes != 4096 {
				t.Fatalf("summary repair hint omitted max bytes: %+v", validation.RepairHints[0])
			}
			if tc.wantReport && validation.Report == nil {
				t.Fatalf("invalid adapter report validation omitted partial report: %+v", validation)
			}
			if !tc.wantReport && validation.Report != nil {
				t.Fatalf("invalid adapter report validation included unexpected report: %+v", validation)
			}
		})
	}
	assertGateNotExists(t, filepath.Join(caseRoot, ".rekit", "facts", "observations.jsonl"))
}

func TestRecordExecutionAcceptsAdapterReportForAuthorizedGate(t *testing.T) {
	repoRoot, caseRoot, pack := gateFixture(t)
	writePreauthorizedProfile(t, caseRoot)
	authorized, err := Apply(repoRoot, caseRoot, pack, Options{Action: "debug", Lane: "main", Actor: "gate-test", Subject: "authorized debug", TargetRef: "target-alpha", RuntimeSeconds: 30, DiskMB: 64, Requests: 1, OutputPaths: "workspace/main/debug/session-1", StopConditions: "timeout"})
	if err != nil {
		t.Fatal(err)
	}
	reportPath := filepath.Join(caseRoot, "workspace", "main", "debug", "session-1", "adapter-report.json")
	writeGateText(t, reportPath, `{
  "schemaVersion": 1,
  "kind": "adapter-execution-report",
  "adapterId": "unit-adapter",
  "action": "debug",
  "status": "succeeded",
  "gateEventId": "`+authorized.EventID+`",
  "actualBudget": {"runtimeSeconds": 24, "diskMB": 33, "requests": 1},
  "outputRefs": ["workspace/main/debug/session-1/result.json"],
  "evidenceRefs": ["workspace/main/debug/session-1/evidence.json"],
  "summary": "Adapter completed bounded debug run"
}`)

	result, err := RecordExecution(repoRoot, caseRoot, pack, Options{GateEventID: authorized.EventID, Actor: "executor-1", ExecutionReportPath: reportPath})
	if err != nil {
		t.Fatal(err)
	}
	if !result.Applied || result.ExecutionEvidence == nil || result.ExecutionEvidence.Execution.Adapter == nil {
		t.Fatalf("unexpected adapter execution evidence result: %+v", result)
	}
	if result.ExecutionEvidence.Status != "succeeded" || result.ExecutionEvidence.Summary != "Adapter completed bounded debug run" || result.ExecutionEvidence.Execution.ActualBudget.RuntimeSeconds != 24 {
		t.Fatalf("adapter report defaults were not consumed: %+v", result.ExecutionEvidence)
	}
	if result.ExecutionEvidence.Execution.ExecutionReportPath != "workspace/main/debug/session-1/adapter-report.json" || result.ExecutionEvidence.Execution.Adapter.AdapterID != "unit-adapter" {
		t.Fatalf("adapter report provenance missing: %+v", result.ExecutionEvidence.Execution)
	}
	if strings.Join(result.ExecutionEvidence.Execution.OutputRefs, ",") != "workspace/main/debug/session-1/result.json" || strings.Join(result.ExecutionEvidence.EvidenceRefs, ",") != "workspace/main/debug/session-1/evidence.json" {
		t.Fatalf("adapter report refs were not consumed: %+v", result.ExecutionEvidence)
	}
	assertGateNotExists(t, filepath.Join(caseRoot, ".rekit", "facts", "authority.jsonl"))
	assertGateNotExists(t, filepath.Join(caseRoot, ".rekit", "facts", "confirmed.jsonl"))
}

func TestRecordExecutionRejectsAdapterReportDriftAfterValidation(t *testing.T) {
	repoRoot, caseRoot, pack := gateFixture(t)
	writePreauthorizedProfile(t, caseRoot)
	authorized, err := Apply(repoRoot, caseRoot, pack, Options{Action: "debug", Lane: "main", Actor: "gate-test", Subject: "authorized debug", TargetRef: "target-alpha", RuntimeSeconds: 30, DiskMB: 64, Requests: 1, OutputPaths: "workspace/main/debug/session-1", StopConditions: "timeout"})
	if err != nil {
		t.Fatal(err)
	}
	reportPath := filepath.Join(caseRoot, "workspace", "main", "debug", "session-1", "adapter-report.json")
	writeGateText(t, reportPath, `{
  "schemaVersion": 1,
  "kind": "adapter-execution-report",
  "adapterId": "unit-adapter",
  "action": "debug",
  "status": "succeeded",
  "gateEventId": "`+authorized.EventID+`",
  "actualBudget": {"runtimeSeconds": 24, "diskMB": 33, "requests": 1},
  "outputRefs": ["workspace/main/debug/session-1/result.json"],
  "summary": "Adapter completed bounded debug run"
}`)
	validation, err := ValidateAdapterExecutionReport(repoRoot, caseRoot, pack, Options{GateEventID: authorized.EventID, ExecutionReportPath: reportPath})
	if err != nil {
		t.Fatal(err)
	}
	if !validation.Valid || validation.RecordExpectedReportSHA256 == "" {
		t.Fatalf("validation omitted expected report hash: %+v", validation)
	}
	writeGateText(t, reportPath, `{
  "schemaVersion": 1,
  "kind": "adapter-execution-report",
  "adapterId": "unit-adapter",
  "action": "debug",
  "status": "succeeded",
  "gateEventId": "`+authorized.EventID+`",
  "actualBudget": {"runtimeSeconds": 25, "diskMB": 33, "requests": 1},
  "outputRefs": ["workspace/main/debug/session-1/result.json"],
  "summary": "Adapter sidecar drifted after validation"
}`)
	_, err = RecordExecution(repoRoot, caseRoot, pack, Options{GateEventID: authorized.EventID, Actor: "executor-1", ExecutionReportPath: reportPath, ExpectedExecutionReportSHA256: validation.RecordExpectedReportSHA256})
	if err == nil || !strings.Contains(err.Error(), "adapter execution report sha256 changed after validation") {
		t.Fatalf("RecordExecution drift error = %v, want sha256 mismatch", err)
	}
	assertGateNotExists(t, filepath.Join(caseRoot, ".rekit", "facts", "observations.jsonl"))
}

func TestRecordExecutionAcceptsCwdRelativeAdapterReportForAuthorizedGate(t *testing.T) {
	repoRoot, caseRoot, pack := gateFixture(t)
	writePreauthorizedProfile(t, caseRoot)
	authorized, err := Apply(repoRoot, caseRoot, pack, Options{Action: "debug", Lane: "main", Actor: "gate-test", Subject: "authorized debug", TargetRef: "target-alpha", RuntimeSeconds: 30, DiskMB: 64, Requests: 1, OutputPaths: "workspace/main/debug/session-1", StopConditions: "timeout"})
	if err != nil {
		t.Fatal(err)
	}
	workspace := filepath.Join(caseRoot, "workspace", "main", "debug", "session-1")
	writeGateText(t, filepath.Join(workspace, "adapter-report.json"), `{
  "schemaVersion": 1,
  "kind": "adapter-execution-report",
  "adapterId": "unit-adapter",
  "action": "debug",
  "status": "succeeded",
  "gateEventId": "`+authorized.EventID+`",
  "actualBudget": {"runtimeSeconds": 24, "diskMB": 33, "requests": 1},
  "outputRefs": ["workspace/main/debug/session-1/result.json"],
  "summary": "Adapter completed bounded debug run"
}`)

	result, err := RecordExecution(repoRoot, caseRoot, pack, Options{GateEventID: authorized.EventID, Actor: "executor-1", ExecutionReportCwd: workspace, ExecutionReportPath: "adapter-report.json"})
	if err != nil {
		t.Fatal(err)
	}
	if !result.Applied || result.ExecutionEvidence == nil || result.ExecutionEvidence.Execution.ExecutionReportPath != "workspace/main/debug/session-1/adapter-report.json" || result.ExecutionEvidence.Execution.Adapter == nil || result.ExecutionEvidence.Execution.Adapter.AdapterID != "unit-adapter" {
		t.Fatalf("unexpected cwd-relative adapter execution evidence result: %+v", result)
	}
	second, err := RecordExecution(repoRoot, caseRoot, pack, Options{GateEventID: authorized.EventID, Actor: "executor-1", ExecutionReportCwd: workspace, ExecutionReportPath: "adapter-report.json"})
	if err != nil {
		t.Fatal(err)
	}
	if second.Applied || second.EventID != result.EventID || second.Reason != "duplicate eventId" {
		t.Fatalf("cwd-relative adapter execution evidence replay should be idempotent: first=%+v second=%+v", result, second)
	}
	lines := readGateLines(t, filepath.Join(caseRoot, ".rekit", "facts", "observations.jsonl"))
	if len(lines) != 1 {
		t.Fatalf("cwd-relative adapter execution evidence replay wrote %d lines, want 1: %q", len(lines), lines)
	}
	assertGateNotExists(t, filepath.Join(caseRoot, ".rekit", "facts", "authority.jsonl"))
	assertGateNotExists(t, filepath.Join(caseRoot, ".rekit", "facts", "confirmed.jsonl"))
}

func TestRecordExecutionRejectsAdapterReportMismatch(t *testing.T) {
	repoRoot, caseRoot, pack := gateFixture(t)
	writePreauthorizedProfile(t, caseRoot)
	authorized, err := Apply(repoRoot, caseRoot, pack, Options{Action: "debug", Lane: "main", Actor: "gate-test", Subject: "authorized debug", TargetRef: "target-alpha", RuntimeSeconds: 30, DiskMB: 64, Requests: 1, OutputPaths: "workspace/main/debug/session-1", StopConditions: "timeout"})
	if err != nil {
		t.Fatal(err)
	}
	reportPath := filepath.Join(caseRoot, "workspace", "main", "debug", "session-1", "bad-report.json")
	writeGateText(t, reportPath, `{
  "schemaVersion": 1,
  "kind": "adapter-execution-report",
  "adapterId": "unit-adapter",
  "action": "debug",
  "status": "succeeded",
  "gateEventId": "evt-other",
  "actualBudget": {"runtimeSeconds": 24, "diskMB": 33, "requests": 1},
  "outputRefs": ["workspace/main/debug/session-1/result.json"]
}`)

	_, err = RecordExecution(repoRoot, caseRoot, pack, Options{GateEventID: authorized.EventID, Actor: "executor-1", ExecutionReportPath: reportPath})
	if err == nil || !strings.Contains(err.Error(), "gateEventId") {
		t.Fatalf("RecordExecution error = %v, want adapter gateEventId mismatch", err)
	}
	assertGateNotExists(t, filepath.Join(caseRoot, ".rekit", "facts", "observations.jsonl"))
}

func TestRecordExecutionAcceptsAdapterReportEscalation(t *testing.T) {
	repoRoot, caseRoot, pack := gateFixture(t)
	writePreauthorizedProfile(t, caseRoot)
	authorized, err := Apply(repoRoot, caseRoot, pack, Options{Action: "debug", Lane: "main", Actor: "gate-test", Subject: "authorized debug", TargetRef: "target-alpha", RuntimeSeconds: 30, DiskMB: 64, Requests: 1, OutputPaths: "workspace/main/debug/session-1", StopConditions: "timeout"})
	if err != nil {
		t.Fatal(err)
	}
	reportPath := filepath.Join(caseRoot, "workspace", "main", "debug", "session-1", "escalated-report.json")
	writeGateText(t, reportPath, `{
  "schemaVersion": 1,
  "kind": "adapter-execution-report",
  "adapterId": "unit-adapter",
  "action": "debug",
  "status": "escalated",
  "gateEventId": "`+authorized.EventID+`",
  "actualBudget": {"runtimeSeconds": 24, "diskMB": 33, "requests": 1},
  "outputRefs": ["workspace/main/debug/session-1/result.json"],
  "escalation": "new risk needs Mission Commander review",
  "summary": "Adapter escalated before continuing"
}`)

	result, err := RecordExecution(repoRoot, caseRoot, pack, Options{GateEventID: authorized.EventID, Actor: "executor-1", ExecutionReportPath: reportPath})
	if err != nil {
		t.Fatal(err)
	}
	if !result.Applied || result.ExecutionEvidence == nil || result.ExecutionEvidence.Status != "escalated" || result.ExecutionEvidence.Execution.Escalation != "new risk needs Mission Commander review" {
		t.Fatalf("adapter escalation was not consumed: %+v", result.ExecutionEvidence)
	}
	if result.ExecutionEvidence.Execution.Adapter == nil || result.ExecutionEvidence.Execution.Adapter.Escalation != "new risk needs Mission Commander review" {
		t.Fatalf("adapter escalation provenance missing: %+v", result.ExecutionEvidence.Execution.Adapter)
	}
	if strings.Join(result.NextSteps, "\n") == "" || !strings.Contains(strings.Join(result.NextSteps, "\n"), "boundary hit or escalation") || !gateContainsSubstring(result.NextSteps, "/rekit handoff main") {
		t.Fatalf("escalation next steps missing: %+v", result.NextSteps)
	}
	commander := result.MissionCommanderAction
	if commander.State != "needs-main-escalation" || commander.PrimaryCommand != "/rekit handoff main" || !strings.Contains(commander.Prompt, "boundary/escalation") || gateContainsSubstring(commander.FollowUpCommands, "/rekit continue") || !gateContainsSubstring(commander.Boundary, "stop autonomous work") || !gateContainsSubstring(commander.Boundary, "no authority/confirmed") {
		t.Fatalf("escalation evidence omitted Mission Commander escalation handoff: %+v", commander)
	}
	assertGateNotExists(t, filepath.Join(caseRoot, ".rekit", "facts", "authority.jsonl"))
	assertGateNotExists(t, filepath.Join(caseRoot, ".rekit", "facts", "confirmed.jsonl"))
}

func TestRecordExecutionRejectsAdapterReportBoundaryWithoutMarker(t *testing.T) {
	repoRoot, caseRoot, pack := gateFixture(t)
	writePreauthorizedProfile(t, caseRoot)
	authorized, err := Apply(repoRoot, caseRoot, pack, Options{Action: "debug", Lane: "main", Actor: "gate-test", Subject: "authorized debug", TargetRef: "target-alpha", RuntimeSeconds: 30, DiskMB: 64, Requests: 1, OutputPaths: "workspace/main/debug/session-1", StopConditions: "timeout"})
	if err != nil {
		t.Fatal(err)
	}
	reportPath := filepath.Join(caseRoot, "workspace", "main", "debug", "session-1", "boundary-no-marker-report.json")
	writeGateText(t, reportPath, `{
  "schemaVersion": 1,
  "kind": "adapter-execution-report",
  "adapterId": "unit-adapter",
  "action": "debug",
  "status": "boundary-hit",
  "gateEventId": "`+authorized.EventID+`",
  "actualBudget": {"runtimeSeconds": 24, "diskMB": 33, "requests": 1},
  "outputRefs": ["workspace/main/debug/session-1/result.json"]
}`)

	_, err = RecordExecution(repoRoot, caseRoot, pack, Options{GateEventID: authorized.EventID, Actor: "executor-1", ExecutionReportPath: reportPath})
	if err == nil || !strings.Contains(err.Error(), "requires boundaryHits or escalation") {
		t.Fatalf("RecordExecution error = %v, want boundary marker rejection", err)
	}
	assertGateNotExists(t, filepath.Join(caseRoot, ".rekit", "facts", "observations.jsonl"))
}

func TestRecordExecutionRejectsAdapterReportBudgetExceedWithoutMarker(t *testing.T) {
	repoRoot, caseRoot, pack := gateFixture(t)
	writePreauthorizedProfile(t, caseRoot)
	authorized, err := Apply(repoRoot, caseRoot, pack, Options{Action: "debug", Lane: "main", Actor: "gate-test", Subject: "authorized debug", TargetRef: "target-alpha", RuntimeSeconds: 30, DiskMB: 64, Requests: 1, OutputPaths: "workspace/main/debug/session-1", StopConditions: "timeout"})
	if err != nil {
		t.Fatal(err)
	}
	reportPath := filepath.Join(caseRoot, "workspace", "main", "debug", "session-1", "budget-no-marker-report.json")
	writeGateText(t, reportPath, `{
  "schemaVersion": 1,
  "kind": "adapter-execution-report",
  "adapterId": "unit-adapter",
  "action": "debug",
  "status": "succeeded",
  "gateEventId": "`+authorized.EventID+`",
  "actualBudget": {"runtimeSeconds": 31, "diskMB": 33, "requests": 1},
  "outputRefs": ["workspace/main/debug/session-1/result.json"]
}`)

	_, err = RecordExecution(repoRoot, caseRoot, pack, Options{GateEventID: authorized.EventID, Actor: "executor-1", ExecutionReportPath: reportPath})
	if err == nil || !strings.Contains(err.Error(), "actualBudget exceeds authorized request") {
		t.Fatalf("RecordExecution error = %v, want budget marker rejection", err)
	}
	assertGateNotExists(t, filepath.Join(caseRoot, ".rekit", "facts", "observations.jsonl"))
}

func TestRecordExecutionRejectsAdapterReportExplicitEscalationMismatch(t *testing.T) {
	repoRoot, caseRoot, pack := gateFixture(t)
	writePreauthorizedProfile(t, caseRoot)
	authorized, err := Apply(repoRoot, caseRoot, pack, Options{Action: "debug", Lane: "main", Actor: "gate-test", Subject: "authorized debug", TargetRef: "target-alpha", RuntimeSeconds: 30, DiskMB: 64, Requests: 1, OutputPaths: "workspace/main/debug/session-1", StopConditions: "timeout"})
	if err != nil {
		t.Fatal(err)
	}
	reportPath := filepath.Join(caseRoot, "workspace", "main", "debug", "session-1", "escalation-report.json")
	writeGateText(t, reportPath, `{
  "schemaVersion": 1,
  "kind": "adapter-execution-report",
  "adapterId": "unit-adapter",
  "action": "debug",
  "status": "escalated",
  "gateEventId": "`+authorized.EventID+`",
  "actualBudget": {"runtimeSeconds": 24, "diskMB": 33, "requests": 1},
  "outputRefs": ["workspace/main/debug/session-1/result.json"],
  "escalation": "adapter escalation",
  "summary": "Adapter escalated before completion"
}`)

	_, err = RecordExecution(repoRoot, caseRoot, pack, Options{GateEventID: authorized.EventID, Actor: "executor-1", Escalation: "explicit escalation", ExecutionReportPath: reportPath})
	if err == nil || !strings.Contains(err.Error(), "escalation") {
		t.Fatalf("RecordExecution error = %v, want adapter escalation mismatch", err)
	}
	assertGateNotExists(t, filepath.Join(caseRoot, ".rekit", "facts", "observations.jsonl"))
}

func TestRecordExecutionRejectsAdapterReportExplicitStatusMismatch(t *testing.T) {
	repoRoot, caseRoot, pack := gateFixture(t)
	writePreauthorizedProfile(t, caseRoot)
	authorized, err := Apply(repoRoot, caseRoot, pack, Options{Action: "debug", Lane: "main", Actor: "gate-test", Subject: "authorized debug", TargetRef: "target-alpha", RuntimeSeconds: 30, DiskMB: 64, Requests: 1, OutputPaths: "workspace/main/debug/session-1", StopConditions: "timeout"})
	if err != nil {
		t.Fatal(err)
	}
	reportPath := filepath.Join(caseRoot, "workspace", "main", "debug", "session-1", "status-report.json")
	writeGateText(t, reportPath, `{
  "schemaVersion": 1,
  "kind": "adapter-execution-report",
  "adapterId": "unit-adapter",
  "action": "debug",
  "status": "failed",
  "gateEventId": "`+authorized.EventID+`",
  "actualBudget": {"runtimeSeconds": 24, "diskMB": 33, "requests": 1},
  "outputRefs": ["workspace/main/debug/session-1/result.json"],
  "summary": "Adapter failed before completion"
}`)

	_, err = RecordExecution(repoRoot, caseRoot, pack, Options{GateEventID: authorized.EventID, Actor: "executor-1", ExecutionStatus: "succeeded", ExecutionReportPath: reportPath})
	if err == nil || !strings.Contains(err.Error(), "status") {
		t.Fatalf("RecordExecution error = %v, want adapter status mismatch", err)
	}
	assertGateNotExists(t, filepath.Join(caseRoot, ".rekit", "facts", "observations.jsonl"))
}

func TestRecordExecutionRejectsAdapterReportBoundaryHitsOutsideAuthorizedStopConditions(t *testing.T) {
	repoRoot, caseRoot, pack := gateFixture(t)
	writePreauthorizedProfile(t, caseRoot)
	authorized, err := Apply(repoRoot, caseRoot, pack, Options{Action: "debug", Lane: "main", Actor: "gate-test", Subject: "authorized debug", TargetRef: "target-alpha", RuntimeSeconds: 30, DiskMB: 64, Requests: 1, OutputPaths: "workspace/main/debug/session-1", StopConditions: "timeout"})
	if err != nil {
		t.Fatal(err)
	}
	reportPath := filepath.Join(caseRoot, "workspace", "main", "debug", "session-1", "boundary-not-authorized-report.json")
	writeGateText(t, reportPath, `{
  "schemaVersion": 1,
  "kind": "adapter-execution-report",
  "adapterId": "unit-adapter",
  "action": "debug",
  "status": "boundary-hit",
  "gateEventId": "`+authorized.EventID+`",
  "actualBudget": {"runtimeSeconds": 24, "diskMB": 33, "requests": 1},
  "outputRefs": ["workspace/main/debug/session-1/result.json"],
  "boundaryHits": ["scope-drift"],
  "summary": "Adapter hit a stop condition outside this gate"
}`)

	_, err = RecordExecution(repoRoot, caseRoot, pack, Options{GateEventID: authorized.EventID, Actor: "executor-1", ExecutionReportPath: reportPath})
	if err == nil || !strings.Contains(err.Error(), "boundaryHits must be covered by authorized gate stopConditions") {
		t.Fatalf("RecordExecution error = %v, want boundaryHits stopCondition coverage rejection", err)
	}
	assertGateNotExists(t, filepath.Join(caseRoot, ".rekit", "facts", "observations.jsonl"))
}

func TestRecordExecutionRejectsExplicitBoundaryHitsOutsideAuthorizedStopConditions(t *testing.T) {
	repoRoot, caseRoot, pack := gateFixture(t)
	writePreauthorizedProfile(t, caseRoot)
	authorized, err := Apply(repoRoot, caseRoot, pack, Options{Action: "debug", Lane: "main", Actor: "gate-test", Subject: "authorized debug", TargetRef: "target-alpha", RuntimeSeconds: 30, DiskMB: 64, Requests: 1, OutputPaths: "workspace/main/debug/session-1", StopConditions: "timeout"})
	if err != nil {
		t.Fatal(err)
	}
	_, err = RecordExecution(repoRoot, caseRoot, pack, Options{GateEventID: authorized.EventID, Actor: "executor-1", ExecutionStatus: "boundary-hit", ActualRuntimeSeconds: 24, ActualDiskMB: 32, ActualRequests: 1, OutputRefs: "workspace/main/debug/session-1/result.json", BoundaryHits: "scope-drift"})
	if err == nil || !strings.Contains(err.Error(), "boundaryHits must be covered by authorized gate stopConditions") {
		t.Fatalf("RecordExecution error = %v, want explicit boundaryHits stopCondition coverage rejection", err)
	}
	assertGateNotExists(t, filepath.Join(caseRoot, ".rekit", "facts", "observations.jsonl"))
}

func TestRecordExecutionRejectsAdapterReportFailureWithoutSummary(t *testing.T) {
	repoRoot, caseRoot, pack := gateFixture(t)
	writePreauthorizedProfile(t, caseRoot)
	authorized, err := Apply(repoRoot, caseRoot, pack, Options{Action: "debug", Lane: "main", Actor: "gate-test", Subject: "authorized debug", TargetRef: "target-alpha", RuntimeSeconds: 30, DiskMB: 64, Requests: 1, OutputPaths: "workspace/main/debug/session-1", StopConditions: "timeout"})
	if err != nil {
		t.Fatal(err)
	}
	reportPath := filepath.Join(caseRoot, "workspace", "main", "debug", "session-1", "failed-without-summary-report.json")
	writeGateText(t, reportPath, `{
  "schemaVersion": 1,
  "kind": "adapter-execution-report",
  "adapterId": "unit-adapter",
  "action": "debug",
  "status": "failed",
  "gateEventId": "`+authorized.EventID+`",
  "actualBudget": {"runtimeSeconds": 24, "diskMB": 33, "requests": 1},
  "outputRefs": ["workspace/main/debug/session-1/result.json"]
}`)

	_, err = RecordExecution(repoRoot, caseRoot, pack, Options{GateEventID: authorized.EventID, Actor: "executor-1", ExecutionReportPath: reportPath})
	if err == nil || !strings.Contains(err.Error(), "requires a bounded summary") {
		t.Fatalf("RecordExecution error = %v, want failed status summary rejection", err)
	}
	assertGateNotExists(t, filepath.Join(caseRoot, ".rekit", "facts", "observations.jsonl"))
}

func TestRecordExecutionRejectsOutOfScopeAdapterReportRefs(t *testing.T) {
	repoRoot, caseRoot, pack := gateFixture(t)
	writePreauthorizedProfile(t, caseRoot)
	authorized, err := Apply(repoRoot, caseRoot, pack, Options{Action: "debug", Lane: "main", Actor: "gate-test", Subject: "authorized debug", TargetRef: "target-alpha", RuntimeSeconds: 30, DiskMB: 64, Requests: 1, OutputPaths: "workspace/main/debug/session-1", StopConditions: "timeout"})
	if err != nil {
		t.Fatal(err)
	}
	reportPath := filepath.Join(caseRoot, "workspace", "main", "debug", "session-1", "bad-refs-report.json")
	writeGateText(t, reportPath, `{
  "schemaVersion": 1,
  "kind": "adapter-execution-report",
  "adapterId": "unit-adapter",
  "action": "debug",
  "status": "succeeded",
  "gateEventId": "`+authorized.EventID+`",
  "actualBudget": {"runtimeSeconds": 24, "diskMB": 33, "requests": 1},
  "outputRefs": ["workspace/main/other/result.json"]
}`)

	_, err = RecordExecution(repoRoot, caseRoot, pack, Options{GateEventID: authorized.EventID, Actor: "executor-1", ExecutionReportPath: reportPath})
	if err == nil || !strings.Contains(err.Error(), "outputRefs must stay within authorized gate outputPaths") {
		t.Fatalf("RecordExecution error = %v, want adapter outputRef boundary rejection", err)
	}
	assertGateNotExists(t, filepath.Join(caseRoot, ".rekit", "facts", "observations.jsonl"))
}

func TestRecordExecutionRejectsOutOfScopeAdapterReportEvidenceRefs(t *testing.T) {
	repoRoot, caseRoot, pack := gateFixture(t)
	writePreauthorizedProfile(t, caseRoot)
	authorized, err := Apply(repoRoot, caseRoot, pack, Options{Action: "debug", Lane: "main", Actor: "gate-test", Subject: "authorized debug", TargetRef: "target-alpha", RuntimeSeconds: 30, DiskMB: 64, Requests: 1, OutputPaths: "workspace/main/debug/session-1", StopConditions: "timeout"})
	if err != nil {
		t.Fatal(err)
	}
	reportPath := filepath.Join(caseRoot, "workspace", "main", "debug", "session-1", "bad-evidence-refs-report.json")
	writeGateText(t, reportPath, `{
  "schemaVersion": 1,
  "kind": "adapter-execution-report",
  "adapterId": "unit-adapter",
  "action": "debug",
  "status": "succeeded",
  "gateEventId": "`+authorized.EventID+`",
  "actualBudget": {"runtimeSeconds": 24, "diskMB": 33, "requests": 1},
  "outputRefs": ["workspace/main/debug/session-1/result.json"],
  "evidenceRefs": ["workspace/main/other/evidence.json"]
}`)

	_, err = RecordExecution(repoRoot, caseRoot, pack, Options{GateEventID: authorized.EventID, Actor: "executor-1", ExecutionReportPath: reportPath})
	if err == nil || !strings.Contains(err.Error(), "evidenceRefs must stay within authorized gate outputPaths") {
		t.Fatalf("RecordExecution error = %v, want adapter evidenceRef boundary rejection", err)
	}
	assertGateNotExists(t, filepath.Join(caseRoot, ".rekit", "facts", "observations.jsonl"))
}

func TestRecordExecutionRejectsOutOfScopeOutputRefs(t *testing.T) {
	repoRoot, caseRoot, pack := gateFixture(t)
	writePreauthorizedProfile(t, caseRoot)
	authorized, err := Apply(repoRoot, caseRoot, pack, Options{Action: "debug", Lane: "main", Actor: "gate-test", Subject: "authorized debug", TargetRef: "target-alpha", RuntimeSeconds: 30, DiskMB: 64, Requests: 1, OutputPaths: "workspace/main/debug/session-1", StopConditions: "timeout"})
	if err != nil {
		t.Fatal(err)
	}
	_, err = RecordExecution(repoRoot, caseRoot, pack, Options{GateEventID: authorized.EventID, Actor: "executor-1", ExecutionStatus: "succeeded", OutputRefs: "workspace/main/other/result.json"})
	if err == nil || !strings.Contains(err.Error(), "outputRefs must stay within authorized gate outputPaths") {
		t.Fatalf("RecordExecution error = %v, want outputRef boundary rejection", err)
	}
	assertGateNotExists(t, filepath.Join(caseRoot, ".rekit", "facts", "observations.jsonl"))
}

func TestRecordExecutionRejectsOutOfScopeEvidenceRefs(t *testing.T) {
	repoRoot, caseRoot, pack := gateFixture(t)
	writePreauthorizedProfile(t, caseRoot)
	authorized, err := Apply(repoRoot, caseRoot, pack, Options{Action: "debug", Lane: "main", Actor: "gate-test", Subject: "authorized debug", TargetRef: "target-alpha", RuntimeSeconds: 30, DiskMB: 64, Requests: 1, OutputPaths: "workspace/main/debug/session-1", StopConditions: "timeout"})
	if err != nil {
		t.Fatal(err)
	}
	_, err = RecordExecution(repoRoot, caseRoot, pack, Options{GateEventID: authorized.EventID, Actor: "executor-1", ExecutionStatus: "succeeded", OutputRefs: "workspace/main/debug/session-1/result.json", EvidenceRefs: "workspace/main/other/evidence.json"})
	if err == nil || !strings.Contains(err.Error(), "evidenceRefs must stay within authorized gate outputPaths") {
		t.Fatalf("RecordExecution error = %v, want evidenceRef boundary rejection", err)
	}
	assertGateNotExists(t, filepath.Join(caseRoot, ".rekit", "facts", "observations.jsonl"))
}

func TestRecordExecutionRequiresBoundaryMarkerWhenBudgetExceeded(t *testing.T) {
	repoRoot, caseRoot, pack := gateFixture(t)
	writePreauthorizedProfile(t, caseRoot)
	authorized, err := Apply(repoRoot, caseRoot, pack, Options{Action: "debug", Lane: "main", Actor: "gate-test", Subject: "authorized debug", TargetRef: "target-alpha", RuntimeSeconds: 30, DiskMB: 64, Requests: 1, OutputPaths: "workspace/main/debug/session-1", StopConditions: "timeout,budget-exhausted"})
	if err != nil {
		t.Fatal(err)
	}
	_, err = RecordExecution(repoRoot, caseRoot, pack, Options{GateEventID: authorized.EventID, Actor: "executor-1", ExecutionStatus: "succeeded", ActualRuntimeSeconds: 31, ActualDiskMB: 32, ActualRequests: 1, OutputRefs: "workspace/main/debug/session-1/result.json"})
	if err == nil || !strings.Contains(err.Error(), "actual budget exceeds authorized request") {
		t.Fatalf("RecordExecution error = %v, want budget boundary rejection", err)
	}
	result, err := RecordExecution(repoRoot, caseRoot, pack, Options{GateEventID: authorized.EventID, Actor: "executor-1", ExecutionStatus: "boundary-hit", ActualRuntimeSeconds: 31, ActualDiskMB: 32, ActualRequests: 1, OutputRefs: "workspace/main/debug/session-1/result.json", BoundaryHits: "budget-exhausted"})
	if err != nil {
		t.Fatal(err)
	}
	if !result.Applied || result.ExecutionEvidence == nil || strings.Join(result.ExecutionEvidence.Execution.BoundaryHits, ",") != "budget-exhausted" {
		t.Fatalf("unexpected boundary-hit execution evidence: %+v", result)
	}
}

func writePreauthorizedProfile(t *testing.T, caseRoot string) {
	t.Helper()
	writeGateText(t, filepath.Join(caseRoot, ".rekit", "lanes", "main", "autonomy.json"), `{
  "schemaVersion": 1,
  "profileId": "prof-main-debug",
  "lane": "main",
  "mode": "preauthorized",
  "allowedActions": ["debug"],
  "deniedActions": ["symex"],
  "targetScope": [{"match":"exact","value":"target-alpha"}],
  "budget": {"runtimeSeconds": 60, "diskMB": 128, "requests": 2},
  "stopConditions": ["timeout", "scope-drift", "budget-exhausted"],
  "outputPaths": ["workspace/main/debug"],
  "recordRequired": true,
  "notifyMainOn": ["boundary-hit", "new-risk"],
  "grantedBy": "user",
  "grantedAt": "2026-01-01T00:00:00Z",
  "expiresAt": "2999-01-01T00:00:00Z"
}`)
}

func readSingleExecutionEvidence(t *testing.T, path string) ExecutionEvidencePreview {
	t.Helper()
	lines := readGateLines(t, path)
	if len(lines) != 1 {
		t.Fatalf("observation ledger has %d lines, want 1: %q", len(lines), lines)
	}
	var event ExecutionEvidencePreview
	if err := json.Unmarshal([]byte(lines[0]), &event); err != nil {
		t.Fatalf("execution evidence event did not decode: %v\n%s", err, lines[0])
	}
	return event
}

func readSingleGateEvent(t *testing.T, path string) EventPreview {
	t.Helper()
	lines := readGateLines(t, path)
	if len(lines) != 1 {
		t.Fatalf("request ledger has %d lines, want 1: %q", len(lines), lines)
	}
	var event EventPreview
	if err := json.Unmarshal([]byte(lines[0]), &event); err != nil {
		t.Fatalf("request event did not decode: %v\n%s", err, lines[0])
	}
	return event
}

func readGateLines(t *testing.T, path string) []string {
	t.Helper()
	b, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	text := strings.TrimSpace(strings.ReplaceAll(string(b), "\r\n", "\n"))
	if text == "" {
		return nil
	}
	return strings.Split(text, "\n")
}

func writeGateText(t *testing.T, path, text string) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte(text), 0o644); err != nil {
		t.Fatal(err)
	}
}

func assertGateNotExists(t *testing.T, path string) {
	t.Helper()
	if _, err := os.Stat(path); err == nil || !os.IsNotExist(err) {
		t.Fatalf("path exists or stat failed unexpectedly for %s: %v", path, err)
	}
}

func gateNextActionContainsCommand(items []mission.MissionCommanderNextActionItem, want string) bool {
	for _, item := range items {
		if strings.Contains(item.Command, want) {
			return true
		}
	}
	return false
}

func gateNextActionContainsSource(items []mission.MissionCommanderNextActionItem, want string) bool {
	for _, item := range items {
		if strings.Contains(item.Source, want) {
			return true
		}
	}
	return false
}

func gateNextActionBoundaryContains(items []mission.MissionCommanderNextActionItem, want string) bool {
	for _, item := range items {
		if gateContainsSubstring(item.Boundary, want) {
			return true
		}
	}
	return false
}

func assertGateActionQueue(t *testing.T, queue mission.MissionCommanderActionQueue, total, unblocked, blocked, requiresReview, followUp int, currentCommand string) {
	t.Helper()
	if queue.Counts.Total != total || queue.Counts.Unblocked != unblocked || queue.Counts.Blocked != blocked || queue.Counts.RequiresReview != requiresReview || queue.Counts.FollowUp != followUp || queue.CurrentAction == nil || queue.CurrentAction.Command != currentCommand || !strings.Contains(queue.Summary, "current="+currentCommand) {
		t.Fatalf("Mission Commander action queue drifted: %+v", queue)
	}
}

func authorizedFollowThroughContainsForTest(follow AuthorizedExecutionFollowThrough, outcomeName, want string) bool {
	fields := append([]string{follow.State, follow.GateEventID, follow.ReportPath, follow.ActionQueue.Summary}, follow.Boundary...)
	for _, outcome := range follow.Outcomes {
		if outcome.Name != outcomeName {
			continue
		}
		fields = append(fields, outcome.Name, outcome.State, outcome.When, outcome.Command, outcome.Expected)
		fields = append(fields, outcome.Actions...)
		fields = append(fields, outcome.RepairActions...)
		fields = append(fields, outcome.VerificationCommands...)
		fields = append(fields, outcome.Evidence...)
		fields = append(fields, outcome.Boundary...)
	}
	return gateContainsSubstring(fields, want)
}

func executionEvidenceFollowThroughContainsForTest(follow mission.ExecutionEvidenceFollowThrough, outcomeName, want string) bool {
	fields := append([]string{follow.State, follow.GateEventID, follow.ActionQueue.Summary}, follow.Boundary...)
	for _, outcome := range follow.Outcomes {
		if outcome.Name != outcomeName {
			continue
		}
		fields = append(fields, outcome.Name, outcome.State, outcome.When, outcome.Command, outcome.Expected)
		fields = append(fields, outcome.Actions...)
		fields = append(fields, outcome.VerificationCommands...)
		fields = append(fields, outcome.Evidence...)
		fields = append(fields, outcome.Boundary...)
	}
	return gateContainsSubstring(fields, want)
}

func gateContainsSubstring(items []string, want string) bool {
	for _, item := range items {
		if strings.Contains(item, want) {
			return true
		}
	}
	return false
}
