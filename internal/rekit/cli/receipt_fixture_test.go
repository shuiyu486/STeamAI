package cli

import (
	"testing"

	"github.com/shuiyu486/re-context-kits/internal/rekit/mission"
	"github.com/shuiyu486/re-context-kits/internal/rekit/releasecheck"
)

func withProjectHandoffFixture(t *testing.T, handoff releasecheck.ReleaseHandoff) {
	t.Helper()
	previous := projectHandoffBuild
	projectHandoffBuild = func(string) (releasecheck.ReleaseHandoff, error) {
		return handoff, nil
	}
	t.Cleanup(func() { projectHandoffBuild = previous })
}

func withReleaseCheckResultFixture(t *testing.T, result releasecheck.Result) {
	t.Helper()
	previous := releaseCheckBuild
	releaseCheckBuild = func(string) (releasecheck.Result, error) {
		return result, nil
	}
	t.Cleanup(func() { releaseCheckBuild = previous })
}

func releaseHandoffWithoutActiveRouteReceiptFixture() releasecheck.ReleaseHandoff {
	handoff := readyReleaseHandoffFixture(releasecheck.ReleaseHandoff{
		ReadFirst: []releasecheck.ReleaseHandoffDocument{{Path: "docs/context-routing.md"}},
		Validation: []releasecheck.ReleaseHandoffValidation{
			{Command: "go run ./cmd/rekit -- -Command release-check -Format json", Required: true, Present: true, Resolved: true},
			{Command: releasecheck.CanonicalGoTestCommand, Required: true, Present: true, Resolved: true},
			{Command: "go vet ./...", Required: true, Present: true, Resolved: true},
			{Command: "git diff --check", Required: true, Present: true, Resolved: true},
		},
	})
	action := &mission.MissionCommanderNextActionItem{
		Label:          "fixture-route-closure",
		ActionID:       "active-route-local-validation",
		State:          "active-route-validation-required",
		Command:        "/rekit release-run -Format json",
		Source:         "releaseHandoffActiveRoute.localValidationReceipt",
		RequiresReview: true,
		Reasons:        []string{"fixture active route requires a Git-local validation receipt"},
		Boundary:       []string{"fixture does not read checkout Git metadata"},
	}
	handoff.ActiveRoute = releasecheck.ReleaseHandoffActiveRoute{
		Ready:                true,
		Present:              true,
		Path:                 "docs/real-usage-hardening-roadmap.md",
		ProjectionPath:       "docs/batch-plan.md",
		Route:                "fixture-active-route-v1",
		CurrentBatch:         "fixture-route-closure",
		State:                "completed",
		ExclusiveClaim:       "fixture-route-closure",
		NextBatch:            "无；fixture route complete",
		NextBatchUnlocked:    false,
		ProjectionConsistent: true,
		LocalValidationReady: false,
		ReleaseCheckReady:    false,
		CurrentAction:        action,
	}
	handoff.NextBatchSelectionPackage = nil
	handoff.LatestBatch.Handoff.ReleaseInspectionCadence.NextAction = ""
	handoff.KnownGaps = []releasecheck.ReleaseHandoffKnownGap{{Index: 1, Category: "fixture", Summary: "远程 release-gate fixture known gap"}}
	return handoff
}
