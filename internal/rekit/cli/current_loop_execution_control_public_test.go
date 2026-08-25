package cli

import (
	"bytes"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/shuiyu486/re-context-kits/internal/rekit/commands"
	"github.com/shuiyu486/re-context-kits/internal/rekit/executioncontrol"
	"github.com/shuiyu486/re-context-kits/internal/rekit/memberexecution"
	"github.com/shuiyu486/re-context-kits/internal/rekit/mission"
	"github.com/shuiyu486/re-context-kits/internal/rekit/projectstate"
)

func TestRunCurrentLoopCurrentMemberObservationRejectsMissingControl(t *testing.T) {
	fixture := publicEntrypointProductFixture{
		name:       "current",
		stateDir:   projectstate.CurrentDir,
		entrypoint: commands.CurrentPublicEntrypoint,
	}
	caseRoot := publicEntrypointProductCase(t, fixture, "missing-member-observation-control")
	if err := Run([]string{
		"-Command", "overview",
		"-Target", caseRoot,
		"-Pack", "_template",
		"-Format", "json",
	}, &bytes.Buffer{}); err != nil {
		t.Fatal(err)
	}
	setCurrentPublicLaneOwner(t, caseRoot, "main", "missing-control-member", 1)

	preview := runCurrentLoopPreviewWith(t, caseRoot, 2, "-Lane", "main")
	if preview.InitialCurrentStep == nil || preview.InitialCurrentStep.MemberExecution == nil {
		t.Fatalf("current project omitted member dispatch preview: %+v", preview)
	}
	memberPlan := preview.InitialCurrentStep.MemberExecution
	dispatched := runCurrentLoopApplyWith(
		t,
		caseRoot,
		preview,
		"-Lane", "main",
		"-ExpectedMemberExecutionPlanSha256", memberPlan.ExpectedPlanSHA256,
	)
	if dispatched.SegmentCheckpoint == nil || dispatched.StopReason.Code != "external-member-handoff" {
		t.Fatalf("current project did not checkpoint external member dispatch: %+v", dispatched)
	}

	observationPath := writeCurrentLoopObservation(
		t,
		caseRoot,
		"missing-member-control",
		currentLoopObservationEnvelope{
			SchemaVersion:          1,
			Kind:                   "current-loop-external-session-observation",
			CheckpointSHA256:       dispatched.SegmentCheckpoint.ArtifactSHA256,
			ObservationKind:        "member-session-accepted",
			Actor:                  "missing-control-harness",
			ObservedAt:             "2026-08-24T00:30:00Z",
			MemberAttemptID:        memberPlan.AttemptID,
			NoAuthorityOrConfirmed: true,
			NoHeavyTool:            true,
		},
	)
	claimPath, err := projectstate.Join(
		caseRoot,
		"runs",
		"current-loop-segment-claims",
		strings.ToLower(dispatched.SegmentCheckpoint.ArtifactSHA256)+".json",
	)
	if err != nil {
		t.Fatal(err)
	}
	var out bytes.Buffer
	err = Run([]string{
		"-Command", "run-current-loop",
		"-Target", caseRoot,
		"-Pack", "_template",
		"-Lane", "main",
		"-ResumeCurrentLoop",
		"-ExpectedCurrentLoopCheckpointSha256", dispatched.SegmentCheckpoint.ArtifactSHA256,
		"-CurrentLoopObservationPath", observationPath,
		"-WhatIf",
		"-Format", "json",
	}, &out)
	if err == nil || !strings.Contains(err.Error(), "does not match checkpoint birth lineage") {
		t.Fatalf("missing member observation control error = %v\n%s", err, out.String())
	}
	assertFileNotExists(t, claimPath)
	inspection, inspectErr := memberexecution.Inspect(caseRoot, "main", memberPlan.AttemptID)
	if inspectErr != nil || inspection.State != "handoff-ready" {
		t.Fatalf("missing-control observation advanced member attempt: inspection=%+v err=%v", inspection, inspectErr)
	}
}

func TestRunCurrentLoopCurrentMemberObservationRejectsStaleFrozenControl(t *testing.T) {
	fixture := publicEntrypointProductFixture{
		name:       "current",
		stateDir:   projectstate.CurrentDir,
		entrypoint: commands.CurrentPublicEntrypoint,
	}
	caseRoot := publicEntrypointProductCase(t, fixture, "stale-member-observation")
	if err := Run([]string{
		"-Command", "overview",
		"-Target", caseRoot,
		"-Pack", "_template",
		"-Format", "json",
	}, &bytes.Buffer{}); err != nil {
		t.Fatal(err)
	}
	setCurrentPublicLaneOwner(t, caseRoot, "main", "stale-member", 1)

	preview := runCurrentLoopPreviewWith(t, caseRoot, 2, "-Lane", "main")
	if preview.InitialCurrentStep == nil || preview.InitialCurrentStep.MemberExecution == nil {
		t.Fatalf("current project omitted member dispatch preview: %+v", preview)
	}
	memberPlan := preview.InitialCurrentStep.MemberExecution
	dispatched := runCurrentLoopApplyWith(
		t,
		caseRoot,
		preview,
		"-Lane", "main",
		"-ExpectedMemberExecutionPlanSha256", memberPlan.ExpectedPlanSHA256,
	)
	if !dispatched.Applied || dispatched.SegmentCheckpoint == nil ||
		dispatched.StopReason.Code != "external-member-handoff" {
		t.Fatalf("current project did not checkpoint external member dispatch: %+v", dispatched)
	}

	_, status := runPublicEntrypointProductStatus(t, caseRoot, "main")
	operator := status.MissionControlRunbook.CurrentLoopOperator
	job := recordCurrentLoopExternalSessionAttempt(
		t,
		operator,
		"stale-harness",
		"stale-session",
		"mission-commander",
		"2026-08-24T01:00:00Z",
	)
	if job.CurrentAttempt == nil || job.CurrentAttempt.LaunchControl == nil {
		t.Fatalf("current attempt omitted frozen launch control: %+v", job)
	}
	frozen := executioncontrol.CloneBinding(job.CurrentAttempt.LaunchControl)

	paused := applyPublicExecutionControl(t, caseRoot, "main", executioncontrol.ActionPause)
	if paused.State != executioncontrol.StatePaused || paused.ControlGeneration != 1 {
		t.Fatalf("public pause did not commit generation one: %+v", paused)
	}
	resumed := applyPublicExecutionControl(t, caseRoot, "main", executioncontrol.ActionResume)
	if resumed.State != executioncontrol.StateRunning || resumed.ControlGeneration != 2 {
		t.Fatalf("public resume did not commit generation two: %+v", resumed)
	}
	if current, err := executioncontrol.Inspect(caseRoot, "main"); err != nil ||
		current.CurrentGeneration == frozen.ControlGeneration {
		t.Fatalf("control did not advance beyond frozen attempt: current=%+v frozen=%+v err=%v", current, frozen, err)
	}

	observationPath := writeCurrentLoopObservation(
		t,
		caseRoot,
		"stale-member-accepted",
		currentLoopObservationEnvelope{
			SchemaVersion:          1,
			Kind:                   "current-loop-external-session-observation",
			CheckpointSHA256:       dispatched.SegmentCheckpoint.ArtifactSHA256,
			ObservationKind:        "member-session-accepted",
			Actor:                  "stale-harness",
			ObservedAt:             "2026-08-24T01:00:30Z",
			MemberAttemptID:        memberPlan.AttemptID,
			LaunchControl:          frozen,
			NoAuthorityOrConfirmed: true,
			NoHeavyTool:            true,
		},
	)
	claimPath, err := projectstate.Join(
		caseRoot,
		"runs",
		"current-loop-segment-claims",
		strings.ToLower(dispatched.SegmentCheckpoint.ArtifactSHA256)+".json",
	)
	if err != nil {
		t.Fatal(err)
	}
	observationPathState, err := projectstate.Join(
		caseRoot,
		"lanes",
		"main",
		"member-executions",
		memberPlan.AttemptID,
		"observations",
	)
	if err != nil {
		t.Fatal(err)
	}
	segmentRoot, err := projectstate.Join(caseRoot, "runs", "current-loop-segments")
	if err != nil {
		t.Fatal(err)
	}
	segmentsBefore := snapshotFiles(t, segmentRoot)

	var out bytes.Buffer
	err = Run([]string{
		"-Command", "run-current-loop",
		"-Target", caseRoot,
		"-Pack", "_template",
		"-Lane", "main",
		"-ResumeCurrentLoop",
		"-ExpectedCurrentLoopCheckpointSha256", dispatched.SegmentCheckpoint.ArtifactSHA256,
		"-CurrentLoopObservationPath", observationPath,
		"-WhatIf",
		"-Format", "json",
	}, &out)
	if err != nil {
		t.Fatalf("stale member observation preview returned command error = %v\n%s", err, out.String())
	}
	var blocked currentLoopTestPlan
	if err := json.Unmarshal(out.Bytes(), &blocked); err != nil {
		t.Fatalf("stale member observation preview JSON did not decode: %v\n%s", err, out.String())
	}
	if blocked.ExpectedCurrentLoopPlanSHA256 != "" || blocked.InitialCurrentStep != nil ||
		blocked.StopReason.Code != "requires-review" ||
		!strings.Contains(blocked.StopReason.Message, "execution control binding disposition is stale-control-generation") {
		t.Fatalf("stale member observation preview remained executable: %+v", blocked)
	}
	assertFileNotExists(t, claimPath)
	assertFileNotExists(t, observationPathState)
	assertSnapshotEqual(t, segmentsBefore, snapshotFiles(t, segmentRoot))
	inspection, inspectErr := memberexecution.Inspect(caseRoot, "main", memberPlan.AttemptID)
	if inspectErr != nil || inspection.State != "handoff-ready" {
		t.Fatalf("stale observation advanced member attempt: inspection=%+v err=%v", inspection, inspectErr)
	}
}

func TestCurrentMemberStatusAfterPauseResumeDoesNotExposeExecutableHandoff(t *testing.T) {
	fixture := publicEntrypointProductFixture{
		name:       "current",
		stateDir:   projectstate.CurrentDir,
		entrypoint: commands.CurrentPublicEntrypoint,
	}
	caseRoot := publicEntrypointProductCase(t, fixture, "stale-member-status")
	if err := Run([]string{
		"-Command", "overview", "-Target", caseRoot, "-Pack", "_template", "-Format", "json",
	}, &bytes.Buffer{}); err != nil {
		t.Fatal(err)
	}
	setCurrentPublicLaneOwner(t, caseRoot, "main", "stale-status-member", 1)
	preview := runCurrentLoopPreviewWith(t, caseRoot, 2, "-Lane", "main")
	if preview.InitialCurrentStep == nil || preview.InitialCurrentStep.MemberExecution == nil {
		t.Fatalf("current project omitted member dispatch preview: %+v", preview)
	}
	memberPlan := preview.InitialCurrentStep.MemberExecution
	dispatched := runCurrentLoopApplyWith(
		t, caseRoot, preview, "-Lane", "main",
		"-ExpectedMemberExecutionPlanSha256", memberPlan.ExpectedPlanSHA256,
	)
	if dispatched.SegmentCheckpoint == nil || dispatched.StopReason.Code != "external-member-handoff" {
		t.Fatalf("current project did not checkpoint external member dispatch: %+v", dispatched)
	}
	applyPublicExecutionControl(t, caseRoot, "main", executioncontrol.ActionPause)
	applyPublicExecutionControl(t, caseRoot, "main", executioncontrol.ActionResume)

	_, status := runPublicEntrypointProductStatus(t, caseRoot, "main")
	if status.MemberExecution == nil || status.MemberExecution.Ready ||
		status.MemberExecution.State != "diagnostic-stale-execution-control" ||
		status.MemberExecution.ObservationCommand != "" ||
		status.MemberExecution.PreviewCommand != "" {
		t.Fatalf("stale current member status remained executable: %+v", status.MemberExecution)
	}
	operator := status.MissionControlRunbook.CurrentLoopOperator
	if operator == nil || operator.Ready || operator.State != "checkpoint-stale-member-control" ||
		operator.SelectedDriverRequest != nil || operator.StartDriverRequest != nil ||
		operator.ResumeDriverRequest != nil || operator.ExternalMemberHandoff != nil ||
		operator.ExternalSessionJob != nil {
		t.Fatalf("stale current checkpoint exposed executable member progression: %+v", operator)
	}
	for label, request := range map[string]*mission.MissionCommanderDriverRequest{
		"top-level": status.MissionControlRunbook.CurrentDriverRequest,
		"source":    operator.SourceCurrentDriverRequest,
	} {
		if request == nil || !request.CommandExecutable || request.Blocked ||
			request.Invocation == nil || driverStepCommandName(request.Command) != commands.Continue {
			t.Fatalf("stale checkpoint suppressed %s fresh current request: %+v", label, request)
		}
	}
	segment := status.MissionControlRunbook.CurrentLoopSegment
	if segment == nil || segment.ResumeDriverRequest != nil ||
		segment.LegacyUnboundWhatIfCommand != "" ||
		segment.Continuation == nil ||
		segment.Continuation.ObservationContract != nil ||
		segment.Continuation.ExternalMemberHandoff != nil {
		t.Fatalf("stale current checkpoint exposed executable segment carriers: %+v", segment)
	}
	if operator.ObservationInbox != nil && operator.ObservationInbox.SelectedDriverRequest != nil {
		t.Fatalf("stale current checkpoint exposed observation inbox request: %+v", operator.ObservationInbox)
	}
	if !status.MissionControlRunbook.Ready || status.MissionControlRunbook.CurrentCommand == "" ||
		status.MissionControlRunbook.ReplacementExecutorTakeover == nil ||
		status.MissionControlRunbook.Quickstart == nil ||
		!status.MissionControlRunbook.Quickstart.CommandExecutable ||
		status.MissionControlRunbook.Quickstart.Blocked {
		t.Fatalf("stale checkpoint suppressed fresh runbook request: %+v", status.MissionControlRunbook)
	}
}

func TestRunCurrentStepControlBoundResultTurnHoldsAfterPause(t *testing.T) {
	result := newControlBoundExternalResultTurn(t, "control-bound-result-turn")
	caseRoot := result.caseRoot
	job := result.job
	current := result.current
	claimPath := result.claimPath
	assertControlBoundRelayArtifactsAbsent(t, caseRoot, job, claimPath)

	paused := applyPublicExecutionControl(t, caseRoot, "main", executioncontrol.ActionPause)
	if paused.State != executioncontrol.StatePaused || paused.ControlGeneration != 1 {
		t.Fatalf("public pause did not commit generation one: %+v", paused)
	}
	statusData, pausedStatus := runPublicEntrypointProductStatus(t, caseRoot, "main")
	assertPublicEntrypointProductStatus(t, statusData, commands.CurrentPublicEntrypoint)
	if pausedStatus.MissionControlRunbook == nil ||
		pausedStatus.MissionControlRunbook.CurrentLoopSegment != nil ||
		pausedStatus.MissionControlRunbook.CurrentLoopOperator != nil ||
		pausedStatus.MissionControlRunbook.ReplacementExecutorTakeover != nil {
		t.Fatalf("public paused status exposed a live current-loop route: %+v", pausedStatus.MissionControlRunbook)
	}

	applyArgs := []string{
		"-Command", "run-current-step",
		"-Target", caseRoot,
		"-Pack", "_template",
		"-Lane", "main",
		"-ExpectedCurrentStepPlanSha256", current.ExpectedCurrentStepPlanSHA256,
		"-Apply",
		"-Format", "json",
	}
	applied := runControlBoundCurrentStepApply(t, applyArgs)
	held := requireHeldCurrentStepResult(t, applied, executioncontrol.ResultDispositionHeldWhilePaused)
	assertControlBoundRelayArtifactsAbsent(t, caseRoot, job, claimPath)
	inspection, ok, err := memberexecution.Latest(caseRoot, "main")
	if err != nil || !ok || inspection.State != "handoff-ready" {
		t.Fatalf("held relay advanced nested member state: inspection=%+v ok=%t err=%v", inspection, ok, err)
	}

	heldPath := filepath.Join(caseRoot, filepath.FromSlash(held.ReceiptPath))
	heldData, err := os.ReadFile(heldPath)
	if err != nil {
		t.Fatal(err)
	}
	if got := sha256Text(heldData); !strings.EqualFold(got, held.ReceiptSHA256) {
		t.Fatalf("held receipt sha256 = %s, want %s", got, held.ReceiptSHA256)
	}
	assertSingleHeldReceipt(t, heldPath)
	if got, err := os.ReadFile(result.submissionPath); err != nil || !bytes.Equal(got, result.submissionData) {
		t.Fatalf("held classifier changed raw submission: equal=%t err=%v", bytes.Equal(got, result.submissionData), err)
	}

	resumed := applyPublicExecutionControl(t, caseRoot, "main", executioncontrol.ActionResume)
	if resumed.State != executioncontrol.StateRunning || resumed.ControlGeneration != 2 {
		t.Fatalf("public resume did not commit generation two: %+v", resumed)
	}
	assertControlBoundRelayArtifactsAbsent(t, caseRoot, job, claimPath)

	replayed := runControlBoundCurrentStepApply(t, applyArgs)
	replayedHeld := requireHeldCurrentStepResult(t, replayed, executioncontrol.ResultDispositionHeldWhilePaused)
	if replayedHeld.ReceiptPath != held.ReceiptPath || replayedHeld.ReceiptSHA256 != held.ReceiptSHA256 {
		t.Fatalf("resume changed sticky held receipt: first=%+v replay=%+v", held, replayedHeld)
	}
	replayedData, err := os.ReadFile(heldPath)
	if err != nil || !bytes.Equal(replayedData, heldData) {
		t.Fatalf("resume rewrote sticky held receipt: equal=%t err=%v", bytes.Equal(replayedData, heldData), err)
	}
	assertSingleHeldReceipt(t, heldPath)
	assertControlBoundRelayArtifactsAbsent(t, caseRoot, job, claimPath)
}

func TestRunCurrentStepControlBoundResultTurnRejectsPauseBeforeCheckpointClaim(t *testing.T) {
	result := newControlBoundExternalResultTurn(t, "control-bound-claim-race")
	segmentRoot, err := projectstate.Join(result.caseRoot, "runs", "current-loop-segments")
	if err != nil {
		t.Fatal(err)
	}
	segmentsBefore := snapshotFiles(t, segmentRoot)

	currentLoopExternalTurnBeforeClaimHook = func() error {
		paused := applyPublicExecutionControl(t, result.caseRoot, "main", executioncontrol.ActionPause)
		if paused.State != executioncontrol.StatePaused || paused.ControlGeneration != 1 {
			t.Fatalf("pause-before-claim did not commit generation one: %+v", paused)
		}
		return nil
	}
	t.Cleanup(func() { currentLoopExternalTurnBeforeClaimHook = nil })

	var out bytes.Buffer
	err = Run(currentStepApplyArgs(result.caseRoot, result.current), &out)
	if err == nil || !strings.Contains(err.Error(), "execution control binding disposition is held-while-paused") {
		t.Fatalf("pause-before-claim error = %v\n%s", err, out.String())
	}
	currentLoopExternalTurnBeforeClaimHook = nil

	var partial currentStepTestPlan
	if decodeErr := json.Unmarshal(out.Bytes(), &partial); decodeErr != nil {
		t.Fatalf("pause-before-claim partial JSON did not decode: %v\n%s", decodeErr, out.String())
	}
	if !partial.Applied || partial.Receipt == nil || partial.Receipt.State != "nested-partial" ||
		!strings.Contains(partial.Receipt.Outcome, "checkpoint-claim") ||
		partial.ExternalSessionStep == nil || partial.ExternalSessionStep.Turn == nil {
		t.Fatalf("pause-before-claim omitted committed relay partial truth: %+v", partial)
	}
	turn := partial.ExternalSessionStep.Turn
	publication := turn.Relay.ResultPublication
	if turn.FailureStage != "checkpoint-claim" || !turn.Relay.Applied ||
		turn.Resume.Applied || turn.Resume.ObservationReceipt != nil ||
		publication == nil || !publication.Published || publication.Held ||
		publication.Disposition != executioncontrol.ResultDispositionPublished {
		t.Fatalf("pause-before-claim crossed the progression fence: %+v", turn)
	}

	for _, path := range []string{
		filepath.Join(result.caseRoot, filepath.FromSlash(result.job.MemberManifestPath)),
		filepath.Join(result.caseRoot, filepath.FromSlash(result.job.MemberOutputsRoot), "result.txt"),
		filepath.Join(result.caseRoot, filepath.FromSlash(result.job.PublicationPath)),
		filepath.Join(result.caseRoot, filepath.FromSlash(result.job.ObservationPath)),
	} {
		if data, readErr := os.ReadFile(path); readErr != nil || len(data) == 0 {
			t.Fatalf("committed relay artifact %s was not preserved: bytes=%d err=%v", path, len(data), readErr)
		}
	}
	assertFileNotExists(t, result.claimPath)
	assertSnapshotEqual(t, segmentsBefore, snapshotFiles(t, segmentRoot))
	inspection, ok, inspectErr := memberexecution.Latest(result.caseRoot, "main")
	if inspectErr != nil || !ok || inspection.State != "handoff-ready" {
		t.Fatalf("pause-before-claim advanced nested member state: inspection=%+v ok=%t err=%v", inspection, ok, inspectErr)
	}
	heldRoot, err := projectstate.Join(result.caseRoot, "lanes", "main", "execution-control", "held-results")
	if err != nil {
		t.Fatal(err)
	}
	assertFileNotExists(t, heldRoot)

	_, status := runPublicEntrypointProductStatus(t, result.caseRoot, "main")
	if status.MissionControlRunbook == nil || status.MissionControlRunbook.CurrentLoopSegment != nil ||
		status.MissionControlRunbook.CurrentLoopOperator != nil ||
		status.MissionControlRunbook.ReplacementExecutorTakeover != nil {
		t.Fatalf("pause-before-claim public status exposed live progression: %+v", status.MissionControlRunbook)
	}
}

func TestRunCurrentStepControlBoundResultTurnRejectsPauseAfterCheckpointClaim(t *testing.T) {
	result := newControlBoundExternalResultTurn(t, "control-bound-post-claim-race")
	segmentRoot, err := projectstate.Join(result.caseRoot, "runs", "current-loop-segments")
	if err != nil {
		t.Fatal(err)
	}
	segmentsBefore := snapshotFiles(t, segmentRoot)

	currentLoopExternalTurnAfterClaimHook = func() error {
		paused := applyPublicExecutionControl(t, result.caseRoot, "main", executioncontrol.ActionPause)
		if paused.State != executioncontrol.StatePaused || paused.ControlGeneration != 1 {
			t.Fatalf("pause-after-claim did not commit generation one: %+v", paused)
		}
		return nil
	}
	t.Cleanup(func() { currentLoopExternalTurnAfterClaimHook = nil })

	var out bytes.Buffer
	err = Run(currentStepApplyArgs(result.caseRoot, result.current), &out)
	currentLoopExternalTurnAfterClaimHook = nil
	if err == nil || !strings.Contains(err.Error(), "selected current lane \"main\" current driver request is blocked or not executable") {
		t.Fatalf("pause-after-claim refresh error = %v\n%s", err, out.String())
	}
	var applied currentStepTestPlan
	if decodeErr := json.Unmarshal(out.Bytes(), &applied); decodeErr != nil {
		t.Fatalf("pause-after-claim partial JSON did not decode: %v\n%s", decodeErr, out.String())
	}
	if !applied.Applied || applied.Receipt == nil || applied.Receipt.State != "refresh-failed" ||
		applied.ExternalSessionStep == nil || applied.ExternalSessionStep.Turn == nil {
		t.Fatalf("pause-after-claim lost committed relay truth: %+v", applied)
	}
	turn := applied.ExternalSessionStep.Turn
	publication := turn.Relay.ResultPublication
	if !turn.Relay.Applied || publication == nil || !publication.Published || publication.Held ||
		publication.Disposition != executioncontrol.ResultDispositionPublished {
		t.Fatalf("pause-after-claim changed the committed relay: %+v", turn)
	}
	resume := turn.Resume
	if resume.Applied || resume.AppliedSteps != 0 || len(resume.Steps) != 0 ||
		resume.StopReason.Code != "error" || resume.StopReason.Phase != "apply-step" ||
		!strings.Contains(resume.StopReason.Message, "execution control binding disposition is held-while-paused") ||
		resume.SegmentCheckpoint != nil || resume.ObservationReceipt != nil {
		t.Fatalf("pause-after-claim crossed the nested progression fence: %+v", resume)
	}

	claimData, err := os.ReadFile(result.claimPath)
	if err != nil || len(claimData) == 0 {
		t.Fatalf("pause-after-claim did not preserve the committed checkpoint claim: bytes=%d err=%v", len(claimData), err)
	}
	assertSnapshotEqual(t, segmentsBefore, snapshotFiles(t, segmentRoot))
	inspection, ok, inspectErr := memberexecution.Latest(result.caseRoot, "main")
	if inspectErr != nil || !ok || inspection.State != "handoff-ready" {
		t.Fatalf("pause-after-claim advanced nested member state: inspection=%+v ok=%t err=%v", inspection, ok, inspectErr)
	}
	heldRoot, err := projectstate.Join(result.caseRoot, "lanes", "main", "execution-control", "held-results")
	if err != nil {
		t.Fatal(err)
	}
	assertFileNotExists(t, heldRoot)

	_, status := runPublicEntrypointProductStatus(t, result.caseRoot, "main")
	if status.MissionControlRunbook == nil || status.MissionControlRunbook.CurrentLoopSegment != nil ||
		status.MissionControlRunbook.CurrentLoopOperator != nil ||
		status.MissionControlRunbook.ReplacementExecutorTakeover != nil {
		t.Fatalf("pause-after-claim public status exposed live progression: %+v", status.MissionControlRunbook)
	}
}

type controlBoundExternalResultTurn struct {
	caseRoot       string
	job            *mission.CurrentLoopExternalSessionJob
	current        currentStepTestPlan
	claimPath      string
	submissionPath string
	submissionData []byte
}

func newControlBoundExternalResultTurn(t *testing.T, projectName string) controlBoundExternalResultTurn {
	t.Helper()
	fixture := publicEntrypointProductFixture{
		name:       "current",
		stateDir:   projectstate.CurrentDir,
		entrypoint: commands.CurrentPublicEntrypoint,
	}
	caseRoot := publicEntrypointProductCase(t, fixture, projectName)
	if err := Run([]string{
		"-Command", "overview",
		"-Target", caseRoot,
		"-Pack", "_template",
		"-Format", "json",
	}, &bytes.Buffer{}); err != nil {
		t.Fatal(err)
	}
	setCurrentPublicLaneOwner(t, caseRoot, "main", "control-bound-member", 1)

	preview := runCurrentLoopPreviewWith(t, caseRoot, 2, "-Lane", "main")
	if preview.InitialCurrentStep == nil || preview.InitialCurrentStep.MemberExecution == nil {
		t.Fatalf("current project omitted member dispatch preview: %+v", preview)
	}
	memberPlan := preview.InitialCurrentStep.MemberExecution
	dispatched := runCurrentLoopApplyWith(
		t,
		caseRoot,
		preview,
		"-Lane", "main",
		"-ExpectedMemberExecutionPlanSha256", memberPlan.ExpectedPlanSHA256,
	)
	if !dispatched.Applied || dispatched.SegmentCheckpoint == nil || dispatched.StopReason.Code != "external-member-handoff" {
		t.Fatalf("current project did not checkpoint external member dispatch: %+v", dispatched)
	}

	_, status := runPublicEntrypointProductStatus(t, caseRoot, "main")
	operator := status.MissionControlRunbook.CurrentLoopOperator
	job := recordCurrentLoopExternalSessionAttempt(
		t,
		operator,
		"control-bound-harness",
		"control-bound-session",
		"mission-commander",
		"2026-08-19T01:00:00Z",
	)
	if job.CurrentAttempt == nil || job.CurrentAttempt.LaunchControl == nil {
		t.Fatalf("current attempt omitted launch execution control: %+v", job)
	}
	operator.ExternalSessionJob = job
	job = acceptCurrentLoopExternalSessionLaunch(
		t,
		operator,
		"dispatcher",
		"control-bound-harness",
		"control-bound-session",
		"2026-08-19T01:00:01Z",
	)
	if job.CurrentAttempt == nil || job.CurrentAttempt.LaunchControl == nil {
		t.Fatalf("accepted current launch omitted execution control lineage: %+v", job)
	}

	jobRoot := filepath.Join(caseRoot, filepath.Dir(filepath.FromSlash(job.SubmissionPath)))
	if err := os.MkdirAll(filepath.Join(jobRoot, "outputs"), 0o700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(jobRoot, "outputs", "result.txt"), []byte("control-bound raw result\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	submission := map[string]any{
		"schemaVersion":          1,
		"kind":                   "current-loop-external-session-submission",
		"jobId":                  job.JobID,
		"jobSha256":              job.JobSHA256,
		"outcome":                "returned",
		"actor":                  "control-bound-harness",
		"observedAt":             "2026-08-19T01:00:30Z",
		"summary":                "control-bound result returned",
		"noAuthorityOrConfirmed": true,
		"noHeavyTool":            true,
	}
	bindCurrentLoopExternalSubmissionAttempt(t, job, submission)
	submissionData, err := json.MarshalIndent(submission, "", "  ")
	if err != nil {
		t.Fatal(err)
	}
	submissionData = append(submissionData, '\n')
	submissionPath := filepath.Join(caseRoot, filepath.FromSlash(job.SubmissionPath))
	if err := os.WriteFile(submissionPath, submissionData, 0o600); err != nil {
		t.Fatal(err)
	}

	current := runMemberCurrentStepForLane(t, caseRoot, "main", []string{"-WhatIf"})
	if current.ExternalSessionStep == nil || current.ExternalSessionStep.Mode != "result-turn" ||
		current.ExternalSessionStep.Turn == nil || current.ExpectedCurrentStepPlanSHA256 == "" {
		t.Fatalf("current project omitted public external result turn: %+v", current)
	}
	turn := current.ExternalSessionStep.Turn
	if turn.Relay.Submission.LaunchControl == nil ||
		!executioncontrol.SameBinding(turn.Relay.Submission.LaunchControl, job.CurrentAttempt.LaunchControl) {
		t.Fatalf("result turn did not preserve exact launch control lineage: turn=%+v attempt=%+v", turn.Relay.Submission.LaunchControl, job.CurrentAttempt.LaunchControl)
	}

	claimPath, err := projectstate.Join(
		caseRoot,
		"runs",
		"current-loop-segment-claims",
		strings.ToLower(dispatched.SegmentCheckpoint.ArtifactSHA256)+".json",
	)
	if err != nil {
		t.Fatal(err)
	}
	return controlBoundExternalResultTurn{
		caseRoot:       caseRoot,
		job:            job,
		current:        current,
		claimPath:      claimPath,
		submissionPath: submissionPath,
		submissionData: submissionData,
	}
}

func currentStepApplyArgs(caseRoot string, current currentStepTestPlan) []string {
	return []string{
		"-Command", "run-current-step",
		"-Target", caseRoot,
		"-Pack", "_template",
		"-Lane", "main",
		"-ExpectedCurrentStepPlanSha256", current.ExpectedCurrentStepPlanSHA256,
		"-Apply",
		"-Format", "json",
	}
}

func setCurrentPublicLaneOwner(t *testing.T, caseRoot, lane, executor string, generation int) {
	t.Helper()
	board, err := mission.ReadBoard(caseRoot)
	if err != nil {
		t.Fatal(err)
	}
	found := false
	for index := range board.Lanes {
		if board.Lanes[index].ID != lane {
			continue
		}
		board.Lanes[index].CurrentExecutor = executor
		board.Lanes[index].ExecutorGeneration = generation
		board.Lanes[index].UpdatedAt = "2026-08-19T00:59:00Z"
		found = true
	}
	if !found {
		t.Fatalf("board omitted lane %s", lane)
	}
	boardData, err := json.MarshalIndent(board, "", "  ")
	if err != nil {
		t.Fatal(err)
	}
	boardPath, err := projectstate.Join(caseRoot, "board.json")
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(boardPath, append(boardData, '\n'), 0o644); err != nil {
		t.Fatal(err)
	}

	lanePath, err := projectstate.Join(caseRoot, "lanes", lane, "lane.json")
	if err != nil {
		t.Fatal(err)
	}
	laneData, err := os.ReadFile(lanePath)
	if err != nil {
		t.Fatal(err)
	}
	var laneState map[string]any
	if err := json.Unmarshal(laneData, &laneState); err != nil {
		t.Fatal(err)
	}
	laneState["currentExecutor"] = executor
	laneState["executorGeneration"] = generation
	laneState["updatedAt"] = "2026-08-19T00:59:00Z"
	laneData, err = json.MarshalIndent(laneState, "", "  ")
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(lanePath, append(laneData, '\n'), 0o644); err != nil {
		t.Fatal(err)
	}
}

func applyPublicExecutionControl(t *testing.T, caseRoot, lane, action string) executioncontrol.Plan {
	t.Helper()
	var out bytes.Buffer
	if err := Run([]string{
		"-Command", commands.Control,
		"-Target", caseRoot,
		"-Lane", lane,
		"-Action", action,
		"-Actor", "mission-commander",
		"-Reason", "public execution control regression",
		"-WhatIf",
		"-Format", "json",
	}, &out); err != nil {
		t.Fatal(err)
	}
	var preview executioncontrol.Plan
	decodeJSONStrict(t, out.Bytes(), &preview)
	if preview.ApplyCommand == "" || preview.ExpectedPlanSHA256 == "" {
		t.Fatalf("public control preview omitted exact Apply identity: %+v", preview)
	}

	out.Reset()
	applyArgs := rekitCommandCLIArgs(t, preview.ApplyCommand)
	applyArgs = append(applyArgs, "-Target", caseRoot)
	if err := Run(applyArgs, &out); err != nil {
		t.Fatal(err)
	}
	var applied executioncontrol.Plan
	decodeJSONStrict(t, out.Bytes(), &applied)
	if !applied.Applied || applied.AlreadyApplied {
		t.Fatalf("public control action was not freshly committed: %+v", applied)
	}
	return applied
}

func runControlBoundCurrentStepApply(t *testing.T, args []string) currentStepTestPlan {
	t.Helper()
	var out bytes.Buffer
	if err := Run(args, &out); err != nil {
		t.Fatalf("control-bound current-step Apply failed: %v\n%s", err, out.String())
	}
	var applied currentStepTestPlan
	if err := json.Unmarshal(out.Bytes(), &applied); err != nil {
		t.Fatalf("control-bound current-step JSON did not decode: %v\n%s", err, out.String())
	}
	return applied
}

func requireHeldCurrentStepResult(t *testing.T, applied currentStepTestPlan, disposition string) executioncontrol.ResultPublication {
	t.Helper()
	if applied.Applied || applied.Receipt != nil || applied.RefreshedStatus != nil ||
		applied.ExternalSessionStep == nil || applied.ExternalSessionStep.Turn == nil {
		t.Fatalf("held result turn exposed live outer progression: %+v", applied)
	}
	turn := applied.ExternalSessionStep.Turn
	publication := turn.Relay.ResultPublication
	if turn.FailureStage != "relay-held" || turn.Applied || turn.Resume.Applied ||
		turn.Resume.ObservationReceipt != nil || publication == nil || !publication.Held ||
		publication.Published || publication.Disposition != disposition ||
		publication.ReceiptPath == "" || publication.ReceiptSHA256 == "" {
		t.Fatalf("result turn did not return exact typed held publication: %+v", turn)
	}
	return *publication
}

func assertControlBoundRelayArtifactsAbsent(t *testing.T, caseRoot string, job *mission.CurrentLoopExternalSessionJob, claimPath string) {
	t.Helper()
	for _, path := range []string{
		filepath.Join(caseRoot, filepath.FromSlash(job.MemberManifestPath)),
		filepath.Join(caseRoot, filepath.FromSlash(job.MemberOutputsRoot), "result.txt"),
		filepath.Join(caseRoot, filepath.FromSlash(job.PublicationPath)),
		filepath.Join(caseRoot, filepath.FromSlash(job.ObservationPath)),
		claimPath,
	} {
		assertFileNotExists(t, path)
	}
}

func assertSingleHeldReceipt(t *testing.T, receiptPath string) {
	t.Helper()
	entries, err := os.ReadDir(filepath.Dir(receiptPath))
	if err != nil {
		t.Fatal(err)
	}
	if len(entries) != 1 || entries[0].IsDir() || entries[0].Name() != filepath.Base(receiptPath) {
		t.Fatalf("held receipt namespace = %+v, want one sticky receipt %s", entries, filepath.Base(receiptPath))
	}
}
