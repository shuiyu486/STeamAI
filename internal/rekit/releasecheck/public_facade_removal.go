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
	Name                          string   `json:"name"`
	Gate                          string   `json:"gate"`
	Required                      bool     `json:"required"`
	BlocksRemoval                 bool     `json:"blocksRemoval"`
	BlockedExecutionSteps         []string `json:"blockedExecutionSteps"`
	InputInventory                []string `json:"inputInventory"`
	ExitCriteria                  []string `json:"exitCriteria"`
	FailureSignals                []string `json:"failureSignals"`
	EscalationTriggers            []string `json:"escalationTriggers"`
	EscalationEvidence            []string `json:"escalationEvidence"`
	EscalationRecipients          []string `json:"escalationRecipients"`
	EscalationHandoffSteps        []string `json:"escalationHandoffSteps"`
	EscalationDecisionOptions     []string `json:"escalationDecisionOptions"`
	EscalationRetryConditions     []string `json:"escalationRetryConditions"`
	EscalationStopConditions      []string `json:"escalationStopConditions"`
	EscalationResolutionArtifacts []string `json:"escalationResolutionArtifacts"`
	EscalationClosureChecks       []string `json:"escalationClosureChecks"`
	EscalationReopenConditions    []string `json:"escalationReopenConditions"`
	EscalationLedgerEvents        []string `json:"escalationLedgerEvents"`
	EscalationStateTransitions    []string `json:"escalationStateTransitions"`
	EscalationBoundaryGuards      []string `json:"escalationBoundaryGuards"`
	EscalationAuditChecks         []string `json:"escalationAuditChecks"`
	VerificationArtifacts         []string `json:"verificationArtifacts"`
	RemediationActions            []string `json:"remediationActions"`
	ValidationCommands            []string `json:"validationCommands"`
}

type PublicFacadeRemovalExecutionStep struct {
	Name                          string   `json:"name"`
	Action                        string   `json:"action"`
	Required                      bool     `json:"required"`
	DependsOn                     []string `json:"dependsOn"`
	InputInventory                []string `json:"inputInventory"`
	OutputArtifacts               []string `json:"outputArtifacts"`
	FailureSignals                []string `json:"failureSignals"`
	RemediationActions            []string `json:"remediationActions"`
	VerificationArtifacts         []string `json:"verificationArtifacts"`
	LedgerEvents                  []string `json:"ledgerEvents"`
	StateTransitions              []string `json:"stateTransitions"`
	EscalationTriggers            []string `json:"escalationTriggers"`
	EscalationEvidence            []string `json:"escalationEvidence"`
	EscalationRecipients          []string `json:"escalationRecipients"`
	EscalationHandoffSteps        []string `json:"escalationHandoffSteps"`
	EscalationDecisionOptions     []string `json:"escalationDecisionOptions"`
	EscalationRetryConditions     []string `json:"escalationRetryConditions"`
	EscalationStopConditions      []string `json:"escalationStopConditions"`
	EscalationResolutionArtifacts []string `json:"escalationResolutionArtifacts"`
	EscalationClosureChecks       []string `json:"escalationClosureChecks"`
	BoundaryGuards                []string `json:"boundaryGuards"`
	AuditChecks                   []string `json:"auditChecks"`
	ValidationCommands            []string `json:"validationCommands"`
	AllowsPowerShellRuntime       bool     `json:"allowsPowerShellRuntime"`
	AllowsExternalEffects         bool     `json:"allowsExternalEffects"`
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
	details = append(details, fmt.Sprintf("removalPlan=%t planChecks=%d replacementEntrypoints=%d replacementValidationCommands=%d deletionGates=%d deletionGateValidationCommands=%d deletionGateExitCriteria=%d deletionGateFailureSignals=%d deletionGateEscalationTriggers=%d deletionGateEscalationEvidence=%d deletionGateEscalationRecipients=%d deletionGateEscalationHandoffSteps=%d deletionGateEscalationDecisionOptions=%d deletionGateEscalationRetryConditions=%d deletionGateEscalationStopConditions=%d deletionGateEscalationResolutionArtifacts=%d deletionGateEscalationClosureChecks=%d deletionGateEscalationReopenConditions=%d deletionGateEscalationLedgerEvents=%d deletionGateEscalationStateTransitions=%d deletionGateEscalationBoundaryGuards=%d deletionGateEscalationAuditChecks=%d deletionGateVerificationArtifacts=%d deletionGateBlockedExecutionSteps=%d deletionGateRemediationActions=%d executionSteps=%d executionFailureSignals=%d executionRemediationActions=%d executionVerificationArtifacts=%d executionLedgerEvents=%d executionStateTransitions=%d executionEscalationTriggers=%d executionEscalationEvidence=%d executionEscalationRecipients=%d executionEscalationHandoffSteps=%d executionEscalationDecisionOptions=%d executionEscalationRetryConditions=%d executionEscalationStopConditions=%d executionEscalationResolutionArtifacts=%d executionEscalationClosureChecks=%d executionBoundaryGuards=%d executionAuditChecks=%d executionValidationCommands=%d boundaryChecks=%d boundaryValidationCommands=%d recoverySteps=%d recoveryValidationCommands=%d documentationTargets=%d documentationValidationCommands=%d", inventory.RemovalPlan.Ready, len(inventory.RemovalPlan.RequiredPhrases), len(inventory.RemovalPlan.ReplacementEntrypoints), publicFacadeRemovalReplacementValidationCommandCount(inventory.RemovalPlan.ReplacementEntrypoints), len(inventory.RemovalPlan.DeletionGates), publicFacadeRemovalDeletionGateValidationCommandCount(inventory.RemovalPlan.DeletionGates), publicFacadeRemovalDeletionGateExitCriteriaCount(inventory.RemovalPlan.DeletionGates), publicFacadeRemovalDeletionGateFailureSignalCount(inventory.RemovalPlan.DeletionGates), publicFacadeRemovalDeletionGateEscalationTriggerCount(inventory.RemovalPlan.DeletionGates), publicFacadeRemovalDeletionGateEscalationEvidenceCount(inventory.RemovalPlan.DeletionGates), publicFacadeRemovalDeletionGateEscalationRecipientCount(inventory.RemovalPlan.DeletionGates), publicFacadeRemovalDeletionGateEscalationHandoffStepCount(inventory.RemovalPlan.DeletionGates), publicFacadeRemovalDeletionGateEscalationDecisionOptionCount(inventory.RemovalPlan.DeletionGates), publicFacadeRemovalDeletionGateEscalationRetryConditionCount(inventory.RemovalPlan.DeletionGates), publicFacadeRemovalDeletionGateEscalationStopConditionCount(inventory.RemovalPlan.DeletionGates), publicFacadeRemovalDeletionGateEscalationResolutionArtifactCount(inventory.RemovalPlan.DeletionGates), publicFacadeRemovalDeletionGateEscalationClosureCheckCount(inventory.RemovalPlan.DeletionGates), publicFacadeRemovalDeletionGateEscalationReopenConditionCount(inventory.RemovalPlan.DeletionGates), publicFacadeRemovalDeletionGateEscalationLedgerEventCount(inventory.RemovalPlan.DeletionGates), publicFacadeRemovalDeletionGateEscalationStateTransitionCount(inventory.RemovalPlan.DeletionGates), publicFacadeRemovalDeletionGateEscalationBoundaryGuardCount(inventory.RemovalPlan.DeletionGates), publicFacadeRemovalDeletionGateEscalationAuditCheckCount(inventory.RemovalPlan.DeletionGates), publicFacadeRemovalDeletionGateVerificationArtifactCount(inventory.RemovalPlan.DeletionGates), publicFacadeRemovalDeletionGateBlockedExecutionStepCount(inventory.RemovalPlan.DeletionGates), publicFacadeRemovalDeletionGateRemediationActionCount(inventory.RemovalPlan.DeletionGates), len(inventory.RemovalPlan.ExecutionSteps), publicFacadeRemovalExecutionFailureSignalCount(inventory.RemovalPlan.ExecutionSteps), publicFacadeRemovalExecutionRemediationActionCount(inventory.RemovalPlan.ExecutionSteps), publicFacadeRemovalExecutionVerificationArtifactCount(inventory.RemovalPlan.ExecutionSteps), publicFacadeRemovalExecutionLedgerEventCount(inventory.RemovalPlan.ExecutionSteps), publicFacadeRemovalExecutionStateTransitionCount(inventory.RemovalPlan.ExecutionSteps), publicFacadeRemovalExecutionEscalationTriggerCount(inventory.RemovalPlan.ExecutionSteps), publicFacadeRemovalExecutionEscalationEvidenceCount(inventory.RemovalPlan.ExecutionSteps), publicFacadeRemovalExecutionEscalationRecipientCount(inventory.RemovalPlan.ExecutionSteps), publicFacadeRemovalExecutionEscalationHandoffStepCount(inventory.RemovalPlan.ExecutionSteps), publicFacadeRemovalExecutionEscalationDecisionOptionCount(inventory.RemovalPlan.ExecutionSteps), publicFacadeRemovalExecutionEscalationRetryConditionCount(inventory.RemovalPlan.ExecutionSteps), publicFacadeRemovalExecutionEscalationStopConditionCount(inventory.RemovalPlan.ExecutionSteps), publicFacadeRemovalExecutionEscalationResolutionArtifactCount(inventory.RemovalPlan.ExecutionSteps), publicFacadeRemovalExecutionEscalationClosureCheckCount(inventory.RemovalPlan.ExecutionSteps), publicFacadeRemovalExecutionBoundaryGuardCount(inventory.RemovalPlan.ExecutionSteps), publicFacadeRemovalExecutionAuditCheckCount(inventory.RemovalPlan.ExecutionSteps), publicFacadeRemovalExecutionValidationCommandCount(inventory.RemovalPlan.ExecutionSteps), len(inventory.RemovalPlan.BoundaryChecks), publicFacadeRemovalBoundaryValidationCommandCount(inventory.RemovalPlan.BoundaryChecks), len(inventory.RemovalPlan.RecoverySteps), publicFacadeRemovalRecoveryValidationCommandCount(inventory.RemovalPlan.RecoverySteps), len(inventory.RemovalPlan.DocumentationTargets), publicFacadeRemovalDocumentationValidationCommandCount(inventory.RemovalPlan.DocumentationTargets)))
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
				Summary: fmt.Sprintf("removalPlanReady=%t checks=%d replacementEntrypoints=%d replacementValidationCommands=%d deletionGates=%d deletionGateValidationCommands=%d deletionGateExitCriteria=%d deletionGateFailureSignals=%d deletionGateEscalationTriggers=%d deletionGateEscalationEvidence=%d deletionGateEscalationRecipients=%d deletionGateEscalationHandoffSteps=%d deletionGateEscalationDecisionOptions=%d deletionGateEscalationRetryConditions=%d deletionGateEscalationStopConditions=%d deletionGateEscalationResolutionArtifacts=%d deletionGateEscalationClosureChecks=%d deletionGateEscalationReopenConditions=%d deletionGateEscalationLedgerEvents=%d deletionGateEscalationStateTransitions=%d deletionGateEscalationBoundaryGuards=%d deletionGateEscalationAuditChecks=%d deletionGateVerificationArtifacts=%d deletionGateBlockedExecutionSteps=%d deletionGateRemediationActions=%d executionSteps=%d executionFailureSignals=%d executionRemediationActions=%d executionVerificationArtifacts=%d executionLedgerEvents=%d executionStateTransitions=%d executionEscalationTriggers=%d executionEscalationEvidence=%d executionEscalationRecipients=%d executionEscalationHandoffSteps=%d executionEscalationDecisionOptions=%d executionEscalationRetryConditions=%d executionEscalationStopConditions=%d executionEscalationResolutionArtifacts=%d executionEscalationClosureChecks=%d executionBoundaryGuards=%d executionAuditChecks=%d executionValidationCommands=%d boundaryChecks=%d boundaryValidationCommands=%d recoverySteps=%d recoveryValidationCommands=%d documentationTargets=%d documentationValidationCommands=%d", removalPlan.Ready, len(removalPlan.RequiredPhrases), len(removalPlan.ReplacementEntrypoints), publicFacadeRemovalReplacementValidationCommandCount(removalPlan.ReplacementEntrypoints), len(removalPlan.DeletionGates), publicFacadeRemovalDeletionGateValidationCommandCount(removalPlan.DeletionGates), publicFacadeRemovalDeletionGateExitCriteriaCount(removalPlan.DeletionGates), publicFacadeRemovalDeletionGateFailureSignalCount(removalPlan.DeletionGates), publicFacadeRemovalDeletionGateEscalationTriggerCount(removalPlan.DeletionGates), publicFacadeRemovalDeletionGateEscalationEvidenceCount(removalPlan.DeletionGates), publicFacadeRemovalDeletionGateEscalationRecipientCount(removalPlan.DeletionGates), publicFacadeRemovalDeletionGateEscalationHandoffStepCount(removalPlan.DeletionGates), publicFacadeRemovalDeletionGateEscalationDecisionOptionCount(removalPlan.DeletionGates), publicFacadeRemovalDeletionGateEscalationRetryConditionCount(removalPlan.DeletionGates), publicFacadeRemovalDeletionGateEscalationStopConditionCount(removalPlan.DeletionGates), publicFacadeRemovalDeletionGateEscalationResolutionArtifactCount(removalPlan.DeletionGates), publicFacadeRemovalDeletionGateEscalationClosureCheckCount(removalPlan.DeletionGates), publicFacadeRemovalDeletionGateEscalationReopenConditionCount(removalPlan.DeletionGates), publicFacadeRemovalDeletionGateEscalationLedgerEventCount(removalPlan.DeletionGates), publicFacadeRemovalDeletionGateEscalationStateTransitionCount(removalPlan.DeletionGates), publicFacadeRemovalDeletionGateEscalationBoundaryGuardCount(removalPlan.DeletionGates), publicFacadeRemovalDeletionGateEscalationAuditCheckCount(removalPlan.DeletionGates), publicFacadeRemovalDeletionGateVerificationArtifactCount(removalPlan.DeletionGates), publicFacadeRemovalDeletionGateBlockedExecutionStepCount(removalPlan.DeletionGates), publicFacadeRemovalDeletionGateRemediationActionCount(removalPlan.DeletionGates), len(removalPlan.ExecutionSteps), publicFacadeRemovalExecutionFailureSignalCount(removalPlan.ExecutionSteps), publicFacadeRemovalExecutionRemediationActionCount(removalPlan.ExecutionSteps), publicFacadeRemovalExecutionVerificationArtifactCount(removalPlan.ExecutionSteps), publicFacadeRemovalExecutionLedgerEventCount(removalPlan.ExecutionSteps), publicFacadeRemovalExecutionStateTransitionCount(removalPlan.ExecutionSteps), publicFacadeRemovalExecutionEscalationTriggerCount(removalPlan.ExecutionSteps), publicFacadeRemovalExecutionEscalationEvidenceCount(removalPlan.ExecutionSteps), publicFacadeRemovalExecutionEscalationRecipientCount(removalPlan.ExecutionSteps), publicFacadeRemovalExecutionEscalationHandoffStepCount(removalPlan.ExecutionSteps), publicFacadeRemovalExecutionEscalationDecisionOptionCount(removalPlan.ExecutionSteps), publicFacadeRemovalExecutionEscalationRetryConditionCount(removalPlan.ExecutionSteps), publicFacadeRemovalExecutionEscalationStopConditionCount(removalPlan.ExecutionSteps), publicFacadeRemovalExecutionEscalationResolutionArtifactCount(removalPlan.ExecutionSteps), publicFacadeRemovalExecutionEscalationClosureCheckCount(removalPlan.ExecutionSteps), publicFacadeRemovalExecutionBoundaryGuardCount(removalPlan.ExecutionSteps), publicFacadeRemovalExecutionAuditCheckCount(removalPlan.ExecutionSteps), publicFacadeRemovalExecutionValidationCommandCount(removalPlan.ExecutionSteps), len(removalPlan.BoundaryChecks), publicFacadeRemovalBoundaryValidationCommandCount(removalPlan.BoundaryChecks), len(removalPlan.RecoverySteps), publicFacadeRemovalRecoveryValidationCommandCount(removalPlan.RecoverySteps), len(removalPlan.DocumentationTargets), publicFacadeRemovalDocumentationValidationCommandCount(removalPlan.DocumentationTargets)),
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
		if len(gate.EscalationEvidence) == 0 {
			plan.Warnings = append(plan.Warnings, fmt.Sprintf("public facade removal deletion gate missing escalation evidence: %s", gate.Name))
		}
		for _, evidence := range gate.EscalationEvidence {
			if strings.TrimSpace(evidence) == "" {
				plan.Warnings = append(plan.Warnings, fmt.Sprintf("public facade removal deletion gate has empty escalation evidence: %s", gate.Name))
			}
		}
		if len(gate.EscalationRecipients) == 0 {
			plan.Warnings = append(plan.Warnings, fmt.Sprintf("public facade removal deletion gate missing escalation recipients: %s", gate.Name))
		}
		for _, recipient := range gate.EscalationRecipients {
			if strings.TrimSpace(recipient) == "" {
				plan.Warnings = append(plan.Warnings, fmt.Sprintf("public facade removal deletion gate has empty escalation recipient: %s", gate.Name))
			}
		}
		if len(gate.EscalationHandoffSteps) == 0 {
			plan.Warnings = append(plan.Warnings, fmt.Sprintf("public facade removal deletion gate missing escalation handoff steps: %s", gate.Name))
		}
		for _, step := range gate.EscalationHandoffSteps {
			if strings.TrimSpace(step) == "" {
				plan.Warnings = append(plan.Warnings, fmt.Sprintf("public facade removal deletion gate has empty escalation handoff step: %s", gate.Name))
			}
		}
		if len(gate.EscalationDecisionOptions) == 0 {
			plan.Warnings = append(plan.Warnings, fmt.Sprintf("public facade removal deletion gate missing escalation decision options: %s", gate.Name))
		}
		for _, option := range gate.EscalationDecisionOptions {
			if strings.TrimSpace(option) == "" {
				plan.Warnings = append(plan.Warnings, fmt.Sprintf("public facade removal deletion gate has empty escalation decision option: %s", gate.Name))
			}
		}
		if len(gate.EscalationRetryConditions) == 0 {
			plan.Warnings = append(plan.Warnings, fmt.Sprintf("public facade removal deletion gate missing escalation retry conditions: %s", gate.Name))
		}
		for _, condition := range gate.EscalationRetryConditions {
			if strings.TrimSpace(condition) == "" {
				plan.Warnings = append(plan.Warnings, fmt.Sprintf("public facade removal deletion gate has empty escalation retry condition: %s", gate.Name))
			}
		}
		if len(gate.EscalationStopConditions) == 0 {
			plan.Warnings = append(plan.Warnings, fmt.Sprintf("public facade removal deletion gate missing escalation stop conditions: %s", gate.Name))
		}
		for _, condition := range gate.EscalationStopConditions {
			if strings.TrimSpace(condition) == "" {
				plan.Warnings = append(plan.Warnings, fmt.Sprintf("public facade removal deletion gate has empty escalation stop condition: %s", gate.Name))
			}
		}
		if len(gate.EscalationResolutionArtifacts) == 0 {
			plan.Warnings = append(plan.Warnings, fmt.Sprintf("public facade removal deletion gate missing escalation resolution artifacts: %s", gate.Name))
		}
		for _, artifact := range gate.EscalationResolutionArtifacts {
			if strings.TrimSpace(artifact) == "" {
				plan.Warnings = append(plan.Warnings, fmt.Sprintf("public facade removal deletion gate has empty escalation resolution artifact: %s", gate.Name))
			}
		}
		if len(gate.EscalationClosureChecks) == 0 {
			plan.Warnings = append(plan.Warnings, fmt.Sprintf("public facade removal deletion gate missing escalation closure checks: %s", gate.Name))
		}
		for _, check := range gate.EscalationClosureChecks {
			if strings.TrimSpace(check) == "" {
				plan.Warnings = append(plan.Warnings, fmt.Sprintf("public facade removal deletion gate has empty escalation closure check: %s", gate.Name))
			}
		}
		if len(gate.EscalationReopenConditions) == 0 {
			plan.Warnings = append(plan.Warnings, fmt.Sprintf("public facade removal deletion gate missing escalation reopen conditions: %s", gate.Name))
		}
		for _, condition := range gate.EscalationReopenConditions {
			if strings.TrimSpace(condition) == "" {
				plan.Warnings = append(plan.Warnings, fmt.Sprintf("public facade removal deletion gate has empty escalation reopen condition: %s", gate.Name))
			}
		}
		if len(gate.EscalationLedgerEvents) == 0 {
			plan.Warnings = append(plan.Warnings, fmt.Sprintf("public facade removal deletion gate missing escalation ledger events: %s", gate.Name))
		}
		for _, event := range gate.EscalationLedgerEvents {
			if strings.TrimSpace(event) == "" {
				plan.Warnings = append(plan.Warnings, fmt.Sprintf("public facade removal deletion gate has empty escalation ledger event: %s", gate.Name))
			}
		}
		if len(gate.EscalationStateTransitions) == 0 {
			plan.Warnings = append(plan.Warnings, fmt.Sprintf("public facade removal deletion gate missing escalation state transitions: %s", gate.Name))
		}
		for _, transition := range gate.EscalationStateTransitions {
			if strings.TrimSpace(transition) == "" {
				plan.Warnings = append(plan.Warnings, fmt.Sprintf("public facade removal deletion gate has empty escalation state transition: %s", gate.Name))
			}
		}
		if len(gate.EscalationBoundaryGuards) == 0 {
			plan.Warnings = append(plan.Warnings, fmt.Sprintf("public facade removal deletion gate missing escalation boundary guards: %s", gate.Name))
		}
		for _, guard := range gate.EscalationBoundaryGuards {
			if strings.TrimSpace(guard) == "" {
				plan.Warnings = append(plan.Warnings, fmt.Sprintf("public facade removal deletion gate has empty escalation boundary guard: %s", gate.Name))
			}
		}
		if len(gate.EscalationAuditChecks) == 0 {
			plan.Warnings = append(plan.Warnings, fmt.Sprintf("public facade removal deletion gate missing escalation audit checks: %s", gate.Name))
		}
		for _, check := range gate.EscalationAuditChecks {
			if strings.TrimSpace(check) == "" {
				plan.Warnings = append(plan.Warnings, fmt.Sprintf("public facade removal deletion gate has empty escalation audit check: %s", gate.Name))
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
		if len(step.FailureSignals) == 0 {
			plan.Warnings = append(plan.Warnings, fmt.Sprintf("public facade removal execution step missing failure signals: %s", step.Name))
		}
		for _, signal := range step.FailureSignals {
			if strings.TrimSpace(signal) == "" {
				plan.Warnings = append(plan.Warnings, fmt.Sprintf("public facade removal execution step has empty failure signal: %s", step.Name))
			}
		}
		if len(step.RemediationActions) == 0 {
			plan.Warnings = append(plan.Warnings, fmt.Sprintf("public facade removal execution step missing remediation actions: %s", step.Name))
		}
		for _, action := range step.RemediationActions {
			if strings.TrimSpace(action) == "" {
				plan.Warnings = append(plan.Warnings, fmt.Sprintf("public facade removal execution step has empty remediation action: %s", step.Name))
			}
		}
		if len(step.VerificationArtifacts) == 0 {
			plan.Warnings = append(plan.Warnings, fmt.Sprintf("public facade removal execution step missing verification artifacts: %s", step.Name))
		}
		for _, artifact := range step.VerificationArtifacts {
			if strings.TrimSpace(artifact) == "" {
				plan.Warnings = append(plan.Warnings, fmt.Sprintf("public facade removal execution step has empty verification artifact: %s", step.Name))
			}
		}
		if len(step.LedgerEvents) == 0 {
			plan.Warnings = append(plan.Warnings, fmt.Sprintf("public facade removal execution step missing ledger events: %s", step.Name))
		}
		for _, event := range step.LedgerEvents {
			if strings.TrimSpace(event) == "" {
				plan.Warnings = append(plan.Warnings, fmt.Sprintf("public facade removal execution step has empty ledger event: %s", step.Name))
			}
		}
		if len(step.StateTransitions) == 0 {
			plan.Warnings = append(plan.Warnings, fmt.Sprintf("public facade removal execution step missing state transitions: %s", step.Name))
		}
		for _, transition := range step.StateTransitions {
			if strings.TrimSpace(transition) == "" {
				plan.Warnings = append(plan.Warnings, fmt.Sprintf("public facade removal execution step has empty state transition: %s", step.Name))
			}
		}
		if len(step.EscalationTriggers) == 0 {
			plan.Warnings = append(plan.Warnings, fmt.Sprintf("public facade removal execution step missing escalation triggers: %s", step.Name))
		}
		for _, trigger := range step.EscalationTriggers {
			if strings.TrimSpace(trigger) == "" {
				plan.Warnings = append(plan.Warnings, fmt.Sprintf("public facade removal execution step has empty escalation trigger: %s", step.Name))
			}
		}
		if len(step.EscalationEvidence) == 0 {
			plan.Warnings = append(plan.Warnings, fmt.Sprintf("public facade removal execution step missing escalation evidence: %s", step.Name))
		}
		for _, evidence := range step.EscalationEvidence {
			if strings.TrimSpace(evidence) == "" {
				plan.Warnings = append(plan.Warnings, fmt.Sprintf("public facade removal execution step has empty escalation evidence: %s", step.Name))
			}
		}
		if len(step.EscalationRecipients) == 0 {
			plan.Warnings = append(plan.Warnings, fmt.Sprintf("public facade removal execution step missing escalation recipients: %s", step.Name))
		}
		for _, recipient := range step.EscalationRecipients {
			if strings.TrimSpace(recipient) == "" {
				plan.Warnings = append(plan.Warnings, fmt.Sprintf("public facade removal execution step has empty escalation recipient: %s", step.Name))
			}
		}
		if len(step.EscalationHandoffSteps) == 0 {
			plan.Warnings = append(plan.Warnings, fmt.Sprintf("public facade removal execution step missing escalation handoff steps: %s", step.Name))
		}
		for _, handoff := range step.EscalationHandoffSteps {
			if strings.TrimSpace(handoff) == "" {
				plan.Warnings = append(plan.Warnings, fmt.Sprintf("public facade removal execution step has empty escalation handoff step: %s", step.Name))
			}
		}
		if len(step.EscalationDecisionOptions) == 0 {
			plan.Warnings = append(plan.Warnings, fmt.Sprintf("public facade removal execution step missing escalation decision options: %s", step.Name))
		}
		for _, option := range step.EscalationDecisionOptions {
			if strings.TrimSpace(option) == "" {
				plan.Warnings = append(plan.Warnings, fmt.Sprintf("public facade removal execution step has empty escalation decision option: %s", step.Name))
			}
		}
		if len(step.EscalationRetryConditions) == 0 {
			plan.Warnings = append(plan.Warnings, fmt.Sprintf("public facade removal execution step missing escalation retry conditions: %s", step.Name))
		}
		for _, condition := range step.EscalationRetryConditions {
			if strings.TrimSpace(condition) == "" {
				plan.Warnings = append(plan.Warnings, fmt.Sprintf("public facade removal execution step has empty escalation retry condition: %s", step.Name))
			}
		}
		if len(step.EscalationStopConditions) == 0 {
			plan.Warnings = append(plan.Warnings, fmt.Sprintf("public facade removal execution step missing escalation stop conditions: %s", step.Name))
		}
		for _, condition := range step.EscalationStopConditions {
			if strings.TrimSpace(condition) == "" {
				plan.Warnings = append(plan.Warnings, fmt.Sprintf("public facade removal execution step has empty escalation stop condition: %s", step.Name))
			}
		}
		if len(step.EscalationResolutionArtifacts) == 0 {
			plan.Warnings = append(plan.Warnings, fmt.Sprintf("public facade removal execution step missing escalation resolution artifacts: %s", step.Name))
		}
		for _, artifact := range step.EscalationResolutionArtifacts {
			if strings.TrimSpace(artifact) == "" {
				plan.Warnings = append(plan.Warnings, fmt.Sprintf("public facade removal execution step has empty escalation resolution artifact: %s", step.Name))
			}
		}
		if len(step.EscalationClosureChecks) == 0 {
			plan.Warnings = append(plan.Warnings, fmt.Sprintf("public facade removal execution step missing escalation closure checks: %s", step.Name))
		}
		for _, check := range step.EscalationClosureChecks {
			if strings.TrimSpace(check) == "" {
				plan.Warnings = append(plan.Warnings, fmt.Sprintf("public facade removal execution step has empty escalation closure check: %s", step.Name))
			}
		}
		if len(step.BoundaryGuards) == 0 {
			plan.Warnings = append(plan.Warnings, fmt.Sprintf("public facade removal execution step missing boundary guards: %s", step.Name))
		}
		for _, guard := range step.BoundaryGuards {
			if strings.TrimSpace(guard) == "" {
				plan.Warnings = append(plan.Warnings, fmt.Sprintf("public facade removal execution step has empty boundary guard: %s", step.Name))
			}
		}
		if len(step.AuditChecks) == 0 {
			plan.Warnings = append(plan.Warnings, fmt.Sprintf("public facade removal execution step missing audit checks: %s", step.Name))
		}
		for _, check := range step.AuditChecks {
			if strings.TrimSpace(check) == "" {
				plan.Warnings = append(plan.Warnings, fmt.Sprintf("public facade removal execution step has empty audit check: %s", step.Name))
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
			EscalationEvidence: []string{
				"replacementEntrypoints[] diff or inventory excerpt showing the missing approved alternative",
				"goNativePublicSurface readiness output showing the blocked prerequisite",
				"release-check JSON/text excerpt showing the facade dependency or raw CLI UX risk",
			},
			EscalationRecipients: []string{
				"Mission Commander / product direction owner for replacement-entrypoint approval",
				"Go runtime maintainer responsible for goNativePublicSurface readiness",
				"release gate owner responsible for raw CLI and CI replacement coverage",
			},
			EscalationHandoffSteps: []string{
				"attach replacementEntrypoints[] and goNativePublicSurface excerpts to the escalation packet",
				"state whether the missing alternative is user-facing, deterministic runtime or CI-only",
				"record the retry condition before unblocking delete-public-facade",
			},
			EscalationDecisionOptions: []string{
				"approve a documented replacement entrypoint before retrying removal",
				"defer public facade deletion until Go-native public surface is ready",
				"change product direction and keep the public facade retained",
			},
			EscalationRetryConditions: []string{
				"replacementEntrypoints[] includes an approved Go-native backed public alternative",
				"goNativePublicSurface.ready and facadeRemovalReady are true after the decision",
				"release-check JSON/text shows no unresolved facade dependency for the alternative path",
			},
			EscalationStopConditions: []string{
				"no approved user-facing or deterministic replacement entrypoint exists after escalation",
				"product direction explicitly keeps the public facade retained",
				"retry would delete a public entrypoint without goNativePublicSurface.ready",
			},
			EscalationResolutionArtifacts: []string{
				"approved replacement entrypoint decision record or retained-facade decision",
				"goNativePublicSurface readiness excerpt after the escalation decision",
				"release-check JSON/text excerpt proving the chosen retry or stop path",
			},
			EscalationClosureChecks: []string{
				"resolution artifact references an approved retry or retained-facade decision",
				"replacementEntrypoints[] and goNativePublicSurface evidence are attached to the escalation packet",
				"delete-public-facade remains blocked when the resolved decision is stop or defer",
			},
			EscalationReopenConditions: []string{
				"new replacement entrypoint drift appears after escalation closure",
				"goNativePublicSurface.ready or facadeRemovalReady becomes false before removal",
				"release-check shows a facade dependency reintroduced for the approved alternative",
			},
			EscalationLedgerEvents: []string{
				"replacement-entrypoint-escalation-opened event records missing alternative and owner",
				"replacement-entrypoint-decision-recorded event captures retry or stop decision",
				"replacement-entrypoint-escalation-closed event links release-check readiness evidence",
			},
			EscalationStateTransitions: []string{
				"replacement-entrypoint-opened-to-decision-pending transition occurs when alternative ownership is unresolved",
				"replacement-entrypoint-decision-pending-to-retry-ready transition occurs when goNativePublicSurface readiness is restored",
				"replacement-entrypoint-decision-pending-to-stopped transition occurs when retained-facade decision blocks removal",
			},
			EscalationBoundaryGuards: []string{
				"replacement-entrypoint escalation cannot approve PowerShell runtime fallback as replacement",
				"replacement-entrypoint escalation cannot bypass goNativePublicSurface readiness or command catalog coverage",
				"replacement-entrypoint escalation cannot authorize deleting the public facade without retained or revertable decision",
			},
			EscalationAuditChecks: []string{
				"replacement-entrypoint audit confirms no PowerShell runtime fallback was approved",
				"replacement-entrypoint audit confirms goNativePublicSurface and command catalog evidence are linked",
				"replacement-entrypoint audit confirms retained or revertable decision matches removal state",
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
			EscalationEvidence: []string{
				"unclassifiedReferences[] or migrationTargets[] excerpt for the unresolved public reference",
				"documentationTargets[] diff showing the conflicted communication surface",
				"preservation rationale showing why the reference cannot be migrated locally",
			},
			EscalationRecipients: []string{
				"Mission Commander / documentation owner for public communication conflicts",
				"Go runtime maintainer responsible for migration target classification",
				"release maintainer responsible for historical-preservation approval",
			},
			EscalationHandoffSteps: []string{
				"attach the unresolved migration target list and documentation diff to the escalation packet",
				"state which references are rewritten, retired or preserved as historical context",
				"record the owner decision required before reference migration can be retried",
			},
			EscalationDecisionOptions: []string{
				"rewrite the unresolved public references before retrying removal",
				"preserve the references as historical context with owner approval",
				"defer deletion until documentation targets are synchronized",
			},
			EscalationRetryConditions: []string{
				"migrationTargets[] are rewritten, retired or explicitly preserved as historical context",
				"documentationTargets[] are synchronized with the approved reference decision",
				"removalImpact.unclassifiedReferences remains zero after retry classification",
			},
			EscalationStopConditions: []string{
				"owner rejects rewriting or preserving unresolved public facade references",
				"documentationTargets[] cannot be synchronized without a product direction change",
				"removalImpact.unclassifiedReferences remains non-zero after retry classification",
			},
			EscalationResolutionArtifacts: []string{
				"owner decision record for rewriting, preserving or deferring unresolved references",
				"documentationTargets[] synchronization diff or explicit defer rationale",
				"removalImpact classification excerpt after escalation resolution",
			},
			EscalationClosureChecks: []string{
				"owner decision is linked to every unresolved migrationTargets[] row",
				"documentationTargets[] diff or defer rationale is attached before closing escalation",
				"unclassifiedReferences=0 or stop/defer rationale is recorded before retry",
			},
			EscalationReopenConditions: []string{
				"new unclassified public facade reference appears after escalation closure",
				"documentationTargets[] drift from the resolved migration decision",
				"migrationTargets[] gain unresolved required entries before removal",
			},
			EscalationLedgerEvents: []string{
				"public-reference-escalation-opened event records unresolved migration targets",
				"public-reference-decision-recorded event captures rewrite, preserve or defer decision",
				"public-reference-escalation-closed event links documentation sync and classification evidence",
			},
			EscalationStateTransitions: []string{
				"public-reference-opened-to-decision-pending transition occurs when unresolved migration targets remain",
				"public-reference-decision-pending-to-retry-ready transition occurs when references are rewritten or preserved",
				"public-reference-decision-pending-to-stopped transition occurs when documentation synchronization requires product direction",
			},
			EscalationBoundaryGuards: []string{
				"public-reference escalation cannot silently drop unresolved user-facing documentation references",
				"public-reference escalation cannot rewrite historical roadmap references without preservation rationale",
				"public-reference escalation cannot mark non-zero unclassifiedReferences as retry-ready",
			},
			EscalationAuditChecks: []string{
				"public-reference audit confirms every migrationTargets[] row has rewritten, preserved or deferred disposition",
				"public-reference audit confirms historical references keep preservation rationale",
				"public-reference audit confirms unclassifiedReferences remains zero before retry",
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
			EscalationEvidence: []string{
				"smokeMigrationTargets[] excerpt showing the remaining facade compatibility dependency",
				"test coverage comparison showing no Go-native replacement for the retired smoke intent",
				"gateProfile or ciReleaseGate excerpt showing any facade-smoke.ps1 default dependency",
			},
			EscalationRecipients: []string{
				"release gate owner responsible for retiring facade compatibility smoke",
				"Go test owner responsible for replacement package coverage",
				"Mission Commander / CI maintainer for default release gate dependency changes",
			},
			EscalationHandoffSteps: []string{
				"attach smokeMigrationTargets[] and replacement test coverage excerpts to the escalation packet",
				"state whether the blocker is compatibility assertion removal or missing Go-native coverage",
				"record the release gate change needed before facade smoke can be retired",
			},
			EscalationDecisionOptions: []string{
				"retire facade compatibility assertions and use Go-native smoke coverage",
				"add replacement Go-native package coverage before retrying removal",
				"keep facade smoke as a non-default compatibility check for this cycle",
			},
			EscalationRetryConditions: []string{
				"smokeMigrationTargets[] no longer require public facade compatibility assertions",
				"replacement Go-native package or smoke coverage is present for the retired intent",
				"gateProfile and ciReleaseGate have no default facade-smoke.ps1 dependency",
			},
			EscalationStopConditions: []string{
				"replacement Go-native coverage cannot be added within the removal batch scope",
				"release gate owner requires facade-smoke.ps1 as a default check",
				"facade compatibility assertions remain required for supported workflows",
			},
			EscalationResolutionArtifacts: []string{
				"release gate owner decision for retiring, replacing or preserving facade smoke",
				"replacement Go-native coverage transcript or scoped deferral record",
				"gateProfile or ciReleaseGate excerpt confirming default smoke dependency disposition",
			},
			EscalationClosureChecks: []string{
				"smokeMigrationTargets[] disposition is recorded for every facade compatibility target",
				"replacement coverage transcript or non-default smoke decision is attached",
				"ciReleaseGate and gateProfile dependency disposition is verified before closing escalation",
			},
			EscalationReopenConditions: []string{
				"facade-smoke.ps1 returns to a default release gate or CI path",
				"replacement Go-native coverage for the retired smoke intent is removed",
				"supported workflow requires facade compatibility assertions after closure",
			},
			EscalationLedgerEvents: []string{
				"facade-smoke-escalation-opened event records remaining smoke dependency",
				"facade-smoke-decision-recorded event captures retire, replace or non-default decision",
				"facade-smoke-escalation-closed event links coverage or CI dependency evidence",
			},
			EscalationStateTransitions: []string{
				"facade-smoke-opened-to-decision-pending transition occurs when smoke compatibility remains required",
				"facade-smoke-decision-pending-to-retry-ready transition occurs when replacement Go-native coverage is attached",
				"facade-smoke-decision-pending-to-stopped transition occurs when default facade smoke remains required",
			},
			EscalationBoundaryGuards: []string{
				"facade-smoke escalation cannot restore default PowerShell facade smoke in the release gate",
				"facade-smoke escalation cannot remove compatibility coverage without Go-native replacement or owner decision",
				"facade-smoke escalation cannot classify default CI dependency as non-default without evidence",
			},
			EscalationAuditChecks: []string{
				"facade-smoke audit confirms no default release gate depends on facade-smoke.ps1",
				"facade-smoke audit confirms replacement Go-native coverage or owner decision is attached",
				"facade-smoke audit confirms compatibility assertions are retired or explicitly non-default",
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
			EscalationEvidence: []string{
				"recoverySteps[] excerpt showing the missing restore or revert path",
				"documentationTargets[] excerpt showing the ownerless public communication surface",
				"git diff or plan excerpt showing why the removal cannot stay separately revertable",
			},
			EscalationRecipients: []string{
				"Mission Commander / release owner for separately revertable removal approval",
				"documentation owner responsible for public removal communication surfaces",
				"Go runtime maintainer responsible for recovery path validation",
			},
			EscalationHandoffSteps: []string{
				"attach recoverySteps[] and documentationTargets[] excerpts to the escalation packet",
				"state whether the missing path is restore, docs synchronization or revert isolation",
				"record the recovery plan decision required before deletion can proceed",
			},
			EscalationDecisionOptions: []string{
				"add missing recovery steps before deleting the public facade",
				"split the removal into a separately revertable commit",
				"defer deletion until public communication surfaces have owners",
			},
			EscalationRetryConditions: []string{
				"recoverySteps[] cover public facade, synchronized docs and release gate restoration",
				"documentationTargets[] have owners or synchronized removal communication actions",
				"the deletion remains isolated in a separately revertable commit plan",
			},
			EscalationStopConditions: []string{
				"restore-public-facade or synchronized docs recovery path has no owner",
				"the removal cannot be isolated in a separately revertable commit",
				"recovery requires schema migration or external side effects",
			},
			EscalationResolutionArtifacts: []string{
				"recovery path owner decision for restore-public-facade and synchronized docs",
				"separately revertable commit plan or stop rationale",
				"release-check validation transcript for the approved recovery path",
			},
			EscalationClosureChecks: []string{
				"restore-public-facade and synchronized docs owners are recorded before closure",
				"separate revertable commit plan or stop rationale is attached",
				"recovery validation command transcript is linked before retrying delete-public-facade",
			},
			EscalationReopenConditions: []string{
				"restore-public-facade or synchronized docs owner becomes unavailable",
				"removal can no longer be isolated in a separately revertable commit",
				"recovery path gains schema migration or external-effect requirements",
			},
			EscalationLedgerEvents: []string{
				"recovery-path-escalation-opened event records missing owner or revert path",
				"recovery-path-decision-recorded event captures restore, docs or revert decision",
				"recovery-path-escalation-closed event links recovery validation evidence",
			},
			EscalationStateTransitions: []string{
				"recovery-path-opened-to-decision-pending transition occurs when restore owner or revert path is missing",
				"recovery-path-decision-pending-to-retry-ready transition occurs when recovery validation evidence is attached",
				"recovery-path-decision-pending-to-stopped transition occurs when recovery requires schema migration or external effects",
			},
			EscalationBoundaryGuards: []string{
				"recovery-path escalation cannot proceed when restore-public-facade owner is missing",
				"recovery-path escalation cannot merge facade deletion with unrelated schema migration",
				"recovery-path escalation cannot depend on external side effects for local revert readiness",
			},
			EscalationAuditChecks: []string{
				"recovery-path audit confirms restore-public-facade and synchronized docs owners are recorded",
				"recovery-path audit confirms deletion remains isolated in a separately revertable commit plan",
				"recovery-path audit confirms no recovery step requires schema migration or external effects",
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
			EscalationEvidence: []string{
				"release-check JSON/text excerpt showing the failing readiness signal",
				"go test, go vet or git diff --check output that cannot be fixed locally",
				"CI inventory or validation transcript showing required external effects or heavy-tool execution",
			},
			EscalationRecipients: []string{
				"release maintainer responsible for release-check readiness failures",
				"Go runtime maintainer responsible for test/vet/diff failures",
				"Mission Commander for any required external effect or heavy-tool escalation",
			},
			EscalationHandoffSteps: []string{
				"attach release-check JSON/text and failing command output to the escalation packet",
				"state whether the failure is local fixable, CI-only or requires an external effect",
				"record the exact command set that must pass before rerun-release-gate is unblocked",
			},
			EscalationDecisionOptions: []string{
				"fix local release-check, test, vet or diff failures before retrying",
				"defer deletion until CI release gate is green",
				"escalate external-effect or heavy-tool requirements before proceeding",
			},
			EscalationRetryConditions: []string{
				"release-check JSON/text returns ready=true after remediation",
				"go test ./... and go vet ./... pass without local failures",
				"git diff --check has no whitespace errors and CI release gate is green or not required for local retry",
			},
			EscalationStopConditions: []string{
				"release-check remains not ready after local remediation",
				"go test, go vet or git diff failures cannot be fixed inside the removal batch",
				"CI or external-effect requirements exceed the authorized local scope",
			},
			EscalationResolutionArtifacts: []string{
				"local remediation transcript for release-check, go test, go vet and git diff",
				"CI readiness evidence or explicit external-scope escalation record",
				"final retry or stop decision record attached to the release gate packet",
			},
			EscalationClosureChecks: []string{
				"release-check, go test, go vet and git diff transcript is attached",
				"CI or external-scope decision is recorded when local retry is insufficient",
				"rerun-release-gate remains blocked until the retry or stop decision is closed",
			},
			EscalationReopenConditions: []string{
				"release-check ready=false after escalation closure",
				"go test, go vet or git diff starts failing before removal",
				"CI or external-effect requirement appears after local closure",
			},
			EscalationLedgerEvents: []string{
				"release-gate-escalation-opened event records failing release-check, test, vet or diff command",
				"release-gate-decision-recorded event captures retry, stop or external-scope decision",
				"release-gate-escalation-closed event links final release gate transcript",
			},
			EscalationStateTransitions: []string{
				"release-gate-opened-to-decision-pending transition occurs when local release gate evidence fails",
				"release-gate-decision-pending-to-retry-ready transition occurs when release-check, test, vet and diff pass",
				"release-gate-decision-pending-to-stopped transition occurs when CI or external scope exceeds local authorization",
			},
			EscalationBoundaryGuards: []string{
				"release-gate escalation cannot override failing release-check, go test, go vet or diff as green",
				"release-gate escalation cannot depend on CI-only external effects for local retry without owner decision",
				"release-gate escalation cannot mark heavy-tool or authority-confirmed action as release gate remediation",
			},
			EscalationAuditChecks: []string{
				"release-gate audit confirms release-check JSON/text is ready after escalation resolution",
				"release-gate audit confirms go test, go vet and git diff transcripts are attached",
				"release-gate audit confirms no heavy-tool or authority-confirmed action was used as remediation",
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

func publicFacadeRemovalDeletionGateEscalationEvidenceCount(gates []PublicFacadeRemovalDeletionGate) int {
	count := 0
	for _, gate := range gates {
		count += len(gate.EscalationEvidence)
	}
	return count
}

func publicFacadeRemovalDeletionGateEscalationRecipientCount(gates []PublicFacadeRemovalDeletionGate) int {
	count := 0
	for _, gate := range gates {
		count += len(gate.EscalationRecipients)
	}
	return count
}

func publicFacadeRemovalDeletionGateEscalationHandoffStepCount(gates []PublicFacadeRemovalDeletionGate) int {
	count := 0
	for _, gate := range gates {
		count += len(gate.EscalationHandoffSteps)
	}
	return count
}

func publicFacadeRemovalDeletionGateEscalationDecisionOptionCount(gates []PublicFacadeRemovalDeletionGate) int {
	count := 0
	for _, gate := range gates {
		count += len(gate.EscalationDecisionOptions)
	}
	return count
}

func publicFacadeRemovalDeletionGateEscalationRetryConditionCount(gates []PublicFacadeRemovalDeletionGate) int {
	count := 0
	for _, gate := range gates {
		count += len(gate.EscalationRetryConditions)
	}
	return count
}

func publicFacadeRemovalDeletionGateEscalationStopConditionCount(gates []PublicFacadeRemovalDeletionGate) int {
	count := 0
	for _, gate := range gates {
		count += len(gate.EscalationStopConditions)
	}
	return count
}

func publicFacadeRemovalDeletionGateEscalationResolutionArtifactCount(gates []PublicFacadeRemovalDeletionGate) int {
	count := 0
	for _, gate := range gates {
		count += len(gate.EscalationResolutionArtifacts)
	}
	return count
}

func publicFacadeRemovalDeletionGateEscalationClosureCheckCount(gates []PublicFacadeRemovalDeletionGate) int {
	count := 0
	for _, gate := range gates {
		count += len(gate.EscalationClosureChecks)
	}
	return count
}

func publicFacadeRemovalDeletionGateEscalationReopenConditionCount(gates []PublicFacadeRemovalDeletionGate) int {
	count := 0
	for _, gate := range gates {
		count += len(gate.EscalationReopenConditions)
	}
	return count
}

func publicFacadeRemovalDeletionGateEscalationLedgerEventCount(gates []PublicFacadeRemovalDeletionGate) int {
	count := 0
	for _, gate := range gates {
		count += len(gate.EscalationLedgerEvents)
	}
	return count
}

func publicFacadeRemovalDeletionGateEscalationStateTransitionCount(gates []PublicFacadeRemovalDeletionGate) int {
	count := 0
	for _, gate := range gates {
		count += len(gate.EscalationStateTransitions)
	}
	return count
}

func publicFacadeRemovalDeletionGateEscalationBoundaryGuardCount(gates []PublicFacadeRemovalDeletionGate) int {
	count := 0
	for _, gate := range gates {
		count += len(gate.EscalationBoundaryGuards)
	}
	return count
}

func publicFacadeRemovalDeletionGateEscalationAuditCheckCount(gates []PublicFacadeRemovalDeletionGate) int {
	count := 0
	for _, gate := range gates {
		count += len(gate.EscalationAuditChecks)
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
			Name:            "verify-go-native-alternative",
			Action:          "verify /rekit Mission Control and direct Go CLI alternatives before removing the public facade",
			Required:        true,
			DependsOn:       []string{"go-native-public-surface", "public-facade-retained-boundary"},
			InputInventory:  []string{"goNativePublicSurface", "powerShellDeprecation.publicFacade"},
			OutputArtifacts: []string{"release-check JSON/text showing Go-native public surface ready"},
			FailureSignals: []string{
				"goNativePublicSurface.ready or facadeRemovalReady is false",
				"replacementEntrypoints[] omits canonical /rekit skill or direct Go CLI alternative",
				"unsupported command diagnostic no longer exposes the Go-native alternative pattern",
			},
			RemediationActions: []string{
				"restore or update replacementEntrypoints[] until canonical /rekit and direct Go CLI alternatives are present",
				"fix goNativePublicSurface readiness blockers before retrying deletion",
				"repair unsupported command diagnostic so it points to the Go-native command pattern",
			},
			VerificationArtifacts: []string{
				"release-check JSON/text showing goNativePublicSurface.ready and facadeRemovalReady true",
				"replacementEntrypoints[] evidence for canonical /rekit skill and direct Go CLI alternative",
				"unsupported command diagnostic transcript pointing to the Go-native command pattern",
			},
			LedgerEvents: []string{
				"execution.verify-go-native-alternative.started records replacement entrypoint inventory and Go-native readiness inputs",
				"execution.verify-go-native-alternative.failed records failureSignals[], remediationActions[] and blocked deletion steps",
				"execution.verify-go-native-alternative.completed records verificationArtifacts[] proving /rekit and direct Go CLI alternatives",
			},
			StateTransitions: []string{
				"verify-go-native-alternative-pending-to-running transition occurs after replacement entrypoint inventory is captured",
				"verify-go-native-alternative-running-to-blocked transition occurs when Go-native readiness or diagnostic evidence fails",
				"verify-go-native-alternative-running-to-completed transition occurs when replacement verification artifacts are attached",
			},
			EscalationTriggers: []string{
				"escalate when canonical /rekit or direct Go CLI alternative cannot be proven from inventory",
				"escalate when unsupported command diagnostic semantics require product direction before deletion",
				"escalate when replacement evidence depends on PowerShell runtime or facade fallback",
			},
			EscalationEvidence: []string{
				"replacementEntrypoints[] snapshot with canonical /rekit and direct Go CLI dispositions",
				"goNativePublicSurface and facadeRemovalReady release-check transcript",
				"unsupported command diagnostic transcript showing the Go-native alternative",
			},
			EscalationRecipients: []string{
				"Mission Commander for replacement entrypoint readiness decision",
				"runtime owner for Go-native public surface and unsupported diagnostic evidence",
				"documentation owner for /rekit and direct Go CLI public entrypoint wording",
			},
			EscalationHandoffSteps: []string{
				"prepare replacement entrypoint escalation packet with readiness summary and evidence links",
				"route packet to Mission Commander, runtime owner and documentation owner",
				"record owner decision and retry or stop disposition before delete-public-facade",
			},
			EscalationDecisionOptions: []string{
				"approve deletion readiness when replacement entrypoints and diagnostics are proven",
				"defer deletion until Go-native public surface or documentation evidence is repaired",
				"retain public facade when replacement UX remains a product-direction blocker",
			},
			EscalationRetryConditions: []string{
				"retry after Go-native public surface readiness is restored and recorded",
				"retry after canonical /rekit and direct Go CLI replacement evidence is refreshed",
				"retry after unsupported command diagnostic transcript proves the Go-native alternative",
			},
			EscalationStopConditions: []string{
				"stop when no approved Go-native alternative can replace the public facade UX",
				"stop when unsupported command diagnostics cannot be preserved without facade fallback",
				"stop when replacement evidence depends on PowerShell runtime or public facade behavior",
			},
			EscalationResolutionArtifacts: []string{
				"replacement entrypoint decision record with approved retry or stop disposition",
				"Go-native public surface transcript attached to the escalation resolution record",
				"unsupported diagnostic evidence or product-direction blocker linked from resolution",
			},
			EscalationClosureChecks: []string{
				"closure confirms replacement entrypoint decision record is attached",
				"closure confirms Go-native public surface and unsupported diagnostic evidence are current",
				"closure confirms retry or stop disposition is reflected before downstream steps proceed",
			},
			BoundaryGuards: []string{
				"must not accept PowerShell runtime or facade fallback as replacement evidence",
				"must confirm canonical /rekit skill and direct Go CLI alternatives remain documented",
				"must keep unsupported command diagnostic available before deletion is unblocked",
			},
			AuditChecks: []string{
				"audit confirms Go-native public surface and replacement entrypoints are ready",
				"audit confirms no PowerShell runtime or facade fallback was accepted as replacement evidence",
				"audit confirms unsupported command diagnostic still points to the Go-native alternative",
			},
			ValidationCommands:      commands,
			AllowsPowerShellRuntime: false,
			AllowsExternalEffects:   false,
		},
		{
			Name:            "migrate-public-references",
			Action:          "process every public facade migration target before deleting the facade",
			Required:        true,
			DependsOn:       []string{"removalImpact.migrationTargets", "removalPlan.documentationTargets"},
			InputInventory:  []string{"publicFacadeRemoval.removalImpact.migrationTargets", "publicFacadeRemoval.removalPlan.documentationTargets"},
			OutputArtifacts: []string{"updated docs, tests and release inventory references"},
			FailureSignals: []string{
				"migrationTargets[] contains unresolved required public facade references",
				"documentationTargets[] drift from migrated or preserved reference dispositions",
				"removalImpact.unclassifiedReferences is non-zero",
			},
			RemediationActions: []string{
				"rewrite, preserve with rationale or defer every unresolved public facade reference",
				"resynchronize documentationTargets[] with processed migration dispositions",
				"resolve unclassified removalImpact references before delete-public-facade",
			},
			VerificationArtifacts: []string{
				"processed migrationTargets[] disposition table for every public facade reference",
				"documentationTargets[] synchronization evidence for rewritten or preserved references",
				"release-check removalImpact transcript showing unclassifiedReferences=0",
			},
			LedgerEvents: []string{
				"execution.migrate-public-references.started records migrationTargets[] and documentationTargets[] snapshot",
				"execution.migrate-public-references.failed records unresolved references, remediationActions[] and deferral owner",
				"execution.migrate-public-references.completed records processed dispositions and unclassifiedReferences=0 evidence",
			},
			StateTransitions: []string{
				"migrate-public-references-pending-to-running transition occurs after migrationTargets[] and documentationTargets[] are snapshotted",
				"migrate-public-references-running-to-blocked transition occurs when unresolved or unclassified references remain",
				"migrate-public-references-running-to-completed transition occurs when all reference dispositions and docs evidence are attached",
			},
			EscalationTriggers: []string{
				"escalate when unresolved public facade references require owner disposition",
				"escalate when documentationTargets[] conflict with historical preservation requirements",
				"escalate when unclassifiedReferences cannot be categorized by local inventory",
			},
			EscalationEvidence: []string{
				"unresolved public facade reference list with owner and disposition requirement",
				"documentationTargets[] diff or preservation rationale requiring decision",
				"removalImpact unclassifiedReferences transcript and categorization notes",
			},
			EscalationRecipients: []string{
				"documentation owner for user-facing public facade reference disposition",
				"Mission Commander for historical preservation or deferred migration decision",
				"runtime owner for removalImpact categorization and unclassified reference resolution",
			},
			EscalationHandoffSteps: []string{
				"prepare unresolved reference escalation packet with migration and documentation evidence",
				"route packet to documentation owner, Mission Commander and runtime owner",
				"record reference disposition decision before retrying migration or stopping removal",
			},
			EscalationDecisionOptions: []string{
				"approve migration completion when every reference has rewritten or preserved disposition",
				"defer deletion until unresolved documentation targets receive owner disposition",
				"retain or postpone facade removal when historical references require product direction",
			},
			EscalationRetryConditions: []string{
				"retry after every unresolved public reference has owner disposition",
				"retry after documentationTargets[] are synchronized with migrated or preserved references",
				"retry after removalImpact.unclassifiedReferences returns to zero",
			},
			EscalationStopConditions: []string{
				"stop when required user-facing references cannot be migrated or preserved with owner approval",
				"stop when removalImpact.unclassifiedReferences remains non-zero after local classification",
				"stop when historical preservation or product direction blocks public facade removal",
			},
			EscalationResolutionArtifacts: []string{
				"public reference disposition table with owner-approved rewrite, preservation or stop outcome",
				"documentationTargets synchronization diff or historical preservation rationale",
				"removalImpact transcript showing unclassifiedReferences=0 after resolution",
			},
			EscalationClosureChecks: []string{
				"closure confirms every public reference has rewritten, preserved or stop disposition",
				"closure confirms documentationTargets[] and migrationTargets[] are synchronized",
				"closure confirms removalImpact.unclassifiedReferences remains zero",
			},
			BoundaryGuards: []string{
				"must not remove user-facing public facade references without migration or historical-preservation decision",
				"must keep documentationTargets[] synchronized with every migrated reference category",
				"must keep removalImpact.unclassifiedReferences at zero before delete-public-facade",
			},
			AuditChecks: []string{
				"audit confirms every migrationTargets[] row has a processed disposition",
				"audit confirms documentationTargets[] are synchronized with migrated or preserved references",
				"audit confirms removalImpact.unclassifiedReferences remains zero before deletion",
			},
			ValidationCommands:      commands,
			AllowsPowerShellRuntime: false,
			AllowsExternalEffects:   false,
		},
		{
			Name:            "retire-facade-smoke",
			Action:          "rewrite or retire facade compatibility and dependent smoke assertions",
			Required:        true,
			DependsOn:       []string{"removalImpact.smokeMigrationTargets"},
			InputInventory:  []string{"publicFacadeRemoval.removalImpact.smokeMigrationTargets"},
			OutputArtifacts: []string{"Go-native tests or explicitly retired facade assertions"},
			FailureSignals: []string{
				"facade-smoke.ps1 remains in default release gate or CI paths",
				"replacement Go-native smoke coverage is missing for retired facade assertions",
				"smokeMigrationTargets[] still require facade compatibility assertions",
			},
			RemediationActions: []string{
				"remove facade-smoke.ps1 from default release gate or mark it non-default compatibility only",
				"add Go-native coverage or owner-approved retirement record for each smoke intent",
				"record smokeMigrationTargets[] dispositions before deleting the public facade",
			},
			VerificationArtifacts: []string{
				"release gate or CI inventory transcript showing facade-smoke.ps1 absent from defaults",
				"Go-native replacement test transcript or explicit smoke retirement record",
				"smokeMigrationTargets[] disposition evidence before delete-public-facade",
			},
			LedgerEvents: []string{
				"execution.retire-facade-smoke.started records smokeMigrationTargets[] and default release gate inputs",
				"execution.retire-facade-smoke.failed records remaining facade smoke dependencies and remediationActions[]",
				"execution.retire-facade-smoke.completed records Go-native replacement coverage or explicit retirement evidence",
			},
			StateTransitions: []string{
				"retire-facade-smoke-pending-to-running transition occurs after smokeMigrationTargets[] and release gate inputs are captured",
				"retire-facade-smoke-running-to-blocked transition occurs when default facade smoke dependencies remain",
				"retire-facade-smoke-running-to-completed transition occurs when Go-native smoke replacement or retirement evidence is attached",
			},
			EscalationTriggers: []string{
				"escalate when a default release gate still requires facade-smoke.ps1",
				"escalate when Go-native replacement coverage cannot preserve required smoke intent",
				"escalate when smokeMigrationTargets[] require compatibility policy decision",
			},
			EscalationEvidence: []string{
				"default release gate transcript showing remaining facade-smoke.ps1 dependency",
				"smoke intent coverage gap analysis for Go-native replacement",
				"smokeMigrationTargets[] rows requiring compatibility decision",
			},
			EscalationRecipients: []string{
				"release gate owner for default smoke path retirement decision",
				"runtime test owner for Go-native replacement coverage evidence",
				"Mission Commander for compatibility-only facade smoke preservation decision",
			},
			EscalationHandoffSteps: []string{
				"prepare smoke retirement escalation packet with release gate and coverage evidence",
				"route packet to release gate owner, runtime test owner and Mission Commander",
				"record compatibility or replacement coverage decision before delete-public-facade",
			},
			EscalationDecisionOptions: []string{
				"approve smoke retirement when Go-native coverage or retirement record is attached",
				"defer deletion until default facade smoke dependencies are removed from release gates",
				"retain compatibility smoke as non-default when coverage policy requires preservation",
			},
			EscalationRetryConditions: []string{
				"retry after facade-smoke.ps1 is absent from default release gate and CI paths",
				"retry after Go-native replacement coverage or explicit retirement record is attached",
				"retry after smokeMigrationTargets[] dispositions are recorded",
			},
			EscalationStopConditions: []string{
				"stop when default release gate policy still requires facade-smoke.ps1 compatibility coverage",
				"stop when required smoke intent cannot be replaced by Go-native coverage or retirement record",
				"stop when smokeMigrationTargets[] require retained public facade compatibility",
			},
			EscalationResolutionArtifacts: []string{
				"facade smoke retirement or non-default compatibility decision record",
				"Go-native smoke coverage transcript or explicit smoke retirement approval",
				"smokeMigrationTargets disposition table attached to the escalation resolution",
			},
			EscalationClosureChecks: []string{
				"closure confirms facade smoke retirement or compatibility decision is recorded",
				"closure confirms Go-native smoke coverage or retirement approval is attached",
				"closure confirms smokeMigrationTargets[] dispositions are visible before deletion",
			},
			BoundaryGuards: []string{
				"must not keep facade-smoke.ps1 in default release gate or CI path",
				"must require Go-native replacement coverage or explicit non-default compatibility decision",
				"must keep smokeMigrationTargets[] disposition visible before delete-public-facade",
			},
			AuditChecks: []string{
				"audit confirms facade-smoke.ps1 is absent from default release gate and CI paths",
				"audit confirms Go-native replacement coverage or explicit non-default smoke decision is attached",
				"audit confirms smokeMigrationTargets[] dispositions are visible before deletion",
			},
			ValidationCommands:      commands,
			AllowsPowerShellRuntime: false,
			AllowsExternalEffects:   false,
		},
		{
			Name:            "delete-public-facade",
			Action:          "delete rekit/rekit.ps1 only after alternatives, references and smoke targets are resolved in the independent removal batch",
			Required:        true,
			DependsOn:       []string{"verify-go-native-alternative", "migrate-public-references", "retire-facade-smoke"},
			InputInventory:  []string{"publicFacadeRemoval.removalPlan.recoverySteps", "publicFacadeRemoval.removalImpact.workItems"},
			OutputArtifacts: []string{"separate revertable commit removing rekit/rekit.ps1"},
			FailureSignals: []string{
				"delete-public-facade is attempted outside the independent removal batch",
				"rekit/rekit.ps1 removal is not isolated in a separately revertable commit",
				"deletion diff includes schema migration, heavy-tool action or external effect",
			},
			RemediationActions: []string{
				"move deletion work into the independent public facade removal batch when mixed with other changes",
				"split rekit/rekit.ps1 deletion into a separately revertable commit",
				"remove schema migration, heavy-tool action or external effect from the deletion diff",
			},
			VerificationArtifacts: []string{
				"independent public facade removal batch plan entry",
				"separately revertable commit diff removing rekit/rekit.ps1",
				"diff review transcript showing no schema migration, heavy-tool action or external effect",
			},
			LedgerEvents: []string{
				"execution.delete-public-facade.started records independent removal batch and revert owner",
				"execution.delete-public-facade.failed records mixed diff, missing revert path or boundary violation",
				"execution.delete-public-facade.completed records separately revertable rekit/rekit.ps1 deletion commit",
			},
			StateTransitions: []string{
				"delete-public-facade-pending-to-running transition occurs only inside the independent public facade removal batch",
				"delete-public-facade-running-to-blocked transition occurs when deletion is mixed with unrelated migration or effects",
				"delete-public-facade-running-to-completed transition occurs when separately revertable facade deletion commit exists",
			},
			EscalationTriggers: []string{
				"escalate when deletion cannot remain isolated to the independent removal batch",
				"escalate when restore-public-facade recovery owner or revert path is missing",
				"escalate when deletion diff would include schema migration, heavy-tool action or external effect",
			},
			EscalationEvidence: []string{
				"public facade deletion diff showing mixed unrelated changes or boundary violation",
				"restore-public-facade recovery owner and revert path evidence",
				"schema migration, heavy-tool action or external effect evidence in deletion plan",
			},
			EscalationRecipients: []string{
				"Mission Commander for independent public facade removal batch authorization",
				"runtime owner for isolated rekit/rekit.ps1 deletion and revert path evidence",
				"release owner for schema migration, heavy-tool and external-effect boundary review",
			},
			EscalationHandoffSteps: []string{
				"prepare deletion boundary escalation packet with diff, recovery and effect evidence",
				"route packet to Mission Commander, runtime owner and release owner",
				"record independent removal batch authorization or stop decision before deletion",
			},
			EscalationDecisionOptions: []string{
				"approve isolated public facade deletion in the independent removal batch",
				"split or defer deletion until unrelated schema, heavy-tool or external-effect changes are removed",
				"stop deletion and retain facade when recovery path or revert isolation is unavailable",
			},
			EscalationRetryConditions: []string{
				"retry after deletion diff is isolated to the independent removal batch",
				"retry after restore-public-facade recovery path and revert owner are recorded",
				"retry after schema migration, heavy-tool and external-effect changes are removed from the diff",
			},
			EscalationStopConditions: []string{
				"stop when deletion cannot remain isolated to an independent revertable removal batch",
				"stop when restore-public-facade recovery path or revert owner is unavailable",
				"stop when deletion requires schema migration, heavy-tool action or external effect",
			},
			EscalationResolutionArtifacts: []string{
				"independent public facade removal batch authorization or stop decision record",
				"isolated deletion diff and restore-public-facade recovery evidence",
				"boundary review transcript proving no schema, heavy-tool or external-effect change",
			},
			EscalationClosureChecks: []string{
				"closure confirms independent removal batch authorization or stop decision is recorded",
				"closure confirms deletion diff remains isolated and restore-public-facade recovery evidence is attached",
				"closure confirms no schema migration, heavy-tool action or external effect is bundled",
			},
			BoundaryGuards: []string{
				"must run only in the independent public facade removal batch",
				"must remain separately revertable through restore-public-facade recovery step",
				"must not include unrelated schema migration, heavy-tool action or external effect",
			},
			AuditChecks: []string{
				"audit confirms deletion is isolated to the independent public facade removal batch",
				"audit confirms restore-public-facade recovery path can revert the deletion",
				"audit confirms no schema migration, heavy-tool action or external effect is bundled with deletion",
			},
			ValidationCommands:      commands,
			AllowsPowerShellRuntime: false,
			AllowsExternalEffects:   false,
		},
		{
			Name:            "rerun-release-gate",
			Action:          "rerun Go-native release gate and confirm public facade removal inventory and handoff output are synchronized",
			Required:        true,
			DependsOn:       []string{"delete-public-facade", "restore-public-facade recovery path documented"},
			InputInventory:  []string{"publicFacadeRemoval.removalPlan.recoverySteps", "releaseHandoff.signals"},
			OutputArtifacts: []string{"release-check JSON/text, go test, go vet and git diff --check results"},
			FailureSignals: []string{
				"release-check JSON/text is not ready after the deletion result",
				"go test, go vet or git diff --check fails after deletion",
				"releaseHandoff.signals[] omit synchronized public facade removal readiness",
			},
			RemediationActions: []string{
				"rerun release-check JSON/text and fix readiness drift before closure",
				"fix failing go test, go vet or git diff --check results and attach transcripts",
				"resynchronize releaseHandoff.signals[] with public facade removal readiness",
			},
			VerificationArtifacts: []string{
				"release-check JSON/text transcript after the deletion result",
				"go test, go vet and git diff --check transcripts after deletion",
				"releaseHandoff.signals[] transcript synchronized with public facade removal readiness",
			},
			LedgerEvents: []string{
				"execution.rerun-release-gate.started records post-deletion release gate command set",
				"execution.rerun-release-gate.failed records failing command transcript and remediationActions[]",
				"execution.rerun-release-gate.completed records synchronized release-check, test, vet and diff transcripts",
			},
			StateTransitions: []string{
				"rerun-release-gate-pending-to-running transition occurs after deletion result and command set are recorded",
				"rerun-release-gate-running-to-blocked transition occurs when any required local validation command fails",
				"rerun-release-gate-running-to-completed transition occurs when synchronized release gate transcripts are attached",
			},
			EscalationTriggers: []string{
				"escalate when local release gate failures require product or policy decision",
				"escalate when releaseHandoff.signals[] cannot be synchronized after deletion",
				"escalate when validation requires CI-only or external side effects beyond local authorization",
			},
			EscalationEvidence: []string{
				"failing release-check, go test, go vet or git diff transcript",
				"releaseHandoff.signals[] drift transcript after deletion result",
				"CI-only or external-effect validation requirement record",
			},
			EscalationRecipients: []string{
				"release gate owner for local validation failure triage",
				"Mission Commander for CI-only or external validation authorization decision",
				"runtime owner for releaseHandoff.signals[] synchronization evidence",
			},
			EscalationHandoffSteps: []string{
				"prepare release gate escalation packet with failing transcripts and handoff drift evidence",
				"route packet to release gate owner, Mission Commander and runtime owner",
				"record validation retry, CI-only authorization or stop decision before closure",
			},
			EscalationDecisionOptions: []string{
				"approve closure when release-check, tests, vet, diff and handoff transcripts are green",
				"defer closure until failing local validation commands are fixed and rerun",
				"escalate or stop when validation requires CI-only or external effects beyond local scope",
			},
			EscalationRetryConditions: []string{
				"retry after failing local release-check, test, vet or diff commands are fixed",
				"retry after releaseHandoff.signals[] are resynchronized with removal readiness",
				"retry after CI-only or external validation requirements receive explicit local-scope disposition",
			},
			EscalationStopConditions: []string{
				"stop when local release-check, tests, vet or diff remain failing after remediation",
				"stop when releaseHandoff.signals[] cannot be synchronized with removal readiness",
				"stop when validation requires CI-only or external effects outside authorized local scope",
			},
			EscalationResolutionArtifacts: []string{
				"final local validation transcript bundle or release-readiness stop record",
				"releaseHandoff.signals[] synchronization transcript attached to resolution",
				"CI-only or external validation disposition record with local-scope decision",
			},
			EscalationClosureChecks: []string{
				"closure confirms final release-check, test, vet and diff transcripts are attached",
				"closure confirms releaseHandoff.signals[] are synchronized with removal readiness",
				"closure confirms CI-only or external validation disposition is recorded before closing",
			},
			BoundaryGuards: []string{
				"must rerun Go-native release-check JSON/text after deletion result",
				"must require go test, go vet and git diff --check transcripts before closure",
				"must not mark release gate green when any required local command fails",
			},
			AuditChecks: []string{
				"audit confirms release-check JSON/text is ready after the deletion result",
				"audit confirms go test, go vet and git diff --check transcripts are attached",
				"audit confirms no required local release gate command is treated as green while failing",
			},
			ValidationCommands:      commands,
			AllowsPowerShellRuntime: false,
			AllowsExternalEffects:   false,
		},
	}
}

func publicFacadeRemovalExecutionFailureSignalCount(steps []PublicFacadeRemovalExecutionStep) int {
	count := 0
	for _, step := range steps {
		count += len(step.FailureSignals)
	}
	return count
}

func publicFacadeRemovalExecutionRemediationActionCount(steps []PublicFacadeRemovalExecutionStep) int {
	count := 0
	for _, step := range steps {
		count += len(step.RemediationActions)
	}
	return count
}

func publicFacadeRemovalExecutionVerificationArtifactCount(steps []PublicFacadeRemovalExecutionStep) int {
	count := 0
	for _, step := range steps {
		count += len(step.VerificationArtifacts)
	}
	return count
}

func publicFacadeRemovalExecutionLedgerEventCount(steps []PublicFacadeRemovalExecutionStep) int {
	count := 0
	for _, step := range steps {
		count += len(step.LedgerEvents)
	}
	return count
}

func publicFacadeRemovalExecutionStateTransitionCount(steps []PublicFacadeRemovalExecutionStep) int {
	count := 0
	for _, step := range steps {
		count += len(step.StateTransitions)
	}
	return count
}

func publicFacadeRemovalExecutionEscalationTriggerCount(steps []PublicFacadeRemovalExecutionStep) int {
	count := 0
	for _, step := range steps {
		count += len(step.EscalationTriggers)
	}
	return count
}

func publicFacadeRemovalExecutionEscalationEvidenceCount(steps []PublicFacadeRemovalExecutionStep) int {
	count := 0
	for _, step := range steps {
		count += len(step.EscalationEvidence)
	}
	return count
}

func publicFacadeRemovalExecutionEscalationRecipientCount(steps []PublicFacadeRemovalExecutionStep) int {
	count := 0
	for _, step := range steps {
		count += len(step.EscalationRecipients)
	}
	return count
}

func publicFacadeRemovalExecutionEscalationHandoffStepCount(steps []PublicFacadeRemovalExecutionStep) int {
	count := 0
	for _, step := range steps {
		count += len(step.EscalationHandoffSteps)
	}
	return count
}

func publicFacadeRemovalExecutionEscalationDecisionOptionCount(steps []PublicFacadeRemovalExecutionStep) int {
	count := 0
	for _, step := range steps {
		count += len(step.EscalationDecisionOptions)
	}
	return count
}

func publicFacadeRemovalExecutionEscalationRetryConditionCount(steps []PublicFacadeRemovalExecutionStep) int {
	count := 0
	for _, step := range steps {
		count += len(step.EscalationRetryConditions)
	}
	return count
}

func publicFacadeRemovalExecutionEscalationStopConditionCount(steps []PublicFacadeRemovalExecutionStep) int {
	count := 0
	for _, step := range steps {
		count += len(step.EscalationStopConditions)
	}
	return count
}

func publicFacadeRemovalExecutionEscalationResolutionArtifactCount(steps []PublicFacadeRemovalExecutionStep) int {
	count := 0
	for _, step := range steps {
		count += len(step.EscalationResolutionArtifacts)
	}
	return count
}

func publicFacadeRemovalExecutionEscalationClosureCheckCount(steps []PublicFacadeRemovalExecutionStep) int {
	count := 0
	for _, step := range steps {
		count += len(step.EscalationClosureChecks)
	}
	return count
}

func publicFacadeRemovalExecutionBoundaryGuardCount(steps []PublicFacadeRemovalExecutionStep) int {
	count := 0
	for _, step := range steps {
		count += len(step.BoundaryGuards)
	}
	return count
}

func publicFacadeRemovalExecutionAuditCheckCount(steps []PublicFacadeRemovalExecutionStep) int {
	count := 0
	for _, step := range steps {
		count += len(step.AuditChecks)
	}
	return count
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
