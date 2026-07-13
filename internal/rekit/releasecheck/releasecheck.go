package releasecheck

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/shuiyu486/re-context-kits/internal/rekit/manifest"
)

const commandName = "release-check"

type Result struct {
	Command            string                 `json:"command"`
	SchemaVersion      int                    `json:"schemaVersion"`
	IsMutation         bool                   `json:"isMutation"`
	RepoRoot           string                 `json:"repoRoot"`
	Ready              bool                   `json:"ready"`
	Summary            string                 `json:"summary"`
	RecommendedMinimum []GateStep             `json:"recommendedMinimum"`
	RequiredCommands   []GateStep             `json:"requiredCommands"`
	Documents          []DocumentCheck        `json:"documents"`
	Packs              []manifest.PackSummary `json:"packs"`
	Boundaries         []string               `json:"boundaries"`
	KnownGaps          []string               `json:"knownGaps"`
	Warnings           []string               `json:"warnings"`
}

type GateStep struct {
	Command   string `json:"command"`
	Source    string `json:"source"`
	Required  bool   `json:"required"`
	InCatalog bool   `json:"inCatalog"`
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
	"go test ./...",
	"go vet ./...",
	"rekit/rekit.ps1 -Command doctor",
	"facade-smoke.ps1",
	"git diff --check",
}

var requiredDocuments = []DocumentCheck{
	{Path: "docs/release-readiness.md", Purpose: "release gate and known gaps"},
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
		Command:            commandName,
		SchemaVersion:      1,
		IsMutation:         false,
		RepoRoot:           repo,
		Ready:              true,
		Summary:            "release gate inventory ok",
		RecommendedMinimum: catalogGateSteps(cat.RecommendedMinimum),
		RequiredCommands:   requiredGateSteps(requiredCommands, cat.RecommendedMinimum),
		Documents:          documentChecks(repo, requiredDocuments),
		Packs:              packs,
		Boundaries:         append([]string{}, cat.GlobalBoundaries...),
		KnownGaps:          knownGaps(repo),
		Warnings:           []string{},
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

func catalogGateSteps(commands []string) []GateStep {
	steps := make([]GateStep, 0, len(commands))
	for _, command := range commands {
		steps = append(steps, GateStep{Command: command, Source: "catalog", Required: true, InCatalog: true})
	}
	return steps
}

func requiredGateSteps(commands, catalogCommands []string) []GateStep {
	catalogSet := map[string]bool{}
	for _, command := range catalogCommands {
		catalogSet[normalizeCommand(command)] = true
	}
	steps := make([]GateStep, 0, len(commands))
	for _, command := range commands {
		steps = append(steps, GateStep{Command: command, Source: "release-check", Required: true, InCatalog: catalogSet[normalizeCommand(command)]})
	}
	return steps
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
