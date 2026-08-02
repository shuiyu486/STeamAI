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

	boardPath := filepath.Join(caseRoot, ".rekit", "board.json")
	boardData, err := os.ReadFile(boardPath)
	if err != nil {
		t.Fatal(err)
	}
	var board map[string]any
	if err := json.Unmarshal(boardData, &board); err != nil {
		t.Fatal(err)
	}
	lanes, _ := board["lanes"].([]any)
	for _, item := range lanes {
		if lane, ok := item.(map[string]any); ok {
			lane["status"] = "closed"
		}
	}
	boardData, _ = json.Marshal(board)
	if err := os.WriteFile(boardPath, boardData, 0o644); err != nil {
		t.Fatal(err)
	}
	for _, lanePath := range []string{filepath.Join(caseRoot, ".rekit", "lanes", "main", "lane.json")} {
		if data, err := os.ReadFile(lanePath); err == nil {
			var lane map[string]any
			if json.Unmarshal(data, &lane) == nil {
				lane["status"] = "closed"
				data, _ = json.Marshal(lane)
				if err := os.WriteFile(lanePath, data, 0o644); err != nil {
					t.Fatal(err)
				}
			}
		}
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

	plan := decodePlanSubagentsPacket(t, dispatchApplied.FinalStatus.CaseMission.ReviewerDispatchIntakeSummary.LatestPacketPath)
	handoff := plan.ShardHandoffs[0]
	evidencePath := filepath.Join(caseRoot, "workspace", "features", "feature-login", "review-evidence.md")
	if err := os.MkdirAll(filepath.Dir(evidencePath), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(evidencePath, []byte("bounded current-loop reviewer evidence\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	resultSource := filepath.Join(caseRoot, "workspace", "current-loop-reviewer-result.json")
	if err := os.WriteFile(resultSource, reviewerResultForCLIPlan(t, plan, handoff, "accept", "accepted", "reviewer-session-runner"), 0o644); err != nil {
		t.Fatal(err)
	}
	resultInputs := []string{"-ExpectedCurrentLoopReviewerAttemptSha256", resultAttempt.AttemptSnapshotSHA256, "-ReviewerResultInputSourcePath", resultSource, "-Actor", actor}
	resultPreview := runCurrentLoopResumePreviewWith(t, caseRoot, dispatchApplied.SegmentCheckpoint.ArtifactSHA256, resultInputs...)
	if resultPreview.ExpectedCurrentLoopPlanSHA256 == "" || resultPreview.InitialRoute != "reviewer" || resultPreview.MaxSteps != continuation.RemainingMaxSteps || resultPreview.ResumeSource == nil || resultPreview.ApplyCommand == "" || !strings.Contains(resultPreview.ApplyCommand, "-ReviewerResultInputSourcePath") || !strings.Contains(resultPreview.ApplyCommand, resultSource) {
		t.Fatalf("typed durable result continuation did not rebuild a fresh preview and exact apply command: %+v", resultPreview)
	}
	resultApplyFields, err := splitDriverCommand(resultPreview.ApplyCommand)
	if err != nil {
		t.Fatal(err)
	}
	resultApplied := runCurrentLoopResult(t, append([]string{"-Command", resultApplyFields[1]}, resultApplyFields[2:]...))
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
	if campaign == nil || campaign.StopCode != "route-policy" || campaign.SegmentRoute != "reviewer" || campaign.ExpectedRoute != "case" || campaign.RemainingMaxSteps != resultPreview.MaxSteps-len(expectedSteps) || campaign.ObservationContract != nil || !strings.Contains(campaign.WhatIfCommand, "-MaxSteps 3") || len(resultApplied.Steps) != len(expectedSteps) {
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

func TestRunCurrentLoopResumeClaimRemainsConsumedAfterNestedFailure(t *testing.T) {
	caseRoot := fullAttachedCase(t)
	var out bytes.Buffer
	if err := Run([]string{"-Command", "overview", "-Target", caseRoot, "-Pack", "_template", "-Format", "json"}, &out); err != nil {
		t.Fatal(err)
	}
	initial := runCurrentLoopPreview(t, caseRoot, 3)
	driverStepAfterPreviewValidationHook = func() error {
		appendCurrentLoopOpenIntervention(t, caseRoot, "int-current-loop-consumed")
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
	currentLoopBeforeApplyStepHook = func(int) error { return os.ErrPermission }
	t.Cleanup(func() {
		currentLoopBeforeApplyStepHook = nil
		driverStepAfterPreviewValidationHook = nil
	})
	failed := runCurrentLoopResult(t, append([]string{"-Command", applyArgs[1]}, applyArgs[2:]...))
	if failed.Applied || failed.AppliedSteps != 0 || failed.StopReason.Code != "error" {
		t.Fatalf("resume nested failure was not returned without progress: %+v", failed)
	}
	consumed := currentloop.Inspect(repoRoot(t), caseRoot, "_template", failed.FinalStatus.MissionControlRunbook.CurrentDriverRequest)
	if consumed.Ready || consumed.State != "consumed" || consumed.ArtifactSHA256 != first.SegmentCheckpoint.ArtifactSHA256 || consumed.Continuation != nil || consumed.ResumeDriverRequest != nil {
		t.Fatalf("nested failure restored claimed resume budget: %+v", consumed)
	}
	var statusOut bytes.Buffer
	if err := Run([]string{"-Command", "status", "-Target", caseRoot, "-Pack", "_template", "-Format", "json"}, &statusOut); err != nil {
		t.Fatal(err)
	}
	var consumedStatus statusInventory
	if err := json.Unmarshal(statusOut.Bytes(), &consumedStatus); err != nil {
		t.Fatalf("consumed operator status did not decode: %v\n%s", err, statusOut.String())
	}
	consumedOperator := consumedStatus.MissionControlRunbook.CurrentLoopOperator
	if consumedOperator == nil || consumedOperator.Ready || consumedOperator.State != "checkpoint-consumed" || consumedOperator.SelectedDriverRequest != nil || consumedOperator.StartDriverRequest != nil || consumedOperator.ResumeDriverRequest != nil || consumedOperator.ExternalReviewerHandoff != nil {
		t.Fatalf("consumed checkpoint exposed an executable current-loop operator: %+v", consumedOperator)
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
	ApplyCommand                   string                          `json:"applyCommand"`
	SegmentCheckpoint              *currentloop.Inspection         `json:"segmentCheckpoint"`
	FinalStatus                    *statusInventory                `json:"finalStatus"`
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
