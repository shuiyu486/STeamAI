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
	Ready                               bool                             `json:"ready"`
	Summary                             string                           `json:"summary"`
	Entrypoint                          string                           `json:"entrypoint"`
	EntrypointPresent                   bool                             `json:"entrypointPresent"`
	CommandCatalogPath                  string                           `json:"commandCatalogPath"`
	CommandCatalogPresent               bool                             `json:"commandCatalogPresent"`
	DefaultCommand                      string                           `json:"defaultCommand"`
	Commands                            []string                         `json:"commands"`
	HandlerCommands                     []string                         `json:"handlerCommands"`
	SymbolCommands                      map[string]string                `json:"symbolCommands"`
	CommandProfiles                     []commands.PublicProfile         `json:"commandProfiles"`
	CommandProfileSummary               commands.PublicProfileSummary    `json:"commandProfileSummary"`
	CommandProfileGroups                commands.PublicProfileGroups     `json:"commandProfileGroups"`
	CommandProfileBoundaries            []commands.PublicProfileBoundary `json:"commandProfileBoundaries"`
	CommandProfilePolicies              []commands.PublicProfilePolicy   `json:"commandProfilePolicies"`
	MutationBoundaries                  []string                         `json:"mutationBoundaries"`
	AlternativePattern                  string                           `json:"alternativePattern"`
	UnsupportedCommandDiagnostic        string                           `json:"unsupportedCommandDiagnostic"`
	UnsupportedCommandDiagnosticPresent bool                             `json:"unsupportedCommandDiagnosticPresent"`
	Warnings                            []string                         `json:"warnings"`
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
		CommandProfileSummary:        commands.PublicProfileSummaryBaseline(),
		CommandProfileGroups:         commands.PublicProfileGroupsBaseline(),
		CommandProfileBoundaries:     commands.PublicProfileBoundariesBaseline(),
		CommandProfilePolicies:       commands.PublicProfilePoliciesBaseline(),
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
	computedSummary := commands.PublicProfileSummaryFor(inventory.CommandProfiles)
	if inventory.CommandProfileSummary.Total != computedSummary.Total || inventory.CommandProfileSummary.ReadOnly != computedSummary.ReadOnly || inventory.CommandProfileSummary.Mutating != computedSummary.Mutating || inventory.CommandProfileSummary.WritesCase != computedSummary.WritesCase || inventory.CommandProfileSummary.WritesKit != computedSummary.WritesKit || inventory.CommandProfileSummary.ReviewFirst != computedSummary.ReviewFirst || inventory.CommandProfileSummary.ApplyRequired != computedSummary.ApplyRequired || inventory.CommandProfileSummary.HeavyTool != computedSummary.HeavyTool || inventory.CommandProfileSummary.AuthorityConfirmed != computedSummary.AuthorityConfirmed {
		inventory.Warnings = append(inventory.Warnings, "Go-native public command profile summary does not match profile catalog")
	}
	for boundary, count := range computedSummary.Boundaries {
		if inventory.CommandProfileSummary.Boundaries[boundary] != count {
			inventory.Warnings = append(inventory.Warnings, fmt.Sprintf("Go-native public command profile summary boundary count mismatch for %s", boundary))
		}
	}
	if inventory.CommandProfileSummary.ReadOnly == 0 || inventory.CommandProfileSummary.Mutating == 0 || inventory.CommandProfileSummary.WritesCase == 0 || inventory.CommandProfileSummary.ReviewFirst == 0 || inventory.CommandProfileSummary.ApplyRequired == 0 {
		inventory.Warnings = append(inventory.Warnings, "Go-native public command profile summary omitted required public command boundary counts")
	}
	if inventory.CommandProfileSummary.HeavyTool != 0 {
		inventory.Warnings = append(inventory.Warnings, "Go-native public command profile summary unexpectedly includes heavy-tool executors")
	}
	if inventory.CommandProfileSummary.AuthorityConfirmed != 0 {
		inventory.Warnings = append(inventory.Warnings, "Go-native public command profile summary unexpectedly includes authority/confirmed writers")
	}
	computedGroups := commands.PublicProfileGroupsFor(inventory.CommandProfiles)
	if !slices.Equal(inventory.CommandProfileGroups.ReadOnly, computedGroups.ReadOnly) || !slices.Equal(inventory.CommandProfileGroups.Mutating, computedGroups.Mutating) || !slices.Equal(inventory.CommandProfileGroups.WritesCase, computedGroups.WritesCase) || !slices.Equal(inventory.CommandProfileGroups.WritesKit, computedGroups.WritesKit) || !slices.Equal(inventory.CommandProfileGroups.ReviewFirst, computedGroups.ReviewFirst) || !slices.Equal(inventory.CommandProfileGroups.ApplyRequired, computedGroups.ApplyRequired) || !slices.Equal(inventory.CommandProfileGroups.HeavyTool, computedGroups.HeavyTool) || !slices.Equal(inventory.CommandProfileGroups.AuthorityConfirmed, computedGroups.AuthorityConfirmed) {
		inventory.Warnings = append(inventory.Warnings, "Go-native public command profile groups do not match profile catalog")
	}
	for boundary, groupCommands := range computedGroups.ByBoundary {
		if !slices.Equal(inventory.CommandProfileGroups.ByBoundary[boundary], groupCommands) {
			inventory.Warnings = append(inventory.Warnings, fmt.Sprintf("Go-native public command profile group mismatch for boundary %s", boundary))
		}
	}
	if len(inventory.CommandProfileGroups.ReadOnly) != inventory.CommandProfileSummary.ReadOnly || len(inventory.CommandProfileGroups.Mutating) != inventory.CommandProfileSummary.Mutating || len(inventory.CommandProfileGroups.WritesCase) != inventory.CommandProfileSummary.WritesCase || len(inventory.CommandProfileGroups.WritesKit) != inventory.CommandProfileSummary.WritesKit || len(inventory.CommandProfileGroups.ReviewFirst) != inventory.CommandProfileSummary.ReviewFirst || len(inventory.CommandProfileGroups.ApplyRequired) != inventory.CommandProfileSummary.ApplyRequired || len(inventory.CommandProfileGroups.HeavyTool) != inventory.CommandProfileSummary.HeavyTool || len(inventory.CommandProfileGroups.AuthorityConfirmed) != inventory.CommandProfileSummary.AuthorityConfirmed {
		inventory.Warnings = append(inventory.Warnings, "Go-native public command profile group counts do not match summary")
	}
	boundaryRows := map[string]commands.PublicProfileBoundary{}
	for _, row := range inventory.CommandProfileBoundaries {
		if !commands.IsKnownMutationBoundary(row.Boundary) {
			inventory.Warnings = append(inventory.Warnings, fmt.Sprintf("Go-native public command profile boundary row has unknown boundary: %s", row.Boundary))
		}
		if _, ok := boundaryRows[row.Boundary]; ok {
			inventory.Warnings = append(inventory.Warnings, fmt.Sprintf("Go-native public command profile boundary row duplicated boundary: %s", row.Boundary))
		}
		boundaryRows[row.Boundary] = row
		if row.Count != len(row.Commands) {
			inventory.Warnings = append(inventory.Warnings, fmt.Sprintf("Go-native public command profile boundary row count mismatch for %s", row.Boundary))
		}
		if !slices.IsSorted(row.Commands) {
			inventory.Warnings = append(inventory.Warnings, fmt.Sprintf("Go-native public command profile boundary row commands are not sorted for %s", row.Boundary))
		}
		if inventory.CommandProfileSummary.Boundaries[row.Boundary] != row.Count {
			inventory.Warnings = append(inventory.Warnings, fmt.Sprintf("Go-native public command profile boundary row does not match summary for %s", row.Boundary))
		}
		if !slices.Equal(inventory.CommandProfileGroups.ByBoundary[row.Boundary], row.Commands) {
			inventory.Warnings = append(inventory.Warnings, fmt.Sprintf("Go-native public command profile boundary row does not match command groups for %s", row.Boundary))
		}
	}
	for _, boundary := range inventory.MutationBoundaries {
		if _, ok := boundaryRows[boundary]; !ok {
			inventory.Warnings = append(inventory.Warnings, fmt.Sprintf("Go-native public command profile boundary rows missing boundary: %s", boundary))
		}
	}
	if len(inventory.CommandProfileBoundaries) != len(inventory.MutationBoundaries) {
		inventory.Warnings = append(inventory.Warnings, "Go-native public command profile boundary rows do not cover mutation boundaries")
	}
	computedPolicies := commands.PublicProfilePoliciesFor(inventory.CommandProfiles)
	if len(inventory.CommandProfilePolicies) != len(computedPolicies) {
		inventory.Warnings = append(inventory.Warnings, "Go-native public command profile policy rows do not cover required policies")
	}
	for i, policy := range computedPolicies {
		if i >= len(inventory.CommandProfilePolicies) {
			continue
		}
		row := inventory.CommandProfilePolicies[i]
		if row.Policy != policy.Policy || row.Summary != policy.Summary || row.Ready != policy.Ready || row.ViolationCount != policy.ViolationCount || !slices.Equal(row.Commands, policy.Commands) {
			inventory.Warnings = append(inventory.Warnings, fmt.Sprintf("Go-native public command profile policy row drifted: %s", policy.Policy))
		}
	}
	if commands.PublicProfilePolicyViolationCount(inventory.CommandProfilePolicies) != 0 {
		inventory.Warnings = append(inventory.Warnings, "Go-native public command profile policy rows contain violations")
	}
	for _, policy := range inventory.CommandProfilePolicies {
		if !policy.Ready || policy.ViolationCount != 0 || len(policy.Commands) != 0 {
			inventory.Warnings = append(inventory.Warnings, fmt.Sprintf("Go-native public command profile policy is not ready: %s", policy.Policy))
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
