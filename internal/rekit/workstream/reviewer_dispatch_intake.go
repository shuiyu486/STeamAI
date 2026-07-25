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
	"github.com/shuiyu486/re-context-kits/internal/rekit/instance"
	"github.com/shuiyu486/re-context-kits/internal/rekit/mission"
	"github.com/shuiyu486/re-context-kits/internal/rekit/reviewpath"
)

type ReviewerAgentToolRequest struct {
	Tool           string `json:"tool"`
	AgentType      string `json:"agentType"`
	ReadOnly       bool   `json:"readOnly"`
	Prompt         string `json:"prompt"`
	PromptPath     string `json:"promptPath,omitempty"`
	PromptSHA256   string `json:"promptSha256,omitempty"`
	ExpectedOutput string `json:"expectedOutput"`
}

type ReviewerResultStagingCommands struct {
	SourcePath         string `json:"sourcePath,omitempty"`
	SourcePathArgument string `json:"sourcePathArgument"`
	PreviewCommand     string `json:"previewCommand"`
}

type ReviewerResultCollectionCommands struct {
	CandidatePath  string `json:"candidatePath"`
	PreviewCommand string `json:"previewCommand"`
	ApplyCommand   string `json:"applyCommand"`
}

type reviewerResultRecoveryDispositionRecord struct {
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

type reviewerResultRecoveryRecord struct {
	SchemaVersion            int    `json:"schemaVersion"`
	Kind                     string `json:"kind"`
	RepoRoot                 string `json:"repoRoot"`
	CaseRoot                 string `json:"caseRoot"`
	Pack                     string `json:"pack"`
	PacketID                 string `json:"packetId"`
	PacketPath               string `json:"packetPath"`
	ShardID                  string `json:"shardId"`
	Lane                     string `json:"lane"`
	CandidatePath            string `json:"candidatePath"`
	CandidateSHA256          string `json:"candidateSha256"`
	CandidateBytes           int    `json:"candidateBytes"`
	ReviewerResultPath       string `json:"reviewerResultPath"`
	ReviewerResultKind       string `json:"reviewerResultKind"`
	ReviewerResultSHA256     string `json:"reviewerResultSha256"`
	ReviewerResultBytes      int    `json:"reviewerResultBytes"`
	ReviewerResultMode       uint32 `json:"reviewerResultMode"`
	ReviewerResultLinkTarget string `json:"reviewerResultLinkTarget,omitempty"`
	QuarantinePath           string `json:"quarantinePath"`
	Actor                    string `json:"actor"`
	Reason                   string `json:"reason"`
	CreatedAt                string `json:"createdAt"`
	NoVerdict                bool   `json:"noReviewerVerdict"`
	NoFacts                  bool   `json:"noFactsWrite"`
	NoHeavyTool              bool   `json:"noHeavyTool"`
	NoAuthority              bool   `json:"noAuthorityOrConfirmed"`
}

type ReviewerDispatchIntakeHandoff struct {
	PacketID                                 string                            `json:"packetId,omitempty"`
	PacketPath                               string                            `json:"packetPath"`
	SummaryPath                              string                            `json:"summaryPath,omitempty"`
	ResultRoot                               string                            `json:"resultRoot,omitempty"`
	TargetLane                               string                            `json:"targetLane,omitempty"`
	ShardID                                  string                            `json:"shardId"`
	DispatchIndex                            int                               `json:"dispatchIndex,omitempty"`
	DispatchTotal                            int                               `json:"dispatchTotal,omitempty"`
	DispatchCompleted                        int                               `json:"dispatchCompleted"`
	DispatchOpen                             int                               `json:"dispatchOpen"`
	DispatchWaitingForReviewerResult         int                               `json:"dispatchWaitingForReviewerResult"`
	DispatchReadyForPreview                  int                               `json:"dispatchReadyForPreview"`
	DispatchAttachRequired                   int                               `json:"dispatchAttachRequired"`
	DispatchOnlyOpen                         int                               `json:"dispatchOnlyOpen"`
	LatestCompletedShardID                   string                            `json:"latestCompletedShardId,omitempty"`
	NextOpenShardID                          string                            `json:"nextOpenShardId,omitempty"`
	RemainingShardIDs                        []string                          `json:"remainingShardIds,omitempty"`
	State                                    string                            `json:"state"`
	ReviewerResultPath                       string                            `json:"reviewerResultPath,omitempty"`
	ReviewerResultPresent                    bool                              `json:"reviewerResultPresent"`
	ReviewerResultState                      string                            `json:"reviewerResultState,omitempty"`
	ReviewerResultSourcePath                 string                            `json:"reviewerResultSourcePath,omitempty"`
	ReviewerResultSourceState                string                            `json:"reviewerResultSourceState,omitempty"`
	ReviewerResultCandidatePath              string                            `json:"reviewerResultCandidatePath,omitempty"`
	ReviewerResultCandidateState             string                            `json:"reviewerResultCandidateState,omitempty"`
	DispatchPromptPath                       string                            `json:"dispatchPromptPath,omitempty"`
	DispatchPromptSHA256                     string                            `json:"dispatchPromptSha256,omitempty"`
	DispatchPromptState                      string                            `json:"dispatchPromptState,omitempty"`
	DispatchPromptCurrent                    bool                              `json:"dispatchPromptCurrent,omitempty"`
	DispatchPromptActualSHA256               string                            `json:"dispatchPromptActualSha256,omitempty"`
	DispatchPromptFailure                    string                            `json:"dispatchPromptFailure,omitempty"`
	DispatchPromptRepairCommand              string                            `json:"dispatchPromptRepairCommand,omitempty"`
	AgentToolRequest                         *ReviewerAgentToolRequest         `json:"agentToolRequest,omitempty"`
	ReviewerResultStagingCommand             string                            `json:"reviewerResultStagingCommand,omitempty"`
	ReviewerResultCollectionCommands         *ReviewerResultCollectionCommands `json:"reviewerResultCollectionCommands,omitempty"`
	ReviewerResultRecoveryCommand            string                            `json:"reviewerResultRecoveryCommand,omitempty"`
	ReviewerResultRecoveryApplyCommand       string                            `json:"reviewerResultRecoveryApplyCommand,omitempty"`
	ReviewerResultRecoveryDispositionCommand string                            `json:"reviewerResultRecoveryDispositionCommand,omitempty"`
	ReviewerResultRecoveryDispositionPath    string                            `json:"reviewerResultRecoveryDispositionPath,omitempty"`
	IntakeAvailable                          bool                              `json:"intakeAvailable"`
	DispatchOnly                             bool                              `json:"dispatchOnly"`
	VerificationRecorded                     bool                              `json:"verificationRecorded"`
	DecisionRecorded                         bool                              `json:"decisionRecorded"`
	DispatchCommand                          string                            `json:"dispatchCommand,omitempty"`
	PreviewCommand                           string                            `json:"previewCommand,omitempty"`
	ApplyCommand                             string                            `json:"applyCommand,omitempty"`
	BatchPreviewCommand                      string                            `json:"batchPreviewCommand,omitempty"`
	BatchApplyCommand                        string                            `json:"batchApplyCommand,omitempty"`
	OwnerExecutor                            string                            `json:"ownerExecutor,omitempty"`
	OwnerGeneration                          int                               `json:"ownerGeneration,omitempty"`
	OwnerBindingMode                         string                            `json:"ownerBindingMode,omitempty"`
	CurrentExecutor                          string                            `json:"currentExecutor,omitempty"`
	CurrentGeneration                        int                               `json:"currentGeneration,omitempty"`
	OwnerAdoptionRequired                    bool                              `json:"ownerAdoptionRequired"`
	OwnerAdoptionPath                        string                            `json:"ownerAdoptionPath,omitempty"`
	OwnerAdoptionPreviewCommand              string                            `json:"ownerAdoptionPreviewCommand,omitempty"`
	RunbookSteps                             []string                          `json:"runbookSteps,omitempty"`
	Evidence                                 []string                          `json:"evidence,omitempty"`
	Boundary                                 []string                          `json:"boundary,omitempty"`
}

type ReviewerDispatchIntakeSummary struct {
	Total                                  int      `json:"total"`
	WaitingForReviewerResult               int      `json:"waitingForReviewerResult"`
	ReadyForPreview                        int      `json:"readyForPreview"`
	AttachRequired                         int      `json:"attachRequired"`
	DispatchOnly                           int      `json:"dispatchOnly"`
	PromptArtifactBlocked                  int      `json:"promptArtifactBlocked"`
	LaneCount                              int      `json:"laneCount"`
	Lanes                                  []string `json:"lanes,omitempty"`
	PacketCount                            int      `json:"packetCount"`
	LatestPacketDispatchTotal              int      `json:"latestPacketDispatchTotal,omitempty"`
	LatestPacketDispatchCompleted          int      `json:"latestPacketDispatchCompleted"`
	LatestPacketDispatchOpen               int      `json:"latestPacketDispatchOpen"`
	LatestPacketNextOpenShardID            string   `json:"latestPacketNextOpenShardId,omitempty"`
	LatestCompletedShardID                 string   `json:"latestCompletedShardId,omitempty"`
	RemainingShardIDs                      []string `json:"remainingShardIds,omitempty"`
	LatestPacketPath                       string   `json:"latestPacketPath,omitempty"`
	LatestShardID                          string   `json:"latestShardId,omitempty"`
	LatestState                            string   `json:"latestState,omitempty"`
	LatestReviewerResultPath               string   `json:"latestReviewerResultPath,omitempty"`
	LatestDispatchPromptPath               string   `json:"latestDispatchPromptPath,omitempty"`
	LatestDispatchPromptSHA256             string   `json:"latestDispatchPromptSha256,omitempty"`
	LatestDispatchPromptState              string   `json:"latestDispatchPromptState,omitempty"`
	LatestDispatchPromptCurrent            bool     `json:"latestDispatchPromptCurrent,omitempty"`
	LatestDispatchPromptActualSHA256       string   `json:"latestDispatchPromptActualSha256,omitempty"`
	LatestDispatchPromptFailure            string   `json:"latestDispatchPromptFailure,omitempty"`
	LatestReviewerResultSourcePath         string   `json:"latestReviewerResultSourcePath,omitempty"`
	LatestReviewerResultSourceState        string   `json:"latestReviewerResultSourceState,omitempty"`
	LatestReviewerResultCandidatePath      string   `json:"latestReviewerResultCandidatePath,omitempty"`
	LatestReviewerResultCandidateState     string   `json:"latestReviewerResultCandidateState,omitempty"`
	LatestReviewerResultStagingCommand     string   `json:"latestReviewerResultStagingCommand,omitempty"`
	LatestCollectionPreviewCommand         string   `json:"latestCollectionPreviewCommand,omitempty"`
	LatestCollectionApplyCommand           string   `json:"latestCollectionApplyCommand,omitempty"`
	LatestPreviewCommand                   string   `json:"latestPreviewCommand,omitempty"`
	LatestApplyCommand                     string   `json:"latestApplyCommand,omitempty"`
	LatestBatchPreviewCommand              string   `json:"latestBatchPreviewCommand,omitempty"`
	LatestBatchApplyCommand                string   `json:"latestBatchApplyCommand,omitempty"`
	NextActionShardID                      string   `json:"nextActionShardId,omitempty"`
	NextActionState                        string   `json:"nextActionState,omitempty"`
	NextActionDispatchPromptPath           string   `json:"nextActionDispatchPromptPath,omitempty"`
	NextActionDispatchPromptSHA256         string   `json:"nextActionDispatchPromptSha256,omitempty"`
	NextActionDispatchPromptState          string   `json:"nextActionDispatchPromptState,omitempty"`
	NextActionDispatchPromptCurrent        bool     `json:"nextActionDispatchPromptCurrent,omitempty"`
	NextActionDispatchPromptActualSHA256   string   `json:"nextActionDispatchPromptActualSha256,omitempty"`
	NextActionDispatchPromptFailure        string   `json:"nextActionDispatchPromptFailure,omitempty"`
	NextActionDispatchPromptRepairCommand  string   `json:"nextActionDispatchPromptRepairCommand,omitempty"`
	NextActionReviewerResultSourcePath     string   `json:"nextActionReviewerResultSourcePath,omitempty"`
	NextActionReviewerResultSourceState    string   `json:"nextActionReviewerResultSourceState,omitempty"`
	NextActionReviewerResultCandidatePath  string   `json:"nextActionReviewerResultCandidatePath,omitempty"`
	NextActionReviewerResultCandidateState string   `json:"nextActionReviewerResultCandidateState,omitempty"`
	NextActionReviewerResultStagingCommand string   `json:"nextActionReviewerResultStagingCommand,omitempty"`
	NextActionCollectionPreviewCommand     string   `json:"nextActionCollectionPreviewCommand,omitempty"`
	NextActionCollectionApplyCommand       string   `json:"nextActionCollectionApplyCommand,omitempty"`
	NextActionPreviewCommand               string   `json:"nextActionPreviewCommand,omitempty"`
	NextActionApplyCommand                 string   `json:"nextActionApplyCommand,omitempty"`
	NextActionBatchPreviewCommand          string   `json:"nextActionBatchPreviewCommand,omitempty"`
	NextActionBatchApplyCommand            string   `json:"nextActionBatchApplyCommand,omitempty"`
	NextAction                             string   `json:"nextAction,omitempty"`
	NextActionRunbookSteps                 []string `json:"nextActionRunbookSteps,omitempty"`
	Boundary                               []string `json:"boundary,omitempty"`
}

type reviewerPacketIntegrityReference struct {
	Algorithm string `json:"algorithm"`
	Path      string `json:"path"`
}

type reviewerPacketRetirement struct {
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

type reviewerPacketIntegrity struct {
	SchemaVersion int    `json:"schemaVersion"`
	Kind          string `json:"kind"`
	Algorithm     string `json:"algorithm"`
	PacketID      string `json:"packetId"`
	TargetLane    string `json:"targetLane"`
	PacketPath    string `json:"packetPath"`
	PacketSHA256  string `json:"packetSha256"`
	PacketBytes   int    `json:"packetBytes"`
}

type reviewerDispatchPacket struct {
	PacketID              string                              `json:"packetId"`
	PacketIntegrity       *reviewerPacketIntegrityReference   `json:"packetIntegrity"`
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
	DispatchPromptPath          string                            `json:"dispatchPromptPath"`
	DispatchPromptSHA256        string                            `json:"dispatchPromptSha256"`
	AgentToolRequest            *ReviewerAgentToolRequest         `json:"agentToolRequest"`
	StagingCommands             *ReviewerResultStagingCommands    `json:"stagingCommands"`
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
		integrity, integrityErr := readReviewerPacketIntegrity(caseRoot, packetPath)
		integrityPresent := integrityErr == nil
		if integrityPresent && reviewerPacketRetirementCurrent(caseRoot, packetPath, integrity) {
			continue
		}
		packet, ok := readReviewerDispatchPacket(packetPath)
		if !ok {
			if !integrityPresent {
				continue
			}
			if strings.TrimSpace(laneID) != "" && integrity.TargetLane != laneID {
				continue
			}
			packet.PacketID = integrity.PacketID
			packet.TargetLane = integrity.TargetLane
			items = append(items, reviewerPacketIntegrityInvalidHandoff(caseRoot, packet, packetPath, integrity.TargetLane, fmt.Errorf("decode reviewer packet failed while integrity metadata remains")))
			continue
		}
		packetTargetLane := firstText(packet.ReviewerOrchestration.TargetLane, packet.TargetLane, packet.ReviewerOrchestration.OwnerBinding.TargetLane)
		if integrityPresent {
			packetTargetLane = integrity.TargetLane
		}
		if strings.TrimSpace(laneID) != "" && packetTargetLane != laneID {
			continue
		}
		if err := validateReviewerPacketIntegrity(caseRoot, packetPath, packet); err != nil {
			items = append(items, reviewerPacketIntegrityInvalidHandoff(caseRoot, packet, packetPath, packetTargetLane, err))
			continue
		}
		if !strings.EqualFold(strings.TrimSpace(packet.Command), "plan-subagents") || len(packet.ReviewerOrchestration.Dispatches) == 0 {
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

func reviewerPacketIntegrityInvalidHandoff(caseRoot string, packet reviewerDispatchPacket, packetPath, targetLane string, integrityErr error) ReviewerDispatchIntakeHandoff {
	item := ReviewerDispatchIntakeHandoff{
		PacketID:      packet.PacketID,
		PacketPath:    packetPath,
		SummaryPath:   packet.Observability.SummaryPath,
		ResultRoot:    packet.ReviewerOrchestration.ResultRoot,
		TargetLane:    targetLane,
		ShardID:       "packet-integrity",
		DispatchIndex: 1,
		DispatchTotal: 1,
		DispatchOpen:  1,
		RemainingShardIDs: []string{
			"packet-integrity",
		},
		NextOpenShardID: "packet-integrity",
		State:           "reviewer-packet-integrity-invalid",
		Evidence: []string{
			"packet " + reviewerDispatchDisplayPath(caseRoot, packetPath),
			"integrity invalid: " + integrityErr.Error(),
		},
		Boundary: []string{
			"reviewer packet integrity is invalid; do not dispatch, collect, intake, adopt, or continue this lane from the packet",
			"regenerate a canonical reviewer packet; do not repair packet bytes or integrity metadata independently",
			"runtime does not spawn, stop, monitor, or manage reviewer sessions and does not execute heavy tools or write authority/confirmed state",
		},
	}
	return item
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
		case "ready-for-reviewer-result-staging-preview", "ready-for-reviewer-result-collection-preview", "ready-for-reviewer-intake-preview":
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
		blocked := state != "ready-for-reviewer-result-staging-preview" && state != "ready-for-reviewer-result-collection-preview" && state != "ready-for-reviewer-intake-preview" && state != "reviewer-packet-owner-adoption-required" && state != "reviewer-result-recovery-required" && state != "reviewer-result-recovery-finalize-required" && !(state == "reviewer-result-recovery-ambiguous" && handoff.ReviewerResultRecoveryDispositionCommand != "") && !((state == "reviewer-dispatch-prompt-artifact-invalid" || state == "reviewer-dispatch-prompt-artifact-drift") && handoff.DispatchPromptRepairCommand != "")
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
	case "reviewer-packet-integrity-invalid", "reviewer-dispatch-prompt-artifact-invalid", "reviewer-dispatch-prompt-artifact-drift", "reviewer-result-symlink-blocked", "reviewer-result-source-invalid", "reviewer-result-candidate-invalid", "reviewer-result-canonical-invalid", "reviewer-result-collection-required", "reviewer-result-recovery-invalid", "reviewer-result-recovery-ambiguous", "attach-required-before-reviewer-intake":
		return 1
	case "reviewer-result-recovery-required", "reviewer-result-recovery-finalize-required", "ready-for-reviewer-result-staging-preview", "ready-for-reviewer-result-collection-preview", "ready-for-reviewer-intake-preview":
		return 2
	default:
		return 3
	}
}

func ReviewerDispatchIntakeSummaryFor(items []ReviewerDispatchIntakeHandoff) ReviewerDispatchIntakeSummary {
	summary := ReviewerDispatchIntakeSummary{}
	lanes := map[string]bool{}
	packets := map[string]bool{}
	var nextAction *ReviewerDispatchIntakeHandoff
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
		if item.State == "reviewer-dispatch-prompt-artifact-invalid" || item.State == "reviewer-dispatch-prompt-artifact-drift" {
			summary.PromptArtifactBlocked++
		}
		switch item.State {
		case "waiting-for-reviewer-result", "dispatch-only-waiting-for-result":
			summary.WaitingForReviewerResult++
		case "ready-for-reviewer-result-staging-preview", "ready-for-reviewer-result-collection-preview", "ready-for-reviewer-intake-preview":
			summary.ReadyForPreview++
		}
		if item.State == "attach-required-before-reviewer-intake" || item.State == "reviewer-packet-owner-adoption-required" {
			summary.AttachRequired++
		}
		if nextAction == nil || reviewerDispatchActionPriority(item) < reviewerDispatchActionPriority(*nextAction) {
			nextAction = &items[idx]
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
		summary.LatestDispatchPromptPath = latest.DispatchPromptPath
		summary.LatestDispatchPromptSHA256 = latest.DispatchPromptSHA256
		summary.LatestDispatchPromptState = latest.DispatchPromptState
		summary.LatestDispatchPromptCurrent = latest.DispatchPromptCurrent
		summary.LatestDispatchPromptActualSHA256 = latest.DispatchPromptActualSHA256
		summary.LatestDispatchPromptFailure = latest.DispatchPromptFailure
		summary.LatestReviewerResultSourcePath = latest.ReviewerResultSourcePath
		summary.LatestReviewerResultSourceState = latest.ReviewerResultSourceState
		summary.LatestReviewerResultCandidatePath = latest.ReviewerResultCandidatePath
		summary.LatestReviewerResultCandidateState = latest.ReviewerResultCandidateState
		summary.LatestReviewerResultStagingCommand = latest.ReviewerResultStagingCommand
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
		if nextAction != nil {
			summary.NextActionShardID = nextAction.ShardID
			summary.NextActionState = nextAction.State
			summary.NextActionDispatchPromptPath = nextAction.DispatchPromptPath
			summary.NextActionDispatchPromptSHA256 = nextAction.DispatchPromptSHA256
			summary.NextActionDispatchPromptState = nextAction.DispatchPromptState
			summary.NextActionDispatchPromptCurrent = nextAction.DispatchPromptCurrent
			summary.NextActionDispatchPromptActualSHA256 = nextAction.DispatchPromptActualSHA256
			summary.NextActionDispatchPromptFailure = nextAction.DispatchPromptFailure
			summary.NextActionDispatchPromptRepairCommand = nextAction.DispatchPromptRepairCommand
			summary.NextActionReviewerResultSourcePath = nextAction.ReviewerResultSourcePath
			summary.NextActionReviewerResultSourceState = nextAction.ReviewerResultSourceState
			summary.NextActionReviewerResultCandidatePath = nextAction.ReviewerResultCandidatePath
			summary.NextActionReviewerResultCandidateState = nextAction.ReviewerResultCandidateState
			summary.NextActionReviewerResultStagingCommand = nextAction.ReviewerResultStagingCommand
			if nextAction.ReviewerResultCollectionCommands != nil {
				summary.NextActionCollectionPreviewCommand = nextAction.ReviewerResultCollectionCommands.PreviewCommand
				summary.NextActionCollectionApplyCommand = nextAction.ReviewerResultCollectionCommands.ApplyCommand
			}
			summary.NextActionPreviewCommand = nextAction.PreviewCommand
			summary.NextActionApplyCommand = nextAction.ApplyCommand
			summary.NextActionBatchPreviewCommand = nextAction.BatchPreviewCommand
			summary.NextActionBatchApplyCommand = nextAction.BatchApplyCommand
			if nextAction.State == "ready-for-reviewer-intake-preview" && strings.TrimSpace(nextAction.BatchPreviewCommand) != "" {
				summary.LatestBatchPreviewCommand = nextAction.BatchPreviewCommand
				summary.LatestBatchApplyCommand = nextAction.BatchApplyCommand
			}
			summary.NextAction = reviewerDispatchIntakeNextAction(*nextAction)
			summary.NextActionRunbookSteps = reviewerDispatchIntakeRunbookSteps(*nextAction)
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

func readStableReviewerWorkstreamArtifact(caseRoot, path, label string) ([]byte, error) {
	if !reviewpath.CollectionNamespacePathSafe(caseRoot, path, false) {
		return nil, fmt.Errorf("%s path is not safe", label)
	}
	before, err := os.Lstat(path)
	if err != nil {
		return nil, fmt.Errorf("read %s: %w", label, err)
	}
	if !before.Mode().IsRegular() || before.Size() <= 0 || before.Size() > 4<<20 {
		return nil, fmt.Errorf("%s must be a non-empty regular file within size limit", label)
	}
	file, err := os.Open(path)
	if err != nil {
		return nil, fmt.Errorf("read %s: %w", label, err)
	}
	defer file.Close()
	opened, err := file.Stat()
	if err != nil || !opened.Mode().IsRegular() || !os.SameFile(before, opened) {
		return nil, fmt.Errorf("%s changed while opening", label)
	}
	data, err := io.ReadAll(io.LimitReader(file, (4<<20)+1))
	if err != nil {
		return nil, fmt.Errorf("read %s: %w", label, err)
	}
	if len(data) > 4<<20 {
		return nil, fmt.Errorf("%s exceeds size limit", label)
	}
	after, err := os.Lstat(path)
	if err != nil || !os.SameFile(opened, after) {
		return nil, fmt.Errorf("%s changed while reading", label)
	}
	return data, nil
}

func readReviewerPacketIntegrity(caseRoot, packetPath string) (reviewerPacketIntegrity, error) {
	integrityPath := filepath.Join(filepath.Dir(packetPath), "packet.integrity.json")
	data, err := readStableReviewerWorkstreamArtifact(caseRoot, integrityPath, "reviewer packet integrity")
	if err != nil {
		return reviewerPacketIntegrity{}, err
	}
	dec := json.NewDecoder(bytes.NewReader(bytes.TrimSpace(data)))
	dec.DisallowUnknownFields()
	var integrity reviewerPacketIntegrity
	if err := dec.Decode(&integrity); err != nil {
		return reviewerPacketIntegrity{}, fmt.Errorf("decode reviewer packet integrity: %w", err)
	}
	var trailing any
	if err := dec.Decode(&trailing); err != io.EOF {
		return reviewerPacketIntegrity{}, fmt.Errorf("reviewer packet integrity must contain exactly one JSON object")
	}
	if integrity.SchemaVersion != 1 || integrity.Kind != "reviewer-packet-integrity" || !strings.EqualFold(strings.TrimSpace(integrity.Algorithm), "sha256") || strings.TrimSpace(integrity.PacketID) == "" || strings.TrimSpace(integrity.TargetLane) == "" || !casebind.SamePath(integrity.PacketPath, packetPath) || integrity.PacketBytes < 0 {
		return reviewerPacketIntegrity{}, fmt.Errorf("reviewer packet integrity has unsupported identity or provenance")
	}
	if decoded, err := hex.DecodeString(integrity.PacketSHA256); err != nil || len(decoded) != sha256.Size {
		return reviewerPacketIntegrity{}, fmt.Errorf("reviewer packet integrity packetSha256 is invalid")
	}
	return integrity, nil
}

func reviewerPacketRetirementCurrent(caseRoot, packetPath string, integrity reviewerPacketIntegrity) bool {
	inst, err := instance.Read(caseRoot)
	if err != nil || inst.Source == "missing" || inst.Moved() || strings.TrimSpace(inst.TemplateRoot) == "" || strings.TrimSpace(inst.TemplatePack) == "" {
		return false
	}
	retirementPath := filepath.Join(filepath.Dir(packetPath), "packet.retirement.json")
	if !reviewpath.CollectionNamespacePathSafe(caseRoot, retirementPath, false) {
		return false
	}
	data, err := readStableReviewerWorkstreamArtifact(caseRoot, retirementPath, "reviewer packet retirement")
	if err != nil {
		return false
	}
	dec := json.NewDecoder(bytes.NewReader(bytes.TrimSpace(data)))
	dec.DisallowUnknownFields()
	var retirement reviewerPacketRetirement
	if err := dec.Decode(&retirement); err != nil {
		return false
	}
	var trailing any
	if err := dec.Decode(&trailing); err != io.EOF {
		return false
	}
	packetData, packetErr := readStableReviewerWorkstreamArtifact(caseRoot, packetPath, "reviewer packet")
	integrityPath := filepath.Join(filepath.Dir(packetPath), "packet.integrity.json")
	integrityData, integrityErr := readStableReviewerWorkstreamArtifact(caseRoot, integrityPath, "reviewer packet integrity")
	if packetErr != nil || integrityErr != nil {
		return false
	}
	packetSum := sha256.Sum256(packetData)
	integritySum := sha256.Sum256(integrityData)
	_, createdAtErr := time.Parse(time.RFC3339Nano, retirement.CreatedAt)
	return retirement.SchemaVersion == 1 && retirement.Kind == "reviewer-packet-retirement" &&
		casebind.SamePath(retirement.RepoRoot, inst.TemplateRoot) && casebind.SamePath(retirement.CaseRoot, inst.CaseRoot) && retirement.Pack == inst.TemplatePack &&
		retirement.PacketID == integrity.PacketID && retirement.Lane == integrity.TargetLane &&
		casebind.SamePath(retirement.PacketPath, packetPath) && retirement.PacketSHA256 == hex.EncodeToString(packetSum[:]) && retirement.PacketBytes == len(packetData) &&
		casebind.SamePath(retirement.IntegrityPath, integrityPath) && retirement.IntegritySHA256 == hex.EncodeToString(integritySum[:]) && retirement.IntegrityBytes == len(integrityData) &&
		strings.TrimSpace(retirement.Actor) != "" && strings.TrimSpace(retirement.Reason) != "" && createdAtErr == nil && retirement.NoDelete && retirement.NoHeavyTool && retirement.NoAuthority
}

func validateReviewerPacketIntegrity(caseRoot, packetPath string, packet reviewerDispatchPacket) error {
	if packet.PacketIntegrity == nil {
		if _, err := os.Lstat(filepath.Join(filepath.Dir(packetPath), "packet.integrity.json")); err == nil {
			return fmt.Errorf("reviewer packet integrity reference is missing while canonical sidecar exists")
		} else if !os.IsNotExist(err) {
			return fmt.Errorf("inspect reviewer packet integrity: %w", err)
		}
		return nil
	}
	integrityPath := strings.TrimSpace(packet.PacketIntegrity.Path)
	if !strings.EqualFold(strings.TrimSpace(packet.PacketIntegrity.Algorithm), "sha256") ||
		!casebind.SamePath(integrityPath, filepath.Join(filepath.Dir(packetPath), "packet.integrity.json")) {
		return fmt.Errorf("reviewer packet integrity reference is not canonical")
	}
	integrity, err := readReviewerPacketIntegrity(caseRoot, packetPath)
	if err != nil {
		return err
	}
	packetData, err := os.ReadFile(packetPath)
	if err != nil {
		return fmt.Errorf("read reviewer packet for integrity: %w", err)
	}
	sum := sha256.Sum256(packetData)
	if integrity.SchemaVersion != 1 || integrity.Kind != "reviewer-packet-integrity" ||
		!strings.EqualFold(integrity.Algorithm, "sha256") || integrity.PacketID != packet.PacketID ||
		integrity.TargetLane != packet.TargetLane || !casebind.SamePath(integrity.PacketPath, packetPath) ||
		integrity.PacketSHA256 != hex.EncodeToString(sum[:]) || integrity.PacketBytes != len(packetData) {
		return fmt.Errorf("reviewer packet integrity does not match packet bytes and bindings")
	}
	return nil
}

func reviewerDispatchResultRecoveryDispositionCommand(packetPath, shardID, lane string) string {
	return "/rekit plan-subagents -PacketPath " + quoteCommandArg(packetPath) +
		" -RetireReviewerResultRecovery -ShardId " + quoteCommandArg(shardID) +
		" -Lane " + quoteCommandArg(lane) + " -Actor <main-agent> -Reason " +
		quoteCommandArg("retain exact reviewed canonical result and retire ambiguous recovery") + " -WhatIf -Format json"
}

func reviewerDispatchResultRecoveryCommand(packetPath, shardID, lane string) string {
	return "/rekit plan-subagents -PacketPath " + quoteCommandArg(packetPath) +
		" -RecoverReviewerResult -ShardId " + quoteCommandArg(shardID) +
		" -Lane " + quoteCommandArg(lane) + " -Actor <main-agent> -Reason " +
		quoteCommandArg("quarantine conflicting canonical reviewer result") + " -WhatIf -Format json"
}

func reviewerDispatchPromptArtifactRepairCommand(packetPath, shardID, lane string) string {
	return "/rekit plan-subagents -PacketPath " + quoteCommandArg(packetPath) +
		" -RepairReviewerPromptArtifact -ShardId " + quoteCommandArg(shardID) +
		" -Lane " + quoteCommandArg(lane) + " -Actor <main-agent> -WhatIf -Format json"
}

func reviewerResultObstructionRecoverable(path string) bool {
	st, err := os.Lstat(path)
	if err != nil {
		return false
	}
	return runtime.GOOS == "windows" && (st.Mode()&os.ModeSymlink != 0 || st.Mode().IsRegular() && st.Size() == 0)
}

func reviewerResultRecoveryDispositionCurrent(caseRoot string, packet reviewerDispatchPacket, packetPath string, dispatch reviewerDispatchPacketDispatch, targetLane, candidatePath, resultPath, intentPath string) (bool, error) {
	path := filepath.Join(packet.ReviewerOrchestration.ResultRoot, "recoveries", dispatch.ShardID+".recovery.disposition.json")
	data, err := readStableReviewerWorkstreamArtifact(caseRoot, path, "reviewer result recovery disposition")
	if err != nil {
		return false, err
	}
	dec := json.NewDecoder(bytes.NewReader(bytes.TrimSpace(data)))
	dec.DisallowUnknownFields()
	var disposition reviewerResultRecoveryDispositionRecord
	if err := dec.Decode(&disposition); err != nil {
		return false, err
	}
	var trailing any
	if err := dec.Decode(&trailing); err != io.EOF {
		return false, fmt.Errorf("reviewer result recovery disposition must contain exactly one JSON object")
	}
	if _, err := time.Parse(time.RFC3339Nano, disposition.CreatedAt); err != nil {
		return false, err
	}
	candidate, err := readStableReviewerWorkstreamArtifact(caseRoot, candidatePath, "reviewer result candidate")
	if err != nil {
		return false, err
	}
	canonical, err := readStableReviewerWorkstreamArtifact(caseRoot, resultPath, "canonical reviewer result")
	if err != nil {
		return false, err
	}
	intentData, err := readStableReviewerWorkstreamArtifact(caseRoot, intentPath, "reviewer result recovery intent")
	if err != nil {
		return false, err
	}
	intent, err := readReviewerResultRecoveryRecord(caseRoot, intentPath)
	if err != nil {
		return false, err
	}
	current := disposition.SchemaVersion == 1 && disposition.Kind == "reviewer-result-recovery-disposition" && disposition.Decision == "retain-canonical" && casebind.SamePath(disposition.RepoRoot, packet.RepoRoot) && casebind.SamePath(disposition.CaseRoot, caseRoot) && disposition.Pack == packet.Pack && disposition.Pack == intent.Pack && casebind.SamePath(intent.RepoRoot, packet.RepoRoot) && disposition.PacketID == packet.PacketID && casebind.SamePath(disposition.PacketPath, packetPath) && disposition.ShardID == dispatch.ShardID && disposition.Lane == targetLane && casebind.SamePath(disposition.CandidatePath, candidatePath) && disposition.CandidateSHA256 == reviewerDispatchBytesSHA256(candidate) && disposition.CandidateBytes == len(candidate) && casebind.SamePath(disposition.ReviewerResultPath, resultPath) && disposition.CanonicalSHA256 == reviewerDispatchBytesSHA256(canonical) && disposition.CanonicalBytes == len(canonical) && bytes.Equal(canonical, candidate) && casebind.SamePath(disposition.IntentPath, intentPath) && disposition.IntentSHA256 == reviewerDispatchBytesSHA256(intentData) && disposition.IntentBytes == len(intentData) && casebind.SamePath(disposition.QuarantinePath, intent.QuarantinePath) && disposition.Actor != "" && disposition.Reason != "" && disposition.NoDelete && disposition.NoFacts && disposition.NoHeavyTool && disposition.NoAuthority && reviewerResultRecoveryRecordMatches(intent, caseRoot, packet, packetPath, dispatch, targetLane, candidatePath) && reviewerResultRecoveryQuarantineCurrent(caseRoot, intent)
	return current, nil
}

func readReviewerResultRecoveryRecord(caseRoot, path string) (reviewerResultRecoveryRecord, error) {
	data, err := readStableReviewerWorkstreamArtifact(caseRoot, path, "reviewer result recovery record")
	if err != nil {
		return reviewerResultRecoveryRecord{}, err
	}
	dec := json.NewDecoder(bytes.NewReader(bytes.TrimSpace(data)))
	dec.DisallowUnknownFields()
	var record reviewerResultRecoveryRecord
	if err := dec.Decode(&record); err != nil {
		return reviewerResultRecoveryRecord{}, err
	}
	var trailing any
	if err := dec.Decode(&trailing); err != io.EOF {
		return reviewerResultRecoveryRecord{}, fmt.Errorf("reviewer result recovery record must contain exactly one JSON object")
	}
	candidateHash, candidateErr := hex.DecodeString(record.CandidateSHA256)
	resultHash, resultErr := hex.DecodeString(record.ReviewerResultSHA256)
	validResultKind := record.ReviewerResultKind == "regular-file" || record.ReviewerResultKind == "empty-file" || record.ReviewerResultKind == "symlink" || record.ReviewerResultKind == "directory" || record.ReviewerResultKind == "non-regular"
	if record.SchemaVersion != 1 || record.Kind != "reviewer-result-recovery" || candidateErr != nil || resultErr != nil || len(candidateHash) != sha256.Size || len(resultHash) != sha256.Size || record.CandidateBytes <= 0 || !validResultKind || record.ReviewerResultBytes < 0 || (record.ReviewerResultKind == "regular-file" && record.ReviewerResultBytes <= 0) {
		return reviewerResultRecoveryRecord{}, fmt.Errorf("reviewer result recovery record contract is invalid")
	}
	if _, err := time.Parse(time.RFC3339Nano, record.CreatedAt); err != nil || strings.TrimSpace(record.Actor) == "" || strings.TrimSpace(record.Reason) == "" || !record.NoVerdict || !record.NoFacts || !record.NoHeavyTool || !record.NoAuthority {
		return reviewerResultRecoveryRecord{}, fmt.Errorf("reviewer result recovery record decision or boundary is invalid")
	}
	return record, nil
}

func reviewerResultRecoveryRecordMatches(record reviewerResultRecoveryRecord, caseRoot string, packet reviewerDispatchPacket, packetPath string, dispatch reviewerDispatchPacketDispatch, lane, candidatePath string) bool {
	candidate, err := readStableReviewerWorkstreamArtifact(caseRoot, candidatePath, "reviewer result candidate")
	if err != nil {
		return false
	}
	expectedQuarantinePath := filepath.Join(packet.ReviewerOrchestration.ResultRoot, "recoveries", dispatch.ShardID+"-"+record.ReviewerResultSHA256+".json")
	quarantinePathSafe := reviewpath.CollectionNamespacePathSafe(caseRoot, record.QuarantinePath, false)
	if record.ReviewerResultKind != "regular-file" {
		quarantinePathSafe = reviewpath.CollectionNamespacePathSafe(caseRoot, filepath.Dir(record.QuarantinePath), false)
	}
	inst, instErr := instance.Read(caseRoot)
	return instErr == nil && casebind.SamePath(record.RepoRoot, inst.TemplateRoot) && record.Pack == inst.TemplatePack &&
		casebind.SamePath(record.CaseRoot, caseRoot) && record.PacketID == packet.PacketID && casebind.SamePath(record.PacketPath, packetPath) &&
		record.ShardID == dispatch.ShardID && record.Lane == lane && casebind.SamePath(record.CandidatePath, candidatePath) &&
		record.CandidateSHA256 == reviewerDispatchBytesSHA256(candidate) && record.CandidateBytes == len(candidate) &&
		casebind.SamePath(record.ReviewerResultPath, dispatch.ReviewerResultPath) && casebind.SamePath(record.QuarantinePath, expectedQuarantinePath) &&
		quarantinePathSafe
}

func reviewerResultRecoveryQuarantineCurrent(caseRoot string, record reviewerResultRecoveryRecord) bool {
	if record.ReviewerResultKind == "regular-file" {
		data, err := readStableReviewerWorkstreamArtifact(caseRoot, record.QuarantinePath, "quarantined reviewer result")
		return err == nil && reviewerDispatchBytesSHA256(data) == record.ReviewerResultSHA256 && len(data) == record.ReviewerResultBytes
	}
	st, err := os.Lstat(record.QuarantinePath)
	if err != nil {
		return false
	}
	kind := "non-regular"
	linkTarget := ""
	switch {
	case st.Mode()&os.ModeSymlink != 0:
		kind = "symlink"
		linkTarget, err = os.Readlink(record.QuarantinePath)
	case st.IsDir():
		kind = "directory"
		var entries []os.DirEntry
		entries, err = os.ReadDir(record.QuarantinePath)
		if len(entries) != 0 {
			return false
		}
	case st.Mode().IsRegular() && st.Size() == 0:
		kind = "empty-file"
	}
	if err != nil {
		return false
	}
	identity := fmt.Sprintf("kind=%s\nmode=%d\nsize=%d\nlink=%s\n", kind, uint32(st.Mode()), st.Size(), linkTarget)
	return kind == record.ReviewerResultKind && reviewerDispatchBytesSHA256([]byte(identity)) == record.ReviewerResultSHA256 && int(st.Size()) == record.ReviewerResultBytes && uint32(st.Mode()) == record.ReviewerResultMode && linkTarget == record.ReviewerResultLinkTarget
}

func reviewerResultRecoveryRecordsEquivalent(left, right reviewerResultRecoveryRecord) bool {
	return left.SchemaVersion == right.SchemaVersion && left.Kind == right.Kind && left.CreatedAt == right.CreatedAt &&
		casebind.SamePath(left.RepoRoot, right.RepoRoot) && casebind.SamePath(left.CaseRoot, right.CaseRoot) && left.Pack == right.Pack &&
		left.PacketID == right.PacketID && casebind.SamePath(left.PacketPath, right.PacketPath) && left.ShardID == right.ShardID && left.Lane == right.Lane &&
		casebind.SamePath(left.CandidatePath, right.CandidatePath) && left.CandidateSHA256 == right.CandidateSHA256 && left.CandidateBytes == right.CandidateBytes &&
		casebind.SamePath(left.ReviewerResultPath, right.ReviewerResultPath) && left.ReviewerResultKind == right.ReviewerResultKind && left.ReviewerResultSHA256 == right.ReviewerResultSHA256 && left.ReviewerResultBytes == right.ReviewerResultBytes && left.ReviewerResultMode == right.ReviewerResultMode && left.ReviewerResultLinkTarget == right.ReviewerResultLinkTarget &&
		casebind.SamePath(left.QuarantinePath, right.QuarantinePath) && left.Actor == right.Actor && left.Reason == right.Reason &&
		left.NoVerdict == right.NoVerdict && left.NoFacts == right.NoFacts && left.NoHeavyTool == right.NoHeavyTool && left.NoAuthority == right.NoAuthority
}

func reviewerDispatchBytesSHA256(data []byte) string {
	sum := sha256.Sum256(data)
	return hex.EncodeToString(sum[:])
}

type reviewerDispatchPromptArtifact struct {
	Path           string
	ExpectedSHA256 string
	State          string
	Current        bool
	ActualSHA256   string
	Failure        string
}

func reviewerDispatchPromptArtifactStatus(caseRoot, packetPath string, dispatch reviewerDispatchPacketDispatch) reviewerDispatchPromptArtifact {
	path := strings.TrimSpace(dispatch.DispatchPromptPath)
	expectedSHA256 := strings.TrimSpace(dispatch.DispatchPromptSHA256)
	if dispatch.AgentToolRequest != nil {
		if path == "" {
			path = strings.TrimSpace(dispatch.AgentToolRequest.PromptPath)
		}
		if expectedSHA256 == "" {
			expectedSHA256 = strings.TrimSpace(dispatch.AgentToolRequest.PromptSHA256)
		}
	}
	status := reviewerDispatchPromptArtifact{Path: path, ExpectedSHA256: expectedSHA256}
	if path == "" {
		if expectedSHA256 != "" {
			status.State = "invalid"
			status.Failure = "reviewer prompt artifact path is missing"
		}
		return status
	}
	reviewRoot := filepath.Dir(packetPath)
	promptRoot := filepath.Dir(path)
	if reviewRoot == "." || filepath.Base(promptRoot) != "prompts" || !casebind.SamePath(filepath.Dir(promptRoot), reviewRoot) {
		status.State = "invalid"
		status.Failure = "reviewer prompt artifact path must stay under the packet prompts directory"
		return status
	}
	if !reviewpath.CollectionNamespacePathSafe(caseRoot, promptRoot, false) {
		status.State = "invalid"
		status.Failure = "reviewer prompt artifact parent is outside the safe case namespace, missing, or crosses a symlink"
		return status
	}
	classified, err := refsf.ClassifyNonEmptyRegularFile(path)
	if err != nil {
		status.State = "invalid"
		status.Failure = "classify reviewer prompt artifact: " + err.Error()
		return status
	}
	switch classified {
	case refsf.RegularFileMissing:
		status.State = "missing"
		status.Failure = "reviewer prompt artifact is missing"
		return status
	case refsf.RegularFileWaiting:
		status.State = "invalid"
		status.Failure = "reviewer prompt artifact must be a non-empty regular file"
		return status
	case refsf.RegularFileSymlink:
		status.State = "symlink"
		status.Failure = "reviewer prompt artifact must not be a symlink"
		return status
	}
	data, err := readStableReviewerWorkstreamArtifact(caseRoot, path, "reviewer prompt artifact")
	if err != nil {
		status.State = "invalid"
		status.Failure = err.Error()
		return status
	}
	status.ActualSHA256 = reviewerDispatchBytesSHA256(data)
	if expectedSHA256 == "" {
		status.State = "unverified"
		status.Failure = "reviewer prompt artifact sha256 is missing"
		return status
	}
	if _, err := hex.DecodeString(expectedSHA256); err != nil || len(expectedSHA256) != sha256.Size*2 {
		status.State = "invalid"
		status.Failure = "reviewer prompt artifact expected sha256 is invalid"
		return status
	}
	if !strings.EqualFold(expectedSHA256, status.ActualSHA256) {
		status.State = "drift"
		status.Failure = "reviewer prompt artifact sha256 drift"
		return status
	}
	status.State = "ready"
	status.Current = true
	return status
}

func reviewerDispatchPromptArtifactBlocksDispatch(status reviewerDispatchPromptArtifact, state string) bool {
	if status.Path == "" && status.ExpectedSHA256 == "" {
		return false
	}
	if status.Current {
		return false
	}
	return state == "waiting-for-reviewer-result" || state == "dispatch-only-waiting-for-result"
}

func reviewerDispatchPromptArtifactBlockedState(status reviewerDispatchPromptArtifact) string {
	if status.State == "drift" {
		return "reviewer-dispatch-prompt-artifact-drift"
	}
	return "reviewer-dispatch-prompt-artifact-invalid"
}

func reviewerDispatchResultRecoveryApplyCommand(packetPath, shardID, lane string, record reviewerResultRecoveryRecord) string {
	return "/rekit plan-subagents -PacketPath " + quoteCommandArg(packetPath) +
		" -RecoverReviewerResult -ShardId " + quoteCommandArg(shardID) +
		" -Lane " + quoteCommandArg(lane) + " -Actor " + quoteCommandArg(record.Actor) +
		" -Reason " + quoteCommandArg(record.Reason) +
		" -ExpectedCandidateSha256 " + quoteCommandArg(record.CandidateSHA256) +
		" -ExpectedReviewerResultSha256 " + quoteCommandArg(record.ReviewerResultSHA256) +
		" -Apply -Format json"
}

func reviewerDispatchResultStagingCommand(packetPath, shardID, lane, sourcePath string) string {
	return "/rekit plan-subagents -PacketPath " + quoteCommandArg(packetPath) +
		" -StageReviewerResult -ShardId " + quoteCommandArg(shardID) +
		" -ReviewerResultSourcePath " + quoteCommandArg(sourcePath) +
		" -Lane " + quoteCommandArg(lane) + " -Actor <main-agent>" +
		" -WhatIf -Format json"
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

func reviewerDispatchStagingSourcePath(dispatch reviewerDispatchPacketDispatch, expected string) string {
	if dispatch.StagingCommands != nil && strings.TrimSpace(dispatch.StagingCommands.SourcePath) != "" {
		return strings.TrimSpace(dispatch.StagingCommands.SourcePath)
	}
	return expected
}

func reviewerDispatchIntakeHandoffFor(caseRoot string, facts mission.LedgerFacts, packet reviewerDispatchPacket, packetPath, targetLane string, dispatch reviewerDispatchPacketDispatch, idx int) ReviewerDispatchIntakeHandoff {
	resultPath := strings.TrimSpace(dispatch.ReviewerResultPath)
	candidatePath := strings.TrimSpace(dispatch.ReviewerResultCandidatePath)
	expectedSourcePath := filepath.Join(packet.ReviewerOrchestration.ResultRoot, "sources", dispatch.ShardID+".json")
	sourcePath := reviewerDispatchStagingSourcePath(dispatch, expectedSourcePath)
	collectionAvailable := packet.ReviewerOrchestration.PacketPath != "" &&
		reviewpath.CanonicalCollectionShard(caseRoot, packetPath, packet.ReviewerOrchestration.ResultRoot, dispatch.ShardID, candidatePath, resultPath) &&
		reviewpath.CollectionNamespacePathSafe(caseRoot, packetPath, false) &&
		reviewpath.CollectionNamespacePathSafe(caseRoot, packet.ReviewerOrchestration.ResultRoot, false) &&
		reviewpath.CollectionNamespacePathSafe(caseRoot, filepath.Dir(sourcePath), true) &&
		reviewpath.CollectionNamespacePathSafe(caseRoot, filepath.Dir(candidatePath), true) &&
		reviewpath.CollectionNamespacePathSafe(caseRoot, filepath.Dir(resultPath), false) &&
		casebind.SamePath(packet.ReviewerOrchestration.PacketPath, packetPath) &&
		casebind.SamePath(sourcePath, expectedSourcePath) &&
		dispatch.CollectionCommands != nil &&
		casebind.SamePath(dispatch.CollectionCommands.CandidatePath, candidatePath) &&
		reviewerDispatchIntakeCommandAvailable(dispatch.CollectionCommands.PreviewCommand) &&
		reviewerDispatchIntakeCommandAvailable(dispatch.CollectionCommands.ApplyCommand)
	stagingCommand := ""
	var collectionCommands *ReviewerResultCollectionCommands
	if collectionAvailable {
		stagingCommand = reviewerDispatchResultStagingCommand(packetPath, dispatch.ShardID, targetLane, sourcePath)
		commands := reviewerDispatchCollectionCommands(packetPath, dispatch.ShardID, targetLane, candidatePath)
		collectionCommands = &commands
	} else {
		sourcePath = ""
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
	sourceState := ""
	if sourcePath != "" {
		sourceState = "missing"
		if classified, err := refsf.ClassifyNonEmptyRegularFile(sourcePath); err != nil || classified == refsf.RegularFileSymlink || classified == refsf.RegularFileWaiting {
			if classified != refsf.RegularFileMissing {
				sourceState = "invalid"
			}
		} else if classified == refsf.RegularFileReady {
			sourceState = "ready"
		}
	}
	candidateState := ""
	if candidatePath != "" {
		candidateState = "missing"
		if classified, err := refsf.ClassifyNonEmptyRegularFile(candidatePath); err != nil || classified == refsf.RegularFileSymlink || classified == refsf.RegularFileWaiting {
			if classified != refsf.RegularFileMissing {
				candidateState = "invalid"
			}
		} else if classified == refsf.RegularFileReady {
			candidateState = "ready"
		}
	}
	intakeAvailable := reviewerDispatchIntakeCommandAvailable(dispatch.PreviewCommand) && reviewerDispatchIntakeCommandAvailable(dispatch.ApplyCommand)
	verificationRecorded := reviewerDispatchWritebackRecorded(facts.Verifications, packet.PacketID, dispatch.ShardID, resultPath)
	decisionRecorded := reviewerDispatchWritebackRecorded(facts.Decisions, packet.PacketID, dispatch.ShardID, resultPath)
	state := reviewerDispatchIntakeState(resultState, intakeAvailable)
	recoveryCommand := ""
	recoveryApplyCommand := ""
	recoveryDispositionCommand := ""
	recoveryDispositionPath := ""
	if present && collectionAvailable {
		switch candidateState {
		case "ready":
			candidateBytes, candidateErr := readStableReviewerWorkstreamArtifact(caseRoot, candidatePath, "reviewer result candidate")
			resultBytes, resultErr := readStableReviewerWorkstreamArtifact(caseRoot, resultPath, "canonical reviewer result")
			if candidateErr == nil && resultErr == nil && bytes.Equal(candidateBytes, resultBytes) {
				candidateState = "collected"
			} else if !verificationRecorded && !decisionRecorded {
				state = "reviewer-result-recovery-required"
				recoveryCommand = reviewerDispatchResultRecoveryCommand(packetPath, dispatch.ShardID, targetLane)
			} else {
				state = "reviewer-result-candidate-invalid"
			}
		case "missing":
			state = "reviewer-result-collection-required"
		case "invalid":
			state = "reviewer-result-candidate-invalid"
		}
	}
	intentPath := filepath.Join(packet.ReviewerOrchestration.ResultRoot, "recoveries", dispatch.ShardID+".recovery.intent.json")
	receiptPath := filepath.Join(packet.ReviewerOrchestration.ResultRoot, "recoveries", dispatch.ShardID+".recovery.json")
	projectRecoveryState := func() {
		intentState, err := refsf.ClassifyNonEmptyRegularFile(intentPath)
		if err != nil || (intentState != refsf.RegularFileMissing && intentState != refsf.RegularFileReady) {
			state = "reviewer-result-recovery-invalid"
			return
		}
		if intentState == refsf.RegularFileMissing {
			return
		}
		intent, intentErr := readReviewerResultRecoveryRecord(caseRoot, intentPath)
		if intentErr != nil || !reviewerResultRecoveryRecordMatches(intent, caseRoot, packet, packetPath, dispatch, targetLane, candidatePath) || !reviewerResultRecoveryQuarantineCurrent(caseRoot, intent) {
			state = "reviewer-result-recovery-invalid"
			return
		}
		if receipt, receiptErr := readReviewerResultRecoveryRecord(caseRoot, receiptPath); receiptErr == nil && receipt.CreatedAt == intent.CreatedAt && reviewerResultRecoveryRecordsEquivalent(intent, receipt) {
			return
		} else if _, receiptStatErr := os.Lstat(receiptPath); receiptStatErr == nil || !os.IsNotExist(receiptStatErr) {
			state = "reviewer-result-recovery-invalid"
			return
		}
		if resultState != refsf.RegularFileMissing {
			recoveryDispositionPath = filepath.Join(packet.ReviewerOrchestration.ResultRoot, "recoveries", dispatch.ShardID+".recovery.disposition.json")
			if dispositionState, dispositionErr := refsf.ClassifyNonEmptyRegularFile(recoveryDispositionPath); dispositionErr != nil {
				state = "reviewer-result-recovery-invalid"
				return
			} else if dispositionState == refsf.RegularFileReady {
				current, currentErr := reviewerResultRecoveryDispositionCurrent(caseRoot, packet, packetPath, dispatch, targetLane, candidatePath, resultPath, intentPath)
				if currentErr != nil || !current {
					state = "reviewer-result-recovery-invalid"
					return
				}
				return
			} else if dispositionState != refsf.RegularFileMissing {
				state = "reviewer-result-recovery-invalid"
				return
			}
			state = "reviewer-result-recovery-ambiguous"
			recoveryDispositionCommand = reviewerDispatchResultRecoveryDispositionCommand(packetPath, dispatch.ShardID, targetLane)
			return
		}
		state = "reviewer-result-recovery-finalize-required"
		recoveryCommand = reviewerDispatchResultRecoveryCommand(packetPath, dispatch.ShardID, targetLane)
		recoveryApplyCommand = reviewerDispatchResultRecoveryApplyCommand(packetPath, dispatch.ShardID, targetLane, intent)
	}
	if !verificationRecorded && !decisionRecorded {
		projectRecoveryState()
	}
	recoveryProjected := state == "reviewer-result-recovery-invalid" || state == "reviewer-result-recovery-ambiguous" || state == "reviewer-result-recovery-finalize-required" || state == "reviewer-result-collection-required"
	if !recoveryProjected && !present && resultState == refsf.RegularFileWaiting && candidatePath != "" {
		state = "reviewer-result-canonical-invalid"
		if reviewerResultObstructionRecoverable(resultPath) {
			recoveryCommand = reviewerDispatchResultRecoveryCommand(packetPath, dispatch.ShardID, targetLane)
		}
	} else if !recoveryProjected && !present && resultState == refsf.RegularFileSymlink && candidatePath != "" {
		state = "reviewer-result-symlink-blocked"
		if reviewerResultObstructionRecoverable(resultPath) {
			recoveryCommand = reviewerDispatchResultRecoveryCommand(packetPath, dispatch.ShardID, targetLane)
		}
	} else if !recoveryProjected && !present && candidateState == "invalid" {
		state = "reviewer-result-candidate-invalid"
	} else if !recoveryProjected && !present && candidateState == "ready" && collectionCommands != nil && reviewerDispatchIntakeCommandAvailable(collectionCommands.PreviewCommand) {
		state = "ready-for-reviewer-result-collection-preview"
	} else if !recoveryProjected && !present && sourceState == "invalid" {
		state = "reviewer-result-source-invalid"
	} else if !recoveryProjected && !present && sourceState == "ready" && stagingCommand != "" {
		state = "ready-for-reviewer-result-staging-preview"
	}
	promptArtifact := reviewerDispatchPromptArtifactStatus(caseRoot, packetPath, dispatch)
	if reviewerDispatchPromptArtifactBlocksDispatch(promptArtifact, state) {
		state = reviewerDispatchPromptArtifactBlockedState(promptArtifact)
	}
	currentExecutor, currentGeneration := reviewerDispatchCurrentOwner(caseRoot, targetLane)
	adoptionPath := filepath.Join(caseRoot, ".rekit", "reviewer-adoptions", packet.PacketID+".json")
	adoptionCurrent := reviewerDispatchAdoptionCurrent(caseRoot, adoptionPath, packet, packetPath, currentExecutor, currentGeneration)
	ownerStale := currentExecutor != strings.TrimSpace(packet.ReviewerOrchestration.OwnerBinding.CurrentExecutor) ||
		currentGeneration != packet.ReviewerOrchestration.OwnerBinding.ExecutorGeneration
	if ownerStale && !adoptionCurrent {
		state = "reviewer-packet-owner-adoption-required"
	}
	dispatchCommand := reviewerDispatchCommand(dispatch.ShardID, stagingCommand, sourcePath, candidatePath, resultPath, dispatch.DispatchPromptPath, dispatch.DispatchPromptSHA256, dispatch.AgentToolRequest, idx)
	if promptArtifact.Path != "" && promptArtifact.State != "" && !promptArtifact.Current {
		dispatchCommand = ""
	}
	item := ReviewerDispatchIntakeHandoff{
		PacketID:                                 packet.PacketID,
		PacketPath:                               packetPath,
		SummaryPath:                              packet.Observability.SummaryPath,
		ResultRoot:                               packet.ReviewerOrchestration.ResultRoot,
		TargetLane:                               targetLane,
		ShardID:                                  dispatch.ShardID,
		DispatchIndex:                            idx + 1,
		DispatchTotal:                            len(packet.ReviewerOrchestration.Dispatches),
		State:                                    state,
		ReviewerResultPath:                       resultPath,
		ReviewerResultPresent:                    present,
		ReviewerResultState:                      string(resultState),
		ReviewerResultSourcePath:                 sourcePath,
		ReviewerResultSourceState:                sourceState,
		ReviewerResultCandidatePath:              candidatePath,
		ReviewerResultCandidateState:             candidateState,
		DispatchPromptPath:                       promptArtifact.Path,
		DispatchPromptSHA256:                     promptArtifact.ExpectedSHA256,
		DispatchPromptState:                      promptArtifact.State,
		DispatchPromptCurrent:                    promptArtifact.Current,
		DispatchPromptActualSHA256:               promptArtifact.ActualSHA256,
		DispatchPromptFailure:                    promptArtifact.Failure,
		DispatchPromptRepairCommand:              reviewerDispatchPromptArtifactRepairCommand(packetPath, dispatch.ShardID, targetLane),
		AgentToolRequest:                         dispatch.AgentToolRequest,
		ReviewerResultStagingCommand:             stagingCommand,
		ReviewerResultCollectionCommands:         collectionCommands,
		ReviewerResultRecoveryCommand:            recoveryCommand,
		ReviewerResultRecoveryApplyCommand:       recoveryApplyCommand,
		ReviewerResultRecoveryDispositionCommand: recoveryDispositionCommand,
		ReviewerResultRecoveryDispositionPath:    recoveryDispositionPath,
		IntakeAvailable:                          intakeAvailable,
		DispatchOnly:                             !intakeAvailable,
		VerificationRecorded:                     verificationRecorded,
		DecisionRecorded:                         decisionRecorded,
		DispatchCommand:                          dispatchCommand,
		PreviewCommand:                           dispatch.PreviewCommand,
		ApplyCommand:                             dispatch.ApplyCommand,
		BatchPreviewCommand:                      packet.ReviewerOrchestration.BatchPreviewCommand,
		BatchApplyCommand:                        packet.ReviewerOrchestration.BatchApplyCommand,
		OwnerExecutor:                            packet.ReviewerOrchestration.OwnerBinding.CurrentExecutor,
		OwnerGeneration:                          packet.ReviewerOrchestration.OwnerBinding.ExecutorGeneration,
		OwnerBindingMode:                         packet.ReviewerOrchestration.OwnerBinding.BindingMode,
		CurrentExecutor:                          currentExecutor,
		CurrentGeneration:                        currentGeneration,
		OwnerAdoptionRequired:                    ownerStale && !adoptionCurrent,
		OwnerAdoptionPath:                        adoptionPath,
		OwnerAdoptionPreviewCommand:              reviewerDispatchAdoptionPreviewCommand(packetPath, targetLane),
	}
	if item.OwnerAdoptionRequired {
		item.BatchPreviewCommand = ""
		item.BatchApplyCommand = ""
		item.PreviewCommand = ""
		item.ApplyCommand = ""
	}
	item.RunbookSteps = reviewerDispatchIntakeRunbookSteps(item)
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
	if strings.TrimSpace(item.DispatchPromptPath) != "" {
		parts := []string{"reviewerPrompt"}
		if state := strings.TrimSpace(item.DispatchPromptState); state != "" {
			parts = append(parts, state)
		}
		if hash := strings.TrimSpace(item.DispatchPromptSHA256); hash != "" {
			parts = append(parts, "sha256="+hash)
		}
		if actual := strings.TrimSpace(item.DispatchPromptActualSHA256); actual != "" && !strings.EqualFold(actual, item.DispatchPromptSHA256) {
			parts = append(parts, "actualSha256="+actual)
		}
		parts = append(parts, reviewerDispatchDisplayPath(caseRoot, item.DispatchPromptPath))
		if failure := strings.TrimSpace(item.DispatchPromptFailure); failure != "" {
			parts = append(parts, "failure="+failure)
		}
		evidence = append(evidence, strings.Join(parts, " "))
	}
	if strings.TrimSpace(item.ReviewerResultSourcePath) != "" {
		evidence = append(evidence, "reviewerResultSource "+firstText(item.ReviewerResultSourceState, "missing")+" "+reviewerDispatchDisplayPath(caseRoot, item.ReviewerResultSourcePath))
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
			"reviewer returns one JSON object; the main agent saves it to reviewerResultSourcePath, runs staging -WhatIf, and uses its expected-source-hash -Apply command to publish the packet-derived candidate",
			"staging accepts only the packet-derived source path and collection publishes exact bytes without overwriting different candidates or canonical reviewer results",
			"collection publishes exact candidate bytes only to the immutable packet-derived reviewer result path",
		)
	} else if item.IntakeAvailable {
		boundary = append(boundary, "this packet has no canonical collection capability; save reviewer JSON directly to reviewerResultPath and use strict direct or batch intake")
	}
	if item.State == "reviewer-dispatch-prompt-artifact-invalid" || item.State == "reviewer-dispatch-prompt-artifact-drift" {
		boundary = append(boundary, "reviewer prompt artifact must be present, non-symlink, non-empty, and match dispatchPromptSha256 before dispatching a reviewer")
		boundary = append(boundary, "prompt artifact repair is hash-gated, packet-derived, and only creates a missing artifact or accepts exact replay; it never overwrites drifted or invalid existing artifacts")
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
		"dispatch reviewers only from prompt artifacts that are present, non-symlink, non-empty, and match promptSha256",
		"run ready-result batch intake -WhatIf before -Apply; packet order, fail-fast, waiting results, no-authority, and no-heavy boundaries remain enforced",
	}
}

func reviewerDispatchIntakeRunbookSteps(item ReviewerDispatchIntakeHandoff) []string {
	steps := []string{}
	add := func(step string) {
		step = strings.TrimSpace(step)
		if step != "" {
			steps = append(steps, step)
		}
	}
	add("work from this first-screen handoff; open packet.json only if the command preview asks for full packet details")
	switch item.State {
	case "reviewer-packet-owner-adoption-required":
		add("run owner adoption preview: " + firstText(item.OwnerAdoptionPreviewCommand, "<owner-adoption-WhatIf unavailable>"))
		add("review adoptedOwner/currentExecutor/currentGeneration and boundary, then rerun the returned or same command with -Apply if it is still current")
		add("rerun /rekit status or /rekit continue " + firstText(item.TargetLane, "<lane>") + " -WhatIf to resume reviewer dispatch/intake")
	case "reviewer-packet-integrity-invalid":
		add("regenerate the canonical reviewer packet and packet.integrity.json together; do not repair packet bytes or integrity metadata independently")
		add("rerun /rekit status -Format text and use the refreshed reviewer dispatch intake handoff")
	case "reviewer-dispatch-prompt-artifact-invalid", "reviewer-dispatch-prompt-artifact-drift":
		add("run prompt artifact repair preview: " + firstText(item.DispatchPromptRepairCommand, "<prompt-artifact-repair-WhatIf unavailable>"))
		add("if preview is valid and current, run its bounded hash-gated apply command; do not dispatch reviewer while promptSha256 is missing or drifted")
		add("rerun status or continue to verify promptCurrent=true before dispatching a reviewer")
	case "reviewer-result-recovery-required":
		add("run reviewer result recovery preview: " + firstText(item.ReviewerResultRecoveryCommand, "<reviewer-result-recovery-WhatIf unavailable>"))
		add("review the quarantine/recovery intent, then rerun status to obtain finalize guidance")
	case "reviewer-result-recovery-finalize-required":
		add("finalize reviewer result recovery with the hash-bound apply command: " + firstText(item.ReviewerResultRecoveryApplyCommand, "<reviewer-result-recovery-Apply unavailable>"))
		add("rerun status or continue and proceed to collection/intake only after recovery is recorded")
	case "reviewer-result-recovery-invalid":
		add("repair or regenerate the strict reviewer result recovery intent before collection or intake")
		add("rerun reviewer result recovery -WhatIf after the invalid recovery artifact is removed or corrected")
	case "reviewer-result-recovery-ambiguous":
		add("review the canonical reviewer result and quarantine intent; if they are the same intended result, run disposition preview: " + firstText(item.ReviewerResultRecoveryDispositionCommand, "<reviewer-result-recovery-disposition-WhatIf unavailable>"))
		add("run only the bounded disposition apply returned by preview; do not overwrite canonical reviewer results manually")
	case "ready-for-reviewer-result-staging-preview":
		add("reviewer result source is ready at " + firstText(item.ReviewerResultSourcePath, "<reviewer-result-source-path>"))
		add("run staging preview: " + firstText(item.ReviewerResultStagingCommand, "<reviewer-result-staging-WhatIf unavailable>"))
		add("if preview reports the expected source hash, rerun the returned command with -ExpectedSourceSha256 and -Apply to publish the packet-derived candidate")
		if item.ReviewerResultCollectionCommands != nil {
			add("then run collection preview before apply: " + item.ReviewerResultCollectionCommands.PreviewCommand)
		}
	case "ready-for-reviewer-result-collection-preview":
		if item.ReviewerResultCollectionCommands != nil {
			add("run reviewer result collection preview: " + item.ReviewerResultCollectionCommands.PreviewCommand)
			add("if candidate bytes match the packet-derived result, run collection apply: " + item.ReviewerResultCollectionCommands.ApplyCommand)
		} else {
			add("run reviewer result collection -WhatIf for " + item.ShardID + " before reviewer intake")
		}
		add("rerun status or continue; next handoff should become ready-for-reviewer-intake-preview")
	case "ready-for-reviewer-intake-preview":
		add("run reviewer intake preview: " + firstText(item.BatchPreviewCommand, item.PreviewCommand, "<reviewer-intake-WhatIf unavailable>"))
		add("inspect verification, decision, postValidation, and action queue from preview; do not hand-write reviewer ledger events")
		add("if preview remains valid, run the bounded apply command: " + firstText(item.BatchApplyCommand, item.ApplyCommand, "<reviewer-intake-Apply unavailable>"))
	case "attach-required-before-reviewer-intake":
		add("attach or init the target as a rekit case before reviewer intake writeback")
		add("rerun status after attach and use the refreshed reviewer dispatch intake handoff")
	case "reviewer-result-symlink-blocked":
		if item.ReviewerResultRecoveryCommand != "" {
			add("run reviewer result recovery preview: " + item.ReviewerResultRecoveryCommand)
		} else {
			add("replace reviewer result symlink with a regular non-empty reviewer JSON file at " + firstText(item.ReviewerResultPath, "<reviewer-result-path>"))
		}
		add("rerun status before any reviewer intake apply")
	case "reviewer-result-canonical-invalid":
		if item.ReviewerResultRecoveryCommand != "" {
			add("run reviewer result recovery preview: " + item.ReviewerResultRecoveryCommand)
		} else {
			add("repair the canonical reviewer result so it is a non-empty regular file at " + firstText(item.ReviewerResultPath, "<reviewer-result-path>"))
		}
		add("rerun status before collection or intake")
	case "reviewer-result-source-invalid":
		add("replace the invalid reviewer result source with exactly one reviewer JSON object at " + firstText(item.ReviewerResultSourcePath, "<reviewer-result-source-path>"))
		add("rerun staging preview: " + firstText(item.ReviewerResultStagingCommand, "<reviewer-result-staging-WhatIf unavailable>"))
	case "reviewer-result-candidate-invalid":
		add("remove or replace the invalid packet-derived reviewer result candidate at " + firstText(item.ReviewerResultCandidatePath, "<reviewer-result-candidate-path>"))
		if item.ReviewerResultCollectionCommands != nil {
			add("rerun collection preview only after staging republishes a valid candidate: " + item.ReviewerResultCollectionCommands.PreviewCommand)
		}
	case "reviewer-result-collection-required":
		add("publish the packet-derived candidate through staging and collection before reviewer intake")
		if item.ReviewerResultStagingCommand != "" {
			add("staging preview: " + item.ReviewerResultStagingCommand)
		}
		if item.ReviewerResultCollectionCommands != nil {
			add("collection preview: " + item.ReviewerResultCollectionCommands.PreviewCommand)
		}
	default:
		if command := strings.TrimSpace(item.DispatchCommand); command != "" {
			add(command)
		} else {
			add("collect read-only reviewer JSON for " + item.ShardID + " at " + firstText(item.ReviewerResultPath, "<reviewer-result-path>"))
		}
		if item.ReviewerResultStagingCommand != "" {
			add("after saving reviewer JSON, run staging preview: " + item.ReviewerResultStagingCommand)
		}
	}
	add("do not continue the lane until reviewer dispatch/intake handoff total is zero or the current packet action is resolved")
	return mission.UniqueStrings(steps)
}

func reviewerDispatchIntakeNextAction(item ReviewerDispatchIntakeHandoff) string {
	switch item.State {
	case "reviewer-packet-owner-adoption-required":
		return firstText(item.OwnerAdoptionPreviewCommand, "adopt reviewer packet "+item.PacketID+" before intake")
	case "reviewer-packet-integrity-invalid":
		return "regenerate canonical reviewer packet at " + item.PacketPath + "; do not continue from invalid packet integrity"
	case "reviewer-dispatch-prompt-artifact-invalid", "reviewer-dispatch-prompt-artifact-drift":
		if command := strings.TrimSpace(item.DispatchPromptRepairCommand); command != "" {
			return command
		}
		failure := strings.TrimSpace(item.DispatchPromptFailure)
		if failure != "" {
			failure = ": " + failure
		}
		return "regenerate or restore reviewer prompt artifact for " + item.ShardID + " at " + item.DispatchPromptPath + failure + "; do not dispatch reviewer until promptSha256 matches"
	case "reviewer-result-recovery-required":
		return firstText(item.ReviewerResultRecoveryCommand, "run reviewer result recovery -WhatIf for "+item.ShardID)
	case "reviewer-result-recovery-finalize-required":
		return firstText(item.ReviewerResultRecoveryApplyCommand, item.ReviewerResultRecoveryCommand, "finalize reviewer result recovery for "+item.ShardID)
	case "reviewer-result-recovery-invalid":
		return "repair or regenerate the strict reviewer result recovery intent for " + item.ShardID + "; collection remains blocked"
	case "reviewer-result-recovery-ambiguous":
		return firstText(item.ReviewerResultRecoveryDispositionCommand, "review the canonical reviewer result and exact quarantine for "+item.ShardID+"; runtime cannot prove they are the same filesystem object")
	case "ready-for-reviewer-result-staging-preview":
		return firstText(item.ReviewerResultStagingCommand, "run reviewer result staging -WhatIf for "+item.ShardID)
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
		if item.ReviewerResultRecoveryCommand != "" {
			return item.ReviewerResultRecoveryCommand
		}
		return "replace the symlink at " + item.ReviewerResultPath + " with a regular reviewer result before intake"
	case "reviewer-result-canonical-invalid":
		return firstText(item.ReviewerResultRecoveryCommand, "inspect the non-empty or unreadable canonical reviewer result obstruction for "+item.ShardID+"; automatic recovery remains blocked")
	case "reviewer-result-source-invalid":
		return "replace the invalid reviewer result source for " + item.ShardID + " at " + item.ReviewerResultSourcePath + ", then rerun staging -WhatIf"
	case "reviewer-result-candidate-invalid":
		return "replace the invalid reviewer result candidate for " + item.ShardID + " at " + item.ReviewerResultCandidatePath + ", then rerun collection -WhatIf"
	case "reviewer-result-collection-required":
		return "publish the packet-derived reviewer result candidate for " + item.ShardID + " via staging and collection before reviewer intake"
	default:
		if item.AgentToolRequest != nil && strings.TrimSpace(item.ReviewerResultCandidatePath) != "" {
			return item.DispatchCommand
		}
		return "collect read-only reviewer JSON for " + item.ShardID + " at " + item.ReviewerResultPath
	}
}

func reviewerDispatchCommand(shardID, stagingCommand, sourcePath, candidatePath, reviewerResultPath, promptPath, promptSHA256 string, request *ReviewerAgentToolRequest, idx int) string {
	promptRef := reviewerDispatchPromptArtifactRef(promptPath, promptSHA256, request, idx)
	if request != nil && strings.TrimSpace(candidatePath) != "" && strings.TrimSpace(stagingCommand) != "" {
		return "dispatch read-only reviewer for " + shardID + " using " + promptRef + "; save exactly one JSON object to " + reviewerDispatchQuoteCommandArg(sourcePath) + ", run staging preview " + reviewerDispatchQuoteCommandArg(stagingCommand) + ", use its expected-hash Apply command to publish " + reviewerDispatchQuoteCommandArg(candidatePath) + ", then run reviewer result collection WhatIf before Apply"
	}
	return "dispatch read-only reviewer for " + shardID + " using " + promptRef + "; collect JSON at " + reviewerDispatchQuoteCommandArg(reviewerResultPath)
}

func reviewerDispatchPromptArtifactRef(promptPath, promptSHA256 string, request *ReviewerAgentToolRequest, idx int) string {
	promptPath = strings.TrimSpace(promptPath)
	promptSHA256 = strings.TrimSpace(promptSHA256)
	if request != nil {
		if promptPath == "" {
			promptPath = strings.TrimSpace(request.PromptPath)
		}
		if promptSHA256 == "" {
			promptSHA256 = strings.TrimSpace(request.PromptSHA256)
		}
	}
	if promptPath != "" {
		ref := "prompt artifact " + reviewerDispatchQuoteCommandArg(promptPath)
		if promptSHA256 != "" {
			ref += " (sha256=" + promptSHA256 + ")"
		}
		return ref
	}
	if promptSHA256 != "" {
		return "reviewerOrchestration.dispatches[" + strconv.Itoa(idx) + "].dispatchPromptPath (sha256=" + promptSHA256 + ")"
	}
	if request != nil {
		return "reviewerOrchestration.dispatches[" + strconv.Itoa(idx) + "].agentToolRequest.prompt"
	}
	return "reviewerOrchestration.dispatches[" + strconv.Itoa(idx) + "].dispatchPrompt"
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
	lines = append(lines, fmt.Sprintf("- summary: total=%d waitingForReviewerResult=%d readyForPreview=%d attachRequired=%d dispatchOnly=%d promptArtifactBlocked=%d packets=%d latestPacketProgress=%d/%d open=%d nextOpen=%s remaining=%s latestShard=%s latestState=%s sourceState=%s candidateState=%s nextAction=`%s`", summary.Total, summary.WaitingForReviewerResult, summary.ReadyForPreview, summary.AttachRequired, summary.DispatchOnly, summary.PromptArtifactBlocked, summary.PacketCount, summary.LatestPacketDispatchCompleted, summary.LatestPacketDispatchTotal, summary.LatestPacketDispatchOpen, summary.LatestPacketNextOpenShardID, strings.Join(summary.RemainingShardIDs, ","), summary.LatestShardID, summary.LatestState, summary.LatestReviewerResultSourceState, summary.LatestReviewerResultCandidateState, summary.NextAction))
	for _, item := range items {
		lines = append(lines, fmt.Sprintf("- dispatch intake: lane=%s shard=%s state=%s progress=%d/%d open=%d nextOpen=%s remaining=%s sourceState=%s source=`%s` candidateState=%s candidate=`%s` resultPresent=%t resultState=%s packet=`%s` reviewerResult=`%s` preview=`%s` apply=`%s` batchPreview=`%s` batchApply=`%s`", item.TargetLane, item.ShardID, item.State, item.DispatchCompleted, item.DispatchTotal, item.DispatchOpen, item.NextOpenShardID, strings.Join(item.RemainingShardIDs, ","), item.ReviewerResultSourceState, item.ReviewerResultSourcePath, item.ReviewerResultCandidateState, item.ReviewerResultCandidatePath, item.ReviewerResultPresent, item.ReviewerResultState, item.PacketPath, item.ReviewerResultPath, item.PreviewCommand, item.ApplyCommand, item.BatchPreviewCommand, item.BatchApplyCommand))
		if strings.TrimSpace(item.DispatchPromptPath) != "" {
			lines = append(lines, fmt.Sprintf("  - prompt artifact: path=`%s` sha256=%s state=%s current=%t actualSha256=%s failure=%s", item.DispatchPromptPath, item.DispatchPromptSHA256, item.DispatchPromptState, item.DispatchPromptCurrent, item.DispatchPromptActualSHA256, item.DispatchPromptFailure))
		}
		if item.AgentToolRequest != nil {
			lines = append(lines, fmt.Sprintf("  - agent tool: tool=%s agentType=%s readOnly=%t promptPath=`%s` promptSha256=%s expectedOutput=%s", item.AgentToolRequest.Tool, item.AgentToolRequest.AgentType, item.AgentToolRequest.ReadOnly, item.AgentToolRequest.PromptPath, item.AgentToolRequest.PromptSHA256, item.AgentToolRequest.ExpectedOutput))
		}
		if item.ReviewerResultStagingCommand != "" {
			lines = append(lines, fmt.Sprintf("  - staging: source=`%s` state=%s preview=`%s`", item.ReviewerResultSourcePath, item.ReviewerResultSourceState, item.ReviewerResultStagingCommand))
		}
		if item.ReviewerResultCollectionCommands != nil {
			lines = append(lines, fmt.Sprintf("  - collection: preview=`%s` apply=`%s`", item.ReviewerResultCollectionCommands.PreviewCommand, item.ReviewerResultCollectionCommands.ApplyCommand))
		}
		for idx, step := range mission.LimitStrings(item.RunbookSteps, maxHandoffRows) {
			lines = append(lines, fmt.Sprintf("  - runbook step %d: %s", idx+1, step))
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
	fmt.Fprintf(out, "- summary: total=%d waitingForReviewerResult=%d readyForPreview=%d attachRequired=%d dispatchOnly=%d promptArtifactBlocked=%d packets=%d latestPacketProgress=%d/%d open=%d nextOpen=%s remaining=%s latestShard=%s latestState=%s sourceState=%s candidateState=%s nextAction=`%s`\n", summary.Total, summary.WaitingForReviewerResult, summary.ReadyForPreview, summary.AttachRequired, summary.DispatchOnly, summary.PromptArtifactBlocked, summary.PacketCount, summary.LatestPacketDispatchCompleted, summary.LatestPacketDispatchTotal, summary.LatestPacketDispatchOpen, summary.LatestPacketNextOpenShardID, strings.Join(summary.RemainingShardIDs, ","), summary.LatestShardID, summary.LatestState, summary.LatestReviewerResultSourceState, summary.LatestReviewerResultCandidateState, summary.NextAction)
	for _, item := range items {
		fmt.Fprintf(out, "- dispatch intake: lane=%s shard=%s state=%s progress=%d/%d open=%d nextOpen=%s remaining=%s sourceState=%s source=`%s` candidateState=%s candidate=`%s` resultPresent=%t packet=`%s` reviewerResult=`%s` preview=`%s` apply=`%s`\n", item.TargetLane, item.ShardID, item.State, item.DispatchCompleted, item.DispatchTotal, item.DispatchOpen, item.NextOpenShardID, strings.Join(item.RemainingShardIDs, ","), item.ReviewerResultSourceState, item.ReviewerResultSourcePath, item.ReviewerResultCandidateState, item.ReviewerResultCandidatePath, item.ReviewerResultPresent, item.PacketPath, item.ReviewerResultPath, item.PreviewCommand, item.ApplyCommand)
		if strings.TrimSpace(item.DispatchPromptPath) != "" {
			fmt.Fprintf(out, "  - prompt artifact: path=`%s` sha256=%s state=%s current=%t actualSha256=%s failure=%s\n", item.DispatchPromptPath, item.DispatchPromptSHA256, item.DispatchPromptState, item.DispatchPromptCurrent, item.DispatchPromptActualSHA256, item.DispatchPromptFailure)
		}
		if item.AgentToolRequest != nil {
			fmt.Fprintf(out, "  - agent tool: tool=%s agentType=%s readOnly=%t promptPath=`%s` promptSha256=%s expectedOutput=%s\n", item.AgentToolRequest.Tool, item.AgentToolRequest.AgentType, item.AgentToolRequest.ReadOnly, item.AgentToolRequest.PromptPath, item.AgentToolRequest.PromptSHA256, item.AgentToolRequest.ExpectedOutput)
		}
		if item.ReviewerResultStagingCommand != "" {
			fmt.Fprintf(out, "  - staging: source=`%s` state=%s preview=`%s`\n", item.ReviewerResultSourcePath, item.ReviewerResultSourceState, item.ReviewerResultStagingCommand)
		}
		if item.ReviewerResultCollectionCommands != nil {
			fmt.Fprintf(out, "  - collection: preview=`%s` apply=`%s`\n", item.ReviewerResultCollectionCommands.PreviewCommand, item.ReviewerResultCollectionCommands.ApplyCommand)
		}
		for idx, step := range mission.LimitStrings(item.RunbookSteps, maxHandoffRows) {
			fmt.Fprintf(out, "  - runbook step %d: %s\n", idx+1, step)
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
