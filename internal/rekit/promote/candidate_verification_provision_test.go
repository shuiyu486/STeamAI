package promote

import (
	"bytes"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/shuiyu486/re-context-kits/internal/rekit/projectstate"
	syncpkg "github.com/shuiyu486/re-context-kits/internal/rekit/sync"
)

func TestCandidateDecisionVerificationWorkspaceUsesSingleStateRoot(t *testing.T) {
	for _, tc := range []struct {
		name string
		dir  string
	}{
		{name: "current", dir: projectstate.CurrentDir},
		{name: "legacy", dir: projectstate.LegacyDir},
	} {
		t.Run(tc.name, func(t *testing.T) {
			caseRoot := t.TempDir()
			if err := os.Mkdir(filepath.Join(caseRoot, tc.dir), 0o700); err != nil {
				t.Fatal(err)
			}
			workspace, err := candidateDecisionVerificationWorkspace(caseRoot, strings.Repeat("a", 64), strings.Repeat("b", 64))
			if err != nil {
				t.Fatal(err)
			}
			want := filepath.Join(caseRoot, tc.dir, "verifications", "candidate-decisions", shortHash(strings.Repeat("a", 64)+strings.Repeat("b", 64)))
			if workspace != want {
				t.Fatalf("workspace = %q, want %q", workspace, want)
			}
		})
	}

	caseRoot := t.TempDir()
	for _, dir := range []string{projectstate.CurrentDir, projectstate.LegacyDir} {
		if err := os.Mkdir(filepath.Join(caseRoot, dir), 0o700); err != nil {
			t.Fatal(err)
		}
	}
	if _, err := candidateDecisionVerificationWorkspace(caseRoot, "packet", "decision"); err == nil || !strings.Contains(err.Error(), "must not coexist") {
		t.Fatalf("dual-root workspace error = %v", err)
	}
}

func TestProvisionCandidateVerificationCasesPreviewsAppliesReplaysAndVerifies(t *testing.T) {
	repoRoot, sourceCase, _, pack := packMemoryReconsumeFixture(t)
	if _, err := syncpkg.Apply(repoRoot, sourceCase, pack, syncpkg.ApplyOptions{CreateLocalFiles: true, Command: "init", ProjectName: "source"}); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(sourceCase, filepath.FromSlash("references/template/README.md")), []byte("# README\n\nProvisioned reusable candidate.\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	created, packet, managed := candidateDecisionFixture(t, repoRoot, sourceCase, pack, "provision")
	decision := reviewedCandidateDecision(packet, managed, created.ReviewWorkspace.CombinedDiffPath)
	decisionPath := filepath.Join(sourceCase, ".rekit", "reviews", "candidate-provision", "decisions.json")
	writeCandidateDecisionFixture(t, decisionPath, decision)
	if _, err := ApplyCandidateDecisions(repoRoot, sourceCase, pack, CandidateDecisionOptions{PacketPath: created.ReviewWorkspace.PacketPath, DecisionPath: decisionPath}); err != nil {
		t.Fatal(err)
	}
	packetBytes, err := os.ReadFile(created.ReviewWorkspace.PacketPath)
	if err != nil {
		t.Fatal(err)
	}
	decisionBytes, err := os.ReadFile(decisionPath)
	if err != nil {
		t.Fatal(err)
	}
	provisionID := shortHash(sha256Hex(packetBytes) + sha256Hex(decisionBytes))
	workspace := filepath.Join(sourceCase, ".rekit", "verifications", "candidate-decisions", provisionID)
	freshRoot := filepath.Join(workspace, "fresh")
	attachedRoot := filepath.Join(workspace, "attached")
	opt := CandidateVerificationProvisionOptions{PacketPath: created.ReviewWorkspace.PacketPath, DecisionPath: decisionPath, FreshCaseRoot: freshRoot, AttachedCaseRoot: attachedRoot, WhatIf: true}
	preview, err := ProvisionCandidateVerificationCases(repoRoot, sourceCase, pack, opt)
	if err != nil {
		t.Fatal(err)
	}
	if preview.IsMutation || preview.Applied || preview.ProvisionSHA256 == "" || len(preview.Cases) != 2 || !strings.Contains(preview.ApplyCommand, "-ExpectedProvisionSha256") {
		t.Fatalf("unexpected provision preview: %+v", preview)
	}
	if _, err := os.Stat(workspace); !os.IsNotExist(err) {
		t.Fatalf("provision WhatIf wrote workspace: %v", err)
	}
	opt.WhatIf = false
	opt.ExpectedProvisionSHA256 = preview.ProvisionSHA256
	applied, err := ProvisionCandidateVerificationCases(repoRoot, sourceCase, pack, opt)
	if err != nil {
		t.Fatal(err)
	}
	if !applied.IsMutation || !applied.Applied || applied.Replay || applied.Mode != "provisioned" || applied.Cases[0].DoctorRows == 0 || applied.Cases[1].DoctorRows == 0 {
		t.Fatalf("unexpected provision apply: %+v", applied)
	}
	if _, err := os.Stat(applied.ReceiptPath); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(filepath.Join(repoRoot, "packs", pack, "promote-candidates", "review-artifacts", provisionID+".candidate-verification-proof.json")); !os.IsNotExist(err) {
		t.Fatalf("provisioning wrote final proof: %v", err)
	}
	replay, err := ProvisionCandidateVerificationCases(repoRoot, sourceCase, pack, opt)
	if err != nil {
		t.Fatal(err)
	}
	if !replay.Applied || !replay.Replay || replay.Mode != "already-provisioned" {
		t.Fatalf("unexpected provision replay: %+v", replay)
	}
	verified, err := VerifyCandidateDecision(repoRoot, sourceCase, pack, CandidateDecisionVerificationOptions{PacketPath: created.ReviewWorkspace.PacketPath, DecisionPath: decisionPath, FreshCaseRoot: freshRoot, AttachedCaseRoot: attachedRoot, WhatIf: true})
	if err != nil {
		t.Fatal(err)
	}
	if !verified.Ready || verified.Applied {
		t.Fatalf("provisioned cases were not ready for verification: %+v", verified)
	}
}

func TestProvisionCandidateVerificationCasesResumesMatchingIntent(t *testing.T) {
	repoRoot, sourceCase, pack, packetPath, decisionPath, workspace, freshRoot, attachedRoot, preview := candidateProvisionFixture(t, "resume")
	prepared, err := prepareCandidateVerificationProvision(repoRoot, sourceCase, pack, CandidateVerificationProvisionOptions{PacketPath: packetPath, DecisionPath: decisionPath, FreshCaseRoot: freshRoot, AttachedCaseRoot: attachedRoot})
	if err != nil {
		t.Fatal(err)
	}
	if err := createDirectoryChainNoFollow(sourceCase, workspace); err != nil {
		t.Fatal(err)
	}
	if err := writeDurableExclusiveFile(prepared.result.IntentPath, prepared.intentBytes); err != nil {
		t.Fatal(err)
	}
	if _, err := syncpkg.ApplyExclusiveInit(prepared.freshPlan); err != nil {
		t.Fatal(err)
	}
	if err := os.Mkdir(attachedRoot, 0o755); err != nil {
		t.Fatal(err)
	}
	first := prepared.attachedPlan.Writes[0]
	if err := os.MkdirAll(filepath.Dir(first.TargetPath), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(first.TargetPath, first.Content, 0o644); err != nil {
		t.Fatal(err)
	}
	result, err := ProvisionCandidateVerificationCases(repoRoot, sourceCase, pack, CandidateVerificationProvisionOptions{PacketPath: packetPath, DecisionPath: decisionPath, FreshCaseRoot: freshRoot, AttachedCaseRoot: attachedRoot, ExpectedProvisionSHA256: preview.ProvisionSHA256})
	if err != nil {
		t.Fatal(err)
	}
	if !result.Applied || !result.Cases[0].Replay || !result.Cases[1].Replay {
		t.Fatalf("matching intent did not resume complete/partial roots: %+v", result)
	}
}

func TestProvisionCandidateVerificationCasesSecondRootCollisionLeavesNoFreshRoot(t *testing.T) {
	repoRoot, sourceCase, pack, packetPath, decisionPath, workspace, freshRoot, attachedRoot, preview := candidateProvisionFixture(t, "second-collision")
	if err := os.MkdirAll(workspace, 0o755); err != nil {
		t.Fatal(err)
	}
	prepared, err := prepareCandidateVerificationProvision(repoRoot, sourceCase, pack, CandidateVerificationProvisionOptions{PacketPath: packetPath, DecisionPath: decisionPath, FreshCaseRoot: freshRoot, AttachedCaseRoot: attachedRoot})
	if err != nil {
		t.Fatal(err)
	}
	if err := writeDurableExclusiveFile(prepared.result.IntentPath, prepared.intentBytes); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(attachedRoot, []byte("collision"), 0o644); err != nil {
		t.Fatal(err)
	}
	if _, err := ProvisionCandidateVerificationCases(repoRoot, sourceCase, pack, CandidateVerificationProvisionOptions{PacketPath: packetPath, DecisionPath: decisionPath, FreshCaseRoot: freshRoot, AttachedCaseRoot: attachedRoot, ExpectedProvisionSHA256: preview.ProvisionSHA256}); err == nil {
		t.Fatal("second-root collision was accepted")
	}
	if _, err := os.Lstat(freshRoot); !os.IsNotExist(err) {
		t.Fatalf("second-root collision left fresh root: %v", err)
	}
	if got := string(readProvisionFile(t, attachedRoot)); got != "collision" {
		t.Fatalf("second-root collision changed: %q", got)
	}
}

func TestProvisionCandidateVerificationCasesWorkspaceRebindFailsBeforeRootMutation(t *testing.T) {
	repoRoot, sourceCase, pack, packetPath, decisionPath, workspace, freshRoot, attachedRoot, _ := candidateProvisionFixture(t, "workspace-rebind")
	prepared, err := prepareCandidateVerificationProvision(repoRoot, sourceCase, pack, CandidateVerificationProvisionOptions{PacketPath: packetPath, DecisionPath: decisionPath, FreshCaseRoot: freshRoot, AttachedCaseRoot: attachedRoot})
	if err != nil {
		t.Fatal(err)
	}
	if err := ensureCandidateVerificationProvisionWorkspace(&prepared); err != nil {
		t.Fatal(err)
	}
	prepared.closeWorkspace()
	moved := workspace + "-moved"
	if err := os.Rename(workspace, moved); err != nil {
		t.Fatal(err)
	}
	if err := os.Mkdir(workspace, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := revalidateCandidateVerificationWorkspace(prepared); err == nil || !(strings.Contains(err.Error(), "rebound") || strings.Contains(err.Error(), "intent changed")) {
		t.Fatalf("workspace rebind error = %v", err)
	}
	if _, err := os.Lstat(freshRoot); !os.IsNotExist(err) {
		t.Fatalf("workspace rebind left fresh root: %v", err)
	}
	if _, err := os.Lstat(attachedRoot); !os.IsNotExist(err) {
		t.Fatalf("workspace rebind left attached root: %v", err)
	}
}

func TestProvisionCandidateVerificationCasesRootRebindBeforeDoctorWritesNoReplacement(t *testing.T) {
	repoRoot, sourceCase, pack, packetPath, decisionPath, _, freshRoot, attachedRoot, preview := candidateProvisionFixture(t, "doctor-root-rebind")
	moved := freshRoot + "-moved"
	originalHook := candidateVerificationProvisionStageHook
	candidateVerificationProvisionStageHook = func(stage string) error {
		if stage != "before-doctor" {
			return nil
		}
		candidateVerificationProvisionStageHook = nil
		if err := os.Rename(freshRoot, moved); err != nil {
			return fmt.Errorf("root rebind failed closed: %w", err)
		}
		return os.Mkdir(freshRoot, 0o755)
	}
	t.Cleanup(func() { candidateVerificationProvisionStageHook = originalHook })
	result, err := ProvisionCandidateVerificationCases(repoRoot, sourceCase, pack, CandidateVerificationProvisionOptions{PacketPath: packetPath, DecisionPath: decisionPath, FreshCaseRoot: freshRoot, AttachedCaseRoot: attachedRoot, ExpectedProvisionSHA256: preview.ProvisionSHA256})
	if err == nil || result.Applied {
		t.Fatalf("doctor root rebind returned success: result=%+v err=%v", result, err)
	}
	if _, statErr := os.Lstat(filepath.Join(freshRoot, ".rekit")); !os.IsNotExist(statErr) {
		t.Fatalf("replacement root received doctor/provision writes: %v", statErr)
	}
}

func TestProvisionCandidateVerificationCasesWorkspaceRebindBeforeReceiptWritesNoReplacement(t *testing.T) {
	repoRoot, sourceCase, pack, packetPath, decisionPath, workspace, freshRoot, attachedRoot, preview := candidateProvisionFixture(t, "receipt-workspace-rebind")
	moved := workspace + "-moved"
	originalHook := candidateVerificationProvisionStageHook
	candidateVerificationProvisionStageHook = func(stage string) error {
		if stage != "before-receipt" {
			return nil
		}
		candidateVerificationProvisionStageHook = nil
		if err := os.Rename(workspace, moved); err != nil {
			return fmt.Errorf("workspace rebind failed closed: %w", err)
		}
		return os.Mkdir(workspace, 0o755)
	}
	t.Cleanup(func() { candidateVerificationProvisionStageHook = originalHook })
	result, err := ProvisionCandidateVerificationCases(repoRoot, sourceCase, pack, CandidateVerificationProvisionOptions{PacketPath: packetPath, DecisionPath: decisionPath, FreshCaseRoot: freshRoot, AttachedCaseRoot: attachedRoot, ExpectedProvisionSHA256: preview.ProvisionSHA256})
	if err == nil || result.Applied {
		t.Fatalf("workspace receipt rebind returned success: result=%+v err=%v", result, err)
	}
	if _, statErr := os.Lstat(filepath.Join(workspace, "provision.receipt.json")); !os.IsNotExist(statErr) {
		t.Fatalf("replacement workspace received receipt: %v", statErr)
	}
}

func TestProvisionCandidateVerificationCasesResumesReceiptPublishCrash(t *testing.T) {
	repoRoot, sourceCase, pack, packetPath, decisionPath, workspace, freshRoot, attachedRoot, preview := candidateProvisionFixture(t, "receipt-publish-crash")
	originalHook := candidateVerificationProvisionStageHook
	candidateVerificationProvisionStageHook = func(stage string) error {
		if stage == "before-receipt-publish" {
			return fmt.Errorf("simulated receipt publish crash")
		}
		return nil
	}
	t.Cleanup(func() { candidateVerificationProvisionStageHook = originalHook })
	opt := CandidateVerificationProvisionOptions{PacketPath: packetPath, DecisionPath: decisionPath, FreshCaseRoot: freshRoot, AttachedCaseRoot: attachedRoot, ExpectedProvisionSHA256: preview.ProvisionSHA256}
	if result, err := ProvisionCandidateVerificationCases(repoRoot, sourceCase, pack, opt); err == nil || result.Applied {
		t.Fatalf("receipt publish crash returned success: result=%+v err=%v", result, err)
	}
	if _, err := os.Lstat(filepath.Join(workspace, "provision.receipt.json")); !os.IsNotExist(err) {
		t.Fatalf("receipt was exposed before publication: %v", err)
	}
	tempPath := filepath.Join(workspace, ".provision.receipt."+preview.ProvisionID+".tmp")
	if info, err := os.Stat(tempPath); err != nil || !info.Mode().IsRegular() || info.Size() == 0 {
		t.Fatalf("durable owned receipt temp missing: info=%v err=%v", info, err)
	}
	candidateVerificationProvisionStageHook = nil
	result, err := ProvisionCandidateVerificationCases(repoRoot, sourceCase, pack, opt)
	if err != nil {
		t.Fatal(err)
	}
	if !result.Applied || result.Mode != "provisioned" {
		t.Fatalf("receipt crash resume result: %+v", result)
	}
	if _, err := os.Lstat(tempPath); !os.IsNotExist(err) {
		t.Fatalf("receipt resume left owned temp: %v", err)
	}
}

func TestProvisionCandidateVerificationCasesResumesReceiptCrashAfterPublishBeforeTempRemove(t *testing.T) {
	repoRoot, sourceCase, pack, packetPath, decisionPath, workspace, freshRoot, attachedRoot, preview := candidateProvisionFixture(t, "receipt-after-publish-crash")
	originalHook := candidateVerificationProvisionStageHook
	candidateVerificationProvisionStageHook = func(stage string) error {
		if stage == "after-receipt-publish-before-temp-remove" {
			return fmt.Errorf("simulated receipt after-publish crash")
		}
		return nil
	}
	t.Cleanup(func() { candidateVerificationProvisionStageHook = originalHook })
	opt := CandidateVerificationProvisionOptions{PacketPath: packetPath, DecisionPath: decisionPath, FreshCaseRoot: freshRoot, AttachedCaseRoot: attachedRoot, ExpectedProvisionSHA256: preview.ProvisionSHA256}
	if result, err := ProvisionCandidateVerificationCases(repoRoot, sourceCase, pack, opt); err == nil || result.Applied {
		t.Fatalf("receipt after-publish crash returned success: result=%+v err=%v", result, err)
	}
	tempPath := filepath.Join(workspace, candidateVerificationReceiptTempName(preview.ProvisionID))
	finalPath := filepath.Join(workspace, "provision.receipt.json")
	tempInfo, tempErr := os.Lstat(tempPath)
	finalInfo, finalErr := os.Lstat(finalPath)
	if tempErr != nil || finalErr != nil || !os.SameFile(tempInfo, finalInfo) {
		t.Fatalf("receipt and owned temp do not share identity: temp=%v final=%v", tempErr, finalErr)
	}
	candidateVerificationProvisionStageHook = nil
	result, err := ProvisionCandidateVerificationCases(repoRoot, sourceCase, pack, opt)
	if err != nil {
		t.Fatal(err)
	}
	if !result.Applied || !result.Replay || result.Mode != "already-provisioned" {
		t.Fatalf("receipt crash replay result: %+v", result)
	}
	if _, err := os.Lstat(tempPath); !os.IsNotExist(err) {
		t.Fatalf("receipt replay left owned temp: %v", err)
	}
}

func TestProvisionCandidateVerificationCasesReceiptReplayRejectsDifferentFinalIdentityWithoutDeletingTemp(t *testing.T) {
	repoRoot, sourceCase, pack, packetPath, decisionPath, workspace, freshRoot, attachedRoot, preview := candidateProvisionFixture(t, "receipt-after-publish-different-final")
	originalHook := candidateVerificationProvisionStageHook
	candidateVerificationProvisionStageHook = func(stage string) error {
		if stage == "after-receipt-publish-before-temp-remove" {
			return fmt.Errorf("simulated receipt after-publish crash")
		}
		return nil
	}
	t.Cleanup(func() { candidateVerificationProvisionStageHook = originalHook })
	opt := CandidateVerificationProvisionOptions{PacketPath: packetPath, DecisionPath: decisionPath, FreshCaseRoot: freshRoot, AttachedCaseRoot: attachedRoot, ExpectedProvisionSHA256: preview.ProvisionSHA256}
	if _, err := ProvisionCandidateVerificationCases(repoRoot, sourceCase, pack, opt); err == nil {
		t.Fatal("receipt provisioning unexpectedly survived simulated crash")
	}
	tempPath := filepath.Join(workspace, candidateVerificationReceiptTempName(preview.ProvisionID))
	finalPath := filepath.Join(workspace, "provision.receipt.json")
	finalBytes := readProvisionFile(t, finalPath)
	if err := os.Remove(finalPath); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(finalPath, finalBytes, 0o644); err != nil {
		t.Fatal(err)
	}
	candidateVerificationProvisionStageHook = nil
	if result, err := ProvisionCandidateVerificationCases(repoRoot, sourceCase, pack, opt); err == nil || result.Applied || !strings.Contains(err.Error(), "different identities") {
		t.Fatalf("different final receipt replay result=%+v err=%v", result, err)
	}
	if got := readProvisionFile(t, finalPath); !bytes.Equal(got, finalBytes) {
		t.Fatalf("different final receipt was overwritten: %q", got)
	}
	if got := readProvisionFile(t, tempPath); !bytes.Equal(got, finalBytes) {
		t.Fatalf("owned receipt temp was deleted or changed: %q", got)
	}
	tempInfo, _ := os.Lstat(tempPath)
	finalInfo, _ := os.Lstat(finalPath)
	if os.SameFile(tempInfo, finalInfo) {
		t.Fatal("test did not install a different final receipt identity")
	}
}

func TestProvisionCandidateVerificationCasesReceiptReplayRejectsOwnedTempBytesDrift(t *testing.T) {
	repoRoot, sourceCase, pack, packetPath, decisionPath, workspace, freshRoot, attachedRoot, preview := candidateProvisionFixture(t, "receipt-after-publish-temp-drift")
	originalHook := candidateVerificationProvisionStageHook
	candidateVerificationProvisionStageHook = func(stage string) error {
		if stage == "after-receipt-publish-before-temp-remove" {
			return fmt.Errorf("simulated receipt after-publish crash")
		}
		return nil
	}
	t.Cleanup(func() { candidateVerificationProvisionStageHook = originalHook })
	opt := CandidateVerificationProvisionOptions{PacketPath: packetPath, DecisionPath: decisionPath, FreshCaseRoot: freshRoot, AttachedCaseRoot: attachedRoot, ExpectedProvisionSHA256: preview.ProvisionSHA256}
	if _, err := ProvisionCandidateVerificationCases(repoRoot, sourceCase, pack, opt); err == nil {
		t.Fatal("receipt provisioning unexpectedly survived simulated crash")
	}
	tempPath := filepath.Join(workspace, candidateVerificationReceiptTempName(preview.ProvisionID))
	if err := os.Remove(tempPath); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(tempPath, []byte("drifted receipt temp\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	candidateVerificationProvisionStageHook = nil
	if result, err := ProvisionCandidateVerificationCases(repoRoot, sourceCase, pack, opt); err == nil || result.Applied || !strings.Contains(err.Error(), "different bytes") {
		t.Fatalf("drifted receipt temp replay result=%+v err=%v", result, err)
	}
	if got := readProvisionFile(t, tempPath); string(got) != "drifted receipt temp\n" {
		t.Fatalf("drifted owned receipt temp was deleted or changed: %q", got)
	}
}

func TestProvisionCandidateVerificationCasesReceiptPublicationNeverOverwritesDifferentFinal(t *testing.T) {
	repoRoot, sourceCase, pack, packetPath, decisionPath, workspace, freshRoot, attachedRoot, preview := candidateProvisionFixture(t, "receipt-no-replace")
	originalHook := candidateVerificationProvisionStageHook
	candidateVerificationProvisionStageHook = func(stage string) error {
		if stage != "before-receipt-publish" {
			return nil
		}
		candidateVerificationProvisionStageHook = nil
		return os.WriteFile(filepath.Join(workspace, "provision.receipt.json"), []byte("different\n"), 0o644)
	}
	t.Cleanup(func() { candidateVerificationProvisionStageHook = originalHook })
	result, err := ProvisionCandidateVerificationCases(repoRoot, sourceCase, pack, CandidateVerificationProvisionOptions{PacketPath: packetPath, DecisionPath: decisionPath, FreshCaseRoot: freshRoot, AttachedCaseRoot: attachedRoot, ExpectedProvisionSHA256: preview.ProvisionSHA256})
	if err == nil || result.Applied {
		t.Fatalf("different final receipt returned success: result=%+v err=%v", result, err)
	}
	if got := string(readProvisionFile(t, filepath.Join(workspace, "provision.receipt.json"))); got != "different\n" {
		t.Fatalf("different final receipt was overwritten: %q", got)
	}
}

func TestProvisionCandidateVerificationCasesRejectsIntentDrift(t *testing.T) {
	repoRoot, sourceCase, pack, packetPath, decisionPath, workspace, freshRoot, attachedRoot, preview := candidateProvisionFixture(t, "intent-drift")
	if err := os.MkdirAll(workspace, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(workspace, "provision.intent.json"), []byte("{}\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if _, err := ProvisionCandidateVerificationCases(repoRoot, sourceCase, pack, CandidateVerificationProvisionOptions{PacketPath: packetPath, DecisionPath: decisionPath, FreshCaseRoot: freshRoot, AttachedCaseRoot: attachedRoot, ExpectedProvisionSHA256: preview.ProvisionSHA256}); err == nil {
		t.Fatal("drifted provision intent was accepted")
	}
}

func TestProvisionCandidateVerificationCasesRejectsForgedDecisionReceipt(t *testing.T) {
	repoRoot, sourceCase, pack, packetPath, decisionPath, _, freshRoot, attachedRoot, _ := candidateProvisionFixture(t, "forged-decision")
	packetBytes, _ := os.ReadFile(packetPath)
	decisionBytes, _ := os.ReadFile(decisionPath)
	receiptPath := candidateDecisionReceiptPath(filepath.Join(repoRoot, "packs", pack, "promote-candidates"), sha256Hex(packetBytes), sha256Hex(decisionBytes))
	data := readProvisionFile(t, receiptPath)
	var receipt CandidateDecisionReceipt
	if err := json.Unmarshal(data, &receipt); err != nil {
		t.Fatal(err)
	}
	receipt.VerificationProofPath = filepath.Join(sourceCase, "forged-proof.json")
	writeJSONFile(t, receiptPath, receipt)
	if _, err := ProvisionCandidateVerificationCases(repoRoot, sourceCase, pack, CandidateVerificationProvisionOptions{PacketPath: packetPath, DecisionPath: decisionPath, FreshCaseRoot: freshRoot, AttachedCaseRoot: attachedRoot, WhatIf: true}); err == nil || !strings.Contains(err.Error(), "verification binding") {
		t.Fatalf("forged decision receipt error = %v", err)
	}
}

func TestProvisionCandidateVerificationCasesRejectsReceiptTruncatedAfterClose(t *testing.T) {
	repoRoot, sourceCase, pack, packetPath, decisionPath, _, freshRoot, attachedRoot, preview := candidateProvisionFixture(t, "receipt-truncate")
	originalHook := candidateVerificationProvisionReceiptPostWriteHook
	candidateVerificationProvisionReceiptPostWriteHook = func(path string) error {
		return os.Truncate(path, 0)
	}
	t.Cleanup(func() { candidateVerificationProvisionReceiptPostWriteHook = originalHook })
	result, err := ProvisionCandidateVerificationCases(repoRoot, sourceCase, pack, CandidateVerificationProvisionOptions{PacketPath: packetPath, DecisionPath: decisionPath, FreshCaseRoot: freshRoot, AttachedCaseRoot: attachedRoot, ExpectedProvisionSHA256: preview.ProvisionSHA256})
	if err == nil || result.Applied {
		t.Fatalf("post-close truncated receipt returned success: result=%+v err=%v", result, err)
	}
}

func TestProvisionCandidateVerificationCasesRejectsReceiptReplacedAfterClose(t *testing.T) {
	repoRoot, sourceCase, pack, packetPath, decisionPath, _, freshRoot, attachedRoot, preview := candidateProvisionFixture(t, "receipt-replace")
	originalHook := candidateVerificationProvisionReceiptPostWriteHook
	candidateVerificationProvisionReceiptPostWriteHook = func(path string) error {
		replacement := path + ".replacement"
		if err := os.WriteFile(replacement, readProvisionFile(t, path), 0o644); err != nil {
			return err
		}
		return os.Rename(replacement, path)
	}
	t.Cleanup(func() { candidateVerificationProvisionReceiptPostWriteHook = originalHook })
	result, err := ProvisionCandidateVerificationCases(repoRoot, sourceCase, pack, CandidateVerificationProvisionOptions{PacketPath: packetPath, DecisionPath: decisionPath, FreshCaseRoot: freshRoot, AttachedCaseRoot: attachedRoot, ExpectedProvisionSHA256: preview.ProvisionSHA256})
	if err == nil || result.Applied {
		t.Fatalf("post-close replaced receipt returned success: result=%+v err=%v", result, err)
	}
}

func TestProvisionCandidateVerificationCasesRejectsForgedProvisionReceiptCases(t *testing.T) {
	repoRoot, sourceCase, pack, packetPath, decisionPath, _, freshRoot, attachedRoot, preview := candidateProvisionFixture(t, "forged-provision")
	opt := CandidateVerificationProvisionOptions{PacketPath: packetPath, DecisionPath: decisionPath, FreshCaseRoot: freshRoot, AttachedCaseRoot: attachedRoot, ExpectedProvisionSHA256: preview.ProvisionSHA256}
	applied, err := ProvisionCandidateVerificationCases(repoRoot, sourceCase, pack, opt)
	if err != nil {
		t.Fatal(err)
	}
	var receipt candidateVerificationProvisionReceipt
	if err := json.Unmarshal(readProvisionFile(t, applied.ReceiptPath), &receipt); err != nil {
		t.Fatal(err)
	}
	receipt.Cases[0].Writes = receipt.Cases[0].Writes[:len(receipt.Cases[0].Writes)-1]
	writeJSONFile(t, applied.ReceiptPath, receipt)
	if _, err := ProvisionCandidateVerificationCases(repoRoot, sourceCase, pack, opt); err == nil || !strings.Contains(err.Error(), "different bindings") {
		t.Fatalf("forged provision receipt error = %v", err)
	}
}

func TestProvisionCandidateVerificationCasesRejectsWorkspaceAncestorSymlink(t *testing.T) {
	repoRoot, sourceCase, pack, packetPath, decisionPath, workspace, freshRoot, attachedRoot, preview := candidateProvisionFixture(t, "workspace-link")
	ancestor := filepath.Join(sourceCase, ".rekit", "verifications")
	if err := os.MkdirAll(filepath.Dir(ancestor), 0o755); err != nil {
		t.Fatal(err)
	}
	real := filepath.Join(t.TempDir(), "real")
	if err := os.Mkdir(real, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink(real, ancestor); err != nil {
		t.Skipf("symlink unavailable: %v", err)
	}
	if _, err := ProvisionCandidateVerificationCases(repoRoot, sourceCase, pack, CandidateVerificationProvisionOptions{PacketPath: packetPath, DecisionPath: decisionPath, FreshCaseRoot: freshRoot, AttachedCaseRoot: attachedRoot, ExpectedProvisionSHA256: preview.ProvisionSHA256}); err == nil || !strings.Contains(err.Error(), "non-symlink") {
		t.Fatalf("workspace ancestor symlink error = %v", err)
	}
	if _, err := os.Lstat(filepath.Join(real, filepath.Base(filepath.Dir(workspace)))); err == nil {
		t.Fatal("workspace symlink target was mutated")
	}
}

func TestProvisionCandidateVerificationCasesRejectsRootsAndDrift(t *testing.T) {
	repoRoot, sourceCase, _, pack := packMemoryReconsumeFixture(t)
	if _, err := syncpkg.Apply(repoRoot, sourceCase, pack, syncpkg.ApplyOptions{CreateLocalFiles: true, Command: "init", ProjectName: "source"}); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(sourceCase, filepath.FromSlash("references/template/README.md")), []byte("# README\n\nProvision root candidate.\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	created, packet, managed := candidateDecisionFixture(t, repoRoot, sourceCase, pack, "provision-roots")
	decisionPath := filepath.Join(sourceCase, ".rekit", "reviews", "candidate-provision-roots", "decisions.json")
	writeCandidateDecisionFixture(t, decisionPath, reviewedCandidateDecision(packet, managed, created.ReviewWorkspace.CombinedDiffPath))
	if _, err := ApplyCandidateDecisions(repoRoot, sourceCase, pack, CandidateDecisionOptions{PacketPath: created.ReviewWorkspace.PacketPath, DecisionPath: decisionPath}); err != nil {
		t.Fatal(err)
	}
	packetBytes, _ := os.ReadFile(created.ReviewWorkspace.PacketPath)
	decisionBytes, _ := os.ReadFile(decisionPath)
	workspace := filepath.Join(sourceCase, ".rekit", "verifications", "candidate-decisions", shortHash(sha256Hex(packetBytes)+sha256Hex(decisionBytes)))
	outside := filepath.Join(t.TempDir(), "outside")
	if _, err := ProvisionCandidateVerificationCases(repoRoot, sourceCase, pack, CandidateVerificationProvisionOptions{PacketPath: created.ReviewWorkspace.PacketPath, DecisionPath: decisionPath, FreshCaseRoot: outside, AttachedCaseRoot: filepath.Join(workspace, "attached"), WhatIf: true}); err == nil || !strings.Contains(err.Error(), "direct child") {
		t.Fatalf("outside root error = %v", err)
	}
	same := filepath.Join(workspace, "same")
	if _, err := ProvisionCandidateVerificationCases(repoRoot, sourceCase, pack, CandidateVerificationProvisionOptions{PacketPath: created.ReviewWorkspace.PacketPath, DecisionPath: decisionPath, FreshCaseRoot: same, AttachedCaseRoot: same, WhatIf: true}); err == nil || !strings.Contains(err.Error(), "distinct") {
		t.Fatalf("same root error = %v", err)
	}
	fresh := filepath.Join(workspace, "fresh")
	attached := filepath.Join(workspace, "attached")
	preview, err := ProvisionCandidateVerificationCases(repoRoot, sourceCase, pack, CandidateVerificationProvisionOptions{PacketPath: created.ReviewWorkspace.PacketPath, DecisionPath: decisionPath, FreshCaseRoot: fresh, AttachedCaseRoot: attached, WhatIf: true})
	if err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(fresh, 0o755); err != nil {
		t.Fatal(err)
	}
	if _, err := ProvisionCandidateVerificationCases(repoRoot, sourceCase, pack, CandidateVerificationProvisionOptions{PacketPath: created.ReviewWorkspace.PacketPath, DecisionPath: decisionPath, FreshCaseRoot: fresh, AttachedCaseRoot: attached, ExpectedProvisionSHA256: preview.ProvisionSHA256}); err == nil {
		t.Fatal("existing fresh root was accepted without durable intent")
	}
	if _, err := os.Stat(attached); !os.IsNotExist(err) {
		t.Fatalf("fresh collision created attached root: %v", err)
	}
}

func candidateProvisionFixture(t *testing.T, name string) (repoRoot, sourceCase, pack, packetPath, decisionPath, workspace, freshRoot, attachedRoot string, preview CandidateVerificationProvisionResult) {
	t.Helper()
	var ignored string
	repoRoot, sourceCase, ignored, pack = packMemoryReconsumeFixture(t)
	_ = ignored
	if _, err := syncpkg.Apply(repoRoot, sourceCase, pack, syncpkg.ApplyOptions{CreateLocalFiles: true, Command: "init", ProjectName: "source"}); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(sourceCase, filepath.FromSlash("references/template/README.md")), []byte("# README\n\n"+name+" candidate.\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	created, packet, managed := candidateDecisionFixture(t, repoRoot, sourceCase, pack, name)
	decisionPath = filepath.Join(sourceCase, ".rekit", "reviews", "candidate-"+name, "decisions.json")
	writeCandidateDecisionFixture(t, decisionPath, reviewedCandidateDecision(packet, managed, created.ReviewWorkspace.CombinedDiffPath))
	if _, err := ApplyCandidateDecisions(repoRoot, sourceCase, pack, CandidateDecisionOptions{PacketPath: created.ReviewWorkspace.PacketPath, DecisionPath: decisionPath}); err != nil {
		t.Fatal(err)
	}
	packetPath = created.ReviewWorkspace.PacketPath
	packetBytes, _ := os.ReadFile(packetPath)
	decisionBytes, _ := os.ReadFile(decisionPath)
	var err error
	workspace, err = candidateDecisionVerificationWorkspace(sourceCase, sha256Hex(packetBytes), sha256Hex(decisionBytes))
	if err != nil {
		t.Fatal(err)
	}
	freshRoot = filepath.Join(workspace, "fresh")
	attachedRoot = filepath.Join(workspace, "attached")
	preview, err = ProvisionCandidateVerificationCases(repoRoot, sourceCase, pack, CandidateVerificationProvisionOptions{PacketPath: packetPath, DecisionPath: decisionPath, FreshCaseRoot: freshRoot, AttachedCaseRoot: attachedRoot, WhatIf: true})
	if err != nil {
		t.Fatal(err)
	}
	return repoRoot, sourceCase, pack, packetPath, decisionPath, workspace, freshRoot, attachedRoot, preview
}

func readProvisionFile(t *testing.T, path string) []byte {
	t.Helper()
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	return data
}

func writeJSONFile(t *testing.T, path string, value any) {
	t.Helper()
	data, err := json.MarshalIndent(value, "", "  ")
	if err != nil {
		t.Fatal(err)
	}
	data = append(data, '\n')
	if bytes.Equal(readProvisionFile(t, path), data) {
		return
	}
	if err := os.WriteFile(path, data, 0o644); err != nil {
		t.Fatal(err)
	}
}
