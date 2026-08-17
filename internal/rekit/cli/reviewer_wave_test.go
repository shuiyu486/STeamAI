package cli

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/shuiyu486/re-context-kits/internal/rekit/mission"
	"github.com/shuiyu486/re-context-kits/internal/rekit/workstream"
)

func TestRunReviewerWaveSelectedLaneOverridesGlobalReviewerWave(t *testing.T) {
	caseRoot := fullAttachedCase(t)
	var out bytes.Buffer
	if err := Run([]string{"-Command", "start", "-Target", caseRoot, "-Pack", "_template", "-Name", "review", "-Apply"}, &out); err != nil {
		t.Fatal(err)
	}

	planWave := func(root, lane, items string) *workstream.ReviewerDispatchWavePackage {
		t.Helper()
		out.Reset()
		if err := Run([]string{"-Command", "plan-subagents", "-Target", caseRoot, "-Pack", "_template", "-TaskType", "feature-analysis", "-Items", items, "-ItemsPerAgent", "1", "-MaxParallel", "2", "-Lane", lane, "-ReviewOutputDir", filepath.Join(caseRoot, ".rekit", "reviews", root), "-Format", "json"}, &out); err != nil {
			t.Fatal(err)
		}
		status := reviewerWaveStatusForLane(t, caseRoot, lane)
		wave := reviewerWaveFromStatus(status)
		if wave == nil || wave.TargetLane != lane {
			t.Fatalf("reviewer wave for lane %q = %+v", lane, wave)
		}
		return wave
	}

	laneA := "main"
	waveA := planWave("20-lane-a-wave", laneA, "alpha,beta")
	laneB := "feature-review"
	waveB := planWave("10-lane-b-wave", laneB, "gamma")
	global := reviewerWaveFromStatus(reviewerWaveStatus(t, caseRoot))
	if global == nil || global.TargetLane != laneB || global.PacketID != waveB.PacketID {
		t.Fatalf("global reviewer wave = %+v, want lane B %q", global, laneB)
	}

	observationPath := reviewerWaveStrictFile(t, waveA.PacketPath, "selected-lane-wave.json")
	writeReviewerWaveObservations(t, observationPath, reviewerWaveObservationFile{SchemaVersion: 1, PacketID: waveA.PacketID, Observations: []reviewerWaveObservation{
		{ShardID: waveA.SpawnWave[0].ShardID, Kind: "accepted", ReviewerHarness: "go-test-harness", ReviewerSession: "selected-lane-wave-01"},
		{ShardID: waveA.SpawnWave[1].ShardID, Kind: "accepted", ReviewerHarness: "go-test-harness", ReviewerSession: "selected-lane-wave-02"},
	}})
	args := []string{"-Command", "run-reviewer-wave", "-Target", caseRoot, "-Pack", "_template", "-PacketPath", waveA.PacketPath, "-Lane", laneA, "-Actor", "mission-commander", "-ReviewerWaveObservationsPath", observationPath, "-Format", "json"}
	preview := reviewerWavePreview(t, append(args, "-WhatIf")...)
	if preview.Lane != laneA || preview.PacketID != waveA.PacketID || preview.WaveSnapshotSHA256 != waveA.SnapshotSHA256 {
		t.Fatalf("selected-lane reviewer wave preview = %+v", preview)
	}
	selectedStatus := reviewerWaveStatusForLane(t, caseRoot, laneA)
	assertSelectedLaneReviewerWaveContracts(t, selectedStatus, laneA)

	out.Reset()
	if err := Run(append(args, "-ExpectedReviewerWavePlanSha256", preview.ExpectedReviewerWavePlanSHA256, "-Apply"), &out); err != nil {
		t.Fatal(err)
	}
	var applied reviewerWavePlan
	if err := json.Unmarshal(out.Bytes(), &applied); err != nil {
		t.Fatal(err)
	}
	if !applied.Applied || applied.AppliedCount != 2 || applied.RefreshedWave == nil || applied.RefreshedWave.TargetLane != laneA || applied.RefreshedWave.PacketID != waveA.PacketID || len(applied.RefreshedWave.Active) != 2 {
		t.Fatalf("selected-lane reviewer wave Apply = %+v", applied)
	}
	global = reviewerWaveFromStatus(reviewerWaveStatus(t, caseRoot))
	if global == nil || global.TargetLane != laneB || global.PacketID != waveB.PacketID {
		t.Fatalf("selected-lane Apply changed global reviewer wave: %+v", global)
	}
}

func TestRunReviewerWaveSelectedLanePartialFailureRefreshesExactLane(t *testing.T) {
	caseRoot := fullAttachedCase(t)
	var out bytes.Buffer
	if err := Run([]string{"-Command", "start", "-Target", caseRoot, "-Pack", "_template", "-Name", "review", "-Apply"}, &out); err != nil {
		t.Fatal(err)
	}
	plan := func(root, lane, items string) *workstream.ReviewerDispatchWavePackage {
		t.Helper()
		out.Reset()
		if err := Run([]string{"-Command", "plan-subagents", "-Target", caseRoot, "-Pack", "_template", "-TaskType", "feature-analysis", "-Items", items, "-ItemsPerAgent", "1", "-MaxParallel", "2", "-Lane", lane, "-ReviewOutputDir", filepath.Join(caseRoot, ".rekit", "reviews", root), "-Format", "json"}, &out); err != nil {
			t.Fatal(err)
		}
		return reviewerWaveFromStatus(reviewerWaveStatusForLane(t, caseRoot, lane))
	}
	laneA := "main"
	waveA := plan("20-lane-a-partial-wave", laneA, "alpha,beta")
	laneB := "feature-review"
	waveB := plan("10-lane-b-partial-wave", laneB, "gamma")
	observationPath := reviewerWaveStrictFile(t, waveA.PacketPath, "selected-lane-wave-partial.json")
	writeReviewerWaveObservations(t, observationPath, reviewerWaveObservationFile{SchemaVersion: 1, PacketID: waveA.PacketID, Observations: []reviewerWaveObservation{
		{ShardID: waveA.SpawnWave[0].ShardID, Kind: "accepted", ReviewerHarness: "go-test-harness", ReviewerSession: "selected-lane-partial-01"},
		{ShardID: waveA.SpawnWave[1].ShardID, Kind: "accepted", ReviewerHarness: "go-test-harness", ReviewerSession: "selected-lane-partial-02"},
	}})
	args := []string{"-Command", "run-reviewer-wave", "-Target", caseRoot, "-Pack", "_template", "-PacketPath", waveA.PacketPath, "-Lane", laneA, "-Actor", "mission-commander", "-ReviewerWaveObservationsPath", observationPath, "-Format", "json"}
	preview := reviewerWavePreview(t, append(args, "-WhatIf")...)
	reviewerWaveBeforeApplyObservationHook = func(index int) error {
		if index == 2 {
			return errors.New("selected-lane partial failure")
		}
		return nil
	}
	defer func() { reviewerWaveBeforeApplyObservationHook = nil }()
	out.Reset()
	if err := Run(append(args, "-ExpectedReviewerWavePlanSha256", preview.ExpectedReviewerWavePlanSHA256, "-Apply"), &out); err != nil {
		t.Fatal(err)
	}
	var partial reviewerWavePlan
	if err := json.Unmarshal(out.Bytes(), &partial); err != nil {
		t.Fatal(err)
	}
	if !partial.Applied || partial.AppliedCount != 1 || partial.FailedIndex != 2 || partial.RefreshedWave == nil || partial.RefreshedWave.TargetLane != laneA || partial.RefreshedWave.PacketID != waveA.PacketID || len(partial.RefreshedWave.Active) != 1 {
		t.Fatalf("selected-lane reviewer wave partial failure = %+v", partial)
	}
	global := reviewerWaveFromStatus(reviewerWaveStatus(t, caseRoot))
	if global == nil || global.TargetLane != laneB || global.PacketID != waveB.PacketID {
		t.Fatalf("selected-lane partial failure refreshed global reviewer wave: %+v", global)
	}
}

func TestRunReviewerWaveRecordsParallelAcceptances(t *testing.T) {
	caseRoot := fullAttachedCase(t)
	var out bytes.Buffer
	if err := Run([]string{"-Command", "start", "-Target", caseRoot, "-Pack", "_template", "-Name", "review", "-Apply"}, &out); err != nil {
		t.Fatal(err)
	}
	out.Reset()
	if err := Run([]string{"-Command", "plan-subagents", "-Target", caseRoot, "-Pack", "_template", "-TaskType", "feature-analysis", "-Items", "alpha,beta", "-ItemsPerAgent", "1", "-MaxParallel", "2", "-Format", "json"}, &out); err != nil {
		t.Fatal(err)
	}
	status := reviewerWaveStatus(t, caseRoot)
	wave := reviewerWaveFromStatus(status)
	if wave == nil || len(wave.SpawnWave) != 2 {
		t.Fatalf("initial reviewer wave = %+v", wave)
	}
	observationPath := reviewerWaveStrictFile(t, wave.PacketPath, "wave-accepted.json")
	writeReviewerWaveObservations(t, observationPath, reviewerWaveObservationFile{SchemaVersion: 1, PacketID: wave.PacketID, Observations: []reviewerWaveObservation{
		{ShardID: wave.SpawnWave[0].ShardID, Kind: "accepted", ReviewerHarness: "go-test-harness", ReviewerSession: "wave-session-01"},
		{ShardID: wave.SpawnWave[1].ShardID, Kind: "accepted", ReviewerHarness: "go-test-harness", ReviewerSession: "wave-session-02"},
	}})
	args := []string{"-Command", "run-reviewer-wave", "-Target", caseRoot, "-Pack", "_template", "-PacketPath", wave.PacketPath, "-Lane", wave.TargetLane, "-Actor", "mission-commander", "-ReviewerWaveObservationsPath", observationPath, "-Format", "json"}
	preview := reviewerWavePreview(t, append(args, "-WhatIf")...)
	if preview.ObservationCount != 2 || len(preview.Previews) != 2 || preview.ExpectedReviewerWavePlanSHA256 == "" || preview.Previews[0].ExpectedBindingSHA256 == "" || preview.Previews[1].ExpectedBindingSHA256 == "" {
		t.Fatalf("reviewer wave acceptance preview = %+v", preview)
	}
	out.Reset()
	applyArgs := append(append([]string{}, args...), "-ExpectedReviewerWavePlanSha256", preview.ExpectedReviewerWavePlanSHA256, "-Apply")
	if err := Run(applyArgs, &out); err != nil {
		t.Fatal(err)
	}
	var applied reviewerWavePlan
	if err := json.Unmarshal(out.Bytes(), &applied); err != nil {
		t.Fatal(err)
	}
	if !applied.Applied || applied.AppliedCount != 2 || applied.RefreshedWave == nil || applied.RefreshedWave.ActiveSlots != 2 || len(applied.RefreshedWave.Active) != 2 || len(applied.RefreshedWave.SpawnWave) != 0 {
		t.Fatalf("reviewer wave acceptance apply = %+v", applied)
	}
	for _, item := range applied.RefreshedWave.Active {
		if item.AgentToolRequest != nil || item.ReviewerSession == "" {
			t.Fatalf("active reviewer shard still exposed spawn request: %+v", item)
		}
	}
	out.Reset()
	if err := Run(applyArgs, &out); err == nil || (!strings.Contains(err.Error(), "expected plan sha256 mismatch") && !strings.Contains(err.Error(), "current spawnWave")) {
		t.Fatalf("stale reviewer wave bundle was not rejected: %v", err)
	}
}

func TestRunReviewerWaveRecordsReturnedAndFailedObservations(t *testing.T) {
	caseRoot := fullAttachedCase(t)
	var out bytes.Buffer
	if err := Run([]string{"-Command", "start", "-Target", caseRoot, "-Pack", "_template", "-Name", "review", "-Apply"}, &out); err != nil {
		t.Fatal(err)
	}
	out.Reset()
	if err := Run([]string{"-Command", "plan-subagents", "-Target", caseRoot, "-Pack", "_template", "-TaskType", "feature-analysis", "-Items", "alpha,beta", "-ItemsPerAgent", "1", "-MaxParallel", "2", "-Format", "json"}, &out); err != nil {
		t.Fatal(err)
	}
	wave := reviewerWaveFromStatus(reviewerWaveStatus(t, caseRoot))
	acceptPath := reviewerWaveStrictFile(t, wave.PacketPath, "wave-accept-for-terminal.json")
	writeReviewerWaveObservations(t, acceptPath, reviewerWaveObservationFile{SchemaVersion: 1, PacketID: wave.PacketID, Observations: []reviewerWaveObservation{
		{ShardID: wave.SpawnWave[0].ShardID, Kind: "accepted", ReviewerHarness: "go-test-harness", ReviewerSession: "wave-return-session"},
		{ShardID: wave.SpawnWave[1].ShardID, Kind: "accepted", ReviewerHarness: "go-test-harness", ReviewerSession: "wave-fail-session"},
	}})
	acceptArgs := []string{"-Command", "run-reviewer-wave", "-Target", caseRoot, "-Pack", "_template", "-PacketPath", wave.PacketPath, "-Lane", wave.TargetLane, "-Actor", "mission-commander", "-ReviewerWaveObservationsPath", acceptPath, "-Format", "json"}
	acceptPreview := reviewerWavePreview(t, append(acceptArgs, "-WhatIf")...)
	out.Reset()
	if err := Run(append(acceptArgs, "-ExpectedReviewerWavePlanSha256", acceptPreview.ExpectedReviewerWavePlanSHA256, "-Apply"), &out); err != nil {
		t.Fatal(err)
	}
	activeWave := reviewerWaveFromStatus(reviewerWaveStatus(t, caseRoot))
	if len(activeWave.Active) != 2 {
		t.Fatalf("accepted reviewer wave = %+v", activeWave)
	}
	packet := decodePlanSubagentsPacket(t, activeWave.PacketPath)
	first := packet.ShardHandoffs[0]
	evidencePath := filepath.Join(caseRoot, "workspace", "features", "feature-login", "review-evidence.md")
	if err := os.MkdirAll(filepath.Dir(evidencePath), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(evidencePath, []byte("bounded reviewer evidence\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	resultSource := reviewerWaveStrictFile(t, wave.PacketPath, "wave-return-result.json")
	if err := os.WriteFile(resultSource, reviewerResultForCLIPlan(t, packet, first, "accept", "accepted", "wave-return-session"), 0o644); err != nil {
		t.Fatal(err)
	}
	terminalPath := reviewerWaveStrictFile(t, wave.PacketPath, "wave-terminal.json")
	writeReviewerWaveObservations(t, terminalPath, reviewerWaveObservationFile{SchemaVersion: 1, PacketID: activeWave.PacketID, Observations: []reviewerWaveObservation{
		{ShardID: activeWave.Active[0].ShardID, Kind: "returned", ReviewerExitStatus: "completed", ReviewerResultInputSourcePath: resultSource},
		{ShardID: activeWave.Active[1].ShardID, Kind: "failed", ReviewerDispatchID: activeWave.Active[1].ReviewerDispatchID, ReviewerExitStatus: "reviewer-error"},
	}})
	terminalArgs := []string{"-Command", "run-reviewer-wave", "-Target", caseRoot, "-Pack", "_template", "-PacketPath", activeWave.PacketPath, "-Lane", activeWave.TargetLane, "-Actor", "mission-commander", "-ReviewerWaveObservationsPath", terminalPath, "-Format", "json"}
	terminalPreview := reviewerWavePreview(t, append(terminalArgs, "-WhatIf")...)
	if terminalPreview.Previews[0].ExpectedInputSaveSHA256 == "" || terminalPreview.Previews[0].ExpectedDispatchSHA256 == "" || terminalPreview.Previews[1].ExpectedDispatchSHA256 == "" {
		t.Fatalf("terminal reviewer wave preview = %+v", terminalPreview)
	}
	out.Reset()
	if err := Run(append(terminalArgs, "-ExpectedReviewerWavePlanSha256", terminalPreview.ExpectedReviewerWavePlanSHA256, "-Apply"), &out); err != nil {
		t.Fatal(err)
	}
	var applied reviewerWavePlan
	if err := json.Unmarshal(out.Bytes(), &applied); err != nil {
		t.Fatal(err)
	}
	if !applied.Applied || applied.AppliedCount != 2 || applied.RefreshedWave == nil || applied.RefreshedWave.ActiveSlots != 0 || len(applied.RefreshedWave.Returned) != 1 || len(applied.RefreshedWave.Failed) != 1 || len(applied.RefreshedWave.SpawnWave) != 1 || applied.RefreshedWave.SpawnWave[0].ShardID != activeWave.Active[1].ShardID {
		t.Fatalf("terminal reviewer wave apply = %+v", applied)
	}
}

func TestRunReviewerWavePausesForOpenInterventionAndRejectsStalePreview(t *testing.T) {
	caseRoot := fullAttachedCase(t)
	var out bytes.Buffer
	if err := Run([]string{"-Command", "start", "-Target", caseRoot, "-Pack", "_template", "-Name", "review", "-Executor", "executor-a", "-Actor", "mission-commander", "-Reason", "initial reviewer owner", "-Apply"}, &out); err != nil {
		t.Fatal(err)
	}
	out.Reset()
	if err := Run([]string{"-Command", "plan-subagents", "-Target", caseRoot, "-Pack", "_template", "-TaskType", "feature-analysis", "-Items", "alpha,beta", "-ItemsPerAgent", "1", "-MaxParallel", "2", "-Lane", "feature-review", "-Format", "json"}, &out); err != nil {
		t.Fatal(err)
	}
	wave := reviewerWaveFromStatus(reviewerWaveStatus(t, caseRoot))
	observationPath := reviewerWaveStrictFile(t, wave.PacketPath, "wave-intervention-stale-preview.json")
	writeReviewerWaveObservations(t, observationPath, reviewerWaveObservationFile{SchemaVersion: 1, PacketID: wave.PacketID, Observations: []reviewerWaveObservation{{ShardID: wave.SpawnWave[0].ShardID, Kind: "accepted", ReviewerHarness: "go-test-harness", ReviewerSession: "stale-session"}}})
	args := []string{"-Command", "run-reviewer-wave", "-Target", caseRoot, "-Pack", "_template", "-PacketPath", wave.PacketPath, "-Lane", wave.TargetLane, "-Actor", "mission-commander", "-ReviewerWaveObservationsPath", observationPath, "-Format", "json"}
	preview := reviewerWavePreview(t, append(args, "-WhatIf")...)
	if err := appendReviewerWaveIntervention(t, caseRoot, wave.TargetLane, "wave-intervention-stale-preview"); err != nil {
		t.Fatal(err)
	}
	pausedStatus := reviewerWaveStatus(t, caseRoot)
	paused := reviewerWaveFromStatus(pausedStatus)
	pkg := pausedStatus.CaseMission.ReviewerDispatchIntakeSummary.OperatorPackage
	if paused == nil || paused.Ready || !paused.Paused || paused.InterventionID != "wave-intervention-stale-preview" || paused.SnapshotSHA256 != "" || paused.AvailableSlots != 0 || len(paused.SpawnWave) != 0 || pkg == nil || pkg.Ready || !pkg.Paused || pkg.CurrentDriverRequest != nil || pkg.Current == nil || reviewerOperatorItemExposesExecutableAction(*pkg.Current) || reviewerRunLoopExposesExecutableAction(pkg.RunLoop) || reviewerWaveExposesExecutableAction(paused) {
		t.Fatalf("open intervention did not pause reviewer wave: package=%+v wave=%+v", pkg, paused)
	}
	if len(pausedStatus.CaseMission.ReviewerDispatchIntakeHandoffs) == 0 {
		t.Fatal("paused status omitted diagnostic reviewer handoffs")
	}
	for _, handoff := range pausedStatus.CaseMission.ReviewerDispatchIntakeHandoffs {
		if !handoff.Paused || handoff.InterventionID != "wave-intervention-stale-preview" || reviewerHandoffExposesExecutableAction(handoff) {
			t.Fatalf("paused raw reviewer handoff exposed executable action: %+v", handoff)
		}
	}
	if queue := pausedStatus.CaseMission.ReviewerDispatchIntakeActionQueue; queue.CurrentAction != nil || queue.CurrentDriverRequest != nil || queue.Counts.Total != 0 || len(queue.UnblockedActions) != 0 || len(queue.BlockedActions) != 0 {
		t.Fatalf("paused reviewer action queue exposed stale action: %+v", queue)
	}
	out.Reset()
	err := Run(append(args, "-ExpectedReviewerWavePlanSha256", preview.ExpectedReviewerWavePlanSHA256, "-Apply"), &out)
	if err == nil || !strings.Contains(err.Error(), "paused by open intervention") || !strings.Contains(err.Error(), "wave-intervention-stale-preview") {
		t.Fatalf("stale preview intervention error = %v", err)
	}
	refreshed := reviewerWaveFromStatus(reviewerWaveStatus(t, caseRoot))
	if len(refreshed.Active) != 0 || len(refreshed.SpawnWave) != 0 {
		t.Fatalf("stale preview wrote reviewer receipt: %+v", refreshed)
	}

	directArgs := []string{"-Command", "plan-subagents", "-Target", caseRoot, "-Pack", "_template", "-PacketPath", wave.PacketPath, "-ShardId", wave.Shards[0].ShardID, "-RecordReviewerDispatch", "-ReviewerHarness", "go-test-harness", "-ReviewerSession", "direct-stale-session", "-Lane", wave.TargetLane, "-Actor", "mission-commander", "-WhatIf", "-Format", "json"}
	out.Reset()
	err = Run(directArgs, &out)
	if err == nil || !strings.Contains(err.Error(), "paused by open intervention") || !strings.Contains(err.Error(), "wave-intervention-stale-preview") {
		t.Fatalf("direct reviewer dispatch bypassed intervention: %v", err)
	}
	out.Reset()
	err = Run([]string{"-Command", "run-reviewer-step", "-Target", caseRoot, "-Pack", "_template", "-ReviewerHarness", "go-test-harness", "-ReviewerSession", "single-step-stale-session", "-Actor", "mission-commander", "-WhatIf", "-Format", "json"}, &out)
	if err == nil || !strings.Contains(err.Error(), "ready reviewer operator package") {
		t.Fatalf("single reviewer step bypassed intervention: %v", err)
	}
}

func reviewerHandoffExposesExecutableAction(item workstream.ReviewerDispatchIntakeHandoff) bool {
	if item.AgentToolRequest != nil || item.DispatchPromptRepairCommand != "" || item.ReviewerDispatchRecordCommand != "" || item.ReviewerCompletionRecordCommand != "" || item.ReviewerResultInputSaveCommand != "" || item.ReviewerResultInputSaveApplyCommand != "" || item.ReviewerResultSourceCaptureCommand != "" || item.ReviewerResultSourceCaptureApplyCommand != "" || item.ReviewerResultStagingCommand != "" || item.ReviewerResultCollectionCommands != nil || item.PreviewCommand != "" || item.ApplyCommand != "" || item.BatchPreviewCommand != "" || item.BatchApplyCommand != "" || item.DispatchCommand != "" {
		return true
	}
	managed := item.ManagedDispatch
	return managed != nil && (managed.AgentToolRequest != nil || managed.InputSavePreviewCommand != "" || managed.InputSaveApplyCommand != "" || managed.SourceCapturePreviewCommand != "" || managed.SourceCaptureApplyCommand != "" || managed.StagingPreviewCommand != "" || managed.CollectionPreviewCommand != "" || managed.CollectionApplyCommand != "" || managed.IntakePreviewCommand != "" || managed.IntakeApplyCommand != "" || managed.DispatchCommand != "" || managed.NextAction != "")
}

func reviewerOperatorItemExposesExecutableAction(item workstream.ReviewerDispatchOperatorPackageItem) bool {
	return item.AgentToolRequest != nil || item.DispatchPromptRepairCommand != "" || item.ReviewerDispatchRecordCommand != "" || item.ReviewerCompletionRecordCommand != "" || item.ReviewerResultInputSavePreviewCommand != "" || item.ReviewerResultInputSaveApplyCommand != "" || item.ReviewerResultSourceCapturePreviewCommand != "" || item.ReviewerResultSourceCaptureApplyCommand != "" || item.ReviewerResultStagingPreviewCommand != "" || item.ReviewerResultCollectionPreviewCommand != "" || item.ReviewerResultCollectionApplyCommand != "" || item.ReviewerResultIntakePreviewCommand != "" || item.ReviewerResultIntakeApplyCommand != "" || item.ReviewerResultBatchIntakePreviewCommand != "" || item.ReviewerResultBatchIntakeApplyCommand != "" || item.DispatchCommand != "" || item.NextAction != ""
}

func reviewerRunLoopExposesExecutableAction(steps []workstream.ReviewerDispatchRunLoopStep) bool {
	for _, step := range steps {
		if step.AgentToolRequest != nil || step.Command != "" || step.PreviewCommand != "" || step.ApplyCommand != "" {
			return true
		}
	}
	return false
}

func reviewerWaveExposesExecutableAction(wave *workstream.ReviewerDispatchWavePackage) bool {
	groups := [][]workstream.ReviewerDispatchWavePackageItem{wave.Shards, wave.Active, wave.Returned, wave.Failed, wave.Blocked, wave.Complete}
	for _, group := range groups {
		for _, item := range group {
			if item.AgentToolRequest != nil || item.RecordDispatchCommand != "" || item.RecordCompletionCommand != "" || item.CurrentDriverRequest != nil {
				return true
			}
		}
	}
	return false
}

func TestRunReviewerWaveStopsBundleWhenInterventionArrivesBetweenObservations(t *testing.T) {
	caseRoot := fullAttachedCase(t)
	var out bytes.Buffer
	if err := Run([]string{"-Command", "start", "-Target", caseRoot, "-Pack", "_template", "-Name", "review", "-Apply"}, &out); err != nil {
		t.Fatal(err)
	}
	out.Reset()
	if err := Run([]string{"-Command", "plan-subagents", "-Target", caseRoot, "-Pack", "_template", "-TaskType", "feature-analysis", "-Items", "alpha,beta", "-ItemsPerAgent", "1", "-MaxParallel", "2", "-Format", "json"}, &out); err != nil {
		t.Fatal(err)
	}
	wave := reviewerWaveFromStatus(reviewerWaveStatus(t, caseRoot))
	observationPath := reviewerWaveStrictFile(t, wave.PacketPath, "wave-intervention-partial.json")
	writeReviewerWaveObservations(t, observationPath, reviewerWaveObservationFile{SchemaVersion: 1, PacketID: wave.PacketID, Observations: []reviewerWaveObservation{
		{ShardID: wave.SpawnWave[0].ShardID, Kind: "accepted", ReviewerHarness: "go-test-harness", ReviewerSession: "wave-before-intervention"},
		{ShardID: wave.SpawnWave[1].ShardID, Kind: "accepted", ReviewerHarness: "go-test-harness", ReviewerSession: "wave-after-intervention"},
	}})
	args := []string{"-Command", "run-reviewer-wave", "-Target", caseRoot, "-Pack", "_template", "-PacketPath", wave.PacketPath, "-Lane", wave.TargetLane, "-Actor", "mission-commander", "-ReviewerWaveObservationsPath", observationPath, "-Format", "json"}
	preview := reviewerWavePreview(t, append(args, "-WhatIf")...)
	reviewerWaveBeforeObservationInterventionCheckHook = func(index int) error {
		if index == 2 {
			return appendReviewerWaveIntervention(t, caseRoot, wave.TargetLane, "wave-intervention-partial")
		}
		return nil
	}
	defer func() { reviewerWaveBeforeObservationInterventionCheckHook = nil }()
	out.Reset()
	if err := Run(append(args, "-ExpectedReviewerWavePlanSha256", preview.ExpectedReviewerWavePlanSHA256, "-Apply"), &out); err != nil {
		t.Fatal(err)
	}
	var partial reviewerWavePlan
	if err := json.Unmarshal(out.Bytes(), &partial); err != nil {
		t.Fatal(err)
	}
	if !partial.Applied || partial.AppliedCount != 1 || partial.FailedIndex != 2 || !strings.Contains(partial.Failure, "wave-intervention-partial") || partial.RefreshedWave == nil || !partial.RefreshedWave.Paused || len(partial.RefreshedWave.Active) != 1 || partial.RefreshedWave.Active[0].ShardID != wave.SpawnWave[0].ShardID || len(partial.RefreshedWave.SpawnWave) != 0 {
		t.Fatalf("intervention partial receipt = %+v", partial)
	}
}

func TestRunReviewerWaveResumesThroughReplacementExecutorTakeover(t *testing.T) {
	caseRoot := fullAttachedCase(t)
	var out bytes.Buffer
	if err := Run([]string{"-Command", "start", "-Target", caseRoot, "-Pack", "_template", "-Name", "review", "-Executor", "executor-a", "-Actor", "mission-commander", "-Reason", "initial reviewer owner", "-Apply"}, &out); err != nil {
		t.Fatal(err)
	}
	out.Reset()
	if err := Run([]string{"-Command", "plan-subagents", "-Target", caseRoot, "-Pack", "_template", "-TaskType", "feature-analysis", "-Items", "alpha", "-ItemsPerAgent", "1", "-MaxParallel", "1", "-Lane", "feature-review", "-Format", "json"}, &out); err != nil {
		t.Fatal(err)
	}
	wave := reviewerWaveFromStatus(reviewerWaveStatus(t, caseRoot))
	packet := decodePlanSubagentsPacket(t, wave.PacketPath)
	applyWave := func(name string, observations []reviewerWaveObservation) reviewerWavePlan {
		path := reviewerWaveStrictFile(t, wave.PacketPath, name+".json")
		writeReviewerWaveObservations(t, path, reviewerWaveObservationFile{SchemaVersion: 1, PacketID: wave.PacketID, Observations: observations})
		args := []string{"-Command", "run-reviewer-wave", "-Target", caseRoot, "-Pack", "_template", "-PacketPath", wave.PacketPath, "-Lane", wave.TargetLane, "-Actor", "mission-commander", "-ReviewerWaveObservationsPath", path, "-Format", "json"}
		preview := reviewerWavePreview(t, append(args, "-WhatIf")...)
		out.Reset()
		if err := Run(append(args, "-ExpectedReviewerWavePlanSha256", preview.ExpectedReviewerWavePlanSHA256, "-Apply"), &out); err != nil {
			t.Fatal(err)
		}
		var applied reviewerWavePlan
		if err := json.Unmarshal(out.Bytes(), &applied); err != nil {
			t.Fatal(err)
		}
		wave = applied.RefreshedWave
		return applied
	}
	applyWave("wave-takeover-initial-accept", []reviewerWaveObservation{{ShardID: wave.SpawnWave[0].ShardID, Kind: "accepted", ReviewerHarness: "go-test-harness", ReviewerSession: "reviewer-session-a"}})
	oldDispatchID := wave.Active[0].ReviewerDispatchID
	if err := appendReviewerWaveIntervention(t, caseRoot, wave.TargetLane, "wave-intervention-takeover"); err != nil {
		t.Fatal(err)
	}
	pausedStatus := reviewerWaveStatus(t, caseRoot)
	if pausedStatus.CaseMission == nil || pausedStatus.CaseMission.MissionCommanderActionQueue.CurrentAction == nil || pausedStatus.CaseMission.MissionCommanderActionQueue.CurrentAction.State != "needs-reconcile" || reviewerWaveFromStatus(pausedStatus) == nil || !reviewerWaveFromStatus(pausedStatus).Paused {
		t.Fatalf("intervention did not route active wave to reconcile: %+v", pausedStatus.CaseMission)
	}
	out.Reset()
	if err := Run([]string{"-Command", "reconcile", "-Target", caseRoot, "-Pack", "_template", "-Apply", "review", "-InterventionId", "wave-intervention-takeover", "-Executor", "executor-b", "-Actor", "mission-commander", "-Reason", "accept human reviewer correction", "-Format", "json"}, &out); err != nil {
		t.Fatal(err)
	}
	out.Reset()
	err := Run([]string{"-Command", "plan-subagents", "-Target", caseRoot, "-Pack", "_template", "-PacketPath", wave.PacketPath, "-RecordReviewerCompletion", "-ReviewerDispatchId", oldDispatchID, "-ReviewerOutcome", "failed", "-ReviewerExitStatus", "stale-session", "-Lane", wave.TargetLane, "-Actor", "mission-commander", "-WhatIf", "-Format", "json"}, &out)
	if err == nil || (!strings.Contains(err.Error(), "stale lane owner generation") && !strings.Contains(err.Error(), "ownerBinding is stale")) {
		t.Fatalf("old reviewer completion survived replacement takeover: %v", err)
	}
	takeoverStatus := reviewerWaveStatus(t, caseRoot)
	if takeoverStatus.CaseMission == nil || len(takeoverStatus.CaseMission.ReviewerDispatchIntakeHandoffs) != 1 || takeoverStatus.CaseMission.ReviewerDispatchIntakeHandoffs[0].State != "reviewer-packet-owner-adoption-required" {
		t.Fatalf("replacement takeover did not require packet adoption: %+v", takeoverStatus.CaseMission)
	}
	out.Reset()
	if err := Run([]string{"-Command", "plan-subagents", "-Target", caseRoot, "-Pack", "_template", "-PacketPath", wave.PacketPath, "-AdoptReviewerPacket", "-Lane", wave.TargetLane, "-Actor", "mission-commander", "-Reason", "adopt reviewer packet after intervention takeover", "-Apply", "-Format", "json"}, &out); err != nil {
		t.Fatal(err)
	}
	wave = reviewerWaveFromStatus(reviewerWaveStatus(t, caseRoot))
	if wave == nil || !wave.Ready || wave.Paused || len(wave.SpawnWave) != 1 || wave.SpawnWave[0].ShardID != packet.ShardHandoffs[0].ShardID || wave.SpawnWave[0].ReviewerDispatchID != oldDispatchID {
		t.Fatalf("adopted packet did not expose replacement spawn wave: %+v", wave)
	}
	applyWave("wave-takeover-replacement-accept", []reviewerWaveObservation{{ShardID: wave.SpawnWave[0].ShardID, Kind: "accepted", ReviewerHarness: "go-test-harness", ReviewerSession: "reviewer-session-b"}})
	if len(wave.Active) != 1 || wave.Active[0].ReviewerDispatchID == oldDispatchID || wave.Active[0].ReviewerSession != "reviewer-session-b" {
		t.Fatalf("replacement dispatch did not supersede stale session: %+v", wave)
	}
	evidencePath := filepath.Join(caseRoot, "workspace", "features", "feature-login", "review-evidence.md")
	if err := os.MkdirAll(filepath.Dir(evidencePath), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(evidencePath, []byte("bounded reviewer evidence\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	resultPath := reviewerWaveStrictFile(t, wave.PacketPath, "wave-takeover-result.json")
	if err := os.WriteFile(resultPath, reviewerResultForCLIPlan(t, packet, packet.ShardHandoffs[0], "accept", "accepted", "reviewer-session-b"), 0o644); err != nil {
		t.Fatal(err)
	}
	applyWave("wave-takeover-replacement-return", []reviewerWaveObservation{{ShardID: wave.Active[0].ShardID, Kind: "returned", ReviewerExitStatus: "completed", ReviewerResultInputSourcePath: resultPath}})
	loopPreview := runCurrentLoopPreviewWith(t, caseRoot, 8, "-Actor", "mission-commander")
	loopApplied := runCurrentLoopApplyWith(t, caseRoot, loopPreview, "-Actor", "mission-commander")
	if !loopApplied.Applied || loopApplied.AppliedSteps != 4 || loopApplied.StopReason.Code != "route-policy" {
		t.Fatalf("replacement reviewer result did not drain to writeback: %+v", loopApplied)
	}
	for _, kind := range []string{"verification", "decision"} {
		facts, err := mission.ReadStrictFact(caseRoot, kind)
		if err != nil {
			t.Fatal(err)
		}
		matched := 0
		for _, fact := range facts {
			if mission.Value(fact, "packetId") == packet.PacketID && mission.Value(fact, "shardId") == packet.ShardHandoffs[0].ShardID && mission.Value(fact, "ownerExecutor") == "executor-b" {
				matched++
			}
		}
		if matched != 1 {
			t.Fatalf("replacement takeover wrote %d matching %s facts: %+v", matched, kind, facts)
		}
	}
}

func appendReviewerWaveIntervention(t *testing.T, caseRoot, lane, eventID string) error {
	t.Helper()
	var out bytes.Buffer
	return Run([]string{"-Command", "note", "-Target", caseRoot, "-Pack", "_template", "-Kind", "intervention", "-Lane", lane, "-Subject", "pause reviewer wave", "-Summary", "human changed reviewer direction", "-Action", "override", "-Status", "open", "-Actor", "human", "-EventId", eventID, "-CreatedAt", "2026-08-03T00:00:00Z", "-Format", "json"}, &out)
}

func TestRunReviewerWaveRejectsInvalidObservationContracts(t *testing.T) {
	caseRoot := fullAttachedCase(t)
	var out bytes.Buffer
	if err := Run([]string{"-Command", "start", "-Target", caseRoot, "-Pack", "_template", "-Name", "review", "-Apply"}, &out); err != nil {
		t.Fatal(err)
	}
	out.Reset()
	if err := Run([]string{"-Command", "plan-subagents", "-Target", caseRoot, "-Pack", "_template", "-TaskType", "feature-analysis", "-Items", "alpha,beta", "-ItemsPerAgent", "1", "-MaxParallel", "2", "-Format", "json"}, &out); err != nil {
		t.Fatal(err)
	}
	wave := reviewerWaveFromStatus(reviewerWaveStatus(t, caseRoot))
	observationPath := reviewerWaveStrictFile(t, wave.PacketPath, "wave-invalid.json")
	args := []string{"-Command", "run-reviewer-wave", "-Target", caseRoot, "-Pack", "_template", "-PacketPath", wave.PacketPath, "-Lane", wave.TargetLane, "-Actor", "mission-commander", "-ReviewerWaveObservationsPath", observationPath, "-WhatIf", "-Format", "json"}
	valid := reviewerWaveObservation{ShardID: wave.SpawnWave[0].ShardID, Kind: "accepted", ReviewerHarness: "go-test-harness", ReviewerSession: "wave-contract-session"}

	tests := []struct {
		name       string
		file       reviewerWaveObservationFile
		packetPath string
		lane       string
		want       string
	}{
		{name: "duplicate shard", file: reviewerWaveObservationFile{SchemaVersion: 1, PacketID: wave.PacketID, Observations: []reviewerWaveObservation{valid, valid}}, want: "repeats shardId"},
		{name: "packet mismatch", file: reviewerWaveObservationFile{SchemaVersion: 1, PacketID: "other-packet", Observations: []reviewerWaveObservation{valid}}, want: "does not match current packet"},
		{name: "lane mismatch", file: reviewerWaveObservationFile{SchemaVersion: 1, PacketID: wave.PacketID, Observations: []reviewerWaveObservation{valid}}, lane: "other-lane", want: "is not current"},
		{name: "accepted terminal field", file: reviewerWaveObservationFile{SchemaVersion: 1, PacketID: wave.PacketID, Observations: []reviewerWaveObservation{{ShardID: valid.ShardID, Kind: "accepted", ReviewerHarness: valid.ReviewerHarness, ReviewerSession: valid.ReviewerSession, ReviewerExitStatus: "completed"}}}, want: "does not accept terminal"},
		{name: "returned without result", file: reviewerWaveObservationFile{SchemaVersion: 1, PacketID: wave.PacketID, Observations: []reviewerWaveObservation{{ShardID: valid.ShardID, Kind: "returned"}}}, want: "requires reviewerResultInputSourcePath"},
		{name: "failed without dispatch", file: reviewerWaveObservationFile{SchemaVersion: 1, PacketID: wave.PacketID, Observations: []reviewerWaveObservation{{ShardID: valid.ShardID, Kind: "failed", ReviewerExitStatus: "reviewer-error"}}}, want: "requires reviewerDispatchId"},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			writeReviewerWaveObservations(t, observationPath, test.file)
			callArgs := append([]string{}, args...)
			if test.lane != "" {
				for idx := range callArgs {
					if callArgs[idx] == "-Lane" {
						callArgs[idx+1] = test.lane
					}
				}
			}
			var result bytes.Buffer
			err := Run(callArgs, &result)
			if err == nil || !strings.Contains(err.Error(), test.want) {
				t.Fatalf("invalid reviewer wave contract error = %v, want %q", err, test.want)
			}
		})
	}

	if err := os.WriteFile(observationPath, []byte(`{"schemaVersion":1,"packetId":"`+wave.PacketID+`","observations":[{"shardId":"`+valid.ShardID+`","kind":"accepted","reviewerHarness":"go-test-harness","reviewerSession":"wave-contract-session","unknown":true}]}`), 0o644); err != nil {
		t.Fatal(err)
	}
	out.Reset()
	if err := Run(args, &out); err == nil || !strings.Contains(err.Error(), "unknown field") {
		t.Fatalf("unknown reviewer wave field error = %v", err)
	}
}

func TestRunReviewerWaveRejectsAcceptanceBeyondSpawnWave(t *testing.T) {
	caseRoot := fullAttachedCase(t)
	var out bytes.Buffer
	if err := Run([]string{"-Command", "start", "-Target", caseRoot, "-Pack", "_template", "-Name", "review", "-Apply"}, &out); err != nil {
		t.Fatal(err)
	}
	out.Reset()
	if err := Run([]string{"-Command", "plan-subagents", "-Target", caseRoot, "-Pack", "_template", "-TaskType", "feature-analysis", "-Items", "alpha,beta", "-ItemsPerAgent", "1", "-MaxParallel", "1", "-Format", "json"}, &out); err != nil {
		t.Fatal(err)
	}
	wave := reviewerWaveFromStatus(reviewerWaveStatus(t, caseRoot))
	if wave.MaxParallel != 1 || len(wave.SpawnWave) != 1 || len(wave.Shards) != 2 {
		t.Fatalf("bounded reviewer wave = %+v", wave)
	}
	observationPath := reviewerWaveStrictFile(t, wave.PacketPath, "wave-over-capacity.json")
	writeReviewerWaveObservations(t, observationPath, reviewerWaveObservationFile{SchemaVersion: 1, PacketID: wave.PacketID, Observations: []reviewerWaveObservation{
		{ShardID: wave.Shards[0].ShardID, Kind: "accepted", ReviewerHarness: "go-test-harness", ReviewerSession: "wave-capacity-01"},
		{ShardID: wave.Shards[1].ShardID, Kind: "accepted", ReviewerHarness: "go-test-harness", ReviewerSession: "wave-capacity-02"},
	}})
	args := []string{"-Command", "run-reviewer-wave", "-Target", caseRoot, "-Pack", "_template", "-PacketPath", wave.PacketPath, "-Lane", wave.TargetLane, "-Actor", "mission-commander", "-ReviewerWaveObservationsPath", observationPath, "-WhatIf", "-Format", "json"}
	out.Reset()
	if err := Run(args, &out); err == nil || !strings.Contains(err.Error(), "current spawnWave") {
		t.Fatalf("over-capacity acceptance error = %v", err)
	}
	refreshed := reviewerWaveFromStatus(reviewerWaveStatus(t, caseRoot))
	if refreshed.ActiveSlots != 0 {
		t.Fatalf("over-capacity acceptance wrote dispatch state: %+v", refreshed)
	}
}

func TestRunReviewerWaveBindsObservationFileBytes(t *testing.T) {
	caseRoot := fullAttachedCase(t)
	var out bytes.Buffer
	if err := Run([]string{"-Command", "start", "-Target", caseRoot, "-Pack", "_template", "-Name", "review", "-Apply"}, &out); err != nil {
		t.Fatal(err)
	}
	out.Reset()
	if err := Run([]string{"-Command", "plan-subagents", "-Target", caseRoot, "-Pack", "_template", "-TaskType", "feature-analysis", "-Items", "alpha", "-ItemsPerAgent", "1", "-MaxParallel", "1", "-Format", "json"}, &out); err != nil {
		t.Fatal(err)
	}
	wave := reviewerWaveFromStatus(reviewerWaveStatus(t, caseRoot))
	observationPath := reviewerWaveStrictFile(t, wave.PacketPath, "wave-drift.json")
	observation := reviewerWaveObservationFile{SchemaVersion: 1, PacketID: wave.PacketID, Observations: []reviewerWaveObservation{{ShardID: wave.SpawnWave[0].ShardID, Kind: "accepted", ReviewerHarness: "go-test-harness", ReviewerSession: "wave-before-drift"}}}
	writeReviewerWaveObservations(t, observationPath, observation)
	args := []string{"-Command", "run-reviewer-wave", "-Target", caseRoot, "-Pack", "_template", "-PacketPath", wave.PacketPath, "-Lane", wave.TargetLane, "-Actor", "mission-commander", "-ReviewerWaveObservationsPath", observationPath, "-Format", "json"}
	preview := reviewerWavePreview(t, append(args, "-WhatIf")...)
	observation.Observations[0].ReviewerSession = "wave-after-drift"
	writeReviewerWaveObservations(t, observationPath, observation)
	out.Reset()
	err := Run(append(args, "-ExpectedReviewerWavePlanSha256", preview.ExpectedReviewerWavePlanSHA256, "-Apply"), &out)
	if err == nil || !strings.Contains(err.Error(), "expected plan sha256 mismatch") {
		t.Fatalf("observation file drift error = %v", err)
	}
}

func TestReadReviewerWaveObservationsRejectsUnsafeFiles(t *testing.T) {
	caseRoot := t.TempDir()
	packetPath := filepath.Join(caseRoot, ".steamai", "reviews", "unsafe-files", "packet.json")
	workspace := filepath.Dir(reviewerWaveStrictFile(t, packetPath, "fixture.json"))
	outside := filepath.Join(t.TempDir(), "outside.json")
	if err := os.WriteFile(outside, []byte("{}\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if _, _, _, err := readReviewerWaveObservations(caseRoot, outside); err == nil || !strings.Contains(err.Error(), "case-local") {
		t.Fatalf("outside observation error = %v", err)
	}
	oversized := filepath.Join(workspace, "oversized.json")
	if err := os.WriteFile(oversized, bytes.Repeat([]byte{'x'}, 256*1024+1), 0o644); err != nil {
		t.Fatal(err)
	}
	if _, _, _, err := readReviewerWaveObservations(caseRoot, oversized); err == nil || !strings.Contains(err.Error(), "bounded") {
		t.Fatalf("oversized observation error = %v", err)
	}
	linkRoot := filepath.Join(caseRoot, "linked-workspace")
	if err := os.Symlink(filepath.Dir(outside), linkRoot); err != nil {
		t.Skipf("directory symlink unavailable: %v", err)
	}
	linked := filepath.Join(linkRoot, filepath.Base(outside))
	if _, _, _, err := readReviewerWaveObservations(caseRoot, linked); err == nil || !strings.Contains(err.Error(), "symlink-free") {
		t.Fatalf("symlink ancestor observation error = %v", err)
	}
	stablePath := filepath.Join(workspace, "stable.json")
	if err := os.WriteFile(stablePath, []byte("{}\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	reviewerWaveBeforeObservationOpenHook = func() error {
		if err := os.Remove(stablePath); err != nil {
			return err
		}
		return os.Symlink(outside, stablePath)
	}
	defer func() { reviewerWaveBeforeObservationOpenHook = nil }()
	if _, _, _, err := readReviewerWaveObservations(caseRoot, stablePath); err == nil || !strings.Contains(err.Error(), "changed while opening") {
		if err != nil && strings.Contains(err.Error(), "privilege") {
			t.Skipf("file symlink unavailable: %v", err)
		}
		t.Fatalf("observation replacement error = %v", err)
	}
}

func TestRunReviewerWaveRejectsReturnedResultFromPriorDispatch(t *testing.T) {
	caseRoot := fullAttachedCase(t)
	var out bytes.Buffer
	if err := Run([]string{"-Command", "start", "-Target", caseRoot, "-Pack", "_template", "-Name", "review", "-Apply"}, &out); err != nil {
		t.Fatal(err)
	}
	out.Reset()
	if err := Run([]string{"-Command", "plan-subagents", "-Target", caseRoot, "-Pack", "_template", "-TaskType", "feature-analysis", "-Items", "alpha", "-ItemsPerAgent", "1", "-MaxParallel", "1", "-Format", "json"}, &out); err != nil {
		t.Fatal(err)
	}
	wave := reviewerWaveFromStatus(reviewerWaveStatus(t, caseRoot))
	applyWave := func(path string, observations []reviewerWaveObservation) reviewerWavePlan {
		writeReviewerWaveObservations(t, path, reviewerWaveObservationFile{SchemaVersion: 1, PacketID: wave.PacketID, Observations: observations})
		args := []string{"-Command", "run-reviewer-wave", "-Target", caseRoot, "-Pack", "_template", "-PacketPath", wave.PacketPath, "-Lane", wave.TargetLane, "-Actor", "mission-commander", "-ReviewerWaveObservationsPath", path, "-Format", "json"}
		preview := reviewerWavePreview(t, append(args, "-WhatIf")...)
		out.Reset()
		if err := Run(append(args, "-ExpectedReviewerWavePlanSha256", preview.ExpectedReviewerWavePlanSHA256, "-Apply"), &out); err != nil {
			t.Fatal(err)
		}
		var applied reviewerWavePlan
		if err := json.Unmarshal(out.Bytes(), &applied); err != nil {
			t.Fatal(err)
		}
		wave = applied.RefreshedWave
		return applied
	}
	applyWave(reviewerWaveStrictFile(t, wave.PacketPath, "wave-prior-accept.json"), []reviewerWaveObservation{{ShardID: wave.SpawnWave[0].ShardID, Kind: "accepted", ReviewerHarness: "go-test-harness", ReviewerSession: "wave-prior-session"}})
	priorDispatchID := wave.Active[0].ReviewerDispatchID
	applyWave(reviewerWaveStrictFile(t, wave.PacketPath, "wave-prior-failed.json"), []reviewerWaveObservation{{ShardID: wave.Active[0].ShardID, Kind: "failed", ReviewerDispatchID: priorDispatchID, ReviewerExitStatus: "reviewer-error"}})
	applyWave(reviewerWaveStrictFile(t, wave.PacketPath, "wave-current-accept.json"), []reviewerWaveObservation{{ShardID: wave.SpawnWave[0].ShardID, Kind: "accepted", ReviewerHarness: "go-test-harness", ReviewerSession: "wave-current-session"}})
	if wave.Active[0].ReviewerDispatchID == priorDispatchID {
		t.Fatalf("reviewer shard did not receive a new dispatch: %+v", wave.Active[0])
	}
	packet := decodePlanSubagentsPacket(t, wave.PacketPath)
	evidencePath := filepath.Join(caseRoot, "workspace", "features", "feature-login", "review-evidence.md")
	if err := os.MkdirAll(filepath.Dir(evidencePath), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(evidencePath, []byte("bounded reviewer evidence\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	resultSource := reviewerWaveStrictFile(t, wave.PacketPath, "wave-prior-result.json")
	if err := os.WriteFile(resultSource, reviewerResultForCLIPlan(t, packet, packet.ShardHandoffs[0], "accept", "accepted", "wave-prior-session"), 0o644); err != nil {
		t.Fatal(err)
	}
	observationPath := reviewerWaveStrictFile(t, wave.PacketPath, "wave-prior-return.json")
	writeReviewerWaveObservations(t, observationPath, reviewerWaveObservationFile{SchemaVersion: 1, PacketID: wave.PacketID, Observations: []reviewerWaveObservation{{ShardID: wave.Active[0].ShardID, Kind: "returned", ReviewerExitStatus: "completed", ReviewerResultInputSourcePath: resultSource}}})
	args := []string{"-Command", "run-reviewer-wave", "-Target", caseRoot, "-Pack", "_template", "-PacketPath", wave.PacketPath, "-Lane", wave.TargetLane, "-Actor", "mission-commander", "-ReviewerWaveObservationsPath", observationPath, "-WhatIf", "-Format", "json"}
	out.Reset()
	if err := Run(args, &out); err == nil || (!strings.Contains(err.Error(), "returned result dispatch does not match") && !strings.Contains(err.Error(), "belongs to failed reviewer dispatch") && !strings.Contains(err.Error(), "does not match current reviewer dispatch")) {
		t.Fatalf("prior dispatch returned result error = %v", err)
	}
	refreshed := reviewerWaveFromStatus(reviewerWaveStatus(t, caseRoot))
	if len(refreshed.Active) != 1 || refreshed.Active[0].ReviewerDispatchID != wave.Active[0].ReviewerDispatchID || len(refreshed.Returned) != 0 {
		t.Fatalf("prior dispatch result changed current wave: %+v", refreshed)
	}
}

func TestRunReviewerWaveReturnedPartialReportsInputMutation(t *testing.T) {
	caseRoot := fullAttachedCase(t)
	var out bytes.Buffer
	if err := Run([]string{"-Command", "start", "-Target", caseRoot, "-Pack", "_template", "-Name", "review", "-Apply"}, &out); err != nil {
		t.Fatal(err)
	}
	out.Reset()
	if err := Run([]string{"-Command", "plan-subagents", "-Target", caseRoot, "-Pack", "_template", "-TaskType", "feature-analysis", "-Items", "alpha", "-ItemsPerAgent", "1", "-MaxParallel", "1", "-Format", "json"}, &out); err != nil {
		t.Fatal(err)
	}
	wave := reviewerWaveFromStatus(reviewerWaveStatus(t, caseRoot))
	acceptPath := reviewerWaveStrictFile(t, wave.PacketPath, "wave-partial-return-accept.json")
	writeReviewerWaveObservations(t, acceptPath, reviewerWaveObservationFile{SchemaVersion: 1, PacketID: wave.PacketID, Observations: []reviewerWaveObservation{{ShardID: wave.SpawnWave[0].ShardID, Kind: "accepted", ReviewerHarness: "go-test-harness", ReviewerSession: "wave-partial-return"}}})
	acceptArgs := []string{"-Command", "run-reviewer-wave", "-Target", caseRoot, "-Pack", "_template", "-PacketPath", wave.PacketPath, "-Lane", wave.TargetLane, "-Actor", "mission-commander", "-ReviewerWaveObservationsPath", acceptPath, "-Format", "json"}
	acceptPreview := reviewerWavePreview(t, append(acceptArgs, "-WhatIf")...)
	out.Reset()
	if err := Run(append(acceptArgs, "-ExpectedReviewerWavePlanSha256", acceptPreview.ExpectedReviewerWavePlanSHA256, "-Apply"), &out); err != nil {
		t.Fatal(err)
	}
	activeWave := reviewerWaveFromStatus(reviewerWaveStatus(t, caseRoot))
	packet := decodePlanSubagentsPacket(t, activeWave.PacketPath)
	evidencePath := filepath.Join(caseRoot, "workspace", "features", "feature-login", "review-evidence.md")
	if err := os.MkdirAll(filepath.Dir(evidencePath), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(evidencePath, []byte("bounded reviewer evidence\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	resultSource := reviewerWaveStrictFile(t, wave.PacketPath, "wave-partial-return-result.json")
	if err := os.WriteFile(resultSource, reviewerResultForCLIPlan(t, packet, packet.ShardHandoffs[0], "accept", "accepted", "wave-partial-return"), 0o644); err != nil {
		t.Fatal(err)
	}
	terminalPath := reviewerWaveStrictFile(t, wave.PacketPath, "wave-partial-return.json")
	writeReviewerWaveObservations(t, terminalPath, reviewerWaveObservationFile{SchemaVersion: 1, PacketID: activeWave.PacketID, Observations: []reviewerWaveObservation{{ShardID: activeWave.Active[0].ShardID, Kind: "returned", ReviewerExitStatus: "completed", ReviewerResultInputSourcePath: resultSource}}})
	terminalArgs := []string{"-Command", "run-reviewer-wave", "-Target", caseRoot, "-Pack", "_template", "-PacketPath", activeWave.PacketPath, "-Lane", activeWave.TargetLane, "-Actor", "mission-commander", "-ReviewerWaveObservationsPath", terminalPath, "-Format", "json"}
	terminalPreview := reviewerWavePreview(t, append(terminalArgs, "-WhatIf")...)
	reviewerWaveBeforeReturnedCompletionHook = func() error { return os.ErrPermission }
	defer func() { reviewerWaveBeforeReturnedCompletionHook = nil }()
	out.Reset()
	if err := Run(append(terminalArgs, "-ExpectedReviewerWavePlanSha256", terminalPreview.ExpectedReviewerWavePlanSHA256, "-Apply"), &out); err != nil {
		t.Fatal(err)
	}
	var partial reviewerWavePlan
	if err := json.Unmarshal(out.Bytes(), &partial); err != nil {
		t.Fatal(err)
	}
	if !partial.Applied || partial.AppliedCount != 0 || partial.FailedIndex != 1 || partial.Failure == "" || partial.RefreshedWave == nil || len(partial.RefreshedWave.Returned) != 1 {
		t.Fatalf("returned partial receipt = %+v", partial)
	}
}

func TestRunCurrentLoopDrainsReturnedShardBeforeRetryingFailedShard(t *testing.T) {
	caseRoot := fullAttachedCase(t)
	var out bytes.Buffer
	if err := Run([]string{"-Command", "start", "-Target", caseRoot, "-Pack", "_template", "-Name", "review", "-Apply"}, &out); err != nil {
		t.Fatal(err)
	}
	out.Reset()
	if err := Run([]string{"-Command", "plan-subagents", "-Target", caseRoot, "-Pack", "_template", "-TaskType", "feature-analysis", "-Items", "alpha,beta", "-ItemsPerAgent", "1", "-MaxParallel", "2", "-Format", "json"}, &out); err != nil {
		t.Fatal(err)
	}
	wave := reviewerWaveFromStatus(reviewerWaveStatus(t, caseRoot))
	applyWave := func(path string, observations []reviewerWaveObservation) {
		writeReviewerWaveObservations(t, path, reviewerWaveObservationFile{SchemaVersion: 1, PacketID: wave.PacketID, Observations: observations})
		args := []string{"-Command", "run-reviewer-wave", "-Target", caseRoot, "-Pack", "_template", "-PacketPath", wave.PacketPath, "-Lane", wave.TargetLane, "-Actor", "mission-commander", "-ReviewerWaveObservationsPath", path, "-Format", "json"}
		preview := reviewerWavePreview(t, append(args, "-WhatIf")...)
		out.Reset()
		if err := Run(append(args, "-ExpectedReviewerWavePlanSha256", preview.ExpectedReviewerWavePlanSHA256, "-Apply"), &out); err != nil {
			t.Fatal(err)
		}
		var applied reviewerWavePlan
		if err := json.Unmarshal(out.Bytes(), &applied); err != nil {
			t.Fatal(err)
		}
		wave = applied.RefreshedWave
	}
	applyWave(reviewerWaveStrictFile(t, wave.PacketPath, "wave-mixed-drain-accept.json"), []reviewerWaveObservation{
		{ShardID: wave.SpawnWave[0].ShardID, Kind: "accepted", ReviewerHarness: "go-test-harness", ReviewerSession: "wave-mixed-drain-01"},
		{ShardID: wave.SpawnWave[1].ShardID, Kind: "accepted", ReviewerHarness: "go-test-harness", ReviewerSession: "wave-mixed-drain-02"},
	})
	packet := decodePlanSubagentsPacket(t, wave.PacketPath)
	evidencePath := filepath.Join(caseRoot, "workspace", "features", "feature-login", "review-evidence.md")
	if err := os.MkdirAll(filepath.Dir(evidencePath), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(evidencePath, []byte("bounded reviewer evidence\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	resultPath := reviewerWaveStrictFile(t, wave.PacketPath, "wave-mixed-drain-result.json")
	if err := os.WriteFile(resultPath, reviewerResultForCLIPlan(t, packet, packet.ShardHandoffs[0], "accept", "accepted", "wave-mixed-drain-01"), 0o644); err != nil {
		t.Fatal(err)
	}
	applyWave(reviewerWaveStrictFile(t, wave.PacketPath, "wave-mixed-drain-terminal.json"), []reviewerWaveObservation{
		{ShardID: packet.ShardHandoffs[0].ShardID, Kind: "returned", ReviewerExitStatus: "completed", ReviewerResultInputSourcePath: resultPath},
		{ShardID: packet.ShardHandoffs[1].ShardID, Kind: "failed", ReviewerDispatchID: wave.Active[1].ReviewerDispatchID, ReviewerExitStatus: "reviewer-error"},
	})
	status := reviewerWaveStatus(t, caseRoot)
	if status.CaseMission == nil || status.CaseMission.ReviewerDispatchIntakeSummary.NextActionShardID != packet.ShardHandoffs[0].ShardID || status.CaseMission.ReviewerDispatchIntakeSummary.NextActionState != "ready-for-reviewer-result-source-capture-preview" {
		t.Fatalf("mixed returned/failed wave did not prioritize durable result drain: %+v", status.CaseMission.ReviewerDispatchIntakeSummary)
	}
	preview := runCurrentLoopPreviewWith(t, caseRoot, 5, "-Actor", "mission-commander")
	applied := runCurrentLoopApplyWith(t, caseRoot, preview, "-Actor", "mission-commander")
	if !applied.Applied || applied.AppliedSteps != 3 || applied.StopReason.Code != "external-reviewer-handoff" || applied.FinalStatus == nil || applied.FinalStatus.CaseMission == nil || applied.FinalStatus.CaseMission.ReviewerDispatchIntakeSummary.NextActionShardID != packet.ShardHandoffs[1].ShardID {
		t.Fatalf("mixed reviewer wave drain result = %+v", applied)
	}
	for _, ledger := range []string{"verifications.jsonl", "decisions.jsonl"} {
		data, err := os.ReadFile(filepath.Join(caseRoot, ".rekit", "facts", ledger))
		if err != nil && !os.IsNotExist(err) {
			t.Fatal(err)
		}
		if strings.Contains(string(data), packet.PacketID) {
			t.Fatalf("partial packet drain wrote premature %s for packet %s", ledger, packet.PacketID)
		}
	}
}

func TestRunCurrentLoopDrainsReturnedReviewerWave(t *testing.T) {
	caseRoot := fullAttachedCase(t)
	var out bytes.Buffer
	if err := Run([]string{"-Command", "start", "-Target", caseRoot, "-Pack", "_template", "-Name", "review", "-Apply"}, &out); err != nil {
		t.Fatal(err)
	}
	out.Reset()
	if err := Run([]string{"-Command", "plan-subagents", "-Target", caseRoot, "-Pack", "_template", "-TaskType", "feature-analysis", "-Items", "alpha,beta", "-ItemsPerAgent", "1", "-MaxParallel", "2", "-Format", "json"}, &out); err != nil {
		t.Fatal(err)
	}
	wave := reviewerWaveFromStatus(reviewerWaveStatus(t, caseRoot))
	applyWave := func(path string, observations []reviewerWaveObservation) reviewerWavePlan {
		writeReviewerWaveObservations(t, path, reviewerWaveObservationFile{SchemaVersion: 1, PacketID: wave.PacketID, Observations: observations})
		args := []string{"-Command", "run-reviewer-wave", "-Target", caseRoot, "-Pack", "_template", "-PacketPath", wave.PacketPath, "-Lane", wave.TargetLane, "-Actor", "mission-commander", "-ReviewerWaveObservationsPath", path, "-Format", "json"}
		preview := reviewerWavePreview(t, append(args, "-WhatIf")...)
		out.Reset()
		if err := Run(append(args, "-ExpectedReviewerWavePlanSha256", preview.ExpectedReviewerWavePlanSHA256, "-Apply"), &out); err != nil {
			t.Fatal(err)
		}
		var applied reviewerWavePlan
		if err := json.Unmarshal(out.Bytes(), &applied); err != nil {
			t.Fatal(err)
		}
		wave = applied.RefreshedWave
		return applied
	}
	applyWave(reviewerWaveStrictFile(t, wave.PacketPath, "wave-drain-accept.json"), []reviewerWaveObservation{
		{ShardID: wave.SpawnWave[0].ShardID, Kind: "accepted", ReviewerHarness: "go-test-harness", ReviewerSession: "wave-drain-01"},
		{ShardID: wave.SpawnWave[1].ShardID, Kind: "accepted", ReviewerHarness: "go-test-harness", ReviewerSession: "wave-drain-02"},
	})
	packet := decodePlanSubagentsPacket(t, wave.PacketPath)
	evidencePath := filepath.Join(caseRoot, "workspace", "features", "feature-login", "review-evidence.md")
	if err := os.MkdirAll(filepath.Dir(evidencePath), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(evidencePath, []byte("bounded reviewer evidence\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	terminal := make([]reviewerWaveObservation, 0, 2)
	for idx, handoff := range packet.ShardHandoffs {
		session := "wave-drain-0" + string(rune('1'+idx))
		resultPath := reviewerWaveStrictFile(t, wave.PacketPath, "wave-drain-result-0"+string(rune('1'+idx))+".json")
		if err := os.WriteFile(resultPath, reviewerResultForCLIPlan(t, packet, handoff, "accept", "accepted", session), 0o644); err != nil {
			t.Fatal(err)
		}
		terminal = append(terminal, reviewerWaveObservation{ShardID: handoff.ShardID, Kind: "returned", ReviewerExitStatus: "completed", ReviewerResultInputSourcePath: resultPath})
	}
	applyWave(reviewerWaveStrictFile(t, wave.PacketPath, "wave-drain-return.json"), terminal)
	if len(wave.Returned) != 2 || wave.ActiveSlots != 0 {
		t.Fatalf("returned reviewer wave = %+v", wave)
	}
	preview := runCurrentLoopPreviewWith(t, caseRoot, 10, "-Actor", "mission-commander")
	if preview.ExpectedCurrentLoopPlanSHA256 == "" || preview.InitialCurrentStep == nil {
		t.Fatalf("returned wave did not enter deterministic drain loop: %+v", preview)
	}
	applied := runCurrentLoopApplyWith(t, caseRoot, preview, "-Actor", "mission-commander")
	if !applied.Applied || applied.AppliedSteps != 7 || applied.StopReason.Code != "route-policy" || applied.FinalStatus == nil || applied.FinalStatus.MissionControlRunbook == nil || applied.FinalStatus.MissionControlRunbook.Scope != "case" {
		t.Fatalf("reviewer wave drain result = %+v", applied)
	}
	wantShards := map[string]int{}
	for _, handoff := range packet.ShardHandoffs {
		wantShards[handoff.ShardID] = 1
	}
	for _, kind := range []string{"verification", "decision"} {
		facts, err := mission.ReadStrictFact(caseRoot, kind)
		if err != nil {
			t.Fatal(err)
		}
		gotShards := map[string]int{}
		for _, fact := range facts {
			if mission.Value(fact, "packetId") == packet.PacketID {
				gotShards[mission.Value(fact, "shardId")]++
			}
		}
		if len(gotShards) != len(wantShards) {
			t.Fatalf("reviewer wave drain wrote wrong %s shard set: got=%v want=%v facts=%+v", kind, gotShards, wantShards, facts)
		}
		for shardID, wantCount := range wantShards {
			if gotShards[shardID] != wantCount {
				t.Fatalf("reviewer wave drain wrote %s shard %s %d times, want %d: %+v", kind, shardID, gotShards[shardID], wantCount, facts)
			}
		}
	}
}

func TestRunReviewerWaveApplyRejectsCompletionIntentBeforeObservation(t *testing.T) {
	caseRoot := fullAttachedCase(t)
	var out bytes.Buffer
	if err := Run([]string{"-Command", "start", "-Target", caseRoot, "-Pack", "_template", "-Name", "review", "-Apply"}, &out); err != nil {
		t.Fatal(err)
	}
	out.Reset()
	if err := Run([]string{"-Command", "plan-subagents", "-Target", caseRoot, "-Pack", "_template", "-Lane", "feature-review", "-TaskType", "feature-analysis", "-Items", "alpha", "-Format", "json"}, &out); err != nil {
		t.Fatal(err)
	}
	wave := reviewerWaveFromStatus(reviewerWaveStatus(t, caseRoot))
	observationPath := reviewerWaveStrictFile(t, wave.PacketPath, "wave-completion-race.json")
	writeReviewerWaveObservations(t, observationPath, reviewerWaveObservationFile{SchemaVersion: 1, PacketID: wave.PacketID, Observations: []reviewerWaveObservation{{ShardID: wave.SpawnWave[0].ShardID, Kind: "accepted", ReviewerHarness: "go-test-harness", ReviewerSession: "wave-session-closed"}}})
	args := []string{"-Command", "run-reviewer-wave", "-Target", caseRoot, "-Pack", "_template", "-PacketPath", wave.PacketPath, "-Lane", wave.TargetLane, "-Actor", "mission-commander", "-ReviewerWaveObservationsPath", observationPath, "-Format", "json"}
	preview := reviewerWavePreview(t, append(args, "-WhatIf")...)
	reviewerWaveBeforeApplyObservationHook = func(index int) error {
		intentPath := filepath.Join(caseRoot, ".rekit", "lanes", wave.TargetLane, "completion.intent.json")
		return os.WriteFile(intentPath, []byte("{}\n"), 0o600)
	}
	defer func() { reviewerWaveBeforeApplyObservationHook = nil }()
	out.Reset()
	err := Run(append(args, "-ExpectedReviewerWavePlanSha256", preview.ExpectedReviewerWavePlanSHA256, "-Apply"), &out)
	if err != nil {
		t.Fatalf("reviewer wave should return a typed partial failure envelope: %v\n%s", err, out.String())
	}
	var blocked reviewerWavePlan
	if err := json.Unmarshal(out.Bytes(), &blocked); err != nil {
		t.Fatal(err)
	}
	if blocked.Applied || blocked.AppliedCount != 0 || blocked.FailedIndex != 1 || !strings.Contains(blocked.Failure, "completion publication is incomplete") {
		t.Fatalf("reviewer wave did not fail closed on completion intent: %+v", blocked)
	}
	dispatchPath := filepath.Join(filepath.Dir(wave.PacketPath), "sessions", wave.SpawnWave[0].ShardID, "dispatch.json")
	if _, err := os.Stat(dispatchPath); !os.IsNotExist(err) {
		t.Fatalf("completion-raced reviewer dispatch was written: %v", err)
	}
}

func TestRunReviewerWavePartialFailurePreservesEarlierObservation(t *testing.T) {
	caseRoot := fullAttachedCase(t)
	var out bytes.Buffer
	if err := Run([]string{"-Command", "start", "-Target", caseRoot, "-Pack", "_template", "-Name", "review", "-Apply"}, &out); err != nil {
		t.Fatal(err)
	}
	out.Reset()
	if err := Run([]string{"-Command", "plan-subagents", "-Target", caseRoot, "-Pack", "_template", "-TaskType", "feature-analysis", "-Items", "alpha,beta", "-ItemsPerAgent", "1", "-MaxParallel", "2", "-Format", "json"}, &out); err != nil {
		t.Fatal(err)
	}
	wave := reviewerWaveFromStatus(reviewerWaveStatus(t, caseRoot))
	observationPath := reviewerWaveStrictFile(t, wave.PacketPath, "wave-partial.json")
	writeReviewerWaveObservations(t, observationPath, reviewerWaveObservationFile{SchemaVersion: 1, PacketID: wave.PacketID, Observations: []reviewerWaveObservation{
		{ShardID: wave.SpawnWave[0].ShardID, Kind: "accepted", ReviewerHarness: "go-test-harness", ReviewerSession: "wave-session-ok"},
		{ShardID: wave.SpawnWave[1].ShardID, Kind: "accepted", ReviewerHarness: "go-test-harness", ReviewerSession: "wave-session-late"},
	}})
	args := []string{"-Command", "run-reviewer-wave", "-Target", caseRoot, "-Pack", "_template", "-PacketPath", wave.PacketPath, "-Lane", wave.TargetLane, "-Actor", "mission-commander", "-ReviewerWaveObservationsPath", observationPath, "-Format", "json"}
	preview := reviewerWavePreview(t, append(args, "-WhatIf")...)
	reviewerWaveBeforeApplyObservationHook = func(index int) error {
		if index == 2 {
			return os.ErrPermission
		}
		return nil
	}
	defer func() { reviewerWaveBeforeApplyObservationHook = nil }()
	out.Reset()
	err := Run(append(args, "-ExpectedReviewerWavePlanSha256", preview.ExpectedReviewerWavePlanSHA256, "-Apply"), &out)
	if err != nil {
		t.Fatalf("reviewer wave partial receipt was not returned: %v\n%s", err, out.String())
	}
	var partial reviewerWavePlan
	if decodeErr := json.Unmarshal(out.Bytes(), &partial); decodeErr != nil {
		t.Fatal(decodeErr)
	}
	if !partial.Applied || partial.AppliedCount != 1 || partial.FailedIndex != 2 || partial.Failure == "" || partial.RefreshedWave == nil || partial.RefreshedWave.ActiveSlots != 1 || len(partial.RefreshedWave.Active) != 1 || partial.RefreshedWave.Active[0].ShardID != wave.SpawnWave[0].ShardID {
		t.Fatalf("reviewer wave partial receipt = %+v", partial)
	}
}

func reviewerWaveStatus(t *testing.T, caseRoot string) statusInventory {
	t.Helper()
	return reviewerWaveStatusForLane(t, caseRoot, "")
}

func reviewerWaveStatusForLane(t *testing.T, caseRoot, lane string) statusInventory {
	t.Helper()
	args := []string{"-Command", "status", "-Target", caseRoot, "-Pack", "_template"}
	if strings.TrimSpace(lane) != "" {
		args = append(args, "-Lane", lane)
	}
	args = append(args, "-Format", "json")
	var out bytes.Buffer
	if err := Run(args, &out); err != nil {
		t.Fatal(err)
	}
	var status statusInventory
	if err := json.Unmarshal(out.Bytes(), &status); err != nil {
		t.Fatal(err)
	}
	return status
}

func assertSelectedLaneReviewerWaveContracts(t *testing.T, status statusInventory, lane string) {
	t.Helper()
	if status.MissionControlRunbook == nil || status.MissionControlRunbook.CurrentLoopOperator == nil || status.MissionControlRunbook.CurrentLoopOperator.ExternalReviewerHandoff == nil {
		t.Fatalf("selected-lane status omitted current reviewer handoff: %+v", status.MissionControlRunbook)
	}
	handoff := status.MissionControlRunbook.CurrentLoopOperator.ExternalReviewerHandoff
	if handoff.Wave == nil || handoff.Wave.Lane != lane || len(handoff.Wave.Shards) == 0 {
		t.Fatalf("selected-lane status reviewer wave = %+v", handoff.Wave)
	}
	assertAttempt := func(label string, attempt *mission.CurrentLoopReviewerAttempt) {
		t.Helper()
		if attempt == nil || attempt.Identity.Lane != lane {
			t.Fatalf("%s lane binding = %+v, want %q", label, attempt, lane)
		}
		for requestLabel, request := range map[string]*mission.MissionCommanderDriverRequest{
			"current": attempt.CurrentReviewerDriverRequest,
			"durable": attempt.DurableContinuationDriverRequest,
		} {
			if request == nil {
				continue
			}
			if request.Lane != lane {
				t.Fatalf("%s %s request lane = %q, want %q", label, requestLabel, request.Lane, lane)
			}
			if request.CommandExecutable && (request.Command == "" || selectedLaneCommand(request.Command, lane) != request.Command) {
				t.Fatalf("%s %s request command lost exact lane: %q", label, requestLabel, request.Command)
			}
			if command := request.ExpectedReceipt.RefreshStatusCommand; command != "" && selectedLaneCommand(command, lane) != command {
				t.Fatalf("%s %s request refresh lost exact lane: %q", label, requestLabel, command)
			}
		}
		if attempt.RefreshStatusCommand != "" && selectedLaneCommand(attempt.RefreshStatusCommand, lane) != attempt.RefreshStatusCommand {
			t.Fatalf("%s refresh command lost exact lane: %q", label, attempt.RefreshStatusCommand)
		}
		for _, alternative := range attempt.SelectedAction.ObservationContract.Alternatives {
			if alternative.PreviewCommandTemplate == "" || selectedLaneCommand(alternative.PreviewCommandTemplate, lane) != alternative.PreviewCommandTemplate {
				t.Fatalf("%s selected action observation lost exact lane: %+v", label, alternative)
			}
		}
	}
	assertAttempt("current reviewer attempt", handoff.Attempt)
	for idx, attempt := range handoff.Wave.Shards {
		assertAttempt(fmt.Sprintf("reviewer wave shard %d", idx+1), attempt)
	}
}

func reviewerWavePreview(t *testing.T, args ...string) reviewerWavePlan {
	t.Helper()
	var out bytes.Buffer
	if err := Run(args, &out); err != nil {
		t.Fatal(err)
	}
	var plan reviewerWavePlan
	if err := json.Unmarshal(out.Bytes(), &plan); err != nil {
		t.Fatal(err)
	}
	return plan
}

func reviewerWaveStrictFile(t *testing.T, packetPath, name string) string {
	t.Helper()
	root := filepath.Join(filepath.Dir(packetPath), "results", "external-observations")
	if err := os.MkdirAll(root, 0o755); err != nil {
		t.Fatal(err)
	}
	return filepath.Join(root, name)
}

func writeReviewerWaveObservations(t *testing.T, path string, value reviewerWaveObservationFile) {
	t.Helper()
	data, err := json.Marshal(value)
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, append(data, '\n'), 0o644); err != nil {
		t.Fatal(err)
	}
}
