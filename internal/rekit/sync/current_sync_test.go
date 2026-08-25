package sync

import (
	"bytes"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"

	"github.com/shuiyu486/re-context-kits/internal/rekit/casebind"
	"github.com/shuiyu486/re-context-kits/internal/rekit/manifest"
	"github.com/shuiyu486/re-context-kits/internal/rekit/projectstate"
	"github.com/shuiyu486/re-context-kits/internal/rekit/runtimebundle"
)

type currentSyncFixtureState struct {
	repoRoot         string
	caseRoot         string
	pack             string
	sourceExecutable string
	targetExecutable string
}

func TestCurrentSyncPreviewBindsExactExternalMaintenanceInputsWithoutWriting(t *testing.T) {
	fixture := newCurrentSyncFixture(t, "")
	before := snapshotFiles(t, fixture.caseRoot)

	plan, err := CurrentSyncPreview(fixture.repoRoot, fixture.caseRoot, fixture.pack, CurrentSyncOptions{
		Command:          "sync",
		ProjectName:      "refreshed-project",
		SourceExecutable: fixture.sourceExecutable,
	})
	if err != nil {
		t.Fatal(err)
	}
	if plan.SchemaVersion != currentSyncSchemaVersion || plan.Kind != currentSyncPlanKind || plan.Status != "ready-to-refresh" || plan.IsMutation || plan.Applied || plan.Replay || plan.AlreadyCurrent || !plan.RequiresReview || !plan.RequiresConfirmation {
		t.Fatalf("unexpected current sync preview: %+v", plan)
	}
	if plan.ExpectedPlanSHA256 == "" || plan.ExpectedPlanSHA256 != mustCurrentSyncPlanSHA256(t, plan) {
		t.Fatalf("current sync plan SHA-256 is not bound to the complete identity: %+v", plan)
	}
	for flag, want := range map[string]string{
		"-Target":                        fixture.caseRoot,
		"-Pack":                          fixture.pack,
		"-ProjectName":                   "refreshed-project",
		"-SourceRepoRoot":                fixture.repoRoot,
		"-SourceExecutable":              fixture.sourceExecutable,
		"-ExpectedCurrentSyncPlanSha256": plan.ExpectedPlanSHA256,
		"-Format":                        "json",
	} {
		if got, ok := currentSyncTestArgValue(plan.ApplyArgs, flag); !ok || got != want {
			t.Fatalf("current sync ApplyArgs %s = %q, present=%t, want %q; args=%v", flag, got, ok, want, plan.ApplyArgs)
		}
	}
	if !currentSyncTestHasArg(plan.ApplyArgs, "-Apply") {
		t.Fatalf("current sync ApplyArgs omit -Apply: %v", plan.ApplyArgs)
	}
	if currentSyncTestHasArg(plan.ApplyArgs, "-Force") {
		t.Fatalf("current sync ApplyArgs must not include -Force: %v", plan.ApplyArgs)
	}
	if !currentSyncTestContains(plan.ObsoleteControlled, ".steamai/common/policies/obsolete-current-sync-test.md") {
		t.Fatalf("current sync plan omitted obsolete controlled file: %v", plan.ObsoleteControlled)
	}
	stateLeaf := currentSyncTestLeaf(t, plan, ".steamai/state.json")
	var state map[string]json.RawMessage
	if err := json.Unmarshal(stateLeaf.after, &state); err != nil {
		t.Fatal(err)
	}
	var futureState map[string]bool
	if err := json.Unmarshal(state["futureState"], &futureState); err != nil || !futureState["preserve"] {
		t.Fatalf("current sync rebuilt state.json instead of preserving unknown state: %s", stateLeaf.after)
	}
	var managed map[string]syncManagedEntry
	if err := json.Unmarshal(state["managed"], &managed); err != nil {
		t.Fatal(err)
	}
	if managed["references/template/retained-pack-memory.md"].LastAction != "pack-memory-selected-sync" || managed["references/template/README.md"].LastAction != "sync" {
		t.Fatalf("current sync did not preserve non-owned managed state or refresh its owned entry: %+v", managed)
	}
	if _, exists := managed["references/template/obsolete-managed.md"]; exists {
		t.Fatalf("current sync retained obsolete sync-owned managed state: %+v", managed)
	}
	if got := strings.TrimSpace(string(state["templateRoot"])); got != `"."` {
		t.Fatalf("current sync state templateRoot = %s, want project-local root", got)
	}
	assertSnapshotEqual(t, before, snapshotFiles(t, fixture.caseRoot))
	assertNotExists(t, filepath.Join(fixture.caseRoot, projectstate.CurrentDir, filepath.FromSlash(currentSyncNamespaceRel)))
}

func TestCurrentSyncIntentIsDerivedExactlyFromReviewedPlan(t *testing.T) {
	fixture := newCurrentSyncFixture(t, "")
	plan, err := CurrentSyncPreview(fixture.repoRoot, fixture.caseRoot, fixture.pack, CurrentSyncOptions{SourceExecutable: fixture.sourceExecutable})
	if err != nil {
		t.Fatal(err)
	}
	intent, err := buildCurrentSyncIntent(plan)
	if err != nil {
		t.Fatal(err)
	}
	if intent.Plan.prepared != nil || len(intent.Roots) != len(currentSyncControlledRoots) || len(intent.Leaves) == 0 {
		t.Fatalf("current sync intent omitted exact plan-derived publications: %+v", intent)
	}
	if intent.TransactionPath != currentSyncTransactionPath(plan.ExpectedPlanSHA256) {
		t.Fatalf("current sync intent transaction path = %s", intent.TransactionPath)
	}
	preservedRoots := 0
	for index, root := range intent.Roots {
		if root.Name != currentSyncControlledRoots[index] || root.StagePath != intent.TransactionPath+"/stage/controlled/"+root.Name || root.PreviousPath != intent.TransactionPath+"/previous/"+root.Name {
			t.Fatalf("current sync intent root %d is not canonical: %+v", index, root)
		}
		if !root.Mutate {
			preservedRoots++
		}
	}
	activateLast := 0
	preserved := 0
	for index, leaf := range intent.Leaves {
		if leaf.StagePath != fmt.Sprintf("%s/stage/leaves/%06d.bin", intent.TransactionPath, index) || leaf.PreviousPath != fmt.Sprintf("%s/previous/leaves/%06d.bin", intent.TransactionPath, index) {
			t.Fatalf("current sync intent leaf %d has non-canonical transaction paths: %+v", index, leaf)
		}
		if !leaf.Mutate {
			preserved++
		}
		if leaf.ActivateLast {
			activateLast++
			if leaf.Path != projectstate.CurrentDir+"/instance.yml" {
				t.Fatalf("current sync activation leaf = %s", leaf.Path)
			}
		}
	}
	if activateLast != 1 {
		t.Fatalf("current sync activation-last leaves = %d, want 1", activateLast)
	}
	if preserved == 0 {
		t.Fatal("current sync intent did not retain any exact no-mutation leaves")
	}
	if preservedRoots == 0 {
		t.Fatal("current sync intent did not retain any exact no-mutation controlled roots")
	}
	operations := currentSyncExpectedProgressOperations(intent)
	for _, root := range intent.Roots {
		if root.Mutate {
			continue
		}
		for _, operation := range operations {
			if operation.Target == root.Name && strings.HasPrefix(operation.Kind, "root-") {
				t.Fatalf("preserved current sync root entered mutation journal: root=%+v operation=%+v", root, operation)
			}
		}
	}
	for _, leaf := range intent.Leaves {
		if leaf.Mutate {
			continue
		}
		for _, operation := range operations {
			if operation.Target == leaf.Path && strings.Contains(operation.Kind, "-to-") {
				t.Fatalf("preserved current sync leaf entered mutation journal: leaf=%+v operation=%+v", leaf, operation)
			}
		}
	}
	transactionSHA, err := currentSyncCanonicalSHA(currentSyncTransactionIdentityFor(intent))
	if err != nil || transactionSHA != intent.TransactionSHA256 {
		t.Fatalf("current sync transaction SHA-256 is not canonical: got=%s want=%s err=%v", intent.TransactionSHA256, transactionSHA, err)
	}

	intentBytes, err := currentSyncCanonicalData(intent)
	if err != nil {
		t.Fatal(err)
	}
	intentPath := filepath.Join(fixture.caseRoot, projectstate.CurrentDir, filepath.FromSlash(currentSyncIntentRel))
	writeText(t, intentPath, string(intentBytes))
	readBack, exists, err := readCurrentSyncIntent(fixture.caseRoot, filepath.Join(fixture.caseRoot, projectstate.CurrentDir))
	if err != nil || !exists || !currentSyncCanonicalEqual(readBack, intent) {
		t.Fatalf("current sync canonical intent did not round-trip: exists=%t err=%v read=%+v", exists, err, readBack)
	}
}

func TestCurrentSyncDurableIntentRecoveryDoesNotRequireSourceRoot(t *testing.T) {
	fixture := newCurrentSyncFixture(t, "")
	plan, err := CurrentSyncPreview(fixture.repoRoot, fixture.caseRoot, fixture.pack, CurrentSyncOptions{SourceExecutable: fixture.sourceExecutable})
	if err != nil {
		t.Fatal(err)
	}
	intent, err := buildCurrentSyncIntent(plan)
	if err != nil {
		t.Fatal(err)
	}
	intentBytes, err := currentSyncCanonicalData(intent)
	if err != nil {
		t.Fatal(err)
	}
	intentPath := filepath.Join(fixture.caseRoot, projectstate.CurrentDir, filepath.FromSlash(currentSyncIntentRel))
	writeText(t, intentPath, string(intentBytes))
	if err := os.RemoveAll(fixture.repoRoot); err != nil {
		t.Fatal(err)
	}
	stateRoot := filepath.Join(fixture.caseRoot, projectstate.CurrentDir)
	readBack, exists, err := readCurrentSyncIntent(fixture.caseRoot, stateRoot)
	if err != nil || !exists || !currentSyncCanonicalEqual(readBack, intent) {
		t.Fatalf("current sync durable intent depended on missing source root: exists=%t err=%v read=%+v", exists, err, readBack)
	}
	if err := validateCurrentSyncIntentSource(readBack); err == nil || !strings.Contains(err.Error(), "source root physical identity is unavailable") {
		t.Fatalf("current sync pre-stage source validation error = %v", err)
	}
	preview, err := CurrentSyncPreview(fixture.repoRoot, fixture.caseRoot, fixture.pack, CurrentSyncOptions{SourceExecutable: fixture.sourceExecutable})
	if err != nil {
		t.Fatal(err)
	}
	if !preview.RecoveryPending || preview.Status != "recovery-required" || preview.ExpectedPlanSHA256 != plan.ExpectedPlanSHA256 {
		t.Fatalf("current sync source-independent recovery preview = %+v", preview)
	}
}

func TestCurrentSyncIntentRejectsResignedPlanDerivedDrift(t *testing.T) {
	fixture := newCurrentSyncFixture(t, "")
	plan, err := CurrentSyncPreview(fixture.repoRoot, fixture.caseRoot, fixture.pack, CurrentSyncOptions{SourceExecutable: fixture.sourceExecutable})
	if err != nil {
		t.Fatal(err)
	}
	base, err := buildCurrentSyncIntent(plan)
	if err != nil {
		t.Fatal(err)
	}
	stateRoot := filepath.Join(fixture.caseRoot, projectstate.CurrentDir)

	tests := []struct {
		name   string
		tamper func(*currentSyncIntent)
		want   string
	}{
		{
			name: "controlled-root",
			tamper: func(intent *currentSyncIntent) {
				intent.Roots[0].After = intent.Roots[1].After
			},
			want: "controlled roots do not equal",
		},
		{
			name: "leaf",
			tamper: func(intent *currentSyncIntent) {
				intent.Leaves[0].Kind = "forged-kind"
			},
			want: "leaves do not equal",
		},
		{
			name: "apply-args",
			tamper: func(intent *currentSyncIntent) {
				intent.Plan.ApplyArgs = append(intent.Plan.ApplyArgs, "-Force")
			},
			want: "ApplyArgs are not exact",
		},
		{
			name: "transaction-path",
			tamper: func(intent *currentSyncIntent) {
				intent.TransactionPath = currentSyncTransactionsRel + "/forged"
			},
			want: "transaction path is invalid",
		},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			intent := currentSyncCloneIntentTest(t, base)
			test.tamper(&intent)
			intent.TransactionSHA256, err = currentSyncCanonicalSHA(currentSyncTransactionIdentityFor(intent))
			if err != nil {
				t.Fatal(err)
			}
			if err := validateCurrentSyncIntent(intent, fixture.caseRoot, stateRoot); err == nil || !strings.Contains(err.Error(), test.want) {
				t.Fatalf("current sync resigned %s drift error = %v", test.name, err)
			}
		})
	}
}

func TestCurrentSyncIntentUsesPlanScopedTransactionNamespace(t *testing.T) {
	fixture := newCurrentSyncFixture(t, "")
	plan, err := CurrentSyncPreview(fixture.repoRoot, fixture.caseRoot, fixture.pack, CurrentSyncOptions{SourceExecutable: fixture.sourceExecutable})
	if err != nil {
		t.Fatal(err)
	}
	intent, err := buildCurrentSyncIntent(plan)
	if err != nil {
		t.Fatal(err)
	}
	prefix := currentSyncTransactionPath(plan.ExpectedPlanSHA256) + "/"
	for _, root := range intent.Roots {
		if !strings.HasPrefix(root.StagePath, prefix) || !strings.HasPrefix(root.PreviousPath, prefix) {
			t.Fatalf("current sync root escaped transaction namespace: %+v", root)
		}
	}
	for _, leaf := range intent.Leaves {
		if !strings.HasPrefix(leaf.StagePath, prefix) || !strings.HasPrefix(leaf.PreviousPath, prefix) {
			t.Fatalf("current sync leaf escaped transaction namespace: %+v", leaf)
		}
	}

	other := plan
	other.ExpectedPlanSHA256 = strings.Repeat("f", 64)
	if got := currentSyncTransactionPath(other.ExpectedPlanSHA256); got == intent.TransactionPath {
		t.Fatalf("distinct current sync plans shared transaction namespace: %s", got)
	}
}

func TestCurrentSyncProgressIsSignedAndMonotonic(t *testing.T) {
	fixture := newCurrentSyncFixture(t, "")
	plan, err := CurrentSyncPreview(fixture.repoRoot, fixture.caseRoot, fixture.pack, CurrentSyncOptions{SourceExecutable: fixture.sourceExecutable})
	if err != nil {
		t.Fatal(err)
	}
	intent, err := buildCurrentSyncIntent(plan)
	if err != nil {
		t.Fatal(err)
	}
	progress, err := newCurrentSyncProgress(intent)
	if err != nil {
		t.Fatal(err)
	}
	if progress.Phase != "prepared" || progress.Generation != 1 || progress.Pending != nil || len(progress.Completed) != 0 {
		t.Fatalf("unexpected initial current sync progress: %+v", progress)
	}
	begun, err := currentSyncBeginProgressOperation(progress, intent)
	if err != nil {
		t.Fatal(err)
	}
	if begun.Pending == nil || begun.Pending.Kind != "stage-validated" {
		t.Fatalf("current sync did not bind the first pending operation: %+v", begun)
	}
	if err := validateCurrentSyncProgressTransition(progress, begun, intent); err != nil {
		t.Fatal(err)
	}
	completed, err := currentSyncCompleteProgressOperation(begun, intent)
	if err != nil {
		t.Fatal(err)
	}
	if completed.Phase != "publishing" || len(completed.Completed) != 1 || completed.Pending != nil {
		t.Fatalf("current sync did not complete only the pending operation: %+v", completed)
	}
	if err := validateCurrentSyncProgressTransition(begun, completed, intent); err != nil {
		t.Fatal(err)
	}

	rollback := progress
	rollback.Generation = completed.Generation + 1
	rollback.ProgressSHA256, err = currentSyncCanonicalSHA(currentSyncProgressIdentityFor(rollback))
	if err != nil {
		t.Fatal(err)
	}
	if err := validateCurrentSyncProgressTransition(completed, rollback, intent); err == nil || !strings.Contains(err.Error(), "not monotonic") {
		t.Fatalf("current sync accepted progress rollback: %v", err)
	}
}

func TestCurrentSyncProgressRejectsSkippedAndAmbiguousOperations(t *testing.T) {
	fixture := newCurrentSyncFixture(t, "")
	plan, err := CurrentSyncPreview(fixture.repoRoot, fixture.caseRoot, fixture.pack, CurrentSyncOptions{SourceExecutable: fixture.sourceExecutable})
	if err != nil {
		t.Fatal(err)
	}
	intent, err := buildCurrentSyncIntent(plan)
	if err != nil {
		t.Fatal(err)
	}
	progress, err := newCurrentSyncProgress(intent)
	if err != nil {
		t.Fatal(err)
	}
	expected := currentSyncExpectedProgressOperations(intent)

	skipped := progress
	skipped.Generation++
	skipped.Pending = &expected[1]
	skipped, err = currentSyncSignProgress(skipped)
	if err != nil {
		t.Fatal(err)
	}
	if err := validateCurrentSyncProgress(skipped, intent); err == nil || !strings.Contains(err.Error(), "pending operation is invalid") {
		t.Fatalf("current sync accepted skipped operation: %v", err)
	}

	ambiguous := progress
	ambiguous.Generation++
	ambiguous.Completed = []currentSyncProgressOperation{expected[0]}
	ambiguous.Phase = currentSyncProgressPhaseForCompleted(ambiguous.Completed)
	ambiguous, err = currentSyncSignProgress(ambiguous)
	if err != nil {
		t.Fatal(err)
	}
	if err := validateCurrentSyncProgressTransition(progress, ambiguous, intent); err == nil || !strings.Contains(err.Error(), "publish the next pending") {
		t.Fatalf("current sync accepted completion without pending record: %v", err)
	}

	activationIndex := slices.IndexFunc(expected, func(operation currentSyncProgressOperation) bool {
		return operation.Kind == "activated"
	})
	if activationIndex < 0 {
		t.Fatal("current sync progress omitted activated operation")
	}
	activation := progress
	activation.Completed = append([]currentSyncProgressOperation{}, expected[:activationIndex]...)
	activation.Phase = currentSyncProgressPhaseForCompleted(activation.Completed)
	activation, err = currentSyncSignProgress(activation)
	if err != nil {
		t.Fatal(err)
	}
	if err := validateCurrentSyncProgress(activation, intent); err != nil {
		t.Fatal(err)
	}
	activation, err = currentSyncBeginProgressOperation(activation, intent)
	if err != nil {
		t.Fatal(err)
	}
	if activation.Pending == nil || activation.Pending.Kind != "activated" {
		t.Fatalf("current sync activation was not a distinct pending operation: %+v", activation.Pending)
	}
}

func TestCurrentSyncProgressJournalRejectsMissingGeneration(t *testing.T) {
	fixture := newCurrentSyncFixture(t, "")
	plan, err := CurrentSyncPreview(fixture.repoRoot, fixture.caseRoot, fixture.pack, CurrentSyncOptions{SourceExecutable: fixture.sourceExecutable})
	if err != nil {
		t.Fatal(err)
	}
	intent, err := buildCurrentSyncIntent(plan)
	if err != nil {
		t.Fatal(err)
	}
	first, err := newCurrentSyncProgress(intent)
	if err != nil {
		t.Fatal(err)
	}
	second, err := currentSyncBeginProgressOperation(first, intent)
	if err != nil {
		t.Fatal(err)
	}
	secondData, err := currentSyncCanonicalData(second)
	if err != nil {
		t.Fatal(err)
	}
	secondPath := filepath.Join(fixture.caseRoot, projectstate.CurrentDir, filepath.FromSlash(currentSyncProgressPath(intent, second.Generation)))
	writeText(t, secondPath, string(secondData))
	if _, _, err := readCurrentSyncProgress(filepath.Join(fixture.caseRoot, projectstate.CurrentDir), intent); err == nil || !strings.Contains(err.Error(), "not contiguous") {
		t.Fatalf("current sync accepted missing initial progress generation: %v", err)
	}
}

func TestCurrentSyncProgressCanonicalRoundTrip(t *testing.T) {
	fixture := newCurrentSyncFixture(t, "")
	plan, err := CurrentSyncPreview(fixture.repoRoot, fixture.caseRoot, fixture.pack, CurrentSyncOptions{SourceExecutable: fixture.sourceExecutable})
	if err != nil {
		t.Fatal(err)
	}
	intent, err := buildCurrentSyncIntent(plan)
	if err != nil {
		t.Fatal(err)
	}
	progress, err := newCurrentSyncProgress(intent)
	if err != nil {
		t.Fatal(err)
	}
	data, err := currentSyncCanonicalData(progress)
	if err != nil {
		t.Fatal(err)
	}
	progressPath := filepath.Join(fixture.caseRoot, projectstate.CurrentDir, filepath.FromSlash(currentSyncProgressPath(intent, progress.Generation)))
	writeText(t, progressPath, string(data))
	readBack, exists, err := readCurrentSyncProgress(filepath.Join(fixture.caseRoot, projectstate.CurrentDir), intent)
	if err != nil || !exists || !currentSyncCanonicalEqual(readBack, progress) {
		t.Fatalf("current sync canonical progress did not round-trip: exists=%t err=%v read=%+v", exists, err, readBack)
	}

	writeText(t, progressPath, strings.TrimSpace(string(data)))
	if _, _, err := readCurrentSyncProgress(filepath.Join(fixture.caseRoot, projectstate.CurrentDir), intent); err == nil || !strings.Contains(err.Error(), "not canonical") {
		t.Fatalf("current sync accepted non-canonical progress: %v", err)
	}
}

func TestCurrentSyncReceiptBindsCommittedStateAndReplaysLostResponse(t *testing.T) {
	fixture := newCurrentSyncFixture(t, "")
	transaction := publishCommittedCurrentSyncReplayFixture(t, fixture)
	activeIntentPath := filepath.Join(fixture.caseRoot, projectstate.CurrentDir, filepath.FromSlash(currentSyncIntentRel))
	if _, _, err := currentSyncReplayResult(fixture.caseRoot, transaction.plan.ExpectedPlanSHA256); err == nil || !strings.Contains(err.Error(), "requires exact active intent cleanup") {
		t.Fatalf("current sync replay succeeded before terminal cleanup: %v", err)
	}
	if err := os.Remove(activeIntentPath); err != nil {
		t.Fatal(err)
	}
	if _, _, err := currentSyncReplayResult(
		fixture.caseRoot,
		transaction.plan.ExpectedPlanSHA256,
	); err == nil || !strings.Contains(err.Error(), "transaction owner cleanup") {
		t.Fatalf("current sync replay succeeded before owner cleanup: %v", err)
	}
	if err := os.Remove(filepath.Join(
		fixture.caseRoot,
		projectstate.CurrentDir,
		filepath.FromSlash(currentSyncOwnerRel),
	)); err != nil {
		t.Fatal(err)
	}

	result, exists, err := currentSyncReplayResult(fixture.caseRoot, transaction.plan.ExpectedPlanSHA256)
	if err != nil || !exists {
		t.Fatalf("current sync durable replay failed: exists=%t err=%v", exists, err)
	}
	if !result.Replay || result.Applied || result.IsMutation || !result.AlreadyCurrent || result.Receipt == nil ||
		!currentSyncCanonicalEqual(*result.Receipt, transaction.receipt) || result.PlanSHA256 != transaction.plan.ExpectedPlanSHA256 || result.TransactionSHA256 != transaction.intent.TransactionSHA256 {
		t.Fatalf("unexpected current sync replay result: %+v", result)
	}

	tampered := transaction.receipt
	tampered.Targets = transaction.plan.CurrentTargets
	tampered.ReceiptSHA256, err = currentSyncCanonicalSHA(currentSyncReceiptIdentityFor(tampered))
	if err != nil {
		t.Fatal(err)
	}
	writeCurrentSyncCanonicalTest(t, filepath.Join(fixture.caseRoot, projectstate.CurrentDir, filepath.FromSlash(currentSyncReceiptRel)), tampered)
	if _, _, err := currentSyncReplayResult(fixture.caseRoot, transaction.plan.ExpectedPlanSHA256); err == nil || !strings.Contains(err.Error(), "identity is invalid") {
		t.Fatalf("current sync accepted resigned stale receipt: %v", err)
	}
}

type currentSyncReplayFixture struct {
	plan            CurrentSyncPlan
	intent          currentSyncIntent
	receiptProgress currentSyncProgress
	committed       currentSyncProgress
	receipt         currentSyncReceipt
}

func publishCommittedCurrentSyncReplayFixture(t *testing.T, fixture currentSyncFixtureState) currentSyncReplayFixture {
	t.Helper()
	plan, err := CurrentSyncPreview(fixture.repoRoot, fixture.caseRoot, fixture.pack, CurrentSyncOptions{SourceExecutable: fixture.sourceExecutable})
	if err != nil {
		t.Fatal(err)
	}
	intent, err := buildCurrentSyncIntent(plan)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := publishCurrentSyncIntent(fixture.caseRoot, intent); err != nil {
		t.Fatal(err)
	}
	for _, publication := range plan.prepared.publications {
		target := filepath.Join(fixture.caseRoot, projectstate.CurrentDir, filepath.FromSlash(publication.rel))
		if err := os.MkdirAll(filepath.Dir(target), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(target, publication.data, publication.mode); err != nil {
			t.Fatal(err)
		}
	}
	for _, leaf := range plan.prepared.leaves {
		target := filepath.Join(fixture.caseRoot, filepath.FromSlash(leaf.rel))
		if !leaf.afterExists {
			if err := os.Remove(target); err != nil && !os.IsNotExist(err) {
				t.Fatal(err)
			}
			continue
		}
		if err := os.MkdirAll(filepath.Dir(target), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(target, leaf.after, leaf.mode); err != nil {
			t.Fatal(err)
		}
	}
	for _, rel := range plan.ObsoleteControlled {
		if err := os.Remove(filepath.Join(fixture.caseRoot, filepath.FromSlash(rel))); err != nil && !os.IsNotExist(err) {
			t.Fatal(err)
		}
	}

	progress, err := newCurrentSyncProgress(intent)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := publishCurrentSyncProgress(fixture.caseRoot, intent, progress); err != nil {
		t.Fatal(err)
	}
	for !currentSyncReceiptCommitPending(progress) {
		if progress.Pending == nil {
			progress, err = currentSyncBeginProgressOperation(progress, intent)
		} else {
			progress, err = currentSyncCompleteProgressOperation(progress, intent)
		}
		if err != nil {
			t.Fatal(err)
		}
		if _, err := publishCurrentSyncProgress(fixture.caseRoot, intent, progress); err != nil {
			t.Fatal(err)
		}
	}
	receiptProgress := progress
	receipt, err := buildCurrentSyncReceipt(intent, receiptProgress)
	if err != nil {
		t.Fatal(err)
	}
	if receipt.Controlled.SHA256 != plan.NextControlled.SHA256 || receipt.Targets.SHA256 != plan.NextTargets.SHA256 ||
		receipt.Manifest.SHA256 != plan.NextManifest.SHA256 || receipt.RuntimeExecutable.Kind != "runtime-executable" {
		t.Fatalf("current sync receipt omitted final inventory or runtime identity: %+v", receipt)
	}
	writeCurrentSyncCanonicalTest(t, filepath.Join(fixture.caseRoot, projectstate.CurrentDir, filepath.FromSlash(currentSyncReceiptRel)), receipt)
	for range 3 {
		if progress.Pending == nil {
			progress, err = currentSyncBeginProgressOperation(progress, intent)
		} else {
			progress, err = currentSyncCompleteProgressOperation(progress, intent)
		}
		if err != nil {
			t.Fatal(err)
		}
		if _, err := publishCurrentSyncProgress(fixture.caseRoot, intent, progress); err != nil {
			t.Fatal(err)
		}
	}
	if progress.Phase != "receipt-committed" || progress.Pending != nil {
		t.Fatalf("current sync replay fixture did not reach committed state: %+v", progress)
	}
	return currentSyncReplayFixture{plan: plan, intent: intent, receiptProgress: receiptProgress, committed: progress, receipt: receipt}
}

func writeCurrentSyncCanonicalTest(t *testing.T, path string, value any) {
	t.Helper()
	data, err := currentSyncCanonicalData(value)
	if err != nil {
		t.Fatal(err)
	}
	writeText(t, path, string(data))
}

func TestCurrentSyncReplayRejectsIncompleteJournalAndLiveDrift(t *testing.T) {
	t.Run("missing-terminal-generation", func(t *testing.T) {
		fixture := newCurrentSyncFixture(t, "")
		transaction := publishCommittedCurrentSyncReplayFixture(t, fixture)
		path := filepath.Join(fixture.caseRoot, projectstate.CurrentDir, filepath.FromSlash(currentSyncProgressPath(transaction.intent, transaction.committed.Generation)))
		if err := os.Remove(path); err != nil {
			t.Fatal(err)
		}
		if _, _, err := currentSyncReplayResult(fixture.caseRoot, transaction.plan.ExpectedPlanSHA256); err == nil || !strings.Contains(err.Error(), "incomplete") {
			t.Fatalf("current sync accepted incomplete terminal lineage: %v", err)
		}
	})

	t.Run("extra-terminal-generation", func(t *testing.T) {
		fixture := newCurrentSyncFixture(t, "")
		transaction := publishCommittedCurrentSyncReplayFixture(t, fixture)
		extra := transaction.committed
		extra.Generation++
		extra.ProgressSHA256, _ = currentSyncCanonicalSHA(currentSyncProgressIdentityFor(extra))
		writeCurrentSyncCanonicalTest(t, filepath.Join(fixture.caseRoot, projectstate.CurrentDir, filepath.FromSlash(currentSyncProgressPath(transaction.intent, extra.Generation))), extra)
		if _, _, err := currentSyncReplayResult(fixture.caseRoot, transaction.plan.ExpectedPlanSHA256); err == nil {
			t.Fatal("current sync accepted an extra terminal progress generation")
		}
	})

	t.Run("active-intent-drift", func(t *testing.T) {
		fixture := newCurrentSyncFixture(t, "")
		transaction := publishCommittedCurrentSyncReplayFixture(t, fixture)
		active := currentSyncCloneIntentTest(t, transaction.intent)
		active.NoHeavyTool = false
		writeCurrentSyncCanonicalTest(t, filepath.Join(fixture.caseRoot, projectstate.CurrentDir, filepath.FromSlash(currentSyncIntentRel)), active)
		if _, _, err := currentSyncReplayResult(fixture.caseRoot, transaction.plan.ExpectedPlanSHA256); err == nil {
			t.Fatal("current sync accepted active intent drift")
		}
	})

	t.Run("controlled-extra-file", func(t *testing.T) {
		fixture := newCurrentSyncFixture(t, "")
		transaction := publishCommittedCurrentSyncReplayFixture(t, fixture)
		writeText(t, filepath.Join(fixture.caseRoot, projectstate.CurrentDir, "common", "unplanned.md"), "unplanned\n")
		if _, _, err := currentSyncReplayResult(fixture.caseRoot, transaction.plan.ExpectedPlanSHA256); err == nil || !strings.Contains(err.Error(), "unplanned file") {
			t.Fatalf("current sync accepted extra controlled file: %v", err)
		}
	})

	t.Run("controlled-content-drift", func(t *testing.T) {
		fixture := newCurrentSyncFixture(t, "")
		transaction := publishCommittedCurrentSyncReplayFixture(t, fixture)
		binding := transaction.receipt.Controlled.Entries[0]
		writeText(t, filepath.Join(fixture.caseRoot, filepath.FromSlash(binding.Path)), "tampered\n")
		if _, _, err := currentSyncReplayResult(fixture.caseRoot, transaction.plan.ExpectedPlanSHA256); err == nil {
			t.Fatal("current sync accepted controlled content drift")
		}
	})

	t.Run("active-instance-drift", func(t *testing.T) {
		fixture := newCurrentSyncFixture(t, "")
		transaction := publishCommittedCurrentSyncReplayFixture(t, fixture)
		instancePath := filepath.Join(fixture.caseRoot, projectstate.CurrentDir, "instance.yml")
		writeText(t, instancePath, casebind.STeamAIInstanceText(fixture.caseRoot, fixture.pack, "wrong-project", runtimebundle.ManifestRel, transaction.receipt.Manifest.SHA256))
		if _, _, err := currentSyncReplayResult(fixture.caseRoot, transaction.plan.ExpectedPlanSHA256); err == nil || !strings.Contains(err.Error(), "active instance differs") {
			t.Fatalf("current sync accepted active instance drift: %v", err)
		}
	})
}

func TestCurrentSyncReceiptCommitProgressCompletesAfterReceiptPublication(t *testing.T) {
	fixture := newCurrentSyncFixture(t, "")
	plan, err := CurrentSyncPreview(fixture.repoRoot, fixture.caseRoot, fixture.pack, CurrentSyncOptions{SourceExecutable: fixture.sourceExecutable})
	if err != nil {
		t.Fatal(err)
	}
	intent, err := buildCurrentSyncIntent(plan)
	if err != nil {
		t.Fatal(err)
	}
	progress, err := newCurrentSyncProgress(intent)
	if err != nil {
		t.Fatal(err)
	}
	for !currentSyncReceiptCommitPending(progress) {
		if progress.Pending == nil {
			progress, err = currentSyncBeginProgressOperation(progress, intent)
		} else {
			progress, err = currentSyncCompleteProgressOperation(progress, intent)
		}
		if err != nil {
			t.Fatal(err)
		}
	}
	receipt, err := buildCurrentSyncReceipt(intent, progress)
	if err != nil {
		t.Fatal(err)
	}
	published, err := currentSyncCompleteProgressOperation(progress, intent)
	if err != nil {
		t.Fatal(err)
	}
	committing, err := currentSyncBeginProgressOperation(published, intent)
	if err != nil {
		t.Fatal(err)
	}
	if committing.Pending == nil || committing.Pending.Kind != "receipt-committed" {
		t.Fatalf("current sync receipt commit marker was not pending after publication: %+v", committing)
	}
	committed, err := currentSyncCompleteProgressOperation(committing, intent)
	if err != nil {
		t.Fatal(err)
	}
	if committed.Phase != "receipt-committed" || committed.Pending != nil {
		t.Fatalf("current sync receipt operation was not completed after publication: %+v", committed)
	}
	if err := validateCurrentSyncReceipt(receipt, intent, progress); err != nil {
		t.Fatal(err)
	}
	if err := validateCurrentSyncReceipt(receipt, intent, committed); err == nil || !strings.Contains(err.Error(), "identity is invalid") {
		t.Fatalf("current sync receipt unexpectedly rebound to post-publication progress: %v", err)
	}
}

func TestCurrentSyncReceiptRejectsPreActivationProgress(t *testing.T) {
	fixture := newCurrentSyncFixture(t, "")
	plan, err := CurrentSyncPreview(fixture.repoRoot, fixture.caseRoot, fixture.pack, CurrentSyncOptions{SourceExecutable: fixture.sourceExecutable})
	if err != nil {
		t.Fatal(err)
	}
	intent, err := buildCurrentSyncIntent(plan)
	if err != nil {
		t.Fatal(err)
	}
	progress, err := newCurrentSyncProgress(intent)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := buildCurrentSyncReceipt(intent, progress); err == nil || !strings.Contains(err.Error(), "requires pending receipt commit progress") {
		t.Fatalf("current sync accepted pre-activation receipt: %v", err)
	}
}

func TestCurrentSyncIntentRejectsReplacedFilesystemRoots(t *testing.T) {
	fixture := newCurrentSyncFixture(t, "")
	plan, err := CurrentSyncPreview(fixture.repoRoot, fixture.caseRoot, fixture.pack, CurrentSyncOptions{SourceExecutable: fixture.sourceExecutable})
	if err != nil {
		t.Fatal(err)
	}
	intent, err := buildCurrentSyncIntent(plan)
	if err != nil {
		t.Fatal(err)
	}
	stateRoot := filepath.Join(fixture.caseRoot, projectstate.CurrentDir)
	movedStateRoot := stateRoot + "-replaced"
	if err := os.Rename(stateRoot, movedStateRoot); err != nil {
		t.Fatal(err)
	}
	if err := os.Mkdir(stateRoot, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := validateCurrentSyncIntent(intent, fixture.caseRoot, stateRoot); err == nil || !strings.Contains(err.Error(), "state root physical identity changed") {
		t.Fatalf("current sync replaced state root error = %v", err)
	}
}

func currentSyncCloneIntentTest(t *testing.T, value currentSyncIntent) currentSyncIntent {
	t.Helper()
	data, err := json.Marshal(value)
	if err != nil {
		t.Fatal(err)
	}
	var clone currentSyncIntent
	if err := json.Unmarshal(data, &clone); err != nil {
		t.Fatal(err)
	}
	return clone
}

func TestCurrentSyncAlreadyCurrentIsPureNoOp(t *testing.T) {
	fixture := newCurrentSyncFixture(t, "")
	initial, err := CurrentSyncPreview(fixture.repoRoot, fixture.caseRoot, fixture.pack, CurrentSyncOptions{SourceExecutable: fixture.sourceExecutable})
	if err != nil {
		t.Fatal(err)
	}
	for _, publication := range initial.prepared.publications {
		target := filepath.Join(fixture.caseRoot, projectstate.CurrentDir, filepath.FromSlash(publication.rel))
		if err := os.MkdirAll(filepath.Dir(target), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(target, publication.data, publication.mode); err != nil {
			t.Fatal(err)
		}
	}
	for _, leaf := range initial.prepared.leaves {
		target := filepath.Join(fixture.caseRoot, filepath.FromSlash(leaf.rel))
		if !leaf.afterExists {
			if err := os.Remove(target); err != nil && !os.IsNotExist(err) {
				t.Fatal(err)
			}
			continue
		}
		if err := os.MkdirAll(filepath.Dir(target), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(target, leaf.after, leaf.mode); err != nil {
			t.Fatal(err)
		}
	}
	for _, rel := range initial.ObsoleteControlled {
		if err := os.Remove(filepath.Join(fixture.caseRoot, filepath.FromSlash(rel))); err != nil && !os.IsNotExist(err) {
			t.Fatal(err)
		}
	}
	plan, err := CurrentSyncPreview(fixture.repoRoot, fixture.caseRoot, fixture.pack, CurrentSyncOptions{SourceExecutable: fixture.sourceExecutable})
	if err != nil {
		t.Fatal(err)
	}
	if !plan.AlreadyCurrent || plan.Status != "already-current" || plan.RequiresConfirmation || len(plan.ApplyArgs) != 0 {
		t.Fatalf("current sync already-current plan was not a pure no-op: %+v", plan)
	}
	if _, err := buildCurrentSyncIntent(plan); err == nil || !strings.Contains(err.Error(), "reviewed plan state is invalid") {
		t.Fatalf("current sync accepted already-current mutation intent: %v", err)
	}
}

func TestCurrentSyncProgressLimitCoversExactJournal(t *testing.T) {
	fixture := newCurrentSyncFixture(t, "")
	plan, err := CurrentSyncPreview(fixture.repoRoot, fixture.caseRoot, fixture.pack, CurrentSyncOptions{SourceExecutable: fixture.sourceExecutable})
	if err != nil {
		t.Fatal(err)
	}
	intent, err := buildCurrentSyncIntent(plan)
	if err != nil {
		t.Fatal(err)
	}
	maxGenerations, err := currentSyncMaxProgressGenerations(intent)
	if err != nil {
		t.Fatal(err)
	}
	if want := 1 + 2*len(currentSyncExpectedProgressOperations(intent)); maxGenerations != want {
		t.Fatalf("current sync progress generation limit = %d, want %d", maxGenerations, want)
	}
	if maxGenerations <= currentSyncMaxFiles {
		t.Skip("fixture journal does not exceed controlled-file limit")
	}
}

func TestCurrentSyncPreviewRejectsForceRefresh(t *testing.T) {
	fixture := newCurrentSyncFixture(t, "")
	before := snapshotFiles(t, fixture.caseRoot)

	_, err := CurrentSyncPreview(fixture.repoRoot, fixture.caseRoot, fixture.pack, CurrentSyncOptions{
		ForceLocalTemplates: true,
		SourceExecutable:    fixture.sourceExecutable,
	})
	if err == nil || !strings.Contains(err.Error(), "does not accept -Force") {
		t.Fatalf("current sync force error = %v", err)
	}
	assertSnapshotEqual(t, before, snapshotFiles(t, fixture.caseRoot))
}

func TestCurrentSyncPreviewPreservesExistingProtectedTemplate(t *testing.T) {
	fixture := newCurrentSyncFixture(t, "")

	plan, err := CurrentSyncPreview(fixture.repoRoot, fixture.caseRoot, fixture.pack, CurrentSyncOptions{SourceExecutable: fixture.sourceExecutable})
	if err != nil {
		t.Fatal(err)
	}
	leaf := currentSyncTestLeaf(t, plan, "references/template/task-handoff.md")
	if leaf.action != "skip-existing-local-file" || !bytes.Equal(leaf.before, leaf.after) {
		t.Fatalf("protected handoff target was not preserved: action=%s before=%q after=%q", leaf.action, leaf.before, leaf.after)
	}
}

func TestCurrentSyncPlanLeavesRejectsProtectedManagedTargets(t *testing.T) {
	fixture := newCurrentSyncFixture(t, "")
	m, err := manifest.Load(fixture.repoRoot, fixture.pack)
	if err != nil {
		t.Fatal(err)
	}
	m.ManagedFiles = append(m.ManagedFiles, "references/template/task-handoff.md")

	_, _, err = currentSyncPlanLeaves(fixture.repoRoot, fixture.caseRoot, filepath.Join(fixture.caseRoot, projectstate.CurrentDir), m, fixture.pack, "project", strings.Repeat("a", 64), []byte("validated skill\n"))
	if err == nil || !strings.Contains(err.Error(), "protected as") {
		t.Fatalf("current sync protected managed target error = %v", err)
	}
}

func TestCurrentSyncPlanLeavesAllowsDeclaredManagedBlockInLocalHost(t *testing.T) {
	fixture := newCurrentSyncFixture(t, "")
	m, err := manifest.Load(fixture.repoRoot, fixture.pack)
	if err != nil {
		t.Fatal(err)
	}
	if !slices.Contains(m.LocalFiles, m.ManagedBlock["file"]) {
		m.LocalFiles = append(m.LocalFiles, m.ManagedBlock["file"])
	}
	leaves, _, err := currentSyncPlanLeaves(
		fixture.repoRoot,
		fixture.caseRoot,
		filepath.Join(fixture.caseRoot, projectstate.CurrentDir),
		m,
		fixture.pack,
		"project",
		strings.Repeat("a", 64),
		[]byte("validated skill\n"),
	)
	if err != nil {
		t.Fatal(err)
	}
	for _, leaf := range leaves {
		if leaf.kind != "managed-block" {
			continue
		}
		if leaf.rel != m.ManagedBlock["file"] ||
			!bytes.Contains(leaf.after, []byte("local prefix")) ||
			!bytes.Contains(leaf.after, []byte(m.ManagedBlock["blockId"])) {
			t.Fatalf("current sync managed block did not preserve its local host: %+v", leaf)
		}
		return
	}
	t.Fatal("current sync plan omitted declared managed block")
}

func TestCurrentSyncPlanLeavesRejectsStateRootAndDuplicateTargets(t *testing.T) {
	fixture := newCurrentSyncFixture(t, "")
	m, err := manifest.Load(fixture.repoRoot, fixture.pack)
	if err != nil {
		t.Fatal(err)
	}

	t.Run("mutable-current-state", func(t *testing.T) {
		copy := *m
		copy.ManagedFiles = append([]string{}, m.ManagedFiles...)
		copy.ManagedFiles = append(copy.ManagedFiles, ".steamai/lanes/main/authority.json")
		_, _, err := currentSyncPlanLeaves(fixture.repoRoot, fixture.caseRoot, filepath.Join(fixture.caseRoot, projectstate.CurrentDir), &copy, fixture.pack, "project", strings.Repeat("a", 64), []byte("validated skill\n"))
		if err == nil || !strings.Contains(err.Error(), "outside the allowed refresh leaves") {
			t.Fatalf("current sync mutable state target error = %v", err)
		}
	})

	t.Run("duplicate", func(t *testing.T) {
		copy := *m
		copy.ManagedFiles = append([]string{}, m.ManagedFiles...)
		copy.ManagedFiles = append(copy.ManagedFiles, copy.ManagedFiles[0])
		_, _, err := currentSyncPlanLeaves(fixture.repoRoot, fixture.caseRoot, filepath.Join(fixture.caseRoot, projectstate.CurrentDir), &copy, fixture.pack, "project", strings.Repeat("a", 64), []byte("validated skill\n"))
		if err == nil || !strings.Contains(err.Error(), "declared more than once") {
			t.Fatalf("current sync duplicate target error = %v", err)
		}
	})
}

func TestCurrentSyncNormalizeTargetRelRejectsNonPortablePaths(t *testing.T) {
	for _, value := range []string{"../escape", `.rekit\\state.json`, `NUL.txt`, `state.json:stream`, `C:relative`, `trailing.`, `double//separator`} {
		if _, err := currentSyncNormalizeTargetRel(value); err == nil {
			t.Errorf("current sync accepted unsafe target path %q", value)
		}
	}
}

func TestCurrentSyncPreviewRejectsNonExternalMaintenanceSources(t *testing.T) {
	fixture := newCurrentSyncFixture(t, "")

	t.Run("source-executable-is-not-running-maintenance-process", func(t *testing.T) {
		other := filepath.Join(t.TempDir(), runtimebundle.ExecutableName())
		if err := os.WriteFile(other, []byte("not the running maintenance process\n"), 0o700); err != nil {
			t.Fatal(err)
		}
		_, err := CurrentSyncPreview(fixture.repoRoot, fixture.caseRoot, fixture.pack, CurrentSyncOptions{SourceExecutable: other})
		if err == nil || !strings.Contains(err.Error(), "must be the running central maintenance process image") {
			t.Fatalf("current sync unrelated source executable error = %v", err)
		}
	})

	t.Run("source-repository-is-target-project", func(t *testing.T) {
		_, err := CurrentSyncPreview(fixture.caseRoot, fixture.caseRoot, fixture.pack, CurrentSyncOptions{SourceExecutable: fixture.sourceExecutable})
		if err == nil || !strings.Contains(err.Error(), "external to the target project") {
			t.Fatalf("current sync source project alias error = %v", err)
		}
	})

	t.Run("target-project-is-inside-source-repository", func(t *testing.T) {
		nested := newCurrentSyncFixture(t, filepath.Join(fixture.repoRoot, "nested-target"))
		_, err := CurrentSyncPreview(fixture.repoRoot, nested.caseRoot, fixture.pack, CurrentSyncOptions{SourceExecutable: fixture.sourceExecutable})
		if err == nil || !strings.Contains(err.Error(), "external to the target project") {
			t.Fatalf("current sync nested target error = %v", err)
		}
	})

	t.Run("source-executable-is-target-runtime-hardlink", func(t *testing.T) {
		alias := filepath.Join(t.TempDir(), runtimebundle.ExecutableName())
		if err := os.Link(fixture.targetExecutable, alias); err != nil {
			t.Fatal(err)
		}
		_, err := CurrentSyncPreview(fixture.repoRoot, fixture.caseRoot, fixture.pack, CurrentSyncOptions{SourceExecutable: alias})
		if err == nil || !strings.Contains(err.Error(), "physically distinct from the target runtime") {
			t.Fatalf("current sync target runtime hardlink error = %v", err)
		}
	})
}

func newCurrentSyncFixture(t *testing.T, caseRoot string) currentSyncFixtureState {
	t.Helper()
	repoRoot, pack := exclusiveInitFixture(t)
	manifestPath := filepath.Join(repoRoot, "packs", pack, "manifest.yml")
	manifestBytes, err := os.ReadFile(manifestPath)
	if err != nil {
		t.Fatal(err)
	}
	manifestText := strings.Replace(string(manifestBytes), "authorityFiles:\n  - references/template/README.md", "authorityFiles:\n  - references/template/task-handoff.md", 1)
	manifestText = strings.Replace(manifestText, "  requestDefaultTargetLane: main\n", "  requestDefaultTargetLane: main\n  handoffPath: references/template/task-handoff.md\n", 1)
	manifestText = strings.Replace(manifestText, "    canWrite: references/template/README.md\n", "    canWrite: references/template/task-handoff.md\n", 1)
	writeText(t, manifestPath, manifestText)
	sourceExecutable, err := os.Executable()
	if err != nil {
		t.Fatal(err)
	}
	oldRepo := filepath.Join(t.TempDir(), "old-repo")
	if err := copyTreeForExclusiveTest(repoRoot, oldRepo); err != nil {
		t.Fatal(err)
	}
	writeText(t, filepath.Join(oldRepo, "common", "policies", "obsolete-current-sync-test.md"), "obsolete controlled asset\n")
	if caseRoot == "" {
		caseRoot = filepath.Join(t.TempDir(), "case")
	}
	if err := os.MkdirAll(caseRoot, 0o755); err != nil {
		t.Fatal(err)
	}
	oldExecutable := filepath.Join(t.TempDir(), runtimebundle.ExecutableName())
	if err := os.WriteFile(oldExecutable, []byte("old current sync runtime executable\n"), 0o700); err != nil {
		t.Fatal(err)
	}
	bundle, err := runtimebundle.PublishForTest(caseRoot, oldRepo, pack, oldExecutable)
	if err != nil {
		t.Fatal(err)
	}
	stateRoot := filepath.Join(caseRoot, projectstate.CurrentDir)
	writeText(t, filepath.Join(stateRoot, "instance.yml"), casebind.STeamAIInstanceText(caseRoot, pack, "current-project", runtimebundle.ManifestRel, bundle.ManifestSHA256))
	state := map[string]any{
		"schemaVersion": 1,
		"templateRoot":  ".",
		"templatePack":  pack,
		"lastSyncAt":    "2026-01-01T00:00:00Z",
		"managed": map[string]syncManagedEntry{
			"references/template/README.md":               {SourceHash: strings.Repeat("1", 64), TargetHashAtSync: strings.Repeat("2", 64), LastAction: "sync"},
			"references/template/obsolete-managed.md":     {SourceHash: strings.Repeat("5", 64), TargetHashAtSync: strings.Repeat("6", 64), LastAction: "sync"},
			"references/template/retained-pack-memory.md": {SourceHash: strings.Repeat("3", 64), TargetHashAtSync: strings.Repeat("4", 64), LastAction: "pack-memory-selected-sync"},
		},
		"futureState": map[string]bool{"preserve": true},
	}
	stateData, err := json.MarshalIndent(state, "", "  ")
	if err != nil {
		t.Fatal(err)
	}
	writeText(t, filepath.Join(stateRoot, "state.json"), string(append(stateData, '\n')))
	writeText(t, filepath.Join(caseRoot, filepath.FromSlash("references/template/README.md")), "local managed drift\n")
	writeText(t, filepath.Join(caseRoot, filepath.FromSlash("references/template/task-handoff.md")), "local handoff stays unless forced\n")
	writeText(t, filepath.Join(caseRoot, "CLAUDE.local.md"), "local prefix\n")
	writeText(t, filepath.Join(caseRoot, ".claude", "skills", "steamai", "SKILL.md"), "old project skill\n")
	writeText(t, filepath.Join(stateRoot, "lanes", "main", "sentinel.json"), "{\"preserve\":true}\n")
	return currentSyncFixtureState{
		repoRoot:         repoRoot,
		caseRoot:         caseRoot,
		pack:             pack,
		sourceExecutable: sourceExecutable,
		targetExecutable: filepath.Join(stateRoot, "runtime", "bin", runtimebundle.ExecutableName()),
	}
}

func mustCurrentSyncPlanSHA256(t *testing.T, plan CurrentSyncPlan) string {
	t.Helper()
	sha, err := currentSyncCanonicalSHA(currentSyncIdentity(plan))
	if err != nil {
		t.Fatal(err)
	}
	return sha
}

func currentSyncTestLeaf(t *testing.T, plan CurrentSyncPlan, rel string) currentSyncLeaf {
	t.Helper()
	if plan.prepared == nil {
		t.Fatal("current sync preview omitted runtime-only prepared state")
	}
	for _, leaf := range plan.prepared.leaves {
		if leaf.rel == rel {
			return leaf
		}
	}
	t.Fatalf("current sync leaf not found: %s", rel)
	return currentSyncLeaf{}
}

func currentSyncTestArgValue(args []string, flag string) (string, bool) {
	for index := 0; index+1 < len(args); index++ {
		if args[index] == flag {
			return args[index+1], true
		}
	}
	return "", false
}

func currentSyncTestHasArg(args []string, flag string) bool {
	return slices.Contains(args, flag)
}

func currentSyncTestContains(values []string, want string) bool {
	return slices.Contains(values, want)
}
