package cli

import (
	"bytes"
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"slices"
	"strconv"
	"strings"
	"testing"

	"github.com/shuiyu486/re-context-kits/internal/rekit/currentloop"
	"github.com/shuiyu486/re-context-kits/internal/rekit/memberexecution"
	"github.com/shuiyu486/re-context-kits/internal/rekit/mission"
	"github.com/shuiyu486/re-context-kits/internal/rekit/runtime"
	"github.com/shuiyu486/re-context-kits/internal/rekit/workstream"
)

func TestCurrentLoopOperatorPackagesDirectReviewerResultHandoff(t *testing.T) {
	selected := &mission.MissionCommanderDriverRequest{
		Command: "/rekit run-current-loop -Target case -ResumeCurrentLoop -ExpectedCurrentLoopCheckpointSha256 checkpoint -WhatIf -Format json",
	}
	pkg := &workstream.ReviewerDispatchOperatorPackage{
		Ready:                true,
		CurrentRunLoopStepID: "save-result-input",
		Current: &workstream.ReviewerDispatchOperatorPackageItem{
			State:                  "awaiting-reviewer-result",
			ReviewerResultDropPath: ".rekit/reviews/review/results/shard.json",
		},
	}

	handoff := statusCurrentLoopExternalReviewerHandoff(pkg, selected)
	if handoff == nil || handoff.ReviewerResultDropPathRole != "direct-reviewer-result-destination" || len(handoff.ObservationContract.Alternatives) != 2 {
		t.Fatalf("direct reviewer result handoff = %+v", handoff)
	}
	attempt := handoff.Attempt
	if attempt == nil || attempt.SchemaVersion != 1 || !strings.HasPrefix(attempt.AttemptID, "reviewer-attempt-") || len(attempt.AttemptSnapshotSHA256) != 64 || attempt.SelectedAction.Kind != "observe-reviewer-terminal-state" || attempt.DurableContinuationDriverRequest == nil || attempt.DurableContinuationDriverRequest.Command != selected.Command || attempt.ReviewerResultDropPathRole != "direct-reviewer-result-destination" {
		t.Fatalf("direct reviewer attempt package = %+v", attempt)
	}
	if actual := statusCurrentLoopReviewerAttemptSHA256(attempt); actual != attempt.AttemptSnapshotSHA256 {
		t.Fatalf("direct reviewer attempt snapshot sha = %s, want %s", attempt.AttemptSnapshotSHA256, actual)
	}
	direct := handoff.ObservationContract.Alternatives[0]
	failed := handoff.ObservationContract.Alternatives[1]
	if direct.Kind != "reviewer-result-direct-write" || direct.Transition != "external-write-then-refresh-status" || direct.PreviewCommandTemplate != selected.Command || len(direct.RequiredFlags) != 0 {
		t.Fatalf("direct reviewer result observation = %+v", direct)
	}
	if failed.Kind != "reviewer-session-failed" || !strings.Contains(failed.PreviewCommandTemplate, "-ReviewerOutcome failed") || !strings.Contains(failed.PreviewCommandTemplate, "-ReviewerExitStatus <exit-status>") {
		t.Fatalf("direct reviewer failure observation = %+v", failed)
	}
}

func TestCurrentLoopOperatorOmitsUnfocusedReviewerHandoff(t *testing.T) {
	caseRoot := fullAttachedCase(t)
	var out bytes.Buffer
	if err := Run([]string{"-Command", "start", "-Target", caseRoot, "-Pack", "_template", "-Name", "review", "-Apply"}, &out); err != nil {
		t.Fatal(err)
	}
	out.Reset()
	if err := Run([]string{"-Command", "plan-subagents", "-Target", caseRoot, "-Pack", "_template", "-TaskType", "feature-analysis", "-Items", "alpha"}, &out); err != nil {
		t.Fatal(err)
	}
	appendCurrentLoopOpenIntervention(t, caseRoot, "int-current-loop-reviewer-unfocused")
	out.Reset()
	if err := Run([]string{"-Command", "status", "-Target", caseRoot, "-Pack", "_template", "-Format", "json"}, &out); err != nil {
		t.Fatal(err)
	}
	var status statusInventory
	if err := json.Unmarshal(out.Bytes(), &status); err != nil {
		t.Fatalf("unfocused reviewer status did not decode: %v\n%s", err, out.String())
	}
	operator := status.MissionControlRunbook.CurrentLoopOperator
	if status.MissionControlRunbook.Scope != "case" || operator == nil || operator.ExternalReviewerHandoff != nil || operator.SourceCurrentDriverRequest == nil || driverStepCommandName(operator.SourceCurrentDriverRequest.Command) != "reconcile" {
		t.Fatalf("unfocused reviewer handoff leaked into case operator: scope=%s operator=%+v", status.MissionControlRunbook.Scope, operator)
	}
}

func TestRunCurrentLoopAppliesBoundedCaseSteps(t *testing.T) {
	caseRoot := fullAttachedCase(t)
	var out bytes.Buffer
	if err := Run([]string{"-Command", "overview", "-Target", caseRoot, "-Pack", "_template", "-Format", "json"}, &out); err != nil {
		t.Fatal(err)
	}

	preview := runCurrentLoopPreview(t, caseRoot, 2)
	if preview.ExpectedCurrentLoopPlanSHA256 == "" || preview.InitialRoute != "case" || preview.InitialCurrentStep == nil || preview.StopReason.Code != "ready" {
		t.Fatalf("unexpected current-loop preview: %+v", preview)
	}
	if preview.Applied || preview.AppliedSteps != 0 || len(preview.Steps) != 0 || preview.SegmentCheckpoint != nil {
		t.Fatalf("current-loop preview mutated: %+v", preview)
	}
	if _, err := os.Stat(filepath.Join(caseRoot, ".rekit", "runs", "current-loop-segments")); !os.IsNotExist(err) {
		t.Fatalf("current-loop WhatIf created checkpoint namespace: %v", err)
	}

	out.Reset()
	err := Run([]string{"-Command", "run-current-loop", "-Target", caseRoot, "-Pack", "_template", "-MaxSteps", "3", "-ExpectedCurrentLoopPlanSha256", preview.ExpectedCurrentLoopPlanSHA256, "-Apply", "-Format", "json"}, &out)
	if err == nil || !strings.Contains(err.Error(), "expected plan sha256 mismatch") {
		t.Fatalf("maxSteps drift was not rejected: %v", err)
	}

	applied := runCurrentLoopApply(t, caseRoot, preview)
	if !applied.Applied || applied.AppliedSteps != 1 || len(applied.Steps) != 1 || applied.StopReason.Code != "no-progress" || applied.FinalStatus == nil {
		t.Fatalf("unexpected bounded current-loop apply: %+v", applied)
	}
	for index, receipt := range applied.Steps {
		if receipt.Step != index+1 || receipt.Route != "case" || receipt.ExpectedCurrentStepPlanSHA256 == "" || receipt.CurrentStepReceipt == nil || receipt.CurrentStepReceipt.State != "refreshed" {
			t.Fatalf("current-loop step receipt %d = %+v", index+1, receipt)
		}
	}
	if applied.Steps[0].RequestAfter == nil || applied.Steps[0].RequestAfter.Command != applied.Steps[0].RequestBefore.Command {
		t.Fatalf("no-progress receipt did not retain the repeated request: %+v", applied.Steps[0])
	}
	for _, forbidden := range []string{"authority.jsonl", "confirmed.jsonl"} {
		if _, err := os.Stat(filepath.Join(caseRoot, ".rekit", "facts", forbidden)); !os.IsNotExist(err) {
			t.Fatalf("current-loop created forbidden ledger %s: %v", forbidden, err)
		}
	}
}

func TestRunCurrentLoopAppliesTwoDistinctCaseStepsToLimit(t *testing.T) {
	caseRoot := currentLoopCaseWithOpenIntervention(t, "int-current-loop-limit")
	preview := runCurrentLoopPreview(t, caseRoot, 2)
	if preview.ExpectedCurrentLoopPlanSHA256 == "" || preview.InitialCurrentStep == nil || driverStepCommandName(preview.InitialCurrentStep.CurrentDriverRequest.Command) != "reconcile" {
		t.Fatalf("unexpected reconcile-first loop preview: %+v", preview)
	}
	applied := runCurrentLoopApply(t, caseRoot, preview)
	if !applied.Applied || applied.AppliedSteps != 2 || len(applied.Steps) != 2 || applied.StopReason.Code != "limit-reached" || applied.StopReason.Phase != "after-step" || applied.FinalStatus == nil {
		t.Fatalf("two-step loop did not reach its bound: %+v", applied)
	}
	if driverStepCommandName(applied.Steps[0].RequestBefore.Command) != "reconcile" || driverStepCommandName(applied.Steps[1].RequestBefore.Command) != "continue" || applied.Steps[0].ExpectedCurrentStepPlanSHA256 == applied.Steps[1].ExpectedCurrentStepPlanSHA256 {
		t.Fatalf("loop did not execute distinct reconcile and continue plans: %+v", applied.Steps)
	}
}

func TestRunCurrentLoopTerminalCheckpointProductPath(t *testing.T) {
	caseRoot := fullAttachedCase(t)
	var out bytes.Buffer
	if err := Run([]string{"-Command", "overview", "-Target", caseRoot, "-Pack", "_template", "-Format", "json"}, &out); err != nil {
		t.Fatal(err)
	}
	preview := runCurrentLoopPreview(t, caseRoot, 2)
	applied := runCurrentLoopApply(t, caseRoot, preview)
	if !applied.Applied || applied.AppliedSteps != 1 || applied.SegmentCheckpoint == nil {
		t.Fatalf("terminal fixture did not create a segment checkpoint: %+v", applied)
	}
	artifactPath := filepath.Join(caseRoot, filepath.FromSlash(applied.SegmentCheckpoint.ArtifactPath))
	artifactData, err := os.ReadFile(artifactPath)
	if err != nil {
		t.Fatal(err)
	}
	var artifact struct {
		Payload json.RawMessage `json:"payload"`
	}
	if err := json.Unmarshal(artifactData, &artifact); err != nil {
		t.Fatal(err)
	}
	var payload currentloop.Payload
	if err := json.Unmarshal(artifact.Payload, &payload); err != nil {
		t.Fatal(err)
	}
	payload.StatusAvailable = true
	payload.RefreshedCurrentDriverRequest = nil
	payload.RefreshedCurrentDriverRequestSHA256 = ""
	payload.Continuation = nil
	last := &payload.StepReceipts[len(payload.StepReceipts)-1]
	last.RequestAfter = nil
	last.RequestAfterSHA256 = ""
	last.CurrentStepReceipt.RefreshedCurrentDriverRequest = nil
	last.CurrentStepReceiptSHA256, _ = currentloop.ValueSHA256(last.CurrentStepReceipt)
	if err := os.Remove(artifactPath); err != nil {
		t.Fatal(err)
	}
	if _, err := currentloop.Write(repoRoot(t), caseRoot, "_template", payload); err != nil {
		t.Fatal(err)
	}
	terminal := currentloop.Inspect(repoRoot(t), caseRoot, "_template", nil)
	if terminal.Ready || terminal.State != "terminal" || terminal.Continuation != nil {
		t.Fatalf("terminal checkpoint exposed recoverable continuation: %+v", terminal)
	}

	completionEvidence := filepath.Join(caseRoot, ".rekit", "lanes", "main", "workspace", "completion-evidence.md")
	writeCompletionEvidence(t, completionEvidence, "reviewed terminal current-loop evidence")
	completionPreview := previewCompletion(t, &out, caseRoot, "main", ".rekit/lanes/main/workspace/completion-evidence.md")
	if completionPreview.Blocked || completionPreview.CompletionPlanSHA256 == "" {
		t.Fatalf("terminal current-loop completion preview is blocked: %+v", completionPreview)
	}
	completionApplied := applyCompletion(t, &out, caseRoot, completionPreview)
	if !completionApplied.Applied || completionApplied.CompletionReceipt == nil {
		t.Fatalf("terminal current-loop completion did not commit: %+v", completionApplied)
	}

	var statusOut bytes.Buffer
	if err := Run([]string{"-Command", "status", "-Target", caseRoot, "-Pack", "_template", "-Format", "json"}, &statusOut); err != nil {
		t.Fatal(err)
	}
	var fresh statusInventory
	if err := json.Unmarshal(statusOut.Bytes(), &fresh); err != nil {
		t.Fatalf("terminal new-session status did not decode: %v\n%s", err, statusOut.String())
	}
	durable := fresh.MissionControlRunbook.CurrentLoopSegment
	if durable == nil || durable.Ready || durable.State != "stale-current-driver-request" || durable.Continuation != nil {
		t.Fatalf("new-session status recovered terminal budget after durable request drift: runbook=%+v checkpoint=%+v", fresh.MissionControlRunbook, durable)
	}

	var handoffOut bytes.Buffer
	if err := Run([]string{"-Command", "handoff", "-Target", caseRoot, "-Pack", "_template", "-WhatIf", "-Format", "json"}, &handoffOut); err != nil {
		t.Fatal(err)
	}
	var handoff struct {
		CurrentLoopSegment *currentloop.Inspection `json:"currentLoopSegment"`
	}
	if err := json.Unmarshal(handoffOut.Bytes(), &handoff); err != nil {
		t.Fatalf("terminal drift handoff did not decode: %v\n%s", err, handoffOut.String())
	}
	if handoff.CurrentLoopSegment == nil || handoff.CurrentLoopSegment.Ready || handoff.CurrentLoopSegment.State != "stale-current-driver-request" || handoff.CurrentLoopSegment.Continuation != nil {
		t.Fatalf("new-session handoff recovered terminal budget after durable request drift: %+v", handoff.CurrentLoopSegment)
	}
}

func TestRunCurrentLoopPreservesCompletedReceiptWhenSecondStepFails(t *testing.T) {
	caseRoot := currentLoopCaseWithOpenIntervention(t, "int-current-loop-partial")
	preview := runCurrentLoopPreview(t, caseRoot, 2)
	currentLoopBeforeApplyStepHook = func(step int) error {
		if step == 2 {
			return errors.New("injected second-step failure")
		}
		return nil
	}
	t.Cleanup(func() { currentLoopBeforeApplyStepHook = nil })
	applied := runCurrentLoopApply(t, caseRoot, preview)
	if !applied.Applied || applied.AppliedSteps != 1 || len(applied.Steps) != 1 || applied.StopReason.Code != "error" || applied.StopReason.Phase != "apply-step" || !strings.Contains(applied.StopReason.Message, "step 2") || applied.FinalStatus == nil {
		t.Fatalf("second-step failure lost the completed receipt: %+v", applied)
	}
	if driverStepCommandName(applied.Steps[0].RequestBefore.Command) != "reconcile" || applied.Steps[0].CurrentStepReceipt == nil || applied.FinalStatus.MissionControlRunbook == nil || driverStepCommandName(applied.FinalStatus.MissionControlRunbook.CurrentDriverRequest.Command) != "continue" {
		t.Fatalf("partial loop result did not retain refreshed first-step state: %+v", applied)
	}
}

func TestRunCurrentLoopReportsAppliedStepWhenStatusRefreshFails(t *testing.T) {
	caseRoot := fullAttachedCase(t)
	var out bytes.Buffer
	if err := Run([]string{"-Command", "overview", "-Target", caseRoot, "-Pack", "_template", "-Format", "json"}, &out); err != nil {
		t.Fatal(err)
	}
	preview := runCurrentLoopPreview(t, caseRoot, 2)
	currentStepBeforeStatusRefreshHook = func(command string) error {
		if command != "run-driver-step" {
			t.Fatalf("unexpected nested command: %s", command)
		}
		return errors.New("injected refresh failure")
	}
	t.Cleanup(func() { currentStepBeforeStatusRefreshHook = nil })
	applied := runCurrentLoopApply(t, caseRoot, preview)
	if !applied.Applied || applied.AppliedSteps != 1 || len(applied.Steps) != 1 || applied.Steps[0].CurrentStepReceipt == nil || applied.Steps[0].CurrentStepReceipt.State != "refresh-failed" || applied.StopReason.Code != "error" || applied.StopReason.Phase != "refresh-status" || applied.FinalStatus != nil || applied.SegmentCheckpoint == nil || applied.SegmentCheckpoint.State != "status-unavailable" || applied.SegmentCheckpoint.Ready || applied.SegmentCheckpoint.Continuation != nil {
		t.Fatalf("applied step with refresh failure was not reported truthfully: %+v", applied)
	}
	var statusOut bytes.Buffer
	if err := Run([]string{"-Command", "status", "-Target", caseRoot, "-Pack", "_template", "-Format", "json"}, &statusOut); err != nil {
		t.Fatal(err)
	}
	var fresh statusInventory
	if err := json.Unmarshal(statusOut.Bytes(), &fresh); err != nil {
		t.Fatalf("status-unavailable fresh status did not decode: %v\n%s", err, statusOut.String())
	}
	durable := fresh.MissionControlRunbook.CurrentLoopSegment
	if durable == nil || durable.State != "status-unavailable" || durable.Ready || durable.Continuation != nil || durable.ArtifactPath != applied.SegmentCheckpoint.ArtifactPath {
		t.Fatalf("fresh status recovered status-unavailable budget: %+v", durable)
	}
	var handoffOut bytes.Buffer
	if err := Run([]string{"-Command", "handoff", "-Target", caseRoot, "-Pack", "_template", "-WhatIf", "-Format", "json"}, &handoffOut); err != nil {
		t.Fatal(err)
	}
	var handoff struct {
		CurrentLoopSegment *currentloop.Inspection `json:"currentLoopSegment"`
	}
	if err := json.Unmarshal(handoffOut.Bytes(), &handoff); err != nil {
		t.Fatalf("status-unavailable handoff did not decode: %v\n%s", err, handoffOut.String())
	}
	if handoff.CurrentLoopSegment == nil || handoff.CurrentLoopSegment.State != "status-unavailable" || handoff.CurrentLoopSegment.Ready || handoff.CurrentLoopSegment.Continuation != nil || handoff.CurrentLoopSegment.ArtifactPath != applied.SegmentCheckpoint.ArtifactPath {
		t.Fatalf("fresh handoff recovered status-unavailable budget: %+v", handoff.CurrentLoopSegment)
	}
}

func TestRunCurrentLoopStopsForExternalReviewerHandoffs(t *testing.T) {
	caseRoot := fullAttachedCase(t)
	var out bytes.Buffer
	if err := Run([]string{"-Command", "start", "-Target", caseRoot, "-Pack", "_template", "-Name", "review", "-Apply"}, &out); err != nil {
		t.Fatal(err)
	}
	out.Reset()
	if err := Run([]string{"-Command", "plan-subagents", "-Target", caseRoot, "-Pack", "_template", "-TaskType", "feature-analysis", "-Items", "alpha"}, &out); err != nil {
		t.Fatal(err)
	}
	external := runCurrentLoopPreview(t, caseRoot, 10)
	if external.ExpectedCurrentLoopPlanSHA256 != "" || external.StopReason.Code != "external-reviewer-handoff" || external.StopReason.ExternalHandoff == nil || external.StopReason.ExternalHandoff.RunLoopStepID != "spawn-reviewer" {
		t.Fatalf("current-loop did not stop for reviewer handoff: %+v", external)
	}
	if external.ContinuationRequest == nil || external.ContinuationRequest.Kind != "current-loop-campaign-continuation" || external.ContinuationRequest.StopCode != "external-reviewer-handoff" || external.ContinuationRequest.SegmentRoute != "reviewer" || external.ContinuationRequest.ExpectedRoute != "reviewer" || external.ContinuationRequest.RemainingMaxSteps != 10 || external.ContinuationRequest.AppliedStepsInSegment != 0 || external.ContinuationRequest.WhatIfCommand != external.ResumeCommand || external.ContinuationRequest.CumulativeReceipts || external.ContinuationRequest.ObservationContract == nil || len(external.ContinuationRequest.ObservationContract.Alternatives) != 1 {
		t.Fatalf("spawn handoff omitted typed continuation request: %+v", external.ContinuationRequest)
	}
	spawnObservation := external.ContinuationRequest.ObservationContract.Alternatives[0]
	if spawnObservation.Kind != "reviewer-session-accepted" || !slices.Equal(spawnObservation.RequiredFlags, []string{"-ExpectedCurrentLoopReviewerAttemptSha256", "-ReviewerHarness", "-ReviewerSession", "-Actor"}) || !strings.Contains(spawnObservation.PreviewCommandTemplate, "-ExpectedCurrentLoopReviewerAttemptSha256 ") || spawnObservation.Transition != "refresh-status:save-result-input" {
		t.Fatalf("spawn continuation observation contract = %+v", spawnObservation)
	}
	if strings.Contains(external.ContinuationRequest.WhatIfCommand, "-ExpectedCurrentLoopReviewerAttemptSha256") {
		t.Fatalf("spawn continuation shared command carried alternative-specific attempt guard: %s", external.ContinuationRequest.WhatIfCommand)
	}
	statusOperator := external.FinalStatus.MissionControlRunbook.CurrentLoopOperator
	if statusOperator == nil || !statusOperator.Ready || statusOperator.State != "fresh-loop-review-required" || statusOperator.SelectedDriverRequest == nil || driverStepCommandName(statusOperator.SelectedDriverRequest.Command) != "run-current-loop" || statusOperator.SourceCurrentDriverRequest == nil || driverStepCommandName(statusOperator.SourceCurrentDriverRequest.Command) == "run-current-loop" || statusOperator.ExternalReviewerHandoff == nil || statusOperator.ExternalReviewerHandoff.AgentToolRequest == nil || !statusOperator.ExternalReviewerHandoff.AgentToolRequest.ReadOnly || len(statusOperator.ExternalReviewerHandoff.ObservationContract.Alternatives) != 1 {
		t.Fatalf("status current-loop operator did not package the reviewer spawn closure: %+v", statusOperator)
	}
	wave := statusOperator.ExternalReviewerHandoff.Wave
	if wave == nil || wave.SnapshotSHA256 == "" || wave.MaxParallel != 2 || wave.TotalShards != 1 || wave.ActiveSlots != 0 || wave.AvailableSlots != 2 || len(wave.SpawnWave) != 1 || len(wave.Shards) != 1 || wave.SpawnWave[0].Identity.ShardID != wave.Shards[0].Identity.ShardID || wave.SpawnWave[0].SelectedAction.AgentToolRequest == nil {
		t.Fatalf("status current-loop operator omitted reviewer wave closure: %+v", wave)
	}
	spawnAttempt := statusOperator.ExternalReviewerHandoff.Attempt
	if spawnAttempt == nil || spawnAttempt.SelectedAction.Kind != "invoke-reviewer-and-record-acceptance" || spawnAttempt.SelectedAction.Actor != "main-agent-harness" || spawnAttempt.Identity.PacketID == "" || spawnAttempt.Identity.RouteID == "" || spawnAttempt.Identity.ShardID == "" || spawnAttempt.Identity.Lane == "" || spawnAttempt.Identity.PromptSHA256 == "" || spawnAttempt.Identity.OwnerBindingMode == "" || spawnAttempt.Identity.CurrentExecutor != spawnAttempt.Identity.OwnerExecutor || spawnAttempt.Identity.CurrentGeneration != spawnAttempt.Identity.OwnerGeneration || spawnAttempt.CurrentReviewerDriverRequest == nil || spawnAttempt.DurableContinuationDriverRequest == nil || len(spawnAttempt.AttemptSnapshotSHA256) != 64 {
		t.Fatalf("status current-loop operator omitted reviewer attempt identity: %+v", spawnAttempt)
	}
	operatorSpawn := statusOperator.ExternalReviewerHandoff.ObservationContract.Alternatives[0]
	if operatorSpawn.Kind != "reviewer-session-accepted" || operatorSpawn.Transition != "refresh-status:save-result-input" || !strings.Contains(operatorSpawn.PreviewCommandTemplate, "-ExpectedCurrentLoopReviewerAttemptSha256 "+spawnAttempt.AttemptSnapshotSHA256) || !strings.Contains(operatorSpawn.PreviewCommandTemplate, "-ReviewerHarness <harness>") || !strings.Contains(operatorSpawn.PreviewCommandTemplate, "-ReviewerSession <session-id>") || !strings.Contains(operatorSpawn.PreviewCommandTemplate, "-Actor <main-agent>") {
		t.Fatalf("status current-loop operator spawn template = %+v", operatorSpawn)
	}
	if external.FinalStatus.MissionControlRunbook.Quickstart == nil || external.FinalStatus.MissionControlRunbook.Quickstart.CurrentLoopOperator == nil || external.FinalStatus.MissionControlRunbook.ReplacementExecutorTakeover == nil || external.FinalStatus.MissionControlRunbook.ReplacementExecutorTakeover.CurrentLoopOperator == nil {
		t.Fatalf("current-loop operator was not projected to quickstart and replacement takeover: %+v", external.FinalStatus.MissionControlRunbook)
	}
	var handoffPreviewOut bytes.Buffer
	if err := Run([]string{"-Command", "handoff", "-Target", caseRoot, "-Pack", "_template", "-WhatIf", "-Format", "json"}, &handoffPreviewOut); err != nil {
		t.Fatal(err)
	}
	var handoffPreview workstream.HandoffResult
	if err := json.Unmarshal(handoffPreviewOut.Bytes(), &handoffPreview); err != nil {
		t.Fatalf("current-loop operator handoff preview did not decode: %v\n%s", err, handoffPreviewOut.String())
	}
	if handoffPreview.CurrentLoopOperator == nil || handoffPreview.ReplacementExecutorTakeoverPackage == nil || handoffPreview.ReplacementExecutorTakeoverPackage.CurrentLoopOperator == nil || handoffPreview.CurrentLoopOperator.ExternalReviewerHandoff == nil {
		t.Fatalf("handoff JSON omitted current-loop operator: %+v", handoffPreview)
	}
	var handoffApplyOut bytes.Buffer
	if err := runHashBoundHandoffApply(t, []string{"-Command", "handoff", "-Target", caseRoot, "-Pack", "_template", "-Apply", "-Format", "json"}, &handoffApplyOut); err != nil {
		t.Fatal(err)
	}
	durableHandoff, err := os.ReadFile(filepath.Join(caseRoot, ".rekit", "handovers", "latest.md"))
	if err != nil {
		t.Fatal(err)
	}
	for _, expected := range []string{"## Current-loop operator", "selected driver request:", "reviewer attempt: id=", "snapshotSha256=", "Agent request:", "reviewer-session-accepted", "transition=refresh-status:save-result-input"} {
		if !strings.Contains(string(durableHandoff), expected) {
			t.Fatalf("durable handoff omitted current-loop operator detail %q:\n%s", expected, durableHandoff)
		}
	}
	out.Reset()
	err = Run([]string{"-Command", "run-current-loop", "-Target", caseRoot, "-Pack", "_template", "-MaxSteps", "10", "-ExpectedCurrentLoopPlanSha256", strings.Repeat("0", 64), "-Apply", "-Format", "json"}, &out)
	if err == nil || !strings.Contains(err.Error(), "external or reviewed action") {
		t.Fatalf("external reviewer handoff accepted Apply: %v", err)
	}

	actor := "mission-commander"
	dispatchInputs := []string{"-ExpectedCurrentLoopReviewerAttemptSha256", spawnAttempt.AttemptSnapshotSHA256, "-ReviewerHarness", "go-cli-test-harness", "-ReviewerSession", "reviewer-session-runner", "-Actor", actor}
	staleDispatchInputs := append([]string{}, dispatchInputs...)
	staleDispatchInputs[1] = strings.Repeat("0", 64)
	out.Reset()
	err = Run(append([]string{"-Command", "run-current-loop", "-Target", caseRoot, "-Pack", "_template", "-MaxSteps", "10", "-WhatIf", "-Format", "json"}, staleDispatchInputs...), &out)
	if err == nil || !strings.Contains(err.Error(), "expected reviewer attempt sha256 mismatch") {
		t.Fatalf("stale reviewer attempt observation was not rejected before preview: %v", err)
	}
	if _, statErr := os.Stat(filepath.Join(filepath.Dir(spawnAttempt.Identity.PacketPath), "sessions", spawnAttempt.Identity.ShardID)); !os.IsNotExist(statErr) {
		t.Fatalf("stale reviewer attempt observation wrote a session receipt: %v", statErr)
	}
	dispatchPreview := runCurrentLoopPreviewWith(t, caseRoot, external.ContinuationRequest.RemainingMaxSteps, dispatchInputs...)
	if dispatchPreview.ExpectedCurrentLoopPlanSHA256 == "" || dispatchPreview.InitialRoute != "reviewer" {
		t.Fatalf("reviewer deterministic loop preview missing: %+v", dispatchPreview)
	}
	out.Reset()
	driftInputs := []string{"-ExpectedCurrentLoopReviewerAttemptSha256", spawnAttempt.AttemptSnapshotSHA256, "-ReviewerHarness", "go-cli-test-harness", "-ReviewerSession", "reviewer-session-runner", "-Actor", "replacement-commander"}
	driftArgs := []string{"-Command", "run-current-loop", "-Target", caseRoot, "-Pack", "_template", "-MaxSteps", stringInt(dispatchPreview.MaxSteps)}
	driftArgs = append(driftArgs, driftInputs...)
	driftArgs = append(driftArgs, "-ExpectedCurrentLoopPlanSha256", dispatchPreview.ExpectedCurrentLoopPlanSHA256, "-Apply", "-Format", "json")
	err = Run(driftArgs, &out)
	if err == nil || !strings.Contains(err.Error(), "expected plan sha256 mismatch") {
		t.Fatalf("reviewer actor drift was not rejected: %v", err)
	}

	dispatchApplied := runCurrentLoopApplyWith(t, caseRoot, dispatchPreview, dispatchInputs...)
	if dispatchApplied.AppliedSteps != 1 || dispatchApplied.StopReason.Code != "external-reviewer-handoff" {
		t.Fatalf("reviewer loop did not stop after deterministic dispatch: %+v", dispatchApplied)
	}
	continuation := dispatchApplied.ContinuationRequest
	if continuation == nil || continuation.Kind != "current-loop-campaign-continuation" || continuation.StopCode != "external-reviewer-handoff" || continuation.SegmentRoute != "reviewer" || continuation.ExpectedRoute != "reviewer" || continuation.RemainingMaxSteps != dispatchPreview.MaxSteps-1 || continuation.AppliedStepsInSegment != 1 || continuation.SegmentMaxSteps != dispatchPreview.MaxSteps || continuation.CumulativeReceipts || !strings.Contains(continuation.WhatIfCommand, "-MaxSteps 9") {
		t.Fatalf("dispatch result omitted decremented continuation budget: %+v", continuation)
	}
	if dispatchApplied.StopReason.ExternalHandoff.ReviewerResultDropPathRole != "canonical-reviewer-input-destination" || dispatchApplied.StopReason.ExternalHandoff.ReviewerResultInputPath == "" || dispatchApplied.StopReason.ExternalHandoff.ReviewerResultDropPath != dispatchApplied.StopReason.ExternalHandoff.ReviewerResultInputPath {
		t.Fatalf("result handoff did not distinguish canonical input destination: %+v", dispatchApplied.StopReason.ExternalHandoff)
	}
	activeWaveOperator := dispatchApplied.FinalStatus.MissionControlRunbook.CurrentLoopOperator
	if activeWaveOperator == nil || activeWaveOperator.ExternalReviewerHandoff == nil {
		t.Fatalf("reviewer wave active status was not refreshed: %+v", activeWaveOperator)
	}
	resultWave := activeWaveOperator.ExternalReviewerHandoff.Wave
	if resultWave == nil || resultWave.ActiveSlots != 1 || resultWave.AvailableSlots != 1 || len(resultWave.Active) != 1 || len(resultWave.SpawnWave) != 0 || resultWave.Active[0].Receipt.Session != "reviewer-session-runner" || resultWave.Active[0].SelectedAction.AgentToolRequest != nil {
		t.Fatalf("reviewer wave did not project active slot after acceptance: %+v", resultWave)
	}
	if len(continuation.ObservationContract.Alternatives) != 2 || continuation.ObservationContract.Alternatives[0].Kind != "reviewer-result-returned" || continuation.ObservationContract.Alternatives[1].Kind != "reviewer-session-failed" {
		t.Fatalf("result handoff observation alternatives = %+v", continuation.ObservationContract)
	}
	returnedContinuation := continuation.ObservationContract.Alternatives[0]
	failedContinuation := continuation.ObservationContract.Alternatives[1]
	if strings.Contains(continuation.WhatIfCommand, "-ExpectedCurrentLoopReviewerAttemptSha256") || !strings.Contains(returnedContinuation.PreviewCommandTemplate, "-ExpectedCurrentLoopReviewerAttemptSha256 "+dispatchApplied.StopReason.ExpectedReviewerAttemptSHA256) || !strings.Contains(failedContinuation.PreviewCommandTemplate, "-ExpectedCurrentLoopReviewerAttemptSha256 "+dispatchApplied.StopReason.ExpectedReviewerAttemptSHA256) || failedContinuation.Transition != "refresh-status:spawn-reviewer" {
		t.Fatalf("result continuation did not separate shared command from guarded alternatives: shared=%s returned=%+v failed=%+v", continuation.WhatIfCommand, returnedContinuation, failedContinuation)
	}
	var operatorStatusOut bytes.Buffer
	if err := Run([]string{"-Command", "status", "-Target", caseRoot, "-Pack", "_template", "-Format", "json"}, &operatorStatusOut); err != nil {
		t.Fatal(err)
	}
	var operatorStatus statusInventory
	if err := json.Unmarshal(operatorStatusOut.Bytes(), &operatorStatus); err != nil {
		t.Fatalf("reviewer operator status did not decode: %v\n%s", err, operatorStatusOut.String())
	}
	resultOperator := operatorStatus.MissionControlRunbook.CurrentLoopOperator
	if resultOperator == nil || resultOperator.State != "checkpoint-resume-review-required" || resultOperator.ResumeDriverRequest == nil || resultOperator.ExternalReviewerHandoff == nil || resultOperator.ExternalReviewerHandoff.ReviewerResultDropPathRole != "canonical-reviewer-input-destination" || len(resultOperator.ExternalReviewerHandoff.ObservationContract.Alternatives) != 2 {
		t.Fatalf("status current-loop operator did not package the reviewer result closure: %+v", resultOperator)
	}
	resultAttempt := resultOperator.ExternalReviewerHandoff.Attempt
	if resultAttempt == nil || resultAttempt.AttemptID == spawnAttempt.AttemptID || resultAttempt.AttemptSnapshotSHA256 == spawnAttempt.AttemptSnapshotSHA256 || resultAttempt.RunLoopStepID != "save-result-input" || resultAttempt.SelectedAction.Kind != "observe-reviewer-terminal-state" || resultAttempt.Receipt.DispatchID == "" || resultAttempt.Receipt.Harness != "go-cli-test-harness" || resultAttempt.Receipt.Session != "reviewer-session-runner" || resultAttempt.Receipt.SessionLifecycleState == "" || resultAttempt.DurableContinuationDriverRequest == nil || !strings.Contains(resultAttempt.DurableContinuationDriverRequest.Command, dispatchApplied.SegmentCheckpoint.ArtifactSHA256) {
		t.Fatalf("checkpoint reviewer attempt did not preserve lifecycle identity: spawn=%+v result=%+v", spawnAttempt, resultAttempt)
	}
	if resultAttempt.SelectedAction.ObservationContract.Alternatives[0].Transition != "refresh-status:record-completion" || resultAttempt.SelectedAction.ObservationContract.Alternatives[1].Transition != "refresh-status:spawn-reviewer" {
		t.Fatalf("reviewer attempt transition contract = %+v", resultAttempt.SelectedAction.ObservationContract)
	}
	returnedTemplate := resultOperator.ExternalReviewerHandoff.ObservationContract.Alternatives[0].PreviewCommandTemplate
	failedTemplate := resultOperator.ExternalReviewerHandoff.ObservationContract.Alternatives[1].PreviewCommandTemplate
	if !strings.Contains(returnedTemplate, "-ResumeCurrentLoop") || !strings.Contains(returnedTemplate, dispatchApplied.SegmentCheckpoint.ArtifactSHA256) || !strings.Contains(returnedTemplate, "-ExpectedCurrentLoopReviewerAttemptSha256 "+resultAttempt.AttemptSnapshotSHA256) || !strings.Contains(returnedTemplate, "-ReviewerResultInputSourcePath <reviewer-result-source-path>") || !strings.Contains(failedTemplate, "-ReviewerOutcome failed") || !strings.Contains(failedTemplate, "-ReviewerExitStatus <exit-status>") {
		t.Fatalf("checkpoint-bound reviewer result templates are incomplete: returned=%s failed=%s", returnedTemplate, failedTemplate)
	}
	for _, alternative := range resultOperator.ExternalReviewerHandoff.ObservationContract.Alternatives {
		if !strings.Contains(alternative.ObservationEnvelopeTemplate, `"checkpointSha256": "`+dispatchApplied.SegmentCheckpoint.ArtifactSHA256+`"`) || !strings.Contains(alternative.ObservationEnvelopeTemplate, `"reviewerAttemptSha256": "`+resultAttempt.AttemptSnapshotSHA256+`"`) || !strings.Contains(alternative.ObservationPathCommand, "-CurrentLoopObservationPath") {
			t.Fatalf("checkpoint reviewer alternative omitted envelope intake: %+v", alternative)
		}
	}

	plan := decodePlanSubagentsPacket(t, dispatchApplied.FinalStatus.CaseMission.ReviewerDispatchIntakeSummary.LatestPacketPath)
	handoff := plan.ShardHandoffs[0]
	evidencePath := filepath.Join(caseRoot, "workspace", "features", "feature-login", "review-evidence.md")
	if err := os.MkdirAll(filepath.Dir(evidencePath), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(evidencePath, []byte("bounded current-loop reviewer evidence\n"), 0o644); err != nil {
		t.Fatal(err)
	}

	failedObservationPath := writeCurrentLoopObservation(t, caseRoot, "reviewer-failed", currentLoopObservationEnvelope{
		SchemaVersion: 1, Kind: "current-loop-external-session-observation", CheckpointSHA256: dispatchApplied.SegmentCheckpoint.ArtifactSHA256,
		ObservationKind: "reviewer-session-failed", Actor: actor, ReviewerAttemptSHA256: resultAttempt.AttemptSnapshotSHA256, ReviewerExitStatus: "reviewer-error",
		NoAuthorityOrConfirmed: true, NoHeavyTool: true,
	})
	failedPreview := runCurrentLoopResumePreviewWith(t, caseRoot, dispatchApplied.SegmentCheckpoint.ArtifactSHA256, "-CurrentLoopObservationPath", failedObservationPath)
	if failedPreview.ObservationSHA256 == "" || !strings.Contains(failedPreview.ApplyCommand, "-CurrentLoopObservationPath") || strings.Contains(failedPreview.ApplyCommand, "-ReviewerOutcome") || strings.Contains(failedPreview.ApplyCommand, "-Actor") {
		t.Fatalf("failed reviewer envelope preview did not preserve path-only apply: %+v", failedPreview)
	}
	failedApplied := runCurrentLoopResult(t, rekitCommandCLIArgs(t, failedPreview.ApplyCommand))
	if failedApplied.AppliedSteps != 1 || failedApplied.StopReason.Code != "external-reviewer-handoff" || failedApplied.SegmentCheckpoint == nil {
		t.Fatalf("failed reviewer attempt did not return a replacement handoff: %+v", failedApplied)
	}
	replacementOperator := failedApplied.FinalStatus.MissionControlRunbook.CurrentLoopOperator
	if replacementOperator == nil || replacementOperator.ExternalReviewerHandoff == nil || replacementOperator.ExternalReviewerHandoff.Attempt == nil {
		t.Fatalf("failed reviewer attempt omitted replacement spawn attempt: %+v", replacementOperator)
	}
	replacementSpawnAttempt := replacementOperator.ExternalReviewerHandoff.Attempt
	if replacementSpawnAttempt.RunLoopStepID != "spawn-reviewer" || replacementSpawnAttempt.Receipt.DispatchID != resultAttempt.Receipt.DispatchID || replacementSpawnAttempt.Receipt.CompletionOutcome != "failed" {
		t.Fatalf("failed reviewer attempt did not advance to fresh spawn while preserving prior receipt provenance: %+v", replacementSpawnAttempt)
	}
	replacementDispatchObservationPath := writeCurrentLoopObservation(t, caseRoot, "reviewer-accepted", currentLoopObservationEnvelope{
		SchemaVersion: 1, Kind: "current-loop-external-session-observation", CheckpointSHA256: failedApplied.SegmentCheckpoint.ArtifactSHA256,
		ObservationKind: "reviewer-session-accepted", Actor: actor, ReviewerAttemptSHA256: replacementSpawnAttempt.AttemptSnapshotSHA256, ReviewerHarness: "go-cli-test-harness", ReviewerSession: "reviewer-session-replacement",
		NoAuthorityOrConfirmed: true, NoHeavyTool: true,
	})
	replacementDispatchPreview := runCurrentLoopResumePreviewWith(t, caseRoot, failedApplied.SegmentCheckpoint.ArtifactSHA256, "-CurrentLoopObservationPath", replacementDispatchObservationPath)
	replacementDispatchApplied := runCurrentLoopResult(t, rekitCommandCLIArgs(t, replacementDispatchPreview.ApplyCommand))
	if replacementDispatchApplied.AppliedSteps != 1 || replacementDispatchApplied.StopReason.Code != "external-reviewer-handoff" || replacementDispatchApplied.SegmentCheckpoint == nil {
		t.Fatalf("replacement reviewer dispatch did not reach result handoff: %+v", replacementDispatchApplied)
	}
	replacementResultAttempt := replacementDispatchApplied.FinalStatus.MissionControlRunbook.CurrentLoopOperator.ExternalReviewerHandoff.Attempt
	if replacementResultAttempt == nil || replacementResultAttempt.RunLoopStepID != "save-result-input" || replacementResultAttempt.Receipt.Session != "reviewer-session-replacement" || replacementResultAttempt.Receipt.DispatchID == resultAttempt.Receipt.DispatchID {
		t.Fatalf("replacement result attempt did not bind the fresh reviewer session: prior=%+v replacement=%+v", resultAttempt, replacementResultAttempt)
	}

	staleResultSource := filepath.Join(caseRoot, "workspace", "current-loop-stale-reviewer-result.json")
	if err := os.WriteFile(staleResultSource, reviewerResultForCLIPlan(t, plan, handoff, "accept", "accepted", "reviewer-session-runner"), 0o644); err != nil {
		t.Fatal(err)
	}
	staleResultPreview := runCurrentLoopResumePreviewWith(t, caseRoot, replacementDispatchApplied.SegmentCheckpoint.ArtifactSHA256,
		"-ExpectedCurrentLoopReviewerAttemptSha256", replacementResultAttempt.AttemptSnapshotSHA256,
		"-ReviewerResultInputSourcePath", staleResultSource,
		"-Actor", actor,
	)
	if staleResultPreview.ExpectedCurrentLoopPlanSHA256 != "" || staleResultPreview.StopReason.Code != "requires-review" || !strings.Contains(staleResultPreview.StopReason.Message, "current reviewer dispatch") {
		t.Fatalf("late failed-session reviewer result was not rejected before input save: %+v", staleResultPreview)
	}
	if _, err := os.Stat(replacementResultAttempt.ReviewerResultInputPath); !os.IsNotExist(err) {
		t.Fatalf("late failed-session reviewer result occupied canonical input: %v", err)
	}

	resultSource := filepath.Join(caseRoot, "workspace", "current-loop-reviewer-result.json")
	if err := os.WriteFile(resultSource, reviewerResultForCLIPlan(t, plan, handoff, "accept", "accepted", "reviewer-session-replacement"), 0o644); err != nil {
		t.Fatal(err)
	}
	resultObservationPath := writeCurrentLoopObservation(t, caseRoot, "reviewer-returned", currentLoopObservationEnvelope{
		SchemaVersion: 1, Kind: "current-loop-external-session-observation", CheckpointSHA256: replacementDispatchApplied.SegmentCheckpoint.ArtifactSHA256,
		ObservationKind: "reviewer-result-returned", Actor: actor, ReviewerAttemptSHA256: replacementResultAttempt.AttemptSnapshotSHA256, ReviewerResultSourcePath: resultSource,
		NoAuthorityOrConfirmed: true, NoHeavyTool: true,
	})
	resultPreview := runCurrentLoopResumePreviewWith(t, caseRoot, replacementDispatchApplied.SegmentCheckpoint.ArtifactSHA256, "-CurrentLoopObservationPath", resultObservationPath)
	if resultPreview.ExpectedCurrentLoopPlanSHA256 == "" || resultPreview.InitialRoute != "reviewer" || resultPreview.MaxSteps != replacementDispatchPreview.MaxSteps-1 || resultPreview.ResumeSource == nil || resultPreview.ApplyCommand == "" || !strings.Contains(resultPreview.ApplyCommand, "-CurrentLoopObservationPath") || strings.Contains(resultPreview.ApplyCommand, "-ReviewerResultInputSourcePath") || strings.Contains(resultPreview.ApplyCommand, "-Actor") {
		t.Fatalf("typed durable replacement result envelope did not rebuild a path-only apply command: %+v", resultPreview)
	}
	resultApplied := runCurrentLoopResult(t, rekitCommandCLIArgs(t, resultPreview.ApplyCommand))
	if !resultApplied.Applied || resultApplied.AppliedSteps != 6 || resultApplied.StopReason.Code != "route-policy" || resultApplied.FinalStatus == nil || resultApplied.FinalStatus.MissionControlRunbook == nil || resultApplied.FinalStatus.MissionControlRunbook.Scope != "case" {
		var finalScope, finalStep string
		if resultApplied.FinalStatus != nil && resultApplied.FinalStatus.MissionControlRunbook != nil {
			finalScope = resultApplied.FinalStatus.MissionControlRunbook.Scope
			finalStep = resultApplied.FinalStatus.MissionControlRunbook.CurrentRunLoopStepID
		}
		t.Fatalf("reviewer result continuation did not finish deterministic pipeline: applied=%t steps=%d stop=%+v finalScope=%s finalStep=%s", resultApplied.Applied, resultApplied.AppliedSteps, resultApplied.StopReason, finalScope, finalStep)
	}
	expectedSteps := []string{"save-result-input", "record-completion", "source-capture", "stage-candidate", "collect-result", "intake-results"}
	campaign := resultApplied.ContinuationRequest
	if campaign == nil || campaign.StopCode != "route-policy" || campaign.SegmentRoute != "reviewer" || campaign.ExpectedRoute != "case" || campaign.RemainingMaxSteps != resultPreview.MaxSteps-len(expectedSteps) || campaign.ObservationContract != nil || !strings.Contains(campaign.WhatIfCommand, "-MaxSteps "+stringInt(campaign.RemainingMaxSteps)) || len(resultApplied.Steps) != len(expectedSteps) {
		t.Fatalf("fresh cross-route campaign continuation or segment receipts are invalid: %+v", resultApplied)
	}
	if resultApplied.SegmentCheckpoint == nil || !resultApplied.SegmentCheckpoint.Ready || resultApplied.SegmentCheckpoint.Continuation == nil || resultApplied.SegmentCheckpoint.SegmentRoute != "reviewer" || resultApplied.SegmentCheckpoint.ExpectedRoute != "case" || resultApplied.SegmentCheckpoint.RemainingMaxSteps != campaign.RemainingMaxSteps {
		t.Fatalf("reviewer-to-case segment checkpoint is invalid: %+v", resultApplied.SegmentCheckpoint)
	}
	var campaignStatusOut bytes.Buffer
	if err := Run([]string{"-Command", "status", "-Target", caseRoot, "-Pack", "_template", "-Format", "json"}, &campaignStatusOut); err != nil {
		t.Fatal(err)
	}
	var campaignStatus statusInventory
	if err := json.Unmarshal(campaignStatusOut.Bytes(), &campaignStatus); err != nil {
		t.Fatalf("campaign status did not decode: %v\n%s", err, campaignStatusOut.String())
	}
	durableCampaign := campaignStatus.MissionControlRunbook.CurrentLoopSegment
	if durableCampaign == nil || !durableCampaign.Ready || durableCampaign.ArtifactPath != resultApplied.SegmentCheckpoint.ArtifactPath || durableCampaign.Continuation == nil || durableCampaign.Continuation.ExpectedRoute != "case" || durableCampaign.Continuation.RemainingMaxSteps != campaign.RemainingMaxSteps {
		t.Fatalf("new-session status did not recover latest reviewer-to-case segment: %+v", durableCampaign)
	}
	for index, expected := range expectedSteps {
		if resultApplied.Steps[index].Step != index+1 || resultApplied.Steps[index].RunLoopStepID != expected {
			t.Fatalf("reviewer continuation step %d = %+v, want %s", index+1, resultApplied.Steps[index], expected)
		}
	}
	casePreview := runCurrentLoopResumePreview(t, caseRoot, durableCampaign.ArtifactSHA256)
	if casePreview.InitialRoute != "case" || casePreview.InitialLane != campaign.ExpectedLane || casePreview.MaxSteps != campaign.RemainingMaxSteps || casePreview.ExpectedCurrentLoopPlanSHA256 == "" || casePreview.ResumeSource == nil || casePreview.ResumeSource.ArtifactSHA256 != durableCampaign.ArtifactSHA256 || casePreview.ExpectedResumeCheckpointSHA256 != durableCampaign.ArtifactSHA256 || casePreview.ApplyCommand == "" || len(casePreview.Steps) != 0 || casePreview.ContinuationRequest != nil {
		t.Fatalf("durable campaign operator did not rebuild a checkpoint-bound fresh case segment: %+v", casePreview)
	}
	out.Reset()
	err = Run([]string{"-Command", "run-current-loop", "-Target", caseRoot, "-Pack", "_template", "-MaxSteps", stringInt(campaign.RemainingMaxSteps), "-ExpectedCurrentLoopPlanSha256", resultPreview.ExpectedCurrentLoopPlanSHA256, "-Apply", "-Format", "json"}, &out)
	if err == nil || !strings.Contains(err.Error(), "expected plan sha256 mismatch") {
		t.Fatalf("previous reviewer segment hash crossed the route boundary: %v", err)
	}
	verifications, err := os.ReadFile(filepath.Join(caseRoot, ".rekit", "facts", "verifications.jsonl"))
	if err != nil || !strings.Contains(string(verifications), plan.PacketID) {
		t.Fatalf("reviewer continuation omitted verification writeback: %v\n%s", err, verifications)
	}
	decisions, err := os.ReadFile(filepath.Join(caseRoot, ".rekit", "facts", "decisions.jsonl"))
	if err != nil || !strings.Contains(string(decisions), plan.PacketID) {
		t.Fatalf("reviewer continuation omitted decision writeback: %v\n%s", err, decisions)
	}
	for _, forbidden := range []string{"authority.jsonl", "confirmed.jsonl"} {
		if _, err := os.Stat(filepath.Join(caseRoot, ".rekit", "facts", forbidden)); !os.IsNotExist(err) {
			t.Fatalf("reviewer continuation created forbidden ledger %s: %v", forbidden, err)
		}
	}
}

func TestCurrentLoopStopsBeforeNewHumanIntervention(t *testing.T) {
	request := missionDriverRequestForTest(`/rekit reconcile main -InterventionId int-main-1 -WhatIf -Format json`)
	request.Lane = "main"
	status := statusInventory{
		MissionControlRunbook: &statusMissionControlRunbook{
			Scope:                "case",
			CurrentDriverRequest: &request,
		},
	}
	_, route, stop := currentLoopStatusCandidate(status)
	if stop.Code != "" || route != "case" {
		t.Fatalf("initial reconcile should remain previewable: route=%s stop=%+v", route, stop)
	}
	if stop := currentLoopBeforeStepPolicyStop(1, route, &request); stop.Code != "" {
		t.Fatalf("initial reconcile was incorrectly stopped: %+v", stop)
	}
	stop = currentLoopBeforeStepPolicyStop(2, route, &request)
	if stop.Code != "human-intervention" || stop.Phase != "before-step" || stop.CurrentDriverRequest != &request {
		t.Fatalf("refreshed Human-in-the-Lane reconcile was not stopped: %+v", stop)
	}
}

func TestCurrentLoopRouteDriftReturnsTypedCampaignContinuation(t *testing.T) {
	request := missionDriverRequestForTest(`/rekit continue verifier -WhatIf -Format json`)
	request.Lane = "feature-verifier"
	stop := currentLoopStopReason{
		Code:                 "route-policy",
		Phase:                "before-step",
		Message:              "refreshed route or lane changed; review a fresh loop preview",
		CurrentDriverRequest: &request,
	}
	continuation := currentLoopContinuationFor(runtime.Context{Target: `C:\cases\campaign`, Pack: "_template"}, 8, 3, "case", "feature-triage", "case", "", stop)
	if continuation == nil || continuation.StopCode != "route-policy" || continuation.SegmentRoute != "case" || continuation.SegmentLane != "feature-triage" || continuation.ExpectedRoute != "case" || continuation.ExpectedLane != "feature-verifier" || continuation.RemainingMaxSteps != 5 || continuation.ObservationContract != nil || continuation.CumulativeReceipts {
		t.Fatalf("lane drift continuation = %+v", continuation)
	}
	if !strings.Contains(continuation.WhatIfCommand, "-MaxSteps 5") || !continuation.FreshPreviewRequired {
		t.Fatalf("lane drift continuation command = %+v", continuation)
	}
	if got := currentLoopContinuationFor(runtime.Context{}, 3, 3, "case", "main", "reviewer", "", stop); got != nil {
		t.Fatalf("exhausted segment created continuation: %+v", got)
	}
	forgedExternal := stop
	forgedExternal.Code = "external-reviewer-handoff"
	if got := currentLoopContinuationFor(runtime.Context{}, 8, 3, "case", "main", "reviewer", "", forgedExternal); got != nil {
		t.Fatalf("external stop without a typed handoff created continuation: %+v", got)
	}
}

func TestRunCurrentLoopStopsBeforeReconcileSurfacedByFirstStepRefresh(t *testing.T) {
	caseRoot := fullAttachedCase(t)
	var out bytes.Buffer
	if err := Run([]string{"-Command", "overview", "-Target", caseRoot, "-Pack", "_template", "-Format", "json"}, &out); err != nil {
		t.Fatal(err)
	}
	preview := runCurrentLoopPreview(t, caseRoot, 2)
	driverStepAfterPreviewValidationHook = func() error {
		appendCurrentLoopOpenIntervention(t, caseRoot, "int-current-loop-refresh")
		return nil
	}
	t.Cleanup(func() { driverStepAfterPreviewValidationHook = nil })
	applied := runCurrentLoopApply(t, caseRoot, preview)
	if applied.AppliedSteps != 1 || len(applied.Steps) != 1 || applied.StopReason.Code != "human-intervention" || applied.StopReason.Phase != "before-step" || applied.FinalStatus == nil || applied.FinalStatus.MissionControlRunbook == nil {
		t.Fatalf("refreshed intervention was not stopped before reconcile: %+v", applied)
	}
	if applied.SegmentCheckpoint == nil || !applied.SegmentCheckpoint.Ready || applied.SegmentCheckpoint.State != "ready" || applied.SegmentCheckpoint.Continuation == nil || applied.SegmentCheckpoint.Continuation.RemainingMaxSteps != 1 || applied.SegmentCheckpoint.ArtifactPath == "" {
		t.Fatalf("applied Human-in-the-Lane segment omitted durable checkpoint: %+v", applied.SegmentCheckpoint)
	}
	continuation := applied.ContinuationRequest
	if continuation == nil || continuation.StopCode != "human-intervention" || continuation.State != "awaiting-fresh-segment-review" || continuation.SegmentRoute != "case" || continuation.ExpectedRoute != "case" || continuation.RemainingMaxSteps != 1 || continuation.AppliedStepsInSegment != 1 || continuation.ObservationContract != nil || !strings.Contains(continuation.WhatIfCommand, "-MaxSteps 1") {
		t.Fatalf("refreshed intervention omitted fresh review continuation: %+v", continuation)
	}
	request := applied.FinalStatus.MissionControlRunbook.CurrentDriverRequest
	if request == nil || driverStepCommandName(request.Command) != "reconcile" || !strings.Contains(request.Command, "int-current-loop-refresh") {
		t.Fatalf("final status did not retain the fresh reconcile request: %+v", request)
	}
	interventions, err := os.ReadFile(filepath.Join(caseRoot, ".rekit", "facts", "interventions.jsonl"))
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(string(interventions), `"resolvesEventId":"int-current-loop-refresh"`) {
		t.Fatalf("old loop authorization resolved the fresh intervention: %s", interventions)
	}
	var statusOut bytes.Buffer
	if err := Run([]string{"-Command", "status", "-Target", caseRoot, "-Pack", "_template", "-Format", "json"}, &statusOut); err != nil {
		t.Fatal(err)
	}
	var newSessionStatus statusInventory
	if err := json.Unmarshal(statusOut.Bytes(), &newSessionStatus); err != nil {
		t.Fatalf("new-session status did not decode: %v\n%s", err, statusOut.String())
	}
	durable := newSessionStatus.MissionControlRunbook.CurrentLoopSegment
	if durable == nil || !durable.Ready || durable.Continuation == nil || durable.Continuation.RemainingMaxSteps != continuation.RemainingMaxSteps || durable.Continuation.WhatIfCommand != "" || durable.LegacyUnboundWhatIfCommand != continuation.WhatIfCommand || durable.ArtifactPath != applied.SegmentCheckpoint.ArtifactPath || durable.ResumeDriverRequest == nil || durable.ResumeDriverRequest.RunLoopStepID != "resume-current-loop" || !durable.ResumeDriverRequest.CommandExecutable || !strings.Contains(durable.ResumeDriverRequest.Command, "-ResumeCurrentLoop") || !strings.Contains(durable.ResumeDriverRequest.Command, durable.ArtifactSHA256) {
		t.Fatalf("new-session status did not recover exact campaign checkpoint and resume request: %+v", durable)
	}
	var handoffOut bytes.Buffer
	if err := Run([]string{"-Command", "handoff", "-Target", caseRoot, "-Pack", "_template", "-WhatIf", "-Format", "json"}, &handoffOut); err != nil {
		t.Fatal(err)
	}
	var handoff struct {
		CurrentLoopSegment *currentloop.Inspection `json:"currentLoopSegment"`
	}
	if err := json.Unmarshal(handoffOut.Bytes(), &handoff); err != nil {
		t.Fatalf("handoff did not decode: %v\n%s", err, handoffOut.String())
	}
	if handoff.CurrentLoopSegment == nil || !handoff.CurrentLoopSegment.Ready || handoff.CurrentLoopSegment.ArtifactPath != durable.ArtifactPath || handoff.CurrentLoopSegment.Continuation == nil || handoff.CurrentLoopSegment.ResumeDriverRequest == nil {
		t.Fatalf("handoff did not project strict campaign checkpoint: %+v", handoff.CurrentLoopSegment)
	}
	var statusText bytes.Buffer
	if err := Run([]string{"-Command", "status", "-Target", caseRoot, "-Pack", "_template", "-Format", "text"}, &statusText); err != nil {
		t.Fatal(err)
	}
	for _, expected := range []string{"current-loop resume source：artifactSha256=" + durable.ArtifactSHA256, "current-loop checkpoint-bound resume driver request", "-ResumeCurrentLoop", "legacyUnboundCommand="} {
		if !strings.Contains(statusText.String(), expected) {
			t.Fatalf("status text omitted checkpoint-bound resume detail %q:\n%s", expected, statusText.String())
		}
	}
	var handoffText bytes.Buffer
	if err := Run([]string{"-Command", "handoff", "-Target", caseRoot, "-Pack", "_template", "-WhatIf", "-Format", "text"}, &handoffText); err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(handoffText.String(), "current-loop checkpoint-bound resume driver request") || !strings.Contains(handoffText.String(), durable.ArtifactSHA256) {
		t.Fatalf("handoff text omitted checkpoint-bound resume detail:\n%s", handoffText.String())
	}
	artifactPath := filepath.Join(caseRoot, filepath.FromSlash(durable.ArtifactPath))
	if err := os.WriteFile(artifactPath, []byte(`{"tampered":true}`), 0o600); err != nil {
		t.Fatal(err)
	}
	statusOut.Reset()
	if err := Run([]string{"-Command", "status", "-Target", caseRoot, "-Pack", "_template", "-Format", "json"}, &statusOut); err != nil {
		t.Fatal(err)
	}
	var tamperedStatus statusInventory
	if err := json.Unmarshal(statusOut.Bytes(), &tamperedStatus); err != nil {
		t.Fatalf("tampered status did not decode: %v\n%s", err, statusOut.String())
	}
	tampered := tamperedStatus.MissionControlRunbook.CurrentLoopSegment
	if tampered == nil || tampered.Ready || tampered.State != "invalid" || tampered.Continuation != nil || tampered.ArtifactPath != durable.ArtifactPath {
		t.Fatalf("tampered latest segment did not fail closed: %+v", tampered)
	}
	tamperedOperator := tamperedStatus.MissionControlRunbook.CurrentLoopOperator
	if tamperedOperator == nil || tamperedOperator.Ready || tamperedOperator.State != "checkpoint-invalid" || tamperedOperator.SelectedDriverRequest != nil || tamperedOperator.StartDriverRequest != nil || tamperedOperator.ResumeDriverRequest != nil || tamperedOperator.ExternalReviewerHandoff != nil {
		t.Fatalf("tampered checkpoint exposed an executable current-loop operator: %+v", tamperedOperator)
	}
	if previewErr := runCurrentLoopError(caseRoot, []string{"-ResumeCurrentLoop", "-ExpectedCurrentLoopCheckpointSha256", durable.ArtifactSHA256, "-WhatIf"}); previewErr == nil || !strings.Contains(previewErr.Error(), "state=ready") {
		t.Fatalf("tampered checkpoint resume did not fail closed: %v", previewErr)
	}
}

func TestRunCurrentLoopDurableResumeDriverRequestProductPath(t *testing.T) {
	caseRoot := fullAttachedCase(t)
	var out bytes.Buffer
	if err := Run([]string{"-Command", "overview", "-Target", caseRoot, "-Pack", "_template", "-Format", "json"}, &out); err != nil {
		t.Fatal(err)
	}
	initial := runCurrentLoopPreview(t, caseRoot, 3)
	driverStepAfterPreviewValidationHook = func() error {
		appendCurrentLoopOpenIntervention(t, caseRoot, "int-current-loop-resume")
		return nil
	}
	applied := runCurrentLoopApply(t, caseRoot, initial)
	driverStepAfterPreviewValidationHook = nil
	if applied.SegmentCheckpoint == nil || !applied.SegmentCheckpoint.Ready || applied.SegmentCheckpoint.ResumeDriverRequest == nil || applied.SegmentCheckpoint.RemainingMaxSteps != 2 {
		t.Fatalf("initial segment did not expose a durable resume request: %+v", applied.SegmentCheckpoint)
	}
	previewArgs, err := splitDriverCommand(applied.SegmentCheckpoint.ResumeDriverRequest.Command)
	if err != nil {
		t.Fatal(err)
	}
	resumedPreview := runCurrentLoopResult(t, append([]string{"-Command", previewArgs[1]}, previewArgs[2:]...))
	if resumedPreview.MaxSteps != 2 || resumedPreview.ResumeSource == nil || resumedPreview.ResumeSource.ArtifactSHA256 != applied.SegmentCheckpoint.ArtifactSHA256 || resumedPreview.ExpectedResumeCheckpointSHA256 != applied.SegmentCheckpoint.ArtifactSHA256 || driverStepCommandName(resumedPreview.InitialCurrentStep.CurrentDriverRequest.Command) != "reconcile" || resumedPreview.ApplyCommand == "" {
		t.Fatalf("resume driver request did not produce the exact fresh reconcile preview: %+v", resumedPreview)
	}
	applyArgs, err := splitDriverCommand(resumedPreview.ApplyCommand)
	if err != nil {
		t.Fatal(err)
	}
	resumedApplied := runCurrentLoopResult(t, append([]string{"-Command", applyArgs[1]}, applyArgs[2:]...))
	if !resumedApplied.Applied || resumedApplied.AppliedSteps < 1 || resumedApplied.SegmentCheckpoint == nil || resumedApplied.SegmentCheckpoint.Sequence != applied.SegmentCheckpoint.Sequence+1 || resumedApplied.SegmentCheckpoint.ResumeSourceSHA256 != applied.SegmentCheckpoint.ArtifactSHA256 {
		t.Fatalf("checkpoint-bound resume did not publish linked successor segment: %+v", resumedApplied)
	}
	if err := Run(append([]string{"-Command", applyArgs[1]}, applyArgs[2:]...), &bytes.Buffer{}); err == nil || (!strings.Contains(err.Error(), "latest checkpoint to be state=ready") && !strings.Contains(err.Error(), "consumed")) {
		t.Fatalf("stale resume apply was not rejected after successor publication: %v", err)
	}
	latest := currentloop.Inspect(repoRoot(t), caseRoot, "_template", resumedApplied.FinalStatus.MissionControlRunbook.CurrentDriverRequest)
	if latest.ArtifactSHA256 != resumedApplied.SegmentCheckpoint.ArtifactSHA256 || latest.Sequence != resumedApplied.SegmentCheckpoint.Sequence {
		t.Fatalf("stale resume apply mutated the checkpoint chain: %+v", latest)
	}
}

func TestRunCurrentLoopResumePublishesRetryCheckpointAfterZeroWriteFailure(t *testing.T) {
	caseRoot := fullAttachedCase(t)
	var out bytes.Buffer
	if err := Run([]string{"-Command", "overview", "-Target", caseRoot, "-Pack", "_template", "-Format", "json"}, &out); err != nil {
		t.Fatal(err)
	}
	initial := runCurrentLoopPreview(t, caseRoot, 3)
	driverStepAfterPreviewValidationHook = func() error {
		appendCurrentLoopOpenIntervention(t, caseRoot, "int-current-loop-retry")
		return nil
	}
	first := runCurrentLoopApply(t, caseRoot, initial)
	driverStepAfterPreviewValidationHook = nil
	if first.SegmentCheckpoint == nil || first.SegmentCheckpoint.ResumeDriverRequest == nil {
		t.Fatalf("initial segment did not expose resume request: %+v", first.SegmentCheckpoint)
	}
	previewArgs, err := splitDriverCommand(first.SegmentCheckpoint.ResumeDriverRequest.Command)
	if err != nil {
		t.Fatal(err)
	}
	preview := runCurrentLoopResult(t, append([]string{"-Command", previewArgs[1]}, previewArgs[2:]...))
	applyArgs, err := splitDriverCommand(preview.ApplyCommand)
	if err != nil {
		t.Fatal(err)
	}
	driverStepApplyBeforeMutationHook = func(string) error { return os.ErrPermission }
	t.Cleanup(func() {
		driverStepApplyBeforeMutationHook = nil
		driverStepAfterPreviewValidationHook = nil
	})
	failed := runCurrentLoopResult(t, append([]string{"-Command", applyArgs[1]}, applyArgs[2:]...))
	if failed.Applied || failed.AppliedSteps != 0 || failed.StopReason.Code != "zero-progress-retry" || failed.SegmentCheckpoint == nil || !strings.Contains(failed.StopReason.Message, "permission denied") {
		t.Fatalf("resume pre-mutation failure did not return a retry checkpoint: %+v", failed)
	}
	if !failed.SegmentCheckpoint.Ready || failed.SegmentCheckpoint.State != "ready" || failed.SegmentCheckpoint.ResumeSourceSHA256 != first.SegmentCheckpoint.ArtifactSHA256 || failed.SegmentCheckpoint.RemainingMaxSteps != first.SegmentCheckpoint.RemainingMaxSteps || failed.SegmentCheckpoint.ResumeDriverRequest == nil {
		t.Fatalf("resume zero-write retry checkpoint did not preserve the remaining budget: %+v", failed.SegmentCheckpoint)
	}
	if failed.SegmentCheckpoint.Sequence != first.SegmentCheckpoint.Sequence+1 {
		t.Fatalf("resume zero-write retry checkpoint did not append to the chain: first=%+v retry=%+v", first.SegmentCheckpoint, failed.SegmentCheckpoint)
	}
	if err := Run(append([]string{"-Command", applyArgs[1]}, applyArgs[2:]...), &bytes.Buffer{}); err == nil || (!strings.Contains(err.Error(), "expected checkpoint sha256 mismatch") && !strings.Contains(err.Error(), "latest checkpoint")) {
		t.Fatalf("original claimed resume Apply remained executable after retry checkpoint publication: %v", err)
	}
	var statusOut bytes.Buffer
	if err := Run([]string{"-Command", "status", "-Target", caseRoot, "-Pack", "_template", "-Format", "json"}, &statusOut); err != nil {
		t.Fatal(err)
	}
	var retryStatus statusInventory
	if err := json.Unmarshal(statusOut.Bytes(), &retryStatus); err != nil {
		t.Fatalf("retry operator status did not decode: %v\n%s", err, statusOut.String())
	}
	retryOperator := retryStatus.MissionControlRunbook.CurrentLoopOperator
	if retryOperator == nil || !retryOperator.Ready || retryOperator.State != "checkpoint-resume-review-required" || retryOperator.SelectedDriverRequest == nil || retryOperator.ResumeDriverRequest == nil || retryOperator.RemainingMaxSteps != first.SegmentCheckpoint.RemainingMaxSteps {
		t.Fatalf("replacement status did not expose the retry checkpoint: %+v", retryOperator)
	}
	driverStepApplyBeforeMutationHook = nil
	retryPreviewArgs, err := splitDriverCommand(retryOperator.SelectedDriverRequest.Command)
	if err != nil {
		t.Fatal(err)
	}
	retryPreview := runCurrentLoopResult(t, append([]string{"-Command", retryPreviewArgs[1]}, retryPreviewArgs[2:]...))
	retryApplyArgs, err := splitDriverCommand(retryPreview.ApplyCommand)
	if err != nil {
		t.Fatal(err)
	}
	retried := runCurrentLoopResult(t, append([]string{"-Command", retryApplyArgs[1]}, retryApplyArgs[2:]...))
	if !retried.Applied || retried.AppliedSteps < 1 || retried.SegmentCheckpoint == nil || retried.SegmentCheckpoint.ResumeSourceSHA256 != failed.SegmentCheckpoint.ArtifactSHA256 {
		t.Fatalf("replacement session did not continue from the zero-write retry checkpoint: %+v", retried)
	}
}

func TestRunCurrentLoopResumeDoesNotRecoverZeroWriteFailureAfterRequestDrift(t *testing.T) {
	caseRoot := fullAttachedCase(t)
	var out bytes.Buffer
	if err := Run([]string{"-Command", "overview", "-Target", caseRoot, "-Pack", "_template", "-Format", "json"}, &out); err != nil {
		t.Fatal(err)
	}
	initial := runCurrentLoopPreview(t, caseRoot, 3)
	driverStepAfterPreviewValidationHook = func() error {
		appendCurrentLoopOpenIntervention(t, caseRoot, "int-current-loop-drift")
		return nil
	}
	first := runCurrentLoopApply(t, caseRoot, initial)
	driverStepAfterPreviewValidationHook = nil
	previewArgs, err := splitDriverCommand(first.SegmentCheckpoint.ResumeDriverRequest.Command)
	if err != nil {
		t.Fatal(err)
	}
	preview := runCurrentLoopResult(t, append([]string{"-Command", previewArgs[1]}, previewArgs[2:]...))
	applyArgs, err := splitDriverCommand(preview.ApplyCommand)
	if err != nil {
		t.Fatal(err)
	}
	driverStepApplyBeforeMutationHook = func(string) error {
		appendCurrentLoopOpenIntervention(t, caseRoot, "int-current-loop-drift-after-claim")
		return os.ErrPermission
	}
	t.Cleanup(func() {
		driverStepApplyBeforeMutationHook = nil
		driverStepAfterPreviewValidationHook = nil
	})
	failed := runCurrentLoopResult(t, append([]string{"-Command", applyArgs[1]}, applyArgs[2:]...))
	if failed.Applied || failed.AppliedSteps != 0 || failed.StopReason.Code != "error" || failed.SegmentCheckpoint != nil || !strings.Contains(strings.Join(failed.Boundary, " "), "claim remains consumed") {
		t.Fatalf("zero-write failure recovered budget after durable request drift: %+v", failed)
	}
	inspection := currentloop.Inspect(repoRoot(t), caseRoot, "_template", failed.FinalStatus.MissionControlRunbook.CurrentDriverRequest)
	if inspection.Ready || inspection.State != "consumed" || inspection.ArtifactSHA256 != first.SegmentCheckpoint.ArtifactSHA256 || inspection.ResumeDriverRequest != nil {
		t.Fatalf("request drift after claim exposed a retry checkpoint: %+v", inspection)
	}
}

func TestRunCurrentLoopResumeDoesNotRecoverAppliedMutationFailure(t *testing.T) {
	caseRoot := fullAttachedCase(t)
	var out bytes.Buffer
	if err := Run([]string{"-Command", "overview", "-Target", caseRoot, "-Pack", "_template", "-Format", "json"}, &out); err != nil {
		t.Fatal(err)
	}
	initial := runCurrentLoopPreview(t, caseRoot, 3)
	driverStepAfterPreviewValidationHook = func() error {
		appendCurrentLoopOpenIntervention(t, caseRoot, "int-current-loop-partial-resume")
		return nil
	}
	first := runCurrentLoopApply(t, caseRoot, initial)
	driverStepAfterPreviewValidationHook = nil
	if first.SegmentCheckpoint == nil || first.SegmentCheckpoint.ResumeDriverRequest == nil {
		t.Fatalf("initial segment did not expose resume request: %+v", first.SegmentCheckpoint)
	}
	previewArgs, err := splitDriverCommand(first.SegmentCheckpoint.ResumeDriverRequest.Command)
	if err != nil {
		t.Fatal(err)
	}
	preview := runCurrentLoopResult(t, append([]string{"-Command", previewArgs[1]}, previewArgs[2:]...))
	applyArgs, err := splitDriverCommand(preview.ApplyCommand)
	if err != nil {
		t.Fatal(err)
	}
	currentStepBeforeStatusRefreshHook = func(string) error { return os.ErrPermission }
	t.Cleanup(func() {
		currentStepBeforeStatusRefreshHook = nil
		driverStepAfterPreviewValidationHook = nil
	})
	failed := runCurrentLoopResult(t, append([]string{"-Command", applyArgs[1]}, applyArgs[2:]...))
	if !failed.Applied || failed.AppliedSteps != 1 || failed.StopReason.Code != "error" || failed.StopReason.Phase != "refresh-status" || failed.SegmentCheckpoint == nil || failed.SegmentCheckpoint.State != "status-unavailable" || failed.SegmentCheckpoint.Ready || failed.SegmentCheckpoint.Continuation != nil || failed.SegmentCheckpoint.ResumeSourceSHA256 != first.SegmentCheckpoint.ArtifactSHA256 {
		t.Fatalf("resume applied mutation failure recovered consumed budget: %+v", failed)
	}
	var statusOut bytes.Buffer
	if err := Run([]string{"-Command", "status", "-Target", caseRoot, "-Pack", "_template", "-Format", "json"}, &statusOut); err != nil {
		t.Fatal(err)
	}
	var fresh statusInventory
	if err := json.Unmarshal(statusOut.Bytes(), &fresh); err != nil {
		t.Fatal(err)
	}
	operator := fresh.MissionControlRunbook.CurrentLoopOperator
	if operator == nil || !operator.Ready || operator.State != "fresh-loop-review-required" || operator.StartDriverRequest == nil || operator.SelectedDriverRequest == nil || operator.ResumeDriverRequest != nil || operator.RemainingMaxSteps != 0 {
		t.Fatalf("replacement status recovered prior budget instead of requiring a fresh campaign: %+v", operator)
	}
}

func TestRunCurrentLoopReturnsTypedApplyError(t *testing.T) {
	caseRoot := fullAttachedCase(t)
	var out bytes.Buffer
	if err := Run([]string{"-Command", "overview", "-Target", caseRoot, "-Pack", "_template", "-Format", "json"}, &out); err != nil {
		t.Fatal(err)
	}
	preview := runCurrentLoopPreview(t, caseRoot, 2)
	currentLoopBeforeApplyStepHook = func(step int) error {
		return os.ErrPermission
	}
	t.Cleanup(func() {
		currentLoopBeforeApplyStepHook = nil
	})
	applied := runCurrentLoopApply(t, caseRoot, preview)
	if applied.Applied || applied.AppliedSteps != 0 || len(applied.Steps) != 0 || applied.StopReason.Code != "error" || applied.StopReason.Phase != "apply-step" || !strings.Contains(applied.StopReason.Message, "permission denied") || applied.FinalStatus == nil {
		t.Fatalf("typed current-loop apply error was not returned: %+v", applied)
	}
}

func TestRunCurrentLoopValidatesOuterContract(t *testing.T) {
	caseRoot := fullAttachedCase(t)
	cases := []struct {
		args []string
		want string
	}{
		{args: []string{"-Command", "run-current-loop", "-Target", caseRoot, "-Pack", "_template", "-WhatIf", "-Format", "json"}, want: "requires -MaxSteps"},
		{args: []string{"-Command", "run-current-loop", "-Target", caseRoot, "-Pack", "_template", "-MaxSteps", "21", "-WhatIf", "-Format", "json"}, want: "between 1 and 20"},
		{args: []string{"-Command", "run-current-loop", "-Target", caseRoot, "-Pack", "_template", "-ResumeCurrentLoop", "-MaxSteps", "2", "-WhatIf", "-Format", "json"}, want: "does not accept -MaxSteps"},
		{args: []string{"-Command", "run-current-loop", "-Target", caseRoot, "-Pack", "_template", "-ResumeCurrentLoop", "-WhatIf", "-Format", "json"}, want: "requires -ExpectedCurrentLoopCheckpointSha256"},
		{args: []string{"-Command", "run-current-loop", "-Target", caseRoot, "-Pack", "_template", "-MaxSteps", "2", "-Lane", "unexpected", "-WhatIf", "-Format", "json"}, want: "unsupported flag"},
	}
	for _, tc := range cases {
		var out bytes.Buffer
		err := Run(tc.args, &out)
		if err == nil || !strings.Contains(err.Error(), tc.want) {
			t.Fatalf("current-loop args error = %v, want %q", err, tc.want)
		}
	}
}

func currentLoopCaseWithOpenIntervention(t *testing.T, eventID string) string {
	t.Helper()
	caseRoot := fullAttachedCase(t)
	var out bytes.Buffer
	if err := Run([]string{"-Command", "overview", "-Target", caseRoot, "-Pack", "_template", "-Format", "json"}, &out); err != nil {
		t.Fatal(err)
	}
	appendCurrentLoopOpenIntervention(t, caseRoot, eventID)
	return caseRoot
}

func appendCurrentLoopOpenIntervention(t *testing.T, caseRoot, eventID string) {
	t.Helper()
	path := filepath.Join(caseRoot, ".rekit", "facts", "interventions.jsonl")
	file, err := os.OpenFile(path, os.O_APPEND|os.O_WRONLY, 0o644)
	if err != nil {
		t.Fatal(err)
	}
	entry := `{"kind":"intervention","eventId":"` + eventID + `","lane":"main","subject":"current loop correction","summary":"operator redirected the current loop","action":"override","target":"current-loop","approvedBy":"lead","scope":"metadata","status":"open","batchId":"batch-current-loop"}` + "\n"
	if _, err := file.WriteString(entry); err != nil {
		_ = file.Close()
		t.Fatal(err)
	}
	if err := file.Close(); err != nil {
		t.Fatal(err)
	}
}

type currentLoopTestPlan struct {
	MaxSteps                       int                             `json:"maxSteps"`
	InitialRoute                   string                          `json:"initialRoute"`
	InitialLane                    string                          `json:"initialLane"`
	Applied                        bool                            `json:"applied"`
	AppliedSteps                   int                             `json:"appliedSteps"`
	InitialCurrentStep             *currentStepPlan                `json:"initialCurrentStep"`
	ExpectedCurrentLoopPlanSHA256  string                          `json:"expectedCurrentLoopPlanSha256"`
	Steps                          []currentLoopStepReceipt        `json:"steps"`
	StopReason                     currentLoopStopReason           `json:"stopReason"`
	ResumeCommand                  string                          `json:"resumeCommand"`
	ContinuationRequest            *currentLoopContinuationRequest `json:"continuationRequest"`
	ResumeSource                   *currentloop.Inspection         `json:"resumeSource"`
	ExpectedResumeCheckpointSHA256 string                          `json:"expectedResumeCheckpointSha256"`
	ObservationPath                string                          `json:"observationPath"`
	ObservationSHA256              string                          `json:"observationSha256"`
	ObservationKind                string                          `json:"observationKind"`
	ObservationActor               string                          `json:"observationActor"`
	ObservationReceipt             *currentLoopObservationReceipt  `json:"observationReceipt"`
	ApplyCommand                   string                          `json:"applyCommand"`
	SegmentCheckpoint              *currentloop.Inspection         `json:"segmentCheckpoint"`
	FinalStatus                    *statusInventory                `json:"finalStatus"`
	Boundary                       []string                        `json:"boundary"`
}

func runCurrentLoopPreview(t *testing.T, caseRoot string, maxSteps int) currentLoopTestPlan {
	t.Helper()
	return runCurrentLoopPreviewWith(t, caseRoot, maxSteps)
}

func runCurrentLoopPreviewWith(t *testing.T, caseRoot string, maxSteps int, inputs ...string) currentLoopTestPlan {
	t.Helper()
	args := []string{"-Command", "run-current-loop", "-Target", caseRoot, "-Pack", "_template", "-MaxSteps", stringInt(maxSteps)}
	args = append(args, inputs...)
	args = append(args, "-WhatIf", "-Format", "json")
	return runCurrentLoopResult(t, args)
}

func runCurrentLoopResumePreview(t *testing.T, caseRoot, checkpointSHA256 string) currentLoopTestPlan {
	t.Helper()
	return runCurrentLoopResumePreviewWith(t, caseRoot, checkpointSHA256)
}

func runCurrentLoopResumePreviewWith(t *testing.T, caseRoot, checkpointSHA256 string, inputs ...string) currentLoopTestPlan {
	t.Helper()
	args := []string{"-Command", "run-current-loop", "-Target", caseRoot, "-Pack", "_template", "-ResumeCurrentLoop", "-ExpectedCurrentLoopCheckpointSha256", checkpointSHA256}
	args = append(args, inputs...)
	args = append(args, "-WhatIf", "-Format", "json")
	return runCurrentLoopResult(t, args)
}

func writeCurrentLoopObservation(t *testing.T, caseRoot, name string, envelope currentLoopObservationEnvelope) string {
	t.Helper()
	dir := filepath.Join(caseRoot, ".rekit", "external-session-observations")
	if err := os.MkdirAll(dir, 0o700); err != nil {
		t.Fatal(err)
	}
	path := filepath.Join(dir, name+".json")
	data, err := json.Marshal(envelope)
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, append(data, '\n'), 0o600); err != nil {
		t.Fatal(err)
	}
	return path
}

func TestCurrentLoopObservationEnvelopeRejectsInvalidFilesAndFlags(t *testing.T) {
	caseRoot := fullAttachedCase(t)
	checkpointSHA256 := strings.Repeat("a", 64)
	valid := currentLoopObservationEnvelope{
		SchemaVersion: 1, Kind: "current-loop-external-session-observation", CheckpointSHA256: checkpointSHA256,
		ObservationKind: "member-session-accepted", Actor: "harness", ObservedAt: "2026-08-03T03:01:00Z", MemberAttemptID: "g000001-a000001-0123456789abcdef",
		NoAuthorityOrConfirmed: true, NoHeavyTool: true,
	}
	validData, err := json.Marshal(valid)
	if err != nil {
		t.Fatal(err)
	}
	inside := writeCurrentLoopObservation(t, caseRoot, "strict-valid", valid)
	outside := filepath.Join(t.TempDir(), "outside.json")
	if err := os.WriteFile(outside, append(validData, '\n'), 0o600); err != nil {
		t.Fatal(err)
	}
	unknown := filepath.Join(caseRoot, ".rekit", "external-session-observations", "unknown.json")
	unknownData := append(append([]byte{}, validData[:len(validData)-1]...), []byte(",\"unknown\":true}\n")...)
	if err := os.WriteFile(unknown, unknownData, 0o600); err != nil {
		t.Fatal(err)
	}
	trailing := filepath.Join(caseRoot, ".rekit", "external-session-observations", "trailing.json")
	if err := os.WriteFile(trailing, append(append([]byte{}, validData...), []byte("\n{}\n")...), 0o600); err != nil {
		t.Fatal(err)
	}
	oversize := filepath.Join(caseRoot, ".rekit", "external-session-observations", "oversize.json")
	if err := os.WriteFile(oversize, bytes.Repeat([]byte{'x'}, maxCurrentLoopObservationBytes+1), 0o600); err != nil {
		t.Fatal(err)
	}
	for _, tc := range []struct {
		path string
		want string
	}{
		{outside, "escapes anchored case root"},
		{unknown, "unknown field"},
		{trailing, "exactly one JSON object"},
		{oversize, "bounded non-empty regular file"},
	} {
		if _, err := readCurrentLoopObservationEnvelope(caseRoot, tc.path); err == nil || !strings.Contains(err.Error(), tc.want) {
			t.Fatalf("observation file %s error = %v, want %q", tc.path, err, tc.want)
		}
	}
	base := []string{"-ResumeCurrentLoop", "-ExpectedCurrentLoopCheckpointSha256", checkpointSHA256, "-CurrentLoopObservationPath", inside}
	for _, tc := range []struct {
		extra []string
		want  string
	}{
		{[]string{"-ExpectedCurrentLoopObservationSha256", strings.Repeat("b", 64), "-WhatIf"}, "does not accept -ExpectedCurrentLoopObservationSha256"},
		{[]string{"-Apply", "-ExpectedCurrentLoopPlanSha256", strings.Repeat("c", 64)}, "requires -ExpectedCurrentLoopObservationSha256"},
		{[]string{"-Actor", "legacy", "-WhatIf"}, "cannot be combined"},
		{[]string{"-MemberExecutionAttemptId", valid.MemberAttemptID, "-MemberExecutionOutcome", "accepted", "-MemberExecutionObservedAt", valid.ObservedAt, "-WhatIf"}, "cannot be combined"},
	} {
		if err := runCurrentLoopError(caseRoot, append(append([]string{}, base...), tc.extra...)); err == nil || !strings.Contains(err.Error(), tc.want) {
			t.Fatalf("observation flag contract error = %v, want %q", err, tc.want)
		}
	}
}

func TestCurrentLoopObservationInboxFailsClosedOnAmbiguityAndInvalidEntry(t *testing.T) {
	caseRoot := fullAttachedCase(t)
	inspection := currentloop.Inspection{
		ArtifactSHA256: strings.Repeat("a", 64),
		ExpectedLane:   "main",
		Continuation: &currentloop.Continuation{
			ObservationContract:   &currentloop.ObservationContract{Alternatives: []currentloop.ObservationAlternative{{Kind: "member-session-accepted"}}},
			ExternalMemberHandoff: &mission.CurrentLoopExternalMemberHandoff{AttemptID: "g000001-a000001-deadbeefdeadbeef"},
		},
	}
	inboxDir := filepath.Join(caseRoot, filepath.FromSlash(currentLoopObservationInboxRel))
	if err := os.MkdirAll(inboxDir, 0o700); err != nil {
		t.Fatal(err)
	}
	valid := currentLoopObservationEnvelope{
		SchemaVersion: 1, Kind: "current-loop-external-session-observation", CheckpointSHA256: inspection.ArtifactSHA256,
		ObservationKind: "member-session-accepted", Actor: "harness", ObservedAt: "2026-08-04T00:00:00Z", MemberAttemptID: inspection.Continuation.ExternalMemberHandoff.AttemptID,
		NoAuthorityOrConfirmed: true, NoHeavyTool: true,
	}
	for _, name := range []string{"one.json", "two.json"} {
		data, err := json.Marshal(valid)
		if err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(filepath.Join(inboxDir, name), append(data, '\n'), 0o600); err != nil {
			t.Fatal(err)
		}
	}
	ambiguous := inspectCurrentLoopObservationInbox(caseRoot, inspection)
	if ambiguous.State != "ambiguous" || ambiguous.MatchingCount != 2 || ambiguous.SelectedCandidate != nil || ambiguous.SelectedDriverRequest != nil {
		t.Fatalf("ambiguous inbox was not blocked: %+v", ambiguous)
	}
	if err := os.Remove(filepath.Join(inboxDir, "two.json")); err != nil {
		t.Fatal(err)
	}
	bad := valid
	bad.Actor = ""
	badData, err := json.Marshal(bad)
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(inboxDir, "bad.json"), append(badData, '\n'), 0o600); err != nil {
		t.Fatal(err)
	}
	invalid := inspectCurrentLoopObservationInbox(caseRoot, inspection)
	if invalid.State != "invalid" || invalid.InvalidCount != 1 || invalid.SelectedCandidate != nil || invalid.SelectedDriverRequest != nil {
		t.Fatalf("invalid inbox was not blocked: %+v", invalid)
	}
	if err := os.Remove(filepath.Join(inboxDir, "bad.json")); err != nil {
		t.Fatal(err)
	}
	wrongAttempt := valid
	wrongAttempt.MemberAttemptID = "g000001-a000002-deadbeefdeadbeef"
	wrongAttemptData, err := json.Marshal(wrongAttempt)
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(inboxDir, "wrong-attempt.json"), append(wrongAttemptData, '\n'), 0o600); err != nil {
		t.Fatal(err)
	}
	attemptMismatch := inspectCurrentLoopObservationInbox(caseRoot, inspection)
	if attemptMismatch.State != "invalid" || attemptMismatch.InvalidCount != 1 || attemptMismatch.SelectedCandidate != nil || attemptMismatch.SelectedDriverRequest != nil {
		t.Fatalf("attempt-mismatched inbox observation was not blocked: %+v", attemptMismatch)
	}
	if err := os.Remove(filepath.Join(inboxDir, "one.json")); err != nil {
		t.Fatal(err)
	}
	staleOnly := inspectCurrentLoopObservationInbox(caseRoot, inspection)
	if staleOnly.State != "invalid" || staleOnly.InvalidCount != 1 || staleOnly.MatchingCount != 0 || staleOnly.SelectedCandidate != nil {
		t.Fatalf("attempt-mismatched observation was not invalid without another candidate: %+v", staleOnly)
	}
	if err := os.Remove(filepath.Join(inboxDir, "wrong-attempt.json")); err != nil {
		t.Fatal(err)
	}
	reviewerAttempt := strings.Repeat("b", 64)
	inspection.Continuation.ExternalMemberHandoff = nil
	inspection.Continuation.ObservationContract = &currentloop.ObservationContract{Alternatives: []currentloop.ObservationAlternative{{
		Kind:                   "reviewer-session-failed",
		PreviewCommandTemplate: "/rekit run-current-loop -ExpectedCurrentLoopReviewerAttemptSha256 " + reviewerAttempt + " -WhatIf -Format json",
	}}}
	wrongReviewer := currentLoopObservationEnvelope{
		SchemaVersion: 1, Kind: "current-loop-external-session-observation", CheckpointSHA256: inspection.ArtifactSHA256,
		ObservationKind: "reviewer-session-failed", Actor: "harness", ReviewerAttemptSHA256: strings.Repeat("c", 64), ReviewerExitStatus: "failed",
		NoAuthorityOrConfirmed: true, NoHeavyTool: true,
	}
	wrongReviewerData, err := json.Marshal(wrongReviewer)
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(inboxDir, "wrong-reviewer.json"), append(wrongReviewerData, '\n'), 0o600); err != nil {
		t.Fatal(err)
	}
	reviewerMismatch := inspectCurrentLoopObservationInbox(caseRoot, inspection)
	if reviewerMismatch.State != "invalid" || reviewerMismatch.InvalidCount != 1 || reviewerMismatch.SelectedCandidate != nil {
		t.Fatalf("reviewer-attempt-mismatched observation was not blocked: %+v", reviewerMismatch)
	}
	if err := os.Remove(filepath.Join(inboxDir, "wrong-reviewer.json")); err != nil {
		t.Fatal(err)
	}
	staleCheckpoint := valid
	staleCheckpoint.CheckpointSHA256 = strings.Repeat("d", 64)
	staleCheckpointData, err := json.Marshal(staleCheckpoint)
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(inboxDir, "stale-checkpoint.json"), append(staleCheckpointData, '\n'), 0o600); err != nil {
		t.Fatal(err)
	}
	stale := inspectCurrentLoopObservationInbox(caseRoot, inspection)
	if stale.State != "empty" || stale.StaleCount != 1 || stale.MatchingCount != 0 || stale.SelectedCandidate != nil || stale.SelectedDriverRequest != nil {
		t.Fatalf("stale checkpoint observation was selected instead of counted: %+v", stale)
	}
}

func TestRunCurrentLoopMemberExecutionCheckpoint(t *testing.T) {
	caseRoot := fullAttachedCase(t)
	var out bytes.Buffer
	if err := Run([]string{"-Command", "overview", "-Target", caseRoot, "-Pack", "_template", "-Format", "json"}, &out); err != nil {
		t.Fatal(err)
	}
	board, err := mission.ReadBoard(caseRoot)
	if err != nil {
		t.Fatal(err)
	}
	board.Lanes[0].CurrentExecutor = "loop-member"
	board.Lanes[0].ExecutorGeneration = 1
	board.Lanes[0].UpdatedAt = "2026-08-03T03:00:00Z"
	boardData, _ := json.MarshalIndent(board, "", "  ")
	if err := os.WriteFile(filepath.Join(caseRoot, ".rekit", "board.json"), append(boardData, '\n'), 0o644); err != nil {
		t.Fatal(err)
	}
	preview := runCurrentLoopPreview(t, caseRoot, 5)
	if preview.InitialCurrentStep == nil || preview.InitialCurrentStep.MemberExecution == nil || preview.ExpectedCurrentLoopPlanSHA256 == "" {
		t.Fatalf("member loop did not bind external handoff: %+v", preview)
	}
	memberPlan := preview.InitialCurrentStep.MemberExecution
	applied := runCurrentLoopApplyWith(t, caseRoot, preview, "-ExpectedMemberExecutionPlanSha256", memberPlan.ExpectedPlanSHA256)
	if !applied.Applied || applied.AppliedSteps != 1 || applied.SegmentCheckpoint == nil || applied.Steps[0].CurrentStepReceipt.Outcome != "current-step-applied" || !strings.Contains(strings.Join(applied.Steps[0].CurrentStepReceipt.Boundary, "\n"), "member execution outcome: handoff-ready") {
		t.Fatalf("member loop did not checkpoint dispatch receipt: %+v", applied)
	}
	if applied.StopReason.Code != "external-member-handoff" || applied.StopReason.ExternalMemberHandoff == nil {
		t.Fatalf("unexpected member loop stop: %+v", applied.StopReason)
	}
	memberHandoff := applied.StopReason.ExternalMemberHandoff
	if memberHandoff.AttemptID != memberPlan.AttemptID || memberHandoff.Lane != memberPlan.Owner.Lane || memberHandoff.Executor != memberPlan.Owner.Executor || memberHandoff.ExecutorGeneration != memberPlan.Owner.ExecutorGeneration || memberHandoff.HandoffPath == "" || memberHandoff.ManifestPath != memberPlan.ExternalHandoff.ManifestPath || memberHandoff.OutputsRoot != memberPlan.ExternalHandoff.OutputsRoot || len(memberHandoff.ObservationContract.Alternatives) != 3 {
		t.Fatalf("member loop stop omitted durable external handoff identity: %+v", memberHandoff)
	}
	for _, alternative := range memberHandoff.ObservationContract.Alternatives {
		if !strings.Contains(alternative.PreviewCommandTemplate, "-MemberExecutionAttemptId "+memberPlan.AttemptID) || !strings.Contains(alternative.PreviewCommandTemplate, "-MemberExecutionOutcome ") {
			t.Fatalf("member observation alternative is not attempt bound: %+v", alternative)
		}
	}
	if !applied.SegmentCheckpoint.Ready || applied.SegmentCheckpoint.State != "ready" || applied.SegmentCheckpoint.RemainingMaxSteps != 4 || applied.SegmentCheckpoint.Continuation == nil || applied.SegmentCheckpoint.Continuation.ExternalMemberHandoff == nil || len(applied.SegmentCheckpoint.Continuation.ObservationContract.Alternatives) != 3 {
		t.Fatalf("member loop checkpoint is not resumable with durable observation handoff: %+v", applied.SegmentCheckpoint)
	}
	var statusOut bytes.Buffer
	if err := Run([]string{"-Command", "status", "-Target", caseRoot, "-Pack", "_template", "-Format", "json"}, &statusOut); err != nil {
		t.Fatal(err)
	}
	var durableStatus statusInventory
	if err := json.Unmarshal(statusOut.Bytes(), &durableStatus); err != nil {
		t.Fatal(err)
	}
	operator := durableStatus.MissionControlRunbook.CurrentLoopOperator
	if operator == nil || operator.ExternalMemberHandoff == nil || operator.SelectedDriverRequest == nil || operator.SelectedDriverRequest.Command == "" {
		t.Fatalf("status omitted durable member current-loop operator handoff: %+v", operator)
	}
	for _, alternative := range operator.ExternalMemberHandoff.ObservationContract.Alternatives {
		if !strings.Contains(alternative.PreviewCommandTemplate, "-ResumeCurrentLoop") || !strings.Contains(alternative.PreviewCommandTemplate, applied.SegmentCheckpoint.ArtifactSHA256) || !strings.Contains(alternative.PreviewCommandTemplate, memberPlan.AttemptID) {
			t.Fatalf("status member observation alternative is not checkpoint/attempt bound: %+v", alternative)
		}
		if !strings.Contains(alternative.ObservationEnvelopeTemplate, `"checkpointSha256": "`+applied.SegmentCheckpoint.ArtifactSHA256+`"`) || !strings.Contains(alternative.ObservationEnvelopeTemplate, `"memberAttemptId": "`+memberPlan.AttemptID+`"`) || !strings.Contains(alternative.ObservationPathCommand, "-CurrentLoopObservationPath") || !strings.Contains(alternative.ObservationPathCommand, applied.SegmentCheckpoint.ArtifactSHA256) {
			t.Fatalf("status member observation alternative omitted envelope intake: %+v", alternative)
		}
	}
	var statusText bytes.Buffer
	if err := Run([]string{"-Command", "status", "-Target", caseRoot, "-Pack", "_template", "-Format", "text"}, &statusText); err != nil {
		t.Fatal(err)
	}
	for _, want := range []string{"current-loop member handoff：state=handoff-ready attempt=" + memberPlan.AttemptID, "owner=loop-member/1", "current-loop member observation：kind=member-session-accepted", "observationPathCommand=`/rekit run-current-loop", "observationEnvelopeTemplate=`{", applied.SegmentCheckpoint.ArtifactSHA256} {
		if !strings.Contains(statusText.String(), want) {
			t.Fatalf("text status omitted member handoff detail %q:\n%s", want, statusText.String())
		}
	}
	if err := runCurrentLoopError(caseRoot, []string{"-ResumeCurrentLoop", "-ExpectedCurrentLoopCheckpointSha256", applied.SegmentCheckpoint.ArtifactSHA256, "-MemberExecutionAttemptId", "g000001-a999999-deadbeefdeadbeef", "-MemberExecutionOutcome", "accepted", "-MemberExecutionObservedAt", "2026-08-03T03:00:30Z", "-Actor", "harness", "-WhatIf"}); err == nil || !strings.Contains(err.Error(), "does not match checkpoint attempt") {
		t.Fatalf("external member checkpoint accepted a different attempt observation: %v", err)
	}
	if err := runCurrentLoopError(caseRoot, []string{"-ResumeCurrentLoop", "-ExpectedCurrentLoopCheckpointSha256", applied.SegmentCheckpoint.ArtifactSHA256, "-WhatIf"}); err == nil || !strings.Contains(err.Error(), "member observation") {
		t.Fatalf("external member checkpoint resumed without a member observation: %v", err)
	}
	observationDir := filepath.Join(caseRoot, ".rekit", "external-session-observations", "inbox")
	if err := os.MkdirAll(observationDir, 0o700); err != nil {
		t.Fatal(err)
	}
	observationPath := filepath.Join(observationDir, "member-accepted.json")
	observationBytes, err := json.Marshal(currentLoopObservationEnvelope{
		SchemaVersion: 1, Kind: "current-loop-external-session-observation", CheckpointSHA256: applied.SegmentCheckpoint.ArtifactSHA256,
		ObservationKind: "member-session-accepted", Actor: "harness", ObservedAt: "2026-08-03T03:01:00Z", MemberAttemptID: memberPlan.AttemptID,
		NoAuthorityOrConfirmed: true, NoHeavyTool: true,
	})
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(observationPath, append(observationBytes, '\n'), 0o600); err != nil {
		t.Fatal(err)
	}
	var inboxStatusOut bytes.Buffer
	if err := Run([]string{"-Command", "status", "-Target", caseRoot, "-Pack", "_template", "-Format", "json"}, &inboxStatusOut); err != nil {
		t.Fatal(err)
	}
	var inboxStatus statusInventory
	if err := json.Unmarshal(inboxStatusOut.Bytes(), &inboxStatus); err != nil {
		t.Fatal(err)
	}
	inboxOperator := inboxStatus.MissionControlRunbook.CurrentLoopOperator
	if inboxOperator == nil || inboxOperator.ObservationInbox == nil || inboxOperator.ObservationInbox.State != "ready" || inboxOperator.SelectedDriverRequest == nil || inboxOperator.SelectedDriverRequest.Command != inboxOperator.ObservationInbox.SelectedDriverRequest.Command || !strings.Contains(inboxOperator.SelectedDriverRequest.Command, observationPath) {
		t.Fatalf("status did not select the unique canonical inbox observation: %+v", inboxOperator)
	}
	for _, alternative := range inboxOperator.ExternalMemberHandoff.ObservationContract.Alternatives {
		if strings.Contains(alternative.PreviewCommandTemplate, "-CurrentLoopObservationPath") {
			t.Fatalf("inbox request leaked into legacy member observation template: %+v", alternative)
		}
		fields, err := splitDriverCommand(alternative.PreviewCommandTemplate)
		if err != nil || len(fields) < 2 || fields[0] != "/rekit" {
			t.Fatalf("legacy member observation template is not consumable: %q: %v", alternative.PreviewCommandTemplate, err)
		}
	}
	accepted := runCurrentLoopResult(t, rekitCommandCLIArgs(t, inboxOperator.SelectedDriverRequest.Command))
	if accepted.MaxSteps != 4 || accepted.ResumeSource == nil || accepted.InitialCurrentStep == nil || accepted.InitialCurrentStep.MemberExecution == nil || accepted.InitialCurrentStep.MemberExecution.Outcome != "accepted" {
		t.Fatalf("member observation did not resume exact checkpoint budget: %+v", accepted)
	}
	observationSHA256 := sha256Text(append(observationBytes, '\n'))
	if accepted.ObservationPath != observationPath || accepted.ObservationSHA256 != observationSHA256 {
		t.Fatalf("member observation preview omitted exact envelope identity: %+v", accepted)
	}
	for _, required := range []string{"-CurrentLoopObservationPath \"" + observationPath + "\"", "-ExpectedCurrentLoopObservationSha256 \"" + observationSHA256 + "\"", "-ExpectedMemberExecutionPlanSha256 \"" + accepted.InitialCurrentStep.MemberExecution.ExpectedPlanSHA256 + "\""} {
		if !strings.Contains(accepted.ApplyCommand, required) {
			t.Fatalf("member resume apply command omitted %q: %s", required, accepted.ApplyCommand)
		}
	}
	for _, forbidden := range []string{"-MemberExecutionAttemptId", "-MemberExecutionOutcome", "-MemberExecutionObservedAt", "-Actor"} {
		if strings.Contains(accepted.ApplyCommand, forbidden) {
			t.Fatalf("member envelope apply command leaked reconstructed flag %s: %s", forbidden, accepted.ApplyCommand)
		}
	}
	driftedBytes := append(append([]byte{}, observationBytes...), []byte(" \n")...)
	if err := os.WriteFile(observationPath, driftedBytes, 0o600); err != nil {
		t.Fatal(err)
	}
	if err := Run(rekitCommandCLIArgs(t, accepted.ApplyCommand), &bytes.Buffer{}); err == nil || !strings.Contains(err.Error(), "observation sha256 mismatch") {
		t.Fatalf("member observation byte drift was not rejected before claim: %v", err)
	}
	if err := os.WriteFile(observationPath, append(observationBytes, '\n'), 0o600); err != nil {
		t.Fatal(err)
	}
	secondPath := filepath.Join(observationDir, "member-accepted-duplicate.json")
	if err := os.WriteFile(secondPath, append(observationBytes, '\n'), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := Run(rekitCommandCLIArgs(t, accepted.ApplyCommand), &bytes.Buffer{}); err == nil || !strings.Contains(err.Error(), "no longer has exactly one") {
		t.Fatalf("member inbox ambiguity introduced after preview was not rejected before claim: %v", err)
	}
	if err := os.Remove(secondPath); err != nil {
		t.Fatal(err)
	}
	currentLoopBeforeApplyStepHook = func(step int) error {
		if step != 1 {
			return nil
		}
		return os.WriteFile(observationPath, append(append([]byte{}, observationBytes...), []byte(" \n")...), 0o600)
	}
	t.Cleanup(func() { currentLoopBeforeApplyStepHook = nil })
	acceptedApplied := runCurrentLoopResult(t, rekitCommandCLIArgs(t, accepted.ApplyCommand))
	currentLoopBeforeApplyStepHook = nil
	if !acceptedApplied.Applied || acceptedApplied.InitialCurrentStep == nil || acceptedApplied.InitialCurrentStep.MemberExecution == nil || acceptedApplied.InitialCurrentStep.MemberExecution.Inspection.State != "accepted" {
		t.Fatalf("member accepted resume apply command did not persist observation: %+v", acceptedApplied)
	}
	if acceptedApplied.SegmentCheckpoint == nil || acceptedApplied.SegmentCheckpoint.ArtifactSHA256 == "" {
		t.Fatalf("member accepted resume did not publish a checkpoint: checkpoint=%+v plan=%+v", acceptedApplied.SegmentCheckpoint, acceptedApplied)
	}
	if acceptedApplied.ObservationReceipt == nil || acceptedApplied.ObservationReceipt.State != "processed" || acceptedApplied.ObservationReceipt.SourceCheckpointSHA256 != applied.SegmentCheckpoint.ArtifactSHA256 || acceptedApplied.ObservationReceipt.SuccessorCheckpointSHA256 != acceptedApplied.SegmentCheckpoint.ArtifactSHA256 || acceptedApplied.ObservationReceipt.ObservationSHA256 != observationSHA256 || acceptedApplied.ObservationReceipt.ObservationKind != "member-session-accepted" {
		t.Fatalf("member accepted resume omitted one-shot processed receipt: %+v", acceptedApplied.ObservationReceipt)
	}
	if acceptedApplied.ContinuationRequest == nil || acceptedApplied.ContinuationRequest.ExternalMemberHandoff == nil || acceptedApplied.ContinuationRequest.ExternalMemberHandoff.State != "accepted" || len(acceptedApplied.ContinuationRequest.ObservationContract.Alternatives) != 2 {
		t.Fatalf("accepted member continuation did not narrow to returned/failed: %+v", acceptedApplied.ContinuationRequest)
	}
	for _, alternative := range acceptedApplied.ContinuationRequest.ObservationContract.Alternatives {
		if alternative.Kind == "member-session-accepted" {
			t.Fatalf("accepted member continuation still permits duplicate accepted: %+v", acceptedApplied.ContinuationRequest)
		}
	}
	if err := os.Remove(observationPath); err != nil {
		t.Fatal(err)
	}
	var receiptStatusOut bytes.Buffer
	if err := Run([]string{"-Command", "status", "-Target", caseRoot, "-Pack", "_template", "-Format", "json"}, &receiptStatusOut); err != nil {
		t.Fatal(err)
	}
	var receiptStatus statusInventory
	if err := json.Unmarshal(receiptStatusOut.Bytes(), &receiptStatus); err != nil {
		t.Fatal(err)
	}
	receiptInbox := receiptStatus.MissionControlRunbook.CurrentLoopOperator.ObservationInbox
	if receiptInbox == nil || receiptInbox.LatestReceipt == nil || receiptInbox.LatestReceipt.SourceCheckpointSHA256 != applied.SegmentCheckpoint.ArtifactSHA256 || receiptInbox.LatestReceipt.SuccessorCheckpointSHA256 != acceptedApplied.SegmentCheckpoint.ArtifactSHA256 || receiptInbox.LatestReceipt.ObservationSHA256 != observationSHA256 || receiptInbox.LatestReceipt.ObservationKind != "member-session-accepted" || receiptInbox.LatestReceipt.Actor != "harness" {
		t.Fatalf("fresh status did not recover processed observation receipt: %+v", receiptInbox)
	}
	if receiptStatus.MissionControlRunbook.CurrentLoopOperator.ObservationReceipt == nil || receiptStatus.MissionControlRunbook.CurrentLoopOperator.ObservationReceipt.ObservationKind != "member-session-accepted" {
		t.Fatalf("fresh status omitted top-level processed observation receipt: %+v", receiptStatus.MissionControlRunbook.CurrentLoopOperator)
	}
	inspection := acceptedApplied.InitialCurrentStep.MemberExecution.Inspection
	output := []byte("member-result\n")
	if err := os.MkdirAll(inspection.OutputsRoot, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(inspection.OutputsRoot, "result.txt"), output, 0o600); err != nil {
		t.Fatal(err)
	}
	manifest := memberexecution.ResultManifest{SchemaVersion: 1, Kind: memberexecution.KindManifest, AttemptID: memberPlan.AttemptID, Owner: memberPlan.Owner, Summary: "returned through resume", Outputs: []memberexecution.Output{{Path: "result.txt", SHA256: sha256Text(output), Bytes: int64(len(output))}}, NoAuthority: true, NoConfirmed: true, NoHeavyTool: true}
	manifestData, _ := memberexecution.MarshalResultManifest(manifest)
	if err := os.WriteFile(inspection.ManifestPath, manifestData, 0o600); err != nil {
		t.Fatal(err)
	}
	returnedSource := acceptedApplied.SegmentCheckpoint
	returnedObservationPath := filepath.Join(observationDir, "member-returned.json")
	returnedObservationBytes, err := json.Marshal(currentLoopObservationEnvelope{
		SchemaVersion: 1, Kind: "current-loop-external-session-observation", CheckpointSHA256: returnedSource.ArtifactSHA256,
		ObservationKind: "member-session-returned", Actor: "harness", ObservedAt: "2026-08-03T03:02:00Z", MemberAttemptID: memberPlan.AttemptID, Reason: "bounded result complete",
		NoAuthorityOrConfirmed: true, NoHeavyTool: true,
	})
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(returnedObservationPath, append(returnedObservationBytes, '\n'), 0o600); err != nil {
		t.Fatal(err)
	}
	var returnedInboxStatusOut bytes.Buffer
	if err := Run([]string{"-Command", "status", "-Target", caseRoot, "-Pack", "_template", "-Format", "json"}, &returnedInboxStatusOut); err != nil {
		t.Fatal(err)
	}
	var returnedInboxStatus statusInventory
	if err := json.Unmarshal(returnedInboxStatusOut.Bytes(), &returnedInboxStatus); err != nil {
		t.Fatal(err)
	}
	returnedRequest := returnedInboxStatus.MissionControlRunbook.CurrentLoopOperator.SelectedDriverRequest
	if returnedRequest == nil || !strings.Contains(returnedRequest.Command, returnedObservationPath) {
		t.Fatalf("returned member observation was not selected from canonical inbox: %+v", returnedInboxStatus.MissionControlRunbook.CurrentLoopOperator)
	}
	returned := runCurrentLoopResult(t, rekitCommandCLIArgs(t, returnedRequest.Command))
	returnedApplied := runCurrentLoopResult(t, rekitCommandCLIArgs(t, returned.ApplyCommand))
	if !returnedApplied.Applied || returnedApplied.InitialCurrentStep == nil || returnedApplied.InitialCurrentStep.MemberExecution == nil || returnedApplied.InitialCurrentStep.MemberExecution.Inspection.State != "intake-ready" {
		t.Fatalf("member returned resume apply command did not persist intake: %+v", returnedApplied)
	}
	if returnedApplied.ObservationReceipt == nil || returnedApplied.ObservationReceipt.ObservationKind != "member-session-returned" || returnedApplied.ObservationReceipt.Actor != "harness" || returnedApplied.ObservationReceipt.ObservationPath != returnedObservationPath || returnedApplied.ObservationReceipt.SourceCheckpointSHA256 != returnedSource.ArtifactSHA256 {
		t.Fatalf("member returned resume omitted processed observation receipt: %+v", returnedApplied.ObservationReceipt)
	}
	if returnedApplied.AppliedSteps != 1 || returnedApplied.StopReason.Code != "member-intake-ready" || returnedApplied.SegmentCheckpoint == nil || !returnedApplied.SegmentCheckpoint.Ready || returnedApplied.SegmentCheckpoint.RemainingMaxSteps != returned.MaxSteps-returnedApplied.AppliedSteps || returnedApplied.SegmentCheckpoint.Continuation == nil || returnedApplied.SegmentCheckpoint.Continuation.ExternalMemberHandoff != nil || returnedApplied.SegmentCheckpoint.Continuation.ObservationContract != nil {
		t.Fatalf("member returned result did not publish a clean remaining-budget continuation: %+v", returnedApplied)
	}
	if returnedApplied.StopReason.ExternalMemberHandoff != nil || returnedApplied.ContinuationRequest != nil && returnedApplied.ContinuationRequest.ExternalMemberHandoff != nil {
		t.Fatalf("intake-ready member result retained a stale external handoff: stop=%+v continuation=%+v", returnedApplied.StopReason, returnedApplied.ContinuationRequest)
	}
	var returnedStatusOut bytes.Buffer
	if err := Run([]string{"-Command", "status", "-Target", caseRoot, "-Pack", "_template", "-Format", "json"}, &returnedStatusOut); err != nil {
		t.Fatal(err)
	}
	var returnedStatus statusInventory
	if err := json.Unmarshal(returnedStatusOut.Bytes(), &returnedStatus); err != nil {
		t.Fatal(err)
	}
	returnedOperator := returnedStatus.MissionControlRunbook.CurrentLoopOperator
	if returnedOperator == nil || returnedOperator.ExternalMemberHandoff != nil {
		t.Fatalf("intake-ready status retained a stale member handoff: %+v", returnedOperator)
	}
	if returnedOperator.ObservationReceipt == nil || returnedOperator.ObservationReceipt.ObservationKind != "member-session-returned" || returnedOperator.ObservationReceipt.Actor != "harness" || returnedOperator.ObservationReceipt.SourceCheckpointSHA256 != returnedSource.ArtifactSHA256 || returnedOperator.ObservationReceipt.SuccessorCheckpointSHA256 != returnedApplied.SegmentCheckpoint.ArtifactSHA256 {
		t.Fatalf("intake-ready status omitted terminal successor observation receipt: %+v", returnedOperator)
	}
	returnedTakeover := returnedStatus.MissionControlRunbook.ReplacementExecutorTakeover
	if returnedTakeover == nil || returnedTakeover.CurrentLoopOperator == nil || returnedTakeover.CurrentLoopOperator.ObservationReceipt == nil || returnedTakeover.CurrentLoopOperator.ObservationReceipt.ObservationKind != "member-session-returned" {
		t.Fatalf("replacement takeover omitted terminal successor observation receipt: %+v", returnedTakeover)
	}
}

func runCurrentLoopError(caseRoot string, inputs []string) error {
	args := []string{"-Command", "run-current-loop", "-Target", caseRoot, "-Pack", "_template"}
	args = append(args, inputs...)
	args = append(args, "-Format", "json")
	return Run(args, &bytes.Buffer{})
}

func runCurrentLoopApply(t *testing.T, caseRoot string, preview currentLoopTestPlan) currentLoopTestPlan {
	t.Helper()
	return runCurrentLoopApplyWith(t, caseRoot, preview)
}

func runCurrentLoopApplyWith(t *testing.T, caseRoot string, preview currentLoopTestPlan, inputs ...string) currentLoopTestPlan {
	t.Helper()
	args := []string{"-Command", "run-current-loop", "-Target", caseRoot, "-Pack", "_template", "-MaxSteps", stringInt(preview.MaxSteps)}
	args = append(args, inputs...)
	args = append(args, "-ExpectedCurrentLoopPlanSha256", preview.ExpectedCurrentLoopPlanSHA256, "-Apply", "-Format", "json")
	return runCurrentLoopResult(t, args)
}

func runCurrentLoopResult(t *testing.T, args []string) currentLoopTestPlan {
	t.Helper()
	var out bytes.Buffer
	if err := Run(args, &out); err != nil {
		t.Fatal(err)
	}
	var result currentLoopTestPlan
	if err := json.Unmarshal(out.Bytes(), &result); err != nil {
		t.Fatalf("current-loop JSON did not decode: %v\n%s", err, out.String())
	}
	return result
}

func stringInt(value int) string {
	return strconv.Itoa(value)
}
