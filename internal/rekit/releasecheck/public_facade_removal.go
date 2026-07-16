package releasecheck

import (
	"fmt"
	"os"
	"path/filepath"
	"slices"
	"strings"
)

type PublicFacadeRemoval struct {
	Ready         bool                              `json:"ready"`
	Summary       string                            `json:"summary"`
	Prerequisites []PublicFacadeRemovalPrerequisite `json:"prerequisites"`
	RemovalPlan   PublicFacadeRemovalPlan           `json:"removalPlan"`
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

func publicFacadeRemovalHandoffDetails(inventory PublicFacadeRemoval) []string {
	details := make([]string, 0, len(inventory.Prerequisites)+2)
	details = append(details, fmt.Sprintf("ready=%t prerequisites=%d", inventory.Ready, len(inventory.Prerequisites)))
	details = append(details, fmt.Sprintf("removalPlan=%t planChecks=%d", inventory.RemovalPlan.Ready, len(inventory.RemovalPlan.RequiredPhrases)))
	for _, prerequisite := range inventory.Prerequisites {
		details = append(details, fmt.Sprintf("%s ready=%t %s", prerequisite.Name, prerequisite.Ready, prerequisite.Summary))
	}
	return details
}

func publicFacadeRemovalInventory(repo string, powerShell PowerShellDeprecation, goSurface GoNativePublicSurface) PublicFacadeRemoval {
	removalPlan := publicFacadeRemovalPlan(repo)
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
		},
		RemovalPlan: removalPlan,
		Warnings:    append([]string{}, removalPlan.Warnings...),
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
