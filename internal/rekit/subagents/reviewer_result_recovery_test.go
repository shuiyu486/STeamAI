package subagents

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/shuiyu486/re-context-kits/internal/rekit/defaults"
)

func TestRecoverReviewerResultWhatIfApplyCollectionAndIntake(t *testing.T) {
	requireReviewerResultExactMove(t, "regular-file")
	repoRoot := filepath.Clean(filepath.Join("..", "..", ".."))
	caseRoot := filepath.Join(t.TempDir(), "case")
	writeReviewerIntakeCase(t, repoRoot, caseRoot)
	plan, err := WritePlan(repoRoot, caseRoot, defaults.DefaultPack, Options{TaskType: "feature-analysis", Items: "alpha", Lane: reviewerIntakeLane})
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
	candidate := reviewerResultForPacket(t, packet, "accept", "accepted", nil)
	if err := os.MkdirAll(filepath.Dir(handoff.ReviewerResultCandidatePath), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(handoff.ReviewerResultCandidatePath, candidate, 0o644); err != nil {
		t.Fatal(err)
	}
	corrupt := []byte(`{"different":true}`)
	if err := os.WriteFile(handoff.ReviewerResultPath, corrupt, 0o644); err != nil {
		t.Fatal(err)
	}

	opt := ReviewerResultRecoveryOptions{PacketPath: plan.PacketPath, ShardID: handoff.ShardID, Lane: packet.TargetLane, Actor: "mission-commander", Reason: "quarantine corrupted canonical reviewer result", WhatIf: true}
	preview, err := RecoverReviewerResult(repoRoot, caseRoot, defaults.DefaultPack, opt)
	if err != nil {
		t.Fatal(err)
	}
	if preview.Applied || preview.CandidateSHA256 == "" || preview.ReviewerResultSHA256 == "" || !strings.Contains(preview.MissionCommanderAction.PrimaryCommand, "-ExpectedCandidateSha256") {
		t.Fatalf("unexpected recovery preview: %+v", preview)
	}
	assertReviewerRunbookContains(t, preview.RunbookSteps, "-RecoverReviewerResult")
	assertReviewerRunbookContains(t, preview.RunbookSteps, "-ExpectedCandidateSha256")
	assertReviewerRunbookContains(t, preview.RunbookSteps, "-ExpectedReviewerResultSha256")
	assertReviewerRunbookContains(t, preview.RunbookSteps, "hash-bound")
	if _, err := os.Stat(preview.QuarantinePath); !os.IsNotExist(err) {
		t.Fatalf("recovery WhatIf wrote quarantine: %v", err)
	}

	opt.WhatIf = false
	opt.ExpectedCandidateSHA256 = preview.CandidateSHA256
	opt.ExpectedReviewerResultSHA256 = preview.ReviewerResultSHA256
	applied, err := RecoverReviewerResult(repoRoot, caseRoot, defaults.DefaultPack, opt)
	if err != nil {
		t.Fatal(err)
	}
	if !applied.Applied || applied.MissionCommanderAction.State != "reviewer-result-recovered-ready-for-collection-preview" {
		t.Fatalf("unexpected recovery apply: %+v", applied)
	}
	assertReviewerRunbookContains(t, applied.RunbookSteps, "-CollectReviewerResult")
	assertReviewerRunbookContains(t, applied.RunbookSteps, "separate bounded operations")
	quarantined, err := os.ReadFile(applied.QuarantinePath)
	if err != nil || string(quarantined) != string(corrupt) {
		t.Fatalf("quarantine bytes = %q err=%v", quarantined, err)
	}
	if _, err := os.Stat(handoff.ReviewerResultPath); !os.IsNotExist(err) {
		t.Fatalf("canonical result still exists: %v", err)
	}
	replayed, err := RecoverReviewerResult(repoRoot, caseRoot, defaults.DefaultPack, opt)
	if err != nil || !replayed.AlreadyRecovered || !replayed.Applied {
		t.Fatalf("recovery replay: %+v err=%v", replayed, err)
	}
	assertReviewerRunbookContains(t, replayed.RunbookSteps, "-CollectReviewerResult")

	writeReviewerSessionReceiptsForResult(t, handoff, candidate)
	collectionOpt := ReviewerResultCollectionOptions{PacketPath: plan.PacketPath, ShardID: handoff.ShardID, Lane: packet.TargetLane, Actor: "mission-commander", WhatIf: true}
	collectionPreview, err := CollectReviewerResult(repoRoot, caseRoot, defaults.DefaultPack, collectionOpt)
	if err != nil || collectionPreview.Status != "previewed" {
		t.Fatalf("collection preview after recovery: %+v err=%v", collectionPreview, err)
	}
	collectionOpt.WhatIf = false
	collectionOpt.ExpectedCandidateSHA256 = collectionPreview.CandidateSHA256
	if _, err := CollectReviewerResult(repoRoot, caseRoot, defaults.DefaultPack, collectionOpt); err != nil {
		t.Fatal(err)
	}
	intakeOpt := ReviewerIntakeOptions{PacketPath: plan.PacketPath, ReviewerResultPath: handoff.ReviewerResultPath, Lane: packet.TargetLane, Actor: "mission-commander", ExpectedShardID: handoff.ShardID, WhatIf: true}
	intakePreview, err := IntakeReviewerResult(repoRoot, caseRoot, defaults.DefaultPack, intakeOpt)
	if err != nil || !intakePreview.ReadyForWriteback {
		t.Fatalf("intake preview after recovery: %+v err=%v", intakePreview, err)
	}
}

func requireReviewerResultExactMove(t *testing.T, kind string) {
	t.Helper()
	if !reviewerResultExactMoveSupported(kind) {
		t.Skipf("exact %s reviewer result move is unavailable on this platform", kind)
	}
}

func TestRecoverReviewerResultResumesInterruptedIntent(t *testing.T) {
	requireReviewerResultExactMove(t, "regular-file")
	repoRoot := filepath.Clean(filepath.Join("..", "..", ".."))
	caseRoot := filepath.Join(t.TempDir(), "case")
	writeReviewerIntakeCase(t, repoRoot, caseRoot)
	plan, err := WritePlan(repoRoot, caseRoot, defaults.DefaultPack, Options{TaskType: "feature-analysis", Items: "alpha", Lane: reviewerIntakeLane})
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
	candidate := reviewerResultForPacket(t, packet, "accept", "accepted", nil)
	if err := os.MkdirAll(filepath.Dir(handoff.ReviewerResultCandidatePath), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(handoff.ReviewerResultCandidatePath, candidate, 0o644); err != nil {
		t.Fatal(err)
	}
	corrupt := []byte(`{"interrupted":true}`)
	if err := os.WriteFile(handoff.ReviewerResultPath, corrupt, 0o644); err != nil {
		t.Fatal(err)
	}
	opt := ReviewerResultRecoveryOptions{PacketPath: plan.PacketPath, ShardID: handoff.ShardID, Lane: packet.TargetLane, Actor: "mission-commander", Reason: "resume interrupted recovery", WhatIf: true}
	preview, err := RecoverReviewerResult(repoRoot, caseRoot, defaults.DefaultPack, opt)
	if err != nil {
		t.Fatal(err)
	}
	prepared, err := prepareReviewerResultCollectionMode(repoRoot, caseRoot, defaults.DefaultPack, ReviewerResultCollectionOptions{PacketPath: plan.PacketPath, ShardID: handoff.ShardID, Lane: packet.TargetLane, Actor: "mission-commander", WhatIf: true}, true)
	if err != nil {
		t.Fatal(err)
	}
	paths := reviewerResultRecoveryPaths(prepared)
	result := newReviewerResultRecoveryResult(repoRoot, caseRoot, defaults.DefaultPack, opt, prepared, paths)
	if err := ensureReviewerResultRecoveryRoot(caseRoot, paths); err != nil {
		t.Fatal(err)
	}
	if err := writeReviewerResultRecoveryReceipt(caseRoot, paths.intentPath, reviewerResultRecoveryReceipt(result)); err != nil {
		t.Fatal(err)
	}
	if err := quarantineReviewerResult(caseRoot, handoff.ReviewerResultPath, paths.quarantinePath, paths.intentPath, corrupt); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(paths.receiptPath); !os.IsNotExist(err) {
		t.Fatalf("unexpected committed receipt before resume: %v", err)
	}
	collectionOpt := ReviewerResultCollectionOptions{PacketPath: plan.PacketPath, ShardID: handoff.ShardID, Lane: packet.TargetLane, Actor: "mission-commander", WhatIf: true}
	if _, err := CollectReviewerResult(repoRoot, caseRoot, defaults.DefaultPack, collectionOpt); err == nil || !strings.Contains(err.Error(), "must be finalized before collection") {
		t.Fatalf("collection during interrupted recovery error = %v", err)
	}

	resumePreview, err := RecoverReviewerResult(repoRoot, caseRoot, defaults.DefaultPack, opt)
	if err != nil {
		t.Fatal(err)
	}
	if resumePreview.MissionCommanderAction.State != "needs-reviewer-result-recovery-finalize-apply" {
		t.Fatalf("unexpected interrupted recovery preview: %+v", resumePreview)
	}
	assertReviewerRunbookContains(t, resumePreview.RunbookSteps, "interrupted")
	assertReviewerRunbookContains(t, resumePreview.RunbookSteps, "-ExpectedCandidateSha256")
	assertReviewerRunbookContains(t, resumePreview.RunbookSteps, "before collection or intake")
	opt.WhatIf = false
	opt.ExpectedCandidateSHA256 = preview.CandidateSHA256
	opt.ExpectedReviewerResultSHA256 = preview.ReviewerResultSHA256
	resumed, err := RecoverReviewerResult(repoRoot, caseRoot, defaults.DefaultPack, opt)
	if err != nil {
		t.Fatal(err)
	}
	if !resumed.Applied || resumed.AlreadyRecovered {
		t.Fatalf("unexpected resumed recovery: %+v", resumed)
	}
	assertReviewerRunbookContains(t, resumed.RunbookSteps, "-CollectReviewerResult")
	if _, err := readReviewerResultRecoveryReceipt(caseRoot, paths.receiptPath); err != nil {
		t.Fatal(err)
	}
}

func TestRecoverReviewerResultReusesIntentCreatedBeforeQuarantine(t *testing.T) {
	requireReviewerResultExactMove(t, "empty-file")
	repoRoot := filepath.Clean(filepath.Join("..", "..", ".."))
	caseRoot := filepath.Join(t.TempDir(), "case")
	writeReviewerIntakeCase(t, repoRoot, caseRoot)
	plan, err := WritePlan(repoRoot, caseRoot, defaults.DefaultPack, Options{TaskType: "feature-analysis", Items: "alpha", Lane: reviewerIntakeLane})
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
	candidate := reviewerResultForPacket(t, packet, "accept", "accepted", nil)
	if err := os.MkdirAll(filepath.Dir(handoff.ReviewerResultCandidatePath), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(handoff.ReviewerResultCandidatePath, candidate, 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(handoff.ReviewerResultPath, nil, 0o644); err != nil {
		t.Fatal(err)
	}
	opt := ReviewerResultRecoveryOptions{PacketPath: plan.PacketPath, ShardID: handoff.ShardID, Lane: packet.TargetLane, Actor: "mission-commander", Reason: "resume pre-quarantine intent", WhatIf: true}
	preview, err := RecoverReviewerResult(repoRoot, caseRoot, defaults.DefaultPack, opt)
	if err != nil {
		t.Fatal(err)
	}
	prepared, err := prepareReviewerResultCollectionMode(repoRoot, caseRoot, defaults.DefaultPack, ReviewerResultCollectionOptions{PacketPath: plan.PacketPath, ShardID: handoff.ShardID, Lane: packet.TargetLane, Actor: opt.Actor, WhatIf: true}, true)
	if err != nil {
		t.Fatal(err)
	}
	paths := reviewerResultRecoveryPaths(prepared)
	result := newReviewerResultRecoveryResult(repoRoot, caseRoot, defaults.DefaultPack, opt, prepared, paths)
	if err := ensureReviewerResultRecoveryRoot(caseRoot, paths); err != nil {
		t.Fatal(err)
	}
	intent := reviewerResultRecoveryReceipt(result)
	if err := writeReviewerResultRecoveryReceipt(caseRoot, paths.intentPath, intent); err != nil {
		t.Fatal(err)
	}

	opt.WhatIf = false
	opt.ExpectedCandidateSHA256 = preview.CandidateSHA256
	opt.ExpectedReviewerResultSHA256 = preview.ReviewerResultSHA256
	if _, err := RecoverReviewerResult(repoRoot, caseRoot, defaults.DefaultPack, opt); err != nil {
		t.Fatal(err)
	}
	committed, err := readReviewerResultRecoveryReceipt(caseRoot, paths.receiptPath)
	if err != nil {
		t.Fatal(err)
	}
	if committed.CreatedAt != intent.CreatedAt {
		t.Fatalf("committed CreatedAt = %q, want durable intent %q", committed.CreatedAt, intent.CreatedAt)
	}
	collectionOpt := ReviewerResultCollectionOptions{PacketPath: plan.PacketPath, ShardID: handoff.ShardID, Lane: packet.TargetLane, Actor: opt.Actor, WhatIf: true}
	if _, err := CollectReviewerResult(repoRoot, caseRoot, defaults.DefaultPack, collectionOpt); err != nil {
		t.Fatal(err)
	}
}

func TestRecoverReviewerResultInterruptedIntentBlocksDirectIntake(t *testing.T) {
	requireReviewerResultExactMove(t, "regular-file")
	repoRoot := filepath.Clean(filepath.Join("..", "..", ".."))
	caseRoot := filepath.Join(t.TempDir(), "case")
	writeReviewerIntakeCase(t, repoRoot, caseRoot)
	plan, err := WritePlan(repoRoot, caseRoot, defaults.DefaultPack, Options{TaskType: "feature-analysis", Items: "alpha", Lane: reviewerIntakeLane})
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
	candidate := reviewerResultForPacket(t, packet, "accept", "accepted", nil)
	if err := os.MkdirAll(filepath.Dir(handoff.ReviewerResultCandidatePath), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(handoff.ReviewerResultCandidatePath, candidate, 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(handoff.ReviewerResultPath, []byte(`{"interrupted":true}`), 0o644); err != nil {
		t.Fatal(err)
	}
	opt := ReviewerResultRecoveryOptions{PacketPath: plan.PacketPath, ShardID: handoff.ShardID, Lane: packet.TargetLane, Actor: "mission-commander", Reason: "interrupted direct intake guard", WhatIf: true}
	prepared, err := prepareReviewerResultCollectionMode(repoRoot, caseRoot, defaults.DefaultPack, ReviewerResultCollectionOptions{PacketPath: plan.PacketPath, ShardID: handoff.ShardID, Lane: packet.TargetLane, Actor: opt.Actor, WhatIf: true}, true)
	if err != nil {
		t.Fatal(err)
	}
	paths := reviewerResultRecoveryPaths(prepared)
	result := newReviewerResultRecoveryResult(repoRoot, caseRoot, defaults.DefaultPack, opt, prepared, paths)
	if err := ensureReviewerResultRecoveryRoot(caseRoot, paths); err != nil {
		t.Fatal(err)
	}
	intent := reviewerResultRecoveryReceipt(result)
	if err := writeReviewerResultRecoveryReceipt(caseRoot, paths.intentPath, intent); err != nil {
		t.Fatal(err)
	}
	if err := quarantineReviewerResult(caseRoot, handoff.ReviewerResultPath, paths.quarantinePath, paths.intentPath, prepared.canonical); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(handoff.ReviewerResultPath, candidate, 0o644); err != nil {
		t.Fatal(err)
	}
	intakeOpt := ReviewerIntakeOptions{PacketPath: plan.PacketPath, ReviewerResultPath: handoff.ReviewerResultPath, Lane: packet.TargetLane, Actor: opt.Actor, ExpectedShardID: handoff.ShardID, WhatIf: true}
	if _, err := IntakeReviewerResult(repoRoot, caseRoot, defaults.DefaultPack, intakeOpt); err == nil || !strings.Contains(err.Error(), "recovery is incomplete") {
		t.Fatalf("direct intake during interrupted recovery error = %v", err)
	}
	batchOpt := ReviewerBatchIntakeOptions{PacketPath: plan.PacketPath, Lane: packet.TargetLane, Actor: opt.Actor, WhatIf: true}
	if _, err := IntakeReadyReviewerResults(repoRoot, caseRoot, defaults.DefaultPack, batchOpt); err == nil || !strings.Contains(err.Error(), "recovery is incomplete") {
		t.Fatalf("batch intake during interrupted recovery error = %v", err)
	}
}

func TestRecoverReviewerResultInterruptedIntentRejectsActorReasonDrift(t *testing.T) {
	requireReviewerResultExactMove(t, "regular-file")
	repoRoot := filepath.Clean(filepath.Join("..", "..", ".."))
	caseRoot := filepath.Join(t.TempDir(), "case")
	writeReviewerIntakeCase(t, repoRoot, caseRoot)
	plan, err := WritePlan(repoRoot, caseRoot, defaults.DefaultPack, Options{TaskType: "feature-analysis", Items: "alpha", Lane: reviewerIntakeLane})
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
	candidate := reviewerResultForPacket(t, packet, "accept", "accepted", nil)
	if err := os.MkdirAll(filepath.Dir(handoff.ReviewerResultCandidatePath), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(handoff.ReviewerResultCandidatePath, candidate, 0o644); err != nil {
		t.Fatal(err)
	}
	corrupt := []byte(`{"interrupted":true}`)
	if err := os.WriteFile(handoff.ReviewerResultPath, corrupt, 0o644); err != nil {
		t.Fatal(err)
	}
	opt := ReviewerResultRecoveryOptions{PacketPath: plan.PacketPath, ShardID: handoff.ShardID, Lane: packet.TargetLane, Actor: "mission-commander", Reason: "original reason", WhatIf: true}
	prepared, err := prepareReviewerResultCollectionMode(repoRoot, caseRoot, defaults.DefaultPack, ReviewerResultCollectionOptions{PacketPath: plan.PacketPath, ShardID: handoff.ShardID, Lane: packet.TargetLane, Actor: "mission-commander", WhatIf: true}, true)
	if err != nil {
		t.Fatal(err)
	}
	paths := reviewerResultRecoveryPaths(prepared)
	result := newReviewerResultRecoveryResult(repoRoot, caseRoot, defaults.DefaultPack, opt, prepared, paths)
	if err := ensureReviewerResultRecoveryRoot(caseRoot, paths); err != nil {
		t.Fatal(err)
	}
	if err := writeReviewerResultRecoveryReceipt(caseRoot, paths.intentPath, reviewerResultRecoveryReceipt(result)); err != nil {
		t.Fatal(err)
	}
	if err := quarantineReviewerResult(caseRoot, handoff.ReviewerResultPath, paths.quarantinePath, paths.intentPath, corrupt); err != nil {
		t.Fatal(err)
	}
	opt.Actor = "replacement-commander"
	opt.Reason = "different reason"
	if _, err := RecoverReviewerResult(repoRoot, caseRoot, defaults.DefaultPack, opt); err == nil || !strings.Contains(err.Error(), "does not match") {
		t.Fatalf("actor/reason drift error = %v", err)
	}
}

func TestRecoverReviewerResultQuarantinesCanonicalObstructions(t *testing.T) {
	for _, kind := range []string{"empty-file", "symlink"} {
		t.Run(kind, func(t *testing.T) {
			requireReviewerResultExactMove(t, kind)
			repoRoot := filepath.Clean(filepath.Join("..", "..", ".."))
			caseRoot := filepath.Join(t.TempDir(), "case")
			writeReviewerIntakeCase(t, repoRoot, caseRoot)
			plan, err := WritePlan(repoRoot, caseRoot, defaults.DefaultPack, Options{TaskType: "feature-analysis", Items: "alpha", Lane: reviewerIntakeLane})
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
			candidate := reviewerResultForPacket(t, packet, "accept", "accepted", nil)
			if err := os.MkdirAll(filepath.Dir(handoff.ReviewerResultCandidatePath), 0o755); err != nil {
				t.Fatal(err)
			}
			if err := os.WriteFile(handoff.ReviewerResultCandidatePath, candidate, 0o644); err != nil {
				t.Fatal(err)
			}
			switch kind {
			case "empty-file":
				if err := os.WriteFile(handoff.ReviewerResultPath, nil, 0o644); err != nil {
					t.Fatal(err)
				}
			case "directory":
				if err := os.Mkdir(handoff.ReviewerResultPath, 0o755); err != nil {
					t.Fatal(err)
				}
			case "symlink":
				target := filepath.Join(caseRoot, "workspace", "outside-result.json")
				if err := os.WriteFile(target, []byte("outside must remain\n"), 0o644); err != nil {
					t.Fatal(err)
				}
				if err := os.Symlink(target, handoff.ReviewerResultPath); err != nil {
					t.Skipf("symlink unavailable: %v", err)
				}
			}
			opt := ReviewerResultRecoveryOptions{PacketPath: plan.PacketPath, ShardID: handoff.ShardID, Lane: packet.TargetLane, Actor: "mission-commander", Reason: "quarantine canonical obstruction", WhatIf: true}
			preview, err := RecoverReviewerResult(repoRoot, caseRoot, defaults.DefaultPack, opt)
			if err != nil {
				t.Fatal(err)
			}
			if preview.ReviewerResultKind != kind || preview.ReviewerResultSHA256 == "" {
				t.Fatalf("unexpected obstruction preview: %+v", preview)
			}
			opt.WhatIf = false
			opt.ExpectedCandidateSHA256 = preview.CandidateSHA256
			opt.ExpectedReviewerResultSHA256 = preview.ReviewerResultSHA256
			applied, err := RecoverReviewerResult(repoRoot, caseRoot, defaults.DefaultPack, opt)
			if err != nil {
				t.Fatal(err)
			}
			if !applied.Applied || applied.ReviewerResultKind != kind {
				t.Fatalf("unexpected obstruction recovery: %+v", applied)
			}
			if _, err := os.Lstat(handoff.ReviewerResultPath); !os.IsNotExist(err) {
				t.Fatalf("canonical obstruction remains: %v", err)
			}
			if !reviewerResultRecoveryQuarantineMatches(caseRoot, reviewerResultRecoveryReceipt(applied)) {
				t.Fatal("quarantined obstruction does not match receipt")
			}
			collectionOpt := ReviewerResultCollectionOptions{PacketPath: plan.PacketPath, ShardID: handoff.ShardID, Lane: packet.TargetLane, Actor: "mission-commander", WhatIf: true}
			if _, err := CollectReviewerResult(repoRoot, caseRoot, defaults.DefaultPack, collectionOpt); err != nil {
				t.Fatal(err)
			}
		})
	}
}

func TestRecoverReviewerResultRejectsCanonicalDirectories(t *testing.T) {
	for _, nonEmpty := range []bool{false, true} {
		t.Run(map[bool]string{false: "empty", true: "non-empty"}[nonEmpty], func(t *testing.T) {
			repoRoot := filepath.Clean(filepath.Join("..", "..", ".."))
			caseRoot := filepath.Join(t.TempDir(), "case")
			writeReviewerIntakeCase(t, repoRoot, caseRoot)
			plan, err := WritePlan(repoRoot, caseRoot, defaults.DefaultPack, Options{TaskType: "feature-analysis", Items: "alpha", Lane: reviewerIntakeLane})
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
			candidate := reviewerResultForPacket(t, packet, "accept", "accepted", nil)
			if err := os.MkdirAll(filepath.Dir(handoff.ReviewerResultCandidatePath), 0o755); err != nil {
				t.Fatal(err)
			}
			if err := os.WriteFile(handoff.ReviewerResultCandidatePath, candidate, 0o644); err != nil {
				t.Fatal(err)
			}
			if err := os.Mkdir(handoff.ReviewerResultPath, 0o755); err != nil {
				t.Fatal(err)
			}
			if nonEmpty {
				if err := os.WriteFile(filepath.Join(handoff.ReviewerResultPath, "foreign.txt"), []byte("do not remove\n"), 0o644); err != nil {
					t.Fatal(err)
				}
			}
			opt := ReviewerResultRecoveryOptions{PacketPath: plan.PacketPath, ShardID: handoff.ShardID, Lane: packet.TargetLane, Actor: "mission-commander", Reason: "recover obstruction", WhatIf: true}
			_, err = RecoverReviewerResult(repoRoot, caseRoot, defaults.DefaultPack, opt)
			want := "canonical reviewer result directory cannot be recovered automatically"
			if nonEmpty {
				want = "non-empty canonical reviewer result directory"
			}
			if err == nil || !strings.Contains(err.Error(), want) {
				t.Fatalf("directory recovery error = %v, want %q", err, want)
			}
			if _, statErr := os.Stat(handoff.ReviewerResultPath); statErr != nil {
				t.Fatalf("canonical directory was changed: %v", statErr)
			}
		})
	}
}
func TestRecoverReviewerResultRejectsDriftAndWriteback(t *testing.T) {
	requireReviewerResultExactMove(t, "regular-file")
	repoRoot := filepath.Clean(filepath.Join("..", "..", ".."))
	caseRoot := filepath.Join(t.TempDir(), "case")
	writeReviewerIntakeCase(t, repoRoot, caseRoot)
	plan, err := WritePlan(repoRoot, caseRoot, defaults.DefaultPack, Options{TaskType: "feature-analysis", Items: "alpha", Lane: reviewerIntakeLane})
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
	candidate := reviewerResultForPacket(t, packet, "accept", "accepted", nil)
	if err := os.MkdirAll(filepath.Dir(handoff.ReviewerResultCandidatePath), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(handoff.ReviewerResultCandidatePath, candidate, 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(handoff.ReviewerResultPath, []byte(`{"different":true}`), 0o644); err != nil {
		t.Fatal(err)
	}
	opt := ReviewerResultRecoveryOptions{PacketPath: plan.PacketPath, ShardID: handoff.ShardID, Lane: packet.TargetLane, Actor: "mission-commander", Reason: "recover", WhatIf: true}
	preview, err := RecoverReviewerResult(repoRoot, caseRoot, defaults.DefaultPack, opt)
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(handoff.ReviewerResultPath, []byte(`{"changed":true}`), 0o644); err != nil {
		t.Fatal(err)
	}
	opt.WhatIf = false
	opt.ExpectedCandidateSHA256 = preview.CandidateSHA256
	opt.ExpectedReviewerResultSHA256 = preview.ReviewerResultSHA256
	if _, err := RecoverReviewerResult(repoRoot, caseRoot, defaults.DefaultPack, opt); err == nil || !strings.Contains(err.Error(), "changed after recovery preview") {
		t.Fatalf("drift error = %v", err)
	}

	// A matching reviewer result can be intaken; once writeback exists, recovery must never undo it.
	if err := os.WriteFile(handoff.ReviewerResultPath, candidate, 0o644); err != nil {
		t.Fatal(err)
	}
	writeReviewerSessionReceiptsForResult(t, handoff, candidate)
	intakeOpt := ReviewerIntakeOptions{PacketPath: plan.PacketPath, ReviewerResultPath: handoff.ReviewerResultPath, Lane: packet.TargetLane, Actor: "mission-commander", ExpectedShardID: handoff.ShardID, WhatIf: false}
	if _, err := IntakeReviewerResult(repoRoot, caseRoot, defaults.DefaultPack, intakeOpt); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(handoff.ReviewerResultPath, []byte(`{"corrupt":true}`), 0o644); err != nil {
		t.Fatal(err)
	}
	opt.WhatIf = true
	opt.ExpectedCandidateSHA256 = ""
	opt.ExpectedReviewerResultSHA256 = ""
	if _, err := RecoverReviewerResult(repoRoot, caseRoot, defaults.DefaultPack, opt); err == nil || !strings.Contains(err.Error(), "forbidden after reviewer verification writeback") {
		t.Fatalf("post-writeback recovery error = %v", err)
	}
}

func TestReviewerResultRecoveryReceiptStrictJSON(t *testing.T) {
	path := filepath.Join(t.TempDir(), "receipt.json")
	if err := os.WriteFile(path, []byte(`{"schemaVersion":1}`+"\n"+`{}`), 0o600); err != nil {
		t.Fatal(err)
	}
	if _, err := readReviewerResultRecoveryReceipt("", path); err == nil || !strings.Contains(err.Error(), "exactly one JSON object") {
		t.Fatalf("trailing receipt error = %v", err)
	}
}
