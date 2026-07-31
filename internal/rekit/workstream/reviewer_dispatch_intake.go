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
	"github.com/shuiyu486/re-context-kits/internal/rekit/reviewerresult"
	"github.com/shuiyu486/re-context-kits/internal/rekit/reviewersession"
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
	SourcePath           string `json:"sourcePath,omitempty"`
	SourcePathArgument   string `json:"sourcePathArgument"`
	SourceCaptureInput   string `json:"sourceCaptureInput,omitempty"`
	InputSaveCommand     string `json:"inputSaveCommand,omitempty"`
	InputSaveApply       string `json:"inputSaveApply,omitempty"`
	SourceCaptureCommand string `json:"sourceCaptureCommand,omitempty"`
	SourceCaptureApply   string `json:"sourceCaptureApply,omitempty"`
	PreviewCommand       string `json:"previewCommand"`
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
	ReviewerResultInputPath                  string                            `json:"reviewerResultInputPath,omitempty"`
	ReviewerResultInputState                 string                            `json:"reviewerResultInputState,omitempty"`
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
	ReviewerDispatchID                       string                            `json:"reviewerDispatchId,omitempty"`
	ReviewerDispatchReceiptPath              string                            `json:"reviewerDispatchReceiptPath,omitempty"`
	ReviewerDispatchReceiptSHA256            string                            `json:"reviewerDispatchReceiptSha256,omitempty"`
	ReviewerHarness                          string                            `json:"reviewerHarness,omitempty"`
	ReviewerSession                          string                            `json:"reviewerSession,omitempty"`
	ReviewerSessionOutcome                   string                            `json:"reviewerSessionOutcome,omitempty"`
	ReviewerSessionExitStatus                string                            `json:"reviewerSessionExitStatus,omitempty"`
	ReviewerCompletionReceiptPath            string                            `json:"reviewerCompletionReceiptPath,omitempty"`
	ReviewerCompletionReceiptSHA256          string                            `json:"reviewerCompletionReceiptSha256,omitempty"`
	ReviewerSessionReceiptState              string                            `json:"reviewerSessionReceiptState,omitempty"`
	ReviewerSessionReceiptFailure            string                            `json:"reviewerSessionReceiptFailure,omitempty"`
	ReviewerDispatchRecordCommand            string                            `json:"reviewerDispatchRecordCommand,omitempty"`
	ReviewerCompletionRecordCommand          string                            `json:"reviewerCompletionRecordCommand,omitempty"`
	AgentToolRequest                         *ReviewerAgentToolRequest         `json:"agentToolRequest,omitempty"`
	ReviewerResultInputSaveCommand           string                            `json:"reviewerResultInputSaveCommand,omitempty"`
	ReviewerResultInputSaveApplyCommand      string                            `json:"reviewerResultInputSaveApplyCommand,omitempty"`
	ReviewerResultSourceCaptureCommand       string                            `json:"reviewerResultSourceCaptureCommand,omitempty"`
	ReviewerResultSourceCaptureApplyCommand  string                            `json:"reviewerResultSourceCaptureApplyCommand,omitempty"`
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
	ManagedDispatch                          *ReviewerManagedDispatchHandoff   `json:"managedDispatch,omitempty"`
	PreviewCommand                           string                            `json:"previewCommand,omitempty"`
	ApplyCommand                             string                            `json:"applyCommand,omitempty"`
	BatchPreviewCommand                      string                            `json:"batchPreviewCommand,omitempty"`
	BatchApplyCommand                        string                            `json:"batchApplyCommand,omitempty"`
	RefreshStatusCommand                     string                            `json:"refreshStatusCommand,omitempty"`
	OwnerExecutor                            string                            `json:"ownerExecutor,omitempty"`
	OwnerGeneration                          int                               `json:"ownerGeneration,omitempty"`
	OwnerBindingMode                         string                            `json:"ownerBindingMode,omitempty"`
	CurrentExecutor                          string                            `json:"currentExecutor,omitempty"`
	CurrentGeneration                        int                               `json:"currentGeneration,omitempty"`
	OwnerAdoptionRequired                    bool                              `json:"ownerAdoptionRequired"`
	OwnerAdoptionCurrent                     bool                              `json:"ownerAdoptionCurrent,omitempty"`
	OwnerAdoptionPath                        string                            `json:"ownerAdoptionPath,omitempty"`
	OwnerAdoptionActor                       string                            `json:"ownerAdoptionActor,omitempty"`
	OwnerAdoptionReason                      string                            `json:"ownerAdoptionReason,omitempty"`
	OwnerAdoptionCreatedAt                   string                            `json:"ownerAdoptionCreatedAt,omitempty"`
	OwnerAdoptionPreviewCommand              string                            `json:"ownerAdoptionPreviewCommand,omitempty"`
	PacketRetirementPreviewCommand           string                            `json:"packetRetirementPreviewCommand,omitempty"`
	RunbookSteps                             []string                          `json:"runbookSteps,omitempty"`
	Evidence                                 []string                          `json:"evidence,omitempty"`
	Boundary                                 []string                          `json:"boundary,omitempty"`
}

type ReviewerManagedDispatchHandoff struct {
	Mode                        string                    `json:"mode,omitempty"`
	Scope                       string                    `json:"scope,omitempty"`
	TargetLane                  string                    `json:"targetLane,omitempty"`
	PacketPath                  string                    `json:"packetPath,omitempty"`
	PromptRoot                  string                    `json:"promptRoot,omitempty"`
	ResultRoot                  string                    `json:"resultRoot,omitempty"`
	ReviewerCount               int                       `json:"reviewerCount,omitempty"`
	MaxParallel                 int                       `json:"maxParallel,omitempty"`
	Runbook                     []string                  `json:"runbook,omitempty"`
	CompletionCriteria          []string                  `json:"completionCriteria,omitempty"`
	ShardID                     string                    `json:"shardId"`
	ReviewerRole                string                    `json:"reviewerRole,omitempty"`
	Status                      string                    `json:"status,omitempty"`
	Items                       []string                  `json:"items,omitempty"`
	PromptPath                  string                    `json:"promptPath,omitempty"`
	PromptSHA256                string                    `json:"promptSha256,omitempty"`
	AgentToolRequest            *ReviewerAgentToolRequest `json:"agentToolRequest,omitempty"`
	ReviewerResultPath          string                    `json:"reviewerResultPath,omitempty"`
	ReviewerResultCandidatePath string                    `json:"reviewerResultCandidatePath,omitempty"`
	ReviewerResultInputPath     string                    `json:"reviewerResultInputPath,omitempty"`
	ReviewerResultSourcePath    string                    `json:"reviewerResultSourcePath,omitempty"`
	InputSavePreviewCommand     string                    `json:"inputSavePreviewCommand,omitempty"`
	InputSaveApplyCommand       string                    `json:"inputSaveApplyCommand,omitempty"`
	SourceCapturePreviewCommand string                    `json:"sourceCapturePreviewCommand,omitempty"`
	SourceCaptureApplyCommand   string                    `json:"sourceCaptureApplyCommand,omitempty"`
	StagingPreviewCommand       string                    `json:"stagingPreviewCommand,omitempty"`
	CollectionPreviewCommand    string                    `json:"collectionPreviewCommand,omitempty"`
	CollectionApplyCommand      string                    `json:"collectionApplyCommand,omitempty"`
	IntakePreviewCommand        string                    `json:"intakePreviewCommand,omitempty"`
	IntakeApplyCommand          string                    `json:"intakeApplyCommand,omitempty"`
	DispatchCommand             string                    `json:"dispatchCommand,omitempty"`
	ReviewerResultSkeleton      string                    `json:"reviewerResultSkeleton,omitempty"`
	ExpectedOutput              string                    `json:"expectedOutput,omitempty"`
	NextAction                  string                    `json:"nextAction,omitempty"`
	Boundary                    []string                  `json:"boundary,omitempty"`
}

type ReviewerDispatchOperatorPackage struct {
	Ready                bool                                   `json:"ready"`
	Summary              string                                 `json:"summary,omitempty"`
	PacketID             string                                 `json:"packetId,omitempty"`
	PacketPath           string                                 `json:"packetPath,omitempty"`
	TargetLane           string                                 `json:"targetLane,omitempty"`
	Current              *ReviewerDispatchOperatorPackageItem   `json:"current,omitempty"`
	CurrentRunLoopStepID string                                 `json:"currentRunLoopStepId,omitempty"`
	CurrentDriverRequest *mission.MissionCommanderDriverRequest `json:"currentDriverRequest,omitempty"`
	RefreshStatusCommand string                                 `json:"refreshStatusCommand,omitempty"`
	RunLoop              []ReviewerDispatchRunLoopStep          `json:"runLoop,omitempty"`
	RunbookSteps         []string                               `json:"runbookSteps,omitempty"`
	CompletionCriteria   []string                               `json:"completionCriteria,omitempty"`
	Boundary             []string                               `json:"boundary,omitempty"`
}

type ReviewerDispatchRunLoopStep struct {
	StepID           string                    `json:"stepId"`
	Order            int                       `json:"order"`
	Actor            string                    `json:"actor"`
	Description      string                    `json:"description"`
	Command          string                    `json:"command,omitempty"`
	PreviewCommand   string                    `json:"previewCommand,omitempty"`
	ApplyCommand     string                    `json:"applyCommand,omitempty"`
	Path             string                    `json:"path,omitempty"`
	AgentToolRequest *ReviewerAgentToolRequest `json:"agentToolRequest,omitempty"`
	Boundary         []string                  `json:"boundary,omitempty"`
}

type ReviewerDispatchOperatorPackageItem struct {
	ShardID                                   string                    `json:"shardId"`
	State                                     string                    `json:"state,omitempty"`
	ReviewerRole                              string                    `json:"reviewerRole,omitempty"`
	Status                                    string                    `json:"status,omitempty"`
	Items                                     []string                  `json:"items,omitempty"`
	DispatchPromptPath                        string                    `json:"dispatchPromptPath,omitempty"`
	DispatchPromptSHA256                      string                    `json:"dispatchPromptSha256,omitempty"`
	DispatchPromptState                       string                    `json:"dispatchPromptState,omitempty"`
	DispatchPromptCurrent                     bool                      `json:"dispatchPromptCurrent,omitempty"`
	DispatchPromptActualSHA256                string                    `json:"dispatchPromptActualSha256,omitempty"`
	DispatchPromptFailure                     string                    `json:"dispatchPromptFailure,omitempty"`
	DispatchPromptRepairCommand               string                    `json:"dispatchPromptRepairCommand,omitempty"`
	ReviewerDispatchID                        string                    `json:"reviewerDispatchId,omitempty"`
	ReviewerDispatchReceiptPath               string                    `json:"reviewerDispatchReceiptPath,omitempty"`
	ReviewerDispatchReceiptSHA256             string                    `json:"reviewerDispatchReceiptSha256,omitempty"`
	ReviewerHarness                           string                    `json:"reviewerHarness,omitempty"`
	ReviewerSession                           string                    `json:"reviewerSession,omitempty"`
	ReviewerSessionOutcome                    string                    `json:"reviewerSessionOutcome,omitempty"`
	ReviewerSessionExitStatus                 string                    `json:"reviewerSessionExitStatus,omitempty"`
	ReviewerCompletionReceiptPath             string                    `json:"reviewerCompletionReceiptPath,omitempty"`
	ReviewerCompletionReceiptSHA256           string                    `json:"reviewerCompletionReceiptSha256,omitempty"`
	ReviewerSessionReceiptState               string                    `json:"reviewerSessionReceiptState,omitempty"`
	ReviewerSessionReceiptFailure             string                    `json:"reviewerSessionReceiptFailure,omitempty"`
	ReviewerDispatchRecordCommand             string                    `json:"reviewerDispatchRecordCommand,omitempty"`
	ReviewerCompletionRecordCommand           string                    `json:"reviewerCompletionRecordCommand,omitempty"`
	AgentToolRequest                          *ReviewerAgentToolRequest `json:"agentToolRequest,omitempty"`
	ExpectedReviewerResultSkeleton            string                    `json:"expectedReviewerResultSkeleton,omitempty"`
	ExpectedOutput                            string                    `json:"expectedOutput,omitempty"`
	ReviewerResultDropPath                    string                    `json:"reviewerResultDropPath,omitempty"`
	ReviewerResultInputPath                   string                    `json:"reviewerResultInputPath,omitempty"`
	ReviewerResultInputState                  string                    `json:"reviewerResultInputState,omitempty"`
	ReviewerResultSourcePath                  string                    `json:"reviewerResultSourcePath,omitempty"`
	ReviewerResultSourceState                 string                    `json:"reviewerResultSourceState,omitempty"`
	ReviewerResultCandidatePath               string                    `json:"reviewerResultCandidatePath,omitempty"`
	ReviewerResultCandidateState              string                    `json:"reviewerResultCandidateState,omitempty"`
	ReviewerResultPath                        string                    `json:"reviewerResultPath,omitempty"`
	ReviewerResultState                       string                    `json:"reviewerResultState,omitempty"`
	ReviewerResultPresent                     bool                      `json:"reviewerResultPresent,omitempty"`
	ReviewerResultInputSavePreviewCommand     string                    `json:"reviewerResultInputSavePreviewCommand,omitempty"`
	ReviewerResultInputSaveApplyCommand       string                    `json:"reviewerResultInputSaveApplyCommand,omitempty"`
	ReviewerResultSourceCapturePreviewCommand string                    `json:"reviewerResultSourceCapturePreviewCommand,omitempty"`
	ReviewerResultSourceCaptureApplyCommand   string                    `json:"reviewerResultSourceCaptureApplyCommand,omitempty"`
	ReviewerResultStagingPreviewCommand       string                    `json:"reviewerResultStagingPreviewCommand,omitempty"`
	ReviewerResultCollectionPreviewCommand    string                    `json:"reviewerResultCollectionPreviewCommand,omitempty"`
	ReviewerResultCollectionApplyCommand      string                    `json:"reviewerResultCollectionApplyCommand,omitempty"`
	ReviewerResultIntakePreviewCommand        string                    `json:"reviewerResultIntakePreviewCommand,omitempty"`
	ReviewerResultIntakeApplyCommand          string                    `json:"reviewerResultIntakeApplyCommand,omitempty"`
	ReviewerResultBatchIntakePreviewCommand   string                    `json:"reviewerResultBatchIntakePreviewCommand,omitempty"`
	ReviewerResultBatchIntakeApplyCommand     string                    `json:"reviewerResultBatchIntakeApplyCommand,omitempty"`
	DispatchCommand                           string                    `json:"dispatchCommand,omitempty"`
	NextAction                                string                    `json:"nextAction,omitempty"`
}

type ReviewerPacketRetirementHandoff struct {
	PacketID        string   `json:"packetId,omitempty"`
	PacketPath      string   `json:"packetPath"`
	IntegrityPath   string   `json:"integrityPath"`
	RetirementPath  string   `json:"retirementPath"`
	TargetLane      string   `json:"targetLane,omitempty"`
	State           string   `json:"state"`
	PacketSHA256    string   `json:"packetSha256"`
	PacketBytes     int      `json:"packetBytes"`
	IntegritySHA256 string   `json:"integritySha256"`
	IntegrityBytes  int      `json:"integrityBytes"`
	Actor           string   `json:"actor,omitempty"`
	Reason          string   `json:"reason,omitempty"`
	CreatedAt       string   `json:"createdAt,omitempty"`
	NoDelete        bool     `json:"noDelete"`
	NoHeavyTool     bool     `json:"noHeavyTool"`
	NoAuthority     bool     `json:"noAuthorityOrConfirmed"`
	NextAction      string   `json:"nextAction,omitempty"`
	RunbookSteps    []string `json:"runbookSteps,omitempty"`
	Evidence        []string `json:"evidence,omitempty"`
	Boundary        []string `json:"boundary,omitempty"`
}

type ReviewerPacketRetirementSummary struct {
	Total           int      `json:"total"`
	LaneCount       int      `json:"laneCount"`
	Lanes           []string `json:"lanes,omitempty"`
	PacketCount     int      `json:"packetCount"`
	LatestPacketID  string   `json:"latestPacketId,omitempty"`
	LatestPacket    string   `json:"latestPacket,omitempty"`
	LatestState     string   `json:"latestState,omitempty"`
	LatestLane      string   `json:"latestLane,omitempty"`
	LatestReceipt   string   `json:"latestReceipt,omitempty"`
	LatestActor     string   `json:"latestActor,omitempty"`
	LatestReason    string   `json:"latestReason,omitempty"`
	LatestCreatedAt string   `json:"latestCreatedAt,omitempty"`
	NextAction      string   `json:"nextAction,omitempty"`
	RunbookSteps    []string `json:"runbookSteps,omitempty"`
	Boundary        []string `json:"boundary,omitempty"`
}

type ReviewerDispatchIntakeSummary struct {
	Total                                             int                              `json:"total"`
	WaitingForReviewerResult                          int                              `json:"waitingForReviewerResult"`
	ReadyForPreview                                   int                              `json:"readyForPreview"`
	AttachRequired                                    int                              `json:"attachRequired"`
	DispatchOnly                                      int                              `json:"dispatchOnly"`
	PromptArtifactBlocked                             int                              `json:"promptArtifactBlocked"`
	LaneCount                                         int                              `json:"laneCount"`
	Lanes                                             []string                         `json:"lanes,omitempty"`
	PacketCount                                       int                              `json:"packetCount"`
	LatestPacketDispatchTotal                         int                              `json:"latestPacketDispatchTotal,omitempty"`
	LatestPacketDispatchCompleted                     int                              `json:"latestPacketDispatchCompleted"`
	LatestPacketDispatchOpen                          int                              `json:"latestPacketDispatchOpen"`
	LatestPacketNextOpenShardID                       string                           `json:"latestPacketNextOpenShardId,omitempty"`
	LatestCompletedShardID                            string                           `json:"latestCompletedShardId,omitempty"`
	RemainingShardIDs                                 []string                         `json:"remainingShardIds,omitempty"`
	LatestPacketPath                                  string                           `json:"latestPacketPath,omitempty"`
	LatestShardID                                     string                           `json:"latestShardId,omitempty"`
	LatestState                                       string                           `json:"latestState,omitempty"`
	LatestReviewerResultPath                          string                           `json:"latestReviewerResultPath,omitempty"`
	LatestReviewerResultInputPath                     string                           `json:"latestReviewerResultInputPath,omitempty"`
	LatestReviewerResultInputState                    string                           `json:"latestReviewerResultInputState,omitempty"`
	LatestDispatchPromptPath                          string                           `json:"latestDispatchPromptPath,omitempty"`
	LatestDispatchPromptSHA256                        string                           `json:"latestDispatchPromptSha256,omitempty"`
	LatestDispatchPromptState                         string                           `json:"latestDispatchPromptState,omitempty"`
	LatestDispatchPromptCurrent                       bool                             `json:"latestDispatchPromptCurrent,omitempty"`
	LatestDispatchPromptActualSHA256                  string                           `json:"latestDispatchPromptActualSha256,omitempty"`
	LatestDispatchPromptFailure                       string                           `json:"latestDispatchPromptFailure,omitempty"`
	LatestReviewerResultSourcePath                    string                           `json:"latestReviewerResultSourcePath,omitempty"`
	LatestReviewerResultSourceState                   string                           `json:"latestReviewerResultSourceState,omitempty"`
	LatestReviewerResultCandidatePath                 string                           `json:"latestReviewerResultCandidatePath,omitempty"`
	LatestReviewerResultCandidateState                string                           `json:"latestReviewerResultCandidateState,omitempty"`
	LatestReviewerResultSourceCaptureCommand          string                           `json:"latestReviewerResultSourceCaptureCommand,omitempty"`
	LatestReviewerResultSourceCaptureApplyCommand     string                           `json:"latestReviewerResultSourceCaptureApplyCommand,omitempty"`
	LatestReviewerResultStagingCommand                string                           `json:"latestReviewerResultStagingCommand,omitempty"`
	LatestCollectionPreviewCommand                    string                           `json:"latestCollectionPreviewCommand,omitempty"`
	LatestCollectionApplyCommand                      string                           `json:"latestCollectionApplyCommand,omitempty"`
	LatestPreviewCommand                              string                           `json:"latestPreviewCommand,omitempty"`
	LatestApplyCommand                                string                           `json:"latestApplyCommand,omitempty"`
	LatestBatchPreviewCommand                         string                           `json:"latestBatchPreviewCommand,omitempty"`
	LatestBatchApplyCommand                           string                           `json:"latestBatchApplyCommand,omitempty"`
	NextActionShardID                                 string                           `json:"nextActionShardId,omitempty"`
	NextActionState                                   string                           `json:"nextActionState,omitempty"`
	NextActionDispatchPromptPath                      string                           `json:"nextActionDispatchPromptPath,omitempty"`
	NextActionDispatchPromptSHA256                    string                           `json:"nextActionDispatchPromptSha256,omitempty"`
	NextActionDispatchPromptState                     string                           `json:"nextActionDispatchPromptState,omitempty"`
	NextActionDispatchPromptCurrent                   bool                             `json:"nextActionDispatchPromptCurrent,omitempty"`
	NextActionDispatchPromptActualSHA256              string                           `json:"nextActionDispatchPromptActualSha256,omitempty"`
	NextActionDispatchPromptFailure                   string                           `json:"nextActionDispatchPromptFailure,omitempty"`
	NextActionDispatchPromptRepairCommand             string                           `json:"nextActionDispatchPromptRepairCommand,omitempty"`
	NextActionReviewerResultInputPath                 string                           `json:"nextActionReviewerResultInputPath,omitempty"`
	NextActionReviewerResultInputState                string                           `json:"nextActionReviewerResultInputState,omitempty"`
	NextActionReviewerResultSourcePath                string                           `json:"nextActionReviewerResultSourcePath,omitempty"`
	NextActionReviewerResultSourceState               string                           `json:"nextActionReviewerResultSourceState,omitempty"`
	NextActionReviewerResultCandidatePath             string                           `json:"nextActionReviewerResultCandidatePath,omitempty"`
	NextActionReviewerResultCandidateState            string                           `json:"nextActionReviewerResultCandidateState,omitempty"`
	NextActionReviewerResultSourceCaptureCommand      string                           `json:"nextActionReviewerResultSourceCaptureCommand,omitempty"`
	NextActionReviewerResultSourceCaptureApplyCommand string                           `json:"nextActionReviewerResultSourceCaptureApplyCommand,omitempty"`
	NextActionReviewerResultStagingCommand            string                           `json:"nextActionReviewerResultStagingCommand,omitempty"`
	NextActionCollectionPreviewCommand                string                           `json:"nextActionCollectionPreviewCommand,omitempty"`
	NextActionCollectionApplyCommand                  string                           `json:"nextActionCollectionApplyCommand,omitempty"`
	NextActionPreviewCommand                          string                           `json:"nextActionPreviewCommand,omitempty"`
	NextActionApplyCommand                            string                           `json:"nextActionApplyCommand,omitempty"`
	NextActionBatchPreviewCommand                     string                           `json:"nextActionBatchPreviewCommand,omitempty"`
	NextActionBatchApplyCommand                       string                           `json:"nextActionBatchApplyCommand,omitempty"`
	NextActionPacketRetirementPreviewCommand          string                           `json:"nextActionPacketRetirementPreviewCommand,omitempty"`
	NextAction                                        string                           `json:"nextAction,omitempty"`
	NextActionRunbookSteps                            []string                         `json:"nextActionRunbookSteps,omitempty"`
	OperatorPackage                                   *ReviewerDispatchOperatorPackage `json:"operatorPackage,omitempty"`
	Boundary                                          []string                         `json:"boundary,omitempty"`
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
	OwnerBinding          reviewerDispatchPacketOwner         `json:"ownerBinding"`
	Route                 reviewerDispatchPacketRoute         `json:"route"`
	Observability         reviewerDispatchPacketObservability `json:"observability"`
	ReviewerOrchestration reviewerDispatchPacketOrchestration `json:"reviewerOrchestration"`
}

type reviewerDispatchPacketRoute struct {
	ID string `json:"id"`
}

type reviewerDispatchPacketObservability struct {
	SummaryPath string `json:"summaryPath"`
}

type reviewerDispatchPacketOrchestration struct {
	Mode                  string                           `json:"mode"`
	TargetLane            string                           `json:"targetLane"`
	PacketPath            string                           `json:"packetPath"`
	ResultRoot            string                           `json:"resultRoot"`
	OwnerBinding          reviewerDispatchPacketOwner      `json:"ownerBinding"`
	ManagedDispatchPacket *reviewerManagedDispatchPacket   `json:"managedDispatchPacket"`
	Dispatches            []reviewerDispatchPacketDispatch `json:"dispatches"`
	BatchPreviewCommand   string                           `json:"batchPreviewCommand"`
	BatchApplyCommand     string                           `json:"batchApplyCommand"`
}

type reviewerManagedDispatchPacket struct {
	Mode                string                      `json:"mode"`
	Scope               string                      `json:"scope"`
	TargetLane          string                      `json:"targetLane"`
	OwnerBinding        reviewerDispatchPacketOwner `json:"ownerBinding"`
	PacketPath          string                      `json:"packetPath"`
	PromptRoot          string                      `json:"promptRoot"`
	ResultRoot          string                      `json:"resultRoot"`
	ReviewerCount       int                         `json:"reviewerCount"`
	MaxParallel         int                         `json:"maxParallel"`
	Dispatches          []reviewerManagedDispatch   `json:"dispatches"`
	BatchPreviewCommand string                      `json:"batchPreviewCommand"`
	BatchApplyCommand   string                      `json:"batchApplyCommand"`
	Runbook             []string                    `json:"runbook"`
	Boundary            []string                    `json:"boundary"`
	CompletionCriteria  []string                    `json:"completionCriteria"`
}

type reviewerManagedDispatch struct {
	ShardID                     string                    `json:"shardId"`
	ReviewerRole                string                    `json:"reviewerRole"`
	Status                      string                    `json:"status"`
	Items                       []string                  `json:"items"`
	PromptPath                  string                    `json:"promptPath"`
	PromptSHA256                string                    `json:"promptSha256"`
	AgentToolRequest            *ReviewerAgentToolRequest `json:"agentToolRequest"`
	ReviewerResultPath          string                    `json:"reviewerResultPath"`
	ReviewerResultCandidatePath string                    `json:"reviewerResultCandidatePath"`
	ReviewerResultInputPath     string                    `json:"reviewerResultInputPath"`
	ReviewerResultSourcePath    string                    `json:"reviewerResultSourcePath"`
	InputSavePreviewCommand     string                    `json:"inputSavePreviewCommand,omitempty"`
	InputSaveApplyCommand       string                    `json:"inputSaveApplyCommand,omitempty"`
	SourceCapturePreviewCommand string                    `json:"sourceCapturePreviewCommand"`
	SourceCaptureApplyCommand   string                    `json:"sourceCaptureApplyCommand"`
	StagingPreviewCommand       string                    `json:"stagingPreviewCommand"`
	CollectionPreviewCommand    string                    `json:"collectionPreviewCommand"`
	CollectionApplyCommand      string                    `json:"collectionApplyCommand"`
	IntakePreviewCommand        string                    `json:"intakePreviewCommand"`
	IntakeApplyCommand          string                    `json:"intakeApplyCommand"`
	DispatchCommand             string                    `json:"dispatchCommand"`
	ReviewerResultSkeleton      string                    `json:"reviewerResultSkeleton"`
	ExpectedOutput              string                    `json:"expectedOutput"`
	NextAction                  string                    `json:"nextAction"`
	Boundary                    []string                  `json:"boundary"`
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

type reviewerPacketOwnerAdoption struct {
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

type reviewerDispatchPacketDispatch struct {
	ShardID                     string                            `json:"shardId"`
	Status                      string                            `json:"status"`
	Items                       []string                          `json:"items"`
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
		packet, packetErr := readReviewerDispatchPacket(caseRoot, packetPath)
		if packetErr != nil {
			if !integrityPresent {
				continue
			}
			if strings.TrimSpace(laneID) != "" && integrity.TargetLane != laneID {
				continue
			}
			packet.PacketID = integrity.PacketID
			packet.TargetLane = integrity.TargetLane
			items = append(items, reviewerPacketIntegrityInvalidHandoff(caseRoot, packet, packetPath, integrity.TargetLane, fmt.Errorf("decode reviewer packet failed while integrity metadata remains: %w", packetErr)))
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

func ReviewerPacketRetirementHandoffs(caseRoot, laneID string) ([]ReviewerPacketRetirementHandoff, error) {
	packetPaths, err := reviewerDispatchPacketPaths(caseRoot)
	if err != nil {
		return nil, err
	}
	items := []ReviewerPacketRetirementHandoff{}
	for _, packetPath := range packetPaths {
		integrity, err := readReviewerPacketIntegrity(caseRoot, packetPath)
		if err != nil {
			continue
		}
		if strings.TrimSpace(laneID) != "" && integrity.TargetLane != laneID {
			continue
		}
		retirement, ok := currentReviewerPacketRetirement(caseRoot, packetPath, integrity)
		if !ok {
			continue
		}
		items = append(items, reviewerPacketRetirementHandoff(caseRoot, packetPath, integrity, retirement))
	}
	return limitReviewerPacketRetirementHandoffs(items, maxHandoffRows), nil
}

func limitReviewerPacketRetirementHandoffs(items []ReviewerPacketRetirementHandoff, limit int) []ReviewerPacketRetirementHandoff {
	if limit <= 0 || len(items) <= limit {
		return items
	}
	return append([]ReviewerPacketRetirementHandoff{}, items[len(items)-limit:]...)
}

func reviewerPacketRetirementHandoff(caseRoot, packetPath string, integrity reviewerPacketIntegrity, retirement reviewerPacketRetirement) ReviewerPacketRetirementHandoff {
	integrityPath := filepath.Join(filepath.Dir(packetPath), "packet.integrity.json")
	retirementPath := filepath.Join(filepath.Dir(packetPath), "packet.retirement.json")
	item := ReviewerPacketRetirementHandoff{
		PacketID:        integrity.PacketID,
		PacketPath:      packetPath,
		IntegrityPath:   integrityPath,
		RetirementPath:  retirementPath,
		TargetLane:      integrity.TargetLane,
		State:           "reviewer-packet-retired",
		PacketSHA256:    retirement.PacketSHA256,
		PacketBytes:     retirement.PacketBytes,
		IntegritySHA256: retirement.IntegritySHA256,
		IntegrityBytes:  retirement.IntegrityBytes,
		Actor:           retirement.Actor,
		Reason:          retirement.Reason,
		CreatedAt:       retirement.CreatedAt,
		NoDelete:        retirement.NoDelete,
		NoHeavyTool:     retirement.NoHeavyTool,
		NoAuthority:     retirement.NoAuthority,
		NextAction:      reviewerPacketRetirementNextAction(integrity.TargetLane),
		Evidence: []string{
			"exact retirement receipt " + reviewerDispatchDisplayPath(caseRoot, retirementPath),
			"packet " + reviewerDispatchDisplayPath(caseRoot, packetPath) + " sha256=" + retirement.PacketSHA256,
			"integrity " + reviewerDispatchDisplayPath(caseRoot, integrityPath) + " sha256=" + retirement.IntegritySHA256,
		},
		Boundary: reviewerPacketRetirementBoundary(),
	}
	item.RunbookSteps = reviewerPacketRetirementRunbookSteps(item)
	return item
}

func reviewerPacketRetirementNextAction(lane string) string {
	lane = firstText(lane, "<lane>")
	return "regenerate a new canonical reviewer packet for " + lane + " if reviewer work remains; otherwise rerun /rekit continue " + lane + " -WhatIf to resume lane continuation"
}

func reviewerPacketRetirementRunbookSteps(item ReviewerPacketRetirementHandoff) []string {
	lane := firstText(item.TargetLane, "<lane>")
	return mission.UniqueStrings([]string{
		"exact invalid reviewer packet retirement is current; do not dispatch, collect, intake, adopt, or continue from the retired packet",
		"if reviewer work remains, regenerate the canonical reviewer packet and packet.integrity.json together; do not repair packet bytes or integrity metadata independently",
		"if no reviewer work remains, rerun /rekit continue " + lane + " -WhatIf and then the bounded Apply only after reviewing the refreshed preview",
	})
}

func reviewerPacketRetirementBoundary() []string {
	return []string{
		"retired reviewer packet is closed provenance only and must not be dispatched, collected, intaken, adopted, or used for lane continuation",
		"retirement does not delete, repair, or rewrite packet bytes or packet.integrity.json",
		"runtime does not spawn, stop, monitor, or manage reviewer sessions and does not execute heavy tools or write authority/confirmed state",
	}
}

func limitReviewerDispatchIntakeHandoffs(items []ReviewerDispatchIntakeHandoff, limit int) []ReviewerDispatchIntakeHandoff {
	if limit <= 0 || len(items) <= limit {
		return items
	}
	limited := append([]ReviewerDispatchIntakeHandoff{}, items[len(items)-limit:]...)
	bestPriority := reviewerDispatchActionPriority(limited[0])
	for _, item := range limited[1:] {
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
		NextOpenShardID:                "packet-integrity",
		State:                          "reviewer-packet-integrity-invalid",
		PacketRetirementPreviewCommand: reviewerDispatchPacketRetirementPreviewCommand(packetPath, targetLane),
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
	item.RunbookSteps = reviewerDispatchIntakeRunbookSteps(item)
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
		case "ready-for-reviewer-result-source-capture-preview", "ready-for-reviewer-result-staging-preview", "ready-for-reviewer-result-collection-preview", "reviewer-result-recovery-disposed-ready-for-collection-preview", "ready-for-reviewer-intake-preview":
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
		blocked := state != "reviewer-session-running-unknown" && state != "ready-for-reviewer-result-source-capture-preview" && state != "ready-for-reviewer-result-staging-preview" && state != "ready-for-reviewer-result-collection-preview" && state != "reviewer-result-recovery-disposed-ready-for-collection-preview" && state != "ready-for-reviewer-intake-preview" && state != "reviewer-packet-owner-adoption-required" && state != "reviewer-result-recovery-required" && state != "reviewer-result-recovery-finalize-required" && !(state == "reviewer-result-recovery-ambiguous" && handoff.ReviewerResultRecoveryDispositionCommand != "") && !((state == "reviewer-dispatch-prompt-artifact-invalid" || state == "reviewer-dispatch-prompt-artifact-drift") && handoff.DispatchPromptRepairCommand != "")
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
	packetActions = orderReviewerDispatchMissionActions(packetActions)
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

func orderReviewerDispatchMissionActions(items []mission.MissionCommanderNextActionItem) []mission.MissionCommanderNextActionItem {
	out := append([]mission.MissionCommanderNextActionItem{}, items...)
	slices.SortStableFunc(out, func(a, b mission.MissionCommanderNextActionItem) int {
		return reviewerDispatchMissionActionPriority(a) - reviewerDispatchMissionActionPriority(b)
	})
	return out
}

func reviewerDispatchMissionActionPriority(item mission.MissionCommanderNextActionItem) int {
	return reviewerDispatchActionPriority(ReviewerDispatchIntakeHandoff{State: item.State})
}

func reviewerDispatchActionPriority(item ReviewerDispatchIntakeHandoff) int {
	switch item.State {
	case "reviewer-packet-owner-adoption-required":
		return 0
	case "reviewer-dispatch-prompt-artifact-invalid", "reviewer-dispatch-prompt-artifact-drift", "reviewer-result-recovery-finalize-required":
		return 1
	case "reviewer-result-recovery-required", "reviewer-result-recovery-ambiguous":
		return 2
	case "reviewer-session-receipt-invalid", "reviewer-session-receipt-owner-stale", "reviewer-session-failed":
		return 3
	case "ready-for-reviewer-intake-preview":
		return 3
	case "ready-for-reviewer-result-collection-preview", "reviewer-result-recovery-disposed-ready-for-collection-preview":
		return 4
	case "ready-for-reviewer-result-staging-preview":
		return 5
	case "ready-for-reviewer-result-source-capture-preview":
		return 6
	case "reviewer-packet-integrity-invalid", "reviewer-result-symlink-blocked", "reviewer-result-input-invalid", "reviewer-result-source-invalid", "reviewer-result-candidate-invalid", "reviewer-result-canonical-invalid", "reviewer-result-collection-required", "reviewer-result-recovery-invalid", "attach-required-before-reviewer-intake":
		return 7
	case "ready-for-reviewer-completion-receipt-preview":
		return 7
	case "reviewer-session-running-unknown":
		return 8
	case "ready-for-reviewer-dispatch", "waiting-for-reviewer-result", "dispatch-only-waiting-for-result":
		return 9
	default:
		return 9
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
		case "ready-for-reviewer-dispatch", "reviewer-session-running-unknown", "ready-for-reviewer-completion-receipt-preview", "waiting-for-reviewer-result", "dispatch-only-waiting-for-result":
			summary.WaitingForReviewerResult++
		case "ready-for-reviewer-result-source-capture-preview", "ready-for-reviewer-result-staging-preview", "ready-for-reviewer-result-collection-preview", "reviewer-result-recovery-disposed-ready-for-collection-preview", "ready-for-reviewer-intake-preview":
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
		summary.LatestReviewerResultInputPath = latest.ReviewerResultInputPath
		summary.LatestReviewerResultInputState = latest.ReviewerResultInputState
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
		summary.LatestReviewerResultSourceCaptureCommand = latest.ReviewerResultSourceCaptureCommand
		summary.LatestReviewerResultSourceCaptureApplyCommand = latest.ReviewerResultSourceCaptureApplyCommand
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
			summary.NextActionReviewerResultInputPath = nextAction.ReviewerResultInputPath
			summary.NextActionReviewerResultInputState = nextAction.ReviewerResultInputState
			summary.NextActionReviewerResultSourcePath = nextAction.ReviewerResultSourcePath
			summary.NextActionReviewerResultSourceState = nextAction.ReviewerResultSourceState
			summary.NextActionReviewerResultCandidatePath = nextAction.ReviewerResultCandidatePath
			summary.NextActionReviewerResultCandidateState = nextAction.ReviewerResultCandidateState
			summary.NextActionReviewerResultSourceCaptureCommand = nextAction.ReviewerResultSourceCaptureCommand
			summary.NextActionReviewerResultSourceCaptureApplyCommand = nextAction.ReviewerResultSourceCaptureApplyCommand
			summary.NextActionReviewerResultStagingCommand = nextAction.ReviewerResultStagingCommand
			if nextAction.ReviewerResultCollectionCommands != nil {
				summary.NextActionCollectionPreviewCommand = nextAction.ReviewerResultCollectionCommands.PreviewCommand
				summary.NextActionCollectionApplyCommand = nextAction.ReviewerResultCollectionCommands.ApplyCommand
			}
			summary.NextActionPreviewCommand = nextAction.PreviewCommand
			summary.NextActionApplyCommand = nextAction.ApplyCommand
			summary.NextActionBatchPreviewCommand = nextAction.BatchPreviewCommand
			summary.NextActionBatchApplyCommand = nextAction.BatchApplyCommand
			summary.NextActionPacketRetirementPreviewCommand = nextAction.PacketRetirementPreviewCommand
			if nextAction.State == "ready-for-reviewer-intake-preview" && strings.TrimSpace(nextAction.BatchPreviewCommand) != "" {
				summary.LatestBatchPreviewCommand = nextAction.BatchPreviewCommand
				summary.LatestBatchApplyCommand = nextAction.BatchApplyCommand
			}
			summary.NextAction = reviewerDispatchIntakeNextAction(*nextAction)
			summary.NextActionRunbookSteps = reviewerDispatchIntakeRunbookSteps(*nextAction)
			summary.OperatorPackage = reviewerDispatchOperatorPackageFor(*nextAction)
		}
		summary.Boundary = reviewerDispatchIntakeSummaryBoundary()
	}
	return summary
}

func ReviewerPacketRetirementSummaryFor(items []ReviewerPacketRetirementHandoff) ReviewerPacketRetirementSummary {
	summary := ReviewerPacketRetirementSummary{}
	lanes := map[string]bool{}
	packets := map[string]bool{}
	for _, item := range items {
		summary.Total++
		if lane := strings.TrimSpace(item.TargetLane); lane != "" {
			lanes[lane] = true
		}
		packetKey := firstText(item.PacketID, item.PacketPath)
		if packetKey != "" {
			packets[packetKey] = true
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
		summary.LatestPacketID = latest.PacketID
		summary.LatestPacket = latest.PacketPath
		summary.LatestState = latest.State
		summary.LatestLane = latest.TargetLane
		summary.LatestReceipt = latest.RetirementPath
		summary.LatestActor = latest.Actor
		summary.LatestReason = latest.Reason
		summary.LatestCreatedAt = latest.CreatedAt
		summary.NextAction = latest.NextAction
		summary.RunbookSteps = append([]string{}, latest.RunbookSteps...)
		summary.Boundary = reviewerPacketRetirementSummaryBoundary()
	}
	return summary
}

func reviewerPacketRetirementSummaryBoundary() []string {
	return mission.UniqueStrings(append([]string{"reviewer packet retirement summary is read-only and does not reopen retired packet work"}, reviewerPacketRetirementBoundary()...))
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

func readReviewerDispatchPacket(caseRoot, path string) (reviewerDispatchPacket, error) {
	data, err := readStableReviewerWorkstreamArtifact(caseRoot, path, "reviewer packet")
	if err != nil {
		return reviewerDispatchPacket{}, err
	}
	trimmed := bytes.TrimSpace(data)
	dec := json.NewDecoder(bytes.NewReader(trimmed))
	var fields map[string]json.RawMessage
	if err := dec.Decode(&fields); err != nil {
		return reviewerDispatchPacket{}, fmt.Errorf("decode reviewer packet JSON: %w", err)
	}
	var trailing any
	if err := dec.Decode(&trailing); err != io.EOF {
		return reviewerDispatchPacket{}, fmt.Errorf("reviewer packet must contain exactly one JSON object")
	}
	allowed := map[string]bool{
		"schemaVersion": true, "packetId": true, "packetIntegrity": true, "command": true,
		"isMutation": true, "writesReviewArtifacts": true, "planRoot": true, "repoRoot": true,
		"pack": true, "manifestPath": true, "targetLane": true, "ownerBinding": true,
		"route": true, "input": true, "shardPolicy": true, "shards": true,
		"shardHandoffs": true, "reviewerOrchestration": true, "mainAgentResponsibilities": true,
		"subagentPermissions": true, "outputContract": true, "reviewRequired": true,
		"observability": true, "reviewLoop": true,
	}
	for field := range fields {
		if !allowed[field] {
			return reviewerDispatchPacket{}, fmt.Errorf("decode reviewer packet JSON: json: unknown field %q", field)
		}
	}
	var packet reviewerDispatchPacket
	if err := json.Unmarshal(trimmed, &packet); err != nil {
		return reviewerDispatchPacket{}, fmt.Errorf("decode reviewer packet JSON: %w", err)
	}
	return packet, nil
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
	_, ok := currentReviewerPacketRetirement(caseRoot, packetPath, integrity)
	return ok
}

func currentReviewerPacketRetirement(caseRoot, packetPath string, integrity reviewerPacketIntegrity) (reviewerPacketRetirement, bool) {
	inst, err := instance.Read(caseRoot)
	if err != nil || inst.Source == "missing" || inst.Moved() || strings.TrimSpace(inst.TemplateRoot) == "" || strings.TrimSpace(inst.TemplatePack) == "" {
		return reviewerPacketRetirement{}, false
	}
	retirementPath := filepath.Join(filepath.Dir(packetPath), "packet.retirement.json")
	if !reviewpath.CollectionNamespacePathSafe(caseRoot, retirementPath, false) {
		return reviewerPacketRetirement{}, false
	}
	data, err := readStableReviewerWorkstreamArtifact(caseRoot, retirementPath, "reviewer packet retirement")
	if err != nil {
		return reviewerPacketRetirement{}, false
	}
	dec := json.NewDecoder(bytes.NewReader(bytes.TrimSpace(data)))
	dec.DisallowUnknownFields()
	var retirement reviewerPacketRetirement
	if err := dec.Decode(&retirement); err != nil {
		return reviewerPacketRetirement{}, false
	}
	var trailing any
	if err := dec.Decode(&trailing); err != io.EOF {
		return reviewerPacketRetirement{}, false
	}
	packetData, packetErr := readStableReviewerWorkstreamArtifact(caseRoot, packetPath, "reviewer packet")
	integrityPath := filepath.Join(filepath.Dir(packetPath), "packet.integrity.json")
	integrityData, integrityErr := readStableReviewerWorkstreamArtifact(caseRoot, integrityPath, "reviewer packet integrity")
	if packetErr != nil || integrityErr != nil {
		return reviewerPacketRetirement{}, false
	}
	packetSum := sha256.Sum256(packetData)
	integritySum := sha256.Sum256(integrityData)
	_, createdAtErr := time.Parse(time.RFC3339Nano, retirement.CreatedAt)
	current := retirement.SchemaVersion == 1 && retirement.Kind == "reviewer-packet-retirement" &&
		casebind.SamePath(retirement.RepoRoot, inst.TemplateRoot) && casebind.SamePath(retirement.CaseRoot, inst.CaseRoot) && retirement.Pack == inst.TemplatePack &&
		retirement.PacketID == integrity.PacketID && retirement.Lane == integrity.TargetLane &&
		casebind.SamePath(retirement.PacketPath, packetPath) && retirement.PacketSHA256 == hex.EncodeToString(packetSum[:]) && retirement.PacketBytes == len(packetData) &&
		casebind.SamePath(retirement.IntegrityPath, integrityPath) && retirement.IntegritySHA256 == hex.EncodeToString(integritySum[:]) && retirement.IntegrityBytes == len(integrityData) &&
		strings.TrimSpace(retirement.Actor) != "" && strings.TrimSpace(retirement.Reason) != "" && createdAtErr == nil && retirement.NoDelete && retirement.NoHeavyTool && retirement.NoAuthority
	return retirement, current
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
	return state == "ready-for-reviewer-dispatch" || state == "waiting-for-reviewer-result" || state == "dispatch-only-waiting-for-result" || state == "reviewer-session-failed" || state == "reviewer-session-receipt-owner-stale"
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

func reviewerDispatchStagingInputPath(dispatch reviewerDispatchPacketDispatch, expected string) string {
	if dispatch.StagingCommands != nil {
		input := strings.TrimSpace(dispatch.StagingCommands.SourceCaptureInput)
		if input != "" && input != "<case-local-reviewer-json-input>" {
			return input
		}
	}
	return expected
}

func reviewerDispatchInputSaveCommand(packetPath, shardID, targetLane string) string {
	return "/rekit plan-subagents -PacketPath " + reviewerDispatchQuoteCommandArg(packetPath) +
		" -SaveReviewerResultInput -ShardId " + reviewerDispatchQuoteCommandArg(shardID) +
		" -ReviewerResultInputSourcePath <reviewer-result-json-path> -Lane " + reviewerDispatchQuoteCommandArg(targetLane) +
		" -Actor <main-agent> -WhatIf -Format json"
}

func reviewerDispatchInputSaveApplyCommand(packetPath, shardID, targetLane string) string {
	return "/rekit plan-subagents -PacketPath " + reviewerDispatchQuoteCommandArg(packetPath) +
		" -SaveReviewerResultInput -ShardId " + reviewerDispatchQuoteCommandArg(shardID) +
		" -ReviewerResultInputSourcePath <reviewer-result-json-path> -Lane " + reviewerDispatchQuoteCommandArg(targetLane) +
		" -Actor <main-agent> -ExpectedReviewerResultInputSha256 <inputSha256-from-WhatIf> -Apply -Format json"
}

func reviewerDispatchSourceCaptureCommand(packetPath, shardID, targetLane, inputPath string) string {
	return "/rekit plan-subagents -PacketPath " + reviewerDispatchQuoteCommandArg(packetPath) +
		" -CaptureReviewerResultSource -ShardId " + reviewerDispatchQuoteCommandArg(shardID) +
		" -ReviewerResultInputPath " + reviewerDispatchQuoteCommandArg(inputPath) + " -Lane " + reviewerDispatchQuoteCommandArg(targetLane) +
		" -Actor <main-agent> -WhatIf -Format json"
}

func reviewerDispatchSourceCaptureApplyCommand(packetPath, shardID, targetLane, inputPath string) string {
	return "/rekit plan-subagents -PacketPath " + reviewerDispatchQuoteCommandArg(packetPath) +
		" -CaptureReviewerResultSource -ShardId " + reviewerDispatchQuoteCommandArg(shardID) +
		" -ReviewerResultInputPath " + reviewerDispatchQuoteCommandArg(inputPath) + " -Lane " + reviewerDispatchQuoteCommandArg(targetLane) +
		" -Actor <main-agent> -ExpectedReviewerResultInputSha256 <inputSha256-from-WhatIf> -Apply -Format json"
}

func reviewerResultInputSession(data []byte) (string, bool) {
	result, err := reviewerresult.Decode(data)
	if err != nil {
		return "", false
	}
	return result.ReviewerSession, true
}

func reviewerDispatchSessionOwner(owner reviewerDispatchPacketOwner) reviewersession.Owner {
	return reviewersession.Owner{CurrentExecutor: owner.CurrentExecutor, ExecutorGeneration: owner.ExecutorGeneration, BindingMode: owner.BindingMode}
}

func reviewerDispatchSessionID(packetID, routeID, shardID, promptSHA256, harness, session string) string {
	value := strings.Join([]string{packetID, routeID, shardID, strings.ToLower(promptSHA256), harness, session}, "\n")
	return reviewerDispatchBytesSHA256([]byte(value))
}

func reviewerDispatchSessionStaticBindingsCurrent(packet reviewerDispatchPacket, packetPath string, packetBytes []byte, targetLane string, dispatch reviewerDispatchPacketDispatch, receipt reviewersession.DispatchReceipt, receiptPath string) bool {
	if dispatch.AgentToolRequest == nil {
		return false
	}
	expectedID := reviewerDispatchSessionID(packet.PacketID, packet.Route.ID, dispatch.ShardID, dispatch.DispatchPromptSHA256, receipt.ReviewerHarness, receipt.ReviewerSession)
	return receipt.DispatchID == expectedID &&
		casebind.SamePath(receiptPath, reviewersession.DispatchPath(packetPath, dispatch.ShardID, expectedID)) &&
		receipt.PacketID == packet.PacketID &&
		casebind.SamePath(receipt.PacketPath, packetPath) &&
		receipt.PacketSHA256 == reviewerDispatchBytesSHA256(packetBytes) &&
		receipt.RouteID == packet.Route.ID &&
		receipt.ShardID == dispatch.ShardID &&
		slices.Equal(receipt.Items, dispatch.Items) &&
		casebind.SamePath(receipt.PromptPath, dispatch.DispatchPromptPath) &&
		receipt.PromptSHA256 == dispatch.DispatchPromptSHA256 &&
		receipt.AgentType == dispatch.AgentToolRequest.AgentType &&
		receipt.ReadOnly == dispatch.AgentToolRequest.ReadOnly &&
		receipt.TargetLane == targetLane &&
		receipt.PacketOwner == reviewerDispatchSessionOwner(packet.OwnerBinding)
}

func reviewerDispatchSessionCurrentOwnerBindings(caseRoot string, packet reviewerDispatchPacket, packetPath string, receipt reviewersession.DispatchReceipt, currentExecutor string, currentGeneration int) bool {
	if receipt.EffectiveOwner.CurrentExecutor != currentExecutor || receipt.EffectiveOwner.ExecutorGeneration != currentGeneration {
		return false
	}
	owner := packet.OwnerBinding
	if owner.CurrentExecutor == currentExecutor && owner.ExecutorGeneration == currentGeneration {
		return receipt.EffectiveOwner == reviewerDispatchSessionOwner(owner) && receipt.OwnerAdoptionPath == "" && receipt.OwnerAdoptionSHA256 == ""
	}
	adoptionPath := filepath.Join(caseRoot, ".rekit", "reviewer-adoptions", packet.PacketID+".json")
	adoption, current := reviewerDispatchCurrentAdoption(caseRoot, adoptionPath, packet, packetPath, currentExecutor, currentGeneration)
	if !current {
		return false
	}
	data, err := readStableReviewerWorkstreamArtifact(caseRoot, adoptionPath, "reviewer packet adoption")
	if err != nil {
		return false
	}
	return receipt.EffectiveOwner == reviewerDispatchSessionOwner(adoption.AdoptedOwner) && casebind.SamePath(receipt.OwnerAdoptionPath, adoptionPath) && receipt.OwnerAdoptionSHA256 == reviewerDispatchBytesSHA256(data)
}

type reviewerSessionLifecycle struct {
	dispatchID        string
	dispatchPath      string
	dispatchSHA256    string
	harness           string
	session           string
	outcome           string
	exitStatus        string
	completionPath    string
	completionSHA256  string
	state             string
	failure           string
	dispatchCommand   string
	completionCommand string
}

func reviewerDispatchSessionLifecycle(caseRoot string, packet reviewerDispatchPacket, packetPath, targetLane string, dispatch reviewerDispatchPacketDispatch, inputPath, inputState, inputSession, inputSHA256 string, inputBytes int, currentExecutor string, currentGeneration int) reviewerSessionLifecycle {
	if dispatch.AgentToolRequest == nil || strings.TrimSpace(dispatch.DispatchPromptPath) == "" || strings.TrimSpace(dispatch.DispatchPromptSHA256) == "" || dispatch.StagingCommands == nil || strings.TrimSpace(dispatch.ReviewerResultCandidatePath) == "" {
		return reviewerSessionLifecycle{}
	}
	packetBytes, err := readStableReviewerWorkstreamArtifact(caseRoot, packetPath, "reviewer packet")
	if err != nil {
		return reviewerSessionLifecycle{state: "reviewer-session-receipt-invalid", failure: err.Error()}
	}
	if strings.TrimSpace(packet.Route.ID) == "" {
		return reviewerSessionLifecycle{state: "reviewer-session-receipt-invalid", failure: "reviewer packet route binding is missing"}
	}
	if len(dispatch.Items) == 0 {
		return reviewerSessionLifecycle{state: "reviewer-session-receipt-invalid", failure: "reviewer packet shard items binding is missing"}
	}
	if packet.OwnerBinding.TargetLane != targetLane {
		return reviewerSessionLifecycle{state: "reviewer-session-receipt-invalid", failure: "reviewer packet owner target lane does not match current lane"}
	}
	if !casebind.SamePath(packet.ReviewerOrchestration.PacketPath, packetPath) {
		return reviewerSessionLifecycle{state: "reviewer-session-receipt-invalid", failure: "reviewer packet orchestration path does not match current packet"}
	}
	if packet.ReviewerOrchestration.OwnerBinding != packet.OwnerBinding {
		return reviewerSessionLifecycle{state: "reviewer-session-receipt-invalid", failure: "reviewer packet orchestration owner binding does not match packet owner binding"}
	}
	if dispatch.AgentToolRequest.PromptPath != "" && !casebind.SamePath(dispatch.AgentToolRequest.PromptPath, dispatch.DispatchPromptPath) {
		return reviewerSessionLifecycle{state: "reviewer-session-receipt-invalid", failure: "reviewer agent prompt path does not match dispatch prompt path"}
	}
	if dispatch.AgentToolRequest.PromptSHA256 != "" && dispatch.AgentToolRequest.PromptSHA256 != dispatch.DispatchPromptSHA256 {
		return reviewerSessionLifecycle{state: "reviewer-session-receipt-invalid", failure: "reviewer agent prompt hash does not match dispatch prompt hash"}
	}
	if !dispatch.AgentToolRequest.ReadOnly {
		return reviewerSessionLifecycle{state: "reviewer-session-receipt-invalid", failure: "reviewer agent request is not read-only"}
	}
	if strings.TrimSpace(dispatch.AgentToolRequest.AgentType) == "" {
		return reviewerSessionLifecycle{state: "reviewer-session-receipt-invalid", failure: "reviewer agent type binding is missing"}
	}
	root := filepath.Join(filepath.Dir(packetPath), "sessions", dispatch.ShardID, "dispatches")
	entries, err := os.ReadDir(root)
	if os.IsNotExist(err) {
		return reviewerSessionLifecycle{state: "ready-for-reviewer-dispatch", dispatchCommand: reviewerDispatchSessionRecordCommand(packetPath, dispatch.ShardID, targetLane)}
	}
	if err != nil {
		return reviewerSessionLifecycle{state: "reviewer-session-receipt-invalid", failure: err.Error()}
	}
	names := []string{}
	for _, entry := range entries {
		if !entry.IsDir() && strings.HasSuffix(strings.ToLower(entry.Name()), ".json") {
			names = append(names, entry.Name())
		}
	}
	sort.Strings(names)
	if len(names) == 0 {
		return reviewerSessionLifecycle{state: "reviewer-session-receipt-invalid", failure: "dispatch receipt namespace contains no receipt"}
	}
	var selected reviewerSessionLifecycle
	var selectedAt time.Time
	var selectedStale reviewerSessionLifecycle
	var selectedStaleAt time.Time
	var exactInputMatch reviewerSessionLifecycle
	exactInputMatches := 0
	matchingInputDispatches := 0
	for _, name := range names {
		path := filepath.Join(root, name)
		data, readErr := readStableReviewerWorkstreamArtifact(caseRoot, path, "reviewer session dispatch receipt")
		if readErr != nil {
			return reviewerSessionLifecycle{state: "reviewer-session-receipt-invalid", failure: readErr.Error()}
		}
		receipt, decodeErr := reviewersession.DecodeDispatch(data)
		if decodeErr != nil || !reviewerDispatchSessionStaticBindingsCurrent(packet, packetPath, packetBytes, targetLane, dispatch, receipt, path) {
			if decodeErr != nil {
				return reviewerSessionLifecycle{state: "reviewer-session-receipt-invalid", failure: decodeErr.Error()}
			}
			return reviewerSessionLifecycle{state: "reviewer-session-receipt-invalid", failure: "dispatch receipt does not match current packet shard bindings"}
		}
		candidate := reviewerSessionLifecycle{dispatchID: receipt.DispatchID, dispatchPath: path, dispatchSHA256: reviewerDispatchBytesSHA256(data), harness: receipt.ReviewerHarness, session: receipt.ReviewerSession, state: "reviewer-session-running-unknown"}
		if !reviewerDispatchSessionCurrentOwnerBindings(caseRoot, packet, packetPath, receipt, currentExecutor, currentGeneration) {
			candidate.state = "reviewer-session-receipt-owner-stale"
			candidate.failure = "dispatch receipt owner generation or adoption provenance is stale"
		}
		completionPath := reviewersession.CompletionPath(packetPath, dispatch.ShardID, receipt.DispatchID)
		completionState, stateErr := refsf.ClassifyNonEmptyRegularFile(completionPath)
		if stateErr != nil || (completionState != refsf.RegularFileMissing && completionState != refsf.RegularFileReady) {
			return reviewerSessionLifecycle{state: "reviewer-session-receipt-invalid", failure: "completion receipt must be a non-empty regular file"}
		}
		if completionState == refsf.RegularFileReady {
			completionData, readErr := readStableReviewerWorkstreamArtifact(caseRoot, completionPath, "reviewer session completion receipt")
			if readErr != nil {
				return reviewerSessionLifecycle{state: "reviewer-session-receipt-invalid", failure: readErr.Error()}
			}
			completion, decodeErr := reviewersession.DecodeCompletion(completionData)
			if decodeErr != nil || reviewersession.ValidateCompletionDispatchLineage(completion, receipt, path, candidate.dispatchSHA256) != nil || completion.PacketID != packet.PacketID || completion.ShardID != dispatch.ShardID {
				if decodeErr != nil {
					return reviewerSessionLifecycle{state: "reviewer-session-receipt-invalid", failure: decodeErr.Error()}
				}
				return reviewerSessionLifecycle{state: "reviewer-session-receipt-invalid", failure: "completion receipt does not match dispatch receipt bindings"}
			}
			candidate.outcome = completion.Outcome
			candidate.exitStatus = completion.ExitStatus
			candidate.completionPath = completionPath
			candidate.completionSHA256 = reviewerDispatchBytesSHA256(completionData)
			if completion.CompletionOwner.CurrentExecutor != currentExecutor || completion.CompletionOwner.ExecutorGeneration != currentGeneration {
				candidate.state = "reviewer-session-receipt-owner-stale"
				candidate.failure = "completion receipt owner generation is stale"
			} else if completion.Outcome == "failed" {
				candidate.state = "reviewer-session-failed"
			} else if inputState != "ready" || !casebind.SamePath(completion.ReviewerResultInputPath, inputPath) || !strings.EqualFold(completion.ReviewerResultInputSHA256, inputSHA256) || completion.ReviewerResultInputBytes != inputBytes {
				candidate.state = "reviewer-session-receipt-invalid"
				candidate.failure = "successful completion receipt does not match an available exact current reviewer result input"
			} else {
				candidate.state = "reviewer-session-completed"
			}
		} else if candidate.state != "reviewer-session-receipt-owner-stale" && inputState == "ready" && receipt.ReviewerSession == inputSession {
			candidate.state = "ready-for-reviewer-completion-receipt-preview"
			candidate.completionCommand = reviewerDispatchSessionCompletionCommand(packetPath, receipt.DispatchID, targetLane, inputPath)
		}
		recordedAt, _ := time.Parse(time.RFC3339Nano, receipt.RecordedAt)
		if candidate.state == "reviewer-session-receipt-owner-stale" && (selectedStale.dispatchID == "" || recordedAt.After(selectedStaleAt)) {
			selectedStale = candidate
			selectedStaleAt = recordedAt
		}
		if inputState == "ready" {
			if receipt.ReviewerSession != inputSession {
				continue
			}
			matchingInputDispatches++
			if candidate.state == "reviewer-session-completed" {
				exactInputMatches++
				exactInputMatch = candidate
			}
		}
		if selected.dispatchID == "" || recordedAt.After(selectedAt) {
			selected = candidate
			selectedAt = recordedAt
		}
	}
	if inputState == "ready" {
		if exactInputMatches == 1 {
			return exactInputMatch
		}
		if exactInputMatches > 1 {
			return reviewerSessionLifecycle{state: "reviewer-session-receipt-invalid", failure: "reviewer result input matches multiple successful completion receipt lineages"}
		}
		if matchingInputDispatches == 0 && selectedStale.dispatchID != "" {
			return selectedStale
		}
		if matchingInputDispatches == 0 {
			return reviewerSessionLifecycle{state: "reviewer-session-receipt-invalid", failure: "reviewer result input session is not bound to a dispatch receipt"}
		}
		if matchingInputDispatches > 1 {
			return reviewerSessionLifecycle{state: "reviewer-session-receipt-invalid", failure: "reviewer result input session has multiple dispatches without one exact successful completion lineage"}
		}
	}
	return selected
}

func reviewerDispatchSessionRecordCommand(packetPath, shardID, lane string) string {
	return "/rekit plan-subagents -PacketPath " + quoteCommandArg(packetPath) + " -RecordReviewerDispatch -ShardId " + quoteCommandArg(shardID) + " -ReviewerHarness <harness> -ReviewerSession <session-id> -Lane " + quoteCommandArg(lane) + " -Actor <main-agent> -WhatIf -Format json"
}

func reviewerDispatchSessionCompletionCommand(packetPath, dispatchID, lane, inputPath string) string {
	return "/rekit plan-subagents -PacketPath " + quoteCommandArg(packetPath) + " -RecordReviewerCompletion -ReviewerDispatchId " + quoteCommandArg(dispatchID) + " -ReviewerOutcome succeeded -ReviewerExitStatus completed -ReviewerResultInputPath " + quoteCommandArg(inputPath) + " -Lane " + quoteCommandArg(lane) + " -Actor <main-agent> -WhatIf -Format json"
}

func reviewerDispatchIntakeHandoffFor(caseRoot string, facts mission.LedgerFacts, packet reviewerDispatchPacket, packetPath, targetLane string, dispatch reviewerDispatchPacketDispatch, idx int) ReviewerDispatchIntakeHandoff {
	resultPath := strings.TrimSpace(dispatch.ReviewerResultPath)
	candidatePath := strings.TrimSpace(dispatch.ReviewerResultCandidatePath)
	expectedInputPath := filepath.Join(packet.ReviewerOrchestration.ResultRoot, "inputs", dispatch.ShardID+".reviewer-input.json")
	inputPath := reviewerDispatchStagingInputPath(dispatch, expectedInputPath)
	expectedSourcePath := filepath.Join(packet.ReviewerOrchestration.ResultRoot, "sources", dispatch.ShardID+".json")
	sourcePath := reviewerDispatchStagingSourcePath(dispatch, expectedSourcePath)
	collectionAvailable := packet.ReviewerOrchestration.PacketPath != "" &&
		reviewpath.CanonicalCollectionShard(caseRoot, packetPath, packet.ReviewerOrchestration.ResultRoot, dispatch.ShardID, candidatePath, resultPath) &&
		reviewpath.CollectionNamespacePathSafe(caseRoot, packetPath, false) &&
		reviewpath.CollectionNamespacePathSafe(caseRoot, packet.ReviewerOrchestration.ResultRoot, false) &&
		reviewpath.CollectionNamespacePathSafe(caseRoot, filepath.Dir(inputPath), true) &&
		reviewpath.CollectionNamespacePathSafe(caseRoot, inputPath, true) &&
		reviewpath.CollectionNamespacePathSafe(caseRoot, filepath.Dir(sourcePath), true) &&
		reviewpath.CollectionNamespacePathSafe(caseRoot, filepath.Dir(candidatePath), true) &&
		reviewpath.CollectionNamespacePathSafe(caseRoot, filepath.Dir(resultPath), false) &&
		casebind.SamePath(packet.ReviewerOrchestration.PacketPath, packetPath) &&
		casebind.SamePath(inputPath, expectedInputPath) &&
		casebind.SamePath(sourcePath, expectedSourcePath) &&
		dispatch.CollectionCommands != nil &&
		casebind.SamePath(dispatch.CollectionCommands.CandidatePath, candidatePath) &&
		reviewerDispatchIntakeCommandAvailable(dispatch.CollectionCommands.PreviewCommand) &&
		reviewerDispatchIntakeCommandAvailable(dispatch.CollectionCommands.ApplyCommand)
	inputSaveCommand := ""
	inputSaveApplyCommand := ""
	sourceCaptureCommand := ""
	sourceCaptureApplyCommand := ""
	stagingCommand := ""
	var collectionCommands *ReviewerResultCollectionCommands
	if collectionAvailable {
		inputSaveCommand = reviewerDispatchInputSaveCommand(packetPath, dispatch.ShardID, targetLane)
		inputSaveApplyCommand = reviewerDispatchInputSaveApplyCommand(packetPath, dispatch.ShardID, targetLane)
		sourceCaptureCommand = reviewerDispatchSourceCaptureCommand(packetPath, dispatch.ShardID, targetLane, inputPath)
		sourceCaptureApplyCommand = reviewerDispatchSourceCaptureApplyCommand(packetPath, dispatch.ShardID, targetLane, inputPath)
		stagingCommand = reviewerDispatchResultStagingCommand(packetPath, dispatch.ShardID, targetLane, sourcePath)
		commands := reviewerDispatchCollectionCommands(packetPath, dispatch.ShardID, targetLane, candidatePath)
		collectionCommands = &commands
	} else {
		inputPath = ""
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
	inputState := ""
	inputSession := ""
	inputSHA256 := ""
	inputBytes := 0
	if inputPath != "" {
		inputState = "missing"
		if classified, err := refsf.ClassifyNonEmptyRegularFile(inputPath); err != nil || classified == refsf.RegularFileSymlink || classified == refsf.RegularFileWaiting {
			if classified != refsf.RegularFileMissing {
				inputState = "invalid"
			}
		} else if classified == refsf.RegularFileReady {
			inputState = "ready"
			data, readErr := readStableReviewerWorkstreamArtifact(caseRoot, inputPath, "reviewer result input")
			var valid bool
			if readErr == nil {
				inputSession, valid = reviewerResultInputSession(data)
			}
			if !valid {
				inputState = "invalid"
			} else {
				inputSHA256 = reviewerDispatchBytesSHA256(data)
				inputBytes = len(data)
			}
		}
	}
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
				state = "reviewer-result-recovery-disposed-ready-for-collection-preview"
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
	recoveryProjected := state == "reviewer-result-recovery-invalid" || state == "reviewer-result-recovery-ambiguous" || state == "reviewer-result-recovery-finalize-required" || state == "reviewer-result-recovery-disposed-ready-for-collection-preview" || state == "reviewer-result-collection-required"
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
	} else if !recoveryProjected && !present && inputState == "invalid" {
		state = "reviewer-result-input-invalid"
	} else if !recoveryProjected && !present && inputState == "ready" && sourceCaptureCommand != "" {
		state = "ready-for-reviewer-result-source-capture-preview"
	}
	promptArtifact := reviewerDispatchPromptArtifactStatus(caseRoot, packetPath, dispatch)
	currentExecutor, currentGeneration := reviewerDispatchCurrentOwner(caseRoot, targetLane)
	sessionLifecycle := reviewerDispatchSessionLifecycle(caseRoot, packet, packetPath, targetLane, dispatch, inputPath, inputState, inputSession, inputSHA256, inputBytes, currentExecutor, currentGeneration)
	if (sessionLifecycle.state == "reviewer-session-failed" || sessionLifecycle.state == "reviewer-session-receipt-owner-stale") && strings.TrimSpace(sessionLifecycle.dispatchCommand) == "" {
		sessionLifecycle.dispatchCommand = reviewerDispatchSessionRecordCommand(packetPath, dispatch.ShardID, targetLane)
	}
	if !recoveryProjected && sessionLifecycle.state != "" && reviewerDispatchSessionLifecycleCanProject(state) {
		switch sessionLifecycle.state {
		case "reviewer-session-completed":
			if !present && inputState == "ready" && sourceCaptureCommand != "" {
				state = "ready-for-reviewer-result-source-capture-preview"
			}
		default:
			state = sessionLifecycle.state
		}
	}
	if reviewerDispatchPromptArtifactBlocksDispatch(promptArtifact, state) {
		state = reviewerDispatchPromptArtifactBlockedState(promptArtifact)
	}
	adoptionPath := filepath.Join(caseRoot, ".rekit", "reviewer-adoptions", packet.PacketID+".json")
	adoption, adoptionCurrent := reviewerDispatchCurrentAdoption(caseRoot, adoptionPath, packet, packetPath, currentExecutor, currentGeneration)
	ownerStale := currentExecutor != strings.TrimSpace(packet.ReviewerOrchestration.OwnerBinding.CurrentExecutor) ||
		currentGeneration != packet.ReviewerOrchestration.OwnerBinding.ExecutorGeneration
	if ownerStale && !adoptionCurrent {
		state = "reviewer-packet-owner-adoption-required"
	}
	dispatchCommand := reviewerDispatchCommand(dispatch.ShardID, sourceCaptureCommand, sourceCaptureApplyCommand, stagingCommand, inputPath, sourcePath, candidatePath, resultPath, dispatch.DispatchPromptPath, dispatch.DispatchPromptSHA256, dispatch.AgentToolRequest, idx)
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
		ReviewerResultInputPath:                  inputPath,
		ReviewerResultInputState:                 inputState,
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
		ReviewerDispatchID:                       sessionLifecycle.dispatchID,
		ReviewerDispatchReceiptPath:              sessionLifecycle.dispatchPath,
		ReviewerDispatchReceiptSHA256:            sessionLifecycle.dispatchSHA256,
		ReviewerHarness:                          sessionLifecycle.harness,
		ReviewerSession:                          sessionLifecycle.session,
		ReviewerSessionOutcome:                   sessionLifecycle.outcome,
		ReviewerSessionExitStatus:                sessionLifecycle.exitStatus,
		ReviewerCompletionReceiptPath:            sessionLifecycle.completionPath,
		ReviewerCompletionReceiptSHA256:          sessionLifecycle.completionSHA256,
		ReviewerSessionReceiptState:              sessionLifecycle.state,
		ReviewerSessionReceiptFailure:            sessionLifecycle.failure,
		ReviewerDispatchRecordCommand:            sessionLifecycle.dispatchCommand,
		ReviewerCompletionRecordCommand:          sessionLifecycle.completionCommand,
		AgentToolRequest:                         dispatch.AgentToolRequest,
		ReviewerResultInputSaveCommand:           inputSaveCommand,
		ReviewerResultInputSaveApplyCommand:      inputSaveApplyCommand,
		ReviewerResultSourceCaptureCommand:       sourceCaptureCommand,
		ReviewerResultSourceCaptureApplyCommand:  sourceCaptureApplyCommand,
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
		RefreshStatusCommand:                     reviewerDispatchStatusCommand(caseRoot),
		OwnerExecutor:                            packet.ReviewerOrchestration.OwnerBinding.CurrentExecutor,
		OwnerGeneration:                          packet.ReviewerOrchestration.OwnerBinding.ExecutorGeneration,
		OwnerBindingMode:                         packet.ReviewerOrchestration.OwnerBinding.BindingMode,
		CurrentExecutor:                          currentExecutor,
		CurrentGeneration:                        currentGeneration,
		OwnerAdoptionRequired:                    ownerStale && !adoptionCurrent,
		OwnerAdoptionCurrent:                     ownerStale && adoptionCurrent,
		OwnerAdoptionPath:                        adoptionPath,
		OwnerAdoptionActor:                       adoption.Actor,
		OwnerAdoptionReason:                      adoption.Reason,
		OwnerAdoptionCreatedAt:                   adoption.CreatedAt,
		OwnerAdoptionPreviewCommand:              reviewerDispatchAdoptionPreviewCommand(packetPath, targetLane),
		ManagedDispatch:                          reviewerManagedDispatchHandoffFor(packet, dispatch.ShardID),
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

func reviewerManagedDispatchHandoffFor(packet reviewerDispatchPacket, shardID string) *ReviewerManagedDispatchHandoff {
	managedPacket := packet.ReviewerOrchestration.ManagedDispatchPacket
	if managedPacket == nil {
		return nil
	}
	for _, dispatch := range managedPacket.Dispatches {
		if dispatch.ShardID != shardID {
			continue
		}
		request := dispatch.AgentToolRequest
		return &ReviewerManagedDispatchHandoff{
			Mode:                        managedPacket.Mode,
			Scope:                       managedPacket.Scope,
			TargetLane:                  managedPacket.TargetLane,
			PacketPath:                  managedPacket.PacketPath,
			PromptRoot:                  managedPacket.PromptRoot,
			ResultRoot:                  managedPacket.ResultRoot,
			ReviewerCount:               managedPacket.ReviewerCount,
			MaxParallel:                 managedPacket.MaxParallel,
			Runbook:                     append([]string{}, managedPacket.Runbook...),
			CompletionCriteria:          append([]string{}, managedPacket.CompletionCriteria...),
			ShardID:                     dispatch.ShardID,
			ReviewerRole:                dispatch.ReviewerRole,
			Status:                      dispatch.Status,
			Items:                       append([]string{}, dispatch.Items...),
			PromptPath:                  dispatch.PromptPath,
			PromptSHA256:                dispatch.PromptSHA256,
			AgentToolRequest:            request,
			ReviewerResultPath:          dispatch.ReviewerResultPath,
			ReviewerResultCandidatePath: dispatch.ReviewerResultCandidatePath,
			ReviewerResultInputPath:     dispatch.ReviewerResultInputPath,
			ReviewerResultSourcePath:    dispatch.ReviewerResultSourcePath,
			InputSavePreviewCommand:     dispatch.InputSavePreviewCommand,
			InputSaveApplyCommand:       dispatch.InputSaveApplyCommand,
			SourceCapturePreviewCommand: dispatch.SourceCapturePreviewCommand,
			SourceCaptureApplyCommand:   dispatch.SourceCaptureApplyCommand,
			StagingPreviewCommand:       dispatch.StagingPreviewCommand,
			CollectionPreviewCommand:    dispatch.CollectionPreviewCommand,
			CollectionApplyCommand:      dispatch.CollectionApplyCommand,
			IntakePreviewCommand:        dispatch.IntakePreviewCommand,
			IntakeApplyCommand:          dispatch.IntakeApplyCommand,
			DispatchCommand:             dispatch.DispatchCommand,
			ReviewerResultSkeleton:      dispatch.ReviewerResultSkeleton,
			ExpectedOutput:              dispatch.ExpectedOutput,
			NextAction:                  dispatch.NextAction,
			Boundary:                    mission.UniqueStrings(append(append([]string{}, managedPacket.Boundary...), dispatch.Boundary...)),
		}
	}
	return nil
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
	_, current := reviewerDispatchCurrentAdoption(caseRoot, path, packet, packetPath, currentExecutor, currentGeneration)
	return current
}

func reviewerDispatchCurrentAdoption(caseRoot, path string, packet reviewerDispatchPacket, packetPath, currentExecutor string, currentGeneration int) (reviewerPacketOwnerAdoption, bool) {
	data, err := os.ReadFile(path)
	if err != nil {
		return reviewerPacketOwnerAdoption{}, false
	}
	var adoption reviewerPacketOwnerAdoption
	decoder := json.NewDecoder(bytes.NewReader(bytes.TrimSpace(data)))
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(&adoption); err != nil {
		return reviewerPacketOwnerAdoption{}, false
	}
	var trailing any
	if err := decoder.Decode(&trailing); err != io.EOF {
		return reviewerPacketOwnerAdoption{}, false
	}
	if _, err := time.Parse(time.RFC3339Nano, adoption.CreatedAt); err != nil {
		return reviewerPacketOwnerAdoption{}, false
	}
	packetBytes, err := os.ReadFile(packetPath)
	if err != nil {
		return reviewerPacketOwnerAdoption{}, false
	}
	sum := sha256.Sum256(packetBytes)
	owner := packet.ReviewerOrchestration.OwnerBinding
	current := adoption.SchemaVersion == 1 &&
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
	if !current {
		return reviewerPacketOwnerAdoption{}, false
	}
	return adoption, true
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

func reviewerDispatchPacketRetirementPreviewCommand(packetPath, lane string) string {
	return "/rekit plan-subagents -PacketPath " + quoteCommandArg(packetPath) + " -RetireInvalidReviewerPacket -Lane " + quoteCommandArg(lane) + " -Actor <actor> -Reason <reason> -WhatIf -Format json"
}

func reviewerDispatchIntakeCommandAvailable(command string) bool {
	command = strings.TrimSpace(command)
	return command != "" && !strings.HasPrefix(command, "n/a:")
}

func reviewerDispatchSessionLifecycleCanProject(state string) bool {
	return state == "waiting-for-reviewer-result" ||
		state == "dispatch-only-waiting-for-result" ||
		state == "ready-for-reviewer-result-source-capture-preview" ||
		state == "ready-for-reviewer-intake-preview"
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
	if item.OwnerAdoptionRequired {
		evidence = append(evidence, fmt.Sprintf("ownerAdoption required ownerExecutor=%s generation=%d currentExecutor=%s generation=%d", firstText(item.OwnerExecutor, "unassigned"), item.OwnerGeneration, firstText(item.CurrentExecutor, "unassigned"), item.CurrentGeneration))
	} else if item.OwnerAdoptionCurrent {
		evidence = append(evidence, fmt.Sprintf("ownerAdoption current %s adoptedExecutor=%s generation=%d actor=%s reason=%s createdAt=%s", reviewerDispatchDisplayPath(caseRoot, item.OwnerAdoptionPath), firstText(item.CurrentExecutor, "unassigned"), item.CurrentGeneration, item.OwnerAdoptionActor, item.OwnerAdoptionReason, item.OwnerAdoptionCreatedAt))
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
	if strings.TrimSpace(item.ReviewerDispatchReceiptPath) != "" {
		evidence = append(evidence, fmt.Sprintf("reviewerSession dispatchId=%s harness=%s session=%s state=%s dispatchReceipt=%s sha256=%s", item.ReviewerDispatchID, item.ReviewerHarness, item.ReviewerSession, item.ReviewerSessionReceiptState, reviewerDispatchDisplayPath(caseRoot, item.ReviewerDispatchReceiptPath), item.ReviewerDispatchReceiptSHA256))
	}
	if strings.TrimSpace(item.ReviewerCompletionReceiptPath) != "" {
		evidence = append(evidence, fmt.Sprintf("reviewerSession completion outcome=%s exitStatus=%s receipt=%s sha256=%s", item.ReviewerSessionOutcome, item.ReviewerSessionExitStatus, reviewerDispatchDisplayPath(caseRoot, item.ReviewerCompletionReceiptPath), item.ReviewerCompletionReceiptSHA256))
	}
	if strings.TrimSpace(item.ReviewerResultInputPath) != "" {
		evidence = append(evidence, "reviewerResultInput "+firstText(item.ReviewerResultInputState, "missing")+" "+reviewerDispatchDisplayPath(caseRoot, item.ReviewerResultInputPath))
	}
	if strings.TrimSpace(item.ReviewerResultSourcePath) != "" {
		evidence = append(evidence, "reviewerResultSource "+firstText(item.ReviewerResultSourceState, "missing")+" "+reviewerDispatchDisplayPath(caseRoot, item.ReviewerResultSourcePath))
	}
	if strings.TrimSpace(item.ReviewerResultCandidatePath) != "" {
		evidence = append(evidence, "reviewerResultCandidate "+firstText(item.ReviewerResultCandidateState, "missing")+" "+reviewerDispatchDisplayPath(caseRoot, item.ReviewerResultCandidatePath))
	}
	if item.ManagedDispatch != nil {
		evidence = append(evidence, fmt.Sprintf("managedDispatch packet %s shard=%s reviewer=%s maxParallel=%d", reviewerDispatchDisplayPath(caseRoot, item.ManagedDispatch.PacketPath), item.ManagedDispatch.ShardID, item.ManagedDispatch.ReviewerRole, item.ManagedDispatch.MaxParallel))
	}
	if strings.TrimSpace(item.ReviewerResultPath) != "" {
		state := "missing"
		if item.ReviewerResultPresent {
			state = "present"
		}
		evidence = append(evidence, "reviewerResult "+state+" "+reviewerDispatchDisplayPath(caseRoot, item.ReviewerResultPath))
	}
	if strings.TrimSpace(item.ReviewerResultRecoveryDispositionPath) != "" {
		evidence = append(evidence, "reviewerResultRecoveryDisposition current "+reviewerDispatchDisplayPath(caseRoot, item.ReviewerResultRecoveryDispositionPath))
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
	if item.ReviewerSessionReceiptState != "" {
		boundary = append(boundary, "reviewer session receipts are immutable harness observations; runtime does not spawn, poll, monitor, stop, or manage the recorded session")
		boundary = append(boundary, "successful completion receipt and exact current input binding are required before source capture; failed or stale-owner attempts cannot enter intake")
	}
	if item.ManagedDispatch != nil {
		boundary = append(boundary, "managed dispatch packet is read-only recovery context; status/handoff/continue do not spawn reviewers or execute managed dispatch commands")
		boundary = append(boundary, "runtime-rebuilt reviewer intake handoff commands remain the executable WhatIf/Apply source; managed dispatch commands are provenance for replacement executor takeover")
	}
	if item.ReviewerResultCollectionCommands != nil {
		inputTarget := firstText(item.ReviewerResultInputPath, "the packet-derived reviewerResultInputPath")
		boundary = append(boundary,
			"reviewer returns one JSON object; the main agent saves it to "+inputTarget+", runs source capture -WhatIf, and uses its expected-input-hash -Apply command to publish the packet-derived source",
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
	if item.OwnerAdoptionCurrent {
		boundary = append(boundary, "current reviewer packet adoption receipt transfers strict intake ownership without mutating the immutable packet or spawning reviewers")
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

func reviewerDispatchOperatorPackageFor(item ReviewerDispatchIntakeHandoff) *ReviewerDispatchOperatorPackage {
	managed := item.ManagedDispatch
	if managed == nil || (item.VerificationRecorded && item.DecisionRecorded) {
		return nil
	}
	current := ReviewerDispatchOperatorPackageItem{
		ShardID:                                   item.ShardID,
		State:                                     item.State,
		ReviewerRole:                              managed.ReviewerRole,
		Status:                                    managed.Status,
		Items:                                     append([]string{}, managed.Items...),
		DispatchPromptPath:                        firstText(item.DispatchPromptPath, managed.PromptPath),
		DispatchPromptSHA256:                      firstText(item.DispatchPromptSHA256, managed.PromptSHA256),
		DispatchPromptState:                       item.DispatchPromptState,
		DispatchPromptCurrent:                     item.DispatchPromptCurrent,
		DispatchPromptActualSHA256:                item.DispatchPromptActualSHA256,
		DispatchPromptFailure:                     item.DispatchPromptFailure,
		DispatchPromptRepairCommand:               item.DispatchPromptRepairCommand,
		ReviewerDispatchID:                        item.ReviewerDispatchID,
		ReviewerDispatchReceiptPath:               item.ReviewerDispatchReceiptPath,
		ReviewerDispatchReceiptSHA256:             item.ReviewerDispatchReceiptSHA256,
		ReviewerHarness:                           item.ReviewerHarness,
		ReviewerSession:                           item.ReviewerSession,
		ReviewerSessionOutcome:                    item.ReviewerSessionOutcome,
		ReviewerSessionExitStatus:                 item.ReviewerSessionExitStatus,
		ReviewerCompletionReceiptPath:             item.ReviewerCompletionReceiptPath,
		ReviewerCompletionReceiptSHA256:           item.ReviewerCompletionReceiptSHA256,
		ReviewerSessionReceiptState:               item.ReviewerSessionReceiptState,
		ReviewerSessionReceiptFailure:             item.ReviewerSessionReceiptFailure,
		ReviewerDispatchRecordCommand:             item.ReviewerDispatchRecordCommand,
		ReviewerCompletionRecordCommand:           item.ReviewerCompletionRecordCommand,
		AgentToolRequest:                          reviewerDispatchOperatorAgentToolRequest(item, managed),
		ExpectedReviewerResultSkeleton:            managed.ReviewerResultSkeleton,
		ExpectedOutput:                            firstText(managed.ExpectedOutput, reviewerAgentToolRequestExpectedOutput(item.AgentToolRequest), reviewerAgentToolRequestExpectedOutput(managed.AgentToolRequest)),
		ReviewerResultDropPath:                    firstText(item.ReviewerResultInputPath, managed.ReviewerResultInputPath, item.ReviewerResultPath, managed.ReviewerResultPath),
		ReviewerResultInputPath:                   firstText(item.ReviewerResultInputPath, managed.ReviewerResultInputPath),
		ReviewerResultInputState:                  item.ReviewerResultInputState,
		ReviewerResultSourcePath:                  firstText(item.ReviewerResultSourcePath, managed.ReviewerResultSourcePath),
		ReviewerResultSourceState:                 item.ReviewerResultSourceState,
		ReviewerResultCandidatePath:               firstText(item.ReviewerResultCandidatePath, managed.ReviewerResultCandidatePath),
		ReviewerResultCandidateState:              item.ReviewerResultCandidateState,
		ReviewerResultPath:                        firstText(item.ReviewerResultPath, managed.ReviewerResultPath),
		ReviewerResultState:                       item.ReviewerResultState,
		ReviewerResultPresent:                     item.ReviewerResultPresent,
		ReviewerResultInputSavePreviewCommand:     firstText(item.ReviewerResultInputSaveCommand, managed.InputSavePreviewCommand),
		ReviewerResultInputSaveApplyCommand:       firstText(item.ReviewerResultInputSaveApplyCommand, managed.InputSaveApplyCommand),
		ReviewerResultSourceCapturePreviewCommand: firstText(item.ReviewerResultSourceCaptureCommand, managed.SourceCapturePreviewCommand),
		ReviewerResultSourceCaptureApplyCommand:   firstText(item.ReviewerResultSourceCaptureApplyCommand, managed.SourceCaptureApplyCommand),
		ReviewerResultStagingPreviewCommand:       firstText(item.ReviewerResultStagingCommand, managed.StagingPreviewCommand),
		ReviewerResultIntakePreviewCommand:        firstText(item.PreviewCommand, managed.IntakePreviewCommand),
		ReviewerResultIntakeApplyCommand:          firstText(item.ApplyCommand, managed.IntakeApplyCommand),
		ReviewerResultBatchIntakePreviewCommand:   item.BatchPreviewCommand,
		ReviewerResultBatchIntakeApplyCommand:     item.BatchApplyCommand,
		DispatchCommand:                           item.DispatchCommand,
		NextAction:                                reviewerDispatchIntakeNextAction(item),
	}
	if item.ReviewerResultCollectionCommands != nil {
		current.ReviewerResultCollectionPreviewCommand = firstText(item.ReviewerResultCollectionCommands.PreviewCommand, managed.CollectionPreviewCommand)
		current.ReviewerResultCollectionApplyCommand = firstText(item.ReviewerResultCollectionCommands.ApplyCommand, managed.CollectionApplyCommand)
	} else {
		current.ReviewerResultCollectionPreviewCommand = managed.CollectionPreviewCommand
		current.ReviewerResultCollectionApplyCommand = managed.CollectionApplyCommand
	}
	runbook := append([]string{}, managed.Runbook...)
	runbook = append(runbook, item.RunbookSteps...)
	criteria := append([]string{}, managed.CompletionCriteria...)
	criteria = append(criteria,
		"save exactly one reviewer JSON object to reviewerResultDropPath before source capture, staging, collection, or intake",
		"rerun status or continue until reviewerDispatchIntakeSummary.total is zero before ordinary lane continuation",
	)
	boundary := append([]string{}, reviewerDispatchIntakeSummaryBoundary()...)
	boundary = append(boundary, item.Boundary...)
	boundary = append(boundary,
		"reviewer dispatch operator package is read-only; it does not call Agent tool, spawn reviewers, monitor sessions, or execute reviewer work",
		"operator package does not write facts, authority, confirmed state, or execute heavy tools",
	)
	runLoop := reviewerDispatchOperatorRunLoop(current)
	currentRunLoopStepID := reviewerDispatchCurrentRunLoopStepID(item)
	boundary = mission.UniqueStrings(boundary)
	return &ReviewerDispatchOperatorPackage{
		Ready:                true,
		Summary:              fmt.Sprintf("managed reviewer dispatch operator package ready: packet=%s shard=%s state=%s", firstText(item.PacketID, item.PacketPath), item.ShardID, item.State),
		PacketID:             item.PacketID,
		PacketPath:           firstText(item.PacketPath, managed.PacketPath),
		TargetLane:           firstText(item.TargetLane, managed.TargetLane),
		Current:              &current,
		CurrentRunLoopStepID: currentRunLoopStepID,
		CurrentDriverRequest: reviewerDispatchOperatorCurrentDriverRequest(item, current, currentRunLoopStepID, runLoop, boundary),
		RefreshStatusCommand: strings.TrimSpace(item.RefreshStatusCommand),
		RunLoop:              runLoop,
		RunbookSteps:         mission.UniqueStrings(runbook),
		CompletionCriteria:   mission.UniqueStrings(criteria),
		Boundary:             boundary,
	}
}

func reviewerDispatchOperatorCurrentDriverRequest(item ReviewerDispatchIntakeHandoff, current ReviewerDispatchOperatorPackageItem, currentRunLoopStepID string, runLoop []ReviewerDispatchRunLoopStep, boundary []string) *mission.MissionCommanderDriverRequest {
	currentRunLoopStepID = strings.TrimSpace(currentRunLoopStepID)
	if currentRunLoopStepID == "" {
		return nil
	}
	command := reviewerDispatchOperatorRunLoopStepCommand(currentRunLoopStepID, runLoop)
	label := firstText(item.PacketID, item.PacketPath, current.ShardID)
	actionID := strings.TrimSpace(firstText(item.PacketID, item.PacketPath))
	if actionID == "" {
		actionID = strings.TrimSpace(current.ShardID)
	} else if strings.TrimSpace(current.ShardID) != "" {
		actionID = strings.TrimSpace(actionID + ":" + current.ShardID)
	}
	action := mission.MissionCommanderNextActionItem{
		Lane:           item.TargetLane,
		Label:          label,
		ActionID:       actionID,
		State:          current.State,
		Command:        command,
		Source:         "reviewerDispatchOperatorPackage",
		RequiresReview: true,
		Reasons: []string{
			"consume the current reviewer dispatch operator run-loop step from this typed driver request",
		},
		Boundary: boundary,
	}
	request := mission.MissionCommanderCurrentDriverRequest(action, currentRunLoopStepID, reviewerDispatchOperatorMissionRunLoop(runLoop))
	if request == nil {
		return nil
	}
	refreshed := mission.MissionCommanderDriverRequestWithRefreshStatusCommand(*request, item.RefreshStatusCommand)
	return &refreshed
}

func reviewerDispatchOperatorRunLoopStepCommand(stepID string, runLoop []ReviewerDispatchRunLoopStep) string {
	stepID = strings.TrimSpace(stepID)
	for _, step := range runLoop {
		if step.StepID != stepID {
			continue
		}
		return firstText(step.Command, step.PreviewCommand, step.ApplyCommand)
	}
	return ""
}

func reviewerDispatchOperatorMissionRunLoop(runLoop []ReviewerDispatchRunLoopStep) []mission.MissionCommanderRunLoopStep {
	steps := []mission.MissionCommanderRunLoopStep{}
	for _, step := range runLoop {
		steps = append(steps, mission.MissionCommanderRunLoopStep{
			StepID:      strings.TrimSpace(step.StepID),
			Order:       step.Order,
			Actor:       strings.TrimSpace(step.Actor),
			Description: strings.TrimSpace(step.Description),
			Command:     firstText(step.Command, step.PreviewCommand, step.ApplyCommand),
			Source:      "reviewerDispatchOperatorPackage.runLoop",
			Boundary:    mission.UniqueStrings(step.Boundary),
		})
	}
	return steps
}

func reviewerDispatchOperatorPromptRepairCommand(current ReviewerDispatchOperatorPackageItem) string {
	switch strings.TrimSpace(current.State) {
	case "reviewer-dispatch-prompt-artifact-invalid", "reviewer-dispatch-prompt-artifact-drift":
		return current.DispatchPromptRepairCommand
	}
	state := strings.TrimSpace(current.DispatchPromptState)
	if current.DispatchPromptCurrent && (state == "" || state == "ready") {
		return ""
	}
	return current.DispatchPromptRepairCommand
}

func reviewerDispatchRunLoopRequiresCurrentPrompt(state string) bool {
	switch state {
	case "ready-for-reviewer-dispatch", "waiting-for-reviewer-result", "dispatch-only-waiting-for-result", "reviewer-session-failed", "reviewer-session-receipt-owner-stale":
		return true
	default:
		return false
	}
}

func reviewerDispatchCurrentRunLoopStepID(item ReviewerDispatchIntakeHandoff) string {
	if strings.TrimSpace(item.DispatchPromptPath) != "" && !item.DispatchPromptCurrent && reviewerDispatchRunLoopRequiresCurrentPrompt(item.State) {
		return "verify-prompt"
	}
	switch item.State {
	case "reviewer-dispatch-prompt-artifact-invalid", "reviewer-dispatch-prompt-artifact-drift":
		return "verify-prompt"
	case "ready-for-reviewer-dispatch", "waiting-for-reviewer-result", "dispatch-only-waiting-for-result":
		return "spawn-reviewer"
	case "reviewer-session-running-unknown":
		return "save-result-input"
	case "ready-for-reviewer-completion-receipt-preview":
		return "record-completion"
	case "reviewer-session-failed", "reviewer-session-receipt-owner-stale":
		return "spawn-reviewer"
	case "ready-for-reviewer-result-source-capture-preview":
		return "source-capture"
	case "ready-for-reviewer-result-staging-preview":
		return "stage-candidate"
	case "ready-for-reviewer-result-collection-preview", "reviewer-result-recovery-disposed-ready-for-collection-preview":
		return "collect-result"
	case "ready-for-reviewer-intake-preview":
		return "intake-results"
	default:
		return ""
	}
}

func reviewerDispatchOperatorRunLoop(current ReviewerDispatchOperatorPackageItem) []ReviewerDispatchRunLoopStep {
	steps := []ReviewerDispatchRunLoopStep{}
	add := func(step ReviewerDispatchRunLoopStep) {
		step.StepID = strings.TrimSpace(step.StepID)
		step.Actor = strings.TrimSpace(step.Actor)
		step.Description = strings.TrimSpace(step.Description)
		if step.StepID == "" || step.Description == "" {
			return
		}
		step.Order = len(steps) + 1
		step.Boundary = mission.UniqueStrings(step.Boundary)
		steps = append(steps, step)
	}
	if strings.TrimSpace(current.DispatchPromptPath) != "" {
		add(ReviewerDispatchRunLoopStep{
			StepID:         "verify-prompt",
			Actor:          "main-agent",
			Description:    "verify the packet-derived reviewer prompt artifact is current before reviewer dispatch",
			PreviewCommand: reviewerDispatchOperatorPromptRepairCommand(current),
			Path:           current.DispatchPromptPath,
			Boundary: []string{
				"prompt artifact must be present, non-symlink, non-empty, and match dispatchPromptSha256 before dispatch",
				"if promptCurrent is false, run the prompt repair WhatIf/Apply path before invoking the Agent tool",
			},
		})
	}
	add(ReviewerDispatchRunLoopStep{
		StepID:           "spawn-reviewer",
		Actor:            "main-agent-harness",
		Description:      "invoke the read-only Agent tool request for this shard and obtain exactly one ReviewerResult JSON object",
		Command:          current.DispatchCommand,
		AgentToolRequest: current.AgentToolRequest,
		Boundary: []string{
			"the main Agent or harness performs the Agent tool call; Go runtime does not spawn, poll, monitor, stop, or manage reviewer sessions",
			"the reviewer must not write files, facts, authority, confirmed state, or execute heavy tools",
			"agentToolRequest is a read-only handoff for the external harness and is not executed by /rekit",
		},
	})
	add(ReviewerDispatchRunLoopStep{
		StepID:         "record-dispatch",
		Actor:          "main-agent",
		Description:    "after the harness accepts the reviewer session, record the immutable dispatch receipt",
		PreviewCommand: current.ReviewerDispatchRecordCommand,
		Path:           current.ReviewerDispatchReceiptPath,
		Boundary: []string{
			"record dispatch only after a real harness/session id exists; use the hash-bound Apply command returned by preview",
			"dispatch receipt is an immutable harness observation and does not spawn or rerun the reviewer",
		},
	})
	add(ReviewerDispatchRunLoopStep{
		StepID:         "save-result-input",
		Actor:          "main-agent",
		Description:    "save the reviewer JSON object to the packet-derived reviewer result input path",
		PreviewCommand: current.ReviewerResultInputSavePreviewCommand,
		ApplyCommand:   current.ReviewerResultInputSaveApplyCommand,
		Path:           firstText(current.ReviewerResultInputPath, current.ReviewerResultDropPath),
		Boundary: []string{
			"save exactly one reviewer JSON object before completion receipt, source capture, staging, collection, or intake",
			"the saved input must contain the same reviewerSession bound to the dispatch receipt",
		},
	})
	add(ReviewerDispatchRunLoopStep{
		StepID:         "record-completion",
		Actor:          "main-agent",
		Description:    "when the reviewer session exits, record succeeded or failed completion before any source capture",
		PreviewCommand: current.ReviewerCompletionRecordCommand,
		Path:           current.ReviewerCompletionReceiptPath,
		Boundary: []string{
			"successful completion requires exact dispatch receipt and reviewer result input path/hash/bytes bindings",
			"failed or stale-owner completion cannot enter source capture, collection, or intake",
		},
	})
	add(ReviewerDispatchRunLoopStep{
		StepID:         "source-capture",
		Actor:          "main-agent",
		Description:    "publish the exact completed reviewer input to the packet-derived source path",
		PreviewCommand: current.ReviewerResultSourceCapturePreviewCommand,
		ApplyCommand:   current.ReviewerResultSourceCaptureApplyCommand,
		Path:           current.ReviewerResultSourcePath,
		Boundary: []string{
			"source capture requires a successful current completion receipt and uses the expected input hash returned by preview",
			"source capture does not collect reviewer result or write verification/decision facts",
		},
	})
	add(ReviewerDispatchRunLoopStep{
		StepID:         "stage-candidate",
		Actor:          "main-agent",
		Description:    "stage the packet-derived source into the reviewer result candidate path",
		PreviewCommand: current.ReviewerResultStagingPreviewCommand,
		Path:           current.ReviewerResultCandidatePath,
		Boundary: []string{
			"staging uses only the packet-derived source path and the expected source hash returned by preview",
			"staging publishes a candidate for later collection; it does not write facts",
		},
	})
	add(ReviewerDispatchRunLoopStep{
		StepID:         "collect-result",
		Actor:          "main-agent",
		Description:    "collect the staged candidate into the immutable canonical reviewer result path",
		PreviewCommand: current.ReviewerResultCollectionPreviewCommand,
		ApplyCommand:   current.ReviewerResultCollectionApplyCommand,
		Path:           current.ReviewerResultPath,
		Boundary: []string{
			"collection is no-overwrite and exact-byte bound; different canonical bytes require reviewer result recovery",
			"collection does not perform reviewer intake or write authority/confirmed state",
		},
	})
	add(ReviewerDispatchRunLoopStep{
		StepID:         "intake-results",
		Actor:          "main-agent",
		Description:    "run reviewer intake WhatIf, inspect verification/decision previews, then apply the bounded writeback command",
		PreviewCommand: firstText(current.ReviewerResultBatchIntakePreviewCommand, current.ReviewerResultIntakePreviewCommand),
		ApplyCommand:   firstText(current.ReviewerResultBatchIntakeApplyCommand, current.ReviewerResultIntakeApplyCommand),
		Path:           current.ReviewerResultPath,
		Boundary: []string{
			"intake must run WhatIf before Apply and stops at the first blocked, partial, or invalid shard",
			"intake writes verification/decision facts only; it does not write authority/confirmed state or execute heavy tools",
		},
	})
	return steps
}

func reviewerDispatchOperatorAgentToolRequest(item ReviewerDispatchIntakeHandoff, managed *ReviewerManagedDispatchHandoff) *ReviewerAgentToolRequest {
	request := item.AgentToolRequest
	if request == nil && managed != nil {
		request = managed.AgentToolRequest
	}
	if request == nil {
		return nil
	}
	copy := *request
	if managed != nil {
		if strings.TrimSpace(copy.PromptPath) == "" {
			copy.PromptPath = firstText(item.DispatchPromptPath, managed.PromptPath)
		}
		if strings.TrimSpace(copy.PromptSHA256) == "" {
			copy.PromptSHA256 = firstText(item.DispatchPromptSHA256, managed.PromptSHA256)
		}
	}
	return &copy
}

func reviewerAgentToolRequestExpectedOutput(request *ReviewerAgentToolRequest) string {
	if request == nil {
		return ""
	}
	return request.ExpectedOutput
}

func reviewerDispatchOperatorPackageMarkdownLines(pkg *ReviewerDispatchOperatorPackage) []string {
	if pkg == nil || !pkg.Ready || pkg.Current == nil {
		return nil
	}
	current := *pkg.Current
	lines := []string{fmt.Sprintf("- operator package: ready=%t packet=%s lane=%s shard=%s state=%s currentRunLoopStep=%s prompt=`%s` promptSha256=%s input=`%s` source=`%s` candidate=`%s` result=`%s` nextAction=`%s`", pkg.Ready, firstText(pkg.PacketID, pkg.PacketPath), pkg.TargetLane, current.ShardID, current.State, pkg.CurrentRunLoopStepID, current.DispatchPromptPath, current.DispatchPromptSHA256, current.ReviewerResultInputPath, current.ReviewerResultSourcePath, current.ReviewerResultCandidatePath, current.ReviewerResultPath, current.NextAction)}
	if request := pkg.CurrentDriverRequest; request != nil {
		lines = append(lines, fmt.Sprintf("  - driver request：kind=%s step=%s actor=%s executable=%t blocked=%t requiresReview=%t command=`%s` guidance=`%s` state=%s source=%s lane=%s label=%s gateEventId=%s actionId=%s", request.Kind, request.RunLoopStepID, request.Actor, request.CommandExecutable, request.Blocked, request.RequiresReview, request.Command, request.Guidance, request.State, request.Source, request.Lane, request.Label, request.GateEventID, request.ActionID))
		lines = append(lines, fmt.Sprintf("  - driver request expected receipt：state=%s command=`%s` refreshStatusCommand=`%s` description=%s", request.ExpectedReceipt.State, request.ExpectedReceipt.Command, request.ExpectedReceipt.RefreshStatusCommand, request.ExpectedReceipt.Description))
		if strings.TrimSpace(pkg.RefreshStatusCommand) != "" {
			lines = append(lines, "  - operator refresh status command：`"+pkg.RefreshStatusCommand+"`")
		}
		for _, boundary := range mission.LimitStrings(request.Boundary, maxHandoffRows) {
			lines = append(lines, "  - driver request boundary："+boundary)
		}
		for _, boundary := range mission.LimitStrings(request.ExpectedReceipt.Boundary, maxHandoffRows) {
			lines = append(lines, "  - driver request expected receipt boundary："+boundary)
		}
	}
	if current.AgentToolRequest != nil {
		request := current.AgentToolRequest
		lines = append(lines, fmt.Sprintf("  - operator agent tool: tool=%s agentType=%s readOnly=%t promptPath=`%s` promptSha256=%s expectedOutput=%s", request.Tool, request.AgentType, request.ReadOnly, request.PromptPath, request.PromptSHA256, request.ExpectedOutput))
	}
	if strings.TrimSpace(current.ReviewerSessionReceiptState) != "" {
		lines = append(lines, fmt.Sprintf("  - operator reviewer session: state=%s dispatchId=%s harness=%s session=%s outcome=%s exitStatus=%s dispatchReceipt=`%s` dispatchSha256=%s completionReceipt=`%s` completionSha256=%s failure=%s", current.ReviewerSessionReceiptState, current.ReviewerDispatchID, current.ReviewerHarness, current.ReviewerSession, current.ReviewerSessionOutcome, current.ReviewerSessionExitStatus, current.ReviewerDispatchReceiptPath, current.ReviewerDispatchReceiptSHA256, current.ReviewerCompletionReceiptPath, current.ReviewerCompletionReceiptSHA256, current.ReviewerSessionReceiptFailure))
	}
	if strings.TrimSpace(current.ReviewerDispatchRecordCommand) != "" {
		lines = append(lines, "  - operator dispatch receipt preview: `"+current.ReviewerDispatchRecordCommand+"`")
	}
	if strings.TrimSpace(current.ReviewerCompletionRecordCommand) != "" {
		lines = append(lines, "  - operator completion receipt preview: `"+current.ReviewerCompletionRecordCommand+"`")
	}
	if strings.TrimSpace(current.ExpectedReviewerResultSkeleton) != "" {
		lines = append(lines, "  - operator expected reviewer result skeleton: `"+current.ExpectedReviewerResultSkeleton+"`")
	}
	if strings.TrimSpace(current.ReviewerResultInputSavePreviewCommand) != "" || strings.TrimSpace(current.ReviewerResultSourceCapturePreviewCommand) != "" || strings.TrimSpace(current.ReviewerResultStagingPreviewCommand) != "" || strings.TrimSpace(current.ReviewerResultCollectionPreviewCommand) != "" {
		lines = append(lines, fmt.Sprintf("  - operator result pipeline: drop=`%s` inputSavePreview=`%s` inputSaveApply=`%s` sourceCapturePreview=`%s` sourceCaptureApply=`%s` stagingPreview=`%s` collectionPreview=`%s` collectionApply=`%s`", current.ReviewerResultDropPath, current.ReviewerResultInputSavePreviewCommand, current.ReviewerResultInputSaveApplyCommand, current.ReviewerResultSourceCapturePreviewCommand, current.ReviewerResultSourceCaptureApplyCommand, current.ReviewerResultStagingPreviewCommand, current.ReviewerResultCollectionPreviewCommand, current.ReviewerResultCollectionApplyCommand))
	}
	if strings.TrimSpace(current.ReviewerResultBatchIntakePreviewCommand) != "" || strings.TrimSpace(current.ReviewerResultIntakePreviewCommand) != "" {
		lines = append(lines, fmt.Sprintf("  - operator intake: preview=`%s` apply=`%s` batchPreview=`%s` batchApply=`%s`", current.ReviewerResultIntakePreviewCommand, current.ReviewerResultIntakeApplyCommand, current.ReviewerResultBatchIntakePreviewCommand, current.ReviewerResultBatchIntakeApplyCommand))
	}
	if strings.TrimSpace(current.DispatchCommand) != "" {
		lines = append(lines, "  - operator dispatch: `"+current.DispatchCommand+"`")
	}
	for _, step := range pkg.RunLoop {
		lines = append(lines, fmt.Sprintf("  - operator run loop step %d: id=%s actor=%s command=`%s` preview=`%s` apply=`%s` path=`%s` description=%s", step.Order, step.StepID, step.Actor, step.Command, step.PreviewCommand, step.ApplyCommand, step.Path, step.Description))
		if step.AgentToolRequest != nil {
			request := step.AgentToolRequest
			lines = append(lines, fmt.Sprintf("    - operator run loop agent tool: step=%s tool=%s agentType=%s readOnly=%t promptPath=`%s` promptSha256=%s expectedOutput=%s", step.StepID, request.Tool, request.AgentType, request.ReadOnly, request.PromptPath, request.PromptSHA256, request.ExpectedOutput))
		}
		for _, boundary := range mission.LimitStrings(step.Boundary, maxHandoffRows) {
			lines = append(lines, "    - operator run loop boundary: "+boundary)
		}
	}
	for idx, step := range mission.LimitStrings(pkg.RunbookSteps, maxHandoffRows) {
		lines = append(lines, fmt.Sprintf("  - operator runbook step %d: %s", idx+1, step))
	}
	for idx, criteria := range mission.LimitStrings(pkg.CompletionCriteria, maxHandoffRows) {
		lines = append(lines, fmt.Sprintf("  - operator completion criteria %d: %s", idx+1, criteria))
	}
	for _, boundary := range mission.LimitStrings(pkg.Boundary, maxHandoffRows) {
		lines = append(lines, "  - operator boundary: "+boundary)
	}
	return lines
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
		add("if the invalid packet is obsolete, run retirement preview: " + firstText(item.PacketRetirementPreviewCommand, "<reviewer-packet-retirement-WhatIf unavailable>"))
		add("review exact packet/integrity hashes from preview; only use the returned hash-bound Apply command to retire this invalid packet snapshot")
		add("otherwise regenerate the canonical reviewer packet and packet.integrity.json together; do not repair packet bytes or integrity metadata independently")
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
	case "ready-for-reviewer-dispatch":
		add("invoke the read-only Agent tool request from this handoff, then record the immutable dispatch receipt: " + firstText(item.ReviewerDispatchRecordCommand, "<reviewer-dispatch-receipt-WhatIf unavailable>"))
		add("use the returned hash-bound Apply only after the harness has actually accepted the reviewer session")
	case "reviewer-session-running-unknown":
		add("the dispatch receipt is durable, but runtime has no live reviewer visibility; inspect the harness session " + firstText(item.ReviewerSession, "<reviewer-session>"))
		add("after a successful read-only reviewer returns JSON, save it through input save preview: " + firstText(item.ReviewerResultInputSaveCommand, "<reviewer-result-input-save-WhatIf unavailable>"))
		add("after success or failure, record a completion receipt before source capture")
	case "ready-for-reviewer-completion-receipt-preview":
		add("reviewer result input is ready; record successful completion preview: " + firstText(item.ReviewerCompletionRecordCommand, "<reviewer-completion-receipt-WhatIf unavailable>"))
		add("review dispatch and input hashes, then use only the returned hash-bound Apply command")
	case "reviewer-session-failed":
		add("do not source-capture the failed attempt; dispatch a new reviewer session and record a distinct dispatch receipt")
	case "reviewer-session-receipt-owner-stale":
		add("the latest reviewer session receipt belongs to a stale lane owner generation; complete owner adoption and dispatch a replacement reviewer session")
	case "reviewer-session-receipt-invalid":
		add("repair or retire the invalid reviewer session receipt namespace before source capture: " + firstText(item.ReviewerSessionReceiptFailure, "receipt validation failed"))
	case "ready-for-reviewer-result-source-capture-preview":
		add("reviewer result input is ready at " + firstText(item.ReviewerResultInputPath, "<reviewer-result-input-path>"))
		add("run source capture preview: " + firstText(item.ReviewerResultSourceCaptureCommand, "<reviewer-result-source-capture-WhatIf unavailable>"))
		add("if preview reports the expected input hash, rerun the returned command with -ExpectedReviewerResultInputSha256 and -Apply to publish reviewerResultSourcePath")
		if item.ReviewerResultStagingCommand != "" {
			add("then rerun status and use staging preview after reviewerResultSourcePath is ready: " + item.ReviewerResultStagingCommand)
		}
	case "ready-for-reviewer-result-staging-preview":
		add("reviewer result source is ready at " + firstText(item.ReviewerResultSourcePath, "<reviewer-result-source-path>"))
		add("run staging preview: " + firstText(item.ReviewerResultStagingCommand, "<reviewer-result-staging-WhatIf unavailable>"))
		add("if preview reports the expected source hash, rerun the returned command with -ExpectedSourceSha256 and -Apply to publish the packet-derived candidate")
		if item.ReviewerResultCollectionCommands != nil {
			add("then run collection preview before apply: " + item.ReviewerResultCollectionCommands.PreviewCommand)
		}
	case "ready-for-reviewer-result-collection-preview", "reviewer-result-recovery-disposed-ready-for-collection-preview":
		if item.State == "reviewer-result-recovery-disposed-ready-for-collection-preview" {
			add("reviewer result recovery disposition is current at " + firstText(item.ReviewerResultRecoveryDispositionPath, "<reviewer-result-recovery-disposition-path>"))
		}
		if item.ReviewerResultCollectionCommands != nil {
			add("run reviewer result collection preview: " + item.ReviewerResultCollectionCommands.PreviewCommand)
			add("if candidate bytes match the packet-derived result, run collection apply: " + item.ReviewerResultCollectionCommands.ApplyCommand)
		} else {
			add("run reviewer result collection -WhatIf for " + item.ShardID + " before reviewer intake")
		}
		add("rerun status or continue; next handoff should become ready-for-reviewer-intake-preview")
	case "ready-for-reviewer-intake-preview":
		if item.OwnerAdoptionCurrent {
			add("owner adoption receipt is current at " + firstText(item.OwnerAdoptionPath, "<owner-adoption-receipt>"))
		}
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
	case "reviewer-result-input-invalid":
		add("replace the invalid reviewer result input with exactly one reviewer JSON object at " + firstText(item.ReviewerResultInputPath, "<reviewer-result-input-path>"))
		add("rerun source capture preview: " + firstText(item.ReviewerResultSourceCaptureCommand, "<reviewer-result-source-capture-WhatIf unavailable>"))
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
			add("collect read-only reviewer JSON for " + item.ShardID + " at " + firstText(item.ReviewerResultInputPath, item.ReviewerResultPath, "<reviewer-result-input-path>"))
		}
		if item.ReviewerResultSourceCaptureCommand != "" {
			add("after saving reviewer JSON input at " + firstText(item.ReviewerResultInputPath, "<reviewer-result-input-path>") + ", run source capture preview: " + item.ReviewerResultSourceCaptureCommand)
			add("if capture preview reports the expected input hash, run the returned or template apply command: " + firstText(item.ReviewerResultSourceCaptureApplyCommand, "<source-capture-Apply unavailable>"))
		}
		if item.ReviewerResultStagingCommand != "" {
			add("after source capture publishes reviewerResultSourcePath, run staging preview: " + item.ReviewerResultStagingCommand)
		}
	}
	if item.ManagedDispatch != nil {
		add("managed dispatch packet is available for replacement executor takeover; verify promptSha256 and use runtime-rebuilt WhatIf/Apply commands from this handoff for writes")
	}
	add("do not continue the lane until reviewer dispatch/intake handoff total is zero or the current packet action is resolved")
	return mission.UniqueStrings(steps)
}

func reviewerDispatchIntakeNextAction(item ReviewerDispatchIntakeHandoff) string {
	switch item.State {
	case "reviewer-packet-owner-adoption-required":
		return firstText(item.OwnerAdoptionPreviewCommand, "adopt reviewer packet "+item.PacketID+" before intake")
	case "reviewer-packet-integrity-invalid":
		return firstText(item.PacketRetirementPreviewCommand, "regenerate canonical reviewer packet at "+item.PacketPath+"; do not continue from invalid packet integrity")
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
	case "ready-for-reviewer-dispatch":
		return firstText(item.ReviewerDispatchRecordCommand, item.DispatchCommand)
	case "reviewer-session-running-unknown":
		return firstText(item.ReviewerResultInputSaveCommand, "inspect harness reviewer session "+firstText(item.ReviewerSession, item.ReviewerDispatchID)+"; save reviewer JSON input and record completion receipt when it finishes")
	case "ready-for-reviewer-completion-receipt-preview":
		return firstText(item.ReviewerCompletionRecordCommand, "record reviewer session completion -WhatIf for "+item.ShardID)
	case "reviewer-session-failed":
		return firstText(item.ReviewerDispatchRecordCommand, "dispatch a replacement reviewer session for "+item.ShardID+"; failed attempts cannot enter source capture")
	case "reviewer-session-receipt-owner-stale":
		return firstText(item.ReviewerDispatchRecordCommand, "record a replacement reviewer dispatch for current lane owner generation before source capture for "+item.ShardID)
	case "reviewer-session-receipt-invalid":
		return "repair or retire invalid reviewer session receipts for " + item.ShardID + ": " + item.ReviewerSessionReceiptFailure
	case "ready-for-reviewer-result-staging-preview":
		return firstText(item.ReviewerResultStagingCommand, "run reviewer result staging -WhatIf for "+item.ShardID)
	case "ready-for-reviewer-result-collection-preview", "reviewer-result-recovery-disposed-ready-for-collection-preview":
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
	case "reviewer-result-input-invalid":
		return "replace the invalid reviewer result input for " + item.ShardID + " at " + item.ReviewerResultInputPath + ", then rerun source capture -WhatIf"
	case "ready-for-reviewer-result-source-capture-preview":
		return firstText(item.ReviewerResultSourceCaptureCommand, "run reviewer result source capture -WhatIf for "+item.ShardID)
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

func reviewerDispatchCommand(shardID, captureCommand, captureApplyCommand, stagingCommand, inputPath, sourcePath, candidatePath, reviewerResultPath, promptPath, promptSHA256 string, request *ReviewerAgentToolRequest, idx int) string {
	promptRef := reviewerDispatchPromptArtifactRef(promptPath, promptSHA256, request, idx)
	if request != nil && strings.TrimSpace(candidatePath) != "" && strings.TrimSpace(stagingCommand) != "" {
		return "dispatch read-only reviewer for " + shardID + " using " + promptRef + "; save exactly one JSON object to " + reviewerDispatchQuoteCommandArg(inputPath) + ", run source capture preview " + reviewerDispatchQuoteCommandArg(captureCommand) + ", then run hash-gated source capture Apply " + reviewerDispatchQuoteCommandArg(captureApplyCommand) + " to publish " + reviewerDispatchQuoteCommandArg(sourcePath) + "; run staging preview " + reviewerDispatchQuoteCommandArg(stagingCommand) + ", use its expected-source-hash Apply command to publish " + reviewerDispatchQuoteCommandArg(candidatePath) + ", then run reviewer result collection WhatIf before Apply"
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

func reviewerDispatchStatusCommand(caseRoot string) string {
	caseRoot = strings.TrimSpace(caseRoot)
	if caseRoot == "" {
		return "/rekit status -Format json"
	}
	return "/rekit status -Target " + quoteCommandArg(caseRoot) + " -Format json"
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
	lines = append(lines, fmt.Sprintf("- summary: total=%d waitingForReviewerResult=%d readyForPreview=%d attachRequired=%d dispatchOnly=%d promptArtifactBlocked=%d packets=%d latestPacketProgress=%d/%d open=%d nextOpen=%s remaining=%s latestShard=%s latestState=%s inputState=%s sourceState=%s candidateState=%s nextAction=`%s`", summary.Total, summary.WaitingForReviewerResult, summary.ReadyForPreview, summary.AttachRequired, summary.DispatchOnly, summary.PromptArtifactBlocked, summary.PacketCount, summary.LatestPacketDispatchCompleted, summary.LatestPacketDispatchTotal, summary.LatestPacketDispatchOpen, summary.LatestPacketNextOpenShardID, strings.Join(summary.RemainingShardIDs, ","), summary.LatestShardID, summary.LatestState, summary.LatestReviewerResultInputState, summary.LatestReviewerResultSourceState, summary.LatestReviewerResultCandidateState, summary.NextAction))
	lines = append(lines, reviewerDispatchOperatorPackageMarkdownLines(summary.OperatorPackage)...)
	for _, item := range items {
		lines = append(lines, fmt.Sprintf("- dispatch intake: lane=%s shard=%s state=%s progress=%d/%d open=%d nextOpen=%s remaining=%s inputState=%s input=`%s` sourceState=%s source=`%s` candidateState=%s candidate=`%s` resultPresent=%t resultState=%s packet=`%s` reviewerResult=`%s` preview=`%s` apply=`%s` batchPreview=`%s` batchApply=`%s`", item.TargetLane, item.ShardID, item.State, item.DispatchCompleted, item.DispatchTotal, item.DispatchOpen, item.NextOpenShardID, strings.Join(item.RemainingShardIDs, ","), item.ReviewerResultInputState, item.ReviewerResultInputPath, item.ReviewerResultSourceState, item.ReviewerResultSourcePath, item.ReviewerResultCandidateState, item.ReviewerResultCandidatePath, item.ReviewerResultPresent, item.ReviewerResultState, item.PacketPath, item.ReviewerResultPath, item.PreviewCommand, item.ApplyCommand, item.BatchPreviewCommand, item.BatchApplyCommand))
		if strings.TrimSpace(item.DispatchPromptPath) != "" {
			lines = append(lines, fmt.Sprintf("  - prompt artifact: path=`%s` sha256=%s state=%s current=%t actualSha256=%s failure=%s", item.DispatchPromptPath, item.DispatchPromptSHA256, item.DispatchPromptState, item.DispatchPromptCurrent, item.DispatchPromptActualSHA256, item.DispatchPromptFailure))
		}
		if item.AgentToolRequest != nil {
			lines = append(lines, fmt.Sprintf("  - agent tool: tool=%s agentType=%s readOnly=%t promptPath=`%s` promptSha256=%s expectedOutput=%s", item.AgentToolRequest.Tool, item.AgentToolRequest.AgentType, item.AgentToolRequest.ReadOnly, item.AgentToolRequest.PromptPath, item.AgentToolRequest.PromptSHA256, item.AgentToolRequest.ExpectedOutput))
		}
		if item.ReviewerSessionReceiptState != "" {
			lines = append(lines, fmt.Sprintf("  - reviewer session: state=%s dispatchId=%s harness=%s session=%s outcome=%s exitStatus=%s dispatchReceipt=`%s` completionReceipt=`%s` failure=%s", item.ReviewerSessionReceiptState, item.ReviewerDispatchID, item.ReviewerHarness, item.ReviewerSession, item.ReviewerSessionOutcome, item.ReviewerSessionExitStatus, item.ReviewerDispatchReceiptPath, item.ReviewerCompletionReceiptPath, item.ReviewerSessionReceiptFailure))
		}
		if item.ManagedDispatch != nil {
			lines = append(lines, fmt.Sprintf("  - managed dispatch: mode=%s shard=%s role=%s reviewers=%d maxParallel=%d prompt=`%s` promptSha256=%s input=`%s` source=`%s` candidate=`%s` result=`%s`", item.ManagedDispatch.Mode, item.ManagedDispatch.ShardID, item.ManagedDispatch.ReviewerRole, item.ManagedDispatch.ReviewerCount, item.ManagedDispatch.MaxParallel, item.ManagedDispatch.PromptPath, item.ManagedDispatch.PromptSHA256, item.ManagedDispatch.ReviewerResultInputPath, item.ManagedDispatch.ReviewerResultSourcePath, item.ManagedDispatch.ReviewerResultCandidatePath, item.ManagedDispatch.ReviewerResultPath))
		}
		if item.ReviewerResultSourceCaptureCommand != "" {
			lines = append(lines, fmt.Sprintf("  - source capture: input=`%s` inputState=%s source=`%s` sourceState=%s preview=`%s` apply=`%s`", item.ReviewerResultInputPath, item.ReviewerResultInputState, item.ReviewerResultSourcePath, item.ReviewerResultSourceState, item.ReviewerResultSourceCaptureCommand, item.ReviewerResultSourceCaptureApplyCommand))
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
	fmt.Fprintf(out, "- summary: total=%d waitingForReviewerResult=%d readyForPreview=%d attachRequired=%d dispatchOnly=%d promptArtifactBlocked=%d packets=%d latestPacketProgress=%d/%d open=%d nextOpen=%s remaining=%s latestShard=%s latestState=%s inputState=%s sourceState=%s candidateState=%s nextAction=`%s`\n", summary.Total, summary.WaitingForReviewerResult, summary.ReadyForPreview, summary.AttachRequired, summary.DispatchOnly, summary.PromptArtifactBlocked, summary.PacketCount, summary.LatestPacketDispatchCompleted, summary.LatestPacketDispatchTotal, summary.LatestPacketDispatchOpen, summary.LatestPacketNextOpenShardID, strings.Join(summary.RemainingShardIDs, ","), summary.LatestShardID, summary.LatestState, summary.LatestReviewerResultInputState, summary.LatestReviewerResultSourceState, summary.LatestReviewerResultCandidateState, summary.NextAction)
	for _, line := range reviewerDispatchOperatorPackageMarkdownLines(summary.OperatorPackage) {
		fmt.Fprintln(out, line)
	}
	for _, item := range items {
		fmt.Fprintf(out, "- dispatch intake: lane=%s shard=%s state=%s progress=%d/%d open=%d nextOpen=%s remaining=%s inputState=%s input=`%s` sourceState=%s source=`%s` candidateState=%s candidate=`%s` resultPresent=%t packet=`%s` reviewerResult=`%s` preview=`%s` apply=`%s`\n", item.TargetLane, item.ShardID, item.State, item.DispatchCompleted, item.DispatchTotal, item.DispatchOpen, item.NextOpenShardID, strings.Join(item.RemainingShardIDs, ","), item.ReviewerResultInputState, item.ReviewerResultInputPath, item.ReviewerResultSourceState, item.ReviewerResultSourcePath, item.ReviewerResultCandidateState, item.ReviewerResultCandidatePath, item.ReviewerResultPresent, item.PacketPath, item.ReviewerResultPath, item.PreviewCommand, item.ApplyCommand)
		if strings.TrimSpace(item.DispatchPromptPath) != "" {
			fmt.Fprintf(out, "  - prompt artifact: path=`%s` sha256=%s state=%s current=%t actualSha256=%s failure=%s\n", item.DispatchPromptPath, item.DispatchPromptSHA256, item.DispatchPromptState, item.DispatchPromptCurrent, item.DispatchPromptActualSHA256, item.DispatchPromptFailure)
		}
		if item.AgentToolRequest != nil {
			fmt.Fprintf(out, "  - agent tool: tool=%s agentType=%s readOnly=%t promptPath=`%s` promptSha256=%s expectedOutput=%s\n", item.AgentToolRequest.Tool, item.AgentToolRequest.AgentType, item.AgentToolRequest.ReadOnly, item.AgentToolRequest.PromptPath, item.AgentToolRequest.PromptSHA256, item.AgentToolRequest.ExpectedOutput)
		}
		if item.ManagedDispatch != nil {
			fmt.Fprintf(out, "  - managed dispatch: mode=%s shard=%s role=%s reviewers=%d maxParallel=%d prompt=`%s` promptSha256=%s input=`%s` source=`%s` candidate=`%s` result=`%s`\n", item.ManagedDispatch.Mode, item.ManagedDispatch.ShardID, item.ManagedDispatch.ReviewerRole, item.ManagedDispatch.ReviewerCount, item.ManagedDispatch.MaxParallel, item.ManagedDispatch.PromptPath, item.ManagedDispatch.PromptSHA256, item.ManagedDispatch.ReviewerResultInputPath, item.ManagedDispatch.ReviewerResultSourcePath, item.ManagedDispatch.ReviewerResultCandidatePath, item.ManagedDispatch.ReviewerResultPath)
		}
		if item.ReviewerResultSourceCaptureCommand != "" {
			fmt.Fprintf(out, "  - source capture: input=`%s` inputState=%s source=`%s` sourceState=%s preview=`%s` apply=`%s`\n", item.ReviewerResultInputPath, item.ReviewerResultInputState, item.ReviewerResultSourcePath, item.ReviewerResultSourceState, item.ReviewerResultSourceCaptureCommand, item.ReviewerResultSourceCaptureApplyCommand)
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

func appendReviewerPacketRetirementHandoff(lines []string, items []ReviewerPacketRetirementHandoff) []string {
	lines = append(lines, "", "## Reviewer packet retirement handoff", "")
	if len(items) == 0 {
		return append(lines, "- none")
	}
	summary := ReviewerPacketRetirementSummaryFor(items)
	lines = append(lines, fmt.Sprintf("- summary: total=%d packets=%d lanes=%d latestPacket=%s latestState=%s latestLane=%s latestReceipt=`%s` nextAction=`%s`", summary.Total, summary.PacketCount, summary.LaneCount, summary.LatestPacketID, summary.LatestState, summary.LatestLane, summary.LatestReceipt, summary.NextAction))
	for _, item := range items {
		lines = append(lines, fmt.Sprintf("- packet retirement: lane=%s packet=%s state=%s receipt=`%s` packetSha256=%s integritySha256=%s noDelete=%t noHeavyTool=%t noAuthorityOrConfirmed=%t nextAction=`%s`", item.TargetLane, firstText(item.PacketID, item.PacketPath), item.State, item.RetirementPath, item.PacketSHA256, item.IntegritySHA256, item.NoDelete, item.NoHeavyTool, item.NoAuthority, item.NextAction))
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

func WriteReviewerPacketRetirementHandoffSection(out *bytes.Buffer, title string, items []ReviewerPacketRetirementHandoff) {
	if len(items) == 0 {
		return
	}
	summary := ReviewerPacketRetirementSummaryFor(items)
	fmt.Fprintln(out, title)
	fmt.Fprintln(out)
	fmt.Fprintf(out, "- summary: total=%d packets=%d lanes=%d latestPacket=%s latestState=%s latestLane=%s latestReceipt=`%s` nextAction=`%s`\n", summary.Total, summary.PacketCount, summary.LaneCount, summary.LatestPacketID, summary.LatestState, summary.LatestLane, summary.LatestReceipt, summary.NextAction)
	for _, item := range items {
		fmt.Fprintf(out, "- packet retirement: lane=%s packet=%s state=%s receipt=`%s` packetSha256=%s integritySha256=%s noDelete=%t noHeavyTool=%t noAuthorityOrConfirmed=%t nextAction=`%s`\n", item.TargetLane, firstText(item.PacketID, item.PacketPath), item.State, item.RetirementPath, item.PacketSHA256, item.IntegritySHA256, item.NoDelete, item.NoHeavyTool, item.NoAuthority, item.NextAction)
		for idx, step := range mission.LimitStrings(item.RunbookSteps, maxHandoffRows) {
			fmt.Fprintf(out, "  - runbook step %d: %s\n", idx+1, step)
		}
		for _, evidence := range mission.LimitStrings(item.Evidence, maxHandoffRows) {
			fmt.Fprintf(out, "  - evidence: %s\n", evidence)
		}
		for _, boundary := range mission.LimitStrings(item.Boundary, maxHandoffRows) {
			fmt.Fprintf(out, "  - boundary: %s\n", boundary)
		}
	}
	fmt.Fprintln(out)
}
