package sessionhost

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
	"time"

	"github.com/shuiyu486/re-context-kits/internal/rekit/adapterhost"
	"github.com/shuiyu486/re-context-kits/internal/rekit/autonomy"
	"github.com/shuiyu486/re-context-kits/internal/rekit/binaryinventory"
	"github.com/shuiyu486/re-context-kits/internal/rekit/executioncontrol"
	"github.com/shuiyu486/re-context-kits/internal/rekit/gate"
	"github.com/shuiyu486/re-context-kits/internal/rekit/laneowner"
	"github.com/shuiyu486/re-context-kits/internal/rekit/memberexecution"
	"github.com/shuiyu486/re-context-kits/internal/rekit/mission"
	"github.com/shuiyu486/re-context-kits/internal/rekit/workstream"
)

func TestBinaryREAdapterLifecycleRecoversAcknowledgedDurableTailWithoutRelaunch(t *testing.T) {
	repoRoot := sessionhostTestRepoRoot(t)
	caseRoot := provisionSessionhostAttachedCase(t, repoRoot, liveAcceptancePack)
	bootstrap := DailyResult{CaseRoot: caseRoot}
	intent, err := applyDailyOnboarding(
		caseRoot,
		"inspect harmless synthetic existing IDA indexes",
		"binary-re-lifecycle-test",
		&bootstrap,
	)
	if err != nil {
		t.Fatal(err)
	}
	bootstrap.Lane = intent.Identity.InitialLane
	if err := ensureDailyStarted(caseRoot, intent.Identity.Pack, &bootstrap); err != nil {
		t.Fatal(err)
	}
	proof := &LiveAcceptanceVMPIDA{}
	if err := prepareLiveAcceptanceVMPIDA(caseRoot, intent.Identity.Pack, bootstrap.Lane, proof); err != nil {
		t.Fatal(err)
	}

	previousNow := binaryREAdapterNow
	previousReviewer := binaryREEvidenceReviewRunClaude
	binaryREAdapterNow = func() string { return "2026-08-19T01:00:00Z" }
	t.Cleanup(func() {
		binaryREAdapterNow = previousNow
		binaryREEvidenceReviewRunClaude = previousReviewer
		if root, rootErr := claudeRecoveryRootPath(caseRoot); rootErr == nil {
			_ = os.RemoveAll(root)
		}
		if root, rootErr := claudeRawResultRoot(caseRoot); rootErr == nil {
			_ = os.RemoveAll(root)
		}
	})

	reviewerCalls := 0
	binaryREEvidenceReviewRunClaude = func(
		_ context.Context,
		opt Options,
		pkg mission.CurrentLoopExternalSessionHarnessPackage,
		sessionID string,
		_ func() error,
	) claudeRun {
		reviewerCalls++
		if pkg.Launch == nil || opt.launchControlBinding == nil {
			return claudeRun{failureDetail: "evidence review fixture omitted its launch control binding"}
		}
		input, _, found, readErr := readBinaryREAdapterArtifact[binaryREVMPIDAEvidenceReviewInput](
			caseRoot,
			pkg.Launch.Input.Path,
			"binary-re lifecycle test evidence input",
		)
		if readErr != nil || !found {
			return claudeRun{failureDetail: "read evidence review fixture: " + errorText(readErr)}
		}
		response, marshalErr := json.Marshal(evidenceReviewResponse{
			Decision:            "accepted",
			Summary:             "exact synthetic evidence lineage",
			Reason:              "request, selected row, report, receipt, and observation agree",
			EvidenceRefs:        append([]string{}, input.EvidenceRefs...),
			SelectedEvidenceRef: input.Selected.EvidenceRef,
			ObservationEventID:  input.ObservationEventID,
			ReceiptSHA256:       input.ReceiptSHA256,
		})
		if marshalErr != nil {
			return claudeRun{failureDetail: marshalErr.Error()}
		}
		return claudeRun{
			launchControlBinding: cloneClaudeLaunchControlBinding(opt.launchControlBinding),
			envelope: claudeEnvelope{
				Type:      "result",
				Subtype:   "success",
				SessionID: sessionID,
			},
			sessionID:        sessionID,
			structuredOutput: response,
			started:          true,
			exitCode:         0,
			observedAt:       "2026-08-19T01:00:01Z",
		}
	}

	adapterPath, err := os.Executable()
	if err != nil {
		t.Fatal(err)
	}
	adapterCalls := 0
	runnerObservedIntent := false
	runner := func(
		path string,
		opt adapterhost.AuthorizedRunOptions,
		timeout time.Duration,
	) (adapterhost.AuthorizedRunResult, int, error) {
		adapterCalls++
		intentPath, pathErr := binaryREAdapterArtifactPath(caseRoot, bootstrap.Lane, opt.GateEventID, "execution-intent.json")
		if pathErr != nil {
			return adapterhost.AuthorizedRunResult{}, 0, pathErr
		}
		stored, _, found, readErr := readBinaryREAdapterArtifact[binaryREAdapterExecutionIntent](
			caseRoot,
			intentPath,
			"binary-re lifecycle test execution intent",
		)
		if readErr != nil {
			return adapterhost.AuthorizedRunResult{}, 0, readErr
		}
		runnerObservedIntent = found && stored.GateEventID == opt.GateEventID &&
			opt.ExecutionControlBinding != nil && stored.Control == *opt.ExecutionControlBinding
		return adapterhost.RunAuthorizedGateProcess(path, opt, timeout)
	}
	dailyOpt := DailyOptions{
		Target:                            caseRoot,
		SelectedLane:                      bootstrap.Lane,
		ExpectedClaudeExecutableSHA256:    strings.Repeat("a", 64),
		ExpectedClaudeExecutablePublisher: liveAcceptanceClaudePublisher,
		binaryREAdapterPath:               adapterPath,
		binaryREAdapterRunner:             runner,
		evidenceReviewRunner:              binaryREEvidenceReviewRunClaude,
	}

	fresh, found, err := runBinaryREAdapterLifecycle(
		context.Background(), dailyOpt, caseRoot, intent.Identity.Pack, bootstrap.Lane,
	)
	if err != nil {
		t.Fatal(err)
	}
	if !found || !fresh.ReadyForMember || fresh.State != "ready-for-member" ||
		fresh.AdapterReplay || fresh.EvidenceReviewReplay || !fresh.ChildLaunched ||
		fresh.AdapterProcessID <= 0 || fresh.Run == nil || !fresh.Run.ChildLaunched ||
		fresh.ResultPublication == nil || !fresh.ResultPublication.Published ||
		adapterCalls != 1 || reviewerCalls != 1 || !runnerObservedIntent {
		t.Fatalf("fresh ordinary binary-re lifecycle = %+v adapterCalls=%d reviewerCalls=%d intentBeforeRunner=%t", fresh, adapterCalls, reviewerCalls, runnerObservedIntent)
	}

	executionIntent, executionIntentData, executionIntentFound, err := readBinaryREAdapterArtifact[binaryREAdapterExecutionIntent](
		caseRoot,
		fresh.ExecutionIntentPath,
		"binary-re lifecycle test execution intent",
	)
	if err != nil || !executionIntentFound {
		t.Fatalf("read execution intent: found=%t err=%v", executionIntentFound, err)
	}
	executionResult, _, executionResultFound, err := readBinaryREAdapterArtifact[binaryREAdapterExecutionResult](
		caseRoot,
		fresh.ExecutionResultPath,
		"binary-re lifecycle test execution result",
	)
	if err != nil || !executionResultFound {
		t.Fatalf("read execution result: found=%t err=%v", executionResultFound, err)
	}
	if sha, replay, persistErr := persistBinaryREAdapterExecutionResult(
		caseRoot,
		fresh.ExecutionResultPath,
		executionResult,
		executionIntent,
		fresh.ExecutionIntentPath,
		bytesSHA256(executionIntentData),
	); persistErr != nil || !replay || !strings.EqualFold(sha, fresh.ExecutionResultSHA256) {
		t.Fatalf("exact execution result replay: sha=%s replay=%t err=%v", sha, replay, persistErr)
	}
	driftedExecution := executionResult
	driftedExecution.Run.ChildProcessID++
	if _, _, persistErr := persistBinaryREAdapterExecutionResult(
		caseRoot,
		fresh.ExecutionResultPath,
		driftedExecution,
		executionIntent,
		fresh.ExecutionIntentPath,
		bytesSHA256(executionIntentData),
	); persistErr == nil || !strings.Contains(persistErr.Error(), "differs from the durable terminal result") {
		t.Fatalf("drifted execution result error = %v", persistErr)
	}

	closureData := readBinaryRELifecycleTestFile(t, caseRoot, fresh.ClosurePath)
	bindingData := readBinaryRELifecycleTestFile(t, caseRoot, fresh.TaskBindingPath)
	receiptData := readBinaryRELifecycleTestFile(t, caseRoot, fresh.Run.ReceiptPath)
	ackID, ackSHA := fresh.AcknowledgementEventID, fresh.AcknowledgementSHA256
	if err := os.Remove(filepath.Join(caseRoot, filepath.FromSlash(fresh.ClosurePath))); err != nil {
		t.Fatal(err)
	}
	if err := os.Remove(filepath.Join(caseRoot, filepath.FromSlash(fresh.TaskBindingPath))); err != nil {
		t.Fatal(err)
	}

	recoveredClosure, found, err := runBinaryREAdapterLifecycle(
		context.Background(), dailyOpt, caseRoot, intent.Identity.Pack, bootstrap.Lane,
	)
	if err != nil {
		t.Fatal(err)
	}
	if !found || !recoveredClosure.ReadyForMember || !recoveredClosure.AdapterReplay ||
		!recoveredClosure.EvidenceReviewReplay || adapterCalls != 1 || reviewerCalls != 1 ||
		recoveredClosure.AcknowledgementEventID != ackID ||
		!strings.EqualFold(recoveredClosure.AcknowledgementSHA256, ackSHA) {
		t.Fatalf("closure recovery = %+v adapterCalls=%d reviewerCalls=%d", recoveredClosure, adapterCalls, reviewerCalls)
	}
	assertBinaryRELifecycleTestFile(t, caseRoot, recoveredClosure.ClosurePath, closureData)
	assertBinaryRELifecycleTestFile(t, caseRoot, recoveredClosure.TaskBindingPath, bindingData)
	assertBinaryRELifecycleTestFile(t, caseRoot, recoveredClosure.Run.ReceiptPath, receiptData)

	if err := os.Remove(filepath.Join(caseRoot, filepath.FromSlash(recoveredClosure.TaskBindingPath))); err != nil {
		t.Fatal(err)
	}
	recoveredBinding, found, err := runBinaryREAdapterLifecycle(
		context.Background(), dailyOpt, caseRoot, intent.Identity.Pack, bootstrap.Lane,
	)
	if err != nil {
		t.Fatal(err)
	}
	if !found || !recoveredBinding.ReadyForMember || !recoveredBinding.AdapterReplay ||
		!recoveredBinding.EvidenceReviewReplay || adapterCalls != 1 || reviewerCalls != 1 ||
		recoveredBinding.ClosureSHA256 != recoveredClosure.ClosureSHA256 ||
		recoveredBinding.TaskBindingSHA256 != recoveredClosure.TaskBindingSHA256 {
		t.Fatalf("binding recovery = %+v adapterCalls=%d reviewerCalls=%d", recoveredBinding, adapterCalls, reviewerCalls)
	}
	assertBinaryRELifecycleTestFile(t, caseRoot, recoveredBinding.TaskBindingPath, bindingData)
	assertBinaryRELifecycleTestFile(t, caseRoot, recoveredBinding.Run.ReceiptPath, receiptData)

	beforeCompleteReplay, err := liveAcceptanceTreeSHA256(caseRoot)
	if err != nil {
		t.Fatal(err)
	}
	completed, found, err := runBinaryREAdapterLifecycle(
		context.Background(), dailyOpt, caseRoot, intent.Identity.Pack, bootstrap.Lane,
	)
	if err != nil {
		t.Fatal(err)
	}
	afterCompleteReplay, err := liveAcceptanceTreeSHA256(caseRoot)
	if err != nil {
		t.Fatal(err)
	}
	if found || completed.Kind != "" || beforeCompleteReplay != afterCompleteReplay || adapterCalls != 1 || reviewerCalls != 1 {
		t.Fatalf("completed lifecycle replay = %+v found=%t treeBefore=%s treeAfter=%s adapterCalls=%d reviewerCalls=%d", completed, found, beforeCompleteReplay, afterCompleteReplay, adapterCalls, reviewerCalls)
	}

	pause, err := executioncontrol.Preview(caseRoot, executioncontrol.Options{
		Lane: bootstrap.Lane, Action: executioncontrol.ActionPause, Actor: "binary-re-lifecycle-test",
		Reason: "prove stale terminal result cannot replay across control head", PublicationStamp: "2026-08-19T01:00:02Z",
	})
	if err != nil {
		t.Fatal(err)
	}
	if _, err := executioncontrol.Apply(caseRoot, executioncontrol.Options{
		Lane: pause.Lane, Action: pause.Action, Actor: pause.Actor, Reason: pause.Reason,
		PublicationStamp: pause.PublicationStamp, ExpectedPlanSHA256: pause.ExpectedPlanSHA256,
	}); err != nil {
		t.Fatal(err)
	}
	if _, _, persistErr := persistBinaryREAdapterExecutionResult(
		caseRoot,
		fresh.ExecutionResultPath,
		executionResult,
		executionIntent,
		fresh.ExecutionIntentPath,
		bytesSHA256(executionIntentData),
	); persistErr == nil || !strings.Contains(persistErr.Error(), "lane execution is paused") {
		t.Fatalf("stale execution result error = %v", persistErr)
	}
}

type binaryInventoryLifecycleFixture struct {
	repoRoot   string
	caseRoot   string
	pack       string
	lane       string
	sourcePath string
	gateID     string
}

func newBinaryInventoryLifecycleFixture(t *testing.T) binaryInventoryLifecycleFixture {
	t.Helper()
	sourceRepoRoot := sessionhostTestRepoRoot(t)
	caseRoot := provisionSessionhostAttachedCase(t, sourceRepoRoot, liveAcceptancePack)
	bootstrap := DailyResult{CaseRoot: caseRoot}
	intent, err := applyDailyOnboarding(caseRoot, "inspect one harmless synthetic PE", "binary-inventory-lifecycle-test", &bootstrap)
	if err != nil {
		t.Fatal(err)
	}
	bootstrap.Lane = intent.Identity.InitialLane
	if err := ensureDailyStarted(caseRoot, intent.Identity.Pack, &bootstrap); err != nil {
		t.Fatal(err)
	}
	repoRoot, err := runtimeContextForDailyPack(caseRoot, intent.Identity.Pack)
	if err != nil {
		t.Fatal(err)
	}
	board, err := mission.ReadBoard(caseRoot)
	if err != nil {
		t.Fatal(err)
	}
	selected, found := mission.LookupBoardLane(board.Lanes, bootstrap.Lane, false)
	if !found || strings.TrimSpace(selected.Workspace) == "" {
		t.Fatalf("binary inventory fixture lane workspace is unavailable: %+v", selected)
	}
	sourcePath := "inputs/synthetic-pe.bin"
	sourceFull := filepath.Join(caseRoot, filepath.FromSlash(sourcePath))
	if err := os.MkdirAll(filepath.Dir(sourceFull), 0o700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(sourceFull, syntheticPEForBinaryRELifecycleTest(), 0o600); err != nil {
		t.Fatal(err)
	}
	current, _, exists, err := autonomy.Read(caseRoot, bootstrap.Lane)
	if err != nil || !exists || !reflect.DeepEqual(current, autonomy.DefaultProfile(bootstrap.Lane)) {
		t.Fatalf("binary inventory fixture manual profile = %+v exists=%t err=%v", current, exists, err)
	}
	outputRoot := filepath.ToSlash(filepath.Join(selected.Workspace, "inventory", "session-1"))
	now := time.Now().UTC()
	plan, err := autonomy.PreviewProvision(autonomy.ProfileProvisionOptions{
		RepoRoot: repoRoot, CaseRoot: caseRoot, Pack: intent.Identity.Pack, Lane: bootstrap.Lane,
		Profile: autonomy.Profile{
			SchemaVersion: 1, ProfileID: "binary-inventory-lifecycle", Lane: bootstrap.Lane, Mode: autonomy.ModePreauthorized,
			AllowedActions: []string{"inspect"}, TargetScope: []autonomy.Target{{Match: "exact", Value: sourcePath}},
			Budget:         autonomy.Budget{RuntimeSeconds: 10, DiskMB: 4, Requests: 1},
			StopConditions: []string{"scope-drift", "source-drift", "output-exceeds-bounded-evidence-packet"},
			OutputPaths:    []string{outputRoot}, RecordRequired: true, NotifyMainOn: []string{"boundary-hit", "new-risk"},
			GrantedBy: "binary-inventory-lifecycle-test", GrantedAt: now.Add(-time.Minute).Format(time.RFC3339),
			ExpiresAt: now.Add(10 * time.Minute).Format(time.RFC3339),
		},
	})
	if err != nil {
		t.Fatal(err)
	}
	if _, err := autonomy.ApplyProfilePlan(plan, plan.ExpectedPlanSHA256); err != nil {
		t.Fatal(err)
	}
	authorized, err := gate.Apply(repoRoot, caseRoot, intent.Identity.Pack, gate.Options{
		Action: "inspect", Lane: bootstrap.Lane, Actor: "binary-inventory-lifecycle-test",
		Subject: "inspect synthetic PE", Summary: "bounded typed PE inventory", TargetRef: sourcePath,
		RuntimeSeconds: 10, DiskMB: 4, Requests: 1, OutputPaths: outputRoot,
		StopConditions: "scope-drift,source-drift,output-exceeds-bounded-evidence-packet",
	})
	if err != nil || !authorized.Applied || authorized.Event == nil || authorized.Event.Gate.Authorization.Decision != autonomy.DecisionPreauthorized {
		t.Fatalf("binary inventory fixture gate = %+v err=%v", authorized, err)
	}
	return binaryInventoryLifecycleFixture{
		repoRoot: repoRoot, caseRoot: caseRoot, pack: intent.Identity.Pack,
		lane: bootstrap.Lane, sourcePath: sourcePath, gateID: authorized.EventID,
	}
}

func TestBinaryInventoryAdapterLifecycleAcceptsBindsAndReplaysWithoutRelaunch(t *testing.T) {
	fixture := newBinaryInventoryLifecycleFixture(t)
	previousNow := binaryREAdapterNow
	previousReviewer := binaryREEvidenceReviewRunClaude
	binaryREAdapterNow = func() string { return "2026-08-21T01:00:00Z" }
	t.Cleanup(func() {
		binaryREAdapterNow = previousNow
		binaryREEvidenceReviewRunClaude = previousReviewer
		if root, err := claudeRecoveryRootPath(fixture.caseRoot); err == nil {
			_ = os.RemoveAll(root)
		}
		if root, err := claudeRawResultRoot(fixture.caseRoot); err == nil {
			_ = os.RemoveAll(root)
		}
	})
	reviewerCalls := 0
	binaryREEvidenceReviewRunClaude = binaryInventoryLifecycleReviewer(t, fixture.caseRoot, "accepted", &reviewerCalls)
	adapterPath, err := os.Executable()
	if err != nil {
		t.Fatal(err)
	}
	adapterCalls := 0
	runner := func(path string, opt adapterhost.AuthorizedRunOptions, timeout time.Duration) (adapterhost.AuthorizedRunResult, int, error) {
		adapterCalls++
		if opt.AdapterID != binaryinventory.AdapterID || opt.GateEventID != fixture.gateID {
			return adapterhost.AuthorizedRunResult{}, 0, fmt.Errorf("generic lifecycle runner received wrong adapter binding")
		}
		return adapterhost.RunAuthorizedGateProcess(path, opt, timeout)
	}
	opt := DailyOptions{
		Target: fixture.caseRoot, SelectedLane: fixture.lane,
		ExpectedClaudeExecutableSHA256: strings.Repeat("a", 64), ExpectedClaudeExecutablePublisher: liveAcceptanceClaudePublisher,
		binaryREAdapterPath: adapterPath, binaryREAdapterRunner: runner,
		evidenceReviewRunner: binaryREEvidenceReviewRunClaude,
	}
	first, found, err := runBinaryREAdapterLifecycle(context.Background(), opt, fixture.caseRoot, fixture.pack, fixture.lane)
	if err != nil {
		t.Fatal(err)
	}
	if !found || !first.ReadyForMember || first.State != "ready-for-member" ||
		first.AdapterID != binaryinventory.AdapterID || first.EvidenceReviewDecision != "accepted" ||
		first.SelectedEvidenceRef != first.Run.PacketPath || !first.ChildLaunched || first.AdapterReplay || first.EvidenceReviewReplay ||
		adapterCalls != 1 || reviewerCalls != 1 {
		t.Fatalf("fresh binary inventory lifecycle = %+v adapterCalls=%d reviewerCalls=%d", first, adapterCalls, reviewerCalls)
	}
	binding, bindingPath, bindingSHA, err := memberexecution.ReadTaskBindingForOwner(fixture.caseRoot, fixture.lane, binaryInventoryLifecycleGeneration(t, fixture.caseRoot, fixture.lane))
	if err != nil {
		t.Fatal(err)
	}
	if binding == nil || binding.Kind != "binary-inventory-evidence" || bindingPath != first.TaskBindingPath ||
		!strings.EqualFold(bindingSHA, first.TaskBindingSHA256) ||
		binding.Values["source-path"] != fixture.sourcePath || binding.Values["inventory-path"] != first.Run.PacketPath ||
		binding.Values["selected-evidence-ref"] != first.Run.PacketPath || binding.Values["observation-event-id"] != first.Run.ObservationEventID {
		t.Fatalf("binary inventory member binding = %+v path=%s sha=%s", binding, bindingPath, bindingSHA)
	}
	before, err := liveAcceptanceTreeSHA256(fixture.caseRoot)
	if err != nil {
		t.Fatal(err)
	}
	second, found, err := runBinaryREAdapterLifecycle(context.Background(), opt, fixture.caseRoot, fixture.pack, fixture.lane)
	if err != nil {
		t.Fatal(err)
	}
	after, err := liveAcceptanceTreeSHA256(fixture.caseRoot)
	if err != nil {
		t.Fatal(err)
	}
	if found || second.Kind != "" || before != after || adapterCalls != 1 || reviewerCalls != 1 {
		t.Fatalf("completed binary inventory replay = %+v found=%t before=%s after=%s adapterCalls=%d reviewerCalls=%d", second, found, before, after, adapterCalls, reviewerCalls)
	}
}

func TestBinaryInventoryAdapterLifecycleRejectedStopsBeforeAcknowledgementAndBinding(t *testing.T) {
	fixture := newBinaryInventoryLifecycleFixture(t)
	previousNow := binaryREAdapterNow
	previousReviewer := binaryREEvidenceReviewRunClaude
	binaryREAdapterNow = func() string { return "2026-08-21T02:00:00Z" }
	t.Cleanup(func() {
		binaryREAdapterNow = previousNow
		binaryREEvidenceReviewRunClaude = previousReviewer
		if root, err := claudeRecoveryRootPath(fixture.caseRoot); err == nil {
			_ = os.RemoveAll(root)
		}
		if root, err := claudeRawResultRoot(fixture.caseRoot); err == nil {
			_ = os.RemoveAll(root)
		}
	})
	reviewerCalls := 0
	binaryREEvidenceReviewRunClaude = binaryInventoryLifecycleReviewer(t, fixture.caseRoot, "rejected", &reviewerCalls)
	adapterPath, err := os.Executable()
	if err != nil {
		t.Fatal(err)
	}
	adapterCalls := 0
	runner := func(path string, opt adapterhost.AuthorizedRunOptions, timeout time.Duration) (adapterhost.AuthorizedRunResult, int, error) {
		adapterCalls++
		return adapterhost.RunAuthorizedGateProcess(path, opt, timeout)
	}
	opt := DailyOptions{
		Target: fixture.caseRoot, SelectedLane: fixture.lane,
		ExpectedClaudeExecutableSHA256: strings.Repeat("a", 64), ExpectedClaudeExecutablePublisher: liveAcceptanceClaudePublisher,
		binaryREAdapterPath: adapterPath, binaryREAdapterRunner: runner,
		evidenceReviewRunner: binaryREEvidenceReviewRunClaude,
	}
	first, found, err := runBinaryREAdapterLifecycle(context.Background(), opt, fixture.caseRoot, fixture.pack, fixture.lane)
	if err != nil {
		t.Fatal(err)
	}
	if !found || first.State != "evidence-review-rejected" || first.ReadyForMember ||
		first.EvidenceReviewDecision != "rejected" || first.AcknowledgementEventID != "" ||
		first.ClosurePath != "" || first.TaskBindingPath != "" || adapterCalls != 1 || reviewerCalls != 1 {
		t.Fatalf("rejected binary inventory lifecycle = %+v adapterCalls=%d reviewerCalls=%d", first, adapterCalls, reviewerCalls)
	}
	binding, _, _, err := memberexecution.ReadTaskBindingForOwner(fixture.caseRoot, fixture.lane, binaryInventoryLifecycleGeneration(t, fixture.caseRoot, fixture.lane))
	if err != nil {
		t.Fatal(err)
	}
	if binding != nil {
		t.Fatalf("rejected binary inventory review published member binding: %+v", binding)
	}
	facts, err := mission.ReadStrictLedgerFacts(fixture.caseRoot)
	if err != nil {
		t.Fatal(err)
	}
	if workstream.ExecutionEvidenceReviewAcknowledgedIDs(facts)[fixture.gateID] {
		t.Fatal("rejected binary inventory review wrote acknowledgement")
	}
	second, found, err := runBinaryREAdapterLifecycle(context.Background(), opt, fixture.caseRoot, fixture.pack, fixture.lane)
	if err != nil {
		t.Fatal(err)
	}
	if !found || second.State != "evidence-review-rejected" || !second.AdapterReplay || !second.EvidenceReviewReplay ||
		adapterCalls != 1 || reviewerCalls != 1 {
		t.Fatalf("rejected binary inventory replay = %+v adapterCalls=%d reviewerCalls=%d", second, adapterCalls, reviewerCalls)
	}
}

func binaryInventoryLifecycleGeneration(t *testing.T, caseRoot, lane string) int {
	t.Helper()
	owner, err := laneowner.Read(caseRoot, lane)
	if err != nil {
		t.Fatal(err)
	}
	return owner.ExecutorGeneration
}

func binaryInventoryLifecycleReviewer(
	t *testing.T,
	caseRoot,
	decision string,
	calls *int,
) func(context.Context, Options, mission.CurrentLoopExternalSessionHarnessPackage, string, func() error) claudeRun {
	t.Helper()
	return func(_ context.Context, opt Options, pkg mission.CurrentLoopExternalSessionHarnessPackage, sessionID string, _ func() error) claudeRun {
		*calls++
		if pkg.Launch == nil || pkg.Launch.Input.Role != "mission-commander-binary-inventory-evidence-review-input" || opt.launchControlBinding == nil {
			return claudeRun{failureDetail: "binary inventory review fixture omitted typed role or launch control"}
		}
		input, _, found, err := readBinaryREAdapterArtifact[binaryInventoryEvidenceReviewInput](caseRoot, pkg.Launch.Input.Path, "binary inventory lifecycle review input")
		if err != nil || !found {
			return claudeRun{failureDetail: "read binary inventory review input: " + errorText(err)}
		}
		response, err := json.Marshal(evidenceReviewResponse{
			Decision: decision, Summary: "exact binary inventory lineage", Reason: "source, inventory, report, receipt, and observation agree",
			EvidenceRefs: append([]string{}, input.EvidenceRefs...), SelectedEvidenceRef: input.SelectedEvidenceRef,
			ObservationEventID: input.ObservationEventID, ReceiptSHA256: input.ReceiptSHA256,
		})
		if err != nil {
			return claudeRun{failureDetail: err.Error()}
		}
		return claudeRun{
			launchControlBinding: cloneClaudeLaunchControlBinding(opt.launchControlBinding),
			envelope:             claudeEnvelope{Type: "result", Subtype: "success", SessionID: sessionID},
			sessionID:            sessionID, structuredOutput: response, started: true, exitCode: 0,
			observedAt: "2026-08-21T02:00:01Z",
		}
	}
}

func syntheticPEForBinaryRELifecycleTest() []byte {
	data := make([]byte, 0x400)
	copy(data[0:], []byte{'M', 'Z'})
	data[0x3c] = 0x80
	copy(data[0x80:], []byte{'P', 'E', 0, 0})
	data[0x84], data[0x85] = 0x64, 0x86
	data[0x86], data[0x87] = 1, 0
	data[0x94], data[0x95] = 0xf0, 0
	data[0x96], data[0x97] = 0x02, 0
	optional := 0x98
	data[optional], data[optional+1] = 0x0b, 0x02
	data[optional+0x10], data[optional+0x11] = 0x00, 0x10
	data[optional+0x18], data[optional+0x19] = 0x00, 0x10
	data[optional+0x1c], data[optional+0x1f] = 0x00, 0x40
	data[optional+0x20], data[optional+0x21] = 0x00, 0x10
	data[optional+0x38], data[optional+0x39] = 0x00, 0x20
	data[optional+0x3c], data[optional+0x3d] = 0x00, 0x02
	data[optional+0x6c] = 0x10
	section := optional + 0xf0
	copy(data[section:], []byte(".text\x00\x00\x00"))
	data[section+8], data[section+9] = 0x00, 0x01
	data[section+12], data[section+13] = 0x00, 0x10
	data[section+16], data[section+17] = 0x00, 0x02
	data[section+20], data[section+21] = 0x00, 0x02
	data[section+36], data[section+37], data[section+38], data[section+39] = 0x20, 0x00, 0x00, 0x60
	return data
}

func readBinaryRELifecycleTestFile(t *testing.T, caseRoot, rel string) []byte {
	t.Helper()
	data, err := os.ReadFile(filepath.Join(caseRoot, filepath.FromSlash(rel)))
	if err != nil {
		t.Fatal(err)
	}
	return data
}

func assertBinaryRELifecycleTestFile(t *testing.T, caseRoot, rel string, want []byte) {
	t.Helper()
	got := readBinaryRELifecycleTestFile(t, caseRoot, rel)
	if string(got) != string(want) {
		t.Fatalf("recovered lifecycle file %s changed exact bytes", rel)
	}
}
