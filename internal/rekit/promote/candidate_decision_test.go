package promote

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"

	syncpkg "github.com/shuiyu486/re-context-kits/internal/rekit/sync"
)

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
	decision := CandidateDecisionFile{
		SchemaVersion: 1,
		Kind:          "pack-memory-candidate-decisions",
		PacketHash:    sha256Hex(packetBytes),
		Decisions: []CandidateDecisionItem{{
			CandidatePath:  managed.CandidatePath,
			Decision:       "accept",
			CandidateHash:  fileSHA256(managed.CandidatePath),
			PackTargetHash: fileSHA256(managed.PackTarget),
			Reason:         "reviewed bounded diff and accepted reusable content",
			Actor:          "mission-commander",
			EvidenceRefs:   []CandidateDecisionEvidence{{Path: created.ReviewWorkspace.CombinedDiffPath, SHA256: fileSHA256(created.ReviewWorkspace.CombinedDiffPath)}},
		}},
	}
	writeCandidateDecisionFixture(t, decisionPath, decision)
	originalPack, err := os.ReadFile(managed.PackTarget)
	if err != nil {
		t.Fatal(err)
	}

	preview, err := ApplyCandidateDecisions(repoRoot, caseRoot, pack, CandidateDecisionOptions{PacketPath: created.ReviewWorkspace.PacketPath, DecisionPath: decisionPath, WhatIf: true})
	if err != nil {
		t.Fatal(err)
	}
	if preview.IsMutation || preview.Applied || preview.Accepted != 1 || len(preview.Actions) != 1 || preview.Actions[0].Action != "merge-accepted-candidate-and-cleanup" {
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
	if applied.Receipt == nil || applied.ReceiptPath == "" || !applied.Receipt.VerificationPending || !strings.Contains(applied.Receipt.VerificationCommand, "-FreshCaseRoot <fresh-case>") {
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
		decision := CandidateDecisionFile{SchemaVersion: 1, Kind: "pack-memory-candidate-decisions", PacketHash: sha256Hex(packetBytes), Decisions: []CandidateDecisionItem{{CandidatePath: item.CandidatePath, Decision: "accept", CandidateHash: fileSHA256(item.CandidatePath), PackTargetHash: fileSHA256(item.PackTarget), Reason: "reviewed", Actor: "mission-commander", EvidenceRefs: []CandidateDecisionEvidence{{Path: created.ReviewWorkspace.CombinedDiffPath, SHA256: fileSHA256(created.ReviewWorkspace.CombinedDiffPath)}}}}}
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
	if err != nil || !result.Applied || result.Rejected != 1 {
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
	if err != nil || !result.Applied || result.Rejected != 1 {
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
	return CandidateDecisionFile{
		SchemaVersion: 1,
		Kind:          "pack-memory-candidate-decisions",
		PacketHash:    sha256Hex(packetBytes),
		Decisions: []CandidateDecisionItem{{
			CandidatePath:  managed.CandidatePath,
			Decision:       "accept",
			CandidateHash:  fileSHA256(managed.CandidatePath),
			PackTargetHash: fileSHA256(managed.PackTarget),
			Reason:         "reviewed bounded candidate diff",
			Actor:          "mission-commander",
			EvidenceRefs:   []CandidateDecisionEvidence{{Path: evidencePath, SHA256: fileSHA256(evidencePath)}},
		}},
	}
}

func writeCandidateDecisionFixture(t *testing.T, path string, decision CandidateDecisionFile) {
	t.Helper()
	data, err := json.MarshalIndent(decision, "", "  ")
	if err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, append(data, '\n'), 0o644); err != nil {
		t.Fatal(err)
	}
}
