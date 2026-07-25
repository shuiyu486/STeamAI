package subagents

import (
	"bytes"
	"crypto/rand"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/shuiyu486/re-context-kits/internal/rekit/instance"
	"github.com/shuiyu486/re-context-kits/internal/rekit/mission"
	"github.com/shuiyu486/re-context-kits/internal/rekit/reviewpath"
)

type ReviewerPromptArtifactRepairOptions struct {
	PacketPath           string
	ShardID              string
	Lane                 string
	Actor                string
	ExpectedPromptSHA256 string
	WhatIf               bool
}

type ReviewerPromptArtifactRepairResult struct {
	SchemaVersion               int                                      `json:"schemaVersion"`
	Command                     string                                   `json:"command"`
	Mode                        string                                   `json:"mode"`
	CaseRoot                    string                                   `json:"caseRoot"`
	RepoRoot                    string                                   `json:"repoRoot"`
	Pack                        string                                   `json:"pack"`
	IsMutation                  bool                                     `json:"isMutation"`
	Applied                     bool                                     `json:"applied"`
	AlreadyCurrent              bool                                     `json:"alreadyCurrent"`
	RequiresConfirmation        bool                                     `json:"requiresConfirmation"`
	Status                      string                                   `json:"status"`
	PacketID                    string                                   `json:"packetId"`
	PacketPath                  string                                   `json:"packetPath"`
	ShardID                     string                                   `json:"shardId"`
	Lane                        string                                   `json:"lane"`
	Actor                       string                                   `json:"actor"`
	PromptPath                  string                                   `json:"promptPath"`
	PromptSHA256                string                                   `json:"promptSha256"`
	PromptBytes                 int                                      `json:"promptBytes"`
	ExistingPromptState         string                                   `json:"existingPromptState"`
	ExistingPromptSHA256        string                                   `json:"existingPromptSha256,omitempty"`
	ExistingPromptBytes         int                                      `json:"existingPromptBytes,omitempty"`
	ApplyCommand                string                                   `json:"applyCommand,omitempty"`
	MissionCommanderAction      mission.MissionCommanderAction           `json:"missionCommanderAction"`
	MissionCommanderNextActions []mission.MissionCommanderNextActionItem `json:"missionCommanderNextActions"`
	MissionCommanderActionQueue mission.MissionCommanderActionQueue      `json:"missionCommanderActionQueue"`
	NextSteps                   []string                                 `json:"nextSteps"`
	Boundary                    []string                                 `json:"boundary"`
}

type preparedReviewerPromptArtifactRepair struct {
	packetPath          string
	packet              Packet
	packetSnapshot      reviewerPacketAnchoredSnapshot
	handoff             ShardHandoff
	promptPath          string
	prompt              []byte
	existingPromptState string
	existingPrompt      []byte
	lane                string
	actor               string
}

func RepairReviewerPromptArtifact(repoRoot, caseRoot, pack string, opt ReviewerPromptArtifactRepairOptions) (ReviewerPromptArtifactRepairResult, error) {
	inst, err := instance.AssertAttached(caseRoot, repoRoot, pack)
	if err != nil {
		return ReviewerPromptArtifactRepairResult{}, err
	}
	repoRoot, err = filepath.Abs(repoRoot)
	if err != nil {
		return ReviewerPromptArtifactRepairResult{}, err
	}
	if !opt.WhatIf {
		if err := validateReviewerPromptArtifactRepairExpectedHash(opt); err != nil {
			return ReviewerPromptArtifactRepairResult{}, err
		}
	}
	prepared, err := prepareReviewerPromptArtifactRepair(repoRoot, inst.CaseRoot, pack, opt)
	if err != nil {
		return ReviewerPromptArtifactRepairResult{}, err
	}
	result := newReviewerPromptArtifactRepairResult(repoRoot, inst.CaseRoot, pack, opt, prepared)
	if opt.WhatIf {
		return finalizeReviewerPromptArtifactRepairPreview(result, prepared), nil
	}
	if !strings.EqualFold(result.PromptSHA256, strings.TrimSpace(opt.ExpectedPromptSHA256)) {
		return ReviewerPromptArtifactRepairResult{}, fmt.Errorf("reviewer prompt artifact changed after preview")
	}
	if prepared.existingPromptState == "ready" && bytes.Equal(prepared.existingPrompt, prepared.prompt) {
		result.Applied = true
		result.AlreadyCurrent = true
		result.Status = "already-current"
		result.NextSteps = []string{"prompt artifact already matches the packet-derived dispatch prompt; rerun status and dispatch only after promptCurrent=true"}
		result.MissionCommanderAction = mission.MissionCommanderAction{State: "reviewer-prompt-artifact-current", PrimaryCommand: "", Boundary: result.Boundary}
		return finalizeReviewerPromptArtifactRepairResult(result), nil
	}
	if prepared.existingPromptState != "missing" {
		return ReviewerPromptArtifactRepairResult{}, fmt.Errorf("reviewer prompt artifact %q is %s; refusing overwrite", prepared.promptPath, prepared.existingPromptState)
	}
	unlock, lockErr := acquireReviewerIntakeLock(inst.CaseRoot, "reviewer-prompt-"+prepared.packet.PacketID+"-"+prepared.handoff.ShardID)
	if lockErr != nil {
		return ReviewerPromptArtifactRepairResult{}, lockErr
	}
	defer unlock()
	prepared, err = prepareReviewerPromptArtifactRepair(repoRoot, inst.CaseRoot, pack, opt)
	if err != nil {
		return ReviewerPromptArtifactRepairResult{}, err
	}
	result = newReviewerPromptArtifactRepairResult(repoRoot, inst.CaseRoot, pack, opt, prepared)
	if !strings.EqualFold(result.PromptSHA256, strings.TrimSpace(opt.ExpectedPromptSHA256)) {
		return ReviewerPromptArtifactRepairResult{}, fmt.Errorf("reviewer prompt artifact changed after preview")
	}
	if prepared.existingPromptState == "ready" && bytes.Equal(prepared.existingPrompt, prepared.prompt) {
		result.Applied = true
		result.AlreadyCurrent = true
		result.Status = "already-current"
		result.NextSteps = []string{"prompt artifact already matches the packet-derived dispatch prompt; rerun status and dispatch only after promptCurrent=true"}
		result.MissionCommanderAction = mission.MissionCommanderAction{State: "reviewer-prompt-artifact-current", PrimaryCommand: "", Boundary: result.Boundary}
		return finalizeReviewerPromptArtifactRepairResult(result), nil
	}
	if prepared.existingPromptState != "missing" {
		return ReviewerPromptArtifactRepairResult{}, fmt.Errorf("reviewer prompt artifact %q is %s; refusing overwrite", prepared.promptPath, prepared.existingPromptState)
	}
	if err := publishReviewerPromptArtifact(inst.CaseRoot, prepared); err != nil {
		return ReviewerPromptArtifactRepairResult{}, err
	}
	result.Applied = true
	result.Status = "restored"
	result.NextSteps = []string{"rerun status and dispatch the read-only reviewer only after dispatchPromptCurrent=true and promptSha256 matches"}
	result.MissionCommanderAction = mission.MissionCommanderAction{State: "reviewer-prompt-artifact-restored", PrimaryCommand: "", Boundary: result.Boundary}
	return finalizeReviewerPromptArtifactRepairResult(result), nil
}

func prepareReviewerPromptArtifactRepair(repoRoot, caseRoot, pack string, opt ReviewerPromptArtifactRepairOptions) (preparedReviewerPromptArtifactRepair, error) {
	packetPath, err := requiredAbsolutePath(opt.PacketPath, "review packet")
	if err != nil {
		return preparedReviewerPromptArtifactRepair{}, err
	}
	if _, ok := reviewpath.CanonicalCollectionNamespace(caseRoot, packetPath); !ok || !reviewpath.CollectionNamespacePathSafe(caseRoot, packetPath, false) {
		return preparedReviewerPromptArtifactRepair{}, fmt.Errorf("review packet must be a symlink-free canonical case review packet")
	}
	packetData, packetSnapshot, err := readReviewerPacketAndIntegrityAnchored(caseRoot, packetPath)
	if err != nil {
		return preparedReviewerPromptArtifactRepair{}, err
	}
	packet, err := decodeIntakePacket(packetData)
	if err != nil {
		return preparedReviewerPromptArtifactRepair{}, err
	}
	if err := validateIntakePacket(packet, packetPath, repoRoot, caseRoot, pack); err != nil {
		return preparedReviewerPromptArtifactRepair{}, err
	}
	if err := validatePacketIntegrityBytes(packetPath, packet, packetData, packetSnapshot.integrity); err != nil {
		return preparedReviewerPromptArtifactRepair{}, err
	}
	if err := validateIntakePacketRoute(repoRoot, pack, packet); err != nil {
		return preparedReviewerPromptArtifactRepair{}, err
	}
	lane := strings.TrimSpace(opt.Lane)
	actor := strings.TrimSpace(opt.Actor)
	shardID := strings.TrimSpace(opt.ShardID)
	if lane == "" || actor == "" || shardID == "" {
		return preparedReviewerPromptArtifactRepair{}, fmt.Errorf("reviewer prompt artifact repair requires -PacketPath, -ShardId, -Lane, and -Actor")
	}
	if lane != packet.TargetLane {
		return preparedReviewerPromptArtifactRepair{}, fmt.Errorf("reviewer prompt artifact repair lane %q does not match packet targetLane %q", lane, packet.TargetLane)
	}
	handoff, ok := shardHandoffByID(packet.ShardHandoffs, shardID)
	if !ok {
		return preparedReviewerPromptArtifactRepair{}, fmt.Errorf("reviewer prompt artifact repair shard %q is not present in packet", shardID)
	}
	dispatch, ok := reviewerOrchestrationDispatchByID(packet.ReviewerOrchestration.Dispatches, shardID)
	if !ok {
		return preparedReviewerPromptArtifactRepair{}, fmt.Errorf("reviewer prompt artifact repair shard %q is not present in reviewerOrchestration.dispatches", shardID)
	}
	promptPath, expectedSHA256, promptText, err := reviewerPromptArtifactRepairBinding(handoff, dispatch)
	if err != nil {
		return preparedReviewerPromptArtifactRepair{}, err
	}
	if err := validateReviewerPromptArtifactRepairPath(caseRoot, packetPath, shardID, promptPath); err != nil {
		return preparedReviewerPromptArtifactRepair{}, err
	}
	if _, err := hex.DecodeString(expectedSHA256); err != nil || len(expectedSHA256) != sha256.Size*2 {
		return preparedReviewerPromptArtifactRepair{}, fmt.Errorf("reviewer prompt artifact repair requires a valid packet dispatchPromptSha256")
	}
	promptBytes := []byte(strings.TrimRight(promptText, "\r\n") + "\n")
	if !strings.EqualFold(expectedSHA256, sha256Hex(promptBytes)) {
		return preparedReviewerPromptArtifactRepair{}, fmt.Errorf("reviewer prompt artifact repair cannot derive bytes matching packet dispatchPromptSha256")
	}
	state, existing, err := reviewerPromptArtifactExisting(caseRoot, promptPath)
	if err != nil {
		return preparedReviewerPromptArtifactRepair{}, err
	}
	return preparedReviewerPromptArtifactRepair{packetPath: packetPath, packet: packet, packetSnapshot: packetSnapshot, handoff: handoff, promptPath: promptPath, prompt: promptBytes, existingPromptState: state, existingPrompt: existing, lane: lane, actor: actor}, nil
}

func reviewerPromptArtifactRepairBinding(handoff ShardHandoff, dispatch ReviewerDispatch) (string, string, string, error) {
	path := strings.TrimSpace(handoff.DispatchPromptPath)
	expectedSHA256 := strings.TrimSpace(handoff.DispatchPromptSHA256)
	prompt := handoff.DispatchPrompt
	for _, candidate := range []string{dispatch.DispatchPromptPath, promptAgentPath(dispatch.AgentToolRequest), promptAgentPath(handoff.AgentToolRequest)} {
		candidate = strings.TrimSpace(candidate)
		if candidate == "" {
			continue
		}
		if path == "" {
			path = candidate
			continue
		}
		if !samePath(path, candidate) {
			return "", "", "", fmt.Errorf("reviewer prompt artifact path bindings disagree for shard %q", handoff.ShardID)
		}
	}
	for _, candidate := range []string{dispatch.DispatchPromptSHA256, promptAgentSHA256(dispatch.AgentToolRequest), promptAgentSHA256(handoff.AgentToolRequest)} {
		candidate = strings.TrimSpace(candidate)
		if candidate == "" {
			continue
		}
		if expectedSHA256 == "" {
			expectedSHA256 = candidate
			continue
		}
		if !strings.EqualFold(expectedSHA256, candidate) {
			return "", "", "", fmt.Errorf("reviewer prompt artifact sha256 bindings disagree for shard %q", handoff.ShardID)
		}
	}
	for _, candidate := range []string{dispatch.DispatchPrompt, promptAgentPrompt(dispatch.AgentToolRequest), promptAgentPrompt(handoff.AgentToolRequest)} {
		if strings.TrimSpace(candidate) == "" {
			continue
		}
		if strings.TrimSpace(prompt) == "" {
			prompt = candidate
			continue
		}
		if prompt != candidate {
			return "", "", "", fmt.Errorf("reviewer prompt artifact prompt bindings disagree for shard %q", handoff.ShardID)
		}
	}
	if path == "" || expectedSHA256 == "" || strings.TrimSpace(prompt) == "" {
		return "", "", "", fmt.Errorf("reviewer prompt artifact repair requires prompt path, prompt sha256, and packet dispatch prompt for shard %q", handoff.ShardID)
	}
	return path, expectedSHA256, prompt, nil
}

func promptAgentPath(request *ReviewerAgentToolRequest) string {
	if request == nil {
		return ""
	}
	return request.PromptPath
}

func promptAgentSHA256(request *ReviewerAgentToolRequest) string {
	if request == nil {
		return ""
	}
	return request.PromptSHA256
}

func promptAgentPrompt(request *ReviewerAgentToolRequest) string {
	if request == nil {
		return ""
	}
	return request.Prompt
}

func reviewerOrchestrationDispatchByID(dispatches []ReviewerDispatch, shardID string) (ReviewerDispatch, bool) {
	for _, dispatch := range dispatches {
		if dispatch.ShardID == shardID {
			return dispatch, true
		}
	}
	return ReviewerDispatch{}, false
}

func validateReviewerPromptArtifactRepairPath(caseRoot, packetPath, shardID, promptPath string) error {
	reviewRoot := filepath.Dir(packetPath)
	expectedPath := filepath.Join(reviewRoot, "prompts", shardID+".prompt.md")
	if !samePath(promptPath, expectedPath) {
		return fmt.Errorf("reviewer prompt artifact repair path must be the packet-derived prompts/<shard>.prompt.md path")
	}
	if !reviewpath.CollectionNamespacePathSafe(caseRoot, reviewRoot, false) || !reviewpath.CollectionNamespacePathSafe(caseRoot, filepath.Dir(promptPath), true) {
		return fmt.Errorf("reviewer prompt artifact repair path must stay under a symlink-free canonical review namespace")
	}
	if st, err := os.Lstat(filepath.Dir(promptPath)); err == nil {
		if st.Mode()&os.ModeSymlink != 0 || !st.IsDir() {
			return fmt.Errorf("reviewer prompt artifact parent must be a non-symlink directory")
		}
	} else if !os.IsNotExist(err) {
		return err
	}
	return nil
}

func reviewerPromptArtifactExisting(caseRoot, promptPath string) (string, []byte, error) {
	if err := rejectReviewerArtifactSymlinkPath(caseRoot, filepath.Dir(promptPath), true); err != nil {
		return "invalid", nil, err
	}
	st, err := os.Lstat(promptPath)
	if os.IsNotExist(err) {
		return "missing", nil, nil
	}
	if err != nil {
		return "invalid", nil, err
	}
	if st.Mode()&os.ModeSymlink != 0 {
		return "symlink", nil, nil
	}
	if !st.Mode().IsRegular() || st.Size() == 0 {
		return "invalid", nil, nil
	}
	data, err := readStableReviewerArtifact(caseRoot, promptPath, "reviewer prompt artifact", maxReviewPacketBytes)
	if err != nil {
		return "invalid", nil, err
	}
	return "ready", data, nil
}

func newReviewerPromptArtifactRepairResult(repoRoot, caseRoot, pack string, opt ReviewerPromptArtifactRepairOptions, prepared preparedReviewerPromptArtifactRepair) ReviewerPromptArtifactRepairResult {
	promptSHA256 := sha256Hex(prepared.prompt)
	boundary := []string{
		"reviewer prompt artifact repair derives bytes only from the immutable canonical reviewer packet dispatch prompt",
		"WhatIf never writes; Apply requires the exact promptSha256 returned by WhatIf and revalidates packet integrity before writing",
		"Apply only creates a missing packet-derived prompt artifact or accepts exact replay; it never overwrites drifted, symlink, empty, directory, or different existing prompt artifacts",
		"runtime does not spawn, stop, monitor, or manage reviewer sessions and does not execute heavy tools or write authority/confirmed state",
	}
	result := ReviewerPromptArtifactRepairResult{
		SchemaVersion:        1,
		Command:              commandName,
		Mode:                 "reviewer-prompt-artifact-repair",
		CaseRoot:             caseRoot,
		RepoRoot:             repoRoot,
		Pack:                 strings.TrimSpace(pack),
		IsMutation:           !opt.WhatIf,
		RequiresConfirmation: true,
		PacketID:             prepared.packet.PacketID,
		PacketPath:           prepared.packetPath,
		ShardID:              prepared.handoff.ShardID,
		Lane:                 prepared.lane,
		Actor:                prepared.actor,
		PromptPath:           prepared.promptPath,
		PromptSHA256:         promptSHA256,
		PromptBytes:          len(prepared.prompt),
		ExistingPromptState:  prepared.existingPromptState,
		Boundary:             boundary,
	}
	if len(prepared.existingPrompt) > 0 {
		result.ExistingPromptSHA256 = sha256Hex(prepared.existingPrompt)
		result.ExistingPromptBytes = len(prepared.existingPrompt)
	}
	result.ApplyCommand = reviewerPromptArtifactRepairCommand(prepared.packetPath, prepared.handoff.ShardID, prepared.lane, prepared.actor, promptSHA256, true)
	return result
}

func finalizeReviewerPromptArtifactRepairPreview(result ReviewerPromptArtifactRepairResult, prepared preparedReviewerPromptArtifactRepair) ReviewerPromptArtifactRepairResult {
	if prepared.existingPromptState == "ready" && bytes.Equal(prepared.existingPrompt, prepared.prompt) {
		result.Status = "already-current"
		result.AlreadyCurrent = true
		result.ApplyCommand = ""
		result.NextSteps = []string{"prompt artifact already matches the packet-derived dispatch prompt; rerun status and dispatch only after promptCurrent=true"}
		result.MissionCommanderAction = mission.MissionCommanderAction{State: "reviewer-prompt-artifact-current", PrimaryCommand: "", Boundary: result.Boundary}
		return finalizeReviewerPromptArtifactRepairResult(result)
	}
	if prepared.existingPromptState != "missing" {
		result.Status = "blocked-existing-" + prepared.existingPromptState
		result.ApplyCommand = ""
		result.NextSteps = []string{"inspect the existing prompt artifact and restore it out-of-band or regenerate the canonical reviewer packet; this command will not overwrite existing prompt bytes"}
		result.MissionCommanderAction = mission.MissionCommanderAction{State: "reviewer-prompt-artifact-repair-blocked", PrimaryCommand: "", Boundary: result.Boundary}
		return finalizeReviewerPromptArtifactRepairResult(result)
	}
	result.Status = "previewed"
	result.NextSteps = []string{"review the packet-derived prompt artifact path and promptSha256, then run the returned repair command with -Apply"}
	result.MissionCommanderAction = mission.MissionCommanderAction{State: "needs-reviewer-prompt-artifact-repair-apply", PrimaryCommand: result.ApplyCommand, Boundary: result.Boundary}
	return finalizeReviewerPromptArtifactRepairResult(result)
}

func finalizeReviewerPromptArtifactRepairResult(result ReviewerPromptArtifactRepairResult) ReviewerPromptArtifactRepairResult {
	if result.MissionCommanderAction.State != "" {
		result.MissionCommanderNextActions = []mission.MissionCommanderNextActionItem{{
			Lane:           result.Lane,
			Label:          result.PacketID,
			ActionID:       result.PacketID + ":" + result.ShardID + ":prompt",
			State:          result.MissionCommanderAction.State,
			Command:        result.MissionCommanderAction.PrimaryCommand,
			Source:         "reviewerPromptArtifactRepair",
			RequiresReview: true,
			Blocked:        result.MissionCommanderAction.PrimaryCommand == "",
			Reasons:        append([]string{}, result.NextSteps...),
			Boundary:       append([]string{}, result.Boundary...),
		}}
	}
	result.MissionCommanderActionQueue = mission.MissionCommanderActionQueueFor(result.MissionCommanderNextActions)
	return result
}

func validateReviewerPromptArtifactRepairExpectedHash(opt ReviewerPromptArtifactRepairOptions) error {
	decoded, err := hex.DecodeString(strings.TrimSpace(opt.ExpectedPromptSHA256))
	if err != nil || len(decoded) != sha256.Size {
		return fmt.Errorf("reviewer prompt artifact repair Apply requires a valid -ExpectedPromptSha256 from WhatIf")
	}
	return nil
}

func reviewerPromptArtifactRepairCommand(packetPath, shardID, lane, actor, expectedPromptSHA256 string, apply bool) string {
	command := "/rekit plan-subagents -PacketPath " + quoteCommandArg(packetPath) +
		" -RepairReviewerPromptArtifact -ShardId " + quoteCommandArg(shardID) +
		" -Lane " + quoteCommandArg(lane) +
		" -Actor " + quoteCommandArg(actor)
	if apply {
		return command + " -ExpectedPromptSha256 " + quoteCommandArg(expectedPromptSHA256) + " -Apply -Format json"
	}
	return command + " -WhatIf -Format json"
}

func publishReviewerPromptArtifact(caseRoot string, prepared preparedReviewerPromptArtifactRepair) error {
	root, err := os.OpenRoot(caseRoot)
	if err != nil {
		return err
	}
	defer root.Close()
	reviewRel, err := filepath.Rel(caseRoot, filepath.Dir(prepared.packetPath))
	if err != nil || filepath.IsAbs(reviewRel) || reviewRel == ".." || strings.HasPrefix(reviewRel, ".."+string(filepath.Separator)) {
		return fmt.Errorf("reviewer prompt artifact review root escapes attached case")
	}
	reviewRoot, err := openReviewerDirectoryNoFollow(root, reviewRel)
	if err != nil {
		return err
	}
	defer reviewRoot.Close()
	openedReview, err := reviewRoot.Lstat(".")
	if err != nil || prepared.packetSnapshot.parentID == nil || !os.SameFile(openedReview, prepared.packetSnapshot.parentID) {
		return fmt.Errorf("review packet namespace changed after prompt repair validation")
	}
	if err := reviewRoot.Mkdir("prompts", 0o755); err != nil && !os.IsExist(err) {
		return err
	}
	promptRoot, err := openReviewerDirectoryNoFollow(reviewRoot, "prompts")
	if err != nil {
		return err
	}
	defer promptRoot.Close()
	promptRel := filepath.Join(reviewRel, "prompts")
	if !reviewerDirectoryPathMatches(caseRoot, promptRel, promptRoot) || !reviewerPacketSnapshotCurrent(caseRoot, prepared.packetPath, prepared.packetSnapshot) {
		return fmt.Errorf("reviewer prompt artifact namespace changed while publishing")
	}
	name := filepath.Base(prepared.promptPath)
	if name != prepared.handoff.ShardID+".prompt.md" {
		return fmt.Errorf("reviewer prompt artifact leaf is not canonical")
	}
	if existing, present, readErr := existingReviewerPromptArtifactFromRoot(promptRoot, name); readErr != nil || present {
		if readErr != nil {
			return readErr
		}
		if bytes.Equal(existing, prepared.prompt) {
			return nil
		}
		return fmt.Errorf("reviewer prompt artifact %q already contains different bytes; refusing overwrite", prepared.promptPath)
	}
	random := make([]byte, 16)
	if _, err := rand.Read(random); err != nil {
		return err
	}
	tmpName := ".reviewer-prompt-" + hex.EncodeToString(random) + ".tmp"
	tmp, err := promptRoot.OpenFile(tmpName, os.O_CREATE|os.O_EXCL|os.O_WRONLY, 0o644)
	if err != nil {
		return err
	}
	defer promptRoot.Remove(tmpName)
	if _, err := tmp.Write(prepared.prompt); err != nil {
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
	if !reviewerDirectoryPathMatches(caseRoot, promptRel, promptRoot) || !reviewerPacketSnapshotCurrent(caseRoot, prepared.packetPath, prepared.packetSnapshot) {
		return fmt.Errorf("reviewer prompt artifact namespace changed before publishing")
	}
	if err := promptRoot.Link(tmpName, name); err != nil {
		if existing, present, readErr := existingReviewerPromptArtifactFromRoot(promptRoot, name); readErr == nil && present && bytes.Equal(existing, prepared.prompt) {
			return nil
		}
		return fmt.Errorf("publish reviewer prompt artifact without replacement: %w", err)
	}
	published, present, err := existingReviewerPromptArtifactFromRoot(promptRoot, name)
	if err != nil || !present || !bytes.Equal(published, prepared.prompt) {
		return fmt.Errorf("verify published reviewer prompt artifact: %w", err)
	}
	if !reviewerDirectoryPathMatches(caseRoot, promptRel, promptRoot) || !reviewerPacketSnapshotCurrent(caseRoot, prepared.packetPath, prepared.packetSnapshot) {
		return fmt.Errorf("reviewer prompt artifact namespace changed after publishing")
	}
	return nil
}

func existingReviewerPromptArtifactFromRoot(root *os.Root, name string) ([]byte, bool, error) {
	st, err := root.Lstat(name)
	if os.IsNotExist(err) {
		return nil, false, nil
	}
	if err != nil {
		return nil, false, err
	}
	if st.Mode()&os.ModeSymlink != 0 {
		return nil, true, fmt.Errorf("reviewer prompt artifact must not be a symlink: %s", name)
	}
	if !st.Mode().IsRegular() || st.Size() == 0 {
		return nil, true, fmt.Errorf("reviewer prompt artifact must be a non-empty regular file: %s", name)
	}
	data, err := readStableReviewerArtifactFromRoot(root, name, "reviewer prompt artifact", maxReviewPacketBytes)
	return data, err == nil, err
}
