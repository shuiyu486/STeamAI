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
	"runtime"
	"slices"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/shuiyu486/re-context-kits/internal/rekit/doctor"
	refsf "github.com/shuiyu486/re-context-kits/internal/rekit/fs"
	"github.com/shuiyu486/re-context-kits/internal/rekit/instance"
	"github.com/shuiyu486/re-context-kits/internal/rekit/manifest"
	"github.com/shuiyu486/re-context-kits/internal/rekit/mission"
	"github.com/shuiyu486/re-context-kits/internal/rekit/note"
	"github.com/shuiyu486/re-context-kits/internal/rekit/overview"
	"github.com/shuiyu486/re-context-kits/internal/rekit/reviewerresult"
	"github.com/shuiyu486/re-context-kits/internal/rekit/reviewersession"
	"github.com/shuiyu486/re-context-kits/internal/rekit/reviewpath"
	"github.com/shuiyu486/re-context-kits/internal/rekit/workstream"
)

const (
	maxReviewerResultBytes          = 64 * 1024
	maxReviewPacketBytes            = 1024 * 1024
	maxReviewerPacketAdoptionBytes  = 32 * 1024
	reviewerPacketAdoptionDirectory = ".rekit/reviewer-adoptions"
)

var appendReviewerNote = note.Append

type ReviewerResult = reviewerresult.Result

type ReviewerIntakeOptions struct {
	PacketPath         string
	ReviewerResultPath string
	ExpectedShardID    string
	Lane               string
	Actor              string
	WhatIf             bool
}

type ReviewerBatchIntakeOptions struct {
	PacketPath string
	Lane       string
	Actor      string
	WhatIf     bool
}

type ReviewerPacketAdoptionOptions struct {
	PacketPath string
	Lane       string
	Actor      string
	Reason     string
	WhatIf     bool
}

type ReviewerPacketAdoption struct {
	SchemaVersion          int          `json:"schemaVersion"`
	Kind                   string       `json:"kind"`
	PacketID               string       `json:"packetId"`
	PacketPath             string       `json:"packetPath"`
	PacketSHA256           string       `json:"packetSha256"`
	RepoRoot               string       `json:"repoRoot"`
	CaseRoot               string       `json:"caseRoot"`
	Pack                   string       `json:"pack"`
	Lane                   string       `json:"lane"`
	DispatchedOwner        OwnerBinding `json:"dispatchedOwner"`
	AdoptedOwner           OwnerBinding `json:"adoptedOwner"`
	Actor                  string       `json:"actor"`
	Reason                 string       `json:"reason"`
	CreatedAt              string       `json:"createdAt"`
	NoSpawn                bool         `json:"noSpawn"`
	NoHeavyTool            bool         `json:"noHeavyTool"`
	NoAuthorityOrConfirmed bool         `json:"noAuthorityOrConfirmed"`
}

type ReviewerPacketAdoptionResult struct {
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
	AdoptionPath                string                                   `json:"adoptionPath"`
	Lane                        string                                   `json:"lane"`
	Actor                       string                                   `json:"actor"`
	Reason                      string                                   `json:"reason"`
	DispatchedOwner             OwnerBinding                             `json:"dispatchedOwner"`
	AdoptedOwner                OwnerBinding                             `json:"adoptedOwner"`
	MissionCommanderAction      mission.MissionCommanderAction           `json:"missionCommanderAction"`
	MissionCommanderNextActions []mission.MissionCommanderNextActionItem `json:"missionCommanderNextActions"`
	MissionCommanderActionQueue mission.MissionCommanderActionQueue      `json:"missionCommanderActionQueue"`
	NextSteps                   []string                                 `json:"nextSteps"`
	Boundary                    []string                                 `json:"boundary"`
}

type ReviewerBatchIntakeResult struct {
	SchemaVersion                 int                                       `json:"schemaVersion"`
	Command                       string                                    `json:"command"`
	Mode                          string                                    `json:"mode"`
	CaseRoot                      string                                    `json:"caseRoot"`
	RepoRoot                      string                                    `json:"repoRoot"`
	Pack                          string                                    `json:"pack"`
	IsMutation                    bool                                      `json:"isMutation"`
	Applied                       bool                                      `json:"applied"`
	PacketPath                    string                                    `json:"packetPath"`
	Lane                          string                                    `json:"lane"`
	Actor                         string                                    `json:"actor"`
	Total                         int                                       `json:"total"`
	Ready                         int                                       `json:"ready"`
	Waiting                       int                                       `json:"waiting"`
	Processed                     int                                       `json:"processed"`
	Completed                     int                                       `json:"completed"`
	AlreadyComplete               int                                       `json:"alreadyComplete"`
	Stopped                       bool                                      `json:"stopped"`
	StopShardID                   string                                    `json:"stopShardId,omitempty"`
	StopReason                    string                                    `json:"stopReason,omitempty"`
	Partial                       bool                                      `json:"partial"`
	NextOpenShardID               string                                    `json:"nextOpenShardId,omitempty"`
	RemainingShardIDs             []string                                  `json:"remainingShardIds,omitempty"`
	RerunCommand                  string                                    `json:"rerunCommand,omitempty"`
	RecoveryAction                *mission.MissionCommanderNextActionItem   `json:"recoveryAction,omitempty"`
	Results                       []ReviewerIntakeResult                    `json:"results"`
	NextSteps                     []string                                  `json:"nextSteps"`
	MissionCommanderAction        mission.MissionCommanderAction            `json:"missionCommanderAction"`
	MissionCommanderNextActions   []mission.MissionCommanderNextActionItem  `json:"missionCommanderNextActions,omitempty"`
	MissionCommanderActionQueue   mission.MissionCommanderActionQueue       `json:"missionCommanderActionQueue"`
	MissionCommanderDriverReceipt *workstream.MissionCommanderDriverReceipt `json:"missionCommanderDriverReceipt,omitempty"`
	Boundary                      []string                                  `json:"boundary"`
}

type ReviewerIntakeResult struct {
	SchemaVersion               int                                      `json:"schemaVersion"`
	Command                     string                                   `json:"command"`
	Mode                        string                                   `json:"mode"`
	CaseRoot                    string                                   `json:"caseRoot"`
	RepoRoot                    string                                   `json:"repoRoot"`
	Pack                        string                                   `json:"pack"`
	IsMutation                  bool                                     `json:"isMutation"`
	Applied                     bool                                     `json:"applied"`
	WritebackStatus             string                                   `json:"writebackStatus"`
	ReadyForWriteback           bool                                     `json:"readyForWriteback"`
	ReviewRequired              bool                                     `json:"reviewRequired"`
	PacketPath                  string                                   `json:"packetPath"`
	ReviewerResultPath          string                                   `json:"reviewerResultPath"`
	IntakeID                    string                                   `json:"intakeId"`
	Lane                        string                                   `json:"lane"`
	ShardID                     string                                   `json:"shardId"`
	Actor                       string                                   `json:"actor"`
	OwnerBinding                OwnerBinding                             `json:"ownerBinding"`
	ReviewerSession             string                                   `json:"reviewerSession"`
	ReviewerResult              ReviewerResult                           `json:"reviewerResult"`
	VerificationVerdict         string                                   `json:"verificationVerdict"`
	MainDecision                string                                   `json:"mainDecision"`
	BlockedReasons              []string                                 `json:"blockedReasons"`
	RepairGuidance              []ReviewerIntakeRepairGuidance           `json:"repairGuidance,omitempty"`
	OrchestrationSnapshot       ReviewerOrchestrationIntake              `json:"orchestrationSnapshot"`
	Verification                *note.AppendResult                       `json:"verification,omitempty"`
	Decision                    *note.AppendResult                       `json:"decision,omitempty"`
	PostValidation              *ReviewerPostValidation                  `json:"postValidation,omitempty"`
	MissionCommanderAction      mission.MissionCommanderAction           `json:"missionCommanderAction"`
	MissionCommanderNextActions []mission.MissionCommanderNextActionItem `json:"missionCommanderNextActions,omitempty"`
	MissionCommanderActionQueue mission.MissionCommanderActionQueue      `json:"missionCommanderActionQueue"`
	NextSteps                   []string                                 `json:"nextSteps"`
	Summary                     ReviewerIntakeSummary                    `json:"summary"`
}

type reviewerSessionWritebackProvenance struct {
	ReviewerHarness           string
	DispatchID                string
	DispatchPath              string
	DispatchSHA256            string
	CompletionPath            string
	CompletionSHA256          string
	ReviewerResultInputPath   string
	ReviewerResultInputSHA256 string
}

type ReviewerIntakeSummary struct {
	Status                              string                               `json:"status"`
	ReadyForWriteback                   bool                                 `json:"readyForWriteback"`
	Applied                             bool                                 `json:"applied"`
	Lane                                string                               `json:"lane,omitempty"`
	ShardID                             string                               `json:"shardId,omitempty"`
	IntakeID                            string                               `json:"intakeId,omitempty"`
	ReviewerSession                     string                               `json:"reviewerSession,omitempty"`
	VerificationVerdict                 string                               `json:"verificationVerdict,omitempty"`
	MainDecision                        string                               `json:"mainDecision,omitempty"`
	DispatchIndex                       int                                  `json:"dispatchIndex"`
	DispatchTotal                       int                                  `json:"dispatchTotal"`
	ShardStatusBefore                   string                               `json:"shardStatusBefore,omitempty"`
	ShardStatusAfter                    string                               `json:"shardStatusAfter,omitempty"`
	NextDispatches                      []string                             `json:"nextDispatches,omitempty"`
	OrchestrationProgress               *ReviewerIntakeOrchestrationProgress `json:"orchestrationProgress,omitempty"`
	BlockedCount                        int                                  `json:"blockedCount"`
	RepairGuidanceCount                 int                                  `json:"repairGuidanceCount"`
	RepairGuidanceSummary               *ReviewerIntakeRepairGuidanceSummary `json:"repairGuidanceSummary,omitempty"`
	PostValidationPresent               bool                                 `json:"postValidationPresent"`
	PostValidationValid                 bool                                 `json:"postValidationValid"`
	PostValidationOverviewVerifications int                                  `json:"postValidationOverviewVerifications"`
	PostValidationOverviewDecisions     int                                  `json:"postValidationOverviewDecisions"`
	ReviewerWritebacks                  int                                  `json:"reviewerWritebacks"`
	ReviewerWritebackSummary            *workstream.ReviewerWritebackSummary `json:"reviewerWritebackSummary,omitempty"`
	ActionTotal                         int                                  `json:"actionTotal"`
	ActionUnblocked                     int                                  `json:"actionUnblocked"`
	ActionBlocked                       int                                  `json:"actionBlocked"`
	ActionRequiresReview                int                                  `json:"actionRequiresReview"`
	ActionFollowUp                      int                                  `json:"actionFollowUp"`
	QueueSummary                        string                               `json:"queueSummary,omitempty"`
	CurrentAction                       *ReviewerIntakeNextActionSummary     `json:"currentAction,omitempty"`
	NextActions                         []ReviewerIntakeNextActionSummary    `json:"nextActions,omitempty"`
	Boundary                            []string                             `json:"boundary,omitempty"`
}

type ReviewerIntakeNextActionSummary struct {
	Lane           string   `json:"lane,omitempty"`
	Label          string   `json:"label,omitempty"`
	GateEventID    string   `json:"gateEventId,omitempty"`
	ActionID       string   `json:"actionId,omitempty"`
	State          string   `json:"state"`
	Source         string   `json:"source"`
	Command        string   `json:"command"`
	Blocked        bool     `json:"blocked,omitempty"`
	RequiresReview bool     `json:"requiresReview,omitempty"`
	Reasons        []string `json:"reasons,omitempty"`
	Boundary       []string `json:"boundary,omitempty"`
}

type ReviewerIntakeOrchestrationProgress struct {
	DispatchIndex      int      `json:"dispatchIndex"`
	DispatchTotal      int      `json:"dispatchTotal"`
	Completed          int      `json:"completed"`
	Open               int      `json:"open"`
	CurrentShardID     string   `json:"currentShardId,omitempty"`
	CurrentShardStatus string   `json:"currentShardStatus,omitempty"`
	NextOpenShardID    string   `json:"nextOpenShardId,omitempty"`
	RemainingShardIDs  []string `json:"remainingShardIds,omitempty"`
	Boundary           []string `json:"boundary,omitempty"`
}

type ReviewerIntakeRepairGuidance struct {
	Reason   string   `json:"reason"`
	Action   string   `json:"action"`
	Evidence []string `json:"evidence,omitempty"`
	Boundary []string `json:"boundary,omitempty"`
}

type ReviewerIntakeRepairGuidanceSummary struct {
	Total           int      `json:"total"`
	PrimaryReason   string   `json:"primaryReason,omitempty"`
	PrimaryAction   string   `json:"primaryAction,omitempty"`
	Evidence        []string `json:"evidence,omitempty"`
	Boundary        []string `json:"boundary,omitempty"`
	NextSafeCommand string   `json:"nextSafeCommand,omitempty"`
}

type ReviewerPostValidation struct {
	Overview   overview.Inventory            `json:"overview"`
	Handoff    workstream.HandoffResult      `json:"handoff"`
	DoctorRows []doctor.Row                  `json:"doctorRows"`
	Valid      bool                          `json:"valid"`
	Summary    ReviewerPostValidationSummary `json:"summary"`
}

type ReviewerPostValidationSummary struct {
	Valid                    bool                                      `json:"valid"`
	OverviewVerifications    int                                       `json:"overviewVerifications"`
	OverviewDecisions        int                                       `json:"overviewDecisions"`
	DoctorRows               int                                       `json:"doctorRows"`
	Lane                     string                                    `json:"lane,omitempty"`
	Project                  bool                                      `json:"project"`
	ExecutorActionPresent    bool                                      `json:"executorActionPresent"`
	ExecutorActionReady      bool                                      `json:"executorActionReady"`
	ExecutorActionBlocked    bool                                      `json:"executorActionBlocked"`
	ExecutorActionState      string                                    `json:"executorActionState,omitempty"`
	ReviewerWritebacks       int                                       `json:"reviewerWritebacks"`
	ReviewerWritebackSummary *workstream.ReviewerWritebackSummary      `json:"reviewerWritebackSummary,omitempty"`
	QueueSummary             string                                    `json:"queueSummary,omitempty"`
	CurrentAction            *ReviewerPostValidationNextActionSummary  `json:"currentAction,omitempty"`
	NextActions              []ReviewerPostValidationNextActionSummary `json:"nextActions,omitempty"`
	Boundary                 []string                                  `json:"boundary,omitempty"`
}

type ReviewerPostValidationNextActionSummary struct {
	Lane           string   `json:"lane,omitempty"`
	Label          string   `json:"label,omitempty"`
	GateEventID    string   `json:"gateEventId,omitempty"`
	ActionID       string   `json:"actionId,omitempty"`
	State          string   `json:"state"`
	Source         string   `json:"source"`
	Command        string   `json:"command"`
	Blocked        bool     `json:"blocked,omitempty"`
	RequiresReview bool     `json:"requiresReview,omitempty"`
	Reasons        []string `json:"reasons,omitempty"`
	Boundary       []string `json:"boundary,omitempty"`
}

type ReviewerOrchestrationIntake struct {
	Mode                 string   `json:"mode"`
	PacketPath           string   `json:"packetPath"`
	ResultRoot           string   `json:"resultRoot"`
	ReviewerCount        int      `json:"reviewerCount"`
	DispatchIndex        int      `json:"dispatchIndex"`
	DispatchTotal        int      `json:"dispatchTotal"`
	ShardStatusBefore    string   `json:"shardStatusBefore"`
	ShardStatusAfter     string   `json:"shardStatusAfter"`
	PreviewRequiredFirst bool     `json:"previewRequiredFirst"`
	MainAgentOwns        []string `json:"mainAgentOwns"`
	RuntimeBoundary      []string `json:"runtimeBoundary"`
	NextDispatches       []string `json:"nextDispatches"`
}

func AdoptReviewerPacket(repoRoot, caseRoot, pack string, opt ReviewerPacketAdoptionOptions) (ReviewerPacketAdoptionResult, error) {
	inst, err := instance.AssertAttached(caseRoot, repoRoot, pack)
	if err != nil {
		return ReviewerPacketAdoptionResult{}, err
	}
	caseRoot = inst.CaseRoot
	packetPath, err := requiredAbsolutePath(opt.PacketPath, "review packet")
	if err != nil {
		return ReviewerPacketAdoptionResult{}, err
	}
	packetBytes, err := readStableReviewerArtifact(filepath.Dir(packetPath), packetPath, "review packet", maxReviewPacketBytes)
	if err != nil {
		return ReviewerPacketAdoptionResult{}, err
	}
	packet, err := decodeIntakePacket(packetBytes)
	if err != nil {
		return ReviewerPacketAdoptionResult{}, err
	}
	if err := validateIntakePacket(packet, packetPath, repoRoot, caseRoot, pack); err != nil {
		return ReviewerPacketAdoptionResult{}, err
	}
	if err := validatePacketIntegrity(caseRoot, packetPath, packet, packetBytes); err != nil {
		return ReviewerPacketAdoptionResult{}, err
	}
	if err := validateIntakePacketRoute(repoRoot, pack, packet); err != nil {
		return ReviewerPacketAdoptionResult{}, err
	}
	lane := strings.TrimSpace(opt.Lane)
	if lane == "" || lane != packet.TargetLane {
		return ReviewerPacketAdoptionResult{}, fmt.Errorf("reviewer packet adoption lane %q does not match packet targetLane %q", lane, packet.TargetLane)
	}
	actor := strings.TrimSpace(opt.Actor)
	reason := strings.TrimSpace(opt.Reason)
	if actor == "" || reason == "" {
		return ReviewerPacketAdoptionResult{}, fmt.Errorf("reviewer packet adoption requires -Actor and -Reason")
	}
	board, err := mission.ReadBoard(caseRoot)
	if err != nil {
		return ReviewerPacketAdoptionResult{}, fmt.Errorf("read board for reviewer packet adoption: %w", err)
	}
	current, ok := mission.LookupBoardLane(board.Lanes, lane, false)
	if !ok || strings.TrimSpace(current.CurrentExecutor) == "" {
		return ReviewerPacketAdoptionResult{}, fmt.Errorf("reviewer packet adoption target lane %q has no current executor", lane)
	}
	adoptedOwner := OwnerBinding{
		TargetLane:             lane,
		CurrentExecutor:        strings.TrimSpace(current.CurrentExecutor),
		ExecutorGeneration:     current.ExecutorGeneration,
		LastTakeoverAt:         current.LastTakeoverAt,
		LastTakeoverBy:         current.LastTakeoverBy,
		LastTakeoverReason:     current.LastTakeoverReason,
		BindingMode:            "durable-lane-executor-adoption",
		RequiredForIntake:      true,
		MainAgentSpawnOwner:    packet.OwnerBinding.MainAgentSpawnOwner,
		RuntimeSessionBoundary: packet.OwnerBinding.RuntimeSessionBoundary,
	}
	if adoptedOwner.CurrentExecutor == strings.TrimSpace(packet.OwnerBinding.CurrentExecutor) && adoptedOwner.ExecutorGeneration == packet.OwnerBinding.ExecutorGeneration {
		return ReviewerPacketAdoptionResult{}, fmt.Errorf("reviewer packet ownerBinding is already current; adoption is not needed")
	}
	if adoptedOwner.ExecutorGeneration <= packet.OwnerBinding.ExecutorGeneration {
		return ReviewerPacketAdoptionResult{}, fmt.Errorf("reviewer packet adoption requires a newer executor generation: packet=%d current=%d", packet.OwnerBinding.ExecutorGeneration, adoptedOwner.ExecutorGeneration)
	}
	adoptionPath := reviewerPacketAdoptionPath(caseRoot, packet.PacketID)
	result := ReviewerPacketAdoptionResult{
		SchemaVersion:        1,
		Command:              commandName,
		Mode:                 "reviewer-packet-adoption",
		CaseRoot:             caseRoot,
		RepoRoot:             repoRoot,
		Pack:                 pack,
		IsMutation:           !opt.WhatIf,
		RequiresConfirmation: true,
		PacketID:             packet.PacketID,
		PacketPath:           packetPath,
		AdoptionPath:         adoptionPath,
		Lane:                 lane,
		Actor:                actor,
		Reason:               reason,
		DispatchedOwner:      packet.OwnerBinding,
		AdoptedOwner:         adoptedOwner,
		Boundary: []string{
			"adoption preserves the immutable reviewer packet and existing reviewer result packetId bindings",
			"adoption only transfers strict intake ownership to the current durable lane executor",
			"runtime does not spawn or monitor reviewers, execute heavy tools, or write authority/confirmed state",
		},
	}
	applyCommand := reviewerPacketAdoptionCommand(packetPath, lane, actor, reason, true)
	if opt.WhatIf {
		result.NextSteps = []string{"review the exact packet and executor generation change, then run the same adoption command with -Apply"}
		result.MissionCommanderAction = mission.MissionCommanderAction{State: "needs-reviewer-packet-adoption-apply", PrimaryCommand: applyCommand, Boundary: result.Boundary}
	} else {
		unlock, lockErr := acquireReviewerIntakeLock(caseRoot, "reviewer-adoption-"+packet.PacketID)
		if lockErr != nil {
			return ReviewerPacketAdoptionResult{}, lockErr
		}
		defer unlock()
		packetBytes, err = readStableReviewerArtifact(filepath.Dir(packetPath), packetPath, "review packet", maxReviewPacketBytes)
		if err != nil {
			return ReviewerPacketAdoptionResult{}, err
		}
		packet, err = decodeIntakePacket(packetBytes)
		if err != nil {
			return ReviewerPacketAdoptionResult{}, err
		}
		if err := validateIntakePacket(packet, packetPath, repoRoot, caseRoot, pack); err != nil {
			return ReviewerPacketAdoptionResult{}, err
		}
		if err := validatePacketIntegrity(caseRoot, packetPath, packet, packetBytes); err != nil {
			return ReviewerPacketAdoptionResult{}, err
		}
		if err := validateAdoptedOwnerStillCurrent(caseRoot, adoptedOwner); err != nil {
			return ReviewerPacketAdoptionResult{}, err
		}
		adoption := ReviewerPacketAdoption{
			SchemaVersion:          1,
			Kind:                   "reviewer-packet-owner-adoption",
			PacketID:               packet.PacketID,
			PacketPath:             packetPath,
			PacketSHA256:           sha256Hex(packetBytes),
			RepoRoot:               repoRoot,
			CaseRoot:               caseRoot,
			Pack:                   pack,
			Lane:                   lane,
			DispatchedOwner:        packet.OwnerBinding,
			AdoptedOwner:           adoptedOwner,
			Actor:                  actor,
			Reason:                 reason,
			CreatedAt:              time.Now().UTC().Format(time.RFC3339Nano),
			NoSpawn:                true,
			NoHeavyTool:            true,
			NoAuthorityOrConfirmed: true,
		}
		if err := writeReviewerPacketAdoption(adoptionPath, adoption); err != nil {
			return ReviewerPacketAdoptionResult{}, err
		}
		result.Applied = true
		result.NextSteps = []string{"rerun the same packet-level reviewer batch intake with -WhatIf before -Apply"}
		batchPreview := packet.ReviewerOrchestration.BatchPreviewCommand
		if strings.TrimSpace(batchPreview) == "" {
			batchPreview = reviewerPacketBatchPreviewCommand(packetPath, lane, actor)
		}
		result.MissionCommanderAction = mission.MissionCommanderAction{State: "reviewer-packet-adopted", PrimaryCommand: batchPreview, Boundary: result.Boundary}
	}
	result.MissionCommanderNextActions = []mission.MissionCommanderNextActionItem{{Lane: lane, Label: packet.PacketID, State: result.MissionCommanderAction.State, Command: result.MissionCommanderAction.PrimaryCommand, Source: "reviewerPacketAdoption", RequiresReview: true, Reasons: append([]string{}, result.NextSteps...), Boundary: append([]string{}, result.Boundary...)}}
	result.MissionCommanderActionQueue = mission.MissionCommanderActionQueueFor(result.MissionCommanderNextActions)
	return result, nil
}

func IntakeReadyReviewerResults(repoRoot, caseRoot, pack string, opt ReviewerBatchIntakeOptions) (ReviewerBatchIntakeResult, error) {
	inst, err := instance.AssertAttached(caseRoot, repoRoot, pack)
	if err != nil {
		return ReviewerBatchIntakeResult{}, err
	}
	caseRoot = inst.CaseRoot
	packetPath, err := requiredAbsolutePath(opt.PacketPath, "review packet")
	if err != nil {
		return ReviewerBatchIntakeResult{}, err
	}
	packetBytes, err := readStableReviewerArtifact(filepath.Dir(packetPath), packetPath, "review packet", maxReviewPacketBytes)
	if err != nil {
		return ReviewerBatchIntakeResult{}, err
	}
	packet, err := decodeIntakePacket(packetBytes)
	if err != nil {
		return ReviewerBatchIntakeResult{}, err
	}
	if err := validateIntakePacket(packet, packetPath, repoRoot, caseRoot, pack); err != nil {
		return ReviewerBatchIntakeResult{}, err
	}
	if err := validatePacketIntegrity(caseRoot, packetPath, packet, packetBytes); err != nil {
		return ReviewerBatchIntakeResult{}, err
	}
	if err := validateIntakePacketRoute(repoRoot, pack, packet); err != nil {
		return ReviewerBatchIntakeResult{}, err
	}
	lane := strings.TrimSpace(opt.Lane)
	if lane == "" {
		return ReviewerBatchIntakeResult{}, fmt.Errorf("reviewer batch intake requires -Lane <lane id>")
	}
	if lane != packet.TargetLane {
		return ReviewerBatchIntakeResult{}, fmt.Errorf("reviewer batch intake lane %q does not match packet targetLane %q", lane, packet.TargetLane)
	}
	actor := strings.TrimSpace(opt.Actor)
	if actor == "" {
		return ReviewerBatchIntakeResult{}, fmt.Errorf("reviewer batch intake requires -Actor <main-agent actor>")
	}
	result := ReviewerBatchIntakeResult{
		SchemaVersion: 1,
		Command:       commandName,
		Mode:          "reviewer-batch-intake",
		CaseRoot:      caseRoot,
		RepoRoot:      repoRoot,
		Pack:          pack,
		IsMutation:    !opt.WhatIf,
		PacketPath:    packetPath,
		Lane:          lane,
		Actor:         actor,
		Total:         len(packet.ShardHandoffs),
		Boundary: []string{
			"batch intake processes packet shard handoffs in deterministic packet order and stops at the first blocked, partial, or failed intake",
			"each shard preserves strict reviewer result validation and verification-before-decision writeback",
			"runtime does not spawn or monitor reviewers, execute heavy tools, or write authority/confirmed state",
		},
	}
shardLoop:
	for _, handoff := range packet.ShardHandoffs {
		resultPath, err := requiredAbsolutePath(handoff.ReviewerResultPath, "reviewer result")
		if err != nil {
			return finalizeReviewerBatchIntakeResult(result), err
		}
		if !pathInside(packet.ReviewerOrchestration.ResultRoot, resultPath) {
			return finalizeReviewerBatchIntakeResult(result), fmt.Errorf("reviewer batch intake result path %q for shard %s is outside packet resultRoot %q", resultPath, handoff.ShardID, packet.ReviewerOrchestration.ResultRoot)
		}
		fileState, err := refsf.ClassifyNonEmptyRegularFile(resultPath)
		if err != nil {
			return finalizeReviewerBatchIntakeResult(result), err
		}
		switch fileState {
		case refsf.RegularFileMissing, refsf.RegularFileWaiting:
			result.Waiting++
			continue
		case refsf.RegularFileSymlink:
			return finalizeReviewerBatchIntakeResult(result), fmt.Errorf("reviewer batch intake result path %q for shard %s must not be a symlink", resultPath, handoff.ShardID)
		}
		if err := ensureReviewerResultCollectedForIntake(caseRoot, packet, packetPath, handoff, resultPath); err != nil {
			result.Stopped = true
			result.StopShardID = handoff.ShardID
			result.StopReason = err.Error()
			result.NextSteps = []string{"publish the packet-derived reviewer result candidate via staging and collection before rerunning batch intake"}
			return finalizeReviewerBatchIntakeResult(result), err
		}
		result.Ready++
		intake, intakeErr := IntakeReviewerResult(repoRoot, caseRoot, pack, ReviewerIntakeOptions{
			PacketPath:         packetPath,
			ReviewerResultPath: resultPath,
			ExpectedShardID:    handoff.ShardID,
			Lane:               lane,
			Actor:              actor,
			WhatIf:             opt.WhatIf,
		})
		writebackStatus := strings.TrimSpace(intake.WritebackStatus)
		switch writebackStatus {
		case "":
		case "complete":
			result.Results = append(result.Results, intake)
			result.Processed++
			result.Completed++
			result.Applied = result.Applied || intake.Applied
		case "already-complete":
			result.Results = append(result.Results, intake)
			result.Processed++
			result.AlreadyComplete++
		default:
			result.Results = append(result.Results, intake)
			result.Processed++
		}
		if intakeErr != nil {
			result.Stopped = true
			result.StopShardID = handoff.ShardID
			result.StopReason = intakeErr.Error()
			result.NextSteps = []string{"repair or retry the stop shard using its strict single-result intake handoff before rerunning batch intake"}
			return finalizeReviewerBatchIntakeResult(result), intakeErr
		}
		switch writebackStatus {
		case "blocked", "event-id-collision", "verification-recorded", "complete-post-validation-failed":
			result.Stopped = true
			result.StopShardID = handoff.ShardID
			result.StopReason = "reviewer intake stopped with writebackStatus=" + writebackStatus
			break shardLoop
		}
	}
	if result.Ready == 0 {
		result.NextSteps = []string{"collect reviewer result JSON at packet shard result paths, then rerun the same batch intake with -WhatIf"}
	} else if result.Stopped {
		result.NextSteps = []string{"repair or retry the stop shard using its strict single-result intake handoff before rerunning batch intake"}
	} else if result.Waiting > 0 {
		result.NextSteps = []string{"collect the remaining reviewer result JSON, then rerun the same batch intake with -WhatIf before applying or continuing the lane"}
	} else if opt.WhatIf {
		result.NextSteps = []string{"inspect every previewed shard result, then rerun the same batch intake with -Apply"}
	} else {
		result.NextSteps = []string{"consume the final shard postValidation and downstream reviewer writeback handoff before continuing the lane"}
	}
	return finalizeReviewerBatchIntakeResult(result), nil
}

func finalizeReviewerBatchIntakeResult(result ReviewerBatchIntakeResult) ReviewerBatchIntakeResult {
	closed := result.Completed + result.AlreadyComplete
	result.Partial = result.Stopped || (closed > 0 && closed < result.Total) || (result.Waiting > 0 && result.Processed > 0)
	result.RerunCommand = reviewerPacketBatchPreviewCommand(result.PacketPath, result.Lane, result.Actor)
	result.NextOpenShardID, result.RemainingShardIDs = reviewerBatchIntakeOpenShardProgress(result)
	if result.Stopped {
		recovery := reviewerBatchIntakeRecoveryAction(result)
		result.RecoveryAction = &recovery
	}
	action := reviewerBatchIntakeMissionCommanderAction(result)
	result.MissionCommanderAction = action
	result.MissionCommanderNextActions = reviewerBatchIntakeMissionCommanderNextActions(result, action)
	result.MissionCommanderActionQueue = reviewerBatchIntakeActionQueueWithRefresh(mission.MissionCommanderActionQueueFor(result.MissionCommanderNextActions), result.CaseRoot)
	result.MissionCommanderDriverReceipt = reviewerBatchIntakeMissionCommanderDriverReceipt(result)
	return result
}

func reviewerBatchIntakeActionQueueWithRefresh(queue mission.MissionCommanderActionQueue, caseRoot string) mission.MissionCommanderActionQueue {
	if queue.CurrentDriverRequest == nil {
		return queue
	}
	refreshed := mission.MissionCommanderDriverRequestWithRefreshStatusCommand(*queue.CurrentDriverRequest, reviewerBatchIntakeStatusCommand(caseRoot))
	queue.CurrentDriverRequest = &refreshed
	return queue
}

func reviewerBatchIntakeMissionCommanderDriverReceipt(result ReviewerBatchIntakeResult) *workstream.MissionCommanderDriverReceipt {
	command := reviewerPacketBatchCommand(result.PacketPath, result.Lane, result.Actor, result.IsMutation)
	if strings.TrimSpace(command) == "" {
		command = result.Command
	}
	return &workstream.MissionCommanderDriverReceipt{
		SchemaVersion:                 1,
		State:                         "refreshed",
		Outcome:                       reviewerBatchIntakeDriverReceiptOutcome(result),
		Lane:                          result.Lane,
		Command:                       command,
		RefreshedActionQueueSummary:   result.MissionCommanderActionQueue.Summary,
		RefreshedCurrentRunLoopStep:   result.MissionCommanderActionQueue.CurrentRunLoopStepID,
		RefreshedCurrentDriverRequest: result.MissionCommanderActionQueue.CurrentDriverRequest,
		Boundary: mission.UniqueStrings([]string{
			"driver receipt records the reviewer batch intake command result after deterministic ready-result intake evaluation",
			"driver receipt does not prove the Go runtime spawned, polled, stopped, or managed reviewer sessions",
			"reviewer batch intake preserves WhatIf-before-Apply and stops at the first blocked, partial, or invalid shard",
			"reviewer batch intake does not write authority/confirmed state or execute heavy tools",
			"after consuming this receipt, run refreshedCurrentDriverRequest or expectedReceipt.refreshStatusCommand only under the request boundary",
		}),
	}
}

func reviewerBatchIntakeDriverReceiptOutcome(result ReviewerBatchIntakeResult) string {
	if result.IsMutation {
		return "reviewer-batch-intake-apply-result"
	}
	return "reviewer-batch-intake-preview-result"
}

func reviewerBatchIntakeStatusCommand(caseRoot string) string {
	caseRoot = strings.TrimSpace(caseRoot)
	if caseRoot == "" {
		return "/rekit status -Format json"
	}
	return "/rekit status -Target " + quoteReviewerCommandArg(caseRoot) + " -Format json"
}

func reviewerBatchIntakeOpenShardProgress(result ReviewerBatchIntakeResult) (string, []string) {
	closed := map[string]bool{}
	for _, item := range result.Results {
		shardID := strings.TrimSpace(item.ShardID)
		if shardID == "" {
			continue
		}
		switch strings.TrimSpace(item.WritebackStatus) {
		case "complete", "already-complete":
			closed[shardID] = true
		}
	}
	remaining := []string{}
	if stop := strings.TrimSpace(result.StopShardID); stop != "" {
		remaining = append(remaining, stop)
	}
	for _, item := range result.Results {
		progress := item.Summary.OrchestrationProgress
		if progress == nil {
			continue
		}
		for _, shardID := range progress.RemainingShardIDs {
			shardID = strings.TrimSpace(shardID)
			if shardID == "" || closed[shardID] {
				continue
			}
			remaining = append(remaining, shardID)
		}
	}
	remaining = mission.UniqueStrings(remaining)
	if len(remaining) == 0 && result.Waiting > 0 {
		return "", remaining
	}
	if len(remaining) == 0 {
		return "", nil
	}
	return remaining[0], remaining
}

func reviewerBatchIntakeRecoveryAction(result ReviewerBatchIntakeResult) mission.MissionCommanderNextActionItem {
	command := strings.TrimSpace(result.RerunCommand)
	state := "reviewer-batch-intake-stopped"
	reasons := []string{}
	if stop := strings.TrimSpace(result.StopShardID); stop != "" {
		reasons = append(reasons, "stopShardId="+stop)
	}
	if reason := strings.TrimSpace(result.StopReason); reason != "" {
		reasons = append(reasons, "stopReason="+reason)
	}
	if len(result.RemainingShardIDs) > 0 {
		reasons = append(reasons, "remainingShards="+strings.Join(result.RemainingShardIDs, ","))
	}
	if command == "" {
		command = "repair reviewer batch intake stop shard " + textOr(result.StopShardID, "<stop-shard>") + ", then rerun packet-level ready reviewer results intake -WhatIf"
		state = "reviewer-batch-intake-repair-required"
	}
	return mission.MissionCommanderNextActionItem{
		Lane:           result.Lane,
		Label:          filepath.Base(result.PacketPath),
		ActionID:       "reviewer-batch-intake-stop-shard",
		State:          state,
		Command:        command,
		Source:         "reviewerBatchIntake.recovery",
		Blocked:        true,
		RequiresReview: true,
		Reasons:        mission.UniqueStrings(append(reasons, result.NextSteps...)),
		Boundary:       append(append([]string{}, result.Boundary...), "repair the stop shard and rerun -WhatIf before any batch Apply or lane continuation"),
	}
}

func reviewerBatchIntakeMissionCommanderAction(result ReviewerBatchIntakeResult) mission.MissionCommanderAction {
	previewCommand := reviewerPacketBatchPreviewCommand(result.PacketPath, result.Lane, result.Actor)
	applyCommand := reviewerPacketBatchApplyCommand(result.PacketPath, result.Lane, result.Actor)
	boundary := append([]string{}, result.Boundary...)
	if result.Stopped {
		primary := previewCommand
		if result.RecoveryAction != nil && strings.TrimSpace(result.RecoveryAction.Command) != "" {
			primary = result.RecoveryAction.Command
		}
		return mission.MissionCommanderAction{State: "reviewer-batch-intake-stopped", PrimaryCommand: primary, Boundary: append(boundary, "repair the stop shard before applying or continuing")}
	}
	if result.Ready == 0 || result.Waiting > 0 {
		return mission.MissionCommanderAction{State: "ready-for-reviewer-batch-intake-preview", PrimaryCommand: previewCommand, Boundary: boundary}
	}
	allPreviewed := !result.IsMutation && result.Processed > 0
	for _, item := range result.Results {
		switch strings.TrimSpace(item.WritebackStatus) {
		case "previewed", "already-complete":
		default:
			allPreviewed = false
		}
		if !allPreviewed {
			break
		}
	}
	if allPreviewed {
		return mission.MissionCommanderAction{State: "ready-for-reviewer-batch-intake-apply-after-preview", PrimaryCommand: applyCommand, Boundary: append(boundary, "apply only after inspecting every previewed shard and cited evidenceRefs")}
	}
	if result.Completed > 0 || result.AlreadyComplete > 0 {
		if primary, followUp := reviewerBatchIntakePostValidationCommands(result); primary != "" {
			return mission.MissionCommanderAction{State: "reviewer-batch-intake-writeback-complete", PrimaryCommand: primary, FollowUpCommands: followUp, Boundary: boundary}
		}
	}
	return mission.MissionCommanderAction{State: "ready-for-reviewer-batch-intake-preview", PrimaryCommand: previewCommand, Boundary: boundary}
}

func reviewerBatchIntakeMissionCommanderNextActions(result ReviewerBatchIntakeResult, action mission.MissionCommanderAction) []mission.MissionCommanderNextActionItem {
	if action.State == "reviewer-batch-intake-writeback-complete" {
		if items := reviewerBatchIntakePostValidationNextActions(result); len(items) > 0 {
			return mission.UniqueCommanderNextActions(items)
		}
	}
	items := []mission.MissionCommanderNextActionItem{}
	if result.Stopped && result.RecoveryAction != nil {
		items = append(items, *result.RecoveryAction)
	}
	blocked := result.Stopped
	requiresReview := true
	if action.State == "reviewer-batch-intake-writeback-complete" {
		requiresReview = false
	}
	if action.PrimaryCommand != "" {
		items = append(items, mission.MissionCommanderNextActionItem{Lane: result.Lane, Label: filepath.Base(result.PacketPath), ActionID: "reviewer-batch-intake-current", State: action.State, Command: action.PrimaryCommand, Source: "reviewerBatchIntake", Blocked: blocked, RequiresReview: requiresReview, Reasons: append([]string{}, result.NextSteps...), Boundary: append([]string{}, action.Boundary...)})
	}
	for _, command := range action.FollowUpCommands {
		items = append(items, mission.MissionCommanderNextActionItem{Lane: result.Lane, Label: filepath.Base(result.PacketPath), State: action.State, Command: command, Source: "reviewerBatchIntake.followUp", Blocked: blocked, RequiresReview: requiresReview, Reasons: append(append([]string{}, result.NextSteps...), "follow-up is available only after the reviewer batch intake primary action is satisfied"), Boundary: append([]string{}, action.Boundary...)})
	}
	return mission.UniqueCommanderNextActions(items)
}

func reviewerBatchIntakePostValidationNextActions(result ReviewerBatchIntakeResult) []mission.MissionCommanderNextActionItem {
	for i := len(result.Results) - 1; i >= 0; i-- {
		item := result.Results[i]
		status := strings.TrimSpace(item.WritebackStatus)
		if status != "complete" && status != "already-complete" {
			continue
		}
		items := reviewerIntakePostValidationNextActions(item)
		if len(items) == 0 {
			continue
		}
		out := make([]mission.MissionCommanderNextActionItem, 0, len(items))
		for _, next := range items {
			copyItem := next
			copyItem.Source = "reviewerBatchIntake." + textOr(next.Source, "postValidation")
			copyItem.Reasons = append([]string{"reviewer batch intake processed packet ready results; consume the latest shard postValidation handoff"}, next.Reasons...)
			out = append(out, copyItem)
		}
		return out
	}
	return nil
}

func reviewerBatchIntakePostValidationCommands(result ReviewerBatchIntakeResult) (string, []string) {
	items := reviewerBatchIntakePostValidationNextActions(result)
	if len(items) == 0 {
		return "", nil
	}
	primary := items[0].Command
	followUp := []string{}
	for _, item := range items[1:] {
		followUp = append(followUp, item.Command)
	}
	return primary, mission.UniqueStrings(followUp)
}

func validateDirectReviewerSessionReceiptsForIntake(caseRoot, packetPath string, packet Packet, packetBytes []byte, handoff ShardHandoff, _ []byte, result ReviewerResult, effectiveOwner OwnerBinding, adoption *ReviewerPacketAdoption) error {
	dispatchRoot := filepath.Join(reviewerSessionRoot(packetPath, handoff.ShardID), "dispatches")
	if !reviewpath.CollectionNamespacePathSafe(caseRoot, dispatchRoot, true) {
		return fmt.Errorf("direct reviewer session dispatch receipt namespace is not symlink-free and case-local")
	}
	entries, err := os.ReadDir(dispatchRoot)
	if os.IsNotExist(err) {
		return nil
	}
	if err != nil {
		return fmt.Errorf("read direct reviewer session dispatch receipts for intake: %w", err)
	}
	names := make([]string, 0, len(entries))
	for _, entry := range entries {
		if entry.IsDir() || !strings.HasSuffix(strings.ToLower(entry.Name()), ".json") {
			continue
		}
		names = append(names, entry.Name())
	}
	sort.Strings(names)
	if len(names) == 0 {
		return fmt.Errorf("direct reviewer session receipt namespace exists without a valid dispatch receipt")
	}
	adoptionPath, adoptionSHA256 := reviewerSessionAdoptionProvenance(caseRoot, packet.PacketID, adoption)
	matchingDispatches := 0
	for _, name := range names {
		dispatchPath := filepath.Join(dispatchRoot, name)
		dispatchBytes, readErr := readReviewerSessionReceiptBytes(caseRoot, dispatchPath, "direct reviewer session dispatch receipt")
		if readErr != nil {
			return readErr
		}
		dispatch, decodeErr := reviewersession.DecodeDispatch(dispatchBytes)
		if decodeErr != nil {
			return decodeErr
		}
		if !reviewerSessionDispatchMatchesCurrent(packetPath, dispatchPath, packet, packetBytes, handoff, dispatch, effectiveOwner, adoptionPath, adoptionSHA256) {
			return fmt.Errorf("direct reviewer session dispatch receipt does not match current immutable packet, prompt, owner, and adoption bindings")
		}
		completionPath := reviewerSessionCompletionPath(packetPath, handoff.ShardID, dispatch.DispatchID)
		if !reviewpath.CollectionNamespacePathSafe(caseRoot, completionPath, true) {
			return fmt.Errorf("direct reviewer session completion receipt path is not symlink-free and case-local")
		}
		completionState, stateErr := refsf.ClassifyNonEmptyRegularFile(completionPath)
		if stateErr != nil || (completionState != refsf.RegularFileMissing && completionState != refsf.RegularFileReady) {
			return fmt.Errorf("direct reviewer session completion receipt must be a non-empty regular file")
		}
		if completionState == refsf.RegularFileReady {
			completionBytes, readErr := readReviewerSessionReceiptBytes(caseRoot, completionPath, "direct reviewer session completion receipt")
			if readErr != nil {
				return readErr
			}
			completion, decodeErr := reviewersession.DecodeCompletion(completionBytes)
			if decodeErr != nil || reviewersession.ValidateCompletionDispatchLineage(completion, dispatch, dispatchPath, sha256Hex(dispatchBytes)) != nil {
				return fmt.Errorf("direct reviewer session completion receipt does not match its current dispatch lineage")
			}
			if dispatch.ReviewerSession == result.ReviewerSession {
				if completion.Outcome == "failed" {
					return fmt.Errorf("direct reviewer result session has a failed completion receipt")
				}
				return fmt.Errorf("direct reviewer result session has an unsupported successful completion receipt")
			}
		}
		if dispatch.ReviewerSession == result.ReviewerSession {
			matchingDispatches++
		}
	}
	if matchingDispatches == 1 {
		return nil
	}
	if matchingDispatches > 1 {
		return fmt.Errorf("direct reviewer result session matches multiple current durable dispatch receipts")
	}
	return fmt.Errorf("direct reviewer result session is not bound to a current durable dispatch receipt")
}

func reviewerSessionProvenanceForIntake(caseRoot, packetPath string, packet Packet, packetBytes []byte, handoff ShardHandoff, resultBytes []byte, result ReviewerResult, effectiveOwner OwnerBinding, adoption *ReviewerPacketAdoption) (reviewerSessionWritebackProvenance, error) {
	if !reviewerSessionReceiptsRequired(handoff) {
		if packet.ReviewerOrchestration.ManagedDispatchPacket == nil {
			return reviewerSessionWritebackProvenance{}, nil
		}
		if err := validateDirectReviewerSessionReceiptsForIntake(caseRoot, packetPath, packet, packetBytes, handoff, resultBytes, result, effectiveOwner, adoption); err != nil {
			return reviewerSessionWritebackProvenance{}, err
		}
		return reviewerSessionWritebackProvenance{}, nil
	}
	dispatchRoot := filepath.Join(reviewerSessionRoot(packetPath, handoff.ShardID), "dispatches")
	entries, err := os.ReadDir(dispatchRoot)
	if os.IsNotExist(err) {
		return reviewerSessionWritebackProvenance{}, fmt.Errorf("managed reviewer intake requires a durable dispatch and successful completion receipt")
	}
	if err != nil {
		return reviewerSessionWritebackProvenance{}, fmt.Errorf("read reviewer session dispatch receipts for intake: %w", err)
	}
	exactMatches := []reviewerSessionWritebackProvenance{}
	matchingDispatches := 0
	adoptionPath, adoptionSHA256 := reviewerSessionAdoptionProvenance(caseRoot, packet.PacketID, adoption)
	for _, entry := range entries {
		if entry.IsDir() || !strings.HasSuffix(strings.ToLower(entry.Name()), ".json") {
			continue
		}
		dispatchPath := filepath.Join(dispatchRoot, entry.Name())
		dispatchBytes, readErr := readReviewerSessionReceiptBytes(caseRoot, dispatchPath, "reviewer session dispatch receipt")
		if readErr != nil {
			continue
		}
		dispatch, decodeErr := reviewersession.DecodeDispatch(dispatchBytes)
		if decodeErr != nil || !reviewerSessionDispatchMatchesCurrent(packetPath, dispatchPath, packet, packetBytes, handoff, dispatch, effectiveOwner, adoptionPath, adoptionSHA256) || dispatch.ReviewerSession != result.ReviewerSession {
			continue
		}
		matchingDispatches++
		completionPath := reviewerSessionCompletionPath(packetPath, handoff.ShardID, dispatch.DispatchID)
		completionBytes, readErr := readReviewerSessionReceiptBytes(caseRoot, completionPath, "reviewer session completion receipt")
		if readErr != nil {
			continue
		}
		completion, decodeErr := reviewersession.DecodeCompletion(completionBytes)
		if decodeErr != nil || reviewersession.ValidateCompletionDispatchLineage(completion, dispatch, dispatchPath, sha256Hex(dispatchBytes)) != nil || completion.Outcome != "succeeded" || completion.CompletionOwner != reviewerSessionOwner(effectiveOwner) {
			continue
		}
		expectedInputPath := ""
		if handoff.ReviewerStagingCommands != nil {
			expectedInputPath = handoff.ReviewerStagingCommands.SourceCaptureInput
		}
		if expectedInputPath == "" || !samePath(completion.ReviewerResultInputPath, expectedInputPath) {
			continue
		}
		inputBytes, readErr := readStableReviewerArtifact(filepath.Dir(completion.ReviewerResultInputPath), completion.ReviewerResultInputPath, "reviewer session completion result input", maxReviewerResultBytes)
		if readErr != nil || completion.ReviewerResultInputSHA256 != sha256Hex(inputBytes) || completion.ReviewerResultInputBytes != len(inputBytes) || !bytes.Equal(inputBytes, resultBytes) {
			continue
		}
		exactMatches = append(exactMatches, reviewerSessionWritebackProvenance{ReviewerHarness: dispatch.ReviewerHarness, DispatchID: dispatch.DispatchID, DispatchPath: dispatchPath, DispatchSHA256: sha256Hex(dispatchBytes), CompletionPath: completionPath, CompletionSHA256: sha256Hex(completionBytes), ReviewerResultInputPath: completion.ReviewerResultInputPath, ReviewerResultInputSHA256: completion.ReviewerResultInputSHA256})
	}
	if len(exactMatches) == 1 {
		return exactMatches[0], nil
	}
	if len(exactMatches) > 1 {
		return reviewerSessionWritebackProvenance{}, fmt.Errorf("reviewer intake result matches multiple successful completion receipt lineages")
	}
	if matchingDispatches > 0 {
		return reviewerSessionWritebackProvenance{}, fmt.Errorf("reviewer intake requires a successful completion receipt matching the exact canonical result bytes and current owner")
	}
	return reviewerSessionWritebackProvenance{}, fmt.Errorf("reviewer intake result session is not bound to a current durable dispatch receipt")
}

func ensureReviewerResultCollectedForIntake(caseRoot string, packet Packet, packetPath string, handoff ShardHandoff, resultPath string) error {
	collectionBound := handoff.ReviewerStagingCommands != nil || handoff.ReviewerCollectionCommands != nil || strings.TrimSpace(handoff.ReviewerResultCandidatePath) != ""
	if !collectionBound {
		return nil
	}
	if !samePath(resultPath, handoff.ReviewerResultPath) {
		return fmt.Errorf("reviewer result intake for shard %q requires packet-derived canonical reviewerResultPath after staging and collection", handoff.ShardID)
	}
	candidatePath, err := requiredAbsolutePath(handoff.ReviewerResultCandidatePath, "reviewer result candidate")
	if err != nil {
		return fmt.Errorf("reviewer result intake requires a packet-derived reviewer result candidate for shard %q: %w", handoff.ShardID, err)
	}
	resultRoot, err := requiredAbsolutePath(packet.ReviewerOrchestration.ResultRoot, "reviewer result root")
	if err != nil {
		return err
	}
	if !reviewpath.CanonicalCollectionShard(caseRoot, packetPath, resultRoot, handoff.ShardID, candidatePath, handoff.ReviewerResultPath) ||
		!reviewpath.CollectionNamespacePathSafe(caseRoot, packetPath, false) ||
		!reviewpath.CollectionNamespacePathSafe(caseRoot, resultRoot, false) ||
		!reviewpath.CollectionNamespacePathSafe(caseRoot, filepath.Dir(candidatePath), true) ||
		!reviewpath.CollectionNamespacePathSafe(caseRoot, resultPath, false) {
		return fmt.Errorf("reviewer result intake for shard %q requires canonical packet-derived collection paths", handoff.ShardID)
	}
	candidate, err := readStableReviewerArtifact(resultRoot, candidatePath, "reviewer result candidate", maxReviewerResultBytes)
	if err != nil {
		return fmt.Errorf("reviewer result intake for shard %q requires staging and collection before canonical intake: %w", handoff.ShardID, err)
	}
	canonical, err := readStableReviewerArtifact(resultRoot, resultPath, "canonical reviewer result", maxReviewerResultBytes)
	if err != nil {
		return err
	}
	if !bytes.Equal(candidate, canonical) {
		return fmt.Errorf("reviewer result intake for shard %q requires canonical reviewer result bytes to match the packet-derived candidate; run collection/recovery before intake", handoff.ShardID)
	}
	return nil
}

func IntakeReviewerResult(repoRoot, caseRoot, pack string, opt ReviewerIntakeOptions) (ReviewerIntakeResult, error) {
	inst, err := instance.AssertAttached(caseRoot, repoRoot, pack)
	if err != nil {
		return ReviewerIntakeResult{}, err
	}
	caseRoot = inst.CaseRoot
	packetPath, err := requiredAbsolutePath(opt.PacketPath, "review packet")
	if err != nil {
		return ReviewerIntakeResult{}, err
	}
	resultPath, err := requiredAbsolutePath(opt.ReviewerResultPath, "reviewer result")
	if err != nil {
		return ReviewerIntakeResult{}, err
	}
	lane := strings.TrimSpace(opt.Lane)
	if lane == "" {
		return ReviewerIntakeResult{}, fmt.Errorf("reviewer intake requires -Lane <lane id>")
	}
	actor := strings.TrimSpace(opt.Actor)
	if actor == "" {
		return ReviewerIntakeResult{}, fmt.Errorf("reviewer intake requires -Actor <main-agent actor>")
	}

	packetBytes, err := readStableReviewerArtifact(filepath.Dir(packetPath), packetPath, "review packet", maxReviewPacketBytes)
	if err != nil {
		return ReviewerIntakeResult{}, err
	}
	packet, err := decodeIntakePacket(packetBytes)
	if err != nil {
		return ReviewerIntakeResult{}, err
	}
	if err := validateIntakePacket(packet, packetPath, repoRoot, caseRoot, pack); err != nil {
		return ReviewerIntakeResult{}, err
	}
	if err := validatePacketIntegrity(caseRoot, packetPath, packet, packetBytes); err != nil {
		return ReviewerIntakeResult{}, err
	}
	if err := validateIntakePacketRoute(repoRoot, pack, packet); err != nil {
		return ReviewerIntakeResult{}, err
	}
	if lane != packet.TargetLane {
		return ReviewerIntakeResult{}, fmt.Errorf("reviewer intake lane %q does not match packet targetLane %q", lane, packet.TargetLane)
	}
	effectiveOwner, adoption, err := validateOwnerBinding(caseRoot, packet, packetPath, packetBytes)
	if err != nil {
		return ReviewerIntakeResult{}, err
	}
	resultBytes, err := readBoundedFile(resultPath, "reviewer result", maxReviewerResultBytes)
	if err != nil {
		return ReviewerIntakeResult{}, err
	}
	reviewerResult, err := decodeReviewerResult(resultBytes)
	if err != nil {
		return ReviewerIntakeResult{}, err
	}
	if expectedShardID := strings.TrimSpace(opt.ExpectedShardID); expectedShardID != "" && reviewerResult.ShardID != expectedShardID {
		return ReviewerIntakeResult{}, fmt.Errorf("reviewer result shard %q does not match expected packet handoff shard %q", reviewerResult.ShardID, expectedShardID)
	}
	if reviewerResult.PacketID != packet.PacketID {
		return ReviewerIntakeResult{}, fmt.Errorf("reviewer result packetId %q does not match packet %q", reviewerResult.PacketID, packet.PacketID)
	}
	handoff, ok := shardHandoffByID(packet.ShardHandoffs, reviewerResult.ShardID)
	if !ok {
		return ReviewerIntakeResult{}, fmt.Errorf("reviewer result shard %q is not present in packet handoffs", reviewerResult.ShardID)
	}
	if err := ensureReviewerResultCollectedForIntake(caseRoot, packet, packetPath, handoff, resultPath); err != nil {
		return ReviewerIntakeResult{}, err
	}
	if err := ensureReviewerResultIntakeRecoveryComplete(caseRoot, packet, packetPath, reviewerResult.ShardID, lane, resultPath); err != nil {
		return ReviewerIntakeResult{}, err
	}
	sessionProvenance, err := reviewerSessionProvenanceForIntake(caseRoot, packetPath, packet, packetBytes, handoff, resultBytes, reviewerResult, effectiveOwner, adoption)
	if err != nil {
		return ReviewerIntakeResult{}, err
	}
	if reviewerResult.RouteID != packet.Route.ID {
		return ReviewerIntakeResult{}, fmt.Errorf("reviewer result routeId %q does not match packet route %q", reviewerResult.RouteID, packet.Route.ID)
	}
	if err := validateRouteOutput(packet.OutputContract, reviewerResult.RouteOutput); err != nil {
		return ReviewerIntakeResult{}, err
	}
	if err := validateRouteOutputBindings(reviewerResult); err != nil {
		return ReviewerIntakeResult{}, err
	}
	shard, ok := shardByID(packet.Shards, reviewerResult.ShardID)
	if !ok {
		return ReviewerIntakeResult{}, fmt.Errorf("reviewer result shard %q is not present in packet", reviewerResult.ShardID)
	}
	if !slices.Equal(reviewerResult.Items, shard.Items) {
		return ReviewerIntakeResult{}, fmt.Errorf("reviewer result items do not match packet shard %s: got %v, want %v", shard.ID, reviewerResult.Items, shard.Items)
	}
	orchestrationSnapshot := reviewerOrchestrationIntake(caseRoot, packet, reviewerResult.ShardID, lane, opt.WhatIf)
	mapping, ok := reviewerDecisionMappingByDecision(reviewerResult.Decision)
	if !ok {
		return ReviewerIntakeResult{}, fmt.Errorf("invalid reviewer decision %q; allowed: %s", reviewerResult.Decision, strings.Join(reviewerResultContract().AllowedDecisions, ","))
	}

	intakeID := reviewerIntakeID(packet, reviewerResult, lane)
	blocked, err := reviewerIntakeBlockers(repoRoot, caseRoot, packet, reviewerResult, mapping)
	if err != nil {
		return ReviewerIntakeResult{}, err
	}
	priorBlocked, err := existingReviewerWritebackBlockers(caseRoot, packet, reviewerResult, lane, intakeID)
	if err != nil {
		return ReviewerIntakeResult{}, err
	}
	blocked = append(blocked, priorBlocked...)
	result := ReviewerIntakeResult{
		SchemaVersion:         1,
		Command:               commandName,
		Mode:                  "reviewer-intake",
		CaseRoot:              caseRoot,
		RepoRoot:              repoRoot,
		Pack:                  pack,
		IsMutation:            !opt.WhatIf,
		Applied:               false,
		WritebackStatus:       "validated",
		ReadyForWriteback:     len(blocked) == 0,
		ReviewRequired:        true,
		PacketPath:            packetPath,
		ReviewerResultPath:    resultPath,
		IntakeID:              intakeID,
		Lane:                  lane,
		ShardID:               shard.ID,
		Actor:                 actor,
		OwnerBinding:          effectiveOwner,
		ReviewerSession:       reviewerResult.ReviewerSession,
		ReviewerResult:        reviewerResult,
		VerificationVerdict:   mapping.VerificationVerdict,
		MainDecision:          mapping.MainDecision,
		BlockedReasons:        blocked,
		OrchestrationSnapshot: orchestrationSnapshot,
	}
	if len(blocked) > 0 {
		result.IsMutation = false
		result.WritebackStatus = "blocked"
		result.OrchestrationSnapshot.ShardStatusAfter = result.WritebackStatus
		result.RepairGuidance = reviewerIntakeRepairGuidance(result)
		result.NextSteps = []string{"resolve reviewer intake blockers or dispatch a smaller read-only shard; no ledger events were written"}
		validation, validationErr := reviewerPostValidation(repoRoot, caseRoot, pack, lane, false)
		if validationErr != nil {
			return ReviewerIntakeResult{}, validationErr
		}
		result.PostValidation = &validation
		return finalizeReviewerIntakeResult(result), nil
	}

	verificationOpt, decisionOpt := reviewerNoteOptions(packet, reviewerResult, mapping, effectiveOwner, sessionProvenance, lane, actor, intakeID, packetPath, resultPath)
	verificationPreview, err := appendReviewerNote(repoRoot, caseRoot, pack, verificationOpt, true)
	if err != nil {
		return ReviewerIntakeResult{}, fmt.Errorf("preview reviewer verification note: %w", err)
	}
	decisionPreview, err := appendReviewerNote(repoRoot, caseRoot, pack, decisionOpt, true)
	if err != nil {
		return ReviewerIntakeResult{}, fmt.Errorf("preview main decision note: %w", err)
	}
	result.Verification = &verificationPreview
	result.Decision = &decisionPreview
	if err := ensureExpectedAppendResult(caseRoot, "verification", verificationPreview); err != nil {
		result.WritebackStatus = "event-id-collision"
		result.OrchestrationSnapshot.ShardStatusAfter = result.WritebackStatus
		result.ReadyForWriteback = false
		result.BlockedReasons = append(result.BlockedReasons, err.Error())
		return finalizeReviewerIntakeResult(result), err
	}
	if err := ensureExpectedAppendResult(caseRoot, "decision", decisionPreview); err != nil {
		result.WritebackStatus = "event-id-collision"
		result.OrchestrationSnapshot.ShardStatusAfter = result.WritebackStatus
		result.ReadyForWriteback = false
		result.BlockedReasons = append(result.BlockedReasons, err.Error())
		return finalizeReviewerIntakeResult(result), err
	}
	if opt.WhatIf {
		result.IsMutation = false
		result.WritebackStatus = "previewed"
		result.NextSteps = []string{"inspect verification and decision WhatIf events, then rerun the same reviewer intake with -Apply to append both events"}
		if isDuplicateAppend(verificationPreview) && isDuplicateAppend(decisionPreview) {
			result.WritebackStatus = "already-complete"
			result.NextSteps = []string{"reviewer intake writeback is already complete; consume postValidation before handing off or continuing the lane"}
		}
		result.OrchestrationSnapshot.ShardStatusAfter = result.WritebackStatus
		validation, err := reviewerPostValidation(repoRoot, caseRoot, pack, lane, false)
		if err != nil {
			return ReviewerIntakeResult{}, err
		}
		result.PostValidation = &validation
		return finalizeReviewerIntakeResult(result), nil
	}

	unlock, err := acquireReviewerIntakeLock(caseRoot, reviewerResultMutationLockID(packet.PacketID, reviewerResult.ShardID))
	if err != nil {
		return ReviewerIntakeResult{}, err
	}
	defer unlock()
	packetBytes, err = readStableReviewerArtifact(filepath.Dir(packetPath), packetPath, "review packet", maxReviewPacketBytes)
	if err != nil {
		return ReviewerIntakeResult{}, err
	}
	packet, err = decodeIntakePacket(packetBytes)
	if err != nil {
		return ReviewerIntakeResult{}, err
	}
	if err := validateIntakePacket(packet, packetPath, repoRoot, caseRoot, pack); err != nil {
		return ReviewerIntakeResult{}, err
	}
	if err := validatePacketIntegrity(caseRoot, packetPath, packet, packetBytes); err != nil {
		return ReviewerIntakeResult{}, err
	}
	currentResultBytes, err := readBoundedFile(resultPath, "reviewer result", maxReviewerResultBytes)
	if err != nil {
		return ReviewerIntakeResult{}, err
	}
	if !bytes.Equal(currentResultBytes, resultBytes) {
		return ReviewerIntakeResult{}, fmt.Errorf("reviewer result changed before writeback")
	}
	latestHandoff, ok := shardHandoffByID(packet.ShardHandoffs, reviewerResult.ShardID)
	if !ok {
		return ReviewerIntakeResult{}, fmt.Errorf("reviewer result shard %q is not present in packet handoffs", reviewerResult.ShardID)
	}
	if err := ensureReviewerResultCollectedForIntake(caseRoot, packet, packetPath, latestHandoff, resultPath); err != nil {
		return ReviewerIntakeResult{}, err
	}
	if err := ensureReviewerResultIntakeRecoveryComplete(caseRoot, packet, packetPath, reviewerResult.ShardID, lane, resultPath); err != nil {
		return ReviewerIntakeResult{}, err
	}
	currentSessionProvenance, err := reviewerSessionProvenanceForIntake(caseRoot, packetPath, packet, packetBytes, latestHandoff, currentResultBytes, reviewerResult, effectiveOwner, adoption)
	if err != nil {
		return ReviewerIntakeResult{}, err
	}
	if currentSessionProvenance != sessionProvenance {
		return ReviewerIntakeResult{}, fmt.Errorf("reviewer session receipt provenance changed before writeback")
	}
	if err := validateAdoptedOwnerStillCurrent(caseRoot, effectiveOwner); err != nil {
		return ReviewerIntakeResult{}, fmt.Errorf("reviewer intake owner changed before writeback: %w", err)
	}
	priorBlocked, err = existingReviewerWritebackBlockers(caseRoot, packet, reviewerResult, lane, intakeID)
	if err != nil {
		return ReviewerIntakeResult{}, err
	}
	if len(priorBlocked) > 0 {
		result.IsMutation = false
		result.ReadyForWriteback = false
		result.WritebackStatus = "blocked"
		result.OrchestrationSnapshot.ShardStatusAfter = result.WritebackStatus
		result.BlockedReasons = append(result.BlockedReasons, priorBlocked...)
		result.NextSteps = []string{"resolve existing reviewer writeback conflict before applying a new reviewer result for this shard"}
		validation, validationErr := reviewerPostValidation(repoRoot, caseRoot, pack, lane, false)
		if validationErr != nil {
			return ReviewerIntakeResult{}, validationErr
		}
		result.PostValidation = &validation
		return finalizeReviewerIntakeResult(result), nil
	}

	verificationApply, err := appendReviewerNote(repoRoot, caseRoot, pack, verificationOpt, false)
	result.Verification = &verificationApply
	if err != nil {
		if verificationApply.EventID != "" {
			result.Applied = verificationApply.Applied
			result.WritebackStatus = "verification-recorded"
			result.OrchestrationSnapshot.ShardStatusAfter = result.WritebackStatus
			result.NextSteps = []string{"retry the identical reviewer intake apply; deterministic event IDs will preserve the verification event and complete the missing decision idempotently"}
			return finalizeReviewerIntakeResult(result), fmt.Errorf("append reviewer verification note after event %s; writebackStatus=%s and retry is safe: %w", verificationApply.EventID, result.WritebackStatus, err)
		}
		return ReviewerIntakeResult{}, fmt.Errorf("append reviewer verification note: %w", err)
	}
	if err := ensureExpectedAppendResult(caseRoot, "verification", verificationApply); err != nil {
		result.WritebackStatus = "event-id-collision"
		result.OrchestrationSnapshot.ShardStatusAfter = result.WritebackStatus
		result.ReadyForWriteback = false
		result.BlockedReasons = append(result.BlockedReasons, err.Error())
		return finalizeReviewerIntakeResult(result), err
	}
	result.Applied = verificationApply.Applied
	result.WritebackStatus = "verification-recorded"
	result.OrchestrationSnapshot.ShardStatusAfter = result.WritebackStatus
	decisionApply, err := appendReviewerNote(repoRoot, caseRoot, pack, decisionOpt, false)
	result.Decision = &decisionApply
	if err != nil {
		result.NextSteps = []string{"retry the identical reviewer intake apply; deterministic event IDs will preserve the verification event and complete the missing decision idempotently"}
		return finalizeReviewerIntakeResult(result), fmt.Errorf("append main decision note after verification event %s; writebackStatus=%s and retry is safe: %w", verificationApply.EventID, result.WritebackStatus, err)
	}
	if err := ensureExpectedAppendResult(caseRoot, "decision", decisionApply); err != nil {
		result.WritebackStatus = "event-id-collision"
		result.OrchestrationSnapshot.ShardStatusAfter = result.WritebackStatus
		result.ReadyForWriteback = false
		result.BlockedReasons = append(result.BlockedReasons, err.Error())
		return finalizeReviewerIntakeResult(result), err
	}
	result.Applied = verificationApply.Applied || decisionApply.Applied
	result.WritebackStatus = "complete"
	if !verificationApply.Applied && !decisionApply.Applied {
		result.WritebackStatus = "already-complete"
	}
	result.OrchestrationSnapshot.ShardStatusAfter = result.WritebackStatus
	validation, err := reviewerPostValidation(repoRoot, caseRoot, pack, lane, true)
	if err != nil {
		result.WritebackStatus = "complete-post-validation-failed"
		result.NextSteps = []string{"ledger writeback completed; rerun the identical reviewer intake apply or run overview/handoff/doctor to recover post-validation snapshots"}
		return finalizeReviewerIntakeResult(result), fmt.Errorf("reviewer ledger writeback completed but post-validation failed; writebackStatus=%s: %w", result.WritebackStatus, err)
	}
	result.PostValidation = &validation
	result.NextSteps = []string{"consume postValidation overview/handoff/doctor snapshots", "continue or hand off the lane only when postValidation shows the expected blocker state"}
	return finalizeReviewerIntakeResult(result), nil
}

func finalizeReviewerIntakeResult(result ReviewerIntakeResult) ReviewerIntakeResult {
	if reviewerIntakeNeedsRepairGuidance(result.WritebackStatus) && len(result.RepairGuidance) == 0 {
		result.RepairGuidance = reviewerIntakeRepairGuidance(result)
	}
	action := reviewerIntakeMissionCommanderAction(result)
	result.MissionCommanderAction = action
	result.MissionCommanderNextActions = reviewerIntakeMissionCommanderNextActions(result, action)
	result.MissionCommanderActionQueue = mission.MissionCommanderActionQueueFor(result.MissionCommanderNextActions)
	result.Summary = reviewerIntakeSummary(result)
	return result
}

func reviewerIntakeSummary(result ReviewerIntakeResult) ReviewerIntakeSummary {
	summary := ReviewerIntakeSummary{
		Status:                result.WritebackStatus,
		ReadyForWriteback:     result.ReadyForWriteback,
		Applied:               result.Applied,
		Lane:                  result.Lane,
		ShardID:               result.ShardID,
		IntakeID:              result.IntakeID,
		ReviewerSession:       result.ReviewerSession,
		VerificationVerdict:   result.VerificationVerdict,
		MainDecision:          result.MainDecision,
		DispatchIndex:         result.OrchestrationSnapshot.DispatchIndex,
		DispatchTotal:         result.OrchestrationSnapshot.DispatchTotal,
		ShardStatusBefore:     result.OrchestrationSnapshot.ShardStatusBefore,
		ShardStatusAfter:      result.OrchestrationSnapshot.ShardStatusAfter,
		NextDispatches:        append([]string{}, result.OrchestrationSnapshot.NextDispatches...),
		OrchestrationProgress: reviewerIntakeOrchestrationProgress(result),
		BlockedCount:          len(result.BlockedReasons),
		RepairGuidanceCount:   len(result.RepairGuidance),
		Boundary: []string{
			"intake summary is read-only; full reviewer result, note writebacks, orchestration snapshot, postValidation, and action queue remain available",
			"reviewer intake must run -WhatIf before -Apply and must not execute heavy tools",
			"reviewer intake does not write authority/confirmed state",
		},
	}
	if result.PostValidation != nil {
		summary.PostValidationPresent = true
		summary.PostValidationValid = result.PostValidation.Summary.Valid
		summary.PostValidationOverviewVerifications = result.PostValidation.Summary.OverviewVerifications
		summary.PostValidationOverviewDecisions = result.PostValidation.Summary.OverviewDecisions
		summary.ReviewerWritebacks = result.PostValidation.Summary.ReviewerWritebacks
		summary.ReviewerWritebackSummary = result.PostValidation.Summary.ReviewerWritebackSummary
	}
	queue := result.MissionCommanderActionQueue
	summary.QueueSummary = strings.TrimSpace(queue.Summary)
	summary.ActionTotal = queue.Counts.Total
	summary.ActionUnblocked = queue.Counts.Unblocked
	summary.ActionBlocked = queue.Counts.Blocked
	summary.ActionRequiresReview = queue.Counts.RequiresReview
	summary.ActionFollowUp = queue.Counts.FollowUp
	if len(result.RepairGuidance) > 0 {
		repairSummary := reviewerIntakeRepairGuidanceSummary(result.RepairGuidance, result.MissionCommanderAction.PrimaryCommand)
		summary.RepairGuidanceSummary = &repairSummary
	}
	if queue.CurrentAction != nil {
		current := reviewerIntakeNextActionSummary(*queue.CurrentAction)
		summary.CurrentAction = &current
	}
	for _, item := range result.MissionCommanderNextActions {
		summary.NextActions = append(summary.NextActions, reviewerIntakeNextActionSummary(item))
	}
	if len(result.BlockedReasons) > 0 || len(result.RepairGuidance) > 0 {
		summary.Boundary = append(summary.Boundary, "do not apply reviewer intake while blockedReasons or repairGuidance remain unresolved")
	}
	if result.PostValidation != nil {
		summary.Boundary = append(summary.Boundary, "consume postValidation summary before continuing or handing off the lane")
	}
	return summary
}

func reviewerIntakeNextActionSummary(item mission.MissionCommanderNextActionItem) ReviewerIntakeNextActionSummary {
	return ReviewerIntakeNextActionSummary{
		Lane:           item.Lane,
		Label:          item.Label,
		GateEventID:    item.GateEventID,
		ActionID:       item.ActionID,
		State:          item.State,
		Source:         item.Source,
		Command:        item.Command,
		Blocked:        item.Blocked,
		RequiresReview: item.RequiresReview,
		Reasons:        append([]string{}, item.Reasons...),
		Boundary:       append([]string{}, item.Boundary...),
	}
}

func reviewerIntakeOrchestrationProgress(result ReviewerIntakeResult) *ReviewerIntakeOrchestrationProgress {
	snapshot := result.OrchestrationSnapshot
	currentShardID := strings.TrimSpace(result.ShardID)
	if snapshot.DispatchTotal == 0 && snapshot.DispatchIndex == 0 && currentShardID == "" && len(snapshot.NextDispatches) == 0 {
		return nil
	}
	currentComplete := reviewerIntakeShardCountsAsComplete(result.WritebackStatus)
	remaining := append([]string{}, snapshot.NextDispatches...)
	if !currentComplete && currentShardID != "" {
		remaining = append([]string{currentShardID}, remaining...)
	}
	remaining = mission.UniqueStrings(cleanRepairGuidanceStrings(remaining))
	progress := ReviewerIntakeOrchestrationProgress{
		DispatchIndex:      snapshot.DispatchIndex,
		DispatchTotal:      snapshot.DispatchTotal,
		CurrentShardID:     currentShardID,
		CurrentShardStatus: strings.TrimSpace(snapshot.ShardStatusAfter),
		Open:               len(remaining),
		RemainingShardIDs:  remaining,
		Boundary: []string{
			"orchestration progress is read-only; main Agent remains responsible for reviewer dispatch and intake ordering",
			"remainingShardIds includes the current shard until verification-before-decision writeback completes",
		},
	}
	if progress.DispatchTotal <= 0 {
		progress.DispatchTotal = len(progress.RemainingShardIDs)
		if currentComplete && progress.DispatchIndex > progress.DispatchTotal {
			progress.DispatchTotal = progress.DispatchIndex
		}
	}
	if progress.DispatchTotal > 0 {
		progress.Completed = progress.DispatchTotal - progress.Open
	}
	if progress.Completed < 0 {
		progress.Completed = 0
	}
	if progress.DispatchTotal > 0 && progress.Completed > progress.DispatchTotal {
		progress.Completed = progress.DispatchTotal
	}
	if progress.DispatchTotal > 0 && progress.Open > progress.DispatchTotal {
		progress.Open = progress.DispatchTotal
	}
	if len(progress.RemainingShardIDs) > 0 {
		progress.NextOpenShardID = progress.RemainingShardIDs[0]
	}
	progress.Boundary = mission.UniqueStrings(cleanRepairGuidanceStrings(progress.Boundary))
	return &progress
}

func reviewerIntakeShardCountsAsComplete(status string) bool {
	switch strings.TrimSpace(status) {
	case "complete", "already-complete":
		return true
	default:
		return false
	}
}

func reviewerIntakeMissionCommanderAction(result ReviewerIntakeResult) mission.MissionCommanderAction {
	status := strings.TrimSpace(result.WritebackStatus)
	previewCommand := reviewerIntakeCommand(result, false)
	applyCommand := reviewerIntakeCommand(result, true)
	handoffCommand := "/rekit handoff -Lane " + result.Lane
	continuePreviewCommand := "/rekit continue -Lane " + result.Lane + " -WhatIf"
	boundary := reviewerIntakeCommanderBoundary(result)
	switch status {
	case "previewed":
		return mission.MissionCommanderAction{
			State:            "ready-for-reviewer-intake-apply",
			Prompt:           fmt.Sprintf("reviewer intake `%s` 已通过 WhatIf；主 Agent 复核 verification/decision/postValidation 后可 apply。", result.IntakeID),
			PrimaryCommand:   applyCommand,
			FollowUpCommands: []string{handoffCommand},
			Boundary:         boundary,
		}
	case "complete":
		primary, followUp := reviewerIntakePostValidationCommands(result)
		if primary == "" {
			primary = handoffCommand
		}
		return mission.MissionCommanderAction{
			State:            "reviewer-intake-writeback-complete",
			Prompt:           fmt.Sprintf("reviewer intake `%s` 已完成 verification-before-decision writeback；按 postValidation 的 lane next action 接续。", result.IntakeID),
			PrimaryCommand:   primary,
			FollowUpCommands: followUp,
			Boundary:         boundary,
		}
	case "already-complete":
		primary, followUp := reviewerIntakePostValidationCommands(result)
		if primary == "" {
			primary = handoffCommand
			followUp = append(followUp, continuePreviewCommand)
		}
		return mission.MissionCommanderAction{
			State:            "reviewer-intake-already-complete",
			Prompt:           fmt.Sprintf("reviewer intake `%s` 已由 deterministic event IDs 完成；不要重复写 ledger，直接消费 postValidation。", result.IntakeID),
			PrimaryCommand:   primary,
			FollowUpCommands: followUp,
			Boundary:         append(boundary, "duplicate reviewer intake does not append verification or decision events"),
		}
	case "verification-recorded":
		return mission.MissionCommanderAction{
			State:            "reviewer-intake-partial-writeback",
			Prompt:           fmt.Sprintf("reviewer intake `%s` 已记录 verification 但 decision 尚未完成；重跑同一个 apply command 以幂等恢复。", result.IntakeID),
			PrimaryCommand:   applyCommand,
			FollowUpCommands: []string{handoffCommand},
			Boundary:         append(boundary, "retry the identical apply command; do not hand-write the missing decision event"),
		}
	case "blocked", "event-id-collision":
		state := "reviewer-intake-blocked"
		if status == "event-id-collision" {
			state = "reviewer-intake-event-id-collision"
		}
		return mission.MissionCommanderAction{
			State:          state,
			Prompt:         fmt.Sprintf("reviewer intake `%s` 当前不可写回；先解决 blockedReasons，再重新预览。", result.IntakeID),
			PrimaryCommand: previewCommand,
			Boundary:       append(boundary, "do not apply reviewer intake while blockedReasons are present"),
		}
	case "complete-post-validation-failed":
		return mission.MissionCommanderAction{
			State:            "reviewer-intake-post-validation-failed",
			Prompt:           fmt.Sprintf("reviewer intake `%s` ledger writeback 已完成但 postValidation 失败；先恢复 overview/handoff/doctor snapshot。", result.IntakeID),
			PrimaryCommand:   "/rekit overview",
			FollowUpCommands: []string{handoffCommand, applyCommand},
			Boundary:         append(boundary, "do not continue the lane until postValidation is available"),
		}
	default:
		return mission.MissionCommanderAction{
			State:            "reviewer-intake-" + textOr(status, "validated"),
			Prompt:           fmt.Sprintf("reviewer intake `%s` 已校验；先运行 WhatIf 预览，再决定是否 apply。", result.IntakeID),
			PrimaryCommand:   previewCommand,
			FollowUpCommands: []string{applyCommand},
			Boundary:         boundary,
		}
	}
}

func reviewerIntakeRepairGuidanceSummary(guidance []ReviewerIntakeRepairGuidance, nextSafeCommand string) ReviewerIntakeRepairGuidanceSummary {
	summary := ReviewerIntakeRepairGuidanceSummary{Total: len(guidance), NextSafeCommand: strings.TrimSpace(nextSafeCommand)}
	for _, item := range guidance {
		if summary.PrimaryReason == "" {
			summary.PrimaryReason = strings.TrimSpace(item.Reason)
		}
		if summary.PrimaryAction == "" {
			summary.PrimaryAction = strings.TrimSpace(item.Action)
		}
		summary.Evidence = append(summary.Evidence, item.Evidence...)
		summary.Boundary = append(summary.Boundary, item.Boundary...)
	}
	summary.Evidence = mission.UniqueStrings(cleanRepairGuidanceStrings(summary.Evidence))
	summary.Boundary = mission.UniqueStrings(cleanRepairGuidanceStrings(summary.Boundary))
	return summary
}

func reviewerIntakeNeedsRepairGuidance(status string) bool {
	switch strings.TrimSpace(status) {
	case "blocked", "event-id-collision", "complete-post-validation-failed":
		return true
	default:
		return false
	}
}

func reviewerIntakeRepairGuidance(result ReviewerIntakeResult) []ReviewerIntakeRepairGuidance {
	guidance := []ReviewerIntakeRepairGuidance{}
	for _, reason := range result.BlockedReasons {
		reason = strings.TrimSpace(reason)
		if reason == "" {
			continue
		}
		guidance = append(guidance, reviewerIntakeRepairGuidanceForReason(reason, result))
	}
	if len(guidance) == 0 && reviewerIntakeNeedsRepairGuidance(result.WritebackStatus) {
		guidance = append(guidance, reviewerIntakeRepairGuidanceForReason(strings.TrimSpace(result.WritebackStatus), result))
	}
	return guidance
}

func reviewerIntakeRepairGuidanceForReason(reason string, result ReviewerIntakeResult) ReviewerIntakeRepairGuidance {
	lower := strings.ToLower(reason)
	item := ReviewerIntakeRepairGuidance{Reason: reason, Boundary: []string{"do not apply reviewer intake until this blocker is resolved", "do not write authority/confirmed or execute heavy tools from reviewer intake"}}
	switch {
	case strings.Contains(lower, "evidenceref") || strings.Contains(lower, "evidencerefs") || strings.Contains(lower, "no inspectable evidence"):
		item.Action = "add a non-empty case-local bounded evidence file, or cite the packetId when the packet itself is the reviewed evidence; rerun reviewer intake -WhatIf before -Apply"
		item.Evidence = append([]string{}, result.ReviewerResult.EvidenceRefs...)
		if len(item.Evidence) == 0 && result.ReviewerResult.RouteOutput != nil {
			if ref := routeOutputString(result.ReviewerResult.RouteOutput, "evidence"); ref != "" {
				item.Evidence = append(item.Evidence, ref)
			}
		}
	case strings.Contains(lower, "unresolved conflicts"):
		item.Action = "resolve or split the conflicted reviewer shard, then write a new conflict-free ReviewerResult and rerun -WhatIf"
		item.Evidence = append([]string{}, result.ReviewerResult.Conflicts...)
	case strings.Contains(lower, "blocked write") || strings.Contains(lower, "heavy-tool") || strings.Contains(lower, "authority/confirmed") || strings.Contains(lower, "external-effect"):
		item.Action = "replace requested reviewer routeOutput with read-only main-agent handoff; any write, heavy-tool, authority/confirmed, or external-effect action needs a separate gate"
		for _, key := range []string{"tool_scope", "next_action"} {
			if value := routeOutputString(result.ReviewerResult.RouteOutput, key); value != "" {
				item.Evidence = append(item.Evidence, key+"="+value)
			}
		}
	case strings.Contains(lower, "recommendedverdict") || strings.Contains(lower, "mapped verification verdict"):
		item.Action = "align recommendedVerdict with the mapped reviewer decision verdict, or change reviewer decision and rerun -WhatIf"
		item.Evidence = []string{"recommendedVerdict=" + result.ReviewerResult.RecommendedVerdict, "mappedVerdict=" + result.VerificationVerdict, "reviewerDecision=" + result.ReviewerResult.Decision}
	case strings.Contains(lower, "low-confidence"):
		item.Action = "collect independent evidence or dispatch a smaller read-only reviewer shard before accepting/rejecting this result"
		item.Evidence = []string{"confidence=" + result.ReviewerResult.Confidence, "reviewerDecision=" + result.ReviewerResult.Decision}
	case strings.Contains(lower, "event-id") || strings.Contains(lower, "eventid") || strings.Contains(lower, "collision"):
		item.Action = "inspect the existing ledger event for this deterministic eventId and rerun only after the conflicting event is resolved by the main Agent"
		item.Evidence = []string{"intakeId=" + result.IntakeID}
	case strings.Contains(lower, "post-validation"):
		item.Action = "rerun overview, handoff, and doctor to recover post-validation snapshots before continuing the lane"
		item.Evidence = []string{"intakeId=" + result.IntakeID}
	default:
		item.Action = "inspect blockedReasons and reviewer result provenance, then rerun reviewer intake -WhatIf after repair"
		item.Evidence = []string{"intakeId=" + result.IntakeID}
	}
	item.Evidence = mission.UniqueStrings(cleanRepairGuidanceStrings(item.Evidence))
	item.Boundary = mission.UniqueStrings(cleanRepairGuidanceStrings(item.Boundary))
	return item
}

func cleanRepairGuidanceStrings(values []string) []string {
	out := []string{}
	for _, value := range values {
		value = strings.Join(strings.Fields(strings.TrimSpace(value)), " ")
		if value != "" {
			out = append(out, value)
		}
	}
	return out
}

func reviewerIntakeMissionCommanderNextActions(result ReviewerIntakeResult, action mission.MissionCommanderAction) []mission.MissionCommanderNextActionItem {
	status := strings.TrimSpace(result.WritebackStatus)
	if status == "complete" || status == "already-complete" {
		if items := reviewerIntakePostValidationNextActions(result); len(items) > 0 {
			return mission.UniqueCommanderNextActions(items)
		}
	}
	items := []mission.MissionCommanderNextActionItem{}
	blocked := reviewerIntakeNextActionBlocked(result)
	requiresReview := reviewerIntakeNextActionRequiresReview(result)
	if action.PrimaryCommand != "" {
		items = append(items, mission.MissionCommanderNextActionItem{
			Lane:           result.Lane,
			Label:          reviewerIntakeLaneLabel(result.Lane),
			State:          action.State,
			Command:        action.PrimaryCommand,
			Source:         "reviewerIntake." + textOr(status, "validated"),
			Blocked:        blocked,
			RequiresReview: requiresReview,
			Reasons:        reviewerIntakeNextActionReasons(result),
			Boundary:       append([]string{}, action.Boundary...),
		})
	}
	for _, command := range action.FollowUpCommands {
		items = append(items, mission.MissionCommanderNextActionItem{
			Lane:           result.Lane,
			Label:          reviewerIntakeLaneLabel(result.Lane),
			State:          action.State,
			Command:        command,
			Source:         "reviewerIntake." + textOr(status, "validated") + ".followUp",
			Blocked:        blocked,
			RequiresReview: requiresReview,
			Reasons:        append(reviewerIntakeNextActionReasons(result), "follow-up is available only after the reviewer intake primary action is satisfied"),
			Boundary:       append([]string{}, action.Boundary...),
		})
	}
	return mission.UniqueCommanderNextActions(items)
}

func reviewerIntakePostValidationNextActions(result ReviewerIntakeResult) []mission.MissionCommanderNextActionItem {
	if result.PostValidation == nil {
		return nil
	}
	items := []mission.MissionCommanderNextActionItem{}
	for _, item := range result.PostValidation.Handoff.MissionCommanderNextActions {
		copyItem := item
		copyItem.Source = "reviewerIntake.postValidation." + textOr(item.Source, "handoff")
		copyItem.Reasons = append([]string{"post-review validation passed; consume returned overview/handoff/doctor snapshots"}, item.Reasons...)
		items = append(items, copyItem)
	}
	return items
}

func reviewerIntakePostValidationCommands(result ReviewerIntakeResult) (string, []string) {
	items := reviewerIntakePostValidationNextActions(result)
	if len(items) == 0 {
		return "", nil
	}
	primary := items[0].Command
	followUp := []string{}
	for _, item := range items[1:] {
		followUp = append(followUp, item.Command)
	}
	return primary, mission.UniqueStrings(followUp)
}

func reviewerIntakeNextActionBlocked(result ReviewerIntakeResult) bool {
	switch strings.TrimSpace(result.WritebackStatus) {
	case "blocked", "event-id-collision", "complete-post-validation-failed":
		return true
	default:
		return false
	}
}

func reviewerIntakeNextActionRequiresReview(result ReviewerIntakeResult) bool {
	switch strings.TrimSpace(result.WritebackStatus) {
	case "complete", "already-complete":
		return false
	default:
		return true
	}
}

func reviewerIntakeNextActionReasons(result ReviewerIntakeResult) []string {
	reasons := []string{}
	if result.IntakeID != "" {
		reasons = append(reasons, "reviewer intake "+result.IntakeID)
	}
	reasons = append(reasons, result.BlockedReasons...)
	for _, repair := range result.RepairGuidance {
		if strings.TrimSpace(repair.Action) != "" {
			reasons = append(reasons, "repair: "+repair.Action)
		}
	}
	switch strings.TrimSpace(result.WritebackStatus) {
	case "previewed":
		reasons = append(reasons, "WhatIf preview passed; apply only after main-agent evidence review")
	case "verification-recorded":
		reasons = append(reasons, "verification event was recorded; identical retry completes missing decision idempotently")
	case "complete", "already-complete":
		reasons = append(reasons, "verification-before-decision writeback is complete")
	}
	return mission.UniqueStrings(reasons)
}

func reviewerIntakeCommanderBoundary(result ReviewerIntakeResult) []string {
	boundary := []string{
		"reviewer intake is main-agent owned; reviewer output alone is not a ledger event",
		"run -WhatIf and inspect cited evidenceRefs before -Apply",
		"do not write authority/confirmed state from reviewer intake",
		"do not execute heavy tools from reviewer intake",
	}
	if result.WritebackStatus == "previewed" {
		boundary = append(boundary, "apply only if verification and decision previews match the reviewed shard")
	}
	return mission.UniqueStrings(boundary)
}

func reviewerIntakeCommand(result ReviewerIntakeResult, apply bool) string {
	mode := "-WhatIf"
	if apply {
		mode = "-Apply"
	}
	return "/rekit plan-subagents -Target " + quoteCommandArg(result.CaseRoot) + " -Pack " + quoteCommandArg(result.Pack) + " -PacketPath " + quoteCommandArg(result.PacketPath) + " -ReviewerResultPath " + quoteCommandArg(result.ReviewerResultPath) + " -Lane " + quoteCommandArg(result.Lane) + " -Actor " + quoteCommandArg(textOr(result.Actor, "<main-agent>")) + " " + mode + " -Format json"
}

func reviewerIntakeLaneLabel(lane string) string {
	return mission.BoardLaneLabel(mission.BoardLane{ID: lane})
}

func validatePacketIntegrity(_ string, packetPath string, packet Packet, packetBytes []byte) error {
	integrityPath := filepath.Join(filepath.Dir(packetPath), "packet.integrity.json")
	if packet.PacketIntegrity == nil {
		if _, err := os.Lstat(integrityPath); err == nil {
			return fmt.Errorf("review packet integrity reference is missing while canonical sidecar exists")
		} else if !os.IsNotExist(err) {
			return fmt.Errorf("inspect review packet integrity: %w", err)
		}
		return nil
	}
	if !strings.EqualFold(strings.TrimSpace(packet.PacketIntegrity.Algorithm), "sha256") || !samePath(packet.PacketIntegrity.Path, integrityPath) {
		return fmt.Errorf("review packet integrity reference is not canonical")
	}
	data, err := readStableReviewerArtifact(filepath.Dir(packetPath), integrityPath, "review packet integrity", maxReviewPacketBytes)
	if err != nil {
		return err
	}
	dec := json.NewDecoder(bytes.NewReader(bytes.TrimSpace(data)))
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

func readBoundedFile(path, label string, limit int64) ([]byte, error) {
	file, err := os.Open(path)
	if err != nil {
		return nil, fmt.Errorf("read %s: %w", label, err)
	}
	defer file.Close()
	data, err := io.ReadAll(io.LimitReader(file, limit+1))
	if err != nil {
		return nil, fmt.Errorf("read %s: %w", label, err)
	}
	if int64(len(data)) > limit {
		return nil, fmt.Errorf("%s exceeds %d-byte limit", label, limit)
	}
	return data, nil
}

func decodeIntakePacket(data []byte) (Packet, error) {
	dec := json.NewDecoder(bytes.NewReader(bytes.TrimSpace(data)))
	dec.DisallowUnknownFields()
	var packet Packet
	if err := dec.Decode(&packet); err != nil {
		return Packet{}, fmt.Errorf("decode review packet: %w", err)
	}
	var trailing any
	if err := dec.Decode(&trailing); err != io.EOF {
		return Packet{}, fmt.Errorf("review packet must contain exactly one JSON object")
	}
	return packet, nil
}

func decodeReviewerResult(data []byte) (ReviewerResult, error) {
	return reviewerresult.Decode(data)
}

func validateRouteOutput(outputContract string, routeOutput map[string]any) error {
	return reviewersession.ValidateRouteOutput(splitCSV(outputContract), routeOutput)
}

func validateRouteOutputBindings(result ReviewerResult) error {
	for _, binding := range []struct {
		field string
		want  string
	}{
		{field: "decision", want: result.Decision},
		{field: "confidence", want: result.Confidence},
	} {
		value, ok := result.RouteOutput[binding.field]
		if !ok {
			continue
		}
		text, ok := value.(string)
		if !ok {
			return fmt.Errorf("reviewer result routeOutput field %q must be a string", binding.field)
		}
		if !strings.EqualFold(strings.TrimSpace(text), strings.TrimSpace(binding.want)) {
			return fmt.Errorf("reviewer result routeOutput field %q %q does not match top-level value %q", binding.field, text, binding.want)
		}
	}
	if item := routeOutputString(result.RouteOutput, "item"); item != "" {
		for _, itemValue := range splitCSV(item) {
			if !slices.Contains(result.Items, itemValue) {
				return fmt.Errorf("reviewer result routeOutput item %q is outside top-level items %v", itemValue, result.Items)
			}
		}
	}
	if evidence := routeOutputString(result.RouteOutput, "evidence"); evidence != "" {
		for _, ref := range splitCSV(evidence) {
			if !slices.Contains(result.EvidenceRefs, ref) {
				return fmt.Errorf("reviewer result routeOutput evidence %q is not present in top-level evidenceRefs", ref)
			}
		}
	}
	return nil
}

func validateOwnerBinding(caseRoot string, packet Packet, packetPath string, packetBytes []byte) (OwnerBinding, *ReviewerPacketAdoption, error) {
	binding := packet.OwnerBinding
	if strings.TrimSpace(binding.TargetLane) == "" {
		return OwnerBinding{}, nil, fmt.Errorf("review packet ownerBinding.targetLane is required for reviewer intake")
	}
	if !binding.RequiredForIntake {
		return binding, nil, nil
	}
	if err := validateAdoptedOwnerStillCurrent(caseRoot, binding); err == nil {
		return binding, nil, nil
	}
	adoption, err := readReviewerPacketAdoption(caseRoot, packet, packetPath, packetBytes)
	if err != nil {
		return OwnerBinding{}, nil, err
	}
	if adoption == nil {
		board, boardErr := mission.ReadBoard(caseRoot)
		if boardErr != nil {
			return OwnerBinding{}, nil, fmt.Errorf("read board for reviewer owner binding validation: %w", boardErr)
		}
		lane, _ := mission.LookupBoardLane(board.Lanes, binding.TargetLane, false)
		return OwnerBinding{}, nil, fmt.Errorf("review packet ownerBinding is stale for lane %s: packet executor=%s generation=%d current executor=%s generation=%d; run reviewer packet adoption WhatIf then Apply", binding.TargetLane, textOr(binding.CurrentExecutor, "unassigned"), binding.ExecutorGeneration, textOr(lane.CurrentExecutor, "unassigned"), lane.ExecutorGeneration)
	}
	if err := validateAdoptedOwnerStillCurrent(caseRoot, adoption.AdoptedOwner); err != nil {
		return OwnerBinding{}, nil, fmt.Errorf("reviewer packet adoption is stale: %w", err)
	}
	return adoption.AdoptedOwner, adoption, nil
}

func validateAdoptedOwnerStillCurrent(caseRoot string, binding OwnerBinding) error {
	board, err := mission.ReadBoard(caseRoot)
	if err != nil {
		return fmt.Errorf("read board for reviewer owner binding validation: %w", err)
	}
	lane, ok := mission.LookupBoardLane(board.Lanes, binding.TargetLane, false)
	if !ok {
		return fmt.Errorf("review packet ownerBinding target lane %q is not present in current board", binding.TargetLane)
	}
	currentExecutor := strings.TrimSpace(lane.CurrentExecutor)
	expectedExecutor := strings.TrimSpace(binding.CurrentExecutor)
	if currentExecutor != expectedExecutor || lane.ExecutorGeneration != binding.ExecutorGeneration {
		return fmt.Errorf("owner binding is not current for lane %s: expected executor=%s generation=%d current executor=%s generation=%d", binding.TargetLane, textOr(binding.CurrentExecutor, "unassigned"), binding.ExecutorGeneration, textOr(currentExecutor, "unassigned"), lane.ExecutorGeneration)
	}
	return nil
}

func reviewerPacketAdoptionPath(caseRoot, packetID string) string {
	return filepath.Join(caseRoot, filepath.FromSlash(reviewerPacketAdoptionDirectory), strings.TrimSpace(packetID)+".json")
}

func reviewerPacketAdoptionCommand(packetPath, lane, actor, reason string, apply bool) string {
	parts := []string{"/rekit", "plan-subagents", "-PacketPath", quoteReviewerCommandArg(packetPath), "-AdoptReviewerPacket", "-Lane", quoteReviewerCommandArg(lane), "-Actor", quoteReviewerCommandArg(actor), "-Reason", quoteReviewerCommandArg(reason)}
	if apply {
		parts = append(parts, "-Apply")
	} else {
		parts = append(parts, "-WhatIf")
	}
	parts = append(parts, "-Format", "json")
	return strings.Join(parts, " ")
}

func reviewerPacketBatchPreviewCommand(packetPath, lane, actor string) string {
	return reviewerPacketBatchCommand(packetPath, lane, actor, false)
}

func reviewerPacketBatchApplyCommand(packetPath, lane, actor string) string {
	return reviewerPacketBatchCommand(packetPath, lane, actor, true)
}

func reviewerPacketBatchCommand(packetPath, lane, actor string, apply bool) string {
	mode := "-WhatIf"
	if apply {
		mode = "-Apply"
	}
	return strings.Join([]string{"/rekit", "plan-subagents", "-PacketPath", quoteReviewerCommandArg(packetPath), "-ReadyReviewerResults", "-Lane", quoteReviewerCommandArg(lane), "-Actor", quoteReviewerCommandArg(actor), mode, "-Format", "json"}, " ")
}

func quoteReviewerCommandArg(value string) string {
	value = strings.TrimSpace(value)
	if value == "" || !strings.ContainsAny(value, " \t\r\n\"") {
		return value
	}
	return strconv.Quote(value)
}

func sha256Hex(data []byte) string {
	sum := sha256.Sum256(data)
	return hex.EncodeToString(sum[:])
}

func rejectReviewerAdoptionSymlinkPath(root, path string, allowMissing bool) error {
	rootFull, err := filepath.Abs(root)
	if err != nil {
		return err
	}
	rootState, err := os.Lstat(rootFull)
	if os.IsNotExist(err) && allowMissing {
		return nil
	}
	if err != nil {
		return err
	}
	if rootState.Mode()&os.ModeSymlink != 0 {
		return fmt.Errorf("reviewer packet adoption metadata root must not be a symlink: %s", rootFull)
	}
	pathFull, err := filepath.Abs(path)
	if err != nil {
		return err
	}
	rel, err := filepath.Rel(rootFull, pathFull)
	if err != nil || rel == ".." || strings.HasPrefix(rel, ".."+string(filepath.Separator)) || filepath.IsAbs(rel) {
		return fmt.Errorf("reviewer packet adoption path escapes case metadata root: %s", path)
	}
	current := rootFull
	for part := range strings.SplitSeq(rel, string(filepath.Separator)) {
		if part == "" || part == "." {
			continue
		}
		current = filepath.Join(current, part)
		st, statErr := os.Lstat(current)
		if os.IsNotExist(statErr) && allowMissing {
			return nil
		}
		if statErr != nil {
			return statErr
		}
		if st.Mode()&os.ModeSymlink != 0 {
			return fmt.Errorf("reviewer packet adoption path must not traverse symlink: %s", current)
		}
	}
	return nil
}

func writeReviewerPacketAdoption(path string, adoption ReviewerPacketAdoption) error {
	root := filepath.Join(adoption.CaseRoot, ".rekit")
	if err := rejectReviewerAdoptionSymlinkPath(root, filepath.Dir(path), true); err != nil {
		return err
	}
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return err
	}
	if err := rejectReviewerAdoptionSymlinkPath(root, path, true); err != nil {
		return err
	}
	state, err := refsf.ClassifyNonEmptyRegularFile(path)
	if err != nil {
		return err
	}
	if state == refsf.RegularFileSymlink {
		return fmt.Errorf("reviewer packet adoption path %q must not be a symlink", path)
	}
	if state == refsf.RegularFileReady {
		data, readErr := readBoundedFile(path, "reviewer packet adoption", maxReviewerPacketAdoptionBytes)
		if readErr != nil {
			return readErr
		}
		existing, decodeErr := decodeReviewerPacketAdoption(data)
		if decodeErr != nil {
			return decodeErr
		}
		if existing.PacketID == adoption.PacketID && existing.PacketSHA256 == adoption.PacketSHA256 && existing.AdoptedOwner == adoption.AdoptedOwner {
			return nil
		}
		if !reviewerPacketAdoptionSameContract(existing, adoption) || existing.AdoptedOwner.ExecutorGeneration >= adoption.AdoptedOwner.ExecutorGeneration {
			return fmt.Errorf("reviewer packet adoption path %q already contains a different or newer adoption", path)
		}
		if err := archiveReviewerPacketAdoption(path, existing); err != nil {
			return err
		}
	}
	encoded, err := json.MarshalIndent(adoption, "", "  ")
	if err != nil {
		return err
	}
	tmp, err := os.CreateTemp(filepath.Dir(path), ".reviewer-adoption-*.tmp")
	if err != nil {
		return err
	}
	tmpPath := tmp.Name()
	defer os.Remove(tmpPath)
	if _, err = tmp.Write(append(encoded, '\n')); err != nil {
		tmp.Close()
		return err
	}
	if err = tmp.Sync(); err != nil {
		tmp.Close()
		return err
	}
	if err = tmp.Close(); err != nil {
		return err
	}
	if state == refsf.RegularFileReady && runtime.GOOS == "windows" {
		backupPath := path + ".previous"
		if previousState, err := refsf.ClassifyNonEmptyRegularFile(backupPath); err != nil {
			return err
		} else if previousState != refsf.RegularFileMissing {
			return fmt.Errorf("reviewer packet adoption recovery path %q already exists", backupPath)
		}
		if err := os.Rename(path, backupPath); err != nil {
			return err
		}
		if err := os.Rename(tmpPath, path); err != nil {
			_ = os.Rename(backupPath, path)
			return err
		}
		return os.Remove(backupPath)
	}
	return os.Rename(tmpPath, path)
}

func archiveReviewerPacketAdoption(path string, adoption ReviewerPacketAdoption) error {
	historyDir := filepath.Join(filepath.Dir(path), "history")
	root := filepath.Join(adoption.CaseRoot, ".rekit")
	if err := rejectReviewerAdoptionSymlinkPath(root, historyDir, true); err != nil {
		return err
	}
	if err := os.MkdirAll(historyDir, 0o755); err != nil {
		return err
	}
	historyPath := filepath.Join(historyDir, adoption.PacketID+"-generation-"+strconv.Itoa(adoption.AdoptedOwner.ExecutorGeneration)+".json")
	if err := rejectReviewerAdoptionSymlinkPath(root, historyPath, true); err != nil {
		return err
	}
	encoded, err := json.MarshalIndent(adoption, "", "  ")
	if err != nil {
		return err
	}
	file, err := os.OpenFile(historyPath, os.O_CREATE|os.O_EXCL|os.O_WRONLY, 0o600)
	if os.IsExist(err) {
		existing, readErr := readBoundedFile(historyPath, "reviewer packet adoption history", maxReviewerPacketAdoptionBytes)
		if readErr != nil {
			return readErr
		}
		decoded, decodeErr := decodeReviewerPacketAdoption(existing)
		if decodeErr != nil {
			return decodeErr
		}
		if decoded == adoption {
			return nil
		}
		return fmt.Errorf("reviewer packet adoption history path %q already contains a different adoption", historyPath)
	}
	if err != nil {
		return err
	}
	if _, err := file.Write(append(encoded, '\n')); err != nil {
		file.Close()
		return err
	}
	if err := file.Sync(); err != nil {
		file.Close()
		return err
	}
	return file.Close()
}

func reviewerPacketAdoptionSameContract(left, right ReviewerPacketAdoption) bool {
	return left.SchemaVersion == right.SchemaVersion &&
		left.Kind == right.Kind &&
		left.PacketID == right.PacketID &&
		samePath(left.PacketPath, right.PacketPath) &&
		left.PacketSHA256 == right.PacketSHA256 &&
		samePath(left.RepoRoot, right.RepoRoot) &&
		samePath(left.CaseRoot, right.CaseRoot) &&
		strings.EqualFold(left.Pack, right.Pack) &&
		left.Lane == right.Lane &&
		left.DispatchedOwner == right.DispatchedOwner &&
		left.NoSpawn && right.NoSpawn &&
		left.NoHeavyTool && right.NoHeavyTool &&
		left.NoAuthorityOrConfirmed && right.NoAuthorityOrConfirmed
}

func readReviewerPacketAdoption(caseRoot string, packet Packet, packetPath string, packetBytes []byte) (*ReviewerPacketAdoption, error) {
	path := reviewerPacketAdoptionPath(caseRoot, packet.PacketID)
	if err := rejectReviewerAdoptionSymlinkPath(filepath.Join(caseRoot, ".rekit"), path, true); err != nil {
		return nil, err
	}
	state, err := refsf.ClassifyNonEmptyRegularFile(path)
	if err != nil {
		return nil, err
	}
	if state == refsf.RegularFileMissing || state == refsf.RegularFileWaiting {
		return nil, nil
	}
	if state == refsf.RegularFileSymlink {
		return nil, fmt.Errorf("reviewer packet adoption path %q must not be a symlink", path)
	}
	data, err := readBoundedFile(path, "reviewer packet adoption", maxReviewerPacketAdoptionBytes)
	if err != nil {
		return nil, err
	}
	adoption, err := decodeReviewerPacketAdoption(data)
	if err != nil {
		return nil, err
	}
	if adoption.SchemaVersion != 1 || adoption.Kind != "reviewer-packet-owner-adoption" || adoption.PacketID != packet.PacketID || !samePath(adoption.PacketPath, packetPath) || adoption.PacketSHA256 != sha256Hex(packetBytes) || !samePath(adoption.RepoRoot, packet.RepoRoot) || !samePath(adoption.CaseRoot, caseRoot) || !strings.EqualFold(adoption.Pack, packet.Pack) || adoption.Lane != packet.TargetLane || adoption.DispatchedOwner != packet.OwnerBinding || !adoption.NoSpawn || !adoption.NoHeavyTool || !adoption.NoAuthorityOrConfirmed {
		return nil, fmt.Errorf("reviewer packet adoption does not bind the exact packet, case, pack, lane, owner, and runtime boundary")
	}
	if adoption.AdoptedOwner.TargetLane != packet.TargetLane || adoption.AdoptedOwner.BindingMode != "durable-lane-executor-adoption" || !adoption.AdoptedOwner.RequiredForIntake || adoption.AdoptedOwner.MainAgentSpawnOwner != packet.OwnerBinding.MainAgentSpawnOwner || adoption.AdoptedOwner.RuntimeSessionBoundary != packet.OwnerBinding.RuntimeSessionBoundary || strings.TrimSpace(adoption.AdoptedOwner.CurrentExecutor) == "" || adoption.AdoptedOwner.ExecutorGeneration <= packet.OwnerBinding.ExecutorGeneration {
		return nil, fmt.Errorf("reviewer packet adoption does not contain a valid replacement executor owner binding")
	}
	if strings.TrimSpace(adoption.Actor) == "" || strings.TrimSpace(adoption.Reason) == "" {
		return nil, fmt.Errorf("reviewer packet adoption requires non-empty actor and reason provenance")
	}
	if _, err := time.Parse(time.RFC3339Nano, adoption.CreatedAt); err != nil {
		return nil, fmt.Errorf("reviewer packet adoption createdAt is invalid: %w", err)
	}
	return &adoption, nil
}

func decodeReviewerPacketAdoption(data []byte) (ReviewerPacketAdoption, error) {
	dec := json.NewDecoder(bytes.NewReader(bytes.TrimSpace(data)))
	dec.DisallowUnknownFields()
	var adoption ReviewerPacketAdoption
	if err := dec.Decode(&adoption); err != nil {
		return ReviewerPacketAdoption{}, fmt.Errorf("decode reviewer packet adoption: %w", err)
	}
	var trailing any
	if err := dec.Decode(&trailing); err != io.EOF {
		return ReviewerPacketAdoption{}, fmt.Errorf("reviewer packet adoption must contain exactly one JSON object")
	}
	return adoption, nil
}

func validateIntakePacket(packet Packet, packetPath, repoRoot, caseRoot, pack string) error {
	if packet.SchemaVersion != 1 || !packetIdentityMatches(packet) || packet.Command != commandName || !packet.WritesReviewArtifacts || packet.IsMutation || !packet.ReviewRequired {
		return fmt.Errorf("review packet is not a supported non-mutating %s packet", commandName)
	}
	if !samePath(packet.RepoRoot, repoRoot) {
		return fmt.Errorf("review packet repoRoot %q does not match current repo %q", packet.RepoRoot, repoRoot)
	}
	if strings.TrimSpace(packet.PlanRoot) == "" || !samePath(packet.PlanRoot, caseRoot) {
		return fmt.Errorf("review packet planRoot %q does not match current case %q", packet.PlanRoot, caseRoot)
	}
	if strings.TrimSpace(packet.TargetLane) == "" {
		return fmt.Errorf("review packet targetLane is required for reviewer intake")
	}
	if strings.TrimSpace(packet.OwnerBinding.TargetLane) == "" || packet.OwnerBinding.TargetLane != packet.TargetLane {
		return fmt.Errorf("review packet ownerBinding.targetLane %q does not match targetLane %q", packet.OwnerBinding.TargetLane, packet.TargetLane)
	}
	if !strings.EqualFold(strings.TrimSpace(packet.Pack), strings.TrimSpace(pack)) {
		return fmt.Errorf("review packet pack %q does not match current pack %q", packet.Pack, pack)
	}
	if strings.TrimSpace(packet.Observability.PacketPath) == "" || !samePath(packet.Observability.PacketPath, packetPath) {
		return fmt.Errorf("review packet observability packetPath %q does not match current packet path %q", packet.Observability.PacketPath, packetPath)
	}
	return nil
}

func validateIntakePacketRoute(repoRoot, pack string, packet Packet) error {
	m, err := manifest.Load(repoRoot, pack)
	if err != nil {
		return err
	}
	if !samePath(packet.ManifestPath, m.ManifestPath) {
		return fmt.Errorf("review packet manifestPath %q does not match current pack manifest %q", packet.ManifestPath, m.ManifestPath)
	}
	for _, route := range m.SubagentRoutes {
		if !strings.EqualFold(strings.TrimSpace(route.ID), strings.TrimSpace(packet.Route.ID)) {
			continue
		}
		if packet.Route != toRoute(route) {
			return fmt.Errorf("review packet route %q does not match the current pack manifest route definition", packet.Route.ID)
		}
		if packet.MainAgentResponsibilities != packet.Route.MainAgentOwns || packet.SubagentPermissions != packet.Route.SubagentPermissions || packet.OutputContract != packet.Route.OutputContract {
			return fmt.Errorf("review packet top-level route contract does not match route %q", packet.Route.ID)
		}
		return nil
	}
	return fmt.Errorf("review packet route %q is not present in the current pack manifest", packet.Route.ID)
}

func reviewerIntakeBlockers(repoRoot, caseRoot string, packet Packet, result ReviewerResult, mapping ReviewerDecisionMapping) ([]string, error) {
	blocked := []string{}
	if len(result.EvidenceRefs) == 0 {
		blocked = append(blocked, "reviewer result has no inspectable evidenceRefs")
	} else {
		invalidRefs, err := invalidReviewerEvidenceRefs(caseRoot, packet, result.EvidenceRefs)
		if err != nil {
			return nil, err
		}
		for _, ref := range invalidRefs {
			blocked = append(blocked, fmt.Sprintf("reviewer result evidenceRef %q is not the packet id or a non-empty case-local bounded evidence file", ref))
		}
	}
	if len(result.Conflicts) > 0 {
		blocked = append(blocked, "reviewer result reports unresolved conflicts: "+strings.Join(result.Conflicts, "; "))
	}
	if reviewerResultRequestsBlockedAction(repoRoot, packet, result) {
		blocked = append(blocked, "reviewer result requests a blocked write, heavy-tool, authority/confirmed, or external-effect action")
	}
	if result.RecommendedVerdict != mapping.VerificationVerdict {
		blocked = append(blocked, fmt.Sprintf("recommendedVerdict %q conflicts with mapped verification verdict %q", result.RecommendedVerdict, mapping.VerificationVerdict))
	}
	if result.Confidence == "low" && (result.Decision == "accept" || result.Decision == "reject") {
		blocked = append(blocked, "low-confidence accept/reject cannot be written back without independent evidence review")
	}
	return blocked, nil
}

func invalidReviewerEvidenceRefs(caseRoot string, packet Packet, refs []string) ([]string, error) {
	invalid := []string{}
	caseReal, err := filepath.EvalSymlinks(caseRoot)
	if err != nil {
		return nil, fmt.Errorf("resolve case root for reviewer evidence: %w", err)
	}
	for _, ref := range refs {
		value := strings.TrimSpace(ref)
		if value == packet.PacketID {
			continue
		}
		if strings.ContainsAny(value, ",;\r\n") {
			invalid = append(invalid, value)
			continue
		}
		pathPart := strings.TrimSpace(strings.SplitN(value, "#", 2)[0])
		if pathPart == "" || filepath.IsAbs(pathPart) {
			invalid = append(invalid, value)
			continue
		}
		path, joinErr := refsf.SafeJoin(caseRoot, pathPart)
		if joinErr != nil {
			invalid = append(invalid, value)
			continue
		}
		pathReal, evalErr := filepath.EvalSymlinks(path)
		if evalErr != nil || !pathInside(caseReal, pathReal) {
			invalid = append(invalid, value)
			continue
		}
		st, statErr := os.Stat(pathReal)
		if statErr != nil || st.IsDir() || st.Size() == 0 || evidenceFileBlank(pathReal, st.Size()) {
			invalid = append(invalid, value)
		}
	}
	return invalid, nil
}

func evidenceFileBlank(path string, size int64) bool {
	if size > 4096 {
		return false
	}
	data, err := os.ReadFile(path)
	return err != nil || len(bytes.TrimSpace(data)) == 0
}

func reviewerResultRequestsBlockedAction(repoRoot string, packet Packet, result ReviewerResult) bool {
	if scope := strings.ToLower(strings.TrimSpace(routeOutputString(result.RouteOutput, "tool_scope"))); scope != "" && !isNA(scope) && scope != "read-only" {
		return true
	}
	blockedTerms := []string{"write file", "append ledger", "authority", "confirmed", "external effect", "external-effect", "run debugger", "dump memory", "patch binary", "write", "create", "save", "modify", "delete", "curl", "http", "network", "gdb", "debugger", "trace", "dump", "patch", "hook", "inject", "execute"}
	if m, err := manifest.Load(repoRoot, packet.Pack); err == nil {
		blockedTerms = append(blockedTerms, m.HeavyToolGateIDs()...)
	} else {
		return true
	}
	for _, key := range []string{"next_action", "tool_scope"} {
		text := strings.ToLower(routeOutputString(result.RouteOutput, key))
		for _, term := range blockedTerms {
			if containsActionTerm(text, term) {
				return true
			}
		}
	}
	return false
}

func routeOutputString(routeOutput map[string]any, key string) string {
	value, _ := routeOutput[key].(string)
	return strings.TrimSpace(value)
}

func containsActionTerm(text, term string) bool {
	term = strings.ToLower(strings.TrimSpace(term))
	if term == "" {
		return false
	}
	normalizedText := normalizeActionText(text)
	normalizedTerm := normalizeActionText(term)
	if normalizedTerm == "" {
		return false
	}
	if strings.Contains(normalizedTerm, " ") {
		return strings.Contains(normalizedText, normalizedTerm)
	}
	return slices.Contains(strings.Fields(normalizedText), normalizedTerm)
}

func normalizeActionText(value string) string {
	value = strings.ToLower(strings.TrimSpace(value))
	return strings.Join(strings.FieldsFunc(value, func(r rune) bool {
		return !(r >= 'a' && r <= 'z') && !(r >= '0' && r <= '9')
	}), " ")
}

func reviewerNoteOptions(packet Packet, result ReviewerResult, mapping ReviewerDecisionMapping, owner OwnerBinding, session reviewerSessionWritebackProvenance, lane, actor, intakeID, packetPath, resultPath string) (note.Options, note.Options) {
	target := strings.Join(result.Items, ",")
	evidence := strings.Join(result.EvidenceRefs, ",")
	verificationID := "evt-" + intakeID + "-verification"
	ownerGeneration := ""
	if owner.ExecutorGeneration > 0 {
		ownerGeneration = strconv.Itoa(owner.ExecutorGeneration)
	}
	verification := note.Options{
		Kind:                      "verification",
		Lane:                      lane,
		Subject:                   "reviewer verdict for " + result.ShardID,
		Summary:                   result.Summary,
		Actor:                     actor,
		Confidence:                result.Confidence,
		BatchID:                   intakeID,
		Target:                    target,
		Verifier:                  "manual-review",
		Verdict:                   mapping.VerificationVerdict,
		EvidenceRefs:              evidence,
		EventID:                   verificationID,
		PacketID:                  packet.PacketID,
		RouteID:                   packet.Route.ID,
		ShardID:                   result.ShardID,
		PacketPath:                packetPath,
		ReviewerResultPath:        resultPath,
		ReviewerSession:           result.ReviewerSession,
		ReviewerHarness:           session.ReviewerHarness,
		ReviewerDispatchID:        session.DispatchID,
		ReviewerDispatchPath:      session.DispatchPath,
		ReviewerDispatchSHA256:    session.DispatchSHA256,
		ReviewerCompletionPath:    session.CompletionPath,
		ReviewerCompletionSHA256:  session.CompletionSHA256,
		ReviewerResultInputPath:   session.ReviewerResultInputPath,
		ReviewerResultInputSHA256: session.ReviewerResultInputSHA256,
		OwnerExecutor:             owner.CurrentExecutor,
		OwnerGeneration:           ownerGeneration,
		OwnerBindingMode:          owner.BindingMode,
		OwnerBindingTarget:        owner.TargetLane,
		ReviewerDecision:          result.Decision,
		RecommendedVerdict:        result.RecommendedVerdict,
		ReviewerRisks:             result.Risks,
		ReviewerConflicts:         result.Conflicts,
		RouteOutput:               result.RouteOutput,
	}
	decision := note.Options{
		Kind:                      "decision",
		Lane:                      lane,
		Subject:                   "main merge decision for " + result.ShardID,
		Summary:                   "mapped reviewer decision " + result.Decision + ": " + result.Summary,
		Actor:                     actor,
		Confidence:                result.Confidence,
		Decision:                  mapping.MainDecision,
		Reason:                    "validated bounded reviewer intake " + intakeID,
		Status:                    reviewerDecisionStatus(mapping.MainDecision),
		BatchID:                   intakeID,
		Target:                    target,
		EvidenceRefs:              verificationID + "," + evidence,
		EventID:                   "evt-" + intakeID + "-decision",
		PacketID:                  packet.PacketID,
		RouteID:                   packet.Route.ID,
		ShardID:                   result.ShardID,
		PacketPath:                packetPath,
		ReviewerResultPath:        resultPath,
		ReviewerSession:           result.ReviewerSession,
		ReviewerHarness:           session.ReviewerHarness,
		ReviewerDispatchID:        session.DispatchID,
		ReviewerDispatchPath:      session.DispatchPath,
		ReviewerDispatchSHA256:    session.DispatchSHA256,
		ReviewerCompletionPath:    session.CompletionPath,
		ReviewerCompletionSHA256:  session.CompletionSHA256,
		ReviewerResultInputPath:   session.ReviewerResultInputPath,
		ReviewerResultInputSHA256: session.ReviewerResultInputSHA256,
		OwnerExecutor:             owner.CurrentExecutor,
		OwnerGeneration:           ownerGeneration,
		OwnerBindingMode:          owner.BindingMode,
		OwnerBindingTarget:        owner.TargetLane,
		ReviewerDecision:          result.Decision,
		RecommendedVerdict:        result.RecommendedVerdict,
		ReviewerRisks:             result.Risks,
		ReviewerConflicts:         result.Conflicts,
		RouteOutput:               result.RouteOutput,
	}
	return verification, decision
}

func reviewerDecisionStatus(decision string) string {
	switch strings.TrimSpace(decision) {
	case "accept", "reject", "supersede":
		return "resolved"
	default:
		return "open"
	}
}

func reviewerPostValidation(repoRoot, caseRoot, pack, lane string, allowInit bool) (ReviewerPostValidation, error) {
	if !allowInit {
		if _, err := os.Stat(filepath.Join(caseRoot, ".rekit", "board.json")); err != nil {
			if os.IsNotExist(err) {
				validation := ReviewerPostValidation{Valid: false}
				validation.Summary = reviewerPostValidationSummary(validation)
				return validation, nil
			}
			return ReviewerPostValidation{}, err
		}
	}
	overviewResult, err := overview.BuildInventory(repoRoot, caseRoot, pack)
	if err != nil {
		return ReviewerPostValidation{}, fmt.Errorf("post-review overview validation: %w", err)
	}
	handoffResult, err := workstream.HandoffPreview(repoRoot, caseRoot, pack, workstream.HandoffOptions{Selector: lane})
	if err != nil {
		return ReviewerPostValidation{}, fmt.Errorf("post-review handoff validation: %w", err)
	}
	doctorRows, err := doctor.Case(repoRoot, caseRoot, pack)
	if err != nil {
		return ReviewerPostValidation{}, fmt.Errorf("post-review doctor validation: %w", err)
	}
	validation := ReviewerPostValidation{Overview: overviewResult, Handoff: handoffResult, DoctorRows: doctorRows, Valid: true}
	validation.Summary = reviewerPostValidationSummary(validation)
	return validation, nil
}

func reviewerPostValidationSummary(validation ReviewerPostValidation) ReviewerPostValidationSummary {
	summary := ReviewerPostValidationSummary{
		Valid:                 validation.Valid,
		OverviewVerifications: validation.Overview.Sections.Verifications.Total,
		OverviewDecisions:     validation.Overview.Sections.Decisions.Total,
		DoctorRows:            len(validation.DoctorRows),
		Project:               validation.Handoff.Project,
		ReviewerWritebacks:    len(validation.Handoff.ReviewerWritebacks),
	}
	if summary.ReviewerWritebacks > 0 {
		writebackSummary := workstream.ReviewerWritebackSummaryFor(validation.Handoff.ReviewerWritebacks)
		summary.ReviewerWritebackSummary = &writebackSummary
	}
	if validation.Handoff.Lane != nil {
		summary.Lane = validation.Handoff.Lane.ID
	}
	if action := validation.Handoff.ExecutorAction; action != nil {
		summary.ExecutorActionPresent = true
		summary.ExecutorActionReady = action.Ready
		summary.ExecutorActionBlocked = action.Blocked
		summary.ExecutorActionState = action.MissionCommanderAction.State
	}
	queue := validation.Handoff.MissionCommanderActionQueue
	summary.QueueSummary = strings.TrimSpace(queue.Summary)
	if queue.CurrentAction != nil {
		current := reviewerPostValidationNextActionSummary(*queue.CurrentAction)
		summary.CurrentAction = &current
	}
	for _, item := range validation.Handoff.MissionCommanderNextActions {
		summary.NextActions = append(summary.NextActions, reviewerPostValidationNextActionSummary(item))
	}
	if validation.Valid {
		summary.Boundary = []string{
			"postValidation summary is read-only; full overview/handoff/doctor snapshots remain available",
			"consume currentAction and nextActions before continuing or handing off the lane",
			"reviewer intake does not write authority/confirmed state and does not execute heavy tools",
		}
	} else {
		summary.Boundary = []string{
			"postValidation summary is unavailable until the case Mission Commander board exists",
			"do not continue the lane until overview/handoff/doctor validation is available",
		}
	}
	return summary
}

func reviewerPostValidationNextActionSummary(item mission.MissionCommanderNextActionItem) ReviewerPostValidationNextActionSummary {
	return ReviewerPostValidationNextActionSummary{
		Lane:           item.Lane,
		Label:          item.Label,
		GateEventID:    item.GateEventID,
		ActionID:       item.ActionID,
		State:          item.State,
		Source:         item.Source,
		Command:        item.Command,
		Blocked:        item.Blocked,
		RequiresReview: item.RequiresReview,
		Reasons:        append([]string{}, item.Reasons...),
		Boundary:       append([]string{}, item.Boundary...),
	}
}

func reviewerDecisionMappingByDecision(decision string) (ReviewerDecisionMapping, bool) {
	for _, mapping := range reviewerDecisionMappings() {
		if mapping.ReviewerDecision == strings.TrimSpace(decision) {
			return mapping, true
		}
	}
	return ReviewerDecisionMapping{}, false
}

func reviewerOrchestrationIntake(caseRoot string, packet Packet, shardID, lane string, whatIf bool) ReviewerOrchestrationIntake {
	completed := reviewerCompletedShards(caseRoot, packet, lane)
	normalizedShard := strings.TrimSpace(shardID)
	before := "planned"
	if completed[normalizedShard] {
		before = "already-complete"
	}
	after := "previewed"
	if !whatIf {
		after = "writeback-attempted"
	}
	snapshot := ReviewerOrchestrationIntake{
		Mode:                 packet.ReviewerOrchestration.Mode,
		PacketPath:           packet.Observability.PacketPath,
		ResultRoot:           packet.Observability.ResultRoot,
		ReviewerCount:        len(packet.ReviewerOrchestration.Dispatches),
		DispatchTotal:        len(packet.ReviewerOrchestration.Dispatches),
		ShardStatusBefore:    before,
		ShardStatusAfter:     after,
		PreviewRequiredFirst: true,
		MainAgentOwns:        append([]string{}, packet.ReviewLoop.MainAgentOwns...),
		RuntimeBoundary:      append([]string{}, packet.Observability.BlockedActions...),
	}
	if len(packet.ReviewerOrchestration.Dispatches) > 0 {
		for idx, dispatch := range packet.ReviewerOrchestration.Dispatches {
			if dispatch.ShardID == normalizedShard {
				snapshot.DispatchIndex = idx + 1
				continue
			}
			if dispatch.Status == "planned" && !completed[dispatch.ShardID] {
				snapshot.NextDispatches = append(snapshot.NextDispatches, dispatch.ShardID)
			}
		}
	} else {
		for idx, shard := range packet.Shards {
			if shard.ID == normalizedShard {
				snapshot.DispatchIndex = idx + 1
				continue
			}
			if !completed[shard.ID] {
				snapshot.NextDispatches = append(snapshot.NextDispatches, shard.ID)
			}
		}
	}
	if snapshot.Mode == "" {
		snapshot.Mode = "manual-main-agent-intake"
	}
	if snapshot.PacketPath == "" {
		snapshot.PacketPath = packet.Observability.PacketPath
	}
	if snapshot.ResultRoot == "" {
		snapshot.ResultRoot = packet.Observability.ResultRoot
	}
	if snapshot.ReviewerCount == 0 {
		snapshot.ReviewerCount = len(packet.Shards)
		snapshot.DispatchTotal = len(packet.Shards)
	}
	return snapshot
}

func reviewerCompletedShards(caseRoot string, packet Packet, lane string) map[string]bool {
	verificationByShard := map[string]bool{}
	decisionByShard := map[string]bool{}
	for _, kind := range []struct {
		name string
		seen map[string]bool
	}{
		{name: "verification", seen: verificationByShard},
		{name: "decision", seen: decisionByShard},
	} {
		events, err := mission.ReadStrictFact(caseRoot, kind.name)
		if err != nil {
			continue
		}
		for _, event := range events {
			if mission.Value(event, "packetId") != packet.PacketID || mission.Value(event, "routeId") != packet.Route.ID || mission.Value(event, "lane") != strings.TrimSpace(lane) {
				continue
			}
			if shard := mission.Value(event, "shardId"); shard != "" {
				kind.seen[shard] = true
			}
		}
	}
	completed := map[string]bool{}
	for shard := range verificationByShard {
		if decisionByShard[shard] {
			completed[shard] = true
		}
	}
	return completed
}

func shardByID(shards []Shard, shardID string) (Shard, bool) {
	for _, shard := range shards {
		if shard.ID == strings.TrimSpace(shardID) {
			return shard, true
		}
	}
	return Shard{}, false
}

func reviewerIntakeID(packet Packet, result ReviewerResult, lane string) string {
	canonical := struct {
		PacketID string         `json:"packetId"`
		Result   ReviewerResult `json:"result"`
		Lane     string         `json:"lane"`
	}{PacketID: packet.PacketID, Result: result, Lane: strings.TrimSpace(lane)}
	encoded, err := json.Marshal(canonical)
	if err != nil {
		panic(err)
	}
	sum := sha256.Sum256(encoded)
	return "review-" + hex.EncodeToString(sum[:])[:16]
}

func reviewerResultMutationLockID(packetID, shardID string) string {
	return "reviewer-result-" + strings.TrimSpace(packetID) + "-" + strings.TrimSpace(shardID)
}

func requiredAbsolutePath(path, label string) (string, error) {
	value := strings.TrimSpace(path)
	if value == "" {
		flag := "ReviewerResultPath"
		if label == "review packet" {
			flag = "PacketPath"
		}
		return "", fmt.Errorf("reviewer intake requires -%s", flag)
	}
	full, err := filepath.Abs(value)
	if err != nil {
		return "", err
	}
	return full, nil
}

func samePath(left, right string) bool {
	leftFull, leftErr := filepath.Abs(left)
	rightFull, rightErr := filepath.Abs(right)
	if leftErr != nil || rightErr != nil {
		return false
	}
	leftClean := filepath.Clean(leftFull)
	rightClean := filepath.Clean(rightFull)
	if runtime.GOOS == "windows" {
		return strings.EqualFold(leftClean, rightClean)
	}
	return leftClean == rightClean
}

func pathInside(root, path string) bool {
	if samePath(root, path) {
		return true
	}
	rootClean := filepath.Clean(root)
	pathClean := filepath.Clean(path)
	if runtime.GOOS == "windows" {
		rootClean = strings.ToLower(rootClean)
		pathClean = strings.ToLower(pathClean)
	}
	rel, err := filepath.Rel(rootClean, pathClean)
	if err != nil {
		return false
	}
	return rel != ".." && !strings.HasPrefix(rel, ".."+string(filepath.Separator)) && !filepath.IsAbs(rel)
}

func isNA(value string) bool {
	switch strings.ToLower(strings.TrimSpace(value)) {
	case "", "n/a", "na", "none", "no-op", "noop":
		return true
	default:
		return false
	}
}

func acquireReviewerIntakeLock(caseRoot, key string) (func(), error) {
	metadataRoot := filepath.Join(caseRoot, ".rekit")
	lockDir := filepath.Join(metadataRoot, "locks")
	if err := rejectReviewerAdoptionSymlinkPath(metadataRoot, lockDir, true); err != nil {
		return nil, err
	}
	if err := os.MkdirAll(lockDir, 0o755); err != nil {
		return nil, err
	}
	lockPath := filepath.Join(lockDir, key+".lock")
	if err := rejectReviewerAdoptionSymlinkPath(metadataRoot, lockPath, true); err != nil {
		return nil, err
	}
	deadline := time.Now().Add(10 * time.Second)
	for {
		file, err := os.OpenFile(lockPath, os.O_CREATE|os.O_EXCL|os.O_WRONLY, 0o600)
		if err == nil {
			_, _ = fmt.Fprintf(file, "pid=%d\n", os.Getpid())
			_ = file.Close()
			return func() { _ = os.Remove(lockPath) }, nil
		}
		if !os.IsExist(err) {
			return nil, err
		}
		if time.Now().After(deadline) {
			return nil, fmt.Errorf("timed out waiting for reviewer intake lock %s", lockPath)
		}
		time.Sleep(25 * time.Millisecond)
	}
}

func existingReviewerWritebackBlockers(caseRoot string, packet Packet, result ReviewerResult, lane, intakeID string) ([]string, error) {
	blocked := []string{}
	for _, kind := range []string{"verification", "decision"} {
		events, err := mission.ReadStrictFact(caseRoot, kind)
		if err != nil {
			return nil, err
		}
		for _, event := range events {
			if !reviewerEventMatches(event, packet, result.ShardID, lane) {
				continue
			}
			if mission.Value(event, "batchId") != intakeID {
				blocked = append(blocked, fmt.Sprintf("existing reviewer %s event %q already records packet %s shard %s for lane %s", kind, mission.Value(event, "eventId"), packet.PacketID, result.ShardID, lane))
			}
		}
	}
	return blocked, nil
}

func reviewerEventMatches(event map[string]any, packet Packet, shardID, lane string) bool {
	return mission.Value(event, "packetId") == packet.PacketID && mission.Value(event, "routeId") == packet.Route.ID && mission.Value(event, "shardId") == shardID && mission.Value(event, "lane") == lane
}

func isDuplicateAppend(result note.AppendResult) bool {
	return result.Reason == "duplicate eventId"
}

func ensureExpectedAppendResult(caseRoot, kind string, result note.AppendResult) error {
	if !isDuplicateAppend(result) {
		return nil
	}
	event, ok, err := existingEventByID(caseRoot, kind, result.EventID)
	if err != nil {
		return err
	}
	if !ok {
		return fmt.Errorf("duplicate reviewer %s eventId %q exists outside expected %s ledger", kind, result.EventID, kind)
	}
	if diff := eventMismatch(result.Event, event); diff != "" {
		return fmt.Errorf("duplicate reviewer %s eventId %q does not match expected intake event: %s", kind, result.EventID, diff)
	}
	return nil
}

func existingEventByID(caseRoot, kind, eventID string) (map[string]any, bool, error) {
	events, err := mission.ReadStrictFact(caseRoot, kind)
	if err != nil {
		return nil, false, err
	}
	for _, event := range events {
		if mission.Value(event, "eventId") == eventID {
			return event, true, nil
		}
	}
	return nil, false, nil
}

func eventMismatch(expected map[string]any, existing map[string]any) string {
	for _, key := range []string{"schemaVersion", "kind", "lane", "subject", "summary", "actor", "risk", "related", "confidence", "decision", "reason", "status", "batchId", "evidenceRefs", "target", "verifier", "verdict", "action", "approvedBy", "scope", "expires", "eventId", "packetId", "routeId", "shardId", "packetPath", "reviewerResultPath", "reviewerSession", "reviewerHarness", "reviewerDispatchId", "reviewerDispatchReceiptPath", "reviewerDispatchReceiptSha256", "reviewerCompletionReceiptPath", "reviewerCompletionReceiptSha256", "reviewerResultInputPath", "reviewerResultInputSha256", "ownerExecutor", "ownerGeneration", "ownerBindingMode", "ownerBindingTarget"} {
		left := eventValue(expected, key)
		right := eventValue(existing, key)
		if left != right {
			return fmt.Sprintf("field %s got %q want %q", key, right, left)
		}
	}
	return ""
}

func eventValue(event map[string]any, key string) string {
	value, ok := event[key]
	if !ok || value == nil {
		return ""
	}
	switch typed := value.(type) {
	case string:
		return strings.TrimSpace(typed)
	case []string:
		parts := append([]string{}, typed...)
		for i := range parts {
			parts[i] = strings.TrimSpace(parts[i])
		}
		return strings.Join(parts, "\x1f")
	case []any:
		parts := make([]string, 0, len(typed))
		for _, item := range typed {
			parts = append(parts, strings.TrimSpace(fmt.Sprint(item)))
		}
		return strings.Join(parts, "\x1f")
	case map[string]any:
		keys := make([]string, 0, len(typed))
		for key := range typed {
			keys = append(keys, key)
		}
		sort.Strings(keys)
		parts := make([]string, 0, len(keys))
		for _, key := range keys {
			parts = append(parts, key+"="+strings.TrimSpace(fmt.Sprint(typed[key])))
		}
		return strings.Join(parts, "\x1f")
	case map[string]string:
		keys := make([]string, 0, len(typed))
		for key := range typed {
			keys = append(keys, key)
		}
		sort.Strings(keys)
		parts := make([]string, 0, len(keys))
		for _, key := range keys {
			parts = append(parts, key+"="+strings.TrimSpace(typed[key]))
		}
		return strings.Join(parts, "\x1f")
	case float64:
		if typed == float64(int64(typed)) {
			return fmt.Sprintf("%d", int64(typed))
		}
		return fmt.Sprint(typed)
	default:
		return strings.TrimSpace(fmt.Sprint(typed))
	}
}
