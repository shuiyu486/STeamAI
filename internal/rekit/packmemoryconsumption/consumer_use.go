package packmemoryconsumption

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"path/filepath"
	"strings"

	refsf "github.com/shuiyu486/re-context-kits/internal/rekit/fs"
	"github.com/shuiyu486/re-context-kits/internal/rekit/memberexecution"
	"github.com/shuiyu486/re-context-kits/internal/rekit/releasecheck"
)

const (
	KindConsumerUseStatement = "pack-memory-consumer-use-statement"
	KindConsumerUseProof     = "pack-memory-consumer-use-proof"
	maxConsumerUseBytes      = 1 << 20
)

type ConsumerUseOptions struct {
	ChangeID   string
	Lane       string
	AttemptID  string
	OutputPath string
}

type ConsumerUseStatement struct {
	SchemaVersion int    `json:"schemaVersion"`
	Kind          string `json:"kind"`
	ChangeID      string `json:"changeId"`
	SourceSHA256  string `json:"sourceSha256"`
	Quote         string `json:"quote"`
	AppliedAs     string `json:"appliedAs"`
	NoAuthority   bool   `json:"noAuthority"`
	NoConfirmed   bool   `json:"noConfirmed"`
	NoHeavyTool   bool   `json:"noHeavyTool"`
}

type ConsumerUseProof struct {
	SchemaVersion            int                                    `json:"schemaVersion"`
	Kind                     string                                 `json:"kind"`
	RepoRoot                 string                                 `json:"repoRoot"`
	CaseRoot                 string                                 `json:"caseRoot"`
	Pack                     string                                 `json:"pack"`
	ChangeID                 string                                 `json:"changeId"`
	AuthoritySHA256          string                                 `json:"authoritySha256"`
	ManagedPath              string                                 `json:"managedPath"`
	SourcePath               string                                 `json:"sourcePath"`
	SourceSHA256             string                                 `json:"sourceSha256"`
	ConsumptionReceiptPath   string                                 `json:"consumptionReceiptPath"`
	ConsumptionReceiptSHA256 string                                 `json:"consumptionReceiptSha256"`
	ConsumptionPlanSHA256    string                                 `json:"consumptionPlanSha256"`
	Lane                     string                                 `json:"lane"`
	AttemptID                string                                 `json:"attemptId"`
	Owner                    memberexecution.Owner                  `json:"owner"`
	TaskContextPath          string                                 `json:"taskContextPath"`
	TaskContextSHA256        string                                 `json:"taskContextSha256"`
	ManifestPath             string                                 `json:"manifestPath"`
	ManifestSHA256           string                                 `json:"manifestSha256"`
	OutputPath               string                                 `json:"outputPath"`
	OutputSHA256             string                                 `json:"outputSha256"`
	OutputBytes              int64                                  `json:"outputBytes"`
	Quote                    string                                 `json:"quote"`
	QuoteSHA256              string                                 `json:"quoteSha256"`
	AppliedAs                string                                 `json:"appliedAs"`
	ProofSHA256              string                                 `json:"proofSha256"`
	Verified                 bool                                   `json:"verified"`
	ReadOnly                 bool                                   `json:"readOnly"`
	NoAuthority              bool                                   `json:"noAuthority"`
	NoConfirmed              bool                                   `json:"noConfirmed"`
	NoHeavyTool              bool                                   `json:"noHeavyTool"`
	Boundary                 []string                               `json:"boundary"`
	Authority                releasecheck.CompletedPackMemoryChange `json:"authority"`
}

type currentConsumptionReceipt struct {
	repo                string
	caseRoot            string
	change              releasecheck.CompletedPackMemoryChange
	receipt             Receipt
	receiptPath         string
	receiptBytes        []byte
	acceptedDeltaChunks [][]byte
}

var currentConsumerBeforeFinalValidationHook func() error

func VerifyConsumerUse(repoRoot, caseRoot, pack string, opt ConsumerUseOptions) (ConsumerUseProof, error) {
	current, err := inspectCurrentConsumptionReceipt(repoRoot, caseRoot, pack, opt.ChangeID)
	if err != nil {
		return ConsumerUseProof{}, err
	}
	lane := strings.TrimSpace(opt.Lane)
	attemptID := strings.TrimSpace(opt.AttemptID)
	outputPath := filepath.ToSlash(strings.TrimSpace(opt.OutputPath))
	if lane == "" || attemptID == "" || outputPath == "" {
		return ConsumerUseProof{}, fmt.Errorf("pack-memory consumer-use verification requires lane, attemptId, and outputPath")
	}
	inspection, err := memberexecution.Inspect(current.caseRoot, lane, attemptID)
	if err != nil {
		return ConsumerUseProof{}, err
	}
	if inspection.State != "intake-ready" || inspection.Intent == nil || inspection.TaskContext == nil || inspection.Manifest == nil || inspection.AttemptID != attemptID || inspection.Owner.Lane != lane || !strings.EqualFold(inspection.Intent.Pack, pack) {
		return ConsumerUseProof{}, fmt.Errorf("pack-memory consumer-use proof requires a strict intake-ready consumer result")
	}
	if err := memberexecution.ValidateActionableTaskContext(current.caseRoot, inspection); err != nil {
		return ConsumerUseProof{}, err
	}
	if err := validateConsumerTaskBinding(*inspection.TaskContext, current); err != nil {
		return ConsumerUseProof{}, err
	}
	matches, err := memberexecution.CurrentOwnerMatches(current.caseRoot, pack, inspection.Owner)
	if err != nil {
		return ConsumerUseProof{}, err
	}
	if !matches {
		return ConsumerUseProof{}, fmt.Errorf("pack-memory consumer owner generation is stale")
	}
	output, ok := consumerUseOutput(inspection.Manifest.Outputs, outputPath)
	if !ok {
		return ConsumerUseProof{}, fmt.Errorf("pack-memory consumer-use output is not declared by the strict result manifest: %s", outputPath)
	}
	fullOutput, err := refsf.SafeJoin(inspection.OutputsRoot, output.Path)
	if err != nil {
		return ConsumerUseProof{}, err
	}
	outputBytes, err := refsf.ReadStableRegularFileAnchored(current.caseRoot, fullOutput, "pack-memory consumer-use output", maxConsumerUseBytes)
	if err != nil || int64(len(outputBytes)) != output.Bytes || !strings.EqualFold(sha256Hex(outputBytes), output.SHA256) {
		return ConsumerUseProof{}, fmt.Errorf("pack-memory consumer-use output does not match the strict result manifest: %w", err)
	}
	statement, err := decodeConsumerUseStatement(outputBytes)
	if err != nil {
		return ConsumerUseProof{}, err
	}
	if statement.ChangeID != current.change.ChangeID || !strings.EqualFold(statement.SourceSHA256, current.change.SourceSHA256) {
		return ConsumerUseProof{}, fmt.Errorf("pack-memory consumer-use statement does not match the selected sync change")
	}
	if len([]byte(statement.Quote)) < 8 || !consumerQuoteInAcceptedDelta(current.acceptedDeltaChunks, []byte(statement.Quote)) {
		return ConsumerUseProof{}, fmt.Errorf("pack-memory consumer-use quote is not a bounded exact accepted-delta excerpt")
	}
	if strings.TrimSpace(statement.AppliedAs) == "" || len([]byte(statement.AppliedAs)) > 4096 {
		return ConsumerUseProof{}, fmt.Errorf("pack-memory consumer-use appliedAs must be bounded and non-empty")
	}
	proof := ConsumerUseProof{
		SchemaVersion: 1, Kind: KindConsumerUseProof, RepoRoot: current.repo, CaseRoot: current.caseRoot, Pack: pack,
		ChangeID: current.change.ChangeID, AuthoritySHA256: current.change.AuthoritySHA256, ManagedPath: current.change.ManagedPath,
		SourcePath: current.change.SourcePath, SourceSHA256: current.change.SourceSHA256, ConsumptionReceiptPath: current.receiptPath,
		ConsumptionReceiptSHA256: sha256Hex(current.receiptBytes), ConsumptionPlanSHA256: current.receipt.PlanSHA256,
		Lane: lane, AttemptID: attemptID, Owner: inspection.Owner, TaskContextPath: inspection.TaskContextPath, TaskContextSHA256: inspection.TaskContextSHA256,
		ManifestPath: inspection.ManifestPath, ManifestSHA256: inspection.ManifestSHA256, OutputPath: output.Path, OutputSHA256: output.SHA256, OutputBytes: output.Bytes,
		Quote: statement.Quote, QuoteSHA256: sha256Hex([]byte(statement.Quote)), AppliedAs: statement.AppliedAs,
		Verified: true, ReadOnly: true, NoAuthority: true, NoConfirmed: true, NoHeavyTool: true, Authority: current.change,
		Boundary: []string{
			"selected sync receipt, current managed target, sync state, completed-change authority, and source bytes were revalidated",
			"consumer bytes came from one strict intake-ready current-owner ResultManifest output",
			"quote is an exact accepted-delta excerpt and appliedAs is consumer-authored usage context",
			"verification is read-only and does not grant authority, confirmed, or execute heavy tools",
		},
	}
	proof.ProofSHA256, err = consumerUseProofSHA256(proof)
	if err != nil {
		return ConsumerUseProof{}, err
	}
	return proof, nil
}

func BindConsumerTask(caseRoot, lane, changeID string) (string, string, error) {
	path, err := consumptionReceiptPath(caseRoot, changeID)
	if err != nil {
		return "", "", err
	}
	receiptBytes, err := refsf.ReadStableRegularFileAnchored(caseRoot, path, "pack-memory consumer task receipt", maxConsumerUseBytes)
	if err != nil {
		return "", "", err
	}
	var receipt Receipt
	if err := decodeStrictJSON(receiptBytes, &receipt); err != nil || receipt.ChangeID != changeID || receipt.SchemaVersion != SchemaVersion || receipt.Kind != KindReceipt {
		return "", "", fmt.Errorf("pack-memory consumer task receipt is invalid: %w", err)
	}
	return memberexecution.WriteTaskBinding(caseRoot, lane, memberexecution.TaskBinding{
		Kind: "pack-memory-consumer",
		Values: map[string]string{
			"changeId":      receipt.ChangeID,
			"sourceSha256":  receipt.SourceSHA256,
			"receiptSha256": sha256Hex(receiptBytes),
			"planSha256":    receipt.PlanSHA256,
		},
	})
}

func ValidateCurrentConsumerTask(repoRoot, caseRoot, pack, lane string) error {
	binding, err := memberexecution.CurrentTaskBinding(caseRoot, lane)
	if err != nil {
		return err
	}
	if binding == nil || binding.Kind != "pack-memory-consumer" {
		return fmt.Errorf("current member task is not a pack-memory consumer")
	}
	changeID := strings.TrimSpace(binding.Values["changeId"])
	current, err := inspectCurrentConsumptionReceipt(repoRoot, caseRoot, pack, changeID)
	if err != nil {
		return err
	}
	return validateConsumerTaskBinding(memberexecution.TaskContext{Binding: binding}, current)
}

func sameConsumerTaskBinding(left, right *memberexecution.TaskBinding) bool {
	if left == nil || right == nil || left.Kind != right.Kind || len(left.Values) != len(right.Values) {
		return false
	}
	for key, value := range left.Values {
		if !strings.EqualFold(strings.TrimSpace(value), strings.TrimSpace(right.Values[key])) {
			return false
		}
	}
	return true
}

func WithCurrentConsumerAttemptLease(repoRoot, caseRoot, pack string, inspection memberexecution.Inspection, apply func() error) error {
	if inspection.TaskContext == nil || inspection.TaskContext.Binding == nil || inspection.TaskContext.Binding.Kind != "pack-memory-consumer" {
		return apply()
	}
	lane := strings.TrimSpace(inspection.TaskContext.Owner.Lane)
	return WithCurrentConsumerTaskLease(repoRoot, caseRoot, pack, lane, func() error {
		binding, err := memberexecution.CurrentTaskBinding(caseRoot, lane)
		if err != nil {
			return err
		}
		if !sameConsumerTaskBinding(inspection.TaskContext.Binding, binding) {
			return fmt.Errorf("pack-memory consumer attempt binding changed before launch")
		}
		if err := memberexecution.ValidateActionableTaskContext(caseRoot, inspection); err != nil {
			return err
		}
		current, err := memberexecution.Inspect(caseRoot, lane, inspection.AttemptID)
		if err != nil {
			return err
		}
		if current.TaskContext == nil || current.TaskContextSHA256 != inspection.TaskContextSHA256 {
			return fmt.Errorf("pack-memory consumer attempt task context changed before launch")
		}
		return apply()
	})
}

func WithCurrentConsumerTaskLease(repoRoot, caseRoot, pack, lane string, apply func() error) (retErr error) {
	repoLease, err := acquireMutationLease(repoRoot)
	if err != nil {
		return err
	}
	defer func() { retErr = errors.Join(retErr, repoLease.Unlock()) }()
	caseLease, err := acquireMutationLease(caseRoot)
	if err != nil {
		return err
	}
	defer func() { retErr = errors.Join(retErr, caseLease.Unlock()) }()
	if currentConsumerBeforeFinalValidationHook != nil {
		if err := currentConsumerBeforeFinalValidationHook(); err != nil {
			return err
		}
	}
	if err := ValidateCurrentConsumerTask(repoRoot, caseRoot, pack, lane); err != nil {
		return err
	}
	return apply()
}

func validateConsumerTaskBinding(task memberexecution.TaskContext, current currentConsumptionReceipt) error {
	if task.Binding == nil || task.Binding.Kind != "pack-memory-consumer" {
		return fmt.Errorf("pack-memory consumer attempt is not bound to the selected sync task")
	}
	expected := map[string]string{
		"changeId":      current.change.ChangeID,
		"sourceSha256":  current.change.SourceSHA256,
		"receiptSha256": sha256Hex(current.receiptBytes),
		"planSha256":    current.receipt.PlanSHA256,
	}
	if len(task.Binding.Values) != len(expected) {
		return fmt.Errorf("pack-memory consumer task binding does not match the selected sync receipt")
	}
	for key, value := range expected {
		if !strings.EqualFold(strings.TrimSpace(task.Binding.Values[key]), value) {
			return fmt.Errorf("pack-memory consumer task binding changed: %s", key)
		}
	}
	return nil
}

func inspectCurrentConsumptionReceipt(repoRoot, caseRoot, pack, changeID string) (currentConsumptionReceipt, error) {
	if err := validateArtifactID(changeID); err != nil {
		return currentConsumptionReceipt{}, err
	}
	repo, caseFull, _, state, catalog, err := prepare(repoRoot, caseRoot, pack)
	if err != nil {
		return currentConsumptionReceipt{}, err
	}
	change, ok := selectedChange(catalog, changeID)
	if !ok {
		return currentConsumptionReceipt{}, fmt.Errorf("completed pack-memory change not found: %s", changeID)
	}
	status, err := inspectChange(repo, caseFull, pack, state, change)
	if err != nil {
		return currentConsumptionReceipt{}, err
	}
	if status.State != "already-consumed" {
		return currentConsumptionReceipt{}, fmt.Errorf("pack-memory change is not currently backed by a valid selected sync receipt: %s", status.State)
	}
	receiptBytes, err := refsf.ReadStableRegularFileAnchored(caseFull, status.ReceiptPath, "current pack-memory consumption receipt", maxConsumerUseBytes)
	if err != nil {
		return currentConsumptionReceipt{}, fmt.Errorf("read current pack-memory consumption receipt: %w", err)
	}
	var receipt Receipt
	if err := decodeStrictJSON(receiptBytes, &receipt); err != nil || receipt.SchemaVersion != SchemaVersion || receipt.Kind != KindReceipt || !validSHA256(receipt.PlanSHA256) {
		return currentConsumptionReceipt{}, fmt.Errorf("decode current pack-memory consumption receipt: %w", err)
	}
	sourcePath := filepath.Join(repo, filepath.FromSlash(change.SourcePath))
	sourceBytes, err := refsf.ReadStableRegularFileAnchored(repo, sourcePath, "completed pack-memory consumer-use source", maxConsumptionFileBytes)
	if err != nil || !strings.EqualFold(sha256Hex(sourceBytes), change.SourceSHA256) {
		return currentConsumptionReceipt{}, fmt.Errorf("completed pack-memory consumer-use source drifted: %w", err)
	}
	var predecessorBytes []byte
	if receipt.BackupPath != "" {
		backupPath, joinErr := refsf.SafeJoin(caseFull, receipt.BackupPath)
		if joinErr != nil {
			return currentConsumptionReceipt{}, joinErr
		}
		predecessorBytes, err = refsf.ReadStableRegularFileAnchored(caseFull, backupPath, "pack-memory consumer-use predecessor backup", maxConsumptionFileBytes)
		if err != nil || !strings.EqualFold(sha256Hex(predecessorBytes), receipt.TargetSHA256Before) {
			return currentConsumptionReceipt{}, fmt.Errorf("pack-memory consumer-use predecessor backup drifted: %w", err)
		}
	} else if receipt.TargetSHA256Before != "" {
		return currentConsumptionReceipt{}, fmt.Errorf("pack-memory consumer-use predecessor backup is missing")
	}
	deltaChunks := acceptedDeltaChunks(predecessorBytes, sourceBytes)
	if len(deltaChunks) == 0 {
		return currentConsumptionReceipt{}, fmt.Errorf("completed pack-memory change has no accepted delta")
	}
	return currentConsumptionReceipt{repo: repo, caseRoot: caseFull, change: change, receipt: receipt, receiptPath: status.ReceiptPath, receiptBytes: receiptBytes, acceptedDeltaChunks: deltaChunks}, nil
}

func acceptedDeltaChunks(before, after []byte) [][]byte {
	beforeLines := bytes.Split(bytes.ReplaceAll(before, []byte("\r\n"), []byte("\n")), []byte("\n"))
	afterLines := bytes.Split(bytes.ReplaceAll(after, []byte("\r\n"), []byte("\n")), []byte("\n"))
	known := make(map[string]int, len(beforeLines))
	for _, line := range beforeLines {
		known[string(line)]++
	}
	chunks := [][]byte{}
	for _, line := range afterLines {
		key := string(line)
		if known[key] > 0 {
			known[key]--
			continue
		}
		if trimmed := bytes.TrimSpace(line); len(trimmed) > 0 {
			chunks = append(chunks, append([]byte{}, trimmed...))
		}
	}
	return chunks
}

func consumerQuoteInAcceptedDelta(chunks [][]byte, quote []byte) bool {
	quote = bytes.TrimSpace(quote)
	for _, chunk := range chunks {
		if bytes.Contains(chunk, quote) {
			return true
		}
	}
	return false
}

func decodeConsumerUseStatement(data []byte) (ConsumerUseStatement, error) {
	if len(data) == 0 || len(data) > maxConsumerUseBytes {
		return ConsumerUseStatement{}, fmt.Errorf("pack-memory consumer-use statement must be bounded and non-empty")
	}
	var statement ConsumerUseStatement
	dec := json.NewDecoder(bytes.NewReader(data))
	dec.DisallowUnknownFields()
	if err := dec.Decode(&statement); err != nil {
		return ConsumerUseStatement{}, fmt.Errorf("decode pack-memory consumer-use statement: %w", err)
	}
	var trailing any
	if err := dec.Decode(&trailing); err != io.EOF {
		return ConsumerUseStatement{}, fmt.Errorf("pack-memory consumer-use statement must contain exactly one JSON object")
	}
	statement.ChangeID = strings.TrimSpace(statement.ChangeID)
	statement.SourceSHA256 = strings.TrimSpace(statement.SourceSHA256)
	statement.Quote = strings.TrimSpace(statement.Quote)
	statement.AppliedAs = strings.TrimSpace(statement.AppliedAs)
	if statement.SchemaVersion != 1 || statement.Kind != KindConsumerUseStatement || statement.ChangeID == "" || !validSHA256(statement.SourceSHA256) || statement.Quote == "" || statement.AppliedAs == "" || !statement.NoAuthority || !statement.NoConfirmed || !statement.NoHeavyTool {
		return ConsumerUseStatement{}, fmt.Errorf("invalid pack-memory consumer-use statement contract")
	}
	return statement, nil
}

func consumerUseOutput(outputs []memberexecution.Output, path string) (memberexecution.Output, bool) {
	for _, output := range outputs {
		if filepath.ToSlash(output.Path) == path {
			return output, true
		}
	}
	return memberexecution.Output{}, false
}

func consumerUseProofSHA256(proof ConsumerUseProof) (string, error) {
	proof.ProofSHA256 = ""
	data, err := canonical(proof)
	if err != nil {
		return "", err
	}
	return sha256Hex(data), nil
}
