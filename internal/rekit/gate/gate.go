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
	Status              string          `json:"status"`
	ActualBudget        autonomy.Budget `json:"actualBudget"`
	OutputRefs          []string        `json:"outputRefs,omitempty"`
	BoundaryHits        []string        `json:"boundaryHits,omitempty"`
	Escalation          string          `json:"escalation,omitempty"`
	GateEventID         string          `json:"gateEventId"`
	GateStatus          string          `json:"gateStatus"`
	Authorization       string          `json:"authorization"`
	RecordRequired      bool            `json:"recordRequired"`
	NotifyMainOn        []string        `json:"notifyMainOn,omitempty"`
	ExecutionReportPath string          `json:"executionReportPath,omitempty"`
	Adapter             *AdapterReport  `json:"adapter,omitempty"`
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
	AuthorizedBudget        autonomy.Budget                       `json:"authorizedBudget"`
	StopConditions          []string                              `json:"stopConditions,omitempty"`
	ReportPathRule          string                                `json:"reportPathRule"`
	SummaryMaxBytes         int                                   `json:"summaryMaxBytes"`
	RecordRequired          bool                                  `json:"recordRequired"`
	NotifyMainOn            []string                              `json:"notifyMainOn,omitempty"`
	BoundaryStatusRequires  []string                              `json:"boundaryStatusRequires,omitempty"`
	StatusSummaryRequires   []string                              `json:"statusSummaryRequires,omitempty"`
	EscalationMaxBytes      int                                   `json:"escalationMaxBytes"`
	ValidationFailureStages []AdapterReportValidationFailureStage `json:"validationFailureStages,omitempty"`
	ValidationFailureCodes  []AdapterReportValidationFailureCode  `json:"validationFailureCodes,omitempty"`
	DeniedActions           []string                              `json:"deniedActions,omitempty"`
	LiveValidation          AdapterReportLiveValidation           `json:"liveValidation"`
	NextSteps               []string                              `json:"nextSteps,omitempty"`
}

type AdapterReportLiveValidation struct {
	InvocationCwd   string                       `json:"invocationCwd"`
	SidecarTemplate AdapterReportSidecarTemplate `json:"sidecarTemplate"`
	ValidateCommand string                       `json:"validateCommand"`
	RecordCommand   string                       `json:"recordCommand"`
	ValidateArgs    []string                     `json:"validateArgs"`
	RecordArgs      []string                     `json:"recordArgs"`
	ReplayBehavior  string                       `json:"replayBehavior"`
	Notes           []string                     `json:"notes,omitempty"`
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

type AdapterReportValidationFailureCode struct {
	Code        string `json:"code"`
	Stage       string `json:"stage"`
	Description string `json:"description"`
}

type AdapterExecutionReportValidation struct {
	SchemaVersion int                            `json:"schemaVersion"`
	Command       string                         `json:"command"`
	Kind          string                         `json:"kind"`
	CaseRoot      string                         `json:"caseRoot"`
	RepoRoot      string                         `json:"repoRoot"`
	Pack          string                         `json:"pack"`
	IsMutation    bool                           `json:"isMutation"`
	Applied       bool                           `json:"applied"`
	Valid         bool                           `json:"valid"`
	Error         string                         `json:"error,omitempty"`
	Errors        []string                       `json:"errors,omitempty"`
	FailureCode   string                         `json:"failureCode,omitempty"`
	FailureStage  string                         `json:"failureStage,omitempty"`
	GateEventID   string                         `json:"gateEventId"`
	ReportPath    string                         `json:"reportPath,omitempty"`
	Report        *AdapterReport                 `json:"report,omitempty"`
	Contract      AdapterExecutionReportContract `json:"contract"`
	NextSteps     []string                       `json:"nextSteps"`
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

type ApplyResult struct {
	SchemaVersion     int                       `json:"schemaVersion"`
	Command           string                    `json:"command"`
	CaseRoot          string                    `json:"caseRoot"`
	RepoRoot          string                    `json:"repoRoot"`
	Pack              string                    `json:"pack"`
	IsMutation        bool                      `json:"isMutation"`
	Applied           bool                      `json:"applied"`
	EventID           string                    `json:"eventId"`
	Path              string                    `json:"path"`
	Reason            string                    `json:"reason,omitempty"`
	Event             *EventPreview             `json:"event,omitempty"`
	ExecutionEvidence *ExecutionEvidencePreview `json:"executionEvidence,omitempty"`
	MissionBrief      mission.Brief             `json:"missionBrief"`
	ExecutorAction    mission.ExecutorAction    `json:"executorAction"`
	NextSteps         []string                  `json:"nextSteps"`
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
		result.Reason = "duplicate eventId"
		return result, nil
	}
	if _, _, err := mission.AppendFact(inst.CaseRoot, "request", preview); err != nil {
		return ApplyResult{}, err
	}
	result.Applied = true
	result.MissionBrief = gateMissionBrief(inst.CaseRoot)
	result.ExecutorAction = gateExecutorAction(inst.CaseRoot, preview.Lane, result.MissionBrief)
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
	execution, err := executionEvidence(inst.CaseRoot, gateEvent, opt)
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
		result.Reason = "duplicate eventId"
		return result, nil
	}
	if _, _, err := mission.AppendFact(inst.CaseRoot, "observation", execution); err != nil {
		return ApplyResult{}, err
	}
	result.Applied = true
	result.MissionBrief = gateMissionBrief(inst.CaseRoot)
	result.ExecutorAction = gateExecutorAction(inst.CaseRoot, execution.Lane, result.MissionBrief)
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
	return adapterReportContract(repoRoot, inst.CaseRoot, pack, event), nil
}

func ValidateAdapterExecutionReport(repoRoot, caseRoot, pack string, opt Options) (AdapterExecutionReportValidation, error) {
	inst, gateEvent, err := authorizedGateEvent(repoRoot, caseRoot, pack, opt)
	if err != nil {
		return AdapterExecutionReportValidation{}, err
	}
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
		Contract:      adapterReportContract(repoRoot, inst.CaseRoot, pack, gateEvent),
	}
	reportRel, adapterReport, err := readAdapterExecutionReport(inst.CaseRoot, gateEvent, opt.ExecutionReportCwd, opt.ExecutionReportPath)
	if reportRel != "" {
		validation.ReportPath = reportRel
	}
	if adapterReport != nil {
		validation.Report = adapterReport
	}
	if err != nil {
		validation.Valid = false
		validation.Error = err.Error()
		validation.Errors = []string{err.Error()}
		if validationErr, ok := err.(*adapterReportValidationError); ok {
			validation.FailureCode = validationErr.Code
			validation.FailureStage = validationErr.Stage
		}
		validation.NextSteps = []string{"report is invalid for read-only preflight", "fix the bounded adapter execution report sidecar and rerun gate -ValidateExecutionReport", "do not record it with gate -Apply until valid=true"}
		return validation, nil
	}
	if adapterReport == nil {
		validation.Valid = false
		validation.Error = "gate execution report validation requires -ExecutionReportPath"
		validation.Errors = []string{validation.Error}
		validation.NextSteps = []string{"provide -ExecutionReportPath under an authorized output path", "rerun gate -ValidateExecutionReport before recording evidence"}
		return validation, nil
	}
	validation.Valid = true
	validation.NextSteps = []string{"report is valid for read-only preflight", "record it after the authorized action with gate -Apply -GateEventId ... -ExecutionReportPath ...", "review refs before any authority/confirmed outcome"}
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

func adapterReportContract(repoRoot, caseRoot, pack string, event EventPreview) AdapterExecutionReportContract {
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
		ReportPathRule:          "case-relative, current-workspace relative, or case-contained absolute file path under one authorized outputPath; sidecar must be <= 1048576 bytes and contain no trailing JSON data",
		SummaryMaxBytes:         4096,
		EscalationMaxBytes:      4096,
		RecordRequired:          event.Gate.Authorization.RecordRequired,
		NotifyMainOn:            append([]string{}, event.Gate.Authorization.NotifyMainOn...),
		BoundaryStatusRequires:  []string{"boundaryHits or escalation for boundary-hit/escalated status", "boundaryHits or escalation when actualBudget exceeds authorizedBudget", "boundaryHits must be one of authorized stopConditions"},
		StatusSummaryRequires:   []string{"summary for failed/boundary-hit/escalated/aborted status"},
		ValidationFailureStages: adapterReportValidationFailureStages(),
		ValidationFailureCodes:  adapterReportValidationFailureCodes(),
		DeniedActions:           []string{"heavy-tool execution", "authority writes", "confirmed writes", "out-of-scope output refs", "full trace/dump/log embedding"},
		LiveValidation:          adapterReportLiveValidation(pack, event),
		NextSteps:               []string{"adapter writes bounded report under an authorized output path", "preflight the sidecar with gate -ValidateExecutionReport -GateEventId ... -ExecutionReportPath ... -Format json", "main Agent records valid reports with gate -Apply -GateEventId ... -ExecutionReportPath ...", "review refs before any authority/confirmed outcome"},
	}
}

func adapterReportLiveValidation(pack string, event EventPreview) AdapterReportLiveValidation {
	validateArgs := []string{"-Command", "gate", "-Pack", pack, "-GateEventId", event.EventID, "-ValidateExecutionReport", "-ExecutionReportPath", "adapter-report.json", "-Format", "json"}
	recordArgs := []string{"-Command", "gate", "-Pack", pack, "-Apply", "-GateEventId", event.EventID, "-ExecutionReportPath", "adapter-report.json", "-Actor", "<executor-id>", "-Format", "json"}
	return AdapterReportLiveValidation{
		InvocationCwd: "authorized output workspace; use a workspace-relative sidecar file name such as adapter-report.json and omit -Target",
		SidecarTemplate: AdapterReportSidecarTemplate{
			SchemaVersion: 1,
			Kind:          "adapter-execution-report",
			AdapterID:     "<adapter-id>",
			Action:        event.Gate.Action,
			Status:        "succeeded|failed|boundary-hit|escalated|aborted",
			GateEventID:   event.EventID,
			ActualBudget:  autonomy.Budget{},
			OutputRefs:    []string{"<case-relative output under authorized outputPaths>"},
			EvidenceRefs:  []string{"<case-relative bounded evidence ref>"},
			BoundaryHits:  []string{"<authorized stopCondition token when status/budget requires it>"},
			Escalation:    "<bounded escalation when status/budget requires it>",
			Summary:       "<bounded summary; required for failed/boundary-hit/escalated/aborted>",
		},
		ValidateCommand: "rekit " + strings.Join(validateArgs, " "),
		RecordCommand:   "rekit " + strings.Join(recordArgs, " "),
		ValidateArgs:    validateArgs,
		RecordArgs:      recordArgs,
		ReplayBehavior:  "repeating RecordArgs with the same bounded sidecar returns applied=false and reason=duplicate eventId without appending observations",
		Notes: []string{
			"ValidateArgs is read-only: isMutation=false, applied=false, and no observations/authority/confirmed writes.",
			"RecordArgs records observation evidence only after strict sidecar validation; it never executes the heavy tool.",
			"Use only authorized stopConditions in boundaryHits; failed/boundary-hit/escalated/aborted reports require a bounded summary.",
			"Keep full trace/dump/log data in sidecar artifacts referenced by outputRefs/evidenceRefs, not in this report.",
		},
	}
}

func executionEvidence(caseRoot string, gateEvent EventPreview, opt Options) (ExecutionEvidencePreview, error) {
	reportRel, adapterReport, err := readAdapterExecutionReport(caseRoot, gateEvent, opt.ExecutionReportCwd, opt.ExecutionReportPath)
	if err != nil {
		return ExecutionEvidencePreview{}, err
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
	evidenceRefs := splitList(opt.EvidenceRefs)
	if adapterReport != nil {
		if len(evidenceRefs) > 0 && len(adapterReport.EvidenceRefs) > 0 && strings.Join(normalizedGatePaths(evidenceRefs), ",") != strings.Join(normalizedGatePaths(adapterReport.EvidenceRefs), ",") {
			return ExecutionEvidencePreview{}, fmt.Errorf("adapter execution report evidenceRefs do not match explicit ExecutionEvidenceRefs")
		}
		if len(evidenceRefs) == 0 {
			evidenceRefs = append([]string{}, adapterReport.EvidenceRefs...)
		}
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
	if event.Status == "boundary-hit" || event.Status == "escalated" || event.Execution.Escalation != "" || len(event.Execution.BoundaryHits) > 0 {
		return []string{
			"Execution evidence recorded a boundary hit or escalation; stop autonomous work on this action and notify the main Agent.",
			"Review output refs and evidence refs before recording any authority/confirmed outcome.",
		}
	}
	return []string{
		"Execution evidence recorded the authorized action outcome; /rekit did not execute the heavy tool.",
		"Review output refs and evidence refs before recording any authority/confirmed outcome.",
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
