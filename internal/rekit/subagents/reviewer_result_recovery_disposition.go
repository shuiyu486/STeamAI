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

type ReviewerResultRecoveryDispositionOptions struct {
	PacketPath              string
	ShardID                 string
	Lane                    string
	Actor                   string
	Reason                  string
	ExpectedIntentSHA256    string
	ExpectedCanonicalSHA256 string
	WhatIf                  bool
}

type ReviewerResultRecoveryDisposition struct {
	SchemaVersion      int    `json:"schemaVersion"`
	Kind               string `json:"kind"`
	Decision           string `json:"decision"`
	RepoRoot           string `json:"repoRoot"`
	CaseRoot           string `json:"caseRoot"`
	Pack               string `json:"pack"`
	PacketID           string `json:"packetId"`
	PacketPath         string `json:"packetPath"`
	ShardID            string `json:"shardId"`
	Lane               string `json:"lane"`
	CandidatePath      string `json:"candidatePath"`
	CandidateSHA256    string `json:"candidateSha256"`
	CandidateBytes     int    `json:"candidateBytes"`
	ReviewerResultPath string `json:"reviewerResultPath"`
	CanonicalSHA256    string `json:"canonicalSha256"`
	CanonicalBytes     int    `json:"canonicalBytes"`
	IntentPath         string `json:"intentPath"`
	IntentSHA256       string `json:"intentSha256"`
	IntentBytes        int    `json:"intentBytes"`
	QuarantinePath     string `json:"quarantinePath"`
	Actor              string `json:"actor"`
	Reason             string `json:"reason"`
	CreatedAt          string `json:"createdAt"`
	NoDelete           bool   `json:"noDelete"`
	NoFacts            bool   `json:"noFactsWrite"`
	NoHeavyTool        bool   `json:"noHeavyTool"`
	NoAuthority        bool   `json:"noAuthorityOrConfirmed"`
}

type ReviewerResultRecoveryDispositionResult struct {
	SchemaVersion               int                                      `json:"schemaVersion"`
	Command                     string                                   `json:"command"`
	Mode                        string                                   `json:"mode"`
	CaseRoot                    string                                   `json:"caseRoot"`
	RepoRoot                    string                                   `json:"repoRoot"`
	Pack                        string                                   `json:"pack"`
	IsMutation                  bool                                     `json:"isMutation"`
	Applied                     bool                                     `json:"applied"`
	RequiresConfirmation        bool                                     `json:"requiresConfirmation"`
	PacketID                    string                                   `json:"packetId"`
	PacketPath                  string                                   `json:"packetPath"`
	ShardID                     string                                   `json:"shardId"`
	Lane                        string                                   `json:"lane"`
	Actor                       string                                   `json:"actor"`
	Reason                      string                                   `json:"reason"`
	CandidatePath               string                                   `json:"candidatePath"`
	CandidateSHA256             string                                   `json:"candidateSha256"`
	ReviewerResultPath          string                                   `json:"reviewerResultPath"`
	CanonicalSHA256             string                                   `json:"canonicalSha256"`
	IntentPath                  string                                   `json:"intentPath"`
	IntentSHA256                string                                   `json:"intentSha256"`
	QuarantinePath              string                                   `json:"quarantinePath"`
	DispositionPath             string                                   `json:"dispositionPath"`
	MissionCommanderAction      mission.MissionCommanderAction           `json:"missionCommanderAction"`
	MissionCommanderNextActions []mission.MissionCommanderNextActionItem `json:"missionCommanderNextActions"`
	MissionCommanderActionQueue mission.MissionCommanderActionQueue      `json:"missionCommanderActionQueue"`
	NextSteps                   []string                                 `json:"nextSteps"`
	Boundary                    []string                                 `json:"boundary"`
}

type preparedReviewerResultRecoveryDisposition struct {
	repoRoot        string
	caseRoot        string
	pack            string
	packet          Packet
	packetPath      string
	handoff         ShardHandoff
	lane            string
	actor           string
	reason          string
	candidate       []byte
	canonical       []byte
	intent          ReviewerResultRecoveryReceipt
	intentPath      string
	intentData      []byte
	dispositionPath string
}

func RetireAmbiguousReviewerResultRecovery(repoRoot, caseRoot, pack string, opt ReviewerResultRecoveryDispositionOptions) (ReviewerResultRecoveryDispositionResult, error) {
	inst, err := instance.AssertAttached(caseRoot, repoRoot, pack)
	if err != nil {
		return ReviewerResultRecoveryDispositionResult{}, err
	}
	repoRoot, err = filepath.Abs(repoRoot)
	if err != nil {
		return ReviewerResultRecoveryDispositionResult{}, err
	}
	if !opt.WhatIf {
		for label, value := range map[string]string{"intent": opt.ExpectedIntentSHA256, "canonical": opt.ExpectedCanonicalSHA256} {
			decoded, decodeErr := hex.DecodeString(strings.TrimSpace(value))
			if decodeErr != nil || len(decoded) != sha256.Size {
				return ReviewerResultRecoveryDispositionResult{}, fmt.Errorf("reviewer result recovery disposition Apply requires a valid expected %s SHA-256 from WhatIf", label)
			}
		}
	}
	prepared, err := prepareReviewerResultRecoveryDisposition(repoRoot, inst.CaseRoot, pack, opt)
	if err != nil {
		return ReviewerResultRecoveryDispositionResult{}, err
	}
	boundary := []string{
		"disposition retains the exact canonical reviewer result and does not delete or modify the intent or quarantine",
		"disposition is valid only while the canonical result still exactly matches the reviewed candidate",
		"runtime does not spawn or monitor reviewers, execute heavy tools, append facts, or write authority/confirmed state",
	}
	result := reviewerResultRecoveryDispositionResult(prepared, opt.WhatIf, boundary)
	if opt.WhatIf {
		result.NextSteps = []string{"review the exact intent, quarantine, and canonical candidate hashes, then run the returned disposition command with -Apply"}
		result.MissionCommanderAction = mission.MissionCommanderAction{State: "needs-reviewer-result-recovery-disposition-apply", PrimaryCommand: reviewerResultRecoveryDispositionCommand(prepared, result.IntentSHA256, result.CanonicalSHA256), Boundary: boundary}
		return finalizeReviewerResultRecoveryDispositionResult(result), nil
	}
	unlock, lockErr := acquireReviewerIntakeLock(inst.CaseRoot, reviewerResultMutationLockID(prepared.packet.PacketID, prepared.handoff.ShardID))
	if lockErr != nil {
		return ReviewerResultRecoveryDispositionResult{}, lockErr
	}
	defer unlock()
	prepared, err = prepareReviewerResultRecoveryDisposition(repoRoot, inst.CaseRoot, pack, opt)
	if err != nil {
		return ReviewerResultRecoveryDispositionResult{}, err
	}
	if !strings.EqualFold(sha256Hex(prepared.intentData), strings.TrimSpace(opt.ExpectedIntentSHA256)) || !strings.EqualFold(sha256Hex(prepared.canonical), strings.TrimSpace(opt.ExpectedCanonicalSHA256)) {
		return ReviewerResultRecoveryDispositionResult{}, fmt.Errorf("reviewer result recovery intent or canonical result changed after disposition preview")
	}
	record := reviewerResultRecoveryDispositionRecord(prepared)
	if err := writeReviewerResultRecoveryDisposition(prepared.caseRoot, prepared.dispositionPath, record); err != nil {
		return ReviewerResultRecoveryDispositionResult{}, err
	}
	result = reviewerResultRecoveryDispositionResult(prepared, false, boundary)
	result.Applied = true
	result.NextSteps = []string{"run reviewer result collection -WhatIf, then explicit -Apply before reviewer intake"}
	result.MissionCommanderAction = mission.MissionCommanderAction{State: "reviewer-result-recovery-disposed-ready-for-collection-preview", PrimaryCommand: reviewerResultCollectionCommand(prepared.packetPath, prepared.handoff.ShardID, prepared.lane, prepared.actor, "", false), Boundary: boundary}
	return finalizeReviewerResultRecoveryDispositionResult(result), nil
}

func prepareReviewerResultRecoveryDisposition(repoRoot, caseRoot, pack string, opt ReviewerResultRecoveryDispositionOptions) (preparedReviewerResultRecoveryDisposition, error) {
	prepared, err := prepareReviewerResultCollectionMode(repoRoot, caseRoot, pack, ReviewerResultCollectionOptions{PacketPath: opt.PacketPath, ShardID: opt.ShardID, Lane: opt.Lane, Actor: opt.Actor, WhatIf: true}, true)
	if err != nil {
		return preparedReviewerResultRecoveryDisposition{}, err
	}
	if err := ensureReviewerResultRecoveryAllowed(caseRoot, prepared.packet, prepared.handoff.ShardID, prepared.lane); err != nil {
		return preparedReviewerResultRecoveryDisposition{}, err
	}
	actor := strings.TrimSpace(opt.Actor)
	reason := strings.TrimSpace(opt.Reason)
	if actor == "" || reason == "" {
		return preparedReviewerResultRecoveryDisposition{}, fmt.Errorf("reviewer result recovery disposition requires non-empty -Actor and -Reason")
	}
	if prepared.canonicalObstruction != nil || !prepared.canonicalPresent || !bytes.Equal(prepared.canonical, prepared.candidate) {
		return preparedReviewerResultRecoveryDisposition{}, fmt.Errorf("reviewer result recovery disposition requires the canonical reviewer result to exactly match the reviewed candidate")
	}
	root := filepath.Join(prepared.packet.ReviewerOrchestration.ResultRoot, "recoveries")
	intentPath := filepath.Join(root, prepared.handoff.ShardID+".recovery.intent.json")
	receiptPath := filepath.Join(root, prepared.handoff.ShardID+".recovery.json")
	dispositionPath := filepath.Join(root, prepared.handoff.ShardID+".recovery.disposition.json")
	intentData, err := readStableReviewerArtifact(root, intentPath, "reviewer result recovery intent", maxReviewPacketBytes)
	if err != nil {
		return preparedReviewerResultRecoveryDisposition{}, err
	}
	intent, err := readReviewerResultRecoveryReceipt(caseRoot, intentPath)
	if err != nil {
		return preparedReviewerResultRecoveryDisposition{}, err
	}
	if _, err := os.Lstat(receiptPath); err == nil || !os.IsNotExist(err) {
		return preparedReviewerResultRecoveryDisposition{}, fmt.Errorf("reviewer result recovery disposition requires an unfinished intent without a committed receipt")
	}
	if !reviewerResultRecoveryDispositionIntentMatches(repoRoot, caseRoot, pack, prepared.packet, prepared.packetPath, prepared.handoff, prepared.lane, prepared.candidate, intent) || !reviewerResultRecoveryQuarantineMatches(caseRoot, intent) {
		return preparedReviewerResultRecoveryDisposition{}, fmt.Errorf("reviewer result recovery intent or quarantine does not match current packet bindings")
	}
	if expected := strings.TrimSpace(opt.ExpectedIntentSHA256); expected != "" && !strings.EqualFold(expected, sha256Hex(intentData)) {
		return preparedReviewerResultRecoveryDisposition{}, fmt.Errorf("reviewer result recovery intent changed after disposition preview")
	}
	if expected := strings.TrimSpace(opt.ExpectedCanonicalSHA256); expected != "" && !strings.EqualFold(expected, sha256Hex(prepared.canonical)) {
		return preparedReviewerResultRecoveryDisposition{}, fmt.Errorf("canonical reviewer result changed after disposition preview")
	}
	return preparedReviewerResultRecoveryDisposition{repoRoot: repoRoot, caseRoot: caseRoot, pack: pack, packet: prepared.packet, packetPath: prepared.packetPath, handoff: prepared.handoff, lane: prepared.lane, actor: actor, reason: reason, candidate: prepared.candidate, canonical: prepared.canonical, intent: intent, intentPath: intentPath, intentData: intentData, dispositionPath: dispositionPath}, nil
}

func reviewerResultRecoveryDispositionIntentMatches(repoRoot, caseRoot, pack string, packet Packet, packetPath string, handoff ShardHandoff, lane string, candidate []byte, intent ReviewerResultRecoveryReceipt) bool {
	return casebind.SamePath(repoRoot, packet.RepoRoot) && pack == packet.Pack && casebind.SamePath(intent.RepoRoot, repoRoot) && casebind.SamePath(intent.CaseRoot, caseRoot) && intent.Pack == pack && intent.PacketID == packet.PacketID && casebind.SamePath(intent.PacketPath, packetPath) && intent.ShardID == handoff.ShardID && intent.Lane == lane && casebind.SamePath(intent.CandidatePath, handoff.ReviewerResultCandidatePath) && intent.CandidateSHA256 == sha256Hex(candidate) && intent.CandidateBytes == len(candidate) && casebind.SamePath(intent.ReviewerResultPath, handoff.ReviewerResultPath)
}

func reviewerResultRecoveryDispositionRecord(prepared preparedReviewerResultRecoveryDisposition) ReviewerResultRecoveryDisposition {
	return ReviewerResultRecoveryDisposition{SchemaVersion: 1, Kind: "reviewer-result-recovery-disposition", Decision: "retain-canonical", RepoRoot: prepared.repoRoot, CaseRoot: prepared.caseRoot, Pack: prepared.pack, PacketID: prepared.packet.PacketID, PacketPath: prepared.packetPath, ShardID: prepared.handoff.ShardID, Lane: prepared.lane, CandidatePath: prepared.handoff.ReviewerResultCandidatePath, CandidateSHA256: sha256Hex(prepared.candidate), CandidateBytes: len(prepared.candidate), ReviewerResultPath: prepared.handoff.ReviewerResultPath, CanonicalSHA256: sha256Hex(prepared.canonical), CanonicalBytes: len(prepared.canonical), IntentPath: prepared.intentPath, IntentSHA256: sha256Hex(prepared.intentData), IntentBytes: len(prepared.intentData), QuarantinePath: prepared.intent.QuarantinePath, Actor: prepared.actor, Reason: prepared.reason, CreatedAt: time.Now().UTC().Format(time.RFC3339Nano), NoDelete: true, NoFacts: true, NoHeavyTool: true, NoAuthority: true}
}

func reviewerResultRecoveryDispositionResult(prepared preparedReviewerResultRecoveryDisposition, whatIf bool, boundary []string) ReviewerResultRecoveryDispositionResult {
	return ReviewerResultRecoveryDispositionResult{SchemaVersion: 1, Command: commandName, Mode: "reviewer-result-recovery-disposition", CaseRoot: prepared.caseRoot, RepoRoot: prepared.repoRoot, Pack: prepared.pack, IsMutation: !whatIf, RequiresConfirmation: true, PacketID: prepared.packet.PacketID, PacketPath: prepared.packetPath, ShardID: prepared.handoff.ShardID, Lane: prepared.lane, Actor: prepared.actor, Reason: prepared.reason, CandidatePath: prepared.handoff.ReviewerResultCandidatePath, CandidateSHA256: sha256Hex(prepared.candidate), ReviewerResultPath: prepared.handoff.ReviewerResultPath, CanonicalSHA256: sha256Hex(prepared.canonical), IntentPath: prepared.intentPath, IntentSHA256: sha256Hex(prepared.intentData), QuarantinePath: prepared.intent.QuarantinePath, DispositionPath: prepared.dispositionPath, Boundary: boundary}
}

func finalizeReviewerResultRecoveryDispositionResult(result ReviewerResultRecoveryDispositionResult) ReviewerResultRecoveryDispositionResult {
	result.MissionCommanderNextActions = []mission.MissionCommanderNextActionItem{{Lane: result.Lane, Label: result.PacketID, ActionID: result.PacketID + ":" + result.ShardID, State: result.MissionCommanderAction.State, Command: result.MissionCommanderAction.PrimaryCommand, Source: "reviewerResultRecoveryDisposition", RequiresReview: true, Reasons: append([]string{}, result.NextSteps...), Boundary: append([]string{}, result.Boundary...)}}
	result.MissionCommanderActionQueue = mission.MissionCommanderActionQueueFor(result.MissionCommanderNextActions)
	return result
}

func reviewerResultRecoveryDispositionCommand(prepared preparedReviewerResultRecoveryDisposition, intentSHA, canonicalSHA string) string {
	return "/rekit plan-subagents -PacketPath " + quoteCommandArg(prepared.packetPath) + " -RetireReviewerResultRecovery -ShardId " + quoteCommandArg(prepared.handoff.ShardID) + " -Lane " + quoteCommandArg(prepared.lane) + " -Actor " + quoteCommandArg(prepared.actor) + " -Reason " + quoteCommandArg(prepared.reason) + " -ExpectedIntentSha256 " + intentSHA + " -ExpectedCanonicalSha256 " + canonicalSHA + " -Apply -Format json"
}

func reviewerResultRecoveryDispositionCurrent(caseRoot string, packet Packet, packetPath string, handoff ShardHandoff, lane string, candidate []byte) (bool, error) {
	root := filepath.Join(packet.ReviewerOrchestration.ResultRoot, "recoveries")
	path := filepath.Join(root, handoff.ShardID+".recovery.disposition.json")
	if _, err := os.Lstat(path); os.IsNotExist(err) {
		return false, nil
	} else if err != nil {
		return false, err
	}
	disposition, err := readReviewerResultRecoveryDisposition(caseRoot, path)
	if err != nil {
		return false, err
	}
	canonicalIntentPath := filepath.Join(root, handoff.ShardID+".recovery.intent.json")
	if !casebind.SamePath(disposition.IntentPath, canonicalIntentPath) {
		return false, fmt.Errorf("reviewer result recovery disposition intent path is not canonical")
	}
	intentData, err := readStableReviewerArtifact(root, canonicalIntentPath, "reviewer result recovery intent", maxReviewPacketBytes)
	if err != nil {
		return false, err
	}
	canonical, err := readStableReviewerArtifact(filepath.Dir(handoff.ReviewerResultPath), handoff.ReviewerResultPath, "canonical reviewer result", maxReviewerResultBytes)
	if err != nil {
		return false, err
	}
	intent, err := readReviewerResultRecoveryReceipt(caseRoot, canonicalIntentPath)
	if err != nil {
		return false, err
	}
	if !casebind.SamePath(disposition.RepoRoot, intent.RepoRoot) || !casebind.SamePath(disposition.CaseRoot, caseRoot) || disposition.Pack != intent.Pack || disposition.Actor == "" || disposition.Reason == "" {
		return false, fmt.Errorf("reviewer result recovery disposition provenance is invalid")
	}
	current := disposition.SchemaVersion == 1 && disposition.Kind == "reviewer-result-recovery-disposition" && disposition.Decision == "retain-canonical" && disposition.PacketID == packet.PacketID && disposition.ShardID == handoff.ShardID && disposition.Lane == lane && casebind.SamePath(disposition.PacketPath, packetPath) && casebind.SamePath(disposition.CandidatePath, handoff.ReviewerResultCandidatePath) && disposition.CandidateSHA256 == sha256Hex(candidate) && disposition.CandidateBytes == len(candidate) && casebind.SamePath(disposition.ReviewerResultPath, handoff.ReviewerResultPath) && disposition.CanonicalSHA256 == sha256Hex(canonical) && disposition.CanonicalBytes == len(canonical) && bytes.Equal(canonical, candidate) && disposition.IntentSHA256 == sha256Hex(intentData) && disposition.IntentBytes == len(intentData) && casebind.SamePath(disposition.QuarantinePath, intent.QuarantinePath) && reviewerResultRecoveryDispositionIntentMatches(disposition.RepoRoot, caseRoot, disposition.Pack, packet, packetPath, handoff, lane, candidate, intent) && reviewerResultRecoveryQuarantineMatches(caseRoot, intent) && disposition.NoDelete && disposition.NoFacts && disposition.NoHeavyTool && disposition.NoAuthority
	return current, nil
}

func writeReviewerResultRecoveryDisposition(caseRoot, path string, disposition ReviewerResultRecoveryDisposition) error {
	if !reviewpath.CollectionNamespacePathSafe(caseRoot, filepath.Dir(path), false) || !reviewpath.CollectionNamespacePathSafe(caseRoot, path, true) {
		return fmt.Errorf("reviewer result recovery disposition path is not safe")
	}
	if existing, err := readReviewerResultRecoveryDisposition(caseRoot, path); err == nil {
		if reviewerResultRecoveryDispositionsEquivalent(existing, disposition) {
			return nil
		}
		return fmt.Errorf("reviewer result recovery disposition already exists with different bindings")
	} else if _, statErr := os.Lstat(path); statErr == nil || !os.IsNotExist(statErr) {
		return fmt.Errorf("reviewer result recovery disposition already exists but is invalid")
	}
	data, err := json.MarshalIndent(disposition, "", "  ")
	if err != nil {
		return err
	}
	data = append(data, '\n')
	tmp, err := os.CreateTemp(filepath.Dir(path), ".reviewer-result-recovery-disposition-*.tmp")
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
		return fmt.Errorf("publish reviewer result recovery disposition without replacement: %w", err)
	}
	return nil
}

func readReviewerResultRecoveryDisposition(caseRoot, path string) (ReviewerResultRecoveryDisposition, error) {
	data, err := readStableReviewerArtifact(filepath.Dir(path), path, "reviewer result recovery disposition", maxReviewPacketBytes)
	if err != nil {
		return ReviewerResultRecoveryDisposition{}, err
	}
	dec := json.NewDecoder(bytes.NewReader(bytes.TrimSpace(data)))
	dec.DisallowUnknownFields()
	var disposition ReviewerResultRecoveryDisposition
	if err := dec.Decode(&disposition); err != nil {
		return ReviewerResultRecoveryDisposition{}, err
	}
	var trailing any
	if err := dec.Decode(&trailing); err != io.EOF {
		return ReviewerResultRecoveryDisposition{}, fmt.Errorf("reviewer result recovery disposition must contain exactly one JSON object")
	}
	if _, err := time.Parse(time.RFC3339Nano, disposition.CreatedAt); err != nil {
		return ReviewerResultRecoveryDisposition{}, err
	}
	if disposition.SchemaVersion != 1 || disposition.Kind != "reviewer-result-recovery-disposition" || disposition.Decision != "retain-canonical" || !validReviewerRecoverySHA256(disposition.CandidateSHA256) || !validReviewerRecoverySHA256(disposition.CanonicalSHA256) || !validReviewerRecoverySHA256(disposition.IntentSHA256) || disposition.CandidateBytes <= 0 || disposition.CanonicalBytes <= 0 || disposition.IntentBytes <= 0 || !disposition.NoDelete || !disposition.NoFacts || !disposition.NoHeavyTool || !disposition.NoAuthority {
		return ReviewerResultRecoveryDisposition{}, fmt.Errorf("reviewer result recovery disposition schema or bindings are invalid")
	}
	if !reviewpath.CollectionNamespacePathSafe(caseRoot, path, false) {
		return ReviewerResultRecoveryDisposition{}, fmt.Errorf("reviewer result recovery disposition path is not safe")
	}
	return disposition, nil
}

func reviewerResultRecoveryDispositionsEquivalent(a, b ReviewerResultRecoveryDisposition) bool {
	return a.SchemaVersion == b.SchemaVersion && a.Kind == b.Kind && a.Decision == b.Decision && casebind.SamePath(a.RepoRoot, b.RepoRoot) && casebind.SamePath(a.CaseRoot, b.CaseRoot) && a.Pack == b.Pack && a.PacketID == b.PacketID && casebind.SamePath(a.PacketPath, b.PacketPath) && a.ShardID == b.ShardID && a.Lane == b.Lane && casebind.SamePath(a.CandidatePath, b.CandidatePath) && a.CandidateSHA256 == b.CandidateSHA256 && a.CandidateBytes == b.CandidateBytes && casebind.SamePath(a.ReviewerResultPath, b.ReviewerResultPath) && a.CanonicalSHA256 == b.CanonicalSHA256 && a.CanonicalBytes == b.CanonicalBytes && casebind.SamePath(a.IntentPath, b.IntentPath) && a.IntentSHA256 == b.IntentSHA256 && a.IntentBytes == b.IntentBytes && casebind.SamePath(a.QuarantinePath, b.QuarantinePath) && a.Actor == b.Actor && a.Reason == b.Reason && a.NoDelete == b.NoDelete && a.NoFacts == b.NoFacts && a.NoHeavyTool == b.NoHeavyTool && a.NoAuthority == b.NoAuthority
}
