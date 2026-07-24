package promote

import (
	"bytes"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/shuiyu486/re-context-kits/internal/rekit/mission"
	syncpkg "github.com/shuiyu486/re-context-kits/internal/rekit/sync"
)

func TestDraftCandidateDecisionsPreviewsAppliesAndReplaysDecisionFile(t *testing.T) {
	repoRoot, caseRoot, pack := promoteFixture(t)
	created, packet, managed := candidateDecisionFixture(t, repoRoot, caseRoot, pack, "draft")
	decisionPath := filepath.Join(caseRoot, ".rekit", "reviews", "candidate-draft", "decisions.json")

	preview, err := DraftCandidateDecisions(repoRoot, caseRoot, pack, CandidateDecisionDraftOptions{PacketPath: created.ReviewWorkspace.PacketPath, DecisionPath: decisionPath, Decision: "accept-managed-reject-tooling", Reason: "reviewed bounded candidate diff", Actor: "mission-commander", EvidenceRefs: created.ReviewWorkspace.CombinedDiffPath, WhatIf: true})
	if err != nil {
		t.Fatal(err)
	}
	if preview.IsMutation || preview.Applied || preview.DecisionSHA256 == "" || preview.PacketHash == "" || preview.Accepted != 1 || preview.Rejected != 1 || preview.DecisionCount != 2 || len(preview.Decisions) != 2 || preview.ApplyCommand == "" || !strings.Contains(preview.ApplyCommand, "-ExpectedDecisionSha256") {
		t.Fatalf("unexpected candidate decision draft preview: %+v", preview)
	}
	if _, err := os.Stat(decisionPath); !os.IsNotExist(err) {
		t.Fatalf("candidate decision draft WhatIf wrote decision file: %v", err)
	}
	for _, decision := range preview.Decisions {
		if decision.Reason != "reviewed bounded candidate diff" || decision.Actor != "mission-commander" || decision.CandidateHash == "" || len(decision.EvidenceRefs) != 1 || decision.EvidenceRefs[0].SHA256 == "" {
			t.Fatalf("candidate decision draft omitted reviewed bindings: %+v", decision)
		}
		if sameCandidateDecisionPath(decision.CandidatePath, managed.CandidatePath) && (decision.Decision != "accept" || decision.PackTargetHash == "") {
			t.Fatalf("managed draft decision was not accepted with pack target binding: %+v", decision)
		}
	}

	applied, err := DraftCandidateDecisions(repoRoot, caseRoot, pack, CandidateDecisionDraftOptions{PacketPath: created.ReviewWorkspace.PacketPath, DecisionPath: decisionPath, Decision: "accept-managed-reject-tooling", Reason: "reviewed bounded candidate diff", Actor: "mission-commander", EvidenceRefs: created.ReviewWorkspace.CombinedDiffPath, ExpectedDecisionSHA256: preview.DecisionSHA256})
	if err != nil {
		t.Fatal(err)
	}
	if !applied.IsMutation || !applied.Applied || applied.AlreadyWritten || applied.DecisionSHA256 != preview.DecisionSHA256 {
		t.Fatalf("unexpected candidate decision draft apply: %+v", applied)
	}
	data, err := os.ReadFile(decisionPath)
	if err != nil {
		t.Fatal(err)
	}
	var file CandidateDecisionFile
	if err := decodeStrictJSON(data, &file); err != nil {
		t.Fatal(err)
	}
	if file.PacketHash != preview.PacketHash || len(file.Decisions) != preview.DecisionCount {
		t.Fatalf("candidate decision draft file is not packet-bound: %+v", file)
	}
	replay, err := DraftCandidateDecisions(repoRoot, caseRoot, pack, CandidateDecisionDraftOptions{PacketPath: created.ReviewWorkspace.PacketPath, DecisionPath: decisionPath, Decision: "accept-managed-reject-tooling", Reason: "reviewed bounded candidate diff", Actor: "mission-commander", EvidenceRefs: created.ReviewWorkspace.CombinedDiffPath, ExpectedDecisionSHA256: preview.DecisionSHA256})
	if err != nil {
		t.Fatal(err)
	}
	if !replay.Applied || !replay.AlreadyWritten {
		t.Fatalf("candidate decision draft replay was not idempotent: %+v", replay)
	}
	if _, err := ApplyCandidateDecisions(repoRoot, caseRoot, pack, CandidateDecisionOptions{PacketPath: created.ReviewWorkspace.PacketPath, DecisionPath: decisionPath, WhatIf: true}); err != nil {
		t.Fatalf("drafted candidate decision should be accepted by existing decision planner: %v", err)
	}
	if packet.CandidateResult.RepoRoot == "" {
		t.Fatalf("fixture packet omitted repo binding: %+v", packet)
	}
}

func TestDraftCandidateReviewProofPreviewsAppliesAndReplaysProofNote(t *testing.T) {
	repoRoot, caseRoot, pack := promoteFixture(t)
	created, _, managed := candidateDecisionFixture(t, repoRoot, caseRoot, pack, "review-proof")
	proofPath := filepath.Join(repoRoot, "packs", pack, "promote-candidates", "review-artifacts", "review-proof.candidate-decision-note.md")

	preview, err := DraftCandidateReviewProof(repoRoot, caseRoot, pack, CandidateReviewProofDraftOptions{PacketPath: created.ReviewWorkspace.PacketPath, ProofPath: proofPath, ProofType: "candidate-decision-note", CandidatePath: managed.CandidatePath, Decision: "accept", Reason: "reviewed bounded candidate diff", Actor: "mission-commander", EvidenceRefs: created.ReviewWorkspace.CombinedDiffPath, WhatIf: true})
	if err != nil {
		t.Fatal(err)
	}
	if preview.IsMutation || preview.Applied || preview.ProofSHA256 == "" || preview.PacketHash == "" || preview.ProofPath != proofPath || preview.Proof.CandidateHash == "" || preview.Proof.PackTargetHash == "" || preview.Proof.CandidatePath == managed.CandidatePath || !strings.HasPrefix(preview.Proof.CandidatePath, "packs/") || preview.ApplyCommand == "" || !strings.Contains(preview.ApplyCommand, "-ExpectedProofSha256") {
		t.Fatalf("unexpected candidate review proof preview: %+v", preview)
	}
	if _, err := os.Stat(proofPath); !os.IsNotExist(err) {
		t.Fatalf("candidate review proof WhatIf wrote proof file: %v", err)
	}

	applied, err := DraftCandidateReviewProof(repoRoot, caseRoot, pack, CandidateReviewProofDraftOptions{PacketPath: created.ReviewWorkspace.PacketPath, ProofPath: proofPath, ProofType: "candidate-decision-note", CandidatePath: managed.CandidatePath, Decision: "accept", Reason: "reviewed bounded candidate diff", Actor: "mission-commander", EvidenceRefs: created.ReviewWorkspace.CombinedDiffPath, ExpectedProofSHA256: preview.ProofSHA256})
	if err != nil {
		t.Fatal(err)
	}
	if !applied.IsMutation || !applied.Applied || applied.AlreadyWritten || applied.ProofSHA256 != preview.ProofSHA256 {
		t.Fatalf("unexpected candidate review proof apply: %+v", applied)
	}
	data, err := os.ReadFile(proofPath)
	if err != nil {
		t.Fatal(err)
	}
	if bytes.Contains(data, []byte(caseRoot)) || bytes.Contains(data, []byte(repoRoot)) {
		t.Fatalf("proof note should not persist absolute repo/case roots: %s", string(data))
	}
	var note CandidateReviewProofNote
	if err := decodeStrictJSON(data, &note); err != nil {
		t.Fatal(err)
	}
	if note.PacketHash != preview.PacketHash || note.CandidateHash != fileSHA256(managed.CandidatePath) || len(note.EvidenceRefs) != 1 || note.EvidenceRefs[0].SHA256 == "" {
		t.Fatalf("candidate review proof note is not packet/candidate/evidence-bound: %+v", note)
	}
	replay, err := DraftCandidateReviewProof(repoRoot, caseRoot, pack, CandidateReviewProofDraftOptions{PacketPath: created.ReviewWorkspace.PacketPath, ProofPath: proofPath, ProofType: "candidate-decision-note", CandidatePath: managed.CandidatePath, Decision: "accept", Reason: "reviewed bounded candidate diff", Actor: "mission-commander", EvidenceRefs: created.ReviewWorkspace.CombinedDiffPath, ExpectedProofSHA256: preview.ProofSHA256})
	if err != nil {
		t.Fatal(err)
	}
	if !replay.Applied || !replay.AlreadyWritten {
		t.Fatalf("candidate review proof replay was not idempotent: %+v", replay)
	}
}

func TestDraftCandidateReviewProofRejectsUnsafeInputs(t *testing.T) {
	repoRoot, caseRoot, pack := promoteFixture(t)
	created, _, managed := candidateDecisionFixture(t, repoRoot, caseRoot, pack, "review-proof-unsafe")
	proofPath := filepath.Join(repoRoot, "packs", pack, "promote-candidates", "review-artifacts", "unsafe.candidate-decision-note.json")
	base := CandidateReviewProofDraftOptions{PacketPath: created.ReviewWorkspace.PacketPath, ProofPath: proofPath, ProofType: "candidate-decision-note", CandidatePath: managed.CandidatePath, Decision: "reject", Reason: "reviewed bounded candidate diff", Actor: "mission-commander", EvidenceRefs: created.ReviewWorkspace.CombinedDiffPath, WhatIf: true}
	tests := []struct {
		name      string
		mutate    func(*CandidateReviewProofDraftOptions)
		wantError string
	}{
		{
			name: "missing evidence",
			mutate: func(opt *CandidateReviewProofDraftOptions) {
				opt.EvidenceRefs = ""
			},
			wantError: "requires at least one -EvidenceRefs",
		},
		{
			name: "proof escapes repo",
			mutate: func(opt *CandidateReviewProofDraftOptions) {
				opt.ProofPath = filepath.Join(caseRoot, ".rekit", "proof.json")
			},
			wantError: "must stay under pack promote-candidates/review-artifacts",
		},
		{
			name: "candidate missing",
			mutate: func(opt *CandidateReviewProofDraftOptions) {
				if err := os.Remove(managed.CandidatePath); err != nil {
					t.Fatal(err)
				}
			},
			wantError: "must be a non-empty regular file",
		},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			if tc.name == "candidate missing" {
				repoRoot, caseRoot, pack = promoteFixture(t)
				created, _, managed = candidateDecisionFixture(t, repoRoot, caseRoot, pack, "review-proof-unsafe-missing")
				base = CandidateReviewProofDraftOptions{PacketPath: created.ReviewWorkspace.PacketPath, ProofPath: filepath.Join(repoRoot, "packs", pack, "promote-candidates", "review-artifacts", "unsafe-missing.candidate-decision-note.json"), ProofType: "candidate-decision-note", CandidatePath: managed.CandidatePath, Decision: "reject", Reason: "reviewed bounded candidate diff", Actor: "mission-commander", EvidenceRefs: created.ReviewWorkspace.CombinedDiffPath, WhatIf: true}
			}
			opt := base
			tc.mutate(&opt)
			_, err := DraftCandidateReviewProof(repoRoot, caseRoot, pack, opt)
			if err == nil || !strings.Contains(err.Error(), tc.wantError) {
				t.Fatalf("error = %v, want %q", err, tc.wantError)
			}
		})
	}
}

func TestCandidateReviewWorkspaceSurfacesUsableDecisionDraftHandoff(t *testing.T) {
	repoRoot, caseRoot, pack := promoteFixture(t)
	created, packet, _ := candidateDecisionFixture(t, repoRoot, caseRoot, pack, "draft-handoff")
	handoff := created.ReviewPlan.DecisionDraftHandoff
	if handoff == nil || handoff.PacketPath != created.ReviewWorkspace.PacketPath || handoff.DecisionPath == "" || len(handoff.EvidenceRefs) == 0 || len(handoff.PreviewCommands) == 0 || !strings.Contains(handoff.PreviewCommands[0].PreviewCommand, "-DraftCandidateDecision") || !strings.Contains(handoff.PreviewCommands[0].ApplyCommandTemplate, "<decisionSha256-from-WhatIf>") {
		t.Fatalf("candidate review workspace omitted decision draft handoff: %+v", handoff)
	}
	if packet.CandidateResult.ReviewPlan.DecisionDraftHandoff == nil || packet.CandidateResult.ReviewPlan.DecisionDraftHandoff.NextAction == "" {
		t.Fatalf("candidate review packet omitted durable decision draft handoff: %+v", packet.CandidateResult.ReviewPlan.DecisionDraftHandoff)
	}
	if created.ReviewPlan.MissionCommanderActionQueue.CurrentAction == nil || created.ReviewPlan.MissionCommanderActionQueue.CurrentAction.Source != "reviewPlan.decisionChecklist" || !candidateDecisionDraftNextActionForTest(created.ReviewPlan.MissionCommanderNextActions) {
		t.Fatalf("candidate review handoff should keep review first and queue draft next: %+v", created.ReviewPlan.MissionCommanderActionQueue)
	}
	preview, err := DraftCandidateDecisions(repoRoot, caseRoot, pack, CandidateDecisionDraftOptions{PacketPath: handoff.PacketPath, DecisionPath: handoff.DecisionPath, Decision: handoff.SupportedDecisions[0], Reason: handoff.DefaultReason, Actor: handoff.DefaultActor, EvidenceRefs: strings.Join(handoff.EvidenceRefs, ","), WhatIf: true})
	if err != nil {
		t.Fatal(err)
	}
	if preview.DecisionSHA256 == "" || preview.DecisionPath != handoff.DecisionPath || preview.DecisionCount != 2 || preview.Accepted != 1 || preview.Rejected != 1 {
		t.Fatalf("decision draft handoff did not produce usable preview: %+v", preview)
	}
	if _, err := os.Stat(handoff.DecisionPath); !os.IsNotExist(err) {
		t.Fatalf("decision draft handoff preview wrote decision file: %v", err)
	}
}

func candidateDecisionDraftNextActionForTest(items []mission.MissionCommanderNextActionItem) bool {
	for _, item := range items {
		if item.Source == "reviewPlan.decisionDraftHandoff" && strings.Contains(item.Command, "-DraftCandidateDecision") {
			return true
		}
	}
	return false
}

func TestDraftCandidateDecisionsRejectsUnsafeInputs(t *testing.T) {
	repoRoot, caseRoot, pack := promoteFixture(t)
	created, err := CreateCandidates(repoRoot, caseRoot, pack, CandidateOptions{})
	if err != nil {
		t.Fatal(err)
	}
	created, err = WriteCandidateReviewWorkspace(created, CandidateArtifactOptions{ReviewOutputDir: filepath.Join(caseRoot, ".rekit", "reviews", "candidate-draft-unsafe")})
	if err != nil {
		t.Fatal(err)
	}
	decisionPath := filepath.Join(caseRoot, ".rekit", "reviews", "candidate-draft-unsafe", "decisions.json")
	base := CandidateDecisionDraftOptions{PacketPath: created.ReviewWorkspace.PacketPath, DecisionPath: decisionPath, Decision: "accept-managed-reject-tooling", Reason: "reviewed bounded candidate diff", Actor: "mission-commander", EvidenceRefs: created.ReviewWorkspace.CombinedDiffPath, WhatIf: true}
	tests := []struct {
		name      string
		mutate    func(*CandidateDecisionDraftOptions)
		wantError string
	}{
		{
			name: "accept tooling fail closed",
			mutate: func(opt *CandidateDecisionDraftOptions) {
				opt.Decision = "accept"
			},
			wantError: "cannot be accepted automatically",
		},
		{
			name: "missing evidence",
			mutate: func(opt *CandidateDecisionDraftOptions) {
				opt.EvidenceRefs = ""
			},
			wantError: "requires -EvidenceRefs",
		},
		{
			name: "decision path escapes case",
			mutate: func(opt *CandidateDecisionDraftOptions) {
				opt.DecisionPath = filepath.Join(repoRoot, "outside-decisions.json")
			},
			wantError: "must stay under the attached case",
		},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			opt := base
			tc.mutate(&opt)
			_, err := DraftCandidateDecisions(repoRoot, caseRoot, pack, opt)
			if err == nil || !strings.Contains(err.Error(), tc.wantError) {
				t.Fatalf("error = %v, want %q", err, tc.wantError)
			}
		})
	}
}

func TestApplyCandidateDecisionsPreviewsAndAppliesReviewedManagedCandidate(t *testing.T) {
	repoRoot, caseRoot, pack := promoteFixture(t)
	created, err := CreateCandidates(repoRoot, caseRoot, pack, CandidateOptions{})
	if err != nil {
		t.Fatal(err)
	}
	workspaceRoot := filepath.Join(caseRoot, ".rekit", "reviews", "candidate-decision")
	created, err = WriteCandidateReviewWorkspace(created, CandidateArtifactOptions{ReviewOutputDir: workspaceRoot})
	if err != nil {
		t.Fatal(err)
	}
	packetBytes, err := os.ReadFile(created.ReviewWorkspace.PacketPath)
	if err != nil {
		t.Fatal(err)
	}
	var packet CandidateReviewPacket
	if err := json.Unmarshal(packetBytes, &packet); err != nil {
		t.Fatal(err)
	}
	var managed CandidateReviewItem
	for _, item := range packet.CandidateResult.ReviewPlan.ReviewItems {
		if item.Kind == "managed-doc" && item.ReviewDecision == "pending-review" {
			managed = item
			break
		}
	}
	if managed.CandidatePath == "" {
		t.Fatalf("review packet omitted pending managed candidate: %+v", packet.CandidateResult.ReviewPlan.ReviewItems)
	}
	decisionPath := filepath.Join(workspaceRoot, "decisions.json")
	decision := reviewedCandidateDecision(packet, managed, created.ReviewWorkspace.CombinedDiffPath)
	writeCandidateDecisionFixture(t, decisionPath, decision)
	originalPack, err := os.ReadFile(managed.PackTarget)
	if err != nil {
		t.Fatal(err)
	}

	preview, err := ApplyCandidateDecisions(repoRoot, caseRoot, pack, CandidateDecisionOptions{PacketPath: created.ReviewWorkspace.PacketPath, DecisionPath: decisionPath, WhatIf: true})
	if err != nil {
		t.Fatal(err)
	}
	if preview.IsMutation || preview.Applied || preview.Accepted != 1 || preview.Rejected != 1 || len(preview.Actions) != 2 || preview.Actions[0].Action != "merge-accepted-candidate-and-cleanup" {
		t.Fatalf("unexpected candidate decision preview: %+v", preview)
	}
	packAfterPreview, err := os.ReadFile(managed.PackTarget)
	if err != nil {
		t.Fatal(err)
	}
	if string(packAfterPreview) != string(originalPack) {
		t.Fatal("candidate decision WhatIf changed pack target")
	}

	applied, err := ApplyCandidateDecisions(repoRoot, caseRoot, pack, CandidateDecisionOptions{PacketPath: created.ReviewWorkspace.PacketPath, DecisionPath: decisionPath})
	if err != nil {
		t.Fatal(err)
	}
	if !applied.IsMutation || !applied.Applied || applied.Accepted != 1 || applied.BackupRoot == "" || applied.Actions[0].CandidateBackupPath == "" || applied.Actions[0].TargetBackupPath == "" {
		t.Fatalf("unexpected candidate decision apply: %+v", applied)
	}
	targetBytes, err := os.ReadFile(applied.Actions[0].TargetBackupPath)
	if err != nil {
		t.Fatal(err)
	}
	if string(targetBytes) != string(originalPack) {
		t.Fatalf("target backup mismatch: %q", string(targetBytes))
	}
	candidateBytes, err := os.ReadFile(applied.Actions[0].CandidateBackupPath)
	if err != nil {
		t.Fatal(err)
	}
	if string(candidateBytes) != "# README\n\nReusable package-test candidate.\n" {
		t.Fatalf("candidate backup mismatch: %q", string(candidateBytes))
	}
	packBytes, err := os.ReadFile(managed.PackTarget)
	if err != nil {
		t.Fatal(err)
	}
	if string(packBytes) != "# README\n\nReusable package-test candidate.\n" {
		t.Fatalf("accepted candidate was not merged: %q", string(packBytes))
	}
	if _, err := os.Stat(managed.CandidatePath); !os.IsNotExist(err) {
		t.Fatalf("accepted candidate was not cleaned up: %v", err)
	}
}

func TestVerifyCandidateDecisionPreviewsAppliesAndReplays(t *testing.T) {
	repoRoot, sourceCase, freshCase, pack := packMemoryReconsumeFixture(t)
	attachedCase := filepath.Join(t.TempDir(), "attachedcase")
	if _, err := syncpkg.Apply(repoRoot, sourceCase, pack, syncpkg.ApplyOptions{CreateLocalFiles: true, Command: "init", ProjectName: "source"}); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(sourceCase, filepath.FromSlash("references/template/README.md")), []byte("# README\n\nReviewed reusable candidate.\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	created, packet, managed := candidateDecisionFixture(t, repoRoot, sourceCase, pack, "verification")
	decision := reviewedCandidateDecision(packet, managed, created.ReviewWorkspace.CombinedDiffPath)
	decisionPath := filepath.Join(sourceCase, ".rekit", "reviews", "candidate-verification", "decisions.json")
	writeCandidateDecisionFixture(t, decisionPath, decision)
	applied, err := ApplyCandidateDecisions(repoRoot, sourceCase, pack, CandidateDecisionOptions{PacketPath: created.ReviewWorkspace.PacketPath, DecisionPath: decisionPath})
	if err != nil {
		t.Fatal(err)
	}
	if applied.Receipt == nil || applied.ReceiptPath == "" || !applied.Receipt.VerificationPending || !strings.Contains(applied.Receipt.VerificationProvisionCommand, "-ProvisionCandidateVerificationCases") {
		t.Fatalf("candidate decision receipt omitted verification handoff: %+v", applied)
	}
	if _, err := syncpkg.Apply(repoRoot, freshCase, pack, syncpkg.ApplyOptions{CreateLocalFiles: true, Command: "init", ProjectName: "fresh"}); err != nil {
		t.Fatal(err)
	}
	if _, err := syncpkg.Apply(repoRoot, attachedCase, pack, syncpkg.ApplyOptions{CreateLocalFiles: true, Command: "init", ProjectName: "attached"}); err != nil {
		t.Fatal(err)
	}
	preview, err := VerifyCandidateDecision(repoRoot, sourceCase, pack, CandidateDecisionVerificationOptions{PacketPath: created.ReviewWorkspace.PacketPath, DecisionPath: decisionPath, FreshCaseRoot: freshCase, AttachedCaseRoot: attachedCase, WhatIf: true})
	if err != nil {
		t.Fatal(err)
	}
	if preview.IsMutation || preview.Applied || !preview.Ready || preview.PackDoctorRows == 0 || preview.FreshDoctorRows == 0 || preview.AttachedDoctorRows == 0 {
		t.Fatalf("unexpected candidate verification preview: %+v", preview)
	}
	if _, err := os.Stat(preview.VerificationProofPath); !os.IsNotExist(err) {
		t.Fatalf("candidate verification WhatIf wrote proof: %v", err)
	}
	verified, err := VerifyCandidateDecision(repoRoot, sourceCase, pack, CandidateDecisionVerificationOptions{PacketPath: created.ReviewWorkspace.PacketPath, DecisionPath: decisionPath, FreshCaseRoot: freshCase, AttachedCaseRoot: attachedCase})
	if err != nil {
		t.Fatal(err)
	}
	if !verified.IsMutation || !verified.Applied || !verified.Ready {
		t.Fatalf("unexpected candidate verification apply: %+v", verified)
	}
	proofData, err := os.ReadFile(verified.VerificationProofPath)
	if err != nil {
		t.Fatal(err)
	}
	var proof CandidateDecisionVerificationResult
	if err := decodeStrictJSON(proofData, &proof); err != nil || proof.PacketHash != verified.PacketHash || proof.DecisionHash != verified.DecisionHash || !sameCandidateDecisionPath(proof.ReceiptPath, applied.ReceiptPath) || len(proof.VerifiedActions) != len(applied.Actions) {
		t.Fatalf("candidate verification proof is not durably bound to receipt/actions: proof=%+v err=%v", proof, err)
	}
	if replay, err := VerifyCandidateDecision(repoRoot, sourceCase, pack, CandidateDecisionVerificationOptions{PacketPath: created.ReviewWorkspace.PacketPath, DecisionPath: decisionPath, FreshCaseRoot: freshCase, AttachedCaseRoot: attachedCase}); err != nil || !replay.Applied {
		t.Fatalf("candidate verification replay failed: result=%+v err=%v", replay, err)
	}
	if err := os.WriteFile(verified.VerificationProofPath, []byte("{\"ready\":true}\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if _, err := VerifyCandidateDecision(repoRoot, sourceCase, pack, CandidateDecisionVerificationOptions{PacketPath: created.ReviewWorkspace.PacketPath, DecisionPath: decisionPath, FreshCaseRoot: freshCase, AttachedCaseRoot: attachedCase}); err == nil {
		t.Fatal("candidate verification replay should reject a mismatched existing proof")
	}
}

func TestVerifyCandidateDecisionClosesMixedManagedAcceptAndToolingReject(t *testing.T) {
	repoRoot, sourceCase, freshCase, pack := packMemoryReconsumeFixture(t)
	attachedCase := filepath.Join(t.TempDir(), "attachedcase")
	if _, err := syncpkg.Apply(repoRoot, sourceCase, pack, syncpkg.ApplyOptions{CreateLocalFiles: true, Command: "init", ProjectName: "source"}); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(sourceCase, filepath.FromSlash("references/template/README.md")), []byte("# README\n\nReviewed reusable candidate.\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	created, err := CreateCandidates(repoRoot, sourceCase, pack, CandidateOptions{})
	if err != nil {
		t.Fatal(err)
	}
	created, err = WriteCandidateReviewWorkspace(created, CandidateArtifactOptions{ReviewOutputDir: filepath.Join(sourceCase, ".rekit", "reviews", "candidate-mixed-decision")})
	if err != nil {
		t.Fatal(err)
	}
	packetBytes, err := os.ReadFile(created.ReviewWorkspace.PacketPath)
	if err != nil {
		t.Fatal(err)
	}
	var packet CandidateReviewPacket
	if err := decodeStrictJSON(packetBytes, &packet); err != nil {
		t.Fatal(err)
	}
	var managed, tooling CandidateReviewItem
	for _, item := range packet.CandidateResult.ReviewPlan.ReviewItems {
		if item.ReviewDecision != "pending-review" {
			continue
		}
		switch item.Kind {
		case "managed-doc":
			if item.Path == "references/template/README.md" {
				managed = item
			}
		case "tooling-candidate-source":
			tooling = item
		}
	}
	if managed.CandidatePath == "" || tooling.CandidatePath == "" {
		t.Fatalf("mixed decision fixture omitted candidates: managed=%+v tooling=%+v", managed, tooling)
	}
	evidence := CandidateDecisionEvidence{Path: created.ReviewWorkspace.CombinedDiffPath, SHA256: fileSHA256(created.ReviewWorkspace.CombinedDiffPath)}
	decision := CandidateDecisionFile{
		SchemaVersion: 1,
		Kind:          "pack-memory-candidate-decisions",
		PacketHash:    sha256Hex(packetBytes),
		Decisions: []CandidateDecisionItem{
			{CandidatePath: managed.CandidatePath, Decision: "accept", CandidateHash: fileSHA256(managed.CandidatePath), PackTargetHash: fileSHA256(managed.PackTarget), Reason: "reviewed reusable managed content", Actor: "mission-commander", EvidenceRefs: []CandidateDecisionEvidence{evidence}},
			{CandidatePath: tooling.CandidatePath, Decision: "reject", CandidateHash: fileSHA256(tooling.CandidatePath), Reason: "tooling observation is not reusable", Actor: "mission-commander", EvidenceRefs: []CandidateDecisionEvidence{evidence}},
		},
	}
	decisionPath := filepath.Join(sourceCase, ".rekit", "reviews", "candidate-mixed-decision", "decisions.json")
	writeCandidateDecisionFixture(t, decisionPath, decision)
	preview, err := ApplyCandidateDecisions(repoRoot, sourceCase, pack, CandidateDecisionOptions{PacketPath: created.ReviewWorkspace.PacketPath, DecisionPath: decisionPath, WhatIf: true})
	if err != nil {
		t.Fatal(err)
	}
	if preview.Accepted != 1 || preview.Rejected != 1 || len(preview.Actions) != 2 {
		t.Fatalf("mixed decision preview drifted: %+v", preview)
	}
	applied, err := ApplyCandidateDecisions(repoRoot, sourceCase, pack, CandidateDecisionOptions{PacketPath: created.ReviewWorkspace.PacketPath, DecisionPath: decisionPath})
	if err != nil {
		t.Fatal(err)
	}
	if applied.Receipt == nil || applied.Accepted != 1 || applied.Rejected != 1 || len(applied.Actions) != 2 || !applied.Receipt.VerificationPending {
		t.Fatalf("mixed decision apply omitted durable verification handoff: %+v", applied)
	}
	for _, action := range applied.Actions {
		if _, err := os.Lstat(action.CandidatePath); !os.IsNotExist(err) {
			t.Fatalf("mixed decision candidate was not cleaned up: action=%+v err=%v", action, err)
		}
	}
	if _, err := syncpkg.Apply(repoRoot, freshCase, pack, syncpkg.ApplyOptions{CreateLocalFiles: true, Command: "init", ProjectName: "fresh"}); err != nil {
		t.Fatal(err)
	}
	if _, err := syncpkg.Apply(repoRoot, attachedCase, pack, syncpkg.ApplyOptions{CreateLocalFiles: true, Command: "init", ProjectName: "attached"}); err != nil {
		t.Fatal(err)
	}
	verified, err := VerifyCandidateDecision(repoRoot, sourceCase, pack, CandidateDecisionVerificationOptions{PacketPath: created.ReviewWorkspace.PacketPath, DecisionPath: decisionPath, FreshCaseRoot: freshCase, AttachedCaseRoot: attachedCase})
	if err != nil {
		t.Fatal(err)
	}
	if !verified.Applied || !verified.Ready || len(verified.VerifiedActions) != 2 {
		t.Fatalf("mixed decision verification did not close: %+v", verified)
	}
	kinds := map[string]string{}
	for _, action := range verified.VerifiedActions {
		kinds[action.Kind] = action.Decision
	}
	if kinds["managed-doc"] != "accept" || kinds["tooling-candidate-source"] != "reject" {
		t.Fatalf("mixed verification action binding drifted: %+v", verified.VerifiedActions)
	}

	forgedActions := []CandidateDecisionAction{applied.Actions[0], applied.Actions[0]}
	receipt := *applied.Receipt
	receipt.Accepted = 2
	receipt.Rejected = 0
	receipt.Actions = forgedActions
	transactionPath := filepath.Join(applied.BackupRoot, "transaction.json")
	var transaction candidateDecisionTransaction
	if err := decodeStrictJSON(readCandidateDecisionTestFile(t, transactionPath), &transaction); err != nil {
		t.Fatal(err)
	}
	transaction.Actions = forgedActions
	transaction.Result.Accepted = 2
	transaction.Result.Rejected = 0
	transaction.Result.Actions = forgedActions
	writeCandidateDecisionTestJSON(t, transactionPath, transaction)
	committedPath := filepath.Join(applied.BackupRoot, "committed.json")
	var committed CandidateDecisionResult
	if err := decodeStrictJSON(readCandidateDecisionTestFile(t, committedPath), &committed); err != nil {
		t.Fatal(err)
	}
	committed.Accepted = 2
	committed.Rejected = 0
	committed.Actions = forgedActions
	committed.Receipt = &receipt
	writeCandidateDecisionTestJSON(t, committedPath, committed)
	writeCandidateDecisionTestJSON(t, applied.ReceiptPath, receipt)
	if _, err := loadCandidateDecisionAuthority(repoRoot, sourceCase, pack, created.ReviewWorkspace.PacketPath, decisionPath); err == nil || !strings.Contains(err.Error(), "duplicate candidate decision receipt action") {
		t.Fatalf("forged duplicate-one/omit-another receipt+transaction+committed error = %v", err)
	}
}

func TestApplyCandidateDecisionsClosesToolingOnlyReject(t *testing.T) {
	repoRoot, sourceCase, _, pack := packMemoryReconsumeFixture(t)
	if _, err := syncpkg.Apply(repoRoot, sourceCase, pack, syncpkg.ApplyOptions{CreateLocalFiles: true, Command: "init", ProjectName: "source"}); err != nil {
		t.Fatal(err)
	}
	created, err := CreateCandidates(repoRoot, sourceCase, pack, CandidateOptions{})
	if err != nil {
		t.Fatal(err)
	}
	created, err = WriteCandidateReviewWorkspace(created, CandidateArtifactOptions{ReviewOutputDir: filepath.Join(sourceCase, ".rekit", "reviews", "candidate-tooling-reject")})
	if err != nil {
		t.Fatal(err)
	}
	packetBytes, err := os.ReadFile(created.ReviewWorkspace.PacketPath)
	if err != nil {
		t.Fatal(err)
	}
	var packet CandidateReviewPacket
	if err := decodeStrictJSON(packetBytes, &packet); err != nil {
		t.Fatal(err)
	}
	var tooling CandidateReviewItem
	for _, item := range packet.CandidateResult.ReviewPlan.ReviewItems {
		if item.Kind == "tooling-candidate-source" && item.ReviewDecision == "pending-review" {
			tooling = item
			break
		}
	}
	if tooling.CandidatePath == "" {
		t.Fatal("tooling-only decision fixture omitted tooling candidate")
	}
	evidencePath := ""
	for _, item := range packet.ReviewInput.ToolingItems {
		if item.Kind == tooling.Kind && item.Path == tooling.Path {
			evidencePath = item.SanitizedPreviewPath
			break
		}
	}
	if evidencePath == "" {
		t.Fatal("tooling-only decision fixture omitted sanitized preview evidence")
	}
	decision := CandidateDecisionFile{
		SchemaVersion: 1,
		Kind:          "pack-memory-candidate-decisions",
		PacketHash:    sha256Hex(packetBytes),
		Decisions: []CandidateDecisionItem{{
			CandidatePath: tooling.CandidatePath,
			Decision:      "reject",
			CandidateHash: fileSHA256(tooling.CandidatePath),
			Reason:        "reviewed tooling observation is not reusable",
			Actor:         "mission-commander",
			EvidenceRefs:  []CandidateDecisionEvidence{{Path: evidencePath, SHA256: fileSHA256(evidencePath)}},
		}},
	}
	decisionPath := filepath.Join(sourceCase, ".rekit", "reviews", "candidate-tooling-reject", "decisions.json")
	writeCandidateDecisionFixture(t, decisionPath, decision)
	canonicalCandidateRoot := filepath.Join(repoRoot, "packs", pack, "promote-candidates")
	preview, err := ApplyCandidateDecisions(repoRoot, sourceCase, pack, CandidateDecisionOptions{PacketPath: created.ReviewWorkspace.PacketPath, DecisionPath: decisionPath, WhatIf: true})
	if err != nil {
		t.Fatal(err)
	}
	if preview.IsMutation || preview.Applied || preview.Rejected != 1 || len(preview.Actions) != 1 {
		t.Fatalf("tooling-only reject preview drifted: %+v", preview)
	}
	if _, err := os.Lstat(canonicalCandidateRoot); !os.IsNotExist(err) {
		t.Fatalf("tooling-only WhatIf created candidate root: %v", err)
	}
	applied, err := ApplyCandidateDecisions(repoRoot, sourceCase, pack, CandidateDecisionOptions{PacketPath: created.ReviewWorkspace.PacketPath, DecisionPath: decisionPath})
	if err != nil {
		t.Fatal(err)
	}
	if applied.Accepted != 0 || applied.Rejected != 1 || applied.Receipt == nil || applied.Receipt.VerificationPending || applied.Receipt.VerificationProofPath != "" || len(applied.Actions) != 1 || applied.Actions[0].Kind != "tooling-candidate-source" {
		t.Fatalf("tooling-only reject receipt drifted: %+v", applied)
	}
	if _, err := os.Lstat(tooling.CandidatePath); !os.IsNotExist(err) {
		t.Fatalf("tooling reject did not clean candidate: %v", err)
	}
}

func TestVerifyCandidateDecisionRejectsDriftAndInvalidRoots(t *testing.T) {
	repoRoot, sourceCase, freshCase, pack := packMemoryReconsumeFixture(t)
	attachedCase := filepath.Join(t.TempDir(), "attachedcase")
	if _, err := syncpkg.Apply(repoRoot, sourceCase, pack, syncpkg.ApplyOptions{CreateLocalFiles: true, Command: "init", ProjectName: "source"}); err != nil {
		t.Fatal(err)
	}
	staleFreshCase := filepath.Join(t.TempDir(), "stale-fresh-case")
	if _, err := syncpkg.Apply(repoRoot, staleFreshCase, pack, syncpkg.ApplyOptions{CreateLocalFiles: true, Command: "init", ProjectName: "stale-fresh"}); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(sourceCase, filepath.FromSlash("references/template/README.md")), []byte("# README\n\nReviewed reusable candidate.\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	created, packet, managed := candidateDecisionFixture(t, repoRoot, sourceCase, pack, "verification-drift")
	decision := reviewedCandidateDecision(packet, managed, created.ReviewWorkspace.CombinedDiffPath)
	decisionPath := filepath.Join(sourceCase, ".rekit", "reviews", "candidate-verification-drift", "decisions.json")
	writeCandidateDecisionFixture(t, decisionPath, decision)
	applied, err := ApplyCandidateDecisions(repoRoot, sourceCase, pack, CandidateDecisionOptions{PacketPath: created.ReviewWorkspace.PacketPath, DecisionPath: decisionPath})
	if err != nil {
		t.Fatal(err)
	}
	if _, err := syncpkg.Apply(repoRoot, freshCase, pack, syncpkg.ApplyOptions{CreateLocalFiles: true, Command: "init", ProjectName: "fresh"}); err != nil {
		t.Fatal(err)
	}
	if _, err := syncpkg.Apply(repoRoot, attachedCase, pack, syncpkg.ApplyOptions{CreateLocalFiles: true, Command: "init", ProjectName: "attached"}); err != nil {
		t.Fatal(err)
	}
	base := CandidateDecisionVerificationOptions{PacketPath: created.ReviewWorkspace.PacketPath, DecisionPath: decisionPath, FreshCaseRoot: freshCase, AttachedCaseRoot: attachedCase, WhatIf: true}
	missing := base
	missing.FreshCaseRoot = ""
	if _, err := VerifyCandidateDecision(repoRoot, sourceCase, pack, missing); err == nil {
		t.Fatal("candidate verification should require fresh case root")
	}
	same := base
	same.AttachedCaseRoot = freshCase
	if _, err := VerifyCandidateDecision(repoRoot, sourceCase, pack, same); err == nil {
		t.Fatal("candidate verification should reject identical fresh/attached roots")
	}
	sourceAttached := base
	sourceAttached.AttachedCaseRoot = sourceCase
	if _, err := VerifyCandidateDecision(repoRoot, sourceCase, pack, sourceAttached); err == nil {
		t.Fatal("candidate verification should reject source case as attached root")
	}
	staleFresh := base
	staleFresh.FreshCaseRoot = staleFreshCase
	if _, err := VerifyCandidateDecision(repoRoot, sourceCase, pack, staleFresh); err == nil {
		t.Fatal("candidate verification should reject a fresh case initialized before acceptance")
	}
	staleAttached := base
	staleAttached.AttachedCaseRoot = staleFreshCase
	if _, err := VerifyCandidateDecision(repoRoot, sourceCase, pack, staleAttached); err == nil {
		t.Fatal("candidate verification should reject an attached case initialized before acceptance")
	}
	if err := os.WriteFile(managed.PackTarget, []byte("drifted target\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if _, err := VerifyCandidateDecision(repoRoot, sourceCase, pack, base); err == nil {
		t.Fatal("candidate verification should reject accepted target drift")
	}
	candidateBytes, err := os.ReadFile(applied.Actions[0].CandidateBackupPath)
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(managed.PackTarget, candidateBytes, 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(managed.CandidatePath, []byte("reappeared candidate\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if _, err := VerifyCandidateDecision(repoRoot, sourceCase, pack, base); err == nil {
		t.Fatal("candidate verification should reject reappeared candidate")
	}
	if err := os.Remove(managed.CandidatePath); err != nil {
		t.Fatal(err)
	}
	indexData := fmt.Appendf(nil, "[{\"path\":%q,\"kind\":%q,\"candidate\":%q}]\n", managed.Path, managed.Kind, managed.CandidatePath)
	if err := os.WriteFile(applied.IndexPath, indexData, 0o644); err != nil {
		t.Fatal(err)
	}
	if _, err := VerifyCandidateDecision(repoRoot, sourceCase, pack, base); err == nil {
		t.Fatal("candidate verification should reject reappeared candidate index entry")
	}
	if err := os.Remove(applied.IndexPath); err != nil {
		t.Fatal(err)
	}
	originalBackup, err := os.ReadFile(applied.Actions[0].CandidateBackupPath)
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(applied.Actions[0].CandidateBackupPath, []byte("tampered reviewed backup\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if _, err := VerifyCandidateDecision(repoRoot, sourceCase, pack, base); err == nil {
		t.Fatal("candidate verification should reject a tampered reviewed candidate backup")
	}
	if err := os.WriteFile(applied.Actions[0].CandidateBackupPath, originalBackup, 0o644); err != nil {
		t.Fatal(err)
	}
}

func TestApplyCandidateDecisionsRejectsIncompleteOrDuplicateReviewAuthorityBeforeMutation(t *testing.T) {
	tests := []struct {
		name           string
		mutatePacket   func(*CandidateReviewPacket, CandidateReviewItem, CandidateReviewItem)
		mutateDecision func(*CandidateDecisionFile)
		wantError      string
	}{
		{
			name: "empty pending review candidate path",
			mutatePacket: func(packet *CandidateReviewPacket, managed, _ CandidateReviewItem) {
				for idx := range packet.CandidateResult.ReviewPlan.ReviewItems {
					if packet.CandidateResult.ReviewPlan.ReviewItems[idx].CandidatePath == managed.CandidatePath {
						packet.CandidateResult.ReviewPlan.ReviewItems[idx].CandidatePath = ""
						return
					}
				}
			},
			wantError: "malformed candidate review: pending-review has invalid candidatePath",
		},
		{
			name: "blank pending review candidate path",
			mutatePacket: func(packet *CandidateReviewPacket, managed, _ CandidateReviewItem) {
				for idx := range packet.CandidateResult.ReviewPlan.ReviewItems {
					if packet.CandidateResult.ReviewPlan.ReviewItems[idx].CandidatePath == managed.CandidatePath {
						packet.CandidateResult.ReviewPlan.ReviewItems[idx].CandidatePath = " \t "
						return
					}
				}
			},
			wantError: "malformed candidate review: pending-review has invalid candidatePath",
		},
		{
			name: "normalized duplicate review",
			mutatePacket: func(packet *CandidateReviewPacket, managed, _ CandidateReviewItem) {
				duplicate := managed
				duplicate.CandidatePath = filepath.Dir(managed.CandidatePath) + string(filepath.Separator) + "." + string(filepath.Separator) + filepath.Base(managed.CandidatePath)
				packet.CandidateResult.ReviewPlan.ReviewItems = append(packet.CandidateResult.ReviewPlan.ReviewItems, duplicate)
			},
			wantError: "duplicate candidate review",
		},
		{
			name:         "normalized duplicate decision",
			mutatePacket: func(_ *CandidateReviewPacket, _, _ CandidateReviewItem) {},
			mutateDecision: func(decision *CandidateDecisionFile) {
				duplicate := decision.Decisions[0]
				duplicate.CandidatePath = filepath.Dir(duplicate.CandidatePath) + string(filepath.Separator) + "." + string(filepath.Separator) + filepath.Base(duplicate.CandidatePath)
				decision.Decisions = append(decision.Decisions, duplicate)
			},
			wantError: "duplicate candidate decision",
		},
		{
			name:         "missing pending review decision",
			mutatePacket: func(_ *CandidateReviewPacket, _, _ CandidateReviewItem) {},
			mutateDecision: func(decision *CandidateDecisionFile) {
				decision.Decisions = decision.Decisions[:1]
			},
			wantError: "candidate decisions do not exactly cover pending review items",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			repoRoot, caseRoot, pack := promoteFixture(t)
			created, err := CreateCandidates(repoRoot, caseRoot, pack, CandidateOptions{})
			if err != nil {
				t.Fatal(err)
			}
			created, err = WriteCandidateReviewWorkspace(created, CandidateArtifactOptions{ReviewOutputDir: filepath.Join(caseRoot, ".rekit", "reviews", "candidate-authority-"+safeCandidateName(tt.name))})
			if err != nil {
				t.Fatal(err)
			}
			var packet CandidateReviewPacket
			if err := decodeStrictJSON(readCandidateDecisionTestFile(t, created.ReviewWorkspace.PacketPath), &packet); err != nil {
				t.Fatal(err)
			}
			var managed, tooling CandidateReviewItem
			for _, item := range packet.CandidateResult.ReviewPlan.ReviewItems {
				if item.ReviewDecision != "pending-review" {
					continue
				}
				switch item.Kind {
				case "managed-doc":
					managed = item
				case "tooling-candidate-source":
					tooling = item
				}
			}
			if managed.CandidatePath == "" || tooling.CandidatePath == "" {
				t.Fatalf("authority fixture omitted pending reviews: managed=%+v tooling=%+v", managed, tooling)
			}
			evidence := CandidateDecisionEvidence{Path: created.ReviewWorkspace.CombinedDiffPath, SHA256: fileSHA256(created.ReviewWorkspace.CombinedDiffPath)}
			decision := CandidateDecisionFile{
				SchemaVersion: 1,
				Kind:          "pack-memory-candidate-decisions",
				Decisions: []CandidateDecisionItem{
					{CandidatePath: managed.CandidatePath, Decision: "accept", CandidateHash: fileSHA256(managed.CandidatePath), PackTargetHash: fileSHA256(managed.PackTarget), Reason: "reviewed", Actor: "mission-commander", EvidenceRefs: []CandidateDecisionEvidence{evidence}},
					{CandidatePath: tooling.CandidatePath, Decision: "reject", CandidateHash: fileSHA256(tooling.CandidatePath), Reason: "reviewed", Actor: "mission-commander", EvidenceRefs: []CandidateDecisionEvidence{evidence}},
				},
			}
			tt.mutatePacket(&packet, managed, tooling)
			writeCandidateDecisionTestJSON(t, created.ReviewWorkspace.PacketPath, packet)
			packetBytes := readCandidateDecisionTestFile(t, created.ReviewWorkspace.PacketPath)
			decision.PacketHash = sha256Hex(packetBytes)
			if tt.mutateDecision != nil {
				tt.mutateDecision(&decision)
			}
			decisionPath := filepath.Join(filepath.Dir(created.ReviewWorkspace.PacketPath), "decisions.json")
			writeCandidateDecisionFixture(t, decisionPath, decision)

			candidateBefore := map[string][]byte{
				managed.CandidatePath: readCandidateDecisionTestFile(t, managed.CandidatePath),
				tooling.CandidatePath: readCandidateDecisionTestFile(t, tooling.CandidatePath),
			}
			targetBefore := readCandidateDecisionTestFile(t, managed.PackTarget)
			indexBefore := readCandidateDecisionTestFile(t, created.IndexPath)
			backupRoot := filepath.Join(created.CandidateRoot, ".decision-backup")
			receiptRoot := filepath.Join(created.CandidateRoot, "review-artifacts")

			for _, whatIf := range []bool{true, false} {
				_, err := ApplyCandidateDecisions(repoRoot, caseRoot, pack, CandidateDecisionOptions{PacketPath: created.ReviewWorkspace.PacketPath, DecisionPath: decisionPath, WhatIf: whatIf})
				if err == nil || !strings.Contains(err.Error(), tt.wantError) {
					t.Fatalf("WhatIf=%v error = %v, want %q", whatIf, err, tt.wantError)
				}
				for path, before := range candidateBefore {
					if after := readCandidateDecisionTestFile(t, path); string(after) != string(before) {
						t.Fatalf("WhatIf=%v mutated candidate %s", whatIf, path)
					}
				}
				if after := readCandidateDecisionTestFile(t, managed.PackTarget); string(after) != string(targetBefore) {
					t.Fatalf("WhatIf=%v mutated pack target", whatIf)
				}
				if after := readCandidateDecisionTestFile(t, created.IndexPath); string(after) != string(indexBefore) {
					t.Fatalf("WhatIf=%v mutated candidate index", whatIf)
				}
				if entries, err := os.ReadDir(backupRoot); err == nil && len(entries) != 0 || err != nil && !os.IsNotExist(err) {
					t.Fatalf("WhatIf=%v created transaction entries: entries=%v err=%v", whatIf, entries, err)
				}
				if entries, err := os.ReadDir(receiptRoot); err == nil && len(entries) != 0 || err != nil && !os.IsNotExist(err) {
					t.Fatalf("WhatIf=%v created receipt entries: entries=%v err=%v", whatIf, entries, err)
				}
			}
		})
	}
}

func TestLoadCandidateDecisionAuthorityRejectsEmptyPendingReviewCandidatePathOnReplay(t *testing.T) {
	for _, candidatePath := range []string{"", " \t "} {
		name := "empty"
		if candidatePath != "" {
			name = "blank"
		}
		t.Run(name, func(t *testing.T) {
			repoRoot, caseRoot, pack := promoteFixture(t)
			created, packet, managed := candidateDecisionFixture(t, repoRoot, caseRoot, pack, "authority-replay-"+name)
			decisionPath := filepath.Join(filepath.Dir(created.ReviewWorkspace.PacketPath), "decisions.json")
			decision := reviewedCandidateDecision(packet, managed, created.ReviewWorkspace.CombinedDiffPath)
			writeCandidateDecisionFixture(t, decisionPath, decision)
			applied, err := ApplyCandidateDecisions(repoRoot, caseRoot, pack, CandidateDecisionOptions{PacketPath: created.ReviewWorkspace.PacketPath, DecisionPath: decisionPath})
			if err != nil {
				t.Fatal(err)
			}

			for idx := range packet.CandidateResult.ReviewPlan.ReviewItems {
				if packet.CandidateResult.ReviewPlan.ReviewItems[idx].CandidatePath == managed.CandidatePath {
					packet.CandidateResult.ReviewPlan.ReviewItems[idx].CandidatePath = candidatePath
					break
				}
			}
			writeCandidateDecisionTestJSON(t, created.ReviewWorkspace.PacketPath, packet)
			packetHash := sha256Hex(readCandidateDecisionTestFile(t, created.ReviewWorkspace.PacketPath))
			decision.PacketHash = packetHash
			writeCandidateDecisionTestJSON(t, decisionPath, decision)
			decisionHash := sha256Hex(readCandidateDecisionTestFile(t, decisionPath))
			receiptPath := candidateDecisionReceiptPath(created.CandidateRoot, packetHash, decisionHash)

			receipt := *applied.Receipt
			receipt.PacketHash = packetHash
			receipt.DecisionHash = decisionHash
			receipt.ReceiptPath = receiptPath
			receipt.VerificationProofPath = candidateDecisionVerificationProofPath(created.CandidateRoot, packetHash, decisionHash)
			receipt.VerificationWorkspaceRoot = candidateDecisionVerificationWorkspace(caseRoot, packetHash, decisionHash)
			freshRoot := filepath.Join(receipt.VerificationWorkspaceRoot, "fresh")
			attachedRoot := filepath.Join(receipt.VerificationWorkspaceRoot, "attached")
			receipt.VerificationProvisionCommand = candidateDecisionVerificationProvisionCommand(created.ReviewWorkspace.PacketPath, decisionPath, freshRoot, attachedRoot)
			receipt.VerificationCommand = candidateDecisionVerificationCommand(created.ReviewWorkspace.PacketPath, decisionPath, freshRoot, attachedRoot)
			writeCandidateDecisionTestJSON(t, receiptPath, receipt)

			transactionPath := filepath.Join(applied.BackupRoot, "transaction.json")
			var transaction candidateDecisionTransaction
			if err := decodeStrictJSON(readCandidateDecisionTestFile(t, transactionPath), &transaction); err != nil {
				t.Fatal(err)
			}
			transaction.PacketHash = packetHash
			transaction.DecisionHash = decisionHash
			transaction.Result.PacketHash = packetHash
			writeCandidateDecisionTestJSON(t, transactionPath, transaction)

			committedPath := filepath.Join(applied.BackupRoot, "committed.json")
			var committed CandidateDecisionResult
			if err := decodeStrictJSON(readCandidateDecisionTestFile(t, committedPath), &committed); err != nil {
				t.Fatal(err)
			}
			committed.PacketHash = packetHash
			committed.ReceiptPath = receiptPath
			committed.Receipt = &receipt
			writeCandidateDecisionTestJSON(t, committedPath, committed)

			if _, err := loadCandidateDecisionAuthority(repoRoot, caseRoot, pack, created.ReviewWorkspace.PacketPath, decisionPath); err == nil || !strings.Contains(err.Error(), "malformed candidate review: pending-review has invalid candidatePath") {
				t.Fatalf("authority replay error = %v", err)
			}
		})
	}
}

func TestApplyCandidateDecisionsRejectsHashDriftAndToolingAutoAccept(t *testing.T) {
	repoRoot, caseRoot, pack := promoteFixture(t)
	created, err := CreateCandidates(repoRoot, caseRoot, pack, CandidateOptions{})
	if err != nil {
		t.Fatal(err)
	}
	created, err = WriteCandidateReviewWorkspace(created, CandidateArtifactOptions{ReviewOutputDir: filepath.Join(caseRoot, ".rekit", "reviews", "candidate-drift")})
	if err != nil {
		t.Fatal(err)
	}
	packetBytes, err := os.ReadFile(created.ReviewWorkspace.PacketPath)
	if err != nil {
		t.Fatal(err)
	}
	var packet CandidateReviewPacket
	if err := json.Unmarshal(packetBytes, &packet); err != nil {
		t.Fatal(err)
	}
	for _, item := range packet.CandidateResult.ReviewPlan.ReviewItems {
		if item.ReviewDecision != "pending-review" {
			continue
		}
		decision := reviewedCandidateDecision(packet, item, created.ReviewWorkspace.CombinedDiffPath)
		decisionPath := filepath.Join(caseRoot, ".rekit", "reviews", "candidate-drift", safeCandidateName(item.Kind)+"-decision.json")
		writeCandidateDecisionFixture(t, decisionPath, decision)
		if item.Kind == "tooling-candidate-source" {
			if _, err := ApplyCandidateDecisions(repoRoot, caseRoot, pack, CandidateDecisionOptions{PacketPath: created.ReviewWorkspace.PacketPath, DecisionPath: decisionPath, WhatIf: true}); err == nil {
				t.Fatal("tooling candidate auto-accept should be rejected")
			}
			continue
		}
		if err := os.WriteFile(item.CandidatePath, []byte("drifted candidate\n"), 0o644); err != nil {
			t.Fatal(err)
		}
		if _, err := ApplyCandidateDecisions(repoRoot, caseRoot, pack, CandidateDecisionOptions{PacketPath: created.ReviewWorkspace.PacketPath, DecisionPath: decisionPath, WhatIf: true}); err == nil {
			t.Fatal("candidate hash drift should be rejected")
		}
		break
	}
}

func TestApplyCandidateDecisionsRejectsForgedPacketBindingsAndEvidenceDrift(t *testing.T) {
	tests := []struct {
		name   string
		mutate func(*CandidateReviewPacket, *CandidateDecisionFile, CandidateReviewItem, CandidateResult)
	}{
		{
			name: "canonical root",
			mutate: func(packet *CandidateReviewPacket, _ *CandidateDecisionFile, _ CandidateReviewItem, _ CandidateResult) {
				packet.CandidateResult.CandidateRoot = packet.CandidateResult.RepoRoot
			},
		},
		{
			name: "manifest target",
			mutate: func(packet *CandidateReviewPacket, _ *CandidateDecisionFile, managed CandidateReviewItem, _ CandidateResult) {
				for idx := range packet.CandidateResult.ReviewPlan.ReviewItems {
					if packet.CandidateResult.ReviewPlan.ReviewItems[idx].CandidatePath == managed.CandidatePath {
						packet.CandidateResult.ReviewPlan.ReviewItems[idx].PackTarget = filepath.Join(packet.CandidateResult.RepoRoot, "packs", packet.CandidateResult.Pack, "manifest.yml")
					}
				}
			},
		},
		{
			name: "packet writes",
			mutate: func(packet *CandidateReviewPacket, _ *CandidateDecisionFile, managed CandidateReviewItem, _ CandidateResult) {
				for idx := range packet.CandidateResult.Writes {
					if packet.CandidateResult.Writes[idx].TargetPath == managed.CandidatePath {
						packet.CandidateResult.Writes[idx].Action = "skip"
					}
				}
			},
		},
		{
			name: "evidence drift",
			mutate: func(_ *CandidateReviewPacket, decision *CandidateDecisionFile, _ CandidateReviewItem, created CandidateResult) {
				if err := os.WriteFile(created.ReviewWorkspace.CombinedDiffPath, []byte("changed evidence\n"), 0o644); err != nil {
					panic(err)
				}
				decision.Decisions[0].EvidenceRefs[0].SHA256 = strings.Repeat("0", 64)
			},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			repoRoot, caseRoot, pack := promoteFixture(t)
			created, packet, managed := candidateDecisionFixture(t, repoRoot, caseRoot, pack, tt.name)
			decision := reviewedCandidateDecision(packet, managed, created.ReviewWorkspace.CombinedDiffPath)
			tt.mutate(&packet, &decision, managed, created)
			packetBytes, err := json.MarshalIndent(packet, "", "  ")
			if err != nil {
				t.Fatal(err)
			}
			if err := os.WriteFile(created.ReviewWorkspace.PacketPath, append(packetBytes, '\n'), 0o644); err != nil {
				t.Fatal(err)
			}
			decision.PacketHash = sha256Hex(append(packetBytes, '\n'))
			decisionPath := filepath.Join(caseRoot, ".rekit", "reviews", "candidate-bindings", safeCandidateName(tt.name)+".json")
			writeCandidateDecisionFixture(t, decisionPath, decision)
			if _, err := ApplyCandidateDecisions(repoRoot, caseRoot, pack, CandidateDecisionOptions{PacketPath: created.ReviewWorkspace.PacketPath, DecisionPath: decisionPath, WhatIf: true}); err == nil {
				t.Fatalf("forged %s binding should be rejected", tt.name)
			}
		})
	}
}

func TestApplyCandidateDecisionsRejectAndRollbackCleanupFailure(t *testing.T) {
	repoRoot, caseRoot, pack := promoteFixture(t)
	created, packet, managed := candidateDecisionFixture(t, repoRoot, caseRoot, pack, "rollback")
	decision := reviewedCandidateDecision(packet, managed, created.ReviewWorkspace.CombinedDiffPath)
	decisionPath := filepath.Join(caseRoot, ".rekit", "reviews", "candidate-rollback", "decisions.json")
	writeCandidateDecisionFixture(t, decisionPath, decision)
	originalTarget, err := os.ReadFile(managed.PackTarget)
	if err != nil {
		t.Fatal(err)
	}
	originalRemove := removeCandidateDecisionFile
	removeCandidateDecisionFile = func(path string) error {
		if sameCandidateDecisionPath(path, managed.CandidatePath) {
			return os.ErrPermission
		}
		return originalRemove(path)
	}
	t.Cleanup(func() { removeCandidateDecisionFile = originalRemove })

	result, err := ApplyCandidateDecisions(repoRoot, caseRoot, pack, CandidateDecisionOptions{PacketPath: created.ReviewWorkspace.PacketPath, DecisionPath: decisionPath})
	if err == nil || !result.RolledBack || result.RecoveryRequired || result.FailedAction != "cleanup candidate" {
		t.Fatalf("cleanup failure did not return rolled-back recovery envelope: result=%+v err=%v", result, err)
	}
	target, readErr := os.ReadFile(managed.PackTarget)
	if readErr != nil {
		t.Fatal(readErr)
	}
	if string(target) != string(originalTarget) {
		t.Fatalf("rollback did not restore target: %q", target)
	}
	if _, statErr := os.Stat(managed.CandidatePath); statErr != nil {
		t.Fatalf("rollback lost candidate: %v", statErr)
	}
}

func TestApplyCandidateDecisionsRecoversInterruptedTransactionBeforePlanning(t *testing.T) {
	repoRoot, caseRoot, pack := promoteFixture(t)
	created, packet, managed := candidateDecisionFixture(t, repoRoot, caseRoot, pack, "interrupted")
	decision := reviewedCandidateDecision(packet, managed, created.ReviewWorkspace.CombinedDiffPath)
	decisionPath := filepath.Join(caseRoot, ".rekit", "reviews", "candidate-interrupted", "decisions.json")
	writeCandidateDecisionFixture(t, decisionPath, decision)
	packetBytes, err := os.ReadFile(created.ReviewWorkspace.PacketPath)
	if err != nil {
		t.Fatal(err)
	}
	decisionBytes, err := os.ReadFile(decisionPath)
	if err != nil {
		t.Fatal(err)
	}
	originalTarget, err := os.ReadFile(managed.PackTarget)
	if err != nil {
		t.Fatal(err)
	}
	candidateBytes, err := os.ReadFile(managed.CandidatePath)
	if err != nil {
		t.Fatal(err)
	}
	transactionRoot := filepath.Join(created.CandidateRoot, ".decision-backup", "interrupted-fixture")
	if err := os.MkdirAll(transactionRoot, 0o755); err != nil {
		t.Fatal(err)
	}
	candidateBackup := filepath.Join(transactionRoot, "candidate.md")
	targetBackup := filepath.Join(transactionRoot, "target.md")
	indexBackup := filepath.Join(transactionRoot, "index.json")
	if err := os.WriteFile(candidateBackup, candidateBytes, 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(targetBackup, originalTarget, 0o644); err != nil {
		t.Fatal(err)
	}
	indexBytes, err := os.ReadFile(created.IndexPath)
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(indexBackup, indexBytes, 0o644); err != nil {
		t.Fatal(err)
	}
	result := CandidateDecisionResult{SchemaVersion: 1, Command: "promote", Mode: "candidate-decision", CaseRoot: caseRoot, RepoRoot: repoRoot, Pack: pack, PacketPath: created.ReviewWorkspace.PacketPath, DecisionPath: decisionPath, PacketHash: sha256Hex(packetBytes), IsMutation: true, BackupRoot: transactionRoot, IndexPath: created.IndexPath}
	action := CandidateDecisionAction{CandidatePath: managed.CandidatePath, PackTarget: managed.PackTarget, CandidateBackupPath: candidateBackup, TargetBackupPath: targetBackup}
	transaction := candidateDecisionTransaction{SchemaVersion: 1, Kind: "pack-memory-candidate-decision-transaction", PacketHash: sha256Hex(packetBytes), DecisionHash: sha256Hex(decisionBytes), IndexExisted: true, IndexBackupPath: indexBackup, Result: result, Actions: []CandidateDecisionAction{action}}
	if err := writeCandidateDecisionTransaction(filepath.Join(transactionRoot, "transaction.json"), transaction); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(managed.PackTarget, candidateBytes, 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.Remove(managed.CandidatePath); err != nil {
		t.Fatal(err)
	}
	if err := os.Remove(created.IndexPath); err != nil {
		t.Fatal(err)
	}

	recovered, err := ApplyCandidateDecisions(repoRoot, caseRoot, pack, CandidateDecisionOptions{PacketPath: created.ReviewWorkspace.PacketPath, DecisionPath: decisionPath})
	if err == nil || !recovered.RolledBack || recovered.RecoveryRequired || recovered.FailedAction != "recover interrupted transaction" {
		t.Fatalf("interrupted transaction did not return recovery envelope: result=%+v err=%v", recovered, err)
	}
	target, err := os.ReadFile(managed.PackTarget)
	if err != nil || string(target) != string(originalTarget) {
		t.Fatalf("interrupted recovery target mismatch: %q err=%v", target, err)
	}
	if _, err := os.Stat(managed.CandidatePath); err != nil {
		t.Fatalf("interrupted recovery did not restore candidate: %v", err)
	}
	if _, err := os.Stat(created.IndexPath); err != nil {
		t.Fatalf("interrupted recovery did not restore index: %v", err)
	}
}

func TestApplyCandidateDecisionsRejectsForgedApplyRecoveryPacket(t *testing.T) {
	repoRoot, caseRoot, pack := promoteFixture(t)
	created, packet, managed := candidateDecisionFixture(t, repoRoot, caseRoot, pack, "forged-recovery")
	packet.CandidateResult.CandidateRoot = repoRoot
	packetBytes, err := json.MarshalIndent(packet, "", "  ")
	if err != nil {
		t.Fatal(err)
	}
	packetBytes = append(packetBytes, '\n')
	if err := os.WriteFile(created.ReviewWorkspace.PacketPath, packetBytes, 0o644); err != nil {
		t.Fatal(err)
	}
	decision := reviewedCandidateDecision(packet, managed, created.ReviewWorkspace.CombinedDiffPath)
	decision.PacketHash = sha256Hex(packetBytes)
	decisionPath := filepath.Join(caseRoot, ".rekit", "reviews", "candidate-forged-recovery", "decisions.json")
	writeCandidateDecisionFixture(t, decisionPath, decision)
	outside := filepath.Join(t.TempDir(), "outside.txt")
	if err := os.WriteFile(outside, []byte("unchanged\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if _, err := ApplyCandidateDecisions(repoRoot, caseRoot, pack, CandidateDecisionOptions{PacketPath: created.ReviewWorkspace.PacketPath, DecisionPath: decisionPath}); err == nil {
		t.Fatal("forged apply recovery packet should be rejected")
	}
	content, err := os.ReadFile(outside)
	if err != nil || string(content) != "unchanged\n" {
		t.Fatalf("forged recovery changed external file: %q err=%v", content, err)
	}
}

func TestApplyCandidateDecisionsRejectsConcurrentLock(t *testing.T) {
	repoRoot, caseRoot, pack := promoteFixture(t)
	created, packet, managed := candidateDecisionFixture(t, repoRoot, caseRoot, pack, "lock")
	decision := reviewedCandidateDecision(packet, managed, created.ReviewWorkspace.CombinedDiffPath)
	decisionPath := filepath.Join(caseRoot, ".rekit", "reviews", "candidate-lock", "decisions.json")
	writeCandidateDecisionFixture(t, decisionPath, decision)
	lockPath := filepath.Join(created.CandidateRoot, ".decision.lock")
	if err := os.WriteFile(lockPath, fmt.Appendf(nil, "pid=%d\n", os.Getpid()), 0o600); err != nil {
		t.Fatal(err)
	}
	if _, err := ApplyCandidateDecisions(repoRoot, caseRoot, pack, CandidateDecisionOptions{PacketPath: created.ReviewWorkspace.PacketPath, DecisionPath: decisionPath}); err == nil || !strings.Contains(err.Error(), "another candidate decision transaction is active") {
		t.Fatalf("concurrent lock should fail closed: %v", err)
	}
}

func TestApplyCandidateDecisionsTakesOverMalformedLock(t *testing.T) {
	repoRoot, caseRoot, pack := promoteFixture(t)
	created, packet, managed := candidateDecisionFixture(t, repoRoot, caseRoot, pack, "malformed-lock")
	decision := reviewedCandidateDecision(packet, managed, created.ReviewWorkspace.CombinedDiffPath)
	decision.Decisions[0].Decision = "reject"
	decision.Decisions[0].PackTargetHash = ""
	decisionPath := filepath.Join(caseRoot, ".rekit", "reviews", "candidate-malformed-lock", "decisions.json")
	writeCandidateDecisionFixture(t, decisionPath, decision)
	lockPath := filepath.Join(created.CandidateRoot, ".decision.lock")
	if err := os.WriteFile(lockPath, nil, 0o600); err != nil {
		t.Fatal(err)
	}
	result, err := ApplyCandidateDecisions(repoRoot, caseRoot, pack, CandidateDecisionOptions{PacketPath: created.ReviewWorkspace.PacketPath, DecisionPath: decisionPath})
	if err != nil || !result.Applied || result.Rejected != len(decision.Decisions) {
		t.Fatalf("malformed stale lock takeover failed: result=%+v err=%v", result, err)
	}
}

func TestApplyCandidateDecisionsTakesOverStaleLock(t *testing.T) {
	repoRoot, caseRoot, pack := promoteFixture(t)
	created, packet, managed := candidateDecisionFixture(t, repoRoot, caseRoot, pack, "stale-lock")
	decision := reviewedCandidateDecision(packet, managed, created.ReviewWorkspace.CombinedDiffPath)
	decision.Decisions[0].Decision = "reject"
	decision.Decisions[0].PackTargetHash = ""
	decisionPath := filepath.Join(caseRoot, ".rekit", "reviews", "candidate-stale-lock", "decisions.json")
	writeCandidateDecisionFixture(t, decisionPath, decision)
	lockPath := filepath.Join(created.CandidateRoot, ".decision.lock")
	if err := os.WriteFile(lockPath, []byte("pid=2147483647\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	result, err := ApplyCandidateDecisions(repoRoot, caseRoot, pack, CandidateDecisionOptions{PacketPath: created.ReviewWorkspace.PacketPath, DecisionPath: decisionPath})
	if err != nil || !result.Applied || result.Rejected != len(decision.Decisions) {
		t.Fatalf("stale lock takeover failed: result=%+v err=%v", result, err)
	}
}

func TestApplyCandidateDecisionsRejectsSymlinkBackupParent(t *testing.T) {
	if os.PathSeparator == '\\' {
		t.Skip("symlink creation is not reliably available on Windows test hosts")
	}
	repoRoot, caseRoot, pack := promoteFixture(t)
	created, packet, managed := candidateDecisionFixture(t, repoRoot, caseRoot, pack, "backup-symlink")
	decision := reviewedCandidateDecision(packet, managed, created.ReviewWorkspace.CombinedDiffPath)
	decisionPath := filepath.Join(caseRoot, ".rekit", "reviews", "candidate-backup-symlink", "decisions.json")
	writeCandidateDecisionFixture(t, decisionPath, decision)
	external := t.TempDir()
	if err := os.Symlink(external, filepath.Join(created.CandidateRoot, ".decision-backup")); err != nil {
		t.Fatal(err)
	}
	if _, err := ApplyCandidateDecisions(repoRoot, caseRoot, pack, CandidateDecisionOptions{PacketPath: created.ReviewWorkspace.PacketPath, DecisionPath: decisionPath}); err == nil {
		t.Fatal("symlink backup parent should be rejected")
	}
	entries, err := os.ReadDir(external)
	if err != nil || len(entries) != 0 {
		t.Fatalf("symlink backup parent wrote outside candidate root: entries=%v err=%v", entries, err)
	}
}

func TestApplyCandidateDecisionsRejectsSymlinkCandidate(t *testing.T) {
	if os.PathSeparator == '\\' {
		t.Skip("symlink creation is not reliably available on Windows test hosts")
	}
	repoRoot, caseRoot, pack := promoteFixture(t)
	created, packet, managed := candidateDecisionFixture(t, repoRoot, caseRoot, pack, "symlink")
	realCandidate := managed.CandidatePath + ".real"
	if err := os.Rename(managed.CandidatePath, realCandidate); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink(realCandidate, managed.CandidatePath); err != nil {
		t.Fatal(err)
	}
	decision := reviewedCandidateDecision(packet, managed, created.ReviewWorkspace.CombinedDiffPath)
	decision.Decisions[0].CandidateHash = fileSHA256(realCandidate)
	decisionPath := filepath.Join(caseRoot, ".rekit", "reviews", "candidate-symlink", "decisions.json")
	writeCandidateDecisionFixture(t, decisionPath, decision)
	if _, err := ApplyCandidateDecisions(repoRoot, caseRoot, pack, CandidateDecisionOptions{PacketPath: created.ReviewWorkspace.PacketPath, DecisionPath: decisionPath, WhatIf: true}); err == nil {
		t.Fatal("symlink candidate should be rejected")
	}
}

func candidateDecisionFixture(t *testing.T, repoRoot, caseRoot, pack, name string) (CandidateResult, CandidateReviewPacket, CandidateReviewItem) {
	t.Helper()
	created, err := CreateCandidates(repoRoot, caseRoot, pack, CandidateOptions{})
	if err != nil {
		t.Fatal(err)
	}
	created, err = WriteCandidateReviewWorkspace(created, CandidateArtifactOptions{ReviewOutputDir: filepath.Join(caseRoot, ".rekit", "reviews", "candidate-"+safeCandidateName(name))})
	if err != nil {
		t.Fatal(err)
	}
	packetBytes, err := os.ReadFile(created.ReviewWorkspace.PacketPath)
	if err != nil {
		t.Fatal(err)
	}
	var packet CandidateReviewPacket
	if err := json.Unmarshal(packetBytes, &packet); err != nil {
		t.Fatal(err)
	}
	for _, item := range packet.CandidateResult.ReviewPlan.ReviewItems {
		if item.Kind == "managed-doc" && item.ReviewDecision == "pending-review" {
			return created, packet, item
		}
	}
	t.Fatal("review packet omitted pending managed candidate")
	return CandidateResult{}, CandidateReviewPacket{}, CandidateReviewItem{}
}

func reviewedCandidateDecision(packet CandidateReviewPacket, managed CandidateReviewItem, evidencePath string) CandidateDecisionFile {
	packetBytes, _ := json.MarshalIndent(packet, "", "  ")
	packetBytes = append(packetBytes, '\n')
	evidence := CandidateDecisionEvidence{Path: evidencePath, SHA256: fileSHA256(evidencePath)}
	decisions := []CandidateDecisionItem{{
		CandidatePath:  managed.CandidatePath,
		Decision:       "accept",
		CandidateHash:  fileSHA256(managed.CandidatePath),
		PackTargetHash: fileSHA256(managed.PackTarget),
		Reason:         "reviewed bounded candidate diff",
		Actor:          "mission-commander",
		EvidenceRefs:   []CandidateDecisionEvidence{evidence},
	}}
	for _, item := range packet.CandidateResult.ReviewPlan.ReviewItems {
		if item.ReviewDecision != "pending-review" || sameCandidateDecisionPath(item.CandidatePath, managed.CandidatePath) {
			continue
		}
		decisions = append(decisions, CandidateDecisionItem{
			CandidatePath: item.CandidatePath,
			Decision:      "reject",
			CandidateHash: fileSHA256(item.CandidatePath),
			Reason:        "reviewed bounded candidate diff",
			Actor:         "mission-commander",
			EvidenceRefs:  []CandidateDecisionEvidence{evidence},
		})
	}
	return CandidateDecisionFile{
		SchemaVersion: 1,
		Kind:          "pack-memory-candidate-decisions",
		PacketHash:    sha256Hex(packetBytes),
		Decisions:     decisions,
	}
}

func writeCandidateDecisionFixture(t *testing.T, path string, decision CandidateDecisionFile) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatal(err)
	}
	writeCandidateDecisionTestJSON(t, path, decision)
}

func readCandidateDecisionTestFile(t *testing.T, path string) []byte {
	t.Helper()
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	return data
}

func writeCandidateDecisionTestJSON(t *testing.T, path string, value any) {
	t.Helper()
	data, err := json.MarshalIndent(value, "", "  ")
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, append(data, '\n'), 0o644); err != nil {
		t.Fatal(err)
	}
}
