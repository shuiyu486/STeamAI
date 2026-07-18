package gate

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
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
	if strings.Join(contract.AllowedStatuses, ",") != "succeeded,failed,boundary-hit,escalated,aborted" || strings.Join(contract.AllowedOutputPaths, ",") != "workspace/main/debug/session-1" || contract.AuthorizedBudget.RuntimeSeconds != 30 {
		t.Fatalf("adapter report contract omitted authorized boundaries: %+v", contract)
	}
	if strings.Join(contract.StopConditions, ",") != "timeout" || contract.SummaryMaxBytes != 4096 || contract.EscalationMaxBytes != 4096 || !contract.RecordRequired || !strings.Contains(strings.Join(contract.DeniedActions, ","), "heavy-tool execution") {
		t.Fatalf("adapter report contract omitted live validation rules: %+v", contract)
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
	if validation.Kind != "adapter-execution-report-validation" || validation.IsMutation || validation.Applied || !validation.Valid || validation.ReportPath != "workspace/main/debug/session-1/adapter-report.json" || validation.Report.AdapterID != "unit-adapter" {
		t.Fatalf("unexpected adapter report validation result: %+v", validation)
	}
	if validation.Contract.Kind != "adapter-execution-report-contract" || validation.Contract.GateEventID != authorized.EventID || validation.Contract.AuthorizedBudget.RuntimeSeconds != 30 {
		t.Fatalf("validation omitted adapter contract boundaries: %+v", validation.Contract)
	}
	assertGateNotExists(t, filepath.Join(caseRoot, ".rekit", "facts", "observations.jsonl"))
	assertGateNotExists(t, filepath.Join(caseRoot, ".rekit", "facts", "authority.jsonl"))
	assertGateNotExists(t, filepath.Join(caseRoot, ".rekit", "facts", "confirmed.jsonl"))
}

func TestValidateAdapterExecutionReportRejectsInvalidReportReadOnly(t *testing.T) {
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

	_, err = ValidateAdapterExecutionReport(repoRoot, caseRoot, pack, Options{GateEventID: authorized.EventID, ExecutionReportPath: reportPath})
	if err == nil || !strings.Contains(err.Error(), "requires boundaryHits or escalation") {
		t.Fatalf("ValidateAdapterExecutionReport error = %v, want boundary marker rejection", err)
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
	if strings.Join(result.NextSteps, "\n") == "" || !strings.Contains(strings.Join(result.NextSteps, "\n"), "boundary hit or escalation") {
		t.Fatalf("escalation next steps missing: %+v", result.NextSteps)
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
  "escalation": "adapter escalation"
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
  "outputRefs": ["workspace/main/debug/session-1/result.json"]
}`)

	_, err = RecordExecution(repoRoot, caseRoot, pack, Options{GateEventID: authorized.EventID, Actor: "executor-1", ExecutionStatus: "succeeded", ExecutionReportPath: reportPath})
	if err == nil || !strings.Contains(err.Error(), "status") {
		t.Fatalf("RecordExecution error = %v, want adapter status mismatch", err)
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

func TestRecordExecutionRequiresBoundaryMarkerWhenBudgetExceeded(t *testing.T) {
	repoRoot, caseRoot, pack := gateFixture(t)
	writePreauthorizedProfile(t, caseRoot)
	authorized, err := Apply(repoRoot, caseRoot, pack, Options{Action: "debug", Lane: "main", Actor: "gate-test", Subject: "authorized debug", TargetRef: "target-alpha", RuntimeSeconds: 30, DiskMB: 64, Requests: 1, OutputPaths: "workspace/main/debug/session-1", StopConditions: "timeout"})
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
  "stopConditions": ["timeout", "scope-drift"],
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
