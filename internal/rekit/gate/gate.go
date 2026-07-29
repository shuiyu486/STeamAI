package gate

import (
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"reflect"
	"regexp"
	"sort"
	"strings"
	"time"

	"github.com/shuiyu486/re-context-kits/internal/rekit/adapterexecution"
	"github.com/shuiyu486/re-context-kits/internal/rekit/autonomy"
	refsf "github.com/shuiyu486/re-context-kits/internal/rekit/fs"
	"github.com/shuiyu486/re-context-kits/internal/rekit/instance"
	"github.com/shuiyu486/re-context-kits/internal/rekit/lanemutation"
	"github.com/shuiyu486/re-context-kits/internal/rekit/manifest"
	"github.com/shuiyu486/re-context-kits/internal/rekit/mission"
)

type gateLaneMutationLease interface {
	Validate() error
	Unlock() error
}

var acquireGateLaneMutationLease = func(caseRoot, laneID string) (gateLaneMutationLease, error) {
	return lanemutation.AcquireLane(caseRoot, laneID)
}

type Options struct {
	Action                                        string
	Lane                                          string
	Subject                                       string
	Summary                                       string
	Actor                                         string
	Risk                                          string
	TargetRef                                     string
	BatchID                                       string
	Scope                                         string
	Budget                                        string
	RuntimeSeconds                                int
	DiskMB                                        int
	Requests                                      int
	OutputPaths                                   string
	TriedLightSteps                               string
	StopConditions                                string
	GateEventID                                   string
	ExecutionStatus                               string
	ActualRuntimeSeconds                          int
	ActualDiskMB                                  int
	ActualRequests                                int
	OutputRefs                                    string
	EvidenceRefs                                  string
	BoundaryHits                                  string
	Escalation                                    string
	ExecutionReportPath                           string
	ExecutionReportCwd                            string
	ExecutionReportContract                       bool
	ValidateExecutionReport                       bool
	ScaffoldExecutionReport                       bool
	DraftExecutionReport                          bool
	RecordAdapterExecutionDispatch                bool
	RecordAdapterExecutionReceipt                 bool
	ExpectedExecutionReportSHA256                 string
	ExpectedAdapterExecutionDispatchBindingSHA256 string
	ExpectedAdapterExecutionBindingSHA256         string
	ExpectedAdapterExecutionReceiptSHA256         string
	AdapterExecutionReceiptPath                   string
	AdapterID                                     string
	Executor                                      string
	ExpectedExecutorGeneration                    int
	AdapterHarness                                string
	AdapterSession                                string
	ExecutionExitStatus                           string
}

type Plan struct {
	SchemaVersion               int                                      `json:"schemaVersion"`
	Command                     string                                   `json:"command"`
	CaseRoot                    string                                   `json:"caseRoot"`
	RepoRoot                    string                                   `json:"repoRoot"`
	Pack                        string                                   `json:"pack"`
	IsMutation                  bool                                     `json:"isMutation"`
	ReviewRequired              bool                                     `json:"reviewRequired"`
	RequiresConfirmation        bool                                     `json:"requiresConfirmation"`
	EventPreview                EventPreview                             `json:"eventPreview"`
	MissionBrief                mission.Brief                            `json:"missionBrief"`
	ExecutorAction              mission.ExecutorAction                   `json:"executorAction"`
	WouldExecutorAction         mission.ExecutorAction                   `json:"wouldExecutorAction"`
	MissionCommanderAction      mission.MissionCommanderAction           `json:"missionCommanderAction"`
	MissionCommanderNextActions []mission.MissionCommanderNextActionItem `json:"missionCommanderNextActions,omitempty"`
	BlockedActions              []string                                 `json:"blockedActions"`
	NextSteps                   []string                                 `json:"nextSteps"`
}

type ExecutionEvidencePreview struct {
	SchemaVersion int                      `json:"schemaVersion"`
	Kind          string                   `json:"kind"`
	Lane          string                   `json:"lane"`
	Subject       string                   `json:"subject"`
	Summary       string                   `json:"summary"`
	CreatedAt     string                   `json:"createdAt,omitempty"`
	Status        string                   `json:"status"`
	Actor         string                   `json:"actor,omitempty"`
	Risk          string                   `json:"risk,omitempty"`
	Target        string                   `json:"target,omitempty"`
	BatchID       string                   `json:"batchId,omitempty"`
	Related       []string                 `json:"related,omitempty"`
	EvidenceRefs  []string                 `json:"evidenceRefs,omitempty"`
	Gate          GateDetails              `json:"gate"`
	Execution     ExecutionEvidenceDetails `json:"execution"`
	EventID       string                   `json:"eventId,omitempty"`
}

type ExecutionEvidenceDetails struct {
	Status                         string                            `json:"status"`
	ActualBudget                   autonomy.Budget                   `json:"actualBudget"`
	OutputRefs                     []string                          `json:"outputRefs,omitempty"`
	BoundaryHits                   []string                          `json:"boundaryHits,omitempty"`
	Escalation                     string                            `json:"escalation,omitempty"`
	GateEventID                    string                            `json:"gateEventId"`
	GateStatus                     string                            `json:"gateStatus"`
	Authorization                  string                            `json:"authorization"`
	RecordRequired                 bool                              `json:"recordRequired"`
	NotifyMainOn                   []string                          `json:"notifyMainOn,omitempty"`
	ExecutionReportPath            string                            `json:"executionReportPath,omitempty"`
	ExecutionReportSHA256          string                            `json:"executionReportSha256,omitempty"`
	AdapterExecutionDispatchPath   string                            `json:"adapterExecutionDispatchPath,omitempty"`
	AdapterExecutionDispatchSHA256 string                            `json:"adapterExecutionDispatchSha256,omitempty"`
	AdapterExecutionDispatch       *adapterexecution.DispatchReceipt `json:"adapterExecutionDispatch,omitempty"`
	AdapterExecutionReceiptPath    string                            `json:"adapterExecutionReceiptPath,omitempty"`
	AdapterExecutionReceiptSHA256  string                            `json:"adapterExecutionReceiptSha256,omitempty"`
	AdapterExecution               *adapterexecution.Receipt         `json:"adapterExecution,omitempty"`
	AdapterContext                 *AdapterToolCandidate             `json:"adapterContext,omitempty"`
	Adapter                        *AdapterReport                    `json:"adapter,omitempty"`
}

type AdapterReport struct {
	SchemaVersion int                                     `json:"schemaVersion"`
	Kind          string                                  `json:"kind"`
	AdapterID     string                                  `json:"adapterId"`
	Action        string                                  `json:"action"`
	Status        string                                  `json:"status"`
	GateEventID   string                                  `json:"gateEventId"`
	Dispatch      *adapterexecution.ReportDispatchBinding `json:"dispatch,omitempty"`
	ActualBudget  autonomy.Budget                         `json:"actualBudget"`
	OutputRefs    []string                                `json:"outputRefs,omitempty"`
	EvidenceRefs  []string                                `json:"evidenceRefs,omitempty"`
	BoundaryHits  []string                                `json:"boundaryHits,omitempty"`
	Escalation    string                                  `json:"escalation,omitempty"`
	Summary       string                                  `json:"summary,omitempty"`
}

type AdapterExecutionReportContract struct {
	SchemaVersion                    int                                      `json:"schemaVersion"`
	Command                          string                                   `json:"command"`
	Kind                             string                                   `json:"kind"`
	CaseRoot                         string                                   `json:"caseRoot"`
	RepoRoot                         string                                   `json:"repoRoot"`
	Pack                             string                                   `json:"pack"`
	IsMutation                       bool                                     `json:"isMutation"`
	Lane                             string                                   `json:"lane"`
	Target                           string                                   `json:"target,omitempty"`
	BatchID                          string                                   `json:"batchId,omitempty"`
	Risk                             string                                   `json:"risk,omitempty"`
	Authorization                    autonomy.Decision                        `json:"authorization"`
	ReportKind                       string                                   `json:"reportKind"`
	ReportSchemaVersion              int                                      `json:"reportSchemaVersion"`
	GateEventID                      string                                   `json:"gateEventId"`
	Action                           string                                   `json:"action"`
	AllowedStatuses                  []string                                 `json:"allowedStatuses"`
	RequiredFields                   []string                                 `json:"requiredFields"`
	AllowedOutputPaths               []string                                 `json:"allowedOutputPaths"`
	DefaultReportPath                string                                   `json:"defaultReportPath,omitempty"`
	AuthorizedBudget                 autonomy.Budget                          `json:"authorizedBudget"`
	StopConditions                   []string                                 `json:"stopConditions,omitempty"`
	ReportPathRule                   string                                   `json:"reportPathRule"`
	RefPathRequires                  []string                                 `json:"refPathRequires,omitempty"`
	SummaryMaxBytes                  int                                      `json:"summaryMaxBytes"`
	RecordRequired                   bool                                     `json:"recordRequired"`
	NotifyMainOn                     []string                                 `json:"notifyMainOn,omitempty"`
	BoundaryStatusRequires           []string                                 `json:"boundaryStatusRequires,omitempty"`
	StatusSummaryRequires            []string                                 `json:"statusSummaryRequires,omitempty"`
	EscalationMaxBytes               int                                      `json:"escalationMaxBytes"`
	ValidationFailureStages          []AdapterReportValidationFailureStage    `json:"validationFailureStages,omitempty"`
	ValidationFailureCodes           []AdapterReportValidationFailureCode     `json:"validationFailureCodes,omitempty"`
	ValidationRepairHints            []AdapterReportRepairHint                `json:"validationRepairHints,omitempty"`
	DeniedActions                    []string                                 `json:"deniedActions,omitempty"`
	LiveValidation                   AdapterReportLiveValidation              `json:"liveValidation"`
	ReportSummary                    AdapterReportHandoffSummary              `json:"reportSummary"`
	AuthorizedExecutionFollowThrough AuthorizedExecutionFollowThrough         `json:"authorizedExecutionFollowThrough"`
	MissionCommanderAction           mission.MissionCommanderAction           `json:"missionCommanderAction"`
	MissionCommanderNextActions      []mission.MissionCommanderNextActionItem `json:"missionCommanderNextActions,omitempty"`
	MissionCommanderActionQueue      mission.MissionCommanderActionQueue      `json:"missionCommanderActionQueue"`
	RunbookSteps                     []string                                 `json:"runbookSteps,omitempty"`
	NextSteps                        []string                                 `json:"nextSteps,omitempty"`
}

type AdapterExecutionReportScaffold struct {
	SchemaVersion               int                                      `json:"schemaVersion"`
	Command                     string                                   `json:"command"`
	Kind                        string                                   `json:"kind"`
	CaseRoot                    string                                   `json:"caseRoot"`
	RepoRoot                    string                                   `json:"repoRoot"`
	Pack                        string                                   `json:"pack"`
	IsMutation                  bool                                     `json:"isMutation"`
	Applied                     bool                                     `json:"applied"`
	Replay                      bool                                     `json:"replay,omitempty"`
	Mode                        string                                   `json:"mode"`
	GateEventID                 string                                   `json:"gateEventId"`
	ReportPath                  string                                   `json:"reportPath"`
	ReportSHA256                string                                   `json:"reportSha256"`
	AlreadyExists               bool                                     `json:"alreadyExists"`
	RequiresConfirmation        bool                                     `json:"requiresConfirmation"`
	SidecarTemplate             AdapterReportSidecarTemplate             `json:"sidecarTemplate"`
	ValidateCommand             string                                   `json:"validateCommand"`
	RecordCommand               string                                   `json:"recordCommand,omitempty"`
	ApplyCommand                string                                   `json:"applyCommand,omitempty"`
	Boundary                    []string                                 `json:"boundary,omitempty"`
	NextSteps                   []string                                 `json:"nextSteps,omitempty"`
	RunbookSteps                []string                                 `json:"runbookSteps,omitempty"`
	MissionCommanderAction      mission.MissionCommanderAction           `json:"missionCommanderAction"`
	MissionCommanderNextActions []mission.MissionCommanderNextActionItem `json:"missionCommanderNextActions,omitempty"`
	MissionCommanderActionQueue mission.MissionCommanderActionQueue      `json:"missionCommanderActionQueue"`
}

type AdapterExecutionReportDraft struct {
	SchemaVersion               int                                      `json:"schemaVersion"`
	Command                     string                                   `json:"command"`
	Kind                        string                                   `json:"kind"`
	CaseRoot                    string                                   `json:"caseRoot"`
	RepoRoot                    string                                   `json:"repoRoot"`
	Pack                        string                                   `json:"pack"`
	IsMutation                  bool                                     `json:"isMutation"`
	Applied                     bool                                     `json:"applied"`
	Replay                      bool                                     `json:"replay,omitempty"`
	Mode                        string                                   `json:"mode"`
	GateEventID                 string                                   `json:"gateEventId"`
	ReportPath                  string                                   `json:"reportPath"`
	ReportSHA256                string                                   `json:"reportSha256"`
	AlreadyExists               bool                                     `json:"alreadyExists"`
	ReplacesScaffold            bool                                     `json:"replacesScaffold"`
	RequiresConfirmation        bool                                     `json:"requiresConfirmation"`
	Report                      AdapterReport                            `json:"report"`
	ValidateCommand             string                                   `json:"validateCommand"`
	RecordCommand               string                                   `json:"recordCommand,omitempty"`
	ApplyCommand                string                                   `json:"applyCommand,omitempty"`
	Boundary                    []string                                 `json:"boundary,omitempty"`
	NextSteps                   []string                                 `json:"nextSteps,omitempty"`
	RunbookSteps                []string                                 `json:"runbookSteps,omitempty"`
	MissionCommanderAction      mission.MissionCommanderAction           `json:"missionCommanderAction"`
	MissionCommanderNextActions []mission.MissionCommanderNextActionItem `json:"missionCommanderNextActions,omitempty"`
	MissionCommanderActionQueue mission.MissionCommanderActionQueue      `json:"missionCommanderActionQueue"`
}

type AdapterReportLiveValidation struct {
	InvocationCwd                    string                       `json:"invocationCwd"`
	AuthorizedWorkspaces             []string                     `json:"authorizedWorkspaces,omitempty"`
	ReportFileName                   string                       `json:"reportFileName"`
	CaseRelativeReportPath           string                       `json:"caseRelativeReportPath,omitempty"`
	DispatchRequired                 bool                         `json:"dispatchRequired,omitempty"`
	DispatchPresent                  bool                         `json:"dispatchPresent,omitempty"`
	DispatchCurrent                  bool                         `json:"dispatchCurrent,omitempty"`
	AdapterExecutionDispatchID       string                       `json:"adapterExecutionDispatchId,omitempty"`
	AdapterExecutionDispatchPath     string                       `json:"adapterExecutionDispatchPath,omitempty"`
	AdapterExecutionDispatchSHA256   string                       `json:"adapterExecutionDispatchSha256,omitempty"`
	DispatchError                    string                       `json:"dispatchError,omitempty"`
	DispatchRequirementError         string                       `json:"dispatchRequirementError,omitempty"`
	SidecarTemplate                  AdapterReportSidecarTemplate `json:"sidecarTemplate"`
	ValidateCommand                  string                       `json:"validateCommand"`
	RecordCommand                    string                       `json:"recordCommand,omitempty"`
	ScaffoldCommand                  string                       `json:"scaffoldCommand,omitempty"`
	ScaffoldApplyCommand             string                       `json:"scaffoldApplyCommand,omitempty"`
	SidecarTemplateSHA256            string                       `json:"sidecarTemplateSha256,omitempty"`
	DraftCommand                     string                       `json:"draftCommand,omitempty"`
	DraftApplyCommand                string                       `json:"draftApplyCommand,omitempty"`
	DraftReportSHA256                string                       `json:"draftReportSha256,omitempty"`
	ValidateArgs                     []string                     `json:"validateArgs"`
	RecordArgs                       []string                     `json:"recordArgs,omitempty"`
	ScaffoldArgs                     []string                     `json:"scaffoldArgs,omitempty"`
	ScaffoldApplyArgs                []string                     `json:"scaffoldApplyArgs,omitempty"`
	DraftArgs                        []string                     `json:"draftArgs,omitempty"`
	DraftApplyArgs                   []string                     `json:"draftApplyArgs,omitempty"`
	CaseRelativeValidateCommand      string                       `json:"caseRelativeValidateCommand,omitempty"`
	CaseRelativeRecordCommand        string                       `json:"caseRelativeRecordCommand,omitempty"`
	CaseRelativeScaffoldCommand      string                       `json:"caseRelativeScaffoldCommand,omitempty"`
	CaseRelativeScaffoldApplyCommand string                       `json:"caseRelativeScaffoldApplyCommand,omitempty"`
	CaseRelativeDraftCommand         string                       `json:"caseRelativeDraftCommand,omitempty"`
	CaseRelativeDraftApplyCommand    string                       `json:"caseRelativeDraftApplyCommand,omitempty"`
	CaseRelativeValidateArgs         []string                     `json:"caseRelativeValidateArgs,omitempty"`
	CaseRelativeRecordArgs           []string                     `json:"caseRelativeRecordArgs,omitempty"`
	CaseRelativeScaffoldArgs         []string                     `json:"caseRelativeScaffoldArgs,omitempty"`
	CaseRelativeScaffoldApplyArgs    []string                     `json:"caseRelativeScaffoldApplyArgs,omitempty"`
	CaseRelativeDraftArgs            []string                     `json:"caseRelativeDraftArgs,omitempty"`
	CaseRelativeDraftApplyArgs       []string                     `json:"caseRelativeDraftApplyArgs,omitempty"`
	AdapterCandidates                []AdapterToolCandidate       `json:"adapterCandidates,omitempty"`
	SelectedAdapter                  *AdapterToolCandidate        `json:"selectedAdapter,omitempty"`
	ReplayBehavior                   string                       `json:"replayBehavior"`
	RunbookSteps                     []string                     `json:"runbookSteps,omitempty"`
	Notes                            []string                     `json:"notes,omitempty"`
}

type AdapterReportHandoffSummary struct {
	State                      string   `json:"state"`
	GateEventID                string   `json:"gateEventId"`
	Action                     string   `json:"action"`
	Lane                       string   `json:"lane"`
	ReportPath                 string   `json:"reportPath,omitempty"`
	ReportSHA256               string   `json:"reportSha256,omitempty"`
	RecordExpectedReportSHA256 string   `json:"recordExpectedReportSha256,omitempty"`
	DefaultReportPath          string   `json:"defaultReportPath,omitempty"`
	ReportPresent              bool     `json:"reportPresent"`
	Valid                      bool     `json:"valid"`
	RecordReady                bool     `json:"recordReady"`
	RecordBlocked              bool     `json:"recordBlocked"`
	RequiresValidation         bool     `json:"requiresValidation"`
	RequiresRepair             bool     `json:"requiresRepair"`
	RequiresMainEscalation     bool     `json:"requiresMainEscalation"`
	AllowedStatusCount         int      `json:"allowedStatusCount"`
	AllowedOutputPathCount     int      `json:"allowedOutputPathCount"`
	AuthorizedStopCount        int      `json:"authorizedStopCount"`
	AdapterCandidateCount      int      `json:"adapterCandidateCount"`
	RepairHintCount            int      `json:"repairHintCount"`
	RecordBlockedHintCount     int      `json:"recordBlockedHintCount"`
	EscalateHintCount          int      `json:"escalateHintCount"`
	OutcomeCount               int      `json:"outcomeCount"`
	NextActionCount            int      `json:"nextActionCount"`
	ReviewRequiredActionCount  int      `json:"reviewRequiredActionCount"`
	ActionQueueSummary         string   `json:"actionQueueSummary,omitempty"`
	CurrentAction              string   `json:"currentAction,omitempty"`
	ReportStatus               string   `json:"reportStatus,omitempty"`
	AdapterID                  string   `json:"adapterId,omitempty"`
	ActualRuntimeSeconds       int      `json:"actualRuntimeSeconds,omitempty"`
	ActualDiskMB               int      `json:"actualDiskMB,omitempty"`
	ActualRequests             int      `json:"actualRequests,omitempty"`
	OutputRefCount             int      `json:"outputRefCount"`
	EvidenceRefCount           int      `json:"evidenceRefCount"`
	BoundaryHitCount           int      `json:"boundaryHitCount"`
	HasEscalation              bool     `json:"hasEscalation"`
	HasSummary                 bool     `json:"hasSummary"`
	ValidationFailureCode      string   `json:"validationFailureCode,omitempty"`
	ValidationFailureStage     string   `json:"validationFailureStage,omitempty"`
	Boundary                   []string `json:"boundary,omitempty"`
}

type AuthorizedExecutionFollowThrough struct {
	State       string                              `json:"state"`
	GateEventID string                              `json:"gateEventId"`
	ReportPath  string                              `json:"reportPath,omitempty"`
	Outcomes    []AuthorizedExecutionOutcome        `json:"outcomes"`
	Boundary    []string                            `json:"boundary"`
	ActionQueue mission.MissionCommanderActionQueue `json:"actionQueue"`
}

type AuthorizedExecutionOutcome struct {
	Name                 string   `json:"name"`
	State                string   `json:"state"`
	When                 string   `json:"when"`
	Command              string   `json:"command,omitempty"`
	Actions              []string `json:"actions,omitempty"`
	RepairActions        []string `json:"repairActions,omitempty"`
	VerificationCommands []string `json:"verificationCommands,omitempty"`
	Expected             string   `json:"expected"`
	Evidence             []string `json:"evidence,omitempty"`
	Boundary             []string `json:"boundary,omitempty"`
}

type AdapterToolCandidate struct {
	ID                  string   `json:"id"`
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

type AdapterReportSidecarTemplate struct {
	SchemaVersion int                                     `json:"schemaVersion"`
	Kind          string                                  `json:"kind"`
	AdapterID     string                                  `json:"adapterId"`
	Action        string                                  `json:"action"`
	Status        string                                  `json:"status"`
	GateEventID   string                                  `json:"gateEventId"`
	Dispatch      *adapterexecution.ReportDispatchBinding `json:"dispatch,omitempty"`
	ActualBudget  autonomy.Budget                         `json:"actualBudget"`
	OutputRefs    []string                                `json:"outputRefs"`
	EvidenceRefs  []string                                `json:"evidenceRefs"`
	BoundaryHits  []string                                `json:"boundaryHits"`
	Escalation    string                                  `json:"escalation"`
	Summary       string                                  `json:"summary"`
}

type AdapterReportValidationFailureStage struct {
	Stage       string `json:"stage"`
	Description string `json:"description"`
}

type AdapterContext struct {
	Candidates []AdapterToolCandidate `json:"candidates,omitempty"`
	Selected   *AdapterToolCandidate  `json:"selected,omitempty"`
}

type AdapterReportValidationFailureCode struct {
	Code        string `json:"code"`
	Stage       string `json:"stage"`
	Description string `json:"description"`
}

type AdapterReportRepairHint struct {
	Code                  string   `json:"code,omitempty"`
	Stage                 string   `json:"stage,omitempty"`
	RepairAction          string   `json:"repairAction"`
	Fields                []string `json:"fields,omitempty"`
	AllowedValues         []string `json:"allowedValues,omitempty"`
	AllowedOutputPaths    []string `json:"allowedOutputPaths,omitempty"`
	AllowedStopConditions []string `json:"allowedStopConditions,omitempty"`
	MaxBytes              int      `json:"maxBytes,omitempty"`
	Evidence              []string `json:"evidence,omitempty"`
	Boundary              []string `json:"boundary,omitempty"`
	RecordBlocked         bool     `json:"recordBlocked"`
	RerunValidation       bool     `json:"rerunValidation"`
	EscalateToMain        bool     `json:"escalateToMain,omitempty"`
	Detail                string   `json:"detail"`
}

type AdapterExecutionReportValidation struct {
	SchemaVersion                    int                                      `json:"schemaVersion"`
	Command                          string                                   `json:"command"`
	Kind                             string                                   `json:"kind"`
	CaseRoot                         string                                   `json:"caseRoot"`
	RepoRoot                         string                                   `json:"repoRoot"`
	Pack                             string                                   `json:"pack"`
	IsMutation                       bool                                     `json:"isMutation"`
	Applied                          bool                                     `json:"applied"`
	Valid                            bool                                     `json:"valid"`
	ReportSHA256                     string                                   `json:"reportSha256,omitempty"`
	RecordExpectedReportSHA256       string                                   `json:"recordExpectedReportSha256,omitempty"`
	ReceiptRequired                  bool                                     `json:"receiptRequired"`
	ReceiptPresent                   bool                                     `json:"receiptPresent"`
	ProvenanceValid                  bool                                     `json:"provenanceValid"`
	AdapterExecutionDispatchPath     string                                   `json:"adapterExecutionDispatchPath,omitempty"`
	AdapterExecutionDispatchSHA256   string                                   `json:"adapterExecutionDispatchSha256,omitempty"`
	AdapterExecutionDispatch         *adapterexecution.DispatchReceipt        `json:"adapterExecutionDispatch,omitempty"`
	AdapterExecutionReceiptPath      string                                   `json:"adapterExecutionReceiptPath,omitempty"`
	AdapterExecutionReceiptSHA256    string                                   `json:"adapterExecutionReceiptSha256,omitempty"`
	AdapterExecution                 *adapterexecution.Receipt                `json:"adapterExecution,omitempty"`
	ReceiptPreviewCommand            string                                   `json:"receiptPreviewCommand,omitempty"`
	Error                            string                                   `json:"error,omitempty"`
	Errors                           []string                                 `json:"errors,omitempty"`
	FailureCode                      string                                   `json:"failureCode,omitempty"`
	FailureStage                     string                                   `json:"failureStage,omitempty"`
	RepairHints                      []AdapterReportRepairHint                `json:"repairHints,omitempty"`
	GateEventID                      string                                   `json:"gateEventId"`
	ReportPath                       string                                   `json:"reportPath,omitempty"`
	Report                           *AdapterReport                           `json:"report,omitempty"`
	AdapterContext                   *AdapterContext                          `json:"adapterContext,omitempty"`
	Contract                         AdapterExecutionReportContract           `json:"contract"`
	ReportSummary                    AdapterReportHandoffSummary              `json:"reportSummary"`
	AuthorizedExecutionFollowThrough AuthorizedExecutionFollowThrough         `json:"authorizedExecutionFollowThrough"`
	MissionCommanderAction           mission.MissionCommanderAction           `json:"missionCommanderAction"`
	MissionCommanderNextActions      []mission.MissionCommanderNextActionItem `json:"missionCommanderNextActions,omitempty"`
	MissionCommanderActionQueue      mission.MissionCommanderActionQueue      `json:"missionCommanderActionQueue"`
	RunbookSteps                     []string                                 `json:"runbookSteps,omitempty"`
	NextSteps                        []string                                 `json:"nextSteps"`
}

type adapterReportValidationError struct {
	Code  string
	Stage string
	Err   error
}

func (e *adapterReportValidationError) Error() string {
	if e == nil || e.Err == nil {
		return ""
	}
	return e.Err.Error()
}

func (e *adapterReportValidationError) Unwrap() error {
	if e == nil {
		return nil
	}
	return e.Err
}

func adapterReportValidationErrorf(code, stage, format string, args ...any) error {
	return &adapterReportValidationError{Code: code, Stage: stage, Err: fmt.Errorf(format, args...)}
}

func adapterReportValidationFailureStages() []AdapterReportValidationFailureStage {
	return []AdapterReportValidationFailureStage{
		{Stage: "path", Description: "Report path list, case-root containment, cwd-relative resolution, and authorized output path scope checks."},
		{Stage: "read", Description: "Report file existence, file type, size, and read/open checks."},
		{Stage: "decode", Description: "JSON decoding, unknown field rejection, and trailing data checks."},
		{Stage: "schema", Description: "Report schemaVersion, kind, adapterId, and status checks."},
		{Stage: "identity", Description: "Authorized gate action and gateEventId binding checks."},
		{Stage: "refs", Description: "Case-relative output/evidence refs and authorized output path scope checks."},
		{Stage: "budget", Description: "Actual budget non-negative values and budget-overrun marker checks."},
		{Stage: "boundary", Description: "Boundary hit token syntax, authorized stopCondition coverage, boundary/escalated status marker, and escalation size checks."},
		{Stage: "summary", Description: "Required failure/boundary/escalation/abort summary and bounded summary size checks."},
	}
}

func adapterReportValidationFailureCodes() []AdapterReportValidationFailureCode {
	return []AdapterReportValidationFailureCode{
		{Code: "path-list", Stage: "path", Description: "Execution report path must be a single file path."},
		{Code: "path-invalid", Stage: "path", Description: "Execution report path must be case-contained and case-relative, current-workspace relative, or case-contained absolute."},
		{Code: "report-path-out-of-scope", Stage: "path", Description: "Execution report path must stay within one authorized output path."},
		{Code: "report-not-readable", Stage: "read", Description: "Execution report file could not be stat/open/read."},
		{Code: "report-path-directory", Stage: "read", Description: "Execution report path names a directory instead of a file."},
		{Code: "report-not-regular", Stage: "read", Description: "Execution report path must name a regular non-symlink file."},
		{Code: "report-too-large", Stage: "read", Description: "Execution report sidecar exceeds 1048576 bytes."},
		{Code: "report-json-invalid", Stage: "decode", Description: "Execution report JSON could not be decoded or contains unknown fields."},
		{Code: "report-trailing-data", Stage: "decode", Description: "Execution report contains trailing JSON data after the first object."},
		{Code: "schema-version", Stage: "schema", Description: "Execution report schemaVersion must be 1."},
		{Code: "kind", Stage: "schema", Description: "Execution report kind must be adapter-execution-report."},
		{Code: "adapter-id-missing", Stage: "schema", Description: "Execution report must include adapterId."},
		{Code: "status", Stage: "schema", Description: "Execution report status must be one of the allowed statuses."},
		{Code: "action-mismatch", Stage: "identity", Description: "Execution report action must match the authorized gate action."},
		{Code: "gate-event-mismatch", Stage: "identity", Description: "Execution report gateEventId must match the authorized gate eventId."},
		{Code: "output-refs-invalid", Stage: "refs", Description: "Execution report outputRefs must be case-relative refs."},
		{Code: "output-refs-out-of-scope", Stage: "refs", Description: "Execution report outputRefs must stay within authorized output paths."},
		{Code: "evidence-refs-invalid", Stage: "refs", Description: "Execution report evidenceRefs must be case-relative refs."},
		{Code: "evidence-refs-out-of-scope", Stage: "refs", Description: "Execution report evidenceRefs must stay within authorized output paths."},
		{Code: "actual-budget-negative", Stage: "budget", Description: "Execution report actualBudget values must be non-negative."},
		{Code: "budget-marker-missing", Stage: "budget", Description: "Budget overrun reports must include boundaryHits or escalation."},
		{Code: "boundary-hits-invalid", Stage: "boundary", Description: "Execution report boundaryHits must use supported stop-condition tokens."},
		{Code: "boundary-hits-not-authorized", Stage: "boundary", Description: "Execution report boundaryHits must be covered by the authorized gate stopConditions."},
		{Code: "escalation-too-large", Stage: "boundary", Description: "Execution report escalation must stay within the bounded size limit."},
		{Code: "boundary-marker-missing", Stage: "boundary", Description: "Boundary-hit/escalated reports must include boundaryHits or escalation."},
		{Code: "status-summary-missing", Stage: "summary", Description: "Failed, boundary-hit, escalated, or aborted reports must include a bounded summary."},
		{Code: "summary-too-large", Stage: "summary", Description: "Execution report summary must stay within the bounded size limit."},
	}
}

func adapterReportMissingPathRepairHints(gateEvent EventPreview) []AdapterReportRepairHint {
	hint := AdapterReportRepairHint{
		Stage:              "path",
		RepairAction:       "provide-execution-report-path",
		Fields:             []string{"executionReportPath"},
		AllowedOutputPaths: normalizedGatePaths(gateEvent.Gate.OutputPaths),
		RecordBlocked:      true,
		RerunValidation:    true,
		Detail:             "provide -ExecutionReportPath under an authorized output path before recording evidence",
	}
	return []AdapterReportRepairHint{adapterReportRepairHintWithHandoff(gateEvent, hint)}
}

func adapterReportContractRepairHints(gateEvent EventPreview) []AdapterReportRepairHint {
	hints := adapterReportMissingPathRepairHints(gateEvent)
	for _, failure := range adapterReportValidationFailureCodes() {
		hints = append(hints, adapterReportRepairHints(gateEvent, failure.Code, failure.Stage)...)
	}
	return hints
}

func adapterReportRepairHints(gateEvent EventPreview, code, stage string) []AdapterReportRepairHint {
	if strings.TrimSpace(code) == "" {
		hint := AdapterReportRepairHint{
			Stage:           stage,
			RepairAction:    "escalate-validation-error",
			RecordBlocked:   true,
			RerunValidation: true,
			EscalateToMain:  true,
			Detail:          "validation failed without a stable failureCode; escalate the bounded sidecar and validation error to the main Agent",
		}
		return []AdapterReportRepairHint{adapterReportRepairHintWithHandoff(gateEvent, hint)}
	}
	hint := AdapterReportRepairHint{Code: code, Stage: stage, RecordBlocked: true, RerunValidation: true}
	switch code {
	case "path-list":
		hint.RepairAction = "use-single-report-path"
		hint.Fields = []string{"executionReportPath"}
		hint.Detail = "pass exactly one -ExecutionReportPath value"
	case "path-invalid":
		hint.RepairAction = "fix-report-path-containment"
		hint.Fields = []string{"executionReportPath"}
		hint.Detail = "use a case-relative, current-workspace relative, or case-contained absolute report path"
	case "report-path-out-of-scope":
		hint.RepairAction = "move-report-under-authorized-output-path"
		hint.Fields = []string{"executionReportPath"}
		hint.AllowedOutputPaths = normalizedGatePaths(gateEvent.Gate.OutputPaths)
		hint.Detail = "store the bounded sidecar under one authorized output path"
	case "report-not-readable":
		hint.RepairAction = "create-readable-report-file"
		hint.Fields = []string{"executionReportPath"}
		hint.AllowedOutputPaths = normalizedGatePaths(gateEvent.Gate.OutputPaths)
		hint.Detail = "write a readable adapter execution report sidecar at the requested path"
	case "report-path-directory":
		hint.RepairAction = "use-report-file-not-directory"
		hint.Fields = []string{"executionReportPath"}
		hint.Detail = "point -ExecutionReportPath at the sidecar JSON file, not a directory"
	case "report-not-regular":
		hint.RepairAction = "use-regular-report-file"
		hint.Fields = []string{"executionReportPath"}
		hint.Detail = "replace symlink or special-file sidecars with a regular JSON file"
	case "report-too-large":
		hint.RepairAction = "shrink-report-sidecar"
		hint.Fields = []string{"executionReportPath"}
		hint.MaxBytes = 1 << 20
		hint.Detail = "keep the sidecar <= 1048576 bytes and move full trace/dump/log data into referenced artifacts"
	case "report-json-invalid":
		hint.RepairAction = "fix-report-json"
		hint.Fields = []string{"schemaVersion", "kind", "adapterId", "action", "status", "gateEventId", "actualBudget", "outputRefs", "evidenceRefs", "boundaryHits", "escalation", "summary"}
		hint.Detail = "emit one valid JSON object using only adapter-execution-report fields"
	case "report-trailing-data":
		hint.RepairAction = "remove-trailing-json-data"
		hint.Detail = "keep exactly one JSON object in the sidecar with no trailing data"
	case "schema-version":
		hint.RepairAction = "set-schema-version"
		hint.Fields = []string{"schemaVersion"}
		hint.AllowedValues = []string{"1"}
		hint.Detail = "set schemaVersion to 1"
	case "kind":
		hint.RepairAction = "set-report-kind"
		hint.Fields = []string{"kind"}
		hint.AllowedValues = []string{"adapter-execution-report"}
		hint.Detail = "set kind to adapter-execution-report"
	case "adapter-id-missing":
		hint.RepairAction = "set-adapter-id"
		hint.Fields = []string{"adapterId"}
		hint.Detail = "set adapterId to the executor/tool adapter identifier"
	case "status":
		hint.RepairAction = "set-valid-status"
		hint.Fields = []string{"status"}
		hint.AllowedValues = []string{"succeeded", "failed", "boundary-hit", "escalated", "aborted"}
		hint.Detail = "set status to one allowed adapter execution status"
	case "action-mismatch":
		hint.RepairAction = "match-authorized-action"
		hint.Fields = []string{"action"}
		hint.AllowedValues = []string{gateEvent.Gate.Action}
		hint.Detail = "set action to the authorized gate action"
	case "gate-event-mismatch":
		hint.RepairAction = "match-authorized-gate-event"
		hint.Fields = []string{"gateEventId"}
		hint.AllowedValues = []string{gateEvent.EventID}
		hint.Detail = "set gateEventId to the authorized-gate eventId being validated"
	case "output-refs-invalid":
		hint.RepairAction = "make-output-refs-case-relative"
		hint.Fields = []string{"outputRefs"}
		hint.Detail = "keep outputRefs case-relative and inside the case root"
	case "output-refs-out-of-scope":
		hint.RepairAction = "move-output-refs-under-authorized-output-paths"
		hint.Fields = []string{"outputRefs"}
		hint.AllowedOutputPaths = normalizedGatePaths(gateEvent.Gate.OutputPaths)
		hint.Detail = "keep outputRefs under authorized outputPaths"
	case "evidence-refs-invalid":
		hint.RepairAction = "make-evidence-refs-case-relative"
		hint.Fields = []string{"evidenceRefs"}
		hint.Detail = "keep evidenceRefs case-relative and inside the case root"
	case "evidence-refs-out-of-scope":
		hint.RepairAction = "move-evidence-refs-under-authorized-output-paths"
		hint.Fields = []string{"evidenceRefs"}
		hint.AllowedOutputPaths = normalizedGatePaths(gateEvent.Gate.OutputPaths)
		hint.Detail = "keep evidenceRefs under authorized outputPaths"
	case "actual-budget-negative":
		hint.RepairAction = "set-non-negative-actual-budget"
		hint.Fields = []string{"actualBudget.runtimeSeconds", "actualBudget.diskMB", "actualBudget.requests"}
		hint.Detail = "set all actualBudget values to zero or positive integers"
	case "budget-marker-missing":
		hint.RepairAction = "record-budget-boundary-marker"
		hint.Fields = []string{"actualBudget", "boundaryHits", "escalation"}
		hint.AllowedStopConditions = append([]string{}, gateEvent.Gate.StopConditions...)
		hint.Detail = "when actualBudget exceeds the authorized budget, add authorized boundaryHits or a bounded escalation"
	case "boundary-hits-invalid":
		hint.RepairAction = "use-valid-boundary-hit-tokens"
		hint.Fields = []string{"boundaryHits"}
		hint.AllowedStopConditions = append([]string{}, gateEvent.Gate.StopConditions...)
		hint.Detail = "replace boundaryHits with supported stop-condition tokens"
	case "boundary-hits-not-authorized":
		hint.RepairAction = "use-authorized-boundary-hit-tokens-or-escalate"
		hint.Fields = []string{"boundaryHits", "escalation"}
		hint.AllowedStopConditions = append([]string{}, gateEvent.Gate.StopConditions...)
		hint.EscalateToMain = true
		hint.Detail = "use only authorized stopConditions in boundaryHits, or replace the marker with a bounded escalation for main Agent review"
	case "escalation-too-large":
		hint.RepairAction = "shrink-escalation-summary"
		hint.Fields = []string{"escalation"}
		hint.MaxBytes = 4096
		hint.Detail = "keep escalation <= 4096 bytes and reference larger evidence as outputRefs/evidenceRefs"
	case "boundary-marker-missing":
		hint.RepairAction = "add-boundary-marker"
		hint.Fields = []string{"boundaryHits", "escalation"}
		hint.AllowedStopConditions = append([]string{}, gateEvent.Gate.StopConditions...)
		hint.Detail = "boundary-hit or escalated status requires authorized boundaryHits or a bounded escalation"
	case "status-summary-missing":
		hint.RepairAction = "add-required-status-summary"
		hint.Fields = []string{"summary"}
		hint.MaxBytes = 4096
		hint.Detail = "failed, boundary-hit, escalated, or aborted status requires a bounded summary"
	case "summary-too-large":
		hint.RepairAction = "shrink-status-summary"
		hint.Fields = []string{"summary"}
		hint.MaxBytes = 4096
		hint.Detail = "keep summary <= 4096 bytes and reference larger evidence as outputRefs/evidenceRefs"
	default:
		hint.RepairAction = "escalate-validation-error"
		hint.EscalateToMain = true
		hint.Detail = "validation failed with an unknown failureCode; escalate the bounded sidecar and validation error to the main Agent"
	}
	return []AdapterReportRepairHint{adapterReportRepairHintWithHandoff(gateEvent, hint)}
}

func adapterReportRepairHintWithHandoff(gateEvent EventPreview, hint AdapterReportRepairHint) AdapterReportRepairHint {
	if len(hint.Evidence) == 0 {
		hint.Evidence = adapterReportRepairHintEvidence(hint)
	}
	if len(hint.Boundary) == 0 {
		hint.Boundary = adapterReportRepairHintBoundary(gateEvent, hint)
	}
	return hint
}

func adapterReportRepairHintEvidence(hint AdapterReportRepairHint) []string {
	evidence := []string{"repairHints[].repairAction"}
	if strings.TrimSpace(hint.Code) != "" || strings.TrimSpace(hint.Stage) != "" {
		evidence = append(evidence, "failureCode/failureStage")
	}
	if len(hint.Fields) > 0 {
		evidence = append(evidence, "fields="+strings.Join(hint.Fields, ","))
	}
	if len(hint.AllowedValues) > 0 {
		evidence = append(evidence, "allowedValues="+strings.Join(hint.AllowedValues, ","))
	}
	if len(hint.AllowedOutputPaths) > 0 {
		evidence = append(evidence, "allowedOutputPaths="+strings.Join(hint.AllowedOutputPaths, ","))
	}
	if len(hint.AllowedStopConditions) > 0 {
		evidence = append(evidence, "allowedStopConditions="+strings.Join(hint.AllowedStopConditions, ","))
	}
	if hint.MaxBytes > 0 {
		evidence = append(evidence, fmt.Sprintf("maxBytes=%d", hint.MaxBytes))
	}
	return mission.UniqueStrings(evidence)
}

func adapterReportRepairHintBoundary(gateEvent EventPreview, hint AdapterReportRepairHint) []string {
	boundary := []string{
		"recordBlocked=true: do not record evidence until validation returns valid=true",
		"validation is read-only: no observations/authority/confirmed writes",
		"/rekit never executes the heavy tool",
		"no authority/confirmed writes",
	}
	if hint.RerunValidation {
		boundary = append(boundary, "rerun read-only validation after repair")
	}
	allowedOutputPaths := hint.AllowedOutputPaths
	if len(allowedOutputPaths) == 0 && adapterReportRepairHintUsesAuthorizedOutputPaths(hint) {
		allowedOutputPaths = normalizedGatePaths(gateEvent.Gate.OutputPaths)
	}
	if len(allowedOutputPaths) > 0 {
		boundary = append(boundary, "stay under authorized outputPaths: "+strings.Join(allowedOutputPaths, ","))
	}
	if len(hint.AllowedStopConditions) > 0 {
		boundary = append(boundary, "boundaryHits must use authorized stopConditions: "+strings.Join(hint.AllowedStopConditions, ","))
	}
	if hint.MaxBytes > 0 {
		boundary = append(boundary, fmt.Sprintf("bounded field maxBytes=%d", hint.MaxBytes))
	}
	if hint.EscalateToMain {
		boundary = append(boundary, "escalate to main Agent before autonomous continuation")
	}
	return mission.UniqueStrings(boundary)
}

func adapterReportRepairHintUsesAuthorizedOutputPaths(hint AdapterReportRepairHint) bool {
	for _, field := range hint.Fields {
		switch field {
		case "executionReportPath", "outputRefs", "evidenceRefs":
			return true
		}
	}
	return false
}

func adapterReportRepairHintReasons(hint AdapterReportRepairHint) []string {
	reasons := []string{"repair invalid adapter execution report before record"}
	if strings.TrimSpace(hint.Detail) != "" {
		reasons = append(reasons, hint.Detail)
	}
	reasons = append(reasons, hint.Evidence...)
	if hint.RecordBlocked {
		reasons = append(reasons, "recordBlocked=true; do not record evidence until valid=true")
	}
	if hint.RerunValidation {
		reasons = append(reasons, "rerun read-only validation after repair")
	}
	if hint.EscalateToMain {
		reasons = append(reasons, "repair requires main Agent escalation")
	}
	return mission.UniqueStrings(reasons)
}

func adapterReportRepairHintBoundaries(base []string, hint AdapterReportRepairHint) []string {
	boundary := append([]string{}, base...)
	boundary = append(boundary, hint.Boundary...)
	return mission.UniqueStrings(boundary)
}

func adapterReportRepairNextSteps(hints []AdapterReportRepairHint) []string {
	if len(hints) == 0 {
		return []string{"report is invalid for read-only preflight", "fix the bounded adapter execution report sidecar and rerun gate -ValidateExecutionReport", "do not record it with gate -Apply until valid=true"}
	}
	steps := []string{"report is invalid for read-only preflight"}
	for _, hint := range hints {
		if hint.RepairAction != "" {
			steps = append(steps, "repairAction: "+hint.RepairAction)
		}
	}
	steps = append(steps, "rerun gate -ValidateExecutionReport before recording evidence", "do not record it with gate -Apply until valid=true")
	return steps
}

func adapterReportRunbookSteps(stage, state, reportPath, reportSHA256 string, valid, recordReady, applied, duplicate bool, nextSteps, boundary []string, commander mission.MissionCommanderAction) []string {
	steps := []string{fmt.Sprintf("confirm adapter report %s state=%s", strings.TrimSpace(stage), strings.TrimSpace(state))}
	if strings.TrimSpace(reportPath) != "" {
		steps = append(steps, "confirm report path: "+strings.TrimSpace(reportPath))
	}
	if strings.TrimSpace(reportSHA256) != "" {
		steps = append(steps, "confirm report sha256: "+strings.TrimSpace(reportSHA256))
	}
	for _, next := range nextSteps {
		if strings.TrimSpace(next) != "" {
			steps = append(steps, "handoff reason: "+strings.TrimSpace(next))
		}
	}
	if strings.TrimSpace(commander.PrimaryCommand) != "" {
		steps = append(steps, "run current Mission Commander command: "+strings.TrimSpace(commander.PrimaryCommand))
	}
	if valid && recordReady {
		steps = append(steps, "replace <executor-id> before the hash-bound record Apply command")
		steps = append(steps, "record bounded observation evidence only with -ExpectedExecutionReportSha256 from this validation/status envelope")
	}
	if applied || duplicate {
		steps = append(steps, "after record, review outputRefs/evidenceRefs before any authority/confirmed outcome")
	}
	steps = append(steps,
		"keep contract, scaffold/draft, validation, record, and evidence review as separate bounded operations",
		"/rekit does not execute adapter or heavy tool actions from this handoff",
		"do not write authority/confirmed from adapter report lifecycle handoff",
	)
	for _, guard := range boundary {
		guard = strings.TrimSpace(guard)
		if guard == "" {
			continue
		}
		lower := strings.ToLower(guard)
		if strings.Contains(lower, "read-only") || strings.Contains(lower, "does not") || strings.Contains(lower, "never executes") || strings.Contains(lower, "no authority") || strings.Contains(lower, "do not") {
			steps = append(steps, "boundary guard: "+guard)
		}
	}
	return mission.UniqueStrings(steps)
}

func adapterReportLiveValidationRunbookSteps(live AdapterReportLiveValidation) []string {
	steps := []string{"confirm authorized output workspace and adapter-report.json sidecar path before adapter work"}
	if strings.TrimSpace(live.CaseRelativeReportPath) != "" {
		steps = append(steps, "default case-relative report path: "+strings.TrimSpace(live.CaseRelativeReportPath))
	}
	if strings.TrimSpace(live.ScaffoldCommand) != "" {
		steps = append(steps, "preview scaffold if the sidecar is missing: "+strings.TrimSpace(live.ScaffoldCommand))
	}
	if strings.TrimSpace(live.ScaffoldApplyCommand) != "" {
		steps = append(steps, "write only missing exact scaffold with hash-bound Apply: "+strings.TrimSpace(live.ScaffoldApplyCommand))
	}
	if strings.TrimSpace(live.DraftCommand) != "" {
		steps = append(steps, "draft bounded executor-reported sidecar fields: "+strings.TrimSpace(live.DraftCommand))
	}
	if strings.TrimSpace(live.ValidateCommand) != "" {
		steps = append(steps, "run read-only validation before record: "+strings.TrimSpace(live.ValidateCommand))
	}
	if strings.TrimSpace(live.RecordCommand) != "" {
		steps = append(steps, "record only with current hash-bound validation/status command: "+strings.TrimSpace(live.RecordCommand))
	} else {
		steps = append(steps, "record command is intentionally unavailable until validation/status returns valid=true with -ExpectedExecutionReportSha256")
	}
	steps = append(steps,
		"keep scaffold/draft, validation, record, and evidence review as separate bounded operations",
		"/rekit does not execute adapter or heavy tool actions from this handoff",
		"do not write authority/confirmed from adapter report lifecycle handoff",
	)
	return mission.UniqueStrings(steps)
}

func adapterReportHandoffSummary(gateEvent EventPreview, state, reportPath, reportSHA256 string, report *AdapterReport, allowedStatuses, allowedOutputPaths []string, adapterCandidates []AdapterToolCandidate, stopConditions []string, hints []AdapterReportRepairHint, follow AuthorizedExecutionFollowThrough, queue mission.MissionCommanderActionQueue, items []mission.MissionCommanderNextActionItem, valid bool, failureCode, failureStage string) AdapterReportHandoffSummary {
	reportPath = strings.TrimSpace(reportPath)
	reportSHA256 = strings.TrimSpace(reportSHA256)
	summary := AdapterReportHandoffSummary{
		State:                  state,
		GateEventID:            gateEvent.EventID,
		Action:                 gateEvent.Gate.Action,
		Lane:                   gateEvent.Lane,
		ReportPath:             reportPath,
		ReportSHA256:           reportSHA256,
		DefaultReportPath:      adapterReportDefaultPath(gateEvent.Gate.OutputPaths),
		ReportPresent:          report != nil,
		Valid:                  valid,
		RecordReady:            valid && state == "ready-to-record-evidence",
		RecordBlocked:          !valid,
		RequiresValidation:     state == "needs-adapter-report-validation",
		RequiresRepair:         !valid && state != "needs-adapter-report-validation" && len(hints) > 0,
		RequiresMainEscalation: state == "needs-main-escalation",
		AllowedStatusCount:     len(allowedStatuses),
		AllowedOutputPathCount: len(allowedOutputPaths),
		AuthorizedStopCount:    len(stopConditions),
		AdapterCandidateCount:  len(adapterCandidates),
		RepairHintCount:        len(hints),
		OutcomeCount:           len(follow.Outcomes),
		NextActionCount:        len(items),
		ActionQueueSummary:     queue.Summary,
		ValidationFailureCode:  strings.TrimSpace(failureCode),
		ValidationFailureStage: strings.TrimSpace(failureStage),
	}
	if reportPath == "" {
		summary.ReportPath = summary.DefaultReportPath
	}
	for _, hint := range hints {
		if hint.RecordBlocked {
			summary.RecordBlockedHintCount++
		}
		if hint.EscalateToMain {
			summary.EscalateHintCount++
			if state != "needs-adapter-report-validation" {
				summary.RequiresMainEscalation = true
			}
		}
	}
	for _, item := range items {
		if item.RequiresReview {
			summary.ReviewRequiredActionCount++
		}
	}
	if queue.CurrentAction != nil {
		summary.CurrentAction = queue.CurrentAction.Command
	}
	if report != nil {
		summary.ReportStatus = strings.TrimSpace(report.Status)
		summary.AdapterID = strings.TrimSpace(report.AdapterID)
		summary.ActualRuntimeSeconds = report.ActualBudget.RuntimeSeconds
		summary.ActualDiskMB = report.ActualBudget.DiskMB
		summary.ActualRequests = report.ActualBudget.Requests
		summary.OutputRefCount = len(report.OutputRefs)
		summary.EvidenceRefCount = len(report.EvidenceRefs)
		summary.BoundaryHitCount = len(report.BoundaryHits)
		summary.HasEscalation = strings.TrimSpace(report.Escalation) != ""
		summary.HasSummary = strings.TrimSpace(report.Summary) != ""
	}
	if summary.RecordReady && reportSHA256 != "" {
		summary.RecordExpectedReportSHA256 = reportSHA256
	}
	if summary.RequiresValidation || summary.RecordReady || summary.RequiresRepair || summary.RequiresMainEscalation || summary.OutcomeCount > 0 {
		summary.Boundary = []string{
			"adapter report summary is read-only; full contract/validation/follow-through arrays remain available",
			"validation is read-only and must return valid=true before evidence record",
			"record command writes bounded observation evidence only; /rekit does not execute the heavy tool",
			"do not write authority/confirmed",
		}
	}
	return summary
}

func authorizedExecutionFollowThrough(gateEvent EventPreview, state, reportPath string, commander mission.MissionCommanderAction, items []mission.MissionCommanderNextActionItem, hints []AdapterReportRepairHint, valid, duplicate bool) AuthorizedExecutionFollowThrough {
	reportPath = strings.TrimSpace(reportPath)
	if reportPath == "" && (state == "needs-adapter-report-validation" || (!valid && !duplicate)) {
		reportPath = adapterReportDefaultPath(gateEvent.Gate.OutputPaths)
	}
	follow := AuthorizedExecutionFollowThrough{
		State:       state,
		GateEventID: gateEvent.EventID,
		ReportPath:  reportPath,
		Boundary: []string{
			"authorizedExecutionFollowThrough is guidance only; /rekit does not execute the heavy tool",
			"validation is read-only and must return valid=true before evidence record",
			"record command writes bounded observation evidence only",
			"pre-validation contract/scaffold/draft handoffs do not provide runnable bare record Apply; use validation/status returned -ExpectedExecutionReportSha256 after valid=true",
			"do not write authority/confirmed",
		},
	}
	if state == "needs-adapter-report-validation" {
		follow.Outcomes = append(follow.Outcomes,
			AuthorizedExecutionOutcome{
				Name:                 "write-and-validate-report",
				State:                "needs-adapter-report-validation",
				When:                 "after authorized-gate request is recorded and execution report contract is read",
				Command:              commander.PrimaryCommand,
				Actions:              []string{"write bounded adapter execution report sidecar under the authorized outputPath", "run read-only validation", "do not record evidence until validation returns valid=true"},
				VerificationCommands: []string{commander.PrimaryCommand},
				Expected:             "adapter sidecar is validated read-only without observations/authority/confirmed writes",
				Evidence:             []string{"adapter execution report sidecar", "valid=true or valid=false validation envelope"},
				Boundary:             []string{"sidecar refs stay under authorized outputPaths", "validation is read-only"},
			},
			AuthorizedExecutionOutcome{
				Name:     "valid-report-record",
				State:    "ready-to-record-evidence",
				When:     "validation returns valid=true for the bounded sidecar",
				Actions:  []string{"use the hash-bound record command returned by validation/status", "replace <executor-id> in the record command", "record bounded observation evidence", "handoff to Mission Commander evidence review"},
				Expected: "validated sidecar becomes bounded observation evidence with adapter report provenance",
				Evidence: []string{"valid=true validation envelope", "observation evidence row", "executionEvidenceReview handoff"},
				Boundary: []string{"record only after valid=true", "require validation/status returned -ExpectedExecutionReportSha256", "do not replay heavy tool", "do not write authority/confirmed"},
			},
			AuthorizedExecutionOutcome{
				Name:     "invalid-report-repair",
				State:    "repair-adapter-report",
				When:     "validation returns valid=false or report path is missing",
				Actions:  []string{"follow validation repairHints", "rerun read-only validation", "escalate to main Agent if repair hint requires it"},
				Expected: "record remains blocked until the sidecar validates or main Agent handles escalation",
				Evidence: []string{"failureCode/failureStage", "repairHints", "rerun validation output"},
				Boundary: []string{"do not record evidence until validation returns valid=true", "recordBlocked=true repair hints block record"},
			},
		)
	} else if duplicate {
		follow.Outcomes = append(follow.Outcomes, AuthorizedExecutionOutcome{
			Name:     "duplicate-record-review",
			State:    "evidence-already-recorded",
			When:     "record replay returns applied=false reason=duplicate eventId",
			Command:  "/rekit handoff " + gateCommanderActionLabel(gateEvent.Lane),
			Actions:  []string{"do not rerun the external heavy/tool adapter action", "review the existing observation evidence and output/evidence refs", "use /rekit overview for refreshed queue state"},
			Expected: "duplicate replay does not append observations and routes to evidence review only",
			Evidence: []string{"applied=false", "reason=duplicate eventId", "existing observation evidence eventId"},
			Boundary: []string{"do not append duplicate observation evidence", "do not write authority/confirmed"},
		})
	} else if (state == "ready-for-evidence-review" || state == "needs-main-escalation") && len(hints) == 0 {
		outcome := AuthorizedExecutionOutcome{
			Name:     "recorded-evidence-review",
			State:    state,
			When:     "gate -Apply records bounded observation evidence for the authorized gate",
			Command:  commander.PrimaryCommand,
			Actions:  []string{"review outputRefs/evidenceRefs", "confirm no heavy tool was run by /rekit", "decide any later authority/confirmed outcome outside this record path"},
			Expected: "Mission Commander reviews recorded observation evidence before continuation or authority/confirmed decisions",
			Evidence: []string{"observation evidence row", "executionEvidenceReview item", "Mission Commander handoff"},
			Boundary: []string{"observation evidence is already recorded; do not replay heavy tool", "do not write authority/confirmed"},
		}
		if state == "needs-main-escalation" {
			outcome.Name = "boundary-or-escalation-review"
			outcome.Actions = []string{"stop autonomous work on this action", "notify the main Agent", "review boundaryHits/escalation and output/evidence refs"}
			outcome.Expected = "boundary or escalation evidence is reviewed by the main Agent before autonomous continuation"
			outcome.Evidence = append(outcome.Evidence, "boundaryHits or escalation")
			outcome.Boundary = append(outcome.Boundary, "stop autonomous work on this action until main review")
		}
		follow.Outcomes = append(follow.Outcomes, outcome)
	} else if valid {
		follow.Outcomes = append(follow.Outcomes, AuthorizedExecutionOutcome{
			Name:                 "valid-report-record",
			State:                "ready-to-record-evidence",
			When:                 "gate -ValidateExecutionReport returns valid=true for the bounded sidecar",
			Command:              commander.PrimaryCommand,
			Actions:              []string{"replace <executor-id> in the record command", "record bounded observation evidence from the validated sidecar", "handoff to the Mission Commander to review refs before any authority/confirmed outcome"},
			VerificationCommands: []string{commander.PrimaryCommand},
			Expected:             "observation evidence records adapter status, budget, outputRefs/evidenceRefs, and report provenance without executing the heavy tool",
			Evidence:             []string{"valid=true validation envelope", "observation evidence row", "Mission Commander evidence review handoff"},
			Boundary:             []string{"record only after valid=true", "replace <executor-id>", "do not write authority/confirmed"},
		})
	} else {
		repairActions := []string{}
		for _, hint := range hints {
			if strings.TrimSpace(hint.RepairAction) != "" {
				repairActions = append(repairActions, hint.RepairAction)
			}
		}
		follow.Outcomes = append(follow.Outcomes, AuthorizedExecutionOutcome{
			Name:          "invalid-report-repair",
			State:         adapterReportRepairState(hints),
			When:          "gate -ValidateExecutionReport returns valid=false or a report path is missing",
			Command:       commander.PrimaryCommand,
			RepairActions: repairActions,
			Actions:       []string{"repair the bounded adapter execution report sidecar", "rerun read-only validation", "do not record evidence until validation returns valid=true"},
			Expected:      "invalid sidecar is repaired or escalated before any observation evidence write",
			Evidence:      []string{"failureCode/failureStage", "repairHints", "rerun validation output"},
			Boundary:      []string{"recordBlocked=true repair hints block record", "validation remains read-only", "do not write authority/confirmed"},
		})
	}
	follow.ActionQueue = mission.MissionCommanderActionQueueFor(items)
	return follow
}

type ApplyResult struct {
	SchemaVersion                    int                                      `json:"schemaVersion"`
	Command                          string                                   `json:"command"`
	CaseRoot                         string                                   `json:"caseRoot"`
	RepoRoot                         string                                   `json:"repoRoot"`
	Pack                             string                                   `json:"pack"`
	IsMutation                       bool                                     `json:"isMutation"`
	Applied                          bool                                     `json:"applied"`
	EventID                          string                                   `json:"eventId"`
	Path                             string                                   `json:"path"`
	Reason                           string                                   `json:"reason,omitempty"`
	Event                            *EventPreview                            `json:"event,omitempty"`
	ExecutionEvidence                *ExecutionEvidencePreview                `json:"executionEvidence,omitempty"`
	MissionBrief                     mission.Brief                            `json:"missionBrief"`
	ExecutorAction                   mission.ExecutorAction                   `json:"executorAction"`
	MissionCommanderAction           mission.MissionCommanderAction           `json:"missionCommanderAction"`
	ExecutionEvidenceReview          []mission.ExecutionEvidenceReviewItem    `json:"executionEvidenceReview,omitempty"`
	ExecutionEvidenceReviewSummary   mission.ExecutionEvidenceReviewSummary   `json:"executionEvidenceReviewSummary"`
	AuthorizedExecutionFollowThrough AuthorizedExecutionFollowThrough         `json:"authorizedExecutionFollowThrough"`
	MissionCommanderNextActions      []mission.MissionCommanderNextActionItem `json:"missionCommanderNextActions,omitempty"`
	MissionCommanderActionQueue      mission.MissionCommanderActionQueue      `json:"missionCommanderActionQueue"`
	RunbookSteps                     []string                                 `json:"runbookSteps,omitempty"`
	NextSteps                        []string                                 `json:"nextSteps"`
}

type EventPreview struct {
	SchemaVersion int         `json:"schemaVersion"`
	Kind          string      `json:"kind"`
	Lane          string      `json:"lane"`
	Subject       string      `json:"subject"`
	Summary       string      `json:"summary"`
	CreatedAt     string      `json:"createdAt,omitempty"`
	Status        string      `json:"status"`
	Actor         string      `json:"actor,omitempty"`
	Risk          string      `json:"risk,omitempty"`
	Target        string      `json:"target,omitempty"`
	BatchID       string      `json:"batchId,omitempty"`
	Gate          GateDetails `json:"gate"`
	EventID       string      `json:"eventId,omitempty"`
}

type GateDetails struct {
	Action                      string            `json:"action"`
	Scope                       string            `json:"scope,omitempty"`
	Budget                      string            `json:"budget,omitempty"`
	RequestedBudget             autonomy.Budget   `json:"requestedBudget"`
	OutputPaths                 []string          `json:"outputPaths,omitempty"`
	TriedLightSteps             []string          `json:"triedLightSteps,omitempty"`
	StopConditions              []string          `json:"stopConditions,omitempty"`
	RequiresConfirmation        bool              `json:"requiresConfirmation"`
	DeniedUntilUserConfirmation []string          `json:"deniedUntilUserConfirmation"`
	Authorization               autonomy.Decision `json:"authorization"`
}

func PlanDryRun(repoRoot, caseRoot, pack string, opt Options) (Plan, error) {
	inst, preview, blocked, err := buildPreview(repoRoot, caseRoot, pack, opt)
	if err != nil {
		return Plan{}, err
	}
	brief := gateMissionBrief(inst.CaseRoot)
	executorAction := gateExecutorAction(inst.CaseRoot, preview.Lane, brief)
	wouldExecutorAction := gateWouldExecutorAction(inst.CaseRoot, preview, brief)
	if preview.Gate.Authorization.Decision == autonomy.DecisionPreauthorized {
		wouldExecutorAction.MissionCommanderAction = gateAuthorizedApplyCommanderAction(pack, preview)
	} else {
		wouldExecutorAction.MissionCommanderAction = gatePendingApplyCommanderAction(pack, preview)
	}
	commanderNextActions := gateRequestMissionCommanderNextActions(preview.Lane, wouldExecutorAction, preview.Gate.Authorization.Decision == autonomy.DecisionPreauthorized, false)
	return Plan{
		SchemaVersion:               1,
		Command:                     "gate",
		CaseRoot:                    inst.CaseRoot,
		RepoRoot:                    repoRoot,
		Pack:                        pack,
		IsMutation:                  false,
		ReviewRequired:              preview.Gate.RequiresConfirmation,
		RequiresConfirmation:        preview.Gate.RequiresConfirmation,
		EventPreview:                preview,
		MissionBrief:                brief,
		ExecutorAction:              executorAction,
		WouldExecutorAction:         wouldExecutorAction,
		MissionCommanderAction:      wouldExecutorAction.MissionCommanderAction,
		MissionCommanderNextActions: commanderNextActions,
		BlockedActions:              blocked,
		NextSteps:                   planNextSteps(preview),
	}, nil
}

func Apply(repoRoot, caseRoot, pack string, opt Options) (ApplyResult, error) {
	if strings.TrimSpace(opt.Actor) == "" {
		return ApplyResult{}, fmt.Errorf("gate -Apply requires -Actor <recorded-by>; this records who wrote the gate authorization request")
	}
	inst, preview, _, err := buildPreview(repoRoot, caseRoot, pack, opt)
	if err != nil {
		return ApplyResult{}, err
	}
	preview.CreatedAt = time.Now().UTC().Format(time.RFC3339Nano)
	preview.EventID = eventID(preview)
	relPath, _, err := mission.FactPath(inst.CaseRoot, "request")
	if err != nil {
		return ApplyResult{}, err
	}
	known, err := mission.ReadFactEventIDs(inst.CaseRoot, "request")
	if err != nil {
		return ApplyResult{}, err
	}
	exists := known[preview.EventID]
	result := ApplyResult{
		SchemaVersion: 1,
		Command:       "gate",
		CaseRoot:      inst.CaseRoot,
		RepoRoot:      repoRoot,
		Pack:          pack,
		IsMutation:    true,
		Applied:       false,
		EventID:       preview.EventID,
		Path:          relPath,
		Event:         &preview,
		NextSteps:     applyNextSteps(preview),
	}
	if exists {
		result.MissionBrief = gateMissionBrief(inst.CaseRoot)
		result.ExecutorAction = gateExecutorAction(inst.CaseRoot, preview.Lane, result.MissionBrief)
		if preview.Gate.Authorization.Decision == autonomy.DecisionPreauthorized {
			result.MissionCommanderAction = gateAuthorizedRecordedCommanderAction(pack, preview, true)
			result.MissionCommanderNextActions = gateRequestMissionCommanderNextActions(preview.Lane, mission.ExecutorAction{MissionCommanderAction: result.MissionCommanderAction}, true, true)
			result.MissionCommanderActionQueue = mission.MissionCommanderActionQueueFor(result.MissionCommanderNextActions)
		} else {
			result.MissionCommanderAction = result.ExecutorAction.MissionCommanderAction
			result.MissionCommanderNextActions = gateRequestMissionCommanderNextActions(preview.Lane, result.ExecutorAction, false, true)
			result.MissionCommanderActionQueue = mission.MissionCommanderActionQueueFor(result.MissionCommanderNextActions)
		}
		result.Reason = "duplicate eventId"
		return result, nil
	}
	if _, _, err := mission.AppendFact(inst.CaseRoot, "request", preview); err != nil {
		return ApplyResult{}, err
	}
	result.Applied = true
	result.MissionBrief = gateMissionBrief(inst.CaseRoot)
	result.ExecutorAction = gateExecutorAction(inst.CaseRoot, preview.Lane, result.MissionBrief)
	if preview.Gate.Authorization.Decision == autonomy.DecisionPreauthorized {
		result.MissionCommanderAction = gateAuthorizedRecordedCommanderAction(pack, preview, false)
		result.MissionCommanderNextActions = gateRequestMissionCommanderNextActions(preview.Lane, mission.ExecutorAction{MissionCommanderAction: result.MissionCommanderAction}, true, true)
		result.MissionCommanderActionQueue = mission.MissionCommanderActionQueueFor(result.MissionCommanderNextActions)
	} else {
		result.MissionCommanderAction = result.ExecutorAction.MissionCommanderAction
		result.MissionCommanderNextActions = gateRequestMissionCommanderNextActions(preview.Lane, result.ExecutorAction, false, true)
		result.MissionCommanderActionQueue = mission.MissionCommanderActionQueueFor(result.MissionCommanderNextActions)
	}
	return result, nil
}

func RecordExecution(repoRoot, caseRoot, pack string, opt Options) (_ ApplyResult, retErr error) {
	if strings.TrimSpace(opt.Actor) == "" {
		return ApplyResult{}, fmt.Errorf("gate execution evidence requires -Actor <recorded-by>")
	}
	initialInst, initialGateEvent, err := authorizedGateEvent(repoRoot, caseRoot, pack, opt)
	if err != nil {
		return ApplyResult{}, err
	}
	lease, err := acquireGateLaneMutationLease(initialInst.CaseRoot, initialGateEvent.Lane)
	if err != nil {
		return ApplyResult{}, err
	}
	defer func() { retErr = errors.Join(retErr, lease.Unlock()) }()
	inst, gateEvent, err := authorizedGateEvent(repoRoot, caseRoot, pack, opt)
	if err != nil {
		return ApplyResult{}, err
	}
	if inst.CaseRoot != initialInst.CaseRoot || gateEvent.EventID != initialGateEvent.EventID || gateEvent.Lane != initialGateEvent.Lane {
		return ApplyResult{}, fmt.Errorf("authorized gate routing changed while acquiring mutation lease")
	}
	m, err := manifest.Load(repoRoot, pack)
	if err != nil {
		return ApplyResult{}, err
	}
	execution, err := executionEvidence(inst.CaseRoot, gateEvent, opt, m)
	if err != nil {
		return ApplyResult{}, err
	}
	execution.CreatedAt = time.Now().UTC().Format(time.RFC3339Nano)
	execution.EventID = executionEventID(execution)
	relPath, _, err := mission.FactPath(inst.CaseRoot, "observation")
	if err != nil {
		return ApplyResult{}, err
	}
	if err := lease.Validate(); err != nil {
		return ApplyResult{}, err
	}
	known, err := mission.ReadFactEventIDs(inst.CaseRoot, "observation")
	if err != nil {
		return ApplyResult{}, err
	}
	result := ApplyResult{
		SchemaVersion:     1,
		Command:           "gate",
		CaseRoot:          inst.CaseRoot,
		RepoRoot:          repoRoot,
		Pack:              pack,
		IsMutation:        true,
		Applied:           false,
		EventID:           execution.EventID,
		Path:              relPath,
		ExecutionEvidence: &execution,
		NextSteps:         executionNextSteps(execution),
	}
	if known[execution.EventID] {
		result.MissionBrief = gateMissionBrief(inst.CaseRoot)
		result.ExecutorAction = gateExecutorAction(inst.CaseRoot, execution.Lane, result.MissionBrief)
		result.MissionCommanderAction = executionCommanderAction(execution, result.Applied, true)
		result.ExecutionEvidenceReview = gateDuplicateExecutionEvidenceReviewFromObservation(execution, result.MissionCommanderAction)
		result.MissionCommanderNextActions = gateMissionCommanderNextActions(execution.Lane, mission.ExecutorAction{}, result.ExecutionEvidenceReview)
		result.MissionCommanderActionQueue = mission.MissionCommanderActionQueueFor(result.MissionCommanderNextActions)
		result.ExecutionEvidenceReviewSummary = mission.ExecutionEvidenceReviewSummaryFor(result.ExecutionEvidenceReview, result.MissionCommanderActionQueue)
		result.AuthorizedExecutionFollowThrough = authorizedExecutionFollowThrough(gateEvent, result.MissionCommanderAction.State, execution.Execution.ExecutionReportPath, result.MissionCommanderAction, result.MissionCommanderNextActions, nil, false, true)
		result.RunbookSteps = adapterReportRunbookSteps("record", result.MissionCommanderAction.State, execution.Execution.ExecutionReportPath, execution.Execution.ExecutionReportSHA256, true, false, result.Applied, true, result.NextSteps, result.MissionCommanderAction.Boundary, result.MissionCommanderAction)
		result.Reason = "duplicate eventId"
		return result, nil
	}
	if _, _, err := mission.AppendFact(inst.CaseRoot, "observation", execution); err != nil {
		return ApplyResult{}, err
	}
	if err := lease.Validate(); err != nil {
		return ApplyResult{}, err
	}
	result.Applied = true
	result.MissionBrief = gateMissionBrief(inst.CaseRoot)
	result.ExecutorAction = gateExecutorAction(inst.CaseRoot, execution.Lane, result.MissionBrief)
	result.MissionCommanderAction = executionCommanderAction(execution, result.Applied, false)
	result.ExecutionEvidenceReview = gateExecutionEvidenceReviewFromObservation(execution)
	result.MissionCommanderNextActions = gateMissionCommanderNextActions(execution.Lane, result.ExecutorAction, result.ExecutionEvidenceReview)
	result.MissionCommanderActionQueue = mission.MissionCommanderActionQueueFor(result.MissionCommanderNextActions)
	result.ExecutionEvidenceReviewSummary = mission.ExecutionEvidenceReviewSummaryFor(result.ExecutionEvidenceReview, result.MissionCommanderActionQueue)
	result.AuthorizedExecutionFollowThrough = authorizedExecutionFollowThrough(gateEvent, result.MissionCommanderAction.State, execution.Execution.ExecutionReportPath, result.MissionCommanderAction, result.MissionCommanderNextActions, nil, true, false)
	result.RunbookSteps = adapterReportRunbookSteps("record", result.MissionCommanderAction.State, execution.Execution.ExecutionReportPath, execution.Execution.ExecutionReportSHA256, true, false, result.Applied, false, result.NextSteps, result.MissionCommanderAction.Boundary, result.MissionCommanderAction)
	return result, nil
}

func authorizedGateEvent(repoRoot, caseRoot, pack string, opt Options) (instance.Instance, EventPreview, error) {
	inst, err := instance.AssertAttached(caseRoot, repoRoot, pack)
	if err != nil {
		return instance.Instance{}, EventPreview{}, err
	}
	event, err := findAuthorizedGateEvent(inst.CaseRoot, strings.TrimSpace(opt.GateEventID))
	if err != nil {
		return instance.Instance{}, EventPreview{}, err
	}
	return inst, event, nil
}

func AdapterReportContract(repoRoot, caseRoot, pack string, opt Options) (AdapterExecutionReportContract, error) {
	inst, err := instance.AssertAttached(caseRoot, repoRoot, pack)
	if err != nil {
		return AdapterExecutionReportContract{}, err
	}
	event, err := findAuthorizedGateEvent(inst.CaseRoot, strings.TrimSpace(opt.GateEventID))
	if err != nil {
		return AdapterExecutionReportContract{}, err
	}
	m, err := manifest.Load(repoRoot, pack)
	if err != nil {
		return AdapterExecutionReportContract{}, err
	}
	return adapterReportContract(repoRoot, inst.CaseRoot, pack, event, m), nil
}

func AdapterReportLiveSnapshot(repoRoot, caseRoot, pack string, opt Options) (AdapterExecutionReportValidation, bool, error) {
	reportPath := strings.TrimSpace(opt.ExecutionReportPath)
	if reportPath == "" {
		return AdapterExecutionReportValidation{}, false, nil
	}
	fullPath, _, err := executionReportPath(caseRoot, reportPath)
	if err != nil {
		return AdapterExecutionReportValidation{}, false, err
	}
	if _, err := os.Lstat(fullPath); os.IsNotExist(err) {
		return AdapterExecutionReportValidation{}, false, nil
	} else if err != nil {
		return AdapterExecutionReportValidation{}, true, err
	}
	inst, gateEvent, err := authorizedGateEvent(repoRoot, caseRoot, pack, opt)
	if err != nil {
		return AdapterExecutionReportValidation{}, true, err
	}
	m, err := manifest.Load(repoRoot, pack)
	if err != nil {
		return AdapterExecutionReportValidation{}, true, err
	}
	contract := adapterReportContract(repoRoot, inst.CaseRoot, pack, gateEvent, m)
	if existing, present, err := readAdapterReportRaw(inst.CaseRoot, fullPath, reportPath); err != nil {
		return AdapterExecutionReportValidation{}, true, err
	} else if present {
		scaffoldData, scaffoldErr := adapterReportScaffoldBytes(contract.LiveValidation.SidecarTemplate)
		if scaffoldErr != nil {
			return AdapterExecutionReportValidation{}, true, scaffoldErr
		}
		if bytes.Equal(existing, scaffoldData) {
			return adapterReportScaffoldLiveSnapshot(repoRoot, inst.CaseRoot, pack, gateEvent, contract, reportPath), true, nil
		}
	}
	opt.ExecutionReportPath = reportPath
	validation, err := ValidateAdapterExecutionReport(repoRoot, caseRoot, pack, opt)
	validation.ReportSummary.ReportPresent = true
	if err != nil || !validation.Valid {
		return validation, true, err
	}
	recorded, err := adapterReportEvidenceRecorded(caseRoot, validation.GateEventID, validation.ReportPath, validation.Report)
	if err != nil {
		return AdapterExecutionReportValidation{}, true, err
	}
	if recorded {
		applyRecordedAdapterReportSnapshot(&validation)
	}
	return validation, true, nil
}

func adapterReportScaffoldLiveSnapshot(repoRoot, caseRoot, pack string, gateEvent EventPreview, contract AdapterExecutionReportContract, reportPath string) AdapterExecutionReportValidation {
	reportPath = strings.TrimSpace(reportPath)
	if reportPath == "" {
		reportPath = contract.LiveValidation.CaseRelativeReportPath
	}
	validateCommand := adapterReportValidateSlashCommand(pack, gateEvent.EventID, reportPath)
	draftCommand := adapterReportDraftSlashCommand(pack, gateEvent.EventID, reportPath)
	label := gateCommanderActionLabel(gateEvent.Lane)
	boundary := []string{
		"exact scaffold is present; do not record it as execution evidence",
		"fill bounded execution fields with gate -DraftExecutionReport or edit manually after external adapter completes",
		"validation remains read-only and must return valid=true before evidence record",
		"/rekit never executes the heavy tool",
		"no observations/authority/confirmed writes",
	}
	commander := mission.MissionCommanderAction{
		State:          "adapter-report-scaffold-awaiting-draft",
		Prompt:         fmt.Sprintf("authorized gate `%s` has an exact adapter-report.json scaffold; fill bounded execution fields before validation or record.", gateEvent.EventID),
		PrimaryCommand: draftCommand,
		FollowUpCommands: []string{
			validateCommand,
			"/rekit handoff " + label,
		},
		Boundary: boundary,
	}
	items := []mission.MissionCommanderNextActionItem{
		adapterReportNextActionItem(gateEvent, label, "adapter-report-live-scaffold-draft", commander.State, draftCommand, "adapterReportLiveSnapshot.scaffold.draft", false, true, []string{"exact scaffold is present and still needs bounded execution fields"}, boundary),
		adapterReportNextActionItem(gateEvent, label, "adapter-report-live-scaffold-validation", commander.State, validateCommand, "adapterReportLiveSnapshot.scaffold.validation", true, true, []string{"validation will fail until placeholder fields are replaced by bounded execution values"}, boundary),
		adapterReportNextActionItem(gateEvent, label, "adapter-report-live-scaffold-handoff", commander.State, "/rekit handoff "+label, "adapterReportLiveSnapshot.scaffold.followUp", false, true, []string{"handoff scaffold and draft expectations"}, boundary),
	}
	queue := mission.MissionCommanderActionQueueFor(items)
	follow := AuthorizedExecutionFollowThrough{
		State:       commander.State,
		GateEventID: gateEvent.EventID,
		ReportPath:  reportPath,
		Outcomes: []AuthorizedExecutionOutcome{{
			Name:                 "scaffold-fill-and-validate",
			State:                commander.State,
			When:                 "after the exact adapter-report.json scaffold exists but before bounded execution fields are drafted",
			Command:              draftCommand,
			Actions:              []string{"fill bounded execution fields", "run read-only validation", "record only after valid=true"},
			VerificationCommands: []string{draftCommand, validateCommand},
			Expected:             "scaffold becomes a bounded execution report draft before validation/record",
			Evidence:             []string{"exact scaffold sidecar", "authorized-gate ledger event"},
			Boundary:             append([]string{}, boundary...),
		}},
		Boundary:    append([]string{}, boundary...),
		ActionQueue: queue,
	}
	validation := AdapterExecutionReportValidation{
		SchemaVersion:                    1,
		Command:                          "gate",
		Kind:                             "adapter-execution-report-validation",
		CaseRoot:                         caseRoot,
		RepoRoot:                         repoRoot,
		Pack:                             pack,
		IsMutation:                       false,
		Applied:                          false,
		Valid:                            false,
		Error:                            "adapter execution report is still the exact scaffold template",
		Errors:                           []string{"adapter execution report is still the exact scaffold template"},
		FailureCode:                      "report-scaffold-placeholder",
		FailureStage:                     "schema",
		GateEventID:                      gateEvent.EventID,
		ReportPath:                       reportPath,
		Contract:                         contract,
		MissionCommanderAction:           commander,
		MissionCommanderNextActions:      items,
		MissionCommanderActionQueue:      queue,
		AuthorizedExecutionFollowThrough: follow,
		NextSteps:                        []string{"fill bounded execution fields with gate -DraftExecutionReport or manual adapter output", "rerun read-only validation before recording evidence", "do not record the scaffold template"},
	}
	validation.ReportSummary = adapterReportHandoffSummary(gateEvent, commander.State, reportPath, "", nil, contract.AllowedStatuses, contract.AllowedOutputPaths, contract.LiveValidation.AdapterCandidates, contract.StopConditions, nil, follow, queue, items, false, validation.FailureCode, validation.FailureStage)
	validation.ReportSummary.ReportPresent = true
	validation.ReportSummary.RecordBlocked = true
	validation.ReportSummary.RequiresValidation = false
	validation.ReportSummary.RequiresRepair = true
	validation.ReportSummary.Boundary = append([]string{}, boundary...)
	validation.RunbookSteps = adapterReportRunbookSteps("live scaffold", commander.State, validation.ReportSummary.ReportPath, validation.ReportSHA256, false, validation.ReportSummary.RecordReady, false, false, validation.NextSteps, boundary, commander)
	return validation
}

func applyRecordedAdapterReportSnapshot(validation *AdapterExecutionReportValidation) {
	if validation == nil || validation.Report == nil {
		return
	}
	label := gateCommanderActionLabel(validation.ReportSummary.Lane)
	handoffCommand := "/rekit handoff " + label
	needsMainReview := validation.Report.Status == "boundary-hit" || validation.Report.Status == "escalated" || len(validation.Report.BoundaryHits) > 0 || strings.TrimSpace(validation.Report.Escalation) != ""
	state := "evidence-already-recorded"
	prompt := fmt.Sprintf("authorized gate `%s` 的 exact observation evidence 已记录；只 review output/evidence refs，不要再次 record 或 replay。", validation.GateEventID)
	boundary := []string{
		"observation evidence is already recorded; do not replay heavy tool or adapter action",
		"do not append duplicate observation evidence",
		"review outputRefs/evidenceRefs before any authority/confirmed outcome",
		"no authority/confirmed writes",
	}
	nextSteps := []string{"review the recorded observation evidence; do not record or replay the adapter report again", handoffCommand}
	if needsMainReview {
		state = "needs-main-escalation"
		prompt = fmt.Sprintf("authorized gate `%s` 的 exact observation evidence记录了boundary/escalation；停止该action的自主推进并通知main Agent。", validation.GateEventID)
		boundary = append(boundary, "stop autonomous work on this action until main review")
		nextSteps = []string{"stop autonomous continuation; recorded boundary/escalation requires main Agent review", "review boundaryHits/escalation and output/evidence refs", handoffCommand}
	}
	commander := mission.MissionCommanderAction{
		State:            state,
		Prompt:           prompt,
		PrimaryCommand:   handoffCommand,
		FollowUpCommands: []string{"/rekit overview"},
		Boundary:         boundary,
	}
	items := []mission.MissionCommanderNextActionItem{
		{
			Lane:           validation.ReportSummary.Lane,
			Label:          label,
			GateEventID:    validation.GateEventID,
			ActionID:       adapterReportActionID(validation.GateEventID, "recorded-evidence-review"),
			State:          state,
			Command:        handoffCommand,
			Source:         "adapterReportLiveSnapshot.recordedEvidence",
			Blocked:        needsMainReview,
			RequiresReview: true,
			Reasons:        append([]string{}, nextSteps[:len(nextSteps)-1]...),
			Boundary:       append([]string{}, boundary...),
		},
		{
			Lane:           validation.ReportSummary.Lane,
			Label:          label,
			GateEventID:    validation.GateEventID,
			ActionID:       adapterReportActionID(validation.GateEventID, "recorded-evidence-overview"),
			State:          state,
			Command:        "/rekit overview",
			Source:         "adapterReportLiveSnapshot.recordedEvidence.followUp",
			Blocked:        needsMainReview,
			RequiresReview: true,
			Reasons:        []string{"refresh Mission Commander evidence review queue"},
			Boundary:       append([]string{}, boundary...),
		},
	}
	queue := mission.MissionCommanderActionQueueFor(items)
	outcomeName := "duplicate-record-review"
	outcomeActions := []string{"do not record or replay the adapter report again", "review the existing observation evidence and output/evidence refs"}
	outcomeExpected := "exact recorded evidence routes to review without exposing a record action"
	if needsMainReview {
		outcomeName = "boundary-or-escalation-review"
		outcomeActions = []string{"stop autonomous continuation for this action", "notify main Agent", "review boundaryHits/escalation and output/evidence refs"}
		outcomeExpected = "main Agent reviews recorded boundary/escalation before autonomous continuation"
	}
	follow := AuthorizedExecutionFollowThrough{
		State:       state,
		GateEventID: validation.GateEventID,
		ReportPath:  validation.ReportPath,
		Outcomes: []AuthorizedExecutionOutcome{{
			Name:                 outcomeName,
			State:                state,
			When:                 "the canonical sidecar exactly matches already recorded observation evidence",
			Command:              handoffCommand,
			Actions:              outcomeActions,
			VerificationCommands: []string{handoffCommand, "/rekit overview"},
			Expected:             outcomeExpected,
			Evidence:             []string{"existing observation evidence", "adapter execution report sidecar"},
			Boundary:             append([]string{}, boundary...),
		}},
		Boundary:    append([]string{}, boundary...),
		ActionQueue: queue,
	}
	validation.MissionCommanderAction = commander
	validation.MissionCommanderNextActions = items
	validation.MissionCommanderActionQueue = queue
	validation.AuthorizedExecutionFollowThrough = follow
	validation.NextSteps = nextSteps
	validation.ReportSummary.State = state
	validation.ReportSummary.RecordReady = false
	validation.ReportSummary.RecordBlocked = true
	validation.ReportSummary.RequiresValidation = false
	validation.ReportSummary.RequiresRepair = false
	validation.ReportSummary.RequiresMainEscalation = needsMainReview
	validation.ReportSummary.OutcomeCount = len(follow.Outcomes)
	validation.ReportSummary.NextActionCount = len(items)
	validation.ReportSummary.ReviewRequiredActionCount = len(items)
	validation.ReportSummary.ActionQueueSummary = queue.Summary
	validation.ReportSummary.CurrentAction = handoffCommand
	validation.ReportSummary.Boundary = append([]string{}, boundary...)
	validation.RunbookSteps = adapterReportRunbookSteps("recorded evidence", state, validation.ReportSummary.ReportPath, validation.ReportSHA256, validation.Valid, validation.ReportSummary.RecordReady, true, true, validation.NextSteps, boundary, commander)
}

func adapterReportEvidenceRecorded(caseRoot, gateEventID, reportPath string, report *AdapterReport) (bool, error) {
	if report == nil {
		return false, nil
	}
	observations, err := mission.ReadStrictFact(caseRoot, "observation")
	if err != nil {
		return false, err
	}
	for _, observation := range observations {
		items := mission.ExecutionEvidenceReviewItems([]map[string]any{observation}, "", nil, 0)
		if len(items) != 1 {
			continue
		}
		item := items[0]
		recordedReport, ok := adapterReportFromObservation(observation)
		if ok && adapterReportMatchesRecordedEvidence(item, gateEventID, reportPath, report, recordedReport) {
			return true, nil
		}
	}
	return false, nil
}

func adapterReportMatchesRecordedEvidence(item mission.ExecutionEvidenceReviewItem, gateEventID, reportPath string, report, recordedReport *AdapterReport) bool {
	if item.GateEventID != gateEventID || filepath.Clean(filepath.FromSlash(item.ExecutionReportPath)) != filepath.Clean(filepath.FromSlash(reportPath)) {
		return false
	}
	if item.AdapterID != report.AdapterID || item.AdapterStatus != report.Status || item.Status != report.Status || item.Escalation != report.Escalation {
		return false
	}
	if item.ActualBudget == nil || item.ActualBudget.RuntimeSeconds != report.ActualBudget.RuntimeSeconds || item.ActualBudget.DiskMB != report.ActualBudget.DiskMB || item.ActualBudget.Requests != report.ActualBudget.Requests {
		return false
	}
	if !reflect.DeepEqual(normalizedGatePaths(item.OutputRefs), normalizedGatePaths(report.OutputRefs)) || !reflect.DeepEqual(normalizedGatePaths(item.EvidenceRefs), normalizedGatePaths(report.EvidenceRefs)) || !reflect.DeepEqual(item.BoundaryHits, report.BoundaryHits) {
		return false
	}
	return recordedReport.SchemaVersion == report.SchemaVersion &&
		recordedReport.Kind == report.Kind &&
		recordedReport.AdapterID == report.AdapterID &&
		recordedReport.Action == report.Action &&
		recordedReport.Status == report.Status &&
		recordedReport.GateEventID == report.GateEventID &&
		recordedReport.ActualBudget == report.ActualBudget &&
		reflect.DeepEqual(normalizedGatePaths(recordedReport.OutputRefs), normalizedGatePaths(report.OutputRefs)) &&
		reflect.DeepEqual(normalizedGatePaths(recordedReport.EvidenceRefs), normalizedGatePaths(report.EvidenceRefs)) &&
		reflect.DeepEqual(recordedReport.BoundaryHits, report.BoundaryHits) &&
		recordedReport.Escalation == report.Escalation &&
		recordedReport.Summary == report.Summary
}

func adapterReportFromObservation(observation map[string]any) (*AdapterReport, bool) {
	execution, ok := observation["execution"].(map[string]any)
	if !ok {
		return nil, false
	}
	value, ok := execution["adapter"]
	if !ok || value == nil {
		return nil, false
	}
	data, err := json.Marshal(value)
	if err != nil {
		return nil, false
	}
	var report AdapterReport
	dec := json.NewDecoder(strings.NewReader(string(data)))
	dec.DisallowUnknownFields()
	if err := dec.Decode(&report); err != nil {
		return nil, false
	}
	return &report, true
}

func ValidateAdapterExecutionReport(repoRoot, caseRoot, pack string, opt Options) (AdapterExecutionReportValidation, error) {
	inst, gateEvent, err := authorizedGateEvent(repoRoot, caseRoot, pack, opt)
	if err != nil {
		return AdapterExecutionReportValidation{}, err
	}
	m, err := manifest.Load(repoRoot, pack)
	if err != nil {
		return AdapterExecutionReportValidation{}, err
	}
	adapterCandidates := adapterToolCandidates(m, gateEvent)
	validation := AdapterExecutionReportValidation{
		SchemaVersion: 1,
		Command:       "gate",
		Kind:          "adapter-execution-report-validation",
		CaseRoot:      inst.CaseRoot,
		RepoRoot:      repoRoot,
		Pack:          pack,
		IsMutation:    false,
		Applied:       false,
		GateEventID:   gateEvent.EventID,
		Contract:      adapterReportContract(repoRoot, inst.CaseRoot, pack, gateEvent, m),
	}
	reportRel, reportSHA256, adapterReport, err := readAdapterExecutionReport(inst.CaseRoot, gateEvent, opt.ExecutionReportCwd, opt.ExecutionReportPath)
	if reportRel != "" {
		validation.ReportPath = reportRel
	}
	if reportSHA256 != "" {
		validation.ReportSHA256 = reportSHA256
	}
	if adapterReport != nil {
		validation.Report = adapterReport
	}
	if len(adapterCandidates) > 0 || adapterReport != nil {
		validation.AdapterContext = &AdapterContext{Candidates: adapterCandidates}
		if adapterReport != nil {
			validation.AdapterContext.Selected = selectedAdapterToolCandidate(m, gateEvent, adapterReport.AdapterID)
		}
	}
	if err != nil {
		validation.Valid = false
		validation.Error = err.Error()
		validation.Errors = []string{err.Error()}
		if validationErr, ok := err.(*adapterReportValidationError); ok {
			validation.FailureCode = validationErr.Code
			validation.FailureStage = validationErr.Stage
		}
		validation.RepairHints = adapterReportRepairHints(gateEvent, validation.FailureCode, validation.FailureStage)
		validation.MissionCommanderAction = adapterReportValidationCommanderAction(pack, gateEvent, validation.ReportPath, validation.RecordExpectedReportSHA256, "", nil, false, validation.RepairHints)
		validation.MissionCommanderNextActions = adapterReportValidationCommanderNextActions(gateEvent, validation.MissionCommanderAction, false, validation.RepairHints)
		validation.MissionCommanderActionQueue = mission.MissionCommanderActionQueueFor(validation.MissionCommanderNextActions)
		validation.AuthorizedExecutionFollowThrough = authorizedExecutionFollowThrough(gateEvent, validation.MissionCommanderAction.State, validation.ReportPath, validation.MissionCommanderAction, validation.MissionCommanderNextActions, validation.RepairHints, false, false)
		validation.ReportSummary = adapterReportHandoffSummary(gateEvent, validation.MissionCommanderAction.State, validation.ReportPath, validation.ReportSHA256, validation.Report, validation.Contract.AllowedStatuses, validation.Contract.AllowedOutputPaths, adapterCandidates, validation.Contract.StopConditions, validation.RepairHints, validation.AuthorizedExecutionFollowThrough, validation.MissionCommanderActionQueue, validation.MissionCommanderNextActions, false, validation.FailureCode, validation.FailureStage)
		validation.NextSteps = adapterReportRepairNextSteps(validation.RepairHints)
		validation.RunbookSteps = adapterReportRunbookSteps("validation", validation.MissionCommanderAction.State, validation.ReportSummary.ReportPath, validation.ReportSHA256, false, validation.ReportSummary.RecordReady, false, false, validation.NextSteps, validation.MissionCommanderAction.Boundary, validation.MissionCommanderAction)
		return validation, nil
	}
	if adapterReport == nil {
		validation.Valid = false
		validation.Error = "gate execution report validation requires -ExecutionReportPath"
		validation.Errors = []string{validation.Error}
		validation.RepairHints = adapterReportMissingPathRepairHints(gateEvent)
		validation.MissionCommanderAction = adapterReportValidationCommanderAction(pack, gateEvent, validation.ReportPath, validation.RecordExpectedReportSHA256, "", nil, false, validation.RepairHints)
		validation.MissionCommanderNextActions = adapterReportValidationCommanderNextActions(gateEvent, validation.MissionCommanderAction, false, validation.RepairHints)
		validation.MissionCommanderActionQueue = mission.MissionCommanderActionQueueFor(validation.MissionCommanderNextActions)
		validation.AuthorizedExecutionFollowThrough = authorizedExecutionFollowThrough(gateEvent, validation.MissionCommanderAction.State, validation.ReportPath, validation.MissionCommanderAction, validation.MissionCommanderNextActions, validation.RepairHints, false, false)
		validation.ReportSummary = adapterReportHandoffSummary(gateEvent, validation.MissionCommanderAction.State, validation.ReportPath, validation.ReportSHA256, validation.Report, validation.Contract.AllowedStatuses, validation.Contract.AllowedOutputPaths, adapterCandidates, validation.Contract.StopConditions, validation.RepairHints, validation.AuthorizedExecutionFollowThrough, validation.MissionCommanderActionQueue, validation.MissionCommanderNextActions, false, validation.FailureCode, validation.FailureStage)
		validation.NextSteps = adapterReportRepairNextSteps(validation.RepairHints)
		validation.RunbookSteps = adapterReportRunbookSteps("validation", validation.MissionCommanderAction.State, validation.ReportSummary.ReportPath, validation.ReportSHA256, false, validation.ReportSummary.RecordReady, false, false, validation.NextSteps, validation.MissionCommanderAction.Boundary, validation.MissionCommanderAction)
		return validation, nil
	}
	validation.Valid = true
	validation.RecordExpectedReportSHA256 = validation.ReportSHA256
	validation.ReceiptRequired, err = adapterExecutionReceiptRequired(inst.CaseRoot, gateEvent, m)
	if err != nil {
		validation.Valid = false
		validation.Error = err.Error()
		validation.Errors = []string{validation.Error}
		validation.FailureStage = "provenance"
		validation.FailureCode = "adapter-execution-catalog-invalid"
		validation.MissionCommanderAction = adapterReportValidationCommanderAction(pack, gateEvent, validation.ReportPath, validation.RecordExpectedReportSHA256, "", nil, false, nil)
		validation.MissionCommanderNextActions = adapterReportValidationCommanderNextActions(gateEvent, validation.MissionCommanderAction, false, nil)
		validation.MissionCommanderActionQueue = mission.MissionCommanderActionQueueFor(validation.MissionCommanderNextActions)
		validation.ReportSummary = adapterReportHandoffSummary(gateEvent, validation.MissionCommanderAction.State, validation.ReportPath, validation.ReportSHA256, validation.Report, validation.Contract.AllowedStatuses, validation.Contract.AllowedOutputPaths, adapterCandidates, validation.Contract.StopConditions, nil, AuthorizedExecutionFollowThrough{}, validation.MissionCommanderActionQueue, validation.MissionCommanderNextActions, false, validation.FailureCode, validation.FailureStage)
		validation.ReportSummary.RecordReady = false
		validation.ReportSummary.RecordBlocked = true
		validation.NextSteps = []string{"repair the declared tooling catalog", "rerun read-only report validation", "do not record observation evidence while adapter capability provenance is invalid"}
		return validation, nil
	}
	if validation.ReceiptRequired {
		dispatch, dispatchPath, dispatchSHA, _, dispatchErr := readCurrentAdapterExecutionDispatch(inst.CaseRoot, pack, gateEvent, m)
		if dispatchErr != nil {
			validation.Valid = false
			validation.ProvenanceValid = false
			validation.Error = dispatchErr.Error()
			validation.Errors = []string{validation.Error}
			validation.FailureStage = "provenance"
			validation.FailureCode = "adapter-execution-dispatch-invalid"
			reauthorization := gateEvent
			reauthorization.EventID = ""
			reauthorization.BatchID = gateEvent.EventID + "-dispatch-retry"
			reauthorization.Subject = gateEvent.Subject + " dispatch retry"
			reauthorization.Summary = fmt.Sprintf("Review a distinct authorized execution attempt because gate %s has a report without a valid pre-execution dispatch", gateEvent.EventID)
			validation.MissionCommanderAction = mission.MissionCommanderAction{State: "blocked-by-adapter-execution-provenance-drift", Prompt: fmt.Sprintf("authorized gate `%s` 已有 report 但 immutable pre-execution dispatch 缺失或漂移；不能事后补写 dispatch。", gateEvent.EventID), PrimaryCommand: gateRequestWhatIfSlashCommand(pack, reauthorization), Boundary: append(adapterReportCommanderBoundary(), "do not backfill dispatch after external execution")}
			validation.MissionCommanderNextActions = []mission.MissionCommanderNextActionItem{adapterReportNextActionItem(gateEvent, gateCommanderActionLabel(gateEvent.Lane), "adapter-execution-dispatch-missing", validation.MissionCommanderAction.State, validation.MissionCommanderAction.PrimaryCommand, "adapterExecutionDispatch.validation", true, true, []string{"authorize a distinct gate before any retry"}, validation.MissionCommanderAction.Boundary)}
			validation.MissionCommanderActionQueue = mission.MissionCommanderActionQueueFor(validation.MissionCommanderNextActions)
			validation.ReportSummary = adapterReportHandoffSummary(gateEvent, validation.MissionCommanderAction.State, validation.ReportPath, validation.ReportSHA256, validation.Report, validation.Contract.AllowedStatuses, validation.Contract.AllowedOutputPaths, adapterCandidates, validation.Contract.StopConditions, nil, AuthorizedExecutionFollowThrough{}, validation.MissionCommanderActionQueue, validation.MissionCommanderNextActions, false, validation.FailureCode, validation.FailureStage)
			validation.ReportSummary.RecordReady = false
			validation.ReportSummary.RecordBlocked = true
			validation.NextSteps = []string{"run the returned WhatIf command for a distinct gate", "record its dispatch before external execution", "do not run completion or observation for the unbound report"}
			return validation, nil
		}
		validation.AdapterExecutionDispatchPath = dispatchPath
		validation.AdapterExecutionDispatchSHA256 = dispatchSHA
		validation.AdapterExecutionDispatch = &dispatch
		validation.ReceiptPreviewCommand = adapterExecutionReceiptPreviewSlashCommand(pack, gateEvent, validation.ReportPath, adapterReport.AdapterID)
		receipt, receiptPath, receiptSHA, receiptErr := validateRecordedAdapterExecutionReceipt(inst.CaseRoot, pack, gateEvent, validation.ReportPath, validation.ReportSHA256, adapterReport, m, opt)
		validation.AdapterExecutionReceiptPath = receiptPath
		validation.AdapterExecutionReceiptSHA256 = receiptSHA
		receiptPresent, existsErr := adapterExecutionReceiptExists(inst.CaseRoot, gateEvent)
		validation.ReceiptPresent = receiptPresent
		if receiptErr == nil && existsErr != nil {
			receiptErr = existsErr
		}
		if receiptErr != nil {
			validation.Valid = false
			validation.ProvenanceValid = false
			validation.Error = receiptErr.Error()
			validation.Errors = []string{validation.Error}
			validation.FailureStage = "provenance"
			validation.FailureCode = "adapter-execution-receipt-invalid"
			if validation.ReceiptPresent {
				validation.ReceiptPreviewCommand = ""
				reauthorization := gateEvent
				reauthorization.EventID = ""
				reauthorization.BatchID = gateEvent.EventID + "-provenance-retry"
				reauthorization.Subject = gateEvent.Subject + " provenance retry"
				reauthorization.Summary = fmt.Sprintf("Review a new authorized execution attempt after immutable provenance drift for gate %s; do not adopt or overwrite the existing receipt", gateEvent.EventID)
				validation.MissionCommanderAction = mission.MissionCommanderAction{State: "blocked-by-adapter-execution-provenance-drift", Prompt: fmt.Sprintf("authorized gate `%s` 已有 immutable execution receipt，但 current owner/catalog/report/artifact 与该 receipt 不再一致；不能用新 preview 覆盖 canonical receipt。", gateEvent.EventID), PrimaryCommand: gateRequestWhatIfSlashCommand(pack, reauthorization), Boundary: append(adapterReportCommanderBoundary(), "existing immutable receipt cannot be replaced or adopted after provenance drift")}
				validation.MissionCommanderNextActions = []mission.MissionCommanderNextActionItem{adapterReportNextActionItem(gateEvent, gateCommanderActionLabel(gateEvent.Lane), "adapter-execution-provenance-drift", validation.MissionCommanderAction.State, validation.MissionCommanderAction.PrimaryCommand, "adapterExecutionReceipt.validation", true, true, []string{"restore the exact receipt-bound sidecar/artifact bytes or review a distinct new authorized gate attempt"}, validation.MissionCommanderAction.Boundary)}
				validation.NextSteps = []string{"restore the exact receipt-bound report and artifact bytes when the drift was accidental", "otherwise run the returned WhatIf command, review its distinct provenance-retry identity, then use its Apply command before any new adapter execution", "do not overwrite the existing receipt or record current drifted bytes as observation evidence"}
			} else {
				validation.MissionCommanderAction = mission.MissionCommanderAction{State: "ready-for-adapter-execution-receipt-preview", Prompt: fmt.Sprintf("authorized gate `%s` report 已通过 schema/boundary 校验，但 durable execution provenance 尚未完成；先记录 current owner/catalog/artifact receipt。", gateEvent.EventID), PrimaryCommand: validation.ReceiptPreviewCommand, Boundary: append(adapterReportCommanderBoundary(), "observation record is blocked until receipt provenance is valid")}
				validation.MissionCommanderNextActions = []mission.MissionCommanderNextActionItem{adapterReportNextActionItem(gateEvent, gateCommanderActionLabel(gateEvent.Lane), "adapter-execution-receipt-preview", validation.MissionCommanderAction.State, validation.ReceiptPreviewCommand, "adapterExecutionReceipt.validation", false, true, []string{"report is valid but immutable execution receipt is missing"}, validation.MissionCommanderAction.Boundary)}
				validation.NextSteps = []string{"record immutable adapter execution receipt from the returned preview command", "rerun read-only report validation", "do not record observation evidence or rerun the adapter solely because receipt is missing"}
			}
			validation.MissionCommanderActionQueue = mission.MissionCommanderActionQueueFor(validation.MissionCommanderNextActions)
			validation.ReportSummary = adapterReportHandoffSummary(gateEvent, validation.MissionCommanderAction.State, validation.ReportPath, validation.ReportSHA256, validation.Report, validation.Contract.AllowedStatuses, validation.Contract.AllowedOutputPaths, adapterCandidates, validation.Contract.StopConditions, nil, AuthorizedExecutionFollowThrough{}, validation.MissionCommanderActionQueue, validation.MissionCommanderNextActions, false, validation.FailureCode, validation.FailureStage)
			validation.ReportSummary.RecordReady = false
			validation.ReportSummary.RecordBlocked = true
			validation.RunbookSteps = adapterReportRunbookSteps("provenance", validation.MissionCommanderAction.State, validation.ReportPath, validation.ReportSHA256, false, false, false, false, validation.NextSteps, validation.MissionCommanderAction.Boundary, validation.MissionCommanderAction)
			return validation, nil
		}
		validation.AdapterExecutionDispatchPath = dispatchPath
		validation.AdapterExecutionDispatchSHA256 = dispatchSHA
		validation.AdapterExecutionDispatch = &dispatch
		validation.AdapterExecution = receipt
		validation.ProvenanceValid = true
	}
	validation.MissionCommanderAction = adapterReportValidationCommanderAction(pack, gateEvent, validation.ReportPath, validation.RecordExpectedReportSHA256, validation.AdapterExecutionReceiptSHA256, validation.AdapterExecution, true, nil)
	validation.MissionCommanderNextActions = adapterReportValidationCommanderNextActions(gateEvent, validation.MissionCommanderAction, true, nil)
	validation.MissionCommanderActionQueue = mission.MissionCommanderActionQueueFor(validation.MissionCommanderNextActions)
	validation.AuthorizedExecutionFollowThrough = authorizedExecutionFollowThrough(gateEvent, validation.MissionCommanderAction.State, validation.ReportPath, validation.MissionCommanderAction, validation.MissionCommanderNextActions, nil, true, false)
	validation.ReportSummary = adapterReportHandoffSummary(gateEvent, validation.MissionCommanderAction.State, validation.ReportPath, validation.ReportSHA256, validation.Report, validation.Contract.AllowedStatuses, validation.Contract.AllowedOutputPaths, adapterCandidates, validation.Contract.StopConditions, nil, validation.AuthorizedExecutionFollowThrough, validation.MissionCommanderActionQueue, validation.MissionCommanderNextActions, true, "", "")
	validation.NextSteps = adapterReportValidationNextSteps(pack, gateEvent, validation.ReportPath, validation.RecordExpectedReportSHA256)
	validation.RunbookSteps = adapterReportRunbookSteps("validation", validation.MissionCommanderAction.State, validation.ReportSummary.ReportPath, validation.ReportSHA256, validation.Valid, validation.ReportSummary.RecordReady, false, false, validation.NextSteps, validation.MissionCommanderAction.Boundary, validation.MissionCommanderAction)
	return validation, nil
}

func findAuthorizedGateEvent(caseRoot, gateEventID string) (EventPreview, error) {
	if gateEventID == "" {
		return EventPreview{}, fmt.Errorf("gate execution evidence requires -GateEventId <authorized-gate-event-id>")
	}
	items, err := mission.ReadStrictFact(caseRoot, "request")
	if err != nil {
		return EventPreview{}, err
	}
	for _, item := range items {
		if mission.Value(item, "eventId") != gateEventID {
			continue
		}
		data, err := json.Marshal(item)
		if err != nil {
			return EventPreview{}, err
		}
		var event EventPreview
		if err := json.Unmarshal(data, &event); err != nil {
			return EventPreview{}, fmt.Errorf("invalid gate request event %s: %w", gateEventID, err)
		}
		if event.Status != "authorized-gate" || event.Gate.Authorization.Decision != autonomy.DecisionPreauthorized {
			return EventPreview{}, fmt.Errorf("gate execution evidence requires an authorized-gate request with preauthorized decision; %s has status=%s authorization=%s", gateEventID, event.Status, event.Gate.Authorization.Decision)
		}
		if event.Lane == "" || event.Gate.Action == "" {
			return EventPreview{}, fmt.Errorf("authorized gate request %s is missing lane or action", gateEventID)
		}
		return event, nil
	}
	return EventPreview{}, fmt.Errorf("authorized gate eventId not found: %s", gateEventID)
}

func adapterReportContract(repoRoot, caseRoot, pack string, event EventPreview, m *manifest.Manifest) AdapterExecutionReportContract {
	liveValidation := adapterReportLiveValidation(m, caseRoot, pack, event)
	dispatchRequired := liveValidation.DispatchRequired
	commander := adapterReportContractCommanderAction(event, pack, liveValidation, dispatchRequired)
	requiredFields := []string{"schemaVersion", "kind", "adapterId", "action", "status", "gateEventId", "actualBudget"}
	if dispatchRequired {
		requiredFields = append(requiredFields, "dispatch")
	}
	contract := AdapterExecutionReportContract{
		SchemaVersion:               1,
		Command:                     "gate",
		Kind:                        "adapter-execution-report-contract",
		CaseRoot:                    caseRoot,
		RepoRoot:                    repoRoot,
		Pack:                        pack,
		IsMutation:                  false,
		Lane:                        event.Lane,
		Target:                      event.Target,
		BatchID:                     event.BatchID,
		Risk:                        event.Risk,
		Authorization:               event.Gate.Authorization,
		ReportKind:                  "adapter-execution-report",
		ReportSchemaVersion:         1,
		GateEventID:                 event.EventID,
		Action:                      event.Gate.Action,
		AllowedStatuses:             []string{"succeeded", "failed", "boundary-hit", "escalated", "aborted"},
		RequiredFields:              requiredFields,
		AllowedOutputPaths:          append([]string{}, event.Gate.OutputPaths...),
		AuthorizedBudget:            event.Gate.RequestedBudget,
		StopConditions:              append([]string{}, event.Gate.StopConditions...),
		DefaultReportPath:           adapterReportDefaultPath(event.Gate.OutputPaths),
		ReportPathRule:              "case-relative, current-workspace relative, or case-contained absolute file path under one authorized outputPath; sidecar must be <= 1048576 bytes and contain no trailing JSON data",
		RefPathRequires:             []string{"outputRefs and evidenceRefs must be case-relative", "outputRefs and evidenceRefs must stay under authorized outputPaths"},
		SummaryMaxBytes:             4096,
		EscalationMaxBytes:          4096,
		RecordRequired:              event.Gate.Authorization.RecordRequired,
		NotifyMainOn:                append([]string{}, event.Gate.Authorization.NotifyMainOn...),
		BoundaryStatusRequires:      []string{"boundaryHits or escalation for boundary-hit/escalated status", "boundaryHits or escalation when actualBudget exceeds authorizedBudget", "boundaryHits must be one of authorized stopConditions"},
		StatusSummaryRequires:       []string{"summary for failed/boundary-hit/escalated/aborted status"},
		ValidationFailureStages:     adapterReportValidationFailureStages(),
		ValidationFailureCodes:      adapterReportValidationFailureCodes(),
		ValidationRepairHints:       adapterReportContractRepairHints(event),
		DeniedActions:               []string{"heavy-tool execution", "authority writes", "confirmed writes", "out-of-scope output refs", "full trace/dump/log embedding"},
		LiveValidation:              liveValidation,
		MissionCommanderAction:      commander,
		MissionCommanderNextActions: adapterReportContractCommanderNextActions(event, commander),
		NextSteps:                   adapterReportContractNextSteps(pack, event, liveValidation),
	}
	contract.MissionCommanderActionQueue = mission.MissionCommanderActionQueueFor(contract.MissionCommanderNextActions)
	contract.AuthorizedExecutionFollowThrough = authorizedExecutionFollowThrough(event, commander.State, liveValidation.CaseRelativeReportPath, commander, contract.MissionCommanderNextActions, nil, false, false)
	contract.ReportSummary = adapterReportHandoffSummary(event, commander.State, liveValidation.CaseRelativeReportPath, "", nil, contract.AllowedStatuses, contract.AllowedOutputPaths, liveValidation.AdapterCandidates, contract.StopConditions, contract.ValidationRepairHints, contract.AuthorizedExecutionFollowThrough, contract.MissionCommanderActionQueue, contract.MissionCommanderNextActions, false, "", "")
	contract.RunbookSteps = adapterReportRunbookSteps("contract", commander.State, contract.ReportSummary.ReportPath, "", false, false, false, false, contract.NextSteps, commander.Boundary, commander)
	return contract
}

func adapterReportContractCommanderAction(event EventPreview, pack string, liveValidation AdapterReportLiveValidation, dispatchRequired bool) mission.MissionCommanderAction {
	reportPath := strings.TrimSpace(liveValidation.CaseRelativeReportPath)
	if reportPath == "" {
		reportPath = "<reportPath-under-authorized-outputPath>"
	}
	if !dispatchRequired {
		validateCommand := adapterReportValidateSlashCommand(pack, event.EventID, reportPath)
		return mission.MissionCommanderAction{
			State:          "needs-adapter-report-validation",
			Prompt:         fmt.Sprintf("按 authorized gate `%s` 接手：让 executor/tool adapter 在授权 outputPath 写 bounded sidecar，再用 read-only validation 预检；valid=true 后只能使用 validation/status 返回的 hash-bound record command 记录 observation evidence。", event.EventID),
			PrimaryCommand: validateCommand,
			FollowUpCommands: []string{
				"/rekit handoff " + mission.BoardLaneLabel(mission.BoardLane{ID: event.Lane}),
			},
			Boundary: append(adapterReportCommanderBoundary(), "contract handoff does not provide a runnable record Apply; use validation/status returned -ExpectedExecutionReportSha256 after valid=true"),
		}
	}
	adapterID := liveValidation.SidecarTemplate.AdapterID
	if strings.TrimSpace(liveValidation.DispatchRequirementError) != "" {
		return mission.MissionCommanderAction{
			State:          "blocked-by-adapter-execution-catalog-invalid",
			Prompt:         fmt.Sprintf("authorized gate `%s` 的 managed adapter provenance 无法确定；先修复 durable owner/tooling catalog，再记录 dispatch。", event.EventID),
			PrimaryCommand: "/rekit handoff " + mission.BoardLaneLabel(mission.BoardLane{ID: event.Lane}),
			Boundary:       append(adapterReportCommanderBoundary(), "do not downgrade malformed managed adapter provenance to unmanaged compatibility", "repair owner/catalog before external execution"),
		}
	}
	if !liveValidation.DispatchPresent {
		dispatchCommand := adapterExecutionDispatchPreviewSlashCommand(pack, event, reportPath, adapterID)
		return mission.MissionCommanderAction{
			State:          "ready-for-adapter-execution-dispatch-preview",
			Prompt:         fmt.Sprintf("按 authorized gate `%s` 接手：先为 current lane owner、selected adapter、harness/session 和 report path 记录 immutable dispatch；外部 adapter 只能在 dispatch Apply 后开始。", event.EventID),
			PrimaryCommand: dispatchCommand,
			FollowUpCommands: []string{
				"/rekit handoff " + mission.BoardLaneLabel(mission.BoardLane{ID: event.Lane}),
			},
			Boundary: append(adapterReportCommanderBoundary(), "dispatch preview is read-only; review and Apply its expected binding before external adapter execution", "contract handoff does not provide a runnable record Apply; use validation/status returned -ExpectedExecutionReportSha256 after valid=true"),
		}
	}
	if !liveValidation.DispatchCurrent {
		reauthorization := event
		reauthorization.EventID = ""
		reauthorization.BatchID = event.EventID + "-dispatch-retry"
		reauthorization.Subject = event.Subject + " dispatch retry"
		reauthorization.Summary = fmt.Sprintf("Review a distinct authorized execution attempt because dispatch %s no longer matches the current owner, gate, catalog, session, or report path", event.EventID)
		return mission.MissionCommanderAction{
			State:          "blocked-by-adapter-execution-dispatch-drift",
			Prompt:         fmt.Sprintf("authorized gate `%s` 已有 immutable dispatch，但它不再匹配 current owner/generation、gate、catalog、session 或 report path；不能采用旧 attempt 或同 gate 重跑。", event.EventID),
			PrimaryCommand: gateRequestWhatIfSlashCommand(pack, reauthorization),
			FollowUpCommands: []string{
				"/rekit handoff " + mission.BoardLaneLabel(mission.BoardLane{ID: event.Lane}),
			},
			Boundary: append(adapterReportCommanderBoundary(), "do not adopt a stale dispatch after takeover or provenance drift", "authorize a distinct gate before any retry"),
		}
	}
	return mission.MissionCommanderAction{
		State:          "adapter-execution-dispatched-awaiting-report",
		Prompt:         fmt.Sprintf("authorized gate `%s` 的 immutable dispatch 已记录且仍匹配 current owner/catalog/session；等待 external harness 写 report。若 harness 已知 failed/aborted 但未写 sidecar，只记录 dispatch-bound terminal report，不重跑同一 gate。", event.EventID),
		PrimaryCommand: liveValidation.CaseRelativeDraftCommand,
		FollowUpCommands: []string{
			liveValidation.CaseRelativeValidateCommand,
			"/rekit handoff " + mission.BoardLaneLabel(mission.BoardLane{ID: event.Lane}),
		},
		Boundary: append(adapterReportCommanderBoundary(), "do not infer timeout or failure automatically; terminal report must reflect a known external harness outcome", "do not rerun the adapter under the same gate", "failed/aborted terminal draft remains preview then expected-hash Apply"),
	}
}

func adapterReportContractCommanderNextActions(event EventPreview, commander mission.MissionCommanderAction) []mission.MissionCommanderNextActionItem {
	label := gateCommanderActionLabel(event.Lane)
	items := []mission.MissionCommanderNextActionItem{}
	if commander.PrimaryCommand != "" {
		actionID := "adapter-report-contract-validation"
		reasons := []string{"run read-only validation before recording observation evidence", "adapter sidecar must be valid=true before record"}
		switch commander.State {
		case "ready-for-adapter-execution-dispatch-preview":
			actionID = "adapter-execution-dispatch-preview"
			reasons = []string{"record immutable dispatch before external adapter execution", "completion/report/observation must retain the exact dispatch lineage"}
		case "adapter-execution-dispatched-awaiting-report":
			actionID = "adapter-execution-terminal-report-preview"
			reasons = []string{"immutable dispatch is current but report is missing", "wait for the external harness or record only a known failed/aborted terminal outcome"}
		case "blocked-by-adapter-execution-dispatch-drift":
			actionID = "adapter-execution-dispatch-drift-retry"
			reasons = []string{"immutable dispatch no longer matches current provenance", "authorize a distinct gate before retry"}
		case "blocked-by-adapter-execution-catalog-invalid":
			actionID = "adapter-execution-catalog-repair"
			reasons = []string{"managed adapter provenance is invalid", "repair durable owner/tooling catalog before external execution"}
		}
		blocked := strings.HasPrefix(commander.State, "blocked-by-adapter-execution-")
		items = append(items, adapterReportNextActionItem(event, label, actionID, commander.State, commander.PrimaryCommand, "adapterReportContract.missionCommanderAction", blocked, true, reasons, commander.Boundary))
	}
	for _, followUp := range commander.FollowUpCommands {
		if strings.TrimSpace(followUp) == "" {
			continue
		}
		boundary := append([]string{}, commander.Boundary...)
		reasons := []string{"follow adapter report contract handoff after validation"}
		blocked := false
		if strings.Contains(followUp, "-Apply") {
			blocked = true
			reasons = append(reasons, "run only after validation returns valid=true", "replace <executor-id> before recording evidence")
			boundary = append(boundary, "do not record evidence until validation returns valid=true", "replace <executor-id> before running record command")
		}
		items = append(items, adapterReportNextActionItem(event, label, "adapter-report-contract-follow-up", commander.State, followUp, "adapterReportContract.missionCommanderAction.followUp", blocked, true, reasons, boundary))
	}
	return mission.UniqueCommanderNextActions(items)
}

func adapterReportValidationCommanderNextActions(event EventPreview, commander mission.MissionCommanderAction, valid bool, hints []AdapterReportRepairHint) []mission.MissionCommanderNextActionItem {
	label := gateCommanderActionLabel(event.Lane)
	items := []mission.MissionCommanderNextActionItem{}
	if valid {
		if commander.PrimaryCommand != "" {
			items = append(items, adapterReportNextActionItem(event, label, "adapter-report-record", commander.State, commander.PrimaryCommand, "adapterReportValidation.missionCommanderAction", false, true, []string{"validation returned valid=true", "replace <executor-id> before recording bounded observation evidence"}, append(append([]string{}, commander.Boundary...), "replace <executor-id> before running record command")))
		}
		for idx, followUp := range commander.FollowUpCommands {
			items = append(items, adapterReportNextActionItem(event, label, fmt.Sprintf("adapter-report-record-follow-up-%d", idx+1), commander.State, followUp, "adapterReportValidation.missionCommanderAction.followUp", false, true, []string{"handoff after recording or reviewing valid adapter report evidence"}, commander.Boundary))
		}
		return mission.UniqueCommanderNextActions(items)
	}
	for _, hint := range hints {
		if strings.TrimSpace(hint.RepairAction) == "" {
			continue
		}
		items = append(items, adapterReportNextActionItem(event, label, "adapter-report-repair-"+hint.RepairAction, commander.State, hint.RepairAction, "adapterReportValidation.repairHints", hint.EscalateToMain, true, adapterReportRepairHintReasons(hint), adapterReportRepairHintBoundaries(commander.Boundary, hint)))
	}
	if commander.PrimaryCommand != "" {
		items = append(items, adapterReportNextActionItem(event, label, "adapter-report-rerun-validation", commander.State, commander.PrimaryCommand, "adapterReportValidation.missionCommanderAction", commander.State == "needs-main-escalation", true, []string{"rerun read-only validation after repairing the adapter report", "do not record evidence until validation returns valid=true"}, commander.Boundary))
	}
	return mission.UniqueCommanderNextActions(items)
}

func gateCommanderActionLabel(laneID string) string {
	label := mission.BoardLaneLabel(mission.BoardLane{ID: laneID})
	if strings.TrimSpace(label) == "" {
		label = strings.TrimSpace(laneID)
	}
	if label == "" {
		label = "main"
	}
	return label
}

func adapterReportActionID(eventID, suffix string) string {
	eventID = strings.TrimSpace(eventID)
	suffix = strings.TrimSpace(suffix)
	if eventID == "" {
		return suffix
	}
	if suffix == "" {
		return eventID
	}
	return eventID + ":" + suffix
}

func adapterReportNextActionItem(event EventPreview, label, actionID, state, command, source string, blocked, requiresReview bool, reasons, boundary []string) mission.MissionCommanderNextActionItem {
	return mission.MissionCommanderNextActionItem{
		Lane:           event.Lane,
		Label:          label,
		GateEventID:    event.EventID,
		ActionID:       adapterReportActionID(event.EventID, actionID),
		State:          state,
		Command:        command,
		Source:         source,
		Blocked:        blocked,
		RequiresReview: requiresReview,
		Reasons:        append([]string{}, reasons...),
		Boundary:       append([]string{}, boundary...),
	}
}

func adapterReportContractNextSteps(pack string, event EventPreview, liveValidation AdapterReportLiveValidation) []string {
	reportPath := strings.TrimSpace(liveValidation.CaseRelativeReportPath)
	if reportPath == "" {
		reportPath = "<reportPath-under-authorized-outputPath>"
	}
	return []string{
		"record immutable dispatch before external execution: " + adapterExecutionDispatchPreviewSlashCommand(pack, event, reportPath, liveValidation.SidecarTemplate.AdapterID),
		"after reviewed dispatch Apply, adapter writes bounded report under authorized output path: " + reportPath,
		"preflight read-only: " + adapterReportValidateSlashCommand(pack, event.EventID, reportPath),
		"after valid=true, use the validation/status returned hash-bound record command with -ExpectedExecutionReportSha256; do not run a contract-stage bare record template",
		"replace <executor-id> before record; /rekit records evidence only and never executes the heavy tool",
		"review refs before any authority/confirmed outcome",
	}
}

func adapterReportValidationCommanderAction(pack string, gateEvent EventPreview, reportPath, reportSHA256, receiptSHA256 string, receipt *adapterexecution.Receipt, valid bool, hints []AdapterReportRepairHint) mission.MissionCommanderAction {
	reportPath = strings.TrimSpace(reportPath)
	reportSHA256 = strings.TrimSpace(reportSHA256)
	if reportPath == "" {
		reportPath = adapterReportDefaultPath(gateEvent.Gate.OutputPaths)
	}
	if reportPath == "" {
		reportPath = "<reportPath-under-authorized-outputPath>"
	}
	if valid {
		command := adapterReportRecordSlashCommandWithExpectedHash(pack, gateEvent.EventID, reportPath, reportSHA256, receiptSHA256, receipt)
		return mission.MissionCommanderAction{
			State:          "ready-to-record-evidence",
			Prompt:         fmt.Sprintf("authorized gate `%s` 的 sidecar 与 execution receipt provenance 已 valid=true；只记录 observation evidence，再 review refs。", gateEvent.EventID),
			PrimaryCommand: command,
			FollowUpCommands: []string{
				"/rekit handoff " + mission.BoardLaneLabel(mission.BoardLane{ID: gateEvent.Lane}),
			},
			Boundary: adapterReportCommanderBoundary(),
		}
	}
	return mission.MissionCommanderAction{
		State:          adapterReportRepairState(hints),
		Prompt:         fmt.Sprintf("authorized gate `%s` 的 sidecar 尚未 valid=true；先按 repair hints 修复，再重新运行 read-only validation。", gateEvent.EventID),
		PrimaryCommand: adapterReportValidateSlashCommand(pack, gateEvent.EventID, reportPath),
		Boundary:       append(adapterReportCommanderBoundary(), "do not record evidence until validation returns valid=true"),
	}
}

func adapterReportValidationNextSteps(pack string, gateEvent EventPreview, reportPath, reportSHA256 string) []string {
	return []string{
		"report is valid for read-only preflight",
		"record observation evidence: " + adapterReportRecordSlashCommandWithExpectedHash(pack, gateEvent.EventID, reportPath, reportSHA256),
		"review refs before any authority/confirmed outcome",
	}
}

func adapterReportRepairState(hints []AdapterReportRepairHint) string {
	for _, hint := range hints {
		if hint.RepairAction == "provide-execution-report-path" {
			return "needs-execution-report-path"
		}
		if hint.EscalateToMain {
			return "needs-main-escalation"
		}
	}
	return "repair-adapter-report"
}

func adapterReportValidateSlashCommand(pack, gateEventID, reportPath string) string {
	return adapterReportSlashCommand([]string{"gate", "-Pack", pack, "-GateEventId", gateEventID, "-ValidateExecutionReport", "-ExecutionReportPath", reportPath, "-Format", "json"})
}

func adapterReportRecordSlashCommandWithExpectedHash(pack, gateEventID, reportPath, reportSHA256 string, provenance ...any) string {
	args := []string{"gate", "-Pack", pack, "-Apply", "-GateEventId", gateEventID, "-ExecutionReportPath", reportPath}
	if strings.TrimSpace(reportSHA256) != "" {
		args = append(args, "-ExpectedExecutionReportSha256", strings.TrimSpace(reportSHA256))
	}
	actor := "<executor-id>"
	if len(provenance) == 2 {
		receiptSHA256, _ := provenance[0].(string)
		receipt, _ := provenance[1].(*adapterexecution.Receipt)
		if receipt != nil && strings.TrimSpace(receiptSHA256) != "" {
			args = append(args, "-AdapterExecutionReceiptPath", filepath.ToSlash(filepath.Join(".rekit", "lanes", receipt.Owner.Lane, "adapter-executions", receipt.Gate.GateEventID, "receipt.json")), "-ExpectedAdapterExecutionReceiptSha256", receiptSHA256, "-Executor", receipt.Owner.CurrentExecutor, "-ExpectedExecutorGeneration", fmt.Sprintf("%d", receipt.Owner.ExecutorGeneration))
			actor = receipt.Actor
		}
	}
	args = append(args, "-Actor", actor, "-Format", "json")
	return adapterReportSlashCommand(args)
}

func adapterReportSlashCommand(args []string) string {
	parts := append([]string{"/rekit"}, args...)
	for i, part := range parts {
		parts[i] = quoteAdapterReportCommandArg(part)
	}
	return strings.Join(parts, " ")
}

func quoteAdapterReportCommandArg(arg string) string {
	if arg == "" {
		return `""`
	}
	if !strings.ContainsAny(arg, " \t\"") {
		return arg
	}
	return `"` + strings.ReplaceAll(arg, `"`, `\"`) + `"`
}

func adapterReportCommanderBoundary() []string {
	return []string{
		"validation is read-only: no observations/authority/confirmed writes",
		"record command writes bounded observation evidence only after valid=true",
		"/rekit never executes the heavy tool",
		"no authority/confirmed writes",
	}
}

func adapterReportDefaultPath(outputPaths []string) string {
	for _, outputPath := range normalizedGatePaths(outputPaths) {
		return strings.TrimRight(outputPath, "/") + "/adapter-report.json"
	}
	return ""
}

func adapterReportLiveValidation(m *manifest.Manifest, caseRoot, pack string, event EventPreview) AdapterReportLiveValidation {
	reportFileName := "adapter-report.json"
	caseRelativeReportPath := adapterReportDefaultPath(event.Gate.OutputPaths)
	adapterCandidates := adapterToolCandidates(m, event)
	dispatchRequired, dispatchRequirementErr := adapterExecutionReceiptRequired(caseRoot, event, m)
	dispatch, dispatchPath, dispatchSHA, _, dispatchPresent, dispatchErr := inspectAdapterExecutionDispatch(caseRoot, pack, event, m)
	if dispatchRequirementErr != nil {
		dispatchRequired = true
		dispatchErr = dispatchRequirementErr
	}
	if dispatchPresent && strings.TrimSpace(dispatch.ReportPath) != "" {
		caseRelativeReportPath = dispatch.ReportPath
	}
	template := AdapterReportSidecarTemplate{
		SchemaVersion: 1,
		Kind:          "adapter-execution-report",
		AdapterID:     sidecarAdapterID(adapterCandidates),
		Action:        event.Gate.Action,
		Status:        "succeeded|failed|boundary-hit|escalated|aborted",
		GateEventID:   event.EventID,
		ActualBudget:  autonomy.Budget{},
		OutputRefs:    []string{"<case-relative output under authorized outputPaths>"},
		EvidenceRefs:  []string{"<case-relative bounded evidence ref under authorized outputPaths>"},
		BoundaryHits:  []string{"<authorized stopCondition token when status/budget requires it>"},
		Escalation:    "<bounded escalation when status/budget requires it>",
		Summary:       "<bounded summary; required for failed/boundary-hit/escalated/aborted>",
	}
	if dispatchPresent && dispatchErr == nil {
		template.Dispatch = &adapterexecution.ReportDispatchBinding{DispatchID: dispatch.DispatchID, Path: dispatchPath, SHA256: dispatchSHA}
	}
	templateData, _ := adapterReportScaffoldBytes(template)
	templateSHA256 := sha256HexBytes(templateData)
	validateArgs := []string{"-Command", "gate", "-Pack", pack, "-GateEventId", event.EventID, "-ValidateExecutionReport", "-ExecutionReportPath", reportFileName, "-Format", "json"}
	scaffoldArgs := []string{"-Command", "gate", "-Pack", pack, "-GateEventId", event.EventID, "-ScaffoldExecutionReport", "-ExecutionReportPath", reportFileName, "-Format", "json"}
	scaffoldApplyArgs := []string{"-Command", "gate", "-Pack", pack, "-GateEventId", event.EventID, "-ScaffoldExecutionReport", "-ExecutionReportPath", reportFileName, "-ExpectedExecutionReportSha256", templateSHA256, "-Apply", "-Format", "json"}
	draftArgs := []string{"-Command", "gate", "-Pack", pack, "-GateEventId", event.EventID, "-DraftExecutionReport", "-ExecutionReportPath", reportFileName, "-AdapterId", template.AdapterID, "-ExecutionStatus", "<status>", "-Summary", "<bounded-summary>", "-Format", "json"}
	draftApplyArgs := []string{"-Command", "gate", "-Pack", pack, "-GateEventId", event.EventID, "-DraftExecutionReport", "-ExecutionReportPath", reportFileName, "-AdapterId", template.AdapterID, "-ExecutionStatus", "<status>", "-Summary", "<bounded-summary>", "-ExpectedExecutionReportSha256", "<reportSha256-from-draft-preview>", "-Apply", "-Format", "json"}
	caseRelativeValidateArgs := []string{}
	caseRelativeRecordArgs := []string{}
	caseRelativeScaffoldArgs := []string{}
	caseRelativeScaffoldApplyArgs := []string{}
	caseRelativeDraftArgs := []string{}
	caseRelativeDraftApplyArgs := []string{}
	caseRelativeValidateCommand := ""
	caseRelativeScaffoldCommand := ""
	caseRelativeScaffoldApplyCommand := ""
	caseRelativeDraftCommand := ""
	caseRelativeDraftApplyCommand := ""
	if caseRelativeReportPath != "" {
		caseRelativeValidateArgs = []string{"-Command", "gate", "-Pack", pack, "-GateEventId", event.EventID, "-ValidateExecutionReport", "-ExecutionReportPath", caseRelativeReportPath, "-Format", "json"}
		caseRelativeScaffoldArgs = []string{"-Command", "gate", "-Pack", pack, "-GateEventId", event.EventID, "-ScaffoldExecutionReport", "-ExecutionReportPath", caseRelativeReportPath, "-Format", "json"}
		caseRelativeScaffoldApplyArgs = []string{"-Command", "gate", "-Pack", pack, "-GateEventId", event.EventID, "-ScaffoldExecutionReport", "-ExecutionReportPath", caseRelativeReportPath, "-ExpectedExecutionReportSha256", templateSHA256, "-Apply", "-Format", "json"}
		caseRelativeDraftArgs = []string{"-Command", "gate", "-Pack", pack, "-GateEventId", event.EventID, "-DraftExecutionReport", "-ExecutionReportPath", caseRelativeReportPath, "-AdapterId", template.AdapterID, "-ExecutionStatus", "<status>", "-Summary", "<bounded-summary>", "-Format", "json"}
		caseRelativeDraftApplyArgs = []string{"-Command", "gate", "-Pack", pack, "-GateEventId", event.EventID, "-DraftExecutionReport", "-ExecutionReportPath", caseRelativeReportPath, "-AdapterId", template.AdapterID, "-ExecutionStatus", "<status>", "-Summary", "<bounded-summary>", "-ExpectedExecutionReportSha256", "<reportSha256-from-draft-preview>", "-Apply", "-Format", "json"}
		caseRelativeValidateCommand = "rekit " + strings.Join(caseRelativeValidateArgs, " ")
		caseRelativeScaffoldCommand = "rekit " + strings.Join(caseRelativeScaffoldArgs, " ")
		caseRelativeScaffoldApplyCommand = "rekit " + strings.Join(caseRelativeScaffoldApplyArgs, " ")
		caseRelativeDraftCommand = "rekit " + strings.Join(caseRelativeDraftArgs, " ")
		caseRelativeDraftApplyCommand = "rekit " + strings.Join(caseRelativeDraftApplyArgs, " ")
	}
	live := AdapterReportLiveValidation{
		InvocationCwd:                    "authorized output workspace listed in authorizedWorkspaces; use reportFileName as workspace-relative -ExecutionReportPath and omit -Target; or use caseRelativeReportPath with case-relative commands from any case-local cwd",
		AuthorizedWorkspaces:             normalizedGatePaths(event.Gate.OutputPaths),
		ReportFileName:                   reportFileName,
		CaseRelativeReportPath:           caseRelativeReportPath,
		DispatchRequired:                 dispatchRequired,
		DispatchPresent:                  dispatchPresent,
		DispatchCurrent:                  dispatchPresent && dispatchErr == nil,
		AdapterExecutionDispatchPath:     dispatchPath,
		AdapterExecutionDispatchSHA256:   dispatchSHA,
		SidecarTemplate:                  template,
		ValidateCommand:                  "rekit " + strings.Join(validateArgs, " "),
		ScaffoldCommand:                  "rekit " + strings.Join(scaffoldArgs, " "),
		ScaffoldApplyCommand:             "rekit " + strings.Join(scaffoldApplyArgs, " "),
		SidecarTemplateSHA256:            templateSHA256,
		DraftCommand:                     "rekit " + strings.Join(draftArgs, " "),
		DraftApplyCommand:                "rekit " + strings.Join(draftApplyArgs, " "),
		DraftReportSHA256:                "<reportSha256-from-draft-preview>",
		ValidateArgs:                     validateArgs,
		ScaffoldArgs:                     scaffoldArgs,
		ScaffoldApplyArgs:                scaffoldApplyArgs,
		DraftArgs:                        draftArgs,
		DraftApplyArgs:                   draftApplyArgs,
		CaseRelativeValidateCommand:      caseRelativeValidateCommand,
		CaseRelativeScaffoldCommand:      caseRelativeScaffoldCommand,
		CaseRelativeScaffoldApplyCommand: caseRelativeScaffoldApplyCommand,
		CaseRelativeDraftCommand:         caseRelativeDraftCommand,
		CaseRelativeDraftApplyCommand:    caseRelativeDraftApplyCommand,
		CaseRelativeValidateArgs:         caseRelativeValidateArgs,
		CaseRelativeRecordArgs:           caseRelativeRecordArgs,
		CaseRelativeScaffoldArgs:         caseRelativeScaffoldArgs,
		CaseRelativeScaffoldApplyArgs:    caseRelativeScaffoldApplyArgs,
		CaseRelativeDraftArgs:            caseRelativeDraftArgs,
		CaseRelativeDraftApplyArgs:       caseRelativeDraftApplyArgs,
		AdapterCandidates:                adapterCandidates,
		SelectedAdapter:                  selectedAdapterToolCandidate(m, event, sidecarAdapterID(adapterCandidates)),
		ReplayBehavior:                   "after valid=true, repeating the validation/status returned hash-bound record command with the same bounded sidecar returns applied=false and reason=duplicate eventId without appending observations",
		Notes: []string{
			"ScaffoldArgs and CaseRelativeScaffoldArgs write only the missing adapter-report.json template; they do not execute the adapter, validate the report, record observations, or write authority/confirmed.",
			"ValidateArgs and CaseRelativeValidateArgs are read-only: isMutation=false, applied=false, and no observations/authority/confirmed writes.",
			"Pre-validation contract/scaffold/draft handoffs intentionally omit runnable RecordArgs/CaseRelativeRecordArgs; after valid=true, use validation/status returned hash-bound record command with -ExpectedExecutionReportSha256.",
			"Use only authorized stopConditions in boundaryHits; failed/boundary-hit/escalated/aborted reports require a bounded summary.",
			"Keep outputRefs/evidenceRefs case-relative and under authorized outputPaths so validation and record paths enforce the same artifact boundary.",
			"Keep full trace/dump/log data in sidecar artifacts referenced by outputRefs/evidenceRefs, not in this report.",
		},
	}
	if dispatchPresent {
		live.AdapterExecutionDispatchID = dispatch.DispatchID
	}
	if dispatchRequirementErr != nil {
		live.DispatchRequirementError = dispatchRequirementErr.Error()
	}
	if live.DispatchCurrent {
		live.DraftArgs = []string{"-Command", "gate", "-Pack", pack, "-GateEventId", event.EventID, "-DraftExecutionReport", "-ExecutionReportPath", caseRelativeReportPath, "-AdapterId", template.AdapterID, "-ExecutionStatus", "failed|aborted", "-Summary", "<bounded-summary>", "-Format", "json"}
		live.DraftApplyArgs = []string{"-Command", "gate", "-Pack", pack, "-GateEventId", event.EventID, "-DraftExecutionReport", "-ExecutionReportPath", caseRelativeReportPath, "-AdapterId", template.AdapterID, "-ExecutionStatus", "failed|aborted", "-Summary", "<bounded-summary>", "-ExpectedExecutionReportSha256", "<reportSha256-from-draft-preview>", "-Apply", "-Format", "json"}
		live.DraftCommand = "rekit " + strings.Join(live.DraftArgs, " ")
		live.DraftApplyCommand = "rekit " + strings.Join(live.DraftApplyArgs, " ")
		live.CaseRelativeDraftArgs = append([]string{}, live.DraftArgs...)
		live.CaseRelativeDraftApplyArgs = append([]string{}, live.DraftApplyArgs...)
		live.CaseRelativeDraftCommand = live.DraftCommand
		live.CaseRelativeDraftApplyCommand = live.DraftApplyCommand
	}
	if dispatchErr != nil {
		live.DispatchError = dispatchErr.Error()
	}
	live.RunbookSteps = adapterReportLiveValidationRunbookSteps(live)
	return live
}

func ScaffoldAdapterExecutionReport(repoRoot, caseRoot, pack string, opt Options) (AdapterExecutionReportScaffold, error) {
	inst, gateEvent, err := authorizedGateEvent(repoRoot, caseRoot, pack, opt)
	if err != nil {
		return AdapterExecutionReportScaffold{}, err
	}
	m, err := manifest.Load(repoRoot, pack)
	if err != nil {
		return AdapterExecutionReportScaffold{}, err
	}
	if decoded, decodeErr := hex.DecodeString(strings.TrimSpace(opt.ExpectedExecutionReportSHA256)); strings.TrimSpace(opt.ExpectedExecutionReportSHA256) != "" && (decodeErr != nil || len(decoded) != sha256.Size) {
		return AdapterExecutionReportScaffold{}, fmt.Errorf("gate execution report scaffold Apply requires a valid -ExpectedExecutionReportSha256 from preview")
	}
	contract := adapterReportContract(repoRoot, inst.CaseRoot, pack, gateEvent, m)
	result, data, fullPath, err := adapterReportScaffoldPreview(repoRoot, inst.CaseRoot, pack, gateEvent, contract, opt)
	if err != nil {
		return AdapterExecutionReportScaffold{}, err
	}
	if opt.ExpectedExecutionReportSHA256 == "" {
		return result, nil
	}
	if !strings.EqualFold(result.ReportSHA256, strings.TrimSpace(opt.ExpectedExecutionReportSHA256)) {
		return AdapterExecutionReportScaffold{}, fmt.Errorf("gate execution report scaffold template changed after preview")
	}
	if result.AlreadyExists {
		result.Applied = true
		result.Replay = true
		result.RequiresConfirmation = false
		result.NextSteps = []string{
			"exact adapter-report.json scaffold already exists; duplicate Apply is an idempotent replay and did not rewrite bytes",
			"do not rerun the adapter or record evidence from the scaffold replay; run read-only validation after bounded fields are filled: " + result.ValidateCommand,
			"record bounded observation evidence only with the hash-bound record command returned by validation/status after valid=true",
		}
		result.MissionCommanderAction = adapterReportScaffoldCommanderAction(result)
		result.MissionCommanderNextActions = adapterReportScaffoldCommanderNextActions(gateEvent, result)
		result.MissionCommanderActionQueue = mission.MissionCommanderActionQueueFor(result.MissionCommanderNextActions)
		result.RunbookSteps = adapterReportRunbookSteps("scaffold replay", result.MissionCommanderAction.State, result.ReportPath, result.ReportSHA256, false, false, result.Applied, false, result.NextSteps, result.Boundary, result.MissionCommanderAction)
		return result, nil
	}
	if err := writeAdapterReportScaffold(inst.CaseRoot, fullPath, result.ReportPath, data); err != nil {
		return AdapterExecutionReportScaffold{}, err
	}
	if existing, present, err := readAdapterReportRaw(inst.CaseRoot, fullPath, result.ReportPath); err != nil {
		return AdapterExecutionReportScaffold{}, err
	} else if !present || !bytes.Equal(existing, data) {
		return AdapterExecutionReportScaffold{}, fmt.Errorf("gate execution report scaffold changed while writing: %s", result.ReportPath)
	}
	result.Applied = true
	result.Mode = "scaffolded"
	result.AlreadyExists = false
	result.RequiresConfirmation = false
	result.NextSteps = []string{
		"fill the scaffold with gate -DraftExecutionReport after the external adapter completes, or edit bounded fields manually",
		"run read-only validation: " + result.ValidateCommand,
		"record bounded observation evidence only with the hash-bound record command returned by validation/status after valid=true",
	}
	result.MissionCommanderAction = adapterReportScaffoldCommanderAction(result)
	result.MissionCommanderNextActions = adapterReportScaffoldCommanderNextActions(gateEvent, result)
	result.MissionCommanderActionQueue = mission.MissionCommanderActionQueueFor(result.MissionCommanderNextActions)
	result.RunbookSteps = adapterReportRunbookSteps("scaffold", result.MissionCommanderAction.State, result.ReportPath, result.ReportSHA256, false, false, result.Applied, false, result.NextSteps, result.Boundary, result.MissionCommanderAction)
	return result, nil
}

func DraftAdapterExecutionReport(repoRoot, caseRoot, pack string, opt Options) (_ AdapterExecutionReportDraft, retErr error) {
	inst, gateEvent, err := authorizedGateEvent(repoRoot, caseRoot, pack, opt)
	if err != nil {
		return AdapterExecutionReportDraft{}, err
	}
	m, err := manifest.Load(repoRoot, pack)
	if err != nil {
		return AdapterExecutionReportDraft{}, err
	}
	if decoded, decodeErr := hex.DecodeString(strings.TrimSpace(opt.ExpectedExecutionReportSHA256)); strings.TrimSpace(opt.ExpectedExecutionReportSHA256) != "" && (decodeErr != nil || len(decoded) != sha256.Size) {
		return AdapterExecutionReportDraft{}, fmt.Errorf("gate execution report draft Apply requires a valid -ExpectedExecutionReportSha256 from preview")
	}
	contract := adapterReportContract(repoRoot, inst.CaseRoot, pack, gateEvent, m)
	result, data, scaffoldData, fullPath, err := adapterReportDraftPreview(repoRoot, inst.CaseRoot, pack, gateEvent, contract, opt, m)
	if err != nil {
		return AdapterExecutionReportDraft{}, err
	}
	if opt.ExpectedExecutionReportSHA256 == "" {
		return result, nil
	}
	lease, err := acquireGateLaneMutationLease(inst.CaseRoot, gateEvent.Lane)
	if err != nil {
		return AdapterExecutionReportDraft{}, err
	}
	defer func() { retErr = errors.Join(retErr, lease.Unlock()) }()
	lockedInst, lockedGateEvent, err := authorizedGateEvent(repoRoot, caseRoot, pack, opt)
	if err != nil {
		return AdapterExecutionReportDraft{}, err
	}
	if lockedInst.CaseRoot != inst.CaseRoot || lockedGateEvent.EventID != gateEvent.EventID || lockedGateEvent.Lane != gateEvent.Lane {
		return AdapterExecutionReportDraft{}, fmt.Errorf("authorized gate routing changed while acquiring mutation lease")
	}
	lockedManifest, err := manifest.Load(repoRoot, pack)
	if err != nil {
		return AdapterExecutionReportDraft{}, err
	}
	lockedContract := adapterReportContract(repoRoot, lockedInst.CaseRoot, pack, lockedGateEvent, lockedManifest)
	result, data, scaffoldData, fullPath, err = adapterReportDraftPreview(repoRoot, lockedInst.CaseRoot, pack, lockedGateEvent, lockedContract, opt, lockedManifest)
	if err != nil {
		return AdapterExecutionReportDraft{}, err
	}
	if err := lease.Validate(); err != nil {
		return AdapterExecutionReportDraft{}, err
	}
	if !strings.EqualFold(result.ReportSHA256, strings.TrimSpace(opt.ExpectedExecutionReportSHA256)) {
		return AdapterExecutionReportDraft{}, fmt.Errorf("gate execution report draft changed after preview")
	}
	if result.AlreadyExists && !result.ReplacesScaffold {
		result.Applied = true
		result.Replay = true
		result.RequiresConfirmation = false
		result.NextSteps = []string{
			"exact adapter-report.json draft already exists; duplicate Apply is an idempotent replay and did not rewrite bytes",
			"do not rerun the adapter or record evidence from the draft replay; run read-only validation: " + result.ValidateCommand,
			"record bounded observation evidence only with the hash-bound record command returned by validation/status after valid=true",
		}
		result.MissionCommanderAction = adapterReportDraftCommanderAction(result)
		result.MissionCommanderNextActions = adapterReportDraftCommanderNextActions(gateEvent, result)
		result.MissionCommanderActionQueue = mission.MissionCommanderActionQueueFor(result.MissionCommanderNextActions)
		result.RunbookSteps = adapterReportRunbookSteps("draft replay", result.MissionCommanderAction.State, result.ReportPath, result.ReportSHA256, false, false, result.Applied, false, result.NextSteps, result.Boundary, result.MissionCommanderAction)
		return result, nil
	}
	if err := writeAdapterReportDraft(lockedInst.CaseRoot, fullPath, result.ReportPath, data, scaffoldData); err != nil {
		return AdapterExecutionReportDraft{}, err
	}
	if err := lease.Validate(); err != nil {
		return AdapterExecutionReportDraft{}, fmt.Errorf("adapter execution report draft may already be durable at %s; mutation lease validation failed after write: %w", result.ReportPath, err)
	}
	if existing, present, err := readAdapterReportRaw(lockedInst.CaseRoot, fullPath, result.ReportPath); err != nil {
		return AdapterExecutionReportDraft{}, err
	} else if !present || !bytes.Equal(existing, data) {
		return AdapterExecutionReportDraft{}, fmt.Errorf("gate execution report draft changed while writing: %s", result.ReportPath)
	}
	result.Applied = true
	result.Mode = "drafted"
	result.AlreadyExists = false
	result.ReplacesScaffold = false
	result.RequiresConfirmation = false
	result.NextSteps = []string{
		"run read-only validation: " + result.ValidateCommand,
		"record bounded observation evidence only with the hash-bound record command returned by validation/status after valid=true",
	}
	result.MissionCommanderAction = adapterReportDraftCommanderAction(result)
	result.MissionCommanderNextActions = adapterReportDraftCommanderNextActions(gateEvent, result)
	result.MissionCommanderActionQueue = mission.MissionCommanderActionQueueFor(result.MissionCommanderNextActions)
	result.RunbookSteps = adapterReportRunbookSteps("draft", result.MissionCommanderAction.State, result.ReportPath, result.ReportSHA256, false, false, result.Applied, false, result.NextSteps, result.Boundary, result.MissionCommanderAction)
	return result, nil
}

func adapterReportScaffoldPreview(repoRoot, caseRoot, pack string, gateEvent EventPreview, contract AdapterExecutionReportContract, opt Options) (AdapterExecutionReportScaffold, []byte, string, error) {
	fullPath, reportPath, err := adapterExecutionReportScaffoldPath(caseRoot, gateEvent, opt.ExecutionReportCwd, opt.ExecutionReportPath)
	if err != nil {
		return AdapterExecutionReportScaffold{}, nil, "", err
	}
	template := contract.LiveValidation.SidecarTemplate
	data, err := adapterReportScaffoldBytes(template)
	if err != nil {
		return AdapterExecutionReportScaffold{}, nil, "", err
	}
	reportSHA256 := sha256HexBytes(data)
	validateCommand := adapterReportValidateSlashCommand(pack, gateEvent.EventID, reportPath)
	applyCommand := adapterReportScaffoldSlashCommand(pack, gateEvent.EventID, reportPath, reportSHA256)
	boundary := adapterReportScaffoldBoundary()
	result := AdapterExecutionReportScaffold{
		SchemaVersion:        1,
		Command:              "gate",
		Kind:                 "adapter-execution-report-scaffold",
		CaseRoot:             caseRoot,
		RepoRoot:             repoRoot,
		Pack:                 pack,
		IsMutation:           opt.ExpectedExecutionReportSHA256 != "",
		Applied:              false,
		Mode:                 "preview",
		GateEventID:          gateEvent.EventID,
		ReportPath:           reportPath,
		ReportSHA256:         reportSHA256,
		RequiresConfirmation: true,
		SidecarTemplate:      template,
		ValidateCommand:      validateCommand,
		ApplyCommand:         applyCommand,
		Boundary:             boundary,
		NextSteps: []string{
			"review the scaffolded adapter-report.json template and authorized output path",
			"write scaffold only if the sidecar is missing: " + applyCommand,
			"after the external adapter fills bounded execution fields, run read-only validation: " + validateCommand,
			"record bounded observation evidence only with the hash-bound record command returned by validation/status after valid=true",
		},
	}
	if existing, present, err := readAdapterReportRaw(caseRoot, fullPath, reportPath); err != nil {
		return AdapterExecutionReportScaffold{}, nil, "", err
	} else if present {
		if !bytes.Equal(existing, data) {
			return AdapterExecutionReportScaffold{}, nil, "", fmt.Errorf("gate execution report scaffold target already exists with different bytes; validate or repair existing sidecar instead: %s", reportPath)
		}
		result.AlreadyExists = true
		result.RequiresConfirmation = false
		result.Mode = "already-scaffolded"
		result.ApplyCommand = ""
		result.NextSteps = []string{
			"the exact scaffold already exists; edit placeholder fields after the external adapter completes",
			"run read-only validation after placeholders are filled: " + validateCommand,
			"record bounded observation evidence only with the hash-bound record command returned by validation/status after valid=true",
		}
	}
	result.MissionCommanderAction = adapterReportScaffoldCommanderAction(result)
	result.MissionCommanderNextActions = adapterReportScaffoldCommanderNextActions(gateEvent, result)
	result.MissionCommanderActionQueue = mission.MissionCommanderActionQueueFor(result.MissionCommanderNextActions)
	result.RunbookSteps = adapterReportRunbookSteps("scaffold", result.MissionCommanderAction.State, result.ReportPath, result.ReportSHA256, false, false, result.Applied, false, result.NextSteps, result.Boundary, result.MissionCommanderAction)
	return result, data, fullPath, nil
}

func adapterReportDraftPreview(repoRoot, caseRoot, pack string, gateEvent EventPreview, contract AdapterExecutionReportContract, opt Options, m *manifest.Manifest) (AdapterExecutionReportDraft, []byte, []byte, string, error) {
	fullPath, reportPath, err := adapterExecutionReportScaffoldPath(caseRoot, gateEvent, opt.ExecutionReportCwd, opt.ExecutionReportPath)
	if err != nil {
		return AdapterExecutionReportDraft{}, nil, nil, "", err
	}
	if contract.LiveValidation.DispatchRequired && contract.LiveValidation.DispatchPresent {
		if !contract.LiveValidation.DispatchCurrent {
			return AdapterExecutionReportDraft{}, nil, nil, "", fmt.Errorf("gate execution report draft requires a current immutable dispatch")
		}
		dispatchReportPath := strings.TrimSpace(contract.LiveValidation.CaseRelativeReportPath)
		if dispatchReportPath == "" || reportPath != dispatchReportPath {
			return AdapterExecutionReportDraft{}, nil, nil, "", fmt.Errorf("gate execution report draft path %q must match immutable dispatch report path %q", reportPath, dispatchReportPath)
		}
	}
	report, err := adapterReportDraftFromOptions(caseRoot, gateEvent, contract, opt, m)
	if err != nil {
		return AdapterExecutionReportDraft{}, nil, nil, "", err
	}
	data, err := adapterReportBytes(report)
	if err != nil {
		return AdapterExecutionReportDraft{}, nil, nil, "", err
	}
	scaffoldData, err := adapterReportScaffoldBytes(contract.LiveValidation.SidecarTemplate)
	if err != nil {
		return AdapterExecutionReportDraft{}, nil, nil, "", err
	}
	reportSHA256 := sha256HexBytes(data)
	validateCommand := adapterReportValidateSlashCommand(pack, gateEvent.EventID, reportPath)
	applyCommand := adapterReportDraftApplySlashCommand(pack, gateEvent.EventID, reportPath, reportSHA256, opt, report.AdapterID)
	boundary := adapterReportDraftBoundary()
	result := AdapterExecutionReportDraft{
		SchemaVersion:        1,
		Command:              "gate",
		Kind:                 "adapter-execution-report-draft",
		CaseRoot:             caseRoot,
		RepoRoot:             repoRoot,
		Pack:                 pack,
		IsMutation:           opt.ExpectedExecutionReportSHA256 != "",
		Applied:              false,
		Mode:                 "preview",
		GateEventID:          gateEvent.EventID,
		ReportPath:           reportPath,
		ReportSHA256:         reportSHA256,
		RequiresConfirmation: true,
		Report:               report,
		ValidateCommand:      validateCommand,
		ApplyCommand:         applyCommand,
		Boundary:             boundary,
		NextSteps: []string{
			"review the deterministic adapter-report.json draft and exact hash",
			"write or replace only a missing/exact scaffold sidecar: " + applyCommand,
			"run read-only validation: " + validateCommand,
			"record bounded observation evidence only with the hash-bound record command returned by validation/status after valid=true",
		},
	}
	recoveryStatusRequired := contract.LiveValidation.DispatchRequired && contract.LiveValidation.DispatchPresent && report.Status != "failed" && report.Status != "aborted"
	if existing, present, err := readAdapterReportRaw(caseRoot, fullPath, reportPath); err != nil {
		return AdapterExecutionReportDraft{}, nil, nil, "", err
	} else if present {
		if bytes.Equal(existing, data) {
			result.AlreadyExists = true
			result.RequiresConfirmation = false
			result.Mode = "already-drafted"
			result.ApplyCommand = ""
			result.NextSteps = []string{
				"the exact execution report draft already exists",
				"run read-only validation: " + validateCommand,
				"record bounded observation evidence only with the hash-bound record command returned by validation/status after valid=true",
			}
		} else if bytes.Equal(existing, scaffoldData) {
			if recoveryStatusRequired {
				return AdapterExecutionReportDraft{}, nil, nil, "", fmt.Errorf("gate terminal recovery for an existing immutable dispatch requires -ExecutionStatus failed|aborted; got %q", opt.ExecutionStatus)
			}
			result.ReplacesScaffold = true
		} else {
			return AdapterExecutionReportDraft{}, nil, nil, "", fmt.Errorf("gate execution report draft target already exists with different bytes; validate or repair existing sidecar instead: %s", reportPath)
		}
	} else if recoveryStatusRequired {
		return AdapterExecutionReportDraft{}, nil, nil, "", fmt.Errorf("gate terminal recovery for an existing immutable dispatch requires -ExecutionStatus failed|aborted; got %q", opt.ExecutionStatus)
	}
	result.MissionCommanderAction = adapterReportDraftCommanderAction(result)
	result.MissionCommanderNextActions = adapterReportDraftCommanderNextActions(gateEvent, result)
	result.MissionCommanderActionQueue = mission.MissionCommanderActionQueueFor(result.MissionCommanderNextActions)
	result.RunbookSteps = adapterReportRunbookSteps("draft", result.MissionCommanderAction.State, result.ReportPath, result.ReportSHA256, false, false, result.Applied, false, result.NextSteps, result.Boundary, result.MissionCommanderAction)
	return result, data, scaffoldData, fullPath, nil
}

func adapterReportDraftFromOptions(caseRoot string, gateEvent EventPreview, contract AdapterExecutionReportContract, opt Options, m *manifest.Manifest) (AdapterReport, error) {
	status := strings.ToLower(strings.TrimSpace(opt.ExecutionStatus))
	if status == "" {
		return AdapterReport{}, fmt.Errorf("gate -DraftExecutionReport requires -ExecutionStatus succeeded|failed|boundary-hit|escalated|aborted")
	}
	if !validExecutionStatus(status) {
		return AdapterReport{}, fmt.Errorf("invalid ExecutionStatus %q; allowed: succeeded,failed,boundary-hit,escalated,aborted", opt.ExecutionStatus)
	}
	adapterID := strings.TrimSpace(opt.AdapterID)
	if adapterID == "" {
		adapterID = contract.LiveValidation.SidecarTemplate.AdapterID
	}
	if adapterID == "" || adapterID == "<adapter-id>" {
		return AdapterReport{}, fmt.Errorf("gate -DraftExecutionReport requires -AdapterId when no pack tooling adapter can be selected")
	}
	if selected := selectedAdapterToolCandidate(m, gateEvent, adapterID); selected == nil && len(contract.LiveValidation.AdapterCandidates) > 0 {
		return AdapterReport{}, fmt.Errorf("gate -DraftExecutionReport adapterId %q is not one of the selected pack tooling candidates", adapterID)
	}
	outputRefs, err := parseOutputPaths(caseRoot, opt.OutputRefs)
	if err != nil {
		return AdapterReport{}, err
	}
	evidenceRefs, err := validateCaseRelativePaths(caseRoot, "gate execution report draft evidenceRefs", splitList(opt.EvidenceRefs))
	if err != nil {
		return AdapterReport{}, err
	}
	boundaryHits, err := parseBoundaryHits(opt.BoundaryHits)
	if err != nil {
		return AdapterReport{}, err
	}
	summary := strings.TrimSpace(opt.Summary)
	if summary == "" {
		summary = "Adapter reported " + status + " for authorized " + gateEvent.Gate.Action + " gate"
	}
	var reportDispatch *adapterexecution.ReportDispatchBinding
	dispatchRequired, err := adapterExecutionReceiptRequired(caseRoot, gateEvent, m)
	if err != nil {
		return AdapterReport{}, err
	}
	if dispatchRequired {
		dispatch, dispatchPath, dispatchSHA, dispatchBytes, dispatchErr := readCurrentAdapterExecutionDispatch(caseRoot, contract.Pack, gateEvent, m)
		if dispatchErr != nil {
			return AdapterReport{}, fmt.Errorf("gate execution report draft requires immutable dispatch before external execution: %w", dispatchErr)
		}
		if dispatchBytes <= 0 {
			return AdapterReport{}, fmt.Errorf("gate execution report draft dispatch byte binding is invalid")
		}
		reportDispatch = &adapterexecution.ReportDispatchBinding{DispatchID: dispatch.DispatchID, Path: dispatchPath, SHA256: dispatchSHA}
	}
	report := AdapterReport{
		SchemaVersion: 1,
		Kind:          "adapter-execution-report",
		AdapterID:     adapterID,
		Action:        gateEvent.Gate.Action,
		Status:        status,
		GateEventID:   gateEvent.EventID,
		Dispatch:      reportDispatch,
		ActualBudget:  autonomy.Budget{RuntimeSeconds: opt.ActualRuntimeSeconds, DiskMB: opt.ActualDiskMB, Requests: opt.ActualRequests},
		OutputRefs:    outputRefs,
		EvidenceRefs:  evidenceRefs,
		BoundaryHits:  boundaryHits,
		Escalation:    strings.TrimSpace(opt.Escalation),
		Summary:       summary,
	}
	if err := validateAdapterExecutionReport(caseRoot, gateEvent, &report); err != nil {
		return AdapterReport{}, err
	}
	return report, nil
}

func adapterReportBytes(report AdapterReport) ([]byte, error) {
	data, err := json.MarshalIndent(report, "", "  ")
	if err != nil {
		return nil, err
	}
	return append(data, '\n'), nil
}

func adapterReportScaffoldBytes(template AdapterReportSidecarTemplate) ([]byte, error) {
	data, err := json.MarshalIndent(template, "", "  ")
	if err != nil {
		return nil, err
	}
	return append(data, '\n'), nil
}

func adapterExecutionReportScaffoldPath(caseRoot string, gateEvent EventPreview, cwd, value string) (string, string, error) {
	path := strings.TrimSpace(value)
	if path == "" {
		path = adapterReportDefaultPath(gateEvent.Gate.OutputPaths)
	}
	if path == "" {
		return "", "", fmt.Errorf("gate execution report scaffold requires -ExecutionReportPath or an authorized outputPath default")
	}
	if len(splitList(path)) != 1 {
		return "", "", adapterReportValidationErrorf("path-list", "path", "gate execution report path must be a single file path")
	}
	fullPath, relPath, err := executionReportPath(caseRoot, path)
	if err != nil {
		return "", "", adapterReportValidationErrorf("path-invalid", "path", "%w", err)
	}
	if !outputRefsWithinGate(gateEvent.Gate.OutputPaths, []string{relPath}) {
		if cwdFullPath, cwdRelPath, ok, err := cwdAuthorizedExecutionReportPath(caseRoot, gateEvent, cwd, path); err != nil {
			return "", "", adapterReportValidationErrorf("path-invalid", "path", "%w", err)
		} else if ok {
			fullPath = cwdFullPath
			relPath = cwdRelPath
		} else {
			return "", "", adapterReportValidationErrorf("report-path-out-of-scope", "path", "gate execution report path must stay within authorized gate outputPaths")
		}
	}
	return fullPath, relPath, nil
}

func IsAuthorizedAdapterReportAttempt(repoRoot, caseRoot, pack, gateEventID, lane, action, reportPath string) (bool, error) {
	_, event, err := authorizedGateEvent(repoRoot, caseRoot, pack, Options{GateEventID: gateEventID})
	if err != nil {
		return false, nil
	}
	if event.EventID != strings.TrimSpace(gateEventID) || event.Lane != strings.TrimSpace(lane) || event.Gate.Action != strings.TrimSpace(action) {
		return false, nil
	}
	_, relPath, err := executionReportPath(caseRoot, reportPath)
	if err != nil {
		return false, err
	}
	return outputRefsWithinGate(event.Gate.OutputPaths, []string{relPath}), nil
}

func ReadAdapterExecutionReportIdentity(caseRoot, reportPath string) (string, bool, error) {
	fullPath, relPath, err := executionReportPath(caseRoot, reportPath)
	if err != nil {
		return "", false, err
	}
	data, present, err := readAdapterReportRaw(caseRoot, fullPath, relPath)
	if err != nil || !present {
		return "", present, err
	}
	var identity struct {
		GateEventID string `json:"gateEventId"`
	}
	var fields map[string]json.RawMessage
	if err := json.Unmarshal(data, &fields); err != nil {
		return "", true, err
	}
	raw, ok := fields["gateEventId"]
	if !ok || json.Unmarshal(raw, &identity.GateEventID) != nil {
		return "", true, fmt.Errorf("adapter execution report gateEventId is not a string: %s", relPath)
	}
	return strings.TrimSpace(identity.GateEventID), true, nil
}

func readAdapterReportRaw(caseRoot, fullPath, relPath string) ([]byte, bool, error) {
	if err := rejectAdapterReportSymlinkExistingPath(caseRoot, fullPath); err != nil {
		if os.IsNotExist(err) {
			return nil, false, nil
		}
		return nil, false, err
	}
	st, err := os.Lstat(fullPath)
	if os.IsNotExist(err) {
		return nil, false, nil
	}
	if err != nil {
		return nil, true, adapterReportValidationErrorf("report-not-readable", "read", "read adapter execution report %s: %w", relPath, err)
	}
	if st.Mode()&os.ModeSymlink != 0 || !st.Mode().IsRegular() {
		if st.IsDir() {
			return nil, true, adapterReportValidationErrorf("report-path-directory", "read", "adapter execution report path is a directory: %s", relPath)
		}
		return nil, true, adapterReportValidationErrorf("report-not-regular", "read", "adapter execution report must be a regular non-symlink file: %s", relPath)
	}
	if st.Size() > 1<<20 {
		return nil, true, adapterReportValidationErrorf("report-too-large", "read", "adapter execution report is too large: %s %d > %d", relPath, st.Size(), 1<<20)
	}
	f, err := os.Open(fullPath)
	if err != nil {
		return nil, true, adapterReportValidationErrorf("report-not-readable", "read", "read adapter execution report %s: %w", relPath, err)
	}
	defer f.Close()
	opened, err := f.Stat()
	if err != nil {
		return nil, true, adapterReportValidationErrorf("report-not-readable", "read", "stat opened adapter execution report %s: %w", relPath, err)
	}
	if !opened.Mode().IsRegular() || !os.SameFile(st, opened) {
		return nil, true, adapterReportValidationErrorf("report-not-regular", "read", "adapter execution report changed or is not a regular file: %s", relPath)
	}
	data, err := io.ReadAll(io.LimitReader(f, 1<<20+1))
	if err != nil {
		return nil, true, adapterReportValidationErrorf("report-not-readable", "read", "read adapter execution report %s: %w", relPath, err)
	}
	if len(data) > 1<<20 {
		return nil, true, adapterReportValidationErrorf("report-too-large", "read", "adapter execution report is too large: %s %d > %d", relPath, len(data), 1<<20)
	}
	postOpen, err := os.Lstat(fullPath)
	if err != nil || postOpen.Mode()&os.ModeSymlink != 0 || !postOpen.Mode().IsRegular() || !os.SameFile(opened, postOpen) {
		return nil, true, adapterReportValidationErrorf("report-not-regular", "read", "adapter execution report path changed after open: %s", relPath)
	}
	return data, true, nil
}

func writeAdapterReportScaffold(caseRoot, fullPath, relPath string, data []byte) error {
	return writeAdapterReportBytes(caseRoot, fullPath, relPath, data, nil, false)
}

func writeAdapterReportDraft(caseRoot, fullPath, relPath string, data, scaffoldData []byte) error {
	return writeAdapterReportBytes(caseRoot, fullPath, relPath, data, scaffoldData, true)
}

func writeAdapterReportBytes(caseRoot, fullPath, relPath string, data, replaceData []byte, allowReplace bool) error {
	if err := ensureAdapterReportParent(caseRoot, filepath.Dir(fullPath)); err != nil {
		return err
	}
	if existing, present, err := readAdapterReportRaw(caseRoot, fullPath, relPath); err != nil {
		return err
	} else if present {
		if bytes.Equal(existing, data) {
			return nil
		}
		if allowReplace && bytes.Equal(existing, replaceData) {
			return os.WriteFile(fullPath, data, 0o644)
		}
		return fmt.Errorf("gate execution report target already exists with different bytes; refusing overwrite: %s", relPath)
	}
	f, err := os.OpenFile(fullPath, os.O_WRONLY|os.O_CREATE|os.O_EXCL, 0o644)
	if os.IsExist(err) {
		if existing, present, readErr := readAdapterReportRaw(caseRoot, fullPath, relPath); readErr != nil {
			return readErr
		} else if present && bytes.Equal(existing, data) {
			return nil
		} else if present && allowReplace && bytes.Equal(existing, replaceData) {
			return os.WriteFile(fullPath, data, 0o644)
		}
	}
	if err != nil {
		return err
	}
	_, writeErr := f.Write(data)
	closeErr := f.Close()
	if writeErr != nil {
		return writeErr
	}
	if closeErr != nil {
		return closeErr
	}
	return ensureAdapterReportParent(caseRoot, filepath.Dir(fullPath))
}

func ensureAdapterReportParent(caseRoot, parentPath string) error {
	caseAbs, err := filepath.Abs(caseRoot)
	if err != nil {
		return err
	}
	parentAbs, err := filepath.Abs(parentPath)
	if err != nil {
		return err
	}
	caseClean := strings.TrimRight(filepath.Clean(caseAbs), string(filepath.Separator))
	parentClean := strings.TrimRight(filepath.Clean(parentAbs), string(filepath.Separator))
	prefix := caseClean + string(filepath.Separator)
	if !strings.EqualFold(parentClean, caseClean) && !strings.HasPrefix(strings.ToLower(parentClean), strings.ToLower(prefix)) {
		return fmt.Errorf("gate execution report path must stay within case root: %s", parentPath)
	}
	rel, err := filepath.Rel(caseClean, parentClean)
	if err != nil {
		return err
	}
	current := caseClean
	if rel == "." {
		return nil
	}
	for component := range strings.SplitSeq(rel, string(filepath.Separator)) {
		if component == "" || component == "." || component == ".." {
			return fmt.Errorf("gate execution report path contains invalid parent component: %s", parentPath)
		}
		current = filepath.Join(current, component)
		st, err := os.Lstat(current)
		if os.IsNotExist(err) {
			if err := os.Mkdir(current, 0o755); err != nil && !os.IsExist(err) {
				return err
			}
			st, err = os.Lstat(current)
		}
		if err != nil {
			return err
		}
		if st.Mode()&os.ModeSymlink != 0 || !st.Mode().IsDir() {
			return adapterReportValidationErrorf("report-not-regular", "read", "adapter execution report parent must be a non-symlink directory: %s", current)
		}
	}
	return nil
}

func sha256HexBytes(data []byte) string {
	sum := sha256.Sum256(data)
	return hex.EncodeToString(sum[:])
}

func adapterReportScaffoldSlashCommand(pack, gateEventID, reportPath, reportSHA256 string) string {
	return adapterReportSlashCommand([]string{"gate", "-Pack", pack, "-GateEventId", gateEventID, "-ScaffoldExecutionReport", "-ExecutionReportPath", reportPath, "-ExpectedExecutionReportSha256", reportSHA256, "-Apply", "-Format", "json"})
}

func adapterReportDraftSlashCommand(pack, gateEventID, reportPath string) string {
	return adapterReportSlashCommand([]string{"gate", "-Pack", pack, "-GateEventId", gateEventID, "-DraftExecutionReport", "-ExecutionReportPath", reportPath, "-Format", "json"})
}

func adapterReportDraftApplySlashCommand(pack, gateEventID, reportPath, reportSHA256 string, opt Options, adapterID string) string {
	args := []string{"gate", "-Pack", pack, "-GateEventId", gateEventID, "-DraftExecutionReport", "-ExecutionReportPath", reportPath}
	args = appendGateCommandArg(args, "-AdapterId", adapterID)
	args = appendGateCommandArg(args, "-ExecutionStatus", opt.ExecutionStatus)
	args = appendGateCommandIntArg(args, "-ActualRuntimeSeconds", opt.ActualRuntimeSeconds)
	args = appendGateCommandIntArg(args, "-ActualDiskMB", opt.ActualDiskMB)
	args = appendGateCommandIntArg(args, "-ActualRequests", opt.ActualRequests)
	args = appendGateCommandArg(args, "-OutputRefs", opt.OutputRefs)
	args = appendGateCommandArg(args, "-ExecutionEvidenceRefs", opt.EvidenceRefs)
	args = appendGateCommandArg(args, "-BoundaryHits", opt.BoundaryHits)
	args = appendGateCommandArg(args, "-Escalation", opt.Escalation)
	args = appendGateCommandArg(args, "-Summary", opt.Summary)
	args = append(args, "-ExpectedExecutionReportSha256", reportSHA256, "-Apply", "-Format", "json")
	return adapterReportSlashCommand(args)
}

func adapterReportScaffoldBoundary() []string {
	return []string{
		"scaffold writes only a bounded adapter-report.json template under authorized outputPaths",
		"scaffold does not execute the adapter or heavy tool",
		"scaffold does not validate or record observation evidence",
		"do not record evidence until read-only validation returns valid=true",
		"no observations/authority/confirmed writes during scaffold preview",
		"refuse to overwrite a different existing sidecar",
	}
}

func adapterReportDraftBoundary() []string {
	return []string{
		"draft writes only a bounded adapter-report.json sidecar under authorized outputPaths",
		"draft records executor-reported fields; it does not execute the adapter or heavy tool",
		"draft validates fields before writing but does not record observation evidence",
		"draft may replace the exact scaffold template and refuses different existing sidecars",
		"record evidence only after a separate read-only validation returns valid=true",
		"no observations/authority/confirmed writes during draft preview/apply",
	}
}

func adapterReportScaffoldCommanderAction(result AdapterExecutionReportScaffold) mission.MissionCommanderAction {
	state := "ready-for-adapter-report-scaffold-apply"
	primary := result.ApplyCommand
	followUps := []string{result.ValidateCommand}
	prompt := fmt.Sprintf("review scaffold preview for authorized gate `%s`, then write missing bounded adapter-report.json; external adapter fills fields before read-only validation, and record only with validation/status returned hash-bound command after valid=true.", result.GateEventID)
	boundary := append([]string{}, result.Boundary...)
	boundary = append(boundary, "scaffold handoff does not provide a runnable record Apply; use validation/status returned -ExpectedExecutionReportSha256 after valid=true")
	if result.Applied || result.AlreadyExists {
		state = "adapter-report-scaffolded-awaiting-adapter-output"
		primary = result.ValidateCommand
		followUps = nil
		prompt = fmt.Sprintf("authorized gate `%s` has an adapter-report.json scaffold; let the external adapter fill bounded fields, then run read-only validation; record only with validation/status returned hash-bound command after valid=true.", result.GateEventID)
	}
	return mission.MissionCommanderAction{State: state, Prompt: prompt, PrimaryCommand: primary, FollowUpCommands: followUps, Boundary: boundary}
}

func adapterReportScaffoldCommanderNextActions(gateEvent EventPreview, result AdapterExecutionReportScaffold) []mission.MissionCommanderNextActionItem {
	label := gateCommanderActionLabel(gateEvent.Lane)
	items := []mission.MissionCommanderNextActionItem{}
	state := result.MissionCommanderAction.State
	if result.ApplyCommand != "" && !result.Applied && !result.AlreadyExists {
		items = append(items, adapterReportNextActionItem(gateEvent, label, "adapter-report-scaffold-apply", state, result.ApplyCommand, "adapterReportScaffold.preview", false, true, []string{"write missing adapter-report.json scaffold after reviewing the deterministic template hash"}, result.Boundary))
	}
	if result.ValidateCommand != "" {
		items = append(items, adapterReportNextActionItem(gateEvent, label, "adapter-report-scaffold-validation", state, result.ValidateCommand, "adapterReportScaffold.validation", true, true, []string{"run only after the external adapter fills placeholder execution fields"}, append(append([]string{}, result.Boundary...), "validation remains read-only")))
	}
	items = append(items, adapterReportNextActionItem(gateEvent, label, "adapter-report-scaffold-handoff", state, "/rekit handoff "+label, "adapterReportScaffold.followUp", false, true, []string{"handoff scaffold status and adapter output expectations"}, result.Boundary))
	return mission.UniqueCommanderNextActions(items)
}

func adapterReportDraftCommanderAction(result AdapterExecutionReportDraft) mission.MissionCommanderAction {
	state := "ready-for-adapter-report-draft-apply"
	primary := result.ApplyCommand
	followUps := []string{result.ValidateCommand}
	prompt := fmt.Sprintf("review draft preview for authorized gate `%s`, then write bounded adapter-report.json fields without executing the adapter; record only with validation/status returned hash-bound command after valid=true.", result.GateEventID)
	boundary := append([]string{}, result.Boundary...)
	boundary = append(boundary, "draft handoff does not provide a runnable record Apply; use validation/status returned -ExpectedExecutionReportSha256 after valid=true")
	if result.Applied || result.AlreadyExists {
		state = "adapter-report-drafted-ready-for-validation"
		primary = result.ValidateCommand
		followUps = nil
		prompt = fmt.Sprintf("authorized gate `%s` has a deterministic adapter-report.json draft; run read-only validation before recording evidence, then use validation/status returned hash-bound record command after valid=true.", result.GateEventID)
	}
	return mission.MissionCommanderAction{State: state, Prompt: prompt, PrimaryCommand: primary, FollowUpCommands: followUps, Boundary: boundary}
}

func adapterReportDraftCommanderNextActions(gateEvent EventPreview, result AdapterExecutionReportDraft) []mission.MissionCommanderNextActionItem {
	label := gateCommanderActionLabel(gateEvent.Lane)
	items := []mission.MissionCommanderNextActionItem{}
	state := result.MissionCommanderAction.State
	if result.ApplyCommand != "" && !result.Applied && !result.AlreadyExists {
		reasons := []string{"write deterministic adapter-report.json draft after reviewing reported execution fields and exact hash"}
		if result.ReplacesScaffold {
			reasons = append(reasons, "target is the exact scaffold template and can be replaced by the bounded draft")
		}
		items = append(items, adapterReportNextActionItem(gateEvent, label, "adapter-report-draft-apply", state, result.ApplyCommand, "adapterReportDraft.preview", false, true, reasons, result.Boundary))
	}
	if result.ValidateCommand != "" {
		items = append(items, adapterReportNextActionItem(gateEvent, label, "adapter-report-draft-validation", state, result.ValidateCommand, "adapterReportDraft.validation", !result.Applied && !result.AlreadyExists, true, []string{"run read-only validation after the deterministic draft is written"}, append(append([]string{}, result.Boundary...), "validation remains read-only")))
	}
	items = append(items, adapterReportNextActionItem(gateEvent, label, "adapter-report-draft-handoff", state, "/rekit handoff "+label, "adapterReportDraft.followUp", false, true, []string{"handoff draft status and validation expectations"}, result.Boundary))
	return mission.UniqueCommanderNextActions(items)
}

func executionEvidence(caseRoot string, gateEvent EventPreview, opt Options, m *manifest.Manifest) (ExecutionEvidencePreview, error) {
	reportRel, reportSHA256, adapterReport, err := readAdapterExecutionReport(caseRoot, gateEvent, opt.ExecutionReportCwd, opt.ExecutionReportPath)
	if err != nil {
		return ExecutionEvidencePreview{}, err
	}
	receiptRequired, err := adapterExecutionReceiptRequired(caseRoot, gateEvent, m)
	if err != nil {
		return ExecutionEvidencePreview{}, err
	}
	if receiptRequired && adapterReport == nil {
		return ExecutionEvidencePreview{}, fmt.Errorf("managed adapter execution provenance requires -ExecutionReportPath and an immutable receipt")
	}
	var adapterContext *AdapterToolCandidate
	if adapterReport != nil {
		adapterContext = selectedAdapterToolCandidate(m, gateEvent, adapterReport.AdapterID)
	}
	status := strings.ToLower(strings.TrimSpace(opt.ExecutionStatus))
	if status == "" && adapterReport != nil {
		status = adapterReport.Status
	}
	if status == "" {
		return ExecutionEvidencePreview{}, fmt.Errorf("gate execution evidence requires -ExecutionStatus succeeded|failed|boundary-hit|escalated|aborted")
	}
	if !validExecutionStatus(status) {
		return ExecutionEvidencePreview{}, fmt.Errorf("invalid ExecutionStatus %q; allowed: succeeded,failed,boundary-hit,escalated,aborted", opt.ExecutionStatus)
	}
	if adapterReport != nil && adapterReport.Status != status {
		return ExecutionEvidencePreview{}, fmt.Errorf("adapter execution report status %q does not match ExecutionStatus %q", adapterReport.Status, status)
	}
	actual := autonomy.Budget{RuntimeSeconds: opt.ActualRuntimeSeconds, DiskMB: opt.ActualDiskMB, Requests: opt.ActualRequests}
	if adapterReport != nil {
		if actualBudgetFieldMismatch(actual, adapterReport.ActualBudget) {
			return ExecutionEvidencePreview{}, fmt.Errorf("adapter execution report actualBudget does not match explicit actual budget flags")
		}
		if actual == (autonomy.Budget{}) {
			actual = adapterReport.ActualBudget
		}
	}
	if actual.RuntimeSeconds < 0 || actual.DiskMB < 0 || actual.Requests < 0 {
		return ExecutionEvidencePreview{}, fmt.Errorf("gate execution evidence actual budget values must be non-negative")
	}
	outputRefs, err := parseOutputPaths(caseRoot, opt.OutputRefs)
	if err != nil {
		return ExecutionEvidencePreview{}, err
	}
	if adapterReport != nil {
		if len(outputRefs) > 0 && len(adapterReport.OutputRefs) > 0 && strings.Join(normalizedGatePaths(outputRefs), ",") != strings.Join(normalizedGatePaths(adapterReport.OutputRefs), ",") {
			return ExecutionEvidencePreview{}, fmt.Errorf("adapter execution report outputRefs do not match explicit OutputRefs")
		}
		if len(outputRefs) == 0 {
			outputRefs = adapterReport.OutputRefs
		}
	}
	if len(outputRefs) > 0 && !outputRefsWithinGate(gateEvent.Gate.OutputPaths, outputRefs) {
		return ExecutionEvidencePreview{}, fmt.Errorf("gate execution evidence outputRefs must stay within authorized gate outputPaths")
	}
	evidenceRefs, err := validateCaseRelativePaths(caseRoot, "gate execution evidence evidenceRefs", splitList(opt.EvidenceRefs))
	if err != nil {
		return ExecutionEvidencePreview{}, err
	}
	if adapterReport != nil {
		if len(evidenceRefs) > 0 && len(adapterReport.EvidenceRefs) > 0 && strings.Join(normalizedGatePaths(evidenceRefs), ",") != strings.Join(normalizedGatePaths(adapterReport.EvidenceRefs), ",") {
			return ExecutionEvidencePreview{}, fmt.Errorf("adapter execution report evidenceRefs do not match explicit ExecutionEvidenceRefs")
		}
		if len(evidenceRefs) == 0 {
			evidenceRefs = append([]string{}, adapterReport.EvidenceRefs...)
		}
	}
	if len(evidenceRefs) > 0 && !outputRefsWithinGate(gateEvent.Gate.OutputPaths, evidenceRefs) {
		return ExecutionEvidencePreview{}, fmt.Errorf("gate execution evidence evidenceRefs must stay within authorized gate outputPaths")
	}
	if len(outputRefs) == 0 && len(evidenceRefs) == 0 && reportRel == "" {
		return ExecutionEvidencePreview{}, fmt.Errorf("gate execution evidence requires -OutputRefs, -ExecutionEvidenceRefs, or -ExecutionReportPath")
	}
	if reportRel != "" {
		expectedReportSHA256 := strings.TrimSpace(opt.ExpectedExecutionReportSHA256)
		if expectedReportSHA256 != "" {
			decoded, decodeErr := hex.DecodeString(expectedReportSHA256)
			if decodeErr != nil || len(decoded) != sha256.Size {
				return ExecutionEvidencePreview{}, fmt.Errorf("gate execution evidence requires a valid -ExpectedExecutionReportSha256 from read-only validation")
			}
			if !strings.EqualFold(expectedReportSHA256, reportSHA256) {
				return ExecutionEvidencePreview{}, fmt.Errorf("adapter execution report sha256 changed after validation: expected %s got %s", expectedReportSHA256, reportSHA256)
			}
		}
	} else if strings.TrimSpace(opt.ExpectedExecutionReportSHA256) != "" {
		return ExecutionEvidencePreview{}, fmt.Errorf("gate execution evidence -ExpectedExecutionReportSha256 requires -ExecutionReportPath")
	}
	var adapterExecutionDispatch *adapterexecution.DispatchReceipt
	adapterExecutionDispatchPath := ""
	adapterExecutionDispatchSHA256 := ""
	var adapterExecution *adapterexecution.Receipt
	adapterExecutionReceiptPath := ""
	adapterExecutionReceiptSHA256 := ""
	if receiptRequired {
		adapterExecution, adapterExecutionReceiptPath, adapterExecutionReceiptSHA256, err = validateRecordedAdapterExecutionReceipt(caseRoot, m.Pack, gateEvent, reportRel, reportSHA256, adapterReport, m, opt)
		if err != nil {
			return ExecutionEvidencePreview{}, err
		}
		if strings.TrimSpace(opt.ExpectedAdapterExecutionReceiptSHA256) == "" {
			return ExecutionEvidencePreview{}, fmt.Errorf("gate execution evidence requires -ExpectedAdapterExecutionReceiptSha256 from read-only validation")
		}
		dispatch, dispatchPath, dispatchSHA, _, dispatchErr :=
			readCurrentAdapterExecutionDispatch(
				caseRoot,
				m.Pack,
				gateEvent,
				m,
			)
		if dispatchErr != nil {
			return ExecutionEvidencePreview{}, dispatchErr
		}
		adapterExecutionDispatch = &dispatch
		adapterExecutionDispatchPath = dispatchPath
		adapterExecutionDispatchSHA256 = dispatchSHA
	}
	boundaryHits, err := parseBoundaryHits(opt.BoundaryHits)
	if err != nil {
		return ExecutionEvidencePreview{}, err
	}
	if adapterReport != nil {
		if len(boundaryHits) > 0 && len(adapterReport.BoundaryHits) > 0 && strings.Join(boundaryHits, ",") != strings.Join(adapterReport.BoundaryHits, ",") {
			return ExecutionEvidencePreview{}, fmt.Errorf("adapter execution report boundaryHits do not match explicit BoundaryHits")
		}
		if len(boundaryHits) > 0 && !boundaryHitsWithinGate(gateEvent.Gate.StopConditions, boundaryHits) {
			return ExecutionEvidencePreview{}, fmt.Errorf("gate execution evidence boundaryHits must be covered by authorized gate stopConditions")
		}
		if len(boundaryHits) == 0 {
			boundaryHits = append([]string{}, adapterReport.BoundaryHits...)
		}
	}
	escalation := strings.TrimSpace(opt.Escalation)
	if adapterReport != nil {
		if escalation != "" && adapterReport.Escalation != "" && escalation != adapterReport.Escalation {
			return ExecutionEvidencePreview{}, fmt.Errorf("adapter execution report escalation does not match explicit Escalation")
		}
		if escalation == "" {
			escalation = adapterReport.Escalation
		}
	}
	if len(boundaryHits) > 0 && !boundaryHitsWithinGate(gateEvent.Gate.StopConditions, boundaryHits) {
		return ExecutionEvidencePreview{}, fmt.Errorf("gate execution evidence boundaryHits must be covered by authorized gate stopConditions")
	}
	if exceedsGateBudget(gateEvent.Gate.RequestedBudget, actual) && len(boundaryHits) == 0 && escalation == "" {
		return ExecutionEvidencePreview{}, fmt.Errorf("gate execution evidence actual budget exceeds authorized request; record -BoundaryHits or -Escalation")
	}
	if (status == "boundary-hit" || status == "escalated") && len(boundaryHits) == 0 && escalation == "" {
		return ExecutionEvidencePreview{}, fmt.Errorf("gate execution evidence status %s requires -BoundaryHits or -Escalation", status)
	}
	subject := strings.TrimSpace(opt.Subject)
	if subject == "" {
		subject = "execution evidence for " + mission.Subject(gateEventMap(gateEvent))
	}
	summary := strings.TrimSpace(opt.Summary)
	if summary == "" && adapterReport != nil && strings.TrimSpace(adapterReport.Summary) != "" {
		summary = strings.TrimSpace(adapterReport.Summary)
	}
	if summary == "" {
		summary = "Recorded execution evidence for authorized " + gateEvent.Gate.Action + " gate"
	}
	return ExecutionEvidencePreview{
		SchemaVersion: 1,
		Kind:          "observation",
		Lane:          gateEvent.Lane,
		Subject:       subject,
		Summary:       summary,
		Status:        status,
		Actor:         strings.TrimSpace(opt.Actor),
		Risk:          gateEvent.Risk,
		Target:        gateEvent.Target,
		BatchID:       gateEvent.BatchID,
		Related:       []string{gateEvent.EventID},
		EvidenceRefs:  evidenceRefs,
		Gate:          gateEvent.Gate,
		Execution: ExecutionEvidenceDetails{
			Status:                         status,
			ActualBudget:                   actual,
			OutputRefs:                     outputRefs,
			BoundaryHits:                   boundaryHits,
			Escalation:                     escalation,
			GateEventID:                    gateEvent.EventID,
			GateStatus:                     gateEvent.Status,
			Authorization:                  gateEvent.Gate.Authorization.Decision,
			RecordRequired:                 gateEvent.Gate.Authorization.RecordRequired,
			NotifyMainOn:                   append([]string{}, gateEvent.Gate.Authorization.NotifyMainOn...),
			ExecutionReportPath:            reportRel,
			ExecutionReportSHA256:          reportSHA256,
			AdapterExecutionDispatchPath:   adapterExecutionDispatchPath,
			AdapterExecutionDispatchSHA256: adapterExecutionDispatchSHA256,
			AdapterExecutionDispatch:       adapterExecutionDispatch,
			AdapterExecutionReceiptPath:    adapterExecutionReceiptPath,
			AdapterExecutionReceiptSHA256:  adapterExecutionReceiptSHA256,
			AdapterExecution:               adapterExecution,
			AdapterContext:                 adapterContext,
			Adapter:                        adapterReport,
		},
	}, nil
}

func actualBudgetFieldMismatch(explicit, reported autonomy.Budget) bool {
	return (explicit.RuntimeSeconds != 0 && explicit.RuntimeSeconds != reported.RuntimeSeconds) || (explicit.DiskMB != 0 && explicit.DiskMB != reported.DiskMB) || (explicit.Requests != 0 && explicit.Requests != reported.Requests)
}

func readAdapterExecutionReport(caseRoot string, gateEvent EventPreview, cwd, value string) (string, string, *AdapterReport, error) {
	path := strings.TrimSpace(value)
	if path == "" {
		return "", "", nil, nil
	}
	if len(splitList(path)) != 1 {
		return "", "", nil, adapterReportValidationErrorf("path-list", "path", "gate execution report path must be a single file path")
	}
	fullPath, relPath, err := executionReportPath(caseRoot, path)
	if err != nil {
		return "", "", nil, adapterReportValidationErrorf("path-invalid", "path", "%w", err)
	}
	if !outputRefsWithinGate(gateEvent.Gate.OutputPaths, []string{relPath}) {
		if cwdFullPath, cwdRelPath, ok, err := cwdAuthorizedExecutionReportPath(caseRoot, gateEvent, cwd, path); err != nil {
			return "", "", nil, adapterReportValidationErrorf("path-invalid", "path", "%w", err)
		} else if ok {
			fullPath = cwdFullPath
			relPath = cwdRelPath
		} else {
			return relPath, "", nil, adapterReportValidationErrorf("report-path-out-of-scope", "path", "gate execution report path must stay within authorized gate outputPaths")
		}
	}
	if err := rejectAdapterReportSymlinkPath(caseRoot, fullPath); err != nil {
		return relPath, "", nil, err
	}
	st, err := os.Lstat(fullPath)
	if err != nil {
		return relPath, "", nil, adapterReportValidationErrorf("report-not-readable", "read", "read adapter execution report %s: %w", relPath, err)
	}
	if st.Mode()&os.ModeSymlink != 0 || !st.Mode().IsRegular() {
		if st.IsDir() {
			return relPath, "", nil, adapterReportValidationErrorf("report-path-directory", "read", "adapter execution report path is a directory: %s", relPath)
		}
		return relPath, "", nil, adapterReportValidationErrorf("report-not-regular", "read", "adapter execution report must be a regular non-symlink file: %s", relPath)
	}
	if st.Size() > 1<<20 {
		return relPath, "", nil, adapterReportValidationErrorf("report-too-large", "read", "adapter execution report is too large: %s %d > %d", relPath, st.Size(), 1<<20)
	}
	f, err := os.Open(fullPath)
	if err != nil {
		return relPath, "", nil, adapterReportValidationErrorf("report-not-readable", "read", "read adapter execution report %s: %w", relPath, err)
	}
	defer f.Close()
	opened, err := f.Stat()
	if err != nil {
		return relPath, "", nil, adapterReportValidationErrorf("report-not-readable", "read", "stat opened adapter execution report %s: %w", relPath, err)
	}
	if !opened.Mode().IsRegular() || !os.SameFile(st, opened) {
		return relPath, "", nil, adapterReportValidationErrorf("report-not-regular", "read", "adapter execution report changed or is not a regular file: %s", relPath)
	}
	if err := rejectAdapterReportSymlinkPath(caseRoot, fullPath); err != nil {
		return relPath, "", nil, err
	}
	data, err := io.ReadAll(io.LimitReader(f, 1<<20+1))
	if err != nil {
		return relPath, "", nil, adapterReportValidationErrorf("report-not-readable", "read", "read adapter execution report %s: %w", relPath, err)
	}
	if len(data) > 1<<20 {
		return relPath, "", nil, adapterReportValidationErrorf("report-too-large", "read", "adapter execution report is too large: %s %d > %d", relPath, len(data), 1<<20)
	}
	postOpen, err := os.Lstat(fullPath)
	if err != nil || postOpen.Mode()&os.ModeSymlink != 0 || !postOpen.Mode().IsRegular() || !os.SameFile(opened, postOpen) {
		return relPath, "", nil, adapterReportValidationErrorf("report-not-regular", "read", "adapter execution report path changed after open: %s", relPath)
	}
	reportSHA256 := sha256HexBytes(data)
	var report AdapterReport
	dec := json.NewDecoder(bytes.NewReader(data))
	dec.DisallowUnknownFields()
	if err := dec.Decode(&report); err != nil {
		return relPath, reportSHA256, nil, adapterReportValidationErrorf("report-json-invalid", "decode", "invalid adapter execution report %s: %w", relPath, err)
	}
	var trailing any
	if err := dec.Decode(&trailing); err != io.EOF {
		if err == nil {
			return relPath, reportSHA256, &report, adapterReportValidationErrorf("report-trailing-data", "decode", "invalid adapter execution report %s: trailing data", relPath)
		}
		return relPath, reportSHA256, &report, adapterReportValidationErrorf("report-trailing-data", "decode", "invalid adapter execution report %s: trailing data: %w", relPath, err)
	}
	if err := validateAdapterExecutionReport(caseRoot, gateEvent, &report); err != nil {
		return relPath, reportSHA256, &report, err
	}
	return relPath, reportSHA256, &report, nil
}

func rejectAdapterReportSymlinkExistingPath(caseRoot, path string) error {
	if _, err := os.Lstat(path); os.IsNotExist(err) {
		return os.ErrNotExist
	} else if err != nil {
		return adapterReportValidationErrorf("report-not-readable", "read", "read adapter execution report path %s: %w", path, err)
	}
	return rejectAdapterReportSymlinkPath(caseRoot, path)
}

func rejectAdapterReportSymlinkPath(caseRoot, path string) error {
	rootFull, err := filepath.Abs(caseRoot)
	if err != nil {
		return adapterReportValidationErrorf("path-invalid", "path", "%w", err)
	}
	pathFull, err := filepath.Abs(path)
	if err != nil {
		return adapterReportValidationErrorf("path-invalid", "path", "%w", err)
	}
	rel, err := filepath.Rel(rootFull, pathFull)
	if err != nil || rel == ".." || strings.HasPrefix(rel, ".."+string(filepath.Separator)) || filepath.IsAbs(rel) {
		return adapterReportValidationErrorf("path-invalid", "path", "adapter execution report path escapes case root: %s", path)
	}
	current := rootFull
	parts := strings.Split(rel, string(filepath.Separator))
	for i, part := range parts {
		if part == "" || part == "." {
			continue
		}
		current = filepath.Join(current, part)
		st, err := os.Lstat(current)
		if err != nil {
			if os.IsNotExist(err) && i == len(parts)-1 {
				return os.ErrNotExist
			}
			return adapterReportValidationErrorf("report-not-readable", "read", "read adapter execution report path %s: %w", current, err)
		}
		if st.Mode()&os.ModeSymlink != 0 {
			return adapterReportValidationErrorf("report-not-regular", "read", "adapter execution report path must not traverse symlink: %s", current)
		}
	}
	return nil
}

func executionReportPath(caseRoot, value string) (string, string, error) {
	if filepath.IsAbs(value) {
		return caseContainedPath(caseRoot, value)
	}
	clean, err := validateCaseRelativePath(caseRoot, "gate execution report path", value)
	if err != nil {
		return "", "", err
	}
	fullPath, err := refsf.SafeJoin(caseRoot, clean)
	if err != nil {
		return "", "", fmt.Errorf("gate execution report path escapes case root: %s", value)
	}
	return fullPath, clean, nil
}

func cwdRelativeCasePath(caseRoot, cwd, value string) (string, string, bool, error) {
	cwdAbs, err := filepath.Abs(cwd)
	if err != nil {
		return "", "", false, err
	}
	caseAbs, err := filepath.Abs(caseRoot)
	if err != nil {
		return "", "", false, err
	}
	caseClean := strings.TrimRight(filepath.Clean(caseAbs), string(filepath.Separator))
	cwdClean := strings.TrimRight(filepath.Clean(cwdAbs), string(filepath.Separator))
	prefix := caseClean + string(filepath.Separator)
	if !strings.EqualFold(cwdClean, caseClean) && !strings.HasPrefix(strings.ToLower(cwdClean), strings.ToLower(prefix)) {
		return "", "", false, nil
	}
	full, rel, err := caseContainedPath(caseClean, filepath.Join(cwdClean, filepath.FromSlash(value)))
	if err != nil {
		return "", "", false, err
	}
	return full, rel, true, nil
}

func cwdAuthorizedExecutionReportPath(caseRoot string, gateEvent EventPreview, cwd, value string) (string, string, bool, error) {
	if strings.TrimSpace(cwd) == "" || filepath.IsAbs(value) {
		return "", "", false, nil
	}
	full, rel, ok, err := cwdRelativeCasePath(caseRoot, cwd, value)
	if err != nil || !ok {
		return "", "", ok, err
	}
	if !outputRefsWithinGate(gateEvent.Gate.OutputPaths, []string{rel}) {
		return "", "", false, nil
	}
	return full, rel, true, nil
}

func caseContainedPath(caseRoot, value string) (string, string, error) {
	caseAbs, err := filepath.Abs(caseRoot)
	if err != nil {
		return "", "", err
	}
	fullAbs, err := filepath.Abs(value)
	if err != nil {
		return "", "", err
	}
	caseClean := strings.TrimRight(filepath.Clean(caseAbs), string(filepath.Separator))
	fullClean := strings.TrimRight(filepath.Clean(fullAbs), string(filepath.Separator))
	prefix := caseClean + string(filepath.Separator)
	if !strings.EqualFold(fullClean, caseClean) && !strings.HasPrefix(strings.ToLower(fullClean), strings.ToLower(prefix)) {
		return "", "", fmt.Errorf("gate execution report path must stay within case root: %s", value)
	}
	rel, err := filepath.Rel(caseClean, fullClean)
	if err != nil {
		return "", "", err
	}
	if rel == "." {
		return "", "", fmt.Errorf("gate execution report path must name a file under case root")
	}
	return fullClean, filepath.ToSlash(rel), nil
}

func validateAdapterExecutionReport(caseRoot string, gateEvent EventPreview, report *AdapterReport) error {
	if report.SchemaVersion != 1 {
		return adapterReportValidationErrorf("schema-version", "schema", "adapter execution report schemaVersion has unsupported value: %d", report.SchemaVersion)
	}
	if strings.TrimSpace(report.Kind) != "adapter-execution-report" {
		return adapterReportValidationErrorf("kind", "schema", "adapter execution report kind has unsupported value: %s", report.Kind)
	}
	report.AdapterID = strings.TrimSpace(report.AdapterID)
	if report.AdapterID == "" {
		return adapterReportValidationErrorf("adapter-id-missing", "schema", "adapter execution report is missing adapterId")
	}
	report.Action = strings.ToLower(strings.TrimSpace(report.Action))
	if report.Action != gateEvent.Gate.Action {
		return adapterReportValidationErrorf("action-mismatch", "identity", "adapter execution report action %q does not match authorized gate action %q", report.Action, gateEvent.Gate.Action)
	}
	report.Status = strings.ToLower(strings.TrimSpace(report.Status))
	if !validExecutionStatus(report.Status) {
		return adapterReportValidationErrorf("status", "schema", "adapter execution report status has unsupported value: %s", report.Status)
	}
	report.GateEventID = strings.TrimSpace(report.GateEventID)
	if report.GateEventID != gateEvent.EventID {
		return adapterReportValidationErrorf("gate-event-mismatch", "identity", "adapter execution report gateEventId %q does not match authorized gate eventId %q", report.GateEventID, gateEvent.EventID)
	}
	if report.Dispatch != nil {
		report.Dispatch.DispatchID = strings.ToLower(strings.TrimSpace(report.Dispatch.DispatchID))
		report.Dispatch.Path = strings.TrimSpace(filepath.ToSlash(report.Dispatch.Path))
		report.Dispatch.SHA256 = strings.ToLower(strings.TrimSpace(report.Dispatch.SHA256))
		if !validSHA256String(report.Dispatch.DispatchID) || report.Dispatch.Path == "" || !validSHA256String(report.Dispatch.SHA256) {
			return adapterReportValidationErrorf("dispatch-binding-invalid", "identity", "adapter execution report dispatch binding is invalid")
		}
	}
	if report.ActualBudget.RuntimeSeconds < 0 || report.ActualBudget.DiskMB < 0 || report.ActualBudget.Requests < 0 {
		return adapterReportValidationErrorf("actual-budget-negative", "budget", "adapter execution report actualBudget values must be non-negative")
	}
	outputRefs, err := validateCaseRelativePaths(caseRoot, "adapter execution report outputRefs", report.OutputRefs)
	if err != nil {
		return adapterReportValidationErrorf("output-refs-invalid", "refs", "%w", err)
	}
	if len(outputRefs) > 0 && !outputRefsWithinGate(gateEvent.Gate.OutputPaths, outputRefs) {
		return adapterReportValidationErrorf("output-refs-out-of-scope", "refs", "adapter execution report outputRefs must stay within authorized gate outputPaths")
	}
	report.OutputRefs = outputRefs
	evidenceRefs, err := validateCaseRelativePaths(caseRoot, "adapter execution report evidenceRefs", report.EvidenceRefs)
	if err != nil {
		return adapterReportValidationErrorf("evidence-refs-invalid", "refs", "%w", err)
	}
	if len(evidenceRefs) > 0 && !outputRefsWithinGate(gateEvent.Gate.OutputPaths, evidenceRefs) {
		return adapterReportValidationErrorf("evidence-refs-out-of-scope", "refs", "adapter execution report evidenceRefs must stay within authorized gate outputPaths")
	}
	report.EvidenceRefs = evidenceRefs
	if len(report.BoundaryHits) > 0 {
		if err := validateStopConditions("adapter execution report boundaryHits", report.BoundaryHits); err != nil {
			return adapterReportValidationErrorf("boundary-hits-invalid", "boundary", "%w", err)
		}
		if !boundaryHitsWithinGate(gateEvent.Gate.StopConditions, report.BoundaryHits) {
			return adapterReportValidationErrorf("boundary-hits-not-authorized", "boundary", "adapter execution report boundaryHits must be covered by authorized gate stopConditions")
		}
	}
	report.Escalation = strings.TrimSpace(report.Escalation)
	if len(report.Escalation) > 4096 {
		return adapterReportValidationErrorf("escalation-too-large", "boundary", "adapter execution report escalation is too large")
	}
	if (report.Status == "boundary-hit" || report.Status == "escalated") && len(report.BoundaryHits) == 0 && report.Escalation == "" {
		return adapterReportValidationErrorf("boundary-marker-missing", "boundary", "adapter execution report status %s requires boundaryHits or escalation", report.Status)
	}
	if exceedsGateBudget(gateEvent.Gate.RequestedBudget, report.ActualBudget) && len(report.BoundaryHits) == 0 && report.Escalation == "" {
		return adapterReportValidationErrorf("budget-marker-missing", "budget", "adapter execution report actualBudget exceeds authorized request; record boundaryHits or escalation")
	}
	report.Summary = strings.TrimSpace(report.Summary)
	if requiresAdapterReportSummary(report.Status) && report.Summary == "" {
		return adapterReportValidationErrorf("status-summary-missing", "summary", "adapter execution report status %s requires a bounded summary", report.Status)
	}
	if len(report.Summary) > 4096 {
		return adapterReportValidationErrorf("summary-too-large", "summary", "adapter execution report summary is too large")
	}
	return nil
}

func validateCaseRelativePaths(caseRoot, field string, refs []string) ([]string, error) {
	out := []string{}
	for _, ref := range refs {
		clean, err := validateCaseRelativePath(caseRoot, field, ref)
		if err != nil {
			return nil, err
		}
		out = append(out, clean)
	}
	return out, nil
}

func validateCaseRelativePath(caseRoot, field, value string) (string, error) {
	rel := strings.TrimSpace(value)
	if rel == "" {
		return "", fmt.Errorf("%s contains an empty item", field)
	}
	if filepath.IsAbs(rel) {
		return "", fmt.Errorf("%s must be case-relative: %s", field, rel)
	}
	clean := filepath.ToSlash(filepath.Clean(filepath.FromSlash(rel)))
	if clean == "." || clean == ".." || strings.HasPrefix(clean, "../") {
		return "", fmt.Errorf("%s escapes case root: %s", field, rel)
	}
	if _, err := refsf.SafeJoin(caseRoot, clean); err != nil {
		return "", fmt.Errorf("%s escapes case root: %s", field, rel)
	}
	return clean, nil
}

func validExecutionStatus(status string) bool {
	switch status {
	case "succeeded", "failed", "boundary-hit", "escalated", "aborted":
		return true
	default:
		return false
	}
}

func requiresAdapterReportSummary(status string) bool {
	switch status {
	case "failed", "boundary-hit", "escalated", "aborted":
		return true
	default:
		return false
	}
}

func parseBoundaryHits(value string) ([]string, error) {
	items := splitList(value)
	if len(items) == 0 {
		return nil, nil
	}
	if err := validateStopConditions("boundaryHits", items); err != nil {
		return nil, err
	}
	return items, nil
}

func exceedsGateBudget(allowed, actual autonomy.Budget) bool {
	return (allowed.RuntimeSeconds > 0 && actual.RuntimeSeconds > allowed.RuntimeSeconds) || (allowed.DiskMB > 0 && actual.DiskMB > allowed.DiskMB) || (allowed.Requests > 0 && actual.Requests > allowed.Requests)
}

func boundaryHitsWithinGate(allowed, hits []string) bool {
	if len(hits) == 0 {
		return true
	}
	allowedSet := map[string]bool{}
	for _, item := range allowed {
		item = strings.ToLower(strings.TrimSpace(item))
		if item != "" {
			allowedSet[item] = true
		}
	}
	if len(allowedSet) == 0 {
		return false
	}
	for _, hit := range hits {
		if !allowedSet[strings.ToLower(strings.TrimSpace(hit))] {
			return false
		}
	}
	return true
}

func outputRefsWithinGate(allowed, refs []string) bool {
	if len(allowed) == 0 {
		return false
	}
	allowed = normalizedGatePaths(allowed)
	for _, ref := range normalizedGatePaths(refs) {
		matched := false
		for _, prefix := range allowed {
			if ref == prefix || strings.HasPrefix(ref, strings.TrimRight(prefix, "/")+"/") {
				matched = true
				break
			}
		}
		if !matched {
			return false
		}
	}
	return true
}

func normalizedGatePaths(paths []string) []string {
	out := []string{}
	for _, item := range paths {
		clean := filepath.ToSlash(filepath.Clean(filepath.FromSlash(strings.TrimSpace(item))))
		if clean != "." && clean != "" {
			out = append(out, clean)
		}
	}
	return out
}

func executionEventID(event ExecutionEvidencePreview) string {
	seed := strings.Join([]string{
		event.Kind,
		event.Lane,
		event.Subject,
		event.Summary,
		event.Actor,
		event.Status,
		event.Target,
		event.BatchID,
		strings.Join(event.Related, ","),
		strings.Join(event.EvidenceRefs, ","),
		event.Gate.Action,
		event.Gate.Authorization.Decision,
		event.Execution.GateEventID,
		event.Execution.Status,
		fmt.Sprintf("%d/%d/%d", event.Execution.ActualBudget.RuntimeSeconds, event.Execution.ActualBudget.DiskMB, event.Execution.ActualBudget.Requests),
		strings.Join(event.Execution.OutputRefs, ","),
		strings.Join(event.Execution.BoundaryHits, ","),
		event.Execution.Escalation,
		event.Execution.ExecutionReportPath,
		event.Execution.AdapterExecutionReceiptPath,
		event.Execution.AdapterExecutionReceiptSHA256,
		adapterEventIDSeed(event.Execution.Adapter),
	}, "|")
	sum := sha256.Sum256([]byte(seed))
	return "evt-" + hex.EncodeToString(sum[:])[:16]
}

func adapterEventIDSeed(report *AdapterReport) string {
	if report == nil {
		return ""
	}
	return strings.Join([]string{
		report.Kind,
		report.AdapterID,
		report.Action,
		report.Status,
		report.GateEventID,
		fmt.Sprintf("%d/%d/%d", report.ActualBudget.RuntimeSeconds, report.ActualBudget.DiskMB, report.ActualBudget.Requests),
		strings.Join(report.OutputRefs, ","),
		strings.Join(report.EvidenceRefs, ","),
		strings.Join(report.BoundaryHits, ","),
		report.Escalation,
		report.Summary,
	}, "|")
}

func executionNextSteps(event ExecutionEvidencePreview) []string {
	label := mission.BoardLaneLabel(mission.BoardLane{ID: event.Lane})
	if executionNeedsMainReview(event) {
		return []string{
			"Execution evidence recorded a boundary hit or escalation; stop autonomous work on this action and notify the main Agent.",
			"Mission Commander handoff: /rekit handoff " + label,
			"Review output refs and evidence refs before recording any authority/confirmed outcome.",
		}
	}
	return []string{
		"Execution evidence recorded the authorized action outcome; /rekit did not execute the heavy tool.",
		"Mission Commander handoff: /rekit handoff " + label,
		"Review output refs and evidence refs before recording any authority/confirmed outcome.",
	}
}

func executionCommanderAction(event ExecutionEvidencePreview, applied, duplicate bool) mission.MissionCommanderAction {
	label := mission.BoardLaneLabel(mission.BoardLane{ID: event.Lane})
	if strings.TrimSpace(label) == "" {
		label = "main"
	}
	state := "ready-for-evidence-review"
	prompt := fmt.Sprintf("authorized gate `%s` 的 observation evidence 已记录；先 review output/evidence refs，再考虑任何 authority/confirmed outcome。", event.Execution.GateEventID)
	if duplicate {
		state = "evidence-already-recorded"
		prompt = fmt.Sprintf("authorized gate `%s` 的 observation evidence 已存在（duplicate eventId）；不要重复记录，直接 review output/evidence refs。", event.Execution.GateEventID)
	}
	if executionNeedsMainReview(event) {
		state = "needs-main-escalation"
		prompt = fmt.Sprintf("authorized gate `%s` 的 observation evidence 记录了 boundary/escalation；停止该 action 的自主推进并通知 main Agent。", event.Execution.GateEventID)
		if duplicate {
			prompt = fmt.Sprintf("authorized gate `%s` 的 boundary/escalation observation evidence 已存在（duplicate eventId）；不要重复记录，继续 main review。", event.Execution.GateEventID)
		}
	}
	boundary := []string{
		"record command writes bounded observation evidence only",
		"/rekit did not execute the heavy tool",
		"review outputRefs/evidenceRefs before any authority/confirmed outcome",
		"no authority/confirmed writes",
	}
	if duplicate {
		boundary[0] = "duplicate record did not append observation evidence"
	}
	if executionNeedsMainReview(event) {
		boundary = append(boundary, "stop autonomous work on this action until main review")
	}
	followUp := []string{"/rekit overview"}
	if applied && !executionNeedsMainReview(event) {
		followUp = append(followUp, "/rekit continue "+label+" -WhatIf")
	}
	return mission.MissionCommanderAction{
		State:            state,
		Prompt:           prompt,
		PrimaryCommand:   "/rekit handoff " + label,
		FollowUpCommands: followUp,
		Boundary:         boundary,
	}
}

func executionNeedsMainReview(event ExecutionEvidencePreview) bool {
	return event.Status == "boundary-hit" || event.Status == "escalated" || event.Execution.Escalation != "" || len(event.Execution.BoundaryHits) > 0
}

func gateExecutionEvidenceReviewFromObservation(event ExecutionEvidencePreview) []mission.ExecutionEvidenceReviewItem {
	return gateExecutionEvidenceReviewWithAction(event, mission.MissionCommanderAction{})
}

func gateDuplicateExecutionEvidenceReviewFromObservation(event ExecutionEvidencePreview, commander mission.MissionCommanderAction) []mission.ExecutionEvidenceReviewItem {
	return gateExecutionEvidenceReviewWithAction(event, commander)
}

func gateExecutionEvidenceReviewWithAction(event ExecutionEvidencePreview, commander mission.MissionCommanderAction) []mission.ExecutionEvidenceReviewItem {
	data, err := json.Marshal(event)
	if err != nil {
		return nil
	}
	var observation map[string]any
	if err := json.Unmarshal(data, &observation); err != nil {
		return nil
	}
	item, ok := mission.ExecutionEvidenceReviewItemFromObservation(observation, event.Lane, func(laneID string) string {
		return mission.BoardLaneLabel(mission.BoardLane{ID: laneID})
	})
	if !ok {
		return nil
	}
	if commander.PrimaryCommand != "" {
		item.Boundary = append([]string{}, commander.Boundary...)
		item.MissionCommanderAction = commander
		item.FollowThrough = mission.ExecutionEvidenceReviewFollowThrough(item)
		item.ReviewRunbookSteps = mission.ExecutionEvidenceReviewRunbookSteps(item, true)
	}
	return []mission.ExecutionEvidenceReviewItem{item}
}

func gateMissionCommanderNextActions(laneID string, action mission.ExecutorAction, evidenceReview []mission.ExecutionEvidenceReviewItem) []mission.MissionCommanderNextActionItem {
	label := mission.BoardLaneLabel(mission.BoardLane{ID: laneID})
	if strings.TrimSpace(label) == "" {
		label = strings.TrimSpace(laneID)
	}
	if label == "" {
		label = "main"
	}
	return mission.MissionCommanderNextActions([]mission.LaneExecutorActionSnapshot{{Lane: laneID, Label: label, ExecutorAction: action}}, evidenceReview, action.Blocked)
}

func gatePendingApplyCommanderAction(pack string, preview EventPreview) mission.MissionCommanderAction {
	label := gateCommanderActionLabel(preview.Lane)
	return mission.MissionCommanderAction{
		State:          "needs-gate-apply",
		Prompt:         fmt.Sprintf("先 review `%s` 的 pending-gate preview，再写入 request ledger；这不是 heavy-tool approval。", label),
		PrimaryCommand: gateRequestApplySlashCommand(pack, preview),
		FollowUpCommands: []string{
			"/rekit handoff " + label,
			"/rekit continue " + label + " -WhatIf",
		},
		Boundary: []string{
			"gate apply only writes a request ledger decision",
			"pending-gate still requires explicit authorization before heavy action",
			"/rekit does not execute the heavy tool",
			"no authority/confirmed writes",
		},
	}
}

func gateAuthorizedApplyCommanderAction(pack string, preview EventPreview) mission.MissionCommanderAction {
	label := gateCommanderActionLabel(preview.Lane)
	gateEventID := gateRequestEventID(preview)
	if strings.TrimSpace(preview.Actor) == "" {
		gateEventID = "<gateEventId-after-apply>"
	}
	return mission.MissionCommanderAction{
		State:          "needs-authorized-gate-apply",
		Prompt:         fmt.Sprintf("先 review `%s` 的 authorized-gate preview，再写入 durable lane authorization decision；actual heavy tool 仍在 /rekit 外执行。", label),
		PrimaryCommand: gateRequestApplySlashCommand(pack, preview),
		FollowUpCommands: []string{
			gateExecutionReportContractSlashCommand(pack, gateEventID),
			"/rekit handoff " + label,
		},
		Boundary: gateAuthorizedRequestBoundary(),
	}
}

func gateAuthorizedRecordedCommanderAction(pack string, preview EventPreview, duplicate bool) mission.MissionCommanderAction {
	label := gateCommanderActionLabel(preview.Lane)
	state := "ready-for-execution-report-contract"
	prompt := fmt.Sprintf("authorized gate `%s` 已记录；先读取 execution report contract，再让 lane executor/tool adapter 在授权边界内执行并记录 bounded observation evidence。", preview.EventID)
	boundary := gateAuthorizedRequestBoundary()
	if duplicate {
		state = "authorized-gate-already-recorded"
		prompt = fmt.Sprintf("authorized gate `%s` 已存在（duplicate eventId）；不要重复写 request ledger，直接读取 execution report contract 并继续 evidence handoff。", preview.EventID)
		boundary = append(boundary, "duplicate request did not append ledger row")
	}
	return mission.MissionCommanderAction{
		State:          state,
		Prompt:         prompt,
		PrimaryCommand: gateExecutionReportContractSlashCommand(pack, preview.EventID),
		FollowUpCommands: []string{
			"/rekit handoff " + label,
		},
		Boundary: boundary,
	}
}

func gateRequestMissionCommanderNextActions(laneID string, action mission.ExecutorAction, preauthorized bool, applied bool) []mission.MissionCommanderNextActionItem {
	items := gateMissionCommanderNextActions(laneID, action, nil)
	if applied && !preauthorized {
		return items
	}
	for idx := range items {
		items[idx].RequiresReview = true
		if !applied {
			items[idx].Reasons = append(items[idx].Reasons, "review gate preview before writing request ledger state")
			if items[idx].Source == "missionCommanderActions" {
				items[idx].Blocked = false
				items[idx].Reasons = append(items[idx].Reasons, "gate apply is the bounded action that records the selected request decision")
			}
			if items[idx].Source == "missionCommanderActions.followUp" {
				items[idx].Blocked = true
				items[idx].Reasons = append(items[idx].Reasons, "run only after gate apply succeeds and refreshed request/executor state is available")
			}
			continue
		}
		items[idx].Reasons = append(items[idx].Reasons, "authorized-gate request is recorded; read execution report contract before external heavy-tool execution")
	}
	return mission.UniqueCommanderNextActions(items)
}

func gateRequestApplySlashCommand(pack string, preview EventPreview) string {
	return gateRequestSlashCommand(pack, preview, true)
}

func gateRequestWhatIfSlashCommand(pack string, preview EventPreview) string {
	return gateRequestSlashCommand(pack, preview, false)
}

func gateRequestSlashCommand(pack string, preview EventPreview, apply bool) string {
	actor := strings.TrimSpace(preview.Actor)
	if actor == "" {
		actor = "<actor>"
	}
	mode := "-WhatIf"
	if apply {
		mode = "-Apply"
	}
	args := []string{"gate", "-Pack", pack, "-Action", preview.Gate.Action, "-Lane", preview.Lane, mode, "-Actor", actor}
	args = appendGateCommandArg(args, "-Subject", preview.Subject)
	args = appendGateCommandArg(args, "-Summary", preview.Summary)
	args = appendGateCommandArg(args, "-TargetRef", preview.Target)
	args = appendGateCommandArg(args, "-BatchId", preview.BatchID)
	args = appendGateCommandArg(args, "-Scope", preview.Gate.Scope)
	args = appendGateCommandArg(args, "-Budget", preview.Gate.Budget)
	args = appendGateCommandIntArg(args, "-RuntimeSeconds", preview.Gate.RequestedBudget.RuntimeSeconds)
	args = appendGateCommandIntArg(args, "-DiskMB", preview.Gate.RequestedBudget.DiskMB)
	args = appendGateCommandIntArg(args, "-Requests", preview.Gate.RequestedBudget.Requests)
	args = appendGateCommandArg(args, "-OutputPaths", strings.Join(preview.Gate.OutputPaths, ","))
	args = appendGateCommandArg(args, "-TriedLightSteps", strings.Join(preview.Gate.TriedLightSteps, ","))
	args = appendGateCommandArg(args, "-StopConditions", strings.Join(preview.Gate.StopConditions, ","))
	args = appendGateCommandArg(args, "-Risk", preview.Risk)
	return adapterReportSlashCommand(args)
}

func appendGateCommandArg(args []string, flag, value string) []string {
	value = strings.TrimSpace(value)
	if value == "" {
		return args
	}
	return append(args, flag, value)
}

func appendGateCommandIntArg(args []string, flag string, value int) []string {
	if value == 0 {
		return args
	}
	return append(args, flag, fmt.Sprintf("%d", value))
}

func gateExecutionReportContractSlashCommand(pack, gateEventID string) string {
	return adapterReportSlashCommand([]string{"gate", "-Pack", pack, "-GateEventId", gateEventID, "-ExecutionReportContract", "-Format", "json"})
}

func gateRequestEventID(preview EventPreview) string {
	if strings.TrimSpace(preview.EventID) != "" {
		return strings.TrimSpace(preview.EventID)
	}
	return eventID(preview)
}

func gateAuthorizedRequestBoundary() []string {
	return []string{
		"gate apply writes durable authorization decision only",
		"actual heavy tool must stay within authorized target, budget, output paths, and stop conditions",
		"record bounded observation evidence after execution",
		"/rekit does not execute the heavy tool",
		"no authority/confirmed writes",
	}
}

func gateEventMap(event EventPreview) map[string]any {
	data, err := json.Marshal(event)
	if err != nil {
		return map[string]any{}
	}
	var out map[string]any
	if err := json.Unmarshal(data, &out); err != nil {
		return map[string]any{}
	}
	return out
}

func gateMissionBrief(caseRoot string) mission.Brief {
	brief, err := mission.CaseBrief(caseRoot, mission.BuildOptions{
		MaxRows:            mission.DefaultMaxRows,
		OpenDecisionAction: "review open candidate/decision item(s) with evidence and authority boundary",
	})
	if err != nil {
		return mission.Brief{Summary: "unavailable: " + err.Error()}
	}
	return brief
}

func gateExecutorAction(caseRoot, laneID string, brief mission.Brief) mission.ExecutorAction {
	board, facts, err := gateBoardFacts(caseRoot)
	if err != nil {
		return mission.LaneExecutorAction(mission.Lane{ID: laneID, Label: mission.BoardLaneLabel(mission.BoardLane{ID: laneID})}, mission.Facts{}, brief)
	}
	lane := gateMissionLane(board, laneID)
	return mission.LaneExecutorAction(lane, facts, brief)
}

func gateWouldExecutorAction(caseRoot string, preview EventPreview, brief mission.Brief) mission.ExecutorAction {
	board, facts, err := gateBoardFacts(caseRoot)
	if err != nil {
		facts = mission.Facts{}
	}
	facts.Requests = append(append([]map[string]any{}, facts.Requests...), gatePreviewMap(preview))
	wouldBrief := brief
	if len(board.Lanes) > 0 {
		wouldBrief = mission.BuildWithOptions(mission.BoardLanes(board.Lanes), facts, mission.BuildOptions{
			MaxRows:            mission.DefaultMaxRows,
			OpenDecisionAction: "review open candidate/decision item(s) with evidence and authority boundary",
		})
	}
	return mission.LaneExecutorAction(gateMissionLane(board, preview.Lane), facts, wouldBrief)
}

func gateBoardFacts(caseRoot string) (mission.Board, mission.Facts, error) {
	board, err := mission.ReadBoard(caseRoot)
	if err != nil {
		return mission.Board{}, mission.Facts{}, err
	}
	facts, err := mission.ReadFacts(caseRoot)
	if err != nil {
		return board, mission.Facts{}, err
	}
	return board, facts, nil
}

func gateMissionLane(board mission.Board, laneID string) mission.Lane {
	for _, item := range board.Lanes {
		if strings.EqualFold(strings.TrimSpace(item.ID), strings.TrimSpace(laneID)) {
			return mission.Lane{ID: strings.TrimSpace(item.ID), Label: mission.BoardLaneLabel(item), Status: item.Status}
		}
	}
	return mission.Lane{ID: laneID, Label: mission.BoardLaneLabel(mission.BoardLane{ID: laneID})}
}

func gatePreviewMap(preview EventPreview) map[string]any {
	authorization := map[string]any{
		"decision":  preview.Gate.Authorization.Decision,
		"profileId": preview.Gate.Authorization.ProfileID,
	}
	gate := map[string]any{
		"action":          preview.Gate.Action,
		"scope":           preview.Gate.Scope,
		"budget":          preview.Gate.Budget,
		"authorization":   authorization,
		"stopConditions":  preview.Gate.StopConditions,
		"outputPaths":     preview.Gate.OutputPaths,
		"requestedBudget": preview.Gate.RequestedBudget,
	}
	return map[string]any{
		"kind":      preview.Kind,
		"lane":      preview.Lane,
		"subject":   preview.Subject,
		"summary":   preview.Summary,
		"status":    preview.Status,
		"actor":     preview.Actor,
		"risk":      preview.Risk,
		"target":    preview.Target,
		"batchId":   preview.BatchID,
		"gate":      gate,
		"eventId":   preview.EventID,
		"createdAt": preview.CreatedAt,
	}
}

func planNextSteps(preview EventPreview) []string {
	if preview.Gate.Authorization.Decision == autonomy.DecisionPreauthorized {
		return []string{
			"Record this authorized gate decision in the ledger if the preflight matches the intended action, target, budget, output paths, and stop conditions.",
			"The actual heavy tool still runs outside /rekit; keep execution within the durable lane autonomy profile.",
			"After the tool run, record execution evidence with gate -Apply -GateEventId and include output refs, evidence refs, actual budget use, and any boundary hit or escalation.",
		}
	}
	return []string{
		"Record the pending-gate request in the ledger only if this preview is accepted.",
		"Ask the user to confirm the exact action, target, scope, budget, output paths, and stop conditions.",
		"Run the heavy tool only after explicit confirmation or a valid durable lane autonomy profile; this dry-run is not approval.",
		"After the tool run, append an observation event summarizing output path, findings, errors, and next action.",
	}
}

func applyNextSteps(preview EventPreview) []string {
	if preview.Gate.Authorization.Decision == autonomy.DecisionPreauthorized {
		return []string{
			"This ledger write records a durable lane autonomy authorization decision; /rekit still did not execute the heavy tool.",
			"Run the heavy tool only within the authorized target, budget, output paths, and stop conditions.",
			"After the tool run, record execution evidence with gate -Apply -GateEventId and include output refs, evidence refs, actual budget use, and any boundary hit or escalation.",
		}
	}
	return []string{
		"Ask the user to confirm the exact action, target, scope, budget, output paths, and stop conditions before running the heavy tool.",
		"This ledger write records a pending gate only; it is not heavy-tool approval.",
		"After the tool run, append an observation event summarizing output path, findings, errors, and next action.",
	}
}

func buildPreview(repoRoot, caseRoot, pack string, opt Options) (instance.Instance, EventPreview, []string, error) {
	inst, err := instance.AssertAttached(caseRoot, repoRoot, pack)
	if err != nil {
		return instance.Instance{}, EventPreview{}, nil, err
	}
	m, err := manifest.Load(repoRoot, pack)
	if err != nil {
		return instance.Instance{}, EventPreview{}, nil, err
	}
	action := strings.ToLower(strings.TrimSpace(opt.Action))
	allowed := m.HeavyToolGateIDs()
	if len(allowed) == 0 {
		return instance.Instance{}, EventPreview{}, nil, fmt.Errorf("pack manifest must declare heavyToolGates before gate can accept heavy-tool actions")
	}
	if action == "" {
		return instance.Instance{}, EventPreview{}, nil, fmt.Errorf("gate requires -Action %s", strings.Join(allowed, "|"))
	}
	gate, ok := m.HeavyToolGate(action)
	if !ok {
		return instance.Instance{}, EventPreview{}, nil, fmt.Errorf("invalid gate action %q; allowed: %s", action, strings.Join(allowed, ","))
	}
	if !gate.RequiresConfirmation {
		return instance.Instance{}, EventPreview{}, nil, fmt.Errorf("gate action %q must require user confirmation in pack manifest", action)
	}
	if strings.TrimSpace(gate.DefaultRisk) == "" {
		return instance.Instance{}, EventPreview{}, nil, fmt.Errorf("gate action %q is missing defaultRisk in pack manifest", action)
	}
	if err := validateGateRisk("manifest defaultRisk", gate.DefaultRisk); err != nil {
		return instance.Instance{}, EventPreview{}, nil, fmt.Errorf("gate action %q has invalid %w", action, err)
	}
	if len(gate.StopConditions) == 0 {
		return instance.Instance{}, EventPreview{}, nil, fmt.Errorf("gate action %q is missing stopConditions in pack manifest", action)
	}
	lane := strings.TrimSpace(opt.Lane)
	if lane == "" {
		return instance.Instance{}, EventPreview{}, nil, fmt.Errorf("gate requires -Lane <lane id>")
	}
	lane, err = canonicalLane(inst.CaseRoot, lane)
	if err != nil {
		return instance.Instance{}, EventPreview{}, nil, err
	}
	subject := strings.TrimSpace(opt.Subject)
	if subject == "" {
		subject = action + " gate"
	}
	summary := strings.TrimSpace(opt.Summary)
	risk, err := parseGateRisk(opt.Risk)
	if err != nil {
		return instance.Instance{}, EventPreview{}, nil, err
	}
	if risk == "" {
		risk = strings.TrimSpace(gate.DefaultRisk)
	}
	stopConditions, err := parseStopConditions(opt.StopConditions)
	if err != nil {
		return instance.Instance{}, EventPreview{}, nil, err
	}
	if len(stopConditions) == 0 {
		stopConditions = append([]string{}, gate.StopConditions...)
		if err := validateStopConditions("manifest stopConditions", stopConditions); err != nil {
			return instance.Instance{}, EventPreview{}, nil, fmt.Errorf("gate action %q has invalid %w", action, err)
		}
	}
	outputPaths, err := parseOutputPaths(inst.CaseRoot, opt.OutputPaths)
	if err != nil {
		return instance.Instance{}, EventPreview{}, nil, err
	}
	requestedBudget := autonomy.Budget{RuntimeSeconds: opt.RuntimeSeconds, DiskMB: opt.DiskMB, Requests: opt.Requests}
	target := strings.TrimSpace(opt.TargetRef)
	authorization := authorizationDecision(inst.CaseRoot, lane, m, autonomy.Request{
		Lane:           lane,
		Action:         action,
		Target:         target,
		Budget:         requestedBudget,
		StopConditions: stopConditions,
		OutputPaths:    outputPaths,
	})
	status := "pending-gate"
	if authorization.Decision == autonomy.DecisionPreauthorized {
		status = "authorized-gate"
	}
	if summary == "" {
		if authorization.Decision == autonomy.DecisionPreauthorized {
			summary = "Preauthorized by durable lane autonomy profile before running " + action
		} else {
			summary = "Request user confirmation before running " + action
		}
	}
	blocked := []string{}
	if authorization.RequiresConfirmation {
		blocked = []string{action}
	}
	preview := EventPreview{
		SchemaVersion: 1,
		Kind:          "request",
		Lane:          lane,
		Subject:       subject,
		Summary:       summary,
		Status:        status,
		Actor:         strings.TrimSpace(opt.Actor),
		Risk:          risk,
		Target:        target,
		BatchID:       strings.TrimSpace(opt.BatchID),
		Gate: GateDetails{
			Action:                      action,
			Scope:                       strings.TrimSpace(opt.Scope),
			Budget:                      strings.TrimSpace(opt.Budget),
			RequestedBudget:             requestedBudget,
			OutputPaths:                 outputPaths,
			TriedLightSteps:             splitList(opt.TriedLightSteps),
			StopConditions:              stopConditions,
			RequiresConfirmation:        authorization.RequiresConfirmation,
			DeniedUntilUserConfirmation: blocked,
			Authorization:               authorization,
		},
	}
	return inst, preview, blocked, nil
}

func canonicalLane(caseRoot, lane string) (string, error) {
	board, err := mission.ReadBoard(caseRoot)
	if err != nil {
		if os.IsNotExist(err) {
			return "", fmt.Errorf("gate requires .rekit/board.json to validate lane: %s", filepath.Join(caseRoot, ".rekit", "board.json"))
		}
		return "", err
	}
	known := mission.BoardLaneIDs(board.Lanes)
	for _, item := range board.Lanes {
		if strings.EqualFold(strings.TrimSpace(item.ID), strings.TrimSpace(lane)) {
			return strings.TrimSpace(item.ID), nil
		}
	}
	if len(known) == 0 {
		return "", fmt.Errorf("gate requires at least one lane in .rekit/board.json")
	}
	return "", fmt.Errorf("unknown lane %q; known: %s", lane, strings.Join(known, ","))
}

func splitList(value string) []string {
	parts := strings.FieldsFunc(value, func(r rune) bool { return r == ',' || r == ';' || r == '\n' })
	out := []string{}
	for _, part := range parts {
		part = strings.TrimSpace(part)
		if part != "" {
			out = append(out, part)
		}
	}
	return out
}

func sidecarAdapterID(candidates []AdapterToolCandidate) string {
	if len(candidates) == 0 {
		return "<adapter-id>"
	}
	return candidates[0].ID
}

func selectedAdapterToolCandidate(m *manifest.Manifest, event EventPreview, adapterID string) *AdapterToolCandidate {
	adapterID = strings.TrimSpace(adapterID)
	if adapterID == "" || adapterID == "<adapter-id>" {
		return nil
	}
	for _, candidate := range adapterToolCandidates(m, event) {
		if strings.EqualFold(candidate.ID, adapterID) {
			selected := candidate
			return &selected
		}
	}
	return nil
}

func adapterToolCandidates(m *manifest.Manifest, event EventPreview) []AdapterToolCandidate {
	candidates, err := strictAdapterToolCandidates(m, event)
	if err != nil {
		return nil
	}
	rows := map[string]AdapterToolCandidate{}
	for _, candidate := range candidates {
		key := strings.ToLower(candidate.ID)
		if existing, ok := rows[key]; ok && adapterToolCandidateRank(existing) <= adapterToolCandidateRank(candidate) {
			continue
		}
		rows[key] = candidate
	}
	out := make([]AdapterToolCandidate, 0, len(rows))
	for _, candidate := range rows {
		out = append(out, candidate)
	}
	sort.Slice(out, func(i, j int) bool {
		left := adapterToolCandidateRank(out[i])
		right := adapterToolCandidateRank(out[j])
		if left != right {
			return left < right
		}
		return strings.ToLower(out[i].ID) < strings.ToLower(out[j].ID)
	})
	return out
}

func adapterToolCandidateFromCatalogItem(item map[string]string, catalogPath, action string, gate manifest.HeavyToolGate) AdapterToolCandidate {
	id := strings.TrimSpace(item["id"])
	if id == "" {
		return AdapterToolCandidate{}
	}
	status := strings.TrimSpace(item["status"])
	entry := strings.TrimSpace(item["entry"])
	purpose := strings.TrimSpace(item["purpose"])
	sideEffectsText := strings.TrimSpace(item["sideEffects"])
	sideEffects := adapterCatalogSideEffects(sideEffectsText)
	matchesAction := strings.EqualFold(id, action) || sideEffectsContain(sideEffects, action) || sideEffectsOverlap(sideEffects, gate.SideEffects) || adapterCatalogTextLooksActionSpecific(action, id, entry, sideEffectsText)
	if !matchesAction && !strings.EqualFold(status, "auxiliary") && !adapterCatalogTextNegatesAction(action, purpose) {
		matchesAction = adapterCatalogTextLooksActionSpecific(action, purpose)
	}
	if !matchesAction {
		return AdapterToolCandidate{}
	}
	candidate := AdapterToolCandidate{
		ID:                  id,
		Status:              status,
		Entry:               entry,
		Purpose:             purpose,
		SideEffects:         sideEffects,
		GateActions:         []string{action},
		ToolingCatalogPath:  catalogPath,
		ReportGuidance:      adapterReportGuidance(action),
		EvidenceGuidance:    adapterEvidenceGuidance(),
		StopConditionHints:  append([]string{}, gate.StopConditions...),
		RecordOnlyAfterGate: true,
	}
	return candidate
}

func adapterToolCandidateRank(candidate AdapterToolCandidate) int {
	status := strings.ToLower(strings.TrimSpace(candidate.Status))
	switch status {
	case "mainline", "mainline-template", "supported":
		return 0
	case "cautious", "candidate":
		return 1
	case "auxiliary":
		return 2
	default:
		return 3
	}
}

func adapterCatalogSideEffects(value string) []string {
	items := splitList(value)
	if len(items) > 1 {
		return items
	}
	if strings.TrimSpace(value) == "" || strings.EqualFold(strings.TrimSpace(value), "none") {
		return nil
	}
	return []string{strings.TrimSpace(value)}
}

func adapterCatalogTextLooksActionSpecific(action string, fields ...string) bool {
	needle := strings.ToLower(strings.TrimSpace(action))
	if needle == "" {
		return false
	}
	for _, field := range fields {
		text := strings.ToLower(strings.ReplaceAll(field, "_", "-"))
		if strings.Contains(text, needle) {
			return true
		}
	}
	return false
}

func adapterCatalogTextNegatesAction(action, field string) bool {
	text := strings.ToLower(strings.ReplaceAll(field, "_", "-"))
	switch strings.ToLower(strings.TrimSpace(action)) {
	case "debug":
		return strings.Contains(text, "without executing") || strings.Contains(text, "without debugging") || strings.Contains(text, "without") && strings.Contains(text, "debugging")
	default:
		return strings.Contains(text, "without "+strings.ToLower(strings.TrimSpace(action)))
	}
}

func sideEffectsContain(sideEffects []string, action string) bool {
	action = strings.ToLower(strings.TrimSpace(action))
	for _, effect := range sideEffects {
		if strings.EqualFold(strings.TrimSpace(effect), action) {
			return true
		}
	}
	return false
}

func sideEffectsOverlap(candidate, gate []string) bool {
	seen := map[string]bool{}
	for _, effect := range gate {
		key := strings.ToLower(strings.TrimSpace(effect))
		if key != "" {
			seen[key] = true
		}
	}
	for _, effect := range candidate {
		if seen[strings.ToLower(strings.TrimSpace(effect))] {
			return true
		}
	}
	return false
}

func adapterReportGuidance(action string) []string {
	return []string{
		"select a pack tooling adapter whose purpose and sideEffects match authorized gate action " + action,
		"set sidecar adapterId to the selected adapter id so validation and record evidence preserve concrete adapter provenance",
		"write only bounded summary and case-relative refs in the sidecar; keep full tool output in authorized outputRefs/evidenceRefs",
	}
}

func adapterEvidenceGuidance() []string {
	return []string{
		"preflight with ValidateArgs or CaseRelativeValidateArgs before recording evidence",
		"record with RecordArgs or CaseRelativeRecordArgs only after the action stayed within authorized target, budget, output paths, and stop conditions",
		"do not write authority/confirmed or run any heavy tool through /rekit",
	}
}

func parseOutputPaths(caseRoot, value string) ([]string, error) {
	paths := splitList(value)
	out := []string{}
	for _, rel := range paths {
		clean, err := validateCaseRelativePath(caseRoot, "gate output path", rel)
		if err != nil {
			return nil, err
		}
		out = append(out, clean)
	}
	return out, nil
}

func authorizationDecision(caseRoot, lane string, m *manifest.Manifest, req autonomy.Request) autonomy.Decision {
	profile, path, exists, err := autonomy.Read(caseRoot, lane)
	rel := autonomy.RelPath(lane)
	if err != nil {
		return autonomy.Decision{Decision: autonomy.DecisionInvalidProfile, Mode: autonomy.ModeManualGate, ProfilePath: rel, Source: "lane-autonomy-profile", RequiresConfirmation: true, Reasons: []string{err.Error()}}
	}
	if exists {
		if err := autonomy.Validate(profile, lane, m, caseRoot); err != nil {
			return autonomy.Decision{Decision: autonomy.DecisionInvalidProfile, Mode: profile.Mode, ProfileID: profile.ProfileID, ProfilePath: rel, ProfileHash: autonomy.FileHash(path), Source: "lane-autonomy-profile", RequiresConfirmation: true, Reasons: []string{err.Error()}, RecordRequired: profile.RecordRequired}
		}
	}
	return autonomy.Evaluate(profile, rel, exists, autonomy.FileHash(path), req, time.Now().UTC())
}

func parseGateRisk(value string) (string, error) {
	risk := strings.TrimSpace(value)
	if risk == "" {
		return "", nil
	}
	if err := validateGateRisk("risk", risk); err != nil {
		return "", fmt.Errorf("gate %w", err)
	}
	return risk, nil
}

func validateGateRisk(field, value string) error {
	switch strings.TrimSpace(value) {
	case "medium", "high", "critical":
		return nil
	default:
		return fmt.Errorf("%s has unsupported value: %s", field, value)
	}
}

var stopConditionTokenPattern = regexp.MustCompile(`^[a-z][a-z0-9]*(?:[-_][a-z0-9]+)*$`)

func parseStopConditions(value string) ([]string, error) {
	if strings.TrimSpace(value) == "" {
		return nil, nil
	}
	normalized := strings.NewReplacer(";", ",", "\n", ",").Replace(value)
	conditions := []string{}
	for item := range strings.SplitSeq(normalized, ",") {
		conditions = append(conditions, strings.TrimSpace(item))
	}
	if err := validateStopConditions("stopConditions", conditions); err != nil {
		return nil, fmt.Errorf("gate %w", err)
	}
	return conditions, nil
}

func validateStopConditions(field string, conditions []string) error {
	seen := map[string]bool{}
	for _, condition := range conditions {
		if condition == "" {
			return fmt.Errorf("%s contains an empty item", field)
		}
		if !stopConditionTokenPattern.MatchString(condition) {
			return fmt.Errorf("%s has invalid item: %s", field, condition)
		}
		key := strings.ToLower(condition)
		if seen[key] {
			return fmt.Errorf("%s contains duplicate item: %s", field, condition)
		}
		seen[key] = true
	}
	return nil
}

func eventID(event EventPreview) string {
	seed := strings.Join([]string{
		event.Kind,
		event.Lane,
		event.Subject,
		event.Summary,
		event.Actor,
		event.Risk,
		event.Target,
		event.BatchID,
		event.Gate.Action,
		event.Gate.Scope,
		event.Gate.Budget,
		fmt.Sprintf("%d/%d/%d", event.Gate.RequestedBudget.RuntimeSeconds, event.Gate.RequestedBudget.DiskMB, event.Gate.RequestedBudget.Requests),
		strings.Join(event.Gate.OutputPaths, ","),
		event.Gate.Authorization.Decision,
		event.Gate.Authorization.ProfileHash,
		strings.Join(event.Gate.TriedLightSteps, ","),
		strings.Join(event.Gate.StopConditions, ","),
	}, "|")
	sum := sha256.Sum256([]byte(seed))
	return "evt-" + hex.EncodeToString(sum[:])[:16]
}
