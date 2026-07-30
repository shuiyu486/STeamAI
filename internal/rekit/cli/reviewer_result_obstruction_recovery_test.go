package cli

import (
	"bytes"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/shuiyu486/re-context-kits/internal/rekit/subagents"
)

func TestRunPlanSubagentsReviewerResultObstructionRecoveryCaseLocalE2E(t *testing.T) {
	caseRoot := fullAttachedCase(t)
	var out bytes.Buffer
	if err := Run([]string{"-Command", "start", "-Target", caseRoot, "-Pack", "_template", "-Name", "review", "-Apply"}, &out); err != nil {
		t.Fatal(err)
	}
	writeCaseFile(t, caseRoot, "workspace/recovery-evidence.md", "bounded recovery evidence\n")
	nested := filepath.Join(caseRoot, "workspace")
	oldwd, err := os.Getwd()
	if err != nil {
		t.Fatal(err)
	}
	if err := os.Chdir(nested); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = os.Chdir(oldwd) })

	out.Reset()
	if err := Run([]string{"-Command", "plan-subagents", "-TaskType", "feature-analysis", "-Items", "alpha", "-Lane", "feature-review", "-Format", "json"}, &out); err != nil {
		t.Fatal(err)
	}
	plan := decodePlanSubagentsResult(t, out.Bytes())
	packet := decodePlanSubagentsPacket(t, plan.PacketPath)
	handoff := plan.ShardHandoffs[0]
	candidate, err := json.Marshal(map[string]any{
		"packetId": packet.PacketID, "routeId": packet.Route.ID, "shardId": handoff.ShardID, "items": handoff.Items,
		"reviewerSession": "reviewer-obstruction-e2e", "decision": "accept", "confidence": "high", "summary": "reviewed alpha",
		"evidenceRefs": []string{"workspace/recovery-evidence.md"}, "risks": []string{}, "conflicts": []string{}, "recommendedVerdict": "accepted",
		"routeOutput": map[string]any{"item": "alpha", "decision": "accept", "confidence": "high", "evidence": "workspace/recovery-evidence.md", "risk": "low", "next_action": "main-agent-writeback", "tier_used": "light", "tool_scope": "read-only", "feature": "review", "request_id": "n/a", "candidate_path": "n/a", "defer_reason": "n/a"},
	})
	if err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(filepath.Dir(handoff.ReviewerResultCandidatePath), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(handoff.ReviewerResultCandidatePath, candidate, 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(handoff.ReviewerResultPath, nil, 0o644); err != nil {
		t.Fatal(err)
	}

	out.Reset()
	if err := Run([]string{"-Command", "plan-subagents", "-PacketPath", plan.PacketPath, "-CollectReviewerResult", "-ShardId", handoff.ShardID, "-Lane", "feature-review", "-Actor", "mission-commander", "-WhatIf", "-Format", "json"}, &out); err != nil {
		t.Fatal(err)
	}
	var collectionBlocked subagents.ReviewerResultCollectionResult
	if err := json.Unmarshal(out.Bytes(), &collectionBlocked); err != nil {
		t.Fatal(err)
	}
	if collectionBlocked.Status != "recovery-required" || !collectionBlocked.RecoveryRequired || collectionBlocked.ReviewerResultKind != "empty-file" || !strings.Contains(collectionBlocked.MissionCommanderAction.PrimaryCommand, "-RecoverReviewerResult") {
		t.Fatalf("collection collision omitted recovery handoff: %+v", collectionBlocked)
	}
	out.Reset()
	if err := Run([]string{"-Command", "plan-subagents", "-PacketPath", plan.PacketPath, "-CollectReviewerResult", "-ShardId", handoff.ShardID, "-Lane", "feature-review", "-Actor", "mission-commander", "-WhatIf", "-Format", "text"}, &out); err != nil {
		t.Fatal(err)
	}
	if text := out.String(); !strings.Contains(text, "status=recovery-required") || !strings.Contains(text, "canonicalKind=empty-file") || !strings.Contains(text, "-RecoverReviewerResult") {
		t.Fatalf("collection recovery-required text omitted recovery handoff: %s", text)
	}

	args := []string{"-Command", "plan-subagents", "-PacketPath", plan.PacketPath, "-RecoverReviewerResult", "-ShardId", handoff.ShardID, "-Lane", "feature-review", "-Actor", "mission-commander", "-Reason", "quarantine canonical obstruction"}
	out.Reset()
	if err := Run(append(append([]string{}, args...), "-WhatIf", "-Format", "json"), &out); err != nil {
		t.Fatal(err)
	}
	var preview subagents.ReviewerResultRecoveryResult
	if err := json.Unmarshal(out.Bytes(), &preview); err != nil {
		t.Fatal(err)
	}
	if preview.Applied || preview.ReviewerResultKind != "empty-file" || preview.ReviewerResultSHA256 == "" || !containsSubstring(preview.RunbookSteps, "-RecoverReviewerResult") || !containsSubstring(preview.RunbookSteps, "-ExpectedCandidateSha256") || !containsSubstring(preview.RunbookSteps, "-ExpectedReviewerResultSha256") || !containsSubstring(preview.RunbookSteps, "hash-bound") {
		t.Fatalf("unexpected obstruction recovery preview: %+v", preview)
	}
	out.Reset()
	if err := Run(append(append([]string{}, args...), "-WhatIf", "-Format", "text"), &out); err != nil {
		t.Fatal(err)
	}
	if text := out.String(); !strings.Contains(text, "canonicalKind=empty-file") || !strings.Contains(text, "canonicalMode=") || !strings.Contains(text, "canonicalLinkTarget=") || !strings.Contains(text, "reviewer result recovery runbook：") || !strings.Contains(text, "-RecoverReviewerResult") || !strings.Contains(text, "-ExpectedCandidateSha256") || !strings.Contains(text, "-ExpectedReviewerResultSha256") {
		t.Fatalf("obstruction snapshot omitted from text output: %s", text)
	}

	out.Reset()
	applyArgs := append(append([]string{}, args...), "-ExpectedCandidateSha256", preview.CandidateSHA256, "-ExpectedReviewerResultSha256", preview.ReviewerResultSHA256, "-Apply", "-Format", "json")
	if err := Run(applyArgs, &out); err != nil {
		t.Fatal(err)
	}
	var applied subagents.ReviewerResultRecoveryResult
	if err := json.Unmarshal(out.Bytes(), &applied); err != nil {
		t.Fatal(err)
	}
	if !applied.Applied || applied.ReviewerResultKind != "empty-file" || !containsSubstring(applied.RunbookSteps, "-CollectReviewerResult") {
		t.Fatalf("unexpected obstruction recovery apply: %+v", applied)
	}
	if st, err := os.Lstat(applied.QuarantinePath); err != nil || !st.Mode().IsRegular() || st.Size() != 0 {
		t.Fatalf("quarantined obstruction is not the original empty file: %v %v", st, err)
	}
	if _, err := os.Lstat(handoff.ReviewerResultPath); !os.IsNotExist(err) {
		t.Fatalf("canonical obstruction remains: %v", err)
	}
	if err := os.Remove(applied.ReceiptPath); err != nil {
		t.Fatal(err)
	}

	out.Reset()
	if err := Run([]string{"-Command", "status", "-Target", caseRoot, "-Pack", "_template", "-Format", "json"}, &out); err != nil {
		t.Fatal(err)
	}
	var interruptedStatus struct {
		CaseMission struct {
			ReviewerDispatchIntakeHandoffs []reviewerDispatchIntakeCLIItem      `json:"reviewerDispatchIntakeHandoffs"`
			ReviewerDispatchIntakeSummary  reviewerDispatchIntakeSummaryCLIItem `json:"reviewerDispatchIntakeSummary"`
		} `json:"caseMission"`
	}
	if err := json.Unmarshal(out.Bytes(), &interruptedStatus); err != nil {
		t.Fatalf("status JSON did not decode: %v\n%s", err, out.String())
	}
	if len(interruptedStatus.CaseMission.ReviewerDispatchIntakeHandoffs) != 1 || interruptedStatus.CaseMission.ReviewerDispatchIntakeHandoffs[0].State != "reviewer-result-recovery-finalize-required" || !strings.Contains(interruptedStatus.CaseMission.ReviewerDispatchIntakeHandoffs[0].ReviewerResultRecoveryApplyCommand, "-ExpectedReviewerResultSha256") || interruptedStatus.CaseMission.ReviewerDispatchIntakeSummary.NextActionState != "reviewer-result-recovery-finalize-required" || !strings.Contains(interruptedStatus.CaseMission.ReviewerDispatchIntakeSummary.NextAction, "-ExpectedCandidateSha256") {
		t.Fatalf("status omitted interrupted recovery finalize handoff: %+v", interruptedStatus.CaseMission)
	}
	out.Reset()
	if err := Run([]string{"-Command", "status", "-Target", caseRoot, "-Pack", "_template", "-Format", "text"}, &out); err != nil {
		t.Fatal(err)
	}
	if text := out.String(); !strings.Contains(text, "reviewer-result-recovery-finalize-required") || !strings.Contains(text, "-ExpectedReviewerResultSha256") {
		t.Fatalf("status text omitted interrupted recovery finalize handoff: %s", text)
	}
	out.Reset()
	if err := Run(applyArgs, &out); err != nil {
		t.Fatal(err)
	}
	var finalized subagents.ReviewerResultRecoveryResult
	if err := json.Unmarshal(out.Bytes(), &finalized); err != nil {
		t.Fatal(err)
	}
	if !finalized.Applied || finalized.AlreadyRecovered || finalized.MissionCommanderAction.State != "reviewer-result-recovery-already-applied" || !containsSubstring(finalized.RunbookSteps, "-CollectReviewerResult") {
		t.Fatalf("unexpected interrupted recovery finalize apply: %+v", finalized)
	}
	if _, err := os.Lstat(finalized.ReceiptPath); err != nil {
		t.Fatalf("finalized recovery receipt missing: %v", err)
	}

	out.Reset()
	if err := Run([]string{"-Command", "plan-subagents", "-PacketPath", plan.PacketPath, "-CollectReviewerResult", "-ShardId", handoff.ShardID, "-Lane", "feature-review", "-Actor", "mission-commander", "-WhatIf", "-Format", "json"}, &out); err != nil {
		t.Fatal(err)
	}
	out.Reset()
	if err := Run([]string{"-Command", "plan-subagents", "-PacketPath", plan.PacketPath, "-CollectReviewerResult", "-ShardId", handoff.ShardID, "-Lane", "feature-review", "-Actor", "mission-commander", "-Apply", "-Format", "json"}, &out); err != nil {
		t.Fatal(err)
	}
	recordReviewerSessionReceiptsForCLIPlan(t, &out, nil, plan.PacketPath, handoff, "feature-review", "mission-commander", "go-cli-obstruction-recovery-harness", candidate)
	out.Reset()
	if err := Run([]string{"-Command", "plan-subagents", "-PacketPath", plan.PacketPath, "-ReadyReviewerResults", "-Lane", "feature-review", "-Actor", "mission-commander", "-WhatIf", "-Format", "json"}, &out); err != nil {
		t.Fatal(err)
	}
}

func TestRunPlanSubagentsReviewerResultRecoveryDispositionCaseLocalE2E(t *testing.T) {
	caseRoot := fullAttachedCase(t)
	var out bytes.Buffer
	if err := Run([]string{"-Command", "start", "-Target", caseRoot, "-Pack", "_template", "-Name", "review", "-Apply"}, &out); err != nil {
		t.Fatal(err)
	}
	writeCaseFile(t, caseRoot, "workspace/features/feature-login/review-evidence.md", "bounded disposition evidence\n")

	out.Reset()
	if err := Run([]string{"-Command", "plan-subagents", "-Target", caseRoot, "-Pack", "_template", "-TaskType", "feature-analysis", "-Items", "alpha", "-Lane", "feature-review", "-Format", "json"}, &out); err != nil {
		t.Fatal(err)
	}
	plan := decodePlanSubagentsResult(t, out.Bytes())
	packet := decodePlanSubagentsPacket(t, plan.PacketPath)
	handoff := packet.ShardHandoffs[0]
	candidate := reviewerResultForCLIPlan(t, packet, handoff, "accept", "accepted", "reviewer-disposition-e2e")
	if err := os.MkdirAll(filepath.Dir(handoff.ReviewerResultCandidatePath), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(handoff.ReviewerResultCandidatePath, candidate, 0o644); err != nil {
		t.Fatal(err)
	}
	conflict := []byte(`{"conflict":true}`)
	if err := os.WriteFile(handoff.ReviewerResultPath, conflict, 0o644); err != nil {
		t.Fatal(err)
	}

	recoveryArgs := []string{"-Command", "plan-subagents", "-Target", caseRoot, "-Pack", "_template", "-PacketPath", plan.PacketPath, "-RecoverReviewerResult", "-ShardId", handoff.ShardID, "-Lane", "feature-review", "-Actor", "mission-commander", "-Reason", "quarantine conflicting canonical reviewer result"}
	out.Reset()
	if err := Run(append(append([]string{}, recoveryArgs...), "-WhatIf", "-Format", "json"), &out); err != nil {
		t.Fatal(err)
	}
	var recoveryPreview subagents.ReviewerResultRecoveryResult
	if err := json.Unmarshal(out.Bytes(), &recoveryPreview); err != nil {
		t.Fatalf("recovery preview JSON did not decode: %v\n%s", err, out.String())
	}
	if recoveryPreview.Applied || recoveryPreview.CandidateSHA256 == "" || recoveryPreview.ReviewerResultSHA256 == "" || recoveryPreview.MissionCommanderAction.State != "needs-reviewer-result-recovery-apply" {
		t.Fatalf("unexpected recovery preview: %+v", recoveryPreview)
	}

	out.Reset()
	if err := Run(append(append([]string{}, recoveryArgs...), "-ExpectedCandidateSha256", recoveryPreview.CandidateSHA256, "-ExpectedReviewerResultSha256", recoveryPreview.ReviewerResultSHA256, "-Apply", "-Format", "json"), &out); err != nil {
		t.Fatal(err)
	}
	var recovered subagents.ReviewerResultRecoveryResult
	if err := json.Unmarshal(out.Bytes(), &recovered); err != nil {
		t.Fatalf("recovery apply JSON did not decode: %v\n%s", err, out.String())
	}
	if !recovered.Applied || recovered.ReceiptPath == "" || recovered.IntentPath == "" || recovered.QuarantinePath == "" {
		t.Fatalf("unexpected recovery apply: %+v", recovered)
	}
	if err := os.Remove(recovered.ReceiptPath); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(handoff.ReviewerResultPath, candidate, 0o644); err != nil {
		t.Fatal(err)
	}

	out.Reset()
	if err := Run([]string{"-Command", "status", "-Target", caseRoot, "-Pack", "_template", "-Format", "json"}, &out); err != nil {
		t.Fatal(err)
	}
	var ambiguousStatus struct {
		CaseMission struct {
			ReviewerDispatchIntakeHandoffs []reviewerDispatchIntakeCLIItem      `json:"reviewerDispatchIntakeHandoffs"`
			ReviewerDispatchIntakeSummary  reviewerDispatchIntakeSummaryCLIItem `json:"reviewerDispatchIntakeSummary"`
		} `json:"caseMission"`
	}
	if err := json.Unmarshal(out.Bytes(), &ambiguousStatus); err != nil {
		t.Fatalf("status JSON did not decode: %v\n%s", err, out.String())
	}
	if len(ambiguousStatus.CaseMission.ReviewerDispatchIntakeHandoffs) != 1 {
		t.Fatalf("status handoff count = %d, want 1: %+v", len(ambiguousStatus.CaseMission.ReviewerDispatchIntakeHandoffs), ambiguousStatus.CaseMission)
	}
	item := ambiguousStatus.CaseMission.ReviewerDispatchIntakeHandoffs[0]
	if item.State != "reviewer-result-recovery-ambiguous" || item.ReviewerResultRecoveryApplyCommand != "" || !strings.Contains(item.ReviewerResultRecoveryDispositionCommand, "-RetireReviewerResultRecovery") || !strings.Contains(item.ReviewerResultRecoveryDispositionCommand, "-WhatIf") || !containsSubstring(item.RunbookSteps, "bounded disposition apply") || ambiguousStatus.CaseMission.ReviewerDispatchIntakeSummary.NextActionState != "reviewer-result-recovery-ambiguous" || !strings.Contains(ambiguousStatus.CaseMission.ReviewerDispatchIntakeSummary.NextAction, "-RetireReviewerResultRecovery") {
		t.Fatalf("status omitted ambiguous recovery disposition handoff: item=%+v summary=%+v", item, ambiguousStatus.CaseMission.ReviewerDispatchIntakeSummary)
	}

	out.Reset()
	if err := Run([]string{"-Command", "status", "-Target", caseRoot, "-Pack", "_template", "-Format", "text"}, &out); err != nil {
		t.Fatal(err)
	}
	if text := out.String(); !strings.Contains(text, "reviewer-result-recovery-ambiguous") || !strings.Contains(text, "-RetireReviewerResultRecovery") || !strings.Contains(text, "bounded disposition apply") {
		t.Fatalf("status text omitted ambiguous disposition handoff: %s", text)
	}

	dispositionArgs := []string{"-Command", "plan-subagents", "-Target", caseRoot, "-Pack", "_template", "-PacketPath", plan.PacketPath, "-RetireReviewerResultRecovery", "-ShardId", handoff.ShardID, "-Lane", "feature-review", "-Actor", "mission-commander", "-Reason", "retain restored canonical reviewer result"}
	out.Reset()
	if err := Run(append(append([]string{}, dispositionArgs...), "-WhatIf", "-Format", "json"), &out); err != nil {
		t.Fatal(err)
	}
	var dispositionPreview subagents.ReviewerResultRecoveryDispositionResult
	if err := json.Unmarshal(out.Bytes(), &dispositionPreview); err != nil {
		t.Fatalf("disposition preview JSON did not decode: %v\n%s", err, out.String())
	}
	if dispositionPreview.Applied || dispositionPreview.IntentSHA256 == "" || dispositionPreview.CanonicalSHA256 != recoveryPreview.CandidateSHA256 || dispositionPreview.MissionCommanderAction.State != "needs-reviewer-result-recovery-disposition-apply" || !strings.Contains(dispositionPreview.MissionCommanderAction.PrimaryCommand, "-ExpectedIntentSha256") || !strings.Contains(dispositionPreview.MissionCommanderAction.PrimaryCommand, "-ExpectedCanonicalSha256") {
		t.Fatalf("unexpected disposition preview: %+v", dispositionPreview)
	}
	if _, err := os.Stat(dispositionPreview.DispositionPath); !os.IsNotExist(err) {
		t.Fatalf("disposition preview wrote disposition: %v", err)
	}

	out.Reset()
	if err := Run(append(append([]string{}, dispositionArgs...), "-WhatIf", "-Format", "text"), &out); err != nil {
		t.Fatal(err)
	}
	if text := out.String(); !strings.Contains(text, "intentSha256=") || !strings.Contains(text, "canonicalSha256=") || !strings.Contains(text, "-ExpectedIntentSha256") || !strings.Contains(text, "-ExpectedCanonicalSha256") {
		t.Fatalf("disposition preview text omitted bounded hashes: %s", text)
	}

	out.Reset()
	if err := Run(append(append([]string{}, dispositionArgs...), "-ExpectedIntentSha256", dispositionPreview.IntentSHA256, "-ExpectedCanonicalSha256", dispositionPreview.CanonicalSHA256, "-Apply", "-Format", "json"), &out); err != nil {
		t.Fatal(err)
	}
	var dispositionApplied subagents.ReviewerResultRecoveryDispositionResult
	if err := json.Unmarshal(out.Bytes(), &dispositionApplied); err != nil {
		t.Fatalf("disposition apply JSON did not decode: %v\n%s", err, out.String())
	}
	if !dispositionApplied.Applied || dispositionApplied.MissionCommanderAction.State != "reviewer-result-recovery-disposed-ready-for-collection-preview" {
		t.Fatalf("unexpected disposition apply: %+v", dispositionApplied)
	}
	if data, err := os.ReadFile(handoff.ReviewerResultPath); err != nil || !bytes.Equal(data, candidate) {
		t.Fatalf("canonical reviewer result changed during disposition: %q err=%v", data, err)
	}
	if _, err := os.Stat(dispositionApplied.DispositionPath); err != nil {
		t.Fatalf("disposition record missing: %v", err)
	}

	out.Reset()
	if err := Run([]string{"-Command", "status", "-Target", caseRoot, "-Pack", "_template", "-Format", "json"}, &out); err != nil {
		t.Fatal(err)
	}
	var disposedStatus struct {
		CaseMission struct {
			ReviewerDispatchIntakeHandoffs    []reviewerDispatchIntakeCLIItem      `json:"reviewerDispatchIntakeHandoffs"`
			ReviewerDispatchIntakeSummary     reviewerDispatchIntakeSummaryCLIItem `json:"reviewerDispatchIntakeSummary"`
			ReviewerDispatchIntakeActionQueue missionCommanderActionQueueSnapshot  `json:"reviewerDispatchIntakeActionQueue"`
		} `json:"caseMission"`
	}
	if err := json.Unmarshal(out.Bytes(), &disposedStatus); err != nil {
		t.Fatalf("disposed status JSON did not decode: %v\n%s", err, out.String())
	}
	if len(disposedStatus.CaseMission.ReviewerDispatchIntakeHandoffs) != 1 {
		t.Fatalf("disposed status handoff count = %d, want 1: %+v", len(disposedStatus.CaseMission.ReviewerDispatchIntakeHandoffs), disposedStatus.CaseMission)
	}
	disposedItem := disposedStatus.CaseMission.ReviewerDispatchIntakeHandoffs[0]
	disposedSummary := disposedStatus.CaseMission.ReviewerDispatchIntakeSummary
	disposedCurrent := disposedStatus.CaseMission.ReviewerDispatchIntakeActionQueue.CurrentAction
	if disposedItem.State != "reviewer-result-recovery-disposed-ready-for-collection-preview" || disposedItem.ReviewerResultRecoveryDispositionPath != dispositionApplied.DispositionPath || disposedItem.ReviewerResultCollectionCommands == nil || !strings.Contains(disposedItem.ReviewerResultCollectionCommands.PreviewCommand, "-CollectReviewerResult") || !containsSubstring(disposedItem.RunbookSteps, "recovery disposition is current") || !containsSubstring(disposedItem.Evidence, "reviewerResultRecoveryDisposition current") || disposedSummary.NextActionState != "reviewer-result-recovery-disposed-ready-for-collection-preview" || !strings.Contains(disposedSummary.NextAction, "-CollectReviewerResult") || disposedCurrent == nil || disposedCurrent.Blocked || disposedCurrent.State != "reviewer-result-recovery-disposed-ready-for-collection-preview" || !strings.Contains(disposedCurrent.Command, "-CollectReviewerResult") {
		t.Fatalf("disposed status omitted collection continuation: item=%+v summary=%+v current=%+v", disposedItem, disposedSummary, disposedCurrent)
	}

	out.Reset()
	if err := Run([]string{"-Command", "status", "-Target", caseRoot, "-Pack", "_template", "-Format", "text"}, &out); err != nil {
		t.Fatal(err)
	}
	if text := out.String(); !strings.Contains(text, "reviewer-result-recovery-disposed-ready-for-collection-preview") || !strings.Contains(text, "-CollectReviewerResult") || !strings.Contains(text, "reviewerResultRecoveryDisposition current") {
		t.Fatalf("disposed status text omitted collection continuation: %s", text)
	}

	out.Reset()
	if err := Run([]string{"-Command", "handoff", "-Target", caseRoot, "-Apply", "-Format", "json"}, &out); err != nil {
		t.Fatal(err)
	}
	disposedHandoff := decodeHandoffResult(t, out.Bytes())
	if !disposedHandoff.Project || !disposedHandoff.Applied || disposedHandoff.ReviewerDispatchIntakeSummary.NextActionState != "reviewer-result-recovery-disposed-ready-for-collection-preview" || disposedHandoff.MissionCommanderActionQueue.CurrentAction == nil || disposedHandoff.MissionCommanderActionQueue.CurrentAction.State != "reviewer-result-recovery-disposed-ready-for-collection-preview" || !strings.Contains(disposedHandoff.MissionCommanderActionQueue.CurrentAction.Command, "-CollectReviewerResult") {
		t.Fatalf("disposed project handoff omitted collection continuation: %+v", disposedHandoff)
	}
	latestProjectHandoff := assertStartWrite(t, disposedHandoff.Writes, ".rekit/handovers/latest.md", "write-latest-project-handoff")
	latestProjectHandoffText, err := os.ReadFile(latestProjectHandoff.TargetPath)
	if err != nil {
		t.Fatal(err)
	}
	for _, expected := range []string{"reviewer-result-recovery-disposed-ready-for-collection-preview", "reviewerResultRecoveryDisposition current", "-CollectReviewerResult"} {
		if !bytes.Contains(latestProjectHandoffText, []byte(expected)) {
			t.Fatalf("disposed project handoff text omitted %q:\n%s", expected, string(latestProjectHandoffText))
		}
	}

	out.Reset()
	if err := Run([]string{"-Command", "continue", "-Target", caseRoot, "-Pack", "_template", "review", "-WhatIf", "-Format", "json"}, &out); err != nil {
		t.Fatal(err)
	}
	var disposedContinue struct {
		Blocked                       bool                                 `json:"blocked"`
		ReviewerDispatchIntakeSummary reviewerDispatchIntakeSummaryCLIItem `json:"reviewerDispatchIntakeSummary"`
		MissionCommanderActionQueue   missionCommanderActionQueueSnapshot  `json:"missionCommanderActionQueue"`
		NextSteps                     []string                             `json:"nextSteps"`
	}
	if err := json.Unmarshal(out.Bytes(), &disposedContinue); err != nil {
		t.Fatalf("disposed continue JSON did not decode: %v\n%s", err, out.String())
	}
	continueCurrent := disposedContinue.MissionCommanderActionQueue.CurrentAction
	if !disposedContinue.Blocked || disposedContinue.ReviewerDispatchIntakeSummary.NextActionState != "reviewer-result-recovery-disposed-ready-for-collection-preview" || continueCurrent == nil || continueCurrent.Blocked || continueCurrent.State != "reviewer-result-recovery-disposed-ready-for-collection-preview" || !strings.Contains(continueCurrent.Command, "-CollectReviewerResult") || !containsSubstring(disposedContinue.NextSteps, "-CollectReviewerResult") {
		t.Fatalf("disposed continue preview omitted collection continuation: preview=%+v current=%+v", disposedContinue, continueCurrent)
	}

	out.Reset()
	if err := Run([]string{"-Command", "plan-subagents", "-Target", caseRoot, "-Pack", "_template", "-PacketPath", plan.PacketPath, "-CollectReviewerResult", "-ShardId", handoff.ShardID, "-Lane", "feature-review", "-Actor", "mission-commander", "-WhatIf", "-Format", "json"}, &out); err != nil {
		t.Fatal(err)
	}
	var collectionPreview subagents.ReviewerResultCollectionResult
	if err := json.Unmarshal(out.Bytes(), &collectionPreview); err != nil {
		t.Fatalf("collection preview JSON did not decode: %v\n%s", err, out.String())
	}
	if collectionPreview.Status != "already-collected" || !collectionPreview.AlreadyCollected || collectionPreview.RecoveryRequired {
		t.Fatalf("collection remained blocked after disposition: %+v", collectionPreview)
	}

	out.Reset()
	if err := Run([]string{"-Command", "plan-subagents", "-Target", caseRoot, "-Pack", "_template", "-PacketPath", plan.PacketPath, "-CollectReviewerResult", "-ShardId", handoff.ShardID, "-Lane", "feature-review", "-Actor", "mission-commander", "-Apply", "-Format", "json"}, &out); err != nil {
		t.Fatal(err)
	}
	var collectionApplied subagents.ReviewerResultCollectionResult
	if err := json.Unmarshal(out.Bytes(), &collectionApplied); err != nil {
		t.Fatalf("collection apply JSON did not decode: %v\n%s", err, out.String())
	}
	if !collectionApplied.Applied || collectionApplied.Status != "already-collected" || !collectionApplied.AlreadyCollected {
		t.Fatalf("collection apply did not proceed after disposition: %+v", collectionApplied)
	}

	recordReviewerSessionReceiptsForCLIPlan(t, &out, []string{"-Target", caseRoot, "-Pack", "_template"}, plan.PacketPath, handoff, "feature-review", "mission-commander", "go-cli-disposition-harness", candidate)
	out.Reset()
	if err := Run([]string{"-Command", "plan-subagents", "-Target", caseRoot, "-Pack", "_template", "-PacketPath", plan.PacketPath, "-ReadyReviewerResults", "-Lane", "feature-review", "-Actor", "mission-commander", "-WhatIf", "-Format", "json"}, &out); err != nil {
		t.Fatal(err)
	}
	var ready subagents.ReviewerBatchIntakeResult
	if err := json.Unmarshal(out.Bytes(), &ready); err != nil {
		t.Fatalf("ready reviewer results JSON did not decode: %v\n%s", err, out.String())
	}
	if ready.Ready != 1 || ready.Processed != 1 || ready.Waiting != 0 || ready.Stopped {
		t.Fatalf("ready reviewer results did not proceed after disposition: %+v", ready)
	}
}
