package gate

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
	"time"

	"github.com/shuiyu486/re-context-kits/internal/rekit/autonomy"
	refsf "github.com/shuiyu486/re-context-kits/internal/rekit/fs"
	"github.com/shuiyu486/re-context-kits/internal/rekit/instance"
	"github.com/shuiyu486/re-context-kits/internal/rekit/manifest"
	"github.com/shuiyu486/re-context-kits/internal/rekit/mission"
)

type Options struct {
	Action                  string
	Lane                    string
	Subject                 string
	Summary                 string
	Actor                   string
	Risk                    string
	TargetRef               string
	BatchID                 string
	Scope                   string
	Budget                  string
	RuntimeSeconds          int
	DiskMB                  int
	Requests                int
	OutputPaths             string
	TriedLightSteps         string
	StopConditions          string
	GateEventID             string
	ExecutionStatus         string
	ActualRuntimeSeconds    int
	ActualDiskMB            int
	ActualRequests          int
	OutputRefs              string
	EvidenceRefs            string
	BoundaryHits            string
	Escalation              string
	ExecutionReportPath     string
	ExecutionReportCwd      string
	ExecutionReportContract bool
	ValidateExecutionReport bool
}

type Plan struct {
	SchemaVersion        int                    `json:"schemaVersion"`
	Command              string                 `json:"command"`
	CaseRoot             string                 `json:"caseRoot"`
	RepoRoot             string                 `json:"repoRoot"`
	Pack                 string                 `json:"pack"`
	IsMutation           bool                   `json:"isMutation"`
	ReviewRequired       bool                   `json:"reviewRequired"`
	RequiresConfirmation bool                   `json:"requiresConfirmation"`
	EventPreview         EventPreview           `json:"eventPreview"`
	MissionBrief         mission.Brief          `json:"missionBrief"`
	ExecutorAction       mission.ExecutorAction `json:"executorAction"`
	WouldExecutorAction  mission.ExecutorAction `json:"wouldExecutorAction"`
	BlockedActions       []string               `json:"blockedActions"`
	NextSteps            []string               `json:"nextSteps"`
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
	Status              string                `json:"status"`
	ActualBudget        autonomy.Budget       `json:"actualBudget"`
	OutputRefs          []string              `json:"outputRefs,omitempty"`
	BoundaryHits        []string              `json:"boundaryHits,omitempty"`
	Escalation          string                `json:"escalation,omitempty"`
	GateEventID         string                `json:"gateEventId"`
	GateStatus          string                `json:"gateStatus"`
	Authorization       string                `json:"authorization"`
	RecordRequired      bool                  `json:"recordRequired"`
	NotifyMainOn        []string              `json:"notifyMainOn,omitempty"`
	ExecutionReportPath string                `json:"executionReportPath,omitempty"`
	AdapterContext      *AdapterToolCandidate `json:"adapterContext,omitempty"`
	Adapter             *AdapterReport        `json:"adapter,omitempty"`
}

type AdapterReport struct {
	SchemaVersion int             `json:"schemaVersion"`
	Kind          string          `json:"kind"`
	AdapterID     string          `json:"adapterId"`
	Action        string          `json:"action"`
	Status        string          `json:"status"`
	GateEventID   string          `json:"gateEventId"`
	ActualBudget  autonomy.Budget `json:"actualBudget"`
	OutputRefs    []string        `json:"outputRefs,omitempty"`
	EvidenceRefs  []string        `json:"evidenceRefs,omitempty"`
	BoundaryHits  []string        `json:"boundaryHits,omitempty"`
	Escalation    string          `json:"escalation,omitempty"`
	Summary       string          `json:"summary,omitempty"`
}

type AdapterExecutionReportContract struct {
	SchemaVersion           int                                   `json:"schemaVersion"`
	Command                 string                                `json:"command"`
	Kind                    string                                `json:"kind"`
	CaseRoot                string                                `json:"caseRoot"`
	RepoRoot                string                                `json:"repoRoot"`
	Pack                    string                                `json:"pack"`
	IsMutation              bool                                  `json:"isMutation"`
	Lane                    string                                `json:"lane"`
	Target                  string                                `json:"target,omitempty"`
	BatchID                 string                                `json:"batchId,omitempty"`
	Risk                    string                                `json:"risk,omitempty"`
	Authorization           autonomy.Decision                     `json:"authorization"`
	ReportKind              string                                `json:"reportKind"`
	ReportSchemaVersion     int                                   `json:"reportSchemaVersion"`
	GateEventID             string                                `json:"gateEventId"`
	Action                  string                                `json:"action"`
	AllowedStatuses         []string                              `json:"allowedStatuses"`
	RequiredFields          []string                              `json:"requiredFields"`
	AllowedOutputPaths      []string                              `json:"allowedOutputPaths"`
	DefaultReportPath       string                                `json:"defaultReportPath,omitempty"`
	AuthorizedBudget        autonomy.Budget                       `json:"authorizedBudget"`
	StopConditions          []string                              `json:"stopConditions,omitempty"`
	ReportPathRule          string                                `json:"reportPathRule"`
	RefPathRequires         []string                              `json:"refPathRequires,omitempty"`
	SummaryMaxBytes         int                                   `json:"summaryMaxBytes"`
	RecordRequired          bool                                  `json:"recordRequired"`
	NotifyMainOn            []string                              `json:"notifyMainOn,omitempty"`
	BoundaryStatusRequires  []string                              `json:"boundaryStatusRequires,omitempty"`
	StatusSummaryRequires   []string                              `json:"statusSummaryRequires,omitempty"`
	EscalationMaxBytes      int                                   `json:"escalationMaxBytes"`
	ValidationFailureStages []AdapterReportValidationFailureStage `json:"validationFailureStages,omitempty"`
	ValidationFailureCodes  []AdapterReportValidationFailureCode  `json:"validationFailureCodes,omitempty"`
	ValidationRepairHints   []AdapterReportRepairHint             `json:"validationRepairHints,omitempty"`
	DeniedActions           []string                              `json:"deniedActions,omitempty"`
	LiveValidation          AdapterReportLiveValidation           `json:"liveValidation"`
	MissionCommanderAction  mission.MissionCommanderAction        `json:"missionCommanderAction"`
	NextSteps               []string                              `json:"nextSteps,omitempty"`
}

type AdapterReportLiveValidation struct {
	InvocationCwd               string                       `json:"invocationCwd"`
	AuthorizedWorkspaces        []string                     `json:"authorizedWorkspaces,omitempty"`
	ReportFileName              string                       `json:"reportFileName"`
	CaseRelativeReportPath      string                       `json:"caseRelativeReportPath,omitempty"`
	SidecarTemplate             AdapterReportSidecarTemplate `json:"sidecarTemplate"`
	ValidateCommand             string                       `json:"validateCommand"`
	RecordCommand               string                       `json:"recordCommand"`
	ValidateArgs                []string                     `json:"validateArgs"`
	RecordArgs                  []string                     `json:"recordArgs"`
	CaseRelativeValidateCommand string                       `json:"caseRelativeValidateCommand,omitempty"`
	CaseRelativeRecordCommand   string                       `json:"caseRelativeRecordCommand,omitempty"`
	CaseRelativeValidateArgs    []string                     `json:"caseRelativeValidateArgs,omitempty"`
	CaseRelativeRecordArgs      []string                     `json:"caseRelativeRecordArgs,omitempty"`
	AdapterCandidates           []AdapterToolCandidate       `json:"adapterCandidates,omitempty"`
	SelectedAdapter             *AdapterToolCandidate        `json:"selectedAdapter,omitempty"`
	ReplayBehavior              string                       `json:"replayBehavior"`
	Notes                       []string                     `json:"notes,omitempty"`
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
	SchemaVersion int             `json:"schemaVersion"`
	Kind          string          `json:"kind"`
	AdapterID     string          `json:"adapterId"`
	Action        string          `json:"action"`
	Status        string          `json:"status"`
	GateEventID   string          `json:"gateEventId"`
	ActualBudget  autonomy.Budget `json:"actualBudget"`
	OutputRefs    []string        `json:"outputRefs"`
	EvidenceRefs  []string        `json:"evidenceRefs"`
	BoundaryHits  []string        `json:"boundaryHits"`
	Escalation    string          `json:"escalation"`
	Summary       string          `json:"summary"`
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
	RecordBlocked         bool     `json:"recordBlocked"`
	RerunValidation       bool     `json:"rerunValidation"`
	EscalateToMain        bool     `json:"escalateToMain,omitempty"`
	Detail                string   `json:"detail"`
}

type AdapterExecutionReportValidation struct {
	SchemaVersion          int                            `json:"schemaVersion"`
	Command                string                         `json:"command"`
	Kind                   string                         `json:"kind"`
	CaseRoot               string                         `json:"caseRoot"`
	RepoRoot               string                         `json:"repoRoot"`
	Pack                   string                         `json:"pack"`
	IsMutation             bool                           `json:"isMutation"`
	Applied                bool                           `json:"applied"`
	Valid                  bool                           `json:"valid"`
	Error                  string                         `json:"error,omitempty"`
	Errors                 []string                       `json:"errors,omitempty"`
	FailureCode            string                         `json:"failureCode,omitempty"`
	FailureStage           string                         `json:"failureStage,omitempty"`
	RepairHints            []AdapterReportRepairHint      `json:"repairHints,omitempty"`
	GateEventID            string                         `json:"gateEventId"`
	ReportPath             string                         `json:"reportPath,omitempty"`
	Report                 *AdapterReport                 `json:"report,omitempty"`
	AdapterContext         *AdapterContext                `json:"adapterContext,omitempty"`
	Contract               AdapterExecutionReportContract `json:"contract"`
	MissionCommanderAction mission.MissionCommanderAction `json:"missionCommanderAction"`
	NextSteps              []string                       `json:"nextSteps"`
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
	return []AdapterReportRepairHint{{
		Stage:              "path",
		RepairAction:       "provide-execution-report-path",
		Fields:             []string{"executionReportPath"},
		AllowedOutputPaths: normalizedGatePaths(gateEvent.Gate.OutputPaths),
		RecordBlocked:      true,
		RerunValidation:    true,
		Detail:             "provide -ExecutionReportPath under an authorized output path before recording evidence",
	}}
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
		return []AdapterReportRepairHint{{
			Stage:           stage,
			RepairAction:    "escalate-validation-error",
			RecordBlocked:   true,
			RerunValidation: true,
			EscalateToMain:  true,
			Detail:          "validation failed without a stable failureCode; escalate the bounded sidecar and validation error to the main Agent",
		}}
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
	return []AdapterReportRepairHint{hint}
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

type ApplyResult struct {
	SchemaVersion          int                            `json:"schemaVersion"`
	Command                string                         `json:"command"`
	CaseRoot               string                         `json:"caseRoot"`
	RepoRoot               string                         `json:"repoRoot"`
	Pack                   string                         `json:"pack"`
	IsMutation             bool                           `json:"isMutation"`
	Applied                bool                           `json:"applied"`
	EventID                string                         `json:"eventId"`
	Path                   string                         `json:"path"`
	Reason                 string                         `json:"reason,omitempty"`
	Event                  *EventPreview                  `json:"event,omitempty"`
	ExecutionEvidence      *ExecutionEvidencePreview      `json:"executionEvidence,omitempty"`
	MissionBrief           mission.Brief                  `json:"missionBrief"`
	ExecutorAction         mission.ExecutorAction         `json:"executorAction"`
	MissionCommanderAction mission.MissionCommanderAction `json:"missionCommanderAction"`
	NextSteps              []string                       `json:"nextSteps"`
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
	return Plan{
		SchemaVersion:        1,
		Command:              "gate",
		CaseRoot:             inst.CaseRoot,
		RepoRoot:             repoRoot,
		Pack:                 pack,
		IsMutation:           false,
		ReviewRequired:       preview.Gate.RequiresConfirmation,
		RequiresConfirmation: preview.Gate.RequiresConfirmation,
		EventPreview:         preview,
		MissionBrief:         brief,
		ExecutorAction:       gateExecutorAction(inst.CaseRoot, preview.Lane, brief),
		WouldExecutorAction:  gateWouldExecutorAction(inst.CaseRoot, preview, brief),
		BlockedActions:       blocked,
		NextSteps:            planNextSteps(preview),
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
		result.MissionCommanderAction = result.ExecutorAction.MissionCommanderAction
		result.Reason = "duplicate eventId"
		return result, nil
	}
	if _, _, err := mission.AppendFact(inst.CaseRoot, "request", preview); err != nil {
		return ApplyResult{}, err
	}
	result.Applied = true
	result.MissionBrief = gateMissionBrief(inst.CaseRoot)
	result.ExecutorAction = gateExecutorAction(inst.CaseRoot, preview.Lane, result.MissionBrief)
	result.MissionCommanderAction = result.ExecutorAction.MissionCommanderAction
	return result, nil
}

func RecordExecution(repoRoot, caseRoot, pack string, opt Options) (ApplyResult, error) {
	if strings.TrimSpace(opt.Actor) == "" {
		return ApplyResult{}, fmt.Errorf("gate execution evidence requires -Actor <recorded-by>")
	}
	inst, gateEvent, err := authorizedGateEvent(repoRoot, caseRoot, pack, opt)
	if err != nil {
		return ApplyResult{}, err
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
		result.Reason = "duplicate eventId"
		return result, nil
	}
	if _, _, err := mission.AppendFact(inst.CaseRoot, "observation", execution); err != nil {
		return ApplyResult{}, err
	}
	result.Applied = true
	result.MissionBrief = gateMissionBrief(inst.CaseRoot)
	result.ExecutorAction = gateExecutorAction(inst.CaseRoot, execution.Lane, result.MissionBrief)
	result.MissionCommanderAction = executionCommanderAction(execution, result.Applied, false)
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
	reportRel, adapterReport, err := readAdapterExecutionReport(inst.CaseRoot, gateEvent, opt.ExecutionReportCwd, opt.ExecutionReportPath)
	if reportRel != "" {
		validation.ReportPath = reportRel
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
		validation.MissionCommanderAction = adapterReportValidationCommanderAction(pack, gateEvent, validation.ReportPath, false, validation.RepairHints)
		validation.NextSteps = adapterReportRepairNextSteps(validation.RepairHints)
		return validation, nil
	}
	if adapterReport == nil {
		validation.Valid = false
		validation.Error = "gate execution report validation requires -ExecutionReportPath"
		validation.Errors = []string{validation.Error}
		validation.RepairHints = adapterReportMissingPathRepairHints(gateEvent)
		validation.MissionCommanderAction = adapterReportValidationCommanderAction(pack, gateEvent, validation.ReportPath, false, validation.RepairHints)
		validation.NextSteps = adapterReportRepairNextSteps(validation.RepairHints)
		return validation, nil
	}
	validation.Valid = true
	validation.MissionCommanderAction = adapterReportValidationCommanderAction(pack, gateEvent, validation.ReportPath, true, nil)
	validation.NextSteps = adapterReportValidationNextSteps(pack, gateEvent, validation.ReportPath)
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
	liveValidation := adapterReportLiveValidation(m, pack, event)
	return AdapterExecutionReportContract{
		SchemaVersion:           1,
		Command:                 "gate",
		Kind:                    "adapter-execution-report-contract",
		CaseRoot:                caseRoot,
		RepoRoot:                repoRoot,
		Pack:                    pack,
		IsMutation:              false,
		Lane:                    event.Lane,
		Target:                  event.Target,
		BatchID:                 event.BatchID,
		Risk:                    event.Risk,
		Authorization:           event.Gate.Authorization,
		ReportKind:              "adapter-execution-report",
		ReportSchemaVersion:     1,
		GateEventID:             event.EventID,
		Action:                  event.Gate.Action,
		AllowedStatuses:         []string{"succeeded", "failed", "boundary-hit", "escalated", "aborted"},
		RequiredFields:          []string{"schemaVersion", "kind", "adapterId", "action", "status", "gateEventId", "actualBudget"},
		AllowedOutputPaths:      append([]string{}, event.Gate.OutputPaths...),
		AuthorizedBudget:        event.Gate.RequestedBudget,
		StopConditions:          append([]string{}, event.Gate.StopConditions...),
		DefaultReportPath:       adapterReportDefaultPath(event.Gate.OutputPaths),
		ReportPathRule:          "case-relative, current-workspace relative, or case-contained absolute file path under one authorized outputPath; sidecar must be <= 1048576 bytes and contain no trailing JSON data",
		RefPathRequires:         []string{"outputRefs and evidenceRefs must be case-relative", "outputRefs and evidenceRefs must stay under authorized outputPaths"},
		SummaryMaxBytes:         4096,
		EscalationMaxBytes:      4096,
		RecordRequired:          event.Gate.Authorization.RecordRequired,
		NotifyMainOn:            append([]string{}, event.Gate.Authorization.NotifyMainOn...),
		BoundaryStatusRequires:  []string{"boundaryHits or escalation for boundary-hit/escalated status", "boundaryHits or escalation when actualBudget exceeds authorizedBudget", "boundaryHits must be one of authorized stopConditions"},
		StatusSummaryRequires:   []string{"summary for failed/boundary-hit/escalated/aborted status"},
		ValidationFailureStages: adapterReportValidationFailureStages(),
		ValidationFailureCodes:  adapterReportValidationFailureCodes(),
		ValidationRepairHints:   adapterReportContractRepairHints(event),
		DeniedActions:           []string{"heavy-tool execution", "authority writes", "confirmed writes", "out-of-scope output refs", "full trace/dump/log embedding"},
		LiveValidation:          liveValidation,
		MissionCommanderAction:  adapterReportContractCommanderAction(event, pack, liveValidation),
		NextSteps:               adapterReportContractNextSteps(pack, event, liveValidation),
	}
}

func adapterReportContractCommanderAction(event EventPreview, pack string, liveValidation AdapterReportLiveValidation) mission.MissionCommanderAction {
	reportPath := strings.TrimSpace(liveValidation.CaseRelativeReportPath)
	if reportPath == "" {
		reportPath = "<reportPath-under-authorized-outputPath>"
	}
	validateCommand := adapterReportValidateSlashCommand(pack, event.EventID, reportPath)
	recordCommand := adapterReportRecordSlashCommand(pack, event.EventID, reportPath)
	return mission.MissionCommanderAction{
		State:          "needs-adapter-report-validation",
		Prompt:         fmt.Sprintf("按 authorized gate `%s` 接手：先让 executor/tool adapter 在授权 outputPath 写 bounded sidecar，再用 read-only validation 预检；valid=true 后才 record observation evidence。", event.EventID),
		PrimaryCommand: validateCommand,
		FollowUpCommands: []string{
			recordCommand,
			"/rekit handoff " + mission.BoardLaneLabel(mission.BoardLane{ID: event.Lane}),
		},
		Boundary: adapterReportCommanderBoundary(),
	}
}

func adapterReportContractNextSteps(pack string, event EventPreview, liveValidation AdapterReportLiveValidation) []string {
	reportPath := strings.TrimSpace(liveValidation.CaseRelativeReportPath)
	if reportPath == "" {
		reportPath = "<reportPath-under-authorized-outputPath>"
	}
	return []string{
		"adapter writes bounded report under authorized output path: " + reportPath,
		"preflight read-only: " + adapterReportValidateSlashCommand(pack, event.EventID, reportPath),
		"after valid=true record observation evidence: " + adapterReportRecordSlashCommand(pack, event.EventID, reportPath),
		"replace <executor-id> before record; /rekit records evidence only and never executes the heavy tool",
		"review refs before any authority/confirmed outcome",
	}
}

func adapterReportValidationCommanderAction(pack string, gateEvent EventPreview, reportPath string, valid bool, hints []AdapterReportRepairHint) mission.MissionCommanderAction {
	reportPath = strings.TrimSpace(reportPath)
	if reportPath == "" {
		reportPath = adapterReportDefaultPath(gateEvent.Gate.OutputPaths)
	}
	if reportPath == "" {
		reportPath = "<reportPath-under-authorized-outputPath>"
	}
	if valid {
		return mission.MissionCommanderAction{
			State:          "ready-to-record-evidence",
			Prompt:         fmt.Sprintf("authorized gate `%s` 的 sidecar 已 valid=true；替换 `<executor-id>` 后只记录 observation evidence，再 review refs。", gateEvent.EventID),
			PrimaryCommand: adapterReportRecordSlashCommand(pack, gateEvent.EventID, reportPath),
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

func adapterReportValidationNextSteps(pack string, gateEvent EventPreview, reportPath string) []string {
	return []string{
		"report is valid for read-only preflight",
		"record observation evidence: " + adapterReportRecordSlashCommand(pack, gateEvent.EventID, reportPath),
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

func adapterReportRecordSlashCommand(pack, gateEventID, reportPath string) string {
	return adapterReportSlashCommand([]string{"gate", "-Pack", pack, "-Apply", "-GateEventId", gateEventID, "-ExecutionReportPath", reportPath, "-Actor", "<executor-id>", "-Format", "json"})
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

func adapterReportLiveValidation(m *manifest.Manifest, pack string, event EventPreview) AdapterReportLiveValidation {
	reportFileName := "adapter-report.json"
	caseRelativeReportPath := adapterReportDefaultPath(event.Gate.OutputPaths)
	adapterCandidates := adapterToolCandidates(m, event)
	validateArgs := []string{"-Command", "gate", "-Pack", pack, "-GateEventId", event.EventID, "-ValidateExecutionReport", "-ExecutionReportPath", reportFileName, "-Format", "json"}
	recordArgs := []string{"-Command", "gate", "-Pack", pack, "-Apply", "-GateEventId", event.EventID, "-ExecutionReportPath", reportFileName, "-Actor", "<executor-id>", "-Format", "json"}
	caseRelativeValidateArgs := []string{}
	caseRelativeRecordArgs := []string{}
	caseRelativeValidateCommand := ""
	caseRelativeRecordCommand := ""
	if caseRelativeReportPath != "" {
		caseRelativeValidateArgs = []string{"-Command", "gate", "-Pack", pack, "-GateEventId", event.EventID, "-ValidateExecutionReport", "-ExecutionReportPath", caseRelativeReportPath, "-Format", "json"}
		caseRelativeRecordArgs = []string{"-Command", "gate", "-Pack", pack, "-Apply", "-GateEventId", event.EventID, "-ExecutionReportPath", caseRelativeReportPath, "-Actor", "<executor-id>", "-Format", "json"}
		caseRelativeValidateCommand = "rekit " + strings.Join(caseRelativeValidateArgs, " ")
		caseRelativeRecordCommand = "rekit " + strings.Join(caseRelativeRecordArgs, " ")
	}
	return AdapterReportLiveValidation{
		InvocationCwd:          "authorized output workspace listed in authorizedWorkspaces; use reportFileName as workspace-relative -ExecutionReportPath and omit -Target; or use caseRelativeReportPath with case-relative commands from any case-local cwd",
		AuthorizedWorkspaces:   normalizedGatePaths(event.Gate.OutputPaths),
		ReportFileName:         reportFileName,
		CaseRelativeReportPath: caseRelativeReportPath,
		SidecarTemplate: AdapterReportSidecarTemplate{
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
		},
		ValidateCommand:             "rekit " + strings.Join(validateArgs, " "),
		RecordCommand:               "rekit " + strings.Join(recordArgs, " "),
		ValidateArgs:                validateArgs,
		RecordArgs:                  recordArgs,
		CaseRelativeValidateCommand: caseRelativeValidateCommand,
		CaseRelativeRecordCommand:   caseRelativeRecordCommand,
		CaseRelativeValidateArgs:    caseRelativeValidateArgs,
		CaseRelativeRecordArgs:      caseRelativeRecordArgs,
		AdapterCandidates:           adapterCandidates,
		SelectedAdapter:             selectedAdapterToolCandidate(m, event, sidecarAdapterID(adapterCandidates)),
		ReplayBehavior:              "repeating RecordArgs or CaseRelativeRecordArgs with the same bounded sidecar returns applied=false and reason=duplicate eventId without appending observations",
		Notes: []string{
			"ValidateArgs and CaseRelativeValidateArgs are read-only: isMutation=false, applied=false, and no observations/authority/confirmed writes.",
			"Replace <executor-id> before running RecordArgs or CaseRelativeRecordArgs; both record observation evidence only after strict sidecar validation and never execute the heavy tool.",
			"Use only authorized stopConditions in boundaryHits; failed/boundary-hit/escalated/aborted reports require a bounded summary.",
			"Keep outputRefs/evidenceRefs case-relative and under authorized outputPaths so validation and record paths enforce the same artifact boundary.",
			"Keep full trace/dump/log data in sidecar artifacts referenced by outputRefs/evidenceRefs, not in this report.",
		},
	}
}

func executionEvidence(caseRoot string, gateEvent EventPreview, opt Options, m *manifest.Manifest) (ExecutionEvidencePreview, error) {
	reportRel, adapterReport, err := readAdapterExecutionReport(caseRoot, gateEvent, opt.ExecutionReportCwd, opt.ExecutionReportPath)
	if err != nil {
		return ExecutionEvidencePreview{}, err
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
			Status:              status,
			ActualBudget:        actual,
			OutputRefs:          outputRefs,
			BoundaryHits:        boundaryHits,
			Escalation:          escalation,
			GateEventID:         gateEvent.EventID,
			GateStatus:          gateEvent.Status,
			Authorization:       gateEvent.Gate.Authorization.Decision,
			RecordRequired:      gateEvent.Gate.Authorization.RecordRequired,
			NotifyMainOn:        append([]string{}, gateEvent.Gate.Authorization.NotifyMainOn...),
			ExecutionReportPath: reportRel,
			AdapterContext:      adapterContext,
			Adapter:             adapterReport,
		},
	}, nil
}

func actualBudgetFieldMismatch(explicit, reported autonomy.Budget) bool {
	return (explicit.RuntimeSeconds != 0 && explicit.RuntimeSeconds != reported.RuntimeSeconds) || (explicit.DiskMB != 0 && explicit.DiskMB != reported.DiskMB) || (explicit.Requests != 0 && explicit.Requests != reported.Requests)
}

func readAdapterExecutionReport(caseRoot string, gateEvent EventPreview, cwd, value string) (string, *AdapterReport, error) {
	path := strings.TrimSpace(value)
	if path == "" {
		return "", nil, nil
	}
	if len(splitList(path)) != 1 {
		return "", nil, adapterReportValidationErrorf("path-list", "path", "gate execution report path must be a single file path")
	}
	fullPath, relPath, err := executionReportPath(caseRoot, path)
	if err != nil {
		return "", nil, adapterReportValidationErrorf("path-invalid", "path", "%w", err)
	}
	if !outputRefsWithinGate(gateEvent.Gate.OutputPaths, []string{relPath}) {
		if cwdFullPath, cwdRelPath, ok, err := cwdAuthorizedExecutionReportPath(caseRoot, gateEvent, cwd, path); err != nil {
			return "", nil, adapterReportValidationErrorf("path-invalid", "path", "%w", err)
		} else if ok {
			fullPath = cwdFullPath
			relPath = cwdRelPath
		} else {
			return relPath, nil, adapterReportValidationErrorf("report-path-out-of-scope", "path", "gate execution report path must stay within authorized gate outputPaths")
		}
	}
	st, err := os.Stat(fullPath)
	if err != nil {
		return relPath, nil, adapterReportValidationErrorf("report-not-readable", "read", "read adapter execution report %s: %w", relPath, err)
	}
	if st.IsDir() {
		return relPath, nil, adapterReportValidationErrorf("report-path-directory", "read", "adapter execution report path is a directory: %s", relPath)
	}
	if st.Size() > 1<<20 {
		return relPath, nil, adapterReportValidationErrorf("report-too-large", "read", "adapter execution report is too large: %s %d > %d", relPath, st.Size(), 1<<20)
	}
	f, err := os.Open(fullPath)
	if err != nil {
		return relPath, nil, adapterReportValidationErrorf("report-not-readable", "read", "read adapter execution report %s: %w", relPath, err)
	}
	defer f.Close()
	var report AdapterReport
	dec := json.NewDecoder(io.LimitReader(f, 1<<20))
	dec.DisallowUnknownFields()
	if err := dec.Decode(&report); err != nil {
		return relPath, nil, adapterReportValidationErrorf("report-json-invalid", "decode", "invalid adapter execution report %s: %w", relPath, err)
	}
	var trailing any
	if err := dec.Decode(&trailing); err != io.EOF {
		if err == nil {
			return relPath, &report, adapterReportValidationErrorf("report-trailing-data", "decode", "invalid adapter execution report %s: trailing data", relPath)
		}
		return relPath, &report, adapterReportValidationErrorf("report-trailing-data", "decode", "invalid adapter execution report %s: trailing data: %w", relPath, err)
	}
	if err := validateAdapterExecutionReport(caseRoot, gateEvent, &report); err != nil {
		return relPath, &report, err
	}
	return relPath, &report, nil
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
	if m == nil {
		return nil
	}
	action := strings.ToLower(strings.TrimSpace(event.Gate.Action))
	if action == "" {
		return nil
	}
	gate, ok := m.HeavyToolGate(action)
	if !ok {
		return nil
	}
	rows := map[string]AdapterToolCandidate{}
	for _, rel := range m.ToolingFiles {
		rel = strings.TrimSpace(rel)
		if rel == "" || !strings.HasSuffix(strings.ToLower(rel), "catalog.yml") {
			continue
		}
		path, err := m.SourcePath(rel)
		if err != nil {
			continue
		}
		items, err := manifest.ObjectListFromFile(path, "tools")
		if err != nil {
			continue
		}
		for _, item := range items {
			candidate := adapterToolCandidateFromCatalogItem(item, rel, action, gate)
			if candidate.ID == "" {
				continue
			}
			key := strings.ToLower(candidate.ID)
			if existing, ok := rows[key]; ok && adapterToolCandidateRank(existing) <= adapterToolCandidateRank(candidate) {
				continue
			}
			rows[key] = candidate
		}
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
