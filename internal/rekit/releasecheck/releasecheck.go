package releasecheck

import (
	"encoding/json"
	"fmt"
	"os"
	"path"
	"path/filepath"
	"sort"
	"strings"

	"github.com/shuiyu486/re-context-kits/internal/rekit/caseshim"
	"github.com/shuiyu486/re-context-kits/internal/rekit/defaultdocs"
	"github.com/shuiyu486/re-context-kits/internal/rekit/manifest"
	"github.com/shuiyu486/re-context-kits/internal/rekit/productioncontract"
)

const (
	commandName                 = "release-check"
	CanonicalGoTestCommand      = "go test -count=1 -p=2 -timeout=30m ./..."
	CanonicalGoPackSmokeCommand = "go test -count=1 -timeout=30m ./internal/rekit/cli -run '^TestRunGoSkeletonPackSmokeMatrix$'"
)

type Result struct {
	Command               string                                 `json:"command"`
	SchemaVersion         int                                    `json:"schemaVersion"`
	IsMutation            bool                                   `json:"isMutation"`
	RepoRoot              string                                 `json:"repoRoot"`
	Ready                 bool                                   `json:"ready"`
	Summary               string                                 `json:"summary"`
	GateProfile           GateProfile                            `json:"gateProfile"`
	CIReleaseGate         CIReleaseGate                          `json:"ciReleaseGate"`
	RecommendedMinimum    []GateStep                             `json:"recommendedMinimum"`
	RequiredCommands      []GateStep                             `json:"requiredCommands"`
	Documents             []DocumentCheck                        `json:"documents"`
	Packs                 []manifest.PackSummary                 `json:"packs"`
	ProductionRegistry    productioncontract.RegistryAdmission   `json:"productionRegistry"`
	ProductionPacks       []productioncontract.Admission         `json:"productionPacks"`
	CapabilityContract    productioncontract.CapabilityAdmission `json:"capabilityContract"`
	PowerShellDeprecation PowerShellDeprecation                  `json:"powerShellDeprecation"`
	GoNativePublicSurface GoNativePublicSurface                  `json:"goNativePublicSurface"`
	PublicFacadeRemoval   PublicFacadeRemoval                    `json:"publicFacadeRemoval"`
	CaseShim              caseshim.Readiness                     `json:"caseShim"` // Legacy /rekit + .rekit compatibility gate; retained key preserves schema history.
	PublicDefaultDocs     defaultdocs.Readiness                  `json:"publicDefaultDocs"`
	ReleaseHandoff        ReleaseHandoff                         `json:"releaseHandoff"`
	ReadinessLayers       ReadinessLayers                        `json:"readinessLayers"`
	HeavyToolGateActions  []string                               `json:"heavyToolGateActions"`
	Boundaries            []string                               `json:"boundaries"`
	KnownGaps             []string                               `json:"knownGaps"`
	Warnings              []string                               `json:"warnings"`
}

type ReadinessLayers struct {
	InventoryReady             bool                              `json:"inventoryReady"`
	LocalValidationReady       bool                              `json:"localValidationReady"`
	RealWindowsAcceptanceReady bool                              `json:"realWindowsAcceptanceReady"`
	RemoteCIGreen              bool                              `json:"remoteCIGreen"`
	FormalReleaseReady         bool                              `json:"formalReleaseReady"`
	RepositoryInventory        RepositoryInventoryReadinessLayer `json:"repositoryInventory"`
	LocalValidation            LocalValidationReadinessLayer     `json:"localValidation"`
	RealWindowsAcceptance      WindowsAcceptanceReadinessLayer   `json:"realWindowsAcceptance"`
	GitLocalPublication        GitLocalPublicationReadinessLayer `json:"gitLocalPublication"`
	RemoteCI                   RemoteCIReadinessLayer            `json:"remoteCI"`
	FormalRelease              FormalReleaseReadinessLayer       `json:"formalRelease"`
}

type RepositoryInventoryReadinessLayer struct {
	State string `json:"state"`
	Ready bool   `json:"ready"`
}

type LocalValidationReadinessLayer struct {
	State                         string `json:"state"`
	Ready                         bool   `json:"ready"`
	ExactReceiptInspectionPresent bool   `json:"exactReceiptInspectionPresent"`
}

type WindowsAcceptanceReadinessLayer struct {
	State                        string `json:"state"`
	Ready                        bool   `json:"ready"`
	StructuredObservationPresent bool   `json:"structuredObservationPresent"`
}

type GitLocalPublicationReadinessLayer struct {
	State                           string `json:"state"`
	Ready                           bool   `json:"ready"`
	ExactPostPushObservationPresent bool   `json:"exactPostPushObservationPresent"`
	LocalTrackingRefOnly            bool   `json:"localTrackingRefOnly"`
}

type RemoteCIReadinessLayer struct {
	State                        string                   `json:"state"`
	StructuredObservationPresent bool                     `json:"structuredObservationPresent"`
	CanClaimGreen                bool                     `json:"canClaimGreen"`
	DocumentedClaim              ReadinessDocumentedClaim `json:"documentedClaim"`
}

type ReadinessDocumentedClaim struct {
	Present       bool   `json:"present"`
	Claim         string `json:"claim"`
	Source        string `json:"source"`
	Authoritative bool   `json:"authoritative"`
}

type FormalReleaseReadinessLayer struct {
	State            string `json:"state"`
	CanClaimReleased bool   `json:"canClaimReleased"`
}

type ReleaseCheckResultCounts struct {
	RecommendedMinimum   int
	RequiredCommands     int
	Documents            int
	Packs                int
	GateProfileSteps     int
	HeavyToolGateActions int
	Boundaries           int
	KnownGaps            int
	Warnings             int
}

func ReleaseCheckResultCountsFor(result Result) ReleaseCheckResultCounts {
	return ReleaseCheckResultCounts{
		RecommendedMinimum:   len(result.RecommendedMinimum),
		RequiredCommands:     len(result.RequiredCommands),
		Documents:            len(result.Documents),
		Packs:                len(result.Packs),
		GateProfileSteps:     len(result.GateProfile.Steps),
		HeavyToolGateActions: len(result.HeavyToolGateActions),
		Boundaries:           len(result.Boundaries),
		KnownGaps:            len(result.KnownGaps),
		Warnings:             len(result.Warnings),
	}
}

type GateProfile struct {
	Name               string     `json:"name"`
	Description        string     `json:"description"`
	DefaultFor         []string   `json:"defaultFor"`
	Ready              bool       `json:"ready"`
	StepCount          int        `json:"stepCount"`
	LargeMatrixDefault bool       `json:"largeMatrixDefault"`
	Steps              []GateStep `json:"steps"`
}

type GateStep struct {
	Command    string   `json:"command"`
	Executable string   `json:"executable,omitempty"`
	Arguments  []string `json:"arguments,omitempty"`
	Source     string   `json:"source"`
	Kind       string   `json:"kind"`
	RepoPath   string   `json:"repoPath,omitempty"`
	Present    bool     `json:"present"`
	Required   bool     `json:"required"`
	InCatalog  bool     `json:"inCatalog"`
	Resolved   bool     `json:"resolved"`
}

type DocumentCheck struct {
	Path    string `json:"path"`
	Present bool   `json:"present"`
	Purpose string `json:"purpose"`
}

type catalog struct {
	RecommendedMinimum []string `json:"recommendedMinimum"`
	GlobalBoundaries   []string `json:"globalBoundaries"`
}

var requiredCommands = []GateStep{
	{Command: "go run ./cmd/rekit -- -Command release-check -Format json", Executable: "go", Arguments: []string{"run", "./cmd/rekit", "--", "-Command", "release-check", "-Format", "json"}},
	{Command: "go run ./cmd/rekit -- -Command status", Executable: "go", Arguments: []string{"run", "./cmd/rekit", "--", "-Command", "status"}},
	{Command: "go run ./cmd/rekit -- -Command packs", Executable: "go", Arguments: []string{"run", "./cmd/rekit", "--", "-Command", "packs"}},
	{Command: "go run ./cmd/rekit -- -Command doctor", Executable: "go", Arguments: []string{"run", "./cmd/rekit", "--", "-Command", "doctor"}},
	{Command: CanonicalGoTestCommand, Executable: "go", Arguments: []string{"test", "-count=1", "-p=2", "-timeout=30m", "./..."}},
	{Command: "go vet ./...", Executable: "go", Arguments: []string{"vet", "./..."}},
	{Command: "git diff --check", Executable: "git", Arguments: []string{"diff", "--check"}},
}

var requiredDocuments = []DocumentCheck{
	{Path: "docs/context-routing.md", Purpose: "progressive-disclosure router and read-first policy"},
	{Path: "docs/batch-plan.md", Purpose: "current batch state and next candidates"},
	{Path: "docs/batch-history.md", Purpose: "archived batch details for bounded active-plan rotation"},
	{Path: "docs/release-readiness.md", Purpose: "release gate and known gaps"},
	{Path: "docs/mission-control-product-direction.md", Purpose: "Mission Control product north star"},
	{Path: "docs/autonomous-goal.md", Purpose: "long-term autonomous goal and handoff guide"},
	{Path: "docs/go-first-convergence-plan.md", Purpose: "Go-first stage completion signals"},
	{Path: "docs/powershell-deprecation.md", Purpose: "PowerShell owner and freeze strategy"},
	{Path: "rekit/tests/catalog.json", Purpose: "machine-readable smoke catalog"},
	{Path: "rekit/tests/README.md", Purpose: "smoke selection guide"},
	{Path: "CHANGELOG.md", Purpose: "release notes"},
}

func Build(repoRoot string) (Result, error) {
	repo, err := filepath.Abs(repoRoot)
	if err != nil {
		return Result{}, err
	}
	cat, err := loadCatalog(repo)
	if err != nil {
		return Result{}, err
	}
	packs, err := manifest.List(repo)
	if err != nil {
		return Result{}, err
	}
	productionRegistry := productioncontract.BuildRegistryAdmission(packs)
	productionPacks := []productioncontract.Admission{}
	for _, pack := range packs {
		if pack.Maturity == "mature" {
			productionPacks = append(productionPacks, productioncontract.BuildAdmission(repo, pack))
		}
	}
	check := Result{
		Command:               commandName,
		SchemaVersion:         1,
		IsMutation:            false,
		RepoRoot:              repo,
		Ready:                 true,
		Summary:               "release gate inventory ok",
		CIReleaseGate:         ciReleaseGate(repo),
		RecommendedMinimum:    catalogGateSteps(repo, cat.RecommendedMinimum),
		RequiredCommands:      requiredGateSteps(repo, requiredCommands, cat.RecommendedMinimum),
		Documents:             documentChecks(repo, requiredDocuments),
		Packs:                 packs,
		ProductionRegistry:    productionRegistry,
		ProductionPacks:       productionPacks,
		CapabilityContract:    productioncontract.BuildCapabilityAdmission(repo),
		PowerShellDeprecation: powerShellDeprecation(repo),
		GoNativePublicSurface: goNativePublicSurface(repo),
		CaseShim:              caseshim.Inspect(repo),
		PublicDefaultDocs:     defaultdocs.Inspect(repo),
		HeavyToolGateActions:  heavyToolGateActions(packs),
		Boundaries:            append([]string{}, cat.GlobalBoundaries...),
		KnownGaps:             knownGaps(repo),
		Warnings:              []string{},
	}
	check.GateProfile = gateProfile(check.RequiredCommands)
	if crossWarnings := goNativePublicSurfaceCrossWarnings(check.GoNativePublicSurface, check.PowerShellDeprecation.PublicFacade); len(crossWarnings) > 0 {
		check.GoNativePublicSurface.Warnings = append(check.GoNativePublicSurface.Warnings, crossWarnings...)
		check.GoNativePublicSurface.Ready = false
		check.GoNativePublicSurface.Summary = "Go-native public command surface inventory has warnings"
	}
	check.PublicFacadeRemoval = publicFacadeRemovalInventory(repo, check.PowerShellDeprecation, check.GoNativePublicSurface)
	if !check.GateProfile.Ready {
		check.Ready = false
		check.Warnings = append(check.Warnings, "release gate profile has unresolved commands or missing repo-local scripts")
	}
	for _, step := range check.RequiredCommands {
		if !step.InCatalog {
			check.Ready = false
			check.Warnings = append(check.Warnings, fmt.Sprintf("required command missing from catalog recommendedMinimum: %s", step.Command))
		}
	}
	for _, doc := range check.Documents {
		if !doc.Present {
			check.Ready = false
			check.Warnings = append(check.Warnings, fmt.Sprintf("required release document missing: %s", doc.Path))
		}
	}
	for _, pack := range check.Packs {
		if !pack.SchemaValid {
			check.Ready = false
			check.Warnings = append(check.Warnings, fmt.Sprintf("pack manifest invalid: %s: %s", pack.ID, pack.Error))
		}
	}
	if !check.ProductionRegistry.Ready {
		check.Ready = false
		for _, warning := range check.ProductionRegistry.Warnings {
			check.Warnings = append(check.Warnings, "production registry: "+warning)
		}
	}
	for _, pack := range check.ProductionPacks {
		if pack.Ready {
			continue
		}
		check.Ready = false
		for _, warning := range pack.Warnings {
			check.Warnings = append(check.Warnings, fmt.Sprintf("production pack %s: %s", pack.Pack, warning))
		}
	}
	if !check.CapabilityContract.Ready {
		check.Ready = false
		for _, warning := range check.CapabilityContract.Warnings {
			check.Warnings = append(check.Warnings, "capability contract: "+warning)
		}
	}
	if ReleaseCheckResultCountsFor(check).HeavyToolGateActions == 0 {
		check.Ready = false
		check.Warnings = append(check.Warnings, "no heavy-tool gate actions declared by pack manifests")
	}
	if !check.CIReleaseGate.Ready {
		check.Ready = false
		check.Warnings = append(check.Warnings, check.CIReleaseGate.Warnings...)
	}
	if !check.PowerShellDeprecation.Ready {
		check.Ready = false
		check.Warnings = append(check.Warnings, check.PowerShellDeprecation.Warnings...)
	}
	if !check.GoNativePublicSurface.Ready {
		check.Ready = false
		check.Warnings = append(check.Warnings, check.GoNativePublicSurface.Warnings...)
	}
	if !check.PublicFacadeRemoval.Ready {
		check.Ready = false
		check.Warnings = append(check.Warnings, check.PublicFacadeRemoval.Warnings...)
	}
	if !check.CaseShim.Ready {
		check.Ready = false
		check.Warnings = append(check.Warnings, check.CaseShim.Warnings...)
	}
	if !check.PublicDefaultDocs.Ready {
		check.Ready = false
		check.Warnings = append(check.Warnings, check.PublicDefaultDocs.Warnings...)
	}
	if !check.Ready {
		check.Summary = "release gate inventory has warnings"
	}
	check.ReleaseHandoff = releaseHandoff(repo, check)
	if !check.ReleaseHandoff.Ready {
		check.Ready = false
		check.Warnings = append(check.Warnings, check.ReleaseHandoff.Warnings...)
	}
	if !check.Ready {
		check.Summary = "release gate inventory has warnings"
	}
	check.ReadinessLayers = readinessLayers(check)
	return check, nil
}

func readinessLayers(result Result) ReadinessLayers {
	layers := ReadinessLayers{
		InventoryReady: result.Ready,
		RepositoryInventory: RepositoryInventoryReadinessLayer{
			State: "not-ready",
			Ready: result.Ready,
		},
		LocalValidation: LocalValidationReadinessLayer{
			State: "not-observed",
		},
		RealWindowsAcceptance: WindowsAcceptanceReadinessLayer{
			State: "not-observed",
		},
		GitLocalPublication: GitLocalPublicationReadinessLayer{
			State:                "not-observed",
			LocalTrackingRefOnly: true,
		},
		RemoteCI: RemoteCIReadinessLayer{
			State: "not-observed",
			DocumentedClaim: ReadinessDocumentedClaim{
				Source: "releaseHandoff.latestBatch.handoff.remoteReleaseGate",
			},
		},
		FormalRelease: FormalReleaseReadinessLayer{
			State: "not-evaluated",
		},
	}
	if result.Ready {
		layers.RepositoryInventory.State = "ready"
	}
	latest := result.ReleaseHandoff.LatestBatch.Handoff
	validationReceipt := latest.LocalValidationReceipt
	postPushReceipt := latest.PostPushReceipt
	if active := result.ReleaseHandoff.ActiveRoute; active.Present {
		validationReceipt = active.LocalValidationReceipt
		postPushReceipt = active.PostPushReceipt
	}
	if validationReceipt != nil {
		layers.LocalValidation.State = validationReceipt.State
		layers.LocalValidation.ExactReceiptInspectionPresent = true
		layers.LocalValidation.Ready = validationReceipt.Ready && validationReceipt.State == "validated-implementation-commit"
		layers.LocalValidationReady = layers.LocalValidation.Ready
	}
	if postPushReceipt != nil {
		layers.GitLocalPublication.State = postPushReceipt.State
		layers.GitLocalPublication.ExactPostPushObservationPresent = true
		layers.GitLocalPublication.Ready = postPushReceipt.Ready && postPushReceipt.State == "post-push-complete"
	}
	if claim := strings.TrimSpace(latest.RemoteReleaseGate); claim != "" && claim != "not-recorded" {
		layers.RemoteCI.DocumentedClaim.Present = true
		layers.RemoteCI.DocumentedClaim.Claim = claim
	}
	return layers
}

func loadCatalog(repo string) (catalog, error) {
	data, err := os.ReadFile(filepath.Join(repo, "rekit", "tests", "catalog.json"))
	if err != nil {
		return catalog{}, err
	}
	var cat catalog
	if err := json.Unmarshal(data, &cat); err != nil {
		return catalog{}, err
	}
	return cat, nil
}

func heavyToolGateActions(packs []manifest.PackSummary) []string {
	seen := map[string]bool{}
	for _, pack := range packs {
		for _, action := range pack.HeavyToolGateActions {
			action = strings.ToLower(strings.TrimSpace(action))
			if action != "" {
				seen[action] = true
			}
		}
	}
	actions := make([]string, 0, len(seen))
	for action := range seen {
		actions = append(actions, action)
	}
	sort.Strings(actions)
	return actions
}

func catalogGateSteps(repo string, commands []string) []GateStep {
	steps := make([]GateStep, 0, len(commands))
	for _, command := range commands {
		steps = append(steps, gateStep(repo, command, "catalog", true, true))
	}
	return steps
}

func requiredGateSteps(repo string, commands []GateStep, catalogCommands []string) []GateStep {
	catalogSet := map[string]bool{}
	for _, command := range catalogCommands {
		catalogSet[normalizeCommand(command)] = true
	}
	steps := make([]GateStep, 0, len(commands))
	for _, carrier := range commands {
		inCatalog := catalogSet[normalizeCommand(carrier.Command)]
		step := gateStep(repo, carrier.Command, "release-check", true, inCatalog)
		step.Executable = carrier.Executable
		step.Arguments = append([]string{}, carrier.Arguments...)
		step.Resolved = step.Resolved && strings.TrimSpace(step.Executable) != ""
		steps = append(steps, step)
	}
	return steps
}

func gateProfile(steps []GateStep) GateProfile {
	profile := GateProfile{
		Name:               "local-ci-minimum",
		Description:        "Go-owned local/CI release gate profile; enumerates required checks and resolves repo-local scripts without executing them.",
		DefaultFor:         []string{"local", "ci"},
		Ready:              true,
		StepCount:          len(steps),
		LargeMatrixDefault: false,
		Steps:              append([]GateStep{}, steps...),
	}
	for _, step := range steps {
		if !step.Resolved {
			profile.Ready = false
			break
		}
	}
	return profile
}

func gateStep(repo, command, source string, required, inCatalog bool) GateStep {
	kind, repoPath := resolveGateCommand(command)
	present := true
	if repoPath != "" {
		_, err := os.Stat(filepath.Join(repo, filepath.FromSlash(repoPath)))
		present = err == nil
	}
	return GateStep{
		Command:   command,
		Source:    source,
		Kind:      kind,
		RepoPath:  repoPath,
		Present:   present,
		Required:  required,
		InCatalog: inCatalog,
		Resolved:  inCatalog && present,
	}
}

func resolveGateCommand(command string) (string, string) {
	normalized := normalizeCommand(command)
	fields := strings.Fields(normalized)
	if len(fields) == 0 {
		return "unknown", ""
	}
	first := strings.TrimPrefix(strings.ReplaceAll(fields[0], "\\", "/"), "./")
	if strings.EqualFold(first, "go") {
		if len(fields) >= 3 && strings.EqualFold(fields[1], "run") && strings.HasPrefix(fields[2], "./") {
			return "go-run", strings.TrimPrefix(strings.ReplaceAll(fields[2], "\\", "/"), "./")
		}
		return "go-check", ""
	}
	if strings.EqualFold(first, "git") {
		return "git-check", ""
	}
	if strings.HasSuffix(strings.ToLower(first), ".ps1") {
		repoPath := first
		if !strings.Contains(repoPath, "/") {
			repoPath = path.Join("rekit", "tests", repoPath)
		}
		if strings.EqualFold(repoPath, "rekit/rekit.ps1") {
			return "powershell-facade", repoPath
		}
		return "powershell-smoke", repoPath
	}
	return "external", ""
}

func documentChecks(repo string, docs []DocumentCheck) []DocumentCheck {
	checks := make([]DocumentCheck, 0, len(docs))
	for _, doc := range docs {
		_, err := os.Stat(filepath.Join(repo, filepath.FromSlash(doc.Path)))
		doc.Present = err == nil
		checks = append(checks, doc)
	}
	return checks
}

func knownGaps(repo string) []string {
	data, err := os.ReadFile(filepath.Join(repo, "docs", "release-readiness.md"))
	if err != nil {
		return nil
	}
	return markdownBulletsInSection(string(data), "## Known gaps")
}

func markdownBulletsInSection(text, heading string) []string {
	lines := strings.Split(strings.ReplaceAll(text, "\r\n", "\n"), "\n")
	inSection := false
	var bullets []string
	for _, line := range lines {
		trimmed := strings.TrimSpace(line)
		if trimmed == heading {
			inSection = true
			continue
		}
		if inSection && strings.HasPrefix(trimmed, "## ") {
			break
		}
		if inSection && strings.HasPrefix(trimmed, "- ") {
			bullets = append(bullets, strings.TrimSpace(strings.TrimPrefix(trimmed, "- ")))
		}
	}
	return bullets
}

func normalizeCommand(command string) string {
	command = strings.TrimSpace(strings.ReplaceAll(command, "\\", "/"))
	command = strings.TrimPrefix(command, "./")
	fields := strings.Fields(command)
	return strings.Join(fields, " ")
}
