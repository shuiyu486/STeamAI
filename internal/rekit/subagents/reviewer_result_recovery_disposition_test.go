package subagents

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/shuiyu486/re-context-kits/internal/rekit/defaults"
)

func TestRetireAmbiguousReviewerResultRecoveryRetainsCanonical(t *testing.T) {
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
	conflict := []byte(`{"conflict":true}`)
	if err := os.WriteFile(handoff.ReviewerResultPath, conflict, 0o644); err != nil {
		t.Fatal(err)
	}
	recoveryOpt := ReviewerResultRecoveryOptions{PacketPath: plan.PacketPath, ShardID: handoff.ShardID, Lane: packet.TargetLane, Actor: "mission-commander", Reason: "recover conflict", WhatIf: true}
	preview, err := RecoverReviewerResult(repoRoot, caseRoot, defaults.DefaultPack, recoveryOpt)
	if err != nil {
		t.Fatal(err)
	}
	prepared, err := prepareReviewerResultCollectionMode(repoRoot, caseRoot, defaults.DefaultPack, ReviewerResultCollectionOptions{PacketPath: plan.PacketPath, ShardID: handoff.ShardID, Lane: packet.TargetLane, Actor: "mission-commander", WhatIf: true}, true)
	if err != nil {
		t.Fatal(err)
	}
	paths := reviewerResultRecoveryPaths(prepared)
	result := newReviewerResultRecoveryResult(repoRoot, caseRoot, defaults.DefaultPack, recoveryOpt, prepared, paths)
	if err := ensureReviewerResultRecoveryRoot(caseRoot, paths); err != nil {
		t.Fatal(err)
	}
	if err := writeReviewerResultRecoveryReceipt(caseRoot, paths.intentPath, reviewerResultRecoveryReceipt(result)); err != nil {
		t.Fatal(err)
	}
	if err := quarantineReviewerResult(caseRoot, handoff.ReviewerResultPath, paths.quarantinePath, paths.intentPath, conflict); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(handoff.ReviewerResultPath, candidate, 0o644); err != nil {
		t.Fatal(err)
	}
	dispositionOpt := ReviewerResultRecoveryDispositionOptions{PacketPath: plan.PacketPath, ShardID: handoff.ShardID, Lane: packet.TargetLane, Actor: "mission-commander", Reason: "retain reviewed canonical result", WhatIf: true}
	dispositionPreview, err := RetireAmbiguousReviewerResultRecovery(repoRoot, caseRoot, defaults.DefaultPack, dispositionOpt)
	if err != nil {
		t.Fatal(err)
	}
	if dispositionPreview.IntentSHA256 == "" || dispositionPreview.CanonicalSHA256 != preview.CandidateSHA256 || dispositionPreview.MissionCommanderAction.State != "needs-reviewer-result-recovery-disposition-apply" {
		t.Fatalf("unexpected disposition preview: %+v", dispositionPreview)
	}
	if _, err := os.Stat(dispositionPreview.DispositionPath); !os.IsNotExist(err) {
		t.Fatalf("preview wrote disposition: %v", err)
	}
	dispositionOpt.WhatIf = false
	dispositionOpt.ExpectedIntentSHA256 = dispositionPreview.IntentSHA256
	dispositionOpt.ExpectedCanonicalSHA256 = dispositionPreview.CanonicalSHA256
	applied, err := RetireAmbiguousReviewerResultRecovery(repoRoot, caseRoot, defaults.DefaultPack, dispositionOpt)
	if err != nil {
		t.Fatal(err)
	}
	if !applied.Applied {
		t.Fatalf("disposition not applied: %+v", applied)
	}
	if data, err := os.ReadFile(handoff.ReviewerResultPath); err != nil || string(data) != string(candidate) {
		t.Fatalf("canonical result changed: %q err=%v", data, err)
	}
	if !reviewerResultRecoveryQuarantineMatches(caseRoot, reviewerResultRecoveryReceipt(result)) {
		t.Fatal("quarantine changed during disposition")
	}
	writeReviewerSessionReceiptsForResult(t, handoff, candidate)
	collectionOpt := ReviewerResultCollectionOptions{PacketPath: plan.PacketPath, ShardID: handoff.ShardID, Lane: packet.TargetLane, Actor: "mission-commander", WhatIf: true}
	if _, err := CollectReviewerResult(repoRoot, caseRoot, defaults.DefaultPack, collectionOpt); err != nil {
		t.Fatal(err)
	}
	intakeOpt := ReviewerIntakeOptions{PacketPath: plan.PacketPath, ReviewerResultPath: handoff.ReviewerResultPath, Lane: packet.TargetLane, Actor: "mission-commander", ExpectedShardID: handoff.ShardID, WhatIf: true}
	if intake, err := IntakeReviewerResult(repoRoot, caseRoot, defaults.DefaultPack, intakeOpt); err != nil || !intake.ReadyForWriteback {
		t.Fatalf("intake after disposition: %+v err=%v", intake, err)
	}
	if err := os.WriteFile(handoff.ReviewerResultPath, []byte(`{"drift":true}`), 0o644); err != nil {
		t.Fatal(err)
	}
	if _, err := CollectReviewerResult(repoRoot, caseRoot, defaults.DefaultPack, collectionOpt); err == nil || !strings.Contains(err.Error(), "disposition") {
		t.Fatalf("drifted disposition collection error = %v", err)
	}
}
