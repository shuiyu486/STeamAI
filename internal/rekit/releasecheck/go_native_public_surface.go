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
	Commands                    int
	HandlerCommands             int
	SymbolCommands              int
	CommandProfiles             int
	MutationBoundaries          int
	ProfileSummary              GoNativePublicSurfaceProfileSummaryCounts
	ProfileTotal                int
	Boundaries                  GoNativePublicSurfaceBoundaryCounts
	BoundaryRows                int
	BoundaryCommands            int
	BoundaryCountedCommands     int
	PolicyRows                  int
	PolicyViolations            int
	FacadeRemoval               GoNativePublicSurfaceFacadeRemovalPrerequisiteCounts
	FacadePrerequisites         int
	FacadeNotReadyPrerequisites int
	Warnings                    int
	ReadOnly                    int
	Mutating                    int
	WritesCase                  int
	WritesKit                   int
	ReviewFirst                 int
	ApplyRequired               int
	HeavyTool                   int
	AuthorityConfirmed          int
	Groups                      GoNativePublicSurfaceGroupCounts
	Policies                    GoNativePublicSurfacePolicyCounts
	CaseLocalApplyCommands      int
	CaseLocalReviewCommands     int
	KitReviewFirstCommands      int
	ReadOnlyCommands            int
}

type GoNativePublicSurfaceProfileSummaryCounts struct {
	Total                      int
	ReadOnly                   int
	Mutating                   int
	WritesCase                 int
	WritesKit                  int
	ReviewFirst                int
	ApplyRequired              int
	HeavyTool                  int
	AuthorityConfirmed         int
	BoundaryCaseLocalAppend    int
	BoundaryCaseLocalApply     int
	BoundaryCaseLocalBootstrap int
	BoundaryCaseLocalArtifact  int
	BoundaryCaseLocalReview    int
	BoundaryKitReview          int
	BoundaryReadOnly           int
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

type GoNativePublicSurfaceBoundaryCounts struct {
	Rows            int
	Commands        int
	CountedCommands int
}

type GoNativePublicSurfacePolicyCounts struct {
	Rows              int
	Violations        int
	ViolationCommands int
}

type GoNativePublicSurfaceFacadeRemovalPrerequisiteCounts struct {
	Rows     int
	NotReady int
}

type GoNativePublicSurfaceBoundaryRowCounts struct {
	Commands int
	Count    int
}

type GoNativePublicSurfacePolicyRowCounts struct {
	Commands int
}

func GoNativePublicSurfaceProfileSummaryCountsFor(summary commands.PublicProfileSummary) GoNativePublicSurfaceProfileSummaryCounts {
	return GoNativePublicSurfaceProfileSummaryCounts{
		Total:                      summary.Total,
		ReadOnly:                   summary.ReadOnly,
		Mutating:                   summary.Mutating,
		WritesCase:                 summary.WritesCase,
		WritesKit:                  summary.WritesKit,
		ReviewFirst:                summary.ReviewFirst,
		ApplyRequired:              summary.ApplyRequired,
		HeavyTool:                  summary.HeavyTool,
		AuthorityConfirmed:         summary.AuthorityConfirmed,
		BoundaryCaseLocalAppend:    summary.Boundaries[commands.BoundaryCaseLocalAppend],
		BoundaryCaseLocalApply:     summary.Boundaries[commands.BoundaryCaseLocalApply],
		BoundaryCaseLocalBootstrap: summary.Boundaries[commands.BoundaryCaseLocalReadOrBootstrap],
		BoundaryCaseLocalArtifact:  summary.Boundaries[commands.BoundaryCaseLocalReviewArtifact],
		BoundaryCaseLocalReview:    summary.Boundaries[commands.BoundaryCaseLocalReviewFirst],
		BoundaryKitReview:          summary.Boundaries[commands.BoundaryKitReviewFirst],
		BoundaryReadOnly:           summary.Boundaries[commands.BoundaryReadOnly],
	}
}

func goNativePublicSurfaceProfileSummaryCountsMatch(left, right GoNativePublicSurfaceProfileSummaryCounts) bool {
	return left.Total == right.Total &&
		left.ReadOnly == right.ReadOnly &&
		left.Mutating == right.Mutating &&
		left.WritesCase == right.WritesCase &&
		left.WritesKit == right.WritesKit &&
		left.ReviewFirst == right.ReviewFirst &&
		left.ApplyRequired == right.ApplyRequired &&
		left.HeavyTool == right.HeavyTool &&
		left.AuthorityConfirmed == right.AuthorityConfirmed &&
		left.BoundaryCaseLocalAppend == right.BoundaryCaseLocalAppend &&
		left.BoundaryCaseLocalApply == right.BoundaryCaseLocalApply &&
		left.BoundaryCaseLocalBootstrap == right.BoundaryCaseLocalBootstrap &&
		left.BoundaryCaseLocalArtifact == right.BoundaryCaseLocalArtifact &&
		left.BoundaryCaseLocalReview == right.BoundaryCaseLocalReview &&
		left.BoundaryKitReview == right.BoundaryKitReview &&
		left.BoundaryReadOnly == right.BoundaryReadOnly
}

func goNativePublicSurfaceProfileSummaryRequiredCountsPresent(counts GoNativePublicSurfaceProfileSummaryCounts) bool {
	return counts.ReadOnly > 0 && counts.Mutating > 0 && counts.WritesCase > 0 && counts.ReviewFirst > 0 && counts.ApplyRequired > 0
}

func goNativePublicSurfaceGroupCountsMatchProfileSummary(groupCounts GoNativePublicSurfaceGroupCounts, summaryCounts GoNativePublicSurfaceProfileSummaryCounts) bool {
	return groupCounts.ReadOnly == summaryCounts.ReadOnly &&
		groupCounts.Mutating == summaryCounts.Mutating &&
		groupCounts.WritesCase == summaryCounts.WritesCase &&
		groupCounts.WritesKit == summaryCounts.WritesKit &&
		groupCounts.ReviewFirst == summaryCounts.ReviewFirst &&
		groupCounts.ApplyRequired == summaryCounts.ApplyRequired &&
		groupCounts.HeavyTool == summaryCounts.HeavyTool &&
		groupCounts.AuthorityConfirmed == summaryCounts.AuthorityConfirmed
}

func goNativePublicSurfaceProfileSummaryBoundaryCountFor(counts GoNativePublicSurfaceProfileSummaryCounts, boundary string) int {
	switch boundary {
	case commands.BoundaryCaseLocalAppend:
		return counts.BoundaryCaseLocalAppend
	case commands.BoundaryCaseLocalApply:
		return counts.BoundaryCaseLocalApply
	case commands.BoundaryCaseLocalReadOrBootstrap:
		return counts.BoundaryCaseLocalBootstrap
	case commands.BoundaryCaseLocalReviewArtifact:
		return counts.BoundaryCaseLocalArtifact
	case commands.BoundaryCaseLocalReviewFirst:
		return counts.BoundaryCaseLocalReview
	case commands.BoundaryKitReviewFirst:
		return counts.BoundaryKitReview
	case commands.BoundaryReadOnly:
		return counts.BoundaryReadOnly
	default:
		return 0
	}
}

func GoNativePublicSurfaceCountsFor(surface GoNativePublicSurface) GoNativePublicSurfaceCounts {
	profileSummaryCounts := GoNativePublicSurfaceProfileSummaryCountsFor(surface.CommandProfileSummary)
	groupCounts := GoNativePublicSurfaceGroupCountsFor(surface.CommandProfileGroups)
	boundaryCounts := GoNativePublicSurfaceBoundaryCountsFor(surface.CommandProfileBoundaries)
	policyCounts := GoNativePublicSurfacePolicyCountsFor(surface.CommandProfilePolicies)
	facadeRemovalCounts := GoNativePublicSurfaceFacadeRemovalPrerequisiteCountsFor(surface.FacadeRemovalPrerequisites)
	return GoNativePublicSurfaceCounts{
		Commands:                    len(surface.Commands),
		HandlerCommands:             len(surface.HandlerCommands),
		SymbolCommands:              len(surface.SymbolCommands),
		CommandProfiles:             len(surface.CommandProfiles),
		MutationBoundaries:          len(surface.MutationBoundaries),
		ProfileSummary:              profileSummaryCounts,
		ProfileTotal:                profileSummaryCounts.Total,
		Boundaries:                  boundaryCounts,
		BoundaryRows:                boundaryCounts.Rows,
		BoundaryCommands:            boundaryCounts.Commands,
		BoundaryCountedCommands:     boundaryCounts.CountedCommands,
		PolicyRows:                  policyCounts.Rows,
		PolicyViolations:            policyCounts.Violations,
		FacadeRemoval:               facadeRemovalCounts,
		FacadePrerequisites:         facadeRemovalCounts.Rows,
		FacadeNotReadyPrerequisites: facadeRemovalCounts.NotReady,
		Warnings:                    len(surface.Warnings),
		ReadOnly:                    profileSummaryCounts.ReadOnly,
		Mutating:                    profileSummaryCounts.Mutating,
		WritesCase:                  profileSummaryCounts.WritesCase,
		WritesKit:                   profileSummaryCounts.WritesKit,
		ReviewFirst:                 profileSummaryCounts.ReviewFirst,
		ApplyRequired:               profileSummaryCounts.ApplyRequired,
		HeavyTool:                   profileSummaryCounts.HeavyTool,
		AuthorityConfirmed:          profileSummaryCounts.AuthorityConfirmed,
		Groups:                      groupCounts,
		Policies:                    policyCounts,
		CaseLocalApplyCommands:      groupCounts.CaseLocalApply,
		CaseLocalReviewCommands:     groupCounts.CaseLocalReviewFirst,
		KitReviewFirstCommands:      groupCounts.KitReviewFirst,
		ReadOnlyCommands:            groupCounts.ReadOnly,
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

func GoNativePublicSurfaceBoundaryCountsFor(boundaries []commands.PublicProfileBoundary) GoNativePublicSurfaceBoundaryCounts {
	counts := GoNativePublicSurfaceBoundaryCounts{Rows: len(boundaries)}
	for _, boundary := range boundaries {
		rowCounts := GoNativePublicSurfaceBoundaryRowCountsFor(boundary)
		counts.Commands += rowCounts.Commands
		counts.CountedCommands += rowCounts.Count
	}
	return counts
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

func GoNativePublicSurfaceFacadeRemovalPrerequisiteCountsFor(prerequisites []GoNativePublicSurfacePrerequisite) GoNativePublicSurfaceFacadeRemovalPrerequisiteCounts {
	counts := GoNativePublicSurfaceFacadeRemovalPrerequisiteCounts{Rows: len(prerequisites)}
	for _, prerequisite := range prerequisites {
		if !prerequisite.Ready {
			counts.NotReady++
		}
	}
	return counts
}

func GoNativePublicSurfaceBoundaryRowCountsFor(boundary commands.PublicProfileBoundary) GoNativePublicSurfaceBoundaryRowCounts {
	return GoNativePublicSurfaceBoundaryRowCounts{
		Commands: len(boundary.Commands),
		Count:    boundary.Count,
	}
}

func GoNativePublicSurfacePolicyRowCountsFor(policy commands.PublicProfilePolicy) GoNativePublicSurfacePolicyRowCounts {
	return GoNativePublicSurfacePolicyRowCounts{Commands: len(policy.Commands)}
}

func goNativePublicSurfaceHandoffDetails(surface GoNativePublicSurface) []string {
	counts := GoNativePublicSurfaceCountsFor(surface)
	return []string{
		fmt.Sprintf("entrypoint=%s present=%t catalog=%s catalogPresent=%t", surface.Entrypoint, surface.EntrypointPresent, surface.CommandCatalogPath, surface.CommandCatalogPresent),
		fmt.Sprintf("default=%s commands=%d handlers=%d symbols=%d profiles=%d boundaries=%d alternative=%s", surface.DefaultCommand, counts.Commands, counts.HandlerCommands, counts.SymbolCommands, counts.CommandProfiles, counts.MutationBoundaries, surface.AlternativePattern),
		fmt.Sprintf("profileSummary total=%d readOnly=%d mutating=%d writesCase=%d writesKit=%d reviewFirst=%d applyRequired=%d heavyTool=%d authorityConfirmed=%d", counts.ProfileTotal, counts.ReadOnly, counts.Mutating, counts.WritesCase, counts.WritesKit, counts.ReviewFirst, counts.ApplyRequired, counts.HeavyTool, counts.AuthorityConfirmed),
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
	profileSummaryCounts := GoNativePublicSurfaceProfileSummaryCountsFor(inventory.CommandProfileSummary)
	computedSummary := commands.PublicProfileSummaryFor(inventory.CommandProfiles)
	computedSummaryCounts := GoNativePublicSurfaceProfileSummaryCountsFor(computedSummary)
	if !goNativePublicSurfaceProfileSummaryCountsMatch(profileSummaryCounts, computedSummaryCounts) {
		inventory.Warnings = append(inventory.Warnings, "Go-native public command profile summary does not match profile catalog")
	}
	for _, boundary := range commands.KnownMutationBoundaries() {
		if goNativePublicSurfaceProfileSummaryBoundaryCountFor(profileSummaryCounts, boundary) != goNativePublicSurfaceProfileSummaryBoundaryCountFor(computedSummaryCounts, boundary) {
			inventory.Warnings = append(inventory.Warnings, fmt.Sprintf("Go-native public command profile summary boundary count mismatch for %s", boundary))
		}
	}
	if !goNativePublicSurfaceProfileSummaryRequiredCountsPresent(profileSummaryCounts) {
		inventory.Warnings = append(inventory.Warnings, "Go-native public command profile summary omitted required public command boundary counts")
	}
	if profileSummaryCounts.HeavyTool != 0 {
		inventory.Warnings = append(inventory.Warnings, "Go-native public command profile summary unexpectedly includes heavy-tool executors")
	}
	if profileSummaryCounts.AuthorityConfirmed != 0 {
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
	if !goNativePublicSurfaceGroupCountsMatchProfileSummary(groupCounts, profileSummaryCounts) {
		inventory.Warnings = append(inventory.Warnings, "Go-native public command profile group counts do not match summary")
	}
	boundaryCounts := GoNativePublicSurfaceBoundaryCountsFor(inventory.CommandProfileBoundaries)
	boundaryRows := map[string]commands.PublicProfileBoundary{}
	for _, row := range inventory.CommandProfileBoundaries {
		rowCounts := GoNativePublicSurfaceBoundaryRowCountsFor(row)
		if !commands.IsKnownMutationBoundary(row.Boundary) {
			inventory.Warnings = append(inventory.Warnings, fmt.Sprintf("Go-native public command profile boundary row has unknown boundary: %s", row.Boundary))
		}
		if _, ok := boundaryRows[row.Boundary]; ok {
			inventory.Warnings = append(inventory.Warnings, fmt.Sprintf("Go-native public command profile boundary row duplicated boundary: %s", row.Boundary))
		}
		boundaryRows[row.Boundary] = row
		if rowCounts.Count != rowCounts.Commands {
			inventory.Warnings = append(inventory.Warnings, fmt.Sprintf("Go-native public command profile boundary row count mismatch for %s", row.Boundary))
		}
		if !slices.IsSorted(row.Commands) {
			inventory.Warnings = append(inventory.Warnings, fmt.Sprintf("Go-native public command profile boundary row commands are not sorted for %s", row.Boundary))
		}
		if goNativePublicSurfaceProfileSummaryBoundaryCountFor(profileSummaryCounts, row.Boundary) != rowCounts.Count {
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
	if boundaryCounts.Rows != len(inventory.MutationBoundaries) {
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
	if GoNativePublicSurfaceCountsFor(inventory).Warnings > 0 {
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
			Ready:   counts.MutationBoundaries > 0 && counts.Boundaries.Rows == counts.MutationBoundaries && counts.Boundaries.Commands == counts.Boundaries.CountedCommands && counts.Boundaries.Commands == counts.Commands && counts.ProfileTotal == counts.Commands,
			Summary: fmt.Sprintf("boundaries=%d rows=%d profileTotal=%d", counts.MutationBoundaries, counts.Boundaries.Rows, counts.ProfileTotal),
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
	counts := GoNativePublicSurfaceFacadeRemovalPrerequisiteCountsFor(prerequisites)
	return counts.Rows > 0 && counts.NotReady == 0
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
