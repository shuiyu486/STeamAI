package releasecheck

import (
	"path/filepath"
	"strings"
	"testing"
)

func TestReleaseHandoffInventoryFromRepo(t *testing.T) {
	result, err := Build(repoRoot(t))
	if err != nil {
		t.Fatal(err)
	}
	handoff := result.ReleaseHandoff
	if !handoff.Ready || handoff.Summary != "release handoff summary ok" || len(handoff.Warnings) != 0 {
		t.Fatalf("unexpected release handoff inventory: %+v", handoff)
	}
	if len(handoff.ReadFirst) != 6 || len(handoff.Signals) != 6 || len(handoff.Validation) == 0 || len(handoff.NextActions) == 0 {
		t.Fatalf("release handoff omitted required sections: %+v", handoff)
	}
	assertHandoffReadFirst(t, handoff, "docs/release-readiness.md")
	assertHandoffReadFirst(t, handoff, "docs/autonomous-goal.md")
	assertHandoffReadFirst(t, handoff, "docs/go-first-convergence-plan.md")
	assertHandoffReadFirst(t, handoff, "docs/powershell-deprecation.md")
	assertHandoffReadFirst(t, handoff, "docs/batch-plan.md")
	assertHandoffReadFirst(t, handoff, "CHANGELOG.md")
	assertHandoffSignal(t, handoff, "release-check inventory")
	assertHandoffSignal(t, handoff, "CI release gate")
	assertHandoffSignal(t, handoff, "PowerShell deprecation")
	assertHandoffSignal(t, handoff, "heavy-tool gate manifests")
	assertHandoffSignal(t, handoff, "latest batch documentation")
	assertHandoffSignal(t, handoff, "release notes freshness")
	if handoff.ReleaseNotes.Path != "CHANGELOG.md" || !handoff.ReleaseNotes.Present || handoff.ReleaseNotes.LatestBatchID != handoff.LatestBatch.BatchID || !handoff.ReleaseNotes.Covered || handoff.ReleaseNotes.Summary != "release notes cover latest batch" {
		t.Fatalf("unexpected release notes freshness: %+v", handoff.ReleaseNotes)
	}
	if handoff.LatestBatch.PlanPath != "docs/batch-plan.md" || !handoff.LatestBatch.Present || !strings.Contains(handoff.LatestBatch.Title, "Batch ") || !strings.Contains(handoff.LatestBatch.Status, "已完成") || strings.TrimSpace(handoff.LatestBatch.Goal) == "" || strings.TrimSpace(handoff.LatestBatch.ValidationResult) == "" {
		t.Fatalf("unexpected latest batch summary: %+v", handoff.LatestBatch)
	}
}

func TestReleaseHandoffDetectsMissingHandoffDocs(t *testing.T) {
	repo := t.TempDir()
	writeFile(t, filepath.Join(repo, "rekit", "tests", "catalog.json"), `{
  "recommendedMinimum": ["go run ./cmd/rekit -- -Command release-check -Format json"],
  "globalBoundaries": ["boundary"]
}`)
	writeFile(t, filepath.Join(repo, "docs", "batch-plan.md"), `# Batch implementation plan

### Batch 999：Fixture

状态：已完成。

目标：fixture goal.

验证结果：fixture validation.
`)
	writeFile(t, filepath.Join(repo, "docs", "release-readiness.md"), "## Known gaps\n\n- fixture gap\n")
	writeFile(t, filepath.Join(repo, "CHANGELOG.md"), "# Changelog\n\n## Unreleased\n\n- Batch 999 fixture note.\n")
	writeFile(t, filepath.Join(repo, "packs", "fixture", "manifest.yml"), `id: fixture
name: Fixture
version: 0.0.0
maturity: skeleton
description: Fixture pack.
`)

	result, err := Build(repo)
	if err != nil {
		t.Fatal(err)
	}
	if result.ReleaseHandoff.Ready || result.Ready {
		t.Fatalf("release handoff unexpectedly ready despite missing docs: %+v", result.ReleaseHandoff)
	}
	assertWarningContains(t, result.ReleaseHandoff.Warnings, "docs/autonomous-goal.md")
	assertWarningContains(t, result.Warnings, "release handoff read-first document missing")
}

func assertHandoffReadFirst(t *testing.T, handoff ReleaseHandoff, path string) {
	t.Helper()
	for _, doc := range handoff.ReadFirst {
		if doc.Path == path {
			if !doc.Present || strings.TrimSpace(doc.Purpose) == "" {
				t.Fatalf("read-first doc %s = %+v, want present with purpose", path, doc)
			}
			return
		}
	}
	t.Fatalf("missing read-first doc %s: %+v", path, handoff.ReadFirst)
}

func assertHandoffSignal(t *testing.T, handoff ReleaseHandoff, name string) {
	t.Helper()
	for _, signal := range handoff.Signals {
		if signal.Name == name {
			if !signal.Ready || strings.TrimSpace(signal.Summary) == "" || len(signal.Details) == 0 {
				t.Fatalf("signal %s = %+v, want ready with summary/details", name, signal)
			}
			return
		}
	}
	t.Fatalf("missing signal %s: %+v", name, handoff.Signals)
}
