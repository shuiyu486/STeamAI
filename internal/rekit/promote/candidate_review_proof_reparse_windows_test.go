//go:build windows

package promote

import (
	"bytes"
	"encoding/json"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

func TestDraftCandidateReviewProofRejectsJunctionBeforeMutation(t *testing.T) {
	repoRoot, caseRoot, pack := promoteFixture(t)
	created, _, managed := candidateDecisionFixture(t, repoRoot, caseRoot, pack, "proof-junction")
	reviewArtifacts := filepath.Join(repoRoot, "packs", pack, "promote-candidates", "review-artifacts")
	if err := os.MkdirAll(reviewArtifacts, 0o755); err != nil {
		t.Fatal(err)
	}
	opt := CandidateReviewProofDraftOptions{
		PacketPath:    created.ReviewWorkspace.PacketPath,
		ProofPath:     filepath.Join(reviewArtifacts, "junction-safe-preview.json"),
		ProofType:     "candidate-decision-note",
		CandidatePath: managed.CandidatePath,
		Decision:      "accept",
		Reason:        "junction alias must fail closed",
		Actor:         "mission-commander",
		EvidenceRefs:  created.ReviewWorkspace.CombinedDiffPath,
		WhatIf:        true,
	}
	preview, err := DraftCandidateReviewProof(repoRoot, caseRoot, pack, opt)
	if err != nil {
		t.Fatal(err)
	}
	proofBytes, err := json.MarshalIndent(preview.Proof, "", "  ")
	if err != nil {
		t.Fatal(err)
	}
	proofBytes = append(proofBytes, '\n')
	if preview.ProofSHA256 == "" || sha256Hex(proofBytes) != preview.ProofSHA256 {
		t.Fatalf("junction proof preview bytes do not match proof hash: %s", preview.ProofSHA256)
	}
	outside := t.TempDir()
	outsideProofPath := filepath.Join(outside, "nested", "proof.json")
	if err := os.MkdirAll(filepath.Dir(outsideProofPath), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(outsideProofPath, proofBytes, 0o644); err != nil {
		t.Fatal(err)
	}
	junction := filepath.Join(reviewArtifacts, "junction")
	if output, err := exec.Command("cmd.exe", "/c", "mklink", "/J", junction, outside).CombinedOutput(); err != nil {
		t.Skipf("Windows directory junction unavailable: %v (%s)", err, strings.TrimSpace(string(output)))
	}
	opt.ProofPath = filepath.Join(junction, "nested", "proof.json")
	if _, err := DraftCandidateReviewProof(repoRoot, caseRoot, pack, opt); err == nil {
		t.Fatal("proof WhatIf accepted exact bytes through junction")
	}
	opt.WhatIf = false
	opt.ExpectedProofSHA256 = preview.ProofSHA256
	if _, err := DraftCandidateReviewProof(repoRoot, caseRoot, pack, opt); err == nil {
		t.Fatal("proof draft through junction unexpectedly succeeded")
	}
	if outsideBytes, readErr := os.ReadFile(outsideProofPath); readErr != nil || !bytes.Equal(outsideBytes, proofBytes) {
		t.Fatalf("proof junction changed exact outside proof: err=%v bytes=%q", readErr, outsideBytes)
	}
	entries, readErr := os.ReadDir(filepath.Dir(outsideProofPath))
	if readErr != nil || len(entries) != 1 || entries[0].Name() != filepath.Base(outsideProofPath) {
		t.Fatalf("proof junction created outside side effects: entries=%v err=%v", entries, readErr)
	}
	if _, err := os.Lstat(junction); err != nil {
		t.Fatalf("proof junction was modified or removed: %v", err)
	}
}
