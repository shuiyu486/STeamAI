package subagents

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/shuiyu486/re-context-kits/internal/rekit/defaults"
)

func reviewerPacketSnapshotForTest(t *testing.T, caseRoot, packetPath string) reviewerPacketAnchoredSnapshot {
	t.Helper()
	_, snapshot, err := readReviewerPacketAndIntegrityAnchored(caseRoot, packetPath)
	if err != nil {
		t.Fatal(err)
	}
	return snapshot
}

func TestStageReviewerResultPublishesCandidateForCollection(t *testing.T) {
	repoRoot := filepath.Clean(filepath.Join("..", "..", ".."))
	caseRoot := filepath.Join(t.TempDir(), "case")
	writeReviewerIntakeCase(t, repoRoot, caseRoot)
	plan, err := WritePlan(repoRoot, caseRoot, defaults.DefaultPack, Options{TaskType: "feature-analysis", Items: "alpha", Lane: "feature-intake"})
	if err != nil {
		t.Fatal(err)
	}
	packet := readReviewerPacket(t, plan.PacketPath)
	handoff := packet.ShardHandoffs[0]
	if err := os.MkdirAll(filepath.Join(caseRoot, "workspace"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(caseRoot, "workspace", "review-evidence.md"), []byte("bounded reviewer evidence\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if handoff.ReviewerStagingCommands == nil || handoff.ReviewerStagingCommands.SourcePath == "" {
		t.Fatalf("missing packet-derived reviewer staging source path: %+v", handoff)
	}
	sourcePath := handoff.ReviewerStagingCommands.SourcePath
	if err := os.MkdirAll(filepath.Dir(sourcePath), 0o755); err != nil {
		t.Fatal(err)
	}
	source := reviewerResultForPacket(t, packet, "accept", "accepted", nil)
	if err := os.WriteFile(sourcePath, source, 0o644); err != nil {
		t.Fatal(err)
	}
	opt := ReviewerResultStagingOptions{PacketPath: plan.PacketPath, ShardID: handoff.ShardID, SourcePath: sourcePath, Lane: packet.TargetLane, Actor: "mission-commander", WhatIf: true}
	preview, err := StageReviewerResult(repoRoot, caseRoot, defaults.DefaultPack, opt)
	if err != nil {
		t.Fatal(err)
	}
	if preview.Applied || preview.SourceSHA256 == "" || preview.CandidatePath != handoff.ReviewerResultCandidatePath || preview.MissionCommanderAction.State != "ready-for-reviewer-result-staging-apply" || !strings.Contains(preview.MissionCommanderAction.PrimaryCommand, "-ExpectedSourceSha256") {
		t.Fatalf("unexpected staging preview: %+v", preview)
	}
	assertReviewerRunbookContains(t, preview.RunbookSteps, "run current Mission Commander command")
	assertReviewerRunbookContains(t, preview.RunbookSteps, "hash-bound Apply command")
	if _, err := os.Stat(handoff.ReviewerResultCandidatePath); !os.IsNotExist(err) {
		t.Fatalf("preview wrote candidate: %v", err)
	}
	opt.WhatIf = false
	opt.ExpectedSourceSHA256 = preview.SourceSHA256
	applied, err := StageReviewerResult(repoRoot, caseRoot, defaults.DefaultPack, opt)
	if err != nil {
		t.Fatal(err)
	}
	if !applied.Applied || applied.Status != "staged" || applied.MissionCommanderAction.State != "reviewer-result-staged-ready-for-collection-preview" {
		t.Fatalf("unexpected staging apply: %+v", applied)
	}
	assertReviewerRunbookContains(t, applied.RunbookSteps, "-CollectReviewerResult")
	assertReviewerRunbookContains(t, applied.RunbookSteps, "separate bounded operations")
	if data, err := os.ReadFile(handoff.ReviewerResultCandidatePath); err != nil || string(data) != string(source) {
		t.Fatalf("staged candidate = %q err=%v", data, err)
	}
	if data, err := os.ReadFile(sourcePath); err != nil || string(data) != string(source) {
		t.Fatalf("staging source changed: %q err=%v", data, err)
	}
	replay, err := StageReviewerResult(repoRoot, caseRoot, defaults.DefaultPack, opt)
	if err != nil {
		t.Fatal(err)
	}
	if !replay.AlreadyStaged || replay.Status != "already-staged" {
		t.Fatalf("staging replay is not idempotent: %+v", replay)
	}
	collection, err := CollectReviewerResult(repoRoot, caseRoot, defaults.DefaultPack, ReviewerResultCollectionOptions{PacketPath: plan.PacketPath, ShardID: handoff.ShardID, Lane: packet.TargetLane, Actor: "mission-commander", WhatIf: true})
	if err != nil {
		t.Fatal(err)
	}
	if collection.CandidateSHA256 != preview.SourceSHA256 || collection.ReviewerResult.ReviewerSession != preview.ReviewerResult.ReviewerSession {
		t.Fatalf("collection did not consume staged candidate: %+v", collection)
	}
	assertReviewerRunbookContains(t, collection.RunbookSteps, "run current Mission Commander command")
}

func assertReviewerRunbookContains(t *testing.T, steps []string, want string) {
	t.Helper()
	for _, step := range steps {
		if strings.Contains(step, want) {
			return
		}
	}
	t.Fatalf("runbook missing %q in %v", want, steps)
}

func TestPublishReviewerResultCandidateRejectsPreparedNamespaceRebind(t *testing.T) {
	repoRoot := filepath.Clean(filepath.Join("..", "..", ".."))
	caseRoot := filepath.Join(t.TempDir(), "case")
	writeReviewerIntakeCase(t, repoRoot, caseRoot)
	plan, err := WritePlan(repoRoot, caseRoot, defaults.DefaultPack, Options{TaskType: "feature-analysis", Items: "alpha", Lane: "feature-intake"})
	if err != nil {
		t.Fatal(err)
	}
	packet := readReviewerPacket(t, plan.PacketPath)
	handoff := packet.ShardHandoffs[0]
	resultRootID, err := reviewerDirectoryIdentity(caseRoot, packet.ReviewerOrchestration.ResultRoot)
	if err != nil {
		t.Fatal(err)
	}
	detachedResultRoot := packet.ReviewerOrchestration.ResultRoot + ".detached"
	if err := os.Rename(packet.ReviewerOrchestration.ResultRoot, detachedResultRoot); err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(filepath.Join(packet.ReviewerOrchestration.ResultRoot, "candidates"), 0o755); err != nil {
		t.Fatal(err)
	}
	source := reviewerResultForPacket(t, packet, "accept", "accepted", nil)
	if _, err := publishReviewerResultCandidateAnchoredExpected(caseRoot, packet, handoff, source, reviewerPacketSnapshotForTest(t, caseRoot, plan.PacketPath), resultRootID, nil); err == nil || !strings.Contains(err.Error(), "namespace changed") {
		t.Fatalf("prepared namespace rebind error = %v", err)
	}
	if _, err := os.Stat(handoff.ReviewerResultCandidatePath); !os.IsNotExist(err) {
		t.Fatalf("prepared namespace rebind wrote candidate: %v", err)
	}
	if _, err := os.Stat(filepath.Join(detachedResultRoot, "candidates", handoff.ShardID+".json")); !os.IsNotExist(err) {
		t.Fatalf("prepared namespace rebind wrote detached candidate: %v", err)
	}
}

func TestPublishReviewerResultCandidateCleansCorruptedPublication(t *testing.T) {
	repoRoot := filepath.Clean(filepath.Join("..", "..", ".."))
	caseRoot := filepath.Join(t.TempDir(), "case")
	writeReviewerIntakeCase(t, repoRoot, caseRoot)
	plan, err := WritePlan(repoRoot, caseRoot, defaults.DefaultPack, Options{TaskType: "feature-analysis", Items: "alpha", Lane: "feature-intake"})
	if err != nil {
		t.Fatal(err)
	}
	packet := readReviewerPacket(t, plan.PacketPath)
	handoff := packet.ShardHandoffs[0]
	if err := os.MkdirAll(filepath.Dir(handoff.ReviewerResultCandidatePath), 0o755); err != nil {
		t.Fatal(err)
	}
	packetSnapshot := reviewerPacketSnapshotForTest(t, caseRoot, plan.PacketPath)
	resultRootID, err := reviewerDirectoryIdentity(caseRoot, packet.ReviewerOrchestration.ResultRoot)
	if err != nil {
		t.Fatal(err)
	}
	source := reviewerResultForPacket(t, packet, "accept", "accepted", nil)
	_, err = publishReviewerResultCandidateAnchoredExpected(caseRoot, packet, handoff, source, packetSnapshot, resultRootID, func(stage string) error {
		if stage != "candidate-linked" {
			return nil
		}
		return os.WriteFile(handoff.ReviewerResultCandidatePath, []byte(`{"corrupt":true}`), 0o644)
	})
	if err == nil {
		t.Fatal("corrupted publication succeeded")
	}
	if _, err := os.Stat(handoff.ReviewerResultCandidatePath); !os.IsNotExist(err) {
		t.Fatalf("corrupted publication was not cleaned: %v", err)
	}
}

func TestPublishReviewerResultCandidateCleansPublicationAfterPacketReplacement(t *testing.T) {
	repoRoot := filepath.Clean(filepath.Join("..", "..", ".."))
	caseRoot := filepath.Join(t.TempDir(), "case")
	writeReviewerIntakeCase(t, repoRoot, caseRoot)
	plan, err := WritePlan(repoRoot, caseRoot, defaults.DefaultPack, Options{TaskType: "feature-analysis", Items: "alpha", Lane: "feature-intake"})
	if err != nil {
		t.Fatal(err)
	}
	packet := readReviewerPacket(t, plan.PacketPath)
	handoff := packet.ShardHandoffs[0]
	if err := os.MkdirAll(filepath.Dir(handoff.ReviewerResultCandidatePath), 0o755); err != nil {
		t.Fatal(err)
	}
	packetSnapshot := reviewerPacketSnapshotForTest(t, caseRoot, plan.PacketPath)
	resultRootID, err := reviewerDirectoryIdentity(caseRoot, packet.ReviewerOrchestration.ResultRoot)
	if err != nil {
		t.Fatal(err)
	}
	source := reviewerResultForPacket(t, packet, "accept", "accepted", nil)
	_, err = publishReviewerResultCandidateAnchoredExpected(caseRoot, packet, handoff, source, packetSnapshot, resultRootID, func(stage string) error {
		if stage != "before-candidate-link" {
			return nil
		}
		data, err := os.ReadFile(plan.PacketPath)
		if err != nil {
			return err
		}
		return os.WriteFile(plan.PacketPath, append(data, ' '), 0o644)
	})
	if err == nil || !strings.Contains(err.Error(), "packet changed") {
		t.Fatalf("packet replacement error = %v", err)
	}
	if _, err := os.Stat(handoff.ReviewerResultCandidatePath); !os.IsNotExist(err) {
		t.Fatalf("packet replacement left candidate: %v", err)
	}
}

func TestPublishReviewerResultCandidateCleansDetachedPublication(t *testing.T) {
	repoRoot := filepath.Clean(filepath.Join("..", "..", ".."))
	caseRoot := filepath.Join(t.TempDir(), "case")
	writeReviewerIntakeCase(t, repoRoot, caseRoot)
	plan, err := WritePlan(repoRoot, caseRoot, defaults.DefaultPack, Options{TaskType: "feature-analysis", Items: "alpha", Lane: "feature-intake"})
	if err != nil {
		t.Fatal(err)
	}
	packet := readReviewerPacket(t, plan.PacketPath)
	handoff := packet.ShardHandoffs[0]
	candidateDir := filepath.Dir(handoff.ReviewerResultCandidatePath)
	if err := os.MkdirAll(candidateDir, 0o755); err != nil {
		t.Fatal(err)
	}
	resultRootID, err := reviewerDirectoryIdentity(caseRoot, packet.ReviewerOrchestration.ResultRoot)
	if err != nil {
		t.Fatal(err)
	}
	detachedDir := candidateDir + ".detached"
	source := reviewerResultForPacket(t, packet, "accept", "accepted", nil)
	_, err = publishReviewerResultCandidateAnchoredExpected(caseRoot, packet, handoff, source, reviewerPacketSnapshotForTest(t, caseRoot, plan.PacketPath), resultRootID, func(stage string) error {
		if stage != "before-candidate-link" {
			return nil
		}
		if err := os.Rename(candidateDir, detachedDir); err != nil {
			return err
		}
		return os.MkdirAll(candidateDir, 0o755)
	})
	if err == nil || !strings.Contains(err.Error(), "directory changed") {
		t.Fatalf("detached publication error = %v", err)
	}
	if _, err := os.Stat(filepath.Join(detachedDir, handoff.ShardID+".json")); !os.IsNotExist(err) {
		t.Fatalf("detached publication was not cleaned: %v", err)
	}
	if _, err := os.Stat(handoff.ReviewerResultCandidatePath); !os.IsNotExist(err) {
		t.Fatalf("detached publication wrote canonical replacement: %v", err)
	}
}

func TestPublishReviewerResultCandidateRejectsCaseInternalDirectoryRebind(t *testing.T) {
	repoRoot := filepath.Clean(filepath.Join("..", "..", ".."))
	caseRoot := filepath.Join(t.TempDir(), "case")
	writeReviewerIntakeCase(t, repoRoot, caseRoot)
	plan, err := WritePlan(repoRoot, caseRoot, defaults.DefaultPack, Options{TaskType: "feature-analysis", Items: "alpha", Lane: "feature-intake"})
	if err != nil {
		t.Fatal(err)
	}
	packet := readReviewerPacket(t, plan.PacketPath)
	handoff := packet.ShardHandoffs[0]
	candidateDir := filepath.Dir(handoff.ReviewerResultCandidatePath)
	if err := os.MkdirAll(candidateDir, 0o755); err != nil {
		t.Fatal(err)
	}
	wrongDir := filepath.Join(caseRoot, "wrong-candidates")
	if err := os.MkdirAll(wrongDir, 0o755); err != nil {
		t.Fatal(err)
	}
	detachedDir := candidateDir + ".detached"
	source := reviewerResultForPacket(t, packet, "accept", "accepted", nil)
	_, err = publishReviewerResultCandidateAnchoredWithHook(caseRoot, packet, handoff, source, func(stage string) error {
		if stage != "candidate-directory-opened" {
			return nil
		}
		if err := os.Rename(candidateDir, detachedDir); err != nil {
			return err
		}
		return os.Symlink(wrongDir, candidateDir)
	})
	if err == nil || !strings.Contains(err.Error(), "directory changed") {
		t.Fatalf("candidate directory rebind error = %v", err)
	}
	if _, err := os.Stat(filepath.Join(wrongDir, handoff.ShardID+".json")); !os.IsNotExist(err) {
		t.Fatalf("directory rebind wrote wrong target: %v", err)
	}
	if _, err := os.Stat(filepath.Join(detachedDir, handoff.ShardID+".json")); !os.IsNotExist(err) {
		t.Fatalf("directory rebind published through detached handle: %v", err)
	}
}

func TestStageReviewerResultFailsClosedOnSourceDriftAndCandidateCollision(t *testing.T) {
	repoRoot := filepath.Clean(filepath.Join("..", "..", ".."))
	caseRoot := filepath.Join(t.TempDir(), "case")
	writeReviewerIntakeCase(t, repoRoot, caseRoot)
	plan, err := WritePlan(repoRoot, caseRoot, defaults.DefaultPack, Options{TaskType: "feature-analysis", Items: "alpha", Lane: "feature-intake"})
	if err != nil {
		t.Fatal(err)
	}
	packet := readReviewerPacket(t, plan.PacketPath)
	handoff := packet.ShardHandoffs[0]
	workspace := filepath.Join(caseRoot, "workspace")
	if err := os.MkdirAll(workspace, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(workspace, "review-evidence.md"), []byte("bounded reviewer evidence\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if handoff.ReviewerStagingCommands == nil || handoff.ReviewerStagingCommands.SourcePath == "" {
		t.Fatalf("missing packet-derived reviewer staging source path: %+v", handoff)
	}
	sourcePath := handoff.ReviewerStagingCommands.SourcePath
	if err := os.MkdirAll(filepath.Dir(sourcePath), 0o755); err != nil {
		t.Fatal(err)
	}
	source := reviewerResultForPacket(t, packet, "accept", "accepted", nil)
	if err := os.WriteFile(sourcePath, source, 0o644); err != nil {
		t.Fatal(err)
	}
	opt := ReviewerResultStagingOptions{PacketPath: plan.PacketPath, ShardID: handoff.ShardID, SourcePath: sourcePath, Lane: packet.TargetLane, Actor: "mission-commander", WhatIf: true}
	preview, err := StageReviewerResult(repoRoot, caseRoot, defaults.DefaultPack, opt)
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(sourcePath, append(source, ' '), 0o644); err != nil {
		t.Fatal(err)
	}
	opt.WhatIf = false
	opt.ExpectedSourceSHA256 = preview.SourceSHA256
	if _, err := StageReviewerResult(repoRoot, caseRoot, defaults.DefaultPack, opt); err == nil || !strings.Contains(err.Error(), "changed after preview") {
		t.Fatalf("source drift error = %v", err)
	}
	if _, err := os.Stat(handoff.ReviewerResultCandidatePath); !os.IsNotExist(err) {
		t.Fatalf("source drift wrote candidate: %v", err)
	}
	if err := os.WriteFile(sourcePath, source, 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(filepath.Dir(handoff.ReviewerResultCandidatePath), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(handoff.ReviewerResultCandidatePath, []byte(`{"occupied":true}`), 0o644); err != nil {
		t.Fatal(err)
	}
	opt.WhatIf = true
	opt.ExpectedSourceSHA256 = ""
	if _, err := StageReviewerResult(repoRoot, caseRoot, defaults.DefaultPack, opt); err == nil || !strings.Contains(err.Error(), "refusing overwrite") {
		t.Fatalf("candidate collision error = %v", err)
	}
	if data, err := os.ReadFile(handoff.ReviewerResultCandidatePath); err != nil || string(data) != `{"occupied":true}` {
		t.Fatalf("candidate collision changed target: %q err=%v", data, err)
	}
	if err := os.Remove(handoff.ReviewerResultCandidatePath); err != nil {
		t.Fatal(err)
	}
	outsideRoot := t.TempDir()
	outsideCandidate := filepath.Join(outsideRoot, handoff.ShardID+".json")
	if err := os.Remove(filepath.Dir(handoff.ReviewerResultCandidatePath)); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink(outsideRoot, filepath.Dir(handoff.ReviewerResultCandidatePath)); err == nil {
		opt.WhatIf = false
		opt.ExpectedSourceSHA256 = preview.SourceSHA256
		if _, err := StageReviewerResult(repoRoot, caseRoot, defaults.DefaultPack, opt); err == nil {
			t.Fatal("candidate parent symlink staging succeeded")
		}
		if _, err := os.Stat(outsideCandidate); !os.IsNotExist(err) {
			t.Fatalf("candidate parent symlink wrote outside case: %v", err)
		}
		if err := os.Remove(filepath.Dir(handoff.ReviewerResultCandidatePath)); err != nil {
			t.Fatal(err)
		}
	}
	internalTarget := filepath.Join(caseRoot, "internal-source")
	if err := os.MkdirAll(internalTarget, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(internalTarget, "reviewer-return.json"), source, 0o644); err != nil {
		t.Fatal(err)
	}
	internalLink := filepath.Join(caseRoot, "linked-source")
	if err := os.Symlink(internalTarget, internalLink); err == nil {
		opt.SourcePath = filepath.Join(internalLink, "reviewer-return.json")
		if _, err := StageReviewerResult(repoRoot, caseRoot, defaults.DefaultPack, opt); err == nil || !strings.Contains(err.Error(), "must match the packet-derived") {
			t.Fatalf("case-internal source symlink error = %v", err)
		}
	}
	outside := filepath.Join(t.TempDir(), "outside.json")
	if err := os.WriteFile(outside, source, 0o644); err != nil {
		t.Fatal(err)
	}
	opt.SourcePath = outside
	if _, err := StageReviewerResult(repoRoot, caseRoot, defaults.DefaultPack, opt); err == nil || !strings.Contains(err.Error(), "must match the packet-derived") {
		t.Fatalf("out-of-case source error = %v", err)
	}
}
