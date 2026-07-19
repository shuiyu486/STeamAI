package releasecheck

import (
	"path/filepath"
	"testing"
)

func TestReleaseHandoffDetectsStaleReleaseNotes(t *testing.T) {
	repo := t.TempDir()
	writeReleaseHandoffFixture(t, repo, "Batch 999：Fixture", "Batch 998 old note")
	result, err := Build(repo)
	if err != nil {
		t.Fatal(err)
	}
	if result.ReleaseHandoff.ReleaseNotes.Covered || result.ReleaseHandoff.Ready || result.Ready {
		t.Fatalf("release handoff unexpectedly ready despite stale release notes: %+v", result.ReleaseHandoff)
	}
	assertWarningContains(t, result.ReleaseHandoff.Warnings, "release notes missing latest batch: Batch 999")
	assertWarningContains(t, result.Warnings, "release notes missing latest batch: Batch 999")
}

func TestBatchIDFromTitle(t *testing.T) {
	cases := map[string]string{
		"Batch 147：Release notes freshness gate":  "Batch 147",
		"Batch 147: Release notes freshness gate": "Batch 147",
		"Batch 147 Release notes freshness gate":  "Batch 147",
		"not a batch":                             "",
	}
	for input, want := range cases {
		if got := batchIDFromTitle(input); got != want {
			t.Fatalf("batchIDFromTitle(%q) = %q, want %q", input, got, want)
		}
	}
}

func writeReleaseHandoffFixture(t *testing.T, repo, batchTitle, changelogLine string) {
	t.Helper()
	writeFile(t, filepath.Join(repo, "rekit", "tests", "catalog.json"), `{
  "recommendedMinimum": ["go run ./cmd/rekit -- -Command release-check -Format json"],
  "globalBoundaries": ["boundary"]
}`)
	writeFile(t, filepath.Join(repo, "docs", "context-routing.md"), "# Context routing\n\n按需路由。\n渐进式披露。\n不要默认读取 `docs/batch-history.md` 全文。\n")
	writeFile(t, filepath.Join(repo, "docs", "batch-plan.md"), "# Batch implementation plan\n\n完整历史已拆到 `docs/batch-history.md`。\n\n### "+batchTitle+"\n\n状态：已完成。\n\n目标：fixture goal.\n\n验证结果：fixture validation.\n")
	writeFile(t, filepath.Join(repo, "docs", "release-readiness.md"), "# Release readiness\n\n发布门禁默认依赖 Go-owned `release-check` inventory。\n默认本机验证路径不依赖 PowerShell。\n\n## Known gaps\n\n- fixture gap\n")
	writeFile(t, filepath.Join(repo, "README.md"), "# README\n\n用户主要指挥主 Agent / Mission Commander\nGo CLI/backend 是背后的 canonical deterministic runtime/API\n`rekit.ps1` 仅作为 retained compatibility façade\n默认路径继续向 PowerShell-free / Go-native / 跨平台收敛\n这里不需要你手动执行底层脚本\n用户不需要把 `/rekit` 子命令当成主要交互界面\n")
	writeFile(t, filepath.Join(repo, "CLAUDE.md"), "# CLAUDE\n\nPowerShell-free default/product path、Go-native、跨平台\nPowerShell replacement/removal 不再因“删除 PowerShell”本身停下询问\n")
	writeFile(t, filepath.Join(repo, "docs", "mission-control-product-direction.md"), "# mission control\n\nLane-centric Agent Team Mission Control\n用户主要和一个 **主 Agent / Mission Commander** 会话交互\nGo-first deterministic substrate\n")
	writeFile(t, filepath.Join(repo, "docs", "autonomous-goal.md"), "# autonomous goal\n\nPowerShell-free / Go-native / 跨平台\n每轮自主推进按这个循环做\n默认继续自主推进\n")
	writeFile(t, filepath.Join(repo, "docs", "go-first-convergence-plan.md"), "# go first\n\nGo backend 成为 rekit 的 deterministic runtime owner\n不要把大型 PowerShell matrix 作为默认必跑\nPowerShell-free / Go-native convergence\n")
	writeFile(t, filepath.Join(repo, "docs", "go-runtime-migration.md"), "# runtime migration\n\n当前默认验证应优先运行 Go-native release gate\n`/rekit` remains the public ABI while the default implementation converges to the Go deterministic backend\n")
	writeFile(t, filepath.Join(repo, "docs", "powershell-deprecation.md"), "# powershell\n\nPowerShell-free default/product path / Go-native / 跨平台 convergence\nGo CLI/backend 是 canonical runtime\nPowerShell 当前只保留 `rekit/rekit.ps1` compatibility façade 与按需 parity residue\n")
	writeFile(t, filepath.Join(repo, "docs", "vision.md"), "# vision\n\nClaude Code Agent Team Mission Control 框架\n优先运行 Go-native 检查\n")
	writeFile(t, filepath.Join(repo, "docs", "reference-absorption.md"), "# reference\n\nGo-native release readiness 子集\n不能宣称已具备自动脱壳/逆向引擎\n")
	writeFile(t, filepath.Join(repo, "docs", "agent-team-rollout-plan.md"), "# rollout\n\n公共 `/rekit` 默认路径已由 20 个 Go-owned/no-fallback public commands 支撑\nGo-native `status`、`doctor` 与 `release-check` 不回归\n")
	writeFile(t, filepath.Join(repo, "rekit", "tests", "README.md"), "# tests\n\nGo-first release gate 优先由 Go-owned `release-check` inventory\n推荐最小回归组合\n")
	writeFile(t, filepath.Join(repo, ".claude", "skills", "rekit", "SKILL.md"), "# rekit\n\n产品方向是 Mission Control\n底层 Go CLI 是 canonical runtime\n`rekit.ps1` 只是 retained compatibility façade\ncase 只生成 `.claude/skills/rekit/SKILL.md` 薄 shim\n底层 runtime 只作为 `/rekit` 的内部实现\n")
	writeFile(t, filepath.Join(repo, "rekit", "templates", "case-shim", "SKILL.md"), "# shim\n\ncase-local 薄 shim\n不包含业务逻辑\ncanonical `/rekit`\n.rekit/instance.yml\n.re-template.yml\n<templateRoot>/.claude/skills/rekit/SKILL.md\ncanonical runtime\nsync` / `promote` 默认必须 review-first\n不要在本 shim 里维护模板规则\n不要读取或修改用户级 `~/.claude/skills`\n不要在 shim 中复制逻辑\n不展示底层脚本或 CLI 命令\nGo-native backend\n")
	writeFile(t, filepath.Join(repo, "CHANGELOG.md"), "# Changelog\n\n## Unreleased\n\n- "+changelogLine+".\n")
	writeFile(t, filepath.Join(repo, "packs", "fixture", "manifest.yml"), `id: fixture
name: Fixture
version: 0.0.0
maturity: skeleton
description: Fixture pack.
`)
}
