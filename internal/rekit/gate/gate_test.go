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
