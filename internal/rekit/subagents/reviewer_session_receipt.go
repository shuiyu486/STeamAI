package subagents

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"github.com/shuiyu486/re-context-kits/internal/rekit/casebind"
	"github.com/shuiyu486/re-context-kits/internal/rekit/instance"
	"github.com/shuiyu486/re-context-kits/internal/rekit/mission"
	"github.com/shuiyu486/re-context-kits/internal/rekit/reviewerresult"
	"github.com/shuiyu486/re-context-kits/internal/rekit/reviewersession"
	"github.com/shuiyu486/re-context-kits/internal/rekit/reviewpath"
)

const maxReviewerSessionReceiptBytes int64 = 256 * 1024

type ReviewerSessionOwner = reviewersession.Owner

type ReviewerSessionDispatchReceipt = reviewersession.DispatchReceipt

type ReviewerSessionCompletionReceipt = reviewersession.CompletionReceipt

type ReviewerSessionDispatchOptions struct {
	PacketPath            string
	ShardID               string
	Lane                  string
	Actor                 string
	ReviewerHarness       string
	ReviewerSession       string
	ExpectedBindingSHA256 string
	WhatIf                bool
}

type ReviewerSessionCompletionOptions struct {
	PacketPath                    string
	DispatchID                    string
	Lane                          string
	Actor                         string
	Outcome                       string
	ExitStatus                    string
	ReviewerResultInputPath       string
	ExpectedDispatchReceiptSHA256 string
	ExpectedReviewerResultSHA256  string
	WhatIf                        bool
}

type ReviewerSessionReceiptResult struct {
	SchemaVersion               int                                      `json:"schemaVersion"`
	Command                     string                                   `json:"command"`
	Mode                        string                                   `json:"mode"`
	CaseRoot                    string                                   `json:"caseRoot"`
	RepoRoot                    string                                   `json:"repoRoot"`
	Pack                        string                                   `json:"pack"`
	IsMutation                  bool                                     `json:"isMutation"`
	Applied                     bool                                     `json:"applied"`
	AlreadyRecorded             bool                                     `json:"alreadyRecorded"`
	RequiresConfirmation        bool                                     `json:"requiresConfirmation"`
	PacketID                    string                                   `json:"packetId"`
	PacketPath                  string                                   `json:"packetPath"`
	RouteID                     string                                   `json:"routeId"`
	ShardID                     string                                   `json:"shardId"`
	TargetLane                  string                                   `json:"targetLane"`
	DispatchID                  string                                   `json:"dispatchId"`
	ReviewerHarness             string                                   `json:"reviewerHarness"`
	ReviewerSession             string                                   `json:"reviewerSession"`
	Outcome                     string                                   `json:"outcome,omitempty"`
	ExitStatus                  string                                   `json:"exitStatus,omitempty"`
	ReceiptPath                 string                                   `json:"receiptPath"`
	ReceiptSHA256               string                                   `json:"receiptSha256,omitempty"`
	BindingSHA256               string                                   `json:"bindingSha256,omitempty"`
	DispatchReceiptPath         string                                   `json:"dispatchReceiptPath,omitempty"`
	DispatchReceiptSHA256       string                                   `json:"dispatchReceiptSha256,omitempty"`
	ReviewerResultInputPath     string                                   `json:"reviewerResultInputPath,omitempty"`
	ReviewerResultInputSHA256   string                                   `json:"reviewerResultInputSha256,omitempty"`
	ReviewerResultInputBytes    int                                      `json:"reviewerResultInputBytes,omitempty"`
	EffectiveOwner              ReviewerSessionOwner                     `json:"effectiveOwner"`
	OwnerAdoptionPath           string                                   `json:"ownerAdoptionPath,omitempty"`
	OwnerAdoptionSHA256         string                                   `json:"ownerAdoptionSha256,omitempty"`
	ApplyCommand                string                                   `json:"applyCommand,omitempty"`
	MissionCommanderAction      mission.MissionCommanderAction           `json:"missionCommanderAction"`
	MissionCommanderNextActions []mission.MissionCommanderNextActionItem `json:"missionCommanderNextActions"`
	MissionCommanderActionQueue mission.MissionCommanderActionQueue      `json:"missionCommanderActionQueue"`
	NextSteps                   []string                                 `json:"nextSteps"`
	Boundary                    []string                                 `json:"boundary"`
}

type preparedReviewerSessionDispatch struct {
	packetPath     string
	packet         Packet
	packetBytes    []byte
	packetSnapshot reviewerPacketAnchoredSnapshot
	handoff        ShardHandoff
	effectiveOwner OwnerBinding
	adoptionPath   string
	adoptionSHA256 string
	dispatchID     string
	receiptPath    string
	bindingSHA256  string
	actor          string
	harness        string
	session        string
}

type preparedReviewerSessionCompletion struct {
	packetPath     string
	packet         Packet
	packetSnapshot reviewerPacketAnchoredSnapshot
	handoff        ShardHandoff
	dispatch       ReviewerSessionDispatchReceipt
	dispatchPath   string
	dispatchBytes  []byte
	effectiveOwner OwnerBinding
	adoptionPath   string
	adoptionSHA256 string
	completionPath string
	inputPath      string
	input          []byte
	result         reviewerresult.Result
	actor          string
	outcome        string
	exitStatus     string
}

func RecordReviewerSessionDispatch(repoRoot, caseRoot, pack string, opt ReviewerSessionDispatchOptions) (ReviewerSessionReceiptResult, error) {
	inst, err := instance.AssertAttached(caseRoot, repoRoot, pack)
	if err != nil {
		return ReviewerSessionReceiptResult{}, err
	}
	prepared, err := prepareReviewerSessionDispatch(repoRoot, inst.CaseRoot, pack, opt)
	if err != nil {
		return ReviewerSessionReceiptResult{}, err
	}
	result := reviewerSessionDispatchResult(repoRoot, inst.CaseRoot, pack, opt, prepared)
	if opt.WhatIf {
		result.ApplyCommand = reviewerSessionDispatchCommand(prepared, true)
		result.MissionCommanderAction = mission.MissionCommanderAction{State: "ready-to-record-reviewer-session-dispatch", PrimaryCommand: result.ApplyCommand, Boundary: result.Boundary}
		result.NextSteps = []string{"after the harness has actually dispatched the reviewer, record this immutable dispatch receipt with the returned hash-bound Apply command"}
		return finalizeReviewerSessionReceiptResult(result), nil
	}
	if strings.TrimSpace(opt.ExpectedBindingSHA256) == "" || !strings.EqualFold(opt.ExpectedBindingSHA256, prepared.bindingSHA256) {
		return ReviewerSessionReceiptResult{}, fmt.Errorf("reviewer session dispatch changed after preview")
	}
	unlock, err := acquireReviewerIntakeLock(inst.CaseRoot, reviewerResultMutationLockID(prepared.packet.PacketID, prepared.handoff.ShardID))
	if err != nil {
		return ReviewerSessionReceiptResult{}, err
	}
	defer unlock()
	prepared, err = prepareReviewerSessionDispatch(repoRoot, inst.CaseRoot, pack, opt)
	if err != nil {
		return ReviewerSessionReceiptResult{}, err
	}
	if !strings.EqualFold(opt.ExpectedBindingSHA256, prepared.bindingSHA256) {
		return ReviewerSessionReceiptResult{}, fmt.Errorf("reviewer session dispatch changed after preview")
	}
	receipt := reviewerSessionDispatchReceipt(prepared)
	already, err := writeOrReplayReviewerSessionDispatch(inst.CaseRoot, prepared.receiptPath, receipt)
	if err != nil {
		return ReviewerSessionReceiptResult{}, err
	}
	data, err := readReviewerSessionReceiptBytes(inst.CaseRoot, prepared.receiptPath, "reviewer session dispatch receipt")
	if err != nil {
		return ReviewerSessionReceiptResult{}, err
	}
	result = reviewerSessionDispatchResult(repoRoot, inst.CaseRoot, pack, opt, prepared)
	result.Applied = true
	result.AlreadyRecorded = already
	result.ReceiptSHA256 = sha256Hex(data)
	result.Mode = map[bool]string{true: "already-recorded", false: "recorded"}[already]
	result.MissionCommanderAction = mission.MissionCommanderAction{State: "reviewer-session-running-unknown", PrimaryCommand: "/rekit status", Boundary: result.Boundary}
	result.NextSteps = []string{"the runtime has no live reviewer visibility; when the harness reports success or failure, record a completion receipt before source capture"}
	return finalizeReviewerSessionReceiptResult(result), nil
}

func RecordReviewerSessionCompletion(repoRoot, caseRoot, pack string, opt ReviewerSessionCompletionOptions) (ReviewerSessionReceiptResult, error) {
	inst, err := instance.AssertAttached(caseRoot, repoRoot, pack)
	if err != nil {
		return ReviewerSessionReceiptResult{}, err
	}
	prepared, err := prepareReviewerSessionCompletion(repoRoot, inst.CaseRoot, pack, opt)
	if err != nil {
		return ReviewerSessionReceiptResult{}, err
	}
	result := reviewerSessionCompletionResult(repoRoot, inst.CaseRoot, pack, opt, prepared)
	if opt.WhatIf {
		result.ApplyCommand = reviewerSessionCompletionCommand(prepared, true)
		result.MissionCommanderAction = mission.MissionCommanderAction{State: "ready-to-record-reviewer-session-completion", PrimaryCommand: result.ApplyCommand, Boundary: result.Boundary}
		result.NextSteps = []string{"inspect the dispatch receipt and exact result input binding, then record the immutable completion receipt with the returned hash-bound Apply command"}
		return finalizeReviewerSessionReceiptResult(result), nil
	}
	if !strings.EqualFold(strings.TrimSpace(opt.ExpectedDispatchReceiptSHA256), sha256Hex(prepared.dispatchBytes)) {
		return ReviewerSessionReceiptResult{}, fmt.Errorf("reviewer session dispatch receipt changed after completion preview")
	}
	if prepared.outcome == "succeeded" && !strings.EqualFold(strings.TrimSpace(opt.ExpectedReviewerResultSHA256), sha256Hex(prepared.input)) {
		return ReviewerSessionReceiptResult{}, fmt.Errorf("reviewer result input changed after completion preview")
	}
	unlock, err := acquireReviewerIntakeLock(inst.CaseRoot, reviewerResultMutationLockID(prepared.packet.PacketID, prepared.handoff.ShardID))
	if err != nil {
		return ReviewerSessionReceiptResult{}, err
	}
	defer unlock()
	prepared, err = prepareReviewerSessionCompletion(repoRoot, inst.CaseRoot, pack, opt)
	if err != nil {
		return ReviewerSessionReceiptResult{}, err
	}
	if !strings.EqualFold(strings.TrimSpace(opt.ExpectedDispatchReceiptSHA256), sha256Hex(prepared.dispatchBytes)) ||
		(prepared.outcome == "succeeded" && !strings.EqualFold(strings.TrimSpace(opt.ExpectedReviewerResultSHA256), sha256Hex(prepared.input))) {
		return ReviewerSessionReceiptResult{}, fmt.Errorf("reviewer session completion changed after preview")
	}
	receipt := reviewerSessionCompletionReceipt(prepared)
	already, err := writeOrReplayReviewerSessionCompletion(inst.CaseRoot, prepared.completionPath, receipt)
	if err != nil {
		return ReviewerSessionReceiptResult{}, err
	}
	data, err := readReviewerSessionReceiptBytes(inst.CaseRoot, prepared.completionPath, "reviewer session completion receipt")
	if err != nil {
		return ReviewerSessionReceiptResult{}, err
	}
	result = reviewerSessionCompletionResult(repoRoot, inst.CaseRoot, pack, opt, prepared)
	result.Applied = true
	result.AlreadyRecorded = already
	result.ReceiptSHA256 = sha256Hex(data)
	result.Mode = map[bool]string{true: "already-recorded", false: "recorded"}[already]
	if prepared.outcome == "succeeded" {
		result.MissionCommanderAction = mission.MissionCommanderAction{State: "reviewer-session-completed-ready-for-source-capture", PrimaryCommand: reviewerResultSourceCaptureCommand(prepared.packetPath, prepared.handoff.ShardID, prepared.inputPath, prepared.packet.TargetLane, prepared.actor, "", false), Boundary: result.Boundary}
		result.NextSteps = []string{"run reviewer result source capture -WhatIf; completion records provenance only and does not collect or intake the result"}
	} else {
		result.MissionCommanderAction = mission.MissionCommanderAction{State: "reviewer-session-failed-needs-redispatch", PrimaryCommand: "/rekit status", Boundary: result.Boundary}
		result.NextSteps = []string{"do not source-capture a failed attempt; dispatch a new reviewer session and record a new dispatch receipt"}
	}
	return finalizeReviewerSessionReceiptResult(result), nil
}

func prepareReviewerSessionDispatch(repoRoot, caseRoot, pack string, opt ReviewerSessionDispatchOptions) (preparedReviewerSessionDispatch, error) {
	packetPath, packet, packetBytes, snapshot, err := loadReviewerSessionPacket(repoRoot, caseRoot, pack, opt.PacketPath)
	if err != nil {
		return preparedReviewerSessionDispatch{}, err
	}
	shardID := strings.TrimSpace(opt.ShardID)
	handoff, ok := shardHandoffByID(packet.ShardHandoffs, shardID)
	if !ok {
		return preparedReviewerSessionDispatch{}, fmt.Errorf("reviewer session dispatch shard %q is not present in packet", shardID)
	}
	lane, actor := strings.TrimSpace(opt.Lane), strings.TrimSpace(opt.Actor)
	harness, session := strings.TrimSpace(opt.ReviewerHarness), strings.TrimSpace(opt.ReviewerSession)
	if lane == "" || actor == "" || harness == "" || session == "" || strings.ContainsAny(harness+session, "\r\n") {
		return preparedReviewerSessionDispatch{}, fmt.Errorf("reviewer session dispatch requires single-line -ReviewerHarness/-ReviewerSession and non-empty -Lane/-Actor")
	}
	if lane != packet.TargetLane {
		return preparedReviewerSessionDispatch{}, fmt.Errorf("reviewer session dispatch lane %q does not match packet targetLane %q", lane, packet.TargetLane)
	}
	if strings.TrimSpace(handoff.DispatchPromptPath) == "" || strings.TrimSpace(handoff.DispatchPromptSHA256) == "" || handoff.AgentToolRequest == nil {
		return preparedReviewerSessionDispatch{}, fmt.Errorf("review packet does not provide managed prompt and Agent tool bindings for shard %q", shardID)
	}
	prompt, err := readStableReviewerArtifactAnchored(caseRoot, handoff.DispatchPromptPath, "reviewer dispatch prompt", maxReviewPacketBytes)
	if err != nil || !strings.EqualFold(sha256Hex(prompt), handoff.DispatchPromptSHA256) {
		return preparedReviewerSessionDispatch{}, fmt.Errorf("reviewer dispatch prompt is missing or changed for shard %q", shardID)
	}
	effective, adoption, err := validateOwnerBinding(caseRoot, packet, packetPath, packetBytes)
	if err != nil {
		return preparedReviewerSessionDispatch{}, err
	}
	adoptionPath, adoptionSHA := reviewerSessionAdoptionProvenance(caseRoot, packet.PacketID, adoption)
	dispatchID := reviewerSessionDispatchID(packet.PacketID, packet.Route.ID, shardID, handoff.DispatchPromptSHA256, harness, session)
	receiptPath := reviewerSessionDispatchPath(packetPath, shardID, dispatchID)
	binding := reviewerSessionDispatchBindingSHA(packet, packetBytes, handoff, effective, adoptionSHA, harness, session)
	if !reviewerPacketSnapshotCurrent(caseRoot, packetPath, snapshot) {
		return preparedReviewerSessionDispatch{}, fmt.Errorf("review packet changed while validating reviewer session dispatch")
	}
	return preparedReviewerSessionDispatch{packetPath: packetPath, packet: packet, packetBytes: packetBytes, packetSnapshot: snapshot, handoff: handoff, effectiveOwner: effective, adoptionPath: adoptionPath, adoptionSHA256: adoptionSHA, dispatchID: dispatchID, receiptPath: receiptPath, bindingSHA256: binding, actor: actor, harness: harness, session: session}, nil
}

func prepareReviewerSessionCompletion(repoRoot, caseRoot, pack string, opt ReviewerSessionCompletionOptions) (preparedReviewerSessionCompletion, error) {
	packetPath, packet, packetBytes, snapshot, err := loadReviewerSessionPacket(repoRoot, caseRoot, pack, opt.PacketPath)
	if err != nil {
		return preparedReviewerSessionCompletion{}, err
	}
	dispatchID := strings.TrimSpace(opt.DispatchID)
	if dispatchID == "" {
		return preparedReviewerSessionCompletion{}, fmt.Errorf("reviewer session completion requires -ReviewerDispatchId")
	}
	dispatchPath, dispatchBytes, dispatch, err := findReviewerSessionDispatch(caseRoot, packetPath, dispatchID)
	if err != nil {
		return preparedReviewerSessionCompletion{}, err
	}
	handoff, ok := shardHandoffByID(packet.ShardHandoffs, dispatch.ShardID)
	if !ok {
		return preparedReviewerSessionCompletion{}, fmt.Errorf("reviewer session dispatch receipt does not match a current packet shard")
	}
	lane, actor := strings.TrimSpace(opt.Lane), strings.TrimSpace(opt.Actor)
	if lane == "" || actor == "" || lane != packet.TargetLane {
		return preparedReviewerSessionCompletion{}, fmt.Errorf("reviewer session completion requires current packet -Lane and non-empty -Actor")
	}
	outcome := strings.ToLower(strings.TrimSpace(opt.Outcome))
	if outcome != "succeeded" && outcome != "failed" {
		return preparedReviewerSessionCompletion{}, fmt.Errorf("reviewer session completion outcome must be succeeded or failed")
	}
	exitStatus := strings.TrimSpace(opt.ExitStatus)
	if exitStatus == "" || strings.ContainsAny(exitStatus, "\r\n") {
		return preparedReviewerSessionCompletion{}, fmt.Errorf("reviewer session completion requires single-line -ReviewerExitStatus")
	}
	effective, adoption, err := validateOwnerBinding(caseRoot, packet, packetPath, packetBytes)
	if err != nil {
		return preparedReviewerSessionCompletion{}, err
	}
	if dispatch.EffectiveOwner != reviewerSessionOwner(effective) {
		return preparedReviewerSessionCompletion{}, fmt.Errorf("reviewer session dispatch receipt belongs to a stale lane owner generation")
	}
	adoptionPath, adoptionSHA := reviewerSessionAdoptionProvenance(caseRoot, packet.PacketID, adoption)
	if !reviewerSessionDispatchMatchesCurrent(packetPath, dispatchPath, packet, packetBytes, handoff, dispatch, effective, adoptionPath, adoptionSHA) {
		return preparedReviewerSessionCompletion{}, fmt.Errorf("reviewer session dispatch receipt does not match current immutable packet, prompt, owner, and adoption bindings")
	}
	inputPath := ""
	var input []byte
	var result reviewerresult.Result
	if outcome == "succeeded" {
		inputPath, err = requiredAbsolutePath(opt.ReviewerResultInputPath, "reviewer session completion result input")
		if err != nil {
			return preparedReviewerSessionCompletion{}, err
		}
		expectedInput := ""
		if handoff.ReviewerStagingCommands != nil {
			expectedInput = handoff.ReviewerStagingCommands.SourceCaptureInput
		}
		if expectedInput == "" || !samePath(inputPath, expectedInput) || !reviewpath.CollectionNamespacePathSafe(caseRoot, inputPath, false) {
			return preparedReviewerSessionCompletion{}, fmt.Errorf("reviewer session completion input must match the packet-derived case-local reviewer result input path")
		}
		input, err = readStableReviewerArtifactAnchored(caseRoot, inputPath, "reviewer session completion result input", maxReviewerResultBytes)
		if err != nil {
			return preparedReviewerSessionCompletion{}, err
		}
		result, err = reviewerresult.Decode(input)
		if err != nil {
			return preparedReviewerSessionCompletion{}, err
		}
		if result.PacketID != packet.PacketID || result.RouteID != packet.Route.ID || result.ShardID != dispatch.ShardID || result.ReviewerSession != dispatch.ReviewerSession {
			return preparedReviewerSessionCompletion{}, fmt.Errorf("reviewer session completion result does not match dispatch packet/route/shard/session bindings")
		}
	}
	completionPath := reviewerSessionCompletionPath(packetPath, dispatch.ShardID, dispatchID)
	if !reviewerPacketSnapshotCurrent(caseRoot, packetPath, snapshot) {
		return preparedReviewerSessionCompletion{}, fmt.Errorf("review packet changed while validating reviewer session completion")
	}
	return preparedReviewerSessionCompletion{packetPath: packetPath, packet: packet, packetSnapshot: snapshot, handoff: handoff, dispatch: dispatch, dispatchPath: dispatchPath, dispatchBytes: dispatchBytes, effectiveOwner: effective, adoptionPath: adoptionPath, adoptionSHA256: adoptionSHA, completionPath: completionPath, inputPath: inputPath, input: input, result: result, actor: actor, outcome: outcome, exitStatus: exitStatus}, nil
}

func loadReviewerSessionPacket(repoRoot, caseRoot, pack, path string) (string, Packet, []byte, reviewerPacketAnchoredSnapshot, error) {
	packetPath, err := requiredAbsolutePath(path, "review packet")
	if err != nil {
		return "", Packet{}, nil, reviewerPacketAnchoredSnapshot{}, err
	}
	if _, ok := reviewpath.CanonicalCollectionNamespace(caseRoot, packetPath); !ok || !reviewpath.CollectionNamespacePathSafe(caseRoot, packetPath, false) {
		return "", Packet{}, nil, reviewerPacketAnchoredSnapshot{}, fmt.Errorf("review packet must be a symlink-free canonical case review packet")
	}
	data, snapshot, err := readReviewerPacketAndIntegrityAnchored(caseRoot, packetPath)
	if err != nil {
		return "", Packet{}, nil, reviewerPacketAnchoredSnapshot{}, err
	}
	packet, err := decodeIntakePacket(data)
	if err != nil {
		return "", Packet{}, nil, reviewerPacketAnchoredSnapshot{}, err
	}
	if err := validateIntakePacket(packet, packetPath, repoRoot, caseRoot, pack); err != nil {
		return "", Packet{}, nil, reviewerPacketAnchoredSnapshot{}, err
	}
	if err := validatePacketIntegrityBytes(packetPath, packet, data, snapshot.integrity); err != nil {
		return "", Packet{}, nil, reviewerPacketAnchoredSnapshot{}, err
	}
	return packetPath, packet, data, snapshot, nil
}

func reviewerSessionDispatchReceipt(p preparedReviewerSessionDispatch) ReviewerSessionDispatchReceipt {
	return ReviewerSessionDispatchReceipt{SchemaVersion: 1, Kind: "reviewer-session-dispatch", DispatchID: p.dispatchID, PacketID: p.packet.PacketID, PacketPath: p.packetPath, PacketSHA256: sha256Hex(p.packetBytes), RouteID: p.packet.Route.ID, ShardID: p.handoff.ShardID, Items: append([]string{}, p.handoff.Items...), PromptPath: p.handoff.DispatchPromptPath, PromptSHA256: p.handoff.DispatchPromptSHA256, AgentType: p.handoff.AgentToolRequest.AgentType, ReadOnly: p.handoff.AgentToolRequest.ReadOnly, TargetLane: p.packet.TargetLane, PacketOwner: reviewerSessionOwner(p.packet.OwnerBinding), EffectiveOwner: reviewerSessionOwner(p.effectiveOwner), OwnerAdoptionPath: p.adoptionPath, OwnerAdoptionSHA256: p.adoptionSHA256, ReviewerHarness: p.harness, ReviewerSession: p.session, Actor: p.actor, RecordedAt: time.Now().UTC().Format(time.RFC3339Nano), NoSpawn: true, NoHeavyTool: true, NoAuthority: true}
}

func reviewerSessionCompletionReceipt(p preparedReviewerSessionCompletion) ReviewerSessionCompletionReceipt {
	return ReviewerSessionCompletionReceipt{SchemaVersion: 1, Kind: "reviewer-session-completion", DispatchID: p.dispatch.DispatchID, DispatchReceiptPath: p.dispatchPath, DispatchReceiptSHA256: sha256Hex(p.dispatchBytes), PacketID: p.packet.PacketID, RouteID: p.packet.Route.ID, ShardID: p.dispatch.ShardID, ReviewerHarness: p.dispatch.ReviewerHarness, ReviewerSession: p.dispatch.ReviewerSession, Outcome: p.outcome, ExitStatus: p.exitStatus, ReviewerResultInputPath: p.inputPath, ReviewerResultInputSHA256: sha256Hex(p.input), ReviewerResultInputBytes: len(p.input), CompletionOwner: reviewerSessionOwner(p.effectiveOwner), OwnerAdoptionPath: p.adoptionPath, OwnerAdoptionSHA256: p.adoptionSHA256, Actor: p.actor, RecordedAt: time.Now().UTC().Format(time.RFC3339Nano), NoCollection: true, NoIntake: true, NoFacts: true, NoHeavyTool: true, NoAuthority: true}
}

func reviewerSessionOwner(binding OwnerBinding) ReviewerSessionOwner {
	return ReviewerSessionOwner{CurrentExecutor: binding.CurrentExecutor, ExecutorGeneration: binding.ExecutorGeneration, BindingMode: binding.BindingMode}
}

func reviewerSessionAdoptionProvenance(caseRoot, packetID string, adoption *ReviewerPacketAdoption) (string, string) {
	if adoption == nil {
		return "", ""
	}
	path, err := reviewerPacketAdoptionPath(caseRoot, packetID)
	if err != nil {
		return "", ""
	}
	data, err := readStableReviewerArtifact(filepath.Dir(path), path, "reviewer packet adoption", maxReviewPacketBytes)
	if err != nil {
		return path, ""
	}
	return path, sha256Hex(data)
}

func reviewerSessionDispatchID(packetID, routeID, shardID, promptSHA, harness, session string) string {
	return sha256Hex([]byte(strings.Join([]string{packetID, routeID, shardID, strings.ToLower(promptSHA), harness, session}, "\n")))
}

func reviewerSessionDispatchBindingSHA(packet Packet, packetBytes []byte, handoff ShardHandoff, owner OwnerBinding, adoptionSHA, harness, session string) string {
	value := strings.Join([]string{packet.PacketID, sha256Hex(packetBytes), packet.Route.ID, handoff.ShardID, handoff.DispatchPromptSHA256, harness, session, owner.CurrentExecutor, fmt.Sprint(owner.ExecutorGeneration), owner.BindingMode, adoptionSHA}, "\n")
	return sha256Hex([]byte(value))
}

func reviewerSessionRoot(packetPath, shardID string) string {
	return filepath.Join(filepath.Dir(packetPath), "sessions", shardID)
}

func reviewerSessionDispatchPath(packetPath, shardID, dispatchID string) string {
	return reviewersession.DispatchPath(packetPath, shardID, dispatchID)
}

func reviewerSessionCompletionPath(packetPath, shardID, dispatchID string) string {
	return reviewersession.CompletionPath(packetPath, shardID, dispatchID)
}

func writeOrReplayReviewerSessionDispatch(caseRoot, path string, value ReviewerSessionDispatchReceipt) (bool, error) {
	if existing, err := readReviewerSessionDispatch(caseRoot, path); err == nil {
		if reviewerSessionDispatchEquivalent(existing, value) {
			return true, nil
		}
		return false, fmt.Errorf("reviewer session dispatch receipt already exists with different bindings")
	} else if _, statErr := os.Lstat(path); statErr == nil || !os.IsNotExist(statErr) {
		return false, fmt.Errorf("reviewer session dispatch receipt exists but is invalid")
	}
	return false, writeReviewerSessionReceipt(caseRoot, path, value)
}

func writeOrReplayReviewerSessionCompletion(caseRoot, path string, value ReviewerSessionCompletionReceipt) (bool, error) {
	if existing, err := readReviewerSessionCompletion(caseRoot, path); err == nil {
		if reviewerSessionCompletionEquivalent(existing, value) {
			return true, nil
		}
		return false, fmt.Errorf("reviewer session completion receipt already exists with different bindings")
	} else if _, statErr := os.Lstat(path); statErr == nil || !os.IsNotExist(statErr) {
		return false, fmt.Errorf("reviewer session completion receipt exists but is invalid")
	}
	return false, writeReviewerSessionReceipt(caseRoot, path, value)
}

func writeReviewerSessionReceipt(caseRoot, path string, value any) error {
	parent := filepath.Dir(path)
	if err := os.MkdirAll(parent, 0o700); err != nil {
		return err
	}
	if !reviewpath.CollectionNamespacePathSafe(caseRoot, parent, false) || !reviewpath.CollectionNamespacePathSafe(caseRoot, path, true) {
		return fmt.Errorf("reviewer session receipt path is not safe")
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

func readReviewerSessionReceiptBytes(caseRoot, path, label string) ([]byte, error) {
	if !reviewpath.CollectionNamespacePathSafe(caseRoot, path, false) {
		return nil, fmt.Errorf("%s path is not safe", label)
	}
	return readStableReviewerArtifact(filepath.Dir(path), path, label, maxReviewerSessionReceiptBytes)
}

func readReviewerSessionDispatch(caseRoot, path string) (ReviewerSessionDispatchReceipt, error) {
	data, err := readReviewerSessionReceiptBytes(caseRoot, path, "reviewer session dispatch receipt")
	if err != nil {
		return ReviewerSessionDispatchReceipt{}, err
	}
	return reviewersession.DecodeDispatch(data)
}

func readReviewerSessionCompletion(caseRoot, path string) (ReviewerSessionCompletionReceipt, error) {
	data, err := readReviewerSessionReceiptBytes(caseRoot, path, "reviewer session completion receipt")
	if err != nil {
		return ReviewerSessionCompletionReceipt{}, err
	}
	return reviewersession.DecodeCompletion(data)
}

func findReviewerSessionDispatch(caseRoot, packetPath, dispatchID string) (string, []byte, ReviewerSessionDispatchReceipt, error) {
	sessionsRoot := filepath.Join(filepath.Dir(packetPath), "sessions")
	entries, err := os.ReadDir(sessionsRoot)
	if err != nil {
		return "", nil, ReviewerSessionDispatchReceipt{}, fmt.Errorf("read reviewer session dispatches: %w", err)
	}
	for _, shard := range entries {
		if !shard.IsDir() {
			continue
		}
		path := reviewerSessionDispatchPath(packetPath, shard.Name(), dispatchID)
		receipt, readErr := readReviewerSessionDispatch(caseRoot, path)
		if readErr != nil {
			continue
		}
		data, readErr := readReviewerSessionReceiptBytes(caseRoot, path, "reviewer session dispatch receipt")
		if readErr == nil {
			return path, data, receipt, nil
		}
	}
	return "", nil, ReviewerSessionDispatchReceipt{}, fmt.Errorf("reviewer session dispatch receipt %q was not found", dispatchID)
}

func reviewerSessionDispatchEquivalent(left, right ReviewerSessionDispatchReceipt) bool {
	return left.SchemaVersion == right.SchemaVersion && left.Kind == right.Kind && left.DispatchID == right.DispatchID && left.PacketID == right.PacketID && casebind.SamePath(left.PacketPath, right.PacketPath) && left.PacketSHA256 == right.PacketSHA256 && left.RouteID == right.RouteID && left.ShardID == right.ShardID && stringSlicesEqual(left.Items, right.Items) && casebind.SamePath(left.PromptPath, right.PromptPath) && left.PromptSHA256 == right.PromptSHA256 && left.AgentType == right.AgentType && left.ReadOnly == right.ReadOnly && left.TargetLane == right.TargetLane && left.PacketOwner == right.PacketOwner && left.EffectiveOwner == right.EffectiveOwner && sameOptionalPath(left.OwnerAdoptionPath, right.OwnerAdoptionPath) && left.OwnerAdoptionSHA256 == right.OwnerAdoptionSHA256 && left.ReviewerHarness == right.ReviewerHarness && left.ReviewerSession == right.ReviewerSession && left.Actor == right.Actor && left.NoSpawn == right.NoSpawn && left.NoHeavyTool == right.NoHeavyTool && left.NoAuthority == right.NoAuthority
}

func reviewerSessionCompletionEquivalent(left, right ReviewerSessionCompletionReceipt) bool {
	return left.SchemaVersion == right.SchemaVersion && left.Kind == right.Kind && left.DispatchID == right.DispatchID && casebind.SamePath(left.DispatchReceiptPath, right.DispatchReceiptPath) && left.DispatchReceiptSHA256 == right.DispatchReceiptSHA256 && left.PacketID == right.PacketID && left.RouteID == right.RouteID && left.ShardID == right.ShardID && left.ReviewerHarness == right.ReviewerHarness && left.ReviewerSession == right.ReviewerSession && left.Outcome == right.Outcome && left.ExitStatus == right.ExitStatus && sameOptionalPath(left.ReviewerResultInputPath, right.ReviewerResultInputPath) && left.ReviewerResultInputSHA256 == right.ReviewerResultInputSHA256 && left.ReviewerResultInputBytes == right.ReviewerResultInputBytes && left.CompletionOwner == right.CompletionOwner && sameOptionalPath(left.OwnerAdoptionPath, right.OwnerAdoptionPath) && left.OwnerAdoptionSHA256 == right.OwnerAdoptionSHA256 && left.Actor == right.Actor && left.NoCollection == right.NoCollection && left.NoIntake == right.NoIntake && left.NoFacts == right.NoFacts && left.NoHeavyTool == right.NoHeavyTool && left.NoAuthority == right.NoAuthority
}

func sameOptionalPath(left, right string) bool {
	if left == "" || right == "" {
		return left == right
	}
	return casebind.SamePath(left, right)
}

func stringSlicesEqual(left, right []string) bool {
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

func reviewerSessionDispatchResult(repoRoot, caseRoot, pack string, opt ReviewerSessionDispatchOptions, p preparedReviewerSessionDispatch) ReviewerSessionReceiptResult {
	return ReviewerSessionReceiptResult{SchemaVersion: 1, Command: "plan-subagents", Mode: "preview", CaseRoot: caseRoot, RepoRoot: repoRoot, Pack: pack, IsMutation: !opt.WhatIf, RequiresConfirmation: opt.WhatIf, PacketID: p.packet.PacketID, PacketPath: p.packetPath, RouteID: p.packet.Route.ID, ShardID: p.handoff.ShardID, TargetLane: p.packet.TargetLane, DispatchID: p.dispatchID, ReviewerHarness: p.harness, ReviewerSession: p.session, ReceiptPath: p.receiptPath, BindingSHA256: p.bindingSHA256, EffectiveOwner: reviewerSessionOwner(p.effectiveOwner), OwnerAdoptionPath: p.adoptionPath, OwnerAdoptionSHA256: p.adoptionSHA256, Boundary: reviewerSessionReceiptBoundary()}
}

func reviewerSessionCompletionResult(repoRoot, caseRoot, pack string, opt ReviewerSessionCompletionOptions, p preparedReviewerSessionCompletion) ReviewerSessionReceiptResult {
	return ReviewerSessionReceiptResult{SchemaVersion: 1, Command: "plan-subagents", Mode: "preview", CaseRoot: caseRoot, RepoRoot: repoRoot, Pack: pack, IsMutation: !opt.WhatIf, RequiresConfirmation: opt.WhatIf, PacketID: p.packet.PacketID, PacketPath: p.packetPath, RouteID: p.packet.Route.ID, ShardID: p.dispatch.ShardID, TargetLane: p.packet.TargetLane, DispatchID: p.dispatch.DispatchID, ReviewerHarness: p.dispatch.ReviewerHarness, ReviewerSession: p.dispatch.ReviewerSession, Outcome: p.outcome, ExitStatus: p.exitStatus, ReceiptPath: p.completionPath, DispatchReceiptPath: p.dispatchPath, DispatchReceiptSHA256: sha256Hex(p.dispatchBytes), ReviewerResultInputPath: p.inputPath, ReviewerResultInputSHA256: sha256Hex(p.input), ReviewerResultInputBytes: len(p.input), EffectiveOwner: reviewerSessionOwner(p.effectiveOwner), OwnerAdoptionPath: p.adoptionPath, OwnerAdoptionSHA256: p.adoptionSHA256, Boundary: reviewerSessionReceiptBoundary()}
}

func reviewerSessionReceiptBoundary() []string {
	return []string{"reviewer session receipts record harness observations only; runtime does not spawn, poll, monitor, stop, or manage reviewer sessions", "dispatch/completion receipts do not collect or intake reviewer results and do not create reviewer verdicts", "reviewer session receipts do not execute heavy tools or write facts, authority, or confirmed state"}
}

func finalizeReviewerSessionReceiptResult(result ReviewerSessionReceiptResult) ReviewerSessionReceiptResult {
	result.MissionCommanderNextActions = []mission.MissionCommanderNextActionItem{{
		Lane:           result.TargetLane,
		Label:          result.DispatchID,
		ActionID:       result.DispatchID,
		State:          result.MissionCommanderAction.State,
		Command:        result.MissionCommanderAction.PrimaryCommand,
		Source:         "reviewerSessionReceipt",
		RequiresReview: true,
		Reasons:        append([]string{}, result.NextSteps...),
		Boundary:       append([]string{}, result.Boundary...),
	}}
	result.MissionCommanderActionQueue = mission.MissionCommanderActionQueueFor(result.MissionCommanderNextActions)
	return result
}

func reviewerSessionDispatchCommand(p preparedReviewerSessionDispatch, apply bool) string {
	command := "/rekit plan-subagents -PacketPath " + quoteCommandArg(p.packetPath) + " -RecordReviewerDispatch -ShardId " + quoteCommandArg(p.handoff.ShardID) + " -ReviewerHarness " + quoteCommandArg(p.harness) + " -ReviewerSession " + quoteCommandArg(p.session) + " -Lane " + quoteCommandArg(p.packet.TargetLane) + " -Actor " + quoteCommandArg(p.actor)
	if apply {
		command += " -ExpectedReviewerDispatchBindingSha256 " + quoteCommandArg(p.bindingSHA256) + " -Apply -Format json"
	} else {
		command += " -WhatIf -Format json"
	}
	return command
}

func reviewerSessionCompletionCommand(p preparedReviewerSessionCompletion, apply bool) string {
	command := "/rekit plan-subagents -PacketPath " + quoteCommandArg(p.packetPath) + " -RecordReviewerCompletion -ReviewerDispatchId " + quoteCommandArg(p.dispatch.DispatchID) + " -ReviewerOutcome " + quoteCommandArg(p.outcome) + " -ReviewerExitStatus " + quoteCommandArg(p.exitStatus) + " -Lane " + quoteCommandArg(p.packet.TargetLane) + " -Actor " + quoteCommandArg(p.actor)
	if p.inputPath != "" {
		command += " -ReviewerResultInputPath " + quoteCommandArg(p.inputPath)
	}
	if apply {
		command += " -ExpectedReviewerDispatchReceiptSha256 " + quoteCommandArg(sha256Hex(p.dispatchBytes))
		if len(p.input) > 0 {
			command += " -ExpectedReviewerResultInputSha256 " + quoteCommandArg(sha256Hex(p.input))
		}
		command += " -Apply -Format json"
	} else {
		command += " -WhatIf -Format json"
	}
	return command
}

func reviewerSessionReceiptsRequired(handoff ShardHandoff) bool {
	return handoff.AgentToolRequest != nil &&
		strings.TrimSpace(handoff.DispatchPromptPath) != "" &&
		strings.TrimSpace(handoff.DispatchPromptSHA256) != "" &&
		handoff.ReviewerStagingCommands != nil &&
		strings.TrimSpace(handoff.ReviewerResultCandidatePath) != ""
}

func reviewerSessionDispatchMatchesCurrent(packetPath, dispatchPath string, packet Packet, packetBytes []byte, handoff ShardHandoff, dispatch ReviewerSessionDispatchReceipt, effective OwnerBinding, adoptionPath, adoptionSHA256 string) bool {
	if handoff.AgentToolRequest == nil {
		return false
	}
	expectedID := reviewerSessionDispatchID(packet.PacketID, packet.Route.ID, handoff.ShardID, handoff.DispatchPromptSHA256, dispatch.ReviewerHarness, dispatch.ReviewerSession)
	return dispatch.DispatchID == expectedID &&
		samePath(dispatchPath, reviewerSessionDispatchPath(packetPath, handoff.ShardID, expectedID)) &&
		dispatch.PacketID == packet.PacketID &&
		samePath(dispatch.PacketPath, packetPath) &&
		dispatch.PacketSHA256 == sha256Hex(packetBytes) &&
		dispatch.RouteID == packet.Route.ID &&
		dispatch.ShardID == handoff.ShardID &&
		stringSlicesEqual(dispatch.Items, handoff.Items) &&
		samePath(dispatch.PromptPath, handoff.DispatchPromptPath) &&
		dispatch.PromptSHA256 == handoff.DispatchPromptSHA256 &&
		dispatch.AgentType == handoff.AgentToolRequest.AgentType &&
		dispatch.ReadOnly == handoff.AgentToolRequest.ReadOnly &&
		dispatch.TargetLane == packet.TargetLane &&
		dispatch.PacketOwner == reviewerSessionOwner(packet.OwnerBinding) &&
		dispatch.EffectiveOwner == reviewerSessionOwner(effective) &&
		sameOptionalPath(dispatch.OwnerAdoptionPath, adoptionPath) &&
		dispatch.OwnerAdoptionSHA256 == adoptionSHA256
}

func validateReviewerSessionCompletionForInput(caseRoot, packetPath string, packet Packet, handoff ShardHandoff, inputPath string, input []byte, result reviewerresult.Result) error {
	packetBytes, err := readStableReviewerArtifact(filepath.Dir(packetPath), packetPath, "review packet", maxReviewPacketBytes)
	if err != nil {
		return err
	}
	effective, adoption, err := validateOwnerBinding(caseRoot, packet, packetPath, packetBytes)
	if err != nil {
		return err
	}
	adoptionPath, adoptionSHA256 := reviewerSessionAdoptionProvenance(caseRoot, packet.PacketID, adoption)
	dispatchRoot := filepath.Join(reviewerSessionRoot(packetPath, handoff.ShardID), "dispatches")
	entries, err := os.ReadDir(dispatchRoot)
	if os.IsNotExist(err) {
		if reviewerSessionReceiptsRequired(handoff) {
			return fmt.Errorf("managed reviewer result lacks a durable dispatch and successful completion receipt")
		}
		return nil
	}
	if err != nil {
		return fmt.Errorf("read reviewer session dispatch receipts: %w", err)
	}
	names := make([]string, 0, len(entries))
	for _, entry := range entries {
		if !entry.IsDir() && strings.HasSuffix(strings.ToLower(entry.Name()), ".json") {
			names = append(names, entry.Name())
		}
	}
	sort.Strings(names)
	if len(names) == 0 {
		return fmt.Errorf("reviewer session receipt namespace exists without a valid dispatch receipt")
	}
	exactMatches := 0
	matchingDispatches := 0
	for _, name := range names {
		dispatchPath := filepath.Join(dispatchRoot, name)
		dispatch, readErr := readReviewerSessionDispatch(caseRoot, dispatchPath)
		if readErr != nil || !reviewerSessionDispatchMatchesCurrent(packetPath, dispatchPath, packet, packetBytes, handoff, dispatch, effective, adoptionPath, adoptionSHA256) || dispatch.ReviewerSession != result.ReviewerSession {
			continue
		}
		matchingDispatches++
		completionPath := reviewerSessionCompletionPath(packetPath, handoff.ShardID, dispatch.DispatchID)
		completion, readErr := readReviewerSessionCompletion(caseRoot, completionPath)
		if readErr != nil {
			continue
		}
		dispatchBytes, readErr := readReviewerSessionReceiptBytes(caseRoot, dispatchPath, "reviewer session dispatch receipt")
		if readErr != nil || reviewersession.ValidateCompletionDispatchLineage(completion, dispatch, dispatchPath, sha256Hex(dispatchBytes)) != nil || completion.Outcome != "succeeded" || !samePath(completion.ReviewerResultInputPath, inputPath) || completion.ReviewerResultInputSHA256 != sha256Hex(input) || completion.ReviewerResultInputBytes != len(input) || completion.CompletionOwner != reviewerSessionOwner(effective) {
			continue
		}
		exactMatches++
	}
	if exactMatches == 1 {
		return nil
	}
	if exactMatches > 1 {
		return fmt.Errorf("reviewer result input matches multiple successful completion receipt lineages")
	}
	if matchingDispatches > 0 {
		return fmt.Errorf("matching reviewer session dispatch lacks a successful completion receipt for the exact result input and current lane owner")
	}
	return fmt.Errorf("reviewer result session is not bound to any durable dispatch receipt for this packet shard")
}
