package releasecheck

import (
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"reflect"
	"sort"
	"strings"

	"github.com/shuiyu486/re-context-kits/internal/rekit/caseshim"
	"github.com/shuiyu486/re-context-kits/internal/rekit/defaultdocs"
	"github.com/shuiyu486/re-context-kits/internal/rekit/manifest"
	"github.com/shuiyu486/re-context-kits/internal/rekit/mission"
	"github.com/shuiyu486/re-context-kits/internal/rekit/promote"
	syncpkg "github.com/shuiyu486/re-context-kits/internal/rekit/sync"
)

type ReleaseHandoff struct {
	Ready                bool                                  `json:"ready"`
	Summary              string                                `json:"summary"`
	ReadFirst            []ReleaseHandoffDocument              `json:"readFirst"`
	Signals              []ReleaseHandoffSignal                `json:"signals"`
	LatestBatch          ReleaseHandoffLatestBatch             `json:"latestBatch"`
	ReleaseNotes         ReleaseHandoffReleaseNotes            `json:"releaseNotes"`
	KnownGaps            []ReleaseHandoffKnownGap              `json:"knownGaps"`
	PackMaturity         ReleaseHandoffPackMaturity            `json:"packMaturity"`
	PackMemoryCandidates ReleaseHandoffPackMemoryCandidateList `json:"packMemoryCandidates"`
	Validation           []ReleaseHandoffValidation            `json:"validation"`
	NextActions          []string                              `json:"nextActions"`
	Warnings             []string                              `json:"warnings"`
}

type ReleaseHandoffCounts struct {
	ReadFirst            int
	Signals              int
	KnownGaps            int
	Validation           int
	NextActions          int
	Warnings             int
	PackMaturity         ReleaseHandoffPackMaturityCounts
	PackMemoryCandidates int
}

func ReleaseHandoffCountsFor(handoff ReleaseHandoff) ReleaseHandoffCounts {
	return ReleaseHandoffCounts{
		ReadFirst:            len(handoff.ReadFirst),
		Signals:              len(handoff.Signals),
		KnownGaps:            len(handoff.KnownGaps),
		Validation:           len(handoff.Validation),
		NextActions:          len(handoff.NextActions),
		Warnings:             len(handoff.Warnings),
		PackMaturity:         ReleaseHandoffPackMaturityCountsFor(handoff.PackMaturity),
		PackMemoryCandidates: len(handoff.PackMemoryCandidates.Packs),
	}
}

type ReleaseHandoffDocument struct {
	Path    string `json:"path"`
	Present bool   `json:"present"`
	Purpose string `json:"purpose"`
}

type ReleaseHandoffSignal struct {
	Name    string   `json:"name"`
	Ready   bool     `json:"ready"`
	Summary string   `json:"summary"`
	Details []string `json:"details"`
}

type ReleaseHandoffLatestBatch struct {
	PlanPath         string                           `json:"planPath"`
	Present          bool                             `json:"present"`
	Title            string                           `json:"title"`
	BatchID          string                           `json:"batchId"`
	Status           string                           `json:"status"`
	Goal             string                           `json:"goal"`
	ValidationResult string                           `json:"validationResult"`
	Handoff          ReleaseHandoffLatestBatchHandoff `json:"handoff"`
}

type ReleaseHandoffLatestBatchHandoff struct {
	Completed                bool                                   `json:"completed"`
	LocalValidationReady     bool                                   `json:"localValidationReady"`
	ReleaseCheckReady        bool                                   `json:"releaseCheckReady"`
	RemoteReleaseGate        string                                 `json:"remoteReleaseGate,omitempty"`
	RemoteReleaseGateDetail  *ReleaseHandoffRemoteReleaseGateDetail `json:"remoteReleaseGateDetail,omitempty"`
	ReleaseInspectionCadence ReleaseHandoffReleaseInspectionCadence `json:"releaseInspectionCadence"`
	CommitRefs               []string                               `json:"commitRefs,omitempty"`
	Evidence                 []string                               `json:"evidence,omitempty"`
	ValidationWarnings       []string                               `json:"validationWarnings,omitempty"`
	NextAction               string                                 `json:"nextAction,omitempty"`
}

type ReleaseHandoffReleaseInspectionCadence struct {
	MaxPushes                 int      `json:"maxPushes"`
	ImplementationCommitReady bool     `json:"implementationCommitReady"`
	InspectionCommitReady     bool     `json:"inspectionCommitReady"`
	ThirdInspectionAllowed    bool     `json:"thirdInspectionAllowed"`
	NewRemoteSignal           bool     `json:"newRemoteSignal"`
	State                     string   `json:"state"`
	NextAction                string   `json:"nextAction"`
	Evidence                  []string `json:"evidence,omitempty"`
	Boundary                  []string `json:"boundary,omitempty"`
}

type ReleaseHandoffRemoteReleaseGateDetail struct {
	State            string   `json:"state"`
	RunRefs          []string `json:"runRefs,omitempty"`
	Jobs             []string `json:"jobs,omitempty"`
	EmptySteps       bool     `json:"emptySteps"`
	CompletedFailure bool     `json:"completedFailure"`
	CanClaimGreen    bool     `json:"canClaimGreen"`
	Boundary         []string `json:"boundary,omitempty"`
}

type ReleaseHandoffReleaseNotes struct {
	Path          string `json:"path"`
	Present       bool   `json:"present"`
	Section       string `json:"section"`
	LatestBatchID string `json:"latestBatchId"`
	Covered       bool   `json:"covered"`
	Summary       string `json:"summary"`
}

type ReleaseHandoffKnownGap struct {
	Index    int    `json:"index"`
	Category string `json:"category"`
	Summary  string `json:"summary"`
}

type ReleaseHandoffPackMaturity struct {
	Total                int                            `json:"total"`
	MaturityCounts       map[string]int                 `json:"maturityCounts"`
	PacksByMaturity      map[string][]string            `json:"packsByMaturity"`
	SchemaValid          bool                           `json:"schemaValid"`
	SchemaVersionReady   bool                           `json:"schemaVersionReady"`
	HeavyToolGateReady   bool                           `json:"heavyToolGateReady"`
	HeavyToolGateActions []string                       `json:"heavyToolGateActions"`
	HeavyToolGatesByPack []ReleaseHandoffPackGateStatus `json:"heavyToolGatesByPack"`
	Summary              string                         `json:"summary"`
}

type ReleaseHandoffPackMemoryCandidateList struct {
	Ready                       bool                                      `json:"ready"`
	Summary                     string                                    `json:"summary"`
	Total                       int                                       `json:"total"`
	Packs                       []ReleaseHandoffPackMemoryCandidateStatus `json:"packs"`
	NextAction                  string                                    `json:"nextAction,omitempty"`
	MissionCommanderNextActions []mission.MissionCommanderNextActionItem  `json:"missionCommanderNextActions,omitempty"`
	MissionCommanderActionQueue mission.MissionCommanderActionQueue       `json:"missionCommanderActionQueue"`
	Warnings                    []string                                  `json:"warnings,omitempty"`
}

type ReleaseHandoffPackMemoryCandidateReviewSummary struct {
	Total                  int                                                 `json:"total"`
	CandidateFiles         int                                                 `json:"candidateFiles"`
	ToolingFiles           int                                                 `json:"toolingFiles"`
	IndexEntries           int                                                 `json:"indexEntries"`
	ReviewArtifactCount    int                                                 `json:"reviewArtifactCount"`
	DecisionArtifactCount  int                                                 `json:"decisionArtifactCount"`
	CleanupArtifactCount   int                                                 `json:"cleanupArtifactCount"`
	ReconsumeArtifactCount int                                                 `json:"reconsumeArtifactCount"`
	ProofSummary           ReleaseHandoffPackMemoryCandidateReviewProofSummary `json:"proofSummary"`
	CandidateRoot          string                                              `json:"candidateRoot"`
	ToolingRoot            string                                              `json:"toolingRoot"`
	IndexPath              string                                              `json:"indexPath,omitempty"`
	RequiresReview         bool                                                `json:"requiresReview"`
	RequiresCleanup        bool                                                `json:"requiresCleanup"`
	HasCandidatePaths      bool                                                `json:"hasCandidatePaths"`
	HasToolingPaths        bool                                                `json:"hasToolingPaths"`
	HasIndex               bool                                                `json:"hasIndex"`
	HasDecisionArtifacts   bool                                                `json:"hasDecisionArtifacts"`
	HasCleanupArtifacts    bool                                                `json:"hasCleanupArtifacts"`
	HasReconsumeArtifacts  bool                                                `json:"hasReconsumeArtifacts"`
	NextAction             string                                              `json:"nextAction,omitempty"`
	Boundary               []string                                            `json:"boundary,omitempty"`
}

type ReleaseHandoffPackMemoryCandidateReviewNextMissingProof struct {
	Stage                     string   `json:"stage,omitempty"`
	ProofType                 string   `json:"proofType,omitempty"`
	Path                      string   `json:"path,omitempty"`
	CandidatePath             string   `json:"candidatePath,omitempty"`
	PackTarget                string   `json:"packTarget,omitempty"`
	SourceCaseRoot            string   `json:"sourceCaseRoot,omitempty"`
	When                      string   `json:"when,omitempty"`
	Action                    string   `json:"action,omitempty"`
	Format                    string   `json:"format,omitempty"`
	PacketPath                string   `json:"packetPath,omitempty"`
	CandidateDecisionPath     string   `json:"candidateDecisionPath,omitempty"`
	EvidenceRefs              []string `json:"evidenceRefs,omitempty"`
	DraftCommand              string   `json:"draftCommand,omitempty"`
	DraftApplyTemplate        string   `json:"draftApplyTemplate,omitempty"`
	RequiresPacket            bool     `json:"requiresPacket,omitempty"`
	RequiresCandidateDecision bool     `json:"requiresCandidateDecision,omitempty"`
	RequiresExplicitReview    bool     `json:"requiresExplicitReview,omitempty"`
	Evidence                  []string `json:"evidence,omitempty"`
	Boundary                  []string `json:"boundary,omitempty"`
}

type ReleaseHandoffPackMemoryCandidateReviewProofSummary struct {
	Total                    int                                                      `json:"total"`
	Present                  int                                                      `json:"present"`
	Missing                  int                                                      `json:"missing"`
	DecisionPresent          int                                                      `json:"decisionPresent"`
	DecisionMissing          int                                                      `json:"decisionMissing"`
	CleanupPresent           int                                                      `json:"cleanupPresent"`
	CleanupMissing           int                                                      `json:"cleanupMissing"`
	ReconsumePresent         int                                                      `json:"reconsumePresent"`
	ReconsumeMissing         int                                                      `json:"reconsumeMissing"`
	ProofRoot                string                                                   `json:"proofRoot,omitempty"`
	ProofProgress            string                                                   `json:"proofProgress,omitempty"`
	CurrentStage             string                                                   `json:"currentStage,omitempty"`
	NextMissingProofType     string                                                   `json:"nextMissingProofType,omitempty"`
	NextMissingProofPath     string                                                   `json:"nextMissingProofPath,omitempty"`
	NextMissingCandidatePath string                                                   `json:"nextMissingCandidatePath,omitempty"`
	NextMissingPackTarget    string                                                   `json:"nextMissingPackTarget,omitempty"`
	NextMissingProof         *ReleaseHandoffPackMemoryCandidateReviewNextMissingProof `json:"nextMissingProof,omitempty"`
	Complete                 bool                                                     `json:"complete"`
	NextAction               string                                                   `json:"nextAction,omitempty"`
	Boundary                 []string                                                 `json:"boundary,omitempty"`
}

type ReleaseHandoffPackMemoryCandidateStatus struct {
	Pack                   string                                              `json:"pack"`
	Maturity               string                                              `json:"maturity"`
	CandidateRoot          string                                              `json:"candidateRoot"`
	ToolingRoot            string                                              `json:"toolingRoot"`
	IndexPath              string                                              `json:"indexPath,omitempty"`
	CandidateFiles         int                                                 `json:"candidateFiles"`
	ToolingFiles           int                                                 `json:"toolingFiles"`
	IndexEntries           int                                                 `json:"indexEntries"`
	CandidatePaths         []string                                            `json:"candidatePaths,omitempty"`
	ToolingPaths           []string                                            `json:"toolingPaths,omitempty"`
	IndexCandidates        []ReleaseHandoffPackMemoryCandidateIndexEntry       `json:"indexCandidates,omitempty"`
	ReviewArtifacts        []ReleaseHandoffPackMemoryCandidateReviewArtifact   `json:"reviewArtifacts,omitempty"`
	ReviewSummary          ReleaseHandoffPackMemoryCandidateReviewSummary      `json:"reviewSummary"`
	ProofSummary           ReleaseHandoffPackMemoryCandidateReviewProofSummary `json:"proofSummary"`
	DecisionReceipts       []ReleaseHandoffPackMemoryCandidateDecisionReceipt  `json:"decisionReceipts,omitempty"`
	DecisionDraftHandoff   *promote.CandidateDecisionDraftHandoff              `json:"decisionDraftHandoff,omitempty"`
	PendingVerifications   int                                                 `json:"pendingVerifications"`
	CompletedVerifications int                                                 `json:"completedVerifications"`
	ProofRoot              string                                              `json:"proofRoot,omitempty"`
	HasOpenWork            bool                                                `json:"hasOpenWork"`
	RequiresReview         bool                                                `json:"requiresReview"`
	RequiresCleanup        bool                                                `json:"requiresCleanup"`
	RequiresVerification   bool                                                `json:"requiresVerification"`
	Action                 string                                              `json:"action,omitempty"`
	Evidence               []string                                            `json:"evidence,omitempty"`
	Boundary               []string                                            `json:"boundary,omitempty"`

	repoRootFull string
}

type ReleaseHandoffPackMemoryCandidateDecisionReceiptAction struct {
	CandidatePath       string `json:"candidatePath"`
	Kind                string `json:"kind"`
	Decision            string `json:"decision"`
	PackTarget          string `json:"packTarget,omitempty"`
	CandidateBackupPath string `json:"candidateBackupPath,omitempty"`
	TargetBackupPath    string `json:"targetBackupPath,omitempty"`

	candidatePathFull       string
	packTargetFull          string
	candidateBackupPathFull string
	targetBackupPathFull    string
	evidenceRefsFull        []string
}

type ReleaseHandoffPackMemoryCandidateDecisionReceipt struct {
	Path                         string                                                   `json:"path"`
	SourceCaseRoot               string                                                   `json:"sourceCaseRoot,omitempty"`
	Accepted                     int                                                      `json:"accepted"`
	Rejected                     int                                                      `json:"rejected"`
	Superseded                   int                                                      `json:"superseded"`
	PacketPath                   string                                                   `json:"packetPath"`
	DecisionPath                 string                                                   `json:"decisionPath"`
	Actions                      []ReleaseHandoffPackMemoryCandidateDecisionReceiptAction `json:"actions,omitempty"`
	VerificationPending          bool                                                     `json:"verificationPending"`
	VerificationWorkspaceRoot    string                                                   `json:"verificationWorkspaceRoot,omitempty"`
	VerificationProvisionCommand string                                                   `json:"verificationProvisionCommand,omitempty"`
	VerificationCommand          string                                                   `json:"verificationCommand,omitempty"`
	VerificationProofPath        string                                                   `json:"verificationProofPath,omitempty"`
	VerificationComplete         bool                                                     `json:"verificationComplete"`
	ProvisionStatus              string                                                   `json:"provisionStatus,omitempty"`
	ProvisionIntentPath          string                                                   `json:"provisionIntentPath,omitempty"`
	ProvisionReceiptPath         string                                                   `json:"provisionReceiptPath,omitempty"`
	ProvisionSHA256              string                                                   `json:"provisionSha256,omitempty"`
	ProvisionApplyCommand        string                                                   `json:"provisionApplyCommand,omitempty"`
	ProvisionInProgress          bool                                                     `json:"provisionInProgress"`
	ProvisionComplete            bool                                                     `json:"provisionComplete"`
	ProvisionNextAction          string                                                   `json:"provisionNextAction,omitempty"`
	RetirementStatus             string                                                   `json:"retirementStatus,omitempty"`
	RetirementPreviewCommand     string                                                   `json:"retirementPreviewCommand,omitempty"`
	RetirementIntentPath         string                                                   `json:"retirementIntentPath,omitempty"`
	RetirementReceiptPath        string                                                   `json:"retirementReceiptPath,omitempty"`
	RetirementSHA256             string                                                   `json:"retirementSha256,omitempty"`
	RetirementRequired           bool                                                     `json:"retirementRequired"`
	RetirementInProgress         bool                                                     `json:"retirementInProgress"`
	Retired                      bool                                                     `json:"retired"`
	RetirementNextAction         string                                                   `json:"retirementNextAction,omitempty"`

	pathFull       string
	receiptHash    string
	packetHash     string
	decisionHash   string
	caseRootFull   string
	backupRootFull string
	indexPathFull  string
}

type ReleaseHandoffPackMemoryCandidateIndexEntry struct {
	Path      string `json:"path"`
	Candidate string `json:"candidate"`
}

type ReleaseHandoffPackMemoryCandidateReviewArtifact struct {
	Name                  string   `json:"name"`
	CandidatePath         string   `json:"candidatePath,omitempty"`
	PackTarget            string   `json:"packTarget,omitempty"`
	SourceCaseRoot        string   `json:"sourceCaseRoot,omitempty"`
	PacketPath            string   `json:"packetPath,omitempty"`
	CandidateDecisionPath string   `json:"candidateDecisionPath,omitempty"`
	When                  string   `json:"when"`
	Action                string   `json:"action"`
	Format                string   `json:"format"`
	ExpectedProofs        []string `json:"expectedProofs,omitempty"`
	ProofPath             string   `json:"proofPath,omitempty"`
	ProofPresent          bool     `json:"proofPresent"`
	Evidence              []string `json:"evidence,omitempty"`
	Boundary              []string `json:"boundary,omitempty"`
}

type ReleaseHandoffPackMaturityCounts struct {
	Total                int
	MaturityCounts       int
	PacksByMaturity      int
	HeavyToolGateActions int
	HeavyToolGatesByPack int
}

func ReleaseHandoffPackMaturityCountsFor(maturity ReleaseHandoffPackMaturity) ReleaseHandoffPackMaturityCounts {
	return ReleaseHandoffPackMaturityCounts{
		Total:                maturity.Total,
		MaturityCounts:       len(maturity.MaturityCounts),
		PacksByMaturity:      len(maturity.PacksByMaturity),
		HeavyToolGateActions: len(maturity.HeavyToolGateActions),
		HeavyToolGatesByPack: len(maturity.HeavyToolGatesByPack),
	}
}

type ReleaseHandoffPackGateStatus struct {
	ID             string   `json:"id"`
	Maturity       string   `json:"maturity"`
	SchemaValid    bool     `json:"schemaValid"`
	SchemaVersion  string   `json:"schemaVersion"`
	HeavyToolGates int      `json:"heavyToolGates"`
	Actions        []string `json:"actions"`
}

type ReleaseHandoffValidation struct {
	Command  string `json:"command"`
	Kind     string `json:"kind"`
	RepoPath string `json:"repoPath,omitempty"`
	Required bool   `json:"required"`
	Present  bool   `json:"present"`
	Resolved bool   `json:"resolved"`
}

var releaseHandoffReadFirst = []ReleaseHandoffDocument{
	{Path: "docs/context-routing.md", Purpose: "progressive-disclosure router for deciding what to read next"},
	{Path: "docs/batch-plan.md", Purpose: "current batch state, next candidates, and latest completed batch summary"},
	{Path: "docs/release-readiness.md", Purpose: "release gate, current known gaps, and CI truthfulness rules"},
	{Path: "CHANGELOG.md", Purpose: "user-visible changes and boundaries"},
}

func releaseHandoff(repo string, check Result) ReleaseHandoff {
	handoff := ReleaseHandoff{
		Ready:       true,
		Summary:     "release handoff summary ok",
		ReadFirst:   releaseHandoffDocuments(repo),
		LatestBatch: latestBatchSummary(repo),
		Validation:  releaseHandoffValidation(check.GateProfile.Steps),
		NextActions: releaseHandoffNextActions(),
		Warnings:    []string{},
	}
	handoff.ReleaseNotes = latestReleaseNotes(repo, handoff.LatestBatch)
	handoff.KnownGaps = releaseHandoffKnownGaps(check.KnownGaps)
	handoff.PackMaturity = releaseHandoffPackMaturity(check.Packs, check.HeavyToolGateActions)
	handoff.PackMemoryCandidates = releaseHandoffPackMemoryCandidates(repo, check.Packs)
	handoff.Signals = releaseHandoffSignals(check, handoff.LatestBatch, handoff.ReleaseNotes, handoff.KnownGaps, handoff.PackMaturity, handoff.PackMemoryCandidates)
	handoff.Warnings = releaseHandoffWarnings(handoff)
	if ReleaseHandoffCountsFor(handoff).Warnings > 0 {
		handoff.Ready = false
		handoff.Summary = "release handoff summary has warnings"
	}
	return handoff
}

func releaseHandoffDocuments(repo string) []ReleaseHandoffDocument {
	docs := make([]ReleaseHandoffDocument, 0, len(releaseHandoffReadFirst))
	for _, doc := range releaseHandoffReadFirst {
		_, err := os.Stat(filepath.Join(repo, filepath.FromSlash(doc.Path)))
		doc.Present = err == nil
		docs = append(docs, doc)
	}
	return docs
}

func releaseHandoffSignals(check Result, latest ReleaseHandoffLatestBatch, notes ReleaseHandoffReleaseNotes, gaps []ReleaseHandoffKnownGap, packMaturity ReleaseHandoffPackMaturity, packCandidates ReleaseHandoffPackMemoryCandidateList) []ReleaseHandoffSignal {
	resultCounts := ReleaseCheckResultCountsFor(check)
	ciGateCounts := CIReleaseGateCountsFor(check.CIReleaseGate)
	powerShellCounts := PowerShellDeprecationCountsFor(check.PowerShellDeprecation)
	caseShimCounts := caseshim.ReadinessCountsFor(check.CaseShim)
	publicDefaultDocCounts := defaultdocs.ReadinessCountsFor(check.PublicDefaultDocs)
	return []ReleaseHandoffSignal{
		{
			Name:    "release-check inventory",
			Ready:   check.Ready,
			Summary: check.Summary,
			Details: []string{
				fmt.Sprintf("gateProfile=%s ready=%t steps=%d", check.GateProfile.Name, check.GateProfile.Ready, resultCounts.GateProfileSteps),
				fmt.Sprintf("packs=%d knownGaps=%d warnings=%d", resultCounts.Packs, resultCounts.KnownGaps, resultCounts.Warnings),
			},
		},
		{
			Name:    "CI release gate",
			Ready:   check.CIReleaseGate.Ready,
			Summary: check.CIReleaseGate.Summary,
			Details: []string{
				fmt.Sprintf("workflow=%s", check.CIReleaseGate.WorkflowPath),
				fmt.Sprintf("jobs=%d commands=%d forbidden=%d", ciGateCounts.Jobs, ciGateCounts.RequiredCommands, ciGateCounts.ForbiddenStrings),
			},
		},
		{
			Name:    "PowerShell deprecation",
			Ready:   check.PowerShellDeprecation.Ready,
			Summary: check.PowerShellDeprecation.Summary,
			Details: []string{
				fmt.Sprintf("strategy=%s", check.PowerShellDeprecation.StrategyDocument),
				fmt.Sprintf("fallbackRetirement=%t noFallback=%d candidates=%d removalModules=%d retiredModules=%d", check.PowerShellDeprecation.FallbackRetirement.Ready, powerShellCounts.FallbackNoFallbackCommands, powerShellCounts.FallbackCandidateCommands, powerShellCounts.FallbackRemovalCandidateModules, powerShellCounts.FallbackRetiredModules),
				fmt.Sprintf("facadeRuntime=%t legacyImports=%t dispatcher=%t", check.PowerShellDeprecation.FacadeRuntime.Ready, check.PowerShellDeprecation.FacadeRuntime.LegacyModuleImportsPresent, check.PowerShellDeprecation.FacadeRuntime.CommandDispatcherPresent),
				fmt.Sprintf("publicFacade=%t retained=%t facadeCommands=%d noFallback=%d", check.PowerShellDeprecation.PublicFacade.Ready, check.PowerShellDeprecation.PublicFacade.Retained, powerShellCounts.PublicFacadeCommandSurface, powerShellCounts.PublicFacadeNoFallbackCommands),
				fmt.Sprintf("moduleRemoval=%t candidates=%d retired=%d facadeDeps=%d undocumented=%d", check.PowerShellDeprecation.ModuleRemoval.Ready, powerShellCounts.ModuleRemovalCandidateModules, powerShellCounts.ModuleRemovalRetiredModules, powerShellCounts.ModuleRemovalFacadeRuntimeDependencies, powerShellCounts.ModuleRemovalUndocumentedModules),
				fmt.Sprintf("moduleReferences=%t activeTests=%d fixtures=%d blockers=%d unclassified=%d", check.PowerShellDeprecation.ModuleReferences.Ready, powerShellCounts.ModuleReferencesActiveTestDependencies, powerShellCounts.ModuleReferencesCompatibilityFixtures, powerShellCounts.ModuleReferencesRemovalBlockers, powerShellCounts.ModuleReferencesUnclassifiedReferences),
				fmt.Sprintf("commands=%d modules=%d freezeGates=%d blocked=%d", powerShellCounts.CommandOwnership, powerShellCounts.ModuleStatus, powerShellCounts.FreezeGates, powerShellCounts.BlockedMigrations),
			},
		},
		{
			Name:    "Go-native public surface",
			Ready:   check.GoNativePublicSurface.Ready,
			Summary: check.GoNativePublicSurface.Summary,
			Details: goNativePublicSurfaceHandoffDetails(check.GoNativePublicSurface),
		},
		{
			Name:    "public facade removal prerequisites",
			Ready:   check.PublicFacadeRemoval.Ready,
			Summary: check.PublicFacadeRemoval.Summary,
			Details: publicFacadeRemovalHandoffDetails(check.PublicFacadeRemoval),
		},
		{
			Name:    "case shim readiness",
			Ready:   check.CaseShim.Ready,
			Summary: check.CaseShim.Summary,
			Details: []string{
				fmt.Sprintf("template=%s", check.CaseShim.TemplatePath),
				fmt.Sprintf("requiredPhrases=%d canonicalPhrases=%d forbidden=%d", caseShimCounts.RequiredPhrases, caseShimCounts.CanonicalSkillPhrases, caseShimCounts.ForbiddenStrings),
				"case-local shim stays thin and does not name PowerShell or raw Go CLI commands",
			},
		},
		{
			Name:    "public default docs",
			Ready:   check.PublicDefaultDocs.Ready,
			Summary: check.PublicDefaultDocs.Summary,
			Details: []string{
				fmt.Sprintf("documents=%d requiredPhrases=%d forbiddenCommands=%d forbiddenShellFences=%d", publicDefaultDocCounts.Documents, publicDefaultDocCounts.RequiredPhrases, publicDefaultDocCounts.ForbiddenCommands, publicDefaultDocCounts.ForbiddenShellFences),
				"README, CLAUDE, slash skill, product direction, autonomous goal, release readiness, Go-first plan, runtime migration, deprecation roadmap, vision, reference map, rollout plan, and tests guide keep Mission Control / Go-native defaults",
				"PowerShell façade command snippets and shell fences are not documented as default user paths",
			},
		},
		{
			Name:    "heavy-tool gate manifests",
			Ready:   resultCounts.HeavyToolGateActions > 0,
			Summary: strings.Join(check.HeavyToolGateActions, ","),
			Details: []string{
				fmt.Sprintf("actions=%d", resultCounts.HeavyToolGateActions),
				"gate preview/apply records pending-gate or authorized-gate ledger decisions; no heavy-tool execution",
			},
		},
		{
			Name:    "pack maturity summary",
			Ready:   packMaturity.Total > 0 && packMaturity.SchemaValid && packMaturity.SchemaVersionReady && packMaturity.HeavyToolGateReady,
			Summary: packMaturity.Summary,
			Details: releaseHandoffPackMaturityDetails(packMaturity),
		},
		{
			Name:    "latest batch documentation",
			Ready:   latest.Present && latest.Handoff.Completed && strings.TrimSpace(latest.Goal) != "" && strings.TrimSpace(latest.ValidationResult) != "",
			Summary: latest.Title,
			Details: []string{
				fmt.Sprintf("batch=%s", latest.BatchID),
				fmt.Sprintf("status=%s", latest.Status),
				fmt.Sprintf("localValidationReady=%t releaseCheckReady=%t remoteReleaseGate=%s", latest.Handoff.LocalValidationReady, latest.Handoff.ReleaseCheckReady, latest.Handoff.RemoteReleaseGate),
				fmt.Sprintf("releaseInspectionCadence state=%s maxPushes=%d implementationReady=%t inspectionReady=%t thirdInspectionAllowed=%t newRemoteSignal=%t", latest.Handoff.ReleaseInspectionCadence.State, latest.Handoff.ReleaseInspectionCadence.MaxPushes, latest.Handoff.ReleaseInspectionCadence.ImplementationCommitReady, latest.Handoff.ReleaseInspectionCadence.InspectionCommitReady, latest.Handoff.ReleaseInspectionCadence.ThirdInspectionAllowed, latest.Handoff.ReleaseInspectionCadence.NewRemoteSignal),
				fmt.Sprintf("releaseInspectionNextAction=%s", latest.Handoff.ReleaseInspectionCadence.NextAction),
				fmt.Sprintf("nextAction=%s", latest.Handoff.NextAction),
				fmt.Sprintf("plan=%s", latest.PlanPath),
			},
		},
		{
			Name:    "pack-memory candidates",
			Ready:   packCandidates.Ready,
			Summary: packCandidates.Summary,
			Details: releaseHandoffPackMemoryCandidateDetails(packCandidates),
		},
		{
			Name:    "release notes freshness",
			Ready:   notes.Present && notes.Covered,
			Summary: notes.Summary,
			Details: []string{
				fmt.Sprintf("path=%s", notes.Path),
				fmt.Sprintf("section=%s", notes.Section),
				fmt.Sprintf("latestBatch=%s covered=%t", notes.LatestBatchID, notes.Covered),
			},
		},
		{
			Name:    "known gaps summary",
			Ready:   len(gaps) > 0,
			Summary: fmt.Sprintf("%d known gaps tracked", len(gaps)),
			Details: releaseHandoffKnownGapDetails(gaps),
		},
	}
}

func releaseHandoffPackMaturity(packs []manifest.PackSummary, actions []string) ReleaseHandoffPackMaturity {
	inventory := ReleaseHandoffPackMaturity{
		Total:                len(packs),
		MaturityCounts:       map[string]int{},
		PacksByMaturity:      map[string][]string{},
		SchemaValid:          true,
		SchemaVersionReady:   true,
		HeavyToolGateReady:   len(actions) > 0,
		HeavyToolGateActions: append([]string{}, actions...),
		HeavyToolGatesByPack: []ReleaseHandoffPackGateStatus{},
		Summary:              "pack maturity inventory ok",
	}
	if len(packs) == 0 {
		inventory.SchemaValid = false
		inventory.SchemaVersionReady = false
		inventory.HeavyToolGateReady = false
		inventory.Summary = "pack maturity inventory has warnings"
		return inventory
	}
	sort.Strings(inventory.HeavyToolGateActions)
	for _, pack := range packs {
		maturity := strings.TrimSpace(pack.Maturity)
		if maturity == "" {
			maturity = "unknown"
		}
		inventory.MaturityCounts[maturity]++
		inventory.PacksByMaturity[maturity] = append(inventory.PacksByMaturity[maturity], pack.ID)
		if !pack.SchemaValid || strings.TrimSpace(pack.ID) == "" {
			inventory.SchemaValid = false
		}
		if strings.TrimSpace(pack.SchemaVersion) != "1" {
			inventory.SchemaVersionReady = false
		}
		actions := append([]string{}, pack.HeavyToolGateActions...)
		if pack.HeavyToolGates == 0 || len(actions) == 0 {
			inventory.HeavyToolGateReady = false
		}
		sort.Strings(actions)
		inventory.HeavyToolGatesByPack = append(inventory.HeavyToolGatesByPack, ReleaseHandoffPackGateStatus{
			ID:             pack.ID,
			Maturity:       maturity,
			SchemaValid:    pack.SchemaValid,
			SchemaVersion:  pack.SchemaVersion,
			HeavyToolGates: pack.HeavyToolGates,
			Actions:        actions,
		})
	}
	for maturity := range inventory.PacksByMaturity {
		sort.Strings(inventory.PacksByMaturity[maturity])
	}
	sort.Slice(inventory.HeavyToolGatesByPack, func(i, j int) bool {
		return strings.ToLower(inventory.HeavyToolGatesByPack[i].ID) < strings.ToLower(inventory.HeavyToolGatesByPack[j].ID)
	})
	if !inventory.SchemaValid || !inventory.SchemaVersionReady || !inventory.HeavyToolGateReady {
		inventory.Summary = "pack maturity inventory has warnings"
	}
	return inventory
}

func releaseHandoffPackMemoryCandidates(repo string, packs []manifest.PackSummary) ReleaseHandoffPackMemoryCandidateList {
	inventory := ReleaseHandoffPackMemoryCandidateList{
		Ready:   true,
		Summary: "pack-memory candidate inventory ok",
		Packs:   []ReleaseHandoffPackMemoryCandidateStatus{},
	}
	warnings := []string{}
	for _, pack := range packs {
		if strings.TrimSpace(pack.ID) == "" {
			continue
		}
		status, err := releaseHandoffPackMemoryCandidateStatus(repo, pack)
		if err != nil {
			warnings = append(warnings, err.Error())
			continue
		}
		if !status.HasOpenWork {
			continue
		}
		openUnits := status.CandidateFiles + status.ToolingFiles + status.IndexEntries + status.PendingVerifications
		if openUnits == 0 && status.IndexPath != "" {
			openUnits = 1
		}
		if openUnits == 0 && status.ProofSummary.CleanupMissing > 0 {
			openUnits = status.ProofSummary.CleanupMissing
		}
		inventory.Total += openUnits
		inventory.Packs = append(inventory.Packs, status)
	}
	sort.Slice(inventory.Packs, func(i, j int) bool {
		return strings.ToLower(inventory.Packs[i].Pack) < strings.ToLower(inventory.Packs[j].Pack)
	})
	if len(warnings) > 0 {
		inventory.Ready = false
		inventory.Summary = "pack-memory candidate inventory has warnings"
		inventory.Warnings = warnings
		inventory.NextAction = "repair pack-memory candidate index or candidate directory inventory before release handoff"
		inventory.MissionCommanderNextActions = packMemoryCandidateNextActions(inventory.Packs)
		inventory.MissionCommanderActionQueue = mission.MissionCommanderActionQueueFor(inventory.MissionCommanderNextActions)
		return inventory
	}
	if inventory.Total > 0 {
		inventory.Ready = false
		inventory.Summary = "pack-memory candidate inventory has open review/cleanup/verification work"
		inventory.NextAction = "review listed pack-memory candidates or complete listed candidate decision verification, then rerun release-check/status"
		inventory.Warnings = []string{"open pack-memory candidates or candidate decision verifications require closure before release handoff"}
	} else {
		inventory.NextAction = "no pack-memory candidate cleanup is pending"
	}
	inventory.MissionCommanderNextActions = packMemoryCandidateNextActions(inventory.Packs)
	inventory.MissionCommanderActionQueue = mission.MissionCommanderActionQueueFor(inventory.MissionCommanderNextActions)
	return inventory
}

func RebuildPackMemoryCandidateActionQueue(inventory *ReleaseHandoffPackMemoryCandidateList) {
	if inventory == nil {
		return
	}
	inventory.MissionCommanderNextActions = packMemoryCandidateNextActions(inventory.Packs)
	inventory.MissionCommanderActionQueue = mission.MissionCommanderActionQueueFor(inventory.MissionCommanderNextActions)
}

func packMemoryCandidateNextActions(packs []ReleaseHandoffPackMemoryCandidateStatus) []mission.MissionCommanderNextActionItem {
	items := []mission.MissionCommanderNextActionItem{}
	for _, pack := range packs {
		if primary, ok := packMemoryCandidatePrimaryNextAction(pack); ok {
			items = append(items, primary)
		}
		if followUp, ok := packMemoryCandidateProofFollowUpAction(pack); ok {
			items = append(items, followUp)
		}
	}
	return orderPackMemoryCandidateActions(mission.UniqueCommanderNextActions(items))
}

func orderPackMemoryCandidateActions(items []mission.MissionCommanderNextActionItem) []mission.MissionCommanderNextActionItem {
	out := append([]mission.MissionCommanderNextActionItem{}, items...)
	sort.SliceStable(out, func(i, j int) bool {
		return packMemoryCandidateActionPriority(out[i]) < packMemoryCandidateActionPriority(out[j])
	})
	return out
}

func packMemoryCandidateActionPriority(item mission.MissionCommanderNextActionItem) int {
	priority := packMemoryCandidatePrimaryActionPriority(item)
	if mission.MissionCommanderNextActionIsFollowUp(item) {
		priority += 100
	}
	return priority
}

func packMemoryCandidatePrimaryActionPriority(item mission.MissionCommanderNextActionItem) int {
	switch item.ActionID {
	case "pack-memory-verification-retirement-in-progress":
		return 0
	case "pack-memory-verification-provision-in-progress":
		return 1
	case "pack-memory-verification-retirement-required":
		return 2
	case "pack-memory-verification-run-required":
		return 3
	case "pack-memory-verification-provision-required":
		return 4
	case "pack-memory-cleanup-proof-required":
		return 5
	case "pack-memory-reconsume-proof-required":
		return 6
	case "pack-memory-decision-proof-required":
		return 7
	case "pack-memory-decision-draft-required":
		return 8
	}
	switch item.State {
	case "pack-memory-verification-required":
		return 4
	case "pack-memory-proof-required":
		return 7
	case "pack-memory-review-required":
		return 8
	case "pack-memory-cleanup-required":
		return 9
	case "pack-memory-open-work":
		return 10
	case "pack-memory-ready":
		return 11
	default:
		return 12
	}
}

func packMemoryCandidatePrimaryNextAction(pack ReleaseHandoffPackMemoryCandidateStatus) (mission.MissionCommanderNextActionItem, bool) {
	command := packMemoryCandidateCurrentCommand(pack)
	if strings.TrimSpace(command) == "" {
		return mission.MissionCommanderNextActionItem{}, false
	}
	return mission.MissionCommanderNextActionItem{
		Label:          pack.Pack,
		ActionID:       packMemoryCandidateActionID(pack),
		State:          packMemoryCandidateActionState(pack),
		Command:        command,
		Source:         "packMemoryCandidates." + pack.Pack,
		RequiresReview: true,
		Reasons:        packMemoryCandidateActionReasons(pack),
		Boundary:       packMemoryCandidateActionBoundary(pack),
	}, true
}

func packMemoryCandidateProofFollowUpAction(pack ReleaseHandoffPackMemoryCandidateStatus) (mission.MissionCommanderNextActionItem, bool) {
	next := pack.ProofSummary.NextMissingProof
	if next == nil || strings.TrimSpace(next.DraftCommand) == "" {
		return mission.MissionCommanderNextActionItem{}, false
	}
	if strings.TrimSpace(next.DraftCommand) == strings.TrimSpace(packMemoryCandidateCurrentCommand(pack)) {
		return mission.MissionCommanderNextActionItem{}, false
	}
	actionID := packMemoryCandidateProofActionID(*next)
	return mission.MissionCommanderNextActionItem{
		Label:          pack.Pack,
		ActionID:       actionID,
		State:          "pack-memory-proof-required",
		Command:        next.DraftCommand,
		Source:         "packMemoryCandidates." + pack.Pack + ".followUp.proof",
		RequiresReview: true,
		Reasons:        packMemoryCandidateProofActionReasons(pack, *next, actionID),
		Boundary:       packMemoryCandidateProofActionBoundary(pack, *next),
	}, true
}

func packMemoryCandidateProofActionID(next ReleaseHandoffPackMemoryCandidateReviewNextMissingProof) string {
	switch strings.TrimSpace(next.Stage) {
	case "decision-proof-required":
		return "pack-memory-decision-proof-required"
	case "cleanup-proof-required":
		return "pack-memory-cleanup-proof-required"
	case "reconsume-proof-required":
		return "pack-memory-reconsume-proof-required"
	}
	switch strings.TrimSpace(next.ProofType) {
	case "candidate-decision-note", "blocked-review-note":
		return "pack-memory-decision-proof-required"
	case "candidate-cleanup-proof":
		return "pack-memory-cleanup-proof-required"
	case "pack-doctor-output", "fresh-case-reconsume-proof", "attached-case-reconsume-proof":
		return "pack-memory-reconsume-proof-required"
	}
	return "pack-memory-proof-required"
}

func packMemoryCandidateProofActionReasons(pack ReleaseHandoffPackMemoryCandidateStatus, next ReleaseHandoffPackMemoryCandidateReviewNextMissingProof, actionID string) []string {
	stage := strings.TrimSpace(next.Stage)
	if stage == "" {
		stage = "unknown"
	}
	proofType := strings.TrimSpace(next.ProofType)
	if proofType == "" {
		proofType = "unknown"
	}
	reasons := []string{"pack=" + pack.Pack, "actionId=" + actionID, "proofStage=" + stage, "proofType=" + proofType}
	if strings.TrimSpace(next.Path) != "" {
		reasons = append(reasons, "proof path="+next.Path)
	}
	if strings.TrimSpace(next.CandidatePath) != "" {
		reasons = append(reasons, "candidate="+next.CandidatePath)
	}
	if strings.TrimSpace(next.PackTarget) != "" {
		reasons = append(reasons, "packTarget="+next.PackTarget)
	}
	if strings.TrimSpace(next.SourceCaseRoot) != "" {
		reasons = append(reasons, "sourceCaseRoot="+next.SourceCaseRoot)
	}
	if strings.TrimSpace(pack.Action) != "" {
		reasons = append(reasons, pack.Action)
	}
	return mission.UniqueStrings(reasons)
}

func packMemoryCandidateProofActionBoundary(pack ReleaseHandoffPackMemoryCandidateStatus, next ReleaseHandoffPackMemoryCandidateReviewNextMissingProof) []string {
	boundary := packMemoryCandidateActionBoundary(pack)
	boundary = append(boundary, next.Boundary...)
	boundary = append(boundary,
		"proof follow-up is read-only Mission Commander handoff; it does not replace the current verification action",
		"run proof draft WhatIf and hash-gated Apply explicitly before declaring pack-memory downstream closure",
	)
	return mission.UniqueStrings(boundary)
}

func packMemoryCandidateCurrentCommand(pack ReleaseHandoffPackMemoryCandidateStatus) string {
	if receipt, stage, ok := packMemoryCandidateActiveDecisionReceipt(pack); ok {
		switch stage {
		case "verification-retirement-in-progress":
			return strings.TrimSpace(receipt.RetirementNextAction)
		case "verification-retirement-required":
			return strings.TrimSpace(receipt.RetirementPreviewCommand)
		case "verification-provision-in-progress":
			if command := strings.TrimSpace(receipt.ProvisionApplyCommand); command != "" {
				return command
			}
			return strings.TrimSpace(receipt.ProvisionNextAction)
		case "verification-provision-required":
			return strings.TrimSpace(receipt.VerificationProvisionCommand)
		case "verification-run-required":
			return strings.TrimSpace(receipt.VerificationCommand)
		}
	}
	if next := pack.ProofSummary.NextMissingProof; next != nil {
		if command := strings.TrimSpace(next.DraftCommand); command != "" {
			return command
		}
	}
	if pack.DecisionDraftHandoff != nil {
		if command := strings.TrimSpace(pack.DecisionDraftHandoff.NextAction); command != "" {
			return command
		}
	}
	return strings.TrimSpace(pack.Action)
}

func packMemoryCandidateActiveDecisionReceipt(pack ReleaseHandoffPackMemoryCandidateStatus) (ReleaseHandoffPackMemoryCandidateDecisionReceipt, string, bool) {
	stages := []struct {
		stage   string
		command func(ReleaseHandoffPackMemoryCandidateDecisionReceipt) string
	}{
		{stage: "verification-retirement-in-progress", command: func(receipt ReleaseHandoffPackMemoryCandidateDecisionReceipt) string {
			if receipt.RetirementInProgress {
				return strings.TrimSpace(receipt.RetirementNextAction)
			}
			return ""
		}},
		{stage: "verification-provision-in-progress", command: func(receipt ReleaseHandoffPackMemoryCandidateDecisionReceipt) string {
			if receipt.ProvisionInProgress {
				return strings.TrimSpace(receipt.ProvisionNextAction)
			}
			return ""
		}},
		{stage: "verification-retirement-required", command: func(receipt ReleaseHandoffPackMemoryCandidateDecisionReceipt) string {
			if receipt.RetirementRequired {
				return strings.TrimSpace(receipt.RetirementPreviewCommand)
			}
			return ""
		}},
		{stage: "verification-run-required", command: func(receipt ReleaseHandoffPackMemoryCandidateDecisionReceipt) string {
			if receipt.ProvisionComplete {
				return strings.TrimSpace(receipt.VerificationCommand)
			}
			return ""
		}},
		{stage: "verification-provision-required", command: func(receipt ReleaseHandoffPackMemoryCandidateDecisionReceipt) string {
			if receipt.ProvisionStatus == "required" {
				return strings.TrimSpace(receipt.VerificationProvisionCommand)
			}
			return ""
		}},
	}
	for _, item := range stages {
		for _, receipt := range pack.DecisionReceipts {
			if item.command(receipt) != "" {
				return receipt, item.stage, true
			}
		}
	}
	return ReleaseHandoffPackMemoryCandidateDecisionReceipt{}, "", false
}

func packMemoryCandidateActionID(pack ReleaseHandoffPackMemoryCandidateStatus) string {
	if _, stage, ok := packMemoryCandidateActiveDecisionReceipt(pack); ok {
		return "pack-memory-" + stage
	}
	if pack.ProofSummary.NextMissingProof != nil {
		return packMemoryCandidateProofActionID(*pack.ProofSummary.NextMissingProof)
	}
	if pack.DecisionDraftHandoff != nil && strings.TrimSpace(pack.DecisionDraftHandoff.NextAction) != "" {
		return "pack-memory-decision-draft-required"
	}
	return packMemoryCandidateActionState(pack)
}

func packMemoryCandidateActionState(pack ReleaseHandoffPackMemoryCandidateStatus) string {
	if _, _, ok := packMemoryCandidateActiveDecisionReceipt(pack); ok {
		return "pack-memory-verification-required"
	}
	if pack.ProofSummary.NextMissingProof != nil {
		return "pack-memory-proof-required"
	}
	if pack.RequiresVerification {
		return "pack-memory-verification-required"
	}
	if pack.RequiresReview {
		return "pack-memory-review-required"
	}
	if pack.RequiresCleanup {
		return "pack-memory-cleanup-required"
	}
	if pack.HasOpenWork {
		return "pack-memory-open-work"
	}
	return "pack-memory-ready"
}

func packMemoryCandidateActionReasons(pack ReleaseHandoffPackMemoryCandidateStatus) []string {
	reasons := []string{"pack=" + pack.Pack, "actionId=" + packMemoryCandidateActionID(pack), pack.Action}
	if next := pack.ProofSummary.NextMissingProof; next != nil {
		reasons = append(reasons, "next missing proof="+next.ProofType, "proof path="+next.Path)
	}
	if receipt, stage, ok := packMemoryCandidateActiveDecisionReceipt(pack); ok {
		reasons = append(reasons, "candidate verification stage="+stage, "receipt="+receipt.Path)
		if strings.TrimSpace(receipt.ProvisionStatus) != "" {
			reasons = append(reasons, "provisionStatus="+receipt.ProvisionStatus)
		}
		if strings.TrimSpace(receipt.RetirementStatus) != "" {
			reasons = append(reasons, "retirementStatus="+receipt.RetirementStatus)
		}
		if strings.TrimSpace(receipt.VerificationProofPath) != "" {
			reasons = append(reasons, "verificationProof="+receipt.VerificationProofPath)
		}
		if strings.TrimSpace(receipt.VerificationWorkspaceRoot) != "" {
			reasons = append(reasons, "verificationWorkspace="+receipt.VerificationWorkspaceRoot)
		}
		if strings.TrimSpace(receipt.SourceCaseRoot) != "" {
			reasons = append(reasons, "sourceCaseRoot="+receipt.SourceCaseRoot)
		}
	}
	for _, evidence := range pack.Evidence {
		reasons = append(reasons, evidence)
	}
	return mission.UniqueStrings(reasons)
}

func packMemoryCandidateActionBoundary(pack ReleaseHandoffPackMemoryCandidateStatus) []string {
	boundary := append([]string{}, pack.Boundary...)
	boundary = append(boundary,
		"pack-memory action queue is read-only handoff; it does not merge, cleanup, provision, verify, retire, or write proof",
		"run WhatIf previews and hash-gated Apply commands explicitly before any pack-memory mutation",
	)
	if next := pack.ProofSummary.NextMissingProof; next != nil {
		boundary = append(boundary, next.Boundary...)
	}
	if _, stage, ok := packMemoryCandidateActiveDecisionReceipt(pack); ok {
		switch stage {
		case "verification-provision-required", "verification-provision-in-progress":
			boundary = append(boundary,
				"candidate verification provisioning is source-case-local and exact-hash gated",
				"provisioning does not run final verification, write authority/confirmed, or execute heavy tools",
			)
		case "verification-run-required":
			boundary = append(boundary,
				"candidate decision verification runs only after reviewing completed provisioning intent and receipt",
				"verification writes only bounded pack-memory verification proof; authority/confirmed remain deferred",
			)
		case "verification-retirement-required", "verification-retirement-in-progress":
			boundary = append(boundary,
				"candidate verification retirement is expected-hash gated and limited to the canonical verification workspace plus provisioning artifacts",
				"status/release-check never delete reappeared verification workspaces automatically",
			)
		}
	}
	return mission.UniqueStrings(boundary)
}

func releaseHandoffPackMemoryCandidateStatus(repo string, pack manifest.PackSummary) (ReleaseHandoffPackMemoryCandidateStatus, error) {
	packRoot := filepath.Join(repo, "packs", filepath.FromSlash(pack.ID))
	candidateRoot := filepath.Join(packRoot, "promote-candidates")
	toolingRoot := filepath.Join(packRoot, "tooling", "candidates")
	proofRoot := filepath.Join(candidateRoot, "review-artifacts")
	indexPath := filepath.Join(candidateRoot, "index.json")
	candidateRootRel := filepath.ToSlash(filepath.Join("packs", pack.ID, "promote-candidates"))
	toolingRootRel := filepath.ToSlash(filepath.Join("packs", pack.ID, "tooling", "candidates"))
	proofRootRel := filepath.ToSlash(filepath.Join(candidateRootRel, "review-artifacts"))
	candidatePaths, err := candidateFiles(candidateRoot, candidateRootRel)
	if err != nil {
		return ReleaseHandoffPackMemoryCandidateStatus{}, fmt.Errorf("pack-memory candidate scan failed for %s: %w", pack.ID, err)
	}
	toolingPaths, err := candidateFiles(toolingRoot, toolingRootRel)
	if err != nil {
		return ReleaseHandoffPackMemoryCandidateStatus{}, fmt.Errorf("pack-memory tooling candidate scan failed for %s: %w", pack.ID, err)
	}
	indexCandidates, indexExists, err := candidateIndexEntries(indexPath, repo, candidateRoot, candidateRootRel)
	if err != nil {
		return ReleaseHandoffPackMemoryCandidateStatus{}, fmt.Errorf("pack-memory candidate index invalid for %s: %w", pack.ID, err)
	}
	candidateFileCount := len(candidatePaths)
	toolingFileCount := len(toolingPaths)
	indexEntryCount := len(indexCandidates)
	indexRel := ""
	if indexExists {
		indexRel = filepath.ToSlash(filepath.Join(candidateRootRel, "index.json"))
	}
	receipts, err := packMemoryCandidateDecisionReceipts(repo, proofRoot, proofRootRel)
	if err != nil {
		return ReleaseHandoffPackMemoryCandidateStatus{}, fmt.Errorf("pack-memory candidate receipt scan failed for %s: %w", pack.ID, err)
	}
	pendingVerifications := 0
	completedVerifications := 0
	for _, receipt := range receipts {
		if receipt.VerificationPending && (!receipt.VerificationComplete || receipt.RetirementRequired || receipt.RetirementInProgress) {
			pendingVerifications++
		}
		if receipt.VerificationComplete && (!receipt.VerificationPending || receipt.Retired) {
			completedVerifications++
		}
	}
	status := ReleaseHandoffPackMemoryCandidateStatus{
		Pack:                   pack.ID,
		Maturity:               pack.Maturity,
		CandidateRoot:          candidateRootRel,
		ToolingRoot:            toolingRootRel,
		ProofRoot:              proofRootRel,
		IndexPath:              indexRel,
		CandidateFiles:         candidateFileCount,
		ToolingFiles:           toolingFileCount,
		IndexEntries:           indexEntryCount,
		CandidatePaths:         candidatePaths,
		ToolingPaths:           toolingPaths,
		IndexCandidates:        indexCandidates,
		DecisionReceipts:       receipts,
		PendingVerifications:   pendingVerifications,
		CompletedVerifications: completedVerifications,
		repoRootFull:           repo,
	}
	receiptCleanupArtifacts, err := packMemoryCandidateDecisionCleanupArtifacts(status, proofRoot)
	if err != nil {
		return ReleaseHandoffPackMemoryCandidateStatus{}, fmt.Errorf("pack-memory candidate cleanup proof scan failed for %s: %w", pack.ID, err)
	}
	receiptCleanupMissing := 0
	for _, artifact := range receiptCleanupArtifacts {
		if !artifact.ProofPresent {
			receiptCleanupMissing++
		}
	}
	status.HasOpenWork = candidateFileCount > 0 || toolingFileCount > 0 || indexEntryCount > 0 || indexExists || pendingVerifications > 0 || receiptCleanupMissing > 0
	if !status.HasOpenWork {
		return status, nil
	}
	status.RequiresReview = candidateFileCount > 0 || toolingFileCount > 0
	status.RequiresCleanup = candidateFileCount > 0 || toolingFileCount > 0 || indexEntryCount > 0 || indexExists || receiptCleanupMissing > 0
	status.RequiresVerification = pendingVerifications > 0
	status.Action = "review candidate files against pack targets, record accept/reject/superseded decisions, then cleanup candidatePath and indexPath"
	if !status.RequiresReview && status.RequiresCleanup {
		status.Action = "draft deterministic candidate-cleanup-proof for committed candidate decision receipts"
		if receiptCleanupMissing == 0 {
			status.Action = "cleanup stale pack-memory candidate indexPath or regenerate candidates before review"
		}
	}
	if !status.RequiresReview && !status.RequiresCleanup && status.RequiresVerification {
		status.Action = "run the candidate verification case provisioning WhatIf/expected-hash Apply, then run candidate decision verification WhatIf/Apply"
		for _, receipt := range receipts {
			if receipt.RetirementInProgress {
				status.Action = receipt.RetirementNextAction
				break
			}
			if receipt.RetirementRequired {
				status.Action = "run the candidate verification retirement WhatIf preview command; inspect the exact plan, then run its expected-hash Apply command"
				continue
			}
			if receipt.ProvisionInProgress {
				status.Action = receipt.ProvisionNextAction
				break
			}
			if receipt.ProvisionStatus == "required" || receipt.ProvisionComplete {
				status.Action = receipt.ProvisionNextAction
			}
		}
	}
	status.Evidence = append(status.Evidence, "candidateRoot "+candidateRootRel, "toolingRoot "+toolingRootRel)
	if candidateFileCount > 0 {
		status.Evidence = append(status.Evidence, fmt.Sprintf("promote-candidates files=%d", candidateFileCount))
	}
	if toolingFileCount > 0 {
		status.Evidence = append(status.Evidence, fmt.Sprintf("tooling/candidates files=%d", toolingFileCount))
	}
	if indexExists {
		status.Evidence = append(status.Evidence, fmt.Sprintf("indexPath %s entries=%d", indexRel, indexEntryCount))
	}
	if len(receipts) > 0 {
		status.Evidence = append(status.Evidence, fmt.Sprintf("candidate decision receipts=%d pendingVerification=%d completedVerification=%d", len(receipts), pendingVerifications, completedVerifications))
	}
	reviewArtifacts, err := packMemoryCandidateReviewArtifacts(status, proofRoot)
	if err != nil {
		return ReleaseHandoffPackMemoryCandidateStatus{}, fmt.Errorf("pack-memory candidate review proof scan failed for %s: %w", pack.ID, err)
	}
	status.ReviewArtifacts = reviewArtifacts
	status.ReviewArtifacts = append(status.ReviewArtifacts, receiptCleanupArtifacts...)
	status.DecisionDraftHandoff = packMemoryCandidateDecisionDraftHandoff(status)
	status.ProofSummary = packMemoryCandidateReviewProofSummary(status)
	status.ReviewSummary = packMemoryCandidateReviewSummary(status)
	status.Boundary = []string{
		"release handoff inventories candidate residue and durable decision verification receipts; it does not merge, delete, or validate cases",
		"review candidates before merge and explicitly verify accepted decisions; do not write authority/confirmed",
		"do not promote case-specific artifacts, traces, dumps, captures, payloads, flags, or customer data",
	}
	return status, nil
}

func packMemoryCandidateDecisionDraftHandoff(status ReleaseHandoffPackMemoryCandidateStatus) *promote.CandidateDecisionDraftHandoff {
	if !status.RequiresReview || strings.TrimSpace(status.ProofRoot) == "" {
		return nil
	}
	evidenceRefs := packMemoryCandidateDecisionDraftEvidenceRefs(status)
	handoff := promote.CandidateDecisionDraftHandoff{
		Mode:               "candidate-decision-draft-review-workspace-required",
		DecisionPath:       filepath.ToSlash(filepath.Join(status.ProofRoot, "candidate-decisions.json")),
		EvidenceRefs:       evidenceRefs,
		DefaultReason:      "reviewed pack-memory candidate inventory, decision proofs, and cleanup/reconsume expectations",
		DefaultActor:       "mission-commander",
		SupportedDecisions: packMemoryCandidateDecisionDraftSupportedDecisions(status),
		Boundary: []string{
			"release/status handoff cannot infer the case-local review packet; run promote -CreateCandidates -Review from the attached source case to materialize packet-bound draft commands",
			"draft preview must be run only after reviewing packet.json, bounded diffs, sanitized previews, and evidence refs",
			"WhatIf returns decisionSha256; Apply must use the exact returned -ExpectedDecisionSha256",
			"drafting does not merge candidates, cleanup candidate files, run doctor/init/reconsume, write authority/confirmed, or execute heavy tools",
		},
	}
	if len(evidenceRefs) == 0 {
		handoff.NextAction = "write or select at least one repo-local pack-memory review evidence ref before running promote -DraftCandidateDecision"
		return &handoff
	}
	handoff.NextAction = "rerun promote -CreateCandidates -Review from the attached source case, then use the packet decisionDraftHandoff preview/apply commands"
	return &handoff
}

func packMemoryCandidateDecisionDraftEvidenceRefs(status ReleaseHandoffPackMemoryCandidateStatus) []string {
	out := []string{}
	seen := map[string]bool{}
	for _, artifact := range status.ReviewArtifacts {
		path := strings.TrimSpace(artifact.ProofPath)
		if path == "" {
			continue
		}
		clean := filepath.ToSlash(filepath.Clean(path))
		if seen[clean] {
			continue
		}
		seen[clean] = true
		out = append(out, clean)
	}
	return out
}

func packMemoryCandidateDecisionDraftSupportedDecisions(status ReleaseHandoffPackMemoryCandidateStatus) []string {
	if status.ToolingFiles > 0 {
		return []string{"accept-managed-reject-tooling", "reject", "superseded"}
	}
	return []string{"accept", "accept-managed-reject-tooling", "reject", "superseded"}
}

func packMemoryCandidateReviewSummary(status ReleaseHandoffPackMemoryCandidateStatus) ReleaseHandoffPackMemoryCandidateReviewSummary {
	summary := ReleaseHandoffPackMemoryCandidateReviewSummary{
		Total:                 status.CandidateFiles + status.ToolingFiles + status.IndexEntries,
		CandidateFiles:        status.CandidateFiles,
		ToolingFiles:          status.ToolingFiles,
		IndexEntries:          status.IndexEntries,
		ReviewArtifactCount:   len(status.ReviewArtifacts),
		ProofSummary:          status.ProofSummary,
		CandidateRoot:         status.CandidateRoot,
		ToolingRoot:           status.ToolingRoot,
		IndexPath:             status.IndexPath,
		RequiresReview:        status.RequiresReview,
		RequiresCleanup:       status.RequiresCleanup,
		HasCandidatePaths:     len(status.CandidatePaths) > 0,
		HasToolingPaths:       len(status.ToolingPaths) > 0,
		HasIndex:              strings.TrimSpace(status.IndexPath) != "",
		HasDecisionArtifacts:  false,
		HasCleanupArtifacts:   false,
		HasReconsumeArtifacts: false,
		NextAction:            status.Action,
	}
	if summary.Total == 0 && summary.HasIndex {
		summary.Total = 1
	}
	for _, artifact := range status.ReviewArtifacts {
		switch strings.TrimSpace(artifact.Name) {
		case "candidate-decision-note", "blocked-review-note":
			summary.DecisionArtifactCount++
			summary.HasDecisionArtifacts = true
		case "candidate-cleanup-proof":
			summary.CleanupArtifactCount++
			summary.HasCleanupArtifacts = true
		case "pack-doctor-output", "fresh-case-reconsume-proof", "attached-case-reconsume-proof":
			summary.ReconsumeArtifactCount++
			summary.HasReconsumeArtifacts = true
		}
	}
	if status.HasOpenWork {
		summary.Boundary = []string{
			"pack-memory reviewSummary is read-only; full candidate paths, indexCandidates, and reviewArtifacts remain available",
			"release/status handoff does not merge or delete candidates; deterministic cleanup proof draft still requires explicit promote -DraftReviewProof WhatIf/Apply",
			"review candidates before merge; do not write authority/confirmed or execute heavy tools",
		}
	}
	return summary
}

func packMemoryCandidateReviewArtifacts(status ReleaseHandoffPackMemoryCandidateStatus, proofRoot string) ([]ReleaseHandoffPackMemoryCandidateReviewArtifact, error) {
	artifacts := []ReleaseHandoffPackMemoryCandidateReviewArtifact{}
	baseBoundary := []string{
		"review artifact is guidance only; release/status does not write decision, cleanup, or reconsume proof",
		"deterministic proof drafts require explicit promote -DraftReviewProof WhatIf/Apply",
		"do not write authority/confirmed",
		"do not execute heavy tools",
	}
	for _, path := range status.CandidatePaths {
		packTarget := packMemoryCandidatePackTarget(status.IndexCandidates, path)
		proofPackTarget := packMemoryCandidateProofPackTarget(status, packTarget)
		artifacts = append(artifacts,
			ReleaseHandoffPackMemoryCandidateReviewArtifact{
				Name:          "candidate-decision-note",
				CandidatePath: path,
				PackTarget:    packTarget,
				When:          "before merge, cleanup, or reconsume; choose accept, reject, or superseded for this candidate",
				Action:        "record reviewed decision and selected decisionFollowThrough outcome outside authority/confirmed stores",
				Format:        "strict JSON pack-memory-candidate-review-proof note with decision, reason, candidatePath, packTarget, reviewItem, evidenceRefs, and boundary",
				Evidence:      []string{"decision note path/ref", "selected decisionFollowThrough outcome"},
				Boundary:      append([]string{}, baseBoundary...),
			},
			ReleaseHandoffPackMemoryCandidateReviewArtifact{
				Name:          "candidate-cleanup-proof",
				CandidatePath: path,
				PackTarget:    proofPackTarget,
				When:          "after deleting candidatePath because it was rejected, superseded, or accepted and merged into pack source",
				Action:        "record candidatePath deletion check and indexPath update/removal proof",
				Format:        "strict JSON pack-memory-candidate-lifecycle-proof with candidate-absent and index-entry-absent checks plus hashed evidenceRefs",
				Evidence:      []string{"candidatePath deletion check", "indexPath update/removal check"},
				Boundary: append(append([]string{}, baseBoundary...),
					"cleanup is limited to candidateRoot/toolingRoot and indexPath",
					"do not delete pack source files",
				),
			},
			ReleaseHandoffPackMemoryCandidateReviewArtifact{
				Name:          "pack-doctor-output",
				CandidatePath: path,
				PackTarget:    proofPackTarget,
				When:          "after an accept decision merges reusable content into packTarget",
				Action:        "record doctor command output before declaring accepted merge complete",
				Format:        "strict JSON pack-memory-candidate-lifecycle-proof with a passed pack-doctor check plus hashed evidenceRefs",
				Evidence:      []string{"doctor command output"},
				Boundary: append(append([]string{}, baseBoundary...),
					"doctor validates pack state only",
					"do not create case-local artifacts while checking pack",
				),
			},
		)
	}
	for _, path := range status.ToolingPaths {
		artifacts = append(artifacts,
			ReleaseHandoffPackMemoryCandidateReviewArtifact{
				Name:          "candidate-decision-note",
				CandidatePath: path,
				PackTarget:    packMemoryCandidateToolingProofPackTarget(status),
				When:          "before merge, cleanup, or reconsume; choose accept, reject, or superseded for this tooling candidate",
				Action:        "record reviewed decision and selected decisionFollowThrough outcome outside authority/confirmed stores",
				Format:        "strict JSON pack-memory-candidate-review-proof note with decision, reason, candidatePath, packTarget, reviewItem, evidenceRefs, and boundary",
				Evidence:      []string{"decision note path/ref", "selected decisionFollowThrough outcome"},
				Boundary:      append([]string{}, baseBoundary...),
			},
			ReleaseHandoffPackMemoryCandidateReviewArtifact{
				Name:          "candidate-cleanup-proof",
				CandidatePath: path,
				PackTarget:    packMemoryCandidateToolingProofPackTarget(status),
				When:          "after deleting candidatePath because it was rejected, superseded, or accepted and merged into pack tooling",
				Action:        "record candidatePath deletion check and tooling candidate directory cleanup proof",
				Format:        "strict JSON pack-memory-candidate-lifecycle-proof with candidate-absent check plus hashed evidenceRefs",
				Evidence:      []string{"candidatePath deletion check", "tooling/candidates cleanup check"},
				Boundary: append(append([]string{}, baseBoundary...),
					"cleanup is limited to candidateRoot/toolingRoot and indexPath",
					"do not delete pack source files",
				),
			},
			ReleaseHandoffPackMemoryCandidateReviewArtifact{
				Name:          "pack-doctor-output",
				CandidatePath: path,
				PackTarget:    packMemoryCandidateToolingProofPackTarget(status),
				When:          "after an accept decision merges reusable tooling into packTarget",
				Action:        "record doctor command output before declaring accepted tooling merge complete",
				Format:        "strict JSON pack-memory-candidate-lifecycle-proof with a passed pack-doctor check plus hashed evidenceRefs",
				Evidence:      []string{"doctor command output"},
				Boundary: append(append([]string{}, baseBoundary...),
					"doctor validates pack state only",
					"do not create case-local artifacts while checking pack",
				),
			},
			ReleaseHandoffPackMemoryCandidateReviewArtifact{
				Name:          "fresh-case-reconsume-proof",
				CandidatePath: path,
				PackTarget:    packMemoryCandidateToolingProofPackTarget(status),
				When:          "after accepting tooling candidate into tooling/catalog.yml or tooling/recipes/*",
				Action:        "record temporary fresh-case init and doctor output proving pack tooling reconsume",
				Format:        "strict JSON pack-memory-candidate-lifecycle-proof with passed fresh-case-reconsume and pack-doctor checks plus hashed evidenceRefs",
				Evidence:      []string{"fresh case .rekit/instance.yml", "fresh case doctor output"},
				Boundary: append(append([]string{}, baseBoundary...),
					"use a temporary fresh case only",
					"do not create real case state in the kit repo",
					"sync does not copy tooling recipes into case-local managed docs",
				),
			},
			ReleaseHandoffPackMemoryCandidateReviewArtifact{
				Name:          "attached-case-reconsume-proof",
				CandidatePath: path,
				PackTarget:    packMemoryCandidateToolingProofPackTarget(status),
				When:          "when validating an existing attached case after accepted tooling merge",
				Action:        "record attached-case doctor output proving pack tooling is resolved through templateRoot/templatePack",
				Format:        "strict JSON pack-memory-candidate-lifecycle-proof with passed attached-case-reconsume and pack-doctor checks plus hashed evidenceRefs",
				Evidence:      []string{"attached case doctor output"},
				Boundary: append(append([]string{}, baseBoundary...),
					"do not overwrite case-local files while checking reconsume",
					"fresh/attached case verification reads pack tooling through templateRoot/templatePack",
				),
			},
		)
	}
	if len(artifacts) == 0 && status.IndexPath != "" {
		artifacts = append(artifacts, ReleaseHandoffPackMemoryCandidateReviewArtifact{
			Name:          "candidate-cleanup-proof",
			CandidatePath: status.IndexPath,
			When:          "after confirming stale indexPath has no matching candidate files",
			Action:        "record indexPath removal or regeneration proof",
			Format:        "strict JSON pack-memory-candidate-lifecycle-proof with index removal/regeneration checks plus hashed evidenceRefs",
			Evidence:      []string{"indexPath update/removal check"},
			Boundary: append(append([]string{}, baseBoundary...),
				"cleanup is limited to candidateRoot/toolingRoot and indexPath",
			),
		})
	}
	for i := range artifacts {
		artifact, err := packMemoryCandidateReviewArtifactWithProof(status, artifacts[i], proofRoot)
		if err != nil {
			return nil, err
		}
		artifacts[i] = artifact
	}
	return artifacts, nil
}

func packMemoryCandidateDecisionCleanupArtifacts(status ReleaseHandoffPackMemoryCandidateStatus, proofRoot string) ([]ReleaseHandoffPackMemoryCandidateReviewArtifact, error) {
	artifacts := []ReleaseHandoffPackMemoryCandidateReviewArtifact{}
	baseBoundary := []string{
		"review artifact is guidance only; release/status does not write decision, cleanup, or reconsume proof",
		"deterministic proof drafts require explicit promote -DraftReviewProof WhatIf/Apply",
		"do not write authority/confirmed",
		"do not execute heavy tools",
		"cleanup proof is derived from a durable candidate decision receipt after candidatePath and index cleanup",
		"cleanup proof presence requires strict JSON proof binding to receipt, transaction, committed marker, backup hashes, and current cleanup state",
	}
	for _, receipt := range status.DecisionReceipts {
		for _, action := range receipt.Actions {
			stem := packMemoryCandidateProofStem(action.CandidatePath, action.PackTarget)
			proofPath := filepath.ToSlash(filepath.Join(status.ProofRoot, stem+".candidate-cleanup-proof.md"))
			artifact := ReleaseHandoffPackMemoryCandidateReviewArtifact{
				Name:                  "candidate-cleanup-proof",
				CandidatePath:         action.CandidatePath,
				PackTarget:            action.PackTarget,
				SourceCaseRoot:        receipt.SourceCaseRoot,
				PacketPath:            receipt.PacketPath,
				CandidateDecisionPath: receipt.DecisionPath,
				When:                  "after candidate decision Apply committed receipt cleanup for this reviewed candidate",
				Action:                "draft deterministic cleanup proof bound to candidate review packet, candidate decision file, receipt, transaction journal, committed marker, and backup hashes",
				Format:                "strict JSON proof note generated by promote -DraftReviewProof -ProofType candidate-cleanup-proof",
				Evidence:              []string{"candidate decision receipt", "transaction journal", "committed marker", "candidate backup hash", "candidatePath absent check", "index entry absent check"},
				Boundary: append(append([]string{}, baseBoundary...),
					"cleanup proof draft does not merge, delete, run doctor/init/reconsume, or validate cases",
				),
			}
			artifact.ExpectedProofs = []string{proofPath}
			matchedProof, proofPresent, err := packMemoryCandidateDecisionCleanupProofPath(status, receipt, action, proofRoot, stem)
			if err != nil {
				return nil, err
			}
			if proofPresent {
				artifact.ProofPath = releaseHandoffRepoRelative(status.repoRootFull, matchedProof)
				artifact.ProofPresent = true
			}
			artifacts = append(artifacts, artifact)
		}
	}
	return artifacts, nil
}

func packMemoryCandidateDecisionCleanupProofPath(status ReleaseHandoffPackMemoryCandidateStatus, receipt ReleaseHandoffPackMemoryCandidateDecisionReceipt, action ReleaseHandoffPackMemoryCandidateDecisionReceiptAction, proofRoot, stem string) (string, bool, error) {
	for _, ext := range []string{".md", ".json", ".txt"} {
		candidate := filepath.Join(proofRoot, stem+".candidate-cleanup-proof"+ext)
		info, err := os.Lstat(candidate)
		if os.IsNotExist(err) {
			continue
		}
		if err != nil {
			return "", false, err
		}
		if info.Mode()&os.ModeSymlink != 0 || !info.Mode().IsRegular() || info.Size() == 0 || info.Size() > 1024*1024 {
			return "", false, fmt.Errorf("candidate cleanup proof must be a non-empty regular file: %s", candidate)
		}
		if err := validatePackMemoryCandidateCleanupProof(status, receipt, action, proofRoot, candidate); err != nil {
			return "", false, err
		}
		return candidate, true, nil
	}

	entries, err := os.ReadDir(proofRoot)
	if os.IsNotExist(err) {
		return "", false, nil
	}
	if err != nil {
		return "", false, err
	}
	for _, entry := range entries {
		if entry.IsDir() || !candidateCleanupProofArtifactName(entry.Name()) {
			continue
		}
		candidate := filepath.Join(proofRoot, entry.Name())
		info, err := entry.Info()
		if err != nil {
			return "", false, err
		}
		if info.Mode()&os.ModeSymlink != 0 || !info.Mode().IsRegular() || info.Size() == 0 || info.Size() > 1024*1024 {
			return "", false, fmt.Errorf("candidate cleanup proof must be a non-empty regular file: %s", candidate)
		}
		data, err := os.ReadFile(candidate)
		if err != nil {
			return "", false, err
		}
		var proof candidateReviewProofNoteInventory
		if err := decodeReleaseHandoffStrictJSON(data, &proof); err != nil {
			return "", false, fmt.Errorf("decode candidate cleanup proof %s: %w", candidate, err)
		}
		if !candidateCleanupProofTargetsReceiptAction(status, receipt, action, proof) {
			continue
		}
		if err := validatePackMemoryCandidateCleanupProof(status, receipt, action, proofRoot, candidate); err != nil {
			return "", false, err
		}
		return candidate, true, nil
	}
	return "", false, nil
}

func candidateCleanupProofArtifactName(name string) bool {
	for _, ext := range []string{".md", ".json", ".txt"} {
		if strings.HasSuffix(name, ".candidate-cleanup-proof"+ext) {
			return true
		}
	}
	return false
}

func candidateCleanupProofTargetsReceiptAction(status ReleaseHandoffPackMemoryCandidateStatus, receipt ReleaseHandoffPackMemoryCandidateDecisionReceipt, action ReleaseHandoffPackMemoryCandidateDecisionReceiptAction, proof candidateReviewProofNoteInventory) bool {
	cleanup := proof.Cleanup
	if cleanup == nil || proof.SchemaVersion != 1 || proof.Kind != "pack-memory-candidate-review-proof" || proof.Pack != status.Pack || proof.ProofType != "candidate-cleanup-proof" {
		return false
	}
	if !strings.EqualFold(proof.PacketHash, receipt.packetHash) || !strings.EqualFold(proof.DecisionHash, receipt.decisionHash) {
		return false
	}
	if proof.CandidatePath != action.CandidatePath || proof.PackTarget != action.PackTarget || proof.Decision != action.Decision {
		return false
	}
	return cleanup.DecisionReceiptPath == releaseHandoffRepoRelative(status.repoRootFull, receipt.pathFull) && strings.EqualFold(cleanup.DecisionReceiptHash, receipt.receiptHash)
}

func validatePackMemoryCandidateDecisionProof(status ReleaseHandoffPackMemoryCandidateStatus, artifact ReleaseHandoffPackMemoryCandidateReviewArtifact, proofRoot, path string) error {
	repo := strings.TrimSpace(status.repoRootFull)
	if repo == "" {
		return fmt.Errorf("candidate decision proof validation lacks repo authority: %s", path)
	}
	if !pathWithinReleaseHandoffRoot(proofRoot, path) {
		return fmt.Errorf("candidate decision proof leaves proof root: %s", path)
	}
	data, err := os.ReadFile(path)
	if err != nil {
		return err
	}
	var proof candidateReviewProofNoteInventory
	if err := decodeReleaseHandoffStrictJSON(data, &proof); err != nil {
		return fmt.Errorf("decode candidate decision proof %s: %w", path, err)
	}
	kind, candidateRoot, err := releaseHandoffCandidateDecisionProofKind(status, artifact.CandidatePath)
	if err != nil {
		return err
	}
	candidateFull := filepath.Join(repo, filepath.FromSlash(artifact.CandidatePath))
	if err := validateReleaseHandoffHashedFile(candidateRoot, candidateFull, proof.CandidateHash, "candidate decision proof candidate"); err != nil {
		return err
	}
	expectedPackTarget := releaseHandoffCandidateDecisionProofPackTarget(status, artifact, kind)
	if proof.Cleanup != nil || proof.SchemaVersion != 1 || proof.Kind != "pack-memory-candidate-review-proof" || proof.Pack != status.Pack || proof.ProofType != "candidate-decision-note" || strings.TrimSpace(proof.PacketHash) == "" || strings.TrimSpace(proof.DecisionHash) != "" || proof.CandidatePath != artifact.CandidatePath || proof.PackTarget != expectedPackTarget || strings.TrimSpace(proof.Reason) == "" || strings.TrimSpace(proof.Actor) == "" || len(proof.Boundary) == 0 {
		return fmt.Errorf("candidate decision proof binding mismatch: %s", path)
	}
	if !candidateCleanupProofStoresOnlyRelativePaths(proof) {
		return fmt.Errorf("candidate decision proof stores absolute or escaping path: %s", path)
	}
	if proof.ReviewItem.CandidatePath != proof.CandidatePath || proof.ReviewItem.CandidateHash != proof.CandidateHash || proof.ReviewItem.Kind != kind || proof.ReviewItem.PackTarget != proof.PackTarget {
		return fmt.Errorf("candidate decision proof review item binding mismatch: %s", path)
	}
	switch proof.Decision {
	case "accept":
		if kind != "managed-doc" || strings.TrimSpace(expectedPackTarget) == "" {
			return fmt.Errorf("candidate decision proof cannot accept this candidate: %s", path)
		}
		packTargetFull := filepath.Join(repo, filepath.FromSlash(expectedPackTarget))
		packRoot := filepath.Join(repo, "packs", filepath.FromSlash(status.Pack))
		if !pathWithinReleaseHandoffRoot(packRoot, packTargetFull) {
			return fmt.Errorf("candidate decision proof packTarget leaves pack root: %s", proof.PackTarget)
		}
		packTargetHash := fileSHA256ReleaseHandoff(packTargetFull)
		if strings.TrimSpace(packTargetHash) == "" || !strings.EqualFold(proof.PackTargetHash, packTargetHash) {
			return fmt.Errorf("candidate decision proof packTarget hash mismatch: %s", path)
		}
	case "reject", "superseded":
		if strings.TrimSpace(proof.PackTargetHash) != "" {
			return fmt.Errorf("candidate decision proof unexpected packTarget hash: %s", path)
		}
	default:
		return fmt.Errorf("candidate decision proof has unsupported decision: %s", path)
	}
	if err := validateCandidateDecisionProofEvidenceRefs(repo, proof.EvidenceRefs); err != nil {
		return err
	}
	return nil
}

func releaseHandoffCandidateDecisionProofKind(status ReleaseHandoffPackMemoryCandidateStatus, candidatePath string) (string, string, error) {
	clean := filepath.ToSlash(filepath.Clean(filepath.FromSlash(candidatePath)))
	candidateRoot := strings.TrimRight(filepath.ToSlash(status.CandidateRoot), "/")
	toolingRoot := strings.TrimRight(filepath.ToSlash(status.ToolingRoot), "/")
	switch {
	case toolingRoot != "" && strings.HasPrefix(clean, toolingRoot+"/"):
		return "tooling-candidate-source", filepath.Join(status.repoRootFull, filepath.FromSlash(status.ToolingRoot)), nil
	case candidateRoot != "" && strings.HasPrefix(clean, candidateRoot+"/"):
		return "managed-doc", filepath.Join(status.repoRootFull, filepath.FromSlash(status.CandidateRoot)), nil
	default:
		return "", "", fmt.Errorf("candidate decision proof candidate leaves known candidate roots: %s", candidatePath)
	}
}

func releaseHandoffCandidateDecisionProofPackTarget(status ReleaseHandoffPackMemoryCandidateStatus, artifact ReleaseHandoffPackMemoryCandidateReviewArtifact, kind string) string {
	if kind == "tooling-candidate-source" {
		return filepath.ToSlash(filepath.Dir(filepath.FromSlash(status.ToolingRoot)))
	}
	packTarget := strings.TrimSpace(artifact.PackTarget)
	if packTarget == "" {
		return ""
	}
	if strings.HasPrefix(filepath.ToSlash(packTarget), "packs/") {
		return filepath.ToSlash(packTarget)
	}
	return filepath.ToSlash(filepath.Join("packs", status.Pack, filepath.FromSlash(packTarget)))
}

func validateCandidateDecisionProofEvidenceRefs(repo string, evidenceRefs []candidateDecisionEvidenceInventory) error {
	if len(evidenceRefs) == 0 {
		return fmt.Errorf("candidate decision proof requires non-empty evidenceRefs")
	}
	seen := map[string]bool{}
	for _, evidence := range evidenceRefs {
		if strings.TrimSpace(evidence.Path) == "" || strings.TrimSpace(evidence.SHA256) == "" {
			return fmt.Errorf("candidate decision proof evidence path or hash is empty")
		}
		if !releaseHandoffStoredRelativePath(evidence.Path) {
			return fmt.Errorf("candidate decision proof evidence stores absolute or escaping path: %s", evidence.Path)
		}
		key := filepath.ToSlash(filepath.Clean(filepath.FromSlash(evidence.Path)))
		if seen[key] {
			return fmt.Errorf("candidate decision proof evidence duplicated: %s", evidence.Path)
		}
		seen[key] = true
		full := filepath.Join(repo, filepath.FromSlash(evidence.Path))
		if err := rejectReleaseHandoffSymlinkPath(repo, full, true); err != nil {
			return err
		}
		info, err := os.Lstat(full)
		if os.IsNotExist(err) {
			continue
		}
		if err != nil {
			return err
		}
		if info.Mode()&os.ModeSymlink != 0 || !info.Mode().IsRegular() || info.Size() == 0 || info.Size() > 1024*1024 || !strings.EqualFold(fileSHA256ReleaseHandoff(full), evidence.SHA256) {
			return fmt.Errorf("candidate decision proof evidence hash mismatch: %s", evidence.Path)
		}
	}
	return nil
}

func validatePackMemoryCandidateLifecycleProof(status ReleaseHandoffPackMemoryCandidateStatus, artifact ReleaseHandoffPackMemoryCandidateReviewArtifact, proofRoot, path string) error {
	repo := strings.TrimSpace(status.repoRootFull)
	if repo == "" {
		return fmt.Errorf("candidate lifecycle proof validation lacks repo authority: %s", path)
	}
	if !pathWithinReleaseHandoffRoot(proofRoot, path) {
		return fmt.Errorf("candidate lifecycle proof leaves proof root: %s", path)
	}
	data, err := os.ReadFile(path)
	if err != nil {
		return err
	}
	var proof candidateLifecycleProofInventory
	if err := decodeReleaseHandoffStrictJSON(data, &proof); err != nil {
		return fmt.Errorf("decode candidate lifecycle proof %s: %w", path, err)
	}
	if proof.SchemaVersion != 1 || proof.Kind != "pack-memory-candidate-lifecycle-proof" || proof.Pack != status.Pack || proof.ProofType != artifact.Name || proof.CandidatePath != artifact.CandidatePath || proof.PackTarget != artifact.PackTarget || strings.TrimSpace(proof.Reason) == "" || strings.TrimSpace(proof.Actor) == "" || len(proof.Boundary) == 0 || len(proof.Checks) == 0 {
		return fmt.Errorf("candidate lifecycle proof binding mismatch: %s", path)
	}
	if proof.ReviewItem.CandidatePath != proof.CandidatePath || proof.ReviewItem.PackTarget != proof.PackTarget || proof.ReviewItem.ProofType != proof.ProofType || proof.ReviewItem.Stage != packMemoryCandidateProofArtifactStage(artifact.Name) {
		return fmt.Errorf("candidate lifecycle proof review item binding mismatch: %s", path)
	}
	if !candidateLifecycleProofStoresOnlyRelativePaths(proof) {
		return fmt.Errorf("candidate lifecycle proof stores absolute or escaping path: %s", path)
	}
	switch proof.ProofType {
	case "candidate-cleanup-proof":
		if err := validateOpenCandidateLifecycleCleanupProof(repo, status, artifact, proof); err != nil {
			return err
		}
	case "pack-doctor-output":
		if !candidateLifecycleProofCheckPassed(proof.Checks, "pack-doctor") {
			return fmt.Errorf("candidate lifecycle proof missing pack-doctor check: %s", path)
		}
	case "fresh-case-reconsume-proof":
		if !candidateLifecycleProofCheckPassed(proof.Checks, "fresh-case-reconsume") || !candidateLifecycleProofCheckPassed(proof.Checks, "pack-doctor") {
			return fmt.Errorf("candidate lifecycle proof missing fresh-case reconsume checks: %s", path)
		}
	case "attached-case-reconsume-proof":
		if !candidateLifecycleProofCheckPassed(proof.Checks, "attached-case-reconsume") || !candidateLifecycleProofCheckPassed(proof.Checks, "pack-doctor") {
			return fmt.Errorf("candidate lifecycle proof missing attached-case reconsume checks: %s", path)
		}
	default:
		return fmt.Errorf("candidate lifecycle proof has unsupported proofType: %s", path)
	}
	if err := validateCandidateLifecycleProofEvidenceRefs(repo, proof.EvidenceRefs); err != nil {
		return err
	}
	return nil
}

func validateOpenCandidateLifecycleCleanupProof(repo string, status ReleaseHandoffPackMemoryCandidateStatus, artifact ReleaseHandoffPackMemoryCandidateReviewArtifact, proof candidateLifecycleProofInventory) error {
	if !candidateLifecycleProofCheckPassed(proof.Checks, "candidate-absent") {
		return fmt.Errorf("candidate lifecycle cleanup proof missing candidate-absent check: %s", artifact.CandidatePath)
	}
	candidateFull := filepath.Join(repo, filepath.FromSlash(artifact.CandidatePath))
	candidateRoot := filepath.Join(repo, filepath.FromSlash(status.CandidateRoot))
	if strings.HasPrefix(filepath.ToSlash(artifact.CandidatePath), strings.TrimRight(filepath.ToSlash(status.ToolingRoot), "/")+"/") {
		candidateRoot = filepath.Join(repo, filepath.FromSlash(status.ToolingRoot))
	}
	if err := rejectReleaseHandoffSymlinkPath(candidateRoot, candidateFull, true); err != nil {
		return err
	}
	if _, err := os.Lstat(candidateFull); !os.IsNotExist(err) {
		if err != nil {
			return err
		}
		return fmt.Errorf("candidate lifecycle cleanup proof candidate still exists: %s", artifact.CandidatePath)
	}
	if strings.HasPrefix(filepath.ToSlash(artifact.CandidatePath), strings.TrimRight(filepath.ToSlash(status.CandidateRoot), "/")+"/") {
		if !candidateLifecycleProofCheckPassed(proof.Checks, "index-entry-absent") {
			return fmt.Errorf("candidate lifecycle cleanup proof missing index-entry-absent check: %s", artifact.CandidatePath)
		}
		if strings.TrimSpace(status.IndexPath) != "" {
			indexFull := filepath.Join(repo, filepath.FromSlash(status.IndexPath))
			contains, err := releaseHandoffCandidateIndexContains(status, indexFull, artifact.CandidatePath)
			if err != nil && !os.IsNotExist(err) {
				return err
			}
			if contains {
				return fmt.Errorf("candidate lifecycle cleanup proof index still contains candidate: %s", artifact.CandidatePath)
			}
		}
	}
	return nil
}

func candidateLifecycleProofCheckPassed(checks []candidateLifecycleProofCheckInventory, name string) bool {
	for _, check := range checks {
		if check.Name == name && check.Status == "passed" && strings.TrimSpace(check.Summary) != "" {
			return true
		}
	}
	return false
}

func validateCandidateLifecycleProofEvidenceRefs(repo string, evidenceRefs []candidateDecisionEvidenceInventory) error {
	if len(evidenceRefs) == 0 {
		return fmt.Errorf("candidate lifecycle proof requires non-empty evidenceRefs")
	}
	seen := map[string]bool{}
	for _, evidence := range evidenceRefs {
		if strings.TrimSpace(evidence.Path) == "" || strings.TrimSpace(evidence.SHA256) == "" {
			return fmt.Errorf("candidate lifecycle proof evidence path or hash is empty")
		}
		if !releaseHandoffStoredRelativePath(evidence.Path) {
			return fmt.Errorf("candidate lifecycle proof evidence stores absolute or escaping path: %s", evidence.Path)
		}
		key := filepath.ToSlash(filepath.Clean(filepath.FromSlash(evidence.Path)))
		if seen[key] {
			return fmt.Errorf("candidate lifecycle proof evidence duplicated: %s", evidence.Path)
		}
		seen[key] = true
		full := filepath.Join(repo, filepath.FromSlash(evidence.Path))
		if err := rejectReleaseHandoffSymlinkPath(repo, full, false); err != nil {
			return err
		}
		info, err := os.Lstat(full)
		if err != nil {
			return err
		}
		if info.Mode()&os.ModeSymlink != 0 || !info.Mode().IsRegular() || info.Size() == 0 || info.Size() > 1024*1024 || !strings.EqualFold(fileSHA256ReleaseHandoff(full), evidence.SHA256) {
			return fmt.Errorf("candidate lifecycle proof evidence hash mismatch: %s", evidence.Path)
		}
	}
	return nil
}

func candidateLifecycleProofStoresOnlyRelativePaths(proof candidateLifecycleProofInventory) bool {
	paths := []string{proof.CandidatePath, proof.PackTarget, proof.ReviewItem.CandidatePath, proof.ReviewItem.PackTarget}
	for _, evidence := range proof.EvidenceRefs {
		paths = append(paths, evidence.Path)
	}
	for _, path := range paths {
		if !releaseHandoffStoredRelativePath(path) {
			return false
		}
	}
	return true
}

func validatePackMemoryCandidateCleanupProof(status ReleaseHandoffPackMemoryCandidateStatus, receipt ReleaseHandoffPackMemoryCandidateDecisionReceipt, action ReleaseHandoffPackMemoryCandidateDecisionReceiptAction, proofRoot, path string) error {
	repo := strings.TrimSpace(status.repoRootFull)
	if repo == "" || strings.TrimSpace(receipt.pathFull) == "" || strings.TrimSpace(receipt.backupRootFull) == "" {
		return fmt.Errorf("candidate cleanup proof validation lacks receipt authority fields: %s", path)
	}
	if !pathWithinReleaseHandoffRoot(proofRoot, path) {
		return fmt.Errorf("candidate cleanup proof leaves proof root: %s", path)
	}
	if err := rejectReleaseHandoffSymlinkPath(repo, path, false); err != nil {
		return err
	}
	data, err := os.ReadFile(path)
	if err != nil {
		return err
	}
	var proof candidateReviewProofNoteInventory
	if err := decodeReleaseHandoffStrictJSON(data, &proof); err != nil {
		return fmt.Errorf("decode candidate cleanup proof %s: %w", path, err)
	}
	cleanup := proof.Cleanup
	if cleanup == nil || proof.SchemaVersion != 1 || proof.Kind != "pack-memory-candidate-review-proof" || proof.Pack != status.Pack || proof.ProofType != "candidate-cleanup-proof" || !strings.EqualFold(proof.PacketHash, receipt.packetHash) || !strings.EqualFold(proof.DecisionHash, receipt.decisionHash) || proof.CandidatePath != action.CandidatePath || proof.PackTarget != action.PackTarget || proof.Decision != action.Decision || strings.TrimSpace(proof.Reason) == "" || strings.TrimSpace(proof.Actor) == "" || len(proof.Boundary) == 0 {
		return fmt.Errorf("candidate cleanup proof binding mismatch: %s", path)
	}
	if !candidateCleanupProofStoresOnlyRelativePaths(proof) {
		return fmt.Errorf("candidate cleanup proof stores absolute or escaping path: %s", path)
	}
	expectedCandidateHash := fileSHA256ReleaseHandoff(action.candidateBackupPathFull)
	if strings.TrimSpace(expectedCandidateHash) == "" || !strings.EqualFold(proof.CandidateHash, expectedCandidateHash) {
		return fmt.Errorf("candidate cleanup proof candidate backup hash mismatch: %s", path)
	}
	if proof.ReviewItem.CandidatePath != proof.CandidatePath || proof.ReviewItem.CandidateHash != proof.CandidateHash || proof.ReviewItem.Kind != action.Kind || proof.ReviewItem.PackTarget != proof.PackTarget {
		return fmt.Errorf("candidate cleanup proof review item binding mismatch: %s", path)
	}
	if !cleanup.CandidateAbsent || !cleanup.IndexEntryAbsent {
		return fmt.Errorf("candidate cleanup proof cleanup state mismatch: %s", path)
	}
	if _, err := os.Lstat(action.candidatePathFull); err == nil || !os.IsNotExist(err) {
		return fmt.Errorf("candidate cleanup proof candidatePath is not absent: %s", action.candidatePathFull)
	}
	transactionPath := filepath.Join(receipt.backupRootFull, "transaction.json")
	committedPath := filepath.Join(receipt.backupRootFull, "committed.json")
	if cleanup.DecisionReceiptPath != releaseHandoffRepoRelative(repo, receipt.pathFull) || !strings.EqualFold(cleanup.DecisionReceiptHash, receipt.receiptHash) || cleanup.TransactionPath != releaseHandoffRepoRelative(repo, transactionPath) || cleanup.CommittedPath != releaseHandoffRepoRelative(repo, committedPath) || cleanup.CandidateBackupPath != releaseHandoffRepoRelative(repo, action.candidateBackupPathFull) || cleanup.TargetBackupPath != releaseHandoffRepoRelative(repo, action.targetBackupPathFull) || cleanup.IndexPath != releaseHandoffRepoRelative(repo, receipt.indexPathFull) {
		return fmt.Errorf("candidate cleanup proof durable path binding mismatch: %s", path)
	}
	if !strings.EqualFold(fileSHA256ReleaseHandoff(receipt.pathFull), cleanup.DecisionReceiptHash) {
		return fmt.Errorf("candidate cleanup proof receipt hash mismatch: %s", path)
	}
	if err := validateReleaseHandoffHashedFile(receipt.backupRootFull, transactionPath, cleanup.TransactionHash, "candidate decision transaction"); err != nil {
		return err
	}
	if err := validateReleaseHandoffHashedFile(receipt.backupRootFull, committedPath, cleanup.CommittedHash, "candidate decision committed marker"); err != nil {
		return err
	}
	if err := validateReleaseHandoffHashedFile(receipt.backupRootFull, action.candidateBackupPathFull, cleanup.CandidateBackupHash, "candidate backup"); err != nil {
		return err
	}
	if !strings.EqualFold(cleanup.CandidateBackupHash, expectedCandidateHash) {
		return fmt.Errorf("candidate cleanup proof candidate backup binding mismatch: %s", path)
	}
	if strings.TrimSpace(action.targetBackupPathFull) != "" {
		if err := validateReleaseHandoffHashedFile(receipt.backupRootFull, action.targetBackupPathFull, cleanup.TargetBackupHash, "target backup"); err != nil {
			return err
		}
	} else if strings.TrimSpace(cleanup.TargetBackupHash) != "" {
		return fmt.Errorf("candidate cleanup proof unexpected target backup hash: %s", path)
	}
	if err := validateCandidateCleanupProofPackTarget(status, action, proof, cleanup, expectedCandidateHash, path); err != nil {
		return err
	}
	indexPresent, err := releaseHandoffCandidateIndexPresent(receipt.indexPathFull)
	if err != nil {
		return err
	}
	if cleanup.IndexPresent != indexPresent {
		return fmt.Errorf("candidate cleanup proof index presence mismatch: %s", path)
	}
	if action.Kind == "managed-doc" {
		contains, err := releaseHandoffCandidateIndexContains(status, receipt.indexPathFull, action.CandidatePath)
		if err != nil {
			return err
		}
		if contains {
			return fmt.Errorf("candidate cleanup proof index still contains candidate: %s", action.CandidatePath)
		}
	}
	if err := validateCandidateCleanupProofEvidenceRefs(repo, receipt.caseRootFull, proof.EvidenceRefs); err != nil {
		return err
	}
	return nil
}

func validateCandidateCleanupProofPackTarget(status ReleaseHandoffPackMemoryCandidateStatus, action ReleaseHandoffPackMemoryCandidateDecisionReceiptAction, proof candidateReviewProofNoteInventory, cleanup *candidateReviewCleanupProofInventory, expectedCandidateHash, proofPath string) error {
	if action.Decision != "accept" {
		if strings.TrimSpace(proof.PackTargetHash) != "" || strings.TrimSpace(cleanup.PackTargetHash) != "" {
			return fmt.Errorf("candidate cleanup proof unexpected packTarget hash: %s", proofPath)
		}
		return nil
	}
	if strings.TrimSpace(action.packTargetFull) == "" {
		return fmt.Errorf("candidate cleanup proof accepted action lacks packTarget: %s", proofPath)
	}
	if !pathWithinReleaseHandoffRoot(filepath.Join(status.repoRootFull, "packs", filepath.FromSlash(status.Pack)), action.packTargetFull) {
		return fmt.Errorf("candidate cleanup proof packTarget leaves pack root: %s", action.PackTarget)
	}
	packTargetHash := fileSHA256ReleaseHandoff(action.packTargetFull)
	if strings.TrimSpace(packTargetHash) == "" || !strings.EqualFold(packTargetHash, expectedCandidateHash) || !strings.EqualFold(proof.PackTargetHash, packTargetHash) || !strings.EqualFold(cleanup.PackTargetHash, packTargetHash) {
		return fmt.Errorf("candidate cleanup proof packTarget hash mismatch: %s", proofPath)
	}
	return nil
}

func releaseHandoffCandidateIndexPresent(indexPath string) (bool, error) {
	info, err := os.Lstat(indexPath)
	if os.IsNotExist(err) {
		return false, nil
	}
	if err != nil {
		return false, err
	}
	if info.Mode()&os.ModeSymlink != 0 || !info.Mode().IsRegular() {
		return false, fmt.Errorf("candidate cleanup proof indexPath must be a regular file when present: %s", indexPath)
	}
	return true, nil
}

func releaseHandoffCandidateIndexContains(status ReleaseHandoffPackMemoryCandidateStatus, indexPath, candidatePath string) (bool, error) {
	candidateRoot := filepath.Join(status.repoRootFull, filepath.FromSlash(status.CandidateRoot))
	entries, _, err := candidateIndexEntries(indexPath, status.repoRootFull, candidateRoot, status.CandidateRoot)
	if err != nil {
		return false, err
	}
	for _, entry := range entries {
		if entry.Candidate == candidatePath {
			return true, nil
		}
	}
	return false, nil
}

func validateCandidateCleanupProofEvidenceRefs(repo, caseRoot string, evidenceRefs []candidateDecisionEvidenceInventory) error {
	if len(evidenceRefs) == 0 {
		return fmt.Errorf("candidate cleanup proof requires non-empty evidenceRefs")
	}
	seen := map[string]bool{}
	for _, evidence := range evidenceRefs {
		if strings.TrimSpace(evidence.Path) == "" || strings.TrimSpace(evidence.SHA256) == "" {
			return fmt.Errorf("candidate cleanup proof evidence path or hash is empty")
		}
		if !releaseHandoffStoredRelativePath(evidence.Path) {
			return fmt.Errorf("candidate cleanup proof evidence stores absolute or escaping path: %s", evidence.Path)
		}
		key := filepath.ToSlash(filepath.Clean(filepath.FromSlash(evidence.Path)))
		if seen[key] {
			return fmt.Errorf("candidate cleanup proof evidence duplicated: %s", evidence.Path)
		}
		seen[key] = true
		if !candidateCleanupProofEvidenceHashMatches(repo, caseRoot, evidence) {
			return fmt.Errorf("candidate cleanup proof evidence hash mismatch: %s", evidence.Path)
		}
	}
	return nil
}

func candidateCleanupProofEvidenceHashMatches(repo, caseRoot string, evidence candidateDecisionEvidenceInventory) bool {
	for _, root := range []string{caseRoot, repo} {
		if strings.TrimSpace(root) == "" {
			continue
		}
		full := filepath.Join(root, filepath.FromSlash(evidence.Path))
		if !pathWithinReleaseHandoffRoot(root, full) {
			continue
		}
		if err := rejectReleaseHandoffSymlinkPath(root, full, false); err != nil {
			continue
		}
		info, err := os.Lstat(full)
		if err != nil || info.Mode()&os.ModeSymlink != 0 || !info.Mode().IsRegular() || info.Size() == 0 || info.Size() > 1024*1024 {
			continue
		}
		if strings.EqualFold(fileSHA256ReleaseHandoff(full), evidence.SHA256) {
			return true
		}
	}
	return false
}

func candidateCleanupProofStoresOnlyRelativePaths(proof candidateReviewProofNoteInventory) bool {
	paths := []string{proof.CandidatePath, proof.PackTarget, proof.ReviewItem.CandidatePath, proof.ReviewItem.PackTarget}
	if cleanup := proof.Cleanup; cleanup != nil {
		paths = append(paths, cleanup.DecisionReceiptPath, cleanup.TransactionPath, cleanup.CommittedPath, cleanup.CandidateBackupPath, cleanup.TargetBackupPath, cleanup.IndexPath)
	}
	for _, evidence := range proof.EvidenceRefs {
		paths = append(paths, evidence.Path)
	}
	for _, path := range paths {
		if !releaseHandoffStoredRelativePath(path) {
			return false
		}
	}
	return true
}

func releaseHandoffStoredRelativePath(path string) bool {
	value := strings.TrimSpace(path)
	if value == "" {
		return true
	}
	fromSlash := filepath.FromSlash(value)
	if filepath.IsAbs(value) || filepath.IsAbs(fromSlash) || filepath.VolumeName(fromSlash) != "" {
		return false
	}
	clean := filepath.Clean(fromSlash)
	return clean != ".." && !strings.HasPrefix(clean, ".."+string(filepath.Separator))
}

func packMemoryCandidateReviewProofSummary(status ReleaseHandoffPackMemoryCandidateStatus) ReleaseHandoffPackMemoryCandidateReviewProofSummary {
	summary := ReleaseHandoffPackMemoryCandidateReviewProofSummary{
		Total:     len(status.ReviewArtifacts),
		ProofRoot: status.ProofRoot,
		Complete:  len(status.ReviewArtifacts) == 0,
	}
	missingByStage := map[string]ReleaseHandoffPackMemoryCandidateReviewArtifact{}
	for _, artifact := range status.ReviewArtifacts {
		if artifact.ProofPresent {
			summary.Present++
			if summary.ProofRoot == "" && artifact.ProofPath != "" {
				summary.ProofRoot = filepath.ToSlash(filepath.Dir(artifact.ProofPath))
			}
		} else {
			summary.Missing++
			if summary.ProofRoot == "" && len(artifact.ExpectedProofs) > 0 {
				summary.ProofRoot = filepath.ToSlash(filepath.Dir(artifact.ExpectedProofs[0]))
			}
		}
		switch stage := packMemoryCandidateProofArtifactStage(artifact.Name); stage {
		case "decision-proof-required":
			if artifact.ProofPresent {
				summary.DecisionPresent++
			} else {
				summary.DecisionMissing++
				if _, ok := missingByStage[stage]; !ok {
					missingByStage[stage] = artifact
				}
			}
		case "cleanup-proof-required":
			if artifact.ProofPresent {
				summary.CleanupPresent++
			} else {
				summary.CleanupMissing++
				if _, ok := missingByStage[stage]; !ok {
					missingByStage[stage] = artifact
				}
			}
		case "reconsume-proof-required":
			if artifact.ProofPresent {
				summary.ReconsumePresent++
			} else {
				summary.ReconsumeMissing++
				if _, ok := missingByStage[stage]; !ok {
					missingByStage[stage] = artifact
				}
			}
		}
	}
	if summary.Total > 0 {
		summary.ProofProgress = fmt.Sprintf("%d/%d", summary.Present, summary.Total)
		summary.CurrentStage = packMemoryCandidateProofStage(summary)
		if artifact, ok := missingByStage[summary.CurrentStage]; ok {
			summary.NextMissingProofType = artifact.Name
			summary.NextMissingProofPath = packMemoryCandidateNextExpectedProof(artifact)
			summary.NextMissingCandidatePath = artifact.CandidatePath
			summary.NextMissingPackTarget = artifact.PackTarget
			nextMissingProof := packMemoryCandidateNextMissingProof(summary.CurrentStage, summary.NextMissingProofPath, artifact)
			summary.NextMissingProof = &nextMissingProof
		}
		summary.Complete = summary.Missing == 0
		if summary.Missing > 0 {
			summary.NextAction = "record missing pack-memory review proof: " + summary.NextMissingProofType + " at " + summary.NextMissingProofPath + " for " + summary.NextMissingCandidatePath
		} else {
			summary.NextAction = "all expected pack-memory review proof files are present; review candidate cleanup/reconsume outcomes before release handoff"
		}
		summary.Boundary = []string{
			"proofSummary is read-only; release/status detects and validates bounded proof files but does not create or reconsume them",
			"proof files must stay repo-local review evidence and must not contain case-specific artifacts, traces, dumps, captures, payloads, flags, or customer data",
		}
	}
	return summary
}

func packMemoryCandidateNextExpectedProof(artifact ReleaseHandoffPackMemoryCandidateReviewArtifact) string {
	if strings.TrimSpace(artifact.ProofPath) != "" {
		return artifact.ProofPath
	}
	if len(artifact.ExpectedProofs) > 0 {
		return artifact.ExpectedProofs[0]
	}
	return ""
}

func packMemoryCandidateNextMissingProof(stage, proofPath string, artifact ReleaseHandoffPackMemoryCandidateReviewArtifact) ReleaseHandoffPackMemoryCandidateReviewNextMissingProof {
	proof := ReleaseHandoffPackMemoryCandidateReviewNextMissingProof{
		Stage:                 stage,
		ProofType:             artifact.Name,
		Path:                  proofPath,
		CandidatePath:         artifact.CandidatePath,
		PackTarget:            artifact.PackTarget,
		SourceCaseRoot:        artifact.SourceCaseRoot,
		PacketPath:            artifact.PacketPath,
		CandidateDecisionPath: artifact.CandidateDecisionPath,
		When:                  artifact.When,
		Action:                artifact.Action,
		Format:                artifact.Format,
		Evidence:              append([]string{}, artifact.Evidence...),
		Boundary:              append([]string{}, artifact.Boundary...),
	}
	if artifact.Name == "candidate-decision-note" && strings.TrimSpace(artifact.CandidatePath) != "" {
		proof.RequiresPacket = true
		proof.RequiresExplicitReview = true
		proof.DraftCommand = "/rekit promote -PacketPath <packet.json> -DraftReviewProof -ProofPath " + quoteReleaseHandoffCommandArg(proofPath) + " -ProofType candidate-decision-note -CandidatePath " + quoteReleaseHandoffCommandArg(artifact.CandidatePath) + " -ProofDecision <accept|reject|superseded> -Reason <reviewed-reason> -Actor <actor> -EvidenceRefs <review-evidence-ref> -WhatIf -Format json"
		proof.DraftApplyTemplate = "/rekit promote -PacketPath <packet.json> -DraftReviewProof -ProofPath " + quoteReleaseHandoffCommandArg(proofPath) + " -ProofType candidate-decision-note -CandidatePath " + quoteReleaseHandoffCommandArg(artifact.CandidatePath) + " -ProofDecision <accept|reject|superseded> -Reason <reviewed-reason> -Actor <actor> -EvidenceRefs <review-evidence-ref> -ExpectedProofSha256 <proofSha256-from-WhatIf> -Apply -Format json"
	}
	if artifact.Name == "candidate-cleanup-proof" && strings.TrimSpace(artifact.CandidatePath) != "" {
		proof.RequiresPacket = true
		proof.RequiresCandidateDecision = true
		proof.DraftCommand = "/rekit promote -PacketPath <packet.json> -CandidateDecisionPath <candidate-decisions.json> -DraftReviewProof -ProofPath " + quoteReleaseHandoffCommandArg(proofPath) + " -ProofType candidate-cleanup-proof -CandidatePath " + quoteReleaseHandoffCommandArg(artifact.CandidatePath) + " -Reason <cleanup-proof-reason> -Actor <actor> -EvidenceRefs <cleanup-evidence-ref> -WhatIf -Format json"
		proof.DraftApplyTemplate = "/rekit promote -PacketPath <packet.json> -CandidateDecisionPath <candidate-decisions.json> -DraftReviewProof -ProofPath " + quoteReleaseHandoffCommandArg(proofPath) + " -ProofType candidate-cleanup-proof -CandidatePath " + quoteReleaseHandoffCommandArg(artifact.CandidatePath) + " -Reason <cleanup-proof-reason> -Actor <actor> -EvidenceRefs <cleanup-evidence-ref> -ExpectedProofSha256 <proofSha256-from-WhatIf> -Apply -Format json"
	}
	if packMemoryCandidateLifecycleProofType(artifact.Name) && strings.TrimSpace(artifact.CandidatePath) != "" {
		proof.RequiresPacket = true
		proof.DraftCommand = "/rekit promote -PacketPath <packet.json> -DraftReviewProof -ProofPath " + quoteReleaseHandoffCommandArg(proofPath) + " -ProofType " + quoteReleaseHandoffCommandArg(artifact.Name) + " -CandidatePath " + quoteReleaseHandoffCommandArg(artifact.CandidatePath) + " -Reason <lifecycle-proof-reason> -Actor <actor> -EvidenceRefs <repo-local-lifecycle-evidence-ref> -WhatIf -Format json"
		proof.DraftApplyTemplate = "/rekit promote -PacketPath <packet.json> -DraftReviewProof -ProofPath " + quoteReleaseHandoffCommandArg(proofPath) + " -ProofType " + quoteReleaseHandoffCommandArg(artifact.Name) + " -CandidatePath " + quoteReleaseHandoffCommandArg(artifact.CandidatePath) + " -Reason <lifecycle-proof-reason> -Actor <actor> -EvidenceRefs <repo-local-lifecycle-evidence-ref> -ExpectedProofSha256 <proofSha256-from-WhatIf> -Apply -Format json"
	}
	proof.DraftCommand = releaseHandoffPackMemoryProofCommandWithBindings(proof.DraftCommand, proof.PacketPath, proof.CandidateDecisionPath, proof.EvidenceRefs, proof.SourceCaseRoot)
	proof.DraftApplyTemplate = releaseHandoffPackMemoryProofCommandWithBindings(proof.DraftApplyTemplate, proof.PacketPath, proof.CandidateDecisionPath, proof.EvidenceRefs, proof.SourceCaseRoot)
	return proof
}

func packMemoryCandidateLifecycleProofType(proofType string) bool {
	switch strings.TrimSpace(proofType) {
	case "pack-doctor-output", "fresh-case-reconsume-proof", "attached-case-reconsume-proof":
		return true
	default:
		return false
	}
}

func packMemoryCandidateProofArtifactStage(name string) string {
	switch strings.TrimSpace(name) {
	case "candidate-decision-note", "blocked-review-note":
		return "decision-proof-required"
	case "candidate-cleanup-proof":
		return "cleanup-proof-required"
	case "pack-doctor-output", "fresh-case-reconsume-proof", "attached-case-reconsume-proof":
		return "reconsume-proof-required"
	default:
		return ""
	}
}

func packMemoryCandidateProofStage(summary ReleaseHandoffPackMemoryCandidateReviewProofSummary) string {
	switch {
	case summary.DecisionMissing > 0:
		return "decision-proof-required"
	case summary.CleanupMissing > 0:
		return "cleanup-proof-required"
	case summary.ReconsumeMissing > 0:
		return "reconsume-proof-required"
	case summary.Total > 0:
		return "proof-complete-review-cleanup"
	default:
		return "no-proof-required"
	}
}

type candidateDecisionActionInventory struct {
	CandidatePath       string   `json:"candidatePath"`
	Kind                string   `json:"kind"`
	Decision            string   `json:"decision"`
	PackTarget          string   `json:"packTarget,omitempty"`
	Action              string   `json:"action"`
	CandidateBackupPath string   `json:"candidateBackupPath,omitempty"`
	TargetBackupPath    string   `json:"targetBackupPath,omitempty"`
	EvidenceRefs        []string `json:"evidenceRefs"`
}

type candidateDecisionReceiptInventory struct {
	SchemaVersion                int                                `json:"schemaVersion"`
	Kind                         string                             `json:"kind"`
	Pack                         string                             `json:"pack"`
	RepoRoot                     string                             `json:"repoRoot"`
	CaseRoot                     string                             `json:"caseRoot"`
	PacketPath                   string                             `json:"packetPath"`
	DecisionPath                 string                             `json:"decisionPath"`
	PacketHash                   string                             `json:"packetHash"`
	DecisionHash                 string                             `json:"decisionHash"`
	BackupRoot                   string                             `json:"backupRoot"`
	IndexPath                    string                             `json:"indexPath"`
	Accepted                     int                                `json:"accepted"`
	Rejected                     int                                `json:"rejected"`
	Superseded                   int                                `json:"superseded"`
	Actions                      []candidateDecisionActionInventory `json:"actions"`
	DecisionEvidence             []string                           `json:"decisionEvidence"`
	ReceiptPath                  string                             `json:"receiptPath"`
	VerificationProofPath        string                             `json:"verificationProofPath,omitempty"`
	VerificationPending          bool                               `json:"verificationPending"`
	VerificationWorkspaceRoot    string                             `json:"verificationWorkspaceRoot,omitempty"`
	VerificationProvisionCommand string                             `json:"verificationProvisionCommand,omitempty"`
	VerificationCommand          string                             `json:"verificationCommand,omitempty"`
	Boundary                     []string                           `json:"boundary"`
}

type candidateDecisionEvidenceInventory struct {
	Path   string `json:"path"`
	SHA256 string `json:"sha256"`
}

type candidateDecisionItemInventory struct {
	CandidatePath  string                               `json:"candidatePath"`
	Decision       string                               `json:"decision"`
	CandidateHash  string                               `json:"candidateHash"`
	PackTargetHash string                               `json:"packTargetHash,omitempty"`
	Reason         string                               `json:"reason"`
	Actor          string                               `json:"actor"`
	EvidenceRefs   []candidateDecisionEvidenceInventory `json:"evidenceRefs"`
}

type candidateDecisionFileInventory struct {
	SchemaVersion int                              `json:"schemaVersion"`
	Kind          string                           `json:"kind"`
	PacketHash    string                           `json:"packetHash"`
	Decisions     []candidateDecisionItemInventory `json:"decisions"`
}

type candidateReviewCleanupProofInventory struct {
	DecisionReceiptPath string `json:"decisionReceiptPath"`
	DecisionReceiptHash string `json:"decisionReceiptHash"`
	TransactionPath     string `json:"transactionPath"`
	TransactionHash     string `json:"transactionHash"`
	CommittedPath       string `json:"committedPath"`
	CommittedHash       string `json:"committedHash"`
	CandidateBackupPath string `json:"candidateBackupPath"`
	CandidateBackupHash string `json:"candidateBackupHash"`
	TargetBackupPath    string `json:"targetBackupPath,omitempty"`
	TargetBackupHash    string `json:"targetBackupHash,omitempty"`
	IndexPath           string `json:"indexPath,omitempty"`
	IndexPresent        bool   `json:"indexPresent"`
	IndexEntryAbsent    bool   `json:"indexEntryAbsent"`
	CandidateAbsent     bool   `json:"candidateAbsent"`
	PackTargetHash      string `json:"packTargetHash,omitempty"`
}

type candidateReviewProofReviewItemInventory struct {
	CandidatePath string `json:"candidatePath"`
	CandidateHash string `json:"candidateHash"`
	PackTarget    string `json:"packTarget,omitempty"`
	Kind          string `json:"kind"`
}

type candidateReviewProofNoteInventory struct {
	SchemaVersion  int                                     `json:"schemaVersion"`
	Kind           string                                  `json:"kind"`
	Pack           string                                  `json:"pack"`
	PacketHash     string                                  `json:"packetHash"`
	DecisionHash   string                                  `json:"decisionHash,omitempty"`
	ProofType      string                                  `json:"proofType"`
	CandidatePath  string                                  `json:"candidatePath"`
	CandidateHash  string                                  `json:"candidateHash"`
	PackTarget     string                                  `json:"packTarget,omitempty"`
	PackTargetHash string                                  `json:"packTargetHash,omitempty"`
	Decision       string                                  `json:"decision"`
	Reason         string                                  `json:"reason"`
	Actor          string                                  `json:"actor"`
	EvidenceRefs   []candidateDecisionEvidenceInventory    `json:"evidenceRefs"`
	ReviewItem     candidateReviewProofReviewItemInventory `json:"reviewItem"`
	Cleanup        *candidateReviewCleanupProofInventory   `json:"cleanup,omitempty"`
	Boundary       []string                                `json:"boundary"`
}

type candidateLifecycleProofInventory struct {
	SchemaVersion int                                     `json:"schemaVersion"`
	Kind          string                                  `json:"kind"`
	Pack          string                                  `json:"pack"`
	ProofType     string                                  `json:"proofType"`
	CandidatePath string                                  `json:"candidatePath"`
	PackTarget    string                                  `json:"packTarget,omitempty"`
	Reason        string                                  `json:"reason"`
	Actor         string                                  `json:"actor"`
	EvidenceRefs  []candidateDecisionEvidenceInventory    `json:"evidenceRefs"`
	ReviewItem    candidateLifecycleReviewItemInventory   `json:"reviewItem"`
	Checks        []candidateLifecycleProofCheckInventory `json:"checks"`
	Boundary      []string                                `json:"boundary"`
}

type candidateLifecycleReviewItemInventory struct {
	CandidatePath string `json:"candidatePath"`
	PackTarget    string `json:"packTarget,omitempty"`
	ProofType     string `json:"proofType"`
	Stage         string `json:"stage"`
}

type candidateLifecycleProofCheckInventory struct {
	Name    string `json:"name"`
	Status  string `json:"status"`
	Summary string `json:"summary"`
}

type candidateDecisionResultInventory struct {
	SchemaVersion    int                                `json:"schemaVersion"`
	Command          string                             `json:"command"`
	Mode             string                             `json:"mode"`
	CaseRoot         string                             `json:"caseRoot"`
	RepoRoot         string                             `json:"repoRoot"`
	Pack             string                             `json:"pack"`
	PacketPath       string                             `json:"packetPath"`
	DecisionPath     string                             `json:"decisionPath"`
	PacketHash       string                             `json:"packetHash"`
	IsMutation       bool                               `json:"isMutation"`
	Applied          bool                               `json:"applied"`
	RolledBack       bool                               `json:"rolledBack,omitempty"`
	RecoveryRequired bool                               `json:"recoveryRequired,omitempty"`
	FailedAction     string                             `json:"failedAction,omitempty"`
	Accepted         int                                `json:"accepted"`
	Rejected         int                                `json:"rejected"`
	Superseded       int                                `json:"superseded"`
	BackupRoot       string                             `json:"backupRoot,omitempty"`
	IndexPath        string                             `json:"indexPath,omitempty"`
	ReceiptPath      string                             `json:"receiptPath,omitempty"`
	Receipt          *candidateDecisionReceiptInventory `json:"receipt,omitempty"`
	Actions          []candidateDecisionActionInventory `json:"actions"`
	RecoveryActions  []string                           `json:"recoveryActions,omitempty"`
	NextSteps        []string                           `json:"nextSteps"`
	Boundary         []string                           `json:"boundary"`
}

type candidateDecisionTransactionInventory struct {
	SchemaVersion   int                                `json:"schemaVersion"`
	Kind            string                             `json:"kind"`
	PacketHash      string                             `json:"packetHash"`
	DecisionHash    string                             `json:"decisionHash"`
	IndexExisted    bool                               `json:"indexExisted"`
	IndexBackupPath string                             `json:"indexBackupPath,omitempty"`
	Result          candidateDecisionResultInventory   `json:"result"`
	Actions         []candidateDecisionActionInventory `json:"actions"`
}

type candidateVerificationProvisionArtifactInventory struct {
	SchemaVersion              int                                          `json:"schemaVersion"`
	Kind                       string                                       `json:"kind"`
	RepoRoot                   string                                       `json:"repoRoot"`
	SourceCaseRoot             string                                       `json:"sourceCaseRoot"`
	Pack                       string                                       `json:"pack"`
	PacketPath                 string                                       `json:"packetPath"`
	PacketSHA256               string                                       `json:"packetSha256"`
	DecisionPath               string                                       `json:"decisionPath"`
	DecisionSHA256             string                                       `json:"decisionSha256"`
	DecisionReceiptPath        string                                       `json:"decisionReceiptPath"`
	DecisionReceiptSHA256      string                                       `json:"decisionReceiptSha256"`
	ProvisionID                string                                       `json:"provisionId"`
	ProvisionSHA256            string                                       `json:"provisionSha256"`
	WorkspaceRoot              string                                       `json:"workspaceRoot"`
	IntentPath                 string                                       `json:"intentPath"`
	FreshCaseRoot              string                                       `json:"freshCaseRoot"`
	AttachedCaseRoot           string                                       `json:"attachedCaseRoot"`
	VerificationPreviewCommand string                                       `json:"verificationPreviewCommand"`
	Cases                      []promote.CandidateVerificationProvisionCase `json:"cases"`
	Boundary                   []string                                     `json:"boundary"`
}

type candidateDecisionVerificationInventory struct {
	SchemaVersion            int                                `json:"schemaVersion"`
	Kind                     string                             `json:"kind"`
	Pack                     string                             `json:"pack"`
	CaseRoot                 string                             `json:"caseRoot"`
	FreshCaseRoot            string                             `json:"freshCaseRoot"`
	AttachedCaseRoot         string                             `json:"attachedCaseRoot"`
	PacketHash               string                             `json:"packetHash"`
	DecisionHash             string                             `json:"decisionHash"`
	ReceiptHash              string                             `json:"receiptHash"`
	ReceiptPath              string                             `json:"receiptPath"`
	VerificationProofPath    string                             `json:"verificationProofPath"`
	ProvisionIntentPath      string                             `json:"provisionIntentPath,omitempty"`
	ProvisionIntentSHA256    string                             `json:"provisionIntentSha256,omitempty"`
	ProvisionReceiptPath     string                             `json:"provisionReceiptPath,omitempty"`
	ProvisionReceiptSHA256   string                             `json:"provisionReceiptSha256,omitempty"`
	RetirementPreviewCommand string                             `json:"retirementPreviewCommand,omitempty"`
	IsMutation               bool                               `json:"isMutation"`
	Applied                  bool                               `json:"applied"`
	Ready                    bool                               `json:"ready"`
	PackDoctorRows           int                                `json:"packDoctorRows"`
	FreshDoctorRows          int                                `json:"freshDoctorRows"`
	AttachedDoctorRows       int                                `json:"attachedDoctorRows"`
	VerifiedActions          []candidateDecisionActionInventory `json:"verifiedActions"`
	NextSteps                []string                           `json:"nextSteps"`
	Boundary                 []string                           `json:"boundary"`
}

type candidateVerificationRetirementRootInventory struct {
	Role     string   `json:"role"`
	CaseRoot string   `json:"caseRoot"`
	Deletes  []string `json:"deletes"`
}

type candidateVerificationRetirementArtifactInventory struct {
	SchemaVersion              int                                            `json:"schemaVersion"`
	Kind                       string                                         `json:"kind"`
	RepoRoot                   string                                         `json:"repoRoot"`
	SourceCaseRoot             string                                         `json:"sourceCaseRoot"`
	Pack                       string                                         `json:"pack"`
	PacketPath                 string                                         `json:"packetPath"`
	PacketSHA256               string                                         `json:"packetSha256"`
	DecisionPath               string                                         `json:"decisionPath"`
	DecisionSHA256             string                                         `json:"decisionSha256"`
	DecisionReceiptPath        string                                         `json:"decisionReceiptPath"`
	DecisionReceiptSHA256      string                                         `json:"decisionReceiptSha256"`
	VerificationProofPath      string                                         `json:"verificationProofPath"`
	VerificationProofSHA256    string                                         `json:"verificationProofSha256"`
	ProvisionIntentPath        string                                         `json:"provisionIntentPath"`
	ProvisionIntentSHA256      string                                         `json:"provisionIntentSha256"`
	ProvisionReceiptPath       string                                         `json:"provisionReceiptPath"`
	ProvisionReceiptSHA256     string                                         `json:"provisionReceiptSha256"`
	WorkspaceRoot              string                                         `json:"workspaceRoot"`
	RetirementIntentPath       string                                         `json:"retirementIntentPath"`
	RetirementReceiptPath      string                                         `json:"retirementReceiptPath"`
	RetirementSHA256           string                                         `json:"retirementSha256"`
	Roots                      []candidateVerificationRetirementRootInventory `json:"roots"`
	ProvisionArtifactsToDelete []string                                       `json:"provisionArtifactsToDelete"`
	EmptyAncestorsToRemove     []string                                       `json:"emptyAncestorsToRemove"`
	Boundary                   []string                                       `json:"boundary"`
	RetirementPlans            []syncpkg.ExclusiveInitRetirementPlan          `json:"retirementPlans"`
}

func packMemoryCandidateDecisionReceipts(repo, proofRoot, proofRootRel string) ([]ReleaseHandoffPackMemoryCandidateDecisionReceipt, error) {
	packID := filepath.Base(filepath.Dir(filepath.Dir(proofRoot)))
	candidateRoot := filepath.Dir(proofRoot)
	if err := rejectReleaseHandoffSymlinkPath(repo, proofRoot, true); err != nil {
		return nil, err
	}
	entries, err := os.ReadDir(proofRoot)
	if os.IsNotExist(err) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	receipts := []ReleaseHandoffPackMemoryCandidateDecisionReceipt{}
	for _, entry := range entries {
		if entry.IsDir() || !strings.HasSuffix(entry.Name(), ".candidate-decision-receipt.json") {
			continue
		}
		path := filepath.Join(proofRoot, entry.Name())
		info, err := entry.Info()
		if err != nil {
			return nil, err
		}
		if info.Mode()&os.ModeSymlink != 0 || !info.Mode().IsRegular() || info.Size() == 0 || info.Size() > 1024*1024 {
			return nil, fmt.Errorf("candidate decision receipt must be a non-empty regular file: %s", path)
		}
		data, err := os.ReadFile(path)
		if err != nil {
			return nil, err
		}
		var raw candidateDecisionReceiptInventory
		if err := decodeReleaseHandoffStrictJSON(data, &raw); err != nil {
			return nil, fmt.Errorf("decode candidate decision receipt %s: %w", path, err)
		}
		if raw.SchemaVersion != 1 || raw.Kind != "pack-memory-candidate-decision-receipt" || raw.Pack != packID || !sameReleaseHandoffPath(raw.RepoRoot, repo) || strings.TrimSpace(raw.CaseRoot) == "" || strings.TrimSpace(raw.PacketHash) == "" || strings.TrimSpace(raw.DecisionHash) == "" || strings.TrimSpace(raw.PacketPath) == "" || strings.TrimSpace(raw.DecisionPath) == "" || !sameReleaseHandoffPath(raw.ReceiptPath, path) || raw.Accepted < 0 || raw.Rejected < 0 || raw.Superseded < 0 || raw.Accepted+raw.Rejected+raw.Superseded != len(raw.Actions) || (raw.Accepted > 0 && (!raw.VerificationPending || strings.TrimSpace(raw.VerificationWorkspaceRoot) == "" || strings.TrimSpace(raw.VerificationProvisionCommand) == "" || strings.TrimSpace(raw.VerificationCommand) == "" || strings.TrimSpace(raw.VerificationProofPath) == "")) {
			return nil, fmt.Errorf("candidate decision receipt binding mismatch: %s", path)
		}
		if !sameReleaseHandoffPath(raw.IndexPath, filepath.Join(candidateRoot, "index.json")) {
			return nil, fmt.Errorf("candidate decision receipt index binding mismatch: %s", path)
		}
		if raw.Accepted > 0 {
			provisionID := shortReleaseHandoffHash(raw.PacketHash + raw.DecisionHash)
			expectedWorkspace := filepath.Join(raw.CaseRoot, ".rekit", "verifications", "candidate-decisions", provisionID)
			if !sameReleaseHandoffPath(raw.VerificationWorkspaceRoot, expectedWorkspace) || !strings.Contains(raw.VerificationProvisionCommand, "-ProvisionCandidateVerificationCases") || !strings.Contains(raw.VerificationProvisionCommand, raw.PacketPath) || !strings.Contains(raw.VerificationProvisionCommand, raw.DecisionPath) {
				return nil, fmt.Errorf("candidate decision receipt provisioning binding mismatch: %s", path)
			}
		}
		if !pathWithinReleaseHandoffRoot(candidateRoot, raw.BackupRoot) {
			return nil, fmt.Errorf("candidate decision receipt backup leaves candidate root: %s", raw.BackupRoot)
		}
		if err := validateCandidateDecisionReceiptActions(candidateRoot, raw); err != nil {
			return nil, fmt.Errorf("candidate decision receipt action binding mismatch %s: %w", path, err)
		}
		if err := rejectReleaseHandoffSymlinkPath(repo, raw.BackupRoot, false); err != nil {
			return nil, err
		}
		markerPath := filepath.Join(raw.BackupRoot, "committed.json")
		if err := rejectReleaseHandoffSymlinkPath(raw.BackupRoot, markerPath, false); err != nil {
			return nil, err
		}
		markerInfo, markerErr := os.Lstat(markerPath)
		if markerErr != nil || markerInfo.Mode()&os.ModeSymlink != 0 || !markerInfo.Mode().IsRegular() || markerInfo.Size() == 0 || markerInfo.Size() > 1024*1024 {
			return nil, fmt.Errorf("candidate decision receipt transaction is not committed: %s", raw.BackupRoot)
		}
		if !raw.VerificationPending {
			if err := validateCandidateDecisionReceiptAttestation(repo, path, data, raw); err != nil {
				return nil, fmt.Errorf("candidate decision receipt attestation mismatch %s: %w", path, err)
			}
		}
		proofComplete := false
		proofRel := ""
		proof := candidateDecisionVerificationInventory{}
		if raw.VerificationProofPath != "" {
			if !pathWithinReleaseHandoffRoot(proofRoot, raw.VerificationProofPath) {
				return nil, fmt.Errorf("candidate verification proof leaves proof root: %s", raw.VerificationProofPath)
			}
			if err := rejectReleaseHandoffSymlinkPath(repo, raw.VerificationProofPath, true); err != nil {
				return nil, err
			}
			proofInfo, proofErr := os.Lstat(raw.VerificationProofPath)
			if proofErr == nil {
				if proofInfo.Mode()&os.ModeSymlink != 0 || !proofInfo.Mode().IsRegular() || proofInfo.Size() == 0 || proofInfo.Size() > 1024*1024 {
					return nil, fmt.Errorf("candidate verification proof must be a non-empty regular file: %s", raw.VerificationProofPath)
				}
				proofData, err := os.ReadFile(raw.VerificationProofPath)
				if err != nil {
					return nil, err
				}
				if err := decodeReleaseHandoffStrictJSON(proofData, &proof); err != nil {
					return nil, fmt.Errorf("decode candidate verification proof %s: %w", raw.VerificationProofPath, err)
				}
				if proof.SchemaVersion != 1 || proof.Kind != "pack-memory-candidate-decision-verification" || proof.Pack != raw.Pack || !strings.EqualFold(proof.PacketHash, raw.PacketHash) || !strings.EqualFold(proof.DecisionHash, raw.DecisionHash) || !strings.EqualFold(proof.ReceiptHash, sha256ReleaseHandoff(data)) || !sameReleaseHandoffPath(proof.CaseRoot, raw.CaseRoot) || strings.TrimSpace(proof.FreshCaseRoot) == "" || strings.TrimSpace(proof.AttachedCaseRoot) == "" || sameReleaseHandoffPath(proof.FreshCaseRoot, proof.CaseRoot) || sameReleaseHandoffPath(proof.AttachedCaseRoot, proof.CaseRoot) || sameReleaseHandoffPath(proof.FreshCaseRoot, proof.AttachedCaseRoot) || !sameReleaseHandoffPath(proof.ReceiptPath, path) || !sameReleaseHandoffPath(proof.VerificationProofPath, raw.VerificationProofPath) || !proof.IsMutation || !proof.Applied || !proof.Ready || proof.PackDoctorRows <= 0 || proof.FreshDoctorRows <= 0 || proof.AttachedDoctorRows <= 0 || !candidateDecisionActionsEqual(proof.VerifiedActions, raw.Actions) {
					return nil, fmt.Errorf("candidate verification proof binding mismatch: %s", raw.VerificationProofPath)
				}
				if raw.Accepted > 0 {
					expectedIntentPath := filepath.Join(raw.VerificationWorkspaceRoot, "provision.intent.json")
					expectedReceiptPath := filepath.Join(raw.VerificationWorkspaceRoot, "provision.receipt.json")
					if !sameReleaseHandoffPath(proof.ProvisionIntentPath, expectedIntentPath) || !sameReleaseHandoffPath(proof.ProvisionReceiptPath, expectedReceiptPath) || strings.TrimSpace(proof.ProvisionIntentSHA256) == "" || strings.TrimSpace(proof.ProvisionReceiptSHA256) == "" || strings.TrimSpace(proof.RetirementPreviewCommand) == "" || !strings.Contains(proof.RetirementPreviewCommand, "-RetireCandidateVerificationWorkspace") || !strings.Contains(proof.RetirementPreviewCommand, "-WhatIf") || !strings.Contains(proof.RetirementPreviewCommand, raw.PacketPath) || !strings.Contains(proof.RetirementPreviewCommand, raw.DecisionPath) {
						return nil, fmt.Errorf("candidate verification proof retirement binding mismatch: %s", raw.VerificationProofPath)
					}
				}
				proofComplete = true
				proofRel = releaseHandoffRepoRelative(repo, raw.VerificationProofPath)
			} else if !os.IsNotExist(proofErr) {
				return nil, proofErr
			}
		}
		handoffReceipt := ReleaseHandoffPackMemoryCandidateDecisionReceipt{
			Path:                         filepath.ToSlash(filepath.Join(proofRootRel, entry.Name())),
			SourceCaseRoot:               raw.CaseRoot,
			Accepted:                     raw.Accepted,
			Rejected:                     raw.Rejected,
			Superseded:                   raw.Superseded,
			PacketPath:                   releaseHandoffRepoRelative(repo, raw.PacketPath),
			DecisionPath:                 releaseHandoffRepoRelative(repo, raw.DecisionPath),
			Actions:                      releaseHandoffCandidateDecisionReceiptActions(repo, raw.Actions),
			VerificationPending:          raw.VerificationPending,
			VerificationWorkspaceRoot:    raw.VerificationWorkspaceRoot,
			VerificationProvisionCommand: releaseHandoffPromoteCommandWithTarget(raw.VerificationProvisionCommand, raw.CaseRoot),
			VerificationCommand:          releaseHandoffPromoteCommandWithTarget(raw.VerificationCommand, raw.CaseRoot),
			VerificationProofPath:        proofRel,
			VerificationComplete:         proofComplete,
			pathFull:                     path,
			receiptHash:                  sha256ReleaseHandoff(data),
			packetHash:                   raw.PacketHash,
			decisionHash:                 raw.DecisionHash,
			caseRootFull:                 raw.CaseRoot,
			backupRootFull:               raw.BackupRoot,
			indexPathFull:                raw.IndexPath,
		}
		if raw.Accepted > 0 && !proofComplete {
			if err := populateCandidateVerificationProvisionHandoff(repo, path, data, raw, &handoffReceipt); err != nil {
				return nil, err
			}
		}
		if proofComplete && raw.Accepted > 0 {
			if err := populateCandidateVerificationRetirementHandoff(repo, proofRoot, path, data, raw, proof, &handoffReceipt); err != nil {
				return nil, err
			}
		}
		receipts = append(receipts, handoffReceipt)
	}
	sort.Slice(receipts, func(i, j int) bool { return receipts[i].Path < receipts[j].Path })
	return receipts, nil
}

func releaseHandoffCandidateDecisionReceiptActions(repo string, actions []candidateDecisionActionInventory) []ReleaseHandoffPackMemoryCandidateDecisionReceiptAction {
	out := make([]ReleaseHandoffPackMemoryCandidateDecisionReceiptAction, 0, len(actions))
	for _, action := range actions {
		out = append(out, ReleaseHandoffPackMemoryCandidateDecisionReceiptAction{
			CandidatePath:           releaseHandoffRepoRelative(repo, action.CandidatePath),
			Kind:                    action.Kind,
			Decision:                action.Decision,
			PackTarget:              releaseHandoffRepoRelative(repo, action.PackTarget),
			CandidateBackupPath:     releaseHandoffRepoRelative(repo, action.CandidateBackupPath),
			TargetBackupPath:        releaseHandoffRepoRelative(repo, action.TargetBackupPath),
			candidatePathFull:       action.CandidatePath,
			packTargetFull:          action.PackTarget,
			candidateBackupPathFull: action.CandidateBackupPath,
			targetBackupPathFull:    action.TargetBackupPath,
			evidenceRefsFull:        append([]string{}, action.EvidenceRefs...),
		})
	}
	return out
}

func populateCandidateVerificationProvisionHandoff(repo, decisionReceiptPath string, decisionReceiptData []byte, receipt candidateDecisionReceiptInventory, handoff *ReleaseHandoffPackMemoryCandidateDecisionReceipt) error {
	intentPath := filepath.Join(receipt.VerificationWorkspaceRoot, "provision.intent.json")
	receiptPath := filepath.Join(receipt.VerificationWorkspaceRoot, "provision.receipt.json")
	handoff.ProvisionIntentPath = releaseHandoffRepoRelative(repo, intentPath)
	handoff.ProvisionReceiptPath = releaseHandoffRepoRelative(repo, receiptPath)
	handoff.ProvisionStatus = "required"
	handoff.ProvisionNextAction = "run verificationProvisionCommand; inspect the exact fresh/attached case write plan, then run its expected-hash Apply command"
	intent, intentPresent, err := readCandidateVerificationProvisionHandoffArtifact(receipt.CaseRoot, intentPath)
	if err != nil {
		return err
	}
	provisionReceipt, receiptPresent, err := readCandidateVerificationProvisionHandoffArtifact(receipt.CaseRoot, receiptPath)
	if err != nil {
		return err
	}
	if !intentPresent && !receiptPresent {
		handoff.ProvisionApplyCommand = "run verificationProvisionCommand first; WhatIf returns provisionSha256 and the expected-hash Apply command"
		return nil
	}
	if receiptPresent && !intentPresent {
		return fmt.Errorf("candidate verification provisioning receipt lacks current retained intent: %s", receiptPath)
	}
	if err := validateCandidateVerificationProvisionHandoffArtifact(repo, decisionReceiptPath, decisionReceiptData, receipt, intent, "-intent"); err != nil {
		return err
	}
	handoff.ProvisionSHA256 = intent.ProvisionSHA256
	handoff.ProvisionApplyCommand = candidateVerificationProvisionHandoffApplyCommand(receipt, intent.ProvisionSHA256)
	if !receiptPresent {
		handoff.ProvisionStatus = "in-progress"
		handoff.ProvisionInProgress = true
		handoff.ProvisionNextAction = "resume candidate verification provisioning with provisionApplyCommand"
		return nil
	}
	if !candidateVerificationProvisionArtifactMatches(intent, provisionReceipt) {
		return fmt.Errorf("candidate verification provisioning receipt/intent authority binding mismatch: %s", receiptPath)
	}
	if err := validateCandidateVerificationProvisionHandoffArtifact(repo, decisionReceiptPath, decisionReceiptData, receipt, provisionReceipt, "-receipt"); err != nil {
		return err
	}
	handoff.ProvisionStatus = "complete"
	handoff.ProvisionInProgress = false
	handoff.ProvisionComplete = true
	handoff.ProvisionNextAction = "run verificationCommand only after reviewing the completed provision intent and receipt"
	return nil
}

func readCandidateVerificationProvisionHandoffArtifact(root, path string) (candidateVerificationProvisionArtifactInventory, bool, error) {
	if err := rejectReleaseHandoffSymlinkPath(root, path, true); err != nil {
		return candidateVerificationProvisionArtifactInventory{}, false, err
	}
	info, err := os.Lstat(path)
	if os.IsNotExist(err) {
		return candidateVerificationProvisionArtifactInventory{}, false, nil
	}
	if err != nil || info.Mode()&os.ModeSymlink != 0 || !info.Mode().IsRegular() || info.Size() == 0 || info.Size() > 1024*1024 {
		return candidateVerificationProvisionArtifactInventory{}, false, fmt.Errorf("candidate verification provisioning artifact must be a non-empty regular file: %s", path)
	}
	data, err := os.ReadFile(path)
	if err != nil {
		return candidateVerificationProvisionArtifactInventory{}, false, err
	}
	var artifact candidateVerificationProvisionArtifactInventory
	if err := decodeReleaseHandoffStrictJSON(data, &artifact); err != nil {
		return candidateVerificationProvisionArtifactInventory{}, false, fmt.Errorf("decode candidate verification provisioning artifact %s: %w", path, err)
	}
	return artifact, true, nil
}

func validateCandidateVerificationProvisionHandoffArtifact(repo, decisionReceiptPath string, decisionReceiptData []byte, receipt candidateDecisionReceiptInventory, artifact candidateVerificationProvisionArtifactInventory, kindSuffix string) error {
	provisionID := shortReleaseHandoffHash(receipt.PacketHash + receipt.DecisionHash)
	workspace := receipt.VerificationWorkspaceRoot
	intentPath := filepath.Join(workspace, "provision.intent.json")
	expectedKind := "pack-memory-candidate-verification-case-provision" + kindSuffix
	if artifact.SchemaVersion != 1 || artifact.Kind != expectedKind || artifact.Pack != receipt.Pack || !sameReleaseHandoffPath(artifact.RepoRoot, repo) || !sameReleaseHandoffPath(artifact.SourceCaseRoot, receipt.CaseRoot) || !sameReleaseHandoffPath(artifact.PacketPath, receipt.PacketPath) || !strings.EqualFold(artifact.PacketSHA256, receipt.PacketHash) || !sameReleaseHandoffPath(artifact.DecisionPath, receipt.DecisionPath) || !strings.EqualFold(artifact.DecisionSHA256, receipt.DecisionHash) || !sameReleaseHandoffPath(artifact.DecisionReceiptPath, decisionReceiptPath) || !strings.EqualFold(artifact.DecisionReceiptSHA256, sha256ReleaseHandoff(decisionReceiptData)) || artifact.ProvisionID != provisionID || strings.TrimSpace(artifact.ProvisionSHA256) == "" || !sameReleaseHandoffPath(artifact.WorkspaceRoot, workspace) || !sameReleaseHandoffPath(artifact.IntentPath, intentPath) || !sameReleaseHandoffPath(artifact.FreshCaseRoot, filepath.Join(workspace, "fresh")) || !sameReleaseHandoffPath(artifact.AttachedCaseRoot, filepath.Join(workspace, "attached")) || artifact.VerificationPreviewCommand != receipt.VerificationCommand || len(artifact.Cases) != 2 || len(artifact.Boundary) == 0 {
		return fmt.Errorf("candidate verification provisioning artifact authority binding mismatch: %s", artifact.IntentPath)
	}
	if !strings.EqualFold(artifact.ProvisionSHA256, candidateVerificationProvisionHandoffSHA256(artifact)) {
		return fmt.Errorf("candidate verification provisioning hash mismatch: %s", artifact.IntentPath)
	}
	roles := map[string]string{"fresh": artifact.FreshCaseRoot, "attached": artifact.AttachedCaseRoot}
	for _, item := range artifact.Cases {
		expectedRoot, ok := roles[item.Role]
		if !ok || !sameReleaseHandoffPath(item.CaseRoot, expectedRoot) || strings.TrimSpace(item.ProjectName) == "" || len(item.Writes) == 0 {
			return fmt.Errorf("candidate verification provisioning case binding mismatch: %s", artifact.IntentPath)
		}
	}
	return nil
}

func candidateVerificationProvisionHandoffSHA256(artifact candidateVerificationProvisionArtifactInventory) string {
	result := promote.CandidateVerificationProvisionResult{
		SchemaVersion:              artifact.SchemaVersion,
		Kind:                       strings.TrimSuffix(strings.TrimSuffix(artifact.Kind, "-intent"), "-receipt"),
		RepoRoot:                   artifact.RepoRoot,
		SourceCaseRoot:             artifact.SourceCaseRoot,
		Pack:                       artifact.Pack,
		PacketPath:                 artifact.PacketPath,
		PacketSHA256:               artifact.PacketSHA256,
		DecisionPath:               artifact.DecisionPath,
		DecisionSHA256:             artifact.DecisionSHA256,
		DecisionReceiptPath:        artifact.DecisionReceiptPath,
		DecisionReceiptSHA256:      artifact.DecisionReceiptSHA256,
		ProvisionID:                artifact.ProvisionID,
		WorkspaceRoot:              artifact.WorkspaceRoot,
		IntentPath:                 artifact.IntentPath,
		ReceiptPath:                filepath.Join(artifact.WorkspaceRoot, "provision.receipt.json"),
		Cases:                      append([]promote.CandidateVerificationProvisionCase{}, artifact.Cases...),
		VerificationPreviewCommand: artifact.VerificationPreviewCommand,
		Boundary:                   append([]string{}, artifact.Boundary...),
	}
	data, _ := json.Marshal(result)
	return sha256ReleaseHandoff(data)
}

func candidateVerificationProvisionHandoffApplyCommand(receipt candidateDecisionReceiptInventory, expected string) string {
	workspace := receipt.VerificationWorkspaceRoot
	command := fmt.Sprintf("/rekit promote -PacketPath %s -CandidateDecisionPath %s -ProvisionCandidateVerificationCases -FreshCaseRoot %s -AttachedCaseRoot %s -ExpectedProvisionSha256 %s -Apply -Format json", quoteReleaseHandoffCommandArg(receipt.PacketPath), quoteReleaseHandoffCommandArg(receipt.DecisionPath), quoteReleaseHandoffCommandArg(filepath.Join(workspace, "fresh")), quoteReleaseHandoffCommandArg(filepath.Join(workspace, "attached")), quoteReleaseHandoffCommandArg(expected))
	return releaseHandoffPromoteCommandWithTarget(command, receipt.CaseRoot)
}

func candidateVerificationRetirementHandoffApplyCommand(receipt candidateDecisionReceiptInventory, expected string) string {
	command := fmt.Sprintf("/rekit promote -PacketPath %s -CandidateDecisionPath %s -RetireCandidateVerificationWorkspace -ExpectedRetirementSha256 %s -Apply -Format json", quoteReleaseHandoffCommandArg(receipt.PacketPath), quoteReleaseHandoffCommandArg(receipt.DecisionPath), quoteReleaseHandoffCommandArg(expected))
	return releaseHandoffPromoteCommandWithTarget(command, receipt.CaseRoot)
}

func candidateVerificationProvisionArtifactMatches(left, right candidateVerificationProvisionArtifactInventory) bool {
	right.Kind = strings.TrimSuffix(right.Kind, "-receipt") + "-intent"
	return reflect.DeepEqual(left, right)
}

func quoteReleaseHandoffCommandArg(value string) string {
	return `"` + strings.ReplaceAll(value, `"`, `\"`) + `"`
}

func releaseHandoffPackMemoryProofCommandWithBindings(command, packetPath, decisionPath string, evidenceRefs []string, target string) string {
	command = strings.TrimSpace(command)
	if command == "" {
		return ""
	}
	if strings.TrimSpace(packetPath) != "" {
		command = strings.ReplaceAll(command, "<packet.json>", quoteReleaseHandoffCommandArg(packetPath))
	}
	if strings.TrimSpace(decisionPath) != "" {
		command = strings.ReplaceAll(command, "<candidate-decisions.json>", quoteReleaseHandoffCommandArg(decisionPath))
	}
	if len(evidenceRefs) > 0 {
		command = strings.ReplaceAll(command, "<review-evidence-ref>", quoteReleaseHandoffCommandArg(strings.Join(evidenceRefs, ",")))
	}
	return releaseHandoffPromoteCommandWithTarget(command, target)
}

func releaseHandoffPromoteCommandWithTarget(command, target string) string {
	command = strings.TrimSpace(command)
	target = strings.TrimSpace(target)
	if command == "" || target == "" || !strings.HasPrefix(command, "/rekit promote") || releaseHandoffPromoteCommandHasTarget(command) {
		return command
	}
	return strings.Replace(command, "/rekit promote", "/rekit promote -Target "+quoteReleaseHandoffCommandArg(target), 1)
}

func releaseHandoffPromoteCommandHasTarget(command string) bool {
	for field := range strings.FieldsSeq(command) {
		switch {
		case field == "-Target" || field == "--target":
			return true
		case strings.HasPrefix(field, "-Target=") || strings.HasPrefix(field, "--target="):
			return true
		}
	}
	return false
}

func populateCandidateVerificationRetirementHandoff(repo, proofRoot, decisionReceiptPath string, decisionReceiptData []byte, receipt candidateDecisionReceiptInventory, proof candidateDecisionVerificationInventory, handoff *ReleaseHandoffPackMemoryCandidateDecisionReceipt) error {
	retirementID := shortReleaseHandoffHash(receipt.PacketHash + receipt.DecisionHash)
	intentPath := filepath.Join(proofRoot, retirementID+".candidate-verification-retirement-intent.json")
	retirementReceiptPath := filepath.Join(proofRoot, retirementID+".candidate-verification-retirement-receipt.json")
	handoff.RetirementPreviewCommand = releaseHandoffPromoteCommandWithTarget(proof.RetirementPreviewCommand, receipt.CaseRoot)
	handoff.RetirementIntentPath = releaseHandoffRepoRelative(repo, intentPath)
	handoff.RetirementReceiptPath = releaseHandoffRepoRelative(repo, retirementReceiptPath)

	intent, intentPresent, err := readCandidateVerificationRetirementHandoffArtifact(repo, intentPath)
	if err != nil {
		return err
	}
	retirementReceipt, receiptPresent, err := readCandidateVerificationRetirementHandoffArtifact(repo, retirementReceiptPath)
	if err != nil {
		return err
	}
	if !intentPresent && !receiptPresent {
		if err := validateCandidateVerificationRetirementWorkspaceEvidence(receipt, proof); err != nil {
			return err
		}
		handoff.RetirementStatus = "required"
		handoff.RetirementRequired = true
		handoff.RetirementNextAction = "run the returned retirementPreviewCommand; inspect the exact plan, then run its expected-hash Apply command"
		return nil
	}
	if receiptPresent && !intentPresent {
		return fmt.Errorf("candidate verification retirement receipt lacks current retained intent: %s", retirementReceiptPath)
	}
	if err := validateCandidateVerificationRetirementHandoffArtifact(repo, proofRoot, decisionReceiptPath, decisionReceiptData, receipt, proof, intent, "-intent"); err != nil {
		return err
	}
	handoff.RetirementSHA256 = intent.RetirementSHA256
	if !receiptPresent {
		if err := validateCandidateVerificationRetirementIntentProgress(receipt, proof, intent); err != nil {
			return err
		}
		handoff.RetirementStatus = "in-progress"
		handoff.RetirementInProgress = true
		handoff.RetirementNextAction = candidateVerificationRetirementHandoffApplyCommand(receipt, intent.RetirementSHA256)
		return nil
	}
	if !reflect.DeepEqual(intent, candidateVerificationRetirementIntentFromReceipt(retirementReceipt)) {
		return fmt.Errorf("candidate verification retirement receipt/intent authority binding mismatch: %s", retirementReceiptPath)
	}
	if err := validateCandidateVerificationRetirementHandoffArtifact(repo, proofRoot, decisionReceiptPath, decisionReceiptData, receipt, proof, retirementReceipt, "-receipt"); err != nil {
		return err
	}
	if _, err := os.Lstat(receipt.VerificationWorkspaceRoot); err == nil || !os.IsNotExist(err) {
		return fmt.Errorf("retired candidate verification workspace has reappeared; refusing automatic deletion: %s", receipt.VerificationWorkspaceRoot)
	}
	handoff.RetirementStatus = "retired"
	handoff.Retired = true
	handoff.RetirementNextAction = "retain the repo-local retirement intent and receipt as final evidence"
	return nil
}

func readCandidateVerificationRetirementHandoffArtifact(repo, path string) (candidateVerificationRetirementArtifactInventory, bool, error) {
	if err := rejectReleaseHandoffSymlinkPath(repo, path, true); err != nil {
		return candidateVerificationRetirementArtifactInventory{}, false, err
	}
	info, err := os.Lstat(path)
	if os.IsNotExist(err) {
		return candidateVerificationRetirementArtifactInventory{}, false, nil
	}
	if err != nil || info.Mode()&os.ModeSymlink != 0 || !info.Mode().IsRegular() || info.Size() == 0 || info.Size() > 1024*1024 {
		return candidateVerificationRetirementArtifactInventory{}, false, fmt.Errorf("candidate verification retirement artifact must be a non-empty regular file: %s", path)
	}
	data, err := os.ReadFile(path)
	if err != nil {
		return candidateVerificationRetirementArtifactInventory{}, false, err
	}
	var artifact candidateVerificationRetirementArtifactInventory
	if err := decodeReleaseHandoffStrictJSON(data, &artifact); err != nil {
		return candidateVerificationRetirementArtifactInventory{}, false, fmt.Errorf("decode candidate verification retirement artifact %s: %w", path, err)
	}
	return artifact, true, nil
}

func validateCandidateVerificationRetirementHandoffArtifact(repo, proofRoot, decisionReceiptPath string, decisionReceiptData []byte, receipt candidateDecisionReceiptInventory, proof candidateDecisionVerificationInventory, artifact candidateVerificationRetirementArtifactInventory, kindSuffix string) error {
	retirementID := shortReleaseHandoffHash(receipt.PacketHash + receipt.DecisionHash)
	workspace := receipt.VerificationWorkspaceRoot
	intentPath := filepath.Join(proofRoot, retirementID+".candidate-verification-retirement-intent.json")
	retirementReceiptPath := filepath.Join(proofRoot, retirementID+".candidate-verification-retirement-receipt.json")
	expectedKind := "pack-memory-candidate-verification-retirement" + kindSuffix
	if artifact.SchemaVersion != 1 || artifact.Kind != expectedKind || artifact.Pack != receipt.Pack || !sameReleaseHandoffPath(artifact.RepoRoot, repo) || !sameReleaseHandoffPath(artifact.SourceCaseRoot, receipt.CaseRoot) || !sameReleaseHandoffPath(artifact.PacketPath, receipt.PacketPath) || !strings.EqualFold(artifact.PacketSHA256, receipt.PacketHash) || !sameReleaseHandoffPath(artifact.DecisionPath, receipt.DecisionPath) || !strings.EqualFold(artifact.DecisionSHA256, receipt.DecisionHash) || !sameReleaseHandoffPath(artifact.DecisionReceiptPath, decisionReceiptPath) || !strings.EqualFold(artifact.DecisionReceiptSHA256, sha256ReleaseHandoff(decisionReceiptData)) || !sameReleaseHandoffPath(artifact.VerificationProofPath, proof.VerificationProofPath) || !strings.EqualFold(artifact.VerificationProofSHA256, fileSHA256ReleaseHandoff(proof.VerificationProofPath)) || !sameReleaseHandoffPath(artifact.ProvisionIntentPath, proof.ProvisionIntentPath) || !strings.EqualFold(artifact.ProvisionIntentSHA256, proof.ProvisionIntentSHA256) || !sameReleaseHandoffPath(artifact.ProvisionReceiptPath, proof.ProvisionReceiptPath) || !strings.EqualFold(artifact.ProvisionReceiptSHA256, proof.ProvisionReceiptSHA256) || !sameReleaseHandoffPath(artifact.WorkspaceRoot, workspace) || !sameReleaseHandoffPath(artifact.RetirementIntentPath, intentPath) || !sameReleaseHandoffPath(artifact.RetirementReceiptPath, retirementReceiptPath) || strings.TrimSpace(artifact.RetirementSHA256) == "" || len(artifact.Roots) != 2 || len(artifact.RetirementPlans) != 2 {
		return fmt.Errorf("candidate verification retirement artifact authority binding mismatch: %s", artifact.RetirementIntentPath)
	}
	if len(artifact.ProvisionArtifactsToDelete) != 2 || !sameReleaseHandoffPath(artifact.ProvisionArtifactsToDelete[0], proof.ProvisionReceiptPath) || !sameReleaseHandoffPath(artifact.ProvisionArtifactsToDelete[1], proof.ProvisionIntentPath) || len(artifact.EmptyAncestorsToRemove) != 2 || !sameReleaseHandoffPath(artifact.EmptyAncestorsToRemove[0], filepath.Dir(workspace)) || !sameReleaseHandoffPath(artifact.EmptyAncestorsToRemove[1], filepath.Dir(filepath.Dir(workspace))) {
		return fmt.Errorf("candidate verification retirement artifact cleanup binding mismatch: %s", artifact.RetirementIntentPath)
	}
	for i, role := range []string{"fresh", "attached"} {
		if artifact.Roots[i].Role != role || !sameReleaseHandoffPath(artifact.Roots[i].CaseRoot, filepath.Join(workspace, role)) || len(artifact.Roots[i].Deletes) == 0 {
			return fmt.Errorf("candidate verification retirement artifact root binding mismatch: %s", artifact.RetirementIntentPath)
		}
	}
	result := candidateVerificationRetirementResultFromInventory(artifact, kindSuffix)
	if !strings.EqualFold(artifact.RetirementSHA256, candidateVerificationRetirementHash(result)) {
		return fmt.Errorf("candidate verification retirement artifact stable hash mismatch: %s", artifact.RetirementIntentPath)
	}
	return nil
}

func candidateVerificationRetirementResultFromInventory(artifact candidateVerificationRetirementArtifactInventory, kindSuffix string) promote.CandidateVerificationRetirementResult {
	roots := make([]promote.CandidateVerificationRetirementRoot, 0, len(artifact.Roots))
	for _, root := range artifact.Roots {
		roots = append(roots, promote.CandidateVerificationRetirementRoot{Role: root.Role, CaseRoot: root.CaseRoot, Deletes: append([]string(nil), root.Deletes...)})
	}
	return promote.CandidateVerificationRetirementResult{
		SchemaVersion: artifact.SchemaVersion, Kind: strings.TrimSuffix(artifact.Kind, kindSuffix), RepoRoot: artifact.RepoRoot, SourceCaseRoot: artifact.SourceCaseRoot, Pack: artifact.Pack, PacketPath: artifact.PacketPath, PacketSHA256: artifact.PacketSHA256, DecisionPath: artifact.DecisionPath, DecisionSHA256: artifact.DecisionSHA256, DecisionReceiptPath: artifact.DecisionReceiptPath, DecisionReceiptSHA256: artifact.DecisionReceiptSHA256, VerificationProofPath: artifact.VerificationProofPath, VerificationProofSHA256: artifact.VerificationProofSHA256, ProvisionIntentPath: artifact.ProvisionIntentPath, ProvisionIntentSHA256: artifact.ProvisionIntentSHA256, ProvisionReceiptPath: artifact.ProvisionReceiptPath, ProvisionReceiptSHA256: artifact.ProvisionReceiptSHA256, WorkspaceRoot: artifact.WorkspaceRoot, RetirementIntentPath: artifact.RetirementIntentPath, RetirementReceiptPath: artifact.RetirementReceiptPath, Roots: roots, ProvisionArtifactsToDelete: append([]string(nil), artifact.ProvisionArtifactsToDelete...), EmptyAncestorsToRemove: append([]string(nil), artifact.EmptyAncestorsToRemove...), Boundary: append([]string(nil), artifact.Boundary...), RetirementPlans: append([]syncpkg.ExclusiveInitRetirementPlan(nil), artifact.RetirementPlans...),
	}
}

func candidateVerificationRetirementHash(result promote.CandidateVerificationRetirementResult) string {
	result.Mode = ""
	result.RetirementSHA256 = ""
	result.IsMutation = false
	result.Applied = false
	result.Replay = false
	result.ApplyCommand = ""
	result.NextSteps = nil
	data, _ := json.Marshal(result)
	return sha256ReleaseHandoff(data)
}

func candidateVerificationRetirementIntentFromReceipt(receipt candidateVerificationRetirementArtifactInventory) candidateVerificationRetirementArtifactInventory {
	receipt.Kind = strings.TrimSuffix(receipt.Kind, "-receipt") + "-intent"
	return receipt
}

func validateCandidateVerificationRetirementIntentProgress(receipt candidateDecisionReceiptInventory, proof candidateDecisionVerificationInventory, intent candidateVerificationRetirementArtifactInventory) error {
	if _, err := syncpkg.InspectExclusiveInitRetirementBatch(syncpkg.ExclusiveInitRetirementResume, intent.RetirementPlans...); err != nil {
		return fmt.Errorf("candidate verification retirement cannot resume: %w", err)
	}
	for _, item := range []struct {
		path   string
		sha256 string
	}{
		{path: proof.ProvisionIntentPath, sha256: proof.ProvisionIntentSHA256},
		{path: proof.ProvisionReceiptPath, sha256: proof.ProvisionReceiptSHA256},
	} {
		if err := validateCandidateVerificationRetiringArtifact(item.path, item.sha256); err != nil {
			return err
		}
	}
	if _, err := os.Lstat(receipt.VerificationWorkspaceRoot); err != nil && !os.IsNotExist(err) {
		return err
	}
	return nil
}

func validateCandidateVerificationRetiringArtifact(path, expectedHash string) error {
	quarantine := filepath.Join(filepath.Dir(path), "."+filepath.Base(path)+".retiring-"+expectedHash[:16])
	canonicalInfo, canonicalErr := os.Lstat(path)
	quarantineInfo, quarantineErr := os.Lstat(quarantine)
	canonicalPresent := canonicalErr == nil
	quarantinePresent := quarantineErr == nil
	if canonicalErr != nil && !os.IsNotExist(canonicalErr) {
		return canonicalErr
	}
	if quarantineErr != nil && !os.IsNotExist(quarantineErr) {
		return quarantineErr
	}
	if canonicalPresent && quarantinePresent {
		return fmt.Errorf("candidate verification retirement canonical artifact and quarantine both exist: %s", path)
	}
	if !canonicalPresent && !quarantinePresent {
		return nil
	}
	currentPath, info := path, canonicalInfo
	if quarantinePresent {
		currentPath, info = quarantine, quarantineInfo
	}
	if info.Mode()&os.ModeSymlink != 0 || !info.Mode().IsRegular() {
		return fmt.Errorf("candidate verification retirement provision artifact is not regular: %s", currentPath)
	}
	if !strings.EqualFold(fileSHA256ReleaseHandoff(currentPath), expectedHash) {
		return fmt.Errorf("candidate verification retirement provision artifact changed: %s", currentPath)
	}
	return nil
}

func validateCandidateVerificationRetirementWorkspaceEvidence(receipt candidateDecisionReceiptInventory, proof candidateDecisionVerificationInventory) error {
	for _, item := range []struct {
		path   string
		sha256 string
		label  string
	}{
		{path: proof.ProvisionIntentPath, sha256: proof.ProvisionIntentSHA256, label: "provision intent"},
		{path: proof.ProvisionReceiptPath, sha256: proof.ProvisionReceiptSHA256, label: "provision receipt"},
	} {
		info, err := os.Lstat(item.path)
		if err != nil || info.Mode()&os.ModeSymlink != 0 || !info.Mode().IsRegular() || !strings.EqualFold(fileSHA256ReleaseHandoff(item.path), item.sha256) {
			return fmt.Errorf("candidate verification %s is missing or invalid: %s", item.label, item.path)
		}
	}
	for _, path := range []string{receipt.VerificationWorkspaceRoot, proof.FreshCaseRoot, proof.AttachedCaseRoot} {
		info, err := os.Lstat(path)
		if err != nil || info.Mode()&os.ModeSymlink != 0 || !info.IsDir() {
			return fmt.Errorf("candidate verification workspace is missing or invalid: %s", path)
		}
	}
	return nil
}

func validateCandidateDecisionReceiptAttestation(repo, receiptPath string, receiptData []byte, receipt candidateDecisionReceiptInventory) error {
	for _, path := range []string{receipt.PacketPath, receipt.DecisionPath} {
		info, err := os.Lstat(path)
		if err != nil {
			return err
		}
		if info.Mode()&os.ModeSymlink != 0 || !info.Mode().IsRegular() || info.Size() == 0 || info.Size() > 1024*1024 {
			return fmt.Errorf("packet or decision must be a non-empty regular file: %s", path)
		}
	}
	packetData, err := os.ReadFile(receipt.PacketPath)
	if err != nil {
		return err
	}
	decisionData, err := os.ReadFile(receipt.DecisionPath)
	if err != nil {
		return err
	}
	if sha256ReleaseHandoff(packetData) != receipt.PacketHash || sha256ReleaseHandoff(decisionData) != receipt.DecisionHash {
		return fmt.Errorf("packet or decision hash mismatch")
	}
	var packet promote.CandidateReviewPacket
	if err := decodeReleaseHandoffStrictJSON(packetData, &packet); err != nil {
		return fmt.Errorf("decode candidate review packet: %w", err)
	}
	candidateRoot := filepath.Join(repo, "packs", receipt.Pack, "promote-candidates")
	toolingRoot := filepath.Join(repo, "packs", receipt.Pack, "tooling", "candidates")
	canonicalIndex := filepath.Join(candidateRoot, "index.json")
	if packet.SchemaVersion != 1 || packet.Kind != "pack-memory-candidate-review" || packet.Command != "promote" || packet.CandidateResult.Pack != receipt.Pack || !sameReleaseHandoffPath(packet.CandidateResult.RepoRoot, repo) || !sameReleaseHandoffPath(packet.CandidateResult.CaseRoot, receipt.CaseRoot) || !sameReleaseHandoffPath(packet.CandidateResult.CandidateRoot, candidateRoot) || !sameReleaseHandoffPath(packet.CandidateResult.ToolingRoot, toolingRoot) || strings.TrimSpace(packet.CandidateResult.IndexPath) != "" && !sameReleaseHandoffPath(packet.CandidateResult.IndexPath, canonicalIndex) {
		return fmt.Errorf("candidate review packet binding mismatch")
	}
	var decision candidateDecisionFileInventory
	if err := decodeReleaseHandoffStrictJSON(decisionData, &decision); err != nil {
		return fmt.Errorf("decode candidate decision: %w", err)
	}
	if decision.SchemaVersion != 1 || decision.Kind != "pack-memory-candidate-decisions" || decision.PacketHash != receipt.PacketHash || len(decision.Decisions) != len(receipt.Actions) {
		return fmt.Errorf("candidate decision binding mismatch")
	}
	reviewByCandidate := map[string]promote.CandidateReviewItem{}
	for _, review := range packet.CandidateResult.ReviewPlan.ReviewItems {
		reviewByCandidate[filepath.Clean(review.CandidatePath)] = review
	}
	writeByCandidate := map[string]promote.CandidateWrite{}
	for _, write := range packet.CandidateResult.Writes {
		if write.Action == "create-candidate" {
			writeByCandidate[filepath.Clean(write.TargetPath)] = write
		}
	}
	transactionPath := filepath.Join(receipt.BackupRoot, "transaction.json")
	markerPath := filepath.Join(receipt.BackupRoot, "committed.json")
	for _, path := range []string{transactionPath, markerPath} {
		if err := rejectReleaseHandoffSymlinkPath(receipt.BackupRoot, path, false); err != nil {
			return err
		}
	}
	transactionData, err := os.ReadFile(transactionPath)
	if err != nil {
		return err
	}
	markerData, err := os.ReadFile(markerPath)
	if err != nil {
		return err
	}
	var transaction candidateDecisionTransactionInventory
	if err := decodeReleaseHandoffStrictJSON(transactionData, &transaction); err != nil {
		return fmt.Errorf("decode candidate transaction: %w", err)
	}
	var committed candidateDecisionResultInventory
	if err := decodeReleaseHandoffStrictJSON(markerData, &committed); err != nil {
		return fmt.Errorf("decode candidate committed result: %w", err)
	}
	if transaction.SchemaVersion != 1 || transaction.Kind != "pack-memory-candidate-decision-transaction" || transaction.PacketHash != receipt.PacketHash || transaction.DecisionHash != receipt.DecisionHash || !sameReleaseHandoffPath(transaction.Result.RepoRoot, repo) || !sameReleaseHandoffPath(transaction.Result.CaseRoot, receipt.CaseRoot) || transaction.Result.Pack != receipt.Pack || !sameReleaseHandoffPath(transaction.Result.BackupRoot, receipt.BackupRoot) || !sameReleaseHandoffPath(transaction.Result.IndexPath, receipt.IndexPath) || !sameReleaseHandoffPath(committed.RepoRoot, repo) || !sameReleaseHandoffPath(committed.CaseRoot, receipt.CaseRoot) || committed.Pack != receipt.Pack || !sameReleaseHandoffPath(committed.PacketPath, receipt.PacketPath) || !sameReleaseHandoffPath(committed.DecisionPath, receipt.DecisionPath) || !sameReleaseHandoffPath(committed.BackupRoot, receipt.BackupRoot) || !sameReleaseHandoffPath(committed.IndexPath, receipt.IndexPath) || committed.PacketHash != receipt.PacketHash || !committed.IsMutation || !committed.Applied || committed.RolledBack || committed.RecoveryRequired || committed.Accepted != receipt.Accepted || committed.Rejected != receipt.Rejected || committed.Superseded != receipt.Superseded || !sameReleaseHandoffPath(committed.ReceiptPath, receiptPath) || committed.Receipt == nil || !reflect.DeepEqual(*committed.Receipt, receipt) || !candidateDecisionActionsEqual(transaction.Actions, receipt.Actions) || !candidateDecisionActionsEqual(transaction.Result.Actions, receipt.Actions) || !candidateDecisionActionsEqual(committed.Actions, receipt.Actions) {
		return fmt.Errorf("transaction or committed result binding mismatch")
	}
	decisionByCandidate := map[string]candidateDecisionItemInventory{}
	for _, item := range decision.Decisions {
		candidatePath, err := filepath.Abs(strings.TrimSpace(item.CandidatePath))
		if err != nil || strings.TrimSpace(item.CandidatePath) == "" {
			return fmt.Errorf("candidate decision has invalid candidatePath %q", item.CandidatePath)
		}
		candidatePath = filepath.Clean(candidatePath)
		if _, exists := decisionByCandidate[candidatePath]; exists {
			return fmt.Errorf("duplicate candidate decision: %s", candidatePath)
		}
		decisionByCandidate[candidatePath] = item
	}
	for _, action := range receipt.Actions {
		item, ok := decisionByCandidate[filepath.Clean(action.CandidatePath)]
		review, reviewed := reviewByCandidate[filepath.Clean(action.CandidatePath)]
		write, written := writeByCandidate[filepath.Clean(action.CandidatePath)]
		expectedAction := map[string]string{"accept": "merge-accepted-candidate-and-cleanup", "reject": "cleanup-rejected-candidate", "superseded": "cleanup-superseded-candidate"}[action.Decision]
		if !ok || !reviewed || !written || review.ReviewDecision != "pending-review" || review.Kind != action.Kind || write.Kind != action.Kind || write.Path != review.Path || action.Action != expectedAction || strings.ToLower(strings.TrimSpace(item.Decision)) != action.Decision || strings.TrimSpace(item.CandidateHash) == "" || strings.TrimSpace(item.Reason) == "" || strings.TrimSpace(item.Actor) == "" || len(item.EvidenceRefs) == 0 || action.PackTarget != review.PackTarget {
			return fmt.Errorf("action is not bound to reviewed packet decision: %s", action.CandidatePath)
		}
		if strings.TrimSpace(action.CandidateBackupPath) == "" || !pathWithinReleaseHandoffRoot(receipt.BackupRoot, action.CandidateBackupPath) {
			return fmt.Errorf("candidate backup leaves backup root: %s", action.CandidatePath)
		}
		if err := validateReleaseHandoffHashedFile(receipt.BackupRoot, action.CandidateBackupPath, item.CandidateHash, "candidate backup"); err != nil {
			return err
		}
		if strings.TrimSpace(action.TargetBackupPath) != "" {
			if !pathWithinReleaseHandoffRoot(receipt.BackupRoot, action.TargetBackupPath) {
				return fmt.Errorf("target backup leaves backup root: %s", action.PackTarget)
			}
			if err := validateReleaseHandoffHashedFile(receipt.BackupRoot, action.TargetBackupPath, item.PackTargetHash, "target backup"); err != nil {
				return err
			}
		}
		expectedEvidence := make([]string, 0, len(item.EvidenceRefs))
		for _, evidence := range item.EvidenceRefs {
			full := evidence.Path
			if !filepath.IsAbs(full) {
				full = filepath.Join(receipt.CaseRoot, filepath.FromSlash(full))
			}
			root := receipt.CaseRoot
			if pathWithinReleaseHandoffRoot(repo, full) {
				root = repo
			} else if !pathWithinReleaseHandoffRoot(receipt.CaseRoot, full) {
				return fmt.Errorf("decision evidence leaves repo/case roots: %s", full)
			}
			if err := rejectReleaseHandoffSymlinkPath(root, full, false); err != nil {
				return err
			}
			if strings.TrimSpace(evidence.SHA256) == "" || !strings.EqualFold(fileSHA256ReleaseHandoff(full), evidence.SHA256) {
				return fmt.Errorf("decision evidence hash mismatch: %s", full)
			}
			expectedEvidence = append(expectedEvidence, filepath.Clean(full))
		}
		if !slicesEqual(action.EvidenceRefs, expectedEvidence) {
			return fmt.Errorf("action evidence binding mismatch: %s", action.CandidatePath)
		}
	}
	canonicalReceipt := filepath.Join(filepath.Dir(receiptPath), shortReleaseHandoffHash(receipt.PacketHash+receipt.DecisionHash)+".candidate-decision-receipt.json")
	if len(receiptData) == 0 || !sameReleaseHandoffPath(receipt.ReceiptPath, receiptPath) || !sameReleaseHandoffPath(receiptPath, canonicalReceipt) {
		return fmt.Errorf("receipt path binding mismatch")
	}
	return nil
}

func fileSHA256ReleaseHandoff(path string) string {
	data, err := os.ReadFile(path)
	if err != nil {
		return ""
	}
	return sha256ReleaseHandoff(data)
}

func validateReleaseHandoffHashedFile(root, path, expectedHash, label string) error {
	if strings.TrimSpace(path) == "" || strings.TrimSpace(expectedHash) == "" {
		return fmt.Errorf("%s path or hash is empty", label)
	}
	if err := rejectReleaseHandoffSymlinkPath(root, path, false); err != nil {
		return err
	}
	info, err := os.Lstat(path)
	if err != nil {
		return err
	}
	if info.Mode()&os.ModeSymlink != 0 || !info.Mode().IsRegular() || info.Size() == 0 {
		return fmt.Errorf("%s must be a non-empty regular file: %s", label, path)
	}
	if !strings.EqualFold(fileSHA256ReleaseHandoff(path), expectedHash) {
		return fmt.Errorf("%s hash mismatch: %s", label, path)
	}
	return nil
}

func rejectReleaseHandoffSymlinkPath(root, path string, allowMissingLeaf bool) error {
	rootFull, err := filepath.Abs(root)
	if err != nil {
		return err
	}
	rootInfo, err := os.Lstat(rootFull)
	if os.IsNotExist(err) && allowMissingLeaf {
		return nil
	}
	if err != nil {
		return err
	}
	if rootInfo.Mode()&os.ModeSymlink != 0 {
		return fmt.Errorf("release handoff root must not be a symlink: %s", rootFull)
	}
	pathFull, err := filepath.Abs(path)
	if err != nil {
		return err
	}
	rel, err := filepath.Rel(rootFull, pathFull)
	if err != nil || rel == ".." || strings.HasPrefix(rel, ".."+string(filepath.Separator)) || filepath.IsAbs(rel) {
		return fmt.Errorf("release handoff path escapes root: %s", path)
	}
	current := rootFull
	for part := range strings.SplitSeq(rel, string(filepath.Separator)) {
		if part == "" || part == "." {
			continue
		}
		current = filepath.Join(current, part)
		info, err := os.Lstat(current)
		if os.IsNotExist(err) && allowMissingLeaf {
			return nil
		}
		if err != nil {
			return err
		}
		if info.Mode()&os.ModeSymlink != 0 {
			return fmt.Errorf("release handoff path must not traverse symlink: %s", current)
		}
	}
	return nil
}

func validateCandidateDecisionReceiptActions(candidateRoot string, receipt candidateDecisionReceiptInventory) error {
	packRoot := filepath.Dir(candidateRoot)
	toolingRoot := filepath.Join(packRoot, "tooling", "candidates")
	accepted := 0
	rejected := 0
	superseded := 0
	seen := map[string]struct{}{}
	for _, action := range receipt.Actions {
		candidatePath := filepath.Clean(action.CandidatePath)
		actionRoot := candidateRoot
		if action.Kind == "tooling-candidate-source" {
			if action.Decision == "accept" {
				return fmt.Errorf("tooling candidate cannot be accepted automatically: %s", action.CandidatePath)
			}
			actionRoot = toolingRoot
		} else if action.Kind != "managed-doc" {
			return fmt.Errorf("unsupported action kind %q", action.Kind)
		}
		if strings.TrimSpace(action.CandidatePath) == "" || !pathWithinReleaseHandoffRoot(actionRoot, candidatePath) || strings.TrimSpace(action.Action) == "" || action.EvidenceRefs == nil {
			return fmt.Errorf("invalid action for candidate %s", action.CandidatePath)
		}
		if _, ok := seen[candidatePath]; ok {
			return fmt.Errorf("duplicate candidate action: %s", action.CandidatePath)
		}
		seen[candidatePath] = struct{}{}
		switch action.Decision {
		case "accept":
			accepted++
			if strings.TrimSpace(action.PackTarget) == "" || strings.TrimSpace(action.CandidateBackupPath) == "" || !pathWithinReleaseHandoffRoot(receipt.BackupRoot, action.CandidateBackupPath) {
				return fmt.Errorf("accepted action lacks target or candidate backup: %s", action.CandidatePath)
			}
		case "reject":
			rejected++
		case "superseded":
			superseded++
		default:
			return fmt.Errorf("unsupported decision %q", action.Decision)
		}
	}
	if accepted != receipt.Accepted || rejected != receipt.Rejected || superseded != receipt.Superseded {
		return fmt.Errorf("decision counts do not match actions")
	}
	return nil
}

func sha256ReleaseHandoff(data []byte) string {
	sum := sha256.Sum256(data)
	return hex.EncodeToString(sum[:])
}

func shortReleaseHandoffHash(value string) string {
	return sha256ReleaseHandoff([]byte(value))[:16]
}

func decodeReleaseHandoffStrictJSON(data []byte, out any) error {
	decoder := json.NewDecoder(bytes.NewReader(data))
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(out); err != nil {
		return err
	}
	var trailing any
	if err := decoder.Decode(&trailing); err != io.EOF {
		if err == nil {
			return fmt.Errorf("multiple JSON values")
		}
		return err
	}
	return nil
}

func candidateDecisionActionsEqual(left, right []candidateDecisionActionInventory) bool {
	if len(left) != len(right) {
		return false
	}
	for i := range left {
		if left[i].CandidatePath != right[i].CandidatePath ||
			left[i].Kind != right[i].Kind ||
			left[i].Decision != right[i].Decision ||
			left[i].PackTarget != right[i].PackTarget ||
			left[i].Action != right[i].Action ||
			left[i].CandidateBackupPath != right[i].CandidateBackupPath ||
			left[i].TargetBackupPath != right[i].TargetBackupPath ||
			!slicesEqual(left[i].EvidenceRefs, right[i].EvidenceRefs) {
			return false
		}
	}
	return true
}

func slicesEqual(left, right []string) bool {
	if len(left) != len(right) {
		return false
	}
	for i := range left {
		if left[i] != right[i] {
			return false
		}
	}
	return true
}

func sameReleaseHandoffPath(left, right string) bool {
	leftAbs, leftErr := filepath.Abs(left)
	rightAbs, rightErr := filepath.Abs(right)
	if leftErr != nil || rightErr != nil {
		return false
	}
	leftAbs = filepath.Clean(leftAbs)
	rightAbs = filepath.Clean(rightAbs)
	if leftAbs == rightAbs {
		return true
	}
	leftInfo, leftErr := os.Stat(leftAbs)
	rightInfo, rightErr := os.Stat(rightAbs)
	return leftErr == nil && rightErr == nil && os.SameFile(leftInfo, rightInfo)
}

func pathWithinReleaseHandoffRoot(root, path string) bool {
	rootAbs, rootErr := filepath.Abs(root)
	pathAbs, pathErr := filepath.Abs(path)
	if rootErr != nil || pathErr != nil {
		return false
	}
	rel, err := filepath.Rel(rootAbs, pathAbs)
	return err == nil && rel != ".." && !strings.HasPrefix(rel, ".."+string(filepath.Separator))
}

func releaseHandoffRepoRelative(repo, path string) string {
	if strings.TrimSpace(path) == "" {
		return ""
	}
	if rel, err := filepath.Rel(repo, path); err == nil && rel != ".." && !strings.HasPrefix(rel, ".."+string(filepath.Separator)) {
		return filepath.ToSlash(rel)
	}
	return filepath.ToSlash(path)
}

func packMemoryCandidateReviewArtifactWithProof(status ReleaseHandoffPackMemoryCandidateStatus, artifact ReleaseHandoffPackMemoryCandidateReviewArtifact, proofRoot string) (ReleaseHandoffPackMemoryCandidateReviewArtifact, error) {
	if strings.TrimSpace(status.ProofRoot) == "" {
		return artifact, nil
	}
	stem := packMemoryCandidateProofStem(artifact.CandidatePath, artifact.PackTarget)
	for _, ext := range []string{".md", ".json", ".txt"} {
		name := stem + "." + artifact.Name + ext
		rel := filepath.ToSlash(filepath.Join(status.ProofRoot, name))
		artifact.ExpectedProofs = append(artifact.ExpectedProofs, rel)
		candidate := filepath.Join(proofRoot, name)
		info, err := os.Lstat(candidate)
		if os.IsNotExist(err) {
			continue
		}
		if err != nil {
			return artifact, err
		}
		if info.Mode()&os.ModeSymlink != 0 || !info.Mode().IsRegular() || info.Size() == 0 || info.Size() > 1024*1024 {
			return artifact, fmt.Errorf("candidate review proof must be a non-empty regular file: %s", candidate)
		}
		switch artifact.Name {
		case "candidate-decision-note":
			if err := validatePackMemoryCandidateDecisionProof(status, artifact, proofRoot, candidate); err != nil {
				return artifact, err
			}
		case "candidate-cleanup-proof", "pack-doctor-output", "fresh-case-reconsume-proof", "attached-case-reconsume-proof":
			if err := validatePackMemoryCandidateLifecycleProof(status, artifact, proofRoot, candidate); err != nil {
				return artifact, err
			}
		}
		if artifact.ProofPath == "" {
			artifact.ProofPath = rel
			artifact.ProofPresent = true
		}
	}
	return artifact, nil
}

func packMemoryCandidateProofStem(candidatePath, packTarget string) string {
	base := filepath.Base(filepath.FromSlash(strings.TrimSpace(candidatePath)))
	if base == "." || base == string(filepath.Separator) || base == "" {
		base = filepath.Base(filepath.FromSlash(strings.TrimSpace(packTarget)))
	}
	stem := strings.TrimSuffix(base, ".candidate.md")
	if stem == base {
		stem = strings.TrimSuffix(base, filepath.Ext(base))
	}
	return sanitizePackMemoryCandidateProofStem(stem)
}

func sanitizePackMemoryCandidateProofStem(stem string) string {
	var b strings.Builder
	for _, r := range stem {
		if (r >= 'a' && r <= 'z') || (r >= 'A' && r <= 'Z') || (r >= '0' && r <= '9') || r == '-' || r == '_' || r == '.' {
			b.WriteRune(r)
		} else {
			b.WriteRune('-')
		}
	}
	out := strings.Trim(b.String(), "-._")
	if out == "" {
		return "candidate"
	}
	return out
}

func packMemoryCandidatePackTarget(entries []ReleaseHandoffPackMemoryCandidateIndexEntry, candidatePath string) string {
	for _, entry := range entries {
		if entry.Candidate == candidatePath {
			return entry.Path
		}
	}
	return ""
}

func packMemoryCandidateProofPackTarget(status ReleaseHandoffPackMemoryCandidateStatus, packTarget string) string {
	packTarget = strings.TrimSpace(packTarget)
	if packTarget == "" {
		return ""
	}
	packTarget = filepath.ToSlash(packTarget)
	if strings.HasPrefix(packTarget, "packs/") {
		return packTarget
	}
	return filepath.ToSlash(filepath.Join("packs", status.Pack, filepath.FromSlash(packTarget)))
}

func packMemoryCandidateToolingProofPackTarget(status ReleaseHandoffPackMemoryCandidateStatus) string {
	return filepath.ToSlash(filepath.Dir(filepath.FromSlash(status.ToolingRoot)))
}

func candidateFiles(root, relRoot string) ([]string, error) {
	if _, err := os.Stat(root); os.IsNotExist(err) {
		return nil, nil
	} else if err != nil {
		return nil, err
	}
	paths := []string{}
	err := filepath.WalkDir(root, func(path string, d os.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if d.IsDir() {
			return nil
		}
		if !strings.HasSuffix(d.Name(), ".candidate.md") {
			return nil
		}
		rel, err := filepath.Rel(root, path)
		if err != nil {
			return err
		}
		paths = append(paths, filepath.ToSlash(filepath.Join(relRoot, rel)))
		return nil
	})
	if err != nil {
		return nil, err
	}
	sort.Strings(paths)
	return paths, nil
}

func candidateIndexEntries(path, repo, candidateRoot, candidateRootRel string) ([]ReleaseHandoffPackMemoryCandidateIndexEntry, bool, error) {
	b, err := os.ReadFile(path)
	if os.IsNotExist(err) {
		return nil, false, nil
	}
	if err != nil {
		return nil, false, err
	}
	entries := []ReleaseHandoffPackMemoryCandidateIndexEntry{}
	if err := json.Unmarshal(b, &entries); err != nil {
		return nil, true, err
	}
	for i := range entries {
		entries[i].Path = filepath.ToSlash(strings.TrimSpace(entries[i].Path))
		entries[i].Candidate = normalizeCandidateIndexPath(repo, candidateRoot, candidateRootRel, entries[i].Candidate)
	}
	sort.Slice(entries, func(i, j int) bool {
		if entries[i].Path == entries[j].Path {
			return entries[i].Candidate < entries[j].Candidate
		}
		return entries[i].Path < entries[j].Path
	})
	return entries, true, nil
}

func normalizeCandidateIndexPath(repo, candidateRoot, candidateRootRel, candidate string) string {
	candidate = strings.TrimSpace(candidate)
	if candidate == "" {
		return ""
	}
	if filepath.IsAbs(candidate) {
		if rel, err := filepath.Rel(repo, candidate); err == nil && !strings.HasPrefix(rel, "..") && rel != ".." {
			return filepath.ToSlash(rel)
		}
		if rel, err := filepath.Rel(candidateRoot, candidate); err == nil && !strings.HasPrefix(rel, "..") && rel != ".." {
			return filepath.ToSlash(filepath.Join(candidateRootRel, rel))
		}
		return filepath.ToSlash(candidate)
	}
	normalized := filepath.ToSlash(candidate)
	if strings.HasPrefix(normalized, candidateRootRel+"/") || normalized == candidateRootRel {
		return normalized
	}
	return filepath.ToSlash(filepath.Join(candidateRootRel, normalized))
}

func releaseHandoffPackMemoryCandidateDetails(inventory ReleaseHandoffPackMemoryCandidateList) []string {
	details := []string{
		fmt.Sprintf("openPacks=%d total=%d ready=%t", len(inventory.Packs), inventory.Total, inventory.Ready),
		fmt.Sprintf("nextAction=%s", inventory.NextAction),
	}
	for _, pack := range inventory.Packs {
		details = append(details, fmt.Sprintf("pack=%s maturity=%s candidateFiles=%d toolingFiles=%d indexEntries=%d reviewArtifacts=%d receipts=%d pendingVerification=%d completedVerification=%d requiresReview=%t requiresCleanup=%t requiresVerification=%t action=%s", pack.Pack, pack.Maturity, pack.CandidateFiles, pack.ToolingFiles, pack.IndexEntries, len(pack.ReviewArtifacts), len(pack.DecisionReceipts), pack.PendingVerifications, pack.CompletedVerifications, pack.RequiresReview, pack.RequiresCleanup, pack.RequiresVerification, pack.Action))
		for _, path := range pack.CandidatePaths {
			details = append(details, fmt.Sprintf("candidatePath pack=%s path=%s", pack.Pack, path))
		}
		for _, path := range pack.ToolingPaths {
			details = append(details, fmt.Sprintf("toolingPath pack=%s path=%s", pack.Pack, path))
		}
		for _, entry := range pack.IndexCandidates {
			details = append(details, fmt.Sprintf("indexCandidate pack=%s path=%s candidate=%s", pack.Pack, entry.Path, entry.Candidate))
		}
		for _, receipt := range pack.DecisionReceipts {
			details = append(details, fmt.Sprintf("decisionReceipt pack=%s path=%s accepted=%d rejected=%d superseded=%d verificationPending=%t verificationComplete=%t workspace=%s provisionCommand=%s proofPath=%s command=%s provisionStatus=%s provisionInProgress=%t provisionComplete=%t provisionApplyCommand=%s provisionIntent=%s provisionReceipt=%s provisionSha256=%s provisionNextAction=%s retirementStatus=%s retirementRequired=%t retirementInProgress=%t retired=%t retirementIntent=%s retirementReceipt=%s retirementSha256=%s retirementPreviewCommand=%s retirementNextAction=%s", pack.Pack, receipt.Path, receipt.Accepted, receipt.Rejected, receipt.Superseded, receipt.VerificationPending, receipt.VerificationComplete, receipt.VerificationWorkspaceRoot, receipt.VerificationProvisionCommand, receipt.VerificationProofPath, receipt.VerificationCommand, receipt.ProvisionStatus, receipt.ProvisionInProgress, receipt.ProvisionComplete, receipt.ProvisionApplyCommand, receipt.ProvisionIntentPath, receipt.ProvisionReceiptPath, receipt.ProvisionSHA256, receipt.ProvisionNextAction, receipt.RetirementStatus, receipt.RetirementRequired, receipt.RetirementInProgress, receipt.Retired, receipt.RetirementIntentPath, receipt.RetirementReceiptPath, receipt.RetirementSHA256, receipt.RetirementPreviewCommand, receipt.RetirementNextAction))
		}
		if handoff := pack.DecisionDraftHandoff; handoff != nil {
			details = append(details, fmt.Sprintf("decisionDraftHandoff pack=%s mode=%s decisionPath=%s evidenceRefs=%d supportedDecisions=%s nextAction=%s", pack.Pack, handoff.Mode, handoff.DecisionPath, len(handoff.EvidenceRefs), strings.Join(handoff.SupportedDecisions, ","), handoff.NextAction))
		}
		summary := pack.ReviewSummary
		details = append(details, fmt.Sprintf("reviewSummary pack=%s total=%d decisionArtifacts=%d cleanupArtifacts=%d reconsumeArtifacts=%d proofPresent=%d proofMissing=%d proofComplete=%t nextAction=%s", pack.Pack, summary.Total, summary.DecisionArtifactCount, summary.CleanupArtifactCount, summary.ReconsumeArtifactCount, summary.ProofSummary.Present, summary.ProofSummary.Missing, summary.ProofSummary.Complete, summary.NextAction))
		details = append(details, fmt.Sprintf("proofSummary pack=%s total=%d present=%d missing=%d progress=%s stage=%s nextMissingType=%s nextMissingPath=%s nextMissingCandidate=%s nextMissingTarget=%s complete=%t proofRoot=%s nextAction=%s", pack.Pack, pack.ProofSummary.Total, pack.ProofSummary.Present, pack.ProofSummary.Missing, pack.ProofSummary.ProofProgress, pack.ProofSummary.CurrentStage, pack.ProofSummary.NextMissingProofType, pack.ProofSummary.NextMissingProofPath, pack.ProofSummary.NextMissingCandidatePath, pack.ProofSummary.NextMissingPackTarget, pack.ProofSummary.Complete, pack.ProofSummary.ProofRoot, pack.ProofSummary.NextAction))
		for _, artifact := range pack.ReviewArtifacts {
			details = append(details, fmt.Sprintf("reviewArtifact pack=%s name=%s candidatePath=%s packTarget=%s proofPresent=%t proofPath=%s expectedProofs=%s", pack.Pack, artifact.Name, artifact.CandidatePath, artifact.PackTarget, artifact.ProofPresent, artifact.ProofPath, strings.Join(artifact.ExpectedProofs, ",")))
		}
	}
	for _, warning := range inventory.Warnings {
		details = append(details, "warning="+warning)
	}
	return details
}

func releaseHandoffPackMaturityDetails(inventory ReleaseHandoffPackMaturity) []string {
	details := []string{
		fmt.Sprintf("total=%d schemaValid=%t schemaVersionReady=%t heavyToolGateReady=%t", inventory.Total, inventory.SchemaValid, inventory.SchemaVersionReady, inventory.HeavyToolGateReady),
		fmt.Sprintf("heavyToolGateActions=%s", strings.Join(inventory.HeavyToolGateActions, ",")),
	}
	maturities := make([]string, 0, len(inventory.MaturityCounts))
	for maturity := range inventory.MaturityCounts {
		maturities = append(maturities, maturity)
	}
	sort.Strings(maturities)
	for _, maturity := range maturities {
		details = append(details, fmt.Sprintf("%s=%d:%s", maturity, inventory.MaturityCounts[maturity], strings.Join(inventory.PacksByMaturity[maturity], ",")))
	}
	return details
}

func releaseHandoffKnownGaps(gaps []string) []ReleaseHandoffKnownGap {
	out := make([]ReleaseHandoffKnownGap, 0, len(gaps))
	for i, gap := range gaps {
		category := knownGapCategory(gap)
		gap = compactHandoffText(gap, 220)
		if strings.TrimSpace(gap) == "" {
			continue
		}
		out = append(out, ReleaseHandoffKnownGap{
			Index:    i + 1,
			Category: category,
			Summary:  gap,
		})
	}
	return out
}

func releaseHandoffKnownGapDetails(gaps []ReleaseHandoffKnownGap) []string {
	details := make([]string, 0, len(gaps))
	for _, gap := range gaps {
		details = append(details, fmt.Sprintf("%d:%s:%s", gap.Index, gap.Category, gap.Summary))
	}
	return details
}

func knownGapCategory(gap string) string {
	lower := strings.ToLower(gap)
	categories := []string{}
	categoryPhrases := []struct {
		category string
		phrases  []string
	}{
		{category: "ci-release-gate", phrases: []string{"远程 release-gate", "billing/spending limit"}},
		{category: "cross-platform-product-path", phrases: []string{"cross-platform", "product path"}},
		{category: "session-orchestration", phrases: []string{"session/reviewer orchestrator", "member session"}},
		{category: "dispatch", phrases: []string{"bounded dispatch", "reviewer"}},
		{category: "heavy-tool", phrases: []string{"heavy-tool"}},
		{category: "authority", phrases: []string{"authority/confirmed"}},
		{category: "pack-memory", phrases: []string{"pack-based team memory", "pack-memory"}},
		{category: "policy-schema", phrases: []string{"policy schema"}},
		{category: "powershell-deprecation", phrases: []string{"powershell"}},
	}
	for _, candidate := range categoryPhrases {
		for _, phrase := range candidate.phrases {
			if strings.Contains(lower, phrase) {
				categories = append(categories, candidate.category)
				break
			}
		}
	}
	if len(categories) == 0 {
		return "general"
	}
	return strings.Join(categories, ",")
}

func releaseHandoffValidation(steps []GateStep) []ReleaseHandoffValidation {
	validation := make([]ReleaseHandoffValidation, 0, len(steps))
	for _, step := range steps {
		validation = append(validation, ReleaseHandoffValidation{
			Command:  step.Command,
			Kind:     step.Kind,
			RepoPath: step.RepoPath,
			Required: step.Required,
			Present:  step.Present,
			Resolved: step.Resolved,
		})
	}
	return validation
}

func releaseHandoffNextActions() []string {
	return []string{
		"When latestBatch.handoff.releaseInspectionCadence.state=complete, select the next Windows-verifiable product-path batch; do not create a third inspection record unless a new remote signal appears.",
		"Read docs/context-routing.md first, then use releaseHandoff.signals[] to decide which detailed document is needed.",
		"Read only docs/batch-plan.md current/next/latest sections for routine continuation; search docs/batch-history.md only for old-batch archaeology.",
		"Use releaseHandoff.validation[] or gateProfile.steps[] as the local/CI minimum before tagging or handing off.",
		"Keep CHANGELOG.md Unreleased aligned with the latest completed docs/batch-plan.md batch before handing off.",
		"Continue the autonomous loop from docs/autonomous-goal.md only when goal/stop-condition detail is needed; do not paste the full goal into every handoff.",
	}
}

func releaseHandoffWarnings(handoff ReleaseHandoff) []string {
	warnings := []string{}
	for _, doc := range handoff.ReadFirst {
		if !doc.Present {
			warnings = append(warnings, fmt.Sprintf("release handoff read-first document missing: %s", doc.Path))
		}
	}
	if !handoff.LatestBatch.Present {
		warnings = append(warnings, fmt.Sprintf("release handoff latest batch plan missing: %s", handoff.LatestBatch.PlanPath))
	} else {
		if strings.TrimSpace(handoff.LatestBatch.Title) == "" {
			warnings = append(warnings, "release handoff latest batch title is empty")
		}
		if !handoff.LatestBatch.Handoff.Completed {
			warnings = append(warnings, fmt.Sprintf("release handoff latest batch is not completed: %s", handoff.LatestBatch.Status))
		}
		if strings.TrimSpace(handoff.LatestBatch.Goal) == "" {
			warnings = append(warnings, "release handoff latest batch goal is empty")
		}
		if strings.TrimSpace(handoff.LatestBatch.ValidationResult) == "" {
			warnings = append(warnings, "release handoff latest batch validation result is empty")
		}
	}
	if !handoff.ReleaseNotes.Present {
		warnings = append(warnings, fmt.Sprintf("release handoff release notes missing: %s", handoff.ReleaseNotes.Path))
	} else if strings.TrimSpace(handoff.ReleaseNotes.LatestBatchID) == "" {
		warnings = append(warnings, "release handoff release notes latest batch id is empty")
	} else if !handoff.ReleaseNotes.Covered {
		warnings = append(warnings, fmt.Sprintf("release handoff release notes missing latest batch: %s", handoff.ReleaseNotes.LatestBatchID))
	}
	if ReleaseHandoffCountsFor(handoff).Validation == 0 {
		warnings = append(warnings, "release handoff validation command list is empty")
	}
	warnings = append(warnings, handoff.PackMemoryCandidates.Warnings...)
	for _, signal := range handoff.Signals {
		if !signal.Ready {
			warnings = append(warnings, fmt.Sprintf("release handoff signal not ready: %s", signal.Name))
		}
	}
	return warnings
}

func latestBatchSummary(repo string) ReleaseHandoffLatestBatch {
	const planPath = "docs/batch-plan.md"
	latest := ReleaseHandoffLatestBatch{PlanPath: planPath}
	data, err := os.ReadFile(filepath.Join(repo, filepath.FromSlash(planPath)))
	if err != nil {
		return latest
	}
	latest.Present = true
	lines := strings.Split(strings.ReplaceAll(string(data), "\r\n", "\n"), "\n")
	start, title, batchID := latestBatchSummarySelection(lines)
	if start < 0 {
		return latest
	}
	latest.Title = title
	latest.BatchID = batchID
	end := len(lines)
	for i := start + 1; i < len(lines); i++ {
		if strings.HasPrefix(strings.TrimSpace(lines[i]), "### ") {
			end = i
			break
		}
	}
	sectionLines := lines[start+1 : end]
	operationalLines := sectionLines
	for i, line := range sectionLines {
		if _, ok := markdownFieldValue(strings.TrimSpace(line), "上一批摘要"); ok {
			operationalLines = sectionLines[:i]
			break
		}
	}
	handoffFields := []string{}
	for _, line := range operationalLines {
		trimmed := strings.TrimSpace(line)
		if value, ok := markdownFieldValue(trimmed, "状态"); ok {
			latest.Status = compactHandoffText(value, 160)
			handoffFields = append(handoffFields, value)
		}
		if value, ok := markdownFieldValue(trimmed, "目标"); ok {
			latest.Goal = compactHandoffText(value, 240)
		}
		if value, ok := markdownFieldValue(trimmed, "验证结果"); ok {
			latest.ValidationResult = compactHandoffText(value, 240)
			handoffFields = append(handoffFields, value)
		}
	}
	latest.Handoff = latestBatchHandoff(latest, strings.Join(handoffFields, "\n"))
	return latest
}

func latestBatchSummarySelection(lines []string) (int, string, string) {
	start := -1
	var title string
	var batchID string
	for i, line := range lines {
		trimmed := strings.TrimSpace(line)
		if !strings.HasPrefix(trimmed, "### Batch ") {
			continue
		}
		candidateTitle := strings.TrimSpace(strings.TrimPrefix(trimmed, "### "))
		candidateBatchID := batchIDFromTitle(candidateTitle)
		if candidateBatchID == "" {
			continue
		}
		if start < 0 || latestBatchIDGreater(candidateBatchID, batchID) {
			start = i
			title = candidateTitle
			batchID = candidateBatchID
		}
	}
	return start, title, batchID
}

func latestBatchIDGreater(candidate, current string) bool {
	if current == "" {
		return true
	}
	return latestBatchIDNumber(candidate) > latestBatchIDNumber(current)
}

func latestBatchIDNumber(batchID string) int {
	batchID = strings.TrimSpace(batchID)
	if !strings.HasPrefix(batchID, "Batch ") {
		return -1
	}
	var value int
	if _, err := fmt.Sscanf(batchID, "Batch %d", &value); err != nil {
		return -1
	}
	return value
}

func latestBatchHandoff(latest ReleaseHandoffLatestBatch, section string) ReleaseHandoffLatestBatchHandoff {
	evidenceSection := latestBatchEvidenceSection(latest.BatchID, section)
	handoff := ReleaseHandoffLatestBatchHandoff{
		Completed:               strings.Contains(latest.Status, "已完成"),
		LocalValidationReady:    latestBatchHasLocalValidation(evidenceSection),
		ReleaseCheckReady:       latestBatchReleaseCheckReady(evidenceSection),
		RemoteReleaseGate:       latestBatchRemoteReleaseGate(evidenceSection),
		RemoteReleaseGateDetail: latestBatchRemoteReleaseGateDetail(evidenceSection),
		CommitRefs:              latestBatchCommitRefs(evidenceSection),
		Evidence:                latestBatchEvidence(evidenceSection),
		ValidationWarnings:      latestBatchValidationWarnings(evidenceSection),
	}
	handoff.ReleaseInspectionCadence = latestBatchReleaseInspectionCadence(evidenceSection, handoff)
	handoff.NextAction = latestBatchNextAction(handoff)
	return handoff
}

func latestBatchEvidenceSection(batchID, section string) string {
	batchID = strings.TrimSpace(batchID)
	if batchID == "" {
		return section
	}
	currentBatch := latestBatchIDNumber(batchID)
	if currentBatch < 0 {
		return section
	}
	clauses := []string{}
	for _, clause := range latestBatchEvidenceClauses(section) {
		if latestBatchClauseReferencesOtherBatch(clause, currentBatch) {
			continue
		}
		clauses = append(clauses, clause)
	}
	return strings.Join(clauses, "\n")
}

func latestBatchClauseReferencesOtherBatch(clause string, currentBatch int) bool {
	for _, token := range strings.FieldsFunc(clause, func(r rune) bool {
		return r < '0' || r > '9'
	}) {
		value := 0
		if _, err := fmt.Sscanf(token, "%d", &value); err != nil {
			continue
		}
		if value == currentBatch {
			continue
		}
		idx := strings.Index(clause, token)
		if idx < 0 {
			continue
		}
		prefixStart := max(0, idx-16)
		prefix := strings.ToLower(clause[prefixStart:idx])
		if strings.Contains(prefix, "batch ") || strings.Contains(prefix, "batch") {
			return true
		}
	}
	return false
}

func latestBatchReleaseCheckReady(text string) bool {
	lower := strings.ToLower(text)
	return strings.Contains(lower, "release-check ready=true") ||
		(strings.Contains(lower, "release-check -format json") && strings.Contains(lower, "ready=true")) ||
		latestBatchHasReleaseRunSuccess(lower)
}

func latestBatchHasLocalValidation(text string) bool {
	lower := strings.ToLower(text)
	for _, pending := range []string{"完整本地 release minimum 待", "本地 release minimum 待", "完整本机 release minimum 待", "本机 release minimum 待", "local release minimum pending", "full local release minimum pending"} {
		if strings.Contains(lower, pending) {
			return false
		}
	}
	if latestBatchHasReleaseRunSuccess(lower) {
		return true
	}
	for _, command := range []string{
		"go run ./cmd/rekit -- -Command release-check -Format json",
		"go run ./cmd/rekit -- -Command status",
		"go run ./cmd/rekit -- -Command packs",
		"go run ./cmd/rekit -- -Command doctor",
		"go test ./...",
		"go vet ./...",
		"git diff --check",
	} {
		if !strings.Contains(lower, strings.ToLower(command)) {
			return false
		}
	}
	return true
}

func latestBatchHasReleaseRunSuccess(lower string) bool {
	if !strings.Contains(lower, "release-run") || !strings.Contains(lower, "ready=true") {
		return false
	}
	if strings.Contains(lower, "summary=release run ok") || strings.Contains(lower, "release run ok") {
		return true
	}
	return strings.Contains(lower, "passed=7") && strings.Contains(lower, "failed=0") && strings.Contains(lower, "skipped=0")
}

func latestBatchHasReleaseRunTransientRetry(lower string) bool {
	if !strings.Contains(lower, "release-run") {
		return false
	}
	return strings.Contains(lower, "transientretryreason") || strings.Contains(lower, "release-run step retry") || strings.Contains(lower, "attempts=2")
}

func latestBatchValidationWarnings(text string) []string {
	lower := strings.ToLower(text)
	warnings := []string{}
	if latestBatchHasReleaseRunTransientRetry(lower) {
		warnings = append(warnings, "release-run local validation passed only after a recorded transient retry; review retry reason and first-attempt output before release handoff")
	}
	return mission.UniqueStrings(warnings)
}

func latestBatchRemoteReleaseGate(text string) string {
	remoteText := latestBatchRemoteEvidenceText(text)
	if strings.TrimSpace(remoteText) == "" {
		return "not-recorded"
	}
	remoteLower := strings.ToLower(remoteText)
	emptySteps := latestBatchRemoteHasEmptySteps(remoteText, remoteLower)
	switch {
	case emptySteps && strings.Contains(remoteLower, "completed failure"):
		return "blocked: completed failure with jobs steps=[]"
	case emptySteps:
		return "blocked: jobs steps=[]"
	case latestBatchRemoteGreen(remoteText, remoteLower):
		return "green"
	case strings.Contains(remoteText, "远程 release-gate") || strings.Contains(remoteLower, "release-gate run") || strings.Contains(remoteLower, "workflow run") || strings.Contains(remoteLower, "pr run") || strings.Contains(remoteLower, "implementation run") || strings.Contains(remoteLower, "push run"):
		return "inspected"
	default:
		return "not-recorded"
	}
}

func latestBatchRemoteEvidenceText(text string) string {
	clauses := []string{}
	for _, clause := range latestBatchEvidenceClauses(text) {
		lower := strings.ToLower(clause)
		if latestBatchRemoteInspectionPending(clause, lower) {
			continue
		}
		if !latestBatchRemoteEvidenceClause(clause, lower) {
			continue
		}
		clauses = append(clauses, clause)
	}
	return strings.Join(clauses, "\n")
}

func latestBatchRemoteEvidenceClause(clause, lower string) bool {
	if latestBatchRemoteInspectionPending(clause, lower) {
		return false
	}
	if latestBatchRemoteGreen(clause, lower) {
		return true
	}
	remoteContext := strings.Contains(lower, "release-gate") || strings.Contains(lower, "remote") || strings.Contains(clause, "远程")
	jobContext := strings.Contains(lower, "job") || strings.Contains(lower, "jobs")
	completed := strings.Contains(lower, "completed") || strings.Contains(lower, "failure") || strings.Contains(lower, "success")
	runContext := strings.Contains(lower, "release-gate run") || strings.Contains(lower, "workflow run") || strings.Contains(lower, "pr run") || strings.Contains(lower, "implementation run") || strings.Contains(lower, "push run")
	if runContext {
		return len(latestBatchRemoteRunRefs(clause)) > 0 || jobContext || completed
	}
	if strings.Contains(clause, "远程 release-gate") && (strings.Contains(clause, "已检查") || strings.Contains(clause, "已记录")) {
		return true
	}
	return remoteContext && jobContext && completed
}

func latestBatchEvidenceClauses(text string) []string {
	text = strings.ReplaceAll(text, "\r\n", "\n")
	text = strings.ReplaceAll(text, "\r", "\n")
	text = strings.NewReplacer("。", "\n", "；", "\n", ";", "\n").Replace(text)
	clauses := []string{}
	for line := range strings.SplitSeq(text, "\n") {
		line = strings.TrimSpace(line)
		if line != "" {
			clauses = append(clauses, line)
		}
	}
	return clauses
}

func latestBatchRemoteInspectionPending(text, lower string) bool {
	for _, pending := range []string{
		"远程 release-gate inspection 待",
		"remote release-gate inspection pending",
		"release-gate inspection pending",
		"remote workflow run pending",
		"workflow run pending",
	} {
		if strings.Contains(text, pending) || strings.Contains(lower, pending) {
			return true
		}
	}
	pendingChinese := strings.Contains(text, "尚未检查") || strings.Contains(text, "尚未记录") || strings.Contains(text, "待检查")
	remoteRun := strings.Contains(text, "远程") && (strings.Contains(lower, "workflow run") || strings.Contains(lower, "release-gate run") || strings.Contains(text, "release-gate"))
	return pendingChinese && remoteRun
}

func latestBatchRemoteReleaseGateDetail(text string) *ReleaseHandoffRemoteReleaseGateDetail {
	remoteText := latestBatchRemoteEvidenceText(text)
	remoteLower := strings.ToLower(remoteText)
	state := latestBatchRemoteReleaseGate(text)
	detail := ReleaseHandoffRemoteReleaseGateDetail{
		State:         state,
		CanClaimGreen: state == "green",
	}
	if state != "not-recorded" {
		detail.RunRefs = latestBatchRemoteRunRefs(remoteText)
		detail.Jobs = latestBatchRemoteJobs(remoteLower)
		detail.EmptySteps = latestBatchRemoteHasEmptySteps(remoteText, remoteLower)
		detail.CompletedFailure = strings.Contains(remoteLower, "completed failure")
	}
	switch {
	case state == "green":
		detail.Boundary = []string{"remote CI green is claimable only because the latest batch explicitly records green jobs"}
	case strings.HasPrefix(state, "blocked:"):
		detail.Boundary = []string{"treat remote release-gate steps=[] as a known runner/billing blocker", "do not claim remote CI green", "continue only Windows-verifiable local product-path work"}
	case state == "not-recorded":
		detail.Boundary = []string{"inspect the remote release-gate run before claiming remote CI status", "release-check inventory ready is not remote CI green"}
	default:
		detail.Boundary = []string{"remote release-gate was inspected, but do not claim remote CI green without explicit green jobs"}
	}
	return &detail
}

func latestBatchRemoteHasEmptySteps(text, lower string) bool {
	compact := strings.NewReplacer(" ", "", "\t", "", "`", "").Replace(lower)
	return strings.Contains(compact, "steps:[]") || strings.Contains(compact, "steps=[]") || strings.Contains(text, "steps 为空") || strings.Contains(text, "steps为空")
}

func latestBatchReleaseInspectionCadence(text string, handoff ReleaseHandoffLatestBatchHandoff) ReleaseHandoffReleaseInspectionCadence {
	lower := strings.ToLower(text)
	cadence := ReleaseHandoffReleaseInspectionCadence{
		MaxPushes:                 2,
		ImplementationCommitReady: latestBatchImplementationCommitReady(text),
		InspectionCommitReady:     latestBatchInspectionCommitReady(text, handoff),
		NewRemoteSignal:           latestBatchHasNewRemoteSignal(lower, handoff),
		Boundary: []string{
			"normal batches stop after implementation commit/push plus one release inspection commit/push",
			"do not add a third record commit for the release inspection commit's own CI run",
			"only a remote signal different from the existing steps=[] runner/billing blocker may justify another inspection record",
		},
	}
	cadence.ThirdInspectionAllowed = cadence.NewRemoteSignal
	if cadence.ImplementationCommitReady {
		cadence.Evidence = append(cadence.Evidence, "implementation commit/push recorded")
	}
	if cadence.InspectionCommitReady {
		cadence.Evidence = append(cadence.Evidence, "release inspection commit/run recorded")
	}
	if handoff.RemoteReleaseGateDetail != nil && handoff.RemoteReleaseGateDetail.EmptySteps {
		cadence.Evidence = append(cadence.Evidence, "remote release-gate steps=[] blocker recorded")
	}
	if cadence.NewRemoteSignal {
		cadence.Evidence = append(cadence.Evidence, "new remote signal differs from existing steps=[] blocker")
	}
	switch {
	case !cadence.ImplementationCommitReady:
		cadence.State = "implementation-pending"
		cadence.NextAction = "create/push the implementation commit after local validation"
	case !cadence.InspectionCommitReady:
		cadence.State = "inspection-pending"
		cadence.NextAction = "inspect the implementation commit's remote release-gate run and record exactly one release inspection commit"
	case cadence.NewRemoteSignal:
		cadence.State = "new-remote-signal"
		cadence.NextAction = "review the new remote signal before deciding whether another inspection record is justified"
	default:
		cadence.State = "complete"
		cadence.NextAction = "do not create a third inspection record for the release inspection commit's own CI; continue the next batch"
	}
	return cadence
}

func latestBatchImplementationCommitReady(text string) bool {
	if latestBatchImplementationCommitEvidence(text) {
		return true
	}
	lower := strings.ToLower(text)
	for _, pending := range []string{"待 implementation commit/push", "implementation commit/push 待", "implementation commit/push 与远程 release-gate inspection 待", "pending implementation commit/push", "implementation commit/push and remote release-gate inspection pending", "after implementation commit/push", "尚未创建本批代码提交", "尚未提交推送"} {
		if strings.Contains(lower, pending) || strings.Contains(text, pending) {
			return false
		}
	}
	return false
}

func latestBatchImplementationCommitEvidence(text string) bool {
	for _, clause := range latestBatchEvidenceClauses(text) {
		clauseLower := strings.ToLower(clause)
		if strings.Contains(clauseLower, "do not") || strings.Contains(clauseLower, "不要") || strings.Contains(clauseLower, "不为") {
			continue
		}
		if strings.Contains(clause, "已推送") || strings.Contains(clause, "已提交并推送") || strings.Contains(clauseLower, "implementation commit/push recorded") || strings.Contains(clauseLower, "implementation commit `") || strings.Contains(clauseLower, "implementation commits `") {
			return true
		}
	}
	return false
}

func latestBatchInspectionCommitReady(_ string, handoff ReleaseHandoffLatestBatchHandoff) bool {
	return handoff.RemoteReleaseGate != "not-recorded"
}

func latestBatchHasNewRemoteSignal(lower string, handoff ReleaseHandoffLatestBatchHandoff) bool {
	if strings.Contains(lower, "new remote signal recorded") || strings.Contains(lower, "新远程信号已记录") || strings.Contains(lower, "新信号已记录") {
		return true
	}
	if handoff.RemoteReleaseGate == "green" {
		return true
	}
	if handoff.RemoteReleaseGateDetail == nil || handoff.RemoteReleaseGate == "not-recorded" {
		return false
	}
	return handoff.RemoteReleaseGateDetail.CompletedFailure && !handoff.RemoteReleaseGateDetail.EmptySteps
}

func latestBatchRemoteJobs(lower string) []string {
	jobs := []string{}
	for _, candidate := range []struct {
		match string
		name  string
	}{
		{match: "linux", name: "Linux"},
		{match: "windows", name: "Windows"},
		{match: "macos", name: "macOS"},
	} {
		if strings.Contains(lower, candidate.match) {
			jobs = append(jobs, candidate.name)
		}
	}
	return jobs
}

func latestBatchRemoteRunRefs(text string) []string {
	refs := []string{}
	seen := map[string]bool{}
	for {
		start := strings.Index(text, "`")
		if start < 0 {
			break
		}
		text = text[start+1:]
		end := strings.Index(text, "`")
		if end < 0 {
			break
		}
		token := strings.TrimSpace(text[:end])
		if looksLikeRunRef(token) && !seen[token] {
			seen[token] = true
			refs = append(refs, token)
		}
		text = text[end+1:]
	}
	return refs
}

func looksLikeRunRef(value string) bool {
	if len(value) < 6 || len(value) > 20 {
		return false
	}
	for _, r := range value {
		if r < '0' || r > '9' {
			return false
		}
	}
	return true
}

func latestBatchRemoteGreen(text, lower string) bool {
	if strings.Contains(text, "不能声明远程 CI green") || strings.Contains(lower, "不能声明 remote ci green") || strings.Contains(lower, "cannot claim remote ci green") || strings.Contains(lower, "not remote ci green") {
		return false
	}
	return strings.Contains(lower, "remote ci green") || strings.Contains(text, "远程 CI green")
}

func latestBatchCommitRefs(text string) []string {
	refs := []string{}
	seen := map[string]bool{}
	for _, clause := range latestBatchEvidenceClauses(text) {
		if !latestBatchCommitEvidenceClause(clause) {
			continue
		}
		for _, token := range backtickTokens(latestBatchCommitRefScope(clause)) {
			if looksLikeCommitRef(token) && !seen[token] {
				seen[token] = true
				refs = append(refs, token)
			}
		}
	}
	return refs
}

func latestBatchCommitRefScope(clause string) string {
	lower := strings.ToLower(clause)
	if start := latestBatchCommitMarkerIndex(lower); start >= 0 {
		clause = clause[start:]
		lower = lower[start:]
	}
	cutoff := len(clause)
	for _, marker := range []string{
		"pr #",
		"remote",
		"远程",
		"release-gate run",
		"workflow run",
		"pr run",
		"jobs",
		"job ",
	} {
		if idx := strings.Index(lower, marker); idx >= 0 && idx < cutoff {
			cutoff = idx
		}
	}
	return strings.TrimSpace(clause[:cutoff])
}

func latestBatchCommitMarkerIndex(lower string) int {
	best := -1
	for _, marker := range []string{
		"implementation commits",
		"implementation commit",
		"commits `",
		"commit `",
	} {
		if idx := strings.Index(lower, marker); idx >= 0 && (best < 0 || idx < best) {
			best = idx
		}
	}
	return best
}

func latestBatchCommitEvidenceClause(clause string) bool {
	lower := strings.ToLower(clause)
	if strings.Contains(lower, "do not") || strings.Contains(lower, "不要") || strings.Contains(lower, "不为") {
		return false
	}
	return strings.Contains(lower, "implementation commit") ||
		strings.Contains(lower, "implementation commits") ||
		strings.Contains(lower, "implementation commit/push recorded") ||
		strings.Contains(clause, "已提交并推送") ||
		strings.Contains(clause, "已推送")
}

func backtickTokens(text string) []string {
	tokens := []string{}
	for {
		start := strings.Index(text, "`")
		if start < 0 {
			break
		}
		text = text[start+1:]
		end := strings.Index(text, "`")
		if end < 0 {
			break
		}
		tokens = append(tokens, strings.TrimSpace(text[:end]))
		text = text[end+1:]
	}
	return tokens
}

func looksLikeCommitRef(value string) bool {
	if len(value) < 7 || len(value) > 40 {
		return false
	}
	for _, r := range value {
		switch {
		case r >= '0' && r <= '9':
		case r >= 'a' && r <= 'f':
		case r >= 'A' && r <= 'F':
		default:
			return false
		}
	}
	return true
}

func latestBatchEvidence(text string) []string {
	lower := strings.ToLower(text)
	remoteText := latestBatchRemoteEvidenceText(text)
	remoteLower := strings.ToLower(remoteText)
	evidence := []string{}
	for _, candidate := range []struct {
		match string
		label string
	}{
		{match: "public cli", label: "public CLI product-path validation recorded"},
		{match: "go run ./cmd/rekit -- -command release-check -format json", label: "release-check -Format json recorded"},
		{match: "releasecheck-step", label: "release-check -Format json recorded"},
		{match: "go run ./cmd/rekit -- -command status", label: "status handoff recorded"},
		{match: "status-step", label: "status handoff recorded"},
		{match: "go run ./cmd/rekit -- -command packs", label: "packs inventory recorded"},
		{match: "packs-step", label: "packs inventory recorded"},
		{match: "go run ./cmd/rekit -- -command doctor", label: "doctor validation recorded"},
		{match: "doctor-step", label: "doctor validation recorded"},
		{match: "go test ./...", label: "go test ./... recorded"},
		{match: "go vet ./...", label: "go vet ./... recorded"},
		{match: "git diff --check", label: "git diff --check recorded"},
		{match: "release-run", label: "release-run local release minimum recorded"},
		{match: "release-run-retry", label: "release-run transient retry recorded"},
		{match: "release-check ready=true", label: "release-check ready=true recorded"},
		{match: "release-run-ready", label: "release-check ready=true recorded"},
		{match: "steps: []", label: "remote release-gate jobs steps=[] recorded"},
	} {
		if !latestBatchEvidenceMatched(candidate.match, text, lower, remoteText, remoteLower) {
			continue
		}
		evidence = append(evidence, candidate.label)
	}
	return mission.UniqueStrings(evidence)
}

func latestBatchEvidenceMatched(match, text, lower, remoteText, remoteLower string) bool {
	switch match {
	case "steps: []":
		return latestBatchRemoteReleaseGate(text) != "not-recorded" && latestBatchRemoteHasEmptySteps(remoteText, remoteLower)
	case "release-run-ready":
		return latestBatchHasReleaseRunSuccess(lower)
	case "release-run-retry":
		return latestBatchHasReleaseRunTransientRetry(lower)
	case "releasecheck-step":
		return latestBatchHasReleaseRunSuccess(lower) && strings.Contains(lower, "release-check")
	case "status-step":
		return latestBatchHasReleaseRunSuccess(lower) && strings.Contains(lower, "status")
	case "packs-step":
		return latestBatchHasReleaseRunSuccess(lower) && strings.Contains(lower, "packs")
	case "doctor-step":
		return latestBatchHasReleaseRunSuccess(lower) && strings.Contains(lower, "doctor")
	default:
		return strings.Contains(lower, match)
	}
}

func latestBatchNextAction(handoff ReleaseHandoffLatestBatchHandoff) string {
	switch {
	case !handoff.Completed:
		return "finish the current batch before treating status as a handoff"
	case !handoff.LocalValidationReady:
		return "run the full local release minimum and update docs/batch-plan.md"
	case handoff.ReleaseInspectionCadence.State == "implementation-pending":
		return handoff.ReleaseInspectionCadence.NextAction
	case handoff.ReleaseInspectionCadence.State == "inspection-pending":
		return handoff.ReleaseInspectionCadence.NextAction
	case handoff.ReleaseInspectionCadence.State == "new-remote-signal":
		return handoff.ReleaseInspectionCadence.NextAction
	case handoff.RemoteReleaseGate == "not-recorded":
		return "inspect the remote release-gate run before claiming remote CI status"
	case strings.HasPrefix(handoff.RemoteReleaseGate, "blocked:"):
		return "select the next Windows-verifiable product-path batch from docs/context-routing.md and docs/batch-plan.md; do not create a third inspection record for the release inspection commit's own CI unless a new remote signal appears"
	default:
		return "select the next batch from docs/context-routing.md and docs/batch-plan.md"
	}
}

func latestReleaseNotes(repo string, latest ReleaseHandoffLatestBatch) ReleaseHandoffReleaseNotes {
	const changelogPath = "CHANGELOG.md"
	notes := ReleaseHandoffReleaseNotes{
		Path:          changelogPath,
		Section:       "Unreleased",
		LatestBatchID: latest.BatchID,
		Summary:       "release notes freshness has warnings",
	}
	data, err := os.ReadFile(filepath.Join(repo, filepath.FromSlash(changelogPath)))
	if err != nil {
		return notes
	}
	notes.Present = true
	section := markdownSectionText(string(data), "## Unreleased")
	if strings.TrimSpace(section) == "" {
		section = string(data)
	}
	if strings.TrimSpace(notes.LatestBatchID) != "" {
		notes.Covered = strings.Contains(section, notes.LatestBatchID)
	}
	if notes.Covered {
		notes.Summary = "release notes cover latest batch"
	}
	return notes
}

func markdownSectionText(text, heading string) string {
	lines := strings.Split(strings.ReplaceAll(text, "\r\n", "\n"), "\n")
	inSection := false
	section := []string{}
	for _, line := range lines {
		trimmed := strings.TrimSpace(line)
		if trimmed == heading {
			inSection = true
			continue
		}
		if inSection && strings.HasPrefix(trimmed, "## ") {
			break
		}
		if inSection {
			section = append(section, line)
		}
	}
	return strings.Join(section, "\n")
}

func batchIDFromTitle(title string) string {
	title = strings.TrimSpace(title)
	rest, ok := strings.CutPrefix(title, "Batch")
	if !ok {
		return ""
	}
	rest = strings.TrimSpace(rest)
	if rest == "" {
		return ""
	}
	for i, r := range rest {
		if r == '：' || r == ':' || r == ' ' || r == '\t' {
			if i == 0 {
				return ""
			}
			return "Batch " + rest[:i]
		}
	}
	return "Batch " + rest
}

func markdownFieldValue(line, key string) (string, bool) {
	prefixes := []string{key + "：", key + ":"}
	for _, prefix := range prefixes {
		if value, ok := strings.CutPrefix(line, prefix); ok {
			return strings.TrimSpace(value), true
		}
	}
	return "", false
}

func compactHandoffText(text string, maxRunes int) string {
	text = strings.Join(strings.Fields(strings.TrimSpace(text)), " ")
	if maxRunes <= 0 {
		return text
	}
	runes := []rune(text)
	if len(runes) <= maxRunes {
		return text
	}
	return string(runes[:maxRunes-1]) + "…"
}
