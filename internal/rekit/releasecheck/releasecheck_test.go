package releasecheck

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

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
	assertCommandOwner(t, inventory, "plan-subagents", false, false)
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
| plan-subagents review artifacts | Go manual path + PowerShell internal flow | internal/fallback | no default delegation. |
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
