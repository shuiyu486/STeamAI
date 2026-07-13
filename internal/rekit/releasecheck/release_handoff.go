package releasecheck

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

type ReleaseHandoff struct {
	Ready        bool                       `json:"ready"`
	Summary      string                     `json:"summary"`
	ReadFirst    []ReleaseHandoffDocument   `json:"readFirst"`
	Signals      []ReleaseHandoffSignal     `json:"signals"`
	LatestBatch  ReleaseHandoffLatestBatch  `json:"latestBatch"`
	ReleaseNotes ReleaseHandoffReleaseNotes `json:"releaseNotes"`
	KnownGaps    []ReleaseHandoffKnownGap   `json:"knownGaps"`
	Validation   []ReleaseHandoffValidation `json:"validation"`
	NextActions  []string                   `json:"nextActions"`
	Warnings     []string                   `json:"warnings"`
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
	PlanPath         string `json:"planPath"`
	Present          bool   `json:"present"`
	Title            string `json:"title"`
	BatchID          string `json:"batchId"`
	Status           string `json:"status"`
	Goal             string `json:"goal"`
	ValidationResult string `json:"validationResult"`
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

type ReleaseHandoffValidation struct {
	Command  string `json:"command"`
	Kind     string `json:"kind"`
	RepoPath string `json:"repoPath,omitempty"`
	Required bool   `json:"required"`
	Present  bool   `json:"present"`
	Resolved bool   `json:"resolved"`
}

var releaseHandoffReadFirst = []ReleaseHandoffDocument{
	{Path: "docs/release-readiness.md", Purpose: "one-page release gate, current state, and known gaps"},
	{Path: "docs/autonomous-goal.md", Purpose: "long-term autonomous goal and continuation guard"},
	{Path: "docs/go-first-convergence-plan.md", Purpose: "Go-first convergence stage map"},
	{Path: "docs/powershell-deprecation.md", Purpose: "PowerShell owner, freeze, and removal gates"},
	{Path: "docs/batch-plan.md", Purpose: "latest completed batch and next implementation context"},
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
	handoff.Signals = releaseHandoffSignals(check, handoff.LatestBatch, handoff.ReleaseNotes, handoff.KnownGaps)
	handoff.Warnings = releaseHandoffWarnings(handoff)
	if len(handoff.Warnings) > 0 {
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

func releaseHandoffSignals(check Result, latest ReleaseHandoffLatestBatch, notes ReleaseHandoffReleaseNotes, gaps []ReleaseHandoffKnownGap) []ReleaseHandoffSignal {
	return []ReleaseHandoffSignal{
		{
			Name:    "release-check inventory",
			Ready:   check.Ready,
			Summary: check.Summary,
			Details: []string{
				fmt.Sprintf("gateProfile=%s ready=%t steps=%d", check.GateProfile.Name, check.GateProfile.Ready, check.GateProfile.StepCount),
				fmt.Sprintf("packs=%d knownGaps=%d warnings=%d", len(check.Packs), len(check.KnownGaps), len(check.Warnings)),
			},
		},
		{
			Name:    "CI release gate",
			Ready:   check.CIReleaseGate.Ready,
			Summary: check.CIReleaseGate.Summary,
			Details: []string{
				fmt.Sprintf("workflow=%s", check.CIReleaseGate.WorkflowPath),
				fmt.Sprintf("jobs=%d commands=%d forbidden=%d", len(check.CIReleaseGate.Jobs), len(check.CIReleaseGate.RequiredCommands), len(check.CIReleaseGate.ForbiddenStrings)),
			},
		},
		{
			Name:    "PowerShell deprecation",
			Ready:   check.PowerShellDeprecation.Ready,
			Summary: check.PowerShellDeprecation.Summary,
			Details: []string{
				fmt.Sprintf("strategy=%s", check.PowerShellDeprecation.StrategyDocument),
				fmt.Sprintf("commands=%d modules=%d freezeGates=%d blocked=%d", len(check.PowerShellDeprecation.CommandOwnership), len(check.PowerShellDeprecation.ModuleStatus), len(check.PowerShellDeprecation.FreezeGates), len(check.PowerShellDeprecation.BlockedMigrations)),
			},
		},
		{
			Name:    "heavy-tool gate manifests",
			Ready:   len(check.HeavyToolGateActions) > 0,
			Summary: strings.Join(check.HeavyToolGateActions, ","),
			Details: []string{
				fmt.Sprintf("actions=%d", len(check.HeavyToolGateActions)),
				"gate preview/apply only creates pending-gate requests; no heavy-tool execution",
			},
		},
		{
			Name:    "latest batch documentation",
			Ready:   latest.Present && strings.Contains(latest.Status, "已完成") && strings.TrimSpace(latest.Goal) != "" && strings.TrimSpace(latest.ValidationResult) != "",
			Summary: latest.Title,
			Details: []string{
				fmt.Sprintf("batch=%s", latest.BatchID),
				fmt.Sprintf("status=%s", latest.Status),
				fmt.Sprintf("plan=%s", latest.PlanPath),
			},
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

func releaseHandoffKnownGaps(gaps []string) []ReleaseHandoffKnownGap {
	out := make([]ReleaseHandoffKnownGap, 0, len(gaps))
	for i, gap := range gaps {
		gap = compactHandoffText(gap, 220)
		if strings.TrimSpace(gap) == "" {
			continue
		}
		out = append(out, ReleaseHandoffKnownGap{
			Index:    i + 1,
			Category: knownGapCategory(gap),
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
	if strings.Contains(lower, "bounded dispatch") {
		categories = append(categories, "dispatch")
	}
	if strings.Contains(lower, "heavy-tool") {
		categories = append(categories, "heavy-tool")
	}
	if strings.Contains(lower, "authority/confirmed") {
		categories = append(categories, "authority")
	}
	if strings.Contains(lower, "policy schema") {
		categories = append(categories, "policy-schema")
	}
	if strings.Contains(lower, "powershell") {
		categories = append(categories, "powershell-deprecation")
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
		"Read releaseHandoff.signals[] first, then docs/release-readiness.md for the detailed gate and known gaps.",
		"Use releaseHandoff.validation[] or gateProfile.steps[] as the local/CI minimum before tagging or handing off.",
		"Keep CHANGELOG.md Unreleased aligned with the latest completed docs/batch-plan.md batch before handing off.",
		"Continue the autonomous loop from docs/autonomous-goal.md and append the next completed batch to docs/batch-plan.md.",
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
		if !strings.Contains(handoff.LatestBatch.Status, "已完成") {
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
	if len(handoff.Validation) == 0 {
		warnings = append(warnings, "release handoff validation command list is empty")
	}
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
	start := -1
	for i, line := range lines {
		trimmed := strings.TrimSpace(line)
		if strings.HasPrefix(trimmed, "### Batch ") {
			start = i
			latest.Title = strings.TrimSpace(strings.TrimPrefix(trimmed, "### "))
			latest.BatchID = batchIDFromTitle(latest.Title)
		}
	}
	if start < 0 {
		return latest
	}
	end := len(lines)
	for i := start + 1; i < len(lines); i++ {
		if strings.HasPrefix(strings.TrimSpace(lines[i]), "### ") {
			end = i
			break
		}
	}
	for _, line := range lines[start+1 : end] {
		trimmed := strings.TrimSpace(line)
		if value, ok := markdownFieldValue(trimmed, "状态"); ok {
			latest.Status = compactHandoffText(value, 160)
		}
		if value, ok := markdownFieldValue(trimmed, "目标"); ok {
			latest.Goal = compactHandoffText(value, 240)
		}
		if value, ok := markdownFieldValue(trimmed, "验证结果"); ok {
			latest.ValidationResult = compactHandoffText(value, 240)
		}
	}
	return latest
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
