package commands

import (
	"fmt"
	"slices"
	"sort"
	"strings"
)

const (
	Attach          = "attach"
	Bootstrap       = "bootstrap"
	Continue        = "continue"
	Doctor          = "doctor"
	Gate            = "gate"
	Handoff         = "handoff"
	Init            = "init"
	NextBatch       = "next-batch"
	Note            = "note"
	Overview        = "overview"
	Packs           = "packs"
	PlanSubagents   = "plan-subagents"
	Promote         = "promote"
	Reconcile       = "reconcile"
	ReleaseCheck    = "release-check"
	ReleaseRun      = "release-run"
	Repair          = "repair"
	RunCurrentLoop  = "run-current-loop"
	RunCurrentStep  = "run-current-step"
	RunDriverStep   = "run-driver-step"
	RunReviewerStep = "run-reviewer-step"
	Start           = "start"
	Status          = "status"
	Sync            = "sync"
	Update          = "update"
	Validate        = "validate"
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
	BoundaryCaseLocalReviewWriteback = "case-local-review-writeback"
	BoundaryCaseLocalReviewFirst     = "case-local-review-first"
	BoundaryKitReviewFirst           = "kit-review-first"
)

const (
	PublicProfilePolicyNoHeavyTool              = "no-heavy-tool"
	PublicProfilePolicyNoAuthorityConfirmed     = "no-authority-confirmed"
	PublicProfilePolicyKitWriteReviewFirst      = "kit-write-review-first"
	PublicProfilePolicyReviewFirstApplyRequired = "review-first-apply-required"
	PublicProfilePolicyKnownMutationBoundary    = "known-mutation-boundary"
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

type PublicProfileGroups struct {
	ReadOnly           []string            `json:"readOnly"`
	Mutating           []string            `json:"mutating"`
	WritesCase         []string            `json:"writesCase"`
	WritesKit          []string            `json:"writesKit"`
	ReviewFirst        []string            `json:"reviewFirst"`
	ApplyRequired      []string            `json:"applyRequired"`
	HeavyTool          []string            `json:"heavyTool"`
	AuthorityConfirmed []string            `json:"authorityConfirmed"`
	ByBoundary         map[string][]string `json:"byBoundary"`
}

type PublicProfileBoundary struct {
	Boundary string   `json:"boundary"`
	Count    int      `json:"count"`
	Commands []string `json:"commands"`
}

type PublicProfilePolicy struct {
	Policy         string   `json:"policy"`
	Ready          bool     `json:"ready"`
	Summary        string   `json:"summary"`
	ViolationCount int      `json:"violationCount"`
	Commands       []string `json:"commands"`
}

var publicCommands = []string{
	Attach,
	Bootstrap,
	Continue,
	Doctor,
	Gate,
	Handoff,
	Init,
	NextBatch,
	Note,
	Overview,
	Packs,
	PlanSubagents,
	Promote,
	Reconcile,
	ReleaseCheck,
	ReleaseRun,
	Repair,
	RunCurrentLoop,
	RunCurrentStep,
	RunDriverStep,
	RunReviewerStep,
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
	{Command: NextBatch, MutationBoundary: BoundaryKitReviewFirst, IsMutation: true, WritesKit: true, ReviewFirst: true, ApplyRequired: true},
	{Command: Note, MutationBoundary: BoundaryCaseLocalAppend, IsMutation: true, WritesCase: true},
	{Command: Overview, MutationBoundary: BoundaryCaseLocalReadOrBootstrap, IsMutation: true, WritesCase: true},
	{Command: Packs, MutationBoundary: BoundaryReadOnly},
	{Command: PlanSubagents, MutationBoundary: BoundaryCaseLocalReviewWriteback, IsMutation: true, WritesCase: true, ApplyRequired: true},
	{Command: Promote, MutationBoundary: BoundaryKitReviewFirst, IsMutation: true, WritesKit: true, ReviewFirst: true, ApplyRequired: true},
	{Command: Reconcile, MutationBoundary: BoundaryCaseLocalApply, IsMutation: true, WritesCase: true, ApplyRequired: true},
	{Command: ReleaseCheck, MutationBoundary: BoundaryReadOnly},
	{Command: ReleaseRun, MutationBoundary: BoundaryReadOnly},
	{Command: Repair, MutationBoundary: BoundaryCaseLocalApply, IsMutation: true, WritesCase: true, ApplyRequired: true},
	{Command: RunCurrentLoop, MutationBoundary: BoundaryCaseLocalReviewFirst, IsMutation: true, WritesCase: true, ReviewFirst: true, ApplyRequired: true},
	{Command: RunCurrentStep, MutationBoundary: BoundaryCaseLocalReviewFirst, IsMutation: true, WritesCase: true, ReviewFirst: true, ApplyRequired: true},
	{Command: RunDriverStep, MutationBoundary: BoundaryCaseLocalReviewFirst, IsMutation: true, WritesCase: true, ReviewFirst: true, ApplyRequired: true},
	{Command: RunReviewerStep, MutationBoundary: BoundaryCaseLocalReviewFirst, IsMutation: true, WritesCase: true, ReviewFirst: true, ApplyRequired: true},
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
		"Attach":          Attach,
		"Bootstrap":       Bootstrap,
		"Continue":        Continue,
		"Doctor":          Doctor,
		"Gate":            Gate,
		"Handoff":         Handoff,
		"Init":            Init,
		"NextBatch":       NextBatch,
		"Note":            Note,
		"Overview":        Overview,
		"Packs":           Packs,
		"PlanSubagents":   PlanSubagents,
		"Promote":         Promote,
		"Reconcile":       Reconcile,
		"ReleaseCheck":    ReleaseCheck,
		"ReleaseRun":      ReleaseRun,
		"Repair":          Repair,
		"RunCurrentLoop":  RunCurrentLoop,
		"RunCurrentStep":  RunCurrentStep,
		"RunDriverStep":   RunDriverStep,
		"RunReviewerStep": RunReviewerStep,
		"Start":           Start,
		"Status":          Status,
		"Sync":            Sync,
		"Update":          Update,
		"Validate":        Validate,
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

func PublicProfileGroupsFor(profiles []PublicProfile) PublicProfileGroups {
	groups := PublicProfileGroups{ByBoundary: map[string][]string{}}
	for _, boundary := range KnownMutationBoundaries() {
		groups.ByBoundary[boundary] = []string{}
	}
	for _, profile := range profiles {
		command := strings.TrimSpace(profile.Command)
		if command == "" {
			continue
		}
		groups.ByBoundary[profile.MutationBoundary] = append(groups.ByBoundary[profile.MutationBoundary], command)
		if profile.MutationBoundary == BoundaryReadOnly {
			groups.ReadOnly = append(groups.ReadOnly, command)
		}
		if profile.IsMutation {
			groups.Mutating = append(groups.Mutating, command)
		}
		if profile.WritesCase {
			groups.WritesCase = append(groups.WritesCase, command)
		}
		if profile.WritesKit {
			groups.WritesKit = append(groups.WritesKit, command)
		}
		if profile.ReviewFirst {
			groups.ReviewFirst = append(groups.ReviewFirst, command)
		}
		if profile.ApplyRequired {
			groups.ApplyRequired = append(groups.ApplyRequired, command)
		}
		if profile.HeavyTool {
			groups.HeavyTool = append(groups.HeavyTool, command)
		}
		if profile.AuthorityConfirmed {
			groups.AuthorityConfirmed = append(groups.AuthorityConfirmed, command)
		}
	}
	sort.Strings(groups.ReadOnly)
	sort.Strings(groups.Mutating)
	sort.Strings(groups.WritesCase)
	sort.Strings(groups.WritesKit)
	sort.Strings(groups.ReviewFirst)
	sort.Strings(groups.ApplyRequired)
	sort.Strings(groups.HeavyTool)
	sort.Strings(groups.AuthorityConfirmed)
	for boundary := range groups.ByBoundary {
		sort.Strings(groups.ByBoundary[boundary])
	}
	return groups
}

func PublicProfileGroupsBaseline() PublicProfileGroups {
	return PublicProfileGroupsFor(publicProfiles)
}

func PublicProfileBoundariesFor(groups PublicProfileGroups) []PublicProfileBoundary {
	boundaries := []PublicProfileBoundary{}
	for _, boundary := range KnownMutationBoundaries() {
		commands := append([]string{}, groups.ByBoundary[boundary]...)
		sort.Strings(commands)
		boundaries = append(boundaries, PublicProfileBoundary{
			Boundary: boundary,
			Count:    len(commands),
			Commands: commands,
		})
	}
	return boundaries
}

func PublicProfileBoundariesBaseline() []PublicProfileBoundary {
	return PublicProfileBoundariesFor(PublicProfileGroupsBaseline())
}

func PublicProfilePoliciesFor(profiles []PublicProfile) []PublicProfilePolicy {
	return []PublicProfilePolicy{
		publicProfilePolicy(PublicProfilePolicyNoHeavyTool, "public command profiles must not execute actual heavy tools", profiles, func(profile PublicProfile) bool { return profile.HeavyTool }),
		publicProfilePolicy(PublicProfilePolicyNoAuthorityConfirmed, "public command profiles must not write authority/confirmed state", profiles, func(profile PublicProfile) bool { return profile.AuthorityConfirmed }),
		publicProfilePolicy(PublicProfilePolicyKitWriteReviewFirst, "kit-writing public commands must be review-first", profiles, func(profile PublicProfile) bool { return profile.WritesKit && !profile.ReviewFirst }),
		publicProfilePolicy(PublicProfilePolicyReviewFirstApplyRequired, "review-first public commands must require apply", profiles, func(profile PublicProfile) bool { return profile.ReviewFirst && !profile.ApplyRequired }),
		publicProfilePolicy(PublicProfilePolicyKnownMutationBoundary, "public command profiles must use known mutation boundaries", profiles, func(profile PublicProfile) bool { return !IsKnownMutationBoundary(profile.MutationBoundary) }),
	}
}

func PublicProfilePoliciesBaseline() []PublicProfilePolicy {
	return PublicProfilePoliciesFor(publicProfiles)
}

func PublicProfilePolicyViolationCount(policies []PublicProfilePolicy) int {
	count := 0
	for _, policy := range policies {
		count += policy.ViolationCount
	}
	return count
}

func publicProfilePolicy(policy, summary string, profiles []PublicProfile, violates func(PublicProfile) bool) PublicProfilePolicy {
	commands := []string{}
	for _, profile := range profiles {
		command := strings.TrimSpace(profile.Command)
		if command == "" || !violates(profile) {
			continue
		}
		commands = append(commands, command)
	}
	sort.Strings(commands)
	return PublicProfilePolicy{
		Policy:         policy,
		Ready:          len(commands) == 0,
		Summary:        summary,
		ViolationCount: len(commands),
		Commands:       commands,
	}
}

func KnownMutationBoundaries() []string {
	return []string{
		BoundaryCaseLocalAppend,
		BoundaryCaseLocalApply,
		BoundaryCaseLocalReadOrBootstrap,
		BoundaryCaseLocalReviewWriteback,
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
