package releasecheck

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	syncpkg "github.com/shuiyu486/re-context-kits/internal/rekit/sync"
)

func TestBuildCompletedPackMemoryChangeCatalogIncludesCompletedChangeWithOpenProducerWork(t *testing.T) {
	fixture := writeCompletedPackMemoryChangeFixture(t)
	writeFile(t, filepath.Join(fixture.candidateRoot, "still-open.candidate.md"), "open producer work\n")

	catalog, err := BuildCompletedPackMemoryChangeCatalog(fixture.repo, "fixture")
	if err != nil {
		t.Fatal(err)
	}
	if catalog.SchemaVersion != 1 || catalog.Kind != completedPackMemoryChangeCatalogKind || catalog.RepoRoot != fixture.repo || catalog.Pack != "fixture" || len(catalog.Changes) != 1 || len(catalog.Warnings) != 0 {
		t.Fatalf("unexpected completed pack-memory catalog: %+v", catalog)
	}
	change := catalog.Changes[0]
	wantSourceSHA256 := sha256ReleaseHandoff([]byte(fixture.targetContent))
	if change.ManagedPath != "memory.md" || change.SourcePath != "packs/fixture/memory.md" || change.SourceSHA256 != wantSourceSHA256 || change.AuthoritySHA256 == "" || !strings.HasPrefix(change.ChangeID, "pack-memory-change-") || len(strings.TrimPrefix(change.ChangeID, "pack-memory-change-")) != 64 {
		t.Fatalf("completed change identity drifted: %+v", change)
	}
	if change.DecisionReceiptPath != "packs/fixture/promote-candidates/review-artifacts/fixture.candidate-decision-receipt.json" || change.DecisionReceiptSHA256 != fileSHA256ReleaseHandoff(filepath.Join(fixture.repo, filepath.FromSlash(change.DecisionReceiptPath))) || change.VerificationProofPath != "packs/fixture/promote-candidates/review-artifacts/fixture.candidate-verification-proof.json" || change.VerificationProofSHA256 == "" || change.RetirementIntentPath == "" || change.RetirementIntentSHA256 == "" || change.RetirementReceiptPath == "" || change.RetirementReceiptSHA256 == "" {
		t.Fatalf("completed change authority artifacts drifted: %+v", change)
	}
	wantChangeID := "pack-memory-change-" + sha256ReleaseHandoff([]byte(strings.Join([]string{
		"fixture",
		change.DecisionReceiptSHA256,
		change.VerificationProofSHA256,
		change.RetirementIntentSHA256,
		change.RetirementReceiptSHA256,
		"packs/fixture/memory.md",
		wantSourceSHA256,
	}, "\x00")))
	if change.ChangeID != wantChangeID {
		t.Fatalf("changeId is not bound to receipt hashes and action target/hash: got %s want %s", change.ChangeID, wantChangeID)
	}
	for name, proof := range map[string]CompletedPackMemoryChangeProof{
		"cleanup":  change.CleanupProof,
		"doctor":   change.PackDoctorProof,
		"fresh":    change.FreshReconsumeProof,
		"attached": change.AttachedReconsumeProof,
	} {
		if proof.Path == "" || proof.SHA256 == "" || proof.SHA256 != fileSHA256ReleaseHandoff(filepath.Join(fixture.repo, filepath.FromSlash(proof.Path))) {
			t.Fatalf("%s proof identity drifted: %+v", name, proof)
		}
	}

	again, err := BuildCompletedPackMemoryChangeCatalog(fixture.repo, "fixture")
	if err != nil {
		t.Fatal(err)
	}
	if len(again.Changes) != 1 || again.Changes[0].ChangeID != change.ChangeID || again.Changes[0].SourceSHA256 != wantSourceSHA256 {
		t.Fatalf("completed change identity/hash is not stable: first=%+v second=%+v", change, again.Changes)
	}
}

func TestBuildCompletedPackMemoryChangeCatalogOmitsChangeMissingProof(t *testing.T) {
	fixture := writeCompletedPackMemoryChangeFixture(t)
	if err := os.Remove(filepath.Join(fixture.proofRoot, "memory.attached-case-reconsume-proof.json")); err != nil {
		t.Fatal(err)
	}

	catalog, err := BuildCompletedPackMemoryChangeCatalog(fixture.repo, "fixture")
	if err != nil {
		t.Fatal(err)
	}
	if len(catalog.Changes) != 0 {
		t.Fatalf("change with missing strict proof was cataloged: %+v", catalog.Changes)
	}
}

type completedPackMemoryChangeFixture struct {
	repo          string
	candidateRoot string
	proofRoot     string
	targetContent string
}

func writeCompletedPackMemoryChangeFixture(t *testing.T) completedPackMemoryChangeFixture {
	t.Helper()
	repo := t.TempDir()
	packRoot := filepath.Join(repo, "packs", "fixture")
	candidateRoot := filepath.Join(packRoot, "promote-candidates")
	proofRoot := filepath.Join(candidateRoot, "review-artifacts")
	caseRoot := filepath.Join(repo, "case")
	packetPath := filepath.Join(caseRoot, ".rekit", "reviews", "packet.json")
	decisionPath := filepath.Join(caseRoot, ".rekit", "reviews", "decisions.json")
	packetHash := "packet-hash"
	decisionHash := "decision-hash"
	retirementID := shortReleaseHandoffHash(packetHash + decisionHash)
	workspace := filepath.Join(caseRoot, ".rekit", "verifications", "candidate-decisions", retirementID)
	freshRoot := filepath.Join(workspace, "fresh")
	attachedRoot := filepath.Join(workspace, "attached")
	provisionIntentPath := filepath.Join(workspace, "provision.intent.json")
	provisionReceiptPath := filepath.Join(workspace, "provision.receipt.json")
	verificationProofPath := filepath.Join(proofRoot, "fixture.candidate-verification-proof.json")
	decisionReceiptPath := filepath.Join(proofRoot, "fixture.candidate-decision-receipt.json")
	retirementIntentPath := filepath.Join(proofRoot, retirementID+".candidate-verification-retirement-intent.json")
	retirementReceiptPath := filepath.Join(proofRoot, retirementID+".candidate-verification-retirement-receipt.json")
	backupRoot := filepath.Join(candidateRoot, ".decision-backup", "fixture")
	candidatePath := filepath.Join(candidateRoot, "memory.candidate.md")
	candidateBackupPath := filepath.Join(backupRoot, "actions", "000", "candidate")
	packTarget := filepath.Join(packRoot, "memory.md")
	targetContent := "accepted memory content\n"

	writeFile(t, filepath.Join(packRoot, "manifest.yml"), `schemaVersion: 1
name: fixture
version: 1.0.0
description: completed pack-memory fixture
maturity: experimental
managedFiles:
  - memory.md
`)
	writeFile(t, filepath.Join(backupRoot, "committed.json"), "{\"applied\":true}\n")
	writeFile(t, filepath.Join(backupRoot, "transaction.json"), "{\"kind\":\"transaction\"}\n")
	writeFile(t, provisionIntentPath, "{\"kind\":\"intent\"}\n")
	writeFile(t, provisionReceiptPath, "{\"kind\":\"receipt\"}\n")
	writeFile(t, candidateBackupPath, targetContent)
	writeFile(t, packTarget, targetContent)

	actions := []map[string]any{{
		"candidatePath":       candidatePath,
		"kind":                "managed-doc",
		"decision":            "accept",
		"packTarget":          packTarget,
		"action":              "replace pack target with reviewed candidate",
		"candidateBackupPath": candidateBackupPath,
		"evidenceRefs":        []string{},
	}}
	receipt := map[string]any{
		"schemaVersion": 1, "kind": "pack-memory-candidate-decision-receipt", "pack": "fixture", "repoRoot": repo, "caseRoot": caseRoot,
		"packetPath": packetPath, "decisionPath": decisionPath, "packetHash": packetHash, "decisionHash": decisionHash, "backupRoot": backupRoot,
		"indexPath": filepath.Join(candidateRoot, "index.json"), "accepted": 1, "rejected": 0, "superseded": 0, "actions": actions,
		"decisionEvidence": []string{}, "receiptPath": decisionReceiptPath, "verificationPending": true, "verificationWorkspaceRoot": workspace,
		"verificationProvisionCommand": "/rekit promote -PacketPath " + packetPath + " -CandidateDecisionPath " + decisionPath + " -ProvisionCandidateVerificationCases -WhatIf",
		"verificationCommand":          "/rekit promote -VerifyCandidateDecision -WhatIf", "verificationProofPath": verificationProofPath, "boundary": []string{"fixture boundary"},
	}
	receiptData, err := json.MarshalIndent(receipt, "", "  ")
	if err != nil {
		t.Fatal(err)
	}
	receiptData = append(receiptData, '\n')
	writeFile(t, decisionReceiptPath, string(receiptData))

	verification := map[string]any{
		"schemaVersion": 1, "kind": "pack-memory-candidate-decision-verification", "pack": "fixture",
		"packetHash": packetHash, "decisionHash": decisionHash, "receiptHash": sha256ReleaseHandoff(receiptData),
		"provisionIntentSha256": fileSHA256ReleaseHandoff(provisionIntentPath), "provisionReceiptSha256": fileSHA256ReleaseHandoff(provisionReceiptPath),
		"isMutation": true, "applied": true, "ready": true, "packDoctorRows": 1, "freshDoctorRows": 1, "attachedDoctorRows": 1,
		"verifiedActionsSha256": candidateDecisionActionsSHA256([]candidateDecisionActionInventory{{
			CandidatePath: candidatePath, Kind: "managed-doc", Decision: "accept", PackTarget: packTarget, Action: "replace pack target with reviewed candidate", CandidateBackupPath: candidateBackupPath, EvidenceRefs: []string{},
		}}),
		"nextSteps": []string{"preview retirement"}, "boundary": []string{"fixture boundary"},
	}
	verificationData, err := json.MarshalIndent(verification, "", "  ")
	if err != nil {
		t.Fatal(err)
	}
	verificationData = append(verificationData, '\n')
	writeFile(t, verificationProofPath, string(verificationData))

	ownedContent := "owned verification artifact\n"
	freshOwned := filepath.Join(freshRoot, "owned.txt")
	attachedOwned := filepath.Join(attachedRoot, "owned.txt")
	writeFile(t, freshOwned, ownedContent)
	writeFile(t, attachedOwned, ownedContent)
	ownedHash := sha256ReleaseHandoff([]byte(ownedContent))
	retirement := candidateVerificationRetirementArtifactInventory{
		SchemaVersion: 1, Kind: "pack-memory-candidate-verification-retirement-intent", RepoRoot: repo, SourceCaseRoot: caseRoot, Pack: "fixture",
		PacketPath: packetPath, PacketSHA256: packetHash, DecisionPath: decisionPath, DecisionSHA256: decisionHash,
		DecisionReceiptPath: decisionReceiptPath, DecisionReceiptSHA256: sha256ReleaseHandoff(receiptData), VerificationProofPath: verificationProofPath,
		VerificationProofSHA256: sha256ReleaseHandoff(verificationData), ProvisionIntentPath: provisionIntentPath,
		ProvisionIntentSHA256: fileSHA256ReleaseHandoff(provisionIntentPath), ProvisionReceiptPath: provisionReceiptPath,
		ProvisionReceiptSHA256: fileSHA256ReleaseHandoff(provisionReceiptPath), WorkspaceRoot: workspace,
		RetirementIntentPath: retirementIntentPath, RetirementReceiptPath: retirementReceiptPath,
		Roots: []candidateVerificationRetirementRootInventory{
			{Role: "fresh", CaseRoot: freshRoot, Deletes: []string{freshOwned}},
			{Role: "attached", CaseRoot: attachedRoot, Deletes: []string{attachedOwned}},
		},
		ProvisionArtifactsToDelete: []string{provisionReceiptPath, provisionIntentPath},
		EmptyAncestorsToRemove:     []string{filepath.Dir(workspace), filepath.Dir(filepath.Dir(workspace))},
		Boundary:                   []string{"fixture boundary"},
		RetirementPlans: []syncpkg.ExclusiveInitRetirementPlan{
			{SchemaVersion: 1, Command: "exclusive-init-retirement", CaseRoot: freshRoot, ProvisionID: retirementID, Role: "fresh", Leaves: []syncpkg.ExclusiveInitRetirementLeaf{{Path: "owned.txt", SHA256: ownedHash, Size: int64(len(ownedContent))}}},
			{SchemaVersion: 1, Command: "exclusive-init-retirement", CaseRoot: attachedRoot, ProvisionID: retirementID, Role: "attached", Leaves: []syncpkg.ExclusiveInitRetirementLeaf{{Path: "owned.txt", SHA256: ownedHash, Size: int64(len(ownedContent))}}},
		},
	}
	retirement.RetirementSHA256 = candidateVerificationRetirementHash(candidateVerificationRetirementResultFromInventory(retirement, "-intent"))
	writeRetirement := func(path, kind string) {
		t.Helper()
		retirement.Kind = kind
		data, marshalErr := json.MarshalIndent(retirement, "", "  ")
		if marshalErr != nil {
			t.Fatal(marshalErr)
		}
		writeFile(t, path, string(data)+"\n")
	}
	writeRetirement(retirementIntentPath, "pack-memory-candidate-verification-retirement-intent")
	writeRetirement(retirementReceiptPath, "pack-memory-candidate-verification-retirement-receipt")

	cleanupEvidence := filepath.Join(caseRoot, "cleanup-evidence.md")
	writeFile(t, cleanupEvidence, "cleanup proof evidence\n")
	writeCandidateCleanupProof(t, repo, caseRoot, filepath.Join(proofRoot, "memory.candidate-cleanup-proof.json"), "fixture", packetHash, decisionHash, decisionReceiptPath, filepath.Join(backupRoot, "transaction.json"), filepath.Join(backupRoot, "committed.json"), candidatePath, candidateBackupPath, packTarget, "", filepath.Join(candidateRoot, "index.json"), "accept", "managed-doc", cleanupEvidence)
	for proofType, checks := range map[string][]map[string]any{
		"pack-doctor-output":            {{"name": "pack-doctor", "status": "passed", "summary": "doctor passed"}},
		"fresh-case-reconsume-proof":    {{"name": "fresh-case-reconsume", "status": "passed", "summary": "fresh case passed"}, {"name": "pack-doctor", "status": "passed", "summary": "doctor passed"}},
		"attached-case-reconsume-proof": {{"name": "attached-case-reconsume", "status": "passed", "summary": "attached case passed"}, {"name": "pack-doctor", "status": "passed", "summary": "doctor passed"}},
	} {
		writeCandidateLifecycleProof(t, repo, filepath.Join(proofRoot, "memory."+proofType+".json"), "fixture", proofType, candidatePath, packTarget, checks, verificationProofPath)
	}

	for _, path := range []string{freshOwned, attachedOwned, freshRoot, attachedRoot, provisionReceiptPath, provisionIntentPath, workspace} {
		if err := os.Remove(path); err != nil && !os.IsNotExist(err) {
			t.Fatal(err)
		}
	}
	if strings.TrimSpace(fileSHA256ReleaseHandoff(packTarget)) == "" {
		t.Fatal("fixture pack target hash is empty")
	}
	return completedPackMemoryChangeFixture{repo: repo, candidateRoot: candidateRoot, proofRoot: proofRoot, targetContent: targetContent}
}
