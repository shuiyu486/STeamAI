package cli

import (
	"bytes"
	"encoding/json"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"

	"github.com/shuiyu486/re-context-kits/internal/rekit/subagents"
)

func TestRunPlanSubagentsReviewerResultRecoveryCaseLocalE2E(t *testing.T) {
	if runtime.GOOS != "windows" {
		t.Skip("exact reviewer result recovery requires the Windows handle-validated move primitive")
	}
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
		"reviewerSession": "reviewer-recovery-e2e", "decision": "accept", "confidence": "high", "summary": "reviewed alpha",
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
	if err := os.WriteFile(handoff.ReviewerResultPath, []byte(`{"conflict":true}`), 0o644); err != nil {
		t.Fatal(err)
	}
	args := []string{"-Command", "plan-subagents", "-PacketPath", plan.PacketPath, "-RecoverReviewerResult", "-ShardId", handoff.ShardID, "-Lane", "feature-review", "-Actor", "mission-commander", "-Reason", "quarantine conflicting canonical reviewer result"}
	out.Reset()
	if err := Run(append(append([]string{}, args...), "-WhatIf", "-Format", "json"), &out); err != nil {
		t.Fatal(err)
	}
	var preview subagents.ReviewerResultRecoveryResult
	if err := json.Unmarshal(out.Bytes(), &preview); err != nil {
		t.Fatal(err)
	}
	if preview.Applied || preview.CandidateSHA256 == "" || preview.ReviewerResultSHA256 == "" {
		t.Fatalf("unexpected recovery preview: %+v", preview)
	}
	out.Reset()
	if err := Run(append(append([]string{}, args...), "-WhatIf", "-Format", "text"), &out); err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(out.String(), "plan-subagents reviewer result recovery：") {
		t.Fatalf("unexpected recovery text: %s", out.String())
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
	if !applied.Applied || applied.MissionCommanderAction.State != "reviewer-result-recovered-ready-for-collection-preview" {
		t.Fatalf("unexpected recovery apply: %+v", applied)
	}
	out.Reset()
	if err := Run([]string{"-Command", "plan-subagents", "-PacketPath", plan.PacketPath, "-CollectReviewerResult", "-ShardId", handoff.ShardID, "-Lane", "feature-review", "-Actor", "mission-commander", "-WhatIf", "-Format", "json"}, &out); err != nil {
		t.Fatal(err)
	}
	var collectionPreview subagents.ReviewerResultCollectionResult
	if err := json.Unmarshal(out.Bytes(), &collectionPreview); err != nil {
		t.Fatal(err)
	}
	out.Reset()
	if err := Run([]string{"-Command", "plan-subagents", "-PacketPath", plan.PacketPath, "-CollectReviewerResult", "-ShardId", handoff.ShardID, "-Lane", "feature-review", "-Actor", "mission-commander", "-ExpectedCandidateSha256", collectionPreview.CandidateSHA256, "-Apply", "-Format", "json"}, &out); err != nil {
		t.Fatal(err)
	}
	recordReviewerSessionReceiptsForCLIPlan(t, &out, nil, plan.PacketPath, handoff, "feature-review", "mission-commander", "go-cli-recovery-harness", candidate)
	out.Reset()
	if err := Run([]string{"-Command", "plan-subagents", "-PacketPath", plan.PacketPath, "-ReadyReviewerResults", "-Lane", "feature-review", "-Actor", "mission-commander", "-WhatIf", "-Format", "json"}, &out); err != nil {
		t.Fatal(err)
	}
}

func TestRunPlanSubagentsReviewerPacketRetirementWhatIfApplyE2E(t *testing.T) {
	caseRoot := fullAttachedCase(t)
	var out bytes.Buffer
	if err := Run([]string{"-Command", "start", "-Target", caseRoot, "-Pack", "_template", "-Name", "review", "-Apply"}, &out); err != nil {
		t.Fatal(err)
	}
	out.Reset()
	if err := Run([]string{"-Command", "plan-subagents", "-Target", caseRoot, "-Pack", "_template", "-TaskType", "feature-analysis", "-Items", "alpha", "-Lane", "feature-review", "-Format", "json"}, &out); err != nil {
		t.Fatal(err)
	}
	plan := decodePlanSubagentsResult(t, out.Bytes())
	if err := os.WriteFile(plan.PacketPath, []byte("{truncated"), 0o644); err != nil {
		t.Fatal(err)
	}
	out.Reset()
	if err := Run([]string{"-Command", "status", "-Target", caseRoot, "-Pack", "_template", "-Format", "json"}, &out); err != nil {
		t.Fatal(err)
	}
	var invalidStatus struct {
		CaseMission struct {
			ReviewerDispatchIntakeHandoffs    []reviewerDispatchIntakeCLIItem      `json:"reviewerDispatchIntakeHandoffs"`
			ReviewerDispatchIntakeSummary     reviewerDispatchIntakeSummaryCLIItem `json:"reviewerDispatchIntakeSummary"`
			ReviewerDispatchIntakeActionQueue missionCommanderActionQueueSnapshot  `json:"reviewerDispatchIntakeActionQueue"`
			MissionCommanderActionQueue       missionCommanderActionQueueSnapshot  `json:"missionCommanderActionQueue"`
		} `json:"caseMission"`
	}
	if err := json.Unmarshal(out.Bytes(), &invalidStatus); err != nil {
		t.Fatalf("status JSON did not decode: %v\n%s", err, out.String())
	}
	if len(invalidStatus.CaseMission.ReviewerDispatchIntakeHandoffs) != 1 || !strings.Contains(invalidStatus.CaseMission.ReviewerDispatchIntakeHandoffs[0].PacketRetirementPreviewCommand, "-RetireInvalidReviewerPacket") || !strings.Contains(invalidStatus.CaseMission.ReviewerDispatchIntakeSummary.NextActionPacketRetirementPreviewCommand, "-RetireInvalidReviewerPacket") || !containsSubstring(invalidStatus.CaseMission.ReviewerDispatchIntakeSummary.NextActionRunbookSteps, "retirement preview") {
		t.Fatalf("status omitted invalid packet retirement handoff: %+v", invalidStatus.CaseMission)
	}
	invalidPacket := invalidStatus.CaseMission.ReviewerDispatchIntakeHandoffs[0]
	if invalidPacket.State != "reviewer-packet-integrity-invalid" || invalidPacket.DispatchCommand != "" || invalidPacket.PreviewCommand != "" || invalidPacket.ApplyCommand != "" || invalidPacket.BatchPreviewCommand != "" || invalidPacket.BatchApplyCommand != "" {
		t.Fatalf("invalid packet status did not block reviewer dispatch/intake commands: %+v", invalidPacket)
	}
	retirementRequest := requireMissionCommanderDriverRequest(t, invalidStatus.CaseMission.MissionCommanderActionQueue, "blocked-review", "inspect-current", invalidPacket.PacketRetirementPreviewCommand, true, true, true)
	if invalidStatus.CaseMission.ReviewerDispatchIntakeActionQueue.CurrentDriverRequest == nil || invalidStatus.CaseMission.ReviewerDispatchIntakeActionQueue.CurrentDriverRequest.Command != retirementRequest.Command {
		t.Fatalf("packet retirement reviewer queue request drifted from first screen: first=%+v reviewer=%+v", retirementRequest, invalidStatus.CaseMission.ReviewerDispatchIntakeActionQueue.CurrentDriverRequest)
	}
	if strings.Contains(retirementRequest.Command, "-RecordReviewerDispatch") || strings.Contains(retirementRequest.Command, "-AdoptReviewerPacket") || strings.Contains(retirementRequest.Command, "-ReadyReviewerResults") {
		t.Fatalf("packet retirement driver-loop exposed reviewer dispatch/adoption/intake while packet was invalid: %+v", retirementRequest)
	}
	out.Reset()
	if err := Run([]string{"-Command", "status", "-Target", caseRoot, "-Pack", "_template", "-Format", "text"}, &out); err != nil {
		t.Fatal(err)
	}
	for _, expected := range []string{"status case mission reviewer dispatch next action", "packetRetirementPreview=`/rekit plan-subagents", "status case mission reviewer packet retirement", "-RetireInvalidReviewerPacket"} {
		if !strings.Contains(out.String(), expected) {
			t.Fatalf("status text omitted invalid packet retirement handoff %q:\n%s", expected, out.String())
		}
	}

	baseArgs := []string{"-Target", caseRoot, "-Pack", "_template"}
	actor := "mission-commander"
	reason := "retire exact invalid packet"
	driverRequestArgs := func(label string, request *missionCommanderDriverRequestSnapshot, replacements *strings.Replacer) []string {
		t.Helper()
		if request == nil {
			t.Fatalf("%s missing driver request", label)
		}
		requestCopy := *request
		if replacements != nil {
			requestCopy.Command = replacements.Replace(requestCopy.Command)
		}
		args, ok := missionCommanderDriverRequestCommandCLIArgs(t, &requestCopy)
		if !ok {
			t.Fatalf("%s driver request did not expose executable command: %+v", label, request)
		}
		withBase := append([]string{}, args[:2]...)
		withBase = append(withBase, baseArgs...)
		return append(withBase, args[2:]...)
	}

	out.Reset()
	if err := Run(driverRequestArgs("packet retirement preview", retirementRequest, strings.NewReplacer("<actor>", actor, "<reason>", reason)), &out); err != nil {
		t.Fatal(err)
	}
	var preview struct {
		Applied                     bool                                `json:"applied"`
		IsMutation                  bool                                `json:"isMutation"`
		PacketSHA256                string                              `json:"packetSha256"`
		IntegritySHA256             string                              `json:"integritySha256"`
		MissionCommanderAction      missionCommanderActionSnapshot      `json:"missionCommanderAction"`
		MissionCommanderActionQueue missionCommanderActionQueueSnapshot `json:"missionCommanderActionQueue"`
	}
	if err := json.Unmarshal(out.Bytes(), &preview); err != nil {
		t.Fatal(err)
	}
	if preview.Applied || preview.IsMutation || preview.PacketSHA256 == "" || preview.IntegritySHA256 == "" || preview.MissionCommanderAction.State != "needs-reviewer-packet-retirement-apply" || !strings.Contains(preview.MissionCommanderAction.PrimaryCommand, "-ExpectedPacketSha256") || !strings.Contains(preview.MissionCommanderAction.PrimaryCommand, "-ExpectedIntegritySha256") {
		t.Fatalf("unexpected retirement preview: %+v", preview)
	}
	retirementApplyRequest := requireMissionCommanderDriverRequest(t, preview.MissionCommanderActionQueue, "preview-command", "preview-current", preview.MissionCommanderAction.PrimaryCommand, true, false, true)

	out.Reset()
	if err := Run(driverRequestArgs("packet retirement text preview", retirementRequest, strings.NewReplacer("<actor>", actor, "<reason>", reason, "-Format json", "-Format text")), &out); err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(out.String(), "plan-subagents reviewer packet retirement：") || strings.HasPrefix(strings.TrimSpace(out.String()), "{") {
		t.Fatalf("unexpected retirement text output: %s", out.String())
	}

	out.Reset()
	if err := Run(reviewerPrimaryCommandCLIArgs(t, baseArgs, retirementApplyRequest.Command), &out); err != nil {
		t.Fatal(err)
	}
	var applied subagents.ReviewerPacketRetirementResult
	if err := json.Unmarshal(out.Bytes(), &applied); err != nil {
		t.Fatal(err)
	}
	if !applied.Applied || applied.Replay || applied.Mode != "reviewer-packet-retirement" || applied.MissionCommanderAction.State != "reviewer-packet-retired" {
		t.Fatalf("unexpected retirement apply: %+v", applied)
	}
	retirementBytes, err := os.ReadFile(applied.RetirementPath)
	if err != nil {
		t.Fatal(err)
	}

	out.Reset()
	if err := Run(reviewerPrimaryCommandCLIArgs(t, baseArgs, retirementApplyRequest.Command), &out); err != nil {
		t.Fatal(err)
	}
	var replay subagents.ReviewerPacketRetirementResult
	if err := json.Unmarshal(out.Bytes(), &replay); err != nil {
		t.Fatal(err)
	}
	if !replay.Applied || !replay.Replay || replay.Mode != "already-retired" || replay.PacketID != applied.PacketID || replay.RetirementPath != applied.RetirementPath || replay.MissionCommanderAction.State != "reviewer-packet-retired" || !containsSubstring(replay.NextSteps, "already exists") || !containsSubstring(replay.NextSteps, "do not dispatch") {
		t.Fatalf("duplicate retirement apply did not return already-retired replay handoff: %+v", replay)
	}
	replayedRetirementBytes, err := os.ReadFile(replay.RetirementPath)
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(retirementBytes, replayedRetirementBytes) {
		t.Fatalf("duplicate retirement apply rewrote receipt bytes")
	}

	out.Reset()
	textApplyCommand := strings.Replace(retirementApplyRequest.Command, "-Format json", "-Format text", 1)
	if err := Run(reviewerPrimaryCommandCLIArgs(t, baseArgs, textApplyCommand), &out); err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(out.String(), "mode=already-retired") || !strings.Contains(out.String(), "replay=true") || !strings.Contains(out.String(), "exact reviewer packet retirement receipt already exists") {
		t.Fatalf("duplicate retirement text did not expose replay handoff:\n%s", out.String())
	}

	out.Reset()
	if err := Run([]string{"-Command", "status", "-Target", caseRoot, "-Pack", "_template", "-Format", "json"}, &out); err != nil {
		t.Fatal(err)
	}
	var retiredStatus struct {
		CaseMission struct {
			ReviewerDispatchIntakeHandoffs    []reviewerDispatchIntakeCLIItem        `json:"reviewerDispatchIntakeHandoffs"`
			ReviewerDispatchIntakeSummary     reviewerDispatchIntakeSummaryCLIItem   `json:"reviewerDispatchIntakeSummary"`
			ReviewerPacketRetirementHandoffs  []reviewerPacketRetirementCLIItem      `json:"reviewerPacketRetirementHandoffs"`
			ReviewerPacketRetirementSummary   reviewerPacketRetirementSummaryCLIItem `json:"reviewerPacketRetirementSummary"`
			ReviewerDispatchIntakeActionQueue missionCommanderActionQueueSnapshot    `json:"reviewerDispatchIntakeActionQueue"`
		} `json:"caseMission"`
	}
	if err := json.Unmarshal(out.Bytes(), &retiredStatus); err != nil {
		t.Fatalf("retired status JSON did not decode: %v\n%s", err, out.String())
	}
	if len(retiredStatus.CaseMission.ReviewerDispatchIntakeHandoffs) != 0 || retiredStatus.CaseMission.ReviewerDispatchIntakeSummary.Total != 0 || retiredStatus.CaseMission.ReviewerDispatchIntakeActionQueue.Counts.Total != 0 {
		t.Fatalf("retired status kept invalid packet as open reviewer dispatch: %+v", retiredStatus.CaseMission)
	}
	assertReviewerPacketRetirementClosure(t, "status after retirement", retiredStatus.CaseMission.ReviewerPacketRetirementHandoffs, retiredStatus.CaseMission.ReviewerPacketRetirementSummary, applied.PacketID, "feature-review", applied.PacketSHA256, applied.IntegritySHA256)

	out.Reset()
	if err := Run([]string{"-Command", "status", "-Target", caseRoot, "-Pack", "_template", "-Format", "text"}, &out); err != nil {
		t.Fatal(err)
	}
	assertReviewerPacketRetirementText(t, "status text after retirement", out.String(), "feature-review")
	assertNoReviewerPacketRetirementReopensInvalidPacket(t, "status text after retirement", out.String())

	out.Reset()
	if err := Run([]string{"-Command", "handoff", "-Target", caseRoot, "-Pack", "_template", "review", "-WhatIf", "-Format", "json"}, &out); err != nil {
		t.Fatal(err)
	}
	var retiredHandoff struct {
		ReviewerDispatchIntakeHandoffs   []reviewerDispatchIntakeCLIItem        `json:"reviewerDispatchIntakeHandoffs"`
		ReviewerDispatchIntakeSummary    reviewerDispatchIntakeSummaryCLIItem   `json:"reviewerDispatchIntakeSummary"`
		ReviewerPacketRetirementHandoffs []reviewerPacketRetirementCLIItem      `json:"reviewerPacketRetirementHandoffs"`
		ReviewerPacketRetirementSummary  reviewerPacketRetirementSummaryCLIItem `json:"reviewerPacketRetirementSummary"`
	}
	if err := json.Unmarshal(out.Bytes(), &retiredHandoff); err != nil {
		t.Fatalf("retired handoff JSON did not decode: %v\n%s", err, out.String())
	}
	if len(retiredHandoff.ReviewerDispatchIntakeHandoffs) != 0 || retiredHandoff.ReviewerDispatchIntakeSummary.Total != 0 {
		t.Fatalf("retired handoff kept invalid packet as open reviewer dispatch: %+v", retiredHandoff)
	}
	assertReviewerPacketRetirementClosure(t, "handoff after retirement", retiredHandoff.ReviewerPacketRetirementHandoffs, retiredHandoff.ReviewerPacketRetirementSummary, applied.PacketID, "feature-review", applied.PacketSHA256, applied.IntegritySHA256)

	out.Reset()
	if err := Run([]string{"-Command", "handoff", "-Target", caseRoot, "-Pack", "_template", "review", "-WhatIf", "-Format", "text"}, &out); err != nil {
		t.Fatal(err)
	}
	assertReviewerPacketRetirementText(t, "handoff text after retirement", out.String(), "feature-review")
	assertNoReviewerPacketRetirementReopensInvalidPacket(t, "handoff text after retirement", out.String())

	out.Reset()
	if err := Run([]string{"-Command", "continue", "-Target", caseRoot, "-Pack", "_template", "review", "-WhatIf", "-Format", "json"}, &out); err != nil {
		t.Fatal(err)
	}
	var retiredContinuePreview struct {
		Applied                          bool                                   `json:"applied"`
		Blocked                          bool                                   `json:"blocked"`
		ReviewerDispatchIntakeHandoffs   []reviewerDispatchIntakeCLIItem        `json:"reviewerDispatchIntakeHandoffs"`
		ReviewerDispatchIntakeSummary    reviewerDispatchIntakeSummaryCLIItem   `json:"reviewerDispatchIntakeSummary"`
		ReviewerPacketRetirementHandoffs []reviewerPacketRetirementCLIItem      `json:"reviewerPacketRetirementHandoffs"`
		ReviewerPacketRetirementSummary  reviewerPacketRetirementSummaryCLIItem `json:"reviewerPacketRetirementSummary"`
		MissionCommanderActionQueue      missionCommanderActionQueueSnapshot    `json:"missionCommanderActionQueue"`
	}
	if err := json.Unmarshal(out.Bytes(), &retiredContinuePreview); err != nil {
		t.Fatalf("retired continue preview JSON did not decode: %v\n%s", err, out.String())
	}
	if retiredContinuePreview.Applied || retiredContinuePreview.Blocked || len(retiredContinuePreview.ReviewerDispatchIntakeHandoffs) != 0 || retiredContinuePreview.ReviewerDispatchIntakeSummary.Total != 0 {
		t.Fatalf("retired continue preview kept reviewer blocker: %+v", retiredContinuePreview)
	}
	if current := retiredContinuePreview.MissionCommanderActionQueue.CurrentAction; current != nil && current.Source == "reviewerDispatchIntakeHandoffs" {
		t.Fatalf("retired continue preview current action still points at reviewer dispatch: %+v", current)
	}
	assertReviewerPacketRetirementClosure(t, "continue preview after retirement", retiredContinuePreview.ReviewerPacketRetirementHandoffs, retiredContinuePreview.ReviewerPacketRetirementSummary, applied.PacketID, "feature-review", applied.PacketSHA256, applied.IntegritySHA256)

	out.Reset()
	if err := Run([]string{"-Command", "continue", "-Target", caseRoot, "-Pack", "_template", "review", "-WhatIf", "-Format", "text"}, &out); err != nil {
		t.Fatal(err)
	}
	assertReviewerPacketRetirementText(t, "continue text after retirement", out.String(), "feature-review")
	assertNoReviewerPacketRetirementReopensInvalidPacket(t, "continue text after retirement", out.String())

	out.Reset()
	if err := Run([]string{"-Command", "continue", "-Target", caseRoot, "-Pack", "_template", "review", "-Apply", "-Format", "json"}, &out); err != nil {
		t.Fatal(err)
	}
	var retiredContinueApply struct {
		RunID                            string                                 `json:"runId"`
		Applied                          bool                                   `json:"applied"`
		Blocked                          bool                                   `json:"blocked"`
		ReviewerDispatchIntakeHandoffs   []reviewerDispatchIntakeCLIItem        `json:"reviewerDispatchIntakeHandoffs"`
		ReviewerDispatchIntakeSummary    reviewerDispatchIntakeSummaryCLIItem   `json:"reviewerDispatchIntakeSummary"`
		ReviewerPacketRetirementHandoffs []reviewerPacketRetirementCLIItem      `json:"reviewerPacketRetirementHandoffs"`
		ReviewerPacketRetirementSummary  reviewerPacketRetirementSummaryCLIItem `json:"reviewerPacketRetirementSummary"`
		Writes                           []startWrite                           `json:"writes"`
	}
	if err := json.Unmarshal(out.Bytes(), &retiredContinueApply); err != nil {
		t.Fatalf("retired continue apply JSON did not decode: %v\n%s", err, out.String())
	}
	if !retiredContinueApply.Applied || retiredContinueApply.Blocked || retiredContinueApply.RunID == "" || retiredContinueApply.RunID == "run-preview" || len(retiredContinueApply.ReviewerDispatchIntakeHandoffs) != 0 || retiredContinueApply.ReviewerDispatchIntakeSummary.Total != 0 {
		t.Fatalf("retired continue apply kept reviewer blocker or omitted run: %+v", retiredContinueApply)
	}
	assertReviewerPacketRetirementClosure(t, "continue apply after retirement", retiredContinueApply.ReviewerPacketRetirementHandoffs, retiredContinueApply.ReviewerPacketRetirementSummary, applied.PacketID, "feature-review", applied.PacketSHA256, applied.IntegritySHA256)
	resumePath := assertStartWrite(t, retiredContinueApply.Writes, ".rekit/lanes/feature-review/prompts/RESUME.md", "refresh").TargetPath
	checkpointPath := assertStartWrite(t, retiredContinueApply.Writes, ".rekit/lanes/feature-review/checkpoints/latest.json", "refresh").TargetPath
	statusPath := assertStartWrite(t, retiredContinueApply.Writes, ".rekit/runs/"+retiredContinueApply.RunID+"/status.json", "write").TargetPath
	digestPath := assertStartWrite(t, retiredContinueApply.Writes, ".rekit/runs/"+retiredContinueApply.RunID+"/digest.md", "write").TargetPath

	statusBytes, err := os.ReadFile(statusPath)
	if err != nil {
		t.Fatal(err)
	}
	var runStatus struct {
		ReviewerDispatchIntakeHandoffs   []reviewerDispatchIntakeCLIItem        `json:"reviewerDispatchIntakeHandoffs"`
		ReviewerDispatchIntakeSummary    reviewerDispatchIntakeSummaryCLIItem   `json:"reviewerDispatchIntakeSummary"`
		ReviewerPacketRetirementHandoffs []reviewerPacketRetirementCLIItem      `json:"reviewerPacketRetirementHandoffs"`
		ReviewerPacketRetirementSummary  reviewerPacketRetirementSummaryCLIItem `json:"reviewerPacketRetirementSummary"`
	}
	if err := json.Unmarshal(statusBytes, &runStatus); err != nil {
		t.Fatalf("retired continue run status did not decode: %v\n%s", err, string(statusBytes))
	}
	if len(runStatus.ReviewerDispatchIntakeHandoffs) != 0 || runStatus.ReviewerDispatchIntakeSummary.Total != 0 {
		t.Fatalf("retired continue run status kept reviewer blocker: %+v", runStatus)
	}
	assertReviewerPacketRetirementClosure(t, "continue run status after retirement", runStatus.ReviewerPacketRetirementHandoffs, runStatus.ReviewerPacketRetirementSummary, applied.PacketID, "feature-review", applied.PacketSHA256, applied.IntegritySHA256)

	checkpointBytes, err := os.ReadFile(checkpointPath)
	if err != nil {
		t.Fatal(err)
	}
	var checkpoint struct {
		ReviewerDispatchIntakeHandoffs   []reviewerDispatchIntakeCLIItem        `json:"reviewerDispatchIntakeHandoffs"`
		ReviewerDispatchIntakeSummary    reviewerDispatchIntakeSummaryCLIItem   `json:"reviewerDispatchIntakeSummary"`
		ReviewerPacketRetirementHandoffs []reviewerPacketRetirementCLIItem      `json:"reviewerPacketRetirementHandoffs"`
		ReviewerPacketRetirementSummary  reviewerPacketRetirementSummaryCLIItem `json:"reviewerPacketRetirementSummary"`
	}
	if err := json.Unmarshal(checkpointBytes, &checkpoint); err != nil {
		t.Fatalf("retired checkpoint did not decode: %v\n%s", err, string(checkpointBytes))
	}
	if len(checkpoint.ReviewerDispatchIntakeHandoffs) != 0 || checkpoint.ReviewerDispatchIntakeSummary.Total != 0 {
		t.Fatalf("retired checkpoint kept reviewer blocker: %+v", checkpoint)
	}
	assertReviewerPacketRetirementClosure(t, "checkpoint after retirement", checkpoint.ReviewerPacketRetirementHandoffs, checkpoint.ReviewerPacketRetirementSummary, applied.PacketID, "feature-review", applied.PacketSHA256, applied.IntegritySHA256)

	resumeBytes, err := os.ReadFile(resumePath)
	if err != nil {
		t.Fatal(err)
	}
	digestBytes, err := os.ReadFile(digestPath)
	if err != nil {
		t.Fatal(err)
	}
	for label, text := range map[string]string{"lane RESUME after retirement": string(resumeBytes), "continue digest after retirement": string(digestBytes)} {
		assertReviewerPacketRetirementText(t, label, text, "feature-review")
		assertNoReviewerPacketRetirementReopensInvalidPacket(t, label, text)
	}

	regeneratedRoot := filepath.Join(caseRoot, ".rekit", "reviews", "regenerated-after-retirement")
	out.Reset()
	if err := Run([]string{"-Command", "plan-subagents", "-Target", caseRoot, "-Pack", "_template", "-TaskType", "feature-analysis", "-Items", "beta", "-Lane", "feature-review", "-ReviewOutputDir", regeneratedRoot, "-Format", "json"}, &out); err != nil {
		t.Fatal(err)
	}
	regeneratedPlan := decodePlanSubagentsResult(t, out.Bytes())
	regeneratedPacket := decodePlanSubagentsPacket(t, regeneratedPlan.PacketPath)
	if regeneratedPlan.PacketPath == plan.PacketPath || regeneratedPacket.PacketID == applied.PacketID || regeneratedPacket.PacketID == "" {
		t.Fatalf("regenerated packet did not create a fresh canonical packet: old=%s/%s regenerated=%s/%s", plan.PacketPath, applied.PacketID, regeneratedPlan.PacketPath, regeneratedPacket.PacketID)
	}

	out.Reset()
	if err := Run([]string{"-Command", "status", "-Target", caseRoot, "-Pack", "_template", "-Format", "json"}, &out); err != nil {
		t.Fatal(err)
	}
	var regeneratedStatus struct {
		CaseMission struct {
			ReviewerDispatchIntakeHandoffs    []reviewerDispatchIntakeCLIItem        `json:"reviewerDispatchIntakeHandoffs"`
			ReviewerDispatchIntakeSummary     reviewerDispatchIntakeSummaryCLIItem   `json:"reviewerDispatchIntakeSummary"`
			ReviewerDispatchIntakeActionQueue missionCommanderActionQueueSnapshot    `json:"reviewerDispatchIntakeActionQueue"`
			ReviewerPacketRetirementHandoffs  []reviewerPacketRetirementCLIItem      `json:"reviewerPacketRetirementHandoffs"`
			ReviewerPacketRetirementSummary   reviewerPacketRetirementSummaryCLIItem `json:"reviewerPacketRetirementSummary"`
			MissionCommanderActionQueue       missionCommanderActionQueueSnapshot    `json:"missionCommanderActionQueue"`
		} `json:"caseMission"`
	}
	if err := json.Unmarshal(out.Bytes(), &regeneratedStatus); err != nil {
		t.Fatalf("regenerated status JSON did not decode: %v\n%s", err, out.String())
	}
	assertReviewerPacketRetirementClosure(t, "status after regeneration", regeneratedStatus.CaseMission.ReviewerPacketRetirementHandoffs, regeneratedStatus.CaseMission.ReviewerPacketRetirementSummary, applied.PacketID, "feature-review", applied.PacketSHA256, applied.IntegritySHA256)
	assertReviewerDispatchIntakeSummary(t, "status after regeneration", regeneratedStatus.CaseMission.ReviewerDispatchIntakeSummary, 1, 1, 0, "shard-01", "ready-for-reviewer-dispatch")
	regeneratedDispatch, ok := reviewerDispatchIntakeByShard(regeneratedStatus.CaseMission.ReviewerDispatchIntakeHandoffs, "shard-01")
	if !ok || regeneratedDispatch.PacketID != regeneratedPacket.PacketID || regeneratedDispatch.PacketPath != regeneratedPlan.PacketPath || regeneratedDispatch.PacketID == applied.PacketID || regeneratedDispatch.TargetLane != "feature-review" || regeneratedDispatch.State != "ready-for-reviewer-dispatch" || regeneratedDispatch.PacketRetirementPreviewCommand != "" || !strings.Contains(regeneratedDispatch.DispatchCommand, "dispatch read-only reviewer") {
		t.Fatalf("status after regeneration did not promote fresh packet dispatch: dispatch=%+v regenerated=%+v old=%+v", regeneratedDispatch, regeneratedPlan, applied)
	}
	for _, item := range regeneratedStatus.CaseMission.ReviewerDispatchIntakeHandoffs {
		if item.PacketID == applied.PacketID || item.PacketPath == plan.PacketPath {
			t.Fatalf("status after regeneration reopened retired packet as dispatch: %+v", item)
		}
	}
	if queue := regeneratedStatus.CaseMission.ReviewerDispatchIntakeActionQueue; queue.Counts.Total != 1 || queue.Counts.RequiresReview != 1 || queue.CurrentAction == nil || queue.CurrentAction.Source != "reviewerDispatchIntakeHandoffs" || !strings.Contains(queue.CurrentAction.Command, "-RecordReviewerDispatch") {
		t.Fatalf("status after regeneration omitted reviewer-only dispatch current action: %+v", queue)
	}
	if queue := regeneratedStatus.CaseMission.MissionCommanderActionQueue; queue.CurrentAction == nil || queue.CurrentAction.Source != "reviewerDispatchIntakeHandoffs" || !strings.Contains(queue.CurrentAction.Command, "-RecordReviewerDispatch") {
		t.Fatalf("status after regeneration did not prioritize fresh reviewer dispatch as case mission current action: %+v", queue)
	}

	out.Reset()
	if err := Run([]string{"-Command", "status", "-Target", caseRoot, "-Pack", "_template", "-Format", "text"}, &out); err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(out.String(), "status case mission reviewer dispatch intake：lane=feature-review shard=shard-01 state=ready-for-reviewer-dispatch") || !strings.Contains(out.String(), "status case mission reviewer dispatch queue") {
		t.Fatalf("status text after regeneration omitted fresh reviewer dispatch:\n%s", out.String())
	}
	assertReviewerPacketRetirementText(t, "status text after regeneration", out.String(), "feature-review")
	assertNoReviewerPacketRetirementReopensInvalidPacket(t, "status text after regeneration", out.String())

	out.Reset()
	if err := Run([]string{"-Command", "handoff", "-Target", caseRoot, "-Pack", "_template", "review", "-WhatIf", "-Format", "json"}, &out); err != nil {
		t.Fatal(err)
	}
	var regeneratedHandoff struct {
		ReviewerDispatchIntakeHandoffs   []reviewerDispatchIntakeCLIItem        `json:"reviewerDispatchIntakeHandoffs"`
		ReviewerDispatchIntakeSummary    reviewerDispatchIntakeSummaryCLIItem   `json:"reviewerDispatchIntakeSummary"`
		ReviewerPacketRetirementHandoffs []reviewerPacketRetirementCLIItem      `json:"reviewerPacketRetirementHandoffs"`
		ReviewerPacketRetirementSummary  reviewerPacketRetirementSummaryCLIItem `json:"reviewerPacketRetirementSummary"`
		Writes                           []startWrite                           `json:"writes"`
	}
	if err := json.Unmarshal(out.Bytes(), &regeneratedHandoff); err != nil {
		t.Fatalf("regenerated handoff JSON did not decode: %v\n%s", err, out.String())
	}
	assertReviewerPacketRetirementClosure(t, "handoff after regeneration", regeneratedHandoff.ReviewerPacketRetirementHandoffs, regeneratedHandoff.ReviewerPacketRetirementSummary, applied.PacketID, "feature-review", applied.PacketSHA256, applied.IntegritySHA256)
	assertReviewerDispatchIntakeSummary(t, "handoff after regeneration", regeneratedHandoff.ReviewerDispatchIntakeSummary, 1, 1, 0, "shard-01", "ready-for-reviewer-dispatch")
	regeneratedHandoffDispatch, ok := reviewerDispatchIntakeByShard(regeneratedHandoff.ReviewerDispatchIntakeHandoffs, "shard-01")
	if !ok || regeneratedHandoffDispatch.PacketID != regeneratedPacket.PacketID || regeneratedHandoffDispatch.PacketID == applied.PacketID {
		t.Fatalf("handoff after regeneration did not preserve fresh packet dispatch: %+v", regeneratedHandoff.ReviewerDispatchIntakeHandoffs)
	}

	out.Reset()
	if err := runHashBoundHandoffApply(t, []string{"-Command", "handoff", "-Target", caseRoot, "-Pack", "_template", "review", "-Apply", "-Format", "json"}, &out); err != nil {
		t.Fatal(err)
	}
	var regeneratedHandoffApply struct {
		Applied                          bool                                   `json:"applied"`
		ReviewerDispatchIntakeHandoffs   []reviewerDispatchIntakeCLIItem        `json:"reviewerDispatchIntakeHandoffs"`
		ReviewerDispatchIntakeSummary    reviewerDispatchIntakeSummaryCLIItem   `json:"reviewerDispatchIntakeSummary"`
		ReviewerPacketRetirementHandoffs []reviewerPacketRetirementCLIItem      `json:"reviewerPacketRetirementHandoffs"`
		ReviewerPacketRetirementSummary  reviewerPacketRetirementSummaryCLIItem `json:"reviewerPacketRetirementSummary"`
		Writes                           []startWrite                           `json:"writes"`
	}
	if err := json.Unmarshal(out.Bytes(), &regeneratedHandoffApply); err != nil {
		t.Fatalf("regenerated handoff apply JSON did not decode: %v\n%s", err, out.String())
	}
	if !regeneratedHandoffApply.Applied {
		t.Fatalf("regenerated handoff apply did not apply: %+v", regeneratedHandoffApply)
	}
	assertReviewerPacketRetirementClosure(t, "handoff apply after regeneration", regeneratedHandoffApply.ReviewerPacketRetirementHandoffs, regeneratedHandoffApply.ReviewerPacketRetirementSummary, applied.PacketID, "feature-review", applied.PacketSHA256, applied.IntegritySHA256)
	assertReviewerDispatchIntakeSummary(t, "handoff apply after regeneration", regeneratedHandoffApply.ReviewerDispatchIntakeSummary, 1, 1, 0, "shard-01", "ready-for-reviewer-dispatch")
	latestHandoffPath := assertStartWrite(t, regeneratedHandoffApply.Writes, ".rekit/handovers/feature-review-latest.md", "write-latest-lane-handoff").TargetPath
	latestHandoffBytes, err := os.ReadFile(latestHandoffPath)
	if err != nil {
		t.Fatal(err)
	}
	latestHandoffText := string(latestHandoffBytes)
	if !strings.Contains(latestHandoffText, "## Reviewer dispatch intake handoff") || !strings.Contains(latestHandoffText, "dispatch intake: lane=feature-review shard=shard-01 state=ready-for-reviewer-dispatch") || !strings.Contains(latestHandoffText, regeneratedPacket.PacketID) {
		t.Fatalf("latest handoff after regeneration omitted fresh dispatch:\n%s", latestHandoffText)
	}
	assertReviewerPacketRetirementText(t, "latest handoff after regeneration", latestHandoffText, "feature-review")
	assertNoReviewerPacketRetirementReopensInvalidPacket(t, "latest handoff after regeneration", latestHandoffText)

	out.Reset()
	if err := Run([]string{"-Command", "continue", "-Target", caseRoot, "-Pack", "_template", "review", "-WhatIf", "-Format", "json"}, &out); err != nil {
		t.Fatal(err)
	}
	var regeneratedContinuePreview struct {
		Applied                          bool                                   `json:"applied"`
		Blocked                          bool                                   `json:"blocked"`
		ReviewerDispatchIntakeHandoffs   []reviewerDispatchIntakeCLIItem        `json:"reviewerDispatchIntakeHandoffs"`
		ReviewerDispatchIntakeSummary    reviewerDispatchIntakeSummaryCLIItem   `json:"reviewerDispatchIntakeSummary"`
		ReviewerPacketRetirementHandoffs []reviewerPacketRetirementCLIItem      `json:"reviewerPacketRetirementHandoffs"`
		ReviewerPacketRetirementSummary  reviewerPacketRetirementSummaryCLIItem `json:"reviewerPacketRetirementSummary"`
		MissionCommanderActionQueue      missionCommanderActionQueueSnapshot    `json:"missionCommanderActionQueue"`
	}
	if err := json.Unmarshal(out.Bytes(), &regeneratedContinuePreview); err != nil {
		t.Fatalf("regenerated continue preview JSON did not decode: %v\n%s", err, out.String())
	}
	if regeneratedContinuePreview.Applied || !regeneratedContinuePreview.Blocked || regeneratedContinuePreview.MissionCommanderActionQueue.CurrentAction == nil || regeneratedContinuePreview.MissionCommanderActionQueue.CurrentAction.Source != "reviewerDispatchIntakeHandoffs" {
		t.Fatalf("continue preview after regeneration did not block on fresh reviewer dispatch: %+v", regeneratedContinuePreview)
	}
	assertReviewerPacketRetirementClosure(t, "continue preview after regeneration", regeneratedContinuePreview.ReviewerPacketRetirementHandoffs, regeneratedContinuePreview.ReviewerPacketRetirementSummary, applied.PacketID, "feature-review", applied.PacketSHA256, applied.IntegritySHA256)
	assertReviewerDispatchIntakeSummary(t, "continue preview after regeneration", regeneratedContinuePreview.ReviewerDispatchIntakeSummary, 1, 1, 0, "shard-01", "ready-for-reviewer-dispatch")
	regeneratedContinueDispatch, ok := reviewerDispatchIntakeByShard(regeneratedContinuePreview.ReviewerDispatchIntakeHandoffs, "shard-01")
	if !ok || regeneratedContinueDispatch.PacketID != regeneratedPacket.PacketID || regeneratedContinueDispatch.PacketID == applied.PacketID {
		t.Fatalf("continue preview after regeneration did not preserve fresh packet dispatch: %+v", regeneratedContinuePreview.ReviewerDispatchIntakeHandoffs)
	}

	out.Reset()
	if err := Run([]string{"-Command", "continue", "-Target", caseRoot, "-Pack", "_template", "review", "-WhatIf", "-Format", "text"}, &out); err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(out.String(), "continue reviewer dispatch intake：lane=feature-review shard=shard-01 state=ready-for-reviewer-dispatch") {
		t.Fatalf("continue text after regeneration omitted fresh reviewer dispatch:\n%s", out.String())
	}
	assertReviewerPacketRetirementText(t, "continue text after regeneration", out.String(), "feature-review")
	assertNoReviewerPacketRetirementReopensInvalidPacket(t, "continue text after regeneration", out.String())

	out.Reset()
	if err := Run([]string{"-Command", "continue", "-Target", caseRoot, "-Pack", "_template", "review", "-Apply", "-Format", "json"}, &out); err != nil {
		t.Fatal(err)
	}
	var regeneratedContinueApply struct {
		Applied                          bool                                   `json:"applied"`
		Blocked                          bool                                   `json:"blocked"`
		ReviewerDispatchIntakeHandoffs   []reviewerDispatchIntakeCLIItem        `json:"reviewerDispatchIntakeHandoffs"`
		ReviewerDispatchIntakeSummary    reviewerDispatchIntakeSummaryCLIItem   `json:"reviewerDispatchIntakeSummary"`
		ReviewerPacketRetirementHandoffs []reviewerPacketRetirementCLIItem      `json:"reviewerPacketRetirementHandoffs"`
		ReviewerPacketRetirementSummary  reviewerPacketRetirementSummaryCLIItem `json:"reviewerPacketRetirementSummary"`
		Writes                           []startWrite                           `json:"writes"`
	}
	if err := json.Unmarshal(out.Bytes(), &regeneratedContinueApply); err != nil {
		t.Fatalf("regenerated continue apply JSON did not decode: %v\n%s", err, out.String())
	}
	if regeneratedContinueApply.Applied || !regeneratedContinueApply.Blocked || len(regeneratedContinueApply.Writes) != 0 {
		t.Fatalf("continue apply after regeneration should remain blocked with zero writes while reviewer dispatch is open: %+v", regeneratedContinueApply)
	}
	assertReviewerPacketRetirementClosure(t, "continue apply after regeneration", regeneratedContinueApply.ReviewerPacketRetirementHandoffs, regeneratedContinueApply.ReviewerPacketRetirementSummary, applied.PacketID, "feature-review", applied.PacketSHA256, applied.IntegritySHA256)
	assertReviewerDispatchIntakeSummary(t, "continue apply after regeneration", regeneratedContinueApply.ReviewerDispatchIntakeSummary, 1, 1, 0, "shard-01", "ready-for-reviewer-dispatch")
}
