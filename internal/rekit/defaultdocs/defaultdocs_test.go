package defaultdocs

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestInspectRepoPublicDefaultDocsReady(t *testing.T) {
	readiness := Inspect(repoRoot(t))
	counts := ReadinessCountsFor(readiness)
	if !readiness.Ready || readiness.Summary != "public default docs readiness ok" || counts.Warnings != 0 {
		t.Fatalf("unexpected public default docs readiness: %+v", readiness)
	}
	assertDocument(t, readiness, "README.md")
	assertDocument(t, readiness, ".claude/skills/rekit/SKILL.md")
	assertDocument(t, readiness, "CLAUDE.md")
	assertDocument(t, readiness, "docs/context-routing.md")
	assertDocument(t, readiness, "docs/batch-plan.md")
	assertDocument(t, readiness, "docs/mission-control-product-direction.md")
	assertDocument(t, readiness, "docs/autonomous-goal.md")
	assertDocument(t, readiness, "docs/release-readiness.md")
	assertDocument(t, readiness, "docs/go-first-convergence-plan.md")
	assertDocument(t, readiness, "docs/go-runtime-migration.md")
	assertDocument(t, readiness, "docs/powershell-deprecation.md")
	assertDocument(t, readiness, "docs/vision.md")
	assertDocument(t, readiness, "docs/reference-absorption.md")
	assertDocument(t, readiness, "docs/agent-team-rollout-plan.md")
	assertDocument(t, readiness, "rekit/tests/README.md")
	assertPhrase(t, readiness, "README.md", "用户主要指挥主 Agent / Mission Commander")
	assertPhrase(t, readiness, "docs/mission-control-product-direction.md", "Lane-centric Agent Team Mission Control")
	assertPhrase(t, readiness, ".claude/skills/rekit/SKILL.md", "底层 Go CLI 是 canonical runtime")
	assertPhrase(t, readiness, "docs/context-routing.md", "按需路由")
	assertPhrase(t, readiness, "docs/context-routing.md", "不要默认读取 `docs/batch-history.md` 全文")
	assertPhrase(t, readiness, "docs/batch-plan.md", "完整历史已拆到 `docs/batch-history.md`")
	assertPhrase(t, readiness, "CLAUDE.md", "PowerShell-free default/product path、Go-native、跨平台")
	assertPhrase(t, readiness, "docs/autonomous-goal.md", "默认继续自主推进")
	assertPhrase(t, readiness, "docs/release-readiness.md", "默认本机验证路径不依赖 PowerShell")
	assertPhrase(t, readiness, "docs/go-first-convergence-plan.md", "不要把大型 PowerShell matrix 作为默认必跑")
	assertPhrase(t, readiness, "docs/go-runtime-migration.md", "当前默认验证应优先运行 Go-native release gate")
	assertPhrase(t, readiness, "docs/powershell-deprecation.md", "Go CLI/backend 是 canonical runtime")
	assertPhrase(t, readiness, "docs/vision.md", "优先运行 Go-native 检查")
	assertPhrase(t, readiness, "docs/reference-absorption.md", "Go-native release readiness 子集")
	assertPhrase(t, readiness, "docs/agent-team-rollout-plan.md", "公共 `/rekit` 默认路径已由 23 个 Go-owned/no-fallback public commands 支撑")
	assertPhrase(t, readiness, "rekit/tests/README.md", "推荐最小回归组合")
	if counts.ForbiddenCommands != 0 {
		t.Fatalf("unexpected forbidden public default commands: %+v", readiness.ForbiddenCommands)
	}
	if counts.ForbiddenShellFences != 0 {
		t.Fatalf("unexpected forbidden public default shell fences: %+v", readiness.ForbiddenShellFences)
	}
}

func TestInspectDetectsPowerShellFacadeCommandSnippet(t *testing.T) {
	repo := t.TempDir()
	writeReadyDocs(t, repo)
	writeFile(t, filepath.Join(repo, "README.md"), readyREADME+"\n```text\n./rekit/rekit.ps1 -Command doctor\n```\n")

	readiness := Inspect(repo)
	if readiness.Ready {
		t.Fatalf("public default docs unexpectedly ready despite PowerShell command snippet: %+v", readiness)
	}
	if len(readiness.ForbiddenCommands) != 1 || !strings.Contains(readiness.ForbiddenCommands[0].Snippet, "rekit.ps1") {
		t.Fatalf("unexpected forbidden command diagnostics: %+v", readiness.ForbiddenCommands)
	}
	assertWarningContains(t, readiness.Warnings, "exposes PowerShell façade command")
}

func TestInspectDetectsPowerShellShellFence(t *testing.T) {
	repo := t.TempDir()
	writeReadyDocs(t, repo)
	writeFile(t, filepath.Join(repo, "docs", "release-readiness.md"), readyReleaseReadiness+"\n```powershell\ngo test ./...\n```\n")

	readiness := Inspect(repo)
	if readiness.Ready {
		t.Fatalf("public default docs unexpectedly ready despite PowerShell shell fence: %+v", readiness)
	}
	if len(readiness.ForbiddenShellFences) != 1 || readiness.ForbiddenShellFences[0].Language != "powershell" {
		t.Fatalf("unexpected forbidden shell fence diagnostics: %+v", readiness.ForbiddenShellFences)
	}
	assertWarningContains(t, readiness.Warnings, "uses PowerShell shell fence")
}

func TestInspectDetectsMissingMissionControlDefaultPhrase(t *testing.T) {
	repo := t.TempDir()
	writeReadyDocs(t, repo)
	writeFile(t, filepath.Join(repo, "README.md"), "# README\n\nmissing public defaults\n")

	readiness := Inspect(repo)
	if readiness.Ready {
		t.Fatalf("public default docs unexpectedly ready despite missing phrases: %+v", readiness)
	}
	assertWarningContains(t, readiness.Warnings, "README.md missing required phrase")
}

func assertDocument(t *testing.T, readiness Readiness, path string) {
	t.Helper()
	for _, doc := range readiness.Documents {
		if doc.Path == path {
			if !doc.Present || strings.TrimSpace(doc.Purpose) == "" {
				t.Fatalf("document %s = %+v, want present with purpose", path, doc)
			}
			return
		}
	}
	t.Fatalf("missing document %s: %+v", path, readiness.Documents)
}

func assertPhrase(t *testing.T, readiness Readiness, path, phrase string) {
	t.Helper()
	for _, check := range readiness.RequiredPhrases {
		if check.Path == path && check.Phrase == phrase {
			if !check.Present {
				t.Fatalf("phrase %s/%q present=false", path, phrase)
			}
			return
		}
	}
	t.Fatalf("missing phrase check %s/%q: %+v", path, phrase, readiness.RequiredPhrases)
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

func writeReadyDocs(t *testing.T, repo string) {
	t.Helper()
	writeFile(t, filepath.Join(repo, "README.md"), readyREADME)
	writeFile(t, filepath.Join(repo, ".claude", "skills", "rekit", "SKILL.md"), readySkill)
	writeFile(t, filepath.Join(repo, "CLAUDE.md"), readyClaude)
	writeFile(t, filepath.Join(repo, "docs", "context-routing.md"), readyContextRouting)
	writeFile(t, filepath.Join(repo, "docs", "batch-plan.md"), readyBatchPlan)
	writeFile(t, filepath.Join(repo, "docs", "mission-control-product-direction.md"), readyMissionControlProductDirection)
	writeFile(t, filepath.Join(repo, "docs", "autonomous-goal.md"), readyAutonomousGoal)
	writeFile(t, filepath.Join(repo, "docs", "release-readiness.md"), readyReleaseReadiness)
	writeFile(t, filepath.Join(repo, "docs", "go-first-convergence-plan.md"), readyGoFirstConvergence)
	writeFile(t, filepath.Join(repo, "docs", "go-runtime-migration.md"), readyGoRuntimeMigration)
	writeFile(t, filepath.Join(repo, "docs", "powershell-deprecation.md"), readyPowerShellDeprecation)
	writeFile(t, filepath.Join(repo, "docs", "vision.md"), readyVision)
	writeFile(t, filepath.Join(repo, "docs", "reference-absorption.md"), readyReferenceAbsorption)
	writeFile(t, filepath.Join(repo, "docs", "agent-team-rollout-plan.md"), readyAgentTeamRollout)
	writeFile(t, filepath.Join(repo, "rekit", "tests", "README.md"), readyTestsReadme)
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

const readyREADME = `# README

用户主要指挥主 Agent / Mission Commander。
Go CLI/backend 是背后的 canonical deterministic runtime/API。
` + "`rekit.ps1` 仅作为 retained compatibility façade。" + `
默认路径继续向 PowerShell-free / Go-native / 跨平台收敛。
这里不需要你手动执行底层脚本。
用户不需要把 ` + "`/rekit`" + ` 子命令当成主要交互界面。
`

const readySkill = `# skill

产品方向是 Mission Control。
底层 Go CLI 是 canonical runtime。
` + "`rekit.ps1` 只是 retained compatibility façade。" + `
底层 runtime 只作为 ` + "`/rekit`" + ` 的内部实现。
`

const readyClaude = `# CLAUDE

PowerShell-free default/product path、Go-native、跨平台。
PowerShell replacement/removal 不再因“删除 PowerShell”本身停下询问。
默认远程 CI workflow 是 ` + "`.github/workflows/release-gate.yml`" + `，定义 Linux、Windows、macOS Go-native release checks。
`

const readyContextRouting = `# context routing

按需路由。
渐进式披露。
不要默认读取 ` + "`docs/batch-history.md`" + ` 全文。
`

const readyBatchPlan = `# batch plan

完整历史已拆到 ` + "`docs/batch-history.md`" + `。
`

const readyMissionControlProductDirection = `# mission

Lane-centric Agent Team Mission Control。
用户主要和一个 **主 Agent / Mission Commander** 会话交互。
Go-first deterministic substrate。
`

const readyAutonomousGoal = `# goal

PowerShell-free / Go-native / 跨平台。
每轮自主推进按这个循环做。
默认继续自主推进。
`

const readyReleaseReadiness = `# release

发布门禁默认依赖 Go-owned ` + "`release-check`" + ` inventory。
默认本机验证路径不依赖 PowerShell。
`

const readyGoFirstConvergence = `# go first

Go backend 成为 rekit 的 deterministic runtime owner。
不要把大型 PowerShell matrix 作为默认必跑。
PowerShell-free / Go-native convergence。
`

const readyGoRuntimeMigration = `# runtime migration

当前默认验证应优先运行 Go-native release gate。
` + "`/rekit`" + ` remains the public ABI while the default implementation converges to the Go deterministic backend。
`

const readyPowerShellDeprecation = `# powershell

PowerShell-free default/product path / Go-native / 跨平台 convergence。
Go CLI/backend 是 canonical runtime。
PowerShell 当前只保留 ` + "`rekit/rekit.ps1`" + ` compatibility façade 与按需 parity residue。
`

const readyVision = `# vision

Claude Code Agent Team Mission Control 框架。
优先运行 Go-native 检查。
`

const readyReferenceAbsorption = `# reference

Go-native release readiness 子集。
不能宣称已具备自动脱壳/逆向引擎。
`

const readyAgentTeamRollout = `# rollout

公共 ` + "`/rekit`" + ` 默认路径已由 23 个 Go-owned/no-fallback public commands 支撑。
Go-native ` + "`status`" + `、` + "`doctor`" + ` 与 ` + "`release-check`" + ` 不回归。
`

const readyTestsReadme = `# tests

Go-first release gate 优先由 Go-owned ` + "`release-check`" + ` inventory。
推荐最小回归组合。
`
