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
	if !result.Ready || len(result.Warnings) != 0 {
		t.Fatalf("release-check unexpectedly not ready: %+v", result)
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
	if !gate.Ready || gate.WorkflowPath != ".github/workflows/release-gate.yml" || gate.Summary != "CI release gate inventory ok" || len(gate.Warnings) != 0 {
		t.Fatalf("unexpected CI release gate inventory: %+v", gate)
	}
	if len(gate.WorkflowChecks) == 0 || len(gate.Jobs) != 3 || len(gate.RequiredCommands) != 18 || len(gate.ForbiddenStrings) == 0 {
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
	if !inventory.Ready || inventory.Summary != "Go-native public command surface inventory ok" || len(inventory.Warnings) != 0 {
		t.Fatalf("unexpected Go-native public surface inventory: %+v", inventory)
	}
	if inventory.Entrypoint != "cmd/rekit" || !inventory.EntrypointPresent || inventory.CommandCatalogPath != "internal/rekit/commands/commands.go" || !inventory.CommandCatalogPresent || inventory.DefaultCommand != "status" || inventory.AlternativePattern != "go run ./cmd/rekit -- -Command <command>" || !inventory.UnsupportedCommandDiagnosticPresent {
		t.Fatalf("unexpected Go-native public surface flags: %+v", inventory)
	}
	if len(inventory.Commands) != 19 || len(inventory.HandlerCommands) != 19 || len(inventory.SymbolCommands) != 19 || len(inventory.CommandProfiles) != 19 || len(inventory.MutationBoundaries) != 7 {
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
	if inventory.CommandProfileSummary.Total != 19 || inventory.CommandProfileSummary.ReadOnly != 5 || inventory.CommandProfileSummary.Mutating != 14 || inventory.CommandProfileSummary.WritesCase != 13 || inventory.CommandProfileSummary.WritesKit != 1 || inventory.CommandProfileSummary.ReviewFirst != 3 || inventory.CommandProfileSummary.ApplyRequired != 11 || inventory.CommandProfileSummary.HeavyTool != 0 || inventory.CommandProfileSummary.AuthorityConfirmed != 0 || inventory.CommandProfileSummary.Boundaries[commands.BoundaryReadOnly] != 5 || inventory.CommandProfileSummary.Boundaries[commands.BoundaryCaseLocalApply] != 8 || inventory.CommandProfileSummary.Boundaries[commands.BoundaryCaseLocalReviewFirst] != 2 || inventory.CommandProfileSummary.Boundaries[commands.BoundaryKitReviewFirst] != 1 {
		t.Fatalf("Go-native public command profile summary drifted: %+v", inventory.CommandProfileSummary)
	}
	if strings.Join(inventory.CommandProfileGroups.ReadOnly, ",") != "doctor,packs,release-check,status,validate" || strings.Join(inventory.CommandProfileGroups.ReviewFirst, ",") != "promote,sync,update" || strings.Join(inventory.CommandProfileGroups.WritesKit, ",") != "promote" || len(inventory.CommandProfileGroups.HeavyTool) != 0 || len(inventory.CommandProfileGroups.AuthorityConfirmed) != 0 || len(inventory.CommandProfileGroups.ByBoundary[commands.BoundaryCaseLocalApply]) != 8 || len(inventory.CommandProfileGroups.ByBoundary[commands.BoundaryCaseLocalReviewFirst]) != 2 {
		t.Fatalf("Go-native public command profile groups drifted: %+v", inventory.CommandProfileGroups)
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
	if !inventory.Ready || inventory.StrategyDocument != "docs/powershell-deprecation.md" || len(inventory.Warnings) != 0 {
		t.Fatalf("unexpected PowerShell deprecation inventory: %+v", inventory)
	}
	if len(inventory.CommandOwnership) == 0 || len(inventory.ModuleStatus) == 0 || len(inventory.FreezeGates) == 0 || len(inventory.BlockedMigrations) == 0 {
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
	if !fallback.Ready || fallback.Summary != "PowerShell fallback retirement inventory ok" || len(fallback.Warnings) != 0 {
		t.Fatalf("unexpected fallback retirement inventory: %+v", fallback)
	}
	if len(fallback.GoDefaultCommands) != 19 || len(fallback.NoFallbackCommands) != 19 || len(fallback.CandidateCommands) != 0 || len(fallback.RemovalCandidateModules) != 0 || len(fallback.RetiredModules) != 13 {
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
	if !facade.Ready || facade.Summary != "PowerShell facade runtime dependency inventory ok" || facade.FacadePath != "rekit/rekit.ps1" || len(facade.Warnings) != 0 {
		t.Fatalf("unexpected PowerShell facade runtime inventory: %+v", facade)
	}
	if facade.LegacyModuleImportsPresent || facade.CommandDispatcherPresent || !facade.NoFallbackGuardPresent || !facade.GoDelegationPresent || !facade.RetiredDispatcherError {
		t.Fatalf("unexpected PowerShell facade runtime dependency flags: %+v", facade)
	}
	if len(facade.ForbiddenPatterns) == 0 || len(facade.RequiredPatterns) == 0 {
		t.Fatalf("PowerShell facade runtime inventory omitted required pattern lists: %+v", facade)
	}
}

func assertPublicFacade(t *testing.T, inventory PowerShellDeprecation) {
	t.Helper()
	facade := inventory.PublicFacade
	if !facade.Ready || facade.Summary != "PowerShell public facade retention inventory ok" || facade.FacadePath != "rekit/rekit.ps1" || len(facade.Warnings) != 0 {
		t.Fatalf("unexpected PowerShell public facade inventory: %+v", facade)
	}
	if !facade.Present || !facade.Retained || !facade.MigrationBoundaryDocumented || !facade.RemovalBoundaryDocumented || facade.GoNativeAlternative != "go run ./cmd/rekit -- -Command <command>" {
		t.Fatalf("unexpected PowerShell public facade retention flags: %+v", facade)
	}
	if len(facade.CommandSurface) != 19 || len(facade.GoDefaultCommands) != 19 || len(facade.NoFallbackCommands) != 19 {
		t.Fatalf("PowerShell public facade inventory omitted expected command lists: %+v", facade)
	}
	for _, command := range []string{"attach", "bootstrap", "continue", "doctor", "gate", "handoff", "init", "note", "overview", "packs", "plan-subagents", "promote", "release-check", "repair", "start", "status", "sync", "update", "validate"} {
		if !slices.Contains(facade.CommandSurface, command) || !slices.Contains(facade.GoDefaultCommands, command) || !slices.Contains(facade.NoFallbackCommands, command) {
			t.Fatalf("public facade command %s missing from command lists: %+v", command, facade)
		}
	}
}

func assertModuleRemoval(t *testing.T, inventory PowerShellDeprecation) {
	t.Helper()
	removal := inventory.ModuleRemoval
	if !removal.Ready || removal.Summary != "PowerShell module removal inventory ok" || len(removal.Warnings) != 0 {
		t.Fatalf("unexpected PowerShell module removal inventory: %+v", removal)
	}
	if len(removal.CandidateModules) != 0 || len(removal.RetiredModules) != 13 || len(removal.FacadeRuntimeDependencies) != 0 || len(removal.UndocumentedModules) != 0 {
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
	if !refs.Ready || refs.Summary != "PowerShell module reference inventory ok" || len(refs.Warnings) != 0 {
		t.Fatalf("unexpected PowerShell module reference inventory: %+v", refs)
	}
	if refs.TotalReferences == 0 || len(refs.ActiveTestDependencies) != 0 || len(refs.CompatibilityFixtures) != 0 || len(refs.InventoryGuards) == 0 || len(refs.RemovalBlockers) != 0 || len(refs.UnclassifiedReferences) != 0 {
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
