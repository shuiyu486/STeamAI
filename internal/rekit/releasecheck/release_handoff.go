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
	"github.com/shuiyu486/re-context-kits/internal/rekit/projectstate"
	"github.com/shuiyu486/re-context-kits/internal/rekit/promote"
	syncpkg "github.com/shuiyu486/re-context-kits/internal/rekit/sync"
)

type ReleaseHandoff struct {
	Ready                     bool                                     `json:"ready"`
	Summary                   string                                   `json:"summary"`
	ReadFirst                 []ReleaseHandoffDocument                 `json:"readFirst"`
	Signals                   []ReleaseHandoffSignal                   `json:"signals"`
	ActiveRoute               ReleaseHandoffActiveRoute                `json:"activeRoute"`
	LatestBatch               ReleaseHandoffLatestBatch                `json:"latestBatch"`
	ReleaseNotes              ReleaseHandoffReleaseNotes               `json:"releaseNotes"`
	KnownGaps                 []ReleaseHandoffKnownGap                 `json:"knownGaps"`
	PackMaturity              ReleaseHandoffPackMaturity               `json:"packMaturity"`
	PackMemoryCandidates      ReleaseHandoffPackMemoryCandidateList    `json:"packMemoryCandidates"`
	NextBatchSelectionPackage *ReleaseHandoffNextBatchSelectionPackage `json:"nextBatchSelectionPackage,omitempty"`
	Validation                []ReleaseHandoffValidation               `json:"validation"`
	NextActions               []string                                 `json:"nextActions"`
	Warnings                  []string                                 `json:"warnings"`
}

type ReleaseHandoffActiveRoute struct {
	Ready                  bool                                    `json:"ready"`
	Present                bool                                    `json:"present"`
	Path                   string                                  `json:"path"`
	ProjectionPath         string                                  `json:"projectionPath"`
	Route                  string                                  `json:"route"`
	CurrentBatch           string                                  `json:"currentBatch"`
	State                  string                                  `json:"state"`
	ExclusiveClaim         string                                  `json:"exclusiveClaim"`
	NextBatch              string                                  `json:"nextBatch"`
	NextBatchUnlocked      bool                                    `json:"nextBatchUnlocked"`
	ProjectionConsistent   bool                                    `json:"projectionConsistent"`
	LocalValidationReady   bool                                    `json:"localValidationReady"`
	ReleaseCheckReady      bool                                    `json:"releaseCheckReady"`
	LocalValidationReceipt *LocalValidationReceiptInspection       `json:"localValidationReceipt,omitempty"`
	PostPushReceipt        *ReleaseHandoffPostPushReceipt          `json:"postPushReceipt,omitempty"`
	CommitRefs             []string                                `json:"commitRefs,omitempty"`
	Evidence               []string                                `json:"evidence,omitempty"`
	CurrentAction          *mission.MissionCommanderNextActionItem `json:"currentAction,omitempty"`
	Warnings               []string                                `json:"warnings,omitempty"`
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

type ReleaseHandoffNextBatchSelectionPackage struct {
	Ready                       bool                                     `json:"ready"`
	Summary                     string                                   `json:"summary"`
	StarterPackage              *ReleaseHandoffNextBatchStarterPackage   `json:"starterPackage,omitempty"`
	MissionCommanderNextActions []mission.MissionCommanderNextActionItem `json:"missionCommanderNextActions,omitempty"`
	MissionCommanderActionQueue mission.MissionCommanderActionQueue      `json:"missionCommanderActionQueue"`
	NextBatchPlanningRoutes     []ReleaseHandoffNextBatchPlanningRoute   `json:"nextBatchPlanningRoutes,omitempty"`
	Boundary                    []string                                 `json:"boundary,omitempty"`
}

type ReleaseHandoffNextBatchPlanningRoute struct {
	Ready                   bool     `json:"ready"`
	Domain                  string   `json:"domain"`
	DomainActionID          string   `json:"domainActionId"`
	ClosurePlaceholder      string   `json:"closurePlaceholder"`
	WhatIfCommandTemplate   string   `json:"whatIfCommandTemplate"`
	CommandExecutable       bool     `json:"commandExecutable"`
	RequiresReview          bool     `json:"requiresReview"`
	RefreshStatusCommand    string   `json:"refreshStatusCommand,omitempty"`
	ExpectedApplySource     string   `json:"expectedApplySource,omitempty"`
	ExpectedApplyDriverKind string   `json:"expectedApplyDriverKind,omitempty"`
	RunbookSteps            []string `json:"runbookSteps,omitempty"`
	Boundary                []string `json:"boundary,omitempty"`
}

type ReleaseHandoffNextBatchStarterPackage struct {
	Ready                   bool                                  `json:"ready"`
	LatestCompletedBatch    string                                `json:"latestCompletedBatch,omitempty"`
	SuggestedNextBatch      string                                `json:"suggestedNextBatch,omitempty"`
	CurrentBatchSection     string                                `json:"currentBatchSection"`
	ChangelogEntry          string                                `json:"changelogEntry"`
	ValidationCommands      []string                              `json:"validationCommands,omitempty"`
	ReleaseCadenceSteps     []string                              `json:"releaseCadenceSteps,omitempty"`
	RecommendedStarterSteps []string                              `json:"recommendedStarterSteps,omitempty"`
	Boundary                []string                              `json:"boundary,omitempty"`
	CurrentRunLoopStepID    string                                `json:"currentRunLoopStepId,omitempty"`
	RunLoop                 []mission.MissionCommanderRunLoopStep `json:"runLoop,omitempty"`
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
	Stage                     string                                                     `json:"stage,omitempty"`
	ProofType                 string                                                     `json:"proofType,omitempty"`
	Path                      string                                                     `json:"path,omitempty"`
	CandidatePath             string                                                     `json:"candidatePath,omitempty"`
	PackTarget                string                                                     `json:"packTarget,omitempty"`
	SourceCaseRoot            string                                                     `json:"sourceCaseRoot,omitempty"`
	When                      string                                                     `json:"when,omitempty"`
	Action                    string                                                     `json:"action,omitempty"`
	Format                    string                                                     `json:"format,omitempty"`
	PacketPath                string                                                     `json:"packetPath,omitempty"`
	CandidateDecisionPath     string                                                     `json:"candidateDecisionPath,omitempty"`
	EvidenceRefs              []string                                                   `json:"evidenceRefs,omitempty"`
	DraftCommand              string                                                     `json:"draftCommand,omitempty"`
	DraftApplyTemplate        string                                                     `json:"draftApplyTemplate,omitempty"`
	CurrentRunLoopStepID      string                                                     `json:"currentRunLoopStepId,omitempty"`
	RunLoop                   []mission.MissionCommanderRunLoopStep                      `json:"runLoop,omitempty"`
	CurrentDriverRequest      *mission.MissionCommanderDriverRequest                     `json:"currentDriverRequest,omitempty"`
	ReconsumeOperator         *ReleaseHandoffPackMemoryCandidateReconsumeOperatorPackage `json:"reconsumeOperator,omitempty"`
	RequiresPacket            bool                                                       `json:"requiresPacket,omitempty"`
	RequiresCandidateDecision bool                                                       `json:"requiresCandidateDecision,omitempty"`
	RequiresExplicitReview    bool                                                       `json:"requiresExplicitReview,omitempty"`
	Evidence                  []string                                                   `json:"evidence,omitempty"`
	Boundary                  []string                                                   `json:"boundary,omitempty"`
}

type ReleaseHandoffPackMemoryCandidateReconsumeOperatorPackage struct {
	SchemaVersion           int                                    `json:"schemaVersion"`
	Kind                    string                                 `json:"kind"`
	OperatorID              string                                 `json:"operatorId"`
	OperatorSnapshotSHA256  string                                 `json:"operatorSnapshotSha256"`
	Pack                    string                                 `json:"pack"`
	SourceCaseRoot          string                                 `json:"sourceCaseRoot"`
	PacketPath              string                                 `json:"packetPath"`
	PacketSHA256            string                                 `json:"packetSha256"`
	CandidateDecisionPath   string                                 `json:"candidateDecisionPath"`
	CandidateDecisionSHA256 string                                 `json:"candidateDecisionSha256"`
	DecisionReceiptPath     string                                 `json:"decisionReceiptPath"`
	DecisionReceiptSHA256   string                                 `json:"decisionReceiptSha256"`
	CandidatePath           string                                 `json:"candidatePath"`
	PackTarget              string                                 `json:"packTarget,omitempty"`
	VerificationState       string                                 `json:"verificationState"`
	VerificationProofPath   string                                 `json:"verificationProofPath,omitempty"`
	EvidenceRefs            []string                               `json:"evidenceRefs,omitempty"`
	CurrentRunLoopStepID    string                                 `json:"currentRunLoopStepId"`
	RunLoop                 []mission.MissionCommanderRunLoopStep  `json:"runLoop"`
	CurrentDriverRequest    *mission.MissionCommanderDriverRequest `json:"currentDriverRequest,omitempty"`
	Boundary                []string                               `json:"boundary"`
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
	Pack                   string                                                     `json:"pack"`
	Maturity               string                                                     `json:"maturity"`
	CandidateRoot          string                                                     `json:"candidateRoot"`
	ToolingRoot            string                                                     `json:"toolingRoot"`
	IndexPath              string                                                     `json:"indexPath,omitempty"`
	CandidateFiles         int                                                        `json:"candidateFiles"`
	ToolingFiles           int                                                        `json:"toolingFiles"`
	IndexEntries           int                                                        `json:"indexEntries"`
	CandidatePaths         []string                                                   `json:"candidatePaths,omitempty"`
	ToolingPaths           []string                                                   `json:"toolingPaths,omitempty"`
	IndexCandidates        []ReleaseHandoffPackMemoryCandidateIndexEntry              `json:"indexCandidates,omitempty"`
	ReviewArtifacts        []ReleaseHandoffPackMemoryCandidateReviewArtifact          `json:"reviewArtifacts,omitempty"`
	ReviewSummary          ReleaseHandoffPackMemoryCandidateReviewSummary             `json:"reviewSummary"`
	ProofSummary           ReleaseHandoffPackMemoryCandidateReviewProofSummary        `json:"proofSummary"`
	DecisionReceipts       []ReleaseHandoffPackMemoryCandidateDecisionReceipt         `json:"decisionReceipts,omitempty"`
	DecisionDraftHandoff   *promote.CandidateDecisionDraftHandoff                     `json:"decisionDraftHandoff,omitempty"`
	ReconsumeOperator      *ReleaseHandoffPackMemoryCandidateReconsumeOperatorPackage `json:"reconsumeOperator,omitempty"`
	PendingVerifications   int                                                        `json:"pendingVerifications"`
	CompletedVerifications int                                                        `json:"completedVerifications"`
	ProofRoot              string                                                     `json:"proofRoot,omitempty"`
	HasOpenWork            bool                                                       `json:"hasOpenWork"`
	RequiresReview         bool                                                       `json:"requiresReview"`
	RequiresCleanup        bool                                                       `json:"requiresCleanup"`
	RequiresVerification   bool                                                       `json:"requiresVerification"`
	Action                 string                                                     `json:"action,omitempty"`
	Evidence               []string                                                   `json:"evidence,omitempty"`
	Boundary               []string                                                   `json:"boundary,omitempty"`

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
	actionsHash    string
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
	EvidenceRefs          []string `json:"evidenceRefs,omitempty"`
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
	{Path: "docs/context-routing.md", Purpose: "canonical progressive-disclosure router; select the current scenario before reading any other document"},
}

func BuildProjectHandoff(repoRoot string) (ReleaseHandoff, error) {
	repo, err := filepath.Abs(repoRoot)
	if err != nil {
		return ReleaseHandoff{}, err
	}
	cat, err := loadCatalog(repo)
	if err != nil {
		return ReleaseHandoff{}, err
	}
	packs, err := manifest.List(repo)
	if err != nil {
		return ReleaseHandoff{}, err
	}
	actions := heavyToolGateActions(packs)
	activeRoute := releaseHandoffActiveRoute(repo)
	latest, activeRoute := releaseHandoffWithValidationReceipt(repo, latestBatchSummary(repo), activeRoute)
	handoff := ReleaseHandoff{
		Ready:       true,
		Summary:     "release handoff summary ok",
		ReadFirst:   releaseHandoffDocuments(repo),
		ActiveRoute: activeRoute,
		LatestBatch: latest,
		Validation:  releaseHandoffValidation(gateProfile(catalogGateSteps(repo, cat.RecommendedMinimum)).Steps),
		NextActions: releaseHandoffNextActions(),
		Warnings:    []string{},
	}
	handoff.ReleaseNotes = latestReleaseNotes(repo, handoff.LatestBatch)
	handoff.KnownGaps = releaseHandoffKnownGaps(knownGaps(repo))
	handoff.PackMaturity = releaseHandoffPackMaturity(packs, actions)
	handoff.PackMemoryCandidates = releaseHandoffPackMemoryCandidates(repo, packs)
	handoff.Warnings = releaseHandoffWarnings(handoff)
	if len(handoff.Warnings) > 0 {
		handoff.Ready = false
		handoff.Summary = "release handoff summary has warnings"
	}
	if handoff.ActiveRoute.Present {
		handoff.LatestBatch.Handoff.NextAction = ""
		handoff.LatestBatch.Handoff.ReleaseInspectionCadence.NextAction = ""
	}
	if handoff.ActiveRoute.CurrentAction == nil || handoff.ActiveRoute.Ready && handoff.ActiveRoute.ProjectionConsistent {
		handoff.ActiveRoute.CurrentAction = releaseHandoffFinalActiveRouteAction(handoff.ActiveRoute)
	}
	handoff.NextBatchSelectionPackage = BuildNextBatchSelectionPackage(handoff)
	return handoff, nil
}

func releaseHandoff(repo string, check Result) ReleaseHandoff {
	activeRoute := releaseHandoffActiveRoute(repo)
	latest, activeRoute := releaseHandoffWithValidationReceipt(repo, latestBatchSummary(repo), activeRoute)
	handoff := ReleaseHandoff{
		Ready:       true,
		Summary:     "release handoff summary ok",
		ReadFirst:   releaseHandoffDocuments(repo),
		ActiveRoute: activeRoute,
		LatestBatch: latest,
		Validation:  releaseHandoffValidation(check.GateProfile.Steps),
		NextActions: releaseHandoffNextActions(),
		Warnings:    []string{},
	}
	handoff.ReleaseNotes = latestReleaseNotes(repo, handoff.LatestBatch)
	handoff.KnownGaps = releaseHandoffKnownGaps(check.KnownGaps)
	handoff.PackMaturity = releaseHandoffPackMaturity(check.Packs, check.HeavyToolGateActions)
	handoff.PackMemoryCandidates = releaseHandoffPackMemoryCandidates(repo, check.Packs)
	handoff.LatestBatch = releaseHandoffLatestBatchWithCurrentInventory(handoff.LatestBatch, check)
	handoff.Signals = releaseHandoffSignals(check, handoff.LatestBatch, handoff.ReleaseNotes, handoff.KnownGaps, handoff.PackMaturity, handoff.PackMemoryCandidates)
	handoff.Warnings = releaseHandoffWarnings(handoff)
	if ReleaseHandoffCountsFor(handoff).Warnings > 0 {
		handoff.Ready = false
		handoff.Summary = "release handoff summary has warnings"
	}
	if handoff.ActiveRoute.Present {
		handoff.LatestBatch.Handoff.NextAction = ""
		handoff.LatestBatch.Handoff.ReleaseInspectionCadence.NextAction = ""
	}
	if handoff.ActiveRoute.CurrentAction == nil || handoff.ActiveRoute.Ready && handoff.ActiveRoute.ProjectionConsistent {
		handoff.ActiveRoute.CurrentAction = releaseHandoffFinalActiveRouteAction(handoff.ActiveRoute)
	}
	handoff.NextBatchSelectionPackage = BuildNextBatchSelectionPackage(handoff)
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
				fmt.Sprintf("model=%s entrypoint=%s stateRoot=%s defaultForNewProjects=%t", check.CaseShim.Model, check.CaseShim.CompatibilityEntrypoint, check.CaseShim.StateRoot, check.CaseShim.DefaultForNewProjects),
				fmt.Sprintf("template=%s", check.CaseShim.TemplatePath),
				fmt.Sprintf("requiredPhrases=%d canonicalPhrases=%d forbidden=%d", caseShimCounts.RequiredPhrases, caseShimCounts.CanonicalSkillPhrases, caseShimCounts.ForbiddenStrings),
				"legacy /rekit case-local shim stays release-blocking but is not the default UX for new STeamAI projects",
			},
		},
		{
			Name:    "public default docs",
			Ready:   check.PublicDefaultDocs.Ready,
			Summary: check.PublicDefaultDocs.Summary,
			Details: []string{
				fmt.Sprintf("model=%s entrypoint=%s stateRoot=%s runtimeSource=%s fallbackAllowed=%t", check.PublicDefaultDocs.Model, check.PublicDefaultDocs.DefaultEntrypoint, check.PublicDefaultDocs.StateRoot, check.PublicDefaultDocs.RuntimeSource, check.PublicDefaultDocs.FallbackAllowed),
				fmt.Sprintf("canonicalRepository=%s canonicalCloneUrl=%s moduleCompatibilityIdentity=%s", check.PublicDefaultDocs.CanonicalRepository, check.PublicDefaultDocs.CanonicalCloneURL, check.PublicDefaultDocs.ModuleCompatibilityIdentity),
				fmt.Sprintf("documents=%d requiredPhrases=%d forbiddenCommands=%d forbiddenShellFences=%d", publicDefaultDocCounts.Documents, publicDefaultDocCounts.RequiredPhrases, publicDefaultDocCounts.ForbiddenCommands, publicDefaultDocCounts.ForbiddenShellFences),
				"README, canonical /steamai skill, project template, router, current route, repository handoff guidance, product direction, and self-contained contract keep project-local no-fallback defaults",
				"legacy /rekit and .rekit remain compatibility surfaces rather than new-project defaults",
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
		if openUnits == 0 && status.ProofSummary.Missing > 0 {
			openUnits = status.ProofSummary.Missing
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
		if command := packMemoryCandidateCurrentProofCommand(*next); command != "" {
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

func packMemoryCandidateCurrentProofCommand(next ReleaseHandoffPackMemoryCandidateReviewNextMissingProof) string {
	current := strings.TrimSpace(next.CurrentRunLoopStepID)
	for _, step := range next.RunLoop {
		if step.StepID == current {
			if command := strings.TrimSpace(step.Command); command != "" {
				return command
			}
		}
	}
	if current == "bind-review-packet" {
		return packMemoryCandidateNextMissingProofBindCommand(next)
	}
	return strings.TrimSpace(next.DraftCommand)
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
	receiptReviewArtifacts, err := packMemoryCandidateDecisionCleanupArtifacts(status, proofRoot)
	if err != nil {
		return ReleaseHandoffPackMemoryCandidateStatus{}, fmt.Errorf("pack-memory candidate post-decision proof scan failed for %s: %w", pack.ID, err)
	}
	receiptProofMissing := 0
	receiptCleanupMissing := 0
	for _, artifact := range receiptReviewArtifacts {
		if artifact.ProofPresent {
			continue
		}
		receiptProofMissing++
		if artifact.Name == "candidate-cleanup-proof" {
			receiptCleanupMissing++
		}
	}
	status.HasOpenWork = candidateFileCount > 0 || toolingFileCount > 0 || indexEntryCount > 0 || indexExists || pendingVerifications > 0 || receiptProofMissing > 0
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
	status.ReviewArtifacts = append(status.ReviewArtifacts, receiptReviewArtifacts...)
	status.DecisionDraftHandoff = packMemoryCandidateDecisionDraftHandoff(status)
	status.ProofSummary = packMemoryCandidateReviewProofSummary(status)
	status.ReviewSummary = packMemoryCandidateReviewSummary(status)
	status.ReconsumeOperator = packMemoryCandidateReconsumeOperator(status)
	if status.ProofSummary.NextMissingProof != nil && status.ReconsumeOperator != nil && status.ProofSummary.NextMissingProof.Stage == "reconsume-proof-required" {
		status.ProofSummary.NextMissingProof.ReconsumeOperator = status.ReconsumeOperator
		status.ReviewSummary.ProofSummary = status.ProofSummary
	}
	status.Boundary = []string{
		"release handoff inventories candidate residue and durable decision verification receipts; it does not merge, delete, or validate cases",
		"review candidates before merge and explicitly verify accepted decisions; do not write authority/confirmed",
		"do not promote case-specific artifacts, traces, dumps, captures, payloads, flags, or customer data",
	}
	return status, nil
}

func packMemoryCandidateReconsumeOperator(status ReleaseHandoffPackMemoryCandidateStatus) *ReleaseHandoffPackMemoryCandidateReconsumeOperatorPackage {
	next := status.ProofSummary.NextMissingProof
	if next == nil || next.Stage != "reconsume-proof-required" {
		return nil
	}
	var receipt *ReleaseHandoffPackMemoryCandidateDecisionReceipt
	for i := range status.DecisionReceipts {
		candidate := &status.DecisionReceipts[i]
		for _, action := range candidate.Actions {
			if action.Decision == "accept" && action.CandidatePath == next.CandidatePath {
				receipt = candidate
				break
			}
		}
		if receipt != nil {
			break
		}
	}
	if receipt == nil {
		return nil
	}
	verificationState := "provision-required"
	currentStep := "provision-verification-cases"
	command := receipt.VerificationProvisionCommand
	switch {
	case receipt.RetirementInProgress:
		verificationState = "retirement-in-progress"
		currentStep = "resume-verification-retirement"
		command = receipt.RetirementNextAction
	case receipt.RetirementRequired:
		verificationState = "retirement-required"
		currentStep = "preview-verification-retirement"
		command = receipt.RetirementPreviewCommand
	case receipt.VerificationComplete:
		verificationState = "verified"
		currentStep = "draft-lifecycle-proof"
		command = next.DraftCommand
	case receipt.ProvisionInProgress:
		verificationState = "provision-in-progress"
		currentStep = "resume-verification-provision"
		command = receipt.ProvisionApplyCommand
	case receipt.ProvisionComplete:
		verificationState = "verification-required"
		currentStep = "verify-candidate-consumers"
		command = receipt.VerificationCommand
	}
	if strings.TrimSpace(command) == "" {
		command = packMemoryCandidateNextMissingProofStatusCommand(*next)
	}
	boundary := mission.UniqueStrings(append(append([]string{}, next.Boundary...),
		"reconsume operator is a typed read-only handoff over existing decision, provision, verification, lifecycle proof, and retirement primitives",
		"execute only currentDriverRequest; the operator never runs init, doctor, sync, promote, verification, or retirement automatically",
		"every mutation remains an explicit WhatIf or expected-hash Apply followed by status refresh",
		"canonical verification proof is the only repo-local lifecycle evidence source; do not copy raw case output or absolute case paths into kit proof",
	))
	steps := []mission.MissionCommanderRunLoopStep{
		{Order: 1, StepID: "provision-verification-cases", Actor: "main-agent", Description: "preview and explicitly provision source-case-local fresh and attached verification consumers", Command: receipt.VerificationProvisionCommand, State: "pack-memory-reconsume-required", Source: "packMemoryCandidateReconsumeOperator.provision", Boundary: boundary},
		{Order: 2, StepID: "verify-candidate-consumers", Actor: "main-agent", Description: "verify pack doctor plus fresh and attached candidate reconsume and write canonical verification proof", Command: receipt.VerificationCommand, State: "pack-memory-reconsume-required", Source: "packMemoryCandidateReconsumeOperator.verify", Boundary: boundary},
		{Order: 3, StepID: "draft-lifecycle-proof", Actor: "main-agent", Description: "draft the next lifecycle proof with canonical verification proof evidence", Command: next.DraftCommand, State: "pack-memory-reconsume-required", Source: "packMemoryCandidateReconsumeOperator.proof", Boundary: boundary},
		{Order: 4, StepID: "refresh-pack-memory-status", Actor: "main-agent", Description: "refresh status and select the next missing lifecycle proof or retirement step", Command: packMemoryCandidateNextMissingProofStatusCommand(*next), State: "pack-memory-reconsume-required", Source: "packMemoryCandidateReconsumeOperator.refresh", Boundary: boundary},
		{Order: 5, StepID: "preview-verification-retirement", Actor: "main-agent", Description: "preview verification workspace retirement after all lifecycle proofs are present", Command: receipt.RetirementPreviewCommand, State: "pack-memory-reconsume-required", Source: "packMemoryCandidateReconsumeOperator.retirement", Boundary: boundary},
	}
	actionID := "pack-memory-reconsume-operator:" + shortReleaseHandoffHash(receipt.receiptHash+next.ProofType+next.CandidatePath)
	action := mission.MissionCommanderNextActionItem{Label: status.Pack, ActionID: actionID, State: "pack-memory-reconsume-required", Command: command, Source: "packMemoryCandidateReconsumeOperator", RequiresReview: true, Reasons: []string{"verificationState=" + verificationState, "proofType=" + next.ProofType, "candidate=" + next.CandidatePath}, Boundary: boundary}
	request := mission.MissionCommanderCurrentDriverRequest(action, currentStep, steps)
	if request != nil {
		refreshed := mission.MissionCommanderDriverRequestWithRefreshStatusCommand(*request, packMemoryCandidateNextMissingProofStatusCommand(*next))
		request = &refreshed
	}
	pkg := &ReleaseHandoffPackMemoryCandidateReconsumeOperatorPackage{
		SchemaVersion: 1, Kind: "pack-memory-reviewed-candidate-reconsume-operator", OperatorID: actionID,
		Pack: status.Pack, SourceCaseRoot: receipt.SourceCaseRoot, PacketPath: receipt.PacketPath, PacketSHA256: receipt.packetHash,
		CandidateDecisionPath: receipt.DecisionPath, CandidateDecisionSHA256: receipt.decisionHash, DecisionReceiptPath: receipt.Path,
		DecisionReceiptSHA256: receipt.receiptHash, CandidatePath: next.CandidatePath, PackTarget: next.PackTarget,
		VerificationState: verificationState, VerificationProofPath: receipt.VerificationProofPath, EvidenceRefs: append([]string{}, next.EvidenceRefs...),
		CurrentRunLoopStepID: currentStep, RunLoop: steps, CurrentDriverRequest: request, Boundary: boundary,
	}
	if pkg.VerificationProofPath != "" {
		pkg.EvidenceRefs = []string{pkg.VerificationProofPath}
	}
	snapshot := *pkg
	snapshot.OperatorSnapshotSHA256 = ""
	data, _ := json.Marshal(snapshot)
	pkg.OperatorSnapshotSHA256 = sha256ReleaseHandoff(data)
	return pkg
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
				Evidence:      []string{"fresh case instance metadata", "fresh case doctor output"},
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
			if action.Decision != "accept" || action.Kind != "managed-doc" || !receipt.VerificationComplete || !receipt.Retired || strings.TrimSpace(receipt.VerificationProofPath) == "" {
				continue
			}
			for _, proofType := range []string{"pack-doctor-output", "fresh-case-reconsume-proof", "attached-case-reconsume-proof"} {
				lifecycle := ReleaseHandoffPackMemoryCandidateReviewArtifact{
					Name:                  proofType,
					CandidatePath:         action.CandidatePath,
					PackTarget:            action.PackTarget,
					SourceCaseRoot:        receipt.SourceCaseRoot,
					PacketPath:            receipt.PacketPath,
					CandidateDecisionPath: receipt.DecisionPath,
					When:                  "after accepted candidate verification produced canonical repo-local proof evidence",
					Action:                "draft deterministic lifecycle proof using the canonical candidate verification proof as hashed repo-local evidence",
					Format:                "strict JSON pack-memory-candidate-lifecycle-proof generated by promote -DraftReviewProof",
					EvidenceRefs:          []string{receipt.VerificationProofPath},
					Evidence:              []string{"canonical candidate verification proof"},
					Boundary: append(append([]string{}, baseBoundary...),
						"reconsume operator reuses existing provision, verification, and retirement receipts; it never runs init, doctor, sync, promote, or retirement automatically",
						"lifecycle proof must use canonical repo-local verification evidence and must not copy raw case output into the kit",
					),
				}
				bound, err := packMemoryCandidateReviewArtifactWithProof(status, lifecycle, proofRoot)
				if err != nil {
					return nil, err
				}
				artifacts = append(artifacts, bound)
			}
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
	if packMemoryCandidateLifecycleProofType(proof.ProofType) && len(artifact.EvidenceRefs) > 0 {
		if err := validatePackMemoryCandidateReconsumeEvidence(status, artifact, proof, path); err != nil {
			return err
		}
	}
	return nil
}

func validatePackMemoryCandidateReconsumeEvidence(status ReleaseHandoffPackMemoryCandidateStatus, artifact ReleaseHandoffPackMemoryCandidateReviewArtifact, proof candidateLifecycleProofInventory, proofPath string) error {
	if len(artifact.EvidenceRefs) != 1 || len(proof.EvidenceRefs) != 1 {
		return fmt.Errorf("candidate lifecycle proof requires exactly one canonical verification evidence ref: %s", proofPath)
	}
	expectedPath := filepath.ToSlash(filepath.Clean(filepath.FromSlash(artifact.EvidenceRefs[0])))
	actualPath := filepath.ToSlash(filepath.Clean(filepath.FromSlash(proof.EvidenceRefs[0].Path)))
	if expectedPath != actualPath {
		return fmt.Errorf("candidate lifecycle proof evidence is not the canonical verification proof: %s", proofPath)
	}
	var receipt *ReleaseHandoffPackMemoryCandidateDecisionReceipt
	for i := range status.DecisionReceipts {
		candidate := &status.DecisionReceipts[i]
		if candidate.VerificationComplete && candidate.Retired && candidate.VerificationProofPath == expectedPath {
			for _, action := range candidate.Actions {
				if action.Decision == "accept" && action.Kind == "managed-doc" && action.CandidatePath == artifact.CandidatePath && action.PackTarget == artifact.PackTarget {
					receipt = candidate
					break
				}
			}
		}
		if receipt != nil {
			break
		}
	}
	if receipt == nil {
		return fmt.Errorf("candidate lifecycle proof lacks current retired decision receipt authority: %s", proofPath)
	}
	evidencePath := filepath.Join(status.repoRootFull, filepath.FromSlash(expectedPath))
	data, err := os.ReadFile(evidencePath)
	if err != nil {
		return err
	}
	if !strings.EqualFold(proof.EvidenceRefs[0].SHA256, sha256ReleaseHandoff(data)) {
		return fmt.Errorf("candidate lifecycle proof canonical verification evidence hash mismatch: %s", proofPath)
	}
	var verification candidateDecisionVerificationInventory
	if err := decodeReleaseHandoffStrictJSON(data, &verification); err != nil {
		return fmt.Errorf("decode canonical candidate verification proof %s: %w", evidencePath, err)
	}
	if verification.SchemaVersion != 1 || verification.Kind != "pack-memory-candidate-decision-verification" || verification.Pack != status.Pack || !strings.EqualFold(verification.PacketHash, receipt.packetHash) || !strings.EqualFold(verification.DecisionHash, receipt.decisionHash) || !strings.EqualFold(verification.ReceiptHash, receipt.receiptHash) || !strings.EqualFold(verification.VerifiedActionsSHA256, receipt.actionsHash) || strings.TrimSpace(verification.ProvisionIntentSHA256) == "" || strings.TrimSpace(verification.ProvisionReceiptSHA256) == "" || !verification.IsMutation || !verification.Applied || !verification.Ready || verification.PackDoctorRows <= 0 || verification.FreshDoctorRows <= 0 || verification.AttachedDoctorRows <= 0 {
		return fmt.Errorf("candidate lifecycle proof canonical verification authority binding mismatch: %s", proofPath)
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
		EvidenceRefs:          append([]string{}, artifact.EvidenceRefs...),
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
	if packMemoryCandidateLifecycleProofType(proof.ProofType) && len(proof.EvidenceRefs) > 0 {
		reason := quoteReleaseHandoffCommandArg("canonical candidate verification proof reviewed for " + proof.ProofType)
		actor := quoteReleaseHandoffCommandArg("mission-commander")
		proof.DraftCommand = strings.ReplaceAll(strings.ReplaceAll(proof.DraftCommand, "<lifecycle-proof-reason>", reason), "<actor>", actor)
		proof.DraftApplyTemplate = strings.ReplaceAll(strings.ReplaceAll(proof.DraftApplyTemplate, "<lifecycle-proof-reason>", reason), "<actor>", actor)
	}
	RefreshPackMemoryCandidateNextMissingProofWorkflow(&proof)
	return proof
}

func RefreshPackMemoryCandidateNextMissingProofWorkflow(next *ReleaseHandoffPackMemoryCandidateReviewNextMissingProof) {
	if next == nil {
		return
	}
	next.CurrentRunLoopStepID = packMemoryCandidateNextMissingProofCurrentRunLoopStepID(*next)
	next.RunLoop = packMemoryCandidateNextMissingProofRunLoop(*next)
	next.CurrentDriverRequest = packMemoryCandidateNextMissingProofCurrentDriverRequest(*next)
}

func packMemoryCandidateNextMissingProofCurrentDriverRequest(next ReleaseHandoffPackMemoryCandidateReviewNextMissingProof) *mission.MissionCommanderDriverRequest {
	currentRunLoopStepID := strings.TrimSpace(next.CurrentRunLoopStepID)
	if currentRunLoopStepID == "" {
		return nil
	}
	actionID := packMemoryCandidateProofActionID(next)
	label := strings.TrimSpace(next.ProofType)
	if label == "" {
		label = strings.TrimSpace(next.CandidatePath)
	}
	if label == "" {
		label = strings.TrimSpace(next.Path)
	}
	action := mission.MissionCommanderNextActionItem{
		Label:          label,
		ActionID:       actionID,
		State:          "pack-memory-proof-required",
		Command:        packMemoryCandidateCurrentProofCommand(next),
		Source:         "packMemoryCandidateProof.nextMissingProof",
		RequiresReview: true,
		Reasons:        packMemoryCandidateNextMissingProofDriverReasons(next, actionID),
		Boundary:       packMemoryCandidateNextMissingProofDriverBoundary(next),
	}
	request := mission.MissionCommanderCurrentDriverRequest(action, currentRunLoopStepID, next.RunLoop)
	if request == nil {
		return nil
	}
	request.Boundary = mission.UniqueStrings(append(request.Boundary, action.Boundary...))
	refreshed := mission.MissionCommanderDriverRequestWithRefreshStatusCommand(*request, packMemoryCandidateNextMissingProofStatusCommand(next))
	return &refreshed
}

func packMemoryCandidateNextMissingProofDriverReasons(next ReleaseHandoffPackMemoryCandidateReviewNextMissingProof, actionID string) []string {
	reasons := []string{"actionId=" + actionID}
	if stage := strings.TrimSpace(next.Stage); stage != "" {
		reasons = append(reasons, "proofStage="+stage)
	}
	if proofType := strings.TrimSpace(next.ProofType); proofType != "" {
		reasons = append(reasons, "proofType="+proofType)
	}
	if proofPath := strings.TrimSpace(next.Path); proofPath != "" {
		reasons = append(reasons, "proof path="+proofPath)
	}
	if candidate := strings.TrimSpace(next.CandidatePath); candidate != "" {
		reasons = append(reasons, "candidate="+candidate)
	}
	if packTarget := strings.TrimSpace(next.PackTarget); packTarget != "" {
		reasons = append(reasons, "packTarget="+packTarget)
	}
	if sourceCase := strings.TrimSpace(next.SourceCaseRoot); sourceCase != "" {
		reasons = append(reasons, "sourceCaseRoot="+sourceCase)
	}
	return mission.UniqueStrings(reasons)
}

func packMemoryCandidateNextMissingProofDriverBoundary(next ReleaseHandoffPackMemoryCandidateReviewNextMissingProof) []string {
	boundary := append([]string{}, next.Boundary...)
	boundary = append(boundary,
		"nextMissingProof.currentDriverRequest is the typed durable consumer request; do not reconstruct proof workflow commands from terminal prose",
		"run only the currentDriverRequest command when commandExecutable=true; otherwise treat guidance as main-agent review input",
		"status/release-check do not create proof files, merge candidates, cleanup paths, reconsume cases, write authority/confirmed, or execute heavy tools",
	)
	if strings.TrimSpace(next.Stage) == "reconsume-proof-required" || packMemoryCandidateLifecycleProofType(next.ProofType) {
		boundary = append(boundary, "reconsume proof requests record bounded proof workflow only; they do not run doctor/init/reconsume or create fresh/attached cases automatically")
	}
	return mission.UniqueStrings(boundary)
}

func packMemoryCandidateNextMissingProofCurrentRunLoopStepID(next ReleaseHandoffPackMemoryCandidateReviewNextMissingProof) string {
	if strings.TrimSpace(next.ProofType) == "" && strings.TrimSpace(next.Path) == "" && strings.TrimSpace(next.CandidatePath) == "" {
		return ""
	}
	if next.RequiresPacket && strings.TrimSpace(next.PacketPath) == "" {
		return "bind-review-packet"
	}
	if next.RequiresCandidateDecision && strings.TrimSpace(next.CandidateDecisionPath) == "" {
		return "bind-review-packet"
	}
	if strings.TrimSpace(next.DraftCommand) != "" {
		return "draft-proof-whatif"
	}
	return "inspect-proof-gap"
}

func packMemoryCandidateNextMissingProofRunLoop(next ReleaseHandoffPackMemoryCandidateReviewNextMissingProof) []mission.MissionCommanderRunLoopStep {
	current := packMemoryCandidateNextMissingProofCurrentRunLoopStepID(next)
	if current == "" {
		return nil
	}
	steps := []mission.MissionCommanderRunLoopStep{}
	add := func(step mission.MissionCommanderRunLoopStep) {
		step.StepID = strings.TrimSpace(step.StepID)
		step.Actor = strings.TrimSpace(step.Actor)
		step.Description = strings.TrimSpace(step.Description)
		if step.StepID == "" || step.Description == "" {
			return
		}
		step.Order = len(steps) + 1
		step.Boundary = mission.UniqueStrings(step.Boundary)
		steps = append(steps, step)
	}
	statusCommand := packMemoryCandidateNextMissingProofStatusCommand(next)
	commonBoundary := []string{
		"pack-memory proof workflow is an operator handoff; status/release-check do not create proof files",
		"proof Apply requires the ExpectedProofSha256 returned by the matching WhatIf",
		"no authority/confirmed writes and no heavy-tool execution",
		"proof files must stay repo-local review evidence and must not contain case-specific artifacts, traces, dumps, captures, payloads, flags, or customer data",
	}
	add(mission.MissionCommanderRunLoopStep{
		StepID:      "inspect-proof-gap",
		Actor:       "main-agent",
		Description: "inspect the next missing pack-memory proof, candidate path, pack target, expected proof path, and read-only boundaries",
		Command:     statusCommand,
		State:       "pack-memory-proof-required",
		Source:      "packMemoryCandidateProof.workflow.inspect",
		Boundary: append([]string{
			"inspect is read-only and must not write proof, merge candidates, cleanup paths, or reconsume cases",
		}, commonBoundary...),
	})
	add(mission.MissionCommanderRunLoopStep{
		StepID:      "bind-review-packet",
		Actor:       "main-agent",
		Description: "bind a canonical case-local review packet, candidate decision path, and bounded evidence refs before proof drafting",
		Command:     packMemoryCandidateNextMissingProofBindCommand(next),
		State:       "pack-memory-proof-required",
		Source:      "packMemoryCandidateProof.workflow.reviewPacket",
		Boundary: append([]string{
			"release-check cannot infer the case-local review packet without an attached source case",
			"do not fabricate packet, candidate decision, or evidence refs",
		}, commonBoundary...),
	})
	add(mission.MissionCommanderRunLoopStep{
		StepID:      "draft-proof-whatif",
		Actor:       "main-agent",
		Description: "run the DraftReviewProof WhatIf for the next missing proof and inspect the returned proof hash",
		Command:     strings.TrimSpace(next.DraftCommand),
		State:       "pack-memory-proof-required",
		Source:      "packMemoryCandidateProof.workflow.whatIf",
		Boundary: append([]string{
			"WhatIf is read-only and does not create the proof file",
			"replace placeholders only with reviewed packet, decision, actor, reason, and repo-local evidence refs",
		}, commonBoundary...),
	})
	add(mission.MissionCommanderRunLoopStep{
		StepID:      "apply-proof-with-expected-hash",
		Actor:       "main-agent",
		Description: "apply the proof draft only with the ExpectedProofSha256 returned by the matching WhatIf preview",
		Command:     strings.TrimSpace(next.DraftApplyTemplate),
		State:       "pack-memory-proof-required",
		Source:      "packMemoryCandidateProof.workflow.apply",
		Boundary: append([]string{
			"Apply writes only the bounded proof file at the next missing proof path",
			"do not reuse a stale proof hash after packet, decision, evidence, reason, actor, or candidate state changes",
		}, commonBoundary...),
	})
	add(mission.MissionCommanderRunLoopStep{
		StepID:      "refresh-pack-memory-status",
		Actor:       "main-agent",
		Description: "rerun status or release-check to verify the proof is present and recompute the next pack-memory action",
		Command:     statusCommand,
		State:       "pack-memory-proof-required",
		Source:      "packMemoryCandidateProof.workflow.refresh",
		Boundary: append([]string{
			"refresh is read-only and must not infer proof completion from a prior preview",
		}, commonBoundary...),
	})
	add(mission.MissionCommanderRunLoopStep{
		StepID:      "continue-review-cleanup-reconsume",
		Actor:       "main-agent",
		Description: "continue with the refreshed Mission Commander pack-memory queue for remaining cleanup, reconsume, verification, or release handoff",
		Command:     statusCommand,
		State:       "pack-memory-proof-required",
		Source:      "packMemoryCandidateProof.workflow.continue",
		Boundary: append([]string{
			"do not declare closure until refreshed status/release-check exposes no open pack-memory work",
			"cleanup, reconsume, and verification remain separate explicit bounded steps",
		}, commonBoundary...),
	})
	return steps
}

func packMemoryCandidateNextMissingProofStatusCommand(next ReleaseHandoffPackMemoryCandidateReviewNextMissingProof) string {
	if target := strings.TrimSpace(next.SourceCaseRoot); target != "" {
		return "/rekit status -Target " + quoteReleaseHandoffCommandArg(target) + " -Format json"
	}
	return "/rekit release-check -Format json"
}

func packMemoryCandidateNextMissingProofBindCommand(next ReleaseHandoffPackMemoryCandidateReviewNextMissingProof) string {
	if packetPath := strings.TrimSpace(next.PacketPath); packetPath != "" {
		return "review packet already bound: " + packetPath
	}
	if target := strings.TrimSpace(next.SourceCaseRoot); target != "" {
		return "/rekit promote -Target " + quoteReleaseHandoffCommandArg(target) + " -CreateCandidates -Review -Format json"
	}
	return "rerun promote -CreateCandidates -Review from the attached source case to bind a canonical review packet"
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
	CaseRoot                 string                             `json:"caseRoot,omitempty"`
	FreshCaseRoot            string                             `json:"freshCaseRoot,omitempty"`
	AttachedCaseRoot         string                             `json:"attachedCaseRoot,omitempty"`
	PacketHash               string                             `json:"packetHash"`
	DecisionHash             string                             `json:"decisionHash"`
	ReceiptHash              string                             `json:"receiptHash"`
	ReceiptPath              string                             `json:"receiptPath,omitempty"`
	VerificationProofPath    string                             `json:"verificationProofPath,omitempty"`
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
	VerifiedActionsSHA256    string                             `json:"verifiedActionsSha256"`
	VerifiedActions          []candidateDecisionActionInventory `json:"verifiedActions,omitempty"`
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
			expectedWorkspace, err := projectstate.Join(raw.CaseRoot, "verifications", "candidate-decisions", provisionID)
			if err != nil {
				return nil, err
			}
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
				if proof.SchemaVersion != 1 || proof.Kind != "pack-memory-candidate-decision-verification" || proof.Pack != raw.Pack || !strings.EqualFold(proof.PacketHash, raw.PacketHash) || !strings.EqualFold(proof.DecisionHash, raw.DecisionHash) || !strings.EqualFold(proof.ReceiptHash, sha256ReleaseHandoff(data)) || !strings.EqualFold(proof.VerifiedActionsSHA256, candidateDecisionActionsSHA256(raw.Actions)) || !proof.IsMutation || !proof.Applied || !proof.Ready || proof.PackDoctorRows <= 0 || proof.FreshDoctorRows <= 0 || proof.AttachedDoctorRows <= 0 {
					return nil, fmt.Errorf("candidate verification proof binding mismatch: %s", raw.VerificationProofPath)
				}
				if raw.Accepted > 0 {
					if strings.TrimSpace(proof.ProvisionIntentSHA256) == "" || strings.TrimSpace(proof.ProvisionReceiptSHA256) == "" {
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
			actionsHash:                  candidateDecisionActionsSHA256(raw.Actions),
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
		joined := quoteReleaseHandoffCommandArg(strings.Join(evidenceRefs, ","))
		command = strings.ReplaceAll(command, "<review-evidence-ref>", joined)
		command = strings.ReplaceAll(command, "<repo-local-lifecycle-evidence-ref>", joined)
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
	handoff.RetirementPreviewCommand = releaseHandoffPromoteCommandWithTarget(fmt.Sprintf("/rekit promote -PacketPath %s -CandidateDecisionPath %s -RetireCandidateVerificationWorkspace -WhatIf -Format json", quoteReleaseHandoffCommandArg(receipt.PacketPath), quoteReleaseHandoffCommandArg(receipt.DecisionPath)), receipt.CaseRoot)
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
	verificationProofPath := receipt.VerificationProofPath
	provisionIntentPath := filepath.Join(workspace, "provision.intent.json")
	provisionReceiptPath := filepath.Join(workspace, "provision.receipt.json")
	intentPath := filepath.Join(proofRoot, retirementID+".candidate-verification-retirement-intent.json")
	retirementReceiptPath := filepath.Join(proofRoot, retirementID+".candidate-verification-retirement-receipt.json")
	expectedKind := "pack-memory-candidate-verification-retirement" + kindSuffix
	if artifact.SchemaVersion != 1 || artifact.Kind != expectedKind || artifact.Pack != receipt.Pack || !sameReleaseHandoffPath(artifact.RepoRoot, repo) || !sameReleaseHandoffPath(artifact.SourceCaseRoot, receipt.CaseRoot) || !sameReleaseHandoffPath(artifact.PacketPath, receipt.PacketPath) || !strings.EqualFold(artifact.PacketSHA256, receipt.PacketHash) || !sameReleaseHandoffPath(artifact.DecisionPath, receipt.DecisionPath) || !strings.EqualFold(artifact.DecisionSHA256, receipt.DecisionHash) || !sameReleaseHandoffPath(artifact.DecisionReceiptPath, decisionReceiptPath) || !strings.EqualFold(artifact.DecisionReceiptSHA256, sha256ReleaseHandoff(decisionReceiptData)) || !sameReleaseHandoffPath(artifact.VerificationProofPath, verificationProofPath) || !strings.EqualFold(artifact.VerificationProofSHA256, fileSHA256ReleaseHandoff(verificationProofPath)) || !sameReleaseHandoffPath(artifact.ProvisionIntentPath, provisionIntentPath) || !strings.EqualFold(artifact.ProvisionIntentSHA256, proof.ProvisionIntentSHA256) || !sameReleaseHandoffPath(artifact.ProvisionReceiptPath, provisionReceiptPath) || !strings.EqualFold(artifact.ProvisionReceiptSHA256, proof.ProvisionReceiptSHA256) || !sameReleaseHandoffPath(artifact.WorkspaceRoot, workspace) || !sameReleaseHandoffPath(artifact.RetirementIntentPath, intentPath) || !sameReleaseHandoffPath(artifact.RetirementReceiptPath, retirementReceiptPath) || strings.TrimSpace(artifact.RetirementSHA256) == "" || len(artifact.Roots) != 2 || len(artifact.RetirementPlans) != 2 {
		return fmt.Errorf("candidate verification retirement artifact authority binding mismatch: %s", artifact.RetirementIntentPath)
	}
	if len(artifact.ProvisionArtifactsToDelete) != 2 || !sameReleaseHandoffPath(artifact.ProvisionArtifactsToDelete[0], provisionReceiptPath) || !sameReleaseHandoffPath(artifact.ProvisionArtifactsToDelete[1], provisionIntentPath) || len(artifact.EmptyAncestorsToRemove) != 2 || !sameReleaseHandoffPath(artifact.EmptyAncestorsToRemove[0], filepath.Dir(workspace)) || !sameReleaseHandoffPath(artifact.EmptyAncestorsToRemove[1], filepath.Dir(filepath.Dir(workspace))) {
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
		{path: filepath.Join(receipt.VerificationWorkspaceRoot, "provision.intent.json"), sha256: proof.ProvisionIntentSHA256},
		{path: filepath.Join(receipt.VerificationWorkspaceRoot, "provision.receipt.json"), sha256: proof.ProvisionReceiptSHA256},
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
		{path: filepath.Join(receipt.VerificationWorkspaceRoot, "provision.intent.json"), sha256: proof.ProvisionIntentSHA256, label: "provision intent"},
		{path: filepath.Join(receipt.VerificationWorkspaceRoot, "provision.receipt.json"), sha256: proof.ProvisionReceiptSHA256, label: "provision receipt"},
	} {
		info, err := os.Lstat(item.path)
		if err != nil || info.Mode()&os.ModeSymlink != 0 || !info.Mode().IsRegular() || !strings.EqualFold(fileSHA256ReleaseHandoff(item.path), item.sha256) {
			return fmt.Errorf("candidate verification %s is missing or invalid: %s", item.label, item.path)
		}
	}
	for _, path := range []string{receipt.VerificationWorkspaceRoot, filepath.Join(receipt.VerificationWorkspaceRoot, "fresh"), filepath.Join(receipt.VerificationWorkspaceRoot, "attached")} {
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

func candidateDecisionActionsSHA256(actions []candidateDecisionActionInventory) string {
	data, err := json.Marshal(actions)
	if err != nil {
		return ""
	}
	return sha256ReleaseHandoff(data)
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

func BuildNextBatchSelectionPackage(handoff ReleaseHandoff) *ReleaseHandoffNextBatchSelectionPackage {
	if !releaseHandoffReadyForNextBatchSelection(handoff) {
		return nil
	}
	if releaseHandoffUsesExactActiveRouteNextBatch(handoff) {
		current := releaseHandoffExactActiveRouteNextBatchAction(handoff.ActiveRoute)
		actions := []mission.MissionCommanderNextActionItem{current}
		queue := mission.MissionCommanderActionQueueFor(actions)
		return &ReleaseHandoffNextBatchSelectionPackage{
			Ready:                       true,
			Summary:                     queue.Summary,
			MissionCommanderNextActions: actions,
			MissionCommanderActionQueue: queue,
			Boundary:                    append([]string{}, current.Boundary...),
		}
	}
	current := releaseHandoffNextBatchSelectionAction(handoff)
	actions := append([]mission.MissionCommanderNextActionItem{current}, releaseHandoffNextBatchCandidateActions(handoff)...)
	actions = mission.UniqueCommanderNextActions(actions)
	queue := mission.MissionCommanderActionQueueFor(actions)
	starter := releaseHandoffNextBatchStarterPackage(handoff)
	return &ReleaseHandoffNextBatchSelectionPackage{
		Ready:                       true,
		Summary:                     queue.Summary,
		StarterPackage:              starter,
		MissionCommanderNextActions: append([]mission.MissionCommanderNextActionItem{}, actions...),
		MissionCommanderActionQueue: queue,
		NextBatchPlanningRoutes:     releaseHandoffNextBatchPlanningRoutes(actions, starter),
		Boundary:                    append([]string{}, current.Boundary...),
	}
}

func releaseHandoffNextBatchStarterPackage(handoff ReleaseHandoff) *ReleaseHandoffNextBatchStarterPackage {
	latest := strings.TrimSpace(handoff.LatestBatch.BatchID)
	next := releaseHandoffNextBatchID(latest)
	sectionTitle := "Batch <next>：<Windows-verifiable product-path closure>"
	if next != "" {
		sectionTitle = next + "：<Windows-verifiable product-path closure>"
	}
	currentSection := strings.Join([]string{
		"### " + sectionTitle,
		"",
		"状态：进行中。本批选择 <candidate-domain>，推进一个 Windows 本机可验证的 Mission Commander / replacement executor product-path slice。写清上一批完成后仍需要解决的接手断点，以及本批为何不是字段/文案/summary 微调。",
		"",
		"目标：<用 1 段描述可由 status/handoff/continue/release-check 或临时 case 验证的 operational closure。>",
		"",
		"边界：本批不新增 PowerShell runtime logic，不执行 heavy-tool，不写 authority/confirmed，不自动执行 reviewer/adapter/pack-memory/gate mutation；所有写入仍走既有 explicit WhatIf → expected-hash Apply 或 authorized-gate boundary。",
		"",
		"验证标准：<列 focused regressions、必要 product-path/temporary case checks，并在实现后运行完整本机 release minimum。>",
	}, "\n")
	changelogBatch := next
	if changelogBatch == "" {
		changelogBatch = "Batch <next>"
	}
	changelogEntry := "- " + changelogBatch + " 新增 <product-path closure>：<用户可见变化、同源 runtime/CLI envelope、关键边界与验证结果。>"
	validationCommands := releaseHandoffValidationCommands(handoff.Validation)
	return &ReleaseHandoffNextBatchStarterPackage{
		Ready:                true,
		LatestCompletedBatch: latest,
		SuggestedNextBatch:   next,
		CurrentBatchSection:  currentSection,
		ChangelogEntry:       changelogEntry,
		ValidationCommands:   validationCommands,
		ReleaseCadenceSteps: []string{
			"先在 Windows 本机完成 focused regressions 与完整 release minimum。",
			"提交并推送一次 implementation commit（代码、测试、文档、本机验证）。",
			"推送成功后立即继续下一批，不轮询或等待 remote release-gate。",
			"远程 Linux/macOS/Windows workflow 只作为异步信号；仅在正式发布、跨平台专项或周期复审时等待并记录结果。",
		},
		RecommendedStarterSteps: []string{
			"从 next-batch candidate-domain 中选择一个中型 product-path closure。",
			"先更新 docs/batch-plan.md current batch state，再改 runtime/tests。",
			"实现时优先复用既有 typed handoff/envelope，不创建并行业务逻辑。",
			"focused regressions 通过后再运行完整本机 release minimum。",
		},
		Boundary: mission.UniqueStrings(append([]string{
			"starter package is read-only guidance; it must not create or modify docs by itself",
			"do not use starter package templates to justify a single-field, summary, or projection-only micro-batch",
			"do not execute reviewer, adapter, pack-memory, gate, heavy-tool, sync, or promote mutation from starter guidance",
		}, releaseHandoffNextBatchCandidateBoundary(handoff)...)),
		CurrentRunLoopStepID: "select-candidate-domain",
		RunLoop:              releaseHandoffNextBatchStarterRunLoop(validationCommands),
	}
}

func releaseHandoffNextBatchStarterRunLoop(validationCommands []string) []mission.MissionCommanderRunLoopStep {
	validationCommand := strings.Join(validationCommands, " && ")
	if strings.TrimSpace(validationCommand) == "" {
		validationCommand = "run the selected focused regressions, then the full local release minimum"
	}
	steps := []mission.MissionCommanderRunLoopStep{}
	add := func(step mission.MissionCommanderRunLoopStep) {
		step.StepID = strings.TrimSpace(step.StepID)
		step.Actor = strings.TrimSpace(step.Actor)
		step.Description = strings.TrimSpace(step.Description)
		if step.StepID == "" || step.Actor == "" || step.Description == "" {
			return
		}
		step.Order = len(steps) + 1
		step.Boundary = mission.UniqueStrings(step.Boundary)
		steps = append(steps, step)
	}
	add(mission.MissionCommanderRunLoopStep{StepID: "select-candidate-domain", Actor: "main-agent", Description: "choose one next-batch candidate domain and name the user-visible operational closure before editing docs", Command: "select a Windows-verifiable product-path closure from the next-batch candidate domains", State: "ready-for-next-batch-selection", Source: "releaseHandoffNextBatch.starter.select", Boundary: []string{"starter package is read-only guidance and does not select the batch by itself", "do not choose a single-field, summary, or projection-only micro-batch"}})
	add(mission.MissionCommanderRunLoopStep{StepID: "draft-batch-plan", Actor: "main-agent", Description: "write the selected Batch section into docs/batch-plan.md current batch state before implementation", Command: "update docs/batch-plan.md current batch state with the selected product-path slice", State: "ready-for-next-batch-selection", Source: "releaseHandoffNextBatch.starter.batchPlan", Boundary: []string{"batch-plan edits must name the real handoff gap and verification standard", "do not copy long history or case-specific artifacts into the active batch section"}})
	add(mission.MissionCommanderRunLoopStep{StepID: "implement-slice", Actor: "main-agent", Description: "implement only the selected runtime, CLI, test, or documentation support needed for that closure", Command: "implement the selected Windows-verifiable product-path slice", State: "ready-for-next-batch-selection", Source: "releaseHandoffNextBatch.starter.implementation", Boundary: []string{"do not add PowerShell runtime logic or a parallel runtime path", "do not execute reviewer, adapter, pack-memory, gate, heavy-tool, sync, or promote mutation from starter guidance"}})
	add(mission.MissionCommanderRunLoopStep{StepID: "update-release-notes", Actor: "main-agent", Description: "update CHANGELOG.md Unreleased with the selected Batch entry, boundaries, and validation result", Command: "update CHANGELOG.md Unreleased for the selected Batch", State: "ready-for-next-batch-selection", Source: "releaseHandoffNextBatch.starter.releaseNotes", Boundary: []string{"CHANGELOG should record user-visible change and release truth, not full implementation history"}})
	add(mission.MissionCommanderRunLoopStep{StepID: "validate-local", Actor: "main-agent", Description: "run focused regressions for the selected slice and then the local release minimum", Command: validationCommand, State: "ready-for-next-batch-selection", Source: "releaseHandoffNextBatch.starter.validation", Boundary: []string{"focused regressions should prove the operational closure before full release validation", "release-check inventory ready is not remote CI green"}})
	add(mission.MissionCommanderRunLoopStep{StepID: "commit-and-continue", Actor: "main-agent", Description: "commit and push the Windows-validated implementation, then continue the next batch without waiting for remote CI", Command: "commit/push the implementation once, then continue without polling remote release-gate", State: "ready-for-next-batch-selection", Source: "releaseHandoffNextBatch.starter.releaseCadence", Boundary: []string{"normal batches use one implementation commit/push after the Windows local release minimum", "remote Linux/macOS/Windows workflow is asynchronous and non-blocking outside release, cross-platform work, or periodic review"}})
	return steps
}

func releaseHandoffNextBatchPlanningRoutes(actions []mission.MissionCommanderNextActionItem, starter *ReleaseHandoffNextBatchStarterPackage) []ReleaseHandoffNextBatchPlanningRoute {
	if starter == nil || !starter.Ready || len(actions) == 0 {
		return nil
	}
	placeholder := "<Windows-verifiable product-path closure>"
	routes := []ReleaseHandoffNextBatchPlanningRoute{}
	for _, action := range actions {
		if strings.TrimSpace(action.State) != "next-batch-candidate-domain" {
			continue
		}
		domain := strings.TrimSpace(action.Label)
		if domain == "" {
			continue
		}
		routes = append(routes, ReleaseHandoffNextBatchPlanningRoute{
			Ready:                   true,
			Domain:                  domain,
			DomainActionID:          strings.TrimSpace(action.ActionID),
			ClosurePlaceholder:      placeholder,
			WhatIfCommandTemplate:   "/rekit next-batch -Domain " + quoteReleaseHandoffCommandArg(domain) + " -Closure " + quoteReleaseHandoffCommandArg(placeholder) + " -WhatIf -Format json",
			CommandExecutable:       false,
			RequiresReview:          true,
			RefreshStatusCommand:    "/rekit status -Format json",
			ExpectedApplySource:     "nextBatchCommand",
			ExpectedApplyDriverKind: "preview-command",
			RunbookSteps: []string{
				"choose exactly one nextBatchPlanningRoutes[] item and replace closurePlaceholder with a concrete product-path closure",
				"run whatIfCommandTemplate only after replacing the closure placeholder; do not execute the placeholder template verbatim",
				"review the returned expectedNextBatchPlanSha256, then consume the returned missionCommanderActionQueue.currentDriverRequest for hash-bound Apply",
				"after Apply, run refreshStatusCommand and rebuild status before implementation",
			},
			Boundary: []string{
				"nextBatchPlanningRoutes are read-only durable handoff templates; release-check/status does not choose a batch or edit docs",
				"the placeholder template is not commandExecutable until closurePlaceholder is replaced with a concrete closure",
				"WhatIf is read-only and Apply writes only docs/batch-history.md, CHANGELOG.md, and docs/batch-plan.md when the expected hash matches",
				"planning routes do not execute reviewer, adapter, pack-memory, gate, sync, promote, heavy-tool, authority, confirmed, commit, push, or remote CI actions",
			},
		})
	}
	return routes
}

func releaseHandoffValidationCommands(validation []ReleaseHandoffValidation) []string {
	commands := make([]string, 0, len(validation))
	for _, item := range validation {
		if strings.TrimSpace(item.Command) != "" {
			commands = append(commands, item.Command)
		}
	}
	return mission.UniqueStrings(commands)
}

func releaseHandoffNextBatchID(latest string) string {
	n := latestBatchIDNumber(latest)
	if n < 0 {
		return ""
	}
	return fmt.Sprintf("Batch %d", n+1)
}

func releaseHandoffUsesExactActiveRouteNextBatch(handoff ReleaseHandoff) bool {
	return handoff.ActiveRoute.Present &&
		handoff.ActiveRoute.Ready &&
		handoff.ActiveRoute.ProjectionConsistent &&
		handoff.ActiveRoute.State == "completed" &&
		handoff.ActiveRoute.NextBatchUnlocked &&
		releaseHandoffBatchID(handoff.ActiveRoute.NextBatch) != "" &&
		strings.EqualFold(
			releaseHandoffBatchID(handoff.ActiveRoute.NextBatch),
			strings.TrimSpace(handoff.ActiveRoute.ExclusiveClaim),
		)
}

func releaseHandoffExactActiveRouteNextBatchAction(route ReleaseHandoffActiveRoute) mission.MissionCommanderNextActionItem {
	next := releaseHandoffBatchID(route.NextBatch)
	return mission.MissionCommanderNextActionItem{
		Label:          next,
		ActionID:       "active-route-next-batch-selection",
		State:          "ready-for-next-batch-selection",
		Command:        "continue the exact unlocked active route next batch " + next + " from " + route.Path,
		Source:         "releaseHandoffActiveRoute.nextBatch",
		Blocked:        false,
		RequiresReview: true,
		Reasons: mission.UniqueStrings([]string{
			"the approved active route completed its current batch",
			"exact next batch is unlocked by the active route: " + next,
			"exclusive claim matches the exact next batch; generic candidate domains are not available",
		}),
		Boundary: []string{
			"consume only the exact active-route next batch id; do not select from a generic candidate pool",
			"read the active route and batch-plan projection before implementation",
			"planning guidance is read-only and does not write docs, case state, authority/confirmed, or execute tools",
		},
	}
}

func releaseHandoffActiveRouteValidationAction(route ReleaseHandoffActiveRoute) *mission.MissionCommanderNextActionItem {
	if !route.Present || !route.Ready || !route.ProjectionConsistent || route.State != "completed" {
		return nil
	}
	validation := route.LocalValidationReceipt
	if validation == nil || (!validation.Ready && validation.State != "recorded-for-implementation-commit") {
		state := "not-recorded"
		if validation != nil && strings.TrimSpace(validation.State) != "" {
			state = validation.State
		}
		return &mission.MissionCommanderNextActionItem{
			Label:          route.ExclusiveClaim,
			ActionID:       "active-route-local-validation",
			State:          "active-route-validation-required",
			Command:        "/rekit release-run -Format json",
			Source:         "releaseHandoffActiveRoute.localValidationReceipt",
			RequiresReview: true,
			Reasons: mission.UniqueStrings([]string{
				"the completed active route has no current exact machine validation receipt: " + state,
				"route completion and machine validation remain independent truth sources",
			}),
			Boundary: []string{
				"run the full local release minimum only after the exact route/current/state/claim/next projection is complete",
				"release-run records a Git-local typed receipt and does not commit, push, or claim remote CI green",
				"do not use the latest numbered batch receipt as active-route evidence",
			},
		}
	}
	if validation.State == "recorded-for-implementation-commit" && validation.Receipt != nil && validation.Receipt.Subject != nil {
		return releaseHandoffActiveRouteCommitAction(route)
	}
	if validation.Ready && (route.PostPushReceipt == nil || !route.PostPushReceipt.Ready) {
		state := "not-recorded"
		if route.PostPushReceipt != nil && strings.TrimSpace(route.PostPushReceipt.State) != "" {
			state = route.PostPushReceipt.State
		}
		return &mission.MissionCommanderNextActionItem{
			Label:          route.ExclusiveClaim,
			ActionID:       "active-route-post-push-reconcile",
			State:          state,
			Command:        "make the validated direct active-route commit the clean synchronized main HEAD, then refresh status",
			Source:         "releaseHandoffActiveRoute.postPushReceipt",
			RequiresReview: true,
			Reasons: mission.UniqueStrings([]string{
				"the active-route implementation commit matches the exact validation receipt",
				"post-push receipt is not ready: " + state,
			}),
			Boundary: []string{
				"do not amend or add another commit after the validated direct implementation commit",
				"use only local Git refs for status; this action does not fetch, pull, push, or claim remote CI green",
				"refresh status after the user reconciles main and the local origin/main tracking ref",
			},
		}
	}
	return nil
}

func releaseHandoffFinalActiveRouteAction(route ReleaseHandoffActiveRoute) *mission.MissionCommanderNextActionItem {
	if action := releaseHandoffActiveRouteValidationAction(route); action != nil {
		return action
	}
	if route.Ready && route.ProjectionConsistent && route.State == "completed" && !releaseHandoffNextBatchSelectable(route.NextBatch) {
		return releaseHandoffCompletedRouteAction(route)
	}
	if route.NextBatchUnlocked {
		return nil
	}
	return releaseHandoffActiveRouteAction(route)
}

func releaseHandoffActiveRouteCommitAction(route ReleaseHandoffActiveRoute) *mission.MissionCommanderNextActionItem {
	return &mission.MissionCommanderNextActionItem{
		Label:          route.ExclusiveClaim,
		ActionID:       "active-route-implementation-commit",
		State:          "recorded-for-implementation-commit",
		Command:        "create and push the one direct implementation commit bound by the active-route validation receipt, then refresh status",
		Source:         "releaseHandoffActiveRoute.localValidationReceipt",
		RequiresReview: true,
		Reasons: mission.UniqueStrings([]string{
			"the full local release minimum passed for the exact active route subject",
			"the current working-tree artifact snapshot still matches the Git-local typed receipt",
		}),
		Boundary: []string{
			"commit only the exact artifact set bound by activeRoute.localValidationReceipt",
			"do not reinterpret this commit as a repair for the latest numbered batch",
			"refresh status after push so postPushReceipt can bind HEAD to the local origin/main tracking ref",
			"the receipt does not execute commit or push and does not claim remote CI green",
		},
	}
}

func releaseHandoffReadyForNextBatchSelection(handoff ReleaseHandoff) bool {
	if handoff.ActiveRoute.Present {
		if !releaseHandoffUsesExactActiveRouteNextBatch(handoff) || !handoff.ActiveRoute.LocalValidationReady || !handoff.ActiveRoute.ReleaseCheckReady || handoff.ActiveRoute.PostPushReceipt == nil || !handoff.ActiveRoute.PostPushReceipt.Ready {
			return false
		}
	} else {
		if !handoff.LatestBatch.Handoff.LocalValidationReady || !handoff.LatestBatch.Handoff.ReleaseCheckReady {
			return false
		}
		cadence := handoff.LatestBatch.Handoff.ReleaseInspectionCadence
		if cadence.State != "complete" || !cadence.ImplementationCommitReady {
			return false
		}
	}
	if !handoff.Ready {
		return false
	}
	packCandidates := handoff.PackMemoryCandidates
	return packCandidates.Ready && packCandidates.Total == 0 && len(packCandidates.Packs) == 0
}

func releaseHandoffNextBatchSelectionAction(handoff ReleaseHandoff) mission.MissionCommanderNextActionItem {
	reasons := []string{
		"latest batch Windows local validation is complete",
		"implementation commit/push evidence is recorded",
		"project is ready for the next Windows-verifiable product-path batch without waiting for remote CI",
	}
	if latest := strings.TrimSpace(handoff.LatestBatch.BatchID); latest != "" {
		reasons = append(reasons, "latest completed batch: "+latest)
	}
	if gate := strings.TrimSpace(handoff.LatestBatch.Handoff.RemoteReleaseGate); gate != "" && gate != "not-recorded" {
		reasons = append(reasons, "latest asynchronous remote release gate: "+gate)
	}
	boundary := []string{
		"do not poll or wait for remote CI during a normal Windows-first batch",
		"do not claim remote CI green unless latest batch evidence explicitly records green jobs",
		"consume remote Linux/macOS/Windows results only for release, cross-platform work, or periodic review",
		"avoid single-field, summary, text, or handoff projection micro-batches; choose an operational closure with runtime or product-path verification",
		"run focused regressions and the Windows local release minimum before the next implementation commit",
	}
	if detail := handoff.LatestBatch.Handoff.RemoteReleaseGateDetail; detail != nil && detail.State != "not-recorded" {
		reasons = append(reasons, "previous asynchronous remote release-gate detail recorded")
		boundary = append(boundary, detail.Boundary...)
	}
	boundary = append(boundary, handoff.LatestBatch.Handoff.ReleaseInspectionCadence.Boundary...)
	return mission.MissionCommanderNextActionItem{
		Label:          "next-batch",
		ActionID:       "next-batch-selection",
		State:          "ready-for-next-batch-selection",
		Command:        "select the next Windows-verifiable product-path closure from docs/context-routing.md and docs/batch-plan.md, then update docs/batch-plan.md current batch state before implementation",
		Source:         "releaseHandoffNextBatch",
		Blocked:        false,
		RequiresReview: false,
		Reasons:        mission.UniqueStrings(reasons),
		Boundary:       mission.UniqueStrings(boundary),
	}
}

func releaseHandoffNextBatchCandidateActions(handoff ReleaseHandoff) []mission.MissionCommanderNextActionItem {
	reasons := releaseHandoffNextBatchCandidateReasons(handoff)
	boundary := releaseHandoffNextBatchCandidateBoundary(handoff)
	domains := []struct {
		label    string
		actionID string
		command  string
	}{
		{label: "mission-commander", actionID: "next-batch-mission-commander-operational-closure", command: "select a Mission Commander operational closure slice with status/handoff/continue product-path verification"},
		{label: "replacement-executor", actionID: "next-batch-replacement-executor-takeover", command: "select a replacement executor takeover slice that can be resumed from status or durable handoff without prior chat context"},
		{label: "reviewer-orchestration", actionID: "next-batch-reviewer-orchestration-closure", command: "select a reviewer orchestration slice that improves bounded dispatch, intake, writeback, or recovery without auto-spawning reviewers"},
		{label: "authorized-evidence", actionID: "next-batch-authorized-execution-evidence", command: "select an authorized execution evidence slice that tightens adapter report validation, repair, recording, or acknowledgement handoff"},
		{label: "adapter-live-validation", actionID: "next-batch-adapter-live-validation", command: "select an adapter live validation slice with strict authorized-gate scope and machine-readable repair evidence"},
		{label: "pack-memory-ux", actionID: "next-batch-pack-memory-ux", command: "select a pack-memory UX slice that improves candidate review, verification, cleanup, or reconsume closure without automatic mutation"},
		{label: "go-native-product-path", actionID: "next-batch-go-native-product-path", command: "select a Go-native product-path slice that reduces PowerShell-free or cross-session operational friction with Windows local validation"},
	}
	items := make([]mission.MissionCommanderNextActionItem, 0, len(domains))
	for _, domain := range domains {
		items = append(items, mission.MissionCommanderNextActionItem{
			Label:          domain.label,
			ActionID:       domain.actionID,
			State:          "next-batch-candidate-domain",
			Command:        domain.command,
			Source:         "releaseHandoffNextBatch.followUp.candidateDomain",
			Blocked:        false,
			RequiresReview: false,
			Reasons:        reasons,
			Boundary:       boundary,
		})
	}
	return items
}

func releaseHandoffNextBatchCandidateReasons(handoff ReleaseHandoff) []string {
	reasons := []string{
		"latest batch Windows local validation and implementation push cadence is complete",
		"candidate domains are offered only after release handoff is ready for next-batch selection",
	}
	packCandidates := handoff.PackMemoryCandidates
	if packCandidates.Ready && packCandidates.Total == 0 && len(packCandidates.Packs) == 0 {
		if nextAction := strings.TrimSpace(packCandidates.NextAction); nextAction != "" {
			reasons = append(reasons, "pack-memory candidate queue is closed: "+nextAction)
		} else {
			reasons = append(reasons, "pack-memory candidate queue is closed")
		}
	} else if strings.TrimSpace(packCandidates.NextAction) != "" {
		reasons = append(reasons, "pack-memory candidate queue next action: "+packCandidates.NextAction)
	}
	if latest := strings.TrimSpace(handoff.LatestBatch.BatchID); latest != "" {
		reasons = append(reasons, "latest completed batch: "+latest)
	}
	if gate := strings.TrimSpace(handoff.LatestBatch.Handoff.RemoteReleaseGate); gate != "" {
		reasons = append(reasons, "latest remote release gate: "+gate)
	}
	return mission.UniqueStrings(reasons)
}

func releaseHandoffNextBatchCandidateBoundary(handoff ReleaseHandoff) []string {
	boundary := []string{
		"candidate-domain follow-ups are selection guidance only; update docs/batch-plan.md current batch state before implementation",
		"do not execute reviewer, adapter, pack-memory, gate, or heavy-tool mutations from next-batch selection guidance",
		"choose one medium product-path closure with focused regressions plus the local release minimum",
		"do not use candidate-domain follow-ups to justify single-field, summary, or projection-only micro-batches",
	}
	boundary = append(boundary, handoff.LatestBatch.Handoff.ReleaseInspectionCadence.Boundary...)
	if detail := handoff.LatestBatch.Handoff.RemoteReleaseGateDetail; detail != nil {
		boundary = append(boundary, detail.Boundary...)
	}
	return mission.UniqueStrings(boundary)
}

func releaseHandoffNextActions() []string {
	return []string{
		"When latestBatch.handoff.releaseInspectionCadence.state=complete, select the next Windows-verifiable product-path batch without polling or waiting for remote CI.",
		"Read docs/context-routing.md first, then use releaseHandoff.signals[] to decide which detailed document is needed.",
		"Read only docs/batch-plan.md current/next/latest sections for routine continuation; search docs/batch-history.md only for old-batch archaeology.",
		"Use releaseHandoff.validation[] or gateProfile.steps[] as the local/CI minimum before tagging or handing off.",
		"Keep CHANGELOG.md Unreleased aligned with the latest completed docs/batch-plan.md batch before handing off.",
		"Continue the autonomous loop from docs/autonomous-goal.md only when goal/stop-condition detail is needed; do not paste the full goal into every handoff.",
	}
}

func releaseHandoffWarnings(handoff ReleaseHandoff) []string {
	warnings := []string{}
	warnings = append(warnings, handoff.ActiveRoute.Warnings...)
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

func releaseHandoffActiveRoute(repo string) ReleaseHandoffActiveRoute {
	const (
		routePath      = "docs/real-usage-hardening-roadmap.md"
		projectionPath = "docs/batch-plan.md"
	)
	route := ReleaseHandoffActiveRoute{Path: routePath, ProjectionPath: projectionPath}
	routeData, err := os.ReadFile(filepath.Join(repo, filepath.FromSlash(routePath)))
	if err != nil {
		return route
	}
	route.Present = true
	route.Route = markdownTableValue(routeData, "路线")
	route.CurrentBatch = markdownTableValue(routeData, "当前批次")
	route.State = strings.ToLower(markdownTableValue(routeData, "状态"))
	route.ExclusiveClaim = markdownTableValue(routeData, "唯一允许领取")
	route.NextBatch = markdownTableValue(routeData, "下一批")
	projectionData, projectionErr := os.ReadFile(filepath.Join(repo, filepath.FromSlash(projectionPath)))
	projectionRoute := ""
	projectionCurrent := ""
	projectionState := ""
	projectionClaim := ""
	projectionNext := ""
	if projectionErr == nil {
		projectionRoute = markdownTableValue(projectionData, "路线")
		projectionCurrent = markdownTableValue(projectionData, "当前批次")
		projectionState = strings.ToLower(markdownTableValue(projectionData, "状态"))
		projectionClaim = markdownTableValue(projectionData, "唯一允许领取")
		projectionNext = markdownTableValue(projectionData, "下一批")
		route.ProjectionConsistent = route.Route == projectionRoute &&
			route.CurrentBatch == projectionCurrent &&
			route.State == projectionState &&
			route.ExclusiveClaim == projectionClaim &&
			route.NextBatch == projectionNext
	}
	if route.Route == "" || route.CurrentBatch == "" || route.State == "" || route.ExclusiveClaim == "" || route.NextBatch == "" {
		route.Warnings = append(route.Warnings, "active approved route omitted route/current/state/claim/next fields")
	}
	if projectionErr != nil {
		route.Warnings = append(route.Warnings, "active approved route batch-plan projection is unavailable")
	} else if projectionRoute == "" || projectionCurrent == "" || projectionState == "" || projectionClaim == "" || projectionNext == "" {
		route.Warnings = append(route.Warnings, "active approved route batch-plan projection omitted route/current/state/claim/next fields")
	} else if !route.ProjectionConsistent {
		route.Warnings = append(route.Warnings, "active approved route and batch-plan projection disagree")
	}
	if !releaseHandoffActiveRouteStateSupported(route.State) {
		route.Warnings = append(route.Warnings, "active approved route uses unsupported state: "+route.State)
	}
	current := releaseHandoffCurrentClaim(route.CurrentBatch, route.ExclusiveClaim)
	next := releaseHandoffBatchID(route.NextBatch)
	claim := strings.TrimSpace(route.ExclusiveClaim)
	if current == "" || (route.State != "completed" && !strings.EqualFold(claim, current)) {
		route.Warnings = append(route.Warnings, "active approved route exclusive claim does not match the current batch")
	}
	if route.State == "completed" && releaseHandoffNextBatchSelectable(route.NextBatch) && (next == "" || !strings.EqualFold(claim, next)) {
		route.Warnings = append(route.Warnings, "active approved route exclusive claim does not match the exact next batch")
	}
	route.Ready = len(route.Warnings) == 0
	route.NextBatchUnlocked = route.Ready && route.ProjectionConsistent && route.State == "completed" && next != "" && strings.EqualFold(claim, next)
	if route.Ready && route.ProjectionConsistent && route.State == "completed" {
		route.CurrentAction = releaseHandoffActiveRouteValidationAction(route)
		if route.CurrentAction == nil && !releaseHandoffNextBatchSelectable(route.NextBatch) {
			route.CurrentAction = releaseHandoffCompletedRouteAction(route)
		}
	} else if !route.NextBatchUnlocked {
		route.CurrentAction = releaseHandoffActiveRouteAction(route)
		if !route.Ready || !route.ProjectionConsistent {
			route.CurrentAction.ActionID = "active-route-conflict"
			route.CurrentAction.State = "blocked-route-conflict"
			route.CurrentAction.Command = fmt.Sprintf("repair the active route projection between %s and %s before continuing or selecting another batch", route.Path, route.ProjectionPath)
			route.CurrentAction.Blocked = true
			route.CurrentAction.RequiresReview = true
			route.CurrentAction.Reasons = mission.UniqueStrings(append(route.CurrentAction.Reasons, route.Warnings...))
		}
	}
	return route
}

func markdownTableValue(data []byte, field string) string {
	for line := range strings.SplitSeq(strings.ReplaceAll(string(data), "\r\n", "\n"), "\n") {
		columns := strings.Split(strings.TrimSpace(line), "|")
		if len(columns) < 4 || strings.TrimSpace(columns[1]) != field {
			continue
		}
		return strings.TrimSpace(strings.ReplaceAll(columns[2], "`", ""))
	}
	return ""
}

func releaseHandoffBatchID(value string) string {
	value = strings.TrimSpace(value)
	if index := strings.IndexFunc(value, func(r rune) bool {
		return r == ',' || r == '，' || r == ';' || r == '；' || r == '\t' || r == '\n' || r == '\r'
	}); index >= 0 {
		value = strings.TrimSpace(value[:index])
	}
	first := ""
	second := ""
	for field := range strings.FieldsSeq(value) {
		if first == "" {
			first = field
			continue
		}
		second = field
		break
	}
	if first == "" {
		return ""
	}
	if second != "" && strings.EqualFold(first, "batch") {
		return first + " " + second
	}
	return first
}

func releaseHandoffCurrentClaim(current, claim string) string {
	claim = strings.TrimSpace(claim)
	current = strings.TrimSpace(current)
	if claim == "" || current == "" {
		return ""
	}
	if strings.EqualFold(claim, current) || strings.HasPrefix(strings.ToLower(current), strings.ToLower(claim)+" ") {
		return claim
	}
	return releaseHandoffBatchID(current)
}

func releaseHandoffActiveRouteStateSupported(state string) bool {
	switch strings.ToLower(strings.TrimSpace(state)) {
	case "in_progress", "blocked", "completed":
		return true
	default:
		return false
	}
}

func releaseHandoffNextBatchSelectable(value string) bool {
	value = strings.ToLower(strings.TrimSpace(value))
	return value != "" && !strings.HasPrefix(value, "无") && !strings.Contains(value, "deferred")
}

func releaseHandoffCompletedRouteAction(route ReleaseHandoffActiveRoute) *mission.MissionCommanderNextActionItem {
	label := releaseHandoffBatchID(route.CurrentBatch)
	if label == "" {
		label = "completed-route"
	}
	return &mission.MissionCommanderNextActionItem{
		Label:          label,
		ActionID:       "active-route-completed",
		State:          "completed-no-next-batch",
		Command:        "the approved route is complete; wait for an explicit user route change before selecting further work",
		Source:         "releaseHandoffActiveRoute",
		Blocked:        false,
		RequiresReview: false,
		Reasons: mission.UniqueStrings([]string{
			"active durable route is completed: " + route.Route,
			"no selectable next batch is recorded: " + route.NextBatch,
		}),
		Boundary: []string{
			"do not generate or consume next-batch selection guidance while the completed route has no selectable next batch",
			"deferred work remains deferred until the user explicitly changes the active route",
			"latest numbered batch handoff cannot reopen or replace this completed route",
		},
	}
}

func releaseHandoffActiveRouteAction(route ReleaseHandoffActiveRoute) *mission.MissionCommanderNextActionItem {
	currentID := strings.TrimSpace(route.ExclusiveClaim)
	if currentID == "" {
		currentID = releaseHandoffBatchID(route.CurrentBatch)
	}
	if currentID == "" {
		currentID = "active-route"
	}
	return &mission.MissionCommanderNextActionItem{
		Label:          currentID,
		ActionID:       "active-route-current-batch",
		State:          route.State,
		Command:        fmt.Sprintf("continue %s from %s; do not select another batch until its acceptance and local validation unlock %s", route.CurrentBatch, route.Path, route.NextBatch),
		Source:         "releaseHandoffActiveRoute",
		Blocked:        route.State == "blocked",
		RequiresReview: route.State == "blocked",
		Reasons: mission.UniqueStrings([]string{
			"active durable route: " + route.Route,
			"exclusive current claim: " + route.ExclusiveClaim,
			"latest numbered batch is completed evidence only and cannot replace the active route",
		}),
		Boundary: []string{
			"do not generate or consume free-form candidate-domain selection while the active route is not unlocked",
			"read the active route current card through docs/context-routing.md before implementation",
			"route and batch-plan projection disagreement is fail-closed",
		},
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
