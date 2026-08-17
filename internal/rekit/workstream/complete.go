package workstream

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"slices"
	"sort"
	"strings"
	"time"

	"github.com/shuiyu486/re-context-kits/internal/rekit/casebind"
	refsf "github.com/shuiyu486/re-context-kits/internal/rekit/fs"
	"github.com/shuiyu486/re-context-kits/internal/rekit/instance"
	"github.com/shuiyu486/re-context-kits/internal/rekit/lanecompletion"
	"github.com/shuiyu486/re-context-kits/internal/rekit/manifest"
	"github.com/shuiyu486/re-context-kits/internal/rekit/memberexecution"
	"github.com/shuiyu486/re-context-kits/internal/rekit/mission"
	"github.com/shuiyu486/re-context-kits/internal/rekit/projectstate"
	"github.com/shuiyu486/re-context-kits/internal/rekit/reviewerresult"
	"github.com/shuiyu486/re-context-kits/internal/rekit/reviewersession"
)

const (
	completionIntentFile = "completion.intent.json"
	completionCommitFile = "completion.json"
)

var (
	completionAfterIntentHook                  func() error
	completionBeforeCommitHook                 func() error
	errMemberManifestReviewerWritebackRequired = errors.New("member manifest reviewer writeback required")
)

func SetCompletionAfterIntentHookForTest(hook func() error) func() {
	previous := completionAfterIntentHook
	completionAfterIntentHook = hook
	return func() { completionAfterIntentHook = previous }
}

func SetCompletionBeforeCommitHookForTest(hook func() error) func() {
	previous := completionBeforeCommitHook
	completionBeforeCommitHook = hook
	return func() { completionBeforeCommitHook = previous }
}

type CompleteOptions struct {
	Selector              string
	Actor                 string
	Reason                string
	EvidenceRefs          string
	ExpectedPreviewSHA256 string
}

type CompletionBlocker struct {
	Kind   string `json:"kind"`
	Detail string `json:"detail"`
}

type CompletionEvidence struct {
	Ref    string `json:"ref"`
	SHA256 string `json:"sha256"`
	Bytes  int64  `json:"bytes"`
}

type CompletionReceipt struct {
	SchemaVersion      int                  `json:"schemaVersion"`
	Kind               string               `json:"kind"`
	State              string               `json:"state"`
	Sequence           int                  `json:"sequence,omitempty"`
	PreviousReceiptSHA string               `json:"previousReceiptSha256,omitempty"`
	Lane               string               `json:"lane"`
	Label              string               `json:"label"`
	Authority          bool                 `json:"authority"`
	PreviousStatus     string               `json:"previousStatus"`
	Actor              string               `json:"actor"`
	Reason             string               `json:"reason"`
	EvidenceRefs       []string             `json:"evidenceRefs"`
	Evidence           []CompletionEvidence `json:"evidence"`
	CurrentExecutor    string               `json:"currentExecutor,omitempty"`
	ExecutorGeneration int                  `json:"executorGeneration,omitempty"`
	CompletedAt        string               `json:"completedAt"`
	EventID            string               `json:"eventId"`
	PreviewSHA256      string               `json:"previewSha256"`
	IntentSHA256       string               `json:"intentSha256"`
	LaneSHA256         string               `json:"laneSha256"`
	BoardLaneSHA256    string               `json:"boardLaneSha256"`
	ResumeSHA256       string               `json:"resumeSha256"`
	CheckpointSHA256   string               `json:"checkpointSha256"`
	NoAuthority        bool                 `json:"noAuthority"`
	NoConfirmed        bool                 `json:"noConfirmed"`
	NoHeavyTool        bool                 `json:"noHeavyTool"`
}

type completionIntent struct {
	SchemaVersion      int                  `json:"schemaVersion"`
	Kind               string               `json:"kind"`
	Sequence           int                  `json:"sequence,omitempty"`
	PreviousReceiptSHA string               `json:"previousReceiptSha256,omitempty"`
	Lane               string               `json:"lane"`
	Label              string               `json:"label"`
	Authority          bool                 `json:"authority"`
	PreviousStatus     string               `json:"previousStatus"`
	Actor              string               `json:"actor"`
	Reason             string               `json:"reason"`
	EvidenceRefs       []string             `json:"evidenceRefs"`
	Evidence           []CompletionEvidence `json:"evidence"`
	CurrentExecutor    string               `json:"currentExecutor,omitempty"`
	ExecutorGeneration int                  `json:"executorGeneration,omitempty"`
	CreatedAt          string               `json:"createdAt"`
	EventID            string               `json:"eventId"`
	PreviewSHA256      string               `json:"previewSha256"`
	NoAuthority        bool                 `json:"noAuthority"`
	NoConfirmed        bool                 `json:"noConfirmed"`
	NoHeavyTool        bool                 `json:"noHeavyTool"`
}

type CompleteResult struct {
	SchemaVersion               int                                      `json:"schemaVersion"`
	Command                     string                                   `json:"command"`
	CaseRoot                    string                                   `json:"caseRoot"`
	RepoRoot                    string                                   `json:"repoRoot"`
	Pack                        string                                   `json:"pack"`
	IsMutation                  bool                                     `json:"isMutation"`
	Applied                     bool                                     `json:"applied"`
	RequiresConfirmation        bool                                     `json:"requiresConfirmation"`
	Selector                    string                                   `json:"selector"`
	Lane                        Lane                                     `json:"lane"`
	Actor                       string                                   `json:"actor"`
	Reason                      string                                   `json:"reason"`
	EvidenceRefs                []string                                 `json:"evidenceRefs"`
	Evidence                    []CompletionEvidence                     `json:"evidence"`
	Blocked                     bool                                     `json:"blocked"`
	Blockers                    []CompletionBlocker                      `json:"blockers,omitempty"`
	CompletionPlanSHA256        string                                   `json:"completionPlanSha256,omitempty"`
	ApplyCommand                string                                   `json:"applyCommand,omitempty"`
	CompletionReceipt           *CompletionReceipt                       `json:"completionReceipt,omitempty"`
	MissionBrief                mission.Brief                            `json:"missionBrief"`
	MissionCommanderNextActions []mission.MissionCommanderNextActionItem `json:"missionCommanderNextActions,omitempty"`
	MissionCommanderActionQueue mission.MissionCommanderActionQueue      `json:"missionCommanderActionQueue"`
	WouldWrites                 []StartWrite                             `json:"wouldWrites,omitempty"`
	Writes                      []StartWrite                             `json:"writes,omitempty"`
	BlockedActions              []string                                 `json:"blockedActions"`
	NextSteps                   []string                                 `json:"nextSteps"`
}

type CompletePreviewCandidate struct {
	Ready          bool           `json:"ready"`
	Lane           Lane           `json:"lane"`
	PreviewCommand string         `json:"previewCommand,omitempty"`
	Preview        CompleteResult `json:"preview"`
}

type MissionCompletionHandoff struct {
	Ready                 bool                `json:"ready"`
	State                 string              `json:"state"`
	OperationallyComplete bool                `json:"operationallyComplete"`
	OpenLaneCount         int                 `json:"openLaneCount"`
	CompletedLaneCount    int                 `json:"completedLaneCount"`
	Receipts              []CompletionReceipt `json:"receipts,omitempty"`
	Summary               string              `json:"summary"`
	Boundary              []string            `json:"boundary"`
}

type completeContext struct {
	inst                instance.Instance
	manifest            *manifest.Manifest
	board               board
	lane                Lane
	selector            string
	actor               string
	reason              string
	evidenceRefs        []string
	evidence            []CompletionEvidence
	facts               mission.LedgerFacts
	blockers            []CompletionBlocker
	lifecycle           lanecompletion.Inspection
	sequence            int
	previousSHA         string
	intent              *completionIntent
	intentFile          string
	commitFile          string
	boardFile           string
	memberReviewBlocker string
}

func NextCompletePreviewCandidate(repoRoot, caseRoot, pack string) (CompletePreviewCandidate, error) {
	inst, err := instance.AssertAttached(caseRoot, repoRoot, pack)
	if err != nil {
		return CompletePreviewCandidate{}, err
	}
	b, err := readBoard(inst.CaseRoot)
	if err != nil {
		return CompletePreviewCandidate{}, err
	}
	for _, entry := range mission.OpenBoardLanes(b.Lanes) {
		if strings.TrimSpace(entry.CurrentExecutor) == "" || entry.ExecutorGeneration < 1 {
			continue
		}
		latest, ok, err := memberexecution.Latest(inst.CaseRoot, entry.ID)
		if err != nil {
			if memberexecution.IsPendingDispatch(err) {
				continue
			}
			return CompletePreviewCandidate{}, fmt.Errorf("derive completion candidate for %s: %w", entry.ID, err)
		}
		if !ok || latest.State != "intake-ready" || latest.Manifest == nil || latest.Latest == nil || latest.Latest.Outcome != "returned" {
			continue
		}
		ownerCurrent, err := memberexecution.CurrentOwnerMatches(inst.CaseRoot, pack, latest.Owner)
		if err != nil {
			return CompletePreviewCandidate{}, fmt.Errorf("derive completion candidate owner for %s: %w", entry.ID, err)
		}
		if !ownerCurrent {
			continue
		}
		manifestRef := relativePath(inst.CaseRoot, latest.ManifestPath)
		opt := CompleteOptions{
			Selector:     workstreamLabel(Lane{ID: entry.ID, Authority: entry.Authority}),
			Actor:        entry.CurrentExecutor,
			Reason:       "accepted reviewer lineage completed the current durable member result",
			EvidenceRefs: manifestRef,
		}
		preview, err := CompletePreview(repoRoot, inst.CaseRoot, pack, opt)
		if err != nil {
			return CompletePreviewCandidate{}, fmt.Errorf("derive completion candidate for %s: %w", entry.ID, err)
		}
		if preview.Blocked || preview.CompletionPlanSHA256 == "" || preview.ApplyCommand == "" {
			continue
		}
		args := []string{"/rekit", "complete", opt.Selector, "-Target", inst.CaseRoot, "-Pack", pack, "-Actor", opt.Actor, "-Reason", opt.Reason, "-EvidenceRefs", opt.EvidenceRefs, "-WhatIf", "-Format", "json"}
		for index := range args {
			args[index] = quoteCompleteCommandArg(args[index])
		}
		return CompletePreviewCandidate{Ready: true, Lane: preview.Lane, PreviewCommand: strings.Join(args, " "), Preview: preview}, nil
	}
	return CompletePreviewCandidate{}, nil
}

func CompletePreview(repoRoot, caseRoot, pack string, opt CompleteOptions) (CompleteResult, error) {
	ctx, err := newCompleteContext(repoRoot, caseRoot, pack, opt, false)
	if err != nil {
		return CompleteResult{}, err
	}
	result := ctx.result(false, false)
	result.WouldWrites, err = ctx.plannedWrites()
	if err != nil {
		return CompleteResult{}, err
	}
	if result.Blocked {
		result.WouldWrites = nil
		result.NextSteps = []string{"resolve every typed completion blocker, refresh status, then preview complete again"}
		return result, nil
	}
	hash, err := completePreviewSHA256(result)
	if err != nil {
		return CompleteResult{}, err
	}
	result.CompletionPlanSHA256 = hash
	result.ApplyCommand = completeApplyCommand(ctx, hash)
	result.NextSteps = []string{"review the completion evidence, blockers, and exact write set, then run applyCommand", "after apply, refresh status to select the next open lane or read the mission-complete handoff"}
	return result, nil
}

func CompleteApply(repoRoot, caseRoot, pack string, opt CompleteOptions) (result CompleteResult, err error) {
	mutationStarted := false
	defer func() {
		if err != nil && !mutationStarted {
			err = MarkZeroProgress(err)
		}
	}()
	ctx, err := newCompleteContext(repoRoot, caseRoot, pack, opt, true)
	if err != nil {
		return CompleteResult{}, err
	}
	lease, err := acquireProjectMutationLock(ctx.inst.CaseRoot)
	if err != nil {
		return CompleteResult{}, err
	}
	defer func() {
		if unlockErr := lease.Unlock(); unlockErr != nil {
			err = errors.Join(err, unlockErr)
			result = CompleteResult{}
		}
	}()
	ctx, err = newCompleteContext(repoRoot, caseRoot, pack, opt, true)
	if err != nil {
		return CompleteResult{}, err
	}
	if err := lease.Validate(); err != nil {
		return CompleteResult{}, err
	}
	expected := strings.ToLower(strings.TrimSpace(opt.ExpectedPreviewSHA256))
	if expected == "" {
		return CompleteResult{}, fmt.Errorf("complete apply requires -ExpectedCompletePlanSha256 from the reviewed preview")
	}
	if ctx.intent == nil {
		if len(ctx.blockers) > 0 {
			return CompleteResult{}, fmt.Errorf("complete is blocked: %s", completionBlockerSummary(ctx.blockers))
		}
		previewOpt := opt
		previewOpt.ExpectedPreviewSHA256 = ""
		preview, err := CompletePreview(repoRoot, caseRoot, pack, previewOpt)
		if err != nil {
			return CompleteResult{}, err
		}
		if preview.Blocked || !strings.EqualFold(expected, preview.CompletionPlanSHA256) {
			return CompleteResult{}, fmt.Errorf("complete preview sha256 mismatch: got %s want %s", expected, preview.CompletionPlanSHA256)
		}
		intent := ctx.newIntent(expected, isoNow())
		mutationStarted = true
		if err := writeCompletionExclusive(ctx.inst.CaseRoot, ctx.intentPath(), intent); err != nil {
			return CompleteResult{}, err
		}
		ctx.intent = &intent
		if completionAfterIntentHook != nil {
			if err := completionAfterIntentHook(); err != nil {
				return CompleteResult{}, err
			}
		}
	} else {
		mutationStarted = true
	}
	ctx, err = newCompleteContext(repoRoot, caseRoot, pack, opt, true)
	if err != nil {
		return CompleteResult{}, err
	}
	if len(ctx.blockers) > 0 {
		return CompleteResult{}, fmt.Errorf("complete recovery is blocked: %s", completionBlockerSummary(ctx.blockers))
	}
	if err := ctx.validateIntent(expected); err != nil {
		return CompleteResult{}, err
	}
	if err := lease.Validate(); err != nil {
		return CompleteResult{}, err
	}
	writes, receipt, err := ctx.publishCompletion()
	if err != nil {
		return CompleteResult{}, err
	}
	ctx.lane.Status = "closed"
	ctx.blockers = nil
	result = ctx.result(true, true)
	result.Lane = ctx.lane
	result.Writes = writes
	result.CompletionPlanSHA256 = receipt.PreviewSHA256
	result.CompletionReceipt = &receipt
	result.NextSteps = []string{"run /rekit status to select the next open lane or read the typed mission-complete handoff", "completion records operational closure only; no authority/confirmed conclusion was written or inferred"}
	return result, nil
}

func newCompleteContext(repoRoot, caseRoot, pack string, opt CompleteOptions, allowPending bool) (completeContext, error) {
	inst, err := instance.AssertAttached(caseRoot, repoRoot, pack)
	if err != nil {
		return completeContext{}, err
	}
	m, err := manifest.Load(repoRoot, pack)
	if err != nil {
		return completeContext{}, err
	}
	b, err := readBoard(inst.CaseRoot)
	if os.IsNotExist(err) {
		boardPath, entrypoint, pathErr := selectedMissionCommanderSurface(inst.CaseRoot)
		if pathErr != nil {
			return completeContext{}, pathErr
		}
		return completeContext{}, fmt.Errorf("complete requires existing %s; run %s start -Apply first", boardPath, entrypoint)
	}
	if err != nil {
		return completeContext{}, err
	}
	if strings.TrimSpace(b.DefaultAuthorityLane) == "" {
		b.DefaultAuthorityLane = m.WorkstreamDefaults["defaultAuthorityLane"]
	}
	selector := strings.TrimSpace(opt.Selector)
	if selector == "" {
		return completeContext{}, fmt.Errorf("complete requires a lane selector")
	}
	lane, err := resolveHandoffLane(inst.CaseRoot, b, selector)
	if err != nil {
		return completeContext{}, err
	}
	actor := strings.TrimSpace(opt.Actor)
	reason := strings.TrimSpace(opt.Reason)
	if actor == "" || reason == "" {
		return completeContext{}, fmt.Errorf("complete requires non-empty -Actor and -Reason")
	}
	refs, evidence, err := validateCompletionEvidenceRefs(inst.CaseRoot, opt.EvidenceRefs)
	if err != nil {
		return completeContext{}, err
	}
	facts, err := mission.ReadStrictLedgerFacts(inst.CaseRoot)
	if err != nil {
		return completeContext{}, err
	}
	if latest, ok, inspectErr := memberexecution.Latest(inst.CaseRoot, lane.ID); inspectErr != nil {
		return completeContext{}, fmt.Errorf("complete member execution intake: %w", inspectErr)
	} else if ok {
		if latest.State != "intake-ready" || latest.Manifest == nil || latest.Latest == nil || latest.Latest.Outcome != "returned" {
			return completeContext{}, fmt.Errorf("complete requires latest durable member execution to be intake-ready; got %s", latest.State)
		}
		entry, found := mission.LookupBoardLane(b.Lanes, lane.ID, false)
		if !found || latest.Owner.Executor != entry.CurrentExecutor || latest.Owner.ExecutorGeneration != entry.ExecutorGeneration {
			return completeContext{}, fmt.Errorf("complete member execution owner is stale")
		}
		manifestRef := relativePath(inst.CaseRoot, latest.ManifestPath)
		bound := false
		for _, item := range evidence {
			if completionEvidencePathsEqual(strings.SplitN(item.Ref, "#", 2)[0], manifestRef) && strings.EqualFold(item.SHA256, latest.ManifestSHA256) {
				bound = true
				break
			}
		}
		if !bound {
			return completeContext{}, fmt.Errorf("complete evidence must bind latest member result manifest %s at sha256 %s", manifestRef, latest.ManifestSHA256)
		}
		for _, output := range latest.Manifest.Outputs {
			outputRef := relativePath(inst.CaseRoot, filepath.Join(latest.OutputsRoot, filepath.FromSlash(output.Path)))
			refs = append(refs, outputRef)
		}
		refs = mission.UniqueStrings(refs)
		refs, evidence, err = validateCompletionEvidenceRefs(inst.CaseRoot, strings.Join(refs, ","))
		if err != nil {
			return completeContext{}, fmt.Errorf("complete member result snapshot: %w", err)
		}
		if err := validateMemberCompletionNamespace(inst.CaseRoot, lane.ID, evidence); err != nil {
			return completeContext{}, err
		}
		memberReviewBlocker := ""
		reviewerInput, reviewErr := requireMemberManifestReviewerWriteback(inst.CaseRoot, lane.ID, manifestRef, facts)
		if reviewErr != nil {
			memberReviewBlocker = reviewErr.Error()
		} else {
			refs = mission.UniqueStrings(append(refs, reviewerInput.Ref))
			refs, evidence, err = validateCompletionEvidenceRefs(inst.CaseRoot, strings.Join(refs, ","))
			if err != nil {
				return completeContext{}, fmt.Errorf("complete reviewer input snapshot: %w", err)
			}
		}
		ctx := completeContext{inst: inst, manifest: m, board: b, lane: lane, selector: selector, actor: actor, reason: reason, evidenceRefs: refs, evidence: evidence, facts: facts, memberReviewBlocker: memberReviewBlocker}
		return finishCompleteContext(ctx, allowPending)
	}
	ctx := completeContext{inst: inst, manifest: m, board: b, lane: lane, selector: selector, actor: actor, reason: reason, evidenceRefs: refs, evidence: evidence, facts: facts}
	return finishCompleteContext(ctx, allowPending)
}

func finishCompleteContext(ctx completeContext, allowPending bool) (completeContext, error) {
	inst, lane := ctx.inst, ctx.lane
	operations, operationErr := lanecompletion.InspectOperations(inst.CaseRoot)
	if operationErr != nil {
		return completeContext{}, operationErr
	}
	if operations.Pending {
		return completeContext{}, fmt.Errorf("complete refuses case while reopen operation publication is incomplete; recover the exact reopen Apply")
	}
	lifecycle, lifecycleErr := lanecompletion.Inspect(inst.CaseRoot, lane.ID)
	if lifecycleErr != nil {
		return completeContext{}, lifecycleErr
	}
	ctx.lifecycle = lifecycle
	switch lifecycle.State {
	case lanecompletion.StateNone:
		ctx.sequence = 1
	case lanecompletion.StateReopened:
		ctx.sequence = lifecycle.HeadSequence + 1
		ctx.previousSHA = lifecycle.HeadReceiptSHA256
	case lanecompletion.StateComplete:
		return completeContext{}, fmt.Errorf("lane is already complete: %s", lane.ID)
	case lanecompletion.StatePending:
		if lifecycle.PendingKind != "complete" {
			return completeContext{}, fmt.Errorf("lane %s has pending reopen publication; recover with the original hash-bound reopen apply", lane.ID)
		}
		if !allowPending {
			return completeContext{}, fmt.Errorf("lane %s has pending completion publication; recover with the original hash-bound complete apply", lane.ID)
		}
		ctx.sequence = lifecycle.PendingSequence
		ctx.previousSHA = lifecycle.HeadReceiptSHA256
	}
	ctx.intentFile, lifecycleErr = lanecompletion.IntentPathE(inst.CaseRoot, lane.ID, ctx.sequence, "complete")
	if lifecycleErr != nil {
		return completeContext{}, lifecycleErr
	}
	ctx.commitFile, lifecycleErr = lanecompletion.ReceiptPathE(inst.CaseRoot, lane.ID, ctx.sequence, "complete")
	if lifecycleErr != nil {
		return completeContext{}, lifecycleErr
	}
	ctx.boardFile, lifecycleErr = boardRelPath(inst.CaseRoot)
	if lifecycleErr != nil {
		return completeContext{}, lifecycleErr
	}
	intent, intentErr := readCompletionIntent(ctx.intentPath())
	if intentErr == nil {
		ctx.intent = &intent
	} else if !os.IsNotExist(intentErr) {
		return completeContext{}, intentErr
	}
	status := strings.ToLower(strings.TrimSpace(lane.Status))
	if status == "closed" && (ctx.intent == nil || lifecycle.State != lanecompletion.StatePending) {
		return completeContext{}, fmt.Errorf("lane %s has uncommitted closed state; recover with the original hash-bound complete apply", lane.ID)
	}
	if status == "archived" || status == "paused" {
		return completeContext{}, fmt.Errorf("target lane is not open: %s", lane.ID)
	}
	var err error
	ctx.blockers, err = ctx.completionBlockers()
	if err != nil {
		return completeContext{}, err
	}
	return ctx, nil
}

func HasAcceptedMemberManifestReviewerWriteback(caseRoot, laneID, manifestRef string) (bool, error) {
	facts, err := mission.ReadStrictLedgerFacts(caseRoot)
	if err != nil {
		return false, err
	}
	_, err = requireMemberManifestReviewerWriteback(caseRoot, laneID, manifestRef, facts)
	if err == nil {
		return true, nil
	}
	if errors.Is(err, errMemberManifestReviewerWritebackRequired) {
		return false, nil
	}
	return false, err
}

func requireMemberManifestReviewerWriteback(caseRoot, laneID, manifestRef string, facts mission.LedgerFacts) (CompletionEvidence, error) {
	if rejection, rejected, err := memberManifestReviewerRejection(caseRoot, laneID, manifestRef, facts); err != nil {
		return CompletionEvidence{}, err
	} else if rejected {
		return CompletionEvidence{}, fmt.Errorf("%w: current member manifest %s was canonically rejected by decision %s and requires correction plus a replacement owner generation", errMemberManifestReviewerWritebackRequired, manifestRef, rejection.DecisionEventID)
	}
	manifestFull := filepath.Join(caseRoot, filepath.FromSlash(manifestRef))
	verificationByPacket := map[string]map[string]any{}
	for _, event := range facts.Verifications {
		packetID := mission.Value(event, "packetId")
		if packetID == "" || mission.Value(event, "lane") != laneID || !strings.EqualFold(mission.Value(event, "verdict"), "accepted") || !eventTargetBindsPath(event, manifestRef, manifestFull) {
			continue
		}
		verificationByPacket[packetID] = event
	}
	if len(verificationByPacket) == 0 {
		return CompletionEvidence{}, fmt.Errorf("%w: complete requires a reviewer packet verification accepted for current member manifest %s", errMemberManifestReviewerWritebackRequired, manifestRef)
	}
	for _, event := range facts.Decisions {
		packetID := mission.Value(event, "packetId")
		verification, ok := verificationByPacket[packetID]
		if !ok || mission.Value(event, "lane") != laneID || !strings.EqualFold(mission.Value(event, "decision"), "accept") || !strings.EqualFold(mission.Value(event, "reviewerDecision"), "accept") || !eventEvidenceReferences(event, mission.Value(verification, "eventId")) {
			continue
		}
		if mission.Value(event, "shardId") == mission.Value(verification, "shardId") && mission.Value(event, "packetPath") == mission.Value(verification, "packetPath") && mission.Value(event, "reviewerResultInputSha256") == mission.Value(verification, "reviewerResultInputSha256") {
			input, err := validateReviewerWritebackPacket(caseRoot, laneID, manifestRef, packetID, mission.Value(event, "shardId"), mission.Value(event, "packetPath"), mission.Value(event, "reviewerResultInputSha256"))
			if err != nil {
				return CompletionEvidence{}, fmt.Errorf("complete reviewer packet binding is invalid: %w", err)
			}
			return input, nil
		}
	}
	return CompletionEvidence{}, fmt.Errorf("%w: complete requires an accepted reviewer decision/writeback bound to current member manifest %s", errMemberManifestReviewerWritebackRequired, manifestRef)
}

func validateReviewerWritebackPacket(caseRoot, laneID, manifestRef, packetID, shardID, packetRef, inputSHA256 string) (CompletionEvidence, error) {
	packetPath := filepath.FromSlash(strings.TrimSpace(packetRef))
	if !filepath.IsAbs(packetPath) {
		packetPath = filepath.Join(caseRoot, packetPath)
	}
	packet, err := readReviewerDispatchPacket(caseRoot, packetPath)
	if err != nil {
		return CompletionEvidence{}, err
	}
	if err := validateReviewerPacketIntegrity(caseRoot, packetPath, packet); err != nil {
		return CompletionEvidence{}, err
	}
	if packet.PacketID != packetID || firstText(packet.ReviewerOrchestration.TargetLane, packet.TargetLane, packet.ReviewerOrchestration.OwnerBinding.TargetLane) != laneID || !casebind.SamePath(packet.ReviewerOrchestration.PacketPath, packetPath) || packet.ReviewerOrchestration.OwnerBinding != packet.OwnerBinding {
		return CompletionEvidence{}, fmt.Errorf("reviewer packet identity does not match accepted writeback")
	}
	var shard *reviewerDispatchPacketDispatch
	for idx := range packet.ReviewerOrchestration.Dispatches {
		if packet.ReviewerOrchestration.Dispatches[idx].ShardID == shardID {
			shard = &packet.ReviewerOrchestration.Dispatches[idx]
			break
		}
	}
	if shard == nil {
		return CompletionEvidence{}, fmt.Errorf("reviewer packet does not contain accepted shard %s", shardID)
	}
	inputPath := ""
	if shard.StagingCommands != nil {
		inputPath = strings.TrimSpace(shard.StagingCommands.SourceCaptureInput)
	}
	if inputPath == "" {
		inputPath = filepath.Join(packet.ReviewerOrchestration.ResultRoot, "inputs", shardID+".reviewer-input.json")
	}
	input, err := refsf.ReadStableRegularFileAnchored(caseRoot, inputPath, "reviewer result input", 64<<10)
	if err != nil {
		return CompletionEvidence{}, err
	}
	inputEvidence := CompletionEvidence{Ref: relativePath(caseRoot, inputPath), SHA256: reviewerDispatchBytesSHA256(input), Bytes: int64(len(input))}
	if !strings.EqualFold(inputEvidence.SHA256, strings.TrimSpace(inputSHA256)) {
		return CompletionEvidence{}, fmt.Errorf("reviewer result input sha256 does not match accepted writeback")
	}
	result, err := reviewerresult.Decode(input)
	if err != nil {
		return CompletionEvidence{}, err
	}
	if result.PacketID != packetID || result.RouteID != packet.Route.ID || result.ShardID != shardID || strings.TrimSpace(result.ReviewerSession) == "" {
		return CompletionEvidence{}, fmt.Errorf("reviewer result input does not match packet/route/shard/session bindings")
	}
	if err := validateReviewerWritebackItems(caseRoot, manifestRef, shard.Items, result.Items); err != nil {
		return CompletionEvidence{}, err
	}
	if result.Decision != "accept" || result.RecommendedVerdict != "accepted" {
		return CompletionEvidence{}, fmt.Errorf("accepted reviewer writeback does not match canonical reviewer result decision %q and recommended verdict %q", result.Decision, result.RecommendedVerdict)
	}
	board, err := mission.ReadBoard(caseRoot)
	if err != nil {
		return CompletionEvidence{}, err
	}
	owner, found := mission.LookupBoardLane(board.Lanes, laneID, false)
	if !found {
		return CompletionEvidence{}, fmt.Errorf("reviewer packet lane is not present in current board")
	}
	packetBytes, err := refsf.ReadStableRegularFileAnchored(caseRoot, packetPath, "reviewer packet", 4<<20)
	if err != nil {
		return CompletionEvidence{}, err
	}
	dispatchRoot := filepath.Join(filepath.Dir(packetPath), "sessions", shardID, "dispatches")
	entries, err := os.ReadDir(dispatchRoot)
	if err != nil {
		return CompletionEvidence{}, fmt.Errorf("read reviewer session dispatch receipts: %w", err)
	}
	names := []string{}
	for _, entry := range entries {
		if !entry.IsDir() && strings.HasSuffix(strings.ToLower(entry.Name()), ".json") {
			names = append(names, entry.Name())
		}
	}
	sort.Strings(names)
	exact := 0
	for _, name := range names {
		dispatchPath := filepath.Join(dispatchRoot, name)
		dispatchBytes, readErr := refsf.ReadStableRegularFileAnchored(caseRoot, dispatchPath, "reviewer session dispatch receipt", 256<<10)
		if readErr != nil {
			continue
		}
		dispatch, decodeErr := reviewersession.DecodeDispatch(dispatchBytes)
		if decodeErr != nil || !reviewerDispatchSessionStaticBindingsCurrent(packet, packetPath, packetBytes, laneID, *shard, dispatch, dispatchPath) || dispatch.ReviewerSession != result.ReviewerSession || !reviewerDispatchSessionCurrentOwnerBindings(caseRoot, packet, packetPath, dispatch, owner.CurrentExecutor, owner.ExecutorGeneration) {
			continue
		}
		completionPath := reviewersession.CompletionPath(packetPath, shardID, dispatch.DispatchID)
		completionBytes, readErr := refsf.ReadStableRegularFileAnchored(caseRoot, completionPath, "reviewer session completion receipt", 256<<10)
		if readErr != nil {
			continue
		}
		completion, decodeErr := reviewersession.DecodeCompletion(completionBytes)
		if decodeErr != nil || reviewersession.ValidateCompletionDispatchLineage(completion, dispatch, dispatchPath, reviewerDispatchBytesSHA256(dispatchBytes)) != nil || completion.Outcome != "succeeded" || !casebind.SamePath(completion.ReviewerResultInputPath, inputPath) || !strings.EqualFold(completion.ReviewerResultInputSHA256, inputSHA256) || completion.ReviewerResultInputBytes != len(input) || completion.CompletionOwner != dispatch.EffectiveOwner {
			continue
		}
		exact++
	}
	if exact != 1 {
		return CompletionEvidence{}, fmt.Errorf("reviewer result input requires exactly one current successful dispatch/completion receipt lineage; got %d", exact)
	}
	return inputEvidence, nil
}

func validateReviewerWritebackItems(caseRoot, manifestRef string, shardItems, resultItems []string) error {
	if !reviewerResultBindsManifest(caseRoot, shardItems, manifestRef) || !reviewerResultBindsManifest(caseRoot, resultItems, manifestRef) || !equalStrings(resultItems, shardItems) {
		return fmt.Errorf("reviewer packet shard and result input do not bind the current member manifest")
	}
	return nil
}

func completionEvidencePathKey(path string) string {
	path, _, _ = strings.Cut(strings.TrimSpace(path), "#")
	key := filepath.ToSlash(filepath.Clean(filepath.FromSlash(path)))
	if runtime.GOOS == "windows" {
		key = strings.ToLower(key)
	}
	return key
}

func completionEvidencePathsEqual(left, right string) bool {
	return completionEvidencePathKey(left) == completionEvidencePathKey(right)
}

func eventTargetBindsPath(event map[string]any, rel, full string) bool {
	for _, item := range eventStringList(event["target"]) {
		path, _, _ := strings.Cut(item, "#")
		if completionEvidencePathsEqual(path, rel) || casebind.SamePath(path, full) {
			return true
		}
	}
	return false
}

func eventEvidenceReferences(event map[string]any, expected string) bool {
	for _, ref := range eventStringList(event["evidenceRefs"]) {
		if strings.EqualFold(strings.TrimSpace(ref), strings.TrimSpace(expected)) {
			return true
		}
	}
	return false
}

func eventStringList(value any) []string {
	switch typed := value.(type) {
	case []any:
		out := make([]string, 0, len(typed))
		for _, item := range typed {
			if text, ok := item.(string); ok && strings.TrimSpace(text) != "" {
				out = append(out, strings.TrimSpace(text))
			}
		}
		return out
	case []string:
		return typed
	case string:
		return strings.FieldsFunc(typed, func(r rune) bool { return r == ',' || r == ';' || r == '\n' || r == '\r' })
	default:
		return nil
	}
}

func (ctx completeContext) validateMemberReviewerLineage(evidence []CompletionEvidence) error {
	latest, ok, err := memberexecution.Latest(ctx.inst.CaseRoot, ctx.lane.ID)
	if err != nil || !ok {
		if err != nil {
			return err
		}
		return nil
	}
	facts, err := mission.ReadStrictLedgerFacts(ctx.inst.CaseRoot)
	if err != nil {
		return err
	}
	manifestRef := relativePath(ctx.inst.CaseRoot, latest.ManifestPath)
	input, err := requireMemberManifestReviewerWriteback(ctx.inst.CaseRoot, ctx.lane.ID, manifestRef, facts)
	if err != nil {
		return err
	}
	for _, item := range evidence {
		if completionEvidencePathsEqual(strings.SplitN(item.Ref, "#", 2)[0], input.Ref) && strings.EqualFold(item.SHA256, input.SHA256) && item.Bytes == input.Bytes {
			return nil
		}
	}
	return fmt.Errorf("completion evidence does not bind canonical reviewer input %s", input.Ref)
}

func openAuthorizedGateAdapterHandoffs(
	items []AuthorizedGateAdapterHandoff,
) []AuthorizedGateAdapterHandoff {
	return slices.DeleteFunc(items, func(item AuthorizedGateAdapterHandoff) bool {
		return item.Acknowledged
	})
}

func (ctx completeContext) completionBlockers() ([]CompletionBlocker, error) {
	blockers := []CompletionBlocker{}
	add := func(kind, detail string) { blockers = append(blockers, CompletionBlocker{Kind: kind, Detail: detail}) }
	if ctx.memberReviewBlocker != "" {
		add("member-manifest-review", ctx.memberReviewBlocker)
	}
	laneFacts := mission.LaneFacts(ctx.facts.Facts, ctx.lane.ID)
	for _, item := range mission.EffectiveOpenLaneInterventions(ctx.facts.Facts, ctx.lane.ID) {
		add("open-intervention", firstText(mission.Value(item, "eventId"), mission.Value(item, "subject")))
	}
	for _, item := range laneFacts.Requests {
		if mission.IsPendingGateRequest(item) {
			add("pending-gate", firstText(mission.Value(item, "eventId"), mission.Value(item, "subject")))
		}
	}
	for _, item := range continueOpenDecisionItems(laneFacts) {
		add("open-"+item.SourceKind, firstText(mission.Value(item.Event, "eventId"), mission.Value(item.Event, "subject")))
	}
	laneRoot, err := laneRootPath(ctx.inst.CaseRoot, ctx.lane)
	if err != nil {
		return nil, err
	}
	tasks, err := mission.ReadStrictJSONLineObjects(LaneTasksJSONLPath(laneRoot))
	if err != nil {
		return nil, err
	}
	for _, task := range tasks {
		status := strings.ToLower(strings.TrimSpace(mission.Value(task, "status")))
		if status != "closed" && status != "resolved" {
			add("open-task", firstText(mission.Value(task, "taskId"), mission.Value(task, "eventId"), mission.Value(task, "summary")))
		}
	}
	if items, err := ReviewerDispatchIntakeHandoffs(ctx.inst.CaseRoot, ctx.facts, ctx.lane.ID); err != nil {
		return nil, err
	} else if len(items) > 0 {
		add("reviewer-dispatch-intake", fmt.Sprintf("%d reviewer action(s) remain open", len(items)))
	}
	if items, err := ReviewerPacketRetirementHandoffs(ctx.inst.CaseRoot, ctx.lane.ID); err != nil {
		return nil, err
	} else if len(items) > 0 {
		add("reviewer-packet-retirement", fmt.Sprintf("%d invalid reviewer packet(s) require disposition", len(items)))
	}
	if items := laneExecutionEvidenceReview(ctx.lane, ctx.facts); len(items) > 0 {
		add("execution-evidence-review", fmt.Sprintf("%d execution evidence item(s) require acknowledgement", len(items)))
	}
	if items := openAuthorizedGateAdapterHandoffs(AuthorizedGateAdapterHandoffsWithAcknowledgements(ctx.manifest.RepoRoot, ctx.inst.CaseRoot, ctx.manifest.Pack, ctx.facts.Requests, ctx.lane.ID, ExecutionEvidenceReviewAcknowledgedIDs(ctx.facts))); len(items) > 0 {
		add("authorized-gate-adapter", fmt.Sprintf("%d authorized adapter action(s) remain open", len(items)))
	}
	if ctx.lane.Authority {
		for _, boardLane := range ctx.board.Lanes {
			if boardLane.ID == ctx.lane.ID {
				continue
			}
			if strings.ToLower(strings.TrimSpace(boardLane.Status)) != "closed" {
				add("open-feature-lane", boardLane.ID)
				continue
			}
			if _, err := InspectLaneCompletion(ctx.inst.CaseRoot, boardLane.ID); err != nil {
				add("uncommitted-feature-closure", boardLane.ID+": "+err.Error())
			}
		}
	}
	sort.SliceStable(blockers, func(i, j int) bool {
		if blockers[i].Kind == blockers[j].Kind {
			return blockers[i].Detail < blockers[j].Detail
		}
		return blockers[i].Kind < blockers[j].Kind
	})
	return blockers, nil
}

func (ctx completeContext) result(mutating, applied bool) CompleteResult {
	brief := projectMissionBrief(ctx.board.Lanes, ctx.facts)
	actions := []mission.MissionCommanderNextActionItem{}
	queue := mission.MissionCommanderActionQueueFor(actions)
	if applied {
		if b, err := readBoard(ctx.inst.CaseRoot); err == nil {
			brief = projectMissionBrief(b.Lanes, ctx.facts)
			actions, queue = completionFreshActions(ctx.inst.CaseRoot, b, ctx.facts)
		}
	}
	return CompleteResult{
		SchemaVersion: 1, Command: "complete", CaseRoot: ctx.inst.CaseRoot, RepoRoot: ctx.manifest.RepoRoot, Pack: ctx.manifest.Pack,
		IsMutation: mutating, Applied: applied, RequiresConfirmation: !applied, Selector: ctx.selector, Lane: ctx.lane,
		Actor: ctx.actor, Reason: ctx.reason, EvidenceRefs: append([]string{}, ctx.evidenceRefs...), Evidence: append([]CompletionEvidence{}, ctx.evidence...), Blocked: len(ctx.blockers) > 0,
		Blockers: append([]CompletionBlocker{}, ctx.blockers...), MissionBrief: brief, MissionCommanderNextActions: actions, MissionCommanderActionQueue: queue,
		BlockedActions: []string{"authority/confirmed writes", "heavy-tool execution", "implicit lane reopen", "completion while durable blockers remain open"},
	}
}

func completionFreshActions(caseRoot string, b board, facts mission.LedgerFacts) ([]mission.MissionCommanderNextActionItem, mission.MissionCommanderActionQueue) {
	items := []mission.MissionCommanderNextActionItem{}
	brief := projectMissionBrief(b.Lanes, facts)
	for _, boardLane := range mission.OpenBoardLanes(b.Lanes) {
		lane, err := readLaneByID(caseRoot, boardLane.ID)
		if err != nil {
			continue
		}
		action := laneExecutorActionFor(lane, mission.LaneFacts(facts.Facts, lane.ID), brief)
		items = append(items, mission.MissionCommanderNextActions([]mission.LaneExecutorActionSnapshot{laneCommanderActionSnapshot(lane, action)}, laneExecutionEvidenceReview(lane, facts), action.Blocked)...)
	}
	return items, mission.MissionCommanderActionQueueFor(items)
}

func (ctx completeContext) plannedWrites() ([]StartWrite, error) {
	laneRoot, err := laneRootPath(ctx.inst.CaseRoot, ctx.lane)
	if err != nil {
		return nil, err
	}
	return []StartWrite{
		{Path: relativePath(ctx.inst.CaseRoot, ctx.intentPath()), Kind: "lane-completion-intent", Action: "would-create", TargetPath: ctx.intentPath()},
		{Path: relativePath(ctx.inst.CaseRoot, LaneEventsJSONLPath(laneRoot)), Kind: "lane-event", Action: "would-append-lane-completed", TargetPath: LaneEventsJSONLPath(laneRoot)},
		{Path: relativePath(ctx.inst.CaseRoot, filepath.Join(laneRoot, "lane.json")), Kind: "lane", Action: "would-close", TargetPath: filepath.Join(laneRoot, "lane.json")},
		{Path: ctx.boardFile, Kind: "board", Action: "would-refresh"},
		{Path: relativePath(ctx.inst.CaseRoot, filepath.Join(laneRoot, "prompts", "RESUME.md")), Kind: "lane-resume", Action: "would-refresh"},
		{Path: relativePath(ctx.inst.CaseRoot, filepath.Join(laneRoot, "checkpoints", "latest.json")), Kind: "lane-checkpoint", Action: "would-refresh"},
		{Path: relativePath(ctx.inst.CaseRoot, ctx.commitPath()), Kind: "lane-completion-commit", Action: "would-create-last", TargetPath: ctx.commitPath()},
	}, nil
}

func (ctx completeContext) newIntent(previewSHA, now string) completionIntent {
	return completionIntent{
		SchemaVersion: 1, Kind: "lane-completion-intent", Sequence: ctx.sequence, PreviousReceiptSHA: ctx.previousSHA, Lane: ctx.lane.ID, Label: workstreamLabel(ctx.lane), Authority: ctx.lane.Authority,
		PreviousStatus: strings.ToLower(strings.TrimSpace(ctx.lane.Status)), Actor: ctx.actor, Reason: ctx.reason, EvidenceRefs: append([]string{}, ctx.evidenceRefs...), Evidence: append([]CompletionEvidence{}, ctx.evidence...),
		CurrentExecutor: ctx.lane.CurrentExecutor, ExecutorGeneration: ctx.lane.ExecutorGeneration, CreatedAt: now,
		EventID: eventID(ctx.lane.ID, "lane-completed", now), PreviewSHA256: previewSHA, NoAuthority: true, NoConfirmed: true, NoHeavyTool: true,
	}
}

func (ctx completeContext) validateIntent(expected string) error {
	intent := ctx.intent
	if intent == nil || intent.SchemaVersion != 1 || intent.Kind != "lane-completion-intent" || intent.Sequence != ctx.sequence || !strings.EqualFold(intent.PreviousReceiptSHA, ctx.previousSHA) || intent.Lane != ctx.lane.ID ||
		intent.Actor != ctx.actor || intent.Reason != ctx.reason || !equalStrings(intent.EvidenceRefs, ctx.evidenceRefs) || !equalCompletionEvidence(intent.Evidence, ctx.evidence) ||
		!strings.EqualFold(intent.PreviewSHA256, expected) || !intent.NoAuthority || !intent.NoConfirmed || !intent.NoHeavyTool {
		return fmt.Errorf("existing lane completion intent does not match the requested actor, reason, evidence, lane, or preview hash")
	}
	if intent.CurrentExecutor != ctx.lane.CurrentExecutor || intent.ExecutorGeneration != ctx.lane.ExecutorGeneration {
		return fmt.Errorf("lane executor owner changed after completion intent")
	}
	if _, err := time.Parse(time.RFC3339Nano, intent.CreatedAt); err != nil {
		return fmt.Errorf("lane completion intent createdAt is invalid: %w", err)
	}
	return nil
}

func (ctx completeContext) publishCompletion() ([]StartWrite, CompletionReceipt, error) {
	intent := *ctx.intent
	intentSHA, err := hashJSON(intent)
	if err != nil {
		return nil, CompletionReceipt{}, err
	}
	laneRoot, err := laneRootPath(ctx.inst.CaseRoot, ctx.lane)
	if err != nil {
		return nil, CompletionReceipt{}, err
	}
	writes := []StartWrite{{Path: relativePath(ctx.inst.CaseRoot, ctx.intentPath()), Kind: "lane-completion-intent", Action: "existing", TargetPath: ctx.intentPath()}}
	event := map[string]any{
		"schemaVersion": 1, "eventId": intent.EventID, "kind": "lane-completed", "lane": ctx.lane.ID, "time": intent.CreatedAt,
		"summary": "operationally completed lane: " + workstreamLabel(ctx.lane), "previousStatus": intent.PreviousStatus,
		"actor": intent.Actor, "reason": intent.Reason, "evidenceRefs": intent.EvidenceRefs, "evidence": intent.Evidence, "currentExecutor": intent.CurrentExecutor,
		"executorGeneration": intent.ExecutorGeneration, "previewSha256": intent.PreviewSHA256, "noAuthority": true, "noConfirmed": true, "noHeavyTool": true,
	}
	eventWrite, err := appendCompletionLaneEvent(ctx.inst.CaseRoot, LaneEventsJSONLPath(laneRoot), event)
	if err != nil {
		return nil, CompletionReceipt{}, err
	}
	writes = append(writes, eventWrite)
	ctx.lane.Status = "closed"
	ctx.lane.UpdatedAt = intent.CreatedAt
	lanePath := filepath.Join(laneRoot, "lane.json")
	if err := writeJSON(lanePath, ctx.lane); err != nil {
		return nil, CompletionReceipt{}, err
	}
	writes = append(writes, StartWrite{Path: relativePath(ctx.inst.CaseRoot, lanePath), Kind: "lane", Action: "close", TargetPath: lanePath})
	boardPath, err := saveBoard(ctx.inst.CaseRoot, ctx.manifest)
	if err != nil {
		return nil, CompletionReceipt{}, err
	}
	writes = append(writes, StartWrite{Path: relativePath(ctx.inst.CaseRoot, boardPath), Kind: "board", Action: "refresh", TargetPath: boardPath})
	resumePath, checkpointPath, err := writeLaneResume(ctx.inst.CaseRoot, ctx.manifest, ctx.lane)
	if err != nil {
		return nil, CompletionReceipt{}, err
	}
	writes = append(writes,
		StartWrite{Path: relativePath(ctx.inst.CaseRoot, resumePath), Kind: "lane-resume", Action: "refresh", TargetPath: resumePath},
		StartWrite{Path: relativePath(ctx.inst.CaseRoot, checkpointPath), Kind: "lane-checkpoint", Action: "refresh", TargetPath: checkpointPath},
	)
	laneSHA, err := hashFile(lanePath)
	if err != nil {
		return nil, CompletionReceipt{}, err
	}
	boardLaneSHA, err := currentBoardLaneSHA(ctx.inst.CaseRoot, ctx.lane.ID)
	if err != nil {
		return nil, CompletionReceipt{}, err
	}
	resumeSHA, err := hashFile(resumePath)
	if err != nil {
		return nil, CompletionReceipt{}, err
	}
	checkpointSHA, err := hashFile(checkpointPath)
	if err != nil {
		return nil, CompletionReceipt{}, err
	}
	receipt := CompletionReceipt{
		SchemaVersion: 1, Kind: "lane-completion", State: "committed", Sequence: intent.Sequence, PreviousReceiptSHA: intent.PreviousReceiptSHA, Lane: intent.Lane, Label: intent.Label, Authority: intent.Authority,
		PreviousStatus: intent.PreviousStatus, Actor: intent.Actor, Reason: intent.Reason, EvidenceRefs: append([]string{}, intent.EvidenceRefs...), Evidence: append([]CompletionEvidence{}, intent.Evidence...),
		CurrentExecutor: intent.CurrentExecutor, ExecutorGeneration: intent.ExecutorGeneration, CompletedAt: intent.CreatedAt, EventID: intent.EventID,
		PreviewSHA256: intent.PreviewSHA256, IntentSHA256: intentSHA, LaneSHA256: laneSHA, BoardLaneSHA256: boardLaneSHA,
		ResumeSHA256: resumeSHA, CheckpointSHA256: checkpointSHA, NoAuthority: true, NoConfirmed: true, NoHeavyTool: true,
	}
	if completionBeforeCommitHook != nil {
		if err := completionBeforeCommitHook(); err != nil {
			return nil, CompletionReceipt{}, err
		}
	}
	if err := validateMemberCompletionNamespace(ctx.inst.CaseRoot, ctx.lane.ID, intent.Evidence); err != nil {
		return nil, CompletionReceipt{}, fmt.Errorf("lane completion member evidence changed before final commit: %w", err)
	}
	if err := ctx.validateMemberReviewerLineage(intent.Evidence); err != nil {
		return nil, CompletionReceipt{}, fmt.Errorf("lane completion reviewer lineage changed before final commit: %w", err)
	}
	if err := writeCompletionExclusive(ctx.inst.CaseRoot, ctx.commitPath(), receipt); err != nil {
		if existing, inspectErr := InspectLaneCompletion(ctx.inst.CaseRoot, ctx.lane.ID); inspectErr != nil || !completionReceiptsEqual(existing, receipt) {
			return nil, CompletionReceipt{}, err
		}
	}
	writes = append(writes, StartWrite{Path: relativePath(ctx.inst.CaseRoot, ctx.commitPath()), Kind: "lane-completion-commit", Action: "create-last", TargetPath: ctx.commitPath()})
	return writes, receipt, nil
}

func InspectLaneCompletion(caseRoot, laneID string) (CompletionReceipt, error) {
	if err := validateLaneIDSegment(laneID); err != nil {
		return CompletionReceipt{}, err
	}
	lifecycle, err := lanecompletion.Inspect(caseRoot, laneID)
	if err != nil {
		return CompletionReceipt{}, err
	}
	if lifecycle.State != lanecompletion.StateComplete || lifecycle.CurrentCompletion == nil || len(lifecycle.Transitions) == 0 {
		return CompletionReceipt{}, fmt.Errorf("lane has no current committed completion: %s state=%s", laneID, lifecycle.State)
	}
	var receipt CompletionReceipt
	if err := convertCompletionJSON(lifecycle.CurrentCompletion, &receipt); err != nil {
		return CompletionReceipt{}, err
	}
	transition := lifecycle.Transitions[len(lifecycle.Transitions)-1]
	intent, err := readCompletionIntent(transition.IntentPath)
	if err != nil {
		return CompletionReceipt{}, err
	}
	_, currentEvidence, err := validateCompletionEvidenceRefs(caseRoot, strings.Join(receipt.EvidenceRefs, ","))
	if err != nil || !equalCompletionEvidence(currentEvidence, receipt.Evidence) {
		return CompletionReceipt{}, fmt.Errorf("lane completion evidence content mismatch: %s", laneID)
	}
	if err := validateMemberCompletionNamespace(caseRoot, laneID, receipt.Evidence); err != nil {
		return CompletionReceipt{}, err
	}
	if err := validateCommittedMemberReviewerLineage(caseRoot, laneID, receipt.Evidence); err != nil {
		return CompletionReceipt{}, err
	}
	if !equalStrings(intent.EvidenceRefs, receipt.EvidenceRefs) || !equalCompletionEvidence(intent.Evidence, receipt.Evidence) {
		return CompletionReceipt{}, fmt.Errorf("lane completion evidence identity mismatch: %s", laneID)
	}
	intentSHA, err := hashJSON(intent)
	if err != nil || intentSHA != receipt.IntentSHA256 {
		return CompletionReceipt{}, fmt.Errorf("lane completion intent hash mismatch: %s", laneID)
	}
	root, err := projectstate.Join(caseRoot, "lanes", laneID)
	if err != nil {
		return CompletionReceipt{}, err
	}
	checks := []struct{ path, want, label string }{
		{filepath.Join(root, "lane.json"), receipt.LaneSHA256, "lane"},
		{filepath.Join(root, "prompts", "RESUME.md"), receipt.ResumeSHA256, "resume"},
		{filepath.Join(root, "checkpoints", "latest.json"), receipt.CheckpointSHA256, "checkpoint"},
	}
	for _, check := range checks {
		actual, hashErr := hashFile(check.path)
		if hashErr != nil || !strings.EqualFold(actual, check.want) {
			return CompletionReceipt{}, fmt.Errorf("lane completion %s hash mismatch: %s", check.label, laneID)
		}
	}
	lane, err := readLane(filepath.Join(root, "lane.json"))
	if err != nil || strings.ToLower(strings.TrimSpace(lane.Status)) != "closed" || lane.CurrentExecutor != receipt.CurrentExecutor || lane.ExecutorGeneration != receipt.ExecutorGeneration {
		return CompletionReceipt{}, fmt.Errorf("lane completion lane state mismatch: %s", laneID)
	}
	boardSHA, err := currentBoardLaneSHA(caseRoot, laneID)
	if err != nil || !strings.EqualFold(boardSHA, receipt.BoardLaneSHA256) {
		return CompletionReceipt{}, fmt.Errorf("lane completion board projection mismatch: %s", laneID)
	}
	return receipt, nil
}

func validateCommittedMemberReviewerLineage(caseRoot, laneID string, evidence []CompletionEvidence) error {
	latest, ok, err := memberexecution.Latest(caseRoot, laneID)
	if err != nil {
		return fmt.Errorf("lane completion reviewer lineage member inspection failed: %s: %w", laneID, err)
	}
	if !ok {
		return nil
	}
	facts, err := mission.ReadStrictLedgerFacts(caseRoot)
	if err != nil {
		return err
	}
	manifestRef := relativePath(caseRoot, latest.ManifestPath)
	input, err := requireMemberManifestReviewerWriteback(caseRoot, laneID, manifestRef, facts)
	if err != nil {
		return fmt.Errorf("lane completion reviewer lineage mismatch: %s: %w", laneID, err)
	}
	for _, item := range evidence {
		if completionEvidencePathsEqual(strings.SplitN(item.Ref, "#", 2)[0], input.Ref) && strings.EqualFold(item.SHA256, input.SHA256) && item.Bytes == input.Bytes {
			return nil
		}
	}
	return fmt.Errorf("lane completion reviewer input evidence mismatch: %s", laneID)
}

func validateMemberCompletionNamespace(caseRoot, laneID string, evidence []CompletionEvidence) error {
	latest, ok, err := memberexecution.Latest(caseRoot, laneID)
	if err != nil {
		return fmt.Errorf("lane completion member execution inspection failed: %s: %w", laneID, err)
	}
	if !ok {
		return nil
	}
	if latest.State != "intake-ready" || latest.Manifest == nil || latest.Latest == nil || latest.Latest.Outcome != "returned" {
		return fmt.Errorf("lane completion latest member execution is not intake-ready: %s", laneID)
	}
	expected := map[string]CompletionEvidence{}
	manifestRef := relativePath(caseRoot, latest.ManifestPath)
	manifestBytes, err := refsf.ReadStableRegularFileAnchored(caseRoot, latest.ManifestPath, "member execution manifest", 4<<20)
	if err != nil || !strings.EqualFold(reviewerDispatchBytesSHA256(manifestBytes), latest.ManifestSHA256) {
		return fmt.Errorf("lane completion member manifest identity mismatch: %s", laneID)
	}
	expected[completionEvidencePathKey(manifestRef)] = CompletionEvidence{Ref: manifestRef, SHA256: latest.ManifestSHA256, Bytes: int64(len(manifestBytes))}
	for _, output := range latest.Manifest.Outputs {
		ref := relativePath(caseRoot, filepath.Join(latest.OutputsRoot, filepath.FromSlash(output.Path)))
		expected[completionEvidencePathKey(ref)] = CompletionEvidence{Ref: ref, SHA256: output.SHA256, Bytes: output.Bytes}
	}
	actual := map[string]CompletionEvidence{}
	memberRootPrefix := completionEvidencePathKey(relativePath(caseRoot, filepath.Dir(latest.AttemptRoot))) + "/"
	memberPrefix := completionEvidencePathKey(relativePath(caseRoot, latest.AttemptRoot)) + "/evidence/"
	for _, item := range evidence {
		key := completionEvidencePathKey(item.Ref)
		if strings.HasPrefix(key, memberRootPrefix) && !strings.HasPrefix(key, memberPrefix) {
			return fmt.Errorf("lane completion member evidence references a non-current or non-canonical member namespace: %s", item.Ref)
		}
		if strings.HasPrefix(key, memberPrefix) {
			if _, duplicate := actual[key]; duplicate {
				return fmt.Errorf("lane completion member evidence contains duplicate path: %s", item.Ref)
			}
			actual[key] = item
		}
	}
	if len(actual) != len(expected) {
		return fmt.Errorf("lane completion member evidence namespace mismatch: %s", laneID)
	}
	for path, want := range expected {
		got, ok := actual[path]
		if !ok || !strings.EqualFold(got.SHA256, want.SHA256) || got.Bytes != want.Bytes {
			return fmt.Errorf("lane completion member evidence identity mismatch: %s", path)
		}
	}
	return nil
}

func InspectMissionCompletion(caseRoot string) (MissionCompletionHandoff, error) {
	b, err := mission.ReadBoard(caseRoot)
	if err != nil {
		return MissionCompletionHandoff{}, err
	}
	out := MissionCompletionHandoff{State: "active", OpenLaneCount: len(mission.OpenBoardLanes(b.Lanes)), Boundary: completionBoundary()}
	operations, operationErr := lanecompletion.InspectOperations(caseRoot)
	if operationErr != nil {
		out.State = "reopen-publication-incomplete"
		out.Summary = "lane reopen operation lifecycle is invalid"
		return out, operationErr
	}
	if operations.Pending {
		out.State = "reopen-publication-incomplete"
		out.Summary = fmt.Sprintf("lane reopen operation publication is pending: sequence=%d", operations.PendingSequence)
		return out, fmt.Errorf("lane reopen operation publication is incomplete: sequence=%d", operations.PendingSequence)
	}
	for _, lane := range b.Lanes {
		lifecycle, lifecycleErr := lanecompletion.Inspect(caseRoot, lane.ID)
		if lifecycleErr != nil {
			out.State = "completion-publication-incomplete"
			out.Summary = "lane completion lifecycle is invalid: " + lane.ID
			return out, lifecycleErr
		}
		closed := strings.EqualFold(strings.TrimSpace(lane.Status), "closed")
		switch lifecycle.State {
		case lanecompletion.StateNone:
			if closed {
				out.State = "completion-publication-incomplete"
				out.Summary = "closed lane lacks a committed completion publication: " + lane.ID
				return out, fmt.Errorf("closed lane lacks committed completion: %s", lane.ID)
			}
		case lanecompletion.StatePending:
			out.State = "completion-publication-incomplete"
			out.Summary = "lane completion lifecycle publication is pending: " + lane.ID
			return out, fmt.Errorf("lane completion publication is incomplete: %s transition=%s sequence=%d", lane.ID, lifecycle.PendingKind, lifecycle.PendingSequence)
		case lanecompletion.StateReopened:
			if closed {
				out.State = "completion-publication-incomplete"
				out.Summary = "reopened lane remains closed: " + lane.ID
				return out, fmt.Errorf("lane reopen projection is incomplete: %s", lane.ID)
			}
		case lanecompletion.StateComplete:
			if !closed {
				out.State = "completion-publication-incomplete"
				out.Summary = "completed lane is not closed: " + lane.ID
				return out, fmt.Errorf("lane completion projection is incomplete: %s", lane.ID)
			}
			receipt, err := InspectLaneCompletion(caseRoot, lane.ID)
			if err != nil {
				out.State = "completion-publication-incomplete"
				out.Summary = "closed lane lacks a current committed completion publication: " + lane.ID
				return out, err
			}
			out.Receipts = append(out.Receipts, receipt)
		}
	}
	sort.Slice(out.Receipts, func(i, j int) bool { return out.Receipts[i].Lane < out.Receipts[j].Lane })
	out.CompletedLaneCount = len(out.Receipts)
	if len(b.Lanes) > 0 && out.OpenLaneCount == 0 && out.CompletedLaneCount == len(b.Lanes) {
		out.Ready = true
		out.State = "mission-complete"
		out.OperationallyComplete = true
		out.Summary = "all durable lanes are operationally complete with reviewed lane-completion receipts"
	}
	return out, nil
}

func completionBoundary() []string {
	return []string{
		"operational completion is derived only from current committed lane-completion receipts",
		"completion does not write or infer authority/confirmed conclusions",
		"completion does not execute heavy tools or manage external sessions",
		"closed lanes are reopened only by the dedicated review-first reopen owner, never by start, continue, gate, note, reviewer, or executor takeover",
	}
}

func validateCompletionEvidenceRefs(caseRoot, value string) ([]string, []CompletionEvidence, error) {
	const maxEvidenceBytes = 16 << 20

	parts := strings.FieldsFunc(value, func(r rune) bool { return r == ',' || r == ';' || r == '\n' || r == '\r' })
	refs := mission.UniqueStrings(parts)
	if len(refs) == 0 {
		return nil, nil, fmt.Errorf("complete requires at least one case-local non-empty -EvidenceRefs file")
	}
	evidence := make([]CompletionEvidence, 0, len(refs))
	for _, ref := range refs {
		pathPart := strings.TrimSpace(strings.SplitN(ref, "#", 2)[0])
		if pathPart == "" || filepath.IsAbs(pathPart) {
			return nil, nil, fmt.Errorf("completion evidence ref must be case-relative: %s", ref)
		}
		path, err := refsf.SafeJoin(caseRoot, pathPart)
		if err != nil {
			return nil, nil, err
		}
		data, err := refsf.ReadStableRegularFileAnchored(caseRoot, path, "completion evidence ref", maxEvidenceBytes)
		if err != nil {
			return nil, nil, fmt.Errorf("completion evidence ref is unreadable: %s: %w", ref, err)
		}
		sum := sha256.Sum256(data)
		evidence = append(evidence, CompletionEvidence{Ref: ref, SHA256: hex.EncodeToString(sum[:]), Bytes: int64(len(data))})
	}
	return refs, evidence, nil
}

func completePreviewSHA256(result CompleteResult) (string, error) {
	result.CompletionPlanSHA256 = ""
	result.ApplyCommand = ""
	data, err := json.Marshal(result)
	if err != nil {
		return "", err
	}
	sum := sha256.Sum256(data)
	return hex.EncodeToString(sum[:]), nil
}

func completeApplyCommand(ctx completeContext, hash string) string {
	args := []string{"/rekit", "complete", workstreamLabel(ctx.lane), "-Actor", ctx.actor, "-Reason", ctx.reason, "-EvidenceRefs", strings.Join(ctx.evidenceRefs, ","), "-ExpectedCompletePlanSha256", hash, "-Apply", "-Format", "json"}
	for i := range args {
		args[i] = quoteCompleteCommandArg(args[i])
	}
	return strings.Join(args, " ")
}

func quoteCompleteCommandArg(value string) string {
	if value != "" && !strings.ContainsAny(value, " \t\r\n\"'") {
		return value
	}
	return `"` + strings.ReplaceAll(value, `"`, `\"`) + `"`
}

func completionBlockerSummary(items []CompletionBlocker) string {
	parts := make([]string, 0, len(items))
	for _, item := range items {
		parts = append(parts, item.Kind+"="+item.Detail)
	}
	return strings.Join(parts, "; ")
}

func (ctx completeContext) intentPath() string {
	return ctx.intentFile
}

func (ctx completeContext) commitPath() string {
	return ctx.commitFile
}

func readCompletionIntent(path string) (completionIntent, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return completionIntent{}, err
	}
	var intent completionIntent
	if err := json.Unmarshal(data, &intent); err != nil {
		return completionIntent{}, fmt.Errorf("invalid lane completion intent %s: %w", path, err)
	}
	return intent, nil
}

func writeCompletionExclusive(caseRoot, path string, value any) error {
	return lanecompletion.WriteExclusiveJSON(caseRoot, path, value)
}

func appendCompletionLaneEvent(caseRoot, path string, event map[string]any) (StartWrite, error) {
	rel := relativePath(caseRoot, path)
	id := mission.Value(event, "eventId")
	existing, err := mission.ReadStrictJSONLineObjects(path)
	if err != nil {
		return StartWrite{}, err
	}
	for _, item := range existing {
		if mission.Value(item, "eventId") != id {
			continue
		}
		for _, key := range []string{"kind", "lane", "actor", "reason", "previewSha256", "currentExecutor", "executorGeneration"} {
			if mission.Value(item, key) != mission.Value(event, key) {
				return StartWrite{}, fmt.Errorf("lane completion event %s differs on %s; refusing recovery", id, key)
			}
		}
		return StartWrite{Path: rel, Kind: "lane-event", Action: "already-appended", TargetPath: path}, nil
	}
	if err := mission.AppendJSONLine(path, event); err != nil {
		return StartWrite{}, err
	}
	return StartWrite{Path: rel, Kind: "lane-event", Action: "append-lane-completed", TargetPath: path}, nil
}

func currentBoardLaneSHA(caseRoot, laneID string) (string, error) {
	b, err := mission.ReadBoard(caseRoot)
	if err != nil {
		return "", err
	}
	for _, lane := range b.Lanes {
		if lane.ID == laneID {
			return hashJSON(lane)
		}
	}
	return "", fmt.Errorf("board omits lane completion projection: %s", laneID)
}

func hashJSON(value any) (string, error) {
	data, err := json.Marshal(value)
	if err != nil {
		return "", err
	}
	sum := sha256.Sum256(data)
	return hex.EncodeToString(sum[:]), nil
}

func hashFile(path string) (string, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return "", err
	}
	sum := sha256.Sum256(data)
	return hex.EncodeToString(sum[:]), nil
}

func completionReceiptsEqual(left, right CompletionReceipt) bool {
	leftData, _ := json.Marshal(left)
	rightData, _ := json.Marshal(right)
	return string(leftData) == string(rightData)
}

func convertCompletionJSON(source, target any) error {
	data, err := json.Marshal(source)
	if err != nil {
		return err
	}
	return json.Unmarshal(data, target)
}

func equalCompletionEvidence(left, right []CompletionEvidence) bool {
	if len(left) != len(right) {
		return false
	}
	for i := range left {
		if left[i] != right[i] {
			return false
		}
	}
	return true
}

func equalStrings(left, right []string) bool {
	if len(left) != len(right) {
		return false
	}
	for i := range left {
		if left[i] != right[i] {
			return false
		}
	}
	return true
}
