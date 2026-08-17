package defaultdocs

import (
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strings"
)

type Readiness struct {
	Model                string                `json:"model"`
	DefaultEntrypoint    string                `json:"defaultEntrypoint"`
	StateRoot            string                `json:"stateRoot"`
	RuntimeSource        string                `json:"runtimeSource"`
	FallbackAllowed      bool                  `json:"fallbackAllowed"`
	Ready                bool                  `json:"ready"`
	Summary              string                `json:"summary"`
	Documents            []DocumentCheck       `json:"documents"`
	RequiredPhrases      []PhraseCheck         `json:"requiredPhrases"`
	ForbiddenCommands    []ForbiddenCommand    `json:"forbiddenCommands"`
	ForbiddenShellFences []ForbiddenShellFence `json:"forbiddenShellFences"`
	Boundaries           []string              `json:"boundaries"`
	Warnings             []string              `json:"warnings"`
}

type DocumentCheck struct {
	Path    string `json:"path"`
	Present bool   `json:"present"`
	Purpose string `json:"purpose"`
}

type PhraseCheck struct {
	Path    string `json:"path"`
	Phrase  string `json:"phrase"`
	Present bool   `json:"present"`
}

type ForbiddenCommand struct {
	Path    string `json:"path"`
	Pattern string `json:"pattern"`
	Line    int    `json:"line"`
	Snippet string `json:"snippet"`
	Present bool   `json:"present"`
}

type ForbiddenShellFence struct {
	Path     string `json:"path"`
	Language string `json:"language"`
	Line     int    `json:"line"`
	Snippet  string `json:"snippet"`
	Present  bool   `json:"present"`
}

type ReadinessCounts struct {
	Documents            int
	RequiredPhrases      int
	ForbiddenCommands    int
	ForbiddenShellFences int
	Boundaries           int
	Warnings             int
}

func ReadinessCountsFor(readiness Readiness) ReadinessCounts {
	return ReadinessCounts{
		Documents:            len(readiness.Documents),
		RequiredPhrases:      len(readiness.RequiredPhrases),
		ForbiddenCommands:    len(readiness.ForbiddenCommands),
		ForbiddenShellFences: len(readiness.ForbiddenShellFences),
		Boundaries:           len(readiness.Boundaries),
		Warnings:             len(readiness.Warnings),
	}
}

type requiredPhrase struct {
	path   string
	phrase string
}

const forbiddenFacadeCommandPattern = `rekit/rekit.ps1 command snippet`

var documents = []DocumentCheck{
	{Path: "README.md", Purpose: "primary current STeamAI public usage entrypoint"},
	{Path: ".claude/skills/steamai/SKILL.md", Purpose: "canonical current /steamai slash skill"},
	{Path: "rekit/templates/steamai-project/SKILL.md", Purpose: "project-local /steamai skill published into current projects"},
	{Path: "CLAUDE.md", Purpose: "stable project maintainer boundaries and router pointer"},
	{Path: "docs/context-routing.md", Purpose: "canonical progressive-disclosure router and current-model route"},
	{Path: "docs/real-usage-hardening-roadmap.md", Purpose: "active STeamAI self-contained route and current batch card"},
	{Path: "docs/batch-plan.md", Purpose: "compact current route projection"},
	{Path: "docs/mission-control-product-direction.md", Purpose: "STeamAI self-contained Mission Control product direction"},
	{Path: "docs/steamai-self-contained-project.md", Purpose: "current self-contained project contract"},
	{Path: "docs/autonomous-goal.md", Purpose: "short approved-route goal anchor"},
	{Path: "docs/release-readiness.md", Purpose: "routed current and legacy release gate guidance"},
	{Path: "docs/powershell-deprecation.md", Purpose: "routed PowerShell retirement contract"},
	{Path: "rekit/tests/README.md", Purpose: "routed smoke selection guide"},
}

var requiredPhrases = []requiredPhrase{
	{path: "README.md", phrase: "用户主要指挥主 Agent / Mission Commander"},
	{path: "README.md", phrase: "/steamai"},
	{path: "README.md", phrase: "未接入的普通目录在 init 前没有项目级 `/steamai`"},
	{path: "README.md", phrase: "目前仓库尚未提供面向普通用户的独立安装包"},
	{path: "README.md", phrase: "唯一 current 状态根 `.steamai/`"},
	{path: "README.md", phrase: "不能依赖旧绝对路径、机器 PATH 或原中央 kit"},
	{path: "README.md", phrase: "旧 `/rekit`、`.rekit` 和中央 kit/thin-shim 模型只在迁移期间兼容，不是新项目默认"},
	{path: ".claude/skills/steamai/SKILL.md", phrase: "STeamAI 项目内 Mission Control 入口"},
	{path: ".claude/skills/steamai/SKILL.md", phrase: "新项目唯一可变状态根是 `${CLAUDE_PROJECT_DIR}/.steamai`"},
	{path: ".claude/skills/steamai/SKILL.md", phrase: "不通过 PATH、全局 plugin、项目内 Go source 或外部 kit 回退"},
	{path: ".claude/skills/steamai/SKILL.md", phrase: "bounded-autonomous-v1"},
	{path: ".claude/skills/steamai/SKILL.md", phrase: "不要让用户记 SHA"},
	{path: ".claude/skills/steamai/SKILL.md", phrase: "typed `invocation` 是唯一通用命令桥"},
	{path: ".claude/skills/steamai/SKILL.md", phrase: "[\"runtime\", \"-Command\", invocation.command]"},
	{path: ".claude/skills/steamai/SKILL.md", phrase: "`commandExecutable=false`"},
	{path: "rekit/templates/steamai-project/SKILL.md", phrase: "自包含 STeamAI 项目"},
	{path: "rekit/templates/steamai-project/SKILL.md", phrase: "`${CLAUDE_PROJECT_DIR}/.steamai/instance.yml`"},
	{path: "rekit/templates/steamai-project/SKILL.md", phrase: "不通过 PATH 或外部 kit 回退"},
	{path: "rekit/templates/steamai-project/SKILL.md", phrase: "bounded-autonomous-v1"},
	{path: "rekit/templates/steamai-project/SKILL.md", phrase: "不要让用户记 SHA"},
	{path: "rekit/templates/steamai-project/SKILL.md", phrase: "typed `invocation` 是唯一通用命令桥"},
	{path: "rekit/templates/steamai-project/SKILL.md", phrase: "[\"runtime\", \"-Command\", invocation.command]"},
	{path: "rekit/templates/steamai-project/SKILL.md", phrase: "`commandExecutable=false`"},
	{path: "CLAUDE.md", phrase: "`/steamai` canonical skill"},
	{path: "CLAUDE.md", phrase: "legacy `/rekit` compatibility skill"},
	{path: "CLAUDE.md", phrase: "不得回退机器 PATH 或外部 kit"},
	{path: "CLAUDE.md", phrase: "case public JSON 的 project-local typed command 由 resolved state root 统一投影"},
	{path: "docs/context-routing.md", phrase: "本项目文档必须做成按需路由、渐进式披露的样式"},
	{path: "docs/context-routing.md", phrase: "STeamAI 自包含项目 / `.steamai` / `/steamai` / runtime bundle / legacy 迁移"},
	{path: "docs/context-routing.md", phrase: "不把旧中央 kit/thin-shim 流程当新项目默认"},
	{path: "docs/real-usage-hardening-roadmap.md", phrase: "active source"},
	{path: "docs/real-usage-hardening-roadmap.md", phrase: "当前路线是 `steamai-self-contained-project-v1`"},
	{path: "docs/real-usage-hardening-roadmap.md", phrase: "拒绝 PATH/外部 kit fallback"},
	{path: "docs/real-usage-hardening-roadmap.md", phrase: "默认 quickstart 只保留 `cd <project> → claude → /steamai`"},
	{path: "docs/batch-plan.md", phrase: "当前路线是 `steamai-self-contained-project-v1`"},
	{path: "docs/batch-plan.md", phrase: "唯一允许领取"},
	{path: "docs/mission-control-product-direction.md", phrase: "STeamAI Lane-centric Agent Team Mission Control"},
	{path: "docs/mission-control-product-direction.md", phrase: "新项目的用户入口是 `/steamai`"},
	{path: "docs/mission-control-product-direction.md", phrase: "唯一 current 状态根是 `.steamai`"},
	{path: "docs/mission-control-product-direction.md", phrase: "`/rekit`、`.rekit` 和 `rekit.ps1` 只作为迁移期 compatibility surface"},
	{path: "docs/steamai-self-contained-project.md", phrase: "一个真实项目目录 = 一个自包含 STeamAI 项目"},
	{path: "docs/steamai-self-contained-project.md", phrase: "不能依赖旧绝对路径、机器全局 PATH 或原中央 kit 仓库"},
	{path: "docs/steamai-self-contained-project.md", phrase: "旧 `/rekit` 与 `.rekit` 在迁移期间只作为兼容入口"},
	{path: "docs/steamai-self-contained-project.md", phrase: "case public JSON 按 resolved state root 投影全部 project-local typed command"},
	{path: "docs/autonomous-goal.md", phrase: "聊天 goal 只负责启动或继续**已批准路线**"},
	{path: "docs/autonomous-goal.md", phrase: "默认继续自主推进仅表示继续**已批准路线**"},
	{path: "docs/release-readiness.md", phrase: "普通 batch 默认依赖 Go-owned `release-check` inventory"},
	{path: "docs/release-readiness.md", phrase: "current STeamAI entry readiness"},
	{path: "docs/release-readiness.md", phrase: "legacy `/rekit` / `.rekit` compatibility readiness"},
	{path: "docs/release-readiness.md", phrase: "默认本机验证路径不依赖 PowerShell"},
	{path: "docs/powershell-deprecation.md", phrase: "PowerShell-free default/product path / Go-native / 跨平台 convergence"},
	{path: "docs/powershell-deprecation.md", phrase: "Go CLI/backend 是 canonical runtime"},
	{path: "docs/powershell-deprecation.md", phrase: "PowerShell 当前只保留 `rekit/rekit.ps1` compatibility façade 与按需 parity residue"},
	{path: "rekit/tests/README.md", phrase: "Go-first release gate 优先由 Go-owned `release-check` inventory"},
	{path: "rekit/tests/README.md", phrase: "推荐最小回归组合"},
}

var forbiddenFacadeCommand = regexp.MustCompile(`(?i)(^|[\s` + "`" + `])(?:\.?[\\/])?rekit[\\/]rekit\.ps1\s+(?:-[a-z][a-z0-9-]*\s+)*?(?:release-check|status|packs|doctor|validate|overview|continue|start|handoff|sync|promote|note|gate|plan-subagents|attach|init|bootstrap|repair)\b`)

var boundaries = []string{
	"current public daily-use entrypoint is natural-language Mission Control through /steamai",
	"current projects use only .steamai state and a verified project-local runtime/pack without PATH or external-kit fallback",
	"/rekit, .rekit, the central kit, and the thin shim remain explicit legacy compatibility or migration surfaces, not new-project defaults",
	"legacy compatibility readiness is checked separately and remains required for release readiness",
	"this readiness check is read-only and does not publish a runtime bundle, migrate state, or change compatibility delegation",
}

func Inspect(repoRoot string) Readiness {
	readiness := Readiness{
		Model:                "steamai-self-contained-current",
		DefaultEntrypoint:    "/steamai",
		StateRoot:            ".steamai",
		RuntimeSource:        "project-local-verified-bundle",
		FallbackAllowed:      false,
		Ready:                true,
		Summary:              "public default docs readiness ok",
		Documents:            []DocumentCheck{},
		RequiredPhrases:      []PhraseCheck{},
		ForbiddenCommands:    []ForbiddenCommand{},
		ForbiddenShellFences: []ForbiddenShellFence{},
		Boundaries:           append([]string{}, boundaries...),
		Warnings:             []string{},
	}
	texts := map[string]string{}
	for _, doc := range documents {
		data, err := os.ReadFile(filepath.Join(repoRoot, filepath.FromSlash(doc.Path)))
		if err != nil {
			doc.Present = false
			readiness.Warnings = append(readiness.Warnings, fmt.Sprintf("public default doc missing: %s: %v", doc.Path, err))
		} else {
			doc.Present = true
			texts[doc.Path] = string(data)
		}
		readiness.Documents = append(readiness.Documents, doc)
	}
	for _, required := range requiredPhrases {
		check := PhraseCheck{Path: required.path, Phrase: required.phrase}
		check.Present = strings.Contains(texts[required.path], required.phrase)
		if !check.Present {
			readiness.Warnings = append(readiness.Warnings, fmt.Sprintf("public default doc %s missing required phrase: %s", required.path, required.phrase))
		}
		readiness.RequiredPhrases = append(readiness.RequiredPhrases, check)
	}
	for _, doc := range documents {
		text, ok := texts[doc.Path]
		if !ok {
			continue
		}
		readiness.ForbiddenCommands = append(readiness.ForbiddenCommands, forbiddenCommandsInDoc(doc.Path, text)...)
		readiness.ForbiddenShellFences = append(readiness.ForbiddenShellFences, forbiddenShellFencesInDoc(doc.Path, text)...)
	}
	for _, forbidden := range readiness.ForbiddenCommands {
		if forbidden.Present {
			readiness.Warnings = append(readiness.Warnings, fmt.Sprintf("public default doc %s:%d exposes PowerShell façade command as a default path: %s", forbidden.Path, forbidden.Line, forbidden.Snippet))
		}
	}
	for _, forbidden := range readiness.ForbiddenShellFences {
		if forbidden.Present {
			readiness.Warnings = append(readiness.Warnings, fmt.Sprintf("public default doc %s:%d uses PowerShell shell fence in default docs: %s", forbidden.Path, forbidden.Line, forbidden.Snippet))
		}
	}
	if ReadinessCountsFor(readiness).Warnings > 0 {
		readiness.Ready = false
		readiness.Summary = "public default docs readiness has warnings"
	}
	return readiness
}

func forbiddenCommandsInDoc(path, text string) []ForbiddenCommand {
	lines := strings.Split(strings.ReplaceAll(text, "\r\n", "\n"), "\n")
	checks := []ForbiddenCommand{}
	for i, line := range lines {
		if !forbiddenFacadeCommand.MatchString(line) {
			continue
		}
		checks = append(checks, ForbiddenCommand{
			Path:    path,
			Pattern: forbiddenFacadeCommandPattern,
			Line:    i + 1,
			Snippet: strings.TrimSpace(line),
			Present: true,
		})
	}
	return checks
}

func forbiddenShellFencesInDoc(path, text string) []ForbiddenShellFence {
	lines := strings.Split(strings.ReplaceAll(text, "\r\n", "\n"), "\n")
	checks := []ForbiddenShellFence{}
	for i, line := range lines {
		trimmed := strings.TrimSpace(line)
		if trimmed != "```powershell" && trimmed != "```pwsh" {
			continue
		}
		checks = append(checks, ForbiddenShellFence{
			Path:     path,
			Language: strings.TrimPrefix(trimmed, "```"),
			Line:     i + 1,
			Snippet:  trimmed,
			Present:  true,
		})
	}
	return checks
}
