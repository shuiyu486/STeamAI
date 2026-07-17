package releasecheck

import (
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"slices"
	"sort"
	"strings"
)

type PowerShellDeprecation struct {
	StrategyDocument   string                       `json:"strategyDocument"`
	Ready              bool                         `json:"ready"`
	Summary            string                       `json:"summary"`
	CommandOwnership   []PowerShellCommandOwner     `json:"commandOwnership"`
	ModuleStatus       []PowerShellModuleStatus     `json:"moduleStatus"`
	FreezeGates        []PowerShellFreezeGate       `json:"freezeGates"`
	BlockedMigrations  []string                     `json:"blockedMigrations"`
	FallbackRetirement PowerShellFallbackRetirement `json:"fallbackRetirement"`
	FacadeRuntime      PowerShellFacadeRuntime      `json:"facadeRuntime"`
	PublicFacade       PowerShellPublicFacade       `json:"publicFacade"`
	ModuleRemoval      PowerShellModuleRemoval      `json:"moduleRemoval"`
	ModuleReferences   PowerShellModuleReferences   `json:"moduleReferences"`
	Warnings           []string                     `json:"warnings"`
}

type PowerShellDeprecationCounts struct {
	CommandOwnership                        int
	ModuleStatus                            int
	FreezeGates                             int
	BlockedMigrations                       int
	Warnings                                int
	FallbackGoDefaultCommands               int
	FallbackNoFallbackCommands              int
	FallbackCandidateCommands               int
	FallbackBlockedCommands                 int
	FallbackRemovalCandidateModules         int
	FallbackRetiredModules                  int
	FallbackWarnings                        int
	FacadeRuntimeForbiddenPatterns          int
	FacadeRuntimeRequiredPatterns           int
	FacadeRuntimeWarnings                   int
	PublicFacadeCommandSurface              int
	PublicFacadeGoDefaultCommands           int
	PublicFacadeNoFallbackCommands          int
	PublicFacadeWarnings                    int
	ModuleRemovalCandidateModules           int
	ModuleRemovalRetiredModules             int
	ModuleRemovalUndocumentedModules        int
	ModuleRemovalFacadeRuntimeDependencies  int
	ModuleRemovalWarnings                   int
	ModuleReferencesTotal                   int
	ModuleReferencesActiveTestDependencies  int
	ModuleReferencesCompatibilityFixtures   int
	ModuleReferencesInventoryGuards         int
	ModuleReferencesDocumentationReferences int
	ModuleReferencesHistoricalReferences    int
	ModuleReferencesRemovalBlockers         int
	ModuleReferencesUnclassifiedReferences  int
	ModuleReferencesWarnings                int
}

func PowerShellDeprecationCountsFor(deprecation PowerShellDeprecation) PowerShellDeprecationCounts {
	return PowerShellDeprecationCounts{
		CommandOwnership:                        len(deprecation.CommandOwnership),
		ModuleStatus:                            len(deprecation.ModuleStatus),
		FreezeGates:                             len(deprecation.FreezeGates),
		BlockedMigrations:                       len(deprecation.BlockedMigrations),
		Warnings:                                len(deprecation.Warnings),
		FallbackGoDefaultCommands:               len(deprecation.FallbackRetirement.GoDefaultCommands),
		FallbackNoFallbackCommands:              len(deprecation.FallbackRetirement.NoFallbackCommands),
		FallbackCandidateCommands:               len(deprecation.FallbackRetirement.CandidateCommands),
		FallbackBlockedCommands:                 len(deprecation.FallbackRetirement.BlockedCommands),
		FallbackRemovalCandidateModules:         len(deprecation.FallbackRetirement.RemovalCandidateModules),
		FallbackRetiredModules:                  len(deprecation.FallbackRetirement.RetiredModules),
		FallbackWarnings:                        len(deprecation.FallbackRetirement.Warnings),
		FacadeRuntimeForbiddenPatterns:          len(deprecation.FacadeRuntime.ForbiddenPatterns),
		FacadeRuntimeRequiredPatterns:           len(deprecation.FacadeRuntime.RequiredPatterns),
		FacadeRuntimeWarnings:                   len(deprecation.FacadeRuntime.Warnings),
		PublicFacadeCommandSurface:              len(deprecation.PublicFacade.CommandSurface),
		PublicFacadeGoDefaultCommands:           len(deprecation.PublicFacade.GoDefaultCommands),
		PublicFacadeNoFallbackCommands:          len(deprecation.PublicFacade.NoFallbackCommands),
		PublicFacadeWarnings:                    len(deprecation.PublicFacade.Warnings),
		ModuleRemovalCandidateModules:           len(deprecation.ModuleRemoval.CandidateModules),
		ModuleRemovalRetiredModules:             len(deprecation.ModuleRemoval.RetiredModules),
		ModuleRemovalUndocumentedModules:        len(deprecation.ModuleRemoval.UndocumentedModules),
		ModuleRemovalFacadeRuntimeDependencies:  len(deprecation.ModuleRemoval.FacadeRuntimeDependencies),
		ModuleRemovalWarnings:                   len(deprecation.ModuleRemoval.Warnings),
		ModuleReferencesTotal:                   deprecation.ModuleReferences.TotalReferences,
		ModuleReferencesActiveTestDependencies:  len(deprecation.ModuleReferences.ActiveTestDependencies),
		ModuleReferencesCompatibilityFixtures:   len(deprecation.ModuleReferences.CompatibilityFixtures),
		ModuleReferencesInventoryGuards:         len(deprecation.ModuleReferences.InventoryGuards),
		ModuleReferencesDocumentationReferences: len(deprecation.ModuleReferences.DocumentationReferences),
		ModuleReferencesHistoricalReferences:    len(deprecation.ModuleReferences.HistoricalReferences),
		ModuleReferencesRemovalBlockers:         len(deprecation.ModuleReferences.RemovalBlockers),
		ModuleReferencesUnclassifiedReferences:  len(deprecation.ModuleReferences.UnclassifiedReferences),
		ModuleReferencesWarnings:                len(deprecation.ModuleReferences.Warnings),
	}
}

type PowerShellCommandOwner struct {
	Area      string   `json:"area"`
	Owner     string   `json:"owner"`
	Status    string   `json:"status"`
	Strategy  string   `json:"strategy"`
	Commands  []string `json:"commands"`
	GoDefault bool     `json:"goDefault"`
	Blocked   bool     `json:"blocked"`
}

type PowerShellModuleStatus struct {
	Path   string `json:"path"`
	Status string `json:"status"`
	Notes  string `json:"notes"`
}

type PowerShellFreezeGate struct {
	Name        string `json:"name"`
	Description string `json:"description"`
}

type PowerShellFacadeRuntime struct {
	Ready                      bool     `json:"ready"`
	Summary                    string   `json:"summary"`
	FacadePath                 string   `json:"facadePath"`
	LegacyModuleImportsPresent bool     `json:"legacyModuleImportsPresent"`
	CommandDispatcherPresent   bool     `json:"commandDispatcherPresent"`
	NoFallbackGuardPresent     bool     `json:"noFallbackGuardPresent"`
	GoDelegationPresent        bool     `json:"goDelegationPresent"`
	RetiredDispatcherError     bool     `json:"retiredDispatcherError"`
	ForbiddenPatterns          []string `json:"forbiddenPatterns"`
	RequiredPatterns           []string `json:"requiredPatterns"`
	Warnings                   []string `json:"warnings"`
}

type PowerShellPublicFacade struct {
	Ready                       bool     `json:"ready"`
	Summary                     string   `json:"summary"`
	FacadePath                  string   `json:"facadePath"`
	Present                     bool     `json:"present"`
	Retained                    bool     `json:"retained"`
	CommandSurface              []string `json:"commandSurface"`
	GoDefaultCommands           []string `json:"goDefaultCommands"`
	NoFallbackCommands          []string `json:"noFallbackCommands"`
	GoNativeAlternative         string   `json:"goNativeAlternative"`
	MigrationBoundaryDocumented bool     `json:"migrationBoundaryDocumented"`
	RemovalBoundaryDocumented   bool     `json:"removalBoundaryDocumented"`
	Warnings                    []string `json:"warnings"`
}

type PowerShellModuleRemoval struct {
	Ready                     bool                          `json:"ready"`
	Summary                   string                        `json:"summary"`
	CandidateModules          []PowerShellModuleRemovalItem `json:"candidateModules"`
	RetiredModules            []PowerShellModuleRemovalItem `json:"retiredModules"`
	UndocumentedModules       []string                      `json:"undocumentedModules"`
	FacadeRuntimeDependencies []string                      `json:"facadeRuntimeDependencies"`
	Warnings                  []string                      `json:"warnings"`
}

type PowerShellModuleRemovalItem struct {
	Path               string `json:"path"`
	Status             string `json:"status"`
	Notes              string `json:"notes"`
	Present            bool   `json:"present"`
	ReferencedByFacade bool   `json:"referencedByFacade"`
}

type PowerShellModuleReferences struct {
	Ready                   bool                        `json:"ready"`
	Summary                 string                      `json:"summary"`
	TotalReferences         int                         `json:"totalReferences"`
	ActiveTestDependencies  []PowerShellModuleReference `json:"activeTestDependencies"`
	CompatibilityFixtures   []PowerShellModuleReference `json:"compatibilityFixtures"`
	InventoryGuards         []PowerShellModuleReference `json:"inventoryGuards"`
	DocumentationReferences []PowerShellModuleReference `json:"documentationReferences"`
	HistoricalReferences    []PowerShellModuleReference `json:"historicalReferences"`
	RemovalBlockers         []PowerShellModuleReference `json:"removalBlockers"`
	UnclassifiedReferences  []PowerShellModuleReference `json:"unclassifiedReferences"`
	Warnings                []string                    `json:"warnings"`
}

type PowerShellModuleReference struct {
	Path    string `json:"path"`
	Line    int    `json:"line"`
	Kind    string `json:"kind"`
	Target  string `json:"target"`
	Snippet string `json:"snippet"`
}

type PowerShellFallbackRetirement struct {
	Ready                   bool                                  `json:"ready"`
	Summary                 string                                `json:"summary"`
	GoDefaultCommands       []string                              `json:"goDefaultCommands"`
	NoFallbackCommands      []string                              `json:"noFallbackCommands"`
	CandidateCommands       []PowerShellFallbackRetirementCommand `json:"candidateCommands"`
	BlockedCommands         []PowerShellFallbackRetirementCommand `json:"blockedCommands"`
	RemovalCandidateModules []PowerShellFallbackRetirementModule  `json:"removalCandidateModules"`
	RetiredModules          []PowerShellFallbackRetirementModule  `json:"retiredModules"`
	Warnings                []string                              `json:"warnings"`
}

type PowerShellFallbackRetirementCommand struct {
	Area     string   `json:"area"`
	Commands []string `json:"commands"`
	Status   string   `json:"status"`
	Strategy string   `json:"strategy"`
}

type PowerShellFallbackRetirementModule struct {
	Path   string `json:"path"`
	Status string `json:"status"`
	Notes  string `json:"notes"`
}

func powerShellDeprecation(repo string) PowerShellDeprecation {
	const strategyPath = "docs/powershell-deprecation.md"
	strategy := PowerShellDeprecation{
		StrategyDocument: strategyPath,
		Ready:            true,
		Summary:          "PowerShell deprecation inventory ok",
		Warnings:         []string{},
	}
	data, err := os.ReadFile(filepath.Join(repo, filepath.FromSlash(strategyPath)))
	if err != nil {
		strategy.Ready = false
		strategy.Summary = "PowerShell deprecation strategy missing"
		strategy.Warnings = append(strategy.Warnings, err.Error())
		return strategy
	}
	text := string(data)
	strategy.CommandOwnership = parsePowerShellCommandOwnership(text, powerShellValidateSet(repo), powerShellDefaultDelegationCommands(repo))
	strategy.ModuleStatus = parsePowerShellModuleStatus(text)
	strategy.FreezeGates = parsePowerShellFreezeGates(text)
	strategy.BlockedMigrations = markdownBulletsInSection(text, "## 禁止迁移清单")
	strategy.FallbackRetirement = powerShellFallbackRetirement(repo, strategy.CommandOwnership, strategy.ModuleStatus)
	strategy.FacadeRuntime = powerShellFacadeRuntime(repo)
	strategy.PublicFacade = powerShellPublicFacade(repo, strategy.ModuleStatus, strategy.FallbackRetirement)
	strategy.ModuleRemoval = powerShellModuleRemoval(repo, strategy.ModuleStatus, strategy.FacadeRuntime)
	strategy.ModuleReferences = powerShellModuleReferences(repo)
	strategy.Warnings = append(strategy.Warnings, powerShellDeprecationWarnings(repo, strategy)...)
	strategy.Warnings = append(strategy.Warnings, strategy.FallbackRetirement.Warnings...)
	strategy.Warnings = append(strategy.Warnings, strategy.FacadeRuntime.Warnings...)
	strategy.Warnings = append(strategy.Warnings, strategy.PublicFacade.Warnings...)
	strategy.Warnings = append(strategy.Warnings, strategy.ModuleRemoval.Warnings...)
	strategy.Warnings = append(strategy.Warnings, strategy.ModuleReferences.Warnings...)
	if PowerShellDeprecationCountsFor(strategy).Warnings > 0 {
		strategy.Ready = false
		strategy.Summary = "PowerShell deprecation inventory has warnings"
	}
	return strategy
}

func parsePowerShellCommandOwnership(text string, validateSet, defaultCommands map[string]bool) []PowerShellCommandOwner {
	rows := markdownTableRowsInSection(text, "## 命令归属矩阵")
	owners := make([]PowerShellCommandOwner, 0, len(rows))
	for _, cells := range rows {
		if len(cells) < 4 || cells[0] == "区域" {
			continue
		}
		area := stripInlineCode(cells[0])
		commands := commandTokens(area, validateSet)
		owner := PowerShellCommandOwner{
			Area:     area,
			Owner:    stripInlineCode(cells[1]),
			Status:   stripInlineCode(cells[2]),
			Strategy: stripInlineCode(cells[3]),
			Commands: commands,
		}
		combined := strings.ToLower(owner.Owner + " " + owner.Status + " " + owner.Strategy + " " + owner.Area)
		owner.Blocked = strings.Contains(combined, "blocked") || strings.Contains(combined, "禁止") || strings.Contains(combined, "不得")
		for _, token := range commands {
			if defaultCommands[token] {
				owner.GoDefault = true
				break
			}
		}
		owners = append(owners, owner)
	}
	return owners
}

func parsePowerShellModuleStatus(text string) []PowerShellModuleStatus {
	rows := markdownTableRowsInSection(text, "## PowerShell 模块状态")
	modules := make([]PowerShellModuleStatus, 0, len(rows))
	for _, cells := range rows {
		if len(cells) < 3 || cells[0] == "模块" {
			continue
		}
		modules = append(modules, PowerShellModuleStatus{
			Path:   stripInlineCode(cells[0]),
			Status: stripInlineCode(cells[1]),
			Notes:  stripInlineCode(cells[2]),
		})
	}
	return modules
}

func parsePowerShellFreezeGates(text string) []PowerShellFreezeGate {
	lines := markdownSectionLines(text, "## Freeze / deprecation gates")
	gates := []PowerShellFreezeGate{}
	pattern := regexp.MustCompile(`^\d+\. \*\*([^*]+)\*\*[:：]?\s*(.*)$`)
	for _, line := range lines {
		match := pattern.FindStringSubmatch(strings.TrimSpace(line))
		if len(match) != 3 {
			continue
		}
		gates = append(gates, PowerShellFreezeGate{Name: strings.TrimSpace(match[1]), Description: strings.TrimSpace(match[2])})
	}
	return gates
}

func powerShellModuleRetired(module PowerShellModuleStatus) bool {
	status := strings.ToLower(module.Status)
	return strings.Contains(status, "retired") || strings.Contains(status, "removed") || strings.Contains(status, "archived") || strings.Contains(status, "deleted")
}

func powerShellModulePresent(repo, path string) bool {
	_, err := os.Stat(filepath.Join(repo, filepath.FromSlash(path)))
	return err == nil
}

func powerShellPublicFacade(repo string, modules []PowerShellModuleStatus, fallback PowerShellFallbackRetirement) PowerShellPublicFacade {
	const facadePath = "rekit/rekit.ps1"
	inventory := PowerShellPublicFacade{
		Ready:               true,
		Summary:             "PowerShell public facade retention inventory ok",
		FacadePath:          facadePath,
		CommandSurface:      sortedStringMapKeys(powerShellValidateSet(repo)),
		GoDefaultCommands:   sortedStringMapKeys(powerShellDefaultDelegationCommands(repo)),
		NoFallbackCommands:  append([]string{}, fallback.NoFallbackCommands...),
		GoNativeAlternative: "go run ./cmd/rekit -- -Command <command>",
		Warnings:            []string{},
	}
	inventory.Present = powerShellModulePresent(repo, facadePath)
	for _, module := range modules {
		if filepath.ToSlash(module.Path) != facadePath {
			continue
		}
		combined := strings.ToLower(module.Status + " " + module.Notes)
		inventory.Retained = strings.Contains(combined, "retained") || strings.Contains(combined, "façade-stable") || strings.Contains(combined, "facade-stable")
		break
	}
	sort.Strings(inventory.NoFallbackCommands)
	strategyText := ""
	if data, err := os.ReadFile(filepath.Join(repo, filepath.FromSlash("docs/powershell-deprecation.md"))); err == nil {
		strategyText = string(data)
	}
	inventory.MigrationBoundaryDocumented = strings.Contains(strategyText, "迁移期公共入口") && strings.Contains(strategyText, "不承载业务 runtime")
	inventory.RemovalBoundaryDocumented = strings.Contains(strategyText, "删除公共 `rekit/rekit.ps1` façade") && strings.Contains(strategyText, "独立 removal batch") && strings.Contains(strategyText, "替代入口")
	if !inventory.Present {
		inventory.Warnings = append(inventory.Warnings, "PowerShell public facade missing: rekit/rekit.ps1")
	}
	if !inventory.Retained {
		inventory.Warnings = append(inventory.Warnings, "PowerShell public facade is not documented as retained")
	}
	if len(inventory.CommandSurface) == 0 {
		inventory.Warnings = append(inventory.Warnings, "PowerShell public facade command surface is empty")
	}
	if len(inventory.GoDefaultCommands) == 0 {
		inventory.Warnings = append(inventory.Warnings, "PowerShell public facade Go-default command list is empty")
	}
	if len(inventory.NoFallbackCommands) == 0 {
		inventory.Warnings = append(inventory.Warnings, "PowerShell public facade no-fallback command list is empty")
	}
	for _, command := range inventory.CommandSurface {
		if !slices.Contains(inventory.GoDefaultCommands, command) {
			inventory.Warnings = append(inventory.Warnings, fmt.Sprintf("PowerShell public facade command is not Go-default: %s", command))
		}
		if !slices.Contains(inventory.NoFallbackCommands, command) {
			inventory.Warnings = append(inventory.Warnings, fmt.Sprintf("PowerShell public facade command is not no-fallback: %s", command))
		}
	}
	for _, command := range inventory.GoDefaultCommands {
		if !slices.Contains(inventory.CommandSurface, command) {
			inventory.Warnings = append(inventory.Warnings, fmt.Sprintf("PowerShell Go-default command missing from public facade command surface: %s", command))
		}
	}
	for _, command := range inventory.NoFallbackCommands {
		if !slices.Contains(inventory.CommandSurface, command) {
			inventory.Warnings = append(inventory.Warnings, fmt.Sprintf("PowerShell no-fallback command missing from public facade command surface: %s", command))
		}
	}
	if !inventory.MigrationBoundaryDocumented {
		inventory.Warnings = append(inventory.Warnings, "PowerShell public facade migration boundary is not documented")
	}
	if !inventory.RemovalBoundaryDocumented {
		inventory.Warnings = append(inventory.Warnings, "PowerShell public facade removal boundary is not documented")
	}
	if len(inventory.Warnings) > 0 {
		inventory.Ready = false
		inventory.Summary = "PowerShell public facade retention inventory has warnings"
	}
	return inventory
}

func powerShellFallbackRetirement(repo string, owners []PowerShellCommandOwner, modules []PowerShellModuleStatus) PowerShellFallbackRetirement {
	inventory := PowerShellFallbackRetirement{
		Ready:                   true,
		Summary:                 "PowerShell fallback retirement inventory ok",
		GoDefaultCommands:       sortedStringMapKeys(powerShellDefaultDelegationCommands(repo)),
		NoFallbackCommands:      []string{},
		CandidateCommands:       []PowerShellFallbackRetirementCommand{},
		BlockedCommands:         []PowerShellFallbackRetirementCommand{},
		RemovalCandidateModules: []PowerShellFallbackRetirementModule{},
		RetiredModules:          []PowerShellFallbackRetirementModule{},
		Warnings:                []string{},
	}
	coveredDefault := map[string]bool{}
	for _, owner := range owners {
		entry := PowerShellFallbackRetirementCommand{
			Area:     owner.Area,
			Commands: append([]string{}, owner.Commands...),
			Status:   owner.Status,
			Strategy: owner.Strategy,
		}
		if owner.Blocked {
			inventory.BlockedCommands = append(inventory.BlockedCommands, entry)
			continue
		}
		if !owner.GoDefault {
			continue
		}
		for _, command := range owner.Commands {
			coveredDefault[command] = true
		}
		combined := strings.ToLower(owner.Status + " " + owner.Strategy)
		switch {
		case strings.Contains(combined, "no powershell fallback") || strings.Contains(combined, "no fallback"):
			inventory.NoFallbackCommands = appendUniqueStrings(inventory.NoFallbackCommands, owner.Commands...)
		case strings.Contains(combined, "fallback") || strings.Contains(combined, "removal-candidate") || strings.Contains(combined, "removal candidate") || strings.Contains(combined, "legacy"):
			inventory.CandidateCommands = append(inventory.CandidateCommands, entry)
		default:
			inventory.Warnings = append(inventory.Warnings, fmt.Sprintf("Go-default command row missing fallback retirement strategy: %s", owner.Area))
		}
	}
	for _, module := range modules {
		combined := strings.ToLower(module.Status + " " + module.Notes)
		present := powerShellModulePresent(repo, module.Path)
		switch {
		case powerShellModuleRetired(module) && !present:
			inventory.RetiredModules = append(inventory.RetiredModules, PowerShellFallbackRetirementModule(module))
		case strings.Contains(combined, "removal-candidate") || strings.Contains(combined, "removal candidate"):
			inventory.RemovalCandidateModules = append(inventory.RemovalCandidateModules, PowerShellFallbackRetirementModule(module))
		}
	}
	sort.Strings(inventory.GoDefaultCommands)
	sort.Strings(inventory.NoFallbackCommands)
	sortFallbackRetirementCommands(inventory.CandidateCommands)
	sortFallbackRetirementCommands(inventory.BlockedCommands)
	sort.Slice(inventory.RemovalCandidateModules, func(i, j int) bool {
		return inventory.RemovalCandidateModules[i].Path < inventory.RemovalCandidateModules[j].Path
	})
	sort.Slice(inventory.RetiredModules, func(i, j int) bool {
		return inventory.RetiredModules[i].Path < inventory.RetiredModules[j].Path
	})
	if len(inventory.GoDefaultCommands) == 0 {
		inventory.Warnings = append(inventory.Warnings, "PowerShell fallback retirement inventory has no Go-default commands")
	}
	for _, command := range inventory.GoDefaultCommands {
		if !coveredDefault[command] {
			inventory.Warnings = append(inventory.Warnings, fmt.Sprintf("Go-default command missing fallback retirement classification: %s", command))
		}
	}
	if len(inventory.NoFallbackCommands) == 0 {
		inventory.Warnings = append(inventory.Warnings, "PowerShell fallback retirement inventory has no no-fallback Go-default commands")
	}
	if len(inventory.CandidateCommands) == 0 && len(inventory.RemovalCandidateModules) == 0 && len(inventory.RetiredModules) == 0 {
		inventory.Warnings = append(inventory.Warnings, "PowerShell fallback retirement inventory has no fallback retirement candidate commands, removal-candidate modules, or retired modules")
	}
	if len(inventory.Warnings) > 0 {
		inventory.Ready = false
		inventory.Summary = "PowerShell fallback retirement inventory has warnings"
	}
	return inventory
}

func sortFallbackRetirementCommands(items []PowerShellFallbackRetirementCommand) {
	sort.Slice(items, func(i, j int) bool {
		if items[i].Area == items[j].Area {
			return strings.Join(items[i].Commands, ",") < strings.Join(items[j].Commands, ",")
		}
		return items[i].Area < items[j].Area
	})
}

func powerShellFacadeRuntime(repo string) PowerShellFacadeRuntime {
	const facadePath = "rekit/rekit.ps1"
	inventory := PowerShellFacadeRuntime{
		Ready:             true,
		Summary:           "PowerShell facade runtime dependency inventory ok",
		FacadePath:        facadePath,
		ForbiddenPatterns: []string{},
		RequiredPatterns:  []string{},
		Warnings:          []string{},
	}
	data, err := os.ReadFile(filepath.Join(repo, filepath.FromSlash(facadePath)))
	if err != nil {
		inventory.Ready = false
		inventory.Summary = "PowerShell facade runtime dependency inventory has warnings"
		inventory.Warnings = append(inventory.Warnings, err.Error())
		return inventory
	}
	text := string(data)
	forbidden := []string{
		". (Join-Path $RuntimeRoot 'lib\\",
		"Get-RekitPackInventory -RepoRoot",
		"Invoke-RekitStart -Target",
		"Sync-RekitPack -Target",
		"Promote-RekitChanges -Target",
		"Write-RekitSubagentPlan -Target",
	}
	for _, pattern := range forbidden {
		inventory.ForbiddenPatterns = append(inventory.ForbiddenPatterns, pattern)
		if strings.Contains(text, pattern) {
			inventory.Warnings = append(inventory.Warnings, fmt.Sprintf("PowerShell facade still contains retired runtime dependency pattern: %s", pattern))
		}
	}
	required := []string{
		"function Test-RekitGoDefaultDelegationCommand",
		"function Test-RekitNoPowerShellFallbackCommand",
		"Invoke-RekitGoBackend -Invocation $goInvocation",
		"if (Test-RekitNoPowerShellFallbackCommand -Name $Command)",
		"retired PowerShell fallback dispatcher",
	}
	for _, pattern := range required {
		inventory.RequiredPatterns = append(inventory.RequiredPatterns, pattern)
		if !strings.Contains(text, pattern) {
			inventory.Warnings = append(inventory.Warnings, fmt.Sprintf("PowerShell facade missing required Go/no-fallback pattern: %s", pattern))
		}
	}
	inventory.LegacyModuleImportsPresent = strings.Contains(text, ". (Join-Path $RuntimeRoot 'lib\\")
	inventory.CommandDispatcherPresent = strings.Contains(text, "Get-RekitPackInventory -RepoRoot") || strings.Contains(text, "Invoke-RekitStart -Target") || strings.Contains(text, "Sync-RekitPack -Target") || strings.Contains(text, "Promote-RekitChanges -Target") || strings.Contains(text, "Write-RekitSubagentPlan -Target")
	inventory.NoFallbackGuardPresent = strings.Contains(text, "if (Test-RekitNoPowerShellFallbackCommand -Name $Command)")
	inventory.GoDelegationPresent = strings.Contains(text, "Invoke-RekitGoBackend -Invocation $goInvocation")
	inventory.RetiredDispatcherError = strings.Contains(text, "retired PowerShell fallback dispatcher")
	if len(inventory.Warnings) > 0 {
		inventory.Ready = false
		inventory.Summary = "PowerShell facade runtime dependency inventory has warnings"
	}
	return inventory
}

func powerShellModuleRemoval(repo string, modules []PowerShellModuleStatus, facade PowerShellFacadeRuntime) PowerShellModuleRemoval {
	inventory := PowerShellModuleRemoval{
		Ready:                     true,
		Summary:                   "PowerShell module removal inventory ok",
		CandidateModules:          []PowerShellModuleRemovalItem{},
		RetiredModules:            []PowerShellModuleRemovalItem{},
		UndocumentedModules:       []string{},
		FacadeRuntimeDependencies: []string{},
		Warnings:                  []string{},
	}
	facadeRefs := map[string]bool{}
	if facade.LegacyModuleImportsPresent {
		for _, module := range powerShellRuntimeModules(repo) {
			facadeRefs[module] = true
		}
	}
	documented := map[string]bool{}
	for _, module := range modules {
		documented[filepath.ToSlash(module.Path)] = true
		combined := strings.ToLower(module.Status + " " + module.Notes)
		path := filepath.ToSlash(module.Path)
		item := PowerShellModuleRemovalItem{
			Path:               path,
			Status:             module.Status,
			Notes:              module.Notes,
			Present:            powerShellModulePresent(repo, path),
			ReferencedByFacade: facadeRefs[path],
		}
		switch {
		case powerShellModuleRetired(module):
			if item.Present {
				inventory.Warnings = append(inventory.Warnings, fmt.Sprintf("PowerShell retired module still present on disk: %s", path))
			}
			inventory.RetiredModules = append(inventory.RetiredModules, item)
		case strings.Contains(combined, "removal-candidate") || strings.Contains(combined, "removal candidate"):
			if !item.Present {
				inventory.Warnings = append(inventory.Warnings, fmt.Sprintf("PowerShell removal-candidate module missing on disk: %s", path))
			}
			if item.ReferencedByFacade {
				inventory.FacadeRuntimeDependencies = append(inventory.FacadeRuntimeDependencies, path)
				inventory.Warnings = append(inventory.Warnings, fmt.Sprintf("PowerShell removal-candidate module still referenced by facade: %s", path))
			}
			inventory.CandidateModules = append(inventory.CandidateModules, item)
		}
	}
	for _, module := range powerShellRuntimeModules(repo) {
		if !documented[module] {
			inventory.UndocumentedModules = append(inventory.UndocumentedModules, module)
		}
	}
	sort.Slice(inventory.CandidateModules, func(i, j int) bool {
		return inventory.CandidateModules[i].Path < inventory.CandidateModules[j].Path
	})
	sort.Slice(inventory.RetiredModules, func(i, j int) bool {
		return inventory.RetiredModules[i].Path < inventory.RetiredModules[j].Path
	})
	sort.Strings(inventory.UndocumentedModules)
	sort.Strings(inventory.FacadeRuntimeDependencies)
	if len(inventory.CandidateModules) == 0 && len(inventory.RetiredModules) == 0 {
		inventory.Warnings = append(inventory.Warnings, "PowerShell module removal inventory has no removal-candidate or retired modules")
	}
	if len(inventory.UndocumentedModules) > 0 {
		inventory.Warnings = append(inventory.Warnings, fmt.Sprintf("PowerShell runtime modules missing from removal inventory: %s", strings.Join(inventory.UndocumentedModules, ", ")))
	}
	if len(inventory.Warnings) > 0 {
		inventory.Ready = false
		inventory.Summary = "PowerShell module removal inventory has warnings"
	}
	return inventory
}

func powerShellModuleReferences(repo string) PowerShellModuleReferences {
	inventory := PowerShellModuleReferences{
		Ready:                   true,
		Summary:                 "PowerShell module reference inventory ok",
		ActiveTestDependencies:  []PowerShellModuleReference{},
		CompatibilityFixtures:   []PowerShellModuleReference{},
		InventoryGuards:         []PowerShellModuleReference{},
		DocumentationReferences: []PowerShellModuleReference{},
		HistoricalReferences:    []PowerShellModuleReference{},
		RemovalBlockers:         []PowerShellModuleReference{},
		UnclassifiedReferences:  []PowerShellModuleReference{},
		Warnings:                []string{},
	}
	for _, ref := range powerShellModuleReferenceScan(repo) {
		switch ref.Kind {
		case "active-test-dependency":
			inventory.ActiveTestDependencies = append(inventory.ActiveTestDependencies, ref)
		case "compatibility-fixture":
			inventory.CompatibilityFixtures = append(inventory.CompatibilityFixtures, ref)
		case "inventory-guard":
			inventory.InventoryGuards = append(inventory.InventoryGuards, ref)
		case "documentation-reference":
			inventory.DocumentationReferences = append(inventory.DocumentationReferences, ref)
		case "historical-reference":
			inventory.HistoricalReferences = append(inventory.HistoricalReferences, ref)
		case "removal-blocker":
			inventory.RemovalBlockers = append(inventory.RemovalBlockers, ref)
		default:
			ref.Kind = "unclassified"
			inventory.UnclassifiedReferences = append(inventory.UnclassifiedReferences, ref)
		}
		inventory.TotalReferences++
	}
	if len(inventory.InventoryGuards) == 0 {
		inventory.Warnings = append(inventory.Warnings, "PowerShell module reference inventory has no inventory guard markers")
	}
	if len(inventory.RemovalBlockers) > 0 {
		inventory.Warnings = append(inventory.Warnings, fmt.Sprintf("PowerShell module reference inventory has removal blockers: %d", len(inventory.RemovalBlockers)))
	}
	if len(inventory.UnclassifiedReferences) > 0 {
		inventory.Warnings = append(inventory.Warnings, fmt.Sprintf("PowerShell module reference inventory has unclassified references: %d", len(inventory.UnclassifiedReferences)))
	}
	if len(inventory.Warnings) > 0 {
		inventory.Ready = false
		inventory.Summary = "PowerShell module reference inventory has warnings"
	}
	return inventory
}

func powerShellModuleReferenceScan(repo string) []PowerShellModuleReference {
	refs := []PowerShellModuleReference{}
	_ = filepath.WalkDir(repo, func(path string, entry os.DirEntry, err error) error {
		if err != nil {
			return nil
		}
		if entry.IsDir() {
			switch entry.Name() {
			case ".git", ".claude", "memory":
				return filepath.SkipDir
			}
			return nil
		}
		if !powerShellReferenceFile(path) {
			return nil
		}
		rel, err := filepath.Rel(repo, path)
		if err != nil {
			return nil
		}
		rel = filepath.ToSlash(rel)
		data, err := os.ReadFile(path)
		if err != nil {
			return nil
		}
		for index, line := range strings.Split(strings.ReplaceAll(string(data), "\r\n", "\n"), "\n") {
			target := powerShellModuleReferenceTarget(line)
			if target == "" {
				continue
			}
			refs = append(refs, PowerShellModuleReference{
				Path:    rel,
				Line:    index + 1,
				Kind:    powerShellModuleReferenceKind(rel, line),
				Target:  target,
				Snippet: strings.TrimSpace(line),
			})
		}
		return nil
	})
	sort.Slice(refs, func(i, j int) bool {
		if refs[i].Path == refs[j].Path {
			return refs[i].Line < refs[j].Line
		}
		return refs[i].Path < refs[j].Path
	})
	return refs
}

func powerShellReferenceFile(path string) bool {
	switch strings.ToLower(filepath.Ext(path)) {
	case ".go", ".md", ".ps1", ".yml", ".yaml":
		return true
	default:
		return false
	}
}

func powerShellModuleReferenceTarget(line string) string {
	if match := regexp.MustCompile(`rekit[/\\]lib[/\\][A-Za-z0-9_.-]+\.ps1`).FindString(line); match != "" {
		return filepath.ToSlash(match)
	}
	if strings.Contains(line, "rekit/lib/*.ps1") || strings.Contains(line, "rekit\\lib\\*.ps1") {
		return "rekit/lib/*.ps1"
	}
	if match := regexp.MustCompile(`Join-Path\s+\$(RekitRoot|RuntimeRoot|isolatedRoot)\s+'lib\\([^']+\.ps1)'`).FindStringSubmatch(line); len(match) == 3 {
		if match[1] == "isolatedRoot" {
			return "isolated/lib/" + match[2]
		}
		return "rekit/lib/" + match[2]
	}
	if match := regexp.MustCompile(`lib\\([A-Za-z0-9_.-]+\.ps1)`).FindStringSubmatch(line); len(match) == 2 {
		return "rekit/lib/" + match[1]
	}
	return ""
}

func powerShellModuleReferenceKind(path, line string) string {
	switch {
	case path == "rekit/tests/continue-preflight-smoke.ps1" && strings.Contains(line, "$RekitRoot"):
		return "active-test-dependency"
	case path == "rekit/tests/facade-smoke.ps1" && strings.Contains(line, "$isolatedRoot"):
		return "compatibility-fixture"
	case strings.HasPrefix(path, "internal/rekit/releasecheck/") || strings.HasPrefix(path, "internal/rekit/manifest/") || strings.HasPrefix(path, "internal/rekit/cli/"):
		return "inventory-guard"
	case path == "CHANGELOG.md" || path == "docs/batch-plan.md" || strings.HasSuffix(path, "-migration.md") || path == "docs/reference-absorption.md" || path == "docs/agent-team-rollout-plan.md":
		return "historical-reference"
	case strings.HasSuffix(path, ".md") || path == "CLAUDE.md" || path == "README.md":
		return "documentation-reference"
	case strings.HasSuffix(path, ".ps1") || strings.HasSuffix(path, ".go"):
		return "removal-blocker"
	default:
		return "unclassified"
	}
}

func sortedStringMapKeys(values map[string]bool) []string {
	keys := make([]string, 0, len(values))
	for key := range values {
		keys = append(keys, key)
	}
	sort.Strings(keys)
	return keys
}

func appendUniqueStrings(out []string, values ...string) []string {
	seen := map[string]bool{}
	for _, value := range out {
		seen[value] = true
	}
	for _, value := range values {
		if value == "" || seen[value] {
			continue
		}
		out = append(out, value)
		seen[value] = true
	}
	return out
}

func powerShellDeprecationWarnings(repo string, strategy PowerShellDeprecation) []string {
	warnings := []string{}
	if len(strategy.CommandOwnership) == 0 {
		warnings = append(warnings, "PowerShell command ownership matrix is empty")
	}
	if len(strategy.ModuleStatus) == 0 {
		warnings = append(warnings, "PowerShell module status matrix is empty")
	}
	if len(strategy.FreezeGates) == 0 {
		warnings = append(warnings, "PowerShell freeze gates are empty")
	}
	if len(strategy.BlockedMigrations) == 0 {
		warnings = append(warnings, "PowerShell blocked migration list is empty")
	}

	validateSet := powerShellValidateSet(repo)
	documentedCommands := map[string]bool{}
	for _, row := range strategy.CommandOwnership {
		for _, token := range commandTokens(row.Area, validateSet) {
			documentedCommands[token] = true
		}
	}
	for command := range powerShellDefaultDelegationCommands(repo) {
		if !documentedCommands[command] {
			warnings = append(warnings, fmt.Sprintf("Go-default facade command missing from PowerShell deprecation matrix: %s", command))
		}
	}

	documentedModules := map[string]bool{}
	for _, module := range strategy.ModuleStatus {
		documentedModules[filepath.ToSlash(module.Path)] = true
		if !powerShellModulePresent(repo, module.Path) && !powerShellModuleRetired(module) {
			warnings = append(warnings, fmt.Sprintf("documented PowerShell module missing on disk: %s", module.Path))
		}
	}
	for _, module := range powerShellRuntimeModules(repo) {
		if !documentedModules[module] {
			warnings = append(warnings, fmt.Sprintf("PowerShell runtime module missing from deprecation matrix: %s", module))
		}
	}
	return warnings
}

func powerShellRuntimeModules(repo string) []string {
	modules := []string{"rekit/rekit.ps1"}
	matches, err := filepath.Glob(filepath.Join(repo, "rekit", "lib", "*.ps1"))
	if err != nil {
		return modules
	}
	for _, match := range matches {
		rel, err := filepath.Rel(repo, match)
		if err != nil {
			continue
		}
		modules = append(modules, filepath.ToSlash(rel))
	}
	return modules
}

func powerShellValidateSet(repo string) map[string]bool {
	data, err := os.ReadFile(filepath.Join(repo, "rekit", "rekit.ps1"))
	if err != nil {
		return map[string]bool{}
	}
	match := regexp.MustCompile(`(?s)\[ValidateSet\((.*?)\)\]`).FindStringSubmatch(string(data))
	if len(match) != 2 {
		return map[string]bool{}
	}
	return powerShellSingleQuotedItems(match[1])
}

func powerShellDefaultDelegationCommands(repo string) map[string]bool {
	data, err := os.ReadFile(filepath.Join(repo, "rekit", "rekit.ps1"))
	if err != nil {
		return map[string]bool{}
	}
	match := regexp.MustCompile(`(?s)function\s+Test-RekitGoDefaultDelegationCommand\s*\{.*?@\((.*?)\)`).FindStringSubmatch(string(data))
	if len(match) != 2 {
		return map[string]bool{}
	}
	return powerShellSingleQuotedItems(match[1])
}

func powerShellSingleQuotedItems(text string) map[string]bool {
	items := map[string]bool{}
	for _, match := range regexp.MustCompile(`'([^']+)'`).FindAllStringSubmatch(text, -1) {
		items[match[1]] = true
	}
	return items
}

func commandTokens(text string, validateSet map[string]bool) []string {
	out := []string{}
	seen := map[string]bool{}
	for _, token := range regexp.MustCompile(`[a-z][a-z0-9-]*`).FindAllString(strings.ToLower(text), -1) {
		if validateSet[token] && !seen[token] {
			out = append(out, token)
			seen[token] = true
		}
	}
	return out
}

func markdownTableRowsInSection(text, heading string) [][]string {
	lines := markdownSectionLines(text, heading)
	rows := [][]string{}
	for _, line := range lines {
		trimmed := strings.TrimSpace(line)
		if !strings.HasPrefix(trimmed, "|") || !strings.HasSuffix(trimmed, "|") || strings.Contains(trimmed, "---") {
			continue
		}
		cells := strings.Split(strings.Trim(trimmed, "|"), "|")
		for i := range cells {
			cells[i] = strings.TrimSpace(cells[i])
		}
		rows = append(rows, cells)
	}
	return rows
}

func markdownSectionLines(text, heading string) []string {
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
	return section
}

func stripInlineCode(text string) string {
	return strings.TrimSpace(strings.ReplaceAll(text, "`", ""))
}
