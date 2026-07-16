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
	Ready                  bool                                       `json:"ready"`
	Summary                string                                     `json:"summary"`
	Document               string                                     `json:"document"`
	RequiredPhrases        []PublicFacadeRemovalPlanPhrase            `json:"requiredPhrases"`
	ReplacementEntrypoints []PublicFacadeRemovalReplacementEntrypoint `json:"replacementEntrypoints"`
	DeletionGates          []PublicFacadeRemovalDeletionGate          `json:"deletionGates"`
	ExecutionSteps         []PublicFacadeRemovalExecutionStep         `json:"executionSteps"`
	BoundaryChecks         []PublicFacadeRemovalPlanBoundaryCheck     `json:"boundaryChecks"`
	RecoverySteps          []PublicFacadeRemovalRecoveryStep          `json:"recoverySteps"`
	DocumentationTargets   []PublicFacadeRemovalDocumentationTarget   `json:"documentationTargets"`
	Warnings               []string                                   `json:"warnings"`
}

type PublicFacadeRemovalPlanPhrase struct {
	Name    string `json:"name"`
	Phrase  string `json:"phrase"`
	Present bool   `json:"present"`
}

type PublicFacadeRemovalReplacementEntrypoint struct {
	Name               string   `json:"name"`
	Entrypoint         string   `json:"entrypoint"`
	Audience           string   `json:"audience"`
	Purpose            string   `json:"purpose"`
	Required           bool     `json:"required"`
	GoNativeBacked     bool     `json:"goNativeBacked"`
	UserFacing         bool     `json:"userFacing"`
	ValidationCommands []string `json:"validationCommands"`
}

type PublicFacadeRemovalDeletionGate struct {
	Name                  string   `json:"name"`
	Gate                  string   `json:"gate"`
	Required              bool     `json:"required"`
	BlocksRemoval         bool     `json:"blocksRemoval"`
	BlockedExecutionSteps []string `json:"blockedExecutionSteps"`
	InputInventory        []string `json:"inputInventory"`
	ExitCriteria          []string `json:"exitCriteria"`
	FailureSignals        []string `json:"failureSignals"`
	EscalationTriggers    []string `json:"escalationTriggers"`
	VerificationArtifacts []string `json:"verificationArtifacts"`
	RemediationActions    []string `json:"remediationActions"`
	ValidationCommands    []string `json:"validationCommands"`
}

type PublicFacadeRemovalExecutionStep struct {
	Name                    string   `json:"name"`
	Action                  string   `json:"action"`
	Required                bool     `json:"required"`
	DependsOn               []string `json:"dependsOn"`
	InputInventory          []string `json:"inputInventory"`
	OutputArtifacts         []string `json:"outputArtifacts"`
	ValidationCommands      []string `json:"validationCommands"`
	AllowsPowerShellRuntime bool     `json:"allowsPowerShellRuntime"`
	AllowsExternalEffects   bool     `json:"allowsExternalEffects"`
}

type PublicFacadeRemovalPlanBoundaryCheck struct {
	Name               string   `json:"name"`
	Boundary           string   `json:"boundary"`
	Required           bool     `json:"required"`
	Preserved          bool     `json:"preserved"`
	Evidence           []string `json:"evidence"`
	ValidationCommands []string `json:"validationCommands"`
}

type PublicFacadeRemovalRecoveryStep struct {
	Name               string   `json:"name"`
	Action             string   `json:"action"`
	Required           bool     `json:"required"`
	Paths              []string `json:"paths"`
	ValidationCommands []string `json:"validationCommands"`
}

type PublicFacadeRemovalDocumentationTarget struct {
	Path               string   `json:"path"`
	Purpose            string   `json:"purpose"`
	Action             string   `json:"action"`
	Required           bool     `json:"required"`
	ValidationCommands []string `json:"validationCommands"`
}

type PublicFacadeRemovalImpact struct {
	Ready                  bool                                      `json:"ready"`
	Summary                string                                    `json:"summary"`
	FacadePath             string                                    `json:"facadePath"`
	FacadePresent          bool                                      `json:"facadePresent"`
	References             []PublicFacadeRemovalImpactReference      `json:"references"`
	ReferenceCategories    []PublicFacadeRemovalImpactCategory       `json:"referenceCategories"`
	WorkItems              []PublicFacadeRemovalImpactWorkItem       `json:"workItems"`
	MigrationTargets       []PublicFacadeRemovalMigrationTarget      `json:"migrationTargets"`
	SmokeMigrationTargets  []PublicFacadeRemovalSmokeMigrationTarget `json:"smokeMigrationTargets"`
	UnclassifiedReferences []PublicFacadeRemovalImpactReference      `json:"unclassifiedReferences"`
	Warnings               []string                                  `json:"warnings"`
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

type PublicFacadeRemovalMigrationTarget struct {
	Path                      string   `json:"path"`
	Category                  string   `json:"category"`
	Action                    string   `json:"action"`
	Required                  bool     `json:"required"`
	GoNativePreferred         bool     `json:"goNativePreferred"`
	PreserveHistoricalContext bool     `json:"preserveHistoricalContext"`
	ValidationCommands        []string `json:"validationCommands"`
}

type PublicFacadeRemovalSmokeMigrationTarget struct {
	Path                   string   `json:"path"`
	Category               string   `json:"category"`
	Action                 string   `json:"action"`
	Required               bool     `json:"required"`
	GoNativePreferred      bool     `json:"goNativePreferred"`
	AllowFacadeCompat      bool     `json:"allowFacadeCompat"`
	ValidationCommands     []string `json:"validationCommands"`
	RetireFacadeAssertions bool     `json:"retireFacadeAssertions"`
}

func publicFacadeRemovalHandoffDetails(inventory PublicFacadeRemoval) []string {
	details := make([]string, 0, len(inventory.Prerequisites)+3)
	details = append(details, fmt.Sprintf("ready=%t prerequisites=%d", inventory.Ready, len(inventory.Prerequisites)))
	details = append(details, fmt.Sprintf("removalPlan=%t planChecks=%d replacementEntrypoints=%d replacementValidationCommands=%d deletionGates=%d deletionGateValidationCommands=%d deletionGateExitCriteria=%d deletionGateFailureSignals=%d deletionGateEscalationTriggers=%d deletionGateVerificationArtifacts=%d deletionGateBlockedExecutionSteps=%d deletionGateRemediationActions=%d executionSteps=%d executionValidationCommands=%d boundaryChecks=%d boundaryValidationCommands=%d recoverySteps=%d recoveryValidationCommands=%d documentationTargets=%d documentationValidationCommands=%d", inventory.RemovalPlan.Ready, len(inventory.RemovalPlan.RequiredPhrases), len(inventory.RemovalPlan.ReplacementEntrypoints), publicFacadeRemovalReplacementValidationCommandCount(inventory.RemovalPlan.ReplacementEntrypoints), len(inventory.RemovalPlan.DeletionGates), publicFacadeRemovalDeletionGateValidationCommandCount(inventory.RemovalPlan.DeletionGates), publicFacadeRemovalDeletionGateExitCriteriaCount(inventory.RemovalPlan.DeletionGates), publicFacadeRemovalDeletionGateFailureSignalCount(inventory.RemovalPlan.DeletionGates), publicFacadeRemovalDeletionGateEscalationTriggerCount(inventory.RemovalPlan.DeletionGates), publicFacadeRemovalDeletionGateVerificationArtifactCount(inventory.RemovalPlan.DeletionGates), publicFacadeRemovalDeletionGateBlockedExecutionStepCount(inventory.RemovalPlan.DeletionGates), publicFacadeRemovalDeletionGateRemediationActionCount(inventory.RemovalPlan.DeletionGates), len(inventory.RemovalPlan.ExecutionSteps), publicFacadeRemovalExecutionValidationCommandCount(inventory.RemovalPlan.ExecutionSteps), len(inventory.RemovalPlan.BoundaryChecks), publicFacadeRemovalBoundaryValidationCommandCount(inventory.RemovalPlan.BoundaryChecks), len(inventory.RemovalPlan.RecoverySteps), publicFacadeRemovalRecoveryValidationCommandCount(inventory.RemovalPlan.RecoverySteps), len(inventory.RemovalPlan.DocumentationTargets), publicFacadeRemovalDocumentationValidationCommandCount(inventory.RemovalPlan.DocumentationTargets)))
	details = append(details, fmt.Sprintf("removalImpact=%t impactReferences=%d impactCategories=%d workItems=%d validationCommands=%d migrationTargets=%d migrationValidationCommands=%d smokeMigrationTargets=%d smokeMigrationValidationCommands=%d unclassified=%d", inventory.RemovalImpact.Ready, len(inventory.RemovalImpact.References), len(inventory.RemovalImpact.ReferenceCategories), len(inventory.RemovalImpact.WorkItems), publicFacadeRemovalImpactValidationCommandCount(inventory.RemovalImpact.WorkItems), len(inventory.RemovalImpact.MigrationTargets), publicFacadeRemovalMigrationValidationCommandCount(inventory.RemovalImpact.MigrationTargets), len(inventory.RemovalImpact.SmokeMigrationTargets), publicFacadeRemovalSmokeMigrationValidationCommandCount(inventory.RemovalImpact.SmokeMigrationTargets), len(inventory.RemovalImpact.UnclassifiedReferences)))
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
				Summary: fmt.Sprintf("removalPlanReady=%t checks=%d replacementEntrypoints=%d replacementValidationCommands=%d deletionGates=%d deletionGateValidationCommands=%d deletionGateExitCriteria=%d deletionGateFailureSignals=%d deletionGateEscalationTriggers=%d deletionGateVerificationArtifacts=%d deletionGateBlockedExecutionSteps=%d deletionGateRemediationActions=%d executionSteps=%d executionValidationCommands=%d boundaryChecks=%d boundaryValidationCommands=%d recoverySteps=%d recoveryValidationCommands=%d documentationTargets=%d documentationValidationCommands=%d", removalPlan.Ready, len(removalPlan.RequiredPhrases), len(removalPlan.ReplacementEntrypoints), publicFacadeRemovalReplacementValidationCommandCount(removalPlan.ReplacementEntrypoints), len(removalPlan.DeletionGates), publicFacadeRemovalDeletionGateValidationCommandCount(removalPlan.DeletionGates), publicFacadeRemovalDeletionGateExitCriteriaCount(removalPlan.DeletionGates), publicFacadeRemovalDeletionGateFailureSignalCount(removalPlan.DeletionGates), publicFacadeRemovalDeletionGateEscalationTriggerCount(removalPlan.DeletionGates), publicFacadeRemovalDeletionGateVerificationArtifactCount(removalPlan.DeletionGates), publicFacadeRemovalDeletionGateBlockedExecutionStepCount(removalPlan.DeletionGates), publicFacadeRemovalDeletionGateRemediationActionCount(removalPlan.DeletionGates), len(removalPlan.ExecutionSteps), publicFacadeRemovalExecutionValidationCommandCount(removalPlan.ExecutionSteps), len(removalPlan.BoundaryChecks), publicFacadeRemovalBoundaryValidationCommandCount(removalPlan.BoundaryChecks), len(removalPlan.RecoverySteps), publicFacadeRemovalRecoveryValidationCommandCount(removalPlan.RecoverySteps), len(removalPlan.DocumentationTargets), publicFacadeRemovalDocumentationValidationCommandCount(removalPlan.DocumentationTargets)),
			},
			{
				Name:    "removal-impact-inventoried",
				Ready:   removalImpact.Ready,
				Summary: fmt.Sprintf("removalImpactReady=%t references=%d categories=%d workItems=%d validationCommands=%d migrationTargets=%d migrationValidationCommands=%d smokeMigrationTargets=%d smokeMigrationValidationCommands=%d unclassified=%d", removalImpact.Ready, len(removalImpact.References), len(removalImpact.ReferenceCategories), len(removalImpact.WorkItems), publicFacadeRemovalImpactValidationCommandCount(removalImpact.WorkItems), len(removalImpact.MigrationTargets), publicFacadeRemovalMigrationValidationCommandCount(removalImpact.MigrationTargets), len(removalImpact.SmokeMigrationTargets), publicFacadeRemovalSmokeMigrationValidationCommandCount(removalImpact.SmokeMigrationTargets), len(removalImpact.UnclassifiedReferences)),
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
		ReplacementEntrypoints: publicFacadeRemovalReplacementEntrypoints(),
		DeletionGates:          publicFacadeRemovalDeletionGates(),
		ExecutionSteps:         publicFacadeRemovalExecutionSteps(),
		BoundaryChecks:         publicFacadeRemovalBoundaryChecks(),
		RecoverySteps:          publicFacadeRemovalRecoverySteps(),
		DocumentationTargets:   publicFacadeRemovalDocumentationTargets(),
		Warnings:               []string{},
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
	if len(plan.ReplacementEntrypoints) == 0 {
		plan.Warnings = append(plan.Warnings, "public facade removal replacement entrypoints are empty")
	}
	for _, entrypoint := range plan.ReplacementEntrypoints {
		if !entrypoint.Required {
			plan.Warnings = append(plan.Warnings, fmt.Sprintf("public facade removal replacement entrypoint is not required: %s", entrypoint.Name))
		}
		if !entrypoint.GoNativeBacked {
			plan.Warnings = append(plan.Warnings, fmt.Sprintf("public facade removal replacement entrypoint is not Go-native backed: %s", entrypoint.Name))
		}
		if strings.TrimSpace(entrypoint.Name) == "" {
			plan.Warnings = append(plan.Warnings, "public facade removal replacement entrypoint missing name")
		}
		if strings.TrimSpace(entrypoint.Entrypoint) == "" {
			plan.Warnings = append(plan.Warnings, fmt.Sprintf("public facade removal replacement entrypoint missing entrypoint: %s", entrypoint.Name))
		}
		if strings.TrimSpace(entrypoint.Audience) == "" {
			plan.Warnings = append(plan.Warnings, fmt.Sprintf("public facade removal replacement entrypoint missing audience: %s", entrypoint.Name))
		}
		if strings.TrimSpace(entrypoint.Purpose) == "" {
			plan.Warnings = append(plan.Warnings, fmt.Sprintf("public facade removal replacement entrypoint missing purpose: %s", entrypoint.Name))
		}
		if len(entrypoint.ValidationCommands) == 0 {
			plan.Warnings = append(plan.Warnings, fmt.Sprintf("public facade removal replacement entrypoint missing validation commands: %s", entrypoint.Name))
		}
		for _, command := range entrypoint.ValidationCommands {
			if strings.TrimSpace(command) == "" {
				plan.Warnings = append(plan.Warnings, fmt.Sprintf("public facade removal replacement entrypoint has empty validation command: %s", entrypoint.Name))
			}
		}
		for _, command := range publicFacadeRemovalImpactValidationCommands() {
			if !slices.Contains(entrypoint.ValidationCommands, command) {
				plan.Warnings = append(plan.Warnings, fmt.Sprintf("public facade removal replacement entrypoint missing validation command %q: %s", command, entrypoint.Name))
			}
		}
	}
	if len(plan.DeletionGates) == 0 {
		plan.Warnings = append(plan.Warnings, "public facade removal deletion gates are empty")
	}
	executionStepNames := map[string]bool{}
	for _, step := range plan.ExecutionSteps {
		if strings.TrimSpace(step.Name) != "" {
			executionStepNames[step.Name] = true
		}
	}
	for _, gate := range plan.DeletionGates {
		if !gate.Required {
			plan.Warnings = append(plan.Warnings, fmt.Sprintf("public facade removal deletion gate is not required: %s", gate.Name))
		}
		if !gate.BlocksRemoval {
			plan.Warnings = append(plan.Warnings, fmt.Sprintf("public facade removal deletion gate does not block removal: %s", gate.Name))
		}
		if len(gate.BlockedExecutionSteps) == 0 {
			plan.Warnings = append(plan.Warnings, fmt.Sprintf("public facade removal deletion gate missing blocked execution steps: %s", gate.Name))
		}
		if !slices.Contains(gate.BlockedExecutionSteps, "delete-public-facade") {
			plan.Warnings = append(plan.Warnings, fmt.Sprintf("public facade removal deletion gate does not block delete-public-facade: %s", gate.Name))
		}
		for _, step := range gate.BlockedExecutionSteps {
			if strings.TrimSpace(step) == "" {
				plan.Warnings = append(plan.Warnings, fmt.Sprintf("public facade removal deletion gate has empty blocked execution step: %s", gate.Name))
			} else if !executionStepNames[step] {
				plan.Warnings = append(plan.Warnings, fmt.Sprintf("public facade removal deletion gate references unknown blocked execution step %q: %s", step, gate.Name))
			}
		}
		if strings.TrimSpace(gate.Name) == "" {
			plan.Warnings = append(plan.Warnings, "public facade removal deletion gate missing name")
		}
		if strings.TrimSpace(gate.Gate) == "" {
			plan.Warnings = append(plan.Warnings, fmt.Sprintf("public facade removal deletion gate missing gate: %s", gate.Name))
		}
		if len(gate.InputInventory) == 0 {
			plan.Warnings = append(plan.Warnings, fmt.Sprintf("public facade removal deletion gate missing input inventory: %s", gate.Name))
		}
		for _, input := range gate.InputInventory {
			if strings.TrimSpace(input) == "" {
				plan.Warnings = append(plan.Warnings, fmt.Sprintf("public facade removal deletion gate has empty input inventory: %s", gate.Name))
			}
		}
		if len(gate.ExitCriteria) == 0 {
			plan.Warnings = append(plan.Warnings, fmt.Sprintf("public facade removal deletion gate missing exit criteria: %s", gate.Name))
		}
		for _, criterion := range gate.ExitCriteria {
			if strings.TrimSpace(criterion) == "" {
				plan.Warnings = append(plan.Warnings, fmt.Sprintf("public facade removal deletion gate has empty exit criterion: %s", gate.Name))
			}
		}
		if len(gate.FailureSignals) == 0 {
			plan.Warnings = append(plan.Warnings, fmt.Sprintf("public facade removal deletion gate missing failure signals: %s", gate.Name))
		}
		for _, signal := range gate.FailureSignals {
			if strings.TrimSpace(signal) == "" {
				plan.Warnings = append(plan.Warnings, fmt.Sprintf("public facade removal deletion gate has empty failure signal: %s", gate.Name))
			}
		}
		if len(gate.EscalationTriggers) == 0 {
			plan.Warnings = append(plan.Warnings, fmt.Sprintf("public facade removal deletion gate missing escalation triggers: %s", gate.Name))
		}
		for _, trigger := range gate.EscalationTriggers {
			if strings.TrimSpace(trigger) == "" {
				plan.Warnings = append(plan.Warnings, fmt.Sprintf("public facade removal deletion gate has empty escalation trigger: %s", gate.Name))
			}
		}
		if len(gate.VerificationArtifacts) == 0 {
			plan.Warnings = append(plan.Warnings, fmt.Sprintf("public facade removal deletion gate missing verification artifacts: %s", gate.Name))
		}
		for _, artifact := range gate.VerificationArtifacts {
			if strings.TrimSpace(artifact) == "" {
				plan.Warnings = append(plan.Warnings, fmt.Sprintf("public facade removal deletion gate has empty verification artifact: %s", gate.Name))
			}
		}
		if len(gate.RemediationActions) == 0 {
			plan.Warnings = append(plan.Warnings, fmt.Sprintf("public facade removal deletion gate missing remediation actions: %s", gate.Name))
		}
		for _, action := range gate.RemediationActions {
			if strings.TrimSpace(action) == "" {
				plan.Warnings = append(plan.Warnings, fmt.Sprintf("public facade removal deletion gate has empty remediation action: %s", gate.Name))
			}
		}
		if len(gate.ValidationCommands) == 0 {
			plan.Warnings = append(plan.Warnings, fmt.Sprintf("public facade removal deletion gate missing validation commands: %s", gate.Name))
		}
		for _, command := range gate.ValidationCommands {
			if strings.TrimSpace(command) == "" {
				plan.Warnings = append(plan.Warnings, fmt.Sprintf("public facade removal deletion gate has empty validation command: %s", gate.Name))
			}
		}
		for _, command := range publicFacadeRemovalImpactValidationCommands() {
			if !slices.Contains(gate.ValidationCommands, command) {
				plan.Warnings = append(plan.Warnings, fmt.Sprintf("public facade removal deletion gate missing validation command %q: %s", command, gate.Name))
			}
		}
	}
	if len(plan.ExecutionSteps) == 0 {
		plan.Warnings = append(plan.Warnings, "public facade removal execution steps are empty")
	}
	for _, step := range plan.ExecutionSteps {
		if !step.Required {
			plan.Warnings = append(plan.Warnings, fmt.Sprintf("public facade removal execution step is not required: %s", step.Name))
		}
		if strings.TrimSpace(step.Name) == "" {
			plan.Warnings = append(plan.Warnings, "public facade removal execution step missing name")
		}
		if strings.TrimSpace(step.Action) == "" {
			plan.Warnings = append(plan.Warnings, fmt.Sprintf("public facade removal execution step missing action: %s", step.Name))
		}
		if len(step.DependsOn) == 0 {
			plan.Warnings = append(plan.Warnings, fmt.Sprintf("public facade removal execution step missing dependencies: %s", step.Name))
		}
		for _, dependency := range step.DependsOn {
			if strings.TrimSpace(dependency) == "" {
				plan.Warnings = append(plan.Warnings, fmt.Sprintf("public facade removal execution step has empty dependency: %s", step.Name))
			}
		}
		if len(step.InputInventory) == 0 {
			plan.Warnings = append(plan.Warnings, fmt.Sprintf("public facade removal execution step missing input inventory: %s", step.Name))
		}
		for _, input := range step.InputInventory {
			if strings.TrimSpace(input) == "" {
				plan.Warnings = append(plan.Warnings, fmt.Sprintf("public facade removal execution step has empty input inventory: %s", step.Name))
			}
		}
		if len(step.OutputArtifacts) == 0 {
			plan.Warnings = append(plan.Warnings, fmt.Sprintf("public facade removal execution step missing output artifacts: %s", step.Name))
		}
		for _, artifact := range step.OutputArtifacts {
			if strings.TrimSpace(artifact) == "" {
				plan.Warnings = append(plan.Warnings, fmt.Sprintf("public facade removal execution step has empty output artifact: %s", step.Name))
			}
		}
		if len(step.ValidationCommands) == 0 {
			plan.Warnings = append(plan.Warnings, fmt.Sprintf("public facade removal execution step missing validation commands: %s", step.Name))
		}
		for _, command := range step.ValidationCommands {
			if strings.TrimSpace(command) == "" {
				plan.Warnings = append(plan.Warnings, fmt.Sprintf("public facade removal execution step has empty validation command: %s", step.Name))
			}
		}
		for _, command := range publicFacadeRemovalImpactValidationCommands() {
			if !slices.Contains(step.ValidationCommands, command) {
				plan.Warnings = append(plan.Warnings, fmt.Sprintf("public facade removal execution step missing validation command %q: %s", command, step.Name))
			}
		}
		if step.AllowsPowerShellRuntime {
			plan.Warnings = append(plan.Warnings, fmt.Sprintf("public facade removal execution step allows PowerShell runtime logic: %s", step.Name))
		}
		if step.AllowsExternalEffects {
			plan.Warnings = append(plan.Warnings, fmt.Sprintf("public facade removal execution step allows external effects: %s", step.Name))
		}
	}
	if len(plan.BoundaryChecks) == 0 {
		plan.Warnings = append(plan.Warnings, "public facade removal boundary checks are empty")
	}
	for _, check := range plan.BoundaryChecks {
		if !check.Required {
			plan.Warnings = append(plan.Warnings, fmt.Sprintf("public facade removal boundary check is not required: %s", check.Name))
		}
		if !check.Preserved {
			plan.Warnings = append(plan.Warnings, fmt.Sprintf("public facade removal boundary check is not preserved: %s", check.Name))
		}
		if strings.TrimSpace(check.Name) == "" {
			plan.Warnings = append(plan.Warnings, "public facade removal boundary check missing name")
		}
		if strings.TrimSpace(check.Boundary) == "" {
			plan.Warnings = append(plan.Warnings, fmt.Sprintf("public facade removal boundary check missing boundary: %s", check.Name))
		}
		if len(check.Evidence) == 0 {
			plan.Warnings = append(plan.Warnings, fmt.Sprintf("public facade removal boundary check missing evidence: %s", check.Name))
		}
		for _, evidence := range check.Evidence {
			if strings.TrimSpace(evidence) == "" {
				plan.Warnings = append(plan.Warnings, fmt.Sprintf("public facade removal boundary check has empty evidence: %s", check.Name))
			}
		}
		if len(check.ValidationCommands) == 0 {
			plan.Warnings = append(plan.Warnings, fmt.Sprintf("public facade removal boundary check missing validation commands: %s", check.Name))
		}
		for _, command := range check.ValidationCommands {
			if strings.TrimSpace(command) == "" {
				plan.Warnings = append(plan.Warnings, fmt.Sprintf("public facade removal boundary check has empty validation command: %s", check.Name))
			}
		}
		for _, command := range publicFacadeRemovalImpactValidationCommands() {
			if !slices.Contains(check.ValidationCommands, command) {
				plan.Warnings = append(plan.Warnings, fmt.Sprintf("public facade removal boundary check missing validation command %q: %s", command, check.Name))
			}
		}
	}
	for _, step := range plan.RecoverySteps {
		if !step.Required {
			plan.Warnings = append(plan.Warnings, fmt.Sprintf("public facade removal recovery step is not required: %s", step.Name))
		}
		if strings.TrimSpace(step.Action) == "" {
			plan.Warnings = append(plan.Warnings, fmt.Sprintf("public facade removal recovery step missing action: %s", step.Name))
		}
		if len(step.Paths) == 0 {
			plan.Warnings = append(plan.Warnings, fmt.Sprintf("public facade removal recovery step missing paths: %s", step.Name))
		}
		for _, path := range step.Paths {
			if strings.TrimSpace(path) == "" {
				plan.Warnings = append(plan.Warnings, fmt.Sprintf("public facade removal recovery step has empty path: %s", step.Name))
			}
		}
		if len(step.ValidationCommands) == 0 {
			plan.Warnings = append(plan.Warnings, fmt.Sprintf("public facade removal recovery step missing validation commands: %s", step.Name))
		}
		for _, command := range step.ValidationCommands {
			if strings.TrimSpace(command) == "" {
				plan.Warnings = append(plan.Warnings, fmt.Sprintf("public facade removal recovery step has empty validation command: %s", step.Name))
			}
		}
		for _, command := range publicFacadeRemovalImpactValidationCommands() {
			if !slices.Contains(step.ValidationCommands, command) {
				plan.Warnings = append(plan.Warnings, fmt.Sprintf("public facade removal recovery step missing validation command %q: %s", command, step.Name))
			}
		}
	}
	for _, target := range plan.DocumentationTargets {
		if !target.Required {
			plan.Warnings = append(plan.Warnings, fmt.Sprintf("public facade removal documentation target is not required: %s", target.Path))
		}
		if strings.TrimSpace(target.Path) == "" {
			plan.Warnings = append(plan.Warnings, "public facade removal documentation target missing path")
		}
		if strings.TrimSpace(target.Purpose) == "" {
			plan.Warnings = append(plan.Warnings, fmt.Sprintf("public facade removal documentation target missing purpose: %s", target.Path))
		}
		if strings.TrimSpace(target.Action) == "" {
			plan.Warnings = append(plan.Warnings, fmt.Sprintf("public facade removal documentation target missing action: %s", target.Path))
		}
		if len(target.ValidationCommands) == 0 {
			plan.Warnings = append(plan.Warnings, fmt.Sprintf("public facade removal documentation target missing validation commands: %s", target.Path))
		}
		for _, command := range publicFacadeRemovalImpactValidationCommands() {
			if !slices.Contains(target.ValidationCommands, command) {
				plan.Warnings = append(plan.Warnings, fmt.Sprintf("public facade removal documentation target missing validation command %q: %s", command, target.Path))
			}
		}
	}
	if len(plan.Warnings) > 0 {
		plan.Ready = false
		plan.Summary = "public facade removal plan has warnings"
	}
	return plan
}

func publicFacadeRemovalReplacementEntrypoints() []PublicFacadeRemovalReplacementEntrypoint {
	commands := publicFacadeRemovalImpactValidationCommands()
	return []PublicFacadeRemovalReplacementEntrypoint{
		{
			Name:               "canonical-rekit-skill",
			Entrypoint:         ".claude/skills/rekit/SKILL.md",
			Audience:           "users and Mission Commander",
			Purpose:            "keep /rekit skill as the primary user-facing Mission Control entrypoint after public facade removal",
			Required:           true,
			GoNativeBacked:     true,
			UserFacing:         true,
			ValidationCommands: commands,
		},
		{
			Name:               "case-local-thin-shim",
			Entrypoint:         "rekit/templates/case-shim/SKILL.md",
			Audience:           "attached case users and replacement session executors",
			Purpose:            "keep case-local /rekit shims delegating to the canonical skill and Go-native runtime",
			Required:           true,
			GoNativeBacked:     true,
			UserFacing:         true,
			ValidationCommands: commands,
		},
		{
			Name:               "direct-go-cli",
			Entrypoint:         "go run ./cmd/rekit -- -Command <command>",
			Audience:           "automation and deterministic runtime maintainers",
			Purpose:            "provide the low-level Go-native command alternative for the public command surface",
			Required:           true,
			GoNativeBacked:     true,
			UserFacing:         false,
			ValidationCommands: commands,
		},
		{
			Name:               "cross-platform-release-gate",
			Entrypoint:         ".github/workflows/release-gate.yml",
			Audience:           "CI and release maintainers",
			Purpose:            "keep Linux, Windows and macOS release checks on Go-native commands without a facade dependency",
			Required:           true,
			GoNativeBacked:     true,
			UserFacing:         false,
			ValidationCommands: commands,
		},
	}
}

func publicFacadeRemovalReplacementValidationCommandCount(entrypoints []PublicFacadeRemovalReplacementEntrypoint) int {
	count := 0
	for _, entrypoint := range entrypoints {
		count += len(entrypoint.ValidationCommands)
	}
	return count
}

func publicFacadeRemovalDeletionGates() []PublicFacadeRemovalDeletionGate {
	commands := publicFacadeRemovalImpactValidationCommands()
	return []PublicFacadeRemovalDeletionGate{
		{
			Name:                  "go-native-alternatives-ready",
			Gate:                  "replacement entrypoints and Go-native public surface must be ready before deleting the public facade",
			Required:              true,
			BlocksRemoval:         true,
			BlockedExecutionSteps: []string{"delete-public-facade", "rerun-release-gate"},
			InputInventory:        []string{"publicFacadeRemoval.removalPlan.replacementEntrypoints", "goNativePublicSurface"},
			ExitCriteria: []string{
				"replacementEntrypoints[] covers all required user-facing and deterministic alternatives",
				"goNativePublicSurface.ready and facadeRemovalReady are true",
				"release-check JSON/text show the alternative path without facade dependency",
			},
			FailureSignals: []string{
				"replacementEntrypoints[] is missing a required Go-native backed alternative",
				"goNativePublicSurface.ready or facadeRemovalReady is false",
				"release-check output still documents a facade dependency for the alternative path",
			},
			EscalationTriggers: []string{
				"the deletion gate requires a product direction change before retrying removal",
				"the deletion gate requires deleting a public entrypoint without an approved Go-native alternative",
				"the deletion gate requires authority/confirmed, actual heavy-tool or external side effects",
			},
			VerificationArtifacts: []string{
				"release-check JSON/text output showing goNativePublicSurface.ready and facadeRemovalReady",
				"publicFacadeRemoval.removalPlan.replacementEntrypoints[] inventory review",
				"Go-native public surface command/profile coverage evidence",
			},
			RemediationActions: []string{
				"keep the public facade retained until replacementEntrypoints[] and goNativePublicSurface are ready",
				"update Go-native public surface inventory or replacement entrypoint docs before retrying removal",
				"rerun release-check JSON/text after remediation and before delete-public-facade",
			},
			ValidationCommands: commands,
		},
		{
			Name:                  "public-references-migrated",
			Gate:                  "all public facade migration targets must be processed before deleting the public facade",
			Required:              true,
			BlocksRemoval:         true,
			BlockedExecutionSteps: []string{"delete-public-facade", "rerun-release-gate"},
			InputInventory:        []string{"publicFacadeRemoval.removalImpact.migrationTargets", "publicFacadeRemoval.removalPlan.documentationTargets"},
			ExitCriteria: []string{
				"migrationTargets[] have been rewritten, retired or explicitly preserved as historical context",
				"documentationTargets[] have been synchronized with the removal result",
				"removalImpact.unclassifiedReferences remains zero after reference migration",
			},
			FailureSignals: []string{
				"migrationTargets[] still contains unprocessed public facade references",
				"documentationTargets[] drift from the migration or historical-preservation result",
				"removalImpact.unclassifiedReferences is non-zero",
			},
			EscalationTriggers: []string{
				"the deletion gate requires a product direction change before retrying removal",
				"the deletion gate requires deleting a public entrypoint without an approved Go-native alternative",
				"the deletion gate requires authority/confirmed, actual heavy-tool or external side effects",
			},
			VerificationArtifacts: []string{
				"migrationTargets[] processing diff or preservation rationale",
				"documentationTargets[] synchronization diff",
				"removalImpact reference classification output with unclassifiedReferences=0",
			},
			RemediationActions: []string{
				"process unresolved migrationTargets[] before deleting the public facade",
				"synchronize documentationTargets[] with the chosen migration or historical-preservation result",
				"clear unclassifiedReferences through classification or documented preservation before retrying removal",
			},
			ValidationCommands: commands,
		},
		{
			Name:                  "facade-smoke-retired",
			Gate:                  "facade compatibility and dependent smoke targets must be rewritten or retired before deleting the public facade",
			Required:              true,
			BlocksRemoval:         true,
			BlockedExecutionSteps: []string{"delete-public-facade", "rerun-release-gate"},
			InputInventory:        []string{"publicFacadeRemoval.removalImpact.smokeMigrationTargets"},
			ExitCriteria: []string{
				"smokeMigrationTargets[] no longer require public facade compatibility assertions",
				"replacement Go-native checks cover the retired facade smoke intent",
				"no default release gate step depends on facade-smoke.ps1",
			},
			FailureSignals: []string{
				"smokeMigrationTargets[] still require public facade compatibility assertions",
				"no replacement Go-native check covers the retired facade smoke intent",
				"ciReleaseGate or gateProfile still references facade-smoke.ps1",
			},
			EscalationTriggers: []string{
				"the deletion gate requires a product direction change before retrying removal",
				"the deletion gate requires deleting a public entrypoint without an approved Go-native alternative",
				"the deletion gate requires authority/confirmed, actual heavy-tool or external side effects",
			},
			VerificationArtifacts: []string{
				"smokeMigrationTargets[] retirement or migration review",
				"replacement Go-native smoke or package-test output",
				"CI release gate inventory proving no facade-smoke.ps1 default dependency",
			},
			RemediationActions: []string{
				"retire facade compatibility assertions from smokeMigrationTargets[] before deleting the facade",
				"add or confirm Go-native package-test coverage for the retired facade smoke intent",
				"keep facade-smoke.ps1 out of the default release gate before retrying removal",
			},
			ValidationCommands: commands,
		},
		{
			Name:                  "recovery-path-ready",
			Gate:                  "recovery steps and synchronized docs must be ready before deleting the public facade",
			Required:              true,
			BlocksRemoval:         true,
			BlockedExecutionSteps: []string{"delete-public-facade", "rerun-release-gate"},
			InputInventory:        []string{"publicFacadeRemoval.removalPlan.recoverySteps", "publicFacadeRemoval.removalPlan.documentationTargets"},
			ExitCriteria: []string{
				"recoverySteps[] include public facade, docs and release gate restoration paths",
				"documentationTargets[] include every public removal communication surface",
				"the removal can be reverted by a separate commit without schema migration",
			},
			FailureSignals: []string{
				"recoverySteps[] miss public facade, docs or release gate restoration",
				"documentationTargets[] omit a public removal communication surface",
				"the removal cannot be reverted as a separate commit",
			},
			EscalationTriggers: []string{
				"the deletion gate requires a product direction change before retrying removal",
				"the deletion gate requires deleting a public entrypoint without an approved Go-native alternative",
				"the deletion gate requires authority/confirmed, actual heavy-tool or external side effects",
			},
			VerificationArtifacts: []string{
				"recoverySteps[] review covering public facade, docs and release gate restoration",
				"documentationTargets[] communication surface checklist",
				"separate revertable commit or revert plan evidence",
			},
			RemediationActions: []string{
				"add or repair recoverySteps[] before deleting the public facade",
				"synchronize documentationTargets[] for every public removal communication surface",
				"split the removal into a separately revertable commit before retrying deletion",
			},
			ValidationCommands: commands,
		},
		{
			Name:                  "release-gate-green",
			Gate:                  "Go-native release gate must pass before and after deleting the public facade",
			Required:              true,
			BlocksRemoval:         true,
			BlockedExecutionSteps: []string{"delete-public-facade", "rerun-release-gate"},
			InputInventory:        []string{"gateProfile", "ciReleaseGate", "releaseHandoff.signals"},
			ExitCriteria: []string{
				"gateProfile and ciReleaseGate remain ready on Go-native commands",
				"releaseHandoff.signals[] show synchronized public facade removal readiness",
				"go test, go vet and git diff --check pass after the removal step",
			},
			FailureSignals: []string{
				"gateProfile or ciReleaseGate is not ready",
				"releaseHandoff.signals[] omit synchronized public facade removal readiness",
				"go test, go vet or git diff --check fails after removal",
			},
			EscalationTriggers: []string{
				"the deletion gate requires a product direction change before retrying removal",
				"the deletion gate requires deleting a public entrypoint without an approved Go-native alternative",
				"the deletion gate requires authority/confirmed, actual heavy-tool or external side effects",
			},
			VerificationArtifacts: []string{
				"release-check JSON/text output after removal",
				"go test ./... and go vet ./... output",
				"git diff --check output with no whitespace errors",
			},
			RemediationActions: []string{
				"fix gateProfile or ciReleaseGate drift before deleting the public facade",
				"repair releaseHandoff.signals[] so public facade removal readiness is synchronized",
				"fix go test, go vet or git diff --check failures before retrying removal",
			},
			ValidationCommands: commands,
		},
	}
}

func publicFacadeRemovalDeletionGateValidationCommandCount(gates []PublicFacadeRemovalDeletionGate) int {
	count := 0
	for _, gate := range gates {
		count += len(gate.ValidationCommands)
	}
	return count
}

func publicFacadeRemovalDeletionGateExitCriteriaCount(gates []PublicFacadeRemovalDeletionGate) int {
	count := 0
	for _, gate := range gates {
		count += len(gate.ExitCriteria)
	}
	return count
}

func publicFacadeRemovalDeletionGateFailureSignalCount(gates []PublicFacadeRemovalDeletionGate) int {
	count := 0
	for _, gate := range gates {
		count += len(gate.FailureSignals)
	}
	return count
}

func publicFacadeRemovalDeletionGateEscalationTriggerCount(gates []PublicFacadeRemovalDeletionGate) int {
	count := 0
	for _, gate := range gates {
		count += len(gate.EscalationTriggers)
	}
	return count
}

func publicFacadeRemovalDeletionGateVerificationArtifactCount(gates []PublicFacadeRemovalDeletionGate) int {
	count := 0
	for _, gate := range gates {
		count += len(gate.VerificationArtifacts)
	}
	return count
}

func publicFacadeRemovalDeletionGateBlockedExecutionStepCount(gates []PublicFacadeRemovalDeletionGate) int {
	count := 0
	for _, gate := range gates {
		count += len(gate.BlockedExecutionSteps)
	}
	return count
}

func publicFacadeRemovalDeletionGateRemediationActionCount(gates []PublicFacadeRemovalDeletionGate) int {
	count := 0
	for _, gate := range gates {
		count += len(gate.RemediationActions)
	}
	return count
}

func publicFacadeRemovalExecutionSteps() []PublicFacadeRemovalExecutionStep {
	commands := publicFacadeRemovalImpactValidationCommands()
	return []PublicFacadeRemovalExecutionStep{
		{
			Name:                    "verify-go-native-alternative",
			Action:                  "verify /rekit Mission Control and direct Go CLI alternatives before removing the public facade",
			Required:                true,
			DependsOn:               []string{"go-native-public-surface", "public-facade-retained-boundary"},
			InputInventory:          []string{"goNativePublicSurface", "powerShellDeprecation.publicFacade"},
			OutputArtifacts:         []string{"release-check JSON/text showing Go-native public surface ready"},
			ValidationCommands:      commands,
			AllowsPowerShellRuntime: false,
			AllowsExternalEffects:   false,
		},
		{
			Name:                    "migrate-public-references",
			Action:                  "process every public facade migration target before deleting the facade",
			Required:                true,
			DependsOn:               []string{"removalImpact.migrationTargets", "removalPlan.documentationTargets"},
			InputInventory:          []string{"publicFacadeRemoval.removalImpact.migrationTargets", "publicFacadeRemoval.removalPlan.documentationTargets"},
			OutputArtifacts:         []string{"updated docs, tests and release inventory references"},
			ValidationCommands:      commands,
			AllowsPowerShellRuntime: false,
			AllowsExternalEffects:   false,
		},
		{
			Name:                    "retire-facade-smoke",
			Action:                  "rewrite or retire facade compatibility and dependent smoke assertions",
			Required:                true,
			DependsOn:               []string{"removalImpact.smokeMigrationTargets"},
			InputInventory:          []string{"publicFacadeRemoval.removalImpact.smokeMigrationTargets"},
			OutputArtifacts:         []string{"Go-native tests or explicitly retired facade assertions"},
			ValidationCommands:      commands,
			AllowsPowerShellRuntime: false,
			AllowsExternalEffects:   false,
		},
		{
			Name:                    "delete-public-facade",
			Action:                  "delete rekit/rekit.ps1 only after alternatives, references and smoke targets are resolved in the independent removal batch",
			Required:                true,
			DependsOn:               []string{"verify-go-native-alternative", "migrate-public-references", "retire-facade-smoke"},
			InputInventory:          []string{"publicFacadeRemoval.removalPlan.recoverySteps", "publicFacadeRemoval.removalImpact.workItems"},
			OutputArtifacts:         []string{"separate revertable commit removing rekit/rekit.ps1"},
			ValidationCommands:      commands,
			AllowsPowerShellRuntime: false,
			AllowsExternalEffects:   false,
		},
		{
			Name:                    "rerun-release-gate",
			Action:                  "rerun Go-native release gate and confirm public facade removal inventory and handoff output are synchronized",
			Required:                true,
			DependsOn:               []string{"delete-public-facade", "restore-public-facade recovery path documented"},
			InputInventory:          []string{"publicFacadeRemoval.removalPlan.recoverySteps", "releaseHandoff.signals"},
			OutputArtifacts:         []string{"release-check JSON/text, go test, go vet and git diff --check results"},
			ValidationCommands:      commands,
			AllowsPowerShellRuntime: false,
			AllowsExternalEffects:   false,
		},
	}
}

func publicFacadeRemovalExecutionValidationCommandCount(steps []PublicFacadeRemovalExecutionStep) int {
	count := 0
	for _, step := range steps {
		count += len(step.ValidationCommands)
	}
	return count
}

func publicFacadeRemovalBoundaryChecks() []PublicFacadeRemovalPlanBoundaryCheck {
	commands := publicFacadeRemovalImpactValidationCommands()
	return []PublicFacadeRemovalPlanBoundaryCheck{
		{
			Name:               "no-powershell-runtime-logic",
			Boundary:           "do not add PowerShell runtime logic or reintroduce retired PowerShell modules during public facade removal",
			Required:           true,
			Preserved:          true,
			Evidence:           []string{"docs/powershell-deprecation.md", "powerShellDeprecation.facadeRuntime", "powerShellDeprecation.moduleRemoval"},
			ValidationCommands: commands,
		},
		{
			Name:               "no-actual-heavy-tool",
			Boundary:           "do not execute actual heavy-tool, debug, dump, patch, hook, network or exploit replay actions in the removal batch",
			Required:           true,
			Preserved:          true,
			Evidence:           []string{"docs/release-readiness.md", "goNativePublicSurface.commandProfilePolicies", "publicFacadeRemoval.removalPlan.executionSteps"},
			ValidationCommands: commands,
		},
		{
			Name:               "no-authority-confirmed-write",
			Boundary:           "do not write authority or confirmed state while removing the public facade",
			Required:           true,
			Preserved:          true,
			Evidence:           []string{"goNativePublicSurface.commandProfileSummary", "goNativePublicSurface.commandProfilePolicies", "docs/release-readiness.md"},
			ValidationCommands: commands,
		},
		{
			Name:               "sync-promote-review-first",
			Boundary:           "preserve sync/promote review-first and explicit apply semantics",
			Required:           true,
			Preserved:          true,
			Evidence:           []string{"goNativePublicSurface.commandProfileGroups", "docs/release-readiness.md", "docs/powershell-deprecation.md"},
			ValidationCommands: commands,
		},
		{
			Name:               "case-local-write-semantics",
			Boundary:           "preserve case-local write semantics for attach/bootstrap/continue/gate/handoff/init/repair/start",
			Required:           true,
			Preserved:          true,
			Evidence:           []string{"goNativePublicSurface.commandProfileBoundaries", "goNativePublicSurface.commandProfilePolicies", "docs/release-readiness.md"},
			ValidationCommands: commands,
		},
		{
			Name:               "no-external-effects",
			Boundary:           "do not publish, scan, fuzz, connect to devices, call external services or create real case artifacts during removal readiness work",
			Required:           true,
			Preserved:          true,
			Evidence:           []string{"docs/release-readiness.md", "docs/powershell-deprecation.md", "publicFacadeRemoval.removalPlan.executionSteps"},
			ValidationCommands: commands,
		},
	}
}

func publicFacadeRemovalBoundaryValidationCommandCount(checks []PublicFacadeRemovalPlanBoundaryCheck) int {
	count := 0
	for _, check := range checks {
		count += len(check.ValidationCommands)
	}
	return count
}

func publicFacadeRemovalRecoverySteps() []PublicFacadeRemovalRecoveryStep {
	return []PublicFacadeRemovalRecoveryStep{
		{
			Name:               "separate-revertable-commit",
			Action:             "keep the public facade removal in a separate revertable commit",
			Required:           true,
			Paths:              []string{"rekit/rekit.ps1"},
			ValidationCommands: publicFacadeRemovalImpactValidationCommands(),
		},
		{
			Name:               "restore-public-facade",
			Action:             "restore the public facade file if the removal batch fails validation",
			Required:           true,
			Paths:              []string{"rekit/rekit.ps1"},
			ValidationCommands: publicFacadeRemovalImpactValidationCommands(),
		},
		{
			Name:               "restore-synchronized-docs",
			Action:             "restore public docs and release notes synchronized during the removal batch",
			Required:           true,
			Paths:              []string{"README.md", "CLAUDE.md", ".claude/skills/rekit/SKILL.md", "rekit/templates/case-shim/SKILL.md", "docs/release-readiness.md", "docs/powershell-deprecation.md", "docs/batch-plan.md", "CHANGELOG.md"},
			ValidationCommands: publicFacadeRemovalImpactValidationCommands(),
		},
		{
			Name:               "rerun-go-native-release-gate",
			Action:             "rerun the Go-native release gate after restoring the facade and docs",
			Required:           true,
			Paths:              []string{"cmd/rekit", "internal/rekit", "docs/release-readiness.md"},
			ValidationCommands: publicFacadeRemovalImpactValidationCommands(),
		},
	}
}

func publicFacadeRemovalRecoveryValidationCommandCount(steps []PublicFacadeRemovalRecoveryStep) int {
	count := 0
	for _, step := range steps {
		count += len(step.ValidationCommands)
	}
	return count
}

func publicFacadeRemovalDocumentationTargets() []PublicFacadeRemovalDocumentationTarget {
	return []PublicFacadeRemovalDocumentationTarget{
		{Path: "README.md", Purpose: "public default docs", Action: "remove or replace public facade references in user-facing overview", Required: true, ValidationCommands: publicFacadeRemovalImpactValidationCommands()},
		{Path: "CLAUDE.md", Purpose: "project instructions", Action: "keep project maintenance guidance aligned with Go-native public path", Required: true, ValidationCommands: publicFacadeRemovalImpactValidationCommands()},
		{Path: ".claude/skills/rekit/SKILL.md", Purpose: "canonical /rekit skill", Action: "keep the user-facing command path on /rekit and Mission Control instead of the removed facade", Required: true, ValidationCommands: publicFacadeRemovalImpactValidationCommands()},
		{Path: "rekit/templates/case-shim/SKILL.md", Purpose: "case-local thin shim", Action: "keep case-local shim documentation free of removed facade defaults", Required: true, ValidationCommands: publicFacadeRemovalImpactValidationCommands()},
		{Path: "docs/release-readiness.md", Purpose: "release gate checklist", Action: "update public facade removal readiness and known gaps", Required: true, ValidationCommands: publicFacadeRemovalImpactValidationCommands()},
		{Path: "docs/powershell-deprecation.md", Purpose: "PowerShell deprecation roadmap", Action: "update removal plan, recovery plan and retained facade status", Required: true, ValidationCommands: publicFacadeRemovalImpactValidationCommands()},
		{Path: "docs/go-first-convergence-plan.md", Purpose: "Go-first convergence plan", Action: "update Stage 8 progress without declaring global completion", Required: true, ValidationCommands: publicFacadeRemovalImpactValidationCommands()},
		{Path: "docs/batch-plan.md", Purpose: "batch implementation record", Action: "record the independent removal batch scope, validation and recovery result", Required: true, ValidationCommands: publicFacadeRemovalImpactValidationCommands()},
		{Path: "CHANGELOG.md", Purpose: "release notes", Action: "record user-visible removal batch changes and boundaries", Required: true, ValidationCommands: publicFacadeRemovalImpactValidationCommands()},
	}
}

func publicFacadeRemovalDocumentationValidationCommandCount(targets []PublicFacadeRemovalDocumentationTarget) int {
	count := 0
	for _, target := range targets {
		count += len(target.ValidationCommands)
	}
	return count
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
		MigrationTargets:       []PublicFacadeRemovalMigrationTarget{},
		SmokeMigrationTargets:  []PublicFacadeRemovalSmokeMigrationTarget{},
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
	impact.MigrationTargets = publicFacadeRemovalMigrationTargets(references)
	impact.SmokeMigrationTargets = publicFacadeRemovalSmokeMigrationTargets(references)
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
	if len(impact.MigrationTargets) != len(impact.References) {
		impact.Warnings = append(impact.Warnings, "public facade removal migration targets do not cover every reference")
	}
	for _, target := range impact.MigrationTargets {
		if !target.Required {
			impact.Warnings = append(impact.Warnings, fmt.Sprintf("public facade removal migration target is not required: %s", target.Path))
		}
		if strings.TrimSpace(target.Path) == "" {
			impact.Warnings = append(impact.Warnings, "public facade removal migration target missing path")
		}
		if strings.TrimSpace(target.Category) == "" {
			impact.Warnings = append(impact.Warnings, fmt.Sprintf("public facade removal migration target missing category: %s", target.Path))
		}
		if strings.TrimSpace(target.Action) == "" {
			impact.Warnings = append(impact.Warnings, fmt.Sprintf("public facade removal migration target missing action: %s", target.Path))
		}
		if !target.GoNativePreferred {
			impact.Warnings = append(impact.Warnings, fmt.Sprintf("public facade removal migration target is not Go-native preferred: %s", target.Path))
		}
		if target.Category == "roadmap-and-history-doc" && !target.PreserveHistoricalContext {
			impact.Warnings = append(impact.Warnings, fmt.Sprintf("public facade removal roadmap/history migration target does not preserve historical context: %s", target.Path))
		}
		if len(target.ValidationCommands) == 0 {
			impact.Warnings = append(impact.Warnings, fmt.Sprintf("public facade removal migration target missing validation commands: %s", target.Path))
		}
		for _, command := range target.ValidationCommands {
			if strings.TrimSpace(command) == "" {
				impact.Warnings = append(impact.Warnings, fmt.Sprintf("public facade removal migration target has empty validation command: %s", target.Path))
			}
		}
		for _, command := range publicFacadeRemovalImpactValidationCommands() {
			if !slices.Contains(target.ValidationCommands, command) {
				impact.Warnings = append(impact.Warnings, fmt.Sprintf("public facade removal migration target missing validation command %q: %s", command, target.Path))
			}
		}
	}
	if len(impact.SmokeMigrationTargets) == 0 {
		impact.Warnings = append(impact.Warnings, "public facade removal smoke migration targets are empty")
	}
	for _, target := range impact.SmokeMigrationTargets {
		if !target.Required {
			impact.Warnings = append(impact.Warnings, fmt.Sprintf("public facade removal smoke migration target is not required: %s", target.Path))
		}
		if strings.TrimSpace(target.Path) == "" {
			impact.Warnings = append(impact.Warnings, "public facade removal smoke migration target missing path")
		}
		if strings.TrimSpace(target.Category) == "" {
			impact.Warnings = append(impact.Warnings, fmt.Sprintf("public facade removal smoke migration target missing category: %s", target.Path))
		}
		if strings.TrimSpace(target.Action) == "" {
			impact.Warnings = append(impact.Warnings, fmt.Sprintf("public facade removal smoke migration target missing action: %s", target.Path))
		}
		if !target.GoNativePreferred {
			impact.Warnings = append(impact.Warnings, fmt.Sprintf("public facade removal smoke migration target is not Go-native preferred: %s", target.Path))
		}
		if target.AllowFacadeCompat {
			impact.Warnings = append(impact.Warnings, fmt.Sprintf("public facade removal smoke migration target still allows facade compatibility: %s", target.Path))
		}
		if !target.RetireFacadeAssertions {
			impact.Warnings = append(impact.Warnings, fmt.Sprintf("public facade removal smoke migration target does not retire facade assertions: %s", target.Path))
		}
		if len(target.ValidationCommands) == 0 {
			impact.Warnings = append(impact.Warnings, fmt.Sprintf("public facade removal smoke migration target missing validation commands: %s", target.Path))
		}
		for _, command := range target.ValidationCommands {
			if strings.TrimSpace(command) == "" {
				impact.Warnings = append(impact.Warnings, fmt.Sprintf("public facade removal smoke migration target has empty validation command: %s", target.Path))
			}
		}
		for _, command := range publicFacadeRemovalImpactValidationCommands() {
			if !slices.Contains(target.ValidationCommands, command) {
				impact.Warnings = append(impact.Warnings, fmt.Sprintf("public facade removal smoke migration target missing validation command %q: %s", command, target.Path))
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

func publicFacadeRemovalMigrationTargets(references []PublicFacadeRemovalImpactReference) []PublicFacadeRemovalMigrationTarget {
	targets := make([]PublicFacadeRemovalMigrationTarget, 0, len(references))
	for _, reference := range references {
		targets = append(targets, PublicFacadeRemovalMigrationTarget{
			Path:                      reference.Path,
			Category:                  reference.Category,
			Action:                    publicFacadeRemovalMigrationAction(reference.Category),
			Required:                  true,
			GoNativePreferred:         true,
			PreserveHistoricalContext: publicFacadeRemovalPreserveHistoricalContext(reference.Category),
			ValidationCommands:        publicFacadeRemovalImpactValidationCommands(),
		})
	}
	sort.Slice(targets, func(i, j int) bool {
		if targets[i].Category == targets[j].Category {
			return targets[i].Path < targets[j].Path
		}
		return targets[i].Category < targets[j].Category
	})
	return targets
}

func publicFacadeRemovalMigrationValidationCommandCount(targets []PublicFacadeRemovalMigrationTarget) int {
	count := 0
	for _, target := range targets {
		count += len(target.ValidationCommands)
	}
	return count
}

func publicFacadeRemovalMigrationAction(category string) string {
	switch category {
	case "public-facade-entrypoint":
		return "remove the public facade entrypoint only in the independent removal batch after Go-native alternatives are verified"
	case "facade-compatibility-smoke", "facade-dependent-smoke":
		return "rewrite or retire facade-dependent smoke while preferring direct Go backend validation"
	case "smoke-catalog-doc":
		return "update smoke catalog and tests guide entries to describe Go-native or retired facade coverage"
	case "pack-wrapper-compatibility":
		return "replace pack wrapper facade references with Go-native validation or document compatibility retirement"
	case "public-default-doc":
		return "remove public facade defaults while keeping /rekit and Mission Control as the user-facing path"
	case "roadmap-and-history-doc":
		return "update roadmap/history text without losing historical context or implying the facade remains the default path"
	case "release-inventory-and-tests":
		return "update release inventory, tests and expected counts after the removal batch"
	default:
		return "classify and migrate this public facade reference before deleting the facade"
	}
}

func publicFacadeRemovalPreserveHistoricalContext(category string) bool {
	return category == "roadmap-and-history-doc"
}

func publicFacadeRemovalSmokeMigrationTargets(references []PublicFacadeRemovalImpactReference) []PublicFacadeRemovalSmokeMigrationTarget {
	targets := []PublicFacadeRemovalSmokeMigrationTarget{}
	for _, reference := range references {
		if reference.Category != "facade-compatibility-smoke" && reference.Category != "facade-dependent-smoke" {
			continue
		}
		targets = append(targets, PublicFacadeRemovalSmokeMigrationTarget{
			Path:                   reference.Path,
			Category:               reference.Category,
			Action:                 publicFacadeRemovalSmokeMigrationAction(reference.Category),
			Required:               true,
			GoNativePreferred:      true,
			AllowFacadeCompat:      false,
			ValidationCommands:     publicFacadeRemovalImpactValidationCommands(),
			RetireFacadeAssertions: true,
		})
	}
	sort.Slice(targets, func(i, j int) bool {
		if targets[i].Category == targets[j].Category {
			return targets[i].Path < targets[j].Path
		}
		return targets[i].Category < targets[j].Category
	})
	return targets
}

func publicFacadeRemovalSmokeMigrationValidationCommandCount(targets []PublicFacadeRemovalSmokeMigrationTarget) int {
	count := 0
	for _, target := range targets {
		count += len(target.ValidationCommands)
	}
	return count
}

func publicFacadeRemovalSmokeMigrationAction(category string) string {
	switch category {
	case "facade-compatibility-smoke":
		return "retire facade compatibility smoke or rewrite it as Go-native source-invariant coverage before deleting the public facade"
	case "facade-dependent-smoke":
		return "rewrite smoke to prefer direct Go backend invocation and retire facade delegation assertions before deleting the public facade"
	default:
		return "rewrite smoke coverage to remove public facade dependency before deleting the public facade"
	}
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
