package commands

import (
	"slices"
	"strings"
	"testing"
)

func TestPublicCommandCatalog(t *testing.T) {
	commands := Public()
	if len(commands) != 30 || !slices.IsSorted(commands) {
		t.Fatalf("unexpected public command catalog: %v", commands)
	}
	for _, command := range []string{"attach", "bootstrap", "complete", "continue", "doctor", "gate", "handoff", "init", "next-batch", "note", "onboard", "overview", "packs", "plan-subagents", "promote", "reconcile", "reopen", "release-check", "release-run", "repair", "run-current-loop", "run-current-step", "run-driver-step", "run-reviewer-step", "run-reviewer-wave", "start", "status", "sync", "update", "validate"} {
		if !slices.Contains(commands, command) || !IsPublic(command) || !IsPublic(" "+command+" ") {
			t.Fatalf("public command %s missing or not recognized: %v", command, commands)
		}
	}
	for _, blocked := range []string{"debug", "dump", "network", "authority", "confirmed", ""} {
		if IsPublic(blocked) {
			t.Fatalf("blocked command %q must not be public", blocked)
		}
	}
}

func TestPublicCommandProfiles(t *testing.T) {
	profiles := PublicProfiles()
	if len(profiles) != len(Public()) || !slices.IsSortedFunc(profiles, func(a, b PublicProfile) int { return strings.Compare(a.Command, b.Command) }) {
		t.Fatalf("unexpected public command profiles: %+v", profiles)
	}
	profileMap := PublicProfileMap()
	for _, command := range Public() {
		profile, ok := profileMap[command]
		if !ok || profile.Command != command || !IsKnownMutationBoundary(profile.MutationBoundary) || profile.HeavyTool || profile.AuthorityConfirmed {
			t.Fatalf("invalid public command profile for %s: %+v", command, profile)
		}
	}
	for _, command := range []string{ReleaseCheck, Status, Packs, Doctor, Validate} {
		profile := profileMap[command]
		if profile.IsMutation || profile.WritesCase || profile.WritesKit || profile.ApplyRequired || profile.ReviewFirst || profile.MutationBoundary != BoundaryReadOnly {
			t.Fatalf("read-only command %s has mutating profile: %+v", command, profile)
		}
	}
	releaseRun := profileMap[ReleaseRun]
	if !releaseRun.IsMutation || releaseRun.WritesCase || releaseRun.WritesKit || releaseRun.ApplyRequired || releaseRun.ReviewFirst || releaseRun.MutationBoundary != BoundaryLocalValidationReceipt {
		t.Fatalf("release-run Git-local receipt profile drifted: %+v", releaseRun)
	}
	for _, command := range []string{Complete, Reopen, Onboard, NextBatch, Sync, Update, Promote, RunCurrentLoop, RunCurrentStep, RunDriverStep, RunReviewerStep, RunReviewerWave} {
		profile := profileMap[command]
		if !profile.IsMutation || !profile.ReviewFirst || !profile.ApplyRequired {
			t.Fatalf("review-first command %s missing mutation guards: %+v", command, profile)
		}
	}
	if profileMap[Promote].WritesCase || !profileMap[Promote].WritesKit || !profileMap[NextBatch].WritesKit || profileMap[NextBatch].WritesCase || !profileMap[Sync].WritesCase || profileMap[Sync].WritesKit {
		t.Fatalf("unexpected kit/case write boundaries: next-batch=%+v promote=%+v sync=%+v", profileMap[NextBatch], profileMap[Promote], profileMap[Sync])
	}
	summary := PublicProfileSummaryBaseline()
	if summary.Total != 30 || summary.ReadOnly != 5 || summary.Mutating != 25 || summary.WritesCase != 22 || summary.WritesKit != 2 || summary.ReviewFirst != 12 || summary.ApplyRequired != 22 || summary.HeavyTool != 0 || summary.AuthorityConfirmed != 0 || summary.Boundaries[BoundaryCaseLocalApply] != 9 || summary.Boundaries[BoundaryCaseLocalReviewWriteback] != 1 || summary.Boundaries[BoundaryCaseLocalReviewFirst] != 10 || summary.Boundaries[BoundaryKitReviewFirst] != 2 || summary.Boundaries[BoundaryLocalValidationReceipt] != 1 || summary.Boundaries[BoundaryReadOnly] != 5 {
		t.Fatalf("unexpected public command profile summary: %+v", summary)
	}
	groups := PublicProfileGroupsBaseline()
	if strings.Join(groups.ReadOnly, ",") != "doctor,packs,release-check,status,validate" || strings.Join(groups.WritesKit, ",") != "next-batch,promote" || strings.Join(groups.ReviewFirst, ",") != "complete,next-batch,onboard,promote,reopen,run-current-loop,run-current-step,run-driver-step,run-reviewer-step,run-reviewer-wave,sync,update" || len(groups.HeavyTool) != 0 || len(groups.AuthorityConfirmed) != 0 || len(groups.ByBoundary[BoundaryCaseLocalApply]) != 9 || strings.Join(groups.ByBoundary[BoundaryCaseLocalReviewWriteback], ",") != PlanSubagents || strings.Join(groups.ByBoundary[BoundaryCaseLocalReviewFirst], ",") != "complete,onboard,reopen,run-current-loop,run-current-step,run-driver-step,run-reviewer-step,run-reviewer-wave,sync,update" || strings.Join(groups.ByBoundary[BoundaryKitReviewFirst], ",") != "next-batch,promote" || strings.Join(groups.ByBoundary[BoundaryLocalValidationReceipt], ",") != ReleaseRun {
		t.Fatalf("unexpected public command profile groups: %+v", groups)
	}
	boundaries := PublicProfileBoundariesBaseline()
	if len(boundaries) != len(KnownMutationBoundaries()) || boundaries[0].Boundary != BoundaryCaseLocalAppend || boundaries[0].Count != 1 || strings.Join(boundaries[1].Commands, ",") != "attach,bootstrap,continue,gate,handoff,init,reconcile,repair,start" || !slices.ContainsFunc(boundaries, func(boundary PublicProfileBoundary) bool {
		return boundary.Boundary == BoundaryCaseLocalReviewWriteback && boundary.Count == 1 && strings.Join(boundary.Commands, ",") == PlanSubagents
	}) || boundaries[len(boundaries)-1].Boundary != BoundaryReadOnly || boundaries[len(boundaries)-1].Count != 5 {
		t.Fatalf("unexpected public command profile boundaries: %+v", boundaries)
	}
	policies := PublicProfilePoliciesBaseline()
	if len(policies) != 5 || PublicProfilePolicyViolationCount(policies) != 0 || policies[0].Policy != PublicProfilePolicyNoHeavyTool || !policies[0].Ready || policies[3].Policy != PublicProfilePolicyReviewFirstApplyRequired || len(policies[3].Commands) != 0 {
		t.Fatalf("unexpected public command profile policies: %+v", policies)
	}
}

func TestPublicCommandHandlerCoverageHelpers(t *testing.T) {
	symbols := SymbolValues()
	if len(symbols) != len(Public()) || symbols["Complete"] != "complete" || symbols["PlanSubagents"] != "plan-subagents" || symbols["ReleaseCheck"] != "release-check" || symbols["RunCurrentLoop"] != "run-current-loop" || symbols["RunCurrentStep"] != "run-current-step" || symbols["RunDriverStep"] != "run-driver-step" || symbols["RunReviewerStep"] != "run-reviewer-step" || symbols["RunReviewerWave"] != "run-reviewer-wave" {
		t.Fatalf("unexpected public command symbols: %+v", symbols)
	}
	if missing := MissingPublicHandlers([]string{"status", "packs", "unknown"}); !slices.Contains(missing, "release-check") || slices.Contains(missing, "unknown") {
		t.Fatalf("unexpected missing handler coverage: %v", missing)
	}
	if unknown := UnknownPublicHandlers([]string{" status ", "unknown", "packs", "unknown"}); len(unknown) != 1 || unknown[0] != "unknown" {
		t.Fatalf("unexpected unknown handler coverage: %v", unknown)
	}
}

func TestUnsupportedErrorNamesSupportedSurface(t *testing.T) {
	err := UnsupportedError("debug")
	if err == nil {
		t.Fatal("UnsupportedError returned nil")
	}
	message := err.Error()
	for _, expected := range []string{"debug", "supported commands:", "release-check", "status", GoNativeAlternativePattern} {
		if !strings.Contains(message, expected) {
			t.Fatalf("unsupported command error missing %q: %s", expected, message)
		}
	}
}
