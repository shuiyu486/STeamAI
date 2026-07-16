package releasecheck

import (
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"slices"
	"sort"
	"strings"
)

type PublicFacadeRemoval struct {
	Ready         bool                              `json:"ready"`
	Summary       string                            `json:"summary"`
	Prerequisites []PublicFacadeRemovalPrerequisite `json:"prerequisites"`
	RemovalPlan   PublicFacadeRemovalPlan           `json:"removalPlan"`
	RemovalImpact PublicFacadeRemovalImpact         `json:"removalImpact"`
	Warnings      []string                          `json:"warnings"`
}

type PublicFacadeRemovalPrerequisite struct {
	Name    string `json:"name"`
	Ready   bool   `json:"ready"`
	Summary string `json:"summary"`
}

type PublicFacadeRemovalPlan struct {
	Ready           bool                            `json:"ready"`
	Summary         string                          `json:"summary"`
	Document        string                          `json:"document"`
	RequiredPhrases []PublicFacadeRemovalPlanPhrase `json:"requiredPhrases"`
	Warnings        []string                        `json:"warnings"`
}

type PublicFacadeRemovalPlanPhrase struct {
	Name    string `json:"name"`
	Phrase  string `json:"phrase"`
	Present bool   `json:"present"`
}

type PublicFacadeRemovalImpact struct {
	Ready                  bool                                 `json:"ready"`
	Summary                string                               `json:"summary"`
	FacadePath             string                               `json:"facadePath"`
	FacadePresent          bool                                 `json:"facadePresent"`
	References             []PublicFacadeRemovalImpactReference `json:"references"`
	ReferenceCategories    []PublicFacadeRemovalImpactCategory  `json:"referenceCategories"`
	WorkItems              []PublicFacadeRemovalImpactWorkItem  `json:"workItems"`
	UnclassifiedReferences []PublicFacadeRemovalImpactReference `json:"unclassifiedReferences"`
	Warnings               []string                             `json:"warnings"`
}

type PublicFacadeRemovalImpactReference struct {
	Path                string `json:"path"`
	Category            string `json:"category"`
	MentionsFacade      bool   `json:"mentionsFacade"`
	MentionsFacadeSmoke bool   `json:"mentionsFacadeSmoke"`
}

type PublicFacadeRemovalImpactCategory struct {
	Name  string   `json:"name"`
	Count int      `json:"count"`
	Paths []string `json:"paths"`
}

type PublicFacadeRemovalImpactWorkItem struct {
	Category           string   `json:"category"`
	Action             string   `json:"action"`
	Required           bool     `json:"required"`
	Count              int      `json:"count"`
	Paths              []string `json:"paths"`
	ValidationCommands []string `json:"validationCommands"`
}

func publicFacadeRemovalHandoffDetails(inventory PublicFacadeRemoval) []string {
	details := make([]string, 0, len(inventory.Prerequisites)+3)
	details = append(details, fmt.Sprintf("ready=%t prerequisites=%d", inventory.Ready, len(inventory.Prerequisites)))
	details = append(details, fmt.Sprintf("removalPlan=%t planChecks=%d", inventory.RemovalPlan.Ready, len(inventory.RemovalPlan.RequiredPhrases)))
	details = append(details, fmt.Sprintf("removalImpact=%t impactReferences=%d impactCategories=%d workItems=%d validationCommands=%d unclassified=%d", inventory.RemovalImpact.Ready, len(inventory.RemovalImpact.References), len(inventory.RemovalImpact.ReferenceCategories), len(inventory.RemovalImpact.WorkItems), publicFacadeRemovalImpactValidationCommandCount(inventory.RemovalImpact.WorkItems), len(inventory.RemovalImpact.UnclassifiedReferences)))
	for _, prerequisite := range inventory.Prerequisites {
		details = append(details, fmt.Sprintf("%s ready=%t %s", prerequisite.Name, prerequisite.Ready, prerequisite.Summary))
	}
	return details
}

func publicFacadeRemovalInventory(repo string, powerShell PowerShellDeprecation, goSurface GoNativePublicSurface) PublicFacadeRemoval {
	removalPlan := publicFacadeRemovalPlan(repo)
	removalImpact := publicFacadeRemovalImpact(repo)
	inventory := PublicFacadeRemoval{
		Ready:   true,
		Summary: "public facade removal prerequisites ok",
		Prerequisites: []PublicFacadeRemovalPrerequisite{
			{
				Name:    "public-facade-retained-boundary",
				Ready:   powerShell.PublicFacade.Ready && powerShell.PublicFacade.Present && powerShell.PublicFacade.Retained && powerShell.PublicFacade.MigrationBoundaryDocumented && powerShell.PublicFacade.RemovalBoundaryDocumented,
				Summary: fmt.Sprintf("publicFacadeReady=%t present=%t retained=%t migrationBoundary=%t removalBoundary=%t", powerShell.PublicFacade.Ready, powerShell.PublicFacade.Present, powerShell.PublicFacade.Retained, powerShell.PublicFacade.MigrationBoundaryDocumented, powerShell.PublicFacade.RemovalBoundaryDocumented),
			},
			{
				Name:    "facade-command-surface-no-fallback",
				Ready:   len(powerShell.PublicFacade.CommandSurface) > 0 && slices.Equal(powerShell.PublicFacade.CommandSurface, powerShell.PublicFacade.GoDefaultCommands) && slices.Equal(powerShell.PublicFacade.CommandSurface, powerShell.PublicFacade.NoFallbackCommands),
				Summary: fmt.Sprintf("facadeCommands=%d goDefault=%d noFallback=%d", len(powerShell.PublicFacade.CommandSurface), len(powerShell.PublicFacade.GoDefaultCommands), len(powerShell.PublicFacade.NoFallbackCommands)),
			},
			{
				Name:    "go-native-public-surface",
				Ready:   goSurface.Ready && goSurface.FacadeRemovalReady,
				Summary: fmt.Sprintf("goNativeReady=%t facadeRemovalReady=%t prerequisites=%d", goSurface.Ready, goSurface.FacadeRemovalReady, len(goSurface.FacadeRemovalPrerequisites)),
			},
			{
				Name:    "legacy-runtime-detached",
				Ready:   powerShell.FacadeRuntime.Ready && !powerShell.FacadeRuntime.LegacyModuleImportsPresent && !powerShell.FacadeRuntime.CommandDispatcherPresent && powerShell.FacadeRuntime.NoFallbackGuardPresent && powerShell.FacadeRuntime.GoDelegationPresent && powerShell.FacadeRuntime.RetiredDispatcherError,
				Summary: fmt.Sprintf("facadeRuntimeReady=%t legacyImports=%t dispatcher=%t noFallbackGuard=%t goDelegation=%t retiredError=%t", powerShell.FacadeRuntime.Ready, powerShell.FacadeRuntime.LegacyModuleImportsPresent, powerShell.FacadeRuntime.CommandDispatcherPresent, powerShell.FacadeRuntime.NoFallbackGuardPresent, powerShell.FacadeRuntime.GoDelegationPresent, powerShell.FacadeRuntime.RetiredDispatcherError),
			},
			{
				Name:    "legacy-module-removal-settled",
				Ready:   powerShell.ModuleRemoval.Ready && len(powerShell.ModuleRemoval.CandidateModules) == 0 && len(powerShell.ModuleRemoval.FacadeRuntimeDependencies) == 0 && len(powerShell.ModuleRemoval.UndocumentedModules) == 0,
				Summary: fmt.Sprintf("moduleRemovalReady=%t candidates=%d retired=%d facadeDeps=%d undocumented=%d", powerShell.ModuleRemoval.Ready, len(powerShell.ModuleRemoval.CandidateModules), len(powerShell.ModuleRemoval.RetiredModules), len(powerShell.ModuleRemoval.FacadeRuntimeDependencies), len(powerShell.ModuleRemoval.UndocumentedModules)),
			},
			{
				Name:    "module-reference-blockers-clear",
				Ready:   powerShell.ModuleReferences.Ready && len(powerShell.ModuleReferences.ActiveTestDependencies) == 0 && len(powerShell.ModuleReferences.CompatibilityFixtures) == 0 && len(powerShell.ModuleReferences.RemovalBlockers) == 0 && len(powerShell.ModuleReferences.UnclassifiedReferences) == 0,
				Summary: fmt.Sprintf("moduleReferencesReady=%t activeTests=%d fixtures=%d blockers=%d unclassified=%d", powerShell.ModuleReferences.Ready, len(powerShell.ModuleReferences.ActiveTestDependencies), len(powerShell.ModuleReferences.CompatibilityFixtures), len(powerShell.ModuleReferences.RemovalBlockers), len(powerShell.ModuleReferences.UnclassifiedReferences)),
			},
			{
				Name:    "removal-plan-documented",
				Ready:   removalPlan.Ready,
				Summary: fmt.Sprintf("removalPlanReady=%t checks=%d", removalPlan.Ready, len(removalPlan.RequiredPhrases)),
			},
			{
				Name:    "removal-impact-inventoried",
				Ready:   removalImpact.Ready,
				Summary: fmt.Sprintf("removalImpactReady=%t references=%d categories=%d workItems=%d validationCommands=%d unclassified=%d", removalImpact.Ready, len(removalImpact.References), len(removalImpact.ReferenceCategories), len(removalImpact.WorkItems), publicFacadeRemovalImpactValidationCommandCount(removalImpact.WorkItems), len(removalImpact.UnclassifiedReferences)),
			},
		},
		RemovalPlan:   removalPlan,
		RemovalImpact: removalImpact,
		Warnings:      append(append([]string{}, removalPlan.Warnings...), removalImpact.Warnings...),
	}
	for _, prerequisite := range inventory.Prerequisites {
		if !prerequisite.Ready {
			inventory.Warnings = append(inventory.Warnings, fmt.Sprintf("public facade removal prerequisite is not ready: %s", prerequisite.Name))
		}
	}
	if len(inventory.Warnings) > 0 {
		inventory.Ready = false
		inventory.Summary = "public facade removal prerequisites have warnings"
	}
	return inventory
}

func publicFacadeRemovalPlan(repo string) PublicFacadeRemovalPlan {
	const document = "docs/powershell-deprecation.md"
	plan := PublicFacadeRemovalPlan{
		Ready:    true,
		Summary:  "public facade removal plan documented",
		Document: document,
		RequiredPhrases: []PublicFacadeRemovalPlanPhrase{
			{Name: "section", Phrase: "## Public façade removal plan inventory"},
			{Name: "independent-removal-batch", Phrase: "独立 removal batch"},
			{Name: "alternative-entrypoint", Phrase: "替代入口"},
			{Name: "recovery-plan", Phrase: "恢复计划"},
			{Name: "validation-commands", Phrase: "验证命令"},
			{Name: "documentation-sync", Phrase: "文档同步"},
			{Name: "release-notes", Phrase: "CHANGELOG"},
			{Name: "no-powershell-runtime-logic", Phrase: "不新增 PowerShell runtime logic"},
			{Name: "no-heavy-tool-authority", Phrase: "actual heavy-tool、authority/confirmed"},
		},
		Warnings: []string{},
	}
	data, err := os.ReadFile(filepath.Join(repo, filepath.FromSlash(document)))
	if err != nil {
		plan.Ready = false
		plan.Summary = "public facade removal plan missing"
		plan.Warnings = append(plan.Warnings, err.Error())
		return plan
	}
	text := string(data)
	for i := range plan.RequiredPhrases {
		plan.RequiredPhrases[i].Present = strings.Contains(text, plan.RequiredPhrases[i].Phrase)
		if !plan.RequiredPhrases[i].Present {
			plan.Warnings = append(plan.Warnings, fmt.Sprintf("public facade removal plan missing required phrase %s: %s", plan.RequiredPhrases[i].Name, plan.RequiredPhrases[i].Phrase))
		}
	}
	if len(plan.Warnings) > 0 {
		plan.Ready = false
		plan.Summary = "public facade removal plan has warnings"
	}
	return plan
}

func publicFacadeRemovalImpact(repo string) PublicFacadeRemovalImpact {
	const facadePath = "rekit/rekit.ps1"
	impact := PublicFacadeRemovalImpact{
		Ready:                  true,
		Summary:                "public facade removal impact inventory ok",
		FacadePath:             facadePath,
		References:             []PublicFacadeRemovalImpactReference{},
		ReferenceCategories:    []PublicFacadeRemovalImpactCategory{},
		WorkItems:              []PublicFacadeRemovalImpactWorkItem{},
		UnclassifiedReferences: []PublicFacadeRemovalImpactReference{},
		Warnings:               []string{},
	}
	if _, err := os.Stat(filepath.Join(repo, filepath.FromSlash(facadePath))); err == nil {
		impact.FacadePresent = true
	} else {
		impact.Warnings = append(impact.Warnings, fmt.Sprintf("public facade removal impact inventory cannot find facade path: %s", facadePath))
	}
	references, warnings := publicFacadeRemovalReferences(repo)
	impact.References = references
	impact.Warnings = append(impact.Warnings, warnings...)
	impact.ReferenceCategories = publicFacadeRemovalImpactCategories(references)
	impact.WorkItems = publicFacadeRemovalImpactWorkItems(impact.ReferenceCategories)
	for _, reference := range references {
		if reference.Category == "unclassified" {
			impact.UnclassifiedReferences = append(impact.UnclassifiedReferences, reference)
		}
	}
	if len(impact.References) == 0 {
		impact.Warnings = append(impact.Warnings, "public facade removal impact inventory found no facade references")
	}
	if len(impact.WorkItems) != len(impact.ReferenceCategories) {
		impact.Warnings = append(impact.Warnings, "public facade removal impact work items do not cover every reference category")
	}
	for _, workItem := range impact.WorkItems {
		if strings.TrimSpace(workItem.Action) == "" {
			impact.Warnings = append(impact.Warnings, fmt.Sprintf("public facade removal impact work item missing action: %s", workItem.Category))
		}
		if len(workItem.ValidationCommands) == 0 {
			impact.Warnings = append(impact.Warnings, fmt.Sprintf("public facade removal impact work item missing validation commands: %s", workItem.Category))
		}
		for _, command := range workItem.ValidationCommands {
			if strings.TrimSpace(command) == "" {
				impact.Warnings = append(impact.Warnings, fmt.Sprintf("public facade removal impact work item has empty validation command: %s", workItem.Category))
			}
		}
		for _, command := range publicFacadeRemovalImpactValidationCommands() {
			if !slices.Contains(workItem.ValidationCommands, command) {
				impact.Warnings = append(impact.Warnings, fmt.Sprintf("public facade removal impact work item missing validation command %q: %s", command, workItem.Category))
			}
		}
	}
	for _, reference := range impact.UnclassifiedReferences {
		impact.Warnings = append(impact.Warnings, fmt.Sprintf("public facade removal impact reference is unclassified: %s", reference.Path))
	}
	if len(impact.Warnings) > 0 {
		impact.Ready = false
		impact.Summary = "public facade removal impact inventory has warnings"
	}
	return impact
}

func publicFacadeRemovalReferences(repo string) ([]PublicFacadeRemovalImpactReference, []string) {
	references := []PublicFacadeRemovalImpactReference{}
	warnings := []string{}
	err := filepath.WalkDir(repo, func(path string, entry fs.DirEntry, err error) error {
		if err != nil {
			warnings = append(warnings, err.Error())
			return nil
		}
		name := entry.Name()
		if entry.IsDir() {
			if name == ".git" || name == ".codegraph" || name == ".rekit" || name == "node_modules" || name == "vendor" {
				return filepath.SkipDir
			}
			return nil
		}
		rel, err := filepath.Rel(repo, path)
		if err != nil {
			warnings = append(warnings, err.Error())
			return nil
		}
		rel = filepath.ToSlash(rel)
		isFacadeEntrypoint := rel == "rekit/rekit.ps1"
		data, err := os.ReadFile(path)
		if err != nil {
			warnings = append(warnings, err.Error())
			return nil
		}
		text := string(data)
		mentionsFacade := strings.Contains(text, "rekit/rekit.ps1") || strings.Contains(text, `rekit\rekit.ps1`) || strings.Contains(text, "rekit.ps1")
		mentionsFacadeSmoke := strings.Contains(text, "facade-smoke.ps1")
		if !isFacadeEntrypoint && !mentionsFacade && !mentionsFacadeSmoke {
			return nil
		}
		references = append(references, PublicFacadeRemovalImpactReference{
			Path:                rel,
			Category:            publicFacadeRemovalImpactCategory(rel),
			MentionsFacade:      mentionsFacade,
			MentionsFacadeSmoke: mentionsFacadeSmoke,
		})
		return nil
	})
	if err != nil {
		warnings = append(warnings, err.Error())
	}
	sort.Slice(references, func(i, j int) bool {
		if references[i].Category == references[j].Category {
			return references[i].Path < references[j].Path
		}
		return references[i].Category < references[j].Category
	})
	return references, warnings
}

func publicFacadeRemovalImpactCategory(path string) string {
	switch {
	case path == "rekit/rekit.ps1":
		return "public-facade-entrypoint"
	case path == "rekit/tests/facade-smoke.ps1":
		return "facade-compatibility-smoke"
	case strings.HasPrefix(path, "rekit/tests/") && strings.HasSuffix(path, ".ps1"):
		return "facade-dependent-smoke"
	case strings.HasPrefix(path, "rekit/tests/"):
		return "smoke-catalog-doc"
	case strings.HasPrefix(path, "packs/") && strings.HasSuffix(path, ".ps1"):
		return "pack-wrapper-compatibility"
	case path == "README.md" || path == "CLAUDE.md" || strings.HasPrefix(path, ".claude/skills/"):
		return "public-default-doc"
	case path == "CHANGELOG.md" || strings.HasPrefix(path, "docs/"):
		return "roadmap-and-history-doc"
	case strings.HasPrefix(path, "internal/rekit/"):
		return "release-inventory-and-tests"
	default:
		return "unclassified"
	}
}

func publicFacadeRemovalImpactCategories(references []PublicFacadeRemovalImpactReference) []PublicFacadeRemovalImpactCategory {
	pathsByCategory := map[string][]string{}
	for _, reference := range references {
		pathsByCategory[reference.Category] = append(pathsByCategory[reference.Category], reference.Path)
	}
	categories := make([]PublicFacadeRemovalImpactCategory, 0, len(pathsByCategory))
	for name, paths := range pathsByCategory {
		sort.Strings(paths)
		categories = append(categories, PublicFacadeRemovalImpactCategory{Name: name, Count: len(paths), Paths: paths})
	}
	sort.Slice(categories, func(i, j int) bool { return categories[i].Name < categories[j].Name })
	return categories
}

func publicFacadeRemovalImpactWorkItems(categories []PublicFacadeRemovalImpactCategory) []PublicFacadeRemovalImpactWorkItem {
	workItems := make([]PublicFacadeRemovalImpactWorkItem, 0, len(categories))
	for _, category := range categories {
		workItems = append(workItems, PublicFacadeRemovalImpactWorkItem{
			Category:           category.Name,
			Action:             publicFacadeRemovalImpactAction(category.Name),
			Required:           true,
			Count:              category.Count,
			Paths:              append([]string{}, category.Paths...),
			ValidationCommands: publicFacadeRemovalImpactValidationCommands(),
		})
	}
	return workItems
}

func publicFacadeRemovalImpactValidationCommandCount(workItems []PublicFacadeRemovalImpactWorkItem) int {
	count := 0
	for _, workItem := range workItems {
		count += len(workItem.ValidationCommands)
	}
	return count
}

func publicFacadeRemovalImpactAction(category string) string {
	switch category {
	case "public-facade-entrypoint":
		return "remove or replace the public facade entrypoint only in the independent removal batch"
	case "facade-compatibility-smoke":
		return "retire or rewrite facade compatibility smoke as Go-native/source-invariant coverage"
	case "facade-dependent-smoke":
		return "rewrite facade-dependent smoke invocations or explicitly retire compatibility coverage"
	case "smoke-catalog-doc":
		return "update smoke catalog and tests guide rows that name facade compatibility scripts"
	case "pack-wrapper-compatibility":
		return "update pack wrapper compatibility scripts that reference the public facade"
	case "public-default-doc":
		return "sync public default docs away from public facade file references"
	case "roadmap-and-history-doc":
		return "update roadmap/history docs while preserving historical context without defaulting to the facade"
	case "release-inventory-and-tests":
		return "update release inventory and tests that assert facade removal readiness counts"
	case "unclassified":
		return "classify or remove unexpected facade references before deleting the facade"
	default:
		return "review and classify facade removal impact before deleting the facade"
	}
}

func publicFacadeRemovalImpactValidationCommands() []string {
	return []string{
		"go run ./cmd/rekit -- -Command release-check -Format json",
		"go run ./cmd/rekit -- -Command release-check",
		"go run ./cmd/rekit -- -Command status",
		"go run ./cmd/rekit -- -Command packs",
		"go run ./cmd/rekit -- -Command doctor",
		"go test ./...",
		"go vet ./...",
		"git diff --check",
	}
}
