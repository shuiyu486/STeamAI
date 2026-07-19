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
)

type CandidateOptions struct {
	WhatIf bool
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

type CandidateReviewPlan struct {
	Mode                   string                         `json:"mode"`
	Scope                  string                         `json:"scope"`
	CandidateRoot          string                         `json:"candidateRoot"`
	ToolingRoot            string                         `json:"toolingRoot"`
	IndexPath              string                         `json:"indexPath,omitempty"`
	ItemCount              int                            `json:"itemCount"`
	ReviewItems            []CandidateReviewItem          `json:"reviewItems"`
	CleanupTargets         []CandidateCleanupTarget       `json:"cleanupTargets"`
	Reconsume              CandidateReconsumeGuidance     `json:"reconsume"`
	MissionCommanderAction mission.MissionCommanderAction `json:"missionCommanderAction"`
	RuntimeBoundary        []string                       `json:"runtimeBoundary"`
	CompletionCriteria     []string                       `json:"completionCriteria"`
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

type CandidateCleanupTarget struct {
	Path          string `json:"path"`
	Kind          string `json:"kind"`
	CandidatePath string `json:"candidatePath"`
	CleanupWhen   string `json:"cleanupWhen"`
}

type CandidateReconsumeGuidance struct {
	Mode        string   `json:"mode"`
	ManagedDocs string   `json:"managedDocs"`
	Tooling     string   `json:"tooling"`
	Commands    []string `json:"commands"`
	Boundary    []string `json:"boundary"`
}

type CandidateResult struct {
	SchemaVersion     int                 `json:"schemaVersion"`
	Command           string              `json:"command"`
	CaseRoot          string              `json:"caseRoot"`
	RepoRoot          string              `json:"repoRoot"`
	Pack              string              `json:"pack"`
	IsMutation        bool                `json:"isMutation"`
	Applied           bool                `json:"applied"`
	CandidateRoot     string              `json:"candidateRoot"`
	ToolingRoot       string              `json:"toolingRoot"`
	IndexPath         string              `json:"indexPath,omitempty"`
	Created           int                 `json:"created"`
	Blocked           int                 `json:"blocked"`
	Skipped           int                 `json:"skipped"`
	Writes            []CandidateWrite    `json:"writes"`
	ReviewPlan        CandidateReviewPlan `json:"reviewPlan"`
	RequiresReview    bool                `json:"requiresReview"`
	RequiresCleanup   bool                `json:"requiresCleanup"`
	DeniedWriteAction []string            `json:"deniedWriteAction"`
	NextSteps         []string            `json:"nextSteps"`
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
		changed := caseText != packText
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
			"no authority/confirmed writes",
			"no heavy-tool execution",
		},
		CompletionCriteria: []string{
			"each created candidate has an explicit accept, reject, or superseded decision",
			"accepted managed docs are merged only after reviewing the candidate against current packTarget; promote -Apply is not a candidate-scoped accept path",
			"accepted tooling candidates are merged into tooling/catalog.yml or tooling/recipes/* and validated with doctor",
			"rejected or superseded candidate files are deleted and indexPath is updated or removed",
			"fresh or attached case reconsume path is verified after accepted tooling merges",
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
		if item.CleanupPath != "" {
			plan.CleanupTargets = append(plan.CleanupTargets, CandidateCleanupTarget{Path: write.Path, Kind: write.Kind, CandidatePath: item.CleanupPath, CleanupWhen: cleanupWhen})
		}
	}
	plan.ItemCount = len(plan.ReviewItems)
	plan.MissionCommanderAction = candidateReviewMissionCommanderAction(result, whatIf)
	return plan
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
