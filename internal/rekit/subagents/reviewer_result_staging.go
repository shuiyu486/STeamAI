package subagents

import (
	"bytes"
	"crypto/rand"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"

	"github.com/shuiyu486/re-context-kits/internal/rekit/instance"
	"github.com/shuiyu486/re-context-kits/internal/rekit/mission"
	"github.com/shuiyu486/re-context-kits/internal/rekit/reviewpath"
)

type ReviewerResultStagingOptions struct {
	PacketPath           string
	ShardID              string
	SourcePath           string
	Lane                 string
	Actor                string
	ExpectedSourceSHA256 string
	WhatIf               bool
}

type ReviewerResultStagingResult struct {
	SchemaVersion               int                                      `json:"schemaVersion"`
	Command                     string                                   `json:"command"`
	Mode                        string                                   `json:"mode"`
	CaseRoot                    string                                   `json:"caseRoot"`
	RepoRoot                    string                                   `json:"repoRoot"`
	Pack                        string                                   `json:"pack"`
	IsMutation                  bool                                     `json:"isMutation"`
	Applied                     bool                                     `json:"applied"`
	Status                      string                                   `json:"status"`
	RequiresConfirmation        bool                                     `json:"requiresConfirmation"`
	PacketID                    string                                   `json:"packetId"`
	PacketPath                  string                                   `json:"packetPath"`
	ShardID                     string                                   `json:"shardId"`
	Lane                        string                                   `json:"lane"`
	Actor                       string                                   `json:"actor"`
	SourcePath                  string                                   `json:"sourcePath"`
	SourceSHA256                string                                   `json:"sourceSha256"`
	SourceBytes                 int                                      `json:"sourceBytes"`
	CandidatePath               string                                   `json:"candidatePath"`
	AlreadyStaged               bool                                     `json:"alreadyStaged"`
	ReviewerResult              ReviewerResult                           `json:"reviewerResult"`
	MissionCommanderAction      mission.MissionCommanderAction           `json:"missionCommanderAction"`
	MissionCommanderNextActions []mission.MissionCommanderNextActionItem `json:"missionCommanderNextActions"`
	MissionCommanderActionQueue mission.MissionCommanderActionQueue      `json:"missionCommanderActionQueue"`
	NextSteps                   []string                                 `json:"nextSteps"`
	Boundary                    []string                                 `json:"boundary"`
}

type reviewerPacketAnchoredSnapshot struct {
	parentID  os.FileInfo
	packet    []byte
	integrity []byte
}

type preparedReviewerResultStaging struct {
	packetPath     string
	packet         Packet
	packetSnapshot reviewerPacketAnchoredSnapshot
	resultRootID   os.FileInfo
	handoff        ShardHandoff
	sourcePath     string
	source         []byte
	result         ReviewerResult
	alreadyStaged  bool
	lane           string
	actor          string
}

func StageReviewerResult(repoRoot, caseRoot, pack string, opt ReviewerResultStagingOptions) (ReviewerResultStagingResult, error) {
	inst, err := instance.AssertAttached(caseRoot, repoRoot, pack)
	if err != nil {
		return ReviewerResultStagingResult{}, err
	}
	caseRoot = inst.CaseRoot
	if !opt.WhatIf {
		decoded, decodeErr := hex.DecodeString(strings.TrimSpace(opt.ExpectedSourceSHA256))
		if decodeErr != nil || len(decoded) != sha256.Size {
			return ReviewerResultStagingResult{}, fmt.Errorf("reviewer result staging Apply requires a valid -ExpectedSourceSha256 from WhatIf")
		}
	}
	prepared, err := prepareReviewerResultStaging(repoRoot, caseRoot, pack, opt)
	if err != nil {
		return ReviewerResultStagingResult{}, err
	}
	result := newReviewerResultStagingResult(repoRoot, caseRoot, pack, opt, prepared)
	if opt.WhatIf {
		result.Status = "previewed"
		if prepared.alreadyStaged {
			result.Status = "already-staged"
			result.NextSteps = []string{"the exact source bytes are already staged; run reviewer result collection -WhatIf"}
			result.MissionCommanderAction = mission.MissionCommanderAction{
				State:          "reviewer-result-staged-ready-for-collection-preview",
				PrimaryCommand: reviewerResultCollectionCommand(prepared.packetPath, prepared.handoff.ShardID, prepared.lane, prepared.actor, false),
				Boundary:       result.Boundary,
			}
		} else {
			result.NextSteps = []string{"inspect the source, packet bindings, and exact source SHA-256, then run the returned staging command with -Apply"}
			result.MissionCommanderAction = mission.MissionCommanderAction{
				State:          "ready-for-reviewer-result-staging-apply",
				PrimaryCommand: reviewerResultStagingCommand(prepared.packetPath, prepared.handoff.ShardID, prepared.sourcePath, prepared.lane, prepared.actor, result.SourceSHA256, true),
				Boundary:       result.Boundary,
			}
		}
		return finalizeReviewerResultStagingResult(result), nil
	}

	unlock, err := acquireReviewerIntakeLock(caseRoot, reviewerResultMutationLockID(prepared.packet.PacketID, prepared.handoff.ShardID))
	if err != nil {
		return ReviewerResultStagingResult{}, err
	}
	defer unlock()
	prepared, err = prepareReviewerResultStaging(repoRoot, caseRoot, pack, opt)
	if err != nil {
		return ReviewerResultStagingResult{}, err
	}
	if !strings.EqualFold(sha256Hex(prepared.source), strings.TrimSpace(opt.ExpectedSourceSHA256)) {
		return ReviewerResultStagingResult{}, fmt.Errorf("reviewer result staging source changed after preview")
	}
	if !reviewerPacketSnapshotCurrent(caseRoot, prepared.packetPath, prepared.packetSnapshot) {
		return ReviewerResultStagingResult{}, fmt.Errorf("review packet changed after staging validation")
	}
	already, err := publishReviewerResultCandidateAnchoredExpected(caseRoot, prepared.packet, prepared.handoff, prepared.source, prepared.packetSnapshot, prepared.resultRootID, nil)
	if err != nil {
		return ReviewerResultStagingResult{}, err
	}
	result = newReviewerResultStagingResult(repoRoot, caseRoot, pack, opt, prepared)
	result.Applied = true
	result.AlreadyStaged = already
	if already {
		result.Status = "already-staged"
	} else {
		result.Status = "staged"
	}
	result.NextSteps = []string{"run reviewer result collection -WhatIf, inspect its candidate/result bindings, then use its explicit -Apply command"}
	result.MissionCommanderAction = mission.MissionCommanderAction{
		State:          "reviewer-result-staged-ready-for-collection-preview",
		PrimaryCommand: reviewerResultCollectionCommand(prepared.packetPath, prepared.handoff.ShardID, prepared.lane, prepared.actor, false),
		Boundary:       result.Boundary,
	}
	return finalizeReviewerResultStagingResult(result), nil
}

func prepareReviewerResultStaging(repoRoot, caseRoot, pack string, opt ReviewerResultStagingOptions) (preparedReviewerResultStaging, error) {
	packetPath, err := requiredAbsolutePath(opt.PacketPath, "review packet")
	if err != nil {
		return preparedReviewerResultStaging{}, err
	}
	if _, ok := reviewpath.CanonicalCollectionNamespace(caseRoot, packetPath); !ok || !reviewpath.CollectionNamespacePathSafe(caseRoot, packetPath, false) {
		return preparedReviewerResultStaging{}, fmt.Errorf("review packet must be a symlink-free canonical case review packet")
	}
	packetData, packetRootID, err := readReviewerPacketAndIntegrityAnchored(caseRoot, packetPath)
	if err != nil {
		return preparedReviewerResultStaging{}, err
	}
	packet, err := decodeIntakePacket(packetData)
	if err != nil {
		return preparedReviewerResultStaging{}, err
	}
	if err := validateIntakePacket(packet, packetPath, repoRoot, caseRoot, pack); err != nil {
		return preparedReviewerResultStaging{}, err
	}
	if err := validatePacketIntegrityBytes(packetPath, packet, packetData, packetRootID.integrity); err != nil {
		return preparedReviewerResultStaging{}, err
	}
	if err := validateIntakePacketRoute(repoRoot, pack, packet); err != nil {
		return preparedReviewerResultStaging{}, err
	}
	lane := strings.TrimSpace(opt.Lane)
	actor := strings.TrimSpace(opt.Actor)
	shardID := strings.TrimSpace(opt.ShardID)
	if lane == "" || actor == "" || shardID == "" {
		return preparedReviewerResultStaging{}, fmt.Errorf("reviewer result staging requires -PacketPath, -ShardId, -ReviewerResultSourcePath, -Lane, and -Actor")
	}
	if lane != packet.TargetLane {
		return preparedReviewerResultStaging{}, fmt.Errorf("reviewer result staging lane %q does not match packet targetLane %q", lane, packet.TargetLane)
	}
	handoff, ok := shardHandoffByID(packet.ShardHandoffs, shardID)
	if !ok {
		return preparedReviewerResultStaging{}, fmt.Errorf("reviewer result staging shard %q is not present in packet", shardID)
	}
	namespace, ok := reviewpath.CanonicalCollectionNamespace(caseRoot, packetPath)
	if !ok {
		return preparedReviewerResultStaging{}, fmt.Errorf("review packet does not provide a canonical collection namespace")
	}
	expectedCandidatePath := filepath.Join(namespace.ResultRoot, "candidates", shardID+".json")
	expectedResultPath := filepath.Join(namespace.ResultRoot, shardID+".json")
	if !reviewpath.CanonicalCollectionShard(caseRoot, packetPath, packet.ReviewerOrchestration.ResultRoot, shardID, handoff.ReviewerResultCandidatePath, handoff.ReviewerResultPath) ||
		!samePath(packet.Observability.ReviewRoot, namespace.ReviewRoot) ||
		!samePath(packet.Observability.ResultRoot, namespace.ResultRoot) ||
		!samePath(packet.ReviewerOrchestration.PacketPath, packetPath) ||
		!samePath(handoff.ReviewerResultCandidatePath, expectedCandidatePath) ||
		!samePath(handoff.ReviewerResultPath, expectedResultPath) {
		return preparedReviewerResultStaging{}, fmt.Errorf("review packet staging paths must match the canonical case review namespace for shard %q", shardID)
	}
	if !reviewpath.CollectionNamespacePathSafe(caseRoot, namespace.ResultRoot, false) ||
		!reviewpath.CollectionNamespacePathSafe(caseRoot, filepath.Dir(expectedCandidatePath), true) {
		return preparedReviewerResultStaging{}, fmt.Errorf("reviewer result staging target must not traverse symlinks or escape the attached case")
	}
	resultRootID, err := reviewerDirectoryIdentity(caseRoot, namespace.ResultRoot)
	if err != nil {
		return preparedReviewerResultStaging{}, err
	}
	if !reviewerPacketSnapshotCurrent(caseRoot, packetPath, packetRootID) {
		return preparedReviewerResultStaging{}, fmt.Errorf("review packet namespace changed while validating staging")
	}
	sourcePath, err := requiredAbsolutePath(opt.SourcePath, "reviewer result staging source")
	if err != nil {
		return preparedReviewerResultStaging{}, err
	}
	if handoff.ReviewerStagingCommands != nil && strings.TrimSpace(handoff.ReviewerStagingCommands.SourcePath) != "" {
		expectedSourcePath, err := requiredAbsolutePath(handoff.ReviewerStagingCommands.SourcePath, "reviewer result staging source handoff")
		if err != nil {
			return preparedReviewerResultStaging{}, err
		}
		if !reviewpath.CollectionNamespacePathSafe(caseRoot, expectedSourcePath, false) || !samePath(sourcePath, expectedSourcePath) {
			return preparedReviewerResultStaging{}, fmt.Errorf("reviewer result staging source must match the packet-derived reviewerStagingCommands.sourcePath for shard %q", shardID)
		}
	}
	if samePath(sourcePath, expectedCandidatePath) || samePath(sourcePath, expectedResultPath) {
		return preparedReviewerResultStaging{}, fmt.Errorf("reviewer result staging source must be separate from canonical candidate and result paths")
	}
	source, err := readStableReviewerArtifactAnchored(caseRoot, sourcePath, "reviewer result staging source", maxReviewerResultBytes)
	if err != nil {
		return preparedReviewerResultStaging{}, err
	}
	reviewerResult, err := validateReviewerResultCandidate(repoRoot, caseRoot, packet, shardID, source)
	if err != nil {
		return preparedReviewerResultStaging{}, err
	}
	if expected := strings.TrimSpace(opt.ExpectedSourceSHA256); expected != "" && !strings.EqualFold(expected, sha256Hex(source)) {
		return preparedReviewerResultStaging{}, fmt.Errorf("reviewer result staging source changed after preview")
	}
	existing, present, err := existingReviewerResultCandidateAnchored(caseRoot, namespace.ResultRoot, expectedCandidatePath)
	if err != nil {
		return preparedReviewerResultStaging{}, err
	}
	alreadyStaged := present && bytes.Equal(existing, source)
	if present && !alreadyStaged {
		return preparedReviewerResultStaging{}, fmt.Errorf("canonical reviewer result candidate %q already contains different bytes; refusing overwrite", expectedCandidatePath)
	}
	return preparedReviewerResultStaging{
		packetPath:     packetPath,
		packet:         packet,
		packetSnapshot: packetRootID,
		resultRootID:   resultRootID,
		handoff:        handoff,
		sourcePath:     sourcePath,
		source:         source,
		result:         reviewerResult,
		alreadyStaged:  alreadyStaged,
		lane:           lane,
		actor:          actor,
	}, nil
}

func readReviewerPacketAndIntegrityAnchored(caseRoot, packetPath string) ([]byte, reviewerPacketAnchoredSnapshot, error) {
	root, err := os.OpenRoot(caseRoot)
	if err != nil {
		return nil, reviewerPacketAnchoredSnapshot{}, err
	}
	defer root.Close()
	parentRel, err := filepath.Rel(caseRoot, filepath.Dir(packetPath))
	if err != nil || filepath.IsAbs(parentRel) || parentRel == ".." || strings.HasPrefix(parentRel, ".."+string(filepath.Separator)) {
		return nil, reviewerPacketAnchoredSnapshot{}, fmt.Errorf("review packet escapes attached case")
	}
	parent, err := openReviewerDirectoryNoFollow(root, parentRel)
	if err != nil {
		return nil, reviewerPacketAnchoredSnapshot{}, fmt.Errorf("read review packet: %w", err)
	}
	defer parent.Close()
	packet, err := readStableReviewerArtifactFromRoot(parent, filepath.Base(packetPath), "review packet", maxReviewPacketBytes)
	if err != nil {
		return nil, reviewerPacketAnchoredSnapshot{}, err
	}
	parentID, err := parent.Lstat(".")
	if err != nil {
		return nil, reviewerPacketAnchoredSnapshot{}, err
	}
	integrity, _, err := readOptionalReviewerArtifactFromRoot(parent, "packet.integrity.json", "review packet integrity", maxReviewPacketBytes)
	if err != nil {
		return nil, reviewerPacketAnchoredSnapshot{}, err
	}
	if !reviewerDirectoryPathMatches(caseRoot, parentRel, parent) {
		return nil, reviewerPacketAnchoredSnapshot{}, fmt.Errorf("review packet namespace changed while reading")
	}
	return packet, reviewerPacketAnchoredSnapshot{parentID: parentID, packet: packet, integrity: integrity}, nil
}

func reviewerPacketSnapshotCurrent(caseRoot, packetPath string, expected reviewerPacketAnchoredSnapshot) bool {
	packet, current, err := readReviewerPacketAndIntegrityAnchored(caseRoot, packetPath)
	return err == nil && expected.parentID != nil && os.SameFile(current.parentID, expected.parentID) &&
		bytes.Equal(packet, expected.packet) && bytes.Equal(current.integrity, expected.integrity)
}

func readOptionalReviewerArtifactFromRoot(root *os.Root, name, label string, limit int64) ([]byte, bool, error) {
	if _, err := root.Lstat(name); os.IsNotExist(err) {
		return nil, false, nil
	} else if err != nil {
		return nil, false, err
	}
	data, err := readStableReviewerArtifactFromRoot(root, name, label, limit)
	return data, err == nil, err
}

func readStableReviewerArtifactFromRoot(root *os.Root, name, label string, limit int64) ([]byte, error) {
	before, err := root.Lstat(name)
	if err != nil {
		return nil, fmt.Errorf("read %s: %w", label, err)
	}
	if !before.Mode().IsRegular() || before.Size() == 0 {
		return nil, fmt.Errorf("%s must be a non-empty regular file: %s", label, name)
	}
	file, err := root.Open(name)
	if err != nil {
		return nil, fmt.Errorf("read %s: %w", label, err)
	}
	defer file.Close()
	opened, err := file.Stat()
	if err != nil || !opened.Mode().IsRegular() || !os.SameFile(before, opened) {
		return nil, fmt.Errorf("%s changed while opening: %s", label, name)
	}
	data, err := io.ReadAll(io.LimitReader(file, limit+1))
	if err != nil {
		return nil, fmt.Errorf("read %s: %w", label, err)
	}
	if int64(len(data)) > limit {
		return nil, fmt.Errorf("%s exceeds %d-byte limit", label, limit)
	}
	after, err := root.Lstat(name)
	if err != nil || !os.SameFile(opened, after) {
		return nil, fmt.Errorf("%s changed while reading: %s", label, name)
	}
	return data, nil
}

func validatePacketIntegrityBytes(packetPath string, packet Packet, packetBytes, integrityBytes []byte) error {
	integrityPath := filepath.Join(filepath.Dir(packetPath), "packet.integrity.json")
	if packet.PacketIntegrity == nil {
		if len(integrityBytes) != 0 {
			return fmt.Errorf("review packet integrity reference is missing while canonical sidecar exists")
		}
		return nil
	}
	if !strings.EqualFold(strings.TrimSpace(packet.PacketIntegrity.Algorithm), "sha256") || !samePath(packet.PacketIntegrity.Path, integrityPath) {
		return fmt.Errorf("review packet integrity reference is not canonical")
	}
	if len(integrityBytes) == 0 {
		return fmt.Errorf("review packet integrity is missing")
	}
	dec := json.NewDecoder(bytes.NewReader(bytes.TrimSpace(integrityBytes)))
	dec.DisallowUnknownFields()
	var integrity reviewerPacketIntegrity
	if err := dec.Decode(&integrity); err != nil {
		return fmt.Errorf("decode review packet integrity: %w", err)
	}
	var trailing any
	if err := dec.Decode(&trailing); err != io.EOF {
		return fmt.Errorf("review packet integrity must contain exactly one JSON object")
	}
	if integrity.SchemaVersion != 1 || integrity.Kind != "reviewer-packet-integrity" || !strings.EqualFold(integrity.Algorithm, "sha256") || integrity.PacketID != packet.PacketID || integrity.TargetLane != packet.TargetLane || !samePath(integrity.PacketPath, packetPath) || integrity.PacketSHA256 != sha256Hex(packetBytes) || integrity.PacketBytes != len(packetBytes) {
		return fmt.Errorf("review packet integrity does not match packet bytes and bindings")
	}
	return nil
}

func reviewerDirectoryIdentity(caseRoot, path string) (os.FileInfo, error) {
	root, err := os.OpenRoot(caseRoot)
	if err != nil {
		return nil, err
	}
	defer root.Close()
	rel, err := filepath.Rel(caseRoot, path)
	if err != nil || filepath.IsAbs(rel) || rel == ".." || strings.HasPrefix(rel, ".."+string(filepath.Separator)) {
		return nil, fmt.Errorf("reviewer directory escapes attached case: %s", path)
	}
	directory, err := openReviewerDirectoryNoFollow(root, rel)
	if err != nil {
		return nil, err
	}
	defer directory.Close()
	return directory.Lstat(".")
}

func reviewerDirectoryIdentityCurrent(caseRoot, path string, expected os.FileInfo) bool {
	current, err := reviewerDirectoryIdentity(caseRoot, path)
	return err == nil && expected != nil && os.SameFile(current, expected)
}

func readStableReviewerArtifactAnchored(caseRoot, path, label string, limit int64) ([]byte, error) {
	root, err := os.OpenRoot(caseRoot)
	if err != nil {
		return nil, err
	}
	defer root.Close()
	rel, err := filepath.Rel(caseRoot, path)
	if err != nil || filepath.IsAbs(rel) || rel == ".." || strings.HasPrefix(rel, ".."+string(filepath.Separator)) {
		return nil, fmt.Errorf("reviewer artifact path escapes root %q: %s", caseRoot, path)
	}
	directoryRel := filepath.Dir(rel)
	directory, err := openReviewerDirectoryNoFollow(root, directoryRel)
	if err != nil {
		return nil, fmt.Errorf("read %s: %w", label, err)
	}
	defer directory.Close()
	name := filepath.Base(rel)
	before, err := directory.Lstat(name)
	if err != nil {
		return nil, fmt.Errorf("read %s: %w", label, err)
	}
	if !before.Mode().IsRegular() || before.Size() == 0 {
		return nil, fmt.Errorf("%s must be a non-empty regular file: %s", label, path)
	}
	file, err := directory.Open(name)
	if err != nil {
		return nil, fmt.Errorf("read %s: %w", label, err)
	}
	defer file.Close()
	opened, err := file.Stat()
	if err != nil {
		return nil, err
	}
	if !opened.Mode().IsRegular() || !os.SameFile(before, opened) {
		return nil, fmt.Errorf("%s changed while opening: %s", label, path)
	}
	data, err := io.ReadAll(io.LimitReader(file, limit+1))
	if err != nil {
		return nil, fmt.Errorf("read %s: %w", label, err)
	}
	if int64(len(data)) > limit {
		return nil, fmt.Errorf("%s exceeds %d-byte limit", label, limit)
	}
	after, err := directory.Lstat(name)
	if err != nil || !os.SameFile(opened, after) || !reviewerDirectoryPathMatches(caseRoot, directoryRel, directory) {
		return nil, fmt.Errorf("%s changed while reading: %s", label, path)
	}
	return data, nil
}

func openReviewerDirectoryNoFollow(root *os.Root, rel string) (*os.Root, error) {
	clean := filepath.Clean(rel)
	if filepath.IsAbs(clean) || clean == ".." || strings.HasPrefix(clean, ".."+string(filepath.Separator)) {
		return nil, fmt.Errorf("reviewer directory escapes its anchored root: %s", rel)
	}
	current, err := root.OpenRoot(".")
	if err != nil {
		return nil, err
	}
	if clean == "." {
		return current, nil
	}
	for component := range strings.SplitSeq(clean, string(filepath.Separator)) {
		if component == "" || component == "." || component == ".." {
			current.Close()
			return nil, fmt.Errorf("reviewer directory contains an invalid component: %s", rel)
		}
		before, err := current.Lstat(component)
		if err != nil {
			current.Close()
			return nil, err
		}
		if !before.Mode().IsDir() || before.Mode()&os.ModeSymlink != 0 {
			current.Close()
			return nil, fmt.Errorf("reviewer directory component must be a non-symlink directory: %s", component)
		}
		next, err := current.OpenRoot(component)
		if err != nil {
			current.Close()
			return nil, err
		}
		opened, openedErr := next.Lstat(".")
		after, afterErr := current.Lstat(component)
		if openedErr != nil || afterErr != nil || !opened.Mode().IsDir() || !os.SameFile(before, opened) || !os.SameFile(opened, after) {
			next.Close()
			current.Close()
			return nil, fmt.Errorf("reviewer directory component changed while opening: %s", component)
		}
		current.Close()
		current = next
	}
	return current, nil
}

func reviewerDirectoryPathMatches(caseRoot, rel string, opened *os.Root) bool {
	root, err := os.OpenRoot(caseRoot)
	if err != nil {
		return false
	}
	defer root.Close()
	current, err := openReviewerDirectoryNoFollow(root, rel)
	if err != nil {
		return false
	}
	defer current.Close()
	currentInfo, err := current.Lstat(".")
	if err != nil {
		return false
	}
	openedInfo, err := opened.Lstat(".")
	return err == nil && os.SameFile(currentInfo, openedInfo)
}

func existingReviewerResultCandidateAnchored(caseRoot, resultRoot, candidatePath string) ([]byte, bool, error) {
	root, err := os.OpenRoot(caseRoot)
	if err != nil {
		return nil, false, err
	}
	defer root.Close()
	resultRel, err := filepath.Rel(caseRoot, resultRoot)
	if err != nil || filepath.IsAbs(resultRel) || resultRel == ".." || strings.HasPrefix(resultRel, ".."+string(filepath.Separator)) {
		return nil, false, fmt.Errorf("reviewer result root escapes attached case")
	}
	result, err := openReviewerDirectoryNoFollow(root, resultRel)
	if err != nil {
		return nil, false, err
	}
	defer result.Close()
	candidates, err := openReviewerDirectoryNoFollow(result, "candidates")
	if os.IsNotExist(err) {
		return nil, false, nil
	}
	if err != nil {
		return nil, false, err
	}
	defer candidates.Close()
	candidateRel := filepath.Join(resultRel, "candidates")
	if !reviewerDirectoryPathMatches(caseRoot, candidateRel, candidates) {
		return nil, false, fmt.Errorf("reviewer result candidate directory changed while reading")
	}
	data, present, err := existingReviewerResultAnchored(candidates, filepath.Base(candidatePath))
	if err != nil || !present {
		return data, present, err
	}
	if !reviewerDirectoryPathMatches(caseRoot, candidateRel, candidates) {
		return nil, false, fmt.Errorf("reviewer result candidate directory changed while reading")
	}
	return data, true, nil
}

func publishReviewerResultCandidateAnchoredWithHook(caseRoot string, packet Packet, handoff ShardHandoff, data []byte, hook func(string) error) (bool, error) {
	resultRootID, err := reviewerDirectoryIdentity(caseRoot, packet.ReviewerOrchestration.ResultRoot)
	if err != nil {
		return false, err
	}
	packetData, packetSnapshot, err := readReviewerPacketAndIntegrityAnchored(caseRoot, packet.ReviewerOrchestration.PacketPath)
	if err != nil {
		return false, err
	}
	if decoded, err := decodeIntakePacket(packetData); err != nil || decoded.PacketID != packet.PacketID {
		return false, fmt.Errorf("review packet changed before reviewer result publication")
	}
	return publishReviewerResultCandidateAnchoredExpected(caseRoot, packet, handoff, data, packetSnapshot, resultRootID, hook)
}

func publishReviewerResultCandidateAnchoredExpected(caseRoot string, packet Packet, handoff ShardHandoff, data []byte, expectedPacket reviewerPacketAnchoredSnapshot, expectedResultRoot os.FileInfo, hook func(string) error) (bool, error) {
	root, err := os.OpenRoot(caseRoot)
	if err != nil {
		return false, err
	}
	defer root.Close()
	resultRel, err := filepath.Rel(caseRoot, packet.ReviewerOrchestration.ResultRoot)
	if err != nil || filepath.IsAbs(resultRel) || resultRel == ".." || strings.HasPrefix(resultRel, ".."+string(filepath.Separator)) {
		return false, fmt.Errorf("reviewer result root escapes attached case")
	}
	resultRoot, err := openReviewerDirectoryNoFollow(root, resultRel)
	if err != nil {
		return false, err
	}
	defer resultRoot.Close()
	openedResultRoot, err := resultRoot.Lstat(".")
	if err != nil || expectedResultRoot == nil || !os.SameFile(openedResultRoot, expectedResultRoot) {
		return false, fmt.Errorf("reviewer result namespace changed after staging validation")
	}
	if err := resultRoot.Mkdir("candidates", 0o755); err != nil && !os.IsExist(err) {
		return false, err
	}
	candidateRoot, err := openReviewerDirectoryNoFollow(resultRoot, "candidates")
	if err != nil {
		return false, err
	}
	defer candidateRoot.Close()
	if !reviewerDirectoryPathMatches(caseRoot, filepath.Join(resultRel, "candidates"), candidateRoot) ||
		!reviewerDirectoryIdentityCurrent(caseRoot, packet.ReviewerOrchestration.ResultRoot, expectedResultRoot) {
		return false, fmt.Errorf("reviewer result candidate directory changed while publishing")
	}
	if hook != nil {
		if err := hook("candidate-directory-opened"); err != nil {
			return false, err
		}
	}
	name := filepath.Base(handoff.ReviewerResultCandidatePath)
	if name != handoff.ShardID+".json" {
		return false, fmt.Errorf("reviewer result candidate leaf is not canonical")
	}
	if existing, present, readErr := existingReviewerResultAnchored(candidateRoot, name); readErr != nil || present {
		if readErr != nil {
			return false, readErr
		}
		if bytes.Equal(existing, data) {
			if !reviewerDirectoryPathMatches(caseRoot, filepath.Join(resultRel, "candidates"), candidateRoot) ||
				!reviewerDirectoryIdentityCurrent(caseRoot, packet.ReviewerOrchestration.ResultRoot, expectedResultRoot) {
				return false, fmt.Errorf("reviewer result candidate directory changed while publishing")
			}
			if !reviewerPacketSnapshotCurrent(caseRoot, packet.ReviewerOrchestration.PacketPath, expectedPacket) {
				return false, fmt.Errorf("review packet changed while verifying staged reviewer result candidate")
			}
			return true, nil
		}
		return false, fmt.Errorf("canonical reviewer result candidate %q already contains different bytes; refusing overwrite", handoff.ReviewerResultCandidatePath)
	}
	random := make([]byte, 16)
	if _, err := rand.Read(random); err != nil {
		return false, err
	}
	tmpName := ".reviewer-result-" + hex.EncodeToString(random) + ".tmp"
	tmp, err := candidateRoot.OpenFile(tmpName, os.O_CREATE|os.O_EXCL|os.O_WRONLY, 0o644)
	if err != nil {
		return false, err
	}
	defer candidateRoot.Remove(tmpName)
	if _, err := tmp.Write(data); err != nil {
		tmp.Close()
		return false, err
	}
	if err := tmp.Sync(); err != nil {
		tmp.Close()
		return false, err
	}
	if err := tmp.Close(); err != nil {
		return false, err
	}
	if !reviewerDirectoryPathMatches(caseRoot, filepath.Join(resultRel, "candidates"), candidateRoot) ||
		!reviewerDirectoryIdentityCurrent(caseRoot, packet.ReviewerOrchestration.ResultRoot, expectedResultRoot) {
		return false, fmt.Errorf("reviewer result candidate directory changed while publishing")
	}
	if hook != nil {
		if err := hook("before-candidate-link"); err != nil {
			return false, err
		}
	}
	if err := candidateRoot.Link(tmpName, name); err != nil {
		if existing, present, readErr := existingReviewerResultAnchored(candidateRoot, name); readErr == nil && present {
			if bytes.Equal(existing, data) {
				if !reviewerDirectoryPathMatches(caseRoot, filepath.Join(resultRel, "candidates"), candidateRoot) ||
					!reviewerDirectoryIdentityCurrent(caseRoot, packet.ReviewerOrchestration.ResultRoot, expectedResultRoot) {
					return false, fmt.Errorf("reviewer result candidate directory changed while publishing")
				}
				if !reviewerPacketSnapshotCurrent(caseRoot, packet.ReviewerOrchestration.PacketPath, expectedPacket) {
					return false, fmt.Errorf("review packet changed while verifying staged reviewer result candidate")
				}
				return true, nil
			}
			return false, fmt.Errorf("canonical reviewer result candidate %q already contains different bytes; refusing overwrite", handoff.ReviewerResultCandidatePath)
		}
		return false, fmt.Errorf("publish reviewer result candidate without replacement: %w", err)
	}
	if hook != nil {
		if err := hook("candidate-linked"); err != nil {
			removePublishedReviewerResultLink(candidateRoot, tmpName, name)
			return false, err
		}
	}
	published, present, err := existingReviewerResultAnchored(candidateRoot, name)
	if err != nil || !present || !bytes.Equal(published, data) {
		removePublishedReviewerResultLink(candidateRoot, tmpName, name)
		if err != nil {
			return false, err
		}
		return false, fmt.Errorf("verify published reviewer result candidate: published bytes do not match source")
	}
	if !reviewerDirectoryPathMatches(caseRoot, filepath.Join(resultRel, "candidates"), candidateRoot) ||
		!reviewerDirectoryIdentityCurrent(caseRoot, packet.ReviewerOrchestration.ResultRoot, expectedResultRoot) {
		removePublishedReviewerResultLink(candidateRoot, tmpName, name)
		return false, fmt.Errorf("reviewer result candidate directory changed while publishing")
	}
	if !reviewerPacketSnapshotCurrent(caseRoot, packet.ReviewerOrchestration.PacketPath, expectedPacket) {
		removePublishedReviewerResultLink(candidateRoot, tmpName, name)
		return false, fmt.Errorf("review packet changed while publishing reviewer result candidate")
	}
	return false, nil
}

func removePublishedReviewerResultLink(root *os.Root, tmpName, name string) {
	tmpInfo, tmpErr := root.Lstat(tmpName)
	publishedInfo, publishedErr := root.Lstat(name)
	if tmpErr == nil && publishedErr == nil && os.SameFile(tmpInfo, publishedInfo) {
		_ = root.Remove(name)
	}
}

func existingReviewerResultAnchored(root *os.Root, name string) ([]byte, bool, error) {
	before, err := root.Lstat(name)
	if os.IsNotExist(err) {
		return nil, false, nil
	}
	if err != nil {
		return nil, false, err
	}
	if !before.Mode().IsRegular() || before.Size() == 0 {
		return nil, false, fmt.Errorf("reviewer result candidate must be a non-empty regular file: %s", name)
	}
	file, err := root.Open(name)
	if err != nil {
		return nil, false, err
	}
	defer file.Close()
	opened, err := file.Stat()
	if err != nil || !opened.Mode().IsRegular() || !os.SameFile(before, opened) {
		return nil, false, fmt.Errorf("reviewer result candidate changed while opening: %s", name)
	}
	data, err := io.ReadAll(io.LimitReader(file, maxReviewerResultBytes+1))
	if err != nil {
		return nil, false, err
	}
	if int64(len(data)) > maxReviewerResultBytes {
		return nil, false, fmt.Errorf("reviewer result candidate exceeds %d-byte limit", maxReviewerResultBytes)
	}
	after, err := root.Lstat(name)
	if err != nil || !os.SameFile(opened, after) {
		return nil, false, fmt.Errorf("reviewer result candidate changed while reading: %s", name)
	}
	return data, true, nil
}

func newReviewerResultStagingResult(repoRoot, caseRoot, pack string, opt ReviewerResultStagingOptions, prepared preparedReviewerResultStaging) ReviewerResultStagingResult {
	return ReviewerResultStagingResult{
		SchemaVersion:        1,
		Command:              commandName,
		Mode:                 "reviewer-result-staging",
		CaseRoot:             caseRoot,
		RepoRoot:             repoRoot,
		Pack:                 pack,
		IsMutation:           !opt.WhatIf,
		RequiresConfirmation: true,
		PacketID:             prepared.packet.PacketID,
		PacketPath:           prepared.packetPath,
		ShardID:              prepared.handoff.ShardID,
		Lane:                 prepared.lane,
		Actor:                prepared.actor,
		SourcePath:           prepared.sourcePath,
		SourceSHA256:         sha256Hex(prepared.source),
		SourceBytes:          len(prepared.source),
		CandidatePath:        prepared.handoff.ReviewerResultCandidatePath,
		AlreadyStaged:        prepared.alreadyStaged,
		ReviewerResult:       prepared.result,
		Boundary: []string{
			"staging validates one symlink-free case-local regular source against the packet shard and publishes exact bytes only to the packet-derived candidate path",
			"staging never overwrites a different candidate; exact replay is idempotent and the source remains unchanged",
			"runtime does not spawn, stop, poll, monitor, or manage reviewer sessions",
			"staging does not append facts, execute heavy tools, modify managed/project source files, or write authority/confirmed state",
			"immutable collection and reviewer intake remain separate -WhatIf then explicit -Apply operations",
		},
	}
}

func finalizeReviewerResultStagingResult(result ReviewerResultStagingResult) ReviewerResultStagingResult {
	result.MissionCommanderNextActions = []mission.MissionCommanderNextActionItem{{
		Lane:           result.Lane,
		Label:          result.PacketID,
		ActionID:       result.PacketID + ":" + result.ShardID,
		State:          result.MissionCommanderAction.State,
		Command:        result.MissionCommanderAction.PrimaryCommand,
		Source:         "reviewerResultStaging",
		RequiresReview: true,
		Reasons:        append([]string{}, result.NextSteps...),
		Boundary:       append([]string{}, result.Boundary...),
	}}
	result.MissionCommanderActionQueue = mission.MissionCommanderActionQueueFor(result.MissionCommanderNextActions)
	return result
}

func reviewerResultStagingCommand(packetPath, shardID, sourcePath, lane, actor, expectedSourceSHA256 string, apply bool) string {
	command := "/rekit plan-subagents -PacketPath " + quoteCommandArg(packetPath) +
		" -StageReviewerResult -ShardId " + quoteCommandArg(shardID) +
		" -ReviewerResultSourcePath " + quoteCommandArg(sourcePath) +
		" -Lane " + quoteCommandArg(lane) +
		" -Actor " + quoteCommandArg(actor)
	if apply {
		return command + " -ExpectedSourceSha256 " + quoteCommandArg(expectedSourceSHA256) + " -Apply -Format json"
	}
	return command + " -WhatIf -Format json"
}
