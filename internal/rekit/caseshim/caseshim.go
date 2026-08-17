package caseshim

import (
	"bytes"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/shuiyu486/re-context-kits/internal/rekit/sourceartifact"
)

type Readiness struct {
	Model                   string        `json:"model"`
	CompatibilityEntrypoint string        `json:"compatibilityEntrypoint"`
	StateRoot               string        `json:"stateRoot"`
	DefaultForNewProjects   bool          `json:"defaultForNewProjects"`
	TemplatePath            string        `json:"templatePath"`
	CanonicalSkillPath      string        `json:"canonicalSkillPath"`
	Ready                   bool          `json:"ready"`
	Summary                 string        `json:"summary"`
	RequiredPhrases         []PhraseCheck `json:"requiredPhrases"`
	CanonicalSkillPhrases   []PhraseCheck `json:"canonicalSkillPhrases"`
	ForbiddenStrings        []StringCheck `json:"forbiddenStrings"`
	Boundaries              []string      `json:"boundaries"`
	Warnings                []string      `json:"warnings"`
}

type InstalledReadiness struct {
	ShimPath        string   `json:"shimPath"`
	TemplatePath    string   `json:"templatePath"`
	MatchesTemplate bool     `json:"matchesTemplate"`
	Ready           bool     `json:"ready"`
	Summary         string   `json:"summary"`
	Warnings        []string `json:"warnings"`
}

type PhraseCheck struct {
	Phrase  string `json:"phrase"`
	Present bool   `json:"present"`
}

type StringCheck struct {
	Pattern string `json:"pattern"`
	Present bool   `json:"present"`
}

type ReadinessCounts struct {
	RequiredPhrases       int
	CanonicalSkillPhrases int
	ForbiddenStrings      int
	Boundaries            int
	Warnings              int
}

func ReadinessCountsFor(readiness Readiness) ReadinessCounts {
	return ReadinessCounts{
		RequiredPhrases:       len(readiness.RequiredPhrases),
		CanonicalSkillPhrases: len(readiness.CanonicalSkillPhrases),
		ForbiddenStrings:      len(readiness.ForbiddenStrings),
		Boundaries:            len(readiness.Boundaries),
		Warnings:              len(readiness.Warnings),
	}
}

const TemplateRelPath = "rekit/templates/case-shim/SKILL.md"
const CanonicalSkillRelPath = ".claude/skills/rekit/SKILL.md"

var requiredShimPhrases = []string{
	"case-local 薄 shim",
	"不包含业务逻辑",
	"canonical `/rekit`",
	".rekit/instance.yml",
	".re-template.yml",
	"<templateRoot>/.claude/skills/rekit/SKILL.md",
	"canonical runtime",
	"sync` / `promote` 默认必须 review-first",
	"新会话 first screen 先使用 `/rekit`",
	"status case shim ready=true",
	"installedShimMatchesTemplate=true",
	"durable artifacts 接手",
	"next-batch action queues",
	"不要在本 shim 里维护模板规则",
	"不要读取或修改用户级 `~/.claude/skills`",
	"不要在 shim 中复制逻辑",
	"不展示底层脚本或 CLI 命令",
	"Go-native backend",
}

var requiredCanonicalSkillPhrases = []string{
	"底层 Go CLI 是 canonical runtime",
	"`rekit.ps1` 只是 retained compatibility façade",
	"case 只生成 `.claude/skills/rekit/SKILL.md` 薄 shim",
	"底层 runtime 只作为 `/rekit` 的内部实现",
}

var forbiddenShimStrings = []string{
	"rekit.ps1",
	".ps1",
	"PowerShell",
	"pwsh",
	"cmd/rekit",
	"go run",
	"REKIT_GO_DISABLE",
	"Set-ExecutionPolicy",
	"Invoke-Rekit",
}

var boundaries = []string{
	"legacy case-local /rekit shim compatibility remains release-blocking while migration support is retained",
	"legacy case-local shim is not the default UX or install path for new STeamAI projects",
	"legacy case-local shim contains no runtime logic",
	"legacy case-local shim delegates to the central-kit canonical /rekit compatibility skill",
	"legacy .rekit first screen remains /rekit status with installed shim readiness and durable artifact handoff",
	"legacy case-local shim does not name PowerShell, pwsh, Go CLI commands, or façade fallback switches",
	"sync/promote remain review-first through the canonical runtime",
}

func Inspect(repoRoot string) Readiness {
	result := Readiness{
		Model:                   "legacy-rekit-case-shim-compatibility",
		CompatibilityEntrypoint: "/rekit",
		StateRoot:               ".rekit",
		DefaultForNewProjects:   false,
		TemplatePath:            TemplateRelPath,
		CanonicalSkillPath:      CanonicalSkillRelPath,
		Ready:                   true,
		Summary:                 "case shim readiness ok",
		RequiredPhrases:         []PhraseCheck{},
		CanonicalSkillPhrases:   []PhraseCheck{},
		ForbiddenStrings:        []StringCheck{},
		Boundaries:              append([]string{}, boundaries...),
		Warnings:                []string{},
	}

	shimText, err := readRepoText(repoRoot, TemplateRelPath)
	if err != nil {
		result.Ready = false
		result.Warnings = append(result.Warnings, err.Error())
	}
	canonicalText, err := readRepoText(repoRoot, CanonicalSkillRelPath)
	if err != nil {
		result.Ready = false
		result.Warnings = append(result.Warnings, err.Error())
	}

	for _, phrase := range requiredShimPhrases {
		present := strings.Contains(shimText, phrase)
		result.RequiredPhrases = append(result.RequiredPhrases, PhraseCheck{Phrase: phrase, Present: present})
		if !present {
			result.Ready = false
			result.Warnings = append(result.Warnings, fmt.Sprintf("case shim missing required phrase: %s", phrase))
		}
	}
	for _, phrase := range requiredCanonicalSkillPhrases {
		present := strings.Contains(canonicalText, phrase)
		result.CanonicalSkillPhrases = append(result.CanonicalSkillPhrases, PhraseCheck{Phrase: phrase, Present: present})
		if !present {
			result.Ready = false
			result.Warnings = append(result.Warnings, fmt.Sprintf("canonical skill missing required phrase: %s", phrase))
		}
	}
	lowerShim := strings.ToLower(shimText)
	for _, pattern := range forbiddenShimStrings {
		present := strings.Contains(lowerShim, strings.ToLower(pattern))
		result.ForbiddenStrings = append(result.ForbiddenStrings, StringCheck{Pattern: pattern, Present: present})
		if present {
			result.Ready = false
			result.Warnings = append(result.Warnings, fmt.Sprintf("case shim contains forbidden runtime/default entrypoint string: %s", pattern))
		}
	}
	if ReadinessCountsFor(result).Warnings > 0 {
		result.Summary = "case shim readiness has warnings"
	}
	return result
}

func AssertReady(repoRoot string) error {
	readiness := Inspect(repoRoot)
	if readiness.Ready {
		return nil
	}
	return fmt.Errorf("case shim readiness failed: %s", strings.Join(readiness.Warnings, "; "))
}

func InspectInstalled(repoRoot, caseRoot string) InstalledReadiness {
	template := filepath.Join(repoRoot, filepath.FromSlash(TemplateRelPath))
	shim := filepath.Join(caseRoot, ".claude", "skills", "rekit", "SKILL.md")
	result := InstalledReadiness{
		ShimPath:        shim,
		TemplatePath:    template,
		MatchesTemplate: true,
		Ready:           true,
		Summary:         "installed case shim readiness ok",
		Warnings:        []string{},
	}
	left, err := os.ReadFile(shim)
	if err != nil {
		result.MatchesTemplate = false
		result.Ready = false
		result.Warnings = append(result.Warnings, fmt.Sprintf("case shim readiness missing installed shim: %s: %v", shim, err))
		result.Summary = "installed case shim readiness has warnings"
		return result
	}
	right, err := os.ReadFile(template)
	if err != nil {
		result.MatchesTemplate = false
		result.Ready = false
		result.Warnings = append(result.Warnings, fmt.Sprintf("case shim readiness missing canonical thin shim template: %s: %v", template, err))
		result.Summary = "installed case shim readiness has warnings"
		return result
	}
	if !bytes.Equal(sourceartifact.SemanticText(left), sourceartifact.SemanticText(right)) {
		result.MatchesTemplate = false
		result.Ready = false
		result.Warnings = append(result.Warnings, fmt.Sprintf("case-local /rekit shim differs from canonical thin shim template: %s", shim))
	}
	if len(result.Warnings) > 0 {
		result.Summary = "installed case shim readiness has warnings"
	}
	return result
}

func readRepoText(repoRoot, rel string) (string, error) {
	path := filepath.Join(repoRoot, filepath.FromSlash(rel))
	data, err := os.ReadFile(path)
	if err != nil {
		return "", fmt.Errorf("case shim readiness missing %s: %w", rel, err)
	}
	return string(data), nil
}
