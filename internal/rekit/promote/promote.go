package promote

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"time"

	"github.com/shuiyu486/re-context-kits/internal/rekit/doctor"
	refsf "github.com/shuiyu486/re-context-kits/internal/rekit/fs"
	"github.com/shuiyu486/re-context-kits/internal/rekit/instance"
	"github.com/shuiyu486/re-context-kits/internal/rekit/manifest"
	"github.com/shuiyu486/re-context-kits/internal/rekit/mission"
	"github.com/shuiyu486/re-context-kits/internal/rekit/review"
	"github.com/shuiyu486/re-context-kits/internal/rekit/sourceartifact"
)

type CandidateOptions struct {
	WhatIf bool
}

type CandidateArtifactOptions struct {
	ReviewOutputDir string
	PacketPath      string
	DiffPath        string
}

type ApplyOptions struct {
	WhatIf bool
}

type ApplyWrite struct {
	Path       string `json:"path"`
	Kind       string `json:"kind"`
	Action     string `json:"action"`
	SourcePath string `json:"sourcePath,omitempty"`
	TargetPath string `json:"targetPath,omitempty"`
	BackupPath string `json:"backupPath,omitempty"`
	Reason     string `json:"reason,omitempty"`
}

type ApplyResult struct {
	SchemaVersion     int          `json:"schemaVersion"`
	Command           string       `json:"command"`
	CaseRoot          string       `json:"caseRoot"`
	RepoRoot          string       `json:"repoRoot"`
	Pack              string       `json:"pack"`
	IsMutation        bool         `json:"isMutation"`
	Applied           bool         `json:"applied"`
	BackupRoot        string       `json:"backupRoot,omitempty"`
	Changed           int          `json:"changed"`
	Blocked           int          `json:"blocked"`
	Skipped           int          `json:"skipped"`
	Writes            []ApplyWrite `json:"writes"`
	ValidationRows    []doctor.Row `json:"validationRows,omitempty"`
	RequiresReview    bool         `json:"requiresReview"`
	RequiresCleanup   bool         `json:"requiresCleanup"`
	DeniedWriteAction []string     `json:"deniedWriteAction"`
	NextSteps         []string     `json:"nextSteps"`
}

type CandidateWrite struct {
	Path       string `json:"path"`
	Kind       string `json:"kind"`
	Action     string `json:"action"`
	SourcePath string `json:"sourcePath,omitempty"`
	TargetPath string `json:"targetPath,omitempty"`
	Reason     string `json:"reason,omitempty"`
}

type CandidateReviewArtifact struct {
	Path          string   `json:"path,omitempty"`
	Kind          string   `json:"kind,omitempty"`
	Name          string   `json:"name"`
	When          string   `json:"when"`
	Action        string   `json:"action"`
	CandidatePath string   `json:"candidatePath,omitempty"`
	PackTarget    string   `json:"packTarget,omitempty"`
	Format        string   `json:"format"`
	Evidence      []string `json:"evidence,omitempty"`
	Boundary      []string `json:"boundary,omitempty"`
}

type CandidateDecisionDraftCommand struct {
	Decision             string   `json:"decision"`
	When                 string   `json:"when"`
	PreviewCommand       string   `json:"previewCommand"`
	ApplyCommandTemplate string   `json:"applyCommandTemplate"`
	Expected             string   `json:"expected"`
	Boundary             []string `json:"boundary,omitempty"`
}

type CandidateDecisionDraftHandoff struct {
	Mode               string                          `json:"mode"`
	PacketPath         string                          `json:"packetPath,omitempty"`
	DecisionPath       string                          `json:"decisionPath,omitempty"`
	EvidenceRefs       []string                        `json:"evidenceRefs,omitempty"`
	DefaultReason      string                          `json:"defaultReason,omitempty"`
	DefaultActor       string                          `json:"defaultActor,omitempty"`
	SupportedDecisions []string                        `json:"supportedDecisions,omitempty"`
	PreviewCommands    []CandidateDecisionDraftCommand `json:"previewCommands,omitempty"`
	NextAction         string                          `json:"nextAction,omitempty"`
	Boundary           []string                        `json:"boundary,omitempty"`
}

type CandidateReviewNextMissingProof struct {
	Stage                     string   `json:"stage,omitempty"`
	ProofType                 string   `json:"proofType,omitempty"`
	Path                      string   `json:"path,omitempty"`
	CandidatePath             string   `json:"candidatePath,omitempty"`
	PackTarget                string   `json:"packTarget,omitempty"`
	When                      string   `json:"when,omitempty"`
	Action                    string   `json:"action,omitempty"`
	Format                    string   `json:"format,omitempty"`
	DraftCommand              string   `json:"draftCommand,omitempty"`
	DraftApplyTemplate        string   `json:"draftApplyTemplate,omitempty"`
	RequiresPacket            bool     `json:"requiresPacket,omitempty"`
	RequiresCandidateDecision bool     `json:"requiresCandidateDecision,omitempty"`
	RequiresExplicitReview    bool     `json:"requiresExplicitReview,omitempty"`
	Evidence                  []string `json:"evidence,omitempty"`
	Boundary                  []string `json:"boundary,omitempty"`
}

type CandidateReviewProofSummary struct {
	Total                    int                              `json:"total"`
	Present                  int                              `json:"present"`
	Missing                  int                              `json:"missing"`
	DecisionPresent          int                              `json:"decisionPresent"`
	DecisionMissing          int                              `json:"decisionMissing"`
	CleanupPresent           int                              `json:"cleanupPresent"`
	CleanupMissing           int                              `json:"cleanupMissing"`
	ReconsumePresent         int                              `json:"reconsumePresent"`
	ReconsumeMissing         int                              `json:"reconsumeMissing"`
	ProofRoot                string                           `json:"proofRoot,omitempty"`
	ProofProgress            string                           `json:"proofProgress,omitempty"`
	CurrentStage             string                           `json:"currentStage,omitempty"`
	NextMissingProofType     string                           `json:"nextMissingProofType,omitempty"`
	NextMissingProofPath     string                           `json:"nextMissingProofPath,omitempty"`
	NextMissingCandidatePath string                           `json:"nextMissingCandidatePath,omitempty"`
	NextMissingPackTarget    string                           `json:"nextMissingPackTarget,omitempty"`
	NextMissingProof         *CandidateReviewNextMissingProof `json:"nextMissingProof,omitempty"`
	Complete                 bool                             `json:"complete"`
	NextAction               string                           `json:"nextAction,omitempty"`
	Boundary                 []string                         `json:"boundary,omitempty"`
}

type CandidateReviewSummary struct {
	Mode                       string                      `json:"mode"`
	Pack                       string                      `json:"pack"`
	Total                      int                         `json:"total"`
	PendingReviewCount         int                         `json:"pendingReviewCount"`
	BlockedCount               int                         `json:"blockedCount"`
	NotNeededCount             int                         `json:"notNeededCount"`
	CreatedCount               int                         `json:"createdCount"`
	SkippedCount               int                         `json:"skippedCount"`
	ManagedDocCount            int                         `json:"managedDocCount"`
	ToolingCandidateCount      int                         `json:"toolingCandidateCount"`
	CleanupTargetCount         int                         `json:"cleanupTargetCount"`
	ReviewArtifactCount        int                         `json:"reviewArtifactCount"`
	DecisionChecklistCount     int                         `json:"decisionChecklistCount"`
	DecisionFollowThroughCount int                         `json:"decisionFollowThroughCount"`
	ExecutionStepCount         int                         `json:"executionStepCount"`
	ReconsumeCheckCount        int                         `json:"reconsumeCheckCount"`
	NextActionCount            int                         `json:"nextActionCount"`
	ReviewRequiredActionCount  int                         `json:"reviewRequiredActionCount"`
	CurrentAction              string                      `json:"currentAction,omitempty"`
	CandidateRoot              string                      `json:"candidateRoot"`
	ToolingRoot                string                      `json:"toolingRoot"`
	IndexPath                  string                      `json:"indexPath,omitempty"`
	RequiresReview             bool                        `json:"requiresReview"`
	RequiresCleanup            bool                        `json:"requiresCleanup"`
	HasToolingCandidate        bool                        `json:"hasToolingCandidate"`
	HasBlockedItems            bool                        `json:"hasBlockedItems"`
	HasIndex                   bool                        `json:"hasIndex"`
	HasDecisionArtifacts       bool                        `json:"hasDecisionArtifacts"`
	HasCleanupArtifacts        bool                        `json:"hasCleanupArtifacts"`
	HasReconsumeArtifacts      bool                        `json:"hasReconsumeArtifacts"`
	ProofSummary               CandidateReviewProofSummary `json:"proofSummary"`
	WhatIf                     bool                        `json:"whatIf"`
	Boundary                   []string                    `json:"boundary,omitempty"`
}

type CandidateReviewPlan struct {
	Mode                        string                                   `json:"mode"`
	Scope                       string                                   `json:"scope"`
	CandidateRoot               string                                   `json:"candidateRoot"`
	ToolingRoot                 string                                   `json:"toolingRoot"`
	IndexPath                   string                                   `json:"indexPath,omitempty"`
	ItemCount                   int                                      `json:"itemCount"`
	ReviewSummary               CandidateReviewSummary                   `json:"reviewSummary"`
	ReviewItems                 []CandidateReviewItem                    `json:"reviewItems"`
	DecisionChecklist           []CandidateDecisionChecklist             `json:"decisionChecklist"`
	DecisionFollowThrough       []CandidateDecisionFollowThrough         `json:"decisionFollowThrough"`
	CleanupTargets              []CandidateCleanupTarget                 `json:"cleanupTargets"`
	ReviewArtifacts             []CandidateReviewArtifact                `json:"reviewArtifacts,omitempty"`
	DecisionDraftHandoff        *CandidateDecisionDraftHandoff           `json:"decisionDraftHandoff,omitempty"`
	MainAgentExecutionPlan      []CandidateExecutionStep                 `json:"mainAgentExecutionPlan"`
	Reconsume                   CandidateReconsumeGuidance               `json:"reconsume"`
	MissionCommanderAction      mission.MissionCommanderAction           `json:"missionCommanderAction"`
	MissionCommanderNextActions []mission.MissionCommanderNextActionItem `json:"missionCommanderNextActions,omitempty"`
	MissionCommanderActionQueue mission.MissionCommanderActionQueue      `json:"missionCommanderActionQueue"`
	RuntimeBoundary             []string                                 `json:"runtimeBoundary"`
	CompletionCriteria          []string                                 `json:"completionCriteria"`
}

type CandidateReviewItem struct {
	Path             string   `json:"path"`
	Kind             string   `json:"kind"`
	Action           string   `json:"action"`
	ReviewDecision   string   `json:"reviewDecision"`
	CandidatePath    string   `json:"candidatePath,omitempty"`
	PackTarget       string   `json:"packTarget,omitempty"`
	MergeTargetHint  string   `json:"mergeTargetHint,omitempty"`
	RejectTargetHint string   `json:"rejectTargetHint,omitempty"`
	CleanupPath      string   `json:"cleanupPath,omitempty"`
	MainAgentActions []string `json:"mainAgentActions"`
}

type CandidateDecisionChecklist struct {
	Path                 string   `json:"path"`
	Kind                 string   `json:"kind"`
	ReviewDecision       string   `json:"reviewDecision"`
	CandidatePath        string   `json:"candidatePath,omitempty"`
	PackTarget           string   `json:"packTarget,omitempty"`
	ReviewAction         string   `json:"reviewAction"`
	AcceptActions        []string `json:"acceptActions,omitempty"`
	RejectActions        []string `json:"rejectActions,omitempty"`
	CleanupActions       []string `json:"cleanupActions,omitempty"`
	VerificationCommands []string `json:"verificationCommands,omitempty"`
	Boundary             []string `json:"boundary"`
}

type CandidateDecisionFollowThrough struct {
	Path           string                     `json:"path"`
	Kind           string                     `json:"kind"`
	ReviewDecision string                     `json:"reviewDecision"`
	CandidatePath  string                     `json:"candidatePath,omitempty"`
	PackTarget     string                     `json:"packTarget,omitempty"`
	Outcomes       []CandidateDecisionOutcome `json:"outcomes"`
	Boundary       []string                   `json:"boundary"`
}

type CandidateDecisionOutcome struct {
	Decision             string   `json:"decision"`
	State                string   `json:"state"`
	When                 string   `json:"when"`
	Actions              []string `json:"actions,omitempty"`
	CleanupActions       []string `json:"cleanupActions,omitempty"`
	VerificationCommands []string `json:"verificationCommands,omitempty"`
	Expected             string   `json:"expected"`
	Evidence             []string `json:"evidence,omitempty"`
	Boundary             []string `json:"boundary,omitempty"`
}

type CandidateCleanupTarget struct {
	Path           string   `json:"path"`
	Kind           string   `json:"kind"`
	CandidatePath  string   `json:"candidatePath"`
	IndexPath      string   `json:"indexPath,omitempty"`
	CleanupWhen    string   `json:"cleanupWhen"`
	CleanupActions []string `json:"cleanupActions"`
}

type CandidateExecutionStep struct {
	Name      string   `json:"name"`
	When      string   `json:"when"`
	AppliesTo []string `json:"appliesTo,omitempty"`
	Actions   []string `json:"actions,omitempty"`
	Commands  []string `json:"commands,omitempty"`
	Expected  string   `json:"expected"`
	Evidence  []string `json:"evidence,omitempty"`
	Boundary  []string `json:"boundary,omitempty"`
}

type CandidateReconsumeGuidance struct {
	Mode                  string                           `json:"mode"`
	ManagedDocs           string                           `json:"managedDocs"`
	Tooling               string                           `json:"tooling"`
	Commands              []string                         `json:"commands"`
	VerificationChecklist []CandidateReconsumeVerification `json:"verificationChecklist"`
	Boundary              []string                         `json:"boundary"`
}

type CandidateReconsumeVerification struct {
	Name     string   `json:"name"`
	When     string   `json:"when"`
	Commands []string `json:"commands,omitempty"`
	Expected string   `json:"expected"`
	Evidence []string `json:"evidence,omitempty"`
	Boundary []string `json:"boundary,omitempty"`
}

type CandidateResult struct {
	SchemaVersion     int                    `json:"schemaVersion"`
	Command           string                 `json:"command"`
	CaseRoot          string                 `json:"caseRoot"`
	RepoRoot          string                 `json:"repoRoot"`
	Pack              string                 `json:"pack"`
	IsMutation        bool                   `json:"isMutation"`
	Applied           bool                   `json:"applied"`
	CandidateRoot     string                 `json:"candidateRoot"`
	ToolingRoot       string                 `json:"toolingRoot"`
	IndexPath         string                 `json:"indexPath,omitempty"`
	Created           int                    `json:"created"`
	Blocked           int                    `json:"blocked"`
	Skipped           int                    `json:"skipped"`
	Writes            []CandidateWrite       `json:"writes"`
	ReviewPlan        CandidateReviewPlan    `json:"reviewPlan"`
	ReviewWorkspace   *review.ArtifactResult `json:"reviewWorkspace,omitempty"`
	RequiresReview    bool                   `json:"requiresReview"`
	RequiresCleanup   bool                   `json:"requiresCleanup"`
	DeniedWriteAction []string               `json:"deniedWriteAction"`
	NextSteps         []string               `json:"nextSteps"`
}

type CandidateReviewPacket struct {
	SchemaVersion   int             `json:"schemaVersion"`
	Kind            string          `json:"kind"`
	Command         string          `json:"command"`
	CandidateResult CandidateResult `json:"candidateResult"`
	ReviewInput     review.Plan     `json:"reviewInput"`
	Boundary        []string        `json:"boundary"`
}

type candidateIndexEntry struct {
	Path      string `json:"path"`
	Candidate string `json:"candidate"`
}

var deniedApplyActions = []string{"authority/confirmed writes", "heavy-tool execution", "tooling candidate writes"}

func Plan(repoRoot, caseRoot, pack string) (review.Plan, error) {
	if _, err := instance.AssertAttached(caseRoot, repoRoot, pack); err != nil {
		return review.Plan{}, err
	}
	m, err := manifest.Load(repoRoot, pack)
	if err != nil {
		return review.Plan{}, err
	}
	denyPatterns := append([]string{}, m.PromoteDenyPatterns...)
	denyPatterns = append(denyPatterns, caseSpecificPatterns(caseRoot)...)
	managed := map[string]bool{}
	for _, rel := range m.ManagedFiles {
		managed[rel] = true
	}
	items := []review.Item{}
	for _, rel := range m.PromoteFiles {
		if !managed[rel] {
			items = append(items, review.Item{Path: rel, Kind: "managed-doc", Direction: "case-to-kit", Action: "skip-non-managed-promote-file", RiskLevel: "none"})
			continue
		}
		caseFile, err := refsf.SafeJoin(caseRoot, rel)
		if err != nil {
			return review.Plan{}, err
		}
		packFile, err := m.SourcePath(rel)
		if err != nil {
			return review.Plan{}, err
		}
		caseText, caseExists, err := review.ReadTextIfExists(caseFile)
		if err != nil {
			return review.Plan{}, err
		}
		packText, packExists, err := review.ReadTextIfExists(packFile)
		if err != nil {
			return review.Plan{}, err
		}
		violations := []string{}
		if caseExists {
			violations = review.MatchAny(caseText, denyPatterns)
		}
		changed := !sourceTextEqual(caseText, packText)
		action := "candidate-after-llm-review"
		switch {
		case !caseExists:
			action = "skip-missing-case-file"
		case !packExists:
			action = "blocked-missing-pack-file"
		case !changed:
			action = "unchanged"
		case len(violations) > 0:
			action = "blocked-deny-pattern"
		}
		risk := "medium"
		if action == "unchanged" || strings.HasPrefix(action, "skip") {
			risk = "none"
		} else if len(violations) > 0 || strings.HasPrefix(action, "blocked") {
			risk = "high"
		}
		recommendation := "llm-review-before-merge"
		if action == "unchanged" || strings.HasPrefix(action, "skip") {
			recommendation = "skip"
		} else if len(violations) > 0 {
			recommendation = "do-not-apply-directly"
		}
		items = append(items, review.Item{Path: rel, Kind: "managed-doc", Direction: "case-to-kit", Action: action, RiskLevel: risk, CasePath: caseFile, PackPath: packFile, CaseHash: review.FileHash(caseFile), PackHash: review.FileHash(packFile), Changed: changed, DenyViolations: violations, MechanicalRecommendation: recommendation})
	}
	tooling := []review.Item{}
	for _, rel := range m.ToolingCandidateSources {
		source, err := refsf.SafeJoin(caseRoot, rel)
		if err != nil {
			return review.Plan{}, err
		}
		raw, exists, err := review.ReadTextIfExists(source)
		if err != nil {
			return review.Plan{}, err
		}
		if !exists {
			tooling = append(tooling, review.Item{Path: rel, Kind: "tooling-candidate-source", Direction: "case-to-kit", Action: "skip-missing-source", RiskLevel: "none"})
			continue
		}
		sanitized, counts := sanitizeToolingCandidate(raw, caseRoot)
		remaining := review.MatchAny(sanitized, denyPatterns)
		action := "sanitized-preview-for-llm-review"
		risk := "medium"
		recommendation := "llm-review-before-merge"
		if len(remaining) > 0 {
			action = "blocked-after-sanitization"
			risk = "high"
			recommendation = "do-not-create-candidate"
		}
		previewText := ""
		if len(remaining) == 0 {
			previewText = toolingCandidateHeader(rel) + sanitized
		}
		tooling = append(tooling, review.Item{Path: rel, Kind: "tooling-candidate-source", Direction: "case-to-kit", Action: action, RiskLevel: risk, SourcePath: source, SourceHash: review.FileHash(source), ReplacementCounts: counts, DenyViolations: remaining, MechanicalRecommendation: recommendation, SanitizedPreviewText: previewText})
	}
	return review.Plan{SchemaVersion: 1, Command: "promote", Direction: "case-to-kit", CaseRoot: caseRoot, RepoRoot: repoRoot, Pack: pack, ManifestPath: m.ManifestPath, ManifestVersion: m.Version, Items: items, ToolingItems: tooling}, nil
}

func sourceTextEqual(left, right string) bool {
	return string(sourceartifact.SemanticText([]byte(left))) == string(sourceartifact.SemanticText([]byte(right)))
}

func CreateCandidates(repoRoot, caseRoot, pack string, opt CandidateOptions) (CandidateResult, error) {
	plan, err := Plan(repoRoot, caseRoot, pack)
	if err != nil {
		return CandidateResult{}, err
	}
	m, err := manifest.Load(repoRoot, pack)
	if err != nil {
		return CandidateResult{}, err
	}
	candidateRoot := filepath.Join(m.PackRoot, "promote-candidates")
	toolingRoot := filepath.Join(m.PackRoot, "tooling", "candidates")
	if err := assertInsideRoot(m.PackRoot, candidateRoot); err != nil {
		return CandidateResult{}, err
	}
	if err := assertInsideRoot(m.PackRoot, toolingRoot); err != nil {
		return CandidateResult{}, err
	}

	stamp := time.Now().Format("20060102-150405")
	writes := []CandidateWrite{}
	index := []candidateIndexEntry{}
	created := 0
	blocked := 0
	skipped := 0
	addWrite := func(write CandidateWrite) {
		writes = append(writes, write)
		switch write.Action {
		case "create-candidate", "would-create-candidate":
			created++
		case "blocked-deny-pattern", "blocked-missing-pack-file", "blocked-after-sanitization":
			blocked++
		default:
			if strings.HasPrefix(write.Action, "skip") || write.Action == "unchanged" {
				skipped++
			}
		}
	}

	for _, item := range plan.Items {
		switch item.Action {
		case "candidate-after-llm-review":
			candidate, err := uniqueCandidatePath(candidateRoot, stamp+"_"+safeCandidateName(item.Path)+".candidate.md")
			if err != nil {
				return CandidateResult{}, err
			}
			if err := assertInsideRoot(candidateRoot, candidate); err != nil {
				return CandidateResult{}, err
			}
			action := "create-candidate"
			if opt.WhatIf {
				action = "would-create-candidate"
			} else {
				text, err := os.ReadFile(item.CasePath)
				if err != nil {
					return CandidateResult{}, err
				}
				if err := writeNewFile(candidate, text); err != nil {
					return CandidateResult{}, err
				}
			}
			index = append(index, candidateIndexEntry{Path: item.Path, Candidate: candidate})
			addWrite(CandidateWrite{Path: item.Path, Kind: item.Kind, Action: action, SourcePath: item.CasePath, TargetPath: candidate})
		case "blocked-deny-pattern", "blocked-missing-pack-file":
			addWrite(CandidateWrite{Path: item.Path, Kind: item.Kind, Action: item.Action, SourcePath: item.CasePath, TargetPath: item.PackPath, Reason: strings.Join(item.DenyViolations, ",")})
		case "skip-missing-case-file", "skip-non-managed-promote-file", "unchanged":
			addWrite(CandidateWrite{Path: item.Path, Kind: item.Kind, Action: item.Action, SourcePath: item.CasePath, TargetPath: item.PackPath})
		}
	}

	indexPath := ""
	if len(index) > 0 {
		indexPath = filepath.Join(candidateRoot, "index.json")
		if err := assertInsideRoot(candidateRoot, indexPath); err != nil {
			return CandidateResult{}, err
		}
		if !opt.WhatIf {
			b, err := json.MarshalIndent(index, "", "  ")
			if err != nil {
				return CandidateResult{}, err
			}
			if err := os.MkdirAll(filepath.Dir(indexPath), 0o755); err != nil {
				return CandidateResult{}, err
			}
			if err := os.WriteFile(indexPath, append(b, '\n'), 0o644); err != nil {
				return CandidateResult{}, err
			}
		}
	}

	for _, item := range plan.ToolingItems {
		switch item.Action {
		case "sanitized-preview-for-llm-review":
			candidate, err := uniqueCandidatePath(toolingRoot, stamp+"_tooling_"+safeCandidateName(item.Path)+".candidate.md")
			if err != nil {
				return CandidateResult{}, err
			}
			if err := assertInsideRoot(toolingRoot, candidate); err != nil {
				return CandidateResult{}, err
			}
			action := "create-candidate"
			if opt.WhatIf {
				action = "would-create-candidate"
			} else {
				if err := writeNewFile(candidate, []byte(item.SanitizedPreviewText)); err != nil {
					return CandidateResult{}, err
				}
			}
			addWrite(CandidateWrite{Path: item.Path, Kind: item.Kind, Action: action, SourcePath: item.SourcePath, TargetPath: candidate})
		case "blocked-after-sanitization":
			addWrite(CandidateWrite{Path: item.Path, Kind: item.Kind, Action: item.Action, SourcePath: item.SourcePath, Reason: strings.Join(item.DenyViolations, ",")})
		case "skip-missing-source":
			addWrite(CandidateWrite{Path: item.Path, Kind: item.Kind, Action: item.Action, SourcePath: item.SourcePath})
		}
	}

	result := CandidateResult{SchemaVersion: 1, Command: "promote", CaseRoot: plan.CaseRoot, RepoRoot: plan.RepoRoot, Pack: plan.Pack, IsMutation: !opt.WhatIf, Applied: !opt.WhatIf, CandidateRoot: candidateRoot, ToolingRoot: toolingRoot, IndexPath: indexPath, Created: created, Blocked: blocked, Skipped: skipped, Writes: writes, RequiresReview: true, RequiresCleanup: !opt.WhatIf && created > 0, DeniedWriteAction: []string{"promote -Apply", "pack managed file overwrite", "authority/confirmed writes", "heavy-tool execution"}, NextSteps: []string{"review each reviewPlan.reviewItems entry before merging", "merge accepted managed-doc candidates into pack sources only after review-first confirmation", "merge accepted tooling candidates into tooling/catalog.yml or tooling/recipes/*", "delete rejected or superseded candidate files and update or remove indexPath after review", "run doctor after accepted merges and reconsume from a fresh or attached case"}}
	result.ReviewPlan = candidateReviewPlan(result, opt.WhatIf)
	return result, nil
}

func WriteCandidateReviewWorkspace(result CandidateResult, opt CandidateArtifactOptions) (CandidateResult, error) {
	plan, err := Plan(result.RepoRoot, result.CaseRoot, result.Pack)
	if err != nil {
		return CandidateResult{}, err
	}
	workspace, err := review.WriteArtifacts(plan, review.ArtifactOptions{
		ReviewOutputDir: opt.ReviewOutputDir,
		PacketPath:      opt.PacketPath,
		DiffPath:        opt.DiffPath,
	})
	if err != nil {
		return CandidateResult{}, err
	}
	packetBytes, err := os.ReadFile(workspace.PacketPath)
	if err != nil {
		return CandidateResult{}, err
	}
	var reviewInput review.Plan
	if err := json.Unmarshal(packetBytes, &reviewInput); err != nil {
		return CandidateResult{}, fmt.Errorf("decode candidate review input: %w", err)
	}
	result.ReviewWorkspace = &workspace
	result = withCandidateDecisionDraftHandoff(result, reviewInput, workspace)
	packetResult := result
	packet := CandidateReviewPacket{
		SchemaVersion:   1,
		Kind:            "pack-memory-candidate-review",
		Command:         "promote",
		CandidateResult: packetResult,
		ReviewInput:     reviewInput,
		Boundary: []string{
			"review workspace records candidate review context, bounded diffs, sanitized previews, and candidate decision draft handoff only",
			"review workspace does not merge or delete candidates, update pack sources, run doctor/init/reconsume, or create cleanup/reconsume proof files",
			"candidate decision draft handoff only previews or writes a case-local decision JSON after explicit WhatIf hash review",
			"review workspace does not write authority/confirmed or execute heavy tools",
		},
	}
	encoded, err := json.MarshalIndent(packet, "", "  ")
	if err != nil {
		return CandidateResult{}, err
	}
	if err := os.WriteFile(workspace.PacketPath, append(encoded, '\n'), 0o644); err != nil {
		return CandidateResult{}, err
	}
	if err := os.WriteFile(workspace.SummaryPath, []byte(candidateReviewWorkspaceSummary(result, reviewInput, workspace)), 0o644); err != nil {
		return CandidateResult{}, err
	}
	return result, nil
}

func candidateReviewWorkspaceSummary(result CandidateResult, plan review.Plan, workspace review.ArtifactResult) string {
	summary := result.ReviewPlan.ReviewSummary
	lines := []string{
		"# rekit pack-memory candidate review workspace",
		"",
		"## 执行摘要",
		"",
		fmt.Sprintf("- pack: %s", result.Pack),
		fmt.Sprintf("- candidates: created=%d blocked=%d skipped=%d", result.Created, result.Blocked, result.Skipped),
		fmt.Sprintf("- review: pending=%d artifacts=%d proof=%s stage=%s", summary.PendingReviewCount, summary.ReviewArtifactCount, summary.ProofSummary.ProofProgress, summary.ProofSummary.CurrentStage),
		fmt.Sprintf("- packet: %s", workspace.PacketPath),
		fmt.Sprintf("- combined diff: %s", workspace.CombinedDiffPath),
		"",
		"## 执行清单",
		"",
		"1. Read packet.json candidateResult.reviewPlan and reviewInput before choosing candidate decisions.",
		"2. Inspect bounded diffs and sanitized previews under this workspace.",
		"3. Use decisionDraftHandoff to run a Go-native draft preview, inspect decisionSha256, then use the returned expected-hash Apply command if the reviewed decisions are correct.",
		"4. Follow only the selected decisionFollowThrough outcome; cleanup and reconsume remain explicit main-Agent actions.",
		"",
		"## 验证标准",
		"",
		fmt.Sprintf("- review input changed/planned items: %d", plan.Summary.Changed),
		fmt.Sprintf("- review input blocked items: %d", plan.Summary.Blocked),
		fmt.Sprintf("- next missing proof: %s", summary.ProofSummary.NextMissingProofPath),
	}
	if handoff := result.ReviewPlan.DecisionDraftHandoff; handoff != nil {
		lines = append(lines,
			fmt.Sprintf("- decision draft handoff: mode=%s decisionPath=%s", handoff.Mode, handoff.DecisionPath),
			fmt.Sprintf("- decision draft next action: %s", handoff.NextAction),
		)
		for _, command := range handoff.PreviewCommands {
			lines = append(lines,
				fmt.Sprintf("- decision draft preview (%s): %s", command.Decision, command.PreviewCommand),
				fmt.Sprintf("- decision draft apply template (%s): %s", command.Decision, command.ApplyCommandTemplate),
			)
		}
	}
	lines = append(lines,
		"",
		"## 风险与注意事项",
		"",
		"- This workspace is review evidence plus decision draft handoff only; it does not merge or delete candidates, update pack sources, run doctor/init/reconsume, or create cleanup/reconsume proof files.",
		"- Draft handoff can only preview or write one case-local decision JSON after explicit WhatIf hash review; it does not write authority/confirmed or execute heavy tools.",
		"- Do not copy case-specific samples, traces, dumps, captures, payloads, flags, customer data, or absolute case paths into pack sources.",
	)
	return strings.Join(lines, "\r\n") + "\r\n"
}

func withCandidateDecisionDraftHandoff(result CandidateResult, plan review.Plan, workspace review.ArtifactResult) CandidateResult {
	handoff := candidateDecisionDraftHandoff(result, plan, workspace)
	if handoff == nil {
		return result
	}
	result.ReviewPlan.DecisionDraftHandoff = handoff
	result.ReviewPlan.MainAgentExecutionPlan = append([]CandidateExecutionStep{candidateDecisionDraftExecutionStep(*handoff)}, result.ReviewPlan.MainAgentExecutionPlan...)
	draftAction := candidateDecisionDraftNextAction(*handoff)
	if len(result.ReviewPlan.MissionCommanderNextActions) == 0 {
		result.ReviewPlan.MissionCommanderNextActions = []mission.MissionCommanderNextActionItem{draftAction}
	} else {
		updated := append([]mission.MissionCommanderNextActionItem{}, result.ReviewPlan.MissionCommanderNextActions[:1]...)
		updated = append(updated, draftAction)
		updated = append(updated, result.ReviewPlan.MissionCommanderNextActions[1:]...)
		result.ReviewPlan.MissionCommanderNextActions = updated
	}
	result.ReviewPlan.MissionCommanderNextActions = mission.UniqueCommanderNextActions(result.ReviewPlan.MissionCommanderNextActions)
	result.ReviewPlan.MissionCommanderActionQueue = mission.MissionCommanderActionQueueFor(result.ReviewPlan.MissionCommanderNextActions)
	result.ReviewPlan.MissionCommanderAction = candidateDecisionDraftCommanderAction(result.ReviewPlan.MissionCommanderAction, *handoff)
	result.ReviewPlan.ReviewSummary = CandidateReviewSummaryFor(result, result.ReviewPlan, !result.IsMutation)
	return result
}

func candidateDecisionDraftHandoff(result CandidateResult, plan review.Plan, workspace review.ArtifactResult) *CandidateDecisionDraftHandoff {
	if candidatePendingReviewCount(result.ReviewPlan.ReviewItems) == 0 {
		return nil
	}
	evidenceRefs := candidateDecisionDraftHandoffEvidenceRefs(plan, workspace)
	handoff := CandidateDecisionDraftHandoff{
		Mode:               "candidate-decision-draft-handoff",
		PacketPath:         workspace.PacketPath,
		DecisionPath:       filepath.Join(workspace.ReviewRoot, "candidate-decisions.json"),
		EvidenceRefs:       evidenceRefs,
		DefaultReason:      "reviewed durable candidate packet, bounded diff, and sanitized previews",
		DefaultActor:       "mission-commander",
		SupportedDecisions: candidateDecisionDraftSupportedDecisions(result.ReviewPlan.ReviewItems),
		Boundary: []string{
			"run the draft preview after reviewing packet.json, bounded diffs, sanitized previews, and evidence refs",
			"WhatIf returns decisionSha256; Apply must use the exact returned -ExpectedDecisionSha256",
			"draft Apply writes only the case-local decisionPath JSON and is exact replay only",
			"drafting does not merge candidates, cleanup candidate files, run doctor/init/reconsume, write authority/confirmed, or execute heavy tools",
		},
	}
	if !result.IsMutation {
		handoff.Mode = "candidate-decision-draft-after-materialize"
		handoff.NextAction = "rerun promote -CreateCandidates -Review without -WhatIf before using decision draft commands; preview candidates do not exist yet"
		handoff.Boundary = append([]string{"WhatIf did not create candidatePath bytes; DraftCandidateDecision requires materialized candidates"}, handoff.Boundary...)
		return &handoff
	}
	if len(evidenceRefs) == 0 {
		handoff.NextAction = "choose at least one non-empty case-local or repo-local evidence ref before running promote -DraftCandidateDecision"
		return &handoff
	}
	for _, decision := range handoff.SupportedDecisions {
		preview := candidateDecisionDraftCommand(handoff.PacketPath, handoff.DecisionPath, decision, handoff.DefaultReason, handoff.DefaultActor, strings.Join(handoff.EvidenceRefs, ","), "", false)
		apply := candidateDecisionDraftCommand(handoff.PacketPath, handoff.DecisionPath, decision, handoff.DefaultReason, handoff.DefaultActor, strings.Join(handoff.EvidenceRefs, ","), "<decisionSha256-from-WhatIf>", true)
		handoff.PreviewCommands = append(handoff.PreviewCommands, CandidateDecisionDraftCommand{
			Decision:             decision,
			When:                 candidateDecisionDraftCommandWhen(decision),
			PreviewCommand:       preview,
			ApplyCommandTemplate: apply,
			Expected:             "inspect generated decisions and decisionSha256 before replacing the placeholder in the Apply template",
			Boundary:             append([]string{}, handoff.Boundary...),
		})
	}
	if len(handoff.PreviewCommands) > 0 {
		handoff.NextAction = handoff.PreviewCommands[0].PreviewCommand
	}
	return &handoff
}

func candidateDecisionDraftHandoffEvidenceRefs(plan review.Plan, workspace review.ArtifactResult) []string {
	candidates := []string{workspace.CombinedDiffPath, workspace.SummaryPath}
	for _, item := range plan.Items {
		candidates = append(candidates, item.DiffPath)
	}
	for _, item := range plan.ToolingItems {
		candidates = append(candidates, item.SanitizedPreviewPath)
	}
	out := []string{}
	seen := map[string]bool{}
	for _, path := range candidates {
		path = strings.TrimSpace(path)
		if path == "" || seen[filepath.Clean(path)] {
			continue
		}
		info, err := os.Stat(path)
		if err != nil || info.IsDir() || info.Size() == 0 {
			continue
		}
		seen[filepath.Clean(path)] = true
		out = append(out, path)
	}
	return out
}

func candidateDecisionDraftSupportedDecisions(items []CandidateReviewItem) []string {
	hasTooling := false
	for _, item := range items {
		if item.ReviewDecision == "pending-review" && item.Kind == "tooling-candidate-source" {
			hasTooling = true
			break
		}
	}
	if hasTooling {
		return []string{"accept-managed-reject-tooling", "reject", "superseded"}
	}
	return []string{"accept", "accept-managed-reject-tooling", "reject", "superseded"}
}

func candidateDecisionDraftCommandWhen(decision string) string {
	switch decision {
	case "accept":
		return "after every pending candidate is a reviewed managed-doc candidate that should be accepted"
	case "accept-managed-reject-tooling":
		return "after managed-doc candidates should be accepted and tooling candidates should remain manual or rejected"
	case "reject":
		return "after every pending candidate should be rejected"
	case "superseded":
		return "after every pending candidate is replaced by newer reviewed content"
	default:
		return "after reviewing all pending candidates"
	}
}

func candidatePendingReviewCount(items []CandidateReviewItem) int {
	count := 0
	for _, item := range items {
		if item.ReviewDecision == "pending-review" {
			count++
		}
	}
	return count
}

func candidateDecisionDraftExecutionStep(handoff CandidateDecisionDraftHandoff) CandidateExecutionStep {
	commands := []string{}
	for _, command := range handoff.PreviewCommands {
		commands = append(commands, command.PreviewCommand, command.ApplyCommandTemplate)
	}
	return CandidateExecutionStep{
		Name:     "draft-candidate-decision-file",
		When:     "after reviewing candidate review packet, bounded diffs, sanitized previews, and before candidate merge/cleanup",
		Commands: commands,
		Expected: "Go-native draft preview returns decisionSha256 and the expected-hash Apply writes only decisionPath",
		Evidence: append([]string{"decisionDraftHandoff packetPath/decisionPath/evidenceRefs", "draft preview decisionSha256"}, handoff.EvidenceRefs...),
		Boundary: append([]string{}, handoff.Boundary...),
	}
}

func candidateDecisionDraftNextAction(handoff CandidateDecisionDraftHandoff) mission.MissionCommanderNextActionItem {
	return mission.MissionCommanderNextActionItem{
		State:          "pack-memory-candidates:decision-draft-preview",
		Command:        handoff.NextAction,
		Source:         "reviewPlan.decisionDraftHandoff",
		RequiresReview: true,
		Reasons:        []string{"use the Go-native candidate decision draft path instead of hand-authoring packet/candidate/target/evidence hashes", "inspect decisionSha256 before running the expected-hash Apply template"},
		Boundary:       append([]string{}, handoff.Boundary...),
	}
}

func candidateDecisionDraftCommanderAction(base mission.MissionCommanderAction, handoff CandidateDecisionDraftHandoff) mission.MissionCommanderAction {
	commands := append([]string{}, base.FollowUpCommands...)
	for _, command := range handoff.PreviewCommands {
		commands = append(commands, command.PreviewCommand)
	}
	if len(handoff.PreviewCommands) == 0 && strings.TrimSpace(handoff.NextAction) != "" {
		commands = append(commands, handoff.NextAction)
	}
	base.FollowUpCommands = mission.UniqueStrings(commands)
	base.Boundary = mission.UniqueStrings(append(base.Boundary, handoff.Boundary...))
	if strings.TrimSpace(base.Prompt) != "" {
		base.Prompt += " decisionDraftHandoff 提供 Go-native decision file preview/apply；不要手写 packet/candidate/evidence hash。"
	}
	return base
}

func Apply(repoRoot, caseRoot, pack string, opt ApplyOptions) (ApplyResult, error) {
	plan, err := Plan(repoRoot, caseRoot, pack)
	if err != nil {
		return ApplyResult{}, err
	}
	m, err := manifest.Load(repoRoot, pack)
	if err != nil {
		return ApplyResult{}, err
	}
	backupRoot, err := uniqueBackupRoot(filepath.Join(m.PackRoot, "promote-candidates", ".backup"))
	if err != nil {
		return ApplyResult{}, err
	}
	if err := assertInsideRoot(m.PackRoot, backupRoot); err != nil {
		return ApplyResult{}, err
	}

	writes := []ApplyWrite{}
	changed := 0
	blocked := 0
	skipped := 0
	addWrite := func(write ApplyWrite) {
		writes = append(writes, write)
		switch write.Action {
		case "promote", "would-promote":
			changed++
		case "blocked-deny-pattern", "blocked-missing-pack-file":
			blocked++
		default:
			if strings.HasPrefix(write.Action, "skip") || write.Action == "unchanged" {
				skipped++
			}
		}
	}
	restoreFailure := func(err error) (ApplyResult, error) {
		if !opt.WhatIf && len(writes) > 0 {
			if restoreErr := restorePromoteBackups(writes); restoreErr != nil {
				return ApplyResult{}, fmt.Errorf("%w; restore failed: %v", err, restoreErr)
			}
			return ApplyResult{}, fmt.Errorf("%w; pack files restored from backup", err)
		}
		return ApplyResult{}, err
	}

	for _, item := range plan.Items {
		switch item.Action {
		case "candidate-after-llm-review":
			if err := assertInsideRoot(m.PackRoot, item.PackPath); err != nil {
				return restoreFailure(err)
			}
			backupPath, err := packBackupPath(item.PackPath, m.PackRoot, backupRoot)
			if err != nil {
				return restoreFailure(err)
			}
			action := "promote"
			if opt.WhatIf {
				action = "would-promote"
				addWrite(ApplyWrite{Path: item.Path, Kind: item.Kind, Action: action, SourcePath: item.CasePath, TargetPath: item.PackPath, BackupPath: backupPath})
				continue
			}
			text, err := os.ReadFile(item.CasePath)
			if err != nil {
				return restoreFailure(err)
			}
			if err := backupPackFile(item.PackPath, m.PackRoot, backupRoot); err != nil {
				return restoreFailure(err)
			}
			addWrite(ApplyWrite{Path: item.Path, Kind: item.Kind, Action: action, SourcePath: item.CasePath, TargetPath: item.PackPath, BackupPath: backupPath})
			if err := os.MkdirAll(filepath.Dir(item.PackPath), 0o755); err != nil {
				return restoreFailure(err)
			}
			if err := os.WriteFile(item.PackPath, text, 0o644); err != nil {
				return restoreFailure(err)
			}
		case "blocked-deny-pattern", "blocked-missing-pack-file":
			addWrite(ApplyWrite{Path: item.Path, Kind: item.Kind, Action: item.Action, SourcePath: item.CasePath, TargetPath: item.PackPath, Reason: strings.Join(item.DenyViolations, ",")})
		case "skip-missing-case-file", "skip-non-managed-promote-file", "unchanged":
			addWrite(ApplyWrite{Path: item.Path, Kind: item.Kind, Action: item.Action, SourcePath: item.CasePath, TargetPath: item.PackPath})
		}
	}

	result := ApplyResult{SchemaVersion: 1, Command: "promote", CaseRoot: plan.CaseRoot, RepoRoot: plan.RepoRoot, Pack: plan.Pack, IsMutation: !opt.WhatIf, Applied: !opt.WhatIf, BackupRoot: backupRoot, Changed: changed, Blocked: blocked, Skipped: skipped, Writes: writes, RequiresReview: true, RequiresCleanup: !opt.WhatIf && changed > 0, DeniedWriteAction: deniedApplyActions, NextSteps: []string{"run doctor after apply", "review backupRoot if any promoted pack file must be restored"}}
	if opt.WhatIf {
		result.BackupRoot = ""
		result.NextSteps = []string{"review would-promote entries before rerunning with -Apply"}
		return result, nil
	}

	rows, err := doctor.Pack(repoRoot, pack)
	if err != nil {
		if restoreErr := restorePromoteBackups(writes); restoreErr != nil {
			return result, fmt.Errorf("pack validation failed after promote apply: %w; restore failed: %v", err, restoreErr)
		}
		return result, fmt.Errorf("pack validation failed after promote apply; pack files restored from backup: %w", err)
	}
	result.ValidationRows = rows
	return result, nil
}

func candidateReviewPlan(result CandidateResult, whatIf bool) CandidateReviewPlan {
	mode := "candidate-review"
	if whatIf {
		mode = "candidate-review-preview"
	}
	plan := CandidateReviewPlan{
		Mode:          mode,
		Scope:         "review generated managed-doc and tooling candidates, choose merge or reject per item, then clean rejected candidates and verify reconsume",
		CandidateRoot: result.CandidateRoot,
		ToolingRoot:   result.ToolingRoot,
		IndexPath:     result.IndexPath,
		RuntimeBoundary: []string{
			"create-candidates writes only promote-candidates and tooling/candidates when not WhatIf",
			"reviewPlan does not merge candidates into pack sources",
			"tooling candidates require manual catalog or recipe merge before reuse",
			"reviewArtifacts identifies expected decision/cleanup/reconsume evidence and deterministic draft handoffs only; create-candidates does not write proof files",
			"mainAgentExecutionPlan and decisionFollowThrough are guidance only; runtime does not execute merge, cleanup, init, or doctor commands",
			"decisionChecklist describes review options; decisionFollowThrough maps accepted/rejected/superseded outcomes to follow-through steps for the main Agent",
			"no authority/confirmed writes",
			"no heavy-tool execution",
		},
		CompletionCriteria: []string{
			"each created candidate has an explicit accept, reject, or superseded decision before using decisionFollowThrough outcomes",
			"accepted managed docs are merged only after reviewing the candidate against current packTarget; promote -Apply is not a candidate-scoped accept path",
			"accepted tooling candidates are merged into tooling/catalog.yml or tooling/recipes/* and validated with doctor plus fresh or attached case reconsume",
			"rejected or superseded candidate files are deleted and indexPath is updated or removed",
			"fresh or attached case reconsume path is verified after accepted tooling merges using reconsume.verificationChecklist",
		},
		Reconsume: CandidateReconsumeGuidance{
			Mode:        "pack-memory-reconsume-after-merge",
			ManagedDocs: "accepted managed-doc candidates become reusable only after manual merge into the pack source; promote -Apply remains a separately reviewed whole-plan writeback, not a candidate-scoped accept path",
			Tooling:     "accepted tooling candidates become reusable only after a human merges them into tooling/catalog.yml or tooling/recipes/*; verify reconsume from a fresh case or attached case because sync does not copy tooling recipes into case-local managed docs",
			Commands: []string{
				"go run ./cmd/rekit -- -Command doctor -Pack " + result.Pack,
				"go run ./cmd/rekit -- -Command init -Target <fresh-case> -Pack " + result.Pack + " -ProjectName <name> -Apply",
				"go run ./cmd/rekit -- -Command doctor -Target <fresh-case> -Pack " + result.Pack,
			},
			VerificationChecklist: []CandidateReconsumeVerification{
				{
					Name:     "pack-doctor-after-merge",
					When:     "after accepting any managed-doc or tooling candidate into pack sources",
					Commands: []string{"go run ./cmd/rekit -- -Command doctor -Pack " + result.Pack},
					Expected: "pack doctor passes with merged reusable content and no case-specific residue",
					Evidence: []string{"doctor command output"},
				},
				{
					Name:     "fresh-case-reconsume",
					When:     "after accepting any tooling candidate into tooling/catalog.yml or tooling/recipes/*",
					Commands: []string{"go run ./cmd/rekit -- -Command init -Target <fresh-case> -Pack " + result.Pack + " -ProjectName <name> -Apply", "go run ./cmd/rekit -- -Command doctor -Target <fresh-case> -Pack " + result.Pack},
					Expected: "fresh case binds templateRoot/templatePack and doctor passes while tooling remains pack-sourced",
					Evidence: []string{"fresh case instance metadata", "fresh case doctor output"},
					Boundary: []string{"use a temporary fresh case only; do not create real case state in the kit repo"},
				},
				{
					Name:     "attached-case-reconsume",
					When:     "when validating an existing attached case after accepted tooling merges",
					Commands: []string{"go run ./cmd/rekit -- -Command doctor -Target <attached-case> -Pack " + result.Pack},
					Expected: "attached case resolves pack tooling through templateRoot/templatePack without copying tooling recipes into managed docs",
					Evidence: []string{"attached case doctor output"},
					Boundary: []string{"do not overwrite case-local files while checking reconsume"},
				},
			},
			Boundary: []string{
				"do not promote case-local samples, traces, dumps, captures, artifacts, payloads, flags, or customer data",
				"fresh case reads pack tooling through templateRoot/templatePack; no tooling recipe copy is expected in case-local managed docs",
			},
		},
	}
	cleanupWhen := "reject, superseded, or merged into a different pack source; then update or remove indexPath if it references this candidate"
	if whatIf {
		cleanupWhen = "if rerun without WhatIf creates this candidate, delete it when rejected, superseded, or merged elsewhere and update or remove indexPath"
	}
	for _, write := range result.Writes {
		item := candidateReviewItem(result, write)
		plan.ReviewItems = append(plan.ReviewItems, item)
		plan.DecisionChecklist = append(plan.DecisionChecklist, candidateDecisionChecklist(result, item, whatIf))
		plan.DecisionFollowThrough = append(plan.DecisionFollowThrough, candidateDecisionFollowThrough(result, item, whatIf))
		plan.ReviewArtifacts = append(plan.ReviewArtifacts, candidateReviewArtifacts(result, item, whatIf)...)
		if item.CleanupPath != "" {
			plan.CleanupTargets = append(plan.CleanupTargets, CandidateCleanupTarget{Path: write.Path, Kind: write.Kind, CandidatePath: item.CleanupPath, IndexPath: result.IndexPath, CleanupWhen: cleanupWhen, CleanupActions: candidateCleanupActions(result, item, whatIf)})
		}
	}
	plan.ItemCount = len(plan.ReviewItems)
	plan.MainAgentExecutionPlan = candidateMainAgentExecutionPlan(result, plan, whatIf)
	plan.MissionCommanderAction = candidateReviewMissionCommanderAction(result, whatIf)
	plan.MissionCommanderNextActions = candidateMissionCommanderNextActions(result, plan, whatIf)
	plan.MissionCommanderActionQueue = mission.MissionCommanderActionQueueFor(plan.MissionCommanderNextActions)
	plan.ReviewSummary = CandidateReviewSummaryFor(result, plan, whatIf)
	return plan
}

func CandidateReviewSummaryFor(result CandidateResult, plan CandidateReviewPlan, whatIf bool) CandidateReviewSummary {
	summary := CandidateReviewSummary{
		Mode:                       plan.Mode,
		Pack:                       result.Pack,
		Total:                      len(plan.ReviewItems),
		CreatedCount:               result.Created,
		BlockedCount:               result.Blocked,
		SkippedCount:               result.Skipped,
		CleanupTargetCount:         len(plan.CleanupTargets),
		ReviewArtifactCount:        len(plan.ReviewArtifacts),
		DecisionChecklistCount:     len(plan.DecisionChecklist),
		DecisionFollowThroughCount: len(plan.DecisionFollowThrough),
		ExecutionStepCount:         len(plan.MainAgentExecutionPlan),
		ReconsumeCheckCount:        len(plan.Reconsume.VerificationChecklist),
		NextActionCount:            len(plan.MissionCommanderNextActions),
		CandidateRoot:              plan.CandidateRoot,
		ToolingRoot:                plan.ToolingRoot,
		IndexPath:                  plan.IndexPath,
		RequiresReview:             result.RequiresReview,
		RequiresCleanup:            result.RequiresCleanup,
		HasIndex:                   strings.TrimSpace(plan.IndexPath) != "",
		ProofSummary:               candidateReviewProofSummary(result, plan, whatIf),
		WhatIf:                     whatIf,
	}
	for _, item := range plan.ReviewItems {
		switch strings.TrimSpace(item.ReviewDecision) {
		case "pending-review":
			summary.PendingReviewCount++
		case "blocked":
			summary.HasBlockedItems = true
		case "not-needed":
			summary.NotNeededCount++
		}
		switch strings.TrimSpace(item.Kind) {
		case "managed-doc":
			summary.ManagedDocCount++
		case "tooling-candidate-source":
			summary.ToolingCandidateCount++
			summary.HasToolingCandidate = true
		}
	}
	for _, artifact := range plan.ReviewArtifacts {
		switch strings.TrimSpace(artifact.Name) {
		case "candidate-decision-note", "blocked-review-note":
			summary.HasDecisionArtifacts = true
		case "candidate-cleanup-proof":
			summary.HasCleanupArtifacts = true
		case "pack-doctor-output", "fresh-case-reconsume-proof", "attached-case-reconsume-proof":
			summary.HasReconsumeArtifacts = true
		}
	}
	for _, action := range plan.MissionCommanderNextActions {
		if action.RequiresReview {
			summary.ReviewRequiredActionCount++
		}
	}
	if plan.MissionCommanderActionQueue.CurrentAction != nil {
		summary.CurrentAction = plan.MissionCommanderActionQueue.CurrentAction.Command
	}
	if summary.Total > 0 || summary.HasIndex {
		summary.Boundary = candidateReviewSummaryBoundary(whatIf)
	}
	return summary
}

func candidateReviewSummaryBoundary(whatIf bool) []string {
	boundary := []string{
		"reviewSummary is read-only; full reviewPlan arrays remain available",
		"reviewSummary does not merge candidates, cleanup candidate files, run doctor/init, or create review artifacts",
		"reviewSummary preserves review-first pack-memory flow and does not write authority/confirmed or execute heavy tools",
	}
	if whatIf {
		boundary = append([]string{"WhatIf did not write candidate files or indexPath"}, boundary...)
	}
	return boundary
}

func candidateReviewProofSummary(result CandidateResult, plan CandidateReviewPlan, whatIf bool) CandidateReviewProofSummary {
	summary := CandidateReviewProofSummary{
		Total:     len(plan.ReviewArtifacts),
		ProofRoot: filepath.ToSlash(filepath.Join("packs", result.Pack, "promote-candidates", "review-artifacts")),
		Complete:  len(plan.ReviewArtifacts) == 0,
	}
	missingByStage := map[string]CandidateReviewArtifact{}
	for _, artifact := range plan.ReviewArtifacts {
		stage := candidateReviewProofArtifactStage(artifact.Name)
		switch stage {
		case "decision-proof-required":
			summary.DecisionMissing++
			if _, ok := missingByStage[stage]; !ok {
				missingByStage[stage] = artifact
			}
		case "cleanup-proof-required":
			summary.CleanupMissing++
			if _, ok := missingByStage[stage]; !ok {
				missingByStage[stage] = artifact
			}
		case "reconsume-proof-required":
			summary.ReconsumeMissing++
			if _, ok := missingByStage[stage]; !ok {
				missingByStage[stage] = artifact
			}
		}
	}
	if summary.Total > 0 {
		summary.Missing = summary.DecisionMissing + summary.CleanupMissing + summary.ReconsumeMissing
		summary.Present = summary.Total - summary.Missing
		summary.ProofProgress = fmt.Sprintf("%d/%d", summary.Present, summary.Total)
		summary.CurrentStage = candidateReviewProofStage(summary)
		if artifact, ok := missingByStage[summary.CurrentStage]; ok {
			summary.NextMissingProofType = artifact.Name
			summary.NextMissingProofPath = candidateReviewNextExpectedProof(summary.ProofRoot, artifact)
			summary.NextMissingCandidatePath = artifact.CandidatePath
			summary.NextMissingPackTarget = artifact.PackTarget
			nextMissingProof := candidateReviewNextMissingProof(summary.CurrentStage, summary.NextMissingProofPath, artifact)
			summary.NextMissingProof = &nextMissingProof
		}
		summary.Complete = summary.Missing == 0
		if summary.Missing > 0 {
			summary.NextAction = "record expected pack-memory review proof: " + summary.NextMissingProofType + " at " + summary.NextMissingProofPath + " for " + summary.NextMissingCandidatePath
		} else {
			summary.NextAction = "no pack-memory review proof is required"
		}
		summary.Boundary = candidateReviewProofSummaryBoundary(whatIf)
	}
	return summary
}

func candidateReviewProofArtifactStage(name string) string {
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

func candidateReviewProofStage(summary CandidateReviewProofSummary) string {
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

func candidateReviewNextExpectedProof(proofRoot string, artifact CandidateReviewArtifact) string {
	stem := candidateReviewProofStem(artifact.CandidatePath, artifact.PackTarget)
	return filepath.ToSlash(filepath.Join(proofRoot, stem+"."+artifact.Name+".md"))
}

func candidateReviewNextMissingProof(stage, proofPath string, artifact CandidateReviewArtifact) CandidateReviewNextMissingProof {
	proof := CandidateReviewNextMissingProof{
		Stage:         stage,
		ProofType:     artifact.Name,
		Path:          proofPath,
		CandidatePath: artifact.CandidatePath,
		PackTarget:    artifact.PackTarget,
		When:          artifact.When,
		Action:        artifact.Action,
		Format:        artifact.Format,
		Evidence:      append([]string{}, artifact.Evidence...),
		Boundary:      append([]string{}, artifact.Boundary...),
	}
	if artifact.Name == "candidate-decision-note" && strings.TrimSpace(artifact.CandidatePath) != "" {
		proof.RequiresPacket = true
		proof.RequiresExplicitReview = true
		proof.DraftCommand = "/rekit promote -PacketPath <packet.json> -DraftReviewProof -ProofPath " + quoteCandidateDecisionArg(proofPath) + " -ProofType candidate-decision-note -CandidatePath " + quoteCandidateDecisionArg(artifact.CandidatePath) + " -ProofDecision <accept|reject|superseded> -Reason <reviewed-reason> -Actor <actor> -EvidenceRefs <review-evidence-ref> -WhatIf -Format json"
		proof.DraftApplyTemplate = "/rekit promote -PacketPath <packet.json> -DraftReviewProof -ProofPath " + quoteCandidateDecisionArg(proofPath) + " -ProofType candidate-decision-note -CandidatePath " + quoteCandidateDecisionArg(artifact.CandidatePath) + " -ProofDecision <accept|reject|superseded> -Reason <reviewed-reason> -Actor <actor> -EvidenceRefs <review-evidence-ref> -ExpectedProofSha256 <proofSha256-from-WhatIf> -Apply -Format json"
	}
	if artifact.Name == "candidate-cleanup-proof" && strings.TrimSpace(artifact.CandidatePath) != "" {
		proof.RequiresPacket = true
		proof.RequiresCandidateDecision = true
		proof.DraftCommand = "/rekit promote -PacketPath <packet.json> -CandidateDecisionPath <candidate-decisions.json> -DraftReviewProof -ProofPath " + quoteCandidateDecisionArg(proofPath) + " -ProofType candidate-cleanup-proof -CandidatePath " + quoteCandidateDecisionArg(artifact.CandidatePath) + " -Reason <cleanup-proof-reason> -Actor <actor> -EvidenceRefs <cleanup-evidence-ref> -WhatIf -Format json"
		proof.DraftApplyTemplate = "/rekit promote -PacketPath <packet.json> -CandidateDecisionPath <candidate-decisions.json> -DraftReviewProof -ProofPath " + quoteCandidateDecisionArg(proofPath) + " -ProofType candidate-cleanup-proof -CandidatePath " + quoteCandidateDecisionArg(artifact.CandidatePath) + " -Reason <cleanup-proof-reason> -Actor <actor> -EvidenceRefs <cleanup-evidence-ref> -ExpectedProofSha256 <proofSha256-from-WhatIf> -Apply -Format json"
	}
	if candidateReviewLifecycleProofType(artifact.Name) && strings.TrimSpace(artifact.CandidatePath) != "" {
		proof.RequiresPacket = true
		proof.DraftCommand = "/rekit promote -PacketPath <packet.json> -DraftReviewProof -ProofPath " + quoteCandidateDecisionArg(proofPath) + " -ProofType " + quoteCandidateDecisionArg(artifact.Name) + " -CandidatePath " + quoteCandidateDecisionArg(artifact.CandidatePath) + " -Reason <lifecycle-proof-reason> -Actor <actor> -EvidenceRefs <repo-local-lifecycle-evidence-ref> -WhatIf -Format json"
		proof.DraftApplyTemplate = "/rekit promote -PacketPath <packet.json> -DraftReviewProof -ProofPath " + quoteCandidateDecisionArg(proofPath) + " -ProofType " + quoteCandidateDecisionArg(artifact.Name) + " -CandidatePath " + quoteCandidateDecisionArg(artifact.CandidatePath) + " -Reason <lifecycle-proof-reason> -Actor <actor> -EvidenceRefs <repo-local-lifecycle-evidence-ref> -ExpectedProofSha256 <proofSha256-from-WhatIf> -Apply -Format json"
	}
	return proof
}

func candidateReviewLifecycleProofType(proofType string) bool {
	switch strings.TrimSpace(proofType) {
	case "pack-doctor-output", "fresh-case-reconsume-proof", "attached-case-reconsume-proof":
		return true
	default:
		return false
	}
}

func candidateReviewProofStem(candidatePath, packTarget string) string {
	base := filepath.Base(filepath.FromSlash(strings.TrimSpace(candidatePath)))
	if base == "." || base == string(filepath.Separator) || base == "" {
		base = filepath.Base(filepath.FromSlash(strings.TrimSpace(packTarget)))
	}
	stem := strings.TrimSuffix(base, ".candidate.md")
	if stem == base {
		stem = strings.TrimSuffix(base, filepath.Ext(base))
	}
	return sanitizeCandidateReviewProofStem(stem)
}

func sanitizeCandidateReviewProofStem(stem string) string {
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

func candidateReviewProofSummaryBoundary(whatIf bool) []string {
	boundary := []string{
		"proofSummary is read-only; promote create-candidates reports expected proof files but does not create or validate their contents",
		"proof files must stay repo-local review evidence and must not contain case-specific artifacts, traces, dumps, captures, payloads, flags, or customer data",
	}
	if whatIf {
		boundary = append([]string{"WhatIf did not create candidatePath, indexPath, or proof files"}, boundary...)
	}
	return boundary
}

func candidateMainAgentExecutionPlan(result CandidateResult, plan CandidateReviewPlan, whatIf bool) []CandidateExecutionStep {
	steps := []CandidateExecutionStep{}
	if len(plan.ReviewArtifacts) > 0 {
		steps = append(steps, CandidateExecutionStep{
			Name:      "collect-review-artifacts",
			When:      "while reviewing candidates and before declaring cleanup/reconsume complete",
			AppliesTo: candidateReviewArtifactAppliesTo(plan.ReviewArtifacts),
			Actions:   []string{"record one decision artifact per pending-review candidate", "record cleanup evidence after deleting rejected, superseded, or accepted-merged candidatePath", "record doctor and fresh/attached reconsume evidence after accepted tooling merges", "keep artifacts outside authority/confirmed stores"},
			Expected:  "replacement executor can resume from reviewArtifacts without reopening every candidate, indexPath, or command transcript",
			Evidence:  []string{"reviewArtifacts entries populated with decision-note, cleanup-proof, doctor-output, and reconsume-proof expectations"},
			Boundary:  []string{"reviewArtifacts is evidence guidance only", "create-candidates does not create proof files or run cleanup/reconsume commands", "deterministic proof drafts require explicit promote -DraftReviewProof WhatIf/Apply", "do not write authority/confirmed"},
		})
	}
	if whatIf {
		steps = append(steps, CandidateExecutionStep{
			Name:     "materialize-candidates",
			When:     "after reviewing WhatIf preview scope and confirming candidate generation is still desired",
			Commands: []string{"go run ./cmd/rekit -- -Command promote -Target <attached-case> -Pack " + result.Pack + " -CreateCandidates -Format json"},
			Expected: "candidate files and indexPath are created only under candidateRoot/toolingRoot",
			Evidence: []string{"rerun create-candidates JSON result", "created candidatePath list"},
			Boundary: []string{"WhatIf did not write candidate files or indexPath", "do not merge from preview-only candidate paths"},
		})
	}
	steps = append(steps, CandidateExecutionStep{
		Name:      "review-decisions",
		When:      "before any merge, cleanup, or reconsume verification",
		AppliesTo: candidateExecutionAppliesTo(plan.ReviewItems),
		Actions:   []string{"review each decisionChecklist entry", "choose accept, reject, or superseded for pending-review items", "follow only the matching decisionFollowThrough outcome for the chosen decision", "treat blocked items as non-promotable until source or manifest is fixed"},
		Expected:  "every created candidate has an explicit decision before cleanup or merge and the chosen outcome maps to concrete follow-through",
		Evidence:  []string{"decision notes outside authority/confirmed stores", "reviewed candidatePath and packTarget refs", "matching decisionFollowThrough outcome"},
		Boundary:  []string{"do not write authority/confirmed", "do not execute heavy tools", "do not promote case-specific artifacts"},
	})
	if len(plan.CleanupTargets) > 0 {
		steps = append(steps, CandidateExecutionStep{
			Name:      "cleanup-rejected-or-merged-candidates",
			When:      "after reject, superseded decision, or accepted merge into pack source",
			AppliesTo: candidateCleanupAppliesTo(plan.CleanupTargets),
			Actions:   []string{"delete candidatePath after reject, superseded decision, or accepted merge", "update or remove indexPath after deleting candidatePath"},
			Expected:  "rejected or superseded candidates are gone and indexPath no longer points at removed candidates",
			Evidence:  []string{"candidatePath deletion check", "indexPath update/removal check"},
			Boundary:  []string{"do not delete pack source files", "cleanup is limited to candidateRoot/toolingRoot and indexPath"},
		})
	}
	steps = append(steps, CandidateExecutionStep{
		Name:     "pack-doctor-after-accepted-merge",
		When:     "after accepting any managed-doc or tooling candidate into pack sources",
		Commands: []string{"go run ./cmd/rekit -- -Command doctor -Pack " + result.Pack},
		Expected: "pack doctor passes with merged reusable content and no case-specific residue",
		Evidence: []string{"doctor command output"},
		Boundary: []string{"doctor validates pack state only", "do not create case-local artifacts while checking pack"},
	})
	if candidateHasToolingPendingReview(plan.ReviewItems) {
		steps = append(steps, CandidateExecutionStep{
			Name:     "fresh-case-reconsume-after-tooling-merge",
			When:     "after accepting any tooling candidate into tooling/catalog.yml or tooling/recipes/*",
			Commands: []string{"go run ./cmd/rekit -- -Command init -Target <fresh-case> -Pack " + result.Pack + " -ProjectName <name> -Apply", "go run ./cmd/rekit -- -Command doctor -Target <fresh-case> -Pack " + result.Pack},
			Expected: "fresh case binds templateRoot/templatePack and doctor passes while tooling remains pack-sourced",
			Evidence: []string{"fresh case instance metadata", "fresh case doctor output"},
			Boundary: []string{"use a temporary fresh case only", "do not create real case state in the kit repo", "sync does not copy tooling recipes into case-local managed docs"},
		})
		steps = append(steps, CandidateExecutionStep{
			Name:     "attached-case-reconsume-after-tooling-merge",
			When:     "when validating an existing attached case after accepted tooling merges",
			Commands: []string{"go run ./cmd/rekit -- -Command doctor -Target <attached-case> -Pack " + result.Pack},
			Expected: "attached case resolves pack tooling through templateRoot/templatePack without copying tooling recipes into managed docs",
			Evidence: []string{"attached case doctor output"},
			Boundary: []string{"do not overwrite case-local files while checking reconsume"},
		})
	}
	return steps
}

func candidateMissionCommanderNextActions(result CandidateResult, plan CandidateReviewPlan, whatIf bool) []mission.MissionCommanderNextActionItem {
	items := []mission.MissionCommanderNextActionItem{}
	mode := "pack-memory-candidates"
	if whatIf {
		mode = "pack-memory-candidates-preview"
	}
	baseBoundary := []string{
		"reviewPlan guidance only; runtime does not execute merge, cleanup, init, or doctor commands",
		"do not write authority/confirmed",
		"do not execute heavy tools",
	}
	if whatIf {
		baseBoundary = append([]string{"WhatIf did not write candidate files or indexPath"}, baseBoundary...)
	}
	items = append(items, mission.MissionCommanderNextActionItem{
		State:          mode + ":review-decisions",
		Command:        "review reviewPlan.decisionChecklist",
		Source:         "reviewPlan.decisionChecklist",
		RequiresReview: true,
		Reasons:        []string{"choose accept, reject, or superseded for every pending-review candidate before merge or cleanup", "use decisionFollowThrough outcome entries to run only the actions that match the chosen decision", "record the matching reviewArtifacts decision-note before cleanup or reconsume"},
		Boundary:       append([]string{}, baseBoundary...),
	})
	for _, target := range plan.CleanupTargets {
		label := target.Path
		if label == "" {
			label = target.CandidatePath
		}
		reasons := []string{"cleanup candidate after reject, superseded decision, or accepted merge into another pack source"}
		if target.IndexPath != "" {
			reasons = append(reasons, "update or remove indexPath after deleting candidatePath")
		}
		items = append(items, mission.MissionCommanderNextActionItem{
			Label:          label,
			ActionID:       "cleanup-pack-memory-candidate:" + target.Path,
			State:          mode + ":cleanup-candidate",
			Command:        "delete candidatePath and update/remove indexPath",
			Source:         "reviewPlan.cleanupTargets",
			RequiresReview: true,
			Reasons:        append(reasons, "record reviewArtifacts cleanup-proof after candidatePath deletion and indexPath update/removal"),
			Boundary: append(append([]string{}, baseBoundary...),
				"cleanup is limited to candidateRoot/toolingRoot and indexPath",
				"do not delete pack source files",
			),
		})
	}
	items = append(items, mission.MissionCommanderNextActionItem{
		State:    mode + ":pack-doctor",
		Command:  "go run ./cmd/rekit -- -Command doctor -Pack " + result.Pack,
		Source:   "reviewPlan.reconsume.verificationChecklist",
		Reasons:  []string{"run after accepted managed-doc or tooling candidate merge"},
		Boundary: []string{"doctor validates pack state only", "do not create case-local artifacts while checking pack"},
	})
	if candidateHasToolingPendingReview(plan.ReviewItems) {
		items = append(items, mission.MissionCommanderNextActionItem{
			State:    mode + ":fresh-case-init",
			Command:  "go run ./cmd/rekit -- -Command init -Target <fresh-case> -Pack " + result.Pack + " -ProjectName <name> -Apply",
			Source:   "reviewPlan.reconsume.verificationChecklist",
			Reasons:  []string{"create a temporary fresh case before validating accepted tooling candidate reconsume"},
			Boundary: []string{"use a temporary fresh case only", "do not create real case state in the kit repo"},
		})
		items = append(items, mission.MissionCommanderNextActionItem{
			State:    mode + ":fresh-case-doctor",
			Command:  "go run ./cmd/rekit -- -Command doctor -Target <fresh-case> -Pack " + result.Pack,
			Source:   "reviewPlan.reconsume.verificationChecklist",
			Reasons:  []string{"verify accepted tooling candidate reconsumes from pack tooling in a fresh case"},
			Boundary: []string{"use a temporary fresh case only", "sync does not copy tooling recipes into case-local managed docs"},
		})
		items = append(items, mission.MissionCommanderNextActionItem{
			State:    mode + ":attached-case-reconsume",
			Command:  "go run ./cmd/rekit -- -Command doctor -Target <attached-case> -Pack " + result.Pack,
			Source:   "reviewPlan.reconsume.verificationChecklist",
			Reasons:  []string{"verify an attached case resolves pack tooling after accepted tooling merge"},
			Boundary: []string{"do not overwrite case-local files while checking reconsume"},
		})
	}
	return mission.UniqueCommanderNextActions(items)
}

func candidateExecutionAppliesTo(items []CandidateReviewItem) []string {
	paths := []string{}
	for _, item := range items {
		if item.Path != "" {
			paths = append(paths, item.Path)
		}
	}
	return paths
}

func candidateCleanupAppliesTo(items []CandidateCleanupTarget) []string {
	paths := []string{}
	for _, item := range items {
		if item.Path != "" {
			paths = append(paths, item.Path)
		}
	}
	return paths
}

func candidateHasToolingPendingReview(items []CandidateReviewItem) bool {
	for _, item := range items {
		if item.Kind == "tooling-candidate-source" && item.ReviewDecision == "pending-review" {
			return true
		}
	}
	return false
}

func candidateReviewMissionCommanderAction(result CandidateResult, whatIf bool) mission.MissionCommanderAction {
	state := "ready-to-review-pack-memory-candidates"
	prompt := fmt.Sprintf("pack `%s` 的 promote candidates 已生成；逐项 review reviewPlan.reviewItems，merge/reject 后 cleanup，并验证 reconsume。", result.Pack)
	if whatIf {
		state = "preview-pack-memory-candidates"
		prompt = fmt.Sprintf("pack `%s` 的 promote candidate review plan 仍是 WhatIf preview；确认 scope 后再决定是否 rerun without -WhatIf。", result.Pack)
	}
	if result.Blocked > 0 {
		prompt += " blocked item 不可直接合入，先修 source/manifest 或记录 reject。"
	}
	followUp := []string{
		"go run ./cmd/rekit -- -Command doctor -Pack " + result.Pack,
		"go run ./cmd/rekit -- -Command init -Target <fresh-case> -Pack " + result.Pack + " -ProjectName <name> -Apply",
		"go run ./cmd/rekit -- -Command doctor -Target <fresh-case> -Pack " + result.Pack,
	}
	if whatIf {
		followUp = append([]string{"/rekit promote -CreateCandidates -Pack " + result.Pack + " -Format json"}, followUp...)
	}
	boundary := []string{
		"reviewPlan does not merge candidates into pack sources",
		"promote -Apply is not a candidate-scoped accept path",
		"delete rejected or superseded candidate files and update or remove indexPath",
		"accepted tooling candidates require manual tooling/catalog.yml or tooling/recipes/* merge",
		"verify fresh or attached case reconsume after accepted tooling merges",
		"no authority/confirmed writes",
		"no heavy-tool execution",
	}
	if whatIf {
		boundary = append([]string{"WhatIf did not write candidate files or indexPath"}, boundary...)
	}
	if result.Created == 0 && result.Blocked > 0 {
		state = "blocked-pack-memory-candidates"
		prompt = fmt.Sprintf("pack `%s` 的 promote candidate review plan 只有 blocked/no-op item；不要合入，先修 source/manifest 或记录 reject。", result.Pack)
		followUp = []string{"go run ./cmd/rekit -- -Command doctor -Pack " + result.Pack}
	}
	return mission.MissionCommanderAction{
		State:            state,
		Prompt:           prompt,
		PrimaryCommand:   "review reviewPlan.reviewItems",
		FollowUpCommands: followUp,
		Boundary:         boundary,
	}
}

func candidateReviewItem(result CandidateResult, write CandidateWrite) CandidateReviewItem {
	item := CandidateReviewItem{Path: write.Path, Kind: write.Kind, Action: write.Action}
	switch write.Action {
	case "create-candidate", "would-create-candidate":
		item.ReviewDecision = "pending-review"
		item.CandidatePath = write.TargetPath
		item.CleanupPath = write.TargetPath
		item.RejectTargetHint = "delete candidatePath after recording reject or superseded decision"
		if write.Kind == "tooling-candidate-source" {
			item.PackTarget = filepath.ToSlash(filepath.Join("packs", result.Pack, "tooling"))
			item.MergeTargetHint = "merge accepted reusable guidance into tooling/catalog.yml or tooling/recipes/*; do not copy into case-local managed docs"
			item.MainAgentActions = []string{"inspect sanitized tooling candidate", "confirm no case-specific residue remains", "merge accepted reusable content into pack tooling catalog or recipe", "delete rejected or superseded candidatePath", "run doctor and verify fresh or attached case reconsume"}
		} else {
			if write.Path != "" {
				item.PackTarget = filepath.Join(result.RepoRoot, "packs", result.Pack, filepath.FromSlash(write.Path))
			}
			item.MergeTargetHint = "merge accepted reusable guidance into pack managed doc packTarget after reviewing candidatePath; do not treat promote -Apply as a candidate-scoped accept path"
			item.MainAgentActions = []string{"inspect candidate against current pack source", "extract reusable guidance and resolve conflicts", "merge accepted content or reject candidate", "delete rejected or superseded candidatePath and update or remove indexPath", "run doctor after accepted merge"}
		}
	case "blocked-deny-pattern", "blocked-missing-pack-file", "blocked-after-sanitization":
		item.ReviewDecision = "blocked"
		item.PackTarget = write.TargetPath
		item.RejectTargetHint = "do not merge; fix source or manifest first, then regenerate candidates"
		item.MainAgentActions = []string{"treat blocked item as non-promotable", "do not copy raw case-specific content into pack", "document why it was rejected or regenerate after sanitization/manifest fix"}
	case "skip-missing-case-file", "skip-non-managed-promote-file", "skip-missing-source", "unchanged":
		item.ReviewDecision = "not-needed"
		item.PackTarget = write.TargetPath
		item.MainAgentActions = []string{"no candidate review needed", "leave pack source unchanged"}
	default:
		item.ReviewDecision = "inspect"
		item.PackTarget = write.TargetPath
		item.MainAgentActions = []string{"inspect action before deciding merge, reject, or cleanup"}
	}
	return item
}

func candidateReviewArtifacts(_ CandidateResult, item CandidateReviewItem, whatIf bool) []CandidateReviewArtifact {
	baseBoundary := []string{
		"review artifact is guidance only; create-candidates does not write decision, cleanup, or reconsume proof",
		"deterministic proof drafts require explicit promote -DraftReviewProof WhatIf/Apply",
		"do not write authority/confirmed",
		"do not execute heavy tools",
	}
	if whatIf {
		baseBoundary = append([]string{"WhatIf did not create candidatePath; collect this artifact only after materializing candidates"}, baseBoundary...)
	}
	artifacts := []CandidateReviewArtifact{}
	switch item.ReviewDecision {
	case "pending-review":
		artifacts = append(artifacts, CandidateReviewArtifact{
			Path:          item.Path,
			Kind:          item.Kind,
			Name:          "candidate-decision-note",
			When:          "before merge, cleanup, or reconsume; choose accept, reject, or superseded for this candidate",
			Action:        "record reviewed decision and selected decisionFollowThrough outcome outside authority/confirmed stores",
			CandidatePath: item.CandidatePath,
			PackTarget:    item.PackTarget,
			Format:        "strict JSON pack-memory-candidate-review-proof note with decision, reason, candidatePath, packTarget, reviewItem, evidenceRefs, and boundary",
			Evidence:      []string{"decision note path/ref", "selected decisionFollowThrough outcome"},
			Boundary:      append([]string{}, baseBoundary...),
		})
		if item.CleanupPath != "" {
			when := "after deleting candidatePath because it was rejected, superseded, or accepted and merged into pack source"
			format := "strict JSON pack-memory-candidate-lifecycle-proof with candidate-absent and index-entry-absent checks plus hashed evidenceRefs"
			if item.Kind == "tooling-candidate-source" {
				format = "strict JSON pack-memory-candidate-lifecycle-proof with candidate-absent check plus hashed evidenceRefs"
			}
			if whatIf {
				when = "after rerun without WhatIf materializes candidatePath, then deleting it because it was rejected, superseded, or accepted and merged"
			}
			artifacts = append(artifacts, CandidateReviewArtifact{
				Path:          item.Path,
				Kind:          item.Kind,
				Name:          "candidate-cleanup-proof",
				When:          when,
				Action:        "record candidatePath deletion check and indexPath update/removal proof",
				CandidatePath: item.CleanupPath,
				Format:        format,
				Evidence:      []string{"candidatePath deletion check", "indexPath update/removal check"},
				Boundary: append(append([]string{}, baseBoundary...),
					"cleanup is limited to candidateRoot/toolingRoot and indexPath",
					"do not delete pack source files",
				),
			})
		}
		artifacts = append(artifacts, CandidateReviewArtifact{
			Path:       item.Path,
			Kind:       item.Kind,
			Name:       "pack-doctor-output",
			When:       "after an accept decision merges reusable content into packTarget",
			Action:     "record doctor command output before declaring accepted merge complete",
			PackTarget: item.PackTarget,
			Format:     "strict JSON pack-memory-candidate-lifecycle-proof with a passed pack-doctor check plus hashed evidenceRefs",
			Evidence:   []string{"doctor command output"},
			Boundary: append(append([]string{}, baseBoundary...),
				"doctor validates pack state only",
				"do not create case-local artifacts while checking pack",
			),
		})
		if item.Kind == "tooling-candidate-source" {
			artifacts = append(artifacts,
				CandidateReviewArtifact{
					Path:          item.Path,
					Kind:          item.Kind,
					Name:          "fresh-case-reconsume-proof",
					When:          "after accepting tooling candidate into tooling/catalog.yml or tooling/recipes/*",
					Action:        "record temporary fresh-case init and doctor output proving pack tooling reconsume",
					CandidatePath: item.CandidatePath,
					PackTarget:    item.PackTarget,
					Format:        "strict JSON pack-memory-candidate-lifecycle-proof with passed fresh-case-reconsume and pack-doctor checks plus hashed evidenceRefs",
					Evidence:      []string{"fresh case instance metadata", "fresh case doctor output"},
					Boundary: append(append([]string{}, baseBoundary...),
						"use a temporary fresh case only",
						"do not create real case state in the kit repo",
						"sync does not copy tooling recipes into case-local managed docs",
					),
				},
				CandidateReviewArtifact{
					Path:          item.Path,
					Kind:          item.Kind,
					Name:          "attached-case-reconsume-proof",
					When:          "when validating an existing attached case after accepted tooling merge",
					Action:        "record attached-case doctor output proving pack tooling is resolved through templateRoot/templatePack",
					CandidatePath: item.CandidatePath,
					PackTarget:    item.PackTarget,
					Format:        "strict JSON pack-memory-candidate-lifecycle-proof with passed attached-case-reconsume and pack-doctor checks plus hashed evidenceRefs",
					Evidence:      []string{"attached case doctor output"},
					Boundary: append(append([]string{}, baseBoundary...),
						"do not overwrite case-local files while checking reconsume",
						"fresh/attached case verification reads pack tooling through templateRoot/templatePack",
					),
				},
			)
		}
	case "blocked":
		artifacts = append(artifacts, CandidateReviewArtifact{
			Path:       item.Path,
			Kind:       item.Kind,
			Name:       "blocked-review-note",
			When:       "when a blocked candidate item appears in reviewPlan.reviewItems",
			Action:     "record why the item is non-promotable or regenerate after source/manifest/sanitization fix",
			PackTarget: item.PackTarget,
			Format:     "markdown or JSON note naming blocked action, reason, and next regeneration/fix step",
			Evidence:   []string{"review note for blocked item", "rerun promote -CreateCandidates result if regenerated"},
			Boundary: append(append([]string{}, baseBoundary...),
				"blocked item must not be copied into pack sources",
			),
		})
	}
	return artifacts
}

func candidateReviewArtifactAppliesTo(items []CandidateReviewArtifact) []string {
	out := []string{}
	seen := map[string]bool{}
	for _, item := range items {
		label := item.Path
		if label == "" {
			label = item.Name
		}
		if label == "" || seen[label] {
			continue
		}
		seen[label] = true
		out = append(out, label)
	}
	return out
}

func candidateDecisionFollowThrough(result CandidateResult, item CandidateReviewItem, whatIf bool) CandidateDecisionFollowThrough {
	follow := CandidateDecisionFollowThrough{Path: item.Path, Kind: item.Kind, ReviewDecision: item.ReviewDecision, CandidatePath: item.CandidatePath, PackTarget: item.PackTarget, Boundary: []string{"decisionFollowThrough is guidance only; runtime does not execute merge, cleanup, init, or doctor commands", "do not write authority/confirmed", "do not execute heavy tools", "do not promote case-local samples, traces, dumps, captures, artifacts, payloads, flags, or customer data"}}
	switch item.ReviewDecision {
	case "pending-review":
		follow.Outcomes = append(follow.Outcomes,
			candidateDecisionOutcome(result, item, "accept", whatIf),
			candidateDecisionOutcome(result, item, "reject", whatIf),
			candidateDecisionOutcome(result, item, "superseded", whatIf),
		)
	case "blocked":
		follow.Outcomes = append(follow.Outcomes, CandidateDecisionOutcome{
			Decision: "blocked",
			State:    "pack-memory-candidate:blocked-non-promotable",
			When:     "blocked-deny-pattern, blocked-missing-pack-file, or blocked-after-sanitization item appears in reviewPlan.reviewItems",
			Actions:  []string{"do not merge candidate content", "fix source/manifest/sanitization issue and regenerate candidates, or record reject outside authority/confirmed stores"},
			Expected: "blocked item is not copied into pack sources",
			Evidence: []string{"review note for blocked item", "rerun promote -CreateCandidates result if regenerated"},
			Boundary: []string{"blocked item must not be copied into pack sources"},
		})
	case "not-needed":
		follow.Outcomes = append(follow.Outcomes, CandidateDecisionOutcome{
			Decision: "not-needed",
			State:    "pack-memory-candidate:no-op",
			When:     "item is missing, unchanged, or outside managed promote scope",
			Actions:  []string{"leave pack source unchanged", "do not create cleanup work for missing or unchanged item"},
			Expected: "no candidate merge or cleanup is needed",
			Boundary: []string{"leave pack source unchanged"},
		})
	default:
		follow.Outcomes = append(follow.Outcomes, CandidateDecisionOutcome{
			Decision: "inspect",
			State:    "pack-memory-candidate:inspect",
			When:     "reviewPlan item has an unrecognized action",
			Actions:  []string{"inspect item action before deciding merge, reject, or cleanup"},
			Expected: "main Agent chooses a safe reviewed outcome before touching pack sources or candidates",
			Boundary: []string{"do not merge or cleanup until the action is understood"},
		})
	}
	return follow
}

func candidateDecisionOutcome(result CandidateResult, item CandidateReviewItem, decision string, whatIf bool) CandidateDecisionOutcome {
	outcome := CandidateDecisionOutcome{Decision: decision}
	switch decision {
	case "accept":
		outcome.State = "pack-memory-candidate:accepted-merge"
		outcome.When = "after candidatePath is reviewed as reusable, case-neutral content"
		outcome.Actions = append([]string{"merge reusable content into packTarget or an explicitly chosen pack tooling recipe/catalog target"}, item.MainAgentActions...)
		outcome.CleanupActions = candidateCleanupActions(result, item, whatIf)
		outcome.VerificationCommands = []string{"go run ./cmd/rekit -- -Command doctor -Pack " + result.Pack}
		outcome.Expected = "accepted reusable content lives in pack sources and candidatePath/indexPath cleanup is complete"
		outcome.Evidence = []string{"pack source diff", "candidatePath deletion check", "doctor command output"}
		outcome.Boundary = []string{"promote -Apply is not a candidate-scoped accept path", "do not write authority/confirmed", "do not execute heavy tools"}
		if item.Kind == "tooling-candidate-source" {
			outcome.State = "pack-memory-candidate:accepted-tooling-reconsume"
			outcome.Actions = append(outcome.Actions, "verify fresh or attached case reconsume after tooling merge")
			outcome.VerificationCommands = append(outcome.VerificationCommands, "go run ./cmd/rekit -- -Command init -Target <fresh-case> -Pack "+result.Pack+" -ProjectName <name> -Apply", "go run ./cmd/rekit -- -Command doctor -Target <fresh-case> -Pack "+result.Pack, "go run ./cmd/rekit -- -Command doctor -Target <attached-case> -Pack "+result.Pack)
			outcome.Expected = "accepted tooling is resolved from pack tooling by fresh or attached cases without copying tooling recipes into managed docs"
			outcome.Evidence = append(outcome.Evidence, "fresh case instance metadata", "fresh or attached case doctor output")
			outcome.Boundary = append(outcome.Boundary, "tooling candidates require manual tooling/catalog.yml or tooling/recipes/* merge", "fresh/attached case verification reads pack tooling through templateRoot/templatePack")
		}
	case "reject":
		outcome.State = "pack-memory-candidate:rejected-cleanup"
		outcome.When = "after candidatePath is reviewed and rejected"
		outcome.Actions = []string{"record reject decision outside authority/confirmed stores", "do not merge candidate content"}
		outcome.CleanupActions = candidateCleanupActions(result, item, whatIf)
		outcome.Expected = "rejected candidatePath is deleted and indexPath no longer points at it"
		outcome.Evidence = []string{"review note for reject decision", "candidatePath deletion check", "indexPath update/removal check"}
		outcome.Boundary = []string{"do not delete pack source files", "cleanup is limited to candidateRoot/toolingRoot and indexPath"}
	case "superseded":
		outcome.State = "pack-memory-candidate:superseded-cleanup"
		outcome.When = "after candidatePath is superseded by another accepted candidate or existing pack source"
		outcome.Actions = []string{"record superseded decision outside authority/confirmed stores", "merge only the chosen replacement if it was separately accepted"}
		outcome.CleanupActions = candidateCleanupActions(result, item, whatIf)
		outcome.Expected = "superseded candidatePath is deleted and replacement/pack source decision is traceable outside authority/confirmed stores"
		outcome.Evidence = []string{"review note naming the replacement", "candidatePath deletion check", "indexPath update/removal check"}
		outcome.Boundary = []string{"do not delete pack source files", "cleanup is limited to candidateRoot/toolingRoot and indexPath"}
	}
	return outcome
}

func candidateDecisionChecklist(result CandidateResult, item CandidateReviewItem, whatIf bool) CandidateDecisionChecklist {
	check := CandidateDecisionChecklist{Path: item.Path, Kind: item.Kind, ReviewDecision: item.ReviewDecision, CandidatePath: item.CandidatePath, PackTarget: item.PackTarget, Boundary: []string{"do not write authority/confirmed", "do not execute heavy tools", "do not promote case-local samples, traces, dumps, captures, artifacts, payloads, flags, or customer data"}}
	switch item.ReviewDecision {
	case "pending-review":
		check.ReviewAction = "inspect candidatePath against packTarget and choose accept, reject, or superseded"
		check.AcceptActions = append([]string{"review candidatePath for reusable, case-neutral content", "merge accepted reusable content into packTarget or an explicitly chosen pack tooling recipe/catalog target"}, item.MainAgentActions...)
		check.RejectActions = []string{"record reject or superseded decision outside authority/confirmed stores", "delete candidatePath after reject or superseded decision"}
		check.CleanupActions = candidateCleanupActions(result, item, whatIf)
		check.VerificationCommands = []string{"go run ./cmd/rekit -- -Command doctor -Pack " + result.Pack}
		if item.Kind == "tooling-candidate-source" {
			check.VerificationCommands = append(check.VerificationCommands, "go run ./cmd/rekit -- -Command init -Target <fresh-case> -Pack "+result.Pack+" -ProjectName <name> -Apply", "go run ./cmd/rekit -- -Command doctor -Target <fresh-case> -Pack "+result.Pack)
			check.Boundary = append(check.Boundary, "fresh-case-reconsume required after accepted tooling merge", "tooling candidates require manual tooling/catalog.yml or tooling/recipes/* merge", "fresh/attached case verification reads pack tooling through templateRoot/templatePack")
		} else {
			check.Boundary = append(check.Boundary, "promote -Apply is not a candidate-scoped accept path")
		}
	case "blocked":
		check.ReviewAction = "treat blocked item as non-promotable until source or manifest is fixed"
		check.RejectActions = []string{"do not merge candidate content", "fix source/manifest then regenerate candidates, or record reject"}
		check.Boundary = append(check.Boundary, "blocked item must not be copied into pack sources")
	case "not-needed":
		check.ReviewAction = "no merge or cleanup action is needed"
		check.Boundary = append(check.Boundary, "leave pack source unchanged")
	default:
		check.ReviewAction = "inspect item action before deciding merge, reject, or cleanup"
		check.CleanupActions = candidateCleanupActions(result, item, whatIf)
	}
	return check
}

func candidateCleanupActions(result CandidateResult, item CandidateReviewItem, whatIf bool) []string {
	if item.CleanupPath == "" {
		return []string{}
	}
	actions := []string{"delete candidatePath after reject, superseded decision, or accepted merge into another pack source"}
	if result.IndexPath != "" {
		actions = append(actions, "update or remove indexPath after deleting candidatePath")
	}
	if whatIf {
		actions[0] = "if rerun without WhatIf creates candidatePath, delete it after reject, superseded decision, or accepted merge elsewhere"
	}
	return actions
}

func uniqueBackupRoot(root string) (string, error) {
	stamp := time.Now().Format("20060102-150405")
	candidate := filepath.Join(root, stamp)
	if _, err := os.Stat(candidate); os.IsNotExist(err) {
		return candidate, nil
	} else if err != nil {
		return "", err
	}
	for i := 1; i <= 999; i++ {
		candidate = filepath.Join(root, fmt.Sprintf("%s-%d", stamp, i))
		if _, err := os.Stat(candidate); os.IsNotExist(err) {
			return candidate, nil
		} else if err != nil {
			return "", err
		}
	}
	return "", fmt.Errorf("unable to allocate unique backup root under %s", root)
}

func packBackupPath(path, packRoot, backupRoot string) (string, error) {
	packFull, err := filepath.Abs(packRoot)
	if err != nil {
		return "", err
	}
	pathFull, err := filepath.Abs(path)
	if err != nil {
		return "", err
	}
	rel, err := filepath.Rel(packFull, pathFull)
	if err != nil {
		return "", err
	}
	if rel == ".." || strings.HasPrefix(rel, ".."+string(filepath.Separator)) || filepath.IsAbs(rel) {
		return "", fmt.Errorf("cannot backup file outside pack root: %s", path)
	}
	dest := filepath.Join(backupRoot, rel)
	if err := assertInsideRoot(backupRoot, dest); err != nil {
		return "", err
	}
	return dest, nil
}

func backupPackFile(path, packRoot, backupRoot string) error {
	if !refsf.Exists(path) {
		return fmt.Errorf("missing pack file: %s", path)
	}
	dest, err := packBackupPath(path, packRoot, backupRoot)
	if err != nil {
		return err
	}
	b, err := os.ReadFile(path)
	if err != nil {
		return err
	}
	if err := os.MkdirAll(filepath.Dir(dest), 0o755); err != nil {
		return err
	}
	return os.WriteFile(dest, b, 0o644)
}

func restorePromoteBackups(writes []ApplyWrite) error {
	for _, write := range writes {
		if write.BackupPath == "" || write.TargetPath == "" || write.Action != "promote" {
			continue
		}
		b, err := os.ReadFile(write.BackupPath)
		if err != nil {
			return err
		}
		if err := os.WriteFile(write.TargetPath, b, 0o644); err != nil {
			return err
		}
	}
	return nil
}

func assertInsideRoot(root, path string) error {
	rootFull, err := filepath.Abs(root)
	if err != nil {
		return err
	}
	pathFull, err := filepath.Abs(path)
	if err != nil {
		return err
	}
	rel, err := filepath.Rel(rootFull, pathFull)
	if err != nil {
		return err
	}
	if rel == ".." || strings.HasPrefix(rel, ".."+string(filepath.Separator)) || filepath.IsAbs(rel) {
		return fmt.Errorf("candidate path escapes root: %s", path)
	}
	return nil
}

func safeCandidateName(value string) string {
	replacer := strings.NewReplacer("\\", "_", "/", "_", ":", "_", "*", "_", "?", "_", "\"", "_", "<", "_", ">", "_", "|", "_")
	out := strings.TrimSpace(replacer.Replace(value))
	if out == "" {
		return "candidate"
	}
	return out
}

func uniqueCandidatePath(root, name string) (string, error) {
	candidate := filepath.Join(root, name)
	if _, err := os.Stat(candidate); os.IsNotExist(err) {
		return candidate, nil
	} else if err != nil {
		return "", err
	}

	ext := filepath.Ext(name)
	base := strings.TrimSuffix(name, ext)
	for i := 1; i <= 999; i++ {
		candidate = filepath.Join(root, fmt.Sprintf("%s-%d%s", base, i, ext))
		if _, err := os.Stat(candidate); os.IsNotExist(err) {
			return candidate, nil
		} else if err != nil {
			return "", err
		}
	}
	return "", fmt.Errorf("unable to allocate unique candidate path for %s", name)
}

func writeNewFile(path string, text []byte) error {
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return err
	}
	file, err := os.OpenFile(path, os.O_WRONLY|os.O_CREATE|os.O_EXCL, 0o644)
	if err != nil {
		return err
	}
	defer file.Close()
	_, err = file.Write(text)
	return err
}

func toolingCandidateHeader(rel string) string {
	return "# Tooling candidate from case\r\n\r\n" +
		"Source: `" + rel + "`\r\n" +
		"Generated by: `rekit promote`\r\n\r\n" +
		"> Review before merging into `tooling/catalog.yml` or `tooling/recipes/*`.\r\n\r\n" +
		"---\r\n\r\n"
}

func caseSpecificPatterns(caseRoot string) []string {
	name := filepath.Base(filepath.Clean(caseRoot))
	parts := regexp.MustCompile(`[-_\.\s]+`).Split(name, -1)
	out := []string{}
	for _, part := range parts {
		if len(part) >= 4 {
			out = append(out, regexp.QuoteMeta(part))
		}
	}
	return out
}

func sanitizeToolingCandidate(text, caseRoot string) (string, map[string]int) {
	counts := map[string]int{}
	out := text
	replace := func(key, pattern, repl string) {
		re := regexp.MustCompile(pattern)
		matches := re.FindAllStringIndex(out, -1)
		counts[key] += len(matches)
		out = re.ReplaceAllString(out, repl)
	}
	casePattern := regexp.QuoteMeta(strings.TrimRight(filepath.Clean(caseRoot), string(filepath.Separator)))
	replace("caseRoot", `(?i)`+casePattern, "<caseRoot>")
	replace("absolutePath", `(?i)[A-Za-z]:\\[^`+"`"+`\r\n|，。；;\)\] ]+`, "<absolutePath>")
	for _, pattern := range caseSpecificPatterns(caseRoot) {
		replace("targetExe", `(?i)`+pattern+`[A-Za-z0-9_.-]*\.exe`, "<target.exe>")
		replace("casePath", `(?i)`+pattern+`[\\/]`, "<case>/")
		replace("caseTerm", `(?i)`+pattern, "<caseTerm>")
	}
	replace("toolsRoot", `(?i)\.\.[\\/]tools[\\/]`, "<toolsRoot>/")
	replace("artifactsPath", `(?i)artifacts[\\/][^`+"`"+`\r\n|，。；;\)\] ]+`, "<artifactsPath>")
	replace("capturesPath", `(?i)captures[\\/][^`+"`"+`\r\n|，。；;\)\] ]+`, "<capturesPath>")
	replace("traceFile", `(?i)[A-Za-z0-9_.-]*trace[A-Za-z0-9_.-]*\.(csv|jsonl|log|txt|bin)`, "<traceFile>")
	replace("dumpFile", `(?i)[A-Za-z0-9_.-]*dump[A-Za-z0-9_.-]*\.(dmp|bin|raw|exe|dll)`, "<dumpFile>")
	replace("address", `0x[0-9A-Fa-f]{6,}`, "<address>")
	replace("ctx", `(?i)ctx\d+`, "<ctxNNN>")
	replace("round", `(?i)round\d+`, "<roundN>")
	replace("task", `(?i)Task #\d+`, "Task #<n>")
	return out, counts
}
