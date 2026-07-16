package releasecheck

import (
	"encoding/json"
	"fmt"
	"os"
	"path"
	"path/filepath"
	"sort"
	"strings"

	"github.com/shuiyu486/re-context-kits/internal/rekit/caseshim"
	"github.com/shuiyu486/re-context-kits/internal/rekit/defaultdocs"
	"github.com/shuiyu486/re-context-kits/internal/rekit/manifest"
)

const commandName = "release-check"

type Result struct {
	Command               string                 `json:"command"`
	SchemaVersion         int                    `json:"schemaVersion"`
	IsMutation            bool                   `json:"isMutation"`
	RepoRoot              string                 `json:"repoRoot"`
	Ready                 bool                   `json:"ready"`
	Summary               string                 `json:"summary"`
	GateProfile           GateProfile            `json:"gateProfile"`
	CIReleaseGate         CIReleaseGate          `json:"ciReleaseGate"`
	RecommendedMinimum    []GateStep             `json:"recommendedMinimum"`
	RequiredCommands      []GateStep             `json:"requiredCommands"`
	Documents             []DocumentCheck        `json:"documents"`
	Packs                 []manifest.PackSummary `json:"packs"`
	PowerShellDeprecation PowerShellDeprecation  `json:"powerShellDeprecation"`
	GoNativePublicSurface GoNativePublicSurface  `json:"goNativePublicSurface"`
	CaseShim              caseshim.Readiness     `json:"caseShim"`
	PublicDefaultDocs     defaultdocs.Readiness  `json:"publicDefaultDocs"`
	ReleaseHandoff        ReleaseHandoff         `json:"releaseHandoff"`
	HeavyToolGateActions  []string               `json:"heavyToolGateActions"`
	Boundaries            []string               `json:"boundaries"`
	KnownGaps             []string               `json:"knownGaps"`
	Warnings              []string               `json:"warnings"`
}

type GateProfile struct {
	Name               string     `json:"name"`
	Description        string     `json:"description"`
	DefaultFor         []string   `json:"defaultFor"`
	Ready              bool       `json:"ready"`
	StepCount          int        `json:"stepCount"`
	LargeMatrixDefault bool       `json:"largeMatrixDefault"`
	Steps              []GateStep `json:"steps"`
}

type GateStep struct {
	Command   string `json:"command"`
	Source    string `json:"source"`
	Kind      string `json:"kind"`
	RepoPath  string `json:"repoPath,omitempty"`
	Present   bool   `json:"present"`
	Required  bool   `json:"required"`
	InCatalog bool   `json:"inCatalog"`
	Resolved  bool   `json:"resolved"`
}

type DocumentCheck struct {
	Path    string `json:"path"`
	Present bool   `json:"present"`
	Purpose string `json:"purpose"`
}

type catalog struct {
	RecommendedMinimum []string `json:"recommendedMinimum"`
	GlobalBoundaries   []string `json:"globalBoundaries"`
}

var requiredCommands = []string{
	"go run ./cmd/rekit -- -Command release-check -Format json",
	"go run ./cmd/rekit -- -Command status",
	"go run ./cmd/rekit -- -Command packs",
	"go run ./cmd/rekit -- -Command doctor",
	"go test ./...",
	"go vet ./...",
	"git diff --check",
}

var requiredDocuments = []DocumentCheck{
	{Path: "docs/release-readiness.md", Purpose: "release gate and known gaps"},
	{Path: "docs/mission-control-product-direction.md", Purpose: "Mission Control product north star"},
	{Path: "docs/autonomous-goal.md", Purpose: "long-term autonomous goal and handoff guide"},
	{Path: "docs/go-first-convergence-plan.md", Purpose: "Go-first stage completion signals"},
	{Path: "docs/powershell-deprecation.md", Purpose: "PowerShell owner and freeze strategy"},
	{Path: "rekit/tests/catalog.json", Purpose: "machine-readable smoke catalog"},
	{Path: "rekit/tests/README.md", Purpose: "smoke selection guide"},
	{Path: "CHANGELOG.md", Purpose: "release notes"},
}

func Build(repoRoot string) (Result, error) {
	repo, err := filepath.Abs(repoRoot)
	if err != nil {
		return Result{}, err
	}
	cat, err := loadCatalog(repo)
	if err != nil {
		return Result{}, err
	}
	packs, err := manifest.List(repo)
	if err != nil {
		return Result{}, err
	}
	check := Result{
		Command:               commandName,
		SchemaVersion:         1,
		IsMutation:            false,
		RepoRoot:              repo,
		Ready:                 true,
		Summary:               "release gate inventory ok",
		CIReleaseGate:         ciReleaseGate(repo),
		RecommendedMinimum:    catalogGateSteps(repo, cat.RecommendedMinimum),
		RequiredCommands:      requiredGateSteps(repo, requiredCommands, cat.RecommendedMinimum),
		Documents:             documentChecks(repo, requiredDocuments),
		Packs:                 packs,
		PowerShellDeprecation: powerShellDeprecation(repo),
		GoNativePublicSurface: goNativePublicSurface(repo),
		CaseShim:              caseshim.Inspect(repo),
		PublicDefaultDocs:     defaultdocs.Inspect(repo),
		HeavyToolGateActions:  heavyToolGateActions(packs),
		Boundaries:            append([]string{}, cat.GlobalBoundaries...),
		KnownGaps:             knownGaps(repo),
		Warnings:              []string{},
	}
	check.GateProfile = gateProfile(check.RecommendedMinimum)
	if crossWarnings := goNativePublicSurfaceCrossWarnings(check.GoNativePublicSurface, check.PowerShellDeprecation.PublicFacade); len(crossWarnings) > 0 {
		check.GoNativePublicSurface.Warnings = append(check.GoNativePublicSurface.Warnings, crossWarnings...)
		check.GoNativePublicSurface.Ready = false
		check.GoNativePublicSurface.Summary = "Go-native public command surface inventory has warnings"
	}
	if !check.GateProfile.Ready {
		check.Ready = false
		check.Warnings = append(check.Warnings, "release gate profile has unresolved commands or missing repo-local scripts")
	}
	for _, step := range check.RequiredCommands {
		if !step.InCatalog {
			check.Ready = false
			check.Warnings = append(check.Warnings, fmt.Sprintf("required command missing from catalog recommendedMinimum: %s", step.Command))
		}
	}
	for _, doc := range check.Documents {
		if !doc.Present {
			check.Ready = false
			check.Warnings = append(check.Warnings, fmt.Sprintf("required release document missing: %s", doc.Path))
		}
	}
	for _, pack := range check.Packs {
		if !pack.SchemaValid {
			check.Ready = false
			check.Warnings = append(check.Warnings, fmt.Sprintf("pack manifest invalid: %s: %s", pack.ID, pack.Error))
		}
	}
	if len(check.HeavyToolGateActions) == 0 {
		check.Ready = false
		check.Warnings = append(check.Warnings, "no heavy-tool gate actions declared by pack manifests")
	}
	if !check.CIReleaseGate.Ready {
		check.Ready = false
		check.Warnings = append(check.Warnings, check.CIReleaseGate.Warnings...)
	}
	if !check.PowerShellDeprecation.Ready {
		check.Ready = false
		check.Warnings = append(check.Warnings, check.PowerShellDeprecation.Warnings...)
	}
	if !check.GoNativePublicSurface.Ready {
		check.Ready = false
		check.Warnings = append(check.Warnings, check.GoNativePublicSurface.Warnings...)
	}
	if !check.CaseShim.Ready {
		check.Ready = false
		check.Warnings = append(check.Warnings, check.CaseShim.Warnings...)
	}
	if !check.PublicDefaultDocs.Ready {
		check.Ready = false
		check.Warnings = append(check.Warnings, check.PublicDefaultDocs.Warnings...)
	}
	if !check.Ready {
		check.Summary = "release gate inventory has warnings"
	}
	check.ReleaseHandoff = releaseHandoff(repo, check)
	if !check.ReleaseHandoff.Ready {
		check.Ready = false
		check.Warnings = append(check.Warnings, check.ReleaseHandoff.Warnings...)
	}
	if !check.Ready {
		check.Summary = "release gate inventory has warnings"
	}
	return check, nil
}

func loadCatalog(repo string) (catalog, error) {
	data, err := os.ReadFile(filepath.Join(repo, "rekit", "tests", "catalog.json"))
	if err != nil {
		return catalog{}, err
	}
	var cat catalog
	if err := json.Unmarshal(data, &cat); err != nil {
		return catalog{}, err
	}
	return cat, nil
}

func heavyToolGateActions(packs []manifest.PackSummary) []string {
	seen := map[string]bool{}
	for _, pack := range packs {
		for _, action := range pack.HeavyToolGateActions {
			action = strings.ToLower(strings.TrimSpace(action))
			if action != "" {
				seen[action] = true
			}
		}
	}
	actions := make([]string, 0, len(seen))
	for action := range seen {
		actions = append(actions, action)
	}
	sort.Strings(actions)
	return actions
}

func catalogGateSteps(repo string, commands []string) []GateStep {
	steps := make([]GateStep, 0, len(commands))
	for _, command := range commands {
		steps = append(steps, gateStep(repo, command, "catalog", true, true))
	}
	return steps
}

func requiredGateSteps(repo string, commands, catalogCommands []string) []GateStep {
	catalogSet := map[string]bool{}
	for _, command := range catalogCommands {
		catalogSet[normalizeCommand(command)] = true
	}
	steps := make([]GateStep, 0, len(commands))
	for _, command := range commands {
		inCatalog := catalogSet[normalizeCommand(command)]
		steps = append(steps, gateStep(repo, command, "release-check", true, inCatalog))
	}
	return steps
}

func gateProfile(steps []GateStep) GateProfile {
	profile := GateProfile{
		Name:               "local-ci-minimum",
		Description:        "Go-owned local/CI release gate profile; enumerates required checks and resolves repo-local scripts without executing them.",
		DefaultFor:         []string{"local", "ci"},
		Ready:              true,
		StepCount:          len(steps),
		LargeMatrixDefault: false,
		Steps:              append([]GateStep{}, steps...),
	}
	for _, step := range steps {
		if !step.Resolved {
			profile.Ready = false
			break
		}
	}
	return profile
}

func gateStep(repo, command, source string, required, inCatalog bool) GateStep {
	kind, repoPath := resolveGateCommand(command)
	present := true
	if repoPath != "" {
		_, err := os.Stat(filepath.Join(repo, filepath.FromSlash(repoPath)))
		present = err == nil
	}
	return GateStep{
		Command:   command,
		Source:    source,
		Kind:      kind,
		RepoPath:  repoPath,
		Present:   present,
		Required:  required,
		InCatalog: inCatalog,
		Resolved:  inCatalog && present,
	}
}

func resolveGateCommand(command string) (string, string) {
	normalized := normalizeCommand(command)
	fields := strings.Fields(normalized)
	if len(fields) == 0 {
		return "unknown", ""
	}
	first := strings.TrimPrefix(strings.ReplaceAll(fields[0], "\\", "/"), "./")
	if strings.EqualFold(first, "go") {
		if len(fields) >= 3 && strings.EqualFold(fields[1], "run") && strings.HasPrefix(fields[2], "./") {
			return "go-run", strings.TrimPrefix(strings.ReplaceAll(fields[2], "\\", "/"), "./")
		}
		return "go-check", ""
	}
	if strings.EqualFold(first, "git") {
		return "git-check", ""
	}
	if strings.HasSuffix(strings.ToLower(first), ".ps1") {
		repoPath := first
		if !strings.Contains(repoPath, "/") {
			repoPath = path.Join("rekit", "tests", repoPath)
		}
		if strings.EqualFold(repoPath, "rekit/rekit.ps1") {
			return "powershell-facade", repoPath
		}
		return "powershell-smoke", repoPath
	}
	return "external", ""
}

func documentChecks(repo string, docs []DocumentCheck) []DocumentCheck {
	checks := make([]DocumentCheck, 0, len(docs))
	for _, doc := range docs {
		_, err := os.Stat(filepath.Join(repo, filepath.FromSlash(doc.Path)))
		doc.Present = err == nil
		checks = append(checks, doc)
	}
	return checks
}

func knownGaps(repo string) []string {
	data, err := os.ReadFile(filepath.Join(repo, "docs", "release-readiness.md"))
	if err != nil {
		return nil
	}
	return markdownBulletsInSection(string(data), "## Known gaps")
}

func markdownBulletsInSection(text, heading string) []string {
	lines := strings.Split(strings.ReplaceAll(text, "\r\n", "\n"), "\n")
	inSection := false
	var bullets []string
	for _, line := range lines {
		trimmed := strings.TrimSpace(line)
		if trimmed == heading {
			inSection = true
			continue
		}
		if inSection && strings.HasPrefix(trimmed, "## ") {
			break
		}
		if inSection && strings.HasPrefix(trimmed, "- ") {
			bullets = append(bullets, strings.TrimSpace(strings.TrimPrefix(trimmed, "- ")))
		}
	}
	return bullets
}

func normalizeCommand(command string) string {
	command = strings.TrimSpace(strings.ReplaceAll(command, "\\", "/"))
	command = strings.TrimPrefix(command, "./")
	fields := strings.Fields(command)
	return strings.Join(fields, " ")
}
