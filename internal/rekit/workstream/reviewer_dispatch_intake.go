package workstream

import (
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"runtime"
	"slices"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/shuiyu486/re-context-kits/internal/rekit/casebind"
	refsf "github.com/shuiyu486/re-context-kits/internal/rekit/fs"
	"github.com/shuiyu486/re-context-kits/internal/rekit/mission"
	"github.com/shuiyu486/re-context-kits/internal/rekit/reviewpath"
)

type ReviewerAgentToolRequest struct {
	Tool           string `json:"tool"`
	AgentType      string `json:"agentType"`
	ReadOnly       bool   `json:"readOnly"`
	Prompt         string `json:"prompt"`
	ExpectedOutput string `json:"expectedOutput"`
}

type ReviewerResultCollectionCommands struct {
	CandidatePath  string `json:"candidatePath"`
	PreviewCommand string `json:"previewCommand"`
	ApplyCommand   string `json:"applyCommand"`
}

type ReviewerDispatchIntakeHandoff struct {
	PacketID                         string                            `json:"packetId,omitempty"`
	PacketPath                       string                            `json:"packetPath"`
	SummaryPath                      string                            `json:"summaryPath,omitempty"`
	ResultRoot                       string                            `json:"resultRoot,omitempty"`
	TargetLane                       string                            `json:"targetLane,omitempty"`
	ShardID                          string                            `json:"shardId"`
	DispatchIndex                    int                               `json:"dispatchIndex,omitempty"`
	DispatchTotal                    int                               `json:"dispatchTotal,omitempty"`
	DispatchCompleted                int                               `json:"dispatchCompleted"`
	DispatchOpen                     int                               `json:"dispatchOpen"`
	DispatchWaitingForReviewerResult int                               `json:"dispatchWaitingForReviewerResult"`
	DispatchReadyForPreview          int                               `json:"dispatchReadyForPreview"`
	DispatchAttachRequired           int                               `json:"dispatchAttachRequired"`
	DispatchOnlyOpen                 int                               `json:"dispatchOnlyOpen"`
	LatestCompletedShardID           string                            `json:"latestCompletedShardId,omitempty"`
	NextOpenShardID                  string                            `json:"nextOpenShardId,omitempty"`
	RemainingShardIDs                []string                          `json:"remainingShardIds,omitempty"`
	State                            string                            `json:"state"`
	ReviewerResultPath               string                            `json:"reviewerResultPath,omitempty"`
	ReviewerResultPresent            bool                              `json:"reviewerResultPresent"`
	ReviewerResultState              string                            `json:"reviewerResultState,omitempty"`
	ReviewerResultCandidatePath      string                            `json:"reviewerResultCandidatePath,omitempty"`
	ReviewerResultCandidateState     string                            `json:"reviewerResultCandidateState,omitempty"`
	AgentToolRequest                 *ReviewerAgentToolRequest         `json:"agentToolRequest,omitempty"`
	ReviewerResultCollectionCommands *ReviewerResultCollectionCommands `json:"reviewerResultCollectionCommands,omitempty"`
	IntakeAvailable                  bool                              `json:"intakeAvailable"`
	DispatchOnly                     bool                              `json:"dispatchOnly"`
	VerificationRecorded             bool                              `json:"verificationRecorded"`
	DecisionRecorded                 bool                              `json:"decisionRecorded"`
	DispatchCommand                  string                            `json:"dispatchCommand,omitempty"`
	PreviewCommand                   string                            `json:"previewCommand,omitempty"`
	ApplyCommand                     string                            `json:"applyCommand,omitempty"`
	BatchPreviewCommand              string                            `json:"batchPreviewCommand,omitempty"`
	BatchApplyCommand                string                            `json:"batchApplyCommand,omitempty"`
	OwnerExecutor                    string                            `json:"ownerExecutor,omitempty"`
	OwnerGeneration                  int                               `json:"ownerGeneration,omitempty"`
	OwnerBindingMode                 string                            `json:"ownerBindingMode,omitempty"`
	CurrentExecutor                  string                            `json:"currentExecutor,omitempty"`
	CurrentGeneration                int                               `json:"currentGeneration,omitempty"`
	OwnerAdoptionRequired            bool                              `json:"ownerAdoptionRequired"`
	OwnerAdoptionPath                string                            `json:"ownerAdoptionPath,omitempty"`
	OwnerAdoptionPreviewCommand      string                            `json:"ownerAdoptionPreviewCommand,omitempty"`
	Evidence                         []string                          `json:"evidence,omitempty"`
	Boundary                         []string                          `json:"boundary,omitempty"`
}

type ReviewerDispatchIntakeSummary struct {
	Total                              int      `json:"total"`
	WaitingForReviewerResult           int      `json:"waitingForReviewerResult"`
	ReadyForPreview                    int      `json:"readyForPreview"`
	AttachRequired                     int      `json:"attachRequired"`
	DispatchOnly                       int      `json:"dispatchOnly"`
	LaneCount                          int      `json:"laneCount"`
	Lanes                              []string `json:"lanes,omitempty"`
	PacketCount                        int      `json:"packetCount"`
	LatestPacketDispatchTotal          int      `json:"latestPacketDispatchTotal,omitempty"`
	LatestPacketDispatchCompleted      int      `json:"latestPacketDispatchCompleted"`
	LatestPacketDispatchOpen           int      `json:"latestPacketDispatchOpen"`
	LatestPacketNextOpenShardID        string   `json:"latestPacketNextOpenShardId,omitempty"`
	LatestCompletedShardID             string   `json:"latestCompletedShardId,omitempty"`
	RemainingShardIDs                  []string `json:"remainingShardIds,omitempty"`
	LatestPacketPath                   string   `json:"latestPacketPath,omitempty"`
	LatestShardID                      string   `json:"latestShardId,omitempty"`
	LatestState                        string   `json:"latestState,omitempty"`
	LatestReviewerResultPath           string   `json:"latestReviewerResultPath,omitempty"`
	LatestReviewerResultCandidatePath  string   `json:"latestReviewerResultCandidatePath,omitempty"`
	LatestReviewerResultCandidateState string   `json:"latestReviewerResultCandidateState,omitempty"`
	LatestCollectionPreviewCommand     string   `json:"latestCollectionPreviewCommand,omitempty"`
	LatestCollectionApplyCommand       string   `json:"latestCollectionApplyCommand,omitempty"`
	LatestPreviewCommand               string   `json:"latestPreviewCommand,omitempty"`
	LatestApplyCommand                 string   `json:"latestApplyCommand,omitempty"`
	LatestBatchPreviewCommand          string   `json:"latestBatchPreviewCommand,omitempty"`
	LatestBatchApplyCommand            string   `json:"latestBatchApplyCommand,omitempty"`
	NextAction                         string   `json:"nextAction,omitempty"`
	Boundary                           []string `json:"boundary,omitempty"`
}

type reviewerDispatchPacket struct {
	PacketID              string                              `json:"packetId"`
	Command               string                              `json:"command"`
	RepoRoot              string                              `json:"repoRoot"`
	Pack                  string                              `json:"pack"`
	TargetLane            string                              `json:"targetLane"`
	Observability         reviewerDispatchPacketObservability `json:"observability"`
	ReviewerOrchestration reviewerDispatchPacketOrchestration `json:"reviewerOrchestration"`
}

type reviewerDispatchPacketObservability struct {
	SummaryPath string `json:"summaryPath"`
}

type reviewerDispatchPacketOrchestration struct {
	Mode                string                           `json:"mode"`
	TargetLane          string                           `json:"targetLane"`
	PacketPath          string                           `json:"packetPath"`
	ResultRoot          string                           `json:"resultRoot"`
	OwnerBinding        reviewerDispatchPacketOwner      `json:"ownerBinding"`
	Dispatches          []reviewerDispatchPacketDispatch `json:"dispatches"`
	BatchPreviewCommand string                           `json:"batchPreviewCommand"`
	BatchApplyCommand   string                           `json:"batchApplyCommand"`
}

type reviewerDispatchPacketOwner struct {
	TargetLane             string `json:"targetLane"`
	CurrentExecutor        string `json:"currentExecutor"`
	ExecutorGeneration     int    `json:"executorGeneration"`
	LastTakeoverAt         string `json:"lastTakeoverAt,omitempty"`
	LastTakeoverBy         string `json:"lastTakeoverBy,omitempty"`
	LastTakeoverReason     string `json:"lastTakeoverReason,omitempty"`
	BindingMode            string `json:"bindingMode"`
	RequiredForIntake      bool   `json:"requiredForIntake"`
	MainAgentSpawnOwner    string `json:"mainAgentSpawnOwner"`
	RuntimeSessionBoundary string `json:"runtimeSessionBoundary"`
}

type reviewerDispatchPacketDispatch struct {
	ShardID                     string                            `json:"shardId"`
	Status                      string                            `json:"status"`
	ReviewerResultPath          string                            `json:"reviewerResultPath"`
	ReviewerResultCandidatePath string                            `json:"reviewerResultCandidatePath"`
	AgentToolRequest            *ReviewerAgentToolRequest         `json:"agentToolRequest"`
	CollectionCommands          *ReviewerResultCollectionCommands `json:"collectionCommands"`
	PreviewCommand              string                            `json:"previewCommand"`
	ApplyCommand                string                            `json:"applyCommand"`
}

func ReviewerDispatchIntakeHandoffs(caseRoot string, facts mission.LedgerFacts, laneID string) ([]ReviewerDispatchIntakeHandoff, error) {
	packetPaths, err := reviewerDispatchPacketPaths(caseRoot)
	if err != nil {
		return nil, err
	}
	items := []ReviewerDispatchIntakeHandoff{}
	for _, packetPath := range packetPaths {
		packet, ok := readReviewerDispatchPacket(packetPath)
		if !ok || !strings.EqualFold(strings.TrimSpace(packet.Command), "plan-subagents") || len(packet.ReviewerOrchestration.Dispatches) == 0 {
			continue
		}
		packetTargetLane := firstText(packet.ReviewerOrchestration.TargetLane, packet.TargetLane, packet.ReviewerOrchestration.OwnerBinding.TargetLane)
		if strings.TrimSpace(laneID) != "" && packetTargetLane != laneID {
			continue
		}
		items = append(items, reviewerDispatchIntakeHandoffsForPacket(caseRoot, facts, packet, packetPath, packetTargetLane)...)
	}
	return limitReviewerDispatchIntakeHandoffs(items, maxHandoffRows), nil
}

func limitReviewerDispatchIntakeHandoffs(items []ReviewerDispatchIntakeHandoff, limit int) []ReviewerDispatchIntakeHandoff {
	if limit <= 0 || len(items) <= limit {
		return items
	}
	limited := append([]ReviewerDispatchIntakeHandoff{}, items[len(items)-limit:]...)
	bestPriority := 4
	for _, item := range limited {
		bestPriority = min(bestPriority, reviewerDispatchActionPriority(item))
	}
	for idx := len(items) - limit - 1; idx >= 0; idx-- {
		priority := reviewerDispatchActionPriority(items[idx])
		if priority < bestPriority {
			limited[0] = items[idx]
			bestPriority = priority
		}
	}
	return limited
}

func reviewerDispatchIntakeHandoffsForPacket(caseRoot string, facts mission.LedgerFacts, packet reviewerDispatchPacket, packetPath, targetLane string) []ReviewerDispatchIntakeHandoff {
	all := make([]ReviewerDispatchIntakeHandoff, 0, len(packet.ReviewerOrchestration.Dispatches))
	for idx, dispatch := range packet.ReviewerOrchestration.Dispatches {
		all = append(all, reviewerDispatchIntakeHandoffFor(caseRoot, facts, packet, packetPath, targetLane, dispatch, idx))
	}
	progress := reviewerDispatchPacketProgress(all)
	open := []ReviewerDispatchIntakeHandoff{}
	for _, item := range all {
		if item.VerificationRecorded && item.DecisionRecorded {
			continue
		}
		item.DispatchCompleted = progress.Completed
		item.DispatchOpen = progress.Open
		item.DispatchWaitingForReviewerResult = progress.WaitingForReviewerResult
		item.DispatchReadyForPreview = progress.ReadyForPreview
		item.DispatchAttachRequired = progress.AttachRequired
		item.DispatchOnlyOpen = progress.DispatchOnly
		item.LatestCompletedShardID = progress.LatestCompletedShardID
		item.NextOpenShardID = progress.NextOpenShardID
		item.RemainingShardIDs = append([]string{}, progress.RemainingShardIDs...)
		open = append(open, item)
	}
	return open
}

type reviewerDispatchProgress struct {
	Completed                int
	Open                     int
	WaitingForReviewerResult int
	ReadyForPreview          int
	AttachRequired           int
	DispatchOnly             int
	LatestCompletedShardID   string
	NextOpenShardID          string
	RemainingShardIDs        []string
}

func reviewerDispatchPacketProgress(items []ReviewerDispatchIntakeHandoff) reviewerDispatchProgress {
	progress := reviewerDispatchProgress{}
	for _, item := range items {
		if item.VerificationRecorded && item.DecisionRecorded {
			progress.Completed++
			progress.LatestCompletedShardID = item.ShardID
			continue
		}
		progress.Open++
		if progress.NextOpenShardID == "" {
			progress.NextOpenShardID = item.ShardID
		}
		progress.RemainingShardIDs = append(progress.RemainingShardIDs, item.ShardID)
		if item.DispatchOnly {
			progress.DispatchOnly++
		}
		switch item.State {
		case "waiting-for-reviewer-result", "dispatch-only-waiting-for-result":
			progress.WaitingForReviewerResult++
		case "ready-for-reviewer-result-collection-preview", "ready-for-reviewer-intake-preview":
			progress.ReadyForPreview++
		case "reviewer-packet-owner-adoption-required":
			progress.AttachRequired++
		case "attach-required-before-reviewer-intake":
			progress.AttachRequired++
		}
	}
	return progress
}

func MissionCommanderNextActionsWithReviewerDispatches(base []mission.MissionCommanderNextActionItem, handoffs []ReviewerDispatchIntakeHandoff) []mission.MissionCommanderNextActionItem {
	if len(handoffs) == 0 {
		return mission.UniqueCommanderNextActions(base)
	}
	packetOrder := []string{}
	packetRepresentatives := map[string]ReviewerDispatchIntakeHandoff{}
	for _, handoff := range handoffs {
		packetID := firstText(handoff.PacketID, handoff.PacketPath)
		if packetID == "" {
			continue
		}
		current, seen := packetRepresentatives[packetID]
		if !seen {
			packetOrder = append(packetOrder, packetID)
		}
		if !seen || reviewerDispatchActionPriority(handoff) < reviewerDispatchActionPriority(current) {
			packetRepresentatives[packetID] = handoff
		}
	}
	packetActions := []mission.MissionCommanderNextActionItem{}
	for _, packetID := range packetOrder {
		handoff := packetRepresentatives[packetID]
		state := handoff.State
		blocked := state != "ready-for-reviewer-result-collection-preview" && state != "ready-for-reviewer-intake-preview" && state != "reviewer-packet-owner-adoption-required"
		packetActions = append(packetActions, mission.MissionCommanderNextActionItem{
			Lane:           handoff.TargetLane,
			Label:          packetID,
			ActionID:       packetID,
			State:          state,
			Command:        reviewerDispatchIntakeNextAction(handoff),
			Source:         "reviewerDispatchIntakeHandoffs",
			Blocked:        blocked,
			RequiresReview: true,
			Reasons:        []string{"active reviewer packet must be resolved before ordinary lane continuation", "use packet-level WhatIf before any reviewer intake or adoption Apply"},
			Boundary:       append([]string{}, handoff.Boundary...),
		})
	}
	if len(packetActions) == 0 {
		return mission.UniqueCommanderNextActions(base)
	}
	evidenceNeedsMainReview := slices.ContainsFunc(base, func(item mission.MissionCommanderNextActionItem) bool {
		return item.Source == "executionEvidenceReview" && item.State == "needs-main-escalation"
	})
	if evidenceNeedsMainReview {
		for idx := range packetActions {
			packetActions[idx].Blocked = true
			packetActions[idx].Reasons = append(packetActions[idx].Reasons, "execution evidence main review must complete before reviewer packet work")
		}
	}
	priorityBase := []mission.MissionCommanderNextActionItem{}
	ordinaryBase := []mission.MissionCommanderNextActionItem{}
	for _, item := range base {
		if strings.HasPrefix(item.Command, "/rekit continue ") {
			item.Blocked = true
			item.RequiresReview = true
			item.Reasons = append(item.Reasons, "active reviewer packet must complete before lane continuation")
			item.Boundary = append(item.Boundary, "do not continue while reviewer dispatch/intake work remains open")
		}
		if reviewerDispatchBaseActionHasPriority(item) {
			priorityBase = append(priorityBase, item)
		} else {
			ordinaryBase = append(ordinaryBase, item)
		}
	}
	items := append([]mission.MissionCommanderNextActionItem{}, priorityBase...)
	items = append(items, packetActions...)
	items = append(items, ordinaryBase...)
	return mission.UniqueCommanderNextActions(items)
}

func reviewerDispatchBaseActionHasPriority(item mission.MissionCommanderNextActionItem) bool {
	if item.Source == "executionEvidenceReview" && item.State == "needs-main-escalation" {
		return true
	}
	return item.Source == "missionCommanderActions" && (item.State == "needs-start-apply" || item.State == "needs-reconcile")
}

func reviewerDispatchActionPriority(item ReviewerDispatchIntakeHandoff) int {
	switch item.State {
	case "reviewer-packet-owner-adoption-required":
		return 0
	case "reviewer-result-symlink-blocked", "reviewer-result-candidate-invalid", "reviewer-result-canonical-invalid", "attach-required-before-reviewer-intake":
		return 1
	case "ready-for-reviewer-result-collection-preview", "ready-for-reviewer-intake-preview":
		return 2
	default:
		return 3
	}
}

func ReviewerDispatchIntakeSummaryFor(items []ReviewerDispatchIntakeHandoff) ReviewerDispatchIntakeSummary {
	summary := ReviewerDispatchIntakeSummary{}
	lanes := map[string]bool{}
	packets := map[string]bool{}
	var latestReady *ReviewerDispatchIntakeHandoff
	for idx := range items {
		item := items[idx]
		summary.Total++
		if lane := strings.TrimSpace(item.TargetLane); lane != "" {
			lanes[lane] = true
		}
		packetKey := firstText(item.PacketID, item.PacketPath)
		if packetKey != "" {
			packets[packetKey] = true
		}
		if item.DispatchOnly {
			summary.DispatchOnly++
		}
		switch item.State {
		case "waiting-for-reviewer-result", "dispatch-only-waiting-for-result":
			summary.WaitingForReviewerResult++
		case "ready-for-reviewer-result-collection-preview", "ready-for-reviewer-intake-preview":
			summary.ReadyForPreview++
			latestReady = &items[idx]
		case "attach-required-before-reviewer-intake", "reviewer-packet-owner-adoption-required":
			summary.AttachRequired++
		}
	}
	for lane := range lanes {
		summary.Lanes = append(summary.Lanes, lane)
	}
	sort.Strings(summary.Lanes)
	summary.LaneCount = len(summary.Lanes)
	summary.PacketCount = len(packets)
	if len(items) > 0 {
		latest := items[len(items)-1]
		summary.LatestPacketPath = latest.PacketPath
		summary.LatestShardID = latest.ShardID
		summary.LatestState = latest.State
		summary.LatestReviewerResultPath = latest.ReviewerResultPath
		summary.LatestReviewerResultCandidatePath = latest.ReviewerResultCandidatePath
		summary.LatestReviewerResultCandidateState = latest.ReviewerResultCandidateState
		if latest.ReviewerResultCollectionCommands != nil {
			summary.LatestCollectionPreviewCommand = latest.ReviewerResultCollectionCommands.PreviewCommand
			summary.LatestCollectionApplyCommand = latest.ReviewerResultCollectionCommands.ApplyCommand
		}
		summary.LatestPreviewCommand = latest.PreviewCommand
		summary.LatestApplyCommand = latest.ApplyCommand
		summary.LatestBatchPreviewCommand = latest.BatchPreviewCommand
		summary.LatestBatchApplyCommand = latest.BatchApplyCommand
		summary.LatestPacketDispatchTotal = latest.DispatchTotal
		summary.LatestPacketDispatchCompleted = latest.DispatchCompleted
		summary.LatestPacketDispatchOpen = latest.DispatchOpen
		summary.LatestPacketNextOpenShardID = latest.NextOpenShardID
		summary.LatestCompletedShardID = latest.LatestCompletedShardID
		summary.RemainingShardIDs = append([]string{}, latest.RemainingShardIDs...)
		summary.NextAction = reviewerDispatchIntakeNextAction(latest)
		if latestReady != nil {
			summary.LatestBatchPreviewCommand = latestReady.BatchPreviewCommand
			summary.LatestBatchApplyCommand = latestReady.BatchApplyCommand
			if strings.TrimSpace(summary.LatestBatchPreviewCommand) != "" {
				summary.NextAction = summary.LatestBatchPreviewCommand
			} else {
				summary.NextAction = reviewerDispatchIntakeNextAction(*latestReady)
			}
		}
		summary.Boundary = reviewerDispatchIntakeSummaryBoundary()
	}
	return summary
}

func reviewerDispatchPacketPaths(caseRoot string) ([]string, error) {
	root := filepath.Join(caseRoot, ".rekit", "reviews")
	entries, err := os.ReadDir(root)
	if os.IsNotExist(err) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	paths := []string{}
	for _, entry := range entries {
		if !entry.IsDir() {
			continue
		}
		packetPath := filepath.Join(root, entry.Name(), "packet.json")
		if refsfExists(packetPath) {
			paths = append(paths, packetPath)
		}
	}
	sort.Strings(paths)
	return paths, nil
}

func readReviewerDispatchPacket(path string) (reviewerDispatchPacket, bool) {
	data, err := os.ReadFile(path)
	if err != nil {
		return reviewerDispatchPacket{}, false
	}
	var packet reviewerDispatchPacket
	if err := json.Unmarshal(data, &packet); err != nil {
		return reviewerDispatchPacket{}, false
	}
	return packet, true
}

func reviewerDispatchCollectionCommands(packetPath, shardID, lane, candidatePath string) ReviewerResultCollectionCommands {
	base := "/rekit plan-subagents -PacketPath " + quoteCommandArg(packetPath) +
		" -CollectReviewerResult -ShardId " + quoteCommandArg(shardID) +
		" -Lane " + quoteCommandArg(lane) + " -Actor <main-agent>"
	return ReviewerResultCollectionCommands{
		CandidatePath:  candidatePath,
		PreviewCommand: base + " -WhatIf -Format json",
		ApplyCommand:   base + " -Apply -Format json",
	}
}

func reviewerDispatchIntakeHandoffFor(caseRoot string, facts mission.LedgerFacts, packet reviewerDispatchPacket, packetPath, targetLane string, dispatch reviewerDispatchPacketDispatch, idx int) ReviewerDispatchIntakeHandoff {
	resultPath := strings.TrimSpace(dispatch.ReviewerResultPath)
	candidatePath := strings.TrimSpace(dispatch.ReviewerResultCandidatePath)
	collectionAvailable := packet.ReviewerOrchestration.PacketPath != "" &&
		reviewpath.CanonicalCollectionShard(caseRoot, packetPath, packet.ReviewerOrchestration.ResultRoot, dispatch.ShardID, candidatePath, resultPath) &&
		reviewpath.CollectionNamespacePathSafe(caseRoot, packetPath, false) &&
		reviewpath.CollectionNamespacePathSafe(caseRoot, packet.ReviewerOrchestration.ResultRoot, false) &&
		reviewpath.CollectionNamespacePathSafe(caseRoot, filepath.Dir(candidatePath), true) &&
		reviewpath.CollectionNamespacePathSafe(caseRoot, filepath.Dir(resultPath), false) &&
		casebind.SamePath(packet.ReviewerOrchestration.PacketPath, packetPath) &&
		dispatch.CollectionCommands != nil &&
		casebind.SamePath(dispatch.CollectionCommands.CandidatePath, candidatePath) &&
		reviewerDispatchIntakeCommandAvailable(dispatch.CollectionCommands.PreviewCommand) &&
		reviewerDispatchIntakeCommandAvailable(dispatch.CollectionCommands.ApplyCommand)
	var collectionCommands *ReviewerResultCollectionCommands
	if collectionAvailable {
		commands := reviewerDispatchCollectionCommands(packetPath, dispatch.ShardID, targetLane, candidatePath)
		collectionCommands = &commands
	} else {
		candidatePath = ""
	}
	resultState := refsf.RegularFileMissing
	if resultPath != "" {
		classified, err := refsf.ClassifyNonEmptyRegularFile(resultPath)
		if err != nil {
			resultState = refsf.RegularFileWaiting
		} else {
			resultState = classified
		}
	}
	present := resultState == refsf.RegularFileReady
	candidateState := ""
	if candidatePath != "" {
		candidateState = "missing"
		if present {
			candidateState = "collected"
		} else if classified, err := refsf.ClassifyNonEmptyRegularFile(candidatePath); err != nil || classified == refsf.RegularFileSymlink || classified == refsf.RegularFileWaiting {
			candidateState = "invalid"
		} else if classified == refsf.RegularFileReady {
			candidateState = "ready"
		}
	}
	intakeAvailable := reviewerDispatchIntakeCommandAvailable(dispatch.PreviewCommand) && reviewerDispatchIntakeCommandAvailable(dispatch.ApplyCommand)
	verificationRecorded := reviewerDispatchWritebackRecorded(facts.Verifications, packet.PacketID, dispatch.ShardID, resultPath)
	decisionRecorded := reviewerDispatchWritebackRecorded(facts.Decisions, packet.PacketID, dispatch.ShardID, resultPath)
	state := reviewerDispatchIntakeState(resultState, intakeAvailable)
	if !present && resultState == refsf.RegularFileWaiting && candidatePath != "" {
		state = "reviewer-result-canonical-invalid"
	} else if !present && candidateState == "invalid" {
		state = "reviewer-result-candidate-invalid"
	} else if !present && candidateState == "ready" && collectionCommands != nil && reviewerDispatchIntakeCommandAvailable(collectionCommands.PreviewCommand) {
		state = "ready-for-reviewer-result-collection-preview"
	}
	currentExecutor, currentGeneration := reviewerDispatchCurrentOwner(caseRoot, targetLane)
	adoptionPath := filepath.Join(caseRoot, ".rekit", "reviewer-adoptions", packet.PacketID+".json")
	adoptionCurrent := reviewerDispatchAdoptionCurrent(caseRoot, adoptionPath, packet, packetPath, currentExecutor, currentGeneration)
	ownerStale := currentExecutor != strings.TrimSpace(packet.ReviewerOrchestration.OwnerBinding.CurrentExecutor) ||
		currentGeneration != packet.ReviewerOrchestration.OwnerBinding.ExecutorGeneration
	if ownerStale && !adoptionCurrent {
		state = "reviewer-packet-owner-adoption-required"
	}
	item := ReviewerDispatchIntakeHandoff{
		PacketID:                         packet.PacketID,
		PacketPath:                       packetPath,
		SummaryPath:                      packet.Observability.SummaryPath,
		ResultRoot:                       packet.ReviewerOrchestration.ResultRoot,
		TargetLane:                       targetLane,
		ShardID:                          dispatch.ShardID,
		DispatchIndex:                    idx + 1,
		DispatchTotal:                    len(packet.ReviewerOrchestration.Dispatches),
		State:                            state,
		ReviewerResultPath:               resultPath,
		ReviewerResultPresent:            present,
		ReviewerResultState:              string(resultState),
		ReviewerResultCandidatePath:      candidatePath,
		ReviewerResultCandidateState:     candidateState,
		AgentToolRequest:                 dispatch.AgentToolRequest,
		ReviewerResultCollectionCommands: collectionCommands,
		IntakeAvailable:                  intakeAvailable,
		DispatchOnly:                     !intakeAvailable,
		VerificationRecorded:             verificationRecorded,
		DecisionRecorded:                 decisionRecorded,
		DispatchCommand:                  reviewerDispatchCommand(dispatch.ShardID, candidatePath, resultPath, dispatch.AgentToolRequest, idx),
		PreviewCommand:                   dispatch.PreviewCommand,
		ApplyCommand:                     dispatch.ApplyCommand,
		BatchPreviewCommand:              packet.ReviewerOrchestration.BatchPreviewCommand,
		BatchApplyCommand:                packet.ReviewerOrchestration.BatchApplyCommand,
		OwnerExecutor:                    packet.ReviewerOrchestration.OwnerBinding.CurrentExecutor,
		OwnerGeneration:                  packet.ReviewerOrchestration.OwnerBinding.ExecutorGeneration,
		OwnerBindingMode:                 packet.ReviewerOrchestration.OwnerBinding.BindingMode,
		CurrentExecutor:                  currentExecutor,
		CurrentGeneration:                currentGeneration,
		OwnerAdoptionRequired:            ownerStale && !adoptionCurrent,
		OwnerAdoptionPath:                adoptionPath,
		OwnerAdoptionPreviewCommand:      reviewerDispatchAdoptionPreviewCommand(packetPath, targetLane),
	}
	if item.OwnerAdoptionRequired {
		item.BatchPreviewCommand = ""
		item.BatchApplyCommand = ""
		item.PreviewCommand = ""
		item.ApplyCommand = ""
	}
	item.Evidence = reviewerDispatchIntakeEvidence(caseRoot, item)
	item.Boundary = reviewerDispatchIntakeBoundary(item)
	return item
}

func reviewerDispatchCurrentOwner(caseRoot, laneID string) (string, int) {
	board, err := mission.ReadBoard(caseRoot)
	if err != nil {
		return "", 0
	}
	lane, ok := mission.LookupBoardLane(board.Lanes, laneID, false)
	if !ok {
		return "", 0
	}
	return strings.TrimSpace(lane.CurrentExecutor), lane.ExecutorGeneration
}

func reviewerDispatchAdoptionCurrent(caseRoot, path string, packet reviewerDispatchPacket, packetPath, currentExecutor string, currentGeneration int) bool {
	data, err := os.ReadFile(path)
	if err != nil {
		return false
	}
	var adoption struct {
		SchemaVersion          int                         `json:"schemaVersion"`
		Kind                   string                      `json:"kind"`
		PacketID               string                      `json:"packetId"`
		PacketPath             string                      `json:"packetPath"`
		PacketSHA256           string                      `json:"packetSha256"`
		RepoRoot               string                      `json:"repoRoot"`
		CaseRoot               string                      `json:"caseRoot"`
		Pack                   string                      `json:"pack"`
		Lane                   string                      `json:"lane"`
		DispatchedOwner        reviewerDispatchPacketOwner `json:"dispatchedOwner"`
		AdoptedOwner           reviewerDispatchPacketOwner `json:"adoptedOwner"`
		Actor                  string                      `json:"actor"`
		Reason                 string                      `json:"reason"`
		CreatedAt              string                      `json:"createdAt"`
		NoSpawn                bool                        `json:"noSpawn"`
		NoHeavyTool            bool                        `json:"noHeavyTool"`
		NoAuthorityOrConfirmed bool                        `json:"noAuthorityOrConfirmed"`
	}
	decoder := json.NewDecoder(bytes.NewReader(bytes.TrimSpace(data)))
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(&adoption); err != nil {
		return false
	}
	var trailing any
	if err := decoder.Decode(&trailing); err != io.EOF {
		return false
	}
	if _, err := time.Parse(time.RFC3339Nano, adoption.CreatedAt); err != nil {
		return false
	}
	packetBytes, err := os.ReadFile(packetPath)
	if err != nil {
		return false
	}
	sum := sha256.Sum256(packetBytes)
	owner := packet.ReviewerOrchestration.OwnerBinding
	return adoption.SchemaVersion == 1 &&
		adoption.Kind == "reviewer-packet-owner-adoption" &&
		adoption.PacketID == packet.PacketID &&
		reviewerDispatchSamePath(adoption.PacketPath, packetPath) &&
		adoption.PacketSHA256 == hex.EncodeToString(sum[:]) &&
		reviewerDispatchSamePath(adoption.RepoRoot, packet.RepoRoot) &&
		reviewerDispatchSamePath(adoption.CaseRoot, caseRoot) &&
		strings.EqualFold(strings.TrimSpace(adoption.Pack), strings.TrimSpace(packet.Pack)) &&
		adoption.Lane == packet.TargetLane &&
		adoption.DispatchedOwner == owner &&
		adoption.AdoptedOwner.TargetLane == owner.TargetLane &&
		strings.TrimSpace(adoption.AdoptedOwner.CurrentExecutor) == currentExecutor &&
		adoption.AdoptedOwner.ExecutorGeneration == currentGeneration &&
		adoption.AdoptedOwner.BindingMode == "durable-lane-executor-adoption" &&
		adoption.AdoptedOwner.RequiredForIntake &&
		adoption.AdoptedOwner.MainAgentSpawnOwner == owner.MainAgentSpawnOwner &&
		adoption.AdoptedOwner.RuntimeSessionBoundary == owner.RuntimeSessionBoundary &&
		strings.TrimSpace(adoption.Actor) != "" &&
		strings.TrimSpace(adoption.Reason) != "" &&
		strings.TrimSpace(adoption.CreatedAt) != "" &&
		adoption.NoSpawn && adoption.NoHeavyTool && adoption.NoAuthorityOrConfirmed
}

func reviewerDispatchSamePath(left, right string) bool {
	leftPath, leftErr := filepath.Abs(strings.TrimSpace(left))
	rightPath, rightErr := filepath.Abs(strings.TrimSpace(right))
	if leftErr != nil || rightErr != nil {
		return false
	}
	leftClean := filepath.Clean(leftPath)
	rightClean := filepath.Clean(rightPath)
	if runtime.GOOS == "windows" {
		return strings.EqualFold(leftClean, rightClean)
	}
	return leftClean == rightClean
}

func reviewerDispatchAdoptionPreviewCommand(packetPath, lane string) string {
	return "/rekit plan-subagents -PacketPath " + quoteCommandArg(packetPath) + " -AdoptReviewerPacket -Lane " + quoteCommandArg(lane) + " -Actor <actor> -Reason <reason> -WhatIf -Format json"
}

func reviewerDispatchIntakeCommandAvailable(command string) bool {
	command = strings.TrimSpace(command)
	return command != "" && !strings.HasPrefix(command, "n/a:")
}

func reviewerDispatchIntakeState(resultState refsf.RegularFileState, intakeAvailable bool) string {
	switch {
	case resultState == refsf.RegularFileSymlink:
		return "reviewer-result-symlink-blocked"
	case !intakeAvailable && resultState == refsf.RegularFileReady:
		return "attach-required-before-reviewer-intake"
	case !intakeAvailable:
		return "dispatch-only-waiting-for-result"
	case resultState == refsf.RegularFileReady:
		return "ready-for-reviewer-intake-preview"
	default:
		return "waiting-for-reviewer-result"
	}
}

func reviewerDispatchWritebackRecorded(events []map[string]any, packetID, shardID, reviewerResultPath string) bool {
	for _, event := range events {
		eventPacketID := firstObjectText(event, "packetId")
		eventShardID := firstObjectText(event, "shardId")
		if strings.TrimSpace(packetID) != "" && eventPacketID != packetID {
			continue
		}
		if strings.TrimSpace(shardID) != "" && eventShardID != shardID {
			continue
		}
		if eventPacketID != "" && eventShardID != "" {
			return true
		}
		if strings.TrimSpace(reviewerResultPath) != "" && firstObjectText(event, "reviewerResultPath") == reviewerResultPath {
			return true
		}
	}
	return false
}

func reviewerDispatchIntakeEvidence(caseRoot string, item ReviewerDispatchIntakeHandoff) []string {
	evidence := []string{}
	if strings.TrimSpace(item.PacketPath) != "" {
		evidence = append(evidence, "packet "+reviewerDispatchDisplayPath(caseRoot, item.PacketPath))
	}
	if strings.TrimSpace(item.SummaryPath) != "" {
		evidence = append(evidence, "summary "+reviewerDispatchDisplayPath(caseRoot, item.SummaryPath))
	}
	if strings.TrimSpace(item.ResultRoot) != "" {
		evidence = append(evidence, "resultRoot "+reviewerDispatchDisplayPath(caseRoot, item.ResultRoot))
	}
	if strings.TrimSpace(item.ReviewerResultCandidatePath) != "" {
		evidence = append(evidence, "reviewerResultCandidate "+firstText(item.ReviewerResultCandidateState, "missing")+" "+reviewerDispatchDisplayPath(caseRoot, item.ReviewerResultCandidatePath))
	}
	if strings.TrimSpace(item.ReviewerResultPath) != "" {
		state := "missing"
		if item.ReviewerResultPresent {
			state = "present"
		}
		evidence = append(evidence, "reviewerResult "+state+" "+reviewerDispatchDisplayPath(caseRoot, item.ReviewerResultPath))
	}
	if item.VerificationRecorded {
		evidence = append(evidence, "verification writeback already recorded")
	}
	if item.DecisionRecorded {
		evidence = append(evidence, "decision writeback already recorded")
	}
	return mission.UniqueStrings(evidence)
}

func reviewerDispatchIntakeBoundary(item ReviewerDispatchIntakeHandoff) []string {
	boundary := []string{
		"reviewer dispatch intake handoff is read-only; full packet.json and reviewerOrchestration remain source of truth",
		"runtime does not spawn, stop, monitor, or manage reviewer sessions",
		"reviewer intake must run -WhatIf before -Apply and must not write authority/confirmed state or execute heavy tools",
	}
	if item.ReviewerResultCollectionCommands != nil {
		boundary = append(boundary,
			"reviewer returns one JSON object; the main agent saves it to the packet-derived candidate path and runs collection -WhatIf before -Apply",
			"collection publishes exact candidate bytes only to the immutable packet-derived reviewer result path and never overwrites different bytes",
		)
	} else if item.IntakeAvailable {
		boundary = append(boundary, "this packet has no canonical collection capability; save reviewer JSON directly to reviewerResultPath and use strict direct or batch intake")
	}
	if item.DispatchOnly {
		boundary = append(boundary, "dispatch-only packets require an attached rekit case before reviewer-intake writeback")
	}
	if item.ReviewerResultState == string(refsf.RegularFileSymlink) {
		boundary = append(boundary, "reviewer result symlinks are rejected by strict batch intake; replace the path with a regular non-empty file")
	}
	if item.OwnerAdoptionRequired {
		boundary = append(boundary, "review packet owner binding is stale; adopt the immutable packet before intake or lane continuation")
	}
	return mission.UniqueStrings(boundary)
}

func reviewerDispatchIntakeSummaryBoundary() []string {
	return []string{
		"reviewer dispatch intake summary is read-only; full packet.json, summary.md, and reviewerOrchestration remain available",
		"runtime does not spawn, stop, monitor, or manage reviewer sessions from status/handoff/continue",
		"run ready-result batch intake -WhatIf before -Apply; packet order, fail-fast, waiting results, no-authority, and no-heavy boundaries remain enforced",
	}
}

func reviewerDispatchIntakeNextAction(item ReviewerDispatchIntakeHandoff) string {
	switch item.State {
	case "reviewer-packet-owner-adoption-required":
		return firstText(item.OwnerAdoptionPreviewCommand, "adopt reviewer packet "+item.PacketID+" before intake")
	case "ready-for-reviewer-result-collection-preview":
		if item.ReviewerResultCollectionCommands != nil {
			return firstText(item.ReviewerResultCollectionCommands.PreviewCommand, "run reviewer result collection -WhatIf for "+item.ShardID)
		}
		return "run reviewer result collection -WhatIf for " + item.ShardID
	case "ready-for-reviewer-intake-preview":
		return firstText(item.BatchPreviewCommand, item.PreviewCommand, "run reviewer-intake -WhatIf for "+item.ShardID)
	case "attach-required-before-reviewer-intake":
		return "attach or init the target as a rekit case before reviewer-intake writeback for " + item.ShardID
	case "reviewer-result-symlink-blocked":
		return "replace the symlink reviewer result with a regular non-empty file for " + item.ShardID
	case "reviewer-result-candidate-invalid":
		return "replace the invalid reviewer result candidate for " + item.ShardID + " at " + item.ReviewerResultCandidatePath + ", then rerun collection -WhatIf"
	case "reviewer-result-canonical-invalid":
		return "remove or repair the non-regular canonical reviewer result for " + item.ShardID + " at " + item.ReviewerResultPath + ", then rerun collection -WhatIf"
	default:
		if item.AgentToolRequest != nil && strings.TrimSpace(item.ReviewerResultCandidatePath) != "" {
			return item.DispatchCommand
		}
		return "collect read-only reviewer JSON for " + item.ShardID + " at " + item.ReviewerResultPath
	}
}

func reviewerDispatchCommand(shardID, candidatePath, reviewerResultPath string, request *ReviewerAgentToolRequest, idx int) string {
	if request != nil && strings.TrimSpace(candidatePath) != "" {
		return "dispatch read-only reviewer for " + shardID + " using reviewerOrchestration.dispatches[" + strconv.Itoa(idx) + "].agentToolRequest.prompt; save exactly one JSON object at " + reviewerDispatchQuoteCommandArg(candidatePath) + ", then run reviewer result collection WhatIf before Apply"
	}
	return "dispatch read-only reviewer for " + shardID + " using reviewerOrchestration.dispatches[" + strconv.Itoa(idx) + "].dispatchPrompt; collect JSON at " + reviewerDispatchQuoteCommandArg(reviewerResultPath)
}

func reviewerDispatchQuoteCommandArg(value string) string {
	value = strings.TrimSpace(value)
	if value == "" {
		return "\"\""
	}
	if !strings.ContainsAny(value, " \t\r\n\"") {
		return value
	}
	return strconv.Quote(value)
}

func reviewerDispatchDisplayPath(caseRoot, path string) string {
	if strings.TrimSpace(path) == "" {
		return ""
	}
	if rel := relativePath(caseRoot, path); rel != "" && rel != path {
		return rel
	}
	return path
}

func refsfExists(path string) bool {
	_, err := os.Stat(path)
	return err == nil
}

func appendReviewerDispatchIntakeHandoff(lines []string, items []ReviewerDispatchIntakeHandoff) []string {
	lines = append(lines, "", "## Reviewer dispatch intake handoff", "")
	if len(items) == 0 {
		return append(lines, "- none")
	}
	summary := ReviewerDispatchIntakeSummaryFor(items)
	lines = append(lines, fmt.Sprintf("- summary: total=%d waitingForReviewerResult=%d readyForPreview=%d attachRequired=%d dispatchOnly=%d packets=%d latestPacketProgress=%d/%d open=%d nextOpen=%s remaining=%s latestShard=%s latestState=%s candidateState=%s nextAction=`%s`", summary.Total, summary.WaitingForReviewerResult, summary.ReadyForPreview, summary.AttachRequired, summary.DispatchOnly, summary.PacketCount, summary.LatestPacketDispatchCompleted, summary.LatestPacketDispatchTotal, summary.LatestPacketDispatchOpen, summary.LatestPacketNextOpenShardID, strings.Join(summary.RemainingShardIDs, ","), summary.LatestShardID, summary.LatestState, summary.LatestReviewerResultCandidateState, summary.NextAction))
	for _, item := range items {
		lines = append(lines, fmt.Sprintf("- dispatch intake: lane=%s shard=%s state=%s progress=%d/%d open=%d nextOpen=%s remaining=%s candidateState=%s candidate=`%s` resultPresent=%t resultState=%s packet=`%s` reviewerResult=`%s` preview=`%s` apply=`%s` batchPreview=`%s` batchApply=`%s`", item.TargetLane, item.ShardID, item.State, item.DispatchCompleted, item.DispatchTotal, item.DispatchOpen, item.NextOpenShardID, strings.Join(item.RemainingShardIDs, ","), item.ReviewerResultCandidateState, item.ReviewerResultCandidatePath, item.ReviewerResultPresent, item.ReviewerResultState, item.PacketPath, item.ReviewerResultPath, item.PreviewCommand, item.ApplyCommand, item.BatchPreviewCommand, item.BatchApplyCommand))
		if item.AgentToolRequest != nil {
			lines = append(lines, fmt.Sprintf("  - agent tool: tool=%s agentType=%s readOnly=%t expectedOutput=%s", item.AgentToolRequest.Tool, item.AgentToolRequest.AgentType, item.AgentToolRequest.ReadOnly, item.AgentToolRequest.ExpectedOutput))
		}
		if item.ReviewerResultCollectionCommands != nil {
			lines = append(lines, fmt.Sprintf("  - collection: preview=`%s` apply=`%s`", item.ReviewerResultCollectionCommands.PreviewCommand, item.ReviewerResultCollectionCommands.ApplyCommand))
		}
		for _, evidence := range mission.LimitStrings(item.Evidence, maxHandoffRows) {
			lines = append(lines, "  - evidence: "+evidence)
		}
		for _, boundary := range mission.LimitStrings(item.Boundary, maxHandoffRows) {
			lines = append(lines, "  - boundary: "+boundary)
		}
	}
	return lines
}

func WriteReviewerDispatchIntakeHandoffSection(out *bytes.Buffer, title string, items []ReviewerDispatchIntakeHandoff) {
	if len(items) == 0 {
		return
	}
	summary := ReviewerDispatchIntakeSummaryFor(items)
	fmt.Fprintln(out, title)
	fmt.Fprintln(out)
	fmt.Fprintf(out, "- summary: total=%d waitingForReviewerResult=%d readyForPreview=%d attachRequired=%d dispatchOnly=%d packets=%d latestPacketProgress=%d/%d open=%d nextOpen=%s remaining=%s latestShard=%s latestState=%s candidateState=%s nextAction=`%s`\n", summary.Total, summary.WaitingForReviewerResult, summary.ReadyForPreview, summary.AttachRequired, summary.DispatchOnly, summary.PacketCount, summary.LatestPacketDispatchCompleted, summary.LatestPacketDispatchTotal, summary.LatestPacketDispatchOpen, summary.LatestPacketNextOpenShardID, strings.Join(summary.RemainingShardIDs, ","), summary.LatestShardID, summary.LatestState, summary.LatestReviewerResultCandidateState, summary.NextAction)
	for _, item := range items {
		fmt.Fprintf(out, "- dispatch intake: lane=%s shard=%s state=%s progress=%d/%d open=%d nextOpen=%s remaining=%s candidateState=%s candidate=`%s` resultPresent=%t packet=`%s` reviewerResult=`%s` preview=`%s` apply=`%s`\n", item.TargetLane, item.ShardID, item.State, item.DispatchCompleted, item.DispatchTotal, item.DispatchOpen, item.NextOpenShardID, strings.Join(item.RemainingShardIDs, ","), item.ReviewerResultCandidateState, item.ReviewerResultCandidatePath, item.ReviewerResultPresent, item.PacketPath, item.ReviewerResultPath, item.PreviewCommand, item.ApplyCommand)
		if item.AgentToolRequest != nil {
			fmt.Fprintf(out, "  - agent tool: tool=%s agentType=%s readOnly=%t expectedOutput=%s\n", item.AgentToolRequest.Tool, item.AgentToolRequest.AgentType, item.AgentToolRequest.ReadOnly, item.AgentToolRequest.ExpectedOutput)
		}
		if item.ReviewerResultCollectionCommands != nil {
			fmt.Fprintf(out, "  - collection: preview=`%s` apply=`%s`\n", item.ReviewerResultCollectionCommands.PreviewCommand, item.ReviewerResultCollectionCommands.ApplyCommand)
		}
		for _, evidence := range item.Evidence {
			fmt.Fprintf(out, "  - evidence: %s\n", evidence)
		}
		for _, boundary := range item.Boundary {
			fmt.Fprintf(out, "  - boundary: %s\n", boundary)
		}
	}
	fmt.Fprintln(out)
}
