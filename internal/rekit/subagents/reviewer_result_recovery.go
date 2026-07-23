package subagents

import (
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/shuiyu486/re-context-kits/internal/rekit/casebind"
	"github.com/shuiyu486/re-context-kits/internal/rekit/instance"
	"github.com/shuiyu486/re-context-kits/internal/rekit/mission"
	"github.com/shuiyu486/re-context-kits/internal/rekit/reviewpath"
)

type ReviewerResultRecoveryOptions struct {
	PacketPath                   string
	ShardID                      string
	Lane                         string
	Actor                        string
	Reason                       string
	ExpectedCandidateSHA256      string
	ExpectedReviewerResultSHA256 string
	WhatIf                       bool
}

type ReviewerResultRecoveryReceipt struct {
	SchemaVersion        int    `json:"schemaVersion"`
	Kind                 string `json:"kind"`
	RepoRoot             string `json:"repoRoot"`
	CaseRoot             string `json:"caseRoot"`
	Pack                 string `json:"pack"`
	PacketID             string `json:"packetId"`
	PacketPath           string `json:"packetPath"`
	ShardID              string `json:"shardId"`
	Lane                 string `json:"lane"`
	CandidatePath        string `json:"candidatePath"`
	CandidateSHA256      string `json:"candidateSha256"`
	CandidateBytes       int    `json:"candidateBytes"`
	ReviewerResultPath   string `json:"reviewerResultPath"`
	ReviewerResultSHA256 string `json:"reviewerResultSha256"`
	ReviewerResultBytes  int    `json:"reviewerResultBytes"`
	QuarantinePath       string `json:"quarantinePath"`
	Actor                string `json:"actor"`
	Reason               string `json:"reason"`
	CreatedAt            string `json:"createdAt"`
	NoVerdict            bool   `json:"noReviewerVerdict"`
	NoFacts              bool   `json:"noFactsWrite"`
	NoHeavyTool          bool   `json:"noHeavyTool"`
	NoAuthority          bool   `json:"noAuthorityOrConfirmed"`
}

type ReviewerResultRecoveryResult struct {
	SchemaVersion               int                                      `json:"schemaVersion"`
	Command                     string                                   `json:"command"`
	Mode                        string                                   `json:"mode"`
	CaseRoot                    string                                   `json:"caseRoot"`
	RepoRoot                    string                                   `json:"repoRoot"`
	Pack                        string                                   `json:"pack"`
	IsMutation                  bool                                     `json:"isMutation"`
	Applied                     bool                                     `json:"applied"`
	AlreadyRecovered            bool                                     `json:"alreadyRecovered"`
	RequiresConfirmation        bool                                     `json:"requiresConfirmation"`
	PacketID                    string                                   `json:"packetId"`
	PacketPath                  string                                   `json:"packetPath"`
	ShardID                     string                                   `json:"shardId"`
	Lane                        string                                   `json:"lane"`
	Actor                       string                                   `json:"actor"`
	Reason                      string                                   `json:"reason"`
	CandidatePath               string                                   `json:"candidatePath"`
	CandidateSHA256             string                                   `json:"candidateSha256"`
	CandidateBytes              int                                      `json:"candidateBytes"`
	ReviewerResultPath          string                                   `json:"reviewerResultPath"`
	ReviewerResultSHA256        string                                   `json:"reviewerResultSha256"`
	ReviewerResultBytes         int                                      `json:"reviewerResultBytes"`
	QuarantinePath              string                                   `json:"quarantinePath"`
	IntentPath                  string                                   `json:"intentPath"`
	ReceiptPath                 string                                   `json:"receiptPath"`
	MissionCommanderAction      mission.MissionCommanderAction           `json:"missionCommanderAction"`
	MissionCommanderNextActions []mission.MissionCommanderNextActionItem `json:"missionCommanderNextActions"`
	MissionCommanderActionQueue mission.MissionCommanderActionQueue      `json:"missionCommanderActionQueue"`
	NextSteps                   []string                                 `json:"nextSteps"`
	Boundary                    []string                                 `json:"boundary"`
}

func RecoverReviewerResult(repoRoot, caseRoot, pack string, opt ReviewerResultRecoveryOptions) (ReviewerResultRecoveryResult, error) {
	inst, err := instance.AssertAttached(caseRoot, repoRoot, pack)
	if err != nil {
		return ReviewerResultRecoveryResult{}, err
	}
	repoRoot, err = filepath.Abs(repoRoot)
	if err != nil {
		return ReviewerResultRecoveryResult{}, err
	}
	if !opt.WhatIf {
		if err := validateReviewerResultRecoveryExpectedHashes(opt); err != nil {
			return ReviewerResultRecoveryResult{}, err
		}
	}
	prepared, err := prepareReviewerResultCollectionMode(repoRoot, inst.CaseRoot, pack, ReviewerResultCollectionOptions{
		PacketPath: opt.PacketPath, ShardID: opt.ShardID, Lane: opt.Lane, Actor: opt.Actor, WhatIf: true,
	}, true)
	if err != nil {
		return ReviewerResultRecoveryResult{}, err
	}
	if err := ensureReviewerResultRecoveryAllowed(inst.CaseRoot, prepared.packet, prepared.handoff.ShardID, prepared.lane); err != nil {
		return ReviewerResultRecoveryResult{}, err
	}
	if strings.TrimSpace(opt.Reason) == "" {
		return ReviewerResultRecoveryResult{}, fmt.Errorf("reviewer result recovery requires non-empty -Reason")
	}
	paths := reviewerResultRecoveryPaths(prepared)
	result := newReviewerResultRecoveryResult(repoRoot, inst.CaseRoot, pack, opt, prepared, paths)
	if len(prepared.canonical) == 0 {
		if opt.WhatIf {
			return resumeReviewerResultRecovery(inst.CaseRoot, result, paths, opt)
		}
		unlock, lockErr := acquireReviewerIntakeLock(inst.CaseRoot, "reviewer-collection-"+prepared.packet.PacketID+"-"+prepared.handoff.ShardID)
		if lockErr != nil {
			return ReviewerResultRecoveryResult{}, lockErr
		}
		defer unlock()
		prepared, err = prepareReviewerResultCollectionMode(repoRoot, inst.CaseRoot, pack, ReviewerResultCollectionOptions{
			PacketPath: opt.PacketPath, ShardID: opt.ShardID, Lane: opt.Lane, Actor: opt.Actor, WhatIf: true,
		}, true)
		if err != nil {
			return ReviewerResultRecoveryResult{}, err
		}
		if err := ensureReviewerResultRecoveryAllowed(inst.CaseRoot, prepared.packet, prepared.handoff.ShardID, prepared.lane); err != nil {
			return ReviewerResultRecoveryResult{}, err
		}
		if len(prepared.canonical) != 0 {
			return ReviewerResultRecoveryResult{}, fmt.Errorf("reviewer result changed after recovery preview")
		}
		paths = reviewerResultRecoveryPaths(prepared)
		result = newReviewerResultRecoveryResult(repoRoot, inst.CaseRoot, pack, opt, prepared, paths)
		return resumeReviewerResultRecovery(inst.CaseRoot, result, paths, opt)
	}
	if bytes.Equal(prepared.canonical, prepared.candidate) {
		return ReviewerResultRecoveryResult{}, fmt.Errorf("canonical reviewer result already matches the candidate; recovery is not required")
	}
	if opt.WhatIf {
		result.NextSteps = []string{"inspect the exact candidate and canonical reviewer result hashes, then run the returned recovery command with -Apply"}
		result.MissionCommanderAction = mission.MissionCommanderAction{
			State:          "needs-reviewer-result-recovery-apply",
			PrimaryCommand: reviewerResultRecoveryCommand(prepared.packetPath, prepared.handoff.ShardID, prepared.lane, prepared.actor, strings.TrimSpace(opt.Reason), result.CandidateSHA256, result.ReviewerResultSHA256, true),
			Boundary:       result.Boundary,
		}
		return finalizeReviewerResultRecoveryResult(result), nil
	}

	unlock, err := acquireReviewerIntakeLock(inst.CaseRoot, "reviewer-collection-"+prepared.packet.PacketID+"-"+prepared.handoff.ShardID)
	if err != nil {
		return ReviewerResultRecoveryResult{}, err
	}
	defer unlock()
	prepared, err = prepareReviewerResultCollectionMode(repoRoot, inst.CaseRoot, pack, ReviewerResultCollectionOptions{
		PacketPath: opt.PacketPath, ShardID: opt.ShardID, Lane: opt.Lane, Actor: opt.Actor, WhatIf: true,
	}, true)
	if err != nil {
		return ReviewerResultRecoveryResult{}, err
	}
	if err := ensureReviewerResultRecoveryAllowed(inst.CaseRoot, prepared.packet, prepared.handoff.ShardID, prepared.lane); err != nil {
		return ReviewerResultRecoveryResult{}, err
	}
	paths = reviewerResultRecoveryPaths(prepared)
	result = newReviewerResultRecoveryResult(repoRoot, inst.CaseRoot, pack, opt, prepared, paths)
	if !strings.EqualFold(result.CandidateSHA256, strings.TrimSpace(opt.ExpectedCandidateSHA256)) || !strings.EqualFold(result.ReviewerResultSHA256, strings.TrimSpace(opt.ExpectedReviewerResultSHA256)) {
		return ReviewerResultRecoveryResult{}, fmt.Errorf("reviewer result or candidate changed after recovery preview")
	}
	receipt := reviewerResultRecoveryReceipt(result)
	if err := ensureReviewerResultRecoveryRoot(inst.CaseRoot, paths); err != nil {
		return ReviewerResultRecoveryResult{}, err
	}
	if err := writeReviewerResultRecoveryReceipt(inst.CaseRoot, paths.intentPath, receipt); err != nil {
		return ReviewerResultRecoveryResult{}, fmt.Errorf("write reviewer result recovery intent: %w", err)
	}
	if err := quarantineReviewerResult(inst.CaseRoot, prepared.handoff.ReviewerResultPath, paths.quarantinePath, prepared.canonical); err != nil {
		return ReviewerResultRecoveryResult{}, err
	}
	if err := writeReviewerResultRecoveryReceipt(inst.CaseRoot, paths.receiptPath, receipt); err != nil {
		return ReviewerResultRecoveryResult{}, err
	}
	result.Applied = true
	result.NextSteps = []string{"run reviewer result collection -WhatIf again; apply collection only after confirming the candidate remains correct"}
	result.MissionCommanderAction = mission.MissionCommanderAction{
		State:          "reviewer-result-recovered-ready-for-collection-preview",
		PrimaryCommand: reviewerResultCollectionCommand(prepared.packetPath, prepared.handoff.ShardID, prepared.lane, prepared.actor, false),
		Boundary:       result.Boundary,
	}
	return finalizeReviewerResultRecoveryResult(result), nil
}

type reviewerResultRecoveryPathSet struct {
	quarantinePath string
	intentPath     string
	receiptPath    string
}

func reviewerResultRecoveryPaths(prepared preparedReviewerResultCollection) reviewerResultRecoveryPathSet {
	root := filepath.Join(prepared.packet.ReviewerOrchestration.ResultRoot, "recoveries")
	resultSHA := sha256Hex(prepared.canonical)
	if len(prepared.canonical) == 0 {
		resultSHA = "missing"
	}
	return reviewerResultRecoveryPathSet{
		quarantinePath: filepath.Join(root, prepared.handoff.ShardID+"-"+resultSHA+".json"),
		intentPath:     filepath.Join(root, prepared.handoff.ShardID+".recovery.intent.json"),
		receiptPath:    filepath.Join(root, prepared.handoff.ShardID+".recovery.json"),
	}
}

func newReviewerResultRecoveryResult(repoRoot, caseRoot, pack string, opt ReviewerResultRecoveryOptions, prepared preparedReviewerResultCollection, paths reviewerResultRecoveryPathSet) ReviewerResultRecoveryResult {
	boundary := []string{
		"recovery quarantines one exact conflicting canonical reviewer result; it never overwrites it with candidate bytes",
		"recovery does not create a reviewer verdict, append facts, or undo an existing verification or decision",
		"collection and reviewer intake remain separate WhatIf then explicit Apply operations",
		"runtime does not spawn or monitor reviewers, execute heavy tools, or write authority/confirmed state",
	}
	return ReviewerResultRecoveryResult{
		SchemaVersion: 1, Command: commandName, Mode: "reviewer-result-recovery",
		CaseRoot: caseRoot, RepoRoot: repoRoot, Pack: pack,
		IsMutation: !opt.WhatIf, RequiresConfirmation: true,
		PacketID: prepared.packet.PacketID, PacketPath: prepared.packetPath,
		ShardID: prepared.handoff.ShardID, Lane: prepared.lane, Actor: prepared.actor, Reason: strings.TrimSpace(opt.Reason),
		CandidatePath: prepared.handoff.ReviewerResultCandidatePath, CandidateSHA256: sha256Hex(prepared.candidate), CandidateBytes: len(prepared.candidate),
		ReviewerResultPath: prepared.handoff.ReviewerResultPath, ReviewerResultSHA256: sha256Hex(prepared.canonical), ReviewerResultBytes: len(prepared.canonical),
		QuarantinePath: paths.quarantinePath, IntentPath: paths.intentPath, ReceiptPath: paths.receiptPath, Boundary: boundary,
	}
}

func finalizeReviewerResultRecoveryResult(result ReviewerResultRecoveryResult) ReviewerResultRecoveryResult {
	if result.MissionCommanderAction.State == "" {
		result.NextSteps = []string{"the exact reviewer result recovery is already recorded; run collection -WhatIf if the canonical result remains absent"}
		result.MissionCommanderAction = mission.MissionCommanderAction{
			State:          "reviewer-result-recovery-already-applied",
			PrimaryCommand: reviewerResultCollectionCommand(result.PacketPath, result.ShardID, result.Lane, result.Actor, false),
			Boundary:       result.Boundary,
		}
	}
	result.MissionCommanderNextActions = []mission.MissionCommanderNextActionItem{{
		Lane: result.Lane, Label: result.PacketID, ActionID: result.PacketID + ":" + result.ShardID,
		State: result.MissionCommanderAction.State, Command: result.MissionCommanderAction.PrimaryCommand,
		Source: "reviewerResultRecovery", RequiresReview: true,
		Reasons: append([]string{}, result.NextSteps...), Boundary: append([]string{}, result.Boundary...),
	}}
	result.MissionCommanderActionQueue = mission.MissionCommanderActionQueueFor(result.MissionCommanderNextActions)
	return result
}

func validateReviewerResultRecoveryExpectedHashes(opt ReviewerResultRecoveryOptions) error {
	for label, value := range map[string]string{
		"candidate":       strings.TrimSpace(opt.ExpectedCandidateSHA256),
		"reviewer result": strings.TrimSpace(opt.ExpectedReviewerResultSHA256),
	} {
		decoded, err := hex.DecodeString(value)
		if value == "" || err != nil || len(decoded) != sha256.Size {
			return fmt.Errorf("reviewer result recovery Apply requires a valid expected %s SHA-256 from WhatIf", label)
		}
	}
	return nil
}

func ensureReviewerResultRecoveryAllowed(caseRoot string, packet Packet, shardID, lane string) error {
	for _, kind := range []string{"verification", "decision"} {
		events, err := mission.ReadStrictFact(caseRoot, kind)
		if err != nil {
			return err
		}
		for _, event := range events {
			if reviewerEventMatches(event, packet, shardID, lane) {
				return fmt.Errorf("reviewer result recovery is forbidden after reviewer %s writeback for packet %s shard %s", kind, packet.PacketID, shardID)
			}
		}
	}
	return nil
}

func ensureReviewerResultCollectionRecoveryComplete(caseRoot string, packet Packet, packetPath string, handoff ShardHandoff, lane string, candidate []byte) error {
	root := filepath.Join(packet.ReviewerOrchestration.ResultRoot, "recoveries")
	intentPath := filepath.Join(root, handoff.ShardID+".recovery.intent.json")
	if _, err := os.Lstat(intentPath); os.IsNotExist(err) {
		return nil
	} else if err != nil {
		return err
	}
	intent, err := readReviewerResultRecoveryReceipt(caseRoot, intentPath)
	if err != nil {
		return fmt.Errorf("reviewer result recovery intent is invalid; collection remains blocked: %w", err)
	}
	result := ReviewerResultRecoveryResult{
		RepoRoot: intent.RepoRoot, CaseRoot: caseRoot, Pack: intent.Pack,
		PacketID: packet.PacketID, PacketPath: packetPath, ShardID: handoff.ShardID, Lane: lane,
		Actor: intent.Actor, Reason: intent.Reason,
		CandidatePath: handoff.ReviewerResultCandidatePath, CandidateSHA256: sha256Hex(candidate), CandidateBytes: len(candidate),
		ReviewerResultPath: handoff.ReviewerResultPath, ReviewerResultSHA256: intent.ReviewerResultSHA256, ReviewerResultBytes: intent.ReviewerResultBytes,
		QuarantinePath: intent.QuarantinePath,
		IntentPath:     intentPath, ReceiptPath: filepath.Join(root, handoff.ShardID+".recovery.json"),
	}
	if !reviewerResultRecoveryReceiptCurrent(intent, result) {
		return fmt.Errorf("reviewer result recovery intent does not match current collection bindings")
	}
	receipt, err := readReviewerResultRecoveryReceipt(caseRoot, result.ReceiptPath)
	if err != nil || receipt.CreatedAt != intent.CreatedAt || !reviewerResultRecoveryReceiptEquivalent(receipt, intent) {
		return fmt.Errorf("reviewer result recovery must be finalized before collection")
	}
	return nil
}

func ensureReviewerResultRecoveryRoot(caseRoot string, paths reviewerResultRecoveryPathSet) error {
	root := filepath.Dir(paths.receiptPath)
	if !samePath(root, filepath.Dir(paths.intentPath)) || !samePath(root, filepath.Dir(paths.quarantinePath)) {
		return fmt.Errorf("reviewer result recovery paths do not share one canonical root")
	}
	if _, err := os.Lstat(root); os.IsNotExist(err) {
		if err := os.Mkdir(root, 0o700); err != nil {
			return err
		}
	} else if err != nil {
		return err
	}
	if !reviewpath.CollectionNamespacePathSafe(caseRoot, root, false) ||
		!reviewpath.CollectionNamespacePathSafe(caseRoot, paths.intentPath, true) ||
		!reviewpath.CollectionNamespacePathSafe(caseRoot, paths.receiptPath, true) ||
		!reviewpath.CollectionNamespacePathSafe(caseRoot, paths.quarantinePath, true) {
		return fmt.Errorf("reviewer result recovery paths are not safe")
	}
	return nil
}

func quarantineReviewerResult(caseRoot, resultPath, quarantinePath string, expected []byte) error {
	if len(expected) == 0 {
		return fmt.Errorf("canonical reviewer result is missing")
	}
	root := filepath.Dir(filepath.Dir(resultPath))
	quarantineRoot := filepath.Dir(quarantinePath)
	if !reviewpath.CollectionNamespacePathSafe(caseRoot, root, false) || !reviewpath.CollectionNamespacePathSafe(caseRoot, resultPath, false) {
		return fmt.Errorf("reviewer result recovery paths are not safe")
	}
	if _, err := os.Lstat(quarantineRoot); os.IsNotExist(err) {
		if err := os.Mkdir(quarantineRoot, 0o700); err != nil {
			return err
		}
	} else if err != nil {
		return err
	}
	if !reviewpath.CollectionNamespacePathSafe(caseRoot, quarantineRoot, false) || !reviewpath.CollectionNamespacePathSafe(caseRoot, quarantinePath, true) {
		return fmt.Errorf("reviewer result recovery quarantine path is not safe")
	}
	if _, err := os.Lstat(quarantinePath); err == nil {
		existing, readErr := readStableReviewerArtifact(quarantineRoot, quarantinePath, "quarantined reviewer result", maxReviewerResultBytes)
		if readErr != nil {
			return readErr
		}
		if !bytes.Equal(existing, expected) {
			return fmt.Errorf("reviewer result recovery quarantine already contains different bytes")
		}
		current, readErr := readStableReviewerArtifact(root, resultPath, "canonical reviewer result", maxReviewerResultBytes)
		if readErr != nil || !bytes.Equal(current, expected) {
			return fmt.Errorf("canonical reviewer result changed before interrupted recovery resumed")
		}
		if err := os.Remove(resultPath); err != nil {
			return err
		}
		return nil
	} else if !os.IsNotExist(err) {
		return err
	}
	if err := os.Rename(resultPath, quarantinePath); err != nil {
		return fmt.Errorf("quarantine canonical reviewer result: %w", err)
	}
	moved, err := readStableReviewerArtifact(quarantineRoot, quarantinePath, "quarantined reviewer result", maxReviewerResultBytes)
	if err != nil || !bytes.Equal(moved, expected) {
		return fmt.Errorf("verify quarantined reviewer result: %w", err)
	}
	return nil
}

func resumeReviewerResultRecovery(caseRoot string, result ReviewerResultRecoveryResult, paths reviewerResultRecoveryPathSet, opt ReviewerResultRecoveryOptions) (ReviewerResultRecoveryResult, error) {
	receipt, receiptErr := readReviewerResultRecoveryReceipt(caseRoot, paths.receiptPath)
	if receiptErr == nil {
		return recoveredReviewerResult(result, receipt, true)
	}
	intent, intentErr := readReviewerResultRecoveryReceipt(caseRoot, paths.intentPath)
	if intentErr != nil {
		return ReviewerResultRecoveryResult{}, fmt.Errorf("canonical reviewer result is missing and no exact recovery receipt or recoverable intent exists")
	}
	result.ReviewerResultSHA256 = intent.ReviewerResultSHA256
	result.ReviewerResultBytes = intent.ReviewerResultBytes
	result.QuarantinePath = intent.QuarantinePath
	if !reviewerResultRecoveryReceiptCurrent(intent, result) {
		return ReviewerResultRecoveryResult{}, fmt.Errorf("reviewer result recovery intent does not match the current candidate and canonical namespace")
	}
	quarantined, err := readStableReviewerArtifact(filepath.Dir(intent.QuarantinePath), intent.QuarantinePath, "quarantined reviewer result", maxReviewerResultBytes)
	if err != nil || sha256Hex(quarantined) != intent.ReviewerResultSHA256 || len(quarantined) != intent.ReviewerResultBytes {
		return ReviewerResultRecoveryResult{}, fmt.Errorf("exact quarantined reviewer result for recovery intent is missing or changed")
	}
	if result.IsMutation {
		if !strings.EqualFold(result.CandidateSHA256, strings.TrimSpace(opt.ExpectedCandidateSHA256)) || !strings.EqualFold(result.ReviewerResultSHA256, strings.TrimSpace(opt.ExpectedReviewerResultSHA256)) {
			return ReviewerResultRecoveryResult{}, fmt.Errorf("reviewer result or candidate changed after recovery preview")
		}
		if err := writeReviewerResultRecoveryReceipt(caseRoot, paths.receiptPath, intent); err != nil {
			return ReviewerResultRecoveryResult{}, err
		}
		return recoveredReviewerResult(result, intent, false)
	}
	result.NextSteps = []string{"an interrupted exact reviewer result recovery is ready to finalize; run the returned expected-hash recovery command with -Apply"}
	result.MissionCommanderAction = mission.MissionCommanderAction{
		State:          "needs-reviewer-result-recovery-finalize-apply",
		PrimaryCommand: reviewerResultRecoveryCommand(result.PacketPath, result.ShardID, result.Lane, result.Actor, result.Reason, result.CandidateSHA256, result.ReviewerResultSHA256, true),
		Boundary:       result.Boundary,
	}
	return finalizeReviewerResultRecoveryResult(result), nil
}

func recoveredReviewerResult(result ReviewerResultRecoveryResult, receipt ReviewerResultRecoveryReceipt, already bool) (ReviewerResultRecoveryResult, error) {
	result.ReviewerResultSHA256 = receipt.ReviewerResultSHA256
	result.ReviewerResultBytes = receipt.ReviewerResultBytes
	result.QuarantinePath = receipt.QuarantinePath
	if !reviewerResultRecoveryReceiptCurrent(receipt, result) {
		return ReviewerResultRecoveryResult{}, fmt.Errorf("reviewer result recovery receipt does not match the current candidate and canonical namespace")
	}
	quarantined, err := readStableReviewerArtifact(filepath.Dir(receipt.QuarantinePath), receipt.QuarantinePath, "quarantined reviewer result", maxReviewerResultBytes)
	if err != nil || sha256Hex(quarantined) != receipt.ReviewerResultSHA256 || len(quarantined) != receipt.ReviewerResultBytes {
		return ReviewerResultRecoveryResult{}, fmt.Errorf("exact quarantined reviewer result is missing or changed")
	}
	result.Applied = true
	result.AlreadyRecovered = already
	return finalizeReviewerResultRecoveryResult(result), nil
}

func reviewerResultRecoveryReceipt(result ReviewerResultRecoveryResult) ReviewerResultRecoveryReceipt {
	return ReviewerResultRecoveryReceipt{
		SchemaVersion: 1, Kind: "reviewer-result-recovery",
		RepoRoot: result.RepoRoot, CaseRoot: result.CaseRoot, Pack: result.Pack,
		PacketID: result.PacketID, PacketPath: result.PacketPath, ShardID: result.ShardID, Lane: result.Lane,
		CandidatePath: result.CandidatePath, CandidateSHA256: result.CandidateSHA256, CandidateBytes: result.CandidateBytes,
		ReviewerResultPath: result.ReviewerResultPath, ReviewerResultSHA256: result.ReviewerResultSHA256, ReviewerResultBytes: result.ReviewerResultBytes,
		QuarantinePath: result.QuarantinePath, Actor: result.Actor, Reason: result.Reason,
		CreatedAt: time.Now().UTC().Format(time.RFC3339Nano), NoVerdict: true, NoFacts: true, NoHeavyTool: true, NoAuthority: true,
	}
}

func writeReviewerResultRecoveryReceipt(caseRoot, path string, receipt ReviewerResultRecoveryReceipt) error {
	root := filepath.Dir(path)
	if !reviewpath.CollectionNamespacePathSafe(caseRoot, root, false) || !reviewpath.CollectionNamespacePathSafe(caseRoot, path, true) {
		return fmt.Errorf("reviewer result recovery receipt path is not safe")
	}
	if existing, readErr := readReviewerResultRecoveryReceipt(caseRoot, path); readErr == nil {
		if reviewerResultRecoveryReceiptEquivalent(existing, receipt) {
			return nil
		}
		return fmt.Errorf("reviewer result recovery receipt already exists with different bindings")
	} else if _, statErr := os.Lstat(path); statErr == nil {
		return fmt.Errorf("reviewer result recovery receipt already exists but is invalid")
	} else if !os.IsNotExist(statErr) {
		return statErr
	}
	data, err := json.MarshalIndent(receipt, "", "  ")
	if err != nil {
		return err
	}
	data = append(data, '\n')
	tmp, err := os.CreateTemp(root, ".reviewer-result-recovery-*.tmp")
	if err != nil {
		return err
	}
	tmpPath := tmp.Name()
	defer os.Remove(tmpPath)
	if err := tmp.Chmod(0o600); err != nil {
		tmp.Close()
		return err
	}
	if _, err := tmp.Write(data); err != nil {
		tmp.Close()
		return err
	}
	if err := tmp.Sync(); err != nil {
		tmp.Close()
		return err
	}
	if err := tmp.Close(); err != nil {
		return err
	}
	if err := os.Link(tmpPath, path); err != nil {
		if existing, readErr := readReviewerResultRecoveryReceipt(caseRoot, path); readErr == nil && reviewerResultRecoveryReceiptEquivalent(existing, receipt) {
			return nil
		}
		return fmt.Errorf("publish reviewer result recovery receipt without replacement: %w", err)
	}
	existing, err := readReviewerResultRecoveryReceipt(caseRoot, path)
	if err != nil || !reviewerResultRecoveryReceiptEquivalent(existing, receipt) {
		return fmt.Errorf("verify published reviewer result recovery receipt: %w", err)
	}
	return nil
}

func readReviewerResultRecoveryReceipt(_ string, path string) (ReviewerResultRecoveryReceipt, error) {
	data, err := readStableReviewerArtifact(filepath.Dir(path), path, "reviewer result recovery receipt", maxReviewPacketBytes)
	if err != nil {
		return ReviewerResultRecoveryReceipt{}, err
	}
	dec := json.NewDecoder(bytes.NewReader(bytes.TrimSpace(data)))
	dec.DisallowUnknownFields()
	var receipt ReviewerResultRecoveryReceipt
	if err := dec.Decode(&receipt); err != nil {
		return ReviewerResultRecoveryReceipt{}, err
	}
	var trailing any
	if err := dec.Decode(&trailing); err != io.EOF {
		return ReviewerResultRecoveryReceipt{}, fmt.Errorf("reviewer result recovery receipt must contain exactly one JSON object")
	}
	if _, err := time.Parse(time.RFC3339Nano, receipt.CreatedAt); err != nil {
		return ReviewerResultRecoveryReceipt{}, err
	}
	if receipt.SchemaVersion != 1 || receipt.Kind != "reviewer-result-recovery" {
		return ReviewerResultRecoveryReceipt{}, fmt.Errorf("reviewer result recovery receipt schema or kind is invalid")
	}
	if !validReviewerRecoverySHA256(receipt.CandidateSHA256) || !validReviewerRecoverySHA256(receipt.ReviewerResultSHA256) || receipt.CandidateBytes <= 0 || receipt.ReviewerResultBytes <= 0 {
		return ReviewerResultRecoveryReceipt{}, fmt.Errorf("reviewer result recovery receipt hash or byte binding is invalid")
	}
	if strings.TrimSpace(receipt.PacketID) == "" || strings.TrimSpace(receipt.ShardID) == "" || strings.TrimSpace(receipt.Lane) == "" || strings.TrimSpace(receipt.Actor) == "" || strings.TrimSpace(receipt.Reason) == "" {
		return ReviewerResultRecoveryReceipt{}, fmt.Errorf("reviewer result recovery receipt identity binding is incomplete")
	}
	if !receipt.NoVerdict || !receipt.NoFacts || !receipt.NoHeavyTool || !receipt.NoAuthority {
		return ReviewerResultRecoveryReceipt{}, fmt.Errorf("reviewer result recovery receipt boundary flags are invalid")
	}
	return receipt, nil
}

func validReviewerRecoverySHA256(value string) bool {
	decoded, err := hex.DecodeString(strings.TrimSpace(value))
	return err == nil && len(decoded) == sha256.Size
}

func reviewerResultRecoveryReceiptEquivalent(a, b ReviewerResultRecoveryReceipt) bool {
	return a.SchemaVersion == b.SchemaVersion && a.Kind == b.Kind &&
		casebind.SamePath(a.RepoRoot, b.RepoRoot) && casebind.SamePath(a.CaseRoot, b.CaseRoot) && a.Pack == b.Pack &&
		a.PacketID == b.PacketID && casebind.SamePath(a.PacketPath, b.PacketPath) && a.ShardID == b.ShardID && a.Lane == b.Lane &&
		casebind.SamePath(a.CandidatePath, b.CandidatePath) && a.CandidateSHA256 == b.CandidateSHA256 && a.CandidateBytes == b.CandidateBytes &&
		casebind.SamePath(a.ReviewerResultPath, b.ReviewerResultPath) && a.ReviewerResultSHA256 == b.ReviewerResultSHA256 && a.ReviewerResultBytes == b.ReviewerResultBytes &&
		casebind.SamePath(a.QuarantinePath, b.QuarantinePath) && a.Actor == b.Actor && a.Reason == b.Reason &&
		a.NoVerdict == b.NoVerdict && a.NoFacts == b.NoFacts && a.NoHeavyTool == b.NoHeavyTool && a.NoAuthority == b.NoAuthority
}

func reviewerResultRecoveryReceiptCurrent(receipt ReviewerResultRecoveryReceipt, result ReviewerResultRecoveryResult) bool {
	expectedQuarantinePath := filepath.Join(filepath.Dir(result.ReceiptPath), result.ShardID+"-"+receipt.ReviewerResultSHA256+".json")
	return receipt.SchemaVersion == 1 && receipt.Kind == "reviewer-result-recovery" &&
		casebind.SamePath(receipt.RepoRoot, result.RepoRoot) && casebind.SamePath(receipt.CaseRoot, result.CaseRoot) && receipt.Pack == result.Pack &&
		receipt.PacketID == result.PacketID && casebind.SamePath(receipt.PacketPath, result.PacketPath) && receipt.ShardID == result.ShardID && receipt.Lane == result.Lane &&
		casebind.SamePath(receipt.CandidatePath, result.CandidatePath) && receipt.CandidateSHA256 == result.CandidateSHA256 && receipt.CandidateBytes == result.CandidateBytes &&
		casebind.SamePath(receipt.ReviewerResultPath, result.ReviewerResultPath) && validReviewerRecoverySHA256(receipt.ReviewerResultSHA256) && receipt.ReviewerResultBytes > 0 &&
		casebind.SamePath(receipt.QuarantinePath, expectedQuarantinePath) && reviewpath.CollectionNamespacePathSafe(result.CaseRoot, receipt.QuarantinePath, false) &&
		receipt.Actor == result.Actor && receipt.Reason == result.Reason &&
		receipt.NoVerdict && receipt.NoFacts && receipt.NoHeavyTool && receipt.NoAuthority
}

func reviewerResultRecoveryCommand(packetPath, shardID, lane, actor, reason, candidateSHA256, resultSHA256 string, apply bool) string {
	command := "/rekit plan-subagents -PacketPath " + quoteCommandArg(packetPath) + " -RecoverReviewerResult -ShardId " + quoteCommandArg(shardID) + " -Lane " + quoteCommandArg(lane) + " -Actor " + quoteCommandArg(actor) + " -Reason " + quoteCommandArg(reason)
	if apply {
		command += " -ExpectedCandidateSha256 " + quoteCommandArg(candidateSHA256) + " -ExpectedReviewerResultSha256 " + quoteCommandArg(resultSHA256)
		return command + " -Apply -Format json"
	}
	return command + " -WhatIf -Format json"
}
