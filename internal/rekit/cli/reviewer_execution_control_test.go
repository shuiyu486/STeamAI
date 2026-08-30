package cli

import (
	"bytes"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/shuiyu486/re-context-kits/internal/rekit/executioncontrol"
	"github.com/shuiyu486/re-context-kits/internal/rekit/subagents"
)

func TestRunPlanSubagentsRejectsReviewerSessionBornBeforeControlAdvance(t *testing.T) {
	const lane = "feature-review-birth-control"
	caseRoot := filepath.Join(t.TempDir(), "case")
	var initOut bytes.Buffer
	runInitApplyFromPreview(t, &initOut, "-Command", "init", "-Target", caseRoot, "-Pack", "_template", "-ProjectName", "review-birth-control")
	var onboardOut bytes.Buffer
	onboardArgs := []string{
		"-Command", "onboard", "-Target", caseRoot, "-Pack", "_template",
		"-ProjectName", "review-birth-control", "-Goal", "review birth control",
		"-Actor", "mission-commander", "-Executor", "reviewer-main",
		"-InitialLane", lane, "-WhatIf", "-Format", "json",
	}
	if err := Run(onboardArgs, &onboardOut); err != nil {
		t.Fatal(err)
	}
	var onboard onboardCLIPlan
	if err := json.Unmarshal(onboardOut.Bytes(), &onboard); err != nil {
		t.Fatal(err)
	}
	onboardOut.Reset()
	if err := Run(onboard.ApplyArgs, &onboardOut); err != nil {
		t.Fatal(err)
	}
	writeCaseFile(t, caseRoot, ".steamai/board.json", `{"lanes":[{"id":"feature-review-birth-control","status":"open","workspace":"workspace/review-birth-control","currentExecutor":"reviewer-main","executorGeneration":1}]}`+"\n")
	writeCaseFile(t, caseRoot, ".steamai/lanes/feature-review-birth-control/lane.json", `{
  "schemaVersion": 1,
  "id": "feature-review-birth-control",
  "type": "feature",
  "name": "review-birth-control",
  "title": "Review birth control",
  "status": "open",
  "authority": false,
  "workspace": "workspace/review-birth-control",
  "canWrite": ["own-workspace"],
  "readOnly": [".steamai/facts/**"],
  "outputs": ["observation", "request", "candidate", "summary"],
  "counters": {},
  "currentExecutor": "reviewer-main",
  "executorGeneration": 1,
  "createdAt": "2026-08-25T00:00:00Z",
  "updatedAt": "2026-08-25T00:00:00Z"
}`+"\n")

	var out bytes.Buffer
	if err := Run([]string{
		"-Command", "plan-subagents", "-Target", caseRoot, "-Pack", "_template",
		"-TaskType", "feature-analysis", "-Items", "alpha", "-ItemsPerAgent", "1", "-MaxParallel", "1",
		"-Lane", lane, "-Format", "json",
	}, &out); err != nil {
		t.Fatal(err)
	}
	plan := decodePlanSubagentsResult(t, out.Bytes())
	if len(plan.ShardHandoffs) != 1 || plan.ShardHandoffs[0].AgentToolRequest == nil || plan.ShardHandoffs[0].AgentToolRequest.LaunchControl == nil {
		t.Fatalf("review plan omitted frozen launch control: %+v", plan.ShardHandoffs)
	}
	handoff := plan.ShardHandoffs[0]
	born := executioncontrol.CloneBinding(handoff.AgentToolRequest.LaunchControl)
	freshStatus := reviewerWaveStatusForLane(t, caseRoot, lane)
	freshWave := reviewerWaveFromStatus(freshStatus)
	if freshStatus.MissionControlRunbook == nil || freshStatus.MissionControlRunbook.CurrentLoopOperator == nil || freshStatus.MissionControlRunbook.CurrentLoopOperator.ExternalReviewerHandoff == nil || freshStatus.MissionControlRunbook.CurrentLoopOperator.ExternalReviewerHandoff.Attempt == nil {
		t.Fatalf("current status omitted reviewer attempt: %+v", freshStatus.MissionControlRunbook)
	}
	attempt := freshStatus.MissionControlRunbook.CurrentLoopOperator.ExternalReviewerHandoff.Attempt
	if freshWave == nil || len(freshWave.SpawnWave) != 1 || attempt.Identity.LaunchControl == nil || attempt.SelectedAction.AgentToolRequest == nil || !executioncontrol.SameBinding(attempt.Identity.LaunchControl, born) || !executioncontrol.SameBinding(attempt.SelectedAction.AgentToolRequest.LaunchControl, born) {
		t.Fatalf("current status lost frozen reviewer launch control: wave=%+v attempt=%+v", freshWave, attempt)
	}

	applyReviewerControlForTest(t, caseRoot, lane, executioncontrol.ActionPause, "pause launched reviewer lineage", "2026-08-25T00:01:00Z")
	applyReviewerControlForTest(t, caseRoot, lane, executioncontrol.ActionResume, "resume only future reviewer work", "2026-08-25T00:02:00Z")

	out.Reset()
	err := Run([]string{
		"-Command", "plan-subagents", "-Target", caseRoot, "-Pack", "_template",
		"-PacketPath", plan.PacketPath, "-RecordReviewerDispatch", "-ShardId", handoff.ShardID,
		"-ReviewerHarness", "go-test-harness", "-ReviewerSession", "born-before-control-advance",
		"-Lane", lane, "-Actor", "mission-commander", "-WhatIf", "-Format", "json",
	}, &out)
	if err == nil || !strings.Contains(err.Error(), executioncontrol.ResultDispositionStaleControl) {
		t.Fatalf("stale reviewer birth dispatch preview error = %v", err)
	}
	packet := decodePlanSubagentsPacket(t, plan.PacketPath)
	if packet.ShardHandoffs[0].AgentToolRequest == nil || !executioncontrol.SameBinding(packet.ShardHandoffs[0].AgentToolRequest.LaunchControl, born) {
		t.Fatalf("review packet rebound launch control after control advance: %+v", packet.ShardHandoffs[0].AgentToolRequest)
	}
	status := reviewerWaveStatusForLane(t, caseRoot, lane)
	wave := reviewerWaveFromStatus(status)
	if wave == nil || !wave.Paused || wave.Ready || len(wave.SpawnWave) != 0 {
		t.Fatalf("stale reviewer packet still exposed a spawn wave: %+v", wave)
	}
	sessionRoot := filepath.Join(filepath.Dir(plan.PacketPath), "sessions", handoff.ShardID)
	if _, statErr := os.Lstat(sessionRoot); !os.IsNotExist(statErr) {
		t.Fatalf("stale reviewer birth wrote session receipt namespace: %v", statErr)
	}
}

func TestRunPlanSubagentsRejectsStaleReviewerDispatchBeforeCompletionWrite(t *testing.T) {
	const lane = "feature-review-control"
	caseRoot := filepath.Join(t.TempDir(), "case")
	var initOut bytes.Buffer
	runInitApplyFromPreview(t, &initOut, "-Command", "init", "-Target", caseRoot, "-Pack", "_template", "-ProjectName", "review-control")
	onboardArgs := []string{
		"-Command", "onboard", "-Target", caseRoot, "-Pack", "_template",
		"-ProjectName", "review-control", "-Goal", "review control",
		"-Actor", "mission-commander", "-Executor", "reviewer-main",
		"-InitialLane", lane, "-WhatIf", "-Format", "json",
	}
	initOut.Reset()
	if err := Run(onboardArgs, &initOut); err != nil {
		t.Fatal(err)
	}
	var onboard onboardCLIPlan
	if err := json.Unmarshal(initOut.Bytes(), &onboard); err != nil {
		t.Fatal(err)
	}
	initOut.Reset()
	if err := Run(onboard.ApplyArgs, &initOut); err != nil {
		t.Fatal(err)
	}
	writeCaseFile(t, caseRoot, ".steamai/board.json", `{"lanes":[{"id":"feature-review-control","status":"open","workspace":"workspace/review-control","currentExecutor":"reviewer-main","executorGeneration":1}]}`+"\n")
	writeCaseFile(t, caseRoot, ".steamai/lanes/feature-review-control/lane.json", `{
  "schemaVersion": 1,
  "id": "feature-review-control",
  "type": "feature",
  "name": "review-control",
  "title": "Review control",
  "status": "open",
  "authority": false,
  "workspace": "workspace/review-control",
  "canWrite": ["own-workspace"],
  "readOnly": [".steamai/facts/**"],
  "outputs": ["observation", "request", "candidate", "summary"],
  "counters": {},
  "currentExecutor": "reviewer-main",
  "executorGeneration": 1,
  "createdAt": "2026-08-24T00:00:00Z",
  "updatedAt": "2026-08-24T00:00:00Z"
}`+"\n")

	var out bytes.Buffer
	if err := Run([]string{
		"-Command", "plan-subagents", "-Target", caseRoot, "-Pack", "_template",
		"-TaskType", "feature-analysis", "-Items", "alpha", "-ItemsPerAgent", "1", "-MaxParallel", "1",
		"-Lane", lane, "-Format", "json",
	}, &out); err != nil {
		t.Fatal(err)
	}
	plan := decodePlanSubagentsResult(t, out.Bytes())
	if len(plan.ShardHandoffs) != 1 {
		t.Fatalf("review plan handoffs = %+v", plan.ShardHandoffs)
	}
	handoff := plan.ShardHandoffs[0]

	out.Reset()
	if err := Run([]string{
		"-Command", "plan-subagents", "-Target", caseRoot, "-Pack", "_template",
		"-PacketPath", plan.PacketPath, "-RecordReviewerDispatch", "-ShardId", handoff.ShardID,
		"-ReviewerHarness", "go-test-harness", "-ReviewerSession", "stale-reviewer-session",
		"-Lane", lane, "-Actor", "mission-commander", "-WhatIf", "-Format", "json",
	}, &out); err != nil {
		t.Fatal(err)
	}
	var dispatchPreview subagents.ReviewerSessionReceiptResult
	if err := json.Unmarshal(out.Bytes(), &dispatchPreview); err != nil {
		t.Fatal(err)
	}

	out.Reset()
	if err := Run([]string{
		"-Command", "plan-subagents", "-Target", caseRoot, "-Pack", "_template",
		"-PacketPath", plan.PacketPath, "-RecordReviewerDispatch", "-ShardId", handoff.ShardID,
		"-ReviewerHarness", "go-test-harness", "-ReviewerSession", "stale-reviewer-session",
		"-Lane", lane, "-Actor", "mission-commander",
		"-ExpectedReviewerDispatchBindingSha256", dispatchPreview.BindingSHA256, "-Apply", "-Format", "json",
	}, &out); err != nil {
		t.Fatal(err)
	}
	var dispatch subagents.ReviewerSessionReceiptResult
	if err := json.Unmarshal(out.Bytes(), &dispatch); err != nil {
		t.Fatal(err)
	}

	out.Reset()
	if err := Run([]string{
		"-Command", "plan-subagents", "-Target", caseRoot, "-Pack", "_template",
		"-PacketPath", plan.PacketPath, "-RecordReviewerCompletion", "-ReviewerDispatchId", dispatch.DispatchID,
		"-ReviewerOutcome", "failed", "-ReviewerExitStatus", "stale-generation",
		"-Lane", lane, "-Actor", "mission-commander", "-WhatIf", "-Format", "json",
	}, &out); err != nil {
		t.Fatal(err)
	}
	var completionPreview subagents.ReviewerSessionReceiptResult
	if err := json.Unmarshal(out.Bytes(), &completionPreview); err != nil {
		t.Fatal(err)
	}

	applyReviewerControlForTest(t, caseRoot, lane, executioncontrol.ActionPause, "pause reviewer lineage", "2026-08-24T00:01:00Z")
	applyReviewerControlForTest(t, caseRoot, lane, executioncontrol.ActionResume, "resume only future reviewer work", "2026-08-24T00:02:00Z")

	out.Reset()
	err := Run([]string{
		"-Command", "plan-subagents", "-Target", caseRoot, "-Pack", "_template",
		"-PacketPath", plan.PacketPath, "-RecordReviewerCompletion", "-ReviewerDispatchId", dispatch.DispatchID,
		"-ReviewerOutcome", "failed", "-ReviewerExitStatus", "stale-generation",
		"-Lane", lane, "-Actor", "mission-commander",
		"-ExpectedReviewerDispatchReceiptSha256", completionPreview.DispatchReceiptSHA256, "-Apply", "-Format", "json",
	}, &out)
	if err == nil || !strings.Contains(err.Error(), executioncontrol.ResultDispositionStaleControl) {
		t.Fatalf("stale reviewer completion error = %v", err)
	}
	if _, statErr := os.Lstat(completionPreview.ReceiptPath); !os.IsNotExist(statErr) {
		t.Fatalf("stale reviewer completion wrote receipt: %v", statErr)
	}
}

func applyReviewerControlForTest(t *testing.T, caseRoot, lane, action, reason, stamp string) {
	t.Helper()
	preview, err := executioncontrol.Preview(caseRoot, executioncontrol.Options{
		Lane: lane, Action: action, Actor: "mission-commander", Reason: reason, PublicationStamp: stamp,
	})
	if err != nil {
		t.Fatal(err)
	}
	if _, err := executioncontrol.Apply(caseRoot, executioncontrol.Options{
		Lane: preview.Lane, Action: preview.Action, Actor: preview.Actor, Reason: preview.Reason,
		PublicationStamp: preview.PublicationStamp, ExpectedPlanSHA256: preview.ExpectedPlanSHA256,
	}); err != nil {
		t.Fatal(err)
	}
}
