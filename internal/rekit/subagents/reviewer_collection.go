package subagents

import (
	"bytes"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"

	"github.com/shuiyu486/re-context-kits/internal/rekit/instance"
	"github.com/shuiyu486/re-context-kits/internal/rekit/mission"
)

type ReviewerResultCollectionOptions struct {
	PacketPath string
	ShardID    string
	Lane       string
	Actor      string
	WhatIf     bool
}

type ReviewerResultCollectionResult struct {
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
	ReviewerSession             string                                   `json:"reviewerSession"`
	CandidatePath               string                                   `json:"candidatePath"`
	CandidateSHA256             string                                   `json:"candidateSha256"`
	CandidateBytes              int                                      `json:"candidateBytes"`
	ReviewerResultPath          string                                   `json:"reviewerResultPath"`
	ReviewerResultSHA256        string                                   `json:"reviewerResultSha256,omitempty"`
	AlreadyCollected            bool                                     `json:"alreadyCollected"`
	ReviewerResult              ReviewerResult                           `json:"reviewerResult"`
	MissionCommanderAction      mission.MissionCommanderAction           `json:"missionCommanderAction"`
	MissionCommanderNextActions []mission.MissionCommanderNextActionItem `json:"missionCommanderNextActions"`
	MissionCommanderActionQueue mission.MissionCommanderActionQueue      `json:"missionCommanderActionQueue"`
	NextSteps                   []string                                 `json:"nextSteps"`
	Boundary                    []string                                 `json:"boundary"`
}

type preparedReviewerResultCollection struct {
	packetPath       string
	packetData       []byte
	packet           Packet
	handoff          ShardHandoff
	candidate        []byte
	result           ReviewerResult
	alreadyCollected bool
	lane             string
	actor            string
}

func CollectReviewerResult(repoRoot, caseRoot, pack string, opt ReviewerResultCollectionOptions) (ReviewerResultCollectionResult, error) {
	inst, err := instance.AssertAttached(caseRoot, repoRoot, pack)
	if err != nil {
		return ReviewerResultCollectionResult{}, err
	}
	caseRoot = inst.CaseRoot
	prepared, err := prepareReviewerResultCollection(repoRoot, caseRoot, pack, opt)
	if err != nil {
		return ReviewerResultCollectionResult{}, err
	}
	result := newReviewerResultCollectionResult(repoRoot, caseRoot, pack, opt, prepared)
	if opt.WhatIf {
		result.Status = "previewed"
		result.AlreadyCollected = prepared.alreadyCollected
		if prepared.alreadyCollected {
			result.Status = "already-collected"
			result.ReviewerResultSHA256 = result.CandidateSHA256
			result.NextSteps = []string{"the exact candidate bytes are already canonical; run packet-level ready reviewer results intake with -WhatIf"}
		} else {
			result.NextSteps = []string{"inspect the exact candidate/result bindings, then rerun the same reviewer result collection with -Apply"}
		}
		if prepared.alreadyCollected {
			batchPreview := strings.TrimSpace(prepared.packet.ReviewerOrchestration.BatchPreviewCommand)
			if batchPreview == "" {
				batchPreview = reviewerPacketBatchPreviewCommand(prepared.packetPath, prepared.lane, prepared.actor)
			}
			result.MissionCommanderAction = mission.MissionCommanderAction{State: "reviewer-result-collected-ready-for-batch-intake-preview", PrimaryCommand: batchPreview, Boundary: result.Boundary}
		} else {
			result.MissionCommanderAction = mission.MissionCommanderAction{
				State:          "ready-for-reviewer-result-collection-apply",
				PrimaryCommand: reviewerResultCollectionCommand(prepared.packetPath, prepared.handoff.ShardID, prepared.lane, prepared.actor, true),
				Boundary:       result.Boundary,
			}
		}
		return finalizeReviewerResultCollectionResult(result), nil
	}

	unlock, err := acquireReviewerIntakeLock(caseRoot, "reviewer-collection-"+prepared.packet.PacketID+"-"+prepared.handoff.ShardID)
	if err != nil {
		return ReviewerResultCollectionResult{}, err
	}
	defer unlock()
	prepared, err = prepareReviewerResultCollection(repoRoot, caseRoot, pack, opt)
	if err != nil {
		return ReviewerResultCollectionResult{}, err
	}
	result = newReviewerResultCollectionResult(repoRoot, caseRoot, pack, opt, prepared)
	already, err := publishReviewerResult(prepared.packet.ReviewerOrchestration.ResultRoot, prepared.handoff.ReviewerResultPath, prepared.candidate)
	if err != nil {
		return ReviewerResultCollectionResult{}, err
	}
	result.Applied = true
	result.AlreadyCollected = already
	result.ReviewerResultSHA256 = result.CandidateSHA256
	if already {
		result.Status = "already-collected"
	} else {
		result.Status = "collected"
	}
	if _, _, ownerErr := validateOwnerBinding(caseRoot, prepared.packet, prepared.packetPath, prepared.packetData); ownerErr != nil {
		result.NextSteps = []string{"the immutable reviewer result is collected; adopt the stale reviewer packet with -WhatIf before -Apply, then run packet-level batch intake"}
		result.MissionCommanderAction = mission.MissionCommanderAction{
			State:          "reviewer-result-collected-owner-adoption-required",
			PrimaryCommand: reviewerPacketAdoptionCommand(prepared.packetPath, prepared.lane, prepared.actor, "replacement executor accepts collected reviewer result", false),
			Boundary:       result.Boundary,
		}
	} else {
		batchPreview := prepared.packet.ReviewerOrchestration.BatchPreviewCommand
		if strings.TrimSpace(batchPreview) == "" {
			batchPreview = reviewerPacketBatchPreviewCommand(prepared.packetPath, prepared.lane, prepared.actor)
		}
		result.NextSteps = []string{"run packet-level ready reviewer results intake with -WhatIf, inspect every ready shard, then use its explicit -Apply command"}
		result.MissionCommanderAction = mission.MissionCommanderAction{
			State:          "reviewer-result-collected-ready-for-batch-intake-preview",
			PrimaryCommand: batchPreview,
			Boundary:       result.Boundary,
		}
	}
	return finalizeReviewerResultCollectionResult(result), nil
}

func prepareReviewerResultCollection(repoRoot, caseRoot, pack string, opt ReviewerResultCollectionOptions) (preparedReviewerResultCollection, error) {
	packetPath, err := requiredAbsolutePath(opt.PacketPath, "review packet")
	if err != nil {
		return preparedReviewerResultCollection{}, err
	}
	packetData, err := readStableReviewerArtifact(filepath.Dir(packetPath), packetPath, "review packet", maxReviewPacketBytes)
	if err != nil {
		return preparedReviewerResultCollection{}, err
	}
	packet, err := decodeIntakePacket(packetData)
	if err != nil {
		return preparedReviewerResultCollection{}, err
	}
	if err := validateIntakePacket(packet, packetPath, repoRoot, caseRoot, pack); err != nil {
		return preparedReviewerResultCollection{}, err
	}
	if err := validateIntakePacketRoute(repoRoot, pack, packet); err != nil {
		return preparedReviewerResultCollection{}, err
	}
	lane := strings.TrimSpace(opt.Lane)
	actor := strings.TrimSpace(opt.Actor)
	shardID := strings.TrimSpace(opt.ShardID)
	if lane == "" || actor == "" || shardID == "" {
		return preparedReviewerResultCollection{}, fmt.Errorf("reviewer result collection requires -PacketPath, -ShardId, -Lane, and -Actor")
	}
	if lane != packet.TargetLane {
		return preparedReviewerResultCollection{}, fmt.Errorf("reviewer result collection lane %q does not match packet targetLane %q", lane, packet.TargetLane)
	}
	handoff, ok := shardHandoffByID(packet.ShardHandoffs, shardID)
	if !ok {
		return preparedReviewerResultCollection{}, fmt.Errorf("reviewer result collection shard %q is not present in packet", shardID)
	}
	candidatePath, err := requiredAbsolutePath(handoff.ReviewerResultCandidatePath, "reviewer result candidate")
	if err != nil {
		return preparedReviewerResultCollection{}, fmt.Errorf("review packet shard %s does not provide a strict reviewer result candidate path: %w", shardID, err)
	}
	resultPath, err := requiredAbsolutePath(handoff.ReviewerResultPath, "reviewer result")
	if err != nil {
		return preparedReviewerResultCollection{}, err
	}
	resultRoot, err := requiredAbsolutePath(packet.ReviewerOrchestration.ResultRoot, "reviewer result root")
	if err != nil {
		return preparedReviewerResultCollection{}, err
	}
	reviewRoot := filepath.Dir(packetPath)
	caseReviewRoot := filepath.Join(caseRoot, ".rekit", "reviews")
	expectedResultRoot := filepath.Join(reviewRoot, "results")
	expectedCandidatePath := filepath.Join(expectedResultRoot, "candidates", shardID+".json")
	expectedResultPath := filepath.Join(expectedResultRoot, shardID+".json")
	if !pathInside(caseReviewRoot, packetPath) || !samePath(packetPath, filepath.Join(reviewRoot, "packet.json")) || !samePath(resultRoot, expectedResultRoot) || !samePath(candidatePath, expectedCandidatePath) || !samePath(resultPath, expectedResultPath) {
		return preparedReviewerResultCollection{}, fmt.Errorf("reviewer result collection paths must match the canonical case review namespace for shard %q", shardID)
	}
	if !samePath(packet.Observability.ReviewRoot, reviewRoot) || !samePath(packet.Observability.ResultRoot, expectedResultRoot) || !samePath(packet.ReviewerOrchestration.PacketPath, packetPath) || !samePath(handoff.ReviewerResultCandidatePath, expectedCandidatePath) || !samePath(handoff.ReviewerResultPath, expectedResultPath) {
		return preparedReviewerResultCollection{}, fmt.Errorf("review packet observability/orchestration paths do not match canonical collection paths for shard %q", shardID)
	}
	candidate, err := readStableReviewerArtifact(resultRoot, candidatePath, "reviewer result candidate", maxReviewerResultBytes)
	if err != nil {
		return preparedReviewerResultCollection{}, err
	}
	reviewerResult, err := decodeReviewerResult(candidate)
	if err != nil {
		return preparedReviewerResultCollection{}, err
	}
	if reviewerResult.PacketID != packet.PacketID {
		return preparedReviewerResultCollection{}, fmt.Errorf("reviewer result packetId %q does not match packet %q", reviewerResult.PacketID, packet.PacketID)
	}
	if reviewerResult.RouteID != packet.Route.ID {
		return preparedReviewerResultCollection{}, fmt.Errorf("reviewer result routeId %q does not match packet route %q", reviewerResult.RouteID, packet.Route.ID)
	}
	if reviewerResult.ShardID != shardID {
		return preparedReviewerResultCollection{}, fmt.Errorf("reviewer result shard %q does not match expected packet handoff shard %q", reviewerResult.ShardID, shardID)
	}
	shard, ok := shardByID(packet.Shards, shardID)
	if !ok || !slicesEqual(reviewerResult.Items, shard.Items) {
		return preparedReviewerResultCollection{}, fmt.Errorf("reviewer result items do not match packet shard %s: got %v, want %v", shardID, reviewerResult.Items, shard.Items)
	}
	if err := validateRouteOutput(packet.OutputContract, reviewerResult.RouteOutput); err != nil {
		return preparedReviewerResultCollection{}, err
	}
	if err := validateRouteOutputBindings(reviewerResult); err != nil {
		return preparedReviewerResultCollection{}, err
	}
	mapping, ok := reviewerDecisionMappingByDecision(reviewerResult.Decision)
	if !ok {
		return preparedReviewerResultCollection{}, fmt.Errorf("invalid reviewer decision %q", reviewerResult.Decision)
	}
	blocked, err := reviewerIntakeBlockers(repoRoot, caseRoot, packet, reviewerResult, mapping)
	if err != nil {
		return preparedReviewerResultCollection{}, err
	}
	if len(blocked) > 0 {
		return preparedReviewerResultCollection{}, fmt.Errorf("reviewer result candidate is not ready for immutable collection: %s", strings.Join(blocked, "; "))
	}
	alreadyCollected, err := preflightReviewerResultTarget(resultRoot, resultPath, candidate)
	if err != nil {
		return preparedReviewerResultCollection{}, err
	}
	return preparedReviewerResultCollection{packetPath: packetPath, packetData: packetData, packet: packet, handoff: handoff, candidate: candidate, result: reviewerResult, alreadyCollected: alreadyCollected, lane: lane, actor: actor}, nil
}

func newReviewerResultCollectionResult(repoRoot, caseRoot, pack string, opt ReviewerResultCollectionOptions, prepared preparedReviewerResultCollection) ReviewerResultCollectionResult {
	boundary := []string{
		"collection validates one case-local candidate and publishes its exact bytes only to the immutable packet-derived reviewerResultPath",
		"collection never overwrites a different canonical reviewer result; exact replay is idempotent",
		"runtime does not spawn, stop, poll, monitor, or manage reviewer sessions",
		"collection does not append facts, execute heavy tools, modify managed/project source files, or write authority/confirmed state",
		"reviewer intake remains a separate packet-level -WhatIf then explicit -Apply operation",
	}
	return ReviewerResultCollectionResult{
		SchemaVersion:        1,
		Command:              commandName,
		Mode:                 "reviewer-result-collection",
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
		ReviewerSession:      prepared.result.ReviewerSession,
		CandidatePath:        prepared.handoff.ReviewerResultCandidatePath,
		CandidateSHA256:      sha256Hex(prepared.candidate),
		CandidateBytes:       len(prepared.candidate),
		ReviewerResultPath:   prepared.handoff.ReviewerResultPath,
		ReviewerResult:       prepared.result,
		Boundary:             boundary,
	}
}

func finalizeReviewerResultCollectionResult(result ReviewerResultCollectionResult) ReviewerResultCollectionResult {
	result.MissionCommanderNextActions = []mission.MissionCommanderNextActionItem{{
		Lane:           result.Lane,
		Label:          result.PacketID,
		ActionID:       result.PacketID + ":" + result.ShardID,
		State:          result.MissionCommanderAction.State,
		Command:        result.MissionCommanderAction.PrimaryCommand,
		Source:         "reviewerResultCollection",
		RequiresReview: true,
		Reasons:        append([]string{}, result.NextSteps...),
		Boundary:       append([]string{}, result.Boundary...),
	}}
	result.MissionCommanderActionQueue = mission.MissionCommanderActionQueueFor(result.MissionCommanderNextActions)
	return result
}

func reviewerResultCollectionCommand(packetPath, shardID, lane, actor string, apply bool) string {
	command := "/rekit plan-subagents -PacketPath " + quoteCommandArg(packetPath) + " -CollectReviewerResult -ShardId " + quoteCommandArg(shardID) + " -Lane " + quoteCommandArg(lane) + " -Actor " + quoteCommandArg(actor)
	if apply {
		return command + " -Apply -Format json"
	}
	return command + " -WhatIf -Format json"
}

func shardHandoffByID(handoffs []ShardHandoff, shardID string) (ShardHandoff, bool) {
	for _, handoff := range handoffs {
		if handoff.ShardID == shardID {
			return handoff, true
		}
	}
	return ShardHandoff{}, false
}

func readStableReviewerArtifact(root, path, label string, limit int64) ([]byte, error) {
	if err := rejectReviewerArtifactSymlinkPath(root, path, false); err != nil {
		return nil, err
	}
	before, err := os.Lstat(path)
	if err != nil {
		return nil, fmt.Errorf("read %s: %w", label, err)
	}
	if !before.Mode().IsRegular() || before.Size() == 0 {
		return nil, fmt.Errorf("%s must be a non-empty regular file: %s", label, path)
	}
	file, err := os.Open(path)
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
	after, err := os.Lstat(path)
	if err != nil || !os.SameFile(opened, after) {
		return nil, fmt.Errorf("%s changed while reading: %s", label, path)
	}
	return data, nil
}

func rejectReviewerArtifactSymlinkPath(root, path string, allowMissing bool) error {
	rootFull, err := filepath.Abs(root)
	if err != nil {
		return err
	}
	pathFull, err := filepath.Abs(path)
	if err != nil {
		return err
	}
	rel, err := filepath.Rel(rootFull, pathFull)
	if err != nil || rel == ".." || strings.HasPrefix(rel, ".."+string(filepath.Separator)) || filepath.IsAbs(rel) {
		return fmt.Errorf("reviewer artifact path escapes root %q: %s", rootFull, path)
	}
	current := rootFull
	parts := append([]string{"."}, strings.Split(rel, string(filepath.Separator))...)
	for _, part := range parts {
		if part != "." && part != "" {
			current = filepath.Join(current, part)
		}
		st, statErr := os.Lstat(current)
		if os.IsNotExist(statErr) && allowMissing {
			return nil
		}
		if statErr != nil {
			return statErr
		}
		if st.Mode()&os.ModeSymlink != 0 {
			return fmt.Errorf("reviewer artifact path must not traverse symlink: %s", current)
		}
	}
	return nil
}

func preflightReviewerResultTarget(root, path string, data []byte) (bool, error) {
	existing, exists, err := existingReviewerResult(root, path)
	if err != nil {
		return false, err
	}
	if !exists {
		return false, nil
	}
	if bytes.Equal(existing, data) {
		return true, nil
	}
	return false, fmt.Errorf("canonical reviewer result %q already contains different bytes; refusing overwrite", path)
}

func publishReviewerResult(root, path string, data []byte) (bool, error) {
	if err := rejectReviewerArtifactSymlinkPath(root, filepath.Dir(path), false); err != nil {
		return false, err
	}
	if already, err := preflightReviewerResultTarget(root, path, data); err != nil || already {
		return already, err
	}
	tmp, err := os.CreateTemp(filepath.Dir(path), ".reviewer-result-*.tmp")
	if err != nil {
		return false, err
	}
	tmpPath := tmp.Name()
	defer os.Remove(tmpPath)
	if err := tmp.Chmod(0o644); err != nil {
		tmp.Close()
		return false, err
	}
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
	if err := rejectReviewerArtifactSymlinkPath(root, path, true); err != nil {
		return false, err
	}
	if err := os.Link(tmpPath, path); err != nil {
		if existing, exists, readErr := existingReviewerResult(root, path); readErr == nil && exists {
			if bytes.Equal(existing, data) {
				return true, nil
			}
			return false, fmt.Errorf("canonical reviewer result %q already contains different bytes; refusing overwrite", path)
		}
		return false, fmt.Errorf("publish canonical reviewer result without replacement: %w", err)
	}
	published, exists, err := existingReviewerResult(root, path)
	if err != nil {
		return false, fmt.Errorf("verify published canonical reviewer result %q: %w", path, err)
	}
	if !exists || !bytes.Equal(published, data) {
		return false, fmt.Errorf("verify published canonical reviewer result %q: published bytes do not match candidate", path)
	}
	return false, nil
}

func existingReviewerResult(root, path string) ([]byte, bool, error) {
	st, err := os.Lstat(path)
	if os.IsNotExist(err) {
		return nil, false, nil
	}
	if err != nil {
		return nil, false, err
	}
	if st.Mode()&os.ModeSymlink != 0 || !st.Mode().IsRegular() || st.Size() == 0 {
		return nil, false, fmt.Errorf("canonical reviewer result must be a non-empty regular file: %s", path)
	}
	data, err := readStableReviewerArtifact(root, path, "canonical reviewer result", maxReviewerResultBytes)
	return data, err == nil, err
}

func slicesEqual(left, right []string) bool {
	if len(left) != len(right) {
		return false
	}
	for idx := range left {
		if left[idx] != right[idx] {
			return false
		}
	}
	return true
}
