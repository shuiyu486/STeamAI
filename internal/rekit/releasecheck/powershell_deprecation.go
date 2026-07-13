package releasecheck

import (
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strings"
)

type PowerShellDeprecation struct {
	StrategyDocument  string                   `json:"strategyDocument"`
	Ready             bool                     `json:"ready"`
	Summary           string                   `json:"summary"`
	CommandOwnership  []PowerShellCommandOwner `json:"commandOwnership"`
	ModuleStatus      []PowerShellModuleStatus `json:"moduleStatus"`
	FreezeGates       []PowerShellFreezeGate   `json:"freezeGates"`
	BlockedMigrations []string                 `json:"blockedMigrations"`
	Warnings          []string                 `json:"warnings"`
}

type PowerShellCommandOwner struct {
	Area      string `json:"area"`
	Owner     string `json:"owner"`
	Status    string `json:"status"`
	Strategy  string `json:"strategy"`
	GoDefault bool   `json:"goDefault"`
	Blocked   bool   `json:"blocked"`
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
	strategy.Warnings = append(strategy.Warnings, powerShellDeprecationWarnings(repo, strategy)...)
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
		owner := PowerShellCommandOwner{
			Area:     area,
			Owner:    stripInlineCode(cells[1]),
			Status:   stripInlineCode(cells[2]),
			Strategy: stripInlineCode(cells[3]),
		}
		combined := strings.ToLower(owner.Owner + " " + owner.Status + " " + owner.Strategy + " " + owner.Area)
		owner.Blocked = strings.Contains(combined, "blocked") || strings.Contains(combined, "禁止") || strings.Contains(combined, "不得")
		for _, token := range commandTokens(area, validateSet) {
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
