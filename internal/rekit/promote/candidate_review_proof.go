package promote

import (
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	refsf "github.com/shuiyu486/re-context-kits/internal/rekit/fs"
	"github.com/shuiyu486/re-context-kits/internal/rekit/instance"
	"github.com/shuiyu486/re-context-kits/internal/rekit/manifest"
)

type CandidateReviewProofDraftOptions struct {
	PacketPath          string
	DecisionPath        string
	ProofPath           string
	ProofType           string
	CandidatePath       string
	Decision            string
	Reason              string
	Actor               string
	EvidenceRefs        string
	ExpectedProofSHA256 string
	WhatIf              bool
}

type CandidateReviewProofDraftResult struct {
	SchemaVersion  int                      `json:"schemaVersion"`
	Command        string                   `json:"command"`
	Kind           string                   `json:"kind"`
	Mode           string                   `json:"mode"`
	CaseRoot       string                   `json:"caseRoot"`
	RepoRoot       string                   `json:"repoRoot"`
	Pack           string                   `json:"pack"`
	PacketPath     string                   `json:"packetPath"`
	DecisionPath   string                   `json:"decisionPath,omitempty"`
	PacketHash     string                   `json:"packetHash"`
	DecisionHash   string                   `json:"decisionHash,omitempty"`
	ProofPath      string                   `json:"proofPath"`
	ProofType      string                   `json:"proofType"`
	ProofSHA256    string                   `json:"proofSha256"`
	CandidatePath  string                   `json:"candidatePath"`
	PackTarget     string                   `json:"packTarget,omitempty"`
	Decision       string                   `json:"decision"`
	Reason         string                   `json:"reason"`
	Actor          string                   `json:"actor"`
	IsMutation     bool                     `json:"isMutation"`
	Applied        bool                     `json:"applied"`
	AlreadyWritten bool                     `json:"alreadyWritten,omitempty"`
	Proof          CandidateReviewProofNote `json:"proof"`
	PreviewCommand string                   `json:"previewCommand,omitempty"`
	ApplyCommand   string                   `json:"applyCommand,omitempty"`
	NextSteps      []string                 `json:"nextSteps"`
	Boundary       []string                 `json:"boundary"`
}

type CandidateReviewCleanupProof struct {
	DecisionReceiptPath string `json:"decisionReceiptPath"`
	DecisionReceiptHash string `json:"decisionReceiptHash"`
	TransactionPath     string `json:"transactionPath"`
	TransactionHash     string `json:"transactionHash"`
	CommittedPath       string `json:"committedPath"`
	CommittedHash       string `json:"committedHash"`
	CandidateBackupPath string `json:"candidateBackupPath"`
	CandidateBackupHash string `json:"candidateBackupHash"`
	TargetBackupPath    string `json:"targetBackupPath,omitempty"`
	TargetBackupHash    string `json:"targetBackupHash,omitempty"`
	IndexPath           string `json:"indexPath,omitempty"`
	IndexPresent        bool   `json:"indexPresent"`
	IndexEntryAbsent    bool   `json:"indexEntryAbsent"`
	CandidateAbsent     bool   `json:"candidateAbsent"`
	PackTargetHash      string `json:"packTargetHash,omitempty"`
}

type CandidateReviewProofNote struct {
	SchemaVersion  int                               `json:"schemaVersion"`
	Kind           string                            `json:"kind"`
	Pack           string                            `json:"pack"`
	PacketHash     string                            `json:"packetHash"`
	DecisionHash   string                            `json:"decisionHash,omitempty"`
	ProofType      string                            `json:"proofType"`
	CandidatePath  string                            `json:"candidatePath"`
	CandidateHash  string                            `json:"candidateHash"`
	PackTarget     string                            `json:"packTarget,omitempty"`
	PackTargetHash string                            `json:"packTargetHash,omitempty"`
	Decision       string                            `json:"decision"`
	Reason         string                            `json:"reason"`
	Actor          string                            `json:"actor"`
	EvidenceRefs   []CandidateDecisionEvidence       `json:"evidenceRefs"`
	ReviewItem     CandidateReviewProofReviewItemRef `json:"reviewItem"`
	Cleanup        *CandidateReviewCleanupProof      `json:"cleanup,omitempty"`
	Boundary       []string                          `json:"boundary"`
}

type CandidateReviewProofReviewItemRef struct {
	CandidatePath string `json:"candidatePath"`
	CandidateHash string `json:"candidateHash"`
	PackTarget    string `json:"packTarget,omitempty"`
	Kind          string `json:"kind"`
}

type preparedCandidateReviewProofDraft struct {
	result     CandidateReviewProofDraftResult
	proofBytes []byte
}

func DraftCandidateReviewProof(repoRoot, caseRoot, pack string, opt CandidateReviewProofDraftOptions) (CandidateReviewProofDraftResult, error) {
	inst, err := instance.AssertAttached(caseRoot, repoRoot, pack)
	if err != nil {
		return CandidateReviewProofDraftResult{}, err
	}
	if opt.WhatIf && strings.TrimSpace(opt.ExpectedProofSHA256) != "" {
		return CandidateReviewProofDraftResult{}, fmt.Errorf("candidate review proof draft WhatIf does not accept -ExpectedProofSha256")
	}
	if !opt.WhatIf {
		decoded, decodeErr := hex.DecodeString(strings.TrimSpace(opt.ExpectedProofSHA256))
		if decodeErr != nil || len(decoded) != sha256.Size {
			return CandidateReviewProofDraftResult{}, fmt.Errorf("candidate review proof draft Apply requires a valid -ExpectedProofSha256 from WhatIf")
		}
	}
	prepared, err := prepareCandidateReviewProofDraft(repoRoot, inst.CaseRoot, pack, opt)
	if err != nil {
		return CandidateReviewProofDraftResult{}, err
	}
	result := prepared.result
	if opt.WhatIf {
		return result, nil
	}
	if !strings.EqualFold(result.ProofSHA256, strings.TrimSpace(opt.ExpectedProofSHA256)) {
		return CandidateReviewProofDraftResult{}, fmt.Errorf("candidate review proof draft changed after preview")
	}
	already, err := writeCandidateReviewProofDraftFile(repoRoot, result.ProofPath, prepared.proofBytes)
	if err != nil {
		return CandidateReviewProofDraftResult{}, err
	}
	result.Applied = true
	result.AlreadyWritten = already
	if !already {
		result.Mode = "proof-drafted"
		result.NextSteps = []string{"review the drafted proof note", "rerun release-check or status to refresh pack-memory proof summary", "continue candidate decision/cleanup/reconsume flow only after required proof is present"}
	}
	return result, nil
}

func prepareCandidateReviewProofDraft(repoRoot, caseRoot, pack string, opt CandidateReviewProofDraftOptions) (preparedCandidateReviewProofDraft, error) {
	packetPath, packetBytes, err := readStrictCandidateDecisionFile(opt.PacketPath, "candidate review packet")
	if err != nil {
		return preparedCandidateReviewProofDraft{}, err
	}
	var packet CandidateReviewPacket
	if err := decodeStrictJSON(packetBytes, &packet); err != nil {
		return preparedCandidateReviewProofDraft{}, fmt.Errorf("decode candidate review packet: %w", err)
	}
	if packet.SchemaVersion != 1 || packet.Kind != "pack-memory-candidate-review" || packet.Command != "promote" || packet.CandidateResult.Pack != pack || !sameCandidateDecisionPath(packet.CandidateResult.RepoRoot, repoRoot) || !sameCandidateDecisionPath(packet.CandidateResult.CaseRoot, caseRoot) {
		return preparedCandidateReviewProofDraft{}, fmt.Errorf("candidate review packet binding mismatch")
	}
	m, err := manifest.Load(repoRoot, pack)
	if err != nil {
		return preparedCandidateReviewProofDraft{}, err
	}
	canonicalCandidateRoot := filepath.Join(m.PackRoot, "promote-candidates")
	canonicalToolingRoot := filepath.Join(m.PackRoot, "tooling", "candidates")
	if !sameCandidateDecisionPath(packet.CandidateResult.CandidateRoot, canonicalCandidateRoot) || !sameCandidateDecisionPath(packet.CandidateResult.ToolingRoot, canonicalToolingRoot) {
		return preparedCandidateReviewProofDraft{}, fmt.Errorf("candidate review packet roots do not match canonical pack candidate roots")
	}
	proofType := strings.TrimSpace(opt.ProofType)
	if proofType == "" {
		proofType = "candidate-decision-note"
	}
	switch proofType {
	case "candidate-decision-note", "candidate-cleanup-proof":
	default:
		return preparedCandidateReviewProofDraft{}, fmt.Errorf("candidate review proof draft supports only -ProofType candidate-decision-note or candidate-cleanup-proof")
	}
	candidatePath := strings.TrimSpace(opt.CandidatePath)
	if candidatePath == "" {
		return preparedCandidateReviewProofDraft{}, fmt.Errorf("candidate review proof draft requires -CandidatePath")
	}
	if proofType == "candidate-cleanup-proof" {
		return prepareCandidateCleanupReviewProofDraft(repoRoot, caseRoot, pack, opt, packetPath, packetBytes)
	}
	decision := strings.ToLower(strings.TrimSpace(opt.Decision))
	switch decision {
	case "accept", "reject", "superseded":
	case "":
		return preparedCandidateReviewProofDraft{}, fmt.Errorf("candidate review proof draft requires -Decision accept|reject|superseded")
	default:
		return preparedCandidateReviewProofDraft{}, fmt.Errorf("candidate review proof draft requires a per-candidate decision: accept, reject, or superseded")
	}
	reviewItem, err := candidateReviewProofItem(packet, candidatePath, repoRoot)
	if err != nil {
		return preparedCandidateReviewProofDraft{}, err
	}
	reviewItemRoot := canonicalCandidateRoot
	if reviewItem.Kind == "tooling-candidate-source" {
		reviewItemRoot = canonicalToolingRoot
	}
	if err := assertInsideRoot(reviewItemRoot, reviewItem.CandidatePath); err != nil {
		return preparedCandidateReviewProofDraft{}, err
	}
	if _, err := os.Lstat(reviewItem.CandidatePath); err != nil {
		return preparedCandidateReviewProofDraft{}, fmt.Errorf("candidate %s must be a non-empty regular file: %w", reviewItem.CandidatePath, err)
	}
	if err := rejectCandidateDecisionSymlinkPath(m.PackRoot, reviewItem.CandidatePath, false); err != nil {
		return preparedCandidateReviewProofDraft{}, err
	}
	state, err := refsf.ClassifyNonEmptyRegularFile(reviewItem.CandidatePath)
	if err != nil {
		return preparedCandidateReviewProofDraft{}, fmt.Errorf("candidate %s must be a non-empty regular file: %w", reviewItem.CandidatePath, err)
	}
	if state != refsf.RegularFileReady {
		return preparedCandidateReviewProofDraft{}, fmt.Errorf("candidate %s must be a non-empty regular file, got %s", reviewItem.CandidatePath, state)
	}
	if reviewItem.Kind == "tooling-candidate-source" && decision == "accept" {
		return preparedCandidateReviewProofDraft{}, fmt.Errorf("candidate review proof draft cannot accept tooling candidates automatically; use reject or superseded, or record manual tooling merge proof")
	}
	if decision == "accept" {
		if reviewItem.Kind != "managed-doc" || strings.TrimSpace(reviewItem.PackTarget) == "" {
			return preparedCandidateReviewProofDraft{}, fmt.Errorf("candidate review proof draft accept requires a managed-doc packTarget")
		}
		if err := assertInsideRoot(m.PackRoot, reviewItem.PackTarget); err != nil {
			return preparedCandidateReviewProofDraft{}, err
		}
		if err := rejectCandidateDecisionSymlinkPath(m.PackRoot, reviewItem.PackTarget, false); err != nil {
			return preparedCandidateReviewProofDraft{}, err
		}
		state, err := refsf.ClassifyNonEmptyRegularFile(reviewItem.PackTarget)
		if err != nil {
			return preparedCandidateReviewProofDraft{}, fmt.Errorf("packTarget %s must be a non-empty regular file: %w", reviewItem.PackTarget, err)
		}
		if state != refsf.RegularFileReady {
			return preparedCandidateReviewProofDraft{}, fmt.Errorf("packTarget %s must be a non-empty regular file, got %s", reviewItem.PackTarget, state)
		}
	}
	if strings.TrimSpace(opt.Reason) == "" {
		return preparedCandidateReviewProofDraft{}, fmt.Errorf("candidate review proof draft requires -Reason")
	}
	if strings.TrimSpace(opt.Actor) == "" {
		return preparedCandidateReviewProofDraft{}, fmt.Errorf("candidate review proof draft requires -Actor")
	}
	evidenceRefs, err := parseCandidateReviewProofEvidenceRefs(repoRoot, caseRoot, opt.EvidenceRefs)
	if err != nil {
		return preparedCandidateReviewProofDraft{}, err
	}
	if len(evidenceRefs) == 0 {
		return preparedCandidateReviewProofDraft{}, fmt.Errorf("candidate review proof draft requires at least one -EvidenceRefs item")
	}
	proofPath, err := candidateReviewProofDraftPath(repoRoot, packet.CandidateResult.CandidateRoot, opt.ProofPath, proofType, reviewItem.CandidatePath)
	if err != nil {
		return preparedCandidateReviewProofDraft{}, err
	}
	packetHash := sha256Hex(packetBytes)
	proofReviewItem := reviewItem
	proofReviewItem.CandidatePath = candidateReviewProofRepoRelative(repoRoot, reviewItem.CandidatePath)
	proofReviewItem.PackTarget = candidateReviewProofRepoRelative(repoRoot, reviewItem.PackTarget)
	proof := CandidateReviewProofNote{
		SchemaVersion:  1,
		Kind:           "pack-memory-candidate-review-proof",
		Pack:           pack,
		PacketHash:     packetHash,
		ProofType:      proofType,
		CandidatePath:  proofReviewItem.CandidatePath,
		CandidateHash:  reviewItem.CandidateHash,
		PackTarget:     proofReviewItem.PackTarget,
		PackTargetHash: candidateReviewProofPackTargetHash(m.PackRoot, reviewItem.PackTarget, decision),
		Decision:       decision,
		Reason:         strings.TrimSpace(opt.Reason),
		Actor:          strings.TrimSpace(opt.Actor),
		EvidenceRefs:   evidenceRefs,
		ReviewItem:     proofReviewItem,
		Boundary:       candidateReviewProofDraftBoundary(),
	}
	proofBytes, err := json.MarshalIndent(proof, "", "  ")
	if err != nil {
		return preparedCandidateReviewProofDraft{}, err
	}
	proofBytes = append(proofBytes, '\n')
	proofSHA256 := sha256Hex(proofBytes)
	previewCommand := candidateReviewProofDraftCommand(packetPath, "", proofPath, proofType, reviewItem.CandidatePath, decision, proof.Reason, proof.Actor, opt.EvidenceRefs, "", false)
	applyCommand := candidateReviewProofDraftCommand(packetPath, "", proofPath, proofType, reviewItem.CandidatePath, decision, proof.Reason, proof.Actor, opt.EvidenceRefs, proofSHA256, true)
	result := CandidateReviewProofDraftResult{
		SchemaVersion:  1,
		Command:        "promote",
		Kind:           "pack-memory-candidate-review-proof-draft",
		Mode:           "proof-draft-preview",
		CaseRoot:       caseRoot,
		RepoRoot:       repoRoot,
		Pack:           pack,
		PacketPath:     packetPath,
		PacketHash:     packetHash,
		ProofPath:      proofPath,
		ProofType:      proofType,
		ProofSHA256:    proofSHA256,
		CandidatePath:  reviewItem.CandidatePath,
		PackTarget:     reviewItem.PackTarget,
		Decision:       decision,
		Reason:         proof.Reason,
		Actor:          proof.Actor,
		IsMutation:     !opt.WhatIf,
		Applied:        false,
		Proof:          proof,
		PreviewCommand: previewCommand,
		ApplyCommand:   applyCommand,
		NextSteps:      []string{"review the deterministic proof note and exact hash", "write only the proof note with the returned expected-hash Apply command", "rerun release-check or status to refresh pack-memory proof summary"},
		Boundary:       candidateReviewProofDraftBoundary(),
	}
	if existing, err := os.ReadFile(proofPath); err == nil {
		if bytes.Equal(existing, proofBytes) {
			result.Mode = "already-drafted"
			result.ApplyCommand = ""
			result.NextSteps = []string{"the exact proof note already exists", "rerun release-check or status to refresh pack-memory proof summary"}
		} else {
			return preparedCandidateReviewProofDraft{}, fmt.Errorf("candidate review proof draft target already exists with different bytes: %s", proofPath)
		}
	} else if !os.IsNotExist(err) {
		return preparedCandidateReviewProofDraft{}, err
	}
	return preparedCandidateReviewProofDraft{result: result, proofBytes: proofBytes}, nil
}

func prepareCandidateCleanupReviewProofDraft(repoRoot, caseRoot, pack string, opt CandidateReviewProofDraftOptions, packetPath string, packetBytes []byte) (preparedCandidateReviewProofDraft, error) {
	if strings.TrimSpace(opt.DecisionPath) == "" {
		return preparedCandidateReviewProofDraft{}, fmt.Errorf("candidate cleanup proof draft requires -CandidateDecisionPath")
	}
	decisionPath, decisionBytes, err := readStrictCandidateDecisionFile(opt.DecisionPath, "candidate decision")
	if err != nil {
		return preparedCandidateReviewProofDraft{}, err
	}
	authority, err := loadCandidateDecisionAuthority(repoRoot, caseRoot, pack, packetPath, decisionPath)
	if err != nil {
		return preparedCandidateReviewProofDraft{}, err
	}
	packetHash := sha256Hex(packetBytes)
	decisionHash := sha256Hex(decisionBytes)
	if !strings.EqualFold(authority.packetHash, packetHash) || !strings.EqualFold(authority.decisionHash, decisionHash) {
		return preparedCandidateReviewProofDraft{}, fmt.Errorf("candidate cleanup proof draft packet/decision binding mismatch")
	}
	candidatePath := candidateReviewProofResolveRepoPath(repoRoot, opt.CandidatePath)
	action, ok := candidateReviewCleanupProofAction(authority.receipt.Actions, candidatePath)
	if !ok {
		return preparedCandidateReviewProofDraft{}, fmt.Errorf("candidate cleanup proof draft candidatePath not found in decision receipt actions: %s", candidatePath)
	}
	if _, err := os.Lstat(action.CandidatePath); err == nil || !os.IsNotExist(err) {
		return preparedCandidateReviewProofDraft{}, fmt.Errorf("reviewed candidate cleanup is incomplete: %s", action.CandidatePath)
	}
	indexEntryAbsent := true
	if action.Kind == "managed-doc" {
		indexEntryAbsent = !candidateIndexStillContains(authority.receipt.IndexPath, action.CandidatePath)
		if !indexEntryAbsent {
			return preparedCandidateReviewProofDraft{}, fmt.Errorf("reviewed candidate index cleanup is incomplete: %s", action.CandidatePath)
		}
	}
	if strings.TrimSpace(opt.Reason) == "" {
		return preparedCandidateReviewProofDraft{}, fmt.Errorf("candidate cleanup proof draft requires -Reason")
	}
	if strings.TrimSpace(opt.Actor) == "" {
		return preparedCandidateReviewProofDraft{}, fmt.Errorf("candidate cleanup proof draft requires -Actor")
	}
	evidenceInput := strings.TrimSpace(opt.EvidenceRefs)
	if evidenceInput == "" {
		evidenceInput = strings.Join(action.EvidenceRefs, ",")
	}
	evidenceRefs, err := parseCandidateReviewProofEvidenceRefs(repoRoot, caseRoot, evidenceInput)
	if err != nil {
		return preparedCandidateReviewProofDraft{}, err
	}
	if len(evidenceRefs) == 0 {
		return preparedCandidateReviewProofDraft{}, fmt.Errorf("candidate cleanup proof draft requires at least one receipt or -EvidenceRefs item")
	}
	proofPath, err := candidateReviewProofDraftPath(repoRoot, authority.candidateRoot, opt.ProofPath, "candidate-cleanup-proof", action.CandidatePath)
	if err != nil {
		return preparedCandidateReviewProofDraft{}, err
	}
	receiptHash := sha256Hex(authority.receiptBytes)
	transactionPath := filepath.Join(authority.receipt.BackupRoot, "transaction.json")
	_, transactionBytes, err := readStrictCandidateDecisionFile(transactionPath, "candidate decision transaction")
	if err != nil {
		return preparedCandidateReviewProofDraft{}, err
	}
	committedPath := filepath.Join(authority.receipt.BackupRoot, "committed.json")
	_, committedBytes, err := readStrictCandidateDecisionFile(committedPath, "candidate decision committed marker")
	if err != nil {
		return preparedCandidateReviewProofDraft{}, err
	}
	indexPresent := false
	if info, err := os.Lstat(authority.receipt.IndexPath); err == nil {
		if info.Mode()&os.ModeSymlink != 0 || !info.Mode().IsRegular() {
			return preparedCandidateReviewProofDraft{}, fmt.Errorf("candidate cleanup proof indexPath must be a regular file when present: %s", authority.receipt.IndexPath)
		}
		indexPresent = true
	} else if !os.IsNotExist(err) {
		return preparedCandidateReviewProofDraft{}, err
	}
	backupHash := fileSHA256(action.CandidateBackupPath)
	cleanup := CandidateReviewCleanupProof{
		DecisionReceiptPath: candidateReviewProofRepoRelative(repoRoot, authority.receiptPath),
		DecisionReceiptHash: receiptHash,
		TransactionPath:     candidateReviewProofRepoRelative(repoRoot, transactionPath),
		TransactionHash:     sha256Hex(transactionBytes),
		CommittedPath:       candidateReviewProofRepoRelative(repoRoot, committedPath),
		CommittedHash:       sha256Hex(committedBytes),
		CandidateBackupPath: candidateReviewProofRepoRelative(repoRoot, action.CandidateBackupPath),
		CandidateBackupHash: backupHash,
		TargetBackupPath:    candidateReviewProofRepoRelative(repoRoot, action.TargetBackupPath),
		TargetBackupHash:    fileSHA256(action.TargetBackupPath),
		IndexPath:           candidateReviewProofRepoRelative(repoRoot, authority.receipt.IndexPath),
		IndexPresent:        indexPresent,
		IndexEntryAbsent:    indexEntryAbsent,
		CandidateAbsent:     true,
	}
	if action.Decision == "accept" {
		cleanup.PackTargetHash = fileSHA256(action.PackTarget)
	}
	proofReviewItem := CandidateReviewProofReviewItemRef{CandidatePath: candidateReviewProofRepoRelative(repoRoot, action.CandidatePath), CandidateHash: backupHash, PackTarget: candidateReviewProofRepoRelative(repoRoot, action.PackTarget), Kind: action.Kind}
	proof := CandidateReviewProofNote{
		SchemaVersion:  1,
		Kind:           "pack-memory-candidate-review-proof",
		Pack:           pack,
		PacketHash:     packetHash,
		DecisionHash:   decisionHash,
		ProofType:      "candidate-cleanup-proof",
		CandidatePath:  proofReviewItem.CandidatePath,
		CandidateHash:  backupHash,
		PackTarget:     proofReviewItem.PackTarget,
		PackTargetHash: cleanup.PackTargetHash,
		Decision:       action.Decision,
		Reason:         strings.TrimSpace(opt.Reason),
		Actor:          strings.TrimSpace(opt.Actor),
		EvidenceRefs:   evidenceRefs,
		ReviewItem:     proofReviewItem,
		Cleanup:        &cleanup,
		Boundary:       candidateReviewProofDraftBoundary(),
	}
	proofBytes, err := json.MarshalIndent(proof, "", "  ")
	if err != nil {
		return preparedCandidateReviewProofDraft{}, err
	}
	proofBytes = append(proofBytes, '\n')
	proofSHA256 := sha256Hex(proofBytes)
	previewCommand := candidateReviewProofDraftCommand(packetPath, decisionPath, proofPath, "candidate-cleanup-proof", action.CandidatePath, action.Decision, proof.Reason, proof.Actor, evidenceInput, "", false)
	applyCommand := candidateReviewProofDraftCommand(packetPath, decisionPath, proofPath, "candidate-cleanup-proof", action.CandidatePath, action.Decision, proof.Reason, proof.Actor, evidenceInput, proofSHA256, true)
	result := CandidateReviewProofDraftResult{
		SchemaVersion:  1,
		Command:        "promote",
		Kind:           "pack-memory-candidate-review-proof-draft",
		Mode:           "proof-draft-preview",
		CaseRoot:       caseRoot,
		RepoRoot:       repoRoot,
		Pack:           pack,
		PacketPath:     packetPath,
		DecisionPath:   decisionPath,
		PacketHash:     packetHash,
		DecisionHash:   decisionHash,
		ProofPath:      proofPath,
		ProofType:      "candidate-cleanup-proof",
		ProofSHA256:    proofSHA256,
		CandidatePath:  action.CandidatePath,
		PackTarget:     action.PackTarget,
		Decision:       action.Decision,
		Reason:         proof.Reason,
		Actor:          proof.Actor,
		IsMutation:     !opt.WhatIf,
		Applied:        false,
		Proof:          proof,
		PreviewCommand: previewCommand,
		ApplyCommand:   applyCommand,
		NextSteps:      []string{"review the deterministic cleanup proof note and exact hash", "write only the cleanup proof note with the returned expected-hash Apply command", "rerun release-check or status to refresh pack-memory proof summary"},
		Boundary:       candidateReviewProofDraftBoundary(),
	}
	if existing, err := os.ReadFile(proofPath); err == nil {
		if bytes.Equal(existing, proofBytes) {
			result.Mode = "already-drafted"
			result.ApplyCommand = ""
			result.NextSteps = []string{"the exact cleanup proof note already exists", "rerun release-check or status to refresh pack-memory proof summary"}
		} else {
			return preparedCandidateReviewProofDraft{}, fmt.Errorf("candidate review proof draft target already exists with different bytes: %s", proofPath)
		}
	} else if !os.IsNotExist(err) {
		return preparedCandidateReviewProofDraft{}, err
	}
	return preparedCandidateReviewProofDraft{result: result, proofBytes: proofBytes}, nil
}

func candidateReviewCleanupProofAction(actions []CandidateDecisionAction, candidatePath string) (CandidateDecisionAction, bool) {
	for _, action := range actions {
		if candidateReviewProofSameCleanPath(action.CandidatePath, candidatePath) {
			return action, true
		}
	}
	return CandidateDecisionAction{}, false
}

func candidateReviewProofSameCleanPath(left, right string) bool {
	leftFull, leftErr := filepath.Abs(left)
	rightFull, rightErr := filepath.Abs(right)
	if leftErr != nil || rightErr != nil {
		return false
	}
	leftFull = filepath.Clean(leftFull)
	rightFull = filepath.Clean(rightFull)
	if strings.EqualFold(leftFull, rightFull) {
		return true
	}
	return sameCandidateDecisionPath(leftFull, rightFull)
}

func candidateReviewProofItem(packet CandidateReviewPacket, candidatePath, repoRoot string) (CandidateReviewProofReviewItemRef, error) {
	candidatePath = candidateReviewProofResolveRepoPath(repoRoot, candidatePath)
	for _, item := range packet.CandidateResult.ReviewPlan.ReviewItems {
		if sameCandidateDecisionPath(item.CandidatePath, candidatePath) || candidateReviewProofSameCleanPath(item.CandidatePath, candidatePath) {
			return CandidateReviewProofReviewItemRef{CandidatePath: item.CandidatePath, CandidateHash: fileSHA256(item.CandidatePath), PackTarget: item.PackTarget, Kind: item.Kind}, nil
		}
	}
	return CandidateReviewProofReviewItemRef{}, fmt.Errorf("candidate review proof draft candidatePath not found in packet reviewItems: %s", candidatePath)
}

func candidateReviewProofResolveRepoPath(repoRoot, path string) string {
	clean := filepath.Clean(strings.TrimSpace(path))
	if filepath.IsAbs(clean) {
		return clean
	}
	joined := filepath.Join(repoRoot, filepath.FromSlash(strings.TrimSpace(path)))
	if sameCandidateDecisionPath(joined, clean) {
		return clean
	}
	return joined
}

func candidateReviewProofRepoRelative(repoRoot, path string) string {
	if strings.TrimSpace(path) == "" {
		return ""
	}
	if rel, err := filepath.Rel(repoRoot, path); err == nil && rel != ".." && !strings.HasPrefix(rel, ".."+string(filepath.Separator)) && !filepath.IsAbs(rel) {
		return filepath.ToSlash(rel)
	}
	return filepath.ToSlash(path)
}

func candidateReviewProofPackTargetHash(packRoot, packTarget, decision string) string {
	if decision != "accept" || strings.TrimSpace(packTarget) == "" {
		return ""
	}
	if err := assertInsideRoot(packRoot, packTarget); err != nil {
		return ""
	}
	state, err := refsf.ClassifyNonEmptyRegularFile(packTarget)
	if err != nil || state != refsf.RegularFileReady {
		return ""
	}
	return fileSHA256(packTarget)
}

func parseCandidateReviewProofEvidenceRefs(repoRoot, caseRoot, value string) ([]CandidateDecisionEvidence, error) {
	refs := splitCandidateDecisionDraftList(value)
	out := []CandidateDecisionEvidence{}
	for _, ref := range refs {
		evidence, err := candidateDecisionDraftEvidenceRef(repoRoot, caseRoot, ref)
		if err != nil {
			return nil, err
		}
		evidence.Path = candidateReviewProofEvidencePath(repoRoot, caseRoot, evidence.Path)
		out = append(out, evidence)
	}
	return out, nil
}

func candidateReviewProofEvidencePath(repoRoot, caseRoot, path string) string {
	value := strings.TrimSpace(path)
	if value == "" {
		return ""
	}
	if filepath.IsAbs(value) {
		if rel, err := filepath.Rel(caseRoot, value); err == nil && rel != ".." && !strings.HasPrefix(rel, ".."+string(filepath.Separator)) && !filepath.IsAbs(rel) {
			return filepath.ToSlash(rel)
		}
		if rel, err := filepath.Rel(repoRoot, value); err == nil && rel != ".." && !strings.HasPrefix(rel, ".."+string(filepath.Separator)) && !filepath.IsAbs(rel) {
			return filepath.ToSlash(rel)
		}
	}
	return filepath.ToSlash(value)
}

func candidateReviewProofDraftPath(repoRoot, candidateRoot, value, proofType, candidatePath string) (string, error) {
	path := strings.TrimSpace(value)
	if path == "" {
		stem := strings.TrimSuffix(filepath.Base(filepath.ToSlash(candidatePath)), filepath.Ext(candidatePath))
		path = filepath.ToSlash(filepath.Join(candidateRoot, "review-artifacts", stem+"."+proofType+".json"))
	}
	full, err := filepath.Abs(candidateReviewProofResolveRepoPath(repoRoot, path))
	if err != nil {
		return "", err
	}
	if !pathWithinCandidateReviewRoot(repoRoot, candidateRoot, full) {
		return "", fmt.Errorf("candidate review proof path must stay under pack promote-candidates/review-artifacts: %s", path)
	}
	switch filepath.Ext(full) {
	case ".json", ".md", ".txt":
	default:
		return "", fmt.Errorf("candidate review proof draft path must end with .json, .md, or .txt: %s", path)
	}
	return full, nil
}

func pathWithinCandidateReviewRoot(repoRoot, candidateRoot, path string) bool {
	root := filepath.Join(candidateRoot, "review-artifacts")
	rootFull, err := filepath.Abs(root)
	if err != nil {
		return false
	}
	pathFull, err := filepath.Abs(path)
	if err != nil {
		return false
	}
	repoFull, err := filepath.Abs(repoRoot)
	if err != nil {
		return false
	}
	rootClean := strings.TrimRight(filepath.Clean(rootFull), string(filepath.Separator))
	pathClean := filepath.Clean(pathFull)
	repoClean := strings.TrimRight(filepath.Clean(repoFull), string(filepath.Separator))
	return (strings.EqualFold(pathClean, rootClean) || strings.HasPrefix(strings.ToLower(pathClean), strings.ToLower(rootClean+string(filepath.Separator)))) && (strings.EqualFold(pathClean, repoClean) || strings.HasPrefix(strings.ToLower(pathClean), strings.ToLower(repoClean+string(filepath.Separator))))
}

func writeCandidateReviewProofDraftFile(caseRoot, path string, data []byte) (bool, error) {
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return false, err
	}
	if err := rejectCandidateDecisionSymlinkPath(caseRoot, filepath.Dir(path), false); err != nil {
		return false, err
	}
	if err := rejectCandidateDecisionSymlinkPath(caseRoot, path, true); err != nil {
		return false, err
	}
	err := writeDurableExclusiveFile(path, data)
	if err == nil {
		return false, nil
	}
	if !os.IsExist(err) {
		return false, err
	}
	state, stateErr := refsf.ClassifyNonEmptyRegularFile(path)
	if stateErr != nil {
		return false, stateErr
	}
	if state != refsf.RegularFileReady {
		return false, fmt.Errorf("existing candidate review proof draft must be a non-empty regular file: %s", path)
	}
	existing, readErr := os.ReadFile(path)
	if readErr != nil {
		return false, readErr
	}
	if !bytes.Equal(existing, data) {
		return false, fmt.Errorf("existing candidate review proof draft does not match replay: %s", path)
	}
	return true, nil
}

func candidateReviewProofDraftCommand(packetPath, decisionPath, proofPath, proofType, candidatePath, decision, reason, actor, evidenceRefs, expectedProofSHA256 string, apply bool) string {
	base := "/rekit promote -PacketPath " + quoteCandidateDecisionArg(packetPath)
	if strings.TrimSpace(decisionPath) != "" {
		base += " -CandidateDecisionPath " + quoteCandidateDecisionArg(decisionPath)
	}
	base += " -DraftReviewProof -ProofPath " + quoteCandidateDecisionArg(proofPath) +
		" -ProofType " + quoteCandidateDecisionArg(proofType) +
		" -CandidatePath " + quoteCandidateDecisionArg(candidatePath) +
		" -ProofDecision " + quoteCandidateDecisionArg(decision) +
		" -Reason " + quoteCandidateDecisionArg(reason) +
		" -Actor " + quoteCandidateDecisionArg(actor) +
		" -EvidenceRefs " + quoteCandidateDecisionArg(evidenceRefs)
	if apply {
		return base + " -ExpectedProofSha256 " + quoteCandidateDecisionArg(expectedProofSHA256) + " -Apply -Format json"
	}
	return base + " -WhatIf -Format json"
}

func candidateReviewProofDraftBoundary() []string {
	return []string{
		"draft writes only a repo-local pack-memory review proof note under promote-candidates/review-artifacts",
		"draft does not merge or cleanup candidates, run doctor/init/reconsume, write authority/confirmed, or execute heavy tools",
		"Apply requires the exact proofSha256 returned by WhatIf",
		"proof note must not contain case-specific artifacts, traces, dumps, captures, payloads, flags, or customer data",
	}
}
