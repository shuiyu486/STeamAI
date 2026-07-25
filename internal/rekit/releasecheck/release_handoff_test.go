package releasecheck

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"

	"github.com/shuiyu486/re-context-kits/internal/rekit/defaults"
	"github.com/shuiyu486/re-context-kits/internal/rekit/manifest"
	"github.com/shuiyu486/re-context-kits/internal/rekit/promote"
	syncpkg "github.com/shuiyu486/re-context-kits/internal/rekit/sync"
)

func TestReleaseHandoffInventoryFromRepo(t *testing.T) {
	result, err := Build(cleanReleaseRepoRoot(t))
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
	assertHandoffSignalDetailContains(t, handoff, "public facade removal prerequisites", "migrationTargets=75")
	assertHandoffSignalDetailContains(t, handoff, "public facade removal prerequisites", "migrationValidationCommands=600")
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
	if !latestHandoff.Completed || strings.TrimSpace(latestHandoff.RemoteReleaseGate) == "" || latestHandoff.RemoteReleaseGateDetail == nil || strings.TrimSpace(latestHandoff.RemoteReleaseGateDetail.State) == "" || strings.TrimSpace(latestHandoff.NextAction) == "" {
		t.Fatalf("unexpected latest batch handoff: %+v", latestHandoff)
	}
	if cadence := latestHandoff.ReleaseInspectionCadence; cadence.MaxPushes != 2 || cadence.State == "" || cadence.NextAction == "" || cadence.ThirdInspectionAllowed != cadence.NewRemoteSignal || len(cadence.Boundary) == 0 || !releaseHandoffStringsContain(cadence.Boundary, "do not add a third record commit") {
		t.Fatalf("latest batch release inspection cadence drifted: %+v", cadence)
	}
	if latestHandoff.RemoteReleaseGateDetail.State != latestHandoff.RemoteReleaseGate {
		t.Fatalf("remote gate detail state drifted from summary: %+v", latestHandoff.RemoteReleaseGateDetail)
	}
	if latestHandoff.LocalValidationReady {
		for _, evidence := range []string{"release-check -Format json recorded", "status handoff recorded", "go test ./... recorded", "git diff --check recorded"} {
			if !slices.Contains(latestHandoff.Evidence, evidence) {
				t.Fatalf("latest batch handoff evidence missing %q: %+v", evidence, latestHandoff.Evidence)
			}
		}
	}
}

func TestLatestBatchSummarySelectsFirstCurrentBatchSection(t *testing.T) {
	repo := t.TempDir()
	planPath := filepath.Join(repo, "docs", "batch-plan.md")
	if err := os.MkdirAll(filepath.Dir(planPath), 0o755); err != nil {
		t.Fatal(err)
	}
	plan := `# Batch implementation plan

### Current batch state

### Batch 539：current batch

状态：已完成 current implementation、完整本地 release minimum、implementation commit/push 与远程 release-gate inspection。

目标：current goal

验证结果：已通过 ` + "`go run ./cmd/rekit -- -Command release-check -Format json`" + `（ready=true）、` + "`go run ./cmd/rekit -- -Command status`" + `、` + "`go run ./cmd/rekit -- -Command packs`" + `、` + "`go run ./cmd/rekit -- -Command doctor`" + `、` + "`go test ./...`" + `、` + "`go vet ./...`" + `、` + "`git diff --check`" + `；远程 release-gate run ` + "`29945764199`" + ` completed failure，Linux/macOS/Windows jobs 均 ` + "`steps=[]`" + `。

上一批摘要：Batch 538 已完成。

### Batch 538：previous batch

状态：已完成 previous implementation。

目标：previous goal

验证结果：previous validation。
`
	if err := os.WriteFile(planPath, []byte(plan), 0o644); err != nil {
		t.Fatal(err)
	}
	latest := latestBatchSummary(repo)
	if latest.BatchID != "Batch 539" || !strings.Contains(latest.Title, "current batch") || latest.Goal != "current goal" || !strings.Contains(latest.ValidationResult, "release-check -Format json") || !latest.Handoff.ReleaseCheckReady || latest.Handoff.RemoteReleaseGate != "blocked: completed failure with jobs steps=[]" {
		t.Fatalf("latest batch parser selected stale historical section: %+v", latest)
	}
}

func TestLatestBatchHandoffExtractsValidationEvidence(t *testing.T) {
	section := `状态：已完成 fixture implementation、durable docs、完整本地 release minimum、commit/push 与远程 release-gate inspection。

验证结果：已通过 public CLI 临时 product-path 验证与完整本地 release minimum：` + "`" + `go run ./cmd/rekit -- -Command release-check -Format json` + "`" + `、` + "`" + `go run ./cmd/rekit -- -Command status` + "`" + `、` + "`" + `go run ./cmd/rekit -- -Command packs` + "`" + `、` + "`" + `go run ./cmd/rekit -- -Command doctor` + "`" + `、` + "`" + `go test ./...` + "`" + `、` + "`" + `go vet ./...` + "`" + `、` + "`" + `git diff --check` + "`" + `；` + "`" + `release-check ready=true` + "`" + `。implementation commits ` + "`" + `abc123d` + "`" + ` / ` + "`" + `9887297` + "`" + ` 已提交并推送；远程 release-gate run ` + "`" + `123456789` + "`" + ` 为 completed failure，Windows/Linux/macOS jobs 均 failure 且 ` + "`" + `steps: []` + "`" + `。`

	latest := ReleaseHandoffLatestBatch{Status: "已完成 fixture", ValidationResult: "fixture validation"}
	handoff := latestBatchHandoff(latest, section)
	if !handoff.Completed || !handoff.LocalValidationReady || !handoff.ReleaseCheckReady || handoff.RemoteReleaseGate != "blocked: completed failure with jobs steps=[]" || !strings.Contains(handoff.NextAction, "third inspection") {
		t.Fatalf("unexpected latest batch handoff: %+v", handoff)
	}
	if cadence := handoff.ReleaseInspectionCadence; cadence.MaxPushes != 2 || cadence.State != "complete" || !cadence.ImplementationCommitReady || !cadence.InspectionCommitReady || cadence.ThirdInspectionAllowed || cadence.NewRemoteSignal || !strings.Contains(cadence.NextAction, "third inspection") || !releaseHandoffStringsContain(cadence.Evidence, "implementation commit/push recorded") || !releaseHandoffStringsContain(cadence.Evidence, "release inspection commit/run recorded") || !releaseHandoffStringsContain(cadence.Boundary, "only a remote signal different") {
		t.Fatalf("unexpected release inspection cadence: %+v", cadence)
	}
	for _, evidence := range []string{"public CLI product-path validation recorded", "release-check -Format json recorded", "status handoff recorded", "packs inventory recorded", "doctor validation recorded", "go test ./... recorded", "go vet ./... recorded", "git diff --check recorded", "release-check ready=true recorded", "remote release-gate jobs steps=[] recorded"} {
		if !slices.Contains(handoff.Evidence, evidence) {
			t.Fatalf("latest batch handoff evidence missing %q: %+v", evidence, handoff.Evidence)
		}
	}
	for _, want := range []string{"abc123d", "9887297"} {
		if !slices.Contains(handoff.CommitRefs, want) {
			t.Fatalf("commit refs missing %q: %+v", want, handoff.CommitRefs)
		}
	}
	if slices.Contains(handoff.CommitRefs, "123456789") {
		t.Fatalf("remote run id should not be treated as a commit ref: %+v", handoff.CommitRefs)
	}
}

func TestLatestBatchReleaseInspectionCadenceWaitsForImplementationCommit(t *testing.T) {
	section := `状态：已完成 runtime/test/docs implementation，但尚未提交推送；完整本地 release minimum、implementation commit/push 与远程 release-gate inspection 待最终执行。

目标：正常批次最多两次 push；第三个记录提交只有出现不同于既有 ` + "`" + `steps=[]` + "`" + ` runner/billing blocker 的新远程信号时才允许。

验证结果：完整本地 release minimum 待最终执行；当前仅已通过 ` + "`" + `go run ./cmd/rekit -- -Command release-check -Format json` + "`" + `、` + "`" + `go run ./cmd/rekit -- -Command status` + "`" + `、` + "`" + `go run ./cmd/rekit -- -Command packs` + "`" + `、` + "`" + `go run ./cmd/rekit -- -Command doctor` + "`" + `、` + "`" + `go test ./...` + "`" + `、` + "`" + `go vet ./...` + "`" + `、` + "`" + `git diff --check` + "`" + `。远程 release-gate inspection 待 implementation commit/push 后执行。`
	handoff := latestBatchHandoff(ReleaseHandoffLatestBatch{Status: "已完成 fixture"}, section)
	if handoff.LocalValidationReady {
		t.Fatalf("pending local release minimum should not be marked ready: %+v", handoff)
	}
	if cadence := handoff.ReleaseInspectionCadence; cadence.State != "implementation-pending" || cadence.ImplementationCommitReady || cadence.InspectionCommitReady || len(cadence.Evidence) != 0 || cadence.ThirdInspectionAllowed || !strings.Contains(cadence.NextAction, "implementation commit") {
		t.Fatalf("unexpected pre-commit cadence: %+v", cadence)
	}
	if !strings.Contains(handoff.NextAction, "local release minimum") {
		t.Fatalf("latest next action should wait for local validation before implementation commit: %+v", handoff)
	}
}

func TestLatestBatchRemoteGateDoesNotTreatNegativeGreenAsGreen(t *testing.T) {
	section := `验证结果：已通过完整本地 release minimum；release-check ready=true。远程 release-gate inspection 待 commit/push 后执行；若仍为 jobs steps: []，按既有 blocker 记录，不能声明远程 CI green。`
	if got := latestBatchRemoteReleaseGate(section); got != "not-recorded" {
		t.Fatalf("remote gate should stay not-recorded before inspection, got %q", got)
	}
}

func TestLatestBatchRemoteGateIgnoresPolicyOnlyEmptyStepsBeforeInspection(t *testing.T) {
	section := `状态：已完成 runtime/test/doc 工作树实现、focused tests、受影响 package tests 与完整本地 release minimum；尚未创建本批代码提交，尚未检查本批对应的远程 workflow run。本批远程 release-gate 若继续出现 jobs ` + "`" + `steps=[]` + "`" + ` 且无 logs，应仅记录为既有 runner/billing blocker，不能声明 remote CI green，也不要为后续 release inspection 记录自身的 CI 追加第三个记录提交。

验证结果：完整本地 release minimum 已通过：` + "`" + `go run ./cmd/rekit -- -Command release-check -Format json` + "`" + ` 返回 ` + "`" + `ready=true` + "`" + `，` + "`" + `go run ./cmd/rekit -- -Command status` + "`" + `、` + "`" + `go run ./cmd/rekit -- -Command packs` + "`" + `、` + "`" + `go run ./cmd/rekit -- -Command doctor` + "`" + `、` + "`" + `go test ./...` + "`" + `、` + "`" + `go vet ./...` + "`" + ` 与 ` + "`" + `git diff --check` + "`" + ` 均通过。`

	handoff := latestBatchHandoff(ReleaseHandoffLatestBatch{Status: "已完成 fixture"}, section)
	if !handoff.LocalValidationReady || !handoff.ReleaseCheckReady {
		t.Fatalf("local validation should be ready before commit: %+v", handoff)
	}
	if handoff.RemoteReleaseGate != "not-recorded" || handoff.RemoteReleaseGateDetail == nil || handoff.RemoteReleaseGateDetail.EmptySteps {
		t.Fatalf("policy-only steps=[] should not be treated as inspected remote gate: %+v", handoff.RemoteReleaseGateDetail)
	}
	if slices.Contains(handoff.Evidence, "remote release-gate jobs steps=[] recorded") {
		t.Fatalf("policy-only steps=[] should not become remote evidence: %+v", handoff.Evidence)
	}
	cadence := handoff.ReleaseInspectionCadence
	if cadence.State != "implementation-pending" || cadence.ImplementationCommitReady || cadence.InspectionCommitReady || !strings.Contains(cadence.NextAction, "implementation commit") {
		t.Fatalf("unexpected pre-commit cadence for policy-only remote text: %+v", cadence)
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
	if handoff.RemoteReleaseGateDetail == nil || handoff.RemoteReleaseGateDetail.State != "blocked: completed failure with jobs steps=[]" || !handoff.RemoteReleaseGateDetail.EmptySteps || !handoff.RemoteReleaseGateDetail.CompletedFailure || handoff.RemoteReleaseGateDetail.CanClaimGreen {
		t.Fatalf("unexpected remote gate detail: %+v", handoff.RemoteReleaseGateDetail)
	}
	for _, want := range []string{"123456789"} {
		if !slices.Contains(handoff.RemoteReleaseGateDetail.RunRefs, want) {
			t.Fatalf("remote gate detail run refs missing %q: %+v", want, handoff.RemoteReleaseGateDetail.RunRefs)
		}
	}
	for _, want := range []string{"Linux", "Windows", "macOS"} {
		if !slices.Contains(handoff.RemoteReleaseGateDetail.Jobs, want) {
			t.Fatalf("remote gate detail jobs missing %q: %+v", want, handoff.RemoteReleaseGateDetail.Jobs)
		}
	}
	if !releaseHandoffStringsContain(handoff.RemoteReleaseGateDetail.Boundary, "do not claim remote CI green") {
		t.Fatalf("remote gate detail boundary missing no-green guard: %+v", handoff.RemoteReleaseGateDetail.Boundary)
	}
}

func TestLatestBatchReleaseReadinessIgnoresBoundaryOnlyCommitAndInspectionWords(t *testing.T) {
	section := `状态：已完成 runtime/test/doc 工作树实现与完整本地 release minimum；尚未创建本批代码提交，尚未检查本批对应的远程 workflow run。

目标：正常批次最多两次 push：implementation commit/push 和 release inspection commit/push；若远程 release-gate 后续仍为 jobs ` + "`" + `steps=[]` + "`" + `，只记录既有 blocker，do not add a third inspection record。

验证结果：完整本地 release minimum 已通过：` + "`" + `go run ./cmd/rekit -- -Command release-check -Format json` + "`" + ` 返回 ` + "`" + `ready=true` + "`" + `，` + "`" + `go run ./cmd/rekit -- -Command status` + "`" + `、` + "`" + `go run ./cmd/rekit -- -Command packs` + "`" + `、` + "`" + `go run ./cmd/rekit -- -Command doctor` + "`" + `、` + "`" + `go test ./...` + "`" + `、` + "`" + `go vet ./...` + "`" + ` 与 ` + "`" + `git diff --check` + "`" + ` 均通过。`

	handoff := latestBatchHandoff(ReleaseHandoffLatestBatch{Status: "已完成 fixture"}, section)
	if handoff.RemoteReleaseGate != "not-recorded" || handoff.RemoteReleaseGateDetail == nil || handoff.RemoteReleaseGateDetail.EmptySteps || len(handoff.RemoteReleaseGateDetail.RunRefs) != 0 || len(handoff.RemoteReleaseGateDetail.Jobs) != 0 {
		t.Fatalf("boundary-only remote words should not become remote evidence: %+v", handoff.RemoteReleaseGateDetail)
	}
	cadence := handoff.ReleaseInspectionCadence
	if cadence.State != "implementation-pending" || cadence.ImplementationCommitReady || cadence.InspectionCommitReady || releaseHandoffStringsContain(cadence.Evidence, "implementation commit/push recorded") || releaseHandoffStringsContain(cadence.Evidence, "release inspection commit/run recorded") || releaseHandoffStringsContain(cadence.Evidence, "remote release-gate steps=[] blocker recorded") {
		t.Fatalf("boundary-only commit/inspection words should not become cadence evidence: %+v", cadence)
	}
}

func TestLatestBatchReleaseReadinessWaitsForRemoteInspectionAfterImplementationPush(t *testing.T) {
	section := `状态：已完成 runtime/test/doc 工作树实现、完整本地 release minimum 与 implementation commit/push；implementation commit ` + "`" + `abc123d` + "`" + ` 已推送。尚未检查本批对应的远程 workflow run。

目标：release inspection commit/push 只记录 implementation run；若后续 CI 为 jobs ` + "`" + `steps=[]` + "`" + `，按既有 blocker 处理，不要追加第三个记录提交。

验证结果：完整本地 release minimum 已通过：` + "`" + `go run ./cmd/rekit -- -Command release-check -Format json` + "`" + ` 返回 ` + "`" + `ready=true` + "`" + `，` + "`" + `go run ./cmd/rekit -- -Command status` + "`" + `、` + "`" + `go run ./cmd/rekit -- -Command packs` + "`" + `、` + "`" + `go run ./cmd/rekit -- -Command doctor` + "`" + `、` + "`" + `go test ./...` + "`" + `、` + "`" + `go vet ./...` + "`" + ` 与 ` + "`" + `git diff --check` + "`" + ` 均通过。`

	handoff := latestBatchHandoff(ReleaseHandoffLatestBatch{Status: "已完成 fixture"}, section)
	if handoff.RemoteReleaseGate != "not-recorded" || handoff.RemoteReleaseGateDetail == nil || handoff.RemoteReleaseGateDetail.EmptySteps || slices.Contains(handoff.Evidence, "remote release-gate jobs steps=[] recorded") {
		t.Fatalf("remote inspection should stay not-recorded before run evidence: %+v evidence=%+v", handoff.RemoteReleaseGateDetail, handoff.Evidence)
	}
	cadence := handoff.ReleaseInspectionCadence
	if cadence.State != "inspection-pending" || !cadence.ImplementationCommitReady || cadence.InspectionCommitReady || !releaseHandoffStringsContain(cadence.Evidence, "implementation commit/push recorded") || releaseHandoffStringsContain(cadence.Evidence, "release inspection commit/run recorded") {
		t.Fatalf("implementation push should wait for scoped remote inspection evidence: %+v", cadence)
	}
}

func TestLatestBatchReleaseReadinessPrefersExplicitRemoteEvidenceOverStalePendingText(t *testing.T) {
	section := `状态：已完成 runtime/test/doc 工作树实现、完整本地 release minimum、implementation commit/push 与 implementation remote release-gate inspection；implementation commits ` + "`" + `abc123d` + "`" + ` / ` + "`" + `def456a` + "`" + ` 已推送并进入 PR #10。PR run ` + "`" + `123456789` + "`" + ` completed failure，Linux/macOS/Windows jobs ` + "`" + `111111111` + "`" + `/` + "`" + `222222222` + "`" + `/` + "`" + `333333333` + "`" + ` 均 ` + "`" + `steps=[]` + "`" + `、` + "`" + `runner_id=0` + "`" + ` 且无 logs，仍属既有 runner/billing blocker。

验证结果：完整本地 release minimum 已通过；read-only status 同时验证：本批在尚未创建代码提交/尚未检查远程 workflow run 时应保持 ` + "`" + `implementation-pending` + "`" + ` / ` + "`" + `remoteReleaseGate=not-recorded` + "`" + `。`

	handoff := latestBatchHandoff(ReleaseHandoffLatestBatch{Status: "已完成 fixture"}, section)
	if handoff.RemoteReleaseGate != "blocked: completed failure with jobs steps=[]" || handoff.RemoteReleaseGateDetail == nil || !handoff.RemoteReleaseGateDetail.EmptySteps || !handoff.RemoteReleaseGateDetail.CompletedFailure {
		t.Fatalf("explicit remote evidence should win over stale pending wording: %+v", handoff.RemoteReleaseGateDetail)
	}
	cadence := handoff.ReleaseInspectionCadence
	if cadence.State != "complete" || !cadence.ImplementationCommitReady || !cadence.InspectionCommitReady || !releaseHandoffStringsContain(cadence.Evidence, "release inspection commit/run recorded") || !releaseHandoffStringsContain(cadence.Evidence, "remote release-gate steps=[] blocker recorded") {
		t.Fatalf("explicit remote evidence should complete cadence: %+v", cadence)
	}
}

func TestReleaseHandoffPackMemoryCandidatesDetectsOpenResidue(t *testing.T) {
	repo := t.TempDir()
	candidatePath := filepath.Join(repo, "packs", "fixture", "promote-candidates", "candidate.candidate.md")
	toolingPath := filepath.Join(repo, "packs", "fixture", "tooling", "candidates", "tool.candidate.md")
	proofRoot := filepath.Join(repo, "packs", "fixture", "promote-candidates", "review-artifacts")
	proofPath := filepath.Join(proofRoot, "candidate.candidate-decision-note.md")
	writeFile(t, candidatePath, "# candidate\n")
	writeFile(t, toolingPath, "# tooling\n")
	writeFile(t, filepath.Join(repo, "packs", "fixture", "promote-candidates", "index.json"), `[
  {
    "path": "references/template/README.md",
    "candidate": "candidate.candidate.md"
  }
]
`)
	writeFile(t, proofPath, "# decision\n")
	looseProof := releaseHandoffPackMemoryCandidates(repo, []manifest.PackSummary{{ID: "fixture", Maturity: "skeleton"}})
	if looseProof.Ready || looseProof.Summary != "pack-memory candidate inventory has warnings" || len(looseProof.Warnings) == 0 || !strings.Contains(fmt.Sprint(looseProof.Warnings), "decode candidate decision proof") {
		t.Fatalf("loose candidate decision proof was not rejected: %+v", looseProof)
	}
	evidencePath := filepath.Join(proofRoot, "candidate.review-evidence.md")
	writeFile(t, evidencePath, "reviewed decision evidence\n")
	writeCandidateDecisionProof(t, repo, proofPath, "fixture", "packet-hash", candidatePath, filepath.Join(repo, "packs", "fixture", "references", "template", "README.md"), "reject", "managed-doc", evidencePath)

	inventory := releaseHandoffPackMemoryCandidates(repo, []manifest.PackSummary{{ID: "fixture", Maturity: "skeleton"}})
	if inventory.Ready || inventory.Summary != "pack-memory candidate inventory has open review/cleanup/verification work" || inventory.Total != 3 || len(inventory.Packs) != 1 || !strings.Contains(inventory.NextAction, "review listed pack-memory candidates") || len(inventory.Warnings) == 0 {
		t.Fatalf("unexpected pack-memory candidate inventory: %+v", inventory)
	}
	if inventory.MissionCommanderActionQueue.CurrentAction == nil || inventory.MissionCommanderActionQueue.Counts.Total != 1 || inventory.MissionCommanderActionQueue.Counts.RequiresReview != 1 || len(inventory.MissionCommanderNextActions) != 1 || inventory.MissionCommanderNextActions[0].Label != "fixture" || inventory.MissionCommanderNextActions[0].State != "pack-memory-proof-required" || !strings.Contains(inventory.MissionCommanderNextActions[0].Command, "-DraftReviewProof") || !strings.Contains(inventory.MissionCommanderNextActions[0].Command, "-ProofDecision") || !releaseHandoffStringsContain(inventory.MissionCommanderNextActions[0].Boundary, "read-only handoff") {
		t.Fatalf("pack-memory candidate action queue omitted current proof handoff: %+v", inventory.MissionCommanderActionQueue)
	}
	pack := inventory.Packs[0]
	if pack.Pack != "fixture" || pack.CandidateRoot != "packs/fixture/promote-candidates" || pack.ToolingRoot != "packs/fixture/tooling/candidates" || pack.IndexPath != "packs/fixture/promote-candidates/index.json" || pack.CandidateFiles != 1 || pack.ToolingFiles != 1 || pack.IndexEntries != 1 || !pack.HasOpenWork || !pack.RequiresReview || !pack.RequiresCleanup {
		t.Fatalf("unexpected pack-memory candidate status: %+v", pack)
	}
	if !slices.Contains(pack.CandidatePaths, "packs/fixture/promote-candidates/candidate.candidate.md") || !slices.Contains(pack.ToolingPaths, "packs/fixture/tooling/candidates/tool.candidate.md") || len(pack.IndexCandidates) != 1 || pack.IndexCandidates[0].Candidate != "packs/fixture/promote-candidates/candidate.candidate.md" || pack.IndexCandidates[0].Path != "references/template/README.md" {
		t.Fatalf("pack-memory candidate identity handoff drifted: %+v", pack)
	}
	if !releaseHandoffReviewArtifactContains(pack.ReviewArtifacts, "candidate-decision-note", "packs/fixture/promote-candidates/candidate.candidate.md", "references/template/README.md") || !releaseHandoffReviewArtifactContains(pack.ReviewArtifacts, "candidate-cleanup-proof", "packs/fixture/promote-candidates/candidate.candidate.md", "packs/fixture/references/template/README.md") || !releaseHandoffReviewArtifactContains(pack.ReviewArtifacts, "fresh-case-reconsume-proof", "packs/fixture/tooling/candidates/tool.candidate.md", "packs/fixture/tooling") || !releaseHandoffReviewArtifactContains(pack.ReviewArtifacts, "attached-case-reconsume-proof", "packs/fixture/tooling/candidates/tool.candidate.md", "packs/fixture/tooling") {
		t.Fatalf("pack-memory candidate review artifact handoff drifted: %+v", pack.ReviewArtifacts)
	}
	if summary := pack.ReviewSummary; summary.Total != 3 || summary.CandidateFiles != 1 || summary.ToolingFiles != 1 || summary.IndexEntries != 1 || summary.ReviewArtifactCount != len(pack.ReviewArtifacts) || summary.DecisionArtifactCount != 2 || summary.CleanupArtifactCount != 2 || summary.ReconsumeArtifactCount != 4 || summary.ProofSummary.Total != len(pack.ReviewArtifacts) || summary.ProofSummary.Present != 1 || summary.ProofSummary.Missing != 7 || summary.ProofSummary.DecisionPresent != 1 || summary.ProofSummary.DecisionMissing != 1 || summary.ProofSummary.CleanupMissing != 2 || summary.ProofSummary.ReconsumeMissing != 4 || summary.ProofSummary.ProofProgress != "1/8" || summary.ProofSummary.CurrentStage != "decision-proof-required" || summary.ProofSummary.NextMissingProofType != "candidate-decision-note" || summary.ProofSummary.NextMissingProofPath != "packs/fixture/promote-candidates/review-artifacts/tool.candidate-decision-note.md" || summary.ProofSummary.NextMissingCandidatePath != "packs/fixture/tooling/candidates/tool.candidate.md" || summary.ProofSummary.NextMissingPackTarget != "packs/fixture/tooling" || summary.ProofSummary.NextMissingProof == nil || summary.ProofSummary.NextMissingProof.ProofType != "candidate-decision-note" || summary.ProofSummary.NextMissingProof.Stage != "decision-proof-required" || !summary.ProofSummary.NextMissingProof.RequiresPacket || !summary.ProofSummary.NextMissingProof.RequiresExplicitReview || !strings.Contains(summary.ProofSummary.NextMissingProof.DraftCommand, "-DraftReviewProof") || !strings.Contains(summary.ProofSummary.NextMissingProof.DraftApplyTemplate, "<proofSha256-from-WhatIf>") || !strings.Contains(summary.ProofSummary.NextMissingProof.Action, "selected decisionFollowThrough outcome") || !releaseHandoffStringsContain(summary.ProofSummary.NextMissingProof.Evidence, "decision note path/ref") || summary.ProofSummary.Complete || summary.ProofSummary.ProofRoot != "packs/fixture/promote-candidates/review-artifacts" || !strings.Contains(summary.ProofSummary.NextAction, "candidate-decision-note") || !releaseHandoffStringsContain(summary.ProofSummary.Boundary, "read-only") || !summary.RequiresReview || !summary.RequiresCleanup || !summary.HasCandidatePaths || !summary.HasToolingPaths || !summary.HasIndex || !summary.HasDecisionArtifacts || !summary.HasCleanupArtifacts || !summary.HasReconsumeArtifacts || !strings.Contains(summary.NextAction, "record accept/reject") || !releaseHandoffStringsContain(summary.Boundary, "reviewSummary is read-only") {
		t.Fatalf("pack-memory candidate review summary drifted: %+v", summary)
	}
	if pack.ProofRoot != "packs/fixture/promote-candidates/review-artifacts" || pack.ProofSummary.Total != len(pack.ReviewArtifacts) || pack.ProofSummary.Present != 1 || pack.ProofSummary.Missing != 7 || pack.ProofSummary.ProofProgress != "1/8" || pack.ProofSummary.CurrentStage != "decision-proof-required" || pack.ProofSummary.NextMissingProofPath != "packs/fixture/promote-candidates/review-artifacts/tool.candidate-decision-note.md" || pack.ProofSummary.NextMissingProof == nil || !strings.Contains(pack.ProofSummary.NextMissingProof.Format, "decision") || !strings.Contains(pack.ProofSummary.NextMissingProof.DraftCommand, "-ProofDecision") || pack.ProofSummary.Complete {
		t.Fatalf("pack-memory candidate proof summary drifted: %+v", pack.ProofSummary)
	}
	if !releaseHandoffReviewArtifactProofContains(pack.ReviewArtifacts, "candidate-decision-note", "packs/fixture/promote-candidates/candidate.candidate.md", "packs/fixture/promote-candidates/review-artifacts/candidate.candidate-decision-note.md", true) || !releaseHandoffReviewArtifactProofContains(pack.ReviewArtifacts, "candidate-cleanup-proof", "packs/fixture/promote-candidates/candidate.candidate.md", "packs/fixture/promote-candidates/review-artifacts/candidate.candidate-cleanup-proof.md", false) {
		t.Fatalf("pack-memory candidate proof handoff drifted: %+v", pack.ReviewArtifacts)
	}
	writeFile(t, filepath.Join(proofRoot, "candidate.candidate-cleanup-proof.md"), "# cleanup\n")
	looseCleanupProof := releaseHandoffPackMemoryCandidates(repo, []manifest.PackSummary{{ID: "fixture", Maturity: "skeleton"}})
	if looseCleanupProof.Ready || looseCleanupProof.Summary != "pack-memory candidate inventory has warnings" || len(looseCleanupProof.Warnings) == 0 || !strings.Contains(fmt.Sprint(looseCleanupProof.Warnings), "decode candidate lifecycle proof") {
		t.Fatalf("loose open candidate cleanup proof was not rejected: %+v", looseCleanupProof)
	}
	writeFile(t, filepath.Join(proofRoot, "candidate.cleanup-evidence.md"), "candidate absent and index entry absent\n")
	if err := os.Remove(candidatePath); err != nil {
		t.Fatal(err)
	}
	writeFile(t, filepath.Join(repo, "packs", "fixture", "promote-candidates", "index.json"), `[]
`)
	writeCandidateLifecycleProof(t, repo, filepath.Join(proofRoot, "candidate.candidate-cleanup-proof.md"), "fixture", "candidate-cleanup-proof", candidatePath, filepath.Join(repo, "packs", "fixture", "references", "template", "README.md"), []map[string]any{{"name": "candidate-absent", "status": "passed", "summary": "candidate path is absent"}, {"name": "index-entry-absent", "status": "passed", "summary": "index no longer references candidate"}}, filepath.Join(proofRoot, "candidate.cleanup-evidence.md"))
	cleanupInventory := releaseHandoffPackMemoryCandidates(repo, []manifest.PackSummary{{ID: "fixture", Maturity: "skeleton"}})
	if cleanupInventory.Ready || cleanupInventory.Summary != "pack-memory candidate inventory has open review/cleanup/verification work" || len(cleanupInventory.Packs) != 1 || cleanupInventory.Packs[0].ProofSummary.Present != 0 || cleanupInventory.Packs[0].ProofSummary.CleanupPresent != 0 || cleanupInventory.Packs[0].ProofSummary.CurrentStage != "decision-proof-required" || len(cleanupInventory.Packs[0].CandidatePaths) != 0 {
		t.Fatalf("unexpected post-cleanup candidate inventory: %+v", cleanupInventory)
	}
	writeFile(t, filepath.Join(proofRoot, "tool.candidate-decision-note.md"), "# tooling decision\n")
	looseToolingProof := releaseHandoffPackMemoryCandidates(repo, []manifest.PackSummary{{ID: "fixture", Maturity: "skeleton"}})
	if looseToolingProof.Ready || looseToolingProof.Summary != "pack-memory candidate inventory has warnings" || len(looseToolingProof.Warnings) == 0 || !strings.Contains(fmt.Sprint(looseToolingProof.Warnings), "decode candidate decision proof") {
		t.Fatalf("loose tooling decision proof was not rejected: %+v", looseToolingProof)
	}
	writeFile(t, filepath.Join(proofRoot, "tool.decision-evidence.md"), "tooling rejected\n")
	writeCandidateDecisionProof(t, repo, filepath.Join(proofRoot, "tool.candidate-decision-note.md"), "fixture", "packet-hash", toolingPath, filepath.Join(repo, "packs", "fixture", "tooling"), "reject", "tooling-candidate-source", filepath.Join(proofRoot, "tool.decision-evidence.md"))
	writeFile(t, filepath.Join(proofRoot, "tool.pack-doctor-output.md"), "# doctor\n")
	looseDoctorProof := releaseHandoffPackMemoryCandidates(repo, []manifest.PackSummary{{ID: "fixture", Maturity: "skeleton"}})
	if looseDoctorProof.Ready || looseDoctorProof.Summary != "pack-memory candidate inventory has warnings" || len(looseDoctorProof.Warnings) == 0 || !strings.Contains(fmt.Sprint(looseDoctorProof.Warnings), "decode candidate lifecycle proof") {
		t.Fatalf("loose doctor proof was not rejected: %+v", looseDoctorProof)
	}
	writeFile(t, filepath.Join(proofRoot, "tool.doctor-evidence.md"), "doctor passed\n")
	writeCandidateLifecycleProof(t, repo, filepath.Join(proofRoot, "tool.pack-doctor-output.md"), "fixture", "pack-doctor-output", toolingPath, "packs/fixture/tooling", []map[string]any{{"name": "pack-doctor", "status": "passed", "summary": "doctor passed"}}, filepath.Join(proofRoot, "tool.doctor-evidence.md"))
	writeFile(t, filepath.Join(proofRoot, "tool.fresh-case-reconsume-proof.md"), "# fresh\n")
	looseFreshProof := releaseHandoffPackMemoryCandidates(repo, []manifest.PackSummary{{ID: "fixture", Maturity: "skeleton"}})
	if looseFreshProof.Ready || looseFreshProof.Summary != "pack-memory candidate inventory has warnings" || len(looseFreshProof.Warnings) == 0 || !strings.Contains(fmt.Sprint(looseFreshProof.Warnings), "decode candidate lifecycle proof") {
		t.Fatalf("loose fresh reconsume proof was not rejected: %+v", looseFreshProof)
	}
	writeFile(t, filepath.Join(proofRoot, "tool.fresh-evidence.md"), "fresh reconsume passed\n")
	writeCandidateLifecycleProof(t, repo, filepath.Join(proofRoot, "tool.fresh-case-reconsume-proof.md"), "fixture", "fresh-case-reconsume-proof", toolingPath, "packs/fixture/tooling", []map[string]any{{"name": "fresh-case-reconsume", "status": "passed", "summary": "fresh case doctor passed"}, {"name": "pack-doctor", "status": "passed", "summary": "pack doctor passed"}}, filepath.Join(proofRoot, "tool.fresh-evidence.md"))
	writeFile(t, filepath.Join(proofRoot, "tool.attached-case-reconsume-proof.md"), "# attached\n")
	looseAttachedProof := releaseHandoffPackMemoryCandidates(repo, []manifest.PackSummary{{ID: "fixture", Maturity: "skeleton"}})
	if looseAttachedProof.Ready || looseAttachedProof.Summary != "pack-memory candidate inventory has warnings" || len(looseAttachedProof.Warnings) == 0 || !strings.Contains(fmt.Sprint(looseAttachedProof.Warnings), "decode candidate lifecycle proof") {
		t.Fatalf("loose attached reconsume proof was not rejected: %+v", looseAttachedProof)
	}
	writeFile(t, filepath.Join(proofRoot, "tool.attached-evidence.md"), "attached reconsume passed\n")
	writeCandidateLifecycleProof(t, repo, filepath.Join(proofRoot, "tool.attached-case-reconsume-proof.md"), "fixture", "attached-case-reconsume-proof", toolingPath, "packs/fixture/tooling", []map[string]any{{"name": "attached-case-reconsume", "status": "passed", "summary": "attached case doctor passed"}, {"name": "pack-doctor", "status": "passed", "summary": "pack doctor passed"}}, filepath.Join(proofRoot, "tool.attached-evidence.md"))
	lifecycleInventory := releaseHandoffPackMemoryCandidates(repo, []manifest.PackSummary{{ID: "fixture", Maturity: "skeleton"}})
	if lifecycleInventory.Ready || len(lifecycleInventory.Packs) != 1 || lifecycleInventory.Packs[0].ProofSummary.Present != 4 || lifecycleInventory.Packs[0].ProofSummary.DecisionPresent != 1 || lifecycleInventory.Packs[0].ProofSummary.CleanupPresent != 0 || lifecycleInventory.Packs[0].ProofSummary.ReconsumePresent != 3 || lifecycleInventory.Packs[0].ProofSummary.CurrentStage != "cleanup-proof-required" {
		t.Fatalf("strict lifecycle proofs were not accepted: %+v", lifecycleInventory)
	}
	writeFile(t, filepath.Join(proofRoot, "tool.candidate-cleanup-proof.md"), "# tooling cleanup\n")
	looseToolingCleanupProof := releaseHandoffPackMemoryCandidates(repo, []manifest.PackSummary{{ID: "fixture", Maturity: "skeleton"}})
	if looseToolingCleanupProof.Ready || looseToolingCleanupProof.Summary != "pack-memory candidate inventory has warnings" || len(looseToolingCleanupProof.Warnings) == 0 || !strings.Contains(fmt.Sprint(looseToolingCleanupProof.Warnings), "decode candidate lifecycle proof") {
		t.Fatalf("loose tooling cleanup proof was not rejected: %+v", looseToolingCleanupProof)
	}
	if handoff := pack.DecisionDraftHandoff; handoff == nil || handoff.Mode != "candidate-decision-draft-review-workspace-required" || handoff.DecisionPath != "packs/fixture/promote-candidates/review-artifacts/candidate-decisions.json" || !slices.Contains(handoff.EvidenceRefs, "packs/fixture/promote-candidates/review-artifacts/candidate.candidate-decision-note.md") || !slices.Contains(handoff.SupportedDecisions, "accept-managed-reject-tooling") || !strings.Contains(handoff.NextAction, "promote -CreateCandidates -Review") || !releaseHandoffStringsContain(handoff.Boundary, "cannot infer the case-local review packet") {
		t.Fatalf("pack-memory candidate decision draft handoff drifted: %+v", pack.DecisionDraftHandoff)
	}
	for _, evidence := range []string{"promote-candidates files=1", "tooling/candidates files=1", "indexPath packs/fixture/promote-candidates/index.json entries=1"} {
		if !slices.Contains(pack.Evidence, evidence) {
			t.Fatalf("pack-memory candidate evidence missing %q: %+v", evidence, pack.Evidence)
		}
	}
	if !releaseHandoffStringsContain(pack.Boundary, "does not merge, delete") || !releaseHandoffStringsContain(releaseHandoffPackMemoryCandidateDetails(inventory), "pack=fixture") {
		t.Fatalf("pack-memory candidate handoff omitted boundary/detail: pack=%+v details=%+v", pack, releaseHandoffPackMemoryCandidateDetails(inventory))
	}
}

func TestReleaseHandoffPackMemoryCandidateDecisionVerificationReceipt(t *testing.T) {
	repo := t.TempDir()
	proofRoot := filepath.Join(repo, "packs", "fixture", "promote-candidates", "review-artifacts")
	receiptPath := filepath.Join(proofRoot, "fixture.candidate-decision-receipt.json")
	proofPath := filepath.Join(proofRoot, "fixture.candidate-verification-proof.json")
	packetPath := filepath.Join(repo, "case", ".rekit", "reviews", "packet.json")
	decisionPath := filepath.Join(repo, "case", ".rekit", "reviews", "decisions.json")
	packetHash := "packet-hash"
	decisionHash := "decision-hash"
	backupRoot := filepath.Join(repo, "packs", "fixture", "promote-candidates", ".decision-backup", "fixture")
	writeFile(t, filepath.Join(backupRoot, "committed.json"), "{\"applied\":true}\n")
	caseRoot := filepath.Join(repo, "case")
	freshCaseRoot := filepath.Join(repo, "fresh-case")
	attachedCaseRoot := filepath.Join(repo, "attached-case")
	candidateRoot := filepath.Dir(proofRoot)
	candidatePath := filepath.Join(candidateRoot, "memory.candidate.md")
	candidateBackupPath := filepath.Join(backupRoot, "actions", "000", "candidate")
	actions := []map[string]any{{
		"candidatePath":       candidatePath,
		"kind":                "managed-doc",
		"decision":            "accept",
		"packTarget":          filepath.Join(repo, "packs", "fixture", "memory.md"),
		"action":              "replace pack target with reviewed candidate",
		"candidateBackupPath": candidateBackupPath,
		"evidenceRefs":        []string{},
	}}
	receipt := map[string]any{
		"schemaVersion":                1,
		"kind":                         "pack-memory-candidate-decision-receipt",
		"pack":                         "fixture",
		"repoRoot":                     repo,
		"caseRoot":                     caseRoot,
		"packetPath":                   packetPath,
		"decisionPath":                 decisionPath,
		"packetHash":                   "packet-hash",
		"decisionHash":                 "decision-hash",
		"backupRoot":                   backupRoot,
		"indexPath":                    filepath.Join(candidateRoot, "index.json"),
		"accepted":                     1,
		"rejected":                     0,
		"superseded":                   0,
		"actions":                      actions,
		"decisionEvidence":             []string{},
		"receiptPath":                  receiptPath,
		"verificationPending":          true,
		"verificationWorkspaceRoot":    filepath.Join(caseRoot, ".rekit", "verifications", "candidate-decisions", shortReleaseHandoffHash(packetHash+decisionHash)),
		"verificationProvisionCommand": "/rekit promote -PacketPath " + packetPath + " -CandidateDecisionPath " + decisionPath + " -ProvisionCandidateVerificationCases -FreshCaseRoot <workspace>/fresh -AttachedCaseRoot <workspace>/attached -WhatIf -Format json",
		"verificationCommand":          "/rekit promote -VerifyCandidateDecision -FreshCaseRoot <workspace>/fresh -AttachedCaseRoot <workspace>/attached -WhatIf -Format json",
		"verificationProofPath":        proofPath,
		"boundary":                     []string{"fixture boundary"},
	}
	data, err := json.MarshalIndent(receipt, "", "  ")
	if err != nil {
		t.Fatal(err)
	}
	writeFile(t, receiptPath, string(data)+"\n")

	inventory := releaseHandoffPackMemoryCandidates(repo, []manifest.PackSummary{{ID: "fixture", Maturity: "skeleton"}})
	if inventory.Ready || inventory.Total != 1 || len(inventory.Packs) != 1 {
		t.Fatalf("pending candidate verification was not projected: %+v", inventory)
	}
	pack := inventory.Packs[0]
	receiptStatus := pack.DecisionReceipts[0]
	if pack.PendingVerifications != 1 || pack.CompletedVerifications != 0 || !pack.RequiresVerification || pack.RequiresReview || !pack.RequiresCleanup || len(pack.DecisionReceipts) != 1 || len(receiptStatus.Actions) != 1 || receiptStatus.Actions[0].CandidatePath != "packs/fixture/promote-candidates/memory.candidate.md" || receiptStatus.VerificationComplete || receiptStatus.VerificationWorkspaceRoot == "" || !strings.Contains(receiptStatus.VerificationProvisionCommand, "-ProvisionCandidateVerificationCases") || !strings.Contains(receiptStatus.VerificationCommand, "-FreshCaseRoot") || receiptStatus.ProvisionStatus != "required" || receiptStatus.ProvisionIntentPath == "" || receiptStatus.ProvisionReceiptPath == "" || receiptStatus.ProvisionApplyCommand == "" || receiptStatus.ProvisionInProgress || receiptStatus.ProvisionComplete || !strings.Contains(receiptStatus.ProvisionNextAction, "verificationProvisionCommand") || !strings.Contains(pack.Action, "candidate-cleanup-proof") || pack.ProofSummary.NextMissingProof == nil || !pack.ProofSummary.NextMissingProof.RequiresCandidateDecision || !strings.Contains(pack.ProofSummary.NextMissingProof.DraftCommand, "-ProofType candidate-cleanup-proof") || !strings.Contains(pack.ProofSummary.NextMissingProof.DraftApplyTemplate, "<proofSha256-from-WhatIf>") {
		t.Fatalf("pending candidate verification handoff drifted: %+v", pack)
	}

	workspace := filepath.Join(caseRoot, ".rekit", "verifications", "candidate-decisions", shortReleaseHandoffHash(packetHash+decisionHash))
	freshCaseRoot = filepath.Join(workspace, "fresh")
	attachedCaseRoot = filepath.Join(workspace, "attached")
	provisionIntentPath := filepath.Join(workspace, "provision.intent.json")
	provisionReceiptPath := filepath.Join(workspace, "provision.receipt.json")
	writeFile(t, provisionIntentPath, "{\"kind\":\"intent\"}\n")
	writeFile(t, provisionReceiptPath, "{\"kind\":\"receipt\"}\n")
	if err := os.MkdirAll(filepath.Join(workspace, "fresh"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(filepath.Join(workspace, "attached"), 0o755); err != nil {
		t.Fatal(err)
	}
	retirementPreviewCommand := "/rekit promote -PacketPath " + packetPath + " -CandidateDecisionPath " + decisionPath + " -RetireCandidateVerificationWorkspace -WhatIf -Format json"
	proof := map[string]any{
		"schemaVersion":            1,
		"kind":                     "pack-memory-candidate-decision-verification",
		"pack":                     "fixture",
		"caseRoot":                 caseRoot,
		"freshCaseRoot":            freshCaseRoot,
		"attachedCaseRoot":         attachedCaseRoot,
		"packetHash":               packetHash,
		"decisionHash":             decisionHash,
		"receiptHash":              sha256ReleaseHandoff(append(data, '\n')),
		"receiptPath":              receiptPath,
		"verificationProofPath":    proofPath,
		"provisionIntentPath":      provisionIntentPath,
		"provisionIntentSha256":    fileSHA256ReleaseHandoff(provisionIntentPath),
		"provisionReceiptPath":     provisionReceiptPath,
		"provisionReceiptSha256":   fileSHA256ReleaseHandoff(provisionReceiptPath),
		"retirementPreviewCommand": retirementPreviewCommand,
		"isMutation":               true,
		"applied":                  true,
		"ready":                    true,
		"packDoctorRows":           1,
		"freshDoctorRows":          1,
		"attachedDoctorRows":       1,
		"verifiedActions":          actions,
		"nextSteps":                []string{"rerun release-check"},
		"boundary":                 []string{"fixture boundary"},
	}
	assertProofRejected := func(name, content string) {
		t.Helper()
		writeFile(t, proofPath, content)
		invalid := releaseHandoffPackMemoryCandidates(repo, []manifest.PackSummary{{ID: "fixture", Maturity: "skeleton"}})
		if invalid.Ready || invalid.Summary != "pack-memory candidate inventory has warnings" || len(invalid.Warnings) == 0 {
			t.Fatalf("%s verification proof was not rejected: %+v", name, invalid)
		}
	}
	assertProofRejected("malformed", "{\"ready\":true}\n")
	proof["packetHash"] = "wrong-packet-hash"
	wrongHashData, err := json.MarshalIndent(proof, "", "  ")
	if err != nil {
		t.Fatal(err)
	}
	assertProofRejected("wrong hash", string(wrongHashData)+"\n")
	proof["packetHash"] = "packet-hash"
	proof["receiptPath"] = filepath.Join(proofRoot, "wrong-receipt.json")
	wrongReceiptData, err := json.MarshalIndent(proof, "", "  ")
	if err != nil {
		t.Fatal(err)
	}
	assertProofRejected("wrong receipt", string(wrongReceiptData)+"\n")
	proof["receiptPath"] = receiptPath
	proof["verifiedActions"] = []map[string]any{{
		"candidatePath": candidatePath,
		"kind":          "managed-doc",
		"decision":      "reject",
		"action":        "reject candidate",
		"evidenceRefs":  []string{},
	}}
	wrongActionsData, err := json.MarshalIndent(proof, "", "  ")
	if err != nil {
		t.Fatal(err)
	}
	assertProofRejected("wrong actions", string(wrongActionsData)+"\n")
	proof["verifiedActions"] = actions
	proofData, err := json.MarshalIndent(proof, "", "  ")
	if err != nil {
		t.Fatal(err)
	}
	writeFile(t, proofPath, string(proofData)+"\n")
	retirementRequired := releaseHandoffPackMemoryCandidates(repo, []manifest.PackSummary{{ID: "fixture", Maturity: "skeleton"}})
	if retirementRequired.Ready || retirementRequired.Total != 1 || len(retirementRequired.Packs) != 1 {
		t.Fatalf("completed candidate verification did not require retirement: %+v", retirementRequired)
	}
	retirement := retirementRequired.Packs[0].DecisionReceipts[0]
	if retirement.RetirementStatus != "required" || !retirement.RetirementRequired || retirement.RetirementInProgress || retirement.Retired || retirement.RetirementPreviewCommand != retirementPreviewCommand || !strings.Contains(retirement.RetirementNextAction, "expected-hash Apply") {
		t.Fatalf("required candidate verification retirement handoff drifted: %+v", retirement)
	}
}

func TestReleaseHandoffPackMemoryCandidateVerificationRetirementLifecycle(t *testing.T) {
	repo := t.TempDir()
	proofRoot := filepath.Join(repo, "packs", "fixture", "promote-candidates", "review-artifacts")
	candidateRoot := filepath.Dir(proofRoot)
	caseRoot := filepath.Join(repo, "case")
	packetPath := filepath.Join(caseRoot, ".rekit", "reviews", "packet.json")
	decisionPath := filepath.Join(caseRoot, ".rekit", "reviews", "decisions.json")
	packetHash := "packet-hash"
	decisionHash := "decision-hash"
	retirementID := shortReleaseHandoffHash(packetHash + decisionHash)
	workspace := filepath.Join(caseRoot, ".rekit", "verifications", "candidate-decisions", retirementID)
	freshRoot := filepath.Join(workspace, "fresh")
	attachedRoot := filepath.Join(workspace, "attached")
	provisionIntentPath := filepath.Join(workspace, "provision.intent.json")
	provisionReceiptPath := filepath.Join(workspace, "provision.receipt.json")
	proofPath := filepath.Join(proofRoot, "fixture.candidate-verification-proof.json")
	receiptPath := filepath.Join(proofRoot, "fixture.candidate-decision-receipt.json")
	retirementIntentPath := filepath.Join(proofRoot, retirementID+".candidate-verification-retirement-intent.json")
	retirementReceiptPath := filepath.Join(proofRoot, retirementID+".candidate-verification-retirement-receipt.json")
	backupRoot := filepath.Join(candidateRoot, ".decision-backup", "fixture")
	writeFile(t, filepath.Join(backupRoot, "committed.json"), "{\"applied\":true}\n")
	writeFile(t, provisionIntentPath, "{\"kind\":\"intent\"}\n")
	writeFile(t, provisionReceiptPath, "{\"kind\":\"receipt\"}\n")
	for _, root := range []string{freshRoot, attachedRoot} {
		if err := os.MkdirAll(root, 0o755); err != nil {
			t.Fatal(err)
		}
	}
	actions := []map[string]any{{
		"candidatePath":       filepath.Join(candidateRoot, "memory.candidate.md"),
		"kind":                "managed-doc",
		"decision":            "accept",
		"packTarget":          filepath.Join(repo, "packs", "fixture", "memory.md"),
		"action":              "replace pack target with reviewed candidate",
		"candidateBackupPath": filepath.Join(backupRoot, "actions", "000", "candidate"),
		"evidenceRefs":        []string{},
	}}
	receipt := map[string]any{
		"schemaVersion": 1, "kind": "pack-memory-candidate-decision-receipt", "pack": "fixture", "repoRoot": repo, "caseRoot": caseRoot,
		"packetPath": packetPath, "decisionPath": decisionPath, "packetHash": packetHash, "decisionHash": decisionHash, "backupRoot": backupRoot,
		"indexPath": filepath.Join(candidateRoot, "index.json"), "accepted": 1, "rejected": 0, "superseded": 0, "actions": actions,
		"decisionEvidence": []string{}, "receiptPath": receiptPath, "verificationPending": true, "verificationWorkspaceRoot": workspace,
		"verificationProvisionCommand": "/rekit promote -PacketPath " + packetPath + " -CandidateDecisionPath " + decisionPath + " -ProvisionCandidateVerificationCases -WhatIf",
		"verificationCommand":          "/rekit promote -VerifyCandidateDecision -WhatIf", "verificationProofPath": proofPath, "boundary": []string{"fixture boundary"},
	}
	receiptData, err := json.MarshalIndent(receipt, "", "  ")
	if err != nil {
		t.Fatal(err)
	}
	receiptData = append(receiptData, '\n')
	writeFile(t, receiptPath, string(receiptData))
	previewCommand := "/rekit promote -PacketPath " + packetPath + " -CandidateDecisionPath " + decisionPath + " -RetireCandidateVerificationWorkspace -WhatIf -Format json"
	proof := map[string]any{
		"schemaVersion": 1, "kind": "pack-memory-candidate-decision-verification", "pack": "fixture", "caseRoot": caseRoot,
		"freshCaseRoot": freshRoot, "attachedCaseRoot": attachedRoot, "packetHash": packetHash, "decisionHash": decisionHash,
		"receiptHash": sha256ReleaseHandoff(receiptData), "receiptPath": receiptPath, "verificationProofPath": proofPath,
		"provisionIntentPath": provisionIntentPath, "provisionIntentSha256": fileSHA256ReleaseHandoff(provisionIntentPath),
		"provisionReceiptPath": provisionReceiptPath, "provisionReceiptSha256": fileSHA256ReleaseHandoff(provisionReceiptPath),
		"retirementPreviewCommand": previewCommand, "isMutation": true, "applied": true, "ready": true,
		"packDoctorRows": 1, "freshDoctorRows": 1, "attachedDoctorRows": 1, "verifiedActions": actions,
		"nextSteps": []string{"preview retirement"}, "boundary": []string{"fixture boundary"},
	}
	proofData, err := json.MarshalIndent(proof, "", "  ")
	if err != nil {
		t.Fatal(err)
	}
	proofData = append(proofData, '\n')
	writeFile(t, proofPath, string(proofData))
	acceptedCandidateBackupPath := actions[0]["candidateBackupPath"].(string)
	acceptedPackTarget := actions[0]["packTarget"].(string)
	writeFile(t, acceptedCandidateBackupPath, "accepted memory content\n")
	writeFile(t, acceptedPackTarget, "accepted memory content\n")
	writeFile(t, filepath.Join(backupRoot, "transaction.json"), "{\"kind\":\"transaction\"}\n")
	acceptedCleanupEvidence := filepath.Join(caseRoot, "cleanup-evidence.md")
	writeFile(t, acceptedCleanupEvidence, "cleanup proof evidence\n")
	writeCandidateCleanupProof(t, repo, caseRoot, filepath.Join(proofRoot, "accepted-noncanonical.candidate-cleanup-proof.json"), "fixture", packetHash, decisionHash, receiptPath, filepath.Join(backupRoot, "transaction.json"), filepath.Join(backupRoot, "committed.json"), actions[0]["candidatePath"].(string), acceptedCandidateBackupPath, acceptedPackTarget, "", filepath.Join(candidateRoot, "index.json"), "accept", "managed-doc", acceptedCleanupEvidence)
	ownedContent := "owned verification artifact\n"
	freshOwned := filepath.Join(freshRoot, "owned.txt")
	attachedOwned := filepath.Join(attachedRoot, "owned.txt")
	writeFile(t, freshOwned, ownedContent)
	writeFile(t, attachedOwned, ownedContent)
	ownedHash := sha256ReleaseHandoff([]byte(ownedContent))
	roots := []candidateVerificationRetirementRootInventory{
		{Role: "fresh", CaseRoot: freshRoot, Deletes: []string{freshOwned}},
		{Role: "attached", CaseRoot: attachedRoot, Deletes: []string{attachedOwned}},
	}
	plans := []syncpkg.ExclusiveInitRetirementPlan{
		{SchemaVersion: 1, Command: "exclusive-init-retirement", CaseRoot: freshRoot, ProvisionID: retirementID, Role: "fresh", Leaves: []syncpkg.ExclusiveInitRetirementLeaf{{Path: "owned.txt", SHA256: ownedHash, Size: int64(len(ownedContent))}}},
		{SchemaVersion: 1, Command: "exclusive-init-retirement", CaseRoot: attachedRoot, ProvisionID: retirementID, Role: "attached", Leaves: []syncpkg.ExclusiveInitRetirementLeaf{{Path: "owned.txt", SHA256: ownedHash, Size: int64(len(ownedContent))}}},
	}
	artifact := candidateVerificationRetirementArtifactInventory{
		SchemaVersion: 1, Kind: "pack-memory-candidate-verification-retirement-intent", RepoRoot: repo, SourceCaseRoot: caseRoot, Pack: "fixture",
		PacketPath: packetPath, PacketSHA256: packetHash, DecisionPath: decisionPath, DecisionSHA256: decisionHash,
		DecisionReceiptPath: receiptPath, DecisionReceiptSHA256: sha256ReleaseHandoff(receiptData), VerificationProofPath: proofPath,
		VerificationProofSHA256: sha256ReleaseHandoff(proofData), ProvisionIntentPath: provisionIntentPath,
		ProvisionIntentSHA256: fileSHA256ReleaseHandoff(provisionIntentPath), ProvisionReceiptPath: provisionReceiptPath,
		ProvisionReceiptSHA256: fileSHA256ReleaseHandoff(provisionReceiptPath), WorkspaceRoot: workspace,
		RetirementIntentPath: retirementIntentPath, RetirementReceiptPath: retirementReceiptPath, Roots: roots,
		ProvisionArtifactsToDelete: []string{provisionReceiptPath, provisionIntentPath},
		EmptyAncestorsToRemove:     []string{filepath.Dir(workspace), filepath.Dir(filepath.Dir(workspace))},
		Boundary:                   []string{"fixture boundary"}, RetirementPlans: plans,
	}
	artifact.RetirementSHA256 = candidateVerificationRetirementHash(candidateVerificationRetirementResultFromInventory(artifact, "-intent"))
	writeRetirementArtifact := func(path, kind string) {
		t.Helper()
		artifact.Kind = kind
		data, marshalErr := json.MarshalIndent(artifact, "", "  ")
		if marshalErr != nil {
			t.Fatal(marshalErr)
		}
		writeFile(t, path, string(data)+"\n")
	}
	writeRetirementArtifact(retirementIntentPath, "pack-memory-candidate-verification-retirement-intent")
	inProgress := releaseHandoffPackMemoryCandidates(repo, []manifest.PackSummary{{ID: "fixture", Maturity: "skeleton"}})
	if inProgress.Ready || len(inProgress.Packs) != 1 || !inProgress.Packs[0].DecisionReceipts[0].RetirementInProgress || inProgress.Packs[0].DecisionReceipts[0].RetirementStatus != "in-progress" || !strings.Contains(inProgress.Packs[0].Action, "resume") {
		t.Fatalf("retirement intent was not projected as in-progress: %+v", inProgress)
	}
	assertInvalidResume := func(name, want string, mutate func() func()) {
		t.Helper()
		restore := mutate()
		invalid := releaseHandoffPackMemoryCandidates(repo, []manifest.PackSummary{{ID: "fixture", Maturity: "skeleton"}})
		if invalid.Ready || invalid.Summary != "pack-memory candidate inventory has warnings" || len(invalid.Warnings) == 0 || !strings.Contains(fmt.Sprint(invalid.Warnings), want) || strings.Contains(fmt.Sprint(invalid.Packs), "retirementStatus:in-progress") {
			t.Fatalf("%s retirement drift was not rejected: %+v", name, invalid)
		}
		restore()
	}
	assertInvalidResume("extra object", "extra object", func() func() {
		extra := filepath.Join(freshRoot, "extra.txt")
		writeFile(t, extra, "extra\n")
		return func() { _ = os.Remove(extra) }
	})
	assertInvalidResume("different bytes", "different bytes", func() func() {
		writeFile(t, freshOwned, "different\n")
		return func() { writeFile(t, freshOwned, ownedContent) }
	})
	assertInvalidResume("symlink", "symlink", func() func() {
		if err := os.Remove(freshOwned); err != nil {
			t.Fatal(err)
		}
		if err := os.Symlink(attachedOwned, freshOwned); err != nil {
			writeFile(t, freshOwned, ownedContent)
			t.Skipf("symlink unavailable: %v", err)
		}
		return func() {
			_ = os.Remove(freshOwned)
			writeFile(t, freshOwned, ownedContent)
		}
	})
	assertInvalidResume("tree quarantine drift", "extra object", func() func() {
		quarantine := filepath.Join(freshRoot, ".owned.txt.rekit-retire-deadbeefdeadbeef.tmp")
		writeFile(t, quarantine, ownedContent)
		return func() { _ = os.Remove(quarantine) }
	})
	provisionQuarantine := func(path, hash string) string {
		return filepath.Join(filepath.Dir(path), "."+filepath.Base(path)+".retiring-"+hash[:16])
	}
	intentQuarantine := provisionQuarantine(provisionIntentPath, artifact.ProvisionIntentSHA256)
	if err := os.Rename(provisionIntentPath, intentQuarantine); err != nil {
		t.Fatal(err)
	}
	quarantineOnly := releaseHandoffPackMemoryCandidates(repo, []manifest.PackSummary{{ID: "fixture", Maturity: "skeleton"}})
	if quarantineOnly.Ready || len(quarantineOnly.Packs) != 1 || !quarantineOnly.Packs[0].DecisionReceipts[0].RetirementInProgress {
		t.Fatalf("exact quarantine-only provision artifact was not resumable: %+v", quarantineOnly)
	}
	writeFile(t, provisionIntentPath, "{\"kind\":\"intent\"}\n")
	dual := releaseHandoffPackMemoryCandidates(repo, []manifest.PackSummary{{ID: "fixture", Maturity: "skeleton"}})
	if dual.Ready || len(dual.Warnings) == 0 || !strings.Contains(fmt.Sprint(dual.Warnings), "both exist") {
		t.Fatalf("dual canonical/quarantine provision artifact was not rejected: %+v", dual)
	}
	if err := os.Remove(provisionIntentPath); err != nil {
		t.Fatal(err)
	}
	writeFile(t, intentQuarantine, "different\n")
	wrongQuarantine := releaseHandoffPackMemoryCandidates(repo, []manifest.PackSummary{{ID: "fixture", Maturity: "skeleton"}})
	if wrongQuarantine.Ready || len(wrongQuarantine.Warnings) == 0 || !strings.Contains(fmt.Sprint(wrongQuarantine.Warnings), "changed") {
		t.Fatalf("different quarantine bytes were not rejected: %+v", wrongQuarantine)
	}
	if err := os.Remove(intentQuarantine); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink(provisionReceiptPath, intentQuarantine); err == nil {
		symlinkQuarantine := releaseHandoffPackMemoryCandidates(repo, []manifest.PackSummary{{ID: "fixture", Maturity: "skeleton"}})
		if symlinkQuarantine.Ready || len(symlinkQuarantine.Warnings) == 0 || !strings.Contains(fmt.Sprint(symlinkQuarantine.Warnings), "not regular") {
			t.Fatalf("symlink quarantine was not rejected: %+v", symlinkQuarantine)
		}
		if err := os.Remove(intentQuarantine); err != nil {
			t.Fatal(err)
		}
	}
	if err := os.Mkdir(intentQuarantine, 0o755); err != nil {
		t.Fatal(err)
	}
	nonregularQuarantine := releaseHandoffPackMemoryCandidates(repo, []manifest.PackSummary{{ID: "fixture", Maturity: "skeleton"}})
	if nonregularQuarantine.Ready || len(nonregularQuarantine.Warnings) == 0 || !strings.Contains(fmt.Sprint(nonregularQuarantine.Warnings), "not regular") {
		t.Fatalf("non-regular quarantine was not rejected: %+v", nonregularQuarantine)
	}
	if err := os.Remove(intentQuarantine); err != nil {
		t.Fatal(err)
	}
	bothAbsent := releaseHandoffPackMemoryCandidates(repo, []manifest.PackSummary{{ID: "fixture", Maturity: "skeleton"}})
	if bothAbsent.Ready || len(bothAbsent.Packs) != 1 || !bothAbsent.Packs[0].DecisionReceipts[0].RetirementInProgress {
		t.Fatalf("absent canonical/quarantine provision artifact was not resumable: %+v", bothAbsent)
	}
	writeRetirementArtifact(retirementReceiptPath, "pack-memory-candidate-verification-retirement-receipt")
	for _, path := range []string{freshOwned, attachedOwned} {
		if err := os.Remove(path); err != nil {
			t.Fatal(err)
		}
	}
	for _, path := range []string{freshRoot, attachedRoot} {
		if err := os.Remove(path); err != nil {
			t.Fatal(err)
		}
	}
	for _, path := range []string{provisionReceiptPath, provisionIntentPath, workspace} {
		if err := os.Remove(path); err != nil && !os.IsNotExist(err) {
			t.Fatal(err)
		}
	}
	retired := releaseHandoffPackMemoryCandidates(repo, []manifest.PackSummary{{ID: "fixture", Maturity: "skeleton"}})
	if !retired.Ready || retired.Total != 0 || len(retired.Packs) != 0 || len(retired.Warnings) != 0 {
		t.Fatalf("exact retirement receipt did not close handoff: %+v", retired)
	}
	if err := os.MkdirAll(workspace, 0o755); err != nil {
		t.Fatal(err)
	}
	reappeared := releaseHandoffPackMemoryCandidates(repo, []manifest.PackSummary{{ID: "fixture", Maturity: "skeleton"}})
	if reappeared.Ready || reappeared.Summary != "pack-memory candidate inventory has warnings" || len(reappeared.Warnings) == 0 || !strings.Contains(fmt.Sprint(reappeared.Warnings), "reappeared") {
		t.Fatalf("reappeared retired workspace was not rejected: %+v", reappeared)
	}
	if _, err := os.Stat(workspace); err != nil {
		t.Fatalf("release handoff mutated reappeared workspace: %v", err)
	}
}

func TestReleaseHandoffPackMemoryToolingRejectReceiptDoesNotBlock(t *testing.T) {
	repo := t.TempDir()
	candidateRoot := filepath.Join(repo, "packs", "fixture", "promote-candidates")
	proofRoot := filepath.Join(candidateRoot, "review-artifacts")
	toolingRoot := filepath.Join(repo, "packs", "fixture", "tooling", "candidates")
	backupRoot := filepath.Join(candidateRoot, ".decision-backup", "tooling-reject")
	caseRoot := filepath.Join(t.TempDir(), "case")
	packetPath := filepath.Join(caseRoot, "packet.json")
	decisionPath := filepath.Join(caseRoot, "decisions.json")
	candidatePath := filepath.Join(toolingRoot, "tool.candidate.md")
	candidateBackupPath := filepath.Join(backupRoot, "candidate-backup.md")
	evidencePath := filepath.Join(caseRoot, "tooling-review.md")
	packet := promote.CandidateReviewPacket{
		SchemaVersion: 1,
		Kind:          "pack-memory-candidate-review",
		Command:       "promote",
		CandidateResult: promote.CandidateResult{
			SchemaVersion: 1,
			Command:       "promote",
			CaseRoot:      caseRoot,
			RepoRoot:      repo,
			Pack:          "fixture",
			IsMutation:    true,
			Applied:       true,
			CandidateRoot: candidateRoot,
			ToolingRoot:   toolingRoot,
			Created:       1,
			Writes: []promote.CandidateWrite{{
				Path:       "tooling/tool.md",
				Kind:       "tooling-candidate-source",
				Action:     "create-candidate",
				SourcePath: filepath.Join(caseRoot, "tooling", "tool.md"),
				TargetPath: candidatePath,
			}},
			ReviewPlan: promote.CandidateReviewPlan{
				Mode:          "candidate-review",
				Scope:         "pack-memory",
				CandidateRoot: candidateRoot,
				ToolingRoot:   toolingRoot,
				ItemCount:     1,
				ReviewItems: []promote.CandidateReviewItem{{
					Path:           "tooling/tool.md",
					Kind:           "tooling-candidate-source",
					Action:         "review-sanitized-tooling-candidate",
					ReviewDecision: "pending-review",
					CandidatePath:  candidatePath,
					CleanupPath:    candidatePath,
				}},
			},
			RequiresReview:  true,
			RequiresCleanup: true,
		},
		Boundary: []string{"fixture review boundary"},
	}
	packetJSON, err := json.MarshalIndent(packet, "", "  ")
	if err != nil {
		t.Fatal(err)
	}
	writeFile(t, packetPath, string(packetJSON)+"\n")
	writeFile(t, candidateBackupPath, "reviewed tooling candidate\n")
	writeFile(t, evidencePath, "reviewed reject evidence\n")
	packetData, err := os.ReadFile(packetPath)
	if err != nil {
		t.Fatal(err)
	}
	packetHash := sha256ReleaseHandoff(packetData)
	decision := map[string]any{
		"schemaVersion": 1,
		"kind":          "pack-memory-candidate-decisions",
		"packetHash":    packetHash,
		"decisions": []map[string]any{{
			"candidatePath": candidatePath,
			"decision":      "reject",
			"candidateHash": fileSHA256ReleaseHandoff(candidateBackupPath),
			"reason":        "reviewed tooling observation is not reusable",
			"actor":         "mission-commander",
			"evidenceRefs": []map[string]any{{
				"path":   evidencePath,
				"sha256": fileSHA256ReleaseHandoff(evidencePath),
			}},
		}},
	}
	decisionData, err := json.MarshalIndent(decision, "", "  ")
	if err != nil {
		t.Fatal(err)
	}
	writeFile(t, decisionPath, string(decisionData)+"\n")
	decisionFileData, err := os.ReadFile(decisionPath)
	if err != nil {
		t.Fatal(err)
	}
	decisionHash := sha256ReleaseHandoff(decisionFileData)
	receiptID := sha256ReleaseHandoff([]byte(packetHash + decisionHash))[:16]
	receiptPath := filepath.Join(proofRoot, receiptID+".candidate-decision-receipt.json")
	action := map[string]any{
		"candidatePath":       candidatePath,
		"kind":                "tooling-candidate-source",
		"decision":            "reject",
		"action":              "cleanup-rejected-candidate",
		"candidateBackupPath": candidateBackupPath,
		"evidenceRefs":        []string{evidencePath},
	}
	indexPath := filepath.Join(candidateRoot, "index.json")
	result := map[string]any{
		"schemaVersion": 1,
		"command":       "promote",
		"mode":          "candidate-decision",
		"caseRoot":      caseRoot,
		"repoRoot":      repo,
		"pack":          "fixture",
		"packetPath":    packetPath,
		"decisionPath":  decisionPath,
		"packetHash":    packetHash,
		"isMutation":    true,
		"applied":       true,
		"accepted":      0,
		"rejected":      1,
		"superseded":    0,
		"backupRoot":    backupRoot,
		"indexPath":     indexPath,
		"actions":       []map[string]any{action},
		"nextSteps":     []string{"rerun release-check"},
		"boundary":      []string{"fixture boundary"},
	}
	transaction := map[string]any{
		"schemaVersion": 1,
		"kind":          "pack-memory-candidate-decision-transaction",
		"packetHash":    packetHash,
		"decisionHash":  decisionHash,
		"indexExisted":  false,
		"result":        result,
		"actions":       []map[string]any{action},
	}
	transactionData, err := json.MarshalIndent(transaction, "", "  ")
	if err != nil {
		t.Fatal(err)
	}
	writeFile(t, filepath.Join(backupRoot, "transaction.json"), string(transactionData)+"\n")
	receipt := map[string]any{
		"schemaVersion":       1,
		"kind":                "pack-memory-candidate-decision-receipt",
		"pack":                "fixture",
		"repoRoot":            repo,
		"caseRoot":            caseRoot,
		"packetPath":          packetPath,
		"decisionPath":        decisionPath,
		"packetHash":          packetHash,
		"decisionHash":        decisionHash,
		"backupRoot":          backupRoot,
		"indexPath":           indexPath,
		"accepted":            0,
		"rejected":            1,
		"superseded":          0,
		"actions":             []map[string]any{action},
		"decisionEvidence":    []string{evidencePath},
		"receiptPath":         receiptPath,
		"verificationPending": false,
		"boundary":            []string{"fixture boundary"},
	}
	data, err := json.MarshalIndent(receipt, "", "  ")
	if err != nil {
		t.Fatal(err)
	}
	result["receiptPath"] = receiptPath
	result["receipt"] = receipt
	committedData, err := json.MarshalIndent(result, "", "  ")
	if err != nil {
		t.Fatal(err)
	}
	writeFile(t, filepath.Join(backupRoot, "committed.json"), string(committedData)+"\n")
	writeFile(t, receiptPath, string(data)+"\n")
	cleanupProofPath := filepath.Join(proofRoot, "tool.candidate-cleanup-proof.md")
	writeFile(t, cleanupProofPath, "# cleanup proof\n")
	looseProof := releaseHandoffPackMemoryCandidates(repo, []manifest.PackSummary{{ID: "fixture", Maturity: "skeleton"}})
	if looseProof.Ready || looseProof.Summary != "pack-memory candidate inventory has warnings" || len(looseProof.Warnings) == 0 || !strings.Contains(fmt.Sprint(looseProof.Warnings), "decode candidate cleanup proof") {
		t.Fatalf("loose cleanup proof was not rejected: %+v", looseProof)
	}
	writeCandidateCleanupProof(t, repo, caseRoot, cleanupProofPath, "fixture", packetHash, decisionHash, receiptPath, filepath.Join(backupRoot, "transaction.json"), filepath.Join(backupRoot, "committed.json"), candidatePath, candidateBackupPath, "", "", indexPath, "reject", "tooling-candidate-source", evidencePath)
	inventory := releaseHandoffPackMemoryCandidates(repo, []manifest.PackSummary{{ID: "fixture", Maturity: "skeleton"}})
	if !inventory.Ready || inventory.Total != 0 || len(inventory.Packs) != 0 || len(inventory.Warnings) != 0 {
		t.Fatalf("completed tooling reject receipt blocked release handoff: %+v", inventory)
	}
	renamedReceiptPath := filepath.Join(proofRoot, "renamed.candidate-decision-receipt.json")
	receipt["receiptPath"] = renamedReceiptPath
	renamedData, err := json.MarshalIndent(receipt, "", "  ")
	if err != nil {
		t.Fatal(err)
	}
	writeFile(t, renamedReceiptPath, string(renamedData)+"\n")
	if err := os.Remove(receiptPath); err != nil {
		t.Fatal(err)
	}
	renamed := releaseHandoffPackMemoryCandidates(repo, []manifest.PackSummary{{ID: "fixture", Maturity: "skeleton"}})
	if renamed.Ready || renamed.Summary != "pack-memory candidate inventory has warnings" || len(renamed.Warnings) == 0 {
		t.Fatalf("renamed terminal receipt was not rejected: %+v", renamed)
	}
	if err := os.Remove(renamedReceiptPath); err != nil {
		t.Fatal(err)
	}
	receipt["receiptPath"] = receiptPath
	writeFile(t, receiptPath, string(data)+"\n")
	writeFile(t, decisionPath, "{\"schemaVersion\":1}\n")
	drifted := releaseHandoffPackMemoryCandidates(repo, []manifest.PackSummary{{ID: "fixture", Maturity: "skeleton"}})
	if drifted.Ready || drifted.Summary != "pack-memory candidate inventory has warnings" || len(drifted.Warnings) == 0 {
		t.Fatalf("terminal receipt accepted decision hash drift: %+v", drifted)
	}
	writeFile(t, decisionPath, string(decisionData)+"\n")
	writeFile(t, candidateBackupPath, "tampered candidate backup\n")
	tamperedBackup := releaseHandoffPackMemoryCandidates(repo, []manifest.PackSummary{{ID: "fixture", Maturity: "skeleton"}})
	if tamperedBackup.Ready || tamperedBackup.Summary != "pack-memory candidate inventory has warnings" || len(tamperedBackup.Warnings) == 0 {
		t.Fatalf("terminal receipt accepted candidate backup drift: %+v", tamperedBackup)
	}
	writeFile(t, candidateBackupPath, "reviewed tooling candidate\n")
	action["decision"] = "accept"
	action["action"] = "merge-accepted-candidate-and-cleanup"
	action["packTarget"] = filepath.Join(repo, "packs", "fixture", "tooling", "recipes", "tool.md")
	receipt["accepted"] = 1
	receipt["rejected"] = 0
	receipt["verificationPending"] = true
	receipt["verificationCommand"] = "/rekit promote -VerifyCandidateDecision -WhatIf"
	receipt["verificationProofPath"] = filepath.Join(proofRoot, "tooling-accept.candidate-verification-proof.json")
	forged, err := json.MarshalIndent(receipt, "", "  ")
	if err != nil {
		t.Fatal(err)
	}
	writeFile(t, receiptPath, string(forged)+"\n")
	invalid := releaseHandoffPackMemoryCandidates(repo, []manifest.PackSummary{{ID: "fixture", Maturity: "skeleton"}})
	if invalid.Ready || invalid.Summary != "pack-memory candidate inventory has warnings" || len(invalid.Warnings) == 0 {
		t.Fatalf("forged tooling accept receipt was not rejected: %+v", invalid)
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

func writeCandidateLifecycleProof(t *testing.T, repo, proofPath, pack, proofType, candidatePath, packTarget string, checks []map[string]any, evidencePath string) {
	t.Helper()
	evidenceRel, err := filepath.Rel(repo, evidencePath)
	if err != nil || strings.HasPrefix(evidenceRel, ".."+string(filepath.Separator)) || filepath.IsAbs(evidenceRel) {
		t.Fatalf("candidate lifecycle proof evidence leaves repo: %s", evidencePath)
	}
	proof := map[string]any{
		"schemaVersion": 1,
		"kind":          "pack-memory-candidate-lifecycle-proof",
		"pack":          pack,
		"proofType":     proofType,
		"candidatePath": releaseHandoffRepoRelative(repo, candidatePath),
		"packTarget":    releaseHandoffRepoRelative(repo, packTarget),
		"reason":        "reviewed candidate lifecycle proof fixture",
		"actor":         "mission-commander",
		"evidenceRefs": []map[string]any{{
			"path":   filepath.ToSlash(evidenceRel),
			"sha256": fileSHA256ReleaseHandoff(evidencePath),
		}},
		"reviewItem": map[string]any{
			"candidatePath": releaseHandoffRepoRelative(repo, candidatePath),
			"packTarget":    releaseHandoffRepoRelative(repo, packTarget),
			"proofType":     proofType,
			"stage":         packMemoryCandidateProofArtifactStage(proofType),
		},
		"checks":   checks,
		"boundary": []string{"candidate lifecycle proof fixture is read-only release handoff evidence"},
	}
	data, err := json.MarshalIndent(proof, "", "  ")
	if err != nil {
		t.Fatal(err)
	}
	writeFile(t, proofPath, string(data)+"\n")
}

func writeCandidateDecisionProof(t *testing.T, repo, proofPath, pack, packetHash, candidatePath, packTarget, decision, kind, evidencePath string) {
	t.Helper()
	candidateHash := fileSHA256ReleaseHandoff(candidatePath)
	packTargetHash := ""
	if decision == "accept" {
		packTargetHash = fileSHA256ReleaseHandoff(packTarget)
	}
	evidenceRel, err := filepath.Rel(repo, evidencePath)
	if err != nil || strings.HasPrefix(evidenceRel, ".."+string(filepath.Separator)) || filepath.IsAbs(evidenceRel) {
		t.Fatalf("candidate decision proof evidence leaves repo: %s", evidencePath)
	}
	proof := map[string]any{
		"schemaVersion":  1,
		"kind":           "pack-memory-candidate-review-proof",
		"pack":           pack,
		"packetHash":     packetHash,
		"proofType":      "candidate-decision-note",
		"candidatePath":  releaseHandoffRepoRelative(repo, candidatePath),
		"candidateHash":  candidateHash,
		"packTarget":     releaseHandoffRepoRelative(repo, packTarget),
		"packTargetHash": packTargetHash,
		"decision":       decision,
		"reason":         "reviewed candidate decision proof fixture",
		"actor":          "mission-commander",
		"evidenceRefs": []map[string]any{{
			"path":   filepath.ToSlash(evidenceRel),
			"sha256": fileSHA256ReleaseHandoff(evidencePath),
		}},
		"reviewItem": map[string]any{
			"candidatePath": releaseHandoffRepoRelative(repo, candidatePath),
			"candidateHash": candidateHash,
			"packTarget":    releaseHandoffRepoRelative(repo, packTarget),
			"kind":          kind,
		},
		"boundary": []string{"candidate decision proof fixture is read-only release handoff evidence"},
	}
	data, err := json.MarshalIndent(proof, "", "  ")
	if err != nil {
		t.Fatal(err)
	}
	writeFile(t, proofPath, string(data)+"\n")
}

func writeCandidateCleanupProof(t *testing.T, repo, caseRoot, proofPath, pack, packetHash, decisionHash, receiptPath, transactionPath, committedPath, candidatePath, candidateBackupPath, packTarget, targetBackupPath, indexPath, decision, kind, evidencePath string) {
	t.Helper()
	candidateHash := fileSHA256ReleaseHandoff(candidateBackupPath)
	packTargetHash := ""
	if decision == "accept" {
		packTargetHash = fileSHA256ReleaseHandoff(packTarget)
	}
	evidenceRel, err := filepath.Rel(caseRoot, evidencePath)
	if err != nil || strings.HasPrefix(evidenceRel, ".."+string(filepath.Separator)) || filepath.IsAbs(evidenceRel) {
		evidenceRel, err = filepath.Rel(repo, evidencePath)
		if err != nil || strings.HasPrefix(evidenceRel, ".."+string(filepath.Separator)) || filepath.IsAbs(evidenceRel) {
			t.Fatalf("cleanup proof evidence leaves repo/case: %s", evidencePath)
		}
	}
	proof := map[string]any{
		"schemaVersion":  1,
		"kind":           "pack-memory-candidate-review-proof",
		"pack":           pack,
		"packetHash":     packetHash,
		"decisionHash":   decisionHash,
		"proofType":      "candidate-cleanup-proof",
		"candidatePath":  releaseHandoffRepoRelative(repo, candidatePath),
		"candidateHash":  candidateHash,
		"packTarget":     releaseHandoffRepoRelative(repo, packTarget),
		"packTargetHash": packTargetHash,
		"decision":       decision,
		"reason":         "reviewed cleanup receipt and current candidate residue state",
		"actor":          "mission-commander",
		"evidenceRefs": []map[string]any{{
			"path":   filepath.ToSlash(evidenceRel),
			"sha256": fileSHA256ReleaseHandoff(evidencePath),
		}},
		"reviewItem": map[string]any{
			"candidatePath": releaseHandoffRepoRelative(repo, candidatePath),
			"candidateHash": candidateHash,
			"packTarget":    releaseHandoffRepoRelative(repo, packTarget),
			"kind":          kind,
		},
		"cleanup": map[string]any{
			"decisionReceiptPath": releaseHandoffRepoRelative(repo, receiptPath),
			"decisionReceiptHash": fileSHA256ReleaseHandoff(receiptPath),
			"transactionPath":     releaseHandoffRepoRelative(repo, transactionPath),
			"transactionHash":     fileSHA256ReleaseHandoff(transactionPath),
			"committedPath":       releaseHandoffRepoRelative(repo, committedPath),
			"committedHash":       fileSHA256ReleaseHandoff(committedPath),
			"candidateBackupPath": releaseHandoffRepoRelative(repo, candidateBackupPath),
			"candidateBackupHash": candidateHash,
			"targetBackupPath":    releaseHandoffRepoRelative(repo, targetBackupPath),
			"targetBackupHash":    fileSHA256ReleaseHandoff(targetBackupPath),
			"indexPath":           releaseHandoffRepoRelative(repo, indexPath),
			"indexPresent":        refExists(indexPath),
			"indexEntryAbsent":    true,
			"candidateAbsent":     true,
			"packTargetHash":      packTargetHash,
		},
		"boundary": []string{"cleanup proof fixture is read-only release handoff evidence"},
	}
	data, err := json.MarshalIndent(proof, "", "  ")
	if err != nil {
		t.Fatal(err)
	}
	writeFile(t, proofPath, string(data)+"\n")
}

func refExists(path string) bool {
	_, err := os.Lstat(path)
	return err == nil
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

func releaseHandoffReviewArtifactProofContains(items []ReleaseHandoffPackMemoryCandidateReviewArtifact, name, candidatePath, proofPath string, present bool) bool {
	for _, item := range items {
		if item.Name == name && item.CandidatePath == candidatePath && item.ProofPresent == present && slices.Contains(item.ExpectedProofs, proofPath) {
			if present {
				return item.ProofPath == proofPath
			}
			return item.ProofPath == ""
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
