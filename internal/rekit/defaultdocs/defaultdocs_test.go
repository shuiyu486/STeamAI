package defaultdocs

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/shuiyu486/re-context-kits/internal/rekit/skillcontract"
)

func TestInspectRepoPublicDefaultDocsReady(t *testing.T) {
	readiness := Inspect(repoRoot(t))
	counts := ReadinessCountsFor(readiness)
	if !readiness.Ready || readiness.Summary != "public default docs readiness ok" || counts.Warnings != 0 {
		t.Fatalf("unexpected current STeamAI public default docs readiness: %+v", readiness)
	}
	if readiness.Model != "steamai-self-contained-current" || readiness.DefaultEntrypoint != "/steamai" || readiness.StateRoot != ".steamai" || readiness.RuntimeSource != "project-local-verified-bundle" || readiness.FallbackAllowed || readiness.CanonicalRepository != canonicalRepository || readiness.CanonicalCloneURL != canonicalCloneURL || readiness.ModuleCompatibilityIdentity != moduleCompatibilityIdentity {
		t.Fatalf("unexpected current STeamAI public defaults: %+v", readiness)
	}
	assertDocument(t, readiness, "README.md")
	assertDocument(t, readiness, ".claude/skills/steamai/SKILL.md")
	assertDocument(t, readiness, "rekit/templates/steamai-project/SKILL.md")
	assertDocument(t, readiness, "CLAUDE.md")
	assertDocument(t, readiness, "docs/context-routing.md")
	assertDocument(t, readiness, "docs/real-usage-hardening-roadmap.md")
	assertDocument(t, readiness, "docs/batch-plan.md")
	assertDocument(t, readiness, "docs/mission-control-product-direction.md")
	assertDocument(t, readiness, "docs/steamai-self-contained-project.md")
	assertDocument(t, readiness, "docs/autonomous-goal.md")
	assertDocument(t, readiness, "docs/reference-absorption.md")
	assertDocument(t, readiness, "docs/release-readiness.md")
	assertDocument(t, readiness, "docs/promote-sync.md")
	assertDocument(t, readiness, "docs/powershell-deprecation.md")
	assertDocument(t, readiness, "rekit/tests/README.md")
	assertPhrase(t, readiness, "README.md", canonicalRepository)
	assertPhrase(t, readiness, "README.md", canonicalCloneURL)
	assertPhrase(t, readiness, "README.md", moduleCompatibilityIdentity)
	assertPhrase(t, readiness, "README.md", "旧 `/rekit`、`.rekit` 和中央 kit/thin-shim 模型只在迁移期间兼容，不是新项目默认")
	assertPhrase(t, readiness, "README.md", "暂停、恢复或停止同样只需自然语言")
	assertPhrase(t, readiness, ".claude/skills/steamai/SKILL.md", "新项目唯一可变状态根是 `${CLAUDE_PROJECT_DIR}/.steamai`")
	assertPhrase(t, readiness, ".claude/skills/steamai/SKILL.md", "`control` 始终 review-first")
	assertPhrase(t, readiness, ".claude/skills/steamai/SKILL.md", "不通过 PATH、全局 plugin、项目内 Go source 或外部 kit 回退")
	assertPhrase(t, readiness, "rekit/templates/steamai-project/SKILL.md", "新项目唯一可变状态根是 `${CLAUDE_PROJECT_DIR}/.steamai`")
	assertPhrase(t, readiness, "rekit/templates/steamai-project/SKILL.md", "不通过 PATH、全局 plugin、项目内 Go source 或外部 kit 回退")
	assertPhrase(t, readiness, "rekit/templates/steamai-project/SKILL.md", "`control` 始终 review-first")
	assertPhrase(t, readiness, "CLAUDE.md", "`/steamai` canonical skill")
	assertPhrase(t, readiness, "CLAUDE.md", "legacy `/rekit` compatibility skill")
	assertPhrase(t, readiness, "CLAUDE.md", "case public JSON 的 project-local typed command 由 resolved state root 统一投影")
	assertPhrase(t, readiness, "CLAUDE.md", "当前已批准路线是 `steamai-product-optimization-v1`")
	assertPhrase(t, readiness, "CLAUDE.md", "不做 installer")
	assertPhrase(t, readiness, "CLAUDE.md", "exact lane `control` 使用独立 append-only generation")
	assertPhrase(t, readiness, "docs/context-routing.md", "STeamAI 自包含项目 / `.steamai` / `/steamai` / runtime bundle / legacy 迁移")
	assertPhrase(t, readiness, "docs/context-routing.md", "durable pause/resume/stop 与 late-result isolation")
	assertPhrase(t, readiness, "docs/context-routing.md", "GitHub repository identity / clone / rename / Go module compatibility")
	assertPhrase(t, readiness, "docs/real-usage-hardening-roadmap.md", "当前路线是 `steamai-product-optimization-v1`")
	assertPhrase(t, readiness, "docs/real-usage-hardening-roadmap.md", "source-clone-first")
	assertPhrase(t, readiness, "docs/real-usage-hardening-roadmap.md", "不实现 installer")
	assertPhrase(t, readiness, "docs/real-usage-hardening-roadmap.md", "拒绝 PATH/外部 kit fallback")
	assertPhrase(t, readiness, "docs/batch-plan.md", "当前路线是 `steamai-product-optimization-v1`")
	assertPhrase(t, readiness, "docs/mission-control-product-direction.md", "新项目的用户入口是 `/steamai`")
	assertPhrase(t, readiness, "docs/mission-control-product-direction.md", "唯一 current 状态根是 `.steamai`")
	assertPhrase(t, readiness, "docs/steamai-self-contained-project.md", "一个真实项目目录 = 一个自包含 STeamAI 项目")
	assertPhrase(t, readiness, "docs/steamai-self-contained-project.md", "旧 `/rekit` 与 `.rekit` 在迁移期间只作为兼容入口")
	assertPhrase(t, readiness, "docs/steamai-self-contained-project.md", "case public JSON 按 resolved state root 投影全部 project-local typed command")
	assertPhrase(t, readiness, "docs/steamai-self-contained-project.md", "## 7. Durable execution control")
	assertPhrase(t, readiness, "docs/reference-absorption.md", "git clone "+canonicalCloneURL)
	assertPhrase(t, readiness, "docs/release-readiness.md", canonicalRepository)
	assertPhrase(t, readiness, "docs/release-readiness.md", canonicalCloneURL)
	assertPhrase(t, readiness, "docs/release-readiness.md", moduleCompatibilityIdentity)
	assertPhrase(t, readiness, "docs/release-readiness.md", "current STeamAI entry readiness")
	assertPhrase(t, readiness, "docs/release-readiness.md", "legacy `/rekit` / `.rekit` compatibility readiness")
	assertPhrase(t, readiness, "docs/release-readiness.md", "`docs/promote-sync.md` 纳入 current guidance inventory")
	assertPhrase(t, readiness, "docs/release-readiness.md", "Public `control` 是Go-owned、case-local、review-first mutation")
	assertPhrase(t, readiness, "docs/promote-sync.md", "current 项目使用 `/steamai` 或自然语言")
	assertPhrase(t, readiness, "docs/promote-sync.md", "legacy-only 项目才使用 `/rekit`")
	assertPhrase(t, readiness, "docs/promote-sync.md", "<active-state-root>")
	if counts.GuidanceConflicts != 0 {
		t.Fatalf("unexpected current/legacy guidance conflicts: %+v", readiness.GuidanceConflicts)
	}
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

func TestInspectDetectsLegacyGitHubRepositoryCloneReference(t *testing.T) {
	for _, reference := range []string{
		legacyRepositoryURL,
		legacyRepositoryURL + ".git",
		"git@github.com:shuiyu486/re-context-kits.git",
		"ssh://git@github.com/shuiyu486/re-context-kits.git",
		"git://github.com/shuiyu486/re-context-kits.git",
		"git clone shuiyu486/re-context-kits",
		"gh repo clone shuiyu486/re-context-kits",
	} {
		t.Run(reference, func(t *testing.T) {
			repo := t.TempDir()
			writeReadyDocs(t, repo)
			writeFile(t, filepath.Join(repo, "docs", "reference-absorption.md"), readyReferenceAbsorption+"\nlegacy: "+reference+"\n")

			readiness := Inspect(repo)
			if readiness.Ready {
				t.Fatalf("public default docs unexpectedly ready despite legacy GitHub repository clone reference %q: %+v", reference, readiness)
			}
			assertWarningContains(t, readiness.Warnings, "still uses a legacy GitHub repository clone reference")
		})
	}
}

func TestInspectRequiresCanonicalRepositorySeparateFromCloneURL(t *testing.T) {
	repo := t.TempDir()
	writeReadyDocs(t, repo)
	withoutRepository := strings.Replace(readyREADME, "Canonical repository: "+canonicalRepository+"；", "Canonical repository omitted；", 1)
	writeFile(t, filepath.Join(repo, "README.md"), withoutRepository)

	readiness := Inspect(repo)
	if readiness.Ready {
		t.Fatalf("public default docs unexpectedly ready when clone URL only satisfied repository identity: %+v", readiness)
	}
	assertWarningContains(t, readiness.Warnings, "README.md missing required phrase: "+canonicalRepository)
}

func TestInspectDetectsCurrentLegacyGuidanceConflict(t *testing.T) {
	for _, test := range []struct {
		name    string
		path    string
		content string
	}{
		{
			name:    "standalone legacy promote command",
			path:    "docs/promote-sync.md",
			content: readyPromoteSync + "\n## 默认日常流程\n\n/rekit promote\n",
		},
		{
			name:    "fixed legacy observation root",
			path:    "docs/release-readiness.md",
			content: readyReleaseReadiness + "\ncurrent observation: `.rekit/facts/observations.jsonl`\n",
		},
	} {
		t.Run(test.name, func(t *testing.T) {
			repo := t.TempDir()
			writeReadyDocs(t, repo)
			writeFile(t, filepath.Join(repo, filepath.FromSlash(test.path)), test.content)

			readiness := Inspect(repo)
			if readiness.Ready || len(readiness.GuidanceConflicts) != 1 {
				t.Fatalf("current/legacy guidance conflict was not detected: %+v", readiness)
			}
			assertWarningContains(t, readiness.Warnings, "contains current/legacy guidance conflict")
		})
	}
}

func TestInspectAllowsExplicitLegacyGuidanceSection(t *testing.T) {
	repo := t.TempDir()
	writeReadyDocs(t, repo)

	readiness := Inspect(repo)
	if !readiness.Ready || len(readiness.GuidanceConflicts) != 0 {
		t.Fatalf("explicit legacy-only guidance should remain allowed: %+v", readiness)
	}
}

func TestInspectDetectsMissingCurrentSTeamAIDefaultPhrase(t *testing.T) {
	repo := t.TempDir()
	writeReadyDocs(t, repo)
	writeFile(t, filepath.Join(repo, ".claude", "skills", "steamai", "SKILL.md"), "# current skill\n\nmissing project-local no-fallback boundary\n")

	readiness := Inspect(repo)
	if readiness.Ready {
		t.Fatalf("current STeamAI public default docs unexpectedly ready despite missing phrases: %+v", readiness)
	}
	assertWarningContains(t, readiness.Warnings, ".claude/skills/steamai/SKILL.md missing required phrase")
}

func TestInspectDetectsCanonicalSTeamAISkillTemplateDrift(t *testing.T) {
	repo := t.TempDir()
	writeReadyDocs(t, repo)
	templatePath := filepath.Join(repo, "rekit", "templates", "steamai-project", "SKILL.md")
	template, err := os.ReadFile(templatePath)
	if err != nil {
		t.Fatal(err)
	}
	writeFile(t, templatePath, string(template)+"\ntemplate-only drift\n")

	readiness := Inspect(repo)
	if readiness.Ready {
		t.Fatalf("public default docs unexpectedly ready despite /steamai skill drift: %+v", readiness)
	}
	assertWarningContains(
		t,
		readiness.Warnings,
		"project-local /steamai template is not generated from the canonical skill",
	)
}

func TestInspectDetectsStaleGeneratedSTeamAIAppendix(t *testing.T) {
	repo := t.TempDir()
	writeReadyDocs(t, repo)
	for _, rel := range []string{skillcontract.CanonicalSkillPath, skillcontract.ProjectTemplatePath} {
		path := filepath.Join(repo, filepath.FromSlash(rel))
		data, err := os.ReadFile(path)
		if err != nil {
			t.Fatal(err)
		}
		stale := strings.Replace(string(data), "-ExpectedContinuePlanSha256", "-ExpectedStalePlanSha256", 1)
		writeFile(t, path, stale)
	}

	readiness := Inspect(repo)
	if readiness.Ready {
		t.Fatalf("public default docs unexpectedly ready despite stale generated appendix: %+v", readiness)
	}
	assertWarningContains(t, readiness.Warnings, "generated STeamAI machine command appendix is stale")
}

func TestInspectAllowsModuleCompatibilityIdentityWithoutRepositoryURL(t *testing.T) {
	repo := t.TempDir()
	writeReadyDocs(t, repo)
	writeFile(t, filepath.Join(repo, "docs", "autonomous-goal.md"), readyAutonomousGoal+"\nmodule "+moduleCompatibilityIdentity+"\n")

	readiness := Inspect(repo)
	if !readiness.Ready {
		t.Fatalf("bare Go module compatibility identity should remain allowed: %+v", readiness)
	}
}

func TestInspectDoesNotRequireLegacyRekitSkillAsCurrentDefault(t *testing.T) {
	repo := t.TempDir()
	writeReadyDocs(t, repo)

	readiness := Inspect(repo)
	if !readiness.Ready {
		t.Fatalf("current STeamAI public defaults should not require a legacy /rekit skill fixture: %+v", readiness)
	}
	for _, doc := range readiness.Documents {
		if doc.Path == ".claude/skills/rekit/SKILL.md" {
			t.Fatalf("legacy /rekit skill leaked into current default document inventory: %+v", readiness.Documents)
		}
	}
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
	readySkill := readySTeamAISkill + "\n" + skillcontract.MachineAppendixStart + "\nstale\n" + skillcontract.MachineAppendixEnd + "\n"
	updatedSkill, err := skillcontract.ReplaceMachineAppendix([]byte(readySkill))
	if err != nil {
		t.Fatal(err)
	}
	writeFile(t, filepath.Join(repo, ".claude", "skills", "steamai", "SKILL.md"), string(updatedSkill))
	writeFile(t, filepath.Join(repo, "rekit", "templates", "steamai-project", "SKILL.md"), string(updatedSkill))
	writeFile(t, filepath.Join(repo, "CLAUDE.md"), readyClaude)
	writeFile(t, filepath.Join(repo, "docs", "context-routing.md"), readyContextRouting)
	writeFile(t, filepath.Join(repo, "docs", "real-usage-hardening-roadmap.md"), readyRealUsageHardeningRoadmap)
	writeFile(t, filepath.Join(repo, "docs", "batch-plan.md"), readyBatchPlan)
	writeFile(t, filepath.Join(repo, "docs", "mission-control-product-direction.md"), readyMissionControlProductDirection)
	writeFile(t, filepath.Join(repo, "docs", "steamai-self-contained-project.md"), readySTeamAISelfContainedProject)
	writeFile(t, filepath.Join(repo, "docs", "autonomous-goal.md"), readyAutonomousGoal)
	writeFile(t, filepath.Join(repo, "docs", "reference-absorption.md"), readyReferenceAbsorption)
	writeFile(t, filepath.Join(repo, "docs", "release-readiness.md"), readyReleaseReadiness)
	writeFile(t, filepath.Join(repo, "docs", "promote-sync.md"), readyPromoteSync)
	writeFile(t, filepath.Join(repo, "docs", "powershell-deprecation.md"), readyPowerShellDeprecation)
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
Canonical repository: https://github.com/shuiyu486/STeamAI；clone: https://github.com/shuiyu486/STeamAI.git；Go module compatibility identity: github.com/shuiyu486/re-context-kits。
未接入的普通目录在 init 前没有项目级 ` + "`/steamai`" + `；目前仓库尚未提供面向普通用户的独立安装包。
新项目使用 /steamai，唯一 current 状态根 ` + "`.steamai/`" + `。
项目内 verified runtime 不能依赖旧绝对路径、机器 PATH 或原中央 kit。
旧 ` + "`/rekit`" + `、` + "`.rekit`" + ` 和中央 kit/thin-shim 模型只在迁移期间兼容，不是新项目默认。
暂停、恢复或停止同样只需自然语言。
`

const readySTeamAISkill = `# STeamAI 项目内 Mission Control 入口

新项目唯一可变状态根是 ` + "`${CLAUDE_PROJECT_DIR}/.steamai`" + `。
旧 ` + "`.rekit`" + ` 项目只走兼容入口。
不通过 PATH、全局 plugin、项目内 Go source 或外部 kit 回退。
bounded-autonomous-v1 preview 后 exact Apply；不要让用户记 SHA。
` + "`control`" + ` 始终 review-first。
typed ` + "`invocation`" + ` 是唯一通用命令桥；机器命令附录是固定 front door、deterministic owner bridge、argv 与 Apply binding 的唯一 owner；` + "`commandExecutable=false`" + ` 不执行。
`

const readyClaude = `# CLAUDE

Canonical repository: https://github.com/shuiyu486/STeamAI；module compatibility: github.com/shuiyu486/re-context-kits。
` + "`/steamai`" + ` canonical skill；legacy ` + "`/rekit`" + ` compatibility skill。
新项目不得回退机器 PATH 或外部 kit。
case public JSON 的 project-local typed command 由 resolved state root 统一投影。
exact lane ` + "`control`" + ` 使用独立 append-only generation。
当前已批准路线是 ` + "`steamai-product-optimization-v1`" + `；不做 installer。
`

const readyContextRouting = `# context routing

本项目文档必须做成按需路由、渐进式披露的样式。
STeamAI 自包含项目 / ` + "`.steamai`" + ` / ` + "`/steamai`" + ` / runtime bundle / legacy 迁移。
GitHub repository identity / clone / rename / Go module compatibility。
durable pause/resume/stop 与 late-result isolation。
不把旧中央 kit/thin-shim 流程当新项目默认。
`

const readyRealUsageHardeningRoadmap = `# roadmap

本文件是 active source。
Canonical repository: https://github.com/shuiyu486/STeamAI；module compatibility: github.com/shuiyu486/re-context-kits。
当前路线是 ` + "`steamai-product-optimization-v1`" + `，保持 source-clone-first，不实现 installer。
拒绝 PATH/外部 kit fallback。
默认 quickstart 只保留 ` + "`cd <project> → claude → /steamai`" + `。
`

const readyBatchPlan = `# batch plan

当前路线是 ` + "`steamai-product-optimization-v1`" + `。
唯一允许领取：完成 current closure。
`

const readyMissionControlProductDirection = `# mission

STeamAI Lane-centric Agent Team Mission Control。
新项目的用户入口是 ` + "`/steamai`" + `，唯一 current 状态根是 ` + "`.steamai`" + `。
` + "`/rekit`" + `、` + "`.rekit`" + ` 和 ` + "`rekit.ps1`" + ` 只作为迁移期 compatibility surface。
`

const readySTeamAISelfContainedProject = `# self-contained project

一个真实项目目录 = 一个自包含 STeamAI 项目。
项目复制或移动后不能依赖旧绝对路径、机器全局 PATH 或原中央 kit 仓库。
旧 ` + "`/rekit`" + ` 与 ` + "`.rekit`" + ` 在迁移期间只作为兼容入口。
case public JSON 按 resolved state root 投影全部 project-local typed command。
## 7. Durable execution control
`

const readyAutonomousGoal = `# goal

聊天 goal 只负责启动或继续**已批准路线**。
默认继续自主推进仅表示继续**已批准路线**。
`

const readyReferenceAbsorption = `# reference absorption

git clone https://github.com/shuiyu486/STeamAI.git
Go module compatibility identity: github.com/shuiyu486/re-context-kits。
`

const readyReleaseReadiness = `# release

Canonical repository: https://github.com/shuiyu486/STeamAI；clone: https://github.com/shuiyu486/STeamAI.git；module compatibility: github.com/shuiyu486/re-context-kits。
普通 batch 默认依赖 Go-owned ` + "`release-check`" + ` inventory。
current STeamAI entry readiness 与 legacy ` + "`/rekit`" + ` / ` + "`.rekit`" + ` compatibility readiness 分开验证且都参与 ready。
` + "`docs/promote-sync.md`" + ` 纳入 current guidance inventory。
Public ` + "`control`" + ` 是Go-owned、case-local、review-first mutation。
默认本机验证路径不依赖 PowerShell。
`

const readyPromoteSync = `# promote sync

current 项目使用 ` + "`/steamai`" + ` 或自然语言发起请求；legacy-only 项目才使用 ` + "`/rekit`" + `。
review 写入 <active-state-root>；双根 fail-closed。

## Legacy compatibility

legacy-only 项目可使用 ` + "`/rekit promote`" + `。
`

const readyPowerShellDeprecation = `# powershell

PowerShell-free default/product path / Go-native / 跨平台 convergence。
Go CLI/backend 是 canonical runtime。
PowerShell 当前只保留 ` + "`rekit/rekit.ps1`" + ` compatibility façade 与按需 parity residue。
`

const readyTestsReadme = `# tests

Go-first release gate 优先由 Go-owned ` + "`release-check`" + ` inventory。
推荐最小回归组合。
`
