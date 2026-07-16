package commands

import (
	"fmt"
	"slices"
	"sort"
	"strings"
)

const (
	Attach        = "attach"
	Bootstrap     = "bootstrap"
	Continue      = "continue"
	Doctor        = "doctor"
	Gate          = "gate"
	Handoff       = "handoff"
	Init          = "init"
	Note          = "note"
	Overview      = "overview"
	Packs         = "packs"
	PlanSubagents = "plan-subagents"
	Promote       = "promote"
	ReleaseCheck  = "release-check"
	Repair        = "repair"
	Start         = "start"
	Status        = "status"
	Sync          = "sync"
	Update        = "update"
	Validate      = "validate"
)

const (
	DefaultCommand             = Status
	GoNativeEntrypoint         = "cmd/rekit"
	GoNativeAlternativePattern = "go run ./cmd/rekit -- -Command <command>"
)

const (
	BoundaryReadOnly                 = "read-only"
	BoundaryCaseLocalAppend          = "case-local-append"
	BoundaryCaseLocalApply           = "case-local-apply"
	BoundaryCaseLocalReadOrBootstrap = "case-local-read-or-bootstrap"
	BoundaryCaseLocalReviewArtifact  = "case-local-review-artifact"
	BoundaryCaseLocalReviewFirst     = "case-local-review-first"
	BoundaryKitReviewFirst           = "kit-review-first"
)

type PublicProfile struct {
	Command            string `json:"command"`
	MutationBoundary   string `json:"mutationBoundary"`
	IsMutation         bool   `json:"isMutation"`
	WritesCase         bool   `json:"writesCase"`
	WritesKit          bool   `json:"writesKit"`
	ReviewFirst        bool   `json:"reviewFirst"`
	ApplyRequired      bool   `json:"applyRequired"`
	HeavyTool          bool   `json:"heavyTool"`
	AuthorityConfirmed bool   `json:"authorityConfirmed"`
}

type PublicProfileSummary struct {
	Total              int            `json:"total"`
	ReadOnly           int            `json:"readOnly"`
	Mutating           int            `json:"mutating"`
	WritesCase         int            `json:"writesCase"`
	WritesKit          int            `json:"writesKit"`
	ReviewFirst        int            `json:"reviewFirst"`
	ApplyRequired      int            `json:"applyRequired"`
	HeavyTool          int            `json:"heavyTool"`
	AuthorityConfirmed int            `json:"authorityConfirmed"`
	Boundaries         map[string]int `json:"boundaries"`
}

var publicCommands = []string{
	Attach,
	Bootstrap,
	Continue,
	Doctor,
	Gate,
	Handoff,
	Init,
	Note,
	Overview,
	Packs,
	PlanSubagents,
	Promote,
	ReleaseCheck,
	Repair,
	Start,
	Status,
	Sync,
	Update,
	Validate,
}

var publicProfiles = []PublicProfile{
	{Command: Attach, MutationBoundary: BoundaryCaseLocalApply, IsMutation: true, WritesCase: true, ApplyRequired: true},
	{Command: Bootstrap, MutationBoundary: BoundaryCaseLocalApply, IsMutation: true, WritesCase: true, ApplyRequired: true},
	{Command: Continue, MutationBoundary: BoundaryCaseLocalApply, IsMutation: true, WritesCase: true, ApplyRequired: true},
	{Command: Doctor, MutationBoundary: BoundaryReadOnly},
	{Command: Gate, MutationBoundary: BoundaryCaseLocalApply, IsMutation: true, WritesCase: true, ApplyRequired: true},
	{Command: Handoff, MutationBoundary: BoundaryCaseLocalApply, IsMutation: true, WritesCase: true, ApplyRequired: true},
	{Command: Init, MutationBoundary: BoundaryCaseLocalApply, IsMutation: true, WritesCase: true, ApplyRequired: true},
	{Command: Note, MutationBoundary: BoundaryCaseLocalAppend, IsMutation: true, WritesCase: true},
	{Command: Overview, MutationBoundary: BoundaryCaseLocalReadOrBootstrap, IsMutation: true, WritesCase: true},
	{Command: Packs, MutationBoundary: BoundaryReadOnly},
	{Command: PlanSubagents, MutationBoundary: BoundaryCaseLocalReviewArtifact, IsMutation: true, WritesCase: true},
	{Command: Promote, MutationBoundary: BoundaryKitReviewFirst, IsMutation: true, WritesKit: true, ReviewFirst: true, ApplyRequired: true},
	{Command: ReleaseCheck, MutationBoundary: BoundaryReadOnly},
	{Command: Repair, MutationBoundary: BoundaryCaseLocalApply, IsMutation: true, WritesCase: true, ApplyRequired: true},
	{Command: Start, MutationBoundary: BoundaryCaseLocalApply, IsMutation: true, WritesCase: true, ApplyRequired: true},
	{Command: Status, MutationBoundary: BoundaryReadOnly},
	{Command: Sync, MutationBoundary: BoundaryCaseLocalReviewFirst, IsMutation: true, WritesCase: true, ReviewFirst: true, ApplyRequired: true},
	{Command: Update, MutationBoundary: BoundaryCaseLocalReviewFirst, IsMutation: true, WritesCase: true, ReviewFirst: true, ApplyRequired: true},
	{Command: Validate, MutationBoundary: BoundaryReadOnly},
}

func Public() []string {
	out := append([]string{}, publicCommands...)
	sort.Strings(out)
	return out
}

func PublicSet() map[string]bool {
	set := map[string]bool{}
	for _, command := range publicCommands {
		set[command] = true
	}
	return set
}

func SymbolValues() map[string]string {
	return map[string]string{
		"Attach":        Attach,
		"Bootstrap":     Bootstrap,
		"Continue":      Continue,
		"Doctor":        Doctor,
		"Gate":          Gate,
		"Handoff":       Handoff,
		"Init":          Init,
		"Note":          Note,
		"Overview":      Overview,
		"Packs":         Packs,
		"PlanSubagents": PlanSubagents,
		"Promote":       Promote,
		"ReleaseCheck":  ReleaseCheck,
		"Repair":        Repair,
		"Start":         Start,
		"Status":        Status,
		"Sync":          Sync,
		"Update":        Update,
		"Validate":      Validate,
	}
}

func PublicProfiles() []PublicProfile {
	out := append([]PublicProfile{}, publicProfiles...)
	sort.Slice(out, func(i, j int) bool { return out[i].Command < out[j].Command })
	return out
}

func PublicProfileMap() map[string]PublicProfile {
	profiles := map[string]PublicProfile{}
	for _, profile := range publicProfiles {
		profiles[profile.Command] = profile
	}
	return profiles
}

func PublicProfileCommands(profiles []PublicProfile) []string {
	out := []string{}
	seen := map[string]bool{}
	for _, profile := range profiles {
		command := strings.TrimSpace(profile.Command)
		if command == "" || seen[command] {
			continue
		}
		seen[command] = true
		out = append(out, command)
	}
	sort.Strings(out)
	return out
}

func PublicProfileSummaryFor(profiles []PublicProfile) PublicProfileSummary {
	summary := PublicProfileSummary{Boundaries: map[string]int{}}
	for _, profile := range profiles {
		summary.Total++
		summary.Boundaries[profile.MutationBoundary]++
		if profile.MutationBoundary == BoundaryReadOnly {
			summary.ReadOnly++
		}
		if profile.IsMutation {
			summary.Mutating++
		}
		if profile.WritesCase {
			summary.WritesCase++
		}
		if profile.WritesKit {
			summary.WritesKit++
		}
		if profile.ReviewFirst {
			summary.ReviewFirst++
		}
		if profile.ApplyRequired {
			summary.ApplyRequired++
		}
		if profile.HeavyTool {
			summary.HeavyTool++
		}
		if profile.AuthorityConfirmed {
			summary.AuthorityConfirmed++
		}
	}
	return summary
}

func PublicProfileSummaryBaseline() PublicProfileSummary {
	return PublicProfileSummaryFor(publicProfiles)
}

func KnownMutationBoundaries() []string {
	return []string{
		BoundaryCaseLocalAppend,
		BoundaryCaseLocalApply,
		BoundaryCaseLocalReadOrBootstrap,
		BoundaryCaseLocalReviewArtifact,
		BoundaryCaseLocalReviewFirst,
		BoundaryKitReviewFirst,
		BoundaryReadOnly,
	}
}

func IsKnownMutationBoundary(name string) bool {
	return slices.Contains(KnownMutationBoundaries(), strings.TrimSpace(name))
}

func IsPublic(name string) bool {
	return PublicSet()[strings.ToLower(strings.TrimSpace(name))]
}

func SupportedList() string {
	return strings.Join(Public(), ", ")
}

func MissingPublicHandlers(handlerNames []string) []string {
	missing := []string{}
	for _, command := range Public() {
		if !slices.Contains(handlerNames, command) {
			missing = append(missing, command)
		}
	}
	return missing
}

func UnknownPublicHandlers(handlerNames []string) []string {
	unknown := []string{}
	public := PublicSet()
	seen := map[string]bool{}
	for _, name := range handlerNames {
		name = strings.TrimSpace(name)
		if name == "" || seen[name] {
			continue
		}
		seen[name] = true
		if !public[name] {
			unknown = append(unknown, name)
		}
	}
	sort.Strings(unknown)
	return unknown
}

func AlternativeFor(name string) string {
	name = strings.TrimSpace(name)
	if name == "" {
		name = "<command>"
	}
	return "go run ./cmd/rekit -- -Command " + name
}

func UnsupportedError(name string) error {
	name = strings.TrimSpace(name)
	if name == "" {
		name = "<empty>"
	}
	return fmt.Errorf("go backend does not implement public command %q; supported commands: %s; use %s", name, SupportedList(), AlternativeFor("<command>"))
}
