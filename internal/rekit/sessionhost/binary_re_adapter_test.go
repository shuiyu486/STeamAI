package sessionhost

import (
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/shuiyu486/re-context-kits/internal/rekit/adapterhost"
	"github.com/shuiyu486/re-context-kits/internal/rekit/executioncontrol"
	"github.com/shuiyu486/re-context-kits/internal/rekit/mission"
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
