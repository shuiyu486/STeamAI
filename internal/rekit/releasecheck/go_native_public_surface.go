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
	Ready                               bool                     `json:"ready"`
	Summary                             string                   `json:"summary"`
	Entrypoint                          string                   `json:"entrypoint"`
	EntrypointPresent                   bool                     `json:"entrypointPresent"`
	CommandCatalogPath                  string                   `json:"commandCatalogPath"`
	CommandCatalogPresent               bool                     `json:"commandCatalogPresent"`
	DefaultCommand                      string                   `json:"defaultCommand"`
	Commands                            []string                 `json:"commands"`
	HandlerCommands                     []string                 `json:"handlerCommands"`
	SymbolCommands                      map[string]string        `json:"symbolCommands"`
	CommandProfiles                     []commands.PublicProfile `json:"commandProfiles"`
	MutationBoundaries                  []string                 `json:"mutationBoundaries"`
	AlternativePattern                  string                   `json:"alternativePattern"`
	UnsupportedCommandDiagnostic        string                   `json:"unsupportedCommandDiagnostic"`
	UnsupportedCommandDiagnosticPresent bool                     `json:"unsupportedCommandDiagnosticPresent"`
	Warnings                            []string                 `json:"warnings"`
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
		CommandProfiles:              commands.PublicProfiles(),
		MutationBoundaries:           commands.KnownMutationBoundaries(),
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
	profileCommands := commands.PublicProfileCommands(inventory.CommandProfiles)
	for _, command := range commands.MissingPublicHandlers(profileCommands) {
		inventory.Warnings = append(inventory.Warnings, fmt.Sprintf("Go-native public command profile missing from profile catalog: %s", command))
	}
	for _, command := range commands.UnknownPublicHandlers(profileCommands) {
		inventory.Warnings = append(inventory.Warnings, fmt.Sprintf("Go-native public command profile outside public command catalog: %s", command))
	}
	profileSeen := map[string]bool{}
	for _, profile := range inventory.CommandProfiles {
		if strings.TrimSpace(profile.Command) == "" {
			inventory.Warnings = append(inventory.Warnings, "Go-native public command profile contains an empty command")
			continue
		}
		if profileSeen[profile.Command] {
			inventory.Warnings = append(inventory.Warnings, fmt.Sprintf("Go-native public command profile contains duplicate command: %s", profile.Command))
		}
		profileSeen[profile.Command] = true
		if !commands.IsKnownMutationBoundary(profile.MutationBoundary) {
			inventory.Warnings = append(inventory.Warnings, fmt.Sprintf("Go-native public command profile has unknown mutation boundary for %s: %s", profile.Command, profile.MutationBoundary))
		}
		if profile.HeavyTool {
			inventory.Warnings = append(inventory.Warnings, fmt.Sprintf("Go-native public command profile incorrectly marks public command as heavy-tool executor: %s", profile.Command))
		}
		if profile.AuthorityConfirmed {
			inventory.Warnings = append(inventory.Warnings, fmt.Sprintf("Go-native public command profile incorrectly marks public command as authority/confirmed writer: %s", profile.Command))
		}
		if profile.WritesKit && !profile.ReviewFirst {
			inventory.Warnings = append(inventory.Warnings, fmt.Sprintf("Go-native public command profile writes kit without review-first boundary: %s", profile.Command))
		}
		if profile.ReviewFirst && !profile.ApplyRequired {
			inventory.Warnings = append(inventory.Warnings, fmt.Sprintf("Go-native public command profile review-first command missing apply-required boundary: %s", profile.Command))
		}
	}
	for _, boundary := range inventory.MutationBoundaries {
		if !commands.IsKnownMutationBoundary(boundary) {
			inventory.Warnings = append(inventory.Warnings, fmt.Sprintf("Go-native public command surface exposes unknown mutation boundary: %s", boundary))
		}
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
