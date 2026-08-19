package sessionhost

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"reflect"
	"slices"
	"strings"
	"time"

	"github.com/shuiyu486/re-context-kits/internal/rekit/adapterhost"
	"github.com/shuiyu486/re-context-kits/internal/rekit/defaults"
	"github.com/shuiyu486/re-context-kits/internal/rekit/executioncontrol"
	rekitfs "github.com/shuiyu486/re-context-kits/internal/rekit/fs"
	"github.com/shuiyu486/re-context-kits/internal/rekit/lanemutation"
	"github.com/shuiyu486/re-context-kits/internal/rekit/laneowner"
	"github.com/shuiyu486/re-context-kits/internal/rekit/memberexecution"
	"github.com/shuiyu486/re-context-kits/internal/rekit/mission"
	"github.com/shuiyu486/re-context-kits/internal/rekit/projectstate"
	"github.com/shuiyu486/re-context-kits/internal/rekit/workstream"
)

const (
	binaryREAdapterLifecycleKind       = "binary-re-vmp-ida-adapter-lifecycle"
	binaryREAdapterExecutionIntentKind = "binary-re-vmp-ida-execution-intent"
	binaryREAdapterExecutionResultKind = "binary-re-vmp-ida-execution-result"
	binaryREEvidenceReviewIntentKind   = "binary-re-vmp-ida-evidence-review-intent"
	binaryREEvidenceReviewDecisionKind = "binary-re-vmp-ida-evidence-review-decision"
	binaryREEvidenceReviewClosureKind  = "binary-re-vmp-ida-evidence-review-closure"
	binaryREAdapterActor               = "mission-commander"
	binaryREAdapterProcessTimeout      = 30 * time.Second
	binaryREAdapterArtifactMaxBytes    = 2 << 20
)

var (
	binaryREAdapterNow              = nowRFC3339Nano
	binaryREEvidenceReviewRunClaude = runClaude
)

type binaryREAuthorizedRunner func(string, adapterhost.AuthorizedRunOptions, time.Duration) (adapterhost.AuthorizedRunResult, int, error)

type BinaryREAdapterLifecycleResult struct {
	SchemaVersion                int                                 `json:"schemaVersion"`
	Kind                         string                              `json:"kind"`
	State                        string                              `json:"state"`
	GateEventID                  string                              `json:"gateEventId"`
	AdapterID                    string                              `json:"adapterId"`
	RequestPath                  string                              `json:"requestPath"`
	RequestSHA256                string                              `json:"requestSha256"`
	ExecutionIntentPath          string                              `json:"executionIntentPath"`
	ExecutionIntentSHA256        string                              `json:"executionIntentSha256"`
	ExecutionResultPath          string                              `json:"executionResultPath,omitempty"`
	ExecutionResultSHA256        string                              `json:"executionResultSha256,omitempty"`
	ExecutionStatus              string                              `json:"executionStatus,omitempty"`
	AdapterProcessID             int                                 `json:"adapterProcessId,omitempty"`
	ChildLaunched                bool                                `json:"childLaunched"`
	AdapterReplay                bool                                `json:"adapterReplay"`
	Run                          *adapterhost.AuthorizedRunResult    `json:"run,omitempty"`
	EvidenceReviewInputPath      string                              `json:"evidenceReviewInputPath,omitempty"`
	EvidenceReviewInputSHA256    string                              `json:"evidenceReviewInputSha256,omitempty"`
	EvidenceReviewIntentPath     string                              `json:"evidenceReviewIntentPath,omitempty"`
	EvidenceReviewIntentSHA256   string                              `json:"evidenceReviewIntentSha256,omitempty"`
	EvidenceReviewDecisionPath   string                              `json:"evidenceReviewDecisionPath,omitempty"`
	EvidenceReviewDecisionSHA256 string                              `json:"evidenceReviewDecisionSha256,omitempty"`
	EvidenceReviewSession        string                              `json:"evidenceReviewSession,omitempty"`
	EvidenceReviewDecision       string                              `json:"evidenceReviewDecision,omitempty"`
	EvidenceReviewReplay         bool                                `json:"evidenceReviewReplay"`
	ResultPublication            *executioncontrol.ResultPublication `json:"resultPublication,omitempty"`
	AcknowledgementEventID       string                              `json:"acknowledgementEventId,omitempty"`
	AcknowledgementSHA256        string                              `json:"acknowledgementSha256,omitempty"`
	ClosurePath                  string                              `json:"closurePath,omitempty"`
	ClosureSHA256                string                              `json:"closureSha256,omitempty"`
	TaskBindingPath              string                              `json:"taskBindingPath,omitempty"`
	TaskBindingSHA256            string                              `json:"taskBindingSha256,omitempty"`
	SelectedEvidenceRef          string                              `json:"selectedEvidenceRef,omitempty"`
	ReadyForMember               bool                                `json:"readyForMember"`
	NoAuthority                  bool                                `json:"noAuthorityOrConfirmed"`
	NoHeavyToolAfterExecution    bool                                `json:"noHeavyToolAfterExecution"`
}

type binaryREAdapterSelection struct {
	Handoff      workstream.AuthorizedGateAdapterHandoff
	Request      adapterhost.VMPIDAIndexRequestRead
	CreatedAt    string
	Acknowledged bool
}

func prepareBinaryREAdapterBeforeMember(
	parent context.Context,
	opt DailyOptions,
	result *DailyResult,
) (bool, error) {
	if result == nil {
		return false, fmt.Errorf("daily result is missing before binary-re adapter preparation")
	}
	lifecycle, found, err := runBinaryREAdapterLifecycle(
		parent,
		opt,
		result.CaseRoot,
		result.Pack,
		result.Lane,
	)
	if err != nil {
		return false, fmt.Errorf("prepare ordinary binary-re adapter lifecycle: %w", err)
	}
	if !found {
		return true, nil
	}
	result.BinaryREAdapter = &lifecycle
	if lifecycle.ReadyForMember {
		return true, nil
	}
	result.FinalState = lifecycle.State
	result.Blocked = true
	result.Replay = lifecycle.AdapterReplay || lifecycle.EvidenceReviewReplay
	return false, nil
}

type binaryREAdapterExecutionIntent struct {
	SchemaVersion  int                      `json:"schemaVersion"`
	Kind           string                   `json:"kind"`
	Lane           string                   `json:"lane"`
	GateEventID    string                   `json:"gateEventId"`
	RequestPath    string                   `json:"requestPath"`
	RequestSHA256  string                   `json:"requestSha256"`
	ReportPath     string                   `json:"reportPath"`
	AdapterSession string                   `json:"adapterSession"`
	Actor          string                   `json:"actor"`
	Owner          laneowner.Snapshot       `json:"owner"`
	Control        executioncontrol.Binding `json:"executionControl"`
	CreatedAt      string                   `json:"createdAt"`
	NoAuthority    bool                     `json:"noAuthorityOrConfirmed"`
	NoAutoGate     bool                     `json:"noAutoGate"`
}

type binaryREAdapterExecutionResult struct {
	SchemaVersion             int                             `json:"schemaVersion"`
	Kind                      string                          `json:"kind"`
	GateEventID               string                          `json:"gateEventId"`
	ExecutionIntentPath       string                          `json:"executionIntentPath"`
	ExecutionIntentSHA256     string                          `json:"executionIntentSha256"`
	Control                   executioncontrol.Binding        `json:"executionControl"`
	Run                       adapterhost.AuthorizedRunResult `json:"run"`
	NoAuthority               bool                            `json:"noAuthorityOrConfirmed"`
	NoHeavyToolAfterExecution bool                            `json:"noHeavyToolAfterExecution"`
}

type binaryREEvidenceReviewIntent struct {
	SchemaVersion            int                      `json:"schemaVersion"`
	Kind                     string                   `json:"kind"`
	GateEventID              string                   `json:"gateEventId"`
	InputPath                string                   `json:"inputPath"`
	InputSHA256              string                   `json:"inputSha256"`
	ExecutionIntentPath      string                   `json:"executionIntentPath"`
	ExecutionIntentSHA256    string                   `json:"executionIntentSha256"`
	SessionID                string                   `json:"sessionId"`
	AttemptID                string                   `json:"attemptId"`
	AttemptSHA256            string                   `json:"attemptSha256"`
	StartedAt                string                   `json:"startedAt"`
	AcknowledgementCreatedAt string                   `json:"acknowledgementCreatedAt"`
	Owner                    laneowner.Snapshot       `json:"owner"`
	Control                  executioncontrol.Binding `json:"executionControl"`
	NoAuthority              bool                     `json:"noAuthorityOrConfirmed"`
	NoHeavyTool              bool                     `json:"noHeavyTool"`
}

type binaryREEvidenceReviewDecision struct {
	SchemaVersion int                           `json:"schemaVersion"`
	Kind          string                        `json:"kind"`
	GateEventID   string                        `json:"gateEventId"`
	InputPath     string                        `json:"inputPath"`
	InputSHA256   string                        `json:"inputSha256"`
	IntentPath    string                        `json:"intentPath"`
	IntentSHA256  string                        `json:"intentSha256"`
	SessionID     string                        `json:"sessionId"`
	Control       executioncontrol.Binding      `json:"executionControl"`
	ObservedAt    string                        `json:"observedAt"`
	Source        executioncontrol.ResultSource `json:"source"`
	Response      evidenceReviewResponse        `json:"response"`
	NoAuthority   bool                          `json:"noAuthorityOrConfirmed"`
	NoHeavyTool   bool                          `json:"noHeavyTool"`
}

type binaryREEvidenceReviewClosure struct {
	SchemaVersion          int                      `json:"schemaVersion"`
	Kind                   string                   `json:"kind"`
	GateEventID            string                   `json:"gateEventId"`
	ExecutionIntentPath    string                   `json:"executionIntentPath"`
	ExecutionIntentSHA256  string                   `json:"executionIntentSha256"`
	ExecutionResultPath    string                   `json:"executionResultPath"`
	ExecutionResultSHA256  string                   `json:"executionResultSha256"`
	InputPath              string                   `json:"inputPath"`
	InputSHA256            string                   `json:"inputSha256"`
	IntentPath             string                   `json:"intentPath"`
	IntentSHA256           string                   `json:"intentSha256"`
	DecisionPath           string                   `json:"decisionPath"`
	DecisionSHA256         string                   `json:"decisionSha256"`
	SessionID              string                   `json:"sessionId"`
	AcknowledgementEventID string                   `json:"acknowledgementEventId"`
	AcknowledgementSHA256  string                   `json:"acknowledgementSha256"`
	SelectedEvidenceRef    string                   `json:"selectedEvidenceRef"`
	Control                executioncontrol.Binding `json:"executionControl"`
	ClosedAt               string                   `json:"closedAt"`
	NoAuthority            bool                     `json:"noAuthorityOrConfirmed"`
	NoHeavyTool            bool                     `json:"noHeavyTool"`
}

func runBinaryREAdapterLifecycle(
	parent context.Context,
	dailyOpt DailyOptions,
	caseRoot,
	pack,
	lane string,
) (BinaryREAdapterLifecycleResult, bool, error) {
	if strings.TrimSpace(pack) != defaults.DefaultPack {
		return BinaryREAdapterLifecycleResult{}, false, nil
	}
	repoRoot, err := runtimeContextForDailyPack(caseRoot, pack)
	if err != nil {
		return BinaryREAdapterLifecycleResult{}, false, err
	}
	selection, found, err := discoverBinaryREVMPIDASelection(repoRoot, caseRoot, pack, lane)
	if err != nil || !found {
		return BinaryREAdapterLifecycleResult{}, found, err
	}
	intent, intentPath, intentSHA, err := ensureBinaryREAdapterExecutionIntent(caseRoot, lane, selection)
	if err != nil {
		return BinaryREAdapterLifecycleResult{}, true, err
	}
	result := BinaryREAdapterLifecycleResult{
		SchemaVersion: 1, Kind: binaryREAdapterLifecycleKind, State: "execution-ready",
		GateEventID: selection.Handoff.EventID, AdapterID: adapterhost.VMPIDAIndexAdapterID,
		RequestPath: selection.Request.RequestPath, RequestSHA256: selection.Request.RequestSHA256,
		ExecutionIntentPath: intentPath, ExecutionIntentSHA256: intentSHA,
		NoAuthority: true, NoHeavyToolAfterExecution: true,
	}

	executionResultPath, err := binaryREAdapterArtifactPath(caseRoot, lane, intent.GateEventID, "execution-result.json")
	if err != nil {
		return result, true, err
	}
	storedExecution, storedData, executionReplay, err := readBinaryREAdapterArtifact[binaryREAdapterExecutionResult](
		caseRoot,
		executionResultPath,
		"binary-re adapter execution result",
	)
	if err != nil {
		return result, true, err
	}
	var run adapterhost.AuthorizedRunResult
	if executionReplay {
		if err := validateBinaryREAdapterExecutionResult(storedExecution, caseRoot, intent, intentPath, intentSHA); err != nil {
			return result, true, err
		}
		if err := requireBinaryREControlCurrent(caseRoot, storedExecution.Control); err != nil {
			return result, true, err
		}
		run = storedExecution.Run
		result.ExecutionResultPath = executionResultPath
		result.ExecutionResultSHA256 = bytesSHA256(storedData)
		result.AdapterReplay = true
	} else {
		adapterPath := strings.TrimSpace(dailyOpt.binaryREAdapterPath)
		if adapterPath == "" {
			adapterPath, err = os.Executable()
			if err != nil {
				return result, true, err
			}
		}
		runner := dailyOpt.binaryREAdapterRunner
		if runner == nil {
			runner = adapterhost.RunAuthorizedGateProcess
		}
		processID := 0
		run, processID, err = runner(adapterPath, adapterhost.AuthorizedRunOptions{
			RepoRoot: repoRoot, CaseRoot: caseRoot, Pack: pack,
			GateEventID: selection.Handoff.EventID, ExecutionReportPath: intent.ReportPath,
			AdapterSession: intent.AdapterSession, Actor: intent.Actor,
			DeferSuccessfulTaskBinding: true,
			ExecutionControlBinding:    executioncontrol.CloneBinding(&intent.Control),
		}, binaryREAdapterProcessTimeout)
		result.AdapterProcessID = processID
		result.ChildLaunched = run.ChildLaunched
		result.AdapterReplay = run.Replay
		if err != nil {
			return result, true, err
		}
		if err := validateBinaryREAuthorizedRun(caseRoot, intent, run); err != nil {
			return result, true, err
		}
		storedExecution = binaryREAdapterExecutionResult{
			SchemaVersion: 1, Kind: binaryREAdapterExecutionResultKind,
			GateEventID: intent.GateEventID, ExecutionIntentPath: intentPath,
			ExecutionIntentSHA256: intentSHA, Control: intent.Control, Run: run,
			NoAuthority: true, NoHeavyToolAfterExecution: true,
		}
		executionSHA, replay, persistErr := persistBinaryREAdapterExecutionResult(
			caseRoot, executionResultPath, storedExecution, intent, intentPath, intentSHA,
		)
		if persistErr != nil {
			return result, true, persistErr
		}
		result.ExecutionResultPath = executionResultPath
		result.ExecutionResultSHA256 = executionSHA
		result.AdapterReplay = result.AdapterReplay || replay
	}
	result.ExecutionStatus = run.ExecutionStatus
	runCopy := run
	result.Run = &runCopy
	if run.ExecutionStatus != "succeeded" {
		result.State = "adapter-execution-" + run.ExecutionStatus
		return result, true, nil
	}

	item, err := binaryREObservationItem(caseRoot, lane, run)
	if err != nil {
		return result, true, err
	}
	input, err := inspectBinaryREVMPIDAEvidence(caseRoot, lane, item, intent.RequestPath, intent.RequestSHA256, run)
	if err != nil {
		return result, true, err
	}
	inputPath, err := binaryREAdapterArtifactPath(caseRoot, lane, intent.GateEventID, "evidence-review", "input.json")
	if err != nil {
		return result, true, err
	}
	inputSHA, err := writeBinaryREAdapterArtifact(caseRoot, inputPath, "binary-re evidence review input", input)
	if err != nil {
		return result, true, err
	}
	result.EvidenceReviewInputPath = inputPath
	result.EvidenceReviewInputSHA256 = inputSHA
	reviewIntent, reviewIntentPath, reviewIntentSHA, err := ensureBinaryREEvidenceReviewIntent(
		caseRoot, lane, intent, intentPath, intentSHA, inputPath, inputSHA,
	)
	if err != nil {
		return result, true, err
	}
	result.EvidenceReviewIntentPath = reviewIntentPath
	result.EvidenceReviewIntentSHA256 = reviewIntentSHA
	decision, decisionPath, decisionSHA, publication, replay, err := runBinaryREEvidenceReview(
		parent, dailyOpt, caseRoot, pack, lane, input, inputPath, inputSHA,
		reviewIntent, reviewIntentPath, reviewIntentSHA,
	)
	result.EvidenceReviewDecisionPath = decisionPath
	result.EvidenceReviewDecisionSHA256 = decisionSHA
	result.EvidenceReviewSession = reviewIntent.SessionID
	result.EvidenceReviewDecision = decision.Response.Decision
	result.EvidenceReviewReplay = replay
	result.ResultPublication = &publication
	result.SelectedEvidenceRef = input.Selected.EvidenceRef
	if err != nil {
		return result, true, err
	}
	if publication.Held {
		result.State = publication.Disposition
		return result, true, nil
	}
	if decision.Response.Decision != "accepted" {
		result.State = "evidence-review-rejected"
		return result, true, nil
	}

	ackID, ackSHA, err := acknowledgeBinaryREEvidenceReview(
		caseRoot, pack, lane, item, input, reviewIntent, decision.Response,
	)
	if err != nil {
		return result, true, err
	}
	closurePath, err := binaryREAdapterArtifactPath(caseRoot, lane, intent.GateEventID, "evidence-review", "closure.json")
	if err != nil {
		return result, true, err
	}
	closure := binaryREEvidenceReviewClosure{
		SchemaVersion: 1, Kind: binaryREEvidenceReviewClosureKind, GateEventID: intent.GateEventID,
		ExecutionIntentPath: intentPath, ExecutionIntentSHA256: intentSHA,
		ExecutionResultPath: executionResultPath, ExecutionResultSHA256: result.ExecutionResultSHA256,
		InputPath: inputPath, InputSHA256: inputSHA,
		IntentPath: reviewIntentPath, IntentSHA256: reviewIntentSHA,
		DecisionPath: decisionPath, DecisionSHA256: decisionSHA,
		SessionID: reviewIntent.SessionID, AcknowledgementEventID: ackID,
		AcknowledgementSHA256: ackSHA, SelectedEvidenceRef: input.Selected.EvidenceRef,
		Control: intent.Control, ClosedAt: reviewIntent.AcknowledgementCreatedAt,
		NoAuthority: true, NoHeavyTool: true,
	}
	closureSHA, err := writeBinaryREAdapterArtifact(caseRoot, closurePath, "binary-re evidence review closure", closure)
	if err != nil {
		return result, true, err
	}
	bindingPath, bindingSHA, err := bindBinaryREEvidenceForMember(
		caseRoot, lane, intent.Control, input,
		intentPath, intentSHA, executionResultPath, result.ExecutionResultSHA256,
		inputPath, inputSHA, reviewIntentPath, reviewIntentSHA,
		decisionPath, decisionSHA, closurePath, closureSHA,
		reviewIntent.SessionID, ackID, ackSHA,
	)
	if err != nil {
		return result, true, err
	}
	result.State = "ready-for-member"
	result.ReadyForMember = true
	result.AcknowledgementEventID = ackID
	result.AcknowledgementSHA256 = ackSHA
	result.ClosurePath = closurePath
	result.ClosureSHA256 = closureSHA
	result.TaskBindingPath = bindingPath
	result.TaskBindingSHA256 = bindingSHA
	return result, true, nil
}

func discoverBinaryREVMPIDASelection(repoRoot, caseRoot, pack, lane string) (binaryREAdapterSelection, bool, error) {
	facts, err := mission.ReadStrictLedgerFacts(caseRoot)
	if err != nil {
		return binaryREAdapterSelection{}, false, err
	}
	acknowledged := workstream.ExecutionEvidenceReviewAcknowledgedIDs(facts)
	matches := []binaryREAdapterSelection{}
	for _, requestEvent := range facts.Requests {
		handoffs := workstream.AuthorizedGateAdapterHandoffsWithAcknowledgements(
			repoRoot, caseRoot, pack, []map[string]any{requestEvent}, lane, acknowledged,
		)
		if len(handoffs) == 0 {
			continue
		}
		if len(handoffs) != 1 {
			return binaryREAdapterSelection{}, false, fmt.Errorf("binary-re adapter gate projection is not unique")
		}
		handoff := handoffs[0]
		target := filepath.ToSlash(strings.TrimSpace(handoff.Target))
		if !strings.HasPrefix(target, adapterhost.VMPIDAIndexRequestRoot+"/") {
			continue
		}
		request, readErr := adapterhost.ReadVMPIDAIndexRequest(caseRoot, target)
		if readErr != nil {
			return binaryREAdapterSelection{}, false, fmt.Errorf("read binary-re VMP IDA gate request: %w", readErr)
		}
		if handoff.Status != "authorized-gate" || handoff.Action != "inspect" || handoff.Authorization != "preauthorized" {
			return binaryREAdapterSelection{}, false, fmt.Errorf("binary-re VMP IDA gate authorization projection drifted")
		}
		if handoff.LiveValidation == nil {
			return binaryREAdapterSelection{}, false, fmt.Errorf("binary-re VMP IDA gate omitted live adapter validation")
		}
		live := handoff.LiveValidation
		if live.AdapterCandidateCount == 0 {
			continue
		}
		if live.AdapterCandidateCount != 1 || live.SelectedAdapter == nil ||
			live.SelectedAdapterID != adapterhost.VMPIDAIndexAdapterID ||
			live.SelectedAdapter.ID != adapterhost.VMPIDAIndexAdapterID ||
			live.SidecarTemplateAdapterID != adapterhost.VMPIDAIndexAdapterID ||
			!slices.Contains(live.SelectedAdapter.GateActions, "inspect") {
			return binaryREAdapterSelection{}, false, fmt.Errorf("binary-re VMP IDA gate adapter candidate is ambiguous or drifted")
		}
		if strings.TrimSpace(handoff.ReportContractError) != "" || strings.TrimSpace(handoff.ReportPath) == "" {
			return binaryREAdapterSelection{}, false, fmt.Errorf("binary-re VMP IDA gate report contract is unavailable: %s", handoff.ReportContractError)
		}
		createdAt := mission.Value(requestEvent, "createdAt")
		if _, timeErr := time.Parse(time.RFC3339Nano, createdAt); timeErr != nil {
			return binaryREAdapterSelection{}, false, fmt.Errorf("binary-re VMP IDA gate createdAt is invalid: %w", timeErr)
		}
		selection := binaryREAdapterSelection{
			Handoff: handoff, Request: request, CreatedAt: createdAt, Acknowledged: handoff.Acknowledged,
		}
		if selection.Acknowledged {
			complete, completionErr := completedBinaryREAdapterLifecycle(caseRoot, lane, selection)
			if completionErr != nil {
				return binaryREAdapterSelection{}, false, completionErr
			}
			if complete {
				continue
			}
		}
		matches = append(matches, selection)
	}
	if len(matches) == 0 {
		return binaryREAdapterSelection{}, false, nil
	}
	if len(matches) != 1 {
		return binaryREAdapterSelection{}, false, fmt.Errorf("multiple exact binary-re VMP IDA authorized gates require an explicit distinct lane route")
	}
	return matches[0], true, nil
}

func completedBinaryREAdapterLifecycle(
	caseRoot,
	lane string,
	selection binaryREAdapterSelection,
) (bool, error) {
	intentPath, err := binaryREAdapterArtifactPath(caseRoot, lane, selection.Handoff.EventID, "execution-intent.json")
	if err != nil {
		return false, err
	}
	intent, intentData, found, err := readBinaryREAdapterArtifact[binaryREAdapterExecutionIntent](
		caseRoot,
		intentPath,
		"binary-re adapter execution intent",
	)
	if err != nil {
		return false, err
	}
	if !found {
		// Acknowledged evidence from the retired acceptance-only path is provenance,
		// not an ordinary lifecycle to recover.
		return true, nil
	}
	if err := validateBinaryREAdapterExecutionIntent(intent, lane, selection); err != nil {
		return false, err
	}
	intentSHA := bytesSHA256(intentData)
	executionResultPath, err := binaryREAdapterArtifactPath(caseRoot, lane, intent.GateEventID, "execution-result.json")
	if err != nil {
		return false, err
	}
	executionResult, executionResultData, found, err := readBinaryREAdapterArtifact[binaryREAdapterExecutionResult](
		caseRoot,
		executionResultPath,
		"binary-re adapter execution result",
	)
	if err != nil {
		return false, err
	}
	if !found {
		return false, fmt.Errorf("acknowledged binary-re adapter lifecycle omitted its durable execution result")
	}
	if err := validateBinaryREAdapterExecutionResult(executionResult, caseRoot, intent, intentPath, intentSHA); err != nil {
		return false, err
	}
	if executionResult.Run.ExecutionStatus != "succeeded" {
		return false, fmt.Errorf("acknowledged binary-re adapter lifecycle did not succeed")
	}
	executionResultSHA := bytesSHA256(executionResultData)
	item, err := binaryREObservationItem(caseRoot, lane, executionResult.Run)
	if err != nil {
		return false, err
	}
	expectedInput, err := inspectBinaryREVMPIDAEvidence(
		caseRoot,
		lane,
		item,
		intent.RequestPath,
		intent.RequestSHA256,
		executionResult.Run,
	)
	if err != nil {
		return false, err
	}
	inputPath, err := binaryREAdapterArtifactPath(caseRoot, lane, intent.GateEventID, "evidence-review", "input.json")
	if err != nil {
		return false, err
	}
	input, inputData, found, err := readBinaryREAdapterArtifact[binaryREVMPIDAEvidenceReviewInput](
		caseRoot,
		inputPath,
		"binary-re evidence review input",
	)
	if err != nil {
		return false, err
	}
	if !found || !reflect.DeepEqual(input, expectedInput) {
		return false, fmt.Errorf("acknowledged binary-re evidence review input is missing or drifted")
	}
	inputSHA := bytesSHA256(inputData)
	reviewIntentPath, err := binaryREAdapterArtifactPath(caseRoot, lane, intent.GateEventID, "evidence-review", "intent.json")
	if err != nil {
		return false, err
	}
	reviewIntent, reviewIntentData, found, err := readBinaryREAdapterArtifact[binaryREEvidenceReviewIntent](
		caseRoot,
		reviewIntentPath,
		"binary-re evidence review intent",
	)
	if err != nil {
		return false, err
	}
	if !found {
		return false, fmt.Errorf("acknowledged binary-re lifecycle omitted its evidence review intent")
	}
	if err := validateBinaryREEvidenceReviewIntent(
		reviewIntent,
		intent,
		intentPath,
		intentSHA,
		inputPath,
		inputSHA,
	); err != nil {
		return false, err
	}
	reviewIntentSHA := bytesSHA256(reviewIntentData)
	decisionPath, err := binaryREAdapterArtifactPath(caseRoot, lane, intent.GateEventID, "evidence-review", "decision.json")
	if err != nil {
		return false, err
	}
	decision, decisionData, found, err := readBinaryREAdapterArtifact[binaryREEvidenceReviewDecision](
		caseRoot,
		decisionPath,
		"binary-re evidence review decision",
	)
	if err != nil {
		return false, err
	}
	if !found {
		return false, fmt.Errorf("acknowledged binary-re lifecycle omitted its evidence review decision")
	}
	decisionSHA := bytesSHA256(decisionData)
	if err := validateBinaryREEvidenceReviewDecision(
		decision,
		input,
		inputPath,
		inputSHA,
		reviewIntent,
		reviewIntentPath,
		reviewIntentSHA,
	); err != nil {
		return false, err
	}
	if decision.Response.Decision != "accepted" {
		return false, fmt.Errorf("acknowledged binary-re lifecycle has a non-accepted evidence review decision")
	}
	closurePath, err := binaryREAdapterArtifactPath(caseRoot, lane, intent.GateEventID, "evidence-review", "closure.json")
	if err != nil {
		return false, err
	}
	closure, closureData, closureFound, err := readBinaryREAdapterArtifact[binaryREEvidenceReviewClosure](
		caseRoot,
		closurePath,
		"binary-re evidence review closure",
	)
	if err != nil {
		return false, err
	}
	if !closureFound {
		return false, nil
	}
	if err := validateBinaryREEvidenceReviewClosure(
		caseRoot,
		lane,
		closure,
		intent,
		intentPath,
		intentSHA,
		executionResultPath,
		executionResultSHA,
		input,
		inputPath,
		inputSHA,
		reviewIntent,
		reviewIntentPath,
		reviewIntentSHA,
		decision,
		decisionPath,
		decisionSHA,
	); err != nil {
		return false, err
	}
	closureSHA := bytesSHA256(closureData)
	expectedBinding := binaryREMemberTaskBinding(
		input,
		intentPath,
		intentSHA,
		executionResultPath,
		executionResultSHA,
		inputPath,
		inputSHA,
		reviewIntentPath,
		reviewIntentSHA,
		decisionPath,
		decisionSHA,
		closurePath,
		closureSHA,
		reviewIntent.SessionID,
		closure.AcknowledgementEventID,
		closure.AcknowledgementSHA256,
	)
	binding, _, _, err := memberexecution.ReadTaskBindingForOwner(
		caseRoot,
		lane,
		intent.Control.Owner.ExecutorGeneration,
	)
	if err != nil {
		return false, err
	}
	if binding == nil {
		return false, nil
	}
	if !reflect.DeepEqual(*binding, expectedBinding) {
		return false, fmt.Errorf("acknowledged binary-re member task binding drifted from its reviewed lifecycle")
	}
	return true, nil
}

func validateBinaryREEvidenceReviewClosure(
	caseRoot,
	lane string,
	closure binaryREEvidenceReviewClosure,
	executionIntent binaryREAdapterExecutionIntent,
	executionIntentPath,
	executionIntentSHA,
	executionResultPath,
	executionResultSHA string,
	input binaryREVMPIDAEvidenceReviewInput,
	inputPath,
	inputSHA string,
	intent binaryREEvidenceReviewIntent,
	intentPath,
	intentSHA string,
	decision binaryREEvidenceReviewDecision,
	decisionPath,
	decisionSHA string,
) error {
	if closure.SchemaVersion != 1 || closure.Kind != binaryREEvidenceReviewClosureKind ||
		closure.GateEventID != executionIntent.GateEventID ||
		closure.ExecutionIntentPath != executionIntentPath ||
		!strings.EqualFold(closure.ExecutionIntentSHA256, executionIntentSHA) ||
		closure.ExecutionResultPath != executionResultPath ||
		!strings.EqualFold(closure.ExecutionResultSHA256, executionResultSHA) ||
		closure.InputPath != inputPath || !strings.EqualFold(closure.InputSHA256, inputSHA) ||
		closure.IntentPath != intentPath || !strings.EqualFold(closure.IntentSHA256, intentSHA) ||
		closure.DecisionPath != decisionPath || !strings.EqualFold(closure.DecisionSHA256, decisionSHA) ||
		closure.SessionID != intent.SessionID || closure.SelectedEvidenceRef != input.Selected.EvidenceRef ||
		closure.Control != executionIntent.Control || closure.ClosedAt != intent.AcknowledgementCreatedAt ||
		!closure.NoAuthority || !closure.NoHeavyTool || closure.AcknowledgementEventID == "" ||
		!validBinaryRESHA256(closure.AcknowledgementSHA256) {
		return fmt.Errorf("binary-re evidence review closure drifted from its exact reviewed lifecycle")
	}
	if _, err := time.Parse(time.RFC3339Nano, closure.ClosedAt); err != nil {
		return err
	}
	return validateBinaryREAcknowledgement(caseRoot, lane, input, intent, decision, closure)
}

func validateBinaryREAcknowledgement(
	caseRoot,
	lane string,
	input binaryREVMPIDAEvidenceReviewInput,
	intent binaryREEvidenceReviewIntent,
	decision binaryREEvidenceReviewDecision,
	closure binaryREEvidenceReviewClosure,
) error {
	facts, err := mission.ReadStrictLedgerFacts(caseRoot)
	if err != nil {
		return err
	}
	matches := 0
	for _, event := range facts.Verifications {
		if mission.Value(event, "eventId") != closure.AcknowledgementEventID {
			continue
		}
		encoded, marshalErr := json.Marshal(event)
		if marshalErr != nil {
			return marshalErr
		}
		if !strings.EqualFold(bytesSHA256(encoded), closure.AcknowledgementSHA256) ||
			mission.Value(event, "kind") != "verification" || mission.Value(event, "lane") != lane ||
			mission.Value(event, "subject") != "execution evidence review accepted" ||
			mission.Value(event, "summary") != "accepted recorded execution evidence for gateEventId "+input.GateEventID ||
			mission.Value(event, "actor") != binaryREAdapterActor || mission.Value(event, "verifier") != "tool-review" ||
			mission.Value(event, "verdict") != "accepted" || mission.Value(event, "status") != "resolved" ||
			mission.Value(event, "reason") != decision.Response.Summary+"; "+decision.Response.Reason ||
			mission.Value(event, "target") != input.RequestPath ||
			mission.Value(event, "createdAt") != intent.AcknowledgementCreatedAt ||
			!slices.Equal(binaryREEventStrings(event["evidenceRefs"]), input.EvidenceRefs) ||
			!slices.Contains(binaryREEventStrings(event["related"]), input.GateEventID) {
			return fmt.Errorf("binary-re evidence review acknowledgement drifted from its exact closure")
		}
		matches++
	}
	if matches != 1 || !workstream.ExecutionEvidenceReviewAcknowledgedIDs(facts)[input.GateEventID] {
		return fmt.Errorf("binary-re evidence review closure lacks one exact durable acknowledgement")
	}
	return nil
}

func binaryREEventStrings(value any) []string {
	out := []string{}
	switch items := value.(type) {
	case []string:
		out = append(out, items...)
	case []any:
		for _, item := range items {
			if text, ok := item.(string); ok {
				out = append(out, text)
			}
		}
	}
	return out
}

func ensureBinaryREAdapterExecutionIntent(
	caseRoot,
	lane string,
	selection binaryREAdapterSelection,
) (_ binaryREAdapterExecutionIntent, _ string, _ string, retErr error) {
	path, err := binaryREAdapterArtifactPath(caseRoot, lane, selection.Handoff.EventID, "execution-intent.json")
	if err != nil {
		return binaryREAdapterExecutionIntent{}, "", "", err
	}
	if existing, data, found, readErr := readBinaryREAdapterArtifact[binaryREAdapterExecutionIntent](caseRoot, path, "binary-re adapter execution intent"); readErr != nil {
		return binaryREAdapterExecutionIntent{}, "", "", readErr
	} else if found {
		if err := validateBinaryREAdapterExecutionIntent(existing, lane, selection); err != nil {
			return binaryREAdapterExecutionIntent{}, "", "", err
		}
		if err := requireBinaryREControlCurrent(caseRoot, existing.Control); err != nil {
			return binaryREAdapterExecutionIntent{}, "", "", err
		}
		return existing, path, bytesSHA256(data), nil
	}
	owner, err := laneowner.Read(caseRoot, lane)
	if err != nil {
		return binaryREAdapterExecutionIntent{}, "", "", err
	}
	control, err := executioncontrol.CaptureBinding(caseRoot, owner)
	if err != nil {
		return binaryREAdapterExecutionIntent{}, "", "", err
	}
	intent := binaryREAdapterExecutionIntent{
		SchemaVersion: 1, Kind: binaryREAdapterExecutionIntentKind, Lane: lane,
		GateEventID: selection.Handoff.EventID, RequestPath: selection.Request.RequestPath,
		RequestSHA256: selection.Request.RequestSHA256, ReportPath: selection.Handoff.ReportPath,
		AdapterSession: "binary-re-vmp-ida-" + shortStableIdentity(selection.Handoff.EventID),
		Actor:          binaryREAdapterActor, Owner: owner, Control: control, CreatedAt: selection.CreatedAt,
		NoAuthority: true, NoAutoGate: true,
	}
	if err := validateBinaryREAdapterExecutionIntent(intent, lane, selection); err != nil {
		return binaryREAdapterExecutionIntent{}, "", "", err
	}
	lease, err := lanemutation.AcquireLane(caseRoot, lane)
	if err != nil {
		return binaryREAdapterExecutionIntent{}, "", "", err
	}
	defer func() { retErr = errors.Join(retErr, lease.Unlock()) }()
	if err := executioncontrol.RequireCurrentBindingWithLease(caseRoot, lease, control); err != nil {
		return binaryREAdapterExecutionIntent{}, "", "", err
	}
	if existing, data, found, readErr := readBinaryREAdapterArtifact[binaryREAdapterExecutionIntent](caseRoot, path, "binary-re adapter execution intent"); readErr != nil {
		return binaryREAdapterExecutionIntent{}, "", "", readErr
	} else if found {
		if err := validateBinaryREAdapterExecutionIntent(existing, lane, selection); err != nil {
			return binaryREAdapterExecutionIntent{}, "", "", err
		}
		if err := executioncontrol.RequireCurrentBindingWithLease(caseRoot, lease, existing.Control); err != nil {
			return binaryREAdapterExecutionIntent{}, "", "", err
		}
		return existing, path, bytesSHA256(data), nil
	}
	sha, err := writeBinaryREAdapterArtifact(caseRoot, path, "binary-re adapter execution intent", intent)
	if err != nil {
		return binaryREAdapterExecutionIntent{}, "", "", err
	}
	if err := lease.Validate(); err != nil {
		return binaryREAdapterExecutionIntent{}, "", "", fmt.Errorf("binary-re adapter execution intent may already be durable: %w", err)
	}
	return intent, path, sha, nil
}

func validateBinaryREAdapterExecutionIntent(intent binaryREAdapterExecutionIntent, lane string, selection binaryREAdapterSelection) error {
	if intent.SchemaVersion != 1 || intent.Kind != binaryREAdapterExecutionIntentKind || intent.Lane != lane ||
		intent.GateEventID != selection.Handoff.EventID || intent.RequestPath != selection.Request.RequestPath ||
		!strings.EqualFold(intent.RequestSHA256, selection.Request.RequestSHA256) || intent.ReportPath != selection.Handoff.ReportPath ||
		intent.AdapterSession != "binary-re-vmp-ida-"+shortStableIdentity(selection.Handoff.EventID) ||
		intent.Actor != binaryREAdapterActor || intent.Owner != intent.Control.Owner || intent.Control.Lane != lane ||
		intent.CreatedAt != selection.CreatedAt || !intent.NoAuthority || !intent.NoAutoGate {
		return fmt.Errorf("binary-re adapter execution intent drifted from the exact gate, request, owner, or boundary")
	}
	if _, err := time.Parse(time.RFC3339Nano, intent.CreatedAt); err != nil {
		return err
	}
	return executioncontrol.ValidateBinding(intent.Control)
}

func validateBinaryREAuthorizedRun(caseRoot string, intent binaryREAdapterExecutionIntent, run adapterhost.AuthorizedRunResult) error {
	if run.SchemaVersion != 1 || run.Kind != "vmp-ida-index-authorized-run" ||
		!casePathEqual(run.CaseRoot, caseRoot) || run.Pack != defaults.DefaultPack ||
		run.GateEventID != intent.GateEventID || run.AdapterID != adapterhost.VMPIDAIndexAdapterID ||
		run.AdapterSession != intent.AdapterSession || run.ReportPath != intent.ReportPath ||
		run.DispatchPath == "" || !validBinaryRESHA256(run.DispatchSHA256) ||
		!validBinaryRESHA256(run.ReportSHA256) || run.ReceiptPath == "" ||
		!validBinaryRESHA256(run.ReceiptSHA256) || run.ObservationEventID == "" ||
		run.TaskBindingPath != "" || run.TaskBindingSHA256 != "" ||
		run.ProfilePath == "" || !validBinaryRESHA256(run.ProfileSHA256) ||
		(!run.ProfileRevoked && !run.ProfileAlreadyManual) || run.ExecutionExitStatus == "" ||
		!run.NoNetwork || run.NoNetworkBoundary != adapterhost.VMPIDAIndexNoNetworkBoundary || !run.NoAuthority {
		return fmt.Errorf("binary-re authorized adapter result omitted deferred-review or strict execution lineage")
	}
	switch run.ExecutionStatus {
	case "succeeded":
		if run.PacketPath == "" || !validBinaryRESHA256(run.PacketSHA256) {
			return fmt.Errorf("binary-re successful adapter result omitted its exact evidence packet")
		}
	case "failed", "aborted":
		if (run.PacketPath == "") != (run.PacketSHA256 == "") ||
			(run.PacketSHA256 != "" && !validBinaryRESHA256(run.PacketSHA256)) {
			return fmt.Errorf("binary-re terminal adapter result contains an invalid evidence packet binding")
		}
	default:
		return fmt.Errorf("binary-re authorized adapter result is not terminal")
	}
	return nil
}

func validateBinaryREAdapterExecutionResult(
	result binaryREAdapterExecutionResult,
	caseRoot string,
	intent binaryREAdapterExecutionIntent,
	intentPath,
	intentSHA string,
) error {
	if result.SchemaVersion != 1 || result.Kind != binaryREAdapterExecutionResultKind ||
		result.GateEventID != intent.GateEventID || result.ExecutionIntentPath != intentPath ||
		!strings.EqualFold(result.ExecutionIntentSHA256, intentSHA) || result.Control != intent.Control ||
		!result.NoAuthority || !result.NoHeavyToolAfterExecution {
		return fmt.Errorf("binary-re adapter execution result drifted from its exact intent or control birth")
	}
	if err := executioncontrol.ValidateBinding(result.Control); err != nil {
		return err
	}
	return validateBinaryREAuthorizedRun(caseRoot, intent, result.Run)
}

func persistBinaryREAdapterExecutionResult(
	caseRoot,
	path string,
	result binaryREAdapterExecutionResult,
	intent binaryREAdapterExecutionIntent,
	intentPath,
	intentSHA string,
) (_ string, _ bool, retErr error) {
	if err := validateBinaryREAdapterExecutionResult(result, caseRoot, intent, intentPath, intentSHA); err != nil {
		return "", false, err
	}
	lease, err := lanemutation.AcquireLane(caseRoot, intent.Lane)
	if err != nil {
		return "", false, err
	}
	defer func() { retErr = errors.Join(retErr, lease.Unlock()) }()
	if err := executioncontrol.RequireCurrentBindingWithLease(caseRoot, lease, intent.Control); err != nil {
		return "", false, err
	}
	if existing, data, found, readErr := readBinaryREAdapterArtifact[binaryREAdapterExecutionResult](
		caseRoot,
		path,
		"binary-re adapter execution result",
	); readErr != nil {
		return "", false, readErr
	} else if found {
		if err := validateBinaryREAdapterExecutionResult(existing, caseRoot, intent, intentPath, intentSHA); err != nil {
			return "", false, err
		}
		if !reflect.DeepEqual(existing, result) {
			return "", false, fmt.Errorf("binary-re adapter execution result differs from the durable terminal result")
		}
		return bytesSHA256(data), true, nil
	}
	sha, err := writeBinaryREAdapterArtifact(caseRoot, path, "binary-re adapter execution result", result)
	if err != nil {
		return "", false, err
	}
	if err := lease.Validate(); err != nil {
		return "", false, fmt.Errorf("binary-re adapter execution result may already be durable: %w", err)
	}
	return sha, false, nil
}

func validBinaryRESHA256(value string) bool {
	decoded, err := hex.DecodeString(strings.TrimSpace(value))
	return err == nil && len(decoded) == sha256.Size
}

func binaryREObservationItem(caseRoot, lane string, run adapterhost.AuthorizedRunResult) (mission.ExecutionEvidenceReviewItem, error) {
	facts, err := mission.ReadStrictLedgerFacts(caseRoot)
	if err != nil {
		return mission.ExecutionEvidenceReviewItem{}, err
	}
	var matched *mission.ExecutionEvidenceReviewItem
	for _, observation := range facts.Observations {
		item, ok := mission.ExecutionEvidenceReviewItemFromObservation(observation, lane, nil)
		if !ok || item.EventID != run.ObservationEventID || item.GateEventID != run.GateEventID {
			continue
		}
		if matched != nil {
			return mission.ExecutionEvidenceReviewItem{}, fmt.Errorf("binary-re adapter observation identity is not unique")
		}
		copy := item
		matched = &copy
	}
	if matched == nil {
		return mission.ExecutionEvidenceReviewItem{}, fmt.Errorf("binary-re adapter observation lineage is missing")
	}
	return *matched, nil
}

func ensureBinaryREEvidenceReviewIntent(
	caseRoot,
	lane string,
	executionIntent binaryREAdapterExecutionIntent,
	executionIntentPath,
	executionIntentSHA,
	inputPath,
	inputSHA string,
) (_ binaryREEvidenceReviewIntent, _ string, _ string, retErr error) {
	path, err := binaryREAdapterArtifactPath(caseRoot, lane, executionIntent.GateEventID, "evidence-review", "intent.json")
	if err != nil {
		return binaryREEvidenceReviewIntent{}, "", "", err
	}
	if existing, data, found, readErr := readBinaryREAdapterArtifact[binaryREEvidenceReviewIntent](caseRoot, path, "binary-re evidence review intent"); readErr != nil {
		return binaryREEvidenceReviewIntent{}, "", "", readErr
	} else if found {
		if err := validateBinaryREEvidenceReviewIntent(existing, executionIntent, executionIntentPath, executionIntentSHA, inputPath, inputSHA); err != nil {
			return binaryREEvidenceReviewIntent{}, "", "", err
		}
		if err := requireBinaryREControlCurrent(caseRoot, existing.Control); err != nil {
			return binaryREEvidenceReviewIntent{}, "", "", err
		}
		return existing, path, bytesSHA256(data), nil
	}
	lease, err := lanemutation.AcquireLane(caseRoot, lane)
	if err != nil {
		return binaryREEvidenceReviewIntent{}, "", "", err
	}
	defer func() { retErr = errors.Join(retErr, lease.Unlock()) }()
	if err := executioncontrol.RequireCurrentBindingWithLease(caseRoot, lease, executionIntent.Control); err != nil {
		return binaryREEvidenceReviewIntent{}, "", "", err
	}
	if existing, data, found, readErr := readBinaryREAdapterArtifact[binaryREEvidenceReviewIntent](caseRoot, path, "binary-re evidence review intent"); readErr != nil {
		return binaryREEvidenceReviewIntent{}, "", "", readErr
	} else if found {
		if err := validateBinaryREEvidenceReviewIntent(existing, executionIntent, executionIntentPath, executionIntentSHA, inputPath, inputSHA); err != nil {
			return binaryREEvidenceReviewIntent{}, "", "", err
		}
		return existing, path, bytesSHA256(data), nil
	}
	startedAt := binaryREAdapterNow()
	attemptSHA := hashStableIdentity(inputSHA, executionIntentSHA)
	intent := binaryREEvidenceReviewIntent{
		SchemaVersion: 1, Kind: binaryREEvidenceReviewIntentKind,
		GateEventID: executionIntent.GateEventID, InputPath: inputPath, InputSHA256: inputSHA,
		ExecutionIntentPath: executionIntentPath, ExecutionIntentSHA256: executionIntentSHA,
		SessionID:     stableUUID(executionIntent.GateEventID, inputSHA, executionIntentSHA),
		AttemptID:     "binary-re-evidence-review-" + shortStableIdentity(executionIntent.GateEventID),
		AttemptSHA256: attemptSHA, StartedAt: startedAt, AcknowledgementCreatedAt: startedAt,
		Owner: executionIntent.Owner, Control: executionIntent.Control,
		NoAuthority: true, NoHeavyTool: true,
	}
	if err := validateBinaryREEvidenceReviewIntent(intent, executionIntent, executionIntentPath, executionIntentSHA, inputPath, inputSHA); err != nil {
		return binaryREEvidenceReviewIntent{}, "", "", err
	}
	sha, err := writeBinaryREAdapterArtifact(caseRoot, path, "binary-re evidence review intent", intent)
	if err != nil {
		return binaryREEvidenceReviewIntent{}, "", "", err
	}
	if err := lease.Validate(); err != nil {
		return binaryREEvidenceReviewIntent{}, "", "", fmt.Errorf("binary-re evidence review intent may already be durable: %w", err)
	}
	return intent, path, sha, nil
}

func validateBinaryREEvidenceReviewIntent(
	intent binaryREEvidenceReviewIntent,
	executionIntent binaryREAdapterExecutionIntent,
	executionIntentPath,
	executionIntentSHA,
	inputPath,
	inputSHA string,
) error {
	if intent.SchemaVersion != 1 || intent.Kind != binaryREEvidenceReviewIntentKind ||
		intent.GateEventID != executionIntent.GateEventID || intent.InputPath != inputPath ||
		!strings.EqualFold(intent.InputSHA256, inputSHA) || intent.ExecutionIntentPath != executionIntentPath ||
		!strings.EqualFold(intent.ExecutionIntentSHA256, executionIntentSHA) || intent.Owner != executionIntent.Owner ||
		intent.Control != executionIntent.Control || intent.SessionID != stableUUID(executionIntent.GateEventID, inputSHA, executionIntentSHA) ||
		intent.AttemptID != "binary-re-evidence-review-"+shortStableIdentity(executionIntent.GateEventID) ||
		!strings.EqualFold(intent.AttemptSHA256, hashStableIdentity(inputSHA, executionIntentSHA)) ||
		!intent.NoAuthority || !intent.NoHeavyTool {
		return fmt.Errorf("binary-re evidence review intent drifted from its exact input, execution intent, or control birth")
	}
	if _, err := time.Parse(time.RFC3339Nano, intent.StartedAt); err != nil {
		return err
	}
	if _, err := time.Parse(time.RFC3339Nano, intent.AcknowledgementCreatedAt); err != nil {
		return err
	}
	return executioncontrol.ValidateBinding(intent.Control)
}

func runBinaryREEvidenceReview(
	parent context.Context,
	dailyOpt DailyOptions,
	caseRoot,
	pack,
	lane string,
	input binaryREVMPIDAEvidenceReviewInput,
	inputPath,
	inputSHA string,
	intent binaryREEvidenceReviewIntent,
	intentPath,
	intentSHA string,
) (binaryREEvidenceReviewDecision, string, string, executioncontrol.ResultPublication, bool, error) {
	decisionPath, err := binaryREAdapterArtifactPath(caseRoot, lane, intent.GateEventID, "evidence-review", "decision.json")
	if err != nil {
		return binaryREEvidenceReviewDecision{}, "", "", executioncontrol.ResultPublication{}, false, err
	}
	pkg := binaryREEvidenceReviewPackage(caseRoot, inputPath, inputSHA, intent)
	reviewOpt := binaryREEvidenceReviewOptions(dailyOpt, caseRoot, pack, intent.Control)
	if existing, data, found, readErr := readBinaryREAdapterArtifact[binaryREEvidenceReviewDecision](caseRoot, decisionPath, "binary-re evidence review decision"); readErr != nil {
		return binaryREEvidenceReviewDecision{}, "", "", executioncontrol.ResultPublication{}, false, readErr
	} else if found {
		if err := validateBinaryREEvidenceReviewDecision(existing, input, inputPath, inputSHA, intent, intentPath, intentSHA); err != nil {
			return binaryREEvidenceReviewDecision{}, "", "", executioncontrol.ResultPublication{}, false, err
		}
		if recovered, ok, recoverErr := recoverClaudeRunForCase(caseRoot, reviewOpt, pkg); recoverErr != nil {
			return binaryREEvidenceReviewDecision{}, "", "", executioncontrol.ResultPublication{}, false, recoverErr
		} else if ok {
			if err := removeClaudeRecoveryForCase(caseRoot, reviewOpt, pkg, recovered); err != nil {
				return binaryREEvidenceReviewDecision{}, "", "", executioncontrol.ResultPublication{}, false, err
			}
		}
		return existing, decisionPath, bytesSHA256(data), executioncontrol.ResultPublication{Published: true, Disposition: executioncontrol.ResultDispositionPublished}, true, nil
	}

	run, recovered, err := recoverClaudeRunForCase(caseRoot, reviewOpt, pkg)
	if err != nil {
		return binaryREEvidenceReviewDecision{}, "", "", executioncontrol.ResultPublication{}, false, err
	}
	replay := recovered
	if !recovered {
		run = binaryREEvidenceReviewRunClaude(parent, reviewOpt, pkg, intent.SessionID, nil)
		if !run.success() {
			return binaryREEvidenceReviewDecision{}, "", "", executioncontrol.ResultPublication{}, false, fmt.Errorf("independent binary-re evidence review failed: %s", run.failureReason())
		}
		if err := persistClaudeRecoveryForCase(caseRoot, reviewOpt, pkg, run); err != nil {
			return binaryREEvidenceReviewDecision{}, "", "", executioncontrol.ResultPublication{}, false, err
		}
		run, recovered, err = recoverClaudeRunForCase(caseRoot, reviewOpt, pkg)
		if err != nil || !recovered {
			return binaryREEvidenceReviewDecision{}, "", "", executioncontrol.ResultPublication{}, false, errors.Join(err, fmt.Errorf("independent binary-re evidence review recovery is missing"))
		}
	}
	if err := validateClaudeStructuredResult(pkg, run); err != nil {
		return binaryREEvidenceReviewDecision{}, "", "", executioncontrol.ResultPublication{}, replay, err
	}
	var response evidenceReviewResponse
	if err := strictJSON(run.structuredOutput, &response); err != nil {
		return binaryREEvidenceReviewDecision{}, "", "", executioncontrol.ResultPublication{}, replay, err
	}
	if err := validateBinaryREEvidenceReviewResponse(response, input); err != nil {
		return binaryREEvidenceReviewDecision{}, "", "", executioncontrol.ResultPublication{}, replay, err
	}
	publicationOpt, err := claudeResultPublicationOptions(reviewOpt, pkg, run)
	if err != nil {
		return binaryREEvidenceReviewDecision{}, "", "", executioncontrol.ResultPublication{}, replay, err
	}
	decision := binaryREEvidenceReviewDecision{
		SchemaVersion: 1, Kind: binaryREEvidenceReviewDecisionKind, GateEventID: intent.GateEventID,
		InputPath: inputPath, InputSHA256: inputSHA, IntentPath: intentPath, IntentSHA256: intentSHA,
		SessionID: intent.SessionID, Control: intent.Control, ObservedAt: publicationOpt.ObservedAt,
		Source: publicationOpt.Source, Response: response, NoAuthority: true, NoHeavyTool: true,
	}
	if err := validateBinaryREEvidenceReviewDecision(decision, input, inputPath, inputSHA, intent, intentPath, intentSHA); err != nil {
		return binaryREEvidenceReviewDecision{}, "", "", executioncontrol.ResultPublication{}, replay, err
	}
	decisionSHA := ""
	publication, err := executioncontrol.PublishResult(caseRoot, publicationOpt, func() error {
		var writeErr error
		decisionSHA, writeErr = writeBinaryREAdapterArtifact(caseRoot, decisionPath, "binary-re evidence review decision", decision)
		return writeErr
	})
	if err != nil {
		return binaryREEvidenceReviewDecision{}, "", "", publication, replay, err
	}
	if publication.Held {
		return decision, decisionPath, "", publication, replay, nil
	}
	if decisionSHA == "" {
		return binaryREEvidenceReviewDecision{}, "", "", publication, replay, fmt.Errorf("binary-re evidence review decision publication omitted its durable sha256")
	}
	if err := removeClaudeRecoveryForCase(caseRoot, reviewOpt, pkg, run); err != nil {
		return binaryREEvidenceReviewDecision{}, "", "", publication, replay, err
	}
	return decision, decisionPath, decisionSHA, publication, replay, nil
}

func binaryREEvidenceReviewPackage(caseRoot, inputPath, inputSHA string, intent binaryREEvidenceReviewIntent) mission.CurrentLoopExternalSessionHarnessPackage {
	control := intent.Control
	return mission.CurrentLoopExternalSessionHarnessPackage{
		SchemaVersion: 1, State: "launch-ready", CaseRoot: caseRoot,
		SessionKind: "mission-commander-evidence-review",
		Launch: &mission.CurrentLoopExternalSessionHarnessLaunch{
			Ready: true, Tool: "Claude Code Agent", AgentType: "read-only-evidence-reviewer", ReadOnly: true,
			Input:          mission.CurrentLoopExternalSessionHarnessInput{Path: inputPath, SHA256: inputSHA, Role: "mission-commander-evidence-review-input"},
			ExpectedOutput: "one strict accepted/rejected evidence review decision bound to the exact selected row, observation, and receipt",
			Attempt: mission.CurrentLoopExternalSessionAttempt{
				AttemptID: intent.AttemptID, AttemptSHA256: intent.AttemptSHA256, Generation: 1,
				Harness: defaultHarness, Session: intent.SessionID, Actor: binaryREAdapterActor,
				StartedAt: intent.StartedAt, LaunchControl: &control,
			},
		},
	}
}

func binaryREEvidenceReviewOptions(dailyOpt DailyOptions, caseRoot, pack string, control executioncontrol.Binding) Options {
	timeout := dailyOpt.Timeout
	if timeout <= 0 {
		timeout = defaultTimeout
	}
	return Options{
		Target: caseRoot, Pack: pack, Actor: binaryREAdapterActor,
		ClaudePath:                        dailyOpt.ClaudePath,
		ExpectedClaudeExecutableSHA256:    dailyOpt.ExpectedClaudeExecutableSHA256,
		ExpectedClaudeExecutablePublisher: dailyOpt.ExpectedClaudeExecutablePublisher,
		Model:                             dailyOpt.Model, Timeout: timeout, MaxAttempts: 1,
		projectExecutionLease: dailyOpt.projectExecutionLease,
		launchControlBinding:  executioncontrol.CloneBinding(&control),
	}
}

func validateBinaryREEvidenceReviewResponse(response evidenceReviewResponse, input binaryREVMPIDAEvidenceReviewInput) error {
	if err := validateEvidenceReviewResponse(response); err != nil {
		return err
	}
	if response.SelectedEvidenceRef != input.Selected.EvidenceRef ||
		response.ObservationEventID != input.ObservationEventID ||
		!strings.EqualFold(response.ReceiptSHA256, input.ReceiptSHA256) ||
		!slices.Equal(response.EvidenceRefs, input.EvidenceRefs) {
		return fmt.Errorf("independent binary-re evidence review decision drifted from exact input lineage")
	}
	return nil
}

func validateBinaryREEvidenceReviewDecision(
	decision binaryREEvidenceReviewDecision,
	input binaryREVMPIDAEvidenceReviewInput,
	inputPath,
	inputSHA string,
	intent binaryREEvidenceReviewIntent,
	intentPath,
	intentSHA string,
) error {
	if decision.SchemaVersion != 1 || decision.Kind != binaryREEvidenceReviewDecisionKind ||
		decision.GateEventID != intent.GateEventID || decision.InputPath != inputPath ||
		!strings.EqualFold(decision.InputSHA256, inputSHA) || decision.IntentPath != intentPath ||
		!strings.EqualFold(decision.IntentSHA256, intentSHA) || decision.SessionID != intent.SessionID ||
		decision.Control != intent.Control || !decision.NoAuthority || !decision.NoHeavyTool {
		return fmt.Errorf("binary-re evidence review decision drifted from its exact intent or control birth")
	}
	if _, err := time.Parse(time.RFC3339Nano, decision.ObservedAt); err != nil {
		return err
	}
	if err := validateBinaryREEvidenceReviewResponse(decision.Response, input); err != nil {
		return err
	}
	if decision.Source.SessionKind != "mission-commander-evidence-review" ||
		decision.Source.AttemptID != intent.AttemptID || decision.Source.AttemptSHA256 != intent.AttemptSHA256 ||
		decision.Source.SessionID != intent.SessionID || decision.Source.Ref == "" ||
		len(decision.Source.SHA256) != 64 || decision.Source.Bytes < 1 {
		return fmt.Errorf("binary-re evidence review decision omitted its exact raw result identity")
	}
	return nil
}

func acknowledgeBinaryREEvidenceReview(
	caseRoot,
	pack,
	lane string,
	item mission.ExecutionEvidenceReviewItem,
	input binaryREVMPIDAEvidenceReviewInput,
	intent binaryREEvidenceReviewIntent,
	decision evidenceReviewResponse,
) (string, string, error) {
	eventArgs := []string{
		"-Kind", "verification",
		"-Lane", lane,
		"-Subject", "execution evidence review accepted",
		"-Summary", "accepted recorded execution evidence for gateEventId " + input.GateEventID,
		"-Actor", binaryREAdapterActor,
		"-Verifier", "tool-review",
		"-Verdict", "accepted",
		"-Status", "resolved",
		"-Related", strings.Join([]string{item.EventID, item.GateEventID}, ","),
		"-Reason", decision.Summary + "; " + decision.Reason,
		"-TargetRef", input.RequestPath,
		"-EvidenceRefs", strings.Join(input.EvidenceRefs, ","),
		"-CreatedAt", intent.AcknowledgementCreatedAt,
	}
	preview, _, err := runPublicNotePreviewApply(caseRoot, pack, eventArgs, intent.Control)
	if err != nil {
		return "", "", fmt.Errorf("record binary-re evidence acknowledgement through public note route: %w", err)
	}
	facts, err := mission.ReadStrictLedgerFacts(caseRoot)
	if err != nil {
		return "", "", err
	}
	if !workstream.ExecutionEvidenceReviewAcknowledgedIDs(facts)[input.GateEventID] {
		return "", "", fmt.Errorf("binary-re evidence acknowledgement did not close the exact gate review")
	}
	return preview.EventID, preview.EventSHA256, nil
}

func bindBinaryREEvidenceForMember(
	caseRoot,
	lane string,
	control executioncontrol.Binding,
	input binaryREVMPIDAEvidenceReviewInput,
	executionIntentPath,
	executionIntentSHA,
	executionResultPath,
	executionResultSHA,
	inputPath,
	inputSHA,
	intentPath,
	intentSHA,
	decisionPath,
	decisionSHA,
	closurePath,
	closureSHA,
	sessionID,
	ackID,
	ackSHA string,
) (string, string, error) {
	binding := binaryREMemberTaskBinding(
		input,
		executionIntentPath,
		executionIntentSHA,
		executionResultPath,
		executionResultSHA,
		inputPath,
		inputSHA,
		intentPath,
		intentSHA,
		decisionPath,
		decisionSHA,
		closurePath,
		closureSHA,
		sessionID,
		ackID,
		ackSHA,
	)
	return memberexecution.WriteTaskBindingForOwnerWithControlBinding(
		caseRoot, lane, control.Owner.CurrentExecutor, control.Owner.ExecutorGeneration, control, binding,
	)
}

func binaryREMemberTaskBinding(
	input binaryREVMPIDAEvidenceReviewInput,
	executionIntentPath,
	executionIntentSHA,
	executionResultPath,
	executionResultSHA,
	inputPath,
	inputSHA,
	intentPath,
	intentSHA,
	decisionPath,
	decisionSHA,
	closurePath,
	closureSHA,
	sessionID,
	ackID,
	ackSHA string,
) memberexecution.TaskBinding {
	return memberexecution.TaskBinding{
		Kind: "vmp-ida-index-evidence",
		Values: map[string]string{
			"gate-event-id": input.GateEventID, "profile-hash": input.ProfileSHA256,
			"request-path": input.RequestPath, "request-sha256": input.RequestSHA256,
			"packet-path": input.PacketPath, "packet-sha256": input.PacketSHA256,
			"report-path": input.ReportPath, "report-sha256": input.ReportSHA256,
			"dispatch-path": input.DispatchPath, "dispatch-sha256": input.DispatchSHA256,
			"receipt-path": input.ReceiptPath, "receipt-sha256": input.ReceiptSHA256,
			"observation-event-id":             input.ObservationEventID,
			"selected-evidence-ref":            input.Selected.EvidenceRef,
			"execution-intent-path":            executionIntentPath,
			"execution-intent-sha256":          executionIntentSHA,
			"execution-result-path":            executionResultPath,
			"execution-result-sha256":          executionResultSHA,
			"evidence-review-input-path":       inputPath,
			"evidence-review-input-sha256":     inputSHA,
			"evidence-review-intent-path":      intentPath,
			"evidence-review-intent-sha256":    intentSHA,
			"evidence-review-decision-path":    decisionPath,
			"evidence-review-decision-sha256":  decisionSHA,
			"evidence-review-closure-path":     closurePath,
			"evidence-review-closure-sha256":   closureSHA,
			"evidence-review-session-id":       sessionID,
			"evidence-review-ack-event-id":     ackID,
			"evidence-review-ack-event-sha256": ackSHA,
		},
	}
}

func requireBinaryREControlCurrent(caseRoot string, control executioncontrol.Binding) (retErr error) {
	lease, err := lanemutation.AcquireLane(caseRoot, control.Lane)
	if err != nil {
		return err
	}
	defer func() { retErr = errors.Join(retErr, lease.Unlock()) }()
	return executioncontrol.RequireCurrentBindingWithLease(caseRoot, lease, control)
}

func binaryREAdapterArtifactPath(caseRoot, lane, gateEventID string, parts ...string) (string, error) {
	base := []string{"lanes", lane, "adapter-executions", gateEventID}
	return projectstate.Rel(caseRoot, append(base, parts...)...)
}

func writeBinaryREAdapterArtifact(caseRoot, rel, label string, value any) (string, error) {
	data, err := json.MarshalIndent(value, "", "  ")
	if err != nil {
		return "", err
	}
	data = append(data, '\n')
	if _, err := rekitfs.WriteExclusiveRegularFileAnchored(caseRoot, rel, label, data); err != nil {
		return "", err
	}
	return bytesSHA256(data), nil
}

func readBinaryREAdapterArtifact[T any](caseRoot, rel, label string) (T, []byte, bool, error) {
	var value T
	path, err := rekitfs.SafeJoin(caseRoot, rel)
	if err != nil {
		return value, nil, false, err
	}
	data, err := rekitfs.ReadStableRegularFileAnchored(caseRoot, path, label, binaryREAdapterArtifactMaxBytes)
	if errors.Is(err, os.ErrNotExist) {
		return value, nil, false, nil
	}
	if err != nil {
		return value, nil, false, err
	}
	if err := strictJSON(data, &value); err != nil {
		return value, nil, false, err
	}
	canonical, err := json.MarshalIndent(value, "", "  ")
	if err != nil {
		return value, nil, false, err
	}
	canonical = append(canonical, '\n')
	if !bytes.Equal(data, canonical) {
		return value, nil, false, fmt.Errorf("%s is not canonical", label)
	}
	return value, data, true, nil
}

func shortStableIdentity(values ...string) string {
	return hashStableIdentity(values...)[:16]
}

func hashStableIdentity(values ...string) string {
	hash := sha256.New()
	for _, value := range values {
		hash.Write([]byte(value))
		hash.Write([]byte{0})
	}
	return hex.EncodeToString(hash.Sum(nil))
}

func stableUUID(values ...string) string {
	sum := sha256.Sum256([]byte(strings.Join(values, "\x00")))
	value := append([]byte(nil), sum[:16]...)
	value[6] = (value[6] & 0x0f) | 0x50
	value[8] = (value[8] & 0x3f) | 0x80
	hexValue := hex.EncodeToString(value)
	return hexValue[:8] + "-" + hexValue[8:12] + "-" + hexValue[12:16] + "-" + hexValue[16:20] + "-" + hexValue[20:]
}
