package mission

import (
	"encoding/json"
	"fmt"
	"slices"
	"strconv"
	"strings"
)

const DefaultMaxRows = 10

type Lane struct {
	ID     string
	Label  string
	Status string
}

type Facts struct {
	Candidates    []map[string]any
	Requests      []map[string]any
	Decisions     []map[string]any
	Interventions []map[string]any
}

func FactsWithEvent(facts Facts, kind string, event map[string]any) Facts {
	out := Facts{
		Candidates:    append([]map[string]any{}, facts.Candidates...),
		Requests:      append([]map[string]any{}, facts.Requests...),
		Decisions:     append([]map[string]any{}, facts.Decisions...),
		Interventions: append([]map[string]any{}, facts.Interventions...),
	}
	switch strings.ToLower(strings.TrimSpace(kind)) {
	case "candidate":
		out.Candidates = append(out.Candidates, event)
	case "request":
		out.Requests = append(out.Requests, event)
	case "decision":
		out.Decisions = append(out.Decisions, event)
	case "intervention":
		out.Interventions = append(out.Interventions, event)
	}
	return out
}

type BuildOptions struct {
	MaxRows            int
	OpenDecisionAction string
}

type Brief struct {
	Summary          string   `json:"summary"`
	ReadyLanes       []string `json:"readyLanes"`
	BlockedLanes     []string `json:"blockedLanes"`
	PendingGates     []string `json:"pendingGates"`
	AuthorizedGates  []string `json:"authorizedGates"`
	OpenDecisions    []string `json:"openDecisions"`
	Interventions    []string `json:"interventions"`
	NextAgentActions []string `json:"nextAgentActions"`
	Escalations      []string `json:"escalations"`
}

type ExecutorAction struct {
	Blocked                bool                   `json:"blocked"`
	Ready                  bool                   `json:"ready"`
	BlockerReasons         []string               `json:"blockerReasons"`
	PendingGates           int                    `json:"pendingGates"`
	OpenInterventions      int                    `json:"openInterventions"`
	OpenDecisions          int                    `json:"openDecisions"`
	ReconcileRequired      bool                   `json:"reconcileRequired"`
	PendingGateRequired    bool                   `json:"pendingGateRequired"`
	OpenDecisionRequired   bool                   `json:"openDecisionRequired"`
	ResumeCommand          string                 `json:"resumeCommand"`
	HandoffCommand         string                 `json:"handoffCommand"`
	NextAgentActions       []string               `json:"nextAgentActions"`
	Escalations            []string               `json:"escalations"`
	MissionCommanderAction MissionCommanderAction `json:"missionCommanderAction"`
}

type MissionCommanderAction struct {
	State            string   `json:"state"`
	Prompt           string   `json:"prompt"`
	PrimaryCommand   string   `json:"primaryCommand,omitempty"`
	FollowUpCommands []string `json:"followUpCommands,omitempty"`
	Boundary         []string `json:"boundary,omitempty"`
}

type MissionCommanderNextActionItem struct {
	Lane           string   `json:"lane,omitempty"`
	Label          string   `json:"label,omitempty"`
	GateEventID    string   `json:"gateEventId,omitempty"`
	ActionID       string   `json:"actionId,omitempty"`
	State          string   `json:"state"`
	Command        string   `json:"command"`
	Source         string   `json:"source"`
	Blocked        bool     `json:"blocked,omitempty"`
	RequiresReview bool     `json:"requiresReview,omitempty"`
	Reasons        []string `json:"reasons,omitempty"`
	Boundary       []string `json:"boundary,omitempty"`
}

type MissionCommanderRunLoopStep struct {
	StepID      string   `json:"stepId"`
	Order       int      `json:"order"`
	Actor       string   `json:"actor"`
	Description string   `json:"description"`
	Command     string   `json:"command,omitempty"`
	State       string   `json:"state,omitempty"`
	Source      string   `json:"source,omitempty"`
	Boundary    []string `json:"boundary,omitempty"`
}

type MissionCommanderDriverRequest struct {
	Kind              string                                   `json:"kind"`
	RunLoopStepID     string                                   `json:"runLoopStepId"`
	Actor             string                                   `json:"actor,omitempty"`
	State             string                                   `json:"state,omitempty"`
	Source            string                                   `json:"source,omitempty"`
	Lane              string                                   `json:"lane,omitempty"`
	Label             string                                   `json:"label,omitempty"`
	GateEventID       string                                   `json:"gateEventId,omitempty"`
	ActionID          string                                   `json:"actionId,omitempty"`
	Command           string                                   `json:"command,omitempty"`
	Guidance          string                                   `json:"guidance,omitempty"`
	CommandExecutable bool                                     `json:"commandExecutable"`
	Blocked           bool                                     `json:"blocked,omitempty"`
	RequiresReview    bool                                     `json:"requiresReview,omitempty"`
	ExpectedReceipt   MissionCommanderDriverReceiptExpectation `json:"expectedReceipt"`
	Boundary          []string                                 `json:"boundary,omitempty"`
}

type MissionCommanderDriverReceiptExpectation struct {
	State                string   `json:"state"`
	Command              string   `json:"command,omitempty"`
	RefreshStatusCommand string   `json:"refreshStatusCommand,omitempty"`
	Description          string   `json:"description,omitempty"`
	Boundary             []string `json:"boundary,omitempty"`
}

type MissionCommanderDriverReceipt struct {
	SchemaVersion                 int                            `json:"schemaVersion"`
	State                         string                         `json:"state"`
	Outcome                       string                         `json:"outcome"`
	RunID                         string                         `json:"runId,omitempty"`
	BatchID                       string                         `json:"batchId,omitempty"`
	Lane                          string                         `json:"lane,omitempty"`
	Command                       string                         `json:"command,omitempty"`
	RunStatusPath                 string                         `json:"runStatusPath,omitempty"`
	RunDigestPath                 string                         `json:"runDigestPath,omitempty"`
	RefreshedActionQueueSummary   string                         `json:"refreshedActionQueueSummary,omitempty"`
	RefreshedCurrentRunLoopStep   string                         `json:"refreshedCurrentRunLoopStep,omitempty"`
	RefreshedCurrentDriverRequest *MissionCommanderDriverRequest `json:"refreshedCurrentDriverRequest,omitempty"`
	Boundary                      []string                       `json:"boundary,omitempty"`
}

type CurrentLoopReviewerAgentToolRequest struct {
	Tool           string `json:"tool"`
	AgentType      string `json:"agentType"`
	ReadOnly       bool   `json:"readOnly"`
	Prompt         string `json:"prompt"`
	PromptPath     string `json:"promptPath,omitempty"`
	PromptSHA256   string `json:"promptSha256,omitempty"`
	ExpectedOutput string `json:"expectedOutput"`
}

type CurrentLoopObservationAlternative struct {
	Kind                        string   `json:"kind"`
	RequiredFlags               []string `json:"requiredFlags"`
	PreviewCommandTemplate      string   `json:"previewCommandTemplate"`
	ObservationEnvelopeTemplate string   `json:"observationEnvelopeTemplate,omitempty"`
	ObservationPathCommand      string   `json:"observationPathCommand,omitempty"`
	Transition                  string   `json:"transition,omitempty"`
	Constraints                 []string `json:"constraints"`
}

type CurrentLoopObservationContract struct {
	Alternatives []CurrentLoopObservationAlternative `json:"alternatives"`
	Boundary     []string                            `json:"boundary"`
}

type CurrentLoopObservationReceipt struct {
	State                     string   `json:"state"`
	SourceCheckpointSHA256    string   `json:"sourceCheckpointSha256"`
	SuccessorCheckpointSHA256 string   `json:"successorCheckpointSha256"`
	ObservationPath           string   `json:"observationPath"`
	ObservationSHA256         string   `json:"observationSha256"`
	ObservationKind           string   `json:"observationKind"`
	Actor                     string   `json:"actor"`
	Boundary                  []string `json:"boundary"`
}

type CurrentLoopObservationInboxCandidate struct {
	Path            string `json:"path"`
	SHA256          string `json:"sha256"`
	ObservationKind string `json:"observationKind"`
	Actor           string `json:"actor"`
}

type CurrentLoopObservationInbox struct {
	State                 string                                `json:"state"`
	Path                  string                                `json:"path"`
	CandidateCount        int                                   `json:"candidateCount"`
	MatchingCount         int                                   `json:"matchingCount"`
	StaleCount            int                                   `json:"staleCount"`
	InvalidCount          int                                   `json:"invalidCount"`
	SelectedCandidate     *CurrentLoopObservationInboxCandidate `json:"selectedCandidate,omitempty"`
	SelectedDriverRequest *MissionCommanderDriverRequest        `json:"selectedDriverRequest,omitempty"`
	LatestReceipt         *CurrentLoopObservationReceipt        `json:"latestReceipt,omitempty"`
	Warnings              []string                              `json:"warnings,omitempty"`
	Boundary              []string                              `json:"boundary"`
}

type CurrentLoopReviewerAttemptIdentity struct {
	PacketID          string `json:"packetId"`
	PacketPath        string `json:"packetPath"`
	RouteID           string `json:"routeId"`
	ShardID           string `json:"shardId"`
	Lane              string `json:"lane"`
	PromptPath        string `json:"promptPath"`
	PromptSHA256      string `json:"promptSha256"`
	OwnerExecutor     string `json:"ownerExecutor"`
	OwnerGeneration   int    `json:"ownerGeneration"`
	OwnerBindingMode  string `json:"ownerBindingMode"`
	CurrentExecutor   string `json:"currentExecutor"`
	CurrentGeneration int    `json:"currentGeneration"`
}

type CurrentLoopReviewerAttemptReceipt struct {
	DispatchID              string `json:"dispatchId,omitempty"`
	DispatchPath            string `json:"dispatchPath,omitempty"`
	DispatchSHA256          string `json:"dispatchSha256,omitempty"`
	Harness                 string `json:"harness,omitempty"`
	Session                 string `json:"session,omitempty"`
	CompletionPath          string `json:"completionPath,omitempty"`
	CompletionSHA256        string `json:"completionSha256,omitempty"`
	CompletionOutcome       string `json:"completionOutcome,omitempty"`
	CompletionExitStatus    string `json:"completionExitStatus,omitempty"`
	SessionLifecycleState   string `json:"sessionLifecycleState,omitempty"`
	SessionLifecycleFailure string `json:"sessionLifecycleFailure,omitempty"`
}

type CurrentLoopReviewerAttemptAction struct {
	Kind                string                               `json:"kind"`
	Actor               string                               `json:"actor"`
	Description         string                               `json:"description"`
	RequiredInputs      []string                             `json:"requiredInputs"`
	ObservationContract CurrentLoopObservationContract       `json:"observationContract"`
	AgentToolRequest    *CurrentLoopReviewerAgentToolRequest `json:"agentToolRequest,omitempty"`
}

type CurrentLoopReviewerAttempt struct {
	SchemaVersion                    int                                `json:"schemaVersion"`
	AttemptID                        string                             `json:"attemptId"`
	AttemptSnapshotSHA256            string                             `json:"attemptSnapshotSha256"`
	State                            string                             `json:"state"`
	RunLoopStepID                    string                             `json:"runLoopStepId"`
	Identity                         CurrentLoopReviewerAttemptIdentity `json:"identity"`
	Receipt                          CurrentLoopReviewerAttemptReceipt  `json:"receipt"`
	SelectedAction                   CurrentLoopReviewerAttemptAction   `json:"selectedAction"`
	CurrentReviewerDriverRequest     *MissionCommanderDriverRequest     `json:"currentReviewerDriverRequest,omitempty"`
	DurableContinuationDriverRequest *MissionCommanderDriverRequest     `json:"durableContinuationDriverRequest,omitempty"`
	RefreshStatusCommand             string                             `json:"refreshStatusCommand,omitempty"`
	ReviewerResultDropPath           string                             `json:"reviewerResultDropPath,omitempty"`
	ReviewerResultDropPathRole       string                             `json:"reviewerResultDropPathRole,omitempty"`
	ReviewerResultInputPath          string                             `json:"reviewerResultInputPath,omitempty"`
	ReviewerResultSourcePath         string                             `json:"reviewerResultSourcePath,omitempty"`
	ReviewerResultCandidatePath      string                             `json:"reviewerResultCandidatePath,omitempty"`
	ReviewerResultPath               string                             `json:"reviewerResultPath,omitempty"`
	CompletionCriteria               []string                           `json:"completionCriteria"`
	Boundary                         []string                           `json:"boundary"`
}

type CurrentLoopReviewerWave struct {
	SnapshotSHA256 string                        `json:"snapshotSha256"`
	PacketID       string                        `json:"packetId"`
	PacketPath     string                        `json:"packetPath"`
	RouteID        string                        `json:"routeId,omitempty"`
	Lane           string                        `json:"lane"`
	MaxParallel    int                           `json:"maxParallel"`
	TotalShards    int                           `json:"totalShards"`
	ActiveSlots    int                           `json:"activeSlots"`
	AvailableSlots int                           `json:"availableSlots"`
	SpawnWave      []*CurrentLoopReviewerAttempt `json:"spawnWave,omitempty"`
	Active         []*CurrentLoopReviewerAttempt `json:"active,omitempty"`
	Returned       []*CurrentLoopReviewerAttempt `json:"returned,omitempty"`
	Failed         []*CurrentLoopReviewerAttempt `json:"failed,omitempty"`
	Blocked        []*CurrentLoopReviewerAttempt `json:"blocked,omitempty"`
	Complete       []*CurrentLoopReviewerAttempt `json:"complete,omitempty"`
	Shards         []*CurrentLoopReviewerAttempt `json:"shards"`
	Boundary       []string                      `json:"boundary"`
}

type CurrentLoopExternalReviewerHandoff struct {
	Attempt                       *CurrentLoopReviewerAttempt          `json:"attempt,omitempty"`
	Wave                          *CurrentLoopReviewerWave             `json:"wave,omitempty"`
	State                         string                               `json:"state"`
	RunLoopStepID                 string                               `json:"runLoopStepId"`
	RequiredInputs                []string                             `json:"requiredInputs"`
	ObservationContract           CurrentLoopObservationContract       `json:"observationContract"`
	AgentToolRequest              *CurrentLoopReviewerAgentToolRequest `json:"agentToolRequest,omitempty"`
	DispatchPromptPath            string                               `json:"dispatchPromptPath,omitempty"`
	DispatchPromptSHA256          string                               `json:"dispatchPromptSha256,omitempty"`
	ReviewerResultDropPath        string                               `json:"reviewerResultDropPath,omitempty"`
	ReviewerResultDropPathRole    string                               `json:"reviewerResultDropPathRole,omitempty"`
	ReviewerResultInputPath       string                               `json:"reviewerResultInputPath,omitempty"`
	ReviewerResultSourcePath      string                               `json:"reviewerResultSourcePath,omitempty"`
	RecordDispatchPreviewTemplate string                               `json:"recordDispatchPreviewTemplate,omitempty"`
	Boundary                      []string                             `json:"boundary"`
}

type CurrentLoopExternalMemberHandoff struct {
	State               string                         `json:"state"`
	AttemptID           string                         `json:"attemptId"`
	Lane                string                         `json:"lane"`
	Executor            string                         `json:"executor"`
	ExecutorGeneration  int                            `json:"executorGeneration"`
	HandoffPath         string                         `json:"handoffPath"`
	ManifestPath        string                         `json:"manifestPath"`
	OutputsRoot         string                         `json:"outputsRoot"`
	NextSteps           []string                       `json:"nextSteps"`
	ObservationContract CurrentLoopObservationContract `json:"observationContract"`
	Boundary            []string                       `json:"boundary"`
}

type CurrentLoopOperatorPackage struct {
	Ready                      bool                                `json:"ready"`
	State                      string                              `json:"state"`
	CaseRoot                   string                              `json:"caseRoot"`
	Pack                       string                              `json:"pack"`
	Route                      string                              `json:"route"`
	Lane                       string                              `json:"lane,omitempty"`
	DefaultMaxSteps            int                                 `json:"defaultMaxSteps,omitempty"`
	RemainingMaxSteps          int                                 `json:"remainingMaxSteps,omitempty"`
	SourceCurrentDriverRequest *MissionCommanderDriverRequest      `json:"sourceCurrentDriverRequest,omitempty"`
	SelectedDriverRequest      *MissionCommanderDriverRequest      `json:"selectedDriverRequest,omitempty"`
	StartDriverRequest         *MissionCommanderDriverRequest      `json:"startDriverRequest,omitempty"`
	ResumeDriverRequest        *MissionCommanderDriverRequest      `json:"resumeDriverRequest,omitempty"`
	ObservationInbox           *CurrentLoopObservationInbox        `json:"observationInbox,omitempty"`
	ObservationReceipt         *CurrentLoopObservationReceipt      `json:"observationReceipt,omitempty"`
	ExternalMemberHandoff      *CurrentLoopExternalMemberHandoff   `json:"externalMemberHandoff,omitempty"`
	ExternalReviewerHandoff    *CurrentLoopExternalReviewerHandoff `json:"externalReviewerHandoff,omitempty"`
	RunbookSteps               []string                            `json:"runbookSteps"`
	CompletionCriteria         []string                            `json:"completionCriteria"`
	Boundary                   []string                            `json:"boundary"`
}

type MissionCommanderActionQueue struct {
	Summary               string                            `json:"summary"`
	Counts                MissionCommanderActionQueueCounts `json:"counts"`
	CurrentAction         *MissionCommanderNextActionItem   `json:"currentAction,omitempty"`
	CurrentRunLoopStepID  string                            `json:"currentRunLoopStepId,omitempty"`
	CurrentActionRunLoop  []MissionCommanderRunLoopStep     `json:"currentActionRunLoop,omitempty"`
	CurrentDriverRequest  *MissionCommanderDriverRequest    `json:"currentDriverRequest,omitempty"`
	UnblockedActions      []MissionCommanderNextActionItem  `json:"unblockedActions,omitempty"`
	BlockedActions        []MissionCommanderNextActionItem  `json:"blockedActions,omitempty"`
	ReviewRequiredActions []MissionCommanderNextActionItem  `json:"reviewRequiredActions,omitempty"`
	FollowUpActions       []MissionCommanderNextActionItem  `json:"followUpActions,omitempty"`
}

type MissionCommanderActionQueueCounts struct {
	Total          int `json:"total"`
	Unblocked      int `json:"unblocked"`
	Blocked        int `json:"blocked"`
	RequiresReview int `json:"requiresReview"`
	FollowUp       int `json:"followUp"`
}

type ExecutionEvidenceReviewItem struct {
	Lane                           string                                  `json:"lane,omitempty"`
	EventID                        string                                  `json:"eventId,omitempty"`
	GateEventID                    string                                  `json:"gateEventId,omitempty"`
	Subject                        string                                  `json:"subject,omitempty"`
	Summary                        string                                  `json:"summary,omitempty"`
	Status                         string                                  `json:"status,omitempty"`
	Action                         string                                  `json:"action,omitempty"`
	Target                         string                                  `json:"target,omitempty"`
	OutputRefs                     []string                                `json:"outputRefs,omitempty"`
	EvidenceRefs                   []string                                `json:"evidenceRefs,omitempty"`
	ExecutionReportPath            string                                  `json:"executionReportPath,omitempty"`
	ExecutionReportSHA256          string                                  `json:"executionReportSha256,omitempty"`
	AdapterExecutionDispatchID     string                                  `json:"adapterExecutionDispatchId,omitempty"`
	AdapterExecutionDispatchPath   string                                  `json:"adapterExecutionDispatchPath,omitempty"`
	AdapterExecutionDispatchSHA256 string                                  `json:"adapterExecutionDispatchSha256,omitempty"`
	AdapterExecutionReceiptPath    string                                  `json:"adapterExecutionReceiptPath,omitempty"`
	AdapterExecutionReceiptSHA256  string                                  `json:"adapterExecutionReceiptSha256,omitempty"`
	CurrentExecutor                string                                  `json:"currentExecutor,omitempty"`
	ExecutorGeneration             int                                     `json:"executorGeneration,omitempty"`
	AdapterHarness                 string                                  `json:"adapterHarness,omitempty"`
	AdapterSession                 string                                  `json:"adapterSession,omitempty"`
	ToolingCatalogSHA256           string                                  `json:"toolingCatalogSha256,omitempty"`
	AdapterExecutionArtifactCount  int                                     `json:"adapterExecutionArtifactCount,omitempty"`
	ActualBudget                   *ExecutionEvidenceBudget                `json:"actualBudget,omitempty"`
	AdapterID                      string                                  `json:"adapterId,omitempty"`
	AdapterStatus                  string                                  `json:"adapterStatus,omitempty"`
	AdapterContext                 *ExecutionEvidenceAdapterContext        `json:"adapterContext,omitempty"`
	BoundaryHits                   []string                                `json:"boundaryHits,omitempty"`
	Escalation                     string                                  `json:"escalation,omitempty"`
	Acknowledgement                *ExecutionEvidenceReviewAcknowledgement `json:"acknowledgement,omitempty"`
	FollowThrough                  ExecutionEvidenceFollowThrough          `json:"followThrough"`
	ReviewCommand                  string                                  `json:"reviewCommand"`
	HandoffCommand                 string                                  `json:"handoffCommand"`
	ReviewRunbookSteps             []string                                `json:"reviewRunbookSteps,omitempty"`
	Boundary                       []string                                `json:"boundary"`
	MissionCommanderAction         MissionCommanderAction                  `json:"missionCommanderAction"`
}

type ExecutionEvidenceReviewAcknowledgement struct {
	State                        string   `json:"state"`
	AcknowledgementReviewCommand string   `json:"acknowledgementReviewCommand,omitempty"`
	AcceptedPreviewCommand       string   `json:"acceptedPreviewCommand,omitempty"`
	RejectedPreviewCommand       string   `json:"rejectedPreviewCommand,omitempty"`
	RecordCommand                string   `json:"recordCommand,omitempty"`
	Related                      []string `json:"related,omitempty"`
	EvidenceRefs                 []string `json:"evidenceRefs,omitempty"`
	Boundary                     []string `json:"boundary,omitempty"`
}

type ExecutionEvidenceAdapterContext struct {
	ID                  string   `json:"id,omitempty"`
	Status              string   `json:"status,omitempty"`
	Entry               string   `json:"entry,omitempty"`
	Purpose             string   `json:"purpose,omitempty"`
	SideEffects         []string `json:"sideEffects,omitempty"`
	GateActions         []string `json:"gateActions,omitempty"`
	ToolingCatalogPath  string   `json:"toolingCatalogPath,omitempty"`
	ReportGuidance      []string `json:"reportGuidance,omitempty"`
	EvidenceGuidance    []string `json:"evidenceGuidance,omitempty"`
	StopConditionHints  []string `json:"stopConditionHints,omitempty"`
	RecordOnlyAfterGate bool     `json:"recordOnlyAfterGate"`
}

type ExecutionEvidenceReviewSummary struct {
	Total                       int                              `json:"total"`
	ReadyForReviewCount         int                              `json:"readyForReviewCount"`
	MainEscalationCount         int                              `json:"mainEscalationCount"`
	DuplicateCount              int                              `json:"duplicateCount"`
	OutputRefCount              int                              `json:"outputRefCount"`
	EvidenceRefCount            int                              `json:"evidenceRefCount"`
	BoundaryHitCount            int                              `json:"boundaryHitCount"`
	HasEscalation               bool                             `json:"hasEscalation"`
	HasExecutionReport          bool                             `json:"hasExecutionReport"`
	HasAdapter                  bool                             `json:"hasAdapter"`
	LatestEventID               string                           `json:"latestEventId,omitempty"`
	LatestGateEventID           string                           `json:"latestGateEventId,omitempty"`
	LatestStatus                string                           `json:"latestStatus,omitempty"`
	LatestAction                string                           `json:"latestAction,omitempty"`
	LatestTarget                string                           `json:"latestTarget,omitempty"`
	LatestReviewCommand         string                           `json:"latestReviewCommand,omitempty"`
	LatestHandoffCommand        string                           `json:"latestHandoffCommand,omitempty"`
	LatestCommanderState        string                           `json:"latestCommanderState,omitempty"`
	LatestCommanderPrimary      string                           `json:"latestCommanderPrimary,omitempty"`
	LatestExecutionReportPath   string                           `json:"latestExecutionReportPath,omitempty"`
	LatestExecutionReportSHA256 string                           `json:"latestExecutionReportSha256,omitempty"`
	LatestAdapterID             string                           `json:"latestAdapterId,omitempty"`
	LatestAdapterStatus         string                           `json:"latestAdapterStatus,omitempty"`
	LatestAdapterContext        *ExecutionEvidenceAdapterContext `json:"latestAdapterContext,omitempty"`
	LatestBoundaryHits          []string                         `json:"latestBoundaryHits,omitempty"`
	LatestEscalation            string                           `json:"latestEscalation,omitempty"`
	OutcomeCount                int                              `json:"outcomeCount"`
	FollowThroughState          string                           `json:"followThroughState,omitempty"`
	ActionQueueSummary          string                           `json:"actionQueueSummary,omitempty"`
	CurrentAction               string                           `json:"currentAction,omitempty"`
	NextActionCount             int                              `json:"nextActionCount"`
	ReviewRequiredActionCount   int                              `json:"reviewRequiredActionCount"`
	Boundary                    []string                         `json:"boundary,omitempty"`
}

type ExecutionEvidenceBudget struct {
	RuntimeSeconds int `json:"runtimeSeconds"`
	DiskMB         int `json:"diskMB"`
	Requests       int `json:"requests"`
}

type ExecutionEvidenceFollowThrough struct {
	State       string                      `json:"state"`
	GateEventID string                      `json:"gateEventId"`
	Outcomes    []ExecutionEvidenceOutcome  `json:"outcomes"`
	Boundary    []string                    `json:"boundary"`
	ActionQueue MissionCommanderActionQueue `json:"actionQueue"`
}

type ExecutionEvidenceOutcome struct {
	Name                 string   `json:"name"`
	State                string   `json:"state"`
	When                 string   `json:"when"`
	Command              string   `json:"command,omitempty"`
	Actions              []string `json:"actions,omitempty"`
	VerificationCommands []string `json:"verificationCommands,omitempty"`
	Expected             string   `json:"expected"`
	Evidence             []string `json:"evidence,omitempty"`
	Boundary             []string `json:"boundary,omitempty"`
}

type LaneExecutorActionSnapshot struct {
	Lane               string         `json:"lane"`
	Label              string         `json:"label"`
	Status             string         `json:"status"`
	Workspace          string         `json:"workspace,omitempty"`
	CurrentExecutor    string         `json:"currentExecutor,omitempty"`
	ExecutorGeneration int            `json:"executorGeneration,omitempty"`
	LastTakeoverAt     string         `json:"lastTakeoverAt,omitempty"`
	LastTakeoverBy     string         `json:"lastTakeoverBy,omitempty"`
	LastTakeoverReason string         `json:"lastTakeoverReason,omitempty"`
	ExecutorAction     ExecutorAction `json:"executorAction"`
}

func Build(lanes []Lane, facts Facts, maxRows int) Brief {
	return BuildWithOptions(lanes, facts, BuildOptions{MaxRows: maxRows})
}

func BuildWithOptions(lanes []Lane, facts Facts, opts BuildOptions) Brief {
	maxRows := opts.MaxRows
	if maxRows <= 0 {
		maxRows = DefaultMaxRows
	}
	open := OpenLanes(lanes)
	blocked := map[string][]string{}
	pendingGateLines := []string{}
	authorizedGateLines := []string{}
	for _, gate := range facts.Requests {
		if IsPendingGateRequest(gate) {
			lane := Value(gate, "lane")
			if lane != "" {
				blocked[lane] = append(blocked[lane], "pending-gate")
			}
			pendingGateLines = append(pendingGateLines, GateLine(gate))
			continue
		}
		if IsAuthorizedGateRequest(gate) {
			authorizedGateLines = append(authorizedGateLines, GateLine(gate))
		}
	}
	interventionLines := []string{}
	for _, item := range EffectiveOpenInterventions(facts.Interventions) {
		lane := Value(item, "lane")
		if lane != "" {
			blocked[lane] = append(blocked[lane], "intervention")
		}
		interventionLines = append(interventionLines, InterventionLine(item))
	}
	openDecisionItems := OpenDecisionItems(facts)
	openDecisionCount := len(openDecisionItems)
	openDecisions := OpenDecisionLines(facts)
	for _, lane := range OpenDecisionLanes(facts) {
		blocked[lane] = append(blocked[lane], "open-decision")
	}
	pendingGateCount := len(pendingGateLines)
	authorizedGateCount := len(authorizedGateLines)
	interventionCount := len(interventionLines)
	pendingGateLines = LimitStrings(pendingGateLines, maxRows)
	authorizedGateLines = LimitStrings(authorizedGateLines, maxRows)
	interventionLines = LimitStrings(interventionLines, maxRows)
	openDecisions = LimitStrings(openDecisions, maxRows)
	readyLanes := []string{}
	blockedLanes := []string{}
	for _, lane := range open {
		label := FirstText(lane.Label, lane.ID)
		if reasons := UniqueStrings(blocked[lane.ID]); len(reasons) > 0 {
			blockedLanes = append(blockedLanes, fmt.Sprintf("%s (%s)", label, strings.Join(reasons, ",")))
		} else {
			readyLanes = append(readyLanes, label)
		}
	}
	nextActions := NextActionsWithOptions(readyLanes, pendingGateLines, interventionLines, openDecisions, opts)
	escalations := Escalations(pendingGateLines, interventionLines, openDecisions)
	return Brief{
		Summary:          fmt.Sprintf("openLanes=%d ready=%d blocked=%d pendingGates=%d authorizedGates=%d openDecisions=%d interventions=%d", len(open), len(readyLanes), len(blockedLanes), pendingGateCount, authorizedGateCount, openDecisionCount, interventionCount),
		ReadyLanes:       readyLanes,
		BlockedLanes:     blockedLanes,
		PendingGates:     pendingGateLines,
		AuthorizedGates:  authorizedGateLines,
		OpenDecisions:    openDecisions,
		Interventions:    interventionLines,
		NextAgentActions: nextActions,
		Escalations:      escalations,
	}
}

func MissionCommanderNextActions(actions []LaneExecutorActionSnapshot, evidenceReview []ExecutionEvidenceReviewItem, blocked bool) []MissionCommanderNextActionItem {
	items := []MissionCommanderNextActionItem{}
	evidenceNeedsMainReview := ExecutionEvidenceReviewNeedsMainReview(evidenceReview)
	for _, item := range evidenceReview {
		reasons := []string{}
		if item.GateEventID != "" {
			reasons = append(reasons, "review execution evidence for gateEventId "+item.GateEventID)
		}
		if item.ReviewCommand != "" {
			reasons = append(reasons, item.ReviewCommand)
		}
		if evidenceNeedsMainReview {
			reasons = append(reasons, "boundary hit or escalation in execution evidence; stop autonomous continuation and notify main Agent")
		}
		if item.MissionCommanderAction.PrimaryCommand != "" {
			items = append(items, MissionCommanderNextActionItem{
				Lane:           evidenceReviewLane(item),
				Label:          evidenceReviewLabel(item),
				GateEventID:    item.GateEventID,
				State:          item.MissionCommanderAction.State,
				Command:        item.MissionCommanderAction.PrimaryCommand,
				Source:         "executionEvidenceReview",
				Blocked:        evidenceNeedsMainReview,
				RequiresReview: true,
				Reasons:        reasons,
				Boundary:       append([]string{}, item.MissionCommanderAction.Boundary...),
			})
		}
		for _, followUp := range item.MissionCommanderAction.FollowUpCommands {
			if strings.Contains(followUp, "/rekit continue") && (blocked || evidenceNeedsMainReview) {
				continue
			}
			items = append(items, MissionCommanderNextActionItem{
				Lane:           evidenceReviewLane(item),
				Label:          evidenceReviewLabel(item),
				GateEventID:    item.GateEventID,
				State:          item.MissionCommanderAction.State,
				Command:        followUp,
				Source:         "executionEvidenceReview.followUp",
				Blocked:        evidenceNeedsMainReview,
				RequiresReview: true,
				Reasons:        reasons,
				Boundary:       append([]string{}, item.MissionCommanderAction.Boundary...),
			})
		}
	}
	if evidenceNeedsMainReview {
		return UniqueCommanderNextActions(items)
	}
	for _, item := range actions {
		action := item.ExecutorAction.MissionCommanderAction
		if action.PrimaryCommand == "" {
			continue
		}
		blocked := item.ExecutorAction.Blocked
		requiresReview := item.ExecutorAction.Blocked
		if (action.State == "needs-reconcile" || action.State == "needs-gate-decision" || action.State == "needs-open-decision-review") && strings.Contains(action.PrimaryCommand, " -WhatIf") {
			blocked = false
			requiresReview = true
		}
		items = append(items, MissionCommanderNextActionItem{
			Lane:           item.Lane,
			Label:          item.Label,
			State:          action.State,
			Command:        action.PrimaryCommand,
			Source:         "missionCommanderActions",
			Blocked:        blocked,
			RequiresReview: requiresReview,
			Reasons:        commanderActionReasons(item.ExecutorAction),
			Boundary:       append([]string{}, action.Boundary...),
		})
	}
	for _, item := range actions {
		action := item.ExecutorAction.MissionCommanderAction
		for _, followUp := range action.FollowUpCommands {
			items = append(items, MissionCommanderNextActionItem{
				Lane:           item.Lane,
				Label:          item.Label,
				State:          action.State,
				Command:        followUp,
				Source:         "missionCommanderActions.followUp",
				Blocked:        item.ExecutorAction.Blocked,
				RequiresReview: item.ExecutorAction.Blocked,
				Reasons:        commanderActionFollowUpReasons(item.ExecutorAction, followUp),
				Boundary:       append([]string{}, action.Boundary...),
			})
		}
	}
	return UniqueCommanderNextActions(items)
}

func ExecutionEvidenceReviewNeedsMainReview(items []ExecutionEvidenceReviewItem) bool {
	return slices.ContainsFunc(items, ExecutionEvidenceReviewItemNeedsMainReview)
}

func ExecutionEvidenceReviewItemNeedsMainReview(item ExecutionEvidenceReviewItem) bool {
	return item.Status == "boundary-hit" || item.Status == "escalated" || item.Escalation != "" || len(item.BoundaryHits) > 0
}

func ExecutionEvidenceReviewSummaryFor(items []ExecutionEvidenceReviewItem, queue MissionCommanderActionQueue) ExecutionEvidenceReviewSummary {
	summary := ExecutionEvidenceReviewSummary{}
	for _, item := range items {
		summary.Total++
		if ExecutionEvidenceReviewItemNeedsMainReview(item) {
			summary.MainEscalationCount++
		} else {
			summary.ReadyForReviewCount++
		}
		if item.MissionCommanderAction.State == "evidence-already-recorded" {
			summary.DuplicateCount++
		}
		summary.OutputRefCount += len(item.OutputRefs)
		summary.EvidenceRefCount += len(item.EvidenceRefs)
		summary.BoundaryHitCount += len(item.BoundaryHits)
		if strings.TrimSpace(item.Escalation) != "" {
			summary.HasEscalation = true
		}
		if strings.TrimSpace(item.ExecutionReportPath) != "" {
			summary.HasExecutionReport = true
		}
		if strings.TrimSpace(item.AdapterID) != "" || strings.TrimSpace(item.AdapterStatus) != "" || item.AdapterContext != nil {
			summary.HasAdapter = true
		}
		summary.OutcomeCount += len(item.FollowThrough.Outcomes)
	}
	if len(items) > 0 {
		latest := items[len(items)-1]
		summary.LatestEventID = latest.EventID
		summary.LatestGateEventID = latest.GateEventID
		summary.LatestStatus = latest.Status
		summary.LatestAction = latest.Action
		summary.LatestTarget = latest.Target
		summary.LatestReviewCommand = latest.ReviewCommand
		summary.LatestHandoffCommand = latest.HandoffCommand
		summary.LatestCommanderState = latest.MissionCommanderAction.State
		summary.LatestCommanderPrimary = latest.MissionCommanderAction.PrimaryCommand
		summary.LatestExecutionReportPath = latest.ExecutionReportPath
		summary.LatestExecutionReportSHA256 = latest.ExecutionReportSHA256
		summary.LatestAdapterID = latest.AdapterID
		summary.LatestAdapterStatus = latest.AdapterStatus
		summary.LatestAdapterContext = cloneExecutionEvidenceAdapterContext(latest.AdapterContext)
		summary.LatestBoundaryHits = LimitStrings(latest.BoundaryHits, DefaultMaxRows)
		summary.LatestEscalation = latest.Escalation
		summary.FollowThroughState = latest.FollowThrough.State
		summary.Boundary = executionEvidenceReviewSummaryBoundary()
	}
	if len(items) > 0 {
		summary.ActionQueueSummary = queue.Summary
		if queue.CurrentAction != nil {
			summary.CurrentAction = queue.CurrentAction.Command
		}
		summary.NextActionCount = queue.Counts.Total
		summary.ReviewRequiredActionCount = queue.Counts.RequiresReview
	}
	return summary
}

func executionEvidenceReviewSummaryBoundary() []string {
	return []string{
		"execution evidence review summary is read-only; full executionEvidenceReview remains available",
		"observation evidence is already recorded; do not replay heavy tool",
		"review outputRefs/evidenceRefs before any authority/confirmed outcome",
		"no authority/confirmed writes",
	}
}

func cloneExecutionEvidenceAdapterContext(context *ExecutionEvidenceAdapterContext) *ExecutionEvidenceAdapterContext {
	if context == nil {
		return nil
	}
	copy := *context
	copy.SideEffects = append([]string{}, context.SideEffects...)
	copy.GateActions = append([]string{}, context.GateActions...)
	copy.ReportGuidance = append([]string{}, context.ReportGuidance...)
	copy.EvidenceGuidance = append([]string{}, context.EvidenceGuidance...)
	copy.StopConditionHints = append([]string{}, context.StopConditionHints...)
	return &copy
}

func commanderActionReasons(action ExecutorAction) []string {
	reasons := append([]string{}, action.BlockerReasons...)
	if len(reasons) == 0 && action.Ready {
		reasons = append(reasons, "ready lane primary action")
	}
	if len(reasons) == 0 {
		reasons = append(reasons, "read-only handoff")
	}
	return reasons
}

func commanderActionFollowUpReasons(action ExecutorAction, command string) []string {
	reasons := commanderActionReasons(action)
	if action.Blocked {
		reasons = append(reasons, "follow-up is available only after resolving current lane blockers")
		if strings.Contains(command, "/rekit continue") {
			reasons = append(reasons, "run as -WhatIf first; do not continue autonomously while lane remains blocked")
		}
	} else {
		reasons = append(reasons, "follow Mission Commander handoff after primary action")
	}
	return reasons
}

func evidenceReviewLabel(item ExecutionEvidenceReviewItem) string {
	if item.GateEventID != "" {
		return item.GateEventID
	}
	return FirstText(item.Subject, item.EventID, item.Summary)
}

func evidenceReviewLane(item ExecutionEvidenceReviewItem) string {
	if lane := strings.TrimSpace(item.Lane); lane != "" {
		return lane
	}
	command := strings.TrimSpace(item.HandoffCommand)
	if command == "" {
		command = strings.TrimSpace(item.MissionCommanderAction.PrimaryCommand)
	}
	if lane, ok := strings.CutPrefix(command, "/rekit handoff "); ok {
		return strings.TrimSpace(lane)
	}
	return ""
}

func UniqueCommanderNextActions(items []MissionCommanderNextActionItem) []MissionCommanderNextActionItem {
	seen := map[string]bool{}
	out := []MissionCommanderNextActionItem{}
	for _, item := range items {
		item.Command = strings.TrimSpace(item.Command)
		if item.Command == "" {
			continue
		}
		key := item.Source + "\x00" + item.Lane + "\x00" + item.GateEventID + "\x00" + item.ActionID + "\x00" + item.Command
		if seen[key] {
			continue
		}
		seen[key] = true
		out = append(out, item)
	}
	return out
}

func MissionCommanderActionQueueFor(items []MissionCommanderNextActionItem) MissionCommanderActionQueue {
	queue := MissionCommanderActionQueue{}
	for _, item := range items {
		queue.Counts.Total++
		if item.Blocked {
			queue.Counts.Blocked++
			queue.BlockedActions = append(queue.BlockedActions, item)
		} else {
			queue.Counts.Unblocked++
			queue.UnblockedActions = append(queue.UnblockedActions, item)
		}
		if item.RequiresReview {
			queue.Counts.RequiresReview++
			queue.ReviewRequiredActions = append(queue.ReviewRequiredActions, item)
		}
		if MissionCommanderNextActionIsFollowUp(item) {
			queue.Counts.FollowUp++
			queue.FollowUpActions = append(queue.FollowUpActions, item)
		}
	}
	if current, ok := firstMissionCommanderCurrentAction(items); ok {
		queue.CurrentAction = missionCommanderNextActionPtr(current)
		queue.CurrentRunLoopStepID = missionCommanderCurrentRunLoopStepID(current)
		queue.CurrentActionRunLoop = MissionCommanderCurrentActionRunLoop(current, queue.FollowUpActions)
		queue.CurrentDriverRequest = MissionCommanderCurrentDriverRequest(current, queue.CurrentRunLoopStepID, queue.CurrentActionRunLoop)
	}
	queue.Summary = MissionCommanderActionQueueSummary(queue)
	return queue
}

func MissionCommanderNextActionIsFollowUp(item MissionCommanderNextActionItem) bool {
	return strings.Contains(item.Source, ".followUp")
}

func MissionCommanderDriverRequestWithRefreshStatusCommand(request MissionCommanderDriverRequest, refreshStatusCommand string) MissionCommanderDriverRequest {
	refreshStatusCommand = strings.TrimSpace(refreshStatusCommand)
	if refreshStatusCommand == "" {
		return request
	}
	request.ExpectedReceipt.RefreshStatusCommand = refreshStatusCommand
	request.ExpectedReceipt.Boundary = UniqueStrings(append(request.ExpectedReceipt.Boundary, "after the explicit outcome, run expectedReceipt.refreshStatusCommand before choosing follow-up work"))
	return request
}

func MissionCommanderCurrentDriverRequest(current MissionCommanderNextActionItem, currentRunLoopStepID string, runLoop []MissionCommanderRunLoopStep) *MissionCommanderDriverRequest {
	currentRunLoopStepID = strings.TrimSpace(currentRunLoopStepID)
	if currentRunLoopStepID == "" {
		return nil
	}
	executable := MissionCommanderNextActionCommandExecutable(current.Command)
	request := &MissionCommanderDriverRequest{
		Kind:              missionCommanderDriverRequestKind(current, executable),
		RunLoopStepID:     currentRunLoopStepID,
		State:             strings.TrimSpace(current.State),
		Source:            strings.TrimSpace(current.Source),
		Lane:              strings.TrimSpace(current.Lane),
		Label:             strings.TrimSpace(current.Label),
		GateEventID:       strings.TrimSpace(current.GateEventID),
		ActionID:          strings.TrimSpace(current.ActionID),
		CommandExecutable: executable,
		Blocked:           current.Blocked,
		RequiresReview:    current.RequiresReview,
		Boundary: []string{
			"driver request is a read-only handoff; the Go runtime does not spawn, poll, stop, or run external sessions",
			"driver receipt must come from the explicit command/guidance outcome after refreshed durable state",
		},
		ExpectedReceipt: MissionCommanderDriverReceiptExpectation{
			State:       "refresh-required",
			Description: "rerun /rekit status or the returned command result handoff before selecting follow-up work",
			Boundary: []string{
				"do not infer completion from the driver request alone",
				"do not write authority/confirmed or execute heavy tools from this read-only request envelope",
			},
		},
	}
	if executable {
		request.Command = strings.TrimSpace(current.Command)
	} else {
		request.Guidance = strings.TrimSpace(current.Command)
	}
	for _, step := range runLoop {
		if step.StepID != currentRunLoopStepID {
			continue
		}
		request.Actor = strings.TrimSpace(step.Actor)
		request.ExpectedReceipt.Command = strings.TrimSpace(step.Command)
		request.ExpectedReceipt.Description = strings.TrimSpace(step.Description)
		request.ExpectedReceipt.Boundary = UniqueStrings(append(request.ExpectedReceipt.Boundary, step.Boundary...))
		break
	}
	if !executable && request.Guidance != "" {
		request.Boundary = append(request.Boundary, "guidance text must be reviewed by the main Agent or harness, not executed as a shell command")
	}
	if current.Blocked {
		request.Boundary = append(request.Boundary, "blocked current actions require blocker review before any autonomous continue/run")
	}
	if current.RequiresReview {
		request.Boundary = append(request.Boundary, "review-required current actions must be previewed or reviewed before any apply/follow-up")
	}
	request.Boundary = UniqueStrings(request.Boundary)
	request.ExpectedReceipt.Boundary = UniqueStrings(request.ExpectedReceipt.Boundary)
	return request
}

func missionCommanderDriverRequestKind(current MissionCommanderNextActionItem, executable bool) string {
	if current.Blocked {
		return "blocked-review"
	}
	if !executable {
		return "review-guidance"
	}
	if current.RequiresReview || strings.Contains(current.Command, " -WhatIf") {
		return "preview-command"
	}
	return "execute-command"
}

func MissionCommanderCurrentActionRunLoop(current MissionCommanderNextActionItem, followUps []MissionCommanderNextActionItem) []MissionCommanderRunLoopStep {
	steps := []MissionCommanderRunLoopStep{}
	add := func(step MissionCommanderRunLoopStep) {
		step.StepID = strings.TrimSpace(step.StepID)
		step.Actor = strings.TrimSpace(step.Actor)
		step.Description = strings.TrimSpace(step.Description)
		if step.StepID == "" || step.Description == "" {
			return
		}
		step.Order = len(steps) + 1
		step.Boundary = UniqueStrings(step.Boundary)
		steps = append(steps, step)
	}
	inspectBoundary := []string{
		"status, overview, handoff, and continue projections are read-only handoffs",
		"do not skip blocked/review-required reasons or boundary lines when choosing the next command",
	}
	if strings.TrimSpace(current.Command) != "" && !MissionCommanderNextActionCommandExecutable(current.Command) {
		inspectBoundary = append(inspectBoundary, "not every current action command is shell-executable; guidance text must be reviewed but not run as a command")
	}
	if current.Blocked {
		inspectBoundary = append(inspectBoundary, "blocked current actions must not be treated as autonomous continue/run permission")
	}
	add(MissionCommanderRunLoopStep{
		StepID:      "inspect-current",
		Actor:       "main-agent",
		Description: "inspect the selected Mission Commander current action, reasons, and boundary before running any command",
		State:       current.State,
		Source:      current.Source,
		Boundary:    inspectBoundary,
	})
	if strings.TrimSpace(current.Command) != "" && MissionCommanderNextActionCommandExecutable(current.Command) {
		stepID := "apply-or-run-current"
		description := "run the current command only after inspection confirms it is the intended next action"
		boundary := []string{
			"the main Agent or executor runs this command explicitly; the Go runtime does not auto-run queued actions",
			"do not write authority/confirmed state or execute heavy tools unless the command itself is an authorized gate-backed bounded action",
		}
		if current.Blocked || current.RequiresReview || strings.Contains(current.Command, " -WhatIf") {
			stepID = "preview-current"
			description = "run or review the current command as a bounded preview/review step before any apply or follow-up"
			boundary = append(boundary, "review preview output and returned expected hashes before any apply follow-up")
		}
		if current.Blocked {
			boundary = append(boundary, "blocked current actions must not be treated as autonomous continue/run permission")
		}
		add(MissionCommanderRunLoopStep{
			StepID:      stepID,
			Actor:       "main-agent",
			Description: description,
			Command:     current.Command,
			State:       current.State,
			Source:      current.Source,
			Boundary:    boundary,
		})
	}
	add(MissionCommanderRunLoopStep{
		StepID:      "refresh-state",
		Actor:       "main-agent",
		Description: "rerun /rekit status or the returned command result handoff and rebuild the Mission Commander action queue",
		Boundary: []string{
			"only follow-up actions whose blockers are cleared and hashes match should be applied",
			"do not assume current action completion from terminal text alone; refresh durable state first",
		},
	})
	if follow := missionCommanderFirstRelevantFollowUp(current, followUps); follow.Command != "" {
		add(MissionCommanderRunLoopStep{
			StepID:      "follow-up-after-refresh",
			Actor:       "main-agent",
			Description: "after refresh confirms the blocker/review step is closed, consider the next follow-up command",
			Command:     follow.Command,
			State:       follow.State,
			Source:      follow.Source,
			Boundary: []string{
				"follow-up commands remain candidates until refreshed state makes them current or unblocked",
				"run -WhatIf previews before any apply follow-up when the command exposes a preview/apply pair",
			},
		})
	}
	return steps
}

func missionCommanderCurrentRunLoopStepID(current MissionCommanderNextActionItem) string {
	if current.Blocked || !MissionCommanderNextActionCommandExecutable(current.Command) {
		return "inspect-current"
	}
	if current.RequiresReview || strings.Contains(current.Command, " -WhatIf") {
		return "preview-current"
	}
	return "apply-or-run-current"
}

func MissionCommanderNextActionCommandExecutable(command string) bool {
	command = strings.TrimSpace(command)
	return strings.HasPrefix(command, "/rekit") || strings.HasPrefix(command, "rekit")
}

func missionCommanderFirstRelevantFollowUp(current MissionCommanderNextActionItem, followUps []MissionCommanderNextActionItem) MissionCommanderNextActionItem {
	for _, follow := range followUps {
		if current.Lane != follow.Lane || current.Label != follow.Label || current.GateEventID != follow.GateEventID || current.ActionID != follow.ActionID || current.State != follow.State {
			continue
		}
		return follow
	}
	return MissionCommanderNextActionItem{}
}

func missionCommanderNextActionIsIdleGuidance(item MissionCommanderNextActionItem) bool {
	return strings.HasPrefix(item.Source, "releaseHandoffNextBatch")
}

func missionCommanderNextActionIsActiveProjectWork(item MissionCommanderNextActionItem) bool {
	return strings.HasPrefix(item.Source, "executionEvidenceReview") || strings.HasPrefix(item.Source, "reviewerDispatch") || strings.HasPrefix(item.Source, "reviewerPacket") || strings.HasPrefix(item.Source, "packMemoryCandidates") || strings.HasPrefix(item.Source, "adapterReport")
}

func missionCommanderNextActionIsBoundedLanePrimary(item MissionCommanderNextActionItem) bool {
	return item.Source == "missionCommanderActions" &&
		(item.State == "needs-start-apply" || item.State == "needs-reconcile")
}

func MissionCommanderActionQueueSummary(queue MissionCommanderActionQueue) string {
	current := "none"
	if queue.CurrentAction != nil {
		current = queue.CurrentAction.Command
	}
	return fmt.Sprintf("total=%d unblocked=%d blocked=%d requiresReview=%d followUp=%d current=%s", queue.Counts.Total, queue.Counts.Unblocked, queue.Counts.Blocked, queue.Counts.RequiresReview, queue.Counts.FollowUp, current)
}

func firstMissionCommanderCurrentAction(items []MissionCommanderNextActionItem) (MissionCommanderNextActionItem, bool) {
	if len(items) == 0 {
		return MissionCommanderNextActionItem{}, false
	}
	current := items[0]
	currentPriority := missionCommanderNextActionCurrentPriority(current)
	for _, item := range items[1:] {
		priority := missionCommanderNextActionCurrentPriority(item)
		if priority < currentPriority {
			current = item
			currentPriority = priority
		}
	}
	return current, true
}

func missionCommanderNextActionCurrentPriority(item MissionCommanderNextActionItem) int {
	followUp := MissionCommanderNextActionIsFollowUp(item)
	idleGuidance := missionCommanderNextActionIsIdleGuidance(item)
	activeProjectWork := missionCommanderNextActionIsActiveProjectWork(item)
	boundedLanePrimary := missionCommanderNextActionIsBoundedLanePrimary(item)
	if !followUp && !item.Blocked && boundedLanePrimary {
		return 0
	}
	if !followUp && !item.Blocked && activeProjectWork {
		return 10
	}
	if !followUp && !item.Blocked && !idleGuidance {
		return 20
	}
	if !followUp && activeProjectWork {
		return 30
	}
	if !followUp && !idleGuidance {
		return 40
	}
	if !followUp && !item.Blocked {
		return 50
	}
	if !followUp {
		return 60
	}
	if item.RequiresReview || item.Blocked {
		return 70
	}
	return 80
}

func missionCommanderNextActionPtr(item MissionCommanderNextActionItem) *MissionCommanderNextActionItem {
	copy := item
	return &copy
}

func LaneExecutorActionSnapshots(lanes []BoardLane, facts Facts, brief Brief) []LaneExecutorActionSnapshot {
	items := make([]LaneExecutorActionSnapshot, 0, len(lanes))
	for _, lane := range lanes {
		label := BoardLaneLabel(lane)
		items = append(items, LaneExecutorActionSnapshot{
			Lane:               lane.ID,
			Label:              label,
			Status:             lane.Status,
			Workspace:          lane.Workspace,
			CurrentExecutor:    lane.CurrentExecutor,
			ExecutorGeneration: lane.ExecutorGeneration,
			LastTakeoverAt:     lane.LastTakeoverAt,
			LastTakeoverBy:     lane.LastTakeoverBy,
			LastTakeoverReason: lane.LastTakeoverReason,
			ExecutorAction:     LaneExecutorAction(Lane{ID: lane.ID, Label: label, Status: lane.Status}, facts, brief),
		})
	}
	return items
}

func LaneExecutorAction(lane Lane, facts Facts, brief Brief) ExecutorAction {
	label := laneCommandLabel(lane)
	laneFacts := LaneFacts(facts, lane.ID)
	openInterventionItems := EffectiveOpenInterventions(laneFacts.Interventions)
	pendingGateItems := FilterLane(laneFacts.Requests, lane.ID, "pending-gate")
	pendingGates := len(pendingGateItems)
	openInterventions := len(openInterventionItems)
	openDecisionItems := OpenDecisionItems(laneFacts)
	openDecisions := len(openDecisionItems)
	reasons := []string{}
	if pendingGates > 0 {
		reasons = append(reasons, "pending-gate")
	}
	if openInterventions > 0 {
		reasons = append(reasons, "intervention")
	}
	if openDecisions > 0 {
		reasons = append(reasons, "open-decision")
	}
	blocked := len(reasons) > 0
	ready := !blocked && slices.Contains(brief.ReadyLanes, label)
	return ExecutorAction{
		Blocked:                blocked,
		Ready:                  ready,
		BlockerReasons:         reasons,
		PendingGates:           pendingGates,
		OpenInterventions:      openInterventions,
		OpenDecisions:          openDecisions,
		ReconcileRequired:      openInterventions > 0,
		PendingGateRequired:    pendingGates > 0,
		OpenDecisionRequired:   openDecisions > 0,
		ResumeCommand:          "/rekit continue " + label,
		HandoffCommand:         "/rekit handoff " + label,
		NextAgentActions:       LaneExecutorNextActions(label, ready, pendingGates, openInterventions, openDecisions),
		Escalations:            LaneExecutorEscalations(pendingGates, openInterventions, openDecisions),
		MissionCommanderAction: LaneMissionCommanderActionForLane(label, lane.ID, lane.Status, ready, pendingGateItems, openInterventionItems, openDecisionItems),
	}
}

func LaneExecutorNextActions(label string, ready bool, pendingGates, openInterventions, openDecisions int) []string {
	actions := []string{}
	if openInterventions > 0 {
		actions = append(actions, "reconcile open intervention(s) before continuing this lane")
	}
	if pendingGates > 0 {
		actions = append(actions, "resolve or keep deferred pending-gate request(s); gate records the request and never executes heavy-tool")
	}
	if openDecisions > 0 {
		actions = append(actions, "review open candidate/decision item(s) with evidence and authority boundary")
	}
	if len(actions) == 0 && ready {
		actions = append(actions, "/rekit continue "+strings.TrimSpace(label))
	}
	if len(actions) == 0 {
		actions = append(actions, "/rekit handoff "+strings.TrimSpace(label))
	}
	return actions
}

func LaneExecutorEscalations(pendingGates, openInterventions, openDecisions int) []string {
	escalations := []string{}
	if pendingGates > 0 {
		escalations = append(escalations, "pending-gate requires main-agent/user decision before heavy action")
	}
	if openInterventions > 0 {
		escalations = append(escalations, "open intervention must be reconciled into durable lane state")
	}
	if openDecisions > 0 {
		escalations = append(escalations, "authority/confirmed outcome remains deferred until explicitly approved")
	}
	return escalations
}

func LaneMissionCommanderAction(label, laneID, status string, ready bool, pendingGates, openInterventions, openDecisions int) MissionCommanderAction {
	pendingGateItems := make([]map[string]any, max(pendingGates, 0))
	interventionItems := make([]map[string]any, max(openInterventions, 0))
	openDecisionItems := make([]map[string]any, max(openDecisions, 0))
	return LaneMissionCommanderActionForLane(label, laneID, status, ready, pendingGateItems, interventionItems, openDecisionItems)
}

func LaneMissionCommanderActionForLane(label, laneID, status string, ready bool, pendingGateItems []map[string]any, openInterventions []map[string]any, openDecisionItems []map[string]any) MissionCommanderAction {
	label = strings.TrimSpace(label)
	if label == "" {
		label = "main"
	}
	gateLane := strings.TrimSpace(laneID)
	if gateLane == "" {
		gateLane = label
	}
	status = strings.ToLower(strings.TrimSpace(status))
	action := MissionCommanderAction{
		State:          "read-only-handoff",
		Prompt:         fmt.Sprintf("按 `%s` 接手，先阅读/刷新交接；当前不要继续执行该 lane。", label),
		PrimaryCommand: "/rekit handoff " + label,
		Boundary: []string{
			"no authority/confirmed writes",
			"no heavy-tool execution",
			"do not run continue for blocked lanes",
		},
	}
	if status == "paused" || status == "closed" || status == "archived" {
		action.State = "lane-not-open"
		action.Prompt = fmt.Sprintf("按 `%s` 接手，先阅读交接；该 lane status=%s，当前不要继续执行。", label, status)
		return action
	}
	if len(openInterventions) > 0 {
		action.State = "needs-reconcile"
		action.Prompt = fmt.Sprintf("按 `%s` 接手，先 review concrete reconcile preview，再写入 selected open intervention resolution。", label)
		if len(openInterventions) == 1 && strings.TrimSpace(Value(openInterventions[0], "eventId")) != "" {
			eventID := Value(openInterventions[0], "eventId")
			action.PrimaryCommand = "/rekit reconcile " + label + " -InterventionId " + quoteCommandArg(eventID) + " -WhatIf"
			action.FollowUpCommands = []string{"/rekit reconcile " + label + " -InterventionId " + quoteCommandArg(eventID) + " -Apply", "/rekit continue " + label + " -WhatIf", "/rekit handoff " + label}
			action.Boundary = append(action.Boundary, "review reconcile -WhatIf output before running the bounded -Apply follow-up")
			return action
		}
		action.PrimaryCommand = "/rekit handoff " + label
		action.FollowUpCommands = multiInterventionReconcilePreviewCommands(label, openInterventions)
		action.FollowUpCommands = append(action.FollowUpCommands, "/rekit continue "+label+" -WhatIf")
		action.Boundary = append(action.Boundary, "multiple or unidentified open interventions require handoff review before selecting a concrete eventId")
		return action
	}
	if len(pendingGateItems) > 0 {
		action.State = "needs-gate-decision"
		action.Prompt = fmt.Sprintf("按 `%s` 接手，先 review concrete gate preview，再写入 pending-gate decision。", label)
		gateActions := pendingGatePreviewActions(pendingGateItems)
		if len(pendingGateItems) == 1 && len(gateActions) == 1 {
			gateAction := gateActions[0]
			action.PrimaryCommand = "/rekit gate -Action " + quoteCommandArg(gateAction) + " -Lane " + gateLane + " -WhatIf"
			action.FollowUpCommands = []string{"/rekit gate -Action " + quoteCommandArg(gateAction) + " -Lane " + gateLane + " -Apply -Actor <actor>", "/rekit continue " + label + " -WhatIf", "/rekit handoff " + label}
			action.Boundary = append(action.Boundary, "review gate -WhatIf output before running the bounded -Apply follow-up")
			return action
		}
		action.PrimaryCommand = "/rekit handoff " + label
		for _, gateAction := range gateActions {
			action.FollowUpCommands = append(action.FollowUpCommands, "/rekit gate -Action "+quoteCommandArg(gateAction)+" -Lane "+gateLane+" -WhatIf")
		}
		if len(gateActions) != len(pendingGateItems) {
			action.FollowUpCommands = append(action.FollowUpCommands, "/rekit gate -Action <action> -Lane "+gateLane+" -WhatIf")
		}
		action.FollowUpCommands = append(action.FollowUpCommands, "/rekit continue "+label+" -WhatIf")
		action.Boundary = append(action.Boundary, "multiple or unidentified pending-gate requests require handoff review before selecting a concrete action")
		return action
	}
	if len(openDecisionItems) > 0 {
		action.State = "needs-open-decision-review"
		action.Prompt = fmt.Sprintf("按 `%s` 接手，先 review open candidate/decision 与 evidence/authority boundary，再决定是否继续。", label)
		if len(openDecisionItems) == 1 && strings.TrimSpace(Value(openDecisionItems[0], "lane")) != "" {
			action.PrimaryCommand = openDecisionNotePreviewCommand(openDecisionItems[0])
			action.FollowUpCommands = []string{"/rekit continue " + label + " -WhatIf", "/rekit handoff " + label}
			action.Boundary = append(action.Boundary, "review note -WhatIf output and run only the returned hash-bound recordCommand")
			return action
		}
		action.PrimaryCommand = "/rekit handoff " + label
		action.FollowUpCommands = append(openDecisionPreviewCommands(openDecisionItems), "/rekit continue "+label+" -WhatIf")
		action.Boundary = append(action.Boundary, "multiple or unidentified open decisions require handoff review before selecting a concrete candidate/decision item")
		return action
	}
	if ready {
		action.State = "ready-to-continue"
		action.Prompt = fmt.Sprintf("按 `%s` 接手，然后继续该 lane。", label)
		action.PrimaryCommand = "/rekit continue " + label
		action.FollowUpCommands = []string{"/rekit handoff " + label}
		return action
	}
	return action
}

func multiInterventionReconcilePreviewCommands(label string, openInterventions []map[string]any) []string {
	commands := []string{}
	seen := map[string]bool{}
	for _, item := range openInterventions {
		eventID := strings.TrimSpace(Value(item, "eventId"))
		if eventID == "" || seen[eventID] {
			continue
		}
		seen[eventID] = true
		commands = append(commands, "/rekit reconcile "+label+" -InterventionId "+quoteCommandArg(eventID)+" -WhatIf")
	}
	if len(commands) != len(openInterventions) {
		commands = append(commands, "/rekit reconcile "+label+" -InterventionId <eventId> -WhatIf")
	}
	return commands
}

func openDecisionPreviewCommands(openDecisionItems []map[string]any) []string {
	commands := []string{}
	seen := map[string]bool{}
	for _, item := range openDecisionItems {
		command := openDecisionNotePreviewCommand(item)
		if command == "" || seen[command] {
			continue
		}
		seen[command] = true
		commands = append(commands, command)
	}
	return commands
}

func openDecisionNotePreviewCommand(item map[string]any) string {
	lane := strings.TrimSpace(Value(item, "lane"))
	if lane == "" {
		return ""
	}
	decision := Value(item, "decision")
	if decision == "" || decision == "defer" || decision == "pending-user" {
		decision = "<accept|reject|defer|supersede>"
	}
	parts := []string{"/rekit", "note", "-Kind", "decision", "-Lane", lane}
	parts = appendCommandArg(parts, "-Subject", openDecisionNoteSubject(item))
	parts = appendCommandArg(parts, "-Summary", openDecisionNoteSummary(item))
	parts = appendCommandArg(parts, "-Decision", decision)
	parts = appendCommandArg(parts, "-Reason", FirstText(Value(item, "reason"), "reviewed open candidate/decision item"))
	parts = appendCommandArg(parts, "-TargetRef", Value(item, "target"))
	if eventID := Value(item, "eventId"); eventID != "" {
		parts = appendCommandArg(parts, "-Related", eventID)
	}
	parts = appendCommandArg(parts, "-EvidenceRefs", Value(item, "evidenceRefs"))
	parts = appendCommandArg(parts, "-BatchId", Value(item, "batchId"))
	parts = append(parts, "-WhatIf")
	return joinCommand(parts...)
}

func openDecisionNoteSubject(item map[string]any) string {
	kind := FirstText(Value(item, "kind"), "decision")
	subject := Value(item, "subject")
	if strings.TrimSpace(subject) == "" {
		subject = FirstText(Value(item, "summary"), "open item")
	}
	return "decision for " + kind + ": " + subject
}

func openDecisionNoteSummary(item map[string]any) string {
	summary := Value(item, "summary")
	if strings.TrimSpace(summary) == "" {
		summary = Value(item, "subject")
	}
	return FirstText(summary, "record reviewed open candidate/decision outcome")
}

func appendCommandArg(parts []string, flag, value string) []string {
	value = strings.TrimSpace(value)
	if value == "" {
		return parts
	}
	return append(parts, flag, value)
}

func joinCommand(parts ...string) string {
	out := append([]string{}, parts...)
	for idx := range out {
		out[idx] = quoteCommandArg(out[idx])
	}
	return strings.Join(out, " ")
}

func pendingGatePreviewActions(pendingGateItems []map[string]any) []string {
	actions := []string{}
	seen := map[string]bool{}
	for _, item := range pendingGateItems {
		gate, ok := item["gate"].(map[string]any)
		if !ok {
			continue
		}
		action := strings.TrimSpace(Value(gate, "action"))
		if action == "" || seen[action] {
			continue
		}
		seen[action] = true
		actions = append(actions, action)
	}
	return actions
}

func quoteCommandArg(value string) string {
	value = strings.TrimSpace(value)
	if value == "" {
		return ""
	}
	if strings.ContainsAny(value, " \t\r\n\"'") {
		return strconv.Quote(value)
	}
	return value
}

func laneCommandLabel(lane Lane) string {
	label := strings.TrimSpace(lane.Label)
	if label != "" {
		return label
	}
	if lane.ID == "main" {
		return "main"
	}
	if name, ok := strings.CutPrefix(lane.ID, "feature-"); ok {
		return name
	}
	return lane.ID
}

func OpenLanes(lanes []Lane) []Lane {
	open := []Lane{}
	for _, lane := range lanes {
		if isOpenLaneStatus(lane.Status) {
			open = append(open, lane)
		}
	}
	return open
}

func isOpenLaneStatus(status string) bool {
	status = strings.ToLower(strings.TrimSpace(status))
	return status != "archived" && status != "paused" && status != "closed"
}

func OpenEvents(items []map[string]any) []map[string]any {
	open := []map[string]any{}
	for _, item := range items {
		status := strings.ToLower(strings.TrimSpace(Value(item, "status")))
		if status == "" || !IsClosedStatus(status) {
			open = append(open, item)
		}
	}
	return open
}

func OpenCandidates(items []map[string]any) []map[string]any {
	open := []map[string]any{}
	for _, item := range items {
		status := strings.ToLower(strings.TrimSpace(Value(item, "status")))
		switch status {
		case "confirmed", "accepted", "rejected", "resolved", "superseded":
			continue
		default:
			open = append(open, item)
		}
	}
	return open
}

func EffectiveOpenCandidates(facts Facts) []map[string]any {
	resolved := candidateDecisionResolutionIDs(facts.Decisions)
	open := []map[string]any{}
	for _, item := range OpenCandidates(facts.Candidates) {
		eventID := strings.TrimSpace(Value(item, "eventId"))
		if eventID != "" && resolved[eventID] {
			continue
		}
		open = append(open, item)
	}
	return open
}

func candidateDecisionResolutionIDs(decisions []map[string]any) map[string]bool {
	resolved := map[string]bool{}
	for _, decision := range decisions {
		if !candidateClosingDecision(decision) {
			continue
		}
		for _, related := range stringListValue(decision["related"]) {
			resolved[related] = true
		}
	}
	return resolved
}

func candidateClosingDecision(decision map[string]any) bool {
	status := strings.ToLower(strings.TrimSpace(Value(decision, "status")))
	if status != "" && !IsTerminalStatus(status) {
		return false
	}
	switch strings.ToLower(strings.TrimSpace(FirstText(Value(decision, "decision"), Value(decision, "action")))) {
	case "accept", "reject", "supersede":
		return true
	default:
		return false
	}
}

func stringListValue(value any) []string {
	items := []string{}
	add := func(value string) {
		for _, part := range strings.FieldsFunc(value, func(r rune) bool { return r == ',' || r == ';' || r == '\n' }) {
			if part = strings.TrimSpace(part); part != "" {
				items = append(items, part)
			}
		}
	}
	switch t := value.(type) {
	case string:
		add(t)
	case []string:
		for _, item := range t {
			add(item)
		}
	case []any:
		for _, item := range t {
			add(fmt.Sprint(item))
		}
	default:
		if value != nil {
			add(fmt.Sprint(value))
		}
	}
	return UniqueStrings(items)
}

func OpenDecisionEvents(decisions []map[string]any) []map[string]any {
	open := []map[string]any{}
	for _, decision := range decisions {
		status := strings.ToLower(strings.TrimSpace(Value(decision, "status")))
		decisionValue := strings.ToLower(strings.TrimSpace(FirstText(Value(decision, "decision"), Value(decision, "action"))))
		if (status == "" && decisionValue == "defer") || (status != "" && !IsTerminalStatus(status)) || decisionValue == "pending-user" {
			open = append(open, decision)
		}
	}
	return open
}

func OpenDecisionLanes(facts Facts) []string {
	lanes := []string{}
	for _, candidate := range EffectiveOpenCandidates(facts) {
		if lane := Value(candidate, "lane"); lane != "" {
			lanes = append(lanes, lane)
		}
	}
	for _, decision := range OpenDecisionEvents(facts.Decisions) {
		if lane := Value(decision, "lane"); lane != "" {
			lanes = append(lanes, lane)
		}
	}
	return UniqueStrings(lanes)
}

func OpenDecisionItems(facts Facts) []map[string]any {
	items := []map[string]any{}
	items = append(items, EffectiveOpenCandidates(facts)...)
	items = append(items, OpenDecisionEvents(facts.Decisions)...)
	return items
}

func OpenDecisionLines(facts Facts) []string {
	lines := []string{}
	for _, item := range OpenDecisionItems(facts) {
		lines = append(lines, OpenDecisionLine(item))
	}
	return lines
}

func LaneFacts(facts Facts, laneID string) Facts {
	return Facts{
		Candidates:    FilterLane(facts.Candidates, laneID, ""),
		Requests:      FilterLane(facts.Requests, laneID, ""),
		Decisions:     FilterLane(facts.Decisions, laneID, ""),
		Interventions: FilterLane(facts.Interventions, laneID, ""),
	}
}

func FilterLane(items []map[string]any, laneID, status string) []map[string]any {
	out := []map[string]any{}
	for _, item := range items {
		if Value(item, "lane") != laneID {
			continue
		}
		if status != "" && Value(item, "status") != status {
			continue
		}
		out = append(out, item)
	}
	return out
}

func IsPendingGateRequest(item map[string]any) bool {
	return Value(item, "status") == "pending-gate"
}

func IsAuthorizedGateRequest(item map[string]any) bool {
	return Value(item, "status") == "authorized-gate"
}

func GateLine(item map[string]any) string {
	parts := []string{Subject(item)}
	AddPart(&parts, "lane", Value(item, "lane"))
	AddPart(&parts, "risk", Value(item, "risk"))
	AddPart(&parts, "target", Value(item, "target"))
	addGateParts(&parts, item)
	return strings.Join(parts, " | ")
}

func LaneGateLine(item map[string]any) string {
	parts := []string{Subject(item)}
	addGateParts(&parts, item)
	AddPart(&parts, "risk", Value(item, "risk"))
	AddPart(&parts, "target", Value(item, "target"))
	return strings.Join(parts, " | ")
}

func addGateParts(parts *[]string, item map[string]any) {
	gate, ok := item["gate"].(map[string]any)
	if !ok {
		return
	}
	AddPart(parts, "action", Value(gate, "action"))
	AddPart(parts, "scope", Value(gate, "scope"))
	AddPart(parts, "requestedBudget", budgetLine(gate["requestedBudget"]))
	AddPart(parts, "outputPaths", Value(gate, "outputPaths"))
	AddPart(parts, "stopConditions", Value(gate, "stopConditions"))
	if eventID := Value(item, "eventId"); eventID != "" && Value(item, "status") == "authorized-gate" {
		AddPart(parts, "eventId", eventID)
		AddPart(parts, "reportContract", "/rekit gate -ExecutionReportContract -GateEventId "+eventID+" -Format json")
	}
	if auth, ok := gate["authorization"].(map[string]any); ok {
		AddPart(parts, "auth", Value(auth, "decision"))
		AddPart(parts, "profile", Value(auth, "profileId"))
	}
}

func InterventionLine(item map[string]any) string {
	parts := []string{Subject(item)}
	AddPart(&parts, "lane", Value(item, "lane"))
	AddPart(&parts, "action", Value(item, "action"))
	AddPart(&parts, "status", FirstText(Value(item, "status"), "open"))
	AddPart(&parts, "target", Value(item, "target"))
	return strings.Join(parts, " | ")
}

func LaneInterventionLine(item map[string]any) string {
	parts := []string{Subject(item)}
	AddPart(&parts, "action", Value(item, "action"))
	AddPart(&parts, "status", FirstText(Value(item, "status"), "open"))
	AddPart(&parts, "target", Value(item, "target"))
	return strings.Join(parts, " | ")
}

func CandidateLine(item map[string]any) string {
	parts := []string{"candidate: " + Subject(item)}
	AddPart(&parts, "lane", Value(item, "lane"))
	AddPart(&parts, "status", Value(item, "status"))
	AddPart(&parts, "summary", Value(item, "summary"))
	return strings.Join(parts, " | ")
}

func LaneCandidateLine(item map[string]any) string {
	parts := []string{"candidate: " + Subject(item)}
	AddPart(&parts, "status", Value(item, "status"))
	AddPart(&parts, "summary", Value(item, "summary"))
	return strings.Join(parts, " | ")
}

func DecisionLine(item map[string]any) string {
	parts := []string{Subject(item)}
	AddPart(&parts, "lane", Value(item, "lane"))
	AddPart(&parts, "decision", FirstText(Value(item, "decision"), Value(item, "action")))
	AddPart(&parts, "reason", Value(item, "reason"))
	return strings.Join(parts, " | ")
}

func LaneDecisionLine(item map[string]any) string {
	parts := []string{Subject(item)}
	AddPart(&parts, "decision", FirstText(Value(item, "decision"), Value(item, "action")))
	AddPart(&parts, "reason", Value(item, "reason"))
	return strings.Join(parts, " | ")
}

func OpenDecisionLine(item map[string]any) string {
	if Value(item, "kind") == "candidate" {
		return CandidateLine(item)
	}
	return DecisionLine(item)
}

func LaneOpenDecisionLine(item map[string]any) string {
	if Value(item, "kind") == "candidate" {
		return LaneCandidateLine(item)
	}
	return LaneDecisionLine(item)
}

func NextActions(ready, gates, interventions, decisions []string, maxRows int) []string {
	return NextActionsWithOptions(ready, gates, interventions, decisions, BuildOptions{MaxRows: maxRows})
}

func NextActionsWithOptions(ready, gates, interventions, decisions []string, opts BuildOptions) []string {
	maxRows := opts.MaxRows
	if maxRows <= 0 {
		maxRows = DefaultMaxRows
	}
	actions := []string{}
	if len(interventions) > 0 {
		actions = append(actions, "reconcile open intervention(s) before continuing the affected lane")
	}
	if len(gates) > 0 {
		actions = append(actions, "resolve or keep deferred pending-gate request(s); gate records the request and never executes heavy-tool")
	}
	if len(decisions) > 0 {
		action := FirstText(opts.OpenDecisionAction, "review open candidates/decisions and record accept/reject/defer with evidence")
		actions = append(actions, action)
	}
	for _, lane := range ready {
		actions = append(actions, "/rekit continue "+lane)
	}
	if len(actions) == 0 {
		actions = append(actions, "/rekit start <name>", "/rekit handoff")
	}
	return LimitStrings(actions, maxRows)
}

func Escalations(gates, interventions, decisions []string) []string {
	escalations := []string{}
	if len(gates) > 0 {
		escalations = append(escalations, "pending-gate requires main-agent/user decision before heavy action")
	}
	if len(interventions) > 0 {
		escalations = append(escalations, "open intervention must be reconciled into durable lane state")
	}
	if len(decisions) > 0 {
		escalations = append(escalations, "authority/confirmed outcome remains deferred until explicitly approved")
	}
	return escalations
}

func Subject(item map[string]any) string {
	return FirstText(Value(item, "subject"), Value(item, "summary"), Value(item, "kind"), "item")
}

func budgetLine(value any) string {
	budget := map[string]any{}
	switch t := value.(type) {
	case map[string]any:
		budget = t
	default:
		data, err := json.Marshal(t)
		if err != nil {
			return ""
		}
		if err := json.Unmarshal(data, &budget); err != nil {
			return ""
		}
	}
	runtimeSeconds := Value(budget, "runtimeSeconds")
	diskMB := Value(budget, "diskMB")
	requests := Value(budget, "requests")
	if emptyBudgetValue(runtimeSeconds) && emptyBudgetValue(diskMB) && emptyBudgetValue(requests) {
		return ""
	}
	parts := []string{}
	AddPart(&parts, "runtimeSeconds", runtimeSeconds)
	AddPart(&parts, "diskMB", diskMB)
	AddPart(&parts, "requests", requests)
	return strings.Join(parts, ",")
}

func emptyBudgetValue(value string) bool {
	value = strings.TrimSpace(value)
	return value == "" || value == "0" || value == "0.0"
}

func AddPart(parts *[]string, key, value string) {
	if strings.TrimSpace(value) != "" {
		*parts = append(*parts, key+"="+strings.TrimSpace(value))
	}
}

func FirstText(values ...string) string {
	for _, value := range values {
		if strings.TrimSpace(value) != "" {
			return strings.TrimSpace(value)
		}
	}
	return ""
}

func Value(m map[string]any, key string) string {
	v, ok := m[key]
	if !ok || v == nil {
		return ""
	}
	switch t := v.(type) {
	case string:
		return t
	case []string:
		parts := []string{}
		for _, item := range t {
			text := strings.TrimSpace(item)
			if text != "" {
				parts = append(parts, text)
			}
		}
		return strings.Join(parts, ",")
	case []any:
		parts := []string{}
		for _, item := range t {
			text := strings.TrimSpace(fmt.Sprint(item))
			if text != "" {
				parts = append(parts, text)
			}
		}
		return strings.Join(parts, ",")
	case bool:
		if t {
			return "true"
		}
		return "false"
	default:
		return strings.TrimSpace(fmt.Sprint(t))
	}
}

func LimitStrings(items []string, n int) []string {
	if n <= 0 || len(items) <= n {
		return items
	}
	return items[len(items)-n:]
}

func UniqueStrings(items []string) []string {
	seen := map[string]bool{}
	out := []string{}
	for _, item := range items {
		item = strings.TrimSpace(item)
		if item == "" || seen[item] {
			continue
		}
		seen[item] = true
		out = append(out, item)
	}
	return out
}

func IsTerminalStatus(status string) bool {
	switch strings.ToLower(strings.TrimSpace(status)) {
	case "confirmed", "accepted", "rejected", "resolved", "deferred", "superseded":
		return true
	default:
		return false
	}
}

func IsClosedStatus(status string) bool {
	switch strings.ToLower(strings.TrimSpace(status)) {
	case "confirmed", "accepted", "rejected", "resolved", "superseded":
		return true
	default:
		return false
	}
}
