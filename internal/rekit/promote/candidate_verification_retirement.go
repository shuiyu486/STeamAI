package promote

import (
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"time"

	"github.com/shuiyu486/re-context-kits/internal/rekit/mission"
	syncpkg "github.com/shuiyu486/re-context-kits/internal/rekit/sync"
)

// CandidateVerificationRetirementOptions controls the domain-only retirement
// flow. Verification roots are always derived from decision authority.
type CandidateVerificationRetirementOptions struct {
	PacketPath               string
	DecisionPath             string
	ExpectedRetirementSHA256 string
	WhatIf                   bool
}

type CandidateVerificationRetirementRoot struct {
	Role     string   `json:"role"`
	CaseRoot string   `json:"caseRoot"`
	Deletes  []string `json:"deletes"`
}

type CandidateVerificationRetirementResult struct {
	SchemaVersion               int                                   `json:"schemaVersion"`
	Kind                        string                                `json:"kind"`
	Mode                        string                                `json:"mode"`
	RepoRoot                    string                                `json:"repoRoot"`
	SourceCaseRoot              string                                `json:"sourceCaseRoot"`
	Pack                        string                                `json:"pack"`
	PacketPath                  string                                `json:"packetPath"`
	PacketSHA256                string                                `json:"packetSha256"`
	DecisionPath                string                                `json:"decisionPath"`
	DecisionSHA256              string                                `json:"decisionSha256"`
	DecisionReceiptPath         string                                `json:"decisionReceiptPath"`
	DecisionReceiptSHA256       string                                `json:"decisionReceiptSha256"`
	VerificationProofPath       string                                `json:"verificationProofPath"`
	VerificationProofSHA256     string                                `json:"verificationProofSha256"`
	ProvisionIntentPath         string                                `json:"provisionIntentPath"`
	ProvisionIntentSHA256       string                                `json:"provisionIntentSha256"`
	ProvisionReceiptPath        string                                `json:"provisionReceiptPath"`
	ProvisionReceiptSHA256      string                                `json:"provisionReceiptSha256"`
	WorkspaceRoot               string                                `json:"workspaceRoot"`
	RetirementIntentPath        string                                `json:"retirementIntentPath"`
	RetirementReceiptPath       string                                `json:"retirementReceiptPath"`
	RetirementSHA256            string                                `json:"retirementSha256"`
	IsMutation                  bool                                  `json:"isMutation"`
	Applied                     bool                                  `json:"applied"`
	Replay                      bool                                  `json:"replay"`
	Roots                       []CandidateVerificationRetirementRoot `json:"roots"`
	ProvisionArtifactsToDelete  []string                              `json:"provisionArtifactsToDelete"`
	EmptyAncestorsToRemove      []string                              `json:"emptyAncestorsToRemove"`
	ApplyCommand                string                                `json:"applyCommand,omitempty"`
	MissionCommanderAction      *mission.MissionCommanderAction       `json:"missionCommanderAction,omitempty"`
	MissionCommanderActionQueue *mission.MissionCommanderActionQueue  `json:"missionCommanderActionQueue,omitempty"`
	NextSteps                   []string                              `json:"nextSteps"`
	Boundary                    []string                              `json:"boundary"`
	RetirementPlans             []syncpkg.ExclusiveInitRetirementPlan `json:"retirementPlans"`
}

type candidateVerificationRetirementArtifact struct {
	SchemaVersion              int                                   `json:"schemaVersion"`
	Kind                       string                                `json:"kind"`
	RepoRoot                   string                                `json:"repoRoot"`
	SourceCaseRoot             string                                `json:"sourceCaseRoot"`
	Pack                       string                                `json:"pack"`
	PacketPath                 string                                `json:"packetPath"`
	PacketSHA256               string                                `json:"packetSha256"`
	DecisionPath               string                                `json:"decisionPath"`
	DecisionSHA256             string                                `json:"decisionSha256"`
	DecisionReceiptPath        string                                `json:"decisionReceiptPath"`
	DecisionReceiptSHA256      string                                `json:"decisionReceiptSha256"`
	VerificationProofPath      string                                `json:"verificationProofPath"`
	VerificationProofSHA256    string                                `json:"verificationProofSha256"`
	ProvisionIntentPath        string                                `json:"provisionIntentPath"`
	ProvisionIntentSHA256      string                                `json:"provisionIntentSha256"`
	ProvisionReceiptPath       string                                `json:"provisionReceiptPath"`
	ProvisionReceiptSHA256     string                                `json:"provisionReceiptSha256"`
	WorkspaceRoot              string                                `json:"workspaceRoot"`
	RetirementIntentPath       string                                `json:"retirementIntentPath"`
	RetirementReceiptPath      string                                `json:"retirementReceiptPath"`
	RetirementSHA256           string                                `json:"retirementSha256"`
	Roots                      []CandidateVerificationRetirementRoot `json:"roots"`
	ProvisionArtifactsToDelete []string                              `json:"provisionArtifactsToDelete"`
	EmptyAncestorsToRemove     []string                              `json:"emptyAncestorsToRemove"`
	Boundary                   []string                              `json:"boundary"`
	RetirementPlans            []syncpkg.ExclusiveInitRetirementPlan `json:"retirementPlans"`
}

type preparedCandidateVerificationRetirement struct {
	result          CandidateVerificationRetirementResult
	artifact        candidateVerificationRetirementArtifact
	intentBytes     []byte
	receiptBytes    []byte
	retirementPlans []syncpkg.ExclusiveInitRetirementPlan
}

type pinnedCandidateVerificationRetirementWorkspace struct {
	parent     *os.Root
	root       *os.Root
	parentInfo os.FileInfo
	rootInfo   os.FileInfo
	name       string
}

var candidateVerificationRetirementStageHook func(string) error

// RetireCandidateVerificationWorkspace removes a completed, exact canonical
// verification workspace through a WhatIf -> expected SHA-256 domain flow.
func RetireCandidateVerificationWorkspace(repoRoot, sourceCaseRoot, pack string, opt CandidateVerificationRetirementOptions) (CandidateVerificationRetirementResult, error) {
	if opt.WhatIf {
		if retired, ok, err := loadCompletedCandidateVerificationRetirement(repoRoot, sourceCaseRoot, pack, opt); err != nil {
			return CandidateVerificationRetirementResult{}, err
		} else if ok {
			return finalizeCandidateVerificationRetirementResult(retired), nil
		}
	} else {
		expected := strings.TrimSpace(opt.ExpectedRetirementSHA256)
		decoded, decodeErr := hex.DecodeString(expected)
		if decodeErr != nil || len(decoded) != sha256.Size {
			return CandidateVerificationRetirementResult{}, fmt.Errorf("candidate verification retirement Apply requires a valid ExpectedRetirementSHA256 from WhatIf")
		}
		if retired, ok, err := loadCompletedCandidateVerificationRetirement(repoRoot, sourceCaseRoot, pack, opt); err != nil {
			return CandidateVerificationRetirementResult{}, err
		} else if ok {
			if !strings.EqualFold(expected, retired.RetirementSHA256) {
				return CandidateVerificationRetirementResult{}, fmt.Errorf("candidate verification retirement changed after preview")
			}
			return finalizeCandidateVerificationRetirementResult(retired), nil
		}
	}
	prepared, err := prepareCandidateVerificationRetirement(repoRoot, sourceCaseRoot, pack, opt)
	if err != nil {
		if resumed, ok, resumeErr := prepareCandidateVerificationRetirementFromIntent(repoRoot, sourceCaseRoot, pack, opt); resumeErr != nil {
			return CandidateVerificationRetirementResult{}, resumeErr
		} else if ok {
			prepared = resumed
		} else {
			return CandidateVerificationRetirementResult{}, err
		}
	}
	if opt.WhatIf {
		if _, ok, err := readCandidateVerificationRetirementArtifact(prepared.result.RetirementIntentPath); err != nil {
			return CandidateVerificationRetirementResult{}, err
		} else if !ok {
			if _, err := syncpkg.InspectExclusiveInitRetirementBatch(syncpkg.ExclusiveInitRetirementFirst, prepared.retirementPlans...); err != nil {
				return CandidateVerificationRetirementResult{}, err
			}
			if err := inspectCandidateVerificationWorkspaceArtifacts(prepared, false); err != nil {
				return CandidateVerificationRetirementResult{}, err
			}
		}
		prepared.result.Mode = "previewed"
		prepared.result.ApplyCommand = candidateVerificationRetirementCommand(prepared.result)
		prepared.result.NextSteps = []string{"inspect the exact canonical workspace deletion plan, then run the returned expected-hash Apply command"}
		return finalizeCandidateVerificationRetirementResult(prepared.result), nil
	}
	expected := strings.TrimSpace(opt.ExpectedRetirementSHA256)
	decoded, decodeErr := hex.DecodeString(expected)
	if decodeErr != nil || len(decoded) != sha256.Size {
		return CandidateVerificationRetirementResult{}, fmt.Errorf("candidate verification retirement Apply requires a valid ExpectedRetirementSHA256 from WhatIf")
	}
	unlock, err := acquireCandidateDecisionLock(filepath.Join(repoRoot, "packs", pack, "promote-candidates"))
	if err != nil {
		return CandidateVerificationRetirementResult{}, err
	}
	defer unlock()
	prepared, err = prepareCandidateVerificationRetirement(repoRoot, sourceCaseRoot, pack, opt)
	if err != nil {
		if resumed, ok, resumeErr := prepareCandidateVerificationRetirementFromIntent(repoRoot, sourceCaseRoot, pack, opt); resumeErr != nil {
			return CandidateVerificationRetirementResult{}, resumeErr
		} else if ok {
			prepared = resumed
		} else {
			return CandidateVerificationRetirementResult{}, err
		}
	}
	if !strings.EqualFold(expected, prepared.result.RetirementSHA256) {
		return CandidateVerificationRetirementResult{}, fmt.Errorf("candidate verification retirement changed after preview")
	}
	if receipt, ok, err := readCandidateVerificationRetirementArtifact(prepared.result.RetirementReceiptPath); err != nil {
		return CandidateVerificationRetirementResult{}, err
	} else if ok {
		if !candidateVerificationRetirementArtifactMatches(receipt, prepared.artifact, "-receipt") {
			return CandidateVerificationRetirementResult{}, fmt.Errorf("candidate verification retirement receipt has different bindings")
		}
		if err := assertRetiredCandidateVerificationWorkspaceAbsent(prepared.result); err != nil {
			return CandidateVerificationRetirementResult{}, err
		}
		prepared.result.Mode = "already-retired"
		prepared.result.IsMutation = true
		prepared.result.Applied = true
		prepared.result.Replay = true
		prepared.result.NextSteps = []string{"retain the repo-local retirement intent and receipt as final evidence"}
		return finalizeCandidateVerificationRetirementResult(prepared.result), nil
	}
	intent, intentCurrent, err := readCandidateVerificationRetirementArtifact(prepared.result.RetirementIntentPath)
	if err != nil {
		return CandidateVerificationRetirementResult{}, err
	}
	if intentCurrent {
		if !candidateVerificationRetirementArtifactMatches(intent, prepared.artifact, "-intent") {
			return CandidateVerificationRetirementResult{}, fmt.Errorf("candidate verification retirement intent has different bindings")
		}
		if _, err := syncpkg.InspectExclusiveInitRetirementBatch(syncpkg.ExclusiveInitRetirementResume, prepared.retirementPlans...); err != nil {
			return CandidateVerificationRetirementResult{}, err
		}
		if err := inspectCandidateVerificationWorkspaceArtifacts(prepared, true); err != nil {
			return CandidateVerificationRetirementResult{}, err
		}
	} else {
		if _, err := syncpkg.InspectExclusiveInitRetirementBatch(syncpkg.ExclusiveInitRetirementFirst, prepared.retirementPlans...); err != nil {
			return CandidateVerificationRetirementResult{}, err
		}
		if err := inspectCandidateVerificationWorkspaceArtifacts(prepared, false); err != nil {
			return CandidateVerificationRetirementResult{}, err
		}
		if err := publishCandidateVerificationRetirementArtifact(prepared.result.RetirementIntentPath, prepared.intentBytes); err != nil {
			return CandidateVerificationRetirementResult{}, err
		}
	}
	if candidateVerificationRetirementStageHook != nil {
		if err := candidateVerificationRetirementStageHook("after-intent"); err != nil {
			return CandidateVerificationRetirementResult{}, err
		}
	}
	if err := inspectCandidateVerificationWorkspaceArtifacts(prepared, intentCurrent); err != nil {
		return CandidateVerificationRetirementResult{}, err
	}
	var workspaceHandle *pinnedCandidateVerificationRetirementWorkspace
	if _, statErr := os.Lstat(prepared.result.WorkspaceRoot); statErr == nil {
		workspaceHandle, err = pinCandidateVerificationRetirementWorkspace(prepared.result.WorkspaceRoot)
		if err != nil {
			return CandidateVerificationRetirementResult{}, err
		}
		defer workspaceHandle.close()
	} else if !os.IsNotExist(statErr) {
		return CandidateVerificationRetirementResult{}, statErr
	}
	if candidateVerificationRetirementStageHook != nil {
		if err := candidateVerificationRetirementStageHook("before-root-apply"); err != nil {
			return CandidateVerificationRetirementResult{}, err
		}
	}
	mode := syncpkg.ExclusiveInitRetirementFirst
	if intentCurrent {
		mode = syncpkg.ExclusiveInitRetirementResume
	}
	if workspaceHandle == nil {
		inspection, err := syncpkg.InspectExclusiveInitRetirementBatch(syncpkg.ExclusiveInitRetirementResume, prepared.retirementPlans...)
		if err != nil {
			return CandidateVerificationRetirementResult{}, err
		}
		if !intentCurrent || inspection.RemainingRoots != 0 || inspection.RemainingLeaves != 0 || inspection.RemainingDirectories != 0 {
			return CandidateVerificationRetirementResult{}, fmt.Errorf("candidate verification retirement workspace parent identity is unavailable")
		}
		if err := reconcileCandidateVerificationWorkspaceRootQuarantine(prepared); err != nil {
			return CandidateVerificationRetirementResult{}, err
		}
	} else {
		if err := validatePinnedCandidateVerificationRetirementWorkspace(workspaceHandle); err != nil {
			return CandidateVerificationRetirementResult{}, err
		}
		if _, err := syncpkg.ApplyExclusiveInitRetirementBatchBoundToParent(mode, workspaceHandle.rootInfo, prepared.retirementPlans...); err != nil {
			return CandidateVerificationRetirementResult{}, err
		}
	}
	if candidateVerificationRetirementStageHook != nil {
		if err := candidateVerificationRetirementStageHook("after-roots"); err != nil {
			return CandidateVerificationRetirementResult{}, err
		}
	}
	if err := removeCandidateVerificationProvisionArtifacts(prepared, workspaceHandle); err != nil {
		return CandidateVerificationRetirementResult{}, err
	}
	if candidateVerificationRetirementStageHook != nil {
		if err := candidateVerificationRetirementStageHook("before-receipt"); err != nil {
			return CandidateVerificationRetirementResult{}, err
		}
	}
	if err := assertRetiredCandidateVerificationWorkspaceAbsent(prepared.result); err != nil {
		return CandidateVerificationRetirementResult{}, err
	}
	if err := publishCandidateVerificationRetirementArtifact(prepared.result.RetirementReceiptPath, prepared.receiptBytes); err != nil {
		return CandidateVerificationRetirementResult{}, err
	}
	if err := assertRetiredCandidateVerificationWorkspaceAbsent(prepared.result); err != nil {
		return CandidateVerificationRetirementResult{}, err
	}
	prepared.result.Mode = "retired"
	prepared.result.IsMutation = true
	prepared.result.Applied = true
	prepared.result.NextSteps = []string{"retain the repo-local retirement intent and receipt as final evidence"}
	return finalizeCandidateVerificationRetirementResult(prepared.result), nil
}

func prepareCandidateVerificationRetirementFromIntent(repoRoot, sourceCaseRoot, pack string, opt CandidateVerificationRetirementOptions) (preparedCandidateVerificationRetirement, bool, error) {
	authority, err := loadCandidateDecisionAuthority(repoRoot, sourceCaseRoot, pack, opt.PacketPath, opt.DecisionPath)
	if err != nil {
		return preparedCandidateVerificationRetirement{}, false, err
	}
	id := shortHash(authority.packetHash + authority.decisionHash)
	proofRoot := filepath.Join(authority.candidateRoot, "review-artifacts")
	intentPath := filepath.Join(proofRoot, id+".candidate-verification-retirement-intent.json")
	intent, ok, err := readCandidateVerificationRetirementArtifact(intentPath)
	if err != nil || !ok {
		return preparedCandidateVerificationRetirement{}, false, err
	}
	if err := validateCandidateVerificationRetirementArtifactAuthority(intent, authority, repoRoot, pack, "-intent"); err != nil {
		return preparedCandidateVerificationRetirement{}, false, err
	}
	if err := validateCandidateVerificationRetirementProof(authority, intent); err != nil {
		return preparedCandidateVerificationRetirement{}, false, err
	}
	result := candidateVerificationRetirementResultFromArtifact(intent)
	if candidateVerificationRetirementHash(result) != intent.RetirementSHA256 {
		return preparedCandidateVerificationRetirement{}, false, fmt.Errorf("candidate verification retirement intent stable hash mismatch")
	}
	artifact := intent
	artifact.Kind = strings.TrimSuffix(intent.Kind, "-intent")
	receiptArtifact := artifact
	receiptArtifact.Kind += "-receipt"
	receiptBytes, err := strictCandidateVerificationRetirementBytes(receiptArtifact)
	if err != nil {
		return preparedCandidateVerificationRetirement{}, false, err
	}
	intentBytes, err := strictCandidateVerificationRetirementBytes(intent)
	if err != nil {
		return preparedCandidateVerificationRetirement{}, false, err
	}
	return preparedCandidateVerificationRetirement{result: result, artifact: artifact, intentBytes: intentBytes, receiptBytes: receiptBytes, retirementPlans: append([]syncpkg.ExclusiveInitRetirementPlan(nil), intent.RetirementPlans...)}, true, nil
}

func validateCandidateVerificationRetirementArtifactAuthority(artifact candidateVerificationRetirementArtifact, authority candidateDecisionAuthority, repoRoot, pack, kindSuffix string) error {
	id := shortHash(authority.packetHash + authority.decisionHash)
	proofRoot := filepath.Join(authority.candidateRoot, "review-artifacts")
	workspace := candidateDecisionVerificationWorkspace(authority.instance.CaseRoot, authority.packetHash, authority.decisionHash)
	intentPath := filepath.Join(proofRoot, id+".candidate-verification-retirement-intent.json")
	receiptPath := filepath.Join(proofRoot, id+".candidate-verification-retirement-receipt.json")
	if artifact.SchemaVersion != 1 || artifact.Kind != "pack-memory-candidate-verification-retirement"+kindSuffix || artifact.PacketSHA256 != authority.packetHash || artifact.DecisionSHA256 != authority.decisionHash || artifact.DecisionReceiptSHA256 != sha256Hex(authority.receiptBytes) || !sameCandidateDecisionPath(artifact.RepoRoot, repoRoot) || !sameCandidateDecisionPath(artifact.SourceCaseRoot, authority.instance.CaseRoot) || artifact.Pack != pack || !sameCandidateDecisionPath(artifact.PacketPath, authority.packetPath) || !sameCandidateDecisionPath(artifact.DecisionPath, authority.decisionPath) || !sameCandidateDecisionPath(artifact.DecisionReceiptPath, authority.receiptPath) || !sameCandidateDecisionPath(artifact.WorkspaceRoot, workspace) || !sameCandidateDecisionPath(artifact.VerificationProofPath, candidateDecisionVerificationProofPath(authority.candidateRoot, authority.packetHash, authority.decisionHash)) || !sameCandidateDecisionPath(artifact.ProvisionIntentPath, filepath.Join(workspace, "provision.intent.json")) || !sameCandidateDecisionPath(artifact.ProvisionReceiptPath, filepath.Join(workspace, "provision.receipt.json")) || !sameCandidateDecisionPath(artifact.RetirementIntentPath, intentPath) || !sameCandidateDecisionPath(artifact.RetirementReceiptPath, receiptPath) || len(artifact.Roots) != 2 || len(artifact.RetirementPlans) != 2 {
		return fmt.Errorf("candidate verification retirement artifact authority binding mismatch")
	}
	expectedProvisionArtifacts := []string{filepath.Join(workspace, "provision.receipt.json"), filepath.Join(workspace, "provision.intent.json")}
	expectedAncestors := []string{filepath.Dir(workspace), filepath.Dir(filepath.Dir(workspace))}
	if len(artifact.ProvisionArtifactsToDelete) != len(expectedProvisionArtifacts) || len(artifact.EmptyAncestorsToRemove) != len(expectedAncestors) {
		return fmt.Errorf("candidate verification retirement artifact cleanup binding mismatch")
	}
	for i := range expectedProvisionArtifacts {
		if !sameCandidateDecisionPath(artifact.ProvisionArtifactsToDelete[i], expectedProvisionArtifacts[i]) || !sameCandidateDecisionPath(artifact.EmptyAncestorsToRemove[i], expectedAncestors[i]) {
			return fmt.Errorf("candidate verification retirement artifact cleanup binding mismatch")
		}
	}
	for i, role := range []string{"fresh", "attached"} {
		root := filepath.Join(workspace, role)
		plan := artifact.RetirementPlans[i]
		if artifact.Roots[i].Role != role || !sameCandidateDecisionPath(artifact.Roots[i].CaseRoot, root) || plan.SchemaVersion != 1 || plan.Command != "exclusive-init-retirement" || plan.Role != role || !sameCandidateDecisionPath(plan.CaseRoot, root) || plan.ProvisionID != id || len(plan.Leaves) == 0 || len(artifact.Roots[i].Deletes) != len(plan.Leaves) {
			return fmt.Errorf("candidate verification retirement artifact root binding mismatch")
		}
		for j, leaf := range plan.Leaves {
			expectedDelete := filepath.Join(root, filepath.FromSlash(leaf.Path))
			if !sameCandidateDecisionPath(artifact.Roots[i].Deletes[j], expectedDelete) || strings.TrimSpace(leaf.SHA256) == "" || leaf.Size <= 0 {
				return fmt.Errorf("candidate verification retirement artifact leaf binding mismatch")
			}
		}
	}
	return nil
}

func validateCandidateVerificationRetirementProof(authority candidateDecisionAuthority, artifact candidateVerificationRetirementArtifact) error {
	_, proofBytes, err := readStrictCandidateDecisionFile(artifact.VerificationProofPath, "candidate verification proof")
	if err != nil {
		return err
	}
	if sha256Hex(proofBytes) != artifact.VerificationProofSHA256 {
		return fmt.Errorf("candidate verification retirement proof hash mismatch")
	}
	var proof CandidateDecisionVerificationResult
	if err := decodeStrictJSON(proofBytes, &proof); err != nil {
		return fmt.Errorf("decode candidate verification proof: %w", err)
	}
	if proof.SchemaVersion != 1 || proof.Kind != "pack-memory-candidate-decision-verification" || proof.Pack != authority.receipt.Pack || proof.PacketHash != authority.packetHash || proof.DecisionHash != authority.decisionHash || proof.ReceiptHash != sha256Hex(authority.receiptBytes) || proof.ProvisionIntentSHA256 != artifact.ProvisionIntentSHA256 || proof.ProvisionReceiptSHA256 != artifact.ProvisionReceiptSHA256 || proof.VerifiedActionsSHA256 != candidateDecisionActionsSHA256(authority.receipt.Actions) || !proof.IsMutation || !proof.Applied || !proof.Ready || proof.PackDoctorRows <= 0 || proof.FreshDoctorRows <= 0 || proof.AttachedDoctorRows <= 0 || len(artifact.Roots) != 2 {
		return fmt.Errorf("candidate verification retirement proof authority binding mismatch")
	}
	return nil
}

func loadCompletedCandidateVerificationRetirement(repoRoot, sourceCaseRoot, pack string, opt CandidateVerificationRetirementOptions) (CandidateVerificationRetirementResult, bool, error) {
	authority, err := loadCandidateDecisionAuthority(repoRoot, sourceCaseRoot, pack, opt.PacketPath, opt.DecisionPath)
	if err != nil {
		return CandidateVerificationRetirementResult{}, false, err
	}
	id := shortHash(authority.packetHash + authority.decisionHash)
	proofRoot := filepath.Join(authority.candidateRoot, "review-artifacts")
	intentPath := filepath.Join(proofRoot, id+".candidate-verification-retirement-intent.json")
	receiptPath := filepath.Join(proofRoot, id+".candidate-verification-retirement-receipt.json")
	receipt, ok, err := readCandidateVerificationRetirementArtifact(receiptPath)
	if err != nil || !ok {
		return CandidateVerificationRetirementResult{}, false, err
	}
	intent, intentOK, err := readCandidateVerificationRetirementArtifact(intentPath)
	if err != nil || !intentOK {
		return CandidateVerificationRetirementResult{}, false, fmt.Errorf("candidate verification retirement receipt lacks current retained intent")
	}
	expectedIntent := receipt
	expectedIntent.Kind = strings.TrimSuffix(receipt.Kind, "-receipt") + "-intent"
	if !reflect.DeepEqual(intent, expectedIntent) {
		return CandidateVerificationRetirementResult{}, false, fmt.Errorf("candidate verification retirement receipt/intent authority binding mismatch")
	}
	if err := validateCandidateVerificationRetirementArtifactAuthority(receipt, authority, repoRoot, pack, "-receipt"); err != nil {
		return CandidateVerificationRetirementResult{}, false, err
	}
	if err := validateCandidateVerificationRetirementProof(authority, receipt); err != nil {
		return CandidateVerificationRetirementResult{}, false, err
	}
	result := candidateVerificationRetirementResultFromArtifact(receipt)
	if candidateVerificationRetirementHash(result) != receipt.RetirementSHA256 {
		return CandidateVerificationRetirementResult{}, false, fmt.Errorf("candidate verification retirement receipt stable hash mismatch")
	}
	if err := assertRetiredCandidateVerificationWorkspaceAbsent(result); err != nil {
		return CandidateVerificationRetirementResult{}, false, err
	}
	result.Mode = "already-retired"
	result.IsMutation = !opt.WhatIf
	result.Applied = !opt.WhatIf
	result.Replay = true
	result.NextSteps = []string{"retain the repo-local retirement intent and receipt as final evidence"}
	return result, true, nil
}

func candidateVerificationRetirementResultFromArtifact(artifact candidateVerificationRetirementArtifact) CandidateVerificationRetirementResult {
	return CandidateVerificationRetirementResult{SchemaVersion: artifact.SchemaVersion, Kind: strings.TrimSuffix(strings.TrimSuffix(artifact.Kind, "-receipt"), "-intent"), RepoRoot: artifact.RepoRoot, SourceCaseRoot: artifact.SourceCaseRoot, Pack: artifact.Pack, PacketPath: artifact.PacketPath, PacketSHA256: artifact.PacketSHA256, DecisionPath: artifact.DecisionPath, DecisionSHA256: artifact.DecisionSHA256, DecisionReceiptPath: artifact.DecisionReceiptPath, DecisionReceiptSHA256: artifact.DecisionReceiptSHA256, VerificationProofPath: artifact.VerificationProofPath, VerificationProofSHA256: artifact.VerificationProofSHA256, ProvisionIntentPath: artifact.ProvisionIntentPath, ProvisionIntentSHA256: artifact.ProvisionIntentSHA256, ProvisionReceiptPath: artifact.ProvisionReceiptPath, ProvisionReceiptSHA256: artifact.ProvisionReceiptSHA256, WorkspaceRoot: artifact.WorkspaceRoot, RetirementIntentPath: artifact.RetirementIntentPath, RetirementReceiptPath: artifact.RetirementReceiptPath, RetirementSHA256: artifact.RetirementSHA256, Roots: append([]CandidateVerificationRetirementRoot(nil), artifact.Roots...), ProvisionArtifactsToDelete: append([]string(nil), artifact.ProvisionArtifactsToDelete...), EmptyAncestorsToRemove: append([]string(nil), artifact.EmptyAncestorsToRemove...), Boundary: append([]string(nil), artifact.Boundary...), RetirementPlans: append([]syncpkg.ExclusiveInitRetirementPlan(nil), artifact.RetirementPlans...)}
}

func prepareCandidateVerificationRetirement(repoRoot, sourceCaseRoot, pack string, opt CandidateVerificationRetirementOptions) (preparedCandidateVerificationRetirement, error) {
	authority, err := loadCandidateDecisionAuthority(repoRoot, sourceCaseRoot, pack, opt.PacketPath, opt.DecisionPath)
	if err != nil {
		return preparedCandidateVerificationRetirement{}, err
	}
	if authority.receipt.Accepted <= 0 || !authority.receipt.VerificationPending {
		return preparedCandidateVerificationRetirement{}, fmt.Errorf("candidate verification retirement requires a current accepted decision receipt")
	}
	workspace := candidateDecisionVerificationWorkspace(authority.instance.CaseRoot, authority.packetHash, authority.decisionHash)
	freshRoot := filepath.Join(workspace, "fresh")
	attachedRoot := filepath.Join(workspace, "attached")
	provisionIntentPath := filepath.Join(workspace, "provision.intent.json")
	provisionReceiptPath := filepath.Join(workspace, "provision.receipt.json")
	proofPath := candidateDecisionVerificationProofPath(authority.candidateRoot, authority.packetHash, authority.decisionHash)
	proofBytes, provisionIntentBytes, provisionReceiptBytes, provisionReceipt, err := loadCandidateVerificationRetirementEvidence(authority, proofPath, provisionIntentPath, provisionReceiptPath, freshRoot, attachedRoot)
	if err != nil {
		return preparedCandidateVerificationRetirement{}, err
	}
	plans := make([]syncpkg.ExclusiveInitPlan, 0, 2)
	roots := make([]CandidateVerificationRetirementRoot, 0, 2)
	for _, provisioned := range provisionReceipt.Cases {
		plan := syncpkg.ExclusiveInitPlan{SchemaVersion: 1, Command: "exclusive-init", CaseRoot: provisioned.CaseRoot, RepoRoot: repoRoot, Pack: pack, ProjectName: provisioned.ProjectName, ProvisionID: provisionReceipt.ProvisionID, Role: provisioned.Role, CreatedAt: time.Unix(0, 0).UTC().Format(time.RFC3339Nano), Writes: append([]syncpkg.ExclusiveInitWrite(nil), provisioned.Writes...), BlockedActions: []string{"existing root takeover", "overwrite", "backup", "force", "authority/confirmed writes", "heavy-tool execution"}}
		plans = append(plans, plan)
		deletes := make([]string, 0, len(plan.Writes))
		for _, write := range plan.Writes {
			deletes = append(deletes, write.TargetPath)
		}
		roots = append(roots, CandidateVerificationRetirementRoot{Role: provisioned.Role, CaseRoot: provisioned.CaseRoot, Deletes: deletes})
	}
	retirementPlans := make([]syncpkg.ExclusiveInitRetirementPlan, 0, len(plans))
	for _, plan := range plans {
		retirementPlan, err := syncpkg.BuildExclusiveInitRetirementPlan(plan)
		if err != nil {
			return preparedCandidateVerificationRetirement{}, err
		}
		retirementPlans = append(retirementPlans, retirementPlan)
	}
	proofRoot := filepath.Join(authority.candidateRoot, "review-artifacts")
	retirementID := shortHash(authority.packetHash + authority.decisionHash)
	intentPath := filepath.Join(proofRoot, retirementID+".candidate-verification-retirement-intent.json")
	receiptPath := filepath.Join(proofRoot, retirementID+".candidate-verification-retirement-receipt.json")
	result := CandidateVerificationRetirementResult{SchemaVersion: 1, Kind: "pack-memory-candidate-verification-retirement", RepoRoot: repoRoot, SourceCaseRoot: authority.instance.CaseRoot, Pack: pack, PacketPath: authority.packetPath, PacketSHA256: authority.packetHash, DecisionPath: authority.decisionPath, DecisionSHA256: authority.decisionHash, DecisionReceiptPath: authority.receiptPath, DecisionReceiptSHA256: sha256Hex(authority.receiptBytes), VerificationProofPath: proofPath, VerificationProofSHA256: sha256Hex(proofBytes), ProvisionIntentPath: provisionIntentPath, ProvisionIntentSHA256: sha256Hex(provisionIntentBytes), ProvisionReceiptPath: provisionReceiptPath, ProvisionReceiptSHA256: sha256Hex(provisionReceiptBytes), WorkspaceRoot: workspace, RetirementIntentPath: intentPath, RetirementReceiptPath: receiptPath, Roots: roots, ProvisionArtifactsToDelete: []string{provisionReceiptPath, provisionIntentPath}, EmptyAncestorsToRemove: []string{filepath.Dir(workspace), filepath.Dir(filepath.Dir(workspace))}, RetirementPlans: append([]syncpkg.ExclusiveInitRetirementPlan(nil), retirementPlans...), Boundary: []string{"retirement deletes only the two exact generated cases and their canonical provision workspace", "retirement retains the repo-local intent and receipt", "retirement does not write authority/confirmed or execute heavy tools", "workspace recreation after receipt fails closed and is never deleted automatically"}}
	result.RetirementSHA256 = candidateVerificationRetirementHash(result)
	artifact := candidateVerificationRetirementArtifact{SchemaVersion: 1, Kind: result.Kind, RepoRoot: result.RepoRoot, SourceCaseRoot: result.SourceCaseRoot, Pack: result.Pack, PacketPath: result.PacketPath, PacketSHA256: result.PacketSHA256, DecisionPath: result.DecisionPath, DecisionSHA256: result.DecisionSHA256, DecisionReceiptPath: result.DecisionReceiptPath, DecisionReceiptSHA256: result.DecisionReceiptSHA256, VerificationProofPath: result.VerificationProofPath, VerificationProofSHA256: result.VerificationProofSHA256, ProvisionIntentPath: result.ProvisionIntentPath, ProvisionIntentSHA256: result.ProvisionIntentSHA256, ProvisionReceiptPath: result.ProvisionReceiptPath, ProvisionReceiptSHA256: result.ProvisionReceiptSHA256, WorkspaceRoot: result.WorkspaceRoot, RetirementIntentPath: result.RetirementIntentPath, RetirementReceiptPath: result.RetirementReceiptPath, RetirementSHA256: result.RetirementSHA256, Roots: append([]CandidateVerificationRetirementRoot(nil), result.Roots...), ProvisionArtifactsToDelete: append([]string(nil), result.ProvisionArtifactsToDelete...), EmptyAncestorsToRemove: append([]string(nil), result.EmptyAncestorsToRemove...), Boundary: append([]string(nil), result.Boundary...), RetirementPlans: append([]syncpkg.ExclusiveInitRetirementPlan(nil), retirementPlans...)}
	intent := artifact
	intent.Kind += "-intent"
	receipt := artifact
	receipt.Kind += "-receipt"
	intentBytes, err := strictCandidateVerificationRetirementBytes(intent)
	if err != nil {
		return preparedCandidateVerificationRetirement{}, err
	}
	receiptBytes, err := strictCandidateVerificationRetirementBytes(receipt)
	if err != nil {
		return preparedCandidateVerificationRetirement{}, err
	}
	return preparedCandidateVerificationRetirement{result: result, artifact: artifact, intentBytes: intentBytes, receiptBytes: receiptBytes, retirementPlans: retirementPlans}, nil
}

func loadCandidateVerificationRetirementEvidence(authority candidateDecisionAuthority, proofPath, intentPath, receiptPath, freshRoot, attachedRoot string) ([]byte, []byte, []byte, candidateVerificationProvisionReceipt, error) {
	_, proofBytes, err := readStrictCandidateDecisionFile(proofPath, "candidate verification proof")
	if err != nil {
		return nil, nil, nil, candidateVerificationProvisionReceipt{}, err
	}
	_, intentBytes, err := readStrictCandidateDecisionFile(intentPath, "candidate verification provision intent")
	if err != nil {
		return nil, nil, nil, candidateVerificationProvisionReceipt{}, err
	}
	_, receiptBytes, err := readStrictCandidateDecisionFile(receiptPath, "candidate verification provision receipt")
	if err != nil {
		return nil, nil, nil, candidateVerificationProvisionReceipt{}, err
	}
	var intent, receipt candidateVerificationProvisionReceipt
	if err := decodeStrictJSON(intentBytes, &intent); err != nil {
		return nil, nil, nil, candidateVerificationProvisionReceipt{}, fmt.Errorf("decode candidate verification provision intent: %w", err)
	}
	if err := decodeStrictJSON(receiptBytes, &receipt); err != nil {
		return nil, nil, nil, candidateVerificationProvisionReceipt{}, fmt.Errorf("decode candidate verification provision receipt: %w", err)
	}
	expectedIntent := receipt
	expectedIntent.Kind = "pack-memory-candidate-verification-case-provision-intent"
	if !candidateVerificationProvisionReceiptMatches(intent, expectedIntent) || !bytes.Equal(intentBytes, mustCandidateVerificationProvisionBytes(expectedIntent)) {
		return nil, nil, nil, candidateVerificationProvisionReceipt{}, fmt.Errorf("candidate verification provision intent/receipt binding mismatch")
	}
	if receipt.Kind != "pack-memory-candidate-verification-case-provision-receipt" || !sameCandidateDecisionPath(receipt.WorkspaceRoot, filepath.Dir(intentPath)) || !sameCandidateDecisionPath(receipt.IntentPath, intentPath) || !sameCandidateDecisionPath(receipt.FreshCaseRoot, freshRoot) || !sameCandidateDecisionPath(receipt.AttachedCaseRoot, attachedRoot) || len(receipt.Cases) != 2 || !sameCandidateDecisionPath(receipt.Cases[0].CaseRoot, freshRoot) || !sameCandidateDecisionPath(receipt.Cases[1].CaseRoot, attachedRoot) || receipt.Cases[0].Role != "fresh" || receipt.Cases[1].Role != "attached" || receipt.DecisionReceiptSHA256 != sha256Hex(authority.receiptBytes) || !sameCandidateDecisionPath(receipt.DecisionReceiptPath, authority.receiptPath) {
		return nil, nil, nil, candidateVerificationProvisionReceipt{}, fmt.Errorf("candidate verification provision receipt binding mismatch")
	}
	var proof CandidateDecisionVerificationResult
	if err := decodeStrictJSON(proofBytes, &proof); err != nil {
		return nil, nil, nil, candidateVerificationProvisionReceipt{}, fmt.Errorf("decode candidate verification proof: %w", err)
	}
	if proof.SchemaVersion != 1 || proof.Kind != "pack-memory-candidate-decision-verification" || proof.Pack != authority.receipt.Pack || proof.PacketHash != authority.packetHash || proof.DecisionHash != authority.decisionHash || proof.ReceiptHash != sha256Hex(authority.receiptBytes) || proof.ProvisionIntentSHA256 != sha256Hex(intentBytes) || proof.ProvisionReceiptSHA256 != sha256Hex(receiptBytes) || proof.VerifiedActionsSHA256 != candidateDecisionActionsSHA256(authority.receipt.Actions) || !proof.IsMutation || !proof.Applied || !proof.Ready || proof.PackDoctorRows <= 0 || proof.FreshDoctorRows <= 0 || proof.AttachedDoctorRows <= 0 {
		return nil, nil, nil, candidateVerificationProvisionReceipt{}, fmt.Errorf("candidate verification proof/provision/decision receipt binding mismatch")
	}
	return proofBytes, intentBytes, receiptBytes, receipt, nil
}

func mustCandidateVerificationProvisionBytes(value candidateVerificationProvisionReceipt) []byte {
	data, _ := candidateVerificationProvisionReceiptBytes(value)
	return data
}

func candidateVerificationRetirementHash(result CandidateVerificationRetirementResult) string {
	stable := result
	stable.Mode = ""
	stable.RetirementSHA256 = ""
	stable.IsMutation = false
	stable.Applied = false
	stable.Replay = false
	stable.ApplyCommand = ""
	stable.MissionCommanderAction = nil
	stable.MissionCommanderActionQueue = nil
	stable.NextSteps = nil
	data, _ := json.Marshal(stable)
	return sha256Hex(data)
}

func candidateVerificationRetirementCommand(result CandidateVerificationRetirementResult) string {
	return fmt.Sprintf("/rekit promote -Target %s -PacketPath %s -CandidateDecisionPath %s -RetireCandidateVerificationWorkspace -ExpectedRetirementSha256 %s -Apply -Format json", quoteCandidateDecisionArg(result.SourceCaseRoot), quoteCandidateDecisionArg(result.PacketPath), quoteCandidateDecisionArg(result.DecisionPath), result.RetirementSHA256)
}

func strictCandidateVerificationRetirementBytes(value candidateVerificationRetirementArtifact) ([]byte, error) {
	data, err := json.MarshalIndent(value, "", "  ")
	if err != nil {
		return nil, err
	}
	return append(data, '\n'), nil
}

func candidateVerificationRetirementArtifactMatches(actual candidateVerificationRetirementArtifact, expected candidateVerificationRetirementArtifact, kindSuffix string) bool {
	expected.Kind += kindSuffix
	return reflect.DeepEqual(actual, expected)
}

func readCandidateVerificationRetirementArtifact(path string) (candidateVerificationRetirementArtifact, bool, error) {
	info, err := os.Lstat(path)
	if os.IsNotExist(err) {
		return candidateVerificationRetirementArtifact{}, false, nil
	}
	if err != nil || info.Mode()&os.ModeSymlink != 0 || !info.Mode().IsRegular() || info.Size() <= 0 || info.Size() > maxCandidateDecisionBytes {
		return candidateVerificationRetirementArtifact{}, false, fmt.Errorf("candidate verification retirement artifact must be a bounded non-symlink regular file: %s", path)
	}
	_, data, err := readStrictCandidateDecisionFile(path, "candidate verification retirement artifact")
	if err != nil {
		return candidateVerificationRetirementArtifact{}, false, err
	}
	var artifact candidateVerificationRetirementArtifact
	if err := decodeStrictJSON(data, &artifact); err != nil {
		return candidateVerificationRetirementArtifact{}, false, err
	}
	return artifact, true, nil
}

func publishCandidateVerificationRetirementArtifact(path string, expected []byte) error {
	rootPath := filepath.Dir(path)
	if err := rejectCandidateDecisionSymlinkPath(rootPath, path, true); err != nil {
		return err
	}
	root, err := os.OpenRoot(rootPath)
	if err != nil {
		return err
	}
	defer root.Close()
	name := filepath.Base(path)
	tempName := "." + name + "." + sha256Hex(expected)[:16] + ".tmp"
	if err := reconcileCandidateVerificationRetirementTemp(root, tempName, name, expected); err != nil {
		return err
	}
	if existing, err := root.ReadFile(name); err == nil {
		if !bytes.Equal(existing, expected) {
			return fmt.Errorf("candidate verification retirement artifact already exists with different bytes: %s", path)
		}
		return syncCandidateDecisionDirectory(rootPath)
	} else if !os.IsNotExist(err) {
		return err
	}
	if _, err := root.Lstat(tempName); os.IsNotExist(err) {
		if err := writeWorkspaceExclusiveFile(root, tempName, expected); err != nil {
			return err
		}
	} else if err != nil {
		return err
	}
	if err := root.Link(tempName, name); err != nil {
		return fmt.Errorf("publish candidate verification retirement artifact without replacement: %w", err)
	}
	if err := reconcileCandidateVerificationRetirementTemp(root, tempName, name, expected); err != nil {
		return err
	}
	return syncCandidateDecisionDirectory(rootPath)
}

func reconcileCandidateVerificationRetirementTemp(root *os.Root, tempName, finalName string, expected []byte) error {
	tempInfo, err := root.Lstat(tempName)
	if os.IsNotExist(err) {
		return nil
	}
	if err != nil || !tempInfo.Mode().IsRegular() {
		return fmt.Errorf("candidate verification retirement owned temp is not regular")
	}
	tempBytes, err := root.ReadFile(tempName)
	if err != nil || !bytes.Equal(tempBytes, expected) {
		return fmt.Errorf("candidate verification retirement owned temp has different bytes")
	}
	finalInfo, err := root.Lstat(finalName)
	if os.IsNotExist(err) {
		return nil
	}
	if err != nil || !finalInfo.Mode().IsRegular() {
		return fmt.Errorf("candidate verification retirement final artifact is not regular")
	}
	finalBytes, err := root.ReadFile(finalName)
	if err != nil || !bytes.Equal(finalBytes, expected) || !os.SameFile(tempInfo, finalInfo) {
		return fmt.Errorf("candidate verification retirement temp/final reconciliation mismatch")
	}
	return root.Remove(tempName)
}

func inspectCandidateVerificationWorkspaceArtifacts(prepared preparedCandidateVerificationRetirement, allowPartial bool) error {
	for _, path := range prepared.result.ProvisionArtifactsToDelete {
		info, err := os.Lstat(path)
		if os.IsNotExist(err) && allowPartial {
			continue
		}
		if err != nil || info.Mode()&os.ModeSymlink != 0 || !info.Mode().IsRegular() {
			return fmt.Errorf("candidate verification retirement provision artifact is missing or not regular: %s", path)
		}
		if err := rejectCandidateDecisionSymlinkPath(prepared.result.WorkspaceRoot, path, false); err != nil {
			return err
		}
		data, err := os.ReadFile(path)
		if err != nil {
			return err
		}
		expectedHash := prepared.result.ProvisionIntentSHA256
		if sameCandidateDecisionPath(path, prepared.result.ProvisionReceiptPath) {
			expectedHash = prepared.result.ProvisionReceiptSHA256
		}
		if sha256Hex(data) != expectedHash {
			return fmt.Errorf("candidate verification retirement provision artifact changed: %s", path)
		}
	}
	return nil
}

func pinCandidateVerificationRetirementWorkspace(workspace string) (*pinnedCandidateVerificationRetirementWorkspace, error) {
	parentPath := filepath.Dir(workspace)
	if err := rejectCandidateDecisionSymlinkPath(filepath.Dir(parentPath), parentPath, false); err != nil {
		return nil, err
	}
	parentInfo, err := os.Lstat(parentPath)
	if err != nil || parentInfo.Mode()&os.ModeSymlink != 0 || !parentInfo.IsDir() {
		return nil, fmt.Errorf("candidate verification retirement workspace parent is not a non-symlink directory")
	}
	parent, err := os.OpenRoot(parentPath)
	if err != nil {
		return nil, err
	}
	rootInfo, err := parent.Lstat(filepath.Base(workspace))
	if err != nil || rootInfo.Mode()&os.ModeSymlink != 0 || !rootInfo.IsDir() {
		_ = parent.Close()
		return nil, fmt.Errorf("candidate verification retirement workspace is not a non-symlink directory")
	}
	root, err := parent.OpenRoot(filepath.Base(workspace))
	if err != nil {
		_ = parent.Close()
		return nil, err
	}
	opened, err := root.Stat(".")
	if err != nil || !os.SameFile(rootInfo, opened) {
		_ = root.Close()
		_ = parent.Close()
		return nil, fmt.Errorf("candidate verification retirement workspace changed while opening")
	}
	return &pinnedCandidateVerificationRetirementWorkspace{parent: parent, root: root, parentInfo: parentInfo, rootInfo: rootInfo, name: filepath.Base(workspace)}, nil
}

func validatePinnedCandidateVerificationRetirementWorkspace(workspace *pinnedCandidateVerificationRetirementWorkspace) error {
	if workspace == nil || workspace.parent == nil || workspace.root == nil {
		return fmt.Errorf("candidate verification retirement workspace handle is unavailable")
	}
	current, err := workspace.parent.Lstat(workspace.name)
	opened, openErr := workspace.root.Stat(".")
	if err != nil || openErr != nil || current.Mode()&os.ModeSymlink != 0 || !current.IsDir() || !os.SameFile(workspace.rootInfo, current) || !os.SameFile(workspace.rootInfo, opened) {
		return fmt.Errorf("candidate verification retirement workspace was rebound")
	}
	return nil
}

func (workspace *pinnedCandidateVerificationRetirementWorkspace) close() {
	if workspace == nil {
		return
	}
	if workspace.root != nil {
		_ = workspace.root.Close()
	}
	if workspace.parent != nil {
		_ = workspace.parent.Close()
	}
}

func removeCandidateVerificationProvisionArtifacts(prepared preparedCandidateVerificationRetirement, workspace *pinnedCandidateVerificationRetirementWorkspace) error {
	if workspace == nil {
		if err := assertRetiredCandidateVerificationWorkspaceAbsent(prepared.result); err != nil {
			return err
		}
		return nil
	}
	for _, name := range []string{"provision.receipt.json", "provision.intent.json"} {
		expectedHash := prepared.result.ProvisionIntentSHA256
		if name == "provision.receipt.json" {
			expectedHash = prepared.result.ProvisionReceiptSHA256
		}
		if err := retireCandidateVerificationWorkspaceArtifact(workspace.root, name, expectedHash); err != nil {
			return err
		}
	}
	if err := syncCandidateDecisionDirectory(prepared.result.WorkspaceRoot); err != nil {
		return err
	}
	if err := retireCandidateVerificationWorkspaceRoot(prepared, workspace); err != nil {
		return err
	}
	for _, ancestor := range prepared.result.EmptyAncestorsToRemove {
		if sameCandidateDecisionPath(filepath.Base(ancestor), ".rekit") {
			break
		}
		if err := os.Remove(ancestor); err != nil {
			if !os.IsNotExist(err) {
				break
			}
			continue
		}
		if err := syncCandidateDecisionDirectory(filepath.Dir(ancestor)); err != nil {
			return err
		}
	}
	return nil
}

func reconcileCandidateVerificationWorkspaceRootQuarantine(prepared preparedCandidateVerificationRetirement) error {
	parentPath := filepath.Dir(prepared.result.WorkspaceRoot)
	parent, err := os.OpenRoot(parentPath)
	if os.IsNotExist(err) {
		return nil
	}
	if err != nil {
		return err
	}
	defer parent.Close()
	name := filepath.Base(prepared.result.WorkspaceRoot)
	quarantine := "." + name + ".retiring-" + prepared.result.RetirementSHA256[:16]
	if _, err := parent.Lstat(name); err == nil || !os.IsNotExist(err) {
		return fmt.Errorf("candidate verification retirement workspace canonical name rebound")
	}
	info, err := parent.Lstat(quarantine)
	if os.IsNotExist(err) {
		return nil
	}
	if err != nil || info.Mode()&os.ModeSymlink != 0 || !info.IsDir() {
		return fmt.Errorf("candidate verification retirement workspace quarantine changed")
	}
	root, err := parent.OpenRoot(quarantine)
	if err != nil {
		return err
	}
	defer root.Close()
	opened, err := root.Stat(".")
	entries, readErr := fs.ReadDir(root.FS(), ".")
	if err != nil || readErr != nil || !os.SameFile(info, opened) || len(entries) != 0 {
		return fmt.Errorf("candidate verification retirement workspace quarantine is not exact empty")
	}
	if candidateVerificationRetirementStageHook != nil {
		if err := candidateVerificationRetirementStageHook("before-workspace-quarantine-remove"); err != nil {
			return err
		}
	}
	current, err := parent.Lstat(quarantine)
	currentRoot, openErr := parent.OpenRoot(quarantine)
	if err != nil || openErr != nil {
		if currentRoot != nil {
			_ = currentRoot.Close()
		}
		return fmt.Errorf("candidate verification retirement workspace quarantine changed before removal")
	}
	currentOpened, statErr := currentRoot.Stat(".")
	currentEntries, readErr := fs.ReadDir(currentRoot.FS(), ".")
	closeErr := currentRoot.Close()
	if statErr != nil || readErr != nil || closeErr != nil || current.Mode()&os.ModeSymlink != 0 || !current.IsDir() || !os.SameFile(info, current) || !os.SameFile(info, currentOpened) || len(currentEntries) != 0 {
		return fmt.Errorf("candidate verification retirement workspace quarantine changed before removal")
	}
	if err := root.Close(); err != nil {
		return err
	}
	if err := parent.Remove(quarantine); err != nil {
		return err
	}
	return syncCandidateDecisionDirectory(parentPath)
}

func retireCandidateVerificationWorkspaceRoot(prepared preparedCandidateVerificationRetirement, workspace *pinnedCandidateVerificationRetirementWorkspace) error {
	quarantine := "." + workspace.name + ".retiring-" + prepared.result.RetirementSHA256[:16]
	canonical, canonicalErr := workspace.parent.Lstat(workspace.name)
	quarantined, quarantineErr := workspace.parent.Lstat(quarantine)
	canonicalPresent := canonicalErr == nil
	quarantinePresent := quarantineErr == nil
	if canonicalErr != nil && !os.IsNotExist(canonicalErr) {
		return canonicalErr
	}
	if quarantineErr != nil && !os.IsNotExist(quarantineErr) {
		return quarantineErr
	}
	if canonicalPresent && quarantinePresent {
		return fmt.Errorf("candidate verification retirement workspace and owned quarantine both exist")
	}
	if quarantinePresent {
		if quarantined.Mode()&os.ModeSymlink != 0 || !quarantined.IsDir() || !os.SameFile(quarantined, workspace.rootInfo) {
			return fmt.Errorf("candidate verification retirement workspace quarantine identity changed")
		}
	} else {
		if !canonicalPresent || canonical.Mode()&os.ModeSymlink != 0 || !canonical.IsDir() || !os.SameFile(canonical, workspace.rootInfo) {
			return fmt.Errorf("candidate verification retirement workspace was rebound before quarantine")
		}
		if candidateVerificationRetirementStageHook != nil {
			if err := candidateVerificationRetirementStageHook("before-workspace-quarantine"); err != nil {
				return err
			}
		}
		if err := workspace.parent.Rename(workspace.name, quarantine); err != nil {
			return fmt.Errorf("candidate verification retirement quarantine workspace: %w", err)
		}
		if candidateVerificationRetirementStageHook != nil {
			if err := candidateVerificationRetirementStageHook("after-workspace-quarantine"); err != nil {
				return err
			}
		}
	}
	if _, err := workspace.parent.Lstat(workspace.name); !os.IsNotExist(err) {
		return fmt.Errorf("candidate verification retirement workspace canonical name rebound")
	}
	current, err := workspace.parent.Lstat(quarantine)
	opened, openErr := workspace.root.Stat(".")
	if err != nil || openErr != nil || current.Mode()&os.ModeSymlink != 0 || !current.IsDir() || !os.SameFile(workspace.rootInfo, current) || !os.SameFile(workspace.rootInfo, opened) {
		return fmt.Errorf("candidate verification retirement quarantined workspace identity changed")
	}
	if err := workspace.root.Close(); err != nil {
		return err
	}
	workspace.root = nil
	if err := workspace.parent.Remove(quarantine); err != nil {
		return fmt.Errorf("candidate verification retirement remove quarantined workspace: %w", err)
	}
	return syncCandidateDecisionDirectory(filepath.Dir(prepared.result.WorkspaceRoot))
}

func retireCandidateVerificationWorkspaceArtifact(root *os.Root, name, expectedHash string) error {
	quarantine := "." + name + ".retiring-" + expectedHash[:16]
	info, err := root.Lstat(name)
	if os.IsNotExist(err) {
		quarantined, quarantineErr := root.Lstat(quarantine)
		if os.IsNotExist(quarantineErr) {
			return nil
		}
		if quarantineErr != nil || !quarantined.Mode().IsRegular() {
			return fmt.Errorf("candidate verification retirement quarantine is not regular: %s", quarantine)
		}
		data, readErr := root.ReadFile(quarantine)
		if readErr != nil || sha256Hex(data) != expectedHash {
			return fmt.Errorf("candidate verification retirement quarantine changed: %s", quarantine)
		}
		return root.Remove(quarantine)
	}
	if err != nil || info.Mode()&os.ModeSymlink != 0 || !info.Mode().IsRegular() {
		return fmt.Errorf("candidate verification retirement refuses provision artifact: %s", name)
	}
	if _, quarantineErr := root.Lstat(quarantine); quarantineErr == nil || !os.IsNotExist(quarantineErr) {
		return fmt.Errorf("candidate verification retirement quarantine already exists: %s", quarantine)
	}
	data, err := root.ReadFile(name)
	if err != nil || sha256Hex(data) != expectedHash {
		return fmt.Errorf("candidate verification retirement provision artifact changed: %s", name)
	}
	if candidateVerificationRetirementStageHook != nil {
		if err := candidateVerificationRetirementStageHook("before-artifact-quarantine:" + name); err != nil {
			return err
		}
	}
	if err := root.Rename(name, quarantine); err != nil {
		return fmt.Errorf("candidate verification retirement quarantine provision artifact %s: %w", name, err)
	}
	movedInfo, err := root.Lstat(quarantine)
	if err != nil || !os.SameFile(info, movedInfo) || !movedInfo.Mode().IsRegular() {
		if _, currentErr := root.Lstat(name); os.IsNotExist(currentErr) {
			_ = root.Rename(quarantine, name)
		}
		return fmt.Errorf("candidate verification retirement provision artifact identity changed during quarantine: %s", name)
	}
	moved, err := root.ReadFile(quarantine)
	if err != nil || sha256Hex(moved) != expectedHash {
		if _, currentErr := root.Lstat(name); os.IsNotExist(currentErr) {
			_ = root.Rename(quarantine, name)
		}
		return fmt.Errorf("candidate verification retirement quarantined provision artifact changed: %s", name)
	}
	if candidateVerificationRetirementStageHook != nil {
		if err := candidateVerificationRetirementStageHook("after-artifact-quarantine:" + name); err != nil {
			return err
		}
	}
	if err := root.Remove(quarantine); err != nil {
		return err
	}
	return nil
}

func assertRetiredCandidateVerificationWorkspaceAbsent(result CandidateVerificationRetirementResult) error {
	if _, err := os.Lstat(result.WorkspaceRoot); err == nil || !os.IsNotExist(err) {
		return fmt.Errorf("retired candidate verification workspace has reappeared; refusing automatic deletion: %s", result.WorkspaceRoot)
	}
	return nil
}
