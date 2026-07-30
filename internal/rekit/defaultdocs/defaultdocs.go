package defaultdocs

import (
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strings"
)

type Readiness struct {
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
	{Path: "README.md", Purpose: "primary public usage and maintenance entrypoint"},
	{Path: ".claude/skills/rekit/SKILL.md", Purpose: "canonical slash skill instructions"},
	{Path: "CLAUDE.md", Purpose: "project maintainer instructions"},
	{Path: "docs/context-routing.md", Purpose: "progressive-disclosure router and read-first policy"},
	{Path: "docs/batch-plan.md", Purpose: "current batch state and next candidates"},
	{Path: "docs/mission-control-product-direction.md", Purpose: "Lane-centric Mission Control product direction"},
	{Path: "docs/autonomous-goal.md", Purpose: "long-term autonomous handoff anchor"},
	{Path: "docs/release-readiness.md", Purpose: "release gate default validation path"},
	{Path: "docs/go-first-convergence-plan.md", Purpose: "Go-first convergence stage map and validation defaults"},
	{Path: "docs/go-runtime-migration.md", Purpose: "Go runtime migration history and fallback boundary guidance"},
	{Path: "docs/powershell-deprecation.md", Purpose: "PowerShell-free roadmap and fallback retirement gates"},
	{Path: "docs/vision.md", Purpose: "long-term framework vision and validation guidance"},
	{Path: "docs/reference-absorption.md", Purpose: "reference absorption map and handoff validation guidance"},
	{Path: "docs/agent-team-rollout-plan.md", Purpose: "Agent Team rollout status and validation guidance"},
	{Path: "rekit/tests/README.md", Purpose: "smoke selection guide and recommended validation defaults"},
}

var requiredPhrases = []requiredPhrase{
	{path: "README.md", phrase: "用户主要指挥主 Agent / Mission Commander"},
	{path: "README.md", phrase: "Go CLI/backend 是背后的 canonical deterministic runtime/API"},
	{path: "README.md", phrase: "`rekit.ps1` 仅作为 retained compatibility façade"},
	{path: "README.md", phrase: "默认路径继续向 PowerShell-free / Go-native / 跨平台收敛"},
	{path: "README.md", phrase: "这里不需要你手动执行底层脚本"},
	{path: "README.md", phrase: "用户不需要把 `/rekit` 子命令当成主要交互界面"},
	{path: "README.md", phrase: "`continue -Apply` 不写 authority/confirmed"},
	{path: "docs/mission-control-product-direction.md", phrase: "Lane-centric Agent Team Mission Control"},
	{path: "docs/mission-control-product-direction.md", phrase: "用户主要和一个 **主 Agent / Mission Commander** 会话交互"},
	{path: "docs/mission-control-product-direction.md", phrase: "Go-first deterministic substrate"},
	{path: ".claude/skills/rekit/SKILL.md", phrase: "产品方向是 Mission Control"},
	{path: ".claude/skills/rekit/SKILL.md", phrase: "底层 Go CLI 是 canonical runtime"},
	{path: ".claude/skills/rekit/SKILL.md", phrase: "`rekit.ps1` 只是 retained compatibility façade"},
	{path: ".claude/skills/rekit/SKILL.md", phrase: "底层 runtime 只作为 `/rekit` 的内部实现"},
	{path: ".claude/skills/rekit/SKILL.md", phrase: "`continue -Apply` 不写 authority/confirmed"},
	{path: "docs/context-routing.md", phrase: "按需路由"},
	{path: "docs/context-routing.md", phrase: "渐进式披露"},
	{path: "docs/context-routing.md", phrase: "不要默认读取 `docs/batch-history.md` 全文"},
	{path: "docs/batch-plan.md", phrase: "完整历史已拆到 `docs/batch-history.md`"},
	{path: "CLAUDE.md", phrase: "PowerShell-free default/product path、Go-native、跨平台"},
	{path: "CLAUDE.md", phrase: "PowerShell replacement/removal 不再因“删除 PowerShell”本身停下询问"},
	{path: "docs/autonomous-goal.md", phrase: "每轮自主推进按这个循环做"},
	{path: "docs/autonomous-goal.md", phrase: "默认继续自主推进"},
	{path: "docs/release-readiness.md", phrase: "发布门禁默认依赖 Go-owned `release-check` inventory"},
	{path: "docs/release-readiness.md", phrase: "默认本机验证路径不依赖 PowerShell"},
	{path: "docs/go-first-convergence-plan.md", phrase: "Go backend 成为 rekit 的 deterministic runtime owner"},
	{path: "docs/go-first-convergence-plan.md", phrase: "不要把大型 PowerShell matrix 作为默认必跑"},
	{path: "docs/go-first-convergence-plan.md", phrase: "PowerShell-free / Go-native convergence"},
	{path: "docs/go-runtime-migration.md", phrase: "当前默认验证应优先运行 Go-native release gate"},
	{path: "docs/go-runtime-migration.md", phrase: "`/rekit` remains the public ABI"},
	{path: "docs/go-runtime-migration.md", phrase: "default implementation converges to the Go deterministic backend"},
	{path: "docs/powershell-deprecation.md", phrase: "PowerShell-free default/product path / Go-native / 跨平台 convergence"},
	{path: "docs/powershell-deprecation.md", phrase: "Go CLI/backend 是 canonical runtime"},
	{path: "docs/powershell-deprecation.md", phrase: "PowerShell 当前只保留 `rekit/rekit.ps1` compatibility façade 与按需 parity residue"},
	{path: "docs/vision.md", phrase: "Claude Code Agent Team Mission Control 框架"},
	{path: "docs/vision.md", phrase: "优先运行 Go-native 检查"},
	{path: "docs/reference-absorption.md", phrase: "Go-native release readiness 子集"},
	{path: "docs/reference-absorption.md", phrase: "不能宣称已具备自动脱壳/逆向引擎"},
	{path: "docs/agent-team-rollout-plan.md", phrase: "公共 `/rekit` 默认路径已由 22 个 Go-owned/no-fallback public commands 支撑"},
	{path: "docs/agent-team-rollout-plan.md", phrase: "Go-native `status`、`doctor` 与 `release-check` 不回归"},
	{path: "rekit/tests/README.md", phrase: "Go-first release gate 优先由 Go-owned `release-check` inventory"},
	{path: "rekit/tests/README.md", phrase: "推荐最小回归组合"},
}

var forbiddenFacadeCommand = regexp.MustCompile(`(?i)(^|[\s` + "`" + `])(?:\.?[\\/])?rekit[\\/]rekit\.ps1\s+(?:-[a-z][a-z0-9-]*\s+)*?(?:release-check|status|packs|doctor|validate|overview|continue|start|handoff|sync|promote|note|gate|plan-subagents|attach|init|bootstrap|repair)\b`)

var boundaries = []string{
	"public docs may mention PowerShell only as legacy façade, fallback, or compatibility residue",
	"public daily-use docs must keep natural-language Mission Control and `/rekit` as the user-facing path",
	"Go CLI/backend remains the canonical deterministic runtime/API behind `/rekit`",
	"this readiness check is read-only and does not delete PowerShell files or change façade delegation",
}

func Inspect(repoRoot string) Readiness {
	readiness := Readiness{
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
