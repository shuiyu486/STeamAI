package releasecheck

import (
	"path/filepath"
	"testing"
)

func TestReleaseHandoffDetectsStaleReleaseNotes(t *testing.T) {
	repo := t.TempDir()
	writeReleaseHandoffFixture(t, repo, "Batch 999：Fixture", "Batch 998 old note")
	result, err := Build(repo)
	if err != nil {
		t.Fatal(err)
	}
	if result.ReleaseHandoff.ReleaseNotes.Covered || result.ReleaseHandoff.Ready || result.Ready {
		t.Fatalf("release handoff unexpectedly ready despite stale release notes: %+v", result.ReleaseHandoff)
	}
	assertWarningContains(t, result.ReleaseHandoff.Warnings, "release notes missing latest batch: Batch 999")
	assertWarningContains(t, result.Warnings, "release notes missing latest batch: Batch 999")
}

func TestBatchIDFromTitle(t *testing.T) {
	cases := map[string]string{
		"Batch 147：Release notes freshness gate":  "Batch 147",
		"Batch 147: Release notes freshness gate": "Batch 147",
		"Batch 147 Release notes freshness gate":  "Batch 147",
		"not a batch":                             "",
	}
	for input, want := range cases {
		if got := batchIDFromTitle(input); got != want {
			t.Fatalf("batchIDFromTitle(%q) = %q, want %q", input, got, want)
		}
	}
}

func writeReleaseHandoffFixture(t *testing.T, repo, batchTitle, changelogLine string) {
	t.Helper()
	writeFile(t, filepath.Join(repo, "rekit", "tests", "catalog.json"), `{
  "recommendedMinimum": ["go run ./cmd/rekit -- -Command release-check -Format json"],
  "globalBoundaries": ["boundary"]
}`)
	writeFile(t, filepath.Join(repo, "docs", "batch-plan.md"), "# Batch implementation plan\n\n### "+batchTitle+"\n\n状态：已完成。\n\n目标：fixture goal.\n\n验证结果：fixture validation.\n")
	writeFile(t, filepath.Join(repo, "docs", "release-readiness.md"), "## Known gaps\n\n- fixture gap\n")
	writeFile(t, filepath.Join(repo, "docs", "autonomous-goal.md"), "# autonomous goal\n")
	writeFile(t, filepath.Join(repo, "docs", "go-first-convergence-plan.md"), "# go first\n")
	writeFile(t, filepath.Join(repo, "docs", "powershell-deprecation.md"), "# powershell\n")
	writeFile(t, filepath.Join(repo, "CHANGELOG.md"), "# Changelog\n\n## Unreleased\n\n- "+changelogLine+".\n")
	writeFile(t, filepath.Join(repo, "packs", "fixture", "manifest.yml"), `id: fixture
name: Fixture
version: 0.0.0
maturity: skeleton
description: Fixture pack.
`)
}
