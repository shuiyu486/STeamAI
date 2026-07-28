package subagents

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/shuiyu486/re-context-kits/internal/rekit/defaults"
	"github.com/shuiyu486/re-context-kits/internal/rekit/mission"
	"github.com/shuiyu486/re-context-kits/internal/rekit/workstream"
)

func TestReviewerSessionReceiptDispatchCompletionAndSourceCapture(t *testing.T) {
	repoRoot := filepath.Clean(filepath.Join("..", "..", ".."))
	caseRoot := filepath.Join(t.TempDir(), "case")
	writeReviewerIntakeCase(t, repoRoot, caseRoot)
	plan, err := WritePlan(repoRoot, caseRoot, defaults.DefaultPack, Options{TaskType: "feature-analysis", Items: "alpha", Lane: "feature-intake"})
	if err != nil {
		t.Fatal(err)
	}
	packet := readReviewerPacket(t, plan.PacketPath)
	if err := os.MkdirAll(filepath.Join(caseRoot, "workspace"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(caseRoot, "workspace", "review-evidence.md"), []byte("bounded reviewer evidence\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	handoff := packet.ShardHandoffs[0]
	dispatchOpt := ReviewerSessionDispatchOptions{PacketPath: plan.PacketPath, ShardID: handoff.ShardID, Lane: packet.TargetLane, Actor: "mission-commander", ReviewerHarness: "claude-code-agent", ReviewerSession: "reviewer-session-1", WhatIf: true}
	dispatchPreview, err := RecordReviewerSessionDispatch(repoRoot, caseRoot, defaults.DefaultPack, dispatchOpt)
	if err != nil {
		t.Fatal(err)
	}
	if dispatchPreview.Applied || dispatchPreview.DispatchID == "" || dispatchPreview.BindingSHA256 == "" || !strings.Contains(dispatchPreview.ApplyCommand, "-ExpectedReviewerDispatchBindingSha256") {
		t.Fatalf("unexpected dispatch preview: %+v", dispatchPreview)
	}
	if _, err := os.Lstat(dispatchPreview.ReceiptPath); !os.IsNotExist(err) {
		t.Fatalf("dispatch preview wrote receipt: %v", err)
	}
	dispatchOpt.WhatIf = false
	dispatchOpt.ExpectedBindingSHA256 = dispatchPreview.BindingSHA256
	dispatch, err := RecordReviewerSessionDispatch(repoRoot, caseRoot, defaults.DefaultPack, dispatchOpt)
	if err != nil {
		t.Fatal(err)
	}
	if !dispatch.Applied || dispatch.AlreadyRecorded || dispatch.ReceiptSHA256 == "" {
		t.Fatalf("unexpected dispatch apply: %+v", dispatch)
	}
	replayed, err := RecordReviewerSessionDispatch(repoRoot, caseRoot, defaults.DefaultPack, dispatchOpt)
	if err != nil || !replayed.AlreadyRecorded {
		t.Fatalf("dispatch replay: %+v err=%v", replayed, err)
	}

	inputPath := handoff.ReviewerStagingCommands.SourceCaptureInput
	if err := os.MkdirAll(filepath.Dir(inputPath), 0o755); err != nil {
		t.Fatal(err)
	}
	input := reviewerResultForPacket(t, packet, "accept", "accepted", nil)
	if err := os.WriteFile(inputPath, input, 0o644); err != nil {
		t.Fatal(err)
	}
	captureOpt := ReviewerResultSourceCaptureOptions{PacketPath: plan.PacketPath, ShardID: handoff.ShardID, InputPath: inputPath, Lane: packet.TargetLane, Actor: "mission-commander", WhatIf: true}
	if _, err := CaptureReviewerResultSource(repoRoot, caseRoot, defaults.DefaultPack, captureOpt); err == nil || !strings.Contains(err.Error(), "lacks a successful completion receipt") {
		t.Fatalf("source capture before completion error = %v", err)
	}

	handoffs, err := workstream.ReviewerDispatchIntakeHandoffs(caseRoot, mission.LedgerFacts{}, packet.TargetLane)
	if err != nil {
		t.Fatal(err)
	}
	if len(handoffs) != 1 || handoffs[0].State != "ready-for-reviewer-completion-receipt-preview" || handoffs[0].ReviewerDispatchID != dispatch.DispatchID || handoffs[0].ReviewerSession != dispatch.ReviewerSession || !strings.Contains(handoffs[0].ReviewerCompletionRecordCommand, "-RecordReviewerCompletion") {
		t.Fatalf("unexpected completion receipt handoff: %+v", handoffs)
	}

	completionOpt := ReviewerSessionCompletionOptions{PacketPath: plan.PacketPath, DispatchID: dispatch.DispatchID, Lane: packet.TargetLane, Actor: "mission-commander", Outcome: "succeeded", ExitStatus: "completed", ReviewerResultInputPath: inputPath, WhatIf: true}
	completionPreview, err := RecordReviewerSessionCompletion(repoRoot, caseRoot, defaults.DefaultPack, completionOpt)
	if err != nil {
		t.Fatal(err)
	}
	if completionPreview.DispatchReceiptSHA256 != dispatch.ReceiptSHA256 || completionPreview.ReviewerResultInputSHA256 == "" || !strings.Contains(completionPreview.ApplyCommand, "-ExpectedReviewerResultInputSha256") {
		t.Fatalf("unexpected completion preview: %+v", completionPreview)
	}
	completionOpt.WhatIf = false
	completionOpt.ExpectedDispatchReceiptSHA256 = completionPreview.DispatchReceiptSHA256
	completionOpt.ExpectedReviewerResultSHA256 = completionPreview.ReviewerResultInputSHA256
	completion, err := RecordReviewerSessionCompletion(repoRoot, caseRoot, defaults.DefaultPack, completionOpt)
	if err != nil {
		t.Fatal(err)
	}
	if !completion.Applied || completion.AlreadyRecorded || completion.Outcome != "succeeded" {
		t.Fatalf("unexpected completion apply: %+v", completion)
	}
	handoffs, err = workstream.ReviewerDispatchIntakeHandoffs(caseRoot, mission.LedgerFacts{}, packet.TargetLane)
	if err != nil {
		t.Fatal(err)
	}
	if len(handoffs) != 1 || handoffs[0].State != "ready-for-reviewer-result-source-capture-preview" || handoffs[0].ReviewerSessionOutcome != "succeeded" || handoffs[0].ReviewerCompletionReceiptSHA256 == "" {
		t.Fatalf("unexpected completed session handoff: %+v", handoffs)
	}
	capture, err := CaptureReviewerResultSource(repoRoot, caseRoot, defaults.DefaultPack, captureOpt)
	if err != nil {
		t.Fatal(err)
	}
	if capture.Applied || capture.InputSHA256 != completion.ReviewerResultInputSHA256 || capture.ReviewerResult.ReviewerSession != dispatch.ReviewerSession {
		t.Fatalf("unexpected receipt-bound source capture: %+v", capture)
	}
	if err := os.MkdirAll(filepath.Dir(handoff.ReviewerResultCandidatePath), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(handoff.ReviewerResultCandidatePath, input, 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(handoff.ReviewerResultPath, input, 0o644); err != nil {
		t.Fatal(err)
	}
	resultPath := handoff.ReviewerResultPath
	intake, err := IntakeReviewerResult(repoRoot, caseRoot, defaults.DefaultPack, ReviewerIntakeOptions{PacketPath: plan.PacketPath, ReviewerResultPath: resultPath, Lane: packet.TargetLane, Actor: "mission-commander", WhatIf: true})
	if err != nil {
		t.Fatal(err)
	}
	if intake.Verification == nil || intake.Decision == nil {
		t.Fatalf("receipt-bound intake omitted event previews: %+v", intake)
	}
	for _, event := range []map[string]any{intake.Verification.Event, intake.Decision.Event} {
		if mission.Value(event, "reviewerDispatchId") != dispatch.DispatchID || mission.Value(event, "reviewerHarness") != dispatch.ReviewerHarness || mission.Value(event, "reviewerDispatchReceiptSha256") != dispatch.ReceiptSHA256 || mission.Value(event, "reviewerCompletionReceiptSha256") != completion.ReceiptSHA256 || mission.Value(event, "reviewerResultInputSha256") != completion.ReviewerResultInputSHA256 {
			t.Fatalf("reviewer writeback omitted receipt provenance: %+v", event)
		}
	}
}

func TestReviewerSessionReceiptRejectsTamperedCompletionLineage(t *testing.T) {
	repoRoot := filepath.Clean(filepath.Join("..", "..", ".."))
	caseRoot := filepath.Join(t.TempDir(), "case")
	writeReviewerIntakeCase(t, repoRoot, caseRoot)
	plan, err := WritePlan(repoRoot, caseRoot, defaults.DefaultPack, Options{TaskType: "feature-analysis", Items: "alpha", Lane: "feature-intake"})
	if err != nil {
		t.Fatal(err)
	}
	packet := readReviewerPacket(t, plan.PacketPath)
	if err := os.MkdirAll(filepath.Join(caseRoot, "workspace"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(caseRoot, "workspace", "review-evidence.md"), []byte("bounded reviewer evidence\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	handoff := packet.ShardHandoffs[0]
	dispatchOpt := ReviewerSessionDispatchOptions{PacketPath: plan.PacketPath, ShardID: handoff.ShardID, Lane: packet.TargetLane, Actor: "mission-commander", ReviewerHarness: "claude-code-agent-tamper", ReviewerSession: "reviewer-session-1", WhatIf: true}
	dispatchPreview, err := RecordReviewerSessionDispatch(repoRoot, caseRoot, defaults.DefaultPack, dispatchOpt)
	if err != nil {
		t.Fatal(err)
	}
	dispatchOpt.WhatIf = false
	dispatchOpt.ExpectedBindingSHA256 = dispatchPreview.BindingSHA256
	dispatch, err := RecordReviewerSessionDispatch(repoRoot, caseRoot, defaults.DefaultPack, dispatchOpt)
	if err != nil {
		t.Fatal(err)
	}
	inputPath := handoff.ReviewerStagingCommands.SourceCaptureInput
	if err := os.MkdirAll(filepath.Dir(inputPath), 0o755); err != nil {
		t.Fatal(err)
	}
	input := reviewerResultForPacket(t, packet, "accept", "accepted", nil)
	if err := os.WriteFile(inputPath, input, 0o644); err != nil {
		t.Fatal(err)
	}
	completionOpt := ReviewerSessionCompletionOptions{PacketPath: plan.PacketPath, DispatchID: dispatch.DispatchID, Lane: packet.TargetLane, Actor: "mission-commander", Outcome: "succeeded", ExitStatus: "completed", ReviewerResultInputPath: inputPath, WhatIf: true}
	completionPreview, err := RecordReviewerSessionCompletion(repoRoot, caseRoot, defaults.DefaultPack, completionOpt)
	if err != nil {
		t.Fatal(err)
	}
	completionOpt.WhatIf = false
	completionOpt.ExpectedDispatchReceiptSHA256 = completionPreview.DispatchReceiptSHA256
	completionOpt.ExpectedReviewerResultSHA256 = completionPreview.ReviewerResultInputSHA256
	completion, err := RecordReviewerSessionCompletion(repoRoot, caseRoot, defaults.DefaultPack, completionOpt)
	if err != nil {
		t.Fatal(err)
	}
	completionBytes, err := os.ReadFile(completion.ReceiptPath)
	if err != nil {
		t.Fatal(err)
	}
	var receipt ReviewerSessionCompletionReceipt
	if err := json.Unmarshal(completionBytes, &receipt); err != nil {
		t.Fatal(err)
	}
	receipt.ReviewerHarness = "tampered-harness"
	completionBytes, err = json.MarshalIndent(receipt, "", "  ")
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(completion.ReceiptPath, append(completionBytes, '\n'), 0o600); err != nil {
		t.Fatal(err)
	}
	captureOpt := ReviewerResultSourceCaptureOptions{PacketPath: plan.PacketPath, ShardID: handoff.ShardID, InputPath: inputPath, Lane: packet.TargetLane, Actor: "mission-commander", WhatIf: true}
	if _, err := CaptureReviewerResultSource(repoRoot, caseRoot, defaults.DefaultPack, captureOpt); err == nil || !strings.Contains(err.Error(), "lacks a successful completion") {
		t.Fatalf("tampered completion source capture error = %v", err)
	}
	if err := os.MkdirAll(filepath.Dir(handoff.ReviewerResultCandidatePath), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(handoff.ReviewerResultCandidatePath, input, 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(handoff.ReviewerResultPath, input, 0o644); err != nil {
		t.Fatal(err)
	}
	if _, err := IntakeReviewerResult(repoRoot, caseRoot, defaults.DefaultPack, ReviewerIntakeOptions{PacketPath: plan.PacketPath, ReviewerResultPath: handoff.ReviewerResultPath, Lane: packet.TargetLane, Actor: "mission-commander", WhatIf: true}); err == nil || !strings.Contains(err.Error(), "requires a successful completion") {
		t.Fatalf("tampered completion intake error = %v", err)
	}
	handoffs, err := workstream.ReviewerDispatchIntakeHandoffs(caseRoot, mission.LedgerFacts{}, packet.TargetLane)
	if err != nil {
		t.Fatal(err)
	}
	if len(handoffs) != 1 || handoffs[0].State != "reviewer-session-receipt-invalid" {
		t.Fatalf("tampered completion did not fail closed in status: %+v", handoffs)
	}
}

func TestReviewerSessionCompletionRejectsTamperedDispatchBindings(t *testing.T) {
	repoRoot := filepath.Clean(filepath.Join("..", "..", ".."))
	caseRoot := filepath.Join(t.TempDir(), "case")
	writeReviewerIntakeCase(t, repoRoot, caseRoot)
	plan, err := WritePlan(repoRoot, caseRoot, defaults.DefaultPack, Options{TaskType: "feature-analysis", Items: "alpha", Lane: "feature-intake"})
	if err != nil {
		t.Fatal(err)
	}
	packet := readReviewerPacket(t, plan.PacketPath)
	handoff := packet.ShardHandoffs[0]
	dispatchOpt := ReviewerSessionDispatchOptions{PacketPath: plan.PacketPath, ShardID: handoff.ShardID, Lane: packet.TargetLane, Actor: "mission-commander", ReviewerHarness: "claude-code-agent", ReviewerSession: "reviewer-session-1", WhatIf: true}
	dispatchPreview, err := RecordReviewerSessionDispatch(repoRoot, caseRoot, defaults.DefaultPack, dispatchOpt)
	if err != nil {
		t.Fatal(err)
	}
	dispatchOpt.WhatIf = false
	dispatchOpt.ExpectedBindingSHA256 = dispatchPreview.BindingSHA256
	dispatch, err := RecordReviewerSessionDispatch(repoRoot, caseRoot, defaults.DefaultPack, dispatchOpt)
	if err != nil {
		t.Fatal(err)
	}
	inputPath := handoff.ReviewerStagingCommands.SourceCaptureInput
	if err := os.MkdirAll(filepath.Dir(inputPath), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(inputPath, reviewerResultForPacket(t, packet, "accept", "accepted", nil), 0o644); err != nil {
		t.Fatal(err)
	}
	dispatchBytes, err := os.ReadFile(dispatch.ReceiptPath)
	if err != nil {
		t.Fatal(err)
	}
	var receipt ReviewerSessionDispatchReceipt
	if err := json.Unmarshal(dispatchBytes, &receipt); err != nil {
		t.Fatal(err)
	}
	receipt.Items = []string{"tampered-item"}
	dispatchBytes, err = json.MarshalIndent(receipt, "", "  ")
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(dispatch.ReceiptPath, append(dispatchBytes, '\n'), 0o600); err != nil {
		t.Fatal(err)
	}
	completionOpt := ReviewerSessionCompletionOptions{PacketPath: plan.PacketPath, DispatchID: dispatch.DispatchID, Lane: packet.TargetLane, Actor: "mission-commander", Outcome: "succeeded", ExitStatus: "completed", ReviewerResultInputPath: inputPath, WhatIf: true}
	if _, err := RecordReviewerSessionCompletion(repoRoot, caseRoot, defaults.DefaultPack, completionOpt); err == nil || !strings.Contains(err.Error(), "does not match current immutable packet") {
		t.Fatalf("completion with tampered dispatch error = %v", err)
	}
	handoffs, err := workstream.ReviewerDispatchIntakeHandoffs(caseRoot, mission.LedgerFacts{}, packet.TargetLane)
	if err != nil {
		t.Fatal(err)
	}
	if len(handoffs) != 1 || handoffs[0].State != "reviewer-session-receipt-invalid" {
		t.Fatalf("tampered dispatch did not fail closed in status: %+v", handoffs)
	}
}

func TestReviewerSessionReceiptRequiresOriginalInputThroughIntake(t *testing.T) {
	repoRoot := filepath.Clean(filepath.Join("..", "..", ".."))
	caseRoot := filepath.Join(t.TempDir(), "case")
	writeReviewerIntakeCase(t, repoRoot, caseRoot)
	plan, err := WritePlan(repoRoot, caseRoot, defaults.DefaultPack, Options{TaskType: "feature-analysis", Items: "alpha", Lane: "feature-intake"})
	if err != nil {
		t.Fatal(err)
	}
	packet := readReviewerPacket(t, plan.PacketPath)
	if err := os.MkdirAll(filepath.Join(caseRoot, "workspace"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(caseRoot, "workspace", "review-evidence.md"), []byte("bounded reviewer evidence\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	handoff := packet.ShardHandoffs[0]
	input := reviewerResultForPacket(t, packet, "accept", "accepted", nil)
	writeReviewerSessionReceiptsForResult(t, handoff, input)
	if err := os.MkdirAll(filepath.Dir(handoff.ReviewerResultCandidatePath), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(handoff.ReviewerResultCandidatePath, input, 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(handoff.ReviewerResultPath, input, 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.Remove(handoff.ReviewerStagingCommands.SourceCaptureInput); err != nil {
		t.Fatal(err)
	}
	handoffs, err := workstream.ReviewerDispatchIntakeHandoffs(caseRoot, mission.LedgerFacts{}, packet.TargetLane)
	if err != nil {
		t.Fatal(err)
	}
	if len(handoffs) != 1 || handoffs[0].State != "reviewer-session-receipt-invalid" || handoffs[0].ReviewerResultInputState != "missing" {
		t.Fatalf("missing original input did not block intake status: %+v", handoffs)
	}
	if _, err := IntakeReviewerResult(repoRoot, caseRoot, defaults.DefaultPack, ReviewerIntakeOptions{PacketPath: plan.PacketPath, ReviewerResultPath: handoff.ReviewerResultPath, Lane: packet.TargetLane, Actor: "mission-commander", WhatIf: true}); err == nil || !strings.Contains(err.Error(), "requires a successful completion") {
		t.Fatalf("intake without original input error = %v", err)
	}
}

func TestReviewerSessionReceiptRequiredForManagedPacket(t *testing.T) {
	repoRoot := filepath.Clean(filepath.Join("..", "..", ".."))
	caseRoot := filepath.Join(t.TempDir(), "case")
	writeReviewerIntakeCase(t, repoRoot, caseRoot)
	plan, err := WritePlan(repoRoot, caseRoot, defaults.DefaultPack, Options{TaskType: "feature-analysis", Items: "alpha", Lane: "feature-intake"})
	if err != nil {
		t.Fatal(err)
	}
	packet := readReviewerPacket(t, plan.PacketPath)
	if err := os.MkdirAll(filepath.Join(caseRoot, "workspace"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(caseRoot, "workspace", "review-evidence.md"), []byte("bounded reviewer evidence\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	handoff := packet.ShardHandoffs[0]
	inputPath := handoff.ReviewerStagingCommands.SourceCaptureInput
	if err := os.MkdirAll(filepath.Dir(inputPath), 0o755); err != nil {
		t.Fatal(err)
	}
	input := reviewerResultForPacket(t, packet, "accept", "accepted", nil)
	if err := os.WriteFile(inputPath, input, 0o644); err != nil {
		t.Fatal(err)
	}
	captureOpt := ReviewerResultSourceCaptureOptions{PacketPath: plan.PacketPath, ShardID: handoff.ShardID, InputPath: inputPath, Lane: packet.TargetLane, Actor: "mission-commander", WhatIf: true}
	if _, err := CaptureReviewerResultSource(repoRoot, caseRoot, defaults.DefaultPack, captureOpt); err == nil || !strings.Contains(err.Error(), "lacks a durable dispatch") {
		t.Fatalf("source capture without dispatch receipt error = %v", err)
	}
	if err := os.MkdirAll(filepath.Dir(handoff.ReviewerResultCandidatePath), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(handoff.ReviewerResultCandidatePath, input, 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(handoff.ReviewerResultPath, input, 0o644); err != nil {
		t.Fatal(err)
	}
	resultPath := handoff.ReviewerResultPath
	verificationPath := filepath.Join(caseRoot, ".rekit", "facts", "verifications.jsonl")
	decisionPath := filepath.Join(caseRoot, ".rekit", "facts", "decisions.jsonl")
	verificationBefore := readOptionalFile(t, verificationPath)
	decisionBefore := readOptionalFile(t, decisionPath)
	if _, err := IntakeReviewerResult(repoRoot, caseRoot, defaults.DefaultPack, ReviewerIntakeOptions{PacketPath: plan.PacketPath, ReviewerResultPath: resultPath, Lane: packet.TargetLane, Actor: "mission-commander", WhatIf: true}); err == nil || !strings.Contains(err.Error(), "requires a durable dispatch") {
		t.Fatalf("intake without dispatch receipt error = %v", err)
	}
	if readOptionalFile(t, verificationPath) != verificationBefore || readOptionalFile(t, decisionPath) != decisionBefore {
		t.Fatal("receipt-gated intake changed verification or decision facts")
	}
	handoffs, err := workstream.ReviewerDispatchIntakeHandoffs(caseRoot, mission.LedgerFacts{}, packet.TargetLane)
	if err != nil {
		t.Fatal(err)
	}
	if len(handoffs) != 1 || handoffs[0].State != "ready-for-reviewer-dispatch" || !strings.Contains(handoffs[0].ReviewerDispatchRecordCommand, "-RecordReviewerDispatch") {
		t.Fatalf("managed packet without receipts did not remain dispatch-gated: %+v", handoffs)
	}
}

func TestReviewerSessionLifecycleSelectsAttemptMatchingCurrentInput(t *testing.T) {
	repoRoot := filepath.Clean(filepath.Join("..", "..", ".."))
	caseRoot := filepath.Join(t.TempDir(), "case")
	writeReviewerIntakeCase(t, repoRoot, caseRoot)
	plan, err := WritePlan(repoRoot, caseRoot, defaults.DefaultPack, Options{TaskType: "feature-analysis", Items: "alpha", Lane: "feature-intake"})
	if err != nil {
		t.Fatal(err)
	}
	packet := readReviewerPacket(t, plan.PacketPath)
	if err := os.MkdirAll(filepath.Join(caseRoot, "workspace"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(caseRoot, "workspace", "review-evidence.md"), []byte("bounded reviewer evidence\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	handoff := packet.ShardHandoffs[0]
	recordDispatch := func(harness, session string) ReviewerSessionReceiptResult {
		t.Helper()
		opt := ReviewerSessionDispatchOptions{PacketPath: plan.PacketPath, ShardID: handoff.ShardID, Lane: packet.TargetLane, Actor: "mission-commander", ReviewerHarness: harness, ReviewerSession: session, WhatIf: true}
		preview, err := RecordReviewerSessionDispatch(repoRoot, caseRoot, defaults.DefaultPack, opt)
		if err != nil {
			t.Fatal(err)
		}
		opt.WhatIf = false
		opt.ExpectedBindingSHA256 = preview.BindingSHA256
		result, err := RecordReviewerSessionDispatch(repoRoot, caseRoot, defaults.DefaultPack, opt)
		if err != nil {
			t.Fatal(err)
		}
		return result
	}
	dispatchA := recordDispatch("claude-code-agent-a", "reviewer-session-1")
	inputPath := handoff.ReviewerStagingCommands.SourceCaptureInput
	if err := os.MkdirAll(filepath.Dir(inputPath), 0o755); err != nil {
		t.Fatal(err)
	}
	input := reviewerResultForPacket(t, packet, "accept", "accepted", nil)
	if err := os.WriteFile(inputPath, input, 0o644); err != nil {
		t.Fatal(err)
	}
	completionOpt := ReviewerSessionCompletionOptions{PacketPath: plan.PacketPath, DispatchID: dispatchA.DispatchID, Lane: packet.TargetLane, Actor: "mission-commander", Outcome: "succeeded", ExitStatus: "completed", ReviewerResultInputPath: inputPath, WhatIf: true}
	completionPreview, err := RecordReviewerSessionCompletion(repoRoot, caseRoot, defaults.DefaultPack, completionOpt)
	if err != nil {
		t.Fatal(err)
	}
	completionOpt.WhatIf = false
	completionOpt.ExpectedDispatchReceiptSHA256 = completionPreview.DispatchReceiptSHA256
	completionOpt.ExpectedReviewerResultSHA256 = completionPreview.ReviewerResultInputSHA256
	if _, err := RecordReviewerSessionCompletion(repoRoot, caseRoot, defaults.DefaultPack, completionOpt); err != nil {
		t.Fatal(err)
	}
	dispatchB := recordDispatch("claude-code-agent-b", "reviewer-session-2")
	dispatchC := recordDispatch("claude-code-agent-c", "reviewer-session-1")
	handoffs, err := workstream.ReviewerDispatchIntakeHandoffs(caseRoot, mission.LedgerFacts{}, packet.TargetLane)
	if err != nil {
		t.Fatal(err)
	}
	if len(handoffs) != 1 || handoffs[0].State != "ready-for-reviewer-result-source-capture-preview" || handoffs[0].ReviewerDispatchID != dispatchA.DispatchID || handoffs[0].ReviewerDispatchID == dispatchB.DispatchID || handoffs[0].ReviewerDispatchID == dispatchC.DispatchID || handoffs[0].ReviewerSession != "reviewer-session-1" {
		t.Fatalf("lifecycle did not select exact input session attempt: %+v", handoffs)
	}
}

func TestReviewerSessionReceiptRejectsTakeoverOfPriorAttempt(t *testing.T) {
	repoRoot := filepath.Clean(filepath.Join("..", "..", ".."))
	caseRoot := filepath.Join(t.TempDir(), "case")
	writeReviewerIntakeCase(t, repoRoot, caseRoot)
	plan, err := WritePlan(repoRoot, caseRoot, defaults.DefaultPack, Options{TaskType: "feature-analysis", Items: "alpha", Lane: "feature-intake"})
	if err != nil {
		t.Fatal(err)
	}
	packet := readReviewerPacket(t, plan.PacketPath)
	if err := os.MkdirAll(filepath.Join(caseRoot, "workspace"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(caseRoot, "workspace", "review-evidence.md"), []byte("bounded reviewer evidence\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	handoff := packet.ShardHandoffs[0]
	dispatchOpt := ReviewerSessionDispatchOptions{PacketPath: plan.PacketPath, ShardID: handoff.ShardID, Lane: packet.TargetLane, Actor: "mission-commander", ReviewerHarness: "claude-code-agent", ReviewerSession: "reviewer-session-before-takeover", WhatIf: true}
	preview, err := RecordReviewerSessionDispatch(repoRoot, caseRoot, defaults.DefaultPack, dispatchOpt)
	if err != nil {
		t.Fatal(err)
	}
	dispatchOpt.WhatIf = false
	dispatchOpt.ExpectedBindingSHA256 = preview.BindingSHA256
	dispatch, err := RecordReviewerSessionDispatch(repoRoot, caseRoot, defaults.DefaultPack, dispatchOpt)
	if err != nil {
		t.Fatal(err)
	}
	inputPath := handoff.ReviewerStagingCommands.SourceCaptureInput
	if err := os.MkdirAll(filepath.Dir(inputPath), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(inputPath, reviewerResultForPacket(t, packet, "accept", "accepted", nil), 0o644); err != nil {
		t.Fatal(err)
	}
	root, err := filepath.Abs(repoRoot)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := workstream.StartApply(root, caseRoot, defaults.DefaultPack, workstream.StartOptions{Name: "intake", Executor: "session-replacement", Actor: "mission-commander", TakeoverReason: "replace reviewer receipt owner"}); err != nil {
		t.Fatal(err)
	}
	if _, err := AdoptReviewerPacket(repoRoot, caseRoot, defaults.DefaultPack, ReviewerPacketAdoptionOptions{PacketPath: plan.PacketPath, Lane: packet.TargetLane, Actor: "mission-commander", Reason: "adopt packet after reviewer dispatch"}); err != nil {
		t.Fatal(err)
	}
	handoffs, err := workstream.ReviewerDispatchIntakeHandoffs(caseRoot, mission.LedgerFacts{}, packet.TargetLane)
	if err != nil {
		t.Fatal(err)
	}
	if len(handoffs) != 1 || handoffs[0].State != "reviewer-session-receipt-owner-stale" || handoffs[0].ReviewerDispatchID != dispatch.DispatchID {
		t.Fatalf("unexpected stale owner receipt handoff: %+v", handoffs)
	}
	completionOpt := ReviewerSessionCompletionOptions{PacketPath: plan.PacketPath, DispatchID: dispatch.DispatchID, Lane: packet.TargetLane, Actor: "mission-commander", Outcome: "succeeded", ExitStatus: "completed", ReviewerResultInputPath: inputPath, WhatIf: true}
	if _, err := RecordReviewerSessionCompletion(repoRoot, caseRoot, defaults.DefaultPack, completionOpt); err == nil || !strings.Contains(err.Error(), "stale lane owner generation") {
		t.Fatalf("completion after takeover error = %v", err)
	}
	captureOpt := ReviewerResultSourceCaptureOptions{PacketPath: plan.PacketPath, ShardID: handoff.ShardID, InputPath: inputPath, Lane: packet.TargetLane, Actor: "mission-commander", WhatIf: true}
	if _, err := CaptureReviewerResultSource(repoRoot, caseRoot, defaults.DefaultPack, captureOpt); err == nil || !strings.Contains(err.Error(), "not bound") {
		t.Fatalf("source capture after takeover error = %v", err)
	}
}

func TestReviewerSessionCompletionRejectsSessionMismatchAndFailedCapture(t *testing.T) {
	repoRoot := filepath.Clean(filepath.Join("..", "..", ".."))
	caseRoot := filepath.Join(t.TempDir(), "case")
	writeReviewerIntakeCase(t, repoRoot, caseRoot)
	plan, err := WritePlan(repoRoot, caseRoot, defaults.DefaultPack, Options{TaskType: "feature-analysis", Items: "alpha", Lane: "feature-intake"})
	if err != nil {
		t.Fatal(err)
	}
	packet := readReviewerPacket(t, plan.PacketPath)
	if err := os.MkdirAll(filepath.Join(caseRoot, "workspace"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(caseRoot, "workspace", "review-evidence.md"), []byte("bounded reviewer evidence\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	handoff := packet.ShardHandoffs[0]
	dispatchOpt := ReviewerSessionDispatchOptions{PacketPath: plan.PacketPath, ShardID: handoff.ShardID, Lane: packet.TargetLane, Actor: "mission-commander", ReviewerHarness: "claude-code-agent", ReviewerSession: "bound-session", WhatIf: true}
	preview, err := RecordReviewerSessionDispatch(repoRoot, caseRoot, defaults.DefaultPack, dispatchOpt)
	if err != nil {
		t.Fatal(err)
	}
	dispatchOpt.WhatIf = false
	dispatchOpt.ExpectedBindingSHA256 = preview.BindingSHA256
	dispatch, err := RecordReviewerSessionDispatch(repoRoot, caseRoot, defaults.DefaultPack, dispatchOpt)
	if err != nil {
		t.Fatal(err)
	}
	inputPath := handoff.ReviewerStagingCommands.SourceCaptureInput
	if err := os.MkdirAll(filepath.Dir(inputPath), 0o755); err != nil {
		t.Fatal(err)
	}
	input := reviewerResultForPacket(t, packet, "accept", "accepted", nil)
	if err := os.WriteFile(inputPath, input, 0o644); err != nil {
		t.Fatal(err)
	}
	completionOpt := ReviewerSessionCompletionOptions{PacketPath: plan.PacketPath, DispatchID: dispatch.DispatchID, Lane: packet.TargetLane, Actor: "mission-commander", Outcome: "succeeded", ExitStatus: "completed", ReviewerResultInputPath: inputPath, WhatIf: true}
	if _, err := RecordReviewerSessionCompletion(repoRoot, caseRoot, defaults.DefaultPack, completionOpt); err == nil || !strings.Contains(err.Error(), "session bindings") {
		t.Fatalf("session mismatch error = %v", err)
	}

	failedOpt := ReviewerSessionCompletionOptions{PacketPath: plan.PacketPath, DispatchID: dispatch.DispatchID, Lane: packet.TargetLane, Actor: "mission-commander", Outcome: "failed", ExitStatus: "agent-error", WhatIf: true}
	failedPreview, err := RecordReviewerSessionCompletion(repoRoot, caseRoot, defaults.DefaultPack, failedOpt)
	if err != nil {
		t.Fatal(err)
	}
	failedOpt.WhatIf = false
	failedOpt.ExpectedDispatchReceiptSHA256 = failedPreview.DispatchReceiptSHA256
	if _, err := RecordReviewerSessionCompletion(repoRoot, caseRoot, defaults.DefaultPack, failedOpt); err != nil {
		t.Fatal(err)
	}
	captureOpt := ReviewerResultSourceCaptureOptions{PacketPath: plan.PacketPath, ShardID: handoff.ShardID, InputPath: inputPath, Lane: packet.TargetLane, Actor: "mission-commander", WhatIf: true}
	if _, err := CaptureReviewerResultSource(repoRoot, caseRoot, defaults.DefaultPack, captureOpt); err == nil || (!strings.Contains(err.Error(), "not bound") && !strings.Contains(err.Error(), "does not match")) {
		t.Fatalf("failed completion capture error = %v", err)
	}
}
