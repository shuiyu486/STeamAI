package releasecheck

import (
	"path/filepath"
	"slices"
	"strings"
	"testing"

	"github.com/shuiyu486/re-context-kits/internal/rekit/defaults"
	"github.com/shuiyu486/re-context-kits/internal/rekit/manifest"
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
	if len(handoff.ReadFirst) != 6 || len(handoff.Signals) != 8 || len(handoff.KnownGaps) == 0 || handoff.PackMaturity.Total == 0 || len(handoff.Validation) == 0 || len(handoff.NextActions) == 0 {
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
	assertHandoffSignal(t, handoff, "pack maturity summary")
	assertHandoffPackMaturity(t, handoff)
	assertHandoffSignal(t, handoff, "latest batch documentation")
	assertHandoffSignal(t, handoff, "release notes freshness")
	assertHandoffSignal(t, handoff, "known gaps summary")
	assertHandoffKnownGap(t, handoff, "dispatch")
	assertHandoffKnownGap(t, handoff, "heavy-tool")
	assertHandoffKnownGap(t, handoff, "authority")
	assertHandoffKnownGap(t, handoff, "policy-schema")
	assertHandoffKnownGap(t, handoff, "powershell-deprecation")
	if handoff.ReleaseNotes.Path != "CHANGELOG.md" || !handoff.ReleaseNotes.Present || handoff.ReleaseNotes.LatestBatchID != handoff.LatestBatch.BatchID || !handoff.ReleaseNotes.Covered || handoff.ReleaseNotes.Summary != "release notes cover latest batch" {
		t.Fatalf("unexpected release notes freshness: %+v", handoff.ReleaseNotes)
	}
	if handoff.LatestBatch.PlanPath != "docs/batch-plan.md" || !handoff.LatestBatch.Present || !strings.Contains(handoff.LatestBatch.Title, "Batch ") || !strings.Contains(handoff.LatestBatch.Status, "已完成") || strings.TrimSpace(handoff.LatestBatch.Goal) == "" || strings.TrimSpace(handoff.LatestBatch.ValidationResult) == "" {
		t.Fatalf("unexpected latest batch summary: %+v", handoff.LatestBatch)
	}
}

func TestReleaseHandoffPackMaturityDetectsMissingHeavyToolGates(t *testing.T) {
	inventory := releaseHandoffPackMaturity([]manifest.PackSummary{
		{ID: "fixture", Maturity: "skeleton", SchemaValid: true},
	}, nil)
	if inventory.Total != 1 || inventory.MaturityCounts["skeleton"] != 1 || inventory.SchemaValid != true || inventory.HeavyToolGateReady || inventory.Summary != "pack maturity inventory has warnings" {
		t.Fatalf("unexpected drifted pack maturity inventory: %+v", inventory)
	}
	if len(inventory.HeavyToolGatesByPack) != 1 || inventory.HeavyToolGatesByPack[0].ID != "fixture" || inventory.HeavyToolGatesByPack[0].HeavyToolGates != 0 {
		t.Fatalf("unexpected drifted pack gate status: %+v", inventory.HeavyToolGatesByPack)
	}
}

func TestReleaseHandoffDetectsMissingKnownGaps(t *testing.T) {
	repo := t.TempDir()
	writeReleaseHandoffFixture(t, repo, "Batch 999：Fixture", "Batch 999 fixture note")
	writeFile(t, filepath.Join(repo, "docs", "release-readiness.md"), "# Release readiness\n\n## Known gaps\n\n")

	result, err := Build(repo)
	if err != nil {
		t.Fatal(err)
	}
	if len(result.ReleaseHandoff.KnownGaps) != 0 || result.ReleaseHandoff.Ready || result.Ready {
		t.Fatalf("release handoff unexpectedly ready despite missing known gaps: %+v", result.ReleaseHandoff)
	}
	assertWarningContains(t, result.ReleaseHandoff.Warnings, "known gaps summary")
	assertWarningContains(t, result.Warnings, "known gaps summary")
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

func assertHandoffPackMaturity(t *testing.T, handoff ReleaseHandoff) {
	t.Helper()
	inventory := handoff.PackMaturity
	if inventory.Total != 10 || !inventory.SchemaValid || !inventory.HeavyToolGateReady || inventory.Summary != "pack maturity inventory ok" {
		t.Fatalf("unexpected pack maturity inventory: %+v", inventory)
	}
	if inventory.MaturityCounts["template"] != 1 || inventory.MaturityCounts["mature"] != 1 || inventory.MaturityCounts["skeleton"] != 8 {
		t.Fatalf("unexpected maturity counts: %+v", inventory.MaturityCounts)
	}
	if strings.Join(inventory.HeavyToolGateActions, ",") != "debug,dump,full-trace,inject,network,patch,symex" {
		t.Fatalf("unexpected heavy-tool gate actions: %v", inventory.HeavyToolGateActions)
	}
	assertHandoffMaturityPack(t, inventory, "template", "_template")
	assertHandoffMaturityPack(t, inventory, "mature", defaults.DefaultPack)
	assertHandoffMaturityPack(t, inventory, "skeleton", "web-security")
	if len(inventory.HeavyToolGatesByPack) != inventory.Total {
		t.Fatalf("heavy-tool gate rows = %d, want total %d", len(inventory.HeavyToolGatesByPack), inventory.Total)
	}
	for _, pack := range inventory.HeavyToolGatesByPack {
		if strings.TrimSpace(pack.ID) == "" || strings.TrimSpace(pack.Maturity) == "" || !pack.SchemaValid || pack.HeavyToolGates == 0 || len(pack.Actions) == 0 {
			t.Fatalf("unexpected pack gate row: %+v", pack)
		}
	}
}

func assertHandoffMaturityPack(t *testing.T, inventory ReleaseHandoffPackMaturity, maturity, packID string) {
	t.Helper()
	if slices.Contains(inventory.PacksByMaturity[maturity], packID) {
		return
	}
	t.Fatalf("pack maturity %s missing %s: %+v", maturity, packID, inventory.PacksByMaturity)
}

func assertHandoffKnownGap(t *testing.T, handoff ReleaseHandoff, category string) {
	t.Helper()
	for _, gap := range handoff.KnownGaps {
		if strings.Contains(gap.Category, category) {
			if gap.Index <= 0 || strings.TrimSpace(gap.Summary) == "" {
				t.Fatalf("known gap %s = %+v, want index and summary", category, gap)
			}
			return
		}
	}
	t.Fatalf("missing known gap category %s: %+v", category, handoff.KnownGaps)
}
