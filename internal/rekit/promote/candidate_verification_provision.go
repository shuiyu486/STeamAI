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
	"strings"
	"time"

	"github.com/shuiyu486/re-context-kits/internal/rekit/doctor"
	syncpkg "github.com/shuiyu486/re-context-kits/internal/rekit/sync"
)

type CandidateVerificationProvisionOptions struct {
	PacketPath              string
	DecisionPath            string
	FreshCaseRoot           string
	AttachedCaseRoot        string
	ExpectedProvisionSHA256 string
	WhatIf                  bool
}

type CandidateVerificationProvisionCase struct {
	Role        string                       `json:"role"`
	CaseRoot    string                       `json:"caseRoot"`
	ProjectName string                       `json:"projectName"`
	Applied     bool                         `json:"applied"`
	Replay      bool                         `json:"replay"`
	DoctorRows  int                          `json:"doctorRows"`
	Writes      []syncpkg.ExclusiveInitWrite `json:"writes"`
}

type CandidateVerificationProvisionResult struct {
	SchemaVersion              int                                  `json:"schemaVersion"`
	Kind                       string                               `json:"kind"`
	Mode                       string                               `json:"mode"`
	RepoRoot                   string                               `json:"repoRoot"`
	SourceCaseRoot             string                               `json:"sourceCaseRoot"`
	Pack                       string                               `json:"pack"`
	PacketPath                 string                               `json:"packetPath"`
	PacketSHA256               string                               `json:"packetSha256"`
	DecisionPath               string                               `json:"decisionPath"`
	DecisionSHA256             string                               `json:"decisionSha256"`
	DecisionReceiptPath        string                               `json:"decisionReceiptPath"`
	DecisionReceiptSHA256      string                               `json:"decisionReceiptSha256"`
	ProvisionID                string                               `json:"provisionId"`
	ProvisionSHA256            string                               `json:"provisionSha256"`
	WorkspaceRoot              string                               `json:"workspaceRoot"`
	IntentPath                 string                               `json:"intentPath"`
	ReceiptPath                string                               `json:"receiptPath"`
	IsMutation                 bool                                 `json:"isMutation"`
	Applied                    bool                                 `json:"applied"`
	Replay                     bool                                 `json:"replay"`
	Cases                      []CandidateVerificationProvisionCase `json:"cases"`
	ApplyCommand               string                               `json:"applyCommand,omitempty"`
	VerificationPreviewCommand string                               `json:"verificationPreviewCommand"`
	NextSteps                  []string                             `json:"nextSteps"`
	Boundary                   []string                             `json:"boundary"`
}

type candidateVerificationProvisionReceipt struct {
	SchemaVersion              int                                  `json:"schemaVersion"`
	Kind                       string                               `json:"kind"`
	RepoRoot                   string                               `json:"repoRoot"`
	SourceCaseRoot             string                               `json:"sourceCaseRoot"`
	Pack                       string                               `json:"pack"`
	PacketPath                 string                               `json:"packetPath"`
	PacketSHA256               string                               `json:"packetSha256"`
	DecisionPath               string                               `json:"decisionPath"`
	DecisionSHA256             string                               `json:"decisionSha256"`
	DecisionReceiptPath        string                               `json:"decisionReceiptPath"`
	DecisionReceiptSHA256      string                               `json:"decisionReceiptSha256"`
	ProvisionID                string                               `json:"provisionId"`
	ProvisionSHA256            string                               `json:"provisionSha256"`
	WorkspaceRoot              string                               `json:"workspaceRoot"`
	IntentPath                 string                               `json:"intentPath"`
	FreshCaseRoot              string                               `json:"freshCaseRoot"`
	AttachedCaseRoot           string                               `json:"attachedCaseRoot"`
	VerificationPreviewCommand string                               `json:"verificationPreviewCommand"`
	Cases                      []CandidateVerificationProvisionCase `json:"cases"`
	Boundary                   []string                             `json:"boundary"`
}

type preparedCandidateVerificationProvision struct {
	result       CandidateVerificationProvisionResult
	freshPlan    syncpkg.ExclusiveInitPlan
	attachedPlan syncpkg.ExclusiveInitPlan
	intent       candidateVerificationProvisionReceipt
	intentBytes  []byte
	receipt      candidateVerificationProvisionReceipt
	receiptBytes []byte
	workspaceID  os.FileInfo
	workspace    *os.Root
}

var (
	candidateVerificationProvisionReceiptPostWriteHook func(string) error
	candidateVerificationProvisionStageHook            func(string) error
)

func ProvisionCandidateVerificationCases(repoRoot, sourceCaseRoot, pack string, opt CandidateVerificationProvisionOptions) (CandidateVerificationProvisionResult, error) {
	prepared, err := prepareCandidateVerificationProvision(repoRoot, sourceCaseRoot, pack, opt)
	if err != nil {
		return CandidateVerificationProvisionResult{}, err
	}
	if opt.WhatIf {
		prepared.result.Mode = "previewed"
		prepared.result.ApplyCommand = candidateVerificationProvisionCommand(prepared.result, true)
		prepared.result.NextSteps = []string{"inspect the exact case roots and no-overwrite write sets, then run the returned expected-hash Apply command"}
		return prepared.result, nil
	}
	expected := strings.TrimSpace(opt.ExpectedProvisionSHA256)
	decoded, decodeErr := hex.DecodeString(expected)
	if decodeErr != nil || len(decoded) != sha256.Size {
		return CandidateVerificationProvisionResult{}, fmt.Errorf("candidate verification provisioning Apply requires a valid -ExpectedProvisionSha256 from WhatIf")
	}
	unlock, err := acquireCandidateDecisionLock(filepath.Join(repoRoot, "packs", pack, "promote-candidates"))
	if err != nil {
		return CandidateVerificationProvisionResult{}, err
	}
	defer unlock()
	prepared, err = prepareCandidateVerificationProvision(repoRoot, sourceCaseRoot, pack, opt)
	if err != nil {
		return CandidateVerificationProvisionResult{}, err
	}
	defer prepared.closeWorkspace()
	if !strings.EqualFold(expected, prepared.result.ProvisionSHA256) {
		return CandidateVerificationProvisionResult{}, fmt.Errorf("candidate verification provisioning changed after preview")
	}
	if err := ensureCandidateVerificationProvisionWorkspace(&prepared); err != nil {
		return CandidateVerificationProvisionResult{}, err
	}
	prepared.receiptBytes, err = candidateVerificationProvisionReceiptBytes(prepared.receipt)
	if err != nil {
		return CandidateVerificationProvisionResult{}, err
	}
	if existing, ok, err := readCandidateVerificationProvisionReceipt(prepared.result.ReceiptPath); err != nil {
		return CandidateVerificationProvisionResult{}, err
	} else if ok {
		if !candidateVerificationProvisionReceiptMatches(existing, prepared.receipt) {
			return CandidateVerificationProvisionResult{}, fmt.Errorf("candidate verification provisioning receipt already exists with different bindings")
		}
		if err := revalidateCandidateVerificationWorkspace(prepared); err != nil {
			return CandidateVerificationProvisionResult{}, err
		}
		if _, err := verifyCandidateVerificationProvisionReceipt(prepared.workspace, prepared.receiptBytes, nil); err != nil {
			return CandidateVerificationProvisionResult{}, err
		}
		if err := reconcileCandidateVerificationReceiptTemp(prepared.workspace, candidateVerificationReceiptTempName(prepared.result.ProvisionID), "provision.receipt.json", prepared.receiptBytes); err != nil {
			return CandidateVerificationProvisionResult{}, err
		}
		if err := verifyProvisionedCandidateVerificationCases(repoRoot, pack, prepared.freshPlan, prepared.attachedPlan); err != nil {
			return CandidateVerificationProvisionResult{}, err
		}
		prepared.result.Mode = "already-provisioned"
		prepared.result.IsMutation = true
		prepared.result.Applied = true
		prepared.result.Replay = true
		for i := range prepared.result.Cases {
			prepared.result.Cases[i].Applied = true
			prepared.result.Cases[i].Replay = true
		}
		prepared.result.NextSteps = []string{"run the returned candidate decision verification WhatIf command"}
		return prepared.result, nil
	}
	if err := revalidateCandidateVerificationWorkspace(prepared); err != nil {
		return CandidateVerificationProvisionResult{}, err
	}
	batch, err := syncpkg.ReserveExclusiveInitBatch(prepared.freshPlan, prepared.attachedPlan)
	if err != nil {
		return CandidateVerificationProvisionResult{}, err
	}
	defer batch.Rollback()
	if err := revalidateCandidateVerificationWorkspace(prepared); err != nil {
		return CandidateVerificationProvisionResult{}, err
	}
	if err := batch.ValidateRoots(); err != nil {
		return CandidateVerificationProvisionResult{}, err
	}
	initialized, err := batch.Apply()
	if err != nil {
		return CandidateVerificationProvisionResult{}, err
	}
	fresh, attached := initialized[0], initialized[1]
	if err := batch.ValidateRoots(); err != nil {
		return CandidateVerificationProvisionResult{}, err
	}
	if err := revalidateCandidateVerificationWorkspace(prepared); err != nil {
		return CandidateVerificationProvisionResult{}, err
	}
	if candidateVerificationProvisionStageHook != nil {
		if err := candidateVerificationProvisionStageHook("before-doctor"); err != nil {
			return CandidateVerificationProvisionResult{}, err
		}
	}
	if err := batch.ValidateRoots(); err != nil {
		return CandidateVerificationProvisionResult{}, err
	}
	freshRows, err := doctor.Case(repoRoot, prepared.freshPlan.CaseRoot, pack)
	if err != nil {
		return CandidateVerificationProvisionResult{}, fmt.Errorf("fresh verification case doctor: %w", err)
	}
	if err := batch.ValidateRoots(); err != nil {
		return CandidateVerificationProvisionResult{}, err
	}
	attachedRows, err := doctor.Case(repoRoot, prepared.attachedPlan.CaseRoot, pack)
	if err != nil {
		return CandidateVerificationProvisionResult{}, fmt.Errorf("attached verification case doctor: %w", err)
	}
	if err := batch.ValidateRoots(); err != nil {
		return CandidateVerificationProvisionResult{}, err
	}
	prepared.result.IsMutation = true
	prepared.result.Applied = true
	prepared.result.Mode = "provisioned"
	prepared.result.Cases[0].Applied = fresh.Applied
	prepared.result.Cases[0].Replay = fresh.Replay
	prepared.result.Cases[0].DoctorRows = len(freshRows)
	prepared.result.Cases[1].Applied = attached.Applied
	prepared.result.Cases[1].Replay = attached.Replay
	prepared.result.Cases[1].DoctorRows = len(attachedRows)
	if err := revalidateCandidateVerificationWorkspace(prepared); err != nil {
		return CandidateVerificationProvisionResult{}, err
	}
	if err := batch.ValidateRoots(); err != nil {
		return CandidateVerificationProvisionResult{}, err
	}
	if candidateVerificationProvisionStageHook != nil {
		if err := candidateVerificationProvisionStageHook("before-receipt"); err != nil {
			return CandidateVerificationProvisionResult{}, err
		}
	}
	if err := batch.ValidateRoots(); err != nil {
		return CandidateVerificationProvisionResult{}, err
	}
	receiptID, err := writeCandidateVerificationProvisionReceipt(prepared.workspace, prepared.result.ProvisionID, prepared.receiptBytes)
	if err != nil {
		return CandidateVerificationProvisionResult{}, err
	}
	if candidateVerificationProvisionReceiptPostWriteHook != nil {
		if err := candidateVerificationProvisionReceiptPostWriteHook(prepared.result.ReceiptPath); err != nil {
			return CandidateVerificationProvisionResult{}, err
		}
	}
	if _, err := verifyCandidateVerificationProvisionReceipt(prepared.workspace, prepared.receiptBytes, receiptID); err != nil {
		return CandidateVerificationProvisionResult{}, err
	}
	if err := batch.ValidateRoots(); err != nil {
		return CandidateVerificationProvisionResult{}, err
	}
	if err := revalidateCandidateVerificationWorkspace(prepared); err != nil {
		return CandidateVerificationProvisionResult{}, err
	}
	prepared.result.NextSteps = []string{"run the returned candidate decision verification WhatIf command; provisioning does not write the final verification proof"}
	return prepared.result, nil
}

func prepareCandidateVerificationProvision(repoRoot, sourceCaseRoot, pack string, opt CandidateVerificationProvisionOptions) (preparedCandidateVerificationProvision, error) {
	authority, err := loadCandidateDecisionAuthority(repoRoot, sourceCaseRoot, pack, opt.PacketPath, opt.DecisionPath)
	if err != nil {
		return preparedCandidateVerificationProvision{}, err
	}
	inst := authority.instance
	packetPath := authority.packetPath
	decisionPath := authority.decisionPath
	packetHash := authority.packetHash
	decisionHash := authority.decisionHash
	receiptPath := authority.receiptPath
	receiptBytes := authority.receiptBytes
	receipt := authority.receipt
	if receipt.Accepted <= 0 || !receipt.VerificationPending {
		return preparedCandidateVerificationProvision{}, fmt.Errorf("candidate verification provisioning requires a current accepted decision receipt")
	}
	canonicalProofPath := candidateDecisionVerificationProofPath(authority.candidateRoot, packetHash, decisionHash)
	if fileSHA256(canonicalProofPath) != "" {
		return preparedCandidateVerificationProvision{}, fmt.Errorf("candidate decision verification is already complete")
	}
	provisionID := shortHash(packetHash + decisionHash)
	workspace := filepath.Join(inst.CaseRoot, ".rekit", "verifications", "candidate-decisions", provisionID)
	freshRoot, attachedRoot, err := validateCandidateVerificationProvisionRoots(workspace, opt.FreshCaseRoot, opt.AttachedCaseRoot)
	if err != nil {
		return preparedCandidateVerificationProvision{}, err
	}
	createdAt := time.Unix(0, 0).UTC()
	freshOpt := syncpkg.ExclusiveInitOptions{ProjectName: provisionID + "-fresh", ProvisionID: provisionID, Role: "fresh", CreatedAt: createdAt}
	attachedOpt := syncpkg.ExclusiveInitOptions{ProjectName: provisionID + "-attached", ProvisionID: provisionID, Role: "attached", CreatedAt: createdAt}
	freshPlan, freshErr := syncpkg.PlanExclusiveInit(repoRoot, freshRoot, pack, freshOpt)
	attachedPlan, attachedErr := syncpkg.PlanExclusiveInit(repoRoot, attachedRoot, pack, attachedOpt)
	verificationCommand := fmt.Sprintf("/rekit promote -PacketPath %s -CandidateDecisionPath %s -VerifyCandidateDecision -FreshCaseRoot %s -AttachedCaseRoot %s -WhatIf -Format json", quoteCandidateDecisionArg(packetPath), quoteCandidateDecisionArg(decisionPath), quoteCandidateDecisionArg(freshRoot), quoteCandidateDecisionArg(attachedRoot))
	intentPath := filepath.Join(workspace, "provision.intent.json")
	receiptOutPath := filepath.Join(workspace, "provision.receipt.json")
	intent, intentBytes, intentOK, intentErr := readCandidateVerificationProvisionIntent(intentPath)
	if intentErr != nil {
		return preparedCandidateVerificationProvision{}, intentErr
	}
	if freshErr != nil || attachedErr != nil {
		if !intentOK {
			if freshErr != nil {
				return preparedCandidateVerificationProvision{}, freshErr
			}
			return preparedCandidateVerificationProvision{}, attachedErr
		}
		freshPlan, err = syncpkg.PlanExclusiveInitReplay(repoRoot, freshRoot, pack, freshOpt)
		if err != nil {
			return preparedCandidateVerificationProvision{}, err
		}
		attachedPlan, err = syncpkg.PlanExclusiveInitReplay(repoRoot, attachedRoot, pack, attachedOpt)
		if err != nil {
			return preparedCandidateVerificationProvision{}, err
		}
	}
	cases := []CandidateVerificationProvisionCase{{Role: "fresh", CaseRoot: freshRoot, ProjectName: freshPlan.ProjectName, Writes: freshPlan.Writes}, {Role: "attached", CaseRoot: attachedRoot, ProjectName: attachedPlan.ProjectName, Writes: attachedPlan.Writes}}
	result := CandidateVerificationProvisionResult{SchemaVersion: 1, Kind: "pack-memory-candidate-verification-case-provision", RepoRoot: repoRoot, SourceCaseRoot: inst.CaseRoot, Pack: pack, PacketPath: packetPath, PacketSHA256: packetHash, DecisionPath: decisionPath, DecisionSHA256: decisionHash, DecisionReceiptPath: receiptPath, DecisionReceiptSHA256: sha256Hex(receiptBytes), ProvisionID: provisionID, WorkspaceRoot: workspace, IntentPath: intentPath, ReceiptPath: receiptOutPath, Cases: cases, VerificationPreviewCommand: verificationCommand, Boundary: []string{"provisioning creates only two source-case-local verification cases with exclusive writes", "provisioning does not run final verification, write authority/confirmed, or execute heavy tools", "verification cases are not deleted automatically"}}
	result.ProvisionSHA256 = candidateVerificationProvisionHash(result)
	receiptOut := candidateVerificationProvisionReceipt{SchemaVersion: 1, Kind: result.Kind + "-receipt", RepoRoot: repoRoot, SourceCaseRoot: inst.CaseRoot, Pack: pack, PacketPath: packetPath, PacketSHA256: packetHash, DecisionPath: decisionPath, DecisionSHA256: decisionHash, DecisionReceiptPath: receiptPath, DecisionReceiptSHA256: result.DecisionReceiptSHA256, ProvisionID: provisionID, ProvisionSHA256: result.ProvisionSHA256, WorkspaceRoot: workspace, IntentPath: result.IntentPath, FreshCaseRoot: freshRoot, AttachedCaseRoot: attachedRoot, VerificationPreviewCommand: verificationCommand, Cases: append([]CandidateVerificationProvisionCase(nil), cases...), Boundary: append([]string(nil), result.Boundary...)}
	expectedIntent := receiptOut
	expectedIntent.Kind = result.Kind + "-intent"
	expectedIntentBytes, err := json.MarshalIndent(expectedIntent, "", "  ")
	if err != nil {
		return preparedCandidateVerificationProvision{}, err
	}
	expectedIntentBytes = append(expectedIntentBytes, '\n')
	if intentOK && (!candidateVerificationProvisionReceiptMatches(intent, expectedIntent) || !bytes.Equal(intentBytes, expectedIntentBytes)) {
		return preparedCandidateVerificationProvision{}, fmt.Errorf("candidate verification provisioning intent already exists with different bindings")
	}
	return preparedCandidateVerificationProvision{result: result, freshPlan: freshPlan, attachedPlan: attachedPlan, intent: expectedIntent, intentBytes: expectedIntentBytes, receipt: receiptOut}, nil
}

func validateCandidateVerificationProvisionRoots(workspace, fresh, attached string) (string, string, error) {
	fresh, err := filepath.Abs(strings.TrimSpace(fresh))
	if err != nil || strings.TrimSpace(fresh) == "" {
		return "", "", fmt.Errorf("candidate verification provisioning requires -FreshCaseRoot")
	}
	attached, err = filepath.Abs(strings.TrimSpace(attached))
	if err != nil || strings.TrimSpace(attached) == "" {
		return "", "", fmt.Errorf("candidate verification provisioning requires -AttachedCaseRoot")
	}
	if sameCandidateDecisionPath(fresh, attached) {
		return "", "", fmt.Errorf("candidate verification provisioning requires distinct case roots")
	}
	for role, root := range map[string]string{"fresh": fresh, "attached": attached} {
		if !sameCandidateDecisionPath(filepath.Dir(root), workspace) || filepath.Base(root) == "." || filepath.Base(root) == ".." {
			return "", "", fmt.Errorf("%s verification case root must be a direct child of %s", role, workspace)
		}
	}
	return fresh, attached, nil
}

func candidateVerificationProvisionHash(result CandidateVerificationProvisionResult) string {
	copy := result
	copy.ProvisionSHA256 = ""
	copy.ApplyCommand = ""
	copy.NextSteps = nil
	data, _ := json.Marshal(copy)
	return sha256Hex(data)
}

func candidateVerificationProvisionCommand(result CandidateVerificationProvisionResult, apply bool) string {
	command := fmt.Sprintf("/rekit promote -PacketPath %s -CandidateDecisionPath %s -ProvisionCandidateVerificationCases -FreshCaseRoot %s -AttachedCaseRoot %s", quoteCandidateDecisionArg(result.PacketPath), quoteCandidateDecisionArg(result.DecisionPath), quoteCandidateDecisionArg(result.Cases[0].CaseRoot), quoteCandidateDecisionArg(result.Cases[1].CaseRoot))
	if apply {
		command += " -ExpectedProvisionSha256 " + result.ProvisionSHA256 + " -Apply"
	} else {
		command += " -WhatIf"
	}
	return command + " -Format json"
}

func ensureCandidateVerificationProvisionWorkspace(prepared *preparedCandidateVerificationProvision) error {
	workspace := prepared.result.WorkspaceRoot
	if err := createDirectoryChainNoFollow(prepared.result.SourceCaseRoot, workspace); err != nil {
		return err
	}
	info, err := os.Lstat(workspace)
	if err != nil {
		return err
	}
	if info.Mode()&os.ModeSymlink != 0 || !info.IsDir() {
		return fmt.Errorf("candidate verification provisioning workspace must be a non-symlink directory: %s", workspace)
	}
	prepared.workspaceID = info
	workspaceRoot, err := os.OpenRoot(workspace)
	if err != nil {
		return err
	}
	openedInfo, err := workspaceRoot.Stat(".")
	if err != nil || !os.SameFile(info, openedInfo) {
		_ = workspaceRoot.Close()
		if err != nil {
			return err
		}
		return fmt.Errorf("candidate verification provisioning workspace changed while opening")
	}
	prepared.workspace = workspaceRoot
	if existing, readErr := prepared.workspace.ReadFile("provision.intent.json"); readErr == nil {
		if !bytes.Equal(existing, prepared.intentBytes) {
			return fmt.Errorf("candidate verification provisioning intent already exists with different bindings")
		}
		return nil
	} else if !os.IsNotExist(readErr) {
		return readErr
	}
	return writeWorkspaceExclusiveFile(prepared.workspace, "provision.intent.json", prepared.intentBytes)
}

func (prepared *preparedCandidateVerificationProvision) closeWorkspace() {
	if prepared != nil && prepared.workspace != nil {
		_ = prepared.workspace.Close()
		prepared.workspace = nil
	}
}

func createDirectoryChainNoFollow(root, target string) error {
	root, err := filepath.Abs(root)
	if err != nil {
		return err
	}
	target, err = filepath.Abs(target)
	if err != nil {
		return err
	}
	rel, err := filepath.Rel(root, target)
	if err != nil || rel == ".." || strings.HasPrefix(rel, ".."+string(filepath.Separator)) || filepath.IsAbs(rel) {
		return fmt.Errorf("candidate verification workspace escapes source case: %s", target)
	}
	current := root
	rootInfo, err := os.Lstat(current)
	if err != nil {
		return err
	}
	if rootInfo.Mode()&os.ModeSymlink != 0 || !rootInfo.IsDir() {
		return fmt.Errorf("candidate verification workspace ancestor must be a non-symlink directory: %s", current)
	}
	for part := range strings.SplitSeq(rel, string(filepath.Separator)) {
		if part == "" || part == "." {
			continue
		}
		current = filepath.Join(current, part)
		info, err := os.Lstat(current)
		if os.IsNotExist(err) {
			if err := os.Mkdir(current, 0o755); err != nil && !os.IsExist(err) {
				return err
			}
			info, err = os.Lstat(current)
		}
		if err != nil {
			return err
		}
		if info.Mode()&os.ModeSymlink != 0 || !info.IsDir() {
			return fmt.Errorf("candidate verification workspace ancestor must be a non-symlink directory: %s", current)
		}
	}
	return nil
}

func writeWorkspaceExclusiveFile(root *os.Root, name string, data []byte) error {
	file, err := root.OpenFile(name, os.O_WRONLY|os.O_CREATE|os.O_EXCL, 0o644)
	if err != nil {
		return err
	}
	written, writeErr := file.Write(data)
	if writeErr == nil && written != len(data) {
		writeErr = io.ErrShortWrite
	}
	if writeErr == nil {
		writeErr = file.Sync()
	}
	if closeErr := file.Close(); writeErr == nil {
		writeErr = closeErr
	}
	if writeErr != nil {
		_ = root.Remove(name)
	}
	return writeErr
}

func writeCandidateVerificationProvisionReceipt(root *os.Root, provisionID string, data []byte) (os.FileInfo, error) {
	if root == nil {
		return nil, fmt.Errorf("candidate verification provisioning workspace handle is closed")
	}
	name := "provision.receipt.json"
	tempName := candidateVerificationReceiptTempName(provisionID)
	if err := reconcileCandidateVerificationReceiptTemp(root, tempName, name, data); err != nil {
		return nil, err
	}
	if _, err := root.Lstat(name); err == nil {
		return verifyCandidateVerificationProvisionReceipt(root, data, nil)
	} else if !os.IsNotExist(err) {
		return nil, err
	}
	if _, err := root.Lstat(tempName); os.IsNotExist(err) {
		if err := writeWorkspaceExclusiveFile(root, tempName, data); err != nil {
			return nil, err
		}
	} else if err != nil {
		return nil, err
	}
	if candidateVerificationProvisionStageHook != nil {
		if err := candidateVerificationProvisionStageHook("before-receipt-publish"); err != nil {
			return nil, err
		}
	}
	if err := root.Link(tempName, name); err != nil {
		return nil, fmt.Errorf("publish candidate verification provisioning receipt without replacement: %w", err)
	}
	identity, err := verifyCandidateVerificationProvisionReceipt(root, data, nil)
	if err != nil {
		removeCandidateVerificationReceiptLink(root, tempName, name)
		return nil, err
	}
	if candidateVerificationProvisionStageHook != nil {
		if err := candidateVerificationProvisionStageHook("after-receipt-publish-before-temp-remove"); err != nil {
			return nil, err
		}
	}
	if err := removeCandidateVerificationReceiptTempForFinal(root, tempName, name, data); err != nil {
		return nil, err
	}
	return identity, nil
}

func candidateVerificationReceiptTempName(provisionID string) string {
	return ".provision.receipt." + provisionID + ".tmp"
}

func candidateVerificationProvisionReceiptBytes(receipt candidateVerificationProvisionReceipt) ([]byte, error) {
	data, err := json.MarshalIndent(receipt, "", "  ")
	if err != nil {
		return nil, err
	}
	return append(data, '\n'), nil
}

func reconcileCandidateVerificationReceiptTemp(root *os.Root, tempName, name string, expected []byte) error {
	info, err := root.Lstat(tempName)
	if os.IsNotExist(err) {
		return nil
	}
	if err != nil {
		return err
	}
	if !info.Mode().IsRegular() {
		return fmt.Errorf("candidate verification provisioning owned receipt temp is not regular")
	}
	if _, verifyErr := verifyCandidateVerificationProvisionNamedFile(root, tempName, expected, nil); verifyErr != nil {
		return fmt.Errorf("candidate verification provisioning owned receipt temp has different bytes")
	}
	if _, err := root.Lstat(name); os.IsNotExist(err) {
		return nil
	} else if err != nil {
		return err
	}
	return removeCandidateVerificationReceiptTempForFinal(root, tempName, name, expected)
}

func removeCandidateVerificationReceiptTempForFinal(root *os.Root, tempName, name string, expected []byte) error {
	tempInfo, err := verifyCandidateVerificationProvisionNamedFile(root, tempName, expected, nil)
	if err != nil {
		return err
	}
	finalInfo, err := verifyCandidateVerificationProvisionNamedFile(root, name, expected, nil)
	if err != nil {
		return err
	}
	if !os.SameFile(tempInfo, finalInfo) {
		return fmt.Errorf("candidate verification provisioning owned receipt temp and final have different identities")
	}
	if err := root.Remove(tempName); err != nil && !os.IsNotExist(err) {
		return err
	}
	return nil
}

func verifyCandidateVerificationProvisionReceipt(root *os.Root, expected []byte, identity os.FileInfo) (os.FileInfo, error) {
	return verifyCandidateVerificationProvisionNamedFile(root, "provision.receipt.json", expected, identity)
}

func verifyCandidateVerificationProvisionNamedFile(root *os.Root, name string, expected []byte, identity os.FileInfo) (os.FileInfo, error) {
	before, err := root.Lstat(name)
	if err != nil || !before.Mode().IsRegular() || (identity != nil && !os.SameFile(identity, before)) {
		return nil, fmt.Errorf("candidate verification provisioning receipt changed after creation: %s", name)
	}
	file, err := root.Open(name)
	if err != nil {
		return nil, err
	}
	opened, statErr := file.Stat()
	data, readErr := io.ReadAll(io.LimitReader(file, int64(len(expected))+1))
	closeErr := file.Close()
	if statErr != nil {
		return nil, statErr
	}
	if readErr != nil {
		return nil, readErr
	}
	if closeErr != nil {
		return nil, closeErr
	}
	after, err := root.Lstat(name)
	if err != nil || !opened.Mode().IsRegular() || !os.SameFile(before, opened) || !os.SameFile(opened, after) || !bytes.Equal(data, expected) {
		return nil, fmt.Errorf("candidate verification provisioning receipt changed after creation: %s", name)
	}
	return opened, nil
}

func removeCandidateVerificationReceiptLink(root *os.Root, tempName, name string) {
	tempInfo, tempErr := root.Lstat(tempName)
	finalInfo, finalErr := root.Lstat(name)
	if tempErr == nil && finalErr == nil && os.SameFile(tempInfo, finalInfo) {
		_ = root.Remove(name)
	}
}

func revalidateCandidateVerificationWorkspace(prepared preparedCandidateVerificationProvision) error {
	if err := rejectCandidateDecisionSymlinkPath(prepared.result.SourceCaseRoot, prepared.result.WorkspaceRoot, false); err != nil {
		return err
	}
	info, err := os.Lstat(prepared.result.WorkspaceRoot)
	if err != nil {
		return err
	}
	if prepared.workspaceID != nil && !os.SameFile(prepared.workspaceID, info) {
		return fmt.Errorf("candidate verification provisioning workspace was rebound")
	}
	if prepared.workspace == nil {
		return fmt.Errorf("candidate verification provisioning workspace handle is closed")
	}
	openedInfo, err := prepared.workspace.Stat(".")
	if err != nil || !os.SameFile(prepared.workspaceID, openedInfo) {
		return fmt.Errorf("candidate verification provisioning workspace handle identity changed")
	}
	intentBytes, err := prepared.workspace.ReadFile("provision.intent.json")
	if err != nil || !bytes.Equal(intentBytes, prepared.intentBytes) {
		return fmt.Errorf("candidate verification provisioning intent changed after preparation")
	}
	return nil
}

func candidateVerificationProvisionReceiptMatches(left, right candidateVerificationProvisionReceipt) bool {
	if left.SchemaVersion != right.SchemaVersion || left.Kind != right.Kind || left.Pack != right.Pack || left.PacketSHA256 != right.PacketSHA256 || left.DecisionSHA256 != right.DecisionSHA256 || left.DecisionReceiptSHA256 != right.DecisionReceiptSHA256 || left.ProvisionID != right.ProvisionID || left.ProvisionSHA256 != right.ProvisionSHA256 || left.VerificationPreviewCommand != right.VerificationPreviewCommand {
		return false
	}
	for _, pair := range [][2]string{{left.RepoRoot, right.RepoRoot}, {left.SourceCaseRoot, right.SourceCaseRoot}, {left.PacketPath, right.PacketPath}, {left.DecisionPath, right.DecisionPath}, {left.DecisionReceiptPath, right.DecisionReceiptPath}, {left.WorkspaceRoot, right.WorkspaceRoot}, {left.IntentPath, right.IntentPath}, {left.FreshCaseRoot, right.FreshCaseRoot}, {left.AttachedCaseRoot, right.AttachedCaseRoot}} {
		if !sameCandidateDecisionPath(pair[0], pair[1]) {
			return false
		}
	}
	if !reflect.DeepEqual(left.Boundary, right.Boundary) || len(left.Cases) != len(right.Cases) {
		return false
	}
	for i := range left.Cases {
		leftCase, rightCase := left.Cases[i], right.Cases[i]
		if leftCase.Role != rightCase.Role || leftCase.ProjectName != rightCase.ProjectName || !sameCandidateDecisionPath(leftCase.CaseRoot, rightCase.CaseRoot) || !reflect.DeepEqual(leftCase.Writes, rightCase.Writes) {
			return false
		}
	}
	return true
}

func readCandidateVerificationProvisionIntent(path string) (candidateVerificationProvisionReceipt, []byte, bool, error) {
	info, err := os.Lstat(path)
	if os.IsNotExist(err) {
		return candidateVerificationProvisionReceipt{}, nil, false, nil
	}
	if err != nil {
		return candidateVerificationProvisionReceipt{}, nil, false, err
	}
	if info.Mode()&os.ModeSymlink != 0 || !info.Mode().IsRegular() || info.Size() <= 0 || info.Size() > maxCandidateDecisionBytes {
		return candidateVerificationProvisionReceipt{}, nil, false, fmt.Errorf("candidate verification provisioning artifact must be a bounded non-symlink regular file: %s", path)
	}
	if err := rejectCandidateDecisionSymlinkPath(filepath.Dir(path), path, false); err != nil {
		return candidateVerificationProvisionReceipt{}, nil, false, err
	}
	file, err := os.Open(path)
	if err != nil {
		return candidateVerificationProvisionReceipt{}, nil, false, err
	}
	openedInfo, statErr := file.Stat()
	data, readErr := io.ReadAll(io.LimitReader(file, maxCandidateDecisionBytes+1))
	closeErr := file.Close()
	if statErr != nil {
		return candidateVerificationProvisionReceipt{}, nil, false, statErr
	}
	if readErr != nil {
		return candidateVerificationProvisionReceipt{}, nil, false, readErr
	}
	if closeErr != nil {
		return candidateVerificationProvisionReceipt{}, nil, false, closeErr
	}
	currentInfo, err := os.Lstat(path)
	if err != nil || currentInfo.Mode()&os.ModeSymlink != 0 || !os.SameFile(info, openedInfo) || !os.SameFile(openedInfo, currentInfo) || len(data) > maxCandidateDecisionBytes {
		return candidateVerificationProvisionReceipt{}, nil, false, fmt.Errorf("candidate verification provisioning artifact changed while reading: %s", path)
	}
	var intent candidateVerificationProvisionReceipt
	if err := decodeStrictJSON(data, &intent); err != nil {
		return candidateVerificationProvisionReceipt{}, nil, false, err
	}
	return intent, data, true, nil
}

func readCandidateVerificationProvisionReceipt(path string) (candidateVerificationProvisionReceipt, bool, error) {
	receipt, _, ok, err := readCandidateVerificationProvisionIntent(path)
	return receipt, ok, err
}

func verifyProvisionedCandidateVerificationCases(repoRoot, pack string, plans ...syncpkg.ExclusiveInitPlan) error {
	for _, plan := range plans {
		if _, err := syncpkg.ApplyExclusiveInit(plan); err != nil {
			return err
		}
		if _, err := doctor.Case(repoRoot, plan.CaseRoot, pack); err != nil {
			return err
		}
	}
	return nil
}
