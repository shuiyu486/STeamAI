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
	if len(handoff.ReadFirst) != 7 || len(handoff.Signals) != 12 || len(handoff.KnownGaps) == 0 || handoff.PackMaturity.Total == 0 || len(handoff.Validation) == 0 || len(handoff.NextActions) == 0 {
		t.Fatalf("release handoff omitted required sections: %+v", handoff)
	}
	assertHandoffReadFirst(t, handoff, "docs/release-readiness.md")
	assertHandoffReadFirst(t, handoff, "docs/mission-control-product-direction.md")
	assertHandoffReadFirst(t, handoff, "docs/autonomous-goal.md")
	assertHandoffReadFirst(t, handoff, "docs/go-first-convergence-plan.md")
	assertHandoffReadFirst(t, handoff, "docs/powershell-deprecation.md")
	assertHandoffReadFirst(t, handoff, "docs/batch-plan.md")
	assertHandoffReadFirst(t, handoff, "CHANGELOG.md")
	assertHandoffSignal(t, handoff, "release-check inventory")
	assertHandoffSignal(t, handoff, "CI release gate")
	assertHandoffSignalDetail(t, handoff, "PowerShell deprecation", "fallbackRetirement=true noFallback=19 candidates=0 removalModules=0 retiredModules=13")
	assertHandoffSignalDetail(t, handoff, "PowerShell deprecation", "facadeRuntime=true legacyImports=false dispatcher=false")
	assertHandoffSignalDetail(t, handoff, "PowerShell deprecation", "publicFacade=true retained=true facadeCommands=19 noFallback=19")
	assertHandoffSignalDetail(t, handoff, "PowerShell deprecation", "moduleRemoval=true candidates=0 retired=13 facadeDeps=0 undocumented=0")
	assertHandoffSignalDetail(t, handoff, "PowerShell deprecation", "moduleReferences=true activeTests=0 fixtures=0 blockers=0 unclassified=0")
	assertHandoffSignalDetail(t, handoff, "Go-native public surface", "entrypoint=cmd/rekit present=true catalog=internal/rekit/commands/commands.go catalogPresent=true")
	assertHandoffSignalDetail(t, handoff, "Go-native public surface", "default=status commands=19 handlers=19 symbols=19 profiles=19 boundaries=7 alternative=go run ./cmd/rekit -- -Command <command>")
	assertHandoffSignalDetail(t, handoff, "Go-native public surface", "profileSummary total=19 readOnly=5 mutating=14 writesCase=13 writesKit=1 reviewFirst=3 applyRequired=11 heavyTool=0 authorityConfirmed=0")
	assertHandoffSignalDetail(t, handoff, "Go-native public surface", "profileGroups readOnly=doctor,packs,release-check,status,validate reviewFirst=promote,sync,update writesKit=promote")
	assertHandoffSignalDetail(t, handoff, "Go-native public surface", "profileBoundaries rows=7 caseLocalApply=attach,bootstrap,continue,gate,handoff,init,repair,start kitReviewFirst=promote readOnly=doctor,packs,release-check,status,validate")
	assertHandoffSignalDetail(t, handoff, "Go-native public surface", "profilePolicies rows=5 violations=0")
	assertHandoffSignalDetail(t, handoff, "Go-native public surface", "facadeRemovalReady=true prerequisites=5")
	assertHandoffSignalDetail(t, handoff, "Go-native public surface", "unsupportedDiagnostic=true")
	assertHandoffSignalDetail(t, handoff, "public facade removal prerequisites", "ready=true prerequisites=8")
	assertHandoffSignalDetailContains(t, handoff, "public facade removal prerequisites", "removalPlan=true planChecks=9")
	assertHandoffSignalDetailContains(t, handoff, "public facade removal prerequisites", "replacementEntrypoints=4")
	assertHandoffSignalDetailContains(t, handoff, "public facade removal prerequisites", "replacementValidationCommands=32")
	assertHandoffSignalDetailContains(t, handoff, "public facade removal prerequisites", "deletionGates=5")
	assertHandoffSignalDetailContains(t, handoff, "public facade removal prerequisites", "deletionGateValidationCommands=40")
	assertHandoffSignalDetailContains(t, handoff, "public facade removal prerequisites", "recoverySteps=4")
	assertHandoffSignalDetailContains(t, handoff, "public facade removal prerequisites", "recoveryValidationCommands=32")
	assertHandoffSignalDetailContains(t, handoff, "public facade removal prerequisites", "documentationTargets=9")
	assertHandoffSignalDetailContains(t, handoff, "public facade removal prerequisites", "documentationValidationCommands=72")
	assertHandoffSignalDetailContains(t, handoff, "public facade removal prerequisites", "executionSteps=5")
	assertHandoffSignalDetailContains(t, handoff, "public facade removal prerequisites", "executionValidationCommands=40")
	assertHandoffSignalDetailContains(t, handoff, "public facade removal prerequisites", "boundaryChecks=6")
	assertHandoffSignalDetailContains(t, handoff, "public facade removal prerequisites", "boundaryValidationCommands=48")
	assertHandoffSignalDetailContains(t, handoff, "public facade removal prerequisites", "removalImpact=true impactReferences=")
	assertHandoffSignalDetailContains(t, handoff, "public facade removal prerequisites", "workItems=")
	assertHandoffSignalDetailContains(t, handoff, "public facade removal prerequisites", "validationCommands=")
	assertHandoffSignalDetailContains(t, handoff, "public facade removal prerequisites", "migrationTargets=74")
	assertHandoffSignalDetailContains(t, handoff, "public facade removal prerequisites", "migrationValidationCommands=592")
	assertHandoffSignalDetailContains(t, handoff, "public facade removal prerequisites", "smokeMigrationTargets=29")
	assertHandoffSignalDetailContains(t, handoff, "public facade removal prerequisites", "smokeMigrationValidationCommands=232")
	assertHandoffSignalDetail(t, handoff, "public facade removal prerequisites", "public-facade-retained-boundary ready=true publicFacadeReady=true present=true retained=true migrationBoundary=true removalBoundary=true")
	assertHandoffSignalDetail(t, handoff, "public facade removal prerequisites", "go-native-public-surface ready=true goNativeReady=true facadeRemovalReady=true prerequisites=5")
	assertHandoffSignal(t, handoff, "case shim readiness")
	assertHandoffSignal(t, handoff, "public default docs")
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
		{ID: "fixture", Maturity: "skeleton", SchemaValid: true, SchemaVersion: "1"},
	}, nil)
	if inventory.Total != 1 || inventory.MaturityCounts["skeleton"] != 1 || inventory.SchemaValid != true || !inventory.SchemaVersionReady || inventory.HeavyToolGateReady || inventory.Summary != "pack maturity inventory has warnings" {
		t.Fatalf("unexpected drifted pack maturity inventory: %+v", inventory)
	}
	if len(inventory.HeavyToolGatesByPack) != 1 || inventory.HeavyToolGatesByPack[0].ID != "fixture" || inventory.HeavyToolGatesByPack[0].HeavyToolGates != 0 {
		t.Fatalf("unexpected drifted pack gate status: %+v", inventory.HeavyToolGatesByPack)
	}
}

func TestReleaseHandoffPackMaturityDetectsMissingSchemaVersion(t *testing.T) {
	inventory := releaseHandoffPackMaturity([]manifest.PackSummary{
		{ID: "fixture", Maturity: "skeleton", SchemaValid: true, HeavyToolGates: 1, HeavyToolGateActions: []string{"debug"}},
	}, []string{"debug"})
	if inventory.Total != 1 || inventory.SchemaValid != true || inventory.SchemaVersionReady || !inventory.HeavyToolGateReady || inventory.Summary != "pack maturity inventory has warnings" {
		t.Fatalf("unexpected schema-version drifted pack maturity inventory: %+v", inventory)
	}
	if len(inventory.HeavyToolGatesByPack) != 1 || inventory.HeavyToolGatesByPack[0].ID != "fixture" || inventory.HeavyToolGatesByPack[0].SchemaVersion != "" {
		t.Fatalf("unexpected schema-version drifted pack gate status: %+v", inventory.HeavyToolGatesByPack)
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
	writeFile(t, filepath.Join(repo, "README.md"), "# README\n\n用户主要指挥主 Agent / Mission Commander\nGo CLI/backend 是背后的 canonical deterministic runtime/API\n`rekit.ps1` 仅作为迁移期 legacy façade\n默认路径继续向 PowerShell-free / Go-native / 跨平台收敛\n这里不需要你手动执行底层脚本\n用户不需要把 `/rekit` 子命令当成主要交互界面\n")
	writeFile(t, filepath.Join(repo, "docs", "mission-control-product-direction.md"), "# mission control\n\nLane-centric Agent Team Mission Control\n用户主要和一个 **主 Agent / Mission Commander** 会话交互\nGo-first deterministic substrate\n")
	writeFile(t, filepath.Join(repo, "docs", "go-first-convergence-plan.md"), "# go first\n\nGo backend 成为 rekit 的 deterministic runtime owner\n不要把大型 PowerShell matrix 作为默认必跑\nPowerShell-free / Go-native convergence\n")
	writeFile(t, filepath.Join(repo, "docs", "powershell-deprecation.md"), "# powershell\n\nPowerShell-free / Go-native / 跨平台 convergence\nGo CLI/backend 是 canonical runtime\nPowerShell 只作为迁移期 legacy façade / fallback / parity residue\n")
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

func assertHandoffSignalDetail(t *testing.T, handoff ReleaseHandoff, name, detail string) {
	t.Helper()
	for _, signal := range handoff.Signals {
		if signal.Name == name {
			if !signal.Ready || strings.TrimSpace(signal.Summary) == "" || len(signal.Details) == 0 {
				t.Fatalf("signal %s = %+v, want ready with summary/details", name, signal)
			}
			if slices.Contains(signal.Details, detail) {
				return
			}
			t.Fatalf("signal %s missing detail %q: %+v", name, detail, signal.Details)
		}
	}
	t.Fatalf("missing signal %s: %+v", name, handoff.Signals)
}

func assertHandoffSignalDetailContains(t *testing.T, handoff ReleaseHandoff, name, detail string) {
	t.Helper()
	for _, signal := range handoff.Signals {
		if signal.Name == name {
			if !signal.Ready || strings.TrimSpace(signal.Summary) == "" || len(signal.Details) == 0 {
				t.Fatalf("signal %s = %+v, want ready with summary/details", name, signal)
			}
			for _, actual := range signal.Details {
				if strings.Contains(actual, detail) {
					return
				}
			}
			t.Fatalf("signal %s missing detail containing %q: %+v", name, detail, signal.Details)
		}
	}
	t.Fatalf("missing signal %s: %+v", name, handoff.Signals)
}

func assertHandoffPackMaturity(t *testing.T, handoff ReleaseHandoff) {
	t.Helper()
	inventory := handoff.PackMaturity
	if inventory.Total != 10 || !inventory.SchemaValid || !inventory.SchemaVersionReady || !inventory.HeavyToolGateReady || inventory.Summary != "pack maturity inventory ok" {
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
		if strings.TrimSpace(pack.ID) == "" || strings.TrimSpace(pack.Maturity) == "" || !pack.SchemaValid || pack.SchemaVersion != "1" || pack.HeavyToolGates == 0 || len(pack.Actions) == 0 {
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
