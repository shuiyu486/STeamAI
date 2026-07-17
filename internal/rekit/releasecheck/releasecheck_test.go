package releasecheck

import (
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"

	"github.com/shuiyu486/re-context-kits/internal/rekit/commands"
)

func TestReleaseCheckIncludesManifestHeavyToolGateActions(t *testing.T) {
	result, err := Build(repoRoot(t))
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
	if got := strings.Join(result.HeavyToolGateActions, ","); got != "debug,dump,full-trace,inject,network,patch,symex" {
		t.Fatalf("HeavyToolGateActions = %q", got)
	}
	for _, pack := range result.Packs {
		if pack.HeavyToolGates != 7 {
			t.Fatalf("pack %s HeavyToolGates = %d, want 7", pack.ID, pack.HeavyToolGates)
		}
	}
}

func TestCIReleaseGateInventoryFromRepo(t *testing.T) {
	gate := ciReleaseGate(repoRoot(t))
	counts := CIReleaseGateCountsFor(gate)
	if !gate.Ready || gate.WorkflowPath != ".github/workflows/release-gate.yml" || gate.Summary != "CI release gate inventory ok" || counts.Warnings != 0 {
		t.Fatalf("unexpected CI release gate inventory: %+v", gate)
	}
	if counts.WorkflowChecks == 0 || counts.Jobs != 3 || counts.RequiredCommands != 18 || counts.ForbiddenStrings == 0 {
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
		assertCICommand(t, gate, job, "go test ./...")
		assertCICommand(t, gate, job, "go vet ./...")
	}
	for _, forbidden := range gate.ForbiddenStrings {
		if forbidden.Present {
			t.Fatalf("forbidden CI release gate pattern present: %+v", forbidden)
		}
	}
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
	if gate.Ready {
		t.Fatalf("CI release gate unexpectedly ready despite drift: %+v", gate)
	}
	assertWarningContains(t, gate.Warnings, "go-checks-windows")
	assertWarningContains(t, gate.Warnings, "go-checks-macos")
	assertWarningContains(t, gate.Warnings, "go run ./cmd/rekit -- -Command status")
	assertWarningContains(t, gate.Warnings, "go vet ./...")
	assertWarningContains(t, gate.Warnings, "pack-smoke-matrix.ps1")
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
	if surfaceCounts.Commands != 19 || surfaceCounts.HandlerCommands != 19 || surfaceCounts.SymbolCommands != 19 || surfaceCounts.CommandProfiles != 19 || surfaceCounts.MutationBoundaries != 7 {
		t.Fatalf("Go-native public surface omitted expected command coverage: %+v", inventory)
	}
	for _, command := range []string{"attach", "bootstrap", "continue", "doctor", "gate", "handoff", "init", "note", "overview", "packs", "plan-subagents", "promote", "release-check", "repair", "start", "status", "sync", "update", "validate"} {
		if !slices.Contains(inventory.Commands, command) || !slices.Contains(inventory.HandlerCommands, command) {
			t.Fatalf("Go-native public command %s missing from catalog or handler coverage: %+v", command, inventory)
		}
	}
	if inventory.SymbolCommands["PlanSubagents"] != "plan-subagents" || inventory.SymbolCommands["ReleaseCheck"] != "release-check" {
		t.Fatalf("Go-native public symbol catalog drifted: %+v", inventory.SymbolCommands)
	}
	profiles := map[string]commands.PublicProfile{}
	for _, profile := range inventory.CommandProfiles {
		profiles[profile.Command] = profile
	}
	if profiles["release-check"].MutationBoundary != commands.BoundaryReadOnly || profiles["release-check"].IsMutation || !profiles["promote"].WritesKit || !profiles["promote"].ReviewFirst || profiles["sync"].WritesKit || !profiles["sync"].WritesCase || !slices.Contains(inventory.MutationBoundaries, commands.BoundaryKitReviewFirst) {
		t.Fatalf("Go-native public command profiles drifted: profiles=%+v boundaries=%+v", inventory.CommandProfiles, inventory.MutationBoundaries)
	}
	if inventory.CommandProfileSummary.Total != 19 || surfaceCounts.ReadOnly != 5 || surfaceCounts.Mutating != 14 || surfaceCounts.WritesCase != 13 || surfaceCounts.WritesKit != 1 || surfaceCounts.ReviewFirst != 3 || surfaceCounts.ApplyRequired != 11 || surfaceCounts.HeavyTool != 0 || surfaceCounts.AuthorityConfirmed != 0 || inventory.CommandProfileSummary.Boundaries[commands.BoundaryReadOnly] != 5 || inventory.CommandProfileSummary.Boundaries[commands.BoundaryCaseLocalApply] != 8 || inventory.CommandProfileSummary.Boundaries[commands.BoundaryCaseLocalReviewFirst] != 2 || inventory.CommandProfileSummary.Boundaries[commands.BoundaryKitReviewFirst] != 1 {
		t.Fatalf("Go-native public command profile summary drifted: %+v", inventory.CommandProfileSummary)
	}
	if strings.Join(inventory.CommandProfileGroups.ReadOnly, ",") != "doctor,packs,release-check,status,validate" || strings.Join(inventory.CommandProfileGroups.ReviewFirst, ",") != "promote,sync,update" || strings.Join(inventory.CommandProfileGroups.WritesKit, ",") != "promote" || surfaceCounts.Groups.HeavyTool != 0 || surfaceCounts.Groups.AuthorityConfirmed != 0 || surfaceCounts.Groups.CaseLocalApply != 8 || surfaceCounts.Groups.CaseLocalReviewFirst != 2 {
		t.Fatalf("Go-native public command profile groups drifted: %+v", inventory.CommandProfileGroups)
	}
	firstBoundaryCounts := GoNativePublicSurfaceBoundaryRowCountsFor(inventory.CommandProfileBoundaries[0])
	lastBoundaryCounts := GoNativePublicSurfaceBoundaryRowCountsFor(inventory.CommandProfileBoundaries[len(inventory.CommandProfileBoundaries)-1])
	if surfaceCounts.Boundaries.Rows != 7 || surfaceCounts.Boundaries.Commands != 19 || surfaceCounts.Boundaries.CountedCommands != 19 || inventory.CommandProfileBoundaries[0].Boundary != commands.BoundaryCaseLocalAppend || firstBoundaryCounts.Count != 1 || firstBoundaryCounts.Commands != 1 || strings.Join(inventory.CommandProfileBoundaries[1].Commands, ",") != "attach,bootstrap,continue,gate,handoff,init,repair,start" || inventory.CommandProfileBoundaries[len(inventory.CommandProfileBoundaries)-1].Boundary != commands.BoundaryReadOnly || lastBoundaryCounts.Count != 5 || lastBoundaryCounts.Commands != 5 {
		t.Fatalf("Go-native public command profile boundary rows drifted: %+v", inventory.CommandProfileBoundaries)
	}
	if surfaceCounts.Policies.Rows != 5 || surfaceCounts.Policies.Violations != 0 || surfaceCounts.Policies.ViolationCommands != 0 || inventory.CommandProfilePolicies[0].Policy != commands.PublicProfilePolicyNoHeavyTool || !inventory.CommandProfilePolicies[0].Ready || inventory.CommandProfilePolicies[3].Policy != commands.PublicProfilePolicyReviewFirstApplyRequired || GoNativePublicSurfacePolicyRowCountsFor(inventory.CommandProfilePolicies[3]).Commands != 0 {
		t.Fatalf("Go-native public command profile policy rows drifted: %+v", inventory.CommandProfilePolicies)
	}
	if !inventory.FacadeRemovalReady || surfaceCounts.FacadeRemoval.Rows != 5 || surfaceCounts.FacadeRemoval.NotReady != 0 || inventory.FacadeRemovalPrerequisites[0].Name != "entrypoint" || !inventory.FacadeRemovalPrerequisites[0].Ready || inventory.FacadeRemovalPrerequisites[4].Name != "unsupported-command-diagnostic" || !inventory.FacadeRemovalPrerequisites[4].Ready {
		t.Fatalf("Go-native public surface facade removal prerequisites drifted: ready=%t prerequisites=%+v", inventory.FacadeRemovalReady, inventory.FacadeRemovalPrerequisites)
	}
}

func TestGoNativePublicSurfaceInventoryDetectsDispatcherDrift(t *testing.T) {
	repo := t.TempDir()
	writeFile(t, filepath.Join(repo, "cmd", "rekit", "main.go"), "package main\n")
	writeFile(t, filepath.Join(repo, "internal", "rekit", "commands", "commands.go"), "package commands\n")
	writeFile(t, filepath.Join(repo, "internal", "rekit", "cli", "cli.go"), `package cli

import "github.com/shuiyu486/re-context-kits/internal/rekit/commands"

func dispatch(command string) error {
	switch command {
	case commands.Status:
		return nil
	default:
		// commands.UnsupportedError(opt.Command)
		return commands.UnsupportedError(command)
	}
}
`)

	inventory := goNativePublicSurface(repo)
	if inventory.Ready {
		t.Fatalf("Go-native public surface unexpectedly ready despite dispatcher drift: %+v", inventory)
	}
	assertWarningContains(t, inventory.Warnings, "Go CLI dispatcher missing public command handler: release-check")
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
	if counts.GoDefaultCommands != 19 || counts.NoFallbackCommands != 19 || counts.CandidateCommands != 0 || counts.RemovalCandidateModules != 0 || counts.RetiredModules != 13 {
		t.Fatalf("fallback retirement inventory omitted expected sections: %+v", fallback)
	}
	for _, command := range []string{"attach", "bootstrap", "continue", "doctor", "gate", "handoff", "init", "note", "overview", "packs", "plan-subagents", "promote", "release-check", "repair", "start", "status", "sync", "update", "validate"} {
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
	if counts.CommandSurface != 19 || counts.GoDefaultCommands != 19 || counts.NoFallbackCommands != 19 {
		t.Fatalf("PowerShell public facade inventory omitted expected command lists: %+v", facade)
	}
	for _, command := range []string{"attach", "bootstrap", "continue", "doctor", "gate", "handoff", "init", "note", "overview", "packs", "plan-subagents", "promote", "release-check", "repair", "start", "status", "sync", "update", "validate"} {
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
	if !inventory.RemovalImpact.Ready || inventory.RemovalImpact.FacadePath != "rekit/rekit.ps1" || !inventory.RemovalImpact.FacadePresent || impactCounts.Warnings != 0 || impactCounts.References == 0 || impactCounts.ReferenceCategories == 0 || impactCounts.WorkItems != impactCounts.ReferenceCategories || impactCounts.WorkItemValidationCommands != impactCounts.WorkItems*8 || impactCounts.MigrationTargets != 74 || impactCounts.MigrationValidationCommands != 592 || impactCounts.SmokeMigrationTargets != 29 || impactCounts.SmokeMigrationValidationCommands != 232 || impactCounts.UnclassifiedReferences != 0 || !publicFacadeRemovalHasImpactCategory(inventory.RemovalImpact, "public-facade-entrypoint") || !publicFacadeRemovalHasImpactCategory(inventory.RemovalImpact, "facade-compatibility-smoke") || !publicFacadeRemovalHasImpactWorkItem(inventory.RemovalImpact, "release-inventory-and-tests") || !publicFacadeRemovalHasMigrationTarget(inventory.RemovalImpact, "rekit/rekit.ps1") || !publicFacadeRemovalHasMigrationTarget(inventory.RemovalImpact, "docs/powershell-deprecation.md") || !publicFacadeRemovalHasSmokeMigrationTarget(inventory.RemovalImpact, "rekit/tests/facade-smoke.ps1") || !publicFacadeRemovalHasSmokeMigrationTarget(inventory.RemovalImpact, "rekit/tests/continue-whatif-smoke.ps1") {
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
