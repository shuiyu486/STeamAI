package promote

import (
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"reflect"
	"runtime"
	"slices"
	"sort"
	"strconv"
	"strings"

	"github.com/shuiyu486/re-context-kits/internal/rekit/doctor"
	refsf "github.com/shuiyu486/re-context-kits/internal/rekit/fs"
	"github.com/shuiyu486/re-context-kits/internal/rekit/instance"
	"github.com/shuiyu486/re-context-kits/internal/rekit/manifest"
)

const maxCandidateDecisionBytes = 1024 * 1024

var removeCandidateDecisionFile = os.Remove

type CandidateDecisionOptions struct {
	PacketPath   string
	DecisionPath string
	WhatIf       bool
}

type CandidateDecisionDraftOptions struct {
	PacketPath             string
	DecisionPath           string
	Decision               string
	Reason                 string
	Actor                  string
	EvidenceRefs           string
	ExpectedDecisionSHA256 string
	WhatIf                 bool
}

type CandidateDecisionVerificationOptions struct {
	PacketPath       string
	DecisionPath     string
	FreshCaseRoot    string
	AttachedCaseRoot string
	WhatIf           bool
}

type CandidateDecisionFile struct {
	SchemaVersion int                     `json:"schemaVersion"`
	Kind          string                  `json:"kind"`
	PacketHash    string                  `json:"packetHash"`
	Decisions     []CandidateDecisionItem `json:"decisions"`
}

type CandidateDecisionItem struct {
	CandidatePath  string                      `json:"candidatePath"`
	Decision       string                      `json:"decision"`
	CandidateHash  string                      `json:"candidateHash"`
	PackTargetHash string                      `json:"packTargetHash,omitempty"`
	Reason         string                      `json:"reason"`
	Actor          string                      `json:"actor"`
	EvidenceRefs   []CandidateDecisionEvidence `json:"evidenceRefs"`
}

type CandidateDecisionDraftResult struct {
	SchemaVersion  int                     `json:"schemaVersion"`
	Command        string                  `json:"command"`
	Mode           string                  `json:"mode"`
	CaseRoot       string                  `json:"caseRoot"`
	RepoRoot       string                  `json:"repoRoot"`
	Pack           string                  `json:"pack"`
	PacketPath     string                  `json:"packetPath"`
	DecisionPath   string                  `json:"decisionPath"`
	PacketHash     string                  `json:"packetHash"`
	DecisionSHA256 string                  `json:"decisionSha256"`
	IsMutation     bool                    `json:"isMutation"`
	Applied        bool                    `json:"applied"`
	AlreadyWritten bool                    `json:"alreadyWritten,omitempty"`
	Decision       string                  `json:"decision"`
	DecisionCount  int                     `json:"decisionCount"`
	Accepted       int                     `json:"accepted"`
	Rejected       int                     `json:"rejected"`
	Superseded     int                     `json:"superseded"`
	Decisions      []CandidateDecisionItem `json:"decisions"`
	PreviewCommand string                  `json:"previewCommand,omitempty"`
	ApplyCommand   string                  `json:"applyCommand,omitempty"`
	NextSteps      []string                `json:"nextSteps"`
	Boundary       []string                `json:"boundary"`
}

type CandidateDecisionEvidence struct {
	Path   string `json:"path"`
	SHA256 string `json:"sha256"`
}

type CandidateDecisionAction struct {
	CandidatePath       string   `json:"candidatePath"`
	Kind                string   `json:"kind"`
	Decision            string   `json:"decision"`
	PackTarget          string   `json:"packTarget,omitempty"`
	Action              string   `json:"action"`
	CandidateBackupPath string   `json:"candidateBackupPath,omitempty"`
	TargetBackupPath    string   `json:"targetBackupPath,omitempty"`
	EvidenceRefs        []string `json:"evidenceRefs"`
}

type CandidateDecisionApplyError struct {
	Result CandidateDecisionResult
	Err    error
}

func (e *CandidateDecisionApplyError) Error() string {
	return e.Err.Error()
}

func (e *CandidateDecisionApplyError) Unwrap() error {
	return e.Err
}

type CandidateDecisionVerificationResult struct {
	SchemaVersion            int                       `json:"schemaVersion"`
	Kind                     string                    `json:"kind"`
	Mode                     string                    `json:"mode,omitempty"`
	Pack                     string                    `json:"pack"`
	CaseRoot                 string                    `json:"caseRoot"`
	FreshCaseRoot            string                    `json:"freshCaseRoot"`
	AttachedCaseRoot         string                    `json:"attachedCaseRoot"`
	PacketHash               string                    `json:"packetHash"`
	DecisionHash             string                    `json:"decisionHash"`
	ReceiptHash              string                    `json:"receiptHash"`
	ReceiptPath              string                    `json:"receiptPath"`
	VerificationProofPath    string                    `json:"verificationProofPath"`
	ProvisionIntentPath      string                    `json:"provisionIntentPath,omitempty"`
	ProvisionIntentSHA256    string                    `json:"provisionIntentSha256,omitempty"`
	ProvisionReceiptPath     string                    `json:"provisionReceiptPath,omitempty"`
	ProvisionReceiptSHA256   string                    `json:"provisionReceiptSha256,omitempty"`
	RetirementPreviewCommand string                    `json:"retirementPreviewCommand,omitempty"`
	IsMutation               bool                      `json:"isMutation"`
	Applied                  bool                      `json:"applied"`
	Replay                   bool                      `json:"replay,omitempty"`
	Ready                    bool                      `json:"ready"`
	PackDoctorRows           int                       `json:"packDoctorRows"`
	FreshDoctorRows          int                       `json:"freshDoctorRows"`
	AttachedDoctorRows       int                       `json:"attachedDoctorRows"`
	VerifiedActions          []CandidateDecisionAction `json:"verifiedActions"`
	VerificationRunbookSteps []string                  `json:"verificationRunbookSteps,omitempty"`
	NextSteps                []string                  `json:"nextSteps"`
	Boundary                 []string                  `json:"boundary"`
}

type CandidateDecisionReceipt struct {
	SchemaVersion                int                       `json:"schemaVersion"`
	Kind                         string                    `json:"kind"`
	Pack                         string                    `json:"pack"`
	RepoRoot                     string                    `json:"repoRoot"`
	CaseRoot                     string                    `json:"caseRoot"`
	PacketPath                   string                    `json:"packetPath"`
	DecisionPath                 string                    `json:"decisionPath"`
	PacketHash                   string                    `json:"packetHash"`
	DecisionHash                 string                    `json:"decisionHash"`
	BackupRoot                   string                    `json:"backupRoot"`
	IndexPath                    string                    `json:"indexPath"`
	Accepted                     int                       `json:"accepted"`
	Rejected                     int                       `json:"rejected"`
	Superseded                   int                       `json:"superseded"`
	Actions                      []CandidateDecisionAction `json:"actions"`
	DecisionEvidence             []string                  `json:"decisionEvidence"`
	ReceiptPath                  string                    `json:"receiptPath"`
	VerificationProofPath        string                    `json:"verificationProofPath,omitempty"`
	VerificationPending          bool                      `json:"verificationPending"`
	VerificationWorkspaceRoot    string                    `json:"verificationWorkspaceRoot,omitempty"`
	VerificationProvisionCommand string                    `json:"verificationProvisionCommand,omitempty"`
	VerificationCommand          string                    `json:"verificationCommand,omitempty"`
	Boundary                     []string                  `json:"boundary"`
}

type CandidateDecisionResult struct {
	SchemaVersion        int                       `json:"schemaVersion"`
	Command              string                    `json:"command"`
	Mode                 string                    `json:"mode"`
	CaseRoot             string                    `json:"caseRoot"`
	RepoRoot             string                    `json:"repoRoot"`
	Pack                 string                    `json:"pack"`
	PacketPath           string                    `json:"packetPath"`
	DecisionPath         string                    `json:"decisionPath"`
	PacketHash           string                    `json:"packetHash"`
	IsMutation           bool                      `json:"isMutation"`
	Applied              bool                      `json:"applied"`
	RolledBack           bool                      `json:"rolledBack,omitempty"`
	RecoveryRequired     bool                      `json:"recoveryRequired,omitempty"`
	FailedAction         string                    `json:"failedAction,omitempty"`
	Accepted             int                       `json:"accepted"`
	Rejected             int                       `json:"rejected"`
	Superseded           int                       `json:"superseded"`
	BackupRoot           string                    `json:"backupRoot,omitempty"`
	IndexPath            string                    `json:"indexPath,omitempty"`
	ReceiptPath          string                    `json:"receiptPath,omitempty"`
	Receipt              *CandidateDecisionReceipt `json:"receipt,omitempty"`
	Actions              []CandidateDecisionAction `json:"actions"`
	RecoveryActions      []string                  `json:"recoveryActions,omitempty"`
	DecisionRunbookSteps []string                  `json:"decisionRunbookSteps,omitempty"`
	NextSteps            []string                  `json:"nextSteps"`
	Boundary             []string                  `json:"boundary"`
}

type candidateDecisionPlan struct {
	result       CandidateDecisionResult
	packet       CandidateReviewPacket
	decisions    CandidateDecisionFile
	decisionHash string
	indexHash    string
	indexExisted bool
	indexEntries []candidateIndexEntry
	items        []candidateDecisionPlanItem
}

type candidateDecisionPlanItem struct {
	decision         CandidateDecisionItem
	review           CandidateReviewItem
	action           CandidateDecisionAction
	candidateRemoved bool
}

type preparedCandidateDecisionDraft struct {
	result        CandidateDecisionDraftResult
	decisionBytes []byte
}

type candidateDecisionTransaction struct {
	SchemaVersion   int                       `json:"schemaVersion"`
	Kind            string                    `json:"kind"`
	PacketHash      string                    `json:"packetHash"`
	DecisionHash    string                    `json:"decisionHash"`
	IndexExisted    bool                      `json:"indexExisted"`
	IndexBackupPath string                    `json:"indexBackupPath,omitempty"`
	Result          CandidateDecisionResult   `json:"result"`
	Actions         []CandidateDecisionAction `json:"actions"`
}

func candidateDecisionActionCounts(actions []CandidateDecisionAction) (accepted, rejected, superseded int) {
	for _, action := range actions {
		switch action.Decision {
		case "accept":
			accepted++
		case "reject":
			rejected++
		case "superseded":
			superseded++
		}
	}
	return accepted, rejected, superseded
}

type candidateDecisionAuthority struct {
	instance      instance.Instance
	manifest      *manifest.Manifest
	packet        CandidateReviewPacket
	decisions     CandidateDecisionFile
	packetPath    string
	decisionPath  string
	packetHash    string
	decisionHash  string
	receiptPath   string
	receiptBytes  []byte
	receipt       CandidateDecisionReceipt
	candidateRoot string
}

func loadCandidateDecisionAuthority(repoRoot, caseRoot, pack, packetInput, decisionInput string) (candidateDecisionAuthority, error) {
	inst, err := instance.AssertAttached(caseRoot, repoRoot, pack)
	if err != nil {
		return candidateDecisionAuthority{}, err
	}
	m, err := manifest.Load(repoRoot, pack)
	if err != nil {
		return candidateDecisionAuthority{}, err
	}
	packetPath, packetBytes, err := readStrictCandidateDecisionFile(packetInput, "candidate review packet")
	if err != nil {
		return candidateDecisionAuthority{}, err
	}
	decisionPath, decisionBytes, err := readStrictCandidateDecisionFile(decisionInput, "candidate decision")
	if err != nil {
		return candidateDecisionAuthority{}, err
	}
	var packet CandidateReviewPacket
	if err := decodeStrictJSON(packetBytes, &packet); err != nil {
		return candidateDecisionAuthority{}, fmt.Errorf("decode candidate review packet: %w", err)
	}
	var decisions CandidateDecisionFile
	if err := decodeStrictJSON(decisionBytes, &decisions); err != nil {
		return candidateDecisionAuthority{}, fmt.Errorf("decode candidate decision: %w", err)
	}
	packetHash := sha256Hex(packetBytes)
	decisionHash := sha256Hex(decisionBytes)
	if packet.SchemaVersion != 1 || packet.Kind != "pack-memory-candidate-review" || packet.Command != "promote" || decisions.SchemaVersion != 1 || decisions.Kind != "pack-memory-candidate-decisions" || !strings.EqualFold(decisions.PacketHash, packetHash) {
		return candidateDecisionAuthority{}, fmt.Errorf("candidate verification packet/decision binding mismatch")
	}
	candidateRoot := filepath.Join(m.PackRoot, "promote-candidates")
	if !sameCandidateDecisionPath(packet.CandidateResult.RepoRoot, repoRoot) || !sameCandidateDecisionPath(packet.CandidateResult.CaseRoot, inst.CaseRoot) || packet.CandidateResult.Pack != pack || !sameCandidateDecisionPath(packet.CandidateResult.CandidateRoot, candidateRoot) {
		return candidateDecisionAuthority{}, fmt.Errorf("candidate verification repo/case/pack/root binding mismatch")
	}
	proofRoot := filepath.Join(candidateRoot, "review-artifacts")
	if err := rejectCandidateDecisionSymlinkPath(candidateRoot, proofRoot, false); err != nil {
		return candidateDecisionAuthority{}, err
	}
	receiptPath := candidateDecisionReceiptPath(candidateRoot, packetHash, decisionHash)
	if err := rejectCandidateDecisionSymlinkPath(candidateRoot, receiptPath, false); err != nil {
		return candidateDecisionAuthority{}, err
	}
	_, receiptBytes, err := readStrictCandidateDecisionFile(receiptPath, "candidate decision receipt")
	if err != nil {
		return candidateDecisionAuthority{}, fmt.Errorf("read candidate decision receipt: %w", err)
	}
	var receipt CandidateDecisionReceipt
	if err := decodeStrictJSON(receiptBytes, &receipt); err != nil {
		return candidateDecisionAuthority{}, fmt.Errorf("decode candidate decision receipt: %w", err)
	}
	canonicalProofPath := candidateDecisionVerificationProofPath(candidateRoot, packetHash, decisionHash)
	if receipt.SchemaVersion != 1 || receipt.Kind != "pack-memory-candidate-decision-receipt" || receipt.Pack != pack || receipt.PacketHash != packetHash || receipt.DecisionHash != decisionHash || !sameCandidateDecisionPath(receipt.RepoRoot, repoRoot) || !sameCandidateDecisionPath(receipt.CaseRoot, inst.CaseRoot) || !sameCandidateDecisionPath(receipt.PacketPath, packetPath) || !sameCandidateDecisionPath(receipt.DecisionPath, decisionPath) || !sameCandidateDecisionPath(receipt.ReceiptPath, receiptPath) || !sameCandidateDecisionPath(receipt.IndexPath, filepath.Join(candidateRoot, "index.json")) {
		return candidateDecisionAuthority{}, fmt.Errorf("candidate decision receipt binding mismatch")
	}
	accepted, rejected, superseded := candidateDecisionActionCounts(receipt.Actions)
	if receipt.Accepted != accepted || receipt.Rejected != rejected || receipt.Superseded != superseded || len(receipt.Actions) != len(decisions.Decisions) {
		return candidateDecisionAuthority{}, fmt.Errorf("candidate decision receipt action counts do not match reviewed decisions")
	}
	if accepted > 0 {
		workspace := candidateDecisionVerificationWorkspace(inst.CaseRoot, packetHash, decisionHash)
		freshRoot := filepath.Join(workspace, "fresh")
		attachedRoot := filepath.Join(workspace, "attached")
		expectedProvisionCommand := candidateDecisionVerificationProvisionCommand(packetPath, decisionPath, freshRoot, attachedRoot)
		expectedVerificationCommand := candidateDecisionVerificationCommand(packetPath, decisionPath, freshRoot, attachedRoot)
		if !receipt.VerificationPending || !sameCandidateDecisionPath(receipt.VerificationProofPath, canonicalProofPath) || !sameCandidateDecisionPath(receipt.VerificationWorkspaceRoot, workspace) || receipt.VerificationProvisionCommand != expectedProvisionCommand || receipt.VerificationCommand != expectedVerificationCommand {
			return candidateDecisionAuthority{}, fmt.Errorf("candidate decision receipt verification binding mismatch")
		}
	}
	if receipt.Accepted == 0 && (receipt.VerificationPending || strings.TrimSpace(receipt.VerificationProofPath) != "" || strings.TrimSpace(receipt.VerificationWorkspaceRoot) != "" || strings.TrimSpace(receipt.VerificationProvisionCommand) != "" || strings.TrimSpace(receipt.VerificationCommand) != "") {
		return candidateDecisionAuthority{}, fmt.Errorf("candidate decision receipt verification binding mismatch")
	}
	if err := assertInsideRoot(candidateRoot, receipt.BackupRoot); err != nil {
		return candidateDecisionAuthority{}, err
	}
	if err := rejectCandidateDecisionSymlinkPath(candidateRoot, receipt.BackupRoot, false); err != nil {
		return candidateDecisionAuthority{}, err
	}
	markerPath := filepath.Join(receipt.BackupRoot, "committed.json")
	transactionPath := filepath.Join(receipt.BackupRoot, "transaction.json")
	if err := rejectCandidateDecisionSymlinkPath(receipt.BackupRoot, markerPath, false); err != nil {
		return candidateDecisionAuthority{}, err
	}
	if err := rejectCandidateDecisionSymlinkPath(receipt.BackupRoot, transactionPath, false); err != nil {
		return candidateDecisionAuthority{}, err
	}
	_, transactionBytes, err := readStrictCandidateDecisionFile(transactionPath, "candidate decision transaction")
	if err != nil {
		return candidateDecisionAuthority{}, err
	}
	var transaction candidateDecisionTransaction
	if err := decodeStrictJSON(transactionBytes, &transaction); err != nil {
		return candidateDecisionAuthority{}, fmt.Errorf("decode candidate decision transaction: %w", err)
	}
	_, markerBytes, err := readStrictCandidateDecisionFile(markerPath, "candidate decision committed marker")
	if err != nil {
		return candidateDecisionAuthority{}, fmt.Errorf("candidate decision receipt transaction is not committed: %s", receipt.BackupRoot)
	}
	var committed CandidateDecisionResult
	if err := decodeStrictJSON(markerBytes, &committed); err != nil {
		return candidateDecisionAuthority{}, fmt.Errorf("decode candidate decision committed marker: %w", err)
	}
	if transaction.SchemaVersion != 1 || transaction.Kind != "pack-memory-candidate-decision-transaction" || transaction.PacketHash != packetHash || transaction.DecisionHash != decisionHash || transaction.Result.Accepted != accepted || transaction.Result.Rejected != rejected || transaction.Result.Superseded != superseded || committed.Accepted != accepted || committed.Rejected != rejected || committed.Superseded != superseded || !reflect.DeepEqual(transaction.Actions, receipt.Actions) || !reflect.DeepEqual(transaction.Result.Actions, receipt.Actions) || !reflect.DeepEqual(committed.Actions, receipt.Actions) || !sameCandidateDecisionPath(transaction.Result.BackupRoot, receipt.BackupRoot) || !sameCandidateDecisionPath(transaction.Result.IndexPath, receipt.IndexPath) || !sameCandidateDecisionPath(transaction.Result.PacketPath, packetPath) || !sameCandidateDecisionPath(transaction.Result.DecisionPath, decisionPath) || transaction.Result.PacketHash != packetHash || transaction.Result.Pack != pack || !sameCandidateDecisionPath(transaction.Result.RepoRoot, repoRoot) || !sameCandidateDecisionPath(transaction.Result.CaseRoot, inst.CaseRoot) || !sameCandidateDecisionPath(committed.BackupRoot, receipt.BackupRoot) || !sameCandidateDecisionPath(committed.IndexPath, receipt.IndexPath) || !sameCandidateDecisionPath(committed.PacketPath, packetPath) || !sameCandidateDecisionPath(committed.DecisionPath, decisionPath) || committed.PacketHash != packetHash || committed.Pack != pack || !sameCandidateDecisionPath(committed.RepoRoot, repoRoot) || !sameCandidateDecisionPath(committed.CaseRoot, inst.CaseRoot) || !committed.Applied || committed.Receipt == nil || !reflect.DeepEqual(*committed.Receipt, receipt) || !sameCandidateDecisionPath(committed.ReceiptPath, receiptPath) {
		return candidateDecisionAuthority{}, fmt.Errorf("candidate decision receipt transaction binding mismatch")
	}
	decisionByCandidate := map[string]CandidateDecisionItem{}
	for _, decision := range decisions.Decisions {
		key, err := candidateDecisionAuthorityKey(decision.CandidatePath)
		if err != nil {
			return candidateDecisionAuthority{}, fmt.Errorf("candidate decision has invalid candidatePath %q", decision.CandidatePath)
		}
		if _, duplicate := decisionByCandidate[key]; duplicate {
			return candidateDecisionAuthority{}, fmt.Errorf("duplicate candidate decision for %s", decision.CandidatePath)
		}
		decisionByCandidate[key] = decision
	}
	reviewByCandidate := map[string]CandidateReviewItem{}
	pendingReviewKeys := map[string]bool{}
	for _, review := range packet.CandidateResult.ReviewPlan.ReviewItems {
		key, include, err := candidateReviewAuthorityKey(review)
		if err != nil {
			return candidateDecisionAuthority{}, err
		}
		if !include {
			continue
		}
		if _, duplicate := reviewByCandidate[key]; duplicate {
			return candidateDecisionAuthority{}, fmt.Errorf("duplicate candidate review for %s", review.CandidatePath)
		}
		reviewByCandidate[key] = review
		if review.ReviewDecision == "pending-review" {
			pendingReviewKeys[key] = true
		}
	}
	decisionKeys := map[string]bool{}
	for key := range decisionByCandidate {
		decisionKeys[key] = true
	}
	if !reflect.DeepEqual(decisionKeys, pendingReviewKeys) {
		return candidateDecisionAuthority{}, fmt.Errorf("candidate decisions do not exactly cover pending review items")
	}
	seenActions := map[string]bool{}
	for _, action := range receipt.Actions {
		key, err := candidateDecisionAuthorityKey(action.CandidatePath)
		if err != nil {
			return candidateDecisionAuthority{}, fmt.Errorf("candidate decision receipt has invalid candidatePath %q", action.CandidatePath)
		}
		if seenActions[key] {
			return candidateDecisionAuthority{}, fmt.Errorf("duplicate candidate decision receipt action for %s", action.CandidatePath)
		}
		seenActions[key] = true
		decision, ok := decisionByCandidate[key]
		review, reviewed := reviewByCandidate[key]
		if !ok || !reviewed || strings.ToLower(strings.TrimSpace(decision.Decision)) != action.Decision || action.Kind != review.Kind || !sameCandidateDecisionPath(action.PackTarget, review.PackTarget) {
			return candidateDecisionAuthority{}, fmt.Errorf("candidate decision receipt action is not bound to reviewed decision: %s", action.CandidatePath)
		}
		expectedEvidence := make([]string, 0, len(decision.EvidenceRefs))
		for _, evidence := range decision.EvidenceRefs {
			full, err := validateCandidateDecisionEvidenceRef(repoRoot, inst.CaseRoot, evidence)
			if err != nil {
				return candidateDecisionAuthority{}, err
			}
			expectedEvidence = append(expectedEvidence, full)
		}
		if !reflect.DeepEqual(action.EvidenceRefs, expectedEvidence) {
			return candidateDecisionAuthority{}, fmt.Errorf("candidate decision receipt evidence binding mismatch: %s", action.CandidatePath)
		}
		expectedAction := map[string]string{"accept": "merge-accepted-candidate-and-cleanup", "reject": "cleanup-rejected-candidate", "superseded": "cleanup-superseded-candidate"}[action.Decision]
		if expectedAction == "" || action.Action != expectedAction {
			return candidateDecisionAuthority{}, fmt.Errorf("candidate decision receipt action outcome mismatch: %s", action.CandidatePath)
		}
		actionRoot := candidateRoot
		if action.Kind == "tooling-candidate-source" {
			if action.Decision == "accept" {
				return candidateDecisionAuthority{}, fmt.Errorf("tooling candidate cannot be accepted automatically: %s", action.CandidatePath)
			}
			actionRoot = filepath.Join(m.PackRoot, "tooling", "candidates")
		} else if action.Kind != "managed-doc" {
			return candidateDecisionAuthority{}, fmt.Errorf("candidate decision receipt has unsupported action kind: %s", action.Kind)
		}
		if err := rejectCandidateDecisionSymlinkPath(actionRoot, action.CandidatePath, true); err != nil {
			return candidateDecisionAuthority{}, err
		}
		if _, err := os.Lstat(action.CandidatePath); err == nil || !os.IsNotExist(err) {
			return candidateDecisionAuthority{}, fmt.Errorf("reviewed candidate cleanup is incomplete: %s", action.CandidatePath)
		}
		if action.Kind == "managed-doc" && candidateIndexStillContains(receipt.IndexPath, action.CandidatePath) {
			return candidateDecisionAuthority{}, fmt.Errorf("reviewed candidate index cleanup is incomplete: %s", action.CandidatePath)
		}
		if strings.TrimSpace(action.CandidateBackupPath) == "" {
			return candidateDecisionAuthority{}, fmt.Errorf("candidate decision receipt action lacks candidate backup: %s", action.CandidatePath)
		}
		if err := rejectCandidateDecisionSymlinkPath(receipt.BackupRoot, action.CandidateBackupPath, false); err != nil {
			return candidateDecisionAuthority{}, err
		}
		if !strings.EqualFold(fileSHA256(action.CandidateBackupPath), strings.TrimSpace(decision.CandidateHash)) {
			return candidateDecisionAuthority{}, fmt.Errorf("reviewed candidate backup hash mismatch: %s", action.CandidateBackupPath)
		}
		if action.Decision == "accept" && (fileSHA256(action.PackTarget) == "" || fileSHA256(action.PackTarget) != fileSHA256(action.CandidateBackupPath)) {
			return candidateDecisionAuthority{}, fmt.Errorf("accepted pack target no longer matches reviewed candidate backup: %s", action.PackTarget)
		}
	}
	if len(seenActions) != len(decisionByCandidate) {
		return candidateDecisionAuthority{}, fmt.Errorf("candidate decision receipt actions do not exactly cover reviewed decisions")
	}
	for key := range decisionByCandidate {
		if !seenActions[key] {
			return candidateDecisionAuthority{}, fmt.Errorf("candidate decision receipt actions do not exactly cover reviewed decisions")
		}
	}
	return candidateDecisionAuthority{instance: inst, manifest: m, packet: packet, decisions: decisions, packetPath: packetPath, decisionPath: decisionPath, packetHash: packetHash, decisionHash: decisionHash, receiptPath: receiptPath, receiptBytes: receiptBytes, receipt: receipt, candidateRoot: candidateRoot}, nil
}

func candidateReviewAuthorityKey(review CandidateReviewItem) (string, bool, error) {
	if strings.TrimSpace(review.CandidatePath) == "" {
		if review.ReviewDecision == "pending-review" {
			return "", false, fmt.Errorf("malformed candidate review: pending-review has invalid candidatePath %q", review.CandidatePath)
		}
		return "", false, nil
	}
	key, err := candidateDecisionAuthorityKey(review.CandidatePath)
	if err != nil {
		return "", false, fmt.Errorf("candidate review has invalid candidatePath %q", review.CandidatePath)
	}
	return key, true, nil
}

func candidateDecisionAuthorityKey(path string) (string, error) {
	path = strings.TrimSpace(path)
	if path == "" {
		return "", fmt.Errorf("empty candidate path")
	}
	full, err := filepath.Abs(path)
	if err != nil {
		return "", err
	}
	key := filepath.Clean(full)
	if runtime.GOOS == "windows" {
		key = strings.ToLower(key)
	}
	return key, nil
}

func VerifyCandidateDecision(repoRoot, caseRoot, pack string, opt CandidateDecisionVerificationOptions) (CandidateDecisionVerificationResult, error) {
	if !opt.WhatIf {
		m, err := manifest.Load(repoRoot, pack)
		if err != nil {
			return CandidateDecisionVerificationResult{}, err
		}
		unlock, err := acquireCandidateDecisionLock(filepath.Join(m.PackRoot, "promote-candidates"))
		if err != nil {
			return CandidateDecisionVerificationResult{}, err
		}
		defer unlock()
	}
	authority, err := loadCandidateDecisionAuthority(repoRoot, caseRoot, pack, opt.PacketPath, opt.DecisionPath)
	if err != nil {
		return CandidateDecisionVerificationResult{}, err
	}
	inst := authority.instance
	m := authority.manifest
	packetHash := authority.packetHash
	decisionHash := authority.decisionHash
	receipt := authority.receipt
	receiptBytes := authority.receiptBytes
	receiptPath := authority.receiptPath
	canonicalCandidateRoot := authority.candidateRoot
	packRows, err := doctor.Pack(repoRoot, pack)
	if err != nil {
		return CandidateDecisionVerificationResult{}, err
	}
	freshRoot, err := filepath.Abs(strings.TrimSpace(opt.FreshCaseRoot))
	if err != nil || strings.TrimSpace(opt.FreshCaseRoot) == "" {
		return CandidateDecisionVerificationResult{}, fmt.Errorf("candidate verification requires -FreshCaseRoot")
	}
	attachedRoot, err := filepath.Abs(strings.TrimSpace(opt.AttachedCaseRoot))
	if err != nil || strings.TrimSpace(opt.AttachedCaseRoot) == "" {
		return CandidateDecisionVerificationResult{}, fmt.Errorf("candidate verification requires -AttachedCaseRoot")
	}
	if sameCandidateDecisionPath(freshRoot, attachedRoot) || sameCandidateDecisionPath(freshRoot, inst.CaseRoot) || sameCandidateDecisionPath(attachedRoot, inst.CaseRoot) {
		return CandidateDecisionVerificationResult{}, fmt.Errorf("candidate verification requires distinct source, fresh, and attached case roots")
	}
	freshRows, err := doctor.Case(repoRoot, freshRoot, pack)
	if err != nil {
		return CandidateDecisionVerificationResult{}, fmt.Errorf("fresh case reconsume validation: %w", err)
	}
	attachedRows, err := doctor.Case(repoRoot, attachedRoot, pack)
	if err != nil {
		return CandidateDecisionVerificationResult{}, fmt.Errorf("attached case reconsume validation: %w", err)
	}
	if err := verifyCandidateDecisionCaseContent(m.PackRoot, freshRoot, "fresh", receipt.Actions); err != nil {
		return CandidateDecisionVerificationResult{}, err
	}
	if err := verifyCandidateDecisionCaseContent(m.PackRoot, attachedRoot, "attached", receipt.Actions); err != nil {
		return CandidateDecisionVerificationResult{}, err
	}
	if !opt.WhatIf {
		if err := verifyCandidateDecisionCommittedState(receipt); err != nil {
			return CandidateDecisionVerificationResult{}, err
		}
	}
	proofPath := candidateDecisionVerificationProofPath(canonicalCandidateRoot, packetHash, decisionHash)
	workspace := candidateDecisionVerificationWorkspace(inst.CaseRoot, packetHash, decisionHash)
	provisionIntentPath := filepath.Join(workspace, "provision.intent.json")
	provisionReceiptPath := filepath.Join(workspace, "provision.receipt.json")
	var provisionIntentBytes, provisionReceiptBytes []byte
	if sameCandidateDecisionPath(freshRoot, filepath.Join(workspace, "fresh")) && sameCandidateDecisionPath(attachedRoot, filepath.Join(workspace, "attached")) {
		_, provisionIntentBytes, err = readStrictCandidateDecisionFile(provisionIntentPath, "candidate verification provision intent")
		if err != nil {
			return CandidateDecisionVerificationResult{}, err
		}
		_, provisionReceiptBytes, err = readStrictCandidateDecisionFile(provisionReceiptPath, "candidate verification provision receipt")
		if err != nil {
			return CandidateDecisionVerificationResult{}, err
		}
	} else {
		provisionIntentPath = ""
		provisionReceiptPath = ""
	}
	provisionIntentHash, provisionReceiptHash, retirementPreviewCommand := "", "", ""
	if len(provisionIntentBytes) > 0 {
		provisionIntentHash = sha256Hex(provisionIntentBytes)
		provisionReceiptHash = sha256Hex(provisionReceiptBytes)
		retirementPreviewCommand = fmt.Sprintf("/rekit promote -PacketPath %s -CandidateDecisionPath %s -RetireCandidateVerificationWorkspace -WhatIf -Format json", quoteCandidateDecisionArg(authority.packetPath), quoteCandidateDecisionArg(authority.decisionPath))
	}
	result := CandidateDecisionVerificationResult{
		SchemaVersion:            1,
		Kind:                     "pack-memory-candidate-decision-verification",
		Pack:                     pack,
		CaseRoot:                 inst.CaseRoot,
		FreshCaseRoot:            freshRoot,
		AttachedCaseRoot:         attachedRoot,
		PacketHash:               packetHash,
		DecisionHash:             decisionHash,
		ReceiptHash:              sha256Hex(receiptBytes),
		ReceiptPath:              receiptPath,
		VerificationProofPath:    proofPath,
		ProvisionIntentPath:      provisionIntentPath,
		ProvisionIntentSHA256:    provisionIntentHash,
		ProvisionReceiptPath:     provisionReceiptPath,
		ProvisionReceiptSHA256:   provisionReceiptHash,
		RetirementPreviewCommand: retirementPreviewCommand,
		IsMutation:               !opt.WhatIf,
		Applied:                  false,
		Ready:                    true,
		PackDoctorRows:           len(packRows),
		FreshDoctorRows:          len(freshRows),
		AttachedDoctorRows:       len(attachedRows),
		VerifiedActions:          append([]CandidateDecisionAction(nil), receipt.Actions...),
		Boundary: []string{
			"verification reads pack and attached cases, then writes only the repo-local proof on Apply",
			"verification does not sync cases, merge candidates, write authority/confirmed, or execute heavy tools",
		},
	}
	if opt.WhatIf {
		result.Mode = "previewed"
		result.NextSteps = []string{"inspect doctor/reconsume validation, then rerun the identical command with -Apply"}
		result.VerificationRunbookSteps = candidateVerificationRunbookSteps(result)
		return result, nil
	}
	result.Applied = true
	result.NextSteps = []string{"run the returned retirementPreviewCommand to preview exact cleanup of the generated verification workspace", "rerun /rekit status or release-check after retirement"}
	proofResult := result
	data, err := json.MarshalIndent(proofResult, "", "  ")
	if err != nil {
		return CandidateDecisionVerificationResult{}, err
	}
	already, err := writeDurableFileIdempotent(proofPath, append(data, '\n'))
	if err != nil {
		return CandidateDecisionVerificationResult{}, err
	}
	result.Mode = "verified"
	if already {
		result.Mode = "already-verified"
		result.Replay = true
		result.NextSteps = []string{"the exact candidate verification proof already exists", "run the returned retirementPreviewCommand to preview exact cleanup of the generated verification workspace", "rerun /rekit status or release-check after retirement"}
	}
	result.VerificationRunbookSteps = candidateVerificationRunbookSteps(result)
	return result, nil
}

func candidateVerificationRunbookSteps(result CandidateDecisionVerificationResult) []string {
	steps := []string{}
	add := func(step string) {
		step = strings.TrimSpace(step)
		if step == "" || slices.Contains(steps, step) {
			return
		}
		steps = append(steps, step)
	}
	addNextSteps := func() {
		for _, step := range result.NextSteps {
			add("follow candidate verification nextSteps: " + step)
		}
	}

	if !result.Ready {
		add("stop candidate verification follow-through; doctor/reconsume validation is not ready")
		addNextSteps()
		add("do not preview verification workspace retirement until verification is ready")
		return steps
	}
	if !result.Applied || !result.IsMutation {
		add(fmt.Sprintf("inspect pack/fresh/attached doctor and reconsume validation for %d accepted candidate actions", len(result.VerifiedActions)))
		addNextSteps()
		add("rerun the identical candidate verification command with -Apply only after the source, fresh, and attached cases still match the receipt")
		add("do not retire the verification workspace until the verification proof has been written by Apply")
		return steps
	}

	if result.VerificationProofPath != "" {
		add("retain candidate verification proof " + result.VerificationProofPath + " as the terminal accepted-candidate reconsume evidence")
	} else {
		add("retain the candidate verification proof as the terminal accepted-candidate reconsume evidence")
	}
	if result.ProvisionIntentPath != "" && result.ProvisionReceiptPath != "" {
		add("confirm provisioning artifacts are bound before retirement: intent=" + result.ProvisionIntentPath + " receipt=" + result.ProvisionReceiptPath)
	}
	if result.RetirementPreviewCommand != "" {
		add("run retirementPreviewCommand with -WhatIf to inspect exact verification workspace cleanup: " + result.RetirementPreviewCommand)
		add("run the returned expected-hash retirement Apply command only after reviewing the exact deletion plan")
		add("after retirement, rerun /rekit status or release-check to confirm pack-memory candidate closure")
	} else {
		add("no retirement preview command is available; keep the verification proof and rerun /rekit status or release-check for downstream closure")
	}
	addNextSteps()
	add("do not continue pack-memory downstream closure until verification proof and any required retirement receipt are accounted for")
	return steps
}

func DraftCandidateDecisions(repoRoot, caseRoot, pack string, opt CandidateDecisionDraftOptions) (CandidateDecisionDraftResult, error) {
	inst, err := instance.AssertAttached(caseRoot, repoRoot, pack)
	if err != nil {
		return CandidateDecisionDraftResult{}, err
	}
	if opt.WhatIf && strings.TrimSpace(opt.ExpectedDecisionSHA256) != "" {
		return CandidateDecisionDraftResult{}, fmt.Errorf("candidate decision draft WhatIf does not accept -ExpectedDecisionSha256")
	}
	if !opt.WhatIf {
		decoded, decodeErr := hex.DecodeString(strings.TrimSpace(opt.ExpectedDecisionSHA256))
		if decodeErr != nil || len(decoded) != sha256.Size {
			return CandidateDecisionDraftResult{}, fmt.Errorf("candidate decision draft Apply requires a valid -ExpectedDecisionSha256 from WhatIf")
		}
	}
	prepared, err := prepareCandidateDecisionDraft(repoRoot, inst.CaseRoot, pack, opt)
	if err != nil {
		return CandidateDecisionDraftResult{}, err
	}
	result := prepared.result
	if opt.WhatIf {
		return result, nil
	}
	if !strings.EqualFold(result.DecisionSHA256, strings.TrimSpace(opt.ExpectedDecisionSHA256)) {
		return CandidateDecisionDraftResult{}, fmt.Errorf("candidate decision draft changed after preview")
	}
	already, err := writeCandidateDecisionDraftFile(inst.CaseRoot, result.DecisionPath, prepared.decisionBytes)
	if err != nil {
		return CandidateDecisionDraftResult{}, err
	}
	result.Applied = true
	result.AlreadyWritten = already
	return result, nil
}

func prepareCandidateDecisionDraft(repoRoot, caseRoot, pack string, opt CandidateDecisionDraftOptions) (preparedCandidateDecisionDraft, error) {
	packetPath, packetBytes, err := readStrictCandidateDecisionFile(opt.PacketPath, "candidate review packet")
	if err != nil {
		return preparedCandidateDecisionDraft{}, err
	}
	decisionPath, err := candidateDecisionDraftPath(caseRoot, opt.DecisionPath)
	if err != nil {
		return preparedCandidateDecisionDraft{}, err
	}
	decisionMode := strings.ToLower(strings.TrimSpace(opt.Decision))
	if decisionMode == "" {
		return preparedCandidateDecisionDraft{}, fmt.Errorf("candidate decision draft requires -Decision")
	}
	reason := strings.TrimSpace(opt.Reason)
	actor := strings.TrimSpace(opt.Actor)
	if reason == "" || actor == "" {
		return preparedCandidateDecisionDraft{}, fmt.Errorf("candidate decision draft requires -Reason and -Actor")
	}
	evidenceRefs, evidenceArg, err := candidateDecisionDraftEvidenceRefs(repoRoot, caseRoot, opt.EvidenceRefs)
	if err != nil {
		return preparedCandidateDecisionDraft{}, err
	}
	var packet CandidateReviewPacket
	if err := decodeStrictJSON(packetBytes, &packet); err != nil {
		return preparedCandidateDecisionDraft{}, fmt.Errorf("decode candidate review packet: %w", err)
	}
	packetHash := sha256Hex(packetBytes)
	if packet.SchemaVersion != 1 || packet.Kind != "pack-memory-candidate-review" || packet.Command != "promote" {
		return preparedCandidateDecisionDraft{}, fmt.Errorf("candidate review packet has unsupported schema/kind/command")
	}
	inst, err := instance.AssertAttached(caseRoot, repoRoot, pack)
	if err != nil {
		return preparedCandidateDecisionDraft{}, err
	}
	m, err := manifest.Load(repoRoot, pack)
	if err != nil {
		return preparedCandidateDecisionDraft{}, err
	}
	canonicalCandidateRoot := filepath.Join(m.PackRoot, "promote-candidates")
	canonicalToolingRoot := filepath.Join(m.PackRoot, "tooling", "candidates")
	canonicalIndexPath := filepath.Join(canonicalCandidateRoot, "index.json")
	if !sameCandidateDecisionPath(packet.CandidateResult.RepoRoot, repoRoot) || !sameCandidateDecisionPath(packet.CandidateResult.CaseRoot, inst.CaseRoot) || packet.CandidateResult.Pack != pack {
		return preparedCandidateDecisionDraft{}, fmt.Errorf("candidate review packet repo/case/pack binding mismatch")
	}
	if !sameCandidateDecisionPath(packet.CandidateResult.CandidateRoot, canonicalCandidateRoot) || !sameCandidateDecisionPath(packet.CandidateResult.ToolingRoot, canonicalToolingRoot) {
		return preparedCandidateDecisionDraft{}, fmt.Errorf("candidate review packet roots do not match canonical pack candidate roots")
	}
	if !candidateDecisionPacketIndexValid(packet, canonicalIndexPath) {
		return preparedCandidateDecisionDraft{}, fmt.Errorf("candidate review packet indexPath does not match canonical index")
	}
	allowMissingCandidateRoot := strings.TrimSpace(packet.CandidateResult.IndexPath) == ""
	for _, path := range []string{m.PackRoot, canonicalCandidateRoot, canonicalToolingRoot, canonicalIndexPath} {
		allowMissing := path == canonicalIndexPath || path == canonicalCandidateRoot && allowMissingCandidateRoot
		if err := rejectCandidateDecisionSymlinkPath(m.PackRoot, path, allowMissing); err != nil {
			return preparedCandidateDecisionDraft{}, err
		}
	}
	indexEntries, err := readCandidateIndex(canonicalIndexPath)
	if err != nil {
		return preparedCandidateDecisionDraft{}, err
	}
	managedTargets := map[string]string{}
	for _, rel := range m.ManagedFiles {
		target, err := m.SourcePath(rel)
		if err != nil {
			return preparedCandidateDecisionDraft{}, err
		}
		managedTargets[filepath.ToSlash(rel)] = filepath.Clean(target)
	}
	reviewKeys := map[string]bool{}
	for _, item := range packet.CandidateResult.ReviewPlan.ReviewItems {
		key, include, err := candidateReviewAuthorityKey(item)
		if err != nil {
			return preparedCandidateDecisionDraft{}, err
		}
		if !include {
			continue
		}
		if reviewKeys[key] {
			return preparedCandidateDecisionDraft{}, fmt.Errorf("duplicate candidate review for %s", item.CandidatePath)
		}
		reviewKeys[key] = true
	}
	decisions := []CandidateDecisionItem{}
	for _, item := range packet.CandidateResult.ReviewPlan.ReviewItems {
		if item.ReviewDecision != "pending-review" {
			continue
		}
		decision, err := candidateDecisionDraftItemDecision(decisionMode, item)
		if err != nil {
			return preparedCandidateDecisionDraft{}, err
		}
		candidatePath := filepath.Clean(strings.TrimSpace(item.CandidatePath))
		candidateRoot := packet.CandidateResult.CandidateRoot
		if item.Kind == "tooling-candidate-source" {
			candidateRoot = packet.CandidateResult.ToolingRoot
		}
		if err := assertInsideRoot(candidateRoot, candidatePath); err != nil {
			return preparedCandidateDecisionDraft{}, err
		}
		if err := rejectCandidateDecisionSymlinkPath(m.PackRoot, candidatePath, false); err != nil {
			return preparedCandidateDecisionDraft{}, err
		}
		if item.Kind == "managed-doc" && !candidateIndexContains(indexEntries, item.Path, candidatePath) {
			return preparedCandidateDecisionDraft{}, fmt.Errorf("managed candidate %s is not bound to path %s in indexPath", candidatePath, item.Path)
		}
		if item.Kind == "managed-doc" {
			expectedTarget, ok := managedTargets[filepath.ToSlash(item.Path)]
			if !ok || !sameCandidateDecisionPath(expectedTarget, item.PackTarget) {
				return preparedCandidateDecisionDraft{}, fmt.Errorf("managed candidate %s packTarget does not match manifest path %s", candidatePath, item.Path)
			}
		}
		state, err := refsf.ClassifyNonEmptyRegularFile(candidatePath)
		if err != nil {
			return preparedCandidateDecisionDraft{}, err
		}
		if state != refsf.RegularFileReady {
			return preparedCandidateDecisionDraft{}, fmt.Errorf("candidate %s must be a non-empty regular file, got %s", candidatePath, state)
		}
		if !candidateWriteMatches(packet.CandidateResult.Writes, item.Path, item.Kind, candidatePath) {
			return preparedCandidateDecisionDraft{}, fmt.Errorf("candidate %s is not bound to a create-candidate write in packet", candidatePath)
		}
		decisionItem := CandidateDecisionItem{
			CandidatePath: candidatePath,
			Decision:      decision,
			CandidateHash: fileSHA256(candidatePath),
			Reason:        reason,
			Actor:         actor,
			EvidenceRefs:  append([]CandidateDecisionEvidence(nil), evidenceRefs...),
		}
		if decision == "accept" {
			if item.Kind != "managed-doc" || strings.TrimSpace(item.PackTarget) == "" {
				return preparedCandidateDecisionDraft{}, fmt.Errorf("candidate %s kind %s cannot be accepted automatically; merge tooling candidates manually", candidatePath, item.Kind)
			}
			if err := assertInsideRoot(m.PackRoot, item.PackTarget); err != nil {
				return preparedCandidateDecisionDraft{}, err
			}
			if err := rejectCandidateDecisionSymlinkPath(m.PackRoot, item.PackTarget, false); err != nil {
				return preparedCandidateDecisionDraft{}, err
			}
			decisionItem.PackTargetHash = fileSHA256(item.PackTarget)
		}
		decisions = append(decisions, decisionItem)
	}
	if len(decisions) == 0 {
		return preparedCandidateDecisionDraft{}, fmt.Errorf("candidate decision draft found no pending review items")
	}
	sort.Slice(decisions, func(i, j int) bool { return decisions[i].CandidatePath < decisions[j].CandidatePath })
	decisionFile := CandidateDecisionFile{SchemaVersion: 1, Kind: "pack-memory-candidate-decisions", PacketHash: packetHash, Decisions: decisions}
	decisionBytes, err := json.MarshalIndent(decisionFile, "", "  ")
	if err != nil {
		return preparedCandidateDecisionDraft{}, err
	}
	decisionBytes = append(decisionBytes, '\n')
	accepted, rejected, superseded := candidateDecisionDraftCounts(decisions)
	mode := "candidate-decision-draft"
	if opt.WhatIf {
		mode = "candidate-decision-draft-preview"
	}
	decisionSHA := sha256Hex(decisionBytes)
	previewCommand := candidateDecisionDraftCommand(packetPath, decisionPath, decisionMode, reason, actor, evidenceArg, "", false)
	applyCommand := candidateDecisionDraftCommand(packetPath, decisionPath, decisionMode, reason, actor, evidenceArg, decisionSHA, true)
	result := CandidateDecisionDraftResult{
		SchemaVersion:  1,
		Command:        "promote",
		Mode:           mode,
		CaseRoot:       inst.CaseRoot,
		RepoRoot:       repoRoot,
		Pack:           pack,
		PacketPath:     packetPath,
		DecisionPath:   decisionPath,
		PacketHash:     packetHash,
		DecisionSHA256: decisionSHA,
		IsMutation:     !opt.WhatIf,
		Decision:       decisionMode,
		DecisionCount:  len(decisions),
		Accepted:       accepted,
		Rejected:       rejected,
		Superseded:     superseded,
		Decisions:      decisions,
		PreviewCommand: previewCommand,
		ApplyCommand:   applyCommand,
		Boundary: []string{
			"candidate decision draft writes only one case-local decision JSON file on Apply",
			"draft decisions are bound to the exact durable review packet, current candidate bytes, pack target hash when accepting, and non-empty evidenceRefs",
			"drafting does not merge candidates, cleanup candidate files, run doctor/init/reconsume, write authority/confirmed, or execute heavy tools",
		},
	}
	if opt.WhatIf {
		result.NextSteps = []string{"inspect generated decisions and exact decision SHA-256, then rerun the returned draft command with -ExpectedDecisionSha256 and -Apply"}
	} else {
		result.NextSteps = []string{"run promote -CandidateDecisionPath with -WhatIf to preview candidate merge/cleanup before -Apply"}
	}
	return preparedCandidateDecisionDraft{result: result, decisionBytes: decisionBytes}, nil
}

func candidateDecisionDraftPath(caseRoot, decisionInput string) (string, error) {
	if strings.TrimSpace(decisionInput) == "" {
		return "", fmt.Errorf("candidate decision draft requires -CandidateDecisionPath")
	}
	decisionPath, err := filepath.Abs(strings.TrimSpace(decisionInput))
	if err != nil {
		return "", err
	}
	if err := assertInsideRoot(caseRoot, decisionPath); err != nil {
		return "", fmt.Errorf("candidate decision draft path must stay under the attached case: %w", err)
	}
	if err := rejectCandidateDecisionSymlinkPath(caseRoot, filepath.Dir(decisionPath), false); err != nil {
		return "", err
	}
	return decisionPath, nil
}

func candidateDecisionDraftItemDecision(decisionMode string, item CandidateReviewItem) (string, error) {
	switch decisionMode {
	case "accept":
		if item.Kind != "managed-doc" {
			return "", fmt.Errorf("candidate %s kind %s cannot be accepted automatically; use reject or superseded for tooling candidates", item.CandidatePath, item.Kind)
		}
		return "accept", nil
	case "accept-managed", "accept-managed-reject-tooling":
		if item.Kind == "managed-doc" {
			return "accept", nil
		}
		return "reject", nil
	case "reject", "superseded":
		return decisionMode, nil
	default:
		return "", fmt.Errorf("unsupported candidate decision draft mode %q; allowed: accept, accept-managed-reject-tooling, reject, superseded", decisionMode)
	}
}

func candidateDecisionDraftEvidenceRefs(repoRoot, caseRoot, refs string) ([]CandidateDecisionEvidence, string, error) {
	parts := splitCandidateDecisionDraftList(refs)
	if len(parts) == 0 {
		return nil, "", fmt.Errorf("candidate decision draft requires -EvidenceRefs")
	}
	evidence := make([]CandidateDecisionEvidence, 0, len(parts))
	for _, ref := range parts {
		item, err := candidateDecisionDraftEvidenceRef(repoRoot, caseRoot, ref)
		if err != nil {
			return nil, "", err
		}
		evidence = append(evidence, item)
	}
	return evidence, candidateDecisionEvidenceRefsArg(evidence), nil
}

func candidateDecisionDraftEvidenceRef(repoRoot, caseRoot, ref string) (CandidateDecisionEvidence, error) {
	value := strings.TrimSpace(ref)
	if value == "" {
		return CandidateDecisionEvidence{}, fmt.Errorf("candidate decision draft evidence ref is empty")
	}
	full := value
	if !filepath.IsAbs(full) {
		full = filepath.Join(caseRoot, filepath.FromSlash(value))
	}
	full, err := filepath.Abs(full)
	if err != nil {
		return CandidateDecisionEvidence{}, err
	}
	insideRepo := assertInsideRoot(repoRoot, full) == nil
	insideCase := assertInsideRoot(caseRoot, full) == nil
	if !insideRepo && !insideCase {
		return CandidateDecisionEvidence{}, fmt.Errorf("evidenceRef %q must stay under repoRoot or caseRoot", value)
	}
	evidenceRoot := caseRoot
	if insideRepo && !insideCase {
		evidenceRoot = repoRoot
	}
	if err := rejectCandidateDecisionSymlinkPath(evidenceRoot, full, false); err != nil {
		return CandidateDecisionEvidence{}, err
	}
	state, err := refsf.ClassifyNonEmptyRegularFile(full)
	if err != nil {
		return CandidateDecisionEvidence{}, err
	}
	if state != refsf.RegularFileReady {
		return CandidateDecisionEvidence{}, fmt.Errorf("evidenceRef %q must be a non-empty regular file, got %s", value, state)
	}
	stored := value
	if insideCase {
		if rel, err := filepath.Rel(caseRoot, full); err == nil && rel != ".." && !strings.HasPrefix(rel, ".."+string(filepath.Separator)) && !filepath.IsAbs(rel) {
			stored = filepath.ToSlash(rel)
		}
	}
	return CandidateDecisionEvidence{Path: stored, SHA256: fileSHA256(full)}, nil
}

func splitCandidateDecisionDraftList(value string) []string {
	items := []string{}
	for _, part := range strings.FieldsFunc(value, func(r rune) bool { return r == ',' || r == ';' }) {
		part = strings.TrimSpace(part)
		if part != "" {
			items = append(items, part)
		}
	}
	return items
}

func candidateDecisionEvidenceRefsArg(evidence []CandidateDecisionEvidence) string {
	parts := make([]string, 0, len(evidence))
	for _, item := range evidence {
		parts = append(parts, item.Path)
	}
	return strings.Join(parts, ",")
}

func candidateDecisionDraftCounts(decisions []CandidateDecisionItem) (accepted, rejected, superseded int) {
	for _, decision := range decisions {
		switch decision.Decision {
		case "accept":
			accepted++
		case "reject":
			rejected++
		case "superseded":
			superseded++
		}
	}
	return accepted, rejected, superseded
}

func candidateDecisionDraftCommand(packetPath, decisionPath, decision, reason, actor, evidenceRefs, expectedDecisionSHA256 string, apply bool) string {
	base := "/rekit promote -PacketPath " + quoteCandidateDecisionArg(packetPath) +
		" -CandidateDecisionPath " + quoteCandidateDecisionArg(decisionPath) +
		" -DraftCandidateDecision -Decision " + quoteCandidateDecisionArg(decision) +
		" -Reason " + quoteCandidateDecisionArg(reason) +
		" -Actor " + quoteCandidateDecisionArg(actor) +
		" -EvidenceRefs " + quoteCandidateDecisionArg(evidenceRefs)
	if apply {
		return base + " -ExpectedDecisionSha256 " + quoteCandidateDecisionArg(expectedDecisionSHA256) + " -Apply -Format json"
	}
	return base + " -WhatIf -Format json"
}

func writeCandidateDecisionDraftFile(caseRoot, path string, data []byte) (bool, error) {
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
		return false, fmt.Errorf("existing candidate decision draft must be a non-empty regular file: %s", path)
	}
	existing, readErr := os.ReadFile(path)
	if readErr != nil {
		return false, readErr
	}
	if !bytes.Equal(existing, data) {
		return false, fmt.Errorf("existing candidate decision draft does not match replay: %s", path)
	}
	return true, nil
}

func ApplyCandidateDecisions(repoRoot, caseRoot, pack string, opt CandidateDecisionOptions) (CandidateDecisionResult, error) {
	if err := prepareCandidateDecisionPlan(opt); err != nil {
		return CandidateDecisionResult{}, err
	}
	if opt.WhatIf {
		plan, err := planCandidateDecisions(repoRoot, caseRoot, pack, opt)
		if err != nil {
			return CandidateDecisionResult{}, err
		}
		plan.result.DecisionRunbookSteps = candidateDecisionRunbookSteps(plan.result)
		return plan.result, nil
	}
	m, err := manifest.Load(repoRoot, pack)
	if err != nil {
		return CandidateDecisionResult{}, err
	}
	candidateRoot := filepath.Join(m.PackRoot, "promote-candidates")
	if err := rejectCandidateDecisionSymlinkPath(m.PackRoot, candidateRoot, true); err != nil {
		return CandidateDecisionResult{}, err
	}
	if err := os.MkdirAll(candidateRoot, 0o755); err != nil {
		return CandidateDecisionResult{}, err
	}
	unlock, err := acquireCandidateDecisionLock(candidateRoot)
	if err != nil {
		return CandidateDecisionResult{}, err
	}
	defer unlock()
	if result, handled, err := recoverCandidateDecisionTransaction(repoRoot, caseRoot, pack, opt, m); handled {
		return result, err
	}
	plan, err := planCandidateDecisions(repoRoot, caseRoot, pack, opt)
	if err != nil {
		return CandidateDecisionResult{}, err
	}
	return applyCandidateDecisionPlan(plan)
}

func prepareCandidateDecisionPlan(opt CandidateDecisionOptions) error {
	_, packetBytes, err := readStrictCandidateDecisionFile(opt.PacketPath, "candidate review packet")
	if err != nil {
		return err
	}
	_, decisionBytes, err := readStrictCandidateDecisionFile(opt.DecisionPath, "candidate decision")
	if err != nil {
		return err
	}
	var packet CandidateReviewPacket
	if err := decodeStrictJSON(packetBytes, &packet); err != nil {
		return fmt.Errorf("decode candidate review packet: %w", err)
	}
	var decisions CandidateDecisionFile
	if err := decodeStrictJSON(decisionBytes, &decisions); err != nil {
		return fmt.Errorf("decode candidate decision: %w", err)
	}
	byCandidate := map[string]CandidateReviewItem{}
	pendingReviewKeys := map[string]bool{}
	for _, item := range packet.CandidateResult.ReviewPlan.ReviewItems {
		key, include, err := candidateReviewAuthorityKey(item)
		if err != nil {
			return err
		}
		if !include {
			continue
		}
		if _, duplicate := byCandidate[key]; duplicate {
			return fmt.Errorf("duplicate candidate review for %s", item.CandidatePath)
		}
		byCandidate[key] = item
		if item.ReviewDecision == "pending-review" {
			pendingReviewKeys[key] = true
		}
	}
	decisionKeys := map[string]bool{}
	for _, decision := range decisions.Decisions {
		key, err := candidateDecisionAuthorityKey(decision.CandidatePath)
		if err != nil {
			return fmt.Errorf("candidate decision has invalid candidatePath %q", decision.CandidatePath)
		}
		if decisionKeys[key] {
			return fmt.Errorf("duplicate candidate decision for %s", decision.CandidatePath)
		}
		decisionKeys[key] = true
	}
	if !reflect.DeepEqual(decisionKeys, pendingReviewKeys) {
		return fmt.Errorf("candidate decisions do not exactly cover pending review items")
	}
	return nil
}

func planCandidateDecisions(repoRoot, caseRoot, pack string, opt CandidateDecisionOptions) (candidateDecisionPlan, error) {
	inst, err := instance.AssertAttached(caseRoot, repoRoot, pack)
	if err != nil {
		return candidateDecisionPlan{}, err
	}
	m, err := manifest.Load(repoRoot, pack)
	if err != nil {
		return candidateDecisionPlan{}, err
	}
	packetPath, packetBytes, err := readStrictCandidateDecisionFile(opt.PacketPath, "candidate review packet")
	if err != nil {
		return candidateDecisionPlan{}, err
	}
	decisionPath, decisionBytes, err := readStrictCandidateDecisionFile(opt.DecisionPath, "candidate decision")
	if err != nil {
		return candidateDecisionPlan{}, err
	}
	var packet CandidateReviewPacket
	if err := decodeStrictJSON(packetBytes, &packet); err != nil {
		return candidateDecisionPlan{}, fmt.Errorf("decode candidate review packet: %w", err)
	}
	var decisions CandidateDecisionFile
	if err := decodeStrictJSON(decisionBytes, &decisions); err != nil {
		return candidateDecisionPlan{}, fmt.Errorf("decode candidate decision: %w", err)
	}
	packetHash := sha256Hex(packetBytes)
	if packet.SchemaVersion != 1 || packet.Kind != "pack-memory-candidate-review" || packet.Command != "promote" {
		return candidateDecisionPlan{}, fmt.Errorf("candidate review packet has unsupported schema/kind/command")
	}
	if decisions.SchemaVersion != 1 || decisions.Kind != "pack-memory-candidate-decisions" {
		return candidateDecisionPlan{}, fmt.Errorf("candidate decision file has unsupported schema/kind")
	}
	if !strings.EqualFold(strings.TrimSpace(decisions.PacketHash), packetHash) {
		return candidateDecisionPlan{}, fmt.Errorf("candidate decision packetHash %q does not match packet %s", decisions.PacketHash, packetHash)
	}
	if !sameCandidateDecisionPath(packet.CandidateResult.RepoRoot, repoRoot) || !sameCandidateDecisionPath(packet.CandidateResult.CaseRoot, inst.CaseRoot) || packet.CandidateResult.Pack != pack {
		return candidateDecisionPlan{}, fmt.Errorf("candidate review packet repo/case/pack binding mismatch")
	}
	if len(decisions.Decisions) == 0 {
		return candidateDecisionPlan{}, fmt.Errorf("candidate decision file contains no decisions")
	}
	canonicalCandidateRoot := filepath.Join(m.PackRoot, "promote-candidates")
	canonicalToolingRoot := filepath.Join(m.PackRoot, "tooling", "candidates")
	canonicalIndexPath := filepath.Join(canonicalCandidateRoot, "index.json")
	if !sameCandidateDecisionPath(packet.CandidateResult.CandidateRoot, canonicalCandidateRoot) || !sameCandidateDecisionPath(packet.CandidateResult.ToolingRoot, canonicalToolingRoot) {
		return candidateDecisionPlan{}, fmt.Errorf("candidate review packet roots do not match canonical pack candidate roots")
	}
	if !candidateDecisionPacketIndexValid(packet, canonicalIndexPath) {
		return candidateDecisionPlan{}, fmt.Errorf("candidate review packet indexPath does not match canonical index")
	}
	allowMissingCandidateRoot := strings.TrimSpace(packet.CandidateResult.IndexPath) == ""
	for _, path := range []string{m.PackRoot, canonicalCandidateRoot, canonicalToolingRoot, canonicalIndexPath} {
		allowMissing := path == canonicalIndexPath || path == canonicalCandidateRoot && allowMissingCandidateRoot
		if err := rejectCandidateDecisionSymlinkPath(m.PackRoot, path, allowMissing); err != nil {
			return candidateDecisionPlan{}, err
		}
	}
	indexEntries, err := readCandidateIndex(canonicalIndexPath)
	if err != nil {
		return candidateDecisionPlan{}, err
	}
	managedTargets := map[string]string{}
	for _, rel := range m.ManagedFiles {
		target, err := m.SourcePath(rel)
		if err != nil {
			return candidateDecisionPlan{}, err
		}
		managedTargets[filepath.ToSlash(rel)] = filepath.Clean(target)
	}
	byCandidate := map[string]CandidateReviewItem{}
	for _, item := range packet.CandidateResult.ReviewPlan.ReviewItems {
		if strings.TrimSpace(item.CandidatePath) == "" {
			continue
		}
		key, _ := candidateDecisionAuthorityKey(item.CandidatePath)
		byCandidate[key] = item
	}
	result := CandidateDecisionResult{
		SchemaVersion: 1,
		Command:       "promote",
		Mode:          "candidate-decision",
		CaseRoot:      inst.CaseRoot,
		RepoRoot:      repoRoot,
		Pack:          pack,
		PacketPath:    packetPath,
		DecisionPath:  decisionPath,
		PacketHash:    packetHash,
		IsMutation:    !opt.WhatIf,
		Applied:       false,
		IndexPath:     canonicalIndexPath,
		Actions:       []CandidateDecisionAction{},
		Boundary: []string{
			"candidate decisions are bound to the exact durable review packet and reviewed candidate/pack-target hashes",
			"accepted managed-doc candidates update only their exact packet packTarget; tooling acceptance remains manual",
			"rejected or superseded candidates are cleaned up and matching index entries are removed",
			"no authority/confirmed writes and no heavy-tool execution",
		},
	}
	if opt.WhatIf {
		result.Mode = "candidate-decision-preview"
		result.NextSteps = []string{"inspect every planned action and evidenceRefs, then rerun the identical command with -Apply"}
	}
	planned := make([]candidateDecisionPlanItem, 0, len(decisions.Decisions))
	for _, decision := range decisions.Decisions {
		key, _ := candidateDecisionAuthorityKey(decision.CandidatePath)
		candidatePath := filepath.Clean(strings.TrimSpace(decision.CandidatePath))
		reviewItem, ok := byCandidate[key]
		if !ok || reviewItem.ReviewDecision != "pending-review" {
			return candidateDecisionPlan{}, fmt.Errorf("candidate %s is not a pending review item in packet", candidatePath)
		}
		candidateRoot := packet.CandidateResult.CandidateRoot
		if reviewItem.Kind == "tooling-candidate-source" {
			candidateRoot = packet.CandidateResult.ToolingRoot
		}
		if err := assertInsideRoot(candidateRoot, candidatePath); err != nil {
			return candidateDecisionPlan{}, err
		}
		if err := rejectCandidateDecisionSymlinkPath(m.PackRoot, candidatePath, false); err != nil {
			return candidateDecisionPlan{}, err
		}
		if reviewItem.Kind == "managed-doc" && !candidateIndexContains(indexEntries, reviewItem.Path, candidatePath) {
			return candidateDecisionPlan{}, fmt.Errorf("managed candidate %s is not bound to path %s in indexPath", candidatePath, reviewItem.Path)
		}
		if reviewItem.Kind == "managed-doc" {
			expectedTarget, ok := managedTargets[filepath.ToSlash(reviewItem.Path)]
			if !ok || !sameCandidateDecisionPath(expectedTarget, reviewItem.PackTarget) {
				return candidateDecisionPlan{}, fmt.Errorf("managed candidate %s packTarget does not match manifest path %s", candidatePath, reviewItem.Path)
			}
		}
		state, err := refsf.ClassifyNonEmptyRegularFile(candidatePath)
		if err != nil {
			return candidateDecisionPlan{}, err
		}
		if state != refsf.RegularFileReady {
			return candidateDecisionPlan{}, fmt.Errorf("candidate %s must be a non-empty regular file, got %s", candidatePath, state)
		}
		if !candidateWriteMatches(packet.CandidateResult.Writes, reviewItem.Path, reviewItem.Kind, candidatePath) {
			return candidateDecisionPlan{}, fmt.Errorf("candidate %s is not bound to a create-candidate write in packet", candidatePath)
		}
		if !strings.EqualFold(strings.TrimSpace(decision.CandidateHash), fileSHA256(candidatePath)) {
			return candidateDecisionPlan{}, fmt.Errorf("candidate hash mismatch for %s", candidatePath)
		}
		if strings.TrimSpace(decision.Reason) == "" || strings.TrimSpace(decision.Actor) == "" || len(decision.EvidenceRefs) == 0 {
			return candidateDecisionPlan{}, fmt.Errorf("candidate decision for %s requires reason, actor, and evidenceRefs", candidatePath)
		}
		evidencePaths := make([]string, 0, len(decision.EvidenceRefs))
		for _, evidenceRef := range decision.EvidenceRefs {
			full, err := validateCandidateDecisionEvidenceRef(repoRoot, inst.CaseRoot, evidenceRef)
			if err != nil {
				return candidateDecisionPlan{}, fmt.Errorf("candidate decision for %s: %w", candidatePath, err)
			}
			evidencePaths = append(evidencePaths, full)
		}
		decision.Decision = strings.ToLower(strings.TrimSpace(decision.Decision))
		action := CandidateDecisionAction{CandidatePath: candidatePath, Kind: reviewItem.Kind, Decision: decision.Decision, PackTarget: reviewItem.PackTarget, EvidenceRefs: evidencePaths}
		switch decision.Decision {
		case "accept":
			if reviewItem.Kind != "managed-doc" || strings.TrimSpace(reviewItem.PackTarget) == "" {
				return candidateDecisionPlan{}, fmt.Errorf("candidate %s kind %s cannot be accepted automatically; merge tooling candidates manually", candidatePath, reviewItem.Kind)
			}
			if err := assertInsideRoot(m.PackRoot, reviewItem.PackTarget); err != nil {
				return candidateDecisionPlan{}, err
			}
			if err := rejectCandidateDecisionSymlinkPath(m.PackRoot, reviewItem.PackTarget, false); err != nil {
				return candidateDecisionPlan{}, err
			}
			if !strings.EqualFold(strings.TrimSpace(decision.PackTargetHash), fileSHA256(reviewItem.PackTarget)) {
				return candidateDecisionPlan{}, fmt.Errorf("pack target hash mismatch for %s", reviewItem.PackTarget)
			}
			action.Action = "merge-accepted-candidate-and-cleanup"
			result.Accepted++
		case "reject":
			action.Action = "cleanup-rejected-candidate"
			result.Rejected++
		case "superseded":
			action.Action = "cleanup-superseded-candidate"
			result.Superseded++
		default:
			return candidateDecisionPlan{}, fmt.Errorf("unsupported candidate decision %q for %s", decision.Decision, candidatePath)
		}
		result.Actions = append(result.Actions, action)
		planned = append(planned, candidateDecisionPlanItem{decision: decision, review: reviewItem, action: action})
	}
	sort.Slice(planned, func(i, j int) bool { return planned[i].action.CandidatePath < planned[j].action.CandidatePath })
	sort.Slice(result.Actions, func(i, j int) bool { return result.Actions[i].CandidatePath < result.Actions[j].CandidatePath })
	indexHash := ""
	indexExisted := false
	if indexBytes, err := os.ReadFile(canonicalIndexPath); err == nil {
		indexHash = sha256Hex(indexBytes)
		indexExisted = true
	} else if !os.IsNotExist(err) {
		return candidateDecisionPlan{}, err
	}
	return candidateDecisionPlan{
		result:       result,
		packet:       packet,
		decisions:    decisions,
		decisionHash: sha256Hex(decisionBytes),
		indexHash:    indexHash,
		indexExisted: indexExisted,
		indexEntries: indexEntries,
		items:        planned,
	}, nil
}

func applyCandidateDecisionPlan(plan candidateDecisionPlan) (CandidateDecisionResult, error) {
	result := plan.result
	backupParent := filepath.Join(plan.packet.CandidateResult.CandidateRoot, ".decision-backup")
	backupRoot := ""
	fail := func(action string, err error) (CandidateDecisionResult, error) {
		result.FailedAction = action
		result.RecoveryRequired = true
		result.NextSteps = []string{"inspect backupRoot and recoveryActions before retrying the reviewed decision"}
		result.DecisionRunbookSteps = candidateDecisionRunbookSteps(result)
		return result, &CandidateDecisionApplyError{Result: result, Err: fmt.Errorf("candidate decision apply %s: %w", action, err)}
	}
	if err := verifyCandidateDecisionPlan(plan); err != nil {
		return fail("pre-commit verification", err)
	}
	if err := rejectCandidateDecisionSymlinkPath(plan.packet.CandidateResult.CandidateRoot, backupParent, true); err != nil {
		return fail("validate backup parent", err)
	}
	if err := os.MkdirAll(backupParent, 0o755); err != nil {
		return fail("create backup parent", err)
	}
	if err := rejectCandidateDecisionSymlinkPath(plan.packet.CandidateResult.CandidateRoot, backupParent, false); err != nil {
		return fail("validate backup parent", err)
	}
	backupRoot, err := os.MkdirTemp(backupParent, shortHash(plan.result.PacketHash+plan.decisionHash)+"-")
	if err != nil {
		return fail("create backup root", err)
	}
	if err := syncCandidateDecisionDirectory(backupParent); err != nil {
		return fail("sync backup parent", err)
	}
	result.BackupRoot = backupRoot
	if err := rejectCandidateDecisionSymlinkPath(plan.packet.CandidateResult.CandidateRoot, backupRoot, false); err != nil {
		return fail("validate backup root", err)
	}

	indexBackup := filepath.Join(backupRoot, "candidate-index.json")
	if plan.indexExisted {
		if err := copyFileExclusive(plan.result.IndexPath, indexBackup); err != nil {
			return fail("stage candidate index backup", err)
		}
		result.RecoveryActions = append(result.RecoveryActions, fmt.Sprintf("restore %s to %s", indexBackup, plan.result.IndexPath))
	}
	for idx := range plan.items {
		item := &plan.items[idx]
		item.action.CandidateBackupPath = filepath.Join(backupRoot, "candidate-"+shortHash(item.action.CandidatePath)+filepath.Ext(item.action.CandidatePath))
		if err := copyFileExclusive(item.action.CandidatePath, item.action.CandidateBackupPath); err != nil {
			return fail("stage candidate backup", err)
		}
		result.RecoveryActions = append(result.RecoveryActions, fmt.Sprintf("restore %s to %s", item.action.CandidateBackupPath, item.action.CandidatePath))
		if item.decision.Decision == "accept" {
			item.action.TargetBackupPath = filepath.Join(backupRoot, "target-"+shortHash(item.action.PackTarget)+filepath.Ext(item.action.PackTarget))
			if err := copyFileExclusive(item.action.PackTarget, item.action.TargetBackupPath); err != nil {
				return fail("stage pack target backup", err)
			}
			result.RecoveryActions = append(result.RecoveryActions, fmt.Sprintf("restore %s to %s", item.action.TargetBackupPath, item.action.PackTarget))
		}
		copyCandidateDecisionActionBackups(result.Actions, item.action)
	}
	transaction := candidateDecisionTransaction{
		SchemaVersion:   1,
		Kind:            "pack-memory-candidate-decision-transaction",
		PacketHash:      plan.result.PacketHash,
		DecisionHash:    plan.decisionHash,
		IndexExisted:    plan.indexExisted,
		IndexBackupPath: indexBackup,
		Result:          result,
		Actions:         append([]CandidateDecisionAction(nil), result.Actions...),
	}
	if err := writeCandidateDecisionTransaction(filepath.Join(backupRoot, "transaction.json"), transaction); err != nil {
		return fail("write transaction journal", err)
	}

	newIndexEntries := append([]candidateIndexEntry(nil), plan.indexEntries...)
	for _, item := range plan.items {
		newIndexEntries = removeCandidateIndexEntry(newIndexEntries, item.action.CandidatePath)
	}
	mutated := make([]candidateDecisionPlanItem, 0, len(plan.items))
	for _, item := range plan.items {
		if item.decision.Decision == "accept" {
			candidateBytes, err := os.ReadFile(item.action.CandidateBackupPath)
			if err != nil {
				return rollbackCandidateDecision(result, mutated, plan, "read staged candidate", err)
			}
			if err := writeFileAtomic(item.action.PackTarget, candidateBytes, 0o644); err != nil {
				return rollbackCandidateDecision(result, mutated, plan, "write pack target", err)
			}
		}
		mutated = append(mutated, item)
	}
	if err := writeCandidateIndexAtomic(plan.result.IndexPath, newIndexEntries); err != nil {
		return rollbackCandidateDecision(result, mutated, plan, "write candidate index", err)
	}
	for _, item := range plan.items {
		if err := removeCandidateDecisionFile(item.action.CandidatePath); err != nil {
			return rollbackCandidateDecision(result, mutated, plan, "cleanup candidate", err)
		}
		mutated = markCandidateDecisionRemoved(mutated, item.action.CandidatePath)
	}
	receipt, nextSteps, err := writeCandidateDecisionReceipt(plan, result)
	if err != nil {
		return rollbackCandidateDecision(result, mutated, plan, "write candidate decision receipt", err)
	}
	result.ReceiptPath = receipt.ReceiptPath
	result.Receipt = &receipt
	result.NextSteps = nextSteps
	result.Applied = true
	result.RecoveryRequired = false
	result.FailedAction = ""
	result.DecisionRunbookSteps = candidateDecisionRunbookSteps(result)
	if err := writeCandidateDecisionCommitted(filepath.Join(backupRoot, "committed.json"), result); err != nil {
		result.FailedAction = "write committed marker"
		return rollbackCandidateDecision(result, mutated, plan, "write committed marker", err)
	}
	return result, nil
}

func writeCandidateDecisionReceipt(plan candidateDecisionPlan, result CandidateDecisionResult) (CandidateDecisionReceipt, []string, error) {
	proofRoot := filepath.Join(plan.packet.CandidateResult.CandidateRoot, "review-artifacts")
	if err := rejectCandidateDecisionSymlinkPath(plan.packet.CandidateResult.CandidateRoot, proofRoot, true); err != nil {
		return CandidateDecisionReceipt{}, nil, err
	}
	if err := os.MkdirAll(proofRoot, 0o755); err != nil {
		return CandidateDecisionReceipt{}, nil, err
	}
	if err := rejectCandidateDecisionSymlinkPath(plan.packet.CandidateResult.CandidateRoot, proofRoot, false); err != nil {
		return CandidateDecisionReceipt{}, nil, err
	}
	receiptPath := candidateDecisionReceiptPath(plan.packet.CandidateResult.CandidateRoot, plan.result.PacketHash, plan.decisionHash)
	evidence := []string{}
	for _, action := range result.Actions {
		evidence = append(evidence, action.EvidenceRefs...)
	}
	sort.Strings(evidence)
	evidence = slicesCompact(evidence)
	verificationProofPath := ""
	if result.Accepted > 0 {
		verificationProofPath = candidateDecisionVerificationProofPath(plan.packet.CandidateResult.CandidateRoot, plan.result.PacketHash, plan.decisionHash)
	}
	receipt := CandidateDecisionReceipt{
		SchemaVersion:         1,
		Kind:                  "pack-memory-candidate-decision-receipt",
		Pack:                  result.Pack,
		RepoRoot:              result.RepoRoot,
		CaseRoot:              result.CaseRoot,
		PacketPath:            result.PacketPath,
		DecisionPath:          result.DecisionPath,
		PacketHash:            result.PacketHash,
		DecisionHash:          plan.decisionHash,
		BackupRoot:            result.BackupRoot,
		IndexPath:             filepath.Join(plan.packet.CandidateResult.CandidateRoot, "index.json"),
		Accepted:              result.Accepted,
		Rejected:              result.Rejected,
		Superseded:            result.Superseded,
		Actions:               append([]CandidateDecisionAction(nil), result.Actions...),
		DecisionEvidence:      evidence,
		ReceiptPath:           receiptPath,
		VerificationProofPath: verificationProofPath,
		VerificationPending:   result.Accepted > 0,
		Boundary: []string{
			"receipt records the exact reviewed candidate decision and cleanup outcome",
			"accepted reusable content still requires explicit pack/fresh/attached doctor verification",
			"no authority/confirmed writes and no heavy-tool execution",
		},
	}
	nextSteps := []string{"retain the candidate decision receipt as terminal cleanup evidence; no accepted candidate verification is pending"}
	if receipt.VerificationPending {
		receipt.VerificationWorkspaceRoot = candidateDecisionVerificationWorkspace(result.CaseRoot, result.PacketHash, plan.decisionHash)
		freshRoot := filepath.Join(receipt.VerificationWorkspaceRoot, "fresh")
		attachedRoot := filepath.Join(receipt.VerificationWorkspaceRoot, "attached")
		receipt.VerificationProvisionCommand = candidateDecisionVerificationProvisionCommand(result.PacketPath, result.DecisionPath, freshRoot, attachedRoot)
		receipt.VerificationCommand = candidateDecisionVerificationCommand(result.PacketPath, result.DecisionPath, freshRoot, attachedRoot)
		nextSteps = []string{
			"run the receipt verificationProvisionCommand with -WhatIf and inspect the no-overwrite fresh/attached verification case plan",
			"run the returned expected-hash provisioning Apply command, then run the verificationCommand only after provisioning succeeds",
		}
	}
	data, err := json.MarshalIndent(receipt, "", "  ")
	if err != nil {
		return CandidateDecisionReceipt{}, nil, err
	}
	if err := writeDurableExclusiveFile(receiptPath, append(data, '\n')); err != nil {
		return CandidateDecisionReceipt{}, nil, err
	}
	return receipt, nextSteps, nil
}

func candidateDecisionRunbookSteps(result CandidateDecisionResult) []string {
	steps := []string{}
	add := func(step string) {
		step = strings.TrimSpace(step)
		if step == "" || slices.Contains(steps, step) {
			return
		}
		steps = append(steps, step)
	}
	addNextSteps := func() {
		for _, step := range result.NextSteps {
			add("follow candidate decision nextSteps: " + step)
		}
	}

	if result.RecoveryRequired || result.FailedAction != "" || result.RolledBack {
		if result.FailedAction != "" {
			add(fmt.Sprintf("stop candidate decision follow-through; failedAction=%s must be resolved before cleanup proof, verification, or reconsume", result.FailedAction))
		} else {
			add("stop candidate decision follow-through; recoveryRequired must be resolved before cleanup proof, verification, or reconsume")
		}
		if result.RolledBack && !result.RecoveryRequired {
			add("rollback completed; review the recovery envelope, rerun WhatIf, then explicitly retry Apply")
		} else if result.BackupRoot != "" {
			add(fmt.Sprintf("inspect backupRoot %s and its transaction/rollback markers before retrying the reviewed decision", result.BackupRoot))
		} else {
			add("inspect the returned recovery envelope before retrying the reviewed decision")
		}
		if len(result.RecoveryActions) > 0 {
			add(fmt.Sprintf("review %d recoveryActions and restore or verify the listed backups if rollback was incomplete", len(result.RecoveryActions)))
		}
		addNextSteps()
		add("after recovery is complete, rerun candidate decision with -WhatIf before any new -Apply")
		add("do not continue pack-memory downstream closure until this recovery envelope is resolved")
		return steps
	}

	if result.Mode == "candidate-decision-preview" || !result.IsMutation {
		add(fmt.Sprintf("inspect %d planned candidate decision actions and every evidenceRefs entry before applying", len(result.Actions)))
		if result.Accepted > 0 {
			add("note accepted managed-doc candidates will require receipt verification provisioning plus pack/fresh/attached reconsume proof after Apply")
		} else {
			add("note this decision has no accepted candidates; Apply should only clean reviewed rejected/superseded candidates and update the index")
		}
		addNextSteps()
		add("rerun the identical candidate decision command with -Apply only after the reviewed packet/decision hashes still match")
		add("do not continue pack-memory downstream closure until Apply produces a receipt or recovery envelope")
		return steps
	}

	if result.Applied {
		if result.ReceiptPath != "" {
			add(fmt.Sprintf("retain candidate decision receipt %s as terminal cleanup evidence", result.ReceiptPath))
		} else {
			add("retain the candidate decision receipt as terminal cleanup evidence")
		}
		if result.Accepted > 0 {
			if result.Receipt != nil && result.Receipt.VerificationProvisionCommand != "" {
				add("run receipt verificationProvisionCommand with -WhatIf and inspect the no-overwrite fresh/attached verification case plan: " + result.Receipt.VerificationProvisionCommand)
			} else {
				add("run the receipt verificationProvisionCommand with -WhatIf and inspect the no-overwrite fresh/attached verification case plan")
			}
			add("run the returned expected-hash provisioning Apply command only after the fresh/attached verification roots are safe")
			if result.Receipt != nil && result.Receipt.VerificationCommand != "" {
				add("after provisioning succeeds, run verificationCommand to prove pack/fresh/attached doctor and reconsume closure: " + result.Receipt.VerificationCommand)
			} else {
				add("after provisioning succeeds, run verificationCommand to prove pack/fresh/attached doctor and reconsume closure")
			}
			if result.Receipt != nil && result.Receipt.VerificationProofPath != "" {
				add("retain verification proof " + result.Receipt.VerificationProofPath + " and rerun /rekit status or doctor to confirm pack-memory candidate closure")
			} else {
				add("retain the candidate decision verification proof and rerun /rekit status or doctor to confirm pack-memory candidate closure")
			}
		} else {
			add(fmt.Sprintf("confirm %d rejected and %d superseded candidate cleanup actions are reflected in the candidate index", result.Rejected, result.Superseded))
			add("no fresh/attached reconsume proof is required because no accepted candidate changed pack content")
		}
		addNextSteps()
		add("do not continue pack-memory downstream closure until cleanup receipt and any required verification proof are accounted for")
		return steps
	}

	add("review candidate decision result actions, receipt, and nextSteps before continuing pack-memory downstream closure")
	addNextSteps()
	add("do not continue pack-memory downstream closure until candidate decision Apply, recovery, or verification follow-through is resolved")
	return steps
}

func verifyCandidateDecisionCaseContent(packRoot, caseRoot, label string, actions []CandidateDecisionAction) error {
	for _, action := range actions {
		if action.Decision != "accept" || action.Kind != "managed-doc" {
			continue
		}
		rel, err := filepath.Rel(packRoot, action.PackTarget)
		if err != nil || rel == ".." || strings.HasPrefix(rel, ".."+string(filepath.Separator)) {
			return fmt.Errorf("accepted pack target leaves pack root: %s", action.PackTarget)
		}
		caseTarget := filepath.Join(caseRoot, rel)
		if err := rejectCandidateDecisionSymlinkPath(caseRoot, caseTarget, false); err != nil {
			return err
		}
		if fileSHA256(caseTarget) == "" || fileSHA256(caseTarget) != fileSHA256(action.CandidateBackupPath) {
			return fmt.Errorf("%s case has not reconsumed accepted candidate content: %s", label, caseTarget)
		}
	}
	return nil
}

func verifyCandidateDecisionCommittedState(receipt CandidateDecisionReceipt) error {
	for _, action := range receipt.Actions {
		if _, err := os.Lstat(action.CandidatePath); err == nil || !os.IsNotExist(err) {
			return fmt.Errorf("reviewed candidate cleanup changed before proof commit: %s", action.CandidatePath)
		}
		if action.Kind == "managed-doc" && candidateIndexStillContains(receipt.IndexPath, action.CandidatePath) {
			return fmt.Errorf("reviewed candidate index cleanup changed before proof commit: %s", action.CandidatePath)
		}
		if action.Decision == "accept" && (fileSHA256(action.PackTarget) == "" || fileSHA256(action.PackTarget) != fileSHA256(action.CandidateBackupPath)) {
			return fmt.Errorf("accepted pack target changed before proof commit: %s", action.PackTarget)
		}
	}
	return nil
}

func candidateDecisionPacketIndexValid(packet CandidateReviewPacket, canonicalIndexPath string) bool {
	if strings.TrimSpace(packet.CandidateResult.IndexPath) != "" {
		return sameCandidateDecisionPath(packet.CandidateResult.IndexPath, canonicalIndexPath)
	}
	for _, item := range packet.CandidateResult.ReviewPlan.ReviewItems {
		if item.Kind == "managed-doc" && item.ReviewDecision == "pending-review" {
			return false
		}
	}
	return true
}

func candidateDecisionReceiptPath(candidateRoot, packetHash, decisionHash string) string {
	return filepath.Join(candidateRoot, "review-artifacts", shortHash(packetHash+decisionHash)+".candidate-decision-receipt.json")
}

func candidateDecisionVerificationProofPath(candidateRoot, packetHash, decisionHash string) string {
	return filepath.Join(candidateRoot, "review-artifacts", shortHash(packetHash+decisionHash)+".candidate-verification-proof.json")
}

func candidateDecisionVerificationWorkspace(caseRoot, packetHash, decisionHash string) string {
	return filepath.Join(caseRoot, ".rekit", "verifications", "candidate-decisions", shortHash(packetHash+decisionHash))
}

func candidateDecisionVerificationProvisionCommand(packetPath, decisionPath, freshRoot, attachedRoot string) string {
	return fmt.Sprintf("/rekit promote -PacketPath %s -CandidateDecisionPath %s -ProvisionCandidateVerificationCases -FreshCaseRoot %s -AttachedCaseRoot %s -WhatIf -Format json", quoteCandidateDecisionArg(packetPath), quoteCandidateDecisionArg(decisionPath), quoteCandidateDecisionArg(freshRoot), quoteCandidateDecisionArg(attachedRoot))
}

func candidateDecisionVerificationCommand(packetPath, decisionPath, freshRoot, attachedRoot string) string {
	return fmt.Sprintf("/rekit promote -PacketPath %s -CandidateDecisionPath %s -VerifyCandidateDecision -FreshCaseRoot %s -AttachedCaseRoot %s -WhatIf -Format json", quoteCandidateDecisionArg(packetPath), quoteCandidateDecisionArg(decisionPath), quoteCandidateDecisionArg(freshRoot), quoteCandidateDecisionArg(attachedRoot))
}

func slicesCompact(values []string) []string {
	out := values[:0]
	for _, value := range values {
		if value == "" || len(out) > 0 && out[len(out)-1] == value {
			continue
		}
		out = append(out, value)
	}
	return out
}

func quoteCandidateDecisionArg(value string) string {
	return `"` + strings.ReplaceAll(value, `"`, `\"`) + `"`
}

func verifyCandidateDecisionPlan(plan candidateDecisionPlan) error {
	packetBytes, err := os.ReadFile(plan.result.PacketPath)
	if err != nil || sha256Hex(packetBytes) != plan.result.PacketHash {
		return fmt.Errorf("candidate review packet changed after preview validation")
	}
	decisionBytes, err := os.ReadFile(plan.result.DecisionPath)
	if err != nil || sha256Hex(decisionBytes) != plan.decisionHash {
		return fmt.Errorf("candidate decision file changed after preview validation")
	}
	if plan.indexExisted {
		indexBytes, err := os.ReadFile(plan.result.IndexPath)
		if err != nil || sha256Hex(indexBytes) != plan.indexHash {
			return fmt.Errorf("candidate index changed after preview validation")
		}
	} else if _, err := os.Lstat(plan.result.IndexPath); err == nil || !os.IsNotExist(err) {
		return fmt.Errorf("candidate index changed after preview validation")
	}
	for _, item := range plan.items {
		if fileSHA256(item.action.CandidatePath) != strings.ToLower(strings.TrimSpace(item.decision.CandidateHash)) {
			return fmt.Errorf("candidate hash changed before apply for %s", item.action.CandidatePath)
		}
		if item.decision.Decision == "accept" && fileSHA256(item.action.PackTarget) != strings.ToLower(strings.TrimSpace(item.decision.PackTargetHash)) {
			return fmt.Errorf("pack target hash changed before apply for %s", item.action.PackTarget)
		}
		for _, evidence := range item.decision.EvidenceRefs {
			if _, err := validateCandidateDecisionEvidenceRef(plan.result.RepoRoot, plan.result.CaseRoot, evidence); err != nil {
				return err
			}
		}
	}
	return nil
}

func copyCandidateDecisionActionBackups(actions []CandidateDecisionAction, action CandidateDecisionAction) {
	for idx := range actions {
		if sameCandidateDecisionPath(actions[idx].CandidatePath, action.CandidatePath) {
			actions[idx].CandidateBackupPath = action.CandidateBackupPath
			actions[idx].TargetBackupPath = action.TargetBackupPath
		}
	}
}

func markCandidateDecisionRemoved(items []candidateDecisionPlanItem, candidatePath string) []candidateDecisionPlanItem {
	for idx := range items {
		if sameCandidateDecisionPath(items[idx].action.CandidatePath, candidatePath) {
			items[idx].candidateRemoved = true
		}
	}
	return items
}

func rollbackCandidateDecision(result CandidateDecisionResult, mutated []candidateDecisionPlanItem, plan candidateDecisionPlan, action string, cause error) (CandidateDecisionResult, error) {
	rollbackErrors := []string{}
	receiptPath := candidateDecisionReceiptPath(plan.packet.CandidateResult.CandidateRoot, plan.result.PacketHash, plan.decisionHash)
	if err := os.Remove(receiptPath); err != nil && !os.IsNotExist(err) {
		rollbackErrors = append(rollbackErrors, err.Error())
	}
	for idx := len(mutated) - 1; idx >= 0; idx-- {
		item := mutated[idx]
		if item.decision.Decision == "accept" {
			if err := copyFile(item.action.TargetBackupPath, item.action.PackTarget); err != nil {
				rollbackErrors = append(rollbackErrors, err.Error())
			}
		}
	}
	if plan.indexExisted {
		if err := copyFile(filepath.Join(result.BackupRoot, "candidate-index.json"), plan.result.IndexPath); err != nil {
			rollbackErrors = append(rollbackErrors, err.Error())
		}
	} else if err := os.Remove(plan.result.IndexPath); err != nil && !os.IsNotExist(err) {
		rollbackErrors = append(rollbackErrors, err.Error())
	}
	for _, item := range mutated {
		if !item.candidateRemoved {
			continue
		}
		if restoreErr := copyFile(item.action.CandidateBackupPath, item.action.CandidatePath); restoreErr != nil {
			rollbackErrors = append(rollbackErrors, restoreErr.Error())
		}
	}
	result.FailedAction = action
	result.RolledBack = len(rollbackErrors) == 0
	result.RecoveryRequired = len(rollbackErrors) != 0
	result.NextSteps = []string{"inspect backupRoot and recoveryActions before retrying the reviewed decision"}
	result.DecisionRunbookSteps = candidateDecisionRunbookSteps(result)
	if len(rollbackErrors) != 0 {
		err := fmt.Errorf("candidate decision apply %s: %w; rollback errors: %s", action, cause, strings.Join(rollbackErrors, "; "))
		return result, &CandidateDecisionApplyError{Result: result, Err: err}
	}
	if markerErr := writeCandidateDecisionCommitted(filepath.Join(result.BackupRoot, "rolled-back.json"), result); markerErr != nil && !os.IsExist(markerErr) {
		result.RecoveryRequired = true
		result.DecisionRunbookSteps = candidateDecisionRunbookSteps(result)
		err := fmt.Errorf("candidate decision apply %s: %w; rollback completed but marker write failed: %v", action, cause, markerErr)
		return result, &CandidateDecisionApplyError{Result: result, Err: err}
	}
	err := fmt.Errorf("candidate decision apply %s: %w; all mutations rolled back", action, cause)
	return result, &CandidateDecisionApplyError{Result: result, Err: err}
}

func recoverCandidateDecisionTransaction(repoRoot, caseRoot, pack string, opt CandidateDecisionOptions, m *manifest.Manifest) (CandidateDecisionResult, bool, error) {
	packetPath, packetBytes, err := readStrictCandidateDecisionFile(opt.PacketPath, "candidate review packet")
	if err != nil {
		return CandidateDecisionResult{}, false, nil
	}
	decisionPath, decisionBytes, err := readStrictCandidateDecisionFile(opt.DecisionPath, "candidate decision")
	if err != nil {
		return CandidateDecisionResult{}, false, nil
	}
	var packet CandidateReviewPacket
	if err := decodeStrictJSON(packetBytes, &packet); err != nil {
		return CandidateDecisionResult{}, false, nil
	}
	if packet.SchemaVersion != 1 || packet.Kind != "pack-memory-candidate-review" || packet.Command != "promote" || packet.CandidateResult.Pack != pack {
		return CandidateDecisionResult{}, false, nil
	}
	inst, err := instance.AssertAttached(caseRoot, repoRoot, pack)
	if err != nil || !sameCandidateDecisionPath(packet.CandidateResult.RepoRoot, repoRoot) || !sameCandidateDecisionPath(packet.CandidateResult.CaseRoot, inst.CaseRoot) {
		return CandidateDecisionResult{}, false, nil
	}
	canonicalCandidateRoot := filepath.Join(m.PackRoot, "promote-candidates")
	canonicalToolingRoot := filepath.Join(m.PackRoot, "tooling", "candidates")
	canonicalIndexPath := filepath.Join(canonicalCandidateRoot, "index.json")
	if !sameCandidateDecisionPath(packet.CandidateResult.CandidateRoot, canonicalCandidateRoot) || !sameCandidateDecisionPath(packet.CandidateResult.ToolingRoot, canonicalToolingRoot) || !candidateDecisionPacketIndexValid(packet, canonicalIndexPath) {
		return CandidateDecisionResult{}, true, fmt.Errorf("candidate review packet roots do not match canonical pack candidate roots")
	}
	backupParent := filepath.Join(canonicalCandidateRoot, ".decision-backup")
	if err := rejectCandidateDecisionSymlinkPath(canonicalCandidateRoot, backupParent, true); err != nil {
		return CandidateDecisionResult{}, true, err
	}
	entries, err := os.ReadDir(backupParent)
	if os.IsNotExist(err) {
		return CandidateDecisionResult{}, false, nil
	}
	if err != nil {
		return CandidateDecisionResult{}, true, err
	}
	var transaction candidateDecisionTransaction
	transactionRoot := ""
	for _, entry := range entries {
		if !entry.IsDir() {
			continue
		}
		root := filepath.Join(backupParent, entry.Name())
		if err := rejectCandidateDecisionSymlinkPath(packet.CandidateResult.CandidateRoot, root, false); err != nil {
			return CandidateDecisionResult{}, true, err
		}
		journalPath := filepath.Join(root, "transaction.json")
		journalBytes, readErr := os.ReadFile(journalPath)
		if readErr != nil {
			continue
		}
		var candidate candidateDecisionTransaction
		if err := decodeStrictJSON(journalBytes, &candidate); err != nil {
			return CandidateDecisionResult{}, true, fmt.Errorf("decode candidate decision transaction: %w", err)
		}
		if _, err := os.Lstat(filepath.Join(root, "committed.json")); err == nil {
			continue
		}
		if _, err := os.Lstat(filepath.Join(root, "rolled-back.json")); err == nil {
			continue
		}
		if candidate.PacketHash != sha256Hex(packetBytes) || candidate.DecisionHash != sha256Hex(decisionBytes) {
			result := candidate.Result
			result.FailedAction = "unfinished candidate decision transaction"
			result.RecoveryRequired = true
			result.DecisionRunbookSteps = candidateDecisionRunbookSteps(result)
			err := fmt.Errorf("unfinished candidate decision transaction %s must be recovered with its original packet and decision before starting another decision", root)
			return result, true, &CandidateDecisionApplyError{Result: result, Err: err}
		}
		transaction = candidate
		transactionRoot = root
		break
	}
	if transactionRoot == "" {
		return CandidateDecisionResult{}, false, nil
	}
	if transaction.SchemaVersion != 1 || transaction.Kind != "pack-memory-candidate-decision-transaction" || !sameCandidateDecisionPath(transaction.Result.RepoRoot, repoRoot) || !sameCandidateDecisionPath(transaction.Result.CaseRoot, inst.CaseRoot) || transaction.Result.Pack != pack || !sameCandidateDecisionPath(transaction.Result.IndexPath, canonicalIndexPath) || !sameCandidateDecisionPath(transaction.Result.BackupRoot, transactionRoot) {
		return CandidateDecisionResult{}, true, fmt.Errorf("candidate decision transaction binding mismatch")
	}
	managedTargets := map[string]bool{}
	for _, rel := range m.ManagedFiles {
		target, err := m.SourcePath(rel)
		if err != nil {
			return CandidateDecisionResult{}, true, err
		}
		managedTargets[filepath.Clean(target)] = true
	}
	for _, action := range transaction.Actions {
		candidateRoot := canonicalCandidateRoot
		if action.Kind == "tooling-candidate-source" {
			candidateRoot = canonicalToolingRoot
		}
		if err := assertInsideRoot(candidateRoot, action.CandidatePath); err != nil {
			return CandidateDecisionResult{}, true, err
		}
		if err := rejectCandidateDecisionSymlinkPath(candidateRoot, action.CandidatePath, true); err != nil {
			return CandidateDecisionResult{}, true, err
		}
		if state, err := refsf.ClassifyNonEmptyRegularFile(action.CandidatePath); err != nil {
			return CandidateDecisionResult{}, true, err
		} else if state != refsf.RegularFileMissing && state != refsf.RegularFileReady {
			return CandidateDecisionResult{}, true, fmt.Errorf("candidate recovery destination must be missing or a non-empty regular file: %s", action.CandidatePath)
		}
		if err := assertInsideRoot(transactionRoot, action.CandidateBackupPath); err != nil {
			return CandidateDecisionResult{}, true, err
		}
		if err := rejectCandidateDecisionSymlinkPath(transactionRoot, action.CandidateBackupPath, false); err != nil {
			return CandidateDecisionResult{}, true, err
		}
		if action.TargetBackupPath != "" {
			if !managedTargets[filepath.Clean(action.PackTarget)] {
				return CandidateDecisionResult{}, true, fmt.Errorf("candidate decision transaction target is not manifest-managed: %s", action.PackTarget)
			}
			if err := assertInsideRoot(transactionRoot, action.TargetBackupPath); err != nil {
				return CandidateDecisionResult{}, true, err
			}
			if err := rejectCandidateDecisionSymlinkPath(transactionRoot, action.TargetBackupPath, false); err != nil {
				return CandidateDecisionResult{}, true, err
			}
			if err := rejectCandidateDecisionSymlinkPath(m.PackRoot, action.PackTarget, false); err != nil {
				return CandidateDecisionResult{}, true, err
			}
		}
	}
	if transaction.IndexExisted {
		if err := assertInsideRoot(transactionRoot, transaction.IndexBackupPath); err != nil {
			return CandidateDecisionResult{}, true, err
		}
		if err := rejectCandidateDecisionSymlinkPath(transactionRoot, transaction.IndexBackupPath, false); err != nil {
			return CandidateDecisionResult{}, true, err
		}
	}
	result := transaction.Result
	result.PacketPath = packetPath
	result.DecisionPath = decisionPath
	result.FailedAction = "recover interrupted transaction"
	result.RecoveryRequired = true
	rollbackErrors := []string{}
	receiptPath := candidateDecisionReceiptPath(canonicalCandidateRoot, transaction.PacketHash, transaction.DecisionHash)
	if err := os.Remove(receiptPath); err != nil && !os.IsNotExist(err) {
		rollbackErrors = append(rollbackErrors, err.Error())
	}
	for idx := len(transaction.Actions) - 1; idx >= 0; idx-- {
		action := transaction.Actions[idx]
		if action.TargetBackupPath != "" {
			if err := copyFile(action.TargetBackupPath, action.PackTarget); err != nil {
				rollbackErrors = append(rollbackErrors, err.Error())
			}
		}
		if action.CandidateBackupPath != "" {
			backupHash := fileSHA256(action.CandidateBackupPath)
			state, stateErr := refsf.ClassifyNonEmptyRegularFile(action.CandidatePath)
			switch {
			case stateErr != nil:
				rollbackErrors = append(rollbackErrors, stateErr.Error())
			case state == refsf.RegularFileMissing:
				if err := copyFileExclusive(action.CandidateBackupPath, action.CandidatePath); err != nil {
					rollbackErrors = append(rollbackErrors, err.Error())
				}
			case state == refsf.RegularFileReady && fileSHA256(action.CandidatePath) == backupHash:
			default:
				rollbackErrors = append(rollbackErrors, fmt.Sprintf("candidate recovery destination changed: %s", action.CandidatePath))
			}
		}
	}
	if transaction.IndexExisted {
		if err := copyFile(transaction.IndexBackupPath, result.IndexPath); err != nil {
			rollbackErrors = append(rollbackErrors, err.Error())
		}
	} else if err := os.Remove(result.IndexPath); err != nil && !os.IsNotExist(err) {
		rollbackErrors = append(rollbackErrors, err.Error())
	}
	result.RolledBack = len(rollbackErrors) == 0
	result.RecoveryRequired = len(rollbackErrors) != 0
	result.NextSteps = []string{"review the recovered transaction, rerun WhatIf, then explicitly retry Apply"}
	result.DecisionRunbookSteps = candidateDecisionRunbookSteps(result)
	if len(rollbackErrors) != 0 {
		err := fmt.Errorf("candidate decision interrupted transaction recovery failed: %s", strings.Join(rollbackErrors, "; "))
		return result, true, &CandidateDecisionApplyError{Result: result, Err: err}
	}
	if err := writeCandidateDecisionCommitted(filepath.Join(transactionRoot, "rolled-back.json"), result); err != nil {
		result.RecoveryRequired = true
		result.DecisionRunbookSteps = candidateDecisionRunbookSteps(result)
		err := fmt.Errorf("candidate decision interrupted transaction rolled back but marker write failed: %w", err)
		return result, true, &CandidateDecisionApplyError{Result: result, Err: err}
	}
	err = fmt.Errorf("candidate decision interrupted transaction detected; all mutations rolled back")
	return result, true, &CandidateDecisionApplyError{Result: result, Err: err}
}

func acquireCandidateDecisionLock(candidateRoot string) (func(), error) {
	if err := rejectCandidateDecisionSymlinkPath(candidateRoot, candidateRoot, false); err != nil {
		return nil, err
	}
	lockPath := filepath.Join(candidateRoot, ".decision.lock")
	pendingPath := lockPath + ".pending-" + strconv.Itoa(os.Getpid())
	if err := writeDurableExclusiveFile(pendingPath, fmt.Appendf(nil, "pid=%d\n", os.Getpid())); err != nil {
		return nil, err
	}
	defer os.Remove(pendingPath)
	for range 2 {
		if err := os.Link(pendingPath, lockPath); err == nil {
			return func() { _ = os.Remove(lockPath) }, nil
		} else if !os.IsExist(err) {
			return nil, err
		}
		ownerPID, readErr := candidateDecisionLockPID(lockPath)
		if readErr == nil && candidateDecisionProcessAlive(ownerPID) {
			return nil, fmt.Errorf("another candidate decision transaction is active: %s", lockPath)
		}
		stalePath := lockPath + ".stale-" + strconv.Itoa(os.Getpid())
		if err := os.Rename(lockPath, stalePath); err != nil {
			return nil, fmt.Errorf("another candidate decision transaction is active: %s", lockPath)
		}
		_ = os.Remove(stalePath)
	}
	return nil, fmt.Errorf("another candidate decision transaction is active: %s", lockPath)
}

func candidateDecisionLockPID(path string) (int, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return 0, err
	}
	value := strings.TrimSpace(strings.TrimPrefix(string(data), "pid="))
	pid, err := strconv.Atoi(value)
	if err != nil || pid <= 0 {
		return 0, fmt.Errorf("invalid candidate decision lock owner")
	}
	return pid, nil
}

func writeCandidateDecisionTransaction(path string, transaction candidateDecisionTransaction) error {
	data, err := json.MarshalIndent(transaction, "", "  ")
	if err != nil {
		return err
	}
	return writeDurableExclusiveFile(path, append(data, '\n'))
}

func writeCandidateDecisionCommitted(path string, result CandidateDecisionResult) error {
	data, err := json.MarshalIndent(result, "", "  ")
	if err != nil {
		return err
	}
	return writeDurableExclusiveFile(path, append(data, '\n'))
}

func writeDurableExclusiveFile(path string, data []byte) error {
	file, err := os.OpenFile(path, os.O_WRONLY|os.O_CREATE|os.O_EXCL, 0o644)
	if err != nil {
		return err
	}
	complete := false
	defer func() {
		if !complete {
			_ = os.Remove(path)
		}
	}()
	written, err := file.Write(data)
	if err == nil && written != len(data) {
		err = io.ErrShortWrite
	}
	if err == nil {
		err = file.Sync()
	}
	closeErr := file.Close()
	if err != nil {
		return err
	}
	if closeErr != nil {
		return closeErr
	}
	if err := syncCandidateDecisionDirectory(filepath.Dir(path)); err != nil {
		return err
	}
	complete = true
	return nil
}

func writeDurableFileIdempotent(path string, data []byte) (bool, error) {
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
		return false, fmt.Errorf("existing durable proof must be a non-empty regular file: %s", path)
	}
	existing, readErr := os.ReadFile(path)
	if readErr != nil {
		return false, readErr
	}
	if !bytes.Equal(existing, data) {
		return false, fmt.Errorf("existing durable proof does not match replay: %s", path)
	}
	return true, nil
}

func readStrictCandidateDecisionFile(path, label string) (string, []byte, error) {
	full, err := filepath.Abs(strings.TrimSpace(path))
	if err != nil || strings.TrimSpace(path) == "" {
		return "", nil, fmt.Errorf("%s path is required", label)
	}
	state, err := refsf.ClassifyNonEmptyRegularFile(full)
	if err != nil {
		return "", nil, err
	}
	if state != refsf.RegularFileReady {
		return "", nil, fmt.Errorf("%s %s must be a non-empty regular file, got %s", label, full, state)
	}
	st, err := os.Stat(full)
	if err != nil {
		return "", nil, err
	}
	if st.Size() > maxCandidateDecisionBytes {
		return "", nil, fmt.Errorf("%s exceeds %d bytes", label, maxCandidateDecisionBytes)
	}
	data, err := os.ReadFile(full)
	return full, data, err
}

func decodeStrictJSON(data []byte, value any) error {
	decoder := json.NewDecoder(bytes.NewReader(data))
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(value); err != nil {
		return err
	}
	var trailing any
	if err := decoder.Decode(&trailing); err != io.EOF {
		if err == nil {
			return fmt.Errorf("unexpected trailing JSON")
		}
		return err
	}
	return nil
}

func sha256Hex(data []byte) string {
	sum := sha256.Sum256(data)
	return hex.EncodeToString(sum[:])
}

func fileSHA256(path string) string {
	data, err := os.ReadFile(path)
	if err != nil {
		return ""
	}
	return sha256Hex(data)
}

func rejectCandidateDecisionSymlinkPath(root, path string, allowMissingLeaf bool) error {
	rootFull, err := filepath.Abs(root)
	if err != nil {
		return err
	}
	rootState, err := os.Lstat(rootFull)
	if err != nil {
		return err
	}
	if rootState.Mode()&os.ModeSymlink != 0 {
		return fmt.Errorf("candidate decision root must not be a symlink: %s", rootFull)
	}
	pathFull, err := filepath.Abs(path)
	if err != nil {
		return err
	}
	rel, err := filepath.Rel(rootFull, pathFull)
	if err != nil || rel == ".." || strings.HasPrefix(rel, ".."+string(filepath.Separator)) || filepath.IsAbs(rel) {
		return fmt.Errorf("path escapes root: %s", path)
	}
	current := rootFull
	for part := range strings.SplitSeq(rel, string(filepath.Separator)) {
		if part == "" || part == "." {
			continue
		}
		current = filepath.Join(current, part)
		st, err := os.Lstat(current)
		if os.IsNotExist(err) && allowMissingLeaf {
			return nil
		}
		if err != nil {
			return err
		}
		if st.Mode()&os.ModeSymlink != 0 {
			return fmt.Errorf("candidate decision path must not traverse symlink: %s", current)
		}
	}
	return nil
}

func validateCandidateDecisionEvidenceRef(repoRoot, caseRoot string, evidence CandidateDecisionEvidence) (string, error) {
	value := strings.TrimSpace(evidence.Path)
	if value == "" || strings.TrimSpace(evidence.SHA256) == "" {
		return "", fmt.Errorf("evidenceRefs require path and sha256")
	}
	full := value
	if !filepath.IsAbs(full) {
		full = filepath.Join(caseRoot, filepath.FromSlash(value))
	}
	full, err := filepath.Abs(full)
	if err != nil {
		return "", err
	}
	insideRepo := assertInsideRoot(repoRoot, full) == nil
	insideCase := assertInsideRoot(caseRoot, full) == nil
	if !insideRepo && !insideCase {
		return "", fmt.Errorf("evidenceRef %q must stay under repoRoot or caseRoot", value)
	}
	evidenceRoot := caseRoot
	if insideRepo {
		evidenceRoot = repoRoot
	}
	if err := rejectCandidateDecisionSymlinkPath(evidenceRoot, full, false); err != nil {
		return "", err
	}
	state, err := refsf.ClassifyNonEmptyRegularFile(full)
	if err != nil {
		return "", err
	}
	if state != refsf.RegularFileReady {
		return "", fmt.Errorf("evidenceRef %q must be a non-empty regular file, got %s", value, state)
	}
	if !strings.EqualFold(strings.TrimSpace(evidence.SHA256), fileSHA256(full)) {
		return "", fmt.Errorf("evidenceRef hash mismatch for %q", value)
	}
	return full, nil
}

func sameCandidateDecisionPath(left, right string) bool {
	leftFull, leftErr := filepath.Abs(left)
	rightFull, rightErr := filepath.Abs(right)
	if leftErr != nil || rightErr != nil {
		return false
	}
	leftFull = filepath.Clean(leftFull)
	rightFull = filepath.Clean(rightFull)
	if leftFull == rightFull {
		return true
	}
	leftInfo, leftErr := os.Stat(leftFull)
	rightInfo, rightErr := os.Stat(rightFull)
	return leftErr == nil && rightErr == nil && os.SameFile(leftInfo, rightInfo)
}

func shortHash(value string) string {
	return sha256Hex([]byte(value))[:16]
}

func copyFile(source, target string) error {
	data, err := os.ReadFile(source)
	if err != nil {
		return err
	}
	if err := os.MkdirAll(filepath.Dir(target), 0o755); err != nil {
		return err
	}
	return os.WriteFile(target, data, 0o644)
}

func copyFileExclusive(source, target string) error {
	data, err := os.ReadFile(source)
	if err != nil {
		return err
	}
	return writeDurableExclusiveFile(target, data)
}

func writeFileAtomic(path string, data []byte, mode os.FileMode) error {
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return err
	}
	temp, err := os.CreateTemp(filepath.Dir(path), ".candidate-decision-*")
	if err != nil {
		return err
	}
	tempPath := temp.Name()
	defer os.Remove(tempPath)
	if err := temp.Chmod(mode); err != nil {
		temp.Close()
		return err
	}
	if _, err := temp.Write(data); err != nil {
		temp.Close()
		return err
	}
	if err := temp.Sync(); err != nil {
		temp.Close()
		return err
	}
	if err := temp.Close(); err != nil {
		return err
	}
	return os.Rename(tempPath, path)
}

func writeCandidateIndexAtomic(path string, entries []candidateIndexEntry) error {
	if len(entries) == 0 {
		if err := os.Remove(path); err != nil && !os.IsNotExist(err) {
			return err
		}
		return nil
	}
	data, err := json.MarshalIndent(entries, "", "  ")
	if err != nil {
		return err
	}
	return writeFileAtomic(path, append(data, '\n'), 0o644)
}

func readCandidateIndex(path string) ([]candidateIndexEntry, error) {
	if strings.TrimSpace(path) == "" {
		return nil, nil
	}
	data, err := os.ReadFile(path)
	if os.IsNotExist(err) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	var entries []candidateIndexEntry
	if err := json.Unmarshal(data, &entries); err != nil {
		return nil, fmt.Errorf("decode candidate index: %w", err)
	}
	return entries, nil
}

func candidateWriteMatches(writes []CandidateWrite, path, kind, candidate string) bool {
	for _, write := range writes {
		if filepath.ToSlash(write.Path) == filepath.ToSlash(path) && write.Kind == kind && write.Action == "create-candidate" && sameCandidateDecisionPath(write.TargetPath, candidate) {
			return true
		}
	}
	return false
}

func candidateIndexContains(entries []candidateIndexEntry, path, candidate string) bool {
	for _, entry := range entries {
		if filepath.ToSlash(entry.Path) == filepath.ToSlash(path) && sameCandidateDecisionPath(entry.Candidate, candidate) {
			return true
		}
	}
	return false
}

func candidateIndexStillContains(indexPath, candidate string) bool {
	entries, err := readCandidateIndex(indexPath)
	if err != nil {
		return true
	}
	candidateAbs, err := filepath.Abs(candidate)
	if err != nil {
		return true
	}
	for _, entry := range entries {
		entryCandidate := entry.Candidate
		if !filepath.IsAbs(entryCandidate) {
			entryCandidate = filepath.Join(filepath.Dir(indexPath), entryCandidate)
		}
		entryAbs, err := filepath.Abs(entryCandidate)
		if err != nil {
			return true
		}
		if filepath.Clean(entryAbs) == filepath.Clean(candidateAbs) || sameCandidateDecisionPath(entryAbs, candidateAbs) {
			return true
		}
	}
	return false
}

func removeCandidateIndexEntry(entries []candidateIndexEntry, candidate string) []candidateIndexEntry {
	out := entries[:0]
	for _, entry := range entries {
		if !sameCandidateDecisionPath(entry.Candidate, candidate) {
			out = append(out, entry)
		}
	}
	return out
}
