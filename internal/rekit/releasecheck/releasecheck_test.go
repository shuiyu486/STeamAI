package releasecheck

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
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
