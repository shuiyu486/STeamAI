package subagents

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	refsf "github.com/shuiyu486/re-context-kits/internal/rekit/fs"
	"github.com/shuiyu486/re-context-kits/internal/rekit/instance"
	"github.com/shuiyu486/re-context-kits/internal/rekit/lanemutation"
	"github.com/shuiyu486/re-context-kits/internal/rekit/manifest"
	"github.com/shuiyu486/re-context-kits/internal/rekit/mission"
	"github.com/shuiyu486/re-context-kits/internal/rekit/projectstate"
	"github.com/shuiyu486/re-context-kits/internal/rekit/reviewerresult"
	"github.com/shuiyu486/re-context-kits/internal/rekit/reviewpath"
)

const commandName = "plan-subagents"

type Options struct {
	Route           string
	TaskType        string
	Items           string
	ItemsFile       string
	ItemsPerAgent   int
	MaxParallel     int
	ReviewOutputDir string
	PacketPath      string
	DiffPath        string
	Lane            string
}

type Result struct {
	SchemaVersion               int                                      `json:"schemaVersion"`
	Command                     string                                   `json:"command"`
	PlanRoot                    string                                   `json:"planRoot"`
	RepoRoot                    string                                   `json:"repoRoot"`
	Pack                        string                                   `json:"pack"`
	IsMutation                  bool                                     `json:"isMutation"`
	WritesReviewArtifacts       bool                                     `json:"writesReviewArtifacts"`
	ReviewRequired              bool                                     `json:"reviewRequired"`
	ReviewRoot                  string                                   `json:"reviewRoot"`
	PacketPath                  string                                   `json:"packetPath"`
	SummaryPath                 string                                   `json:"summaryPath"`
	CombinedDiffPath            string                                   `json:"combinedDiffPath"`
	ItemCount                   int                                      `json:"itemCount"`
	ShardCount                  int                                      `json:"shardCount"`
	TargetLane                  string                                   `json:"targetLane"`
	OwnerBinding                OwnerBinding                             `json:"ownerBinding"`
	ReviewerOrchestration       ReviewerOrchestrationPlan                `json:"reviewerOrchestration"`
	ShardHandoffs               []ShardHandoff                           `json:"shardHandoffs"`
	Observability               Observability                            `json:"observability"`
	ReviewLoop                  ReviewLoop                               `json:"reviewLoop"`
	MissionCommanderAction      mission.MissionCommanderAction           `json:"missionCommanderAction"`
	MissionCommanderNextActions []mission.MissionCommanderNextActionItem `json:"missionCommanderNextActions,omitempty"`
	MissionCommanderActionQueue mission.MissionCommanderActionQueue      `json:"missionCommanderActionQueue"`
}

type ReviewerPacketIntegrityReference struct {
	Algorithm string `json:"algorithm"`
	Path      string `json:"path"`
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

type Packet struct {
	SchemaVersion             int                               `json:"schemaVersion"`
	PacketID                  string                            `json:"packetId"`
	PacketIntegrity           *ReviewerPacketIntegrityReference `json:"packetIntegrity,omitempty"`
	Command                   string                            `json:"command"`
	IsMutation                bool                              `json:"isMutation"`
	WritesReviewArtifacts     bool                              `json:"writesReviewArtifacts"`
	PlanRoot                  string                            `json:"planRoot"`
	RepoRoot                  string                            `json:"repoRoot"`
	Pack                      string                            `json:"pack"`
	ManifestPath              string                            `json:"manifestPath"`
	TargetLane                string                            `json:"targetLane"`
	OwnerBinding              OwnerBinding                      `json:"ownerBinding"`
	Route                     Route                             `json:"route"`
	Input                     Input                             `json:"input"`
	ShardPolicy               ShardPolicy                       `json:"shardPolicy"`
	Shards                    []Shard                           `json:"shards"`
	ShardHandoffs             []ShardHandoff                    `json:"shardHandoffs"`
	ReviewerOrchestration     ReviewerOrchestrationPlan         `json:"reviewerOrchestration"`
	MainAgentResponsibilities string                            `json:"mainAgentResponsibilities"`
	SubagentPermissions       string                            `json:"subagentPermissions"`
	OutputContract            string                            `json:"outputContract"`
	ReviewRequired            bool                              `json:"reviewRequired"`
	Observability             Observability                     `json:"observability"`
	ReviewLoop                ReviewLoop                        `json:"reviewLoop"`
}

type OwnerBinding struct {
	TargetLane             string `json:"targetLane"`
	CurrentExecutor        string `json:"currentExecutor,omitempty"`
	ExecutorGeneration     int    `json:"executorGeneration,omitempty"`
	LastTakeoverAt         string `json:"lastTakeoverAt,omitempty"`
	LastTakeoverBy         string `json:"lastTakeoverBy,omitempty"`
	LastTakeoverReason     string `json:"lastTakeoverReason,omitempty"`
	BindingMode            string `json:"bindingMode"`
	RequiredForIntake      bool   `json:"requiredForIntake"`
	MainAgentSpawnOwner    string `json:"mainAgentSpawnOwner"`
	RuntimeSessionBoundary string `json:"runtimeSessionBoundary"`
}

type Observability struct {
	DispatchMode     string        `json:"dispatchMode"`
	RouteDebug       RouteDebug    `json:"routeDebug"`
	ReviewRoot       string        `json:"reviewRoot"`
	ResultRoot       string        `json:"resultRoot"`
	PromptRoot       string        `json:"promptRoot,omitempty"`
	PacketPath       string        `json:"packetPath"`
	SummaryPath      string        `json:"summaryPath"`
	CombinedDiffPath string        `json:"combinedDiffPath"`
	ShardStatuses    []ShardStatus `json:"shardStatuses"`
	BlockedActions   []string      `json:"blockedActions"`
}

type RouteDebug struct {
	SelectedBy    string `json:"selectedBy"`
	RouteID       string `json:"routeId"`
	TaskTypes     string `json:"taskTypes"`
	Trigger       string `json:"trigger"`
	Reference     string `json:"reference"`
	PolicyOverlay string `json:"policyOverlay"`
}

type ShardStatus struct {
	ShardID        string `json:"shardId"`
	Status         string `json:"status"`
	ItemCount      int    `json:"itemCount"`
	ExpectedOutput string `json:"expectedOutput"`
}

type ReviewLoop struct {
	SpawnOwner         string   `json:"spawnOwner"`
	MergeOwner         string   `json:"mergeOwner"`
	MainAgentOwns      []string `json:"mainAgentOwns"`
	VerdictWriteback   string   `json:"verdictWriteback"`
	CompletionCriteria []string `json:"completionCriteria"`
	FailureHandling    string   `json:"failureHandling"`
}

type ReviewerOrchestrationPlan struct {
	Mode                        string                                   `json:"mode"`
	Scope                       string                                   `json:"scope"`
	TargetLane                  string                                   `json:"targetLane"`
	OwnerBinding                OwnerBinding                             `json:"ownerBinding"`
	PacketPath                  string                                   `json:"packetPath"`
	ResultRoot                  string                                   `json:"resultRoot"`
	ReviewerCount               int                                      `json:"reviewerCount"`
	MaxParallel                 int                                      `json:"maxParallel"`
	Summary                     ReviewerOrchestrationSummary             `json:"summary"`
	ManagedDispatchPacket       *ReviewerManagedDispatchPacket           `json:"managedDispatchPacket,omitempty"`
	Dispatches                  []ReviewerDispatch                       `json:"dispatches"`
	Lifecycle                   []ReviewerOrchestrationStep              `json:"lifecycle"`
	RuntimeBoundary             []string                                 `json:"runtimeBoundary"`
	CompletionCriteria          []string                                 `json:"completionCriteria"`
	BatchPreviewCommand         string                                   `json:"batchPreviewCommand,omitempty"`
	BatchApplyCommand           string                                   `json:"batchApplyCommand,omitempty"`
	MissionCommanderAction      *mission.MissionCommanderAction          `json:"missionCommanderAction,omitempty"`
	MissionCommanderNextActions []mission.MissionCommanderNextActionItem `json:"missionCommanderNextActions,omitempty"`
	MissionCommanderActionQueue *mission.MissionCommanderActionQueue     `json:"missionCommanderActionQueue,omitempty"`
}

type ReviewerOrchestrationSummary struct {
	Mode                   string                                   `json:"mode"`
	Scope                  string                                   `json:"scope,omitempty"`
	TargetLane             string                                   `json:"targetLane,omitempty"`
	ReviewerCount          int                                      `json:"reviewerCount"`
	MaxParallel            int                                      `json:"maxParallel"`
	PacketPath             string                                   `json:"packetPath,omitempty"`
	ResultRoot             string                                   `json:"resultRoot,omitempty"`
	BatchPreviewCommand    string                                   `json:"batchPreviewCommand,omitempty"`
	BatchApplyCommand      string                                   `json:"batchApplyCommand,omitempty"`
	OwnerBinding           ReviewerOrchestrationOwnerSummary        `json:"ownerBinding"`
	ManagedDispatchSummary *ReviewerManagedDispatchSummary          `json:"managedDispatchSummary,omitempty"`
	DispatchCount          int                                      `json:"dispatchCount"`
	IntakeAvailable        bool                                     `json:"intakeAvailable"`
	CollectionAvailable    bool                                     `json:"collectionAvailable"`
	DispatchOnly           bool                                     `json:"dispatchOnly"`
	ActionTotal            int                                      `json:"actionTotal"`
	ActionUnblocked        int                                      `json:"actionUnblocked"`
	ActionBlocked          int                                      `json:"actionBlocked"`
	ActionRequiresReview   int                                      `json:"actionRequiresReview"`
	ActionFollowUp         int                                      `json:"actionFollowUp"`
	QueueSummary           string                                   `json:"queueSummary,omitempty"`
	FirstDispatch          *ReviewerOrchestrationDispatchSummary    `json:"firstDispatch,omitempty"`
	Dispatches             []ReviewerOrchestrationDispatchSummary   `json:"dispatches,omitempty"`
	CurrentAction          *ReviewerOrchestrationNextActionSummary  `json:"currentAction,omitempty"`
	NextActions            []ReviewerOrchestrationNextActionSummary `json:"nextActions,omitempty"`
	Boundary               []string                                 `json:"boundary,omitempty"`
}

type ReviewerOrchestrationOwnerSummary struct {
	TargetLane         string `json:"targetLane,omitempty"`
	BindingMode        string `json:"bindingMode,omitempty"`
	CurrentExecutor    string `json:"currentExecutor,omitempty"`
	ExecutorGeneration int    `json:"executorGeneration"`
	RequiredForIntake  bool   `json:"requiredForIntake"`
	SpawnOwner         string `json:"spawnOwner,omitempty"`
}

type ReviewerOrchestrationDispatchSummary struct {
	ShardID            string `json:"shardId"`
	Status             string `json:"status"`
	ReviewerResultPath string `json:"reviewerResultPath,omitempty"`
	PromptPath         string `json:"promptPath,omitempty"`
	PromptSHA256       string `json:"promptSha256,omitempty"`
	DispatchCommand    string `json:"dispatchCommand,omitempty"`
	PreviewCommand     string `json:"previewCommand,omitempty"`
	ApplyCommand       string `json:"applyCommand,omitempty"`
}

type ReviewerOrchestrationNextActionSummary struct {
	State          string `json:"state"`
	Source         string `json:"source"`
	Command        string `json:"command"`
	Blocked        bool   `json:"blocked,omitempty"`
	RequiresReview bool   `json:"requiresReview,omitempty"`
}

type ReviewerManagedDispatchPacket struct {
	Mode                string                    `json:"mode"`
	Scope               string                    `json:"scope"`
	TargetLane          string                    `json:"targetLane"`
	OwnerBinding        OwnerBinding              `json:"ownerBinding"`
	PacketPath          string                    `json:"packetPath"`
	PromptRoot          string                    `json:"promptRoot,omitempty"`
	ResultRoot          string                    `json:"resultRoot"`
	ReviewerCount       int                       `json:"reviewerCount"`
	MaxParallel         int                       `json:"maxParallel"`
	Dispatches          []ReviewerManagedDispatch `json:"dispatches"`
	BatchPreviewCommand string                    `json:"batchPreviewCommand,omitempty"`
	BatchApplyCommand   string                    `json:"batchApplyCommand,omitempty"`
	Runbook             []string                  `json:"runbook"`
	Boundary            []string                  `json:"boundary"`
	CompletionCriteria  []string                  `json:"completionCriteria"`
}

type ReviewerManagedDispatch struct {
	ShardID                     string                          `json:"shardId"`
	ReviewerRole                string                          `json:"reviewerRole"`
	Status                      string                          `json:"status"`
	Items                       []string                        `json:"items"`
	PromptPath                  string                          `json:"promptPath,omitempty"`
	PromptSHA256                string                          `json:"promptSha256,omitempty"`
	AgentToolRequest            ReviewerManagedAgentToolRequest `json:"agentToolRequest"`
	ReviewerResultPath          string                          `json:"reviewerResultPath"`
	ReviewerResultCandidatePath string                          `json:"reviewerResultCandidatePath,omitempty"`
	ReviewerResultInputPath     string                          `json:"reviewerResultInputPath,omitempty"`
	ReviewerResultSourcePath    string                          `json:"reviewerResultSourcePath,omitempty"`
	InputSavePreviewCommand     string                          `json:"inputSavePreviewCommand,omitempty"`
	InputSaveApplyCommand       string                          `json:"inputSaveApplyCommand,omitempty"`
	SourceCapturePreviewCommand string                          `json:"sourceCapturePreviewCommand,omitempty"`
	SourceCaptureApplyCommand   string                          `json:"sourceCaptureApplyCommand,omitempty"`
	StagingPreviewCommand       string                          `json:"stagingPreviewCommand,omitempty"`
	CollectionPreviewCommand    string                          `json:"collectionPreviewCommand,omitempty"`
	CollectionApplyCommand      string                          `json:"collectionApplyCommand,omitempty"`
	IntakePreviewCommand        string                          `json:"intakePreviewCommand"`
	IntakeApplyCommand          string                          `json:"intakeApplyCommand"`
	DispatchCommand             string                          `json:"dispatchCommand"`
	ReviewerResultSkeleton      string                          `json:"reviewerResultSkeleton"`
	ExpectedOutput              string                          `json:"expectedOutput"`
	NextAction                  string                          `json:"nextAction"`
	Boundary                    []string                        `json:"boundary"`
}

type ReviewerManagedAgentToolRequest struct {
	Tool           string `json:"tool"`
	AgentType      string `json:"agentType"`
	ReadOnly       bool   `json:"readOnly"`
	PromptPath     string `json:"promptPath,omitempty"`
	PromptSHA256   string `json:"promptSha256,omitempty"`
	ExpectedOutput string `json:"expectedOutput"`
}

type ReviewerManagedDispatchSummary struct {
	Mode                string                               `json:"mode"`
	TargetLane          string                               `json:"targetLane"`
	PacketPath          string                               `json:"packetPath"`
	DispatchCount       int                                  `json:"dispatchCount"`
	ReviewerCount       int                                  `json:"reviewerCount"`
	MaxParallel         int                                  `json:"maxParallel"`
	BatchPreviewCommand string                               `json:"batchPreviewCommand,omitempty"`
	BatchApplyCommand   string                               `json:"batchApplyCommand,omitempty"`
	FirstDispatch       *ReviewerManagedDispatchItemSummary  `json:"firstDispatch,omitempty"`
	Dispatches          []ReviewerManagedDispatchItemSummary `json:"dispatches,omitempty"`
	Boundary            []string                             `json:"boundary,omitempty"`
}

type ReviewerManagedDispatchItemSummary struct {
	ShardID                string `json:"shardId"`
	Status                 string `json:"status"`
	PromptPath             string `json:"promptPath,omitempty"`
	PromptSHA256           string `json:"promptSha256,omitempty"`
	ReviewerResultPath     string `json:"reviewerResultPath"`
	SourceCaptureAvailable bool   `json:"sourceCaptureAvailable"`
	CollectionAvailable    bool   `json:"collectionAvailable"`
	IntakeAvailable        bool   `json:"intakeAvailable"`
	NextAction             string `json:"nextAction"`
}

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

type ReviewerDispatch struct {
	ShardID                     string                            `json:"shardId"`
	ReviewerRole                string                            `json:"reviewerRole"`
	Status                      string                            `json:"status"`
	Items                       []string                          `json:"items"`
	DispatchPrompt              string                            `json:"dispatchPrompt"`
	DispatchPromptPath          string                            `json:"dispatchPromptPath,omitempty"`
	DispatchPromptSHA256        string                            `json:"dispatchPromptSha256,omitempty"`
	AgentToolRequest            *ReviewerAgentToolRequest         `json:"agentToolRequest,omitempty"`
	ReviewerResultPath          string                            `json:"reviewerResultPath"`
	ReviewerResultCandidatePath string                            `json:"reviewerResultCandidatePath,omitempty"`
	StagingCommands             *ReviewerResultStagingCommands    `json:"stagingCommands,omitempty"`
	CollectionCommands          *ReviewerResultCollectionCommands `json:"collectionCommands,omitempty"`
	PreviewCommand              string                            `json:"previewCommand"`
	ApplyCommand                string                            `json:"applyCommand"`
}

type ReviewerOrchestrationStep struct {
	Step          string   `json:"step"`
	Owner         string   `json:"owner"`
	Action        string   `json:"action"`
	Inputs        []string `json:"inputs"`
	MustPass      []string `json:"mustPass"`
	NextOnSuccess string   `json:"nextOnSuccess"`
	NextOnFailure string   `json:"nextOnFailure"`
}

type Route struct {
	ID                  string `json:"id"`
	TaskTypes           string `json:"taskTypes"`
	Trigger             string `json:"trigger"`
	ShardBasis          string `json:"shardBasis"`
	TargetItemsPerAgent string `json:"targetItemsPerAgent"`
	MaxParallel         string `json:"maxParallel"`
	Reference           string `json:"reference"`
	PolicyOverlay       string `json:"policyOverlay"`
	SubagentPermissions string `json:"subagentPermissions"`
	MainAgentOwns       string `json:"mainAgentOwns"`
	OutputContract      string `json:"outputContract"`
}

type Input struct {
	TaskType  string `json:"taskType"`
	ItemCount int    `json:"itemCount"`
	ItemsFile string `json:"itemsFile"`
}

type ShardPolicy struct {
	Basis               string `json:"basis"`
	TargetItemsPerAgent int    `json:"targetItemsPerAgent"`
	MaxParallel         int    `json:"maxParallel"`
}

type Shard struct {
	ID     string   `json:"id"`
	Items  []string `json:"items"`
	Prompt string   `json:"prompt"`
}

type ShardHandoff struct {
	ShardID                     string                            `json:"shardId"`
	Status                      string                            `json:"status"`
	ReviewerResultPath          string                            `json:"reviewerResultPath"`
	ReviewerResultCandidatePath string                            `json:"reviewerResultCandidatePath,omitempty"`
	OwnerBinding                OwnerBinding                      `json:"ownerBinding"`
	DispatchPrompt              string                            `json:"dispatchPrompt"`
	DispatchPromptPath          string                            `json:"dispatchPromptPath,omitempty"`
	DispatchPromptSHA256        string                            `json:"dispatchPromptSha256,omitempty"`
	AgentToolRequest            *ReviewerAgentToolRequest         `json:"agentToolRequest,omitempty"`
	ReviewerStagingCommands     *ReviewerResultStagingCommands    `json:"reviewerStagingCommands,omitempty"`
	ReviewerCollectionCommands  *ReviewerResultCollectionCommands `json:"reviewerCollectionCommands,omitempty"`
	Items                       []string                          `json:"items"`
	ReadOnlyBoundary            []string                          `json:"readOnlyBoundary"`
	ExpectedOutput              string                            `json:"expectedOutput"`
	ReviewerWriteback           string                            `json:"reviewerWriteback"`
	ReviewerResultContract      ReviewerResultContract            `json:"reviewerResultContract"`
	ReviewerIntakeCommands      ReviewerIntakeCommands            `json:"reviewerIntakeCommands"`
	MainAgentNextAction         string                            `json:"mainAgentNextAction"`
	IntakeChecklist             []string                          `json:"intakeChecklist"`
	ReviewerDecisionMappings    []ReviewerDecisionMapping         `json:"reviewerDecisionMappings"`
	ConflictHandling            []string                          `json:"conflictHandling"`
	WritebackSequence           []WritebackSequenceStep           `json:"writebackSequence"`
	PostReviewMerge             []string                          `json:"postReviewMerge"`
	CompletionCriteria          []string                          `json:"completionCriteria"`
	FailureHandling             string                            `json:"failureHandling"`
}

type ReviewerResultContract struct {
	OutputFormat     string   `json:"outputFormat"`
	RequiredFields   []string `json:"requiredFields"`
	AllowedDecisions []string `json:"allowedDecisions"`
	EvidenceRules    []string `json:"evidenceRules"`
	ConflictSignals  []string `json:"conflictSignals"`
}

type ReviewerDecisionMapping struct {
	ReviewerDecision    string   `json:"reviewerDecision"`
	VerificationVerdict string   `json:"verificationVerdict"`
	MainDecision        string   `json:"mainDecision"`
	ApplyWhen           []string `json:"applyWhen"`
	Fallback            string   `json:"fallback"`
}

type WritebackSequenceStep struct {
	Step            string                    `json:"step"`
	Owner           string                    `json:"owner"`
	Uses            []string                  `json:"uses"`
	CommandBindings []WritebackCommandBinding `json:"commandBindings,omitempty"`
	MustPass        []string                  `json:"mustPass"`
	BlockedBy       []string                  `json:"blockedBy,omitempty"`
	NextOnSuccess   string                    `json:"nextOnSuccess"`
	NextOnFailure   string                    `json:"nextOnFailure"`
}

type WritebackCommandBinding struct {
	Binding        string   `json:"binding"`
	Source         string   `json:"source"`
	Kind           string   `json:"kind,omitempty"`
	Command        string   `json:"command,omitempty"`
	RequiredFields []string `json:"requiredFields,omitempty"`
	ExpectedOutput string   `json:"expectedOutput"`
}

type ReviewerIntakeCommands struct {
	Purpose        string                         `json:"purpose"`
	PreviewCommand string                         `json:"previewCommand"`
	ApplyCommand   string                         `json:"applyCommand"`
	RequiredFields []string                       `json:"requiredFields"`
	PreviewChecks  []string                       `json:"previewChecks,omitempty"`
	BlockedOutputs []string                       `json:"blockedOutputs,omitempty"`
	RepairGuidance []ReviewerIntakeRepairGuidance `json:"repairGuidance,omitempty"`
}

type artifactPaths struct {
	Root             string
	DiffRoot         string
	PreviewRoot      string
	ResultRoot       string
	PromptRoot       string
	PacketPath       string
	SummaryPath      string
	CombinedDiffPath string
}

func WritePlan(repoRoot, target, pack string, opt Options) (Result, error) {
	planRoot, err := filepath.Abs(target)
	if err != nil {
		return Result{}, err
	}
	if _, err := projectstate.Resolve(planRoot); err != nil {
		return Result{}, err
	}
	caseTarget := instance.LooksLikeCase(planRoot)
	if caseTarget {
		if _, err := instance.AssertAttached(planRoot, repoRoot, pack); err != nil {
			return Result{}, err
		}
	} else {
		if strings.TrimSpace(opt.ReviewOutputDir) == "" {
			return Result{}, fmt.Errorf("plan-subagents target must be an attached rekit case unless -ReviewOutputDir is provided for an explicit out-of-case review artifact path")
		}
		st, err := os.Stat(planRoot)
		if err != nil {
			if os.IsNotExist(err) {
				return Result{}, fmt.Errorf("plan-subagents target directory does not exist: %s", planRoot)
			}
			return Result{}, err
		}
		if !st.IsDir() {
			return Result{}, fmt.Errorf("plan-subagents target is not a directory: %s", planRoot)
		}
	}
	m, err := manifest.Load(repoRoot, pack)
	if err != nil {
		return Result{}, err
	}
	route, err := selectRoute(m, opt.Route, opt.TaskType)
	if err != nil {
		return Result{}, err
	}
	items, itemsFile, err := splitItems(opt.Items, opt.ItemsFile)
	if err != nil {
		return Result{}, err
	}
	itemsPerAgent := optionInt(route.TargetItemsPerAgent, 4)
	if opt.ItemsPerAgent > 0 {
		itemsPerAgent = opt.ItemsPerAgent
	}
	maxParallel := optionInt(route.MaxParallel, 3)
	if opt.MaxParallel > 0 {
		maxParallel = opt.MaxParallel
	}
	shards := newShards(items, itemsPerAgent)
	var planningLease *lanemutation.Lease
	boardExists := false
	if caseTarget {
		boardPath, err := projectstate.Join(planRoot, "board.json")
		if err != nil {
			return Result{}, err
		}
		boardExists = refsf.Exists(boardPath)
	}
	if boardExists {
		ownerBinding, err := resolveOwnerBinding(planRoot, m, opt, true)
		if err != nil {
			return Result{}, err
		}
		planningLease, err = lanemutation.AcquireOpenLane(planRoot, ownerBinding.TargetLane, commandName)
		if err != nil {
			return Result{}, err
		}
		defer func() {
			if planningLease != nil {
				_ = planningLease.Unlock()
			}
		}()
		if err := planningLease.Validate(); err != nil {
			unlockErr := planningLease.Unlock()
			planningLease = nil
			return Result{}, errors.Join(err, unlockErr)
		}
	}
	paths, err := makeArtifactPaths(planRoot, opt)
	if err != nil {
		return Result{}, err
	}
	collectionAvailable := false
	if namespace, ok := reviewpath.CanonicalCollectionNamespace(planRoot, paths.PacketPath); caseTarget && ok {
		if !reviewpath.CollectionNamespacePathSafe(planRoot, paths.Root, true) {
			return Result{}, fmt.Errorf("canonical reviewer artifact paths must not traverse symlinks or escape the attached case")
		}
		collectionAvailable = samePath(paths.Root, namespace.ReviewRoot) &&
			samePath(paths.ResultRoot, namespace.ResultRoot)
	}
	if err := prepareArtifactDirs(paths); err != nil {
		return Result{}, err
	}
	observability := newObservability(route, opt, paths, shards)
	reviewLoop := newReviewLoop(route)
	ownerBinding, err := resolveOwnerBinding(planRoot, m, opt, caseTarget)
	if err != nil {
		return Result{}, err
	}
	targetLane := ownerBinding.TargetLane
	shardHandoffs := newShardHandoffs(shards, route, observability, reviewLoop, planRoot, m.Pack, ownerBinding, caseTarget, collectionAvailable)
	if err := writeReviewerPromptArtifacts(paths.PromptRoot, shardHandoffs); err != nil {
		return Result{}, err
	}
	orchestration := newReviewerOrchestration(planRoot, m.Pack, route, shardHandoffs, observability, reviewLoop, ownerBinding, maxParallel, caseTarget, collectionAvailable)
	commanderAction := reviewerPlanMissionCommanderAction(planRoot, m.Pack, orchestration, caseTarget)
	commanderNextActions := reviewerPlanMissionCommanderNextActions(planRoot, m.Pack, orchestration, commanderAction, caseTarget)
	commanderActionQueue := mission.MissionCommanderActionQueueFor(commanderNextActions)
	orchestration.MissionCommanderAction = &commanderAction
	orchestration.MissionCommanderNextActions = commanderNextActions
	orchestration.MissionCommanderActionQueue = &commanderActionQueue
	orchestration.Summary = reviewerOrchestrationSummary(orchestration)
	packet := Packet{
		SchemaVersion:             1,
		Command:                   commandName,
		IsMutation:                false,
		WritesReviewArtifacts:     true,
		PlanRoot:                  planRoot,
		RepoRoot:                  m.RepoRoot,
		Pack:                      m.Pack,
		ManifestPath:              m.ManifestPath,
		TargetLane:                targetLane,
		OwnerBinding:              ownerBinding,
		Route:                     route,
		Input:                     Input{TaskType: opt.TaskType, ItemCount: len(items), ItemsFile: itemsFile},
		ShardPolicy:               ShardPolicy{Basis: route.ShardBasis, TargetItemsPerAgent: itemsPerAgent, MaxParallel: maxParallel},
		Shards:                    shards,
		ShardHandoffs:             shardHandoffs,
		ReviewerOrchestration:     orchestration,
		MainAgentResponsibilities: route.MainAgentOwns,
		SubagentPermissions:       route.SubagentPermissions,
		OutputContract:            route.OutputContract,
		ReviewRequired:            true,
		Observability:             observability,
		ReviewLoop:                reviewLoop,
	}
	if collectionAvailable {
		packet.PacketIntegrity = &ReviewerPacketIntegrityReference{
			Algorithm: "sha256",
			Path:      filepath.Join(paths.Root, "packet.integrity.json"),
		}
	}
	packet.PacketID = packetIdentity(packet)
	packetData, err := json.MarshalIndent(packet, "", "  ")
	if err != nil {
		return Result{}, err
	}
	packetData = append(packetData, '\n')
	if err := os.WriteFile(paths.PacketPath, packetData, 0o644); err != nil {
		return Result{}, err
	}
	if packet.PacketIntegrity != nil {
		integrity := reviewerPacketIntegrity{
			SchemaVersion: 1,
			Kind:          "reviewer-packet-integrity",
			Algorithm:     packet.PacketIntegrity.Algorithm,
			PacketID:      packet.PacketID,
			TargetLane:    packet.TargetLane,
			PacketPath:    paths.PacketPath,
			PacketSHA256:  sha256Hex(packetData),
			PacketBytes:   len(packetData),
		}
		if err := writeJSON(packet.PacketIntegrity.Path, integrity); err != nil {
			_ = os.Remove(paths.PacketPath)
			return Result{}, err
		}
	}
	if err := os.WriteFile(paths.SummaryPath, []byte(summaryText(packet.PacketID, route, opt.TaskType, len(items), len(shards), itemsPerAgent, maxParallel, observability, reviewLoop, ownerBinding, shardHandoffs, orchestration)), 0o644); err != nil {
		return Result{}, err
	}
	return Result{SchemaVersion: 1, Command: commandName, PlanRoot: planRoot, RepoRoot: m.RepoRoot, Pack: m.Pack, IsMutation: false, WritesReviewArtifacts: true, ReviewRequired: true, ReviewRoot: paths.Root, PacketPath: paths.PacketPath, SummaryPath: paths.SummaryPath, CombinedDiffPath: paths.CombinedDiffPath, ItemCount: len(items), ShardCount: len(shards), TargetLane: targetLane, OwnerBinding: ownerBinding, ReviewerOrchestration: orchestration, ShardHandoffs: shardHandoffs, Observability: observability, ReviewLoop: reviewLoop, MissionCommanderAction: commanderAction, MissionCommanderNextActions: commanderNextActions, MissionCommanderActionQueue: commanderActionQueue}, nil
}

func resolveOwnerBinding(planRoot string, m *manifest.Manifest, opt Options, intakeAvailable bool) (OwnerBinding, error) {
	targetLane := strings.TrimSpace(opt.Lane)
	if targetLane == "" {
		targetLane = strings.TrimSpace(m.WorkstreamDefaults["defaultAuthorityLane"])
	}
	if targetLane == "" {
		return OwnerBinding{}, fmt.Errorf("plan-subagents requires a target lane from -Lane or manifest defaultAuthorityLane")
	}
	binding := OwnerBinding{
		TargetLane:             targetLane,
		BindingMode:            "out-of-case-dispatch-only",
		RequiredForIntake:      false,
		MainAgentSpawnOwner:    "main-agent",
		RuntimeSessionBoundary: "runtime only records reviewer owner provenance; it does not spawn, stop, monitor, or manage reviewer/member sessions",
	}
	if !intakeAvailable {
		return binding, nil
	}
	board, err := mission.ReadBoard(planRoot)
	if err != nil {
		if os.IsNotExist(err) {
			binding.BindingMode = "attached-case-board-missing"
			return binding, nil
		}
		return OwnerBinding{}, fmt.Errorf("read board for reviewer owner binding: %w", err)
	}
	lane, ok := mission.LookupBoardLane(board.Lanes, targetLane, false)
	if !ok {
		boardRel, relErr := projectstate.Rel(planRoot, "board.json")
		if relErr != nil {
			return OwnerBinding{}, relErr
		}
		return OwnerBinding{}, fmt.Errorf("reviewer owner binding target lane %q is not present in %s; known: %s", targetLane, boardRel, strings.Join(mission.BoardLaneIDs(board.Lanes), ","))
	}
	binding.CurrentExecutor = strings.TrimSpace(lane.CurrentExecutor)
	binding.ExecutorGeneration = lane.ExecutorGeneration
	binding.LastTakeoverAt = lane.LastTakeoverAt
	binding.LastTakeoverBy = lane.LastTakeoverBy
	binding.LastTakeoverReason = lane.LastTakeoverReason
	if binding.CurrentExecutor != "" && binding.ExecutorGeneration > 0 {
		binding.BindingMode = "current-executor-generation"
		binding.RequiredForIntake = true
	} else {
		binding.BindingMode = "unassigned-lane"
		binding.RequiredForIntake = false
	}
	return binding, nil
}

func selectRoute(m *manifest.Manifest, routeID, taskType string) (Route, error) {
	if strings.TrimSpace(routeID) != "" {
		for _, route := range m.SubagentRoutes {
			if strings.EqualFold(route.ID, routeID) {
				return toRoute(route), nil
			}
		}
		return Route{}, fmt.Errorf("subagent route not found: %s", routeID)
	}
	route, err := m.SubagentRouteForTaskType(taskType)
	if err != nil {
		return Route{}, err
	}
	return toRoute(route), nil
}

func toRoute(route manifest.SubagentRoute) Route {
	return Route{ID: route.ID, TaskTypes: route.TaskTypes, Trigger: route.Trigger, ShardBasis: route.ShardBasis, TargetItemsPerAgent: route.TargetItemsPerAgent, MaxParallel: route.MaxParallel, Reference: route.Reference, PolicyOverlay: route.PolicyOverlay, SubagentPermissions: route.SubagentPermissions, MainAgentOwns: route.MainAgentOwns, OutputContract: route.OutputContract}
}

func splitItems(items, itemsFile string) ([]string, string, error) {
	text := items
	originalFile := strings.TrimSpace(itemsFile)
	if originalFile != "" {
		abs, err := filepath.Abs(originalFile)
		if err != nil {
			return nil, "", err
		}
		b, err := os.ReadFile(abs)
		if os.IsNotExist(err) {
			return nil, "", fmt.Errorf("missing plan items file: %s", abs)
		}
		if err != nil {
			return nil, "", err
		}
		text = string(b)
	}
	if strings.TrimSpace(text) == "" {
		return []string{}, originalFile, nil
	}
	parts := strings.FieldsFunc(text, func(r rune) bool {
		return r == ',' || r == ';' || r == '\r' || r == '\n' || r == '\t' || r == ' '
	})
	out := []string{}
	for _, part := range parts {
		part = strings.TrimSpace(part)
		if part != "" {
			out = append(out, part)
		}
	}
	return out, originalFile, nil
}

func packetIdentity(packet Packet) string {
	packet.PacketID = ""
	packet.PlanRoot = filepath.Clean(packet.PlanRoot)
	packet.RepoRoot = filepath.Clean(packet.RepoRoot)
	packet.Pack = strings.TrimSpace(packet.Pack)
	encoded, err := json.Marshal(packet)
	if err != nil {
		panic(err)
	}
	sum := sha256.Sum256(encoded)
	return "packet-" + hex.EncodeToString(sum[:])[:16]
}

func packetIdentityMatches(packet Packet) bool {
	id := strings.TrimSpace(packet.PacketID)
	if id == "" {
		return false
	}
	if id == packetIdentity(packet) {
		return true
	}
	return reviewerOrchestrationEmpty(packet.ReviewerOrchestration) && id == legacyPacketIdentity(packet)
}

func reviewerOrchestrationEmpty(plan ReviewerOrchestrationPlan) bool {
	return plan.Mode == "" && plan.Scope == "" && plan.TargetLane == "" && plan.OwnerBinding == (OwnerBinding{}) && plan.PacketPath == "" && plan.ResultRoot == "" && plan.ReviewerCount == 0 && plan.MaxParallel == 0 && reviewerOrchestrationSummaryEmpty(plan.Summary) && plan.ManagedDispatchPacket == nil && len(plan.Dispatches) == 0 && len(plan.Lifecycle) == 0 && len(plan.RuntimeBoundary) == 0 && len(plan.CompletionCriteria) == 0 && plan.MissionCommanderAction == nil && len(plan.MissionCommanderNextActions) == 0 && reviewerActionQueueEmpty(plan.MissionCommanderActionQueue)
}

func reviewerOrchestrationSummaryEmpty(summary ReviewerOrchestrationSummary) bool {
	return summary.Mode == "" && summary.Scope == "" && summary.TargetLane == "" && summary.ReviewerCount == 0 && summary.MaxParallel == 0 && summary.PacketPath == "" && summary.ResultRoot == "" && summary.OwnerBinding == (ReviewerOrchestrationOwnerSummary{}) && summary.ManagedDispatchSummary == nil && summary.DispatchCount == 0 && !summary.IntakeAvailable && !summary.DispatchOnly && summary.ActionTotal == 0 && summary.ActionUnblocked == 0 && summary.ActionBlocked == 0 && summary.ActionRequiresReview == 0 && summary.ActionFollowUp == 0 && summary.QueueSummary == "" && summary.FirstDispatch == nil && len(summary.Dispatches) == 0 && summary.CurrentAction == nil && len(summary.NextActions) == 0 && len(summary.Boundary) == 0
}

func reviewerActionQueueEmpty(queue *mission.MissionCommanderActionQueue) bool {
	return queue == nil || (queue.Summary == "" && queue.CurrentAction == nil && queue.Counts == (mission.MissionCommanderActionQueueCounts{}) && len(queue.UnblockedActions) == 0 && len(queue.BlockedActions) == 0 && len(queue.ReviewRequiredActions) == 0 && len(queue.FollowUpActions) == 0)
}

func legacyPacketIdentity(packet Packet) string {
	packet.PacketID = ""
	packet.PlanRoot = filepath.Clean(packet.PlanRoot)
	packet.RepoRoot = filepath.Clean(packet.RepoRoot)
	packet.Pack = strings.TrimSpace(packet.Pack)
	legacy := struct {
		SchemaVersion             int            `json:"schemaVersion"`
		PacketID                  string         `json:"packetId"`
		Command                   string         `json:"command"`
		IsMutation                bool           `json:"isMutation"`
		WritesReviewArtifacts     bool           `json:"writesReviewArtifacts"`
		PlanRoot                  string         `json:"planRoot"`
		RepoRoot                  string         `json:"repoRoot"`
		Pack                      string         `json:"pack"`
		ManifestPath              string         `json:"manifestPath"`
		TargetLane                string         `json:"targetLane"`
		OwnerBinding              OwnerBinding   `json:"ownerBinding"`
		Route                     Route          `json:"route"`
		Input                     Input          `json:"input"`
		ShardPolicy               ShardPolicy    `json:"shardPolicy"`
		Shards                    []Shard        `json:"shards"`
		ShardHandoffs             []ShardHandoff `json:"shardHandoffs"`
		MainAgentResponsibilities string         `json:"mainAgentResponsibilities"`
		SubagentPermissions       string         `json:"subagentPermissions"`
		OutputContract            string         `json:"outputContract"`
		ReviewRequired            bool           `json:"reviewRequired"`
		Observability             Observability  `json:"observability"`
		ReviewLoop                ReviewLoop     `json:"reviewLoop"`
	}{
		SchemaVersion:             packet.SchemaVersion,
		PacketID:                  packet.PacketID,
		Command:                   packet.Command,
		IsMutation:                packet.IsMutation,
		WritesReviewArtifacts:     packet.WritesReviewArtifacts,
		PlanRoot:                  packet.PlanRoot,
		RepoRoot:                  packet.RepoRoot,
		Pack:                      packet.Pack,
		ManifestPath:              packet.ManifestPath,
		TargetLane:                packet.TargetLane,
		OwnerBinding:              packet.OwnerBinding,
		Route:                     packet.Route,
		Input:                     packet.Input,
		ShardPolicy:               packet.ShardPolicy,
		Shards:                    packet.Shards,
		ShardHandoffs:             packet.ShardHandoffs,
		MainAgentResponsibilities: packet.MainAgentResponsibilities,
		SubagentPermissions:       packet.SubagentPermissions,
		OutputContract:            packet.OutputContract,
		ReviewRequired:            packet.ReviewRequired,
		Observability:             packet.Observability,
		ReviewLoop:                packet.ReviewLoop,
	}
	encoded, err := json.Marshal(legacy)
	if err != nil {
		panic(err)
	}
	sum := sha256.Sum256(encoded)
	return "packet-" + hex.EncodeToString(sum[:])[:16]
}

func newShards(items []string, targetItemsPerAgent int) []Shard {
	if targetItemsPerAgent < 1 {
		targetItemsPerAgent = 4
	}
	shards := []Shard{}
	for start := 0; start < len(items); start += targetItemsPerAgent {
		end := min(start+targetItemsPerAgent, len(items))
		slice := append([]string{}, items[start:end]...)
		shards = append(shards, Shard{ID: fmt.Sprintf("shard-%02d", len(shards)+1), Items: slice, Prompt: shardPrompt(slice)})
	}
	return shards
}

func shardPrompt(items []string) string {
	return "Review only these items: " + strings.Join(items, ", ") + ". Return one reviewer result JSON object using the shard handoff dispatchPrompt; do not return routeOutput alone, write files, or paste long logs."
}

func newShardHandoffs(shards []Shard, route Route, observability Observability, reviewLoop ReviewLoop, planRoot, pack string, ownerBinding OwnerBinding, intakeAvailable, collectionAvailable bool) []ShardHandoff {
	handoffs := make([]ShardHandoff, 0, len(shards))
	readOnlyBoundary := append([]string{}, observability.BlockedActions...)
	targetLane := ownerBinding.TargetLane
	for _, shard := range shards {
		contract := reviewerResultContract()
		resultPath := filepath.Join(observability.ResultRoot, shard.ID+".json")
		candidatePath := filepath.Join(observability.ResultRoot, "candidates", shard.ID+".json")
		sourcePath := filepath.Join(observability.ResultRoot, "sources", shard.ID+".json")
		intake := intakeChecklist()
		mappings := reviewerDecisionMappings()
		conflicts := conflictHandlingSteps()
		commands := reviewerIntakeCommands(planRoot, pack, observability.PacketPath, resultPath, targetLane, intakeAvailable)
		var stagingCommands *ReviewerResultStagingCommands
		var collectionCommands *ReviewerResultCollectionCommands
		inputPath := ""
		reviewerResultCandidatePath := ""
		nextAction := "launch a read-only reviewer with agentToolRequest.promptPath, verify promptSha256, inspect its JSON against reviewerResultContract, save the single JSON object directly at reviewerResultPath, then use reviewerIntakeCommands or packet-level batch intake WhatIf before Apply"
		if collectionAvailable {
			inputPath = filepath.Join(observability.ResultRoot, "inputs", shard.ID+".reviewer-input.json")
			staging := reviewerResultStagingCommands(observability.PacketPath, shard.ID, targetLane, inputPath, sourcePath)
			stagingCommands = &staging
			commands := reviewerResultCollectionCommands(observability.PacketPath, shard.ID, targetLane, candidatePath)
			collectionCommands = &commands
			reviewerResultCandidatePath = candidatePath
			nextAction = "launch a read-only reviewer with agentToolRequest.promptPath, verify promptSha256, inspect its JSON against reviewerResultContract, save the single JSON object to reviewerStagingCommands.sourceCaptureInput, run reviewerStagingCommands.sourceCaptureCommand then reviewerStagingCommands.sourceCaptureApply with the expected input hash, run reviewerStagingCommands.previewCommand then its expected-source-hash Apply command, run reviewerCollectionCommands.previewCommand then applyCommand, then use packet-level batch intake WhatIf before Apply; direct plan-subagents -ReviewerResultPath intake remains available for legacy packets"
		} else if !intakeAvailable {
			nextAction = "launch a read-only reviewer with agentToolRequest.promptPath, verify promptSha256, inspect its JSON against reviewerResultContract, and retain the single JSON object; attach or init the target as a rekit case and regenerate a canonical case-local packet before reviewerCollectionCommands or reviewerIntakeCommands become runnable"
		}
		dispatchPrompt := shardDispatchPrompt(shard, route, readOnlyBoundary, reviewLoop, ownerBinding, resultPath, inputPath, collectionAvailable, intakeAvailable)
		handoffs = append(handoffs, ShardHandoff{
			ShardID:                     shard.ID,
			Status:                      "planned",
			ReviewerResultPath:          resultPath,
			ReviewerResultCandidatePath: reviewerResultCandidatePath,
			OwnerBinding:                ownerBinding,
			DispatchPrompt:              dispatchPrompt,
			AgentToolRequest: &ReviewerAgentToolRequest{
				Tool:           "Claude Code Agent",
				AgentType:      "read-only-reviewer",
				ReadOnly:       true,
				Prompt:         dispatchPrompt,
				ExpectedOutput: "exactly one ReviewerResult JSON object; no Markdown fence or surrounding prose",
			},
			ReviewerStagingCommands:    stagingCommands,
			ReviewerCollectionCommands: collectionCommands,
			Items:                      append([]string{}, shard.Items...),
			ReadOnlyBoundary:           append([]string{}, readOnlyBoundary...),
			ExpectedOutput:             route.OutputContract,
			ReviewerWriteback:          reviewLoop.VerdictWriteback,
			ReviewerResultContract:     contract,
			ReviewerIntakeCommands:     commands,
			MainAgentNextAction:        nextAction,
			IntakeChecklist:            intake,
			ReviewerDecisionMappings:   mappings,
			ConflictHandling:           conflicts,
			WritebackSequence:          writebackSequenceSteps(commands),
			PostReviewMerge:            postReviewMergeSteps(),
			CompletionCriteria:         append([]string{}, reviewLoop.CompletionCriteria...),
			FailureHandling:            reviewLoop.FailureHandling,
		})
	}
	return handoffs
}

func writeReviewerPromptArtifacts(promptRoot string, handoffs []ShardHandoff) error {
	promptRoot = strings.TrimSpace(promptRoot)
	if len(handoffs) == 0 {
		return nil
	}
	if promptRoot == "" {
		return fmt.Errorf("reviewer prompt root is required when shard handoffs are planned")
	}
	for idx := range handoffs {
		shardID := strings.TrimSpace(handoffs[idx].ShardID)
		if shardID == "" || strings.ContainsAny(shardID, "/\\") {
			return fmt.Errorf("reviewer prompt artifact shard id is not path-safe: %q", handoffs[idx].ShardID)
		}
		promptPath := filepath.Join(promptRoot, shardID+".prompt.md")
		promptBytes := []byte(strings.TrimRight(handoffs[idx].DispatchPrompt, "\r\n") + "\n")
		if err := os.WriteFile(promptPath, promptBytes, 0o644); err != nil {
			return err
		}
		promptSHA256 := sha256Hex(promptBytes)
		handoffs[idx].DispatchPromptPath = promptPath
		handoffs[idx].DispatchPromptSHA256 = promptSHA256
		if handoffs[idx].AgentToolRequest != nil {
			handoffs[idx].AgentToolRequest.PromptPath = promptPath
			handoffs[idx].AgentToolRequest.PromptSHA256 = promptSHA256
		}
	}
	return nil
}

func reviewerResultContract() ReviewerResultContract {
	contract := reviewerresult.CurrentContract()
	return ReviewerResultContract{
		OutputFormat:     contract.OutputFormat,
		RequiredFields:   append([]string{}, contract.RequiredFields...),
		AllowedDecisions: append([]string{}, contract.AllowedDecisions...),
		EvidenceRules:    append([]string{}, contract.EvidenceRules...),
		ConflictSignals:  append([]string{}, contract.ConflictSignals...),
	}
}

func intakeChecklist() []string {
	return []string{
		"validate reviewer output against reviewerResultContract before using any writeback template",
		"confirm every accepted/rejected item has inspected evidenceRefs and no out-of-shard claims",
		"map reviewer decision to verification verdict before running the verification previewCommand",
		"defer the main decision when conflicts, missing evidence, or blocked outputs are present",
		"use reviewerIntakeCommands.repairGuidance when preview returns blocked, event-id-collision, or post-validation failed",
		"run reviewerIntakeCommands.previewCommand before applyCommand and inspect verification / decision / postValidation before ledger writeback",
	}
}

func reviewerDecisionMappings() []ReviewerDecisionMapping {
	return []ReviewerDecisionMapping{
		{
			ReviewerDecision:    "accept",
			VerificationVerdict: "accepted",
			MainDecision:        "accept",
			ApplyWhen:           []string{"reviewer result validates", "evidenceRefs are inspected", "no conflict signals are present"},
			Fallback:            "defer when evidenceRefs are missing, confidence is low, or conflicts are present",
		},
		{
			ReviewerDecision:    "reject",
			VerificationVerdict: "rejected",
			MainDecision:        "reject",
			ApplyWhen:           []string{"reviewer result validates", "rejection reason cites inspected evidenceRefs", "no out-of-shard claims are present"},
			Fallback:            "defer when rejection evidence cannot be independently inspected",
		},
		{
			ReviewerDecision:    "defer",
			VerificationVerdict: "inconclusive",
			MainDecision:        "defer",
			ApplyWhen:           []string{"reviewer result is valid but evidence is incomplete", "main agent needs another pass or narrower shard"},
			Fallback:            "keep decision=defer and do not apply accept/reject templates until evidence improves",
		},
		{
			ReviewerDecision:    "abandon",
			VerificationVerdict: "inconclusive",
			MainDecision:        "supersede",
			ApplyWhen:           []string{"shard is out of scope, duplicated, or superseded", "main agent has inspected the superseding evidence"},
			Fallback:            "record a defer decision when no superseding evidence has been inspected",
		},
		{
			ReviewerDecision:    "needs-more-evidence",
			VerificationVerdict: "needs-more-evidence",
			MainDecision:        "defer",
			ApplyWhen:           []string{"reviewer cites missing or inaccessible evidenceRefs", "main agent can name the next evidence collection step"},
			Fallback:            "do not apply an accept/reject main decision; collect evidence or split the shard",
		},
	}
}

func conflictHandlingSteps() []string {
	return []string{
		"if reviewer output fails reviewerResultContract validation, do not run writeback templates; retry with a smaller shard or mark decision=defer",
		"if any conflictSignal is present, map verification verdict to inconclusive or needs-more-evidence and keep main decision deferred unless independently resolved",
		"if reviewer decision and recommendedVerdict disagree, inspect evidenceRefs and record the safer non-accepting outcome",
		"if reviewer requests writes, heavy tools, authority/confirmed changes, or external effects, discard that output for ledger purposes and escalate through the lane gate path",
		"if reviewer intake returns blocked, event-id-collision, or post-validation failed, consume repairGuidance action/evidence/boundary before rerunning previewCommand",
	}
}

func writebackSequenceSteps(commands ReviewerIntakeCommands) []WritebackSequenceStep {
	return []WritebackSequenceStep{
		{
			Step:  "validate-reviewer-result",
			Owner: "main-agent",
			Uses:  []string{"reviewerResultContract", "intakeChecklist"},
			CommandBindings: []WritebackCommandBinding{
				{
					Binding:        "reviewer-output",
					Source:         "reviewerResultContract",
					RequiredFields: reviewerResultContract().RequiredFields,
					ExpectedOutput: "single validated reviewer JSON object; no writes or heavy-tool output",
				},
			},
			MustPass:      []string{"single JSON object validates", "required fields are present", "decision uses an allowed value", "evidenceRefs are inspectable for accepted or rejected outcomes"},
			BlockedBy:     []string{"malformed reviewer output", "missing evidenceRefs", "out-of-shard claims", "reviewer requested writes or heavy tools"},
			NextOnSuccess: "map-reviewer-decision",
			NextOnFailure: "defer-or-retry-shard",
		},
		{
			Step:  "map-reviewer-decision",
			Owner: "main-agent",
			Uses:  []string{"reviewerDecisionMappings", "conflictHandling"},
			CommandBindings: []WritebackCommandBinding{
				{
					Binding:        "decision-map",
					Source:         "reviewerDecisionMappings",
					RequiredFields: []string{"reviewerDecision", "verificationVerdict", "mainDecision"},
					ExpectedOutput: "selected verification verdict and main decision",
				},
				{
					Binding:        "conflict-rules",
					Source:         "conflictHandling",
					RequiredFields: []string{"conflicts", "recommendedVerdict", "evidenceRefs"},
					ExpectedOutput: "conflicts absent, independently resolved, or mapped to safer defer outcome",
				},
			},
			MustPass:      []string{"verification verdict is selected", "main decision is selected", "conflict signals are absent or independently resolved"},
			BlockedBy:     []string{"recommendedVerdict disagreement", "unresolved conflictSignal", "low confidence without inspected evidence"},
			NextOnSuccess: "preview-reviewer-intake",
			NextOnFailure: "record-safer-defer-decision",
		},
		{
			Step:  "preview-reviewer-intake",
			Owner: "main-agent",
			Uses:  []string{"reviewerIntakeCommands.previewCommand"},
			CommandBindings: []WritebackCommandBinding{
				reviewerIntakeCommandBinding("reviewer-intake-preview", "reviewerIntakeCommands.previewCommand", commands.PreviewCommand, commands.RequiredFields, "combined reviewer intake WhatIf JSON envelope for verification, decision, overview, handoff, and doctor"),
			},
			MustPass:      []string{"reviewer intake returns isMutation=false", "reviewer intake returns applied=false", "verification and decision previews match reviewerDecisionMappings", "postValidation is valid", "no authority/confirmed or heavy-tool output is present"},
			BlockedBy:     []string{"strict contract validation fails", "wrong packet/case/pack/shard/items", "missing inspected evidenceRefs", "conflict or blocked action is present", "unexpected executor action", "repairGuidance reports unresolved intake blockers"},
			NextOnSuccess: "apply-reviewer-intake",
			NextOnFailure: "stop-before-ledger-write",
		},
		{
			Step:  "apply-reviewer-intake",
			Owner: "main-agent",
			Uses:  []string{"reviewerIntakeCommands.applyCommand"},
			CommandBindings: []WritebackCommandBinding{
				reviewerIntakeCommandBinding("reviewer-intake-apply", "reviewerIntakeCommands.applyCommand", commands.ApplyCommand, commands.RequiredFields, "verification-before-decision ledger writeback with post-validation snapshots"),
			},
			MustPass:      []string{"reviewer intake preview passed", "main agent inspected cited evidenceRefs", "verification event ID is linked from the decision event", "retry remains idempotent"},
			NextOnSuccess: "post-review-validation",
			NextOnFailure: "retry-same-intake-to-complete-writeback",
		},
		{
			Step:  "post-review-validation",
			Owner: "main-agent",
			Uses:  []string{"postReviewMerge", "overview", "handoff", "doctor"},
			CommandBindings: []WritebackCommandBinding{
				{
					Binding:        "post-review-validation",
					Source:         "postReviewMerge",
					RequiredFields: []string{"overview", "handoff", "doctor"},
					ExpectedOutput: "lane state, blocker summary, and handoff readiness rechecked after ledger writes",
				},
			},
			MustPass:      []string{"accepted decisions that affect lane state are rechecked", "handoff or overview reflects the resulting blocker state", "no reviewer output was treated as a ledger event without main-agent apply"},
			NextOnSuccess: "handoff-or-continue-ready-lane",
			NextOnFailure: "open-main-agent-blocker",
		},
	}
}

func reviewerIntakeCommands(planRoot, pack, packetPath, resultPath, targetLane string, intakeAvailable bool) ReviewerIntakeCommands {
	base := "/rekit plan-subagents -Target " + quoteCommandArg(planRoot) + " -Pack " + quoteCommandArg(pack) + " -PacketPath " + quoteCommandArg(packetPath) + " -ReviewerResultPath " + quoteCommandArg(resultPath) + " -Lane " + quoteCommandArg(targetLane) + " -Actor <main-agent>"
	commands := ReviewerIntakeCommands{
		Purpose:        "strictly validate one reviewer result, preview or append verification-before-decision events, and return overview/handoff/doctor post-validation",
		PreviewCommand: base + " -WhatIf -Format json",
		ApplyCommand:   base + " -Apply -Format json",
		RequiredFields: []string{"target", "pack", "packetPath", "reviewerResultPath", "targetLane", "actor"},
		PreviewChecks: []string{
			"run previewCommand before applyCommand",
			"confirm reviewer intake returns isMutation=false, applied=false, and readyForWriteback=true",
			"if reviewer intake returns blocked, event-id-collision, or complete-post-validation-failed, consume repairGuidance[] action/evidence/boundary before rerunning previewCommand",
			"confirm verification and decision previews match the shard, mapped verdict/decision, and cited evidenceRefs",
			"confirm postValidation overview, handoff, and doctor snapshots are valid",
		},
		BlockedOutputs: []string{
			"reviewer output alone must not be treated as a ledger event",
			"previewCommand must not write facts, authority, confirmed, board, lane, handoff, or source files",
			"applyCommand must not run when strict validation fails, blockers are present, the lane is wrong, or evidenceRefs were not inspected",
			"reviewer intake must not execute heavy tools or write authority/confirmed state",
		},
		RepairGuidance: reviewerIntakePlanningRepairGuidance(),
	}
	if !intakeAvailable {
		commands.PreviewCommand = "n/a: reviewer intake requires an attached rekit case; attach or init the target before running -ReviewerResultPath intake"
		commands.ApplyCommand = "n/a: reviewer intake requires an attached rekit case; attach or init the target before running -ReviewerResultPath intake"
		commands.PreviewChecks = append(commands.PreviewChecks, "out-of-case review artifacts are dispatch-only; reviewer intake/writeback is unavailable until the target is an attached rekit case")
		commands.BlockedOutputs = append(commands.BlockedOutputs, "out-of-case plan packets must not be presented as immediately runnable reviewer intake commands")
		commands.RepairGuidance = append(commands.RepairGuidance, ReviewerIntakeRepairGuidance{Reason: "reviewer intake unavailable until target is attached", Action: "attach or init the target case, then regenerate or rerun reviewer intake with case-local packet and reviewer result paths", Evidence: []string{"reviewOutputDir", "packetPath", "reviewerResultPath"}, Boundary: []string{"do not present out-of-case review artifacts as runnable reviewer intake commands", "do not write verification or decision ledger events for dispatch-only artifacts"}})
	}
	return commands
}

func reviewerBatchIntakeCommands(planRoot, pack, packetPath, targetLane string, intakeAvailable bool) (string, string) {
	if !intakeAvailable {
		return "", ""
	}
	base := "/rekit plan-subagents -Target " + quoteCommandArg(planRoot) + " -Pack " + quoteCommandArg(pack) + " -PacketPath " + quoteCommandArg(packetPath) + " -ReadyReviewerResults -Lane " + quoteCommandArg(targetLane) + " -Actor <main-agent>"
	return base + " -WhatIf -Format json", base + " -Apply -Format json"
}

func reviewerResultStagingCommands(packetPath, shardID, targetLane, inputPath, sourcePath string) ReviewerResultStagingCommands {
	inputSaveBase := "/rekit plan-subagents -PacketPath " + quoteCommandArg(packetPath) + " -SaveReviewerResultInput -ShardId " + quoteCommandArg(shardID) + " -ReviewerResultInputSourcePath <reviewer-result-json-path> -Lane " + quoteCommandArg(targetLane) + " -Actor <main-agent>"
	captureBase := "/rekit plan-subagents -PacketPath " + quoteCommandArg(packetPath) + " -CaptureReviewerResultSource -ShardId " + quoteCommandArg(shardID) + " -ReviewerResultInputPath " + quoteCommandArg(inputPath) + " -Lane " + quoteCommandArg(targetLane) + " -Actor <main-agent>"
	base := "/rekit plan-subagents -PacketPath " + quoteCommandArg(packetPath) + " -StageReviewerResult -ShardId " + quoteCommandArg(shardID) + " -ReviewerResultSourcePath " + quoteCommandArg(sourcePath) + " -Lane " + quoteCommandArg(targetLane) + " -Actor <main-agent>"
	return ReviewerResultStagingCommands{
		SourcePath:           sourcePath,
		SourcePathArgument:   sourcePath,
		SourceCaptureInput:   inputPath,
		InputSaveCommand:     inputSaveBase + " -WhatIf -Format json",
		InputSaveApply:       inputSaveBase + " -ExpectedReviewerResultInputSha256 <inputSha256-from-WhatIf> -Apply -Format json",
		SourceCaptureCommand: captureBase + " -WhatIf -Format json",
		SourceCaptureApply:   captureBase + " -ExpectedReviewerResultInputSha256 <inputSha256-from-WhatIf> -Apply -Format json",
		PreviewCommand:       base + " -WhatIf -Format json",
	}
}

func reviewerResultCollectionCommands(packetPath, shardID, targetLane, candidatePath string) ReviewerResultCollectionCommands {
	commands := ReviewerResultCollectionCommands{CandidatePath: candidatePath}
	base := "/rekit plan-subagents -PacketPath " + quoteCommandArg(packetPath) + " -CollectReviewerResult -ShardId " + quoteCommandArg(shardID) + " -Lane " + quoteCommandArg(targetLane) + " -Actor <main-agent>"
	commands.PreviewCommand = base + " -WhatIf -Format json"
	commands.ApplyCommand = base + " -Apply -Format json"
	return commands
}

func reviewerIntakeCommandBinding(binding, source, command string, requiredFields []string, expectedOutput string) WritebackCommandBinding {
	return WritebackCommandBinding{
		Binding:        binding,
		Source:         source,
		Kind:           "reviewer-intake",
		Command:        command,
		RequiredFields: append([]string{}, requiredFields...),
		ExpectedOutput: expectedOutput,
	}
}

func reviewerIntakePlanningRepairGuidance() []ReviewerIntakeRepairGuidance {
	return []ReviewerIntakeRepairGuidance{
		{
			Reason:   "reviewer result has no inspectable evidenceRefs",
			Action:   "add a non-empty case-local bounded evidence file, or cite the packetId when the packet itself is the reviewed evidence; rerun reviewer intake -WhatIf before -Apply",
			Evidence: []string{"ReviewerResult.evidenceRefs", "routeOutput.evidence"},
			Boundary: reviewerIntakePlanningRepairBoundary(),
		},
		{
			Reason:   "reviewer result reports unresolved conflicts",
			Action:   "resolve or split the conflicted reviewer shard, then write a new conflict-free ReviewerResult and rerun -WhatIf",
			Evidence: []string{"ReviewerResult.conflicts[]"},
			Boundary: reviewerIntakePlanningRepairBoundary(),
		},
		{
			Reason:   "reviewer result requests a blocked write, heavy-tool, authority/confirmed, or external-effect action",
			Action:   "replace requested reviewer routeOutput with read-only main-agent handoff; any write, heavy-tool, authority/confirmed, or external-effect action needs a separate gate",
			Evidence: []string{"routeOutput.tool_scope", "routeOutput.next_action"},
			Boundary: reviewerIntakePlanningRepairBoundary(),
		},
		{
			Reason:   "recommendedVerdict conflicts with mapped verification verdict",
			Action:   "align recommendedVerdict with the mapped reviewer decision verdict, or change reviewer decision and rerun -WhatIf",
			Evidence: []string{"ReviewerResult.recommendedVerdict", "reviewerDecisionMappings[]"},
			Boundary: reviewerIntakePlanningRepairBoundary(),
		},
		{
			Reason:   "low-confidence accept/reject cannot be written back without independent evidence review",
			Action:   "collect independent evidence or dispatch a smaller read-only reviewer shard before accepting/rejecting this result",
			Evidence: []string{"ReviewerResult.confidence", "ReviewerResult.decision"},
			Boundary: reviewerIntakePlanningRepairBoundary(),
		},
		{
			Reason:   "event-id-collision or post-validation failure",
			Action:   "inspect deterministic eventId or rerun overview, handoff, and doctor before continuing the lane",
			Evidence: []string{"reviewer intake eventId", "postValidation overview/handoff/doctor"},
			Boundary: reviewerIntakePlanningRepairBoundary(),
		},
	}
}

func reviewerIntakePlanningRepairBoundary() []string {
	return []string{"do not apply reviewer intake until this blocker is resolved", "do not write authority/confirmed or execute heavy tools from reviewer intake"}
}

func quoteCommandArg(value string) string {
	return `"` + strings.ReplaceAll(value, `"`, `\"`) + `"`
}

func postReviewMergeSteps() []string {
	return []string{
		"inspect reviewer output against expectedOutput before reviewer intake",
		"run reviewerIntakeCommands.previewCommand and inspect verification, decision, and postValidation before applyCommand",
		"let the runtime append the reviewer verification event before the linked main decision event; do not let the reviewer append ledger events directly",
		"retry the identical applyCommand when an interrupted writeback needs idempotent completion",
		"consume the returned overview/handoff/doctor postValidation before handing off or continuing the lane",
	}
}

func newReviewerOrchestration(planRoot, pack string, route Route, handoffs []ShardHandoff, observability Observability, reviewLoop ReviewLoop, ownerBinding OwnerBinding, maxParallel int, intakeAvailable, collectionAvailable bool) ReviewerOrchestrationPlan {
	mode := "manual-main-agent-intake"
	scope := "dispatch read-only reviewers, save one JSON result directly to each reviewerResultPath, then run strict direct or packet-level batch intake preview/apply"
	if collectionAvailable {
		scope = "dispatch read-only reviewers, save each JSON to reviewerStagingCommands.sourceCaptureInput, run source capture preview/expected-input-hash apply to publish reviewerStagingCommands.sourcePath, publish a validated packet-derived candidate with staging preview/expected-source-hash apply, publish immutable canonical results with collection preview/apply, then run packet-level ready-result batch intake preview/apply"
	} else if !intakeAvailable {
		mode = "dispatch-only-unattached-target"
		scope = "dispatch read-only reviewers and retain JSON results only; attach or init the target and regenerate a canonical case-local packet before collection or reviewer-intake writeback"
	}
	dispatches := make([]ReviewerDispatch, 0, len(handoffs))
	for _, handoff := range handoffs {
		dispatches = append(dispatches, ReviewerDispatch{
			ShardID:                     handoff.ShardID,
			ReviewerRole:                "read-only-reviewer",
			Status:                      handoff.Status,
			Items:                       append([]string{}, handoff.Items...),
			DispatchPrompt:              handoff.DispatchPrompt,
			DispatchPromptPath:          handoff.DispatchPromptPath,
			DispatchPromptSHA256:        handoff.DispatchPromptSHA256,
			AgentToolRequest:            handoff.AgentToolRequest,
			ReviewerResultPath:          handoff.ReviewerResultPath,
			ReviewerResultCandidatePath: handoff.ReviewerResultCandidatePath,
			StagingCommands:             handoff.ReviewerStagingCommands,
			CollectionCommands:          handoff.ReviewerCollectionCommands,
			PreviewCommand:              handoff.ReviewerIntakeCommands.PreviewCommand,
			ApplyCommand:                handoff.ReviewerIntakeCommands.ApplyCommand,
		})
	}
	batchPreview, batchApply := reviewerBatchIntakeCommands(planRoot, pack, observability.PacketPath, ownerBinding.TargetLane, intakeAvailable)
	managed := newReviewerManagedDispatchPacket(mode, scope, route, observability, reviewLoop, ownerBinding, maxParallel, batchPreview, batchApply, dispatches)
	return ReviewerOrchestrationPlan{
		Mode:                  mode,
		Scope:                 scope,
		TargetLane:            ownerBinding.TargetLane,
		OwnerBinding:          ownerBinding,
		PacketPath:            observability.PacketPath,
		ResultRoot:            observability.ResultRoot,
		ReviewerCount:         len(handoffs),
		BatchPreviewCommand:   batchPreview,
		BatchApplyCommand:     batchApply,
		MaxParallel:           maxParallel,
		ManagedDispatchPacket: &managed,
		Dispatches:            dispatches,
		Lifecycle:             reviewerOrchestrationLifecycle(intakeAvailable, collectionAvailable),
		RuntimeBoundary:       append([]string{}, observability.BlockedActions...),
		CompletionCriteria:    append([]string{}, reviewLoop.CompletionCriteria...),
	}
}

func newReviewerManagedDispatchPacket(mode, scope string, route Route, observability Observability, reviewLoop ReviewLoop, ownerBinding OwnerBinding, maxParallel int, batchPreview, batchApply string, dispatches []ReviewerDispatch) ReviewerManagedDispatchPacket {
	packet := ReviewerManagedDispatchPacket{
		Mode:                mode,
		Scope:               scope,
		TargetLane:          ownerBinding.TargetLane,
		OwnerBinding:        ownerBinding,
		PacketPath:          observability.PacketPath,
		PromptRoot:          observability.PromptRoot,
		ResultRoot:          observability.ResultRoot,
		ReviewerCount:       len(dispatches),
		MaxParallel:         maxParallel,
		BatchPreviewCommand: batchPreview,
		BatchApplyCommand:   batchApply,
		Runbook: []string{
			"verify packetPath, promptPath, and promptSha256 before dispatching any reviewer",
			"dispatch at most maxParallel read-only reviewers using managedDispatchPacket.dispatches in shard order",
			"collect exactly one ReviewerResult JSON object per shard; no Markdown fences or surrounding prose",
			"for canonical attached cases, save reviewer JSON to reviewerResultInputPath and follow source capture, staging, collection, then batch intake preview/apply",
			"for dispatch-only targets, retain reviewer JSON externally and regenerate a canonical case-local packet after attach/init before intake",
		},
		Boundary: mission.UniqueStrings(append(append([]string{}, observability.BlockedActions...),
			"managed dispatch packet is read-only handoff; runtime does not spawn, stop, monitor, or manage reviewer sessions",
			"reviewers must not write files, append ledgers, run heavy tools, or change authority/confirmed state",
			"main agent or replacement executor owns reviewer output validation and all WhatIf/apply writeback steps",
		)),
		CompletionCriteria: append([]string{}, reviewLoop.CompletionCriteria...),
	}
	for idx, dispatch := range dispatches {
		managed := ReviewerManagedDispatch{
			ShardID:                     dispatch.ShardID,
			ReviewerRole:                dispatch.ReviewerRole,
			Status:                      dispatch.Status,
			Items:                       append([]string{}, dispatch.Items...),
			PromptPath:                  dispatch.DispatchPromptPath,
			PromptSHA256:                dispatch.DispatchPromptSHA256,
			AgentToolRequest:            reviewerManagedAgentToolRequest(dispatch),
			ReviewerResultPath:          dispatch.ReviewerResultPath,
			ReviewerResultCandidatePath: dispatch.ReviewerResultCandidatePath,
			IntakePreviewCommand:        dispatch.PreviewCommand,
			IntakeApplyCommand:          dispatch.ApplyCommand,
			DispatchCommand:             reviewerPlanDispatchCommand(ReviewerOrchestrationPlan{Mode: mode, Summary: ReviewerOrchestrationSummary{DispatchOnly: strings.EqualFold(mode, "dispatch-only-unattached-target")}, Dispatches: dispatches}, idx),
			ReviewerResultSkeleton:      reviewerResultSkeletonJSON("packet.packetId", Shard{ID: dispatch.ShardID, Items: append([]string{}, dispatch.Items...)}, route, reviewerRouteOutputPromptSkeleton(splitCSV(route.OutputContract), dispatch.Items)),
			ExpectedOutput:              route.OutputContract,
			NextAction:                  "dispatch read-only reviewer, collect one JSON result, then follow this shard command chain before packet-level batch intake",
			Boundary:                    append([]string{}, packet.Boundary...),
		}
		if dispatch.StagingCommands != nil {
			managed.ReviewerResultInputPath = dispatch.StagingCommands.SourceCaptureInput
			managed.ReviewerResultSourcePath = dispatch.StagingCommands.SourcePath
			managed.InputSavePreviewCommand = dispatch.StagingCommands.InputSaveCommand
			managed.InputSaveApplyCommand = dispatch.StagingCommands.InputSaveApply
			managed.SourceCapturePreviewCommand = dispatch.StagingCommands.SourceCaptureCommand
			managed.SourceCaptureApplyCommand = dispatch.StagingCommands.SourceCaptureApply
			managed.StagingPreviewCommand = dispatch.StagingCommands.PreviewCommand
			managed.NextAction = "save reviewer JSON through inputSavePreviewCommand and its hash-bound apply; run sourceCapturePreviewCommand, sourceCaptureApplyCommand, stagingPreviewCommand, collection preview/apply, then packet-level batch intake preview/apply"
		}
		if dispatch.CollectionCommands != nil {
			managed.CollectionPreviewCommand = dispatch.CollectionCommands.PreviewCommand
			managed.CollectionApplyCommand = dispatch.CollectionCommands.ApplyCommand
		}
		if strings.EqualFold(mode, "dispatch-only-unattached-target") {
			managed.NextAction = "dispatch read-only reviewer and retain the JSON result externally; attach/init the target and regenerate a canonical case-local packet before intake"
		}
		packet.Dispatches = append(packet.Dispatches, managed)
	}
	return packet
}

func reviewerManagedAgentToolRequest(dispatch ReviewerDispatch) ReviewerManagedAgentToolRequest {
	request := ReviewerManagedAgentToolRequest{
		Tool:           "Claude Code Agent",
		AgentType:      "read-only-reviewer",
		ReadOnly:       true,
		PromptPath:     dispatch.DispatchPromptPath,
		PromptSHA256:   dispatch.DispatchPromptSHA256,
		ExpectedOutput: "exactly one ReviewerResult JSON object; no Markdown fence or surrounding prose",
	}
	if dispatch.AgentToolRequest != nil {
		request.Tool = dispatch.AgentToolRequest.Tool
		request.AgentType = dispatch.AgentToolRequest.AgentType
		request.ReadOnly = dispatch.AgentToolRequest.ReadOnly
		request.PromptPath = textOr(dispatch.AgentToolRequest.PromptPath, request.PromptPath)
		request.PromptSHA256 = textOr(dispatch.AgentToolRequest.PromptSHA256, request.PromptSHA256)
		request.ExpectedOutput = textOr(dispatch.AgentToolRequest.ExpectedOutput, request.ExpectedOutput)
	}
	return request
}

func reviewerPlanMissionCommanderAction(planRoot, pack string, orchestration ReviewerOrchestrationPlan, intakeAvailable bool) mission.MissionCommanderAction {
	boundary := reviewerPlanCommanderBoundary(intakeAvailable)
	primary := reviewerPlanDispatchCommand(orchestration, 0)
	if len(orchestration.Dispatches) == 0 {
		return mission.MissionCommanderAction{
			State:          "reviewer-plan-empty",
			Prompt:         "plan-subagents 未生成 reviewer dispatch；补充 review items 后重新规划。",
			PrimaryCommand: "/rekit plan-subagents -Target " + quoteCommandArg(planRoot) + " -Pack " + quoteCommandArg(pack) + " -Items <items>",
			Boundary:       boundary,
		}
	}
	if !intakeAvailable {
		return mission.MissionCommanderAction{
			State:            "reviewer-dispatch-only-target-unattached",
			Prompt:           fmt.Sprintf("plan-subagents 已生成 %d 个 read-only reviewer dispatch，但 target 尚不是 attached rekit case；只能先收集 reviewer JSON，intake writeback 需 init/attach 后再执行。", orchestration.ReviewerCount),
			PrimaryCommand:   primary,
			FollowUpCommands: []string{"/rekit init -Target " + quoteCommandArg(planRoot) + " -Pack " + quoteCommandArg(pack) + " -WhatIf"},
			Boundary:         boundary,
		}
	}
	return mission.MissionCommanderAction{
		State:            "ready-for-reviewer-dispatch",
		Prompt:           fmt.Sprintf("plan-subagents 已为 lane `%s` 生成 %d 个 read-only reviewer dispatch；主 Agent 先分发 reviewer、收集 JSON result input，经 source capture/staging/collection 后再用 batch intake preview/apply 处理所有 ready shards。", orchestration.TargetLane, orchestration.ReviewerCount),
		PrimaryCommand:   primary,
		FollowUpCommands: reviewerPlanFollowUpCommands(orchestration),
		Boundary:         boundary,
	}
}

func reviewerPlanMissionCommanderNextActions(planRoot, pack string, orchestration ReviewerOrchestrationPlan, action mission.MissionCommanderAction, intakeAvailable bool) []mission.MissionCommanderNextActionItem {
	items := []mission.MissionCommanderNextActionItem{}
	label := reviewerPlanLaneLabel(orchestration.TargetLane)
	boundary := append([]string{}, action.Boundary...)
	if action.PrimaryCommand != "" && len(orchestration.Dispatches) == 0 {
		items = append(items, mission.MissionCommanderNextActionItem{
			Lane:           orchestration.TargetLane,
			Label:          label,
			State:          action.State,
			Command:        action.PrimaryCommand,
			Source:         "reviewerOrchestration.plan",
			Blocked:        action.State == "reviewer-plan-empty",
			RequiresReview: true,
			Reasons: []string{
				"top-level plan guidance points at the first reviewer dispatch before any reviewer-intake writeback",
			},
			Boundary: boundary,
		})
	}
	for idx := range orchestration.Dispatches {
		items = append(items, mission.MissionCommanderNextActionItem{
			Lane:           orchestration.TargetLane,
			Label:          label,
			State:          action.State,
			Command:        reviewerPlanDispatchCommand(orchestration, idx),
			Source:         "reviewerOrchestration.dispatch",
			RequiresReview: true,
			Reasons: []string{
				"plan-subagents only wrote review artifacts; main agent owns reviewer spawn and merge",
				"send reviewerOrchestration.dispatches[].dispatchPromptPath to a read-only reviewer, verify promptSha256, and collect one JSON result",
			},
			Boundary: boundary,
		})
		if !intakeAvailable {
			continue
		}
	}
	if intakeAvailable && len(orchestration.Dispatches) > 0 {
		items = append(items, mission.MissionCommanderNextActionItem{
			Lane:           orchestration.TargetLane,
			Label:          label,
			State:          "ready-for-reviewer-batch-intake-preview",
			Command:        orchestration.BatchPreviewCommand,
			Source:         "reviewerOrchestration.batchIntake.preview",
			Blocked:        true,
			RequiresReview: true,
			Reasons: []string{
				"run after one or more read-only reviewer results are written to packet shard paths; missing results remain waiting",
				"batch preview processes ready shards in packet order and must be inspected before apply",
			},
			Boundary: boundary,
		})
		items = append(items, mission.MissionCommanderNextActionItem{
			Lane:           orchestration.TargetLane,
			Label:          label,
			State:          "ready-for-reviewer-batch-intake-apply-after-preview",
			Command:        orchestration.BatchApplyCommand,
			Source:         "reviewerOrchestration.batchIntake.apply",
			Blocked:        true,
			RequiresReview: true,
			Reasons: []string{
				"run only after batch preview confirms every ready shard and cited evidenceRefs were inspected",
				"batch apply writes verification-before-decision per shard, stops at the first blocked/partial/error shard, and supports idempotent retry",
			},
			Boundary: boundary,
		})
	}
	if !intakeAvailable && len(orchestration.Dispatches) > 0 {
		items = append(items, mission.MissionCommanderNextActionItem{
			Lane:           orchestration.TargetLane,
			Label:          label,
			State:          "needs-attached-rekit-case-before-reviewer-intake",
			Command:        "/rekit init -Target " + quoteCommandArg(planRoot) + " -Pack " + quoteCommandArg(pack) + " -WhatIf",
			Source:         "reviewerOrchestration.dispatchOnly.attachTarget",
			RequiresReview: true,
			Reasons: []string{
				"out-of-case review artifacts are dispatch-only; reviewer intake/writeback is unavailable until the target is an attached rekit case",
				"confirm the target should become a rekit case before running init or attach writes",
			},
			Boundary: boundary,
		})
	}
	return mission.UniqueCommanderNextActions(items)
}

func orchestrationCollectionAvailable(orchestration ReviewerOrchestrationPlan) bool {
	for _, dispatch := range orchestration.Dispatches {
		if dispatch.CollectionCommands != nil && strings.TrimSpace(dispatch.CollectionCommands.PreviewCommand) != "" && strings.TrimSpace(dispatch.CollectionCommands.ApplyCommand) != "" {
			return true
		}
	}
	return false
}

func reviewerOrchestrationSummary(orchestration ReviewerOrchestrationPlan) ReviewerOrchestrationSummary {
	summary := ReviewerOrchestrationSummary{
		Mode:                   orchestration.Mode,
		Scope:                  orchestration.Scope,
		TargetLane:             orchestration.TargetLane,
		ReviewerCount:          orchestration.ReviewerCount,
		MaxParallel:            orchestration.MaxParallel,
		PacketPath:             orchestration.PacketPath,
		ResultRoot:             orchestration.ResultRoot,
		BatchPreviewCommand:    orchestration.BatchPreviewCommand,
		BatchApplyCommand:      orchestration.BatchApplyCommand,
		ManagedDispatchSummary: reviewerManagedDispatchSummary(orchestration.ManagedDispatchPacket),
		OwnerBinding: ReviewerOrchestrationOwnerSummary{
			TargetLane:         orchestration.OwnerBinding.TargetLane,
			BindingMode:        orchestration.OwnerBinding.BindingMode,
			CurrentExecutor:    orchestration.OwnerBinding.CurrentExecutor,
			ExecutorGeneration: orchestration.OwnerBinding.ExecutorGeneration,
			RequiredForIntake:  orchestration.OwnerBinding.RequiredForIntake,
			SpawnOwner:         orchestration.OwnerBinding.MainAgentSpawnOwner,
		},
		DispatchCount:       len(orchestration.Dispatches),
		CollectionAvailable: orchestrationCollectionAvailable(orchestration),
		Boundary: []string{
			"planning summary is read-only; full reviewerOrchestration dispatches, lifecycle, action queue, and shard handoffs remain available",
			"runtime only writes review artifacts and does not spawn, stop, monitor, or manage reviewer sessions",
			"reviewers must return one read-only ReviewerResult JSON object; main agent runs reviewer-intake preview before apply",
			"reviewer intake does not write authority/confirmed state and does not execute heavy tools",
		},
	}
	for idx, dispatch := range orchestration.Dispatches {
		dispatchSummary := ReviewerOrchestrationDispatchSummary{
			ShardID:            dispatch.ShardID,
			Status:             dispatch.Status,
			ReviewerResultPath: dispatch.ReviewerResultPath,
			PromptPath:         dispatch.DispatchPromptPath,
			PromptSHA256:       dispatch.DispatchPromptSHA256,
			DispatchCommand:    reviewerPlanDispatchCommand(orchestration, idx),
			PreviewCommand:     dispatch.PreviewCommand,
			ApplyCommand:       dispatch.ApplyCommand,
		}
		summary.Dispatches = append(summary.Dispatches, dispatchSummary)
		if summary.FirstDispatch == nil {
			first := dispatchSummary
			summary.FirstDispatch = &first
		}
		if !strings.HasPrefix(strings.TrimSpace(dispatch.PreviewCommand), "n/a:") || !strings.HasPrefix(strings.TrimSpace(dispatch.ApplyCommand), "n/a:") {
			summary.IntakeAvailable = true
		}
	}
	summary.DispatchOnly = summary.DispatchCount > 0 && !summary.IntakeAvailable
	if orchestration.MissionCommanderActionQueue != nil {
		queue := *orchestration.MissionCommanderActionQueue
		summary.QueueSummary = strings.TrimSpace(queue.Summary)
		summary.ActionTotal = queue.Counts.Total
		summary.ActionUnblocked = queue.Counts.Unblocked
		summary.ActionBlocked = queue.Counts.Blocked
		summary.ActionRequiresReview = queue.Counts.RequiresReview
		summary.ActionFollowUp = queue.Counts.FollowUp
		if queue.CurrentAction != nil {
			current := reviewerOrchestrationNextActionSummary(*queue.CurrentAction)
			summary.CurrentAction = &current
		}
	}
	for _, item := range orchestration.MissionCommanderNextActions {
		summary.NextActions = append(summary.NextActions, reviewerOrchestrationNextActionSummary(item))
	}
	return summary
}

func reviewerManagedDispatchSummary(packet *ReviewerManagedDispatchPacket) *ReviewerManagedDispatchSummary {
	if packet == nil {
		return nil
	}
	summary := ReviewerManagedDispatchSummary{
		Mode:                packet.Mode,
		TargetLane:          packet.TargetLane,
		PacketPath:          packet.PacketPath,
		DispatchCount:       len(packet.Dispatches),
		ReviewerCount:       packet.ReviewerCount,
		MaxParallel:         packet.MaxParallel,
		BatchPreviewCommand: packet.BatchPreviewCommand,
		BatchApplyCommand:   packet.BatchApplyCommand,
		Boundary:            append([]string{}, packet.Boundary...),
	}
	for _, dispatch := range packet.Dispatches {
		item := ReviewerManagedDispatchItemSummary{
			ShardID:                dispatch.ShardID,
			Status:                 dispatch.Status,
			PromptPath:             dispatch.PromptPath,
			PromptSHA256:           dispatch.PromptSHA256,
			ReviewerResultPath:     dispatch.ReviewerResultPath,
			SourceCaptureAvailable: strings.TrimSpace(dispatch.SourceCapturePreviewCommand) != "" && !strings.HasPrefix(strings.TrimSpace(dispatch.SourceCapturePreviewCommand), "n/a:"),
			CollectionAvailable:    strings.TrimSpace(dispatch.CollectionPreviewCommand) != "" && !strings.HasPrefix(strings.TrimSpace(dispatch.CollectionPreviewCommand), "n/a:"),
			IntakeAvailable:        strings.TrimSpace(dispatch.IntakePreviewCommand) != "" && !strings.HasPrefix(strings.TrimSpace(dispatch.IntakePreviewCommand), "n/a:"),
			NextAction:             dispatch.NextAction,
		}
		summary.Dispatches = append(summary.Dispatches, item)
		if summary.FirstDispatch == nil {
			first := item
			summary.FirstDispatch = &first
		}
	}
	return &summary
}

func reviewerOrchestrationNextActionSummary(item mission.MissionCommanderNextActionItem) ReviewerOrchestrationNextActionSummary {
	return ReviewerOrchestrationNextActionSummary{
		State:          item.State,
		Source:         item.Source,
		Command:        item.Command,
		Blocked:        item.Blocked,
		RequiresReview: item.RequiresReview,
	}
}

func reviewerPlanFollowUpCommands(orchestration ReviewerOrchestrationPlan) []string {
	if strings.TrimSpace(orchestration.BatchPreviewCommand) != "" {
		return []string{orchestration.BatchPreviewCommand, orchestration.BatchApplyCommand}
	}
	return reviewerPlanPreviewCommands(orchestration)
}

func reviewerPlanPreviewCommands(orchestration ReviewerOrchestrationPlan) []string {
	commands := []string{}
	for _, dispatch := range orchestration.Dispatches {
		if strings.TrimSpace(dispatch.PreviewCommand) != "" {
			commands = append(commands, dispatch.PreviewCommand)
		}
	}
	return commands
}

func reviewerPlanDispatchCommand(orchestration ReviewerOrchestrationPlan, idx int) string {
	if idx < 0 || idx >= len(orchestration.Dispatches) {
		return ""
	}
	dispatch := orchestration.Dispatches[idx]
	promptRef := reviewerPlanPromptArtifactRef(dispatch, idx)
	if orchestration.Summary.DispatchOnly || strings.EqualFold(orchestration.Mode, "dispatch-only-unattached-target") {
		return "dispatch read-only reviewer for " + dispatch.ShardID + " using " + promptRef + "; retain the returned JSON until a canonical case-local packet is regenerated"
	}
	if dispatch.StagingCommands != nil {
		if strings.TrimSpace(dispatch.StagingCommands.SourceCaptureCommand) != "" || strings.TrimSpace(dispatch.StagingCommands.SourceCaptureApply) != "" {
			inputTarget := strings.TrimSpace(dispatch.StagingCommands.SourceCaptureInput)
			if inputTarget != "" {
				inputTarget = quoteCommandArg(inputTarget)
			} else {
				inputTarget = "a symlink-free case-local input"
			}
			return "dispatch read-only reviewer for " + dispatch.ShardID + " using " + promptRef + "; save the returned JSON to " + inputTarget + ", run source capture preview " + quoteCommandArg(dispatch.StagingCommands.SourceCaptureCommand) + ", then run hash-gated source capture Apply " + quoteCommandArg(dispatch.StagingCommands.SourceCaptureApply) + " to publish " + quoteCommandArg(dispatch.StagingCommands.SourcePath) + "; run staging preview " + quoteCommandArg(dispatch.StagingCommands.PreviewCommand)
		}
		return "dispatch read-only reviewer for " + dispatch.ShardID + " using " + promptRef + "; save the returned JSON to " + quoteCommandArg(dispatch.StagingCommands.SourcePath) + ", then run " + dispatch.StagingCommands.PreviewCommand
	}
	return "dispatch read-only reviewer for " + dispatch.ShardID + " using " + promptRef + "; collect JSON at " + quoteCommandArg(dispatch.ReviewerResultPath)
}

func reviewerPlanPromptArtifactRef(dispatch ReviewerDispatch, idx int) string {
	promptPath := strings.TrimSpace(dispatch.DispatchPromptPath)
	promptSHA256 := strings.TrimSpace(dispatch.DispatchPromptSHA256)
	if dispatch.AgentToolRequest != nil {
		if promptPath == "" {
			promptPath = strings.TrimSpace(dispatch.AgentToolRequest.PromptPath)
		}
		if promptSHA256 == "" {
			promptSHA256 = strings.TrimSpace(dispatch.AgentToolRequest.PromptSHA256)
		}
	}
	if promptPath != "" {
		ref := "prompt artifact " + quoteCommandArg(promptPath)
		if promptSHA256 != "" {
			ref += " (sha256=" + promptSHA256 + ")"
		}
		return ref
	}
	if promptSHA256 != "" {
		return "reviewerOrchestration.dispatches[" + strconv.Itoa(idx) + "].dispatchPromptPath (sha256=" + promptSHA256 + ")"
	}
	return "reviewerOrchestration.dispatches[" + strconv.Itoa(idx) + "].dispatchPromptPath"
}

func reviewerPlanCommanderBoundary(intakeAvailable bool) []string {
	boundary := []string{
		"runtime only writes review artifacts; it does not spawn, stop, monitor, or manage reviewer sessions",
		"reviewers are read-only and must not write files, append ledgers, run heavy tools, or change authority/confirmed state",
		"main agent owns reviewer output validation, evidence review, ledger writeback, and lane handoff",
	}
	if intakeAvailable {
		boundary = append(boundary,
			"run batch reviewer-intake -WhatIf before -Apply; ready shards are processed in packet order and missing shards remain waiting",
			"do not apply reviewer intake while strict validation, blockedReasons, or evidence review are unresolved; acknowledge missing reviewer results will remain waiting",
		)
	} else {
		boundary = append(boundary,
			"out-of-case plan packets are dispatch-only and must not be presented as runnable reviewer intake writeback",
			"confirm before initializing or attaching the target as a rekit case",
		)
	}
	return mission.UniqueStrings(boundary)
}

func reviewerPlanLaneLabel(lane string) string {
	return mission.BoardLaneLabel(mission.BoardLane{ID: lane})
}

func reviewerOrchestrationLifecycle(intakeAvailable, collectionAvailable bool) []ReviewerOrchestrationStep {
	steps := []ReviewerOrchestrationStep{
		{
			Step:          "dispatch-reviewers",
			Owner:         "main-agent",
			Action:        "launch bounded read-only reviewers from reviewerOrchestration.dispatches[].dispatchPromptPath after verifying promptSha256; runtime records the plan but does not spawn, stop, monitor, or manage reviewer sessions",
			Inputs:        []string{"reviewerOrchestration.dispatches[].dispatchPromptPath", "reviewerOrchestration.dispatches[].dispatchPromptSha256", "ownerBinding", "packetPath"},
			MustPass:      []string{"one reviewerSession is assigned per reviewer result", "reviewers receive only the hashed prompt artifact, read-only boundary, and shard items", "no reviewer writes files or ledgers"},
			NextOnSuccess: "collect-results",
			NextOnFailure: "retry-or-split-failed-shards",
		},
		{
			Step:          "collect-results",
			Owner:         "main-agent",
			Action:        "place each single reviewer JSON object at its reviewerResultPath and keep shard order independent",
			Inputs:        []string{"reviewerResultContract", "reviewerOrchestration.dispatches[].reviewerResultPath"},
			MustPass:      []string{"each result is a single JSON object", "packetId/routeId/shardId/items match the packet", "routeOutput uses only outputContract fields"},
			NextOnSuccess: "preview-intake",
			NextOnFailure: "discard-invalid-result",
		},
		{
			Step:          "preview-intake",
			Owner:         "main-agent",
			Action:        "run each dispatch previewCommand before any applyCommand and inspect verification/decision/postValidation",
			Inputs:        []string{"packetPath", "reviewerResultPath", "targetLane", "actor"},
			MustPass:      []string{"isMutation=false", "applied=false", "readyForWriteback=true", "postValidation is valid when target is an attached case"},
			NextOnSuccess: "apply-intake",
			NextOnFailure: "stop-before-ledger-write",
		},
		{
			Step:          "apply-intake",
			Owner:         "main-agent",
			Action:        "run applyCommand for every previewed shard; runtime appends verification before linked main decision and keeps retries idempotent",
			Inputs:        []string{"preview-intake output", "reviewerIntakeCommands.applyCommand"},
			MustPass:      []string{"verification event precedes linked decision event", "duplicate replay returns already-complete without duplicate ledger rows", "authority/confirmed and heavy-tool outputs remain absent"},
			NextOnSuccess: "post-review-validation",
			NextOnFailure: "retry-same-intake-to-complete-writeback",
		},
		{
			Step:          "post-review-validation",
			Owner:         "main-agent",
			Action:        "consume postValidation overview/handoff/doctor snapshots before handing off or continuing the lane",
			Inputs:        []string{"overview", "handoff", "doctor"},
			MustPass:      []string{"lane handoff reflects reviewer verification and main decision facts", "blocked lane does not recommend continue until blockers are resolved", "reviewer output is never treated as a direct ledger event"},
			NextOnSuccess: "handoff-or-continue-ready-lane",
			NextOnFailure: "open-main-agent-blocker",
		},
	}
	if collectionAvailable {
		steps[1].Action = "save each single reviewer JSON object to reviewerStagingCommands.sourceCaptureInput, run reviewerStagingCommands.sourceCaptureCommand, then run sourceCaptureApply with the expected input hash to publish reviewerStagingCommands.sourcePath; run staging WhatIf then its expected-source-hash Apply to the packet-derived candidate, then run collection WhatIf then Apply to publish exact bytes only to the canonical reviewerResultPath"
		steps[1].Inputs = []string{"reviewerResultContract", "reviewerStagingCommands.sourceCaptureInput", "reviewerStagingCommands.sourceCaptureCommand", "reviewerStagingCommands.sourceCaptureApply", "reviewerStagingCommands.sourcePath", "reviewerStagingCommands.previewCommand", "reviewerResultCandidatePath", "reviewerCollectionCommands"}
		steps[1].MustPass = []string{"each input is one symlink-free case-local regular JSON file", "packetId/routeId/shardId/items match the packet", "source capture input hash is reviewed before no-overwrite source publication", "staging source hash is reviewed before no-overwrite candidate publication", "collection preview passes before immutable no-overwrite apply"}
	} else if intakeAvailable {
		steps[1].Action = "save each single reviewer JSON object directly at reviewerResultPath; this custom packet has no canonical collection capability"
		steps[1].Inputs = []string{"reviewerResultContract", "reviewerOrchestration.dispatches[].reviewerResultPath"}
		steps[1].MustPass = []string{"each result is a single JSON object", "packetId/routeId/shardId/items match the packet", "do not run reviewer result collection for this noncanonical packet"}
	}
	if !intakeAvailable {
		steps[1].Action = "retain each reviewer JSON object externally and regenerate a canonical case-local packet after attach or init"
		steps[1].Inputs = []string{"reviewerResultContract", "reviewerOrchestration.dispatches[].agentToolRequest.promptPath", "reviewerOrchestration.dispatches[].agentToolRequest.promptSha256"}
		steps[1].MustPass = []string{"each result is a single JSON object", "do not present collection or intake commands as runnable", "regenerate the packet after attachment before writeback"}
		steps[2].Action = "defer reviewer-intake until the target is attached or initialized as a rekit case"
		steps[2].MustPass = []string{"previewCommand is n/a for dispatch-only out-of-case review artifacts", "do not expect readyForWriteback or postValidation until the target is an attached rekit case"}
		steps[2].NextOnSuccess = "attach-or-init-target"
		steps[3].Action = "do not apply reviewer intake from out-of-case dispatch-only packets"
		steps[3].MustPass = []string{"applyCommand is n/a until target case attachment is available", "no verification or decision ledger events are expected for dispatch-only artifacts"}
		steps[3].NextOnSuccess = "attach-or-init-target"
	}
	return steps
}

func reviewerResultPromptSkeleton(shard Shard, route Route) string {
	return reviewerResultSkeletonJSON("packet.packetId", shard, route, reviewerRouteOutputPromptSkeleton(splitCSV(route.OutputContract), shard.Items))
}

func reviewerResultSummarySkeleton(packetID string, handoff ShardHandoff, route Route) string {
	shard := Shard{ID: handoff.ShardID, Items: append([]string{}, handoff.Items...)}
	return reviewerResultSkeletonJSON(packetID, shard, route, reviewerRouteOutputSummarySkeleton(splitCSV(handoff.ExpectedOutput), handoff.Items, packetID))
}

func reviewerResultSkeletonJSON(packetID string, shard Shard, route Route, routeOutput map[string]string) string {
	evidenceRef := strings.TrimSpace(packetID)
	if evidenceRef == "" {
		evidenceRef = "packet.packetId"
	}
	skeleton := struct {
		PacketID           string            `json:"packetId"`
		RouteID            string            `json:"routeId"`
		ShardID            string            `json:"shardId"`
		Items              []string          `json:"items"`
		ReviewerSession    string            `json:"reviewerSession"`
		Decision           string            `json:"decision"`
		Confidence         string            `json:"confidence"`
		Summary            string            `json:"summary"`
		EvidenceRefs       []string          `json:"evidenceRefs"`
		Risks              []string          `json:"risks"`
		Conflicts          []string          `json:"conflicts"`
		RecommendedVerdict string            `json:"recommendedVerdict"`
		RouteOutput        map[string]string `json:"routeOutput"`
	}{
		PacketID:           evidenceRef,
		RouteID:            route.ID,
		ShardID:            shard.ID,
		Items:              append([]string{}, shard.Items...),
		ReviewerSession:    "reviewer-session-id",
		Decision:           "select after inspecting evidence",
		Confidence:         "select after inspecting evidence",
		Summary:            "fill evidence-based summary for this shard",
		EvidenceRefs:       []string{evidenceRef},
		Risks:              []string{},
		Conflicts:          []string{},
		RecommendedVerdict: "map from the selected decision",
		RouteOutput:        routeOutput,
	}
	b, err := json.Marshal(skeleton)
	if err != nil {
		return "{}"
	}
	return string(b)
}

func reviewerRouteOutputPromptSkeleton(fields []string, items []string) map[string]string {
	return reviewerRouteOutputSkeleton(fields, items, "packet.packetId")
}

func reviewerRouteOutputSummarySkeleton(fields []string, items []string, packetID string) map[string]string {
	return reviewerRouteOutputSkeleton(fields, items, packetID)
}

func reviewerRouteOutputSkeleton(fields []string, items []string, evidenceRef string) map[string]string {
	evidenceRef = strings.TrimSpace(evidenceRef)
	if evidenceRef == "" {
		evidenceRef = "packet.packetId"
	}
	routeOutput := map[string]string{}
	for _, field := range fields {
		switch field {
		case "item":
			routeOutput[field] = strings.Join(items, ",")
		case "decision":
			routeOutput[field] = "match the evidence-based top-level decision"
		case "confidence":
			routeOutput[field] = "match the evidence-based top-level confidence"
		case "evidence":
			routeOutput[field] = evidenceRef
		case "risk":
			routeOutput[field] = "summarize evidence-based residual risk"
		case "next_action":
			routeOutput[field] = "main-agent review"
		case "tier_used":
			routeOutput[field] = "reviewer"
		case "tool_scope":
			routeOutput[field] = "read-only"
		case "defer_reason":
			routeOutput[field] = "explain deferral or use n/a for a non-deferred decision"
		default:
			routeOutput[field] = "fill " + field
		}
	}
	return routeOutput
}

func reviewerRouteOutputFieldHints(outputContract string, items []string) string {
	return reviewerRouteOutputFieldHintsFor(splitCSV(outputContract), items, "packet.packetId")
}

func reviewerRouteOutputSummaryFieldHints(outputContract string, items []string, packetID string) string {
	return reviewerRouteOutputFieldHintsFor(splitCSV(outputContract), items, packetID)
}

func reviewerRouteOutputFieldHintsFor(fields []string, items []string, evidenceRef string) string {
	skeleton := reviewerRouteOutputSkeleton(fields, items, evidenceRef)
	hints := make([]string, 0, len(fields))
	for _, field := range fields {
		hints = append(hints, field+"="+skeleton[field])
	}
	return strings.Join(hints, "; ")
}

func reviewerEvidenceIntegrityGuidance(items []string) string {
	const manifestSuffix = "/evidence/manifest.json"
	bindings := []string{}
	for _, item := range items {
		item = strings.TrimSpace(item)
		if !strings.HasSuffix(strings.ToLower(item), manifestSuffix) {
			continue
		}
		attemptRoot := item[:len(item)-len(manifestSuffix)]
		bindings = append(bindings, fmt.Sprintf(
			"item %q uses immutable task context %q and exact canonical outputs root %q",
			item,
			attemptRoot+"/task-context.json",
			attemptRoot+"/evidence/outputs",
		))
	}
	if len(bindings) == 0 {
		return "Inspect every cited bounded item before selecting a verdict; do not treat the packet skeleton itself as evidence that the item passed."
	}
	return "For each member evidence manifest, use its own exact binding: " + strings.Join(bindings, "; ") + ". For each binding, first read that immutable task context and the manifest item. Resolve every manifest.outputs[].path by joining it beneath that item's exact canonical outputs root, and inspect the resulting file rather than looking beside the manifest or in the case workspace. Compare each output content against its taskContext.goal, taskContext.expectedOutput, and any taskContext.correction including reviewerRejection evidence. Treat explicit acceptance requirements in the task context as mandatory output conditions: if an inspected output acknowledges that a required value or condition is missing, that is contrary evidence and must produce decision=reject with recommendedVerdict=rejected, not accept or defer. A self-consistent manifest does not satisfy a missing goal, acceptance requirement, or correction requirement. Verify that each manifest path, owner, output SHA-256, and byte-count binding is coherent. Keep every item exactly as listed in Items and keep routeOutput.item equal to the reviewed item required by the route contract. The deterministic runtime revalidates declared SHA-256 and byte counts during strict completion; do not invent a missing semantic or provenance check."
}

func shardDispatchPrompt(shard Shard, route Route, readOnlyBoundary []string, reviewLoop ReviewLoop, ownerBinding OwnerBinding, resultPath, inputPath string, collectionAvailable, intakeAvailable bool) string {
	contract := reviewerResultContract()
	resultHandling := "Return the result to the main agent. The main agent will save it directly at reviewerResultPath for strict reviewer intake: " + resultPath + ". Do not write ledger paths yourself."
	if collectionAvailable {
		resultHandling = "Return the result to the main agent. The main agent will save it to reviewerStagingCommands.sourceCaptureInput (" + inputPath + "), then run reviewerStagingCommands.sourceCaptureCommand and its expected-input-hash Apply to publish reviewerStagingCommands.sourcePath before staging/collection publication to: " + resultPath + ". Do not write source, candidate, canonical result, or ledger paths yourself."
	} else if !intakeAvailable {
		resultHandling = "Return the result to the main agent. The main agent will retain it until the target is attached or initialized and a canonical case-local packet is regenerated. Do not write result or ledger paths yourself."
	}
	lines := []string{
		"You are a read-only reviewer for rekit plan-subagents shard " + shard.ID + ".",
		"Route: " + route.ID + ".",
		"Items: " + strings.Join(shard.Items, ", ") + ".",
		"Owner binding: targetLane=" + ownerBinding.TargetLane + ", mode=" + ownerBinding.BindingMode + ", currentExecutor=" + textOr(ownerBinding.CurrentExecutor, "unassigned") + ", executorGeneration=" + strconv.Itoa(ownerBinding.ExecutorGeneration) + ".",
		"Return exactly one reviewer result JSON object; do not return routeOutput alone.",
		"Reviewer result contract: " + contract.OutputFormat + ".",
		"Required result fields: " + strings.Join(contract.RequiredFields, ", ") + ".",
		"Route output required fields: " + reviewerRouteOutputFieldHints(route.OutputContract, shard.Items) + ".",
		"Reviewer result JSON shape template: " + reviewerResultPromptSkeleton(shard, route) + ".",
		"The shape template contains instructions, not default verdict values: inspect the listed evidence before selecting decision, confidence, recommendedVerdict, risk, next_action, and defer_reason; do not copy instruction text into the result.",
		"Choose accept with recommendedVerdict=accepted when the immutable evidence and its declared hashes support the bounded item; choose reject only with inspected contrary evidence; choose defer or needs-more-evidence only when a concrete evidence gap remains.",
		"Use the exact canonical decision mapping for recommendedVerdict: accept=accepted, reject=rejected, defer=inconclusive, abandon=inconclusive, needs-more-evidence=needs-more-evidence. Do not invent synonyms such as deferred.",
		reviewerEvidenceIntegrityGuidance(shard.Items),
		"Replace packet.packetId with the packet packetId, set routeId to " + route.ID + ", shardId to " + shard.ID + ", and set reviewerSession to your session identifier supplied by the main agent.",
		"Avoid blocked intake: set evidenceRefs to the exact packetId from the JSON shape template and set routeOutput.evidence to that same packetId; never substitute an item path, absolute path, reviewerResultPath, candidate path, diff path, or another identifier. Keep conflicts empty unless unresolved, align recommendedVerdict with decision mapping, set tool_scope exactly to read-only and next_action exactly to main-agent review, and do not request writes, heavy tools, authority/confirmed, or external effects.",
		"If blocked intake is unavoidable, return a safer needs-more-evidence/defer result and let the main agent consume reviewerIntakeCommands.repairGuidance before rerunning previewCommand.",
		"Return items exactly as listed in Items, keep routeOutput.item exactly equal to the reviewed item, keep routeOutput.decision and routeOutput.confidence equal to the top-level decision/confidence, and keep routeOutput.evidence inside evidenceRefs.",
		"Allowed decisions: " + strings.Join(contract.AllowedDecisions, ", ") + ".",
		resultHandling,
		"Do not write files, run heavy tools, append ledgers, or change authority/confirmed state.",
		"The main agent owns reviewer spawn, merge, validation, handoff, and ledger writeback: " + reviewLoop.VerdictWriteback + ".",
	}
	if len(readOnlyBoundary) > 0 {
		lines = append(lines, "Blocked runtime actions: "+strings.Join(readOnlyBoundary, "; ")+".")
	}
	return strings.Join(lines, " ")
}

func newObservability(route Route, opt Options, paths artifactPaths, shards []Shard) Observability {
	statuses := make([]ShardStatus, 0, len(shards))
	for _, shard := range shards {
		statuses = append(statuses, ShardStatus{ShardID: shard.ID, Status: "planned", ItemCount: len(shard.Items), ExpectedOutput: route.OutputContract})
	}
	return Observability{
		DispatchMode: "manual-main-agent",
		RouteDebug: RouteDebug{
			SelectedBy:    routeSelectionReason(route, opt),
			RouteID:       route.ID,
			TaskTypes:     route.TaskTypes,
			Trigger:       route.Trigger,
			Reference:     route.Reference,
			PolicyOverlay: route.PolicyOverlay,
		},
		ReviewRoot:       paths.Root,
		ResultRoot:       paths.ResultRoot,
		PromptRoot:       paths.PromptRoot,
		PacketPath:       paths.PacketPath,
		SummaryPath:      paths.SummaryPath,
		CombinedDiffPath: paths.CombinedDiffPath,
		ShardStatuses:    statuses,
		BlockedActions: []string{
			"runtime does not spawn subagents",
			"subagents must not write files",
			"main agent owns ledger writeback, validation, handoff, authority, and confirmed writes",
		},
	}
}

func routeSelectionReason(route Route, opt Options) string {
	if strings.TrimSpace(opt.Route) != "" {
		return "route"
	}
	taskType := strings.TrimSpace(opt.TaskType)
	if taskType != "" {
		for _, task := range strings.FieldsFunc(route.TaskTypes, func(r rune) bool { return r == ',' || r == ';' }) {
			if strings.EqualFold(strings.TrimSpace(task), taskType) {
				return "taskType"
			}
		}
		return "manifest-default"
	}
	return "manifest-default"
}

func newReviewLoop(route Route) ReviewLoop {
	mainOwns := splitCSV(route.MainAgentOwns)
	return ReviewLoop{
		SpawnOwner:       "main-agent",
		MergeOwner:       "main-agent",
		MainAgentOwns:    mainOwns,
		VerdictWriteback: "/rekit plan-subagents -ReviewerResultPath ... -WhatIf/-Apply validates reviewer results and writes verification-before-decision facts for the main agent",
		CompletionCriteria: []string{
			"each planned shard is accepted, rejected, deferred, or explicitly abandoned",
			"reviewer verdicts are recorded in the ledger before main merge decisions",
			"accepted writes remain gated by main-agent validation and authority/confirmed confirmation",
		},
		FailureHandling: "discard failed shard result and retry later with a smaller bounded shard; do not block unrelated shards",
	}
}

func textOr(value, fallback string) string {
	if strings.TrimSpace(value) != "" {
		return strings.TrimSpace(value)
	}
	return fallback
}

func splitCSV(value string) []string {
	items := []string{}
	for _, part := range strings.FieldsFunc(value, func(r rune) bool { return r == ',' || r == ';' }) {
		part = strings.TrimSpace(part)
		if part != "" {
			items = append(items, part)
		}
	}
	return items
}

func optionInt(value string, fallback int) int {
	if n, err := strconv.Atoi(strings.TrimSpace(value)); err == nil && n > 0 {
		return n
	}
	return fallback
}

func makeArtifactPaths(planRoot string, opt Options) (artifactPaths, error) {
	defaultRoot := strings.TrimSpace(opt.ReviewOutputDir) == ""
	root := strings.TrimSpace(opt.ReviewOutputDir)
	var err error
	if defaultRoot {
		rel, relErr := projectstate.Rel(planRoot, "reviews", time.Now().Format("20060102-150405000")+"-"+commandName)
		if relErr != nil {
			return artifactPaths{}, relErr
		}
		root, err = refsf.SafeJoin(planRoot, rel)
		if err != nil {
			return artifactPaths{}, err
		}
	} else if root, err = filepath.Abs(root); err != nil {
		return artifactPaths{}, err
	}
	packet := strings.TrimSpace(opt.PacketPath)
	if packet == "" {
		packet = filepath.Join(root, "packet.json")
	} else if packet, err = filepath.Abs(packet); err != nil {
		return artifactPaths{}, err
	}
	diffRoot := filepath.Join(root, "diffs")
	combined := strings.TrimSpace(opt.DiffPath)
	if combined == "" {
		combined = filepath.Join(diffRoot, "combined.diff")
	} else if combined, err = filepath.Abs(combined); err != nil {
		return artifactPaths{}, err
	}
	if defaultRoot {
		if err := requirePathUnder(root, packet, "packet path"); err != nil {
			return artifactPaths{}, err
		}
		if err := requirePathUnder(root, combined, "diff path"); err != nil {
			return artifactPaths{}, err
		}
	}
	return artifactPaths{Root: root, DiffRoot: diffRoot, PreviewRoot: filepath.Join(root, "previews"), ResultRoot: filepath.Join(root, "results"), PromptRoot: filepath.Join(root, "prompts"), PacketPath: packet, SummaryPath: filepath.Join(root, "summary.md"), CombinedDiffPath: combined}, nil
}

func requirePathUnder(root, path, label string) error {
	rootFull, err := filepath.Abs(root)
	if err != nil {
		return err
	}
	pathFull, err := filepath.Abs(path)
	if err != nil {
		return err
	}
	rootClean := strings.TrimRight(filepath.Clean(rootFull), string(filepath.Separator))
	pathClean := strings.TrimRight(filepath.Clean(pathFull), string(filepath.Separator))
	prefix := rootClean + string(filepath.Separator)
	if !strings.EqualFold(pathClean, rootClean) && !strings.HasPrefix(strings.ToLower(pathClean), strings.ToLower(prefix)) {
		return fmt.Errorf("%s escapes review root: %s", label, path)
	}
	return nil
}

func prepareArtifactDirs(paths artifactPaths) error {
	for _, dir := range []string{paths.Root, paths.DiffRoot, paths.PreviewRoot, paths.ResultRoot, paths.PromptRoot, filepath.Join(paths.ResultRoot, "inputs"), filepath.Join(paths.ResultRoot, "sources"), filepath.Join(paths.ResultRoot, "candidates"), filepath.Dir(paths.PacketPath), filepath.Dir(paths.CombinedDiffPath)} {
		if err := os.MkdirAll(dir, 0o755); err != nil {
			return err
		}
	}
	if err := os.Remove(paths.CombinedDiffPath); err != nil && !os.IsNotExist(err) {
		return err
	}
	return nil
}

func writeJSON(path string, v any) error {
	b, err := json.MarshalIndent(v, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(path, append(b, '\n'), 0o644)
}

func summaryText(packetID string, route Route, taskType string, itemCount, shardCount, itemsPerAgent, maxParallel int, observability Observability, reviewLoop ReviewLoop, ownerBinding OwnerBinding, shardHandoffs []ShardHandoff, orchestration ReviewerOrchestrationPlan) string {
	lines := []string{
		"# rekit subagent plan",
		"",
		"- route: `" + route.ID + "`",
		"- task type: `" + taskType + "`",
		fmt.Sprintf("- items: `%d`", itemCount),
		fmt.Sprintf("- shards: `%d`", shardCount),
		fmt.Sprintf("- target items per agent: `%d`", itemsPerAgent),
		fmt.Sprintf("- max parallel: `%d`", maxParallel),
		"- writes review artifacts: `true`",
		"",
		"## bounded dispatch observability",
		"",
		"- dispatch mode: `" + observability.DispatchMode + "`",
		"- route selected by: `" + observability.RouteDebug.SelectedBy + "`",
		"- review root: `" + observability.ReviewRoot + "`",
		"- reviewer result root: `" + observability.ResultRoot + "`",
		"- reviewer prompt root: `" + observability.PromptRoot + "`",
		"- packet: `" + observability.PacketPath + "`",
		"- combined diff: `" + observability.CombinedDiffPath + "`",
		"- spawn owner: `" + reviewLoop.SpawnOwner + "`",
		"- merge owner: `" + reviewLoop.MergeOwner + "`",
		"- verdict writeback: `" + reviewLoop.VerdictWriteback + "`",
		"- owner binding target lane: `" + ownerBinding.TargetLane + "`",
		"- owner binding mode: `" + ownerBinding.BindingMode + "`",
		"- owner binding current executor: `" + textOr(ownerBinding.CurrentExecutor, "unassigned") + "`",
		fmt.Sprintf("- owner binding executor generation: `%d`", ownerBinding.ExecutorGeneration),
		"- owner binding required for intake: `" + fmt.Sprintf("%t", ownerBinding.RequiredForIntake) + "`",
		"",
		"### shard status",
		"",
	}
	if len(observability.ShardStatuses) == 0 {
		lines = append(lines, "- no shards planned")
	} else {
		for _, status := range observability.ShardStatuses {
			lines = append(lines, fmt.Sprintf("- %s: `%s`, items=`%d`", status.ShardID, status.Status, status.ItemCount))
		}
	}
	lines = append(lines,
		"",
		"### blocked runtime actions",
		"",
	)
	for _, action := range observability.BlockedActions {
		lines = append(lines, "- "+action)
	}
	lines = append(lines,
		"",
		"### reviewer orchestration",
		"",
		"- mode: `"+orchestration.Mode+"`",
		"- scope: `"+orchestration.Scope+"`",
		fmt.Sprintf("- reviewer count: `%d`", orchestration.ReviewerCount),
		fmt.Sprintf("- max parallel: `%d`", orchestration.MaxParallel),
		"- result root: `"+orchestration.ResultRoot+"`",
	)
	summary := orchestration.Summary
	lines = append(lines,
		fmt.Sprintf("- reviewer orchestration summary: mode=`%s`; targetLane=`%s`; reviewers=`%d`; dispatches=`%d`; maxParallel=`%d`; intakeAvailable=`%t`; collectionAvailable=`%t`; dispatchOnly=`%t`; actions=`%d`; unblocked=`%d`; blocked=`%d`; requiresReview=`%d`; followUp=`%d`; queue=`%s`", summary.Mode, summary.TargetLane, summary.ReviewerCount, summary.DispatchCount, summary.MaxParallel, summary.IntakeAvailable, summary.CollectionAvailable, summary.DispatchOnly, summary.ActionTotal, summary.ActionUnblocked, summary.ActionBlocked, summary.ActionRequiresReview, summary.ActionFollowUp, summary.QueueSummary),
		fmt.Sprintf("- reviewer orchestration summary owner: targetLane=`%s`; mode=`%s`; currentExecutor=`%s`; generation=`%d`; requiredForIntake=`%t`; spawnOwner=`%s`", summary.OwnerBinding.TargetLane, summary.OwnerBinding.BindingMode, textOr(summary.OwnerBinding.CurrentExecutor, "unassigned"), summary.OwnerBinding.ExecutorGeneration, summary.OwnerBinding.RequiredForIntake, summary.OwnerBinding.SpawnOwner),
	)
	if managed := summary.ManagedDispatchSummary; managed != nil {
		lines = append(lines, fmt.Sprintf("- reviewer managed dispatch summary: mode=`%s`; targetLane=`%s`; packet=`%s`; dispatches=`%d`; reviewers=`%d`; maxParallel=`%d`; batchPreview=`%s`; batchApply=`%s`", managed.Mode, managed.TargetLane, managed.PacketPath, managed.DispatchCount, managed.ReviewerCount, managed.MaxParallel, managed.BatchPreviewCommand, managed.BatchApplyCommand))
		if managed.FirstDispatch != nil {
			first := *managed.FirstDispatch
			lines = append(lines, fmt.Sprintf("- reviewer managed dispatch first: shard=`%s`; status=`%s`; prompt=`%s`; promptSha256=`%s`; result=`%s`; sourceCapture=`%t`; collection=`%t`; intake=`%t`; next=`%s`", first.ShardID, first.Status, first.PromptPath, first.PromptSHA256, first.ReviewerResultPath, first.SourceCaptureAvailable, first.CollectionAvailable, first.IntakeAvailable, first.NextAction))
		}
		for _, boundary := range managed.Boundary {
			lines = append(lines, "- reviewer managed dispatch boundary: "+boundary)
		}
	}
	if strings.TrimSpace(summary.BatchPreviewCommand) != "" {
		lines = append(lines, fmt.Sprintf("- reviewer orchestration summary batch intake: preview=`%s`; apply=`%s`", summary.BatchPreviewCommand, summary.BatchApplyCommand))
	}
	if summary.FirstDispatch != nil {
		dispatch := *summary.FirstDispatch
		lines = append(lines, fmt.Sprintf("- reviewer orchestration summary first dispatch: shard=`%s`; status=`%s`; result=`%s`; prompt=`%s`; promptSha256=`%s`; dispatch=`%s`; preview=`%s`; apply=`%s`", dispatch.ShardID, dispatch.Status, dispatch.ReviewerResultPath, dispatch.PromptPath, dispatch.PromptSHA256, dispatch.DispatchCommand, dispatch.PreviewCommand, dispatch.ApplyCommand))
	}
	if summary.CurrentAction != nil {
		item := *summary.CurrentAction
		lines = append(lines, fmt.Sprintf("- reviewer orchestration summary current action: state=`%s`; source=`%s`; blocked=`%t`; requiresReview=`%t`; command=`%s`", item.State, item.Source, item.Blocked, item.RequiresReview, item.Command))
	}
	for _, item := range summary.NextActions {
		lines = append(lines, fmt.Sprintf("- reviewer orchestration summary next action: state=`%s`; source=`%s`; blocked=`%t`; requiresReview=`%t`; command=`%s`", item.State, item.Source, item.Blocked, item.RequiresReview, item.Command))
	}
	for _, boundary := range summary.Boundary {
		lines = append(lines, "- reviewer orchestration summary boundary: "+boundary)
	}
	if orchestration.MissionCommanderAction != nil {
		action := *orchestration.MissionCommanderAction
		lines = append(lines, fmt.Sprintf("- mission commander action: state=`%s`; primary=`%s`; follow-up=`%s`; prompt=`%s`", action.State, action.PrimaryCommand, strings.Join(action.FollowUpCommands, "; "), action.Prompt))
		for _, boundary := range action.Boundary {
			lines = append(lines, "  - mission commander boundary: "+boundary)
		}
	}
	if orchestration.MissionCommanderActionQueue != nil {
		queue := *orchestration.MissionCommanderActionQueue
		current := "none"
		if queue.CurrentAction != nil {
			current = queue.CurrentAction.Command
		}
		lines = append(lines, fmt.Sprintf("- mission commander action queue: summary=`%s`; total=`%d`; unblocked=`%d`; blocked=`%d`; requiresReview=`%d`; followUp=`%d`; current=`%s`", queue.Summary, queue.Counts.Total, queue.Counts.Unblocked, queue.Counts.Blocked, queue.Counts.RequiresReview, queue.Counts.FollowUp, current))
	}
	for _, item := range orchestration.MissionCommanderNextActions {
		lines = append(lines, fmt.Sprintf("- mission commander next action: state=`%s`; source=`%s`; blocked=`%t`; requiresReview=`%t`; command=`%s`", item.State, item.Source, item.Blocked, item.RequiresReview, item.Command))
		for _, reason := range item.Reasons {
			lines = append(lines, "  - mission commander reason: "+reason)
		}
		for _, boundary := range item.Boundary {
			lines = append(lines, "  - mission commander boundary: "+boundary)
		}
	}
	for _, step := range orchestration.Lifecycle {
		lines = append(lines, fmt.Sprintf("- orchestration-step: `%s`; owner=`%s`; action=`%s`; inputs=`%s`; must-pass=`%s`; next-success=`%s`; next-failure=`%s`", step.Step, step.Owner, step.Action, strings.Join(step.Inputs, ","), strings.Join(step.MustPass, "; "), step.NextOnSuccess, step.NextOnFailure))
	}
	if managed := orchestration.ManagedDispatchPacket; managed != nil {
		lines = append(lines, "### managed reviewer dispatch packet", "")
		for _, step := range managed.Runbook {
			lines = append(lines, "- managed dispatch runbook: "+step)
		}
		for _, dispatch := range managed.Dispatches {
			lines = append(lines, fmt.Sprintf("- managed reviewer dispatch: shard=`%s`; role=`%s`; status=`%s`; prompt=`%s`; promptSha256=`%s`; result=`%s`; input=`%s`; source=`%s`; candidate=`%s`; dispatch=`%s`; sourceCapturePreview=`%s`; sourceCaptureApply=`%s`; stagingPreview=`%s`; collectionPreview=`%s`; collectionApply=`%s`; intakePreview=`%s`; intakeApply=`%s`; next=`%s`", dispatch.ShardID, dispatch.ReviewerRole, dispatch.Status, dispatch.PromptPath, dispatch.PromptSHA256, dispatch.ReviewerResultPath, dispatch.ReviewerResultInputPath, dispatch.ReviewerResultSourcePath, dispatch.ReviewerResultCandidatePath, dispatch.DispatchCommand, dispatch.SourceCapturePreviewCommand, dispatch.SourceCaptureApplyCommand, dispatch.StagingPreviewCommand, dispatch.CollectionPreviewCommand, dispatch.CollectionApplyCommand, dispatch.IntakePreviewCommand, dispatch.IntakeApplyCommand, dispatch.NextAction))
		}
	}
	for _, dispatch := range orchestration.Dispatches {
		lines = append(lines, fmt.Sprintf("- reviewer-dispatch: `%s`; role=`%s`; status=`%s`; prompt=`%s`; promptSha256=`%s`; result=`%s`; preview=`%s`; apply=`%s`", dispatch.ShardID, dispatch.ReviewerRole, dispatch.Status, dispatch.DispatchPromptPath, dispatch.DispatchPromptSHA256, dispatch.ReviewerResultPath, dispatch.PreviewCommand, dispatch.ApplyCommand))
	}
	lines = append(lines,
		"",
		"### shard handoff prompts",
		"",
	)
	if len(shardHandoffs) == 0 {
		lines = append(lines, "- no shard handoffs planned")
	} else {
		for _, handoff := range shardHandoffs {
			lines = append(lines, fmt.Sprintf("- %s: prompt=`%s`; promptSha256=`%s`; expected output=`%s`; main-agent result path=`%s`", handoff.ShardID, handoff.DispatchPromptPath, handoff.DispatchPromptSHA256, handoff.ExpectedOutput, handoff.ReviewerResultPath))
			if handoff.ReviewerStagingCommands != nil {
				lines = append(lines, "  - reviewer result input path: `"+handoff.ReviewerStagingCommands.SourceCaptureInput+"`")
				lines = append(lines, "  - reviewer result source capture preview: `"+handoff.ReviewerStagingCommands.SourceCaptureCommand+"`")
				lines = append(lines, "  - reviewer result source path: `"+handoff.ReviewerStagingCommands.SourcePath+"`")
				lines = append(lines, "  - reviewer staging preview: `"+handoff.ReviewerStagingCommands.PreviewCommand+"`")
			}
			lines = append(lines, "  - reviewer result path: `"+handoff.ReviewerResultPath+"`")
			lines = append(lines, "  - reviewer result skeleton: `"+reviewerResultSummarySkeleton(packetID, handoff, route)+"`")
			lines = append(lines, "  - reviewer routeOutput field hints: `"+reviewerRouteOutputSummaryFieldHints(handoff.ExpectedOutput, handoff.Items, packetID)+"`")
			lines = append(lines, fmt.Sprintf("  - reviewer result binding: packetId=`%s`; routeId=`%s`; shardId=`%s`; keep routeOutput decision/confidence aligned with top-level fields and routeOutput evidence inside evidenceRefs", packetID, route.ID, handoff.ShardID))
			contract := handoff.ReviewerResultContract
			lines = append(lines, fmt.Sprintf("  - reviewer result contract: output=`%s`; required=`%s`; allowed decisions=`%s`", contract.OutputFormat, strings.Join(contract.RequiredFields, ","), strings.Join(contract.AllowedDecisions, ",")))
			for _, rule := range contract.EvidenceRules {
				lines = append(lines, "    - evidence-rule: "+rule)
			}
			for _, signal := range contract.ConflictSignals {
				lines = append(lines, "    - conflict-signal: "+signal)
			}
			for _, item := range handoff.IntakeChecklist {
				lines = append(lines, "    - intake-check: "+item)
			}
			for _, mapping := range handoff.ReviewerDecisionMappings {
				lines = append(lines, fmt.Sprintf("    - decision-map: reviewer=`%s` -> verification=`%s`, main=`%s`; when=`%s`; fallback=`%s`", mapping.ReviewerDecision, mapping.VerificationVerdict, mapping.MainDecision, strings.Join(mapping.ApplyWhen, "; "), mapping.Fallback))
			}
			for _, step := range handoff.ConflictHandling {
				lines = append(lines, "    - conflict-handling: "+step)
			}
			for _, step := range handoff.WritebackSequence {
				lines = append(lines, fmt.Sprintf("    - writeback-step: `%s`; owner=`%s`; uses=`%s`; must-pass=`%s`; next-success=`%s`; next-failure=`%s`", step.Step, step.Owner, strings.Join(step.Uses, ","), strings.Join(step.MustPass, "; "), step.NextOnSuccess, step.NextOnFailure))
				for _, binding := range step.CommandBindings {
					lines = append(lines, fmt.Sprintf("      - command-binding: `%s`; source=`%s`; kind=`%s`; command=`%s`; required=`%s`; expected=`%s`", binding.Binding, binding.Source, binding.Kind, binding.Command, strings.Join(binding.RequiredFields, ","), binding.ExpectedOutput))
				}
				for _, blocked := range step.BlockedBy {
					lines = append(lines, "      - writeback-blocker: "+blocked)
				}
			}
			commands := handoff.ReviewerIntakeCommands
			lines = append(lines, fmt.Sprintf("  - reviewer intake preview: `%s`; apply: `%s`; required=`%s`", commands.PreviewCommand, commands.ApplyCommand, strings.Join(commands.RequiredFields, ",")))
			for _, check := range commands.PreviewChecks {
				lines = append(lines, "    - preview-check: "+check)
			}
			for _, blocked := range commands.BlockedOutputs {
				lines = append(lines, "    - blocked-output: "+blocked)
			}
			for _, repair := range commands.RepairGuidance {
				lines = append(lines, "    - repair-guidance: reason=`"+repair.Reason+"`; action=`"+repair.Action+"`")
				for _, evidence := range repair.Evidence {
					lines = append(lines, "      - repair-evidence: "+evidence)
				}
				for _, boundary := range repair.Boundary {
					lines = append(lines, "      - repair-boundary: "+boundary)
				}
			}
			for _, step := range handoff.PostReviewMerge {
				lines = append(lines, "  - post-review: "+step)
			}
		}
	}
	lines = append(lines,
		"",
		"### completion criteria",
		"",
	)
	for _, criterion := range reviewLoop.CompletionCriteria {
		lines = append(lines, "- "+criterion)
	}
	lines = append(lines,
		"",
		"Use the generated packet to launch read-only subagents. The command only writes review artifacts; the main agent owns project writes, validation, and handoff updates.",
	)
	return strings.Join(lines, "\r\n") + "\r\n"
}
