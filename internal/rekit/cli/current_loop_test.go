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
)

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
	if preview.Applied || preview.AppliedSteps != 0 || len(preview.Steps) != 0 {
		t.Fatalf("current-loop preview mutated: %+v", preview)
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
	if !applied.Applied || applied.AppliedSteps != 1 || len(applied.Steps) != 1 || applied.Steps[0].CurrentStepReceipt == nil || applied.Steps[0].CurrentStepReceipt.State != "refresh-failed" || applied.StopReason.Code != "error" || applied.StopReason.Phase != "refresh-status" || applied.FinalStatus != nil {
		t.Fatalf("applied step with refresh failure was not reported truthfully: %+v", applied)
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
	if external.ContinuationRequest == nil || external.ContinuationRequest.RemainingMaxSteps != 10 || external.ContinuationRequest.AppliedStepsInSegment != 0 || external.ContinuationRequest.WhatIfCommand != external.ResumeCommand || external.ContinuationRequest.CumulativeReceipts || len(external.ContinuationRequest.ObservationContract.Alternatives) != 1 {
		t.Fatalf("spawn handoff omitted typed continuation request: %+v", external.ContinuationRequest)
	}
	spawnObservation := external.ContinuationRequest.ObservationContract.Alternatives[0]
	if spawnObservation.Kind != "reviewer-session-accepted" || !slices.Equal(spawnObservation.RequiredFlags, []string{"-ReviewerHarness", "-ReviewerSession", "-Actor"}) {
		t.Fatalf("spawn continuation observation contract = %+v", spawnObservation)
	}
	out.Reset()
	err := Run([]string{"-Command", "run-current-loop", "-Target", caseRoot, "-Pack", "_template", "-MaxSteps", "10", "-ExpectedCurrentLoopPlanSha256", strings.Repeat("0", 64), "-Apply", "-Format", "json"}, &out)
	if err == nil || !strings.Contains(err.Error(), "external or reviewed action") {
		t.Fatalf("external reviewer handoff accepted Apply: %v", err)
	}

	actor := "mission-commander"
	dispatchInputs := []string{"-ReviewerHarness", "go-cli-test-harness", "-ReviewerSession", "reviewer-session-runner", "-Actor", actor}
	dispatchPreview := runCurrentLoopPreviewWith(t, caseRoot, external.ContinuationRequest.RemainingMaxSteps, dispatchInputs...)
	if dispatchPreview.ExpectedCurrentLoopPlanSHA256 == "" || dispatchPreview.InitialRoute != "reviewer" {
		t.Fatalf("reviewer deterministic loop preview missing: %+v", dispatchPreview)
	}
	out.Reset()
	driftInputs := []string{"-ReviewerHarness", "go-cli-test-harness", "-ReviewerSession", "reviewer-session-runner", "-Actor", "replacement-commander"}
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
	if continuation == nil || continuation.RemainingMaxSteps != dispatchPreview.MaxSteps-1 || continuation.AppliedStepsInSegment != 1 || continuation.SegmentMaxSteps != dispatchPreview.MaxSteps || continuation.CumulativeReceipts || !strings.Contains(continuation.WhatIfCommand, "-MaxSteps 9") {
		t.Fatalf("dispatch result omitted decremented continuation budget: %+v", continuation)
	}
	if dispatchApplied.StopReason.ExternalHandoff.ReviewerResultDropPathRole != "canonical-reviewer-input-destination" || dispatchApplied.StopReason.ExternalHandoff.ReviewerResultInputPath == "" || dispatchApplied.StopReason.ExternalHandoff.ReviewerResultDropPath != dispatchApplied.StopReason.ExternalHandoff.ReviewerResultInputPath {
		t.Fatalf("result handoff did not distinguish canonical input destination: %+v", dispatchApplied.StopReason.ExternalHandoff)
	}
	if len(continuation.ObservationContract.Alternatives) != 2 || continuation.ObservationContract.Alternatives[0].Kind != "reviewer-result-returned" || continuation.ObservationContract.Alternatives[1].Kind != "reviewer-session-failed" {
		t.Fatalf("result handoff observation alternatives = %+v", continuation.ObservationContract)
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
	resultInputs := []string{"-ReviewerResultInputSourcePath", resultSource, "-Actor", actor}
	resultPreview := runCurrentLoopPreviewWith(t, caseRoot, continuation.RemainingMaxSteps, resultInputs...)
	if resultPreview.ExpectedCurrentLoopPlanSHA256 == "" || resultPreview.InitialRoute != "reviewer" {
		t.Fatalf("typed result continuation did not rebuild a fresh preview: %+v", resultPreview)
	}
	resultApplied := runCurrentLoopApplyWith(t, caseRoot, resultPreview, resultInputs...)
	if !resultApplied.Applied || resultApplied.AppliedSteps != 6 || resultApplied.StopReason.Code != "route-policy" || resultApplied.FinalStatus == nil || resultApplied.FinalStatus.MissionControlRunbook == nil || resultApplied.FinalStatus.MissionControlRunbook.Scope != "case" {
		t.Fatalf("reviewer result continuation did not finish deterministic pipeline: %+v", resultApplied)
	}
	expectedSteps := []string{"save-result-input", "record-completion", "source-capture", "stage-candidate", "collect-result", "intake-results"}
	if resultApplied.ContinuationRequest != nil || len(resultApplied.Steps) != len(expectedSteps) {
		t.Fatalf("fresh continuation segment receipts or terminal state are invalid: %+v", resultApplied)
	}
	for index, expected := range expectedSteps {
		if resultApplied.Steps[index].Step != index+1 || resultApplied.Steps[index].RunLoopStepID != expected {
			t.Fatalf("reviewer continuation step %d = %+v, want %s", index+1, resultApplied.Steps[index], expected)
		}
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

func TestRunCurrentLoopStopsBeforeReconcileSurfacedByFirstStepRefresh(t *testing.T) {
	caseRoot := fullAttachedCase(t)
	var out bytes.Buffer
	if err := Run([]string{"-Command", "overview", "-Target", caseRoot, "-Pack", "_template", "-Format", "json"}, &out); err != nil {
		t.Fatal(err)
	}
	preview := runCurrentLoopPreview(t, caseRoot, 1)
	driverStepAfterPreviewValidationHook = func() error {
		appendCurrentLoopOpenIntervention(t, caseRoot, "int-current-loop-refresh")
		return nil
	}
	t.Cleanup(func() { driverStepAfterPreviewValidationHook = nil })
	applied := runCurrentLoopApply(t, caseRoot, preview)
	if applied.AppliedSteps != 1 || len(applied.Steps) != 1 || applied.StopReason.Code != "human-intervention" || applied.StopReason.Phase != "before-step" || applied.FinalStatus == nil || applied.FinalStatus.MissionControlRunbook == nil {
		t.Fatalf("refreshed intervention was not stopped before reconcile: %+v", applied)
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
	MaxSteps                      int                             `json:"maxSteps"`
	InitialRoute                  string                          `json:"initialRoute"`
	Applied                       bool                            `json:"applied"`
	AppliedSteps                  int                             `json:"appliedSteps"`
	InitialCurrentStep            *currentStepPlan                `json:"initialCurrentStep"`
	ExpectedCurrentLoopPlanSHA256 string                          `json:"expectedCurrentLoopPlanSha256"`
	Steps                         []currentLoopStepReceipt        `json:"steps"`
	StopReason                    currentLoopStopReason           `json:"stopReason"`
	ResumeCommand                 string                          `json:"resumeCommand"`
	ContinuationRequest           *currentLoopContinuationRequest `json:"continuationRequest"`
	FinalStatus                   *statusInventory                `json:"finalStatus"`
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
