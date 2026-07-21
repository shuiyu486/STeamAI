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
	counts := ReleaseHandoffCountsFor(handoff)
	if !handoff.Ready || handoff.Summary != "release handoff summary ok" || counts.Warnings != 0 {
		t.Fatalf("unexpected release handoff inventory: %+v", handoff)
	}
	if counts.ReadFirst != 4 || counts.Signals != 13 || counts.KnownGaps == 0 || counts.PackMaturity.Total == 0 || counts.Validation == 0 || counts.NextActions == 0 {
		t.Fatalf("release handoff omitted required sections: %+v", handoff)
	}
	assertHandoffReadFirst(t, handoff, "docs/context-routing.md")
	assertHandoffReadFirst(t, handoff, "docs/batch-plan.md")
	assertHandoffReadFirst(t, handoff, "docs/release-readiness.md")
	assertHandoffReadFirst(t, handoff, "CHANGELOG.md")
	assertHandoffSignal(t, handoff, "release-check inventory")
	assertHandoffSignal(t, handoff, "CI release gate")
	assertHandoffSignalDetail(t, handoff, "PowerShell deprecation", "fallbackRetirement=true noFallback=20 candidates=0 removalModules=0 retiredModules=13")
	assertHandoffSignalDetail(t, handoff, "PowerShell deprecation", "facadeRuntime=true legacyImports=false dispatcher=false")
	assertHandoffSignalDetail(t, handoff, "PowerShell deprecation", "publicFacade=true retained=true facadeCommands=20 noFallback=20")
	assertHandoffSignalDetail(t, handoff, "PowerShell deprecation", "moduleRemoval=true candidates=0 retired=13 facadeDeps=0 undocumented=0")
	assertHandoffSignalDetail(t, handoff, "PowerShell deprecation", "moduleReferences=true activeTests=0 fixtures=0 blockers=0 unclassified=0")
	assertHandoffSignalDetail(t, handoff, "Go-native public surface", "entrypoint=cmd/rekit present=true catalog=internal/rekit/commands/commands.go catalogPresent=true")
	assertHandoffSignalDetail(t, handoff, "Go-native public surface", "default=status commands=20 handlers=20 symbols=20 profiles=20 boundaries=7 alternative=go run ./cmd/rekit -- -Command <command>")
	assertHandoffSignalDetail(t, handoff, "Go-native public surface", "profileSummary total=20 readOnly=5 mutating=15 writesCase=14 writesKit=1 reviewFirst=3 applyRequired=13 heavyTool=0 authorityConfirmed=0")
	assertHandoffSignalDetail(t, handoff, "Go-native public surface", "profileGroups readOnly=doctor,packs,release-check,status,validate reviewFirst=promote,sync,update writesKit=promote")
	assertHandoffSignalDetail(t, handoff, "Go-native public surface", "profileBoundaries rows=7 caseLocalApply=attach,bootstrap,continue,gate,handoff,init,reconcile,repair,start caseLocalReviewWriteback=plan-subagents caseLocalReviewFirst=sync,update kitReviewFirst=promote readOnly=doctor,packs,release-check,status,validate")
	assertHandoffSignalDetail(t, handoff, "Go-native public surface", "profilePolicies rows=5 violations=0")
	assertHandoffSignalDetail(t, handoff, "Go-native public surface", "facadeRemovalReady=true prerequisites=5")
	assertHandoffSignalDetail(t, handoff, "Go-native public surface", "unsupportedDiagnostic=true")
	assertHandoffSignalDetail(t, handoff, "public facade removal prerequisites", "ready=true prerequisites=8")
	assertHandoffSignalDetailContains(t, handoff, "public facade removal prerequisites", "removalPlan=true planChecks=9")
	assertHandoffSignalDetailContains(t, handoff, "public facade removal prerequisites", "replacementEntrypoints=4")
	assertHandoffSignalDetailContains(t, handoff, "public facade removal prerequisites", "replacementValidationCommands=32")
	assertHandoffSignalDetailContains(t, handoff, "public facade removal prerequisites", "deletionGates=5")
	assertHandoffSignalDetailContains(t, handoff, "public facade removal prerequisites", "deletionGateValidationCommands=40")
	assertHandoffSignalDetailContains(t, handoff, "public facade removal prerequisites", "deletionGateExitCriteria=15")
	assertHandoffSignalDetailContains(t, handoff, "public facade removal prerequisites", "deletionGateFailureSignals=15")
	assertHandoffSignalDetailContains(t, handoff, "public facade removal prerequisites", "deletionGateEscalationTriggers=15")
	assertHandoffSignalDetailContains(t, handoff, "public facade removal prerequisites", "deletionGateEscalationEvidence=15")
	assertHandoffSignalDetailContains(t, handoff, "public facade removal prerequisites", "deletionGateEscalationRecipients=15")
	assertHandoffSignalDetailContains(t, handoff, "public facade removal prerequisites", "deletionGateEscalationHandoffSteps=15")
	assertHandoffSignalDetailContains(t, handoff, "public facade removal prerequisites", "deletionGateEscalationDecisionOptions=15")
	assertHandoffSignalDetailContains(t, handoff, "public facade removal prerequisites", "deletionGateEscalationRetryConditions=15")
	assertHandoffSignalDetailContains(t, handoff, "public facade removal prerequisites", "deletionGateEscalationStopConditions=15")
	assertHandoffSignalDetailContains(t, handoff, "public facade removal prerequisites", "deletionGateEscalationResolutionArtifacts=15")
	assertHandoffSignalDetailContains(t, handoff, "public facade removal prerequisites", "deletionGateEscalationClosureChecks=15")
	assertHandoffSignalDetailContains(t, handoff, "public facade removal prerequisites", "deletionGateEscalationReopenConditions=15")
	assertHandoffSignalDetailContains(t, handoff, "public facade removal prerequisites", "deletionGateEscalationLedgerEvents=15")
	assertHandoffSignalDetailContains(t, handoff, "public facade removal prerequisites", "deletionGateEscalationStateTransitions=15")
	assertHandoffSignalDetailContains(t, handoff, "public facade removal prerequisites", "deletionGateEscalationBoundaryGuards=15")
	assertHandoffSignalDetailContains(t, handoff, "public facade removal prerequisites", "deletionGateEscalationAuditChecks=15")
	assertHandoffSignalDetailContains(t, handoff, "public facade removal prerequisites", "deletionGateVerificationArtifacts=15")
	assertHandoffSignalDetailContains(t, handoff, "public facade removal prerequisites", "deletionGateBlockedExecutionSteps=10")
	assertHandoffSignalDetailContains(t, handoff, "public facade removal prerequisites", "deletionGateRemediationActions=15")
	assertHandoffSignalDetailContains(t, handoff, "public facade removal prerequisites", "recoverySteps=4")
	assertHandoffSignalDetailContains(t, handoff, "public facade removal prerequisites", "recoveryValidationCommands=32")
	assertHandoffSignalDetailContains(t, handoff, "public facade removal prerequisites", "documentationTargets=9")
	assertHandoffSignalDetailContains(t, handoff, "public facade removal prerequisites", "documentationValidationCommands=72")
	assertHandoffSignalDetailContains(t, handoff, "public facade removal prerequisites", "executionSteps=5")
	assertHandoffSignalDetailContains(t, handoff, "public facade removal prerequisites", "executionFailureSignals=15")
	assertHandoffSignalDetailContains(t, handoff, "public facade removal prerequisites", "executionRemediationActions=15")
	assertHandoffSignalDetailContains(t, handoff, "public facade removal prerequisites", "executionVerificationArtifacts=15")
	assertHandoffSignalDetailContains(t, handoff, "public facade removal prerequisites", "executionLedgerEvents=15")
	assertHandoffSignalDetailContains(t, handoff, "public facade removal prerequisites", "executionStateTransitions=15")
	assertHandoffSignalDetailContains(t, handoff, "public facade removal prerequisites", "executionEscalationTriggers=15")
	assertHandoffSignalDetailContains(t, handoff, "public facade removal prerequisites", "executionEscalationEvidence=15")
	assertHandoffSignalDetailContains(t, handoff, "public facade removal prerequisites", "executionEscalationRecipients=15")
	assertHandoffSignalDetailContains(t, handoff, "public facade removal prerequisites", "executionEscalationHandoffSteps=15")
	assertHandoffSignalDetailContains(t, handoff, "public facade removal prerequisites", "executionEscalationDecisionOptions=15")
	assertHandoffSignalDetailContains(t, handoff, "public facade removal prerequisites", "executionEscalationRetryConditions=15")
	assertHandoffSignalDetailContains(t, handoff, "public facade removal prerequisites", "executionEscalationStopConditions=15")
	assertHandoffSignalDetailContains(t, handoff, "public facade removal prerequisites", "executionEscalationResolutionArtifacts=15")
	assertHandoffSignalDetailContains(t, handoff, "public facade removal prerequisites", "executionEscalationClosureChecks=15")
	assertHandoffSignalDetailContains(t, handoff, "public facade removal prerequisites", "executionEscalationReopenConditions=15")
	assertHandoffSignalDetailContains(t, handoff, "public facade removal prerequisites", "executionEscalationLedgerEvents=15")
	assertHandoffSignalDetailContains(t, handoff, "public facade removal prerequisites", "executionEscalationStateTransitions=15")
	assertHandoffSignalDetailContains(t, handoff, "public facade removal prerequisites", "executionEscalationBoundaryGuards=15")
	assertHandoffSignalDetailContains(t, handoff, "public facade removal prerequisites", "executionEscalationAuditChecks=15")
	assertHandoffSignalDetailContains(t, handoff, "public facade removal prerequisites", "executionBoundaryGuards=15")
	assertHandoffSignalDetailContains(t, handoff, "public facade removal prerequisites", "executionAuditChecks=15")
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
	assertHandoffSignalDetail(t, handoff, "pack-memory candidates", "openPacks=0 total=0 ready=true")
	assertHandoffSignalDetail(t, handoff, "pack-memory candidates", "nextAction=no pack-memory candidate cleanup is pending")
	if !handoff.PackMemoryCandidates.Ready || handoff.PackMemoryCandidates.Total != 0 || len(handoff.PackMemoryCandidates.Packs) != 0 || handoff.PackMemoryCandidates.NextAction != "no pack-memory candidate cleanup is pending" {
		t.Fatalf("unexpected pack-memory candidate handoff: %+v", handoff.PackMemoryCandidates)
	}
	assertHandoffSignal(t, handoff, "release notes freshness")
	assertHandoffSignal(t, handoff, "known gaps summary")
	assertHandoffKnownGap(t, handoff, "ci-release-gate")
	assertHandoffKnownGap(t, handoff, "cross-platform-product-path")
	assertHandoffKnownGap(t, handoff, "session-orchestration")
	assertHandoffKnownGap(t, handoff, "dispatch")
	assertHandoffKnownGap(t, handoff, "heavy-tool")
	assertHandoffKnownGap(t, handoff, "authority")
	assertHandoffKnownGap(t, handoff, "pack-memory")
	assertHandoffKnownGap(t, handoff, "policy-schema")
	assertHandoffKnownGap(t, handoff, "powershell-deprecation")
	if handoff.ReleaseNotes.Path != "CHANGELOG.md" || !handoff.ReleaseNotes.Present || handoff.ReleaseNotes.LatestBatchID != handoff.LatestBatch.BatchID || !handoff.ReleaseNotes.Covered || handoff.ReleaseNotes.Summary != "release notes cover latest batch" {
		t.Fatalf("unexpected release notes freshness: %+v", handoff.ReleaseNotes)
	}
	if handoff.LatestBatch.PlanPath != "docs/batch-plan.md" || !handoff.LatestBatch.Present || !strings.Contains(handoff.LatestBatch.Title, "Batch ") || !strings.Contains(handoff.LatestBatch.Status, "已完成") || strings.TrimSpace(handoff.LatestBatch.Goal) == "" || strings.TrimSpace(handoff.LatestBatch.ValidationResult) == "" {
		t.Fatalf("unexpected latest batch summary: %+v", handoff.LatestBatch)
	}
	latestHandoff := handoff.LatestBatch.Handoff
	if !latestHandoff.Completed || strings.TrimSpace(latestHandoff.RemoteReleaseGate) == "" || strings.TrimSpace(latestHandoff.NextAction) == "" || len(latestHandoff.Evidence) == 0 {
		t.Fatalf("unexpected latest batch handoff: %+v", latestHandoff)
	}
	if latestHandoff.LocalValidationReady {
		for _, evidence := range []string{"release-check -Format json recorded", "status handoff recorded", "go test ./... recorded", "git diff --check recorded"} {
			if !slices.Contains(latestHandoff.Evidence, evidence) {
				t.Fatalf("latest batch handoff evidence missing %q: %+v", evidence, latestHandoff.Evidence)
			}
		}
	}
}

func TestLatestBatchHandoffExtractsValidationEvidence(t *testing.T) {
	section := `状态：已完成 fixture implementation、durable docs、完整本地 release minimum、commit/push 与远程 release-gate inspection。

验证结果：已通过 public CLI 临时 product-path 验证与完整本地 release minimum：` + "`" + `go run ./cmd/rekit -- -Command release-check -Format json` + "`" + `、` + "`" + `go run ./cmd/rekit -- -Command status` + "`" + `、` + "`" + `go run ./cmd/rekit -- -Command packs` + "`" + `、` + "`" + `go run ./cmd/rekit -- -Command doctor` + "`" + `、` + "`" + `go test ./...` + "`" + `、` + "`" + `go vet ./...` + "`" + `、` + "`" + `git diff --check` + "`" + `；` + "`" + `release-check ready=true` + "`" + `。已提交并推送 ` + "`" + `abc123d` + "`" + `；远程 release-gate run ` + "`" + `123456789` + "`" + ` 为 completed failure，Windows/Linux/macOS jobs 均 failure 且 ` + "`" + `steps: []` + "`" + `。`

	latest := ReleaseHandoffLatestBatch{Status: "已完成 fixture", ValidationResult: "fixture validation"}
	handoff := latestBatchHandoff(latest, section)
	if !handoff.Completed || !handoff.LocalValidationReady || !handoff.ReleaseCheckReady || handoff.RemoteReleaseGate != "blocked: completed failure with jobs steps=[]" || !strings.Contains(handoff.NextAction, "known blocker") {
		t.Fatalf("unexpected latest batch handoff: %+v", handoff)
	}
	for _, evidence := range []string{"public CLI product-path validation recorded", "release-check -Format json recorded", "status handoff recorded", "packs inventory recorded", "doctor validation recorded", "go test ./... recorded", "go vet ./... recorded", "git diff --check recorded", "release-check ready=true recorded", "remote release-gate jobs steps=[] recorded"} {
		if !slices.Contains(handoff.Evidence, evidence) {
			t.Fatalf("latest batch handoff evidence missing %q: %+v", evidence, handoff.Evidence)
		}
	}
	if !slices.Contains(handoff.CommitRefs, "abc123d") || slices.Contains(handoff.CommitRefs, "123456789") {
		t.Fatalf("unexpected commit refs: %+v", handoff.CommitRefs)
	}
}

func TestLatestBatchRemoteGateDoesNotTreatNegativeGreenAsGreen(t *testing.T) {
	section := `验证结果：已通过完整本地 release minimum；release-check ready=true。远程 release-gate inspection 待 commit/push 后执行；若仍为 jobs steps: []，按既有 blocker 记录，不能声明远程 CI green。`
	if got := latestBatchRemoteReleaseGate(section); got != "not-recorded" {
		t.Fatalf("remote gate should stay not-recorded before inspection, got %q", got)
	}
}

func TestLatestBatchRemoteGateRecognizesEqualsEmptyStepsAndChineseNegativeGreen(t *testing.T) {
	section := `状态：已完成 fixture；远程 release-gate run ` + "`" + `123456789` + "`" + ` completed failure，Linux/Windows/macOS jobs ` + "`" + `steps=[]` + "`" + `，不能声明远程 CI green。`
	if got := latestBatchRemoteReleaseGate(section); got != "blocked: completed failure with jobs steps=[]" {
		t.Fatalf("remote gate should detect equals empty steps blocker, got %q", got)
	}
	if got := latestBatchRemoteGreen(section, strings.ToLower(section)); got {
		t.Fatalf("negative Chinese remote CI green phrase should not be treated as green")
	}
	handoff := latestBatchHandoff(ReleaseHandoffLatestBatch{Status: "已完成 fixture"}, section)
	if !slices.Contains(handoff.Evidence, "remote release-gate jobs steps=[] recorded") {
		t.Fatalf("latest batch handoff evidence missing remote empty steps: %+v", handoff.Evidence)
	}
}

func TestReleaseHandoffPackMemoryCandidatesDetectsOpenResidue(t *testing.T) {
	repo := t.TempDir()
	writeFile(t, filepath.Join(repo, "packs", "fixture", "promote-candidates", "candidate.candidate.md"), "# candidate\n")
	writeFile(t, filepath.Join(repo, "packs", "fixture", "tooling", "candidates", "tool.candidate.md"), "# tooling\n")
	writeFile(t, filepath.Join(repo, "packs", "fixture", "promote-candidates", "index.json"), `[
  {
    "path": "references/template/README.md",
    "candidate": "candidate.candidate.md"
  }
]
`)

	inventory := releaseHandoffPackMemoryCandidates(repo, []manifest.PackSummary{{ID: "fixture", Maturity: "skeleton"}})
	if inventory.Ready || inventory.Summary != "pack-memory candidate inventory has open review/cleanup work" || inventory.Total != 3 || len(inventory.Packs) != 1 || !strings.Contains(inventory.NextAction, "review listed pack-memory candidates") || len(inventory.Warnings) == 0 {
		t.Fatalf("unexpected pack-memory candidate inventory: %+v", inventory)
	}
	pack := inventory.Packs[0]
	if pack.Pack != "fixture" || pack.CandidateRoot != "packs/fixture/promote-candidates" || pack.ToolingRoot != "packs/fixture/tooling/candidates" || pack.IndexPath != "packs/fixture/promote-candidates/index.json" || pack.CandidateFiles != 1 || pack.ToolingFiles != 1 || pack.IndexEntries != 1 || !pack.HasOpenWork || !pack.RequiresReview || !pack.RequiresCleanup {
		t.Fatalf("unexpected pack-memory candidate status: %+v", pack)
	}
	if !slices.Contains(pack.CandidatePaths, "packs/fixture/promote-candidates/candidate.candidate.md") || !slices.Contains(pack.ToolingPaths, "packs/fixture/tooling/candidates/tool.candidate.md") || len(pack.IndexCandidates) != 1 || pack.IndexCandidates[0].Candidate != "packs/fixture/promote-candidates/candidate.candidate.md" || pack.IndexCandidates[0].Path != "references/template/README.md" {
		t.Fatalf("pack-memory candidate identity handoff drifted: %+v", pack)
	}
	if !releaseHandoffReviewArtifactContains(pack.ReviewArtifacts, "candidate-decision-note", "packs/fixture/promote-candidates/candidate.candidate.md", "references/template/README.md") || !releaseHandoffReviewArtifactContains(pack.ReviewArtifacts, "candidate-cleanup-proof", "packs/fixture/promote-candidates/candidate.candidate.md", "references/template/README.md") || !releaseHandoffReviewArtifactContains(pack.ReviewArtifacts, "fresh-case-reconsume-proof", "packs/fixture/tooling/candidates/tool.candidate.md", "tooling/catalog.yml or tooling/recipes/*") || !releaseHandoffReviewArtifactContains(pack.ReviewArtifacts, "attached-case-reconsume-proof", "packs/fixture/tooling/candidates/tool.candidate.md", "tooling/catalog.yml or tooling/recipes/*") {
		t.Fatalf("pack-memory candidate review artifact handoff drifted: %+v", pack.ReviewArtifacts)
	}
	for _, evidence := range []string{"promote-candidates files=1", "tooling/candidates files=1", "indexPath packs/fixture/promote-candidates/index.json entries=1"} {
		if !slices.Contains(pack.Evidence, evidence) {
			t.Fatalf("pack-memory candidate evidence missing %q: %+v", evidence, pack.Evidence)
		}
	}
	if !releaseHandoffStringsContain(pack.Boundary, "does not merge or delete") || !releaseHandoffStringsContain(releaseHandoffPackMemoryCandidateDetails(inventory), "pack=fixture") {
		t.Fatalf("pack-memory candidate handoff omitted boundary/detail: pack=%+v details=%+v", pack, releaseHandoffPackMemoryCandidateDetails(inventory))
	}
}

func TestReleaseHandoffPackMaturityDetectsMissingHeavyToolGates(t *testing.T) {
	inventory := releaseHandoffPackMaturity([]manifest.PackSummary{
		{ID: "fixture", Maturity: "skeleton", SchemaValid: true, SchemaVersion: "1"},
	}, nil)
	if inventory.Total != 1 || inventory.MaturityCounts["skeleton"] != 1 || inventory.SchemaValid != true || !inventory.SchemaVersionReady || inventory.HeavyToolGateReady || inventory.Summary != "pack maturity inventory has warnings" {
		t.Fatalf("unexpected drifted pack maturity inventory: %+v", inventory)
	}
	counts := ReleaseHandoffPackMaturityCountsFor(inventory)
	if counts.HeavyToolGatesByPack != 1 || inventory.HeavyToolGatesByPack[0].ID != "fixture" || inventory.HeavyToolGatesByPack[0].HeavyToolGates != 0 {
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
	counts := ReleaseHandoffPackMaturityCountsFor(inventory)
	if counts.HeavyToolGatesByPack != 1 || inventory.HeavyToolGatesByPack[0].ID != "fixture" || inventory.HeavyToolGatesByPack[0].SchemaVersion != "" {
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
	if ReleaseHandoffCountsFor(result.ReleaseHandoff).KnownGaps != 0 || result.ReleaseHandoff.Ready || result.Ready {
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
	writeFile(t, filepath.Join(repo, "README.md"), "# README\n\n用户主要指挥主 Agent / Mission Commander\nGo CLI/backend 是背后的 canonical deterministic runtime/API\n`rekit.ps1` 仅作为 retained compatibility façade\n默认路径继续向 PowerShell-free / Go-native / 跨平台收敛\n这里不需要你手动执行底层脚本\n用户不需要把 `/rekit` 子命令当成主要交互界面\n")
	writeFile(t, filepath.Join(repo, "docs", "mission-control-product-direction.md"), "# mission control\n\nLane-centric Agent Team Mission Control\n用户主要和一个 **主 Agent / Mission Commander** 会话交互\nGo-first deterministic substrate\n")
	writeFile(t, filepath.Join(repo, "docs", "go-first-convergence-plan.md"), "# go first\n\nGo backend 成为 rekit 的 deterministic runtime owner\n不要把大型 PowerShell matrix 作为默认必跑\nPowerShell-free / Go-native convergence\n")
	writeFile(t, filepath.Join(repo, "docs", "powershell-deprecation.md"), "# powershell\n\nPowerShell-free default/product path / Go-native / 跨平台 convergence\nGo CLI/backend 是 canonical runtime\nPowerShell 当前只保留 `rekit/rekit.ps1` compatibility façade 与按需 parity residue\n")
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
	assertWarningContains(t, result.ReleaseHandoff.Warnings, "docs/context-routing.md")
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

func releaseHandoffStringsContain(values []string, want string) bool {
	for _, value := range values {
		if strings.Contains(value, want) {
			return true
		}
	}
	return false
}

func releaseHandoffReviewArtifactContains(items []ReleaseHandoffPackMemoryCandidateReviewArtifact, name, candidatePath, packTarget string) bool {
	for _, item := range items {
		if item.Name == name && item.CandidatePath == candidatePath && item.PackTarget == packTarget && strings.TrimSpace(item.When) != "" && strings.TrimSpace(item.Action) != "" && len(item.Evidence) > 0 && len(item.Boundary) > 0 {
			return true
		}
	}
	return false
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
	counts := ReleaseHandoffPackMaturityCountsFor(inventory)
	if counts.HeavyToolGatesByPack != counts.Total {
		t.Fatalf("heavy-tool gate rows = %d, want total %d", counts.HeavyToolGatesByPack, counts.Total)
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
