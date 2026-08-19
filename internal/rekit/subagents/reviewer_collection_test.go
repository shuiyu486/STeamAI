package subagents

import (
	"bytes"
	"encoding/json"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"

	"github.com/shuiyu486/re-context-kits/internal/rekit/defaults"
)

func TestCollectReviewerResultWhatIfApplyAndReplay(t *testing.T) {
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
	if handoff.AgentToolRequest == nil || !handoff.AgentToolRequest.ReadOnly || handoff.ReviewerCollectionCommands == nil || !strings.Contains(handoff.ReviewerCollectionCommands.PreviewCommand, "-CollectReviewerResult") {
		t.Fatalf("plan omitted bounded Agent/collection handoff: %+v", handoff)
	}
	if err := os.MkdirAll(filepath.Dir(handoff.ReviewerResultCandidatePath), 0o755); err != nil {
		t.Fatal(err)
	}
	data := reviewerResultForPacket(t, packet, "accept", "accepted", nil)
	if err := os.WriteFile(handoff.ReviewerResultCandidatePath, data, 0o644); err != nil {
		t.Fatal(err)
	}
	opt := ReviewerResultCollectionOptions{PacketPath: plan.PacketPath, ShardID: handoff.ShardID, Lane: packet.TargetLane, Actor: "mission-commander", WhatIf: true}
	preview, err := CollectReviewerResult(repoRoot, caseRoot, defaults.DefaultPack, opt)
	if err != nil {
		t.Fatal(err)
	}
	if preview.Status != "previewed" || preview.IsMutation || preview.Applied || preview.CandidateSHA256 == "" || preview.CandidateBytes == 0 || !strings.Contains(preview.MissionCommanderAction.PrimaryCommand, "-ExpectedCandidateSha256") || !strings.Contains(preview.MissionCommanderAction.PrimaryCommand, preview.CandidateSHA256) {
		t.Fatalf("unexpected collection preview: %+v", preview)
	}
	assertReviewerRunbookContains(t, preview.RunbookSteps, "-CollectReviewerResult")
	assertReviewerRunbookContains(t, preview.RunbookSteps, "hash-bound Apply command")
	if _, err := os.Stat(handoff.ReviewerResultPath); !os.IsNotExist(err) {
		t.Fatalf("collection WhatIf wrote canonical result: %v", err)
	}
	opt.WhatIf = false
	opt.ExpectedCandidateSHA256 = preview.CandidateSHA256
	applied, err := CollectReviewerResult(repoRoot, caseRoot, defaults.DefaultPack, opt)
	if err != nil {
		t.Fatal(err)
	}
	if !applied.Applied || applied.Status != "collected" || applied.AlreadyCollected || applied.ReviewerResultSHA256 != applied.CandidateSHA256 {
		t.Fatalf("unexpected collection apply: %+v", applied)
	}
	assertReviewerRunbookContains(t, applied.RunbookSteps, "ready reviewer results intake")
	canonical, err := os.ReadFile(handoff.ReviewerResultPath)
	if err != nil {
		t.Fatal(err)
	}
	if string(canonical) != string(data) {
		t.Fatalf("canonical result bytes changed during collection")
	}
	replay, err := CollectReviewerResult(repoRoot, caseRoot, defaults.DefaultPack, opt)
	if err != nil {
		t.Fatal(err)
	}
	if !replay.Applied || replay.Status != "already-collected" || !replay.AlreadyCollected {
		t.Fatalf("collection replay was not idempotent: %+v", replay)
	}
}

func TestCollectReviewerResultApplyRejectsCandidateDriftWithoutPublishing(t *testing.T) {
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
	if err := os.MkdirAll(filepath.Dir(handoff.ReviewerResultCandidatePath), 0o755); err != nil {
		t.Fatal(err)
	}
	original := reviewerResultForPacket(t, packet, "accept", "accepted", nil)
	if err := os.WriteFile(handoff.ReviewerResultCandidatePath, original, 0o644); err != nil {
		t.Fatal(err)
	}
	opt := ReviewerResultCollectionOptions{PacketPath: plan.PacketPath, ShardID: handoff.ShardID, Lane: packet.TargetLane, Actor: "mission-commander", WhatIf: true}
	preview, err := CollectReviewerResult(repoRoot, caseRoot, defaults.DefaultPack, opt)
	if err != nil {
		t.Fatal(err)
	}
	changedValue, err := decodeReviewerResult(original)
	if err != nil {
		t.Fatal(err)
	}
	changedValue.Summary = "changed after collection preview"
	changed, err := json.Marshal(changedValue)
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(handoff.ReviewerResultCandidatePath, changed, 0o644); err != nil {
		t.Fatal(err)
	}
	opt.WhatIf = false
	opt.ExpectedCandidateSHA256 = preview.CandidateSHA256
	if _, err := CollectReviewerResult(repoRoot, caseRoot, defaults.DefaultPack, opt); err == nil || !strings.Contains(err.Error(), "expected candidate sha256 mismatch") {
		t.Fatalf("candidate drift apply error = %v", err)
	}
	if _, err := os.Stat(handoff.ReviewerResultPath); !os.IsNotExist(err) {
		t.Fatalf("candidate drift published canonical reviewer result: %v", err)
	}
}

func TestCollectReviewerResultRejectsMalformedCandidateFiles(t *testing.T) {
	repoRoot := filepath.Clean(filepath.Join("..", "..", ".."))
	caseRoot := filepath.Join(t.TempDir(), "case")
	writeReviewerIntakeCase(t, repoRoot, caseRoot)
	plan, err := WritePlan(repoRoot, caseRoot, defaults.DefaultPack, Options{TaskType: "feature-analysis", Items: "alpha", Lane: reviewerIntakeLane})
	if err != nil {
		t.Fatal(err)
	}
	packet := readReviewerPacket(t, plan.PacketPath)
	handoff := packet.ShardHandoffs[0]
	if err := os.MkdirAll(filepath.Dir(handoff.ReviewerResultCandidatePath), 0o755); err != nil {
		t.Fatal(err)
	}
	opt := ReviewerResultCollectionOptions{PacketPath: plan.PacketPath, ShardID: handoff.ShardID, Lane: packet.TargetLane, Actor: "mission-commander", WhatIf: true}

	tests := []struct {
		name string
		data []byte
		want string
	}{
		{name: "empty", data: nil, want: "non-empty regular file"},
		{name: "unknown-field", data: []byte(`{"packetId":"x","routeId":"x","shardId":"x","items":[],"reviewerSession":"x","decision":"defer","confidence":"high","summary":"x","evidenceRefs":[],"risks":[],"conflicts":[],"recommendedVerdict":"deferred","routeOutput":{},"unknown":true}`), want: "unknown field"},
		{name: "trailing-json", data: []byte(`{} {}`), want: "exactly one json object"},
		{name: "oversize", data: bytes.Repeat([]byte("x"), int(maxReviewerResultBytes)+1), want: "exceeds"},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			if err := os.WriteFile(handoff.ReviewerResultCandidatePath, test.data, 0o644); err != nil {
				t.Fatal(err)
			}
			if _, err := CollectReviewerResult(repoRoot, caseRoot, defaults.DefaultPack, opt); err == nil || !strings.Contains(strings.ToLower(err.Error()), test.want) {
				t.Fatalf("candidate error = %v, want substring %q", err, test.want)
			}
		})
	}

	if err := os.Remove(handoff.ReviewerResultCandidatePath); err != nil {
		t.Fatal(err)
	}
	if err := os.Mkdir(handoff.ReviewerResultCandidatePath, 0o755); err != nil {
		t.Fatal(err)
	}
	if _, err := CollectReviewerResult(repoRoot, caseRoot, defaults.DefaultPack, opt); err == nil || !strings.Contains(strings.ToLower(err.Error()), "regular file") {
		t.Fatalf("directory candidate error = %v", err)
	}
}

func TestCollectReviewerResultRejectsRehashedPacketPathForgery(t *testing.T) {
	repoRoot := filepath.Clean(filepath.Join("..", "..", ".."))
	caseRoot := filepath.Join(t.TempDir(), "case")
	writeReviewerIntakeCase(t, repoRoot, caseRoot)
	plan, err := WritePlan(repoRoot, caseRoot, defaults.DefaultPack, Options{TaskType: "feature-analysis", Items: "alpha", Lane: reviewerIntakeLane})
	if err != nil {
		t.Fatal(err)
	}
	packet := readReviewerPacket(t, plan.PacketPath)
	outsideRoot := filepath.Join(t.TempDir(), "forged-results")
	handoff := &packet.ShardHandoffs[0]
	packet.ReviewerOrchestration.ResultRoot = outsideRoot
	packet.Observability.ResultRoot = outsideRoot
	handoff.ReviewerResultCandidatePath = filepath.Join(outsideRoot, "candidates", handoff.ShardID+".json")
	handoff.ReviewerResultPath = filepath.Join(outsideRoot, handoff.ShardID+".json")
	packet.ReviewerOrchestration.Dispatches[0].ReviewerResultCandidatePath = handoff.ReviewerResultCandidatePath
	packet.ReviewerOrchestration.Dispatches[0].ReviewerResultPath = handoff.ReviewerResultPath
	packet.PacketID = packetIdentity(packet)
	data, err := json.Marshal(packet)
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(plan.PacketPath, data, 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(filepath.Dir(handoff.ReviewerResultCandidatePath), 0o755); err != nil {
		t.Fatal(err)
	}
	candidate := reviewerResultForPacket(t, packet, "accept", "accepted", nil)
	if err := os.WriteFile(handoff.ReviewerResultCandidatePath, candidate, 0o644); err != nil {
		t.Fatal(err)
	}
	opt := ReviewerResultCollectionOptions{PacketPath: plan.PacketPath, ShardID: handoff.ShardID, Lane: packet.TargetLane, Actor: "mission-commander", WhatIf: true}
	if _, err := CollectReviewerResult(repoRoot, caseRoot, defaults.DefaultPack, opt); err == nil || (!strings.Contains(err.Error(), "canonical case review namespace") && !strings.Contains(err.Error(), "packet integrity")) {
		t.Fatalf("rehashed packet path forgery error = %v", err)
	}
}

func TestCollectReviewerResultRejectsNestedCanonicalPacket(t *testing.T) {
	repoRoot := filepath.Clean(filepath.Join("..", "..", ".."))
	caseRoot := filepath.Join(t.TempDir(), "case")
	writeReviewerIntakeCase(t, repoRoot, caseRoot)
	reviewRoot := filepath.Join(caseRoot, ".rekit", "reviews", "outer", "inner")
	plan, err := WritePlan(repoRoot, caseRoot, defaults.DefaultPack, Options{TaskType: "feature-analysis", Items: "alpha", Lane: reviewerIntakeLane, ReviewOutputDir: reviewRoot})
	if err != nil {
		t.Fatal(err)
	}
	packet := readReviewerPacket(t, plan.PacketPath)
	handoff := &packet.ShardHandoffs[0]
	if handoff.ReviewerCollectionCommands != nil || handoff.ReviewerResultCandidatePath != "" || packet.ReviewerOrchestration.Summary.CollectionAvailable {
		t.Fatalf("nested packet advertised collection: %+v", packet.ReviewerOrchestration)
	}
	candidatePath := filepath.Join(packet.ReviewerOrchestration.ResultRoot, "candidates", handoff.ShardID+".json")
	handoff.ReviewerResultCandidatePath = candidatePath
	packet.ReviewerOrchestration.Dispatches[0].ReviewerResultCandidatePath = candidatePath
	packet.PacketID = packetIdentity(packet)
	data, err := json.Marshal(packet)
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(plan.PacketPath, append(data, '\n'), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(filepath.Dir(candidatePath), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(candidatePath, reviewerResultForPacket(t, packet, "accept", "accepted", nil), 0o644); err != nil {
		t.Fatal(err)
	}
	opt := ReviewerResultCollectionOptions{PacketPath: plan.PacketPath, ShardID: handoff.ShardID, Lane: packet.TargetLane, Actor: "mission-commander", WhatIf: true}
	if _, err := CollectReviewerResult(repoRoot, caseRoot, defaults.DefaultPack, opt); err == nil || !strings.Contains(err.Error(), "canonical case review packet") {
		t.Fatalf("nested packet collection error = %v", err)
	}
}

func TestCollectReviewerResultRejectsRekitSymlinkBeforeReadingOrPublishing(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("symlink creation is not reliably available on Windows")
	}
	repoRoot := filepath.Clean(filepath.Join("..", "..", ".."))
	caseRoot := filepath.Join(t.TempDir(), "case")
	writeReviewerIntakeCase(t, repoRoot, caseRoot)
	plan, err := WritePlan(repoRoot, caseRoot, defaults.DefaultPack, Options{TaskType: "feature-analysis", Items: "alpha", Lane: reviewerIntakeLane})
	if err != nil {
		t.Fatal(err)
	}
	packet := readReviewerPacket(t, plan.PacketPath)
	handoff := packet.ShardHandoffs[0]
	if err := os.MkdirAll(filepath.Dir(handoff.ReviewerResultCandidatePath), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(handoff.ReviewerResultCandidatePath, reviewerResultForPacket(t, packet, "accept", "accepted", nil), 0o644); err != nil {
		t.Fatal(err)
	}
	outside := filepath.Join(t.TempDir(), "outside-metadata")
	if err := os.Rename(filepath.Join(caseRoot, ".rekit"), outside); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink(outside, filepath.Join(caseRoot, ".rekit")); err != nil {
		t.Fatal(err)
	}
	opt := ReviewerResultCollectionOptions{PacketPath: plan.PacketPath, ShardID: handoff.ShardID, Lane: packet.TargetLane, Actor: "mission-commander"}
	for _, whatIf := range []bool{true, false} {
		opt.WhatIf = whatIf
		if _, err := CollectReviewerResult(repoRoot, caseRoot, defaults.DefaultPack, opt); err == nil || !strings.Contains(strings.ToLower(err.Error()), "symlink") {
			t.Fatalf("collection with .rekit symlink WhatIf=%t error = %v", whatIf, err)
		}
		if _, err := os.Stat(filepath.Join(outside, "reviews", filepath.Base(filepath.Dir(plan.PacketPath)), "results", handoff.ShardID+".json")); !os.IsNotExist(err) {
			t.Fatalf("collection WhatIf=%t wrote outside canonical result: %v", whatIf, err)
		}
	}
}

func TestCollectReviewerResultRejectsBindingsCollisionAndSymlink(t *testing.T) {
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
	if err := os.MkdirAll(filepath.Dir(handoff.ReviewerResultCandidatePath), 0o755); err != nil {
		t.Fatal(err)
	}
	value, err := decodeReviewerResult(reviewerResultForPacket(t, packet, "accept", "accepted", nil))
	if err != nil {
		t.Fatal(err)
	}
	value.PacketID = "packet-forged"
	forged, _ := json.Marshal(value)
	if err := os.WriteFile(handoff.ReviewerResultCandidatePath, forged, 0o644); err != nil {
		t.Fatal(err)
	}
	opt := ReviewerResultCollectionOptions{PacketPath: plan.PacketPath, ShardID: handoff.ShardID, Lane: packet.TargetLane, Actor: "mission-commander", WhatIf: true}
	if _, err := CollectReviewerResult(repoRoot, caseRoot, defaults.DefaultPack, opt); err == nil || !strings.Contains(err.Error(), "does not match packet") {
		t.Fatalf("forged candidate error = %v", err)
	}
	valid := reviewerResultForPacket(t, packet, "accept", "accepted", nil)
	if err := os.WriteFile(handoff.ReviewerResultCandidatePath, valid, 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(handoff.ReviewerResultPath, []byte(`{"different":true}`), 0o644); err != nil {
		t.Fatal(err)
	}
	recoveryRequired, err := CollectReviewerResult(repoRoot, caseRoot, defaults.DefaultPack, opt)
	if !reviewerResultExactMoveSupported("regular-file") {
		if err == nil || !strings.Contains(err.Error(), "cannot be recovered from collection WhatIf") {
			t.Fatalf("unsupported canonical collision preview error = %v result=%+v", err, recoveryRequired)
		}
		if current, readErr := os.ReadFile(handoff.ReviewerResultPath); readErr != nil {
			t.Fatal(readErr)
		} else if string(current) != `{"different":true}` {
			t.Fatalf("unsupported recovery changed canonical bytes: %q", current)
		}
		return
	}
	if err != nil {
		t.Fatalf("canonical collision preview returned error instead of recovery handoff: %v", err)
	}
	if recoveryRequired.Status != "recovery-required" || !recoveryRequired.RecoveryRequired || recoveryRequired.ReviewerResultSHA256 == recoveryRequired.CandidateSHA256 || !strings.Contains(recoveryRequired.MissionCommanderAction.PrimaryCommand, "-RecoverReviewerResult") {
		t.Fatalf("canonical collision preview omitted recovery handoff: %+v", recoveryRequired)
	}
	assertReviewerRunbookContains(t, recoveryRequired.RunbookSteps, "-RecoverReviewerResult")
	assertReviewerRunbookContains(t, recoveryRequired.RunbookSteps, "separate bounded operations")
	opt.WhatIf = false
	if _, err := CollectReviewerResult(repoRoot, caseRoot, defaults.DefaultPack, opt); err == nil || !strings.Contains(err.Error(), "refusing overwrite") {
		t.Fatalf("canonical collision apply error = %v", err)
	}

	if err := os.Remove(handoff.ReviewerResultPath); err != nil {
		t.Fatal(err)
	}
	if err := os.Remove(handoff.ReviewerResultCandidatePath); err != nil {
		t.Fatal(err)
	}
	target := filepath.Join(t.TempDir(), "candidate.json")
	if err := os.WriteFile(target, valid, 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink(target, handoff.ReviewerResultCandidatePath); err != nil {
		t.Skipf("symlink unavailable: %v", err)
	}
	opt.WhatIf = true
	if _, err := CollectReviewerResult(repoRoot, caseRoot, defaults.DefaultPack, opt); err == nil || !strings.Contains(strings.ToLower(err.Error()), "symlink") {
		t.Fatalf("candidate symlink error = %v", err)
	}
}
