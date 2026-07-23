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

func TestRetireInvalidReviewerPacketWhatIfApplyAndDrift(t *testing.T) {
	repoRoot := filepath.Clean(filepath.Join("..", "..", ".."))
	caseRoot := filepath.Join(t.TempDir(), "case")
	writeReviewerIntakeCase(t, repoRoot, caseRoot)
	plan, err := WritePlan(repoRoot, caseRoot, defaults.DefaultPack, Options{TaskType: "feature-analysis", Items: "alpha", Lane: "feature-intake"})
	if err != nil {
		t.Fatal(err)
	}
	packetData, err := os.ReadFile(plan.PacketPath)
	if err != nil {
		t.Fatal(err)
	}
	var packet map[string]any
	if err := json.Unmarshal(packetData, &packet); err != nil {
		t.Fatal(err)
	}
	packet["targetLane"] = "tampered-lane"
	tampered, err := json.Marshal(packet)
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(plan.PacketPath, append(tampered, '\n'), 0o644); err != nil {
		t.Fatal(err)
	}
	opt := ReviewerPacketRetirementOptions{PacketPath: plan.PacketPath, Lane: "feature-intake", Actor: "mission-commander", Reason: "retire exact corrupted reviewer packet", WhatIf: true}
	preview, err := RetireInvalidReviewerPacket(repoRoot, caseRoot, defaults.DefaultPack, opt)
	if err != nil {
		t.Fatal(err)
	}
	if preview.Applied || preview.IsMutation || preview.InvalidReason == "" || preview.PacketSHA256 == "" || preview.IntegritySHA256 == "" || preview.MissionCommanderAction.State != "needs-reviewer-packet-retirement-apply" || !strings.Contains(preview.MissionCommanderAction.PrimaryCommand, "-ExpectedPacketSha256") {
		t.Fatalf("unexpected retirement preview: %+v", preview)
	}
	if _, err := os.Stat(preview.RetirementPath); !os.IsNotExist(err) {
		t.Fatalf("retirement WhatIf wrote receipt: %v", err)
	}
	handoffs, err := workstream.ReviewerDispatchIntakeHandoffs(caseRoot, mission.LedgerFacts{}, "feature-intake")
	if err != nil || len(handoffs) != 1 || handoffs[0].State != "reviewer-packet-integrity-invalid" {
		t.Fatalf("invalid packet blocker missing before retirement: handoffs=%+v err=%v", handoffs, err)
	}
	opt.WhatIf = false
	opt.ExpectedPacketSHA256 = preview.PacketSHA256
	opt.ExpectedIntegritySHA256 = preview.IntegritySHA256
	applied, err := RetireInvalidReviewerPacket(repoRoot, caseRoot, defaults.DefaultPack, opt)
	if err != nil {
		t.Fatal(err)
	}
	if !applied.Applied || applied.MissionCommanderAction.State != "reviewer-packet-retired" {
		t.Fatalf("unexpected retirement apply: %+v", applied)
	}
	replayed, err := RetireInvalidReviewerPacket(repoRoot, caseRoot, defaults.DefaultPack, opt)
	if err != nil {
		t.Fatal(err)
	}
	if !replayed.Applied || replayed.RetirementPath != applied.RetirementPath {
		t.Fatalf("unexpected retirement replay: %+v", replayed)
	}
	handoffs, err = workstream.ReviewerDispatchIntakeHandoffs(caseRoot, mission.LedgerFacts{}, "feature-intake")
	if err != nil || len(handoffs) != 0 {
		t.Fatalf("exact retirement did not close blocker: handoffs=%+v err=%v", handoffs, err)
	}
	if err := os.WriteFile(plan.PacketPath, append(tampered, '\n', ' '), 0o644); err != nil {
		t.Fatal(err)
	}
	handoffs, err = workstream.ReviewerDispatchIntakeHandoffs(caseRoot, mission.LedgerFacts{}, "feature-intake")
	if err != nil || len(handoffs) != 1 || handoffs[0].State != "reviewer-packet-integrity-invalid" {
		t.Fatalf("changed packet did not invalidate retirement: handoffs=%+v err=%v", handoffs, err)
	}
}

func TestRetireInvalidReviewerPacketApplyRequiresBothPreviewHashes(t *testing.T) {
	repoRoot := filepath.Clean(filepath.Join("..", "..", ".."))
	caseRoot := filepath.Join(t.TempDir(), "case")
	writeReviewerIntakeCase(t, repoRoot, caseRoot)
	plan, err := WritePlan(repoRoot, caseRoot, defaults.DefaultPack, Options{TaskType: "feature-analysis", Items: "alpha", Lane: "feature-intake"})
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(plan.PacketPath, []byte("{truncated"), 0o644); err != nil {
		t.Fatal(err)
	}
	opt := ReviewerPacketRetirementOptions{PacketPath: plan.PacketPath, Lane: "feature-intake", Actor: "mission-commander", Reason: "retire", WhatIf: false}
	if _, err := RetireInvalidReviewerPacket(repoRoot, caseRoot, defaults.DefaultPack, opt); err == nil || !strings.Contains(err.Error(), "expected packet SHA-256") {
		t.Fatalf("missing expected hashes error = %v", err)
	}
	opt.ExpectedPacketSHA256 = strings.Repeat("0", 64)
	if _, err := RetireInvalidReviewerPacket(repoRoot, caseRoot, defaults.DefaultPack, opt); err == nil || !strings.Contains(err.Error(), "expected integrity SHA-256") {
		t.Fatalf("missing expected integrity hash error = %v", err)
	}
}

func TestRetireInvalidReviewerPacketApplyRejectsPreviewDrift(t *testing.T) {
	repoRoot := filepath.Clean(filepath.Join("..", "..", ".."))
	caseRoot := filepath.Join(t.TempDir(), "case")
	writeReviewerIntakeCase(t, repoRoot, caseRoot)
	plan, err := WritePlan(repoRoot, caseRoot, defaults.DefaultPack, Options{TaskType: "feature-analysis", Items: "alpha", Lane: "feature-intake"})
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(plan.PacketPath, []byte("{truncated"), 0o644); err != nil {
		t.Fatal(err)
	}
	opt := ReviewerPacketRetirementOptions{PacketPath: plan.PacketPath, Lane: "feature-intake", Actor: "mission-commander", Reason: "retire", WhatIf: true}
	preview, err := RetireInvalidReviewerPacket(repoRoot, caseRoot, defaults.DefaultPack, opt)
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(plan.PacketPath, []byte("{different-truncated"), 0o644); err != nil {
		t.Fatal(err)
	}
	opt.WhatIf = false
	opt.ExpectedPacketSHA256 = preview.PacketSHA256
	opt.ExpectedIntegritySHA256 = preview.IntegritySHA256
	if _, err := RetireInvalidReviewerPacket(repoRoot, caseRoot, defaults.DefaultPack, opt); err == nil || !strings.Contains(err.Error(), "changed after retirement preview") {
		t.Fatalf("preview drift error = %v", err)
	}
}

func TestRetireInvalidReviewerPacketRejectsForgedReceipt(t *testing.T) {
	repoRoot := filepath.Clean(filepath.Join("..", "..", ".."))
	caseRoot := filepath.Join(t.TempDir(), "case")
	writeReviewerIntakeCase(t, repoRoot, caseRoot)
	plan, err := WritePlan(repoRoot, caseRoot, defaults.DefaultPack, Options{TaskType: "feature-analysis", Items: "alpha", Lane: "feature-intake"})
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(plan.PacketPath, []byte("{truncated"), 0o644); err != nil {
		t.Fatal(err)
	}
	opt := ReviewerPacketRetirementOptions{PacketPath: plan.PacketPath, Lane: "feature-intake", Actor: "mission-commander", Reason: "retire", WhatIf: true}
	preview, err := RetireInvalidReviewerPacket(repoRoot, caseRoot, defaults.DefaultPack, opt)
	if err != nil {
		t.Fatal(err)
	}
	opt.WhatIf = false
	opt.ExpectedPacketSHA256 = preview.PacketSHA256
	opt.ExpectedIntegritySHA256 = preview.IntegritySHA256
	applied, err := RetireInvalidReviewerPacket(repoRoot, caseRoot, defaults.DefaultPack, opt)
	if err != nil {
		t.Fatal(err)
	}
	data, err := os.ReadFile(applied.RetirementPath)
	if err != nil {
		t.Fatal(err)
	}
	var receipt map[string]any
	if err := json.Unmarshal(data, &receipt); err != nil {
		t.Fatal(err)
	}
	receipt["pack"] = "forged-pack"
	forged, err := json.MarshalIndent(receipt, "", "  ")
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(applied.RetirementPath, append(forged, '\n'), 0o600); err != nil {
		t.Fatal(err)
	}
	handoffs, err := workstream.ReviewerDispatchIntakeHandoffs(caseRoot, mission.LedgerFacts{}, "feature-intake")
	if err != nil || len(handoffs) != 1 || handoffs[0].State != "reviewer-packet-integrity-invalid" {
		t.Fatalf("forged receipt suppressed blocker: handoffs=%+v err=%v", handoffs, err)
	}
	if _, err := RetireInvalidReviewerPacket(repoRoot, caseRoot, defaults.DefaultPack, opt); err == nil || !strings.Contains(err.Error(), "different snapshot or decision") {
		t.Fatalf("forged receipt replay error = %v", err)
	}
}

func TestRetireInvalidReviewerPacketRejectsMissingOrMalformedIntegrity(t *testing.T) {
	repoRoot := filepath.Clean(filepath.Join("..", "..", ".."))
	for _, malformed := range []bool{false, true} {
		t.Run(map[bool]string{false: "missing", true: "malformed"}[malformed], func(t *testing.T) {
			caseRoot := filepath.Join(t.TempDir(), "case")
			writeReviewerIntakeCase(t, repoRoot, caseRoot)
			plan, err := WritePlan(repoRoot, caseRoot, defaults.DefaultPack, Options{TaskType: "feature-analysis", Items: "alpha", Lane: "feature-intake"})
			if err != nil {
				t.Fatal(err)
			}
			if err := os.WriteFile(plan.PacketPath, []byte("{truncated"), 0o644); err != nil {
				t.Fatal(err)
			}
			integrityPath := filepath.Join(filepath.Dir(plan.PacketPath), "packet.integrity.json")
			if malformed {
				if err := os.WriteFile(integrityPath, []byte("{malformed"), 0o644); err != nil {
					t.Fatal(err)
				}
			} else if err := os.Remove(integrityPath); err != nil {
				t.Fatal(err)
			}
			opt := ReviewerPacketRetirementOptions{PacketPath: plan.PacketPath, Lane: "feature-intake", Actor: "mission-commander", Reason: "retire", WhatIf: true}
			if _, err := RetireInvalidReviewerPacket(repoRoot, caseRoot, defaults.DefaultPack, opt); err == nil {
				t.Fatal("retirement accepted packet without strict integrity provenance")
			}
		})
	}
}

func TestRetireInvalidReviewerPacketRejectsValidAndWrongLane(t *testing.T) {
	repoRoot := filepath.Clean(filepath.Join("..", "..", ".."))
	caseRoot := filepath.Join(t.TempDir(), "case")
	writeReviewerIntakeCase(t, repoRoot, caseRoot)
	plan, err := WritePlan(repoRoot, caseRoot, defaults.DefaultPack, Options{TaskType: "feature-analysis", Items: "alpha", Lane: "feature-intake"})
	if err != nil {
		t.Fatal(err)
	}
	opt := ReviewerPacketRetirementOptions{PacketPath: plan.PacketPath, Lane: "feature-intake", Actor: "mission-commander", Reason: "retire", WhatIf: true}
	if _, err := RetireInvalidReviewerPacket(repoRoot, caseRoot, defaults.DefaultPack, opt); err == nil || !strings.Contains(err.Error(), "packet is valid") {
		t.Fatalf("valid packet retirement error = %v", err)
	}
	if err := os.WriteFile(plan.PacketPath, []byte("{truncated"), 0o644); err != nil {
		t.Fatal(err)
	}
	opt.Lane = "other-lane"
	if _, err := RetireInvalidReviewerPacket(repoRoot, caseRoot, defaults.DefaultPack, opt); err == nil || !strings.Contains(err.Error(), "matching -Lane") {
		t.Fatalf("wrong lane retirement error = %v", err)
	}
}
