package cli

import (
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"reflect"
	"runtime"
	"strings"
	"testing"

	"github.com/shuiyu486/re-context-kits/internal/rekit/commands"
	"github.com/shuiyu486/re-context-kits/internal/rekit/currentloop"
	"github.com/shuiyu486/re-context-kits/internal/rekit/defaults"
	"github.com/shuiyu486/re-context-kits/internal/rekit/memberexecution"
	"github.com/shuiyu486/re-context-kits/internal/rekit/mission"
	rekitruntime "github.com/shuiyu486/re-context-kits/internal/rekit/runtime"
	"github.com/shuiyu486/re-context-kits/internal/rekit/workstream"
)

func TestRunCurrentStepRoutesLaneThenReviewerLifecycle(t *testing.T) {
	caseRoot := fullAttachedCase(t)
	var out bytes.Buffer
	if err := Run([]string{"-Command", "overview", "-Target", caseRoot, "-Pack", "_template", "-Format", "json"}, &out); err != nil {
		t.Fatal(err)
	}
	base := []string{"-Command", "run-current-step", "-Target", caseRoot, "-Pack", "_template", "-Lane", "main", "-WhatIf", "-Format", "json"}

	out.Reset()
	err := Run([]string{"-Command", "run-current-step", "-Target", caseRoot, "-Pack", "_template", "-ReviewerHarness", "unexpected", "-WhatIf", "-Format", "json"}, &out)
	if err == nil || !strings.Contains(err.Error(), "case route does not accept reviewer observation inputs") {
		t.Fatalf("case route accepted reviewer inputs: %v", err)
	}
	out.Reset()
	err = Run([]string{"-Command", "run-current-step", "-Target", caseRoot, "-Pack", "_template", "-ExpectedCurrentStepPlanSha256", strings.Repeat("0", 64), "-Apply", "-Format", "json"}, &out)
	if err == nil || !strings.Contains(err.Error(), "expected plan sha256 mismatch") || out.Len() != 0 {
		t.Fatalf("stale zero-progress current-step plan was not rejected without JSON: err=%v stdout=%q", err, out.String())
	}
	out.Reset()
	err = Run([]string{"-Command", "run-current-step", "-Target", caseRoot, "-Pack", "_template", "-Lane", "missing-lane", "-WhatIf", "-Format", "json"}, &out)
	if err == nil || !strings.Contains(err.Error(), "is not current") || out.Len() != 0 {
		t.Fatalf("unknown selected current lane was not rejected without output: err=%v stdout=%q", err, out.String())
	}

	lanePreview := runCurrentStepPreview(t, base)
	if lanePreview.Route != "case" || lanePreview.DriverStep == nil || lanePreview.ReviewerStep != nil || lanePreview.ExpectedCurrentStepPlanSHA256 == "" {
		t.Fatalf("current-step did not route initial lane start: %+v", lanePreview)
	}
	laneApply := runCurrentStepApply(t, caseRoot, lanePreview)
	if !laneApply.Applied || laneApply.Receipt == nil || laneApply.Receipt.Route != "case" || laneApply.Receipt.NestedCommand != "run-driver-step" {
		t.Fatalf("current-step lane apply omitted nested runner receipt: %+v", laneApply)
	}

	out.Reset()
	if err := Run([]string{"-Command", "plan-subagents", "-Target", caseRoot, "-Pack", "_template", "-TaskType", "feature-analysis", "-Items", "alpha"}, &out); err != nil {
		t.Fatal(err)
	}
	plan := decodePlanSubagentsResult(t, out.Bytes())
	packet := decodePlanSubagentsPacket(t, plan.PacketPath)
	handoff := plan.ShardHandoffs[0]
	actor := "mission-commander"

	spawn := runCurrentStepPreview(t, base)
	if spawn.Route != "reviewer" || spawn.ReviewerStep == nil || spawn.ReviewerStep.ExternalHandoff == nil || spawn.ReviewerStep.ExternalHandoff.RunLoopStepID != "spawn-reviewer" || spawn.ExpectedCurrentStepPlanSHA256 != "" {
		t.Fatalf("current-step did not route reviewer spawn handoff: %+v", spawn)
	}
	dispatchArgs := append(append([]string{}, base...), "-ReviewerHarness", "current-step-harness", "-ReviewerSession", "current-step-reviewer", "-Actor", actor)
	dispatch := runCurrentStepPreview(t, dispatchArgs)
	if dispatch.Route != "reviewer" || dispatch.ReviewerStep == nil || dispatch.ReviewerStep.ApplyDriverRequest == nil || dispatch.ExpectedCurrentStepPlanSHA256 == "" {
		t.Fatalf("current-step reviewer dispatch did not return hash-bound apply: %+v", dispatch)
	}
	runCurrentStepApply(t, caseRoot, dispatch, "-ReviewerHarness", "current-step-harness", "-ReviewerSession", "current-step-reviewer", "-Actor", actor)

	waiting := runCurrentStepPreview(t, base)
	if waiting.Route != "reviewer" || waiting.ReviewerStep == nil || waiting.ReviewerStep.ExternalHandoff == nil || waiting.ReviewerStep.ExternalHandoff.RunLoopStepID != "save-result-input" {
		t.Fatalf("current-step did not retain reviewer route while waiting for result: %+v", waiting)
	}
	evidencePath := filepath.Join(caseRoot, "workspace", "review-evidence.md")
	if err := os.WriteFile(evidencePath, []byte("bounded reviewer evidence\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	resultSource := filepath.Join(caseRoot, "workspace", "current-step-reviewer-result.json")
	data, err := json.Marshal(map[string]any{
		"packetId": packet.PacketID, "routeId": packet.Route.ID, "shardId": handoff.ShardID,
		"items": []string{"alpha"}, "reviewerSession": "current-step-reviewer",
		"decision": "accept", "confidence": "high", "summary": "reviewed through current-step",
		"evidenceRefs": []string{"workspace/review-evidence.md"}, "risks": []string{}, "conflicts": []string{},
		"recommendedVerdict": "accepted",
		"routeOutput":        map[string]any{"item": "alpha", "decision": "accept", "confidence": "high", "evidence": "workspace/review-evidence.md", "risk": "low", "next_action": "main-agent-writeback", "tier_used": "light", "tool_scope": "read-only", "feature": "review", "request_id": "n/a", "candidate_path": "n/a", "defer_reason": "n/a"},
	})
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(resultSource, data, 0o644); err != nil {
		t.Fatal(err)
	}
	inputArgs := append(append([]string{}, base...), "-ReviewerResultInputSourcePath", resultSource, "-Actor", actor)
	input := runCurrentStepPreview(t, inputArgs)
	runCurrentStepApply(t, caseRoot, input, "-ReviewerResultInputSourcePath", resultSource, "-Actor", actor)

	for _, stepID := range []string{"record-completion", "source-capture", "stage-candidate", "collect-result", "intake-results"} {
		stepArgs := append(append([]string{}, base...), "-Actor", actor)
		step := runCurrentStepPreview(t, stepArgs)
		if step.Route != "reviewer" || step.ReviewerStep == nil || step.ReviewerStep.CurrentDriverRequest.RunLoopStepID != stepID || step.ExpectedCurrentStepPlanSHA256 == "" {
			t.Fatalf("current-step reviewer pipeline step %s mismatch: %+v", stepID, step)
		}
		runCurrentStepApply(t, caseRoot, step, "-Actor", actor)
	}

	resumed := runCurrentStepPreview(t, base)
	if resumed.Route != "case" || resumed.DriverStep == nil || resumed.ReviewerStep != nil {
		t.Fatalf("current-step did not return to lane route after reviewer intake: %+v", resumed)
	}
	for _, forbidden := range []string{"authority.jsonl", "confirmed.jsonl"} {
		if _, err := os.Stat(filepath.Join(caseRoot, ".rekit", "facts", forbidden)); !os.IsNotExist(err) {
			t.Fatalf("current-step created forbidden ledger %s: %v", forbidden, err)
		}
	}

	driftPreview := runCurrentStepPreview(t, base)
	if driftPreview.Route != "case" || driftPreview.ExpectedCurrentStepPlanSHA256 == "" {
		t.Fatalf("post-review valid plan missing: %+v", driftPreview)
	}
	out.Reset()
	if err := Run([]string{"-Command", "start", "-Target", caseRoot, "-Pack", "_template", "-Name", "drifted", "-Apply", "-Format", "json"}, &out); err != nil {
		t.Fatal(err)
	}
	out.Reset()
	err = Run([]string{"-Command", "run-current-step", "-Target", caseRoot, "-Pack", "_template", "-Lane", "main", "-ExpectedCurrentStepPlanSha256", driftPreview.ExpectedCurrentStepPlanSHA256, "-Apply", "-Format", "json"}, &out)
	if err == nil || !strings.Contains(err.Error(), "expected plan sha256 mismatch") {
		t.Fatalf("valid current-step plan survived durable state drift: %v", err)
	}
}

func TestRunCurrentStepStopsForExecutionEvidenceReviewBeforeMemberDispatch(t *testing.T) {
	for _, test := range []struct {
		name         string
		verification string
		decision     string
		wantSource   string
	}{
		{name: "pending", wantSource: "executionEvidenceReview"},
		{
			name:         "rejected verification",
			verification: `{"kind":"verification","eventId":"verification-rejected-1","lane":"main","subject":"execution evidence review rejected","status":"rejected","verifier":"manual-review","verdict":"rejected","related":["obs-auth-current-step","gate-auth-current-step"]}`,
			wantSource:   "executionEvidenceReview",
		},
		{
			name:       "rejected decision",
			decision:   `{"kind":"decision","eventId":"decision-rejected-1","lane":"main","subject":"execution evidence review rejected","status":"rejected","decision":"reject","related":["obs-auth-current-step","gate-auth-current-step"]}`,
			wantSource: "executionEvidenceReview",
		},
	} {
		t.Run(test.name, func(t *testing.T) {
			caseRoot := currentStepEvidenceReviewCase(t)
			if test.verification != "" {
				writeCaseFile(t, caseRoot, ".rekit/facts/verifications.jsonl", test.verification+"\n")
			}
			if test.decision != "" {
				writeCaseFile(t, caseRoot, ".rekit/facts/decisions.jsonl", test.decision+"\n")
			}
			before := snapshotFiles(t, filepath.Join(caseRoot, ".rekit"))
			preview := runCurrentStepPreview(t, []string{
				"-Command", "run-current-step", "-Target", caseRoot,
				"-Pack", "_template", "-WhatIf", "-Format", "json",
			})
			if preview.CurrentDriverRequest.Source != test.wantSource ||
				preview.CurrentDriverRequest.State != "ready-for-evidence-review" ||
				preview.MemberExecution != nil || preview.DriverStep != nil ||
				preview.ReviewerStep != nil || preview.ExternalSessionStep != nil ||
				preview.ExpectedCurrentStepPlanSHA256 != "" {
				t.Fatalf("evidence review did not remain a zero-mutation typed stop: %+v", preview)
			}
			assertSnapshotEqual(t, before, snapshotFiles(t, filepath.Join(caseRoot, ".rekit")))
		})
	}
}

func TestRunCurrentStepEvidenceReviewSupersedesReadyExternalMemberCheckpoint(t *testing.T) {
	caseRoot := currentStepMemberCase(t, "checkpoint-member")
	loop := runCurrentLoopPreviewWith(t, caseRoot, 2, "-Lane", "main")
	if loop.InitialCurrentStep == nil || loop.InitialCurrentStep.MemberExecution == nil {
		t.Fatalf("member loop preview omitted dispatch: %+v", loop)
	}
	currentStepBeforeStatusRefreshHook = func(command string) error {
		if command != commands.RunCurrentStep {
			return nil
		}
		writeCurrentStepEvidenceReviewObservation(t, caseRoot)
		return nil
	}
	t.Cleanup(func() { currentStepBeforeStatusRefreshHook = nil })
	applied := runCurrentLoopApplyWith(
		t,
		caseRoot,
		loop,
		"-ExpectedMemberExecutionPlanSha256",
		loop.InitialCurrentStep.MemberExecution.ExpectedPlanSHA256,
	)
	currentStepBeforeStatusRefreshHook = nil
	if !applied.Applied || applied.StopReason.Code != "external-member-handoff" ||
		applied.SegmentCheckpoint == nil || !applied.SegmentCheckpoint.Ready {
		t.Fatalf("member dispatch did not publish a ready checkpoint: %+v", applied)
	}

	ctx, err := rekitruntime.New(caseRoot, "_template")
	if err != nil {
		t.Fatal(err)
	}
	status, err := buildInvocationStatusInventory(ctx, Options{
		Command:             commands.Status,
		SelectedCurrentLane: "main",
	})
	if err != nil {
		t.Fatal(err)
	}
	runbook := status.MissionControlRunbook
	if runbook == nil || runbook.CurrentDriverRequest == nil ||
		!currentStepRequestIsEvidenceReview(*runbook.CurrentDriverRequest) ||
		runbook.CurrentLoopSegment == nil ||
		runbook.CurrentLoopOperator == nil ||
		runbook.CurrentLoopOperator.ExternalSessionJob != nil {
		t.Fatalf("evidence review did not suppress the prior external checkpoint handoff: %+v", runbook)
	}
	before := snapshotFiles(t, filepath.Join(caseRoot, ".rekit"))
	preview := runCurrentStepPreview(t, []string{
		"-Command", "run-current-step", "-Target", caseRoot,
		"-Pack", "_template", "-Lane", "main", "-WhatIf", "-Format", "json",
	})
	if !currentStepRequestIsEvidenceReview(preview.CurrentDriverRequest) ||
		preview.MemberExecution != nil || preview.DriverStep != nil ||
		preview.ReviewerStep != nil || preview.ExternalSessionStep != nil ||
		preview.ExpectedCurrentStepPlanSHA256 != "" {
		t.Fatalf("ready checkpoint external wrapper overrode evidence review: %+v", preview)
	}
	assertSnapshotEqual(t, before, snapshotFiles(t, filepath.Join(caseRoot, ".rekit")))
}

func TestRunCurrentLoopRejectsMemberDispatchWhenEvidenceReviewArrivesBeforeApply(t *testing.T) {
	caseRoot := currentStepMemberCase(t, "race-member")
	loop := runCurrentLoopPreview(t, caseRoot, 2)
	if loop.InitialCurrentStep == nil || loop.InitialCurrentStep.MemberExecution == nil {
		t.Fatalf("member loop preview omitted dispatch: %+v", loop)
	}
	currentLoopBeforeApplyStepHook = func(step int) error {
		if step == 1 {
			writeCurrentStepEvidenceReviewObservation(t, caseRoot)
		}
		return nil
	}
	t.Cleanup(func() { currentLoopBeforeApplyStepHook = nil })
	applied := runCurrentLoopApplyWith(
		t,
		caseRoot,
		loop,
		"-ExpectedMemberExecutionPlanSha256",
		loop.InitialCurrentStep.MemberExecution.ExpectedPlanSHA256,
	)
	currentLoopBeforeApplyStepHook = nil
	if applied.Applied || applied.AppliedSteps != 0 || applied.StopReason.Code != "error" ||
		applied.StopReason.Phase != "apply-step" ||
		!strings.Contains(applied.StopReason.Message, "no longer the member-owned continuation") {
		t.Fatalf("evidence-review race did not reject stale member Apply: %+v", applied)
	}
	if _, ok, err := memberexecution.Latest(caseRoot, "main"); err != nil || ok {
		t.Fatalf("rejected stale member Apply published an attempt: present=%t err=%v", ok, err)
	}
}

func TestBuildCurrentStepRejectsUnknownMemberContinuationSourceOrState(t *testing.T) {
	caseRoot := currentStepMemberCase(t, "future-member")
	ctx, err := rekitruntime.New(caseRoot, "_template")
	if err != nil {
		t.Fatal(err)
	}
	status, err := buildStatusInventory(ctx, "")
	if err != nil {
		t.Fatal(err)
	}
	if status.MissionControlRunbook == nil || status.MissionControlRunbook.CurrentDriverRequest == nil {
		t.Fatal("member case omitted current request")
	}
	for _, test := range []struct {
		name   string
		source string
		state  string
	}{
		{name: "future source", source: "missionCommanderActions.v2", state: "ready-to-continue"},
		{name: "future state", source: "missionCommanderActions", state: "ready-to-resume-v2"},
	} {
		t.Run(test.name, func(t *testing.T) {
			candidate := status
			runbook := *status.MissionControlRunbook
			request := *status.MissionControlRunbook.CurrentDriverRequest
			request.Source = test.source
			request.State = test.state
			runbook.CurrentDriverRequest = &request
			candidate.MissionControlRunbook = &runbook
			_, err := buildCurrentStepPlanFromStatus(ctx, Options{WhatIf: true, Pack: "_template"}, candidate)
			if err == nil || !strings.Contains(err.Error(), "rejects unrecognized member continuation") {
				t.Fatalf("unknown continuation was not rejected: %v", err)
			}
		})
	}
}

func TestRunCurrentStepResumesMemberOnlyAfterEvidenceReviewClosure(t *testing.T) {
	for _, test := range []struct {
		name         string
		ledgerPath   string
		closureEvent string
	}{
		{
			name:         "accepted verification",
			ledgerPath:   ".rekit/facts/verifications.jsonl",
			closureEvent: `{"kind":"verification","eventId":"verification-accepted-1","lane":"main","subject":"execution evidence review accepted","status":"resolved","verifier":"manual-review","verdict":"accepted","related":["obs-auth-current-step","gate-auth-current-step"]}`,
		},
		{
			name:         "superseded decision",
			ledgerPath:   ".rekit/facts/decisions.jsonl",
			closureEvent: `{"kind":"decision","eventId":"decision-superseded-1","lane":"main","subject":"execution evidence review superseded","status":"superseded","decision":"supersede","related":["obs-auth-current-step","gate-auth-current-step"]}`,
		},
	} {
		t.Run(test.name, func(t *testing.T) {
			caseRoot := currentStepEvidenceReviewCase(t)
			writeCaseFile(t, caseRoot, test.ledgerPath, test.closureEvent+"\n")
			preview := runCurrentStepPreview(t, []string{
				"-Command", "run-current-step", "-Target", caseRoot,
				"-Pack", "_template", "-WhatIf", "-Format", "json",
			})
			if preview.CurrentDriverRequest.Source != "missionCommanderActions" ||
				preview.CurrentDriverRequest.State != "ready-to-continue" ||
				preview.MemberExecution == nil ||
				preview.ExpectedCurrentStepPlanSHA256 == "" {
				t.Fatalf("closed evidence review did not restore fresh member-owned continuation: %+v", preview)
			}
		})
	}
}

func currentStepEvidenceReviewCase(t *testing.T) string {
	t.Helper()
	caseRoot := currentStepMemberCase(t, "evidence-review-member")
	writeCurrentStepEvidenceReviewObservation(t, caseRoot)
	factsRoot := filepath.Join(caseRoot, ".rekit", "facts")
	writeFactFile(t, factsRoot, "requests.jsonl", nil)
	writeFactFile(t, factsRoot, "candidates.jsonl", nil)
	writeFactFile(t, factsRoot, "decisions.jsonl", nil)
	writeFactFile(t, factsRoot, "interventions.jsonl", nil)
	writeFactFile(t, factsRoot, "verifications.jsonl", nil)
	return caseRoot
}

func currentStepMemberCase(t *testing.T, executor string) string {
	t.Helper()
	caseRoot := fullAttachedCase(t)
	if err := Run([]string{"-Command", "overview", "-Target", caseRoot, "-Pack", "_template", "-Format", "json"}, &bytes.Buffer{}); err != nil {
		t.Fatal(err)
	}
	board, err := mission.ReadBoard(caseRoot)
	if err != nil {
		t.Fatal(err)
	}
	board.Lanes[0].CurrentExecutor = executor
	board.Lanes[0].ExecutorGeneration = 1
	board.Lanes[0].UpdatedAt = "2026-08-11T01:00:00Z"
	boardData, err := json.MarshalIndent(board, "", "  ")
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(caseRoot, ".rekit", "board.json"), append(boardData, '\n'), 0o644); err != nil {
		t.Fatal(err)
	}
	return caseRoot
}

func writeCurrentStepEvidenceReviewObservation(t *testing.T, caseRoot string) {
	t.Helper()
	writeFactFile(t, filepath.Join(caseRoot, ".rekit", "facts"), "observations.jsonl", []string{`{"kind":"observation","eventId":"obs-auth-current-step","lane":"main","subject":"bounded adapter output","summary":"preauthorized adapter output ready for review","status":"complete","target":"target-alpha","evidenceRefs":["evidence/debug.json"],"execution":{"gateEventId":"gate-auth-current-step","authorization":"preauthorized","status":"complete","outputRefs":["workspace/main/debug/out.txt"]},"gate":{"action":"debug","authorization":{"decision":"preauthorized"}}}`})
}

func TestRunCurrentStepMemberDispatchExactReplayDoesNotClaimProgress(t *testing.T) {
	caseRoot := fullAttachedCase(t)
	var out bytes.Buffer
	if err := Run([]string{"-Command", "overview", "-Target", caseRoot, "-Pack", "_template", "-Format", "json"}, &out); err != nil {
		t.Fatal(err)
	}
	board, err := mission.ReadBoard(caseRoot)
	if err != nil {
		t.Fatal(err)
	}
	board.Lanes[0].CurrentExecutor = "replay-member"
	board.Lanes[0].ExecutorGeneration = 1
	board.Lanes[0].UpdatedAt = "2026-08-08T01:00:00Z"
	boardData, err := json.MarshalIndent(board, "", "  ")
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(caseRoot, ".rekit", "board.json"), append(boardData, '\n'), 0o644); err != nil {
		t.Fatal(err)
	}
	base := []string{"-Command", "run-current-step", "-Target", caseRoot, "-Pack", "_template", "-WhatIf", "-Format", "json"}
	preview := runCurrentStepPreview(t, base)
	if preview.MemberExecution == nil {
		t.Fatalf("member dispatch preview missing: %+v", preview)
	}
	if _, err := memberexecution.Apply(*preview.MemberExecution, preview.MemberExecution.ExpectedPlanSHA256); err != nil {
		t.Fatal(err)
	}
	var replayOut bytes.Buffer
	err = Run([]string{
		"-Command", "run-current-step", "-Target", caseRoot, "-Pack", "_template",
		"-ExpectedMemberExecutionPlanSha256", preview.MemberExecution.ExpectedPlanSHA256,
		"-ExpectedCurrentStepPlanSha256", preview.ExpectedCurrentStepPlanSHA256,
		"-Apply", "-Format", "json",
	}, &replayOut)
	if err != nil {
		t.Fatal(err)
	}
	var replay currentStepTestPlan
	if err := json.Unmarshal(replayOut.Bytes(), &replay); err != nil {
		t.Fatalf("exact replay JSON did not decode: %v\n%s", err, replayOut.String())
	}
	if replay.Applied || replay.Receipt == nil || replay.Receipt.Outcome != "current-step-applied" || replay.RefreshedStatus != nil {
		t.Fatalf("exact member replay claimed new progress: %+v", replay)
	}
}

func TestRunCurrentStepDurableMemberExecutionFixtureHandoffAndIntake(t *testing.T) {
	caseRoot := filepath.Join(t.TempDir(), "member-public-journey")
	var out bytes.Buffer
	baseOnboard := []string{"-Command", "onboard", "-Target", caseRoot, "-Pack", "_template", "-ProjectName", "member-public-journey", "-Goal", "bounded member result with reviewer-backed completion", "-Actor", "operator", "-Executor", "member-session-a", "-InitialLane", "feature-analysis"}
	var onboard onboardCLIPlan
	runCompletionJSON(t, &out, append(append([]string{}, baseOnboard...), "-WhatIf", "-Format", "json"), &onboard)
	runCompletionJSON(t, &out, onboard.ApplyArgs, nil)
	var journeyStatus struct {
		MissionControlRunbook *statusMissionControlRunbookSnapshot `json:"missionControlRunbook"`
	}
	runCompletionJSON(t, &out, []string{"-Command", "status", "-Target", caseRoot, "-Pack", "_template", "-Format", "json"}, &journeyStatus)
	overviewArgs, ok := missionCommanderDriverRequestCommandCLIArgs(t, journeyStatus.MissionControlRunbook.Quickstart.CurrentDriverRequest)
	if !ok {
		t.Fatal("member journey overview route is not executable")
	}
	runCompletionJSON(t, &out, overviewArgs, nil)
	runCompletionJSON(t, &out, []string{"-Command", "status", "-Target", caseRoot, "-Pack", "_template", "-Format", "json"}, &journeyStatus)
	startArgs, ok := missionCommanderDriverRequestCommandCLIArgs(t, journeyStatus.MissionControlRunbook.Quickstart.CurrentDriverRequest)
	if !ok {
		t.Fatal("member journey start route is not executable")
	}
	var startPreview struct {
		MissionCommanderActionQueue missionCommanderActionQueueSnapshot `json:"missionCommanderActionQueue"`
	}
	runCompletionJSON(t, &out, startArgs, &startPreview)
	startApplyArgs, ok := missionCommanderDriverRequestCommandCLIArgs(t, startPreview.MissionCommanderActionQueue.CurrentDriverRequest)
	if !ok {
		t.Fatal("member journey start Apply route is not executable")
	}
	runCompletionJSON(t, &out, append(startApplyArgs, "-Target", caseRoot, "-Pack", "_template", "-Format", "json"), nil)
	base := []string{"-Command", "run-current-step", "-Target", caseRoot, "-Pack", "_template", "-Lane", "feature-analysis", "-WhatIf", "-Format", "json"}
	dispatch := runCurrentStepPreview(t, base)
	if dispatch.MemberExecution == nil || dispatch.MemberExecution.ExternalHandoff == nil || dispatch.ExpectedCurrentStepPlanSHA256 == "" {
		t.Fatalf("member dispatch preview mismatch: %+v", dispatch)
	}
	attempt := dispatch.MemberExecution.AttemptID
	memberHash := dispatch.MemberExecution.ExpectedPlanSHA256
	intentData, err := json.MarshalIndent(dispatch.MemberExecution.Inspection.Intent, "", "  ")
	if err != nil {
		t.Fatal(err)
	}
	intentPath := filepath.Join(dispatch.MemberExecution.Inspection.AttemptRoot, "intent.json")
	if err := os.MkdirAll(filepath.Dir(intentPath), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(intentPath, append(intentData, '\n'), 0o600); err != nil {
		t.Fatal(err)
	}
	recoveredDispatch := runCurrentStepPreview(t, base)
	if recoveredDispatch.ExpectedCurrentStepPlanSHA256 != dispatch.ExpectedCurrentStepPlanSHA256 || recoveredDispatch.MemberExecution == nil || recoveredDispatch.MemberExecution.ExpectedPlanSHA256 != memberHash || recoveredDispatch.MemberExecution.AttemptID != attempt {
		t.Fatalf("public current-step did not reconstruct exact intent-only dispatch: recovered=%+v original=%+v", recoveredDispatch, dispatch)
	}
	apply := runMemberCurrentStepForLane(t, caseRoot, "feature-analysis", []string{"-ExpectedMemberExecutionPlanSha256", memberHash, "-ExpectedCurrentStepPlanSha256", recoveredDispatch.ExpectedCurrentStepPlanSHA256, "-Apply"})
	if !apply.Applied || apply.MemberExecution == nil || apply.MemberExecution.Inspection.State != "handoff-ready" || apply.Receipt == nil || apply.Receipt.State != "refreshed" || apply.Receipt.Outcome != "current-step-applied" || apply.RefreshedStatus == nil || apply.Receipt.RefreshedCurrentDriverRequest == nil {
		t.Fatalf("member dispatch Apply mismatch: %+v", apply)
	}

	acceptedPreview := runMemberCurrentStepForLane(t, caseRoot, "feature-analysis", []string{"-MemberExecutionAttemptId", attempt, "-MemberExecutionOutcome", "accepted", "-MemberExecutionObservedAt", "2026-08-03T02:01:00Z", "-Actor", "harness", "-WhatIf"})
	acceptedAuthorized := runMemberCurrentStepForLane(t, caseRoot, "feature-analysis", []string{"-MemberExecutionAttemptId", attempt, "-MemberExecutionOutcome", "accepted", "-MemberExecutionObservedAt", "2026-08-03T02:01:00Z", "-Actor", "harness", "-ExpectedMemberExecutionPlanSha256", acceptedPreview.MemberExecution.ExpectedPlanSHA256, "-WhatIf"})
	acceptedApply := runMemberCurrentStepForLane(t, caseRoot, "feature-analysis", []string{"-MemberExecutionAttemptId", attempt, "-MemberExecutionOutcome", "accepted", "-MemberExecutionObservedAt", "2026-08-03T02:01:00Z", "-Actor", "harness", "-ExpectedMemberExecutionPlanSha256", acceptedPreview.MemberExecution.ExpectedPlanSHA256, "-ExpectedCurrentStepPlanSha256", acceptedAuthorized.ExpectedCurrentStepPlanSHA256, "-Apply"})
	if acceptedApply.MemberExecution == nil || acceptedApply.MemberExecution.Inspection.State != "accepted" || acceptedApply.RefreshedStatus == nil || acceptedApply.Receipt == nil || acceptedApply.Receipt.RefreshedCurrentDriverRequest == nil {
		t.Fatalf("member accepted Apply mismatch: %+v", acceptedApply)
	}

	inspection := acceptedApply.MemberExecution.Inspection
	output := []byte("member-result\n")
	if err := os.MkdirAll(inspection.OutputsRoot, 0o755); err != nil {
		t.Fatal(err)
	}
	outputPath := filepath.Join(inspection.OutputsRoot, "review-items.json")
	if err := os.WriteFile(outputPath, output, 0o600); err != nil {
		t.Fatal(err)
	}
	manifest := memberexecution.ResultManifest{SchemaVersion: 1, Kind: memberexecution.KindManifest, AttemptID: attempt, Owner: acceptedApply.MemberExecution.Owner, Summary: "bounded member result", Outputs: []memberexecution.Output{{Path: "review-items.json", SHA256: sha256Text(output), Bytes: int64(len(output))}}, ReviewerItemsPath: "review-items.json", NoAuthority: true, NoConfirmed: true, NoHeavyTool: true}
	manifestData, err := memberexecution.MarshalResultManifest(manifest)
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(inspection.ManifestPath, manifestData, 0o600); err != nil {
		t.Fatal(err)
	}
	returnedPreview := runMemberCurrentStepForLane(t, caseRoot, "feature-analysis", []string{"-MemberExecutionAttemptId", attempt, "-MemberExecutionOutcome", "returned", "-MemberExecutionObservedAt", "2026-08-03T02:02:00Z", "-Actor", "harness", "-WhatIf"})
	returnedAuthorized := runMemberCurrentStepForLane(t, caseRoot, "feature-analysis", []string{"-MemberExecutionAttemptId", attempt, "-MemberExecutionOutcome", "returned", "-MemberExecutionObservedAt", "2026-08-03T02:02:00Z", "-Actor", "harness", "-ExpectedMemberExecutionPlanSha256", returnedPreview.MemberExecution.ExpectedPlanSHA256, "-WhatIf"})
	returnedApply := runMemberCurrentStepForLane(t, caseRoot, "feature-analysis", []string{"-MemberExecutionAttemptId", attempt, "-MemberExecutionOutcome", "returned", "-MemberExecutionObservedAt", "2026-08-03T02:02:00Z", "-Actor", "harness", "-ExpectedMemberExecutionPlanSha256", returnedPreview.MemberExecution.ExpectedPlanSHA256, "-ExpectedCurrentStepPlanSha256", returnedAuthorized.ExpectedCurrentStepPlanSHA256, "-Apply"})
	if returnedApply.MemberExecution == nil || returnedApply.MemberExecution.Inspection.State != "intake-ready" || returnedApply.RefreshedStatus == nil || returnedApply.Receipt == nil || returnedApply.Receipt.RefreshedCurrentDriverRequest == nil {
		t.Fatalf("member returned intake mismatch: %+v", returnedApply)
	}
	resumed := runCurrentStepPreview(t, base)
	if resumed.DriverStep == nil || resumed.MemberExecution != nil || resumed.ExpectedCurrentStepPlanSHA256 == "" {
		t.Fatalf("member intake did not resume existing continue: %+v", resumed)
	}
	continued := runCurrentStepApply(t, caseRoot, resumed)
	if !continued.Applied || continued.Receipt == nil || continued.Receipt.State != "refreshed" || continued.RefreshedStatus == nil || continued.DriverStep == nil || continued.DriverStep.Receipt == nil || !continued.DriverStep.Receipt.ExpectedReceiptCommandMatched || !continued.DriverStep.Receipt.RefreshStatusCommandMatched {
		t.Fatalf("member intake continue did not publish durable nested receipt/fresh status: %+v", continued)
	}
	status := *continued.RefreshedStatus
	if status.MemberExecution == nil || status.MemberExecution.State != "intake-ready" || status.MemberExecution.ReviewerPlanCommand == "" || len(status.MemberExecution.CompletionEvidence) != 1 {
		t.Fatalf("status omitted member reviewer/completion relay: %+v", status.MemberExecution)
	}
	t.Run("stale owner generation cannot plan reviewer", func(t *testing.T) {
		boardPath := filepath.Join(caseRoot, ".rekit", "board.json")
		original, err := os.ReadFile(boardPath)
		if err != nil {
			t.Fatal(err)
		}
		defer func() {
			if err := os.WriteFile(boardPath, original, 0o644); err != nil {
				t.Errorf("restore board fixture: %v", err)
			}
		}()
		var board mission.Board
		if err := json.Unmarshal(original, &board); err != nil {
			t.Fatal(err)
		}
		for index := range board.Lanes {
			if board.Lanes[index].ID == "feature-analysis" {
				board.Lanes[index].CurrentExecutor = "replacement-session"
				board.Lanes[index].ExecutorGeneration++
			}
		}
		changed, err := json.MarshalIndent(board, "", "  ")
		if err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(boardPath, append(changed, '\n'), 0o644); err != nil {
			t.Fatal(err)
		}
		var staleStatus statusInventory
		runCompletionJSON(t, &out, []string{"-Command", "status", "-Target", caseRoot, "-Pack", "_template", "-Lane", "feature-analysis", "-Format", "json"}, &staleStatus)
		if staleStatus.MemberExecution == nil || staleStatus.MemberExecution.State != "dispatch-preview-required" || staleStatus.MemberExecution.ReviewerPlanCommand != "" || len(staleStatus.MemberExecution.CompletionEvidence) != 0 {
			t.Fatalf("stale member generation drove current reviewer planning: %+v", staleStatus.MemberExecution)
		}
	})
	unreviewed := previewCompletion(t, &out, caseRoot, "analysis", status.MemberExecution.CompletionEvidence[0])
	if !unreviewed.Blocked || !hasCompletionBlocker(unreviewed.Blockers, "member-manifest-review") || unreviewed.CompletionPlanSHA256 != "" {
		t.Fatalf("member completion without manifest-bound reviewer writeback was not blocked: %+v", unreviewed)
	}
	for _, forbidden := range []string{"authority.jsonl", "confirmed.jsonl"} {
		if _, err := os.Stat(filepath.Join(caseRoot, ".rekit", "facts", forbidden)); !os.IsNotExist(err) {
			t.Fatalf("member execution created forbidden ledger %s: %v", forbidden, err)
		}
	}

	out.Reset()
	if err := Run(rekitCommandCLIArgs(t, status.MemberExecution.ReviewerPlanCommand), &out); err != nil {
		t.Fatal(err)
	}
	reviewerPlan := decodePlanSubagentsResult(t, out.Bytes())
	packet := decodePlanSubagentsPacket(t, reviewerPlan.PacketPath)
	handoff := reviewerPlan.ShardHandoffs[0]
	reviewerBase := []string{"-Command", "run-current-step", "-Target", caseRoot, "-Pack", "_template", "-Lane", "feature-analysis", "-WhatIf", "-Format", "json"}
	dispatchReviewer := runCurrentStepPreview(t, append(append([]string{}, reviewerBase...), "-ReviewerHarness", "member-e2e-harness", "-ReviewerSession", "member-e2e-reviewer", "-Actor", "mission-commander"))
	runCurrentStepApply(t, caseRoot, dispatchReviewer, "-ReviewerHarness", "member-e2e-harness", "-ReviewerSession", "member-e2e-reviewer", "-Actor", "mission-commander")
	resultSource := filepath.Join(caseRoot, "workspace", "member-review-result.json")
	reviewerItem := status.MemberExecution.CompletionEvidence[0]
	resultData, _ := json.Marshal(map[string]any{
		"packetId": packet.PacketID, "routeId": packet.Route.ID, "shardId": handoff.ShardID, "items": []string{reviewerItem},
		"reviewerSession": "member-e2e-reviewer", "decision": "accept", "confidence": "high", "summary": "member manifest reviewed", "evidenceRefs": []string{reviewerItem}, "risks": []string{}, "conflicts": []string{}, "recommendedVerdict": "accepted",
		"routeOutput": map[string]any{"item": reviewerItem, "decision": "accept", "confidence": "high", "evidence": reviewerItem, "risk": "low", "next_action": "main-agent-writeback", "tier_used": "light", "tool_scope": "read-only", "feature": "review", "request_id": "n/a", "candidate_path": "n/a", "defer_reason": "n/a"},
	})
	if err := os.WriteFile(resultSource, resultData, 0o644); err != nil {
		t.Fatal(err)
	}
	input := runCurrentStepPreview(t, append(append([]string{}, reviewerBase...), "-ReviewerResultInputSourcePath", resultSource, "-Actor", "mission-commander"))
	runCurrentStepApply(t, caseRoot, input, "-ReviewerResultInputSourcePath", resultSource, "-Actor", "mission-commander")
	for _, stepID := range []string{"record-completion", "source-capture", "stage-candidate", "collect-result", "intake-results"} {
		step := runCurrentStepPreview(t, append(append([]string{}, reviewerBase...), "-Actor", "mission-commander"))
		if step.ReviewerStep == nil || step.ReviewerStep.CurrentDriverRequest.RunLoopStepID != stepID {
			t.Fatalf("member reviewer pipeline step %s mismatch: %+v", stepID, step)
		}
		runCurrentStepApply(t, caseRoot, step, "-Actor", "mission-commander")
	}
	inputPath := handoff.ReviewerStagingCommands.SourceCaptureInput
	inputBytes, err := os.ReadFile(inputPath)
	if err != nil {
		t.Fatal(err)
	}
	if err := os.Remove(inputPath); err != nil {
		t.Fatal(err)
	}
	deletedInput := previewCompletion(t, &out, caseRoot, "analysis", status.MemberExecution.CompletionEvidence[0])
	if !deletedInput.Blocked || !hasCompletionBlocker(deletedInput.Blockers, "member-manifest-review") || deletedInput.CompletionPlanSHA256 != "" {
		t.Fatalf("completion survived deleted canonical reviewer input: %+v", deletedInput)
	}
	if err := os.WriteFile(inputPath, append(append([]byte{}, inputBytes...), ' '), 0o600); err != nil {
		t.Fatal(err)
	}
	if _, accepted, acceptanceErr := workstream.CurrentMemberManifestReviewerAcceptance(caseRoot, "feature-analysis", reviewerItem); acceptanceErr == nil || accepted {
		t.Fatalf("canonical acceptance survived replaced reviewer input: accepted=%t err=%v", accepted, acceptanceErr)
	}
	replacedInput := previewCompletion(t, &out, caseRoot, "analysis", status.MemberExecution.CompletionEvidence[0])
	if !replacedInput.Blocked || !hasCompletionBlocker(replacedInput.Blockers, "member-manifest-review") || replacedInput.CompletionPlanSHA256 != "" {
		t.Fatalf("completion survived replaced canonical reviewer input: %+v", replacedInput)
	}
	if err := os.WriteFile(inputPath, inputBytes, 0o600); err != nil {
		t.Fatal(err)
	}
	acceptance, accepted, err := workstream.CurrentMemberManifestReviewerAcceptance(caseRoot, "feature-analysis", reviewerItem)
	if err != nil || !accepted || acceptance.ManifestRef != reviewerItem || acceptance.ManifestSHA256 != sha256Text(manifestData) || acceptance.PacketID != packet.PacketID || acceptance.RouteID != packet.Route.ID || acceptance.ShardID != handoff.ShardID || acceptance.ReviewerSession != "member-e2e-reviewer" || acceptance.OwnerExecutor != acceptedApply.MemberExecution.Owner.Executor || acceptance.OwnerGeneration != acceptedApply.MemberExecution.Owner.ExecutorGeneration || acceptance.VerificationEventID == "" || acceptance.DecisionEventID == "" {
		t.Fatalf("canonical reviewer acceptance identity mismatch: accepted=%t acceptance=%+v err=%v", accepted, acceptance, err)
	}
	t.Run("accepted writeback rejects canonical reject semantics", func(t *testing.T) {
		completionRoot := filepath.Join(filepath.Dir(reviewerPlan.PacketPath), "sessions", handoff.ShardID, "completions")
		entries, err := os.ReadDir(completionRoot)
		if err != nil {
			t.Fatal(err)
		}
		if len(entries) != 1 || entries[0].IsDir() {
			t.Fatalf("expected exactly one reviewer completion receipt, got %+v", entries)
		}
		completionPath := filepath.Join(completionRoot, entries[0].Name())
		verificationPath := filepath.Join(caseRoot, ".rekit", "facts", "verifications.jsonl")
		decisionPath := filepath.Join(caseRoot, ".rekit", "facts", "decisions.jsonl")
		originals := map[string][]byte{}
		for _, path := range []string{inputPath, completionPath, verificationPath, decisionPath} {
			data, readErr := os.ReadFile(path)
			if readErr != nil {
				t.Fatal(readErr)
			}
			originals[path] = data
		}
		defer func() {
			for path, data := range originals {
				if writeErr := os.WriteFile(path, data, 0o600); writeErr != nil {
					t.Errorf("restore %s: %v", path, writeErr)
				}
			}
		}()

		var rejected map[string]any
		if err := json.Unmarshal(inputBytes, &rejected); err != nil {
			t.Fatal(err)
		}
		rejected["decision"] = "reject"
		rejected["recommendedVerdict"] = "rejected"
		rejected["routeOutput"].(map[string]any)["decision"] = "reject"
		rejectedInput, err := json.Marshal(rejected)
		if err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(inputPath, rejectedInput, 0o600); err != nil {
			t.Fatal(err)
		}
		inputSum := sha256.Sum256(rejectedInput)
		inputSHA := hex.EncodeToString(inputSum[:])

		var completion map[string]any
		if err := json.Unmarshal(originals[completionPath], &completion); err != nil {
			t.Fatal(err)
		}
		completion["reviewerResultInputSha256"] = inputSHA
		completion["reviewerResultInputBytes"] = len(rejectedInput)
		completionBytes, err := json.MarshalIndent(completion, "", "  ")
		if err != nil {
			t.Fatal(err)
		}
		completionBytes = append(completionBytes, '\n')
		if err := os.WriteFile(completionPath, completionBytes, 0o600); err != nil {
			t.Fatal(err)
		}
		completionSum := sha256.Sum256(completionBytes)
		completionSHA := hex.EncodeToString(completionSum[:])

		rewriteAcceptedLedger := func(path string) {
			lines := bytes.Split(bytes.TrimSpace(originals[path]), []byte{'\n'})
			out := []byte{}
			matched := false
			for _, line := range lines {
				var event map[string]any
				if err := json.Unmarshal(line, &event); err != nil {
					t.Fatal(err)
				}
				if event["packetId"] == packet.PacketID {
					event["reviewerResultInputSha256"] = inputSHA
					event["reviewerCompletionReceiptSha256"] = completionSHA
					event["reviewerDecision"] = "accept"
					event["recommendedVerdict"] = "accepted"
					matched = true
				}
				encoded, err := json.Marshal(event)
				if err != nil {
					t.Fatal(err)
				}
				out = append(out, encoded...)
				out = append(out, '\n')
			}
			if !matched {
				t.Fatalf("ledger %s omitted reviewer packet %s", path, packet.PacketID)
			}
			if err := os.WriteFile(path, out, 0o600); err != nil {
				t.Fatal(err)
			}
		}
		rewriteAcceptedLedger(verificationPath)
		rewriteAcceptedLedger(decisionPath)

		reviewed, err := workstream.HasAcceptedMemberManifestReviewerWriteback(caseRoot, "feature-analysis", reviewerItem)
		if err == nil || reviewed || !strings.Contains(err.Error(), "does not match canonical reviewer result decision") {
			t.Fatalf("accepted ledger semantics overrode canonical reject result: reviewed=%t err=%v", reviewed, err)
		}
		tampered := previewCompletion(t, &out, caseRoot, "analysis", reviewerItem)
		if !tampered.Blocked || !hasCompletionBlocker(tampered.Blockers, "member-manifest-review") || tampered.CompletionPlanSHA256 != "" {
			t.Fatalf("completion accepted ledger semantics over canonical reject result: %+v", tampered)
		}
	})
	if runtime.GOOS != "windows" {
		canonicalManifest := filepath.Join(caseRoot, filepath.FromSlash(status.MemberExecution.CompletionEvidence[0]))
		aliasManifest := filepath.Join(filepath.Dir(canonicalManifest), "MANIFEST.JSON")
		manifestBytes, readErr := os.ReadFile(canonicalManifest)
		if readErr != nil {
			t.Fatal(readErr)
		}
		if err := os.WriteFile(aliasManifest, manifestBytes, 0o600); err != nil {
			t.Fatal(err)
		}
		aliasRef, relErr := filepath.Rel(caseRoot, aliasManifest)
		if relErr != nil {
			t.Fatal(relErr)
		}
		aliasRef = filepath.ToSlash(aliasRef)
		aliasCompletion := previewCompletion(t, &out, caseRoot, "analysis", aliasRef)
		if !aliasCompletion.Blocked || aliasCompletion.CompletionPlanSHA256 != "" {
			t.Fatalf("completion accepted case-distinct manifest alias: %+v", aliasCompletion)
		}
		canonicalRef := status.MemberExecution.CompletionEvidence[0]
		for name, refs := range map[string]string{
			"canonical-plus-alias":   canonicalRef + "," + aliasRef,
			"cleaned-path-duplicate": canonicalRef + "," + filepath.ToSlash(filepath.Dir(canonicalRef)) + "/../" + filepath.Base(filepath.Dir(canonicalRef)) + "/" + filepath.Base(canonicalRef),
		} {
			duplicate := previewCompletion(t, &out, caseRoot, "analysis", refs)
			if !duplicate.Blocked || duplicate.CompletionPlanSHA256 != "" {
				t.Fatalf("completion accepted %s member evidence: %+v", name, duplicate)
			}
		}
	}
	readyCompletion := previewCompletion(t, &out, caseRoot, "analysis", status.MemberExecution.CompletionEvidence[0])
	if readyCompletion.Blocked || readyCompletion.CompletionPlanSHA256 == "" {
		t.Fatalf("restored reviewer input did not unblock completion: %+v", readyCompletion)
	}
	var completionStatus statusInventory
	out.Reset()
	if err := Run([]string{"-Command", "status", "-Target", caseRoot, "-Pack", "_template", "-Format", "json"}, &out); err != nil {
		t.Fatal(err)
	}
	decodeJSONStrict(t, out.Bytes(), &completionStatus)
	completionRequest := completionStatus.MissionControlRunbook.CurrentDriverRequest
	if completionRequest == nil || completionRequest.RunLoopStepID != "preview-current" || completionRequest.ActionID != "complete-reviewed-lane-feature-analysis" || completionRequest.Source != "laneCompletion.acceptedReviewerLineage" || completionRequest.Lane != "feature-analysis" || !completionRequest.CommandExecutable || !completionRequest.RequiresReview || !strings.Contains(completionRequest.Command, "/rekit complete analysis") || !strings.Contains(completionRequest.Command, status.MemberExecution.CompletionEvidence[0]) {
		t.Fatalf("durable status did not select evidence-derived completion preview: %+v", completionRequest)
	}
	if completionStatus.MemberExecution == nil || completionStatus.MemberExecution.ReviewerPlanCommand != "" || len(completionStatus.MemberExecution.CompletionEvidence) != 1 {
		t.Fatalf("accepted member manifest requested duplicate reviewer planning: %+v", completionStatus.MemberExecution)
	}
	var completionHandoff workstream.HandoffResult
	out.Reset()
	if err := Run([]string{"-Command", "handoff", "-Target", caseRoot, "-Pack", "_template", "-WhatIf", "-Format", "json"}, &out); err != nil {
		t.Fatal(err)
	}
	decodeJSONStrict(t, out.Bytes(), &completionHandoff)
	if completionHandoff.ReplacementExecutorTakeoverPackage == nil || completionHandoff.ReplacementExecutorTakeoverPackage.CurrentDriverRequest.Command != completionRequest.Command || completionHandoff.MissionCommanderActionQueue.CurrentDriverRequest == nil || completionHandoff.MissionCommanderActionQueue.CurrentDriverRequest.Command != completionRequest.Command {
		t.Fatalf("replacement executor handoff did not preserve completion request: %+v", completionHandoff.ReplacementExecutorTakeoverPackage)
	}
	selectedCompletionCommand := selectedLaneCommand(completionRequest.Command, "feature-analysis")
	if selectedCompletionCommand == "" {
		t.Fatalf("completion request could not be projected to the exact selected lane: %+v", completionRequest)
	}
	var statusCompletionPreview completionProductResult
	runCompletionJSON(t, &out, rekitCommandCLIArgs(t, selectedCompletionCommand), &statusCompletionPreview)
	completionStep := runCurrentStepPreview(t, reviewerBase)
	if completionStep.Route != "case" || completionStep.DriverStep == nil || completionStep.DriverStep.CurrentDriverRequest.Command != selectedCompletionCommand || statusCompletionPreview.Blocked || statusCompletionPreview.CompletionPlanSHA256 == "" || completionStep.DriverStep.ExpectedDriverStepPreviewSHA256 != statusCompletionPreview.CompletionPlanSHA256 || completionStep.DriverStep.ExpectedDriverStepPlanSHA256 == "" || completionStep.ExpectedCurrentStepPlanSHA256 == "" {
		t.Fatalf("bounded current-step did not preserve completion double hash: current=%q selected=%q driverCurrent=%q driverPreviewHash=%q directPreviewHash=%q driverPreview=%+v", completionRequest.Command, selectedCompletionCommand, completionStep.DriverStep.CurrentDriverRequest.Command, completionStep.DriverStep.ExpectedDriverStepPreviewSHA256, statusCompletionPreview.CompletionPlanSHA256, completionStep.DriverStep.PreviewResult)
	}
	laneRoot := filepath.Join(caseRoot, ".rekit", "lanes", "feature-analysis")
	lanePath := filepath.Join(laneRoot, "lane.json")
	boardPath := filepath.Join(caseRoot, ".rekit", "board.json")
	eventsPath := filepath.Join(laneRoot, "events.jsonl")
	laneBeforeIntentDrift, err := os.ReadFile(lanePath)
	if err != nil {
		t.Fatal(err)
	}
	boardBeforeIntentDrift, err := os.ReadFile(boardPath)
	if err != nil {
		t.Fatal(err)
	}
	eventsBeforeIntentDrift, err := os.ReadFile(eventsPath)
	if err != nil {
		t.Fatal(err)
	}
	packetBytes, err := os.ReadFile(reviewerPlan.PacketPath)
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(reviewerPlan.PacketPath, append(packetBytes, ' '), 0o600); err != nil {
		t.Fatal(err)
	}
	if _, accepted, acceptanceErr := workstream.CurrentMemberManifestReviewerAcceptance(caseRoot, "feature-analysis", reviewerItem); acceptanceErr == nil || accepted {
		t.Fatalf("canonical acceptance survived reviewer packet drift: accepted=%t err=%v", accepted, acceptanceErr)
	}
	packetDrift := previewCompletion(t, &out, caseRoot, "analysis", status.MemberExecution.CompletionEvidence[0])
	if !packetDrift.Blocked || !hasCompletionBlocker(packetDrift.Blockers, "member-manifest-review") || packetDrift.CompletionPlanSHA256 != "" {
		t.Fatalf("completion survived reviewer packet integrity drift: %+v", packetDrift)
	}
	if err := os.WriteFile(reviewerPlan.PacketPath, packetBytes, 0o600); err != nil {
		t.Fatal(err)
	}
	restoreAfterIntent := workstream.SetCompletionAfterIntentHookForTest(func() error {
		return os.WriteFile(inputPath, append(append([]byte{}, inputBytes...), ' '), 0o600)
	})
	out.Reset()
	err = Run(append(rekitCommandCLIArgs(t, readyCompletion.ApplyCommand), "-Target", caseRoot, "-Pack", "_template"), &out)
	restoreAfterIntent()
	if err == nil || (!strings.Contains(err.Error(), "reviewer") && !strings.Contains(err.Error(), "blocked") && !strings.Contains(err.Error(), "mismatch")) {
		t.Fatalf("completion Apply survived post-intent reviewer input drift: %v", err)
	}
	if _, statErr := os.Stat(filepath.Join(laneRoot, "completion.intent.json")); statErr != nil {
		t.Fatalf("post-intent drift did not preserve recoverable intent: %v", statErr)
	}
	if _, statErr := os.Stat(filepath.Join(laneRoot, "completion.json")); !os.IsNotExist(statErr) {
		t.Fatalf("post-intent drift published completion commit: %v", statErr)
	}
	for path, want := range map[string][]byte{lanePath: laneBeforeIntentDrift, boardPath: boardBeforeIntentDrift, eventsPath: eventsBeforeIntentDrift} {
		got, readErr := os.ReadFile(path)
		if readErr != nil || !bytes.Equal(got, want) {
			t.Fatalf("post-intent drift published %s: err=%v", path, readErr)
		}
	}
	if err := os.WriteFile(inputPath, inputBytes, 0o600); err != nil {
		t.Fatal(err)
	}
	beforeCommitCalls := 0
	restoreBeforeCommit := workstream.SetCompletionBeforeCommitHookForTest(func() error {
		beforeCommitCalls++
		if beforeCommitCalls == 1 {
			return os.WriteFile(inputPath, append(append([]byte{}, inputBytes...), ' '), 0o600)
		}
		return nil
	})
	out.Reset()
	err = Run(append(rekitCommandCLIArgs(t, readyCompletion.ApplyCommand), "-Target", caseRoot, "-Pack", "_template"), &out)
	if err == nil || !strings.Contains(err.Error(), "reviewer lineage changed before final commit") {
		t.Fatalf("completion final commit survived reviewer input drift: %v", err)
	}
	if _, statErr := os.Stat(filepath.Join(laneRoot, "completion.json")); !os.IsNotExist(statErr) {
		t.Fatalf("publication-final drift wrote completion commit: %v", statErr)
	}
	publishedLane, readErr := os.ReadFile(lanePath)
	if readErr != nil || bytes.Equal(publishedLane, laneBeforeIntentDrift) || !bytes.Contains(publishedLane, []byte(`"status": "closed"`)) {
		t.Fatalf("publication-final drift did not preserve truthful closed-lane prefix: err=%v", readErr)
	}
	if publishedEvents, readErr := os.ReadFile(eventsPath); readErr != nil || bytes.Equal(publishedEvents, eventsBeforeIntentDrift) || !bytes.Contains(publishedEvents, []byte(`"kind":"lane-completed"`)) {
		t.Fatalf("publication-final drift did not preserve truthful lane event prefix: err=%v", readErr)
	}
	if err := os.WriteFile(inputPath, inputBytes, 0o600); err != nil {
		t.Fatal(err)
	}
	var completed completionProductResult
	out.Reset()
	if err := Run(append(rekitCommandCLIArgs(t, readyCompletion.ApplyCommand), "-Target", caseRoot, "-Pack", "_template"), &out); err != nil {
		t.Fatal(err)
	}
	restoreBeforeCommit()
	if err := json.Unmarshal(out.Bytes(), &completed); err != nil {
		t.Fatal(err)
	}
	if !completed.Applied || completed.CompletionReceipt == nil || len(completed.CompletionReceipt.Evidence) != 3 || !strings.EqualFold(completed.CompletionReceipt.Evidence[0].SHA256, returnedApply.MemberExecution.Inspection.ManifestSHA256) {
		t.Fatalf("review-backed completion did not bind member manifest, outputs, and reviewer input: %+v", completed)
	}
	if err := os.Remove(inputPath); err != nil {
		t.Fatal(err)
	}
	completedLane := completed.CompletionReceipt.Lane
	if _, err := workstream.InspectLaneCompletion(caseRoot, completedLane); err == nil || (!strings.Contains(err.Error(), "evidence content mismatch") && !strings.Contains(err.Error(), "reviewer lineage")) {
		t.Fatalf("lane completion survived canonical reviewer input deletion: %v", err)
	}
	var reviewerInputTerminal statusInventory
	out.Reset()
	if err := Run([]string{"-Command", "status", "-Target", caseRoot, "-Pack", "_template", "-Format", "json"}, &out); err == nil {
		_ = json.Unmarshal(out.Bytes(), &reviewerInputTerminal)
		if reviewerInputTerminal.CaseMission != nil && reviewerInputTerminal.CaseMission.MissionCompletion != nil && reviewerInputTerminal.CaseMission.MissionCompletion.OperationallyComplete {
			t.Fatalf("terminal status survived canonical reviewer input deletion: %+v", reviewerInputTerminal.CaseMission.MissionCompletion)
		}
	}
	if err := os.WriteFile(inputPath, append(append([]byte{}, inputBytes...), ' '), 0o600); err != nil {
		t.Fatal(err)
	}
	if _, err := workstream.InspectLaneCompletion(caseRoot, completedLane); err == nil || (!strings.Contains(err.Error(), "evidence content mismatch") && !strings.Contains(err.Error(), "reviewer lineage")) {
		t.Fatalf("lane completion survived canonical reviewer input replacement: %v", err)
	}
	if err := os.WriteFile(inputPath, inputBytes, 0o600); err != nil {
		t.Fatal(err)
	}
	canonicalOutput := filepath.Join(returnedApply.MemberExecution.Inspection.OutputsRoot, "review-items.json")
	if err := os.WriteFile(canonicalOutput, []byte("tampered\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	if _, err := workstream.InspectLaneCompletion(caseRoot, completedLane); err == nil || !strings.Contains(err.Error(), "evidence content mismatch") {
		t.Fatalf("lane completion survived canonical member output replacement: %v", err)
	}
	var terminal statusInventory
	out.Reset()
	if err := Run([]string{"-Command", "status", "-Target", caseRoot, "-Pack", "_template", "-Format", "json"}, &out); err == nil {
		_ = json.Unmarshal(out.Bytes(), &terminal)
		if terminal.CaseMission != nil && terminal.CaseMission.MissionCompletion != nil && terminal.CaseMission.MissionCompletion.OperationallyComplete {
			t.Fatalf("terminal status survived canonical member output replacement: %+v", terminal.CaseMission.MissionCompletion)
		}
	}
	if err := os.Remove(canonicalOutput); err != nil {
		t.Fatal(err)
	}
	if _, err := workstream.InspectLaneCompletion(caseRoot, completedLane); err == nil || !strings.Contains(err.Error(), "evidence content mismatch") {
		t.Fatalf("lane completion survived canonical member output deletion: %v", err)
	}
}

func TestRunCurrentStepMemberExecutionRefreshFailurePreservesAppliedReceipt(t *testing.T) {
	caseRoot := filepath.Join(t.TempDir(), "member-refresh-failure")
	var out bytes.Buffer
	baseOnboard := []string{"-Command", "onboard", "-Target", caseRoot, "-Pack", "_template", "-ProjectName", "member-refresh-failure", "-Goal", "prove truthful member refresh failure", "-Actor", "operator", "-Executor", "member-session-a", "-InitialLane", "feature-analysis"}
	var onboard onboardCLIPlan
	runCompletionJSON(t, &out, append(append([]string{}, baseOnboard...), "-WhatIf", "-Format", "json"), &onboard)
	runCompletionJSON(t, &out, onboard.ApplyArgs, nil)
	var status struct {
		MissionControlRunbook *statusMissionControlRunbookSnapshot `json:"missionControlRunbook"`
	}
	runCompletionJSON(t, &out, []string{"-Command", "status", "-Target", caseRoot, "-Pack", "_template", "-Format", "json"}, &status)
	overviewArgs, _ := missionCommanderDriverRequestCommandCLIArgs(t, status.MissionControlRunbook.Quickstart.CurrentDriverRequest)
	runCompletionJSON(t, &out, overviewArgs, nil)
	runCompletionJSON(t, &out, []string{"-Command", "status", "-Target", caseRoot, "-Pack", "_template", "-Format", "json"}, &status)
	startArgs, _ := missionCommanderDriverRequestCommandCLIArgs(t, status.MissionControlRunbook.Quickstart.CurrentDriverRequest)
	var startPreview struct {
		MissionCommanderActionQueue missionCommanderActionQueueSnapshot `json:"missionCommanderActionQueue"`
	}
	runCompletionJSON(t, &out, startArgs, &startPreview)
	startApplyArgs, _ := missionCommanderDriverRequestCommandCLIArgs(t, startPreview.MissionCommanderActionQueue.CurrentDriverRequest)
	runCompletionJSON(t, &out, append(startApplyArgs, "-Target", caseRoot, "-Pack", "_template", "-Format", "json"), nil)
	preview := runMemberCurrentStepForLane(t, caseRoot, "feature-analysis", []string{"-WhatIf"})
	if preview.MemberExecution == nil {
		t.Fatalf("feature-analysis member dispatch plan missing: %+v", preview)
	}
	currentStepBeforeStatusRefreshHook = func(command string) error {
		if command != commands.RunCurrentStep {
			t.Fatalf("unexpected member refresh hook command: %s", command)
		}
		return errors.New("injected member status refresh failure")
	}
	t.Cleanup(func() { currentStepBeforeStatusRefreshHook = nil })
	out.Reset()
	err := Run([]string{"-Command", "run-current-step", "-Target", caseRoot, "-Pack", "_template", "-Lane", "feature-analysis", "-ExpectedMemberExecutionPlanSha256", preview.MemberExecution.ExpectedPlanSHA256, "-ExpectedCurrentStepPlanSha256", preview.ExpectedCurrentStepPlanSHA256, "-Apply", "-Format", "json"}, &out)
	if err == nil || !strings.Contains(err.Error(), "injected member status refresh failure") {
		t.Fatalf("public member refresh failure did not return original error: %v", err)
	}
	var applied currentStepTestPlan
	if decodeErr := json.Unmarshal(out.Bytes(), &applied); decodeErr != nil {
		t.Fatalf("public member refresh failure omitted decodable partial JSON: %v\n%s", decodeErr, out.String())
	}
	if !applied.Applied || applied.RefreshedStatus != nil || applied.Receipt == nil || applied.Receipt.State != "refresh-failed" || applied.Receipt.Outcome != "current-step-applied-status-refresh-failed" || applied.Receipt.RefreshedCurrentDriverRequest != nil {
		t.Fatalf("member refresh failure partial result=%+v err=%v", applied, err)
	}
	if latest, found, inspectErr := memberexecution.Latest(caseRoot, "feature-analysis"); inspectErr != nil || !found || latest.State != "handoff-ready" {
		t.Fatalf("member mutation was not durably applied: latest=%+v found=%v err=%v", latest, found, inspectErr)
	}
}

func TestMemberExecutionOldGenerationLosesToPublicReconcileLease(t *testing.T) {
	caseRoot := filepath.Join(t.TempDir(), "member-reconcile-race")
	var out bytes.Buffer
	baseOnboard := []string{"-Command", "onboard", "-Target", caseRoot, "-Pack", "_template", "-ProjectName", "member-reconcile-race", "-Goal", "prove shared member and reconcile lease", "-Actor", "operator", "-Executor", "member-session-a", "-InitialLane", "feature-analysis"}
	var onboard onboardCLIPlan
	runCompletionJSON(t, &out, append(append([]string{}, baseOnboard...), "-WhatIf", "-Format", "json"), &onboard)
	runCompletionJSON(t, &out, onboard.ApplyArgs, nil)
	var journeyStatus struct {
		MissionControlRunbook *statusMissionControlRunbookSnapshot `json:"missionControlRunbook"`
	}
	runCompletionJSON(t, &out, []string{"-Command", "status", "-Target", caseRoot, "-Pack", "_template", "-Format", "json"}, &journeyStatus)
	overviewArgs, _ := missionCommanderDriverRequestCommandCLIArgs(t, journeyStatus.MissionControlRunbook.Quickstart.CurrentDriverRequest)
	runCompletionJSON(t, &out, overviewArgs, nil)
	runCompletionJSON(t, &out, []string{"-Command", "status", "-Target", caseRoot, "-Pack", "_template", "-Format", "json"}, &journeyStatus)
	startArgs, _ := missionCommanderDriverRequestCommandCLIArgs(t, journeyStatus.MissionControlRunbook.Quickstart.CurrentDriverRequest)
	var startPreview struct {
		MissionCommanderActionQueue missionCommanderActionQueueSnapshot `json:"missionCommanderActionQueue"`
	}
	runCompletionJSON(t, &out, startArgs, &startPreview)
	startApplyArgs, _ := missionCommanderDriverRequestCommandCLIArgs(t, startPreview.MissionCommanderActionQueue.CurrentDriverRequest)
	runCompletionJSON(t, &out, append(startApplyArgs, "-Target", caseRoot, "-Pack", "_template", "-Format", "json"), nil)

	memberPlan := runCurrentStepPreview(t, []string{"-Command", "run-current-step", "-Target", caseRoot, "-Pack", "_template", "-Lane", "feature-analysis", "-WhatIf", "-Format", "json"})
	if memberPlan.MemberExecution == nil {
		t.Fatalf("member dispatch plan missing: %+v", memberPlan)
	}
	notePreviewArgs := []string{"-Command", "note", "-Target", caseRoot, "-Pack", "_template", "-Kind", "intervention", "-Lane", "feature-analysis", "-EventId", "member-reconcile-race", "-Subject", "replace stale executor", "-Summary", "replace member executor while dispatch is pending", "-Action", "override", "-Status", "open", "-Actor", "operator", "-WhatIf", "-Format", "json"}
	var notePreview struct {
		RecordCommand string `json:"recordCommand"`
	}
	runCompletionJSON(t, &out, notePreviewArgs, &notePreview)
	runCompletionJSON(t, &out, rekitCommandCLIArgs(t, notePreview.RecordCommand), nil)

	leaseHeld := make(chan struct{})
	releaseLease := make(chan struct{})
	restore := workstream.SetReconcileLeaseHookForTest(func(_, lane string) error {
		if lane != "feature-analysis" {
			return nil
		}
		close(leaseHeld)
		<-releaseLease
		return nil
	})
	t.Cleanup(restore)
	reconcileDone := make(chan error, 1)
	go func() {
		var reconcileOut bytes.Buffer
		reconcileDone <- Run([]string{"-Command", "reconcile", "analysis", "-Target", caseRoot, "-Pack", "_template", "-InterventionId", "member-reconcile-race", "-Executor", "member-session-b", "-Actor", "operator", "-Reason", "replace stale member generation", "-Apply", "-Format", "json"}, &reconcileOut)
	}()
	<-leaseHeld
	memberDone := make(chan error, 1)
	go func() {
		var memberOut bytes.Buffer
		memberDone <- Run([]string{"-Command", "run-current-step", "-Target", caseRoot, "-Pack", "_template", "-Lane", "feature-analysis", "-ExpectedMemberExecutionPlanSha256", memberPlan.MemberExecution.ExpectedPlanSHA256, "-ExpectedCurrentStepPlanSha256", memberPlan.ExpectedCurrentStepPlanSHA256, "-Apply", "-Format", "json"}, &memberOut)
	}()
	close(releaseLease)
	if err := <-reconcileDone; err != nil {
		t.Fatalf("public reconcile failed: %v", err)
	}
	if err := <-memberDone; err == nil || (!strings.Contains(err.Error(), "changed") && !strings.Contains(err.Error(), "stale") && !strings.Contains(err.Error(), "mismatch") && !strings.Contains(err.Error(), "not an effective open intervention")) {
		t.Fatalf("old member generation survived public reconcile: %v", err)
	}
	if _, found, err := memberexecution.Latest(caseRoot, "feature-analysis"); err != nil || found {
		t.Fatalf("reconcile race corrupted or published old member execution: found=%v err=%v", found, err)
	}
}

func TestCurrentStepPackMemoryConsumerRequestUsesCaseQueueAndStrictGuard(t *testing.T) {
	original := currentStepValidatePackMemoryConsumerTask
	t.Cleanup(func() { currentStepValidatePackMemoryConsumerTask = original })
	globalRequest := &mission.MissionCommanderDriverRequest{Lane: "", RunLoopStepID: "preview-current", Command: "/rekit sync -WhatIf"}
	caseRequest := &mission.MissionCommanderDriverRequest{Lane: "feature-mission", RunLoopStepID: "preview-current", Command: "/rekit continue mission -WhatIf"}
	status := statusInventory{
		MissionControlRunbook: &statusMissionControlRunbook{Scope: "pack-memory", CurrentDriverRequest: globalRequest},
		CaseMission:           &statusCaseMission{MissionCommanderActionQueue: mission.MissionCommanderActionQueue{CurrentDriverRequest: caseRequest}},
	}
	var gotLane string
	currentStepValidatePackMemoryConsumerTask = func(repoRoot, caseRoot, pack, lane string) error {
		gotLane = lane
		return nil
	}
	request, err := currentStepPackMemoryConsumerRequest("repo", "case", "_template", status)
	if err != nil || request == caseRequest || request.Lane != "feature-mission" || request.Command != caseRequest.Command || gotLane != "feature-mission" {
		t.Fatalf("consumer request=%+v lane=%q err=%v", request, gotLane, err)
	}

	currentStepValidatePackMemoryConsumerTask = func(_, _, _, _ string) error { return errors.New("receipt drift") }
	if _, err := currentStepPackMemoryConsumerRequest("repo", "case", "_template", status); err == nil || !strings.Contains(err.Error(), "receipt drift") {
		t.Fatalf("consumer request accepted strict guard failure: %v", err)
	}
	status.MissionControlRunbook.Scope = "project"
	if _, err := currentStepPackMemoryConsumerRequest("repo", "case", "_template", status); err == nil || !strings.Contains(err.Error(), "no case member request") {
		t.Fatalf("ordinary non-case focus entered consumer route: %v", err)
	}
}

func TestBuildCurrentStepCaseScopePackMemoryConsumerUsesStrictGuard(t *testing.T) {
	caseRoot := fullAttachedCase(t)
	if err := Run([]string{"-Command", "overview", "-Target", caseRoot, "-Pack", "_template", "-Format", "json"}, &bytes.Buffer{}); err != nil {
		t.Fatal(err)
	}
	board, err := mission.ReadBoard(caseRoot)
	if err != nil {
		t.Fatal(err)
	}
	board.Lanes[0].CurrentExecutor = "consumer-member"
	board.Lanes[0].ExecutorGeneration = 1
	board.Lanes[0].UpdatedAt = "2026-08-08T02:00:00Z"
	boardData, err := json.MarshalIndent(board, "", "  ")
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(caseRoot, ".rekit", "board.json"), append(boardData, '\n'), 0o644); err != nil {
		t.Fatal(err)
	}
	if _, _, err := memberexecution.WriteTaskBinding(caseRoot, board.Lanes[0].ID, memberexecution.TaskBinding{
		Kind: "pack-memory-consumer",
		Values: map[string]string{
			"changeId": "change", "sourceSha256": strings.Repeat("a", 64),
			"receiptSha256": strings.Repeat("b", 64), "planSha256": strings.Repeat("c", 64),
		},
	}); err != nil {
		t.Fatal(err)
	}

	originalValidate := currentStepValidatePackMemoryConsumerTask
	originalLease := currentStepWithPackMemoryConsumerTaskLease
	validated, leased := false, false
	currentStepValidatePackMemoryConsumerTask = func(_, target, pack, lane string) error {
		validated = target == caseRoot && pack == "_template" && lane == board.Lanes[0].ID
		return nil
	}
	currentStepWithPackMemoryConsumerTaskLease = func(_, target, pack, lane string, apply func() error) error {
		leased = target == caseRoot && pack == "_template" && lane == board.Lanes[0].ID
		return apply()
	}
	t.Cleanup(func() {
		currentStepValidatePackMemoryConsumerTask = originalValidate
		currentStepWithPackMemoryConsumerTaskLease = originalLease
	})
	preview := runCurrentStepPreview(t, []string{"-Command", "run-current-step", "-Target", caseRoot, "-Pack", "_template", "-Lane", "main", "-WhatIf", "-Format", "json"})
	if !validated || preview.MemberExecution == nil {
		t.Fatalf("case-scope consumer omitted strict preview validation: validated=%t preview=%+v", validated, preview)
	}
	applied := runMemberCurrentStep(t, caseRoot, []string{
		"-ExpectedMemberExecutionPlanSha256", preview.MemberExecution.ExpectedPlanSHA256,
		"-ExpectedCurrentStepPlanSha256", preview.ExpectedCurrentStepPlanSHA256,
		"-Apply",
	})
	if !applied.Applied || !leased {
		t.Fatalf("case-scope consumer bypassed strict Apply lease: leased=%t applied=%+v", leased, applied)
	}
}

func TestCaseScopePackMemoryConsumerDispatchPublishesFocusedExternalSession(t *testing.T) {
	caseRoot := fullAttachedCase(t)
	if err := Run([]string{"-Command", "overview", "-Target", caseRoot, "-Pack", "_template", "-Format", "json"}, &bytes.Buffer{}); err != nil {
		t.Fatal(err)
	}
	board, err := mission.ReadBoard(caseRoot)
	if err != nil {
		t.Fatal(err)
	}
	board.Lanes[0].CurrentExecutor = "consumer-member"
	board.Lanes[0].ExecutorGeneration = 1
	board.Lanes[0].UpdatedAt = "2026-08-08T02:00:00Z"
	boardData, err := json.MarshalIndent(board, "", "  ")
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(caseRoot, ".rekit", "board.json"), append(boardData, '\n'), 0o644); err != nil {
		t.Fatal(err)
	}
	if _, _, err := memberexecution.WriteTaskBinding(caseRoot, board.Lanes[0].ID, memberexecution.TaskBinding{
		Kind: "pack-memory-consumer",
		Values: map[string]string{
			"changeId": "change", "sourceSha256": strings.Repeat("a", 64),
			"receiptSha256": strings.Repeat("b", 64), "planSha256": strings.Repeat("c", 64),
		},
	}); err != nil {
		t.Fatal(err)
	}

	originalValidate := currentStepValidatePackMemoryConsumerTask
	originalLease := currentStepWithPackMemoryConsumerTaskLease
	currentStepValidatePackMemoryConsumerTask = func(_, _, _, _ string) error { return nil }
	currentStepWithPackMemoryConsumerTaskLease = func(_, _, _, _ string, apply func() error) error { return apply() }
	t.Cleanup(func() {
		currentStepValidatePackMemoryConsumerTask = originalValidate
		currentStepWithPackMemoryConsumerTaskLease = originalLease
	})

	lane := board.Lanes[0].ID
	loop := runCurrentLoopPreviewWith(t, caseRoot, 2, "-Lane", lane)
	if loop.InitialCurrentStep == nil || loop.InitialCurrentStep.MemberExecution == nil {
		t.Fatalf("consumer loop preview omitted member dispatch: %+v", loop)
	}
	applied := runCurrentLoopApplyWith(
		t,
		caseRoot,
		loop,
		"-Lane",
		lane,
		"-ExpectedMemberExecutionPlanSha256",
		loop.InitialCurrentStep.MemberExecution.ExpectedPlanSHA256,
	)
	if !applied.Applied || applied.SegmentCheckpoint == nil || !applied.SegmentCheckpoint.Ready || applied.SegmentCheckpoint.State != "ready" || applied.StopReason.Code != "external-member-handoff" {
		t.Fatalf("consumer dispatch omitted ready checkpoint: %+v", applied)
	}

	ctx, err := rekitruntime.New(caseRoot, "_template")
	if err != nil {
		t.Fatal(err)
	}
	status, err := buildInvocationStatusInventory(ctx, Options{
		Command:             commands.Status,
		SelectedCurrentLane: lane,
	})
	if err != nil {
		t.Fatal(err)
	}
	request := status.MissionControlRunbook.CurrentDriverRequest
	if request == nil || request.Source != "current-step-external-session" || request.Lane != board.Lanes[0].ID || status.MissionControlRunbook.CurrentLoopOperator == nil || status.MissionControlRunbook.CurrentLoopOperator.ExternalSessionJob == nil {
		t.Fatalf("consumer status repeated member dispatch instead of focusing external session: request=%+v segment=%+v operator=%+v", request, status.MissionControlRunbook.CurrentLoopSegment, status.MissionControlRunbook.CurrentLoopOperator)
	}
}

func TestBuildCurrentStepStrictPackMemoryConsumerUsesCheckpointExternalSessionWrapper(t *testing.T) {
	caseRoot := fullAttachedCase(t)
	if err := Run([]string{"-Command", "overview", "-Target", caseRoot, "-Pack", "_template", "-Format", "json"}, &bytes.Buffer{}); err != nil {
		t.Fatal(err)
	}
	board, err := mission.ReadBoard(caseRoot)
	if err != nil {
		t.Fatal(err)
	}
	board.Lanes[0].CurrentExecutor = "consumer-member"
	board.Lanes[0].ExecutorGeneration = 1
	board.Lanes[0].UpdatedAt = "2026-08-08T02:00:00Z"
	boardData, err := json.MarshalIndent(board, "", "  ")
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(caseRoot, ".rekit", "board.json"), append(boardData, '\n'), 0o644); err != nil {
		t.Fatal(err)
	}

	lane := board.Lanes[0].ID
	loop := runCurrentLoopPreviewWith(t, caseRoot, 3, "-Lane", lane)
	if loop.InitialCurrentStep == nil || loop.InitialCurrentStep.MemberExecution == nil {
		t.Fatalf("member loop preview omitted dispatch: %+v", loop)
	}
	memberPlan := loop.InitialCurrentStep.MemberExecution
	applied := runCurrentLoopApplyWith(t, caseRoot, loop, "-Lane", lane, "-ExpectedMemberExecutionPlanSha256", memberPlan.ExpectedPlanSHA256)
	if applied.SegmentCheckpoint == nil || !applied.SegmentCheckpoint.Ready || applied.StopReason.Code != "external-member-handoff" {
		t.Fatalf("member loop did not publish ready external checkpoint: %+v", applied)
	}

	ctx, err := rekitruntime.New(caseRoot, "_template")
	if err != nil {
		t.Fatal(err)
	}
	status, err := buildInvocationStatusInventory(ctx, Options{
		Command:             commands.Status,
		SelectedCurrentLane: lane,
	})
	if err != nil {
		t.Fatal(err)
	}
	caseRequest := status.MissionControlRunbook.CurrentLoopOperator.SourceCurrentDriverRequest
	if caseRequest == nil || caseRequest.Lane == "" {
		t.Fatalf("status omitted selected checkpoint source request: %+v", status.MissionControlRunbook.CurrentLoopOperator)
	}
	status.CaseMission.MissionCommanderActionQueue.CurrentDriverRequest = caseRequest
	globalRequest := missionDriverRequestForTest(`/rekit sync -SelectPackMemoryChange change -WhatIf -Format json`)
	status.MissionControlRunbook.Scope = "pack-memory"
	status.MissionControlRunbook.CurrentDriverRequest = &globalRequest

	original := currentStepValidatePackMemoryConsumerTask
	currentStepValidatePackMemoryConsumerTask = func(repoRoot, target, pack, lane string) error {
		if repoRoot != ctx.RepoRoot || target != caseRoot || pack != "_template" || lane != caseRequest.Lane {
			t.Fatalf("strict consumer binding drifted: repo=%s target=%s pack=%s lane=%s", repoRoot, target, pack, lane)
		}
		return nil
	}
	t.Cleanup(func() { currentStepValidatePackMemoryConsumerTask = original })

	plan, err := buildCurrentStepPlanFromStatus(ctx, Options{WhatIf: true, Pack: "_template"}, status)
	if err != nil {
		t.Fatal(err)
	}
	if plan.Route != "case" || plan.MemberExecution != nil || plan.ExternalSessionStep == nil || plan.CurrentDriverRequest.Source != "current-step-external-session" || plan.CurrentDriverRequest.Lane != caseRequest.Lane {
		t.Fatalf("strict consumer checkpoint repeated member dispatch instead of selecting external session: plan=%+v segment=%+v operator=%+v", plan, status.MissionControlRunbook.CurrentLoopSegment, status.MissionControlRunbook.CurrentLoopOperator)
	}
	if plan.ExternalSessionStep.Mode != "attempt-input" || len(plan.ExternalSessionStep.InputRequired) == 0 {
		t.Fatalf("strict consumer external session preview omitted launch input contract: %+v", plan.ExternalSessionStep)
	}
}

func TestBuildCurrentStepStrictConsumerExternalTurnUsesCheckpointSourceRequest(t *testing.T) {
	caseRoot := fullAttachedCase(t)
	if err := Run([]string{"-Command", "overview", "-Target", caseRoot, "-Pack", "_template", "-Format", "json"}, &bytes.Buffer{}); err != nil {
		t.Fatal(err)
	}
	ctx, err := rekitruntime.New(caseRoot, "_template")
	if err != nil {
		t.Fatal(err)
	}
	const lane = "main"
	caseRequest := missionDriverRequestForTest(`/rekit continue -Lane main -WhatIf -Format json`)
	caseRequest.Lane = lane
	caseRequest.Source = "missionCommanderActions"
	caseRequest.State = "ready-to-continue"
	wrapper := missionDriverRequestForTest(`/rekit run-current-step -Target case -Lane main -WhatIf -Format json`)
	wrapper.Lane = lane
	wrapper.Source = "current-step-external-session"
	status := statusInventory{
		TemplateRoot: ctx.RepoRoot, Target: caseRoot, Pack: "_template",
		MissionControlRunbook: &statusMissionControlRunbook{
			Scope:                "pack-memory",
			CurrentDriverRequest: &wrapper,
			CurrentLoopSegment: &currentloop.Inspection{
				State: "ready", Ready: true,
				ResumeDriverRequest:           &wrapper,
				RefreshedCurrentDriverRequest: &caseRequest,
				Continuation:                  &currentloop.Continuation{},
			},
		},
		CaseMission: &statusCaseMission{MissionCommanderActionQueue: mission.MissionCommanderActionQueue{CurrentDriverRequest: &caseRequest}},
	}
	status.MissionControlRunbook.CurrentLoopOperator = statusCurrentLoopOperatorPackage(caseRoot, status.CaseMission, &statusMissionControlRunbook{Scope: "case", CurrentDriverRequest: &wrapper}, *status.MissionControlRunbook.CurrentLoopSegment)
	original := currentStepValidatePackMemoryConsumerTask
	currentStepValidatePackMemoryConsumerTask = func(_, _, _, lane string) error {
		if lane != caseRequest.Lane {
			t.Fatalf("strict consumer source validation lane=%q", lane)
		}
		return nil
	}
	t.Cleanup(func() { currentStepValidatePackMemoryConsumerTask = original })
	plan, err := buildCurrentStepPlanFromStatus(ctx, Options{WhatIf: true, Pack: "_template", currentLoopExternalTurnResume: true}, status)
	if err != nil {
		t.Fatal(err)
	}
	if plan.Route != "case" || plan.PackMemoryConsumerLane != caseRequest.Lane || plan.DriverStep == nil || driverStepCommandName(plan.CurrentDriverRequest.Command) != "continue" {
		t.Fatalf("strict consumer result-turn did not restore checkpoint source request under consumer lease: %+v", plan)
	}
}

func TestRunCurrentStepExternalRouteRejectsUnboundOuterInputs(t *testing.T) {
	caseRoot := fullAttachedCase(t)
	if err := Run([]string{"-Command", "overview", "-Target", caseRoot, "-Pack", "_template", "-Format", "json"}, &bytes.Buffer{}); err != nil {
		t.Fatal(err)
	}
	board, err := mission.ReadBoard(caseRoot)
	if err != nil {
		t.Fatal(err)
	}
	board.Lanes[0].CurrentExecutor = "outer-input-member"
	board.Lanes[0].ExecutorGeneration = 1
	boardData, _ := json.MarshalIndent(board, "", "  ")
	if err := os.WriteFile(filepath.Join(caseRoot, ".rekit", "board.json"), append(boardData, '\n'), 0o644); err != nil {
		t.Fatal(err)
	}
	lane := board.Lanes[0].ID
	loop := runCurrentLoopPreviewWith(t, caseRoot, 3, "-Lane", lane)
	runCurrentLoopApplyWith(t, caseRoot, loop, "-Lane", lane, "-ExpectedMemberExecutionPlanSha256", loop.InitialCurrentStep.MemberExecution.ExpectedPlanSHA256)
	for _, flag := range []string{"-Actor", "-ExpectedMemberExecutionPlanSha256"} {
		var out bytes.Buffer
		err := Run([]string{"-Command", "run-current-step", "-Target", caseRoot, "-Pack", "_template", "-Lane", lane, "-ExternalSessionHarness", "harness", "-ExternalSessionId", "session", "-ExternalSessionActor", "external-actor", "-ExternalSessionStartedAt", "2026-08-05T10:00:00Z", flag, "unexpected", "-WhatIf", "-Format", "json"}, &out)
		if err == nil || !strings.Contains(err.Error(), "external session route does not accept") {
			t.Fatalf("external route silently accepted %s: err=%v output=%s", flag, err, out.String())
		}
	}
}

func TestRunCurrentStepSelectedLaneOverridesGlobalCurrentLane(t *testing.T) {
	caseRoot := fullAttachedCase(t)

	var out bytes.Buffer
	if err := Run([]string{
		"-Command", "start",
		"-Target", caseRoot,
		"-Pack", "_template",
		"-Name", "login",
		"-Apply",
		"-Executor", "session-1",
		"-Actor", "mission-commander",
		"-Reason", "selected current lane regression",
	}, &out); err != nil {
		t.Fatal(err)
	}

	var globalOut bytes.Buffer
	err := Run([]string{
		"-Command", "run-current-step",
		"-Target", caseRoot,
		"-Pack", "_template",
		"-WhatIf", "-Format", "json",
	}, &globalOut)
	if err == nil || !strings.Contains(err.Error(), "requires -Lane from the current typed lane choices") {
		t.Fatalf("global multi-lane current step did not fail closed: err=%v output=%s", err, globalOut.String())
	}

	selected := runCurrentStepPreview(t, []string{
		"-Command", "run-current-step",
		"-Target", caseRoot,
		"-Pack", "_template",
		"-Lane", "main",
		"-WhatIf", "-Format", "json",
	})
	if selected.Route != "case" || selected.DriverStep == nil || selected.ReviewerStep != nil {
		t.Fatalf("unexpected selected-lane plan: %+v", selected)
	}
	request := selected.DriverStep.CurrentDriverRequest
	if request.Lane != "main" {
		t.Fatalf("selected current lane = %q, want main", request.Lane)
	}
	if !request.CommandExecutable || request.Blocked {
		t.Fatalf("selected request is not executable: %+v", request)
	}
	if !strings.Contains(request.Command, "-Lane main") {
		t.Fatalf("selected command does not preserve lane: %q", request.Command)
	}
	if selected.ExpectedCurrentStepPlanSHA256 == "" {
		t.Fatalf("selected plan hash is empty: %+v", selected)
	}

	applied := runCurrentStepApplyWithLane(t, caseRoot, selected, "main")
	if applied.Receipt == nil || applied.Receipt.RefreshedCurrentDriverRequest == nil || applied.Receipt.RefreshedCurrentDriverRequest.Lane != "main" {
		t.Fatalf("selected Apply refreshed a different lane: %+v", applied.Receipt)
	}
	if applied.RefreshedStatus == nil || applied.RefreshedStatus.MissionControlRunbook == nil || applied.RefreshedStatus.MissionControlRunbook.CurrentDriverRequest == nil || applied.RefreshedStatus.MissionControlRunbook.CurrentDriverRequest.Lane != "main" {
		t.Fatalf("selected Apply refreshed status for a different lane: %+v", applied.RefreshedStatus)
	}
}

func TestRunCurrentStepRejectsExternalInputsOutsideExternalRoute(t *testing.T) {
	caseRoot := fullAttachedCase(t)
	if err := Run([]string{"-Command", "overview", "-Target", caseRoot, "-Pack", "_template", "-Format", "json"}, &bytes.Buffer{}); err != nil {
		t.Fatal(err)
	}
	var out bytes.Buffer
	err := Run([]string{"-Command", "run-current-step", "-Target", caseRoot, "-Pack", "_template", "-ExternalSessionActor", "unexpected", "-WhatIf", "-Format", "json"}, &out)
	if err == nil || !strings.Contains(err.Error(), "external session inputs require the focused external session route") {
		t.Fatalf("non-external route accepted external session input: err=%v output=%s", err, out.String())
	}
}

func runMemberCurrentStep(t *testing.T, caseRoot string, inputs []string) currentStepTestPlan {
	t.Helper()
	return runMemberCurrentStepForLane(t, caseRoot, "main", inputs)
}

func runMemberCurrentStepForLane(t *testing.T, caseRoot, lane string, inputs []string) currentStepTestPlan {
	t.Helper()
	args := []string{
		"-Command", "run-current-step",
		"-Target", caseRoot,
		"-Pack", "_template",
		"-Lane", lane,
	}
	args = append(args, inputs...)
	args = append(args, "-Format", "json")
	return runCurrentStepPreview(t, args)
}

func TestSplitPublicCommandPreservesQuotedArguments(t *testing.T) {
	command := `/rekit plan-subagents -Target "C:\case root" -Pack ` + defaults.DefaultPack + ` -Items "manifest ref" -Format json`
	got, err := SplitPublicCommand(command)
	if err != nil {
		t.Fatal(err)
	}
	want := []string{"-Command", "plan-subagents", "-Target", `C:\case root`, "-Pack", defaults.DefaultPack, "-Items", "manifest ref", "-Format", "json"}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("public command args = %q, want %q", got, want)
	}
	if _, err := SplitPublicCommand(`plan-subagents -Format json`); err == nil {
		t.Fatal("command without /rekit prefix was accepted")
	}
}

func TestMemberReviewerItemsArgsUsesCompactManifestReference(t *testing.T) {
	manifestRef := ".rekit/lanes/feature-analysis/member-executions/g000002-a000001/result/manifest.json"
	if got := memberReviewerItemsArgs(manifestRef); !reflect.DeepEqual(got, []string{"-Items", manifestRef}) {
		t.Fatalf("reviewer items args = %q", got)
	}
}

func sha256Text(data []byte) string {
	sum := sha256.Sum256(data)
	return hex.EncodeToString(sum[:])
}

type currentStepTestPlan struct {
	Route                         string                                `json:"route"`
	Applied                       bool                                  `json:"applied"`
	CurrentDriverRequest          mission.MissionCommanderDriverRequest `json:"currentDriverRequest"`
	DriverStep                    *driverStepPlan                       `json:"driverStep"`
	ReviewerStep                  *reviewerStepPlan                     `json:"reviewerStep"`
	MemberExecution               *memberexecution.Plan                 `json:"memberExecution"`
	ExternalSessionStep           *currentStepExternalSessionPlan       `json:"externalSessionStep"`
	ExpectedCurrentStepPlanSHA256 string                                `json:"expectedCurrentStepPlanSha256"`
	Receipt                       *currentStepReceipt                   `json:"receipt"`
	RefreshedStatus               *statusInventory                      `json:"refreshedStatus"`
}

func runCurrentStepPreview(t *testing.T, args []string) currentStepTestPlan {
	t.Helper()
	var out bytes.Buffer
	if err := Run(args, &out); err != nil {
		t.Fatal(err)
	}
	var result currentStepTestPlan
	if err := json.Unmarshal(out.Bytes(), &result); err != nil {
		t.Fatalf("current-step JSON did not decode: %v\n%s", err, out.String())
	}
	return result
}

func runCurrentStepApply(t *testing.T, caseRoot string, preview currentStepTestPlan, inputs ...string) currentStepTestPlan {
	t.Helper()
	return runCurrentStepApplyWithLane(
		t,
		caseRoot,
		preview,
		preview.CurrentDriverRequest.Lane,
		inputs...,
	)
}

func runCurrentStepApplyWithLane(t *testing.T, caseRoot string, preview currentStepTestPlan, lane string, inputs ...string) currentStepTestPlan {
	t.Helper()
	var out bytes.Buffer
	args := []string{"-Command", "run-current-step", "-Target", caseRoot, "-Pack", "_template"}
	if strings.TrimSpace(lane) != "" {
		args = append(args, "-Lane", lane)
	}
	args = append(args, inputs...)
	args = append(args, "-ExpectedCurrentStepPlanSha256", preview.ExpectedCurrentStepPlanSHA256, "-Apply", "-Format", "json")
	if err := Run(args, &out); err != nil {
		t.Fatal(err)
	}
	var result currentStepTestPlan
	if err := json.Unmarshal(out.Bytes(), &result); err != nil {
		t.Fatalf("current-step apply JSON did not decode: %v\n%s", err, out.String())
	}
	if !result.Applied || result.Receipt == nil || result.Receipt.State != "refreshed" {
		t.Fatalf("current-step apply omitted refreshed receipt: %+v", result)
	}
	return result
}
