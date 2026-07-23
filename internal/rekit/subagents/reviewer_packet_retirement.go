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

type ReviewerPacketRetirementOptions struct {
	PacketPath              string
	Lane                    string
	Actor                   string
	Reason                  string
	ExpectedPacketSHA256    string
	ExpectedIntegritySHA256 string
	WhatIf                  bool
}

type ReviewerPacketRetirement struct {
	SchemaVersion   int    `json:"schemaVersion"`
	Kind            string `json:"kind"`
	RepoRoot        string `json:"repoRoot"`
	CaseRoot        string `json:"caseRoot"`
	Pack            string `json:"pack"`
	PacketID        string `json:"packetId"`
	Lane            string `json:"lane"`
	PacketPath      string `json:"packetPath"`
	PacketSHA256    string `json:"packetSha256"`
	PacketBytes     int    `json:"packetBytes"`
	IntegrityPath   string `json:"integrityPath"`
	IntegritySHA256 string `json:"integritySha256"`
	IntegrityBytes  int    `json:"integrityBytes"`
	Actor           string `json:"actor"`
	Reason          string `json:"reason"`
	CreatedAt       string `json:"createdAt"`
	NoDelete        bool   `json:"noDelete"`
	NoHeavyTool     bool   `json:"noHeavyTool"`
	NoAuthority     bool   `json:"noAuthorityOrConfirmed"`
}

type ReviewerPacketRetirementResult struct {
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
	IntegrityPath               string                                   `json:"integrityPath"`
	RetirementPath              string                                   `json:"retirementPath"`
	Lane                        string                                   `json:"lane"`
	Actor                       string                                   `json:"actor"`
	Reason                      string                                   `json:"reason"`
	InvalidReason               string                                   `json:"invalidReason"`
	PacketSHA256                string                                   `json:"packetSha256"`
	PacketBytes                 int                                      `json:"packetBytes"`
	IntegritySHA256             string                                   `json:"integritySha256"`
	IntegrityBytes              int                                      `json:"integrityBytes"`
	MissionCommanderAction      mission.MissionCommanderAction           `json:"missionCommanderAction"`
	MissionCommanderNextActions []mission.MissionCommanderNextActionItem `json:"missionCommanderNextActions"`
	MissionCommanderActionQueue mission.MissionCommanderActionQueue      `json:"missionCommanderActionQueue"`
	NextSteps                   []string                                 `json:"nextSteps"`
	Boundary                    []string                                 `json:"boundary"`
}

type preparedReviewerPacketRetirement struct {
	repoRoot       string
	caseRoot       string
	pack           string
	packetPath     string
	packetData     []byte
	integrityPath  string
	integrityData  []byte
	integrity      reviewerPacketIntegrity
	retirementPath string
	invalidReason  string
	lane           string
	actor          string
	reason         string
}

func RetireInvalidReviewerPacket(repoRoot, caseRoot, pack string, opt ReviewerPacketRetirementOptions) (ReviewerPacketRetirementResult, error) {
	inst, err := instance.AssertAttached(caseRoot, repoRoot, pack)
	if err != nil {
		return ReviewerPacketRetirementResult{}, err
	}
	repoRoot, err = filepath.Abs(repoRoot)
	if err != nil {
		return ReviewerPacketRetirementResult{}, err
	}
	pack = strings.TrimSpace(pack)
	if !opt.WhatIf {
		if err := validateReviewerPacketRetirementExpectedHashes(opt); err != nil {
			return ReviewerPacketRetirementResult{}, err
		}
	}
	prepared, err := prepareReviewerPacketRetirement(repoRoot, inst.CaseRoot, pack, opt)
	if err != nil {
		return ReviewerPacketRetirementResult{}, err
	}
	boundary := []string{
		"retirement only closes one exact invalid canonical reviewer packet snapshot; it does not delete or repair packet bytes",
		"changed packet or integrity bytes invalidate the retirement receipt and restore the blocker",
		"runtime does not spawn or monitor reviewers, execute heavy tools, append facts, or write authority/confirmed state",
	}
	result := ReviewerPacketRetirementResult{
		SchemaVersion: 1, Command: commandName, Mode: "reviewer-packet-retirement",
		CaseRoot: inst.CaseRoot, RepoRoot: repoRoot, Pack: pack,
		IsMutation: !opt.WhatIf, RequiresConfirmation: true,
		PacketID: prepared.integrity.PacketID, PacketPath: prepared.packetPath,
		IntegrityPath: prepared.integrityPath, RetirementPath: prepared.retirementPath,
		Lane: prepared.lane, Actor: prepared.actor, Reason: prepared.reason,
		InvalidReason: prepared.invalidReason,
		PacketSHA256:  sha256Hex(prepared.packetData), PacketBytes: len(prepared.packetData),
		IntegritySHA256: sha256Hex(prepared.integrityData), IntegrityBytes: len(prepared.integrityData),
		Boundary: boundary,
	}
	applyCommand := reviewerPacketRetirementCommand(prepared.packetPath, prepared.lane, prepared.actor, prepared.reason, result.PacketSHA256, result.IntegritySHA256, true)
	if opt.WhatIf {
		result.NextSteps = []string{"review the exact invalid packet/integrity hashes, then run the same retirement command with -Apply"}
		result.MissionCommanderAction = mission.MissionCommanderAction{State: "needs-reviewer-packet-retirement-apply", PrimaryCommand: applyCommand, Boundary: boundary}
	} else {
		unlock, lockErr := acquireReviewerIntakeLock(inst.CaseRoot, "reviewer-retirement-"+sha256Hex([]byte(filepath.Clean(prepared.packetPath))))
		if lockErr != nil {
			return ReviewerPacketRetirementResult{}, lockErr
		}
		defer unlock()
		prepared, err = prepareReviewerPacketRetirement(repoRoot, inst.CaseRoot, pack, opt)
		if err != nil {
			return ReviewerPacketRetirementResult{}, err
		}
		result.PacketID = prepared.integrity.PacketID
		result.PacketPath = prepared.packetPath
		result.IntegrityPath = prepared.integrityPath
		result.RetirementPath = prepared.retirementPath
		result.Lane = prepared.lane
		result.Actor = prepared.actor
		result.Reason = prepared.reason
		result.InvalidReason = prepared.invalidReason
		result.PacketSHA256 = sha256Hex(prepared.packetData)
		result.PacketBytes = len(prepared.packetData)
		result.IntegritySHA256 = sha256Hex(prepared.integrityData)
		result.IntegrityBytes = len(prepared.integrityData)
		receipt := ReviewerPacketRetirement{
			SchemaVersion: 1, Kind: "reviewer-packet-retirement",
			RepoRoot: prepared.repoRoot, CaseRoot: prepared.caseRoot, Pack: prepared.pack,
			PacketID: prepared.integrity.PacketID, Lane: prepared.lane,
			PacketPath: prepared.packetPath, PacketSHA256: sha256Hex(prepared.packetData), PacketBytes: len(prepared.packetData),
			IntegrityPath: prepared.integrityPath, IntegritySHA256: sha256Hex(prepared.integrityData), IntegrityBytes: len(prepared.integrityData),
			Actor: prepared.actor, Reason: prepared.reason,
			NoDelete: true, NoHeavyTool: true, NoAuthority: true,
		}
		existing, err := readReviewerPacketRetirement(prepared.caseRoot, prepared.retirementPath)
		if err == nil {
			if !reviewerPacketRetirementMatches(existing, receipt) {
				return ReviewerPacketRetirementResult{}, fmt.Errorf("reviewer packet retirement already exists for a different snapshot or decision: %s", prepared.retirementPath)
			}
		} else if !os.IsNotExist(err) {
			return ReviewerPacketRetirementResult{}, err
		} else {
			receipt.CreatedAt = time.Now().UTC().Format(time.RFC3339Nano)
			if err := writeReviewerPacketRetirement(prepared.caseRoot, prepared.retirementPath, receipt); err != nil {
				return ReviewerPacketRetirementResult{}, err
			}
		}
		result.Applied = true
		result.NextSteps = []string{"regenerate a new canonical reviewer packet if reviewer work remains; otherwise resume the lane after confirming the blocker is absent"}
		result.MissionCommanderAction = mission.MissionCommanderAction{State: "reviewer-packet-retired", PrimaryCommand: "/rekit status", Boundary: boundary}
	}
	result.MissionCommanderNextActions = []mission.MissionCommanderNextActionItem{{Lane: result.Lane, Label: result.PacketID, State: result.MissionCommanderAction.State, Command: result.MissionCommanderAction.PrimaryCommand, Source: "reviewerPacketRetirement", RequiresReview: true, Reasons: append([]string{}, result.NextSteps...), Boundary: append([]string{}, boundary...)}}
	result.MissionCommanderActionQueue = mission.MissionCommanderActionQueueFor(result.MissionCommanderNextActions)
	return result, nil
}

func validateReviewerPacketRetirementExpectedHashes(opt ReviewerPacketRetirementOptions) error {
	for _, expected := range []struct {
		label string
		value string
	}{
		{label: "packet", value: strings.TrimSpace(opt.ExpectedPacketSHA256)},
		{label: "integrity", value: strings.TrimSpace(opt.ExpectedIntegritySHA256)},
	} {
		decoded, err := hex.DecodeString(expected.value)
		if expected.value == "" || err != nil || len(decoded) != sha256.Size {
			return fmt.Errorf("reviewer packet retirement Apply requires a valid expected %s SHA-256 from WhatIf", expected.label)
		}
	}
	return nil
}

func prepareReviewerPacketRetirement(repoRoot, caseRoot, pack string, opt ReviewerPacketRetirementOptions) (preparedReviewerPacketRetirement, error) {
	packetPath, err := requiredAbsolutePath(opt.PacketPath, "review packet")
	if err != nil {
		return preparedReviewerPacketRetirement{}, err
	}
	namespace, ok := reviewpath.CanonicalCollectionNamespace(caseRoot, packetPath)
	if !ok || !reviewpath.CollectionNamespacePathSafe(caseRoot, packetPath, false) {
		return preparedReviewerPacketRetirement{}, fmt.Errorf("reviewer packet retirement requires a symlink-free canonical case review packet")
	}
	packetData, err := readStableReviewerArtifact(filepath.Dir(packetPath), packetPath, "review packet", maxReviewPacketBytes)
	if err != nil {
		return preparedReviewerPacketRetirement{}, err
	}
	integrityPath := filepath.Join(namespace.ReviewRoot, "packet.integrity.json")
	integrityData, err := readStableReviewerArtifact(namespace.ReviewRoot, integrityPath, "review packet integrity", maxReviewPacketBytes)
	if err != nil {
		return preparedReviewerPacketRetirement{}, err
	}
	integrity, err := decodeReviewerPacketIntegrity(integrityData)
	if err != nil {
		return preparedReviewerPacketRetirement{}, err
	}
	if !casebind.SamePath(integrity.PacketPath, packetPath) || strings.TrimSpace(integrity.PacketSHA256) == "" || integrity.PacketBytes < 0 {
		return preparedReviewerPacketRetirement{}, fmt.Errorf("review packet integrity provenance is not canonical")
	}
	if _, err := hex.DecodeString(integrity.PacketSHA256); err != nil || len(integrity.PacketSHA256) != sha256.Size*2 {
		return preparedReviewerPacketRetirement{}, fmt.Errorf("review packet integrity packetSha256 is invalid")
	}
	lane := strings.TrimSpace(opt.Lane)
	actor := strings.TrimSpace(opt.Actor)
	reason := strings.TrimSpace(opt.Reason)
	if lane == "" || actor == "" || reason == "" || lane != integrity.TargetLane {
		return preparedReviewerPacketRetirement{}, fmt.Errorf("reviewer packet retirement requires matching -Lane plus non-empty -Actor and -Reason")
	}
	invalidReason := ""
	packet, decodeErr := decodeIntakePacket(packetData)
	if decodeErr != nil {
		invalidReason = decodeErr.Error()
	} else if integrityErr := validatePacketIntegrity(caseRoot, packetPath, packet, packetData); integrityErr != nil {
		invalidReason = integrityErr.Error()
	}
	if invalidReason == "" {
		return preparedReviewerPacketRetirement{}, fmt.Errorf("reviewer packet is valid; retirement is only available for an integrity-invalid packet")
	}
	if expected := strings.TrimSpace(opt.ExpectedPacketSHA256); expected != "" && !strings.EqualFold(expected, sha256Hex(packetData)) {
		return preparedReviewerPacketRetirement{}, fmt.Errorf("reviewer packet changed after retirement preview")
	}
	if expected := strings.TrimSpace(opt.ExpectedIntegritySHA256); expected != "" && !strings.EqualFold(expected, sha256Hex(integrityData)) {
		return preparedReviewerPacketRetirement{}, fmt.Errorf("reviewer packet integrity changed after retirement preview")
	}
	return preparedReviewerPacketRetirement{
		repoRoot: repoRoot, caseRoot: caseRoot, pack: pack,
		packetPath: packetPath, packetData: packetData,
		integrityPath: integrityPath, integrityData: integrityData, integrity: integrity,
		retirementPath: filepath.Join(namespace.ReviewRoot, "packet.retirement.json"),
		invalidReason:  invalidReason, lane: lane, actor: actor, reason: reason,
	}, nil
}

func decodeReviewerPacketIntegrity(data []byte) (reviewerPacketIntegrity, error) {
	dec := json.NewDecoder(bytes.NewReader(bytes.TrimSpace(data)))
	dec.DisallowUnknownFields()
	var integrity reviewerPacketIntegrity
	if err := dec.Decode(&integrity); err != nil {
		return reviewerPacketIntegrity{}, fmt.Errorf("decode review packet integrity: %w", err)
	}
	var trailing any
	if err := dec.Decode(&trailing); err != io.EOF {
		return reviewerPacketIntegrity{}, fmt.Errorf("review packet integrity must contain exactly one JSON object")
	}
	if integrity.SchemaVersion != 1 || integrity.Kind != "reviewer-packet-integrity" || !strings.EqualFold(integrity.Algorithm, "sha256") || strings.TrimSpace(integrity.PacketID) == "" || strings.TrimSpace(integrity.TargetLane) == "" {
		return reviewerPacketIntegrity{}, fmt.Errorf("review packet integrity has unsupported identity")
	}
	return integrity, nil
}

func readReviewerPacketRetirement(caseRoot, path string) (ReviewerPacketRetirement, error) {
	if _, err := os.Lstat(path); err != nil {
		return ReviewerPacketRetirement{}, err
	}
	if !reviewpath.CollectionNamespacePathSafe(caseRoot, path, false) {
		return ReviewerPacketRetirement{}, fmt.Errorf("reviewer packet retirement path is not a safe regular file: %s", path)
	}
	data, err := readStableReviewerArtifact(filepath.Dir(path), path, "reviewer packet retirement", maxReviewPacketBytes)
	if err != nil {
		return ReviewerPacketRetirement{}, err
	}
	dec := json.NewDecoder(bytes.NewReader(bytes.TrimSpace(data)))
	dec.DisallowUnknownFields()
	var retirement ReviewerPacketRetirement
	if err := dec.Decode(&retirement); err != nil {
		return ReviewerPacketRetirement{}, fmt.Errorf("decode reviewer packet retirement: %w", err)
	}
	var trailing any
	if err := dec.Decode(&trailing); err != io.EOF {
		return ReviewerPacketRetirement{}, fmt.Errorf("reviewer packet retirement must contain exactly one JSON object")
	}
	if _, err := time.Parse(time.RFC3339Nano, retirement.CreatedAt); err != nil {
		return ReviewerPacketRetirement{}, fmt.Errorf("reviewer packet retirement createdAt is invalid: %w", err)
	}
	return retirement, nil
}

func reviewerPacketRetirementMatches(existing, expected ReviewerPacketRetirement) bool {
	return existing.SchemaVersion == expected.SchemaVersion && existing.Kind == expected.Kind &&
		casebind.SamePath(existing.RepoRoot, expected.RepoRoot) && casebind.SamePath(existing.CaseRoot, expected.CaseRoot) && existing.Pack == expected.Pack &&
		existing.PacketID == expected.PacketID && existing.Lane == expected.Lane &&
		casebind.SamePath(existing.PacketPath, expected.PacketPath) && existing.PacketSHA256 == expected.PacketSHA256 && existing.PacketBytes == expected.PacketBytes &&
		casebind.SamePath(existing.IntegrityPath, expected.IntegrityPath) && existing.IntegritySHA256 == expected.IntegritySHA256 && existing.IntegrityBytes == expected.IntegrityBytes &&
		existing.Actor == expected.Actor && existing.Reason == expected.Reason && existing.NoDelete == expected.NoDelete && existing.NoHeavyTool == expected.NoHeavyTool && existing.NoAuthority == expected.NoAuthority
}

func writeReviewerPacketRetirement(caseRoot, path string, value ReviewerPacketRetirement) error {
	if !reviewpath.CollectionNamespacePathSafe(caseRoot, path, true) {
		return fmt.Errorf("reviewer packet retirement path is not safe: %s", path)
	}
	if _, err := os.Lstat(path); err == nil {
		return fmt.Errorf("reviewer packet retirement already exists: %s", path)
	} else if !os.IsNotExist(err) {
		return err
	}
	if !reviewpath.CollectionNamespacePathSafe(caseRoot, filepath.Dir(path), false) {
		return fmt.Errorf("reviewer packet retirement parent is not safe: %s", filepath.Dir(path))
	}
	data, err := json.MarshalIndent(value, "", "  ")
	if err != nil {
		return err
	}
	data = append(data, '\n')
	file, err := os.OpenFile(path, os.O_WRONLY|os.O_CREATE|os.O_EXCL, 0o600)
	if err != nil {
		return err
	}
	if _, err := file.Write(data); err != nil {
		file.Close()
		_ = os.Remove(path)
		return err
	}
	if err := file.Sync(); err != nil {
		file.Close()
		_ = os.Remove(path)
		return err
	}
	return file.Close()
}

func reviewerPacketRetirementCommand(packetPath, lane, actor, reason, packetSHA256, integritySHA256 string, apply bool) string {
	command := "/rekit plan-subagents -PacketPath " + quoteCommandArg(packetPath) + " -RetireInvalidReviewerPacket -Lane " + quoteCommandArg(lane) + " -Actor " + quoteCommandArg(actor) + " -Reason " + quoteCommandArg(reason)
	if strings.TrimSpace(packetSHA256) != "" && strings.TrimSpace(integritySHA256) != "" {
		command += " -ExpectedPacketSha256 " + quoteCommandArg(packetSHA256) + " -ExpectedIntegritySha256 " + quoteCommandArg(integritySHA256)
	}
	if apply {
		return command + " -Apply -Format json"
	}
	return command + " -WhatIf -Format json"
}
