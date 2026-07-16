package releasecheck

import (
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"slices"
	"sort"
	"strings"

	"github.com/shuiyu486/re-context-kits/internal/rekit/commands"
)

type GoNativePublicSurface struct {
	Ready                               bool              `json:"ready"`
	Summary                             string            `json:"summary"`
	Entrypoint                          string            `json:"entrypoint"`
	EntrypointPresent                   bool              `json:"entrypointPresent"`
	CommandCatalogPath                  string            `json:"commandCatalogPath"`
	CommandCatalogPresent               bool              `json:"commandCatalogPresent"`
	DefaultCommand                      string            `json:"defaultCommand"`
	Commands                            []string          `json:"commands"`
	HandlerCommands                     []string          `json:"handlerCommands"`
	SymbolCommands                      map[string]string `json:"symbolCommands"`
	AlternativePattern                  string            `json:"alternativePattern"`
	UnsupportedCommandDiagnostic        string            `json:"unsupportedCommandDiagnostic"`
	UnsupportedCommandDiagnosticPresent bool              `json:"unsupportedCommandDiagnosticPresent"`
	Warnings                            []string          `json:"warnings"`
}

func goNativePublicSurface(repo string) GoNativePublicSurface {
	const catalogPath = "internal/rekit/commands/commands.go"
	inventory := GoNativePublicSurface{
		Ready:                        true,
		Summary:                      "Go-native public command surface inventory ok",
		Entrypoint:                   commands.GoNativeEntrypoint,
		CommandCatalogPath:           catalogPath,
		DefaultCommand:               commands.DefaultCommand,
		Commands:                     commands.Public(),
		HandlerCommands:              goNativePublicHandlerCommands(repo),
		SymbolCommands:               commands.SymbolValues(),
		AlternativePattern:           commands.GoNativeAlternativePattern,
		UnsupportedCommandDiagnostic: commands.UnsupportedError("unsupported").Error(),
		Warnings:                     []string{},
	}
	inventory.EntrypointPresent = filePresent(repo, filepath.ToSlash(filepath.Join(commands.GoNativeEntrypoint, "main.go")))
	inventory.CommandCatalogPresent = filePresent(repo, catalogPath)
	if data, err := os.ReadFile(filepath.Join(repo, filepath.FromSlash("internal/rekit/cli/cli.go"))); err == nil {
		inventory.UnsupportedCommandDiagnosticPresent = strings.Contains(string(data), "commands.UnsupportedError(opt.Command)")
	}
	if !inventory.EntrypointPresent {
		inventory.Warnings = append(inventory.Warnings, fmt.Sprintf("Go-native public entrypoint missing: %s/main.go", commands.GoNativeEntrypoint))
	}
	if !inventory.CommandCatalogPresent {
		inventory.Warnings = append(inventory.Warnings, fmt.Sprintf("Go-native public command catalog missing: %s", catalogPath))
	}
	if len(inventory.Commands) == 0 {
		inventory.Warnings = append(inventory.Warnings, "Go-native public command catalog is empty")
	}
	if !slices.IsSorted(inventory.Commands) {
		inventory.Warnings = append(inventory.Warnings, "Go-native public command catalog is not sorted")
	}
	if !slices.Contains(inventory.Commands, inventory.DefaultCommand) {
		inventory.Warnings = append(inventory.Warnings, fmt.Sprintf("Go-native default command missing from public command catalog: %s", inventory.DefaultCommand))
	}
	if inventory.AlternativePattern != "go run ./cmd/rekit -- -Command <command>" {
		inventory.Warnings = append(inventory.Warnings, "Go-native alternative pattern drifted from documented public facade fallback")
	}
	if !inventory.UnsupportedCommandDiagnosticPresent {
		inventory.Warnings = append(inventory.Warnings, "Go CLI unsupported command diagnostic is not sourced from public command catalog")
	}
	for _, command := range commands.MissingPublicHandlers(inventory.HandlerCommands) {
		inventory.Warnings = append(inventory.Warnings, fmt.Sprintf("Go CLI dispatcher missing public command handler: %s", command))
	}
	for _, command := range commands.UnknownPublicHandlers(inventory.HandlerCommands) {
		inventory.Warnings = append(inventory.Warnings, fmt.Sprintf("Go CLI dispatcher exposes handler outside public command catalog: %s", command))
	}
	symbolCommandValues := []string{}
	for symbol, command := range inventory.SymbolCommands {
		if strings.TrimSpace(symbol) == "" {
			inventory.Warnings = append(inventory.Warnings, "Go-native public command symbol catalog contains an empty symbol")
		}
		if strings.TrimSpace(command) == "" {
			inventory.Warnings = append(inventory.Warnings, fmt.Sprintf("Go-native public command symbol %q has an empty command", symbol))
			continue
		}
		symbolCommandValues = append(symbolCommandValues, command)
	}
	for _, command := range commands.MissingPublicHandlers(symbolCommandValues) {
		inventory.Warnings = append(inventory.Warnings, fmt.Sprintf("Go-native public command symbol missing from symbol catalog: %s", command))
	}
	for _, command := range commands.UnknownPublicHandlers(symbolCommandValues) {
		inventory.Warnings = append(inventory.Warnings, fmt.Sprintf("Go-native public command symbol outside public command catalog: %s", command))
	}
	seen := map[string]bool{}
	for _, command := range inventory.Commands {
		if strings.TrimSpace(command) == "" {
			inventory.Warnings = append(inventory.Warnings, "Go-native public command catalog contains an empty command")
			continue
		}
		if seen[command] {
			inventory.Warnings = append(inventory.Warnings, fmt.Sprintf("Go-native public command catalog contains duplicate command: %s", command))
		}
		seen[command] = true
	}
	if len(inventory.Warnings) > 0 {
		inventory.Ready = false
		inventory.Summary = "Go-native public command surface inventory has warnings"
	}
	return inventory
}

func goNativePublicHandlerCommands(repo string) []string {
	data, err := os.ReadFile(filepath.Join(repo, filepath.FromSlash("internal/rekit/cli/cli.go")))
	if err != nil {
		return nil
	}
	symbols := commands.SymbolValues()
	seen := map[string]bool{}
	re := regexp.MustCompile(`case\s+([^:]+):`)
	for _, match := range re.FindAllStringSubmatch(string(data), -1) {
		for part := range strings.SplitSeq(match[1], ",") {
			part = strings.TrimSpace(part)
			if !strings.HasPrefix(part, "commands.") {
				continue
			}
			symbol := strings.TrimPrefix(part, "commands.")
			if command, ok := symbols[symbol]; ok {
				seen[command] = true
			} else {
				seen[symbol] = true
			}
		}
	}
	out := make([]string, 0, len(seen))
	for command := range seen {
		out = append(out, command)
	}
	sort.Strings(out)
	return out
}

func goNativePublicSurfaceCrossWarnings(goSurface GoNativePublicSurface, facade PowerShellPublicFacade) []string {
	warnings := []string{}
	for _, command := range facade.CommandSurface {
		if !slices.Contains(goSurface.Commands, command) {
			warnings = append(warnings, fmt.Sprintf("PowerShell public facade command missing from Go-native public command catalog: %s", command))
		}
	}
	for _, command := range goSurface.Commands {
		if !slices.Contains(facade.CommandSurface, command) {
			warnings = append(warnings, fmt.Sprintf("Go-native public command missing from PowerShell public facade command surface: %s", command))
		}
	}
	return warnings
}

func filePresent(repo, rel string) bool {
	_, err := os.Stat(filepath.Join(repo, filepath.FromSlash(rel)))
	return err == nil
}
