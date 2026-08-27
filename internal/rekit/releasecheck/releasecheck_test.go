package releasecheck

import (
	"encoding/json"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"

	"github.com/shuiyu486/re-context-kits/internal/rekit/commands"
	"github.com/shuiyu486/re-context-kits/internal/rekit/defaults"
)

func TestReleaseCheckRequiresBatchHistory(t *testing.T) {
	repo := cleanReleaseRepoRoot(t)
	writeCompletedReleaseHandoffLatestBatchFixture(t, repo)
	path := filepath.Join(repo, "docs", "batch-history.md")
	if err := os.Remove(path); err != nil {
		t.Fatal(err)
	}
	result, err := Build(repo)
	if err != nil {
		t.Fatal(err)
	}
	if result.Ready {
		t.Fatal("release-check should fail when docs/batch-history.md is missing")
	}
	for _, doc := range result.Documents {
		if doc.Path == "docs/batch-history.md" && !doc.Present {
			return
		}
	}
	t.Fatalf("missing batch history was not reported: %+v", result.Documents)
}

func TestReleaseCheckIncludesCurrentAndLegacyEntrypointReadiness(t *testing.T) {
	repo := cleanReleaseRepoRoot(t)
	writeCompletedReleaseHandoffLatestBatchFixture(t, repo)
	result, err := Build(repo)
	if err != nil {
		t.Fatal(err)
	}
	if !result.PublicDefaultDocs.Ready || result.PublicDefaultDocs.Model != "steamai-self-contained-current" || result.PublicDefaultDocs.DefaultEntrypoint != "/steamai" || result.PublicDefaultDocs.StateRoot != ".steamai" || result.PublicDefaultDocs.RuntimeSource != "project-local-verified-bundle" || result.PublicDefaultDocs.FallbackAllowed {
		t.Fatalf("release-check omitted current STeamAI entry readiness: %+v", result.PublicDefaultDocs)
	}
	if !result.CaseShim.Ready || result.CaseShim.Model != "legacy-rekit-case-shim-compatibility" || result.CaseShim.CompatibilityEntrypoint != "/rekit" || result.CaseShim.StateRoot != ".rekit" || result.CaseShim.DefaultForNewProjects {
		t.Fatalf("release-check omitted explicit legacy compatibility readiness: %+v", result.CaseShim)
	}
}

func TestReleaseCheckCurrentEntryFailureBlocksReadyWithoutReplacingLegacyGate(t *testing.T) {
	repo := cleanReleaseRepoRoot(t)
	writeCompletedReleaseHandoffLatestBatchFixture(t, repo)
	if err := os.Remove(filepath.Join(repo, ".claude", "skills", "steamai", "SKILL.md")); err != nil {
		t.Fatal(err)
	}
	result, err := Build(repo)
	if err != nil {
		t.Fatal(err)
	}
	if result.Ready || result.PublicDefaultDocs.Ready || !result.CaseShim.Ready {
		t.Fatalf("release-check did not independently enforce current and legacy readiness: current=%+v legacy=%+v ready=%t", result.PublicDefaultDocs, result.CaseShim, result.Ready)
	}
	assertWarningContains(t, result.Warnings, ".claude/skills/steamai/SKILL.md")
}

func TestReleaseCheckLegacyCompatibilityFailureStillBlocksReady(t *testing.T) {
	repo := cleanReleaseRepoRoot(t)
	writeCompletedReleaseHandoffLatestBatchFixture(t, repo)
	if err := os.Remove(filepath.Join(repo, "rekit", "templates", "case-shim", "SKILL.md")); err != nil {
		t.Fatal(err)
	}
	result, err := Build(repo)
	if err != nil {
		t.Fatal(err)
	}
	if result.Ready || result.CaseShim.Ready || !result.PublicDefaultDocs.Ready {
		t.Fatalf("release-check stopped enforcing legacy compatibility independently: current=%+v legacy=%+v ready=%t", result.PublicDefaultDocs, result.CaseShim, result.Ready)
	}
	assertWarningContains(t, result.Warnings, "case-shim/SKILL.md")
}

func TestReleaseCheckIncludesManifestHeavyToolGateActions(t *testing.T) {
	repo := cleanReleaseRepoRoot(t)
	writeCompletedReleaseHandoffLatestBatchFixture(t, repo)
	result, err := Build(repo)
	if err != nil {
		t.Fatal(err)
	}
	counts := ReleaseCheckResultCountsFor(result)
	if !result.Ready || counts.Warnings != 0 {
		t.Fatalf("release-check unexpectedly not ready: %+v", result)
	}
	if counts.Packs == 0 || counts.HeavyToolGateActions == 0 {
		t.Fatalf("release-check omitted pack or heavy-tool gate inventory: %+v", result)
	}
	if got := strings.Join(result.HeavyToolGateActions, ","); got != "debug,dump,full-trace,inject,inspect,network,patch,symex" {
		t.Fatalf("HeavyToolGateActions = %q", got)
	}
	for _, pack := range result.Packs {
		want := 7
		if pack.ID == "_template" || pack.ID == defaults.DefaultPack || pack.ID == "web-security" {
			want = 8
		}
		if pack.HeavyToolGates != want {
			t.Fatalf("pack %s HeavyToolGates = %d, want %d", pack.ID, pack.HeavyToolGates, want)
		}
	}
}

func TestReleaseCheckIncludesProductionMaturityAdmission(t *testing.T) {
	repo := cleanReleaseRepoRoot(t)
	writeCompletedReleaseHandoffLatestBatchFixture(t, repo)
	result, err := Build(repo)
	if err != nil {
		t.Fatal(err)
	}
	if !result.Ready || !result.ProductionRegistry.Ready {
		t.Fatalf("release-check production registry unexpectedly not ready: registry=%+v warnings=%v", result.ProductionRegistry, result.Warnings)
	}
	if len(result.ProductionPacks) != 2 {
		t.Fatalf("production pack admission count=%d, want 2: %+v", len(result.ProductionPacks), result.ProductionPacks)
	}
	for _, admission := range result.ProductionPacks {
		if !admission.Ready || admission.MaturitySource != "manifest-declared" || admission.ReadyMeaning != "repository-contract-inventory" || admission.FixtureClass != "synthetic-repository-fixture" || admission.RealClaudeReceiptObserved || admission.RealTargetToolReceiptObserved || admission.ReceiptKindMeaning != "expected-instruction-consumption-receipt-kind" || admission.InstructionIdentity == nil || admission.InstructionIdentity.SHA256 == "" || len(admission.InstructionIdentity.Sources) == 0 {
			t.Fatalf("production pack admission omitted or overstated repository inventory evidence: %+v", admission)
		}
		data, err := json.Marshal(admission.InstructionIdentity)
		if err != nil {
			t.Fatal(err)
		}
		if strings.Contains(string(data), "content") {
			t.Fatalf("release admission exposed instruction source content: %s", data)
		}
	}
}

func TestReleaseCheckIncludesSharedCapabilityAdmission(t *testing.T) {
	repo := cleanReleaseRepoRoot(t)
	writeCompletedReleaseHandoffLatestBatchFixture(t, repo)
	result, err := Build(repo)
	if err != nil {
		t.Fatal(err)
	}
	admission := result.CapabilityContract
	if !admission.Ready || len(admission.Warnings) != 0 || len(admission.PolicyClasses) != 4 ||
		admission.Contract == "" || admission.Sinks == "" || admission.Evidence == "" {
		t.Fatalf("release-check capability admission is incomplete: %+v", admission)
	}
}

func TestReleaseCheckRejectsCapabilitySinkSymbolDrift(t *testing.T) {
	repo := cleanReleaseRepoRoot(t)
	writeCompletedReleaseHandoffLatestBatchFixture(t, repo)
	path := filepath.Join(repo, "internal", "rekit", "externalsession", "transport.go")
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	drifted := strings.Replace(string(data), "func validateTransportDelivery(", "func validateTransportDeliveryDrift(", 1)
	if drifted == string(data) {
		t.Fatal("failed to create capability sink symbol drift fixture")
	}
	writeFile(t, path, drifted)

	result, err := Build(repo)
	if err != nil {
		t.Fatal(err)
	}
	if result.Ready || result.CapabilityContract.Ready {
		t.Fatalf("release-check accepted capability sink drift: admission=%+v warnings=%v", result.CapabilityContract, result.Warnings)
	}
	matched := false
	for _, warning := range result.CapabilityContract.Warnings {
		matched = matched || strings.Contains(warning, "capability sink symbol is missing")
	}
	if !matched {
		t.Fatalf("release-check capability sink drift omitted exact warning: %+v", result.CapabilityContract)
	}
}

func TestReleaseCheckRejectsProductionVerifierSymbolDrift(t *testing.T) {
	repo := cleanReleaseRepoRoot(t)
	writeCompletedReleaseHandoffLatestBatchFixture(t, repo)
	path := filepath.Join(repo, "internal", "rekit", "websecurity", "openapi.go")
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	drifted := strings.Replace(string(data), "func ImportOpenAPI(", "func ImportOpenAPIDrift(", 1)
	if drifted == string(data) {
		t.Fatal("failed to create verifier symbol drift fixture")
	}
	writeFile(t, path, drifted)

	result, err := Build(repo)
	if err != nil {
		t.Fatal(err)
	}
	if result.Ready {
		t.Fatalf("release-check accepted production verifier symbol drift: %+v", result.ProductionPacks)
	}
	assertWarningContains(t, result.Warnings, "production pack web-security: production contract semantic verifier symbol is missing")
	for _, admission := range result.ProductionPacks {
		if admission.Pack == "web-security" && admission.Ready {
			t.Fatalf("web-security admission stayed ready after verifier symbol drift: %+v", admission)
		}
	}
}

func TestCIReleaseGateInventoryFromRepo(t *testing.T) {
	gate := ciReleaseGate(repoRoot(t))
	counts := CIReleaseGateCountsFor(gate)
	if !gate.Ready || gate.WorkflowPath != ".github/workflows/release-gate.yml" || gate.Summary != "CI release gate inventory ok" || counts.Warnings != 0 {
		t.Fatalf("unexpected CI release gate inventory: %+v", gate)
	}
	if gate.Kind != "workflow-definition" || !gate.DefinitionReady || gate.DefinitionReady != gate.Ready || gate.ReadyMeaning != "workflow-definition" || gate.ProvesRemoteExecution || gate.ProvesRemoteGreen {
		t.Fatalf("CI release gate readiness meaning drifted: %+v", gate)
	}
	if counts.WorkflowChecks == 0 || counts.Jobs != 3 || counts.RequiredCommands != 24 || counts.ForbiddenStrings == 0 {
		t.Fatalf("CI release gate omitted required sections: %+v", gate)
	}
	assertCIJob(t, gate, "go-checks-linux", "Go release checks (Linux)", "ubuntu-latest")
	assertCIJob(t, gate, "go-checks-windows", "Go release checks (Windows)", "windows-latest")
	assertCIJob(t, gate, "go-checks-macos", "Go release checks (macOS)", "macos-latest")
	for _, job := range []string{"go-checks-linux", "go-checks-windows", "go-checks-macos"} {
		assertCICommand(t, gate, job, "go run ./cmd/rekit -- -Command release-check -Format json")
		assertCICommand(t, gate, job, "go run ./cmd/rekit -- -Command status")
		assertCICommand(t, gate, job, "go run ./cmd/rekit -- -Command packs")
		assertCICommand(t, gate, job, "go run ./cmd/rekit -- -Command doctor")
		assertCICommand(t, gate, job, "go run ./cmd/skillcontractgen -repo . -check")
		assertCICommand(t, gate, job, CanonicalGoPackSmokeCommand)
		assertCICommand(t, gate, job, CanonicalGoTestCommand)
		assertCICommand(t, gate, job, "go vet ./...")
	}
	for _, forbidden := range gate.ForbiddenStrings {
		if forbidden.Present {
			t.Fatalf("forbidden CI release gate pattern present: %+v", forbidden)
		}
	}
}

func TestCIReleaseGateInventoryRequiresVetBeforeTests(t *testing.T) {
	repo := t.TempDir()
	workflow, err := os.ReadFile(filepath.Join(repoRoot(t), ".github", "workflows", "release-gate.yml"))
	if err != nil {
		t.Fatal(err)
	}
	text := strings.ReplaceAll(string(workflow), "\r\n", "\n")
	old := "      - name: Go vet\n        run: go vet ./...\n\n      - name: Go-native pack smoke matrix\n        run: " + CanonicalGoPackSmokeCommand + "\n\n      - name: Go tests\n        run: " + CanonicalGoTestCommand
	new := "      - name: Go tests\n        run: " + CanonicalGoTestCommand + "\n\n      - name: Go vet\n        run: go vet ./...\n\n      - name: Go-native pack smoke matrix\n        run: " + CanonicalGoPackSmokeCommand
	prefix, suffix, ok := strings.Cut(text, old)
	if !ok {
		t.Fatal("failed to locate vet-before-tests workflow fixture block")
	}
	writeFile(t, filepath.Join(repo, ".github", "workflows", "release-gate.yml"), prefix+new+suffix)

	gate := ciReleaseGate(repo)
	if gate.Ready {
		t.Fatalf("CI release gate accepted tests before vet: %+v", gate)
	}
	assertWarningContains(t, gate.Warnings, "must run go vet before go test in go-checks-linux")
}

func TestCIReleaseGateInventoryDetectsDrift(t *testing.T) {
	repo := t.TempDir()
	writeFile(t, filepath.Join(repo, ".github", "workflows", "release-gate.yml"), `name: release-gate
on:
  push:
    branches: [main]
  pull_request:
jobs:
  go-checks:
    name: Go release checks
    runs-on: ubuntu-latest
    steps:
      - name: Release inventory
        run: go run ./cmd/rekit -- -Command release-check -Format json
      - name: Go tests
        run: go test ./...
      - name: Broad matrix
        run: pack-smoke-matrix.ps1
`)
	gate := ciReleaseGate(repo)
	if gate.Ready || gate.DefinitionReady || gate.DefinitionReady != gate.Ready {
		t.Fatalf("CI release gate unexpectedly ready despite drift: %+v", gate)
	}
	if gate.Kind != "workflow-definition" || gate.ReadyMeaning != "workflow-definition" || gate.ProvesRemoteExecution || gate.ProvesRemoteGreen {
		t.Fatalf("drifted CI release gate changed readiness meaning: %+v", gate)
	}
	assertWarningContains(t, gate.Warnings, "go-checks-windows")
	assertWarningContains(t, gate.Warnings, "go-checks-macos")
	assertWarningContains(t, gate.Warnings, "go run ./cmd/rekit -- -Command status")
	assertWarningContains(t, gate.Warnings, "go vet ./...")
	assertWarningContains(t, gate.Warnings, CanonicalGoPackSmokeCommand)
	assertWarningContains(t, gate.Warnings, "pack-smoke-matrix.ps1")
}

func TestCIReleaseGateMissingWorkflowInitializesDefinitionMeaning(t *testing.T) {
	gate := ciReleaseGate(t.TempDir())
	if gate.Ready || gate.DefinitionReady || gate.DefinitionReady != gate.Ready {
		t.Fatalf("missing CI workflow should make its definition not ready: %+v", gate)
	}
	if gate.Kind != "workflow-definition" || gate.ReadyMeaning != "workflow-definition" || gate.ProvesRemoteExecution || gate.ProvesRemoteGreen {
		t.Fatalf("missing CI workflow returned incomplete readiness meaning: %+v", gate)
	}
}

func TestReadinessLayersWorkflowDefinitionReadyDoesNotPromoteRemoteCI(t *testing.T) {
	result := Result{CIReleaseGate: CIReleaseGate{
		Ready:                 true,
		Kind:                  "workflow-definition",
		DefinitionReady:       true,
		ReadyMeaning:          "workflow-definition",
		ProvesRemoteExecution: false,
		ProvesRemoteGreen:     false,
	}}

	layers := readinessLayers(result)
	if layers.RemoteCI.State != "not-observed" || layers.RemoteCI.StructuredObservationPresent || layers.RemoteCI.CanClaimGreen {
		t.Fatalf("workflow definition readiness promoted remote CI truth: gate=%+v remote=%+v", result.CIReleaseGate, layers.RemoteCI)
	}
}

func TestReadinessLayersLocalReceiptReadyDoesNotPromoteRemoteCI(t *testing.T) {
	result := Result{
		Ready: true,
		ReleaseHandoff: ReleaseHandoff{LatestBatch: ReleaseHandoffLatestBatch{Handoff: ReleaseHandoffLatestBatchHandoff{
			LocalValidationReceipt: &LocalValidationReceiptInspection{
				Ready: true,
				State: "validated-implementation-commit",
			},
		}}},
	}

	layers := readinessLayers(result)
	if !layers.RepositoryInventory.Ready || layers.RepositoryInventory.State != "ready" {
		t.Fatalf("repository inventory layer did not preserve Result.Ready: %+v", layers.RepositoryInventory)
	}
	if !layers.LocalValidation.Ready || !layers.LocalValidation.ExactReceiptInspectionPresent || layers.LocalValidation.State != "validated-implementation-commit" {
		t.Fatalf("exact local validation receipt was not projected: %+v", layers.LocalValidation)
	}
	if layers.RemoteCI.State != "not-observed" || layers.RemoteCI.StructuredObservationPresent || layers.RemoteCI.CanClaimGreen {
		t.Fatalf("local validation receipt promoted remote CI truth: %+v", layers.RemoteCI)
	}
}

func TestReadinessLayersPostPushReadyDoesNotPromoteRemoteCI(t *testing.T) {
	result := Result{
		ReleaseHandoff: ReleaseHandoff{LatestBatch: ReleaseHandoffLatestBatch{Handoff: ReleaseHandoffLatestBatchHandoff{
			PostPushReceipt: &ReleaseHandoffPostPushReceipt{
				Ready: true,
				State: "post-push-complete",
			},
		}}},
	}

	layers := readinessLayers(result)
	if !layers.GitLocalPublication.Ready || !layers.GitLocalPublication.ExactPostPushObservationPresent || layers.GitLocalPublication.State != "post-push-complete" || !layers.GitLocalPublication.LocalTrackingRefOnly {
		t.Fatalf("exact post-push local tracking-ref observation was not projected: %+v", layers.GitLocalPublication)
	}
	if layers.RemoteCI.State != "not-observed" || layers.RemoteCI.StructuredObservationPresent || layers.RemoteCI.CanClaimGreen {
		t.Fatalf("post-push observation promoted remote CI truth: %+v", layers.RemoteCI)
	}
}

func TestReadinessLayersProseRemoteGreenIsNonAuthoritative(t *testing.T) {
	latest := latestBatchSummaryFromData("docs/batch-plan.md", []byte("### Batch 816：fixture\n\n状态：已完成 fixture。\n\n目标：fixture。\n\n验证结果：remote CI green.\n"))
	if latest.Handoff.RemoteReleaseGate != "green" || latest.Handoff.RemoteReleaseGateDetail == nil || !latest.Handoff.RemoteReleaseGateDetail.CanClaimGreen {
		t.Fatalf("fixture did not exercise the legacy prose remote-green claim: %+v", latest.Handoff)
	}

	layers := readinessLayers(Result{ReleaseHandoff: ReleaseHandoff{LatestBatch: latest}})
	if layers.RemoteCI.State != "not-observed" || layers.RemoteCI.StructuredObservationPresent || layers.RemoteCI.CanClaimGreen {
		t.Fatalf("prose remote green promoted machine truth: %+v", layers.RemoteCI)
	}
	claim := layers.RemoteCI.DocumentedClaim
	if !claim.Present || claim.Claim != "green" || claim.Source != "releaseHandoff.latestBatch.handoff.remoteReleaseGate" || claim.Authoritative {
		t.Fatalf("prose remote green was not retained as a non-authoritative documented claim: %+v", claim)
	}
}

func TestReleaseCheckJSONCompatibilityAddsExplicitReadinessLayers(t *testing.T) {
	repo := cleanReleaseRepoRoot(t)
	writeCompletedReleaseHandoffLatestBatchFixture(t, repo)
	result, err := Build(repo)
	if err != nil {
		t.Fatal(err)
	}
	data, err := json.Marshal(result)
	if err != nil {
		t.Fatal(err)
	}

	var legacy struct {
		Command       string `json:"command"`
		SchemaVersion int    `json:"schemaVersion"`
		IsMutation    bool   `json:"isMutation"`
		Ready         bool   `json:"ready"`
		Summary       string `json:"summary"`
		CIReleaseGate struct {
			WorkflowPath string `json:"workflowPath"`
			Ready        bool   `json:"ready"`
			Summary      string `json:"summary"`
		} `json:"ciReleaseGate"`
	}
	if err := json.Unmarshal(data, &legacy); err != nil {
		t.Fatal(err)
	}
	if legacy.Command != result.Command || legacy.SchemaVersion != 1 || legacy.IsMutation != result.IsMutation || legacy.Ready != result.Ready || legacy.Summary != result.Summary || legacy.CIReleaseGate.WorkflowPath != result.CIReleaseGate.WorkflowPath || legacy.CIReleaseGate.Ready != result.CIReleaseGate.Ready || legacy.CIReleaseGate.Summary != result.CIReleaseGate.Summary {
		t.Fatalf("existing release-check JSON fields changed: legacy=%+v result=%+v", legacy, result)
	}

	var wire map[string]json.RawMessage
	if err := json.Unmarshal(data, &wire); err != nil {
		t.Fatal(err)
	}
	for _, key := range []string{"command", "schemaVersion", "isMutation", "repoRoot", "ready", "summary", "gateProfile", "ciReleaseGate", "recommendedMinimum", "requiredCommands", "documents", "packs", "productionRegistry", "productionPacks", "powerShellDeprecation", "goNativePublicSurface", "publicFacadeRemoval", "caseShim", "publicDefaultDocs", "releaseHandoff", "heavyToolGateActions", "boundaries", "knownGaps", "warnings", "readinessLayers"} {
		if _, present := wire[key]; !present {
			t.Fatalf("release-check JSON omitted compatible field %q", key)
		}
	}
	var gateWire map[string]json.RawMessage
	if err := json.Unmarshal(wire["ciReleaseGate"], &gateWire); err != nil {
		t.Fatal(err)
	}
	for _, key := range []string{"workflowPath", "ready", "summary", "workflowChecks", "jobs", "requiredCommands", "forbiddenStrings", "warnings", "kind", "definitionReady", "readyMeaning", "provesRemoteExecution", "provesRemoteGreen"} {
		if _, present := gateWire[key]; !present {
			t.Fatalf("ciReleaseGate JSON omitted field %q", key)
		}
	}
	var readinessWire map[string]json.RawMessage
	if err := json.Unmarshal(wire["readinessLayers"], &readinessWire); err != nil {
		t.Fatal(err)
	}
	for _, key := range []string{"repositoryInventory", "localValidation", "gitLocalPublication", "remoteCI", "formalRelease"} {
		if _, present := readinessWire[key]; !present {
			t.Fatalf("readinessLayers JSON omitted layer %q", key)
		}
	}
	for layer, keys := range map[string][]string{
		"repositoryInventory": {"state", "ready"},
		"localValidation":     {"state", "ready", "exactReceiptInspectionPresent"},
		"gitLocalPublication": {"state", "ready", "exactPostPushObservationPresent", "localTrackingRefOnly"},
		"remoteCI":            {"state", "structuredObservationPresent", "canClaimGreen", "documentedClaim"},
		"formalRelease":       {"state", "canClaimReleased"},
	} {
		var layerWire map[string]json.RawMessage
		if err := json.Unmarshal(readinessWire[layer], &layerWire); err != nil {
			t.Fatal(err)
		}
		for _, key := range keys {
			if _, present := layerWire[key]; !present {
				t.Fatalf("readinessLayers.%s JSON omitted field %q", layer, key)
			}
		}
	}
	var layersWire struct {
		GitLocalPublication struct {
			LocalTrackingRefOnly bool `json:"localTrackingRefOnly"`
		} `json:"gitLocalPublication"`
		RemoteCI struct {
			State                        string `json:"state"`
			StructuredObservationPresent bool   `json:"structuredObservationPresent"`
			CanClaimGreen                bool   `json:"canClaimGreen"`
			DocumentedClaim              struct {
				Authoritative bool `json:"authoritative"`
			} `json:"documentedClaim"`
		} `json:"remoteCI"`
		FormalRelease struct {
			State            string `json:"state"`
			CanClaimReleased bool   `json:"canClaimReleased"`
		} `json:"formalRelease"`
	}
	if err := json.Unmarshal(wire["readinessLayers"], &layersWire); err != nil {
		t.Fatal(err)
	}
	if !layersWire.GitLocalPublication.LocalTrackingRefOnly || layersWire.RemoteCI.State != "not-observed" || layersWire.RemoteCI.StructuredObservationPresent || layersWire.RemoteCI.CanClaimGreen || layersWire.RemoteCI.DocumentedClaim.Authoritative || layersWire.FormalRelease.State != "not-evaluated" || layersWire.FormalRelease.CanClaimReleased {
		t.Fatalf("readiness layer JSON semantics drifted: %+v", layersWire)
	}
}

func assertCIJob(t *testing.T, gate CIReleaseGate, id, name, runsOn string) {
	t.Helper()
	for _, job := range gate.Jobs {
		if job.ID == id {
			if !job.Present || !job.Required || job.Name != name || job.RunsOn != runsOn {
				t.Fatalf("CI job %s = %+v, want name=%q runsOn=%q present/required", id, job, name, runsOn)
			}
			return
		}
	}
	t.Fatalf("missing CI job %s: %+v", id, gate.Jobs)
}

func assertCICommand(t *testing.T, gate CIReleaseGate, jobID, command string) {
	t.Helper()
	for _, item := range gate.RequiredCommands {
		if item.Job == jobID && item.Command == command {
			if !item.Present || !item.Required {
				t.Fatalf("CI command %s/%s = %+v, want present/required", jobID, command, item)
			}
			return
		}
	}
	t.Fatalf("missing CI command %s/%s: %+v", jobID, command, gate.RequiredCommands)
}

func TestGoNativePublicSurfaceInventoryFromRepo(t *testing.T) {
	repo := repoRoot(t)
	inventory := goNativePublicSurface(repo)
	surfaceCounts := GoNativePublicSurfaceCountsFor(inventory)
	if !inventory.Ready || inventory.Summary != "Go-native public command surface inventory ok" || surfaceCounts.Warnings != 0 {
		t.Fatalf("unexpected Go-native public surface inventory: %+v", inventory)
	}
	if inventory.Entrypoint != "cmd/rekit" || !inventory.EntrypointPresent || inventory.CommandCatalogPath != "internal/rekit/commands/commands.go" || !inventory.CommandCatalogPresent || inventory.DefaultCommand != "status" || inventory.AlternativePattern != "go run ./cmd/rekit -- -Command <command>" || !inventory.UnsupportedCommandDiagnosticPresent {
		t.Fatalf("unexpected Go-native public surface flags: %+v", inventory)
	}
	coverageCounts := surfaceCounts.Coverage
	if surfaceCounts.Catalog.Commands != 32 || surfaceCounts.Catalog.Empty != 0 || surfaceCounts.Catalog.Duplicates != 0 || coverageCounts.Commands != 32 || coverageCounts.HandlerCommands != 32 || coverageCounts.SymbolCommands != 32 || coverageCounts.ProfileCommands != 32 || coverageCounts.CommandProfiles != 32 || coverageCounts.HandlerMissing != 0 || coverageCounts.HandlerUnknown != 0 || coverageCounts.SymbolMissing != 0 || coverageCounts.SymbolUnknown != 0 || coverageCounts.ProfileMissing != 0 || coverageCounts.ProfileUnknown != 0 || surfaceCounts.MutationBoundaryInventory.Rows != 8 || surfaceCounts.MutationBoundaryInventory.Unknown != 0 {
		t.Fatalf("Go-native public surface omitted expected command coverage: %+v", inventory)
	}
	for _, command := range []string{"attach", "bootstrap", "complete", "continue", "control", "doctor", "gate", "handoff", "init", "migrate-state", "next-batch", "note", "onboard", "overview", "packs", "plan-subagents", "promote", "reconcile", "release-check", "release-run", "repair", "run-current-loop", "run-current-step", "run-driver-step", "run-reviewer-step", "run-reviewer-wave", "start", "status", "sync", "update", "validate"} {
		if !slices.Contains(inventory.Commands, command) || !slices.Contains(inventory.HandlerCommands, command) {
			t.Fatalf("Go-native public command %s missing from catalog or handler coverage: %+v", command, inventory)
		}
	}
	if len(inventory.RuntimeOwners) != 69 {
		t.Fatalf("Go-native exact runtime owner inventory=%d, want 69", len(inventory.RuntimeOwners))
	}
	if warnings := GoNativePublicRuntimeOwnerWarningsFor(inventory.RuntimeOwners, inventory.Commands); len(warnings) != 0 {
		t.Fatalf("Go-native exact runtime owner inventory drifted: %v", warnings)
	}
	assertGoNativeRuntimeOwner(t, inventory.RuntimeOwners, "sync", commands.MutationModeCurrentSync, "pre-runtime-exclusive-owner", "wantsCurrentSyncMaintenance", "", "validateCurrentSyncMaintenanceOptions", "inline-func", "runWithOptions->runPreRuntimeCommand->inline-func")
	assertGoNativeRuntimeOwner(t, inventory.RuntimeOwners, "sync", commands.MutationModeOrdinarySync, "scoped-runtime-owner", "resolveSyncCommandMode", "bindSyncCommand", "validateSyncCommand", "handleSyncCommand", "runWithOptions->runOwnedCommand->runScopedCommand->executeScopedCommandRoute->handleSyncCommand")
	assertGoNativeRuntimeOwner(t, inventory.RuntimeOwners, "plan-subagents", commands.MutationModeReviewerIntake, "scoped-runtime-owner", "resolvePlanSubagentsCommandMode", "bindPlanSubagentsCommand", "validatePlanSubagentsCommand", "handlePlanSubagentsCommand", "runWithOptions->runOwnedCommand->runScopedCommand->executeScopedCommandRoute->handlePlanSubagentsCommand")
	assertGoNativeRuntimeOwner(t, inventory.RuntimeOwners, "*", "pending-current-sync-recovery", "pre-runtime-interceptor-owner", "inline-func", "", "inline-func", "runCurrentSyncRecoveryFrontDoor", "runWithOptions->runPreRuntimeCommand->runCurrentSyncRecoveryFrontDoor")
	if surfaceCounts.SymbolCatalog.Symbols != 32 || surfaceCounts.SymbolCatalog.EmptySymbols != 0 || surfaceCounts.SymbolCatalog.EmptyCommands != 0 || inventory.SymbolCommands["Control"] != "control" || inventory.SymbolCommands["MigrateState"] != "migrate-state" || inventory.SymbolCommands["NextBatch"] != "next-batch" || inventory.SymbolCommands["PlanSubagents"] != "plan-subagents" || inventory.SymbolCommands["Reconcile"] != "reconcile" || inventory.SymbolCommands["ReleaseCheck"] != "release-check" || inventory.SymbolCommands["ReleaseRun"] != "release-run" || inventory.SymbolCommands["RunCurrentLoop"] != "run-current-loop" || inventory.SymbolCommands["RunCurrentStep"] != "run-current-step" {
		t.Fatalf("Go-native public symbol catalog drifted: %+v", inventory.SymbolCommands)
	}
	profiles := map[string]commands.PublicProfile{}
	for _, profile := range inventory.CommandProfiles {
		profiles[profile.Command] = profile
	}
	if surfaceCounts.ProfileCatalog.Rows != 32 || surfaceCounts.ProfileCatalog.Empty != 0 || surfaceCounts.ProfileCatalog.Duplicates != 0 || surfaceCounts.ProfileCatalog.UnknownBoundaries != 0 || surfaceCounts.ProfileCatalog.HeavyTool != 0 || surfaceCounts.ProfileCatalog.AuthorityConfirmed != 0 || surfaceCounts.ProfileCatalog.WritesKitNoReview != 0 || surfaceCounts.ProfileCatalog.ReviewNoApply != 0 || profiles["control"].MutationBoundary != commands.BoundaryCaseLocalReviewFirst || !profiles["control"].WritesCase || !profiles["control"].ReviewFirst || !profiles["control"].ApplyRequired || profiles["control"].HeavyTool || profiles["control"].AuthorityConfirmed || profiles["release-check"].MutationBoundary != commands.BoundaryReadOnly || profiles["release-check"].IsMutation || profiles["release-run"].MutationBoundary != commands.BoundaryLocalValidationReceipt || !profiles["release-run"].IsMutation || profiles["migrate-state"].MutationBoundary != commands.BoundaryCaseLocalReviewFirst || !profiles["migrate-state"].WritesCase || !profiles["migrate-state"].ReviewFirst || !profiles["migrate-state"].ApplyRequired || profiles["next-batch"].MutationBoundary != commands.BoundaryKitReviewFirst || !profiles["next-batch"].WritesKit || !profiles["next-batch"].ReviewFirst || !profiles["next-batch"].ApplyRequired || profiles["reconcile"].MutationBoundary != commands.BoundaryCaseLocalApply || !profiles["reconcile"].WritesCase || !profiles["reconcile"].ApplyRequired || profiles["run-current-loop"].MutationBoundary != commands.BoundaryCaseLocalReviewFirst || !profiles["run-current-loop"].WritesCase || !profiles["run-current-loop"].ReviewFirst || !profiles["run-current-loop"].ApplyRequired || profiles["run-current-step"].MutationBoundary != commands.BoundaryCaseLocalReviewFirst || !profiles["run-current-step"].WritesCase || !profiles["run-current-step"].ReviewFirst || !profiles["run-current-step"].ApplyRequired || !profiles["promote"].WritesKit || !profiles["promote"].ReviewFirst || profiles["sync"].WritesKit || !profiles["sync"].WritesCase || !slices.Contains(inventory.MutationBoundaries, commands.BoundaryKitReviewFirst) {
		t.Fatalf("Go-native public command profiles drifted: profiles=%+v boundaries=%+v", inventory.CommandProfiles, inventory.MutationBoundaries)
	}
	profileSummaryCounts := surfaceCounts.ProfileSummary
	if profileSummaryCounts.Total != 32 || surfaceCounts.ProfileTotal != 32 || profileSummaryCounts.ReadOnly != 5 || profileSummaryCounts.Mutating != 27 || profileSummaryCounts.WritesCase != 24 || profileSummaryCounts.WritesKit != 2 || profileSummaryCounts.ReviewFirst != 14 || profileSummaryCounts.ApplyRequired != 24 || profileSummaryCounts.HeavyTool != 0 || profileSummaryCounts.AuthorityConfirmed != 0 || profileSummaryCounts.BoundaryReadOnly != 5 || profileSummaryCounts.BoundaryLocalValidation != 1 || profileSummaryCounts.BoundaryCaseLocalApply != 9 || profileSummaryCounts.BoundaryCaseLocalWriteback != 1 || profileSummaryCounts.BoundaryCaseLocalReview != 12 || profileSummaryCounts.BoundaryKitReview != 2 {
		t.Fatalf("Go-native public command profile summary drifted: %+v", inventory.CommandProfileSummary)
	}
	if strings.Join(inventory.CommandProfileGroups.ReadOnly, ",") != "doctor,packs,release-check,status,validate" || strings.Join(inventory.CommandProfileGroups.ReviewFirst, ",") != "complete,control,migrate-state,next-batch,onboard,promote,reopen,run-current-loop,run-current-step,run-driver-step,run-reviewer-step,run-reviewer-wave,sync,update" || strings.Join(inventory.CommandProfileGroups.WritesKit, ",") != "next-batch,promote" || surfaceCounts.Groups.HeavyTool != 0 || surfaceCounts.Groups.AuthorityConfirmed != 0 || surfaceCounts.Groups.CaseLocalApply != 9 || surfaceCounts.Groups.CaseLocalReviewWriteback != 1 || surfaceCounts.Groups.CaseLocalReviewFirst != 12 || surfaceCounts.Groups.LocalValidationReceipt != 1 {
		t.Fatalf("Go-native public command profile groups drifted: %+v", inventory.CommandProfileGroups)
	}
	firstBoundaryCounts := GoNativePublicSurfaceBoundaryRowCountsFor(inventory.CommandProfileBoundaries[0])
	lastBoundaryCounts := GoNativePublicSurfaceBoundaryRowCountsFor(inventory.CommandProfileBoundaries[len(inventory.CommandProfileBoundaries)-1])
	if surfaceCounts.Boundaries.Rows != 8 || surfaceCounts.Boundaries.Commands != 32 || surfaceCounts.Boundaries.CountedCommands != 32 || surfaceCounts.Boundaries.Unknown != 0 || surfaceCounts.Boundaries.Duplicates != 0 || surfaceCounts.Boundaries.CountMismatches != 0 || surfaceCounts.Boundaries.Unsorted != 0 || surfaceCounts.Boundaries.SummaryMismatches != 0 || surfaceCounts.Boundaries.GroupMismatches != 0 || surfaceCounts.Boundaries.Missing != 0 || surfaceCounts.Boundaries.CoverageMismatches != 0 || inventory.CommandProfileBoundaries[0].Boundary != commands.BoundaryCaseLocalAppend || firstBoundaryCounts.Count != 1 || firstBoundaryCounts.Commands != 1 || strings.Join(inventory.CommandProfileBoundaries[1].Commands, ",") != "attach,bootstrap,continue,gate,handoff,init,reconcile,repair,start" || strings.Join(inventory.CommandProfileBoundaries[4].Commands, ",") != "complete,control,migrate-state,onboard,reopen,run-current-loop,run-current-step,run-driver-step,run-reviewer-step,run-reviewer-wave,sync,update" || strings.Join(inventory.CommandProfileBoundaries[5].Commands, ",") != "next-batch,promote" || inventory.CommandProfileBoundaries[len(inventory.CommandProfileBoundaries)-1].Boundary != commands.BoundaryReadOnly || lastBoundaryCounts.Count != 5 || lastBoundaryCounts.Commands != 5 {
		t.Fatalf("Go-native public command profile boundary rows drifted: %+v", inventory.CommandProfileBoundaries)
	}
	if surfaceCounts.Policies.Rows != 5 || surfaceCounts.Policies.Violations != 0 || surfaceCounts.Policies.ViolationCommands != 0 || inventory.CommandProfilePolicies[0].Policy != commands.PublicProfilePolicyNoHeavyTool || !inventory.CommandProfilePolicies[0].Ready || inventory.CommandProfilePolicies[3].Policy != commands.PublicProfilePolicyReviewFirstApplyRequired || GoNativePublicSurfacePolicyRowCountsFor(inventory.CommandProfilePolicies[3]).Commands != 0 {
		t.Fatalf("Go-native public command profile policy rows drifted: %+v", inventory.CommandProfilePolicies)
	}
	if !inventory.FacadeRemovalReady || surfaceCounts.FacadeRemoval.Rows != 6 || surfaceCounts.FacadeRemoval.NotReady != 0 || inventory.FacadeRemovalPrerequisites[0].Name != "entrypoint" || !inventory.FacadeRemovalPrerequisites[0].Ready || inventory.FacadeRemovalPrerequisites[2].Name != "runtime-owner-inventory" || !inventory.FacadeRemovalPrerequisites[2].Ready || inventory.FacadeRemovalPrerequisites[5].Name != "unsupported-command-diagnostic" || !inventory.FacadeRemovalPrerequisites[5].Ready {
		t.Fatalf("Go-native public surface facade removal prerequisites drifted: ready=%t prerequisites=%+v", inventory.FacadeRemovalReady, inventory.FacadeRemovalPrerequisites)
	}
}

func assertGoNativeRuntimeOwner(t *testing.T, owners []GoNativePublicRuntimeOwner, command, mode, kind, resolver, binder, validator, handler, callPath string) {
	t.Helper()
	for _, owner := range owners {
		if owner.Command == command && owner.Mode == mode {
			if owner.OwnerKind != kind || owner.Resolver != resolver || owner.Binder != binder || owner.Validator != validator || owner.Handler != handler || owner.PublicationOwner != handler || owner.CallPath != callPath {
				t.Fatalf("Go-native runtime owner %s mode %s drifted: %+v", command, mode, owner)
			}
			return
		}
	}
	t.Fatalf("Go-native runtime owner missing: %s mode %s", command, mode)
}

func TestGoNativePublicRuntimeOwnerWarningsFailClosed(t *testing.T) {
	inventory := goNativePublicSurface(repoRoot(t))
	for _, test := range []struct {
		name   string
		mutate func([]GoNativePublicRuntimeOwner) []GoNativePublicRuntimeOwner
		want   string
	}{
		{
			name: "missing callback",
			mutate: func(owners []GoNativePublicRuntimeOwner) []GoNativePublicRuntimeOwner {
				owners[0].Handler = ""
				return owners
			},
			want: "incomplete",
		},
		{
			name: "duplicate exact scope",
			mutate: func(owners []GoNativePublicRuntimeOwner) []GoNativePublicRuntimeOwner {
				for _, owner := range owners {
					if owner.Command != "*" {
						return append(owners, owner)
					}
				}
				return owners
			},
			want: "duplicates scope",
		},
		{
			name: "missing exact mode",
			mutate: func(owners []GoNativePublicRuntimeOwner) []GoNativePublicRuntimeOwner {
				for index, owner := range owners {
					if owner.Command != "*" {
						return append(owners[:index], owners[index+1:]...)
					}
				}
				return owners
			},
			want: "missing scope",
		},
		{
			name: "wrong pre-runtime call path",
			mutate: func(owners []GoNativePublicRuntimeOwner) []GoNativePublicRuntimeOwner {
				for index := range owners {
					if owners[index].OwnerKind == "pre-runtime-exclusive-owner" {
						owners[index].CallPath = "runWithOptions->runOwnedCommand"
						break
					}
				}
				return owners
			},
			want: "pre-runtime owner inventory is incomplete",
		},
		{
			name: "missing recovery interceptor",
			mutate: func(owners []GoNativePublicRuntimeOwner) []GoNativePublicRuntimeOwner {
				out := owners[:0]
				for _, owner := range owners {
					if owner.OwnerKind != "pre-runtime-interceptor-owner" {
						out = append(out, owner)
					}
				}
				return out
			},
			want: "interceptor owner count drifted",
		},
	} {
		t.Run(test.name, func(t *testing.T) {
			owners := append([]GoNativePublicRuntimeOwner{}, inventory.RuntimeOwners...)
			warnings := GoNativePublicRuntimeOwnerWarningsFor(test.mutate(owners), inventory.Commands)
			if !slices.ContainsFunc(warnings, func(warning string) bool { return strings.Contains(warning, test.want) }) {
				t.Fatalf("runtime owner warnings=%v, want containing %q", warnings, test.want)
			}
		})
	}
}

func TestGoNativePublicSurfaceInventoryDetectsDispatcherDrift(t *testing.T) {
	repo := t.TempDir()
	writeFile(t, filepath.Join(repo, "cmd", "rekit", "main.go"), "package main\n")
	writeFile(t, filepath.Join(repo, "internal", "rekit", "commands", "commands.go"), "package commands\n")
	writeFile(t, filepath.Join(repo, "internal", "rekit", "cli", "cli.go"), "package cli\n")
	writeFile(t, filepath.Join(repo, "internal", "rekit", "cli", "scoped_registry.go"), `package cli

import "github.com/shuiyu486/re-context-kits/internal/rekit/commands"

var directCommandRuntimeOwners = []directCommandRuntimeOwner{
	{Command: commands.Status, Handle: runStatus},
}

func runOwnedCommand(opt Options) error {
	return commands.UnsupportedError(opt.Command)
}
`)

	inventory := goNativePublicSurface(repo)
	if inventory.Ready {
		t.Fatalf("Go-native public surface unexpectedly ready despite dispatcher drift: %+v", inventory)
	}
	assertWarningContains(t, inventory.Warnings, "Go CLI dispatcher missing public command handler: release-check")
}

func TestGoNativePublicHandlerCommandsCombinesDirectAndScopedOwners(t *testing.T) {
	repo := t.TempDir()
	writeFile(t, filepath.Join(repo, "internal", "rekit", "cli", "scoped_registry.go"), `package cli

import "github.com/shuiyu486/re-context-kits/internal/rekit/commands"

var directCommandRuntimeOwners = []directCommandRuntimeOwner{
	{Command: commands.Status, Handle: runStatus},
}

var scopedCommandRuntimeOwners = []scopedCommandRuntimeOwner{
	defaultScopedCommandRuntimeOwner(commands.ReleaseCheck, bindReleaseCheckCommand, validateReleaseCheckCommand, handleReleaseCheckCommand),
}
`)

	got := goNativePublicHandlerCommands(repo)
	if strings.Join(got, ",") != "release-check,status" {
		t.Fatalf("handler coverage did not combine direct and scoped runtime owners: %v", got)
	}
}

func TestGoNativePublicHandlerCommandsRejectsIncompleteOrTextOnlyScopedRoutes(t *testing.T) {
	repo := t.TempDir()
	writeFile(t, filepath.Join(repo, "internal", "rekit", "cli", "scoped_registry.go"), `package cli

import "github.com/shuiyu486/re-context-kits/internal/rekit/commands"

// Direct owner text must not count as an actual callback.
const fakeDirectOwner = "{Command: commands.Status, Handle: runStatus}"

var directCommandRuntimeOwners = []directCommandRuntimeOwner{
	{Command: commands.Status, Handle: nil},
}

// Scope: commands.CommandScope{Command: commands.ReleaseCheck}
var scopedCommandRoutes = []scopedCommandRoute{
	{
		Scope: commands.CommandScope{Command: commands.ReleaseCheck, Mode: commands.MutationModeDefault},
		Bind: bindReleaseCheckCommand,
		Validate: validateReleaseCheckCommand,
		Handle: nil,
	},
}
`)

	if got := goNativePublicHandlerCommands(repo); len(got) != 0 {
		t.Fatalf("incomplete or text-only routes counted as handlers: %v", got)
	}
}

func TestPowerShellDeprecationInventoryFromRepo(t *testing.T) {
	repo := repoRoot(t)
	inventory := powerShellDeprecation(repo)
	counts := PowerShellDeprecationCountsFor(inventory)
	if !inventory.Ready || inventory.StrategyDocument != "docs/powershell-deprecation.md" || counts.Warnings != 0 {
		t.Fatalf("unexpected PowerShell deprecation inventory: %+v", inventory)
	}
	if counts.CommandOwnership == 0 || counts.ModuleStatus == 0 || counts.FreezeGates == 0 || counts.BlockedMigrations == 0 {
		t.Fatalf("PowerShell deprecation inventory omitted required sections: %+v", inventory)
	}
	assertCommandOwner(t, inventory, "sync / update", true, false)
	assertCommandOwner(t, inventory, "plan-subagents", true, false)
	assertCommandOwner(t, inventory, "actual heavy-tool", false, true)
	assertModuleStatus(t, inventory, "rekit/rekit.ps1")
	assertModuleStatus(t, inventory, "rekit/lib/B3.Commands.ps1")
	assertFallbackRetirement(t, inventory)
	assertFacadeRuntime(t, inventory)
	assertPublicFacade(t, inventory)
	assertModuleRemoval(t, inventory)
	assertModuleReferences(t, inventory)
	assertPublicFacadeRemoval(t, publicFacadeRemovalInventory(repo, inventory, goNativePublicSurface(repo)))
}

func TestPowerShellDeprecationInventoryDetectsDrift(t *testing.T) {
	repo := t.TempDir()
	writeFile(t, filepath.Join(repo, "rekit", "rekit.ps1"), `
param(
  [ValidateSet('status','new-default','plan-subagents')]
  [string]$Command = 'status'
)
function Test-RekitGoDefaultDelegationCommand {
  param([string]$Name)
  return (@('status','new-default') -contains $Name)
}
`)
	writeFile(t, filepath.Join(repo, "rekit", "lib", "Known.ps1"), "# known\n")
	writeFile(t, filepath.Join(repo, "rekit", "lib", "Extra.ps1"), "# extra\n")
	writeFile(t, filepath.Join(repo, "docs", "powershell-deprecation.md"), `# PowerShell runtime deprecation strategy

## 命令归属矩阵

| 区域 | 当前 owner | PowerShell 状态 | 冻结/删除策略 |
|---|---|---|---|
| status | Go default | façade + fallback | documented. |
| plan-subagents review artifacts | Go default | façade + fallback | review artifacts only. |
| actual heavy-tool 执行 | 未迁移 | blocked / manual gate | requires separate design. |

## PowerShell 模块状态

| 模块 | 状态 | 说明 |
|---|---|---|
| rekit/rekit.ps1 | façade-stable | entrypoint. |
| rekit/lib/Known.ps1 | compatibility | known module. |

## Freeze / deprecation gates

1. **Documented**：matrix exists.

## 禁止迁移清单

- actual heavy-tool execution.
`)

	inventory := powerShellDeprecation(repo)
	if inventory.Ready {
		t.Fatalf("inventory unexpectedly ready despite drift: %+v", inventory)
	}
	assertWarningContains(t, inventory.Warnings, "new-default")
	assertWarningContains(t, inventory.Warnings, "rekit/lib/Extra.ps1")
}

func TestPublicFacadeRemovalPlanDetectsMissingChecklist(t *testing.T) {
	repo := t.TempDir()
	writeFile(t, filepath.Join(repo, "docs", "powershell-deprecation.md"), `# PowerShell-free Go-native convergence roadmap

## Public façade removal plan inventory

删除必须是独立 removal batch。
`)

	plan := publicFacadeRemovalPlan(repo)
	if plan.Ready || plan.Summary != "public facade removal plan has warnings" || len(plan.RequiredPhrases) != 9 {
		t.Fatalf("public facade removal plan unexpectedly ready: %+v", plan)
	}
	assertWarningContains(t, plan.Warnings, "alternative-entrypoint")
	assertWarningContains(t, plan.Warnings, "recovery-plan")
	assertWarningContains(t, plan.Warnings, "no-heavy-tool-authority")
}

func TestPublicFacadeRemovalImpactDetectsUnclassifiedReference(t *testing.T) {
	repo := t.TempDir()
	writeFile(t, filepath.Join(repo, "rekit", "rekit.ps1"), "# facade\n")
	writeFile(t, filepath.Join(repo, "misc.txt"), "unexpected rekit.ps1 reference\n")

	impact := publicFacadeRemovalImpact(repo)
	if impact.Ready || impact.Summary != "public facade removal impact inventory has warnings" || len(impact.UnclassifiedReferences) != 1 || impact.UnclassifiedReferences[0].Path != "misc.txt" {
		t.Fatalf("public facade removal impact unexpectedly ready: %+v", impact)
	}
	assertWarningContains(t, impact.Warnings, "misc.txt")
}

func TestPublicFacadeRemovalImpactIgnoresClaudeWorktrees(t *testing.T) {
	repo := t.TempDir()
	writeFile(t, filepath.Join(repo, "rekit", "rekit.ps1"), "# facade\n")
	writeFile(t, filepath.Join(repo, "rekit", "tests", "facade-smoke.ps1"), "# facade-smoke.ps1\n")
	writeFile(t, filepath.Join(repo, ".claude", "worktrees", "agent-review", "misc.txt"), "generated rekit.ps1 reference\n")

	impact := publicFacadeRemovalImpact(repo)
	if !impact.Ready || impact.Summary != "public facade removal impact inventory ok" || len(impact.UnclassifiedReferences) != 0 {
		t.Fatalf("generated Claude worktree affected public facade removal impact: %+v", impact)
	}
}

func assertCommandOwner(t *testing.T, inventory PowerShellDeprecation, areaContains string, wantGoDefault, wantBlocked bool) {
	t.Helper()
	for _, row := range inventory.CommandOwnership {
		if strings.Contains(row.Area, areaContains) {
			if row.GoDefault != wantGoDefault || row.Blocked != wantBlocked {
				t.Fatalf("owner row %q = %+v, want goDefault=%t blocked=%t", areaContains, row, wantGoDefault, wantBlocked)
			}
			return
		}
	}
	t.Fatalf("missing command owner row containing %q: %+v", areaContains, inventory.CommandOwnership)
}

func assertModuleStatus(t *testing.T, inventory PowerShellDeprecation, path string) {
	t.Helper()
	for _, module := range inventory.ModuleStatus {
		if module.Path == path {
			if strings.TrimSpace(module.Status) == "" || strings.TrimSpace(module.Notes) == "" {
				t.Fatalf("module row %s has empty status/notes: %+v", path, module)
			}
			return
		}
	}
	t.Fatalf("missing module row %s: %+v", path, inventory.ModuleStatus)
}

func assertFallbackRetirement(t *testing.T, inventory PowerShellDeprecation) {
	t.Helper()
	fallback := inventory.FallbackRetirement
	counts := PowerShellDeprecationCountsFor(inventory).FallbackRetirement
	if !fallback.Ready || fallback.Summary != "PowerShell fallback retirement inventory ok" || counts.Warnings != 0 {
		t.Fatalf("unexpected fallback retirement inventory: %+v", fallback)
	}
	if counts.GoDefaultCommands != 32 || counts.NoFallbackCommands != 32 || counts.CandidateCommands != 0 || counts.RemovalCandidateModules != 0 || counts.RetiredModules != 13 {
		t.Fatalf("fallback retirement inventory omitted expected sections: %+v", fallback)
	}
	for _, command := range []string{"attach", "bootstrap", "complete", "continue", "control", "doctor", "gate", "handoff", "init", "migrate-state", "next-batch", "note", "overview", "packs", "plan-subagents", "promote", "reconcile", "release-check", "release-run", "repair", "run-current-loop", "run-current-step", "run-driver-step", "run-reviewer-step", "run-reviewer-wave", "start", "status", "sync", "update", "validate"} {
		if !slices.Contains(fallback.NoFallbackCommands, command) {
			t.Fatalf("NoFallbackCommands = %v, missing %s", fallback.NoFallbackCommands, command)
		}
	}
}

func assertFacadeRuntime(t *testing.T, inventory PowerShellDeprecation) {
	t.Helper()
	facade := inventory.FacadeRuntime
	counts := PowerShellDeprecationCountsFor(inventory).FacadeRuntime
	if !facade.Ready || facade.Summary != "PowerShell facade runtime dependency inventory ok" || facade.FacadePath != "rekit/rekit.ps1" || counts.Warnings != 0 {
		t.Fatalf("unexpected PowerShell facade runtime inventory: %+v", facade)
	}
	if facade.LegacyModuleImportsPresent || facade.CommandDispatcherPresent || !facade.NoFallbackGuardPresent || !facade.GoDelegationPresent || !facade.RetiredDispatcherError {
		t.Fatalf("unexpected PowerShell facade runtime dependency flags: %+v", facade)
	}
	if counts.ForbiddenPatterns == 0 || counts.RequiredPatterns == 0 {
		t.Fatalf("PowerShell facade runtime inventory omitted required pattern lists: %+v", facade)
	}
}

func assertPublicFacade(t *testing.T, inventory PowerShellDeprecation) {
	t.Helper()
	facade := inventory.PublicFacade
	counts := PowerShellDeprecationCountsFor(inventory).PublicFacade
	if !facade.Ready || facade.Summary != "PowerShell public facade retention inventory ok" || facade.FacadePath != "rekit/rekit.ps1" || counts.Warnings != 0 {
		t.Fatalf("unexpected PowerShell public facade inventory: %+v", facade)
	}
	if !facade.Present || !facade.Retained || !facade.MigrationBoundaryDocumented || !facade.RemovalBoundaryDocumented || facade.GoNativeAlternative != "go run ./cmd/rekit -- -Command <command>" {
		t.Fatalf("unexpected PowerShell public facade retention flags: %+v", facade)
	}
	if counts.CommandSurface != 32 || counts.GoDefaultCommands != 32 || counts.NoFallbackCommands != 32 {
		t.Fatalf("PowerShell public facade inventory omitted expected command lists: %+v", facade)
	}
	for _, command := range []string{"attach", "bootstrap", "complete", "continue", "control", "doctor", "gate", "handoff", "init", "migrate-state", "next-batch", "note", "overview", "packs", "plan-subagents", "promote", "reconcile", "release-check", "release-run", "repair", "run-current-loop", "run-current-step", "run-driver-step", "run-reviewer-step", "run-reviewer-wave", "start", "status", "sync", "update", "validate"} {
		if !slices.Contains(facade.CommandSurface, command) || !slices.Contains(facade.GoDefaultCommands, command) || !slices.Contains(facade.NoFallbackCommands, command) {
			t.Fatalf("public facade command %s missing from command lists: %+v", command, facade)
		}
	}
}

func assertPublicFacadeRemoval(t *testing.T, inventory PublicFacadeRemoval) {
	t.Helper()
	counts := PublicFacadeRemovalCountsFor(inventory)
	if !inventory.Ready || inventory.Summary != "public facade removal prerequisites ok" || counts.Warnings != 0 {
		t.Fatalf("unexpected public facade removal inventory: %+v", inventory)
	}
	if counts.Prerequisites != 8 || inventory.Prerequisites[0].Name != "public-facade-retained-boundary" || !inventory.Prerequisites[0].Ready || inventory.Prerequisites[2].Name != "go-native-public-surface" || !inventory.Prerequisites[2].Ready || inventory.Prerequisites[5].Name != "module-reference-blockers-clear" || !inventory.Prerequisites[5].Ready || inventory.Prerequisites[6].Name != "removal-plan-documented" || !inventory.Prerequisites[6].Ready || inventory.Prerequisites[7].Name != "removal-impact-inventoried" || !inventory.Prerequisites[7].Ready {
		t.Fatalf("public facade removal prerequisites drifted: %+v", inventory.Prerequisites)
	}
	planCounts := counts.Plan
	deletionGateCounts := counts.DeletionGates
	executionCounts := counts.ExecutionSteps
	impactCounts := counts.Impact
	if !inventory.RemovalPlan.Ready || inventory.RemovalPlan.Document != "docs/powershell-deprecation.md" || planCounts.Warnings != 0 || planCounts.RequiredPhrases != 9 || planCounts.ReplacementEntrypoints != 4 || planCounts.ReplacementValidationCommands != 32 || deletionGateCounts.Gates != 5 || deletionGateCounts.ValidationCommands != 40 || deletionGateCounts.ExitCriteria != 15 || deletionGateCounts.FailureSignals != 15 || deletionGateCounts.EscalationTriggers != 15 || deletionGateCounts.EscalationEvidence != 15 || deletionGateCounts.EscalationRecipients != 15 || deletionGateCounts.EscalationHandoffSteps != 15 || deletionGateCounts.EscalationDecisionOptions != 15 || deletionGateCounts.EscalationRetryConditions != 15 || deletionGateCounts.EscalationStopConditions != 15 || deletionGateCounts.EscalationResolutionArtifacts != 15 || deletionGateCounts.EscalationClosureChecks != 15 || deletionGateCounts.EscalationReopenConditions != 15 || deletionGateCounts.EscalationLedgerEvents != 15 || deletionGateCounts.EscalationStateTransitions != 15 || deletionGateCounts.EscalationBoundaryGuards != 15 || deletionGateCounts.EscalationAuditChecks != 15 || deletionGateCounts.VerificationArtifacts != 15 || deletionGateCounts.BlockedExecutionSteps != 10 || deletionGateCounts.RemediationActions != 15 || executionCounts.Steps != 5 || executionCounts.FailureSignals != 15 || executionCounts.RemediationActions != 15 || executionCounts.VerificationArtifacts != 15 || executionCounts.LedgerEvents != 15 || executionCounts.StateTransitions != 15 || executionCounts.EscalationTriggers != 15 || executionCounts.EscalationEvidence != 15 || executionCounts.EscalationRecipients != 15 || executionCounts.EscalationHandoffSteps != 15 || executionCounts.EscalationDecisionOptions != 15 || executionCounts.EscalationRetryConditions != 15 || executionCounts.EscalationStopConditions != 15 || executionCounts.EscalationResolutionArtifacts != 15 || executionCounts.EscalationClosureChecks != 15 || executionCounts.EscalationReopenConditions != 15 || executionCounts.EscalationLedgerEvents != 15 || executionCounts.EscalationStateTransitions != 15 || executionCounts.EscalationBoundaryGuards != 15 || executionCounts.EscalationAuditChecks != 15 || executionCounts.BoundaryGuards != 15 || executionCounts.AuditChecks != 15 || executionCounts.ValidationCommands != 40 || planCounts.BoundaryChecks != 6 || planCounts.BoundaryValidationCommands != 48 || planCounts.RecoverySteps != 4 || planCounts.RecoveryValidationCommands != 32 || planCounts.DocumentationTargets != 9 || planCounts.DocumentationValidationCommands != 72 || !publicFacadeRemovalHasReplacementEntrypoint(inventory.RemovalPlan, "canonical-rekit-skill") || !publicFacadeRemovalHasReplacementEntrypoint(inventory.RemovalPlan, "direct-go-cli") || !publicFacadeRemovalHasDeletionGate(inventory.RemovalPlan, "go-native-alternatives-ready") || !publicFacadeRemovalHasDeletionGate(inventory.RemovalPlan, "release-gate-green") || !publicFacadeRemovalHasExecutionStep(inventory.RemovalPlan, "delete-public-facade") || !publicFacadeRemovalHasExecutionStep(inventory.RemovalPlan, "rerun-release-gate") || !publicFacadeRemovalHasBoundaryCheck(inventory.RemovalPlan, "no-powershell-runtime-logic") || !publicFacadeRemovalHasBoundaryCheck(inventory.RemovalPlan, "no-external-effects") || !publicFacadeRemovalHasRecoveryStep(inventory.RemovalPlan, "restore-public-facade") || !publicFacadeRemovalHasDocumentationTarget(inventory.RemovalPlan, "docs/release-readiness.md") || !publicFacadeRemovalHasDocumentationTarget(inventory.RemovalPlan, "CHANGELOG.md") {
		t.Fatalf("public facade removal plan drifted: %+v", inventory.RemovalPlan)
	}
	if !inventory.RemovalImpact.Ready || inventory.RemovalImpact.FacadePath != "rekit/rekit.ps1" || !inventory.RemovalImpact.FacadePresent || impactCounts.Warnings != 0 || impactCounts.References == 0 || impactCounts.ReferenceCategories == 0 || impactCounts.WorkItems != impactCounts.ReferenceCategories || impactCounts.WorkItemValidationCommands != impactCounts.WorkItems*8 || impactCounts.MigrationTargets == 0 || impactCounts.MigrationValidationCommands != impactCounts.MigrationTargets*8 || impactCounts.SmokeMigrationTargets == 0 || impactCounts.SmokeMigrationValidationCommands != impactCounts.SmokeMigrationTargets*8 || impactCounts.UnclassifiedReferences != 0 || !publicFacadeRemovalHasImpactCategory(inventory.RemovalImpact, "public-facade-entrypoint") || !publicFacadeRemovalHasImpactCategory(inventory.RemovalImpact, "facade-compatibility-smoke") || !publicFacadeRemovalHasImpactWorkItem(inventory.RemovalImpact, "release-inventory-and-tests") || !publicFacadeRemovalHasMigrationTarget(inventory.RemovalImpact, "rekit/rekit.ps1") || !publicFacadeRemovalHasMigrationTarget(inventory.RemovalImpact, "docs/powershell-deprecation.md") || !publicFacadeRemovalHasSmokeMigrationTarget(inventory.RemovalImpact, "rekit/tests/facade-smoke.ps1") || !publicFacadeRemovalHasSmokeMigrationTarget(inventory.RemovalImpact, "rekit/tests/continue-whatif-smoke.ps1") {
		t.Fatalf("public facade removal impact drifted: %+v", inventory.RemovalImpact)
	}
}

func publicFacadeRemovalHasValidationSmoke(count int, commands []string) bool {
	return count == 8 && slices.Contains(commands, "go run ./cmd/rekit -- -Command release-check -Format json")
}

func publicFacadeRemovalHasReplacementEntrypoint(plan PublicFacadeRemovalPlan, name string) bool {
	for _, entrypoint := range plan.ReplacementEntrypoints {
		counts := PublicFacadeRemovalReplacementEntrypointCountsFor(entrypoint)
		if entrypoint.Name == name && entrypoint.Required && entrypoint.GoNativeBacked && strings.TrimSpace(entrypoint.Entrypoint) != "" && strings.TrimSpace(entrypoint.Audience) != "" && strings.TrimSpace(entrypoint.Purpose) != "" && publicFacadeRemovalHasValidationSmoke(counts.ValidationCommands, entrypoint.ValidationCommands) {
			return true
		}
	}
	return false
}

func publicFacadeRemovalHasDeletionGate(plan PublicFacadeRemovalPlan, name string) bool {
	for _, gate := range plan.DeletionGates {
		counts := PublicFacadeRemovalDeletionGateRowCountsFor(gate)
		if gate.Name == name && gate.Required && gate.BlocksRemoval && strings.TrimSpace(gate.Gate) != "" && counts.InputInventory > 0 && counts.BlockedExecutionSteps == 2 && slices.Contains(gate.BlockedExecutionSteps, "delete-public-facade") && slices.Contains(gate.BlockedExecutionSteps, "rerun-release-gate") && counts.ExitCriteria == 3 && counts.FailureSignals == 3 && counts.EscalationTriggers == 3 && counts.EscalationEvidence == 3 && counts.EscalationRecipients == 3 && counts.EscalationHandoffSteps == 3 && counts.EscalationDecisionOptions == 3 && counts.EscalationRetryConditions == 3 && counts.EscalationStopConditions == 3 && counts.EscalationResolutionArtifacts == 3 && counts.EscalationClosureChecks == 3 && counts.EscalationReopenConditions == 3 && counts.EscalationLedgerEvents == 3 && counts.EscalationStateTransitions == 3 && counts.EscalationBoundaryGuards == 3 && counts.EscalationAuditChecks == 3 && counts.VerificationArtifacts == 3 && counts.RemediationActions == 3 && publicFacadeRemovalHasValidationSmoke(counts.ValidationCommands, gate.ValidationCommands) {
			return true
		}
	}
	return false
}

func publicFacadeRemovalHasExecutionStep(plan PublicFacadeRemovalPlan, name string) bool {
	for _, step := range plan.ExecutionSteps {
		counts := PublicFacadeRemovalExecutionStepRowCountsFor(step)
		if step.Name == name && step.Required && strings.TrimSpace(step.Action) != "" && counts.DependsOn > 0 && counts.InputInventory > 0 && counts.OutputArtifacts > 0 && counts.FailureSignals == 3 && counts.RemediationActions == 3 && counts.VerificationArtifacts == 3 && counts.LedgerEvents == 3 && counts.StateTransitions == 3 && counts.EscalationTriggers == 3 && counts.EscalationEvidence == 3 && counts.EscalationRecipients == 3 && counts.EscalationHandoffSteps == 3 && counts.EscalationDecisionOptions == 3 && counts.EscalationRetryConditions == 3 && counts.EscalationStopConditions == 3 && counts.EscalationResolutionArtifacts == 3 && counts.EscalationClosureChecks == 3 && counts.EscalationReopenConditions == 3 && counts.EscalationLedgerEvents == 3 && counts.EscalationStateTransitions == 3 && counts.EscalationBoundaryGuards == 3 && counts.EscalationAuditChecks == 3 && counts.BoundaryGuards == 3 && counts.AuditChecks == 3 && publicFacadeRemovalHasValidationSmoke(counts.ValidationCommands, step.ValidationCommands) && !step.AllowsPowerShellRuntime && !step.AllowsExternalEffects {
			return true
		}
	}
	return false
}

func publicFacadeRemovalHasBoundaryCheck(plan PublicFacadeRemovalPlan, name string) bool {
	for _, check := range plan.BoundaryChecks {
		counts := PublicFacadeRemovalPlanBoundaryCheckCountsFor(check)
		if check.Name == name && check.Required && check.Preserved && strings.TrimSpace(check.Boundary) != "" && counts.Evidence > 0 && publicFacadeRemovalHasValidationSmoke(counts.ValidationCommands, check.ValidationCommands) {
			return true
		}
	}
	return false
}

func publicFacadeRemovalHasRecoveryStep(plan PublicFacadeRemovalPlan, name string) bool {
	for _, step := range plan.RecoverySteps {
		counts := PublicFacadeRemovalRecoveryStepCountsFor(step)
		if step.Name == name && step.Required && strings.TrimSpace(step.Action) != "" && counts.Paths > 0 && publicFacadeRemovalHasValidationSmoke(counts.ValidationCommands, step.ValidationCommands) {
			return true
		}
	}
	return false
}

func publicFacadeRemovalHasDocumentationTarget(plan PublicFacadeRemovalPlan, path string) bool {
	for _, target := range plan.DocumentationTargets {
		counts := PublicFacadeRemovalDocumentationTargetCountsFor(target)
		if target.Path == path && target.Required && strings.TrimSpace(target.Purpose) != "" && strings.TrimSpace(target.Action) != "" && publicFacadeRemovalHasValidationSmoke(counts.ValidationCommands, target.ValidationCommands) {
			return true
		}
	}
	return false
}

func publicFacadeRemovalHasImpactCategory(impact PublicFacadeRemovalImpact, name string) bool {
	for _, category := range impact.ReferenceCategories {
		if category.Name == name && category.Count > 0 {
			return true
		}
	}
	return false
}

func publicFacadeRemovalHasImpactWorkItem(impact PublicFacadeRemovalImpact, category string) bool {
	for _, item := range impact.WorkItems {
		counts := PublicFacadeRemovalImpactWorkItemCountsFor(item)
		if item.Category == category && item.Required && item.Count > 0 && counts.Paths > 0 && strings.TrimSpace(item.Action) != "" && publicFacadeRemovalHasValidationSmoke(counts.ValidationCommands, item.ValidationCommands) {
			return true
		}
	}
	return false
}

func publicFacadeRemovalHasMigrationTarget(impact PublicFacadeRemovalImpact, path string) bool {
	for _, target := range impact.MigrationTargets {
		counts := PublicFacadeRemovalMigrationTargetCountsFor(target)
		if target.Path == path && target.Required && target.GoNativePreferred && strings.TrimSpace(target.Action) != "" && publicFacadeRemovalHasValidationSmoke(counts.ValidationCommands, target.ValidationCommands) {
			return true
		}
	}
	return false
}

func publicFacadeRemovalHasSmokeMigrationTarget(impact PublicFacadeRemovalImpact, path string) bool {
	for _, target := range impact.SmokeMigrationTargets {
		counts := PublicFacadeRemovalSmokeMigrationTargetCountsFor(target)
		if target.Path == path && target.Required && target.GoNativePreferred && !target.AllowFacadeCompat && target.RetireFacadeAssertions && strings.TrimSpace(target.Action) != "" && publicFacadeRemovalHasValidationSmoke(counts.ValidationCommands, target.ValidationCommands) {
			return true
		}
	}
	return false
}

func assertModuleRemoval(t *testing.T, inventory PowerShellDeprecation) {
	t.Helper()
	removal := inventory.ModuleRemoval
	counts := PowerShellDeprecationCountsFor(inventory).ModuleRemoval
	if !removal.Ready || removal.Summary != "PowerShell module removal inventory ok" || counts.Warnings != 0 {
		t.Fatalf("unexpected PowerShell module removal inventory: %+v", removal)
	}
	if counts.CandidateModules != 0 || counts.RetiredModules != 13 || counts.FacadeRuntimeDependencies != 0 || counts.UndocumentedModules != 0 {
		t.Fatalf("PowerShell module removal inventory omitted expected sections: %+v", removal)
	}
	for _, module := range removal.RetiredModules {
		if strings.TrimSpace(module.Path) == "" || strings.TrimSpace(module.Status) == "" || strings.TrimSpace(module.Notes) == "" || module.Present || module.ReferencedByFacade {
			t.Fatalf("unexpected PowerShell retired module: %+v", module)
		}
	}
}

func assertModuleReferences(t *testing.T, inventory PowerShellDeprecation) {
	t.Helper()
	refs := inventory.ModuleReferences
	counts := PowerShellDeprecationCountsFor(inventory).ModuleReferences
	if !refs.Ready || refs.Summary != "PowerShell module reference inventory ok" || counts.Warnings != 0 {
		t.Fatalf("unexpected PowerShell module reference inventory: %+v", refs)
	}
	if counts.TotalReferences == 0 || counts.ActiveTestDependencies != 0 || counts.CompatibilityFixtures != 0 || counts.InventoryGuards == 0 || counts.RemovalBlockers != 0 || counts.UnclassifiedReferences != 0 {
		t.Fatalf("PowerShell module reference inventory omitted expected sections: %+v", refs)
	}
}

func TestPowerShellModuleReferenceTarget(t *testing.T) {
	tests := map[string]string{
		`rekit/lib/B3.State.ps1`:                     "rekit/lib/B3.State.ps1",
		`rekit\lib\B3.State.ps1`:                     "rekit/lib/B3.State.ps1",
		`rekit/lib/*.ps1`:                            "rekit/lib/*.ps1",
		`Join-Path $RekitRoot 'lib\B3.State.ps1'`:    "rekit/lib/B3.State.ps1",
		`Join-Path $RuntimeRoot 'lib\B3.State.ps1'`:  "rekit/lib/B3.State.ps1",
		`Join-Path $isolatedRoot 'lib\B3.State.ps1'`: "isolated/lib/B3.State.ps1",
		`fixture lib\B3.State.ps1`:                   "rekit/lib/B3.State.ps1",
		`no retired PowerShell module reference`:     "",
	}
	for line, want := range tests {
		if got := powerShellModuleReferenceTarget(line); got != want {
			t.Errorf("powerShellModuleReferenceTarget(%q) = %q, want %q", line, got, want)
		}
	}
}

func assertWarningContains(t *testing.T, warnings []string, want string) {
	t.Helper()
	for _, warning := range warnings {
		if strings.Contains(warning, want) {
			return
		}
	}
	t.Fatalf("warnings missing %q: %v", want, warnings)
}

func writeFile(t *testing.T, path, content string) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}
}

func cleanReleaseRepoRoot(t *testing.T) string {
	t.Helper()
	src := repoRoot(t)
	dst := filepath.Join(t.TempDir(), "repo")
	if err := filepath.WalkDir(src, func(path string, d os.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if d == nil {
			return nil
		}
		rel, err := filepath.Rel(src, path)
		if err != nil {
			return err
		}
		relSlash := filepath.ToSlash(rel)
		if rel == "." {
			return os.MkdirAll(dst, 0o755)
		}
		if d.IsDir() {
			if relSlash == ".git" || relSlash == ".codegraph" || releaseCheckTestVolatileCandidateDir(relSlash) {
				return filepath.SkipDir
			}
			info, err := d.Info()
			if err != nil {
				return err
			}
			return os.MkdirAll(filepath.Join(dst, rel), info.Mode().Perm())
		}
		info, err := d.Info()
		if err != nil {
			return err
		}
		if !info.Mode().IsRegular() {
			return nil
		}
		data, err := os.ReadFile(path)
		if err != nil {
			return err
		}
		target := filepath.Join(dst, rel)
		if err := os.MkdirAll(filepath.Dir(target), 0o755); err != nil {
			return err
		}
		return os.WriteFile(target, data, info.Mode().Perm())
	}); err != nil {
		t.Fatal(err)
	}
	return dst
}

func releaseCheckTestVolatileCandidateDir(relSlash string) bool {
	parts := strings.Split(relSlash, "/")
	if len(parts) >= 3 && parts[0] == "packs" && parts[2] == "promote-candidates" {
		return true
	}
	return len(parts) >= 4 && parts[0] == "packs" && parts[2] == "tooling" && parts[3] == "candidates"
}

func repoRoot(t *testing.T) string {
	t.Helper()
	wd, err := os.Getwd()
	if err != nil {
		t.Fatal(err)
	}
	for {
		if _, err := os.Stat(filepath.Join(wd, "go.mod")); err == nil {
			return wd
		}
		parent := filepath.Dir(wd)
		if parent == wd {
			t.Fatal("go.mod not found while locating repo root")
		}
		wd = parent
	}
}
