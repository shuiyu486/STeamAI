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
	writeFile(t, filepath.Join(repo, "docs", "batch-plan.md"), "# Batch implementation plan\n\n### "+batchTitle+"\n\n状态：已完成。\n\n目标：fixture goal.\n\n验证结果：fixture validation.\n")
	writeFile(t, filepath.Join(repo, "docs", "release-readiness.md"), "## Known gaps\n\n- fixture gap\n")
	writeFile(t, filepath.Join(repo, "README.md"), "# README\n\n用户主要指挥主 Agent / Mission Commander\nGo CLI/backend 是背后的 canonical deterministic runtime/API\n`rekit.ps1` 仅作为迁移期 legacy façade\n默认路径继续向 PowerShell-free / Go-native / 跨平台收敛\n这里不需要你手动执行底层脚本\n用户不需要把 `/rekit` 子命令当成主要交互界面\n")
	writeFile(t, filepath.Join(repo, "CLAUDE.md"), "# CLAUDE\n\nPowerShell-free / Go-native / 跨平台收敛\nPowerShell replacement/removal 不再因“删除 PowerShell”本身停下询问\n默认远程 CI 入口是 `.github/workflows/release-gate.yml`，在 Linux、Windows、macOS 上运行 Go-native release checks\n")
	writeFile(t, filepath.Join(repo, "docs", "mission-control-product-direction.md"), "# mission control\n")
	writeFile(t, filepath.Join(repo, "docs", "autonomous-goal.md"), "# autonomous goal\n\nPowerShell-free / Go-native / 跨平台\n每轮自主推进按这个循环做\n默认继续自主推进\n")
	writeFile(t, filepath.Join(repo, "docs", "go-first-convergence-plan.md"), "# go first\n")
	writeFile(t, filepath.Join(repo, "docs", "powershell-deprecation.md"), "# powershell\n")
	writeFile(t, filepath.Join(repo, ".claude", "skills", "rekit", "SKILL.md"), "# rekit\n\n产品方向是 Mission Control\n底层 Go CLI 是 canonical runtime\n`rekit.ps1` 只是迁移期 legacy façade\ncase 只生成 `.claude/skills/rekit/SKILL.md` 薄 shim\n底层 runtime 只作为 `/rekit` 的内部实现\n")
	writeFile(t, filepath.Join(repo, "rekit", "templates", "case-shim", "SKILL.md"), "# shim\n\ncase-local 薄 shim\n不包含业务逻辑\ncanonical `/rekit`\n.rekit/instance.yml\n.re-template.yml\n<templateRoot>/.claude/skills/rekit/SKILL.md\ncanonical runtime\nsync` / `promote` 默认必须 review-first\n不要在本 shim 里维护模板规则\n不要读取或修改用户级 `~/.claude/skills`\n不要在 shim 中复制逻辑\n不展示底层脚本或 CLI 命令\nGo-native backend\n")
	writeFile(t, filepath.Join(repo, "CHANGELOG.md"), "# Changelog\n\n## Unreleased\n\n- "+changelogLine+".\n")
	writeFile(t, filepath.Join(repo, "packs", "fixture", "manifest.yml"), `id: fixture
name: Fixture
version: 0.0.0
maturity: skeleton
description: Fixture pack.
`)
}
