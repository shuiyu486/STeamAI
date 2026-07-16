package releasecheck

import (
	"fmt"
	"os"
	"path/filepath"
	"regexp"
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
	ModuleRemoval      PowerShellModuleRemoval      `json:"moduleRemoval"`
	Warnings           []string                     `json:"warnings"`
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

type PowerShellModuleRemoval struct {
	Ready                     bool                          `json:"ready"`
	Summary                   string                        `json:"summary"`
	CandidateModules          []PowerShellModuleRemovalItem `json:"candidateModules"`
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

type PowerShellFallbackRetirement struct {
	Ready                   bool                                  `json:"ready"`
	Summary                 string                                `json:"summary"`
	GoDefaultCommands       []string                              `json:"goDefaultCommands"`
	NoFallbackCommands      []string                              `json:"noFallbackCommands"`
	CandidateCommands       []PowerShellFallbackRetirementCommand `json:"candidateCommands"`
	BlockedCommands         []PowerShellFallbackRetirementCommand `json:"blockedCommands"`
	RemovalCandidateModules []PowerShellFallbackRetirementModule  `json:"removalCandidateModules"`
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
	strategy.ModuleRemoval = powerShellModuleRemoval(repo, strategy.ModuleStatus, strategy.FacadeRuntime)
	strategy.Warnings = append(strategy.Warnings, powerShellDeprecationWarnings(repo, strategy)...)
	strategy.Warnings = append(strategy.Warnings, strategy.FallbackRetirement.Warnings...)
	strategy.Warnings = append(strategy.Warnings, strategy.FacadeRuntime.Warnings...)
	strategy.Warnings = append(strategy.Warnings, strategy.ModuleRemoval.Warnings...)
	if len(strategy.Warnings) > 0 {
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

func powerShellFallbackRetirement(repo string, owners []PowerShellCommandOwner, modules []PowerShellModuleStatus) PowerShellFallbackRetirement {
	inventory := PowerShellFallbackRetirement{
		Ready:                   true,
		Summary:                 "PowerShell fallback retirement inventory ok",
		GoDefaultCommands:       sortedStringMapKeys(powerShellDefaultDelegationCommands(repo)),
		NoFallbackCommands:      []string{},
		CandidateCommands:       []PowerShellFallbackRetirementCommand{},
		BlockedCommands:         []PowerShellFallbackRetirementCommand{},
		RemovalCandidateModules: []PowerShellFallbackRetirementModule{},
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
		if strings.Contains(combined, "removal-candidate") || strings.Contains(combined, "removal candidate") {
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
	if len(inventory.CandidateCommands) == 0 && len(inventory.RemovalCandidateModules) == 0 {
		inventory.Warnings = append(inventory.Warnings, "PowerShell fallback retirement inventory has no fallback retirement candidate commands or removal-candidate modules")
	}
	if len(inventory.RemovalCandidateModules) == 0 {
		inventory.Warnings = append(inventory.Warnings, "PowerShell fallback retirement inventory has no removal-candidate modules")
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
		if !strings.Contains(combined, "removal-candidate") && !strings.Contains(combined, "removal candidate") {
			continue
		}
		path := filepath.ToSlash(module.Path)
		_, err := os.Stat(filepath.Join(repo, filepath.FromSlash(path)))
		item := PowerShellModuleRemovalItem{
			Path:               path,
			Status:             module.Status,
			Notes:              module.Notes,
			Present:            err == nil,
			ReferencedByFacade: facadeRefs[path],
		}
		if !item.Present {
			inventory.Warnings = append(inventory.Warnings, fmt.Sprintf("PowerShell removal-candidate module missing on disk: %s", path))
		}
		if item.ReferencedByFacade {
			inventory.FacadeRuntimeDependencies = append(inventory.FacadeRuntimeDependencies, path)
			inventory.Warnings = append(inventory.Warnings, fmt.Sprintf("PowerShell removal-candidate module still referenced by facade: %s", path))
		}
		inventory.CandidateModules = append(inventory.CandidateModules, item)
	}
	for _, module := range powerShellRuntimeModules(repo) {
		if !documented[module] {
			inventory.UndocumentedModules = append(inventory.UndocumentedModules, module)
		}
	}
	sort.Slice(inventory.CandidateModules, func(i, j int) bool {
		return inventory.CandidateModules[i].Path < inventory.CandidateModules[j].Path
	})
	sort.Strings(inventory.UndocumentedModules)
	sort.Strings(inventory.FacadeRuntimeDependencies)
	if len(inventory.CandidateModules) == 0 {
		inventory.Warnings = append(inventory.Warnings, "PowerShell module removal inventory has no removal-candidate modules")
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
		if _, err := os.Stat(filepath.Join(repo, filepath.FromSlash(module.Path))); err != nil {
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
