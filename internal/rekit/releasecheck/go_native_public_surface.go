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
	Ready                               bool                                `json:"ready"`
	Summary                             string                              `json:"summary"`
	Entrypoint                          string                              `json:"entrypoint"`
	EntrypointPresent                   bool                                `json:"entrypointPresent"`
	CommandCatalogPath                  string                              `json:"commandCatalogPath"`
	CommandCatalogPresent               bool                                `json:"commandCatalogPresent"`
	DefaultCommand                      string                              `json:"defaultCommand"`
	Commands                            []string                            `json:"commands"`
	HandlerCommands                     []string                            `json:"handlerCommands"`
	SymbolCommands                      map[string]string                   `json:"symbolCommands"`
	CommandProfiles                     []commands.PublicProfile            `json:"commandProfiles"`
	CommandProfileSummary               commands.PublicProfileSummary       `json:"commandProfileSummary"`
	CommandProfileGroups                commands.PublicProfileGroups        `json:"commandProfileGroups"`
	CommandProfileBoundaries            []commands.PublicProfileBoundary    `json:"commandProfileBoundaries"`
	CommandProfilePolicies              []commands.PublicProfilePolicy      `json:"commandProfilePolicies"`
	FacadeRemovalReady                  bool                                `json:"facadeRemovalReady"`
	FacadeRemovalPrerequisites          []GoNativePublicSurfacePrerequisite `json:"facadeRemovalPrerequisites"`
	MutationBoundaries                  []string                            `json:"mutationBoundaries"`
	AlternativePattern                  string                              `json:"alternativePattern"`
	UnsupportedCommandDiagnostic        string                              `json:"unsupportedCommandDiagnostic"`
	UnsupportedCommandDiagnosticPresent bool                                `json:"unsupportedCommandDiagnosticPresent"`
	Warnings                            []string                            `json:"warnings"`
}

type GoNativePublicSurfacePrerequisite struct {
	Name    string `json:"name"`
	Ready   bool   `json:"ready"`
	Summary string `json:"summary"`
}

type GoNativePublicSurfaceCounts struct {
	Commands                int
	HandlerCommands         int
	SymbolCommands          int
	CommandProfiles         int
	MutationBoundaries      int
	BoundaryRows            int
	PolicyRows              int
	PolicyViolations        int
	FacadePrerequisites     int
	ReadOnly                int
	Mutating                int
	WritesCase              int
	WritesKit               int
	ReviewFirst             int
	ApplyRequired           int
	HeavyTool               int
	AuthorityConfirmed      int
	Groups                  GoNativePublicSurfaceGroupCounts
	Policies                GoNativePublicSurfacePolicyCounts
	CaseLocalApplyCommands  int
	CaseLocalReviewCommands int
	KitReviewFirstCommands  int
	ReadOnlyCommands        int
}

type GoNativePublicSurfaceGroupCounts struct {
	ReadOnly                 int
	Mutating                 int
	WritesCase               int
	WritesKit                int
	ReviewFirst              int
	ApplyRequired            int
	HeavyTool                int
	AuthorityConfirmed       int
	CaseLocalAppend          int
	CaseLocalApply           int
	CaseLocalReadOrBootstrap int
	CaseLocalReviewArtifact  int
	CaseLocalReviewFirst     int
	KitReviewFirst           int
	BoundaryReadOnly         int
}

type GoNativePublicSurfacePolicyCounts struct {
	Rows              int
	Violations        int
	ViolationCommands int
}

type GoNativePublicSurfacePolicyRowCounts struct {
	Commands int
}

func GoNativePublicSurfaceCountsFor(surface GoNativePublicSurface) GoNativePublicSurfaceCounts {
	groupCounts := GoNativePublicSurfaceGroupCountsFor(surface.CommandProfileGroups)
	policyCounts := GoNativePublicSurfacePolicyCountsFor(surface.CommandProfilePolicies)
	return GoNativePublicSurfaceCounts{
		Commands:                len(surface.Commands),
		HandlerCommands:         len(surface.HandlerCommands),
		SymbolCommands:          len(surface.SymbolCommands),
		CommandProfiles:         len(surface.CommandProfiles),
		MutationBoundaries:      len(surface.MutationBoundaries),
		BoundaryRows:            len(surface.CommandProfileBoundaries),
		PolicyRows:              policyCounts.Rows,
		PolicyViolations:        policyCounts.Violations,
		FacadePrerequisites:     len(surface.FacadeRemovalPrerequisites),
		ReadOnly:                surface.CommandProfileSummary.ReadOnly,
		Mutating:                surface.CommandProfileSummary.Mutating,
		WritesCase:              surface.CommandProfileSummary.WritesCase,
		WritesKit:               surface.CommandProfileSummary.WritesKit,
		ReviewFirst:             surface.CommandProfileSummary.ReviewFirst,
		ApplyRequired:           surface.CommandProfileSummary.ApplyRequired,
		HeavyTool:               surface.CommandProfileSummary.HeavyTool,
		AuthorityConfirmed:      surface.CommandProfileSummary.AuthorityConfirmed,
		Groups:                  groupCounts,
		Policies:                policyCounts,
		CaseLocalApplyCommands:  groupCounts.CaseLocalApply,
		CaseLocalReviewCommands: groupCounts.CaseLocalReviewFirst,
		KitReviewFirstCommands:  groupCounts.KitReviewFirst,
		ReadOnlyCommands:        groupCounts.ReadOnly,
	}
}

func GoNativePublicSurfaceGroupCountsFor(groups commands.PublicProfileGroups) GoNativePublicSurfaceGroupCounts {
	return GoNativePublicSurfaceGroupCounts{
		ReadOnly:                 len(groups.ReadOnly),
		Mutating:                 len(groups.Mutating),
		WritesCase:               len(groups.WritesCase),
		WritesKit:                len(groups.WritesKit),
		ReviewFirst:              len(groups.ReviewFirst),
		ApplyRequired:            len(groups.ApplyRequired),
		HeavyTool:                len(groups.HeavyTool),
		AuthorityConfirmed:       len(groups.AuthorityConfirmed),
		CaseLocalAppend:          len(groups.ByBoundary[commands.BoundaryCaseLocalAppend]),
		CaseLocalApply:           len(groups.ByBoundary[commands.BoundaryCaseLocalApply]),
		CaseLocalReadOrBootstrap: len(groups.ByBoundary[commands.BoundaryCaseLocalReadOrBootstrap]),
		CaseLocalReviewArtifact:  len(groups.ByBoundary[commands.BoundaryCaseLocalReviewArtifact]),
		CaseLocalReviewFirst:     len(groups.ByBoundary[commands.BoundaryCaseLocalReviewFirst]),
		KitReviewFirst:           len(groups.ByBoundary[commands.BoundaryKitReviewFirst]),
		BoundaryReadOnly:         len(groups.ByBoundary[commands.BoundaryReadOnly]),
	}
}

func GoNativePublicSurfacePolicyCountsFor(policies []commands.PublicProfilePolicy) GoNativePublicSurfacePolicyCounts {
	counts := GoNativePublicSurfacePolicyCounts{Rows: len(policies)}
	for _, policy := range policies {
		rowCounts := GoNativePublicSurfacePolicyRowCountsFor(policy)
		counts.Violations += policy.ViolationCount
		counts.ViolationCommands += rowCounts.Commands
	}
	return counts
}

func GoNativePublicSurfacePolicyRowCountsFor(policy commands.PublicProfilePolicy) GoNativePublicSurfacePolicyRowCounts {
	return GoNativePublicSurfacePolicyRowCounts{Commands: len(policy.Commands)}
}

func goNativePublicSurfaceHandoffDetails(surface GoNativePublicSurface) []string {
	counts := GoNativePublicSurfaceCountsFor(surface)
	return []string{
		fmt.Sprintf("entrypoint=%s present=%t catalog=%s catalogPresent=%t", surface.Entrypoint, surface.EntrypointPresent, surface.CommandCatalogPath, surface.CommandCatalogPresent),
		fmt.Sprintf("default=%s commands=%d handlers=%d symbols=%d profiles=%d boundaries=%d alternative=%s", surface.DefaultCommand, counts.Commands, counts.HandlerCommands, counts.SymbolCommands, counts.CommandProfiles, counts.MutationBoundaries, surface.AlternativePattern),
		fmt.Sprintf("profileSummary total=%d readOnly=%d mutating=%d writesCase=%d writesKit=%d reviewFirst=%d applyRequired=%d heavyTool=%d authorityConfirmed=%d", surface.CommandProfileSummary.Total, counts.ReadOnly, counts.Mutating, counts.WritesCase, counts.WritesKit, counts.ReviewFirst, counts.ApplyRequired, counts.HeavyTool, counts.AuthorityConfirmed),
		fmt.Sprintf("profileGroups readOnly=%s reviewFirst=%s writesKit=%s", strings.Join(surface.CommandProfileGroups.ReadOnly, ","), strings.Join(surface.CommandProfileGroups.ReviewFirst, ","), strings.Join(surface.CommandProfileGroups.WritesKit, ",")),
		fmt.Sprintf("profileBoundaries rows=%d caseLocalApply=%s kitReviewFirst=%s readOnly=%s", counts.BoundaryRows, strings.Join(surface.CommandProfileGroups.ByBoundary[commands.BoundaryCaseLocalApply], ","), strings.Join(surface.CommandProfileGroups.ByBoundary[commands.BoundaryKitReviewFirst], ","), strings.Join(surface.CommandProfileGroups.ByBoundary[commands.BoundaryReadOnly], ",")),
		fmt.Sprintf("profilePolicies rows=%d violations=%d", counts.PolicyRows, counts.PolicyViolations),
		fmt.Sprintf("facadeRemovalReady=%t prerequisites=%d", surface.FacadeRemovalReady, counts.FacadePrerequisites),
		fmt.Sprintf("unsupportedDiagnostic=%t", surface.UnsupportedCommandDiagnosticPresent),
	}
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
	groupCounts := GoNativePublicSurfaceGroupCountsFor(inventory.CommandProfileGroups)
	if groupCounts.ReadOnly != inventory.CommandProfileSummary.ReadOnly || groupCounts.Mutating != inventory.CommandProfileSummary.Mutating || groupCounts.WritesCase != inventory.CommandProfileSummary.WritesCase || groupCounts.WritesKit != inventory.CommandProfileSummary.WritesKit || groupCounts.ReviewFirst != inventory.CommandProfileSummary.ReviewFirst || groupCounts.ApplyRequired != inventory.CommandProfileSummary.ApplyRequired || groupCounts.HeavyTool != inventory.CommandProfileSummary.HeavyTool || groupCounts.AuthorityConfirmed != inventory.CommandProfileSummary.AuthorityConfirmed {
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
	if GoNativePublicSurfacePolicyCountsFor(inventory.CommandProfilePolicies).Rows != GoNativePublicSurfacePolicyCountsFor(computedPolicies).Rows {
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
	if GoNativePublicSurfaceCountsFor(inventory).PolicyViolations != 0 {
		inventory.Warnings = append(inventory.Warnings, "Go-native public command profile policy rows contain violations")
	}
	for _, policy := range inventory.CommandProfilePolicies {
		rowCounts := GoNativePublicSurfacePolicyRowCountsFor(policy)
		if !policy.Ready || policy.ViolationCount != 0 || rowCounts.Commands != 0 {
			inventory.Warnings = append(inventory.Warnings, fmt.Sprintf("Go-native public command profile policy is not ready: %s", policy.Policy))
		}
	}
	inventory.FacadeRemovalPrerequisites = goNativePublicSurfaceFacadeRemovalPrerequisites(inventory)
	inventory.FacadeRemovalReady = goNativePublicSurfacePrerequisitesReady(inventory.FacadeRemovalPrerequisites)
	for _, prerequisite := range inventory.FacadeRemovalPrerequisites {
		if !prerequisite.Ready {
			inventory.Warnings = append(inventory.Warnings, fmt.Sprintf("Go-native public surface facade removal prerequisite is not ready: %s", prerequisite.Name))
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

func goNativePublicSurfaceFacadeRemovalPrerequisites(inventory GoNativePublicSurface) []GoNativePublicSurfacePrerequisite {
	counts := GoNativePublicSurfaceCountsFor(inventory)
	return []GoNativePublicSurfacePrerequisite{
		{
			Name:    "entrypoint",
			Ready:   inventory.EntrypointPresent,
			Summary: fmt.Sprintf("entrypoint=%s present=%t", inventory.Entrypoint, inventory.EntrypointPresent),
		},
		{
			Name:    "catalog-handler-symbol-profile-coverage",
			Ready:   counts.Commands > 0 && counts.HandlerCommands == counts.Commands && counts.SymbolCommands == counts.Commands && counts.CommandProfiles == counts.Commands,
			Summary: fmt.Sprintf("commands=%d handlers=%d symbols=%d profiles=%d", counts.Commands, counts.HandlerCommands, counts.SymbolCommands, counts.CommandProfiles),
		},
		{
			Name:    "mutation-boundary-inventory",
			Ready:   counts.MutationBoundaries > 0 && counts.BoundaryRows == counts.MutationBoundaries && inventory.CommandProfileSummary.Total == counts.Commands,
			Summary: fmt.Sprintf("boundaries=%d rows=%d profileTotal=%d", counts.MutationBoundaries, counts.BoundaryRows, inventory.CommandProfileSummary.Total),
		},
		{
			Name:    "profile-policy-guards",
			Ready:   counts.PolicyRows > 0 && counts.PolicyViolations == 0 && counts.HeavyTool == 0 && counts.AuthorityConfirmed == 0,
			Summary: fmt.Sprintf("policies=%d violations=%d heavyTool=%d authorityConfirmed=%d", counts.PolicyRows, counts.PolicyViolations, counts.HeavyTool, counts.AuthorityConfirmed),
		},
		{
			Name:    "unsupported-command-diagnostic",
			Ready:   inventory.UnsupportedCommandDiagnosticPresent && inventory.AlternativePattern == commands.GoNativeAlternativePattern,
			Summary: fmt.Sprintf("alternative=%s unsupportedDiagnostic=%t", inventory.AlternativePattern, inventory.UnsupportedCommandDiagnosticPresent),
		},
	}
}

func goNativePublicSurfacePrerequisitesReady(prerequisites []GoNativePublicSurfacePrerequisite) bool {
	if len(prerequisites) == 0 {
		return false
	}
	for _, prerequisite := range prerequisites {
		if !prerequisite.Ready {
			return false
		}
	}
	return true
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
