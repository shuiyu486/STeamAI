package subagents

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	refsf "github.com/shuiyu486/re-context-kits/internal/rekit/fs"
	"github.com/shuiyu486/re-context-kits/internal/rekit/instance"
	"github.com/shuiyu486/re-context-kits/internal/rekit/manifest"
	"github.com/shuiyu486/re-context-kits/internal/rekit/mission"
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

type Packet struct {
	SchemaVersion             int                       `json:"schemaVersion"`
	PacketID                  string                    `json:"packetId"`
	Command                   string                    `json:"command"`
	IsMutation                bool                      `json:"isMutation"`
	WritesReviewArtifacts     bool                      `json:"writesReviewArtifacts"`
	PlanRoot                  string                    `json:"planRoot"`
	RepoRoot                  string                    `json:"repoRoot"`
	Pack                      string                    `json:"pack"`
	ManifestPath              string                    `json:"manifestPath"`
	TargetLane                string                    `json:"targetLane"`
	OwnerBinding              OwnerBinding              `json:"ownerBinding"`
	Route                     Route                     `json:"route"`
	Input                     Input                     `json:"input"`
	ShardPolicy               ShardPolicy               `json:"shardPolicy"`
	Shards                    []Shard                   `json:"shards"`
	ShardHandoffs             []ShardHandoff            `json:"shardHandoffs"`
	ReviewerOrchestration     ReviewerOrchestrationPlan `json:"reviewerOrchestration"`
	MainAgentResponsibilities string                    `json:"mainAgentResponsibilities"`
	SubagentPermissions       string                    `json:"subagentPermissions"`
	OutputContract            string                    `json:"outputContract"`
	ReviewRequired            bool                      `json:"reviewRequired"`
	Observability             Observability             `json:"observability"`
	ReviewLoop                ReviewLoop                `json:"reviewLoop"`
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
	Dispatches                  []ReviewerDispatch                       `json:"dispatches"`
	Lifecycle                   []ReviewerOrchestrationStep              `json:"lifecycle"`
	RuntimeBoundary             []string                                 `json:"runtimeBoundary"`
	CompletionCriteria          []string                                 `json:"completionCriteria"`
	MissionCommanderAction      *mission.MissionCommanderAction          `json:"missionCommanderAction,omitempty"`
	MissionCommanderNextActions []mission.MissionCommanderNextActionItem `json:"missionCommanderNextActions,omitempty"`
	MissionCommanderActionQueue *mission.MissionCommanderActionQueue     `json:"missionCommanderActionQueue,omitempty"`
}

type ReviewerDispatch struct {
	ShardID            string   `json:"shardId"`
	ReviewerRole       string   `json:"reviewerRole"`
	Status             string   `json:"status"`
	Items              []string `json:"items"`
	DispatchPrompt     string   `json:"dispatchPrompt"`
	ReviewerResultPath string   `json:"reviewerResultPath"`
	PreviewCommand     string   `json:"previewCommand"`
	ApplyCommand       string   `json:"applyCommand"`
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
	ShardID                  string                    `json:"shardId"`
	Status                   string                    `json:"status"`
	ReviewerResultPath       string                    `json:"reviewerResultPath"`
	OwnerBinding             OwnerBinding              `json:"ownerBinding"`
	DispatchPrompt           string                    `json:"dispatchPrompt"`
	Items                    []string                  `json:"items"`
	ReadOnlyBoundary         []string                  `json:"readOnlyBoundary"`
	ExpectedOutput           string                    `json:"expectedOutput"`
	ReviewerWriteback        string                    `json:"reviewerWriteback"`
	ReviewerResultContract   ReviewerResultContract    `json:"reviewerResultContract"`
	ReviewerIntakeCommands   ReviewerIntakeCommands    `json:"reviewerIntakeCommands"`
	MainAgentNextAction      string                    `json:"mainAgentNextAction"`
	IntakeChecklist          []string                  `json:"intakeChecklist"`
	ReviewerDecisionMappings []ReviewerDecisionMapping `json:"reviewerDecisionMappings"`
	ConflictHandling         []string                  `json:"conflictHandling"`
	WritebackSequence        []WritebackSequenceStep   `json:"writebackSequence"`
	PostReviewMerge          []string                  `json:"postReviewMerge"`
	CompletionCriteria       []string                  `json:"completionCriteria"`
	FailureHandling          string                    `json:"failureHandling"`
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
	Purpose        string   `json:"purpose"`
	PreviewCommand string   `json:"previewCommand"`
	ApplyCommand   string   `json:"applyCommand"`
	RequiredFields []string `json:"requiredFields"`
	PreviewChecks  []string `json:"previewChecks,omitempty"`
	BlockedOutputs []string `json:"blockedOutputs,omitempty"`
}

type artifactPaths struct {
	Root             string
	DiffRoot         string
	PreviewRoot      string
	ResultRoot       string
	PacketPath       string
	SummaryPath      string
	CombinedDiffPath string
}

func WritePlan(repoRoot, target, pack string, opt Options) (Result, error) {
	planRoot, err := filepath.Abs(target)
	if err != nil {
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
	paths, err := makeArtifactPaths(planRoot, opt)
	if err != nil {
		return Result{}, err
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
	shardHandoffs := newShardHandoffs(shards, route, observability, reviewLoop, planRoot, m.Pack, ownerBinding, caseTarget)
	orchestration := newReviewerOrchestration(shardHandoffs, observability, reviewLoop, ownerBinding, maxParallel, caseTarget)
	commanderAction := reviewerPlanMissionCommanderAction(planRoot, m.Pack, orchestration, caseTarget)
	commanderNextActions := reviewerPlanMissionCommanderNextActions(planRoot, m.Pack, orchestration, commanderAction, caseTarget)
	commanderActionQueue := mission.MissionCommanderActionQueueFor(commanderNextActions)
	orchestration.MissionCommanderAction = &commanderAction
	orchestration.MissionCommanderNextActions = commanderNextActions
	orchestration.MissionCommanderActionQueue = &commanderActionQueue
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
	packet.PacketID = packetIdentity(packet)
	if err := writeJSON(paths.PacketPath, packet); err != nil {
		return Result{}, err
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
		return OwnerBinding{}, fmt.Errorf("reviewer owner binding target lane %q is not present in .rekit/board.json; known: %s", targetLane, strings.Join(mission.BoardLaneIDs(board.Lanes), ","))
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
	if len(m.SubagentRoutes) == 0 {
		return Route{}, fmt.Errorf("manifest has no subagentRoutes: %s", m.ManifestPath)
	}
	if strings.TrimSpace(routeID) != "" {
		for _, route := range m.SubagentRoutes {
			if strings.EqualFold(route.ID, routeID) {
				return toRoute(route), nil
			}
		}
		return Route{}, fmt.Errorf("subagent route not found: %s", routeID)
	}
	if strings.TrimSpace(taskType) != "" {
		for _, route := range m.SubagentRoutes {
			for _, task := range strings.FieldsFunc(route.TaskTypes, func(r rune) bool { return r == ',' || r == ';' }) {
				if strings.EqualFold(strings.TrimSpace(task), taskType) {
					return toRoute(route), nil
				}
			}
		}
	}
	return toRoute(m.SubagentRoutes[0]), nil
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
	return plan.Mode == "" && plan.Scope == "" && plan.TargetLane == "" && plan.OwnerBinding == (OwnerBinding{}) && plan.PacketPath == "" && plan.ResultRoot == "" && plan.ReviewerCount == 0 && plan.MaxParallel == 0 && len(plan.Dispatches) == 0 && len(plan.Lifecycle) == 0 && len(plan.RuntimeBoundary) == 0 && len(plan.CompletionCriteria) == 0 && plan.MissionCommanderAction == nil && len(plan.MissionCommanderNextActions) == 0 && reviewerActionQueueEmpty(plan.MissionCommanderActionQueue)
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

func newShardHandoffs(shards []Shard, route Route, observability Observability, reviewLoop ReviewLoop, planRoot, pack string, ownerBinding OwnerBinding, intakeAvailable bool) []ShardHandoff {
	handoffs := make([]ShardHandoff, 0, len(shards))
	readOnlyBoundary := append([]string{}, observability.BlockedActions...)
	targetLane := ownerBinding.TargetLane
	for _, shard := range shards {
		contract := reviewerResultContract()
		resultPath := filepath.Join(observability.ResultRoot, shard.ID+".json")
		intake := intakeChecklist()
		mappings := reviewerDecisionMappings()
		conflicts := conflictHandlingSteps()
		commands := reviewerIntakeCommands(planRoot, pack, observability.PacketPath, resultPath, targetLane, intakeAvailable)
		nextAction := "launch a read-only reviewer with dispatchPrompt, inspect its JSON against reviewerResultContract, place it at reviewerResultPath, run reviewerIntakeCommands.previewCommand, inspect the combined verification/decision/postValidation envelope, then run reviewerIntakeCommands.applyCommand"
		if !intakeAvailable {
			nextAction = "launch a read-only reviewer with dispatchPrompt, inspect its JSON against reviewerResultContract, and place it at reviewerResultPath; attach or init the target as a rekit case before running reviewerIntakeCommands.previewCommand or applyCommand"
		}
		handoffs = append(handoffs, ShardHandoff{
			ShardID:                  shard.ID,
			Status:                   "planned",
			ReviewerResultPath:       resultPath,
			OwnerBinding:             ownerBinding,
			DispatchPrompt:           shardDispatchPrompt(shard, route, readOnlyBoundary, reviewLoop, ownerBinding, resultPath),
			Items:                    append([]string{}, shard.Items...),
			ReadOnlyBoundary:         append([]string{}, readOnlyBoundary...),
			ExpectedOutput:           route.OutputContract,
			ReviewerWriteback:        reviewLoop.VerdictWriteback,
			ReviewerResultContract:   contract,
			ReviewerIntakeCommands:   commands,
			MainAgentNextAction:      nextAction,
			IntakeChecklist:          intake,
			ReviewerDecisionMappings: mappings,
			ConflictHandling:         conflicts,
			WritebackSequence:        writebackSequenceSteps(commands),
			PostReviewMerge:          postReviewMergeSteps(),
			CompletionCriteria:       append([]string{}, reviewLoop.CompletionCriteria...),
			FailureHandling:          reviewLoop.FailureHandling,
		})
	}
	return handoffs
}

func reviewerResultContract() ReviewerResultContract {
	return ReviewerResultContract{
		OutputFormat:     "single JSON object per shard with route-specific fields nested under routeOutput; no markdown tables, file writes, ledger appends, authority, confirmed, or heavy-tool output",
		RequiredFields:   []string{"packetId", "routeId", "shardId", "items", "reviewerSession", "decision", "confidence", "summary", "evidenceRefs", "risks", "conflicts", "recommendedVerdict", "routeOutput"},
		AllowedDecisions: []string{"accept", "reject", "defer", "abandon", "needs-more-evidence"},
		EvidenceRules: []string{
			"accepted or rejected reviewer decisions must cite evidenceRefs from the packet, reviewed artifacts, or bounded evidence paths",
			"route-specific outputContract fields must be returned inside routeOutput so strict intake can preserve pack-specific data without allowing unknown top-level fields",
			"missing, ambiguous, or inaccessible evidenceRefs require decision=needs-more-evidence or defer",
			"do not paste long logs; cite stable packet/evidence references and summarize the relevant observation",
		},
		ConflictSignals: []string{
			"reviewer decision conflicts with evidenceRefs or route output contract",
			"reviewer requests file writes, ledger append, authority/confirmed changes, heavy tools, or external effects",
			"reviewer output overlaps another shard or changes items outside this shard",
			"reviewer confidence is low or evidence cannot be independently inspected by the main agent",
		},
	}
}

func intakeChecklist() []string {
	return []string{
		"validate reviewer output against reviewerResultContract before using any writeback template",
		"confirm every accepted/rejected item has inspected evidenceRefs and no out-of-shard claims",
		"map reviewer decision to verification verdict before running the verification previewCommand",
		"defer the main decision when conflicts, missing evidence, or blocked outputs are present",
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
			BlockedBy:     []string{"strict contract validation fails", "wrong packet/case/pack/shard/items", "missing inspected evidenceRefs", "conflict or blocked action is present", "unexpected executor action"},
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
			"confirm verification and decision previews match the shard, mapped verdict/decision, and cited evidenceRefs",
			"confirm postValidation overview, handoff, and doctor snapshots are valid",
		},
		BlockedOutputs: []string{
			"reviewer output alone must not be treated as a ledger event",
			"previewCommand must not write facts, authority, confirmed, board, lane, handoff, or source files",
			"applyCommand must not run when strict validation fails, blockers are present, the lane is wrong, or evidenceRefs were not inspected",
			"reviewer intake must not execute heavy tools or write authority/confirmed state",
		},
	}
	if !intakeAvailable {
		commands.PreviewCommand = "n/a: reviewer intake requires an attached rekit case; attach or init the target before running -ReviewerResultPath intake"
		commands.ApplyCommand = "n/a: reviewer intake requires an attached rekit case; attach or init the target before running -ReviewerResultPath intake"
		commands.PreviewChecks = append(commands.PreviewChecks, "out-of-case review artifacts are dispatch-only; reviewer intake/writeback is unavailable until the target is an attached rekit case")
		commands.BlockedOutputs = append(commands.BlockedOutputs, "out-of-case plan packets must not be presented as immediately runnable reviewer intake commands")
	}
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

func newReviewerOrchestration(handoffs []ShardHandoff, observability Observability, reviewLoop ReviewLoop, ownerBinding OwnerBinding, maxParallel int, intakeAvailable bool) ReviewerOrchestrationPlan {
	mode := "manual-main-agent-intake"
	scope := "dispatch read-only reviewers, collect one JSON result per shard, then run reviewer-intake preview/apply for each shard"
	if !intakeAvailable {
		mode = "dispatch-only-unattached-target"
		scope = "dispatch read-only reviewers and collect JSON results only; attach or init the target before reviewer-intake writeback"
	}
	dispatches := make([]ReviewerDispatch, 0, len(handoffs))
	for _, handoff := range handoffs {
		dispatches = append(dispatches, ReviewerDispatch{
			ShardID:            handoff.ShardID,
			ReviewerRole:       "read-only-reviewer",
			Status:             handoff.Status,
			Items:              append([]string{}, handoff.Items...),
			DispatchPrompt:     handoff.DispatchPrompt,
			ReviewerResultPath: handoff.ReviewerResultPath,
			PreviewCommand:     handoff.ReviewerIntakeCommands.PreviewCommand,
			ApplyCommand:       handoff.ReviewerIntakeCommands.ApplyCommand,
		})
	}
	return ReviewerOrchestrationPlan{
		Mode:               mode,
		Scope:              scope,
		TargetLane:         ownerBinding.TargetLane,
		OwnerBinding:       ownerBinding,
		PacketPath:         observability.PacketPath,
		ResultRoot:         observability.ResultRoot,
		ReviewerCount:      len(handoffs),
		MaxParallel:        maxParallel,
		Dispatches:         dispatches,
		Lifecycle:          reviewerOrchestrationLifecycle(intakeAvailable),
		RuntimeBoundary:    append([]string{}, observability.BlockedActions...),
		CompletionCriteria: append([]string{}, reviewLoop.CompletionCriteria...),
	}
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
		Prompt:           fmt.Sprintf("plan-subagents 已为 lane `%s` 生成 %d 个 read-only reviewer dispatch；主 Agent 先分发 reviewer、收集 JSON result，再逐 shard 运行 reviewer-intake preview/apply。", orchestration.TargetLane, orchestration.ReviewerCount),
		PrimaryCommand:   primary,
		FollowUpCommands: reviewerPlanPreviewCommands(orchestration),
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
	for idx, dispatch := range orchestration.Dispatches {
		items = append(items, mission.MissionCommanderNextActionItem{
			Lane:           orchestration.TargetLane,
			Label:          label,
			State:          action.State,
			Command:        reviewerPlanDispatchCommand(orchestration, idx),
			Source:         "reviewerOrchestration.dispatch",
			RequiresReview: true,
			Reasons: []string{
				"plan-subagents only wrote review artifacts; main agent owns reviewer spawn and merge",
				"send reviewerOrchestration.dispatches[].dispatchPrompt to a read-only reviewer and collect one JSON result",
			},
			Boundary: boundary,
		})
		if !intakeAvailable {
			continue
		}
		items = append(items, mission.MissionCommanderNextActionItem{
			Lane:           orchestration.TargetLane,
			Label:          label,
			State:          "ready-for-reviewer-intake-preview",
			Command:        dispatch.PreviewCommand,
			Source:         "reviewerOrchestration.intake.preview",
			Blocked:        true,
			RequiresReview: true,
			Reasons: []string{
				"run only after the read-only reviewer result is written to " + dispatch.ReviewerResultPath,
				"preview must return isMutation=false, applied=false, readyForWriteback=true, and valid postValidation before apply",
			},
			Boundary: boundary,
		})
		items = append(items, mission.MissionCommanderNextActionItem{
			Lane:           orchestration.TargetLane,
			Label:          label,
			State:          "ready-for-reviewer-intake-apply-after-preview",
			Command:        dispatch.ApplyCommand,
			Source:         "reviewerOrchestration.intake.apply",
			Blocked:        true,
			RequiresReview: true,
			Reasons: []string{
				"run only after reviewer-intake preview returns writebackStatus=previewed and cited evidenceRefs were inspected",
				"reviewer-intake apply writes verification-before-decision facts; retry the identical command for partial recovery",
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
	return "dispatch read-only reviewer for " + dispatch.ShardID + " using reviewerOrchestration.dispatches[" + strconv.Itoa(idx) + "].dispatchPrompt; collect JSON at " + quoteCommandArg(dispatch.ReviewerResultPath)
}

func reviewerPlanCommanderBoundary(intakeAvailable bool) []string {
	boundary := []string{
		"runtime only writes review artifacts; it does not spawn, stop, monitor, or manage reviewer sessions",
		"reviewers are read-only and must not write files, append ledgers, run heavy tools, or change authority/confirmed state",
		"main agent owns reviewer output validation, evidence review, ledger writeback, and lane handoff",
	}
	if intakeAvailable {
		boundary = append(boundary,
			"run reviewer-intake -WhatIf before -Apply for every reviewer result",
			"do not apply reviewer intake while strict validation, blockedReasons, or evidence review are unresolved",
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

func reviewerOrchestrationLifecycle(intakeAvailable bool) []ReviewerOrchestrationStep {
	steps := []ReviewerOrchestrationStep{
		{
			Step:          "dispatch-reviewers",
			Owner:         "main-agent",
			Action:        "launch bounded read-only reviewers from reviewerOrchestration.dispatches[]; runtime records the plan but does not spawn, stop, monitor, or manage reviewer sessions",
			Inputs:        []string{"reviewerOrchestration.dispatches[].dispatchPrompt", "ownerBinding", "packetPath"},
			MustPass:      []string{"one reviewerSession is assigned per reviewer result", "reviewers receive only read-only boundary and shard items", "no reviewer writes files or ledgers"},
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
	if !intakeAvailable {
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
		Decision:           "needs-more-evidence",
		Confidence:         "medium",
		Summary:            "fill summary for this shard",
		EvidenceRefs:       []string{evidenceRef},
		Risks:              []string{},
		Conflicts:          []string{},
		RecommendedVerdict: "needs-more-evidence",
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
			routeOutput[field] = "needs-more-evidence"
		case "confidence":
			routeOutput[field] = "medium"
		case "evidence":
			routeOutput[field] = evidenceRef
		case "risk":
			routeOutput[field] = "unknown"
		case "next_action":
			routeOutput[field] = "defer for main-agent evidence review"
		case "tier_used":
			routeOutput[field] = "reviewer"
		case "tool_scope":
			routeOutput[field] = "read-only"
		case "defer_reason":
			routeOutput[field] = "fill defer reason"
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

func shardDispatchPrompt(shard Shard, route Route, readOnlyBoundary []string, reviewLoop ReviewLoop, ownerBinding OwnerBinding, resultPath string) string {
	contract := reviewerResultContract()
	lines := []string{
		"You are a read-only reviewer for rekit plan-subagents shard " + shard.ID + ".",
		"Route: " + route.ID + ".",
		"Items: " + strings.Join(shard.Items, ", ") + ".",
		"Owner binding: targetLane=" + ownerBinding.TargetLane + ", mode=" + ownerBinding.BindingMode + ", currentExecutor=" + textOr(ownerBinding.CurrentExecutor, "unassigned") + ", executorGeneration=" + strconv.Itoa(ownerBinding.ExecutorGeneration) + ".",
		"Return exactly one reviewer result JSON object; do not return routeOutput alone.",
		"Reviewer result contract: " + contract.OutputFormat + ".",
		"Required result fields: " + strings.Join(contract.RequiredFields, ", ") + ".",
		"Route output required fields: " + reviewerRouteOutputFieldHints(route.OutputContract, shard.Items) + ".",
		"Reviewer result JSON skeleton: " + reviewerResultPromptSkeleton(shard, route) + ".",
		"Replace packet.packetId with the packet packetId, set routeId to " + route.ID + ", shardId to " + shard.ID + ", and set reviewerSession to your session identifier supplied by the main agent.",
		"Keep routeOutput.decision and routeOutput.confidence equal to the top-level decision/confidence; keep routeOutput.evidence inside evidenceRefs.",
		"Allowed decisions: " + strings.Join(contract.AllowedDecisions, ", ") + ".",
		"Return the result to the main agent for placement at: " + resultPath + ". Do not write this path yourself.",
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
		root, err = refsf.SafeJoin(planRoot, filepath.ToSlash(filepath.Join(".rekit", "reviews", time.Now().Format("20060102-150405000")+"-"+commandName)))
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
	return artifactPaths{Root: root, DiffRoot: diffRoot, PreviewRoot: filepath.Join(root, "previews"), ResultRoot: filepath.Join(root, "results"), PacketPath: packet, SummaryPath: filepath.Join(root, "summary.md"), CombinedDiffPath: combined}, nil
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
	for _, dir := range []string{paths.Root, paths.DiffRoot, paths.PreviewRoot, paths.ResultRoot, filepath.Dir(paths.PacketPath), filepath.Dir(paths.CombinedDiffPath)} {
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
	for _, dispatch := range orchestration.Dispatches {
		lines = append(lines, fmt.Sprintf("- reviewer-dispatch: `%s`; role=`%s`; status=`%s`; result=`%s`; preview=`%s`; apply=`%s`", dispatch.ShardID, dispatch.ReviewerRole, dispatch.Status, dispatch.ReviewerResultPath, dispatch.PreviewCommand, dispatch.ApplyCommand))
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
			lines = append(lines, fmt.Sprintf("- %s: `%s`; expected output=`%s`; main-agent result path=`%s`", handoff.ShardID, handoff.DispatchPrompt, handoff.ExpectedOutput, handoff.ReviewerResultPath))
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
