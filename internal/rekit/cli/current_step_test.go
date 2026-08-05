package cli

import (
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"

	"github.com/shuiyu486/re-context-kits/internal/rekit/commands"
	"github.com/shuiyu486/re-context-kits/internal/rekit/memberexecution"
	"github.com/shuiyu486/re-context-kits/internal/rekit/mission"
	"github.com/shuiyu486/re-context-kits/internal/rekit/workstream"
)

func TestRunCurrentStepRoutesLaneThenReviewerLifecycle(t *testing.T) {
	caseRoot := fullAttachedCase(t)
	var out bytes.Buffer
	if err := Run([]string{"-Command", "overview", "-Target", caseRoot, "-Pack", "_template", "-Format", "json"}, &out); err != nil {
		t.Fatal(err)
	}
	base := []string{"-Command", "run-current-step", "-Target", caseRoot, "-Pack", "_template", "-WhatIf", "-Format", "json"}

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
	err = Run([]string{"-Command", "run-current-step", "-Target", caseRoot, "-Pack", "_template", "-Lane", "feature-review", "-WhatIf", "-Format", "json"}, &out)
	if err == nil || !strings.Contains(err.Error(), "unsupported flag") {
		t.Fatalf("unsupported current-step outer flag was not rejected: %v", err)
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
	err = Run([]string{"-Command", "run-current-step", "-Target", caseRoot, "-Pack", "_template", "-ExpectedCurrentStepPlanSha256", driftPreview.ExpectedCurrentStepPlanSHA256, "-Apply", "-Format", "json"}, &out)
	if err == nil || !strings.Contains(err.Error(), "expected plan sha256 mismatch") {
		t.Fatalf("valid current-step plan survived durable state drift: %v", err)
	}
}

func TestRunCurrentStepDurableMemberExecutionHandoffAndIntake(t *testing.T) {
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
	base := []string{"-Command", "run-current-step", "-Target", caseRoot, "-Pack", "_template", "-WhatIf", "-Format", "json"}
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
	apply := runMemberCurrentStep(t, caseRoot, []string{"-ExpectedMemberExecutionPlanSha256", memberHash, "-ExpectedCurrentStepPlanSha256", recoveredDispatch.ExpectedCurrentStepPlanSHA256, "-Apply"})
	if !apply.Applied || apply.MemberExecution == nil || apply.MemberExecution.Inspection.State != "handoff-ready" || apply.Receipt == nil || apply.Receipt.State != "refreshed" || apply.Receipt.Outcome != "current-step-applied" || apply.RefreshedStatus == nil || apply.Receipt.RefreshedCurrentDriverRequest == nil {
		t.Fatalf("member dispatch Apply mismatch: %+v", apply)
	}

	acceptedPreview := runMemberCurrentStep(t, caseRoot, []string{"-MemberExecutionAttemptId", attempt, "-MemberExecutionOutcome", "accepted", "-MemberExecutionObservedAt", "2026-08-03T02:01:00Z", "-Actor", "harness", "-WhatIf"})
	acceptedAuthorized := runMemberCurrentStep(t, caseRoot, []string{"-MemberExecutionAttemptId", attempt, "-MemberExecutionOutcome", "accepted", "-MemberExecutionObservedAt", "2026-08-03T02:01:00Z", "-Actor", "harness", "-ExpectedMemberExecutionPlanSha256", acceptedPreview.MemberExecution.ExpectedPlanSHA256, "-WhatIf"})
	acceptedApply := runMemberCurrentStep(t, caseRoot, []string{"-MemberExecutionAttemptId", attempt, "-MemberExecutionOutcome", "accepted", "-MemberExecutionObservedAt", "2026-08-03T02:01:00Z", "-Actor", "harness", "-ExpectedMemberExecutionPlanSha256", acceptedPreview.MemberExecution.ExpectedPlanSHA256, "-ExpectedCurrentStepPlanSha256", acceptedAuthorized.ExpectedCurrentStepPlanSHA256, "-Apply"})
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
	returnedPreview := runMemberCurrentStep(t, caseRoot, []string{"-MemberExecutionAttemptId", attempt, "-MemberExecutionOutcome", "returned", "-MemberExecutionObservedAt", "2026-08-03T02:02:00Z", "-Actor", "harness", "-WhatIf"})
	returnedAuthorized := runMemberCurrentStep(t, caseRoot, []string{"-MemberExecutionAttemptId", attempt, "-MemberExecutionOutcome", "returned", "-MemberExecutionObservedAt", "2026-08-03T02:02:00Z", "-Actor", "harness", "-ExpectedMemberExecutionPlanSha256", returnedPreview.MemberExecution.ExpectedPlanSHA256, "-WhatIf"})
	returnedApply := runMemberCurrentStep(t, caseRoot, []string{"-MemberExecutionAttemptId", attempt, "-MemberExecutionOutcome", "returned", "-MemberExecutionObservedAt", "2026-08-03T02:02:00Z", "-Actor", "harness", "-ExpectedMemberExecutionPlanSha256", returnedPreview.MemberExecution.ExpectedPlanSHA256, "-ExpectedCurrentStepPlanSha256", returnedAuthorized.ExpectedCurrentStepPlanSHA256, "-Apply"})
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
	reviewerBase := []string{"-Command", "run-current-step", "-Target", caseRoot, "-Pack", "_template", "-WhatIf", "-Format", "json"}
	dispatchReviewer := runCurrentStepPreview(t, append(append([]string{}, reviewerBase...), "-ReviewerHarness", "member-e2e-harness", "-ReviewerSession", "member-e2e-reviewer", "-Actor", "mission-commander"))
	runCurrentStepApply(t, caseRoot, dispatchReviewer, "-ReviewerHarness", "member-e2e-harness", "-ReviewerSession", "member-e2e-reviewer", "-Actor", "mission-commander")
	resultSource := filepath.Join(caseRoot, "workspace", "member-review-result.json")
	resultData, _ := json.Marshal(map[string]any{
		"packetId": packet.PacketID, "routeId": packet.Route.ID, "shardId": handoff.ShardID, "items": []string{"member-result"},
		"reviewerSession": "member-e2e-reviewer", "decision": "accept", "confidence": "high", "summary": "member manifest reviewed", "evidenceRefs": []string{status.MemberExecution.CompletionEvidence[0]}, "risks": []string{}, "conflicts": []string{}, "recommendedVerdict": "accepted",
		"routeOutput": map[string]any{"item": "member-result", "decision": "accept", "confidence": "high", "evidence": status.MemberExecution.CompletionEvidence[0], "risk": "low", "next_action": "main-agent-writeback", "tier_used": "light", "tool_scope": "read-only", "feature": "review", "request_id": "n/a", "candidate_path": "n/a", "defer_reason": "n/a"},
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
	replacedInput := previewCompletion(t, &out, caseRoot, "analysis", status.MemberExecution.CompletionEvidence[0])
	if !replacedInput.Blocked || !hasCompletionBlocker(replacedInput.Blockers, "member-manifest-review") || replacedInput.CompletionPlanSHA256 != "" {
		t.Fatalf("completion survived replaced canonical reviewer input: %+v", replacedInput)
	}
	if err := os.WriteFile(inputPath, inputBytes, 0o600); err != nil {
		t.Fatal(err)
	}
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
	var completionHandoff workstream.HandoffResult
	out.Reset()
	if err := Run([]string{"-Command", "handoff", "-Target", caseRoot, "-Pack", "_template", "-WhatIf", "-Format", "json"}, &out); err != nil {
		t.Fatal(err)
	}
	decodeJSONStrict(t, out.Bytes(), &completionHandoff)
	if completionHandoff.ReplacementExecutorTakeoverPackage == nil || completionHandoff.ReplacementExecutorTakeoverPackage.CurrentDriverRequest.Command != completionRequest.Command || completionHandoff.MissionCommanderActionQueue.CurrentDriverRequest == nil || completionHandoff.MissionCommanderActionQueue.CurrentDriverRequest.Command != completionRequest.Command {
		t.Fatalf("replacement executor handoff did not preserve completion request: %+v", completionHandoff.ReplacementExecutorTakeoverPackage)
	}
	var statusCompletionPreview completionProductResult
	runCompletionJSON(t, &out, rekitCommandCLIArgs(t, completionRequest.Command), &statusCompletionPreview)
	completionStep := runCurrentStepPreview(t, reviewerBase)
	if completionStep.Route != "case" || completionStep.DriverStep == nil || completionStep.DriverStep.CurrentDriverRequest.Command != completionRequest.Command || statusCompletionPreview.Blocked || statusCompletionPreview.CompletionPlanSHA256 == "" || completionStep.DriverStep.ExpectedDriverStepPreviewSHA256 != statusCompletionPreview.CompletionPlanSHA256 || completionStep.DriverStep.ExpectedDriverStepPlanSHA256 == "" || completionStep.ExpectedCurrentStepPlanSHA256 == "" {
		t.Fatalf("bounded current-step did not preserve completion double hash: %+v preview=%+v", completionStep, statusCompletionPreview)
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
	preview := runMemberCurrentStep(t, caseRoot, []string{"-WhatIf"})
	currentStepBeforeStatusRefreshHook = func(command string) error {
		if command != commands.RunCurrentStep {
			t.Fatalf("unexpected member refresh hook command: %s", command)
		}
		return errors.New("injected member status refresh failure")
	}
	t.Cleanup(func() { currentStepBeforeStatusRefreshHook = nil })
	out.Reset()
	err := Run([]string{"-Command", "run-current-step", "-Target", caseRoot, "-Pack", "_template", "-ExpectedMemberExecutionPlanSha256", preview.MemberExecution.ExpectedPlanSHA256, "-ExpectedCurrentStepPlanSha256", preview.ExpectedCurrentStepPlanSHA256, "-Apply", "-Format", "json"}, &out)
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

	memberPlan := runCurrentStepPreview(t, []string{"-Command", "run-current-step", "-Target", caseRoot, "-Pack", "_template", "-WhatIf", "-Format", "json"})
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
		memberDone <- Run([]string{"-Command", "run-current-step", "-Target", caseRoot, "-Pack", "_template", "-ExpectedMemberExecutionPlanSha256", memberPlan.MemberExecution.ExpectedPlanSHA256, "-ExpectedCurrentStepPlanSha256", memberPlan.ExpectedCurrentStepPlanSHA256, "-Apply", "-Format", "json"}, &memberOut)
	}()
	close(releaseLease)
	if err := <-reconcileDone; err != nil {
		t.Fatalf("public reconcile failed: %v", err)
	}
	if err := <-memberDone; err == nil || (!strings.Contains(err.Error(), "changed") && !strings.Contains(err.Error(), "stale") && !strings.Contains(err.Error(), "mismatch")) {
		t.Fatalf("old member generation survived public reconcile: %v", err)
	}
	if _, found, err := memberexecution.Latest(caseRoot, "feature-analysis"); err != nil || found {
		t.Fatalf("reconcile race corrupted or published old member execution: found=%v err=%v", found, err)
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
	loop := runCurrentLoopPreview(t, caseRoot, 3)
	runCurrentLoopApplyWith(t, caseRoot, loop, "-ExpectedMemberExecutionPlanSha256", loop.InitialCurrentStep.MemberExecution.ExpectedPlanSHA256)
	for _, flag := range []string{"-Actor", "-ExpectedMemberExecutionPlanSha256"} {
		var out bytes.Buffer
		err := Run([]string{"-Command", "run-current-step", "-Target", caseRoot, "-Pack", "_template", "-ExternalSessionHarness", "harness", "-ExternalSessionId", "session", "-ExternalSessionActor", "external-actor", "-ExternalSessionStartedAt", "2026-08-05T10:00:00Z", flag, "unexpected", "-WhatIf", "-Format", "json"}, &out)
		if err == nil || !strings.Contains(err.Error(), "external session route does not accept") {
			t.Fatalf("external route silently accepted %s: err=%v output=%s", flag, err, out.String())
		}
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
	args := []string{"-Command", "run-current-step", "-Target", caseRoot, "-Pack", "_template"}
	args = append(args, inputs...)
	args = append(args, "-Format", "json")
	return runCurrentStepPreview(t, args)
}

func sha256Text(data []byte) string {
	sum := sha256.Sum256(data)
	return hex.EncodeToString(sum[:])
}

type currentStepTestPlan struct {
	Route                         string                          `json:"route"`
	Applied                       bool                            `json:"applied"`
	DriverStep                    *driverStepPlan                 `json:"driverStep"`
	ReviewerStep                  *reviewerStepPlan               `json:"reviewerStep"`
	MemberExecution               *memberexecution.Plan           `json:"memberExecution"`
	ExternalSessionStep           *currentStepExternalSessionPlan `json:"externalSessionStep"`
	ExpectedCurrentStepPlanSHA256 string                          `json:"expectedCurrentStepPlanSha256"`
	Receipt                       *currentStepReceipt             `json:"receipt"`
	RefreshedStatus               *statusInventory                `json:"refreshedStatus"`
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
	var out bytes.Buffer
	args := []string{"-Command", "run-current-step", "-Target", caseRoot, "-Pack", "_template"}
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
