package releasecheck

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"github.com/shuiyu486/re-context-kits/internal/rekit/caseshim"
	"github.com/shuiyu486/re-context-kits/internal/rekit/defaultdocs"
	"github.com/shuiyu486/re-context-kits/internal/rekit/manifest"
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
	Completed            bool     `json:"completed"`
	LocalValidationReady bool     `json:"localValidationReady"`
	ReleaseCheckReady    bool     `json:"releaseCheckReady"`
	RemoteReleaseGate    string   `json:"remoteReleaseGate,omitempty"`
	CommitRefs           []string `json:"commitRefs,omitempty"`
	Evidence             []string `json:"evidence,omitempty"`
	NextAction           string   `json:"nextAction,omitempty"`
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
	Ready      bool                                      `json:"ready"`
	Summary    string                                    `json:"summary"`
	Total      int                                       `json:"total"`
	Packs      []ReleaseHandoffPackMemoryCandidateStatus `json:"packs"`
	NextAction string                                    `json:"nextAction,omitempty"`
	Warnings   []string                                  `json:"warnings,omitempty"`
}

type ReleaseHandoffPackMemoryCandidateStatus struct {
	Pack            string   `json:"pack"`
	Maturity        string   `json:"maturity"`
	CandidateRoot   string   `json:"candidateRoot"`
	ToolingRoot     string   `json:"toolingRoot"`
	IndexPath       string   `json:"indexPath,omitempty"`
	CandidateFiles  int      `json:"candidateFiles"`
	ToolingFiles    int      `json:"toolingFiles"`
	IndexEntries    int      `json:"indexEntries"`
	HasOpenWork     bool     `json:"hasOpenWork"`
	RequiresReview  bool     `json:"requiresReview"`
	RequiresCleanup bool     `json:"requiresCleanup"`
	Action          string   `json:"action,omitempty"`
	Evidence        []string `json:"evidence,omitempty"`
	Boundary        []string `json:"boundary,omitempty"`
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
		openUnits := status.CandidateFiles + status.ToolingFiles + status.IndexEntries
		if openUnits == 0 && status.IndexPath != "" {
			openUnits = 1
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
		return inventory
	}
	if inventory.Total > 0 {
		inventory.Ready = false
		inventory.Summary = "pack-memory candidate inventory has open review/cleanup work"
		inventory.NextAction = "review listed pack-memory candidates, record decisions, cleanup candidate roots/indexPath, then rerun release-check/status"
		inventory.Warnings = []string{"open pack-memory candidates require review/cleanup before release handoff"}
	} else {
		inventory.NextAction = "no pack-memory candidate cleanup is pending"
	}
	return inventory
}

func releaseHandoffPackMemoryCandidateStatus(repo string, pack manifest.PackSummary) (ReleaseHandoffPackMemoryCandidateStatus, error) {
	packRoot := filepath.Join(repo, "packs", filepath.FromSlash(pack.ID))
	candidateRoot := filepath.Join(packRoot, "promote-candidates")
	toolingRoot := filepath.Join(packRoot, "tooling", "candidates")
	indexPath := filepath.Join(candidateRoot, "index.json")
	candidateFiles, err := countCandidateFiles(candidateRoot)
	if err != nil {
		return ReleaseHandoffPackMemoryCandidateStatus{}, fmt.Errorf("pack-memory candidate scan failed for %s: %w", pack.ID, err)
	}
	toolingFiles, err := countCandidateFiles(toolingRoot)
	if err != nil {
		return ReleaseHandoffPackMemoryCandidateStatus{}, fmt.Errorf("pack-memory tooling candidate scan failed for %s: %w", pack.ID, err)
	}
	indexEntries, indexExists, err := countCandidateIndexEntries(indexPath)
	if err != nil {
		return ReleaseHandoffPackMemoryCandidateStatus{}, fmt.Errorf("pack-memory candidate index invalid for %s: %w", pack.ID, err)
	}
	candidateRootRel := filepath.ToSlash(filepath.Join("packs", pack.ID, "promote-candidates"))
	toolingRootRel := filepath.ToSlash(filepath.Join("packs", pack.ID, "tooling", "candidates"))
	indexRel := ""
	if indexExists {
		indexRel = filepath.ToSlash(filepath.Join(candidateRootRel, "index.json"))
	}
	status := ReleaseHandoffPackMemoryCandidateStatus{
		Pack:           pack.ID,
		Maturity:       pack.Maturity,
		CandidateRoot:  candidateRootRel,
		ToolingRoot:    toolingRootRel,
		IndexPath:      indexRel,
		CandidateFiles: candidateFiles,
		ToolingFiles:   toolingFiles,
		IndexEntries:   indexEntries,
		HasOpenWork:    candidateFiles > 0 || toolingFiles > 0 || indexEntries > 0 || indexExists,
	}
	if !status.HasOpenWork {
		return status, nil
	}
	status.RequiresReview = candidateFiles > 0 || toolingFiles > 0
	status.RequiresCleanup = true
	status.Action = "review candidate files against pack targets, record accept/reject/superseded decisions, then cleanup candidatePath and indexPath"
	if !status.RequiresReview && indexExists {
		status.Action = "cleanup stale pack-memory candidate indexPath or regenerate candidates before review"
	}
	status.Evidence = append(status.Evidence, "candidateRoot "+candidateRootRel, "toolingRoot "+toolingRootRel)
	if candidateFiles > 0 {
		status.Evidence = append(status.Evidence, fmt.Sprintf("promote-candidates files=%d", candidateFiles))
	}
	if toolingFiles > 0 {
		status.Evidence = append(status.Evidence, fmt.Sprintf("tooling/candidates files=%d", toolingFiles))
	}
	if indexExists {
		status.Evidence = append(status.Evidence, fmt.Sprintf("indexPath %s entries=%d", indexRel, indexEntries))
	}
	status.Boundary = []string{
		"release handoff only inventories pack-memory candidate residue; it does not merge or delete candidates",
		"review candidates before merge; do not write authority/confirmed",
		"do not promote case-specific artifacts, traces, dumps, captures, payloads, flags, or customer data",
	}
	return status, nil
}

func countCandidateFiles(root string) (int, error) {
	if _, err := os.Stat(root); os.IsNotExist(err) {
		return 0, nil
	} else if err != nil {
		return 0, err
	}
	count := 0
	err := filepath.WalkDir(root, func(path string, d os.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if d.IsDir() {
			return nil
		}
		if strings.HasSuffix(d.Name(), ".candidate.md") {
			count++
		}
		return nil
	})
	return count, err
}

func countCandidateIndexEntries(path string) (int, bool, error) {
	b, err := os.ReadFile(path)
	if os.IsNotExist(err) {
		return 0, false, nil
	}
	if err != nil {
		return 0, false, err
	}
	entries := []struct {
		Path      string `json:"path"`
		Candidate string `json:"candidate"`
	}{}
	if err := json.Unmarshal(b, &entries); err != nil {
		return 0, true, err
	}
	return len(entries), true, nil
}

func releaseHandoffPackMemoryCandidateDetails(inventory ReleaseHandoffPackMemoryCandidateList) []string {
	details := []string{
		fmt.Sprintf("openPacks=%d total=%d ready=%t", len(inventory.Packs), inventory.Total, inventory.Ready),
		fmt.Sprintf("nextAction=%s", inventory.NextAction),
	}
	for _, pack := range inventory.Packs {
		details = append(details, fmt.Sprintf("pack=%s maturity=%s candidateFiles=%d toolingFiles=%d indexEntries=%d requiresReview=%t requiresCleanup=%t action=%s", pack.Pack, pack.Maturity, pack.CandidateFiles, pack.ToolingFiles, pack.IndexEntries, pack.RequiresReview, pack.RequiresCleanup, pack.Action))
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
	sectionLines := lines[start+1 : end]
	for _, line := range sectionLines {
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
	latest.Handoff = latestBatchHandoff(latest, strings.Join(sectionLines, "\n"))
	return latest
}

func latestBatchHandoff(latest ReleaseHandoffLatestBatch, section string) ReleaseHandoffLatestBatchHandoff {
	handoff := ReleaseHandoffLatestBatchHandoff{
		Completed:            strings.Contains(latest.Status, "已完成"),
		LocalValidationReady: latestBatchHasLocalValidation(section),
		ReleaseCheckReady:    strings.Contains(strings.ToLower(section), "release-check ready=true"),
		RemoteReleaseGate:    latestBatchRemoteReleaseGate(section),
		CommitRefs:           latestBatchCommitRefs(section),
		Evidence:             latestBatchEvidence(section),
	}
	handoff.NextAction = latestBatchNextAction(handoff)
	return handoff
}

func latestBatchHasLocalValidation(text string) bool {
	lower := strings.ToLower(text)
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

func latestBatchRemoteReleaseGate(text string) string {
	lower := strings.ToLower(text)
	switch {
	case strings.Contains(text, "远程 release-gate inspection 待") || strings.Contains(lower, "remote release-gate inspection pending") || strings.Contains(lower, "release-gate inspection pending"):
		return "not-recorded"
	case strings.Contains(text, "steps: []") && strings.Contains(lower, "completed failure"):
		return "blocked: completed failure with jobs steps=[]"
	case strings.Contains(text, "steps: []"):
		return "blocked: jobs steps=[]"
	case latestBatchRemoteGreen(text, lower):
		return "green"
	case strings.Contains(text, "远程 release-gate") || strings.Contains(lower, "release-gate run"):
		return "inspected"
	default:
		return "not-recorded"
	}
}

func latestBatchRemoteGreen(text, lower string) bool {
	if strings.Contains(text, "不能声明远程 CI green") || strings.Contains(lower, "cannot claim remote ci green") || strings.Contains(lower, "not remote ci green") {
		return false
	}
	return strings.Contains(lower, "remote ci green") || strings.Contains(text, "远程 CI green")
}

func latestBatchCommitRefs(text string) []string {
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
		if looksLikeCommitRef(token) && !seen[token] {
			seen[token] = true
			refs = append(refs, token)
		}
		text = text[end+1:]
	}
	return refs
}

func looksLikeCommitRef(value string) bool {
	if len(value) < 7 || len(value) > 40 {
		return false
	}
	hasHexLetter := false
	for _, r := range value {
		switch {
		case r >= '0' && r <= '9':
		case r >= 'a' && r <= 'f':
			hasHexLetter = true
		case r >= 'A' && r <= 'F':
			hasHexLetter = true
		default:
			return false
		}
	}
	return hasHexLetter
}

func latestBatchEvidence(text string) []string {
	lower := strings.ToLower(text)
	evidence := []string{}
	for _, candidate := range []struct {
		match string
		label string
	}{
		{match: "public cli", label: "public CLI product-path validation recorded"},
		{match: "go run ./cmd/rekit -- -command release-check -format json", label: "release-check -Format json recorded"},
		{match: "go run ./cmd/rekit -- -command status", label: "status handoff recorded"},
		{match: "go run ./cmd/rekit -- -command packs", label: "packs inventory recorded"},
		{match: "go run ./cmd/rekit -- -command doctor", label: "doctor validation recorded"},
		{match: "go test ./...", label: "go test ./... recorded"},
		{match: "go vet ./...", label: "go vet ./... recorded"},
		{match: "git diff --check", label: "git diff --check recorded"},
		{match: "release-check ready=true", label: "release-check ready=true recorded"},
		{match: "steps: []", label: "remote release-gate jobs steps=[] recorded"},
	} {
		if !strings.Contains(lower, candidate.match) {
			continue
		}
		if candidate.match == "steps: []" && latestBatchRemoteReleaseGate(text) == "not-recorded" {
			continue
		}
		evidence = append(evidence, candidate.label)
	}
	return evidence
}

func latestBatchNextAction(handoff ReleaseHandoffLatestBatchHandoff) string {
	switch {
	case !handoff.Completed:
		return "finish the current batch before treating status as a handoff"
	case !handoff.LocalValidationReady:
		return "run the full local release minimum and update docs/batch-plan.md"
	case handoff.RemoteReleaseGate == "not-recorded":
		return "inspect the remote release-gate run before claiming remote CI status"
	case strings.HasPrefix(handoff.RemoteReleaseGate, "blocked:"):
		return "treat remote release-gate steps=[] as a known blocker and continue the next Windows-verifiable batch"
	default:
		return "continue the next batch from docs/context-routing.md and docs/batch-plan.md"
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
