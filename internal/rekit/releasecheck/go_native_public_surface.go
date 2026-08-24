package releasecheck

import (
	"fmt"
	"go/ast"
	"go/parser"
	"go/token"
	"os"
	"path/filepath"
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
	RuntimeOwners                       []GoNativePublicRuntimeOwner        `json:"runtimeOwners"`
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

type GoNativePublicRuntimeOwner struct {
	Command          string `json:"command"`
	Mode             string `json:"mode"`
	OwnerKind        string `json:"ownerKind"`
	Resolver         string `json:"resolver"`
	Binder           string `json:"binder,omitempty"`
	Validator        string `json:"validator,omitempty"`
	Handler          string `json:"handler"`
	PublicationOwner string `json:"publicationOwner"`
	CallPath         string `json:"callPath"`
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
	Catalog                     GoNativePublicSurfaceCatalogCounts
	SymbolCatalog               GoNativePublicSurfaceSymbolCatalogCounts
	Coverage                    GoNativePublicSurfaceCoverageCounts
	MutationBoundaryInventory   GoNativePublicSurfaceMutationBoundaryCounts
	ProfileCatalog              GoNativePublicSurfaceProfileCatalogCounts
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

type GoNativePublicSurfaceCatalogCounts struct {
	Commands   int
	Empty      int
	Duplicates int
}

type GoNativePublicSurfaceSymbolCatalogCounts struct {
	Symbols       int
	EmptySymbols  int
	EmptyCommands int
}

type GoNativePublicSurfaceCoverageCounts struct {
	Commands        int
	HandlerCommands int
	SymbolCommands  int
	ProfileCommands int
	CommandProfiles int
	HandlerMissing  int
	HandlerUnknown  int
	SymbolMissing   int
	SymbolUnknown   int
	ProfileMissing  int
	ProfileUnknown  int
}

type GoNativePublicSurfaceCoverageDrift struct {
	HandlerMissing []string
	HandlerUnknown []string
	SymbolMissing  []string
	SymbolUnknown  []string
	ProfileMissing []string
	ProfileUnknown []string
}

type GoNativePublicSurfaceMutationBoundaryCounts struct {
	Rows    int
	Unknown int
}

type GoNativePublicSurfaceProfileCatalogCounts struct {
	Rows               int
	Empty              int
	Duplicates         int
	UnknownBoundaries  int
	HeavyTool          int
	AuthorityConfirmed int
	WritesKitNoReview  int
	ReviewNoApply      int
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
	BoundaryCaseLocalWriteback int
	BoundaryCaseLocalReview    int
	BoundaryKitReview          int
	BoundaryLocalValidation    int
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
	CaseLocalReviewWriteback int
	CaseLocalReviewFirst     int
	KitReviewFirst           int
	LocalValidationReceipt   int
	BoundaryReadOnly         int
}

type GoNativePublicSurfaceBoundaryCounts struct {
	Rows               int
	Commands           int
	CountedCommands    int
	Unknown            int
	Duplicates         int
	CountMismatches    int
	Unsorted           int
	SummaryMismatches  int
	GroupMismatches    int
	Missing            int
	CoverageMismatches int
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

func GoNativePublicSurfaceCatalogCountsFor(commands []string) GoNativePublicSurfaceCatalogCounts {
	counts := GoNativePublicSurfaceCatalogCounts{Commands: len(commands)}
	seen := map[string]bool{}
	for _, command := range commands {
		if strings.TrimSpace(command) == "" {
			counts.Empty++
			continue
		}
		if seen[command] {
			counts.Duplicates++
		}
		seen[command] = true
	}
	return counts
}

func GoNativePublicSurfaceMutationBoundaryCountsFor(boundaries []string) GoNativePublicSurfaceMutationBoundaryCounts {
	counts := GoNativePublicSurfaceMutationBoundaryCounts{Rows: len(boundaries)}
	for _, boundary := range boundaries {
		if !commands.IsKnownMutationBoundary(boundary) {
			counts.Unknown++
		}
	}
	return counts
}

func GoNativePublicSurfaceSymbolCatalogCountsFor(symbols map[string]string) GoNativePublicSurfaceSymbolCatalogCounts {
	counts := GoNativePublicSurfaceSymbolCatalogCounts{Symbols: len(symbols)}
	for symbol, command := range symbols {
		if strings.TrimSpace(symbol) == "" {
			counts.EmptySymbols++
		}
		if strings.TrimSpace(command) == "" {
			counts.EmptyCommands++
		}
	}
	return counts
}

func GoNativePublicSurfaceProfileCatalogCountsFor(profiles []commands.PublicProfile) GoNativePublicSurfaceProfileCatalogCounts {
	counts := GoNativePublicSurfaceProfileCatalogCounts{Rows: len(profiles)}
	seen := map[string]bool{}
	for _, profile := range profiles {
		command := strings.TrimSpace(profile.Command)
		if command == "" {
			counts.Empty++
			continue
		}
		if seen[command] {
			counts.Duplicates++
		}
		seen[command] = true
		if !commands.IsKnownMutationBoundary(profile.MutationBoundary) {
			counts.UnknownBoundaries++
		}
		if profile.HeavyTool {
			counts.HeavyTool++
		}
		if profile.AuthorityConfirmed {
			counts.AuthorityConfirmed++
		}
		if profile.WritesKit && !profile.ReviewFirst {
			counts.WritesKitNoReview++
		}
		if profile.ReviewFirst && !profile.ApplyRequired {
			counts.ReviewNoApply++
		}
	}
	return counts
}

func GoNativePublicSurfaceCoverageCountsFor(surface GoNativePublicSurface) GoNativePublicSurfaceCoverageCounts {
	drift := GoNativePublicSurfaceCoverageDriftFor(surface)
	return GoNativePublicSurfaceCoverageCounts{
		Commands:        len(surface.Commands),
		HandlerCommands: len(surface.HandlerCommands),
		SymbolCommands:  len(surface.SymbolCommands),
		ProfileCommands: len(commands.PublicProfileCommands(surface.CommandProfiles)),
		CommandProfiles: len(surface.CommandProfiles),
		HandlerMissing:  len(drift.HandlerMissing),
		HandlerUnknown:  len(drift.HandlerUnknown),
		SymbolMissing:   len(drift.SymbolMissing),
		SymbolUnknown:   len(drift.SymbolUnknown),
		ProfileMissing:  len(drift.ProfileMissing),
		ProfileUnknown:  len(drift.ProfileUnknown),
	}
}

func GoNativePublicSurfaceCoverageDriftFor(surface GoNativePublicSurface) GoNativePublicSurfaceCoverageDrift {
	symbolCommandValues := goNativePublicSurfaceSymbolCommandValues(surface.SymbolCommands)
	profileCommands := commands.PublicProfileCommands(surface.CommandProfiles)
	return GoNativePublicSurfaceCoverageDrift{
		HandlerMissing: commands.MissingPublicHandlers(surface.HandlerCommands),
		HandlerUnknown: commands.UnknownPublicHandlers(surface.HandlerCommands),
		SymbolMissing:  commands.MissingPublicHandlers(symbolCommandValues),
		SymbolUnknown:  commands.UnknownPublicHandlers(symbolCommandValues),
		ProfileMissing: commands.MissingPublicHandlers(profileCommands),
		ProfileUnknown: commands.UnknownPublicHandlers(profileCommands),
	}
}

func goNativePublicSurfaceSymbolCommandValues(symbols map[string]string) []string {
	values := []string{}
	for _, command := range symbols {
		if strings.TrimSpace(command) != "" {
			values = append(values, command)
		}
	}
	return values
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
		BoundaryCaseLocalWriteback: summary.Boundaries[commands.BoundaryCaseLocalReviewWriteback],
		BoundaryCaseLocalReview:    summary.Boundaries[commands.BoundaryCaseLocalReviewFirst],
		BoundaryKitReview:          summary.Boundaries[commands.BoundaryKitReviewFirst],
		BoundaryLocalValidation:    summary.Boundaries[commands.BoundaryLocalValidationReceipt],
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
		left.BoundaryCaseLocalWriteback == right.BoundaryCaseLocalWriteback &&
		left.BoundaryCaseLocalReview == right.BoundaryCaseLocalReview &&
		left.BoundaryKitReview == right.BoundaryKitReview &&
		left.BoundaryLocalValidation == right.BoundaryLocalValidation &&
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
	case commands.BoundaryCaseLocalReviewWriteback:
		return counts.BoundaryCaseLocalWriteback
	case commands.BoundaryCaseLocalReviewFirst:
		return counts.BoundaryCaseLocalReview
	case commands.BoundaryKitReviewFirst:
		return counts.BoundaryKitReview
	case commands.BoundaryLocalValidationReceipt:
		return counts.BoundaryLocalValidation
	case commands.BoundaryReadOnly:
		return counts.BoundaryReadOnly
	default:
		return 0
	}
}

func GoNativePublicSurfaceCountsFor(surface GoNativePublicSurface) GoNativePublicSurfaceCounts {
	catalogCounts := GoNativePublicSurfaceCatalogCountsFor(surface.Commands)
	symbolCatalogCounts := GoNativePublicSurfaceSymbolCatalogCountsFor(surface.SymbolCommands)
	coverageCounts := GoNativePublicSurfaceCoverageCountsFor(surface)
	mutationBoundaryCounts := GoNativePublicSurfaceMutationBoundaryCountsFor(surface.MutationBoundaries)
	profileCatalogCounts := GoNativePublicSurfaceProfileCatalogCountsFor(surface.CommandProfiles)
	profileSummaryCounts := GoNativePublicSurfaceProfileSummaryCountsFor(surface.CommandProfileSummary)
	groupCounts := GoNativePublicSurfaceGroupCountsFor(surface.CommandProfileGroups)
	boundaryCounts := GoNativePublicSurfaceBoundaryCountsFor(surface.CommandProfileBoundaries, surface.MutationBoundaries, profileSummaryCounts, surface.CommandProfileGroups)
	policyCounts := GoNativePublicSurfacePolicyCountsFor(surface.CommandProfilePolicies)
	facadeRemovalCounts := GoNativePublicSurfaceFacadeRemovalPrerequisiteCountsFor(surface.FacadeRemovalPrerequisites)
	return GoNativePublicSurfaceCounts{
		Commands:                    coverageCounts.Commands,
		HandlerCommands:             coverageCounts.HandlerCommands,
		SymbolCommands:              coverageCounts.SymbolCommands,
		CommandProfiles:             coverageCounts.CommandProfiles,
		MutationBoundaries:          mutationBoundaryCounts.Rows,
		Catalog:                     catalogCounts,
		SymbolCatalog:               symbolCatalogCounts,
		Coverage:                    coverageCounts,
		MutationBoundaryInventory:   mutationBoundaryCounts,
		ProfileCatalog:              profileCatalogCounts,
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
		CaseLocalReviewWriteback: len(groups.ByBoundary[commands.BoundaryCaseLocalReviewWriteback]),
		CaseLocalReviewFirst:     len(groups.ByBoundary[commands.BoundaryCaseLocalReviewFirst]),
		KitReviewFirst:           len(groups.ByBoundary[commands.BoundaryKitReviewFirst]),
		LocalValidationReceipt:   len(groups.ByBoundary[commands.BoundaryLocalValidationReceipt]),
		BoundaryReadOnly:         len(groups.ByBoundary[commands.BoundaryReadOnly]),
	}
}

func GoNativePublicSurfaceBoundaryCountsFor(boundaries []commands.PublicProfileBoundary, mutationBoundaries []string, summaryCounts GoNativePublicSurfaceProfileSummaryCounts, groups commands.PublicProfileGroups) GoNativePublicSurfaceBoundaryCounts {
	counts := GoNativePublicSurfaceBoundaryCounts{Rows: len(boundaries)}
	seen := map[string]bool{}
	for _, boundary := range boundaries {
		rowCounts := GoNativePublicSurfaceBoundaryRowCountsFor(boundary)
		counts.Commands += rowCounts.Commands
		counts.CountedCommands += rowCounts.Count
		if !commands.IsKnownMutationBoundary(boundary.Boundary) {
			counts.Unknown++
		}
		if seen[boundary.Boundary] {
			counts.Duplicates++
		}
		seen[boundary.Boundary] = true
		if rowCounts.Count != rowCounts.Commands {
			counts.CountMismatches++
		}
		if !slices.IsSorted(boundary.Commands) {
			counts.Unsorted++
		}
		if goNativePublicSurfaceProfileSummaryBoundaryCountFor(summaryCounts, boundary.Boundary) != rowCounts.Count {
			counts.SummaryMismatches++
		}
		if !slices.Equal(groups.ByBoundary[boundary.Boundary], boundary.Commands) {
			counts.GroupMismatches++
		}
	}
	for _, boundary := range mutationBoundaries {
		if !seen[boundary] {
			counts.Missing++
		}
	}
	if counts.Rows != len(mutationBoundaries) {
		counts.CoverageMismatches++
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
		fmt.Sprintf("default=%s commands=%d handlers=%d exactRuntimeOwners=%d symbols=%d profiles=%d boundaries=%d alternative=%s", surface.DefaultCommand, counts.Commands, counts.HandlerCommands, len(surface.RuntimeOwners), counts.SymbolCommands, counts.CommandProfiles, counts.MutationBoundaries, surface.AlternativePattern),
		fmt.Sprintf("profileSummary total=%d readOnly=%d mutating=%d writesCase=%d writesKit=%d reviewFirst=%d applyRequired=%d heavyTool=%d authorityConfirmed=%d", counts.ProfileTotal, counts.ReadOnly, counts.Mutating, counts.WritesCase, counts.WritesKit, counts.ReviewFirst, counts.ApplyRequired, counts.HeavyTool, counts.AuthorityConfirmed),
		fmt.Sprintf("profileGroups readOnly=%s reviewFirst=%s writesKit=%s", strings.Join(surface.CommandProfileGroups.ReadOnly, ","), strings.Join(surface.CommandProfileGroups.ReviewFirst, ","), strings.Join(surface.CommandProfileGroups.WritesKit, ",")),
		fmt.Sprintf("profileBoundaries rows=%d caseLocalApply=%s caseLocalReviewWriteback=%s caseLocalReviewFirst=%s kitReviewFirst=%s readOnly=%s", counts.BoundaryRows, strings.Join(surface.CommandProfileGroups.ByBoundary[commands.BoundaryCaseLocalApply], ","), strings.Join(surface.CommandProfileGroups.ByBoundary[commands.BoundaryCaseLocalReviewWriteback], ","), strings.Join(surface.CommandProfileGroups.ByBoundary[commands.BoundaryCaseLocalReviewFirst], ","), strings.Join(surface.CommandProfileGroups.ByBoundary[commands.BoundaryKitReviewFirst], ","), strings.Join(surface.CommandProfileGroups.ByBoundary[commands.BoundaryReadOnly], ",")),
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
		RuntimeOwners:                goNativePublicRuntimeOwners(repo),
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
	for _, path := range []string{
		"internal/rekit/cli/scoped_registry.go",
		"internal/rekit/cli/cli.go",
	} {
		if data, err := os.ReadFile(filepath.Join(repo, filepath.FromSlash(path))); err == nil && strings.Contains(string(data), "commands.UnsupportedError(opt.Command)") {
			inventory.UnsupportedCommandDiagnosticPresent = true
			break
		}
	}
	if !inventory.EntrypointPresent {
		inventory.Warnings = append(inventory.Warnings, fmt.Sprintf("Go-native public entrypoint missing: %s/main.go", commands.GoNativeEntrypoint))
	}
	if !inventory.CommandCatalogPresent {
		inventory.Warnings = append(inventory.Warnings, fmt.Sprintf("Go-native public command catalog missing: %s", catalogPath))
	}
	catalogCounts := GoNativePublicSurfaceCatalogCountsFor(inventory.Commands)
	if catalogCounts.Commands == 0 {
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
	coverageDrift := GoNativePublicSurfaceCoverageDriftFor(inventory)
	for _, command := range coverageDrift.HandlerMissing {
		inventory.Warnings = append(inventory.Warnings, fmt.Sprintf("Go CLI dispatcher missing public command handler: %s", command))
	}
	for _, command := range coverageDrift.HandlerUnknown {
		inventory.Warnings = append(inventory.Warnings, fmt.Sprintf("Go CLI dispatcher exposes handler outside public command catalog: %s", command))
	}
	symbolCatalogCounts := GoNativePublicSurfaceSymbolCatalogCountsFor(inventory.SymbolCommands)
	if symbolCatalogCounts.EmptySymbols != 0 || symbolCatalogCounts.EmptyCommands != 0 {
		for symbol, command := range inventory.SymbolCommands {
			if strings.TrimSpace(symbol) == "" {
				inventory.Warnings = append(inventory.Warnings, "Go-native public command symbol catalog contains an empty symbol")
			}
			if strings.TrimSpace(command) == "" {
				inventory.Warnings = append(inventory.Warnings, fmt.Sprintf("Go-native public command symbol %q has an empty command", symbol))
			}
		}
	}
	for _, command := range coverageDrift.SymbolMissing {
		inventory.Warnings = append(inventory.Warnings, fmt.Sprintf("Go-native public command symbol missing from symbol catalog: %s", command))
	}
	for _, command := range coverageDrift.SymbolUnknown {
		inventory.Warnings = append(inventory.Warnings, fmt.Sprintf("Go-native public command symbol outside public command catalog: %s", command))
	}
	for _, command := range coverageDrift.ProfileMissing {
		inventory.Warnings = append(inventory.Warnings, fmt.Sprintf("Go-native public command profile missing from profile catalog: %s", command))
	}
	for _, command := range coverageDrift.ProfileUnknown {
		inventory.Warnings = append(inventory.Warnings, fmt.Sprintf("Go-native public command profile outside public command catalog: %s", command))
	}
	profileCatalogCounts := GoNativePublicSurfaceProfileCatalogCountsFor(inventory.CommandProfiles)
	if profileCatalogCounts.Empty != 0 || profileCatalogCounts.Duplicates != 0 || profileCatalogCounts.UnknownBoundaries != 0 || profileCatalogCounts.HeavyTool != 0 || profileCatalogCounts.AuthorityConfirmed != 0 || profileCatalogCounts.WritesKitNoReview != 0 || profileCatalogCounts.ReviewNoApply != 0 {
		profileSeen := map[string]bool{}
		for _, profile := range inventory.CommandProfiles {
			command := strings.TrimSpace(profile.Command)
			if command == "" {
				inventory.Warnings = append(inventory.Warnings, "Go-native public command profile contains an empty command")
				continue
			}
			if profileSeen[command] {
				inventory.Warnings = append(inventory.Warnings, fmt.Sprintf("Go-native public command profile contains duplicate command: %s", command))
			}
			profileSeen[command] = true
			if !commands.IsKnownMutationBoundary(profile.MutationBoundary) {
				inventory.Warnings = append(inventory.Warnings, fmt.Sprintf("Go-native public command profile has unknown mutation boundary for %s: %s", command, profile.MutationBoundary))
			}
			if profile.HeavyTool {
				inventory.Warnings = append(inventory.Warnings, fmt.Sprintf("Go-native public command profile incorrectly marks public command as heavy-tool executor: %s", command))
			}
			if profile.AuthorityConfirmed {
				inventory.Warnings = append(inventory.Warnings, fmt.Sprintf("Go-native public command profile incorrectly marks public command as authority/confirmed writer: %s", command))
			}
			if profile.WritesKit && !profile.ReviewFirst {
				inventory.Warnings = append(inventory.Warnings, fmt.Sprintf("Go-native public command profile writes kit without review-first boundary: %s", command))
			}
			if profile.ReviewFirst && !profile.ApplyRequired {
				inventory.Warnings = append(inventory.Warnings, fmt.Sprintf("Go-native public command profile review-first command missing apply-required boundary: %s", command))
			}
		}
	}
	mutationBoundaryCounts := GoNativePublicSurfaceMutationBoundaryCountsFor(inventory.MutationBoundaries)
	if mutationBoundaryCounts.Unknown != 0 {
		for _, boundary := range inventory.MutationBoundaries {
			if !commands.IsKnownMutationBoundary(boundary) {
				inventory.Warnings = append(inventory.Warnings, fmt.Sprintf("Go-native public command surface exposes unknown mutation boundary: %s", boundary))
			}
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
	boundaryCounts := GoNativePublicSurfaceBoundaryCountsFor(inventory.CommandProfileBoundaries, inventory.MutationBoundaries, profileSummaryCounts, inventory.CommandProfileGroups)
	if boundaryCounts.Unknown != 0 || boundaryCounts.Duplicates != 0 || boundaryCounts.CountMismatches != 0 || boundaryCounts.Unsorted != 0 || boundaryCounts.SummaryMismatches != 0 || boundaryCounts.GroupMismatches != 0 || boundaryCounts.Missing != 0 {
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
		if boundaryCounts.Missing != 0 {
			for _, boundary := range inventory.MutationBoundaries {
				if _, ok := boundaryRows[boundary]; !ok {
					inventory.Warnings = append(inventory.Warnings, fmt.Sprintf("Go-native public command profile boundary rows missing boundary: %s", boundary))
				}
			}
		}
	}
	if boundaryCounts.CoverageMismatches != 0 {
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
	runtimeOwnerWarnings := GoNativePublicRuntimeOwnerWarningsFor(inventory.RuntimeOwners, inventory.Commands)
	inventory.Warnings = append(inventory.Warnings, runtimeOwnerWarnings...)
	if catalogCounts.Empty != 0 || catalogCounts.Duplicates != 0 {
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
	}
	if GoNativePublicSurfaceCountsFor(inventory).Warnings > 0 {
		inventory.Ready = false
		inventory.Summary = "Go-native public command surface inventory has warnings"
	}
	return inventory
}

func GoNativePublicRuntimeOwnerWarningsFor(owners []GoNativePublicRuntimeOwner, publicCommands []string) []string {
	warnings := []string{}
	expected := map[commands.CommandScope]bool{}
	for _, descriptor := range mustScopedCommandDescriptors() {
		expected[descriptor.Scope] = true
	}
	seen := map[commands.CommandScope]bool{}
	interceptors := 0
	for _, owner := range owners {
		if owner.OwnerKind == "pre-runtime-interceptor-owner" {
			interceptors++
			if owner.Command != "*" || owner.Mode == "" || owner.Resolver == "" || owner.Validator == "" || owner.Handler == "" || owner.PublicationOwner != owner.Handler || owner.Binder != "" || owner.CallPath != "runWithOptions->runPreRuntimeCommand->"+owner.Handler {
				warnings = append(warnings, "Go CLI pre-runtime interceptor owner inventory is incomplete")
			}
			continue
		}
		scope := commands.CommandScope{Command: owner.Command, Mode: owner.Mode}
		if !commands.IsPublic(owner.Command) || !expected[scope] {
			warnings = append(warnings, fmt.Sprintf("Go CLI runtime owner inventory exposes unknown scope: %s mode %s", owner.Command, owner.Mode))
			continue
		}
		if seen[scope] {
			warnings = append(warnings, fmt.Sprintf("Go CLI runtime owner inventory duplicates scope: %s mode %s", owner.Command, owner.Mode))
			continue
		}
		seen[scope] = true
		switch owner.OwnerKind {
		case "scoped-runtime-owner":
			if owner.Resolver == "" || owner.Binder == "" || owner.Validator == "" || owner.Handler == "" || owner.PublicationOwner != owner.Handler || owner.CallPath != "runWithOptions->runOwnedCommand->runScopedCommand->executeScopedCommandRoute->"+owner.Handler {
				warnings = append(warnings, fmt.Sprintf("Go CLI scoped runtime owner inventory is incomplete for %s mode %s", owner.Command, owner.Mode))
			}
		case "pre-runtime-exclusive-owner":
			if owner.Resolver == "" || owner.Binder != "" || owner.Validator == "" || owner.Handler == "" || owner.PublicationOwner != owner.Handler || owner.CallPath != "runWithOptions->runPreRuntimeCommand->"+owner.Handler {
				warnings = append(warnings, fmt.Sprintf("Go CLI pre-runtime owner inventory is incomplete for %s mode %s", owner.Command, owner.Mode))
			}
		default:
			warnings = append(warnings, fmt.Sprintf("Go CLI runtime owner inventory has unknown owner kind for %s mode %s: %s", owner.Command, owner.Mode, owner.OwnerKind))
		}
	}
	for scope := range expected {
		if !seen[scope] {
			warnings = append(warnings, fmt.Sprintf("Go CLI runtime owner inventory is missing scope: %s mode %s", scope.Command, scope.Mode))
		}
	}
	if interceptors != 1 {
		warnings = append(warnings, fmt.Sprintf("Go CLI pre-runtime interceptor owner count drifted: %d", interceptors))
	}
	if len(publicCommands) == 0 {
		warnings = append(warnings, "Go CLI runtime owner inventory has no public command catalog")
	}
	return warnings
}

func goNativePublicRuntimeOwnerInventoryReady(inventory GoNativePublicSurface) bool {
	return len(GoNativePublicRuntimeOwnerWarningsFor(inventory.RuntimeOwners, inventory.Commands)) == 0
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
			Ready:   counts.Catalog.Commands > 0 && counts.Catalog.Empty == 0 && counts.Catalog.Duplicates == 0 && counts.SymbolCatalog.Symbols == counts.Catalog.Commands && counts.SymbolCatalog.EmptySymbols == 0 && counts.SymbolCatalog.EmptyCommands == 0 && counts.Coverage.HandlerCommands == counts.Catalog.Commands && counts.Coverage.SymbolCommands == counts.Catalog.Commands && counts.Coverage.ProfileCommands == counts.Catalog.Commands && counts.Coverage.CommandProfiles == counts.Catalog.Commands && counts.Coverage.HandlerMissing == 0 && counts.Coverage.HandlerUnknown == 0 && counts.Coverage.SymbolMissing == 0 && counts.Coverage.SymbolUnknown == 0 && counts.Coverage.ProfileMissing == 0 && counts.Coverage.ProfileUnknown == 0,
			Summary: fmt.Sprintf("commands=%d handlers=%d symbols=%d profiles=%d", counts.Catalog.Commands, counts.Coverage.HandlerCommands, counts.SymbolCatalog.Symbols, counts.Coverage.CommandProfiles),
		},
		{
			Name:    "runtime-owner-inventory",
			Ready:   goNativePublicRuntimeOwnerInventoryReady(inventory),
			Summary: fmt.Sprintf("commands=%d exactOwners=%d", counts.Catalog.Commands, len(inventory.RuntimeOwners)),
		},
		{
			Name:    "mutation-boundary-inventory",
			Ready:   counts.MutationBoundaryInventory.Rows > 0 && counts.MutationBoundaryInventory.Unknown == 0 && counts.Boundaries.Rows == counts.MutationBoundaryInventory.Rows && counts.Boundaries.Commands == counts.Boundaries.CountedCommands && counts.Boundaries.Commands == counts.Catalog.Commands && counts.ProfileTotal == counts.Catalog.Commands && counts.Boundaries.Unknown == 0 && counts.Boundaries.Duplicates == 0 && counts.Boundaries.CountMismatches == 0 && counts.Boundaries.Unsorted == 0 && counts.Boundaries.SummaryMismatches == 0 && counts.Boundaries.GroupMismatches == 0 && counts.Boundaries.Missing == 0 && counts.Boundaries.CoverageMismatches == 0,
			Summary: fmt.Sprintf("boundaries=%d rows=%d profileTotal=%d", counts.MutationBoundaryInventory.Rows, counts.Boundaries.Rows, counts.ProfileTotal),
		},
		{
			Name:    "profile-policy-guards",
			Ready:   counts.PolicyRows > 0 && counts.PolicyViolations == 0 && counts.ProfileCatalog.HeavyTool == 0 && counts.ProfileCatalog.AuthorityConfirmed == 0 && counts.ProfileCatalog.WritesKitNoReview == 0 && counts.ProfileCatalog.ReviewNoApply == 0,
			Summary: fmt.Sprintf("policies=%d violations=%d heavyTool=%d authorityConfirmed=%d", counts.PolicyRows, counts.PolicyViolations, counts.ProfileCatalog.HeavyTool, counts.ProfileCatalog.AuthorityConfirmed),
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
	symbols := commands.SymbolValues()
	seen := map[string]bool{}
	goNativePublicScopedHandlerCommands(
		filepath.Join(repo, filepath.FromSlash("internal/rekit/cli/scoped_registry.go")),
		symbols,
		seen,
	)
	out := make([]string, 0, len(seen))
	for command := range seen {
		out = append(out, command)
	}
	sort.Strings(out)
	return out
}

func goNativePublicRuntimeOwners(repo string) []GoNativePublicRuntimeOwner {
	path := filepath.Join(repo, filepath.FromSlash("internal/rekit/cli/scoped_registry.go"))
	file, err := parser.ParseFile(token.NewFileSet(), path, nil, 0)
	if err != nil {
		return nil
	}
	variables := goNativePublicVariableValues(file)
	symbols := commands.SymbolValues()
	owners := goNativePublicScopedRuntimeOwnerRows(variables["scopedCommandRuntimeOwners"], symbols)
	preRuntime := goNativePublicPreRuntimeOwnerRows(variables["preRuntimeCommandOwners"], symbols)
	for _, row := range preRuntime {
		if row.OwnerKind == "pre-runtime-exclusive-owner" {
			for index := range owners {
				if owners[index].Command == row.Command && owners[index].Mode == row.Mode {
					owners = append(owners[:index], owners[index+1:]...)
					break
				}
			}
		}
		owners = append(owners, row)
	}
	sort.Slice(owners, func(i, j int) bool {
		if owners[i].Command != owners[j].Command {
			return owners[i].Command < owners[j].Command
		}
		if owners[i].Mode != owners[j].Mode {
			return owners[i].Mode < owners[j].Mode
		}
		return owners[i].OwnerKind < owners[j].OwnerKind
	})
	return owners
}

func goNativePublicVariableValues(file *ast.File) map[string]ast.Expr {
	values := map[string]ast.Expr{}
	for _, declaration := range file.Decls {
		general, ok := declaration.(*ast.GenDecl)
		if !ok || general.Tok != token.VAR {
			continue
		}
		for _, specification := range general.Specs {
			value, ok := specification.(*ast.ValueSpec)
			if !ok {
				continue
			}
			for index, name := range value.Names {
				if index < len(value.Values) {
					values[name.Name] = value.Values[index]
				}
			}
		}
	}
	return values
}

func goNativePublicScopedRuntimeOwnerRows(expression ast.Expr, symbols map[string]string) []GoNativePublicRuntimeOwner {
	catalog, ok := expression.(*ast.CompositeLit)
	if !ok {
		return nil
	}
	rows := []GoNativePublicRuntimeOwner{}
	for _, element := range catalog.Elts {
		switch owner := element.(type) {
		case *ast.CallExpr:
			rows = append(rows, goNativePublicOwnerCallRows(owner, symbols)...)
		case *ast.CompositeLit:
			rows = append(rows, goNativePublicOwnerLiteralRows(owner, symbols)...)
		}
	}
	return rows
}

func goNativePublicOwnerCallRows(call *ast.CallExpr, symbols map[string]string) []GoNativePublicRuntimeOwner {
	function, ok := call.Fun.(*ast.Ident)
	if !ok || len(call.Args) == 0 {
		return nil
	}
	command, ok := goNativePublicCommandSymbol(call.Args[0], symbols)
	if !ok {
		return nil
	}
	var resolver, binder, validator, handler string
	var modes []string
	switch function.Name {
	case "defaultScopedCommandRuntimeOwner":
		if len(call.Args) != 4 {
			return nil
		}
		resolver = "resolveDefaultCommandMode"
		binder = goNativePublicCallbackSymbol(call.Args[1])
		validator = goNativePublicCallbackSymbol(call.Args[2])
		handler = goNativePublicCallbackSymbol(call.Args[3])
		modes = []string{commands.MutationModeDefault}
	case "descriptorScopedCommandRuntimeOwner":
		if len(call.Args) != 5 {
			return nil
		}
		resolver = goNativePublicCallbackSymbol(call.Args[1])
		binder = goNativePublicCallbackSymbol(call.Args[2])
		validator = goNativePublicCallbackSymbol(call.Args[3])
		handler = goNativePublicCallbackSymbol(call.Args[4])
		for _, descriptor := range commands.ScopedCommandDescriptorsFor(command) {
			modes = append(modes, descriptor.Scope.Mode)
		}
	case "fixedScopedCommandRuntimeOwner":
		if len(call.Args) != 3 {
			return nil
		}
		mode, ok := goNativePublicModeSymbol(call.Args[1])
		if !ok {
			return nil
		}
		resolver = "fixedScopedCommandRuntimeOwner"
		binder = "bindScopedCommand"
		validator = "validateFixedScopedCommand"
		handler = goNativePublicCallbackSymbol(call.Args[2])
		modes = []string{mode}
	default:
		return nil
	}
	if resolver == "" || binder == "" || validator == "" || handler == "" {
		return nil
	}
	rows := make([]GoNativePublicRuntimeOwner, 0, len(modes))
	for _, mode := range modes {
		rows = append(rows, scopedRuntimeOwnerRow(command, mode, resolver, binder, validator, handler))
	}
	return rows
}

func goNativePublicOwnerLiteralRows(owner *ast.CompositeLit, symbols map[string]string) []GoNativePublicRuntimeOwner {
	var command, resolver string
	var routes ast.Expr
	for _, element := range owner.Elts {
		field, ok := element.(*ast.KeyValueExpr)
		if !ok {
			continue
		}
		key, ok := field.Key.(*ast.Ident)
		if !ok {
			continue
		}
		switch key.Name {
		case "Command":
			command, _ = goNativePublicCommandSymbol(field.Value, symbols)
		case "ResolveMode":
			resolver = goNativePublicCallbackSymbol(field.Value)
		case "Routes":
			routes = field.Value
		}
	}
	catalog, ok := routes.(*ast.CompositeLit)
	if !ok || command == "" || resolver == "" {
		return nil
	}
	rows := []GoNativePublicRuntimeOwner{}
	for _, element := range catalog.Elts {
		route, ok := element.(*ast.CallExpr)
		if !ok {
			continue
		}
		function, ok := route.Fun.(*ast.Ident)
		if !ok {
			continue
		}
		switch function.Name {
		case "ownedScopedCommandRoute":
			if len(route.Args) != 5 {
				continue
			}
			mode, ok := goNativePublicModeSymbol(route.Args[1])
			if !ok {
				continue
			}
			binder := goNativePublicCallbackSymbol(route.Args[2])
			validator := goNativePublicCallbackSymbol(route.Args[3])
			handler := goNativePublicCallbackSymbol(route.Args[4])
			if binder != "" && validator != "" && handler != "" {
				rows = append(rows, scopedRuntimeOwnerRow(command, mode, resolver, binder, validator, handler))
			}
		case "gateScopedCommandRoute":
			if len(route.Args) != 1 {
				continue
			}
			mode, ok := goNativePublicModeSymbol(route.Args[0])
			if ok {
				rows = append(rows, scopedRuntimeOwnerRow(command, mode, resolver, "bindGateCommand", "validateGateCommand", "handleGateCommand"))
			}
		}
	}
	return rows
}

func scopedRuntimeOwnerRow(command, mode, resolver, binder, validator, handler string) GoNativePublicRuntimeOwner {
	return GoNativePublicRuntimeOwner{
		Command: command, Mode: mode, OwnerKind: "scoped-runtime-owner",
		Resolver: resolver, Binder: binder, Validator: validator, Handler: handler,
		PublicationOwner: handler,
		CallPath:         "runWithOptions->runOwnedCommand->runScopedCommand->executeScopedCommandRoute->" + handler,
	}
}

func goNativePublicPreRuntimeOwnerRows(expression ast.Expr, symbols map[string]string) []GoNativePublicRuntimeOwner {
	catalog, ok := expression.(*ast.CompositeLit)
	if !ok {
		return nil
	}
	rows := []GoNativePublicRuntimeOwner{}
	for _, element := range catalog.Elts {
		owner, ok := element.(*ast.CompositeLit)
		if !ok {
			continue
		}
		var name, mode, selector, validator, handler string
		var scopes ast.Expr
		for _, element := range owner.Elts {
			field, ok := element.(*ast.KeyValueExpr)
			if !ok {
				continue
			}
			key, ok := field.Key.(*ast.Ident)
			if !ok {
				continue
			}
			switch key.Name {
			case "Name":
				name = goNativePublicStringLiteral(field.Value)
			case "Mode":
				mode, _ = goNativePublicModeSymbol(field.Value)
				if mode == "" {
					mode = goNativePublicStringLiteral(field.Value)
				}
			case "Scopes":
				scopes = field.Value
			case "Select":
				selector = goNativePublicCallbackSymbol(field.Value)
			case "Validate":
				validator = goNativePublicCallbackSymbol(field.Value)
			case "Handle":
				handler = goNativePublicCallbackSymbol(field.Value)
			}
		}
		if name == "" || mode == "" || selector == "" || validator == "" || handler == "" {
			continue
		}
		if scopeRows := goNativePublicPreRuntimeScopes(scopes, symbols); len(scopeRows) > 0 {
			for _, scope := range scopeRows {
				rows = append(rows, GoNativePublicRuntimeOwner{
					Command: scope.Command, Mode: scope.Mode, OwnerKind: "pre-runtime-exclusive-owner",
					Resolver: selector, Validator: validator, Handler: handler,
					PublicationOwner: handler,
					CallPath:         "runWithOptions->runPreRuntimeCommand->" + handler,
				})
			}
			continue
		}
		rows = append(rows, GoNativePublicRuntimeOwner{
			Command: "*", Mode: mode, OwnerKind: "pre-runtime-interceptor-owner",
			Resolver: selector, Validator: validator, Handler: handler,
			PublicationOwner: handler,
			CallPath:         "runWithOptions->runPreRuntimeCommand->" + handler,
		})
	}
	return rows
}

func goNativePublicPreRuntimeScopes(expression ast.Expr, symbols map[string]string) []commands.CommandScope {
	catalog, ok := expression.(*ast.CompositeLit)
	if !ok {
		return nil
	}
	rows := []commands.CommandScope{}
	for _, element := range catalog.Elts {
		scope, ok := element.(*ast.CompositeLit)
		if !ok {
			continue
		}
		var command, mode string
		for _, element := range scope.Elts {
			field, ok := element.(*ast.KeyValueExpr)
			if !ok {
				continue
			}
			key, ok := field.Key.(*ast.Ident)
			if !ok {
				continue
			}
			switch key.Name {
			case "Command":
				command, _ = goNativePublicCommandSymbol(field.Value, symbols)
			case "Mode":
				mode, _ = goNativePublicModeSymbol(field.Value)
			}
		}
		if command != "" && mode != "" {
			rows = append(rows, commands.CommandScope{Command: command, Mode: mode})
		}
	}
	return rows
}

func goNativePublicCallbackSymbol(expression ast.Expr) string {
	switch value := expression.(type) {
	case *ast.Ident:
		if value.Name != "nil" {
			return value.Name
		}
	case *ast.FuncLit:
		return "inline-func"
	}
	return ""
}

func goNativePublicModeSymbol(expression ast.Expr) (string, bool) {
	selector, ok := expression.(*ast.SelectorExpr)
	if !ok {
		return "", false
	}
	pkg, ok := selector.X.(*ast.Ident)
	if !ok || pkg.Name != "commands" {
		return "", false
	}
	mode, ok := goNativePublicModeSymbols()[selector.Sel.Name]
	return mode, ok
}

func goNativePublicModeSymbols() map[string]string {
	return map[string]string{
		"MutationModeDefault":                        commands.MutationModeDefault,
		"MutationModeOrdinaryOnboarding":             commands.MutationModeOrdinaryOnboarding,
		"MutationModeAttachedAdoption":               commands.MutationModeAttachedAdoption,
		"MutationModeDirectAppend":                   commands.MutationModeDirectAppend,
		"MutationModeCurrentSync":                    commands.MutationModeCurrentSync,
		"MutationModeSelectedPackMemorySync":         commands.MutationModeSelectedPackMemorySync,
		"MutationModePackMemoryConsumerVerification": commands.MutationModePackMemoryConsumerVerification,
		"MutationModeOrdinarySync":                   commands.MutationModeOrdinarySync,
		"MutationModeMemberOutputStaging":            commands.MutationModeMemberOutputStaging,
		"MutationModeCandidateRetirement":            commands.MutationModeCandidateRetirement,
		"MutationModeCandidateProvision":             commands.MutationModeCandidateProvision,
		"MutationModeCandidateVerification":          commands.MutationModeCandidateVerification,
		"MutationModeReviewProofDraft":               commands.MutationModeReviewProofDraft,
		"MutationModeCandidateDecisionDraft":         commands.MutationModeCandidateDecisionDraft,
		"MutationModeCandidateDecisionApply":         commands.MutationModeCandidateDecisionApply,
		"MutationModeOrdinaryPromote":                commands.MutationModeOrdinaryPromote,
		"MutationModeCreateCandidates":               commands.MutationModeCreateCandidates,
		"MutationModeRecordReviewerDispatch":         commands.MutationModeRecordReviewerDispatch,
		"MutationModeRecordReviewerCompletion":       commands.MutationModeRecordReviewerCompletion,
		"MutationModeSaveReviewerResultInput":        commands.MutationModeSaveReviewerResultInput,
		"MutationModeCaptureReviewerResultSource":    commands.MutationModeCaptureReviewerResultSource,
		"MutationModeStageReviewerResult":            commands.MutationModeStageReviewerResult,
		"MutationModeCollectReviewerResult":          commands.MutationModeCollectReviewerResult,
		"MutationModeRetireReviewerRecovery":         commands.MutationModeRetireReviewerRecovery,
		"MutationModeRetireReviewerPacket":           commands.MutationModeRetireReviewerPacket,
		"MutationModeRecoverReviewerResult":          commands.MutationModeRecoverReviewerResult,
		"MutationModeRepairReviewerPrompt":           commands.MutationModeRepairReviewerPrompt,
		"MutationModeAdoptReviewerPacket":            commands.MutationModeAdoptReviewerPacket,
		"MutationModeReviewerBatchIntake":            commands.MutationModeReviewerBatchIntake,
		"MutationModeReviewerIntake":                 commands.MutationModeReviewerIntake,
		"MutationModePlanArtifacts":                  commands.MutationModePlanArtifacts,
		"MutationModeProfile":                        commands.MutationModeProfile,
		"MutationModeAdapterDispatch":                commands.MutationModeAdapterDispatch,
		"MutationModeAdapterReceipt":                 commands.MutationModeAdapterReceipt,
		"MutationModeReportScaffold":                 commands.MutationModeReportScaffold,
		"MutationModeReportDraft":                    commands.MutationModeReportDraft,
		"MutationModeDecision":                       commands.MutationModeDecision,
		"MutationModeExecutionObservation":           commands.MutationModeExecutionObservation,
		"MutationModeEventAppend":                    commands.MutationModeEventAppend,
		"MutationModeBoardBootstrap":                 commands.MutationModeBoardBootstrap,
		"MutationModeValidationReceipt":              commands.MutationModeValidationReceipt,
		"MutationModeExternalAttempt":                commands.MutationModeExternalAttempt,
		"MutationModeExternalDispatch":               commands.MutationModeExternalDispatch,
		"MutationModeExternalTurn":                   commands.MutationModeExternalTurn,
		"MutationModeExternalRelay":                  commands.MutationModeExternalRelay,
	}
}

func goNativePublicStringLiteral(expression ast.Expr) string {
	literal, ok := expression.(*ast.BasicLit)
	if !ok || literal.Kind != token.STRING || len(literal.Value) < 2 {
		return ""
	}
	return strings.Trim(literal.Value, "\"")
}

func mustScopedCommandDescriptors() []commands.ScopedCommandDescriptor {
	descriptors, err := commands.ScopedCommandDescriptors()
	if err != nil {
		return nil
	}
	return descriptors
}

func goNativePublicScopedHandlerCommands(path string, symbols map[string]string, seen map[string]bool) {
	file, err := parser.ParseFile(token.NewFileSet(), path, nil, 0)
	if err != nil {
		return
	}
	for _, declaration := range file.Decls {
		general, ok := declaration.(*ast.GenDecl)
		if !ok || general.Tok != token.VAR {
			continue
		}
		for _, specification := range general.Specs {
			value, ok := specification.(*ast.ValueSpec)
			if !ok {
				continue
			}
			for index, name := range value.Names {
				if index >= len(value.Values) {
					continue
				}
				switch name.Name {
				case "scopedCommandRoutes":
					goNativePublicScopedRoutesValue(value.Values[index], symbols, seen)
				case "scopedCommandRuntimeOwners":
					goNativePublicScopedRuntimeOwnersValue(value.Values[index], symbols, seen)
				case "directCommandRuntimeOwners":
					goNativePublicDirectRuntimeOwnersValue(value.Values[index], symbols, seen)
				}
			}
		}
	}
}

func goNativePublicScopedRoutesValue(expression ast.Expr, symbols map[string]string, seen map[string]bool) {
	catalog, ok := expression.(*ast.CompositeLit)
	if !ok {
		return
	}
	for _, element := range catalog.Elts {
		route, ok := element.(*ast.CompositeLit)
		if !ok {
			continue
		}
		command, complete := goNativePublicScopedRouteCommand(route, symbols)
		if complete {
			seen[command] = true
		}
	}
}

func goNativePublicScopedRuntimeOwnersValue(expression ast.Expr, symbols map[string]string, seen map[string]bool) {
	catalog, ok := expression.(*ast.CompositeLit)
	if !ok {
		return
	}
	for _, element := range catalog.Elts {
		switch owner := element.(type) {
		case *ast.CallExpr:
			goNativePublicScopedDefaultOwnerCall(owner, symbols, seen)
		case *ast.CompositeLit:
			goNativePublicScopedOwnerLiteral(owner, symbols, seen)
		}
	}
}

func goNativePublicDirectRuntimeOwnersValue(expression ast.Expr, symbols map[string]string, seen map[string]bool) {
	catalog, ok := expression.(*ast.CompositeLit)
	if !ok {
		return
	}
	for _, element := range catalog.Elts {
		owner, ok := element.(*ast.CompositeLit)
		if !ok {
			continue
		}
		var command string
		handlePresent := false
		if len(owner.Elts) >= 2 {
			command, _ = goNativePublicCommandSymbol(owner.Elts[0], symbols)
			handlePresent = !goNativePublicNilExpression(owner.Elts[1])
		}
		for _, element := range owner.Elts {
			field, ok := element.(*ast.KeyValueExpr)
			if !ok {
				continue
			}
			key, ok := field.Key.(*ast.Ident)
			if !ok {
				continue
			}
			switch key.Name {
			case "Command":
				command, _ = goNativePublicCommandSymbol(field.Value, symbols)
			case "Handle":
				handlePresent = !goNativePublicNilExpression(field.Value)
			}
		}
		if command != "" && handlePresent {
			seen[command] = true
		}
	}
}

func goNativePublicScopedDefaultOwnerCall(call *ast.CallExpr, symbols map[string]string, seen map[string]bool) {
	function, ok := call.Fun.(*ast.Ident)
	if !ok {
		return
	}
	var callbacks []ast.Expr
	switch function.Name {
	case "defaultScopedCommandRuntimeOwner":
		if len(call.Args) < 4 {
			return
		}
		callbacks = call.Args[1:4]
	case "descriptorScopedCommandRuntimeOwner":
		if len(call.Args) < 5 {
			return
		}
		callbacks = call.Args[1:5]
	case "fixedScopedCommandRuntimeOwner":
		if len(call.Args) < 3 {
			return
		}
		callbacks = call.Args[2:3]
	default:
		return
	}
	command, ok := goNativePublicCommandSymbol(call.Args[0], symbols)
	if !ok {
		return
	}
	for _, callback := range callbacks {
		if goNativePublicNilExpression(callback) {
			return
		}
	}
	seen[command] = true
}

func goNativePublicScopedOwnerLiteral(owner *ast.CompositeLit, symbols map[string]string, seen map[string]bool) {
	var command string
	var resolver ast.Expr
	var routes ast.Expr
	for _, element := range owner.Elts {
		field, ok := element.(*ast.KeyValueExpr)
		if !ok {
			continue
		}
		key, ok := field.Key.(*ast.Ident)
		if !ok {
			continue
		}
		switch key.Name {
		case "Command":
			command, _ = goNativePublicCommandSymbol(field.Value, symbols)
		case "ResolveMode":
			resolver = field.Value
		case "Routes":
			routes = field.Value
		}
	}
	if command != "" && !goNativePublicNilExpression(resolver) && goNativePublicScopedOwnerRoutesComplete(routes, symbols) {
		seen[command] = true
	}
}

func goNativePublicScopedOwnerRoutesComplete(expression ast.Expr, symbols map[string]string) bool {
	catalog, ok := expression.(*ast.CompositeLit)
	if !ok || len(catalog.Elts) == 0 {
		return false
	}
	for _, element := range catalog.Elts {
		switch route := element.(type) {
		case *ast.CallExpr:
			function, ok := route.Fun.(*ast.Ident)
			if !ok {
				return false
			}
			switch function.Name {
			case "gateScopedCommandRoute":
				if len(route.Args) != 1 {
					return false
				}
			case "ownedScopedCommandRoute":
				if len(route.Args) != 5 {
					return false
				}
				for _, callback := range route.Args[2:] {
					if goNativePublicNilExpression(callback) {
						return false
					}
				}
			default:
				return false
			}
		case *ast.CompositeLit:
			if _, complete := goNativePublicScopedRouteCommand(route, symbols); !complete {
				return false
			}
		default:
			return false
		}
	}
	return true
}

func goNativePublicScopedRouteCommand(route *ast.CompositeLit, symbols map[string]string) (string, bool) {
	var command string
	required := map[string]bool{"Bind": false, "Validate": false, "Handle": false}
	for _, element := range route.Elts {
		field, ok := element.(*ast.KeyValueExpr)
		if !ok {
			continue
		}
		key, ok := field.Key.(*ast.Ident)
		if !ok {
			continue
		}
		if _, ok := required[key.Name]; ok {
			required[key.Name] = !goNativePublicNilExpression(field.Value)
			continue
		}
		if key.Name != "Scope" {
			continue
		}
		scope, ok := field.Value.(*ast.CompositeLit)
		if !ok {
			continue
		}
		for _, scopeElement := range scope.Elts {
			scopeField, ok := scopeElement.(*ast.KeyValueExpr)
			if !ok {
				continue
			}
			scopeKey, ok := scopeField.Key.(*ast.Ident)
			if !ok || scopeKey.Name != "Command" {
				continue
			}
			command, _ = goNativePublicCommandSymbol(scopeField.Value, symbols)
		}
	}
	return command, command != "" && required["Bind"] && required["Validate"] && required["Handle"]
}

func goNativePublicNilExpression(expression ast.Expr) bool {
	identifier, ok := expression.(*ast.Ident)
	return ok && identifier.Name == "nil"
}

func goNativePublicCommandSymbol(expression ast.Expr, symbols map[string]string) (string, bool) {
	selector, ok := expression.(*ast.SelectorExpr)
	if !ok {
		return "", false
	}
	pkg, ok := selector.X.(*ast.Ident)
	if !ok || pkg.Name != "commands" {
		return "", false
	}
	if command, ok := symbols[selector.Sel.Name]; ok {
		return command, true
	}
	return selector.Sel.Name, selector.Sel.Name != ""
}

func goNativePublicSurfaceCrossWarnings(goSurface GoNativePublicSurface, facade PowerShellPublicFacade) []string {
	warnings := []string{}
	for _, command := range facade.CommandSurface {
		if !slices.Contains(goSurface.Commands, command) {
			warnings = append(warnings, fmt.Sprintf("PowerShell public facade command missing from Go-native public command catalog: %s", command))
		}
	}
	for _, command := range goSurface.Commands {
		if command == commands.Onboard {
			continue
		}
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
