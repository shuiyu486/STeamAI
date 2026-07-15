package defaultdocs

import (
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strings"
)

type Readiness struct {
	Ready             bool               `json:"ready"`
	Summary           string             `json:"summary"`
	Documents         []DocumentCheck    `json:"documents"`
	RequiredPhrases   []PhraseCheck      `json:"requiredPhrases"`
	ForbiddenCommands []ForbiddenCommand `json:"forbiddenCommands"`
	Boundaries        []string           `json:"boundaries"`
	Warnings          []string           `json:"warnings"`
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

type requiredPhrase struct {
	path   string
	phrase string
}

const forbiddenFacadeCommandPattern = `rekit/rekit.ps1 command snippet`

var documents = []DocumentCheck{
	{Path: "README.md", Purpose: "primary public usage and maintenance entrypoint"},
	{Path: ".claude/skills/rekit/SKILL.md", Purpose: "canonical slash skill instructions"},
	{Path: "CLAUDE.md", Purpose: "project maintainer instructions"},
	{Path: "docs/autonomous-goal.md", Purpose: "long-term autonomous handoff anchor"},
}

var requiredPhrases = []requiredPhrase{
	{path: "README.md", phrase: "用户主要指挥主 Agent / Mission Commander"},
	{path: "README.md", phrase: "Go CLI/backend 是背后的 canonical deterministic runtime/API"},
	{path: "README.md", phrase: "`rekit.ps1` 仅作为迁移期 legacy façade"},
	{path: "README.md", phrase: "默认路径继续向 PowerShell-free / Go-native / 跨平台收敛"},
	{path: "README.md", phrase: "这里不需要你手动执行底层脚本"},
	{path: "README.md", phrase: "用户不需要把 `/rekit` 子命令当成主要交互界面"},
	{path: ".claude/skills/rekit/SKILL.md", phrase: "产品方向是 Mission Control"},
	{path: ".claude/skills/rekit/SKILL.md", phrase: "底层 Go CLI 是 canonical runtime"},
	{path: ".claude/skills/rekit/SKILL.md", phrase: "`rekit.ps1` 只是迁移期 legacy façade"},
	{path: ".claude/skills/rekit/SKILL.md", phrase: "底层 runtime 只作为 `/rekit` 的内部实现"},
	{path: "CLAUDE.md", phrase: "PowerShell-free / Go-native / 跨平台收敛"},
	{path: "CLAUDE.md", phrase: "PowerShell replacement/removal 不再因“删除 PowerShell”本身停下询问"},
	{path: "CLAUDE.md", phrase: "默认远程 CI 入口是 `.github/workflows/release-gate.yml`，在 Linux、Windows、macOS 上运行 Go-native release checks"},
	{path: "docs/autonomous-goal.md", phrase: "PowerShell-free / Go-native / 跨平台"},
	{path: "docs/autonomous-goal.md", phrase: "每轮自主推进按这个循环做"},
	{path: "docs/autonomous-goal.md", phrase: "默认继续自主推进"},
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
		Ready:             true,
		Summary:           "public default docs readiness ok",
		Documents:         []DocumentCheck{},
		RequiredPhrases:   []PhraseCheck{},
		ForbiddenCommands: []ForbiddenCommand{},
		Boundaries:        append([]string{}, boundaries...),
		Warnings:          []string{},
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
	}
	for _, forbidden := range readiness.ForbiddenCommands {
		if forbidden.Present {
			readiness.Warnings = append(readiness.Warnings, fmt.Sprintf("public default doc %s:%d exposes PowerShell façade command as a default path: %s", forbidden.Path, forbidden.Line, forbidden.Snippet))
		}
	}
	if len(readiness.Warnings) > 0 {
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
